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
  R3 — markers in CWD, bottom-up search: any of ``domains/``, ``CLAUDE.md``,
       ``.claude/``, ``tickets/``, ``delegations/``, ``pyproject.toml`` with
       a ``[tool.hotam-spec]`` table.
  R4 — ``.hotam-spec-project`` marker file, searched bottom-up up to 5 levels
       from CWD.
  R5 — ``pyproject.toml`` → ``[tool.hotam-spec].project_root`` (relative path
       resolved from the pyproject.toml's own directory).
  R6 — self-hosting fallback: ``repo_paths.repo_root()`` (framework repo ==
       consumer repo — the only assumption that holds for the self-modeling
       core and its test suite).

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

#: R3 — filesystem markers that identify a directory as a project root.
#: Each is a relative path checked against the candidate directory.
MARKER_PATHS: tuple[str, ...] = (
    "domains",
    "CLAUDE.md",
    ".claude",
    "tickets",
    "delegations",
)

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


def _has_marker(candidate: Path) -> bool:
    """Check whether ``candidate`` contains any R3 filesystem marker.

    The pyproject.toml marker is special: it must contain a ``[tool.hotam-spec]``
    table (not just be any pyproject.toml), so a generic pyproject.toml from
    an unrelated project doesn't trigger a false positive.
    """
    for rel in MARKER_PATHS:
        if (candidate / rel).exists():
            return True
    pyproject = candidate / "pyproject.toml"
    if pyproject.is_file():
        try:
            with open(pyproject, "rb") as f:
                data = tomllib.load(f)
            if "tool" in data and PYPROJECT_TABLE in data.get("tool", {}):
                return True
        except (tomllib.TOMLDecodeError, OSError):
            pass
    return False


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

    # R6 — self-hosting fallback (framework repo == consumer repo).
    return repo_paths.repo_root()


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
    lines.append(f"  R3 CWD={cwd} — markers searched: {', '.join(MARKER_PATHS)}, pyproject.toml[tool.{PYPROJECT_TABLE}]")
    lines.append(f"  R4 marker file '{MARKER_FILENAME}' — searched {MAX_MARKER_SEARCH_DEPTH} levels up from CWD")
    lines.append(f"  R5 pyproject.toml [tool.{PYPROJECT_TABLE}].{PYPROJECT_PROJECT_ROOT_KEY} — searched {MAX_MARKER_SEARCH_DEPTH} levels up from CWD")
    lines.append(f"  R6 self-hosting fallback (repo_paths.repo_root()) — returned None or not applicable")
    lines.append(
        "Set one of: env HOTAM_SPEC_PROJECT_ROOT=<dir>, "
        "HOTAM_SPEC_DOMAINS_ROOT=<domains-dir>, "
        "create a marker (domains/, CLAUDE.md, etc.) in CWD, "
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
