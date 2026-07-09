"""Canon: §Proposal — mechanical writer for steward-approved JSON proposals.

Reads a steward-approved JSON proposal from a file path argument, validates it
against the proposal shape (ProposedConflictTransition, ProposedRequirement, or
ProposedRejection), locates the target node in spec/content/graph.py via AST,
applies the field changes via deterministic string replacement, regenerates docs
via gen_spec.py, and runs pytest -q to verify the change is structurally clean.
Optionally runs the P4 closure check to confirm the triggering diagnosis was
actually removed.

This is the FIRST OPERATOR ACTION TOOL: the AI operator emits a proposal
(see hotam_spec/proposal.py); the steward approves out-of-band; then the AI calls
this tool to mechanically land the change. No free-text editing of the graph.

Supported proposal kinds:
  - ConflictTransition — move a Conflict lifecycle (DETECTED → DECIDED etc.)
  - Conflict — materialize a NEW Conflict node (lifecycle starts DETECTED;
    id computed via conflict_identity(axis, context), never caller-supplied)
  - Requirement — add or update a Requirement in the graph
  - Rejection — reject an existing Requirement (status → REJECTED)
  - EntityType — add a new EntityType to the active domain's graph
  - OperatorBudget — replace an existing Operator's ContextBudget (limit/measure)
  - Axis — add a new Axis to the active domain's controlled-vocabulary `axes` tuple
  - Assumption — add a new Assumption to the active domain's `assumptions` tuple
  - Stakeholder — add a new Stakeholder to the active domain's `stakeholders` tuple

Usage:
  uv run python tools/apply_proposal.py proposal.json
  uv run python tools/apply_proposal.py --dry-run proposal.json
  uv run python tools/apply_proposal.py --triggering-kind CONFLICT_STALLED proposal.json
  uv run python tools/apply_proposal.py --batch proposals_array.json

The JSON shapes:

  ProposedConflictTransition DECIDED:
  {
    "kind": "ConflictTransition",
    "conflict_id": "C-8600b1b8",
    "new_lifecycle": "DECIDED(... rationale text ...)",
    "decided_by": "domain-user",
    "revisit_marker": "REVISIT if ...",
    "derived": ["R-foo"]
  }

  ProposedConflictTransition HELD (not resolvable by amending the members;
  requires the same human signoff as DECIDED, plus >=2 elaborated variants):
  {
    "kind": "ConflictTransition",
    "conflict_id": "C-8600b1b8",
    "new_lifecycle": "HELD(... reason it cannot be resolved by amending members ...)",
    "decided_by": "domain-user",
    "variants": [
      {"id": "V-foo", "behavior": "...", "implies": "...", "costs": "..."},
      {"id": "V-bar", "behavior": "...", "implies": "...", "costs": "..."}
    ]
  }

  ProposedRequirement (add or update):
  {
    "kind": "Requirement",
    "id": "R-foo",
    "claim": "The system shall ...",
    "owner": "framework-author",
    "status": "DRAFT",
    "why": "...",
    "assumptions": ["A-python-stack"],
    "enforcement": "ENFORCED",
    "enforced_by": ["check_foo"]
  }

  ProposedRejection:
  {
    "kind": "Rejection",
    "requirement_id": "R-foo",
    "reason": "REJECTED — REPLACES R-bar; see R-new"
  }

  ProposedConflict (materialize a new Conflict node, lifecycle DETECTED):
  {
    "kind": "Conflict",
    "axis": "core-vs-aspect",
    "context": "the scenario in which the members actually collide",
    "members": ["R-foo", "R-bar"],
    "steward": "framework-reviewer",
    "shared_assumption": "A-baz",
    "note": "presentation-only context for the steward (not written to the graph)"
  }

  ProposedOperatorBudget:
  {
    "kind": "OperatorBudget",
    "operator_id": "OP-director",
    "new_limit": 150000,
    "new_measure": "CRYSTAL_CHARS",
    "why": "..."
  }

  ProposedAxis (add a new Axis to the active domain's axes tuple):
  {
    "kind": "Axis",
    "slug": "cost-vs-flexibility",
    "description": "...",
    "why": "..."
  }

  ProposedAssumption (add a new Assumption to the active domain's assumptions tuple):
  {
    "kind": "Assumption",
    "id": "A-foo",
    "statement": "...",
    "status": "HOLDS",
    "owner": "framework-author",
    "why": "..."
  }

  ProposedStakeholder (add a new Stakeholder to the active domain's stakeholders tuple):
  {
    "kind": "Stakeholder",
    "id": "finance",
    "name": "Finance",
    "domain": "money",
    "why": "..."
  }

  ProposedConflictMemberUpdate (add/remove members on an EXISTING conflict;
  post-update members MUST stay >= 2 — R-conflict-min-two-members):
  {
    "kind": "ConflictMemberUpdate",
    "conflict_id": "C-8600b1b8",
    "add_members": ["R-new-party"],
    "remove_members": ["R-old-party"],
    "decided_by": "framework-reviewer"
  }

Exit codes:
  0 — success (write landed, tests green, and if --triggering-kind was supplied:
      closure confirmed — the action is no longer in the post-apply diagnosis).
  1 — failure (validation error, missing id, or pytest red).
  2 — not advanced (write landed, tests green, but the triggering action STILL
      appears in the post-apply diagnosis — the tick (P5) must NOT count this
      as progress; investigate before marking closed).
"""

from __future__ import annotations

import argparse
import ast
import json
import os as _os
import subprocess
import sys
from pathlib import Path

# --- Make hotam_spec importable --------------------------------------------------

_SPEC_ROOT = Path(__file__).resolve().parents[1]
_SRC = _SPEC_ROOT / "src"
if str(_SRC) not in sys.path:
    sys.path.insert(0, str(_SRC))

import re as _re  # noqa: E402

from hotam_spec.conflict import Variant, conflict_identity  # noqa: E402
from hotam_spec.entity import ENTITY_FIELD_KINDS  # noqa: E402
from hotam_spec.lifecycle import STATE_KINDS  # noqa: E402
from hotam_spec.operator import BUDGET_MEASURES  # noqa: E402
from hotam_spec.signoff import Signoff  # noqa: E402
from hotam_spec.assumption import (  # noqa: E402
    ASSUMPTION_STATES,
    DEAD,
    HOLDS,
    IMPLEMENTS,
)
from hotam_spec.proposal import (  # noqa: E402
    Proposal,
    ProposedAssumption,
    ProposedAssumptionTransition,
    ProposedAxis,
    ProposedConflict,
    ProposedConflictMemberUpdate,
    ProposedConflictTransition,
    ProposedEntityType,
    ProposedOperatorBudget,
    ProposedRejection,
    ProposedRequirement,
    ProposedStakeholder,
)

_SLUG_RE = _re.compile(r"^[a-z][a-z0-9-]*$")
_DOMAINS_ROOT = _SPEC_ROOT.parent / "domains"
#: Pin file naming the default active domain when HOTAM_SPEC_ACTIVE_DOMAIN is
#: unset. Lives at domains/.active-domain — COMMITTED (unlike spec/.runtime/,
#: which is gitignored ephemera) so the default is a deliberate, versioned
#: decision. Same repo path as gen_spec.py::_ACTIVE_DOMAIN_PIN_FILE and
#: hotam_spec.graph._ACTIVE_DOMAIN_PIN_FILE (R-active-domain-pin-not-alphabetical).
_ACTIVE_DOMAIN_PIN_FILE = _SPEC_ROOT.parent / "domains" / ".active-domain"


def _resolve_content_graph() -> "Path":
    """Return the active graph.py: domains/<active>/graph.py or legacy spec/content/graph.py.

    RULE: resolution order is (1) HOTAM_SPEC_ACTIVE_DOMAIN env var (must name
    an existing domains/<name>/ dir), (2) spec/.runtime/active-domain pin
    file (must name an existing domains/<name>/ dir), (3) the first domain
    alphabetically. Mirrors hotam_spec.graph._active_domain_graph_file()'s
    and gen_spec.py::_select_active_domain_dir()'s resolution order exactly,
    so all three tools agree on which domain is active when more than one
    domain exists (previously this function ignored the env var entirely and
    always wrote to the alphabetically-first domain's graph.py, silently
    mis-targeting proposals when a second domain sorted first; the pin file
    closes the follow-up gap where even the env-var-aware version still fell
    back to a silent alphabetical default with no committed record of intent).
    """
    if _DOMAINS_ROOT.exists():
        domain_dirs = sorted(
            d
            for d in _DOMAINS_ROOT.iterdir()
            if d.is_dir() and not d.name.startswith("_")
        )
        if domain_dirs:
            env_domain = _os.environ.get("HOTAM_SPEC_ACTIVE_DOMAIN", "").strip()
            if env_domain:
                for d in domain_dirs:
                    if d.name == env_domain:
                        return d / "graph.py"
            if _ACTIVE_DOMAIN_PIN_FILE.exists():
                pinned = _ACTIVE_DOMAIN_PIN_FILE.read_text(encoding="utf-8").strip()
                if pinned:
                    for d in domain_dirs:
                        if d.name == pinned:
                            return d / "graph.py"
            return domain_dirs[0] / "graph.py"
    return _SPEC_ROOT / "content" / "graph.py"


_CONTENT_GRAPH = _resolve_content_graph()
_GEN_SPEC = Path(__file__).resolve().parent / "gen_spec.py"
_RUNTIME_DIR = _SPEC_ROOT / ".runtime"
_LAND_LOG_NAME = "land-log.jsonl"
_PROPOSALS_DIR = _RUNTIME_DIR / "proposals"
_PROPOSALS_APPLIED_DIR = _PROPOSALS_DIR / "applied"
"""Canon: §Proposal — the applied/ sub-folder a landed proposal file is moved
into (R-presented-pending-decision-type).

RULE: a proposal file that lives directly under spec/.runtime/proposals/ (or
under proposals/pending/) is PRESENTED-AWAITING-DECISION; once apply_proposal.py
lands it successfully (write + regen + verify tier all green, and closure
advanced when --triggering-kind was supplied), the file is moved into
proposals/applied/ — same filename, stamped by mtime. Files directly under
proposals/ (the historical flat layout) are treated as pending for backward
compatibility; nothing under proposals/ is ever deleted.

WHY a folder, not a graph node type: the steward's verdict (2026-07-02) was
'да, нужно. наверно такие вещи нужно вести в отдельной папке' — the
PRESENT-awaiting-decision state is transient tooling ephemera, not a business
tension; giving it a node type would put process bookkeeping in the same
ontology as Requirement/Conflict. See R-presented-pending-decision-type.
"""

# ---------------------------------------------------------------------------
# Validation helpers
# ---------------------------------------------------------------------------


def _validate_proposal(raw: dict) -> Proposal:
    """Canon: §Proposal — parse and validate a JSON dict into a Proposal variant.

    RULE: 'kind' must be one of 'ConflictTransition', 'Conflict', 'Requirement',
    'Rejection', 'EntityType', 'OperatorBudget', 'Axis', or 'Assumption'. Each
    kind has its own required fields.

    Returns a Proposal (one of the proposal dataclass variants) or raises
    ValueError with a clear message.
    """
    kind = raw.get("kind", "")
    if kind == "ConflictTransition":
        return _validate_conflict_transition(raw)
    if kind == "Conflict":
        return _validate_conflict(raw)
    if kind == "Requirement":
        return _validate_requirement(raw)
    if kind == "Rejection":
        return _validate_rejection(raw)
    if kind == "EntityType":
        return _validate_entity_type(raw)
    if kind == "OperatorBudget":
        return _validate_operator_budget(raw)
    if kind == "Axis":
        return _validate_axis(raw)
    if kind == "Assumption":
        return _validate_assumption(raw)
    if kind == "Stakeholder":
        return _validate_stakeholder(raw)
    if kind == "AssumptionTransition":
        return _validate_assumption_transition(raw)
    if kind == "ConflictMemberUpdate":
        return _validate_conflict_member_update(raw)
    raise ValueError(
        f"Unsupported proposal kind '{kind}'. "
        f"Supported: 'ConflictTransition', 'Conflict', 'Requirement', "
        f"'Rejection', 'EntityType', 'OperatorBudget', 'Axis', 'Assumption', "
        f"'Stakeholder', 'AssumptionTransition', 'ConflictMemberUpdate'."
    )


def _validate_conflict_transition(raw: dict) -> ProposedConflictTransition:
    """Parse and validate a ConflictTransition proposal."""
    conflict_id = raw.get("conflict_id", "").strip()
    if not conflict_id:
        raise ValueError("'conflict_id' is required and must be non-empty.")
    new_lifecycle = raw.get("new_lifecycle", "").strip()
    if not new_lifecycle:
        raise ValueError("'new_lifecycle' is required and must be non-empty.")
    decided_by = raw.get("decided_by", "").strip()
    if new_lifecycle.startswith("DECIDED") and not decided_by:
        raise ValueError(
            "new_lifecycle starts with DECIDED but decided_by is empty. "
            "A DECIDED transition requires a human decider "
            "(R-decided-needs-human-signoff)."
        )
    variants_raw = raw.get("variants", [])
    if not isinstance(variants_raw, list):
        raise ValueError(
            "'variants' must be a list of {id, behavior, implies, costs} objects."
        )
    variants = tuple(
        Variant(
            id=str(v.get("id", "")),
            behavior=str(v.get("behavior", "")),
            implies=str(v.get("implies", "")),
            costs=str(v.get("costs", "")),
        )
        for v in variants_raw
    )
    if new_lifecycle.startswith("HELD"):
        if not decided_by:
            raise ValueError(
                "new_lifecycle starts with HELD but decided_by is empty. "
                "A HELD transition requires a human signoff, the same lock "
                "as DECIDED (R-decided-needs-human-signoff)."
            )
        if len({v.id for v in variants}) < 2:
            raise ValueError(
                "new_lifecycle starts with HELD but fewer than 2 distinct "
                "Variant ids were supplied (check_held_has_min_two_variants "
                "requires >= 2)."
            )
    revisit_marker = raw.get("revisit_marker", "")
    derived_raw = raw.get("derived", [])
    if not isinstance(derived_raw, list):
        raise ValueError("'derived' must be a list of R-id strings.")
    derived = tuple(str(x) for x in derived_raw)
    shared_assumption = raw.get("shared_assumption", "")
    return ProposedConflictTransition(
        conflict_id=conflict_id,
        new_lifecycle=new_lifecycle,
        decided_by=decided_by,
        revisit_marker=revisit_marker if isinstance(revisit_marker, str) else "",
        derived=derived,
        variants=variants,
        shared_assumption=(
            shared_assumption.strip() if isinstance(shared_assumption, str) else ""
        ),
        date=(raw.get("date", "") or "").strip() if isinstance(raw.get("date", ""), str) else "",
        verbatim=raw.get("verbatim", "") if isinstance(raw.get("verbatim", ""), str) else "",
        instrument=(raw.get("instrument", "personal") or "personal").strip()
        if isinstance(raw.get("instrument", "personal"), str)
        else "personal",
        chosen_variant=(raw.get("chosen_variant", "") or "").strip()
        if isinstance(raw.get("chosen_variant", ""), str)
        else "",
    )


def _validate_requirement(raw: dict) -> ProposedRequirement:
    """Parse and validate a Requirement proposal."""
    req_id = raw.get("id", "").strip()
    if not req_id:
        raise ValueError("'id' is required for a Requirement proposal.")
    claim = raw.get("claim", "").strip()
    if not claim:
        raise ValueError("'claim' is required and must be non-empty.")
    owner = raw.get("owner", "").strip()
    if not owner:
        raise ValueError("'owner' is required and must be non-empty.")
    status = raw.get("status", "").strip()
    if not status:
        raise ValueError("'status' is required and must be non-empty.")
    why = raw.get("why", "")
    assumptions_raw = raw.get("assumptions", [])
    if not isinstance(assumptions_raw, list):
        raise ValueError("'assumptions' must be a list of assumption id strings.")
    assumptions = tuple(str(x) for x in assumptions_raw)
    relations_raw = raw.get("relations", [])
    if not isinstance(relations_raw, list):
        raise ValueError("'relations' must be a list of [kind, target] pairs.")
    relations = tuple((str(r[0]), str(r[1])) for r in relations_raw)
    enforcement = raw.get("enforcement", "PROSE").strip()
    enforced_by_raw = raw.get("enforced_by", [])
    if not isinstance(enforced_by_raw, list):
        raise ValueError("'enforced_by' must be a list of strings.")
    enforced_by = tuple(str(x) for x in enforced_by_raw)
    m_tag = raw.get("m_tag", "")
    enforceability = raw.get("enforceability", "ENFORCEABLE").strip()
    summary = raw.get("summary", "")
    created_at = (raw.get("created_at", "") or "").strip()
    settled_at = (raw.get("settled_at", "") or "").strip()
    return ProposedRequirement(
        id=req_id,
        claim=claim,
        owner=owner,
        status=status,
        why=why if isinstance(why, str) else "",
        assumptions=assumptions,
        relations=relations,
        enforcement=enforcement,
        enforced_by=enforced_by,
        m_tag=m_tag if isinstance(m_tag, str) else "",
        enforceability=enforceability,
        summary=summary if isinstance(summary, str) else "",
        created_at=created_at if isinstance(created_at, str) else "",
        settled_at=settled_at if isinstance(settled_at, str) else "",
    )


def _validate_rejection(raw: dict) -> ProposedRejection:
    """Parse and validate a Rejection proposal."""
    requirement_id = raw.get("requirement_id", "").strip()
    if not requirement_id:
        raise ValueError("'requirement_id' is required for a Rejection proposal.")
    reason = raw.get("reason", "").strip()
    if not reason:
        raise ValueError("'reason' is required and must be non-empty.")
    replaced_by_raw = raw.get("replaced_by", [])
    if isinstance(replaced_by_raw, str):
        replaced_by_raw = [replaced_by_raw]
    if not isinstance(replaced_by_raw, list):
        raise ValueError("'replaced_by' must be a string or a list of R-id strings.")
    replaced_by = tuple(str(x).strip() for x in replaced_by_raw if str(x).strip())
    return ProposedRejection(
        requirement_id=requirement_id,
        reason=reason,
        replaced_by=replaced_by,
    )


def _validate_conflict_member_update(raw: dict) -> ProposedConflictMemberUpdate:
    """Parse and validate a ConflictMemberUpdate proposal.

    RULE: conflict_id non-empty; add_members and remove_members are lists of
    R-id strings (or absent → empty). At least ONE of add/remove must be
    non-empty (a no-op update is a mistake, not a proposal). The post-update
    members count is checked at apply-time (must stay >= 2), not here — the
    validator cannot see the current members without the graph source.
    """
    conflict_id = raw.get("conflict_id", "").strip()
    if not conflict_id:
        raise ValueError(
            "'conflict_id' is required for a ConflictMemberUpdate proposal."
        )
    add_raw = raw.get("add_members", [])
    rem_raw = raw.get("remove_members", [])
    if not isinstance(add_raw, list):
        raise ValueError("'add_members' must be a list of R-id strings.")
    if not isinstance(rem_raw, list):
        raise ValueError("'remove_members' must be a list of R-id strings.")
    add_members = tuple(str(x).strip() for x in add_raw if str(x).strip())
    remove_members = tuple(str(x).strip() for x in rem_raw if str(x).strip())
    if not add_members and not remove_members:
        raise ValueError(
            "at least one of 'add_members' / 'remove_members' must be non-empty "
            "(a ConflictMemberUpdate with neither is a no-op)."
        )
    decided_by = raw.get("decided_by", "").strip()
    return ProposedConflictMemberUpdate(
        conflict_id=conflict_id,
        add_members=add_members,
        remove_members=remove_members,
        decided_by=decided_by,
    )


def _validate_conflict(raw: dict) -> ProposedConflict:
    """Parse and validate a Conflict (creation) proposal.

    RULE: axis, context, steward non-empty; members >= 2 distinct R-… ids;
    no caller-supplied id/conflict_id (the writer computes
    conflict_identity(axis, context) — R-stable-conflict-identity); no
    caller-supplied lifecycle (a new Conflict starts DETECTED; moving it is a
    separate ConflictTransition proposal).
    """
    if "id" in raw or "conflict_id" in raw:
        raise ValueError(
            "a Conflict proposal must not supply an id — the writer computes "
            "conflict_identity(axis, context) (R-stable-conflict-identity)."
        )
    if "lifecycle" in raw:
        raise ValueError(
            "a Conflict proposal must not supply a lifecycle — a new Conflict "
            "starts DETECTED; transitions are a separate ConflictTransition "
            "proposal."
        )
    axis = raw.get("axis", "").strip()
    if not axis:
        raise ValueError("'axis' is required and must be non-empty.")
    context = raw.get("context", "").strip()
    if not context:
        raise ValueError("'context' is required and must be non-empty.")
    members_raw = raw.get("members", [])
    if not isinstance(members_raw, list):
        raise ValueError("'members' must be a list of R-id strings.")
    members = tuple(str(x).strip() for x in members_raw)
    if len(set(members)) < 2:
        raise ValueError(
            "'members' must contain at least two DISTINCT requirement ids "
            "(R-conflict-min-two-members)."
        )
    for m in members:
        if not m.startswith("R-"):
            raise ValueError(f"member '{m}' must be an R-… requirement id.")
    steward = raw.get("steward", "").strip()
    if not steward:
        raise ValueError("'steward' is required and must be non-empty.")
    shared_assumption = raw.get("shared_assumption", "")
    note = raw.get("note", "")
    initial_lifecycle = raw.get("initial_lifecycle", "DETECTED")
    if not isinstance(initial_lifecycle, str) or not initial_lifecycle.strip():
        initial_lifecycle = "DETECTED"
    initial_lifecycle = initial_lifecycle.strip()
    decided_by = raw.get("decided_by", "").strip()
    if initial_lifecycle.startswith("DECIDED") and not decided_by:
        raise ValueError(
            "initial_lifecycle starts with DECIDED but decided_by is empty. "
            "Materializing a conflict already-DECIDED requires a human decider "
            "(R-decided-needs-human-signoff)."
        )
    if (
        not initial_lifecycle.startswith("DECIDED")
        and initial_lifecycle != "DETECTED"
    ):
        raise ValueError(
            "initial_lifecycle must be 'DETECTED' (the default) or start with "
            "'DECIDED(...)'. Other lifecycles (ACKNOWLEDGED / REVISIT_WHEN / "
            "HELD) are reached via a separate ConflictTransition, never at "
            "creation."
        )
    return ProposedConflict(
        axis=axis,
        context=context,
        members=members,
        steward=steward,
        shared_assumption=(
            shared_assumption.strip() if isinstance(shared_assumption, str) else ""
        ),
        note=note if isinstance(note, str) else "",
        initial_lifecycle=initial_lifecycle,
        decided_by=decided_by,
    )


def _validate_operator_budget(raw: dict) -> ProposedOperatorBudget:
    """Parse and validate an OperatorBudget proposal."""
    operator_id = raw.get("operator_id", "").strip()
    if not operator_id:
        raise ValueError("'operator_id' is required for an OperatorBudget proposal.")
    if not operator_id.startswith("OP-"):
        raise ValueError(f"'operator_id' must start with 'OP-'; got '{operator_id}'.")
    new_limit = raw.get("new_limit")
    if not isinstance(new_limit, int) or isinstance(new_limit, bool):
        raise ValueError("'new_limit' is required and must be an int.")
    if new_limit < 0:
        raise ValueError(f"'new_limit' must be >= 0; got {new_limit}.")
    new_measure = raw.get("new_measure", "").strip()
    if new_measure not in BUDGET_MEASURES:
        raise ValueError(
            f"'new_measure' must be one of {sorted(BUDGET_MEASURES)}; "
            f"got '{new_measure}'."
        )
    why = raw.get("why", "")
    return ProposedOperatorBudget(
        operator_id=operator_id,
        new_limit=new_limit,
        new_measure=new_measure,
        why=why if isinstance(why, str) else "",
    )


def _validate_axis(raw: dict) -> ProposedAxis:
    """Parse and validate an Axis proposal."""
    slug = raw.get("slug", "").strip()
    if not slug:
        raise ValueError("'slug' is required for an Axis proposal.")
    if not _SLUG_RE.match(slug):
        raise ValueError(
            f"'slug' must be kebab-case (lowercase letters, digits, hyphens, "
            f"starting with a letter); got '{slug}'."
        )
    description = raw.get("description", "").strip()
    if not description:
        raise ValueError("'description' is required and must be non-empty.")
    why = raw.get("why", "")
    return ProposedAxis(
        slug=slug,
        description=description,
        why=why if isinstance(why, str) else "",
    )


def _validate_assumption(raw: dict) -> ProposedAssumption:
    """Parse and validate an Assumption proposal."""
    assumption_id = raw.get("id", "").strip()
    if not assumption_id:
        raise ValueError("'id' is required for an Assumption proposal.")
    if not assumption_id.startswith("A-"):
        raise ValueError(
            f"'id' must start with 'A-' (R-anchor-everything); got '{assumption_id}'."
        )
    statement = raw.get("statement", "").strip()
    if not statement:
        raise ValueError("'statement' is required and must be non-empty.")
    status = raw.get("status", "").strip()
    if status not in ASSUMPTION_STATES:
        raise ValueError(
            f"'status' must be one of {sorted(ASSUMPTION_STATES)}; got '{status}'."
        )
    owner = raw.get("owner", "").strip()
    if not owner:
        raise ValueError("'owner' is required and must be non-empty.")
    why = raw.get("why", "")
    created_at = (raw.get("created_at", "") or "").strip()
    return ProposedAssumption(
        id=assumption_id,
        statement=statement,
        status=status,
        owner=owner,
        why=why if isinstance(why, str) else "",
        created_at=created_at if isinstance(created_at, str) else "",
    )


def _validate_stakeholder(raw: dict) -> ProposedStakeholder:
    """Parse and validate a Stakeholder proposal."""
    stakeholder_id = raw.get("id", "").strip()
    if not stakeholder_id:
        raise ValueError("'id' is required for a Stakeholder proposal.")
    name = raw.get("name", "").strip()
    if not name:
        raise ValueError("'name' is required and must be non-empty.")
    domain = raw.get("domain", "").strip()
    if not domain:
        raise ValueError("'domain' is required and must be non-empty.")
    why = raw.get("why", "")
    return ProposedStakeholder(
        id=stakeholder_id,
        name=name,
        domain=domain,
        why=why if isinstance(why, str) else "",
    )


def _validate_assumption_transition(raw: dict) -> ProposedAssumptionTransition:
    """Parse and validate an AssumptionTransition proposal (the kill-path).

    RULE: assumption_id + non-empty reason required; new_status in
    ASSUMPTION_STATES; decided_by REQUIRED when new_status in
    (DEAD, HOLDS, IMPLEMENTS) — the signoff asymmetry: only UNCERTAIN (a
    signal-RAISING doubt) is agent-enterable; DEAD/HOLDS reduce live signal and
    IMPLEMENTS re-types a fact-claim into a VOLITIONAL aspiration
    (R-assumption-implements-state), all steward acts (see
    ProposedAssumptionTransition docstring for the full transition table).
    """
    assumption_id = raw.get("assumption_id", "").strip()
    if not assumption_id:
        raise ValueError(
            "'assumption_id' is required for an AssumptionTransition proposal."
        )
    new_status = raw.get("new_status", "").strip()
    if new_status not in ASSUMPTION_STATES:
        raise ValueError(
            f"'new_status' must be one of {sorted(ASSUMPTION_STATES)}; "
            f"got '{new_status}'."
        )
    reason = raw.get("reason", "").strip()
    if not reason:
        raise ValueError(
            "'reason' is required and must be non-empty — an assumption status "
            "change with no recorded reason is drift, not a decision."
        )
    decided_by = raw.get("decided_by", "").strip()
    if new_status in (DEAD, HOLDS, IMPLEMENTS) and not decided_by:
        raise ValueError(
            f"'decided_by' (a Stakeholder id) is required when new_status is "
            f"'{new_status}': a transition that reduces live signal or re-types a "
            f"fact-claim into a VOLITIONAL aspiration needs a named human signoff "
            f"(R-assumption-implements-state, R-trust-anchor-mechanism, "
            f"R-ai-presents-not-decides)."
        )
    return ProposedAssumptionTransition(
        assumption_id=assumption_id,
        new_status=new_status,
        reason=reason,
        decided_by=decided_by,
        date=(raw.get("date", "") or "").strip() if isinstance(raw.get("date", ""), str) else "",
        verbatim=raw.get("verbatim", "") if isinstance(raw.get("verbatim", ""), str) else "",
        instrument=(raw.get("instrument", "personal") or "personal").strip()
        if isinstance(raw.get("instrument", "personal"), str)
        else "personal",
    )


def _validate_entity_type(raw: dict) -> ProposedEntityType:
    """Parse and validate an EntityType proposal."""
    slug = raw.get("slug", "").strip()
    if not slug:
        raise ValueError("'slug' is required for an EntityType proposal.")
    if not _SLUG_RE.match(slug):
        raise ValueError(
            f"'slug' must be kebab-case (lowercase letters, digits, hyphens, "
            f"starting with a letter); got '{slug}'."
        )
    description = raw.get("description", "").strip()
    if not description:
        raise ValueError("'description' is required and must be non-empty.")
    why = raw.get("why", "")

    # Validate states
    states_raw = raw.get("states", [])
    if not isinstance(states_raw, list) or not states_raw:
        raise ValueError(
            "'states' must be a non-empty list of [name, kind, why] triples."
        )
    states: list[tuple[str, str, str]] = []
    for item in states_raw:
        if not isinstance(item, (list, tuple)) or len(item) < 2:
            raise ValueError(
                f"Each state must be [name, kind] or [name, kind, why]; got {item!r}."
            )
        s_name, s_kind = str(item[0]), str(item[1])
        s_why = str(item[2]) if len(item) > 2 else ""
        if s_kind not in STATE_KINDS:
            raise ValueError(
                f"State kind '{s_kind}' is not valid; must be one of {sorted(STATE_KINDS)}."
            )
        states.append((s_name, s_kind, s_why))

    # Exactly one initial state
    initial_count = sum(1 for _, k, _ in states if k == "initial")
    if initial_count != 1:
        raise ValueError(
            f"Exactly one state must have kind='initial'; found {initial_count}."
        )

    state_names = {s[0] for s in states}

    # Validate transitions
    transitions_raw = raw.get("transitions", [])
    if not isinstance(transitions_raw, list):
        raise ValueError("'transitions' must be a list of [src, dst, event] triples.")
    transitions: list[tuple[str, str, str]] = []
    for item in transitions_raw:
        if not isinstance(item, (list, tuple)) or len(item) < 3:
            raise ValueError(
                f"Each transition must be [src, dst, event]; got {item!r}."
            )
        t_src, t_dst, t_event = str(item[0]), str(item[1]), str(item[2])
        if t_src not in state_names:
            raise ValueError(
                f"Transition src '{t_src}' is not a declared state name. "
                f"Declared: {sorted(state_names)}."
            )
        if t_dst not in state_names:
            raise ValueError(
                f"Transition dst '{t_dst}' is not a declared state name. "
                f"Declared: {sorted(state_names)}."
            )
        transitions.append((t_src, t_dst, t_event))

    cyclic = bool(raw.get("cyclic", False))

    # Validate fields
    fields_raw = raw.get("fields", [])
    if not isinstance(fields_raw, list):
        raise ValueError(
            "'fields' must be a list of [name, kind, required, ref_target] tuples."
        )
    fields: list[tuple[str, str, bool, str]] = []
    for item in fields_raw:
        if not isinstance(item, (list, tuple)) or len(item) < 2:
            raise ValueError(f"Each field must be at least [name, kind]; got {item!r}.")
        f_name, f_kind = str(item[0]), str(item[1])
        f_required = bool(item[2]) if len(item) > 2 else False
        f_ref_target = str(item[3]) if len(item) > 3 else ""
        if f_kind not in ENTITY_FIELD_KINDS:
            raise ValueError(
                f"Field kind '{f_kind}' is not valid; must be one of {sorted(ENTITY_FIELD_KINDS)}."
            )
        fields.append((f_name, f_kind, f_required, f_ref_target))

    return ProposedEntityType(
        slug=slug,
        description=description,
        why=why if isinstance(why, str) else "",
        states=tuple(states),
        transitions=tuple(transitions),
        cyclic=cyclic,
        fields=tuple(fields),
    )


# ---------------------------------------------------------------------------
# AST-based requirement locator
# ---------------------------------------------------------------------------


def _find_requirement_call(tree: ast.AST, req_id: str) -> ast.Call | None:
    """Canon: §Proposal — locate the Requirement(...) AST call whose id matches.

    Walks the AST looking for ast.Call nodes whose function is 'Requirement'. For
    each, extracts the 'id' keyword arg (string literal only). Returns the matching
    node or None.
    """
    for node in ast.walk(tree):
        if not isinstance(node, ast.Call):
            continue
        func = node.func
        if isinstance(func, ast.Name) and func.id != "Requirement":
            continue
        if isinstance(func, ast.Attribute) and func.attr != "Requirement":
            continue
        # Extract id= kwarg
        for kw in node.keywords:
            if kw.arg == "id" and isinstance(kw.value, ast.Constant):
                if kw.value.value == req_id:
                    return node  # type: ignore[return-value]
    return None


def _extract_requirement_relations(call: ast.Call) -> tuple[tuple[str, str], ...]:
    """Canon: §Proposal — read the (kind, target) pairs from a Requirement's relations=.

    Parses the `relations=` kwarg of a Requirement(...) call from its AST,
    handling both bare-string-tuple shorthand ("supports", "R-x") and the typed
    Relation("kind", "target") constructor form. Returns a tuple of pairs;
    empty if the kwarg is absent. Used by the replaces-edge writer to APPEND a
    new edge to a replacing requirement's existing relations without clobbering.
    """
    for kw in call.keywords:
        if kw.arg != "relations":
            continue
        val = kw.value
        # Relation("kind", "target") constructor calls inside a tuple.
        out: list[tuple[str, str]] = []
        # The value is either a Tuple of calls/literals, or a single call.
        elts: list[ast.expr] = []
        if isinstance(val, ast.Tuple):
            elts = list(val.elts)
        else:
            elts = [val]
        for elt in elts:
            if isinstance(elt, ast.Call):
                func = elt.func
                is_relation = (
                    (isinstance(func, ast.Name) and func.id == "Relation")
                    or (isinstance(func, ast.Attribute) and func.attr == "Relation")
                )
                if not is_relation:
                    continue
                args = [a for a in elt.args if isinstance(a, ast.Constant)]
                if len(args) >= 2 and isinstance(args[0].value, str) and isinstance(args[1].value, str):
                    out.append((args[0].value, args[1].value))
            elif isinstance(elt, ast.Tuple):
                # bare ("kind", "target") shorthand
                consts = [a for a in elt.elts if isinstance(a, ast.Constant)]
                if len(consts) >= 2 and isinstance(consts[0].value, str) and isinstance(consts[1].value, str):
                    out.append((consts[0].value, consts[1].value))
        return tuple(out)
    return ()


def _find_requirements_tuple_end(tree: ast.AST, source_lines: list[str]) -> int | None:
    """Find the line number (1-indexed) of the closing ')' of `requirements = (...)`.

    Looks for an assignment `requirements = (...)` inside `build_graph()` and
    returns the end_lineno of the Tuple node (the line with the closing paren).
    """
    for node in ast.walk(tree):
        if not isinstance(node, ast.FunctionDef):
            continue
        if node.name != "build_graph":
            continue
        for stmt in node.body:
            if not isinstance(stmt, ast.Assign):
                continue
            for target in stmt.targets:
                if isinstance(target, ast.Name) and target.id == "requirements":
                    # The value is a Tuple
                    val = stmt.value
                    end = getattr(val, "end_lineno", None)
                    return end
    return None


# ---------------------------------------------------------------------------
# AST-based conflict locator
# ---------------------------------------------------------------------------


def _collect_string_assignments(tree: ast.AST) -> dict[str, str]:
    """Canon: §Proposal — fold simple `name = "literal"` assignments for kwarg resolution.

    RULE: collect every ast.Assign whose single target is a bare Name and whose
    value is a string Constant (parenthesized implicit concatenations are folded
    into one Constant by the parser, so they resolve too). A name bound more
    than once with DIFFERENT string values is ambiguous and is DROPPED from the
    map — an ambiguous binding must never silently resolve to the wrong tension.

    WHY: domain graphs bind axis/context prose to local variables for
    readability (c1_axis = "…"; Conflict(axis=c1_axis, …)). Without folding,
    no such Conflict node is addressable by ConflictTransition proposals — the
    C-8600b1b8 lesson (R-conflict-addressing-resolves-variables).
    """
    values: dict[str, str] = {}
    ambiguous: set[str] = set()
    for node in ast.walk(tree):
        if not isinstance(node, ast.Assign) or len(node.targets) != 1:
            continue
        target = node.targets[0]
        if not isinstance(target, ast.Name):
            continue
        if not (
            isinstance(node.value, ast.Constant) and isinstance(node.value.value, str)
        ):
            continue
        name = target.id
        if name in values and values[name] != node.value.value:
            ambiguous.add(name)
        values[name] = node.value.value
    for name in ambiguous:
        values.pop(name, None)
    return values


def _resolve_str_kwarg(value: ast.expr, assignments: dict[str, str]) -> str | None:
    """Canon: §Proposal — resolve a kwarg value node to a string, or None.

    RULE: an ast.Constant string resolves to itself; an ast.Name resolves
    through the folded simple-assignment map (see _collect_string_assignments);
    anything else (f-strings, calls, attribute lookups) is unresolvable and
    returns None.
    """
    if isinstance(value, ast.Constant) and isinstance(value.value, str):
        return value.value
    if isinstance(value, ast.Name):
        return assignments.get(value.id)
    return None


def _find_conflict_call(tree: ast.AST, conflict_id: str) -> ast.Call | None:
    """Canon: §Proposal — locate the Conflict(...) AST call whose computed id matches.

    Walks the AST looking for ast.Call nodes whose function is 'Conflict'. For
    each, resolves the 'axis' and 'context' keyword args — string literals OR
    simple string-variable references folded via _collect_string_assignments
    (R-conflict-addressing-resolves-variables) — and computes
    conflict_identity(axis, context). Returns the matching node or None.
    """
    assignments = _collect_string_assignments(tree)
    for node in ast.walk(tree):
        if not isinstance(node, ast.Call):
            continue
        func = node.func
        # Match: Conflict(...) — either bare name or attribute
        if isinstance(func, ast.Name) and func.id != "Conflict":
            continue
        if isinstance(func, ast.Attribute) and func.attr != "Conflict":
            continue
        # Resolve axis= and context= kwargs (literal or folded variable)
        kwargs: dict[str, str] = {}
        for kw in node.keywords:
            if kw.arg in ("axis", "context"):
                resolved = _resolve_str_kwarg(kw.value, assignments)
                if resolved is not None:
                    kwargs[kw.arg] = resolved
        if "axis" not in kwargs or "context" not in kwargs:
            continue
        computed = conflict_identity(kwargs["axis"], kwargs["context"])
        if computed == conflict_id:
            return node  # type: ignore[return-value]
    return None


# ---------------------------------------------------------------------------
# AST-based operator locator
# ---------------------------------------------------------------------------


def _find_operator_call(tree: ast.AST, operator_id: str) -> ast.Call | None:
    """Canon: §Proposal — locate the Operator(...) AST call whose id matches.

    Walks the AST looking for ast.Call nodes whose function is 'Operator'. For
    each, extracts the 'id' keyword arg (string literal only). Returns the
    matching node or None.
    """
    for node in ast.walk(tree):
        if not isinstance(node, ast.Call):
            continue
        func = node.func
        if isinstance(func, ast.Name) and func.id != "Operator":
            continue
        if isinstance(func, ast.Attribute) and func.attr != "Operator":
            continue
        for kw in node.keywords:
            if kw.arg == "id" and isinstance(kw.value, ast.Constant):
                if kw.value.value == operator_id:
                    return node  # type: ignore[return-value]
    return None


# ---------------------------------------------------------------------------
# Field replacement on source text
# ---------------------------------------------------------------------------


def _byte_col_to_char_col(line: str, byte_col: int) -> int:
    """Canon: §Proposal — convert an ast col_offset (UTF-8 bytes) to a char index.

    `ast` node col_offset/end_col_offset values are UTF-8 BYTE offsets into the
    source line, but Python string slicing (`line[:col]`) indexes by CHARACTER.
    For an ASCII-only line the two coincide; a line containing any multi-byte
    character (e.g. an em-dash '—', 3 bytes in UTF-8) makes byte_col overshoot
    the character index, corrupting any downstream `line[:col]` / `line[col:]`
    slice. This walks the line encoding prefixes until the byte-length matches.
    """
    encoded = line.encode("utf-8")
    if byte_col >= len(encoded):
        return len(line)
    return len(encoded[:byte_col].decode("utf-8", errors="ignore"))


def _kwarg_line_col(call: ast.Call, field: str) -> tuple[int, int] | None:
    """Canon: §Proposal — return (lineno, col_offset) of a keyword arg's VALUE node.

    Returns None if the kwarg is not present.
    """
    for kw in call.keywords:
        if kw.arg == field:
            return (kw.value.lineno, kw.value.col_offset)
    return None


def _replace_or_insert_field(
    source_lines: list[str],
    call: ast.Call,
    field: str,
    new_value: object,
) -> list[str]:
    """Canon: §Proposal — replace or insert a keyword arg in a Conflict(...) call.

    Strategy (deterministic string replacement):
      - If the field already exists as a kwarg, use AST line/col to locate the
        start of the value token; replace the old value with the Python repr of
        new_value using a targeted line edit.
      - If the field is absent, insert it as a new kwarg line just before the
        closing ')' of the Conflict call, indented to match siblings.

    This preserves existing formatting/indentation and avoids ast.unparse
    roundtrip reformatting.
    """
    lines = list(source_lines)

    # Try to find an existing kwarg
    for kw in call.keywords:
        if kw.arg != field:
            continue
        # Found: replace the value on its line
        val_node = kw.value
        lineno = val_node.lineno - 1  # 0-indexed
        line = lines[lineno]
        # ast col_offset is a UTF-8 BYTE offset; convert to a character index
        # before slicing the (character-indexed) Python string (see
        # _byte_col_to_char_col — non-ASCII source, e.g. em-dashes, would
        # otherwise corrupt the slice).
        col = _byte_col_to_char_col(line, val_node.col_offset)

        # Find the end of the value on this line (handle simple strings and tuples)
        # We use a "find the comma or close-paren after the value" heuristic.
        # For simplicity: rebuild from col to end of line, then re-glue.
        # We need the end col of the old value — check end_lineno/end_col_offset.
        end_lineno = getattr(val_node, "end_lineno", None)
        end_col_bytes = getattr(val_node, "end_col_offset", None)
        if end_lineno is not None and end_col_bytes is not None:
            if end_lineno - 1 == lineno:
                # Single-line value: replace col..end_col
                end_col = _byte_col_to_char_col(line, end_col_bytes)
                new_repr = _python_repr(new_value)
                lines[lineno] = line[:col] + new_repr + line[end_col:]
            else:
                # Multi-line value (e.g. long string): replace from col to end
                new_repr = _python_repr(new_value)
                # Grab the suffix after the value on the end line (e.g. ",\n")
                end_line = lines[end_lineno - 1]
                end_col = _byte_col_to_char_col(end_line, end_col_bytes)
                suffix = end_line[end_col:]
                # Remove from the line after start through the end line (inclusive)
                del lines[lineno + 1 : end_lineno - 1 + 1]
                # Now recompute after deletion
                line = lines[lineno]
                lines[lineno] = line[:col] + new_repr + suffix
        return lines

    # Field not present: insert before the closing ')' of the Conflict call.
    # Find the line of the last keyword arg (or the last member of the call)
    # and insert after it.
    end_lineno = getattr(call, "end_lineno", None)
    if end_lineno is None:
        raise RuntimeError(
            f"Cannot determine end line of Conflict call for field '{field}'"
        )

    # Find indentation from an existing kwarg line
    indent = "            "  # default: 12 spaces
    if call.keywords:
        # Find the indentation of the first keyword arg
        for kw in call.keywords:
            kw_linetext = lines[kw.value.lineno - 1]
            stripped = kw_linetext.lstrip()
            indent = kw_linetext[: len(kw_linetext) - len(stripped)]
            break

    new_repr = _python_repr(new_value)
    insert_line = f"{indent}{field}={new_repr},\n"
    # Insert before the line that contains only/mostly the closing paren
    insert_at = end_lineno - 1  # 0-indexed: the '),' or ')' line
    lines.insert(insert_at, insert_line)
    return lines


def _render_relations_expr(relations: tuple[tuple[str, str], ...]) -> str:
    """Canon: §Proposal — render a relations tuple as a Relation(...) source expr.

    RULE: each (kind, target) pair becomes `Relation("kind", "target")`; the
    whole becomes a parenthesized tuple with a trailing comma. Mirrors the
    ADD-path rendering in _render_requirement_source so the UPDATE path (adding
    depends_on/supports/refines edges to an EXISTING requirement) produces
    byte-identical source, not a bare string tuple _python_repr would emit.

    WHY a dedicated renderer (not _python_repr): relations are typed edges
    (Relation constructor calls), not literals; _python_repr only knows how to
    emit str/tuple/Variant literals, so an UPDATE that adds relations would
    otherwise silently drop them (the field was absent from the UPDATE loop).
    """
    items = ", ".join(
        f'Relation("{kind}", "{target}")' for kind, target in relations
    )
    return f"({items},)"


def _replace_or_insert_relations(
    source_lines: list[str],
    call: ast.Call,
    relations: tuple[tuple[str, str], ...],
) -> list[str]:
    """Canon: §Proposal — replace/insert the relations= kwarg on a Requirement call.

    Same line/col strategy as _replace_or_insert_field, but the value is a
    Relation(...) constructor tuple (rendered by _render_relations_expr) rather
    than a _python_repr literal.
    """
    lines = list(source_lines)
    new_repr = _render_relations_expr(relations)

    for kw in call.keywords:
        if kw.arg != "relations":
            continue
        val_node = kw.value
        lineno = val_node.lineno - 1
        line = lines[lineno]
        col = _byte_col_to_char_col(line, val_node.col_offset)
        end_lineno = getattr(val_node, "end_lineno", None)
        end_col_bytes = getattr(val_node, "end_col_offset", None)
        if end_lineno is not None and end_col_bytes is not None:
            if end_lineno - 1 == lineno:
                end_col = _byte_col_to_char_col(line, end_col_bytes)
                lines[lineno] = line[:col] + new_repr + line[end_col:]
            else:
                end_line = lines[end_lineno - 1]
                end_col = _byte_col_to_char_col(end_line, end_col_bytes)
                suffix = end_line[end_col:]
                del lines[lineno + 1 : end_lineno - 1 + 1]
                line = lines[lineno]
                lines[lineno] = line[:col] + new_repr + suffix
        return lines

    # Insert before the closing ')' of the call, indented like siblings.
    end_lineno = getattr(call, "end_lineno", None)
    if end_lineno is None:
        raise RuntimeError("Cannot determine end line of Requirement call for relations")
    indent = "            "
    if call.keywords:
        kw_linetext = lines[call.keywords[0].value.lineno - 1]
        stripped = kw_linetext.lstrip()
        indent = kw_linetext[: len(kw_linetext) - len(stripped)]
    lines.insert(end_lineno - 1, f"{indent}relations={new_repr},\n")
    return lines


def _remove_kwarg_line(
    source_lines: list[str],
    call: ast.Call,
    field: str,
) -> list[str]:
    """Canon: §Proposal — remove a keyword arg's line(s) entirely from a call.

    Used when a proposal clears an optional field (e.g. m_tag) that must be
    ABSENT (not empty-valued) once its guarding condition no longer holds
    (R-m-tag-open-only: m_tag only appears on an OPEN requirement). Mirrors
    _replace_or_insert_field's line/col strategy but deletes the kwarg's
    line span instead of replacing the value, handling both single-line and
    multi-line values.
    """
    lines = list(source_lines)
    for kw in call.keywords:
        if kw.arg != field:
            continue
        val_node = kw.value
        lineno = val_node.lineno - 1  # 0-indexed, start line of the kwarg
        end_lineno = getattr(val_node, "end_lineno", lineno + 1)
        # Delete the whole line range the kwarg occupies (start..end inclusive,
        # 0-indexed) — the kwarg begins at column 0 of its own line in this
        # codebase's rendering convention (one kwarg per line).
        del lines[lineno : end_lineno]
        return lines
    return lines


def _python_repr(value: object) -> str:
    """Canon: §Proposal — produce a Python-literal repr suitable for source insertion.

    Strings → double-quoted; empty tuples → (); tuples of strings → ("a", "b");
    empty string → ""; Variant(...) payloads → a
    `Variant(id=..., behavior=..., implies=..., costs=...)` constructor call
    (mirrors how Conflict itself round-trips through source text).
    """
    from hotam_spec.conflict import Variant  # noqa: PLC0415
    from hotam_spec.signoff import Signoff  # noqa: PLC0415

    if isinstance(value, str):
        # Use double quotes, escape internal double quotes
        escaped = value.replace("\\", "\\\\").replace('"', '\\"')
        return f'"{escaped}"'
    if isinstance(value, Variant):
        return (
            "Variant("
            f"id={_python_repr(value.id)}, "
            f"behavior={_python_repr(value.behavior)}, "
            f"implies={_python_repr(value.implies)}, "
            f"costs={_python_repr(value.costs)})"
        )
    if isinstance(value, Signoff):
        parts = [
            f"decided_by={_python_repr(value.decided_by)}",
            f"date={_python_repr(value.date)}",
        ]
        if value.verbatim:
            parts.append(f"verbatim={_python_repr(value.verbatim)}")
        if value.instrument and value.instrument != "personal":
            parts.append(f"instrument={_python_repr(value.instrument)}")
        if value.chosen_variant:
            parts.append(f"chosen_variant={_python_repr(value.chosen_variant)}")
        return "Signoff(" + ", ".join(parts) + ")"
    if isinstance(value, tuple):
        if not value:
            return "()"
        items = ", ".join(_python_repr(v) for v in value)
        return f"({items},)" if len(value) == 1 else f"({items})"
    return repr(value)


# ---------------------------------------------------------------------------
# Diff rendering
# ---------------------------------------------------------------------------


def _render_diff(original: list[str], modified: list[str], label: str) -> str:
    """Canon: §Proposal — render a minimal unified-style diff between two line lists."""
    import difflib

    diff = list(
        difflib.unified_diff(
            original,
            modified,
            fromfile=f"a/{label}",
            tofile=f"b/{label}",
            lineterm="",
        )
    )
    return "\n".join(diff) if diff else "(no changes)"


# ---------------------------------------------------------------------------
# Requirement rendering
# ---------------------------------------------------------------------------


def _render_requirement_source(proposal: ProposedRequirement, indent: str) -> str:
    """Render a Requirement(...) constructor call as source text.

    Uses the same indentation style as existing entries in the content graph.
    """
    inner = indent + "    "  # one extra level for kwargs
    lines: list[str] = []
    lines.append(f"{indent}Requirement(")
    lines.append(f'{inner}id="{proposal.id}",')
    # Claim: use parenthesized string for readability
    claim_escaped = proposal.claim.replace("\\", "\\\\").replace('"', '\\"')
    lines.append(f'{inner}claim=("{claim_escaped}"),')
    lines.append(f'{inner}owner="{proposal.owner}",')
    lines.append(f'{inner}status="{proposal.status}",')
    if proposal.why:
        why_escaped = proposal.why.replace("\\", "\\\\").replace('"', '\\"')
        lines.append(f'{inner}why=("{why_escaped}"),')
    if proposal.assumptions:
        items = ", ".join(f'"{a}"' for a in proposal.assumptions)
        lines.append(f"{inner}assumptions=({items},),")
    if proposal.relations:
        rel_items = ", ".join(
            f'Relation("{kind}", "{target}")' for kind, target in proposal.relations
        )
        lines.append(f"{inner}relations=({rel_items},),")
    # enforcement: use the constant name if it matches, else string
    enf = proposal.enforcement
    if enf in ("PROSE", "STRUCTURAL", "ENFORCED"):
        lines.append(f"{inner}enforcement={enf},")
    else:
        lines.append(f'{inner}enforcement="{enf}",')
    if proposal.enforced_by:
        items = ", ".join(f'"{e}"' for e in proposal.enforced_by)
        lines.append(f"{inner}enforced_by=({items},),")
    if proposal.m_tag:
        lines.append(f'{inner}m_tag="{proposal.m_tag}",')
    if proposal.enforceability and proposal.enforceability != "ENFORCEABLE":
        lines.append(f'{inner}enforceability="{proposal.enforceability}",')
    if proposal.summary:
        summary_escaped = proposal.summary.replace("\\", "\\\\").replace('"', '\\"')
        lines.append(f'{inner}summary=("{summary_escaped}"),')
    # Timestamps: created_at always stamped at creation; settled_at when status
    # is SETTLED. The date is a fixed string written here (writer-time), NOT
    # exec-time — the graph stays deterministic on import.
    from datetime import date as _date  # noqa: PLC0415

    created = proposal.created_at or _date.today().isoformat()
    lines.append(f'{inner}created_at="{created}",')
    if proposal.status == "SETTLED":
        settled = proposal.settled_at or _date.today().isoformat()
        lines.append(f'{inner}settled_at="{settled}",')
    lines.append(f"{indent}),")
    return "\n".join(lines) + "\n"


# ---------------------------------------------------------------------------
# Apply: Requirement (add or update)
# ---------------------------------------------------------------------------


def _read_requirement_kwarg(call: ast.Call, field: str) -> object | None:
    """Canon: §Proposal — read back a Requirement kwarg's evaluated value from source.

    Returns the resolved value for the named kwarg, or None if the kwarg is
    absent. String Constants resolve to their str; tuples of string Constants
    resolve to a tuple of str; a bare Name (e.g. enforcement=PROSE) resolves to
    its identifier string ('PROSE'). Anything else resolves to None (treated as
    'unverifiable', never a false mismatch).
    """
    for kw in call.keywords:
        if kw.arg != field:
            continue
        v = kw.value
        if isinstance(v, ast.Constant) and isinstance(v.value, str):
            return v.value
        if isinstance(v, ast.Name):
            return v.id
        if isinstance(v, ast.Tuple):
            items: list[str] = []
            for elt in v.elts:
                if isinstance(elt, ast.Constant) and isinstance(elt.value, str):
                    items.append(elt.value)
                else:
                    return None  # non-string element: unverifiable
            return tuple(items)
        return None
    return None


def _verify_requirement_update_reflected(
    emitted_source: str, proposal: ProposedRequirement
) -> None:
    """Canon: §Proposal — confirm an UPDATE actually wrote its load-bearing fields.

    RULE (R-verify-closure-per-action): after the UPDATE loop emits new source,
    re-parse it, locate the target Requirement, and assert that its
    `assumptions`, `status`, `enforcement`, and `enforceability` read back
    exactly what the proposal intended. A mismatch means the writer silently
    failed to effect the change (a no-op edit, a lost line-shift, a matcher that
    resolved to the wrong node) — raise RuntimeError so the land fails LOUDLY
    instead of reporting a clean write over an unchanged graph.

    WHY only these four fields: they are the ones a signature/reattachment wave
    turns on (assumptions re-point; status/enforcement/enforceability flip).
    claim/why/relations/m_tag are verified structurally elsewhere (docs-gen,
    bijection, m-tag invariants); the silent-loss class this guard closes is the
    assumptions-reattach no-op that the Wave-2 batch exposed. enforcement/
    enforceability are only checked when the proposal carries a non-default
    value that must be present in source.
    """
    tree = ast.parse(emitted_source)
    call = _find_requirement_call(tree, proposal.id)
    if call is None:
        raise RuntimeError(
            f"Post-write verify: requirement '{proposal.id}' vanished from the "
            f"emitted source after an UPDATE — the write did not land."
        )
    # assumptions: the reattach field. Absent-in-source == empty tuple.
    got_assumptions = _read_requirement_kwarg(call, "assumptions")
    want_assumptions = proposal.assumptions
    if got_assumptions is None:
        got_assumptions = ()
    if isinstance(got_assumptions, tuple) and tuple(got_assumptions) != tuple(
        want_assumptions
    ):
        raise RuntimeError(
            f"Post-write verify FAILED for '{proposal.id}': proposal set "
            f"assumptions={list(want_assumptions)} but the emitted source reads "
            f"back assumptions={list(got_assumptions)}. The writer silently did "
            f"NOT effect the reattachment (R-verify-closure-per-action)."
        )
    # status: a bare string kwarg — must match exactly.
    got_status = _read_requirement_kwarg(call, "status")
    if got_status is not None and got_status != proposal.status:
        raise RuntimeError(
            f"Post-write verify FAILED for '{proposal.id}': proposal set "
            f"status='{proposal.status}' but the emitted source reads back "
            f"status='{got_status}' (R-verify-closure-per-action)."
        )
    # enforcement: only verify when non-default (default PROSE may be elided).
    got_enf = _read_requirement_kwarg(call, "enforcement")
    if got_enf is not None and got_enf != proposal.enforcement:
        raise RuntimeError(
            f"Post-write verify FAILED for '{proposal.id}': proposal set "
            f"enforcement='{proposal.enforcement}' but the emitted source reads "
            f"back enforcement='{got_enf}' (R-verify-closure-per-action)."
        )
    # enforceability: only verify when the proposal carries a non-default label
    # that must be present in source.
    if proposal.enforceability and proposal.enforceability != "ENFORCEABLE":
        got_enfy = _read_requirement_kwarg(call, "enforceability")
        if got_enfy != proposal.enforceability:
            raise RuntimeError(
                f"Post-write verify FAILED for '{proposal.id}': proposal set "
                f"enforceability='{proposal.enforceability}' but the emitted "
                f"source reads back enforceability='{got_enfy}' "
                f"(R-verify-closure-per-action)."
            )


def _apply_requirement_to_source(
    source_text: str, proposal: ProposedRequirement
) -> str:
    """Apply a ProposedRequirement to graph source: add new or update existing."""
    tree = ast.parse(source_text)
    existing = _find_requirement_call(tree, proposal.id)

    if existing is not None:
        # UPDATE existing requirement fields
        lines = source_text.splitlines(keepends=True)
        call_node = existing
        # When transitioning to SETTLED, stamp settled_at (writer-time date).
        # settled_at from the proposal takes precedence; empty = today.
        from datetime import date as _date  # noqa: PLC0415

        settled_stamp = proposal.settled_at
        if proposal.status == "SETTLED" and not settled_stamp:
            settled_stamp = _date.today().isoformat()
        # Detect whether status is actually changing to SETTLED (re-stamp) vs.
        # staying SETTLED (keep existing if proposal doesn't override). We
        # always write settled_at when the proposal's status is SETTLED.
        for field_name, new_value in [
            ("claim", proposal.claim),
            ("owner", proposal.owner),
            ("status", proposal.status),
            ("why", proposal.why),
            ("assumptions", proposal.assumptions),
            ("enforcement", proposal.enforcement),
            ("enforced_by", proposal.enforced_by),
            ("relations", proposal.relations),
            ("enforceability", proposal.enforceability),
            ("m_tag", proposal.m_tag),
            ("summary", proposal.summary),
            ("settled_at", settled_stamp if proposal.status == "SETTLED" else ""),
        ]:
            # Skip empty optional fields not already present
            if field_name in ("assumptions", "enforced_by", "relations") and not new_value:
                if _kwarg_line_col(call_node, field_name) is None:
                    continue
            # relations render as Relation(...) constructor calls, not bare
            # string tuples — _python_repr cannot produce them, so build the
            # source expression here and splice it via a raw-source insert path.
            if field_name == "relations" and new_value:
                lines = _replace_or_insert_relations(lines, call_node, new_value)
                new_src = "".join(lines)
                # relations reference Requirement ids and need the Relation
                # symbol; ensure it is imported before re-parsing/continuing.
                new_src = _ensure_import(
                    new_src, "hotam_spec.requirement", ["Relation"]
                )
                lines = new_src.splitlines(keepends=True)
                tree = ast.parse(new_src)
                call_node = _find_requirement_call(tree, proposal.id)
                if call_node is None:
                    raise RuntimeError(
                        f"Lost track of requirement '{proposal.id}' after "
                        f"replacing field 'relations'."
                    )
                continue
            # m_tag: if the proposal clears it (e.g. status leaves OPEN) and a
            # m_tag= kwarg is already present in source, remove the kwarg line
            # entirely rather than writing m_tag="" (R-m-tag-open-only expects
            # the field ABSENT, not empty-valued, on non-OPEN requirements).
            if field_name == "m_tag" and not new_value:
                if _kwarg_line_col(call_node, field_name) is None:
                    continue
                lines = _remove_kwarg_line(lines, call_node, field_name)
                new_src = "".join(lines)
                tree = ast.parse(new_src)
                call_node = _find_requirement_call(tree, proposal.id)
                if call_node is None:
                    raise RuntimeError(
                        f"Lost track of requirement '{proposal.id}' after "
                        f"removing field '{field_name}'."
                    )
                continue
            # summary: skip when empty and not already present (optional field).
            if field_name == "summary" and not new_value:
                if _kwarg_line_col(call_node, field_name) is None:
                    continue
            # settled_at: skip when empty (status not SETTLED) and not already
            # present. When status IS SETTLED, new_value is non-empty (stamped).
            if field_name == "settled_at" and not new_value:
                if _kwarg_line_col(call_node, field_name) is None:
                    continue
            # Skip the default enforceability value when the field isn't
            # already present in source (keep terse output for the common case).
            if (
                field_name == "enforceability"
                and new_value == "ENFORCEABLE"
                and _kwarg_line_col(call_node, field_name) is None
            ):
                continue
            lines = _replace_or_insert_field(lines, call_node, field_name, new_value)
            new_src = "".join(lines)
            tree = ast.parse(new_src)
            call_node = _find_requirement_call(tree, proposal.id)
            if call_node is None:
                raise RuntimeError(
                    f"Lost track of requirement '{proposal.id}' after "
                    f"replacing field '{field_name}'."
                )
        result = "".join(lines)
        # POST-CHECK (R-verify-closure-per-action): a mechanical writer that
        # reports success while silently NOT effecting the intended change is a
        # class of bug that bit the signature-wave-2 reattachment (a batch UPDATE
        # was reported clean while three targets' assumptions were never moved —
        # only the DEAD-fallout net later surfaced the loss). Confirm the emitted
        # source actually reflects the proposal's load-bearing fields for THIS
        # target; if not, raise (exit != 0) rather than let a no-op land quietly.
        _verify_requirement_update_reflected(result, proposal)
        return result

    # ADD new requirement at end of requirements tuple
    lines = source_text.splitlines(keepends=True)
    tree = ast.parse(source_text)
    tuple_end = _find_requirements_tuple_end(tree, lines)
    if tuple_end is None:
        raise RuntimeError(
            "Cannot find `requirements = (...)` tuple in build_graph(). "
            "Is spec/content/graph.py well-formed?"
        )

    # Determine indentation from existing Requirement calls
    indent = "        "  # default: 8 spaces
    for node in ast.walk(tree):
        if not isinstance(node, ast.Call):
            continue
        func = node.func
        if isinstance(func, ast.Name) and func.id == "Requirement":
            line_text = lines[node.lineno - 1]
            stripped = line_text.lstrip()
            indent = line_text[: len(line_text) - len(stripped)]
            break

    new_req = _render_requirement_source(proposal, indent)
    # Insert before the closing ')' of the tuple (tuple_end is 1-indexed)
    insert_at = tuple_end - 1  # 0-indexed: the ')' line
    lines.insert(insert_at, new_req)

    result = "".join(lines)

    # Ensure Relation import exists if relations are used. Match the actual
    # import statement (not any occurrence of the word "Relation", which can
    # appear in unrelated prose inside requirement claim/why strings).
    if proposal.relations:
        already_imported = _re.search(
            r"^from hotam_spec\.requirement import\b.*\bRelation\b",
            source_text,
            _re.MULTILINE,
        )
        if not already_imported:
            result = result.replace(
                "from hotam_spec.requirement import",
                "from hotam_spec.requirement import Relation,",
                1,
            )
    return result


# ---------------------------------------------------------------------------
# Apply: Rejection
# ---------------------------------------------------------------------------


def _apply_rejection_to_source(source_text: str, proposal: ProposedRejection) -> str:
    """Apply a ProposedRejection: set status to REJECTED, prepend reason to why.

    Replaces-edge materialization (R-rejected-preserved-not-decoded): when the
    proposal names one or more `replaced_by` successors, the writer appends a
    structural `replaces` Relation edge to EACH successor requirement (edge
    target = this rejected id). The edge is directed: the successor REPLACES the
    rejected node. Existing relations on the successor are preserved (append,
    never clobber). The prose "REJECTED — REPLACES" marker in `why` is written
    too — it remains the human-readable twin; the edge is the machine-traversable
    twin.
    """
    tree = ast.parse(source_text)
    call_node = _find_requirement_call(tree, proposal.requirement_id)
    if call_node is None:
        raise RuntimeError(
            f"Requirement '{proposal.requirement_id}' not found in {_CONTENT_GRAPH}."
        )

    lines = source_text.splitlines(keepends=True)

    # Set status to "REJECTED"
    lines = _replace_or_insert_field(lines, call_node, "status", "REJECTED")
    new_src = "".join(lines)
    tree = ast.parse(new_src)
    call_node = _find_requirement_call(tree, proposal.requirement_id)
    if call_node is None:
        raise RuntimeError(
            f"Lost track of requirement '{proposal.requirement_id}' after "
            f"setting status."
        )

    # Prepend rejection reason to why
    existing_why = ""
    for kw in call_node.keywords:
        if kw.arg == "why" and isinstance(kw.value, ast.Constant):
            existing_why = kw.value.value
            break

    new_why = proposal.reason
    if existing_why:
        new_why = f"{proposal.reason} — (was: {existing_why})"

    lines = _replace_or_insert_field(lines, call_node, "why", new_why)
    new_src = "".join(lines)

    # Materialize structural replaces edges on each named successor.
    for successor_id in proposal.replaced_by:
        tree = ast.parse(new_src)
        successor_call = _find_requirement_call(tree, successor_id)
        if successor_call is None:
            raise RuntimeError(
                f"replaced_by successor '{successor_id}' not found in "
                f"{_CONTENT_GRAPH} — a replaces edge can only target an existing "
                f"Requirement."
            )
        existing_rels = _extract_requirement_relations(successor_call)
        # Append the replaces edge if not already present (idempotent — re-running
        # a landed proposal must not duplicate the edge).
        new_edge = ("replaces", proposal.requirement_id)
        if new_edge in existing_rels:
            continue
        merged = existing_rels + (new_edge,)
        succ_lines = new_src.splitlines(keepends=True)
        succ_lines = _replace_or_insert_relations(succ_lines, successor_call, merged)
        new_src = "".join(succ_lines)
        # relations reference the Relation symbol; ensure it is imported.
        new_src = _ensure_import(new_src, "hotam_spec.requirement", ["Relation"])

    return new_src


# ---------------------------------------------------------------------------
# Apply: ConflictTransition
# ---------------------------------------------------------------------------


def _apply_conflict_transition(
    source_text: str, proposal: ProposedConflictTransition
) -> list[str]:
    """Apply a ConflictTransition to graph source. Returns modified lines.

    Signoff + variants-preservation (§Signoff — K2 fix):
      - When new_lifecycle starts with DECIDED or HELD and decided_by is
        non-empty, the writer builds a Signoff payload from proposal fields
        (decided_by, date, verbatim, instrument, chosen_variant) and attaches it
        as Conflict.signoff — the provenance no longer evaporates into gitignored
        JSON (R-trust-anchor-mechanism becomes auditable from the substrate).
      - When new_lifecycle does NOT start with DECIDED/HELD, the signoff is left
        untouched (an ACKNOWLEDGED transition carries no steward decision).
      - `variants` is NEVER overwritten with an empty tuple: if the proposal
        supplies no variants, the EXISTING variants in the graph are preserved
        (anti-relitigation — a HELD→DECIDED transition must not erase the
        non-chosen variants' implies/costs; K2(b) fix). This mirrors the
        shared_assumption rule: empty proposal value = leave existing untouched.
    """
    tree = ast.parse(source_text)
    call_node = _find_conflict_call(tree, proposal.conflict_id)
    if call_node is None:
        raise RuntimeError(
            f"conflict_id '{proposal.conflict_id}' not found in "
            f"{_CONTENT_GRAPH}. No changes made."
        )

    # Build the Signoff payload for DECIDED/HELD transitions with a named
    # decider. The date defaults to today (writer-time, NOT exec-time — the
    # written value is a fixed string in graph.py, so the graph stays
    # deterministic on import).
    signoff: Signoff | None = None
    decided_stamp = ""
    if (
        proposal.new_lifecycle.startswith(("DECIDED", "HELD"))
        and proposal.decided_by
    ):
        from datetime import date as _date  # noqa: PLC0415

        signoff_date = proposal.date or _date.today().isoformat()
        decided_stamp = signoff_date
        signoff = Signoff(
            decided_by=proposal.decided_by,
            date=signoff_date,
            verbatim=proposal.verbatim,
            instrument=proposal.instrument or "personal",
            chosen_variant=proposal.chosen_variant,
        )

    lines = source_text.splitlines(keepends=True)
    for field_name, new_value in [
        ("lifecycle", proposal.new_lifecycle),
        ("decided_by", proposal.decided_by),
        ("revisit_marker", proposal.revisit_marker),
        ("shared_assumption", proposal.shared_assumption),
        ("derived", proposal.derived),
        ("variants", proposal.variants),
        ("signoff", signoff),
        ("decided_at", decided_stamp),
    ]:
        if field_name == "shared_assumption" and not new_value:
            # Empty = leave the existing edge untouched (never overwrite a live
            # shared_assumption with ""). Re-pointing is opt-in per proposal.
            continue
        if field_name == "revisit_marker" and not new_value:
            existing = _kwarg_line_col(call_node, field_name)
            if existing is None:
                continue
        # K2(b) fix: variants and derived are NEVER overwritten with an empty
        # value — if the proposal supplies none, preserve the existing ones
        # (anti-relitigation: a HELD→DECIDED transition must not erase the
        # non-chosen variants' implies/costs, and a transition that spawns no
        # new derived requirements must not wipe the lineage record).
        if field_name in ("derived", "variants") and not new_value:
            continue
        # signoff=None means this transition carries no steward decision (e.g.
        # ACKNOWLEDGED) — leave any existing signoff untouched, never overwrite
        # a recorded decision with None.
        if field_name == "signoff" and new_value is None:
            continue
        # decided_at: skip when empty (not a DECIDED/HELD transition) and not
        # already present. When it IS a steward decision, decided_stamp is set.
        if field_name == "decided_at" and not new_value:
            if _kwarg_line_col(call_node, field_name) is None:
                continue
        lines = _replace_or_insert_field(lines, call_node, field_name, new_value)
        new_src = "".join(lines)
        tree = ast.parse(new_src)
        call_node = _find_conflict_call(tree, proposal.conflict_id)
        if call_node is None:
            raise RuntimeError(
                f"Lost track of conflict '{proposal.conflict_id}' after "
                f"replacing field '{field_name}'."
            )
    if proposal.variants:
        result = "".join(lines)
        result = _ensure_conflict_import(result, ["Variant"])
        lines = result.splitlines(keepends=True)
    if signoff is not None:
        result = "".join(lines)
        result = _ensure_import(result, "hotam_spec.signoff", ["Signoff"])
        lines = result.splitlines(keepends=True)
    return list(lines)


def _ensure_import(source_text: str, module: str, needed: list[str]) -> str:
    """Ensure `from {module} import ...` names every symbol in `needed`.

    RULE: parse the (single) existing `from {module} import <names>` line into
    a set of whole-token names (split on comma, stripped); append only the
    tokens in `needed` that are ABSENT from that set — a whole-name comparison,
    never a substring test (so `State` is not falsely considered present
    because `StateMachine` is imported, and `Entity` is not falsely present
    because `EntityType` is). If no import line for `module` exists yet, insert
    a fresh one immediately after `from __future__ import annotations`.

    WHY one shared helper (generalized from the old _ensure_conflict_import):
    every writer that materializes a node whose constructor needs symbols from
    a hotam_spec module (Conflict/Variant, EntityType/EntityField/EntityInstance,
    Lifecycle/State/Transition) must guarantee those symbols resolve, or the
    domain's build_graph() raises NameError on the FIRST such node. Porción-1's
    first-EntityType-in-a-domain case had to inject these imports by hand
    because the per-writer substring checks were duplicated and fragile; routing
    them all through one whole-name helper removes that class of drift.
    """
    m = _re.search(
        rf"^from {_re.escape(module)} import ([^\n]+)", source_text, _re.MULTILINE
    )
    if m:
        existing = m.group(1)
        existing_names = {n.strip() for n in existing.split(",") if n.strip()}
        missing = [n for n in needed if n not in existing_names]
        if missing:
            new_import = existing.rstrip() + ", " + ", ".join(missing)
            source_text = source_text.replace(
                f"from {module} import {existing}",
                f"from {module} import {new_import}",
                1,
            )
    else:
        source_text = source_text.replace(
            "from __future__ import annotations\n",
            "from __future__ import annotations\n\n"
            f"from {module} import {', '.join(needed)}\n",
            1,
        )
    return source_text


def _ensure_conflict_import(source_text: str, needed: list[str]) -> str:
    """Ensure `from hotam_spec.conflict import ...` names every symbol in `needed`.

    Thin wrapper over _ensure_import for the conflict module — kept as a named
    seam for the ConflictTransition writer (variants introduce `Variant`) and
    the new-Conflict writer (`Conflict`, `conflict_identity`).
    """
    return _ensure_import(source_text, "hotam_spec.conflict", needed)


def _extract_conflict_members(call: ast.Call) -> tuple[str, ...]:
    """Canon: §Proposal — read the members tuple from a Conflict(...) AST call.

    Returns the literal string members in source order. Handles both a bare
    Tuple of string Constants and a parenthesized tuple. Used by the
    ConflictMemberUpdate writer to compute (current − remove + add) without
    clobbering existing members.
    """
    for kw in call.keywords:
        if kw.arg != "members":
            continue
        val = kw.value
        elts: list[ast.expr] = []
        if isinstance(val, ast.Tuple):
            elts = list(val.elts)
        else:
            elts = [val]
        out: list[str] = []
        for elt in elts:
            if isinstance(elt, ast.Constant) and isinstance(elt.value, str):
                out.append(elt.value)
        return tuple(out)
    return ()


def _apply_conflict_member_update(
    source_text: str, proposal: ProposedConflictMemberUpdate
) -> str:
    """Apply a ProposedConflictMemberUpdate: add/remove members on an existing Conflict.

    Computes new_members = (current − remove_members) + add_members, deduped,
    order-preserving (existing order kept; new members appended). Refuses an
    update that would leave fewer than 2 DISTINCT members
    (R-conflict-min-two-members) — surfaces the invariant at write-time with a
    clear message rather than letting the graph gate fail. Steward-distinctness
    and dangling-ref invariants are re-checked graph-side after the write.
    """
    tree = ast.parse(source_text)
    call_node = _find_conflict_call(tree, proposal.conflict_id)
    if call_node is None:
        raise RuntimeError(
            f"Conflict '{proposal.conflict_id}' not found in {_CONTENT_GRAPH}. "
            f"No changes made."
        )

    current = _extract_conflict_members(call_node)
    # Remove members (those leaving the tension), preserving the rest in order.
    remove_set = set(proposal.remove_members)
    kept = tuple(m for m in current if m not in remove_set)
    # Append add_members that are not already present (dedupe; idempotent).
    existing_after_remove = set(kept)
    appended = tuple(
        m for m in proposal.add_members if m not in existing_after_remove
    )
    new_members = kept + appended

    distinct = len(set(new_members))
    if distinct < 2:
        raise RuntimeError(
            f"ConflictMemberUpdate on '{proposal.conflict_id}' would leave"
            f" {distinct} distinct member(s) ({list(new_members)});"
            f" R-conflict-min-two-members requires >= 2. Refusing to write."
        )

    lines = source_text.splitlines(keepends=True)
    lines = _replace_or_insert_field(lines, call_node, "members", new_members)
    return "".join(lines)


# ---------------------------------------------------------------------------
# Apply: Conflict (materialize a new connector node)
# ---------------------------------------------------------------------------


def _find_conflicts_tuple_end(tree: ast.AST) -> int | None:
    """Find the line number (1-indexed) of the closing ')' of `conflicts = (...)`.

    Looks for an assignment `conflicts = (...)` inside `build_graph()` and
    returns the end_lineno of the Tuple node (the line with the closing paren).
    Mirrors _find_requirements_tuple_end.
    """
    for node in ast.walk(tree):
        if not isinstance(node, ast.FunctionDef):
            continue
        if node.name != "build_graph":
            continue
        for stmt in node.body:
            if not isinstance(stmt, ast.Assign):
                continue
            for target in stmt.targets:
                if isinstance(target, ast.Name) and target.id == "conflicts":
                    return getattr(stmt.value, "end_lineno", None)
    return None


def _collect_call_kwarg_literals(tree: ast.AST, func_name: str, kwarg: str) -> set[str]:
    """Collect the set of string values of `kwarg=` across all `func_name(...)` calls.

    Only ast.Constant string values are collected (declaration sites use
    literals). Used to pre-validate a Conflict proposal against the axes
    vocabulary (Axis.slug) and the stakeholder roster (Stakeholder.id).
    """
    out: set[str] = set()
    for node in ast.walk(tree):
        if not isinstance(node, ast.Call):
            continue
        func = node.func
        name = (
            func.id
            if isinstance(func, ast.Name)
            else (func.attr if isinstance(func, ast.Attribute) else "")
        )
        if name != func_name:
            continue
        for kw in node.keywords:
            if kw.arg == kwarg and isinstance(kw.value, ast.Constant):
                if isinstance(kw.value.value, str):
                    out.add(kw.value.value)
    return out


def _requirement_owner_literal(tree: ast.AST, req_id: str) -> str | None:
    """Return the owner= string literal of the Requirement call with id `req_id`.

    None when the requirement is absent or its owner is not a plain literal.
    """
    call = _find_requirement_call(tree, req_id)
    if call is None:
        return None
    for kw in call.keywords:
        if kw.arg == "owner" and isinstance(kw.value, ast.Constant):
            if isinstance(kw.value.value, str):
                return kw.value.value
    return None


def _render_conflict_source(proposal: ProposedConflict, indent: str) -> str:
    """Render a Conflict(...) constructor call as source text.

    The id is emitted as a conflict_identity(axis, context) CALL — the source
    itself computes the identity, so the node can never drift from its tension
    (R-stable-conflict-identity; validated graph-side by
    check_conflict_id_matches_identity). Lifecycle always starts DETECTED.
    """
    inner = indent + "    "
    axis_repr = _python_repr(proposal.axis)
    ctx_repr = _python_repr(proposal.context)
    lines: list[str] = []
    lines.append(f"{indent}Conflict(")
    lines.append(f"{inner}id=conflict_identity({axis_repr}, {ctx_repr}),")
    lines.append(f"{inner}axis={axis_repr},")
    lines.append(f"{inner}context={ctx_repr},")
    lines.append(f"{inner}members={_python_repr(proposal.members)},")
    lines.append(f"{inner}steward={_python_repr(proposal.steward)},")
    lines.append(f"{inner}lifecycle={_python_repr(proposal.initial_lifecycle)},")
    if proposal.decided_by:
        lines.append(f"{inner}decided_by={_python_repr(proposal.decided_by)},")
    if proposal.shared_assumption:
        lines.append(
            f"{inner}shared_assumption={_python_repr(proposal.shared_assumption)},"
        )
    # Timestamp: created_at always stamped at creation. If initial_lifecycle is
    # DECIDED, also stamp decided_at (a conflict materialized already-DECIDED).
    from datetime import date as _date  # noqa: PLC0415

    created = _date.today().isoformat()
    lines.append(f'{inner}created_at="{created}",')
    if proposal.initial_lifecycle.startswith("DECIDED"):
        lines.append(f'{inner}decided_at="{created}",')
    lines.append(f"{indent}),")
    return "\n".join(lines) + "\n"


def _apply_conflict_to_source(source_text: str, proposal: ProposedConflict) -> str:
    """Apply a ProposedConflict: insert a new Conflict into `conflicts = (...)`.

    Pre-write validation (all failures raise RuntimeError, nothing written):
      - no Conflict with the same conflict_identity(axis, context) may exist;
      - axis MUST be a declared Axis slug (R-axis-controlled-vocab; admitting
        a new axis is out of this kind's scope);
      - every member MUST be an existing Requirement id;
      - steward MUST NOT be the owner of any member
        (R-steward-distinct-from-owners);
      - steward MUST be a declared Stakeholder id (when the source declares
        stakeholders at all).
    The graph-side invariants re-check all of this after the write (pytest
    gate); this pre-validation just fails earlier with a clearer message.
    """
    tree = ast.parse(source_text)
    new_id = conflict_identity(proposal.axis, proposal.context)

    if _find_conflict_call(tree, new_id) is not None:
        raise RuntimeError(
            f"Conflict '{new_id}' (axis={proposal.axis!r}, same normalized "
            f"context) already exists — the tension is already materialized. "
            f"Move it with a ConflictTransition proposal instead."
        )

    axis_slugs = _collect_call_kwarg_literals(tree, "Axis", "slug")
    if proposal.axis not in axis_slugs:
        raise RuntimeError(
            f"axis '{proposal.axis}' is not a declared Axis slug in the graph "
            f"(R-axis-controlled-vocab). Declared: {sorted(axis_slugs)}. "
            f"Admitting a new axis is a separate steward act, out of the "
            f"Conflict proposal's scope."
        )

    for m in proposal.members:
        owner = _requirement_owner_literal(tree, m)
        if _find_requirement_call(tree, m) is None:
            raise RuntimeError(
                f"member '{m}' is not an existing Requirement in the graph — "
                f"a Conflict may only connect existing requirements."
            )
        if owner is not None and owner == proposal.steward:
            raise RuntimeError(
                f"steward '{proposal.steward}' owns member '{m}' — the steward "
                f"must not own any member (R-steward-distinct-from-owners)."
            )

    stakeholder_ids = _collect_call_kwarg_literals(tree, "Stakeholder", "id")
    if stakeholder_ids and proposal.steward not in stakeholder_ids:
        raise RuntimeError(
            f"steward '{proposal.steward}' is not a declared Stakeholder id. "
            f"Declared: {sorted(stakeholder_ids)}."
        )

    lines = source_text.splitlines(keepends=True)
    tuple_end = _find_conflicts_tuple_end(tree)
    if tuple_end is None:
        raise RuntimeError(
            "Cannot find `conflicts = (...)` tuple in build_graph(). "
            "Is the domain graph.py well-formed?"
        )

    # Determine indentation from existing Conflict calls (default 8 spaces)
    indent = "        "
    for node in ast.walk(tree):
        if not isinstance(node, ast.Call):
            continue
        func = node.func
        if isinstance(func, ast.Name) and func.id == "Conflict":
            line_text = lines[node.lineno - 1]
            stripped = line_text.lstrip()
            indent = line_text[: len(line_text) - len(stripped)]
            break

    new_conflict = _render_conflict_source(proposal, indent)
    insert_at = tuple_end - 1  # 0-indexed: the ')' line of the tuple
    lines.insert(insert_at, new_conflict)
    result = "".join(lines)

    # Ensure Conflict + conflict_identity are imported.
    result = _ensure_conflict_import(result, ["Conflict", "conflict_identity"])
    return result


# ---------------------------------------------------------------------------
# Apply: Axis (add a new controlled-vocabulary entry)
# ---------------------------------------------------------------------------


def _find_axes_tuple_end(tree: ast.AST) -> int | None:
    """Find the line number (1-indexed) of the closing ')' of `axes = (...)`.

    Looks for an assignment `axes = (...)` inside `build_graph()` and returns
    the end_lineno of the Tuple node (the line with the closing paren).
    Mirrors _find_requirements_tuple_end / _find_conflicts_tuple_end.
    """
    for node in ast.walk(tree):
        if not isinstance(node, ast.FunctionDef):
            continue
        if node.name != "build_graph":
            continue
        for stmt in node.body:
            if not isinstance(stmt, ast.Assign):
                continue
            for target in stmt.targets:
                if isinstance(target, ast.Name) and target.id == "axes":
                    return getattr(stmt.value, "end_lineno", None)
    return None


def _collect_axis_slugs(tree: ast.AST) -> set[str]:
    """Collect the set of slug= string literals across all Axis(...) calls."""
    return _collect_call_kwarg_literals(tree, "Axis", "slug")


def _render_axis_source(proposal: ProposedAxis, indent: str) -> str:
    """Render an Axis(...) constructor call as source text."""
    inner = indent + "    "
    lines: list[str] = []
    lines.append(f"{indent}Axis(")
    lines.append(f"{inner}slug={_python_repr(proposal.slug)},")
    lines.append(f"{inner}description={_python_repr(proposal.description)},")
    lines.append(f"{indent}),")
    return "\n".join(lines) + "\n"


def _apply_axis_to_source(source_text: str, proposal: ProposedAxis) -> str:
    """Apply a ProposedAxis: insert a new Axis into `axes = (...)`.

    Pre-write validation (raises RuntimeError, nothing written):
      - no Axis with the same slug may already exist (a duplicate slug is a
        re-declaration, not a new axis — R-axis-controlled-vocab).

    WHY no fuzzy-similarity check here: the confront-style lexical similarity
    gate (R-axis-gatekeeper-policy) is the CLI's job (tools/create_axis.py),
    which runs BEFORE constructing this proposal and can be overridden with
    --force-new + a recorded rationale. This writer enforces only the
    structural invariant (exact-slug uniqueness) that the graph-side
    check_axis_in_registry family also depends on; the semantic near-duplicate
    judgment is a steward-facing decision, not a mechanical block.
    """
    tree = ast.parse(source_text)
    existing_slugs = _collect_axis_slugs(tree)
    if proposal.slug in existing_slugs:
        raise RuntimeError(
            f"Axis with slug='{proposal.slug}' already exists in the active "
            f"domain's axes tuple — a duplicate slug is a re-declaration, not "
            f"a new axis (R-axis-controlled-vocab)."
        )

    lines = source_text.splitlines(keepends=True)
    tuple_end = _find_axes_tuple_end(tree)
    if tuple_end is None:
        raise RuntimeError(
            "Cannot find `axes = (...)` tuple in build_graph(). "
            "Is the domain graph.py well-formed?"
        )

    # Determine indentation from existing Axis calls (default 8 spaces).
    indent = "        "
    for node in ast.walk(tree):
        if not isinstance(node, ast.Call):
            continue
        func = node.func
        if isinstance(func, ast.Name) and func.id == "Axis":
            line_text = lines[node.lineno - 1]
            stripped = line_text.lstrip()
            indent = line_text[: len(line_text) - len(stripped)]
            break

    new_axis = _render_axis_source(proposal, indent)
    insert_at = tuple_end - 1  # 0-indexed: the ')' line of the tuple
    lines.insert(insert_at, new_axis)
    result = "".join(lines)

    # Ensure Axis is imported.
    if not _re.search(
        r"^from hotam_spec\.axis import\b.*\bAxis\b", result, _re.MULTILINE
    ):
        m = _re.search(r"^from hotam_spec\.axis import ([^\n]+)", result, _re.MULTILINE)
        if m:
            existing = m.group(1)
            new_import = existing.rstrip() + ", Axis"
            result = result.replace(
                f"from hotam_spec.axis import {existing}",
                f"from hotam_spec.axis import {new_import}",
                1,
            )
        else:
            result = result.replace(
                "from __future__ import annotations\n",
                "from __future__ import annotations\n\nfrom hotam_spec.axis import Axis\n",
                1,
            )
    return result


def _find_stakeholders_tuple_end(tree: ast.AST) -> int | None:
    """Find the line number (1-indexed) of the closing ')' of `stakeholders = (...)`.

    Looks for an assignment `stakeholders = (...)` inside `build_graph()` and
    returns the end_lineno of the Tuple node (the line with the closing paren).
    Mirrors _find_axes_tuple_end.
    """
    for node in ast.walk(tree):
        if not isinstance(node, ast.FunctionDef):
            continue
        if node.name != "build_graph":
            continue
        for stmt in node.body:
            if not isinstance(stmt, ast.Assign):
                continue
            for target in stmt.targets:
                if isinstance(target, ast.Name) and target.id == "stakeholders":
                    return getattr(stmt.value, "end_lineno", None)
    return None


def _collect_stakeholder_ids(tree: ast.AST) -> set[str]:
    """Collect the set of id= string literals across all Stakeholder(...) calls."""
    return _collect_call_kwarg_literals(tree, "Stakeholder", "id")


def _render_stakeholder_source(proposal: ProposedStakeholder, indent: str) -> str:
    """Render a Stakeholder(...) constructor call as source text."""
    inner = indent + "    "
    lines: list[str] = []
    lines.append(f"{indent}Stakeholder(")
    lines.append(f"{inner}id={_python_repr(proposal.id)},")
    lines.append(f"{inner}name={_python_repr(proposal.name)},")
    lines.append(f"{inner}domain={_python_repr(proposal.domain)},")
    lines.append(f"{indent}),")
    return "\n".join(lines) + "\n"


def _apply_stakeholder_to_source(
    source_text: str, proposal: ProposedStakeholder
) -> str:
    """Apply a ProposedStakeholder: insert a new Stakeholder into `stakeholders = (...)`.

    Pre-write validation (raises RuntimeError, nothing written):
      - no Stakeholder with the same id may already exist (a duplicate id is a
        re-declaration, not a new party — §Stakeholder).
    """
    tree = ast.parse(source_text)
    existing_ids = _collect_stakeholder_ids(tree)
    if proposal.id in existing_ids:
        raise RuntimeError(
            f"Stakeholder with id='{proposal.id}' already exists in the active "
            f"domain's stakeholders tuple — a duplicate id is a re-declaration, "
            f"not a new stakeholder."
        )

    lines = source_text.splitlines(keepends=True)
    tuple_end = _find_stakeholders_tuple_end(tree)
    if tuple_end is None:
        raise RuntimeError(
            "Cannot find `stakeholders = (...)` tuple in build_graph(). "
            "Is the domain graph.py well-formed?"
        )

    # Determine indentation from existing Stakeholder calls (default 8 spaces).
    indent = "        "
    for node in ast.walk(tree):
        if not isinstance(node, ast.Call):
            continue
        func = node.func
        if isinstance(func, ast.Name) and func.id == "Stakeholder":
            line_text = lines[node.lineno - 1]
            stripped = line_text.lstrip()
            indent = line_text[: len(line_text) - len(stripped)]
            break

    new_stakeholder = _render_stakeholder_source(proposal, indent)
    insert_at = tuple_end - 1  # 0-indexed: the ')' line of the tuple
    lines.insert(insert_at, new_stakeholder)
    result = "".join(lines)

    # Ensure Stakeholder is imported.
    if not _re.search(
        r"^from hotam_spec\.stakeholder import\b.*\bStakeholder\b",
        result,
        _re.MULTILINE,
    ):
        result = result.replace(
            "from __future__ import annotations\n",
            "from __future__ import annotations\n\n"
            "from hotam_spec.stakeholder import Stakeholder\n",
            1,
        )
    return result


def _find_assumptions_tuple_end(tree: ast.AST) -> int | None:
    """Find the line number (1-indexed) of the closing ')' of `assumptions = (...)`.

    Looks for an assignment `assumptions = (...)` inside `build_graph()` and
    returns the end_lineno of the Tuple node (the line with the closing paren).
    Mirrors _find_axes_tuple_end.
    """
    for node in ast.walk(tree):
        if not isinstance(node, ast.FunctionDef):
            continue
        if node.name != "build_graph":
            continue
        for stmt in node.body:
            if not isinstance(stmt, ast.Assign):
                continue
            for target in stmt.targets:
                if isinstance(target, ast.Name) and target.id == "assumptions":
                    return getattr(stmt.value, "end_lineno", None)
    return None


def _collect_assumption_ids(tree: ast.AST) -> set[str]:
    """Collect the set of id= string literals across all Assumption(...) calls."""
    return _collect_call_kwarg_literals(tree, "Assumption", "id")


def _render_assumption_source(proposal: ProposedAssumption, indent: str) -> str:
    """Render an Assumption(...) constructor call as source text."""
    inner = indent + "    "
    from datetime import date as _date  # noqa: PLC0415

    created = proposal.created_at or _date.today().isoformat()
    lines: list[str] = []
    lines.append(f"{indent}Assumption(")
    lines.append(f'{inner}id={_python_repr(proposal.id)},')
    lines.append(f'{inner}statement={_python_repr(proposal.statement)},')
    lines.append(f"{inner}status={proposal.status},")
    lines.append(f'{inner}owner={_python_repr(proposal.owner)},')
    lines.append(f'{inner}created_at="{created}",')
    lines.append(f"{indent}),")
    return "\n".join(lines) + "\n"


def _apply_assumption_to_source(source_text: str, proposal: ProposedAssumption) -> str:
    """Apply a ProposedAssumption: insert a new Assumption into `assumptions = (...)`.

    Pre-write validation (raises RuntimeError, nothing written):
      - no Assumption with the same id may already exist (a duplicate id is a
        re-declaration, not a new assumption).
    """
    tree = ast.parse(source_text)
    existing_ids = _collect_assumption_ids(tree)
    if proposal.id in existing_ids:
        raise RuntimeError(
            f"Assumption with id='{proposal.id}' already exists in the active "
            f"domain's assumptions tuple — a duplicate id is a re-declaration, "
            f"not a new assumption."
        )

    lines = source_text.splitlines(keepends=True)
    tuple_end = _find_assumptions_tuple_end(tree)
    if tuple_end is None:
        raise RuntimeError(
            "Cannot find `assumptions = (...)` tuple in build_graph(). "
            "Is the domain graph.py well-formed?"
        )

    # Determine indentation from existing Assumption calls (default 8 spaces).
    indent = "        "
    for node in ast.walk(tree):
        if not isinstance(node, ast.Call):
            continue
        func = node.func
        if isinstance(func, ast.Name) and func.id == "Assumption":
            line_text = lines[node.lineno - 1]
            stripped = line_text.lstrip()
            indent = line_text[: len(line_text) - len(stripped)]
            break

    new_assumption = _render_assumption_source(proposal, indent)
    insert_at = tuple_end - 1  # 0-indexed: the ')' line of the tuple
    lines.insert(insert_at, new_assumption)
    result = "".join(lines)

    # Ensure status constant (HOLDS/UNCERTAIN/DEAD) is imported.
    if not _re.search(
        rf"^from hotam_spec\.assumption import\b.*\b{proposal.status}\b",
        result,
        _re.MULTILINE,
    ):
        m = _re.search(
            r"^from hotam_spec\.assumption import ([^\n]+)", result, _re.MULTILINE
        )
        if m:
            existing = m.group(1)
            names = [n.strip() for n in existing.split(",")]
            if proposal.status not in names:
                names.append(proposal.status)
            new_import = ", ".join(names)
            result = result.replace(
                f"from hotam_spec.assumption import {existing}",
                f"from hotam_spec.assumption import {new_import}",
                1,
            )
        else:
            result = result.replace(
                "from __future__ import annotations\n",
                "from __future__ import annotations\n\n"
                f"from hotam_spec.assumption import {proposal.status}\n",
                1,
            )
    return result


def _find_assumption_call(tree: ast.AST, assumption_id: str) -> ast.Call | None:
    """Canon: §Proposal — locate the Assumption(...) AST call whose id matches.

    Mirrors _find_requirement_call: walks the AST for ast.Call nodes named
    'Assumption', matches on the id= string-literal kwarg.
    """
    for node in ast.walk(tree):
        if not isinstance(node, ast.Call):
            continue
        func = node.func
        if isinstance(func, ast.Name) and func.id != "Assumption":
            continue
        if isinstance(func, ast.Attribute) and func.attr != "Assumption":
            continue
        for kw in node.keywords:
            if kw.arg == "id" and isinstance(kw.value, ast.Constant):
                if kw.value.value == assumption_id:
                    return node  # type: ignore[return-value]
    return None


def _apply_assumption_transition(
    source_text: str, proposal: ProposedAssumptionTransition
) -> str:
    """Apply a ProposedAssumptionTransition: UPDATE an existing Assumption's status
    and APPEND the reason to its statement. NEVER deletes the node.

    Pre-write validation (raises RuntimeError, nothing written):
      - the target Assumption id MUST already exist (a transition addresses an
        existing belief, never creates one — creation is ProposedAssumption).

    Determinism: the status= value is written as the BARE constant name
    (HOLDS/UNCERTAIN/DEAD) to match the domain graph's existing style, and the
    constant import is ensured. The reason is appended to the statement string
    with a ' — <reason>' separator so the drift history is preserved in the
    node itself (mirrors R-rejected-preserved-not-deleted for assumptions).
    """
    tree = ast.parse(source_text)
    call_node = _find_assumption_call(tree, proposal.assumption_id)
    if call_node is None:
        raise RuntimeError(
            f"Assumption with id='{proposal.assumption_id}' not found in "
            f"{_CONTENT_GRAPH}. No changes made — an AssumptionTransition "
            f"addresses an EXISTING assumption (creation is a ProposedAssumption)."
        )

    # 1. Append reason to the statement value (preserve the falsification trail).
    lines = source_text.splitlines(keepends=True)
    stmt_kw = next((kw for kw in call_node.keywords if kw.arg == "statement"), None)
    if stmt_kw is not None and isinstance(stmt_kw.value, ast.Constant):
        old_statement = stmt_kw.value.value
        new_statement = f"{old_statement} — [{proposal.new_status}] {proposal.reason}"
        lines = _replace_or_insert_field(
            lines, call_node, "statement", new_statement
        )
        # Re-parse: line numbers of the status kwarg may have shifted if the
        # statement replacement changed the line count.
        tree = ast.parse("".join(lines))
        call_node = _find_assumption_call(tree, proposal.assumption_id)
        if call_node is None:
            raise RuntimeError(
                f"Lost track of Assumption '{proposal.assumption_id}' after "
                f"appending the transition reason to its statement."
            )

    # 2. Replace the status= value with the BARE constant name (not a quoted
    #    string) to match the domain graph style. _replace_or_insert_field via
    #    _python_repr would quote it, so we do a targeted value-node edit here.
    status_kw = next((kw for kw in call_node.keywords if kw.arg == "status"), None)
    if status_kw is None:
        raise RuntimeError(
            f"Assumption '{proposal.assumption_id}' has no status= kwarg to "
            f"transition. No changes made."
        )
    val_node = status_kw.value
    lineno = val_node.lineno - 1
    line = lines[lineno]
    col = _byte_col_to_char_col(line, val_node.col_offset)
    end_col = _byte_col_to_char_col(line, val_node.end_col_offset)
    lines[lineno] = line[:col] + proposal.new_status + line[end_col:]
    result = "".join(lines)

    # 3. Ensure the status constant (HOLDS/UNCERTAIN/DEAD) is imported.
    if not _re.search(
        rf"^from hotam_spec\.assumption import\b.*\b{proposal.new_status}\b",
        result,
        _re.MULTILINE,
    ):
        m = _re.search(
            r"^from hotam_spec\.assumption import ([^\n]+)", result, _re.MULTILINE
        )
        if m:
            existing = m.group(1)
            names = [n.strip() for n in existing.split(",")]
            if proposal.new_status not in names:
                names.append(proposal.new_status)
            new_import = ", ".join(names)
            result = result.replace(
                f"from hotam_spec.assumption import {existing}",
                f"from hotam_spec.assumption import {new_import}",
                1,
            )
        else:
            result = result.replace(
                "from __future__ import annotations\n",
                "from __future__ import annotations\n\n"
                f"from hotam_spec.assumption import {proposal.new_status}\n",
                1,
            )

    # 4. Attach the Signoff payload when a human signed this transition
    #    (decided_by non-empty). The decided_by no longer evaporates into the
    #    gitignored proposal JSON — it lands in the graph as Assumption.signoff,
    #    the provenance of the LAST transition (R-trust-anchor-mechanism, K2(a)
    #    fix). The date defaults to today (writer-time, NOT exec-time — the
    #    written value is a fixed string, so the graph stays deterministic).
    #    The SAME date stamps decided_at (the timestamp field) so the reflection
    #    decay predicate can age the aspiration from the transition date.
    if proposal.decided_by:
        from datetime import date as _date  # noqa: PLC0415

        signoff_date = proposal.date or _date.today().isoformat()
        signoff = Signoff(
            decided_by=proposal.decided_by,
            date=signoff_date,
            verbatim=proposal.verbatim,
            instrument=proposal.instrument or "personal",
        )
        tree = ast.parse(result)
        call_node = _find_assumption_call(tree, proposal.assumption_id)
        if call_node is None:
            raise RuntimeError(
                f"Lost track of Assumption '{proposal.assumption_id}' before "
                f"attaching signoff."
            )
        lines = result.splitlines(keepends=True)
        lines = _replace_or_insert_field(lines, call_node, "signoff", signoff)
        lines = _replace_or_insert_field(lines, call_node, "decided_at", signoff_date)
        result = "".join(lines)
        result = _ensure_import(result, "hotam_spec.signoff", ["Signoff"])
    return result


# ---------------------------------------------------------------------------
# Apply: OperatorBudget
# ---------------------------------------------------------------------------


def _apply_operator_budget(
    source_text: str, proposal: ProposedOperatorBudget
) -> list[str]:
    """Apply an OperatorBudget proposal to graph source. Returns modified lines.

    Replaces the context_budget=ContextBudget(...) kwarg value on the matching
    Operator(...) call with a freshly-rendered ContextBudget(limit=..., measure=...)
    source expression — a raw source-text substitution (NOT _python_repr, which
    only knows literals), since the new value is itself a constructor call.
    """
    tree = ast.parse(source_text)
    call_node = _find_operator_call(tree, proposal.operator_id)
    if call_node is None:
        raise RuntimeError(
            f"operator_id '{proposal.operator_id}' not found in "
            f"{_CONTENT_GRAPH}. No changes made."
        )

    budget_kw = next(
        (kw for kw in call_node.keywords if kw.arg == "context_budget"), None
    )
    if budget_kw is None:
        raise RuntimeError(
            f"Operator '{proposal.operator_id}' has no context_budget= kwarg "
            f"to replace. No changes made."
        )

    lines = source_text.splitlines(keepends=True)
    val_node = budget_kw.value
    lineno = val_node.lineno - 1  # 0-indexed
    end_lineno = getattr(val_node, "end_lineno", val_node.lineno)
    line = lines[lineno]
    col = _byte_col_to_char_col(line, val_node.col_offset)
    new_repr = f'ContextBudget(limit={proposal.new_limit}, measure="{proposal.new_measure}")'

    if end_lineno - 1 == lineno:
        end_col = _byte_col_to_char_col(line, val_node.end_col_offset)
        lines[lineno] = line[:col] + new_repr + line[end_col:]
    else:
        end_line = lines[end_lineno - 1]
        end_col = _byte_col_to_char_col(end_line, val_node.end_col_offset)
        suffix = end_line[end_col:]
        del lines[lineno + 1 : end_lineno - 1 + 1]
        line = lines[lineno]
        lines[lineno] = line[:col] + new_repr + suffix

    return lines


# ---------------------------------------------------------------------------
# Apply: EntityType
# ---------------------------------------------------------------------------


def _find_entity_types_tuple_end(tree: ast.AST) -> int | None:
    """Find the end_lineno of the entity_types=(...) kwarg inside TensionGraph(...).

    Returns None if the kwarg is absent (caller must insert it).
    """
    for node in ast.walk(tree):
        if not isinstance(node, ast.Call):
            continue
        func = node.func
        name = (
            func.id
            if isinstance(func, ast.Name)
            else (func.attr if isinstance(func, ast.Attribute) else "")
        )
        if name != "TensionGraph":
            continue
        for kw in node.keywords:
            if kw.arg == "entity_types":
                return getattr(kw.value, "end_lineno", None)
    return None


def _find_tension_graph_call_end(tree: ast.AST) -> int | None:
    """Return the end_lineno of the TensionGraph(...) call inside build_graph()."""
    for node in ast.walk(tree):
        if not isinstance(node, ast.FunctionDef) or node.name != "build_graph":
            continue
        for stmt in node.body:
            for child in ast.walk(stmt):
                if not isinstance(child, ast.Call):
                    continue
                func = child.func
                name = (
                    func.id
                    if isinstance(func, ast.Name)
                    else (func.attr if isinstance(func, ast.Attribute) else "")
                )
                if name == "TensionGraph":
                    return getattr(child, "end_lineno", None)
    return None


def _find_tension_graph_kwarg_end(tree: ast.AST, kwarg_name: str) -> int | None:
    """Return end_lineno of a specific kwarg value inside TensionGraph(...)."""
    for node in ast.walk(tree):
        if not isinstance(node, ast.Call):
            continue
        func = node.func
        name = (
            func.id
            if isinstance(func, ast.Name)
            else (func.attr if isinstance(func, ast.Attribute) else "")
        )
        if name != "TensionGraph":
            continue
        for kw in node.keywords:
            if kw.arg == kwarg_name:
                return getattr(kw.value, "end_lineno", None)
    return None


def _render_entity_type_source(proposal: ProposedEntityType, indent: str) -> str:
    """Render an EntityType(...) constructor call as source text."""
    inner = indent + "    "
    state_inner = inner + "    "
    lines: list[str] = []
    lines.append(f"{indent}EntityType(")
    lines.append(f'{inner}slug="{proposal.slug}",')
    desc_escaped = proposal.description.replace("\\", "\\\\").replace('"', '\\"')
    lines.append(f'{inner}description="{desc_escaped}",')

    # Lifecycle
    lines.append(f"{inner}lifecycle=Lifecycle(")
    lines.append(f'{state_inner}slug="{proposal.slug}-lifecycle",')
    lines.append(f"{state_inner}states=(")
    for s_name, s_kind, s_why in proposal.states:
        if s_why:
            why_escaped = s_why.replace("\\", "\\\\").replace('"', '\\"')
            lines.append(
                f'{state_inner}    State("{s_name}", kind="{s_kind}", why="{why_escaped}"),'
            )
        else:
            lines.append(f'{state_inner}    State("{s_name}", kind="{s_kind}"),')
    lines.append(f"{state_inner}),")
    if proposal.transitions:
        lines.append(f"{state_inner}transitions=(")
        for t_src, t_dst, t_event in proposal.transitions:
            lines.append(
                f'{state_inner}    Transition("{t_src}", "{t_dst}", event="{t_event}"),'
            )
        lines.append(f"{state_inner}),")
    if proposal.cyclic:
        lines.append(f"{state_inner}cyclic=True,")
    lines.append(f"{inner}),")

    # Fields
    if proposal.fields:
        lines.append(f"{inner}fields=(")
        for f_name, f_kind, f_required, f_ref_target in proposal.fields:
            if f_ref_target:
                lines.append(
                    f'{inner}    EntityField("{f_name}", kind="{f_kind}", '
                    f'required={f_required}, ref_target="{f_ref_target}"),'
                )
            else:
                lines.append(
                    f'{inner}    EntityField("{f_name}", kind="{f_kind}", '
                    f"required={f_required}),"
                )
        lines.append(f"{inner}),")

    if proposal.why:
        why_escaped = proposal.why.replace("\\", "\\\\").replace('"', '\\"')
        lines.append(f'{inner}why="{why_escaped}",')
    lines.append(f"{indent}),")
    return "\n".join(lines) + "\n"


def _apply_entity_type_to_source(
    source_text: str,
    proposal: ProposedEntityType,
    content_graph_path: Path,
) -> str:
    """Apply a ProposedEntityType: insert a new EntityType into entity_types=(...).

    If entity_types=(...) is absent from TensionGraph(), inserts the kwarg.
    Refuses (raises RuntimeError) if an EntityType with the same slug already exists.
    """
    tree = ast.parse(source_text)

    # Check for duplicate slug
    for node in ast.walk(tree):
        if not isinstance(node, ast.Call):
            continue
        func = node.func
        name = (
            func.id
            if isinstance(func, ast.Name)
            else (func.attr if isinstance(func, ast.Attribute) else "")
        )
        if name != "EntityType":
            continue
        for kw in node.keywords:
            if kw.arg == "slug" and isinstance(kw.value, ast.Constant):
                if kw.value.value == proposal.slug:
                    raise RuntimeError(
                        f"EntityType with slug='{proposal.slug}' already exists in "
                        f"{content_graph_path}. Entity-type evolution is a separate proposal type."
                    )

    lines = source_text.splitlines(keepends=True)
    indent = "        "  # default 8 spaces

    # Check if entity_types= kwarg exists in TensionGraph(...)
    tuple_end = _find_entity_types_tuple_end(tree)

    if tuple_end is not None:
        # Insert new EntityType before the closing ')' of entity_types=(...)
        new_et = _render_entity_type_source(proposal, indent)
        lines.insert(tuple_end - 1, new_et)
    else:
        # entity_types=(...) is absent; we need to INSERT the kwarg into TensionGraph(...)
        # Strategy: find the end of TensionGraph call, then insert just before the closing ')'.
        # We insert 'entity_types=(\n    <EntityType>\n),' before any 'entities=' kwarg
        # or before the TensionGraph closing paren.
        tg_end = _find_tension_graph_call_end(tree)
        if tg_end is None:
            raise RuntimeError(
                "Cannot locate TensionGraph(...) call in build_graph(). "
                "Is the domain graph.py well-formed?"
            )

        # Try to find 'entities=' kwarg to insert before it
        entities_end = _find_tension_graph_kwarg_end(tree, "entities")
        if entities_end is not None:
            # Find the line where entities= starts (search backward from entities_end for 'entities=')
            insert_line_idx = None
            for i in range(entities_end - 1, -1, -1):
                if "entities=" in lines[i]:
                    insert_line_idx = i
                    break
            if insert_line_idx is None:
                insert_line_idx = entities_end - 1
        else:
            # Insert just before the TensionGraph closing paren
            insert_line_idx = tg_end - 1

        # Detect indentation from the surrounding kwargs
        kw_indent = "    "
        for i in range(max(0, insert_line_idx - 5), insert_line_idx + 1):
            stripped = lines[i].lstrip()
            if stripped and not stripped.startswith("#"):
                kw_indent = lines[i][: len(lines[i]) - len(stripped)]
                break

        et_rendered = _render_entity_type_source(proposal, indent)
        new_kwarg = f"{kw_indent}entity_types=(\n{et_rendered}{kw_indent}),\n"
        lines.insert(insert_line_idx, new_kwarg)

    result = "".join(lines)

    # Ensure required imports are present. The rendered EntityType always uses
    # Lifecycle + State; a transitions=(...) block uses Transition and a
    # fields=(...) block uses EntityField. Inject the full expected symbol set
    # for both modules through the shared whole-name helper (no substring
    # false-positives); redundant names already imported are left untouched.
    # This is the fix for porción-1's first-EntityType-in-a-domain case, where
    # a domain with no prior entity/lifecycle imports (or a partial import
    # missing State/Transition/EntityField) produced a build_graph() NameError.
    result = _ensure_import(
        result, "hotam_spec.lifecycle", ["Lifecycle", "State", "Transition"]
    )
    result = _ensure_import(
        result, "hotam_spec.entity", ["EntityField", "EntityType"]
    )

    return result


# ---------------------------------------------------------------------------
# LAND trace (R-land-tier-trace)
# ---------------------------------------------------------------------------


def _append_land_log(
    *,
    proposal: Proposal,
    tier: str,
    node_ids: tuple[str, ...] | str,
    pytest_ok: bool,
    closure_exit: int | None,
    runtime_dir: Path | None = None,
) -> None:
    """Canon: §Proposal — append one JSONL record of a landed proposal's verify tier.

    RULE: called ONLY after the verify step (pytest) has actually run, so the
    record states what actually ran — never a plan. Fields: stamp (iso8601,
    real wall clock — this is a runtime observability file, not generated
    substrate, so determinism is not required here, unlike gen_spec.py's
    R-deterministic-generation), kind (Proposal subtype name), target
    (proposal.target_anchor()), tier ("T1" or "T2"), node_ids (the resolved
    pytest node-id tuple for T1, or the literal string "full" for T2),
    pytest_ok (bool), closure_exit (int|None — None when --triggering-kind
    was not supplied).

    Mirrors spawn_agent.py's _append_spawn_log (R-task-spawn-log-runtime):
    same directory (.runtime/, gitignored — R-task-spawn-log-runtime), same
    append-only JSONL shape discipline.

    WHY best-effort: a broken/unwritable .runtime/ directory must never turn
    a successful, tests-green apply into a failed one — the trace is an
    observability aid, not a correctness gate. Any exception writing the log
    is caught and printed as a WARNING to stderr; apply()'s return code is
    never affected by this function.
    """
    target_dir = runtime_dir if runtime_dir is not None else _RUNTIME_DIR
    try:
        import datetime as _dt  # noqa: PLC0415

        target_dir.mkdir(parents=True, exist_ok=True)
        log_path = target_dir / _LAND_LOG_NAME
        entry = {
            "stamp": _dt.datetime.now(_dt.timezone.utc).isoformat(),
            "kind": type(proposal).__name__,
            "target": proposal.target_anchor(),
            "tier": tier,
            "node_ids": list(node_ids) if isinstance(node_ids, tuple) else node_ids,
            "pytest_ok": pytest_ok,
            "closure_exit": closure_exit,
        }
        with log_path.open("a", encoding="utf-8", newline="\n") as fh:
            fh.write(json.dumps(entry, ensure_ascii=False) + "\n")
    except Exception as exc:  # noqa: BLE001 — best-effort, never breaks apply
        print(
            f"WARNING: could not append to land-log ({exc}); continuing "
            f"(R-land-tier-trace is best-effort, never a correctness gate).",
            file=sys.stderr,
        )


def pending_proposal_files(
    proposals_dir: Path | None = None,
    *,
    auto_archive_landed: bool = True,
) -> list[Path]:
    """Canon: §Proposal — list proposal JSON files still awaiting a steward decision.

    RULE (R-presented-pending-decision-type): a file is PENDING iff it is a
    `*.json` file that lives directly under `proposals_dir` (backward-compat:
    the historical flat layout, pre-dating the pending/applied split) or under
    `proposals_dir/pending/`. Files under `proposals_dir/applied/` are landed,
    not pending, and are excluded. Sorted oldest-mtime-first (the longest-
    waiting proposal surfaces first — R-speak-by-reference: age is disclosed,
    not hidden).

    When `auto_archive_landed` is True (the default), each candidate file is
    checked: if the proposal's target anchor is already SETTLED in the active
    graph, the file is silently moved to applied/ (auto-archiving historical
    debris that was hand-landed without passing through the archive path).
    This prevents false "presented, awaiting steward" signals for proposals
    whose atoms are already in the graph.

    WHY both the flat layout and pending/ count as pending: this tool's own
    history (spec/.runtime/proposals/*.json, pre-existing this feature) never
    used a pending/ sub-folder; treating those flat files as pending (rather
    than requiring a one-time migration) is the honest reading of their actual
    state — they were presented and, as far as the graph can tell, most were
    later hand-landed without ever passing through this archiving path.
    """
    base = proposals_dir if proposals_dir is not None else _PROPOSALS_DIR
    if not base.exists():
        return []
    candidates: list[Path] = []
    for p in base.glob("*.json"):
        if p.is_file():
            candidates.append(p)
    pending_sub = base / "pending"
    if pending_sub.exists():
        for p in pending_sub.glob("*.json"):
            if p.is_file():
                candidates.append(p)

    if not auto_archive_landed or not candidates:
        candidates.sort(key=lambda p: (p.stat().st_mtime, p.name))
        return candidates

    # Load the graph once to check which proposals are already landed.
    try:
        from hotam_spec.graph import load_content_graph  # noqa: PLC0415

        g = load_content_graph()
        settled_ids: set[str] = set()
        for r in g.requirements:
            if r.status == "SETTLED":
                settled_ids.add(r.id)
        for r in g.requirements:
            settled_ids.add(r.id)  # any status = exists in graph
        for c in g.conflicts:
            settled_ids.add(c.id)
        for a in g.assumptions:
            settled_ids.add(a.id)
    except Exception:  # noqa: BLE001
        # If graph can't load, skip auto-archive and return all candidates.
        candidates.sort(key=lambda p: (p.stat().st_mtime, p.name))
        return candidates

    out: list[Path] = []
    applied_dir = base / "applied"
    for p in candidates:
        try:
            raw = json.loads(p.read_text(encoding="utf-8"))
        except Exception:  # noqa: BLE001
            out.append(p)
            continue
        # For batch files (list), check all items.
        items = raw if isinstance(raw, list) else [raw]
        all_landed = True
        for item in items:
            if not isinstance(item, dict):
                all_landed = False
                break
            target = _proposal_target_anchor(item)
            if target is None or target not in settled_ids:
                all_landed = False
                break
        if all_landed:
            # Auto-archive: move to applied/
            _archive_proposal_file(p, applied_dir=applied_dir)
        else:
            out.append(p)
    out.sort(key=lambda p: (p.stat().st_mtime, p.name))
    return out


def _proposal_target_anchor(raw: dict) -> str | None:
    """Extract the target anchor id from a raw proposal dict, or None."""
    kind = raw.get("kind", "")
    if kind == "Requirement":
        return raw.get("id", "").strip() or None
    if kind == "Rejection":
        return raw.get("requirement_id", "").strip() or None
    if kind == "ConflictTransition":
        return raw.get("conflict_id", "").strip() or None
    if kind == "Conflict":
        axis = raw.get("axis", "").strip()
        context = raw.get("context", "").strip()
        if axis and context:
            return conflict_identity(axis, context)
        return None
    if kind == "Assumption":
        return raw.get("id", "").strip() or None
    if kind == "AssumptionTransition":
        return raw.get("assumption_id", "").strip() or None
    if kind == "Stakeholder":
        return raw.get("id", "").strip() or None
    if kind == "Axis":
        return raw.get("slug", "").strip() or None
    if kind == "OperatorBudget":
        return raw.get("operator_id", "").strip() or None
    if kind == "EntityType":
        return raw.get("slug", "").strip() or None
    return None


def _archive_proposal_file(
    proposal_path: Path | None, *, applied_dir: Path | None = None
) -> Path | None:
    """Canon: §Proposal — move a landed proposal file into proposals/applied/.

    RULE (R-presented-pending-decision-type): called ONLY after apply() returns
    0 (write + regen + verify tier all green, and closure — if requested —
    advanced). Moves `proposal_path` into `applied_dir` (default
    spec/.runtime/proposals/applied/), creating the directory if needed. On a
    filename collision, a numeric suffix is appended so no landed proposal is
    ever silently overwritten.

    Returns the new path, or None if `proposal_path` is None, does not exist,
    or the move failed (best-effort — mirrors _append_land_log: a filesystem
    hiccup here must never turn a successful land into a reported failure).

    WHY move (not delete, not copy): the pending/ vs applied/ folder split IS
    the state the steward asked for ('вести в отдельной папке') — a file
    living in applied/ IS the record that it landed; nothing under proposals/
    is ever deleted (history is preserved, mirrors R-rejected-preserved-not-deleted).
    """
    if proposal_path is None:
        return None
    try:
        if not proposal_path.exists():
            return None
        target_dir = applied_dir if applied_dir is not None else _PROPOSALS_APPLIED_DIR
        target_dir.mkdir(parents=True, exist_ok=True)
        dest = target_dir / proposal_path.name
        if dest.exists() and dest != proposal_path:
            stem, suffix = proposal_path.stem, proposal_path.suffix
            n = 2
            while dest.exists():
                dest = target_dir / f"{stem}-{n}{suffix}"
                n += 1
        proposal_path.replace(dest)
        print(f"Archived proposal: {dest}")
        return dest
    except Exception as exc:  # noqa: BLE001 — best-effort, never breaks apply
        print(
            f"WARNING: could not archive proposal file ({exc}); continuing "
            f"(archiving is best-effort, never a correctness gate).",
            file=sys.stderr,
        )
        return None


# ---------------------------------------------------------------------------
# Main apply logic
# ---------------------------------------------------------------------------


def apply(
    proposal: Proposal,
    *,
    dry_run: bool = False,
    triggering_kind: str | None = None,
    content_graph: Path | None = None,
    full_suite: bool = False,
    proposal_file: Path | None = None,
) -> int:
    """Canon: §Proposal — apply a validated Proposal to the graph.

    Dispatches to the appropriate handler based on proposal type:
      - ProposedConflictTransition: locate Conflict node, replace fields
      - ProposedConflict: insert a new Conflict node (lifecycle DETECTED)
      - ProposedRequirement: add new or update existing Requirement
      - ProposedRejection: set status to REJECTED, prepend reason
      - ProposedEntityType: insert new EntityType into entity_types=()
      - ProposedOperatorBudget: locate Operator node, replace context_budget=
      - ProposedAxis: insert a new Axis into the active domain's axes=() tuple

    Steps:
      1. Read spec/content/graph.py.
      2. Apply the proposal (type-dispatched).
      3. If dry_run: print diff and return 0 without writing.
      4. Write the file, run gen_spec.py, run the LAND-gate verify tier.
      5. If triggering_kind supplied: run closure.check_closure; return 2 if not advanced.
      6. Return 0 on success, 1 on any non-closure failure.

    `triggering_kind` is the band/kind string of the original action this proposal
    was meant to close (e.g. "CONFLICT_STALLED", "OPEN_ITEM", "STRUCTURE",
    "DRIFT_FALLOUT"). If None, the closure check is skipped (backward-compat for
    P3 unit tests that do not supply a triggering kind).

    `proposal_file`, if supplied, is the path to the JSON proposal file that was
    read for this call; on a successful land (return 0) it is moved into
    proposals/applied/ (R-presented-pending-decision-type). None (the default)
    skips archiving — used by unit tests that construct a Proposal in-process
    without a backing file.

    `full_suite` forces the T2 (full pytest) verify tier even when the T1
    targeted-enforcer selector (tools/gate.py) would otherwise be confident.
    R-land-gate-tier-selector: when False (the default), apply() asks
    gate.select_tier1(proposal.target_anchor()) for a targeted node-id subset;
    if the selector is confident, ONLY that subset (+ ALWAYS_RUN) is run. Any
    selector uncertainty (new node, empty/unresolvable enforced_by, Conflict
    target) falls back to the full suite automatically — the gate never
    silently narrows verification below full coverage; it only ever narrows
    when it can name the exact enforcers a full run would have exercised for
    this target anyway. R-tiered-gate-not-a-commit-gate: this tiering applies
    ONLY to the per-proposal LAND step; wave/commit boundaries remain governed
    by the unabridged full suite (`uv run pytest -q` from spec/), never T1.
    """
    active_graph = content_graph if content_graph is not None else _CONTENT_GRAPH
    source_text = active_graph.read_text(encoding="utf-8")
    original_lines = source_text.splitlines(keepends=True)

    # R-land-gate-tier-selector-fails-closed: determine, from the PRE-write
    # source, whether this proposal's target node already existed. A
    # ProposedConflict always creates (Conflict has no update path — see
    # _apply_conflict_to_source, which errors if the id already exists); a
    # ProposedRequirement may ADD (new node, target_preexisting=False) or
    # UPDATE (existing node, target_preexisting=True). This flag — not the
    # accidental emptiness of a fresh node's enforced_by tuple — is what
    # drives the T1/T2 tier decision below (fail closed on any creation).
    target_preexisting = True
    if isinstance(proposal, (ProposedConflict, ProposedAxis, ProposedAssumption)):
        target_preexisting = False
    elif isinstance(proposal, ProposedAssumptionTransition):
        # An assumption status change (esp. → DEAD) has cluster-wide fallout the
        # assumption's own enforced_by cannot bound (assumptions carry none) —
        # fail closed to the T2 full suite (R-land-gate-tier-selector-fails-closed).
        target_preexisting = False
    elif isinstance(proposal, ProposedConflictMemberUpdate):
        # Member churn changes which requirements the conflict connects; the
        # fallout (dangling refs, steward-distinctness) is not bounded by the
        # conflict's own enforced_by — fail closed to the T2 full suite.
        target_preexisting = False
    elif isinstance(proposal, ProposedRequirement):
        pre_tree = ast.parse(source_text)
        target_preexisting = _find_requirement_call(pre_tree, proposal.id) is not None

    try:
        if isinstance(proposal, ProposedConflictTransition):
            lines = _apply_conflict_transition(source_text, proposal)
        elif isinstance(proposal, ProposedConflict):
            new_source = _apply_conflict_to_source(source_text, proposal)
            lines = new_source.splitlines(keepends=True)
        elif isinstance(proposal, ProposedRequirement):
            new_source = _apply_requirement_to_source(source_text, proposal)
            lines = new_source.splitlines(keepends=True)
        elif isinstance(proposal, ProposedRejection):
            new_source = _apply_rejection_to_source(source_text, proposal)
            lines = new_source.splitlines(keepends=True)
        elif isinstance(proposal, ProposedEntityType):
            new_source = _apply_entity_type_to_source(
                source_text, proposal, active_graph
            )
            lines = new_source.splitlines(keepends=True)
        elif isinstance(proposal, ProposedOperatorBudget):
            lines = _apply_operator_budget(source_text, proposal)
        elif isinstance(proposal, ProposedAxis):
            new_source = _apply_axis_to_source(source_text, proposal)
            lines = new_source.splitlines(keepends=True)
        elif isinstance(proposal, ProposedAssumption):
            new_source = _apply_assumption_to_source(source_text, proposal)
            lines = new_source.splitlines(keepends=True)
        elif isinstance(proposal, ProposedStakeholder):
            new_source = _apply_stakeholder_to_source(source_text, proposal)
            lines = new_source.splitlines(keepends=True)
        elif isinstance(proposal, ProposedAssumptionTransition):
            new_source = _apply_assumption_transition(source_text, proposal)
            lines = new_source.splitlines(keepends=True)
        elif isinstance(proposal, ProposedConflictMemberUpdate):
            new_source = _apply_conflict_member_update(source_text, proposal)
            lines = new_source.splitlines(keepends=True)
        else:
            print(
                f"ERROR: unhandled proposal type {type(proposal).__name__}",
                file=sys.stderr,
            )
            return 1
    except RuntimeError as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        return 1

    if dry_run:
        diff = _render_diff(original_lines, lines, active_graph.name)
        print("=== DRY RUN — proposed diff ===")
        print(diff)
        print("=== (no file written) ===")
        if triggering_kind is not None:
            # In dry-run mode, we cannot truly check closure (no file was written),
            # but we emit the section so callers can verify the flag is wired.
            print(
                f"\n=== CLOSURE CHECK (dry-run, not authoritative) ===\n"
                f"  triggering_kind : {triggering_kind}\n"
                f"  target          : {proposal.target_anchor()}\n"
                f"  note            : (dry-run — run without --dry-run for real closure check)\n"
                f"=== END CLOSURE CHECK ==="
            )
        return 0

    # Write
    new_source = "".join(lines)
    active_graph.write_text(new_source, encoding="utf-8")
    print(f"Written: {active_graph}")

    # Regen — contamination-safe (R-root-crystal-follows-pin).
    #
    # The applied graph (active_graph) may belong to a NON-pinned domain when
    # the operator lands a hotam-dev proposal via HOTAM_SPEC_ACTIVE_DOMAIN=
    # hotam-dev. gen_spec.py resolves the active domain from that same env var,
    # so a single naive `gen_spec` subprocess (inheriting our env) would
    # regenerate the ROOT CLAUDE.md — the resident operator crystal — from the
    # transiently-active domain, overwriting "Operator of hotam-spec-self
    # (N SETTLED)" with "Operator of hotam-dev (7 SETTLED)". That is real
    # contamination of the committed self-host crystal.
    #
    # Fix: decouple the two writes.
    #   Pass 1 (--docs-only, env = applied domain): refresh the APPLIED
    #           domain's docs/gen/ only. Never touches root CLAUDE.md.
    #   Pass 2 (env STRIPPED of HOTAM_SPEC_ACTIVE_DOMAIN): the root crystal
    #           regenerates from the PIN (domains/.active-domain) — always the
    #           self-host domain — plus that domain's docs/gen/.
    # When the applied domain IS the pinned domain (the common self-host case),
    # both passes target the same domain and the result is identical to the old
    # single pass; the extra pass is cheap and keeps ONE code path.
    applied_domain = active_graph.parent.name if active_graph.parent.name else ""
    pinned_domain = ""
    if _ACTIVE_DOMAIN_PIN_FILE.exists():
        pinned_domain = _ACTIVE_DOMAIN_PIN_FILE.read_text(encoding="utf-8").strip()

    # Pass 1: applied domain's docs only (env carried through as-is).
    docs_env = dict(_os.environ)
    if applied_domain:
        docs_env["HOTAM_SPEC_ACTIVE_DOMAIN"] = applied_domain
    regen_docs = subprocess.run(
        [sys.executable, str(_GEN_SPEC), "--docs-only"],
        capture_output=True,
        text=True,
        env=docs_env,
    )
    if regen_docs.returncode != 0:
        print("ERROR: gen_spec.py --docs-only failed:", file=sys.stderr)
        print(regen_docs.stderr, file=sys.stderr)
        return 1

    # Pass 2: root crystal from the pin (env var stripped so the pin wins).
    root_env = dict(_os.environ)
    root_env.pop("HOTAM_SPEC_ACTIVE_DOMAIN", None)
    regen_result = subprocess.run(
        [sys.executable, str(_GEN_SPEC)],
        capture_output=True,
        text=True,
        env=root_env,
    )
    if regen_result.returncode != 0:
        print("ERROR: gen_spec.py failed:", file=sys.stderr)
        print(regen_result.stderr, file=sys.stderr)
        return 1
    print(
        f"gen_spec.py: OK (docs pass: {applied_domain or 'default'}; "
        f"root crystal from pin: {pinned_domain or 'alphabetical fallback'})"
    )

    # Verify — T1 targeted-enforcer gate by default, T2 full suite on --full,
    # on a ProposedRejection (removal blast radius is not bounded by the
    # rejected atom's own enforced_by — R-land-gate-tier-selector-fails-closed),
    # on a node CREATION (new Requirement or new Conflict — a brand-new node's
    # blast radius cannot be known from its own enforced_by, which is either
    # absent or steward-supplied and unverified), or whenever the gate fails
    # closed (R-land-gate-tier-selector).
    pytest_args = [sys.executable, "-m", "pytest", "-q"]
    gate_tier = "T2 (full suite, --full requested)"
    land_tier = "T2"
    land_node_ids: tuple[str, ...] | str = "full"
    if full_suite:
        pytest_args.append(str(_SPEC_ROOT / "tests"))
    elif isinstance(proposal, ProposedRejection):
        gate_tier = (
            "T2 (full suite, ProposedRejection: a rejected atom's own "
            "enforced_by does not bound its removal blast radius)"
        )
        pytest_args.append(str(_SPEC_ROOT / "tests"))
    elif not target_preexisting:
        gate_tier = (
            "T2 (full suite, new node creation: no pre-existing enforced_by "
            "can be trusted to bound a brand-new node's blast radius)"
        )
        pytest_args.append(str(_SPEC_ROOT / "tests"))
    else:
        import gate as _gate  # noqa: PLC0415  (lives in tools/, not a package)

        gate_result = _gate.select_tier1(proposal.target_anchor())
        if gate_result.confident:
            gate_tier = f"T1 (targeted, {len(gate_result.node_ids)} node-id(s))"
            pytest_args.extend(gate_result.node_ids)
            land_tier = "T1"
            land_node_ids = tuple(gate_result.node_ids)
        else:
            gate_tier = f"T2 (full suite, gate fell back closed: {gate_result.reason})"
            pytest_args.append(str(_SPEC_ROOT / "tests"))
    print(f"verify tier: {gate_tier}")

    pytest_result = subprocess.run(
        pytest_args,
        capture_output=True,
        text=True,
        cwd=str(_SPEC_ROOT),
    )
    print(pytest_result.stdout)
    pytest_ok = pytest_result.returncode == 0
    if not pytest_ok:
        print(
            "ERROR: pytest failed after apply. File written but tests are red.",
            file=sys.stderr,
        )
        print(pytest_result.stderr, file=sys.stderr)
        print(
            "NOTE: auto-revert is not implemented in P3. "
            "Inspect the diff and revert manually if needed.",
            file=sys.stderr,
        )
        _append_land_log(
            proposal=proposal,
            tier=land_tier,
            node_ids=land_node_ids,
            pytest_ok=False,
            closure_exit=None,
        )
        return 1

    # Closure check (P4 feedback edge) — only when --triggering-kind is supplied.
    closure_exit: int | None = None
    if triggering_kind is not None:
        import closure  # noqa: PLC0415  (lives in tools/, not a package)

        result = closure.check_closure(proposal, triggering_kind)
        print(
            f"\n=== CLOSURE CHECK ===\n"
            f"  advanced        : {result.advanced}\n"
            f"  target          : {result.target}\n"
            f"  triggering_kind : {result.triggering_kind}\n"
            f"  still_open      : {result.still_open_count}\n"
            f"  note            : {result.note}\n"
            f"=== END CLOSURE CHECK ==="
        )
        if not result.advanced:
            closure_exit = 2
            print(
                "\nERROR: closure FAILED — the triggering action is STILL in the "
                "post-apply diagnosis. The write landed (tests green) but the "
                "action did NOT advance. Investigate before marking closed.",
                file=sys.stderr,
            )
            _append_land_log(
                proposal=proposal,
                tier=land_tier,
                node_ids=land_node_ids,
                pytest_ok=True,
                closure_exit=closure_exit,
            )
            return 2
        closure_exit = 0

    _append_land_log(
        proposal=proposal,
        tier=land_tier,
        node_ids=land_node_ids,
        pytest_ok=True,
        closure_exit=closure_exit,
    )

    summary_target = proposal.target_anchor()
    summary_kind = type(proposal).__name__
    print(
        f"\nSUMMARY:\n"
        f"  kind   : {summary_kind}\n"
        f"  target : {summary_target}\n"
        f"  tests  : GREEN"
    )
    if isinstance(proposal, ProposedConflictTransition):
        print(
            f"  decided_by   : {proposal.decided_by or '(none)'}\n"
            f"  new_lifecycle: {proposal.new_lifecycle}"
        )
    if triggering_kind is not None:
        print(f"  closure: ADVANCED (action {summary_target!r} closed)")
    _archive_proposal_file(proposal_file)
    return 0


# ---------------------------------------------------------------------------
# Entry point
# ---------------------------------------------------------------------------


def main(argv: list[str] | None = None) -> int:
    """Canon: §Proposal — CLI entry point for apply_proposal.py.

    Exit codes:
      0 — success (write landed, tests green, closure confirmed if --triggering-kind).
      1 — failure (validation, missing id, or pytest red).
      2 — not advanced (write+tests green, but triggering action STILL in diagnosis).
    """
    if hasattr(sys.stdout, "reconfigure"):
        # The graph prose is UTF-8 (em-dashes, arrows); a redirected Windows
        # stdout defaults to cp1252 and would crash the dry-run diff print.
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    parser = argparse.ArgumentParser(
        description=(
            "Mechanically apply a steward-approved JSON proposal to spec/content/graph.py. "
            "Optionally verify P4 closure: that the triggering diagnosis was removed."
        )
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Print the proposed diff without writing any files.",
    )
    parser.add_argument(
        "--triggering-kind",
        metavar="KIND",
        default=None,
        help=(
            "The what_now band/kind of the action this proposal was meant to close "
            "(e.g. CONFLICT_STALLED, OPEN_ITEM, STRUCTURE, DRIFT_FALLOUT). "
            "When supplied, runs closure.check_closure after pytest passes. "
            "If the action still appears in the post-apply diagnosis, exit code 2 "
            "is returned (distinguishable from pytest failures which exit 1). "
            "Omit to skip the closure check (backward-compatible with P3 tests)."
        ),
    )
    parser.add_argument(
        "--batch",
        action="store_true",
        help=(
            "Treat the JSON file as an array of proposals and apply them "
            "sequentially. Critical for atomized wave proposals."
        ),
    )
    parser.add_argument(
        "--full",
        action="store_true",
        help=(
            "Force the T2 full pytest suite as the LAND verify step, even when "
            "tools/gate.py's T1 targeted-enforcer selector would be confident. "
            "T1 is the default (R-land-gate-tier-selector); the selector itself "
            "falls back to full-suite automatically on any uncertainty, so "
            "--full is for explicit wave/commit-boundary or steward-requested runs."
        ),
    )
    parser.add_argument(
        "proposal_file",
        help="Path to the steward-approved JSON proposal file.",
    )
    args = parser.parse_args(argv)

    proposal_path = Path(args.proposal_file)
    if not proposal_path.exists():
        print(f"ERROR: proposal file not found: {proposal_path}", file=sys.stderr)
        return 1

    try:
        raw = json.loads(proposal_path.read_text(encoding="utf-8"))
    except json.JSONDecodeError as exc:
        print(f"ERROR: invalid JSON in {proposal_path}: {exc}", file=sys.stderr)
        return 1

    if args.batch:
        if not isinstance(raw, list):
            print(
                "ERROR: --batch expects a JSON array of proposals.",
                file=sys.stderr,
            )
            return 1
        for i, item in enumerate(raw):
            print(f"\n--- Proposal {i + 1}/{len(raw)} ---")
            try:
                proposal = _validate_proposal(item)
            except ValueError as exc:
                print(f"ERROR: invalid proposal #{i + 1}: {exc}", file=sys.stderr)
                return 1
            rc = apply(
                proposal,
                dry_run=args.dry_run,
                triggering_kind=args.triggering_kind,
                full_suite=args.full,
                # A --batch file holds MANY proposals; the file itself is not
                # 1:1 with any single one, so it is not archived per-item.
                proposal_file=None,
            )
            if rc != 0:
                print(
                    f"ERROR: proposal #{i + 1} failed with exit code {rc}. "
                    f"Stopping batch.",
                    file=sys.stderr,
                )
                return rc
        # All batch items landed — archive the batch file itself.
        if not args.dry_run:
            _archive_proposal_file(proposal_path)
        return 0

    try:
        proposal = _validate_proposal(raw)
    except ValueError as exc:
        print(f"ERROR: invalid proposal: {exc}", file=sys.stderr)
        return 1

    return apply(
        proposal,
        dry_run=args.dry_run,
        triggering_kind=args.triggering_kind,
        full_suite=args.full,
        proposal_file=None if args.dry_run else proposal_path,
    )


if __name__ == "__main__":
    sys.exit(main())
