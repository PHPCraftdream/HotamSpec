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
  3. "check_<name>"                   -> resolved via AST scan of test files
                                          for actual call-site usage of the
                                          check_* name (not mere text mention
                                          in comments/docstrings).
  4. "test_<name>" (bare function)    -> AST scan for `def test_<name>(`
                                          and use the owning file as a node-id.
  5. anything else                    -> UNRESOLVED.

Deterministic: no timestamps/randomness; pure filesystem read + AST parse.

AST-strict (Enf#4 fix): names mentioned only in comments, docstrings, or
string literals do NOT count as resolution targets. Only real `def` statements
(for test functions) and real call-site usage (for check_* names) count.
"""

from __future__ import annotations

import ast
import functools
import re
from pathlib import Path

__all__ = [
    "check_to_tests_map",
    "bare_test_func_to_file",
    "resolve_one_enforcer",
    "test_func_has_teeth",
]

# Matches `def <name>(` at line start (module-level test function definitions).
_DEF_RE = re.compile(r"^def (test_\w+)\(", re.MULTILINE)


@functools.lru_cache(maxsize=None)
def _cached_parse_path(path_str: str) -> ast.Module | None:
    """Parse a source file's AST, memoized by absolute path (intra-process).

    Same cache pattern as invariants._cached_parse_path — but kept here to
    avoid circular imports (invariants imports enforcer_resolution).
    Returns None on OSError/SyntaxError so callers keep their skip semantics.
    """
    try:
        source = Path(path_str).read_text(encoding="utf-8")
        return ast.parse(source)
    except (OSError, SyntaxError):
        return None


def _ast_defined_test_funcs(tree: ast.Module) -> set[str]:
    """Return set of test_* function names defined at module top level via AST."""
    names: set[str] = set()
    for node in ast.iter_child_nodes(tree):
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_"):
            names.add(node.name)
    return names


def _ast_called_names(tree: ast.Module) -> set[str]:
    """Return set of all names that appear as call targets anywhere in the AST.

    Catches both bare calls like `check_foo(g)` and attribute calls are ignored
    (we want only bare name calls matching check_* pattern).
    """
    names: set[str] = set()
    for node in ast.walk(tree):
        if isinstance(node, ast.Call):
            if isinstance(node.func, ast.Name):
                names.add(node.func.id)
            elif isinstance(node.func, ast.Attribute):
                names.add(node.func.attr)
    return names


def _ast_referenced_names(tree: ast.Module) -> set[str]:
    """Return set of all Name nodes that appear in code (not in strings/comments).

    This captures imports, variable references, function calls, etc. —
    anything that is a real Python identifier usage, not a string mention.
    """
    names: set[str] = set()
    for node in ast.walk(tree):
        if isinstance(node, ast.Name):
            names.add(node.id)
        elif isinstance(node, ast.Attribute):
            names.add(node.attr)
    return names


@functools.lru_cache(maxsize=None)
def _func_to_file_index(tests_dir_str: str) -> dict[str, str | None]:
    """Build `{test_func_name: rel_file | None}` via AST, memoized per dir.

    A name mapped to None means AMBIGUOUS: two or more files define a function
    with that name (fail-closed). Uses AST to find real `def test_*` at module
    level — a name in a comment or docstring does NOT count.
    """
    index: dict[str, str | None] = {}
    tests_dir = Path(tests_dir_str)
    if not tests_dir.exists():
        return index
    for test_file in sorted(tests_dir.glob("test_*.py")):
        tree = _cached_parse_path(str(test_file.resolve()))
        if tree is None:
            continue
        rel = f"tests/{test_file.name}"
        for name in _ast_defined_test_funcs(tree):
            if name in index:
                index[name] = None  # collision -> ambiguous, fail-closed
            else:
                index[name] = rel
    return index


def check_to_tests_map(tests_dir: Path) -> dict[str, list[str]]:
    """Canon: §Closure — check_* name -> test files that actually reference it in code (AST-strict).

    A check_* name is considered referenced by a test file only if it appears
    as a real Python identifier (Name or Attribute node) in the AST — NOT if
    it merely appears in a comment, docstring, or string literal.
    """
    check_to_tests: dict[str, list[str]] = {}
    if not tests_dir.exists():
        return check_to_tests
    for test_file in sorted(tests_dir.glob("test_*.py")):
        tree = _cached_parse_path(str(test_file.resolve()))
        if tree is None:
            continue
        rel = f"tests/{test_file.name}"
        referenced = _ast_referenced_names(tree)
        for name in referenced:
            if name.startswith("check_"):
                check_to_tests.setdefault(name, [])
                if rel not in check_to_tests[name]:
                    check_to_tests[name].append(rel)
    return check_to_tests


def bare_test_func_to_file(name: str, tests_dir: Path) -> str | None:
    """Canon: §Closure — bare `test_foo` function name -> owning test file (rel path).

    Returns None if zero or more than one file defines a function with that
    name (ambiguous == unresolved, fail-closed). Uses AST — only real `def`
    statements count, not text mentions.
    """
    if not tests_dir.exists():
        return None
    return _func_to_file_index(str(tests_dir.resolve())).get(name)


def test_func_has_teeth(func_name: str, tests_dir: Path) -> bool | None:
    """Canon: §Closure — check if a test function has real assertions (not just pass/docstring).

    Returns True if the test has teeth (contains assert, pytest.raises, or
    calls that could raise). Returns False if the body is only
    pass/docstring/ellipsis with no assert and no function calls.
    Returns None if the function cannot be found.

    A test "has teeth" if its body contains at least one of:
      - ast.Assert node
      - a call to pytest.raises (ast.Call with pytest.raises attribute)
      - any function call at all (conservative: a helper could assert internally)

    A test with ONLY pass, docstring (Expr(Constant(str))), or Ellipsis is toothless.
    """
    if not tests_dir.exists():
        return None

    # Find the file that defines this function
    file_rel = _func_to_file_index(str(tests_dir.resolve())).get(func_name)
    if file_rel is None:
        return None

    file_path = tests_dir / file_rel.removeprefix("tests/")
    tree = _cached_parse_path(str(file_path.resolve()))
    if tree is None:
        return None

    # Find the function def
    for node in ast.iter_child_nodes(tree):
        if isinstance(node, ast.FunctionDef) and node.name == func_name:
            return _body_has_teeth(node.body)

    return None


def _body_has_teeth(body: list[ast.stmt]) -> bool:
    """Check if a function body has real assertions or meaningful calls.

    Returns False only if the body consists entirely of:
      - pass statements
      - docstrings (Expr(Constant(str)))
      - Ellipsis (Expr(Constant(...)))
    with zero assert statements and zero function calls anywhere in the body.
    """
    for node in ast.walk(ast.Module(body=body, type_ignores=[])):
        if isinstance(node, ast.Assert):
            return True
        if isinstance(node, ast.Call):
            return True
        if isinstance(node, ast.Raise):
            return True
    return False


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

    # Rule 3: check_* name -> tests that reference it (AST-strict).
    if entry.startswith("check_"):
        hits = check_to_tests.get(entry)
        if hits:
            return list(hits)
        return None

    # Rule 4: bare test_* function name -> owning file (AST-strict).
    if entry.startswith("test_"):
        owner = bare_test_func_to_file(entry, tests_dir)
        if owner:
            return [owner]
        return None

    # Rule 5: anything else is unresolved.
    return None
