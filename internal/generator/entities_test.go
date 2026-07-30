package generator

import (
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// minimalEntityType builds a well-formed single-state EntityType fixture for
// BuildEntities rendering tests -- deliberately minimal (one initial state,
// no transitions) since these tests exercise the ModelSymbol line only, not
// lifecycle rendering (covered elsewhere).
func minimalEntityType(slug string) ontology.EntityType {
	return ontology.EntityType{
		Slug:        slug,
		Description: "a test entity",
		Lifecycle: ontology.Lifecycle{
			Slug:   slug + "-lifecycle",
			States: []ontology.State{{Name: "ACTIVE", Kind: ontology.StateKindInitial}},
		},
	}
}

// TestBuildEntities_RendersModelSymbolWhenSet proves BuildEntities renders a
// "**Model:** `<model_symbol>`" line near the EntityType's own heading when
// ModelSymbol is set (task #395, W1.3, Deliverable A item 4).
func TestBuildEntities_RendersModelSymbolWhenSet(t *testing.T) {
	t.Parallel()
	et := minimalEntityType("release")
	et.ModelSymbol = "spec/model/release.go:Release"
	g := &ontology.Graph{EntityTypes: []ontology.EntityType{et}}

	got := BuildEntities(g, "fixture-domain")
	want := "**Model:** `spec/model/release.go:Release`"
	if !strings.Contains(got, want) {
		t.Fatalf("BuildEntities output missing %q; got:\n%s", want, got)
	}
}

// TestBuildEntities_OmitsModelLineWhenUnset proves BuildEntities renders no
// "**Model:**" line at all when ModelSymbol is empty -- the overwhelming
// majority case today, since ModelSymbol is a purely additive, optional
// field with zero live EntityTypes setting it yet.
func TestBuildEntities_OmitsModelLineWhenUnset(t *testing.T) {
	t.Parallel()
	et := minimalEntityType("release")
	g := &ontology.Graph{EntityTypes: []ontology.EntityType{et}}

	got := BuildEntities(g, "fixture-domain")
	if strings.Contains(got, "**Model:**") {
		t.Fatalf("BuildEntities output unexpectedly contains a **Model:** line for an EntityType with no ModelSymbol; got:\n%s", got)
	}
}
