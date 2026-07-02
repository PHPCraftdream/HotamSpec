"""Canon: §Closure — T1 tiered LAND gate: select a targeted enforcer subset instead of the full suite.

Stage B (tiered verification gates). The LAND step of the mediation loop
(R-verify-closure-per-action) pays the FULL pytest suite on every single
applied proposal — honest, but a ~100-300s tax per action on a graph this
size. Most proposals touch ONE Requirement or Conflict whose `enforced_by`
tuple already names the exact enforcers (check_* / test_*) that guard it —
the graph IS the test-impact map. T1 runs ONLY that targeted subset plus a
small always-run baseline (determinism + smoke); T2 (full suite) remains the
law at wave/commit boundaries and is the automatic fallback whenever the
selector is not confident.

FAIL CLOSED, always: if a proposal's target has no `enforced_by`, if any
enforcer name cannot be resolved to a concrete pytest node-id, if the
proposal's kind touches framework code rather than graph metadata, or if the
resulting node-id set is empty — select_tier1() returns None and the caller
MUST run the full suite. There is no "confident-enough" partial-uncertainty
path; uncertainty of ANY kind maps to full-suite, never a smaller-than-normal
partial run.

Resolution rules for one `enforced_by` entry (best-effort, exhaustively tried
in order; first match wins):
  1. "test_file.py::test_func"        -> used as a pytest node-id verbatim.
  2. "test_file.py"                   -> the whole file is a node-id.
  3. "check_<name>"                   -> resolved via the same check_*->test-file
                                          mapping gen_spec.py's CONCEPT-MAP block
                                          builds (grep test files for the bare
                                          check_* name; R-prefer-tool-over-hand:
                                          reuses gen_spec._scan_concept_map's
                                          check_to_tests logic rather than
                                          re-deriving it).
  4. "test_<name>" (bare function)    -> grep test files for `def test_<name>(`
                                          and use the owning file as a node-id
                                          (function-level id would be more
                                          precise but bare names are not
                                          guaranteed unique across files —
                                          file-level keeps the mapping honest).
  5. anything else (a doc path, a tool path, a bare requirement/tool name,
     "CRITICAL_CORE_INVARIANTS", ...) -> UNRESOLVED. Any unresolved entry
     fails the whole selection closed.

Run (from spec/):
  uv run python tools/gate.py R-smoke-test          # print the T1 selection for an R-id
  uv run python tools/gate.py C-8600b1b8 --explain  # show why (or why not)

Deterministic: sorted node-id output, no timestamps/randomness.
"""

from __future__ import annotations

import argparse
import re
import sys
from dataclasses import dataclass, field
from pathlib import Path

# --- Make the hotam_spec package + sibling tools importable --------------------

SPEC_ROOT = Path(__file__).resolve().parents[1]
if str(SPEC_ROOT / "src") not in sys.path:
    sys.path.insert(0, str(SPEC_ROOT / "src"))
if str(SPEC_ROOT / "tools") not in sys.path:
    sys.path.insert(0, str(SPEC_ROOT / "tools"))

from hotam_spec.enforcer_resolution import (  # noqa: E402
    bare_test_func_to_file as _shared_bare_test_func_to_file,
    check_to_tests_map as _shared_check_to_tests_map,
    resolve_one_enforcer as _shared_resolve_one_enforcer,
)
from hotam_spec.graph import TensionGraph, load_content_graph  # noqa: E402

SPEC_TESTS_DIR = SPEC_ROOT / "tests"

# --- Always-run baseline (small, fast, in-process; NOT the full suite) --------
#
# Every T1 selection includes these regardless of target, because they are
# the cheapest possible signal that regen honesty and structural determinism
# still hold after a write (R-smoke-test, generator determinism, the
# R<->enforcer bijection). None of these spawn a subprocess.
ALWAYS_RUN: tuple[str, ...] = (
    "tests/test_smoke.py::test_smoke",
    "tests/test_docs_gen.py::test_generator_is_deterministic",
    "tests/test_bijection.py::test_content_graph_bijection_clean",
)


@dataclass(frozen=True)
class GateResult:
    """Canon: §Closure — the outcome of a T1 selection attempt.

    Fields:
      confident   — True iff a targeted node-id set was resolved; False means
                    fail-closed (caller MUST run T2, the full suite).
      node_ids    — the pytest node-id selection (sorted, deduped) when
                    confident; empty tuple when not confident.
      reason      — human-readable note: what was targeted, what resolved,
                    or exactly why the selector fell back to full-suite.
    """

    confident: bool
    node_ids: tuple[str, ...] = field(default_factory=tuple)
    reason: str = ""


def _check_to_tests_map(tests_dir: Path | None = None) -> dict[str, list[str]]:
    """Canon: §Closure — check_* name -> test files that reference it (bare grep).

    Thin wrapper over hotam_spec.enforcer_resolution.check_to_tests_map — the
    shared resolver module (R-prefer-tool-over-hand: one source of truth,
    reused by both this tiered gate and check_enforced_by_resolvable).
    """
    return _shared_check_to_tests_map(tests_dir or SPEC_TESTS_DIR)


def _bare_test_func_to_file(name: str, tests_dir: Path | None = None) -> str | None:
    """Canon: §Closure — bare `test_foo` function name -> owning test file (rel path).

    Thin wrapper over hotam_spec.enforcer_resolution.bare_test_func_to_file.
    """
    return _shared_bare_test_func_to_file(name, tests_dir or SPEC_TESTS_DIR)


def _resolve_one_enforcer(
    entry: str,
    check_to_tests: dict[str, list[str]],
    tests_dir: Path | None = None,
) -> list[str] | None:
    """Resolve a single `enforced_by` string to pytest node-id(s), or None (unresolved).

    Thin wrapper over hotam_spec.enforcer_resolution.resolve_one_enforcer.
    """
    return _shared_resolve_one_enforcer(entry, check_to_tests, tests_dir or SPEC_TESTS_DIR)


def select_tier1(
    target_anchor: str,
    g: TensionGraph | None = None,
    tests_dir: Path | None = None,
) -> GateResult:
    """Canon: §Closure — attempt to build the T1 targeted enforcer subset for a target.

    `target_anchor` is a Requirement.id (R-...) or Conflict.id (C-...) already
    present in the graph (i.e. this is an UPDATE to an existing node, not the
    creation of a brand-new one — a brand-new node has no `enforced_by` yet by
    definition and MUST fail closed). Looks up the node's `enforced_by` tuple
    and resolves each entry to pytest node-id(s); ALWAYS_RUN is unioned in.

    Fails closed (confident=False) when:
      - the target is not found in the graph (new node, or typo);
      - the target's enforced_by tuple is empty;
      - ANY enforced_by entry fails to resolve to a node-id.
    """
    graph = g if g is not None else load_content_graph()

    enforced_by: tuple[str, ...] | None = None
    for req in graph.requirements:
        if req.id == target_anchor:
            enforced_by = req.enforced_by
            break
    if enforced_by is None:
        for conflict in getattr(graph, "conflicts", ()):
            if getattr(conflict, "id", None) == target_anchor:
                # Conflict nodes do not carry enforced_by (§Conflict's own
                # structural checks are graph-wide, not per-instance) —
                # uncertain by construction, fail closed.
                return GateResult(
                    confident=False,
                    reason=(
                        f"target {target_anchor!r} is a Conflict node — Conflict "
                        "has no per-instance enforced_by; fail-closed to full suite."
                    ),
                )

    if enforced_by is None:
        return GateResult(
            confident=False,
            reason=(
                f"target {target_anchor!r} not found in the current graph "
                "(new node, or a target outside Requirement/Conflict) — "
                "fail-closed to full suite."
            ),
        )

    if not enforced_by:
        return GateResult(
            confident=False,
            reason=(
                f"target {target_anchor!r} has an empty enforced_by tuple — "
                "no targeted enforcer is known; fail-closed to full suite."
            ),
        )

    check_to_tests = _check_to_tests_map(tests_dir)
    node_ids: set[str] = set(ALWAYS_RUN)
    unresolved: list[str] = []
    for entry in enforced_by:
        resolved = _resolve_one_enforcer(entry, check_to_tests, tests_dir)
        if resolved is None:
            unresolved.append(entry)
            continue
        node_ids.update(resolved)

    if unresolved:
        return GateResult(
            confident=False,
            reason=(
                f"target {target_anchor!r}: {len(unresolved)} enforced_by "
                f"entr{'y' if len(unresolved) == 1 else 'ies'} could not be "
                f"resolved to a pytest node-id ({unresolved!r}) — "
                "fail-closed to full suite."
            ),
        )

    return GateResult(
        confident=True,
        node_ids=tuple(sorted(node_ids)),
        reason=(
            f"target {target_anchor!r}: resolved {len(enforced_by)} enforced_by "
            f"entr{'y' if len(enforced_by) == 1 else 'ies'} to "
            f"{len(node_ids)} pytest node-id(s) (plus {len(ALWAYS_RUN)} always-run)."
        ),
    )


def main(argv: list[str] | None = None) -> int:
    """Canon: §Closure — CLI entry point: print the T1 selection for a target anchor."""
    if hasattr(sys.stdout, "reconfigure"):
        # Reason strings carry UTF-8 punctuation (em-dashes); a redirected
        # Windows stdout defaults to cp1252 and would mangle or crash on them.
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    parser = argparse.ArgumentParser(
        description=(
            "Print the T1 targeted-enforcer pytest node-id selection for a "
            "Requirement or Conflict anchor, or report why it fails closed."
        )
    )
    parser.add_argument("target", help="Requirement id (R-...) or Conflict id (C-...).")
    parser.add_argument(
        "--explain",
        action="store_true",
        help="Print the reason string in addition to the node-id list.",
    )
    args = parser.parse_args(argv)

    result = select_tier1(args.target)
    if args.explain:
        print(f"confident: {result.confident}")
        print(f"reason: {result.reason}")
    if result.confident:
        for node_id in result.node_ids:
            print(node_id)
        return 0
    print("FAIL-CLOSED: full suite required.", file=sys.stderr)
    return 1


if __name__ == "__main__":
    sys.exit(main())
