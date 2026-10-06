package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/invariants"

	"github.com/PHPCraftdream/HotamSpec/internal/paths"
)

func TestCrystalFixpointStopsWhenReadersSettle(t *testing.T) {
	reader := invariants.CrystalReaderCheckNames[0]
	other := invariants.Violation{Check: "check_other", ID: "x"}
	settled := []invariants.Violation{{Check: reader, ID: "a", Message: "stable"}}
	renders := 0
	written, err := runCrystalFixpoint([]invariants.Violation{other}, func() []invariants.Violation { return settled }, func(patched []invariants.Violation) ([]string, error) {
		renders++
		if len(patched) != 2 || patched[0] != other {
			t.Fatalf("non-reader violations must survive the patch, got %v", patched)
		}
		return []string{"CLAUDE.md"}, nil
	})
	if err != nil || renders != 1 || len(written) != 1 {
		t.Fatalf("want one render and success, got renders=%d written=%v err=%v", renders, written, err)
	}
}

func TestCrystalFixpointFailsWhenReadersNeverSettle(t *testing.T) {
	reader := invariants.CrystalReaderCheckNames[0]
	calls := 0
	_, err := runCrystalFixpoint(nil, func() []invariants.Violation {
		calls++
		return []invariants.Violation{{Check: reader, ID: "a", Message: strconv.Itoa(calls)}}
	}, func([]invariants.Violation) ([]string, error) { return nil, nil })
	if err == nil {
		t.Fatal("a fixpoint that never settles must fail, not succeed silently")
	}
	if !strings.Contains(err.Error(), reader) || strings.Contains(err.Error(), invariants.CrystalReaderCheckNames[1]) {
		t.Fatalf("diagnostic must name exactly the moving check, got %v", err)
	}
}

func TestCrystalFixpointFailsWhenLastRenderStillChanges(t *testing.T) {
	reader := invariants.CrystalReaderCheckNames[0]
	calls := 0
	// Violations disappear only after the final allowed render: the post-write
	// state differs from the last snapshot, so the bound is exhausted unconverged.
	_, err := runCrystalFixpoint(nil, func() []invariants.Violation {
		calls++
		if calls > maxCrystalFixpointRenders {
			return nil
		}
		return []invariants.Violation{{Check: reader, ID: "a", Message: strconv.Itoa(calls)}}
	}, func([]invariants.Violation) ([]string, error) { return nil, nil })
	if err == nil {
		t.Fatal("a change after the last allowed render must be reported")
	}
}

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
