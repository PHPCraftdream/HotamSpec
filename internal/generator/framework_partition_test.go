package generator

import (
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func TestFrameworkPartitionPreservesSettledObligationsAndExcludesDrafts(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{Requirements: []ontology.Requirement{
		{ID: "R-entity-instance-refs-resolve", Claim: "References resolve.", Status: ontology.StatusSETTLED, Enforcement: ontology.EnforcementENFORCED},
		{ID: "R-shop-delivery", Claim: "The order reaches its destination.", Status: ontology.StatusSETTLED, Enforcement: ontology.EnforcementSTRUCTURAL},
		{ID: "R-entity-field-kind-known", Claim: "Draft plumbing.", Status: ontology.StatusDRAFT},
		{ID: "R-shop-draft", Claim: "Draft business rule.", Status: ontology.StatusDRAFT},
	}}
	resident := strings.Join(renderAgentContextConstitutionIndex(g, false), "\n")
	framework := BuildFrameworkInvariants(g, "fixture")
	if strings.Contains(resident, "R-entity-instance-refs-resolve") || !strings.Contains(framework, "R-entity-instance-refs-resolve") {
		t.Fatalf("settled plumbing was duplicated or lost: resident=%s framework=%s", resident, framework)
	}
	if !strings.Contains(resident, "R-shop-delivery") || strings.Contains(framework, "R-shop-delivery") {
		t.Fatalf("settled business obligation was misplaced: resident=%s framework=%s", resident, framework)
	}
	for _, id := range []string{"R-entity-field-kind-known", "R-shop-draft"} {
		if strings.Contains(resident, id) || strings.Contains(framework, id) {
			t.Fatalf("draft %s was published as settled", id)
		}
	}
}
