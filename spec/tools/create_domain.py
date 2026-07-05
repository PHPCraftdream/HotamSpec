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

from __future__ import annotations

from hotam_spec.assumption import Assumption
from hotam_spec.axis import Axis
from hotam_spec.conflict import Conflict, conflict_identity
from hotam_spec.graph import TensionGraph
from hotam_spec.requirement import Requirement
from hotam_spec.stakeholder import Stakeholder

# These imports (Assumption/Axis/Conflict/conflict_identity/Requirement/
# Stakeholder) are pre-declared so tools/apply_proposal.py can append nodes
# into the tuples below without having to inject an import. They are referenced
# only after the first node of each kind is added; the F401 until then is
# intentional scaffolding.
_ = (Assumption, Axis, Conflict, conflict_identity, Requirement, Stakeholder)


def build_graph() -> TensionGraph:
    # NOTE: these MUST be named assignments (not inline kwargs) so the
    # apply_proposal.py writers (ProposedStakeholder/Axis/Requirement/Conflict/
    # Assumption) can locate and append into each tuple. Do not hand-edit —
    # add nodes via tools/apply_proposal.py (R-no-hand-edit-graph).
    stakeholders = (
    )
    axes = (
    )
    requirements = (
    )
    conflicts = (
    )
    assumptions = (
    )
    return TensionGraph(
        axes=axes,
        stakeholders=stakeholders,
        requirements=requirements,
        conflicts=conflicts,
        assumptions=assumptions,
    )
'''

_CLAUDE_MD_TEMPLATE = """\
# {name}

> This is a POINTER, not the live crystal for `{name}`.
>
> The operator crystal (LIVE-STATE, CONSTITUTION, REPO-MAP, AGENT-MAP) does NOT
> live in this file. It materializes in the REPOSITORY-ROOT CLAUDE.md whenever
> `{name}` is the active domain. gen_spec.py rebuilds that root crystal from
> whichever domain is pinned — it never populates per-domain CLAUDE.md files.
>
> To make `{name}` the active domain and get its crystal:
>   echo {name} > domains/.active-domain      # (or: export HOTAM_SPEC_ACTIVE_DOMAIN={name})
>   cd spec && python tools/gen_spec.py  # root CLAUDE.md becomes {name}'s crystal
> (create_domain.py --activate does both in one step.)

## Domain

{description}

## Goals

{goals_bullet}
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


def _activate_domain(name: str, domains_root: Path) -> int:
    """Pin `name` as the active domain and regenerate the root crystal.

    Writes domains/.active-domain = name (the pin gen_spec.py reads first when
    no HOTAM_SPEC_ACTIVE_DOMAIN env var is set), then runs gen_spec.py so the
    REPOSITORY-ROOT CLAUDE.md becomes this domain's operator crystal. This is
    the REAL activation mechanism — the honest replacement for the old, false
    'run gen_spec.py from the domain root to populate' template instruction.
    """
    pin = domains_root / ".active-domain"
    pin.write_text(name + "\n", encoding="utf-8")
    print(f"Pinned active domain: {pin} -> {name}")
    result = subprocess.run(
        [sys.executable, str(Path(__file__).resolve().parent / "gen_spec.py")],
        cwd=str(_SPEC_ROOT),
    )
    if result.returncode != 0:
        print(
            "ERROR: gen_spec.py failed after pinning; the root crystal may be stale.",
            file=sys.stderr,
        )
        return 1
    print(f"Regenerated root CLAUDE.md as the '{name}' crystal.")
    return 0


def scaffold(
    name: str,
    description: str,
    goals: list[str],
    director_purpose: str,
    domains_root: Path,
    *,
    dry_run: bool = False,
    activate: bool = False,
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

    if activate:
        return _activate_domain(name, domains_root)
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
    parser.add_argument(
        "--activate",
        action="store_true",
        help="After scaffolding, pin this domain (domains/.active-domain) and run "
        "gen_spec.py so the repository-root CLAUDE.md becomes this domain's crystal.",
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
        activate=args.activate,
    )


if __name__ == "__main__":
    sys.exit(main())
