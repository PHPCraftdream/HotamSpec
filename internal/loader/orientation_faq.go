package loader

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// OrientationFAQEntry is one declared orientation question in a domain's
// manifest.json "orientation_faq" opt-in list. Each entry declares, for one
// basic onboarding question a freshly-spawned agent (or a new human reader)
// must be able to answer fast ("what is this project?", "what is the
// requirement lifecycle?", "who decides what?", "what is currently
// blocked?", "where is the full requirements list?"), a MECHANICALLY
// CHECKABLE proof that the answer is reachable from the generated crystal in
// at most ONE hop.
//
// An entry carries TWO independent satisfaction signals; the check passes if
// EITHER holds:
//
//   - Keywords: a list of substrings that must ALL appear (case-insensitive)
//     in the crystal's text — i.e. the answer is present INLINE in the
//     crystal itself (the Role block, the Mediation-loop block, the
//     LIVE-STATE block, the Domain Map, ...). An empty/absent Keywords list
//     means "this entry is satisfied via Link, never inline".
//   - Link: a repo-root-relative path (the SAME convention the crystal itself
//     uses for its own cross-references, e.g. "domains/hotam-spec-self/docs/
//     gen/PIPELINE.md") that MUST (a) appear in the crystal's text (as a
//     markdown `[text](path)` link OR as a bare path string) AND (b) resolve
//     to a REAL EXISTING FILE on disk relative to the repo root. This is
//     EXACTLY ONE hop — the file the link points at is where the answer
//     lives; a chain of links (crystal → index → answer) does NOT satisfy
//     this check by design (R-orientation-one-hop-only), because the whole
//     point is that an agent orients without following more than one
//     pointer. An empty/absent Link means "this entry is satisfied via
//     Keywords, never via a one-hop pointer".
//
// REPO-ROOT-RELATIVE, not domain-relative: both the root crystal (active
// domain, <repoRoot>/CLAUDE.md) and a consumer domain's local crystal
// (<domainDir>/CLAUDE.md) render their cross-references as repo-root-
// relative paths ("domains/<name>/docs/gen/..."), so a single resolution
// base (the repo root, derived in the check the SAME way gen-spec derives it
// — tier-1: domainDir's parent is "domains") serves both crystal locations.
type OrientationFAQEntry struct {
	Question string   `json:"question"`
	Keywords []string `json:"keywords"`
	Link     string   `json:"link,omitempty"`
}

// ResolveOrientationFAQ reads the optional "orientation_faq" field from the
// manifest.json sitting next to graph.json, mirroring ResolveDiscipline's /
// ResolveGenProfile's / ResolveRequireProvenance's exact pattern (read
// manifest, tolerate a missing file, tolerate malformed JSON, default when
// absent). Returns nil (the HONEST NO-OP — exactly the same shape every
// sibling opt-in resolver already establishes: a domain that has not
// declared an orientation_faq list is not lying about orientability, the
// same way a domain without discipline:"full" is not lying about scenario
// coverage) for every absent/missing-field/malformed case — preserving 100%
// backward compatibility with every manifest.json in this repo and in the
// wild that predates the orientation_faq field (they carry no
// orientation_faq field, so the orientation check stays an honest no-op for
// them, byte-identical to before this field existed).
//
// Malformed entries inside an otherwise-present list (an entry that is not a
// JSON object, or an entry whose Question is empty) are SILENTLY DROPPED
// rather than failing the whole read — an honest no-op for that one entry,
// never a hard error that would block all-violations. A Question is the one
// REQUIRED field on a well-formed entry (it is what a violation message
// names so a resolver can find the broken question); Keywords and Link are
// both optional but at least one MUST be non-empty for the entry to be
// satisfiable (an entry with neither fires a violation at CHECK time, not
// at READ time — the check, not the loader, is where "this entry cannot be
// satisfied" is diagnosed, because that is a graph-level invariant
// violation, not a load error).
func ResolveOrientationFAQ(graphPath string) []OrientationFAQEntry {
	manifestPath := filepath.Join(filepath.Dir(graphPath), "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		// manifest.json absent (a synthetic test-fixture graph built
		// directly in Go without loader.LoadGraph, or a genuinely
		// un-manifested domain) — honest no-op, mirroring every sibling
		// resolver's missing-manifest default.
		return nil
	}
	var raw struct {
		OrientationFAQ []json.RawMessage `json:"orientation_faq"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		// manifest.json exists but is malformed JSON — honest no-op,
		// mirroring ResolveDiscipline's identical malformed-manifest default
		// (a manifest that cannot even be parsed has certainly not validly
		// declared an orientation_faq list).
		return nil
	}
	out := make([]OrientationFAQEntry, 0, len(raw.OrientationFAQ))
	for _, entryRaw := range raw.OrientationFAQ {
		var entry OrientationFAQEntry
		if err := json.Unmarshal(entryRaw, &entry); err != nil {
			// One malformed entry (not a JSON object) — drop it, keep the
			// rest. See doc comment: per-entry honest no-op, not a hard
			// error.
			continue
		}
		if entry.Question == "" {
			// An entry with no Question cannot be named in a violation
			// message, so it carries no checkable contract — drop it.
			continue
		}
		out = append(out, entry)
	}
	// Collapse every "no checkable questions" outcome (absent field,
	// explicit empty list, all entries dropped) to a nil return — the SAME
	// honest-no-op shape every sibling resolver already establishes
	// (ResolveDiscipline returns "", ResolveGenProfile returns the default),
	// so the invariant check's `len(entries) == 0` no-op gate holds and the
	// resolver's own contract reads "nil = nothing declared".
	if len(out) == 0 {
		return nil
	}
	return out
}
