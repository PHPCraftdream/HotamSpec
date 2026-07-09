"""Canon: §Operator — emits the three-cipher pulse (top action / debt / context) directly from the active domain's graph."""

import argparse
import json
import sys
from pathlib import Path

# Make hotam_spec importable so this standalone tool can resolve the consumer
# project root via the shared R1-R6 chain (R-project-root-not-hardcoded), and
# make tools/ importable so gen_spec (the render-time source of the cipher
# values) and what_now (per-domain diagnosis) can be imported directly.
_SPEC_ROOT = Path(__file__).resolve().parents[1]
if str(_SPEC_ROOT / "src") not in sys.path:
    sys.path.insert(0, str(_SPEC_ROOT / "src"))
_TOOLS = Path(__file__).resolve().parent
if str(_TOOLS) not in sys.path:
    sys.path.insert(0, str(_TOOLS))

from hotam_spec.project_paths import project_root_or_raise  # noqa: E402

# Consumer paths: domains/.active-domain is CONSUMER data, resolved via
# project_root(). In self-hosting R3 yields the same path as parents[2].
_REPO_ROOT = project_root_or_raise()

_PIN_FILE = _REPO_ROOT / "domains" / ".active-domain"


def _pinned_domain() -> str:
    """Return the pinned self-host domain name (whose graph the cipher reflects)."""
    try:
        return _PIN_FILE.read_text(encoding="utf-8").strip()
    except OSError:
        return ""


def _other_domains_open(text: str = "") -> int:  # noqa: ARG001
    """Sum open-action counts across every domain in domains/ EXCEPT the pinned one.

    The three-cipher pulse (top/debt/context) already reflects the pinned
    self-host domain's graph directly (R-domain-map-shows-pulse's root-crystal
    counterpart). This aggregate is the SECOND eye: how many open actions live
    in OTHER domains, invisible to the self-host cipher (e.g. hotam-dev's
    DETECTED conflict). Returns 0 when domains/ is absent or empty.

    Computed directly from each domain's graph.py (via gen_spec's domain
    loader + what_now.diagnose), NOT by parsing the rendered DOMAIN-MAP
    markdown — the graph is the source, the markdown is a rendering of it.
    The `text` parameter is accepted (unused) for backward compatibility with
    callers that still pass rendered CLAUDE.md text.
    """
    import gen_spec as _gen_spec  # noqa: PLC0415
    import what_now as _what_now  # noqa: PLC0415

    if not _gen_spec.DOMAINS_ROOT.exists():
        return 0
    pinned = _pinned_domain()
    total = 0
    for domain_dir in _gen_spec._sorted_domain_dirs():
        if domain_dir.name == pinned:
            continue
        dg = _gen_spec._load_domain_graph(domain_dir)
        if dg is None:
            continue
        try:
            total += len(_what_now.diagnose(dg))
        except Exception:  # noqa: BLE001
            continue
    return total


def main() -> None:
    parser = argparse.ArgumentParser(
        description="Emit the three-cipher pulse as a hook JSON payload."
    )
    parser.parse_args()

    import gen_spec as _gen_spec  # noqa: PLC0415
    from hotam_spec.graph import load_content_graph  # noqa: PLC0415

    g = load_content_graph()
    top, debt = _gen_spec.compute_cipher_lines(g)
    # NOTE: the LIVE-STATE markdown's context line ("- context: ...") has no
    # bold **context:** key (unlike top action / debt), so it was never
    # extracted by the old regex-based bullet parser either — `context` was
    # always empty in the payload. Preserved here for bit-identical output.
    context = ""

    other_open = _other_domains_open()

    if top or debt or context or other_open:
        parts = [p for p in [top, debt, context] if p]
        if other_open > 0:
            parts.append(f"other domains: {other_open} open")
        additional = "Three-cipher pulse — cite in first sentence: " + " · ".join(parts)
    else:
        additional = ""

    payload = {
        "hookSpecificOutput": {
            "hookEventName": "UserPromptSubmit",
            "additionalContext": additional,
        }
    }
    sys.stdout.reconfigure(encoding="utf-8")
    sys.stdout.write(json.dumps(payload, ensure_ascii=False) + "\n")


if __name__ == "__main__":
    main()
