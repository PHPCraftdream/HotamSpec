package generator

import (
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// readerHeaderFixtureGraph builds a minimal non-empty graph; withDomainUser
// controls whether the "domain-user" stakeholder (the domain-resolver role
// binding used by REQUIREMENTS/SPEC/etc.) exists.
func readerHeaderFixtureGraph(withDomainUser bool) *ontology.Graph {
	g := &ontology.Graph{
		Stakeholders: []ontology.Stakeholder{{ID: "ai-agent", Name: "Agent", Domain: "d"}},
		Requirements: []ontology.Requirement{{
			ID: "R-1", Claim: "c", Owner: "ai-agent", Status: ontology.StatusSETTLED,
			Enforcement: ontology.EnforcementPROSE,
		}},
	}
	if withDomainUser {
		g.Stakeholders = append(g.Stakeholders, ontology.Stakeholder{ID: "domain-user", Name: "Domain user", Domain: "d"})
	}
	return g
}

// Without a "domain-user" stakeholder, every domain-resolver document must
// omit the reader line entirely (previously it printed
// "reader: (unresolved-reader)"), while operator-bound documents keep theirs.
func TestDocHeaderOmitsUnresolvedReader(t *testing.T) {
	t.Parallel()
	g := readerHeaderFixtureGraph(false)
	for _, docKind := range []string{"REQUIREMENTS", "SPEC", "TENSIONS", "DECISIONS", "HISTORY", "ENTITIES", "MODELS", "COVERAGE"} {
		if reader := ReaderHeaderLine(docKind, g); reader != "" {
			t.Errorf("ReaderHeaderLine(%s) = %q without domain-user; want empty", docKind, reader)
		}
	}
	for _, docKind := range []string{"REQUIREMENTS", "TENSIONS", "COVERAGE"} {
		joined := strings.Join(docHeaderLines(docKind, g), "\n")
		if strings.Contains(joined, "reader:") {
			t.Errorf("docHeaderLines(%s) prints a reader line without a resolvable stakeholder:\n%s", docKind, joined)
		}
	}
	// Self-hosting-style documents bound to the operator keep their line.
	if reader := ReaderHeaderLine("CONSTITUTION", g); reader != "reader: ai-agent" {
		t.Errorf("ReaderHeaderLine(CONSTITUTION) = %q; want the resolved operator reader", reader)
	}
}

func TestDocHeaderKeepsResolvedReader(t *testing.T) {
	t.Parallel()
	g := readerHeaderFixtureGraph(true)
	for _, docKind := range []string{"REQUIREMENTS", "TENSIONS", "COVERAGE"} {
		lines := docHeaderLines(docKind, g)
		if len(lines) != 3 || lines[1] != "reader: domain-user" {
			t.Errorf("docHeaderLines(%s) = %q; want banner, reader: domain-user, blank", docKind, strings.Join(lines, "\n"))
		}
	}
}
