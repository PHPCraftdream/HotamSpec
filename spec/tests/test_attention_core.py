"""Canon: §Attention — the agent-agnostic attention-code registry and its split.

Enforces the Wave 16 attention core:
  R-attention-registry              — the registry exists; graph diagnosis is
                                      deterministic; ATTENTION_SOURCES is
                                      non-empty and graph-only.
  R-attention-superset-of-diagnose  — collect(g) with no runtime sources equals
                                      diagnose_signals(g) (the deterministic
                                      subset), and collect(g, runtime_sources=...)
                                      is a SUPERSET; a graph source may not be
                                      injected as a runtime source.
  R-attention-agent-agnostic-core   — hotam_spec.attention names no Claude / hook
                                      / platform token; it is a pure core.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

import pytest

# spec/src and spec/tools are already on sys.path via conftest.py's
# suite-wide bootstrap (loaded before any test module); no per-file insert
# needed here (R-shared-tools-in-spec-tools hygiene: conftest is the ONE
# sys.path bootstrap for the test suite).
_SRC = Path(__file__).resolve().parents[1] / "src"

from hotam_spec import attention  # noqa: E402
from hotam_spec.attention import (  # noqa: E402
    ATTENTION_SOURCES,
    READS_GRAPH,
    READS_RUNTIME_FS,
    AttentionSignal,
    AttentionSource,
    collect,
    diagnose_signals,
)


def _graph():
    tests_dir = str(Path(__file__).resolve().parent)
    if tests_dir not in sys.path:
        sys.path.insert(0, tests_dir)
    from fixtures.seed import seed_graph  # noqa: PLC0415

    return seed_graph()


# --- R-attention-registry ---------------------------------------------------


def test_registry_nonempty_and_graph_only() -> None:
    """ATTENTION_SOURCES exists, is non-empty, and every framework source is a
    deterministic GRAPH source (runtime-fs sources are injected, never built in)."""
    assert ATTENTION_SOURCES, "the attention registry must have at least one source"
    assert all(s.reads == READS_GRAPH for s in ATTENTION_SOURCES), (
        "framework ATTENTION_SOURCES must be graph-only "
        "(R-attention-superset-of-diagnose)"
    )
    ids = [s.id for s in ATTENTION_SOURCES]
    assert "diagnose" in ids, "the graph diagnosis source must be registered"


def test_diagnose_signals_deterministic() -> None:
    """The graph-only diagnosis is deterministic: two runs are byte-identical."""
    g = _graph()
    a = diagnose_signals(g)
    b = diagnose_signals(g)
    assert a == b
    assert all(isinstance(s, AttentionSignal) for s in a)


def test_collect_no_runtime_equals_diagnose() -> None:
    """collect(g) with no injected runtime sources IS diagnose_signals(g) — the
    deterministic subset (R-attention-registry / R-attention-superset-of-diagnose)."""
    g = _graph()
    assert collect(g) == diagnose_signals(g)


# --- R-attention-superset-of-diagnose ---------------------------------------


def test_collect_is_superset_of_diagnose() -> None:
    """collect(g, runtime_sources=...) contains every diagnose signal plus the
    injected runtime-fs signals — a strict superset by construction."""
    g = _graph()
    base = set(diagnose_signals(g))

    def fake_runtime(_g):
        return [AttentionSignal("fake-fs", attention.P_RUNTIME, "meter", "hello")]

    src = AttentionSource(id="fake-fs", reads=READS_RUNTIME_FS, collect=fake_runtime)
    sup = collect(g, runtime_sources=(src,))
    assert base.issubset(set(sup))
    assert any(s.source == "fake-fs" for s in sup)
    assert len(sup) == len(base) + 1


def test_graph_source_rejected_as_runtime() -> None:
    """A READS_GRAPH source injected as a runtime source is rejected loudly —
    graph sources belong in ATTENTION_SOURCES, not the injection seam."""
    g = _graph()
    bad = AttentionSource(id="bad", reads=READS_GRAPH, collect=lambda _g: [])
    with pytest.raises(ValueError):
        collect(g, runtime_sources=(bad,))


def test_collect_sorted_stable() -> None:
    """collect output is stably ordered by (priority, source, target, message)."""
    g = _graph()
    sig = collect(g)
    keys = [(s.priority, s.source, s.target, s.message) for s in sig]
    assert keys == sorted(keys)


# --- R-attention-agent-agnostic-core ----------------------------------------

_FORBIDDEN = re.compile(r"claude|anthropic|hook|opus|sonnet|haiku", re.IGNORECASE)


def test_core_names_no_platform_token() -> None:
    """hotam_spec.attention is agent-agnostic: its source text names no Claude /
    Anthropic / hook / model token — the platform seam lives in the tool
    (tools/attention_hook.py), never in the core (R-attention-agent-agnostic-core)."""
    src = (_SRC / "hotam_spec" / "attention.py").read_text(encoding="utf-8")
    offenders = sorted({m.group(0).lower() for m in _FORBIDDEN.finditer(src)})
    assert not offenders, (
        "hotam_spec.attention must name no agent-platform token; found: "
        + ", ".join(offenders)
        + " (R-attention-agent-agnostic-core)"
    )
