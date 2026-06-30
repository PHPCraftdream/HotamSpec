"""Canon: §Requirement — a business requirement as a node in the tension graph.

A Requirement is a claim the system shall satisfy, written machine-checkable
where possible and otherwise EARS-style ("the system shall ..."). It is NOT a
truth: it changes, it contradicts its siblings, and it rests on assumptions that
can die. The requirement carries everything needed to detect the three
invisibilities AROUND it — but the contradiction itself never lives here; it
lives on the Conflict connector node (see §Conflict).

WHY relations are typed tuple-of-id fields (not a generic graph): `supports`,
`refines`, `depends_on` are the SUPPORTIVE structure — the non-adversarial edges.
A contradiction is deliberately NOT among them: you cannot express a conflict as
a Requirement field, because a conflict belongs to neither requirement. This is
the structural enforcement of "conflict is a node, not an edge" — the ontology
makes the naive `conflicts_with` edge unwritable.

WHY assumptions live on the requirement: hidden dependencies (invisibility #2)
are "A relies on an assumption B negates" — only visible if A enumerates its
assumptions so a chain can be walked (graph.requirements_on_assumption). Context
drift (invisibility #3) fires when one of those assumptions flips to DEAD.

Lifecycle (source of truth is `status`, params.py-style):
  DRAFT          — proposed, not yet accepted into the canon.
  SETTLED        — accepted and currently held.
  OPEN(question) — accepted-with-a-hole; the question is normative and MUST be
                   non-empty (invariants.check_open_has_question). Surfaced by
                   the harness and listed in OPEN.md.
  REJECTED       — withdrawn; kept for history (anti-relitigation), not deleted.

Enforcement gradient (R-enforcement-gradient / R-requirement-enforced):
  PROSE      — the requirement is recorded only; no structural or automated check
               enforces it. The promise is held by human discipline alone.
  STRUCTURAL — the requirement is visible and addressable (surfaced by the
               harness, listed in docs) but no check_* invariant or test fires
               automatically on violation. Context-debt: cheaper than PROSE but
               not a reflex.
  ENFORCED   — an invariant (check_*) or test fires on violation; the guarantee
               is automatic. The `enforced_by` field MUST name the enforcer(s)
               so the guarantee is auditable. This is a crystallized reflex: the
               system actively rejects the violation, not just records it.

  The direction of progress is PROSE -> STRUCTURAL -> ENFORCED. UNENFORCED
  (PROSE + STRUCTURAL on SETTLED requirements) is the burn-down meter: it is
  the gap between what is claimed and what is guaranteed.
"""

from __future__ import annotations

from dataclasses import dataclass, field

DRAFT = "DRAFT"
SETTLED = "SETTLED"
REJECTED = "REJECTED"
OPEN_PREFIX = "OPEN"  # status string begins "OPEN(" for an open requirement

PROSE = "PROSE"
STRUCTURAL = "STRUCTURAL"
ENFORCED = "ENFORCED"
ENFORCEMENT_LEVELS: frozenset[str] = frozenset({PROSE, STRUCTURAL, ENFORCED})


@dataclass(frozen=True)
class Relation:
    """Canon: §Requirement — one typed SUPPORTIVE edge to another Requirement.

    RULE: `kind` is one of the supportive relation kinds (supports | refines |
    depends_on); `target` MUST be the id of a Requirement in the graph
    (invariants.check_no_dangling_ids). Conflict is deliberately NOT a relation
    kind — see module docstring.

    WHY depends_on is supportive, not adversarial: it carries invisibility #2 —
    a depends_on chain can lead to an assumption a different requirement negates;
    that latent contradiction is then materialized as a Conflict node, it is not
    this edge.
    """

    kind: str  # "supports" | "refines" | "depends_on"
    target: str


#: The admitted supportive relation kinds (authority for the form invariant).
RELATION_KINDS: frozenset[str] = frozenset({"supports", "refines", "depends_on"})


@dataclass(frozen=True)
class Requirement:
    """Canon: §Requirement — a single requirement node.

    RULE: `status` is DRAFT | SETTLED | REJECTED | "OPEN(<question>)"; an OPEN
    status MUST carry a non-empty question (invariants.check_open_has_question).
    `owner` MUST be a Stakeholder id; every assumption id and every relation
    target MUST resolve in the graph (invariants.check_no_dangling_ids).

    RULE (R-enforcement-gradient / R-requirement-enforced): `enforcement` MUST be
    one of ENFORCEMENT_LEVELS (PROSE | STRUCTURAL | ENFORCED); when `enforcement`
    is ENFORCED, `enforced_by` MUST be a non-empty tuple naming the check_* or
    test that fires on violation (invariants.check_enforced_names_invariant). A
    SETTLED requirement that is not ENFORCED is UNENFORCED — claimed-but-not-
    guaranteed, the burn-down meter measures this gap.

    RULE (m_tag): an OPEN requirement that mirrors a CLAUDE.md M-decision MUST
    carry its M-tag (e.g. `m_tag="M17"`). Non-OPEN requirements may leave it
    empty (the default "").

    WHY (m_tag): this field lets `docs/gen/DECISIONS.md` be generated as the
    canonical home of the M-registry — retiring the hand-maintained M-table in
    CLAUDE.md (the U5 anti-drift fix; the dev-coin Param.status + HOLES.md
    pattern: one source of truth, generated mirror). Format enforced by
    `invariants.check_m_tag_format`.

    Fields:
      id           — stable slug (e.g. "R-87"); the value edges and Conflicts carry.
      claim        — the requirement, machine-checkable predicate or EARS prose.
      assumptions  — tuple of Assumption ids this claim rests on.
      owner        — Stakeholder id that defends this claim.
      relations    — tuple of typed SUPPORTIVE Relations to other Requirements.
      status       — DRAFT | SETTLED | REJECTED | OPEN(question) (source of truth).
      why          — rationale / EARS context (anti-relitigation prose).
      enforcement  — PROSE | STRUCTURAL | ENFORCED (default: PROSE).
      enforced_by  — tuple of check_*/test anchors; MUST be non-empty when
                     enforcement == ENFORCED.
      m_tag        — M-decision tag (e.g. "M17"); non-empty only on OPEN
                     requirements that mirror a CLAUDE.md M-decision.

    WHY frozen + id-stable: a requirement may be renamed, split, or refined; the
    Conflict node that mediates it has identity from (axis, context), so it
    SURVIVES such churn — only its member ids update (see §Conflict identity).
    """

    id: str
    claim: str
    owner: str
    status: str
    why: str = ""
    assumptions: tuple[str, ...] = field(default_factory=tuple)
    relations: tuple[Relation, ...] = field(default_factory=tuple)
    enforcement: str = PROSE
    enforced_by: tuple[str, ...] = field(default_factory=tuple)
    m_tag: str = ""

    def is_open(self) -> bool:
        """Canon: §Requirement — True iff this requirement is an OPEN hole.

        RULE: open iff status starts with "OPEN". Open requirements are surfaced
        by the harness and listed in the generated OPEN.md.

        WHY a method (not status == ...): the open marker carries its question
        inline ("OPEN(which segment?)"); membership is a prefix test, kept here so
        the harness, the generator and the invariant agree.
        """
        return self.status.startswith(OPEN_PREFIX)
