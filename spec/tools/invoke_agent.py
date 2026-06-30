"""Canon: §Agent — invokes a sub-agent by loading its spec/agents/<name>/CLAUDE.md as the operator-prompt and printing it to stdout.

Resolve a named sub-agent by reading its CLAUDE.md operator-prompt from the
agents registry at spec/agents/<name>/CLAUDE.md and printing it to stdout.
This constitutes the agent operator: the loaded CLAUDE.md IS the agent's
mind; reading it = constituting that operator in the current session.

Future M37 work will dispatch the resolved prompt to a real backend (CI runner,
a different coding agent, or a programmatic steward). For now the
print-to-stdout form lets a human (or the parent operator) verify exactly what
the agent will be told before any dispatch happens. This keeps
R-ai-presents-not-decides: the operator presents, the steward decides whether
to dispatch.

WHY: an agent dropped into the repo without its CLAUDE.md is unanchored; it
will invent state from training memory instead of reading the substrate
(R-boot-from-substrate). Forcing resolution through this tool makes the
constitution step explicit and auditable. The tool REFUSES on a missing
directory or missing CLAUDE.md so that silent misconfiguration is impossible.

Usage:
  uv run python tools/invoke_agent.py <name>
  uv run python tools/invoke_agent.py <name> --show-tools
  uv run python tools/invoke_agent.py <name> --show-scope
  uv run python tools/invoke_agent.py <name> --show-tools --show-scope
"""

from __future__ import annotations

import argparse
import importlib.util
import sys
from pathlib import Path

# ---------------------------------------------------------------------------
# Path constants — monkeypatchable in tests
# ---------------------------------------------------------------------------

_SPEC_ROOT = Path(__file__).resolve().parents[1]  # .../spec
_AGENTS_ROOT = _SPEC_ROOT / "agents"


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _available_agents(agents_root: Path) -> list[str]:
    """Return sorted list of agent directory names under agents_root."""
    if not agents_root.exists():
        return []
    return sorted(p.name for p in agents_root.iterdir() if p.is_dir())


def _shared_tools(spec_root: Path) -> list[Path]:
    """Sorted list of shared spec/tools/*.py files (excluding __init__.py)."""
    tools_dir = spec_root / "tools"
    if not tools_dir.exists():
        return []
    return sorted(p for p in tools_dir.glob("*.py") if p.name != "__init__.py")


def _private_tools(agents_root: Path, name: str) -> list[Path]:
    """Sorted list of private spec/agents/<name>/tools/*.py files."""
    private_dir = agents_root / name / "tools"
    if not private_dir.exists():
        return []
    return sorted(p for p in private_dir.glob("*.py") if p.name != "__init__.py")


def _load_scope(agents_root: Path, name: str) -> tuple[str, ...]:
    """Import spec/agents/<name>/scope.py and return its SCOPE tuple."""
    scope_path = agents_root / name / "scope.py"
    if not scope_path.exists():
        raise FileNotFoundError(f"scope.py not found at {scope_path}")
    spec = importlib.util.spec_from_file_location(f"_agent_{name}_scope", scope_path)
    if spec is None or spec.loader is None:
        raise ImportError(f"Cannot load scope.py from {scope_path}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)  # type: ignore[union-attr]
    return tuple(module.SCOPE)


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------


def main(argv: list[str] | None = None) -> int:
    """Resolve a named sub-agent and print its CLAUDE.md to stdout."""
    parser = argparse.ArgumentParser(
        prog="invoke_agent",
        description=(
            "Resolve a named sub-agent and print its CLAUDE.md operator-prompt "
            "to stdout. Optionally list the tool registry and/or the scope tuple."
        ),
    )
    parser.add_argument(
        "name",
        help="Agent directory name under spec/agents/<name>/.",
    )
    parser.add_argument(
        "--show-tools",
        action="store_true",
        help="Also print the tool registry (shared tools then private agent tools).",
    )
    parser.add_argument(
        "--show-scope",
        action="store_true",
        help="Also print the SCOPE tuple from spec/agents/<name>/scope.py.",
    )
    args = parser.parse_args(argv)

    agents_root = _AGENTS_ROOT
    name: str = args.name

    # --- Resolve agent directory ---
    agent_dir = agents_root / name
    if not agent_dir.exists() or not agent_dir.is_dir():
        available = _available_agents(agents_root)
        avail_str = ", ".join(available) if available else "(none)"
        print(
            f"Unknown agent '{name}'. Available: {avail_str}",
            file=sys.stderr,
        )
        return 1

    # --- Resolve CLAUDE.md ---
    claude_md = agent_dir / "CLAUDE.md"
    if not claude_md.exists():
        print(
            f"Agent '{name}' exists but has no CLAUDE.md at {claude_md}.",
            file=sys.stderr,
        )
        return 1

    # --- Print CLAUDE.md ---
    content = (
        claude_md.read_text(encoding="utf-8").replace("\r\n", "\n").replace("\r", "\n")
    )
    print(content, end="")

    # --- Optional: tool registry ---
    if args.show_tools:
        shared = _shared_tools(_SPEC_ROOT)
        private = _private_tools(agents_root, name)
        print("\n\n## Tool registry\n")
        print("### Shared tools (spec/tools/)\n")
        if shared:
            for p in shared:
                print(f"- {p.name}")
        else:
            print("_(none)_")
        print(f"\n### Private tools (spec/agents/{name}/tools/)\n")
        if private:
            for p in private:
                print(f"- {p.name}")
        else:
            print("_(none)_")

    # --- Optional: scope ---
    if args.show_scope:
        try:
            scope = _load_scope(agents_root, name)
        except FileNotFoundError as exc:
            print(f"\n\nScope: ERROR — {exc}", file=sys.stderr)
            return 1
        print("\n\n## Scope\n")
        for item in scope:
            print(f"- {item}")

    return 0


if __name__ == "__main__":
    sys.exit(main())
