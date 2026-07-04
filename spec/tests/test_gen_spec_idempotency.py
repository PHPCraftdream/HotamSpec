"""Canon: §Generator — the ONE idempotency canary (task #46, Measure 4).

Before this file, ~6 map/doc tests each spawned `python tools/gen_spec.py`
TWICE (cold-start ~11s/spawn) purely to assert "gen_spec is byte-idempotent":
test_agent_map, test_concept_map, test_repo_map, test_recently_rejected,
test_embedded_thinking_tools, test_tool_derived_requirements. That single
determinism property does not need to be re-proven per block — it is a global
fact about gen_spec. Those tests now assert their block content against the
session-scoped ``gen_spec_snapshot`` fixture (Measure 1) instead of
regenerating; the byte-identity guarantee lives here, proven ONCE.

This canary genuinely runs gen_spec twice, consecutively, and compares the
resulting root CLAUDE.md AND every domains/<self>/docs/gen/*.md byte-for-byte.
It runs IN-PROCESS under the self-host pin (importlib-isolated module per run,
so no cross-test module state leaks; HOTAM_SPEC_ACTIVE_DOMAIN=hotam-spec-self
so the root crystal follows the pin — R-root-crystal-follows-pin). Because the
working tree starts clean and gen_spec is idempotent, both runs rewrite
identical bytes and the tree is left clean.
"""

from __future__ import annotations

import importlib.util
import os
import sys
from pathlib import Path

import pytest

SPEC_ROOT = Path(__file__).resolve().parents[1]  # .../spec
REPO_ROOT = SPEC_ROOT.parent
ROOT_CLAUDE_MD = REPO_ROOT / "CLAUDE.md"


def _run_gen_spec_isolated() -> None:
    """Exec tools/gen_spec.py in a throw-away module namespace and call main([])
    under the self-host pin (clean rollback of env + cwd)."""
    gen_spec_path = SPEC_ROOT / "tools" / "gen_spec.py"
    spec = importlib.util.spec_from_file_location("gen_spec_canary", gen_spec_path)
    assert spec is not None and spec.loader is not None
    mod = importlib.util.module_from_spec(spec)
    sys.modules["gen_spec_canary"] = mod
    prev_env = os.environ.get("HOTAM_SPEC_ACTIVE_DOMAIN")
    prev_cwd = os.getcwd()
    os.environ["HOTAM_SPEC_ACTIVE_DOMAIN"] = "hotam-spec-self"
    try:
        spec.loader.exec_module(mod)  # type: ignore[union-attr]
        os.chdir(SPEC_ROOT)
        mod.main([])
    finally:
        os.chdir(prev_cwd)
        sys.modules.pop("gen_spec_canary", None)
        if prev_env is None:
            os.environ.pop("HOTAM_SPEC_ACTIVE_DOMAIN", None)
        else:
            os.environ["HOTAM_SPEC_ACTIVE_DOMAIN"] = prev_env


def _domain_gen_dir() -> Path | None:
    gen_spec_path = SPEC_ROOT / "tools" / "gen_spec.py"
    spec = importlib.util.spec_from_file_location("gen_spec_gendir", gen_spec_path)
    assert spec is not None and spec.loader is not None
    mod = importlib.util.module_from_spec(spec)
    sys.modules["gen_spec_gendir"] = mod
    try:
        spec.loader.exec_module(mod)  # type: ignore[union-attr]
        return mod.GEN_DIR
    finally:
        sys.modules.pop("gen_spec_gendir", None)


def _snapshot() -> dict[str, bytes]:
    snap: dict[str, bytes] = {"CLAUDE.md": ROOT_CLAUDE_MD.read_bytes()}
    gen_dir = _domain_gen_dir()
    if gen_dir is not None and gen_dir.exists():
        for p in sorted(gen_dir.glob("*.md")):
            snap[f"docs/gen/{p.name}"] = p.read_bytes()
    return snap


@pytest.mark.framework
def test_gen_spec_is_byte_idempotent() -> None:
    """Two consecutive gen_spec runs produce byte-identical CLAUDE.md + docs/gen.

    This is the single, authoritative determinism canary for gen_spec
    (R-deterministic-generation). If it reddens, gen_spec has a non-deterministic
    block; the specific offending block is found by bisecting the snapshot keys
    reported below.
    """
    _run_gen_spec_isolated()
    first = _snapshot()
    _run_gen_spec_isolated()
    second = _snapshot()

    differing = sorted(
        k for k in set(first) | set(second) if first.get(k) != second.get(k)
    )
    assert not differing, (
        "gen_spec.py is not idempotent: two consecutive runs produced different "
        f"bytes for: {differing}. Some generated block is non-deterministic."
    )
