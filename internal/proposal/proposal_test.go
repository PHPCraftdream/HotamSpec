package proposal

import (
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/generator"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/paths"
)

func TestApply_Requirement_Add(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedRequirement{
		ID:     "R-new",
		Claim:  "a brand new claim",
		Owner:  "sa",
		Status: ontology.StatusDRAFT,
		Why:    "spawned from a decision",
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	g := reload(t, path)
	r, ok := findReq(g, "R-new")
	if !ok {
		t.Fatalf("R-new not present after Apply")
	}
	if r.CreatedAt != today {
		t.Errorf("CreatedAt = %q, want %q (writer-time default)", r.CreatedAt, today)
	}
	if r.Enforcement != ontology.EnforcementPROSE {
		t.Errorf("Enforcement = %q, want PROSE default", r.Enforcement)
	}
}

func TestApply_Requirement_AddEmptyClaimFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedRequirement{ID: "R-new", Claim: "  ", Owner: "sa", Status: ontology.StatusDRAFT}
	assertApplyFails(t, path, p, "'claim'")
}

func TestApply_Requirement_UpdateAppendsHistory(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedRequirement{
		ID:     "R-1",
		Claim:  "revised claim for R-1",
		Owner:  "sa",
		Status: ontology.StatusSETTLED,
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	g := reload(t, path)
	r, ok := findReq(g, "R-1")
	if !ok {
		t.Fatalf("R-1 missing")
	}
	if r.Claim != "revised claim for R-1" {
		t.Errorf("Claim = %q, want revised", r.Claim)
	}
	if r.SettledAt != "" {
		t.Errorf("SettledAt = %q, want preserved empty -- R-1 was already SETTLED before this UPDATE, so a content-only edit must NOT re-stamp settled_at to today (that would erase the real settle date)", r.SettledAt)
	}
	if r.Why != "why R-1" {
		t.Errorf("Why = %q, want preserved original (patch semantics)", r.Why)
	}
	if len(r.History) != 1 {
		t.Fatalf("History len = %d, want 1 derived entry", len(r.History))
	}
	if r.History[0].At != today {
		t.Errorf("History[0].At = %q, want %q", r.History[0].At, today)
	}
}

// TestApply_Requirement_ImplementedByAndVerifiedByAreOrthogonal is the
// carrier test for R-spec-link-embodied-vs-proven's own claim text: "the two
// fields answer two DIFFERENT questions and shall never be merged into a
// single field or treated as aliases of one another." resolveImplementedBy
// and resolveVerifiedBy (internal/proposal/mutate.go) are structurally
// identical wrappers around coalesceSlice (same patch-semantics, same
// clearSentinel handling) -- which makes it easy to accidentally cross-wire
// them (e.g. a copy-paste slip that reads p.ImplementedBy into
// applied.VerifiedBy or vice versa) without any existing test catching it,
// since no prior test named either resolver or exercised the pair together.
// This test proves three things no single-field test can: (1) an UPDATE
// proposal setting ONLY implemented_by leaves the existing verified_by
// completely untouched, (2) an UPDATE proposal setting ONLY verified_by
// leaves the existing implemented_by completely untouched, and (3) the two
// fields clear INDEPENDENTLY via their own clearSentinel -- clearing one
// never clears or otherwise disturbs the other. Together these are exactly
// what "orthogonal, never aliased" means operationally: mutating one
// dimension of a SpecLink cannot leak into or erase the other.
func TestApply_Requirement_ImplementedByAndVerifiedByAreOrthogonal(t *testing.T) {
	t.Parallel()

	seed := baseGraph()
	idx := -1
	for i, r := range seed.Requirements {
		if r.ID == "R-1" {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatalf("fixture R-1 missing from baseGraph")
	}
	seed.Requirements[idx].ImplementedBy = []string{"spec/model/risk.go:NewRisk"}
	seed.Requirements[idx].VerifiedBy = []string{"spec/model/risk_test.go:TestNewRisk"}
	path := writeTempGraph(t, seed)

	// Step 1: UPDATE implemented_by only. verified_by must survive byte-for-byte.
	if err := Apply(path, today, ProposedRequirement{
		ID:            "R-1",
		Claim:         "claim R-1",
		Owner:         "sa",
		Status:        ontology.StatusSETTLED,
		ImplementedBy: []string{"spec/model/risk.go:UpdatedRisk"},
	}); err != nil {
		t.Fatalf("Apply (implemented_by-only update): %v", err)
	}
	g := reload(t, path)
	r, ok := findReq(g, "R-1")
	if !ok {
		t.Fatalf("R-1 missing after implemented_by-only update")
	}
	if got, want := r.ImplementedBy, []string{"spec/model/risk.go:UpdatedRisk"}; len(got) != 1 || got[0] != want[0] {
		t.Errorf("ImplementedBy = %v, want %v", got, want)
	}
	if got, want := r.VerifiedBy, "spec/model/risk_test.go:TestNewRisk"; len(got) != 1 || got[0] != want {
		t.Errorf("VerifiedBy = %v, want unchanged [%q] -- an implemented_by-only update must not touch verified_by (orthogonality)", got, want)
	}

	// Step 2: UPDATE verified_by only. implemented_by (from step 1) must survive.
	if err := Apply(path, today, ProposedRequirement{
		ID:         "R-1",
		Claim:      "claim R-1",
		Owner:      "sa",
		Status:     ontology.StatusSETTLED,
		VerifiedBy: []string{"spec/model/risk_test.go:TestUpdatedRisk"},
	}); err != nil {
		t.Fatalf("Apply (verified_by-only update): %v", err)
	}
	g = reload(t, path)
	r, ok = findReq(g, "R-1")
	if !ok {
		t.Fatalf("R-1 missing after verified_by-only update")
	}
	if got, want := r.VerifiedBy, "spec/model/risk_test.go:TestUpdatedRisk"; len(got) != 1 || got[0] != want {
		t.Errorf("VerifiedBy = %v, want [%q]", got, want)
	}
	if got, want := r.ImplementedBy, "spec/model/risk.go:UpdatedRisk"; len(got) != 1 || got[0] != want {
		t.Errorf("ImplementedBy = %v, want unchanged [%q] -- a verified_by-only update must not touch implemented_by (orthogonality)", got, want)
	}

	// Step 3: clear implemented_by ONLY via the sentinel. verified_by (from
	// step 2) must survive; implemented_by must go empty, not verified_by.
	if err := Apply(path, today, ProposedRequirement{
		ID:            "R-1",
		Claim:         "claim R-1",
		Owner:         "sa",
		Status:        ontology.StatusSETTLED,
		ImplementedBy: []string{clearSentinel},
	}); err != nil {
		t.Fatalf("Apply (implemented_by clear): %v", err)
	}
	g = reload(t, path)
	r, ok = findReq(g, "R-1")
	if !ok {
		t.Fatalf("R-1 missing after implemented_by clear")
	}
	if len(r.ImplementedBy) != 0 {
		t.Errorf("ImplementedBy = %v, want cleared to empty", r.ImplementedBy)
	}
	if got, want := r.VerifiedBy, "spec/model/risk_test.go:TestUpdatedRisk"; len(got) != 1 || got[0] != want {
		t.Errorf("VerifiedBy = %v, want unchanged [%q] -- clearing implemented_by must not clear verified_by (independent clear, not aliased fields)", got, want)
	}
}

// TestApply_Requirement_SettledTransitionStampsSettledAt covers the OTHER
// half of the settled_at contract: a genuine DRAFT -> SETTLED transition
// (not a same-status content edit) DOES stamp settled_at to today, when the
// proposal doesn't supply an explicit settled_at itself.
func TestApply_Requirement_SettledTransitionStampsSettledAt(t *testing.T) {
	t.Parallel()
	g := baseGraph()
	g.Requirements[0].Status = ontology.StatusDRAFT
	g.Requirements[0].SettledAt = ""
	path := writeTempGraph(t, g)

	p := ProposedRequirement{
		ID:     "R-1",
		Claim:  "now settled",
		Owner:  "sa",
		Status: ontology.StatusSETTLED,
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	r, ok := findReq(reload(t, path), "R-1")
	if !ok {
		t.Fatalf("R-1 missing")
	}
	if r.SettledAt != today {
		t.Errorf("SettledAt = %q, want %q (real DRAFT->SETTLED transition)", r.SettledAt, today)
	}
}

// TestApply_Requirement_ExplicitSettledAtAlwaysWins covers the third case:
// an explicit settled_at in the proposal always takes effect, whether or not
// this is a real status transition -- e.g. backdating a requirement whose
// true settle date predates when it was recorded in the graph.
func TestApply_Requirement_ExplicitSettledAtAlwaysWins(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph()) // R-1 already SETTLED, SettledAt ""
	p := ProposedRequirement{
		ID:        "R-1",
		Claim:     "claim R-1",
		Owner:     "sa",
		Status:    ontology.StatusSETTLED,
		SettledAt: "2026-01-01",
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	r, ok := findReq(reload(t, path), "R-1")
	if !ok {
		t.Fatalf("R-1 missing")
	}
	if r.SettledAt != "2026-01-01" {
		t.Errorf("SettledAt = %q, want explicit %q", r.SettledAt, "2026-01-01")
	}
}

// TestApply_Requirement_UpdateLastReviewedAtWithoutEvidenceFails is the
// regression guard for the closed loophole: a plain ProposedRequirement
// UPDATE that sets last_reviewed_at (or review_after) with no evidence must
// be rejected, exactly like a ProposedReviewMark with no evidence is
// (TestApply_ReviewMark_WithoutEvidenceFails). Before this fix,
// ProposedRequirement.mutate coalesced last_reviewed_at/review_after with no
// gate at all, so a routine content-editing UPDATE could silently ride a
// freshness stamp through unattested -- R-requirement-freshness-fields'
// own last_reviewed_at was stamped this way. validate() now requires
// non-empty evidence whenever either field is being set, regardless of
// whether the proposal resolves to CREATE or UPDATE in mutate().
func TestApply_Requirement_UpdateLastReviewedAtWithoutEvidenceFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedRequirement{
		ID:             "R-1",
		Claim:          "claim R-1",
		Owner:          "sa",
		Status:         ontology.StatusSETTLED,
		LastReviewedAt: today,
		// Evidence intentionally omitted.
	}
	assertApplyFails(t, path, p, "evidence")
}

// TestApply_Requirement_UpdateReviewAfterWithoutEvidenceFails covers the
// review_after half of the same gate -- either freshness field alone must
// trigger the evidence requirement, not just last_reviewed_at.
func TestApply_Requirement_UpdateReviewAfterWithoutEvidenceFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedRequirement{
		ID:          "R-1",
		Claim:       "claim R-1",
		Owner:       "sa",
		Status:      ontology.StatusSETTLED,
		ReviewAfter: "2027-01-12",
		// Evidence intentionally omitted.
	}
	assertApplyFails(t, path, p, "evidence")
}

// TestApply_Requirement_UpdateLastReviewedAtWithEvidenceSucceeds proves the
// gate is satisfiable, not just a blanket ban: supplying evidence alongside
// last_reviewed_at on a ProposedRequirement UPDATE still works.
func TestApply_Requirement_UpdateLastReviewedAtWithEvidenceSucceeds(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedRequirement{
		ID:             "R-1",
		Claim:          "claim R-1",
		Owner:          "sa",
		Status:         ontology.StatusSETTLED,
		LastReviewedAt: today,
		Evidence:       []string{"docs/audit-2026-07.md"},
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	r, ok := findReq(reload(t, path), "R-1")
	if !ok {
		t.Fatalf("R-1 missing")
	}
	if r.LastReviewedAt != today {
		t.Errorf("LastReviewedAt = %q, want %q", r.LastReviewedAt, today)
	}
}

// TestApply_Requirement_ContentOnlyUpdateStillWorksWithoutEvidence is the
// preserved-legitimate-path regression guard: the vast majority of UPDATE
// proposals never touch last_reviewed_at/review_after at all (coalesceStr's
// empty-preserves-existing default). Such a plain content-only edit must
// keep working with zero evidence -- the new gate must not fire when neither
// freshness field is being set. This mirrors
// TestApply_Requirement_UpdateAppendsHistory but asserts explicitly on the
// no-evidence-required path so a future change to the gate's condition is
// caught here too.
func TestApply_Requirement_ContentOnlyUpdateStillWorksWithoutEvidence(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedRequirement{
		ID:     "R-1",
		Claim:  "revised claim, no freshness fields touched",
		Owner:  "sa",
		Status: ontology.StatusSETTLED,
		// LastReviewedAt, ReviewAfter, Evidence all intentionally omitted.
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	r, ok := findReq(reload(t, path), "R-1")
	if !ok {
		t.Fatalf("R-1 missing")
	}
	if r.Claim != "revised claim, no freshness fields touched" {
		t.Errorf("Claim = %q, want revised", r.Claim)
	}
	if r.LastReviewedAt != "" {
		t.Errorf("LastReviewedAt = %q, want preserved empty (fixture starts empty, untouched by this proposal)", r.LastReviewedAt)
	}
}

// TestApply_Requirement_CreateWithLastReviewedAtWithoutEvidenceFails covers
// the CREATE path: a brand-new requirement declared already-reviewed must
// also justify last_reviewed_at with evidence -- the gate in validate()
// fires before mutate() branches into CREATE vs UPDATE, so it is
// entry-point-agnostic by construction.
func TestApply_Requirement_CreateWithLastReviewedAtWithoutEvidenceFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedRequirement{
		ID:             "R-new-reviewed",
		Claim:          "a brand new claim, already reviewed at creation",
		Owner:          "sa",
		Status:         ontology.StatusDRAFT,
		LastReviewedAt: today,
	}
	assertApplyFails(t, path, p, "evidence")
}

// TestApply_Requirement_ClearEnforcedBy covers the wave-2 rebind enabler: a
// ProposedRequirement UPDATE whose enforced_by is exactly ["<clear>"] empties
// a previously-populated enforced_by. Without the sentinel this is impossible
// — coalesceSlice treats an empty incoming slice as "preserve old" (patch
// semantics), so downgrading ENFORCED → PROSE/STRUCTURAL could not drop the
// stale enforcer list. See clearSentinel in mutate.go.
func TestApply_Requirement_ClearEnforcedBy(t *testing.T) {
	t.Parallel()
	g := baseGraph()
	g.Requirements[0].Enforcement = ontology.EnforcementENFORCED
	g.Requirements[0].EnforcedBy = []string{"test_legacy.py::test_old", "CRITICAL_CORE_INVARIANTS"}
	path := writeTempGraph(t, g)

	p := ProposedRequirement{
		ID:          "R-1",
		Claim:       "claim R-1",
		Owner:       "sa",
		Status:      ontology.StatusSETTLED,
		Enforcement: ontology.EnforcementPROSE,
		EnforcedBy:  []string{clearSentinel},
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	got := reload(t, path)
	r, ok := findReq(got, "R-1")
	if !ok {
		t.Fatalf("R-1 missing")
	}
	if r.Enforcement != ontology.EnforcementPROSE {
		t.Errorf("Enforcement = %q, want PROSE", r.Enforcement)
	}
	if len(r.EnforcedBy) != 0 {
		t.Errorf("EnforcedBy = %v, want empty (cleared by sentinel)", r.EnforcedBy)
	}
}

func TestApply_Requirement_ClearSentinelMixedWithRealFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedRequirement{
		ID:         "R-1",
		Claim:      "claim R-1",
		Owner:      "sa",
		Status:     ontology.StatusSETTLED,
		EnforcedBy: []string{clearSentinel, "check_typed_anchors"},
	}
	assertApplyFails(t, path, p, clearSentinel)
}

// TestApply_Requirement_ClearBlockedOn covers the wave-6 blocked_on
// close-the-loop fix (review-5 finding N1): an UPDATE proposal whose
// blocked_on is exactly "<clear>" empties a previously-set blocked_on. Without
// the sentinel this is impossible — coalesceStr treats an empty incoming
// value as "preserve old" (patch semantics), so once a requirement is marked
// feature-blocked debt it could never return to closeable-now even after the
// blocking feature ships. See resolveBlockedOn / clearSentinel in mutate.go.
func TestApply_Requirement_ClearBlockedOn(t *testing.T) {
	t.Parallel()
	g := baseGraph()
	g.Requirements[0].BlockedOn = "feature:missing-cli"
	path := writeTempGraph(t, g)

	p := ProposedRequirement{
		ID:        "R-1",
		Claim:     "claim R-1",
		Owner:     "sa",
		Status:    ontology.StatusSETTLED,
		BlockedOn: clearSentinel,
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	got := reload(t, path)
	r, ok := findReq(got, "R-1")
	if !ok {
		t.Fatalf("R-1 missing")
	}
	if r.BlockedOn != "" {
		t.Errorf("BlockedOn = %q, want empty (cleared by sentinel)", r.BlockedOn)
	}
}

// TestApply_Requirement_FlipEnforceabilityToDefaultValueTakesEffect covers a
// real bug found by task W4.2 (prat batch 1): an UPDATE proposal explicitly
// setting Enforceability to ENFORCEABLE (the ontology default value) was a
// silent no-op. The old mutate.go line wrapped p.Enforceability in
// defaultStr(..., ENFORCEABLE) before coalescing against the ENFORCEABLE
// sentinel, so an explicit "ENFORCEABLE" was indistinguishable from
// "omitted" and always collapsed to preserving the OLD value — an
// INHERENTLY_PROSE requirement could never be flipped to ENFORCEABLE via
// apply-proposal, only away from it. Fixed by passing p.Enforceability raw
// with "" as the omitted-sentinel (the same fix already applied to the
// sibling Enforcement field above, for the identical asymmetry).
func TestApply_Requirement_FlipEnforceabilityToDefaultValueTakesEffect(t *testing.T) {
	t.Parallel()
	g := baseGraph()
	g.Requirements[0].Enforceability = ontology.EnforceabilityINHERENTLY_PROSE
	path := writeTempGraph(t, g)

	p := ProposedRequirement{
		ID:             "R-1",
		Claim:          "claim R-1",
		Owner:          "sa",
		Status:         ontology.StatusSETTLED,
		Enforceability: ontology.EnforceabilityENFORCEABLE,
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	got := reload(t, path)
	r, ok := findReq(got, "R-1")
	if !ok {
		t.Fatalf("R-1 missing")
	}
	if r.Enforceability != ontology.EnforceabilityENFORCEABLE {
		t.Errorf("Enforceability = %q, want %q (explicit flip must take effect, not collapse to preserved old value)",
			r.Enforceability, ontology.EnforceabilityENFORCEABLE)
	}
}

// TestApply_Requirement_UpdateOmittedEnforceabilityPreservesExisting pins the
// unchanged patch semantics alongside the fix above: an UPDATE proposal that
// OMITS enforceability (empty string) must still preserve whatever
// enforceability was already set, exactly as blocked_on/why/etc. already do.
func TestApply_Requirement_UpdateOmittedEnforceabilityPreservesExisting(t *testing.T) {
	t.Parallel()
	g := baseGraph()
	g.Requirements[0].Enforceability = ontology.EnforceabilityINHERENTLY_PROSE
	path := writeTempGraph(t, g)

	p := ProposedRequirement{
		ID:     "R-1",
		Claim:  "claim R-1 updated text",
		Owner:  "sa",
		Status: ontology.StatusSETTLED,
		// Enforceability omitted entirely.
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	got := reload(t, path)
	r, ok := findReq(got, "R-1")
	if !ok {
		t.Fatalf("R-1 missing")
	}
	if r.Enforceability != ontology.EnforceabilityINHERENTLY_PROSE {
		t.Errorf("Enforceability = %q, want preserved %q (omitted field must not reset to default)",
			r.Enforceability, ontology.EnforceabilityINHERENTLY_PROSE)
	}
}

// TestApply_Requirement_UpdateOmittedBlockedOnPreservesExisting pins the
// unchanged patch semantics: an UPDATE proposal that OMITS blocked_on (an
// empty string, not the sentinel) must preserve whatever blocked_on was
// already set — this is the existing coalesceStr behavior, and
// resolveBlockedOn must not break it while adding the clear path.
func TestApply_Requirement_UpdateOmittedBlockedOnPreservesExisting(t *testing.T) {
	t.Parallel()
	g := baseGraph()
	g.Requirements[0].BlockedOn = "feature:missing-cli"
	path := writeTempGraph(t, g)

	p := ProposedRequirement{
		ID:     "R-1",
		Claim:  "claim R-1, edited",
		Owner:  "sa",
		Status: ontology.StatusSETTLED,
		// BlockedOn intentionally omitted.
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	got := reload(t, path)
	r, ok := findReq(got, "R-1")
	if !ok {
		t.Fatalf("R-1 missing")
	}
	if r.BlockedOn != "feature:missing-cli" {
		t.Errorf("BlockedOn = %q, want preserved %q", r.BlockedOn, "feature:missing-cli")
	}
}

// TestApply_Requirement_UpdateRealBlockedOnReplacesExisting pins the normal
// replace case: an UPDATE proposal with a real non-empty blocked_on value
// replaces whatever was there before (including replacing one real value with
// another, not just setting from empty).
func TestApply_Requirement_UpdateRealBlockedOnReplacesExisting(t *testing.T) {
	t.Parallel()
	g := baseGraph()
	g.Requirements[0].BlockedOn = "feature:old-blocker"
	path := writeTempGraph(t, g)

	p := ProposedRequirement{
		ID:        "R-1",
		Claim:     "claim R-1",
		Owner:     "sa",
		Status:    ontology.StatusSETTLED,
		BlockedOn: "feature:new-blocker",
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	got := reload(t, path)
	r, ok := findReq(got, "R-1")
	if !ok {
		t.Fatalf("R-1 missing")
	}
	if r.BlockedOn != "feature:new-blocker" {
		t.Errorf("BlockedOn = %q, want %q", r.BlockedOn, "feature:new-blocker")
	}
}

// TestApply_Requirement_CreateWithClearSentinelBlockedOnFails covers the
// CREATE-path decision for blocked_on's sentinel: a brand-new requirement has
// no existing blocked_on to clear, so "<clear>" on a CREATE proposal is
// rejected outright rather than silently writing the literal string
// "<clear>" into a new requirement's blocked_on.
func TestApply_Requirement_CreateWithClearSentinelBlockedOnFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedRequirement{
		ID:        "R-new",
		Claim:     "a brand new claim",
		Owner:     "sa",
		Status:    ontology.StatusDRAFT,
		BlockedOn: clearSentinel,
	}
	assertApplyFails(t, path, p, clearSentinel)
}

// TestApply_Requirement_ClearBlockedOn_ClosesBurnDownLoop is the end-to-end
// acceptance test for review-5 finding N1: seed a requirement as
// feature-blocked debt (IsFeatureBlockedDebt), land an UPDATE proposal that
// clears blocked_on via the sentinel, regenerate UNENFORCED.md via
// generator.BuildUnenforced, and confirm the requirement moved from the
// "feature-blocked" table to the "closeable now" table. This is the actual
// burn-down-metric closure the review named — clearing the field alone isn't
// enough if the generator split doesn't react to it.
func TestApply_Requirement_ClearBlockedOn_ClosesBurnDownLoop(t *testing.T) {
	t.Parallel()
	g := baseGraph()
	g.Requirements[0].BlockedOn = "feature:missing-cli"
	if !g.Requirements[0].IsFeatureBlockedDebt() {
		t.Fatalf("setup: R-1 must start as feature-blocked debt")
	}
	if g.Requirements[0].IsCloseableDebtNow() {
		t.Fatalf("setup: R-1 must NOT start as closeable-now")
	}
	path := writeTempGraph(t, g)

	p := ProposedRequirement{
		ID:        "R-1",
		Claim:     "claim R-1",
		Owner:     "sa",
		Status:    ontology.StatusSETTLED,
		BlockedOn: clearSentinel,
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	got := reload(t, path)
	r, ok := findReq(got, "R-1")
	if !ok {
		t.Fatalf("R-1 missing")
	}
	if !r.IsCloseableDebtNow() {
		t.Errorf("R-1 IsCloseableDebtNow() = false after clearing blocked_on, want true")
	}
	if r.IsFeatureBlockedDebt() {
		t.Errorf("R-1 IsFeatureBlockedDebt() = true after clearing blocked_on, want false")
	}

	rendered := generator.BuildUnenforced(got)
	if !strings.Contains(rendered, "## Closeable debt — closeable now (real, actionable)") {
		t.Fatalf("rendered UNENFORCED.md missing closeable-now heading:\n%s", rendered)
	}
	nowIdx := strings.Index(rendered, "## Closeable debt — closeable now (real, actionable)")
	blockedIdx := strings.Index(rendered, "## Closeable debt — feature-blocked (honest roadmap, not neglected)")
	inherentIdx := strings.Index(rendered, "## Inherent discipline")
	if nowIdx < 0 || blockedIdx < 0 || inherentIdx < 0 {
		t.Fatalf("rendered UNENFORCED.md missing an expected section heading:\n%s", rendered)
	}
	nowSection := rendered[nowIdx:blockedIdx]
	blockedSection := rendered[blockedIdx:inherentIdx]
	if !strings.Contains(nowSection, "`R-1`") {
		t.Errorf("R-1 not found in closeable-now section after clearing blocked_on:\n%s", nowSection)
	}
	if strings.Contains(blockedSection, "`R-1`") {
		t.Errorf("R-1 still present in feature-blocked section after clearing blocked_on:\n%s", blockedSection)
	}
}

func TestApply_ConflictTransition_Decided(t *testing.T) {
	t.Parallel()
	cid := ontology.ConflictIdentity("cost-vs-flexibility", "shared scenario")
	path := writeTempGraph(t, baseGraph())
	p := ProposedConflictTransition{
		ConflictID:   cid,
		NewLifecycle: "DECIDED(chose option A for clarity)",
		DecidedBy:    "outsider",
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	g := reload(t, path)
	c, ok := findConflict(g, cid)
	if !ok {
		t.Fatalf("conflict missing")
	}
	if !c.IsDecided() {
		t.Errorf("Lifecycle = %q, want DECIDED prefix", c.Lifecycle)
	}
	if c.DecidedBy != "outsider" {
		t.Errorf("DecidedBy = %q, want outsider", c.DecidedBy)
	}
	if c.Signoff == nil || c.Signoff.DecidedBy != "outsider" {
		t.Errorf("Signoff not materialized: %+v", c.Signoff)
	}
	if c.DecidedAt != today {
		t.Errorf("DecidedAt = %q, want %q", c.DecidedAt, today)
	}
}

func TestApply_ConflictTransition_DecidedWithoutDecidedByFails(t *testing.T) {
	t.Parallel()
	cid := ontology.ConflictIdentity("cost-vs-flexibility", "shared scenario")
	path := writeTempGraph(t, baseGraph())
	p := ProposedConflictTransition{
		ConflictID:   cid,
		NewLifecycle: "DECIDED(chose option A)",
	}
	assertApplyFails(t, path, p, "decided_by")
}

func TestApply_ConflictTransition_HeldRequiresVariants(t *testing.T) {
	t.Parallel()
	cid := ontology.ConflictIdentity("cost-vs-flexibility", "shared scenario")
	path := writeTempGraph(t, baseGraph())
	p := ProposedConflictTransition{
		ConflictID:   cid,
		NewLifecycle: "HELD(cannot resolve by amending members)",
		DecidedBy:    "outsider",
	}
	assertApplyFails(t, path, p, "2 distinct")
}

// TestApply_ConflictTransition_DirectIDResolves_UnknownIDErrors enforces
// R-conflict-addressing-resolves-variables: findConflictIndex (mutate.go)
// locates a Conflict by its direct string ID — the "direct form" addressing
// mode — and an unknown/nonexistent ID must surface as an ERROR, never a silent
// no-op (which would silently drop a resolver decision) nor a panic.
//
// Two halves in one test: (A) a transition addressed by the literal conflict ID
// is located and mutates the conflict; (B) a nonexistent ID errors and leaves
// the graph on disk untouched. If findConflictIndex ever returned a silent
// -1-without-error path or the mutate stopped checking idx<0, half B fails.
func TestApply_ConflictTransition_DirectIDResolves_UnknownIDErrors(t *testing.T) {
	t.Parallel()
	cid := ontology.ConflictIdentity("cost-vs-flexibility", "shared scenario")
	path := writeTempGraph(t, baseGraph())

	// Half A — direct-form ID resolves: the literal conflict ID is located and
	// the lifecycle mutates to exactly what was sent.
	resolve := ProposedConflictTransition{
		ConflictID:   cid,
		NewLifecycle: "REVISIT_WHEN(next quarterly review)",
	}
	if err := Apply(path, today, resolve); err != nil {
		t.Fatalf("Apply with valid direct conflict_id %q: %v", cid, err)
	}
	c, ok := findConflict(reload(t, path), cid)
	if !ok {
		t.Fatalf("conflict %s missing after apply", cid)
	}
	if c.Lifecycle != "REVISIT_WHEN(next quarterly review)" {
		t.Errorf("direct-form ID did not resolve: Lifecycle = %q, want REVISIT_WHEN(...)", c.Lifecycle)
	}

	// Half B — unknown/nonexistent ID errors (not a silent no-op, not a panic),
	// and the graph on disk is left unchanged. assertApplyFails asserts both the
	// error substring and that the file is byte-identical before/after.
	bad := ProposedConflictTransition{
		ConflictID:   "C-does-not-exist-anywhere",
		NewLifecycle: "DECIDED(x)",
		DecidedBy:    "outsider",
	}
	assertApplyFails(t, path, bad, "not found")
}

func TestApply_Rejection(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedRejection{
		RequirementID: "R-1",
		Reason:        "REJECTED — superseded by R-2",
		ReplacedBy:    []string{"R-2"},
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	g := reload(t, path)
	r, ok := findReq(g, "R-1")
	if !ok {
		t.Fatalf("R-1 missing")
	}
	if r.Status != ontology.StatusREJECTED {
		t.Errorf("Status = %q, want REJECTED", r.Status)
	}
	succ, ok := findReq(g, "R-2")
	if !ok {
		t.Fatalf("R-2 missing")
	}
	hasReplaces := false
	for _, rel := range succ.Relations {
		if rel.Kind == "replaces" && rel.Target == "R-1" {
			hasReplaces = true
		}
	}
	if !hasReplaces {
		t.Errorf("R-2 has no replaces edge to R-1: %+v", succ.Relations)
	}
}

func TestApply_RejectionEmptyReasonFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedRejection{RequirementID: "R-1", Reason: ""}
	assertApplyFails(t, path, p, "'reason'")
}

func TestApply_Conflict_Create(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedConflict{
		Axis:     "cost-vs-flexibility",
		Context:  "a brand new tension surface",
		Members:  []string{"R-1", "R-3"},
		Resolver: "outsider",
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	g := reload(t, path)
	id := ontology.ConflictIdentity("cost-vs-flexibility", "a brand new tension surface")
	c, ok := findConflict(g, id)
	if !ok {
		t.Fatalf("new conflict %s missing", id)
	}
	if c.Lifecycle != ontology.ConflictDETECTED {
		t.Errorf("Lifecycle = %q, want DETECTED", c.Lifecycle)
	}
	if c.CreatedAt != today {
		t.Errorf("CreatedAt = %q, want %q", c.CreatedAt, today)
	}
}

func TestApply_Conflict_ResolverOwnsMemberFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedConflict{
		Axis:     "cost-vs-flexibility",
		Context:  "another tension",
		Members:  []string{"R-1", "R-2"},
		Resolver: "sa",
	}
	assertApplyFails(t, path, p, "owns member")
}

func TestApply_OperatorBudget(t *testing.T) {
	// Hermetic isolation: check_operator_within_budget's CRYSTAL_CHARS branch
	// deliberately reads the REAL <project-root>/CLAUDE.md off disk (resident
	// crystal budget check; see internal/invariants/lifecycle_checks.go). Pin
	// paths.ProjectRoot() to an empty temp dir (no CLAUDE.md) so this test's
	// pass/fail does not depend on the size of the ambient repo's CLAUDE.md.
	t.Setenv(paths.EnvProjectRoot, t.TempDir())
	path := writeTempGraph(t, graphWithOperator())
	p := ProposedOperatorBudget{
		OperatorID: "OP-1",
		NewLimit:   50,
		NewMeasure: ontology.BudgetMeasureCRYSTAL_CHARS,
		Why:        "CRYSTAL_CHARS reflects real working cost",
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	g := reload(t, path)
	var op ontology.Operator
	for _, o := range g.Operators {
		if o.ID == "OP-1" {
			op = o
		}
	}
	if op.ContextBudget.Measure != ontology.BudgetMeasureCRYSTAL_CHARS {
		t.Errorf("Measure = %q, want CRYSTAL_CHARS", op.ContextBudget.Measure)
	}
	if op.ContextBudget.Limit != 50 {
		t.Errorf("Limit = %d, want 50", op.ContextBudget.Limit)
	}
}

func TestApply_OperatorBudget_NegativeLimitFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, graphWithOperator())
	p := ProposedOperatorBudget{
		OperatorID: "OP-1",
		NewLimit:   -1,
		NewMeasure: ontology.BudgetMeasureNODE_COUNT,
	}
	assertApplyFails(t, path, p, "new_limit")
}

func TestApply_Axis(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedAxis{
		Slug:        "speed-vs-quality",
		Description: "tension between shipping speed and quality",
		Why:         "surfaced by a latency conflict",
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	g := reload(t, path)
	found := false
	for _, a := range g.Axes {
		if a.Slug == "speed-vs-quality" {
			found = true
		}
	}
	if !found {
		t.Errorf("axis speed-vs-quality not added")
	}
}

// TestApply_Axis_ExistingSlugUpdatesRatherThanFails documents a deliberate
// semantic shift: before Axis gained an UPDATE path, a proposal naming an
// already-existing slug was rejected as errDuplicate (this test used to be
// named TestApply_Axis_DuplicateSlugFails and asserted exactly that). Now
// that Axis is CREATE-or-UPDATE (mirroring EntityType/Process, which made
// the identical shift when they gained UPDATE modes -- see
// TestApply_EntityType_UpdateAppendsNewField / TestApply_Process_Create's
// sibling UPDATE tests, neither of which has a surviving "duplicate slug
// fails" test either), an existing slug is a valid UPDATE target, not an
// error. See TestApply_Axis_UpdateDescriptionAppendsHistory for the full
// UPDATE-path assertion; this test's role is narrower: prove Apply no
// longer errors for this shape.
func TestApply_Axis_ExistingSlugUpdatesRatherThanFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedAxis{
		Slug:        "cost-vs-flexibility",
		Description: "updated, not duplicated",
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v, want success (existing slug is now a valid UPDATE target)", err)
	}
}

func TestApply_Stakeholder(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedStakeholder{
		ID:     "newparty",
		Name:   "New Party",
		Domain: "governance",
		Why:    "first newcomer door",
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	g := reload(t, path)
	found := false
	for _, s := range g.Stakeholders {
		if s.ID == "newparty" {
			found = true
		}
	}
	if !found {
		t.Errorf("stakeholder newparty not added")
	}
}

func TestApply_Stakeholder_DuplicateIDFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedStakeholder{
		ID:     "outsider",
		Name:   "Dup",
		Domain: "x",
	}
	assertApplyFails(t, path, p, "duplicate")
}

func TestApply_Assumption_Create(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedAssumption{
		ID:        "A-new",
		Statement: "a narrower falsifiable belief",
		Status:    ontology.AssumptionHOLDS,
		Owner:     "sa",
		Why:       "split from an over-broad assumption",
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	g := reload(t, path)
	found := false
	for _, a := range g.Assumptions {
		if a.ID == "A-new" {
			found = true
			if a.CreatedAt != today {
				t.Errorf("CreatedAt = %q, want %q", a.CreatedAt, today)
			}
		}
	}
	if !found {
		t.Errorf("assumption A-new not added")
	}
}

func TestApply_Assumption_DuplicateIDFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedAssumption{
		ID:        "A-base",
		Statement: "dup",
		Status:    ontology.AssumptionHOLDS,
		Owner:     "sa",
	}
	assertApplyFails(t, path, p, "duplicate")
}

func TestApply_AssumptionTransition_Dead(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedAssumptionTransition{
		AssumptionID: "A-base",
		NewStatus:    ontology.AssumptionDEAD,
		Reason:       "falsified by the latest run",
		DecidedBy:    "outsider",
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	g := reload(t, path)
	var a ontology.Assumption
	for _, x := range g.Assumptions {
		if x.ID == "A-base" {
			a = x
		}
	}
	if a.Status != ontology.AssumptionDEAD {
		t.Errorf("Status = %q, want DEAD", a.Status)
	}
	if a.Signoff == nil || a.Signoff.DecidedBy != "outsider" {
		t.Errorf("Signoff not materialized: %+v", a.Signoff)
	}
	if a.DecidedAt != today {
		t.Errorf("DecidedAt = %q, want %q", a.DecidedAt, today)
	}
	if a.Statement == "the substrate is stable" {
		t.Errorf("Statement unchanged; reason must be appended: %q", a.Statement)
	}
}

func TestApply_AssumptionTransition_DeadWithoutDecidedByFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedAssumptionTransition{
		AssumptionID: "A-base",
		NewStatus:    ontology.AssumptionDEAD,
		Reason:       "falsified",
	}
	assertApplyFails(t, path, p, "decided_by")
}

// TestApply_Axis_UpdateDescriptionAppendsHistory proves the Axis UPDATE path
// (mirroring TestApply_Requirement_UpdateAppendsHistory's shape for
// Requirement): a proposal whose slug already names an existing Axis
// REPLACES its description and appends exactly one HistoryEntry, rather
// than being rejected as a duplicate.
func TestApply_Axis_UpdateDescriptionAppendsHistory(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedAxis{
		Slug:        "cost-vs-flexibility",
		Description: "revised: cost vs flexibility, now weighted toward flexibility",
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	g := reload(t, path)
	var a ontology.Axis
	found := false
	for _, x := range g.Axes {
		if x.Slug == "cost-vs-flexibility" {
			a = x
			found = true
		}
	}
	if !found {
		t.Fatalf("axis cost-vs-flexibility missing after UPDATE")
	}
	if a.Description != "revised: cost vs flexibility, now weighted toward flexibility" {
		t.Errorf("Description = %q, want revised", a.Description)
	}
	if len(a.History) != 1 {
		t.Fatalf("History len = %d, want 1", len(a.History))
	}
	if a.History[0].At != today {
		t.Errorf("History[0].At = %q, want %q", a.History[0].At, today)
	}
}

// TestApply_Axis_UpdateOmittedDescriptionPreservesExisting proves patch
// semantics on the Axis UPDATE path: omitting description on an UPDATE
// leaves the existing value untouched and appends no History (a no-op
// UPDATE, mirroring how ProposedRequirement's coalesceStr fields behave
// when omitted).
func TestApply_Axis_UpdateOmittedDescriptionPreservesExisting(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedAxis{
		Slug: "cost-vs-flexibility",
		Why:  "no content change, just touching the node",
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	g := reload(t, path)
	var a ontology.Axis
	for _, x := range g.Axes {
		if x.Slug == "cost-vs-flexibility" {
			a = x
		}
	}
	if a.Description != "cost vs flexibility" {
		t.Errorf("Description = %q, want preserved original", a.Description)
	}
	if len(a.History) != 0 {
		t.Errorf("History = %v, want empty (no-op UPDATE)", a.History)
	}
}

// TestApply_Axis_CreateWithoutDescriptionFails proves validate()/mutate()'s
// deferred create-vs-update split still rejects a genuine CREATE (a
// brand-new slug) with no description -- the UPDATE path's relaxed
// validate() must not accidentally let a malformed CREATE through.
func TestApply_Axis_CreateWithoutDescriptionFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedAxis{
		Slug: "brand-new-axis",
	}
	assertApplyFails(t, path, p, "description")
}

// TestApply_Assumption_SourceRefs proves ProposedAssumption's new
// source_refs field lands on CREATE, and TestApply_Assumption_Create above
// already proves the rest of the CREATE shape unaffected.
func TestApply_Assumption_SourceRefs(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedAssumption{
		ID:         "A-with-refs",
		Statement:  "a belief with attached provenance",
		Status:     ontology.AssumptionHOLDS,
		Owner:      "sa",
		SourceRefs: []string{"docs/survey.md", "review-2026-07"},
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	g := reload(t, path)
	var a ontology.Assumption
	found := false
	for _, x := range g.Assumptions {
		if x.ID == "A-with-refs" {
			a = x
			found = true
		}
	}
	if !found {
		t.Fatalf("assumption A-with-refs missing")
	}
	if len(a.SourceRefs) != 2 || a.SourceRefs[0] != "docs/survey.md" || a.SourceRefs[1] != "review-2026-07" {
		t.Errorf("SourceRefs = %v, want [docs/survey.md review-2026-07]", a.SourceRefs)
	}
}

// TestApply_AssumptionRewrite_ReplacesStatementAndAppendsHistory is the
// carrier test for AssumptionRewrite's central claim (see its doc comment
// in types.go/mutate.go): a rewrite REPLACES Statement outright (unlike
// AssumptionTransition, which appends a suffix) and ALWAYS leaves a History
// trace -- the audit requirement the task's constraint singles out
// explicitly (a rewrite is a silent drift risk without it).
func TestApply_AssumptionRewrite_ReplacesStatementAndAppendsHistory(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedAssumptionRewrite{
		AssumptionID: "A-base",
		NewStatement: "the substrate is stable under normal load, unverified under burst load",
		Reason:       "the original statement overstated the verified scope",
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	g := reload(t, path)
	var a ontology.Assumption
	found := false
	for _, x := range g.Assumptions {
		if x.ID == "A-base" {
			a = x
			found = true
		}
	}
	if !found {
		t.Fatalf("assumption A-base missing")
	}
	if a.Statement != "the substrate is stable under normal load, unverified under burst load" {
		t.Errorf("Statement = %q, want clean replacement (no suffix appended)", a.Statement)
	}
	if a.Status != ontology.AssumptionHOLDS {
		t.Errorf("Status = %q, want unchanged HOLDS -- a rewrite must not touch status", a.Status)
	}
	if len(a.History) != 1 {
		t.Fatalf("History len = %d, want 1 -- a rewrite must always leave an audit trace", len(a.History))
	}
	if a.History[0].At != today {
		t.Errorf("History[0].At = %q, want %q", a.History[0].At, today)
	}
	if !strings.Contains(a.History[0].Summary, "the original statement overstated the verified scope") {
		t.Errorf("History[0].Summary = %q, want it to contain the reason", a.History[0].Summary)
	}
}

func TestApply_AssumptionRewrite_UnknownIDFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedAssumptionRewrite{
		AssumptionID: "A-does-not-exist",
		NewStatement: "irrelevant",
		Reason:       "irrelevant",
	}
	assertApplyFails(t, path, p, "not found")
}

func TestApply_AssumptionRewrite_EmptyReasonFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedAssumptionRewrite{
		AssumptionID: "A-base",
		NewStatement: "a rewrite with no reason",
	}
	assertApplyFails(t, path, p, "reason")
}

func TestApply_AssumptionRewrite_EmptyNewStatementFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedAssumptionRewrite{
		AssumptionID: "A-base",
		Reason:       "trying to clear the statement",
	}
	assertApplyFails(t, path, p, "new_statement")
}

// TestApply_Conflict_SourceRefs proves ProposedConflict's new source_refs
// field lands on CREATE.
func TestApply_Conflict_SourceRefs(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedConflict{
		Axis:       "cost-vs-flexibility",
		Context:    "a brand new scenario for source_refs",
		Members:    []string{"R-1", "R-3"},
		Resolver:   "outsider",
		SourceRefs: []string{"docs/incident-2026-07.md"},
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	g := reload(t, path)
	cid := ontology.ConflictIdentity("cost-vs-flexibility", "a brand new scenario for source_refs")
	c, ok := findConflict(g, cid)
	if !ok {
		t.Fatalf("conflict missing")
	}
	if len(c.SourceRefs) != 1 || c.SourceRefs[0] != "docs/incident-2026-07.md" {
		t.Errorf("SourceRefs = %v, want [docs/incident-2026-07.md]", c.SourceRefs)
	}
}

// TestApply_ConflictTransition_SourceRefsReplacesExisting proves
// ProposedConflictTransition's new source_refs field replaces the existing
// Conflict's SourceRefs when non-empty (the same "empty preserves,
// non-empty replaces" idiom Derived/Variants already use on this proposal
// kind).
func TestApply_ConflictTransition_SourceRefsReplacesExisting(t *testing.T) {
	t.Parallel()
	seed := baseGraph()
	cid := ontology.ConflictIdentity("cost-vs-flexibility", "shared scenario")
	for i, c := range seed.Conflicts {
		if c.ID == cid {
			seed.Conflicts[i].SourceRefs = []string{"docs/original.md"}
		}
	}
	path := writeTempGraph(t, seed)
	p := ProposedConflictTransition{
		ConflictID:   cid,
		NewLifecycle: ontology.ConflictACKNOWLEDGED,
		SourceRefs:   []string{"docs/updated.md"},
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	g := reload(t, path)
	c, ok := findConflict(g, cid)
	if !ok {
		t.Fatalf("conflict missing")
	}
	if len(c.SourceRefs) != 1 || c.SourceRefs[0] != "docs/updated.md" {
		t.Errorf("SourceRefs = %v, want [docs/updated.md]", c.SourceRefs)
	}
}

// TestApply_ConflictTransition_OmittedSourceRefsPreservesExisting proves the
// "empty preserves" half of the same idiom.
func TestApply_ConflictTransition_OmittedSourceRefsPreservesExisting(t *testing.T) {
	t.Parallel()
	seed := baseGraph()
	cid := ontology.ConflictIdentity("cost-vs-flexibility", "shared scenario")
	for i, c := range seed.Conflicts {
		if c.ID == cid {
			seed.Conflicts[i].SourceRefs = []string{"docs/original.md"}
		}
	}
	path := writeTempGraph(t, seed)
	p := ProposedConflictTransition{
		ConflictID:   cid,
		NewLifecycle: ontology.ConflictACKNOWLEDGED,
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	g := reload(t, path)
	c, ok := findConflict(g, cid)
	if !ok {
		t.Fatalf("conflict missing")
	}
	if len(c.SourceRefs) != 1 || c.SourceRefs[0] != "docs/original.md" {
		t.Errorf("SourceRefs = %v, want preserved [docs/original.md]", c.SourceRefs)
	}
}

func TestApply_ConflictMemberUpdate_Add(t *testing.T) {
	t.Parallel()
	cid := ontology.ConflictIdentity("cost-vs-flexibility", "shared scenario")
	path := writeTempGraph(t, baseGraph())
	p := ProposedConflictMemberUpdate{
		ConflictID: cid,
		AddMembers: []string{"R-3"},
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	g := reload(t, path)
	c, ok := findConflict(g, cid)
	if !ok {
		t.Fatalf("conflict missing")
	}
	if len(c.Members) != 3 {
		t.Errorf("Members = %v, want 3 (R-1, R-2, R-3)", c.Members)
	}
}

func TestApply_ConflictMemberUpdate_DropsBelowTwoFails(t *testing.T) {
	t.Parallel()
	cid := ontology.ConflictIdentity("cost-vs-flexibility", "shared scenario")
	path := writeTempGraph(t, baseGraph())
	p := ProposedConflictMemberUpdate{
		ConflictID:    cid,
		RemoveMembers: []string{"R-2"},
	}
	assertApplyFails(t, path, p, ">= 2")
}

func TestApply_EntityType(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedEntityType{
		Slug:        "feature-flag",
		Description: "a deployable feature toggle",
		Why:         "needed to model lifecycle-bearing entities",
		States: []EntityTypeState{
			{Name: "INIT", Kind: ontology.StateKindInitial},
			{Name: "ON", Kind: ontology.StateKindNormal},
			{Name: "OFF", Kind: ontology.StateKindQuiescent},
		},
		Transitions: []EntityTypeTransition{
			{Src: "INIT", Dst: "ON", Event: "enable"},
			{Src: "ON", Dst: "OFF", Event: "disable"},
		},
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	g := reload(t, path)
	found := false
	for _, et := range g.EntityTypes {
		if et.Slug == "feature-flag" {
			found = true
			if len(et.Lifecycle.States) != 3 {
				t.Errorf("States len = %d, want 3", len(et.Lifecycle.States))
			}
			if et.Lifecycle.Slug != "feature-flag-lifecycle" {
				t.Errorf("Lifecycle slug = %q, want feature-flag-lifecycle", et.Lifecycle.Slug)
			}
		}
	}
	if !found {
		t.Errorf("entity type feature-flag not added")
	}
}

func TestApply_EntityType_NoInitialStateFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedEntityType{
		Slug:        "bad-entity",
		Description: "no initial",
		Why:         "x",
		States: []EntityTypeState{
			{Name: "A", Kind: ontology.StateKindNormal},
			{Name: "B", Kind: ontology.StateKindQuiescent},
		},
	}
	assertApplyFails(t, path, p, "initial")
}

// entityTypeBaseGraph returns baseGraph() plus one pre-landed EntityType
// ("feature-flag", one existing field "owner") so UPDATE-mode tests have an
// already-existing slug to target.
func entityTypeBaseGraph() *ontology.Graph {
	g := baseGraph()
	g.EntityTypes = append(g.EntityTypes, ontology.EntityType{
		Slug:        "feature-flag",
		Description: "a deployable feature toggle",
		Why:         "needed to model lifecycle-bearing entities",
		Lifecycle: ontology.Lifecycle{
			Slug: "feature-flag-lifecycle",
			States: []ontology.State{
				{Name: "INIT", Kind: ontology.StateKindInitial},
				{Name: "ON", Kind: ontology.StateKindNormal},
				{Name: "OFF", Kind: ontology.StateKindQuiescent},
			},
			Transitions: []ontology.Transition{
				{Src: "INIT", Dst: "ON", Event: "enable"},
				{Src: "ON", Dst: "OFF", Event: "disable"},
			},
		},
		Fields: []ontology.EntityField{
			{Name: "owner", Kind: "reference", Required: true, RefTarget: "Stakeholder"},
		},
	})
	return g
}

func findEntityType(g *ontology.Graph, slug string) (ontology.EntityType, bool) {
	for _, et := range g.EntityTypes {
		if et.Slug == slug {
			return et, true
		}
	}
	return ontology.EntityType{}, false
}

// TestApply_EntityType_UpdateAppendsNewField covers acceptance case (a):
// an UPDATE proposal (existing slug, fields-only shape) successfully appends
// a new field to an existing EntityType -- the graph after mutate contains
// the EntityType with BOTH the old field ("owner") and the new one
// ("linked_release").
func TestApply_EntityType_UpdateAppendsNewField(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, entityTypeBaseGraph())
	p := ProposedEntityType{
		Slug: "feature-flag",
		Fields: []EntityTypeField{
			{Name: "linked_release", Kind: "reference", Required: false, RefTarget: "release"},
		},
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	g := reload(t, path)
	et, ok := findEntityType(g, "feature-flag")
	if !ok {
		t.Fatalf("feature-flag missing after update")
	}
	if len(et.Fields) != 2 {
		t.Fatalf("Fields len = %d, want 2 (owner + linked_release); got %+v", len(et.Fields), et.Fields)
	}
	names := map[string]ontology.EntityField{}
	for _, f := range et.Fields {
		names[f.Name] = f
	}
	if _, ok := names["owner"]; !ok {
		t.Errorf("old field %q lost after update", "owner")
	}
	added, ok := names["linked_release"]
	if !ok {
		t.Fatalf("new field %q not appended", "linked_release")
	}
	if added.Kind != "reference" || added.RefTarget != "release" {
		t.Errorf("linked_release field = %+v, want kind=reference ref_target=release", added)
	}
	// Lifecycle/description/why must be untouched by the update.
	if len(et.Lifecycle.States) != 3 {
		t.Errorf("States len = %d, want 3 (unchanged by update)", len(et.Lifecycle.States))
	}
	if et.Description != "a deployable feature toggle" {
		t.Errorf("Description = %q, changed by update", et.Description)
	}
	if len(et.History) != 1 {
		t.Fatalf("History len = %d, want 1 entry recording the update", len(et.History))
	}
	if !strings.Contains(et.History[0].Summary, "linked_release") {
		t.Errorf("History[0].Summary = %q, want it to mention linked_release", et.History[0].Summary)
	}
}

// TestApply_EntityType_UpdateDuplicateFieldNameFails covers acceptance case
// (b): an UPDATE proposal naming a field that already exists on the target
// EntityType is rejected, and the graph on disk is unchanged.
func TestApply_EntityType_UpdateDuplicateFieldNameFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, entityTypeBaseGraph())
	p := ProposedEntityType{
		Slug: "feature-flag",
		Fields: []EntityTypeField{
			{Name: "owner", Kind: "string"},
		},
	}
	assertApplyFails(t, path, p, "already has a field named")
}

// TestApply_EntityType_UpdateWithNonEmptyStatesFails covers acceptance case
// (c): an UPDATE proposal (existing slug) that also carries non-empty
// states/transitions/description/why is rejected as an out-of-scope shape,
// and the graph on disk is unchanged. This subtest only exercises 'states'
// non-empty; the sibling subtests exercise transitions/description/why.
func TestApply_EntityType_UpdateWithNonEmptyStatesFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, entityTypeBaseGraph())
	p := ProposedEntityType{
		Slug:        "feature-flag",
		Description: "a full create-shaped payload so validate() lets this reach mutate()",
		Fields: []EntityTypeField{
			{Name: "linked_release", Kind: "reference", RefTarget: "release"},
		},
		States: []EntityTypeState{
			{Name: "INIT", Kind: ontology.StateKindInitial},
		},
	}
	assertApplyFails(t, path, p, "UPDATE currently supports ONLY appending new 'fields'")
}

func TestApply_EntityType_UpdateWithNonEmptyTransitionsFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, entityTypeBaseGraph())
	p := ProposedEntityType{
		Slug: "feature-flag",
		Fields: []EntityTypeField{
			{Name: "linked_release", Kind: "reference", RefTarget: "release"},
		},
		Transitions: []EntityTypeTransition{
			{Src: "INIT", Dst: "ON", Event: "enable"},
		},
	}
	assertApplyFails(t, path, p, "UPDATE currently supports ONLY appending new 'fields'")
}

func TestApply_EntityType_UpdateWithNonEmptyDescriptionFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, entityTypeBaseGraph())
	p := ProposedEntityType{
		Slug:        "feature-flag",
		Description: "a changed description",
		States: []EntityTypeState{
			{Name: "INIT", Kind: ontology.StateKindInitial},
		},
		Fields: []EntityTypeField{
			{Name: "linked_release", Kind: "reference", RefTarget: "release"},
		},
	}
	assertApplyFails(t, path, p, "UPDATE currently supports ONLY appending new 'fields'")
}

func TestApply_EntityType_UpdateWithNonEmptyWhyFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, entityTypeBaseGraph())
	p := ProposedEntityType{
		Slug: "feature-flag",
		Why:  "a changed why",
		Fields: []EntityTypeField{
			{Name: "linked_release", Kind: "reference", RefTarget: "release"},
		},
	}
	assertApplyFails(t, path, p, "UPDATE currently supports ONLY appending new 'fields'")
}

func TestApply_Requirement_AddInvariantGuardFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedRequirement{
		ID:     "not-r-prefixed",
		Claim:  "claim",
		Owner:  "sa",
		Status: ontology.StatusDRAFT,
	}
	assertApplyFails(t, path, p, "invariant violation")
}

// processBaseGraph returns baseGraph() plus two pre-landed EntityTypes
// ("feature-flag", "release") so ProposedProcess.drives_entities tests have
// real, declared EntityType slugs to resolve against -- two so UPDATE-mode
// append tests can prove a NEW slug (distinct from whatever a pre-landed
// Process already drives) is accepted.
func processBaseGraph() *ontology.Graph {
	g := baseGraph()
	g.EntityTypes = append(g.EntityTypes,
		ontology.EntityType{
			Slug:        "feature-flag",
			Description: "a deployable feature toggle",
			Lifecycle: ontology.Lifecycle{
				Slug: "feature-flag-lifecycle",
				States: []ontology.State{
					{Name: "INIT", Kind: ontology.StateKindInitial},
					{Name: "ON", Kind: ontology.StateKindNormal},
				},
				Transitions: []ontology.Transition{
					{Src: "INIT", Dst: "ON", Event: "enable"},
				},
			},
		},
		ontology.EntityType{
			Slug:        "release",
			Description: "a shipped release",
			Lifecycle: ontology.Lifecycle{
				Slug: "release-lifecycle",
				States: []ontology.State{
					{Name: "PLANNED", Kind: ontology.StateKindInitial},
					{Name: "SHIPPED", Kind: ontology.StateKindNormal},
				},
				Transitions: []ontology.Transition{
					{Src: "PLANNED", Dst: "SHIPPED", Event: "ship"},
				},
			},
		},
	)
	return g
}

func findProcess(g *ontology.Graph, id string) (ontology.Process, bool) {
	for _, p := range g.Processes {
		if p.ID == id {
			return p, true
		}
	}
	return ontology.Process{}, false
}

func validProcessProposal() ProposedProcess {
	return ProposedProcess{
		ID: "PR-test-loop",
		Steps: []ProposedStep{
			{Name: "diagnose", RequiresRole: "operator", Why: "find the top action"},
			{Name: "approve", RequiresRole: "resolver", Why: "resolver reviews and signs off"},
		},
		RolesRequired:  []string{"operator", "resolver"},
		DrivesEntities: []string{"feature-flag"},
		Why:            "a worked example process for testing",
	}
}

// TestApply_Process_Create covers acceptance case (a): a valid Process lands
// in graph.json and is loadable back with all fields intact, including the
// stamped shared ontology.ProcessLifecycle.
func TestApply_Process_Create(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, processBaseGraph())
	p := validProcessProposal()
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	g := reload(t, path)
	proc, ok := findProcess(g, "PR-test-loop")
	if !ok {
		t.Fatalf("PR-test-loop not present after Apply")
	}
	if len(proc.Steps) != 2 {
		t.Fatalf("Steps len = %d, want 2", len(proc.Steps))
	}
	if proc.Steps[0].Name != "diagnose" || proc.Steps[0].RequiresRole != "operator" {
		t.Errorf("Steps[0] = %+v, want diagnose/operator", proc.Steps[0])
	}
	if len(proc.RolesRequired) != 2 {
		t.Errorf("RolesRequired = %v, want 2 entries", proc.RolesRequired)
	}
	if len(proc.DrivesEntities) != 1 || proc.DrivesEntities[0] != "feature-flag" {
		t.Errorf("DrivesEntities = %v, want [feature-flag]", proc.DrivesEntities)
	}
	if proc.Lifecycle.Slug != ontology.ProcessLifecycle.Slug {
		t.Errorf("Lifecycle.Slug = %q, want %q (shared ProcessLifecycle)", proc.Lifecycle.Slug, ontology.ProcessLifecycle.Slug)
	}
	if len(proc.Lifecycle.States) != len(ontology.ProcessLifecycle.States) {
		t.Errorf("Lifecycle.States len = %d, want %d", len(proc.Lifecycle.States), len(ontology.ProcessLifecycle.States))
	}
}

// TestApply_Process_DuplicateStepNameOnUpdateFails covers acceptance case
// (b), restated for task #212's UPDATE mode: landing a Process proposal
// whose id already exists in the graph is now a valid UPDATE (see
// TestApply_Process_UpdateAppendsStep below) rather than an inherent
// duplicate error -- but resubmitting a step Name that already exists on
// the target Process (attempting to redefine, not append) is still a clear
// error, not a silent overwrite, mirroring
// TestApply_EntityType_UpdateDuplicateFieldNameFails's precedent.
func TestApply_Process_DuplicateStepNameOnUpdateFails(t *testing.T) {
	t.Parallel()
	g := processBaseGraph()
	g.Processes = append(g.Processes, ontology.Process{
		ID:            "PR-test-loop",
		Lifecycle:     ontology.ProcessLifecycle,
		Steps:         []ontology.Step{{Name: "diagnose", RequiresRole: "operator", Why: "y"}},
		RolesRequired: []string{"operator"},
	})
	path := writeTempGraph(t, g)
	p := validProcessProposal() // steps include "diagnose", which already exists above
	assertApplyFails(t, path, p, "already has a step named")
}

// TestApply_Process_UnknownDrivesEntitiesFails covers acceptance case (c): a
// drives_entities slug that does not resolve to a declared EntityType is
// rejected with a clear, name-carrying error.
func TestApply_Process_UnknownDrivesEntitiesFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, processBaseGraph())
	p := validProcessProposal()
	p.DrivesEntities = []string{"no-such-entity-type"}
	assertApplyFails(t, path, p, "no-such-entity-type")
}

// TestApply_Process_EmptyStepsFails covers acceptance case (d): a brand-new
// (CREATE-shaped, id not yet in the graph) Process with an empty steps list
// is rejected. RolesRequired is also cleared here -- with p.Steps empty, a
// non-empty RolesRequired hits a different, more specific validate() error
// ("roles_required must be empty when steps is empty", exercised by
// TestApply_Process_UpdateRolesRequiredWithoutStepsFails) since it is
// deliberately ambiguous with an UPDATE-appends-only-drives_entities shape;
// clearing it here isolates the "CREATE needs >=1 step" failure mode this
// test targets.
func TestApply_Process_EmptyStepsFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, processBaseGraph())
	p := validProcessProposal()
	p.Steps = nil
	p.RolesRequired = nil
	assertApplyFails(t, path, p, "'steps' must be a non-empty list")
}

// TestApply_Process_StepMissingRequiresRoleFails covers acceptance case (e):
// a step with an empty requires_role is rejected.
func TestApply_Process_StepMissingRequiresRoleFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, processBaseGraph())
	p := validProcessProposal()
	p.Steps = []ProposedStep{
		{Name: "diagnose", RequiresRole: "", Why: "find the top action"},
	}
	p.RolesRequired = nil
	assertApplyFails(t, path, p, "'requires_role' is required")
}

// TestApply_Process_StepMissingWhyFails covers acceptance case (e): a step
// with an empty why is rejected.
func TestApply_Process_StepMissingWhyFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, processBaseGraph())
	p := validProcessProposal()
	p.Steps = []ProposedStep{
		{Name: "diagnose", RequiresRole: "operator", Why: ""},
	}
	p.RolesRequired = []string{"operator"}
	assertApplyFails(t, path, p, "'why' is required")
}

func TestApply_Process_StepMissingNameFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, processBaseGraph())
	p := validProcessProposal()
	p.Steps = []ProposedStep{
		{Name: "  ", RequiresRole: "operator", Why: "find the top action"},
	}
	p.RolesRequired = []string{"operator"}
	assertApplyFails(t, path, p, "'name' is required")
}

func TestApply_Process_IDWithoutPRPrefixFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, processBaseGraph())
	p := validProcessProposal()
	p.ID = "not-pr-prefixed"
	assertApplyFails(t, path, p, "must start with 'PR-'")
}

func TestApply_Process_StepRoleNotInRolesRequiredFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, processBaseGraph())
	p := validProcessProposal()
	p.RolesRequired = []string{"operator"} // "resolver" (used by second step) omitted
	assertApplyFails(t, path, p, "not listed in 'roles_required'")
}

func TestApply_Process_UndemandedRoleInRolesRequiredFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, processBaseGraph())
	p := validProcessProposal()
	p.RolesRequired = append(p.RolesRequired, "reviewer") // no step demands "reviewer"
	assertApplyFails(t, path, p, "no step requires it")
}

func TestApply_Process_NoDrivesEntitiesOK(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, processBaseGraph())
	p := validProcessProposal()
	p.DrivesEntities = nil
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	g := reload(t, path)
	proc, ok := findProcess(g, "PR-test-loop")
	if !ok {
		t.Fatalf("PR-test-loop not present after Apply")
	}
	if len(proc.DrivesEntities) != 0 {
		t.Errorf("DrivesEntities = %v, want empty", proc.DrivesEntities)
	}
}

// processBaseGraphWithLandedProcess returns processBaseGraph() plus one
// pre-landed Process ("PR-test-loop", one step "diagnose"/"operator", one
// drives_entities entry "feature-flag") so UPDATE-mode tests (task #212)
// have an already-existing id to target.
func processBaseGraphWithLandedProcess() *ontology.Graph {
	g := processBaseGraph()
	g.Processes = append(g.Processes, ontology.Process{
		ID:             "PR-test-loop",
		Lifecycle:      ontology.ProcessLifecycle,
		Steps:          []ontology.Step{{Name: "diagnose", RequiresRole: "operator", Why: "find the top action"}},
		RolesRequired:  []string{"operator"},
		DrivesEntities: []string{"feature-flag"},
		Why:            "the original rationale",
	})
	return g
}

// TestApply_Process_UpdateAppendsStep covers acceptance case (a): an UPDATE
// proposal (existing id, new step) successfully appends a step to the end of
// an existing Process's Steps -- the graph after mutate contains BOTH the
// old step ("diagnose") and the new one ("approve"), in order, and the new
// step's role is unioned into RolesRequired.
func TestApply_Process_UpdateAppendsStep(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, processBaseGraphWithLandedProcess())
	p := ProposedProcess{
		ID: "PR-test-loop",
		Steps: []ProposedStep{
			{Name: "approve", RequiresRole: "resolver", Why: "resolver reviews and signs off"},
		},
		RolesRequired: []string{"resolver"},
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	g := reload(t, path)
	proc, ok := findProcess(g, "PR-test-loop")
	if !ok {
		t.Fatalf("PR-test-loop missing after update")
	}
	if len(proc.Steps) != 2 {
		t.Fatalf("Steps len = %d, want 2 (diagnose + approve); got %+v", len(proc.Steps), proc.Steps)
	}
	if proc.Steps[0].Name != "diagnose" {
		t.Errorf("Steps[0].Name = %q, want %q (original order preserved)", proc.Steps[0].Name, "diagnose")
	}
	if proc.Steps[1].Name != "approve" || proc.Steps[1].RequiresRole != "resolver" {
		t.Errorf("Steps[1] = %+v, want approve/resolver", proc.Steps[1])
	}
	wantRoles := map[string]bool{"operator": true, "resolver": true}
	if len(proc.RolesRequired) != 2 {
		t.Fatalf("RolesRequired = %v, want [operator resolver]", proc.RolesRequired)
	}
	for _, r := range proc.RolesRequired {
		if !wantRoles[r] {
			t.Errorf("RolesRequired contains unexpected role %q", r)
		}
	}
	if len(proc.History) != 1 {
		t.Fatalf("History len = %d, want 1 entry recording the update", len(proc.History))
	}
	if !strings.Contains(proc.History[0].Summary, "approve") {
		t.Errorf("History[0].Summary = %q, want it to mention approve", proc.History[0].Summary)
	}
	// why/lifecycle/drives_entities untouched by a steps-only update.
	if proc.Why != "the original rationale" {
		t.Errorf("Why = %q, changed by a steps-only update", proc.Why)
	}
	if len(proc.DrivesEntities) != 1 || proc.DrivesEntities[0] != "feature-flag" {
		t.Errorf("DrivesEntities = %v, changed by a steps-only update", proc.DrivesEntities)
	}
}

// TestApply_Process_UpdateAppendsDrivesEntities covers acceptance case (b):
// an UPDATE proposal (existing id, no new steps, new drives_entities)
// successfully appends a NEW EntityType slug ("release") to the existing
// Process's DrivesEntities, resolving it against the domain's declared
// EntityTypes exactly like the create-path does.
func TestApply_Process_UpdateAppendsDrivesEntities(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, processBaseGraphWithLandedProcess())
	p := ProposedProcess{
		ID:             "PR-test-loop",
		DrivesEntities: []string{"release"},
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	g := reload(t, path)
	proc, ok := findProcess(g, "PR-test-loop")
	if !ok {
		t.Fatalf("PR-test-loop missing after update")
	}
	want := map[string]bool{"feature-flag": true, "release": true}
	if len(proc.DrivesEntities) != 2 {
		t.Fatalf("DrivesEntities = %v, want [feature-flag release]", proc.DrivesEntities)
	}
	for _, slug := range proc.DrivesEntities {
		if !want[slug] {
			t.Errorf("DrivesEntities contains unexpected slug %q", slug)
		}
	}
	// steps untouched by a drives_entities-only update.
	if len(proc.Steps) != 1 || proc.Steps[0].Name != "diagnose" {
		t.Errorf("Steps = %+v, changed by a drives_entities-only update", proc.Steps)
	}
}

// TestApply_Process_UpdateInvalidAppendStepFails covers acceptance case (c):
// an UPDATE proposal appending a step with an empty name/requires_role/why
// is rejected by the SAME validate() rules a CREATE's steps are checked
// against (isProcessStepsShapePresent routes both CREATE and UPDATE through
// the same per-step shape checks in validate.go).
func TestApply_Process_UpdateInvalidAppendStepFails(t *testing.T) {
	t.Parallel()
	t.Run("empty name", func(t *testing.T) {
		t.Parallel()
		path := writeTempGraph(t, processBaseGraphWithLandedProcess())
		p := ProposedProcess{
			ID:            "PR-test-loop",
			Steps:         []ProposedStep{{Name: "  ", RequiresRole: "resolver", Why: "y"}},
			RolesRequired: []string{"resolver"},
		}
		assertApplyFails(t, path, p, "'name' is required")
	})
	t.Run("empty requires_role", func(t *testing.T) {
		t.Parallel()
		path := writeTempGraph(t, processBaseGraphWithLandedProcess())
		p := ProposedProcess{
			ID:    "PR-test-loop",
			Steps: []ProposedStep{{Name: "approve", RequiresRole: "", Why: "y"}},
		}
		assertApplyFails(t, path, p, "'requires_role' is required")
	})
	t.Run("empty why", func(t *testing.T) {
		t.Parallel()
		path := writeTempGraph(t, processBaseGraphWithLandedProcess())
		p := ProposedProcess{
			ID:            "PR-test-loop",
			Steps:         []ProposedStep{{Name: "approve", RequiresRole: "resolver", Why: ""}},
			RolesRequired: []string{"resolver"},
		}
		assertApplyFails(t, path, p, "'why' is required")
	})
}

// TestApply_Process_UpdateUnknownDrivesEntitiesFails covers acceptance case
// (d): an UPDATE proposal appending a drives_entities slug that does not
// resolve to a declared EntityType is rejected with a clear, name-carrying
// error, same as the create-path (TestApply_Process_UnknownDrivesEntitiesFails).
func TestApply_Process_UpdateUnknownDrivesEntitiesFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, processBaseGraphWithLandedProcess())
	p := ProposedProcess{
		ID:             "PR-test-loop",
		DrivesEntities: []string{"no-such-entity-type"},
	}
	assertApplyFails(t, path, p, "no-such-entity-type")
}

// TestApply_Process_UpdateDuplicateDrivesEntityFails is the drives_entities
// analog of TestApply_Process_DuplicateStepNameOnUpdateFails: an UPDATE
// proposal that resubmits a drives_entities slug already present on the
// target Process is rejected (attempting to re-append an existing entry is
// treated the same as attempting to reorder/redefine it), not silently
// deduplicated.
func TestApply_Process_UpdateDuplicateDrivesEntityFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, processBaseGraphWithLandedProcess())
	p := ProposedProcess{
		ID:             "PR-test-loop",
		DrivesEntities: []string{"feature-flag"}, // already present
	}
	assertApplyFails(t, path, p, "already drives entity")
}

// TestApply_Process_UpdateReplacesWhy covers the chosen behavior for Why on
// UPDATE: a non-empty p.Why REPLACES (not appends to) the existing Why --
// the reverse of steps/drives_entities, which only ever append. This mirrors
// ProposedRequirement.mutate's coalesceStr(p.Why, "", existing.Why) idiom.
func TestApply_Process_UpdateReplacesWhy(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, processBaseGraphWithLandedProcess())
	p := ProposedProcess{
		ID:  "PR-test-loop",
		Why: "a corrected rationale",
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	g := reload(t, path)
	proc, ok := findProcess(g, "PR-test-loop")
	if !ok {
		t.Fatalf("PR-test-loop missing after update")
	}
	if proc.Why != "a corrected rationale" {
		t.Errorf("Why = %q, want replaced value", proc.Why)
	}
	if len(proc.History) != 1 || !strings.Contains(proc.History[0].Summary, "why updated") {
		t.Errorf("History = %+v, want one entry mentioning 'why updated'", proc.History)
	}
}

// TestApply_Process_UpdateEmptyWhyPreservesExisting asserts the OTHER half
// of the coalesceStr contract: an omitted (empty) p.Why on an UPDATE that
// only appends a step leaves the existing Why untouched -- "empty means
// preserve", not "empty means clear" (a proposal author who wants to clear
// Why to empty has no sentinel for that yet, mirroring
// ProposedEntityType's UPDATE mode, which does not support editing Why at
// all; this is a milder version -- REPLACE is supported, CLEAR is not).
func TestApply_Process_UpdateEmptyWhyPreservesExisting(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, processBaseGraphWithLandedProcess())
	p := ProposedProcess{
		ID:             "PR-test-loop",
		DrivesEntities: []string{"release"},
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	g := reload(t, path)
	proc, ok := findProcess(g, "PR-test-loop")
	if !ok {
		t.Fatalf("PR-test-loop missing after update")
	}
	if proc.Why != "the original rationale" {
		t.Errorf("Why = %q, want unchanged original value", proc.Why)
	}
}

// TestApply_Process_UpdateRolesRequiredWithoutStepsFails covers the
// validate()-level shape guard: an UPDATE-shaped proposal (no steps) that
// still carries a non-empty roles_required is rejected -- roles_required
// declares roles used by THIS proposal's steps, and with zero steps in the
// proposal there is nothing for it to declare (a stray roles_required here
// is very likely a copy-paste mistake from a steps-bearing proposal).
func TestApply_Process_UpdateRolesRequiredWithoutStepsFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, processBaseGraphWithLandedProcess())
	p := ProposedProcess{
		ID:             "PR-test-loop",
		DrivesEntities: []string{"release"},
		RolesRequired:  []string{"reviewer"},
	}
	assertApplyFails(t, path, p, "'roles_required' must be empty when 'steps' is empty")
}

// TestApply_Process_UpdateNoStepsNoDrivesEntitiesFails covers the remaining
// validate()-level shape guard: an UPDATE-shaped proposal (existing id, no
// steps) that ALSO carries no drives_entities has nothing to do and is
// rejected rather than silently accepted as a no-op.
func TestApply_Process_UpdateNoStepsNoDrivesEntitiesFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, processBaseGraphWithLandedProcess())
	p := ProposedProcess{ID: "PR-test-loop"}
	assertApplyFails(t, path, p, "must supply either 'steps'")
}
