// Package freshness classifies SETTLED requirements by how overdue their
// periodic review is, using the last_reviewed_at / review_after fields
// already present on ontology.Requirement (R-requirement-freshness-fields).
// Dates are compared as plain YYYY-MM-DD strings — the format the graph
// already stores them in — so classification never has to round-trip
// through time.Time or worry about timezones; lexicographic string
// comparison on YYYY-MM-DD is equivalent to calendar-date comparison.
package freshness
