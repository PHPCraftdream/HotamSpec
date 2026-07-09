"""Canon: §Agent — composes a sub-agent's task prompt by prepending the agent's CLAUDE.md crystal, so the subagent boots from substrate (not from raw text).

Resolve the named agent under the domains/<name>/agents/... hierarchy (or the
legacy spec/agents/<name>/ hierarchy), read its CLAUDE.md, and compose a
composite prompt:

  You are operating under the following crystal (your CLAUDE.md):
  ----- CRYSTAL BEGIN -----
  <agent's CLAUDE.md content>
  ----- CRYSTAL END -----

  ## Your task

  <task>

The composite is printed to stdout. Additionally, a JSONL entry is appended to
spec/.runtime/spawn-log.jsonl for runtime observability (the directory and file
are created on first use; they are gitignored, not committed substrate).

Each entry also carries isolation ("worktree" | "shared", default "shared")
and mutating (bool, default false) fields (R-spawn-log-carries-isolation),
settable via --isolation/--mutating. This records ONLY what the caller
declared at spawn time -- it is not a mechanical proof that a "worktree"
isolation value corresponds to a real git worktree, nor that "mutating"
correctly predicts file writes. The stronger claim (parallel mutating agents
actually use worktree isolation in practice) is a separate, honestly-scoped
atom: R-parallel-mutating-agents-use-worktree (see
tools/spawn_log_isolation_status.py for its structural reader, which can only
check the log's internal consistency, not runtime truth).

DETERMINISM: no timestamps are produced inside the tool. The caller MUST pass
--stamp (an ISO 8601 string from outside) so that successive runs over the same
inputs produce identical stdout bytes (R-deterministic-generation). The tool
exits 1 if --stamp is missing.

Usage:
  uv run python tools/spawn_agent.py <agent-path> --task "<task description>" --stamp <iso8601>

Examples:
  uv run python tools/spawn_agent.py domains/hotam-spec-self/agents/director/agents/framework-agent \\
      --task "audit all check_* for atomicity" --stamp 2026-06-29T12:00:00Z

  # Short form — trailing path segment resolved under active domain:
  uv run python tools/spawn_agent.py director/framework-agent \\
      --task "run bijection check" --stamp 2026-06-29T12:00:00Z
"""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

# Make hotam_spec importable so this standalone tool resolves the consumer
# project root via the shared R1-R6 chain (R-project-root-not-hardcoded).
_SPEC_ROOT = Path(__file__).resolve().parents[1]  # .../spec
if str(_SPEC_ROOT / "src") not in sys.path:
    sys.path.insert(0, str(_SPEC_ROOT / "src"))

from hotam_spec.project_paths import project_root_or_raise  # noqa: E402
from hotam_spec.repo_paths import domains_root as _domains_root  # noqa: E402

# ---------------------------------------------------------------------------
# Path constants — monkeypatchable in tests (override slots; None = resolve fresh)
# ---------------------------------------------------------------------------

# Consumer roots: domains/ is CONSUMER data. Module-level names kept as
# override slots for tests; when None, resolved FRESH via project_root()/domains_root()
# on each use (§3.3 — NO import-time resolver-result cache).
_REPO_ROOT: Path | None = None
_DOMAINS_ROOT: Path | None = None
_LEGACY_AGENTS_ROOT = _SPEC_ROOT / "agents"
_RUNTIME_DIR = _SPEC_ROOT / ".runtime"


def _project_root() -> Path:
    """Resolve consumer project root (fresh each call) or return override slot."""
    if _REPO_ROOT is not None:
        return _REPO_ROOT
    return project_root_or_raise()


def _domains_root_path() -> Path:
    """Resolve consumer domains root (fresh each call) or return override slot."""
    if _DOMAINS_ROOT is not None:
        return _DOMAINS_ROOT
    return _domains_root()


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _all_agent_dirs(domains_root: Path, legacy_root: Path) -> list[Path]:
    """Return sorted list of all agent directories reachable from root."""
    results: list[Path] = []

    def _scan(parent: Path) -> None:
        agents_dir = parent / "agents"
        if not agents_dir.exists() or not agents_dir.is_dir():
            return
        for entry in sorted(agents_dir.iterdir()):
            if entry.is_dir():
                results.append(entry)
                _scan(entry)  # recurse for nested agents

    # Domains hierarchy: domains/<name>/agents/...
    if domains_root.exists():
        for domain_dir in sorted(domains_root.iterdir()):
            if domain_dir.is_dir() and not domain_dir.name.startswith("_"):
                _scan(domain_dir)

    # Legacy: spec/agents/<name>/
    if legacy_root.exists():
        for entry in sorted(legacy_root.iterdir()):
            if entry.is_dir():
                results.append(entry)
                _scan(entry)

    return results


def _resolve_agent(
    agent_path: str,
    domains_root: Path,
    legacy_root: Path,
) -> Path | None:
    """Resolve <agent-path> to an existing directory.

    Tries in order:
    1. Absolute or relative path from cwd.
    2. Absolute path from repo root.
    3. Suffix match against all agent directories (trailing path segment).
    """
    # 1. Direct path
    p = Path(agent_path)
    if p.is_absolute() and p.exists() and p.is_dir():
        return p
    # Relative from cwd
    cwd_p = Path.cwd() / p
    if cwd_p.exists() and cwd_p.is_dir():
        return cwd_p
    # Relative from repo root
    repo_p = (_project_root() / p).resolve()
    if repo_p.exists() and repo_p.is_dir():
        return repo_p

    # 2. Suffix match against known agent dirs
    normalised = agent_path.replace("\\", "/").strip("/")
    for candidate in _all_agent_dirs(domains_root, legacy_root):
        candidate_rel = candidate.as_posix()
        if candidate_rel.endswith(normalised):
            return candidate

    return None


def _compose_prompt(crystal: str, task: str) -> str:
    """Compose the deterministic composite prompt string."""
    return (
        "You are operating under the following crystal (your CLAUDE.md):\n\n"
        "----- CRYSTAL BEGIN -----\n"
        f"{crystal}"
        "----- CRYSTAL END -----\n\n"
        "## Your task\n\n"
        f"{task}\n"
    )


def _append_spawn_log(
    runtime_dir: Path,
    stamp: str,
    agent_path: Path,
    task: str,
    prompt: str,
    *,
    isolation: str = "shared",
    mutating: bool = False,
) -> None:
    """Append a JSON line to <runtime_dir>/spawn-log.jsonl.

    isolation/mutating (R-spawn-log-carries-isolation): every entry records
    whether the spawned agent ran in an isolated worktree ("worktree") or the
    shared working tree ("shared"), and whether the caller declared the task
    as mutating (touches tracked files) or not. Defaults (shared/false) match
    the pre-existing behavior for callers that do not pass the new flags, so
    this is additive, not a breaking format change.
    """
    runtime_dir.mkdir(parents=True, exist_ok=True)
    log_path = runtime_dir / "spawn-log.jsonl"
    first_line = task.splitlines()[0] if task.strip() else ""
    entry = {
        "stamp": stamp,
        "agent": agent_path.as_posix(),
        "task_first_line": first_line,
        "prompt_chars": len(prompt),
        "isolation": isolation,
        "mutating": mutating,
    }
    with log_path.open("a", encoding="utf-8", newline="\n") as fh:
        fh.write(json.dumps(entry, ensure_ascii=False) + "\n")


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------


def main(argv: list[str] | None = None) -> int:
    """Compose a sub-agent's task prompt and print to stdout."""
    parser = argparse.ArgumentParser(
        prog="spawn_agent",
        description=(
            "Compose a sub-agent's task prompt by prepending the agent's "
            "CLAUDE.md crystal, so the subagent boots from substrate."
        ),
    )
    parser.add_argument(
        "agent_path",
        help=(
            "Path to the agent directory (absolute, relative from repo root, "
            "or a trailing path suffix like 'director/framework-agent')."
        ),
    )
    parser.add_argument(
        "--task",
        required=True,
        help="The task description to append after the crystal.",
    )
    parser.add_argument(
        "--stamp",
        required=False,
        default=None,
        help=(
            "ISO 8601 timestamp for the spawn-log entry. REQUIRED for "
            "deterministic output (R-deterministic-generation)."
        ),
    )
    parser.add_argument(
        "--isolation",
        choices=("shared", "worktree"),
        default="shared",
        help=(
            "Whether the spawned agent runs in an isolated worktree or the "
            "shared working tree (R-spawn-log-carries-isolation). Default: shared."
        ),
    )
    parser.add_argument(
        "--mutating",
        action="store_true",
        default=False,
        help=(
            "Declare that this task is expected to mutate tracked files "
            "(R-spawn-log-carries-isolation). Default: false."
        ),
    )
    parser.add_argument(
        "--log-only",
        action="store_true",
        default=False,
        help=(
            "Append a spawn-log row (agent, task first line, stamp, isolation, "
            "mutating) WITHOUT composing or printing the crystal prompt "
            "(R-host-spawn-leaves-trace). Use for a HOST-level spawn (Task/Agent "
            "tool) that does not route through the crystal-composition path but "
            "must still leave a trace. In --log-only mode the agent's CLAUDE.md "
            "is NOT required to exist (host spawns may name a logical agent with "
            "no on-disk crystal); prompt_chars is recorded as 0."
        ),
    )
    args = parser.parse_args(argv)

    # --- Require --stamp ---
    if args.stamp is None:
        print(
            "ERROR: --stamp is required (pass an ISO 8601 timestamp from outside "
            "to keep output deterministic per R-deterministic-generation).",
            file=sys.stderr,
        )
        return 1

    # --- --log-only: record a host spawn WITHOUT composing the crystal ---
    # The agent path is recorded verbatim (posix-normalised) even when it does
    # not resolve to an on-disk agent dir, because a host spawn names a logical
    # agent (e.g. "oh"/"fx" fleet worker) that has no CLAUDE.md crystal. This is
    # the honest trace of what actually ran (R-host-spawn-leaves-trace).
    if args.log_only:
        agent_dir = _resolve_agent(args.agent_path, _domains_root_path(), _LEGACY_AGENTS_ROOT)
        logged_path = (
            agent_dir
            if agent_dir is not None
            else Path(args.agent_path.replace("\\", "/"))
        )
        _append_spawn_log(
            _RUNTIME_DIR,
            args.stamp,
            logged_path,
            args.task,
            "",  # no composed prompt in log-only mode -> prompt_chars == 0
            isolation=args.isolation,
            mutating=args.mutating,
        )
        return 0

    # --- Resolve agent directory ---
    domains_root = _domains_root_path()
    legacy_root = _LEGACY_AGENTS_ROOT
    agent_dir = _resolve_agent(args.agent_path, domains_root, legacy_root)

    if agent_dir is None:
        all_agents = _all_agent_dirs(domains_root, legacy_root)
        avail = (
            ", ".join(
                a.as_posix().replace(str(_project_root()).replace("\\", "/") + "/", "")
                for a in all_agents
            )
            if all_agents
            else "(none)"
        )
        print(
            f"Unknown agent at {args.agent_path!r}. Available: {avail}",
            file=sys.stderr,
        )
        return 1

    # --- Resolve CLAUDE.md ---
    claude_md = agent_dir / "CLAUDE.md"
    if not claude_md.exists():
        print(
            f"Agent at {agent_dir} exists but has no CLAUDE.md.",
            file=sys.stderr,
        )
        return 1

    # --- Read crystal ---
    crystal = (
        claude_md.read_text(encoding="utf-8").replace("\r\n", "\n").replace("\r", "\n")
    )
    # Ensure crystal ends with newline for clean composition
    if crystal and not crystal.endswith("\n"):
        crystal += "\n"

    # --- Compose ---
    prompt = _compose_prompt(crystal, args.task)

    # --- Print to stdout ---
    print(prompt, end="")

    # --- Write spawn log ---
    _append_spawn_log(
        _RUNTIME_DIR,
        args.stamp,
        agent_dir,
        args.task,
        prompt,
        isolation=args.isolation,
        mutating=args.mutating,
    )

    return 0


if __name__ == "__main__":
    sys.exit(main())
