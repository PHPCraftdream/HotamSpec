package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
)

// TestInitDomain_ScaffoldsGenuinelyEmptyGraph proves task #364's core change
// directly at the unit level: initDomain no longer auto-seeds a Stakeholder
// ("owner") or a Requirement ("R-domain-exists") — a freshly-scaffolded
// domain is genuinely empty (0 nodes of every kind), which is well-formed by
// construction (R-empty-content-wellformed) and needs no worked example to
// pass `all-violations` clean. This replaces the old
// TestInitDomain_SeedRequirementIsFresh/SeedReviewAfterBeyondDueSoonWindow,
// which asserted freshness properties of a seed Requirement that no longer
// exists.
func TestInitDomain_ScaffoldsGenuinelyEmptyGraph(t *testing.T) {
	if testing.Short() {
		t.Skip("end-to-end land/apply/gen-spec flow (15-45s); skipped in -short, covered by the full run")
	}
	t.Parallel()

	domainDir := t.TempDir()
	today := "2026-07-15"
	if _, err := initDomain(domainDir, "test-empty-scaffold", today); err != nil {
		t.Fatalf("initDomain: %v", err)
	}

	g, err := loadDomainGraph(domainDir)
	if err != nil {
		t.Fatalf("loadDomainGraph: %v", err)
	}
	if !g.IsEmpty() {
		t.Errorf("freshly-scaffolded graph is not empty: %+v", g)
	}
	if len(g.Stakeholders) != 0 {
		t.Errorf("freshly-scaffolded graph has %d stakeholder(s), want 0 (no auto-seed)", len(g.Stakeholders))
	}
	if len(g.Requirements) != 0 {
		t.Errorf("freshly-scaffolded graph has %d requirement(s), want 0 (no auto-seed)", len(g.Requirements))
	}

	violations, err := allViolations(domainDir)
	if err != nil {
		t.Fatalf("allViolations: %v", err)
	}
	if len(violations) != 0 {
		t.Errorf("freshly-scaffolded empty domain has %d violation(s), want 0: %+v", len(violations), violations)
	}
}

// TestCmdInit_TodayFlagAcceptedWithoutError proves `hotam init --today
// <date>` (bare, not init-project) still parses and does not error now that
// initDomain no longer seeds anything the flag would date-stamp (task #364
// retired the seed Requirement --today used to feed LastReviewedAt/
// ReviewAfter). The flag is kept for API/call-site stability (see
// initDomain's own doc comment) and must remain harmless to pass.
func TestCmdInit_TodayFlagAcceptedWithoutError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	domainDir := filepath.Join(dir, "pinned-domain")
	pinnedToday := "2026-01-01"

	if err := cmdInit([]string{"--today", pinnedToday, domainDir}); err != nil {
		t.Fatalf("cmdInit --today %q: %v", pinnedToday, err)
	}

	g, err := loadDomainGraph(domainDir)
	if err != nil {
		t.Fatalf("loadDomainGraph: %v", err)
	}
	if !g.IsEmpty() {
		t.Errorf("cmdInit --today %q produced a non-empty graph: %+v", pinnedToday, g)
	}
}

// TestCmdInit_TodayFlagDefaultsToSystemDate confirms omitting --today still
// works (defaults to system date, exactly like init-project's own
// long-standing --today convention) — the flag is additive, not a new
// required argument, and remains harmless to omit now that nothing in the
// scaffolded graph is date-stamped by it.
func TestCmdInit_TodayFlagDefaultsToSystemDate(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	domainDir := filepath.Join(dir, "default-today-domain")

	if err := cmdInit([]string{domainDir}); err != nil {
		t.Fatalf("cmdInit without --today: %v", err)
	}

	g, err := loadDomainGraph(domainDir)
	if err != nil {
		t.Fatalf("loadDomainGraph: %v", err)
	}
	if !g.IsEmpty() {
		t.Errorf("cmdInit (default --today) produced a non-empty graph: %+v", g)
	}
}

// readManifestRequireProvenance reads a domain's manifest.json and returns
// its raw require_provenance value directly (independent of
// loader.ResolveRequireProvenance) so tests can assert on the literal
// on-disk JSON shape, not just the loader's resolved reading of it.
func readManifestRequireProvenance(t *testing.T, manifestPath string) (value bool, present bool) {
	t.Helper()
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest %s: %v", manifestPath, err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal manifest %s: %v", manifestPath, err)
	}
	rv, ok := raw["require_provenance"]
	if !ok {
		return false, false
	}
	var b bool
	if err := json.Unmarshal(rv, &b); err != nil {
		t.Fatalf("unmarshal require_provenance in %s: %v", manifestPath, err)
	}
	return b, true
}

// TestCmdInit_RequireProvenanceFlag proves R12-b: `hotam init
// --require-provenance` writes "require_provenance": true into manifest.json
// (closing the onboarding gap where a business adopter would otherwise have
// to hand-edit the manifest right after scaffolding), and that
// loader.ResolveRequireProvenance reads that value back as true — the same
// function cmd/hotam/provenance_gate.go consults at land time.
func TestCmdInit_RequireProvenanceFlag(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	domainDir := filepath.Join(dir, "provenance-domain")
	if err := cmdInit([]string{"--require-provenance", domainDir}); err != nil {
		t.Fatalf("cmdInit --require-provenance: %v", err)
	}

	manifestPath := filepath.Join(domainDir, "manifest.json")
	got, present := readManifestRequireProvenance(t, manifestPath)
	if !present {
		t.Fatalf("manifest.json has no require_provenance field after --require-provenance")
	}
	if !got {
		t.Errorf("manifest.json require_provenance = false, want true")
	}

	if resolved := loader.ResolveRequireProvenance(graphPathForDomain(domainDir)); !resolved {
		t.Errorf("loader.ResolveRequireProvenance = false after --require-provenance, want true")
	}
}

// TestCmdInit_ProfileFullAndRequireProvenanceCombine is the regression test
// for the exact landmine named in R12-b: `hotam init --profile full
// --require-provenance` must produce a manifest with BOTH gen_profile=full
// AND require_provenance=true. Before this fix, --profile full's manifest
// write was a blind full-file overwrite that would have silently discarded
// require_provenance if the two flags' writes were layered independently.
func TestCmdInit_ProfileFullAndRequireProvenanceCombine(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	domainDir := filepath.Join(dir, "combo-domain")
	if err := cmdInit([]string{"--profile", "full", "--require-provenance", domainDir}); err != nil {
		t.Fatalf("cmdInit --profile full --require-provenance: %v", err)
	}

	manifestPath := filepath.Join(domainDir, "manifest.json")
	if gotProfile := readManifestGenProfile(t, manifestPath); gotProfile != "full" {
		t.Errorf("combined flags: manifest gen_profile = %q, want \"full\" (--profile full must survive --require-provenance's write)", gotProfile)
	}
	gotProvenance, present := readManifestRequireProvenance(t, manifestPath)
	if !present || !gotProvenance {
		t.Errorf("combined flags: manifest require_provenance present=%v value=%v, want present=true value=true (--require-provenance must survive --profile full's write)", present, gotProvenance)
	}
}

// TestCmdInit_RequireProvenanceDefaultOff confirms omitting --require-provenance
// leaves the manifest exactly as before this task: no require_provenance
// field at all — zero behavior change for the existing default onboarding
// path (matching --profile's own backward-compatibility bar, task #148).
func TestCmdInit_RequireProvenanceDefaultOff(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	domainDir := filepath.Join(dir, "default-domain")
	if err := cmdInit([]string{domainDir}); err != nil {
		t.Fatalf("cmdInit (default): %v", err)
	}

	manifestPath := filepath.Join(domainDir, "manifest.json")
	if _, present := readManifestRequireProvenance(t, manifestPath); present {
		t.Errorf("default cmdInit (no --require-provenance) wrote a require_provenance field; want it absent")
	}
	if resolved := loader.ResolveRequireProvenance(graphPathForDomain(domainDir)); resolved {
		t.Errorf("loader.ResolveRequireProvenance = true without --require-provenance, want false")
	}
}

// readManifestParent reads a domain's manifest.json and returns its "parent"
// value plus whether the key is present at all, distinguishing JSON null
// (present=true, value="") from a genuinely absent key (present=false) —
// exactly the triple loader.ResolveParent/g.ParentDeclared/g.Parent resolve,
// but read directly off disk here so the test asserts on the literal
// scaffolded bytes, not just the loader's own reading of them.
func readManifestParent(t *testing.T, manifestPath string) (value string, present bool) {
	t.Helper()
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest %s: %v", manifestPath, err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal manifest %s: %v", manifestPath, err)
	}
	pv, ok := raw["parent"]
	if !ok {
		return "", false
	}
	if string(pv) == "null" {
		return "", true
	}
	var s string
	if err := json.Unmarshal(pv, &s); err != nil {
		t.Fatalf("unmarshal parent in %s: %v", manifestPath, err)
	}
	return s, true
}

// TestCmdInit_DefaultsToRootParentAndPassesAllViolations proves the core W6.2
// promise for the SIMPLE case: `hotam init <dir>` (bare, no init-project, no
// --parent flag) scaffolds a manifest.json carrying "parent": null (the D6
// EXPLICIT root-domain declaration — see internal/invariants/
// project_parent.go's checkProjectParentDeclared), and a subsequent
// `all-violations` run against that freshly-scaffolded domain is 0
// violations — proving check_project_parent_declared (and every other
// invariant) is satisfied from birth, with no migration window, even for a
// domain scaffolded by the simplest possible onboarding command.
func TestCmdInit_DefaultsToRootParentAndPassesAllViolations(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	domainDir := filepath.Join(dir, "root-domain")
	if err := cmdInit([]string{domainDir}); err != nil {
		t.Fatalf("cmdInit (default): %v", err)
	}

	manifestPath := filepath.Join(domainDir, "manifest.json")
	value, present := readManifestParent(t, manifestPath)
	if !present {
		t.Fatalf("manifest.json has no \"parent\" key after bare `hotam init`; want \"parent\": null")
	}
	if value != "" {
		t.Errorf("manifest.json \"parent\" = %q, want JSON null (root-domain declaration)", value)
	}

	violations, err := allViolations(domainDir)
	if err != nil {
		t.Fatalf("allViolations: %v", err)
	}
	if len(violations) != 0 {
		t.Errorf("freshly-scaffolded `hotam init` domain has %d violation(s), want 0: %+v", len(violations), violations)
	}
}

// TestCmdInit_ParentFlagWritesChildDeclaration proves `hotam init --parent
// <name>` overrides initDomain's root-domain default with a child
// declaration ("parent": "<name>"), composed correctly alongside the other
// manifest-affecting flags (--profile, --require-provenance) so no flag's
// write silently discards another's — mirroring
// TestCmdInit_ProfileFullAndRequireProvenanceCombine's own composition
// proof.
func TestCmdInit_ParentFlagWritesChildDeclaration(t *testing.T) {
	t.Parallel()

	// --parent alone.
	dir := t.TempDir()
	domainDir := filepath.Join(dir, "child-domain")
	if err := cmdInit([]string{"--parent", "some-name", domainDir}); err != nil {
		t.Fatalf("cmdInit --parent some-name: %v", err)
	}
	manifestPath := filepath.Join(domainDir, "manifest.json")
	value, present := readManifestParent(t, manifestPath)
	if !present {
		t.Fatalf("manifest.json has no \"parent\" key after --parent some-name")
	}
	if value != "some-name" {
		t.Errorf("manifest.json \"parent\" = %q, want \"some-name\"", value)
	}

	violations, err := allViolations(domainDir)
	if err != nil {
		t.Fatalf("allViolations: %v", err)
	}
	if len(violations) != 0 {
		t.Errorf("--parent-scaffolded domain has %d violation(s), want 0: %+v", len(violations), violations)
	}

	// --parent composed with --profile full and --require-provenance: none
	// of the three flags' writes may clobber another's.
	comboDir := t.TempDir()
	comboDomain := filepath.Join(comboDir, "combo-domain")
	if err := cmdInit([]string{"--parent", "combo-parent", "--profile", "full", "--require-provenance", comboDomain}); err != nil {
		t.Fatalf("cmdInit --parent --profile full --require-provenance: %v", err)
	}
	comboManifest := filepath.Join(comboDomain, "manifest.json")
	comboParent, comboParentPresent := readManifestParent(t, comboManifest)
	if !comboParentPresent || comboParent != "combo-parent" {
		t.Errorf("combined flags: manifest \"parent\" present=%v value=%q, want present=true value=\"combo-parent\"", comboParentPresent, comboParent)
	}
	if gotProfile := readManifestGenProfile(t, comboManifest); gotProfile != "full" {
		t.Errorf("combined flags: manifest gen_profile = %q, want \"full\" (--parent must not clobber --profile full)", gotProfile)
	}
	comboProvenance, comboProvenancePresent := readManifestRequireProvenance(t, comboManifest)
	if !comboProvenancePresent || !comboProvenance {
		t.Errorf("combined flags: manifest require_provenance present=%v value=%v, want present=true value=true (--parent must not clobber --require-provenance)", comboProvenancePresent, comboProvenance)
	}

	// No --parent → default root declaration (JSON null), matching the bare
	// default proven in TestCmdInit_DefaultsToRootParentAndPassesAllViolations.
	defaultDir := t.TempDir()
	defaultDomain := filepath.Join(defaultDir, "default-parent-domain")
	if err := cmdInit([]string{defaultDomain}); err != nil {
		t.Fatalf("cmdInit (default, no --parent): %v", err)
	}
	defaultValue, defaultPresent := readManifestParent(t, filepath.Join(defaultDomain, "manifest.json"))
	if !defaultPresent || defaultValue != "" {
		t.Errorf("no --parent: manifest \"parent\" present=%v value=%q, want present=true value=\"\" (JSON null)", defaultPresent, defaultValue)
	}
}
