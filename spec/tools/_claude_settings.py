"""Shared Claude-settings-JSON read/write plumbing for spec/tools/*.py installers.

RULE (R-shared-tools-in-spec-tools): CLI-specific helpers — not part of the
core hotam_spec API surface — live under spec/tools/, in a private (``_``
prefixed) sibling module, following the same convention as ``_bootstrap.py``,
``_graph_loader.py``, ``_ticket_store.py``, ``_delegation_store.py``.

Before this module, ``tools/setup_hooks.py`` (writes the COMMITTED
``.claude/settings.json``, R-sensorium-committed) and
``tools/setup_context_hook.py`` (writes the personal, git-ignored
``.claude/settings.local.json``) each independently implemented the same two
primitives: "read a settings JSON file, defaulting to {} on absence/corrupt
content" and "write a settings dict as indented, LF, utf-8 JSON". The two
tools target DIFFERENT files for a real reason (committed sensorium vs.
personal context bridge — task #106 / L1-#7 confirms this split is
intentional, not accidental duplication) and are NOT merged into one file;
only the shared read/write bytes move here.

``setup_hooks.py`` additionally times-tamped-backs-up the committed file
before overwriting it (a project asset, worth a recovery copy);
``setup_context_hook.py`` does not (a personal, disposable file) — that
policy difference is expressed by the ``backup`` keyword, not by two
separate write functions.
"""

from __future__ import annotations

import json
from datetime import datetime, timezone
from pathlib import Path


def load_settings_json(path: Path) -> dict:
    """Read a Claude settings JSON file, defaulting to {} on absence/corruption.

    Never raises: an absent file, an OSError, or unparseable JSON all yield
    the same safe default — callers merge-add onto whatever they get back,
    so starting from {} is always a valid (if empty) base.
    """
    if not path.exists():
        return {}
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError):
        return {}


def _backup_path(target: Path) -> Path:
    stamp = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    return target.with_name(f"{target.name}.bak-{stamp}")


def write_settings_json(path: Path, data: dict, *, backup: bool = False) -> Path | None:
    """Write ``data`` as indented, LF-newline, utf-8 JSON to ``path``.

    Creates parent directories as needed. If ``backup=True`` and ``path``
    already exists, its current contents are copied to a timestamped
    ``<name>.bak-<UTC-stamp>`` sibling BEFORE the new content is written;
    returns that backup path, or None if no backup was made (either
    ``backup=False`` or the file did not previously exist).
    """
    path.parent.mkdir(parents=True, exist_ok=True)
    backup_made: Path | None = None
    if backup and path.exists():
        backup_made = _backup_path(path)
        backup_made.write_text(path.read_text(encoding="utf-8"), encoding="utf-8")
    path.write_text(json.dumps(data, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    return backup_made
