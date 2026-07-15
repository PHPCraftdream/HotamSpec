package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/freshness"
	"github.com/PHPCraftdream/HotamSpec/internal/loader"
)

// TestInitDomain_SeedRequirementIsFresh proves the R11-a fix directly at the
// unit level: initDomain's seed requirement (R-domain-exists) must classify
// as freshness.Fresh — never freshness.NeverReviewed — as of the same
// `today` it was scaffolded with. Before this fix, initDomain left
// LastReviewedAt/ReviewAfter at their zero value, which freshness.Classify
// reports as NeverReviewed: a fresh `hotam init`/`hotam init-project`'s
// very first `hotam status` would report the tool's own bootstrap artifact
// as unreviewed debt, not a real content gap.
func TestInitDomain_SeedRequirementIsFresh(t *testing.T) {
	t.Parallel()

	domainDir := t.TempDir()
	today := "2026-07-15"
	if _, err := initDomain(domainDir, "test-seed-freshness", today); err != nil {
		t.Fatalf("initDomain: %v", err)
	}

	g, err := loadDomainGraph(domainDir)
	if err != nil {
		t.Fatalf("loadDomainGraph: %v", err)
	}

	found := false
	for _, r := range g.Requirements {
		if r.ID != "R-domain-exists" {
			continue
		}
		found = true
		if r.LastReviewedAt != today {
			t.Errorf("seed requirement LastReviewedAt = %q, want %q", r.LastReviewedAt, today)
		}
		if r.ReviewAfter == "" {
			t.Errorf("seed requirement ReviewAfter is empty, want a date derived from %q + %d days", today, seedReviewCadenceDays)
		}
		status := freshness.Classify(r, today)
		if status != freshness.Fresh {
			t.Errorf("freshness.Classify(seed requirement, %q) = %s, want %s (this is the exact false-signal R11-a fixes)", today, status, freshness.Fresh)
		}
	}
	if !found {
		t.Fatalf("seed requirement R-domain-exists not found in scaffolded graph")
	}
}

// TestInitDomain_SeedReviewAfterBeyondDueSoonWindow confirms the exact
// cadence offset chosen (seedReviewCadenceDays == 180) lands the seed
// comfortably outside freshness.DueSoonWindowDays (30 days), so the seed is
// genuinely Fresh at scaffold time, not merely "not yet NeverReviewed but
// now DueSoon" — trading one false signal for another would not satisfy the
// fix's intent.
func TestInitDomain_SeedReviewAfterBeyondDueSoonWindow(t *testing.T) {
	t.Parallel()

	domainDir := t.TempDir()
	today := "2026-07-15"
	if _, err := initDomain(domainDir, "test-seed-cadence", today); err != nil {
		t.Fatalf("initDomain: %v", err)
	}

	g, err := loadDomainGraph(domainDir)
	if err != nil {
		t.Fatalf("loadDomainGraph: %v", err)
	}
	for _, r := range g.Requirements {
		if r.ID != "R-domain-exists" {
			continue
		}
		if r.ReviewAfter <= today {
			t.Fatalf("seed ReviewAfter %q must be strictly after today %q", r.ReviewAfter, today)
		}
		if got := freshness.Classify(r, today); got != freshness.Fresh {
			t.Errorf("seed requirement classified as %s at scaffold time, want %s (review cadence too short)", got, freshness.Fresh)
		}
	}
}

// TestCmdInit_TodayFlagWiredToSeedFreshness proves `hotam init` (bare, not
// init-project) gained --today date-pinning parity with init-project, AND
// that the flag actually reaches initDomain's freshness seeding rather than
// only affecting some other output: an explicit --today must produce a seed
// requirement whose LastReviewedAt equals that exact pinned date and whose
// freshness.Classify result is Fresh, not NeverReviewed.
func TestCmdInit_TodayFlagWiredToSeedFreshness(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	domainDir := filepath.Join(dir, "pinned-domain")
	pinnedToday := "2026-01-01"

	if err := cmdInit([]string{"--today", pinnedToday, domainDir}); err != nil {
		t.Fatalf("cmdInit: %v", err)
	}

	g, err := loadDomainGraph(domainDir)
	if err != nil {
		t.Fatalf("loadDomainGraph: %v", err)
	}
	found := false
	for _, r := range g.Requirements {
		if r.ID != "R-domain-exists" {
			continue
		}
		found = true
		if r.LastReviewedAt != pinnedToday {
			t.Errorf("seed requirement LastReviewedAt = %q, want pinned --today %q", r.LastReviewedAt, pinnedToday)
		}
		if got := freshness.Classify(r, pinnedToday); got != freshness.Fresh {
			t.Errorf("hotam init --today %q: seed requirement classified as %s, want %s", pinnedToday, got, freshness.Fresh)
		}
	}
	if !found {
		t.Fatalf("seed requirement R-domain-exists not found after cmdInit")
	}
}

// TestCmdInit_TodayFlagDefaultsToSystemDate confirms omitting --today still
// works (defaults to system date, exactly like init-project's own
// long-standing --today convention) — the flag is additive, not a new
// required argument.
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
	found := false
	for _, r := range g.Requirements {
		if r.ID != "R-domain-exists" {
			continue
		}
		found = true
		if r.LastReviewedAt == "" {
			t.Errorf("seed requirement LastReviewedAt is empty even without --today; want it defaulted to system date")
		}
	}
	if !found {
		t.Fatalf("seed requirement R-domain-exists not found after cmdInit")
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
