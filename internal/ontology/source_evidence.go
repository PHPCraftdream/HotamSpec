package ontology

// SpecificationSource identifies immutable source bytes, not a semantic proof.
type SpecificationSource struct {
	ID      string `json:"id"`
	Path    string `json:"path"`
	Version string `json:"version"`
	SHA256  string `json:"sha256"`
}

// SourceLink locates a clause in a named source. Anchor is a heading or Lx-Ly range.
type SourceLink struct {
	SourceID string `json:"source_id"`
	Anchor   string `json:"anchor,omitempty"`
}

const (
	CoverageVerified    = "verified"
	CoverageDiscrepancy = "discrepancy"
	CoverageUnsupported = "unsupported_recommendation"
	CoverageUnreachable = "unreachable"
	CoverageUnverified  = "unverified"
)

// CoverageDeclaration records an authored qualification, never an executed verdict.
type CoverageDeclaration struct {
	Status    string `json:"status"`
	Rationale string `json:"rationale"`
	Profile   string `json:"profile,omitempty"`
}
