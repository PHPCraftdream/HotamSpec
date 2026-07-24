package proposal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// selfHostingGraph returns baseGraph() unmodified — SelfHosting is NOT set
// here because internal/loader.LoadGraph always OVERWRITES Graph.SelfHosting
// from the manifest.json sitting next to graph.json on every load (see
// loader.resolveSelfHosting, called from LoadGraph); a field set directly on
// an in-memory ontology.Graph value does not survive Apply's own
// loader.LoadGraph reload. writeTempSelfHostingGraph (below) is what
// actually makes a fixture self-hosting, by writing a real manifest.json
// with self_hosting: true alongside the graph.
func selfHostingGraph() *ontology.Graph {
	return baseGraph()
}

// writeTempSelfHostingGraph writes g to a temp graph.json (writeTempGraph)
// PLUS a sibling manifest.json declaring {"self_hosting": true} — the real
// on-disk shape internal/loader.LoadGraph reads Graph.SelfHosting from (see
// selfHostingGraph's doc comment above). Every self-hosting-lock test in
// this file uses this helper rather than writeTempSelfHostingGraph(t, selfHostingGraph())
// alone, since Apply/ApplyBatch always reload through loader.LoadGraph
// before applyToGraph ever sees the graph.
func writeTempSelfHostingGraph(t *testing.T, g *ontology.Graph) string {
	t.Helper()
	path := writeTempGraph(t, g)
	manifestPath := filepath.Join(filepath.Dir(path), "manifest.json")
	if err := os.WriteFile(manifestPath, []byte(`{"self_hosting": true, "parent": null}`), 0o644); err != nil {
		t.Fatalf("write manifest.json: %v", err)
	}
	return path
}

// TestApplyToGraph_SelfHosting_RequirementCreate_KnownID_NamesSourceFile
// proves the CREATE-path lock for an ID that IS already registered in
// internal/selfspec: the error must name the exact requirements_<topic>.go
// file selfspec.SourceFileFor resolves it to, and must point at `hotam
// sync-self` as the sanctioned path. R-operator-acting-facet is registered
// in requirements_operator.go (internal/selfspec/requirements_operator.go).
func TestApplyToGraph_SelfHosting_RequirementCreate_KnownID_NamesSourceFile(t *testing.T) {
	t.Parallel()
	path := writeTempSelfHostingGraph(t, selfHostingGraph())

	p := ProposedRequirement{
		ID:     "R-operator-acting-facet",
		Claim:  "an attempted hand-authored edit to a self-hosting-locked requirement",
		Owner:  "sa",
		Status: ontology.StatusDRAFT,
	}
	err := Apply(path, today, p)
	if err == nil {
		t.Fatal("expected the self-hosting lock to refuse this CREATE, got nil error")
	}
	msg := err.Error()
	for _, want := range []string{
		"R-operator-acting-facet",
		"requirements_operator.go",
		"hotam sync-self",
	} {
		if !containsString(msg, want) {
			t.Errorf("error = %q, want it to contain %q", msg, want)
		}
	}

	// Graph on disk must be untouched.
	after := readFile(t, path)
	if containsString(after, "an attempted hand-authored edit") {
		t.Error("graph on disk was mutated despite the self-hosting lock refusing the CREATE")
	}
}

// TestApplyToGraph_SelfHosting_RequirementUpdate_KnownID_Refused proves the
// UPDATE path is refused identically to CREATE: R-1 already exists in
// selfHostingGraph's baseGraph (baseGraph is a synthetic fixture, not a real
// selfspec-registered ID, so this exercises the "not yet registered" message
// shape via the UPDATE branch specifically — findRequirementIndex finds it
// in the GRAPH, but selfspec.SourceFileFor still returns false since R-1 was
// never registered in any requirements_<topic>.go file).
func TestApplyToGraph_SelfHosting_RequirementUpdate_KnownID_Refused(t *testing.T) {
	t.Parallel()
	path := writeTempSelfHostingGraph(t, selfHostingGraph())

	p := ProposedRequirement{
		ID:     "R-1",
		Claim:  "an attempted hand-authored UPDATE to an existing self-hosting node",
		Owner:  "sa",
		Status: ontology.StatusSETTLED,
	}
	assertApplyFails(t, path, p, "hotam sync-self")

	g := reload(t, path)
	r, ok := findReq(g, "R-1")
	if !ok {
		t.Fatal("R-1 unexpectedly removed from the graph")
	}
	if r.Claim != "claim R-1" {
		t.Errorf("R-1.Claim = %q, want unchanged %q — the self-hosting lock must refuse BEFORE mutation", r.Claim, "claim R-1")
	}
}

// TestApplyToGraph_SelfHosting_RequirementCreate_UnknownID_NamesPackageNotFile
// proves the CREATE-path lock for a brand-new ID that is NOT registered in
// internal/selfspec anywhere: the error must name the internal/selfspec
// package and the requirements_<topic>.go filing convention, and must NOT
// claim a specific (nonexistent) file.
func TestApplyToGraph_SelfHosting_RequirementCreate_UnknownID_NamesPackageNotFile(t *testing.T) {
	t.Parallel()
	path := writeTempSelfHostingGraph(t, selfHostingGraph())

	p := ProposedRequirement{
		ID:     "R-brand-new-not-yet-registered-anywhere",
		Claim:  "a new requirement nobody has placed into a topic file yet",
		Owner:  "sa",
		Status: ontology.StatusDRAFT,
	}
	err := Apply(path, today, p)
	if err == nil {
		t.Fatal("expected the self-hosting lock to refuse this CREATE, got nil error")
	}
	msg := err.Error()
	for _, want := range []string{
		"R-brand-new-not-yet-registered-anywhere",
		"internal/selfspec",
		"requirements_<topic>.go",
		"hotam sync-self",
	} {
		if !containsString(msg, want) {
			t.Errorf("error = %q, want it to contain %q", msg, want)
		}
	}
	// Must not claim a specific, nonexistent requirements_*.go file exists.
	if containsString(msg, "is registered in internal/selfspec/") {
		t.Errorf("error = %q, wrongly claims a specific file for an unregistered ID", msg)
	}
}

// TestApplyToGraph_SelfHosting_Rejection_MentionsReplacesRelation proves the
// Rejection-specific lock message: it must carry everything the generic
// Requirement lock message does (naming the rejected ID, hotam sync-self)
// PLUS an explicit reminder that landing a Rejection also appends a
// 'replaces' Relation onto the successor node named in replaced_by — a
// structural field on a DIFFERENT node than the one being rejected (see
// ProposedRejection.mutate in mutate.go), which a manual Go-registry
// migration must not forget.
func TestApplyToGraph_SelfHosting_Rejection_MentionsReplacesRelation(t *testing.T) {
	t.Parallel()
	path := writeTempSelfHostingGraph(t, selfHostingGraph())

	p := ProposedRejection{
		RequirementID: "R-1",
		Reason:        "superseded",
		ReplacedBy:    []string{"R-2"},
	}
	err := Apply(path, today, p)
	if err == nil {
		t.Fatal("expected the self-hosting lock to refuse this Rejection, got nil error")
	}
	msg := err.Error()
	for _, want := range []string{
		"R-1",
		"hotam sync-self",
		"replaces",
		"successor",
	} {
		if !containsString(msg, want) {
			t.Errorf("error = %q, want it to contain %q", msg, want)
		}
	}

	// Neither the rejected requirement nor the would-be successor's
	// Relations may have been mutated.
	g := reload(t, path)
	r1, ok := findReq(g, "R-1")
	if !ok {
		t.Fatal("R-1 unexpectedly removed from the graph")
	}
	if r1.Status == ontology.StatusREJECTED {
		t.Error("R-1.Status was mutated to REJECTED despite the self-hosting lock refusing the proposal")
	}
	r2, ok := findReq(g, "R-2")
	if !ok {
		t.Fatal("R-2 unexpectedly removed from the graph")
	}
	for _, rel := range r2.Relations {
		if rel.Kind == "replaces" && rel.Target == "R-1" {
			t.Error("R-2 unexpectedly gained a 'replaces' relation despite the self-hosting lock refusing the proposal")
		}
	}
}

// TestApplyToGraph_SelfHosting_OtherKindsUnaffected proves the lock is
// narrowly scoped to Requirement/Rejection: every other Proposal kind must
// keep applying normally against a self-hosting graph. Exercises a
// representative, cheap-to-construct sample (Stakeholder, Axis, Assumption)
// rather than the full kind roster — cmd/hotam's own tests cover
// GateSignoffBatch/ReviewMark/Conflict* end-to-end against real domain
// fixtures; this is the internal/proposal-level structural proof that
// applyToGraph's switch only matches the two locked kinds.
func TestApplyToGraph_SelfHosting_OtherKindsUnaffected(t *testing.T) {
	t.Parallel()

	t.Run("Stakeholder", func(t *testing.T) {
		t.Parallel()
		path := writeTempSelfHostingGraph(t, selfHostingGraph())
		p := ProposedStakeholder{ID: "new-sh", Name: "New Stakeholder", Domain: "x"}
		if err := Apply(path, today, p); err != nil {
			t.Fatalf("Apply Stakeholder on a self-hosting graph should succeed (lock is Requirement/Rejection-only), got: %v", err)
		}
		g := reload(t, path)
		found := false
		for _, sh := range g.Stakeholders {
			if sh.ID == "new-sh" {
				found = true
			}
		}
		if !found {
			t.Error("new-sh stakeholder not found after apply")
		}
	})

	t.Run("Axis", func(t *testing.T) {
		t.Parallel()
		path := writeTempSelfHostingGraph(t, selfHostingGraph())
		p := ProposedAxis{Slug: "new-axis", Description: "a new tension axis"}
		if err := Apply(path, today, p); err != nil {
			t.Fatalf("Apply Axis on a self-hosting graph should succeed (lock is Requirement/Rejection-only), got: %v", err)
		}
		g := reload(t, path)
		found := false
		for _, a := range g.Axes {
			if a.Slug == "new-axis" {
				found = true
			}
		}
		if !found {
			t.Error("new-axis not found after apply")
		}
	})

	t.Run("Assumption", func(t *testing.T) {
		t.Parallel()
		path := writeTempSelfHostingGraph(t, selfHostingGraph())
		p := ProposedAssumption{ID: "A-new", Statement: "a new assumption", Status: ontology.AssumptionHOLDS, Owner: "sa"}
		if err := Apply(path, today, p); err != nil {
			t.Fatalf("Apply Assumption on a self-hosting graph should succeed (lock is Requirement/Rejection-only), got: %v", err)
		}
		g := reload(t, path)
		found := false
		for _, a := range g.Assumptions {
			if a.ID == "A-new" {
				found = true
			}
		}
		if !found {
			t.Error("A-new assumption not found after apply")
		}
	})
}

// TestApplyToGraph_NonSelfHosting_RequirementAndRejection_Unaffected proves
// the negative control: a graph with SelfHosting == false (or the zero
// value, as every synthetic fixture across this package already is) is
// completely untouched by the new lock — ProposedRequirement (CREATE and
// UPDATE) and ProposedRejection continue to apply exactly as before task
// #350/RAC-B3.
func TestApplyToGraph_NonSelfHosting_RequirementAndRejection_Unaffected(t *testing.T) {
	t.Parallel()

	t.Run("Requirement CREATE", func(t *testing.T) {
		t.Parallel()
		path := writeTempGraph(t, baseGraph()) // SelfHosting is false (zero value)
		p := ProposedRequirement{
			ID: "R-new-non-self-hosting", Claim: "a new requirement in a non-self-hosting domain",
			Owner: "sa", Status: ontology.StatusDRAFT,
		}
		if err := Apply(path, today, p); err != nil {
			t.Fatalf("Apply Requirement CREATE on a non-self-hosting graph should succeed, got: %v", err)
		}
		g := reload(t, path)
		if _, ok := findReq(g, "R-new-non-self-hosting"); !ok {
			t.Error("R-new-non-self-hosting not found after apply")
		}
	})

	t.Run("Requirement UPDATE", func(t *testing.T) {
		t.Parallel()
		path := writeTempGraph(t, baseGraph())
		p := ProposedRequirement{
			ID: "R-1", Claim: "an updated claim on a non-self-hosting domain",
			Owner: "sa", Status: ontology.StatusSETTLED,
		}
		if err := Apply(path, today, p); err != nil {
			t.Fatalf("Apply Requirement UPDATE on a non-self-hosting graph should succeed, got: %v", err)
		}
		g := reload(t, path)
		r, ok := findReq(g, "R-1")
		if !ok {
			t.Fatal("R-1 missing after apply")
		}
		if r.Claim != "an updated claim on a non-self-hosting domain" {
			t.Errorf("R-1.Claim = %q, want the updated claim", r.Claim)
		}
	})

	t.Run("Rejection", func(t *testing.T) {
		t.Parallel()
		path := writeTempGraph(t, baseGraph())
		p := ProposedRejection{RequirementID: "R-1", Reason: "superseded", ReplacedBy: []string{"R-2"}}
		if err := Apply(path, today, p); err != nil {
			t.Fatalf("Apply Rejection on a non-self-hosting graph should succeed, got: %v", err)
		}
		g := reload(t, path)
		r1, ok := findReq(g, "R-1")
		if !ok {
			t.Fatal("R-1 missing after apply")
		}
		if r1.Status != ontology.StatusREJECTED {
			t.Errorf("R-1.Status = %q, want REJECTED", r1.Status)
		}
		r2, ok := findReq(g, "R-2")
		if !ok {
			t.Fatal("R-2 missing after apply")
		}
		hasReplaces := false
		for _, rel := range r2.Relations {
			if rel.Kind == "replaces" && rel.Target == "R-1" {
				hasReplaces = true
			}
		}
		if !hasReplaces {
			t.Error("R-2 did not gain the expected 'replaces' relation to R-1")
		}
	})
}
