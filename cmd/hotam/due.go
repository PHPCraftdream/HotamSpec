package main

import (
	"fmt"
	"time"

	"github.com/PHPCraftdream/HotamSpec/internal/freshness"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// neverReviewedTopN is how many NEVER-REVIEWED requirement ids `hotam due`
// prints individually before folding the rest into the summary count. On
// domains/hotam-spec-self ALL 232 SETTLED requirements are currently
// NEVER-REVIEWED, so printing every id would be 232 lines of noise; a
// count + a bounded sample is enough to act on (backfill review metadata)
// without drowning the OVERDUE section, which is the actually-urgent list.
const neverReviewedTopN = 10

// cmdDue implements `hotam due [--domain <path>] [--today YYYY-MM-DD] [--json]`:
// an advisory report of SETTLED requirements whose review is OVERDUE or
// that have NEVER been reviewed at all. It never gates — exit code is 0
// whether or not stale requirements are found, because freshness is a
// housekeeping signal for the steward, not a structural invariant
// (R-requirement-freshness-fields exists on the model; whether the fields
// are populated is a backlog item, not a violation).
func cmdDue(args []string) error {
	fs := newFlagSet("due")
	domain := fs.String("domain", "", "domain directory (default: "+defaultDomainRel+")")
	todayFlag := fs.String("today", "", "date in YYYY-MM-DD format (default: system date)")
	asJSON := fs.Bool("json", false, "emit machine-readable JSON")
	fs.Parse(args)

	domainDir, err := resolveDomain(*domain)
	if err != nil {
		return err
	}
	today := *todayFlag
	if today == "" {
		today = time.Now().Format("2006-01-02")
	}

	g, err := loadDomainGraph(domainDir)
	if err != nil {
		return err
	}

	report := buildDueReport(g, today)

	if *asJSON {
		return printJSON(report)
	}
	fmt.Println(formatDueReport(report))
	return nil
}

// DueReport is the JSON/text-rendered shape of `hotam due`'s output.
type DueReport struct {
	Today               string     `json:"today"`
	OverdueCount        int        `json:"overdue_count"`
	Overdue             []DueEntry `json:"overdue"`
	NeverReviewedCount  int        `json:"never_reviewed_count"`
	NeverReviewedSample []DueEntry `json:"never_reviewed_sample"`
	DueSoonCount        int        `json:"due_soon_count"`
}

// DueEntry is one requirement's freshness line: enough to act on without
// re-fetching the full requirement via `hotam req show`.
type DueEntry struct {
	ID          string `json:"id"`
	Owner       string `json:"owner"`
	ReviewAfter string `json:"review_after"`
	OverdueDays int    `json:"overdue_days,omitempty"`
	Summary     string `json:"summary,omitempty"`
}

func buildDueReport(g *ontology.Graph, today string) DueReport {
	classified := freshness.ClassifyGraph(g, today)

	var overdue, neverReviewed []freshness.Classification
	dueSoonCount := 0
	for _, c := range classified {
		switch c.Status {
		case freshness.Overdue:
			overdue = append(overdue, c)
		case freshness.NeverReviewed:
			neverReviewed = append(neverReviewed, c)
		case freshness.DueSoon:
			dueSoonCount++
		}
	}

	report := DueReport{
		Today:              today,
		OverdueCount:       len(overdue),
		NeverReviewedCount: len(neverReviewed),
		DueSoonCount:       dueSoonCount,
	}
	for _, c := range overdue {
		report.Overdue = append(report.Overdue, DueEntry{
			ID:          c.Requirement.ID,
			Owner:       c.Requirement.Owner,
			ReviewAfter: c.Requirement.ReviewAfter,
			OverdueDays: c.OverdueDays,
			Summary:     c.Requirement.Summary,
		})
	}
	sampleN := neverReviewedTopN
	if sampleN > len(neverReviewed) {
		sampleN = len(neverReviewed)
	}
	for _, c := range neverReviewed[:sampleN] {
		report.NeverReviewedSample = append(report.NeverReviewedSample, DueEntry{
			ID:      c.Requirement.ID,
			Owner:   c.Requirement.Owner,
			Summary: c.Requirement.Summary,
		})
	}
	return report
}

func formatDueReport(r DueReport) string {
	var out string
	out += fmt.Sprintf("hotam due — freshness report as of %s\n", r.Today)
	out += "\n"
	if r.OverdueCount == 0 {
		out += "OVERDUE: none\n"
	} else {
		out += fmt.Sprintf("OVERDUE (%d), oldest first:\n", r.OverdueCount)
		for _, e := range r.Overdue {
			out += fmt.Sprintf("  [%4dd] %-30s owner=%-12s review_after=%s\n", e.OverdueDays, e.ID, e.Owner, e.ReviewAfter)
		}
	}
	out += "\n"
	if r.NeverReviewedCount == 0 {
		out += "NEVER-REVIEWED: none\n"
	} else {
		out += fmt.Sprintf("NEVER-REVIEWED: %d requirement(s) have no last_reviewed_at and no review_after.\n", r.NeverReviewedCount)
		shown := len(r.NeverReviewedSample)
		out += fmt.Sprintf("  top %d by id:\n", shown)
		for _, e := range r.NeverReviewedSample {
			out += fmt.Sprintf("    %-30s owner=%s\n", e.ID, e.Owner)
		}
		if r.NeverReviewedCount > shown {
			out += fmt.Sprintf("    … and %d more\n", r.NeverReviewedCount-shown)
		}
	}
	if r.DueSoonCount > 0 {
		out += fmt.Sprintf("\nDUE-SOON (next %d days): %d requirement(s) — not yet urgent.\n", freshness.DueSoonWindowDays, r.DueSoonCount)
	}
	return out
}
