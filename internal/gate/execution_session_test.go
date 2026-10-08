package gate

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func executionSessionFixture(t *testing.T) (string, string) {
	t.Helper()
	root := writeModuleFixture(t, "session-fixture", "model", "package model\nfunc Accept() bool { return 1 == 1 }\n", `package model
import (
 "os"
 "syscall"
 "testing"
)
func TestDecision(t *testing.T) {
 marker := os.Getenv("HOTAM_SESSION_MARKER")
 // Direct syscalls make this side effect invisible to Go's native test-log
 // cache: replaying a PASS genuinely skips the observable execution marker.
 file, err := syscall.Open(marker, syscall.O_CREAT|syscall.O_APPEND|syscall.O_WRONLY, 0600)
 if err != nil { t.Fatal(err) }
 if _, err := syscall.Write(file, []byte("executed\n")); err != nil { t.Fatal(err) }
 if err := syscall.Close(file); err != nil { t.Fatal(err) }
 if !Accept() || os.Getenv("HOTAM_SESSION_PROFILE") == "reject" { t.Fatal("check_session_rejected") }
}
`)
	marker := filepath.Join(t.TempDir(), "executions")
	t.Setenv("HOTAM_SESSION_MARKER", marker)
	t.Setenv("HOTAM_SESSION_PROFILE", "accept")
	return root, marker
}

func sessionForTest(t *testing.T) *ExecutionSession {
	t.Helper()
	s := NewExecutionSession()
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	return s
}

func TestExecutionSessionFreshRunsShareCompilationNotVerdicts(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and executes real session fixtures")
	}
	root, marker := executionSessionFixture(t)
	s := sessionForTest(t)
	before := CompileInvocationCount()
	for index := range 2 {
		result := s.RunAtomPackageRecording(root, "model/impl_test.go")
		if result.Err != nil || !result.Passed {
			t.Fatalf("fresh run %d: %+v", index, result)
		}
	}
	if count := CompileInvocationCount() - before; count != 1 {
		t.Fatalf("same session compiled %d times, want 1", count)
	}
	contents, err := os.ReadFile(marker)
	if err != nil || string(contents) != "executed\nexecuted\n" {
		t.Fatalf("a verdict was replayed without executing: %q %v", contents, err)
	}
	other := sessionForTest(t)
	result := other.RunAtomPackageRecording(root, "model/impl_test.go")
	if result.Err != nil || !result.Passed {
		t.Fatalf("independent execution: %+v", result)
	}
	contents, err = os.ReadFile(marker)
	if err != nil || string(contents) != "executed\nexecuted\nexecuted\n" {
		t.Fatalf("independent owner reused a verdict: %q %v", contents, err)
	}
}

func TestExecutionSessionSnapshotSharesAndInvalidatesContentAndProfile(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and executes source/profile mutation fixtures")
	}
	root, marker := executionSessionFixture(t)
	g := &ontology.Graph{DomainDir: root, Requirements: []ontology.Requirement{{ID: "decision", VerifiedBy: []string{"model/impl_test.go:TestDecision"}}}}
	s := sessionForTest(t)
	first, err := s.CollectAtomExecutionSnapshotForPackages(g, nil)
	if err != nil || !first.PackageRuns["model"].Passed {
		t.Fatalf("first snapshot: %+v %v", first, err)
	}
	shared, err := s.CollectAtomExecutionSnapshotForPackages(g, nil)
	if err != nil || shared != first {
		t.Fatalf("unchanged snapshot was not shared: %v", err)
	}
	contents, _ := os.ReadFile(marker)
	if string(contents) != "executed\n" {
		t.Fatalf("snapshot projection reexecuted: %q", contents)
	}
	rows := CollectSpecRowsFromSnapshot(g, first)
	documents, renderErr := BuildSpecDocumentsFromRows(g, rows)
	if renderErr != nil {
		t.Fatal(renderErr)
	}
	repeated, renderErr := BuildSpecDocumentsFromRows(g, rows)
	if renderErr != nil || !reflect.DeepEqual(documents, repeated) {
		t.Fatalf("one snapshot rendered nondeterministically: %v", renderErr)
	}
	contents, _ = os.ReadFile(marker)
	if string(contents) != "executed\n" {
		t.Fatalf("rendering reexecuted the snapshot: %q", contents)
	}
	path := filepath.Join(root, "model", "impl.go")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Preserve byte length and mtime: freshness must observe content itself.
	after := strings.Replace(string(before), "==", "!=", 1)
	if err := os.WriteFile(path, []byte(after), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	changed, err := s.CollectAtomExecutionSnapshotForPackages(g, nil)
	if err != nil || changed == first || changed.PackageRuns["model"].Passed || changed.PackageRuns["model"].Err != nil {
		t.Fatalf("source mutation reused PASS: %+v %v", changed, err)
	}
	if err := os.WriteFile(path, before, 0600); err != nil {
		t.Fatal(err)
	}
	restored, err := s.CollectAtomExecutionSnapshotForPackages(g, nil)
	if err != nil || !restored.PackageRuns["model"].Passed {
		t.Fatalf("restored source did not execute afresh: %+v %v", restored, err)
	}
	t.Setenv("HOTAM_SESSION_PROFILE", "reject")
	profiled, err := s.CollectAtomExecutionSnapshotForPackages(g, nil)
	if err != nil || profiled == restored || profiled.PackageRuns["model"].Passed || profiled.PackageRuns["model"].Err != nil {
		t.Fatalf("profile mutation reused PASS: %+v %v", profiled, err)
	}
}

func TestExecutionSessionCloseOwnsOnlyItsArtifacts(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles independent session artifacts")
	}
	root, _ := executionSessionFixture(t)
	first, second := sessionForTest(t), sessionForTest(t)
	for _, owner := range []*ExecutionSession{first, second} {
		result := owner.RunVerifiedByTest(root, "model/impl_test.go", "TestDecision")
		if result.Err != nil || !result.Passed {
			t.Fatalf("compile/run: %+v", result)
		}
	}
	firstPath, secondPath := first.compileTmpDir, second.compileTmpDir
	if firstPath == secondPath {
		t.Fatal("independent sessions share an artifact directory")
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(firstPath); !os.IsNotExist(err) {
		t.Fatalf("closed owner retained artifacts: %v", err)
	}
	if _, err := os.Stat(secondPath); err != nil {
		t.Fatalf("Close deleted another owner's artifacts: %v", err)
	}
	if result := first.RunVerifiedByTest(root, "model/impl_test.go", "TestDecision"); result.Err == nil || result.Passed {
		t.Fatalf("closed owner accepted execution: %+v", result)
	}
	if _, registered := executionSessions.Load(first); registered {
		t.Fatal("successful Close retained owner")
	}
	if result := second.RunAtomPackageRecording(root, "model/impl_test.go"); result.Err != nil || !result.Passed {
		t.Fatalf("surviving owner cannot execute: %+v", result)
	}
}

func TestExecutionSessionCloseWaitsForUsersAndRejectsNewWork(t *testing.T) {
	s := sessionForTest(t)
	if err := s.begin(); err != nil {
		t.Fatal(err)
	}
	dir, err := s.makeTempDir("hotam-session-user-")
	if err != nil {
		s.users.Done()
		t.Fatal(err)
	}
	closed := make(chan error, 1)
	go func() { closed <- s.Close() }()
	// begin synchronizes with Close's admission lock: once it rejects, the
	// admitted user still owns its resources until it explicitly stops.
	for {
		if err := s.begin(); err != nil {
			break
		}
		s.users.Done()
		runtime.Gosched()
	}
	if _, err := os.Stat(dir); err != nil {
		s.users.Done()
		t.Fatalf("Close deleted an active user's path: %v", err)
	}
	select {
	case err := <-closed:
		s.users.Done()
		t.Fatalf("Close returned before users stopped: %v", err)
	default:
	}
	s.users.Done()
	if err := <-closed; err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("joined owner retained path: %v", err)
	}
}

func TestExecutionSessionPackageFailureKeepsPassingSiblingVerdicts(t *testing.T) {
	if testing.Short() {
		t.Skip("executes a genuinely failing package with passing sibling")
	}
	root, _ := executionSessionFixture(t)
	if err := os.WriteFile(filepath.Join(root, "model", "rejected_test.go"), []byte("package model\nimport \"testing\"\nfunc TestRejected(t *testing.T) { t.Fatal(\"rejected case\") }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	session := sessionForTest(t)
	result := session.RunAtomPackageRecording(root, "model/impl_test.go")
	if result.Err != nil || result.Passed || result.CompileFailed {
		t.Fatalf("failing package classification: %+v", result)
	}
	if decision := result.ForTest("TestDecision"); decision.Err != nil || !decision.Passed {
		t.Fatalf("passing sibling was erased: %+v", decision)
	}
	if rejected := result.ForTest("TestRejected"); rejected.Err != nil || rejected.Passed {
		t.Fatalf("failing case was promoted to PASS: %+v", rejected)
	}
	if absent := result.ForTest("TestMissing"); absent.Err == nil || absent.Passed {
		t.Fatalf("unexecuted case acquired a fake verdict: %+v", absent)
	}
}

func TestExecutionSessionConcurrentRequestsShareOneCompilation(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and executes concurrent verified tests")
	}
	root := writeModuleFixture(t, "concurrent-session", "model", twoTestImplSrc, twoTestTestSrc)
	session := sessionForTest(t)
	before := CompileInvocationCount()
	release := make(chan struct{})
	results := make(chan TestRunResult, 8)
	for index := range 8 {
		test := "TestIsPositive_RejectsZero"
		if index%2 != 0 {
			test = "TestIsNegative_RejectsZero"
		}
		go func(test string) {
			<-release
			results <- session.RunVerifiedByTest(root, "model/impl_test.go", test)
		}(test)
	}
	close(release)
	for range 8 {
		result := <-results
		if result.Err != nil || !result.Passed {
			t.Fatalf("concurrent test request: %+v", result)
		}
	}
	if count := CompileInvocationCount() - before; count != 1 {
		t.Fatalf("concurrent tests compiled %d times, want one shared artifact", count)
	}
}

func TestExecutionSessionSameStatRenameIsFreshForInventoryEligibilityAndExecution(t *testing.T) {
	if testing.Short() {
		t.Skip("executes a test renamed without size or mtime changes")
	}
	root, marker := executionSessionFixture(t)
	session := sessionForTest(t)
	file := "model/impl_test.go"
	path := filepath.Join(root, filepath.FromSlash(file))
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	eligible, err := session.ResolveSpecTest(root, file, "TestDecision")
	if err != nil || !eligible.Found || !eligible.HasTeeth {
		t.Fatalf("original test eligibility: %+v %v", eligible, err)
	}
	initialRange, found, err := session.ResolveSpecSymbolRange(root, file, "TestDecision")
	if err != nil || !found {
		t.Fatalf("original symbol range: %+v %v", initialRange, err)
	}
	mapping, err := session.BuildCheckToTestsMap(filepath.Dir(path))
	if err != nil || !reflect.DeepEqual(mapping["check_session_rejected"], []string{"TestDecision"}) {
		t.Fatalf("initial check-to-test binding: %v %v", mapping, err)
	}
	if result := session.RunVerifiedByTest(root, file, "TestDecision"); result.Err != nil || !result.Passed {
		t.Fatalf("initial real execution: %+v", result)
	}
	renamed := strings.Replace(string(original), "TestDecision", "TestReplaced", 1)
	if len(renamed) != len(original) {
		t.Fatal("rename fixture did not preserve byte length")
	}
	if err := os.WriteFile(path, []byte(renamed), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	eligible, err = session.ResolveSpecTest(root, file, "TestDecision")
	if err != nil || eligible.Found {
		t.Fatalf("old test remained eligible after same-stat rename: %+v %v", eligible, err)
	}
	eligible, err = session.ResolveSpecTest(root, file, "TestReplaced")
	if err != nil || !eligible.Found || !eligible.HasTeeth {
		t.Fatalf("renamed valid test became ineligible: %+v %v", eligible, err)
	}
	if _, found, err := session.ResolveSpecSymbolRange(root, file, "TestDecision"); err != nil || found {
		t.Fatalf("old symbol range survived rename: found=%v error=%v", found, err)
	}
	if range_, found, err := session.ResolveSpecSymbolRange(root, file, "TestReplaced"); err != nil || !found || range_.StartLine != initialRange.StartLine || range_.EndLine != initialRange.EndLine {
		t.Fatalf("renamed symbol range did not reflect actual source: %+v found=%v error=%v", range_, found, err)
	}
	mapping, err = session.BuildCheckToTestsMap(filepath.Dir(path))
	if err != nil || !reflect.DeepEqual(mapping["check_session_rejected"], []string{"TestReplaced"}) {
		t.Fatalf("session inventory retained old binding: %v %v", mapping, err)
	}
	freshMapping, err := BuildCheckToTestsMap(filepath.Dir(path))
	if err != nil || !reflect.DeepEqual(freshMapping["check_session_rejected"], []string{"TestReplaced"}) {
		t.Fatalf("standalone inventory retained old binding: %v %v", freshMapping, err)
	}
	if result := session.RunVerifiedByTest(root, file, "TestDecision"); result.Err == nil || result.Passed {
		t.Fatalf("absent old test fabricated PASS: %+v", result)
	}
	if result := session.RunVerifiedByTest(root, file, "TestReplaced"); result.Err != nil || !result.Passed {
		t.Fatalf("renamed test was not actually executed: %+v", result)
	}
	contents, err := os.ReadFile(marker)
	if err != nil || string(contents) != "executed\nexecuted\n" {
		t.Fatalf("rename verdicts do not correspond to real executions: %q %v", contents, err)
	}
}

func TestExecutionSessionPublicationOutputsDoNotInvalidateAuthoredInputs(t *testing.T) {
	if testing.Short() {
		t.Skip("executes source snapshots around publication and fixture edits")
	}
	root, marker := executionSessionFixture(t)
	put := func(relative, contents string) {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	put("model/testdata/decision", "accept")
	put("model/testdata/nested/go.mod", "module authored-fixture\n")
	put("model/fixture_test.go", "package model\nimport (\n\"os\"\n\"testing\"\n)\nfunc TestInput(t *testing.T) { data, err := os.ReadFile(\"testdata/decision\"); if err != nil { t.Fatal(err) }; if string(data) != \"accept\" { t.Fatal(\"authored fixture rejected\") }; nested, err := os.ReadFile(\"testdata/nested/go.mod\"); if err != nil { t.Fatal(err) }; if string(nested) != \"module authored-fixture\\n\" { t.Fatal(\"nested authored fixture rejected\") } }\n")
	outputs := []string{"docs/gen/SPEC.md", "docs/gen/evidence.json", "framework/GLOSSARY.md", "CLAUDE.md"}
	for _, relative := range outputs {
		put(relative, "published 2026-10-07\n")
	}
	graph := &ontology.Graph{DomainDir: root, Requirements: []ontology.Requirement{{ID: "decision", VerifiedBy: []string{"model/impl_test.go:TestDecision", "model/fixture_test.go:TestInput"}}}}
	session := sessionForTest(t)
	first, err := session.CollectAtomExecutionSnapshotForPackages(graph, nil)
	if err != nil || !first.PackageRuns["model"].Passed {
		t.Fatalf("initial execution: %+v %v", first, err)
	}
	for _, relative := range outputs {
		put(relative, "published 2026-10-08\n")
	}
	put("unrelated/component/go.mod", "module unrelated-component\n")
	put("unrelated/component/value.go", "not source in this owning module\n")
	put("other-checkout/.git", "gitdir: detached-owner\n")
	put("other-checkout/value.go", "another process's checkout\n")
	shared, err := session.CollectAtomExecutionSnapshotForPackages(graph, nil)
	if err != nil || shared != first {
		t.Fatalf("publication or another owner invalidated captured execution: %v", err)
	}
	contents, err := os.ReadFile(marker)
	if err != nil || string(contents) != "executed\n" {
		t.Fatalf("publication silently reran source: %q %v", contents, err)
	}
	put("unrelated/component/value.go", "unrelated module changed again\n")
	put("other-checkout/value.go", "other checkout changed again\n")
	shared, err = session.CollectAtomExecutionSnapshotForPackages(graph, nil)
	if err != nil || shared != first {
		t.Fatalf("separate owner mutation invalidated this snapshot: %v", err)
	}
	put("model/testdata/decision", "reject")
	changed, err := session.CollectAtomExecutionSnapshotForPackages(graph, nil)
	if err != nil || changed == first || changed.PackageRuns["model"].Passed || changed.PackageRuns["model"].Err != nil {
		t.Fatalf("authored input mutation did not invalidate and genuinely fail: %+v %v", changed, err)
	}
	contents, err = os.ReadFile(marker)
	if err != nil || string(contents) != "executed\nexecuted\n" {
		t.Fatalf("authored input verdict was not freshly executed: %q %v", contents, err)
	}
	if result := changed.PackageRuns["model"].ForTest("TestInput"); result.Err != nil || result.Passed {
		t.Fatalf("changed fixture's real failure was lost: %+v", result)
	}
	put("model/testdata/decision", "accept")
	restored, err := session.CollectAtomExecutionSnapshotForPackages(graph, nil)
	if err != nil || !restored.PackageRuns["model"].Passed {
		t.Fatalf("restored fixture did not execute and pass: %+v %v", restored, err)
	}
	put("model/testdata/nested/go.mod", "module changed-authored-fixture\n")
	recollected, err := session.CollectAtomExecutionSnapshotForPackages(graph, nil)
	if err != nil || recollected == restored || recollected.PackageRuns["model"].Passed || recollected.PackageRuns["model"].Err != nil {
		t.Fatalf("nested authored testdata module was wrongly ignored: %+v %v", recollected, err)
	}
}

func TestExecutionSnapshotRunsOnlyAdmittedReferencesAndDiscoveredAtoms(t *testing.T) {
	if testing.Short() {
		t.Skip("executes scoped and explicit full-package atom fixtures")
	}
	root := writeAtomPipelineFixture(t, map[string]string{
		"spec/model/model.go": "package model\ntype Box struct{}\n// A source value is seven.\nfunc (Box) Value() int { return 7 }\n",
		"spec/model/model_test.go": `package model
import (
 "os"
 "syscall"
 "testing"
 hs "example.test/pipeline/hotamspec"
)
func mark(t *testing.T, name string) {
 handle, err := syscall.Open(os.Getenv("HOTAM_SCOPED_EXECUTIONS"), syscall.O_CREAT|syscall.O_APPEND|syscall.O_WRONLY, 0600)
 if err != nil { t.Fatal(err) }
 if _, err := syscall.Write(handle, []byte(name+"\n")); err != nil { t.Fatal(err) }
 if err := syscall.Close(handle); err != nil { t.Fatal(err) }
}
func TestReferenced(t *testing.T) {
 t.Run("selected", func(t *testing.T) { mark(t, "referenced"); if (Box{}).Value() != 7 { t.Fatal("source value is not seven") } })
}
func TestReportOnlyAtom(t *testing.T) {
 mark(t, "atom")
 hs.Fact(t, (Box{}).Value, 7)
}
func TestUnrelatedFailure(t *testing.T) {
 mark(t, "unrelated")
 t.Fatal("this test must not enter a scoped snapshot")
}
`,
	})
	marker := filepath.Join(t.TempDir(), "executions")
	t.Setenv("HOTAM_SCOPED_EXECUTIONS", marker)
	reference := "spec/model/model_test.go:TestReferenced/selected"
	graph := &ontology.Graph{DomainDir: root, SelfExecutingAtoms: true, Requirements: []ontology.Requirement{{ID: "selected", VerifiedBy: []string{reference}, ImplementedBy: []string{"spec/model/model.go:Box.Value"}}}}
	session := sessionForTest(t)
	before := CompileInvocationCount()
	snapshot, err := session.CollectAtomExecutionSnapshotForPackages(graph, nil)
	if err != nil || snapshot.SourceErr != nil || snapshot.DiscoveryErr != nil {
		t.Fatalf("scoped collection: snapshot=%+v error=%v", snapshot, err)
	}
	run := snapshot.PackageRuns["spec/model"]
	if run.Err != nil || !run.Passed || run.CompileFailed {
		t.Fatalf("unrelated failure entered the scoped run: %+v", run)
	}
	if result := run.ForTest("TestReferenced/selected"); result.Err != nil || !result.Passed {
		t.Fatalf("declared subtest has no passing terminal verdict: %+v", result)
	}
	if result := run.ForTest("TestReportOnlyAtom"); result.Err != nil || !result.Passed {
		t.Fatalf("discovered atom was not executed: %+v", result)
	}
	if result := run.ForTest("TestUnrelatedFailure"); result.Err == nil || result.Passed {
		t.Fatalf("unrelated test acquired a verdict: %+v", result)
	}
	if count := CompileInvocationCount() - before; count != 1 {
		t.Fatalf("admitted tests did not share one compilation: %d", count)
	}
	if !SymbolRangeCoveredByProfile(ParseCoverProfile(run.CoverProfile), "example.test/pipeline/model/model.go", 4, 4) {
		t.Fatal("scoped combined run lost declared implementation coverage")
	}
	contents, err := os.ReadFile(marker)
	if err != nil || string(contents) != "referenced\natom\n" {
		t.Fatalf("scoped admission did not execute exactly its two tests once: %q %v", contents, err)
	}
	shared, err := session.CollectAtomExecutionSnapshotForPackages(graph, nil)
	if err != nil || shared != snapshot {
		t.Fatalf("current scoped capture was not shared: %v", err)
	}
	contents, err = os.ReadFile(marker)
	if err != nil || string(contents) != "referenced\natom\n" {
		t.Fatalf("snapshot reuse reexecuted or admitted unrelated work: %q %v", contents, err)
	}
	path := filepath.Join(root, "spec", "model", "model_test.go")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(source), "TestReferenced", "TestReplacement", 1)), 0600); err != nil {
		t.Fatal(err)
	}
	missing, err := session.CollectAtomExecutionSnapshotForPackages(graph, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result := missing.PackageRuns["spec/model"]; result.Err == nil || result.Passed {
		t.Fatalf("renamed admitted reference fabricated PASS: %+v", result)
	}
	// The explicit public full-package API remains genuine: its failing sibling
	// must now execute, rather than inheriting the scoped snapshot's admissions.
	full := session.RunAtomPackageRecording(root, "spec/model/model_test.go")
	if full.Err != nil || full.Passed || full.CompileFailed {
		t.Fatalf("full-package API did not execute the unrelated failure: %+v", full)
	}
	if result := full.ForTest("TestUnrelatedFailure"); result.Err != nil || result.Passed {
		t.Fatalf("full-package failing test has no real terminal result: %+v", result)
	}
	contents, err = os.ReadFile(marker)
	if err != nil || strings.Count(string(contents), "unrelated\n") != 1 {
		t.Fatalf("unrelated test was not excluded until the explicit full-package request: %q %v", contents, err)
	}
}
