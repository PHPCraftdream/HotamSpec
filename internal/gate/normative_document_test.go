package gate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func normativeDocumentFixture(t *testing.T) (*ontology.Graph, *AtomSourceIndex) {
	t.Helper()
	root := writeAtomPipelineFixture(t, map[string]string{
		"manifest.json": `{"self_hosting":false}`,
		"spec/model/text.go": `package model

type Text string

// Source: reference.md:10-12

// >>>>> lang=en
// An escaped delimiter remains content.
// >>>>> lang=ru
// Экранированный разделитель остаётся содержимым.
const EscapedDelimiter Text = "escape-delimiter"
`,
	})
	graph := &ontology.Graph{DomainDir: root, Languages: []string{"en", "ru"}, DefaultLanguage: "en"}
	graph.Conformance = &ontology.ConformanceConfig{
		Clauses: []ontology.SourceClause{{ID: "escape.delimiter"}},
		DocumentSections: []ontology.DocumentSection{{
			ID: "language.escapes", Order: 1,
			TitleTexts: ontology.LocalizedText{"en": "Escapes", "ru": "Escape"},
			Blocks: []ontology.NormativeBlock{{
				ID: "escape.content", TextRef: "spec/model/text.go:EscapedDelimiter", Role: "normative",
				ClauseIDs: []string{"escape.delimiter"}, Examples: []ontology.DocumentExample{{CaseID: "escaped-comma", ComparisonNames: []string{"comma-value"}}},
			}},
		}},
	}
	graph.Requirements = []ontology.Requirement{{
		ID: "R-escape", ClauseLinks: []ontology.ClauseLink{{ClauseID: "escape.delimiter"}},
		Cases: []ontology.CaseDefinition{{ID: "escaped-comma", Test: "spec/model/rule_test.go:TestComma", ClauseIDs: []string{"escape.delimiter"}}},
	}}
	index, err := NewAtomSourceIndexForGraph(graph)
	if err != nil {
		t.Fatal(err)
	}
	return graph, index
}

func TestNormativeIdentitySurvivesProvenanceAndLineEdits(t *testing.T) {
	graph, before := normativeDocumentFixture(t)
	path := filepath.Join(graph.DomainDir, "spec", "model", "text.go")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(string(original), "reference.md:10-12", "reference.md:400-402", 1)
	changed = strings.Replace(changed, "type Text string", "\n\n\ntype Text string", 1)
	if err := os.WriteFile(path, []byte(changed), 0600); err != nil {
		t.Fatal(err)
	}
	after, err := NewAtomSourceIndexForGraph(graph)
	if err != nil {
		t.Fatal(err)
	}
	for _, snapshot := range []*AtomSourceIndex{before, after} {
		if err := ValidateNormativeDocument(graph, snapshot); err != nil {
			t.Fatalf("provenance edit invalidated durable document identity: %v", err)
		}
		texts, err := snapshot.ResolveNormativeText("spec/model/text.go:EscapedDelimiter")
		if err != nil {
			t.Fatal(err)
		}
		texts["en"] = "caller mutation"
		again, err := snapshot.ResolveNormativeText("spec/model/text.go:EscapedDelimiter")
		if err != nil || again["en"] == "caller mutation" {
			t.Fatalf("caller modified snapshot-owned translations: %v %v", again, err)
		}
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := before.ResolveNormativeText("spec/model/text.go:EscapedDelimiter"); err != nil {
		t.Fatalf("publication reread disk instead of invocation snapshot: %v", err)
	}
	if _, err := before.ResolveNormativeText("spec/model/text.go:Missing"); err == nil {
		t.Fatal("stale semantic text reference was accepted")
	}
}

func TestNormativeDocumentRejectsBrokenBindingsAndStructure(t *testing.T) {
	mutations := []struct {
		name   string
		mutate func(*ontology.Graph)
		want   string
	}{
		{"unknown parent", func(g *ontology.Graph) { g.Conformance.DocumentSections[0].ParentID = "missing" }, "unknown parent"},
		{"cycle", func(g *ontology.Graph) { g.Conformance.DocumentSections[0].ParentID = "language.escapes" }, "cycle"},
		{"duplicate section", func(g *ontology.Graph) {
			g.Conformance.DocumentSections = append(g.Conformance.DocumentSections, g.Conformance.DocumentSections[0])
		}, "duplicate section"},
		{"duplicate block", func(g *ontology.Graph) {
			s := &g.Conformance.DocumentSections[0]
			s.Blocks = append(s.Blocks, s.Blocks[0])
		}, "duplicate block"},
		{"unknown clause", func(g *ontology.Graph) { g.Conformance.DocumentSections[0].Blocks[0].ClauseIDs = []string{"missing"} }, "unknown or duplicate clause"},
		{"duplicate clause", func(g *ontology.Graph) {
			g.Conformance.Clauses = append(g.Conformance.Clauses, g.Conformance.Clauses[0])
		}, "duplicate clause"},
		{"stale text", func(g *ontology.Graph) {
			g.Conformance.DocumentSections[0].Blocks[0].TextRef = "spec/model/text.go:Missing"
		}, "Missing"},
		{"missing translation", func(g *ontology.Graph) { delete(g.Conformance.DocumentSections[0].TitleTexts, "ru") }, "translations"},
		{"missing qualification translation", func(g *ontology.Graph) {
			g.Conformance.DocumentSections[0].Blocks[0].QualificationTexts = ontology.LocalizedText{"en": "This profile only"}
		}, "translations"},
		{"stale example", func(g *ontology.Graph) {
			g.Conformance.DocumentSections[0].Blocks[0].Examples = []ontology.DocumentExample{{CaseID: "missing", ComparisonNames: []string{"comma-value"}}}
		}, "stale example"},
		{"duplicate example", func(g *ontology.Graph) {
			g.Conformance.DocumentSections[0].Blocks[0].Examples = []ontology.DocumentExample{{CaseID: "escaped-comma", ComparisonNames: []string{"comma-value"}}, {CaseID: "escaped-comma", ComparisonNames: []string{"different-value"}}}
		}, "duplicated"},
		{"empty case selector", func(g *ontology.Graph) { g.Conformance.DocumentSections[0].Blocks[0].Examples[0].CaseID = "" }, "malformed example case ID"},
		{"missing comparison selectors", func(g *ontology.Graph) { g.Conformance.DocumentSections[0].Blocks[0].Examples[0].ComparisonNames = nil }, "explicit comparison names"},
		{"duplicate comparison selector", func(g *ontology.Graph) {
			g.Conformance.DocumentSections[0].Blocks[0].Examples[0].ComparisonNames = []string{"comma-value", "comma-value"}
		}, "duplicate comparison name"},
		{"empty comparison selector", func(g *ontology.Graph) {
			g.Conformance.DocumentSections[0].Blocks[0].Examples[0].ComparisonNames = []string{""}
		}, "empty, malformed or duplicate comparison name"},
		{"inapplicable clause profile", func(g *ontology.Graph) {
			g.Conformance.Clauses[0].Applicability = &ontology.Applicability{Profiles: []string{"different"}}
		}, "inapplicable to scoped clause"},
		{"inapplicable property feature", func(g *ontology.Graph) {
			g.Requirements[0].Applicability = &ontology.Applicability{Features: []string{"unavailable"}}
		}, "inapplicable to property"},
		{"inapplicable operation", func(g *ontology.Graph) {
			g.Requirements[0].Cases[0].Operation = "parse"
			g.Conformance.Clauses[0].Applicability = &ontology.Applicability{Operations: []string{"write"}}
		}, "inapplicable to scoped clause"},
		{"unreachable selected case", func(g *ontology.Graph) {
			g.Requirements[0].Coverage = &ontology.CoverageDeclaration{Status: ontology.CoverageUnreachable}
		}, "qualified"},
		{"unverified selected case", func(g *ontology.Graph) {
			g.Requirements[0].Coverage = &ontology.CoverageDeclaration{Status: ontology.CoverageUnverified}
		}, "qualified"},
		{"unsupported selected case", func(g *ontology.Graph) {
			g.Requirements[0].Coverage = &ontology.CoverageDeclaration{Status: ontology.CoverageUnsupported}
		}, "qualified"},
		{"inferred case proof", func(g *ontology.Graph) { g.Requirements[0].Cases[0].ClauseIDs = nil }, "no explicit clause binding"},
		{"unlinked case proof", func(g *ontology.Graph) { g.Requirements[0].ClauseLinks = nil }, "not linked"},
		{"ambiguous case", func(g *ontology.Graph) {
			r := g.Requirements[0]
			r.ID = "R-other"
			r.Cases = []ontology.CaseDefinition{{ID: "escaped-comma", Test: "different"}}
			g.Requirements = append(g.Requirements, r)
		}, "ambiguous declarations"},
		{"unknown role", func(g *ontology.Graph) { g.Conformance.DocumentSections[0].Blocks[0].Role = "passed" }, "unknown role"},
		{"unowned source clause", func(g *ontology.Graph) {
			g.Conformance.Clauses = append(g.Conformance.Clauses, ontology.SourceClause{ID: "escape.unowned"})
		}, "no authored document block"},
	}
	for _, test := range mutations {
		t.Run(test.name, func(t *testing.T) {
			graph, index := normativeDocumentFixture(t)
			if err := ValidateNormativeDocument(graph, index); err != nil {
				t.Fatalf("valid authored document rejected before mutation: %v", err)
			}
			test.mutate(graph)
			err := ValidateNormativeDocument(graph, index)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("broken document accepted or wrong diagnostic: %v; want %q", err, test.want)
			}
		})
	}
}

func TestNormativeDocumentKeepsEvidenceOnlyAndInformativeClaimsDistinct(t *testing.T) {
	graph, index := normativeDocumentFixture(t)
	graph.Requirements[0].Cases = append(graph.Requirements[0].Cases, ontology.CaseDefinition{ID: "extra-observation", Test: "spec/model/rule_test.go:TestExtra"})
	if err := ValidateNormativeDocument(graph, index); err != nil {
		t.Fatalf("unselected evidence-only observation acquired inferred proof obligation: %v", err)
	}
	graph.Conformance.Clauses = nil
	graph.Requirements = nil
	block := &graph.Conformance.DocumentSections[0].Blocks[0]
	block.Role, block.ClauseIDs, block.Examples = "informative", nil, nil
	if err := ValidateNormativeDocument(graph, index); err != nil {
		t.Fatalf("an entirely informative document required fake executable clauses: %v", err)
	}
}

func TestDirectNormativeTextResolutionRejectsActiveNestedIncludes(t *testing.T) {
	root := writeAtomPipelineFixture(t, map[string]string{
		"manifest.json": `{"self_hosting":false}`,
		"spec/model/text.go": `package model
type Text string
// >>>>> lang=en
// A document must not recursively load another source.
//
// include: spec/model/text.go:Other
// >>>>> lang=ru
// Документ не должен рекурсивно загружать другой источник.
//
// include: spec/model/text.go:Other
const Rule Text = "rule"
`,
	})
	graph := &ontology.Graph{DomainDir: root, Languages: []string{"en", "ru"}, DefaultLanguage: "en"}
	index, err := NewAtomSourceIndexForGraph(graph)
	if err != nil {
		t.Fatal(err)
	}
	_, err = index.ResolveNormativeText("spec/model/text.go:Rule")
	if err == nil || !strings.Contains(err.Error(), "Rule") || !strings.Contains(err.Error(), "text.go:") || !strings.Contains(err.Error(), "en") {
		t.Fatalf("active nested document include was accepted without a source/language diagnostic: %v", err)
	}
}

func TestNormativeSourceRejectsMissingTranslationBeforePublication(t *testing.T) {
	root := writeAtomPipelineFixture(t, map[string]string{
		"manifest.json": `{"self_hosting":false}`,
		"spec/model/text.go": `package model
type Text string
// >>>>> lang=en
// An escaped delimiter remains content.
const EscapedDelimiter Text = "escape-delimiter"
`,
	})
	graph := &ontology.Graph{DomainDir: root, Languages: []string{"en", "ru"}, DefaultLanguage: "en"}
	index, err := NewAtomSourceIndexForGraph(graph)
	if index != nil || err == nil || !strings.Contains(err.Error(), "text.go:") || !strings.Contains(err.Error(), "EscapedDelimiter") || !strings.Contains(err.Error(), "ru") {
		t.Fatalf("incomplete multilingual source yielded a publishable snapshot or lost source diagnostic: %v", err)
	}
}

func TestNormativeExampleUsesDeclaredApplicabilityAndProfileSpecificQualification(t *testing.T) {
	graph, index := normativeDocumentFixture(t)
	graph.Requirements[0].Cases[0].Profile = "core"
	graph.Conformance.Profiles = []ontology.Profile{{ID: "core", Operations: []string{"parse"}, Features: []string{"unicode-escapes"}}}
	graph.Conformance.Clauses[0].Applicability = &ontology.Applicability{Profiles: []string{"core"}, Operations: []string{"parse"}, Features: []string{"unicode-escapes"}}
	graph.Requirements[0].Coverage = &ontology.CoverageDeclaration{Status: ontology.CoverageUnverified, Profile: "extended"}
	if err := ValidateNormativeDocument(graph, index); err != nil {
		t.Fatalf("applicable selected case inherited another profile's qualification or lost declared operation fallback: %v", err)
	}
	graph.Requirements[0].Coverage.Profile = "core"
	if err := ValidateNormativeDocument(graph, index); err == nil || !strings.Contains(err.Error(), "qualified") {
		t.Fatalf("selected case in its qualified profile remained publishable: %v", err)
	}
	graph.Requirements[0].Coverage = nil
	graph.Conformance.Profiles[0].Features = nil
	if err := ValidateNormativeDocument(graph, index); err == nil || !strings.Contains(err.Error(), "inapplicable") {
		t.Fatalf("selected case lacking a required feature remained publishable: %v", err)
	}
}
