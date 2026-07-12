// Command seed-due is a ONE-TIME backfill generator for TaskList P1-3: it
// reads a domain's graph.json and writes one ProposedReviewMark JSON file
// per SETTLED requirement lacking last_reviewed_at/review_after, so the
// freshness engine (internal/freshness, `hotam due`) has real data instead
// of defaulting every SETTLED requirement to NEVER-REVIEWED.
//
// Policy (see reviewBackfillMonths below): last_reviewed_at = settled_at
// (falling back to created_at if settled_at is blank), review_after =
// last_reviewed_at + reviewBackfillMonths months.
//
// This tool is NOT part of the `hotam` CLI surface — it exists to produce
// proposal JSON files that are then landed one at a time through the
// normal `hotam land` pipeline (apply -> gen-spec -> all-violations), so
// the backfill goes through the exact same invariant-checked write path as
// any other proposal (R-no-hand-edit-graph). Delete this directory once
// the backfill is complete; it has no ongoing purpose.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/PHPCraftdream/HotamSpecGo/internal/loader"
	"github.com/PHPCraftdream/HotamSpecGo/internal/ontology"
)

// reviewBackfillMonths is the default review interval applied to every
// backfilled SETTLED requirement: last_reviewed_at + reviewBackfillMonths
// months = review_after. Six months is a conservative default cadence for
// re-affirming a settled claim is still true; TaskList P1-3 explicitly
// calls this "a simple default policy" and asks for it to live in one
// named constant/flag rather than be inlined ad hoc.
const reviewBackfillMonths = 6

func main() {
	domain := flag.String("domain", "", "domain directory containing graph.json (required)")
	outDir := flag.String("out", "", "directory to write ProposedReviewMark JSON files into (required)")
	months := flag.Int("months", reviewBackfillMonths, "review interval in months added to last_reviewed_at to get review_after")
	fallbackToday := flag.String("fallback-today", "", "YYYY-MM-DD used as last_reviewed_at when a SETTLED requirement has neither settled_at nor created_at (leave empty to hard-fail on such requirements instead)")
	flag.Parse()

	if *domain == "" || *outDir == "" {
		fmt.Fprintln(os.Stderr, "usage: seed-due --domain <path> --out <dir> [--months N] [--fallback-today YYYY-MM-DD]")
		os.Exit(2)
	}
	if err := run(*domain, *outDir, *months, *fallbackToday); err != nil {
		fmt.Fprintln(os.Stderr, "seed-due:", err)
		os.Exit(1)
	}
}

func run(domain, outDir string, months int, fallbackToday string) error {
	graphPath := filepath.Join(domain, "graph.json")
	g, err := loader.LoadGraph(graphPath)
	if err != nil {
		return fmt.Errorf("load graph %s: %w", graphPath, err)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", outDir, err)
	}

	written := 0
	for _, r := range g.Requirements {
		if r.Status != ontology.StatusSETTLED {
			continue
		}
		if r.LastReviewedAt != "" || r.ReviewAfter != "" {
			continue // already has freshness data; not this tool's job to touch it
		}
		reviewedAt := r.SettledAt
		if reviewedAt == "" {
			reviewedAt = r.CreatedAt
		}
		if reviewedAt == "" {
			// Neither settled_at nor created_at is populated -- a
			// pre-existing data gap distinct from the freshness backfill
			// this tool performs (some domains never had those fields
			// filled in at all). Rather than fabricate a fictitious
			// historical date, fall back to the operator-supplied
			// "today" so the review mark honestly says "reviewed now,
			// no earlier record exists" instead of implying a settlement
			// date that was never recorded.
			if fallbackToday == "" {
				return fmt.Errorf("requirement %s is SETTLED but has neither settled_at nor created_at; pass --fallback-today to backfill it as reviewed-now instead", r.ID)
			}
			reviewedAt = fallbackToday
		}
		reviewAfter, err := addMonths(reviewedAt, months)
		if err != nil {
			return fmt.Errorf("requirement %s: %w", r.ID, err)
		}

		mark := struct {
			Kind          string `json:"kind"`
			RequirementID string `json:"requirement_id"`
			ReviewedAt    string `json:"reviewed_at"`
			ReviewAfter   string `json:"review_after"`
		}{
			Kind:          "ReviewMark",
			RequirementID: r.ID,
			ReviewedAt:    reviewedAt,
			ReviewAfter:   reviewAfter,
		}
		data, err := json.MarshalIndent(mark, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal %s: %w", r.ID, err)
		}
		outPath := filepath.Join(outDir, r.ID+".json")
		if err := os.WriteFile(outPath, data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", outPath, err)
		}
		written++
	}
	fmt.Printf("wrote %d ProposedReviewMark file(s) to %s\n", written, outDir)
	return nil
}

// addMonths returns date (YYYY-MM-DD) advanced by months calendar months,
// using Go's normalized AddDate so month-length overflow (e.g. Jan 31 + 1
// month) rolls into the following month consistently rather than erroring.
func addMonths(date string, months int) (string, error) {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return "", fmt.Errorf("parse date %q: %w", date, err)
	}
	return t.AddDate(0, months, 0).Format("2006-01-02"), nil
}
