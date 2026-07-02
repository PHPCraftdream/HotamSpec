"""Canon: §Requirement / §Invariants — R-no-observation-type slice of R-observation-evidence-scope.

Structural scan proving the ONE mechanically checkable slice of
R-observation-evidence-scope: hotam_spec ships no Observation/Evidence class,
and Assumption is the package's sole belief-carrying node type. The full
R-observation-evidence-scope claim (operator epistemics live in the working
dialogue, crystallized only on request) is NOT machine-checkable in its
entirety -- that half stays STRUCTURAL/prose discipline -- but "no such type
exists in the ontology" IS a mechanical AST/namespace fact, so it is split out
as its own ENFORCED atom (R-commit-boundary-checkable's slice pattern:
R-no-observation-type enforces a narrow slice; the parent claim remains a
broader, partly-prose discipline).

WHY a class/name scan (not a docstring grep): a docstring may legitimately
DISCUSS the word 'Observation' (as this very module does) without shipping a
type; only a live class definition or an importable dataclass named
Observation/Evidence in the package is the structural violation this test
guards against.
"""

from __future__ import annotations

import ast
import sys
from pathlib import Path

_SRC = Path(__file__).resolve().parents[1] / "src"
if str(_SRC) not in sys.path:
    sys.path.insert(0, str(_SRC))

_TENSIO_SRC = _SRC / "hotam_spec"

_FORBIDDEN_CLASS_NAMES = frozenset({"Observation", "Evidence"})


def test_no_observation_or_evidence_class_defined_anywhere_in_hotam_spec() -> None:
    """AST-scan every spec/src/hotam_spec/*.py module: no class named
    'Observation' or 'Evidence' may be defined anywhere in the package.

    This is a static AST scan (no import/execution needed), so it catches a
    class definition regardless of whether anything currently instantiates or
    exports it.
    """
    py_files = sorted(_TENSIO_SRC.glob("*.py"))
    assert py_files, f"No .py files found under {_TENSIO_SRC}"

    offenders: list[str] = []
    for path in py_files:
        tree = ast.parse(path.read_text(encoding="utf-8"), filename=str(path))
        for node in ast.walk(tree):
            if isinstance(node, ast.ClassDef) and node.name in _FORBIDDEN_CLASS_NAMES:
                offenders.append(f"{path.name}: class {node.name} (line {node.lineno})")

    assert not offenders, (
        "hotam_spec must ship no Observation/Evidence class -- Assumption is "
        "the sole belief-carrying node type (R-observation-evidence-scope, "
        "R-no-observation-type slice). Offenders:\n" + "\n".join(offenders)
    )


def test_assumption_is_the_only_belief_carrying_dataclass_by_convention() -> None:
    """hotam_spec.assumption.Assumption exists and is the package's designated
    belief-carrying node type (status: HOLDS | UNCERTAIN | DEAD). This test
    anchors the POSITIVE half of the claim -- Assumption exists and is
    importable -- alongside the negative scan above, so the slice reads as
    'exactly one belief-carrying type, and it is this one', not merely
    'no forbidden names'.
    """
    from hotam_spec.assumption import DEAD, HOLDS, UNCERTAIN, Assumption  # noqa: PLC0415

    a = Assumption(id="A-x", statement="x", status=HOLDS, owner="s")
    assert a.status in (HOLDS, UNCERTAIN, DEAD)
