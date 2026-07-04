"""Canon: §Closure — shared enforcer-name -> pytest node-id resolution logic.

Extracted from tools/gate.py so both the T1 tiered gate (a tool) and a new
structural invariant (check_enforced_by_resolvable, which must live in
src/hotam_spec/invariants.py — invariants must not import from tools/) can
share ONE resolution algorithm (R-prefer-tool-over-hand: one source of
truth, not two hand-synced copies).

Resolution rules for one `enforced_by` entry (best-effort, exhaustively
tried in order; first match wins) — identical to gate.py's original
docstring:
  1. "test_file.py::test_func"        -> used as a pytest node-id verbatim.
  2. "test_file.py"                   -> the whole file is a node-id.
  3. "check_<name>"                   -> resolved via a grep of test files
                                          for the bare check_* name.
  4. "test_<name>" (bare function)    -> grep test files for `def test_<name>(`
                                          and use the owning file as a node-id.
  5. anything else                    -> UNRESOLVED.

Deterministic: no timestamps/randomness; pure filesystem read + regex.
"""

from __future__ import annotations

import functools
import re
from pathlib import Path

__all__ = [
    "check_to_tests_map",
    "bare_test_func_to_file",
    "resolve_one_enforcer",
]

# Matches `def <name>(` at line start (module-level test function definitions).
_DEF_RE = re.compile(r"^def (test_\w+)\(", re.MULTILINE)


@functools.lru_cache(maxsize=None)
def _func_to_file_index(tests_dir_str: str) -> dict[str, str | None]:
    """Build `{test_func_name: rel_file | None}` in one pass, memoized per dir.

    A name mapped to None means AMBIGUOUS: two or more files define a function
    with that name (fail-closed, per the original per-call semantics). Built once
    per tests_dir per process; a fresh process starts with an empty cache and
    nothing is persisted to disk (deterministic within a run).
    """
    index: dict[str, str | None] = {}
    tests_dir = Path(tests_dir_str)
    if not tests_dir.exists():
        return index
    for test_file in sorted(tests_dir.glob("test_*.py")):
        try:
            src = test_file.read_text(encoding="utf-8")
        except Exception:
            continue
        rel = f"tests/{test_file.name}"
        for name in set(_DEF_RE.findall(src)):
            if name in index:
                index[name] = None  # collision -> ambiguous, fail-closed
            else:
                index[name] = rel
    return index


def check_to_tests_map(tests_dir: Path) -> dict[str, list[str]]:
    """Canon: §Closure — check_* name -> test files that reference it (bare grep)."""
    check_to_tests: dict[str, list[str]] = {}
    if not tests_dir.exists():
        return check_to_tests
    for test_file in sorted(tests_dir.glob("test_*.py")):
        try:
            test_src = test_file.read_text(encoding="utf-8")
        except Exception:
            continue
        rel = f"tests/{test_file.name}"
        for check_name in re.findall(r"\bcheck_\w+", test_src):
            check_to_tests.setdefault(check_name, [])
            if rel not in check_to_tests[check_name]:
                check_to_tests[check_name].append(rel)
    return check_to_tests


def bare_test_func_to_file(name: str, tests_dir: Path) -> str | None:
    """Canon: §Closure — bare `test_foo` function name -> owning test file (rel path).

    Returns None if zero or more than one file defines a function with that
    name (ambiguous == unresolved, fail-closed).
    """
    if not tests_dir.exists():
        return None
    return _func_to_file_index(str(tests_dir.resolve())).get(name)


def resolve_one_enforcer(
    entry: str,
    check_to_tests: dict[str, list[str]],
    tests_dir: Path,
) -> list[str] | None:
    """Canon: §Closure — resolve a single `enforced_by` string to pytest node-id(s), or None (unresolved)."""
    entry = entry.strip()

    # Rule 1 + 2: already a test path (file.py or file.py::func), possibly
    # prefixed with "tests/" or bare.
    if entry.endswith(".py") or "::" in entry:
        basename = entry.split("/")[-1]
        file_part = basename.split("::")[0]
        if (tests_dir / file_part).exists():
            return [f"tests/{basename}"]
        return None

    # Rule 3: check_* name -> tests that reference it.
    if entry.startswith("check_"):
        hits = check_to_tests.get(entry)
        if hits:
            return list(hits)
        return None

    # Rule 4: bare test_* function name -> owning file.
    if entry.startswith("test_"):
        owner = bare_test_func_to_file(entry, tests_dir)
        if owner:
            return [owner]
        return None

    # Rule 5: anything else is unresolved.
    return None
