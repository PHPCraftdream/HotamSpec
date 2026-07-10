"""Canon: §Entity — scaffolds an EntityType declaration into the active domain's graph via apply_proposal.

WHY: Declaring a new EntityType by hand requires constructing a valid ProposedEntityType
JSON, passing validation (kebab-case slug, exactly one initial state, valid transition
endpoints, known field kinds), and calling apply_proposal. A single typo produces a
confusing error deep in the apply pipeline. This tool parses and validates the high-level
CLI form, constructs the JSON, and delegates to apply_proposal as a subprocess — keeping
the workflow consistent with the manual apply path and testable end-to-end.

R-prefer-tool-over-hand: scaffolding belongs here because the EntityType shape must
satisfy structural contracts (lifecycle well-formedness, ENTITY_FIELD_KINDS, etc.).

Exit codes:
  0 — success (EntityType landed, docs regenerated, tests green).
  1 — failure (validation error, duplicate slug, or apply_proposal failed).
"""

from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
import tempfile
from pathlib import Path

_SLUG_RE = re.compile(r"^[a-z][a-z0-9-]*$")

_SPEC_ROOT = Path(__file__).resolve().parents[1]
_SRC = _SPEC_ROOT / "src"
if _SRC.is_dir() and str(_SRC) not in sys.path:
    sys.path.insert(0, str(_SRC))

from hotam_spec.entity import ENTITY_FIELD_KINDS  # noqa: E402
from hotam_spec.lifecycle import STATE_KINDS  # noqa: E402

_APPLY_PROPOSAL = Path(__file__).resolve().parent / "apply_proposal.py"


def _parse_states(states_str: str) -> list[tuple[str, str, str]]:
    """Parse 'NAME:kind,NAME2:kind2' into list of (name, kind, why) triples."""
    result: list[tuple[str, str, str]] = []
    for part in states_str.split(","):
        part = part.strip()
        if not part:
            continue
        tokens = part.split(":")
        if len(tokens) < 2:
            raise ValueError(
                f"State '{part}' must be 'NAME:kind' (e.g. 'ACTIVE:initial')."
            )
        s_name, s_kind = tokens[0].strip(), tokens[1].strip()
        s_why = tokens[2].strip() if len(tokens) > 2 else ""
        if s_kind not in STATE_KINDS:
            raise ValueError(
                f"State kind '{s_kind}' is not valid. Must be one of {sorted(STATE_KINDS)}."
            )
        result.append((s_name, s_kind, s_why))
    return result


def _parse_transitions(transitions_str: str) -> list[tuple[str, str, str]]:
    """Parse 'event:src->dst,...' into list of (src, dst, event) triples."""
    result: list[tuple[str, str, str]] = []
    for part in transitions_str.split(","):
        part = part.strip()
        if not part:
            continue
        # Format: event:src->dst
        if ":" not in part:
            raise ValueError(
                f"Transition '{part}' must be 'event:src->dst' (e.g. 'suspend:ACTIVE->SUSPENDED')."
            )
        event, arrow_part = part.split(":", 1)
        event = event.strip()
        if "->" not in arrow_part:
            raise ValueError(
                f"Transition '{part}' must be 'event:src->dst' (arrow '->' missing)."
            )
        t_src, t_dst = arrow_part.split("->", 1)
        result.append((t_src.strip(), t_dst.strip(), event))
    return result


def _parse_fields(fields_str: str) -> list[tuple[str, str, bool, str]]:
    """Parse 'name:kind[:required[:ref_target]],...' into list of (name,kind,required,ref) tuples."""
    result: list[tuple[str, str, bool, str]] = []
    for part in fields_str.split(","):
        part = part.strip()
        if not part:
            continue
        tokens = part.split(":")
        if len(tokens) < 2:
            raise ValueError(
                f"Field '{part}' must be at least 'name:kind' (e.g. 'email:string')."
            )
        f_name, f_kind = tokens[0].strip(), tokens[1].strip()
        f_required = len(tokens) > 2 and tokens[2].strip().lower() == "required"
        f_ref_target = tokens[3].strip() if len(tokens) > 3 else ""
        if f_kind not in ENTITY_FIELD_KINDS:
            raise ValueError(
                f"Field kind '{f_kind}' is not valid. Must be one of {sorted(ENTITY_FIELD_KINDS)}."
            )
        result.append((f_name, f_kind, f_required, f_ref_target))
    return result


def scaffold(
    slug: str,
    description: str,
    states_str: str,
    transitions_str: str,
    fields_str: str = "",
    cyclic: bool = False,
) -> int:
    """Validate args, build ProposedEntityType JSON, call apply_proposal. Returns exit code."""
    # Validate slug
    if not slug or not _SLUG_RE.match(slug):
        print(
            f"ERROR: slug '{slug}' must be kebab-case (lowercase letters, digits, hyphens, "
            "starting with a letter).",
            file=sys.stderr,
        )
        return 1

    if not description.strip():
        print(
            "ERROR: --description is required and must be non-empty.", file=sys.stderr
        )
        return 1

    if not states_str.strip():
        print("ERROR: --states is required and must be non-empty.", file=sys.stderr)
        return 1

    if not transitions_str.strip():
        print(
            "ERROR: --transitions is required and must be non-empty.", file=sys.stderr
        )
        return 1

    try:
        states = _parse_states(states_str)
    except ValueError as exc:
        print(f"ERROR: invalid --states: {exc}", file=sys.stderr)
        return 1

    # Exactly one initial
    initial_count = sum(1 for _, k, _ in states if k == "initial")
    if initial_count != 1:
        print(
            f"ERROR: exactly one state must have kind='initial'; found {initial_count}.",
            file=sys.stderr,
        )
        return 1

    state_names = {s[0] for s in states}

    try:
        transitions = _parse_transitions(transitions_str)
    except ValueError as exc:
        print(f"ERROR: invalid --transitions: {exc}", file=sys.stderr)
        return 1

    # Validate transition endpoints
    for t_src, t_dst, t_event in transitions:
        if t_src not in state_names:
            print(
                f"ERROR: transition src '{t_src}' is not a declared state. "
                f"Declared: {sorted(state_names)}.",
                file=sys.stderr,
            )
            return 1
        if t_dst not in state_names:
            print(
                f"ERROR: transition dst '{t_dst}' is not a declared state. "
                f"Declared: {sorted(state_names)}.",
                file=sys.stderr,
            )
            return 1

    fields: list[tuple[str, str, bool, str]] = []
    if fields_str.strip():
        try:
            fields = _parse_fields(fields_str)
        except ValueError as exc:
            print(f"ERROR: invalid --fields: {exc}", file=sys.stderr)
            return 1

    proposal = {
        "kind": "EntityType",
        "slug": slug,
        "description": description,
        "why": f"Domain entity type '{slug}' declared via create_entity_type.",
        "states": [[s[0], s[1], s[2]] for s in states],
        "transitions": [[t[0], t[1], t[2]] for t in transitions],
        "cyclic": cyclic,
        "fields": [[f[0], f[1], f[2], f[3]] for f in fields],
    }

    with tempfile.NamedTemporaryFile(
        mode="w", suffix=".json", delete=False, encoding="utf-8"
    ) as tmp:
        json.dump(proposal, tmp, ensure_ascii=False)
        tmp_path = tmp.name

    try:
        result = subprocess.run(
            [sys.executable, str(_APPLY_PROPOSAL), tmp_path],
            capture_output=False,
        )
        return result.returncode
    finally:
        Path(tmp_path).unlink(missing_ok=True)


def main(argv: list[str] | None = None) -> int:
    """CLI entry point for create_entity_type.py."""
    parser = argparse.ArgumentParser(
        description=(
            "Scaffold an EntityType declaration into the active domain's graph "
            "via apply_proposal."
        )
    )
    parser.add_argument(
        "slug",
        help=(
            "Kebab-case entity type slug (e.g. 'customer', 'order'). "
            "Lowercase letters, digits, hyphens only, starting with a letter."
        ),
    )
    parser.add_argument(
        "--description",
        required=True,
        help="One-line description of the entity type.",
    )
    parser.add_argument(
        "--states",
        required=True,
        help=(
            "Comma-separated 'NAME:kind' pairs. "
            "kind ∈ {initial,normal,terminal,quiescent}. "
            "Exactly one must be 'initial'. "
            "Example: 'ACTIVE:initial,SUSPENDED:normal,CLOSED:quiescent'"
        ),
    )
    parser.add_argument(
        "--transitions",
        required=True,
        help=(
            "Comma-separated 'event:src->dst' triples. "
            "src and dst must be declared state names. "
            "Example: 'suspend:ACTIVE->SUSPENDED,close:ACTIVE->CLOSED,reopen:SUSPENDED->ACTIVE'"
        ),
    )
    parser.add_argument(
        "--fields",
        default="",
        help=(
            "Comma-separated 'name:kind[:required[:ref_target]]' tuples. "
            "kind ∈ {string,number,enum,reference,state}. "
            "Append ':required' to mark the field as required. "
            "Example: 'email:string:required,tier:enum,owner:reference:stakeholder'"
        ),
    )
    parser.add_argument(
        "--cyclic",
        action="store_true",
        default=False,
        help="Mark the entity lifecycle as cyclic (states can loop back).",
    )
    args = parser.parse_args(argv)

    return scaffold(
        slug=args.slug,
        description=args.description,
        states_str=args.states,
        transitions_str=args.transitions,
        fields_str=args.fields,
        cyclic=args.cyclic,
    )


if __name__ == "__main__":
    sys.exit(main())
