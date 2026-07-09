"""Canon: §Graph — centralized repository path roots.

RULE (R-anchor-everything): the repository's canonical directory roots are
computed ONCE here (from this module's own ``__file__`` location) and derived
paths are expressed as relationships to those roots — not as
``Path(__file__).resolve().parents[N]`` with a magic N that silently breaks
when a file moves.

Before this module, ~40 call sites across ``spec/tools/*.py`` and
``spec/src/hotam_spec/*.py`` each wrote ``Path(__file__).resolve().parents[N]``
with an N hand-picked for that file's depth (1 for tools→spec, 2 for
tools→repo-root, 3 for src/hotam_spec→repo-root, etc.). Moving a file one
directory deeper silently broke every such path with no compile-time signal.
This module replaces those fragile literals with named, stable accessors:

  ``repo_root()``    — the absolute repository root (…/HotamSpec).
  ``spec_root()``    — ``<repo>/spec``.
  ``src_root()``     — ``<repo>/spec/src`` (the hotam_spec package parent).
  ``domains_root()`` — ``<repo>/domains``.
  ``tests_root()``   — ``<repo>/spec/tests``.
  ``tools_root()``   — ``<repo>/spec/tools``.
  ``runtime_root()`` — ``<repo>/spec/.runtime`` (gitignored ephemera).

stdlib-only, no side effects, no imports of domain-specific code. The roots
are computed lazily (function calls, not module-level constants) so that
test fixtures that monkeypatch ``Path.__file__`` or relocate the package
continue to work — but in practice the repository layout is fixed and these
are deterministic.
"""

from __future__ import annotations

from pathlib import Path

#: This module lives at spec/src/hotam_spec/repo_paths.py.
#: parents[0] = hotam_spec/, parents[1] = src/, parents[2] = spec/, parents[3] = repo root.
_SRC_HOTAM_SPEC_DIR = Path(__file__).resolve().parent
_SRC_DIR = _SRC_HOTAM_SPEC_DIR.parent
_SPEC_ROOT = _SRC_DIR.parent
_REPO_ROOT = _SPEC_ROOT.parent


def repo_root() -> Path:
    """Return the absolute repository root directory (…/HotamSpec).

    Canon: §Graph — repository root accessor.
    """
    return _REPO_ROOT


def spec_root() -> Path:
    """Return the ``<repo>/spec`` directory.

    Canon: §Graph — spec-root accessor.
    """
    return _SPEC_ROOT


def src_root() -> Path:
    """Return the ``<repo>/spec/src`` directory (hotam_spec package parent).

    Canon: §Graph — src-root accessor.
    """
    return _SRC_DIR


def hotam_spec_root() -> Path:
    """Return the ``<repo>/spec/src/hotam_spec`` directory (the framework package).

    Canon: §Graph — hotam_spec package-root accessor.
    """
    return _SRC_HOTAM_SPEC_DIR


def domains_root() -> Path:
    """Return the ``<repo>/domains`` directory.

    Canon: §Graph — domains-root accessor.
    """
    return _REPO_ROOT / "domains"


def tests_root() -> Path:
    """Return the ``<repo>/spec/tests`` directory.

    Canon: §Graph — tests-root accessor.
    """
    return _SPEC_ROOT / "tests"


def tools_root() -> Path:
    """Return the ``<repo>/spec/tools`` directory.

    Canon: §Graph — tools-root accessor.
    """
    return _SPEC_ROOT / "tools"


def runtime_root() -> Path:
    """Return the ``<repo>/spec/.runtime`` directory (gitignored ephemera).

    WHY a named accessor: ``spec/.runtime/`` is referenced by multiple tools
    (what_now, context_producer, apply_proposal, ...) and its path is always
    ``<spec>/.runtime``. Centralizing it here prevents a typo from silently
    writing to the wrong location.

    Canon: §Graph — runtime-root accessor.
    """
    return _SPEC_ROOT / ".runtime"


def docs_gen_root(domain_name: str | None = None) -> Path:
    """Return the generated-docs directory for a domain (or the legacy fallback).

    When ``domain_name`` is given, returns ``<repo>/domains/<name>/docs/gen``.
    When ``None``, returns the legacy ``<repo>/docs/gen`` (used by --demo and
    by the framework's own anti-drift meta-tests when no domain is active).

    Canon: §Graph — docs-gen-root accessor.
    """
    if domain_name is not None:
        return domains_root() / domain_name / "docs" / "gen"
    return _REPO_ROOT / "docs" / "gen"
