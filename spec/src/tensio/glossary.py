"""Canon: §Glossary — the methodology's controlled vocabulary (framework-side).

RULE: every §-token used in tensio framework docstrings MUST appear here as a
Term entry. Every Term entry MUST be referenced in at least one tensio docstring.
Violations are caught by tests/test_glossary_sync.py.

Canon: §Glossary — this module IS the authoritative membership list of admitted
methodology terms. Domain-side business terms (R-ids, axis slugs, stakeholders)
live in spec/content/graph.py — not here.

WHY a generated controlled vocabulary: terminology drift is its own kind of
invisibility — 'axis'/'dimension', 'steward'/'owner', 'conflict'/'tension'
fragment the methodology language without it. Direct lift of dev-coin's
test_glossary_sync.py pattern, but made GENERATED so the vocabulary and its
mirror (docs/gen/GLOSSARY.md) cannot drift from each other.
"""

from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class Term:
    """Canon: §Glossary — one entry in the controlled methodology vocabulary."""

    slug: str  # the canonical name as it appears in docstrings
    kind: str  # SECTION | LIFECYCLE_STATE | ROLE | CONCEPT | STATUS
    definition: str  # one-sentence definition


TERMS: tuple[Term, ...] = (
    # §-sections (kind=SECTION)
    Term(
        "§Requirement",
        "SECTION",
        "The requirement node — a claim the system shall satisfy.",
    ),
    Term(
        "§Conflict",
        "SECTION",
        "The first-class connector NODE between requirements, carrying axis+context+steward.",
    ),
    Term(
        "§Assumption",
        "SECTION",
        "A belief with its own lifecycle (HOLDS/UNCERTAIN/DEAD).",
    ),
    Term(
        "§Axis", "SECTION", "A controlled-vocabulary entry naming a tension dimension."
    ),
    Term(
        "§Stakeholder",
        "SECTION",
        "Accountability facet: who owns a requirement or stewards a conflict.",
    ),
    Term(
        "§Invariants",
        "SECTION",
        "Structural form of the tension graph (check_* functions).",
    ),
    Term("§Graph", "SECTION", "The TensionGraph container + traversal + loader."),
    Term(
        "§Lifecycle",
        "SECTION",
        "The generic state-machine value-type — keystone for Requirement.status / Conflict.lifecycle / future Operator/Process lifecycles.",
    ),
    Term(
        "§Operator",
        "SECTION",
        "The acting facet of a Stakeholder — owns a sub-domain, carries a context budget, and runs the closed loop.",
    ),
    Term(
        "§ContextBudget",
        "SECTION",
        "The working-store ceiling of an Operator; the substrate is free.",
    ),
    Term(
        "§Loop",
        "SECTION",
        "The closed loop: State→Diagnosis→Next-action→Action→regenerate→State.",
    ),
    Term(
        "§Glossary", "SECTION", "The methodology's controlled vocabulary (this module)."
    ),
    # Lifecycle states (kind=LIFECYCLE_STATE)
    Term(
        "DETECTED",
        "LIFECYCLE_STATE",
        "A conflict has been surfaced but no steward action yet.",
    ),
    Term(
        "ACKNOWLEDGED",
        "LIFECYCLE_STATE",
        "Steward accepts the conflict is real and owns it.",
    ),
    Term(
        "DECIDED",
        "LIFECYCLE_STATE",
        "Resolved with recorded rationale and/or a derived requirement.",
    ),
    Term(
        "REVISIT_WHEN",
        "LIFECYCLE_STATE",
        "Parked with an explicit revisit condition (anti-relitigation).",
    ),
    # Requirement statuses (kind=STATUS)
    Term("DRAFT", "STATUS", "Proposed, not yet accepted into the canon."),
    Term("SETTLED", "STATUS", "Accepted and currently held."),
    Term("OPEN", "STATUS", "Accepted-with-a-hole; carries a non-empty question."),
    Term("REJECTED", "STATUS", "Withdrawn; kept for history (anti-relitigation)."),
    # Assumption statuses
    Term("HOLDS", "STATUS", "Assumption currently believed true."),
    Term("UNCERTAIN", "STATUS", "Assumption whose truth is unconfirmed."),
    Term(
        "DEAD",
        "STATUS",
        "Assumption known false — dependents are surfaced as DRIFT_FALLOUT.",
    ),
    # Enforcement levels (P0)
    Term(
        "PROSE",
        "STATUS",
        "Enforcement level: recorded only, unenforced (soft context-debt).",
    ),
    Term(
        "STRUCTURAL",
        "STATUS",
        "Enforcement level: visible/addressable but no predicate fires.",
    ),
    Term(
        "ENFORCED",
        "STATUS",
        "Enforcement level: a check_* invariant or test fires on violation.",
    ),
    # Roles / concepts (kind=ROLE / CONCEPT)
    Term(
        "steward",
        "ROLE",
        "Holds a conflict; by construction NOT the owner of any of its members.",
    ),
    Term("owner", "ROLE", "Stakeholder who defends a requirement or an assumption."),
    Term(
        "operator",
        "ROLE",
        "An acting agent that owns a bounded sub-domain of the graph.",
    ),
    Term(
        "latent connector",
        "CONCEPT",
        "A requirement pair that SHOULD have a Conflict node but doesn't — a heuristic suspicion.",
    ),
    Term(
        "DRIFT_FALLOUT",
        "CONCEPT",
        "A DEAD assumption with live dependents — they must be revisited.",
    ),
    Term(
        "crystallize",
        "CONCEPT",
        "Move knowledge from working context into the durable substrate (graph + invariants + docs).",
    ),
    Term(
        "anchor",
        "CONCEPT",
        "A stable, short, typed identifier (R-/C-/A-/§) the operator references rather than re-carries.",
    ),
    Term(
        "substrate",
        "CONCEPT",
        "The durable, enforced/regenerable/addressable store — free of context cost.",
    ),
    Term(
        "burn-down",
        "CONCEPT",
        "Promotion of DRAFT requirements to ENFORCED; the methodology's honesty governor.",
    ),
    # §Proposal section and related concepts (P3 — Action half)
    Term(
        "§Proposal",
        "SECTION",
        "Structured operator-→-steward change proposals: the closed loop's ACT half.",
    ),
    Term(
        "playbook",
        "CONCEPT",
        "A band-specific procedure the AI operator follows to convert a what_now action into a structured proposal for steward approval.",
    ),
    Term(
        "apply_proposal",
        "CONCEPT",
        "The mechanical writer that lands a steward-approved proposal into spec/content/; the AI never edits the graph by hand.",
    ),
)


def term_slugs() -> frozenset[str]:
    """Canon: §Glossary — the set of admitted methodology terms (membership source)."""
    return frozenset(t.slug for t in TERMS)
