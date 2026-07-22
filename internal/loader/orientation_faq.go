package loader

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
	diag := ResolveOrientationFAQDiagnostic(graphPath)
	return diag.Entries
}

// DroppedOrientationFAQEntry names one raw "orientation_faq" list entry that
// ResolveOrientationFAQDiagnostic dropped, plus WHY — the diagnosis
// ResolveOrientationFAQ's tolerant contract deliberately discards (see its
// doc comment: "the check, not the loader, is where 'this entry cannot be
// satisfied' is diagnosed"). The invariant layer uses this to turn a silent
// drop into a reported violation instead of quietly shrinking the list.
type DroppedOrientationFAQEntry struct {
	// Index is the entry's position (0-based) in the raw "orientation_faq"
	// JSON array, used to name the entry in a violation message when it has
	// no usable Question (e.g. it is not even a JSON object).
	Index int
	// Raw is the original raw JSON for the dropped entry, truncated for
	// display if long.
	Raw string
	// Reason is a short human-readable diagnosis ("not a JSON object", "the
	// question field is empty").
	Reason string
}

// OrientationFAQDiagnostic is ResolveOrientationFAQDiagnostic's return
// value: the same tolerant Entries list ResolveOrientationFAQ returns, PLUS
// the diagnostic detail the invariant layer needs to fail closed rather than
// silently accept a shrunken list: whether manifest.json exists, whether it
// parsed as valid JSON, and which raw entries (if the field parsed) were
// dropped and why.
type OrientationFAQDiagnostic struct {
	// Entries is byte-identical to ResolveOrientationFAQ's return value
	// (nil when nothing is checkable).
	Entries []OrientationFAQEntry
	// ManifestExists is false when manifest.json itself is absent (a
	// synthetic fixture or a genuinely un-manifested domain) — the loader's
	// oldest, most tolerated honest no-op.
	ManifestExists bool
	// ManifestParsed is false when manifest.json exists but is not valid
	// JSON. Combined with ManifestDeclaresIntent, this is what lets the
	// invariant tell "malformed manifest, domain never touched
	// orientation_faq" (still an honest no-op) apart from "malformed
	// manifest, domain's raw bytes show it tried to declare
	// orientation_faq" (a fail-closed violation).
	ManifestParsed bool
	// ManifestDeclaresIntent is true when the raw manifest bytes contain the
	// literal substring "orientation_faq" — a coarse, JSON-parse-independent
	// signal checked BEFORE attempting to parse, so it still fires when the
	// parse itself fails. Only meaningful when ManifestParsed is false
	// (when the manifest parses cleanly, the parsed OrientationFAQ field
	// itself is the authoritative signal, not this heuristic).
	ManifestDeclaresIntent bool
	// Dropped lists every raw "orientation_faq" array entry that did not
	// survive per-entry validation, in array order.
	Dropped []DroppedOrientationFAQEntry
}

// ResolveOrientationFAQDiagnostic is ResolveOrientationFAQ's diagnostic
// twin: same tolerant parsing, same nil-when-nothing-checkable Entries
// contract (every existing caller of ResolveOrientationFAQ is unaffected —
// that function now just forwards to this one's Entries field), but it ALSO
// preserves the detail the tolerant path throws away, so the invariant layer
// (internal/invariants/orientation_faq.go's checkOrientationFAQAnswered) can
// diagnose two cases the loader itself must stay silent about by design:
//
//  1. A manifest that fails to parse as JSON but whose raw bytes contain the
//     literal "orientation_faq" substring — the domain evidently TRIED to
//     declare an orientation contract and a JSON typo elsewhere in the file
//     silently destroyed it. The loader still returns nil Entries (its
//     documented, reused-by-other-resolvers contract), but
//     ManifestDeclaresIntent is now visible so the check can fail closed
//     instead of silently no-op'ing.
//  2. Individual malformed/unsatisfiable raw entries inside an otherwise
//     well-parsed list — still dropped from Entries exactly as before, but
//     now named in Dropped with a reason, so the check can report "N entries
//     were dropped" as a violation instead of quietly working with a
//     shortened list.
func ResolveOrientationFAQDiagnostic(graphPath string) OrientationFAQDiagnostic {
	manifestPath := filepath.Join(filepath.Dir(graphPath), "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		// manifest.json absent (a synthetic test-fixture graph built
		// directly in Go without loader.LoadGraph, or a genuinely
		// un-manifested domain) — honest no-op, mirroring every sibling
		// resolver's missing-manifest default.
		return OrientationFAQDiagnostic{}
	}

	var raw struct {
		OrientationFAQ []json.RawMessage `json:"orientation_faq"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		// manifest.json exists but is malformed JSON. ResolveOrientationFAQ's
		// documented, reused-by-other-resolvers contract (honest no-op) is
		// preserved for Entries — but a coarse pre-parse heuristic on the RAW
		// bytes (does the literal field name even appear?) tells the
		// invariant layer whether this domain looks like it TRIED to declare
		// the field, so the check can fail closed instead of silently
		// treating "declared but broken" the same as "never declared".
		return OrientationFAQDiagnostic{
			ManifestExists:         true,
			ManifestParsed:         false,
			ManifestDeclaresIntent: strings.Contains(string(data), `"orientation_faq"`),
		}
	}

	out := make([]OrientationFAQEntry, 0, len(raw.OrientationFAQ))
	var dropped []DroppedOrientationFAQEntry
	for i, entryRaw := range raw.OrientationFAQ {
		var entry OrientationFAQEntry
		if err := json.Unmarshal(entryRaw, &entry); err != nil {
			// One malformed entry (not a JSON object) — drop it from
			// Entries, keep the rest. See doc comment: per-entry honest
			// no-op at load time, diagnosed as a violation at check time.
			dropped = append(dropped, DroppedOrientationFAQEntry{
				Index:  i,
				Raw:    truncateForDiagnostic(string(entryRaw)),
				Reason: "not a JSON object",
			})
			continue
		}
		if entry.Question == "" {
			// An entry with no Question cannot be named in a violation
			// message by itself, so it carries no checkable contract — drop
			// it from Entries, but still report the index so the domain can
			// find and fix it.
			dropped = append(dropped, DroppedOrientationFAQEntry{
				Index:  i,
				Raw:    truncateForDiagnostic(string(entryRaw)),
				Reason: "the question field is empty",
			})
			continue
		}
		out = append(out, entry)
	}
	// Collapse "no checkable questions" (absent field, explicit empty list,
	// all entries dropped) to a nil Entries — the SAME honest-no-op shape
	// every sibling resolver already establishes (ResolveDiscipline returns
	// "", ResolveGenProfile returns the default), so
	// ResolveOrientationFAQ's/the invariant check's `len(entries) == 0`
	// no-op gate holds.
	if len(out) == 0 {
		out = nil
	}
	return OrientationFAQDiagnostic{
		Entries:        out,
		ManifestExists: true,
		ManifestParsed: true,
		Dropped:        dropped,
	}
}

// truncateForDiagnostic caps a raw-JSON snippet embedded in a violation
// message so one absurdly long malformed entry cannot blow up diagnostic
// output.
func truncateForDiagnostic(s string) string {
	const maxLen = 120
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "...(truncated)"
}
