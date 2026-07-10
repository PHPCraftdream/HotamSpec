"""Canon: §Operator — emits the three-cipher pulse (top action / debt / context) directly from the active domain's graph."""

import argparse
import json
import sys

import _bootstrap  # noqa: F401  -- side effect: sys.path configured

from hotam_spec.domain_resolution import resolve_active_domain  # noqa: E402


def _pinned_domain() -> str:
    """Return the active self-host domain name (whose graph the cipher reflects).

    Delegates to the single shared resolver
    (hotam_spec.domain_resolution.resolve_active_domain, R-active-domain-pin-not-alphabetical)
    so this agrees with gen_spec.py / apply_proposal.py on env -> pin ->
    alphabetical priority, instead of reading the pin file directly and
    silently ignoring HOTAM_SPEC_ACTIVE_DOMAIN.
    """
    import gen_spec as _gen_spec  # noqa: PLC0415

    return resolve_active_domain(_gen_spec.DOMAINS_ROOT) or ""


def _other_domains_open() -> int:
    """Sum open-action counts across every domain in domains/ EXCEPT the active one.

    The three-cipher pulse (top/debt/context) already reflects the active
    self-host domain's graph directly (R-domain-map-shows-pulse's root-crystal
    counterpart). This aggregate is the SECOND eye: how many open actions live
    in OTHER domains, invisible to the self-host cipher (e.g. hotam-dev's
    DETECTED conflict). Returns 0 when domains/ is absent or empty.

    Computed directly from each domain's graph.py (via gen_spec's domain
    loader + what_now.diagnose), NOT by parsing the rendered DOMAIN-MAP
    markdown — the graph is the source, the markdown is a rendering of it.
    """
    import gen_spec as _gen_spec  # noqa: PLC0415
    import what_now as _what_now  # noqa: PLC0415

    if not _gen_spec.DOMAINS_ROOT.exists():
        return 0
    active = _pinned_domain()
    total = 0
    for domain_dir in _gen_spec._sorted_domain_dirs():
        if domain_dir.name == active:
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
