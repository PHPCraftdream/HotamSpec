package proposal

import (
	"testing"

	"github.com/PHPCraftdream/HotamSpecGo/internal/ontology"
)

func TestApply_Requirement_Add(t *testing.T) {
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
	path := writeTempGraph(t, baseGraph())
	p := ProposedRequirement{ID: "R-new", Claim: "  ", Owner: "sa", Status: ontology.StatusDRAFT}
	assertApplyFails(t, path, p, "'claim'")
}

func TestApply_Requirement_UpdateAppendsHistory(t *testing.T) {
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
	if r.SettledAt != today {
		t.Errorf("SettledAt = %q, want %q", r.SettledAt, today)
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

func TestApply_ConflictTransition_Decided(t *testing.T) {
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
	cid := ontology.ConflictIdentity("cost-vs-flexibility", "shared scenario")
	path := writeTempGraph(t, baseGraph())
	p := ProposedConflictTransition{
		ConflictID:   cid,
		NewLifecycle: "DECIDED(chose option A)",
	}
	assertApplyFails(t, path, p, "decided_by")
}

func TestApply_ConflictTransition_HeldRequiresVariants(t *testing.T) {
	cid := ontology.ConflictIdentity("cost-vs-flexibility", "shared scenario")
	path := writeTempGraph(t, baseGraph())
	p := ProposedConflictTransition{
		ConflictID:   cid,
		NewLifecycle: "HELD(cannot resolve by amending members)",
		DecidedBy:    "outsider",
	}
	assertApplyFails(t, path, p, "2 distinct")
}

func TestApply_Rejection(t *testing.T) {
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
	path := writeTempGraph(t, baseGraph())
	p := ProposedRejection{RequirementID: "R-1", Reason: ""}
	assertApplyFails(t, path, p, "'reason'")
}

func TestApply_Conflict_Create(t *testing.T) {
	path := writeTempGraph(t, baseGraph())
	p := ProposedConflict{
		Axis:     "cost-vs-flexibility",
		Context:  "a brand new tension surface",
		Members:  []string{"R-1", "R-3"},
		Steward:  "outsider",
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
	path := writeTempGraph(t, graphWithOperator())
	p := ProposedOperatorBudget{
		OperatorID: "OP-1",
		NewLimit:   -1,
		NewMeasure: ontology.BudgetMeasureNODE_COUNT,
	}
	assertApplyFails(t, path, p, "new_limit")
}

func TestApply_Axis(t *testing.T) {
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
	path := writeTempGraph(t, baseGraph())
	p := ProposedAxis{
		Slug:        "cost-vs-flexibility",
		Description: "dup",
	}
	assertApplyFails(t, path, p, "duplicate")
}

func TestApply_Stakeholder(t *testing.T) {
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
	path := writeTempGraph(t, baseGraph())
	p := ProposedStakeholder{
		ID:     "outsider",
		Name:   "Dup",
		Domain: "x",
	}
	assertApplyFails(t, path, p, "duplicate")
}

func TestApply_Assumption_Create(t *testing.T) {
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
	path := writeTempGraph(t, baseGraph())
	p := ProposedAssumptionTransition{
		AssumptionID: "A-base",
		NewStatus:    ontology.AssumptionDEAD,
		Reason:       "falsified",
	}
	assertApplyFails(t, path, p, "decided_by")
}

func TestApply_ConflictMemberUpdate_Add(t *testing.T) {
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
	cid := ontology.ConflictIdentity("cost-vs-flexibility", "shared scenario")
	path := writeTempGraph(t, baseGraph())
	p := ProposedConflictMemberUpdate{
		ConflictID:    cid,
		RemoveMembers: []string{"R-2"},
	}
	assertApplyFails(t, path, p, ">= 2")
}

func TestApply_EntityType(t *testing.T) {
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
	path := writeTempGraph(t, baseGraph())
	p := ProposedRequirement{
		ID:     "not-r-prefixed",
		Claim:  "claim",
		Owner:  "sa",
		Status: ontology.StatusDRAFT,
	}
	assertApplyFails(t, path, p, "invariant violation")
}
