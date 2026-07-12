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
	EnforceabilityENFORCEABLE     = "ENFORCEABLE"
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
}

func (r Requirement) IsCloseableDebt() bool {
	return r.Enforcement != EnforcementENFORCED && r.Enforceability == EnforceabilityENFORCEABLE
}

func (r Requirement) IsOpen() bool {
	return strings.HasPrefix(r.Status, StatusOPENPrefix)
}
