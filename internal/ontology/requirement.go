package ontology

import "strings"

const (
	StatusDRAFT      = "DRAFT"
	StatusSETTLED    = "SETTLED"
	StatusREJECTED   = "REJECTED"
	StatusOPENPrefix = "OPEN"
)

const (
	EnforcementPROSE      = "PROSE"
	EnforcementSTRUCTURAL = "STRUCTURAL"
	EnforcementENFORCED   = "ENFORCED"
)

var EnforcementLevels = map[string]struct{}{
	EnforcementPROSE:      {},
	EnforcementSTRUCTURAL: {},
	EnforcementENFORCED:   {},
}

const (
	EnforceabilityENFORCEABLE      = "ENFORCEABLE"
	EnforceabilityINHERENTLY_PROSE = "INHERENTLY_PROSE"
)

var EnforceabilityKinds = map[string]struct{}{
	EnforceabilityENFORCEABLE:      {},
	EnforceabilityINHERENTLY_PROSE: {},
}

var RelationKinds = map[string]struct{}{
	"refines":    {},
	"depends_on": {},
	"replaces":   {},
}

type Relation struct {
	Kind   string `json:"kind"`
	Target string `json:"target"`
}

type HistoryEntry struct {
	At        string `json:"at"`
	Summary   string `json:"summary"`
	DecidedBy string `json:"decided_by"`
	// Signoff, when non-nil, carries the typed provenance (decided_by + date
	// + verbatim + instrument; chosen_variant is Conflict-variant-only and
	// MUST stay empty here — see ProposedRequirement/ProposedAssumptionRewrite
	// validate() in internal/proposal) for a real human decision this History
	// entry records. Purely additive and optional (omitempty) — the same
	// zero-migration pattern BlockedOn/ImplementedBy/VerifiedBy/GateSignoffs
	// already use on Requirement: every History entry landed before task #335
	// has no signoff field at all and round-trips byte-identically. This type
	// is SHARED across Requirement/Assumption/Axis/EntityType/Process
	// History, so the field is available to every one of those node kinds,
	// not just Requirement.
	Signoff *Signoff `json:"signoff,omitempty"`
}

type Requirement struct {
	ID    string `json:"id"`
	Claim string `json:"claim"`
	// ClaimTexts stores explicit authored wording by language; missing entries
	// are never substituted, and Claim must equal the declared default text.
	ClaimTexts     LocalizedText `json:"claim_texts,omitempty"`
	Owner          string        `json:"owner"`
	Status         string        `json:"status"`
	Why            string        `json:"why"`
	Assumptions    []string      `json:"assumptions"`
	Relations      []Relation    `json:"relations"`
	Enforcement    string        `json:"enforcement"`
	EnforcedBy     []string      `json:"enforced_by"`
	MTag           string        `json:"m_tag"`
	Enforceability string        `json:"enforceability"`
	Summary        string        `json:"summary"`
	CreatedAt      string        `json:"created_at"`
	SettledAt      string        `json:"settled_at"`
	LastReviewedAt string        `json:"last_reviewed_at"`
	ReviewAfter    string        `json:"review_after"`
	// Evidence is a RETIRED, superseded surface (task #342, R5-generate-
	// dont-lint inventory consult, resolver decision 2026-07-24): a legacy
	// free-text run-transcript/rationale-pointer field, almost universally
	// empty across the real graph. The mechanical proof it once informally
	// carried now lives entirely in typed carriers — EnforcedBy (check_*/
	// Test* names) and VerifiedBy (file:test refs), backed by
	// check_verified_by_test_passes actually EXECUTING each verified_by
	// test — so a new Evidence entry should NOT be hand-written; if a
	// rationale pointer is genuinely needed, it belongs in Why instead.
	// The field is kept (not removed, not omitempty) only for byte-identical
	// JSON compatibility with already-committed graph.json files: most
	// committed requirements already declare an explicit "evidence": [], and
	// adding omitempty would silently drop that key and break the byte-
	// identical round-trip tests (selfspec/merge_test.go) for ~90% of this
	// domain's requirements. Field position is ALSO load-bearing for byte
	// identity (encoding/json marshals in declaration order) — do not move it.
	Evidence   []string       `json:"evidence"`
	SourceRefs []string       `json:"source_refs"`
	History    []HistoryEntry `json:"history"`
	DeclOrder  int            `json:"decl_order"`
	// BlockedOn names the specific not-yet-built feature (a Planned tool from
	// internal/methodology/tools_data.go, or an absent Go package) that prevents
	// a real enforcement test from being written for this requirement TODAY,
	// even though it is otherwise ENFORCEABLE. Empty means "no known blocker —
	// this is real, actionable closeable debt, not feature-blocked roadmap"
	// (see docs/reviews/2026-07-13-c1-roadmap-debt-triage.md, the analytical
	// source for this field's initial backfill).
	BlockedOn string `json:"blocked_on,omitempty"`
	// ImplementedBy names WHERE this requirement is embodied in authored
	// domain code, as path-qualified `file:symbol` entries (e.g.
	// "spec/model/risk.go:NewRisk"). Orthogonal to EnforcedBy (which names
	// engine-side check_*/Test* enforcers by bare identifier): ImplementedBy
	// points into the domain's own authored spec/ layer. Purely additive and
	// optional (omitempty) — the same zero-migration pattern BlockedOn used —
	// resolution/verification of these entries is a separate concern (see
	// PLAN-authored-spec-discipline.md §4/§12).
	ImplementedBy []string `json:"implemented_by,omitempty"`
	// VerifiedBy names WHERE this requirement is PROVEN, as path-qualified
	// `file:test` entries (e.g. "spec/tests/risk_test.go:TestNewRisk_RejectsMissingOwner").
	// The authored-era counterpart of EnforcedBy: EnforcedBy stays for
	// engine-mechanism enforcers (registry check_* names, repo-wide Test*
	// scan); VerifiedBy carries explicit file-qualified authored tests.
	// Purely additive and optional (omitempty) — see
	// PLAN-authored-spec-discipline.md §4/§12.
	VerifiedBy []string `json:"verified_by,omitempty"`
	// GateSignoffs carries this requirement's per-stage gate-passage facts
	// (see GateSignoff in gate_signoff.go) — the single typed carrier for
	// "which staged-gate methodology stages has this requirement passed (or
	// had explicitly deferred), and in which pipeline run." Purely additive
	// and optional (omitempty) — the same zero-migration pattern BlockedOn/
	// ImplementedBy/VerifiedBy already use — a domain that has no staged-gate
	// methodology (no gate_stage_order in its manifest.json) never
	// populates this field and its JSON output is unchanged.
	GateSignoffs []GateSignoff `json:"gate_signoffs,omitempty"`
	// SourceLinks locate authored clauses backing this requirement. Source
	// identity, version, and hash live once on Graph.SpecificationSources;
	// each link names that source and an anchor: a Markdown heading fragment,
	// a single line (Lx), or a line range (Lx-Ly).
	SourceLinks []SourceLink `json:"source_links,omitempty"`
	// Coverage records an authored qualification only when coverage is
	// unsupported, unreachable, or unverified. Verified/discrepancy are
	// computed outcomes and must not be authored here.
	Coverage *CoverageDeclaration `json:"coverage,omitempty"`
	// AtomKind is empty for legacy value facts and "rule" for an explicitly
	// authored rule atom. Rule semantics are never inferred from test count.
	AtomKind string `json:"atom_kind,omitempty"`
	// AtomDiscovered is a provenance marker: it is set ONLY by atom discovery
	// (internal/selfspec discoverAtomsFromSnapshot) on requirements derived
	// from executed recorder runs — never authored by hand. Invariants use it
	// (instead of verified_by link-path heuristics) to exempt discovery-derived
	// nodes from the hand-registry bijection. omitempty keeps manual
	// requirements' serialized bytes unchanged.
	AtomDiscovered bool `json:"atom_discovered,omitempty"`
	// Cases may be projected from executed Go test cases; Input/Expected remain
	// independent declared values and are never replaced by SUT actual results.
	Cases []CaseDefinition `json:"cases,omitempty"`
	// ClauseLinks bind this atom to declared source-owned clauses and sides.
	ClauseLinks []ClauseLink `json:"clause_links,omitempty"`
	// Strength is the optional authored MUST/SHOULD/MAY obligation level.
	Strength string `json:"strength,omitempty"`
	// Applicability is nil for universal applicability.
	Applicability *Applicability `json:"applicability,omitempty"`
	// Precedence declares scoped strict order, not dependency or file order.
	Precedence []PrecedenceLink `json:"precedence,omitempty"`
}

func (r *Requirement) UnmarshalJSON(data []byte) error {
	type wire Requirement
	var value wire
	if _, err := decodeStrictObject(data, "requirement", &value, nil,
		[]string{"claim_texts", "atom_kind", "cases", "clause_links", "strength", "applicability", "precedence"}); err != nil {
		return err
	}
	*r = Requirement(value)
	return nil
}

func (r Requirement) IsCloseableDebt() bool {
	return r.Enforcement != EnforcementENFORCED && r.Enforceability == EnforceabilityENFORCEABLE
}

// IsCloseableDebtNow is the actionable subset of closeable debt: a real test
// could be written for it TODAY if someone did the work (no missing feature
// blocks it). Mutually exclusive with IsFeatureBlockedDebt; their union equals
// IsCloseableDebt.
func (r Requirement) IsCloseableDebtNow() bool {
	return r.IsCloseableDebt() && r.BlockedOn == ""
}

// IsFeatureBlockedDebt is the honestly-documented roadmap subset of closeable
// debt: the requirement describes a feature that does not exist yet, so no real
// enforcement test is possible until the blocking feature is built. See
// R-speculative-aspects-frozen.
func (r Requirement) IsFeatureBlockedDebt() bool {
	return r.IsCloseableDebt() && r.BlockedOn != ""
}

func (r Requirement) IsOpen() bool {
	return strings.HasPrefix(r.Status, StatusOPENPrefix)
}
