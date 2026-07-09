"""Canon: §Attention — R-attention-claude-adapter: the hook is wired into settings.

Proves the committed sensorium generator (tools/setup_hooks.py) includes the
Claude adapter (tools/attention_hook.py) on UserPromptSubmit, so a fresh clone
gets the attention pulse injected with zero edits, AND that the adapter is a
thin wrapper (it imports the core, does not re-implement sensing).
"""

from __future__ import annotations

from pathlib import Path

_TOOLS = Path(__file__).resolve().parents[1] / "tools"

import setup_hooks  # noqa: E402


def _commands(settings: dict) -> list[str]:
    out: list[str] = []
    for groups in settings.get("hooks", {}).values():
        for group in groups:
            for entry in group.get("hooks", []):
                if entry.get("command"):
                    out.append(entry["command"])
    return out


def test_setup_hooks_wires_attention_adapter_on_user_prompt() -> None:
    """build_settings() emits attention_hook.py as a UserPromptSubmit hook."""
    settings = setup_hooks.build_settings()
    ups = settings["hooks"]["UserPromptSubmit"]
    cmds = []
    for group in ups:
        for entry in group.get("hooks", []):
            cmds.append(entry.get("command", ""))
    assert any("attention_hook.py" in c for c in cmds), (
        "the Claude adapter tools/attention_hook.py must be a UserPromptSubmit "
        "hook in the committed sensorium (R-attention-claude-adapter)"
    )


def test_adapter_delegates_to_core() -> None:
    """The adapter is thin: its source imports the attention core rather than
    re-implementing any sensing logic (R-attention-agent-agnostic-core seam)."""
    src = (_TOOLS / "attention_hook.py").read_text(encoding="utf-8")
    assert "hotam_spec import attention" in src or "hotam_spec.attention" in src, (
        "the adapter must delegate to the attention core, not re-sense"
    )
