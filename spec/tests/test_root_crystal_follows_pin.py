"""Canon: §Domain — R-root-crystal-follows-pin regression tests.

The root CLAUDE.md is the resident operator crystal (the self-host domain's
boot seed). When a proposal is landed for a NON-pinned domain (the operator
exports HOTAM_SPEC_ACTIVE_DOMAIN=hotam-dev to work on the business domain),
apply_proposal.py runs gen_spec.py as a subprocess that inherits that env var.
Before this fix, that subprocess regenerated the ROOT CLAUDE.md from the
transiently-active domain, overwriting "Operator of hotam-spec-self (N SETTLED)"
with "Operator of hotam-dev (7 SETTLED)" — real contamination of the committed
self-host crystal.

Fix (two parts):
  1. gen_spec.py --docs-only regenerates ONLY the active domain's docs/gen/
     (+ per-domain docs via _process_domains); it never touches root CLAUDE.md.
  2. apply_proposal.py runs a --docs-only pass (env = applied domain) THEN an
     env-stripped pass (root crystal from the pin domains/.active-domain).

These tests pin part (1) — the mechanism apply_proposal relies on — plus the
per-domain reader isolation that keeps _process_domains from stamping every
domain's `reader:` header with the env-active domain's binding.
"""

from __future__ import annotations

import os
import subprocess
import sys
from pathlib import Path

SPEC_ROOT = Path(__file__).resolve().parents[1]
REPO_ROOT = SPEC_ROOT.parent
CLAUDE_MD = REPO_ROOT / "CLAUDE.md"
GEN_SPEC = SPEC_ROOT / "tools" / "gen_spec.py"
DOMAINS_ROOT = REPO_ROOT / "domains"
PIN_FILE = DOMAINS_ROOT / ".active-domain"


def _non_pinned_domain() -> str | None:
    """Return a domain name that is NOT the pinned one, or None if <2 domains."""
    if not DOMAINS_ROOT.exists():
        return None
    pinned = ""
    if PIN_FILE.exists():
        pinned = PIN_FILE.read_text(encoding="utf-8").strip()
    others = sorted(
        d.name
        for d in DOMAINS_ROOT.iterdir()
        if d.is_dir() and not d.name.startswith("_") and d.name != pinned
    )
    return others[0] if others else None


def test_docs_only_never_touches_root_claude_md_under_foreign_env() -> None:
    """gen_spec.py --docs-only with HOTAM_SPEC_ACTIVE_DOMAIN set to a NON-pinned
    domain must leave root CLAUDE.md byte-for-byte identical."""
    foreign = _non_pinned_domain()
    if foreign is None:
        import pytest

        pytest.skip("need >= 2 domains to exercise cross-domain contamination")

    before = CLAUDE_MD.read_bytes()
    env = dict(os.environ)
    env["HOTAM_SPEC_ACTIVE_DOMAIN"] = foreign
    result = subprocess.run(
        [sys.executable, str(GEN_SPEC), "--docs-only"],
        capture_output=True,
        text=True,
        env=env,
        cwd=str(SPEC_ROOT),
    )
    after = CLAUDE_MD.read_bytes()
    assert result.returncode == 0, result.stderr
    assert after == before, (
        "gen_spec --docs-only under a foreign HOTAM_SPEC_ACTIVE_DOMAIN "
        "mutated root CLAUDE.md — the resident crystal must follow the pin "
        "(R-root-crystal-follows-pin)."
    )


def test_docs_only_repo_map_describes_own_domain_under_foreign_env() -> None:
    """gen_spec.py --docs-only with HOTAM_SPEC_ACTIVE_DOMAIN set to a NON-pinned
    domain must leave the PINNED domain's own REPO-MAP.md byte-for-byte
    identical — it must describe itself, never the env-active domain.

    Regression guard (#99): build_repo_map_md() defaulted content_dir/gen_dir
    to the module-level env/pin-resolved CONTENT_DIR/GEN_DIR globals, so
    _process_domains() writing the PINNED domain's docs/gen/REPO-MAP.md under
    a foreign HOTAM_SPEC_ACTIVE_DOMAIN described the FOREIGN domain's content
    dir instead of its own -- real contamination of a committed file, caught
    by a full T2 run leaving the working tree dirty.
    """
    pinned = PIN_FILE.read_text(encoding="utf-8").strip() if PIN_FILE.exists() else ""
    if not pinned:
        import pytest

        pytest.skip("no pinned domain")
    foreign = _non_pinned_domain()
    if foreign is None:
        import pytest

        pytest.skip("need >= 2 domains to exercise cross-domain contamination")

    repo_map = DOMAINS_ROOT / pinned / "docs" / "gen" / "REPO-MAP.md"
    before = repo_map.read_bytes()

    # Force a REAL regen of the pinned domain's docs: the dirty-index skip
    # (gen-domain-mtime.json) would otherwise mask the bug whenever the
    # pinned domain's graph.py/manifest.py mtimes are already cached from a
    # prior run in this same working tree -- exactly why an isolated re-run
    # of this test passed even with the bug present (only a full T2 run,
    # which touches other state first, reliably invalidated the cache).
    dirty_index_file = SPEC_ROOT / ".runtime" / "gen-domain-mtime.json"
    saved_index = (
        dirty_index_file.read_bytes() if dirty_index_file.exists() else None
    )
    if dirty_index_file.exists():
        dirty_index_file.unlink()
    try:
        env = dict(os.environ)
        env["HOTAM_SPEC_ACTIVE_DOMAIN"] = foreign
        result = subprocess.run(
            [sys.executable, str(GEN_SPEC), "--docs-only"],
            capture_output=True,
            text=True,
            env=env,
            cwd=str(SPEC_ROOT),
        )
    finally:
        if saved_index is not None:
            dirty_index_file.write_bytes(saved_index)
        elif dirty_index_file.exists():
            dirty_index_file.unlink()
    after = repo_map.read_bytes()
    assert result.returncode == 0, result.stderr
    assert after == before, (
        f"gen_spec --docs-only under a foreign HOTAM_SPEC_ACTIVE_DOMAIN "
        f"mutated the PINNED domain's own REPO-MAP.md ({repo_map}) -- it "
        f"must describe itself, not the env-active domain."
    )


def test_per_domain_reader_isolation() -> None:
    """domain_doc_readers(dir) resolves each domain's own DOC_READERS, so a
    domain's generated docs never carry the env-active domain's reader."""
    if str(SPEC_ROOT / "tools") not in sys.path:
        sys.path.insert(0, str(SPEC_ROOT / "tools"))
    from hotam_spec.graph import domain_doc_readers  # noqa: PLC0415

    pinned = PIN_FILE.read_text(encoding="utf-8").strip() if PIN_FILE.exists() else ""
    if not pinned:
        import pytest

        pytest.skip("no pinned domain")
    pinned_dir = DOMAINS_ROOT / pinned
    # The pinned self-host domain declares DOC_READERS; resolving it directly
    # must not depend on the env var at all.
    os.environ["HOTAM_SPEC_ACTIVE_DOMAIN"] = "hotam-dev"
    try:
        readers = domain_doc_readers(pinned_dir)
    finally:
        os.environ.pop("HOTAM_SPEC_ACTIVE_DOMAIN", None)
    assert isinstance(readers, dict)
    # self-host declares at least the ai-agent / framework-author readers.
    assert readers, "pinned domain must declare DOC_READERS"
