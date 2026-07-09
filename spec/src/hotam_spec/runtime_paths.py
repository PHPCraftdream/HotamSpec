"""Canon: §Graph — runtime-directory resolution for consumer-side ephemera.

R-project-root-not-hardcoded (§3.2, variant 4-C): the framework's runtime
ephemera (context stamps, proposal staging, append-only logs, snapshots,
calibration baselines) are CONSUMER data, not framework-internal. They move
to the consumer's project root so a pip-installed framework never writes
inside its own package directory.

Resolution priority (first non-empty wins):

  1. ``HOTAM_SPEC_RUNTIME_DIR`` env var — used literally if it points at an
     existing directory (or a creatable path — existence is not required so
     a first-run consumer with no runtime dir yet still works).
  2. Default: ``project_root() / ".hotam-spec" / "runtime"``.

Self-hosting migration (data preservation): when the default path (2) is
empty AND the legacy framework-internal path ``spec/.runtime/`` contains
files, ``runtime_dir()`` returns the LEGACY path. This preserves accumulated
calibration data (run-speed-baseline.json, land-log.jsonl history,
enforcer-index.json cache) without a destructive copy. Once the operator
explicitly migrates files to the new location (or sets the env var), the
legacy fallback is bypassed.

stdlib-only, no side effects, no imports of domain-specific code.
"""

from __future__ import annotations

import os
from pathlib import Path

from hotam_spec import project_paths, repo_paths

#: Environment variable overriding the runtime directory (highest priority).
ENV_RUNTIME_DIR = "HOTAM_SPEC_RUNTIME_DIR"


def _env_runtime_dir() -> Path | None:
    """Return the runtime dir from ``HOTAM_SPEC_RUNTIME_DIR``, or ``None``.

    The env var, if set and non-empty, is used literally (existence is NOT
    required — the directory may not exist yet on first run; callers create
    it on demand). Returns ``None`` if unset/empty so the caller falls
    through to the default.
    """
    raw = os.environ.get(ENV_RUNTIME_DIR, "").strip()
    if not raw:
        return None
    return Path(raw).resolve()


def _legacy_runtime_dir() -> Path:
    """Return the legacy framework-internal runtime path (``spec/.runtime``).

    Used as a fallback source for self-hosting data preservation. This is
    the ONLY place that still references the framework-internal spec/.runtime
    location, centralized here so no consumer hardcodes it.
    """
    return repo_paths.spec_root() / ".runtime"


def _legacy_has_data() -> bool:
    """Check whether the legacy ``spec/.runtime/`` contains any files.

    Returns True if the directory exists and contains at least one regular
    file (recursively). This is the trigger for the self-hosting fallback:
    if the new default location is empty but the legacy location has
    accumulated data, we keep using the legacy path to avoid losing
    calibration/history.
    """
    legacy = _legacy_runtime_dir()
    if not legacy.exists():
        return False
    for _ in legacy.rglob("*"):
        if _.is_file():
            return True
    return False


def _default_runtime_dir() -> Path:
    """Resolve the default consumer-side runtime directory.

    Applies the self-hosting legacy fallback: if the default consumer path
    does not yet contain any files BUT the legacy ``spec/.runtime/`` does,
    return the legacy path. This transparently preserves accumulated data
    (run-speed-baseline.json calibration, land-log.jsonl history,
    enforcer-index.json cache) during the migration period.
    """
    consumer_runtime = project_paths.project_root_or_raise() / ".hotam-spec" / "runtime"

    # If the consumer runtime dir already has files, use it.
    if consumer_runtime.exists():
        for _ in consumer_runtime.rglob("*"):
            if _.is_file():
                return consumer_runtime

    # Legacy fallback: spec/.runtime/ has data the new location lacks.
    if _legacy_has_data():
        return _legacy_runtime_dir()

    return consumer_runtime


def runtime_dir() -> Path:
    """Return the runtime directory for ephemera (§3.2 variant 4-C).

    Resolution priority:
      1. ``HOTAM_SPEC_RUNTIME_DIR`` env var (used literally if set).
      2. ``project_root() / ".hotam-spec" / "runtime"``, with self-hosting
         legacy fallback to ``spec/.runtime/`` if the new location is empty
         but the legacy location has accumulated data.

    The returned path may not exist yet — callers create it on demand via
    ``path.mkdir(parents=True, exist_ok=True)`` before writing.

    Canon: §Graph — runtime-dir accessor (R-project-root-not-hardcoded).
    """
    env = _env_runtime_dir()
    if env is not None:
        return env
    return _default_runtime_dir()
