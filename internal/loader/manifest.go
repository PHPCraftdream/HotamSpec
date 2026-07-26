package loader

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// DomainManifest is the typed, in-memory projection of a domain's manifest.json
// — the manifest analogue of ontology.Graph for graph.json. Phase 1 of task
// #341 (R5-manifest-object): the "requirements-as-code" wave (#343-#347)
// already proved the "object is primary, JSON is a serialization/projection"
// discipline for Requirements (internal/selfspec registry ↔ graph.json via
// MergeIntoGraph / hotam sync-self). This struct applies the SAME architectural
// pattern to manifest.json, which until now had NO single typed object: every
// field was read piecemeal by an independent Resolve* function that re-opened
// and re-parsed the file for one key each (resolveSelfHosting, ResolveDiscipline,
// ResolveGenProfile, ResolveRequireProvenance, ResolveParent,
// ResolveDomainPresentation, ResolveGateStageOrder, ResolveGateCohort,
// ResolveOrientationFAQ). DomainManifest unifies those into one typed value a
// caller can hold, inspect, and (in later phases) mutate through a typed path.
//
// PHASE 1 SCOPE (this task) — deliberately narrow, exactly as RAC-A/B phased the
// requirements migration:
//
//   - READ + REPRODUCE only. LoadManifest reads manifest.json into a
//     DomainManifest; WriteManifest re-serializes it. A byte-identical
//     round-trip test on the two real committed manifests
//     (domains/hotam-spec-self, domains/hotam-dev) proves the typed object is a
//     faithful projection — the same proof internal/selfspec/merge_test.go's
//     TestMergeIntoGraph_ByteIdenticalRoundTrip provides for Requirements.
//   - NO authority flip. The Resolve* functions remain the runtime authority;
//     nothing is rewired to read through DomainManifest yet. It is a parallel,
//     proven-byte-identical typed surface, exactly as the selfspec Requirements
//     registry was a proven mirror before RAC-B flipped authority.
//   - NO mutation layer, NO hand-edit guard. The Proposed*-style typed mutation
//   objects (analogous to internal/proposal for graph nodes) and the
//     R-no-hand-edit-graph analogue that would forbid editing manifest.json
//   outside the typed path are the NEXT phases (see
//   internal/loader/drafts/manifest-mutation-phases.go). Phase 1 does not block
//   hand-editing manifest.json — the file stays freely hand-authored, as it is
//   today.
//
// FIELD-DECLARATION ORDER IS LOAD-BEARING for byte identity. Go's
// encoding/json marshals struct fields in declaration order (NOT alphabetical),
// so the order below matches the on-disk field order of the real committed
// manifests: both domains/hotam-spec-self and domains/hotam-dev share the
// prefix self_hosting, purpose, goals, director, parent (hotam-spec-self then
// adds orientation_faq). The optional fields that neither real manifest carries
// today (charter, discipline, gen_profile, require_provenance, gate_stage_order,
// gate_cohort) are declared after that shared prefix with omitempty, so they
// are absent from the round-tripped bytes of the real files — byte-identical.
//
// AUTHOR INPUT vs DERIVED/COMPUTED (brief Q3, resolved here): EVERY field in
// manifest.json is an AUTHOR field — a human (the resolver/domain author)
// edits its meaning directly; none is computed or derived from graph state.
// This differs from graph.json, which mixes authored structural fields with
// event fields (History, GateSignoffs, reviews) that are appended by tooling.
// manifest.json has no event/derived fields at all: it is a pure authored
// declaration of domain identity and methodology opt-ins. Consequently the
// whole DomainManifest is one homogeneous "authored input" surface; the
// author-vs-derived split that RAC's structural/event distinction draws for
// Requirements does not apply here. (The DomainPresentation sub-struct is an
// existing READER-side convenience that groups four of these authored fields —
// purpose/goals/director/charter — for the DOMAIN-MAP renderer; it is NOT a
// derived/computed subset.)
//
// PHASE 1 LIMITATIONS (documented, not fixed — they are later phases' scope):
//
//   - Unknown fields are dropped on round-trip. LoadManifest uses a LENIENT
//     decoder (no DisallowUnknownFields), matching every existing Resolve*
//     function's tolerant contract, so a manifest carrying a field this struct
//     does not model still loads — but that field is lost on WriteManifest.
//     The two real manifests carry no unknown fields, so byte identity holds for
//     them; a strict (DisallowUnknownFields) read is a Phase-2 hardening option.
//   - Parent absent-vs-null is not distinguished. Parent is *string; both a
//     missing "parent" key and an explicit `"parent": null` unmarshal to nil,
//     and WriteManifest always emits `"parent": null` (no omitempty, to
//     reproduce hotam-spec-self's explicit null). The ResolveParent reader
//     (which DOES distinguish key-absent from key-present-with-null via a
//     map[string]json.RawMessage probe) remains the authority for that
//     distinction until a Phase-2 mutation layer needs it on DomainManifest.
//     Both real manifests declare parent, so this is not a fidelity gap for them.
//   - self_hosting is always emitted (no omitempty), so a manifest authored
//     without the key would GAIN `"self_hosting": false` on round-trip. Both
//     real manifests declare it, so byte identity holds for them.
type DomainManifest struct {
	// SelfHosting is the domain's manifest.json "self_hosting" flag
	// (resolveSelfHosting), populated by LoadGraph into ontology.Graph.SelfHosting.
	// NO omitempty: hotam-dev declares `"self_hosting": false` explicitly, and
	// omitting a false zero-value would drop that declaration on round-trip.
	SelfHosting bool `json:"self_hosting"`

	// Purpose is the one-line DOMAIN-MAP description
	// (ResolveDomainPresentation.Purpose).
	Purpose string `json:"purpose,omitempty"`

	// Goals is the DOMAIN-MAP bullet list
	// (ResolveDomainPresentation.Goals).
	Goals []string `json:"goals,omitempty"`

	// Director is the accountable resolver role/name
	// (ResolveDomainPresentation.Director).
	Director string `json:"director,omitempty"`

	// Parent names the parent domain, or nil for a root domain
	// (ResolveParent). *string without omitempty so an explicit JSON null
	// (hotam-spec-self's root declaration) round-trips as `"parent": null`,
	// not as an omitted key.
	Parent *string `json:"parent"`

	// OrientationFAQ is the optional orientation-question list
	// (ResolveOrientationFAQ). Reuses OrientationFAQEntry directly; its
	// Keywords field carries omitempty (added in the same task #341 wave)
	// precisely so a nil Keywords slice re-marshals as an omitted key — the
	// one tag adjustment byte identity required.
	OrientationFAQ []OrientationFAQEntry `json:"orientation_faq,omitempty"`

	// Charter is the optional one-line nature statement
	// (ResolveDomainPresentation.Charter).
	Charter string `json:"charter,omitempty"`

	// Discipline is the optional methodology-discipline opt-in
	// (ResolveDiscipline); the single recognized non-empty value is
	// DisciplineFull ("full").
	Discipline string `json:"discipline,omitempty"`

	// GenProfile is the optional gen-spec profile
	// (ResolveGenProfile); GenProfileFull or GenProfileConsumer.
	GenProfile string `json:"gen_profile,omitempty"`

	// RequireProvenance is the optional provenance-gate opt-in
	// (ResolveRequireProvenance).
	RequireProvenance bool `json:"require_provenance,omitempty"`

	// GateStageOrder is the optional ordered stage vocabulary for gate-signoff
	// monotonicity checks (ResolveGateStageOrder).
	GateStageOrder []string `json:"gate_stage_order,omitempty"`

	// GateCohort is the optional cohort spec for gate-signoff-count asserts
	// (ResolveGateCohort).
	GateCohort *GateCohortSpec `json:"gate_cohort,omitempty"`

	// RequirementsAuthority is the optional "requirements_authority" opt-in
	// (resolveRequirementsAuthorityCode, task #367/RAC2 Phase C). The single
	// recognized non-empty value is RequirementsAuthorityCode ("code"),
	// mirroring Discipline's DisciplineFull ("full") pattern above: it flips a
	// CONSUMER domain's applyToGraph Requirement/Rejection lock on, the same
	// lock SelfHosting flips on for hotam-spec-self itself.
	RequirementsAuthority string `json:"requirements_authority,omitempty"`
}

// LoadManifest reads and decodes the manifest.json at path into a DomainManifest.
// The decode is LENIENT (no DisallowUnknownFields) to match every existing
// Resolve* function's tolerant contract: a manifest carrying a field this struct
// does not model still loads (the unknown field is silently ignored, and would
// be dropped on a subsequent WriteManifest — see DomainManifest's doc comment).
//
// Unlike the Resolve* functions (which each tolerate a missing manifest by
// returning a zero-value default), LoadManifest returns an error when the file
// cannot be read or parsed — it is an explicit load of an explicit path, not a
// tolerant probe. Callers that need the missing-manifest honest-no-op behavior
// continue to use the Resolve* functions.
func LoadManifest(path string) (*DomainManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load manifest: read %s: %w", path, err)
	}
	var m DomainManifest
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("load manifest: decode %s: %w", path, err)
	}
	return &m, nil
}

// WriteManifest re-serializes m to manifest.json at path, through the SAME
// canonical encoder graph.json uses (SetEscapeHTML(false) + 2-space indent), so
// a DomainManifest loaded from a real manifest.json round-trips byte-identically.
// The write is atomic (write-to-temp + rename), reusing atomicWriteFile — the
// same crash-safe writer WriteGraph uses. No lock is written (manifest.json has
// no graph.lock-style content pin today; a hand-edit guard is a later phase).
func WriteManifest(path string, m *DomainManifest) error {
	if m == nil {
		return fmt.Errorf("write manifest: nil manifest")
	}
	data, err := marshalManifest(m)
	if err != nil {
		return fmt.Errorf("write manifest: marshal: %w", err)
	}
	if err := atomicWriteFile(path, data); err != nil {
		return fmt.Errorf("write manifest: %s: %w", path, err)
	}
	return nil
}

// marshalManifest encodes m through the canonical encoder (no HTML escaping,
// 2-space indent, trailing newline from Encoder.Encode), the exact serialization
// shape graph.json's marshalCanonical establishes. Field order is DomainManifest's
// own struct declaration order (see that type's doc comment for why order is
// load-bearing for byte identity).
func marshalManifest(m *DomainManifest) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(m); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ManifestPath returns the manifest.json path sitting next to the given
// graph.json path — the same path computation every Resolve* function performs
// inline (filepath.Join(filepath.Dir(graphPath), "manifest.json")), factored out
// here so LoadManifest/WriteManifest callers and future mutation code share one
// resolution point.
func ManifestPath(graphPath string) string {
	return filepath.Join(filepath.Dir(graphPath), "manifest.json")
}
