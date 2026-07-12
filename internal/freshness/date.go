package freshness

import "time"

// parseDate parses a YYYY-MM-DD date string using the shared dateLayout.
// It is only used for arithmetic (addDays, daysBetween) — the primary
// OVERDUE/DUE-SOON/FRESH classification in Classify is done via plain
// string comparison, since YYYY-MM-DD sorts lexicographically the same as
// it sorts chronologically.
func parseDate(s string) (time.Time, error) {
	return time.Parse(dateLayout, s)
}
