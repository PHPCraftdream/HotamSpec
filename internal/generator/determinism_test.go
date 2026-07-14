package generator

import "testing"

func TestGenerator_DoubleRegenerateIsIdentical(t *testing.T) {
	t.Parallel()
	g := loadDomainGraph(t)
	pairs := []struct {
		name string
		a, b string
	}{
		{"Requirements", BuildRequirements(g, "hotam-spec-self", false), BuildRequirements(g, "hotam-spec-self", false)},
		{"Tensions", BuildTensions(g), BuildTensions(g)},
		{"Open", BuildOpen(g), BuildOpen(g)},
		{"Unenforced", BuildUnenforced(g), BuildUnenforced(g)},
		{"Glossary", BuildGlossary(g, false), BuildGlossary(g, false)},
		{"History", BuildHistory(g), BuildHistory(g)},
		{"Decisions", BuildDecisions(g), BuildDecisions(g)},
		{"Constitution", BuildConstitution(g, "hotam-spec-self", false), BuildConstitution(g, "hotam-spec-self", false)},
		{"Entities", BuildEntities(g, "hotam-spec-self"), BuildEntities(g, "hotam-spec-self")},
		{"RepoMap", BuildRepoMap(g, "hotam-spec-self", hotamSpecSelfFixtureGenDocs(), false, false, false), BuildRepoMap(g, "hotam-spec-self", hotamSpecSelfFixtureGenDocs(), false, false, false)},
		{"FrameworkInvariants", BuildFrameworkInvariants(g, "hotam-spec-self"), BuildFrameworkInvariants(g, "hotam-spec-self")},
		{"LiveState", BuildLiveState(g, 1000, "2026-07-12"), BuildLiveState(g, 1000, "2026-07-12")},
		{"AtomsOperator", BuildAtomsOperator(g), BuildAtomsOperator(g)},
		{"AtomsSubstrate", BuildAtomsSubstrate(g), BuildAtomsSubstrate(g)},
		{"AtomsDiscipline", BuildAtomsDiscipline(g), BuildAtomsDiscipline(g)},
		{"AtomsCheck", BuildAtomsCheck(g), BuildAtomsCheck(g)},
	}
	for _, p := range pairs {
		if p.a != p.b {
			t.Fatalf("generator is non-deterministic: two consecutive builds of %s produced different bytes", p.name)
		}
	}
}

func TestGenerator_GraphJSONDoubleRegenerateIsIdentical(t *testing.T) {
	t.Parallel()
	g := loadDomainGraph(t)
	a, err := BuildGraphJSON(g)
	if err != nil {
		t.Fatalf("first BuildGraphJSON failed: %v", err)
	}
	b, err := BuildGraphJSON(g)
	if err != nil {
		t.Fatalf("second BuildGraphJSON failed: %v", err)
	}
	if a != b {
		t.Fatal("BuildGraphJSON is non-deterministic: two consecutive builds produced different bytes")
	}
}
