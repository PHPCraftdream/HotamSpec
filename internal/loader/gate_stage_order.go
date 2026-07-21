package loader

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// ResolveGateStageOrder reads the optional "gate_stage_order" field from the
// manifest.json sitting next to graph.json, mirroring ResolveDiscipline's /
// ResolveGenProfile's / ResolveOrientationFAQ's exact pattern (read
// manifest, tolerate a missing file, tolerate malformed JSON, default when
// absent). Returns nil (the HONEST NO-OP — exactly the same shape every
// sibling opt-in resolver already establishes) for every absent/
// missing-field/malformed/empty case — preserving 100% backward
// compatibility with every manifest.json in this repo and in the wild that
// predates the gate_stage_order field (they carry no gate_stage_order field,
// so every gate-signoff invariant that consults this resolver stays an
// honest no-op for them, byte-identical to before this field existed).
//
// gate_stage_order is intentionally opaque DATA, not an engine-known enum:
// the engine does not know or care what a "gate" is for any particular
// domain's methodology (e.g. prat/gpsm-sm's P-G0..P-G4) — a domain that
// wants ontology.GateSignoff.State=SIGNED entries checked for monotonic
// stage order declares its own stage vocabulary and order here, as an
// ordered list of strings that must exactly match the Stage values that
// vocabulary's GateSignoff entries carry (see
// internal/invariants/gate_signoff_checks.go's
// check_gate_signoff_monotonic). A domain with a different or no staged-gate
// discipline (hotam-spec-self, hotam-dev) simply never declares this field,
// and the monotonicity/ordering machinery never activates for it.
//
// Duplicate stage names in the declared list are NOT de-duplicated here —
// that is a malformed declaration the invariant layer, not the loader,
// should surface as a violation (mirrors the split ResolveOrientationFAQ
// draws between "load-time honest no-op" and "check-time diagnosed
// violation" for a satisfiability defect on one entry). A blank/whitespace
// string entry, however, can never name a real stage, so — like
// ResolveOrientationFAQ dropping a Question-less entry — it is silently
// dropped at read time rather than preserved as an unsatisfiable stage.
func ResolveGateStageOrder(graphPath string) []string {
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
		GateStageOrder []string `json:"gate_stage_order"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		// manifest.json exists but is malformed JSON — honest no-op,
		// mirroring ResolveDiscipline's/ResolveOrientationFAQ's identical
		// malformed-manifest default.
		return nil
	}
	out := make([]string, 0, len(raw.GateStageOrder))
	for _, stage := range raw.GateStageOrder {
		if stage == "" {
			// A blank entry cannot name a real stage — drop it, keep the
			// rest (mirrors ResolveOrientationFAQ's per-entry honest no-op).
			continue
		}
		out = append(out, stage)
	}
	// Collapse every "nothing usably declared" outcome (absent field,
	// explicit empty list, all entries blank) to a nil return — the SAME
	// honest-no-op shape every sibling resolver already establishes, so a
	// consuming invariant's `len(order) == 0` no-op gate holds uniformly.
	if len(out) == 0 {
		return nil
	}
	return out
}
