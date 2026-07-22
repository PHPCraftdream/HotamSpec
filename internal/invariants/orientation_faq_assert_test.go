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
	manifest := `{
  "purpose": "test domain",
  "parent": null,
  "gate_stage_order": ` + gateOrderJSON + `,
  "orientation_faq": ` + manifestOrientationFAQ + `
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
