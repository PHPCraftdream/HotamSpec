"""Tests for spec/tools/audit_atomicity.py — the atomicity audit tool.

Enforcer for R-audit-atomicity-tool (the audit is performed by a deterministic
TOOL, not a one-off hand invocation) and the lift for the projected
R-tool-audit-atomicity (R-tool-is-its-own-requirement machinery: this file's
existence flips the projected requirement's enforcer).

Covers:
  1. Claim heuristics: compound conjunctions/semicolons flagged; atomic passes.
  2. Invariant audit runs over ALL_INVARIANTS without error.
  3. main(--demo) is deterministic (two runs, byte-identical AUDIT.md) and
     writes to GEN_DIR (monkeypatched to tmp so the real docs are untouched).
"""

from __future__ import annotations

import sys
from pathlib import Path

_SRC = Path(__file__).resolve().parents[1] / "src"
_TOOLS = Path(__file__).resolve().parents[1] / "tools"
for _p in (_SRC, _TOOLS):
    if str(_p) not in sys.path:
        sys.path.insert(0, str(_p))

import audit_atomicity  # noqa: E402
from hotam_spec.invariants import ALL_INVARIANTS  # noqa: E402


def test_audit_claim_flags_compound_conjunction() -> None:
    """A claim joining two obligations with ', and shall' is COMPOUND."""
    verdict, reason = audit_atomicity._audit_claim(
        "The system shall write the file, and shall verify the checksum."
    )
    assert verdict == "COMPOUND"
    assert reason


def test_audit_claim_flags_semicolon() -> None:
    """A claim carrying two clauses split by '; ' is COMPOUND."""
    verdict, _ = audit_atomicity._audit_claim(
        "The system shall write the file; the checksum shall be verified."
    )
    assert verdict == "COMPOUND"


def test_audit_claim_atomic_passes() -> None:
    """A single-concern claim is ATOMIC with no reason."""
    verdict, reason = audit_atomicity._audit_claim(
        "The system shall write the file."
    )
    assert verdict == "ATOMIC"
    assert reason == ""


def test_scope_disclaimer_exempts_trailing_boundary_clause() -> None:
    """A single obligation carrying a trailing '-- this does not ...' scope
    disclaimer is ATOMIC -- the disclaimer narrows the one promise, it is not
    a second obligation (mirrors R-commit-boundary-checkable / R-tiered-gate).
    """
    verdict, _ = audit_atomicity._audit_claim(
        "The gate shall run the full suite at commit boundaries -- this does "
        "not itself verify that a steward actually runs it."
    )
    assert verdict == "ATOMIC"


def test_scope_disclaimer_does_not_hide_a_second_obligation() -> None:
    """NEGATIVE (anti-leak): the scope-disclaimer exemption must not become a
    hole -- a genuine SECOND obligation ('shall ...') smuggled AFTER a
    '-- this does not ...' marker is STILL flagged COMPOUND, so authors cannot
    escape the ratchet by burying a compound behind a disclaimer clause.
    """
    verdict, _ = audit_atomicity._audit_claim(
        "The system shall write the file -- this does not itself run it; "
        "the checksum shall be verified afterwards."
    )
    assert verdict == "COMPOUND"


def test_scope_disclaimer_does_not_hide_compound_before_it() -> None:
    """NEGATIVE (anti-leak): a real two-obligation 'and shall' compound placed
    BEFORE a trailing disclaimer is still caught -- stripping the disclaimer
    only removes the tail, the head compound signals still fire.
    """
    verdict, _ = audit_atomicity._audit_claim(
        "The system shall write the file, and shall verify the checksum "
        "-- this is the mechanically checkable slice."
    )
    assert verdict == "COMPOUND"


def test_sub_rule_declaration_does_not_exempt_a_second_entity_loop() -> None:
    """NEGATIVE (anti-leak): the 'N sub-rules' docstring self-declaration only
    silences the weak multiple-violation-message proxy. A genuinely SECOND
    independent relation -- a second statement-level ``for x in g.<entity>``
    walk -- is a structural signal that MUST still fire COMPOUND even when the
    docstring declares several sub-rules, so the marker cannot be abused to
    wave a real multi-relation check past the ratchet.
    """

    def check_declared_but_two_entity_walks(g):  # pragma: no cover - probe
        """RULE: four sub-rules ANDed under one banner."""
        out = []
        for r in g.requirements:
            out.append("a")
        for c in g.conflicts:
            out.append("b")
        return out

    verdict, reason = audit_atomicity._audit_invariant(
        check_declared_but_two_entity_walks
    )
    assert verdict == "COMPOUND"
    assert "entity types" in reason


def test_sub_rule_declaration_exempts_only_up_to_declared_count() -> None:
    """The message-count exemption is bounded: a check whose distinct
    violation-message count EXCEEDS its self-declared sub-rule count is still
    COMPOUND (the declaration cannot claim fewer rules than the body shows).
    """

    def check_five_msgs_declares_two(g):  # pragma: no cover - probe
        """RULE: two sub-rules."""
        from hotam_spec.invariants import Violation

        out = []
        if len(g.requirements) == 0:
            out.append(Violation("x", "t", "message alpha kind one"))
        if len(g.requirements) == 1:
            out.append(Violation("x", "t", "message beta kind two"))
        if len(g.requirements) == 2:
            out.append(Violation("x", "t", "message gamma kind three"))
        if len(g.requirements) == 3:
            out.append(Violation("x", "t", "message delta kind four"))
        if len(g.requirements) == 4:
            out.append(Violation("x", "t", "message epsilon kind five"))
        return out

    verdict, reason = audit_atomicity._audit_invariant(check_five_msgs_declares_two)
    assert verdict == "COMPOUND"
    assert "declares 2 sub-rule" in reason


def test_comprehension_not_counted_as_second_entity_loop() -> None:
    """A comprehension building a lookup table consumed by a single walk is
    plumbing, not a second relation -- only the statement-level for-loop counts,
    so this one-relation check is ATOMIC (guards the _count_entity_loops AST fix).
    """

    def check_one_walk_with_index(g):  # pragma: no cover - probe
        """RULE: one relation over requirements using a conflict index."""
        by_id = {c.id: c for c in g.conflicts}
        out = []
        for r in g.requirements:
            if r.id in by_id:
                out.append("x")
        return out

    verdict, _ = audit_atomicity._audit_invariant(check_one_walk_with_index)
    assert verdict == "ATOMIC"


def test_audit_invariant_runs_over_all_invariants() -> None:
    """_audit_invariant yields a (verdict, reason) for every registered check_*."""
    for fn in ALL_INVARIANTS:
        verdict, _reason = audit_atomicity._audit_invariant(fn)
        assert verdict in ("ATOMIC", "COMPOUND"), (
            f"{fn.__name__}: unexpected verdict {verdict!r}"
        )


def test_main_demo_is_deterministic_and_writes_audit_md(
    tmp_path: Path, monkeypatch, capsys
) -> None:
    """Two --demo runs produce byte-identical AUDIT.md (deterministic tool)."""
    monkeypatch.setattr(audit_atomicity, "GEN_DIR", tmp_path)

    audit_atomicity.main(["--demo"])
    first = (tmp_path / "AUDIT.md").read_text(encoding="utf-8")
    audit_atomicity.main(["--demo"])
    second = (tmp_path / "AUDIT.md").read_text(encoding="utf-8")

    assert first == second, "audit_atomicity output must be byte-deterministic"
    assert first.startswith("<!-- Generated by tools/audit_atomicity.py")
    assert "# Atomicity Audit" in first
    out = capsys.readouterr().out
    assert "Atomicity Audit" in out
