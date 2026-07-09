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
import json
import re
from pathlib import Path

from hotam_spec.repo_paths import runtime_root as _runtime_root

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


# ---------------------------------------------------------------------------
# Persistent mtime-indexed test scan (cross-process cache, Part 1 of wave 6.3).
#
# _func_to_file_index and _check_to_tests_index both walk the same test_*.py
# files with the same AST parse — so they are unified into a single _TestScan
# built once per process (lru_cache) AND cached cross-process in
# .runtime/enforcer-index.json keyed by a fingerprint of the test directory.
#
# The fingerprint is (max_mtime_ns, file_count) — if NO test file changed mtime
# since the last process wrote the index, the cached scan is reused verbatim,
# skipping the glob + AST-parse loop entirely. If any file was touched, the
# scan is rebuilt and re-persisted. This is the same invalidation strategy as
# a build system (make/cargo): mtime is a sufficient staleness signal for
# deterministic, filesystem-read-only computation.
#
# The persistent cache is OPT-IN: only the canonical spec/tests directory
# participates (identified by path equality with repo_paths.tests_root()).
# Arbitrary tmpdirs (test fixtures) always get a fresh scan — their mtime
# fingerprint would be unstable across test runs and the cache adds no value.
# ---------------------------------------------------------------------------

_INDEX_PATH: Path | None = None


def _index_file() -> Path:
    """Lazily resolve the persistent index path under .runtime/."""
    global _INDEX_PATH
    if _INDEX_PATH is None:
        _INDEX_PATH = _runtime_root() / "enforcer-index.json"
    return _INDEX_PATH


class _TestScan:
    """Frozen snapshot of a test-directory scan: func index + check→tests map."""

    __slots__ = ("func_index", "check_map")

    def __init__(
        self,
        func_index: dict[str, str | None],
        check_map: dict[str, list[str]],
    ) -> None:
        self.func_index = func_index
        self.check_map = check_map


def _compute_fingerprint(tests_dir: Path) -> tuple[int, int] | None:
    """Return (max_mtime_ns, file_count) for test_*.py, or None if dir absent.

    The fingerprint is a cheap staleness signal: if it matches the previously
    recorded value, the AST scan cannot have changed (deterministic read of
    the same bytes). A file_count change (add/delete) also invalidates.
    """
    if not tests_dir.exists():
        return None
    test_files = list(tests_dir.glob("test_*.py"))
    if not test_files:
        return (0, 0)
    max_mtime = max(int(f.stat().st_mtime_ns) for f in test_files)
    return (max_mtime, len(test_files))


def _build_scan_uncached(tests_dir: Path) -> _TestScan:
    """Build a _TestScan from scratch (glob + AST parse). No caching."""
    func_index: dict[str, str | None] = {}
    check_map: dict[str, list[str]] = {}
    if not tests_dir.exists():
        return _TestScan(func_index, check_map)
    for test_file in sorted(tests_dir.glob("test_*.py")):
        tree = _cached_parse_path(str(test_file.resolve()))
        if tree is None:
            continue
        rel = f"tests/{test_file.name}"
        for name in _ast_defined_test_funcs(tree):
            if name in func_index:
                func_index[name] = None  # collision -> ambiguous, fail-closed
            else:
                func_index[name] = rel
        for name in _ast_referenced_names(tree):
            if name.startswith("check_"):
                check_map.setdefault(name, [])
                if rel not in check_map[name]:
                    check_map[name].append(rel)
    return _TestScan(func_index, check_map)


def _load_persistent_scan(tests_dir_str: str, fingerprint: tuple[int, int]) -> _TestScan | None:
    """Load a scan from .runtime/enforcer-index.json if the fingerprint matches.

    Returns None if the index file is absent, corrupt, stale (fingerprint
    mismatch), or if the requested tests_dir is not the canonical spec/tests
    (persistent cache is opt-in for the canonical dir only).
    """
    from hotam_spec.repo_paths import tests_root as _tests_root

    # Opt-in: only the canonical spec/tests dir uses the persistent cache.
    # A tmpdir from a test fixture would write garbage into the shared index.
    if Path(tests_dir_str) != _tests_root().resolve():
        return None
    index_file = _index_file()
    if not index_file.exists():
        return None
    try:
        data = json.loads(index_file.read_text(encoding="utf-8"))
    except (OSError, ValueError):
        return None
    entry = data.get(tests_dir_str)
    if not isinstance(entry, dict):
        return None
    if tuple(entry.get("fingerprint", ())) != fingerprint:
        return None
    func_index = entry.get("func_index")
    check_map = entry.get("check_map")
    if not isinstance(func_index, dict) or not isinstance(check_map, dict):
        return None
    # Restore check_map lists (JSON round-trips them as lists already).
    restored_check: dict[str, list[str]] = {
        str(k): list(v) for k, v in check_map.items() if isinstance(v, list)
    }
    restored_func: dict[str, str | None] = {
        str(k): (str(v) if v is not None else None)
        for k, v in func_index.items()
    }
    return _TestScan(restored_func, restored_check)


def _save_persistent_scan(tests_dir_str: str, fingerprint: tuple[int, int], scan: _TestScan) -> None:
    """Persist a scan to .runtime/enforcer-index.json (best-effort, never raises).

    Failures (disk full, permission) are swallowed: the persistent cache is a
    performance optimization, not a correctness dependency — the lru_cache
    layer still guarantees correctness within the process.
    """
    index_file = _index_file()
    try:
        index_file.parent.mkdir(parents=True, exist_ok=True)
        if index_file.exists():
            try:
                data = json.loads(index_file.read_text(encoding="utf-8"))
            except (OSError, ValueError):
                data = {}
        else:
            data = {}
        if not isinstance(data, dict):
            data = {}
        data[tests_dir_str] = {
            "fingerprint": list(fingerprint),
            "func_index": dict(scan.func_index),
            "check_map": {k: list(v) for k, v in scan.check_map.items()},
        }
        index_file.write_text(
            json.dumps(data, sort_keys=True, indent=2, ensure_ascii=False),
            encoding="utf-8",
        )
    except OSError:
        pass


def _load_packaged_scan() -> _TestScan | None:
    """Load the build-time snapshot from package data (§3.6 portability W3).

    Reads ``hotam_spec/_data/enforcer_map.json`` via importlib.resources (PEP
    391) — works for editable, wheel, and vendor-copy installs. Returns None
    if the snapshot is absent or corrupt (callers fall back to an empty scan).

    This is the FALLBACK layer, consulted only when no live tests/ directory
    exists (pip-installed consumer scenario). In self-hosting mode the
    live-scan always wins.
    """
    try:
        from importlib.resources import files as _files
        importlib_path = Path(str(_files("hotam_spec") / "_data" / "enforcer_map.json"))
        if not importlib_path.is_file():
            return None
        data = json.loads(importlib_path.read_text(encoding="utf-8"))
    except (OSError, ValueError):
        return None
    func_index_raw = data.get("func_index")
    check_map_raw = data.get("check_map")
    if not isinstance(func_index_raw, dict) or not isinstance(check_map_raw, dict):
        return None
    func_index: dict[str, str | None] = {
        str(k): (str(v) if v is not None else None)
        for k, v in func_index_raw.items()
    }
    check_map: dict[str, list[str]] = {
        str(k): list(v) for k, v in check_map_raw.items() if isinstance(v, list)
    }
    return _TestScan(func_index, check_map)


@functools.lru_cache(maxsize=None)
def _test_scan_cached(tests_dir_str: str) -> _TestScan:
    """Return the unified test scan for ``tests_dir_str``, cached per process.

    Three resolution layers (first wins):
      1. lru_cache (intra-process): free, always on.
      2. persistent mtime-index (cross-process): opt-in for canonical tests dir.
      3. live-scan of ``tests_dir`` (the original self-hosting behavior).

    When the tests_dir does NOT exist (pip-installed framework — consumer has
    no spec/tests/), the live-scan returns empty, so a FOURTH fallback kicks in:
    the packaged snapshot at ``hotam_spec/_data/enforcer_map.json`` (read via
    importlib.resources, §3.6 portability W3). This snapshot is generated at
    build time by tools/gen_enforcer_map.py against the canonical spec/tests.

    The package-data snapshot is ONLY consulted when tests_dir is absent or
    empty — it never overrides a real live-scan (dev-cycle freshness wins).
    """
    tests_dir = Path(tests_dir_str)
    fingerprint = _compute_fingerprint(tests_dir)
    if fingerprint is not None:
        cached = _load_persistent_scan(tests_dir_str, fingerprint)
        if cached is not None:
            return cached
        scan = _build_scan_uncached(tests_dir)
        if fingerprint is not None:
            _save_persistent_scan(tests_dir_str, fingerprint, scan)
        return scan
    # tests_dir absent or empty → fallback to packaged snapshot (§3.6 W3).
    # This covers the pip-installed-consumer scenario where spec/tests/ is
    # not shipped. The snapshot is a build-time artifact, never the source of
    # truth in self-hosting mode.
    packaged = _load_packaged_scan()
    if packaged is not None:
        return packaged
    # No tests dir AND no packaged snapshot (shouldn't happen in a built
    # wheel, but be defensive): return an empty scan rather than crash.
    return _TestScan({}, {})


@functools.lru_cache(maxsize=None)
def _func_to_file_index(tests_dir_str: str) -> dict[str, str | None]:
    """Build `{test_func_name: rel_file | None}` via AST, memoized per dir.

    A name mapped to None means AMBIGUOUS: two or more files define a function
    with that name (fail-closed). Uses AST to find real `def test_*` at module
    level — a name in a comment or docstring does NOT count.
    """
    scan = _test_scan_cached(tests_dir_str)
    return dict(scan.func_index)


@functools.lru_cache(maxsize=None)
def _check_to_tests_index(tests_dir_str: str) -> dict[str, list[str]]:
    """Build `{check_name: [test_files]}` via AST, memoized per dir.

    Cached alongside _func_to_file_index — both derive from the same scan.
    """
    scan = _test_scan_cached(tests_dir_str)
    # Deep-copy the lists so callers cannot mutate the cached structures.
    return {k: list(v) for k, v in scan.check_map.items()}


def check_to_tests_map(tests_dir: Path) -> dict[str, list[str]]:
    """Canon: §Closure — check_* name -> test files that actually reference it in code (AST-strict).

    A check_* name is considered referenced by a test file only if it appears
    as a real Python identifier (Name or Attribute node) in the AST — NOT if
    it merely appears in a comment, docstring, or string literal.
    """
    if not tests_dir.exists():
        return {}
    return _check_to_tests_index(str(tests_dir.resolve()))


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
