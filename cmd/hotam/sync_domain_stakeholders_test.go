package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

const stakeholdersModule = "hotamspec-fixture-consumer"

const syncDomainStakeholdersGoTemplate = `package spec

import hotamontology "{{MODULE}}/hotamontology"

var Stakeholders = hotamontology.New[hotamontology.Stakeholder]()

func init() {
	Stakeholders.MustRegister("code-owner", hotamontology.Stakeholder{
		ID:     "code-owner",
		Name:   "{{NAME}}",
		Domain: "owns requirements declared in Go",
	})
	// Same ID as the graph's seed stakeholder, different Name: must NOT rewrite it.
	Stakeholders.MustRegister("fixture-owner", hotamontology.Stakeholder{
		ID:     "fixture-owner",
		Name:   "Renamed In Code",
		Domain: "should never overwrite the graph",
	})
}
`

// declareStakeholders turns the fixture into a code-stakeholder domain:
// vendors the (now stakeholder-aware) mirror, writes spec/stakeholders.go,
// points R-fixture-one's owner at the code-declared stakeholder, and
// re-scaffolds registrydump (which then picks the envelope variant).
func (fx *syncDomainFixture) declareStakeholders(t *testing.T, name string) {
	t.Helper()
	if _, err := vendorOntology(fx.domainDir); err != nil {
		t.Fatalf("vendorOntology: %v", err)
	}
	src := strings.NewReplacer("{{MODULE}}", stakeholdersModule, "{{NAME}}", name).Replace(syncDomainStakeholdersGoTemplate)
	if err := os.WriteFile(filepath.Join(fx.specDir, "stakeholders.go"), []byte(src), 0o644); err != nil {
		t.Fatalf("write stakeholders.go: %v", err)
	}
	reqPath := filepath.Join(fx.specDir, "requirements.go")
	reqSrc, err := os.ReadFile(reqPath)
	if err != nil {
		t.Fatalf("read requirements.go: %v", err)
	}
	reqSrc = []byte(strings.Replace(string(reqSrc), `"fixture-owner"`, `"code-owner"`, 1))
	if err := os.WriteFile(reqPath, reqSrc, 0o644); err != nil {
		t.Fatalf("write requirements.go: %v", err)
	}
	if _, err := scaffoldRegistrydump(fx.domainDir); err != nil {
		t.Fatalf("scaffoldRegistrydump: %v", err)
	}
}

func stakeholderByID(g *ontology.Graph, id string) (ontology.Stakeholder, bool) {
	for _, s := range g.Stakeholders {
		if s.ID == id {
			return s, true
		}
	}
	return ontology.Stakeholder{}, false
}

// A stakeholder declared in code shows up in the dry-run, changes the
// diff-hash, lands with --confirm-hash BEFORE the requirement that owns it
// (0 violations), and an existing graph stakeholder is never rewritten.
func TestCmdSyncDomain_CodeStakeholderDryRunAndLand(t *testing.T) {
	if testing.Short() {
		t.Skip("spawns a real `go run` subprocess; skipped in -short")
	}
	fx := newSyncDomainFixture(t)
	fx.declareStakeholders(t, "Code Owner")
	before, err := os.ReadFile(fx.graphPath)
	if err != nil {
		t.Fatal(err)
	}

	dryOut, _, err := runSyncDomain(t, []string{"--domain", fx.domainDir})
	if err != nil {
		t.Fatalf("dry-run: %v\n%s", err, dryOut)
	}
	if !strings.Contains(dryOut, "[ADDED stakeholder] code-owner") {
		t.Errorf("dry-run does not report the new stakeholder:\n%s", dryOut)
	}
	if strings.Contains(dryOut, "stakeholder] fixture-owner") {
		t.Errorf("dry-run must not report the already-present stakeholder:\n%s", dryOut)
	}
	after, _ := os.ReadFile(fx.graphPath)
	if string(before) != string(after) {
		t.Fatal("dry-run wrote graph.json")
	}
	hash := extractDiffHash(t, dryOut)

	// The hash covers stakeholders: a different declared Name -> a different hash.
	fx.declareStakeholders(t, "Other Name")
	dryOut2, _, err := runSyncDomain(t, []string{"--domain", fx.domainDir})
	if err != nil {
		t.Fatalf("second dry-run: %v\n%s", err, dryOut2)
	}
	if extractDiffHash(t, dryOut2) == hash {
		t.Error("diff-hash did not change when the added stakeholder changed")
	}
	fx.declareStakeholders(t, "Code Owner")

	out, _, err := runSyncDomain(t, []string{"--domain", fx.domainDir, "--today", "2026-07-26", "--confirm-hash", hash})
	if err != nil {
		t.Fatalf("confirm: %v\n%s", err, out)
	}
	g, err := loader.LoadGraph(fx.graphPath)
	if err != nil {
		t.Fatal(err)
	}
	co, ok := stakeholderByID(g, "code-owner")
	if !ok || co.Name != "Code Owner" || co.Domain != "owns requirements declared in Go" {
		t.Errorf("code-owner not landed correctly: %+v ok=%v", co, ok)
	}
	fo, ok := stakeholderByID(g, "fixture-owner")
	if !ok || fo.Name != "Fixture Owner" {
		t.Errorf("existing stakeholder was rewritten: %+v ok=%v", fo, ok)
	}
	if len(g.Stakeholders) != 2 {
		t.Errorf("stakeholders = %d, want 2 (append-only)", len(g.Stakeholders))
	}
	vs, err := allViolations(fx.domainDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) > 0 {
		t.Errorf("expected 0 violations (owner must resolve), got %+v", vs)
	}
}

// A stakeholder-only change (requirements already in sync) is a real diff:
// confirm lands it instead of "nothing to sync".
func TestCmdSyncDomain_StakeholderOnlyChangeLands(t *testing.T) {
	if testing.Short() {
		t.Skip("spawns a real `go run` subprocess; skipped in -short")
	}
	fx := newSyncDomainFixture(t)
	dry, _, err := runSyncDomain(t, []string{"--domain", fx.domainDir})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := runSyncDomain(t, []string{"--domain", fx.domainDir, "--today", "2026-07-26", "--confirm-hash", extractDiffHash(t, dry)}); err != nil {
		t.Fatalf("seed sync: %v", err)
	}

	fx.declareStakeholders(t, "Code Owner")
	// declareStakeholders retargets the owner; the requirement now differs
	// too, so restore the original owner to isolate the stakeholder-only diff.
	reqPath := filepath.Join(fx.specDir, "requirements.go")
	b, _ := os.ReadFile(reqPath)
	if err := os.WriteFile(reqPath, []byte(strings.Replace(string(b), `"code-owner"`, `"fixture-owner"`, 1)), 0o644); err != nil {
		t.Fatal(err)
	}

	dry, _, err = runSyncDomain(t, []string{"--domain", fx.domainDir})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(dry, "no differences") || !strings.Contains(dry, "[ADDED stakeholder] code-owner") {
		t.Fatalf("stakeholder-only diff not reported:\n%s", dry)
	}
	if _, _, err := runSyncDomain(t, []string{"--domain", fx.domainDir, "--today", "2026-07-26", "--confirm-hash", extractDiffHash(t, dry)}); err != nil {
		t.Fatalf("stakeholder-only confirm: %v", err)
	}
	g, _ := loader.LoadGraph(fx.graphPath)
	if _, ok := stakeholderByID(g, "code-owner"); !ok {
		t.Error("code-owner not landed")
	}
}

// Backward compat: a requirements-only domain keeps the legacy bare-array
// registrydump, and a legacy array dump still syncs (stakeholders none).
func TestScaffoldRegistrydump_LegacyTemplateWithoutStakeholders(t *testing.T) {
	fx := newSyncDomainFixture(t)
	data, err := os.ReadFile(filepath.Join(fx.specDir, "registrydump", "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "Stakeholders") {
		t.Errorf("requirements-only domain got the envelope scaffold:\n%s", data)
	}
	fx.declareStakeholders(t, "Code Owner")
	data, _ = os.ReadFile(filepath.Join(fx.specDir, "registrydump", "main.go"))
	if !strings.Contains(string(data), "spec.Stakeholders.All()") {
		t.Errorf("domain declaring Stakeholders did not get the envelope scaffold:\n%s", data)
	}
}

// Scaffolding with Stakeholders declared but the Stakeholder mirror not
// vendored is a clear error, not a non-compiling stub.
func TestScaffoldRegistrydump_StakeholdersNeedVendoredMirror(t *testing.T) {
	fx := newSyncDomainFixture(t)
	src := strings.ReplaceAll(syncDomainStakeholdersGoTemplate, "{{MODULE}}", stakeholdersModule)
	if err := os.WriteFile(filepath.Join(fx.specDir, "stakeholders.go"), []byte(strings.ReplaceAll(src, "{{NAME}}", "x")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(fx.specDir, "hotamontology", "stakeholder.go")); err != nil {
		t.Fatal(err)
	}
	if _, err := scaffoldRegistrydump(fx.domainDir); err == nil || !strings.Contains(err.Error(), "vendor-ontology") {
		t.Fatalf("err = %v, want a vendor-ontology hint", err)
	}
}

// A malformed envelope with a duplicate stakeholder ID is an ordinary error.
func TestDomainDumpFromSubprocess_DuplicateStakeholderIsClearError(t *testing.T) {
	domainDir := t.TempDir()
	specDir := filepath.Join(domainDir, "spec")
	dumpDir := filepath.Join(specDir, "registrydump")
	if err := os.MkdirAll(dumpDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "go.mod"), []byte("module fixture-dupstk\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	js := `{"requirements":[],"stakeholders":[{"id":"s","name":"a"},{"id":"s","name":"b"}]}`
	mainSrc := "package main\n\nimport \"os\"\n\nfunc main() { os.Stdout.WriteString(`" + js + "`) }\n"
	if err := os.WriteFile(filepath.Join(dumpDir, "main.go"), []byte(mainSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err := domainDumpFromSubprocess(domainDir)
	if err == nil || !strings.Contains(err.Error(), "duplicate stakeholder") {
		t.Fatalf("err = %v, want duplicate stakeholder error", err)
	}
}
