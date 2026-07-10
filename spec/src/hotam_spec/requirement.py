"""Canon: §Requirement — a business requirement as a node in the tension graph.

A Requirement is a claim the system shall satisfy, written machine-checkable
where possible and otherwise EARS-style ("the system shall ..."). It is NOT a
truth: it changes, it contradicts its siblings, and it rests on assumptions that
can die. The requirement carries everything needed to detect the three
invisibilities AROUND it — but the contradiction itself never lives here; it
lives on the Conflict connector node (see §Conflict).

WHY relations are typed tuple-of-id fields (not a generic graph): `refines` and
`depends_on` are the SUPPORTIVE structure — the non-adversarial edges. (D2,
2026-07-10: `supports` was merged into `refines` — no check_* invariant ever
differentiated the two kinds semantically, so carrying both was an
undifferentiated distinction; every existing `supports` edge was migrated to
`refines` via a batch ProposedRequirement UPDATE, R-no-hand-edit-graph.)
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

ENFORCEABLE = "ENFORCEABLE"
INHERENTLY_PROSE = "INHERENTLY_PROSE"
ENFORCEABILITY_KINDS: frozenset[str] = frozenset({ENFORCEABLE, INHERENTLY_PROSE})


@dataclass(frozen=True)
class Relation:
    """Canon: §Requirement — one typed SUPPORTIVE edge to another Requirement.

    RULE: `kind` is one of the relation kinds (refines | depends_on |
    replaces); `target` MUST be the id of a Requirement in the graph
    (invariants.check_no_dangling_ids). Conflict is deliberately NOT a
    relation kind — see module docstring. `replaces` is directed: the carrier
    (the Requirement whose `relations` carries the edge) REPLACES the `target`
    (a REJECTED requirement it supersedes) — anti-relitigation as structure.

    WHY depends_on is supportive, not adversarial: it carries invisibility #2 —
    a depends_on chain can lead to an assumption a different requirement negates;
    that latent contradiction is then materialized as a Conflict node, it is not
    this edge.

    WHY no separate `supports` kind (D2, 2026-07-10): no check_* invariant ever
    branched on `kind == "supports"` vs `kind == "refines"` — the two were
    structurally identical (both admitted, both dangling-target-checked the
    same way), so the distinction was undifferentiated vocabulary, not
    semantics. Merged into `refines` (the more frequently used of the two)
    by steward decision; every prior `supports` edge in the graph was
    migrated via a batch ProposedRequirement UPDATE (R-no-hand-edit-graph).
    """

    kind: str  # "refines" | "depends_on" | "replaces"
    target: str


#: The admitted relation kinds (authority for the form invariant). `refines`
#: and `depends_on` are SUPPORTIVE edges (non-adversarial structure);
#: `replaces` is the ANTI-RELITIGATION edge — a directed "this requirement
#: supersedes that one" link, materialized structurally so the
#: REJECTED↔SETTLED replacement relation is graph-traversable, not
#: prose-only (R-rejected-preserved-not-deleted). (D2, 2026-07-10: `supports`
#: merged into `refines` — see the Relation docstring WHY.)
RELATION_KINDS: frozenset[str] = frozenset({"refines", "depends_on", "replaces"})


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
      enforceability — ENFORCEABLE | INHERENTLY_PROSE (default: ENFORCEABLE).
                     ENFORCEABLE means a check_* or test COULD exist (real
                     closeable debt when enforcement is PROSE/STRUCTURAL).
                     INHERENTLY_PROSE means the claim is a disposition or
                     judgment call no check_* could ever verify — permanent
                     discipline, not debt (R-enforceability-kind-declared).

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
    enforceability: str = ENFORCEABLE
    summary: str = ""
    created_at: str = ""
    # ^ ISO YYYY-MM-DD of node CREATION; "" = unknown (legacy nodes predating
    # the timestamp layer have no honest creation date — do NOT fabricate one).
    # Stamped by apply_proposal.py at first materialization, never at exec-time.
    settled_at: str = ""
    # ^ ISO YYYY-MM-DD of the LAST transition into SETTLED; "" = unknown. A
    # requirement re-entering SETTLED from OPEN/REJECTED re-stamps this field.
    # Surfaces 'settled N days ago, not re-confronted' without a separate mechanism.

    def is_closeable_debt(self) -> bool:
        """Canon: §Requirement — True iff this is REAL enforcement-gradient debt.

        RULE: a requirement is closeable debt iff its enforcement is NOT
        ENFORCED (PROSE or STRUCTURAL) AND its enforceability is ENFORCEABLE
        (a check_* or test COULD exist). A requirement whose enforceability
        is INHERENTLY_PROSE is honestly-labeled permanent discipline — it is
        NOT debt, no matter its enforcement level.

        WHY a method (not inline filtering): the enforcement-gradient debt
        count (P0 REFLECTION, docs/gen/UNENFORCED.md) previously conflated
        "no enforcer yet" with "no enforcer possible", so it could never
        converge to zero. This predicate is the single source of truth for
        that count, used by tools/what_now.py and tools/gen_spec.py alike.
        """
        return self.enforcement != ENFORCED and self.enforceability == ENFORCEABLE

    def is_open(self) -> bool:
        """Canon: §Requirement — True iff this requirement is an OPEN hole.

        RULE: open iff status starts with "OPEN". Open requirements are surfaced
        by the harness and listed in the generated OPEN.md.

        WHY a method (not status == ...): the open marker carries its question
        inline ("OPEN(which segment?)"); membership is a prefix test, kept here so
        the harness, the generator and the invariant agree.
        """
        return self.status.startswith(OPEN_PREFIX)
