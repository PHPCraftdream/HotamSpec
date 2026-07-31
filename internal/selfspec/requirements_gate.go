// Origin: one-time bulk extraction by .scratch/selfspec-codegen (task #345, RAC-A) from
// domains/hotam-spec-self/graph.json. That program was a THROWAWAY (run once, never
// committed to git) and is NOT a regeneration path -- it went graph->Go, the OPPOSITE of
// today's sync direction. These literals are HAND-MAINTAINED Go source: edit them directly,
// then mirror Go->graph via `hotam sync-self` (dry-run by default; --confirm-hash to write).
// The "generated" framing describes this file's ORIGIN (task #345's one-time bulk pull from
// the pre-existing JSON graph), not an ongoing regeneration workflow.
//
// Topic: Land and Gate (tiered commit gate, land protocol).
//
// STRUCTURAL fields only: Claim, Why, Owner, Status, Relations, Assumptions,
// Enforcement, EnforcedBy, Enforceability, MTag, Summary, CreatedAt, SettledAt,
// BlockedOn, ImplementedBy, VerifiedBy, SourceRefs, DeclOrder. EVENT fields
// (History, GateSignoffs, LastReviewedAt, ReviewAfter, Evidence) are
// deliberately absent — MergeIntoGraph passes them through from the graph's
// existing node untouched.
package selfspec

import "github.com/PHPCraftdream/HotamSpec/internal/ontology"

var _ = Requirements.MustRegister("R-commit-boundary-checkable", ontology.Requirement{
	ID:             "R-commit-boundary-checkable",
	Claim:          "A gate-status command shall answer, from a runtime land-log, whether a full T2 verification has landed at-or-after the most recent T1-gated land, exiting 0 (boundary satisfied) or 1 (boundary not satisfied, printing the unverified T1-gated targets) -- this is the mechanically checkable SLICE of R-tiered-gate-not-a-commit-gate's claim; it does not itself verify that a resolver runs it, nor detect an imminent commit, nor replace R-tiered-gate-not-a-commit-gate's human-invoked procedural discipline.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "R-tiered-gate-not-a-commit-gate is honestly INHERENTLY_PROSE: no check_* can force a human to run the full suite before `git commit`. But the trace introduced by R-land-tier-trace makes ONE part of that claim mechanically answerable after the fact -- whether the log shows a covering T2 run. Splitting this into its own atom (rather than flipping R-tiered-gate-not-a-commit-gate to ENFORCED) keeps both claims honest: the parent claim stays INHERENTLY_PROSE because it genuinely cannot be machine-verified end to end (the human-invocation half is unreachable by any test); this new atom claims only the reachable half. Not yet implemented: the gate-status command is Planned (internal/methodology/tools_data.go); historically implemented as spec/tools/gate_status.py reading spec/.runtime/land-log.jsonl. settled_at RESTORED 2026-07-13 from git HEAD 9af0176 (pre-corruption value) after a mutate.go bug this session silently reset it on content-only updates; see task #84.",
	Assumptions:    []string{"A-python-stack"},
	Relations:      []ontology.Relation{{Kind: "refines", Target: "R-tiered-gate-not-a-commit-gate"}, {Kind: "refines", Target: "R-land-tier-trace"}},
	Enforcement:    "PROSE",
	EnforcedBy:     []string{},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-07-02",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{"internal/methodology/tools_data.go"},
	DeclOrder:      213,
	BlockedOn:      "blocked on the gate_status tool (Planned) + the land-log it reads (absent)",
})

var _ = Requirements.MustRegister("R-gate-cohort-explicit-denominator", ontology.Requirement{
	ID:             "R-gate-cohort-explicit-denominator",
	Claim:          "A `gate_signoff_count` orientation_faq assert (loader.OrientationFAQAssert) that ALSO declares a manifest-level `gate_cohort` ({\"statuses\": [...], \"exclude\": [...]}) answers a STRONGER question than the assert's pre-existing bare form: 'have ALL cohort requirements passed this stage' -- a cohort member that was NEVER ASSESSED at the target stage (no GateSignoff record at all, neither SIGNED nor DEFERRED) MUST count AGAINST `expect:\"all\"`, never be silently invisible to it. Absent a declared `gate_cohort`, the assert's denominator stays `total = tally.Signed + tally.Deferred` (today's semantics, byte-identical -- a Requirement never evaluated at the stage is invisible to that sum by construction, and this weaker form remains valid for a domain that has not opted into an explicit cohort declaration). Separately, a `gate_signoff_count` assert MUST tally FAIL CLOSED -- refuse to silently conflate distinct pipeline executions -- when its target stage carries GateSignoffs from more than one distinct `pipeline_run` and the assert itself does not declare which `pipeline_run` it means; declaring `pipeline_run` on the assert disambiguates and tallies only that one run's signoffs.",
	Owner:          "framework-author",
	Status:         "DRAFT",
	Why:            "Task #330 (R4-cohort), fourth external review: `gate_signoff_count`'s pre-existing denominator (`tally.Signed + tally.Deferred`) is blind to a Requirement with NO gate-signoff record at all at a stage -- it was never evaluated, so it contributes to neither Signed nor Deferred, and `expect:\"all\"` can pass while that requirement was silently never assessed. Confirmed live in `gpsm-sm`: 35 requirements total, only 32 carry any gate signoff at all at the relevant stage -- the other 3 (`R-domain-exists` REJECTED, `R-gpsm-pg1-passage`/`R-gpsm-project` SETTLED meta-requirements describing the pipeline itself rather than passing through it) are legitimately out of cohort, which is exactly why the simpler alternative (`total = all Requirements` regardless of role) was rejected during this task's design consult in favor of an explicit, domain-declared cohort. `loader.ResolveGateCohort` (`internal/loader/gate_cohort.go`) reads the optional manifest `gate_cohort` field (statuses default `[\"SETTLED\"]` when declared-but-empty); `graphfacts.CohortCount` (`internal/graphfacts/facts.go`) is the trivial counted-filter extraction (matching this package's own 'extracted so divergent reimplementations can't creep' discipline, mirroring `lastSignoffAtStage`'s identical rationale for the gate-tally dedup rule); `evalOrientationAssert` (`internal/invariants/orientation_faq_assert.go`) wires the cohort denominator in when declared, fails closed on an `exclude` id that matches no real Requirement or a `statuses` entry that is not a recognized status (reusing `graphfacts.RequirementStatusTally`'s own exact-match + OPEN-prefix matching rule, not a new one), and also wires the previously-unused `OrientationFAQAssert.State` field (`\"\"`/`\"SIGNED\"` byte-identical default vs `\"DEFERRED\"`). The multi-run half closes a second, related gap: `GateSignoff.PipelineRun` was already mandatory/populated on every signoff but `graphfacts` silently conflated every run's signoffs when tallying a stage -- `lastSignoffAtStage`/`GateSignoffTally` now take a `run` parameter (`\"\"` = all runs = the exact pre-existing behavior, still the default for `internal/generator/pipeline.go`'s Live-state renderer and `internal/generator/claudemd.go`'s DOMAIN-MAP renderer, both unchanged), and a new `graphfacts.PipelineRunsAtStage` plus `OrientationFAQAssert.PipelineRun` field let an assert either disambiguate a genuinely multi-run stage or fail closed when it does not. Actually declaring `gate_cohort` in `gpsm-sm`'s own manifest.json (activating the stronger check there) is intentionally a SEPARATE follow-up, out of this requirement's scope -- this requirement claims only the engine-portion capability now existing and regression-tested.",
	Assumptions:    nil,
	Relations:      []ontology.Relation{{Kind: "refines", Target: "R-orientation-faq-answerable"}},
	Enforcement:    "PROSE",
	EnforcedBy:     nil,
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-07-23",
	SettledAt:      "",
	SourceRefs:     []string{"internal/loader/gate_cohort.go", "internal/loader/orientation_faq.go", "internal/graphfacts/facts.go", "internal/invariants/orientation_faq_assert.go"},
	DeclOrder:      0,
	ImplementedBy:  []string{"internal/graphfacts/facts.go:CohortCount", "internal/invariants/orientation_faq_assert.go:evalOrientationAssert"},
	VerifiedBy:     []string{"internal/invariants/orientation_faq_assert_test.go:TestCheckOrientationFAQAnswered_GateCohort_NeverEvaluatedMemberFailsAll"},
})

var _ = Requirements.MustRegister("R-gate-signoff-single-carrier", ontology.Requirement{
	ID:             "R-gate-signoff-single-carrier",
	Claim:          "A domain that runs its Requirements through a staged review methodology (e.g. prat/gpsm-sm's P-G0..P-G4 planning gates) has ONE typed carrier on the graph recording per-Requirement, per-stage gate passage -- Requirement.gate_signoffs -- instead of that fact being reconstructed from prose scattered across History entries, manifest goals, and Process why-fields (a drift that has already recurred once in this framework's own consumer domains). Within one Requirement and one pipeline_run, SIGNED gate_signoffs MUST appear in the domain-declared gate_stage_order (a manifest.json opt-in DATA field, never an engine-known enum -- the engine serves domains with no staged-gate discipline at all) -- no stage SIGNED before every earlier stage is SIGNED too. Every DEFERRED gate_signoff MUST carry a non-empty deferred_reason, and when that reason references a Conflict id (C-[0-9a-f]{8}), that id MUST resolve to a real Conflict node in the graph. Every SIGNED gate_signoff MUST carry a populated signoff (decided_by + verbatim) and non-empty evidence, and signoff.decided_by, when present, MUST resolve to a known Stakeholder.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "SETTLED: an external review of the gpsm-sm consumer domain found its 'current gate-passage stage' scattered across graph.json History entries (hand-written after each test run), manifest.json goals, and a Process why-field -- three independent text sources that had already drifted out of sync with each other (fixed by hand in this same wave, task D1). The structural fix is Requirement.GateSignoffs (internal/ontology/gate_signoff.go): a typed, append-only list of {stage, state, deferred_reason, evidence, pipeline_run, signoff} entries, landed exclusively via the new ProposedGateSignoffBatch proposal kind (internal/proposal/types.go) so History entries are AUTO-GENERATED from the structural diff (internal/proposal/history.go's summarizeFieldDiff, the SAME mechanism already used for Requirement/EntityType field updates) rather than hand-written -- closing the exact drift class the review found. Stage is a bare string, not an engine enum: the engine also serves hotam-spec-self and hotam-dev, which have no staged-gate methodology at all, so a domain that wants monotonic-order checking declares gate_stage_order itself (internal/loader/gate_stage_order.go, the same tolerant opt-in-resolver shape as ResolveOrientationFAQ/ResolveDiscipline -- honest no-op for every domain that never declares it, exactly like check_orientation_faq_answered's identical boundary). The three checks -- check_gate_signoff_monotonic, check_gate_signoff_deferred_reason_present, check_gate_signoff_deferred_conflict_resolves (internal/invariants/gate_signoff_checks.go) -- make 'gate passage is monotonic and every deferral has a real, resolvable reason' a mechanically verified property of the graph rather than a convention a resolver must remember to honor by hand across dozens of Requirements. This domain (hotam-spec-self) itself declares no gate_stage_order and carries no gate_signoffs, so all three checks are honest no-ops here -- this requirement exists to anchor the checks for the bijection discipline (R-bijection-r-to-enforcer), not to claim hotam-spec-self uses the feature. Task #319 (R3-signoff-strict, external review) added two more checks anchored here -- check_gate_signoff_signed_has_provenance and check_gate_signoff_decided_by_is_known_stakeholder (internal/invariants/gate_signoff_checks.go): before this task the engine allowed a SIGNED gate_signoff to land with ZERO provenance (no decided_by, no verbatim, no evidence) beyond the bare stage/state/pipeline_run fields -- a real weakness for any domain (e.g. the planned life-domain work) where gate signoffs represent genuine human decisions. Both checks are ongoing all-violations invariants (like check_gate_signoff_deferred_reason_present), not proposal-time-only, mirroring that existing choice; verified safe against prat/gpsm-sm sibling-repo existing landed data (64 SIGNED gate_signoffs, all already carrying full decided_by/verbatim/evidence) before being wired up as ongoing rather than proposal-time-only. This domain (hotam-spec-self) still declares no gate_stage_order and carries no gate_signoffs, so all five checks remain honest no-ops here -- this requirement continues to exist only to anchor the checks for the bijection discipline (R-bijection-r-to-enforcer), not to claim hotam-spec-self uses the feature.",
	Assumptions:    []string{},
	Relations:      []ontology.Relation{},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"check_gate_signoff_monotonic", "check_gate_signoff_deferred_reason_present", "check_gate_signoff_deferred_conflict_resolves", "check_gate_signoff_signed_has_provenance", "check_gate_signoff_decided_by_is_known_stakeholder"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-07-21",
	SettledAt:      "2026-07-21",
	SourceRefs:     []string{"internal/ontology/gate_signoff.go", "internal/loader/gate_stage_order.go", "internal/invariants/gate_signoff_checks.go", "internal/proposal/types.go", "internal/proposal/history.go"},
	DeclOrder:      0,
})

var _ = Requirements.MustRegister("R-land-gate-tier-selector", ontology.Requirement{
	ID:             "R-land-gate-tier-selector",
	Claim:          "hotam gate (internal/gate.SelectTier1), when invoked for a target anchor, shall resolve the T1 targeted-enforcer subset from the target's enforced_by tuple (union of resolved check_*/Test* names) instead of requiring the full test suite for that lookup.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "Reworded 2026-07-12 to describe what is actually implemented and tested: internal/gate.SelectTier1 (internal/gate/gate.go) correctly resolves a target's enforced_by into the union of concrete Go check_*/Test* enforcers -- TestSelectTier1_CheckNameResolves, TestSelectTier1_TestFuncNameResolves, TestSelectTier1_MultipleChecksUnion. The prior claim asserted this selection ran 'by default' inside the LAND pipeline (historically tools/apply_proposal.py consulting tools/gate.py) -- that automatic wiring was never carried through: hotam land (cmd/hotam/land.go) runs apply -> gen-spec -> all-violations and does not call gate.SelectTier1 at all; the selector is reachable only via the standalone `hotam gate <anchor>` command (cmd/hotam/gate_cmd.go). This is the honest, narrower claim the code actually guarantees. The lost intent -- wiring T1 selection as LAND's default verify tier, to avoid the full-suite time tax on every proposal -- is preserved as its own open item, not silently dropped: see R-t1-selector-as-land-default (DRAFT). Decision reached via agent consultation (fm), per R-ai-presents-not-decides / R-decided-needs-human-signoff, after an earlier content-refresh batch flagged the mismatch as a found-issue.",
	Assumptions:    []string{"A-python-stack"},
	Relations:      []ontology.Relation{{Kind: "refines", Target: "R-verify-closure-per-action"}},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"TestSelectTier1_CheckNameResolves", "TestSelectTier1_TestFuncNameResolves", "TestSelectTier1_MultipleChecksUnion"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-07-02",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{"internal/gate/gate.go", "cmd/hotam/gate_cmd.go"},
	DeclOrder:      206,
})

var _ = Requirements.MustRegister("R-land-gate-tier-selector-fails-closed", ontology.Requirement{
	ID:             "R-land-gate-tier-selector-fails-closed",
	Claim:          "internal/gate.SelectTier1 shall fall back to Confident=false (fail-closed, signaling the caller to run the full suite) on any selection uncertainty: an unknown/unresolvable target anchor, an empty enforced_by tuple, a Conflict target (no per-instance enforced_by), or any enforced_by entry that cannot be resolved to a concrete Go check_*/Test* function -- never returning a partial or best-effort subset in an uncertain case.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "Reworded 2026-07-12 to describe the real, tested contract of internal/gate.SelectTier1 (internal/gate/gate.go), scoped to what the selector itself decides: TestSelectTier1_FailClosed_TargetNotFound, TestSelectTier1_FailClosed_EmptyEnforcedBy, TestSelectTier1_FailClosed_ConflictTarget, TestSelectTier1_FailClosed_PythonTestPathUnresolved, TestSelectTier1_FailClosed_BareTestFuncUnresolved, TestSelectTier1_FailClosed_PartiallyUnresolved, TestSelectTier1_FailClosed_CheckWithNoTestCoverage. The prior claim additionally asserted fail-closed behavior for apply()-level cases -- a ProposedRejection target, or the creation of a brand-new Requirement/Conflict node -- that were decisions historically made by tools/apply_proposal.py::apply before consulting the selector. Those apply-level cases are not implemented at all today (hotam land does not call the selector, so there is no apply()-level fail-closed decision to make); a ProposedRejection target that already exists in the graph would in fact resolve confidently via its own enforced_by, which is a real behavioral gap, not just a wording gap. That lost intent -- fail-closed treatment for rejection/creation cases once gate is wired into land -- is preserved, not silently dropped: see R-t1-selector-as-land-default (DRAFT). Decision reached via agent consultation (fm), per R-ai-presents-not-decides / R-decided-needs-human-signoff.",
	Assumptions:    []string{"A-python-stack"},
	Relations:      []ontology.Relation{{Kind: "refines", Target: "R-land-gate-tier-selector"}},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"TestSelectTier1_FailClosed_TargetNotFound", "TestSelectTier1_FailClosed_EmptyEnforcedBy", "TestSelectTier1_FailClosed_ConflictTarget", "TestSelectTier1_FailClosed_PythonTestPathUnresolved", "TestSelectTier1_FailClosed_BareTestFuncUnresolved", "TestSelectTier1_FailClosed_PartiallyUnresolved", "TestSelectTier1_FailClosed_CheckWithNoTestCoverage"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-07-02",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{"internal/gate/gate.go", "cmd/hotam/gate_cmd.go"},
	DeclOrder:      207,
})

var _ = Requirements.MustRegister("R-land-is-transactional", ontology.Requirement{
	ID:             "R-land-is-transactional",
	Claim:          "hotam land shall leave a domain's graph.json and generated docs mutually consistent even when a later stage (doc regeneration or the post-regen violations check) fails after the proposal apply stage already wrote a new graph.json — a failure rolls back to the pre-land state rather than leaving the graph and docs divergent.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "TaskList P1-77 / external review flagged a concrete bug in the pre-fix cmd/hotam/land.go:86: step (a) apply writes a new graph.json to disk, but if step (b) gen-spec or step (c) all-violations then fails, the command returned early and left that NEW graph.json sitting next to STALE (pre-land) docs/gen/*.md — the exact drift `hotam land` exists to prevent, now created BY a failed land instead of prevented by a successful one. The fix is a compensating-rollback design, not two-phase commit on the filesystem: cmdLand snapshots graph.json + graph.lock (snapshotGraphFiles) before step (a) ever writes; if step (b) or (c) fails, rollbackLand restores those exact pre-land bytes and then re-runs genSpec. This is sufficient (not just a partial fix) specifically because genSpec is a PURE function of the on-disk graph.json — it calls loadDomainGraph fresh every invocation and deterministically re-renders every doc from it, so restoring the graph and re-rendering restores the docs to the identical pre-land content too, without needing to snapshot docs/gen/*.md separately. A first-ever land into a brand-new domain (graph.json/graph.lock absent pre-land) rolls back to absence via restoreGraphFile's remove-if-absent path, keeping rollback idempotent and correct even at that boundary. Proven 2026-07-13 by three tests in cmd/hotam/land_test.go: TestCmdLand_GenSpecFailure_RollsBackGraphJSON forces a genSpec failure after a successful apply via a --claude-md path pointed at a directory, and proves graph.json is byte-identical to the pre-land baseline and graph.lock is removed again (pre-land state was absent); TestRollbackLand_RestoresFilesAndRegeneratesDocs unit-tests the rollbackLand helper directly, proving the docs are re-rendered to baseline content (not just the graph file restored) after a simulated post-apply, post-gen-spec failure; TestCmdLand_SuccessPathDoesNotRollBack is the regression guard proving the happy path still lands normally and does NOT spuriously roll back (graph.lock remains, new node present in both graph.json and docs) — together these prove the rollback fires exactly when needed and never when not. This is the SAME situation as the sibling R-land-tier-trace/-best-effort/-skips-dry-run requirements (also `hotam land` CLI-orchestration behavior), except those three remain genuinely unimplemented (no runtime land-log writer exists yet in cmd/hotam), so they stay PROSE correctly — this requirement is the one case among the wave-6 closeable-debt list where the enforcer already existed but the resolver could not see it. ENFORCED 2026-07-13: TestCmdLand_GenSpecFailure_RollsBackGraphJSON, TestRollbackLand_RestoresFilesAndRegeneratesDocs, TestCmdLand_SuccessPathDoesNotRollBack in cmd/hotam/land_test.go, resolver widened to cover cmd/ this session (task #97) — internal/gate.TestFuncNames (mechanism #1, the Test*-name half of enforced_by resolution) now walks both internal/**/*_test.go and cmd/**/*_test.go, so a real Go test enforcer living in cmd/hotam is no longer structurally invisible to check_enforced_by_resolvable / hotam gate just because of which directory it happens to live in. Mechanism #2 (the check_* literal -> tests map) stays scoped to internal/invariants only, per its own unchanged reasoning.",
	Assumptions:    nil,
	Relations:      []ontology.Relation{{Kind: "refines", Target: "R-active-loop-apply-tool"}},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"TestCmdLand_GenSpecFailure_RollsBackGraphJSON", "TestRollbackLand_RestoresFilesAndRegeneratesDocs", "TestCmdLand_SuccessPathDoesNotRollBack"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-07-13",
	SettledAt:      "2026-07-13",
	SourceRefs:     nil,
	DeclOrder:      0,
})

var _ = Requirements.MustRegister("R-land-tier-trace", ontology.Requirement{
	ID:             "R-land-tier-trace",
	Claim:          "Every applied proposal that reaches the LAND verify step shall append its verification tier (T1 targeted or T2 full-suite), selected test node-ids (or the literal 'full'), and verify/closure outcome to a runtime land-log, written AFTER the verify step so the record states what actually ran.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "Not yet implemented: there is no land-log.jsonl (or any runtime land-trace file) written by hotam land or any other CLI command. (Historically the Python apply_proposal.py appended tier + selected pytest node-ids + outcome to spec/.runtime/land-log.jsonl strictly AFTER the verify step, so the record describes what was actually verified, never a plan that could still fail -- the reference design for a future implementation.) The rationale remains valid: making the tier decision visible answers, after the fact, which tier a given land used -- mirroring the spawn-log.jsonl precedent (same .runtime/ directory, same append-only JSONL discipline, same gitignored-not-committed-substrate status). The gate_status tool (internal/methodology/tools_data.go) is Planned: 'Not implemented. Historically: reads the runtime land-log and answers the commit-boundary question.' RE-ATOMIZED Wave 8 move 2 (2026-07-03): this atom now carries only the record-shape+timing promise. PROSE (not ENFORCED): no enforcer exists. settled_at RESTORED 2026-07-13 from git HEAD 9af0176 (pre-corruption value) after a mutate.go bug this session silently reset it on content-only updates; see task #84.",
	Assumptions:    []string{"A-python-stack"},
	Relations:      []ontology.Relation{{Kind: "refines", Target: "R-land-gate-tier-selector"}},
	Enforcement:    "PROSE",
	EnforcedBy:     []string{},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-07-02",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{"internal/methodology/tools_data.go"},
	DeclOrder:      212,
	BlockedOn:      "blocked on the .runtime/land-log.jsonl writer (absent) + the gate_status tool (Planned)",
})

var _ = Requirements.MustRegister("R-land-tier-trace-best-effort", ontology.Requirement{
	ID:             "R-land-tier-trace-best-effort",
	Claim:          "A broken or unwritable spec/.runtime/land-log.jsonl location shall never fail an otherwise-green apply -- the write is best-effort, warn only.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "Split from R-land-tier-trace (Wave 8 move 2 atomicity pass, 2026-07-03) -- a distinct behavioral rule of the same land-log mechanism: the trace is diagnostic, not load-bearing for correctness, so a filesystem problem writing the log must never turn a successful proposal apply into a failure. Independently enforced by test_land_log_write_failure_is_best_effort.",
	Assumptions:    []string{"A-python-stack"},
	Relations:      []ontology.Relation{{Kind: "refines", Target: "R-land-tier-trace"}},
	Enforcement:    "PROSE",
	EnforcedBy:     []string{},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-07-03",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{},
	DeclOrder:      234,
	BlockedOn:      "blocked on the .runtime/land-log.jsonl writer (absent)",
})

var _ = Requirements.MustRegister("R-land-tier-trace-skips-dry-run", ontology.Requirement{
	ID:             "R-land-tier-trace-skips-dry-run",
	Claim:          "A dry-run proposal shall never write a spec/.runtime/land-log.jsonl record.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "Split from R-land-tier-trace (Wave 8 move 2 atomicity pass, 2026-07-03) -- a distinct behavioral rule of the same land-log mechanism: a --dry-run apply never reaches a real verify step, so a log entry would misleadingly claim a tier ran when nothing was actually landed. This is the exemption half of the trace contract, independently enforced by test_dry_run_writes_no_log.",
	Assumptions:    []string{"A-python-stack"},
	Relations:      []ontology.Relation{{Kind: "refines", Target: "R-land-tier-trace"}},
	Enforcement:    "PROSE",
	EnforcedBy:     []string{},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-07-03",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{},
	DeclOrder:      233,
	BlockedOn:      "blocked on the .runtime/land-log.jsonl writer (absent)",
})

var _ = Requirements.MustRegister("R-t1-selector-as-land-default", ontology.Requirement{
	ID:             "R-t1-selector-as-land-default",
	Claim:          "hotam land shall, by default, invoke internal/gate.SelectTier1 for its target anchor and run only the resolved T1 enforcer subset (falling back to the full suite on any selector uncertainty, per R-land-gate-tier-selector-fails-closed's contract) instead of relying solely on all-violations, so each proposal apply pays only the verification cost its own enforced_by tuple demands.",
	Owner:          "framework-author",
	Status:         "DRAFT",
	Why:            "Preserves an intent that was about to be silently lost. R-land-gate-tier-selector and R-land-gate-tier-selector-fails-closed originally claimed this wiring existed (Python-era wording: 'the per-proposal LAND verify step ... shall, by default, run the T1 targeted-enforcer subset'); that was never true in the Go port and both were reworded 2026-07-12 to describe only the standalone hotam gate selector's own contract, dropping the 'wired into land by default' assertion because it wasn't real. But the underlying design intent -- avoid the full-suite time tax on every proposal apply, since most proposals touch exactly one node whose enforced_by already names its enforcer(s) -- is still a live, undecided idea, not a rejected one. Held open here as its own DRAFT rather than quietly dropped when the two ENFORCED claims were narrowed (R-conflict-is-connector-node's generative law: important-yet-invisible tension gets a named node, not silent extinguishment). Scope note for whoever settles this: cmd/hotam/land.go's cmdLand would need to call gate.SelectTier1(targetAnchor, g) after apply and, on Confident=true, run `go test -run '^(name1|name2|...)$' ./...` against the resolved node_ids instead of the current always-full all-violations + implicit full go test at commit boundaries; on Confident=false it must still run the full suite (all-violations already does the structural half; the gap is in NOT also running a scoped `go test` subset). Two open questions for the resolver: (1) does 'the full suite' here mean go test ./... or just all-violations (they check different things -- all-violations is structural invariants, go test also runs enforcer test bodies); (2) is a per-land `go test -run` subprocess spawn (Windows subprocess overhead, per the perf note already recorded in R-land-gate-tier-selector's why) actually faster than the current all-violations-only path for the common case, or does it need benchmarking first. Not yet implemented; land's current behavior (apply -> gen-spec -> all-violations, no gate call) is unaffected.",
	Assumptions:    nil,
	Relations:      nil,
	Enforcement:    "PROSE",
	EnforcedBy:     nil,
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-07-12",
	SettledAt:      "",
	SourceRefs:     nil,
	DeclOrder:      0,
})

var _ = Requirements.MustRegister("R-tiered-gate-not-a-commit-gate", ontology.Requirement{
	ID:             "R-tiered-gate-not-a-commit-gate",
	Claim:          "The full go test ./... suite shall remain the mandatory verification gate at wave and commit boundaries -- the T1 targeted-enforcer tier (hotam gate / internal/gate.SelectTier1) is a standalone advisory selection tool, never a substitute for the full-suite run a resolver or wave-closing agent performs before committing.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "Tiering exists to relieve the redundant per-proposal tax, not to weaken the wave/commit-boundary guarantee that the WHOLE graph is still structurally sound after a batch of changes -- a targeted T1 pass on proposal N does not prove proposal N did not regress something proposal N-3 touched. This is inherently a discipline/procedural claim about WHEN each tier is invoked (the Mediation loop step 6 LAND names `go test ./...` as MANDATORY at wave/commit boundaries, vs. the standalone `hotam gate <anchor>` command for per-node T1 selection) rather than a graph-checkable structural property, so it is marked STRUCTURAL / INHERENTLY_PROSE rather than claiming a machine enforcer that cannot honestly exist for a human-involved boundary. CONSISTENT WITH THE GATE-SELECTOR REWORK: the prior claim asserted the T1 tier ran 'by default' inside the LAND step; that automatic wiring does not exist. `hotam land` (cmd/hotam/land.go) runs apply -> gen-spec -> all-violations and does NOT call gate.SelectTier1 at all; the T1 selector is reachable only via the standalone `hotam gate <anchor>` command (cmd/hotam/gate_cmd.go). (Historical note: the original wiring was apply_proposal.py consulting tools/gate.py, invoking pytest; that path was not carried forward.) The lost intent -- wiring T1 selection as LAND's default verify tier -- is preserved as its own open item, not silently dropped: see R-t1-selector-as-land-default (DRAFT) and the reworded R-land-gate-tier-selector / R-land-gate-tier-selector-fails-closed (both narrowed to the standalone selector's own contract). The claim's '--' scope-disclaimer convention (kept from the Wave 8 atomicity pass) marks the clause after '--' as a scope/boundary clause of the SAME rule (defining exactly how far the T1 tier is allowed to reach), not a second independent obligation. No semantic change to the promise.",
	Assumptions:    []string{"A-python-stack"},
	Relations:      []ontology.Relation{{Kind: "refines", Target: "R-verify-closure-per-action"}},
	Enforcement:    "STRUCTURAL",
	EnforcedBy:     []string{},
	MTag:           "",
	Enforceability: "INHERENTLY_PROSE",
	Summary:        "",
	CreatedAt:      "2026-07-02",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{"cmd/hotam/land.go", "cmd/hotam/gate_cmd.go", "internal/gate"},
	DeclOrder:      208,
})
