"""Canon: §Invariants — R-project-root-not-hardcoded.

Syntactic guard that no committed framework code (spec/tools/ + spec/src/
hotam_spec/) reaches the CONSUMER's project root by GUESSING it from its own
``__file__`` location — the exact shape the requirement forbids:
``Path(__file__).resolve().parents[N]`` (or an equivalent ``.parent.parent…``
climb) used to reach the consumer's data (``domains/``, ``tickets/``,
``delegations/``, ``CLAUDE.md``, ``.claude/``, ``docs/gen``). Every such path
MUST instead be derived from ``project_paths.project_root()`` /
``project_root_or_raise()``, resolved once per call through the documented
R1–R6 priority chain, so that self-hosting stays a legitimate case while a
separately-installed consumer's root is found honestly rather than assumed to
equal the framework's own install location.

DETECTION HEURISTIC (deliberately narrow — documented boundaries):

  A file OFFENDS iff it contains an ancestor-climb expression that BOTH
    (1) climbs from ``__file__`` (a ``Path(__file__)…parents[N]`` subscript
        with N >= 2, OR a ``.parent.parent…`` chain of depth >= 2 rooted at a
        ``__file__``-bearing expression), AND
    (2) is immediately joined (``/`` operator) to a CONSUMER-DATA segment — a
        string literal that is exactly one of, or begins with one of:
        ``domains``, ``tickets``, ``delegations``, ``CLAUDE.md``, ``.claude``,
        ``docs`` (the ``docs/gen`` generated-docs tree).

Both conditions must co-occur in the SAME expression: it is the JOIN of a
``__file__``-climb to consumer data that constitutes the "guess at the
consumer's files". This is why a bare framework-internal ``parents[1]`` (a tool
locating ``spec/`` to fix up ``sys.path``) and ``repo_paths``' own framework-
root climb (which serves framework-internal paths, never consumer data) are
NOT flagged: they never reach for consumer data by guessing.

BOUNDARIES (intentionally NOT modeled — honest scope):
  * Top-level ``*.py`` only (non-recursive glob), matching the sibling
    R-work-within-launch-dir / R-core-periphery scanners. The ``cli/``
    subpackage's ``_path_setup.py`` computes a framework-internal bundled
    ``_tools`` path (not consumer data) and is out of the swept set anyway.
  * We do not do data-flow across statements: the climb and the consumer-data
    join must appear in one expression. A file that binds ``root = …parents[2]``
    and later joins ``root / "domains"`` two lines down is not caught — but the
    live codebase writes none such (W1 migration routed every consumer-root
    read through ``project_root_or_raise()``), and the one-expression form is
    the exact literal shape the requirement's claim names.
  * ``project_paths.py`` (the sanctioned resolver) is allowlisted: it is the
    ONE place permitted to compute roots, though in fact it climbs only via
    ``repo_paths.repo_root()`` and never joins consumer data itself.

ALLOWLIST: ``project_paths.py`` — the single sanctioned project-root resolver.
If a future module ever legitimately must compute a consumer path from
``__file__`` under an explicit steward decision, add its basename here WITH a
one-line rationale so the exception is visible and reviewable.
"""

from __future__ import annotations

import ast
from pathlib import Path

_SPEC_ROOT = Path(__file__).resolve().parents[1]
_TOOLS_DIR = _SPEC_ROOT / "tools"
_HOTAM_SPEC_SRC = _SPEC_ROOT / "src" / "hotam_spec"

# Basenames sanctioned to compute a consumer path from __file__. project_paths
# is the single resolver (R1–R6) permitted to speak about roots at all.
# Format: "<basename>": "<rationale>".
_ALLOWLIST: dict[str, str] = {
    "project_paths.py": "the single sanctioned project-root resolver (R1-R6 chain)",
}

# Consumer-data path segments: reaching ANY of these by climbing from __file__
# is the forbidden "guess at the consumer's files". Matched as a whole segment
# or a prefix (so "docs" catches "docs/gen", ".claude" catches ".claude/...").
_CONSUMER_SEGMENTS: tuple[str, ...] = (
    "domains",
    "tickets",
    "delegations",
    "CLAUDE.md",
    ".claude",
    "docs",
)

# Minimum ancestor-climb depth that reaches from a spec/tools/*.py or
# spec/src/hotam_spec/*.py file up to (or above) the repo/consumer root.
# parents[0]/parents[1] and a single .parent stay framework-internal (spec/,
# src/); N >= 2 is the depth at which a climb reaches the repo root and beyond.
_MIN_CLIMB_DEPTH = 2


def _segment_is_consumer_data(value: object) -> bool:
    """True iff a join-RHS literal names consumer data (whole seg or prefix)."""
    if not isinstance(value, str):
        return False
    head = value.replace("\\", "/").split("/", 1)[0]
    return head in _CONSUMER_SEGMENTS


def _references_file_dunder(node: ast.AST) -> bool:
    """True iff the subtree contains a ``__file__`` name reference."""
    for sub in ast.walk(node):
        if isinstance(sub, ast.Name) and sub.id == "__file__":
            return True
    return False


def _file_climb_depth(node: ast.expr) -> int | None:
    """Depth of a ``__file__``-rooted ancestor climb, or ``None`` if not one.

    Recognizes two spellings, both requiring a ``__file__`` in the subtree:
      * ``<expr>.parents[N]``  → depth N,
      * ``<expr>.parent`` chains → depth = number of chained ``.parent``.
    Returns the climb depth so the caller can require depth >= _MIN_CLIMB_DEPTH.
    """
    # <expr>.parents[N]
    if isinstance(node, ast.Subscript):
        val = node.value
        if (
            isinstance(val, ast.Attribute)
            and val.attr == "parents"
            and _references_file_dunder(val)
        ):
            idx = node.slice
            if isinstance(idx, ast.Constant) and isinstance(idx.value, int):
                return idx.value
    # <expr>.parent.parent…  (count the chained .parent attrs)
    if isinstance(node, ast.Attribute) and node.attr == "parent":
        depth = 0
        cur: ast.expr = node
        while isinstance(cur, ast.Attribute) and cur.attr == "parent":
            depth += 1
            cur = cur.value
        if _references_file_dunder(cur):
            return depth
    return None


def _binop_offends(node: ast.BinOp) -> bool:
    """True iff a ``<file-climb> / <consumer-segment>`` join, at any nesting.

    Path joins chain left-associatively: ``base / "domains" / name`` parses as
    ``(base / "domains") / name``. We walk down the left spine of ``/`` BinOps
    and, at each ``/``, check whether the left operand is a deep-enough
    ``__file__`` climb and the right operand a consumer-data literal.
    """
    if not isinstance(node.op, ast.Div):
        return False
    # Right operand names consumer data?
    right = node.right
    right_is_consumer = isinstance(right, ast.Constant) and _segment_is_consumer_data(
        right.value
    )
    if right_is_consumer:
        depth = _file_climb_depth(node.left)
        if depth is not None and depth >= _MIN_CLIMB_DEPTH:
            return True
    return False


def _file_offends(tree: ast.Module) -> bool:
    """True iff any ``/``-join in the module reaches consumer data from a
    deep ``__file__`` climb (see module docstring)."""
    for sub in ast.walk(tree):
        if isinstance(sub, ast.BinOp) and _binop_offends(sub):
            return True
    return False


def _scanned_files() -> list[Path]:
    """Every committed framework .py under spec/tools/ + spec/src/hotam_spec/
    (top level, non-recursive), excluding the allowlist."""
    out: list[Path] = []
    for root in (_TOOLS_DIR, _HOTAM_SPEC_SRC):
        for path in sorted(root.glob("*.py")):
            if path.name in _ALLOWLIST:
                continue
            out.append(path)
    return out


def test_no_committed_code_guesses_consumer_root_from_file() -> None:
    """AST-scan every committed tool/src module: none may reach the consumer's
    data (domains/, tickets/, delegations/, CLAUDE.md, .claude/, docs/gen) by
    climbing ``Path(__file__).resolve().parents[N]`` (or ``.parent.parent…``).
    Consumer roots come from ``project_paths.project_root()``; framework-
    internal paths (spec/, src/) may still climb __file__ shallowly. This is
    the machine-checkable form of R-project-root-not-hardcoded.
    """
    scanned = _scanned_files()
    assert scanned, f"No .py files found under {_TOOLS_DIR} / {_HOTAM_SPEC_SRC}"

    offenders = [
        str(p.relative_to(_SPEC_ROOT))
        for p in scanned
        if _file_offends(ast.parse(p.read_text(encoding="utf-8"), filename=str(p)))
    ]

    assert not offenders, (
        "Committed framework code reaches the consumer's data by GUESSING the "
        "project root from __file__ (Path(__file__).resolve().parents[N] / "
        "'domains' and the like) -- R-project-root-not-hardcoded forbids this. "
        "Derive the path from project_paths.project_root_or_raise() instead, or "
        "(if genuinely sanctioned) add the file to _ALLOWLIST with a rationale. "
        "Offenders:\n" + "\n".join(offenders)
    )


def test_scanner_catches_a_hardcoded_root_negative_control() -> None:
    """Non-vacuity guard: synthetic modules that guess the consumer root from
    __file__ ARE flagged by the SAME predicate the live test relies on, while
    in-bounds shapes (a shallow framework-internal climb, a climb joined to a
    NON-consumer segment, and a consumer path derived from project_root) are
    NOT. Guards against the positive test passing because the scanner silently
    stopped matching.
    """
    # OFFENDING: parents[2] climb joined straight to the consumer's domains/.
    bad_parents = ast.parse(
        "from pathlib import Path\n"
        "d = Path(__file__).resolve().parents[2] / 'domains'\n"
    )
    assert _file_offends(bad_parents)

    # OFFENDING: parents[3] climb joined to CLAUDE.md.
    bad_claude = ast.parse(
        "from pathlib import Path\n"
        "c = Path(__file__).resolve().parents[3] / 'CLAUDE.md'\n"
    )
    assert _file_offends(bad_claude)

    # OFFENDING: .parent.parent climb joined to tickets/, then a subpath.
    bad_chain = ast.parse(
        "from pathlib import Path\n"
        "t = Path(__file__).resolve().parent.parent / 'tickets' / 'backlog'\n"
    )
    assert _file_offends(bad_chain)

    # OFFENDING: reaches docs/gen (generated-docs tree is consumer docs).
    bad_docs = ast.parse(
        "from pathlib import Path\n"
        "g = Path(__file__).resolve().parents[2] / 'docs' / 'gen'\n"
    )
    assert _file_offends(bad_docs)

    # IN-BOUNDS: shallow framework-internal climb to spec/ + a fix-up of
    # sys.path via src/ -- reaches framework code, not consumer data.
    ok_spec = ast.parse(
        "from pathlib import Path\n"
        "spec = Path(__file__).resolve().parents[1]\n"
        "src = spec / 'src'\n"
    )
    assert not _file_offends(ok_spec)

    # IN-BOUNDS: a deep climb joined to a NON-consumer segment (framework-
    # internal bundled _tools, exactly cli/_path_setup.py's shape).
    ok_tools = ast.parse(
        "from pathlib import Path\n"
        "bundled = Path(__file__).resolve().parents[2] / '_tools'\n"
    )
    assert not _file_offends(ok_tools)

    # IN-BOUNDS: the sanctioned path -- consumer data derived from the resolver,
    # no __file__ climb at all.
    ok_resolved = ast.parse(
        "from hotam_spec.project_paths import project_root_or_raise\n"
        "d = project_root_or_raise() / 'domains'\n"
    )
    assert not _file_offends(ok_resolved)

    # IN-BOUNDS: a bare deep climb NOT joined to anything (repo_paths computes
    # its framework root then serves framework-internal paths elsewhere).
    ok_bare_climb = ast.parse(
        "from pathlib import Path\n"
        "repo = Path(__file__).resolve().parents[3]\n"
    )
    assert not _file_offends(ok_bare_climb)
