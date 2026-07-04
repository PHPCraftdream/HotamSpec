"""Canon: §Invariants — Wave 17 framework-vs-domain test tiering guard.

Steward doctrine (verdict #8, verbatim): "Бизнес всегда должен думать, что
фреймворк работает. Быть рабочим — ответственность фреймворка. Его тесты
прогоняются до всего отдельно."

The framework must prove it works INDEPENDENTLY of which business domain is
active. conftest.py tags every collected test as `framework` or `domain`
(the domain-coupled set is the DOMAIN_COUPLED registry). This module guards the
mechanism itself:

  * every test carries exactly one of the two tier markers (no untagged test can
    silently escape the framework guarantee);
  * the two tiers partition the suite (mutually exclusive);
  * the DOMAIN_COUPLED registry references only real test files.

The domain-INDEPENDENCE claim proper (`-m framework` green under any active
domain) is proven by running `pytest -m framework` under a foreign pin
(HOTAM_SPEC_ACTIVE_DOMAIN=hotam-dev) — done at wave/commit boundary, not in a
nested pytest here (which would recurse). This file is the STRUCTURAL enforcer
of the tiering that makes that run meaningful.
"""

from __future__ import annotations

from pathlib import Path

import pytest

from conftest import DOMAIN_COUPLED

_TESTS_DIR = Path(__file__).resolve().parent


def test_every_test_is_tiered(request: pytest.FixtureRequest) -> None:
    """Every collected item carries exactly one of {framework, domain}."""
    items = request.session.items
    assert items, "no tests collected"
    for item in items:
        markers = {m.name for m in item.iter_markers()}
        tiers = markers & {"framework", "domain"}
        assert len(tiers) == 1, (
            f"{item.nodeid} carries tiers {tiers}; exactly one of "
            "{{framework, domain}} required"
        )


def test_tiers_partition_the_suite(request: pytest.FixtureRequest) -> None:
    """framework and domain are mutually exclusive across the whole suite."""
    for item in request.session.items:
        markers = {m.name for m in item.iter_markers()}
        assert not ({"framework", "domain"} <= markers), (
            f"{item.nodeid} is BOTH framework and domain"
        )


def test_domain_coupled_registry_references_real_files() -> None:
    """Every (file, func) in DOMAIN_COUPLED names a test file that exists."""
    assert DOMAIN_COUPLED, "registry unexpectedly empty"
    for fname, func in DOMAIN_COUPLED:
        assert (_TESTS_DIR / fname).is_file(), f"{fname} not found (registry stale)"
        assert func.startswith("test_"), f"{func} is not a test function name"
