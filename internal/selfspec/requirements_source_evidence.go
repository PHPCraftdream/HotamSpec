package selfspec

import "github.com/PHPCraftdream/HotamSpec/internal/ontology"

var _ = Requirements.MustRegister("R-specification-source-bytes-current", ontology.Requirement{
	ID:    "R-specification-source-bytes-current",
	Claim: "An explicitly declared specification source carries its identity, path, declared version and SHA-256; its current bytes are checked against that digest. Missing, invalid and drifted sources are reported as structural failures, not as semantic proof or an invented version.",
	Owner: "framework-author", Status: "SETTLED",
	Why:         "Source drift must remain visible when executable requirements cite a frozen external specification. The new duty activates through specification_sources, not a pre-existing discipline flag.",
	Enforcement: "ENFORCED", Enforceability: "ENFORCEABLE",
	EnforcedBy: []string{"check_specification_sources_current"},
	CreatedAt:  "2026-10-02", SettledAt: "2026-10-02",
	SourceRefs: []string{"internal/source/source.go", "internal/invariants/source_evidence.go", "internal/ontology/source_evidence.go"},
})

var _ = Requirements.MustRegister("R-source-clause-links-resolve", ontology.Requirement{
	ID:    "R-source-clause-links-resolve",
	Claim: "An explicitly authored source_links entry resolves to a declared source and an existing heading or line range. A valid source-to-method-to-test link establishes traceability, not automatic semantic completeness.",
	Owner: "framework-author", Status: "SETTLED",
	Why:         "SourceLinks provide the typed seam from a source clause to its executable carrier and recorded observations. Legacy free-text SourceRefs retain their existing meaning.",
	Enforcement: "ENFORCED", Enforceability: "ENFORCEABLE",
	EnforcedBy: []string{"check_source_links_resolve"},
	CreatedAt:  "2026-10-02", SettledAt: "2026-10-02",
	SourceRefs: []string{"internal/source/source.go", "internal/invariants/source_evidence.go", "internal/ontology/source_evidence.go"},
})

var _ = Requirements.MustRegister("R-coverage-qualification-is-authored", ontology.Requirement{
	ID:    "R-coverage-qualification-is-authored",
	Claim: "Coverage declarations qualify unsupported recommendations, unreachable profile-specific branches, or unverified obligations with explicit rationale. An unreachable declaration includes its profile and source links; verified and discrepancy states come from execution evidence, not an authored passing verdict.",
	Owner: "framework-author", Status: "SETTLED",
	Why:         "A declared limitation or domain proof differs from an observed test verdict and must not hide an actual discrepancy. The qualification duty activates only for the new explicit Coverage field.",
	Enforcement: "ENFORCED", Enforceability: "ENFORCEABLE",
	EnforcedBy: []string{"check_coverage_declarations_justified"},
	CreatedAt:  "2026-10-02", SettledAt: "2026-10-02",
	SourceRefs: []string{"internal/source/source.go", "internal/invariants/source_evidence.go", "internal/ontology/source_evidence.go"},
})
