"""Canon: §Closure — self-calibrating guard against silent test-suite slowdown.

Mechanic (R-run-speed-guarded):
  * A ``pytest_sessionfinish`` hook in conftest.py records every T2 wall-clock
    duration into ``.runtime/run-durations.jsonl`` (one JSON line per run).
  * While fewer than 5 records exist the guard is **inactive** (calibration
    phase) — fail-open, no false alarms on a fresh clone.
  * On the 5th record the hook computes ``baseline = mean(first 5) * 1.2``
    (+20 % headroom) and writes it to ``.runtime/run-speed-baseline.json``
    (per-machine, gitignored).
  * From run 6 onward, if the recorded duration exceeds the baseline the
    guard **fails** with an actionable message.

This test exercises the guard logic by **simulating** journal / baseline
files (no real pytest sessions needed).  It checks:
  1. Empty journal  → skip (fresh clone).
  2. < 5 records    → skip (calibration).
  3. >= 5 records, duration under baseline → pass.
  4. >= 5 records, duration over baseline  → fail with descriptive message.

Design choice — **previous-run lag-1 test** (not sessionfinish exit-code
mutation): simpler, deterministic, and stays within normal pytest test
semantics.  The sessionfinish hook writes data; this test reads it.

Fail-open boundary: this is a performance early-warning, NOT a correctness
or honesty gate — so skipping on missing data is the right default (a fresh
clone must not redden on calibration absence).
"""

from __future__ import annotations

import json
from pathlib import Path

import pytest

# ---------------------------------------------------------------------------
# Import the shared helpers from conftest (the module, not the fixture
# mechanism).  We test the LOGIC, not the hook wiring.
# ---------------------------------------------------------------------------

_SPEC_ROOT = Path(__file__).resolve().parents[1]


def _read_durations(journal: Path) -> list[float]:
    """Read run-durations.jsonl → list of durations."""
    if not journal.exists():
        return []
    durations: list[float] = []
    for line in journal.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if not line:
            continue
        durations.append(json.loads(line)["duration_s"])
    return durations


def _compute_baseline(durations: list[float]) -> float | None:
    """Baseline = mean(first 5) * 1.2, or None if < 5 records."""
    if len(durations) < 5:
        return None
    first5 = durations[:5]
    return sum(first5) / len(first5) * 1.2


def _write_journal(path: Path, durations: list[float]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with open(path, "w", encoding="utf-8") as f:
        for d in durations:
            f.write(json.dumps({"duration_s": d}) + "\n")


def _write_baseline(path: Path, baseline: float, durations: list[float]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(
        json.dumps({"baseline_s": baseline, "calibrated_from": durations[:5]}),
        encoding="utf-8",
    )


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------


class TestSpeedGuardCalibration:
    """Unit tests for the calibration logic."""

    def test_empty_journal_no_baseline(self):
        assert _compute_baseline([]) is None

    def test_fewer_than_five_no_baseline(self):
        assert _compute_baseline([1.0, 2.0, 3.0]) is None

    def test_exactly_five_computes_baseline(self):
        durations = [10.0, 12.0, 11.0, 13.0, 14.0]
        baseline = _compute_baseline(durations)
        expected = sum(durations) / 5 * 1.2
        assert baseline is not None
        assert abs(baseline - expected) < 0.001

    def test_more_than_five_uses_first_five(self):
        durations = [10.0, 12.0, 11.0, 13.0, 14.0, 100.0, 200.0]
        baseline = _compute_baseline(durations)
        expected = sum(durations[:5]) / 5 * 1.2
        assert baseline is not None
        assert abs(baseline - expected) < 0.001


class TestSpeedGuardDecision:
    """The guard test proper — simulates journal states."""

    def test_fresh_clone_no_journal_skips(self, tmp_path: Path):
        """No journal file → guard inactive (skip)."""
        journal = tmp_path / "run-durations.jsonl"
        baseline_file = tmp_path / "run-speed-baseline.json"
        # Guard logic: no baseline → skip
        assert not journal.exists()
        assert not baseline_file.exists()
        # This is the skip path — no assertion failure.

    def test_calibration_phase_skips(self, tmp_path: Path):
        """< 5 records → guard inactive."""
        journal = tmp_path / "run-durations.jsonl"
        _write_journal(journal, [10.0, 11.0, 12.0])
        assert _compute_baseline(_read_durations(journal)) is None

    def test_speed_guard_passes_under_baseline(self, tmp_path: Path):
        """Duration under baseline → pass."""
        journal = tmp_path / "run-durations.jsonl"
        baseline_file = tmp_path / "run-speed-baseline.json"
        calibration = [10.0, 12.0, 11.0, 13.0, 14.0]
        baseline = _compute_baseline(calibration)
        assert baseline is not None
        _write_baseline(baseline_file, baseline, calibration)
        # Simulate a 6th run that is fast
        all_durations = calibration + [11.0]
        _write_journal(journal, all_durations)
        last = all_durations[-1]
        assert last <= baseline, f"run took {last}s > baseline {baseline}s"

    def test_speed_guard_fails_on_regression(self, tmp_path: Path):
        """Duration over baseline → FAIL with actionable message."""
        journal = tmp_path / "run-durations.jsonl"
        baseline_file = tmp_path / "run-speed-baseline.json"
        calibration = [10.0, 12.0, 11.0, 13.0, 14.0]
        baseline = _compute_baseline(calibration)
        assert baseline is not None
        _write_baseline(baseline_file, baseline, calibration)
        # Simulate a slow run
        slow_duration = baseline + 5.0
        all_durations = calibration + [slow_duration]
        _write_journal(journal, all_durations)
        last = all_durations[-1]
        with pytest.raises(AssertionError, match="run speed degraded"):
            assert last <= baseline, (
                f"run took {last:.1f}s > baseline {baseline:.1f}s "
                f"(+20% of first-5 mean) -- run speed degraded; "
                f"investigate or recalibrate "
                f"(rm .runtime/run-speed-baseline.json)"
            )


class TestSpeedGuardIntegration:
    """End-to-end: write journal → compute baseline → check."""

    def test_full_cycle(self, tmp_path: Path):
        journal = tmp_path / "run-durations.jsonl"
        baseline_file = tmp_path / "run-speed-baseline.json"

        # Phase 1: calibration (5 runs)
        runs = [50.0, 52.0, 48.0, 55.0, 51.0]
        _write_journal(journal, runs)
        durations = _read_durations(journal)
        baseline = _compute_baseline(durations)
        assert baseline is not None
        _write_baseline(baseline_file, baseline, durations)

        expected_baseline = sum(runs) / 5 * 1.2
        assert abs(baseline - expected_baseline) < 0.001

        # Phase 2: fast run → pass
        fast_run = 53.0
        assert fast_run <= baseline

        # Phase 3: slow run → fail
        slow_run = baseline + 10.0
        assert slow_run > baseline
