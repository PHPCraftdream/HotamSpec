package main

import (
	"os"
	"path/filepath"
)

// domainGenerationDate returns the date a domain's last gen-spec run stamped
// into its generated docs: `- **generated:** <date>` in docs/gen/live-state.md,
// else `(as of <date>)` in docs/gen/AGENT-CONTEXT.md. Freshness checks judge a
// projection as of that date, so a calendar day passing (a review_after
// crossing, say) never turns an unchanged tree red; only the domain's own
// graph/code changes do. ok is false when neither file carries a date
// (localized, empty or consumer-profile domains), and callers then keep the
// calendar day.
func domainGenerationDate(domainDir string) (string, bool) {
	genDir := filepath.Join(domainDir, "docs", "gen")
	if b, err := os.ReadFile(filepath.Join(genDir, "live-state.md")); err == nil {
		if date, ok := liveStateStampedDate(b); ok {
			return date, true
		}
	}
	if b, err := os.ReadFile(filepath.Join(genDir, "AGENT-CONTEXT.md")); err == nil {
		if date, ok := agentContextStampedDate(b); ok {
			return date, true
		}
	}
	return "", false
}
