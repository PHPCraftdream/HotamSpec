package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCLICompileCleanupOnSuccessAndFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the real CLI and executes passing/failing compiled domain tests")
	}
	binary := buildSharedHotamBinary(t)
	work := t.TempDir()
	temp := filepath.Join(work, "process-temp")
	if err := os.Mkdir(temp, 0700); err != nil {
		t.Fatal(err)
	}
	environment := append(os.Environ(), "GOWORK=off", "TMP="+temp, "TEMP="+temp, "TMPDIR="+temp)
	domain, implementation := killswitchFixtureDomain(t, binary, work, environment)
	marker := filepath.Join(work, "executed")
	testFile := filepath.Join(domain, "spec", "model", "risk_test.go")
	source := `package model
import (
 "os"
 "testing"
)
func TestNewRisk_RejectsMissingOwner(t *testing.T) {
 if err := os.WriteFile(os.Getenv("HOTAM_CLEANUP_EXECUTED"), []byte("executed"), 0600); err != nil { t.Fatal(err) }
 risk, err := NewRisk("")
 if err == nil { t.Fatalf("missing owner accepted: %v", risk) }
}
`
	if err := os.WriteFile(testFile, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	environment = append(environment, "HOTAM_CLEANUP_EXECUTED="+marker)
	for _, fail := range []bool{false, true} {
		if fail {
			if err := os.WriteFile(implementation, []byte(guttedRiskImplSrc), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(marker); err != nil {
				t.Fatal(err)
			}
		}
		command := exec.Command(binary, "all-violations", "--domain", domain, "--json")
		command.Dir, command.Env = work, environment
		out, err := command.CombinedOutput()
		if fail {
			if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
				t.Fatalf("failing command lost its exit status: %v\n%s", err, out)
			}
		} else if err != nil {
			t.Fatalf("passing command failed: %v\n%s", err, out)
		}
		if _, err := os.Stat(marker); err != nil {
			t.Fatalf("the real compiled test did not execute: %v\n%s", err, out)
		}
		directories, err := filepath.Glob(filepath.Join(temp, "hotam-compile-*"))
		if err != nil || len(directories) != 0 {
			t.Fatalf("CLI left compile artifacts (fail=%v): %v %v", fail, directories, err)
		}
	}
}
