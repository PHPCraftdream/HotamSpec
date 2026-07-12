package ontology

import (
	"reflect"
	"testing"
)

func ptrString(s string) *string { return &s }

func buildSmokeGraph() *Graph {
	axes := []Axis{
		{Slug: "latency-vs-completeness", Description: "speed vs full check"},
	}
	stakeholders := []Stakeholder{
		{ID: "finance", Name: "Finance", Domain: "money"},
		{ID: "platform", Name: "Platform", Domain: "latency/SLA"},
	}
	assumptions := []Assumption{
		{ID: "A-single-customer", Statement: "one customer per account", Status: AssumptionHOLDS, Owner: "finance"},
		{ID: "A-api-v3", Statement: "api_version >= 3", Status: AssumptionUNCERTAIN, Owner: "platform"},
		{ID: "A-dead-one", Statement: "legacy flag still set", Status: AssumptionDEAD, Owner: "platform"},
	}
	requirements := []Requirement{
		{
			ID: "R-1", Claim: "the system shall settle fast", Owner: "finance", Status: StatusSETTLED,
			Assumptions: []string{"A-single-customer", "A-api-v3"},
			Relations:   []Relation{{Kind: "depends_on", Target: "R-2"}},
			Enforcement: EnforcementSTRUCTURAL, Enforceability: EnforceabilityENFORCEABLE,
		},
		{
			ID: "R-2", Claim: "the system shall check fully", Owner: "finance", Status: StatusSETTLED,
			Assumptions: []string{"A-single-customer"},
			Enforcement: EnforcementENFORCED, Enforceability: EnforceabilityENFORCEABLE,
			EnforcedBy: []string{"check_full"},
		},
		{
			ID: "R-3", Claim: "the system shall report", Owner: "platform", Status: StatusSETTLED,
			Assumptions: []string{"A-single-customer", "A-api-v3"},
		},
		{
			ID: "R-4", Claim: "the system shall log verbosely", Owner: "platform", Status: StatusREJECTED,
		},
		{
			ID: "R-5", Claim: "the system shall log compactly", Owner: "platform", Status: StatusSETTLED,
			Relations: []Relation{{Kind: "replaces", Target: "R-4"}},
		},
	}
	conflicts := []Conflict{
		{
			ID: "C-12", Axis: "latency-vs-completeness", Context: "checkout peak load",
			Members: []string{"R-1", "R-3"}, Steward: "platform", Lifecycle: ConflictDETECTED,
		},
	}

	entityTypes := []EntityType{
		{
			Slug: "order", Description: "an order",
			Lifecycle: Lifecycle{
				Slug: "order-states",
				States: []State{
					{Name: "PENDING", Kind: StateKindInitial},
					{Name: "FULFILLED", Kind: StateKindQuiescent},
					{Name: "CANCELLED", Kind: StateKindQuiescent},
				},
				Transitions: []Transition{
					{Src: "PENDING", Dst: "FULFILLED", Event: "fulfill"},
					{Src: "PENDING", Dst: "CANCELLED", Event: "cancel"},
				},
			},
		},
	}
	processes := []Process{
		{
			ID: "PR-ship", Lifecycle: ProcessLifecycle, DrivesEntities: []string{"order"},
			Steps: []Step{{Name: "ship", RequiresRole: "ops", Invokes: "order.fulfill"}},
			RolesRequired: []string{"ops"},
		},
		{
			ID: "PR-cancel", Lifecycle: ProcessLifecycle, DrivesEntities: []string{"order"},
			Steps: []Step{{Name: "abort", RequiresRole: "ops", Invokes: "order.cancel"}},
			RolesRequired: []string{"ops"},
		},
	}

	return &Graph{
		Axes:         axes,
		Stakeholders: stakeholders,
		Assumptions:  assumptions,
		Requirements: requirements,
		Conflicts:    conflicts,
		EntityTypes:  entityTypes,
		Processes:    processes,
	}
}

func TestGraphIsEmpty(t *testing.T) {
	if !(&Graph{}).IsEmpty() {
		t.Fatal("empty graph should be empty")
	}
	g := buildSmokeGraph()
	if g.IsEmpty() {
		t.Fatal("populated graph should not be empty")
	}
}

func TestLookupHelpers(t *testing.T) {
	g := buildSmokeGraph()
	if _, ok := AxisSlugs(g)["latency-vs-completeness"]; !ok {
		t.Fatal("axis slug missing")
	}
	if _, ok := RequirementIDs(g)["R-2"]; !ok {
		t.Fatal("requirement id missing")
	}
	if _, ok := AssumptionIDs(g)["A-api-v3"]; !ok {
		t.Fatal("assumption id missing")
	}
	r, ok := RequirementByID(g, "R-1")
	if !ok || r.ID != "R-1" {
		t.Fatal("RequirementByID R-1 failed")
	}
	if _, ok := RequirementByID(g, "nope"); ok {
		t.Fatal("RequirementByID should miss for unknown id")
	}
}

func TestRequirementPredicates(t *testing.T) {
	r := Requirement{Status: "OPEN(which segment?)", Enforcement: EnforcementPROSE, Enforceability: EnforceabilityENFORCEABLE}
	if !r.IsOpen() {
		t.Fatal("OPEN(...) should be open")
	}
	if !r.IsCloseableDebt() {
		t.Fatal("PROSE+ENFORCEABLE should be closeable debt")
	}
	enf := Requirement{Status: StatusSETTLED, Enforcement: EnforcementENFORCED, Enforceability: EnforceabilityENFORCEABLE}
	if enf.IsCloseableDebt() {
		t.Fatal("ENFORCED should not be closeable debt")
	}
	prose := Requirement{Status: StatusSETTLED, Enforcement: EnforcementPROSE, Enforceability: EnforceabilityINHERENTLY_PROSE}
	if prose.IsCloseableDebt() {
		t.Fatal("INHERENTLY_PROSE should not be closeable debt")
	}
}

func TestConflictIdentity(t *testing.T) {
	id := ConflictIdentity("latency-vs-completeness", "checkout peak load")
	if len(id) != 10 || id[:2] != "C-" {
		t.Fatalf("unexpected conflict id %q", id)
	}
	if ConflictIdentity("latency-vs-completeness", "  Checkout   PEAK\tload ") != id {
		t.Fatal("identity must be whitespace/case-insensitive on context")
	}
	if ConflictIdentity("other-axis", "checkout peak load") == id {
		t.Fatal("identity must depend on axis")
	}
}

func TestConflictPredicates(t *testing.T) {
	if !(Conflict{Lifecycle: ConflictDETECTED}).IsUnresolved() {
		t.Fatal("DETECTED should be unresolved")
	}
	if !(Conflict{Lifecycle: "DECIDED(because)"}).IsDecided() {
		t.Fatal("DECIDED(...) should be decided")
	}
	if !(Conflict{Lifecycle: "HELD(reason)"}).IsHeld() {
		t.Fatal("HELD(...) should be held")
	}
	if (Conflict{Lifecycle: "DECIDED(x)"}).IsUnresolved() {
		t.Fatal("DECIDED should not be unresolved")
	}
}

func TestReplacesMap(t *testing.T) {
	g := buildSmokeGraph()
	rm := ReplacesMap(g)
	got, ok := rm["R-4"]
	if !ok {
		t.Fatal("R-4 should have a successor")
	}
	if !reflect.DeepEqual(got, []string{"R-5"}) {
		t.Fatalf("R-4 successors = %v, want [R-5]", got)
	}
}

func TestDependencyChains(t *testing.T) {
	g := buildSmokeGraph()
	chains := DependencyChains(g)
	want := [][]string{{"R-2", "R-1"}}
	if !reflect.DeepEqual(chains, want) {
		t.Fatalf("chains = %v, want %v", chains, want)
	}
}

func TestIndependentSubgraphs(t *testing.T) {
	g := buildSmokeGraph()
	comps := IndependentSubgraphs(g)
	want := [][]string{{"R-1", "R-2"}, {"R-3"}, {"R-4"}, {"R-5"}}
	if !reflect.DeepEqual(comps, want) {
		t.Fatalf("subgraphs = %v, want %v", comps, want)
	}
}

func TestDriftTraversal(t *testing.T) {
	g := buildSmokeGraph()
	onShared := RequirementsOnAssumption(g, "A-single-customer")
	if len(onShared) != 3 {
		t.Fatalf("requirements on A-single-customer = %d, want 3", len(onShared))
	}
	dead := DeadAssumptions(g)
	if len(dead) != 1 || dead[0].ID != "A-dead-one" {
		t.Fatalf("dead = %v, want [A-dead-one]", dead)
	}
	unc := UncertainAssumptions(g)
	if len(unc) != 1 || unc[0].ID != "A-api-v3" {
		t.Fatalf("uncertain = %v, want [A-api-v3]", unc)
	}
}

func TestConflictClusteringAndPairs(t *testing.T) {
	g := buildSmokeGraph()
	byAxis := ConflictsByAxis(g)
	if len(byAxis["latency-vs-completeness"]) != 1 {
		t.Fatalf("cluster size = %d, want 1", len(byAxis["latency-vs-completeness"]))
	}
	pairs := MembersPairSet(g)
	if _, ok := pairs["R-1\x00R-3"]; !ok {
		t.Fatalf("mediated pair missing, got %v", pairs)
	}
}

func TestAssumptionReferenceCounts(t *testing.T) {
	g := buildSmokeGraph()
	rc := AssumptionReferenceCounts(g)
	if rc["A-single-customer"] != 3 {
		t.Fatalf("A-single-customer count = %d, want 3", rc["A-single-customer"])
	}
	if rc["A-api-v3"] != 2 {
		t.Fatalf("A-api-v3 count = %d, want 2", rc["A-api-v3"])
	}
	if _, ok := rc["A-dead-one"]; ok {
		t.Fatal("unreferenced assumption should be absent")
	}
}

func TestLatentConnectorSuspectsAndClusters(t *testing.T) {
	g := buildSmokeGraph()
	suspects := LatentConnectorSuspects(g)
	got := make([][2]string, 0, len(suspects))
	for _, s := range suspects {
		got = append(got, [2]string{s.Left, s.Right})
	}
	want := [][2]string{{"R-1", "R-2"}, {"R-2", "R-3"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("suspects = %v, want %v", got, want)
	}

	clusters := LatentConnectorClusters(g)
	if len(clusters) != 1 {
		t.Fatalf("clusters = %d, want 1 (both suspects share the A-single-customer signature)", len(clusters))
	}
	c := clusters[0]
	if !reflect.DeepEqual(c.Assumptions, []string{"A-single-customer"}) {
		t.Fatalf("cluster assumptions = %v, want [A-single-customer]", c.Assumptions)
	}
	if !reflect.DeepEqual(c.Requirements, []string{"R-1", "R-2", "R-3"}) {
		t.Fatalf("cluster requirements = %v, want [R-1 R-2 R-3]", c.Requirements)
	}
	if len(c.Pairs) != 2 {
		t.Fatalf("cluster pairs = %d, want 2", len(c.Pairs))
	}
}

func TestEntityStateConflictSuspects(t *testing.T) {
	g := buildSmokeGraph()
	suspects := EntityStateConflictSuspects(g)
	if len(suspects) != 1 {
		t.Fatalf("entity state suspects = %d, want 1", len(suspects))
	}
	s := suspects[0]
	if s.Left != "PR-cancel" || s.Right != "PR-ship" {
		t.Fatalf("suspect = {%s,%s}, want {PR-cancel,PR-ship}", s.Left, s.Right)
	}
}

func TestRequirementLifecycle(t *testing.T) {
	lc := RequirementStatusLifecycle
	st, ok := lc.Matches("OPEN(which?)")
	if !ok || st.Name != "OPEN" {
		t.Fatalf("OPEN(...) should match OPEN state, got %+v ok=%v", st, ok)
	}
	if _, ok := lc.Matches("DRAFT2"); ok {
		t.Fatal("DRAFT2 must not match DRAFT (prefix guard)")
	}
	if t1, ok := lc.TransitionFor("DRAFT", "accept"); !ok || t1.Dst != "SETTLED" {
		t.Fatalf("DRAFT+accept -> SETTLED, got %+v ok=%v", t1, ok)
	}
	if !lc.CanTransition("SETTLED", "OPEN") {
		t.Fatal("SETTLED -> OPEN should be allowed")
	}
	if lc.CanTransition("REJECTED", "SETTLED") {
		t.Fatal("REJECTED -> SETTLED should NOT be allowed")
	}
	if _, err := lc.Initial(); err != nil {
		t.Fatalf("initial state error: %v", err)
	}
}

func TestConflictLifecycle(t *testing.T) {
	lc := ConflictLifecycle
	if st, ok := lc.Matches("DECIDED(rationale)"); !ok || st.Name != "DECIDED" {
		t.Fatalf("DECIDED(...) should match DECIDED, got %+v", st)
	}
	if t1, ok := lc.TransitionFor("DETECTED", "steward-acknowledge"); !ok || t1.Dst != "ACKNOWLEDGED" {
		t.Fatalf("DETECTED+ack -> ACKNOWLEDGED, got %+v", t1)
	}
	if t1, ok := lc.TransitionFor("ACKNOWLEDGED", "steward-decide"); !ok || t1.Dst != "DECIDED" {
		t.Fatalf("ACKNOWLEDGED+decide -> DECIDED, got %+v", t1)
	}
	if !lc.Cyclic {
		t.Fatal("conflict lifecycle should be cyclic")
	}
}

func TestCanonicalLifecyclesSanity(t *testing.T) {
	for _, lc := range []Lifecycle{RequirementStatusLifecycle, ConflictLifecycle, OperatorLifecycle, ProcessLifecycle, GoalLifecycle} {
		if len(lc.States) == 0 {
			t.Fatalf("%s has no states", lc.Slug)
		}
		names := lc.StateNames()
		for _, tr := range lc.Transitions {
			if _, ok := names[tr.Src]; !ok {
				t.Fatalf("%s transition src %q not a state", lc.Slug, tr.Src)
			}
			if _, ok := names[tr.Dst]; !ok {
				t.Fatalf("%s transition dst %q not a state", lc.Slug, tr.Dst)
			}
		}
		if _, err := lc.Initial(); err != nil {
			t.Fatalf("%s has no initial: %v", lc.Slug, err)
		}
	}
}

func TestEntityInstanceFieldValue(t *testing.T) {
	e := EntityInstance{
		ID:         "ENT-order-42",
		EntityType: "order",
		State:      "PENDING",
		FieldValues: [][2]string{
			{"amount", "100"},
			{"currency", "USD"},
		},
	}
	if v, ok := e.FieldValue("amount"); !ok || v != "100" {
		t.Fatalf("amount = %q ok=%v", v, ok)
	}
	if _, ok := e.FieldValue("missing"); ok {
		t.Fatal("missing field should return ok=false")
	}
}

func TestSortedKeys(t *testing.T) {
	m := map[string]struct{}{"b": {}, "a": {}, "c": {}}
	got := sortedKeys(m)
	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sortedKeys = %v, want %v", got, want)
	}
}
