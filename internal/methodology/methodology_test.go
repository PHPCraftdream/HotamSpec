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

func TestToolsComplete(t *testing.T) {
	t.Parallel()
	tools := Tools.All()
	// 9 Ported (gen_spec, what_now, apply_proposal, gate, all_violations,
	// req, due, inspect, land — every real `hotam` CLI subcommand as of
	// P1-6 / TaskList #19) + 28 Declared (Python-era methodology surface
	// not yet ported).
	const want = 37
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
		switch tl.Status {
		case Ported, Declared:
		default:
			t.Errorf("tool %q has unknown Status %q", tl.Command, tl.Status)
		}
	}
}

// TestToolsPortedCount pins the exact count of Ported tools (as opposed to
// TestToolsComplete's total, which also counts Declared) so a future edit
// that accidentally demotes/promotes a tool's Status is caught even though
// the total count wouldn't change.
func TestToolsPortedCount(t *testing.T) {
	t.Parallel()
	ported := 0
	for _, tl := range Tools.All() {
		if tl.Status == Ported {
			ported++
		}
	}
	const wantPorted = 9
	if ported != wantPorted {
		t.Fatalf("expected %d Ported tools, got %d", wantPorted, ported)
	}
}
