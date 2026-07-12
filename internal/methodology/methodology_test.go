package methodology

import "testing"

func TestSectionsComplete(t *testing.T) {
	t.Parallel()
	sections := Sections.All()
	const want = 29
	if len(sections) != want {
		t.Fatalf("expected %d sections, got %d", want, len(sections))
	}
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

func TestToolsComplete(t *testing.T) {
	t.Parallel()
	tools := Tools.All()
	const want = 34
	if len(tools) != want {
		t.Fatalf("expected %d tools, got %d", want, len(tools))
	}
	for _, tl := range tools {
		if tl.Command == "" {
			t.Errorf("tool has empty Command: %+v", tl)
		}
		if tl.Canon == "" {
			t.Errorf("tool %q has empty Canon", tl.Command)
		}
		if tl.Purpose == "" {
			t.Errorf("tool %q has empty Purpose", tl.Command)
		}
	}
}
