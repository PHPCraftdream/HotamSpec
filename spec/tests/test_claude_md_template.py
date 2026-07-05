"""Tests: template-driven root CLAUDE.md generation (R-claude-md-template-driven).

CLAUDE.md.template.txt is the human-editable source of root CLAUDE.md: a fixed
header plus exactly two placeholder lines, '<!-- mind -->' and '<!-- business -->'.
gen_spec.py substitutes MIND (domain-agnostic methodology layer) and BUSINESS
(domain-specific claims layer) content into those placeholders, writing the
result to CLAUDE.md. Everything else in the template -- including hand-written
notes below the placeholders -- survives every regeneration verbatim.

Canon: R-claude-md-template-driven.
"""

from __future__ import annotations

import sys
from pathlib import Path

SPEC_ROOT = Path(__file__).resolve().parents[1]
REPO_ROOT = SPEC_ROOT.parent

_tools = str(SPEC_ROOT / "tools")
if _tools not in sys.path:
    sys.path.insert(0, _tools)

import gen_spec as _gs  # noqa: E402
from hotam_spec.graph import load_content_graph  # noqa: E402

CLAUDE_MD_TEMPLATE = REPO_ROOT / "CLAUDE.md.template.txt"
ROOT_CLAUDE_MD = REPO_ROOT / "CLAUDE.md"


def _read(path: Path) -> str:
    return path.read_text(encoding="utf-8").replace("\r\n", "\n").replace("\r", "\n")


def _run_gen_spec_in_process() -> None:
    """Regenerate root CLAUDE.md in-process under the self-host pin (task #46,
    Measure 2 — replaces a subprocess spawn while reproducing its isolation:
    HOTAM_SPEC_ACTIVE_DOMAIN=hotam-spec-self + cwd=spec, both cleanly restored)."""
    import os  # noqa: PLC0415

    prev_env = os.environ.get("HOTAM_SPEC_ACTIVE_DOMAIN")
    prev_cwd = os.getcwd()
    os.environ["HOTAM_SPEC_ACTIVE_DOMAIN"] = "hotam-spec-self"
    try:
        os.chdir(SPEC_ROOT)
        _gs.main([])
    finally:
        os.chdir(prev_cwd)
        if prev_env is None:
            os.environ.pop("HOTAM_SPEC_ACTIVE_DOMAIN", None)
        else:
            os.environ["HOTAM_SPEC_ACTIVE_DOMAIN"] = prev_env


# ===========================================================================
# 1. Template file exists with both placeholders
# ===========================================================================


def test_template_file_exists_with_both_placeholders() -> None:
    """CLAUDE.md.template.txt exists at repo root and has both placeholder lines."""
    assert CLAUDE_MD_TEMPLATE.exists(), (
        f"CLAUDE.md.template.txt not found at {CLAUDE_MD_TEMPLATE}"
    )
    text = _read(CLAUDE_MD_TEMPLATE)
    lines = [ln.strip() for ln in text.split("\n")]
    assert "<!-- mind -->" in lines, (
        "CLAUDE.md.template.txt missing literal '<!-- mind -->' placeholder line"
    )
    assert "<!-- business -->" in lines, (
        "CLAUDE.md.template.txt missing literal '<!-- business -->' placeholder line"
    )


# ===========================================================================
# 2. Generated CLAUDE.md no longer contains the placeholder literals
# ===========================================================================


def test_generated_claude_md_no_longer_contains_placeholder_literals() -> None:
    """After generation, root CLAUDE.md must not contain the raw placeholder LINES.

    Checked line-by-line (not substring-in-text) because the placeholder text
    itself legitimately appears as a quoted example inside the CONSTITUTION
    digest's rendering of R-claude-md-template-driven's own claim text.
    """
    text = _read(ROOT_CLAUDE_MD)
    lines = [ln.strip() for ln in text.split("\n")]
    assert "<!-- mind -->" not in lines, (
        "Generated CLAUDE.md still contains a literal '<!-- mind -->' placeholder LINE — "
        "substitution did not run. Run: uv run python tools/gen_spec.py"
    )
    assert "<!-- business -->" not in lines, (
        "Generated CLAUDE.md still contains a literal '<!-- business -->' placeholder LINE — "
        "substitution did not run. Run: uv run python tools/gen_spec.py"
    )


# ===========================================================================
# 3. Hand-written note in the template survives regeneration
# ===========================================================================


def test_hand_written_note_in_template_survives_regen() -> None:
    """A unique marker appended below both placeholders survives gen_spec.py regen."""
    original = CLAUDE_MD_TEMPLATE.read_bytes()
    # Snapshot the generated CLAUDE.md bytes so the finally block restores them
    # VERBATIM. Restoring by re-running gen_spec is unsafe here: build_live_state
    # reports the resident-crystal char count read from the CURRENT on-disk
    # CLAUDE.md (gen_spec.py:1797), a self-referential field that only reaches a
    # fixpoint over two passes — a single restore regen would leave a stale count
    # and redden test_docs_gen::test_claude_md_live_state_up_to_date. subprocess
    # isolation used to mask this because many LATER subprocess regens
    # re-converged the file; with those removed (task #46), we restore the exact
    # original bytes instead (Measure 2 — reproduce the lost isolation explicitly).
    original_claude_md = ROOT_CLAUDE_MD.read_bytes()
    marker = "TEST-MARKER-d41a9f3b-durable-note-survives-regen"
    try:
        text = _read(CLAUDE_MD_TEMPLATE)
        text_with_marker = text.rstrip("\n") + f"\n\n{marker}\n"
        CLAUDE_MD_TEMPLATE.write_text(text_with_marker, encoding="utf-8", newline="\n")

        _run_gen_spec_in_process()

        generated = _read(ROOT_CLAUDE_MD)
        assert marker in generated, (
            f"Hand-written marker {marker!r} did not survive regeneration — "
            "template substitution is clobbering content outside the two placeholders."
        )
    finally:
        CLAUDE_MD_TEMPLATE.write_bytes(original)
        ROOT_CLAUDE_MD.write_bytes(original_claude_md)


# ===========================================================================
# 4. Regeneration is byte-identical (determinism)
# ===========================================================================


def test_regen_byte_identical() -> None:
    """Rendering CLAUDE.md from the template twice on an unchanged graph is byte-identical."""
    g = load_content_graph()
    first = _gs.render_claude_md_from_template(g)
    second = _gs.render_claude_md_from_template(g)
    assert first == second, (
        "render_claude_md_from_template() is not deterministic across two calls "
        "on an unchanged graph."
    )


# ===========================================================================
# 5. MIND content present in output
# ===========================================================================


def test_mind_content_present_in_output() -> None:
    """Generated CLAUDE.md must contain EMBEDDED-THINKING and EMBEDDED-TOOLS content."""
    text = _read(ROOT_CLAUDE_MD)
    assert "<!-- EMBEDDED-THINKING:BEGIN -->" in text, (
        "Generated CLAUDE.md missing EMBEDDED-THINKING:BEGIN sentinel"
    )
    assert "<!-- EMBEDDED-TOOLS:BEGIN -->" in text, (
        "Generated CLAUDE.md missing EMBEDDED-TOOLS:BEGIN sentinel"
    )
    # Tier-1 distillation: a known thinking topic must appear with a RULE
    # distillate and its Tier-3 full-text pointer, not the full doc body verbatim
    # (R-crystal-is-tiered; see test_embedded_thinking_tools.py for the detailed
    # per-block assertions).
    assert "§Conflict" in text, (
        "Generated CLAUDE.md does not contain the §Conflict topic entry"
    )
    assert "spec/docs/thinking/conflict.md" in text, (
        "Generated CLAUDE.md does not point at the full text of spec/docs/thinking/conflict.md"
    )


# ===========================================================================
# 6. BUSINESS content present in output
# ===========================================================================


def test_business_content_present_in_output() -> None:
    """Generated CLAUDE.md must contain CONSTITUTION and DOMAIN-MAP content."""
    text = _read(ROOT_CLAUDE_MD)
    assert "<!-- CONSTITUTION:BEGIN -->" in text, (
        "Generated CLAUDE.md missing CONSTITUTION:BEGIN sentinel"
    )
    assert "<!-- DOMAIN-MAP:BEGIN -->" in text, (
        "Generated CLAUDE.md missing DOMAIN-MAP:BEGIN sentinel"
    )
    assert "Constitution index" in text, (
        "Generated CLAUDE.md does not contain the CONSTITUTION index heading"
    )
