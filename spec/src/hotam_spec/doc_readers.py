"""Canon: §Requirement — every generated doc names its reader (R-doc-names-reader).

RULE: every generated markdown doc under docs/gen/, spec/docs/{thinking,tools}/,
and docs/methodology/atoms/ carries a `reader: <Stakeholder.id>` line in its
header, naming the Stakeholder.id this doc is written FOR. WHY: a doc with no
named reader is an important-yet-invisible artifact — nobody is accountable
for reading it, so drift between the doc and its audience's actual needs goes
unnoticed (the generative law: important-yet-invisible -> typed anchored node
under a named steward).

WHY a doc-kind -> ROLE mapping, not hardcoded stakeholder ids: the framework
ships content-free (R-content-free-no-business-data /
R-content-free-no-seed-graph) — it must not bake a specific domain's
stakeholder ids (e.g. "framework-author") into framework code. Instead each
doc kind is mapped to a portable ROLE hint (e.g. ROLE_FRAMEWORK_MAINTAINER);
`resolve_reader()` turns that hint into a concrete Stakeholder.id by looking
it up in an EXPLICIT role -> Stakeholder.id binding the active domain
declares in its own `manifest.py` (the `DOC_READERS` dict, read via
`hotam_spec.graph.active_domain_doc_readers()`). A domain with no binding
for a role is an honest gap (see `resolve_reader()` fallback), not a
silent lie — and, critically, NOT a guess: resolution never scans
stakeholder ids for a role-shaped substring (R-doc-readers-declared-not-guessed).
A stakeholder id that happens to contain a role word (e.g. a "travel-agent"
stakeholder in some future business domain) can no longer be silently
captured as the reader of operator-facing docs; only a binding the domain
author wrote down on purpose counts.

Doc kinds fall into three families, each pointing at a distinct role:
  - OPERATOR-FACING (the agent boots from these every turn): CONSTITUTION.md,
    OPEN.md, UNENFORCED.md, methodology atoms, spec/docs/thinking/*.md,
    spec/docs/tools/*.md -> ROLE_OPERATOR (the AI agent that reads/acts).
  - STEWARD-FACING rosters (what a human steward reviews to decide):
    REQUIREMENTS.md, TENSIONS.md, DECISIONS.md, HISTORY.md, ENTITIES.md
    -> ROLE_DOMAIN_STEWARD (the domain's accountable human).
  - FRAMEWORK-INTERNAL (framework's own plumbing, not domain content):
    FRAMEWORK-INVARIANTS.md, GLOSSARY.md -> ROLE_FRAMEWORK_MAINTAINER.

This mapping is deliberate, not incidental: R-requirement-claim-is-atomic
demands one concern per doc; naming the reader forces the generator to ask
"who is this FOR" for every doc kind, surfacing kinds that serve nobody.
"""

from __future__ import annotations

# --- Portable role hints (framework-side, content-free) ---------------------
# A role hint is NOT a Stakeholder.id — it is resolved against the active
# domain's graph.stakeholders at generation time by resolve_reader().

ROLE_OPERATOR = "operator"
ROLE_DOMAIN_STEWARD = "domain-steward"
ROLE_FRAMEWORK_MAINTAINER = "framework-maintainer"

# --- doc-kind -> role hint mapping (the honest table) ------------------------
# Keys are the doc "kind" identifiers used by gen_spec.py's build_* functions
# (one entry per distinct generated-doc family). This IS the reader mapping;
# `check_doc_reader_resolves_to_stakeholder` validates it against the active
# domain's graph so a role hint that resolves to no Stakeholder cannot
# silently drift into a generated doc.

DOC_READER_ROLES: dict[str, str] = {
    # Operator-facing: reloaded every mediation-loop turn (ORIENT/LOCATE steps).
    "CONSTITUTION": ROLE_OPERATOR,
    "OPEN": ROLE_OPERATOR,
    "UNENFORCED": ROLE_OPERATOR,
    "ATOMS_OPERATOR": ROLE_OPERATOR,
    "ATOMS_SUBSTRATE": ROLE_OPERATOR,
    "ATOMS_DISCIPLINE": ROLE_OPERATOR,
    "ATOMS_CHECK": ROLE_OPERATOR,
    "SHARED_THINKING": ROLE_OPERATOR,
    "SHARED_TOOL": ROLE_OPERATOR,
    # Steward-facing rosters: what a human steward reviews to decide.
    "REQUIREMENTS": ROLE_DOMAIN_STEWARD,
    "TENSIONS": ROLE_DOMAIN_STEWARD,
    "DECISIONS": ROLE_DOMAIN_STEWARD,
    "HISTORY": ROLE_DOMAIN_STEWARD,
    "ENTITIES": ROLE_DOMAIN_STEWARD,
    # Framework-internal plumbing: not domain content, framework's own audit trail.
    "FRAMEWORK_INVARIANTS": ROLE_FRAMEWORK_MAINTAINER,
    "GLOSSARY": ROLE_FRAMEWORK_MAINTAINER,
    "AUDIT": ROLE_FRAMEWORK_MAINTAINER,
    "REPO_MAP": ROLE_OPERATOR,
}

# Sentinel returned when the active domain has declared no binding for the
# role hint (or has declared a binding to an id absent from `stakeholder_ids`)
# — an honest "unresolved" marker rather than a fabricated id. A doc carrying
# this sentinel FAILS check_doc_reader_resolves_to_stakeholder, surfacing the
# gap instead of hiding it.
UNRESOLVED_READER = "(unresolved-reader)"


def resolve_reader(
    doc_kind: str,
    stakeholder_ids: frozenset[str],
    role_bindings: dict[str, str] | None = None,
) -> str:
    """Canon: §Requirement — resolve a doc kind to a concrete Stakeholder.id.

    RULE: looks up DOC_READER_ROLES[doc_kind] for the role hint, then returns
    `role_bindings[role]` IF AND ONLY IF that id is present in
    `stakeholder_ids`. Returns UNRESOLVED_READER if the doc_kind is unknown,
    `role_bindings` has no entry for the role, or the bound id is not a known
    Stakeholder — never fabricates or guesses an id
    (R-ai-presents-not-decides applied to doc plumbing: an honest gap, not a
    silent lie).

    WHY an explicit binding dict, not substring matching over stakeholder ids
    (R-doc-readers-declared-not-guessed): a substring hint like "agent" can be
    silently captured by an unrelated stakeholder whose id happens to contain
    that substring (e.g. a "travel-agent" stakeholder in some future business
    domain would wrongly become the reader of operator-facing docs). The
    framework stays content-free by never hardcoding a stakeholder id here —
    instead each ACTIVE DOMAIN declares its own `DOC_READERS: dict[role,
    Stakeholder.id]` in its `manifest.py`, read via
    `hotam_spec.graph.active_domain_doc_readers()` and passed in as
    `role_bindings`. Callers that omit `role_bindings` (or pass `None`) get
    an empty mapping — every doc_kind resolves to UNRESOLVED_READER, the
    same honest gap as "domain has not adopted this aspect yet".
    """
    role = DOC_READER_ROLES.get(doc_kind)
    if role is None:
        return UNRESOLVED_READER
    bindings = role_bindings or {}
    bound_id = bindings.get(role)
    if bound_id is not None and bound_id in stakeholder_ids:
        return bound_id
    return UNRESOLVED_READER


def reader_line(
    doc_kind: str,
    stakeholder_ids: frozenset[str],
    role_bindings: dict[str, str] | None = None,
) -> str:
    """Canon: §Requirement — render the `reader: <id>` header line for a doc kind."""
    return f"reader: {resolve_reader(doc_kind, stakeholder_ids, role_bindings)}"
