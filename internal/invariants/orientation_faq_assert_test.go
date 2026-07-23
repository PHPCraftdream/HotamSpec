package invariants

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// orientationFAQAssertFixture mirrors orientationFAQFixture (orientation_faq_test.go)
// but writes the supplied orientation_faq JSON fragment (which may include
// "assert" entries) and lets the caller supply a fully-populated graph
// (carrying Requirements/Conflicts/GateSignoffs) rather than deriving graph
// content from disk — the assert eval reads LIVE in-memory graph state, not
// anything written to the manifest.
func orientationFAQAssertFixture(t *testing.T, manifestOrientationFAQ, crystal string, gateStageOrder []string) string {
	t.Helper()
	return orientationFAQAssertFixtureWithCohort(t, manifestOrientationFAQ, crystal, gateStageOrder, "")
}

// orientationFAQAssertFixtureWithCohort is orientationFAQAssertFixture's
// twin, additionally accepting a raw "gate_cohort" JSON fragment (e.g.
// `{"statuses":["SETTLED"]}`) to embed in the written manifest.json — "" (the
// default orientationFAQAssertFixture always passes) omits the field
// entirely, the honest-no-op backward-compat path.
func orientationFAQAssertFixtureWithCohort(t *testing.T, manifestOrientationFAQ, crystal string, gateStageOrder []string, gateCohortJSON string) string {
	t.Helper()
	tmp := t.TempDir()
	domainDir := filepath.Join(tmp, "domains", "testdomain")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("MkdirAll domainDir: %v", err)
	}
	gateOrderJSON := "[]"
	if len(gateStageOrder) > 0 {
		gateOrderJSON = `["` + gateStageOrder[0] + `"`
		for _, s := range gateStageOrder[1:] {
			gateOrderJSON += `,"` + s + `"`
		}
		gateOrderJSON += `]`
	}
	gateCohortField := ""
	if gateCohortJSON != "" {
		gateCohortField = `,
  "gate_cohort": ` + gateCohortJSON
	}
	manifest := `{
  "purpose": "test domain",
  "parent": null,
  "gate_stage_order": ` + gateOrderJSON + `,
  "orientation_faq": ` + manifestOrientationFAQ + gateCohortField + `
}
`
	if err := os.WriteFile(filepath.Join(domainDir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("WriteFile manifest.json: %v", err)
	}
	if crystal != "" {
		if err := os.WriteFile(filepath.Join(domainDir, "CLAUDE.md"), []byte(crystal), 0o644); err != nil {
			t.Fatalf("WriteFile CLAUDE.md: %v", err)
		}
	}
	return domainDir
}

// TestCheckOrientationFAQAnswered_AssertExpectAll_LivePass: a
// requirement_count_by_status assert with expect="all" passes when every
// Requirement in the live graph actually has that status.
func TestCheckOrientationFAQAnswered_AssertExpectAll_LivePass(t *testing.T) {
	t.Parallel()
	faq := `[{"question": "all settled?", "assert": {"kind": "requirement_count_by_status", "status": "SETTLED", "expect": "all"}}]`
	crystal := "# Crystal\n"
	domainDir := orientationFAQAssertFixture(t, faq, crystal, nil)
	g := &ontology.Graph{
		DomainDir: domainDir,
		Requirements: []ontology.Requirement{
			{ID: "R-1", Status: ontology.StatusSETTLED},
			{ID: "R-2", Status: ontology.StatusSETTLED},
		},
	}
	if vs := runCheck(t, "check_orientation_faq_answered", g); len(vs) != 0 {
		t.Fatalf("expected 0 violations when every requirement is SETTLED and expect=all, got %v", vs)
	}
}

// TestCheckOrientationFAQAnswered_AssertExpectEq_FailsWhenGraphMovedPast is
// the pinned-count regression: expect={"op":"eq","value":N} must fail once
// the live graph's count has moved past N (the graph grew/shrank since the
// assert was authored).
func TestCheckOrientationFAQAnswered_AssertExpectEq_FailsWhenGraphMovedPast(t *testing.T) {
	t.Parallel()
	faq := `[{"question": "exactly 27 settled?", "assert": {"kind": "requirement_count_by_status", "status": "SETTLED", "expect": {"op": "eq", "value": 27}}}]`
	crystal := "# Crystal\n"
	domainDir := orientationFAQAssertFixture(t, faq, crystal, nil)
	// Live graph now has 32 SETTLED requirements, not the pinned 27.
	reqs := make([]ontology.Requirement, 32)
	for i := range reqs {
		reqs[i] = ontology.Requirement{ID: "R", Status: ontology.StatusSETTLED}
	}
	g := &ontology.Graph{DomainDir: domainDir, Requirements: reqs}
	vs := runCheck(t, "check_orientation_faq_answered", g)
	if len(vs) != 1 {
		t.Fatalf("expected 1 violation when live count (32) has moved past the pinned expect value (27), got %d: %v", len(vs), vs)
	}
	if !hasViolationFor(vs, "exactly 27 settled?") {
		t.Errorf("expected the violation to name the question, got %v", vs)
	}
}

// TestCheckOrientationFAQAnswered_AssertPhrase_StalePhraseFails is THE
// regression case task #321 exists to prevent: crystal/answer text says
// "27/32" (a stale phrase), but the graph's live state is 32/32 — a phrase
// assert with {count}/{total} placeholders MUST fail, proving this closes
// the exact bug class where a keyword-only check would have kept passing
// forever (the phrase "27/32" and "32" both stay lexically present -- the
// keyword-only signal cannot detect this at all).
func TestCheckOrientationFAQAnswered_AssertPhrase_StalePhraseFails(t *testing.T) {
	t.Parallel()
	faq := `[{"question": "how many FT settled?", "assert": {"kind": "requirement_count_by_status", "status": "SETTLED", "phrase": "{count} of {total}"}}]`
	// The crystal's static prose is STALE: it says "27 of 32", but the live
	// graph (below) has moved to 32 of 32.
	crystal := "# Crystal\n\n27 of 32 FT complete.\n"
	domainDir := orientationFAQAssertFixture(t, faq, crystal, nil)
	reqs := make([]ontology.Requirement, 32)
	for i := range reqs {
		reqs[i] = ontology.Requirement{ID: "R", Status: ontology.StatusSETTLED}
	}
	g := &ontology.Graph{DomainDir: domainDir, Requirements: reqs}
	vs := runCheck(t, "check_orientation_faq_answered", g)
	if len(vs) != 1 {
		t.Fatalf("expected 1 violation: live phrase should be '32 of 32', crystal says stale '27 of 32', got %d: %v", len(vs), vs)
	}
	if !hasViolationFor(vs, "how many FT settled?") {
		t.Errorf("expected the violation to name the question, got %v", vs)
	}

	// Positive counterpart: when the crystal's prose IS the live phrase, the
	// assert passes.
	freshCrystal := "# Crystal\n\n32 of 32 FT complete.\n"
	if err := os.WriteFile(filepath.Join(domainDir, "CLAUDE.md"), []byte(freshCrystal), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if vs := runCheck(t, "check_orientation_faq_answered", g); len(vs) != 0 {
		t.Fatalf("expected 0 violations once the crystal's phrase matches the live '32 of 32' state, got %v", vs)
	}
}

// TestCheckOrientationFAQAnswered_AssertUnknownKindFailsClosed covers an
// unrecognized assert.kind — must fail closed with a named violation, never
// silently pass.
func TestCheckOrientationFAQAnswered_AssertUnknownKindFailsClosed(t *testing.T) {
	t.Parallel()
	faq := `[{"question": "bogus kind", "assert": {"kind": "not_a_real_kind", "expect": "all"}}]`
	crystal := "# Crystal\n"
	domainDir := orientationFAQAssertFixture(t, faq, crystal, nil)
	g := &ontology.Graph{DomainDir: domainDir}
	vs := runCheck(t, "check_orientation_faq_answered", g)
	if len(vs) != 1 {
		t.Fatalf("expected 1 violation for an unknown assert.kind, got %d: %v", len(vs), vs)
	}
	if !hasViolationFor(vs, "bogus kind") {
		t.Errorf("expected the violation to name the question, got %v", vs)
	}
}

// TestCheckOrientationFAQAnswered_AssertNoExpectOrPhraseFailsClosed covers
// an assert with neither "expect" nor "phrase" declared — no checkable
// predicate at all.
func TestCheckOrientationFAQAnswered_AssertNoExpectOrPhraseFailsClosed(t *testing.T) {
	t.Parallel()
	faq := `[{"question": "empty assert", "assert": {"kind": "requirement_count_by_status", "status": "SETTLED"}}]`
	crystal := "# Crystal\n"
	domainDir := orientationFAQAssertFixture(t, faq, crystal, nil)
	g := &ontology.Graph{DomainDir: domainDir, Requirements: []ontology.Requirement{{ID: "R-1", Status: ontology.StatusSETTLED}}}
	vs := runCheck(t, "check_orientation_faq_answered", g)
	if len(vs) != 1 {
		t.Fatalf("expected 1 violation for an assert with neither expect nor phrase, got %d: %v", len(vs), vs)
	}
}

// TestCheckOrientationFAQAnswered_AssertGateSignoffCount_StageNotDeclaredFailsClosed
// covers a gate_signoff_count assert naming a stage that is NOT present in
// the domain's declared gate_stage_order.
func TestCheckOrientationFAQAnswered_AssertGateSignoffCount_StageNotDeclaredFailsClosed(t *testing.T) {
	t.Parallel()
	faq := `[{"question": "gate progress", "assert": {"kind": "gate_signoff_count", "stage": "P-G99", "expect": "all"}}]`
	crystal := "# Crystal\n"
	domainDir := orientationFAQAssertFixture(t, faq, crystal, []string{"P-G0", "P-G1"})
	g := &ontology.Graph{DomainDir: domainDir}
	vs := runCheck(t, "check_orientation_faq_answered", g)
	if len(vs) != 1 {
		t.Fatalf("expected 1 violation for a stage not present in gate_stage_order, got %d: %v", len(vs), vs)
	}
}

// TestCheckOrientationFAQAnswered_AssertGateSignoffCount_LivePass proves the
// gate_signoff_count dispatch actually reaches query.GateSignoffTally and
// respects the last-entry-per-Requirement dedup rule end-to-end through the
// invariant layer.
func TestCheckOrientationFAQAnswered_AssertGateSignoffCount_LivePass(t *testing.T) {
	t.Parallel()
	faq := `[{"question": "gate P-G1 all signed?", "assert": {"kind": "gate_signoff_count", "stage": "P-G1", "expect": "all"}}]`
	crystal := "# Crystal\n"
	domainDir := orientationFAQAssertFixture(t, faq, crystal, []string{"P-G0", "P-G1"})
	g := &ontology.Graph{
		DomainDir: domainDir,
		Requirements: []ontology.Requirement{
			{ID: "R-1", GateSignoffs: []ontology.GateSignoff{
				{Stage: "P-G1", State: ontology.GateSignoffStateDeferred, DeferredReason: "x"},
				{Stage: "P-G1", State: ontology.GateSignoffStateSigned},
			}},
			{ID: "R-2", GateSignoffs: []ontology.GateSignoff{{Stage: "P-G1", State: ontology.GateSignoffStateSigned}}},
		},
	}
	if vs := runCheck(t, "check_orientation_faq_answered", g); len(vs) != 0 {
		t.Fatalf("expected 0 violations: both requirements resolve SIGNED at P-G1 (R-1's dedup'd last entry + R-2), got %v", vs)
	}
}

// TestCheckOrientationFAQAnswered_AssertConflictLifecycle_LivePass exercises
// the conflict_count_by_lifecycle dispatch end-to-end.
func TestCheckOrientationFAQAnswered_AssertConflictLifecycle_LivePass(t *testing.T) {
	t.Parallel()
	faq := `[{"question": "any unresolved?", "assert": {"kind": "conflict_count_by_lifecycle", "lifecycle_class": "UNRESOLVED", "expect": "none"}}]`
	crystal := "# Crystal\n"
	domainDir := orientationFAQAssertFixture(t, faq, crystal, nil)
	g := &ontology.Graph{
		DomainDir: domainDir,
		Conflicts: []ontology.Conflict{
			{ID: "C-1", Lifecycle: ontology.ConflictDECIDEDPrefix},
			{ID: "C-2", Lifecycle: ontology.ConflictHELDPrefix},
		},
	}
	if vs := runCheck(t, "check_orientation_faq_answered", g); len(vs) != 0 {
		t.Fatalf("expected 0 violations: no UNRESOLVED conflicts and expect=none, got %v", vs)
	}
}

// TestCheckOrientationFAQAnswered_AssertAdditiveWithKeywords proves assert
// and keywords are ADDITIVE, not alternatives: an entry with BOTH must have
// BOTH pass. Keywords alone would pass; assert alone would pass; but if
// assert fails, the whole entry must fail even though keywords are present
// and correct.
func TestCheckOrientationFAQAnswered_AssertAdditiveWithKeywords(t *testing.T) {
	t.Parallel()
	faq := `[{"question": "purpose and count", "keywords": ["tension graph"], "assert": {"kind": "requirement_count_by_status", "status": "SETTLED", "expect": {"op": "eq", "value": 99}}}]`
	// Keywords ARE inline (would pass alone), but the pinned assert (99) does
	// not match the live count (1) — the entry must still fail.
	crystal := "# Crystal\n\nThis is a tension graph.\n"
	domainDir := orientationFAQAssertFixture(t, faq, crystal, nil)
	g := &ontology.Graph{DomainDir: domainDir, Requirements: []ontology.Requirement{{ID: "R-1", Status: ontology.StatusSETTLED}}}
	vs := runCheck(t, "check_orientation_faq_answered", g)
	if len(vs) != 1 {
		t.Fatalf("expected 1 violation: keywords pass but assert fails, and assert is ADDITIVE not alternative, got %d: %v", len(vs), vs)
	}

	// Positive counterpart: fix the assert to match live state (1) -- now
	// BOTH signals pass, entry passes.
	fixedFAQ := `[{"question": "purpose and count", "keywords": ["tension graph"], "assert": {"kind": "requirement_count_by_status", "status": "SETTLED", "expect": {"op": "eq", "value": 1}}}]`
	domainDir2 := orientationFAQAssertFixture(t, fixedFAQ, crystal, nil)
	g2 := &ontology.Graph{DomainDir: domainDir2, Requirements: []ontology.Requirement{{ID: "R-1", Status: ontology.StatusSETTLED}}}
	if vs := runCheck(t, "check_orientation_faq_answered", g2); len(vs) != 0 {
		t.Fatalf("expected 0 violations once both keywords and assert pass, got %v", vs)
	}
}

// TestCheckOrientationFAQAnswered_AssertNilIsBackwardCompatible proves an
// entry with Assert == nil behaves byte-for-byte like the pre-assert
// keyword-only path: this is the SAME fixture/assertion shape
// TestCheckOrientationFAQAnswered_PassesWhenKeywordsInline (orientation_faq_test.go)
// already uses, re-asserted here explicitly as the backward-compat proof
// for this task's change.
func TestCheckOrientationFAQAnswered_AssertNilIsBackwardCompatible(t *testing.T) {
	t.Parallel()
	faq := `[{"question": "what is this?", "keywords": ["tension graph", "purpose"]}]`
	crystal := "# Crystal\n\nThis project is a tension graph. Its purpose is orientation.\n"
	domainDir := orientationFAQAssertFixture(t, faq, crystal, nil)
	g := &ontology.Graph{DomainDir: domainDir}
	if vs := runCheck(t, "check_orientation_faq_answered", g); len(vs) != 0 {
		t.Fatalf("expected 0 violations for a plain keyword-only entry (Assert nil), got %v", vs)
	}
}

// --- task #330 (R4-cohort): gate_cohort denominator + pipeline_run guard ---

// TestCheckOrientationFAQAnswered_GateCohort_NeverEvaluatedMemberFailsAll is
// THE core regression test this task exists to prove: with a gate_cohort
// declared (statuses:["SETTLED"]), a cohort member that carries NO
// gate-signoff record AT ALL at the target stage (never evaluated, neither
// SIGNED nor DEFERRED) must make expect:"all" FAIL. Before task #330's fix,
// total was Signed+Deferred — a never-evaluated Requirement was invisible to
// that sum, so expect:"all" could silently pass with this exact graph shape
// (2 of 3 cohort members SIGNED, the third never assessed) — this test
// proves that bug is closed.
func TestCheckOrientationFAQAnswered_GateCohort_NeverEvaluatedMemberFailsAll(t *testing.T) {
	t.Parallel()
	faq := `[{"question": "gate P-G1 all cohort members signed?", "assert": {"kind": "gate_signoff_count", "stage": "P-G1", "expect": "all"}}]`
	crystal := "# Crystal\n"
	domainDir := orientationFAQAssertFixtureWithCohort(t, faq, crystal, []string{"P-G0", "P-G1"}, `{"statuses":["SETTLED"]}`)
	g := &ontology.Graph{
		DomainDir: domainDir,
		Requirements: []ontology.Requirement{
			{ID: "R-1", Status: ontology.StatusSETTLED, GateSignoffs: []ontology.GateSignoff{{Stage: "P-G1", State: ontology.GateSignoffStateSigned}}},
			{ID: "R-2", Status: ontology.StatusSETTLED, GateSignoffs: []ontology.GateSignoff{{Stage: "P-G1", State: ontology.GateSignoffStateSigned}}},
			// R-3 is a SETTLED cohort member with NO gate-signoff record at
			// P-G1 at all — never evaluated. The pre-fix denominator
			// (Signed+Deferred=2) would be blind to this and expect:"all"
			// would wrongly pass (2==2). The cohort denominator
			// (CohortCount of SETTLED=3) makes it visible: 2 != 3, fails.
			{ID: "R-3", Status: ontology.StatusSETTLED},
		},
	}
	vs := runCheck(t, "check_orientation_faq_answered", g)
	if len(vs) != 1 {
		t.Fatalf("expected 1 violation: R-3 is a cohort member never evaluated at P-G1, so 2 of 3 != all, got %d: %v", len(vs), vs)
	}
	if !hasViolationFor(vs, "gate P-G1 all cohort members signed?") {
		t.Errorf("expected the violation to name the question, got %v", vs)
	}
}

// TestCheckOrientationFAQAnswered_GateCohort_AbsentFallsBackToSignedPlusDeferred
// proves that with NO gate_cohort declared, behavior is byte-identical to
// the pre-task-#330 total=Signed+Deferred semantics — this is the exact
// fixture/graph shape TestCheckOrientationFAQAnswered_AssertGateSignoffCount_LivePass
// already exercises, re-run here to pin the backward-compat contract
// explicitly for this task.
func TestCheckOrientationFAQAnswered_GateCohort_AbsentFallsBackToSignedPlusDeferred(t *testing.T) {
	t.Parallel()
	faq := `[{"question": "gate P-G1 all signed?", "assert": {"kind": "gate_signoff_count", "stage": "P-G1", "expect": "all"}}]`
	crystal := "# Crystal\n"
	domainDir := orientationFAQAssertFixture(t, faq, crystal, []string{"P-G0", "P-G1"})
	g := &ontology.Graph{
		DomainDir: domainDir,
		Requirements: []ontology.Requirement{
			{ID: "R-1", GateSignoffs: []ontology.GateSignoff{
				{Stage: "P-G1", State: ontology.GateSignoffStateDeferred, DeferredReason: "x"},
				{Stage: "P-G1", State: ontology.GateSignoffStateSigned},
			}},
			{ID: "R-2", GateSignoffs: []ontology.GateSignoff{{Stage: "P-G1", State: ontology.GateSignoffStateSigned}}},
			// R-3 carries NO gate-signoff at all — but since NO gate_cohort
			// is declared, it is simply invisible to the old
			// Signed+Deferred denominator too, exactly like before task
			// #330 — expect:"all" still passes (2 Signed == total 2).
			{ID: "R-3"},
		},
	}
	if vs := runCheck(t, "check_orientation_faq_answered", g); len(vs) != 0 {
		t.Fatalf("expected 0 violations: no gate_cohort declared, so total=Signed+Deferred=2 and count=Signed=2, byte-identical to pre-#330 behavior, got %v", vs)
	}
}

// TestCheckOrientationFAQAnswered_GateCohort_ExcludeTypoFailsClosed proves an
// "exclude" list entry naming a Requirement id that does not exist in the
// live graph fails closed with a named violation, rather than silently
// matching nothing.
func TestCheckOrientationFAQAnswered_GateCohort_ExcludeTypoFailsClosed(t *testing.T) {
	t.Parallel()
	faq := `[{"question": "gate P-G1 all cohort members signed?", "assert": {"kind": "gate_signoff_count", "stage": "P-G1", "expect": "all"}}]`
	crystal := "# Crystal\n"
	domainDir := orientationFAQAssertFixtureWithCohort(t, faq, crystal, []string{"P-G0", "P-G1"}, `{"statuses":["SETTLED"],"exclude":["R-does-not-exist"]}`)
	g := &ontology.Graph{
		DomainDir: domainDir,
		Requirements: []ontology.Requirement{
			{ID: "R-1", Status: ontology.StatusSETTLED, GateSignoffs: []ontology.GateSignoff{{Stage: "P-G1", State: ontology.GateSignoffStateSigned}}},
		},
	}
	vs := runCheck(t, "check_orientation_faq_answered", g)
	if len(vs) != 1 {
		t.Fatalf("expected 1 violation for an exclude id that matches no Requirement in the graph, got %d: %v", len(vs), vs)
	}
	if !hasViolationFor(vs, "gate P-G1 all cohort members signed?") {
		t.Errorf("expected the violation to name the question, got %v", vs)
	}
}

// TestCheckOrientationFAQAnswered_GateCohort_RejectedNotCounted proves a
// REJECTED requirement is correctly excluded from a statuses:["SETTLED"]
// cohort — the exact gpsm-sm shape (R-domain-exists REJECTED, legitimately
// out of cohort).
func TestCheckOrientationFAQAnswered_GateCohort_RejectedNotCounted(t *testing.T) {
	t.Parallel()
	faq := `[{"question": "gate P-G1 all cohort members signed?", "assert": {"kind": "gate_signoff_count", "stage": "P-G1", "expect": "all"}}]`
	crystal := "# Crystal\n"
	domainDir := orientationFAQAssertFixtureWithCohort(t, faq, crystal, []string{"P-G0", "P-G1"}, `{"statuses":["SETTLED"]}`)
	g := &ontology.Graph{
		DomainDir: domainDir,
		Requirements: []ontology.Requirement{
			{ID: "R-1", Status: ontology.StatusSETTLED, GateSignoffs: []ontology.GateSignoff{{Stage: "P-G1", State: ontology.GateSignoffStateSigned}}},
			// REJECTED, no gate signoff at all — must NOT count toward the
			// SETTLED-only cohort, so total stays 1 (just R-1) and the
			// assert still passes.
			{ID: "R-2", Status: ontology.StatusREJECTED},
		},
	}
	if vs := runCheck(t, "check_orientation_faq_answered", g); len(vs) != 0 {
		t.Fatalf("expected 0 violations: R-2 is REJECTED, not a SETTLED cohort member, so total=1 (R-1 only) and count=1, got %v", vs)
	}
}

// TestCheckOrientationFAQAnswered_MultiRun_AmbiguousStageFailsClosedWithoutDeclaredRun
// proves the multi-pipeline-run guard: a stage carrying signoffs from two
// distinct pipeline_run values, with no PipelineRun declared on the assert,
// must fail closed rather than silently conflating the runs.
func TestCheckOrientationFAQAnswered_MultiRun_AmbiguousStageFailsClosedWithoutDeclaredRun(t *testing.T) {
	t.Parallel()
	faq := `[{"question": "gate P-G1 all signed?", "assert": {"kind": "gate_signoff_count", "stage": "P-G1", "expect": "all"}}]`
	crystal := "# Crystal\n"
	domainDir := orientationFAQAssertFixture(t, faq, crystal, []string{"P-G0", "P-G1"})
	g := &ontology.Graph{
		DomainDir: domainDir,
		Requirements: []ontology.Requirement{
			{ID: "R-1", GateSignoffs: []ontology.GateSignoff{{Stage: "P-G1", State: ontology.GateSignoffStateSigned, PipelineRun: "run-a"}}},
			{ID: "R-2", GateSignoffs: []ontology.GateSignoff{{Stage: "P-G1", State: ontology.GateSignoffStateSigned, PipelineRun: "run-b"}}},
		},
	}
	vs := runCheck(t, "check_orientation_faq_answered", g)
	if len(vs) != 1 {
		t.Fatalf("expected 1 violation: stage P-G1 has signoffs from 2 distinct pipeline_runs and no PipelineRun declared, got %d: %v", len(vs), vs)
	}
	if !hasViolationFor(vs, "gate P-G1 all signed?") {
		t.Errorf("expected the violation to name the question, got %v", vs)
	}
}

// TestCheckOrientationFAQAnswered_MultiRun_DeclaredRunTalliesOnlyThatRun is
// the positive counterpart: the SAME ambiguous-stage graph as the previous
// test, but with PipelineRun declared — the assert must tally ONLY that
// run's signoffs and pass.
func TestCheckOrientationFAQAnswered_MultiRun_DeclaredRunTalliesOnlyThatRun(t *testing.T) {
	t.Parallel()
	faq := `[{"question": "gate P-G1 all signed in run-a?", "assert": {"kind": "gate_signoff_count", "stage": "P-G1", "pipeline_run": "run-a", "expect": "all"}}]`
	crystal := "# Crystal\n"
	domainDir := orientationFAQAssertFixture(t, faq, crystal, []string{"P-G0", "P-G1"})
	g := &ontology.Graph{
		DomainDir: domainDir,
		Requirements: []ontology.Requirement{
			{ID: "R-1", GateSignoffs: []ontology.GateSignoff{{Stage: "P-G1", State: ontology.GateSignoffStateSigned, PipelineRun: "run-a"}}},
			// R-2 only has a run-b signoff — invisible when this assert
			// targets run-a, so it does not count against run-a's total.
			{ID: "R-2", GateSignoffs: []ontology.GateSignoff{{Stage: "P-G1", State: ontology.GateSignoffStateSigned, PipelineRun: "run-b"}}},
		},
	}
	if vs := runCheck(t, "check_orientation_faq_answered", g); len(vs) != 0 {
		t.Fatalf("expected 0 violations: pipeline_run=run-a declared, tallies only run-a's 1 SIGNED (count=1,total=1), got %v", vs)
	}
}

// TestCheckOrientationFAQAnswered_AssertState_DeferredCountsDeferredNotSigned
// proves Assert.State: "DEFERRED" makes the assert's "count" read
// tally.Deferred instead of tally.Signed.
func TestCheckOrientationFAQAnswered_AssertState_DeferredCountsDeferredNotSigned(t *testing.T) {
	t.Parallel()
	faq := `[{"question": "gate P-G1 how many deferred?", "assert": {"kind": "gate_signoff_count", "stage": "P-G1", "state": "DEFERRED", "expect": {"op": "eq", "value": 1}}}]`
	crystal := "# Crystal\n"
	domainDir := orientationFAQAssertFixture(t, faq, crystal, []string{"P-G0", "P-G1"})
	g := &ontology.Graph{
		DomainDir: domainDir,
		Requirements: []ontology.Requirement{
			{ID: "R-1", GateSignoffs: []ontology.GateSignoff{{Stage: "P-G1", State: ontology.GateSignoffStateSigned}}},
			{ID: "R-2", GateSignoffs: []ontology.GateSignoff{{Stage: "P-G1", State: ontology.GateSignoffStateDeferred, DeferredReason: "x"}}},
		},
	}
	if vs := runCheck(t, "check_orientation_faq_answered", g); len(vs) != 0 {
		t.Fatalf("expected 0 violations: state=DEFERRED reads count=tally.Deferred=1, matching expect eq 1, got %v", vs)
	}

	// Negative counterpart: pin count at 2 (wrong — only 1 is DEFERRED) to
	// prove state=DEFERRED is actually being read, not silently ignored in
	// favor of Signed (which is also 1, so a sloppy test could pass by
	// accident without this negative check).
	faqWrong := `[{"question": "gate P-G1 how many deferred?", "assert": {"kind": "gate_signoff_count", "stage": "P-G1", "state": "DEFERRED", "expect": {"op": "eq", "value": 2}}}]`
	domainDir2 := orientationFAQAssertFixture(t, faqWrong, crystal, []string{"P-G0", "P-G1"})
	g2 := &ontology.Graph{DomainDir: domainDir2, Requirements: g.Requirements}
	vs := runCheck(t, "check_orientation_faq_answered", g2)
	if len(vs) != 1 {
		t.Fatalf("expected 1 violation: only 1 requirement is DEFERRED, not the pinned 2, got %d: %v", len(vs), vs)
	}
}

// TestCheckOrientationFAQAnswered_AssertState_UnrecognizedFailsClosed proves
// an unrecognized Assert.State value fails closed.
func TestCheckOrientationFAQAnswered_AssertState_UnrecognizedFailsClosed(t *testing.T) {
	t.Parallel()
	faq := `[{"question": "bogus state", "assert": {"kind": "gate_signoff_count", "stage": "P-G1", "state": "BOGUS", "expect": "all"}}]`
	crystal := "# Crystal\n"
	domainDir := orientationFAQAssertFixture(t, faq, crystal, []string{"P-G0", "P-G1"})
	g := &ontology.Graph{
		DomainDir:    domainDir,
		Requirements: []ontology.Requirement{{ID: "R-1", GateSignoffs: []ontology.GateSignoff{{Stage: "P-G1", State: ontology.GateSignoffStateSigned}}}},
	}
	vs := runCheck(t, "check_orientation_faq_answered", g)
	if len(vs) != 1 {
		t.Fatalf("expected 1 violation for an unrecognized assert.state, got %d: %v", len(vs), vs)
	}
	if !hasViolationFor(vs, "bogus state") {
		t.Errorf("expected the violation to name the question, got %v", vs)
	}
}
