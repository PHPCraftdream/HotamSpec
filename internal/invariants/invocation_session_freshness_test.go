package invariants

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/evidence"
	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func TestInvocationWrappersValidateMutationAndClosedOwnership(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and executes wrapper lifecycle fixtures")
	}
	root := t.TempDir()
	marker := filepath.Join(t.TempDir(), "executions")
	t.Setenv("HOTAM_INVOCATION_MARKER", marker)
	t.Setenv("HOTAM_INVOCATION_PROFILE", "accept")
	files := map[string]string{
		"spec/go.mod":         "module invocation-fixture\n\ngo 1.22\n",
		"spec/model/value.go": "package model\nfunc Accept() bool { return 1 == 1 }\n",
		"spec/model/value_test.go": `package model
import (
 "os"
 "syscall"
 "testing"
)
func TestDecision(t *testing.T) {
 handle, err := syscall.Open(os.Getenv("HOTAM_INVOCATION_MARKER"), syscall.O_CREAT|syscall.O_APPEND|syscall.O_WRONLY, 0600)
 if err != nil { t.Fatal(err) }
 if _, err := syscall.Write(handle, []byte("executed\n")); err != nil { t.Fatal(err) }
 if err := syscall.Close(handle); err != nil { t.Fatal(err) }
 if !Accept() || os.Getenv("HOTAM_INVOCATION_PROFILE") == "reject" { t.Fatal("decision rejected") }
}
`,
	}
	for relative, contents := range files {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	reference := "spec/model/value_test.go:TestDecision"
	graph := &ontology.Graph{DomainDir: root, Requirements: []ontology.Requirement{{ID: "decision", Claim: "decision is accepted", Status: ontology.StatusSETTLED, VerifiedBy: []string{reference}}}}
	view, first, err := InvocationExecutionSnapshot(graph)
	if err != nil || !first.PackageRuns["spec/model"].Passed {
		t.Fatalf("initial wrapper execution: %+v %v", first, err)
	}
	t.Cleanup(func() {
		if err := CloseInvocation(view); err != nil {
			t.Error(err)
		}
	})
	if graph.InvocationState != nil {
		t.Fatal("wrapper mutated persistent caller graph")
	}
	assertVerdict := func(report evidence.Report, want string) {
		t.Helper()
		for _, requirement := range report.Requirements {
			for _, observed := range requirement.Tests {
				if observed.Reference == reference {
					if observed.Verdict != want {
						t.Fatalf("evidence wrapper replayed %q, want %q", observed.Verdict, want)
					}
					return
				}
			}
		}
		t.Fatal("evidence wrapper omitted the requested real test")
	}
	report, err := InvocationEvidenceSnapshot(view)
	if err != nil {
		t.Fatal(err)
	}
	assertVerdict(report, evidence.VerdictPass)
	_, shared, err := InvocationExecutionSnapshot(view)
	if err != nil || shared != first {
		t.Fatalf("current wrapper did not share its capture: %v", err)
	}
	path := filepath.Join(root, "spec", "model", "value.go")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	original := files["spec/model/value.go"]
	if err := os.WriteFile(path, []byte(strings.Replace(original, "==", "!=", 1)), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	// Request evidence first: it must itself acquire current execution rather
	// than return the report cached before this same-stat source mutation.
	report, err = InvocationEvidenceSnapshot(view)
	if err != nil {
		t.Fatal(err)
	}
	assertVerdict(report, evidence.VerdictFail)
	_, changed, err := InvocationExecutionSnapshot(view)
	if err != nil || changed == first || changed.PackageRuns["spec/model"].Passed {
		t.Fatalf("execution wrapper retained old source PASS: %+v %v", changed, err)
	}
	if !first.PackageRuns["spec/model"].Passed {
		t.Fatal("refresh mutated an already acquired immutable capture")
	}
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	_, restored, err := InvocationExecutionSnapshot(view)
	if err != nil || !restored.PackageRuns["spec/model"].Passed {
		t.Fatalf("restored source: %+v %v", restored, err)
	}
	report, err = InvocationEvidenceSnapshot(view)
	if err != nil {
		t.Fatal(err)
	}
	assertVerdict(report, evidence.VerdictPass)
	t.Setenv("HOTAM_INVOCATION_PROFILE", "reject")
	report, err = InvocationEvidenceSnapshot(view)
	if err != nil {
		t.Fatal(err)
	}
	assertVerdict(report, evidence.VerdictFail)
	rows := gate.CollectSpecRowsFromSnapshot(view, restored)
	beforeClose, err := gate.BuildSpecDocumentsFromRows(view, rows)
	if err != nil {
		t.Fatal(err)
	}
	if err := CloseInvocation(view); err != nil {
		t.Fatal(err)
	}
	if _, cached, err := InvocationExecutionSnapshot(view); err == nil || cached != nil {
		t.Fatalf("closed borrowed owner replayed execution: %+v %v", cached, err)
	}
	if cached, err := InvocationEvidenceSnapshot(view); err == nil || cached.SchemaVersion != 0 {
		t.Fatalf("closed borrowed owner replayed evidence: %+v %v", cached, err)
	}
	afterClose, err := gate.BuildSpecDocumentsFromRows(view, rows)
	if err != nil || !reflect.DeepEqual(beforeClose, afterClose) {
		t.Fatalf("closing owner invalidated pure acquired-snapshot rendering: %v", err)
	}
	contents, err := os.ReadFile(marker)
	if err != nil || string(contents) != "executed\nexecuted\nexecuted\nexecuted\n" {
		t.Fatalf("wrapper verdicts/rendering did not correspond to four real captures: %q %v", contents, err)
	}
}
