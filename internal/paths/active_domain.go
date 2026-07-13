package paths

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// activeDomainDoc is the JSON shape written to the .hotam-spec-project marker
// file by WriteActiveDomain and read back by ReadActiveDomain. It is OPTIONAL:
// a marker file written empty (the pre-active-domain scaffold behavior) or with
// just "{}" still resolves ok=false from ReadActiveDomain — the marker's primary
// role for project-root detection (searchMarkerFileUpward) is existence-only and
// never parses content, so adding this field is purely additive.
type activeDomainDoc struct {
	ActiveDomain string `json:"active_domain"`
}

// ReadActiveDomain reads the active-domain preference recorded in the marker
// file at markerPath. It returns (name, true) only when the file exists, decodes
// as JSON, and carries a non-empty active_domain field. A missing file, an empty
// file, malformed JSON, or an empty active_domain field all return ("", false) —
// this is advisory resolution, never an error condition, so a hand-written or
// legacy marker degrades gracefully and falls through to the next resolution
// tier instead of breaking project-root detection.
func ReadActiveDomain(markerPath string) (string, bool) {
	data, err := os.ReadFile(markerPath)
	if err != nil {
		return "", false
	}
	var doc activeDomainDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return "", false
	}
	if doc.ActiveDomain == "" {
		return "", false
	}
	return doc.ActiveDomain, true
}

// WriteActiveDomain writes {"active_domain": "<domainName>"} to markerPath,
// pretty-printed as 2-space-indented JSON with a trailing newline (matching the
// repo's JSON-file convention — json.MarshalIndent with "  " indent, like
// graph.lock in internal/loader/lock.go). It creates parent directories as
// needed so it works even when the marker file did not exist before (e.g. a
// domains/-native-marker project promoted via `hotam use`).
func WriteActiveDomain(markerPath, domainName string) error {
	doc := activeDomainDoc{ActiveDomain: domainName}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(markerPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(markerPath, data, 0o644)
}
