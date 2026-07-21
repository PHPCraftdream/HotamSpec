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
}

type Requirement struct {
	ID             string         `json:"id"`
	Claim          string         `json:"claim"`
	Owner          string         `json:"owner"`
	Status         string         `json:"status"`
	Why            string         `json:"why"`
	Assumptions    []string       `json:"assumptions"`
	Relations      []Relation     `json:"relations"`
	Enforcement    string         `json:"enforcement"`
	EnforcedBy     []string       `json:"enforced_by"`
	MTag           string         `json:"m_tag"`
	Enforceability string         `json:"enforceability"`
	Summary        string         `json:"summary"`
	CreatedAt      string         `json:"created_at"`
	SettledAt      string         `json:"settled_at"`
	LastReviewedAt string         `json:"last_reviewed_at"`
	ReviewAfter    string         `json:"review_after"`
	Evidence       []string       `json:"evidence"`
	SourceRefs     []string       `json:"source_refs"`
	History        []HistoryEntry `json:"history"`
	DeclOrder      int            `json:"decl_order"`
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
