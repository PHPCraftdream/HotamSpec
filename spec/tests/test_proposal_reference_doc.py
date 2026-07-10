"""Anti-drift enforcer for docs/PROPOSAL-REFERENCE.md.

WHY: PROPOSAL-REFERENCE.md is a hand-written, field-by-field pretty-print of
the `Proposed*` dataclasses in `hotam_spec.proposal` (see lens-1-redundancy.md
finding #4 -- the rest of the repo lives by docs-as-code/anti-drift, but this
consumer-facing reference had no drift guard at all: nothing caught a doc that
named a field the dataclass no longer has, or omitted one it gained).

This is deliberately NOT a full-fidelity parser of the markdown. It only
checks the one claim that matters for a consumer copy-pasting JSON from the
doc: every backtick-quoted field name listed under a `**Required:**` /
`**Optional:**` line for a `## <Heading>` section names a REAL field on the
corresponding `Proposed*` dataclass, and every real field on that dataclass is
named somewhere in the doc's Required/Optional lines for that section. A
regex-level check, not a semantic one -- it will not catch a wrong default or
a misdescribed type, but it DOES catch the drift class that costs a consumer
the most: "the doc still shows a field that no longer exists" or "a new
required field silently isn't documented".
"""

from __future__ import annotations

import dataclasses
import re
from pathlib import Path

import pytest

from hotam_spec import proposal as proposal_mod

REPO_ROOT = Path(__file__).resolve().parents[2]
DOC_PATH = REPO_ROOT / "docs" / "PROPOSAL-REFERENCE.md"

# Maps the doc's "## <Heading>" text to the Proposed* dataclass it documents.
# "Enum reference" is a shared-vocabulary section, not a proposal kind, and is
# deliberately excluded (there is no single dataclass it maps to).
# "The consumer graph.py AST contract" (added Etap F, 2026-07-10) is likewise
# not a proposal-kind section -- it documents the WRITER's structural
# requirements on a consumer's graph.py (the shape apply_proposal.py's AST
# locators depend on), which has no corresponding Proposed* dataclass to map
# fields against.
_NON_KIND_SECTIONS: frozenset[str] = frozenset(
    {"Enum reference", "The consumer graph.py AST contract"}
)
HEADING_TO_CLASS: dict[str, type] = {
    "Stakeholder": proposal_mod.ProposedStakeholder,
    "Axis": proposal_mod.ProposedAxis,
    "Requirement": proposal_mod.ProposedRequirement,
    "Conflict (creation)": proposal_mod.ProposedConflict,
    "ConflictTransition": proposal_mod.ProposedConflictTransition,
    "Rejection": proposal_mod.ProposedRejection,
    "Assumption": proposal_mod.ProposedAssumption,
    "AssumptionTransition": proposal_mod.ProposedAssumptionTransition,
    "ConflictMemberUpdate": proposal_mod.ProposedConflictMemberUpdate,
    "OperatorBudget": proposal_mod.ProposedOperatorBudget,
    "EntityType": proposal_mod.ProposedEntityType,
}

_HEADING_RE = re.compile(r"^## (.+)$", re.MULTILINE)
_REQUIRED_RE = re.compile(
    r"\*\*Required:\*\*(.*?)(?=\*\*Optional:\*\*|\n\n|```)", re.DOTALL
)
_OPTIONAL_RE = re.compile(r"\*\*Optional:\*\*(.*?)(?=\n\n|```)", re.DOTALL)


def _backtick_tokens_at_top_level(chunk: str) -> list[str]:
    """Extract backtick-quoted tokens that are NOT nested inside parentheses.

    The doc wraps the field name itself in backticks right after the label
    (e.g. "`status` (`DRAFT` | `SETTLED` | ...)") -- the field name is always
    at paren-depth 0; enum values / examples named inside the explanatory
    parenthetical are depth >= 1 and are deliberately skipped, so this stays a
    FIELD-name extractor, not a generic backtick scanner.
    """
    tokens: list[str] = []
    depth = 0
    i, n = 0, len(chunk)
    while i < n:
        c = chunk[i]
        if c == "(":
            depth += 1
            i += 1
            continue
        if c == ")":
            depth -= 1
            i += 1
            continue
        if c == "`" and depth == 0:
            j = chunk.index("`", i + 1)
            tokens.append(chunk[i + 1 : j])
            i = j + 1
            continue
        i += 1
    return tokens


def _doc_sections() -> dict[str, str]:
    """Split PROPOSAL-REFERENCE.md into {heading: body} by level-2 headings."""
    text = DOC_PATH.read_text(encoding="utf-8")
    parts = _HEADING_RE.split(text)
    # parts[0] is the preamble before the first "## "; then heading, body, ...
    sections: dict[str, str] = {}
    it = iter(parts[1:])
    for heading, body in zip(it, it):
        sections[heading.strip()] = body
    return sections


def _documented_fields(body: str) -> set[str]:
    """The set of field names named in a section's Required/Optional lines."""
    fields: set[str] = set()
    for pattern in (_REQUIRED_RE, _OPTIONAL_RE):
        m = pattern.search(body)
        if m:
            fields.update(_backtick_tokens_at_top_level(m.group(1)))
    return fields


def test_doc_exists() -> None:
    """docs/PROPOSAL-REFERENCE.md exists at the expected path."""
    assert DOC_PATH.is_file(), f"missing {DOC_PATH}"


def test_every_proposed_kind_has_a_doc_section() -> None:
    """Every kind in HEADING_TO_CLASS actually has a '## <Heading>' section."""
    sections = _doc_sections()
    missing = [h for h in HEADING_TO_CLASS if h not in sections]
    assert not missing, (
        f"PROPOSAL-REFERENCE.md is missing section(s) for: {missing} "
        "(a Proposed* kind exists in code with no matching '## <Heading>')"
    )


@pytest.mark.parametrize("heading", sorted(HEADING_TO_CLASS))
def test_doc_fields_match_dataclass_fields(heading: str) -> None:
    """Required/Optional field names in each doc section == the dataclass's real fields.

    Two-way check: a field named in the doc that the dataclass does NOT have
    is stale prose (drift after a field was renamed/removed); a dataclass
    field NOT named anywhere in the doc's Required/Optional lines is an
    undocumented field (drift after a field was added).
    """
    sections = _doc_sections()
    body = sections[heading]
    documented = _documented_fields(body)

    cls = HEADING_TO_CLASS[heading]
    real_fields = {f.name for f in dataclasses.fields(cls)}

    undocumented = real_fields - documented
    stale = documented - real_fields

    assert not undocumented, (
        f"PROPOSAL-REFERENCE.md '## {heading}' does not mention field(s) "
        f"{sorted(undocumented)} that {cls.__name__} actually has -- doc is "
        "missing a field (likely added to the dataclass after the doc was written)."
    )
    assert not stale, (
        f"PROPOSAL-REFERENCE.md '## {heading}' mentions field(s) {sorted(stale)} "
        f"that {cls.__name__} does NOT have -- doc is stale (likely renamed/removed "
        "in the dataclass after the doc was written)."
    )


def test_no_stray_headings_without_a_known_kind() -> None:
    """Every '## <Heading>' in the doc (besides the non-kind sections) maps to
    a known kind.

    Catches the opposite drift direction: a NEW Proposed* kind gets a doc
    section, but nobody added it to HEADING_TO_CLASS in this test -- this
    would otherwise let a section silently go unchecked forever.
    """
    sections = _doc_sections()
    known = set(HEADING_TO_CLASS) | _NON_KIND_SECTIONS
    stray = [h for h in sections if h not in known]
    assert not stray, (
        f"PROPOSAL-REFERENCE.md has section(s) {stray} not covered by this "
        "test's HEADING_TO_CLASS mapping -- add the new kind to HEADING_TO_CLASS "
        "in test_proposal_reference_doc.py so its fields stay anti-drift-checked."
    )
