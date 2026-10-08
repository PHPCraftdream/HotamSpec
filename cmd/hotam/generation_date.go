package main

import (
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// The machine marker is locale-independent. Only the comparative publication
// render uses it; calendar review classifiers still receive the current date.
var publicationDateRE = regexp.MustCompile(`<!-- hotam-publication-date: (\d{4}-\d{2}-\d{2}) -->`)

func publicationStampedDate(contents []byte) (string, bool) {
	match := publicationDateRE.FindSubmatch(contents)
	if len(match) != 2 {
		return "", false
	}
	date := string(match[1])
	_, err := time.Parse("2006-01-02", date)
	return date, err == nil
}

// domainGenerationDate uses the publication's own date, including localized
// boot views, so midnight never invalidates content-identical logical outputs.
func domainGenerationDate(domainDir string) (string, bool) {
	genDir := filepath.Join(domainDir, "docs", "gen")
	for _, pattern := range []string{"live-state*.md", "AGENT-CONTEXT*.md"} {
		paths, _ := filepath.Glob(filepath.Join(genDir, pattern))
		for _, path := range paths {
			if contents, err := os.ReadFile(path); err == nil {
				if date, ok := publicationStampedDate(contents); ok {
					return date, true
				}
			}
		}
	}
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
