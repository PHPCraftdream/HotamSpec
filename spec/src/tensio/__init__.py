"""Canon: executable methodology for the lifecycle of contradictory requirements.

Tensio models a body of business requirements NOT as a truth (a single
non-contradictory canon) but as a TENSION GRAPH: many requirements, constantly
changing, mutually contradictory, each resting on assumptions with their own
lifecycle. The goal is the INVERSE of a consistency proof: not to eliminate
contradictions but to guarantee we never fail to SEE them, and to keep them
visible over time as first-class, owned, history-bearing objects.

THE CENTRAL INVERSION (vs the dev-coin reference):
  dev-coin proves CONSISTENCY — one canon, drift forbidden, conflicts closed
  forever (0 open mechanisms). Tensio does the OPPOSITE — it makes contradictions
  visible and KEEPS them visible. A contradiction is never silently "fixed"; it
  is a node that transitions through a lifecycle under a human steward.

THE CENTRAL INSIGHT — Conflict is a connector NODE, not an edge:
  A naive model makes conflict an edge `conflicts_with` between R-87 and R-203.
  That edge holds nothing: remove it and the requirements fall back into
  isolation. Instead a Conflict is a first-class NODE (a mediator) through which
  two otherwise-unconnectable requirements first come to lie in one structure:
  R-87 -> C-12 <- R-203. C-12 carries knowledge belonging to NEITHER member:
    - the TENSION AXIS — along what dimension they diverge (latency vs
      completeness, cost vs flexibility, privacy vs analytics). The axis is born
      only from their meeting.
    - the SHARED CONTEXT — the scenario in which they actually collide (outside
      it they may coexist peacefully).
    - the SHARED ASSUMPTION they interpret differently — often the real root.
  Therefore "to surface a contradiction" technically = to MATERIALIZE the missing
  connector node. The detector's job is redefined: not "find conflicts" but
  "find requirement pairs that SHOULD have a C-node but don't" — latent
  connectors. This catches the invisible, not the already-recorded.

THE THREE INVISIBILITIES the methodology surfaces:
  1. Direct contradictions — two requirements that cannot both hold
     ("<200ms" vs "full synchronous compliance check"). Machine-catchable.
  2. Hidden dependencies — A silently relies on an assumption B negates.
     Contradiction THROUGH a chain — needs a graph, not a list.
  3. Context drift — a requirement meaningful under assumption X, X long false,
     nobody revisited it. Contradiction WITH time — catchable only because
     requirements carry assumptions with their own lifecycle.

THE CLOSED LOOP (why an agent is never lost — the operating procedure):
  State (graph + generated docs + test status)
    -> Diagnosis (tools/what_now.py)
    -> Next-action (typed, prioritized)
    -> Action (edit the graph)
    -> regenerate (tools/gen_spec.py)
    -> State.
  An agent dropped into the repo in ANY state runs `what_now` and deterministically
  derives the next correct action. "Being lost is structurally impossible" is the
  generalization of dev-coin's "drift is structurally impossible".

THE AI'S THREE ROLES + THE HARD BOUNDARY:
  - Detector — proposes materializing a missing C-node ("R-114 tensions with this
    along latency-vs-completeness; here is a scenario where both break").
  - Socratic partner — surfaces hidden assumptions, never resolves.
  - Historian — recalls decision rationale + revisit-conditions that have triggered.
  HARD BOUNDARY: the AI NEVER closes a conflict silently. It presents, justifies,
  asks. The decision and its recording stay with the human steward — otherwise
  invisibility returns, now AI-created.

STORAGE: the store IS the Python code (like dev-coin's params.py). Objects are
frozen dataclasses; edges are tuple-of-id fields; traversal is plain functions;
invariants are boolean check_* functions; anti-drift is generator + meta-test.
No RDF/SHACL/Postgres.

CONTENT-FREE FRAMEWORK: the package itself ships ZERO business data — no example
requirements, no example axes. Tensio is a blank kit; the framework hosts the
ontology, the invariants, the generator and the harness. A real domain is loaded
from `spec/content/graph.py` exposing `build_graph() -> TensionGraph`; an empty
content slot is the legitimate ship state. The worked example lives outside the
framework, in `spec/tests/fixtures/seed.py`, and is loaded only via the explicit
`--demo` flag of the tools or by the tests.

Package structure (module = ontology section / methodology chapter):
  stakeholder — Stakeholder: who owns requirements and stewards conflicts.
  axis        — Axis: one entry of the controlled tension vocabulary.
  assumption  — Assumption: a claim with its own lifecycle (HOLDS/DEAD/UNCERTAIN).
  requirement — Requirement: the claim, its assumptions, owner, typed relations.
  conflict    — Conflict: the first-class connector NODE (axis, context, steward).
  graph       — TensionGraph container + content loader + traversal helpers
                (no business data here; load_content_graph() reads spec/content/).
  invariants  — structural graph invariants (check_* functions returning the
                violation list): the form of the tension graph that must always
                hold (a stewardless conflict, a dangling member, an OPEN with no
                question — all FAIL here).

CANON-SECTION SCHEME (every public object carries a `Canon: §<name>` label):
  §Requirement, §Conflict, §Assumption, §Axis, §Stakeholder — the ontology;
  §Invariants — the structural rules;
  §Graph — the store and its traversal;
  §Loop — the what_now operating procedure (documented, exercised by the harness);
  §Glossary — the controlled methodology vocabulary (tensio.glossary.TERMS).
  §Constitution — the operator's boot sequence generated from the SETTLED laws;
                  a fresh agent reads this to reconstitute as operator without
                  needing a session checkpoint (M33 resolved — P7).
The generator (tools/gen_spec.py) walks modules in a fixed order and emits the
human layer (REQUIREMENTS.md, TENSIONS.md, OPEN.md, GLOSSARY.md,
CONSTITUTION.md); the meta-test (tests/test_docs_gen.py) makes regeneration ==
committed, byte-for-byte.

OPERATOR / SUBSTRATE CONCEPTS (deferred layers, terminology anchored here):
  operator — an acting agent that owns a bounded sub-domain of the graph; its
             crystallized substrate is the durable store free of context cost.
  DRIFT_FALLOUT — a DEAD assumption with live dependents that must be revisited.
  latent connector — a requirement pair that SHOULD have a Conflict node but
             doesn't; the heuristic hunt lives in graph.latent_connector_suspects.
  three-cipher pulse — the operator's per-turn vital-signs anchored in R-boot-from-substrate
             (context % + top what_now action + unenforced-SETTLED+DRAFT debt count).
  boot ritual — the per-turn sequence (CLAUDE.md§Operator boot ritual) that re-loads
             the operator from the substrate so every reply is grounded in the live
             substrate rather than session memory.
"""
