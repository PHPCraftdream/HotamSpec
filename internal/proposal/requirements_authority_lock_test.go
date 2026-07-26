package proposal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// requirementsAuthorityCodeGraph returns baseGraph() unmodified —
// RequirementsAuthorityCode is NOT set here because internal/loader.LoadGraph
// always OVERWRITES Graph.RequirementsAuthorityCode from the manifest.json
// sitting next to graph.json on every load (see
// loader.resolveRequirementsAuthorityCode, called from LoadGraph); a field
// set directly on an in-memory ontology.Graph value does not survive Apply's
// own loader.LoadGraph reload. writeTempRequirementsAuthorityCodeGraph
// (below) is what actually makes a fixture requirements-authority-locked, by
// writing a real manifest.json with requirements_authority: "code" alongside
// the graph — mirrors self_hosting_lock_test.go's
// selfHostingGraph/writeTempSelfHostingGraph pair exactly.
func requirementsAuthorityCodeGraph() *ontology.Graph {
	return baseGraph()
}

// writeTempRequirementsAuthorityCodeGraph writes g to a temp graph.json
// (writeTempGraph) PLUS a sibling manifest.json declaring
// {"requirements_authority": "code"} — the real on-disk shape
// internal/loader.LoadGraph reads Graph.RequirementsAuthorityCode from (see
// requirementsAuthorityCodeGraph's doc comment above). Every
// requirements-authority-lock test in this file uses this helper rather than
// writeTempGraph(t, requirementsAuthorityCodeGraph()) alone, since
// Apply/ApplyBatch always reload through loader.LoadGraph before
// applyToGraph ever sees the graph.
func writeTempRequirementsAuthorityCodeGraph(t *testing.T, g *ontology.Graph) string {
	t.Helper()
	path := writeTempGraph(t, g)
	manifestPath := filepath.Join(filepath.Dir(path), "manifest.json")
	if err := os.WriteFile(manifestPath, []byte(`{"requirements_authority": "code", "parent": null}`), 0o644); err != nil {
		t.Fatalf("write manifest.json: %v", err)
	}
	return path
}

// TestApplyToGraph_RequirementsAuthorityCode_RequirementCreate_Refused proves
// the CREATE-path lock: a domain with manifest.json's
// "requirements_authority": "code" refuses a hand-authored ProposedRequirement,
// with an error naming spec/requirements.go and `hotam sync-domain` — the
// consumer-domain analogue of
// TestApplyToGraph_SelfHosting_RequirementCreate_KnownID_NamesSourceFile.
// Unlike the self-hosting lock (which branches on whether the ID is already
// registered in one of internal/selfspec's 27 thematic files), this lock
// names the single fixed spec/requirements.go path unconditionally — there is
// no thematic split for a consumer domain to look up.
func TestApplyToGraph_RequirementsAuthorityCode_RequirementCreate_Refused(t *testing.T) {
	t.Parallel()
	path := writeTempRequirementsAuthorityCodeGraph(t, requirementsAuthorityCodeGraph())

	p := ProposedRequirement{
		ID:     "R-new-consumer-requirement",
		Claim:  "an attempted hand-authored edit to a requirements-authority-locked domain",
		Owner:  "sa",
		Status: ontology.StatusDRAFT,
	}
	err := Apply(path, today, p)
	if err == nil {
		t.Fatal("expected the requirements-authority lock to refuse this CREATE, got nil error")
	}
	msg := err.Error()
	for _, want := range []string{
		"R-new-consumer-requirement",
		"spec/requirements.go",
		"hotam sync-domain",
	} {
		if !containsString(msg, want) {
			t.Errorf("error = %q, want it to contain %q", msg, want)
		}
	}

	// Graph on disk must be untouched.
	after := readFile(t, path)
	if containsString(after, "an attempted hand-authored edit") {
		t.Error("graph on disk was mutated despite the requirements-authority lock refusing the CREATE")
	}
}

// TestApplyToGraph_RequirementsAuthorityCode_RequirementUpdate_Refused proves
// the UPDATE path is refused identically to CREATE, and that the graph is
// left unmutated (checked BEFORE a.mutate() runs) — mirrors
// TestApplyToGraph_SelfHosting_RequirementUpdate_KnownID_Refused.
func TestApplyToGraph_RequirementsAuthorityCode_RequirementUpdate_Refused(t *testing.T) {
	t.Parallel()
	path := writeTempRequirementsAuthorityCodeGraph(t, requirementsAuthorityCodeGraph())

	p := ProposedRequirement{
		ID:     "R-1",
		Claim:  "an attempted hand-authored UPDATE to an existing requirements-authority-locked node",
		Owner:  "sa",
		Status: ontology.StatusSETTLED,
	}
	assertApplyFails(t, path, p, "hotam sync-domain")

	g := reload(t, path)
	r, ok := findReq(g, "R-1")
	if !ok {
		t.Fatal("R-1 unexpectedly removed from the graph")
	}
	if r.Claim != "claim R-1" {
		t.Errorf("R-1.Claim = %q, want unchanged %q — the requirements-authority lock must refuse BEFORE mutation", r.Claim, "claim R-1")
	}
}

// TestApplyToGraph_RequirementsAuthorityCode_Rejection_MentionsReplacesRelation
// proves the Rejection-specific lock message: it must carry everything the
// generic Requirement lock message does (naming the rejected ID, hotam
// sync-domain) PLUS an explicit reminder that landing a Rejection also
// APPENDS a 'replaces' Relation onto the successor node named in
// replaced_by — mirrors
// TestApplyToGraph_SelfHosting_Rejection_MentionsReplacesRelation.
func TestApplyToGraph_RequirementsAuthorityCode_Rejection_MentionsReplacesRelation(t *testing.T) {
	t.Parallel()
	path := writeTempRequirementsAuthorityCodeGraph(t, requirementsAuthorityCodeGraph())

	p := ProposedRejection{
		RequirementID: "R-1",
		Reason:        "superseded",
		ReplacedBy:    []string{"R-2"},
	}
	err := Apply(path, today, p)
	if err == nil {
		t.Fatal("expected the requirements-authority lock to refuse this Rejection, got nil error")
	}
	msg := err.Error()
	for _, want := range []string{
		"R-1",
		"hotam sync-domain",
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
		t.Error("R-1.Status was mutated to REJECTED despite the requirements-authority lock refusing the proposal")
	}
	r2, ok := findReq(g, "R-2")
	if !ok {
		t.Fatal("R-2 unexpectedly removed from the graph")
	}
	for _, rel := range r2.Relations {
		if rel.Kind == "replaces" && rel.Target == "R-1" {
			t.Error("R-2 unexpectedly gained a 'replaces' relation despite the requirements-authority lock refusing the proposal")
		}
	}
}

// TestApplyToGraph_RequirementsAuthorityCode_OtherKindsUnaffected proves the
// lock is narrowly scoped to Requirement/Rejection, exactly like the
// self-hosting lock — mirrors
// TestApplyToGraph_SelfHosting_OtherKindsUnaffected's representative sample.
func TestApplyToGraph_RequirementsAuthorityCode_OtherKindsUnaffected(t *testing.T) {
	t.Parallel()

	t.Run("Stakeholder", func(t *testing.T) {
		t.Parallel()
		path := writeTempRequirementsAuthorityCodeGraph(t, requirementsAuthorityCodeGraph())
		p := ProposedStakeholder{ID: "new-sh", Name: "New Stakeholder", Domain: "x"}
		if err := Apply(path, today, p); err != nil {
			t.Fatalf("Apply Stakeholder on a requirements-authority-locked graph should succeed (lock is Requirement/Rejection-only), got: %v", err)
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
		path := writeTempRequirementsAuthorityCodeGraph(t, requirementsAuthorityCodeGraph())
		p := ProposedAxis{Slug: "new-axis", Description: "a new tension axis"}
		if err := Apply(path, today, p); err != nil {
			t.Fatalf("Apply Axis on a requirements-authority-locked graph should succeed (lock is Requirement/Rejection-only), got: %v", err)
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
		path := writeTempRequirementsAuthorityCodeGraph(t, requirementsAuthorityCodeGraph())
		p := ProposedAssumption{ID: "A-new", Statement: "a new assumption", Status: ontology.AssumptionHOLDS, Owner: "sa"}
		if err := Apply(path, today, p); err != nil {
			t.Fatalf("Apply Assumption on a requirements-authority-locked graph should succeed (lock is Requirement/Rejection-only), got: %v", err)
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

// TestApplyToGraph_RequirementsAuthorityCode_Unset_RequirementAndRejection_Unaffected
// proves the negative control: a graph with RequirementsAuthorityCode ==
// false (or the zero value, as every synthetic fixture across this package
// already is, and as baseGraph's own writeTempGraph-without-manifest-override
// path leaves it) is completely untouched by the new lock — mirrors
// TestApplyToGraph_NonSelfHosting_RequirementAndRejection_Unaffected.
func TestApplyToGraph_RequirementsAuthorityCode_Unset_RequirementAndRejection_Unaffected(t *testing.T) {
	t.Parallel()

	t.Run("Requirement CREATE", func(t *testing.T) {
		t.Parallel()
		path := writeTempGraph(t, baseGraph()) // RequirementsAuthorityCode is false (zero value)
		p := ProposedRequirement{
			ID: "R-new-non-locked-consumer", Claim: "a new requirement in a domain without the requirements-authority lock",
			Owner: "sa", Status: ontology.StatusDRAFT,
		}
		if err := Apply(path, today, p); err != nil {
			t.Fatalf("Apply Requirement CREATE on a non-locked graph should succeed, got: %v", err)
		}
		g := reload(t, path)
		if _, ok := findReq(g, "R-new-non-locked-consumer"); !ok {
			t.Error("R-new-non-locked-consumer not found after apply")
		}
	})

	t.Run("Requirement UPDATE", func(t *testing.T) {
		t.Parallel()
		path := writeTempGraph(t, baseGraph())
		p := ProposedRequirement{
			ID: "R-1", Claim: "an updated claim on a non-locked domain",
			Owner: "sa", Status: ontology.StatusSETTLED,
		}
		if err := Apply(path, today, p); err != nil {
			t.Fatalf("Apply Requirement UPDATE on a non-locked graph should succeed, got: %v", err)
		}
		g := reload(t, path)
		r, ok := findReq(g, "R-1")
		if !ok {
			t.Fatal("R-1 missing after apply")
		}
		if r.Claim != "an updated claim on a non-locked domain" {
			t.Errorf("R-1.Claim = %q, want the updated claim", r.Claim)
		}
	})

	t.Run("Rejection", func(t *testing.T) {
		t.Parallel()
		path := writeTempGraph(t, baseGraph())
		p := ProposedRejection{RequirementID: "R-1", Reason: "superseded", ReplacedBy: []string{"R-2"}}
		if err := Apply(path, today, p); err != nil {
			t.Fatalf("Apply Rejection on a non-locked graph should succeed, got: %v", err)
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
