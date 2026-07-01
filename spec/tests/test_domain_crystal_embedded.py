"""Tests: root CLAUDE.md carries the domain's operator-facing content directly (post-P22.B).

Historical note: P22.B introduced a DOMAIN-CRYSTAL sentinel block that embedded
the FULL text of domains/<active>/CLAUDE.md (including a nested copy of
CONSTITUTION/AGENT-MAP/SHARED-DOCS) inside root CLAUDE.md. R-claude-md-template-driven
(supersedes R-root-claude-md-contains-domain-crystal) replaced that nested-embedding
model: root CLAUDE.md is now generated directly from CLAUDE.md.template.txt via
render_business_content()/render_mind_content(), which render CONSTITUTION, AGENT-MAP,
etc. ONCE, in root, with no second embedded copy of another CLAUDE.md.

These tests assert the DOMAIN-CRYSTAL sentinel is gone (the old mechanism was
retired, not merely relocated) and that the SETTLED constitutional content it used
to carry indirectly is directly present in root CLAUDE.md.

Canon: R-claude-md-template-driven.
"""

from __future__ import annotations

import subprocess
import sys
from pathlib import Path

import pytest

SPEC_ROOT = Path(__file__).resolve().parents[1]
REPO_ROOT = SPEC_ROOT.parent

_tools = str(SPEC_ROOT / "tools")
if _tools not in sys.path:
    sys.path.insert(0, _tools)

import gen_spec as _gs  # noqa: E402

ROOT_CLAUDE_MD = REPO_ROOT / "CLAUDE.md"
_ACTIVE_DOMAIN = _gs._active_domain()

_DOMAIN_CRYSTAL_BEGIN = "<!-- DOMAIN-CRYSTAL:BEGIN -->"
_DOMAIN_CRYSTAL_END = "<!-- DOMAIN-CRYSTAL:END -->"

# A stable substring known to live in the CONSTITUTION digest, sourced from
# domains/hotam-spec-self/graph.py — proves domain content reaches root directly.
_KNOWN_DOMAIN_SUBSTRING = "R-operator-prompt-from-substrate"


def _read(path: Path) -> str:
    return path.read_text(encoding="utf-8").replace("\r\n", "\n").replace("\r", "\n")


# ===========================================================================
# Test 1: DOMAIN-CRYSTAL sentinel retired (not relocated -- removed)
# ===========================================================================


def test_domain_crystal_sentinel_no_longer_emitted() -> None:
    """Root CLAUDE.md must NOT contain DOMAIN-CRYSTAL sentinels (retired by template model)."""
    if _ACTIVE_DOMAIN is None:
        pytest.skip("No active domain — not applicable")
    text = _read(ROOT_CLAUDE_MD)
    assert _DOMAIN_CRYSTAL_BEGIN not in text, (
        "Root CLAUDE.md still has a DOMAIN-CRYSTAL:BEGIN sentinel — the nested-embedding "
        "model was retired by R-claude-md-template-driven. CONSTITUTION/AGENT-MAP/etc. "
        "should be rendered directly by render_business_content(), not embedded via "
        "a second copy of domains/<active>/CLAUDE.md."
    )


# ===========================================================================
# Test 2: domain constitutional content reaches root directly
# ===========================================================================


def test_domain_constitution_content_reaches_root_directly() -> None:
    """Root CLAUDE.md's own CONSTITUTION block must carry domain-graph content directly.

    No nested crystal required: the same SETTLED requirement id that used to be
    reachable only inside the embedded domain CLAUDE.md is now present in root's
    single CONSTITUTION block.
    """
    if _ACTIVE_DOMAIN is None:
        pytest.skip("No active domain — not applicable")
    text = _read(ROOT_CLAUDE_MD)
    assert _KNOWN_DOMAIN_SUBSTRING in text, (
        f"Root CLAUDE.md does not contain expected domain content. "
        f"Expected substring: {_KNOWN_DOMAIN_SUBSTRING!r}. "
        "Run: uv run python tools/gen_spec.py"
    )


# ===========================================================================
# Test 3: regen byte-identical
# ===========================================================================


def test_root_claude_md_regen_byte_identical() -> None:
    """Running gen_spec.py again must not change root CLAUDE.md (idempotency)."""
    before = _read(ROOT_CLAUDE_MD)
    result = subprocess.run(
        [sys.executable, str(SPEC_ROOT / "tools" / "gen_spec.py")],
        capture_output=True,
        text=True,
        cwd=str(SPEC_ROOT),
    )
    assert result.returncode == 0, f"gen_spec.py failed:\n{result.stderr}"
    after = _read(ROOT_CLAUDE_MD)
    assert before == after, (
        "Root CLAUDE.md changed on re-regen — template rendering is not idempotent. "
        "Check render_claude_md_from_template() for non-determinism."
    )
