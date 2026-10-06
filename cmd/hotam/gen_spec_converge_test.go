package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/paths"
)

func TestGenSpec_CrystalReadersConvergeInOnePass(t *testing.T) {
	projectRoot, domainDir := initDomainUnderRoot(t, "crystal-converge", "2026-10-05")
	manifestPath := filepath.Join(domainDir, "manifest.json")
	manifest := `{"self_hosting":false,"gen_profile":"consumer","parent":null,"orientation_faq":[{"question":"project identity","keywords":["crystal-converge"]}]}` + "\n"
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	crystalPath := filepath.Join(projectRoot, "CLAUDE.md")
	if _, err := os.Stat(crystalPath); !os.IsNotExist(err) {
		t.Fatalf("precondition: crystal must not exist, stat err=%v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, paths.MarkerFilename), nil, 0o644); err != nil {
		t.Fatalf("write project marker: %v", err)
	}
	if _, _, err := genSpec(domainDir, crystalPath, "2026-10-05", "consumer", false); err != nil {
		t.Fatalf("first genSpec: %v", err)
	}
	violations, err := allViolations(domainDir)
	if err != nil {
		t.Fatalf("allViolations: %v", err)
	}
	for _, v := range violations {
		t.Errorf("violation survived one genSpec pass: [%s] %s: %s", v.Check, v.ID, v.Message)
	}
	before, err := os.ReadFile(crystalPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := genSpec(domainDir, crystalPath, "2026-10-05", "consumer", false); err != nil {
		t.Fatalf("second genSpec: %v", err)
	}
	after, err := os.ReadFile(crystalPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("crystal changed between identical genSpec runs")
	}
}
