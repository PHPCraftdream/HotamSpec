package freshness

import (
	"sort"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// Status is the freshness classification of a single SETTLED requirement.
type Status string

const (
	// Overdue means review_after is set and is strictly before today.
	Overdue Status = "OVERDUE"
	// DueSoon means review_after is set, not yet overdue, but falls
	// within the next DueSoonWindowDays days (today <= review_after <
	// today+window).
	DueSoon Status = "DUE-SOON"
	// NeverReviewed means both last_reviewed_at and review_after are
	// empty — the requirement has no freshness signal at all.
	NeverReviewed Status = "NEVER-REVIEWED"
	// Fresh means none of the above: either review_after lies far enough
	// in the future, or (edge case) last_reviewed_at is set but
	// review_after is empty — reviewed at least once with no next-review
	// date scheduled, treated as not currently due.
	Fresh Status = "FRESH"
)

// DueSoonWindowDays is the width of the DUE-SOON lookahead window in days:
// a requirement whose review_after falls within [today, today+window) is
// DUE-SOON rather than FRESH.
const DueSoonWindowDays = 30

// dateLayout is the on-disk date format used throughout the graph
// (YYYY-MM-DD), shared with ontology's stored fields.
const dateLayout = "2006-01-02"

// Classification pairs a Requirement with its computed freshness Status and
// the number of days it is overdue (0 for non-OVERDUE statuses).
type Classification struct {
	Requirement ontology.Requirement
	Status      Status
	OverdueDays int
}

// Classify returns the freshness Status for a single requirement as of
// today (YYYY-MM-DD). Only SETTLED requirements carry a meaningful
// freshness signal; callers filtering a whole graph should skip non-SETTLED
// requirements before calling this (see ClassifyGraph).
func Classify(r ontology.Requirement, today string) Status {
	reviewAfter := r.ReviewAfter
	lastReviewed := r.LastReviewedAt

	if reviewAfter == "" && lastReviewed == "" {
		return NeverReviewed
	}
	if reviewAfter == "" {
		// Reviewed at least once, no next-review date scheduled.
		return Fresh
	}
	if reviewAfter < today {
		return Overdue
	}
	if reviewAfter < addDays(today, DueSoonWindowDays) {
		return DueSoon
	}
	return Fresh
}

// OverdueDays returns how many days r's review_after lies before today, or
// 0 if r is not OVERDUE (or the dates fail to parse). today and
// r.ReviewAfter are both expected in YYYY-MM-DD form.
func OverdueDays(r ontology.Requirement, today string) int {
	if r.ReviewAfter == "" || r.ReviewAfter >= today {
		return 0
	}
	d, err := daysBetween(r.ReviewAfter, today)
	if err != nil {
		return 0
	}
	return d
}

// ClassifyGraph classifies every SETTLED requirement in g as of today and
// returns the results sorted most-urgent-first: OVERDUE (oldest
// review_after first) before NEVER-REVIEWED before DUE-SOON before FRESH,
// with ties broken by requirement ID for determinism.
func ClassifyGraph(g *ontology.Graph, today string) []Classification {
	out := make([]Classification, 0, len(g.Requirements))
	for _, r := range g.Requirements {
		if r.Status != ontology.StatusSETTLED {
			continue
		}
		status := Classify(r, today)
		out = append(out, Classification{
			Requirement: r,
			Status:      status,
			OverdueDays: OverdueDays(r, today),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		pi, pj := statusRank(out[i].Status), statusRank(out[j].Status)
		if pi != pj {
			return pi < pj
		}
		if out[i].Status == Overdue && out[i].OverdueDays != out[j].OverdueDays {
			return out[i].OverdueDays > out[j].OverdueDays
		}
		return out[i].Requirement.ID < out[j].Requirement.ID
	})
	return out
}

func statusRank(s Status) int {
	switch s {
	case Overdue:
		return 0
	case NeverReviewed:
		return 1
	case DueSoon:
		return 2
	default:
		return 3
	}
}

// addDays returns the YYYY-MM-DD date `days` days after date. On parse
// failure it returns date unchanged (so a malformed today/review_after
// degrades to "not due-soon" rather than panicking).
func addDays(date string, days int) string {
	t, err := parseDate(date)
	if err != nil {
		return date
	}
	return t.AddDate(0, 0, days).Format(dateLayout)
}

// daysBetween returns the number of whole days from -> to (to - from), or
// an error if either date fails to parse as YYYY-MM-DD.
func daysBetween(from, to string) (int, error) {
	f, err := parseDate(from)
	if err != nil {
		return 0, err
	}
	t, err := parseDate(to)
	if err != nil {
		return 0, err
	}
	return int(t.Sub(f).Hours() / 24), nil
}
