package diagnose

import (
	"fmt"
	"time"

	"github.com/PHPCraftdream/HotamSpecGo/internal/freshness"
	"github.com/PHPCraftdream/HotamSpecGo/internal/ontology"
)

// FreshnessSignals returns advisory Signals summarizing how many SETTLED
// requirements are OVERDUE for review and how many have NEVER-REVIEWED
// (no last_reviewed_at and no review_after at all), as of today (YYYY-MM-DD).
// It is a two-line summary — never one Signal per requirement — because on
// a graph the size of hotam-spec-self (232/232 SETTLED currently
// NEVER-REVIEWED) a per-requirement signal would drown every other signal
// DiagnoseSignals produces. Detail belongs to `hotam due`, not what-now.
func FreshnessSignals(g *ontology.Graph, today string) []Signal {
	classified := freshness.ClassifyGraph(g, today)

	var overdue, neverReviewed int
	for _, c := range classified {
		switch c.Status {
		case freshness.Overdue:
			overdue++
		case freshness.NeverReviewed:
			neverReviewed++
		}
	}

	var out []Signal
	if overdue > 0 {
		out = append(out, Signal{
			Source:   "diagnose",
			Priority: PAdvisory,
			Check:    "freshness_overdue",
			Target:   "review-freshness",
			Message: fmt.Sprintf(
				"%d SETTLED requirement(s) are OVERDUE for review (review_after < %s) — run `hotam due --today %s` for the list, then land a ProposedReviewMark per requirement re-affirmed.",
				overdue, today, today,
			),
		})
	}
	if neverReviewed > 0 {
		out = append(out, Signal{
			Source:   "diagnose",
			Priority: PAdvisory,
			Check:    "freshness_never_reviewed",
			Target:   "review-freshness",
			Message: fmt.Sprintf(
				"%d SETTLED requirement(s) have NEVER been reviewed (no last_reviewed_at, no review_after) — run `hotam due --today %s` for the list; freshness metadata is currently unpopulated (R-requirement-freshness-fields).",
				neverReviewed, today,
			),
		})
	}
	return out
}

// todayISO returns the current date in the graph's YYYY-MM-DD form.
func todayISO() string {
	return time.Now().Format("2006-01-02")
}
