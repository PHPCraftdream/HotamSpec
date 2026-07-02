"""Content-free framework structural scan — enforces R-content-free-framework.

Walks every module under spec/src/hotam_spec/ and asserts that no top-level name
is an INSTANCE of the domain object types (TensionGraph, Requirement, Conflict,
Axis, Stakeholder, Assumption). Classes themselves, functions, dunders, module-
level constants OTHER than those instances, and the type objects imported for
the purpose of type-checking are all fine.

WHY not a naive token-grep: the framework docstrings legitimately mention
'R-87', 'latency-vs-completeness', 'A-single-customer', etc. as illustrative
examples. A grep on those strings would false-positive. This test checks the
RUNTIME module namespace for live instances, not mentions in strings.

Reference: R-content-free-framework (SETTLED).
"""

from __future__ import annotations

import importlib
import sys
from pathlib import Path

_SRC = Path(__file__).resolve().parents[1] / "src"
if str(_SRC) not in sys.path:
    sys.path.insert(0, str(_SRC))

# Import the types we are scanning for AFTER adding src to path.
from hotam_spec.assumption import Assumption  # noqa: E402
from hotam_spec.axis import Axis  # noqa: E402
from hotam_spec.conflict import Conflict  # noqa: E402
from hotam_spec.entity import EntityInstance, EntityType  # noqa: E402
from hotam_spec.graph import TensionGraph  # noqa: E402
from hotam_spec.requirement import Requirement  # noqa: E402
from hotam_spec.stakeholder import Stakeholder  # noqa: E402

# EntityType/EntityInstance included per R-entity-is-declarative: the framework
# supplies NO built-in entity types — all are declared by domains in build_graph().
_INSTANCE_TYPES = (
    TensionGraph,
    Requirement,
    Conflict,
    Axis,
    Stakeholder,
    Assumption,
    EntityType,
    EntityInstance,
)
_TENSIO_SRC = _SRC / "hotam_spec"


def test_no_domain_instances_in_tensio_src() -> None:
    """No top-level domain-object instance lives in any spec/src/hotam_spec/*.py module.

    Uses the already-imported (canonical) module objects from sys.modules so
    that dataclass type annotations resolve correctly.  Scans every public,
    non-dunder name in each module's namespace.
    """
    py_files = sorted(_TENSIO_SRC.glob("*.py"))
    assert py_files, f"No .py files found under {_TENSIO_SRC}"

    bad: list[str] = []
    for path in py_files:
        stem = path.stem
        mod_key = f"hotam_spec.{stem}" if stem != "__init__" else "hotam_spec"
        # Ensure the module is imported (it should already be).
        mod = importlib.import_module(mod_key)
        for name, value in vars(mod).items():
            if name.startswith("_"):
                continue  # skip dunders and private names
            if isinstance(value, type):
                continue  # the class itself is fine; instances of it are not
            if isinstance(value, _INSTANCE_TYPES):
                bad.append(
                    f"{mod_key}:{name} — "
                    f"top-level {type(value).__name__} instance "
                    f"(violates R-content-free-framework)"
                )

    assert not bad, (
        "spec/src/hotam_spec/ contains domain-object instances:\n"
        + "\n".join(f"  {b}" for b in bad)
    )
