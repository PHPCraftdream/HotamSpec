package generator

import (
	"fmt"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/gate"
)

// engineVersionMDRelPath is where genSpec (cmd/hotam/gen_spec.go) writes
// ENGINE-VERSION.md, relative to a domain's own directory — kept as a named
// constant here (the same discipline specMDRelPath in internal/invariants
// already establishes) so the write side and the check side visibly agree on
// one literal path.
const engineVersionMDRelPath = "docs/gen/ENGINE-VERSION.md"

// engineFingerprintShortLen is the number of hex characters the stamped
// fingerprint is truncated to for display. 16 hex chars (64 bits) is
// collision-safe for this use case and keeps the stamped line compact. The
// full 64-char hash is never needed on the stamp side — the check truncates
// the freshly-computed fingerprint the same way before comparing.
const engineFingerprintShortLen = 16

// engineFingerprintLabel is the marker line prefix the invariant check
// (check_engine_docs_fingerprint_current, internal/invariants) searches for
// to extract the stamped fingerprint. Kept as a shared exported constant so
// the writer and the reader agree on one literal — the same discipline
// DurableNotesMarkerLine / specMDRelPath already establish.
const engineFingerprintLabel = "**Engine content fingerprint:**"

// BuildEngineVersionMD renders docs/gen/ENGINE-VERSION.md: a small dedicated
// generated file carrying the engine identity fingerprint. Unlike every other
// docs/gen/*.md projection (which is a pure function of the domain graph),
// this file's content depends ONLY on the engine's own source packages
// (internal/generator, internal/ontology, internal/loader) — it is engine
// metadata, not domain content — so it is always written regardless of
// whether the domain graph is empty or full (every domain has an engine
// fingerprint).
//
// The fingerprint is computed by gate.EngineDocsFingerprint(moduleRoot): a
// deterministic sha256 content-hash over the three generator-relevant
// packages. Two consecutive gen-spec runs from the same unchanged engine
// binary produce the identical fingerprint, every time — so stamping it does
// not violate task #317's byte-idempotency guarantee for docs/gen/ output.
//
// moduleRoot is the engine's Go module root (the directory containing go.mod),
// the same value genSpec already computes as repoRoot.
func BuildEngineVersionMD(moduleRoot string) (string, error) {
	fp, err := gate.EngineDocsFingerprint(moduleRoot)
	if err != nil {
		return "", fmt.Errorf("build engine-version: %w", err)
	}
	short := fp
	if len(short) > engineFingerprintShortLen {
		short = short[:engineFingerprintShortLen]
	}
	lines := []string{
		generatedHeaderComment,
		"",
		"# ENGINE-VERSION.md — engine identity fingerprint (Hotam-Spec)",
		"",
		"This domain's `docs/gen/` output was produced by an engine build with the",
		"content fingerprint below. If the current engine's fingerprint differs, the",
		"generated docs may be stale — regenerate via `hotam gen-spec --domain <path>`.",
		"",
		engineFingerprintLabel + " `" + short + "`",
		"",
		"The fingerprint is a deterministic sha256 content-hash over the three",
		"generator-relevant engine packages (`internal/generator`,",
		"`internal/ontology`, `internal/loader`), whose changes can affect a domain's",
		"generated-doc shape or content. It is not tied to the repo's git commit SHA",
		"(which would invalidate on every unrelated commit) nor to a manually-bumped",
		"version number.",
	}
	return strings.Join(lines, "\n") + "\n", nil
}
