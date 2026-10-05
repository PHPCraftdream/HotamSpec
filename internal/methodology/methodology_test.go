package methodology

import (
	"testing"
)

func TestSectionsHaveValidMetadata(t *testing.T) {
	t.Parallel()
	sections := Sections.All()
	for _, s := range sections {
		if s.Slug == "" {
			t.Errorf("section has empty Slug: %+v", s)
		}
		if s.Canon == "" {
			t.Errorf("section %q has empty Canon", s.Slug)
		}
		if s.Narrative == "" {
			t.Errorf("section %q has empty Narrative", s.Slug)
		}
		if s.Why == "" {
			t.Errorf("section %q has empty Why", s.Slug)
		}
		switch s.Kind {
		case ONTOLOGY, DISCIPLINE, PROCESS, PLUMBING:
		default:
			t.Errorf("section %q has unknown Kind %q", s.Slug, s.Kind)
		}
	}
}

func TestToolsHaveValidMetadata(t *testing.T) {
	t.Parallel()
	for _, tool := range Tools.All() {
		if tool.Command == "" {
			t.Errorf("tool has empty Command: %+v", tool)
		}
		if tool.Canon == "" {
			t.Errorf("tool %q has empty Canon", tool.Command)
		}
		if tool.Purpose == "" {
			t.Errorf("tool %q has empty Purpose", tool.Command)
		}
		switch tool.Status {
		case Implemented, Planned:
		default:
			t.Errorf("tool %q has unknown Status %q", tool.Command, tool.Status)
		}
	}
}

// TestSectionsNoDuplicateSlugs guards the single-source-of-truth property
// this package exists to hold (TaskList P2-3): every Section must register
// under a unique Slug. registry.MustRegister already panics on a duplicate
// name at init time, so in practice this test can only fail if someone
// bypasses MustRegister — it exists as an explicit, readable "no dupes"
// contract rather than relying solely on the panic side-effect.
func TestSectionsNoDuplicateSlugs(t *testing.T) {
	t.Parallel()
	seen := make(map[string]bool)
	for _, s := range Sections.All() {
		if seen[s.Slug] {
			t.Errorf("duplicate Section slug %q", s.Slug)
		}
		seen[s.Slug] = true
	}
}

func TestSectionsLookup(t *testing.T) {
	t.Parallel()
	c, ok := Sections.Get("§Conflict")
	if !ok {
		t.Fatal("expected §Conflict to be registered")
	}
	if c.Kind != ONTOLOGY {
		t.Errorf("expected §Conflict kind ONTOLOGY, got %q", c.Kind)
	}
}

func TestNamedSectionVariables(t *testing.T) {
	t.Parallel()
	if Conflict.Slug != "§Conflict" {
		t.Fatalf("Conflict.Slug = %q, want %q", Conflict.Slug, "§Conflict")
	}
	if Ticket.Slug != "§Ticket" {
		t.Fatalf("Ticket.Slug = %q, want %q", Ticket.Slug, "§Ticket")
	}
	if ContextBudget.Slug != "§ContextBudget" {
		t.Fatalf("ContextBudget.Slug = %q, want %q", ContextBudget.Slug, "§ContextBudget")
	}
	if Conflict.Kind != ONTOLOGY || Ticket.Kind != PLUMBING || ContextBudget.Kind != PLUMBING {
		t.Fatalf("unexpected Kind: Conflict=%q Ticket=%q ContextBudget=%q", Conflict.Kind, Ticket.Kind, ContextBudget.Kind)
	}
}


// TestStatusToolRegisteredImplemented enforces R-status-single-command-summary's
// existence half: the `status` tool must be registered in the methodology
// registry as Implemented, with a non-empty Purpose describing the composed
// what-now/due/all-violations summary, so the registry entry (read by
// cmd/hotam/tool_wiring.go and by the generated EMBEDDED-TOOLS crystal block)
// can never silently regress to Planned or disappear. The behavioral half
// (the aggregation actually matching what-now/due/all-violations) is
// enforced separately by cmd/hotam's
// TestBuildStatusReport_MatchesWhatNowDueAllViolations, which cannot live
// under internal/ (it needs cmd/hotam's unexported helpers) and so cannot be
// named directly in enforced_by (see internal/gate's Test*-name resolver,
// scoped to internal/**/*_test.go).
func TestStatusToolRegisteredImplemented(t *testing.T) {
	t.Parallel()
	tool, ok := Tools.Get("status")
	if !ok {
		t.Fatal("status tool not registered in methodology.Tools")
	}
	if tool.Status != Implemented {
		t.Errorf("status tool Status = %q, want Implemented", tool.Status)
	}
	if tool.Command != "status" {
		t.Errorf("status tool Command = %q, want %q", tool.Command, "status")
	}
	if tool.Purpose == "" {
		t.Error("status tool has empty Purpose")
	}
}

