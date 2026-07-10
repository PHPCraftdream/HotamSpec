"""Canon: §Graph — project-root resolution for the consumer's data directory.

R-project-root-not-hardcoded: this module resolves WHERE the consumer's data
lives (domains/, tickets/, delegations/, CLAUDE.md, .claude/) — NOT where the
framework itself lives. The framework-internal paths (spec/, tests/, tools/)
are served by ``repo_paths.repo_root()``; this module is a logically separate
accessor for the project-root concept.

The two accessors COINCIDE in self-hosting mode (the HotamSpec repo models
itself: framework repo == consumer repo, so ``project_root()`` falls back to
``repo_paths.repo_root()`` at R6). They DIVERGE when a consumer installs the
framework from PyPI/git/vendor-copy and works in their own repo: then
``project_root()`` resolves the consumer's repo, while ``repo_root()`` stays
at the framework install location.

The resolution chain (first non-empty result wins):

  R1 — ``HOTAM_SPEC_PROJECT_ROOT`` env (must exist, must be a directory).
  R2 — ``HOTAM_SPEC_DOMAINS_ROOT`` env (project = its parent, must exist).
  R3 — markers in CWD, bottom-up search, two tiers:
         * RELIABLE (one alone is enough): ``domains/``, ``delegations/``,
           ``pyproject.toml`` with a ``[tool.hotam-spec]`` table. These are
           specific to a Hotam-Spec project — no unrelated tool creates them.
         * SECONDARY (need 2+, from RELIABLE+SECONDARY combined): ``CLAUDE.md``,
           ``.claude/``, ``tickets/``. Any ONE of these alone is too generic —
           any Claude-Code repo has a ``CLAUDE.md``/``.claude/``, and
           ``tickets/`` is a common folder name in unrelated projects — so a
           lone secondary marker must NOT false-trigger R3 for a foreign
           Claude-Code repo that merely happens to have a ``CLAUDE.md``.
  R4 — ``.hotam-spec-project`` marker file, searched bottom-up up to 5 levels
       from CWD.
  R5 — ``pyproject.toml`` → ``[tool.hotam-spec].project_root`` (relative path
       resolved from the pyproject.toml's own directory).
  R6 — self-hosting fallback: ``repo_paths.repo_root()`` (framework repo ==
       consumer repo — the only assumption that holds for the self-modeling
       core and its test suite), gated on CWD actually being inside the
       framework repo. A consumer process running from a directory that is
       NOT inside the framework's own checkout (the normal case for a
       ``pip install``-ed consumer with no markers yet) must NOT silently
       adopt the framework's install location as its project root — see
       R6's guard below and tests/test_e2e_consumer_subprocess.py, which
       caught this the hard way: a fresh, marker-less consumer directory
       used to resolve to the framework repo and write into it.

Uncertainty rule: if the entire chain is exhausted and nothing matched,
``project_root()`` returns ``None`` — it never guesses. Callers that NEED a
path raise ``ProjectRootUnresolved``, which carries a diagnostic listing
which env vars were checked, which markers were searched, and where.

stdlib-only (tomllib is stdlib since Python 3.11; this repo requires 3.12).
No side effects, no imports of domain-specific code.
"""

from __future__ import annotations

import os
import tomllib
from pathlib import Path

from hotam_spec import repo_paths

#: R1 — explicit project root pin (highest priority).
ENV_PROJECT_ROOT = "HOTAM_SPEC_PROJECT_ROOT"

#: R2 — domains-root override (project root = its parent).
ENV_DOMAINS_ROOT = "HOTAM_SPEC_DOMAINS_ROOT"

#: R4 — marker-file name (empty file, its presence declares project root).
MARKER_FILENAME = ".hotam-spec-project"

#: R3 — RELIABLE filesystem markers: one alone is sufficient. Each is specific
#: enough to a Hotam-Spec project that no unrelated tool/repo creates it as a
#: side effect (unlike CLAUDE.md/.claude/tickets, which any Claude-Code repo
#: or unrelated project may carry).
RELIABLE_MARKER_PATHS: tuple[str, ...] = (
    "domains",
    "delegations",
)

#: R3 — SECONDARY filesystem markers: too generic to trust alone (a foreign
#: Claude-Code repo has CLAUDE.md/.claude/; "tickets" is a common folder name
#: in unrelated projects). Two or more markers from RELIABLE+SECONDARY
#: combined are required before a SECONDARY-only match counts.
SECONDARY_MARKER_PATHS: tuple[str, ...] = (
    "CLAUDE.md",
    ".claude",
    "tickets",
)

#: R3 — full filesystem-marker vocabulary (RELIABLE + SECONDARY), kept for
#: diagnostics and backward-compatible enumeration.
MARKER_PATHS: tuple[str, ...] = RELIABLE_MARKER_PATHS + SECONDARY_MARKER_PATHS

#: R4 — how many parent directories to climb searching for the marker file.
#: 5 levels = CWD + 5 parents (covers monorepo subdirectories).
MAX_MARKER_SEARCH_DEPTH = 5

#: R5 — pyproject.toml table name for hotam-spec configuration.
PYPROJECT_TABLE = "hotam-spec"

#: R5 — key inside [tool.hotam-spec] naming the project root (relative path).
PYPROJECT_PROJECT_ROOT_KEY = "project_root"


class ProjectRootUnresolved(RuntimeError):
    """Raised when ``project_root()`` returned ``None`` and a path is needed.

    Carries a diagnostic string built by ``project_root_or_raise()`` that
    lists every source checked (R1 env, R2 env, R3 markers, R4 marker file,
    R5 pyproject) and the directories searched, so the operator can see
    exactly what failed rather than getting a bare "not found".

    Canon: §Graph — the uncertainty-rule exception type.
    """

    def __init__(self, diagnostic: str) -> None:
        self.diagnostic = diagnostic
        super().__init__(diagnostic)


def _env_dir(name: str) -> Path | None:
    """Return a directory from an env var, or ``None`` if unset/invalid.

    Validates: the var is set, non-empty (after strip), points at an existing
    directory. Returns ``None`` on any failure — callers fall through to the
    next link in the chain.
    """
    raw = os.environ.get(name, "").strip()
    if not raw:
        return None
    candidate = Path(raw).resolve()
    if candidate.is_dir():
        return candidate
    return None


#: R3 — minimum count of SECONDARY-only markers before they count as a match.
#: A lone secondary marker (e.g. just CLAUDE.md) is too generic to trust; 2+
#: together are specific enough to a Hotam-Spec project.
SECONDARY_MARKER_MIN_COUNT = 2


def _has_marker(candidate: Path) -> bool:
    """Check whether ``candidate`` counts as an R3 project-root match.

    Two-tier logic: any ONE RELIABLE marker (``domains/``, ``delegations/``,
    or a ``pyproject.toml`` with a ``[tool.hotam-spec]`` table) is sufficient
    on its own. A SECONDARY marker (``CLAUDE.md``, ``.claude/``, ``tickets/``)
    is too generic alone — a foreign Claude-Code repo may carry a lone
    CLAUDE.md with nothing to do with Hotam-Spec — so SECONDARY markers only
    count once ``SECONDARY_MARKER_MIN_COUNT`` (2) or more of them (counted
    across RELIABLE+SECONDARY combined) are present together.

    The pyproject.toml marker is special: it must contain a ``[tool.hotam-spec]``
    table (not just be any pyproject.toml), so a generic pyproject.toml from
    an unrelated project doesn't trigger a false positive.
    """
    for rel in RELIABLE_MARKER_PATHS:
        if (candidate / rel).exists():
            return True
    pyproject_has_table = False
    pyproject = candidate / "pyproject.toml"
    if pyproject.is_file():
        try:
            with open(pyproject, "rb") as f:
                data = tomllib.load(f)
            if "tool" in data and PYPROJECT_TABLE in data.get("tool", {}):
                pyproject_has_table = True
        except (tomllib.TOMLDecodeError, OSError):
            pass
    if pyproject_has_table:
        return True  # RELIABLE — sufficient alone.
    matched = sum(1 for rel in SECONDARY_MARKER_PATHS if (candidate / rel).exists())
    return matched >= SECONDARY_MARKER_MIN_COUNT


def _search_markers_upward(start: Path, max_depth: int) -> Path | None:
    """Search bottom-up from ``start`` for a directory with R3 markers.

    Checks ``start`` itself, then each parent up to ``max_depth`` levels.
    Returns the first directory containing a marker, or ``None``.
    """
    current = start.resolve()
    for _ in range(max_depth + 1):
        if _has_marker(current):
            return current
        if current.parent == current:
            break
        current = current.parent
    return None


def _search_marker_file_upward(start: Path, max_depth: int) -> Path | None:
    """Search bottom-up for ``.hotam-spec-project`` (R4 marker file).

    Returns the directory CONTAINING the marker file (the project root),
    or ``None``.
    """
    current = start.resolve()
    for _ in range(max_depth + 1):
        if (current / MARKER_FILENAME).exists():
            return current
        if current.parent == current:
            break
        current = current.parent
    return None


def _resolve_pyproject(start: Path, max_depth: int) -> Path | None:
    """R5: find a pyproject.toml with ``[tool.hotam-spec].project_root``.

    Searches bottom-up from ``start`` up to ``max_depth`` levels. Returns the
    resolved project_root path (relative to the pyproject.toml directory) if
    it exists and is a directory, otherwise ``None``.
    """
    current = start.resolve()
    for _ in range(max_depth + 1):
        pyproject = current / "pyproject.toml"
        if pyproject.is_file():
            try:
                with open(pyproject, "rb") as f:
                    data = tomllib.load(f)
                hotam_cfg = data.get("tool", {}).get(PYPROJECT_TABLE, {})
                raw_root = hotam_cfg.get(PYPROJECT_PROJECT_ROOT_KEY, "")
                if raw_root:
                    resolved = (current / raw_root).resolve()
                    if resolved.is_dir():
                        return resolved
            except (tomllib.TOMLDecodeError, OSError):
                pass
        if current.parent == current:
            break
        current = current.parent
    return None


def project_root() -> Path | None:
    """Resolve the consumer's project root via the R1–R6 chain.

    Returns the first matching directory or ``None`` if nothing matched
    (the uncertainty rule — never guesses). Callers needing a path should
    use ``project_root_or_raise()`` to get a diagnostic exception on failure.

    Canon: §Graph — project-root resolver entry point.
    """
    # R1 — explicit env pin.
    r1 = _env_dir(ENV_PROJECT_ROOT)
    if r1 is not None:
        return r1

    # R2 — domains-root env (project = its parent).
    domains_dir = _env_dir(ENV_DOMAINS_ROOT)
    if domains_dir is not None:
        parent = domains_dir.parent
        if parent.is_dir():
            return parent

    # R3 — filesystem markers in CWD, bottom-up.
    cwd = Path.cwd()
    r3 = _search_markers_upward(cwd, MAX_MARKER_SEARCH_DEPTH)
    if r3 is not None:
        return r3

    # R4 — .hotam-spec-project marker file, bottom-up.
    r4 = _search_marker_file_upward(cwd, MAX_MARKER_SEARCH_DEPTH)
    if r4 is not None:
        return r4

    # R5 — pyproject.toml [tool.hotam-spec].project_root.
    r5 = _resolve_pyproject(cwd, MAX_MARKER_SEARCH_DEPTH)
    if r5 is not None:
        return r5

    # R6 — self-hosting fallback (framework repo == consumer repo), gated on
    # CWD actually being inside the framework's own checkout. Without this
    # guard, ANY process anywhere (e.g. a freshly pip-installed consumer, cwd
    # unrelated to the framework install) whose CWD carries none of the R3-R5
    # markers would silently adopt the framework's own repo as project root —
    # a real bug caught by tests/test_e2e_consumer_subprocess.py: the first
    # `hotam-create-domain` call from a brand-new, marker-less consumer
    # directory wrote into the framework's install tree instead of failing
    # or resolving locally.
    repo_root = repo_paths.repo_root()
    try:
        cwd.relative_to(repo_root)
    except ValueError:
        return None
    return repo_root


def _build_diagnostic() -> str:
    """Build a diagnostic string listing all sources checked and their state.

    Used by ``project_root_or_raise()`` when R1–R6 all failed — gives the
    operator visibility into WHY resolution failed rather than a bare error.
    """
    lines: list[str] = [
        "project_root() could not resolve a project root.",
        "The following sources were checked (R1–R6):",
    ]

    # R1
    r1_raw = os.environ.get(ENV_PROJECT_ROOT, "").strip()
    if r1_raw:
        r1_dir = Path(r1_raw).resolve()
        exists = "exists" if r1_dir.is_dir() else "NOT a directory/missing"
        lines.append(f"  R1 env {ENV_PROJECT_ROOT}='{r1_raw}' — {exists}")
    else:
        lines.append(f"  R1 env {ENV_PROJECT_ROOT} — not set")

    # R2
    r2_raw = os.environ.get(ENV_DOMAINS_ROOT, "").strip()
    if r2_raw:
        r2_dir = Path(r2_raw).resolve()
        exists = "exists" if r2_dir.is_dir() else "NOT a directory/missing"
        lines.append(f"  R2 env {ENV_DOMAINS_ROOT}='{r2_raw}' — {exists}")
    else:
        lines.append(f"  R2 env {ENV_DOMAINS_ROOT} — not set")

    cwd = Path.cwd()
    lines.append(
        f"  R3 CWD={cwd} — RELIABLE markers (any one suffices): "
        f"{', '.join(RELIABLE_MARKER_PATHS)}, pyproject.toml[tool.{PYPROJECT_TABLE}]; "
        f"SECONDARY markers (need {SECONDARY_MARKER_MIN_COUNT}+ together): "
        f"{', '.join(SECONDARY_MARKER_PATHS)}"
    )
    lines.append(f"  R4 marker file '{MARKER_FILENAME}' — searched {MAX_MARKER_SEARCH_DEPTH} levels up from CWD")
    lines.append(f"  R5 pyproject.toml [tool.{PYPROJECT_TABLE}].{PYPROJECT_PROJECT_ROOT_KEY} — searched {MAX_MARKER_SEARCH_DEPTH} levels up from CWD")
    lines.append(f"  R6 self-hosting fallback (repo_paths.repo_root()) — returned None or not applicable")
    lines.append(
        "Set one of: env HOTAM_SPEC_PROJECT_ROOT=<dir>, "
        "HOTAM_SPEC_DOMAINS_ROOT=<domains-dir>, "
        "create a RELIABLE marker (domains/, delegations/, or "
        "pyproject.toml[tool.hotam-spec]) in CWD, "
        "or a .hotam-spec-project file."
    )
    return "\n".join(lines)


def project_root_or_raise() -> Path:
    """Return ``project_root()`` or raise ``ProjectRootUnresolved``.

    For callers that NEED a path and cannot proceed without one. The
    exception carries a full diagnostic (which sources were checked, what
    they contained, what CWD was).

    Canon: §Graph — the uncertainty-rule raising accessor.
    """
    result = project_root()
    if result is not None:
        return result
    raise ProjectRootUnresolved(_build_diagnostic())
