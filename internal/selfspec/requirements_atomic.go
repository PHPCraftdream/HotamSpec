// Hand-maintained canonical requirements for the atomic multilingual and
// conformance checks. Keep these Go declarations in sync with their graph
// projection through `hotam sync-self`.
package selfspec

import "github.com/PHPCraftdream/HotamSpec/internal/ontology"

var _ = Requirements.MustRegister("R-language-bundle-complete", ontology.Requirement{
	ID:             "R-language-bundle-complete",
	Claim:          "For a domain declaring more than one language, each atom has exactly one non-empty source phrase for every declared language; malformed, duplicate, unknown, or missing language blocks are reported. This is structural translation completeness, not semantic equivalence.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "The language bundle has its own translation-completeness trigger; enabling an older atom or scenario mode must not silently impose a later obligation.",
	Relations:      []ontology.Relation{{Kind: "refines", Target: "R-opt-in-trigger-owns-its-own-obligations"}},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"check_language_bundle_complete"},
	Enforceability: "ENFORCEABLE",
	CreatedAt:      "2026-10-04",
	SettledAt:      "2026-10-04",
	SourceRefs:     []string{"docs/PLAN-atomic-multilingual-spec.md", "docs/AUTHORED-SPEC-CONTRACT.md"},
	DeclOrder:      0,
	ImplementedBy:  []string{"internal/invariants/language_bundle.go:checkLanguageBundleComplete"},
})

var _ = Requirements.MustRegister("R-language-outputs-current", ontology.Requirement{
	ID:             "R-language-outputs-current",
	Claim:          "Whenever a domain explicitly declares a language list, the expected generated language bundle is fresh across every emitted localized document, SPEC index and shard, and boot crystal; the one-language layout keeps its existing filenames.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "Freshness must cover the complete expected output inventory rather than only the legacy SPEC.md; explicit single-language locales also change service-rendered content and have their own freshness trigger.",
	Relations:      []ontology.Relation{{Kind: "refines", Target: "R-opt-in-trigger-owns-its-own-obligations"}},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"check_language_outputs_current"},
	Enforceability: "ENFORCEABLE",
	CreatedAt:      "2026-10-04",
	SettledAt:      "2026-10-04",
	SourceRefs:     []string{"docs/PLAN-atomic-multilingual-spec.md", "docs/AUTHORED-SPEC-CONTRACT.md"},
	DeclOrder:      0,
	ImplementedBy:  []string{"cmd/hotam/language_freshness_wiring.go:checkLanguageOutputsCurrentReal"},
})

var _ = Requirements.MustRegister("R-conformance-audit-own-fields", ontology.Requirement{
	ID:             "R-conformance-audit-own-fields",
	Claim:          "The conformance audit is triggered only by its own declared rule_cases, clauses, profiles, compositions, or requirement atom_kind, cases, clause_links, strength, applicability, or precedence fields. It validates and reports declared structural conformance evidence without claiming semantic completeness, corpus-wide compliance, or automatic implementation blame.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "Clause inventories, rule cases, profiles and composition provenance introduce distinct obligations; an existing language, discipline, or self-executing opt-in is not consent to those obligations.",
	Relations:      []ontology.Relation{{Kind: "refines", Target: "R-opt-in-trigger-owns-its-own-obligations"}},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"check_conformance_audit"},
	Enforceability: "ENFORCEABLE",
	CreatedAt:      "2026-10-04",
	SettledAt:      "2026-10-04",
	SourceRefs:     []string{"docs/PLAN-atomic-conformance-spec.md", "docs/AUTHORED-SPEC-CONTRACT.md"},
	DeclOrder:      0,
	ImplementedBy:  []string{"internal/invariants/conformance_audit.go:checkConformanceAudit"},
})
