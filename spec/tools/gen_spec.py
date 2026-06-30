"""Canon: §Generator — regenerates docs/gen/ from the executable model (docstrings + graph), making drift structurally impossible.

Generator: the human layer + structural anti-drift (docs-as-code, layer 9).

The single source of truth is the executable model:
  - `spec/src/tensio/*.py` docstrings (the methodology narrative: RULE + Canon:§
    + WHY) — they ship with the framework and are content-free;
  - `spec/content/graph.py:build_graph()` (the domain's tension graph) — populated
    by the user; empty in a fresh framework.

The normative human layer is GENERATED, never hand-written; drift is structurally
impossible because the meta-test (tests/test_docs_gen.py) demands regeneration ==
committed, byte-for-byte.

Pipeline (mirrors dev-coin's gen_spec, purpose inverted from "prove no conflict"
to "render the tensions visibly"):

    tensio docstrings (narrative)            --gen-->  REQUIREMENTS.md
    content graph (Requirements + ...)       --gen-->  REQUIREMENTS.md (roster)
    Conflict clusters by axis + Mermaid      --gen-->  TENSIONS.md
    OPEN requirements + unresolved conflicts --gen-->  OPEN.md

Outputs (committed under docs/gen/, banner-marked, LF):
    REQUIREMENTS.md — requirement roster + methodology narrative.
    TENSIONS.md     — tension map: conflict nodes, clusters by axis, Mermaid.
    OPEN.md         — open registry: OPEN requirements + unresolved conflicts.

Run:
  uv run python tools/gen_spec.py            # regenerate docs/gen/ from spec/content/
  uv run python tools/gen_spec.py --demo     # regenerate docs/demo/ from the fixture

Deterministic byte-for-byte: LF newlines, utf-8, no timestamps/randomness.
Narrative docstrings are read via ast (no code execution); the graph is loaded
via the framework loader (content) or the fixture import (--demo).
"""

from __future__ import annotations

import argparse
import ast
import importlib.util
import os
import re
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path

# --- Make the tensio package importable (model is the source of truth) ------

SPEC_ROOT = Path(__file__).resolve().parents[1]  # .../spec
REPO_ROOT = SPEC_ROOT.parent  # .../HotamSpec
SRC = SPEC_ROOT / "src" / "tensio"
DEMO_DIR = REPO_ROOT / "docs" / "demo"
CLAUDE_MD = REPO_ROOT / "CLAUDE.md"
DOMAINS_ROOT = REPO_ROOT / "domains"


def _resolve_active_gen_dir() -> Path:
    """Return the active gen dir: domains/<first>/docs/gen/ or legacy docs/gen/.

    Computed once at import time for backward-compat with tests that reference
    gen_spec.REQUIREMENTS_MD etc. as module-level paths.
    """
    if DOMAINS_ROOT.exists():
        domain_dirs = sorted(
            d
            for d in DOMAINS_ROOT.iterdir()
            if d.is_dir() and not d.name.startswith("_")
        )
        if domain_dirs:
            return domain_dirs[0] / "docs" / "gen"
    return REPO_ROOT / "docs" / "gen"


def _resolve_active_agents_root() -> Path:
    """Return the active agents root for AGENT-MAP scanning.

    Priority:
      1. domains/<first>/agents/director/agents/ — nested sub-agents of the director.
      2. Legacy spec/agents/ — pre-migration location.

    WHY: after P17 migration the top-level agents live inside the domain's
    director; the legacy spec/agents/ is gone. Returns a Path that may not
    exist (callers guard with .exists()).
    """
    if DOMAINS_ROOT.exists():
        domain_dirs = sorted(
            d
            for d in DOMAINS_ROOT.iterdir()
            if d.is_dir() and not d.name.startswith("_")
        )
        for domain_dir in domain_dirs:
            director_agents = domain_dir / "agents" / "director" / "agents"
            if director_agents.exists():
                return director_agents
    return SPEC_ROOT / "agents"


GEN_DIR = _resolve_active_gen_dir()
_AGENTS_ROOT = _resolve_active_agents_root()

if str(SPEC_ROOT / "src") not in sys.path:
    sys.path.insert(0, str(SPEC_ROOT / "src"))

from tensio.conflict import DECIDED_PREFIX, REVISIT_PREFIX, Conflict  # noqa: E402
from tensio.glossary import TERMS, Term  # noqa: E402
from tensio.graph import (  # noqa: E402
    TensionGraph,
    conflicts_by_axis,
    latent_connector_suspects,
    load_content_graph,
)
from tensio.requirement import DRAFT, ENFORCED, SETTLED  # noqa: E402

# --- CLAUDE.md live-state sentinels -----------------------------------------

_LS_BEGIN = "<!-- LIVE-STATE:BEGIN -->"
_LS_END = "<!-- LIVE-STATE:END -->"

_CONST_BEGIN = "<!-- CONSTITUTION:BEGIN -->"
_CONST_END = "<!-- CONSTITUTION:END -->"

_REPO_MAP_BEGIN = "<!-- REPO-MAP:BEGIN -->"
_REPO_MAP_END = "<!-- REPO-MAP:END -->"

_AGENT_MAP_BEGIN = "<!-- AGENT-MAP:BEGIN -->"
_AGENT_MAP_END = "<!-- AGENT-MAP:END -->"

_SHARED_DOCS_BEGIN = "<!-- SHARED-DOCS:BEGIN -->"
_SHARED_DOCS_END = "<!-- SHARED-DOCS:END -->"

_CONCEPT_MAP_BEGIN = "<!-- CONCEPT-MAP:BEGIN -->"
_CONCEPT_MAP_END = "<!-- CONCEPT-MAP:END -->"

_DOMAIN_MAP_BEGIN = "<!-- DOMAIN-MAP:BEGIN -->"
_DOMAIN_MAP_END = "<!-- DOMAIN-MAP:END -->"

_THINKING_INDEX_BEGIN = "<!-- THINKING-INDEX:BEGIN -->"
_THINKING_INDEX_END = "<!-- THINKING-INDEX:END -->"

# φ-cap: 1_000_000 / φ ≈ 618033 tokens — the crystal must stay well below.
_PHI_CAP = int(1_000_000 / 1.618033988749895)  # 618033

SPEC_DOCS_THINKING_DIR = SPEC_ROOT / "docs" / "thinking"
SPEC_DOCS_TOOLS_DIR = SPEC_ROOT / "docs" / "tools"

ATOMS_DIR = REPO_ROOT / "docs" / "methodology" / "atoms"
ATOMS_OPERATOR_MD = ATOMS_DIR / "operator.md"
ATOMS_SUBSTRATE_MD = ATOMS_DIR / "substrate.md"
ATOMS_DISCIPLINE_MD = ATOMS_DIR / "discipline.md"
ATOMS_CHECK_MD = ATOMS_DIR / "check.md"

REQUIREMENTS_MD = GEN_DIR / "REQUIREMENTS.md"
TENSIONS_MD = GEN_DIR / "TENSIONS.md"
OPEN_MD = GEN_DIR / "OPEN.md"
UNENFORCED_MD = GEN_DIR / "UNENFORCED.md"
GLOSSARY_MD = GEN_DIR / "GLOSSARY.md"
HISTORY_MD = GEN_DIR / "HISTORY.md"
DECISIONS_MD = GEN_DIR / "DECISIONS.md"
CONSTITUTION_MD = GEN_DIR / "CONSTITUTION.md"

BANNER = (
    "<!-- AUTOGENERATED from spec/src/tensio + spec/content — do not edit by "
    "hand. Edits: docstrings/content -> uv run python tools/gen_spec.py -->"
)

# --- Module order for the narrative section of REQUIREMENTS.md --------------
# (module name without .py, section label). The order IS the document order.

MODULE_ORDER: list[tuple[str, str]] = [
    ("__init__", "Methodology overview + the closed loop"),
    ("stakeholder", "§Stakeholder — owners and stewards"),
    ("axis", "§Axis — controlled vocabulary of tension dimensions"),
    ("assumption", "§Assumption — beliefs with a lifecycle"),
    ("requirement", "§Requirement — the requirement node"),
    ("conflict", "§Conflict — the connector node"),
    ("graph", "§Graph — the store, the loader, and traversal"),
    ("lifecycle", "§Lifecycle — the generic state-machine keystone"),
    ("operator", "§Operator — the acting facet of a Stakeholder"),
    ("process", "§Process / §Goal — behavioral aspect (M12) and Goal type (M19)"),
    ("invariants", "§Invariants — structural form"),
]


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _read(path: Path) -> str:
    """Read source as utf-8, normalizing newlines to LF."""
    return path.read_text(encoding="utf-8").replace("\r\n", "\n").replace("\r", "\n")


def _module_docstring(mod: str) -> str:
    """Top-level docstring of src/tensio/<mod>.py via ast (no code execution)."""
    tree = ast.parse(_read(SRC / f"{mod}.py"))
    return ast.get_docstring(tree) or ""


def _cell(text: str) -> str:
    """Escape text for a markdown table cell (LF -> space, | -> \\|)."""
    return text.replace("\n", " ").replace("|", "\\|")


def _mermaid_id(node_id: str) -> str:
    """Sanitize an object id into a Mermaid-safe node identifier."""
    return node_id.replace("-", "_").replace("~", "_").replace(".", "_")


_EMPTY_NOTICE = (
    "_No domain content loaded — `spec/content/graph.py` is absent or empty. "
    "See CLAUDE.md §How to populate to drop in a domain. The methodology "
    "narrative below is the framework itself and is always present._"
)

# ---------------------------------------------------------------------------
# Tool-derived requirements — the tool-as-requirement projection
# (R-tool-is-its-own-requirement)
# ---------------------------------------------------------------------------

_CANON_RE = re.compile(r"^Canon:\s+§(.+?)\s+[—\-]\s+(.+)$")

SPEC_TOOLS_DIR = SPEC_ROOT / "tools"
SPEC_TESTS_DIR = SPEC_ROOT / "tests"


@dataclass(frozen=True)
class ToolRequirement:
    """Projection of one spec/tools/<basename>.py Canon: §<topic> — <claim> marker."""

    id: str  # "R-tool-<basename>"
    basename: str  # e.g. "apply_proposal"
    canon_section: str  # e.g. "Proposal"
    claim: str  # the claim text from the first docstring line
    enforcer: str  # "test_tool_<basename>" if test file exists, else ""


def _scan_tool_requirements(
    spec_tools_dir: Path | None = None,
) -> list[ToolRequirement]:
    """Walk spec/tools/*.py and project each file with a Canon: §<topic> — <claim> marker.

    Files whose module docstring does NOT open with the Canon: pattern are
    silently skipped — they are "rough" utilities, not part of the constitution.
    Returns a list sorted by basename (deterministic ordering).
    """
    tools_dir = spec_tools_dir or SPEC_TOOLS_DIR
    tests_dir = SPEC_TESTS_DIR
    results: list[ToolRequirement] = []
    for path in sorted(tools_dir.glob("*.py")):
        if path.name.startswith("_"):
            continue
        try:
            src = path.read_text(encoding="utf-8")
            tree = ast.parse(src)
            doc = ast.get_docstring(tree) or ""
        except Exception:
            continue
        first_line = doc.split("\n")[0].strip() if doc else ""
        m = _CANON_RE.match(first_line)
        if not m:
            continue
        basename = path.stem  # e.g. "apply_proposal"
        rid = f"R-tool-{basename.replace('_', '-')}"
        canon_section = m.group(1).strip()
        claim = m.group(2).strip()
        enforcer_path = tests_dir / f"test_tool_{basename}.py"
        enforcer = f"test_tool_{basename}" if enforcer_path.exists() else ""
        results.append(
            ToolRequirement(
                id=rid,
                basename=basename,
                canon_section=canon_section,
                claim=claim,
                enforcer=enforcer,
            )
        )
    return results


# ---------------------------------------------------------------------------
# REQUIREMENTS.md — the roster + the methodology narrative
# ---------------------------------------------------------------------------


def build_requirements(g: TensionGraph) -> str:
    """Build REQUIREMENTS.md (roster table + generated narrative) as an LF string."""
    lines: list[str] = [BANNER, ""]
    lines.append("# REQUIREMENTS.md — Requirement roster & methodology (Tensio)")
    lines.append("")
    lines.append(
        "Generated from the executable model: the methodology narrative comes from "
        "`spec/src/tensio` docstrings (RULE + `Canon:§` + WHY); the roster below "
        "comes from `spec/content/graph.py:build_graph()`. Source of truth is the "
        "code; this text is generated, so it cannot drift from the model."
    )
    lines.append("")
    lines.append("---")
    lines.append("")

    # Roster: requirements.
    lines.append("## Requirement roster")
    lines.append("")
    if g.is_empty():
        lines.append(_EMPTY_NOTICE)
        lines.append("")
    elif not g.requirements:
        lines.append("_No requirements declared in this domain yet._")
        lines.append("")
    else:
        lines.append("| id | status | owner | assumptions | claim |")
        lines.append("|---|---|---|---|---|")
        for r in g.requirements:
            assn = ", ".join(r.assumptions) if r.assumptions else "—"
            lines.append(
                f"| `{r.id}` | {_cell(r.status)} | `{r.owner}` | {_cell(assn)} | "
                f"{_cell(r.claim)} |"
            )
        lines.append("")

    # Roster: stakeholders.
    if g.stakeholders:
        lines.append("## Stakeholders")
        lines.append("")
        lines.append("| id | name | domain |")
        lines.append("|---|---|---|")
        for s in g.stakeholders:
            lines.append(f"| `{s.id}` | {_cell(s.name)} | {_cell(s.domain)} |")
        lines.append("")

    # Roster: assumptions.
    if g.assumptions:
        lines.append("## Assumptions")
        lines.append("")
        lines.append("| id | status | owner | statement |")
        lines.append("|---|---|---|---|")
        for a in g.assumptions:
            lines.append(
                f"| `{a.id}` | {a.status} | `{a.owner}` | {_cell(a.statement)} |"
            )
        lines.append("")

    # Roster: operators (§Operator).
    if g.operators:
        lines.append("## Operators")
        lines.append("")
        lines.append("| id | stakeholder | lifecycle | budget | parent |")
        lines.append("|---|---|---|---|---|")
        for op in g.operators:
            budget = (
                f"{op.context_budget.limit} ({op.context_budget.measure})"
                if op.context_budget.limit
                else "unbounded"
            )
            parent = f"`{op.parent}`" if op.parent else "—"
            lines.append(
                f"| `{op.id}` | `{op.stakeholder}` | {op.lifecycle} "
                f"| {budget} | {parent} |"
            )
        lines.append("")

    # Roster: processes (§Process opt-in behavioral aspect, M12).
    if g.processes:
        lines.append("## Processes")
        lines.append("")
        lines.append("| id | lifecycle | steps | roles | drives |")
        lines.append("|---|---|---|---|---|")
        for p in g.processes:
            step_names = ", ".join(s.name for s in p.steps) if p.steps else "—"
            roles = ", ".join(p.roles_required) if p.roles_required else "—"
            drives = ", ".join(p.drives_entities) if p.drives_entities else "—"
            lines.append(
                f"| `{p.id}` | {p.lifecycle.slug} | {_cell(step_names)} "
                f"| {_cell(roles)} | {_cell(drives)} |"
            )
        lines.append("")

    # Roster: goals (§Goal first-class type, M19).
    if g.goals:
        lines.append("## Goals")
        lines.append("")
        lines.append("| id | owner | lifecycle | target | predicate |")
        lines.append("|---|---|---|---|---|")
        for go in g.goals:
            target = go.target_state.target or "—"
            lines.append(
                f"| `{go.id}` | `{go.owner}` | {go.lifecycle} "
                f"| {_cell(target)} | {_cell(go.target_state.predicate)} |"
            )
        lines.append("")

    # Tool-derived requirements section.
    lines.append("---")
    lines.append("")
    lines.append(build_tool_derived_section())

    # Narrative: module docstrings in order.
    lines.append("---")
    lines.append("")
    lines.append("## Methodology (generated from module docstrings)")
    lines.append("")
    for ordinal, (mod, label) in enumerate(MODULE_ORDER, start=1):
        doc = _module_docstring(mod)
        lines.append(f"### {ordinal}. {label} — `tensio.{mod}`")
        lines.append("")
        if doc:
            lines.append(doc.rstrip())
            lines.append("")

    return "\n".join(lines).rstrip() + "\n"


# ---------------------------------------------------------------------------
# TENSIONS.md — the tension map (nodes, clusters by axis, Mermaid)
# ---------------------------------------------------------------------------


def _conflict_block(c: Conflict) -> list[str]:
    """Render one Conflict node as a markdown sub-block."""
    lines: list[str] = []
    lines.append(f"#### `{c.id}` — {c.axis}")
    lines.append("")
    lines.append(f"- **context:** {c.context}")
    lines.append(f"- **members:** {', '.join(f'`{m}`' for m in c.members)}")
    lines.append(f"- **steward:** `{c.steward}`")
    lines.append(f"- **lifecycle:** {c.lifecycle}")
    if c.shared_assumption:
        lines.append(f"- **shared assumption:** `{c.shared_assumption}`")
    if c.derived:
        lines.append(
            f"- **spawned (lineage):** {', '.join(f'`{d}`' for d in c.derived)}"
        )
    if c.revisit_marker:
        lines.append(f"- **revisit marker:** {c.revisit_marker}")
    lines.append("")
    return lines


def _mermaid(g: TensionGraph) -> list[str]:
    """Render the tension map as a Mermaid graph: R-nodes -> C-nodes <- R-nodes."""
    lines: list[str] = ["```mermaid", "graph TD"]
    # Requirement nodes referenced by any conflict (members + derived).
    referenced: list[str] = []
    seen: set[str] = set()
    for c in g.conflicts:
        for rid in list(c.members) + list(c.derived):
            if rid not in seen:
                seen.add(rid)
                referenced.append(rid)
    for rid in referenced:
        lines.append(f'    {_mermaid_id(rid)}["{rid}"]')
    # Conflict nodes + edges.
    for c in g.conflicts:
        cid = _mermaid_id(c.id)
        lines.append(f'    {cid}{{"{c.id}\\n{c.axis}"}}')
        for m in c.members:
            lines.append(f"    {_mermaid_id(m)} --> {cid}")
        for d in c.derived:
            lines.append(f"    {cid} -.spawns.-> {_mermaid_id(d)}")
    lines.append("```")
    return lines


def build_tensions(g: TensionGraph) -> str:
    """Build TENSIONS.md (the tension map) as an LF string."""
    lines: list[str] = [BANNER, ""]
    lines.append("# TENSIONS.md — The tension map (Tensio)")
    lines.append("")
    lines.append(
        "Generated from `spec/content/graph.py` (the domain's tension graph). A "
        "**Conflict** is a first-class connector NODE — `R-a -> C <- R-b` — "
        "carrying the tension axis, the colliding context, and the shared "
        "assumption that belong to neither requirement. Conflicts CLUSTER by axis: "
        "a cluster of size > 1 is one unresolved architectural choice, not N local "
        "disputes."
    )
    lines.append("")
    lines.append("---")
    lines.append("")

    if g.is_empty():
        lines.append(_EMPTY_NOTICE)
        lines.append("")
        return "\n".join(lines).rstrip() + "\n"

    # Clusters by axis.
    clusters = conflicts_by_axis(g)
    lines.append("## Clusters by axis")
    lines.append("")
    if not clusters:
        lines.append("_No conflict nodes yet._")
        lines.append("")
    for axis, cons in clusters.items():
        kind = "ARCHITECTURAL CHOICE (cluster)" if len(cons) > 1 else "single tension"
        lines.append(f"### Axis `{axis}` — {len(cons)} conflict(s), {kind}")
        lines.append("")
        for c in cons:
            lines.extend(_conflict_block(c))

    # Mermaid map.
    lines.append("## Tension map (Mermaid)")
    lines.append("")
    if g.conflicts:
        lines.extend(_mermaid(g))
    else:
        lines.append("_No conflict nodes to render._")
    lines.append("")

    # Controlled vocabulary reference (per-domain).
    lines.append("## Controlled vocabulary of axes (this domain)")
    lines.append("")
    if not g.axes:
        lines.append("_No axes declared in this domain yet._")
        lines.append("")
    else:
        lines.append("| axis slug | description |")
        lines.append("|---|---|")
        for ax in g.axes:
            lines.append(f"| `{ax.slug}` | {_cell(ax.description)} |")
        lines.append("")

    # Latent-connector suspicions (heuristic).
    suspects = latent_connector_suspects(g)
    lines.append("## Latent-connector suspicions (heuristic, for AI review)")
    lines.append("")
    lines.append(
        "Requirement pairs that SHOULD perhaps have a connector node but do not. "
        "This is a heuristic stub for the deferred detector — a suspicion to judge, "
        "never an auto-materialized conflict."
    )
    lines.append("")
    if not suspects:
        lines.append("_None flagged._")
        lines.append("")
    else:
        lines.append("| left | right | hint |")
        lines.append("|---|---|---|")
        for s in suspects:
            lines.append(f"| `{s.left}` | `{s.right}` | {_cell(s.hint)} |")
        lines.append("")

    return "\n".join(lines).rstrip() + "\n"


# ---------------------------------------------------------------------------
# OPEN.md — the open registry (OPEN requirements + unresolved conflicts)
# ---------------------------------------------------------------------------


def build_open(g: TensionGraph) -> str:
    """Build OPEN.md (open registry) as an LF string."""
    open_reqs = [r for r in g.requirements if r.is_open()]
    unresolved = [c for c in g.conflicts if c.is_unresolved()]

    lines: list[str] = [BANNER, ""]
    lines.append("# OPEN.md — Open registry (Tensio)")
    lines.append("")
    lines.append(
        "Generated mirror of what is still open: OPEN(question) requirements and "
        "conflicts not yet resolved by a steward (DETECTED / ACKNOWLEDGED). This is "
        "the visibility-of-the-open layer; run `tools/what_now.py` for the "
        "prioritized next actions that close these."
    )
    lines.append("")
    if g.is_empty():
        lines.append(_EMPTY_NOTICE)
        lines.append("")
        return "\n".join(lines).rstrip() + "\n"

    lines.append(
        f"Open requirements: **{len(open_reqs)}**. "
        f"Unresolved conflicts: **{len(unresolved)}**."
    )
    lines.append("")
    lines.append("---")
    lines.append("")

    lines.append("## OPEN requirements")
    lines.append("")
    if not open_reqs:
        lines.append("_None._")
        lines.append("")
    else:
        lines.append("| id | owner | question |")
        lines.append("|---|---|---|")
        for r in open_reqs:
            question = r.status[len("OPEN") :].strip().strip("()").strip()
            lines.append(f"| `{r.id}` | `{r.owner}` | {_cell(question)} |")
        lines.append("")

    lines.append("## Unresolved conflicts (no steward resolution yet)")
    lines.append("")
    if not unresolved:
        lines.append("_None._")
        lines.append("")
    else:
        lines.append("| id | axis | lifecycle | steward | members |")
        lines.append("|---|---|---|---|---|")
        for c in unresolved:
            mem = ", ".join(c.members)
            lines.append(
                f"| `{c.id}` | `{c.axis}` | {c.lifecycle} | `{c.steward}` | "
                f"{_cell(mem)} |"
            )
        lines.append("")

    return "\n".join(lines).rstrip() + "\n"


# ---------------------------------------------------------------------------
# UNENFORCED.md — burn-down meter (enforcement gradient)
# ---------------------------------------------------------------------------


def build_unenforced(g: TensionGraph) -> str:
    """Build UNENFORCED.md (burn-down meter for the enforcement gradient) as an LF string.

    Canon: §Requirement — UNENFORCED.md is the generated mirror of the
    enforcement gradient (R-enforcement-gradient / R-requirement-enforced).
    It lists every SETTLED requirement whose enforcement is NOT yet ENFORCED,
    the ENFORCED ones (the substrate's reflexes), and a brief DRAFT roster.
    The burn-down ratio line IS the meter: a healthy direction is SETTLED-ENFORCED
    growing while UNENFORCED shrinks.
    """
    settled = [r for r in g.requirements if r.status == SETTLED]
    draft = [r for r in g.requirements if r.status == DRAFT]
    open_reqs = [r for r in g.requirements if r.is_open()]
    rejected = [r for r in g.requirements if r.status == "REJECTED"]

    settled_enforced = [r for r in settled if r.enforcement == ENFORCED]
    settled_unenforced = [r for r in settled if r.enforcement != ENFORCED]

    lines: list[str] = [BANNER, ""]
    lines.append("# UNENFORCED.md — Burn-down meter (Tensio)")
    lines.append("")
    lines.append(
        "Generated mirror of the enforcement gradient. Every requirement carries\n"
        "`enforcement: PROSE | STRUCTURAL | ENFORCED` (R-enforcement-gradient). This\n"
        "report lists every SETTLED requirement whose enforcement is NOT yet ENFORCED —\n"
        "i.e. claimed but not guaranteed, soft context-debt (R-requirement-enforced)."
    )
    lines.append("")
    lines.append(
        "The ratio line below IS the burn-down meter: a healthy direction is SETTLED-ENFORCED\n"
        "growing while UNENFORCED (PROSE+STRUCTURAL of SETTLED) shrinks."
    )
    lines.append("")

    if g.is_empty():
        lines.append(_EMPTY_NOTICE)
        lines.append("")
        return "\n".join(lines).rstrip() + "\n"

    lines.append(
        f"**Burn-down: SETTLED-ENFORCED {len(settled_enforced)} / SETTLED {len(settled)}; "
        f"DRAFT {len(draft)}; OPEN {len(open_reqs)}; REJECTED {len(rejected)}.**"
    )
    lines.append("")
    lines.append("---")
    lines.append("")

    lines.append("## SETTLED but UNENFORCED")
    lines.append("")
    if not settled_unenforced:
        lines.append("_None — all SETTLED requirements are ENFORCED._")
        lines.append("")
    else:
        lines.append("| id | enforcement | owner | claim |")
        lines.append("|---|---|---|---|")
        for r in settled_unenforced:
            lines.append(
                f"| `{r.id}` | {r.enforcement} | `{r.owner}` | {_cell(r.claim)} |"
            )
        lines.append("")

    lines.append("## SETTLED and ENFORCED (the substrate's automatic reflexes)")
    lines.append("")
    if not settled_enforced:
        lines.append("_None yet._")
        lines.append("")
    else:
        lines.append("| id | enforced_by | claim |")
        lines.append("|---|---|---|")
        for r in settled_enforced:
            by = ", ".join(r.enforced_by) if r.enforced_by else "—"
            lines.append(f"| `{r.id}` | {_cell(by)} | {_cell(r.claim)} |")
        lines.append("")

    lines.append("## DRAFT (not yet promoted)")
    lines.append("")
    if not draft:
        lines.append("_No DRAFT requirements._")
        lines.append("")
    else:
        lines.append("| id | owner |")
        lines.append("|---|---|")
        for r in draft:
            lines.append(f"| `{r.id}` | `{r.owner}` |")
        lines.append("")

    return "\n".join(lines).rstrip() + "\n"


# ---------------------------------------------------------------------------
# GLOSSARY.md — the methodology's controlled vocabulary
# ---------------------------------------------------------------------------


def build_glossary(g: TensionGraph) -> str:  # noqa: ARG001
    """Build GLOSSARY.md (controlled vocabulary) as an LF string.

    Canon: §Glossary — generated mirror of tensio.glossary.TERMS. The graph
    argument is accepted for API consistency with the other builders but not
    used: the glossary is framework-side, not domain content.
    """
    lines: list[str] = [BANNER, ""]
    lines.append("# GLOSSARY.md — Methodology controlled vocabulary (Tensio)")
    lines.append("")
    lines.append(
        "Generated mirror of the methodology's own canon terms — the framework's\n"
        "controlled vocabulary that every docstring and generated doc must use\n"
        "consistently. Terminology drift is invisibility (R-glossary-sync-test)."
    )
    lines.append("")
    lines.append(
        "Source: `spec/src/tensio/glossary.py:TERMS`. Domain-side business terms\n"
        "(R-ids, axis slugs, stakeholders) live in `spec/content/graph.py` and are\n"
        "listed in REQUIREMENTS.md / TENSIONS.md — not duplicated here."
    )
    lines.append("")
    lines.append("---")
    lines.append("")

    # Group terms by kind, preserving TERMS order within each group.
    KIND_ORDER = ["SECTION", "LIFECYCLE_STATE", "STATUS", "ROLE", "CONCEPT"]
    KIND_LABELS = {
        "SECTION": "Sections (§-anchors)",
        "LIFECYCLE_STATE": "Lifecycle states",
        "STATUS": "Statuses",
        "ROLE": "Roles",
        "CONCEPT": "Concepts",
    }

    grouped: dict[str, list[Term]] = {k: [] for k in KIND_ORDER}
    for term in TERMS:
        if term.kind in grouped:
            grouped[term.kind].append(term)

    for kind in KIND_ORDER:
        entries = grouped[kind]
        if not entries:
            continue
        lines.append(f"## {KIND_LABELS[kind]}")
        lines.append("| slug | definition |")
        lines.append("|---|---|")
        for term in entries:
            lines.append(f"| `{_cell(term.slug)}` | {_cell(term.definition)} |")
        lines.append("")

    return "\n".join(lines).rstrip() + "\n"


# ---------------------------------------------------------------------------
# HISTORY.md — methodology decision history (REJECTED reqs + DECIDED conflicts)
# ---------------------------------------------------------------------------


def _extract_decided_rationale(lifecycle: str) -> str:
    """Extract the rationale from a DECIDED(...) lifecycle string.

    DECIDED(some rationale here) -> "some rationale here"
    Returns "" if the string is not a properly-formed DECIDED block.
    """
    if not lifecycle.startswith(DECIDED_PREFIX):
        return ""
    inner = lifecycle[len(DECIDED_PREFIX) :].strip()
    if inner.startswith("(") and inner.endswith(")"):
        return inner[1:-1].strip()
    return inner


def _extract_revisit_rationale(lifecycle: str) -> str:
    """Extract the condition from a REVISIT_WHEN(...) lifecycle string."""
    if not lifecycle.startswith(REVISIT_PREFIX):
        return ""
    inner = lifecycle[len(REVISIT_PREFIX) :].strip()
    if inner.startswith("(") and inner.endswith(")"):
        return inner[1:-1].strip()
    return inner


def build_history(g: TensionGraph) -> str:
    """Build HISTORY.md (methodology decision history) as an LF string.

    Canon: §Conflict / §Requirement — generated mirror of the anti-relitigation
    markers: REJECTED requirements (what was tried and discarded) and
    DECIDED / REVISIT_WHEN conflict lifecycles (what was resolved, why, and
    the condition under which to re-open). Source of truth is
    spec/content/graph.py; this text is generated so it cannot drift.
    """
    lines: list[str] = [BANNER, ""]
    lines.append("# HISTORY.md — Methodology decision history (Tensio)")
    lines.append("")
    lines.append(
        "Generated from the anti-relitigation markers in the model: REJECTED\n"
        "requirements (what was tried and discarded — REPLACES marker) and DECIDED /\n"
        "REVISIT_WHEN conflict lifecycles (what was resolved, why, and the condition\n"
        "under which to re-open). Source of truth is `spec/content/graph.py`; this\n"
        "text is generated so it cannot drift."
    )
    lines.append("")
    lines.append(
        "A fresh agent reads this to recover the methodology's history without\n"
        "re-litigating settled questions — the historian role of the AI made into\n"
        "substrate (R-history-from-rejected-markers)."
    )
    lines.append("")
    lines.append("---")
    lines.append("")

    if g.is_empty():
        lines.append(_EMPTY_NOTICE)
        lines.append("")
        return "\n".join(lines).rstrip() + "\n"

    # --- REJECTED requirements -----------------------------------------------
    rejected = [r for r in g.requirements if r.status == "REJECTED"]

    lines.append("## REJECTED requirements (what we tried and discarded)")
    lines.append("")
    if not rejected:
        lines.append("_None._")
        lines.append("")
    else:
        for r in rejected:
            lines.append(f"### `{r.id}` — {_cell(r.claim)}")
            lines.append("")
            lines.append(f"- **owner:** `{r.owner}`")
            lines.append(f"- **why:** {r.why}")
            lines.append("")

    # --- DECIDED conflicts ----------------------------------------------------
    decided = [c for c in g.conflicts if c.is_decided()]

    lines.append("## DECIDED conflicts (resolutions on record)")
    lines.append("")
    if not decided:
        lines.append("_None._")
        lines.append("")
    else:
        for c in decided:
            rationale = _extract_decided_rationale(c.lifecycle)
            lines.append(f"### `{c.id}` — axis `{c.axis}`")
            lines.append("")
            lines.append(f"- **context:** {c.context}")
            lines.append(f"- **members:** {', '.join(f'`{m}`' for m in c.members)}")
            lines.append(f"- **steward:** `{c.steward}`")
            lines.append(f"- **rationale:** {rationale}")
            if c.shared_assumption:
                lines.append(f"- **shared assumption:** `{c.shared_assumption}`")
            if c.derived:
                lines.append(
                    f"- **spawned (derived):** {', '.join(f'`{d}`' for d in c.derived)}"
                )
            if c.revisit_marker:
                lines.append(f"- **revisit when:** {c.revisit_marker}")
            lines.append("")

    # --- REVISIT_WHEN parked decisions ----------------------------------------
    parked = [c for c in g.conflicts if c.lifecycle.startswith(REVISIT_PREFIX)]

    lines.append("## Parked decisions (REVISIT_WHEN)")
    lines.append("")
    if not parked:
        lines.append("_None._")
        lines.append("")
    else:
        for c in parked:
            condition = _extract_revisit_rationale(c.lifecycle)
            lines.append(f"### `{c.id}` — axis `{c.axis}`")
            lines.append("")
            lines.append(f"- **context:** {c.context}")
            lines.append(f"- **members:** {', '.join(f'`{m}`' for m in c.members)}")
            lines.append(f"- **steward:** `{c.steward}`")
            lines.append(f"- **condition:** {condition}")
            if c.shared_assumption:
                lines.append(f"- **shared assumption:** `{c.shared_assumption}`")
            if c.derived:
                lines.append(
                    f"- **spawned (derived):** {', '.join(f'`{d}`' for d in c.derived)}"
                )
            lines.append("")

    return "\n".join(lines).rstrip() + "\n"


# ---------------------------------------------------------------------------
# DECISIONS.md — generated M-registry (open decisions mirrored from graph)
# ---------------------------------------------------------------------------


def _extract_open_question(status: str) -> str:
    """Extract the question text from an OPEN(...) status string.

    OPEN(some question here) -> "some question here"
    Returns the raw status string if it does not parse.
    """
    stripped = status[len("OPEN") :].strip()
    if stripped.startswith("(") and stripped.endswith(")"):
        return stripped[1:-1].strip()
    return status


def build_decisions(g: TensionGraph) -> str:
    """Build DECISIONS.md (generated M-registry) as an LF string.

    Canon: §Requirement — generated mirror of the M-registry.  Every OPEN
    requirement whose `m_tag` is non-empty appears here, sorted by the
    integer value of the M-tag.  This file retires the hand-maintained
    M-table in CLAUDE.md as the source of truth, per
    `R-drift-structurally-impossible` and the dev-coin Param.status +
    HOLES.md precedent: one source of truth, generated mirror.
    """
    tagged = [r for r in g.requirements if r.m_tag]
    # Sort by integer value of tag (M3 < M5 < M17 ...)
    tagged_sorted = sorted(tagged, key=lambda r: int(r.m_tag[1:]))

    lines: list[str] = [BANNER, ""]
    lines.append("# DECISIONS.md — Open methodology decisions (Tensio)")
    lines.append("")
    lines.append(
        "Generated mirror of the M-registry. The SINGLE source of truth is the\n"
        "graph's OPEN requirements with non-empty `m_tag` in\n"
        "`spec/content/graph.py`. This file retires the hand-maintained M-table\n"
        "that lived in CLAUDE.md — per `R-drift-structurally-impossible` and the\n"
        "dev-coin Param.status + HOLES.md precedent: one source of truth,\n"
        "generated mirror."
    )
    lines.append("")
    lines.append(
        "A requirement carries an M-tag iff it mirrors an open methodology\n"
        "decision the steward must ratify. Requirements without an M-tag are\n"
        "domain-level open holes that have not been elevated to\n"
        "methodology-altitude decisions."
    )
    lines.append("")
    lines.append("---")
    lines.append("")

    if g.is_empty():
        lines.append(_EMPTY_NOTICE)
        lines.append("")
        return "\n".join(lines).rstrip() + "\n"

    lines.append("## Open decisions (sorted by M-tag)")
    lines.append("")
    if not tagged_sorted:
        lines.append("_No OPEN requirements carry an M-tag yet._")
        lines.append("")
    else:
        lines.append("| M-tag | requirement | owner | question |")
        lines.append("|---|---|---|---|")
        for r in tagged_sorted:
            question = _extract_open_question(r.status)
            lines.append(f"| {r.m_tag} | `{r.id}` | `{r.owner}` | {_cell(question)} |")
        lines.append("")

    lines.append("## Notes")
    lines.append("")
    lines.append(
        "Decision-IDs not yet anchored to a graph requirement (no `m_tag` mirror)\n"
        "remain prose-only in CLAUDE.md. The convergence direction is to\n"
        "crystallize each such M-row as a Requirement with the corresponding\n"
        "`m_tag`."
    )
    lines.append("")

    return "\n".join(lines).rstrip() + "\n"


# ---------------------------------------------------------------------------
# CONSTITUTION.md — the operator's boot sequence (generated from SETTLED laws)
# ---------------------------------------------------------------------------

# The set of requirement ids that constitute the operator's boot sequence.
# These are the laws — closed loop + hard boundary + super-rules + self/delegation
# + loop machinery + conscience. Selection is authoritative; verified by test.
CONSTITUTION_SET: frozenset[str] = frozenset(
    {
        # Closed loop & operator role
        "R-agent-never-lost",
        "R-drift-structurally-impossible",
        "R-deterministic-generation",
        "R-conflict-is-connector-node",
        "R-two-altitude-ontology",
        "R-empty-content-is-legitimate",
        # Hard boundary
        "R-ai-presents-not-decides",
        "R-steward-distinct-from-owners",
        "R-operator-not-self-approve",
        "R-decided-needs-human-signoff",
        "R-open-states-question",
        "R-rejected-preserved-not-deleted",
        "R-axis-controlled-vocab",
        "R-stable-conflict-identity",
        # Self + delegation
        "R-operator-acting-facet",
        "R-context-budget-rule",
        "R-operator-crystal-is-claude-md",
        # Super-rules (crystallize + anchor)
        "R-crystallize-knowledge-to-code",
        "R-crystallize-before-split",
        "R-working-vs-substrate-budget",
        "R-enforcement-gradient",
        "R-requirement-enforced",
        "R-anchor-everything",
        "R-speak-by-reference",
        # Loop machinery
        "R-active-loop-playbooks",
        "R-verify-closure-per-action",
        # Conscience
        "R-uncrystallizable-is-missing-type",
        "R-stale-substrate",
        "R-critical-core-scope",
    }
)

# Category grouping for §7 table — preserves the constitution-set categories
# in the specified order; within each group requirements appear in graph order.
_CONSTITUTION_CATEGORIES: list[tuple[str, frozenset[str]]] = [
    (
        "Closed loop & operator role",
        frozenset(
            {
                "R-agent-never-lost",
                "R-drift-structurally-impossible",
                "R-deterministic-generation",
                "R-conflict-is-connector-node",
                "R-two-altitude-ontology",
                "R-empty-content-is-legitimate",
            }
        ),
    ),
    (
        "Hard boundary",
        frozenset(
            {
                "R-ai-presents-not-decides",
                "R-steward-distinct-from-owners",
                "R-operator-not-self-approve",
                "R-decided-needs-human-signoff",
                "R-open-states-question",
                "R-rejected-preserved-not-deleted",
                "R-axis-controlled-vocab",
                "R-stable-conflict-identity",
            }
        ),
    ),
    (
        "Self + delegation",
        frozenset(
            {
                "R-operator-acting-facet",
                "R-context-budget-rule",
                "R-operator-crystal-is-claude-md",
            }
        ),
    ),
    (
        "Super-rules (crystallize + anchor)",
        frozenset(
            {
                "R-crystallize-knowledge-to-code",
                "R-crystallize-before-split",
                "R-working-vs-substrate-budget",
                "R-enforcement-gradient",
                "R-requirement-enforced",
                "R-anchor-everything",
                "R-speak-by-reference",
            }
        ),
    ),
    (
        "Loop machinery",
        frozenset(
            {
                "R-active-loop-playbooks",
                "R-verify-closure-per-action",
            }
        ),
    ),
    (
        "Conscience",
        frozenset(
            {
                "R-uncrystallizable-is-missing-type",
                "R-stale-substrate",
                "R-critical-core-scope",
            }
        ),
    ),
]

# The six critical-core invariant function names (verbatim — §5).
_CRITICAL_CORE_NAMES: tuple[str, ...] = (
    "check_steward_not_a_member_owner",
    "check_operator_steward_not_self",
    "check_decided_has_decided_by",
    "check_typed_anchors",
    "check_no_dangling_ids",
    "check_open_has_question",
)


def build_constitution(g: TensionGraph) -> str:
    """Build CONSTITUTION.md (the operator's boot sequence) as an LF string.

    Canon: §Constitution — generated reconstitution from the substrate's
    SETTLED laws. A fresh agent reading this file cold reconstitutes as
    operator #1 (OP-director) without needing a session checkpoint. The
    source of truth is the SETTLED requirements in CONSTITUTION_SET;
    this text is generated so it cannot drift (R-drift-structurally-impossible).
    """
    # Index requirements by id for fast lookup.
    req_by_id = {r.id: r for r in g.requirements}

    lines: list[str] = [BANNER, ""]
    lines.append("# CONSTITUTION.md — The operator's boot sequence (Tensio)")
    lines.append("")
    lines.append(
        "You — the AI agent reading this cold — are the prospective Operator of this\n"
        "repository. Read this file end-to-end before any action. It is generated from\n"
        "the methodology's SETTLED laws (`spec/content/graph.py`). It is your\n"
        "*reconstitution from the substrate*: you do not need a session checkpoint\n"
        "to know what to do."
    )
    lines.append("")

    # --- §1. The role ---
    lines.append("## 1. The role")
    lines.append("")
    lines.append(
        "You are operator #1 — the **acting facet** of a Stakeholder. (R-operator-acting-facet)"
    )
    op_ids = [op.id for op in g.operators] if g.operators else ["OP-director"]
    lines.append(
        f"You appear in the graph as `{op_ids[0]}`. (verify by reading `g.operators`)"
    )
    lines.append(
        "You read CLAUDE.md as your crystal — the anchored map of the whole graph. "
        "(R-operator-crystal-is-claude-md)"
    )
    lines.append(
        "You speak by REFERENCE to anchors, not by re-carrying content. (R-speak-by-reference)"
    )
    lines.append("")

    # --- §2. The closed loop ---
    lines.append("## 2. The closed loop (the operating procedure)")
    lines.append("")
    # Pull the closed-loop description from the tensio __init__ docstring.
    closed_loop_doc = _module_docstring("__init__")
    # Extract the THE CLOSED LOOP section.
    loop_text = ""
    if "THE CLOSED LOOP" in closed_loop_doc:
        start = closed_loop_doc.index("THE CLOSED LOOP")
        end = (
            closed_loop_doc.index("\nTHE AI", start)
            if "\nTHE AI" in closed_loop_doc[start:]
            else len(closed_loop_doc)
        )
        loop_text = closed_loop_doc[start:end].strip()
    if loop_text:
        lines.append(loop_text)
    else:
        lines.append(
            "State (graph + generated docs + test status)\n"
            "  -> Diagnosis (tools/what_now.py)\n"
            "  -> Next-action (typed, prioritized)\n"
            "  -> Action (edit the graph)\n"
            "  -> regenerate (tools/gen_spec.py)\n"
            "  -> State."
        )
    lines.append("")
    lines.append(
        "Anchors: R-agent-never-lost, R-deterministic-generation, R-drift-structurally-impossible."
    )
    lines.append("")

    # --- §3. The hard boundary ---
    lines.append("## 3. The hard boundary")
    lines.append("")
    hard_boundary_ids = [
        "R-ai-presents-not-decides",
        "R-steward-distinct-from-owners",
        "R-operator-not-self-approve",
        "R-decided-needs-human-signoff",
        "R-open-states-question",
        "R-rejected-preserved-not-deleted",
        "R-axis-controlled-vocab",
        "R-stable-conflict-identity",
    ]
    for rid in hard_boundary_ids:
        r = req_by_id.get(rid)
        if r:
            lines.append(f"**{rid}** — {r.claim}")
            lines.append("")
    if g.is_empty():
        lines.append("_No content domain yet — but the hard boundary laws still hold._")
        lines.append("")

    # --- §4. The two super-rules (context discipline) ---
    lines.append("## 4. The two super-rules (context discipline)")
    lines.append("")
    super_rule_ids = [
        ("CRYSTALLIZE", "R-crystallize-knowledge-to-code"),
        ("ANCHOR", "R-anchor-everything"),
        ("REFERENCE", "R-speak-by-reference"),
        ("ORDER", "R-crystallize-before-split"),
        ("BUDGET", "R-working-vs-substrate-budget"),
    ]
    for label, rid in super_rule_ids:
        r = req_by_id.get(rid)
        if r:
            lines.append(f"**{label}** ({rid}):")
            lines.append(f"  Claim: {r.claim}")
            lines.append(f"  Why: {r.why}")
            lines.append("")
    if g.is_empty():
        lines.append("_No content domain yet — but the super-rule laws still hold._")
        lines.append("")

    # --- §5. The conscience ---
    lines.append("## 5. The conscience")
    lines.append("")
    r_ccs = req_by_id.get("R-critical-core-scope")
    if r_ccs:
        lines.append(r_ccs.claim)
        lines.append("")
        lines.append(r_ccs.why)
        lines.append("")
    lines.append(
        "The six critical-core invariants (M7 / R-critical-core-scope) — verified on every run by "
        "`tests/test_conscience.py`. Do NOT skip them; do NOT soften them."
    )
    lines.append("")
    lines.append(
        "The six `CRITICAL_CORE_INVARIANTS` (verbatim function names from "
        "`tensio.invariants`):"
    )
    lines.append("")
    for name in _CRITICAL_CORE_NAMES:
        lines.append(f"  - `{name}`")
    lines.append("")

    # --- §6. The boot sequence ---
    lines.append("## 6. The boot sequence (what to do RIGHT NOW)")
    lines.append("")
    lines.append("Run, in order:")
    lines.append("")
    lines.append(
        "  1. `cd D:/dev/HotamSpec/spec && uv run pytest -q`     → suite green?"
    )
    lines.append(
        "  2. `uv run python tools/gen_spec.py` (twice)          → deterministic?"
    )
    lines.append(
        "  3. `uv run python tools/what_now.py | head -20`       → what is the top action?"
    )
    lines.append(
        "  4. `uv run python tools/tick.py`                      → does the tick agree?"
    )
    lines.append(
        "  5. Read `docs/gen/UNENFORCED.md`                      → what's claimed but not guaranteed?"
    )
    lines.append(
        "  6. Read `docs/gen/HISTORY.md`                         → what's been decided / rejected?"
    )
    lines.append(
        "  7. Read `docs/gen/DECISIONS.md`                       → which M-decisions are open?"
    )
    lines.append("")
    lines.append(
        "If the top action is P3 CONFLICT_STALLED: invoke the relevant playbook\n"
        "(`docs/playbooks/`), surface assumptions, propose 2-3 variants, get steward\n"
        "approval, apply via `tools/apply_proposal.py --triggering-kind CONFLICT_STALLED`.\n"
        "The closure check (R-verify-closure-per-action) will confirm advancement."
    )
    lines.append("")
    lines.append(
        "If the top action is P4 OPEN_ITEM: same procedure with\n"
        "`--triggering-kind OPEN_ITEM`."
    )
    lines.append("")
    lines.append(
        "If the top action is P1 STRUCTURE: stop. A structural violation means the\n"
        "graph is malformed — investigate the root cause; do not edit by hand.\n"
        "`tools/apply_proposal.py` refuses non-stewarded structural changes."
    )
    lines.append("")

    # --- §7. The methodology's laws (full constitutional set) ---
    lines.append("## 7. The methodology's laws (full constitutional set)")
    lines.append("")
    if g.is_empty():
        lines.append(
            "_No content domain loaded yet — `spec/content/graph.py` is absent or "
            "empty. The framework laws above still hold; the roster below will "
            "populate once a domain is loaded._"
        )
        lines.append("")
    else:
        lines.append("| anchor | enforcement | claim |")
        lines.append("|---|---|---|")
        for cat_label, cat_ids in _CONSTITUTION_CATEGORIES:
            # Emit a sub-header row in the table for readability.
            lines.append(f"| **{cat_label}** | | |")
            # Iterate in graph order within this category.
            for r in g.requirements:
                if r.id in cat_ids:
                    enf = r.enforcement if r.enforcement else "PROSE"
                    lines.append(f"| `{r.id}` | {enf} | {_cell(r.claim)} |")
        lines.append("")

    # --- §8. What is yours; what is not ---
    lines.append("## 8. What is yours; what is not")
    lines.append("")
    lines.append("YOUR scope (within the hard boundary):")
    lines.append("")
    lines.append(
        "  - propose Requirements / Conflict transitions / Rejections via the proposal"
    )
    lines.append("    protocol;")
    lines.append("  - run `tick.py`, `what_now.py`, `gen_spec.py`;")
    lines.append("  - call `apply_proposal.py` with a steward-approved JSON;")
    lines.append("  - crystallize working knowledge into requirement-code;")
    lines.append("  - cite anchors in every communication.")
    lines.append("")
    lines.append("NOT yours (steward's act):")
    lines.append("")
    lines.append("  - approving a proposal (the steward writes the `decided_by`);")
    lines.append("  - resolving an OPEN(question) requirement's content;")
    lines.append("  - closing a Conflict (the operator presents, the steward decides);")
    lines.append(
        "  - running `git commit` (the act of recording in history is the steward's)."
    )
    lines.append("")
    lines.append(
        "This is verbatim from R-ai-presents-not-decides + R-operator-not-self-approve."
    )
    lines.append("")

    # --- §9. If you are unsure ---
    lines.append("## 9. If you are unsure")
    lines.append("")
    lines.append(
        "Re-read this file. Then read CLAUDE.md (your crystal — the index).\n"
        "If a question remains, surface it to the steward as a `ProposedRequirement`\n"
        "with status OPEN(<question>). That is how the methodology questions itself."
    )
    lines.append("")

    return "\n".join(lines).rstrip() + "\n"


# ---------------------------------------------------------------------------
# LIVE-STATE block — generated fragment inside CLAUDE.md (§Context / P10a)
# ---------------------------------------------------------------------------

# WHY coarse φ-cap bucket (not exact token count): the LIVE-STATE block lives
# INSIDE CLAUDE.md, so computing CLAUDE.md byte-size with the old block and
# inserting a new block containing that size would create a fixpoint trap —
# the size changes each time the number of digits in the count changes. The
# coarse bucket ("well under φ-cap") is stable across regenerations because
# CLAUDE.md is ~7K tokens (1% of 618K cap) and the bucket boundary is at 50%
# (~309K tokens), far away. Only switch to a precise token count near the cap.
_PHI_CAP_BUCKET_THRESHOLD = _PHI_CAP // 2  # ~309016 tokens ≈ 1.2 MB


def build_live_state(g: TensionGraph) -> str:
    """Build the LIVE-STATE block content (without sentinels) as an LF string.

    Canon: §Context — the three-cipher pulse numbers come from HERE, not from
    hand-written prose. Pure function of g + static φ-cap + the context reader
    (which returns UNMEASURED deterministically when the runtime stamp is absent,
    so the output is deterministic in tests).

    WHY no CLAUDE.md size inside this function: see fixpoint hazard comment above.
    """
    # Lazy import of what_now and context (same pattern as other tools).
    _tools = Path(__file__).resolve().parent
    if str(_tools) not in sys.path:
        sys.path.insert(0, str(_tools))
    import what_now as _what_now  # noqa: PLC0415
    import context as _context  # noqa: PLC0415

    actions = _what_now.diagnose(g)
    if actions:
        top = actions[0]
        top_line = f"[P{top.priority}] {top.kind} on `{top.target}` — {top.imperative}"
    else:
        top_line = "none — graph clean"

    settled = [r for r in g.requirements if r.status == SETTLED]
    draft = [r for r in g.requirements if r.status == DRAFT]
    open_reqs = [r for r in g.requirements if r.is_open()]
    settled_enforced = sum(1 for r in settled if r.enforcement == ENFORCED)
    settled_total = len(settled)
    unenforced = settled_total - settled_enforced

    nodes = len(g.requirements) + len(g.conflicts) + len(g.assumptions)
    op_budget = 0
    if g.operators:
        op_budget = g.operators[0].context_budget.limit

    # Coarse φ-cap status — fixpoint-safe (see WHY comment above).
    crystal_line = f"CLAUDE.md well under φ-cap ({_PHI_CAP} tokens) — no split needed"

    ctx_line = _context.render_line()

    lines = [
        "### Live state (autogenerated by tools/gen_spec.py — do not hand-edit)",
        "",
        f"- **top action:** {top_line}",
        (
            f"- **debt:** {settled_enforced}/{settled_total} SETTLED ENFORCED"
            f" · {len(draft)} DRAFT · {len(open_reqs)} OPEN"
            f" · {unenforced} SETTLED-unenforced"
        ),
        (
            f"- **graph:** {nodes} nodes (req+conflict+assumption);"
            f" OP-director budget {op_budget}"
            f" (headroom {op_budget - nodes})"
        ),
        f"- **crystal:** {crystal_line}",
        f"- {ctx_line}",
    ]
    return "\n".join(lines)


def extract_live_state_block(claude_md_text: str) -> str | None:
    """Extract the text between LIVE-STATE sentinels (excluding sentinels).

    Returns None if sentinels are not found.
    """
    begin_pos = claude_md_text.find(_LS_BEGIN)
    end_pos = claude_md_text.find(_LS_END)
    if begin_pos == -1 or end_pos == -1 or end_pos <= begin_pos:
        return None
    inner = claude_md_text[begin_pos + len(_LS_BEGIN) : end_pos]
    return inner.strip("\n")


def _update_claude_md_live_state(g: TensionGraph) -> None:
    """Rewrite the LIVE-STATE sentinel block in CLAUDE.md with fresh numbers.

    Idempotent: calling twice on an unchanged graph produces identical CLAUDE.md.
    Only runs in non-demo mode (--demo never touches CLAUDE.md).
    """
    if not CLAUDE_MD.exists():
        return
    text = _read(CLAUDE_MD)
    new_block = build_live_state(g)

    if _LS_BEGIN in text and _LS_END in text:
        begin_pos = text.find(_LS_BEGIN)
        end_pos = text.find(_LS_END)
        before = text[: begin_pos + len(_LS_BEGIN)]
        after = text[end_pos:]
        new_text = before + "\n" + new_block + "\n" + after
    else:
        # Sentinels absent — insert after "### Three-cipher pulse" intro para
        # (the section that ends before the next ### or ##). This is a one-time
        # bootstrap for a CLAUDE.md that has never had the block.
        marker = "### Three-cipher pulse"
        marker_pos = text.find(marker)
        if marker_pos == -1:
            # Cannot find the section; write the sentinels at the end as a fallback.
            new_text = text.rstrip("\n") + f"\n\n{_LS_BEGIN}\n{new_block}\n{_LS_END}\n"
        else:
            # Find the end of the introductory paragraph block for that section
            # (next blank line after the paragraph, before the next heading).
            search_from = marker_pos + len(marker)
            next_heading = text.find("\n###", search_from)
            if next_heading == -1:
                next_heading = text.find("\n##", search_from)
            insert_at = next_heading if next_heading != -1 else len(text)
            new_text = (
                text[:insert_at].rstrip("\n")
                + f"\n\n{_LS_BEGIN}\n{new_block}\n{_LS_END}\n"
                + text[insert_at:].lstrip("\n")
            )

    _write(CLAUDE_MD, new_text)


# ---------------------------------------------------------------------------
# CONSTITUTION block — generated digest of SETTLED requirements in CLAUDE.md
# ---------------------------------------------------------------------------

# Category definitions: (label, id-prefix-or-predicate tuples).
# A requirement is assigned to the FIRST matching category.
_DIGEST_CATEGORIES: list[tuple[str, tuple[str, ...]]] = [
    (
        "Operator",
        ("R-operator-", "R-crystal-", "R-context-", "R-budget-", "R-agent-"),
    ),
    (
        "Substrate / Anchoring",
        ("R-anchor-", "R-speak-", "R-stale-", "R-claude-md-"),
    ),
    (
        "Discipline",
        (
            "R-prefer-",
            "R-crystallize-",
            "R-delegation-",
            "R-task-",
            "R-active-loop-",
            "R-shared-tools-",
            "R-verify-",
            "R-working-",
        ),
    ),
    (
        "Check / Invariant",
        (
            "R-statemachine-",
            "R-bijection-",
            "R-conflict-",
            "R-decided-",
            "R-axis-",
            "R-m-tag-",
            "R-typed-",
            "R-requirement-",
            "R-enforcement-",
            "R-check-",
            "R-stable-",
            "R-steward-",
            "R-open-",
        ),
    ),
    (
        "Framework Self",
        (
            "R-drift-",
            "R-deterministic-",
            "R-content-",
            "R-empty-",
            "R-two-altitude-",
            "R-rejected-",
        ),
    ),
    (
        "Lifecycle / Process / Goal",
        ("R-lifecycle-", "R-process-", "R-goal-"),
    ),
    (
        "Boot / Glossary / History / Docs",
        ("R-boot-", "R-glossary-", "R-history-", "R-docs-"),
    ),
]


def _categorize_requirement(rid: str) -> str:
    """Return the category label for a requirement id. Deterministic."""
    for label, prefixes in _DIGEST_CATEGORIES:
        for prefix in prefixes:
            if rid.startswith(prefix):
                return label
    return "Other"


def _render_constitution_block(g: TensionGraph) -> str:
    """Render the CONSTITUTION digest block content (without sentinels)."""
    settled = [r for r in g.requirements if r.status == SETTLED]
    if not settled:
        return "<!-- (generated by tools/gen_spec.py — do not hand-edit) -->\n\n_No SETTLED requirements yet._"

    # Group by category.
    groups: dict[str, list] = {}
    for r in settled:
        cat = _categorize_requirement(r.id)
        groups.setdefault(cat, []).append(r)

    # Sort within each group by id.
    for cat in groups:
        groups[cat].sort(key=lambda r: r.id)

    # Determine category order: follow _DIGEST_CATEGORIES order, then "Other".
    cat_order = [label for label, _ in _DIGEST_CATEGORIES if label in groups]
    if "Other" in groups:
        cat_order.append("Other")

    lines: list[str] = [
        "<!-- (generated by tools/gen_spec.py — do not hand-edit) -->",
        "",
        "### Constitutional Digest (all SETTLED requirements)",
        "",
    ]
    for cat in cat_order:
        lines.append(f"**{cat}**")
        lines.append("")
        for r in groups[cat]:
            enf = f" [{r.enforcement}"
            if r.enforced_by:
                enf += f"·{'·'.join(r.enforced_by)}"
            enf += "]"
            lines.append(f"- **{r.id}** — *{r.claim}*{enf}")
        lines.append("")

    # Append tool-derived requirements (R-tool-is-its-own-requirement projection).
    tool_reqs = _scan_tool_requirements()
    if tool_reqs:
        lines.append("**Tool-derived requirements**")
        lines.append("")
        for tr in tool_reqs:
            enforcer_str = (
                f"enforcer: `{tr.enforcer}`" if tr.enforcer else "enforcer: (none)"
            )
            lines.append(
                f"- **{tr.id}** — *{tr.claim}* "
                f"[STRUCTURAL·tool · §{tr.canon_section}] [{enforcer_str}]"
            )
        lines.append("")

    return "\n".join(lines).rstrip()


def _update_claude_md_constitution(g: TensionGraph) -> None:
    """Rewrite the CONSTITUTION sentinel block in CLAUDE.md with the digest.

    Idempotent: calling twice on an unchanged graph produces identical CLAUDE.md.
    If sentinels are absent, inserts them after the LIVE-STATE block.
    """
    if not CLAUDE_MD.exists():
        return
    text = _read(CLAUDE_MD)
    new_block = _render_constitution_block(g)

    if _CONST_BEGIN in text and _CONST_END in text:
        begin_pos = text.find(_CONST_BEGIN)
        end_pos = text.find(_CONST_END)
        before = text[: begin_pos + len(_CONST_BEGIN)]
        after = text[end_pos:]
        new_text = before + "\n" + new_block + "\n" + after
    else:
        # Insert after LIVE-STATE:END block.
        ls_end_pos = text.find(_LS_END)
        if ls_end_pos != -1:
            insert_at = ls_end_pos + len(_LS_END)
            new_text = (
                text[:insert_at]
                + f"\n\n{_CONST_BEGIN}\n{new_block}\n{_CONST_END}\n"
                + text[insert_at:]
            )
        else:
            # Fallback: append at end.
            new_text = (
                text.rstrip("\n") + f"\n\n{_CONST_BEGIN}\n{new_block}\n{_CONST_END}\n"
            )

    _write(CLAUDE_MD, new_text)


# ---------------------------------------------------------------------------
# Atomized methodology docs (docs/methodology/atoms/)
# Each generator emits one topic file from SETTLED requirements tagged with
# that topic. Topic-grouping uses a helper that scans requirement ids for
# known prefixes — atomic methods, one per topic.
# ---------------------------------------------------------------------------


def _select_settled(g: TensionGraph, predicate) -> list:
    """Return SETTLED requirements satisfying predicate(r) -> bool. One atom."""
    return [r for r in g.requirements if r.status == SETTLED and predicate(r)]


def _render_atoms(title: str, intro: str, reqs: list) -> str:
    """Render one atoms file from a sorted requirement list. One atom."""
    lines: list[str] = [BANNER, "", f"# {title}", "", intro, "", "---", ""]
    if not reqs:
        lines += ["_No atomic requirements in this topic yet._", ""]
    else:
        for r in sorted(reqs, key=lambda r: r.id):
            lines += [f"## `{r.id}` ({r.enforcement})", "", f"**Claim.** {r.claim}", ""]
            if r.why.strip():
                lines += [f"**Why.** {r.why}", ""]
            if r.enforced_by:
                lines += [
                    "**Enforced by:** " + ", ".join(f"`{e}`" for e in r.enforced_by),
                    "",
                ]
    return "\n".join(lines).rstrip() + "\n"


def build_methodology_atoms_operator(g: TensionGraph) -> str:
    """Atomic: operator-altitude atoms (R-operator-*, R-agent-*, R-boot-*, R-prefer-tool-*)."""
    sel = _select_settled(
        g,
        lambda r: r.id.startswith(
            ("R-operator-", "R-agent-", "R-boot-", "R-prefer-tool-")
        ),
    )
    return _render_atoms(
        "Operator atoms",
        "The atomic requirements that constitute the operator's role, identity, and discipline.",
        sel,
    )


def build_methodology_atoms_substrate(g: TensionGraph) -> str:
    """Atomic: substrate atoms (R-claude-md-*, R-content-*, R-deterministic-*, R-drift-*, R-rejected-*)."""
    sel = _select_settled(
        g,
        lambda r: r.id.startswith(
            (
                "R-claude-md-",
                "R-content-",
                "R-deterministic-",
                "R-drift-",
                "R-rejected-",
            )
        ),
    )
    return _render_atoms(
        "Substrate atoms",
        "The atomic requirements that govern how the substrate (graph + generated docs) behaves.",
        sel,
    )


def build_methodology_atoms_discipline(g: TensionGraph) -> str:
    """Atomic: discipline atoms (R-anchor-*, R-speak-*, R-crystallize-*, R-prefer-tool-*, R-shared-tools-*)."""
    sel = _select_settled(
        g,
        lambda r: (
            r.id.startswith(("R-anchor-", "R-speak-", "R-crystallize-"))
            or r.id in {"R-prefer-tool-over-hand", "R-shared-tools-in-spec-tools"}
        ),
    )
    return _render_atoms(
        "Discipline atoms",
        "The atomic requirements that govern operator discipline — anchoring, crystallizing, tool-preference.",
        sel,
    )


def build_methodology_atoms_check(g: TensionGraph) -> str:
    """Atomic: check/enforcement atoms (R-check-*, R-requirement-*, R-bijection-*, R-enforcement-*, R-decided-*)."""
    sel = _select_settled(
        g,
        lambda r: r.id.startswith(
            (
                "R-check-",
                "R-requirement-",
                "R-bijection-",
                "R-enforcement-",
                "R-decided-",
            )
        ),
    )
    return _render_atoms(
        "Check & enforcement atoms",
        "The atomic requirements about how rules are enforced — atomicity of claims, atomicity of checks, bijection.",
        sel,
    )


# ---------------------------------------------------------------------------
# Tool-derived requirements section for REQUIREMENTS.md
# ---------------------------------------------------------------------------


def build_tool_derived_section() -> str:
    """Build the '## Tool-derived requirements' section for REQUIREMENTS.md as an LF string.

    Scans spec/tools/*.py for Canon: §<topic> — <claim> markers and projects
    each into a R-tool-<basename> entry. Sorted by basename (deterministic).
    """
    tool_reqs = _scan_tool_requirements()
    lines: list[str] = []
    lines.append("## Tool-derived requirements")
    lines.append("")
    lines.append(
        "Projected from `spec/tools/*.py` module docstrings whose first line "
        "matches `Canon: §<topic> — <claim>` (R-tool-is-its-own-requirement). "
        "The docstring IS the claim; the body IS the check; the test IS the enforcer. "
        "Deleting the tool deletes the R."
    )
    lines.append("")
    if not tool_reqs:
        lines.append("_No tools carry a Canon: §... marker yet._")
        lines.append("")
    else:
        for tr in tool_reqs:
            enforcer_str = (
                f"enforcer: `{tr.enforcer}`" if tr.enforcer else "enforcer: (none)"
            )
            lines.append(
                f"- **{tr.id}** — *{tr.claim}* "
                f"[STRUCTURAL·tool · §{tr.canon_section}] [{enforcer_str}]"
            )
        lines.append("")
    return "\n".join(lines)


# ---------------------------------------------------------------------------
# Per-agent scoped CONSTITUTION generation (R-agent-scoped-constitution)
# ---------------------------------------------------------------------------


def _render_scoped_constitution_block(
    g: TensionGraph,
    scope: tuple[str, ...],
    tool_reqs: list[ToolRequirement],
) -> str:
    """Render a CONSTITUTION digest block filtered by SCOPE prefixes.

    Includes:
    - SETTLED graph requirements whose id starts with any prefix in scope.
    - Tool-derived requirements whose id starts with any prefix in scope.

    If scope is empty or no requirements match, emits a placeholder block.
    Deterministic: sorted by category then by id within category.
    """
    # Filter graph SETTLED requirements by scope.
    if scope:
        settled = [
            r
            for r in g.requirements
            if r.status == SETTLED and any(r.id.startswith(p) for p in scope)
        ]
    else:
        settled = []

    # Filter tool-derived requirements by scope.
    if scope:
        scoped_tools = [
            tr for tr in tool_reqs if any(tr.id.startswith(p) for p in scope)
        ]
    else:
        scoped_tools = []

    if not settled and not scoped_tools:
        return (
            "<!-- (generated by tools/gen_spec.py — do not hand-edit) -->\n\n"
            "_(no atoms in scope)_"
        )

    # Group by category (same logic as _render_constitution_block).
    groups: dict[str, list] = {}
    for r in settled:
        cat = _categorize_requirement(r.id)
        groups.setdefault(cat, []).append(r)

    for cat in groups:
        groups[cat].sort(key=lambda r: r.id)

    cat_order = [label for label, _ in _DIGEST_CATEGORIES if label in groups]
    if "Other" in groups:
        cat_order.append("Other")

    lines: list[str] = [
        "<!-- (generated by tools/gen_spec.py — do not hand-edit) -->",
        "",
        "### Constitutional Digest (scoped)",
        "",
    ]
    for cat in cat_order:
        lines.append(f"**{cat}**")
        lines.append("")
        for r in groups[cat]:
            enf = f" [{r.enforcement}"
            if r.enforced_by:
                enf += f"·{'·'.join(r.enforced_by)}"
            enf += "]"
            lines.append(f"- **{r.id}** — *{r.claim}*{enf}")
        lines.append("")

    if scoped_tools:
        lines.append("**Tool-derived requirements**")
        lines.append("")
        for tr in scoped_tools:
            enforcer_str = (
                f"enforcer: `{tr.enforcer}`" if tr.enforcer else "enforcer: (none)"
            )
            lines.append(
                f"- **{tr.id}** — *{tr.claim}* "
                f"[STRUCTURAL·tool · §{tr.canon_section}] [{enforcer_str}]"
            )
        lines.append("")

    return "\n".join(lines).rstrip()


def _regenerate_agent_constitutions(
    g: TensionGraph,
    agents_root: Path | None = None,
) -> None:
    """Regenerate CONSTITUTION blocks in all spec/agents/<name>/CLAUDE.md files.

    Walks agents_root (defaults to _AGENTS_ROOT = spec/agents/). For each
    sub-directory found:
    - Loads scope.py via importlib and extracts the SCOPE tuple.
    - Filters g.requirements (SETTLED) + tool-derived requirements by scope.
    - Renders a scoped CONSTITUTION block (same category grouping as root).
    - Writes the block between the CONSTITUTION sentinels in CLAUDE.md.

    Raises RuntimeError if sentinels are absent in an agent CLAUDE.md — missing
    sentinels indicate manual corruption (the scaffold from create_agent.py always
    emits them).

    No-op if agents_root does not exist or contains no sub-directories.
    Deterministic: agents processed in sorted name order; requirements sorted by
    category then by id. LF, utf-8, no timestamps.
    """
    import importlib.util  # noqa: PLC0415

    root = agents_root or _AGENTS_ROOT
    if not root.exists():
        return

    # Pre-scan tool requirements once (shared across agents).
    tool_reqs = _scan_tool_requirements()

    for agent_dir in sorted(root.iterdir()):
        if not agent_dir.is_dir():
            continue

        scope_py = agent_dir / "scope.py"
        claude_md_path = agent_dir / "CLAUDE.md"

        if not scope_py.exists():
            # Not a valid agent directory (no scope.py); skip silently.
            continue
        if not claude_md_path.exists():
            raise RuntimeError(
                f"Agent directory '{agent_dir.name}' has scope.py but no CLAUDE.md. "
                "The scaffold from create_agent.py always creates both; "
                "manual corruption detected."
            )

        # Load scope.py and extract SCOPE tuple.
        spec = importlib.util.spec_from_file_location(
            f"_agent_scope_{agent_dir.name}", scope_py
        )
        if spec is None or spec.loader is None:
            raise RuntimeError(
                f"Cannot load scope.py for agent '{agent_dir.name}': "
                "importlib returned None spec."
            )
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)  # type: ignore[union-attr]
        scope: tuple[str, ...] = getattr(module, "SCOPE", ())

        # Render the scoped CONSTITUTION block.
        new_block = _render_scoped_constitution_block(g, scope, tool_reqs)

        # Update the agent's CLAUDE.md between CONSTITUTION sentinels.
        text = _read(claude_md_path)
        if _CONST_BEGIN not in text or _CONST_END not in text:
            raise RuntimeError(
                f"Agent CLAUDE.md at '{claude_md_path}' is missing "
                f"CONSTITUTION sentinels ('{_CONST_BEGIN}' / '{_CONST_END}'). "
                "This indicates manual corruption; the scaffold always emits them."
            )

        begin_pos = text.find(_CONST_BEGIN)
        end_pos = text.find(_CONST_END)
        before = text[: begin_pos + len(_CONST_BEGIN)]
        after = text[end_pos:]
        new_text = before + "\n" + new_block + "\n" + after
        _write(claude_md_path, new_text)
        print(f"updated agent: {claude_md_path}")


# ---------------------------------------------------------------------------
# REPO-MAP block — generated file-index inside CLAUDE.md
# ---------------------------------------------------------------------------

_CANON_ROLE_RE = re.compile(r"^Canon:\s+\S+\s+[—\-]\s+(.+)$")


def _resolve_active_content_dir() -> Path:
    """Return the active content dir: domains/<first>/ or legacy spec/content/.

    Computed once at import time for backward-compat (CONTENT_DIR used by
    _scan_repo_map() and test_repo_map.py).
    """
    if DOMAINS_ROOT.exists():
        domain_dirs = sorted(
            d
            for d in DOMAINS_ROOT.iterdir()
            if d.is_dir() and not d.name.startswith("_")
        )
        if domain_dirs:
            return domain_dirs[0]
    return SPEC_ROOT / "content"


CONTENT_DIR = _resolve_active_content_dir()


def _docstring_role(path: Path) -> str:
    """Extract a one-line role from a Python file's module docstring.

    Strips the optional 'Canon: §X — ' prefix so that only the descriptive
    part is returned.  Falls back to '(no docstring)' if none is present.
    """
    try:
        src = path.read_text(encoding="utf-8")
        tree = ast.parse(src)
        doc = ast.get_docstring(tree) or ""
    except Exception:
        return "(parse error)"
    first = doc.split("\n")[0].strip() if doc else ""
    if not first:
        return "(no docstring)"
    m = _CANON_ROLE_RE.match(first)
    return m.group(1).strip() if m else first


def _md_title(path: Path) -> str:
    """Extract the first H1 or H2 line from a Markdown file as a short title."""
    try:
        for line in path.read_text(encoding="utf-8").splitlines():
            stripped = line.lstrip("#").strip()
            if line.startswith("#") and stripped:
                # Drop trailing " (Tensio)" and similar suffixes for brevity.
                return stripped.split(" — ")[-1].split(" (")[0].strip()
    except Exception:
        pass
    return path.stem


def _scan_repo_map(
    *,
    src_dir: Path | None = None,
    tools_dir: Path | None = None,
    content_dir: Path | None = None,
    gen_dir: Path | None = None,
    graph: TensionGraph | None = None,
) -> str:
    """Scan substrate areas and return the rendered REPO-MAP block content (no sentinels).

    Sections:
      - Framework body  (spec/src/tensio/*.py  — excluding __init__.py)
      - Tools           (spec/tools/*.py        — excluding __init__.py)
      - Domain content  (spec/content/*.py      — excluding __init__.py and README.md)
      - Generated docs  (docs/gen/*.md)

    For tool entries, appends '  →  R-tool-<basename>' when that id is known
    via _scan_tool_requirements() (cross-reference).
    """
    _src = src_dir or SRC
    _tools = tools_dir or SPEC_TOOLS_DIR
    _content = content_dir or CONTENT_DIR
    _gen = gen_dir or GEN_DIR

    # Determine display labels for content and gen dirs (relative to repo root).
    _content_rel = str(_content.relative_to(REPO_ROOT)).replace("\\", "/")
    _gen_rel = str(_gen.relative_to(REPO_ROOT)).replace("\\", "/")

    # Pre-collect known tool requirement ids for cross-reference.
    tool_req_ids: set[str] = {
        tr.id for tr in _scan_tool_requirements(spec_tools_dir=_tools)
    }

    lines: list[str] = [
        "<!-- (generated by tools/gen_spec.py — do not hand-edit) -->",
        "",
        "### Repository Map",
        "",
    ]

    # --- Framework body -------------------------------------------------------
    lines.append("**Framework body** (`spec/src/tensio/`)")
    lines.append("")
    for p in sorted(_src.glob("*.py")):
        if p.name.startswith("_"):
            continue
        role = _docstring_role(p)
        lines.append(f"- `spec/src/tensio/{p.name}` — {role}")
    lines.append("")

    # --- Tools ----------------------------------------------------------------
    lines.append("**Tools** (`spec/tools/`)")
    lines.append("")
    for p in sorted(_tools.glob("*.py")):
        if p.name.startswith("_"):
            continue
        role = _docstring_role(p)
        rid = f"R-tool-{p.stem.replace('_', '-')}"
        xref = f"  →  {rid}" if rid in tool_req_ids else ""
        lines.append(f"- `spec/tools/{p.name}` — {role}{xref}")
    lines.append("")

    # --- Domain content -------------------------------------------------------
    lines.append(f"**Domain content** (`{_content_rel}/`)")
    lines.append("")
    content_files = sorted(_content.glob("*.py"))
    if content_files:
        for p in content_files:
            if p.name.startswith("_"):
                continue
            role = _docstring_role(p)
            lines.append(f"- `{_content_rel}/{p.name}` — {role}")
    else:
        lines.append("- _(no content files yet)_")
    lines.append("")

    # --- Generated docs -------------------------------------------------------
    lines.append(f"**Generated docs** (`{_gen_rel}/`)")
    lines.append("")
    gen_files = sorted(_gen.glob("*.md"))
    if gen_files:
        for p in gen_files:
            title = _md_title(p)
            lines.append(f"- `{_gen_rel}/{p.name}` — {title}")
    else:
        lines.append("- _(no generated docs yet)_")
    lines.append("")

    return "\n".join(lines).rstrip()


def _update_claude_md_repo_map(g: TensionGraph) -> None:  # noqa: ARG001
    """Rewrite the REPO-MAP sentinel block in CLAUDE.md with a fresh file index.

    Idempotent: calling twice on an unchanged filesystem produces identical CLAUDE.md.
    If sentinels are absent, inserts them after the CONSTITUTION:END block.
    Only runs in non-demo mode.
    """
    if not CLAUDE_MD.exists():
        return
    text = _read(CLAUDE_MD)
    new_block = _scan_repo_map()

    if _REPO_MAP_BEGIN in text and _REPO_MAP_END in text:
        begin_pos = text.find(_REPO_MAP_BEGIN)
        end_pos = text.find(_REPO_MAP_END)
        before = text[: begin_pos + len(_REPO_MAP_BEGIN)]
        after = text[end_pos:]
        new_text = before + "\n" + new_block + "\n" + after
    else:
        # Insert after CONSTITUTION:END block.
        const_end_pos = text.find(_CONST_END)
        if const_end_pos != -1:
            insert_at = const_end_pos + len(_CONST_END)
            new_text = (
                text[:insert_at]
                + f"\n\n{_REPO_MAP_BEGIN}\n{new_block}\n{_REPO_MAP_END}\n"
                + text[insert_at:]
            )
        else:
            # Fallback: append at end.
            new_text = (
                text.rstrip("\n")
                + f"\n\n{_REPO_MAP_BEGIN}\n{new_block}\n{_REPO_MAP_END}\n"
            )

    _write(CLAUDE_MD, new_text)


# ---------------------------------------------------------------------------
# AGENT-MAP block — generated agent-delegation index inside CLAUDE.md
# ---------------------------------------------------------------------------


def _scan_agent_map(
    g: TensionGraph,
    agents_root: Path | None = None,
    tools_dir: Path | None = None,
) -> str:
    """Walk spec/agents/<name>/ and render the AGENT-MAP block content (no sentinels).

    For each agent directory that contains scope.py:
    - Loads PURPOSE (one-line string) and SCOPE (tuple of prefixes) from scope.py.
    - Counts atoms: SETTLED requirements in g whose id starts with any SCOPE prefix.
    - Counts private tools: *.py files in agent_dir/tools/ (excluding __init__.py).
    - Counts shared tools: *.py files in spec/tools/ (excluding __init__.py and
      files starting with '_').
    - Crystal path: spec/agents/<name>/CLAUDE.md (relative to repo root).

    Agents are sorted by name (deterministic). If no agents found, emits the
    placeholder '_(no sub-operators yet)_'.
    """
    import importlib.util  # noqa: PLC0415

    root = agents_root or _AGENTS_ROOT
    _tools = tools_dir or SPEC_TOOLS_DIR

    # Count shared tools once (spec/tools/*.py excluding __init__ and _-prefixed).
    shared_tools_count = len(
        [
            p
            for p in _tools.glob("*.py")
            if not p.name.startswith("_") and p.name != "__init__.py"
        ]
    )

    lines: list[str] = [
        "<!-- (generated by tools/gen_spec.py — do not hand-edit) -->",
        "",
        "### Agent Map",
        "",
    ]

    if not root.exists():
        lines.append("_(no sub-operators yet)_")
        return "\n".join(lines).rstrip()

    agent_dirs = sorted([d for d in root.iterdir() if d.is_dir()])
    if not agent_dirs:
        lines.append("_(no sub-operators yet)_")
        return "\n".join(lines).rstrip()

    found_any = False
    for agent_dir in agent_dirs:
        scope_py = agent_dir / "scope.py"
        if not scope_py.exists():
            continue
        found_any = True

        # Load scope.py.
        spec = importlib.util.spec_from_file_location(
            f"_agent_scope_{agent_dir.name}", scope_py
        )
        module = importlib.util.module_from_spec(spec)  # type: ignore[arg-type]
        spec.loader.exec_module(module)  # type: ignore[union-attr]

        purpose: str = getattr(module, "PURPOSE", "")
        scope: tuple[str, ...] = getattr(module, "SCOPE", ())

        # Count atoms: SETTLED requirements matching any SCOPE prefix.
        if scope:
            atoms_count = sum(
                1
                for r in g.requirements
                if r.status == SETTLED and any(r.id.startswith(p) for p in scope)
            )
        else:
            atoms_count = 0

        # Count private tools.
        private_tools_dir = agent_dir / "tools"
        if private_tools_dir.exists():
            private_tools_count = len(
                [p for p in private_tools_dir.glob("*.py") if p.name != "__init__.py"]
            )
        else:
            private_tools_count = 0

        # Crystal path (relative to repo root).
        try:
            crystal_path = str(
                (agent_dir / "CLAUDE.md").relative_to(REPO_ROOT)
            ).replace("\\", "/")
        except ValueError:
            crystal_path = f"spec/agents/{agent_dir.name}/CLAUDE.md"

        # Scope display: prefixes joined by ` · `.
        scope_str = " · ".join(f"`{p}`" for p in scope) if scope else "_(none)_"

        lines.append(f"#### {agent_dir.name}")
        lines.append(f"- **purpose** — {purpose}")
        lines.append(f"- **scope** — {scope_str}")
        lines.append(f"- **atoms** — {atoms_count} SETTLED in scope")
        lines.append(
            f"- **tools** — {private_tools_count} private · {shared_tools_count} shared"
        )
        lines.append(f"- **crystal** — `{crystal_path}`")
        lines.append("")

    if not found_any:
        lines.append("_(no sub-operators yet)_")

    return "\n".join(lines).rstrip()


def _update_claude_md_agent_map(g: TensionGraph) -> None:
    """Rewrite the AGENT-MAP sentinel block in CLAUDE.md with a fresh agent index.

    Idempotent: calling twice on an unchanged filesystem produces identical CLAUDE.md.
    If sentinels are absent, inserts them after the REPO-MAP:END block.
    Only runs in non-demo mode.
    """
    if not CLAUDE_MD.exists():
        return
    text = _read(CLAUDE_MD)
    new_block = _scan_agent_map(g)

    if _AGENT_MAP_BEGIN in text and _AGENT_MAP_END in text:
        begin_pos = text.find(_AGENT_MAP_BEGIN)
        end_pos = text.find(_AGENT_MAP_END)
        before = text[: begin_pos + len(_AGENT_MAP_BEGIN)]
        after = text[end_pos:]
        new_text = before + "\n" + new_block + "\n" + after
    else:
        # Insert after REPO-MAP:END block.
        repo_end_pos = text.find(_REPO_MAP_END)
        if repo_end_pos != -1:
            insert_at = repo_end_pos + len(_REPO_MAP_END)
            new_text = (
                text[:insert_at]
                + f"\n\n{_AGENT_MAP_BEGIN}\n{new_block}\n{_AGENT_MAP_END}\n"
                + text[insert_at:]
            )
        else:
            # Fallback: append at end.
            new_text = (
                text.rstrip("\n")
                + f"\n\n{_AGENT_MAP_BEGIN}\n{new_block}\n{_AGENT_MAP_END}\n"
            )

    _write(CLAUDE_MD, new_text)


# ---------------------------------------------------------------------------
# Concern 2: _active_domain() — backward-compat helper
# ---------------------------------------------------------------------------


def _active_domain() -> Path | None:
    """Return the first domains/<name>/ dir (alphabetical) or None if domains/ is empty.

    When None, all generation falls back to spec/content/graph.py (current state).
    When non-None, the active graph lives in domains/<name>/graph.py and the
    domain's docs go into domains/<name>/docs/gen/.
    """
    if not DOMAINS_ROOT.exists():
        return None
    domain_dirs = sorted(
        d for d in DOMAINS_ROOT.iterdir() if d.is_dir() and not d.name.startswith("_")
    )
    return domain_dirs[0] if domain_dirs else None


# ---------------------------------------------------------------------------
# Concern 5a: Shared thinking docs (spec/docs/thinking/<topic-slug>.md)
# ---------------------------------------------------------------------------

# Regex to extract §Topic from Canon: §<Topic> — <claim> markers.
_CANON_TOPIC_RE = re.compile(r"Canon:\s+§(\S+)")


def _slug(topic: str) -> str:
    """Convert a §Topic label to a file slug (lowercase, hyphenated)."""
    return re.sub(r"[^a-z0-9]+", "-", topic.lower()).strip("-")


def _collect_canon_docstrings(src_dir: Path) -> dict[str, list[tuple[str, str, str]]]:
    """Scan src_dir/*.py via ast and collect (file_path, node_label, docstring) per Canon: §Topic.

    Returns a dict mapping topic (raw, e.g. 'Invariants') to a list of
    (relative_path, node_label, docstring) tuples, ordered by file then line number.
    """
    result: dict[str, list[tuple[str, str, str]]] = {}

    for py_file in sorted(src_dir.glob("*.py")):
        if py_file.name.startswith("_"):
            continue
        try:
            src = py_file.read_text(encoding="utf-8")
            tree = ast.parse(src)
        except Exception:
            continue

        rel = f"spec/src/tensio/{py_file.name}"

        for node in ast.walk(tree):
            doc = None
            label = None
            if isinstance(node, ast.Module):
                doc = ast.get_docstring(node)
                label = f"{rel} (module)"
            elif isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)):
                doc = ast.get_docstring(node)
                label = f"{rel}::{node.name}"
            elif isinstance(node, ast.ClassDef):
                doc = ast.get_docstring(node)
                label = f"{rel}::{node.name}"
            if not doc or not label:
                continue
            for m in _CANON_TOPIC_RE.finditer(doc):
                topic = m.group(1).rstrip(".,;:")
                result.setdefault(topic, [])
                # Avoid duplicates within the same node (multiple Canon: markers).
                entry = (rel, label, doc)
                if entry not in result[topic]:
                    result[topic].append(entry)

    return result


def build_shared_thinking_docs(src_dir: Path | None = None) -> dict[str, str]:
    """Build content for spec/docs/thinking/<topic-slug>.md files.

    Returns dict mapping topic_slug -> markdown_content.
    """
    _src = src_dir or SRC
    by_topic = _collect_canon_docstrings(_src)
    result: dict[str, str] = {}

    for topic in sorted(by_topic):
        slug = _slug(topic)
        entries = by_topic[topic]
        lines: list[str] = [
            f"<!-- Auto-generated by spec/tools/gen_spec.py from Canon: §{topic} docstrings. Do not hand-edit. -->",
            "",
            f"# Canon — §{topic}",
            "",
            f"> Auto-generated by `spec/tools/gen_spec.py` from docstrings carrying `Canon: §{topic}`. Do not hand-edit.",
            "",
        ]
        for rel, node_label, doc in entries:
            if node_label.endswith("(module)"):
                lines.append(f"## From `{rel}` (module)")
            else:
                # "spec/src/tensio/foo.py::bar" -> show as "spec/src/tensio/foo.py::bar"
                lines.append(f"## From `{node_label}`")
            lines.append("")
            lines.append(doc.rstrip())
            lines.append("")
        result[slug] = "\n".join(lines).rstrip() + "\n"

    return result


def _write_shared_thinking_docs(
    src_dir: Path | None = None, out_dir: Path | None = None
) -> list[Path]:
    """Write spec/docs/thinking/<topic-slug>.md files. Returns list of written paths."""
    _out = out_dir or SPEC_DOCS_THINKING_DIR
    _out.mkdir(parents=True, exist_ok=True)
    docs = build_shared_thinking_docs(src_dir)
    written: list[Path] = []
    for slug, content in sorted(docs.items()):
        path = _out / f"{slug}.md"
        _write(path, content)
        written.append(path)
    return written


# ---------------------------------------------------------------------------
# Concern 5b: Shared tool docs (spec/docs/tools/<basename>.md)
# ---------------------------------------------------------------------------


def build_shared_tool_docs(
    tools_dir: Path | None = None,
) -> dict[str, str]:
    """Build content for spec/docs/tools/<basename>.md files.

    Only processes tools whose module docstring opens with Canon: §<topic> — <claim>.
    Returns dict mapping basename -> markdown_content.
    """
    _tools = tools_dir or SPEC_TOOLS_DIR
    result: dict[str, str] = {}

    for path in sorted(_tools.glob("*.py")):
        if path.name.startswith("_"):
            continue
        try:
            src = path.read_text(encoding="utf-8")
            tree = ast.parse(src)
            doc = ast.get_docstring(tree) or ""
        except Exception:
            continue
        first_line = doc.split("\n")[0].strip() if doc else ""
        m = _CANON_RE.match(first_line)
        if not m:
            continue
        basename = path.stem
        canon_line = first_line

        # Try to get --help output.
        cli_section = ""
        try:
            result_proc = subprocess.run(
                [sys.executable, str(path), "--help"],
                capture_output=True,
                text=True,
                timeout=10,
                cwd=str(_tools.parent),
            )
            help_out = result_proc.stdout or result_proc.stderr
            if help_out.strip():
                cli_section = "\n## CLI usage\n\n```\n" + help_out.rstrip() + "\n```\n"
        except Exception:
            pass  # Gracefully skip if tool has no argparse or errors

        lines: list[str] = [
            "<!-- Auto-generated by spec/tools/gen_spec.py. Do not hand-edit. -->",
            "",
            f"# Tool — {basename}",
            "",
            "> Auto-generated by `spec/tools/gen_spec.py`. Do not hand-edit.",
            "",
            "## Synopsis",
            "",
            canon_line,
            "",
            "## Module docstring",
            "",
            doc.rstrip(),
            "",
        ]
        if cli_section:
            lines.append(cli_section.lstrip("\n"))

        result[basename] = "\n".join(lines).rstrip() + "\n"

    return result


def _write_shared_tool_docs(
    tools_dir: Path | None = None,
    out_dir: Path | None = None,
) -> list[Path]:
    """Write spec/docs/tools/<basename>.md files. Returns list of written paths."""
    _out = out_dir or SPEC_DOCS_TOOLS_DIR
    _out.mkdir(parents=True, exist_ok=True)
    docs = build_shared_tool_docs(tools_dir)
    written: list[Path] = []
    for basename, content in sorted(docs.items()):
        path = _out / f"{basename}.md"
        _write(path, content)
        written.append(path)
    return written


# ---------------------------------------------------------------------------
# Concern 5c: SHARED-DOCS block in agent CLAUDE.md
# ---------------------------------------------------------------------------


def _rel_path_from_agent(agent_dir: Path, target: Path) -> str:
    """Compute a relative path string from agent_dir to target, using forward slashes."""
    return os.path.relpath(target, agent_dir).replace("\\", "/")


def _render_shared_docs_block(
    agent_dir: Path,
    scope: tuple[str, ...],
    thinking_dir: Path | None = None,
    tools_dir: Path | None = None,
    spec_tools_dir: Path | None = None,
) -> str:
    """Render the SHARED-DOCS block content (without sentinels) for an agent.

    - Thinking docs: ALL (every agent reads the methodology).
    - Tools: filtered by scope (include if 'R-tool-<basename>' matches any scope prefix).
      If scope is empty, include all tools.
    """
    _thinking = thinking_dir or SPEC_DOCS_THINKING_DIR
    _tools_docs = tools_dir or SPEC_DOCS_TOOLS_DIR
    _spec_tools = spec_tools_dir or SPEC_TOOLS_DIR

    lines: list[str] = [
        "<!-- (generated by tools/gen_spec.py — do not hand-edit) -->",
        "",
        "### Shared docs (DRY)",
        "",
    ]

    # Thinking docs — all.
    thinking_files = sorted(_thinking.glob("*.md")) if _thinking.exists() else []
    if thinking_files:
        lines.append("**Thinking** — how to operate")
        lines.append("")
        for p in thinking_files:
            # Derive §Topic from filename (reverse of _slug — capitalize first letter).
            topic_slug = p.stem
            topic_label = "§" + topic_slug.replace("-", " ").title().replace(" ", "")
            rel = _rel_path_from_agent(agent_dir, p)
            lines.append(f"- [{topic_label}]({rel})")
        lines.append("")

    # Tool docs — filtered by scope.
    tool_doc_files = sorted(_tools_docs.glob("*.md")) if _tools_docs.exists() else []
    if tool_doc_files:
        if scope:
            # Include tool if R-tool-<basename> starts with any scope prefix.
            filtered = [
                p
                for p in tool_doc_files
                if any(
                    f"R-tool-{p.stem.replace('_', '-')}".startswith(pref)
                    for pref in scope
                )
            ]
        else:
            filtered = tool_doc_files
        if filtered:
            lines.append("**Tools** — in your scope")
            lines.append("")
            for p in filtered:
                rel = _rel_path_from_agent(agent_dir, p)
                lines.append(f"- [{p.stem}]({rel})")
            lines.append("")

    return "\n".join(lines).rstrip()


def _update_agent_shared_docs_block(
    agent_dir: Path,
    scope: tuple[str, ...],
    thinking_dir: Path | None = None,
    tools_dir: Path | None = None,
    spec_tools_dir: Path | None = None,
) -> None:
    """Rewrite the SHARED-DOCS sentinel block in an agent's CLAUDE.md."""
    claude_md_path = agent_dir / "CLAUDE.md"
    if not claude_md_path.exists():
        return
    text = _read(claude_md_path)
    new_block = _render_shared_docs_block(
        agent_dir, scope, thinking_dir, tools_dir, spec_tools_dir
    )

    if _SHARED_DOCS_BEGIN in text and _SHARED_DOCS_END in text:
        begin_pos = text.find(_SHARED_DOCS_BEGIN)
        end_pos = text.find(_SHARED_DOCS_END)
        before = text[: begin_pos + len(_SHARED_DOCS_BEGIN)]
        after = text[end_pos:]
        new_text = before + "\n" + new_block + "\n" + after
    else:
        # Insert after AGENT-MAP:END if present, else after CONSTITUTION:END, else append.
        for sentinel in (_AGENT_MAP_END, _CONST_END):
            pos = text.find(sentinel)
            if pos != -1:
                insert_at = pos + len(sentinel)
                new_text = (
                    text[:insert_at]
                    + f"\n\n{_SHARED_DOCS_BEGIN}\n{new_block}\n{_SHARED_DOCS_END}\n"
                    + text[insert_at:]
                )
                break
        else:
            new_text = (
                text.rstrip("\n")
                + f"\n\n{_SHARED_DOCS_BEGIN}\n{new_block}\n{_SHARED_DOCS_END}\n"
            )
    _write(claude_md_path, new_text)


def _regenerate_agent_shared_docs(
    g: TensionGraph,  # noqa: ARG001
    agents_root: Path | None = None,
    thinking_dir: Path | None = None,
    tools_dir: Path | None = None,
    spec_tools_dir: Path | None = None,
) -> None:
    """Walk agents_root and update SHARED-DOCS block in each agent CLAUDE.md."""
    root = agents_root or _AGENTS_ROOT
    if not root.exists():
        return

    for agent_dir in sorted(root.iterdir()):
        if not agent_dir.is_dir():
            continue
        scope_py = agent_dir / "scope.py"
        if not scope_py.exists():
            continue

        spec = importlib.util.spec_from_file_location(
            f"_agent_scope_sd_{agent_dir.name}", scope_py
        )
        if spec is None or spec.loader is None:
            continue
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)  # type: ignore[union-attr]
        scope: tuple[str, ...] = getattr(module, "SCOPE", ())

        _update_agent_shared_docs_block(
            agent_dir, scope, thinking_dir, tools_dir, spec_tools_dir
        )
        print(f"updated agent shared-docs: {agent_dir / 'CLAUDE.md'}")


# ---------------------------------------------------------------------------
# Concern 5d: CONCEPT-MAP block in domain CLAUDE.md
# ---------------------------------------------------------------------------


def _scan_concept_map(
    src_dir: Path | None = None,
    tests_dir: Path | None = None,
) -> str:
    """Render the CONCEPT-MAP block content (without sentinels).

    For each §-section term in glossary.TERMS (kind == 'SECTION'), builds a
    three-line entry mapping: defined (which tensio/*.py file has the module-level
    Canon: §Topic), enforced (check_* functions in invariants.py whose docstring
    mentions §Topic), and tested (test files that reference §Topic or those checks).

    Deterministic: §-topics sorted alphabetically by slug.
    """
    _src = src_dir or SRC
    _tests = tests_dir or SPEC_TESTS_DIR

    # --- Build: topic -> defining file (module-level Canon: §Topic) ---
    _FIRST_SECTION_RE = re.compile(r"^Canon:\s+§(\w+)")
    topic_to_file: dict[str, str] = {}
    for py_file in sorted(_src.glob("*.py")):
        if py_file.name.startswith("_"):
            continue
        try:
            src_text = py_file.read_text(encoding="utf-8")
            tree = ast.parse(src_text)
            doc = ast.get_docstring(tree) or ""
        except Exception:
            continue
        first_line = doc.split("\n")[0].strip() if doc else ""
        m = _FIRST_SECTION_RE.match(first_line)
        if m:
            topic = m.group(1)
            topic_to_file[topic] = f"spec/src/tensio/{py_file.name}"

    # --- Build: topic -> list of check_* function names ---
    _CANON_SECTION_RE = re.compile(r"§(\w+)")
    topic_to_checks: dict[str, list[str]] = {}
    inv_py = _src / "invariants.py"
    if inv_py.exists():
        try:
            inv_src = inv_py.read_text(encoding="utf-8")
            inv_tree = ast.parse(inv_src)
        except Exception:
            inv_tree = None
        if inv_tree:
            for node in ast.walk(inv_tree):
                if isinstance(node, ast.FunctionDef) and node.name.startswith("check_"):
                    doc = ast.get_docstring(node) or ""
                    for m in _CANON_SECTION_RE.finditer(doc):
                        topic = m.group(1)
                        topic_to_checks.setdefault(topic, [])
                        if node.name not in topic_to_checks[topic]:
                            topic_to_checks[topic].append(node.name)

    # --- Build: topic and check_* -> test files that reference them ---
    check_to_tests: dict[str, list[str]] = {}
    topic_to_direct_tests: dict[str, list[str]] = {}

    if _tests.exists():
        for test_file in sorted(_tests.glob("test_*.py")):
            try:
                test_src = test_file.read_text(encoding="utf-8")
            except Exception:
                continue
            rel = f"spec/tests/{test_file.name}"

            # Collect §Topic references.
            for m in _CANON_SECTION_RE.finditer(test_src):
                topic = m.group(1)
                topic_to_direct_tests.setdefault(topic, [])
                if rel not in topic_to_direct_tests[topic]:
                    topic_to_direct_tests[topic].append(rel)

            # Collect check_* references.
            for check_name in re.findall(r"\bcheck_\w+", test_src):
                check_to_tests.setdefault(check_name, [])
                if rel not in check_to_tests[check_name]:
                    check_to_tests[check_name].append(rel)

    # --- Collect §-section slugs from glossary, sorted alphabetically ---
    section_slugs: list[str] = sorted(t.slug for t in TERMS if t.kind == "SECTION")

    lines: list[str] = [
        "<!-- (generated by tools/gen_spec.py — do not hand-edit) -->",
        "",
        "### Concept Map (§-anchors → defined / enforced / tested)",
        "",
    ]

    for slug in section_slugs:
        topic = slug.lstrip("§")
        lines.append(f"- **{slug}**")

        # defined
        def_file = topic_to_file.get(topic, "_(not yet mapped)_")
        lines.append(f"  - defined: `{def_file}`")

        # enforced
        checks = topic_to_checks.get(topic, [])
        if checks:
            lines.append(f"  - enforced: {', '.join(f'`{c}`' for c in sorted(checks))}")
        else:
            lines.append("  - enforced: _(none)_")

        # tested: union of direct §Topic refs + refs from check_* names
        tested: list[str] = []
        seen_tests: set[str] = set()
        for tf in topic_to_direct_tests.get(topic, []):
            if tf not in seen_tests:
                seen_tests.add(tf)
                tested.append(tf)
        for check_name in sorted(checks):
            for tf in check_to_tests.get(check_name, []):
                if tf not in seen_tests:
                    seen_tests.add(tf)
                    tested.append(tf)
        if tested:
            lines.append(f"  - tested: {', '.join(f'`{tf}`' for tf in sorted(tested))}")
        else:
            lines.append("  - tested: _(none)_")

    return "\n".join(lines).rstrip()


# ---------------------------------------------------------------------------
# Concern 4: DOMAIN-MAP block in root CLAUDE.md
# ---------------------------------------------------------------------------


def _render_domain_map_block(g: TensionGraph | None = None) -> str:  # noqa: ARG001
    """Render the DOMAIN-MAP block content (without sentinels).

    When domains/ is empty, emits a placeholder. When domains/ has content,
    lists each domain: ID, purpose, goals, director, path, atoms-count.
    """
    lines: list[str] = [
        "<!-- (generated by tools/gen_spec.py — do not hand-edit) -->",
        "",
        "### Domain Map",
        "",
    ]

    if not DOMAINS_ROOT.exists():
        lines.append("_(no domains yet — domains/ directory absent)_")
        return "\n".join(lines).rstrip()

    domain_dirs = sorted(
        d for d in DOMAINS_ROOT.iterdir() if d.is_dir() and not d.name.startswith("_")
    )
    if not domain_dirs:
        lines.append("_(no domains yet)_")
        return "\n".join(lines).rstrip()

    for domain_dir in domain_dirs:
        manifest_py = domain_dir / "manifest.py"
        domain_id = domain_dir.name
        description = ""
        goals_text = ""
        director = ""
        atoms_count = 0

        if manifest_py.exists():
            spec = importlib.util.spec_from_file_location(
                f"_manifest_dm_{domain_dir.name}", manifest_py
            )
            if spec and spec.loader:
                mod = importlib.util.module_from_spec(spec)
                try:
                    spec.loader.exec_module(mod)  # type: ignore[union-attr]
                    domain_id = getattr(mod, "ID", domain_dir.name)
                    description = getattr(mod, "DESCRIPTION", "")
                    goals = getattr(mod, "GOALS", ())
                    goals_text = ", ".join(goals) if goals else "—"
                    director = getattr(mod, "DIRECTOR", "")
                except Exception:
                    pass

        # Try to load graph and count atoms.
        graph_py = domain_dir / "graph.py"
        if graph_py.exists():
            try:
                gspec = importlib.util.spec_from_file_location(
                    f"_domain_graph_{domain_dir.name}", graph_py
                )
                if gspec and gspec.loader:
                    gmod = importlib.util.module_from_spec(gspec)
                    gspec.loader.exec_module(gmod)  # type: ignore[union-attr]
                    dg = gmod.build_graph()
                    from tensio.requirement import SETTLED as _SETTLED  # noqa: PLC0415

                    atoms_count = sum(
                        1 for r in dg.requirements if r.status == _SETTLED
                    )
            except Exception:
                pass

        lines.append(f"### {domain_id}")
        lines.append(f"- **purpose** — {description or '—'}")
        lines.append(f"- **goals** — {goals_text or '—'}")
        lines.append(f"- **director** — {director or '—'}")
        lines.append(f"- **path** — `domains/{domain_dir.name}/`")
        lines.append(f"- **atoms-count** — {atoms_count} SETTLED")
        lines.append("")

    return "\n".join(lines).rstrip()


def _update_claude_md_domain_map(g: TensionGraph) -> None:
    """Rewrite (or insert) the DOMAIN-MAP sentinel block in CLAUDE.md.

    Only inserts when domains/ is non-empty; when empty the block stays
    as-is (or absent).  Idempotent.
    """
    if not CLAUDE_MD.exists():
        return
    # Only write the block when domains/ has content; skip otherwise to avoid
    # changing root CLAUDE.md unnecessarily in the empty-domains state.
    active = _active_domain()
    text = _read(CLAUDE_MD)

    # If sentinels already present, always refresh.
    if _DOMAIN_MAP_BEGIN in text and _DOMAIN_MAP_END in text:
        new_block = _render_domain_map_block(g)
        begin_pos = text.find(_DOMAIN_MAP_BEGIN)
        end_pos = text.find(_DOMAIN_MAP_END)
        before = text[: begin_pos + len(_DOMAIN_MAP_BEGIN)]
        after = text[end_pos:]
        new_text = before + "\n" + new_block + "\n" + after
        _write(CLAUDE_MD, new_text)
    elif active is not None:
        # Domains exist but sentinels not yet in CLAUDE.md — insert after AGENT-MAP:END.
        new_block = _render_domain_map_block(g)
        pos = text.find(_AGENT_MAP_END)
        if pos != -1:
            insert_at = pos + len(_AGENT_MAP_END)
            new_text = (
                text[:insert_at]
                + f"\n\n{_DOMAIN_MAP_BEGIN}\n{new_block}\n{_DOMAIN_MAP_END}\n"
                + text[insert_at:]
            )
        else:
            new_text = (
                text.rstrip("\n")
                + f"\n\n{_DOMAIN_MAP_BEGIN}\n{new_block}\n{_DOMAIN_MAP_END}\n"
            )
        _write(CLAUDE_MD, new_text)
    # else: domains/ empty and sentinels not present — no-op


def _render_thinking_index_block(thinking_dir: Path | None = None) -> str:
    """Render the THINKING-INDEX block content (without sentinels).

    Produces an alphabetical list of links to spec/docs/thinking/*.md.
    Paths are relative from repo root (for use in root CLAUDE.md).
    """
    _dir = thinking_dir or SPEC_DOCS_THINKING_DIR
    lines: list[str] = [
        "<!-- (generated by tools/gen_spec.py — do not hand-edit) -->",
        "",
        "### Methodology — how to think",
        "",
    ]
    if _dir.exists():
        for md in sorted(_dir.glob("*.md")):
            slug = md.stem
            label = slug.capitalize()
            rel = f"spec/docs/thinking/{md.name}"
            lines.append(f"- [§{label}]({rel})")
    return "\n".join(lines).rstrip()


def _update_claude_md_thinking_index(thinking_dir: Path | None = None) -> None:
    """Rewrite (or insert) the THINKING-INDEX sentinel block in root CLAUDE.md.

    Inserts after DOMAIN-MAP:END if sentinels not yet present. Idempotent.
    """
    if not CLAUDE_MD.exists():
        return
    text = _read(CLAUDE_MD)
    new_block = _render_thinking_index_block(thinking_dir)

    if _THINKING_INDEX_BEGIN in text and _THINKING_INDEX_END in text:
        begin_pos = text.find(_THINKING_INDEX_BEGIN)
        end_pos = text.find(_THINKING_INDEX_END)
        before = text[: begin_pos + len(_THINKING_INDEX_BEGIN)]
        after = text[end_pos:]
        new_text = before + "\n" + new_block + "\n" + after
        _write(CLAUDE_MD, new_text)
    else:
        # Insert after DOMAIN-MAP:END if present, else append.
        pos = text.find(_DOMAIN_MAP_END)
        if pos != -1:
            insert_at = pos + len(_DOMAIN_MAP_END)
            new_text = (
                text[:insert_at]
                + f"\n\n{_THINKING_INDEX_BEGIN}\n{new_block}\n{_THINKING_INDEX_END}\n"
                + text[insert_at:]
            )
        else:
            new_text = (
                text.rstrip("\n")
                + f"\n\n{_THINKING_INDEX_BEGIN}\n{new_block}\n{_THINKING_INDEX_END}\n"
            )
        _write(CLAUDE_MD, new_text)


# ---------------------------------------------------------------------------
# Concern 1: Per-domain iteration (future-facing; no-op when domains/ empty)
# ---------------------------------------------------------------------------


def _load_domain_manifest(domain_dir: Path) -> dict:
    """Load manifest.py from a domain dir; return dict of ID/DESCRIPTION/GOALS/DIRECTOR."""
    manifest_py = domain_dir / "manifest.py"
    if not manifest_py.exists():
        return {}
    spec = importlib.util.spec_from_file_location(
        f"_manifest_iter_{domain_dir.name}", manifest_py
    )
    if spec is None or spec.loader is None:
        return {}
    mod = importlib.util.module_from_spec(spec)
    try:
        spec.loader.exec_module(mod)  # type: ignore[union-attr]
    except Exception:
        return {}
    return {
        "ID": getattr(mod, "ID", domain_dir.name),
        "DESCRIPTION": getattr(mod, "DESCRIPTION", ""),
        "GOALS": getattr(mod, "GOALS", ()),
        "DIRECTOR": getattr(mod, "DIRECTOR", ""),
    }


def _load_domain_graph(domain_dir: Path) -> "TensionGraph | None":
    """Load graph.py:build_graph() from a domain dir."""
    graph_py = domain_dir / "graph.py"
    if not graph_py.exists():
        return None
    spec = importlib.util.spec_from_file_location(
        f"_domain_graph_iter_{domain_dir.name}", graph_py
    )
    if spec is None or spec.loader is None:
        return None
    mod = importlib.util.module_from_spec(spec)
    try:
        spec.loader.exec_module(mod)  # type: ignore[union-attr]
        return mod.build_graph()
    except Exception:
        return None


def _process_domains(g: TensionGraph) -> None:
    """Walk domains/*/ and generate per-domain docs. No-op when domains/ is empty."""
    if not DOMAINS_ROOT.exists():
        return
    domain_dirs = sorted(
        d for d in DOMAINS_ROOT.iterdir() if d.is_dir() and not d.name.startswith("_")
    )
    if not domain_dirs:
        return

    for domain_dir in domain_dirs:
        manifest = _load_domain_manifest(domain_dir)
        if not manifest:
            print(f"skipping domain {domain_dir.name}: no valid manifest.py")
            continue

        dg = _load_domain_graph(domain_dir) or g  # Fallback to root graph if missing.

        gen_dir = domain_dir / "docs" / "gen"
        gen_dir.mkdir(parents=True, exist_ok=True)

        domain_targets = {
            gen_dir / "REQUIREMENTS.md": build_requirements(dg),
            gen_dir / "TENSIONS.md": build_tensions(dg),
            gen_dir / "OPEN.md": build_open(dg),
            gen_dir / "UNENFORCED.md": build_unenforced(dg),
            gen_dir / "GLOSSARY.md": build_glossary(dg),
            gen_dir / "HISTORY.md": build_history(dg),
            gen_dir / "DECISIONS.md": build_decisions(dg),
            gen_dir / "CONSTITUTION.md": build_constitution(dg),
        }
        for path, text in domain_targets.items():
            _write(path, text)
            print(f"written (domain {domain_dir.name}): {path}")


# ---------------------------------------------------------------------------
# Domain CLAUDE.md — all-5-blocks population (P17 domain-active mode)
# ---------------------------------------------------------------------------


def _strip_stale_root_sentinels() -> None:
    """Remove CONSTITUTION, AGENT-MAP, and SHARED-DOCS sentinel blocks from root CLAUDE.md.

    In domain-active mode (P17+) these blocks belong in the domain's CLAUDE.md, not root.
    If sentinels are absent this function is a no-op (idempotent).
    """
    if not CLAUDE_MD.exists():
        return
    text = _read(CLAUDE_MD)
    changed = False

    for begin_sentinel, end_sentinel in [
        (_CONST_BEGIN, _CONST_END),
        (_AGENT_MAP_BEGIN, _AGENT_MAP_END),
        (_SHARED_DOCS_BEGIN, _SHARED_DOCS_END),
    ]:
        if begin_sentinel in text and end_sentinel in text:
            begin_pos = text.find(begin_sentinel)
            end_pos = text.find(end_sentinel) + len(end_sentinel)
            # Strip up to one leading newline before begin and trailing newline after end.
            strip_start = begin_pos
            if strip_start > 0 and text[strip_start - 1] == "\n":
                strip_start -= 1
            if strip_start > 0 and text[strip_start - 1] == "\n":
                strip_start -= 1
            strip_end = end_pos
            if strip_end < len(text) and text[strip_end] == "\n":
                strip_end += 1
            text = text[:strip_start] + text[strip_end:]
            changed = True

    if changed:
        _write(CLAUDE_MD, text)


def _update_domain_claude_md_all_blocks(domain_dir: Path, g: TensionGraph) -> None:
    """Populate all 5 sentinel blocks in domains/<name>/CLAUDE.md.

    Blocks written: LIVE-STATE, CONSTITUTION, REPO-MAP, AGENT-MAP, SHARED-DOCS.
    If a sentinel pair is absent in the domain CLAUDE.md it is appended.
    Idempotent: calling twice on an unchanged graph and filesystem produces identical output.
    """
    domain_claude_md = domain_dir / "CLAUDE.md"
    if not domain_claude_md.exists():
        return

    # --- LIVE-STATE ---
    text = _read(domain_claude_md)
    live_block = build_live_state(g)
    if _LS_BEGIN in text and _LS_END in text:
        bp = text.find(_LS_BEGIN)
        ep = text.find(_LS_END)
        text = text[: bp + len(_LS_BEGIN)] + "\n" + live_block + "\n" + text[ep:]
    else:
        text = text.rstrip("\n") + f"\n\n{_LS_BEGIN}\n{live_block}\n{_LS_END}\n"
    _write(domain_claude_md, text)

    # --- CONSTITUTION ---
    text = _read(domain_claude_md)
    const_block = _render_constitution_block(g)
    if _CONST_BEGIN in text and _CONST_END in text:
        bp = text.find(_CONST_BEGIN)
        ep = text.find(_CONST_END)
        text = text[: bp + len(_CONST_BEGIN)] + "\n" + const_block + "\n" + text[ep:]
    else:
        # Insert after LIVE-STATE:END.
        ls_end_pos = text.find(_LS_END)
        if ls_end_pos != -1:
            insert_at = ls_end_pos + len(_LS_END)
            text = (
                text[:insert_at]
                + f"\n\n{_CONST_BEGIN}\n{const_block}\n{_CONST_END}\n"
                + text[insert_at:]
            )
        else:
            text = (
                text.rstrip("\n") + f"\n\n{_CONST_BEGIN}\n{const_block}\n{_CONST_END}\n"
            )
    _write(domain_claude_md, text)

    # --- REPO-MAP (scoped to domain dir) ---
    text = _read(domain_claude_md)
    repo_block = _scan_repo_map(
        content_dir=domain_dir,
        gen_dir=domain_dir / "docs" / "gen",
    )
    if _REPO_MAP_BEGIN in text and _REPO_MAP_END in text:
        bp = text.find(_REPO_MAP_BEGIN)
        ep = text.find(_REPO_MAP_END)
        text = text[: bp + len(_REPO_MAP_BEGIN)] + "\n" + repo_block + "\n" + text[ep:]
    else:
        const_end_pos = text.find(_CONST_END)
        if const_end_pos != -1:
            insert_at = const_end_pos + len(_CONST_END)
            text = (
                text[:insert_at]
                + f"\n\n{_REPO_MAP_BEGIN}\n{repo_block}\n{_REPO_MAP_END}\n"
                + text[insert_at:]
            )
        else:
            text = (
                text.rstrip("\n")
                + f"\n\n{_REPO_MAP_BEGIN}\n{repo_block}\n{_REPO_MAP_END}\n"
            )
    _write(domain_claude_md, text)

    # --- AGENT-MAP (scoped to domain director's agents) ---
    text = _read(domain_claude_md)
    agent_block = _scan_agent_map(g, agents_root=_AGENTS_ROOT)
    if _AGENT_MAP_BEGIN in text and _AGENT_MAP_END in text:
        bp = text.find(_AGENT_MAP_BEGIN)
        ep = text.find(_AGENT_MAP_END)
        text = (
            text[: bp + len(_AGENT_MAP_BEGIN)] + "\n" + agent_block + "\n" + text[ep:]
        )
    else:
        repo_end_pos = text.find(_REPO_MAP_END)
        if repo_end_pos != -1:
            insert_at = repo_end_pos + len(_REPO_MAP_END)
            text = (
                text[:insert_at]
                + f"\n\n{_AGENT_MAP_BEGIN}\n{agent_block}\n{_AGENT_MAP_END}\n"
                + text[insert_at:]
            )
        else:
            text = (
                text.rstrip("\n")
                + f"\n\n{_AGENT_MAP_BEGIN}\n{agent_block}\n{_AGENT_MAP_END}\n"
            )
    _write(domain_claude_md, text)

    # --- CONCEPT-MAP ---
    text = _read(domain_claude_md)
    concept_block = _scan_concept_map()
    if _CONCEPT_MAP_BEGIN in text and _CONCEPT_MAP_END in text:
        bp = text.find(_CONCEPT_MAP_BEGIN)
        ep = text.find(_CONCEPT_MAP_END)
        text = (
            text[: bp + len(_CONCEPT_MAP_BEGIN)]
            + "\n"
            + concept_block
            + "\n"
            + text[ep:]
        )
    else:
        agent_end_pos = text.find(_AGENT_MAP_END)
        if agent_end_pos != -1:
            insert_at = agent_end_pos + len(_AGENT_MAP_END)
            text = (
                text[:insert_at]
                + f"\n\n{_CONCEPT_MAP_BEGIN}\n{concept_block}\n{_CONCEPT_MAP_END}\n"
                + text[insert_at:]
            )
        else:
            text = (
                text.rstrip("\n")
                + f"\n\n{_CONCEPT_MAP_BEGIN}\n{concept_block}\n{_CONCEPT_MAP_END}\n"
            )
    _write(domain_claude_md, text)

    # --- SHARED-DOCS ---
    text = _read(domain_claude_md)
    shared_block = _render_shared_docs_block(domain_dir, scope=())
    if _SHARED_DOCS_BEGIN in text and _SHARED_DOCS_END in text:
        bp = text.find(_SHARED_DOCS_BEGIN)
        ep = text.find(_SHARED_DOCS_END)
        text = (
            text[: bp + len(_SHARED_DOCS_BEGIN)]
            + "\n"
            + shared_block
            + "\n"
            + text[ep:]
        )
    else:
        concept_end_pos = text.find(_CONCEPT_MAP_END)
        if concept_end_pos != -1:
            insert_at = concept_end_pos + len(_CONCEPT_MAP_END)
            text = (
                text[:insert_at]
                + f"\n\n{_SHARED_DOCS_BEGIN}\n{shared_block}\n{_SHARED_DOCS_END}\n"
                + text[insert_at:]
            )
        else:
            text = (
                text.rstrip("\n")
                + f"\n\n{_SHARED_DOCS_BEGIN}\n{shared_block}\n{_SHARED_DOCS_END}\n"
            )
    _write(domain_claude_md, text)
    print(f"updated domain CLAUDE.md: {domain_claude_md}")


# ---------------------------------------------------------------------------
# Driver
# ---------------------------------------------------------------------------


def _write(path: Path, text: str) -> None:
    """Write a file as utf-8 with LF newlines (deterministic)."""
    path.parent.mkdir(parents=True, exist_ok=True)
    with open(path, "w", encoding="utf-8", newline="\n") as fh:
        fh.write(text)


def _load_graph(*, demo: bool) -> TensionGraph:
    """Return the graph to render: demo fixture or domain content."""
    if demo:
        tests_dir = str(SPEC_ROOT / "tests")
        if tests_dir not in sys.path:
            sys.path.insert(0, tests_dir)
        from fixtures.seed import seed_graph  # noqa: PLC0415

        return seed_graph()
    return load_content_graph()


def main(argv: list[str] | None = None) -> None:
    """Regenerate the human layer.

    Default: write docs/gen/{REQUIREMENTS,TENSIONS,OPEN}.md from spec/content/.
    --demo: write docs/demo/{REQUIREMENTS,TENSIONS,OPEN}.md from the fixture seed
            (committed docs/gen/ stays untouched, so the anti-drift meta-test
            keeps measuring the content state).
    """
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument(
        "--demo",
        action="store_true",
        help="render the fixture demo graph into docs/demo/ (not docs/gen/).",
    )
    args = parser.parse_args(argv)
    g = _load_graph(demo=args.demo)
    # When a domain is active, write docs into its docs/gen/ rather than root docs/gen/.
    out_dir = DEMO_DIR if args.demo else GEN_DIR
    targets = {
        out_dir / "REQUIREMENTS.md": build_requirements(g),
        out_dir / "TENSIONS.md": build_tensions(g),
        out_dir / "OPEN.md": build_open(g),
        out_dir / "UNENFORCED.md": build_unenforced(g),
        out_dir / "GLOSSARY.md": build_glossary(g),
        out_dir / "HISTORY.md": build_history(g),
        out_dir / "DECISIONS.md": build_decisions(g),
        out_dir / "CONSTITUTION.md": build_constitution(g),
    }
    for path, text in targets.items():
        _write(path, text)
        print(f"written: {path}")

    # Atomized methodology docs (docs/methodology/atoms/) — always written, not demo-gated.
    if not args.demo:
        ATOMS_DIR.mkdir(parents=True, exist_ok=True)
        atoms_targets = {
            ATOMS_DIR / "operator.md": build_methodology_atoms_operator(g),
            ATOMS_DIR / "substrate.md": build_methodology_atoms_substrate(g),
            ATOMS_DIR / "discipline.md": build_methodology_atoms_discipline(g),
            ATOMS_DIR / "check.md": build_methodology_atoms_check(g),
        }
        for path, text in atoms_targets.items():
            _write(path, text)
            print(f"written: {path}")

    if not args.demo:
        active = _active_domain()
        # Root CLAUDE.md: always update LIVE-STATE, DOMAIN-MAP, REPO-MAP.
        # In domain-active mode: strip stale CONSTITUTION/AGENT-MAP/SHARED-DOCS sentinels
        # from root (they belong in the domain's CLAUDE.md, not root).
        _update_claude_md_live_state(g)
        if active is None:
            # Legacy single-domain mode: root gets everything.
            _update_claude_md_constitution(g)
            _update_claude_md_repo_map(g)
            _update_claude_md_agent_map(g)
        else:
            # Domain-active mode: root is FRAMEWORK-ONLY.
            _strip_stale_root_sentinels()
            _update_claude_md_repo_map(g)
            _update_domain_claude_md_all_blocks(active, g)
        _update_claude_md_domain_map(g)
        _update_claude_md_thinking_index()
        print(f"updated: {CLAUDE_MD}")
        _regenerate_agent_constitutions(g)

        # Concern 5a: shared thinking docs.
        thinking_paths = _write_shared_thinking_docs()
        for p in thinking_paths:
            print(f"written (thinking): {p}")

        # Concern 5b: shared tool docs.
        tool_doc_paths = _write_shared_tool_docs()
        for p in tool_doc_paths:
            print(f"written (tool-doc): {p}")

        # Concern 5c: SHARED-DOCS block in agent CLAUDE.md files.
        _regenerate_agent_shared_docs(g)

        # Concern 1: per-domain docs (no-op when domains/ empty).
        _process_domains(g)


if __name__ == "__main__":
    main()
