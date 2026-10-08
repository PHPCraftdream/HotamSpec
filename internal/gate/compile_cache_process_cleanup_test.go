package gate

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCompileCacheTestProcessCleansSuccessAndFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("runs child test processes that compile and execute real fixtures")
	}
	for _, mode := range []string{"pass", "fail"} {
		t.Run(mode, func(t *testing.T) {
			temp := t.TempDir()
			marker := filepath.Join(temp, "compiled")
			command := exec.Command(os.Args[0], "-test.run=^TestCompileCacheLifecycleChild$")
			command.Env = append(os.Environ(), "TMP="+temp, "TEMP="+temp, "TMPDIR="+temp, "HOTAM_COMPILE_LIFECYCLE_CHILD="+mode, "HOTAM_COMPILE_LIFECYCLE_MARKER="+marker)
			out, err := command.CombinedOutput()
			if mode == "fail" {
				if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
					t.Fatalf("failing child changed exit code: %v\n%s", err, out)
				}
			} else if err != nil {
				t.Fatalf("passing child failed: %v\n%s", err, out)
			}
			if _, err := os.Stat(marker); err != nil {
				t.Fatalf("child did not compile its real fixture: %v\n%s", err, out)
			}
			dirs, err := filepath.Glob(filepath.Join(temp, "hotam-compile-*"))
			if err != nil || len(dirs) != 0 {
				t.Fatalf("test process left compile artifacts: %v %v", dirs, err)
			}
		})
	}
}

func TestCompileCacheLifecycleChild(t *testing.T) {
	mode := os.Getenv("HOTAM_COMPILE_LIFECYCLE_CHILD")
	if mode == "" {
		return
	}
	root := writeModuleFixture(t, "cleanup-child", "model", passingImplSrc, passingTestSrc)
	session := NewExecutionSession() // process TestMain is this owner's backstop
	result := session.RunVerifiedByTest(root, "model/impl_test.go", "TestRequireComplete_RejectsZeroFields")
	if result.Err != nil || !result.Passed {
		t.Fatalf("fixture execution failed: %+v", result)
	}
	if _, err := os.Stat(session.compileTmpDir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(os.Getenv("HOTAM_COMPILE_LIFECYCLE_MARKER"), []byte("compiled"), 0600); err != nil {
		t.Fatal(err)
	}
	if mode == "fail" {
		t.Fatal("intentional child failure after compile")
	}
}
