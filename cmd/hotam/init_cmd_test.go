package main

import (
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/freshness"
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
