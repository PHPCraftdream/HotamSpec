"""Canon: §Invariants — R-work-within-launch-dir (write-vector half).

Syntactic guard that no committed framework code (spec/tools/ + spec/src/
hotam_spec/) writes into the sovereign host home — the user's GLOBAL
``~/.claude`` config or anything else under ``Path.home()`` /
``os.path.expanduser("~")``. This is the machine-checkable HALF of
R-work-within-launch-dir: it catches the *committed-code write vector* — the
exact shape of the real historical incident, where
``setup_context_hook.py --patch-global`` physically patched
``~/.claude/cah-bin/bin/cah-status.js`` (steward verdict 2026-07-05). It does
NOT and cannot catch a live agent shelling out through bash at runtime; that
residual stays discipline-of-prose. Honest partial coverage, not theatre: the
incident that motivated the rule was precisely a committed tool, so the vector
this scan closes is the one that actually fired.

DETECTION HEURISTIC (deliberately conservative — documented boundaries):

  A file OFFENDS iff it BOTH
    (1) references a home-rooted path — any of:
          * ``Path.home()`` (or ``<x>.home()`` on a Path),
          * ``os.path.expanduser("~...")`` / ``expanduser("~...")``,
          * a string/bytes literal that STARTS WITH ``~/`` — i.e. a genuine
            home-rooted *path* literal, not a prose mention. A docstring or
            message that merely NAMES ``~/.claude`` mid-sentence (e.g. "never
            touches ~/.claude") does not start with ``~/`` and is deliberately
            NOT treated as a path reference — otherwise every honest "we do not
            touch the home" doc-line would trip the guard.
    AND
    (2) contains at least one WRITE sink anywhere in the same file — a call to
          open(..., mode) with 'w'/'a'/'x'/'+' in mode, or a method call named
          one of: write_text, write_bytes, mkdir, touch, rmtree, unlink,
          replace, rename, symlink_to, chmod, makedirs, remove, rmdir, copy,
          copy2, copyfile, copytree, move.

BOUNDARIES (intentionally NOT modeled — honest scope):
  * We do not do data-flow: (1) and (2) need only co-occur in the SAME file,
    not be proven to reach the same path. A file that reads ~/.claude AND
    (separately) writes only inside the repo is flagged conservatively — the
    correct fix is to route the home-read through a named, reviewed helper or
    add the file to _ALLOWLIST with a rationale. This over-approximates on
    purpose: a home reference next to any writer is a smell worth a human look.
  * Reads of the host home that never co-occur with a writer are allowed
    (e.g. a docstring mentioning ~/.claude, or a pure status reader).
  * Runtime bash / subprocess writes are out of scope (prose discipline).

ALLOWLIST: empty today. Every current tool that manages Claude config writes
into the CONSUMER project root's ``<repo>/.claude/`` (resolved via
``project_root_or_raise()``), never the home global — setup_hooks.py /
setup_context_hook.py / _claude_settings.py all target ``_REPO_ROOT/.claude``,
so none reference a home path at all and none need allowlisting. The
``--patch-global`` mechanism that DID write ~/.claude was removed
(R-work-within-launch-dir, 2026-07-05). If a future tool ever legitimately
must touch the host home under an explicit steward decision, add its filename
here WITH a one-line rationale so the exception is visible and reviewable.
"""

from __future__ import annotations

import ast
from pathlib import Path

_SPEC_ROOT = Path(__file__).resolve().parents[1]
_TOOLS_DIR = _SPEC_ROOT / "tools"
_HOTAM_SPEC_SRC = _SPEC_ROOT / "src" / "hotam_spec"

# Filenames (basename) sanctioned to reference a home path next to a writer.
# Empty today — see module docstring. Format: "<basename>": "<rationale>".
_ALLOWLIST: dict[str, str] = {}

# Method-call names that mutate the filesystem (Path / os / shutil surface).
_WRITE_METHODS = frozenset(
    {
        "write_text",
        "write_bytes",
        "mkdir",
        "makedirs",
        "touch",
        "rmtree",
        "unlink",
        "remove",
        "rmdir",
        "replace",
        "rename",
        "symlink_to",
        "chmod",
        "copy",
        "copy2",
        "copyfile",
        "copytree",
        "move",
    }
)

# open(...) modes that create/modify a file rather than read it.
_WRITE_OPEN_CHARS = frozenset({"w", "a", "x", "+"})


def _references_home(tree: ast.Module) -> bool:
    """True iff the module names a home-rooted path (see docstring rule (1))."""
    for node in ast.walk(tree):
        # Path.home() / x.home()
        if isinstance(node, ast.Call) and isinstance(node.func, ast.Attribute):
            if node.func.attr == "home":
                return True
            # expanduser("~...") called as a method (os.path.expanduser)
            if node.func.attr == "expanduser" and _first_arg_is_home(node):
                return True
        # bare expanduser("~...")
        if isinstance(node, ast.Call) and isinstance(node.func, ast.Name):
            if node.func.id == "expanduser" and _first_arg_is_home(node):
                return True
        # string / bytes literal starting with ~/ or naming ~/.claude
        if isinstance(node, ast.Constant):
            val = node.value
            if isinstance(val, bytes):
                try:
                    val = val.decode("utf-8", "ignore")
                except Exception:  # pragma: no cover - defensive
                    continue
            # Only a literal that STARTS WITH ~/ is a path; a mid-sentence
            # ~/.claude mention (docstrings, messages) is prose, not a path.
            if isinstance(val, str) and val.startswith("~/"):
                return True
    return False


def _first_arg_is_home(call: ast.Call) -> bool:
    """expanduser(...) whose first positional arg is a "~..."-prefixed literal."""
    if not call.args:
        # expanduser() with no literal arg (e.g. a variable) — treat as a home
        # reference conservatively; expanduser exists only to expand ~.
        return True
    first = call.args[0]
    if isinstance(first, ast.Constant) and isinstance(first.value, str):
        return first.value.startswith("~")
    # Non-literal argument to expanduser: still a home-expansion call.
    return True


def _has_write_sink(tree: ast.Module) -> bool:
    """True iff the module contains any filesystem-write call (rule (2))."""
    for node in ast.walk(tree):
        if not isinstance(node, ast.Call):
            continue
        func = node.func
        # <x>.write_text(...) / .mkdir(...) / shutil.move(...) etc.
        if isinstance(func, ast.Attribute) and func.attr in _WRITE_METHODS:
            return True
        # open(..., "w"/"a"/...) — either builtin open or Path.open
        is_open = (isinstance(func, ast.Name) and func.id == "open") or (
            isinstance(func, ast.Attribute) and func.attr == "open"
        )
        if is_open and _open_is_write(node):
            return True
    return False


def _open_is_write(call: ast.Call) -> bool:
    """True iff an open(...) call carries a write/append/create mode."""
    mode = None
    if len(call.args) >= 2 and isinstance(call.args[1], ast.Constant):
        mode = call.args[1].value
    for kw in call.keywords:
        if kw.arg == "mode" and isinstance(kw.value, ast.Constant):
            mode = kw.value.value
    if not isinstance(mode, str):
        return False
    return any(ch in _WRITE_OPEN_CHARS for ch in mode)


def _scanned_files() -> list[Path]:
    """Every committed framework .py under spec/tools/ + spec/src/hotam_spec/,
    excluding __pycache__ and the allowlist."""
    out: list[Path] = []
    for root in (_TOOLS_DIR, _HOTAM_SPEC_SRC):
        for path in sorted(root.glob("*.py")):
            if path.name in _ALLOWLIST:
                continue
            out.append(path)
    return out


def _file_offends(path: Path) -> bool:
    tree = ast.parse(path.read_text(encoding="utf-8"), filename=str(path))
    return _references_home(tree) and _has_write_sink(tree)


def test_no_committed_code_writes_into_the_host_home() -> None:
    """AST-scan every committed tool/src module: none may co-locate a home-path
    reference (Path.home() / expanduser("~") / a ~/.claude literal) with a
    filesystem writer. This closes the committed-code write vector of
    R-work-within-launch-dir — the exact shape of the removed --patch-global
    incident. (Live-agent bash writes remain prose discipline.)
    """
    scanned = _scanned_files()
    assert scanned, f"No .py files found under {_TOOLS_DIR} / {_HOTAM_SPEC_SRC}"

    offenders = [str(p.relative_to(_SPEC_ROOT)) for p in scanned if _file_offends(p)]

    assert not offenders, (
        "Committed framework code references a host-home path next to a "
        "filesystem writer -- a potential ~/.claude write, which "
        "R-work-within-launch-dir forbids absent an explicit steward decision. "
        "Route the write into the launch/repo directory, or (if genuinely "
        "sanctioned) add the file to _ALLOWLIST with a rationale. Offenders:\n"
        + "\n".join(offenders)
    )


def test_scanner_catches_a_home_write_negative_control() -> None:
    """Non-vacuity guard: a synthetic module that expands ~/.claude and writes
    to it is flagged by the SAME predicate the live test relies on, and three
    in-bounds shapes (repo-relative write, a bare ~/.claude mention with no
    writer, a home read with no writer) are NOT flagged. Guards against the
    positive test passing because the scanner silently stopped matching.
    """
    # OFFENDING: expands ~ and writes there.
    bad_expand = ast.parse(
        "import os\n"
        "p = os.path.expanduser('~/.claude/cah-bin/cah-status.js')\n"
        "open(p, 'w').write('patched')\n"
    )
    assert _references_home(bad_expand) and _has_write_sink(bad_expand)

    # OFFENDING: Path.home() + write_text.
    bad_home = ast.parse(
        "from pathlib import Path\n"
        "(Path.home() / '.claude' / 'x.json').write_text('{}')\n"
    )
    assert _references_home(bad_home) and _has_write_sink(bad_home)

    # OFFENDING: bare ~/.claude literal + mkdir.
    bad_literal = ast.parse(
        "from pathlib import Path\n"
        "d = Path('~/.claude/cache')\n"
        "d.mkdir(parents=True)\n"
    )
    assert _references_home(bad_literal) and _has_write_sink(bad_literal)

    # IN-BOUNDS: writes only to a repo-relative path, no home reference.
    ok_repo = ast.parse(
        "from pathlib import Path\n"
        "(Path('.claude') / 'settings.json').write_text('{}')\n"
    )
    assert not _references_home(ok_repo)

    # IN-BOUNDS: a prose ~/.claude mention (mid-sentence, does not start with
    # ~/) is NOT a path reference -- even sitting next to a writer, so an honest
    # "never touches ~/.claude" docstring beside a repo-write does not trip.
    ok_mention = ast.parse(
        "from pathlib import Path\n"
        "MSG = 'this tool never touches ~/.claude or the host statusline'\n"
        "Path('out.json').write_text(MSG)\n"
    )
    assert not _references_home(ok_mention) and _has_write_sink(ok_mention)

    # IN-BOUNDS: reads the host home (status check), never writes.
    ok_read = ast.parse(
        "from pathlib import Path\n"
        "exists = (Path.home() / '.claude' / 'settings.json').exists()\n"
        "print(exists)\n"
    )
    assert _references_home(ok_read) and not _has_write_sink(ok_read)
