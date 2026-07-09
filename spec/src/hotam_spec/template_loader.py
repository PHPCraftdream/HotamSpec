"""Canon: §Graph — template loader via importlib.resources (PEP 391).

R-project-root-not-hardcoded (§3.4 of the portability requirement): the
operator-crystal template (``CLAUDE.md.template.txt``) lives INSIDE the
framework package at ``hotam_spec/_templates/claude_md.template.txt`` so it
ships with every install (editable, wheel, vendor-copy). It is read through
``importlib.resources.files()`` — the stdlib, install-method-agnostic accessor
that works identically for all three install kinds.

Optional override: if ``project_root() / "CLAUDE.md.template.txt"`` exists,
that file is used INSTEAD of the packaged one. A consumer who wants a
different crystal format (e.g. a domain-specific layout) drops their own
template at their project root and it takes priority. If the override is
absent, the packaged template is the fallback.

This module is the ONE source of truth for template resolution (P6): gen_spec
and any other consumer call ``read_claude_md_template()`` here rather than
constructing their own path.
"""

from __future__ import annotations

from pathlib import Path

from importlib.resources import files as _files


def _packaged_template_path() -> Path:
    """Return the Path to the packaged template inside the hotam_spec package.

    Uses ``importlib.resources.files()`` (PEP 391) — works for editable,
    wheel, and vendor-copy installs. Returns a real filesystem Path on all
    platforms (the template is a real .txt file, not a compressed resource).
    """
    return Path(str(_files("hotam_spec") / "_templates" / "claude_md.template.txt"))


def _override_template_path() -> Path | None:
    """Return the consumer override template path, or None if absent.

    The override lives at ``project_root() / "CLAUDE.md.template.txt"``. If
    project_root() cannot be resolved (e.g. a pure-library context with no
    consumer markers), there is no override — return None so the caller
    falls back to the packaged template.
    """
    from hotam_spec.project_paths import project_root  # noqa: PLC0415

    root = project_root()
    if root is None:
        return None
    candidate = root / "CLAUDE.md.template.txt"
    if candidate.is_file():
        return candidate
    return None


def claude_md_template_path() -> Path:
    """Return the effective template path: override if present, else packaged.

    Canon: §Graph — the single resolution entry point (P6). Priority:
      1. ``project_root() / "CLAUDE.md.template.txt"`` (consumer override).
      2. ``hotam_spec/_templates/claude_md.template.txt`` (packaged, via
         importlib.resources).
    """
    override = _override_template_path()
    if override is not None:
        return override
    return _packaged_template_path()


def read_claude_md_template() -> str:
    """Read and return the template text (utf-8, LF-normalized).

    Canon: §Graph — read accessor. Callers should use this rather than
    reading the path directly, so normalization stays in one place.
    """
    path = claude_md_template_path()
    return path.read_text(encoding="utf-8").replace("\r\n", "\n").replace("\r", "\n")
