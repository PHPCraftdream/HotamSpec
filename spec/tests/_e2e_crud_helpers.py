"""Shared CRUD scenario for consumer e2e subprocess tests.

Both test_e2e_consumer_subprocess.py (editable install) and
test_e2e_wheel_subprocess.py (wheel install) exercise the same full
apply-proposal CRUD cycle:

  1. Create a Requirement (via hotam-apply-proposal)
  2. Update the same Requirement (change enforcement to STRUCTURAL)
  3. Reject an existing Requirement (with replaced_by)
  4. Create a Conflict (needs 2+ stakeholders, axis, 2 member requirements)
  5. Reject an INVALID proposal (bad owner) -- verify nonzero exit AND
     that graph.py is byte-identical before/after (rollback test)

This module provides the scenario as a callable function so neither e2e
test file needs to duplicate 200 lines.
"""

from __future__ import annotations

import json
import subprocess
from pathlib import Path
from typing import Callable


def run_crud_scenario(
    run_cli: Callable[..., "subprocess.CompletedProcess[str]"],
    consumer_dir: Path,
    *,
    graph_py: Path,
) -> None:
    """Drive the full CRUD cycle via hotam-apply-proposal subprocess calls.

    Preconditions (caller must guarantee):
      - hotam-create-domain --activate already run (graph.py exists)
      - At least 2 stakeholders already created (for conflict steward/owner
        distinctness -- the caller's quickstart flow creates them)
      - At least 1 axis already created

    ``run_cli`` is a callable with signature:
        run_cli(command: str, *args: str) -> subprocess.CompletedProcess[str]
    that runs a hotam-* CLI command in the consumer directory.

    ``graph_py`` is the path to the consumer's graph.py file.
    """
    # ---------------------------------------------------------------
    # 1. CREATE a new Requirement
    # ---------------------------------------------------------------
    req_create = {
        "kind": "Requirement",
        "id": "R-crud-test-alpha",
        "claim": "The system shall support CRUD operations.",
        "owner": "alice",
        "status": "SETTLED",
        "enforcement": "PROSE",
        "why": "proves the apply-proposal write path end-to-end",
    }
    p = consumer_dir / "crud-create.json"
    p.write_text(json.dumps(req_create), encoding="utf-8")
    r = run_cli("hotam-apply-proposal", str(p))
    assert r.returncode == 0, (
        f"CREATE requirement failed:\n--- stdout ---\n{r.stdout}\n--- stderr ---\n{r.stderr}"
    )
    # Verify the id landed in graph.py
    graph_text = graph_py.read_text(encoding="utf-8")
    assert "R-crud-test-alpha" in graph_text, (
        f"R-crud-test-alpha not found in graph.py after CREATE.\n"
        f"--- stdout ---\n{r.stdout}\n--- stderr ---\n{r.stderr}"
    )

    # ---------------------------------------------------------------
    # 2. UPDATE the same Requirement (change enforcement to STRUCTURAL)
    # ---------------------------------------------------------------
    req_update = {
        "kind": "Requirement",
        "id": "R-crud-test-alpha",
        "claim": "The system shall support CRUD operations.",
        "owner": "alice",
        "status": "SETTLED",
        "enforcement": "STRUCTURAL",
        "why": "updated: now STRUCTURAL enforcement",
    }
    p = consumer_dir / "crud-update.json"
    p.write_text(json.dumps(req_update), encoding="utf-8")
    r = run_cli("hotam-apply-proposal", str(p))
    assert r.returncode == 0, (
        f"UPDATE requirement failed:\n--- stdout ---\n{r.stdout}\n--- stderr ---\n{r.stderr}"
    )
    graph_text = graph_py.read_text(encoding="utf-8")
    assert "STRUCTURAL" in graph_text, (
        "STRUCTURAL not found in graph.py after UPDATE"
    )

    # ---------------------------------------------------------------
    # 3. CREATE a second Requirement (will be rejected next)
    # ---------------------------------------------------------------
    req_to_reject = {
        "kind": "Requirement",
        "id": "R-crud-test-beta",
        "claim": "The system shall be ephemeral.",
        "owner": "alice",
        "status": "SETTLED",
        "enforcement": "PROSE",
        "why": "will be rejected as a test",
    }
    p = consumer_dir / "crud-create-beta.json"
    p.write_text(json.dumps(req_to_reject), encoding="utf-8")
    r = run_cli("hotam-apply-proposal", str(p))
    assert r.returncode == 0, (
        f"CREATE beta failed:\n--- stdout ---\n{r.stdout}\n--- stderr ---\n{r.stderr}"
    )

    # ---------------------------------------------------------------
    # 4. REJECT R-crud-test-beta with replaced_by
    # ---------------------------------------------------------------
    rejection = {
        "kind": "Rejection",
        "requirement_id": "R-crud-test-beta",
        "reason": "REJECTED -- REPLACES R-crud-test-alpha as the durable requirement",
        "replaced_by": ["R-crud-test-alpha"],
    }
    p = consumer_dir / "crud-reject.json"
    p.write_text(json.dumps(rejection), encoding="utf-8")
    r = run_cli("hotam-apply-proposal", str(p))
    assert r.returncode == 0, (
        f"REJECT requirement failed:\n--- stdout ---\n{r.stdout}\n--- stderr ---\n{r.stderr}"
    )
    graph_text = graph_py.read_text(encoding="utf-8")
    assert "REJECTED" in graph_text, (
        "REJECTED not found in graph.py after rejection"
    )

    # ---------------------------------------------------------------
    # 5. CREATE a Conflict (needs 2 distinct members owned by different
    #    stakeholders, steward != member owners, and a valid axis)
    # ---------------------------------------------------------------
    # First, create a second non-rejected requirement with a different owner
    req_gamma = {
        "kind": "Requirement",
        "id": "R-crud-test-gamma",
        "claim": "The system shall prioritize speed.",
        "owner": "bob",
        "status": "SETTLED",
        "enforcement": "PROSE",
        "why": "second member for conflict test",
    }
    p = consumer_dir / "crud-create-gamma.json"
    p.write_text(json.dumps(req_gamma), encoding="utf-8")
    r = run_cli("hotam-apply-proposal", str(p))
    assert r.returncode == 0, (
        f"CREATE gamma failed:\n--- stdout ---\n{r.stdout}\n--- stderr ---\n{r.stderr}"
    )

    # Now create the conflict. steward=carol, members owned by alice & bob.
    conflict = {
        "kind": "Conflict",
        "axis": "speed-vs-rigor",
        "context": "crud test tension scenario",
        "members": ["R-crud-test-alpha", "R-crud-test-gamma"],
        "steward": "carol",
    }
    p = consumer_dir / "crud-conflict.json"
    p.write_text(json.dumps(conflict), encoding="utf-8")
    r = run_cli("hotam-apply-proposal", str(p))
    assert r.returncode == 0, (
        f"CREATE conflict failed:\n--- stdout ---\n{r.stdout}\n--- stderr ---\n{r.stderr}"
    )
    graph_text = graph_py.read_text(encoding="utf-8")
    assert "crud test tension scenario" in graph_text, (
        "conflict context not found in graph.py after conflict creation"
    )

    # ---------------------------------------------------------------
    # 6. INVALID proposal: owner that does not exist in the graph.
    #    Must fail with nonzero exit code AND graph.py must be
    #    byte-identical before/after (rollback verification).
    # ---------------------------------------------------------------
    graph_before = graph_py.read_bytes()
    invalid_proposal = {
        "kind": "Requirement",
        "id": "R-crud-test-invalid",
        "claim": "This should never land.",
        "owner": "nonexistent-stakeholder-xyz",
        "status": "SETTLED",
        "enforcement": "PROSE",
        "why": "testing rollback on invalid owner",
    }
    p = consumer_dir / "crud-invalid.json"
    p.write_text(json.dumps(invalid_proposal), encoding="utf-8")
    r = run_cli("hotam-apply-proposal", str(p))
    assert r.returncode != 0, (
        f"INVALID proposal should have failed but returned 0:\n"
        f"--- stdout ---\n{r.stdout}\n--- stderr ---\n{r.stderr}"
    )
    graph_after = graph_py.read_bytes()
    assert graph_before == graph_after, (
        "graph.py changed after invalid proposal -- rollback failed!\n"
        f"before: {len(graph_before)} bytes, after: {len(graph_after)} bytes"
    )
    # Check for the REVERTED message
    combined_output = r.stdout + r.stderr
    assert "REVERTED" in combined_output, (
        f"Expected REVERTED message in output, got:\n{combined_output}"
    )
