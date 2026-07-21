<!-- LEGACY (Python-era) — content below records unfreeze events from the pre-port
     Python implementation. Tools and paths (spec/tests/frozen_aspects_baseline.json,
     tools/spawn_agent.py, tools/create_domain.py, apply_proposal.py,
     tools/update_baseline.py, graph.py) are historical references to the retired
     Python codebase. The baseline-guard PRINCIPLE (R-speculative-aspects-frozen)
     remains SETTLED; the specific tool names and Python paths below are artifacts. -->

# Frozen-aspects unfreeze history

Hand-authored (not generated). This file preserves the accumulated
`_comment` history that used to live inline in
`spec/tests/frozen_aspects_baseline.json`. That JSON is a guarded
enforcement-perimeter baseline (`R-speculative-aspects-frozen`,
`R-enforcement-perimeter-baselines-guarded`) — its `_comment` field is now a
short fixed pointer to this file; the full narrative below is the durable
record of every resolver-directed unfreeze.

Each entry documents: what was re-hashed, why (concrete-need trigger per
R-speculative-aspects-frozen), and the scope of what changed vs. what stayed
frozen.

---

Canon: §Invariants -- hash-baseline guard for R-speculative-aspects-frozen.

UPDATED 2026-07-02 (Wave 5): tools/spawn_agent.py partially unfrozen
(isolation/mutating flags, R-spawn-log-carries-isolation).

UPDATED 2026-07-03 (Wave 10, honesty of the spawn seam): tools/spawn_agent.py
further partially unfrozen by explicit resolver act (Wave 10 move 2a
directive) to add the --log-only CLI flag -- a narrow, additive change that
appends a spawn-log row WITHOUT composing the crystal, so a HOST-level spawn
(Task/Agent tool) that never routes through the crystal-composition path can
still leave a trace (R-host-spawn-leaves-trace). audit 2026-07-03: the
spawn-log was empty despite ~30+ real host spawns; --log-only closes that
coverage gap. Backward compatible (default off); the core
agent-resolution/prompt-composition logic remains frozen.

UPDATE (Wave 13, the stranger door): tools/create_domain.py re-hashed after
a resolver-directed unfreeze -- the scaffolded graph.py used inline
TensionGraph(...) kwargs and lacked imports, so apply_proposal.py writers
could not append ANY node (a fresh domain could not accept even its first
Stakeholder/Axis/Requirement/Conflict); and the CLAUDE.md template carried a
FALSE activation instruction (run gen_spec from the domain root to
populate) plus empty LIVE-STATE/CONSTITUTION sentinels gen_spec never
fills. Both are the concrete-need trigger R-speculative-aspects-frozen
names. Fix: named-assignment tuples + pre-declared imports in the graph
template, an honest pointer CLAUDE.md template, and a --activate flag (pins
domains/.active-domain + regenerates the root crystal). Federation surface
otherwise unchanged.

UPDATE (Portability W2, 2026-07-09): tools/create_domain.py and
tools/spawn_agent.py re-hashed after a resolver-directed unfreeze for the
portability migration (R-project-root-not-hardcoded, W2 of the
external-framework requirement) -- both tools switched their domains-root
source from parents[N]/__file__-derivatives to project_paths.domains_root()
so a consumer with a different CWD than the framework install gets their
own domains/ resolved. The change is the source-of-root swap only;
agent-resolution/prompt-composition/scaffold-template logic is unchanged.
Concrete-need trigger: the consumer scenario (HotamSpec installed from
PyPI/git, working in D:/ai_dev/prat) requires domains/ to resolve to the
consumer repo, not the framework install path.

UPDATE (Portability W4, 2026-07-09): tools/spawn_agent.py re-hashed again --
_RUNTIME_DIR source swapped from spec_root()/.runtime (framework-internal)
to runtime_paths.runtime_dir() (consumer-scoped, HOTAM_SPEC_RUNTIME_DIR env
with project_root()/.hotam-spec/runtime default, per section 3.2 of the
portability requirement). Resolver pre-approved this class of change
(source-of-path swap only, no new
agent-resolution/prompt-composition/spawn-log-semantics capability) when
authorizing the W2 unfreeze of this same file; this is the same category
applied to the runtime-dir field. tools/create_domain.py hash unchanged
this wave (not touched).

---

UPDATE (2026-07-09, task #83 B5): the inline `_comment` field in
`frozen_aspects_baseline.json` had grown to ~2.5k characters of accumulated
history, which is legitimate content but does not belong inline in a
guarded machine-checked JSON baseline. Moved verbatim to this file; the
JSON's `_comment` is now a short fixed pointer here. No hash values changed
— this is a comment-only edit performed via the sanctioned
`tools/update_baseline.py` writer (the guard's only approved write path for
these files).
