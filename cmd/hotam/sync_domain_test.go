package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/selfspec"
)

// syncDomainRequirementsGoTemplate is the fixture's hand-authored
// spec/requirements.go — the domain-side Go registry `hotam sync-domain`
// reads through the registrydump subprocess bridge. It deliberately mirrors
// the shape R-domain-founded-in-wave-order step 6 (root CLAUDE.md) and
// internal/selfspec.Requirements' own registry declaration: a package-level
// var built with hotamontology.New[...]().MustRegister(...) calls.
//
// {{WHY}} is substituted with the fixture's chosen Why literal (task #390,
// W0.3): Why is a REAL field on the vendored hotamontology.Requirement mirror
// (internal/ontology/canon/requirement.go), so a fixture author can write it
// here exactly like Claim/Owner/Status — this is the regression surface for
// the bug the task fixed (Why used to be silently absent from the vendored
// struct, so no fixture, and no real consumer domain, could ever have
// expressed it here at all; the field simply did not compile).
const syncDomainRequirementsGoTemplate = `package spec

import hotamontology "{{MODULE}}/hotamontology"

var Requirements = hotamontology.New[hotamontology.Requirement]()

func init() {
	Requirements.MustRegister("R-fixture-one", hotamontology.Requirement{
		ID:             "R-fixture-one",
		Claim:          "the fixture component shall behave predictably",
		Owner:          "fixture-owner",
		Status:         "SETTLED",
		Why:            "{{WHY}}",
		Relations:      []hotamontology.Relation{},
		Assumptions:    []string{},
		Enforcement:    "PROSE",
		EnforcedBy:     []string{},
		Enforceability: "INHERENTLY_PROSE",
		MTag:           "",
		Summary:        "",
		CreatedAt:      "2026-01-01",
		SettledAt:      "2026-01-01",
		SourceRefs:     []string{},
		DeclOrder:      1,
	})
}
`

// syncDomainFixtureDefaultWhy is the Why value newSyncDomainFixture bakes in
// by default (used by every existing test that does not care about Why
// specifically) — a real, non-empty rationale string, deliberately NOT ""
// so a test asserting on it can distinguish "the registry's authored value
// landed" from "Why silently came through empty" (the exact failure mode
// the missing vendored field used to cause, see this file's WHY-round-trip
// tests below).
const syncDomainFixtureDefaultWhy = "the fixture component exists to prove the sync-domain module-boundary bridge end to end"

// syncDomainFixture is a hermetic, disk-backed consumer-domain fixture
// carrying everything `hotam sync-domain` needs: a project root (so
// repoRootForDomain's tier-1 resolves cleanly, mirroring
// copySelfDomainUnderRoot's own rationale), a minimal graph.json with one
// Stakeholder whose ID matches the fixture registry's Owner field (so a
// full write-and-verify round trip can reach 0 violations), and a real,
// compilable spec/ Go module: go.mod, vendored hotamontology (via
// vendorOntology, task #365's own writer), a hand-authored requirements.go,
// and a scaffolded registrydump/main.go (via scaffoldRegistrydump, this
// task's own writer) — proving the WHOLE module-boundary bridge end to end,
// not a mocked stand-in for it.
type syncDomainFixture struct {
	root      string
	domainDir string
	graphPath string
	specDir   string
}

func newSyncDomainFixture(t *testing.T) *syncDomainFixture {
	t.Helper()
	root, domainDir := initDomainUnderRoot(t, "fixture-consumer", "2026-07-26")

	g := &ontology.Graph{
		Stakeholders: []ontology.Stakeholder{
			{ID: "fixture-owner", Name: "Fixture Owner", DeclOrder: 1},
		},
	}
	graphPath := graphPathForDomain(domainDir)
	if err := loader.WriteGraph(graphPath, g); err != nil {
		t.Fatalf("seed graph with stakeholder: %v", err)
	}

	specDir := filepath.Join(domainDir, "spec")
	const modulePath = "hotamspec-fixture-consumer"
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("mkdir spec: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "go.mod"), []byte("module "+modulePath+"\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatalf("write spec/go.mod: %v", err)
	}

	if _, err := vendorOntology(domainDir); err != nil {
		t.Fatalf("vendorOntology: %v", err)
	}

	reqSrc := syncDomainRequirementsGoSource(modulePath, syncDomainFixtureDefaultWhy)
	if err := os.WriteFile(filepath.Join(specDir, "requirements.go"), []byte(reqSrc), 0o644); err != nil {
		t.Fatalf("write spec/requirements.go: %v", err)
	}

	if _, err := scaffoldRegistrydump(domainDir); err != nil {
		t.Fatalf("scaffoldRegistrydump: %v", err)
	}

	return &syncDomainFixture{root: root, domainDir: domainDir, graphPath: graphPath, specDir: specDir}
}

// syncDomainRequirementsGoSource fills in syncDomainRequirementsGoTemplate's
// {{MODULE}}/{{WHY}} placeholders.
func syncDomainRequirementsGoSource(modulePath, why string) string {
	src := strings.ReplaceAll(syncDomainRequirementsGoTemplate, "{{MODULE}}", modulePath)
	return strings.ReplaceAll(src, "{{WHY}}", why)
}

// rewriteRequirementWhy overwrites the fixture's spec/requirements.go with a
// NEW Why value for R-fixture-one, everything else unchanged — used to drive
// a second sync-domain pass down the CHANGED path (R-fixture-one already
// exists in the graph from a prior sync; only its Why now disagrees with the
// registry).
func (fx *syncDomainFixture) rewriteRequirementWhy(t *testing.T, why string) {
	t.Helper()
	const modulePath = "hotamspec-fixture-consumer"
	reqSrc := syncDomainRequirementsGoSource(modulePath, why)
	if err := os.WriteFile(filepath.Join(fx.specDir, "requirements.go"), []byte(reqSrc), 0o644); err != nil {
		t.Fatalf("rewrite spec/requirements.go with new Why: %v", err)
	}
}

// runSyncDomain runs cmdSyncDomain in-process, capturing stdout/stderr —
// mirrors sync_self_test.go's runSyncSelf helper exactly (same os.Pipe
// capture mechanism, same non-parallel-with-itself caveat).
func runSyncDomain(t *testing.T, args []string) (stdout, stderr string, err error) {
	t.Helper()

	outR, outW, perr := os.Pipe()
	if perr != nil {
		t.Fatalf("pipe: %v", perr)
	}
	errR, errW, perr := os.Pipe()
	if perr != nil {
		t.Fatalf("pipe: %v", perr)
	}
	prevOut, prevErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outW, errW

	done := make(chan struct{})
	var gotOut, gotErr []byte
	go func() {
		gotOut, _ = readAllSync(outR)
		close(done)
	}()
	doneErr := make(chan struct{})
	go func() {
		gotErr, _ = readAllSync(errR)
		close(doneErr)
	}()

	err = cmdSyncDomain(args)

	os.Stdout, os.Stderr = prevOut, prevErr
	outW.Close()
	errW.Close()
	<-done
	<-doneErr

	return string(gotOut), string(gotErr), err
}

// TestCmdSyncDomain_DryRunNeverWrites proves a dry-run against a fresh
// consumer-domain fixture (registry entry ADDED, graph starts with none of
// it) prints the SyncReport + diff-hash and leaves graph.json byte-identical
// to what was on disk before the call.
func TestCmdSyncDomain_DryRunNeverWrites(t *testing.T) {
	fx := newSyncDomainFixture(t)
	before, err := os.ReadFile(fx.graphPath)
	if err != nil {
		t.Fatalf("read pre-run graph: %v", err)
	}
	// newSyncDomainFixture itself seeds graph.json via loader.WriteGraph
	// (to add the fixture Stakeholder) — WriteGraph always also (re)writes
	// graph.lock as a side effect (internal/loader/loader.go), so graph.lock
	// already exists BEFORE sync-domain ever runs. The dry-run contract this
	// test proves is therefore "leaves graph.lock exactly as it already was",
	// not "graph.lock is absent" (unlike sync-self's fixture, which never
	// calls WriteGraph itself and so starts genuinely lock-free).
	beforeLock, err := os.ReadFile(loader.LockPath(fx.graphPath))
	if err != nil {
		t.Fatalf("read pre-run graph.lock: %v", err)
	}

	stdout, _, err := runSyncDomain(t, []string{"--domain", fx.domainDir})
	if err != nil {
		t.Fatalf("dry-run sync-domain must succeed, got: %v\nstdout:\n%s", err, stdout)
	}
	if !strings.Contains(stdout, "DRY RUN") {
		t.Errorf("stdout missing DRY RUN banner:\n%s", stdout)
	}
	if !strings.Contains(stdout, "[ADDED] R-fixture-one") {
		t.Errorf("stdout does not show the ADDED fixture requirement:\n%s", stdout)
	}
	if !strings.Contains(stdout, "diff-hash:") {
		t.Errorf("stdout missing diff-hash line:\n%s", stdout)
	}

	after, err := os.ReadFile(fx.graphPath)
	if err != nil {
		t.Fatalf("read post-run graph: %v", err)
	}
	if string(before) != string(after) {
		t.Fatal("graph.json was modified by a dry-run — dry-run must NEVER write to disk")
	}
	afterLock, err := os.ReadFile(loader.LockPath(fx.graphPath))
	if err != nil {
		t.Fatalf("read post-run graph.lock: %v", err)
	}
	if string(beforeLock) != string(afterLock) {
		t.Error("graph.lock was modified by a dry-run — dry-run must NEVER write to disk")
	}
}

// TestCmdSyncDomain_HashMismatchRefusesAndDoesNotWrite mirrors
// TestCmdSyncSelf_HashMismatchRefusesAndDoesNotWrite: a stale/incorrect
// --confirm-hash must refuse and leave the graph untouched.
func TestCmdSyncDomain_HashMismatchRefusesAndDoesNotWrite(t *testing.T) {
	fx := newSyncDomainFixture(t)
	before, err := os.ReadFile(fx.graphPath)
	if err != nil {
		t.Fatalf("read pre-run graph: %v", err)
	}

	_, _, err = runSyncDomain(t, []string{
		"--domain", fx.domainDir, "--today", "2026-07-26",
		"--confirm-hash", "0000000000000000000000000000000000000000000000000000000000000000",
	})
	if err == nil {
		t.Fatal("expected an error for a wrong --confirm-hash, got nil")
	}
	if !strings.Contains(err.Error(), "diff changed since PRESENT") {
		t.Errorf("error = %q, want it to mention 'diff changed since PRESENT'", err.Error())
	}

	after, err := os.ReadFile(fx.graphPath)
	if err != nil {
		t.Fatalf("read post-run graph: %v", err)
	}
	if string(before) != string(after) {
		t.Fatal("graph.json was modified despite a hash mismatch")
	}
}

// TestCmdSyncDomain_FullRoundTrip is the primary e2e scenario: dry-run ->
// extract hash -> confirm-hash -> the graph is really written with the
// expected ADDED entry, docs regenerated, and all-violations clean
// afterward — proving the full registrydump subprocess bridge (a REAL `go
// run ./registrydump` inside the fixture's own spec/ module) end to end.
func TestCmdSyncDomain_FullRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("sync-domain full round trip spawns a real `go run` subprocess; skipped in -short")
	}
	fx := newSyncDomainFixture(t)

	dryOut, _, err := runSyncDomain(t, []string{"--domain", fx.domainDir})
	if err != nil {
		t.Fatalf("dry-run failed: %v\n%s", err, dryOut)
	}
	hash := extractDiffHash(t, dryOut)

	confirmOut, _, err := runSyncDomain(t, []string{
		"--domain", fx.domainDir, "--today", "2026-07-26",
		"--confirm-hash", hash,
	})
	if err != nil {
		t.Fatalf("confirm-hash run failed: %v\n%s", err, confirmOut)
	}
	if !strings.Contains(confirmOut, "sync-domain landed") {
		t.Errorf("stdout missing the landed confirmation:\n%s", confirmOut)
	}

	g, err := loader.LoadGraph(fx.graphPath)
	if err != nil {
		t.Fatalf("reload written graph: %v", err)
	}
	var found bool
	for _, r := range g.Requirements {
		if r.ID != "R-fixture-one" {
			continue
		}
		found = true
		if r.Claim != "the fixture component shall behave predictably" {
			t.Errorf("requirement Claim = %q, want the registry's fixture claim", r.Claim)
		}
		// Task #390 (W0.3) regression check: Why is a real field on the
		// vendored hotamontology.Requirement mirror now, so the registry's
		// authored Why value must flow all the way through registrydump's
		// JSON marshal -> domainRegistryFromSubprocess's unmarshal into
		// ontology.Requirement -> SyncGraph -> graph.json, landing exactly as
		// authored — NOT silently dropped to "" (the bug this task fixed: the
		// vendored struct used to have no Why field at all, so this value
		// could never have been expressed in spec/requirements.go in the
		// first place).
		if r.Why != syncDomainFixtureDefaultWhy {
			t.Errorf("requirement Why = %q, want the registry's authored Why %q — Why must not be silently dropped by the vendored ontology mirror", r.Why, syncDomainFixtureDefaultWhy)
		}
		if len(r.History) != 1 || !strings.Contains(r.History[0].Summary, "created via sync-self") {
			t.Errorf("requirement History = %+v, want a single 'created via sync-self' seed entry", r.History)
		}
	}
	if !found {
		t.Fatalf("R-fixture-one was not created by sync-domain")
	}

	violations, err := allViolations(fx.domainDir)
	if err != nil {
		t.Fatalf("allViolations: %v", err)
	}
	if len(violations) > 0 {
		t.Errorf("expected 0 violations after a clean sync-domain round-trip, got %d: %+v", len(violations), violations)
	}

	lockData, err := os.ReadFile(loader.LockPath(fx.graphPath))
	if err != nil {
		t.Fatalf("read graph.lock: %v", err)
	}
	if !strings.Contains(string(lockData), "sync-domain") {
		t.Errorf("graph.lock note does not mention sync-domain:\n%s", lockData)
	}
}

// TestCmdSyncDomain_WhyRoundTripSurvivesChangedSync is task #390's (W0.3)
// dedicated regression test: it reproduces exactly the data-loss scenario
// the resolver flagged. Before this task's fix,
// internal/ontology/canon/requirement.go (the vendored Requirement mirror a
// consumer domain's spec/ module actually compiles against) had NO Why
// field at all, so a domain that had a hand-written Why on a requirement
// BEFORE adopting requirements_authority:"code" could never express that
// Why in spec/requirements.go — the type simply had no such field — and the
// very first `hotam sync-domain` would silently overwrite the graph's Why
// with the zero value "" (Why is a STRUCTURAL field per
// internal/selfspec/merge.go's MergeIntoGraph/SyncGraph, wholesale-replaced
// from the registry on every sync, so there was no way for the old
// hand-written value to survive).
//
// This test proves the fixed round trip end to end, through the REAL
// module-boundary bridge (registrydump subprocess, not an in-process
// shortcut): sync-domain CREATES R-fixture-one with an explicit, non-empty
// Why (simulating a domain author who, now that the field compiles, writes
// a real rationale from day one); a second sync-domain pass — after the
// fixture's spec/requirements.go is rewritten with a DIFFERENT Why, the
// same way a domain author would edit their own authored rationale — must
// carry that NEW Why into the already-existing graph node (the CHANGED
// path, StructuralFieldDiffs/SyncGraph), never falling back to "" and never
// silently keeping the stale first value.
func TestCmdSyncDomain_WhyRoundTripSurvivesChangedSync(t *testing.T) {
	if testing.Short() {
		t.Skip("sync-domain full round trip spawns a real `go run` subprocess; skipped in -short")
	}
	fx := newSyncDomainFixture(t)

	// --- Pass 1: ADDED, with the fixture's default (non-empty) Why. ---
	dryOut1, _, err := runSyncDomain(t, []string{"--domain", fx.domainDir})
	if err != nil {
		t.Fatalf("pass 1 dry-run failed: %v\n%s", err, dryOut1)
	}
	hash1 := extractDiffHash(t, dryOut1)
	if _, _, err := runSyncDomain(t, []string{
		"--domain", fx.domainDir, "--today", "2026-07-26",
		"--confirm-hash", hash1,
	}); err != nil {
		t.Fatalf("pass 1 confirm-hash run failed: %v", err)
	}

	g1, err := loader.LoadGraph(fx.graphPath)
	if err != nil {
		t.Fatalf("reload graph after pass 1: %v", err)
	}
	if why := whyOf(t, g1, "R-fixture-one"); why != syncDomainFixtureDefaultWhy {
		t.Fatalf("after pass 1 (ADDED): Why = %q, want the registry's authored default %q", why, syncDomainFixtureDefaultWhy)
	}

	// --- Pass 2: CHANGED — rewrite the registry's Why to a NEW value and
	// sync again. This is the scenario that used to silently lose data: a
	// second sync must carry the NEW authored Why through, not reset it to
	// "" and not leave the stale pass-1 value in place.
	const updatedWhy = "updated rationale: the fixture component now also guards against concurrent writers"
	fx.rewriteRequirementWhy(t, updatedWhy)

	dryOut2, _, err := runSyncDomain(t, []string{"--domain", fx.domainDir})
	if err != nil {
		t.Fatalf("pass 2 dry-run failed: %v\n%s", err, dryOut2)
	}
	if !strings.Contains(dryOut2, "[CHANGED] R-fixture-one") {
		t.Errorf("pass 2 dry-run does not show R-fixture-one as CHANGED (Why must be detected as a structural-field diff):\n%s", dryOut2)
	}
	if !strings.Contains(dryOut2, "field Why:") {
		t.Errorf("pass 2 dry-run field diff does not mention Why:\n%s", dryOut2)
	}
	hash2 := extractDiffHash(t, dryOut2)

	confirmOut2, _, err := runSyncDomain(t, []string{
		"--domain", fx.domainDir, "--today", "2026-07-27",
		"--confirm-hash", hash2,
	})
	if err != nil {
		t.Fatalf("pass 2 confirm-hash run failed: %v\n%s", err, confirmOut2)
	}

	g2, err := loader.LoadGraph(fx.graphPath)
	if err != nil {
		t.Fatalf("reload graph after pass 2: %v", err)
	}
	if why := whyOf(t, g2, "R-fixture-one"); why != updatedWhy {
		t.Fatalf("after pass 2 (CHANGED): Why = %q, want the newly-authored value %q — Why must not be silently dropped to \"\" or left stale", why, updatedWhy)
	}

	violations, err := allViolations(fx.domainDir)
	if err != nil {
		t.Fatalf("allViolations: %v", err)
	}
	if len(violations) > 0 {
		t.Errorf("expected 0 violations after the Why-changed sync-domain round-trip, got %d: %+v", len(violations), violations)
	}
}

// whyOf returns the Why field of the requirement named id in g, failing the
// test if id is not found.
func whyOf(t *testing.T, g *ontology.Graph, id string) string {
	t.Helper()
	for _, r := range g.Requirements {
		if r.ID == id {
			return r.Why
		}
	}
	t.Fatalf("requirement %s not found in graph", id)
	return ""
}

// TestCmdSyncDomain_MissingSpecDirIsClearError proves gate 0's first failure
// mode: a domain with no spec/ tree at all gets a specific, actionable error
// naming what's missing — never a silent empty-registry "nothing to sync".
func TestCmdSyncDomain_MissingSpecDirIsClearError(t *testing.T) {
	_, domainDir := initDomainUnderRoot(t, "fixture-no-spec", "2026-07-26")

	_, _, err := runSyncDomain(t, []string{"--domain", domainDir})
	if err == nil {
		t.Fatal("expected an error for a domain with no spec/ tree, got nil")
	}
	if !strings.Contains(err.Error(), "spec") {
		t.Errorf("error = %q, want it to mention the missing spec/ tree", err.Error())
	}
}

// TestCmdSyncDomain_MissingRegistrydumpIsClearError proves gate 0's second
// failure mode: a spec/ module that exists but has no registrydump/ program
// yet (vendor-ontology ran, scaffold-registrydump did not) gets a specific
// error naming the missing scaffold step.
func TestCmdSyncDomain_MissingRegistrydumpIsClearError(t *testing.T) {
	_, domainDir := initDomainUnderRoot(t, "fixture-no-dump", "2026-07-26")
	specDir := filepath.Join(domainDir, "spec")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("mkdir spec: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "go.mod"), []byte("module fixture-no-dump\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	_, _, err := runSyncDomain(t, []string{"--domain", domainDir})
	if err == nil {
		t.Fatal("expected an error for a spec/ module with no registrydump/, got nil")
	}
	if !strings.Contains(err.Error(), "scaffold-registrydump") {
		t.Errorf("error = %q, want it to mention `hotam scaffold-registrydump`", err.Error())
	}
}

// TestCmdSyncDomain_BrokenRegistrydumpSubprocessSurfacesRealError proves a
// registrydump program that fails to compile/run surfaces a specific,
// diagnosable error — not a silent empty merge. The fixture's
// spec/requirements.go is corrupted with a syntax error AFTER the fixture
// was otherwise fully scaffolded (compilable), so runRegistryDump's `go run`
// subprocess genuinely fails.
func TestCmdSyncDomain_BrokenRegistrydumpSubprocessSurfacesRealError(t *testing.T) {
	if testing.Short() {
		t.Skip("spawns a real `go run` subprocess; skipped in -short")
	}
	fx := newSyncDomainFixture(t)
	reqPath := filepath.Join(fx.specDir, "requirements.go")
	if err := os.WriteFile(reqPath, []byte("package spec\n\nthis is not valid go syntax {{{\n"), 0o644); err != nil {
		t.Fatalf("corrupt requirements.go: %v", err)
	}

	_, _, err := runSyncDomain(t, []string{"--domain", fx.domainDir})
	if err == nil {
		t.Fatal("expected an error when registrydump fails to compile, got nil")
	}
	if !strings.Contains(err.Error(), "registry dump") && !strings.Contains(err.Error(), "registrydump") {
		t.Errorf("error = %q, want it to name the registry-dump step", err.Error())
	}
}

// TestCmdSyncDomain_SubprocessSmoke is a light real-binary smoke test,
// mirroring TestCmdSyncSelf_SubprocessSmoke: builds the shared hotam binary
// and runs a dry-run against the fixture as a real child process, proving
// the "sync-domain" wiring in main.go is correct end-to-end.
func TestCmdSyncDomain_SubprocessSmoke(t *testing.T) {
	if testing.Short() {
		t.Skip("sync-domain subprocess smoke: builds a real binary + spawns child processes; skipped in -short")
	}
	t.Parallel()

	binPath := buildSharedHotamBinary(t)
	fx := newSyncDomainFixture(t)

	out, err := exec.Command(binPath, "sync-domain", "--domain", fx.domainDir).CombinedOutput()
	if err != nil {
		t.Fatalf("hotam sync-domain (dry-run) subprocess failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "diff-hash:") {
		t.Errorf("subprocess output missing diff-hash line:\n%s", out)
	}
}

// --- unit-level: domainRegistryFromSubprocess / JSON round trip ---

// TestDomainRegistryFromSubprocess_RealFixture proves gate 0's happy path
// directly: given a fully scaffolded fixture, domainRegistryFromSubprocess
// returns a registry containing exactly the fixture's one entry, with the
// exact structural field values the fixture's spec/requirements.go declared.
func TestDomainRegistryFromSubprocess_RealFixture(t *testing.T) {
	if testing.Short() {
		t.Skip("spawns a real `go run` subprocess; skipped in -short")
	}
	fx := newSyncDomainFixture(t)

	reg, err := domainRegistryFromSubprocess(fx.domainDir)
	if err != nil {
		t.Fatalf("domainRegistryFromSubprocess: %v", err)
	}
	all := reg.All()
	if len(all) != 1 {
		t.Fatalf("registry has %d entries, want exactly 1: %+v", len(all), all)
	}
	if all[0].ID != "R-fixture-one" {
		t.Errorf("entry ID = %q, want R-fixture-one", all[0].ID)
	}
	if all[0].Claim != "the fixture component shall behave predictably" {
		t.Errorf("entry Claim = %q, want the fixture claim", all[0].Claim)
	}
	if all[0].Owner != "fixture-owner" {
		t.Errorf("entry Owner = %q, want fixture-owner", all[0].Owner)
	}
}

// TestDomainRegistryFromSubprocess_EmptyRegistryStillSucceeds proves an
// EMPTY (but well-formed, valid JSON "[]") registrydump output is not
// confused with a subprocess FAILURE — json.Marshal(nil slice) legitimately
// prints "null", and both "null" and "[]" must decode to a zero-entry
// registry without error, distinctly from the actual-failure paths (missing
// spec/, missing registrydump/, non-zero exit, invalid JSON) covered above.
func TestDomainRegistryFromSubprocess_EmptyRegistryStillSucceeds(t *testing.T) {
	domainDir := t.TempDir()
	specDir := filepath.Join(domainDir, "spec")
	dumpDir := filepath.Join(specDir, "registrydump")
	if err := os.MkdirAll(dumpDir, 0o755); err != nil {
		t.Fatalf("mkdir registrydump: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "go.mod"), []byte("module fixture-empty\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dumpDir, "main.go"), []byte(`package main

import "os"

func main() { os.Stdout.WriteString("null") }
`), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	reg, err := domainRegistryFromSubprocess(domainDir)
	if err != nil {
		t.Fatalf("domainRegistryFromSubprocess: %v", err)
	}
	if len(reg.All()) != 0 {
		t.Errorf("registry has %d entries, want 0", len(reg.All()))
	}
}

// TestDomainRegistryFromSubprocess_InvalidJSONIsClearError proves a
// registrydump program that runs successfully (exit 0) but prints
// non-JSON garbage is a clear, specific error, not a silent empty merge.
func TestDomainRegistryFromSubprocess_InvalidJSONIsClearError(t *testing.T) {
	domainDir := t.TempDir()
	specDir := filepath.Join(domainDir, "spec")
	dumpDir := filepath.Join(specDir, "registrydump")
	if err := os.MkdirAll(dumpDir, 0o755); err != nil {
		t.Fatalf("mkdir registrydump: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "go.mod"), []byte("module fixture-badjson\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dumpDir, "main.go"), []byte(`package main

import "os"

func main() { os.Stdout.WriteString("this is not json") }
`), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	_, err := domainRegistryFromSubprocess(domainDir)
	if err == nil {
		t.Fatal("expected an error for non-JSON registrydump output, got nil")
	}
	if !strings.Contains(err.Error(), "not a valid JSON") {
		t.Errorf("error = %q, want it to mention invalid JSON", err.Error())
	}
}

// TestDomainRegistryFromSubprocess_SubprocessExitFailureIsClearError proves
// a registrydump program that exits non-zero surfaces a specific,
// diagnosable error naming the failure — not an empty registry.
func TestDomainRegistryFromSubprocess_SubprocessExitFailureIsClearError(t *testing.T) {
	domainDir := t.TempDir()
	specDir := filepath.Join(domainDir, "spec")
	dumpDir := filepath.Join(specDir, "registrydump")
	if err := os.MkdirAll(dumpDir, 0o755); err != nil {
		t.Fatalf("mkdir registrydump: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "go.mod"), []byte("module fixture-exitfail\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dumpDir, "main.go"), []byte(`package main

import "os"

func main() { os.Exit(1) }
`), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	_, err := domainRegistryFromSubprocess(domainDir)
	if err == nil {
		t.Fatal("expected an error for a non-zero registrydump exit, got nil")
	}
	if !strings.Contains(err.Error(), "registry dump") {
		t.Errorf("error = %q, want it to name the registry-dump step", err.Error())
	}
}

// TestDomainRegistryFromSubprocess_DuplicateIDIsClearError proves a
// registrydump program that (incorrectly) emits two entries with the same ID
// surfaces MustRegister's panic as a normal error path, not a crash — this
// exercises domainRegistryFromSubprocess's own reg.MustRegister loop
// directly via a hand-crafted duplicate-ID JSON array (a real registrydump
// built from a real hotamontology.Registry can never itself emit a
// duplicate, since MustRegister panics at REGISTRATION time inside the
// subprocess — so this simulates a hypothetically malformed dump instead of
// depending on an unreachable subprocess state).
func TestDomainRegistryFromSubprocess_DuplicateIDIsClearError(t *testing.T) {
	domainDir := t.TempDir()
	specDir := filepath.Join(domainDir, "spec")
	dumpDir := filepath.Join(specDir, "registrydump")
	if err := os.MkdirAll(dumpDir, 0o755); err != nil {
		t.Fatalf("mkdir registrydump: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "go.mod"), []byte("module fixture-dup\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	dupJSON := `[{"id":"R-dup","claim":"a"},{"id":"R-dup","claim":"b"}]`
	mainSrc := "package main\n\nimport \"os\"\n\nfunc main() { os.Stdout.WriteString(`" + dupJSON + "`) }\n"
	if err := os.WriteFile(filepath.Join(dumpDir, "main.go"), []byte(mainSrc), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("domainRegistryFromSubprocess must return an error, not panic, on a duplicate ID: %v", r)
		}
	}()
	_, err := domainRegistryFromSubprocess(domainDir)
	if err == nil {
		t.Fatal("expected an error for a duplicate-ID registrydump payload, got nil")
	}
}

// TestSyncDomainResult_JSONShape is a light structural check that
// newSyncDomainResult's JSON envelope round-trips through encoding/json —
// primarily a compile-time/shape guard for the --json flag.
func TestSyncDomainResult_JSONShape(t *testing.T) {
	report := &selfspec.SyncReport{Entries: []selfspec.SyncReportEntry{{ID: "R-fixture-one", Kind: selfspec.SyncKindAdded}}}
	res := newSyncDomainResult(true, report, "deadbeef", nil, nil, nil, nil)
	data, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal syncDomainResult: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal syncDomainResult: %v", err)
	}
	if decoded["diff_hash"] != "deadbeef" {
		t.Errorf("decoded diff_hash = %v, want deadbeef", decoded["diff_hash"])
	}
}
