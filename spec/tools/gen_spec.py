"""Canon: §Generator — regenerates docs/gen/ from the executable model (docstrings + graph), making drift structurally impossible.

Generator: the human layer + structural anti-drift (docs-as-code, layer 9).

The single source of truth is the executable model:
  - `spec/src/hotam_spec/*.py` docstrings (the methodology narrative: RULE + Canon:§
    + WHY) — they ship with the framework and are content-free;
  - `domains/<name>/graph.py:build_graph()` (the domain's tension graph) — populated
    by the user; empty in a fresh framework.

The normative human layer is GENERATED, never hand-written; drift is structurally
impossible because the meta-test (tests/test_docs_gen.py) demands regeneration ==
committed, byte-for-byte.

Pipeline (mirrors dev-coin's gen_spec, purpose inverted from "prove no conflict"
to "render the tensions visibly"):

    hotam_spec docstrings (narrative)            --gen-->  REQUIREMENTS.md
    content graph (Requirements + ...)       --gen-->  REQUIREMENTS.md (roster)
    Conflict clusters by axis + Mermaid      --gen-->  TENSIONS.md
    OPEN requirements + unresolved conflicts --gen-->  OPEN.md

Outputs (committed under docs/gen/, banner-marked, LF):
    REQUIREMENTS.md — requirement roster + methodology narrative.
    TENSIONS.md     — tension map: conflict nodes, clusters by axis, Mermaid.
    OPEN.md         — open registry: OPEN requirements + unresolved conflicts.

Run:
  python tools/gen_spec.py            # regenerate docs/gen/ from the active domain
  python tools/gen_spec.py --demo     # regenerate docs/demo/ from the fixture

Deterministic byte-for-byte: LF newlines, utf-8, no timestamps/randomness.
Narrative docstrings are read via ast (no code execution); the graph is loaded
via the framework loader (content) or the fixture import (--demo).
"""

from __future__ import annotations

import argparse
import ast
import importlib.util
import json
import os
import re
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path

# --- Make the hotam_spec package importable (model is the source of truth) ------

SPEC_ROOT = Path(__file__).resolve().parents[1]  # .../spec
_SRC = SPEC_ROOT / "src"
if _SRC.is_dir() and str(_SRC) not in sys.path:
    sys.path.insert(0, str(_SRC))

from hotam_spec.template_loader import (  # noqa: E402
    claude_md_template_path as _claude_md_template_path,
    read_claude_md_template as _read_claude_md_template,
)
from hotam_spec.project_paths import (  # noqa: E402
    project_root_or_raise as _project_root,
)
from hotam_spec.repo_paths import (  # noqa: E402
    domains_root as _domains_root,
    repo_root as _repo_root,
)
# Consumer root: domains/CLAUDE.md/docs live in the CONSUMER's project, resolved
# via project_root() (R1-R6 chain). In self-hosting mode R3 (CWD markers)
# resolves to the same path as _repo_root(), so behavior is unchanged for the
# dev cycle; consumer mode gets their own repo root (R-project-root-not-hardcoded).
REPO_ROOT = _project_root()  # consumer project root
# In self-hosting mode (editable install), source files are at spec/src/hotam_spec/.
# In wheel-installed mode, they're at site-packages/hotam_spec/ (SPEC_ROOT itself,
# since __file__ is hotam_spec/_tools/gen_spec.py, SPEC_ROOT = parents[1] = hotam_spec/).
_SRC_CANDIDATE = SPEC_ROOT / "src" / "hotam_spec"
if _SRC_CANDIDATE.is_dir():
    SRC = _SRC_CANDIDATE
else:
    # Wheel mode: hotam_spec package root IS the source directory.
    import hotam_spec as _hs_pkg  # noqa: PLC0415

    SRC = Path(_hs_pkg.__file__).resolve().parent
DEMO_DIR = REPO_ROOT / "docs" / "demo"
CLAUDE_MD = REPO_ROOT / "CLAUDE.md"
# Template (§3.4 portability W3): lives inside the package at
# hotam_spec/_templates/claude_md.template.txt, read via importlib.resources.
# Optional consumer override: project_root()/"CLAUDE.md.template.txt".
# CLAUDE_MD_TEMPLATE is kept as a module-level Path for backward-compat with
# tests that reference gen_spec.CLAUDE_MD_TEMPLATE — it resolves to the
# EFFECTIVE template (override or packaged).
CLAUDE_MD_TEMPLATE = _claude_md_template_path()
DOMAINS_ROOT = _domains_root()


def _sorted_domain_dirs() -> list[Path]:
    """Return domains/*/ dirs sorted alphabetically (helper, no env resolution)."""
    if not DOMAINS_ROOT.exists():
        return []
    return sorted(
        d for d in DOMAINS_ROOT.iterdir() if d.is_dir() and not d.name.startswith("_")
    )


def _select_active_domain_dir(domain_dirs: list[Path]) -> Path | None:
    """Pick the active domain dir from an already-sorted list.

    Delegates the env→pin→alphabetical NAME resolution to the single
    hotam_spec.domain_resolution.resolve_active_domain() (the ONE shared
    resolver used by graph.py, gen_spec.py and apply_proposal.py alike),
    then maps the returned name back to its directory in ``domain_dirs``.
    Shared by every _resolve_active_*() helper below so GEN_DIR /
    CONTENT_DIR / the agents root and the ROLE-block scope_label never
    disagree about which domain is active.

    WHY a pin file ahead of the alphabetical fallback: with exactly one
    domain, "first alphabetically" and "the intended domain" always coincide
    and the fallback is invisible. With >= 2 domains it stops being harmless:
    a newly-created domain can sort before the long-lived one and silently
    become "active" for anyone who forgot to export the env var, clobbering
    the root CLAUDE.md and the AGENT-MAP/gen targets (observed: hotam-dev
    sorts before hotam-spec-self). The pin file makes the default an
    explicit, committed, version-controlled decision instead of an accident
    of string ordering; alphabetical remains the last-resort fallback for a
    fresh repo with no pin file yet (R-agent-never-lost still holds — there
    is always SOME deterministic answer).
    """
    if not domain_dirs:
        return None
    from hotam_spec.domain_resolution import resolve_active_domain  # noqa: PLC0415

    name = resolve_active_domain(DOMAINS_ROOT)
    if name is None:
        return None
    for d in domain_dirs:
        if d.name == name:
            return d
    return domain_dirs[0]


def _resolve_active_gen_dir() -> Path:
    """Return the active gen dir: domains/<active>/docs/gen/ or legacy docs/gen/.

    Computed once at import time for backward-compat with tests that reference
    gen_spec.REQUIREMENTS_MD etc. as module-level paths.
    """
    active = _select_active_domain_dir(_sorted_domain_dirs())
    if active is not None:
        return active / "docs" / "gen"
    return REPO_ROOT / "docs" / "gen"


def _resolve_active_agents_root() -> Path:
    """Return the active agents root for AGENT-MAP scanning.

    Priority:
      1. domains/<active>/agents/director/agents/ — nested sub-agents of the director.
      2. Legacy spec/agents/ — pre-migration location.

    WHY: after P17 migration the top-level agents live inside the domain's
    director; the legacy spec/agents/ is gone. Returns a Path that may not
    exist (callers guard with .exists()).
    """
    domain_dirs = _sorted_domain_dirs()
    active = _select_active_domain_dir(domain_dirs)
    if active is not None:
        director_agents = active / "agents" / "director" / "agents"
        if director_agents.exists():
            return director_agents
        # Fall back to scanning all domains for backward-compat with the
        # pre-active-domain-aware behavior (first domain carrying agents/).
        for domain_dir in domain_dirs:
            director_agents = domain_dir / "agents" / "director" / "agents"
            if director_agents.exists():
                return director_agents
    return SPEC_ROOT / "agents"


GEN_DIR = _resolve_active_gen_dir()
_AGENTS_ROOT = _resolve_active_agents_root()

if str(SPEC_ROOT / "src") not in sys.path:
    sys.path.insert(0, str(SPEC_ROOT / "src"))

from hotam_spec.conflict import DECIDED_PREFIX, REVISIT_PREFIX, Conflict  # noqa: E402
from hotam_spec.doc_readers import reader_line as _doc_reader_line  # noqa: E402
from hotam_spec.glossary import TERMS, Term  # noqa: E402
from hotam_spec.graph import (  # noqa: E402
    TensionGraph,
    active_domain_doc_readers,
    domain_doc_readers,
    conflicts_by_axis,
    entity_state_conflict_suspects,
    latent_connector_suspects,
    load_content_graph,
    replaces_map,
    stakeholder_ids,
)
from hotam_spec.operator import CRYSTAL_CHARS, NODE_COUNT  # noqa: E402
from hotam_spec.requirement import DRAFT, ENFORCED, SETTLED  # noqa: E402
from hotam_spec.text import short_form  # noqa: E402
from hotam_spec.claude_md import (  # noqa: E402
    extract_block as _claude_md_extract_block,
    replace_block as _claude_md_replace_block,
    insert_block_after as _claude_md_insert_block_after,
    wrap_block as _claude_md_wrap_block,
    end_sentinel as _claude_md_end_sentinel,
)

# --- CLAUDE.md live-state sentinels -----------------------------------------

_LS_BEGIN = "<!-- LIVE-STATE:BEGIN -->"
_LS_END = "<!-- LIVE-STATE:END -->"

_CONST_BEGIN = "<!-- CONSTITUTION:BEGIN -->"
_CONST_END = "<!-- CONSTITUTION:END -->"
_OVERLAP_BEGIN = "<!-- OVERLAP:BEGIN -->"
_OVERLAP_END = "<!-- OVERLAP:END -->"

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

_EMBEDDED_THINKING_BEGIN = "<!-- EMBEDDED-THINKING:BEGIN -->"
_EMBEDDED_THINKING_END = "<!-- EMBEDDED-THINKING:END -->"

_EMBEDDED_TOOLS_BEGIN = "<!-- EMBEDDED-TOOLS:BEGIN -->"
_EMBEDDED_TOOLS_END = "<!-- EMBEDDED-TOOLS:END -->"

_RECENTLY_REJECTED_BEGIN = "<!-- RECENTLY-REJECTED:BEGIN -->"
_RECENTLY_REJECTED_END = "<!-- RECENTLY-REJECTED:END -->"

# Phase 2 crystal seed blocks (R-crystal-carries-role-seed,
# R-crystal-carries-mediation-loop, R-crystal-carries-recursion-seed).
_OPERATOR_ROLE_BEGIN = "<!-- OPERATOR-ROLE:BEGIN -->"
_OPERATOR_ROLE_END = "<!-- OPERATOR-ROLE:END -->"

_MEDIATION_LOOP_BEGIN = "<!-- MEDIATION-LOOP:BEGIN -->"
_MEDIATION_LOOP_END = "<!-- MEDIATION-LOOP:END -->"

_OPERATOR_RECURSION_BEGIN = "<!-- OPERATOR-RECURSION:BEGIN -->"
_OPERATOR_RECURSION_END = "<!-- OPERATOR-RECURSION:END -->"

# Template placeholders substituted by render_claude_md_from_template().
_MIND_PLACEHOLDER = "<!-- mind -->"
_BUSINESS_PLACEHOLDER = "<!-- business -->"

# Host character ceiling: root CLAUDE.md is the resident crystal reloaded by
# reference at the start of every session; if it exceeds the host's real
# limit, the operator boots broken. 150_000 is the observed Claude Code host
# cap; 130_000 is the working warn threshold (headroom before the hard cap).
_HOST_CHAR_CAP = 150_000
_HOST_CHAR_WARN = 130_000

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
ENTITIES_MD = GEN_DIR / "ENTITIES.md"
FRAMEWORK_INVARIANTS_MD = GEN_DIR / "FRAMEWORK-INVARIANTS.md"
REPO_MAP_MD = GEN_DIR / "REPO-MAP.md"

BANNER = (
    "<!-- AUTOGENERATED from spec/src/hotam_spec + domain graph — do not edit by "
    "hand. Edits: docstrings/graph -> python tools/gen_spec.py -->"
)


#: Optional per-domain DOC_READERS override consulted by _reader_header_line.
#: Set (via _doc_readers_override(...)) by _process_domains while rendering ONE
#: specific domain's docs/gen/, so the `reader:` header resolves from the domain
#: actually being rendered rather than from the env-active domain
#: (R-root-crystal-follows-pin). None = fall back to the env-active binding.
_DOC_READERS_OVERRIDE: dict[str, str] | None = None


class _doc_readers_override:  # noqa: N801 — context-manager, lowercase by convention here
    """Temporarily pin the DOC_READERS binding _reader_header_line resolves against."""

    def __init__(self, bindings: dict[str, str]) -> None:
        self._bindings = bindings
        self._prev: dict[str, str] | None = None

    def __enter__(self) -> "None":
        global _DOC_READERS_OVERRIDE
        self._prev = _DOC_READERS_OVERRIDE
        _DOC_READERS_OVERRIDE = self._bindings
        return None

    def __exit__(self, *exc: object) -> None:
        global _DOC_READERS_OVERRIDE
        _DOC_READERS_OVERRIDE = self._prev


def _reader_header_line(doc_kind: str, g: TensionGraph) -> str:
    """Canon: §Requirement — the `reader: <id>` header line for one generated doc.

    RULE: every generated doc names its reader (R-doc-names-reader). Resolves
    `doc_kind` against `doc_readers.DOC_READER_ROLES` + the active graph's
    stakeholders via `doc_readers.reader_line()`. On an empty graph (no
    stakeholders declared yet) this still emits a line — with the honest
    `(unresolved-reader)` sentinel — so the header shape never depends on
    graph population state (R-empty-content-wellformed).

    When _DOC_READERS_OVERRIDE is set (by _process_domains rendering a specific
    domain), that per-domain binding is used instead of the env-active one, so
    a domain's docs never carry another domain's reader (R-root-crystal-follows-pin).
    """
    readers = (
        _DOC_READERS_OVERRIDE
        if _DOC_READERS_OVERRIDE is not None
        else active_domain_doc_readers()
    )
    return _doc_reader_line(doc_kind, stakeholder_ids(g), readers)

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


# --- Intra-process source caches (deterministic within one gen_spec run) -----
# The same ~55 source files (tools/*.py, src/hotam_spec/*.py, thinking/*.md) were
# read and ast.parse'd dozens of times per run. Files do not change during a
# single process → memoize read+parse by absolute path. Cache is module-level so
# a fresh process (a new env pin) starts clean; nothing is persisted to disk.
_SOURCE_TEXT_CACHE: dict[str, str] = {}
_SOURCE_AST_CACHE: dict[str, ast.Module] = {}


def read_source(path: Path) -> str:
    """Return utf-8 text of `path`, memoized by absolute path (raw, no newline norm).

    Mirrors the historical `path.read_text(encoding="utf-8")` call sites so the
    cached bytes are byte-identical to the un-cached ones.
    """
    key = str(path.resolve())
    cached = _SOURCE_TEXT_CACHE.get(key)
    if cached is None:
        cached = path.read_text(encoding="utf-8")
        _SOURCE_TEXT_CACHE[key] = cached
    return cached


def parse_source(path: Path) -> ast.Module:
    """Return the ast.Module for `path`, memoized by absolute path.

    Parsed over read_source(path) so text and AST share one cache key. The
    returned tree is shared — consumers only read it (ast.walk / get_docstring),
    never mutate it.
    """
    key = str(path.resolve())
    tree = _SOURCE_AST_CACHE.get(key)
    if tree is None:
        tree = ast.parse(read_source(path))
        _SOURCE_AST_CACHE[key] = tree
    return tree


def _module_docstring(mod: str) -> str:
    """Top-level docstring of src/hotam_spec/<mod>.py via ast (no code execution)."""
    tree = parse_source(SRC / f"{mod}.py")
    return ast.get_docstring(tree) or ""


def _cell(text: str) -> str:
    """Escape text for a markdown table cell (LF -> space, | -> \\|)."""
    return text.replace("\n", " ").replace("|", "\\|")


def _mermaid_id(node_id: str) -> str:
    """Sanitize an object id into a Mermaid-safe node identifier."""
    return node_id.replace("-", "_").replace("~", "_").replace(".", "_")


_EMPTY_NOTICE = (
    "_No domain content loaded — no `domains/<name>/graph.py` found. "
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
            tree = parse_source(path)
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
    lines: list[str] = [BANNER, _reader_header_line("REQUIREMENTS", g), ""]
    lines.append("# REQUIREMENTS.md — Requirement roster & methodology (Hotam-Spec)")
    lines.append("")
    lines.append(
        "Generated from the executable model: the methodology narrative comes from "
        "`spec/src/hotam_spec` docstrings (RULE + `Canon:§` + WHY); the roster below "
        "comes from `domains/<name>/graph.py:build_graph()`. Source of truth is the "
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
        lines.append(f"### {ordinal}. {label} — `hotam_spec.{mod}`")
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
    if c.variants:
        lines.append("- **variants** (steward chooses one):")
        for v in c.variants:
            lines.append(f"  - `{v.id}`")
            lines.append(f"    - behavior: {v.behavior}")
            lines.append(f"    - implies: {v.implies}")
            lines.append(f"    - costs: {v.costs}")
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
    lines: list[str] = [BANNER, _reader_header_line("TENSIONS", g), ""]
    lines.append("# TENSIONS.md — The tension map (Hotam-Spec)")
    lines.append("")
    lines.append(
        "Generated from the active domain's `graph.py` (the tension graph). A "
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
    lines.append("## Hotam-Specn map (Mermaid)")
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

    lines: list[str] = [BANNER, _reader_header_line("OPEN", g), ""]
    lines.append("# OPEN.md — Open registry (Hotam-Spec)")
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
    closeable_debt = [r for r in settled_unenforced if r.is_closeable_debt()]
    inherent_prose = [r for r in settled_unenforced if not r.is_closeable_debt()]

    lines: list[str] = [BANNER, _reader_header_line("UNENFORCED", g), ""]
    lines.append("# UNENFORCED.md — Burn-down meter (Hotam-Spec)")
    lines.append("")
    lines.append(
        "Generated mirror of the enforcement gradient. Every requirement carries\n"
        "`enforcement: PROSE | STRUCTURAL | ENFORCED` (R-enforcement-gradient) and an\n"
        "`enforceability: ENFORCEABLE | INHERENTLY_PROSE` kind (R-enforceability-kind-declared).\n"
        "This report lists every SETTLED requirement whose enforcement is NOT yet ENFORCED,\n"
        "split into real closeable debt vs permanent discipline."
    )
    lines.append("")
    lines.append(
        "The ratio line below IS the burn-down meter: a healthy direction is SETTLED-ENFORCED\n"
        "growing while closeable debt (ENFORCEABLE, PROSE/STRUCTURAL of SETTLED) shrinks.\n"
        "INHERENTLY_PROSE requirements are NOT counted as debt — they are honestly-labeled\n"
        "judgment calls no check_* could ever verify."
    )
    lines.append("")

    if g.is_empty():
        lines.append(_EMPTY_NOTICE)
        lines.append("")
        return "\n".join(lines).rstrip() + "\n"

    lines.append(
        f"**Burn-down: SETTLED-ENFORCED {len(settled_enforced)} / SETTLED {len(settled)}; "
        f"closeable debt {len(closeable_debt)}; inherent discipline {len(inherent_prose)}; "
        f"DRAFT {len(draft)}; OPEN {len(open_reqs)}; REJECTED {len(rejected)}.**"
    )
    lines.append("")
    lines.append("---")
    lines.append("")

    lines.append("## Closeable debt (ENFORCEABLE, no enforcer yet)")
    lines.append("")
    if not closeable_debt:
        lines.append("_None — all ENFORCEABLE SETTLED requirements are ENFORCED._")
        lines.append("")
    else:
        lines.append("| id | enforcement | owner | claim |")
        lines.append("|---|---|---|---|")
        for r in closeable_debt:
            lines.append(
                f"| `{r.id}` | {r.enforcement} | `{r.owner}` | {_cell(r.claim)} |"
            )
        lines.append("")

    lines.append(
        "## Inherent discipline (INHERENTLY_PROSE — not debt, permanent by design)"
    )
    lines.append("")
    if not inherent_prose:
        lines.append("_None yet tagged._")
        lines.append("")
    else:
        lines.append("| id | enforcement | owner | claim |")
        lines.append("|---|---|---|---|")
        for r in inherent_prose:
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
# ENTITIES.md — the entity registry (entity types, lifecycles, fields, instances)
# ---------------------------------------------------------------------------

# The 7 check_entity_* + check_typed_anchors_entity that cover every EntityType.
_ENTITY_COVERING_CHECKS: tuple[str, ...] = (
    "check_entity_type_lifecycle_wellformed",
    "check_entity_instance_state_in_lifecycle",
    "check_entity_instance_required_fields",
    "check_entity_instance_id_prefix",
    "check_entity_instance_refs_resolve",
    "check_entity_field_kind_known",
    "check_typed_anchors_entity",
)


def _render_entity_type_mermaid(et) -> list[str]:
    """Render a stateDiagram-v2 Mermaid block for one EntityType's lifecycle.

    Deterministic: states in declaration order, transitions in declaration order.
    """
    lc = et.lifecycle
    lines = ["```mermaid", "stateDiagram-v2"]
    # Initial state arrow: [*] --> <initial_state>
    for s in lc.states:
        if s.is_initial():
            lines.append(f"    [*] --> {s.name}")
            break
    # Transitions.
    for t in lc.transitions:
        lines.append(f"    {t.src} --> {t.dst} : {t.event}")
    # State labels for non-initial states (kind annotation).
    for s in lc.states:
        if not s.is_initial():
            lines.append(f"    {s.name}: {s.name} ({s.kind})")
        else:
            lines.append(f"    {s.name}: {s.name} ({s.kind})")
    lines.append("```")
    return lines


def _render_entity_lifecycle_summary(et) -> list[str]:
    """Render the bullet-list lifecycle summary for one EntityType."""
    lc = et.lifecycle
    state_parts = []
    for s in lc.states:
        state_parts.append(f"`{s.name}` ({s.kind})")
    trans_parts = [f"`{t.event}`" for t in lc.transitions]
    cyclic_str = "true" if lc.cyclic else "false"
    return [
        f"- States: {', '.join(state_parts)}",
        f"- Transitions: {', '.join(trans_parts) if trans_parts else '_(none)_'}",
        f"- Cyclic: {cyclic_str}",
    ]


def _render_entity_fields_table(et) -> list[str]:
    """Render the fields table for one EntityType."""
    if not et.fields:
        return ["_(no fields declared)_"]
    lines = [
        "| name | kind | required | ref_target |",
        "|------|------|----------|------------|",
    ]
    for f in et.fields:
        ref = f.ref_target if f.ref_target else ""
        req_str = "true" if f.required else "false"
        lines.append(f"| {f.name} | {f.kind} | {req_str} | {ref} |")
    return lines


def _render_entity_instances_table(g: TensionGraph, slug: str) -> list[str]:
    """Render the instances table for one EntityType slug."""
    instances = [e for e in g.entities if e.entity_type == slug]
    if not instances:
        return ["_(no instances declared)_"]
    # Find all field names for this type.
    et_map = {et.slug: et for et in g.entity_types}
    et = et_map.get(slug)
    field_names = [f.name for f in et.fields] if et else []

    header_parts = ["id", "state"] + field_names
    sep_parts = ["-" * max(len(h), 3) for h in header_parts]
    lines = [
        "| " + " | ".join(header_parts) + " |",
        "| " + " | ".join(sep_parts) + " |",
    ]
    for inst in sorted(instances, key=lambda e: e.id):
        row = [inst.id, inst.state]
        for fn in field_names:
            row.append(inst.field_value(fn) or "")
        lines.append("| " + " | ".join(row) + " |")
    return lines


def _entities_md_has_content(g: TensionGraph) -> bool:
    """Canon: §Entity — True iff ENTITIES.md would render real content (task #106 / L2-#6).

    ENTITIES.md is written ONLY when the domain declares at least one
    EntityType — an opt-in aspect (§Entity). A domain with zero entity_types
    would otherwise get a permanent 368-byte placeholder file that never
    carries any information (build_entities_md's own empty-state branch).
    Callers (_process_domains / main) skip the write entirely when this is
    False, rather than materializing an always-empty projection.
    """
    return bool(g.entity_types)


def build_entities_md(g: TensionGraph, domain_name: str = "") -> str:
    """Build ENTITIES.md (entity registry) as an LF string.

    Canon: §Entity — generated registry of every EntityType declared in the active
    domain's graph: lifecycle Mermaid diagram, fields table, covering invariants,
    and EntityInstance roster. When entity_types is empty (the aspect is opt-in),
    emits the empty-state placeholder. Deterministic: LF, utf-8, sorted by slug.
    """
    # Build header line.
    if domain_name:
        source_hint = f"from `domains/{domain_name}/graph.py:build_graph()`"
    else:
        source_hint = "from the active domain's `graph.py:build_graph()`"

    lines: list[str] = [BANNER, _reader_header_line("ENTITIES", g), ""]
    lines.append("# Entities")
    lines.append("")
    lines.append(
        f"> Generated by `spec/tools/gen_spec.py` {source_hint}. Do not hand-edit."
    )
    lines.append("")

    if not g.entity_types:
        lines.append(
            "_(no entity types declared in this domain — the §Entity aspect is opt-in.)_"
        )
        lines.append("")
        return "\n".join(lines).rstrip() + "\n"

    # Per-type sections, sorted by slug.
    for et in sorted(g.entity_types, key=lambda e: e.slug):
        lines.append(f"## {et.slug}")
        lines.append("")
        if et.description:
            lines.append(et.description)
            lines.append("")

        lines.append("### Lifecycle")
        lines.append("")
        lines.extend(_render_entity_type_mermaid(et))
        lines.append("")
        lines.extend(_render_entity_lifecycle_summary(et))
        lines.append("")

        lines.append("### Fields")
        lines.append("")
        lines.extend(_render_entity_fields_table(et))
        lines.append("")

        lines.append("### Covered by")
        lines.append("")
        for check_name in _ENTITY_COVERING_CHECKS:
            lines.append(f"- `{check_name}`")
        lines.append("")

        lines.append("### Instances")
        lines.append("")
        lines.extend(_render_entity_instances_table(g, et.slug))
        lines.append("")

    # Entity-state tensions.
    lines.append("## Entity-state tensions")
    lines.append("")
    suspects = entity_state_conflict_suspects(g)
    if not suspects:
        lines.append("_(no entity-state tensions surfaced — clean)_")
        lines.append("")
    else:
        for s in suspects:
            lines.append(f"- `{s.left}` × `{s.right}` — {s.hint}")
        lines.append("")

    return "\n".join(lines).rstrip() + "\n"


def _render_entity_derived_constitution_section(g: TensionGraph) -> str:
    """Render the 'Entity-derived requirements' section for the CONSTITUTION block.

    Returns an empty string when entity_types is empty (section is omitted).
    """
    if not g.entity_types:
        return ""
    enforcer_str = ", ".join(f"`{c}`" for c in _ENTITY_COVERING_CHECKS)
    lines: list[str] = ["**Entity-derived requirements**", ""]
    for et in sorted(g.entity_types, key=lambda e: e.slug):
        lines.append(
            f"- **R-entity-{et.slug}** — *{et.description}* "
            f"[STRUCTURAL·entity · §Entity] [enforced_by: {enforcer_str}]"
        )
    lines.append("")
    return "\n".join(lines)


# ---------------------------------------------------------------------------
# GLOSSARY.md — the methodology's controlled vocabulary
# ---------------------------------------------------------------------------


def build_glossary(g: TensionGraph) -> str:
    """Build GLOSSARY.md (controlled vocabulary) as an LF string.

    Canon: §Glossary — generated mirror of hotam_spec.glossary.TERMS. The graph
    argument is used only to resolve the doc's `reader:` header line
    (R-doc-names-reader) — the vocabulary itself is framework-side, not domain
    content.
    """
    lines: list[str] = [BANNER, _reader_header_line("GLOSSARY", g), ""]
    lines.append("# GLOSSARY.md — Methodology controlled vocabulary (Hotam-Spec)")
    lines.append("")
    lines.append(
        "Generated mirror of the methodology's own canon terms — the framework's\n"
        "controlled vocabulary that every docstring and generated doc must use\n"
        "consistently. Terminology drift is invisibility (R-glossary-sync-test)."
    )
    lines.append("")
    lines.append(
        "Source: `spec/src/hotam_spec/glossary.py:TERMS`. Domain-side business terms\n"
        "(R-ids, axis slugs, stakeholders) live in `domains/<name>/graph.py` and are\n"
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
    the condition under which to re-open). Source of truth is the active
    domain's graph.py; this text is generated so it cannot drift.
    """
    lines: list[str] = [BANNER, _reader_header_line("HISTORY", g), ""]
    lines.append("# HISTORY.md — Methodology decision history (Hotam-Spec)")
    lines.append("")
    lines.append(
        "Generated from the anti-relitigation markers in the model: REJECTED\n"
        "requirements (what was tried and discarded — REPLACES marker) and DECIDED /\n"
        "REVISIT_WHEN conflict lifecycles (what was resolved, why, and the condition\n"
        "under which to re-open). Source of truth is the active domain's `graph.py`;\n"
        "this text is generated so it cannot drift."
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
# graph.json — machine-readable, READ-ONLY snapshot of the graph nodes
# ---------------------------------------------------------------------------


def build_graph_json(g: TensionGraph) -> str:
    """Build graph.json — a machine-readable, read-only snapshot of the graph nodes.

    Canon: §Graph — the single source of truth remains the active domain's
    `graph.py:build_graph()` (storage = the Python code itself,
    R-per-node-json-store REJECTED). graph.json is a GENERATED export, never a
    writer: it lets a non-Python reader consume the node roster without parsing
    Python, while the frozen dataclasses stay the one place a node is authored.

    Exports the four core node families (requirements / conflicts / assumptions /
    stakeholders) plus a flat `node_ids` list (every node id, sorted). Only
    stable, primitive fields are emitted so the JSON never leaks dataclass
    internals. Deterministic: keys are sorted, LF newline, utf-8, no timestamps.
    """
    payload = {
        "generated_from": "domains/<active>/graph.py:build_graph()",
        "note": (
            "READ-ONLY generated snapshot. Source of truth is graph.py; edit "
            "the graph only via tools/apply_proposal.py (R-no-hand-edit-graph, "
            "R-per-node-json-store REJECTED)."
        ),
        "requirements": [
            {
                "id": r.id,
                "claim": r.claim,
                "owner": r.owner,
                "status": r.status,
                "enforcement": r.enforcement,
                "enforced_by": list(r.enforced_by),
                "assumptions": list(r.assumptions),
                "why": r.why,
                "last_reviewed_at": getattr(r, "last_reviewed_at", ""),
                "review_after": getattr(r, "review_after", ""),
                "evidence": list(getattr(r, "evidence", ())),
                "source_refs": list(getattr(r, "source_refs", ())),
                "history": [
                    {"at": h.at, "summary": h.summary, "decided_by": h.decided_by}
                    for h in getattr(r, "history", ())
                ],
            }
            for r in g.requirements
        ],
        "conflicts": [
            {
                "id": c.id,
                "axis": c.axis,
                "context": c.context,
                "members": list(c.members),
                "steward": c.steward,
                "lifecycle": c.lifecycle,
            }
            for c in g.conflicts
        ],
        "assumptions": [
            {
                "id": a.id,
                "statement": a.statement,
                "status": a.status,
                "owner": a.owner,
            }
            for a in g.assumptions
        ],
        "stakeholders": [
            {"id": s.id, "name": s.name, "domain": s.domain}
            for s in g.stakeholders
        ],
    }
    payload["node_ids"] = sorted(
        [r.id for r in g.requirements]
        + [c.id for c in g.conflicts]
        + [a.id for a in g.assumptions]
        + [s.id for s in g.stakeholders]
    )
    return json.dumps(payload, indent=2, sort_keys=True, ensure_ascii=False) + "\n"


# ---------------------------------------------------------------------------
# DECISIONS.md — generated M-registry (open decisions mirrored from graph)
# ---------------------------------------------------------------------------


def _decisions_md_has_content(g: TensionGraph) -> bool:
    """Canon: §Requirement — True iff DECISIONS.md would render a real M-registry row (task #106 / L2-#6).

    DECISIONS.md is written ONLY when at least one Requirement carries a
    non-empty `m_tag` — otherwise it is a permanent "no M-tag yet" placeholder
    that never carries information. Callers (_process_domains / main) skip
    the write entirely when this is False.
    """
    return any(r.m_tag for r in g.requirements)


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

    lines: list[str] = [BANNER, _reader_header_line("DECISIONS", g), ""]
    lines.append("# DECISIONS.md — Open methodology decisions (Hotam-Spec)")
    lines.append("")
    lines.append(
        "Generated mirror of the M-registry. The SINGLE source of truth is the\n"
        "graph's OPEN requirements with non-empty `m_tag` in the active domain's\n"
        "`graph.py`. This file retires the hand-maintained M-table\n"
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

    lines: list[str] = [BANNER, _reader_header_line("CONSTITUTION", g), ""]
    lines.append("# CONSTITUTION.md — The operator's boot sequence (Hotam-Spec)")
    lines.append("")
    lines.append(
        "You — the AI agent reading this cold — are the prospective Operator of this\n"
        "repository. Read this file end-to-end before any action. It is generated from\n"
        "the methodology's SETTLED laws (the active domain's `graph.py`). It is your\n"
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
    # Pull the closed-loop description from the hotam_spec __init__ docstring.
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
        "`hotam_spec.invariants`):"
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
        "  1. `cd D:/dev/HotamSpec/spec && python -m pytest -q`   → suite green?"
    )
    lines.append(
        "  2. `python tools/gen_spec.py` (twice)                  → deterministic?"
    )
    lines.append(
        "  3. `python tools/what_now.py | head -20`               → what is the top action?"
    )
    lines.append(
        "  4. `python tools/what_now.py --report`                  → does the tick agree?"
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
            "_No content domain loaded yet — no `domains/<name>/graph.py` found or "
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
    lines.append("  - run `what_now.py` (including `--report`), `gen_spec.py`;")
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

# WHY a coarse bucket, never an exact character count: the LIVE-STATE block
# lives INSIDE CLAUDE.md, so computing CLAUDE.md's own size and writing that
# exact number into itself is a fixpoint trap — the file's length changes
# every time the digit-count of the printed number changes, so the printed
# number is never quite right and never converges. A fixed-string BUCKET
# label (OK / NEAR / OVER) has constant length regardless of the underlying
# byte count, so it is stable across regenerations by construction — the
# classification may change between runs (that's real signal), but the LINE
# ITSELF never grows or shrinks the file, so no fixpoint loop is possible.


def compute_cipher_lines(g: TensionGraph) -> tuple[str, str]:
    """Canon: §Context — pure graph -> (top_line, debt_line) for the three-cipher pulse.

    RULE: the SOLE source of the 'top action' and 'debt' cipher values. Both
    build_live_state (rendering the LIVE-STATE markdown block) and
    tools/emit_cipher.py (the UserPromptSubmit hook payload) call this SAME
    function against the SAME graph, so the hook's pulse is read directly off
    the graph rather than re-parsed out of the markdown build_live_state just
    rendered from it (no regex round-trip through the file emit_cipher used to
    read back from disk).
    """
    # Lazy import of what_now (same pattern as other tools).
    _tools = Path(__file__).resolve().parent
    if str(_tools) not in sys.path:
        sys.path.insert(0, str(_tools))
    import what_now as _what_now  # noqa: PLC0415

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
    # Real closeable debt only (ENFORCEABLE, not yet enforced); INHERENTLY_PROSE
    # requirements are permanent discipline, not debt (R-enforceability-kind-declared).
    unenforced = sum(1 for r in settled if r.is_closeable_debt())
    debt_line = (
        f"{settled_enforced}/{settled_total} SETTLED ENFORCED"
        f" · {len(draft)} DRAFT · {len(open_reqs)} OPEN"
        f" · {unenforced} closeable debt (ENFORCEABLE, still PROSE/STRUCTURAL)"
    )
    return top_line, debt_line


@dataclass(frozen=True)
class LiveStateModel:
    """Canon: §Context — structural form of the LIVE-STATE block, pre-render.

    RULE (I1, additive structural contract): `build_live_state_model(g)` is
    the SOLE place the five cipher/budget/crystal/context values are
    computed; `build_live_state` is a thin string-join over this model's
    fields, so the rendered markdown and the structural model can never
    drift apart (they are the same computation, not two). Added alongside
    the existing byte-level contract (build_live_state's output stays
    identical) — this is a PARALLEL structural contract, not a replacement
    (see task #91 / R-requirement-claim-is-atomic discipline: additive, not
    a rewrite of the test contract).
    """

    top_line: str
    debt_line: str
    budget_line: str
    crystal_line: str
    ctx_line: str


def build_live_state_model(g: TensionGraph) -> LiveStateModel:
    """Canon: §Context — pure graph -> LiveStateModel (the five cipher fields).

    WHY not the exact CLAUDE.md byte-size, and why not calling
    render_claude_md_from_template(g) directly: see fixpoint hazard comment
    above — the exact count can't converge, and the whole-template render
    itself calls this function (infinite recursion, via build_live_state).
    Approximated instead from the sibling sentinel blocks.
    """
    # Lazy import of context (same pattern as other tools).
    _tools = Path(__file__).resolve().parent
    if str(_tools) not in sys.path:
        sys.path.insert(0, str(_tools))
    import context as _context  # noqa: PLC0415

    top_line, debt_line = compute_cipher_lines(g)

    nodes = len(g.requirements) + len(g.conflicts) + len(g.assumptions)
    op_budget = 0
    op_measure = NODE_COUNT
    if g.operators:
        op_budget = g.operators[0].context_budget.limit
        op_measure = g.operators[0].context_budget.measure

    # R-live-state-budget-own-measure: render the graph/budget line in the
    # operator's OWN measure (never mixed units). CRYSTAL_CHARS operators are
    # measured against the resident crystal's real character count (same
    # source check_operator_within_budget and reflection._crystal_chars()
    # read); NODE_COUNT operators are measured against `nodes`. Mixing them
    # (chars-limit minus node-count, as the pre-fix code did) produced a
    # meaningless 'headroom' number every LIVE-STATE turn.
    if op_measure == CRYSTAL_CHARS:
        used = len(_read(CLAUDE_MD)) if CLAUDE_MD.exists() else 0
        budget_line = (
            f"- **graph:** {nodes} nodes (req+conflict+assumption);"
            f" OP-director budget {op_budget} chars (CRYSTAL_CHARS measure)"
            f" — resident crystal {used} chars (headroom {op_budget - used})"
        )
    else:
        budget_line = (
            f"- **graph:** {nodes} nodes (req+conflict+assumption);"
            f" OP-director budget {op_budget} nodes (NODE_COUNT measure)"
            f" (headroom {op_budget - nodes})"
        )

    # Honest bucket against the real host char cap — fixpoint-safe (see WHY
    # comment above). Approximate the resident crystal's size from every
    # OTHER sentinel block (REPO-MAP, THINKING-INDEX, EMBEDDED-THINKING,
    # EMBEDDED-TOOLS, DOMAIN-MAP, CONSTITUTION, AGENT-MAP, CONCEPT-MAP,
    # RECENTLY-REJECTED) plus the static template scaffolding, deliberately
    # calling the sibling block-builders directly rather than
    # render_claude_md_from_template(g) — that function (via render_mind_
    # content/render_business_content) calls build_live_state itself, so
    # calling it from HERE would recurse infinitely. This block's own
    # length is small and near-constant across the OK/NEAR/OVER boundaries,
    # so omitting it does not change which bucket is reported.
    approx_size = (
        len(_memo_block("embedded_thinking", _render_embedded_thinking_block))
        + len(_memo_block("embedded_tools", _render_embedded_tools_block))
        + len(_render_domain_map_block(g))
        + len(_memo_block("constitution", lambda: _render_constitution_block(g)))
        + len(_memo_block("agent_map", lambda: _scan_agent_map(g)))
        + len(_memo_block("concept_map", _scan_concept_map))
        + len(_memo_block("recently_rejected", lambda: _render_recently_rejected_block(g)))
        + len(_memo_block("operator_role", lambda: _render_operator_role_block(g)))
        + len(_memo_block("mediation_loop", _render_mediation_loop_block))
        + len(_memo_block("operator_recursion", _render_operator_recursion_block))
    )
    if approx_size >= _HOST_CHAR_CAP:
        crystal_line = (
            f"OVER host cap {_HOST_CHAR_CAP} chars — split/distill required"
        )
    elif approx_size >= _HOST_CHAR_WARN:
        crystal_line = (
            f"NEAR — approaching {_HOST_CHAR_WARN} char warn threshold "
            f"(host cap {_HOST_CHAR_CAP})"
        )
    else:
        crystal_line = (
            f"OK — under {_HOST_CHAR_WARN} char warn threshold "
            f"(host cap {_HOST_CHAR_CAP})"
        )

    ctx_line = _context.render_line()

    return LiveStateModel(
        top_line=top_line,
        debt_line=debt_line,
        budget_line=budget_line,
        crystal_line=crystal_line,
        ctx_line=ctx_line,
    )


def build_live_state(g: TensionGraph) -> str:
    """Build the LIVE-STATE block content (without sentinels) as an LF string.

    Canon: §Context — thin render over build_live_state_model(g); see that
    function for the actual computation. Kept as a separate entry point
    because ~dozens of test/tool call sites already call build_live_state(g)
    by name for the byte-level contract (R-claude-md-live-state-generated) —
    this wrapper preserves that contract unchanged while the model becomes
    the single source of the five values.
    """
    model = build_live_state_model(g)
    lines = [
        "### Live state (autogenerated by tools/gen_spec.py — do not hand-edit)",
        "",
        f"- **top action:** {model.top_line}",
        f"- **debt:** {model.debt_line}",
        model.budget_line,
        f"- **crystal:** {model.crystal_line}",
        f"- {model.ctx_line}",
    ]
    return "\n".join(lines)


def extract_live_state_block(claude_md_text: str) -> str | None:
    """Extract the text between LIVE-STATE sentinels (excluding sentinels).

    Delegates to hotam_spec.claude_md.extract_block (the reusable
    sentinel-bounded extraction helper). Returns None if sentinels are
    not found.
    """
    return _claude_md_extract_block(claude_md_text, "LIVE-STATE")



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


# ---------------------------------------------------------------------------
# Phase 3 (P22.D-continuation, task #9): framework-plumbing partition.
#
# Canon: §Requirement — R-constitution-separates-plumbing. hotam-spec-self is
# the framework modeling ITSELF, so ~2/3 of its SETTLED requirements are
# INTERNAL guarantees of the framework's own machinery (Entity/Agent/Domain/
# Process/Operator-internals/Lifecycle-keystone/Generator/bijection/anchor
# mechanics/CLAUDE.md machinery) rather than business claims the operator
# carries as "reality". The resident CONSTITUTION index should hold business +
# discipline atoms; framework-plumbing atoms relocate to a generated
# docs/gen/FRAMEWORK-INVARIANTS.md, reachable by an in-block pointer.
#
# This is a PRESENTATIONAL partition only: no atom changes status, no atom is
# dropped from REQUIREMENTS.md's full roster. The invariant that matters:
# CONSTITUTION-index-ids ⊎ FRAMEWORK-INVARIANTS-ids == all SETTLED ids
# (disjoint union, nothing lost) — enforced by tests/test_constitution.py.
#
# Partition approved by steward (framework-author), Phase 3 audit, task #9.
# Unlisted SETTLED ids default to business (no silent loss on new atoms).
# ---------------------------------------------------------------------------

_FRAMEWORK_PLUMBING_IDS: frozenset[str] = frozenset(
    {
        # Entity aspect machinery.
        "R-entity-type-lifecycle-wellformed",
        "R-entity-instance-state-in-lifecycle",
        "R-entity-instance-required-fields",
        "R-entity-instance-id-prefix",
        "R-entity-instance-refs-resolve",
        "R-entity-field-kind-known",
        "R-entity-typed-anchors",
        "R-process-drives-existing-entities",
        "R-step-invokes-known-transition",
        "R-entity-derived-requirement",
        "R-entity-is-declarative",
        "R-entity-reuses-lifecycle",
        "R-entity-checks-by-iteration",
        "R-entity-state-conflict-surfaced",
        "R-entities-md-generated",
        # Agent-scaffolding machinery.
        "R-agent-has-own-tools-dir",
        "R-agent-declares-purpose",
        "R-agent-map-generated",
        "R-agent-scoped-constitution",
        "R-agent-is-recursive-director",
        "R-agent-has-docs-dir",
        "R-agent-references-shared-docs",
        "R-subagent-gets-its-claude-md",
        "R-agent-is-a-directory",
        "R-sub-agent-crystal-triad",
        # Domain-federation machinery.
        "R-domain-is-a-directory",
        "R-domain-has-manifest",
        "R-domain-declares-director",
        "R-domain-owns-graph-py",
        "R-domain-owns-docs-gen",
        "R-domain-owns-tools-and-agents",
        "R-domain-map-generated",
        "R-director-agent-required-per-domain",
        "R-domain-has-docs-dir",
        "R-content-layout-evolution",
        # Process aspect machinery.
        "R-process-types-exist",
        "R-process-opt-in",
        "R-process-lifecycle-wellformed-aspect",
        "R-process-roles-declared-aspect",
        "R-process-goal-owner-is-operator-aspect",
        "R-process-typed-anchors-extended",
        # Goal aspect validation mechanics.
        "R-goal-target-kind-known",
        "R-goal-owner-is-operator",
        # Operator-internals.
        "R-operator-is-frozen-dataclass",
        "R-operator-references-stakeholder",
        "R-operator-has-context-budget",
        "R-operator-may-have-parent",
        "R-context-budget-rule",
        "R-operator-not-self-approve",
        "R-operator-type-vs-facet",
        # Lifecycle keystone.
        "R-lifecycle-type-exists",
        "R-lifecycle-validates-requirement",
        "R-lifecycle-validates-conflict",
        "R-lifecycle-validates-operator",
        "R-lifecycle-validates-goal",
        "R-statemachine-reachable",
        "R-statemachine-deterministic",
        "R-statemachine-terminal-or-cyclic",
        "R-statemachine-guard-on-assumption",
        # Generator / docs machinery.
        "R-drift-structurally-impossible",
        "R-deterministic-generation",
        "R-glossary-generated",
        "R-glossary-sync-fails-dead",
        "R-glossary-sync-fails-unused",
        "R-glossary-drift-stable",
        "R-history-generated-from-rejected",
        "R-history-generated-from-decided",
        "R-docs-generated-from-requirements",
        "R-repo-map-generated",
        "R-claude-md-live-state-generated",
        "R-root-claude-md-is-sentinel-only",
        "R-claude-md-template-driven",
        "R-framework-shared-docs-generated",
        "R-shared-tool-doc-from-docstring-and-help",
        "R-shared-thinking-doc-from-canon-sections",
        # Content-free / ship-state.
        "R-content-free-no-business-data",
        "R-content-free-no-examples",
        "R-content-free-no-seed-graph",
        "R-empty-content-wellformed",
        "R-empty-content-calm-banner",
        "R-empty-content-gen-notice",
        # Bijection / enforcement machinery.
        "R-bijection-r-to-enforcer",
        "R-enforcement-levels-declared",
        "R-enforced-names-enforcer",
        "R-requirement-enforced",
        "R-enforceability-kind-declared",
        "R-check-method-is-atomic",
        "R-audit-atomicity-tool",
        "R-method-matches-docstring",
        # Anchor mechanics.
        "R-m-tag-format-valid",
        "R-anchor-taxonomy",
        # CLAUDE.md machinery.
        "R-recently-rejected-surfaced",
        "R-operator-prompt-loaded-at-session-start",
        "R-three-cipher-pulse-structurally-injected",
        "R-post-compact-regen-from-substrate",
        "R-claude-md-consolidates-when-single-agent",
        "R-operator-crystal-embeds-thinking-distilled",
        "R-operator-crystal-embeds-tools-distilled",
        "R-crystal-carries-role-seed",
        "R-crystal-carries-mediation-loop",
        "R-crystal-carries-recursion-seed",
        "R-constitution-is-index",
        "R-crystal-is-claude-md",
        "R-crystal-reload-by-reference",
        "R-crystal-tree-hierarchy",
        # Misc.
        "R-project-name-hotam-spec",
        "R-parallel-mutating-agents-use-worktree",
        # Dependency mechanics.
        "R-dependency-tracked",
        "R-dependency-drives-parallel",
        "R-dependency-drives-sequential",
        # Tool-plumbing.
        "R-tools-registry-generated",
        "R-tool-is-its-own-requirement",
        # This partition atom itself is CLAUDE.md-machinery (Phase 3, task #9):
        # its claim describes the presentational split, not a business claim
        # the operator mediates.
        "R-constitution-separates-plumbing",
    }
)


def _is_framework_plumbing(rid: str) -> bool:
    """Canon: §Requirement — True iff rid is in the Phase-3 plumbing partition.

    RULE (R-constitution-separates-plumbing): membership is by exact id in
    _FRAMEWORK_PLUMBING_IDS. Unlisted ids default to business/discipline
    (False) — a new SETTLED atom is resident by default, never silently
    dropped from the CONSTITUTION index.
    """
    return rid in _FRAMEWORK_PLUMBING_IDS


def _categorize_requirement(rid: str) -> str:
    """Return the category label for a requirement id. Deterministic."""
    for label, prefixes in _DIGEST_CATEGORIES:
        for prefix in prefixes:
            if rid.startswith(prefix):
                return label
    return "Other"


_ENFORCEMENT_FLAG = {"ENFORCED": "E", "STRUCTURAL": "S", "PROSE": "P"}


def _constitution_index_line(rid: str, claim: str, enforcement: str) -> str:
    """Canon: §Requirement — one index line: id + enforcement flag.

    RULE (R-constitution-is-index): a single atomic renderer shared by the
    root and every scoped CONSTITUTION block so the line format cannot drift
    between the two call sites. enforcement collapses to a single-char flag
    ([E]/[S]/[P]) — full claims live in docs/gen/REQUIREMENTS.md, enforcement
    detail in docs/gen/UNENFORCED.md, not resident here.
    """
    flag = _ENFORCEMENT_FLAG.get(enforcement, "?")
    return f"{rid} [{flag}]"


#: Rule-clusters for Constitution-index rendering only (presentational —
#: no graph node is merged, split or edited). Each tuple is
#: (representative id, id-prefix). Every SETTLED requirement whose id
#: starts with `prefix` collapses into ONE index token headed by
#: `representative`, e.g. "R-land-gate-tier-selector [E] (+6 related: ...)"
#: instead of 7 separate tokens. This is pure rendering: every member id
#: still appears verbatim (as a literal substring) inside the token, so
#: R-constitution-is-index's "every SETTLED id appears in the block"
#: guarantee (test_constitution_lists_all_settled /
#: test_constitution_partitions_all_settled) is untouched. Membership is by
#: id-PREFIX (not a graph relation — none of these atoms carry a `refines`/
#: `supports` edge to a common parent today), the simplest mechanism that
#: needs no graph write (D5, 2026-07-10 steward delegation).
_RULE_CLUSTER_PREFIXES: tuple[tuple[str, str], ...] = (
    ("R-land-gate-tier-selector", "R-land-gate-"),
    ("R-land-gate-tier-selector", "R-land-tier-"),
    ("R-land-gate-tier-selector", "R-tiered-gate-"),
    ("R-land-gate-tier-selector", "R-commit-boundary-checkable"),
    ("R-attention-registry", "R-attention-"),
    ("R-tension-audit-shortlist-tool", "R-tension-audit-"),
    ("R-active-loop-protocol", "R-active-loop-"),
)


def _cluster_representative(rid: str) -> str | None:
    """Return the cluster representative id for `rid`, or None if unclustered.

    RULE: first matching prefix in _RULE_CLUSTER_PREFIXES wins (table is
    small and non-overlapping by construction — see the module comment).
    """
    for representative, prefix in _RULE_CLUSTER_PREFIXES:
        if rid.startswith(prefix):
            return representative
    return None


def _cluster_index_items(reqs: list) -> list[str]:
    """Canon: §Requirement — group a sorted requirement list into index tokens.

    RULE (D5 rule-clusters): requirements sharing a cluster representative
    (see _cluster_representative) collapse into ONE token:
    "<representative> [<flag>] (+N related: <id> [<flag>], ...)". Every
    other requirement renders as today, one token per id
    (_constitution_index_line). Order of `reqs` is preserved for the
    non-clustered items; clustered items are emitted at the position of
    their first (alphabetically earliest, since callers pre-sort by id)
    member.

    WHY presentational only: this changes ONLY how the Constitution index
    renders — no Requirement node is merged, renamed or edited. The full,
    un-clustered id list still lives in docs/gen/REQUIREMENTS.md and
    docs/gen/CONSTITUTION.md; every member id remains a literal substring
    of this block (R-constitution-is-index unchanged).
    """
    by_representative: dict[str, list] = {}
    for r in reqs:
        rep = _cluster_representative(r.id)
        if rep is not None:
            by_representative.setdefault(rep, []).append(r)

    emitted_reps: set[str] = set()
    items: list[str] = []
    for r in reqs:
        rep = _cluster_representative(r.id)
        if rep is None:
            items.append(_constitution_index_line(r.id, r.claim, r.enforcement))
            continue
        if rep in emitted_reps:
            continue
        members = by_representative[rep]
        if len(members) == 1:
            # Solo membership (cluster prefix matched but no siblings present
            # in this requirement set, e.g. a scoped/partial render) — no
            # value in a "(+0 related)" token, fall back to the plain line.
            items.append(_constitution_index_line(r.id, r.claim, r.enforcement))
            emitted_reps.add(rep)
            continue
        head = next((m for m in members if m.id == rep), members[0])
        rest = [m for m in members if m.id != head.id]
        head_flag = _ENFORCEMENT_FLAG.get(head.enforcement, "?")
        rest_str = ", ".join(
            f"{m.id}[{_ENFORCEMENT_FLAG.get(m.enforcement, '?')}]" for m in rest
        )
        items.append(f"{head.id} [{head_flag}] (+{len(rest)} related: {rest_str})")
        emitted_reps.add(rep)
    return items


@dataclass(frozen=True)
class ConstitutionCategory:
    """Canon: §Requirement — one Constitution-index category, pre-render.

    RULE (I1, additive structural contract): the (label, ordered requirement
    list) grouping decision that `_render_constitution_block` turns into
    markdown lines, extracted so a test can assert ON THE GROUPING (which
    category each SETTLED id lands in, and in what order) without parsing
    the rendered `**Label** — id [flag] · id [flag] ...` text back apart.
    `requirements` holds the actual graph `Requirement` objects (already
    typed: `.id`, `.claim`, `.enforcement`) sorted by id, mirroring exactly
    what `_render_constitution_block` iterates over — this model does not
    re-derive those fields, only the grouping + ordering around them.
    """

    label: str
    requirements: tuple  # tuple[Requirement, ...] — graph nodes, not re-typed here


def build_constitution_index_model(g: TensionGraph) -> list[ConstitutionCategory]:
    """Canon: §Requirement — pure graph -> ordered [ConstitutionCategory].

    Business + discipline SETTLED requirements only (framework-plumbing ids
    are excluded — see R-constitution-separates-plumbing / task #9 note on
    `_render_constitution_block`, which calls this for its grouping). Same
    category order as _DIGEST_CATEGORIES, then "Other"; same per-category id
    sort as the render function has always used.
    """
    all_settled = [r for r in g.requirements if r.status == SETTLED]
    settled = [r for r in all_settled if not _is_framework_plumbing(r.id)]
    if not settled:
        return []

    groups: dict[str, list] = {}
    for r in settled:
        cat = _categorize_requirement(r.id)
        groups.setdefault(cat, []).append(r)

    for cat in groups:
        groups[cat].sort(key=lambda r: r.id)

    cat_order = [label for label, _ in _DIGEST_CATEGORIES if label in groups]
    if "Other" in groups:
        cat_order.append("Other")

    return [ConstitutionCategory(label=cat, requirements=tuple(groups[cat])) for cat in cat_order]


def _render_constitution_block(g: TensionGraph) -> str:
    """Render the CONSTITUTION index block content (without sentinels).

    Canon: §Requirement — R-constitution-is-index: one line per SETTLED
    requirement (id + truncated claim + enforcement flag), grouped by
    build_constitution_index_model(g) (the structural grouping — see that
    function). Full claims + WHY + assumptions live in the domain's
    docs/gen/REQUIREMENTS.md roster; enforcement detail in
    docs/gen/UNENFORCED.md — this block is an index, not a catalog
    (R-crystal-reload-by-reference).

    Phase 3 (R-constitution-separates-plumbing, task #9): renders ONLY
    business + discipline atoms — SETTLED ids in _FRAMEWORK_PLUMBING_IDS are
    relocated to docs/gen/FRAMEWORK-INVARIANTS.md (build_framework_invariants),
    reachable via the in-block pointer line below. No atom changes status;
    this is a presentational partition of one index into two.
    """
    all_settled = [r for r in g.requirements if r.status == SETTLED]
    n_settled = len([r for r in all_settled if not _is_framework_plumbing(r.id)])
    domain = _active_domain()
    domain_name = domain.name if domain else "hotam-spec-self"
    roster_path = f"domains/{domain_name}/docs/gen/REQUIREMENTS.md"
    invariants_path = f"domains/{domain_name}/docs/gen/FRAMEWORK-INVARIANTS.md"
    n_plumbing = len(all_settled) - n_settled

    categories = build_constitution_index_model(g)

    if not categories:
        base = "<!-- (generated by tools/gen_spec.py — do not hand-edit) -->\n\n_No SETTLED requirements yet._"
        return base

    lines: list[str] = [
        "<!-- (generated by tools/gen_spec.py — do not hand-edit) -->",
        "",
        "### Constitution index (business + discipline SETTLED requirements)",
        "",
        f"> Full claim + WHY + assumptions: `{roster_path}` (roster) ·",
        "> enforcement detail: `docs/gen/UNENFORCED.md`.",
        "> Flags: [E] ENFORCED · [S] STRUCTURAL · [P] PROSE.",
        f"> Framework internals ({n_plumbing} atoms): `{invariants_path}`.",
        "",
    ]
    for category in categories:
        items = _cluster_index_items(list(category.requirements))
        lines.append(f"**{category.label}** — {' · '.join(items)}")
        lines.append("")

    return "\n".join(lines).rstrip()


def build_framework_invariants(
    g: TensionGraph, domain_name: str | None = None
) -> str:
    """Build FRAMEWORK-INVARIANTS.md — the relocated framework-plumbing index.

    Canon: §Requirement — R-constitution-separates-plumbing (Phase 3, task
    #9). Holds every SETTLED requirement in _FRAMEWORK_PLUMBING_IDS (id +
    truncated claim + enforcement flag, same index-line format as the root
    CONSTITUTION block) plus the Tool-derived requirements section and the
    Entity-derived requirements section, both of which are framework-internal
    machinery projections. This is a generated MIRROR: full claims + WHY live
    in REQUIREMENTS.md; this file is an index, not a catalog.
    """
    settled = [
        r
        for r in g.requirements
        if r.status == SETTLED and _is_framework_plumbing(r.id)
    ]
    # domain_name is passed explicitly when rendering ONE specific domain via
    # _process_domains (so the in-doc path pointers name THAT domain, not the
    # env-active one — R-root-crystal-follows-pin). Falls back to the env-active
    # domain for the top-level single-domain render.
    if domain_name is None:
        domain = _active_domain()
        domain_name = domain.name if domain else "hotam-spec-self"
    roster_path = f"domains/{domain_name}/docs/gen/REQUIREMENTS.md"

    lines: list[str] = [BANNER, _reader_header_line("FRAMEWORK_INVARIANTS", g), ""]
    lines.append("# FRAMEWORK-INVARIANTS.md — Framework-plumbing index (Hotam-Spec)")
    lines.append("")
    lines.append(
        "Hotam-Spec is the framework modeling ITSELF (hotam-spec-self domain), so "
        "most of its SETTLED requirements are internal guarantees of the "
        "framework's own machinery (Entity/Agent/Domain/Process/Operator-"
        "internals/Lifecycle-keystone/Generator/bijection/anchor mechanics/"
        "CLAUDE.md machinery), not business claims the operator mediates as "
        "reality. This index holds exactly those framework-internal atoms, "
        "relocated out of the root CLAUDE.md CONSTITUTION index "
        "(R-constitution-separates-plumbing, Phase 3, task #9)."
    )
    lines.append("")
    lines.append(
        f"> Full claim + WHY + assumptions: `{roster_path}` (roster) · "
        "enforcement detail: `docs/gen/UNENFORCED.md`."
    )
    lines.append("> Flags: [E] ENFORCED · [S] STRUCTURAL · [P] PROSE.")
    lines.append(
        "> No atom here changed status by this relocation — every id below "
        "is (and remains) SETTLED in the graph; only ITS RENDER LOCATION moved."
    )
    lines.append("")

    if not settled:
        lines.append("_No framework-plumbing SETTLED requirements yet._")
    else:
        groups: dict[str, list] = {}
        for r in settled:
            cat = _categorize_requirement(r.id)
            groups.setdefault(cat, []).append(r)
        for cat in groups:
            groups[cat].sort(key=lambda r: r.id)
        cat_order = [label for label, _ in _DIGEST_CATEGORIES if label in groups]
        if "Other" in groups:
            cat_order.append("Other")
        for cat in cat_order:
            lines.append(f"**{cat}**")
            lines.append("")
            for r in groups[cat]:
                lines.append(_constitution_index_line(r.id, r.claim, r.enforcement))
            lines.append("")

    # Tool-derived requirements (R-tool-is-its-own-requirement projection) —
    # framework-internal machinery, relocated here in full (was previously
    # appended to the root CONSTITUTION block).
    tool_reqs = _scan_tool_requirements()
    if tool_reqs:
        lines.append("**Tool-derived requirements**")
        lines.append("")
        for tr in tool_reqs:
            lines.append(_constitution_index_line(tr.id, tr.claim, "STRUCTURAL"))
        lines.append("")

    # Entity-derived requirements (R-entity-derived-requirement projection) —
    # dynamic per-EntityType synthetic ids, also framework-internal machinery.
    entity_section = _render_entity_derived_constitution_section(g)
    if entity_section:
        lines.append(entity_section)

    return "\n".join(lines).rstrip() + "\n"


# ---------------------------------------------------------------------------
# Atomized methodology docs (docs/methodology/atoms/)
# Each generator emits one topic file from SETTLED requirements tagged with
# that topic. Topic-grouping uses a helper that scans requirement ids for
# known prefixes — atomic methods, one per topic.
# ---------------------------------------------------------------------------


def _select_settled(g: TensionGraph, predicate) -> list:
    """Return SETTLED requirements satisfying predicate(r) -> bool. One atom."""
    return [r for r in g.requirements if r.status == SETTLED and predicate(r)]


def _render_atoms(title: str, intro: str, reqs: list, reader: str = "") -> str:
    """Render one atoms file from a sorted requirement list. One atom.

    `reader` (when non-empty) is the pre-rendered `reader: <id>` header line
    (R-doc-names-reader) — callers pass `_reader_header_line(doc_kind, g)`.
    """
    header: list[str] = [BANNER]
    if reader:
        header.append(reader)
    header.append("")
    lines: list[str] = [*header, f"# {title}", "", intro, "", "---", ""]
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
            lines += _render_freshness_lines(r)
    return "\n".join(lines).rstrip() + "\n"


def _render_freshness_lines(r) -> list[str]:
    """Canon: §Requirement — render the optional freshness fields for one requirement.

    Emits a line each for last_reviewed_at / review_after / evidence /
    source_refs when populated, and the derived change trail (history). Absent
    fields render nothing (terse output for the common case). Shared so both the
    atoms detail and any future per-requirement view stay consistent.
    """
    out: list[str] = []
    reviewed = getattr(r, "last_reviewed_at", "")
    review_after = getattr(r, "review_after", "")
    evidence = getattr(r, "evidence", ())
    source_refs = getattr(r, "source_refs", ())
    history = getattr(r, "history", ())
    if reviewed:
        out += [f"**Last reviewed.** {reviewed}", ""]
    if review_after:
        out += [f"**Review after.** {review_after}", ""]
    if evidence:
        out += ["**Evidence.** " + "; ".join(evidence), ""]
    if source_refs:
        out += ["**Sources.** " + ", ".join(source_refs), ""]
    if history:
        out += ["**Change history.**", ""]
        for h in history:
            who = f" · {h.decided_by}" if h.decided_by else ""
            out += [f"- {h.at}{who} — {h.summary}"]
        out += [""]
    return out


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
        reader=_reader_header_line("ATOMS_OPERATOR", g),
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
        reader=_reader_header_line("ATOMS_SUBSTRATE", g),
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
        reader=_reader_header_line("ATOMS_DISCIPLINE", g),
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
        reader=_reader_header_line("ATOMS_CHECK", g),
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
        "### Constitution index (scoped)",
        "",
    ]
    for cat in cat_order:
        lines.append(f"**{cat}**")
        lines.append("")
        for r in groups[cat]:
            lines.append(_constitution_index_line(r.id, r.claim, r.enforcement))
        lines.append("")

    if scoped_tools:
        lines.append("**Tool-derived requirements**")
        lines.append("")
        for tr in scoped_tools:
            lines.append(_constitution_index_line(tr.id, tr.claim, "STRUCTURAL"))
        lines.append("")

    return "\n".join(lines).rstrip()


def _render_overlap_block(g: TensionGraph, this_id: str, scopes: dict[str, tuple[str, ...]]) -> str:
    """Canon: §Scope — render the OVERLAP block for one agent against all others.

    RULE (R-scope-overlap-generated): for every OTHER agent id in `scopes`,
    compute hotam_spec.scope_projection.scope_overlap(project_scope(g,
    scopes[this_id]), project_scope(g, scopes[other_id])); if the overlap is
    non-empty, render one line naming the other agent and the shared node/axis
    ids. If `this_id`'s own scope is empty, OR it shares nothing with any
    other agent (the CURRENT meta-domain state: a single operator, SCOPE=()),
    emit the calm '(no scope overlap)' placeholder — R-empty-content-
    wellformed's discipline applies to overlap output exactly as it does to
    every other generated block: never printed as an error, never silently
    omitted (the sentinel pair is always present so the state is visible).

    WHY generated per-agent rather than once globally: each agent's crystal
    should show ITS OWN contested surface without forcing it to cross-
    reference a shared global file — same locality-of-context principle
    _render_scoped_constitution_block already applies to the CONSTITUTION
    block.
    """
    from hotam_spec.scope_projection import (  # noqa: PLC0415
        overlap_node_ids,
        project_scope,
        scope_overlap,
    )

    this_scope = scopes.get(this_id, ())
    lines: list[str] = [
        "<!-- (generated by tools/gen_spec.py — do not hand-edit) -->",
        "",
        "### Scope overlap",
        "",
    ]
    if not this_scope:
        lines.append("_(no scope overlap)_")
        return "\n".join(lines).rstrip()

    this_view = project_scope(g, this_scope)
    found_any = False
    for other_id in sorted(scopes):
        if other_id == this_id:
            continue
        other_scope = scopes[other_id]
        if not other_scope:
            continue
        other_view = project_scope(g, other_scope)
        overlap = scope_overlap(this_view, other_view)
        node_ids = overlap_node_ids(overlap)
        if not node_ids and not overlap.axes:
            continue
        found_any = True
        parts = []
        if node_ids:
            parts.append("nodes: " + ", ".join(f"`{n}`" for n in node_ids))
        if overlap.axes:
            parts.append("axes: " + ", ".join(f"`{a}`" for a in overlap.axes))
        lines.append(f"- with `{other_id}` — " + "; ".join(parts))

    if not found_any:
        lines.append("_(no scope overlap)_")
    return "\n".join(lines).rstrip()


def _regenerate_agent_constitutions(
    g: TensionGraph,
    agents_root: Path | None = None,
) -> None:
    """Regenerate CONSTITUTION + OVERLAP blocks in all spec/agents/<name>/CLAUDE.md files.

    Walks agents_root (defaults to _AGENTS_ROOT = spec/agents/). For each
    sub-directory found:
    - Loads scope.py via importlib and extracts the SCOPE tuple.
    - Filters g.requirements (SETTLED) + tool-derived requirements by scope.
    - Renders a scoped CONSTITUTION block (same category grouping as root).
    - Writes the block between the CONSTITUTION sentinels in CLAUDE.md.
    - Renders a scoped OVERLAP block (§Scope, R-scope-overlap-generated)
      against every OTHER discovered agent's SCOPE, writing it between
      OVERLAP sentinels immediately after CONSTITUTION:END (inserted if
      absent — the OVERLAP sentinel pair is new with this generator version
      and is not part of create_agent.py's frozen scaffold, so older agent
      CLAUDE.md files get it appended on first regen rather than erroring).

    Raises RuntimeError if CONSTITUTION sentinels are absent in an agent
    CLAUDE.md — missing sentinels indicate manual corruption (the scaffold
    from create_agent.py always emits them). Missing OVERLAP sentinels are
    NOT an error (backward-compat insert).

    No-op if agents_root does not exist or contains no sub-directories. With
    zero or one scoped agent (today's state: no spec/agents/ directory at
    all), no OVERLAP block is ever non-empty — the projection design (§Scope)
    guarantees overlap output is byte-identical to today whenever fewer than
    two operators carry a non-empty SCOPE.
    Deterministic: agents processed in sorted name order; requirements sorted by
    category then by id. LF, utf-8, no timestamps.
    """
    import importlib.util  # noqa: PLC0415

    root = agents_root or _AGENTS_ROOT
    if not root.exists():
        return

    # Pre-scan tool requirements once (shared across agents).
    tool_reqs = _scan_tool_requirements()

    # Pass 1: discover every agent's SCOPE tuple (needed before rendering any
    # single agent's OVERLAP block, which must see ALL other agents' scopes).
    agent_dirs = [d for d in sorted(root.iterdir()) if d.is_dir()]
    scopes: dict[str, tuple[str, ...]] = {}
    for agent_dir in agent_dirs:
        scope_py = agent_dir / "scope.py"
        if not scope_py.exists():
            continue
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
        scopes[agent_dir.name] = tuple(getattr(module, "SCOPE", ()))

    for agent_dir in agent_dirs:
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

        scope = scopes[agent_dir.name]

        # Render the scoped CONSTITUTION block.
        new_const_block = _render_scoped_constitution_block(g, scope, tool_reqs)
        # Render the scoped OVERLAP block against every other agent's scope.
        new_overlap_block = _render_overlap_block(g, agent_dir.name, scopes)

        # Update the agent's CLAUDE.md between CONSTITUTION sentinels.
        text = _read(claude_md_path)
        if _CONST_BEGIN not in text or _CONST_END not in text:
            raise RuntimeError(
                f"Agent CLAUDE.md at '{claude_md_path}' is missing "
                f"CONSTITUTION sentinels ('{_CONST_BEGIN}' / '{_CONST_END}'). "
                "This indicates manual corruption; the scaffold always emits them."
            )

        text = _claude_md_replace_block(text, "CONSTITUTION", new_const_block)

        # Update (or insert) the OVERLAP block immediately after CONSTITUTION:END.
        if _OVERLAP_BEGIN in text and _OVERLAP_END in text:
            text = _claude_md_replace_block(text, "OVERLAP", new_overlap_block)
        else:
            text = _claude_md_insert_block_after(
                text, "CONSTITUTION", "OVERLAP", new_overlap_block
            )

        _write(claude_md_path, text)
        print(f"updated agent: {claude_md_path}")


# ---------------------------------------------------------------------------
# REPO-MAP block — generated file-index inside CLAUDE.md
# ---------------------------------------------------------------------------

_CANON_ROLE_RE = re.compile(r"^Canon:\s+\S+\s+[—\-]\s+(.+)$")


def _resolve_active_content_dir() -> Path:
    """Return the active content dir: domains/<active>/ or legacy spec/content/.

    Computed once at import time for backward-compat (CONTENT_DIR used by
    _scan_repo_map() and test_repo_map.py).
    """
    active = _select_active_domain_dir(_sorted_domain_dirs())
    if active is not None:
        return active
    return SPEC_ROOT / "content"


CONTENT_DIR = _resolve_active_content_dir()


def _docstring_role(path: Path) -> str:
    """Extract a one-line role from a Python file's module docstring.

    Strips the optional 'Canon: §X — ' prefix so that only the descriptive
    part is returned.  Falls back to '(no docstring)' if none is present.
    """
    try:
        tree = parse_source(path)
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
                # Drop trailing " (Hotam-Spec)" and similar suffixes for brevity.
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
      - Framework body  (spec/src/hotam_spec/*.py  — excluding __init__.py)
      - Tools           (spec/tools/*.py        — excluding __init__.py)
      - Domain content  (domains/<name>/*.py    — excluding __init__.py and README.md)
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
    lines.append("**Framework body** (`spec/src/hotam_spec/`)")
    lines.append("")
    for p in sorted(_src.glob("*.py")):
        if p.name.startswith("_"):
            continue
        role = _docstring_role(p)
        lines.append(f"- `spec/src/hotam_spec/{p.name}` — {role}")
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
    # Conditional materialization (task #106 / L2-#6): DECISIONS.md /
    # ENTITIES.md are only written when their registry is non-empty (see
    # _decisions_md_has_content / _entities_md_has_content). When the
    # registry is empty the file itself is absent from _gen — but its
    # emptiness is still surfaced here as an explicit one-line note, so the
    # reader learns "registry checked, currently empty" rather than seeing
    # no trace of the M-registry / entity registry at all.
    if graph is not None:
        if not _decisions_md_has_content(graph) and not (_gen / "DECISIONS.md").exists():
            lines.append(f"- `{_gen_rel}/DECISIONS.md` — _(not written: M-registry empty)_")
        if not _entities_md_has_content(graph) and not (_gen / "ENTITIES.md").exists():
            lines.append(f"- `{_gen_rel}/ENTITIES.md` — _(not written: no entity_types declared)_")
    lines.append("")

    return "\n".join(lines).rstrip()


def build_repo_map_md(
    g: TensionGraph,
    *,
    content_dir: Path | None = None,
    gen_dir: Path | None = None,
) -> str:
    """Build REPO-MAP.md (repository file index) as an LF string.

    Canon: §Generator — the repository map, relocated from the resident crystal
    (root CLAUDE.md REPO-MAP block) to a generated doc to save ~8k chars of
    crystal space. The content is the same _scan_repo_map() output wrapped in
    a standard generated-doc header.

    `content_dir`/`gen_dir` default to the module-level env/pin-resolved
    CONTENT_DIR/GEN_DIR (correct for the root crystal's own REPO-MAP, which
    should describe the active domain). `_process_domains` passes THIS
    domain's own dirs explicitly — otherwise, under a foreign
    HOTAM_SPEC_ACTIVE_DOMAIN (e.g. apply_proposal's --docs-only subprocess
    for a non-pinned domain), every domain's REPO-MAP.md would describe the
    env-active domain's content instead of its own (R-root-crystal-follows-pin
    applied the same fix to the `reader:` header; this closes the same gap
    for REPO-MAP).
    """
    content = _scan_repo_map(graph=g, content_dir=content_dir, gen_dir=gen_dir)
    # Strip the internal header comment — the full doc has its own.
    content = content.replace(
        "<!-- (generated by tools/gen_spec.py — do not hand-edit) -->\n\n", ""
    )
    lines: list[str] = [BANNER, _reader_header_line("REPO_MAP", g), ""]
    lines.append("# REPO-MAP.md — Repository file index (Hotam-Spec)")
    lines.append("")
    lines.append(content)
    lines.append("")
    return "\n".join(lines).rstrip() + "\n"


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


# ---------------------------------------------------------------------------
# Concern 2: _active_domain() — backward-compat helper
# ---------------------------------------------------------------------------


def _active_domain() -> Path | None:
    """Return the active domains/<name>/ dir, or None if domains/ is empty.

    RULE: check HOTAM_SPEC_ACTIVE_DOMAIN env var first (must name an existing
    domains/<name>/ dir); then fall back to the first domain alphabetically
    (delegates to _select_active_domain_dir(), the SAME resolution helper
    used by _resolve_active_gen_dir() / _resolve_active_content_dir() /
    _resolve_active_agents_root()). Mirrors
    hotam_spec.graph._active_domain_graph_file()'s resolution order exactly,
    so the ROLE-block scope_label and the loaded graph content (which DOES
    honor the env var) never disagree about which domain is active (a
    mismatch previously showed e.g. "Operator of `hotam-dev`" next to a
    SETTLED count that was actually hotam-spec-self's).

    When None, all generation falls back to the legacy spec/content/graph.py path.
    When non-None, the active graph lives in domains/<name>/graph.py and the
    domain's docs go into domains/<name>/docs/gen/.
    """
    return _select_active_domain_dir(_sorted_domain_dirs())


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
            tree = parse_source(py_file)
        except Exception:
            continue

        rel = f"spec/src/hotam_spec/{py_file.name}"

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


# Intra-process caches for the two shared-docs builders. Both are called
# twice per gen_spec run: once by _render_embedded_*_block (CLAUDE.md crystal)
# and once by _write_shared_*_docs (spec/docs/ disk write). The computation
# is deterministic within a single run (same source files + same graph), so
# caching the result avoids a full re-collect+render pass (~0.9s saved).
_SHARED_THINKING_DOCS_CACHE: dict[str, str] | None = None
_SHARED_TOOL_DOCS_CACHE: dict[str, str] | None = None


def build_shared_thinking_docs(
    src_dir: Path | None = None,
    reader_stakeholder_ids: frozenset[str] | None = None,
) -> dict[str, str]:
    """Build content for spec/docs/thinking/<topic-slug>.md files.

    `reader_stakeholder_ids` (when None) resolves against the active domain's
    graph so the `reader:` header (R-doc-names-reader) tracks real substrate;
    callers that already loaded a graph (e.g. main()) should pass
    `stakeholder_ids(g)` to avoid a redundant load.

    Returns dict mapping topic_slug -> markdown_content.
    """
    global _SHARED_THINKING_DOCS_CACHE
    if _SHARED_THINKING_DOCS_CACHE is not None:
        return _SHARED_THINKING_DOCS_CACHE
    _src = src_dir or SRC
    if reader_stakeholder_ids is None:
        try:
            reader_stakeholder_ids = stakeholder_ids(load_content_graph())
        except Exception:
            reader_stakeholder_ids = frozenset()
    reader_line = _doc_reader_line(
        "SHARED_THINKING", reader_stakeholder_ids, active_domain_doc_readers()
    )
    by_topic = _collect_canon_docstrings(_src)
    result: dict[str, str] = {}

    for topic in sorted(by_topic):
        slug = _slug(topic)
        entries = by_topic[topic]
        lines: list[str] = [
            f"<!-- Auto-generated by spec/tools/gen_spec.py from Canon: §{topic} docstrings. Do not hand-edit. -->",
            f"<!-- {reader_line} -->",
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
                # "spec/src/hotam_spec/foo.py::bar" -> show as "spec/src/hotam_spec/foo.py::bar"
                lines.append(f"## From `{node_label}`")
            lines.append("")
            lines.append(doc.rstrip())
            lines.append("")
        result[slug] = "\n".join(lines).rstrip() + "\n"

    _SHARED_THINKING_DOCS_CACHE = result
    return result


def _write_shared_thinking_docs(
    src_dir: Path | None = None,
    out_dir: Path | None = None,
    reader_stakeholder_ids: frozenset[str] | None = None,
) -> list[Path]:
    """Write spec/docs/thinking/<topic-slug>.md files. Returns list of written paths."""
    _out = out_dir or SPEC_DOCS_THINKING_DIR
    _out.mkdir(parents=True, exist_ok=True)
    docs = build_shared_thinking_docs(src_dir, reader_stakeholder_ids)
    written: list[Path] = []
    for slug, content in sorted(docs.items()):
        path = _out / f"{slug}.md"
        _write(path, content)
        written.append(path)
    return written


# ---------------------------------------------------------------------------
# Concern 5b: Shared tool docs (spec/docs/tools/<basename>.md)
# ---------------------------------------------------------------------------


def _capture_tool_help(path: Path) -> str:
    """Canon: §Generator — render a tool's `--help` text in-process (no subprocess).

    RULE: imports the tool module by file path (importlib, mirroring the other
    dynamic-import call sites in this file) and invokes its `main`, with stdout
    redirected and `sys.argv` temporarily set to `[path, "--help"]` so argparse's
    `prog` (derived from `os.path.basename(sys.argv[0])`) renders identically to
    a real `python <tool> --help` subprocess invocation. Two `main` calling
    conventions exist across tools/*.py: `main(argv: list[str] | None)` (most
    tools) and `main()` with no params that reads `sys.argv[1:]` internally
    (emit_cipher.py, claude_md_diff_watch.py) — both are tried. gen_spec.py
    itself is special-cased to call its OWN already-executing `main` (module
    identity `__main__`/this module) rather than re-importing the file under a
    synthetic module name, because re-importing this file's `@dataclass`
    definitions under a second module name breaks `dataclasses._is_type`
    (`cls.__module__` no longer resolves via `sys.modules`) — a self-import
    hazard specific to modules using stdlib `dataclasses`. argparse exits via
    SystemExit after printing help; that is caught and swallowed.
    WHY in-process instead of subprocess.run: each subprocess spawn costs
    ~2.7-3.4s on Windows (measured) purely from interpreter startup + package
    import, and gen_spec.py invokes this once per Canon-documented tool (15 as
    of this writing) on EVERY run — spawning is the dominant regen-time cost.
    In-process import pays that cost once per gen_spec.py run (already paid,
    since gen_spec.py itself is one such interpreter), not once per tool.
    Returns "" if the module has no argparse-based `main` or import fails
    (mirrors the old subprocess try/except-swallow behavior byte-for-byte).

    WHY COLUMNS/LINES are pinned here: argparse.HelpFormatter wraps its output
    to the terminal width via shutil.get_terminal_size(), which reads the
    COLUMNS/LINES env vars first and falls back to the real terminal/console
    size (verified: `COLUMNS=80` vs `COLUMNS=140` produce different wrapped
    help text). An unpinned width makes the generated spec/docs/tools/*.md
    byte-identical only by accident of whatever terminal happened to invoke
    gen_spec.py — breaking R-deterministic-generation across environments
    (a narrow CI pty vs. a wide interactive terminal regenerate different
    bytes for the exact same tool). Pinning COLUMNS=80 (and LINES, for
    formatters that also consult height) makes the captured help text --
    and therefore the generated doc -- independent of the invoking terminal.
    """
    import contextlib
    import importlib.util
    import io
    import os

    mod_name: str | None = None
    old_columns = os.environ.get("COLUMNS")
    old_lines = os.environ.get("LINES")
    os.environ["COLUMNS"] = "80"
    os.environ["LINES"] = "24"
    try:
        if path.resolve() == Path(__file__).resolve():
            module = sys.modules[__name__]
        else:
            mod_name = f"_gen_spec_help_probe_{path.stem}"
            spec = importlib.util.spec_from_file_location(mod_name, path)
            if spec is None or spec.loader is None:
                return ""
            module = importlib.util.module_from_spec(spec)
            # Register in sys.modules BEFORE exec: modules using stdlib
            # `dataclasses` resolve `cls.__module__` back through sys.modules
            # at class-definition time (dataclasses._is_type); an unregistered
            # module makes that lookup return None and raise AttributeError.
            sys.modules[mod_name] = module
            spec.loader.exec_module(module)

        main_fn = getattr(module, "main", None)
        if main_fn is None:
            return ""

        buf = io.StringIO()
        old_argv = list(sys.argv)
        try:
            sys.argv = [str(path), "--help"]
            with contextlib.redirect_stdout(buf), contextlib.redirect_stderr(buf):
                try:
                    main_fn(["--help"])
                except TypeError:
                    # main() takes no argv param (reads sys.argv[1:] itself).
                    try:
                        main_fn()
                    except SystemExit:
                        pass
                    except Exception:
                        pass
                except SystemExit:
                    pass
                except Exception:
                    pass
        finally:
            sys.argv = old_argv

        return buf.getvalue()
    except Exception:
        return ""
    finally:
        if mod_name is not None:
            sys.modules.pop(mod_name, None)
        if old_columns is None:
            os.environ.pop("COLUMNS", None)
        else:
            os.environ["COLUMNS"] = old_columns
        if old_lines is None:
            os.environ.pop("LINES", None)
        else:
            os.environ["LINES"] = old_lines


def build_shared_tool_docs(
    tools_dir: Path | None = None,
    reader_stakeholder_ids: frozenset[str] | None = None,
) -> dict[str, str]:
    """Build content for spec/docs/tools/<basename>.md files.

    Only processes tools whose module docstring opens with Canon: §<topic> — <claim>.
    `reader_stakeholder_ids` follows the same contract as
    `build_shared_thinking_docs` (R-doc-names-reader).
    Returns dict mapping basename -> markdown_content.
    """
    global _SHARED_TOOL_DOCS_CACHE
    if _SHARED_TOOL_DOCS_CACHE is not None:
        return _SHARED_TOOL_DOCS_CACHE
    _tools = tools_dir or SPEC_TOOLS_DIR
    if reader_stakeholder_ids is None:
        try:
            reader_stakeholder_ids = stakeholder_ids(load_content_graph())
        except Exception:
            reader_stakeholder_ids = frozenset()
    reader_line = _doc_reader_line(
        "SHARED_TOOL", reader_stakeholder_ids, active_domain_doc_readers()
    )
    result: dict[str, str] = {}

    for path in sorted(_tools.glob("*.py")):
        if path.name.startswith("_"):
            continue
        try:
            tree = parse_source(path)
            doc = ast.get_docstring(tree) or ""
        except Exception:
            continue
        first_line = doc.split("\n")[0].strip() if doc else ""
        m = _CANON_RE.match(first_line)
        if not m:
            continue
        basename = path.stem
        canon_line = first_line

        # Try to get --help output (in-process; see _capture_tool_help).
        cli_section = ""
        try:
            help_out = _capture_tool_help(path)
            if help_out.strip():
                cli_section = "\n## CLI usage\n\n```\n" + help_out.rstrip() + "\n```\n"
        except Exception:
            pass  # Gracefully skip if tool has no argparse or errors

        lines: list[str] = [
            "<!-- Auto-generated by spec/tools/gen_spec.py. Do not hand-edit. -->",
            f"<!-- {reader_line} -->",
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

    _SHARED_TOOL_DOCS_CACHE = result
    return result


def _write_shared_tool_docs(
    tools_dir: Path | None = None,
    out_dir: Path | None = None,
    reader_stakeholder_ids: frozenset[str] | None = None,
) -> list[Path]:
    """Write spec/docs/tools/<basename>.md files. Returns list of written paths."""
    _out = out_dir or SPEC_DOCS_TOOLS_DIR
    _out.mkdir(parents=True, exist_ok=True)
    docs = build_shared_tool_docs(tools_dir, reader_stakeholder_ids)
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
        new_text = _claude_md_replace_block(text, "SHARED-DOCS", new_block)
    else:
        # Insert after AGENT-MAP:END if present, else after CONSTITUTION:END, else append.
        inserted = False
        for anchor in ("AGENT-MAP", "CONSTITUTION"):
            if _claude_md_end_sentinel(anchor) in text:
                new_text = _claude_md_insert_block_after(text, anchor, "SHARED-DOCS", new_block)
                inserted = True
                break
        if not inserted:
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
    three-line entry mapping: defined (which hotam_spec/*.py file has the module-level
    Canon: §Topic), enforced (check_* functions in invariants.py whose docstring
    mentions §Topic), and tested (test files that reference §Topic or those checks).

    Deterministic: §-topics sorted alphabetically by slug.
    """
    _src = src_dir or SRC
    _tests = tests_dir or SPEC_TESTS_DIR

    # --- Build: topic -> defining file (module-level Canon: §Topic) ---
    # When multiple files declare the same §Topic (e.g. requirement.py,
    # doc_readers.py, text.py all open with Canon: §Requirement), prefer the
    # file whose stem matches the topic slug (lowercase). This ensures
    # §Requirement -> requirement.py, not text.py (an incidental mention).
    _FIRST_SECTION_RE = re.compile(r"^Canon:\s+§(\w+)")
    topic_to_file: dict[str, str] = {}
    for py_file in sorted(_src.glob("*.py")):
        if py_file.name.startswith("_"):
            continue
        try:
            tree = parse_source(py_file)
            doc = ast.get_docstring(tree) or ""
        except Exception:
            continue
        first_line = doc.split("\n")[0].strip() if doc else ""
        m = _FIRST_SECTION_RE.match(first_line)
        if m:
            topic = m.group(1)
            rel_path = f"spec/src/hotam_spec/{py_file.name}"
            # Prefer canonical file (stem matches topic slug).
            if topic not in topic_to_file or py_file.stem == topic.lower():
                topic_to_file[topic] = rel_path

    # --- Build: topic -> list of check_* function names ---
    _CANON_SECTION_RE = re.compile(r"§(\w+)")
    topic_to_checks: dict[str, list[str]] = {}
    inv_py = _src / "invariants.py"
    if inv_py.exists():
        try:
            inv_tree = parse_source(inv_py)
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
                test_src = read_source(test_file)
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
        "| § | defined | enforced | tested |",
        "|---|---------|----------|--------|",
    ]

    for slug in section_slugs:
        topic = slug.lstrip("§")

        # defined — prefer canonical file named after slug
        def_file = topic_to_file.get(topic, "")
        if def_file:
            def_cell = f"`{def_file}`"
        else:
            def_cell = "_(not yet mapped)_"

        # enforced
        checks = topic_to_checks.get(topic, [])
        if checks:
            enf_cell = f"{len(checks)} checks"
        else:
            enf_cell = "_(none)_"

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
            test_cell = f"{len(tested)} tests"
        else:
            test_cell = "_(none)_"

        lines.append(f"| **{slug}** | {def_cell} | {enf_cell} | {test_cell} |")

    return "\n".join(lines).rstrip()


# ---------------------------------------------------------------------------
# Concern 4: DOMAIN-MAP block in root CLAUDE.md
# ---------------------------------------------------------------------------


def _domain_pulse(dg: TensionGraph) -> tuple[int, str]:
    """Canon: §Harness — one domain's open-action count + top-action one-liner.

    Runs what_now.diagnose(dg) against a SPECIFIC domain's graph and returns
    (n_open_actions, top_action_line). The top line is the highest-priority
    action's "[P{n}] {target}: {imperative}" rendered as a single line
    (imperative truncated). Returns (0, "") on a clean graph or any error, so a
    diagnosis failure can never break DOMAIN-MAP generation.

    WHY per-domain here (the two-eyed pulse, R-domain-map-shows-pulse): the
    root LIVE-STATE cipher only diagnoses the PINNED self-host domain, so a
    business domain's DETECTED conflict (e.g. hotam-dev C-ec1ec532) was
    INVISIBLE from the root crystal — the banner "every contradiction is
    visible" was false at the altitude of the repo. Surfacing each domain's
    open-action count in DOMAIN-MAP restores that guarantee.
    """
    try:
        import what_now as _what_now  # noqa: PLC0415

        actions = _what_now.diagnose(dg)
    except Exception:
        return (0, "")
    if not actions:
        return (0, "")
    top = actions[0]
    imperative = top.imperative
    # Collapse whitespace; keep the line compact for a crystal-resident index.
    imperative = " ".join(imperative.split())
    if len(imperative) > 140:
        imperative = imperative[:137] + "..."
    return (len(actions), f"[P{top.priority}] {top.target}: {imperative}")


# Intra-process memo for block renderers called 2+ times per run (once
# from build_live_state for the approx_size calculation, once from
# render_mind_content or render_business_content for the actual content —
# same output both times). Keyed by function/block name.
_BLOCK_MEMO: dict[str, str] = {}

# Legacy alias kept for the _render_domain_map_block early returns.
_DOMAIN_MAP_BLOCK_MEMO: str | None = None


def _memo_block(key: str, fn):
    """Return cached result for `key`, or compute via fn(), cache, and return."""
    cached = _BLOCK_MEMO.get(key)
    if cached is not None:
        return cached
    result = fn()
    _BLOCK_MEMO[key] = result
    return result


def _render_domain_map_block(g: TensionGraph | None = None) -> str:  # noqa: ARG001
    """Render the DOMAIN-MAP block content (without sentinels).

    When domains/ is empty, emits a placeholder. When domains/ has content,
    lists each domain: ID, purpose, goals, director, path, atoms-count.
    Memoized: called twice per run (build_live_state size + render_business_content).
    """
    global _DOMAIN_MAP_BLOCK_MEMO
    if _DOMAIN_MAP_BLOCK_MEMO is not None:
        return _DOMAIN_MAP_BLOCK_MEMO
    lines: list[str] = [
        "<!-- (generated by tools/gen_spec.py — do not hand-edit) -->",
        "",
        "### Domain Map",
        "",
    ]

    if not DOMAINS_ROOT.exists():
        lines.append("_(no domains yet — domains/ directory absent)_")
        _DOMAIN_MAP_BLOCK_MEMO = "\n".join(lines).rstrip()
        return _DOMAIN_MAP_BLOCK_MEMO

    domain_dirs = sorted(
        d for d in DOMAINS_ROOT.iterdir() if d.is_dir() and not d.name.startswith("_")
    )
    if not domain_dirs:
        lines.append("_(no domains yet)_")
        _DOMAIN_MAP_BLOCK_MEMO = "\n".join(lines).rstrip()
        return _DOMAIN_MAP_BLOCK_MEMO

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

        # Try to load graph and count atoms + diagnose open actions (the
        # two-eyed pulse: R-domain-map-shows-pulse). Every domain's open
        # actions become visible from the root crystal, so "every contradiction
        # is visible" holds at the altitude of the whole repo — not only for
        # the pinned self-host domain.
        open_actions = 0
        top_action_line = ""
        dg = _load_domain_graph(domain_dir)
        if dg is not None:
            try:
                from hotam_spec.requirement import SETTLED as _SETTLED  # noqa: PLC0415

                atoms_count = sum(
                    1 for r in dg.requirements if r.status == _SETTLED
                )
                open_actions, top_action_line = _domain_pulse(dg)
            except Exception:
                pass

        lines.append(f"### {domain_id}")
        lines.append(f"- **purpose** — {description or '—'}")
        lines.append(f"- **goals** — {goals_text or '—'}")
        lines.append(f"- **director** — {director or '—'}")
        lines.append(f"- **path** — `domains/{domain_dir.name}/`")
        lines.append(f"- **atoms-count** — {atoms_count} SETTLED")
        if open_actions > 0:
            lines.append(
                f"- **open actions** — {open_actions} (top: {top_action_line})"
            )
        else:
            lines.append("- **open actions** — 0 (graph clean)")
        lines.append("")

    _DOMAIN_MAP_BLOCK_MEMO = "\n".join(lines).rstrip()
    return _DOMAIN_MAP_BLOCK_MEMO


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


# ---------------------------------------------------------------------------
# EMBEDDED-THINKING / EMBEDDED-TOOLS blocks — full-content embedding in root
# CLAUDE.md (P22.C consolidation: one operator, one CLAUDE.md, no indirection
# through a separate domain-file that used to be wholesale-embedded).
# ---------------------------------------------------------------------------


# ---------------------------------------------------------------------------
# RECENTLY-REJECTED block — anti-relitigation surface in root CLAUDE.md
# ---------------------------------------------------------------------------


_RECENTLY_REJECTED_CAP = 12

# Regex that matches the "REJECTED <dash> REPLACES" marker regardless of
# which dash variant the graph.py author used: em-dash (U+2014 '—'),
# en-dash (U+2013 '–'), double-dash ('--'), or single hyphen ('-').
# Single source of truth — used by the RECENTLY-REJECTED renderer and any
# other consumer that needs to detect the anti-relitigation marker.
_REJECTED_REPLACES_RE = re.compile(r"REJECTED\s*(?:—|–|--|-)\s*REPLACES")


def _has_rejected_replaces_marker(why: str) -> bool:
    """Return True if ``why`` contains a 'REJECTED <dash> REPLACES' marker.

    Normalizes across em-dash, en-dash, double-dash and single-dash variants
    so that no REJECTED requirement silently falls out of the anti-relitigation
    surface due to inconsistent punctuation in graph.py.
    """
    return _REJECTED_REPLACES_RE.search(why) is not None


def _render_recently_rejected_block(g: TensionGraph) -> str:
    """Render the RECENTLY-REJECTED block content (without sentinels).

    Lists REJECTED requirements that have a known replacement, capped at
    _RECENTLY_REJECTED_CAP entries to keep the resident (paid) crystal from
    growing monotonically as rejections accumulate — the full roster has no
    dates to rank by "recency" honestly, so the cap is applied to the same
    deterministic alphabetical-by-id order the block has always used (id order
    is stable and reproducible, not a claim of true recency). A pointer line
    directs the reader to the full history for anything beyond the cap
    (docs/gen/HISTORY.md carries every REJECTED requirement, capped or not).

    SOURCE OF TRUTH (R-rejected-preserved-not-deleted): a REJECTED requirement
    is listed when EITHER (a) it is the target of a structural `replaces` edge
    (graph.replaces_map — the machine-traversable twin), OR (b) its `why`
    contains the prose 'REJECTED <dash> REPLACES' marker (the legacy text form).
    The structural edge is the PRIMARY source; the prose marker is the FALLBACK
    for the ~38 historical REJECTED nodes not yet migrated to structural edges.
    When a structural edge exists, the successor id(s) are read from the edge;
    when only the prose marker exists, the "REPLACES <X>" substring is parsed
    from `why` as before. Both paths feed the same entry renderer.
    """
    lines: list[str] = [
        "<!-- (generated by tools/gen_spec.py — do not hand-edit) -->",
        "",
        "### Recently rejected (anti-relitigation)",
        "",
        "Before proposing an architectural change, scan this list. A claim already "
        "REJECTED with REPLACES SHOULD NOT be re-derived; cite the replacement instead.",
        "",
    ]

    # Structural replaces edges: REJECTED-id -> tuple of successor ids.
    rmap = replaces_map(g)

    # Membership: structural edge OR prose marker (the historical fallback).
    rejected_with_replaces = sorted(
        [
            r
            for r in g.requirements
            if r.status == "REJECTED"
            and (r.id in rmap or _has_rejected_replaces_marker(r.why))
        ],
        key=lambda r: r.id,
    )

    if not rejected_with_replaces:
        lines.append("_(no anti-relitigation entries — nothing recently rejected.)_")
        return "\n".join(lines).rstrip()

    total = len(rejected_with_replaces)
    shown = rejected_with_replaces[:_RECENTLY_REJECTED_CAP]

    _TRUNC = 220
    for r in shown:
        # Prefer the structural edge (machine-traversable) as the source of the
        # successor id(s); fall back to parsing the prose "REPLACES <X>" substring
        # when no structural edge exists (historical REJECTED without migration).
        if r.id in rmap:
            succ = ", ".join(rmap[r.id])
            summary = f"REPLACES by {succ}"
            # Append the rationale context from the prose marker if present, so
            # the entry stays as informative as the prose-only path was. Extract
            # the tail AFTER the REPLACES marker (the split rationale).
            if "REPLACES" in r.why:
                replaces_start = r.why.index("REPLACES")
                replaces_text = r.why[replaces_start:]
                # Find the rationale separator and keep what follows it.
                for sep in [" — (", " (was:", "\n"]:
                    p = replaces_text.find(sep)
                    if p != -1:
                        tail = replaces_text[p:].strip()
                        summary = f"{summary} {tail}"
                        break
        elif "REPLACES" in r.why:
            replaces_start = r.why.index("REPLACES")
            replaces_text = r.why[replaces_start:]
            end_pos = len(replaces_text)
            for sep in [" — (", " (was:", "\n"]:
                p = replaces_text.find(sep)
                if p != -1 and p < end_pos:
                    end_pos = p
            summary = replaces_text[:end_pos].strip()
        else:
            summary = r.why
        summary = summary.replace("\n", " ").strip()
        if len(summary) > _TRUNC:
            summary = summary[:_TRUNC].rstrip() + "…"
        lines.append(f"- **{r.id}** (REJECTED) — {summary}")

    if total > _RECENTLY_REJECTED_CAP:
        lines.append("")
        lines.append(
            f"_(showing {len(shown)} of {total}, alphabetical by id — "
            "full history: docs/gen/HISTORY.md)_"
        )

    return "\n".join(lines).rstrip()


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


# Intra-process cache: domain graph objects keyed by domain dir name.
# Each domain's graph is built at most ONCE per gen_spec run and shared
# across _render_domain_map_block, _process_domains, and build_live_state
# (the dedup fix for repeated build_graph + diagnose calls).
_DOMAIN_GRAPH_CACHE: dict[str, "TensionGraph | None"] = {}


def _load_domain_graph(domain_dir: Path) -> "TensionGraph | None":
    """Load graph.py:build_graph() from a domain dir (cached per process)."""
    key = domain_dir.name
    if key in _DOMAIN_GRAPH_CACHE:
        return _DOMAIN_GRAPH_CACHE[key]
    graph_py = domain_dir / "graph.py"
    if not graph_py.exists():
        _DOMAIN_GRAPH_CACHE[key] = None
        return None
    spec = importlib.util.spec_from_file_location(
        f"_domain_graph_iter_{domain_dir.name}", graph_py
    )
    if spec is None or spec.loader is None:
        _DOMAIN_GRAPH_CACHE[key] = None
        return None
    mod = importlib.util.module_from_spec(spec)
    try:
        spec.loader.exec_module(mod)  # type: ignore[union-attr]
        result = mod.build_graph()
    except Exception:
        result = None
    _DOMAIN_GRAPH_CACHE[key] = result
    return result


def _domain_dirty_state_file() -> Path:
    """Return the path to the persistent domain dirty-tracking index.

    Canon: §Generator — wave 6.3 Part 3. Stores per-domain graph.py+manifest.py
    mtime so that _process_domains can SKIP regenerating docs/gen/<name>/ when
    nothing in that domain's substrate has changed since the last gen_spec run.
    """
    from hotam_spec.repo_paths import runtime_root as _runtime_root

    return _runtime_root() / "gen-domain-mtime.json"


def _domain_fingerprint(domain_dir: Path) -> tuple[int, int] | None:
    """Return (graph_mtime_ns, manifest_mtime_ns) for a domain, or None if no graph.py.

    Canon: §Generator — wave 6.3 Part 3. The fingerprint captures the two files
    whose change can alter docs/gen/<name>/ output: graph.py (the node data) and
    manifest.py (the reader-stakeholder binding). If either changed mtime, the
    domain is dirty and must be regenerated; if both are unchanged, the docs are
    byte-identical to the last gen and can be skipped (the generator is pure and
    deterministic — same inputs → same outputs).
    """
    graph_py = domain_dir / "graph.py"
    manifest_py = domain_dir / "manifest.py"
    if not graph_py.exists():
        return None
    g_mtime = int(graph_py.stat().st_mtime_ns)
    m_mtime = int(manifest_py.stat().st_mtime_ns) if manifest_py.exists() else 0
    return (g_mtime, m_mtime)


def _load_domain_dirty_index() -> dict[str, list[int]]:
    """Load the persistent dirty-tracking index (best-effort)."""
    index_file = _domain_dirty_state_file()
    if not index_file.exists():
        return {}
    try:
        data = json.loads(index_file.read_text(encoding="utf-8"))
        if isinstance(data, dict):
            return {str(k): list(v) for k, v in data.items() if isinstance(v, list)}
    except (OSError, ValueError):
        pass
    return {}


def _save_domain_dirty_index(index: dict[str, list[int]]) -> None:
    """Persist the dirty-tracking index (best-effort, never raises)."""
    index_file = _domain_dirty_state_file()
    try:
        index_file.parent.mkdir(parents=True, exist_ok=True)
        index_file.write_text(
            json.dumps(index, sort_keys=True, indent=2, ensure_ascii=False),
            encoding="utf-8",
        )
    except OSError:
        pass


def _process_domains(g: TensionGraph) -> None:
    """Walk domains/*/ and generate per-domain docs. No-op when domains/ is empty.

    Wave 6.3 Part 3 (dirty-domain skip): a domain whose graph.py+manifest.py
    mtimes are unchanged since the last gen_spec run is SKIPPED — its docs/gen/
    are already byte-identical (the generator is pure + deterministic). The
    ACTIVE domain is ALWAYS regenerated regardless of mtime: it is the "live
    exit" the operator reads, and the root CLAUDE.md render (in main()) already
    depends on it. Skipping a non-active domain is safe because its docs/gen/
    feed only background reference, not the live operator crystal.

    Determinism guarantee: dirty-skip produces the SAME on-disk result as full
    regen — it just avoids re-writing identical bytes. test_generator_is_deterministic
    (gen_spec twice → diff empty) holds because the second run sees unchanged
    mtimes and skips, leaving the first run's (already correct) output untouched.
    """
    if not DOMAINS_ROOT.exists():
        return
    domain_dirs = sorted(
        d for d in DOMAINS_ROOT.iterdir() if d.is_dir() and not d.name.startswith("_")
    )
    if not domain_dirs:
        return

    active_dir = _active_domain()
    dirty_index = _load_domain_dirty_index()
    new_index: dict[str, list[int]] = {}
    wrote_any = False

    for domain_dir in domain_dirs:
        manifest = _load_domain_manifest(domain_dir)
        if not manifest:
            print(f"skipping domain {domain_dir.name}: no valid manifest.py")
            continue

        fp = _domain_fingerprint(domain_dir)
        is_active = active_dir is not None and domain_dir.name == active_dir.name
        # Record the current fingerprint regardless of skip decisions.
        if fp is not None:
            new_index[domain_dir.name] = list(fp)

        # Dirty check: skip regen if (a) not the active domain, (b) fingerprint
        # matches the last recorded value, and (c) docs/gen/ already exists.
        gen_dir = domain_dir / "docs" / "gen"
        if (
            not is_active
            and fp is not None
            and gen_dir.exists()
            and dirty_index.get(domain_dir.name) == list(fp)
        ):
            print(f"skipping domain {domain_dir.name} (clean, mtime-unchanged)")
            continue

        dg = _load_domain_graph(domain_dir) or g  # Fallback to root graph if missing.

        gen_dir.mkdir(parents=True, exist_ok=True)

        # Resolve the `reader:` header from THIS domain's manifest, not from the
        # env-active domain (R-root-crystal-follows-pin): otherwise landing a
        # non-pinned-domain proposal rewrote every other domain's reader header
        # with the transient env domain's binding (or the unresolved sentinel).
        with _doc_readers_override(domain_doc_readers(domain_dir)):
            domain_targets = {
                gen_dir / "REQUIREMENTS.md": build_requirements(dg),
                gen_dir / "TENSIONS.md": build_tensions(dg),
                gen_dir / "OPEN.md": build_open(dg),
                gen_dir / "UNENFORCED.md": build_unenforced(dg),
                gen_dir / "GLOSSARY.md": build_glossary(dg),
                gen_dir / "HISTORY.md": build_history(dg),
                gen_dir / "CONSTITUTION.md": build_constitution(dg),
                gen_dir / "FRAMEWORK-INVARIANTS.md": build_framework_invariants(
                    dg, domain_name=domain_dir.name
                ),
                gen_dir
                / "REPO-MAP.md": build_repo_map_md(
                    dg, content_dir=domain_dir, gen_dir=gen_dir
                ),
                gen_dir / "graph.json": build_graph_json(dg),
            }
            # Conditional materialization (task #106 / L2-#6): DECISIONS.md /
            # ENTITIES.md are projections of possibly-empty registries (open
            # M-tagged requirements / declared EntityTypes). Writing them
            # unconditionally produces a permanent placeholder file that never
            # carries information once the registry is genuinely empty; write
            # them only when there is real content to project. An existing
            # file from before this aspect activated is left untouched here
            # (no auto-delete on every run — only when the aspect activates
            # does the file appear).
            if _decisions_md_has_content(dg):
                domain_targets[gen_dir / "DECISIONS.md"] = build_decisions(dg)
            if _entities_md_has_content(dg):
                domain_targets[gen_dir / "ENTITIES.md"] = build_entities_md(dg, domain_dir.name)
        for path, text in domain_targets.items():
            _write(path, text)
            print(f"written (domain {domain_dir.name}): {path}")
        wrote_any = True

    # Persist the updated dirty index (even if nothing was written, so the
    # fingerprints are current for the next run).
    if wrote_any or not new_index:
        _save_domain_dirty_index(new_index)
    else:
        _save_domain_dirty_index(new_index)


# ---------------------------------------------------------------------------
# CONCEPT-MAP block in root CLAUDE.md (P22.C: rendered straight into root,
# no more indirection through a separate domain CLAUDE.md file).
# ---------------------------------------------------------------------------


# ---------------------------------------------------------------------------
# Template-driven root CLAUDE.md generation (R-claude-md-template-driven)
#
# Replaces the ~10-sentinel surgical-splice-into-existing-file model with a
# two-placeholder substitution model: CLAUDE.md.template.txt is the
# human-editable source (a fixed header + exactly two placeholder lines).
# render_claude_md_from_template() substitutes <!-- mind --> with the
# domain-agnostic methodology layer (REPO-MAP + THINKING-INDEX +
# EMBEDDED-THINKING + EMBEDDED-TOOLS) and <!-- business --> with the
# domain-specific claims layer (LIVE-STATE + DOMAIN-MAP + CONSTITUTION +
# AGENT-MAP + CONCEPT-MAP + RECENTLY-REJECTED). Every individual block still
# emits its own internal sentinel-comment pair; MIND/BUSINESS are just
# concatenations of those (still-wrapped) blocks in the new grouping/order.
# Anything else in the template -- including hand-written notes below the
# placeholders -- survives every regeneration verbatim.
# ---------------------------------------------------------------------------


def _wrap(begin: str, end: str, content: str) -> str:
    """Wrap block content in its sentinel-comment pair. One atom."""
    return f"{begin}\n{content}\n{end}"


# Canon: §Operator — the CORE-11 thinking topics that constitute the operator's
# invariant reasoning lens (contradiction-as-node, well-formed-not-conflict-free,
# lifecycle keystone, drift, hard boundary, self-budgeting) — the set an operator
# needs to correctly read ANY domain, as opposed to reference material or opt-in
# aspects a given domain may not even instantiate. Established by oxx-research
# (Task #1) and accepted by the steward as the Tier-1 distillation boundary
# (R-crystal-is-tiered). CORE topics get up to 3 RULE+WHY pairs distilled;
# all other topics get 1 (still real content, not a bare heading).
_TIER1_CORE_TOPICS = (
    "conflict",
    "invariants",
    "lifecycle",
    "graph",
    "operator",
    "axis",
    "assumption",
    "stakeholder",
    "proposal",
    "requirement",
    "conscience",
)

_TIER1_MAX_LEN = 400  # per RULE or WHY fragment, hard truncation with an ellipsis


def _extract_rule_and_why(doc: str) -> tuple[str, str]:
    """Canon: §Requirement — pull the RULE and WHY paragraphs out of a docstring.

    RULE: distillation MUST rest on the same RULE:/WHY: convention already used
    throughout spec/src/hotam_spec/*.py docstrings (the same convention
    check_method_matches_docstring already assumes exists). Paragraphs are
    split on blank lines; the first paragraph starting with 'RULE' and the
    first starting with 'WHY' are taken verbatim (whitespace-collapsed) and
    hard-truncated at _TIER1_MAX_LEN chars. A docstring with no RULE label
    (e.g. a bare module docstring opening with 'Canon: §X — <claim>') falls
    back to its first 'Canon:' line as a pseudo-rule.

    WHY a paragraph split, not a smarter parser: the docstrings are already
    prose written for a human, not for machine extraction; the reliable
    signal is the RULE:/WHY: label a human already put at a paragraph start.
    A heavier parser would be guessing at structure that isn't there.
    """
    paragraphs = [p.strip() for p in doc.split("\n\n") if p.strip()]
    rule = ""
    why = ""
    for p in paragraphs:
        if not rule and p.startswith("RULE"):
            rule = p
        if not why and p.startswith("WHY"):
            why = p
        if rule and why:
            break
    if not rule:
        for p in paragraphs:
            if p.startswith("Canon:"):
                rule = p.split("\n", 1)[0]
                break

    def _trim(s: str) -> str:
        s = " ".join(s.split())
        return s if len(s) <= _TIER1_MAX_LEN else s[:_TIER1_MAX_LEN].rstrip() + "…"

    return _trim(rule), _trim(why)


def _distill_thinking_doc(body: str, max_sections: int) -> str:
    """Canon: §Requirement — compact RULE+WHY distillate of one thinking topic.

    RULE: reads at most `max_sections` of the topic's '## From `...`'
    docstring sections (as already produced by build_shared_thinking_docs)
    and keeps only their RULE+WHY pairs, dropping the rest of the prose.
    CORE-11 topics (see _TIER1_CORE_TOPICS) get more sections since they
    carry the operator's reasoning lens; other topics get one.
    """
    sections = re.split(r"(?m)^## From ", body)[1:]
    pairs: list[str] = []
    for sec in sections[:max_sections]:
        _, _, rest = sec.partition("\n")
        rule, why = _extract_rule_and_why(rest)
        piece = " ".join(part for part in (rule, why) if part)
        if piece:
            pairs.append(piece)
    return " ".join(pairs).strip()


def _distill_tool_doc(body: str) -> str:
    """Canon: §Requirement — compact RULE+WHY distillate of one tool doc.

    RULE: reads only the '## Module docstring' section (skips '## CLI usage',
    which is a raw --help transcript with no RULE/WHY markers and would only
    add noise to the distillate).
    """
    m = re.search(r"(?ms)^## Module docstring\n\n(.*?)(?:\n## |\Z)", body)
    content = m.group(1) if m else body
    rule, why = _extract_rule_and_why(content)
    return " ".join(part for part in (rule, why) if part)


def _render_embedded_thinking_block() -> str:
    """Render the EMBEDDED-THINKING block: one RULE sentence per topic + path link.

    Content is built directly from framework docstrings (build_shared_thinking_docs),
    not read back from disk, so this stays deterministic even before the shared
    thinking docs have been (re)written in this run. Each topic is a single line:
    slug + first RULE sentence via short_form + path link to full text
    (R-crystal-reload-by-reference, R-crystal-carries-short-form).
    """
    docs = build_shared_thinking_docs()
    lines: list[str] = [
        "<!-- (generated by tools/gen_spec.py — do not hand-edit) -->",
        "",
        "### Methodology — how to think",
        "",
    ]
    if not docs:
        lines.append("_(no thinking docs yet)_")
        return "\n".join(lines).rstrip()
    for slug in sorted(docs):
        # Extract first RULE sentence from the topic's docstring content.
        body = docs[slug]
        rule_sentence = ""
        # Look for RULE paragraph in sections.
        sections = re.split(r"(?m)^## From ", body)[1:]
        for sec in sections[:1]:
            _, _, rest = sec.partition("\n")
            paragraphs = [p.strip() for p in rest.split("\n\n") if p.strip()]
            for p in paragraphs:
                if p.startswith("RULE"):
                    rule_sentence = short_form(p)
                    break
                if not rule_sentence and p.startswith("Canon:"):
                    rule_sentence = short_form(p.split("\n", 1)[0])
            if rule_sentence:
                break
        if not rule_sentence:
            rule_sentence = f"Canon: §{slug.capitalize()}"
        lines.append(f"- [§{slug.capitalize()}](spec/docs/thinking/{slug}.md) — {rule_sentence}")
    return "\n".join(lines).rstrip()


def _render_embedded_tools_block() -> str:
    """Render the EMBEDDED-TOOLS block: one Canon sentence per tool + path link.

    Content is built directly from tool docstrings (build_shared_tool_docs), for
    the same determinism reason as _render_embedded_thinking_block(). Each tool
    is a single line: name + Canon sentence via short_form + path link
    (R-crystal-reload-by-reference, R-crystal-carries-short-form).
    """
    docs = build_shared_tool_docs()
    lines: list[str] = [
        "<!-- (generated by tools/gen_spec.py — do not hand-edit) -->",
        "",
        "### Tool reference (full text at spec/docs/tools/*.md)",
        "",
    ]
    if not docs:
        lines.append("_(no tool docs yet)_")
        return "\n".join(lines).rstrip()
    for basename in sorted(docs):
        body = docs[basename]
        # Extract the Canon: line from the Synopsis section.
        canon_line = ""
        m = re.search(r"(?m)^Canon:\s+§\S+\s+[—\-]\s+(.+)$", body)
        if m:
            canon_line = short_form(m.group(1).strip())
        if not canon_line:
            canon_line = f"Canon: §{basename}"
        lines.append(f"- [{basename}](spec/docs/tools/{basename}.md) — {canon_line}")
    return "\n".join(lines).rstrip()


def _render_operator_role_block(
    g: TensionGraph,
    *,
    scope_label: str = "",
    atom_count: int | None = None,
) -> str:
    """Canon: §Requirement — R-crystal-carries-role-seed: the resident seed's identity block.

    RULE: pure function of (g, active-domain filesystem state) — same
    determinism contract as _render_domain_map_block. scope_label defaults
    to the active domain name ("(no domain yet)" if none); atom_count
    defaults to the count of SETTLED requirements in g. Parameterized so the
    same function renders a narrower seed for a future real sub-operator
    (R-sub-agent-crystal-triad) without a second implementation.

    WHY caveman-terse: this block is resident (reloaded every session), read
    by a model not a human — telegraphic prose with anchors and commands,
    not connective grammar (steward correction #1).
    """
    if not scope_label:
        domain = _active_domain()
        scope_label = domain.name if domain else "(no domain yet)"
    if atom_count is None:
        atom_count = sum(1 for r in g.requirements if r.status == SETTLED)
    lines = [
        "<!-- (generated by tools/gen_spec.py — do not hand-edit) -->",
        "",
        "### Role (the resident seed)",
        "",
        f"Operator of `{scope_label}` ({atom_count} SETTLED). Guardian: "
        f"**spec** (`domains/{scope_label}/graph.py`) ↔ **tests** (`check_*`/`test_*`) "
        "↔ **business** (steward decisions). Drift between layers = top signal.",
        "",
        "Confront every input against graph reality BEFORE writing. Cite anchors "
        "(`R-…`/`C-…`/`A-…`/`OP-…`), never vibes (R-speak-by-reference). Present, "
        "never decide — steward decides; never close a Conflict silently "
        "(R-ai-presents-not-decides, R-decided-needs-human-signoff).",
        "",
        "**Generative law:** important-yet-invisible → typed anchored node under a "
        "named steward; tension held open as a Conflict node, never quietly "
        "extinguished (R-anchor-everything · R-conflict-is-connector-node · "
        "R-steward-distinct-from-owners). Every RULE below is a projection of this "
        "law.",
    ]
    return "\n".join(lines).rstrip()


_MEDIATION_LOOP_TEXT = """\
### Mediation loop (how ANY input is processed)

Every input — idea, request, bug, hypothesis — six steps. Commands run from `spec/`.
Read-only input (a question, an explanation, a status check) needs only ORIENT +
a direct anchor-cited answer — steps 2-6 are for graph/code changes
(R-agent-conduct-is-rules-not-tests: legalize actual practice, not aspirational rigor).

1. **ORIENT** — reload pulse: top action · debt · context (LIVE-STATE below;
   re-injected each turn by emit_cipher hook). Full list:
   `python tools/what_now.py` (R-boot-reload-three-facts, R-agent-never-lost).
2. **LOCATE** — find what input touches: Constitution index below for R-anchors,
   `docs/gen/TENSIONS.md` for conflict clusters, `docs/gen/REQUIREMENTS.md` for
   full claims + assumptions.
3. **CONFRONT** — check input vs reality: which SETTLED claims contradicted?
   which Assumptions rested on / killed? already rejected — scan
   RECENTLY-REJECTED below, cite replacement, don't re-derive (anti-relitigation).
   Tool: `python tools/confront.py "<claim>"`.
4. **TRANSLATE** — outcome → typed nodes: ProposedRequirement /
   ProposedConflictTransition / ProposedRejection / ProposedConflict /
   ProposedOperatorBudget / ProposedEntityType JSON
   (`spec/src/hotam_spec/proposal.py`), drafted under `spec/.runtime/proposals/`.
   Tension found → Conflict node with axis + context + steward, never a silent
   edit (R-no-hand-edit-graph, R-conflict-is-connector-node).
5. **PRESENT** — show steward the proposal + anchors: resolves what, contradicts
   what, replaces what. Steward decides (R-ai-presents-not-decides).
6. **LAND** — after approval:
   `python tools/apply_proposal.py [--batch] [--triggering-kind KIND] <file.json>`
   → regen (`gen_spec.py`) → tiered gate: `apply_proposal.py` defaults to T1
   (targeted enforcer subset via `tools/gate.py`); pass `--full` to force T2
   (`python -m pytest -q`, the full suite) — T2 is MANDATORY at wave/commit
   boundaries and is the automatic fallback whenever T1 selection fails
   closed → closure verifies triggering diagnosis gone; exit 2 = landed but
   did NOT advance (R-verify-closure-per-action).

Writing nothing = valid outcome ("contradicts R-x; rejected as R-y — cite R-z")."""


def _render_mediation_loop_block() -> str:
    """Canon: §Requirement — R-crystal-carries-mediation-loop: the six-step input loop.

    RULE: static constant _MEDIATION_LOOP_TEXT (no graph/filesystem
    dependency — deterministic by construction); wrapped with the standard
    generated-header comment for consistency with sibling blocks.
    """
    lines = [
        "<!-- (generated by tools/gen_spec.py — do not hand-edit) -->",
        "",
        _MEDIATION_LOOP_TEXT,
    ]
    return "\n".join(lines).rstrip()


def _render_operator_recursion_block(*, domain: str = "") -> str:
    """Canon: §Requirement — R-crystal-carries-recursion-seed: sub-operator spawning.

    RULE: pure function of an optional domain override (defaults to the
    active domain name, "hotam-spec-self" fallback if none). Describes
    recursion as a CAPABILITY — no sub-agent crystal files are materialized
    while R-claude-md-consolidates-when-single-agent holds.
    """
    if not domain:
        d = _active_domain()
        domain = d.name if d else "hotam-spec-self"
    lines = [
        "<!-- (generated by tools/gen_spec.py — do not hand-edit) -->",
        "",
        "### Recursion (spawning sub-operators)",
        "",
        "Sub-operator = THIS SAME seed, narrowed: same Role text + narrower scope "
        "line, same Mediation loop, thinking + constitution filtered by SCOPE "
        "prefixes (R-sub-agent-crystal-triad: scoped thinking + parent reference + "
        "scoped constitution).",
        "",
        "- One domain, zero active sub-agents → exactly ONE CLAUDE.md (this file). "
        "Sub-agent crystals materialize only at real spawn time "
        "(R-claude-md-consolidates-when-single-agent).",
        "- Spawn path (from `spec/`): "
        f"`python tools/create_agent.py <name> --scope R-<prefix>- --purpose \"…\" --parent domains/{domain}/agents` "
        "→ `python tools/gen_spec.py` (fills scoped blocks) → "
        "`python tools/spawn_agent.py <agent-path> --task \"…\" --stamp <iso8601>` "
        "(composes crystal+task; appends `spec/.runtime/spawn-log.jsonl`, "
        "R-task-spawn-log-runtime).",
        "- Delegate only when still over budget AFTER crystallizing "
        "(R-crystallize-before-split, R-context-bounded-delegation). Sub-operator "
        "returns CONCLUSIONS only — shared objects as explicit border, never raw "
        "context (R-delegation-conclusions-only).",
    ]
    return "\n".join(lines).rstrip()


def render_mind_content(g: TensionGraph) -> str:
    """Render the MIND bucket: the domain-agnostic 'how to think' layer.

    Order (Phase 2 crystal seed): OPERATOR-ROLE, MEDIATION-LOOP,
    EMBEDDED-THINKING, EMBEDDED-TOOLS, OPERATOR-RECURSION, THINKING-INDEX,
    REPO-MAP. The seed (role/loop/recursion) brackets the RULE+WHY
    distillates so the operator's identity and operating procedure are the
    first and near-last resident content, with the reference maps trailing.
    """
    parts = [
        _wrap(
            _OPERATOR_ROLE_BEGIN,
            _OPERATOR_ROLE_END,
            _memo_block("operator_role", lambda: _render_operator_role_block(g)),
        ),
        _wrap(
            _MEDIATION_LOOP_BEGIN,
            _MEDIATION_LOOP_END,
            _memo_block("mediation_loop", _render_mediation_loop_block),
        ),
        _wrap(
            _EMBEDDED_THINKING_BEGIN,
            _EMBEDDED_THINKING_END,
            _memo_block("embedded_thinking", _render_embedded_thinking_block),
        ),
        _wrap(
            _EMBEDDED_TOOLS_BEGIN,
            _EMBEDDED_TOOLS_END,
            _memo_block("embedded_tools", _render_embedded_tools_block),
        ),
        _wrap(
            _OPERATOR_RECURSION_BEGIN,
            _OPERATOR_RECURSION_END,
            _memo_block("operator_recursion", _render_operator_recursion_block),
        ),
    ]
    return "\n".join(parts)


def render_business_content(g: TensionGraph) -> str:
    """Render the BUSINESS bucket: the domain-specific 'what this claims' layer.

    Order: LIVE-STATE, DOMAIN-MAP, CONSTITUTION, AGENT-MAP, CONCEPT-MAP,
    RECENTLY-REJECTED.
    """
    parts = [
        _wrap(_LS_BEGIN, _LS_END, build_live_state(g)),
        _wrap(_DOMAIN_MAP_BEGIN, _DOMAIN_MAP_END, _render_domain_map_block(g)),
        _wrap(
            _CONST_BEGIN,
            _CONST_END,
            _memo_block("constitution", lambda: _render_constitution_block(g)),
        ),
        _wrap(
            _AGENT_MAP_BEGIN,
            _AGENT_MAP_END,
            _memo_block("agent_map", lambda: _scan_agent_map(g)),
        ),
        _wrap(
            _CONCEPT_MAP_BEGIN,
            _CONCEPT_MAP_END,
            _memo_block("concept_map", _scan_concept_map),
        ),
        _wrap(
            _RECENTLY_REJECTED_BEGIN,
            _RECENTLY_REJECTED_END,
            _memo_block("recently_rejected", lambda: _render_recently_rejected_block(g)),
        ),
    ]
    return "\n".join(parts)


def render_claude_md_from_template(g: TensionGraph) -> str:
    """Render root CLAUDE.md by substituting the two template placeholders.

    Reads the operator-crystal template via importlib.resources (packaged at
    hotam_spec/_templates/claude_md.template.txt), with an optional consumer
    override at project_root()/"CLAUDE.md.template.txt" (§3.4 portability W3).
    Replaces the literal lines '<!-- mind -->' and '<!-- business -->' with
    the rendered MIND and BUSINESS content respectively. Every other line
    of the template -- including any hand-written notes below the
    placeholders -- is preserved byte-for-byte.

    Raises FileNotFoundError with a helpful message if the template is
    missing: the template is the human-editable source of root CLAUDE.md,
    so a missing template is a real misconfiguration, not something to
    silently paper over with a hardcoded fallback.
    """
    template_path = _claude_md_template_path()
    if not template_path.exists():
        raise FileNotFoundError(
            f"CLAUDE.md.template.txt not found at {template_path}. "
            "This file is the human-editable source of root CLAUDE.md "
            "(R-claude-md-template-driven): a fixed header plus exactly two "
            "placeholder lines, '<!-- mind -->' and '<!-- business -->'. "
            "The packaged template (hotam_spec/_templates/claude_md.template.txt) "
            "or a consumer override (project_root/CLAUDE.md.template.txt) "
            "must be present."
        )
    template_text = _read_claude_md_template()
    mind_content = render_mind_content(g)
    business_content = render_business_content(g)

    lines = template_text.split("\n")
    out_lines: list[str] = []
    for line in lines:
        if line.strip() == _MIND_PLACEHOLDER:
            out_lines.append(mind_content)
        elif line.strip() == _BUSINESS_PLACEHOLDER:
            out_lines.append(business_content)
        else:
            out_lines.append(line)
    return "\n".join(out_lines)


def _update_claude_md_from_template(g: TensionGraph) -> None:
    """Regenerate root CLAUDE.md from CLAUDE.md.template.txt (template-driven model)."""
    new_text = render_claude_md_from_template(g)
    _write(CLAUDE_MD, new_text)


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

    Default: write docs/gen/{REQUIREMENTS,TENSIONS,OPEN}.md from the active domain.
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
    parser.add_argument(
        "--docs-only",
        action="store_true",
        help=(
            "regenerate ONLY the active domain's docs/gen/ (+ methodology "
            "atoms); skip every root CLAUDE.md block and the agent-crystal "
            "regen. Used by apply_proposal.py when it lands a proposal for a "
            "NON-pinned domain (e.g. HOTAM_SPEC_ACTIVE_DOMAIN=hotam-dev): the "
            "applied domain's docs must refresh, but the resident operator "
            "crystal (root CLAUDE.md) must stay bound to the PINNED self-host "
            "domain — never contaminated by the transiently-active env domain "
            "(R-root-crystal-follows-pin). apply_proposal.py then runs a "
            "second, env-stripped gen_spec pass to refresh the root crystal "
            "from the pin."
        ),
    )
    args = parser.parse_args(argv)
    g = _load_graph(demo=args.demo)
    # Seed the domain-graph cache with the already-loaded active domain graph
    # so _render_domain_map_block and _process_domains reuse it (dedup).
    if not args.demo:
        _active_dir = _active_domain()
        if _active_dir is not None:
            _DOMAIN_GRAPH_CACHE[_active_dir.name] = g
    # When a domain is active, write docs into its docs/gen/ rather than root docs/gen/.
    out_dir = DEMO_DIR if args.demo else GEN_DIR
    # Determine domain name for the ENTITIES.md header comment.
    _domain_name_for_entities = ""
    if not args.demo:
        _active = _active_domain()
        if _active is not None:
            _domain_name_for_entities = _active.name
    # --docs-only: skip this env-active `g`-rendered block entirely. Both the
    # out_dir (GEN_DIR) targets and the methodology atoms below render from `g`
    # = the env-active graph; under --docs-only the env domain is a TRANSIENT
    # applied domain (e.g. hotam-dev), and the methodology atoms in particular
    # are self-host substrate that must never be rendered from it
    # (R-root-crystal-follows-pin). The applied domain's docs/gen/ are refreshed
    # authoritatively by _process_domains(g) at the end, which renders EACH
    # domain from its OWN graph — env-independent.
    if not args.docs_only:
        targets = {
            out_dir / "REQUIREMENTS.md": build_requirements(g),
            out_dir / "TENSIONS.md": build_tensions(g),
            out_dir / "OPEN.md": build_open(g),
            out_dir / "UNENFORCED.md": build_unenforced(g),
            out_dir / "GLOSSARY.md": build_glossary(g),
            out_dir / "HISTORY.md": build_history(g),
            out_dir / "CONSTITUTION.md": build_constitution(g),
            out_dir / "FRAMEWORK-INVARIANTS.md": build_framework_invariants(g),
            out_dir / "REPO-MAP.md": build_repo_map_md(g),
            out_dir / "graph.json": build_graph_json(g),
        }
        # Conditional materialization (task #106 / L2-#6) — see the matching
        # comment in _process_domains for the full rationale: DECISIONS.md /
        # ENTITIES.md are written only when their registry is non-empty.
        if _decisions_md_has_content(g):
            targets[out_dir / "DECISIONS.md"] = build_decisions(g)
        if _entities_md_has_content(g):
            targets[out_dir / "ENTITIES.md"] = build_entities_md(g, _domain_name_for_entities)
        for path, text in targets.items():
            _write(path, text)
            print(f"written: {path}")

    # Atomized methodology docs (docs/methodology/atoms/) — always written, not demo-gated.
    if not args.demo and not args.docs_only:
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

    if not args.demo and not args.docs_only:
        # Root CLAUDE.md: template-driven (R-claude-md-template-driven).
        # CLAUDE.md.template.txt is substituted in one pass — MIND (REPO-MAP +
        # THINKING-INDEX + EMBEDDED-THINKING + EMBEDDED-TOOLS) and BUSINESS
        # (LIVE-STATE + DOMAIN-MAP + CONSTITUTION + AGENT-MAP + CONCEPT-MAP +
        # RECENTLY-REJECTED), replacing the old ~10-sentinel surgical splice AND
        # the earlier DOMAIN-CRYSTAL indirection through a separate
        # domains/<name>/CLAUDE.md file (deleted in the P22.C consolidation,
        # tasks #101/#102 — there is exactly ONE CLAUDE.md, at repo root).
        _update_claude_md_from_template(g)
        print(f"updated: {CLAUDE_MD}")

        # _AGENTS_ROOT resolves to an absent directory now that
        # domains/hotam-spec-self/agents/ has been deleted (P22.C) — these are
        # no-ops until a real second agent is scaffolded via create_agent.py.
        _regenerate_agent_constitutions(g)

        # Concern 5a/5b: shared thinking + tool docs (spec/docs/thinking/*.md,
        # spec/docs/tools/*.md) document the FRAMEWORK's own source (docstrings
        # under SPEC_ROOT, which is the loaded gen_spec.py module's own file
        # location — NOT consumer-root-aware like REPO_ROOT/CLAUDE_MD is).
        # Only write them when REPO_ROOT actually IS the framework's own repo
        # (self-hosting): otherwise a consumer project (or a test loading
        # gen_spec against a throwaway domain, e.g.
        # test_portability_w4_smoke_e2e.py) would regenerate the FRAMEWORK's
        # committed docs with whatever reader happens to be active for the
        # unrelated domain being processed — the same class of contamination
        # R-root-crystal-follows-pin already closed for CLAUDE.md and REPO-MAP.
        if REPO_ROOT == SPEC_ROOT.parent:
            thinking_paths = _write_shared_thinking_docs(
                reader_stakeholder_ids=stakeholder_ids(g)
            )
            for p in thinking_paths:
                print(f"written (thinking): {p}")

            tool_doc_paths = _write_shared_tool_docs(
                reader_stakeholder_ids=stakeholder_ids(g)
            )
            for p in tool_doc_paths:
                print(f"written (tool-doc): {p}")
        else:
            print(
                "skipping shared thinking/tool docs: REPO_ROOT != framework's "
                "own repo (not self-hosting)"
            )

        # Concern 5c: SHARED-DOCS block in agent CLAUDE.md files (no-op: no
        # agents exist).
        _regenerate_agent_shared_docs(g)

    if not args.demo:
        # Concern 1: per-domain generated docs (domains/<name>/docs/gen/*.md —
        # the DATA layer). Env-independent (each domain rendered from its OWN
        # graph), so it runs in --docs-only mode too: this is the path that
        # refreshes the applied domain's docs/gen/ without touching the root
        # crystal (R-root-crystal-follows-pin).
        _process_domains(g)


if __name__ == "__main__":
    main()
