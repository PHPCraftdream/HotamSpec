package evidence

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func TestUnexecutedCaseFindingTracksProfileWithoutInventingMeasurement(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the evidence collection pipeline over full fixture domains; skipped in -short")
	}

	root := t.TempDir()
	model := filepath.Join(root, "spec", "model")
	if err := os.MkdirAll(model, 0o755); err != nil {
		t.Fatal(err)
	}
	for path, contents := range map[string]string{
		"spec/go.mod":             "module example.org/profile\n\ngo 1.22\n",
		"spec/model/rule.go":      "package model\nfunc Result() int { return 1 }\n",
		"spec/model/rule_test.go": "package model\nimport \"testing\"\nfunc TestCarrier(t *testing.T) { if Result() != 1 { t.Error(\"wrong result\") } }\n",
	} {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(path)), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	g := &ontology.Graph{DomainDir: root, Requirements: []ontology.Requirement{{
		ID: "R-rule", Claim: "Retain the result.", AtomKind: "rule",
		Cases: []ontology.CaseDefinition{{ID: "missing", Test: "spec/model/rule_test.go:TestCarrier/missing", AtomIDs: []string{"R-rule"}, Profile: "core", Operation: "decode"}},
	}}, Conformance: &ontology.ConformanceConfig{RuleCases: true, Profiles: []ontology.Profile{{ID: "core", Operations: []string{"decode"}}}}}
	snapshot, err := gate.CollectAtomExecutionSnapshot(g)
	if err != nil {
		t.Fatal(err)
	}
	collect := func() Finding {
		t.Helper()
		report, err := CollectFromSnapshot(g, snapshot)
		if err != nil {
			t.Fatal(err)
		}
		for _, finding := range report.Findings {
			if finding.CaseID == "missing" && finding.Kind == FindingExecutionUnavailable && finding.Subject == "test execution" {
				return finding
			}
		}
		t.Fatalf("missing test execution finding: %+v", report.Findings)
		return Finding{}
	}
	before := collect()
	g.Conformance.Profiles[0].Capabilities = map[string]string{"decode": "changed"}
	after := collect()
	if before.ID == after.ID || after.Context.ProfileDetails == nil || after.Context.ProfileDetails.Capabilities["decode"] != "changed" {
		t.Fatalf("profile change inherited the prior case review identity: before=%+v after=%+v", before, after)
	}
	if len(after.Context.MeasuredComponents) != 0 || after.Context.RecordedCaseFingerprint != "" {
		t.Fatalf("missing execution acquired fabricated measured context: %+v", after.Context)
	}
}
