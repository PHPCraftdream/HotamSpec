"""Canon: §Domain — scaffolds domains/<name>/ as a self-contained business domain with manifest.py, graph.py, tools/, agents/director/, docs/gen/, and CLAUDE.md.

WHY: Domains are isolated from the framework (spec/); each business unit owns
its own tension graph, tools, agents, generated docs, and operator crystal.
Hand-creating this tree is error-prone (wrong manifest fields, missing sentinels,
forgotten agents/ tree). create_domain bootstraps ALL of this — including the
initial director-agent — in one command, guaranteeing structural consistency with
R-domain-is-a-directory, R-domain-has-manifest, R-domain-declares-director,
R-domain-owns-graph-py, R-domain-owns-docs-gen, R-domain-owns-tools-and-agents,
R-domain-owns-claude-md, and R-director-agent-required-per-domain.

A freshly created domain contains:
  manifest.py  — ID, DESCRIPTION, GOALS tuple, DIRECTOR constant.
  graph.py     — minimal build_graph() returning an empty TensionGraph.
  tools/       — empty private tools directory.
  agents/      — container for domain agents (director lives here).
  docs/gen/    — placeholder for generated docs.
  CLAUDE.md    — operator crystal placeholder with all sentinel blocks.

After the tree is created, the director agent is scaffolded via create_agent.py
using --parent so it lands at domains/<name>/agents/director/.

Exit codes:
  0 — success.
  1 — validation failure, domain already exists, or director-agent creation failed.
"""

from __future__ import annotations

import argparse
import re
import shutil
import subprocess
import sys
from pathlib import Path

_SPEC_ROOT = Path(__file__).resolve().parents[1]
_REPO_ROOT = _SPEC_ROOT.parent

# Overridable in tests
_DOMAINS_ROOT = _REPO_ROOT / "domains"

_NAME_RE = re.compile(r"^[a-z][a-z0-9-]*$")

_MANIFEST_TEMPLATE = '''\
"""Canon: §Domain — manifest of domain \'{name}\'."""

ID = "{name}"
DESCRIPTION = "{description}"
GOALS = (
{goals_lines}
)
DIRECTOR = "director"
'''

_GRAPH_PY_TEMPLATE = '''\
"""Canon: §Domain — content graph of domain \'{name}\'."""

from tensio.graph import TensionGraph


def build_graph() -> TensionGraph:
    return TensionGraph(
        axes=(),
        stakeholders=(),
        requirements=(),
    )
'''

_CLAUDE_MD_TEMPLATE = """\
# {name}

> This file is the operator crystal for the `{name}` domain director.
> It is a PLACEHOLDER — run `uv run python tools/gen_spec.py` from the domain
> root to populate the blocks below from the live graph.

## Domain

{description}

## Goals

{goals_bullet}

<!-- LIVE-STATE:BEGIN -->
<!-- LIVE-STATE:END -->

<!-- CONSTITUTION:BEGIN -->
<!-- CONSTITUTION:END -->

<!-- REPO-MAP:BEGIN -->
<!-- REPO-MAP:END -->

<!-- AGENT-MAP:BEGIN -->
<!-- AGENT-MAP:END -->
"""


def _validate_name(name: str) -> str | None:
    if not _NAME_RE.match(name):
        return (
            f"Invalid domain name '{name}'. "
            "Must be lowercase letters, digits, and hyphens only, "
            "starting with a letter (e.g. 'my-domain')."
        )
    return None


def _goals_from_str(goals_str: str) -> list[str]:
    return [g.strip() for g in goals_str.split(";") if g.strip()]


def _goals_tuple_lines(goals: list[str]) -> str:
    return "\n".join(f'    "{g}",' for g in goals)


def _goals_bullet(goals: list[str]) -> str:
    return "\n".join(f"- {g}" for g in goals)


def scaffold(
    name: str,
    description: str,
    goals: list[str],
    director_purpose: str,
    domains_root: Path,
    *,
    dry_run: bool = False,
) -> int:
    """Create the domain directory tree. Returns exit code."""
    err = _validate_name(name)
    if err:
        print(f"ERROR: {err}", file=sys.stderr)
        return 1

    if not description.strip():
        print("ERROR: --description is required.", file=sys.stderr)
        return 1
    if not goals:
        print("ERROR: --goals is required (at least one goal).", file=sys.stderr)
        return 1
    if not director_purpose.strip():
        print("ERROR: --director-purpose is required.", file=sys.stderr)
        return 1

    domain_dir = domains_root / name
    if domain_dir.exists():
        print(
            f"ERROR: '{domain_dir}' already exists. "
            "Delete it manually before re-running.",
            file=sys.stderr,
        )
        return 1

    if dry_run:
        print(f"[dry-run] Would create domain at {domain_dir}")
        return 0

    # --- Create directory tree ---
    domain_dir.mkdir(parents=True)

    # manifest.py
    goals_lines = _goals_tuple_lines(goals)
    manifest_content = _MANIFEST_TEMPLATE.format(
        name=name,
        description=description,
        goals_lines=goals_lines,
    )
    (domain_dir / "manifest.py").write_text(manifest_content, encoding="utf-8")

    # graph.py
    graph_content = _GRAPH_PY_TEMPLATE.format(name=name)
    (domain_dir / "graph.py").write_text(graph_content, encoding="utf-8")

    # tools/
    tools_dir = domain_dir / "tools"
    tools_dir.mkdir()
    (tools_dir / "__init__.py").write_text("", encoding="utf-8")

    # agents/
    agents_dir = domain_dir / "agents"
    agents_dir.mkdir()
    (agents_dir / "__init__.py").write_text("", encoding="utf-8")

    # docs/gen/
    docs_gen_dir = domain_dir / "docs" / "gen"
    docs_gen_dir.mkdir(parents=True)
    (docs_gen_dir / ".gitkeep").write_text("", encoding="utf-8")

    # CLAUDE.md
    goals_bullet = _goals_bullet(goals)
    claude_md_content = _CLAUDE_MD_TEMPLATE.format(
        name=name,
        description=description,
        goals_bullet=goals_bullet,
    )
    (domain_dir / "CLAUDE.md").write_text(claude_md_content, encoding="utf-8")

    print(f"Created domain scaffold at {domain_dir}")

    # --- Scaffold director agent via create_agent.py ---
    create_agent_script = Path(__file__).resolve().parent / "create_agent.py"
    director_parent = str(agents_dir)
    cmd = [
        sys.executable,
        str(create_agent_script),
        "director",
        "--purpose",
        director_purpose,
        "--scope",
        "",
        "--parent",
        director_parent,
    ]
    result = subprocess.run(cmd, capture_output=False)
    if result.returncode != 0:
        print(
            "ERROR: director-agent creation failed; rolling back domain directory.",
            file=sys.stderr,
        )
        shutil.rmtree(domain_dir, ignore_errors=True)
        return 1

    print(f"Created director agent at {agents_dir / 'director'}")
    return 0


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(
        description="Scaffold a new business domain at domains/<name>/."
    )
    parser.add_argument(
        "name",
        help="Kebab-case domain name (e.g. my-domain). "
        "Lowercase letters, digits, hyphens only.",
    )
    parser.add_argument(
        "--description",
        default="",
        help="One-line domain description (required).",
    )
    parser.add_argument(
        "--goals",
        default="",
        help="Semicolon-separated list of domain goals (required). "
        "E.g. 'Track requirements;Resolve tensions'.",
    )
    parser.add_argument(
        "--director-purpose",
        default="",
        help="One-line rationale for the director agent (required).",
    )
    args = parser.parse_args(argv)

    missing = []
    if not args.description.strip():
        missing.append("--description")
    if not args.goals.strip():
        missing.append("--goals")
    if not args.director_purpose.strip():
        missing.append("--director-purpose")
    if missing:
        print(
            f"ERROR: the following required arguments are missing: {', '.join(missing)}",
            file=sys.stderr,
        )
        return 1

    goals = _goals_from_str(args.goals)
    return scaffold(
        name=args.name,
        description=args.description,
        goals=goals,
        director_purpose=args.director_purpose,
        domains_root=_DOMAINS_ROOT,
    )


if __name__ == "__main__":
    sys.exit(main())
