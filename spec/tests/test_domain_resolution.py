"""Canon: §Domain — tests for hotam_spec.domain_resolution.resolve_active_domain.

The env→pin→alphabetical resolution order is the single most safety-critical
piece of routing in the framework: it decides which domain's graph.py gets
written to, which domain's docs/gen/ gets regenerated, and which domain's
operator identity appears in the root CLAUDE.md. Three properties MUST hold:

  1. ENV beats PIN — when HOTAM_SPEC_ACTIVE_DOMAIN names an existing domain,
     it wins over domains/.active-domain.
  2. PIN beats ALPHABETICAL — when the env var is unset but the pin file
     names an existing domain, that domain wins over "first alphabetically".
  3. ALPHABETICAL is the last resort — when neither env nor pin is set (or
     points at a non-existent domain), the first domain alphabetically wins.

These tests use tmp_path fixtures so they do not touch the live domains/
directory. Each test builds a synthetic domains/ tree with manifest.py-free
domain directories and exercises resolve_active_domain against it.
"""

from __future__ import annotations

import os
from pathlib import Path

import pytest

from hotam_spec.domain_resolution import (
    ENV_VAR,
    PIN_FILENAME,
    pin_file_path,
    resolve_active_domain,
)


@pytest.fixture
def domains_tree(tmp_path: Path) -> Path:
    """Build a synthetic domains/ root with two domain sub-directories."""
    domains = tmp_path / "domains"
    domains.mkdir()
    (domains / "alpha-domain").mkdir()
    (domains / "beta-domain").mkdir()
    return domains


@pytest.fixture(autouse=True)
def _clean_env(monkeypatch: pytest.MonkeyPatch) -> None:
    """Ensure HOTAM_SPEC_ACTIVE_DOMAIN is unset for every test."""
    monkeypatch.delenv(ENV_VAR, raising=False)


def test_env_beats_pin(domains_tree: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    """ENV var names a domain → that domain wins over the pin file.

    Pin says 'alpha-domain', env says 'beta-domain' → result must be
    'beta-domain' (env has higher priority).
    """
    pin = domains_tree / PIN_FILENAME
    pin.write_text("alpha-domain", encoding="utf-8")
    monkeypatch.setenv(ENV_VAR, "beta-domain")

    result = resolve_active_domain(domains_tree)
    assert result == "beta-domain"


def test_pin_beats_alphabetical(domains_tree: Path) -> None:
    """Pin file names a domain → that domain wins over alphabetical order.

    No env var set. Pin says 'beta-domain'. Alphabetical first would be
    'alpha-domain'. Pin must win → result is 'beta-domain'.
    """
    pin = domains_tree / PIN_FILENAME
    pin.write_text("beta-domain", encoding="utf-8")

    result = resolve_active_domain(domains_tree)
    assert result == "beta-domain"


def test_alphabetical_fallback_when_no_env_no_pin(domains_tree: Path) -> None:
    """No env, no pin → first domain alphabetically.

    'alpha-domain' sorts before 'beta-domain' → result is 'alpha-domain'.
    """
    result = resolve_active_domain(domains_tree)
    assert result == "alpha-domain"


def test_none_when_domains_absent(tmp_path: Path) -> None:
    """No domains/ directory → None (legitimate 'no domain yet' state)."""
    result = resolve_active_domain(tmp_path / "nonexistent")
    assert result is None


def test_none_when_domains_empty(tmp_path: Path) -> None:
    """Empty domains/ → None."""
    domains = tmp_path / "domains"
    domains.mkdir()
    result = resolve_active_domain(domains)
    assert result is None


def test_env_pointing_at_nonexistent_domain_falls_through(
    domains_tree: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """ENV var names a domain that doesn't exist → fall through to pin/alphabetical.

    Env says 'ghost-domain' (not in domains/). No pin. Should fall back to
    alphabetical first = 'alpha-domain'.
    """
    monkeypatch.setenv(ENV_VAR, "ghost-domain")
    result = resolve_active_domain(domains_tree)
    assert result == "alpha-domain"


def test_pin_pointing_at_nonexistent_domain_falls_through(domains_tree: Path) -> None:
    """Pin file names a domain that doesn't exist → fall through to alphabetical.

    Pin says 'ghost-domain' (not in domains/). No env. Should fall back to
    alphabetical first = 'alpha-domain'.
    """
    pin = domains_tree / PIN_FILENAME
    pin.write_text("ghost-domain", encoding="utf-8")
    result = resolve_active_domain(domains_tree)
    assert result == "alpha-domain"


def test_underscore_dirs_excluded(tmp_path: Path) -> None:
    """Directories starting with '_' are private/scaffold, never active."""
    domains = tmp_path / "domains"
    domains.mkdir()
    (domains / "_private").mkdir()
    (domains / "real-domain").mkdir()
    result = resolve_active_domain(domains)
    assert result == "real-domain"


def test_pin_file_path_returns_correct_location(domains_tree: Path) -> None:
    """pin_file_path(domains_root) returns <domains_root>/.active-domain."""
    result = pin_file_path(domains_tree)
    assert result == domains_tree / PIN_FILENAME
    assert result.name == PIN_FILENAME


def test_empty_pin_falls_through(domains_tree: Path) -> None:
    """Pin file exists but is empty/whitespace → fall through to alphabetical."""
    pin = domains_tree / PIN_FILENAME
    pin.write_text("  \n  ", encoding="utf-8")
    result = resolve_active_domain(domains_tree)
    assert result == "alpha-domain"


def test_env_whitespace_stripped(domains_tree: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    """ENV var with surrounding whitespace → stripped before matching."""
    monkeypatch.setenv(ENV_VAR, "  beta-domain  ")
    result = resolve_active_domain(domains_tree)
    assert result == "beta-domain"
