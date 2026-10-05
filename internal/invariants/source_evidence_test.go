package invariants

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func TestSourceInvariantsActivateOnlyForTheirExplicitFields(t *testing.T) {
	oldMode := &ontology.Graph{Discipline: "full"}
	if got := checkSpecificationSourcesCurrent(oldMode); len(got) != 0 {
		t.Fatalf("empty source declaration activated source-current check: %#v", got)
	}
	if got := checkSourceLinksResolve(oldMode); len(got) != 0 {
		t.Fatalf("empty source_links activated link check: %#v", got)
	}
	if got := checkCoverageDeclarationsJustified(oldMode); len(got) != 0 {
		t.Fatalf("absent coverage declaration activated qualification check: %#v", got)
	}

	dir := t.TempDir()
	bytes := []byte("# Contract\n")
	path := filepath.Join(dir, "spec.md")
	if err := os.WriteFile(path, bytes, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(bytes)
	g := &ontology.Graph{
		DomainDir: dir,
		SpecificationSources: []ontology.SpecificationSource{{
			ID: "spec", Path: "spec.md", Version: "1", SHA256: hex.EncodeToString(sum[:]),
		}},
	}
	if got := checkSpecificationSourcesCurrent(g); len(got) != 0 {
		t.Fatalf("current declared source produced violations: %#v", got)
	}
	if got := checkSourceLinksResolve(g); len(got) != 0 {
		t.Fatalf("source metadata alone activated requirement-link check: %#v", got)
	}
	if got := checkCoverageDeclarationsJustified(g); len(got) != 0 {
		t.Fatalf("source metadata alone activated coverage check: %#v", got)
	}

	g.Requirements = []ontology.Requirement{{
		ID:          "R-source-link",
		SourceLinks: []ontology.SourceLink{{SourceID: "spec", Anchor: "#contract"}},
	}}
	if got := checkSourceLinksResolve(g); len(got) != 0 {
		t.Fatalf("valid explicit requirement link produced violations: %#v", got)
	}
	g.Requirements[0].SourceLinks = []ontology.SourceLink{{SourceID: "unknown", Anchor: "L1"}}
	if got := checkSourceLinksResolve(g); len(got) != 1 || got[0].ID != "R-source-link" {
		t.Fatalf("invalid explicit requirement link was not attributed to its requirement: %#v", got)
	}

	g.Requirements[0].SourceLinks = nil
	g.Requirements[0].Coverage = &ontology.CoverageDeclaration{
		Status: ontology.CoverageVerified, Rationale: "authored pass label",
	}
	if got := checkCoverageDeclarationsJustified(g); len(got) != 1 || got[0].ID != "R-source-link" {
		t.Fatalf("invalid explicit coverage declaration was not reported: %#v", got)
	}
}

func TestSpecificationSourceInvariantDetectsContentDrift(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.md")
	if err := os.WriteFile(path, []byte("# Updated\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	g := &ontology.Graph{
		DomainDir: dir,
		SpecificationSources: []ontology.SpecificationSource{{
			ID: "spec", Path: "spec.md", Version: "v1", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		}},
	}
	got := checkSpecificationSourcesCurrent(g)
	if len(got) != 1 || got[0].ID != "spec" {
		t.Fatalf("source drift did not activate its explicit invariant: %#v", got)
	}
}
