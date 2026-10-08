package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/gate"
)

func populateCLICompileCache(t *testing.T) []string {
	t.Helper()

	temp := t.TempDir()
	t.Setenv("TMP", temp)
	t.Setenv("TEMP", temp)
	t.Setenv("TMPDIR", temp)
	root := t.TempDir()
	files := map[string]string{
		"go.mod":        "module cleanup-fixture\n\ngo 1.22\n",
		"value.go":      "package fixture\nfunc Accept(value int) bool { return value > 0 }\n",
		"value_test.go": "package fixture\nimport \"testing\"\nfunc TestPositive(t *testing.T) { if !Accept(1) || Accept(0) { t.Fatal(\"positive boundary violated\") } }\n",
	}
	for name, source := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	session := gate.NewExecutionSession() // runCLI is the process owner under test
	result := session.RunVerifiedByTest(root, "value_test.go", "TestPositive")
	if result.Err != nil || !result.Passed {
		t.Fatalf("real compile failed: %+v", result)
	}
	before, err := filepath.Glob(filepath.Join(temp, "hotam-compile-*"))
	if err != nil || len(before) == 0 {
		t.Fatalf("regression did not create compile artifacts: %v %v", before, err)
	}
	return before
}

func TestCLIUsageErrorCleansExistingCompileCache(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles real fixture binaries before each CLI exit path")
	}
	for _, sample := range []struct {
		name string
		args []string
		code int
	}{
		{"missing command", nil, 2},
		{"unknown command", []string{"unknown-command"}, 2},
		{"invalid flag", []string{"all-violations", "--unknown-flag"}, 2},
		{"command help", []string{"gen-spec", "-h"}, 0},
		{"nested command help", []string{"propose", "requirement", "-h"}, 0},
	} {
		t.Run(sample.name, func(t *testing.T) {
			before := populateCLICompileCache(t)
			if code := runCLI(sample.args); code != sample.code {
				t.Fatalf("command changed its exit code: got %d want %d", code, sample.code)
			}
			for _, dir := range before {
				if _, err := os.Stat(dir); !os.IsNotExist(err) {
					t.Fatalf("command left an owned compile directory: %s %v", dir, err)
				}
			}
		})
	}
}
