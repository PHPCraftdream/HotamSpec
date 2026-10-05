package hotamontology

import (
	"encoding/json"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func TestRequirementSourceMetadataProjectsThroughRegistryDumpJSON(t *testing.T) {
	wantLink := SourceLink{SourceID: "spec", Anchor: "#boundary"}
	wantCoverage := CoverageDeclaration{
		Status:    ontology.CoverageUnreachable,
		Rationale: "the external profile does not expose this operation",
		Profile:   "consumer-profile",
	}
	data, err := json.Marshal([]Requirement{{
		ID:          "R-source-projection",
		SourceLinks: []SourceLink{wantLink},
		Coverage:    &wantCoverage,
	}})
	if err != nil {
		t.Fatal(err)
	}
	var projected []ontology.Requirement
	if err := json.Unmarshal(data, &projected); err != nil {
		t.Fatal(err)
	}
	if len(projected) != 1 {
		t.Fatalf("registry projection produced %d requirements", len(projected))
	}
	if len(projected[0].SourceLinks) != 1 || projected[0].SourceLinks[0] != (ontology.SourceLink{SourceID: wantLink.SourceID, Anchor: wantLink.Anchor}) {
		t.Errorf("projected source links = %#v", projected[0].SourceLinks)
	}
	if projected[0].Coverage == nil || projected[0].Coverage.Status != wantCoverage.Status || projected[0].Coverage.Rationale != wantCoverage.Rationale || projected[0].Coverage.Profile != wantCoverage.Profile {
		t.Errorf("projected coverage declaration = %#v", projected[0].Coverage)
	}
}
