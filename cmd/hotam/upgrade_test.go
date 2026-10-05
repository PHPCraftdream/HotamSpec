package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	ontologyvendor "github.com/PHPCraftdream/HotamSpec/internal/ontology/vendor"
	paths "github.com/PHPCraftdream/HotamSpec/internal/paths"
	recordervendor "github.com/PHPCraftdream/HotamSpec/internal/recorder/vendor"
)

// upgradeFixture scaffolds a full consumer domain under a temp project root
// (initDomainUnderRoot keeps repoRootForDomain off this repo's real root —
// see its doc comment) with a spec/ Go module carrying vendored recorder +
// ontology mirror + a scaffolded registrydump, then STALEN every generated
// file by appending junk. Returns the domain dir, the spec/ module path, and
// the expected (canonical) registrydump content.
func upgradeFixture(t *testing.T, name string) (domainDir, modulePath, expectedRegistrydump string) {
	t.Helper()
	_, domainDir = initDomainUnderRoot(t, name, "2026-10-05")
	specDir := filepath.Join(domainDir, "spec")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("mkdir spec: %v", err)
	}
	modulePath = name + "-spec"
	if err := os.WriteFile(filepath.Join(specDir, "go.mod"), []byte("module "+modulePath+"\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if _, err := vendorRecorder(domainDir); err != nil {
		t.Fatalf("vendorRecorder: %v", err)
	}
	if _, err := vendorOntology(domainDir); err != nil {
		t.Fatalf("vendorOntology: %v", err)
	}
	if _, err := scaffoldRegistrydump(domainDir); err != nil {
		t.Fatalf("scaffoldRegistrydump: %v", err)
	}
	expectedRegistrydump = registrydumpSource(modulePath, false)

	// Stale every generated copy so the upgrade has real work to do.
	for _, p := range []string{
		filepath.Join(specDir, "hotamspec", "hotamspec.go"),
		filepath.Join(specDir, "hotamontology", "requirement.go"),
		filepath.Join(specDir, "hotamontology", "registry.go"),
		filepath.Join(specDir, "registrydump", "main.go"),
	} {
		content := []byte("// stale\npackage x\n")
		if strings.HasSuffix(p, filepath.Join("registrydump", "main.go")) {
			// A STALE engine-generated file still carries the banner — only
			// the body drifted. (A banner-stripped file is the hand-modified
			// case, covered by TestUpgrade_KeepsHandModifiedRegistrydump.)
			content = []byte(registrydumpBanner + "\n// stale\npackage x\n")
		}
		if err := os.WriteFile(p, content, 0o644); err != nil {
			t.Fatalf("stale %s: %v", p, err)
		}
	}
	return domainDir, modulePath, expectedRegistrydump
}

// TestUpgrade_RefreshesStaleVendoredCopies proves the happy path: a domain
// with stale vendored recorder/ontology copies and a stale engine-generated
// registrydump comes out byte-identical to canon again after one upgrade.
func TestUpgrade_RefreshesStaleVendoredCopies(t *testing.T) {
	t.Parallel()
	domainDir, _, wantRD := upgradeFixture(t, "up-stale")

	lines, err := runUpgradeSteps(domainDir, resolveClaudeMDPath(domainDir, ""), "2026-10-05")
	if err != nil {
		t.Fatalf("runUpgradeSteps: %v", err)
	}
	joined := strings.Join(lines, "\n")
	for _, want := range []string{
		"refreshed: spec/hotamspec/hotamspec.go",
		"refreshed: spec/hotamontology/",
		"refreshed: spec/registrydump/main.go",
		"regenerated: docs + crystal",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("summary missing %q:\n%s", want, joined)
		}
	}

	specDir := filepath.Join(domainDir, "spec")
	if got, err := os.ReadFile(filepath.Join(specDir, "hotamspec", "hotamspec.go")); err != nil || string(got) != recordervendor.Source() {
		t.Errorf("spec/hotamspec/hotamspec.go not back to canon (err=%v)", err)
	}
	if got, err := os.ReadFile(filepath.Join(specDir, "hotamontology", "requirement.go")); err != nil || string(got) != ontologyvendor.RequirementSource() {
		t.Errorf("spec/hotamontology/requirement.go not back to canon (err=%v)", err)
	}
	if got, err := os.ReadFile(filepath.Join(specDir, "registrydump", "main.go")); err != nil || string(got) != wantRD {
		t.Errorf("spec/registrydump/main.go not back to generated template (err=%v)", err)
	}
}

// TestUpgrade_Idempotent proves a second upgrade run changes nothing: the
// upgrade-owned files keep their mtimes (write-if-changed) and the whole
// domain tree stays byte-identical. (docs/gen mtimes DO move — genSpec, the
// shared generator `hotam gen-spec` uses, rewrites unconditionally — which is
// why idempotency is asserted on bytes for the full tree and on mtime only
// for upgrade's own writes.)
func TestUpgrade_Idempotent(t *testing.T) {
	t.Parallel()
	domainDir, _, _ := upgradeFixture(t, "up-idem")

	if _, err := runUpgradeSteps(domainDir, resolveClaudeMDPath(domainDir, ""), "2026-10-05"); err != nil {
		t.Fatalf("first upgrade: %v", err)
	}
	specDir := filepath.Join(domainDir, "spec")
	owned := []string{
		filepath.Join(specDir, "hotamspec", "hotamspec.go"),
		filepath.Join(specDir, "hotamontology", "requirement.go"),
		filepath.Join(specDir, "hotamontology", "registry.go"),
		filepath.Join(specDir, "registrydump", "main.go"),
	}
	mtimes := map[string]int64{}
	for _, p := range owned {
		fi, err := os.Stat(p)
		if err != nil {
			t.Fatalf("stat %s: %v", p, err)
		}
		mtimes[p] = fi.ModTime().UnixNano()
	}
	before := listFilesWithBytes(t, domainDir)

	if lines, err := runUpgradeSteps(domainDir, resolveClaudeMDPath(domainDir, ""), "2026-10-05"); err != nil {
		t.Fatalf("second upgrade: %v", err)
	} else if joined := strings.Join(lines, "\n"); !strings.Contains(joined, "already current: spec/hotamspec") || !strings.Contains(joined, "already current: spec/hotamontology/") {
		t.Errorf("second run summary should report vendored copies already current:\n%s", joined)
	}

	for _, p := range owned {
		fi, err := os.Stat(p)
		if err != nil {
			t.Fatalf("stat %s: %v", p, err)
		}
		if fi.ModTime().UnixNano() != mtimes[p] {
			t.Errorf("%s was rewritten by the second upgrade run (mtime changed)", p)
		}
	}
	after := listFilesWithBytes(t, domainDir)
	if len(before) != len(after) {
		t.Fatalf("file set changed between runs: %d vs %d", len(before), len(after))
	}
	for p, b := range before {
		ab, ok := after[p]
		if !ok || string(b) != string(ab) {
			t.Errorf("%s content changed between upgrade runs", p)
		}
	}
}

// TestUpgrade_KeepsHandModifiedRegistrydump proves a registrydump whose
// engine banner was stripped (hand-modified) is left byte-identical and
// reported as kept — the upgrade must not silently discard hand edits.
func TestUpgrade_KeepsHandModifiedRegistrydump(t *testing.T) {
	t.Parallel()
	domainDir, _, _ := upgradeFixture(t, "up-hand")
	rdPath := filepath.Join(domainDir, "spec", "registrydump", "main.go")
	handEdited := []byte("package main\n\n// hand-tuned by the domain author\nfunc main() {}\n")
	if err := os.WriteFile(rdPath, handEdited, 0o644); err != nil {
		t.Fatalf("write hand-edited main.go: %v", err)
	}

	lines, err := runUpgradeSteps(domainDir, resolveClaudeMDPath(domainDir, ""), "2026-10-05")
	if err != nil {
		t.Fatalf("runUpgradeSteps: %v", err)
	}
	if joined := strings.Join(lines, "\n"); !strings.Contains(joined, "hand-modified") {
		t.Errorf("summary must report the hand-modified registrydump:\n%s", joined)
	}
	got, err := os.ReadFile(rdPath)
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	if string(got) != string(handEdited) {
		t.Errorf("hand-modified registrydump was overwritten")
	}
}

// TestUpgrade_NoSpecTree proves a domain with no spec/ tree at all upgrades
// cleanly: vendoring is skipped (not an error) and docs are still regenerated.
func TestUpgrade_NoSpecTree(t *testing.T) {
	t.Parallel()
	_, domainDir := initDomainUnderRoot(t, "up-nospec", "2026-10-05")

	lines, err := runUpgradeSteps(domainDir, resolveClaudeMDPath(domainDir, ""), "2026-10-05")
	if err != nil {
		t.Fatalf("runUpgradeSteps on a spec-less domain: %v", err)
	}
	joined := strings.Join(lines, "\n")
	for _, want := range []string{
		"skipped: spec/hotamspec",
		"skipped: spec/hotamontology",
		"skipped: spec/registrydump/main.go",
		"regenerated: docs + crystal",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("summary missing %q:\n%s", want, joined)
		}
	}
	if _, err := os.Stat(filepath.Join(domainDir, "docs", "gen")); err != nil {
		t.Errorf("docs/gen not regenerated: %v", err)
	}
}

// TestCmdUpgrade_EndToEnd drives the full command (flag parsing + final
// all-violations print) against the stale fixture: it must return nil (the
// violation count is printed output, never an error) and print the summary
// plus the violation ledger.
func TestCmdUpgrade_EndToEnd(t *testing.T) {
	t.Parallel()
	domainDir, _, _ := upgradeFixture(t, "up-e2e")

	err := cmdUpgrade([]string{"--domain", domainDir, "--today", "2026-10-05"})
	if err != nil {
		t.Fatalf("cmdUpgrade: %v", err)
	}
}

// TestUpgrade_DisciplineFull_ConvergesInOnePass reproduces the live upgrade
// defect on a discipline:"full" consumer domain that has adopted the crystal
// convention: check_spec_md_current fires for such a domain even when
// docs/gen/SPEC.md is simply absent (spec_md_current.go's F3 branch), and the
// resident crystal's LIVE-STATE embeds the violation snapshot of the render
// that produced it. upgrade used to call genSpec with includeSpec=false, so
// ONE pass left SPEC.md missing (and a crystal disagreeing with a fresh
// render); only a SECOND run converged. The fix passes specRenderNeeded —
// the same gate `hotam land` uses — so the first upgrade must leave the
// domain at ZERO violations, and a second run must be a byte-level no-op.
func TestUpgrade_DisciplineFull_ConvergesInOnePass(t *testing.T) {
	t.Parallel()
	domainDir, _, _ := upgradeFixture(t, "up-discfull")

	// Opt the domain into discipline:"full" (manifest is the single source
	// ResolveDiscipline/specRenderNeeded read).
	manifestPath := filepath.Join(domainDir, "manifest.json")
	manifest := "{\"self_hosting\": false, \"gen_profile\": \"consumer\", \"discipline\": \"full\", \"parent\": null}\n"
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatalf("write discipline-full manifest: %v", err)
	}
	// Adopt the crystal convention at the project root so the domain carries
	// a resident crystal (the surface check_domain_claude_md_current guards).
	projectRoot := filepath.Dir(filepath.Dir(domainDir))
	if err := os.WriteFile(filepath.Join(projectRoot, paths.MarkerFilename), nil, 0o644); err != nil {
		t.Fatalf("write project marker: %v", err)
	}
	claudeMDPath := resolveClaudeMDPath(domainDir, "")
	if claudeMDPath == "" {
		t.Fatal("resolveClaudeMDPath returned empty — fixture did not adopt the crystal convention")
	}
	// A pre-existing (stale) crystal, as after any engine upgrade.
	if err := os.WriteFile(claudeMDPath, []byte("# stale crystal\n"), 0o644); err != nil {
		t.Fatalf("write stale crystal: %v", err)
	}

	// ONE upgrade pass must fully converge the domain.
	if _, err := runUpgradeSteps(domainDir, claudeMDPath, "2026-10-05"); err != nil {
		t.Fatalf("first upgrade: %v", err)
	}
	violations, err := allViolations(domainDir)
	if err != nil {
		t.Fatalf("allViolations after first upgrade: %v", err)
	}
	for _, v := range violations {
		t.Errorf("violation survived one upgrade pass: [%s] %s: %s", v.Check, v.ID, v.Message)
	}

	// One-pass convergence: a second upgrade run must leave the whole domain
	// tree byte-identical.
	before := listFilesWithBytes(t, domainDir)
	if _, err := runUpgradeSteps(domainDir, claudeMDPath, "2026-10-05"); err != nil {
		t.Fatalf("second upgrade: %v", err)
	}
	after := listFilesWithBytes(t, domainDir)
	if len(before) != len(after) {
		t.Fatalf("file set changed between upgrade runs: %d vs %d", len(before), len(after))
	}
	for p, b := range before {
		ab, ok := after[p]
		if !ok || string(b) != string(ab) {
			t.Errorf("%s content changed between upgrade runs", p)
		}
	}
}

// listFilesWithBytes maps slash-relative paths under dir to their contents.
func listFilesWithBytes(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = data
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
	return out
}
