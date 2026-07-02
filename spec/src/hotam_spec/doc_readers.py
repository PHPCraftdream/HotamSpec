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
it up in the ACTIVE domain graph's `g.stakeholders` at generation time. A
domain with no stakeholder for a role is an honest gap (see
`resolve_reader()` fallback), not a silent lie.

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
}

# --- role hint -> preferred Stakeholder.id substrings (resolution order) ----
# Tried in order against the active domain's stakeholder ids; first match wins.
# This is a RESOLUTION HINT, not a hard binding — resolve_reader() falls back
# honestly (see below) when no stakeholder matches.

_ROLE_ID_HINTS: dict[str, tuple[str, ...]] = {
    ROLE_OPERATOR: ("ai-agent", "agent", "operator"),
    ROLE_DOMAIN_STEWARD: ("domain-user", "steward", "domain"),
    ROLE_FRAMEWORK_MAINTAINER: ("framework-author", "framework-reviewer", "framework"),
}

# Sentinel returned when no stakeholder in the active graph matches the role
# hint — an honest "unresolved" marker rather than a fabricated id. A doc
# carrying this sentinel FAILS check_doc_reader_resolves_to_stakeholder,
# surfacing the gap instead of hiding it.
UNRESOLVED_READER = "(unresolved-reader)"


def resolve_reader(doc_kind: str, stakeholder_ids: frozenset[str]) -> str:
    """Canon: §Requirement — resolve a doc kind to a concrete Stakeholder.id.

    RULE: looks up DOC_READER_ROLES[doc_kind] for the role hint, then returns
    the first id in `stakeholder_ids` matching one of that role's
    `_ROLE_ID_HINTS` substrings (case-insensitive). Returns UNRESOLVED_READER
    if the doc_kind is unknown or no stakeholder matches — never fabricates an
    id (R-ai-presents-not-decides applied to doc plumbing: an honest gap, not
    a silent lie).

    WHY substring matching over stakeholder ids (not an exact id table): the
    framework is content-free and cannot know a domain's exact stakeholder
    slugs in advance; substring hints let the same framework code resolve
    correctly against any domain that names its stakeholders conventionally
    (e.g. "framework-author", "domain-user") while staying honest when it
    cannot.
    """
    role = DOC_READER_ROLES.get(doc_kind)
    if role is None:
        return UNRESOLVED_READER
    hints = _ROLE_ID_HINTS.get(role, ())
    for hint in hints:
        for sid in sorted(stakeholder_ids):
            if hint in sid.lower():
                return sid
    return UNRESOLVED_READER


def reader_line(doc_kind: str, stakeholder_ids: frozenset[str]) -> str:
    """Canon: §Requirement — render the `reader: <id>` header line for a doc kind."""
    return f"reader: {resolve_reader(doc_kind, stakeholder_ids)}"
