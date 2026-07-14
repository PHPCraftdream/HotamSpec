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
// no-op (which would silently drop a steward decision) nor a panic.
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
		Axis:    "cost-vs-flexibility",
		Context: "a brand new tension surface",
		Members: []string{"R-1", "R-3"},
		Steward: "outsider",
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

func TestApply_Conflict_StewardOwnsMemberFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedConflict{
		Axis:    "cost-vs-flexibility",
		Context: "another tension",
		Members: []string{"R-1", "R-2"},
		Steward: "sa",
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

func TestApply_Axis_DuplicateSlugFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedAxis{
		Slug:        "cost-vs-flexibility",
		Description: "dup",
	}
	assertApplyFails(t, path, p, "duplicate")
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
