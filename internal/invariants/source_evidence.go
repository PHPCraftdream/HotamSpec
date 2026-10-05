package invariants

import (
	"sort"

	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/source"
)

func checkSpecificationSourcesCurrent(g *ontology.Graph) []Violation {
	if g == nil || len(g.SpecificationSources) == 0 {
		return nil
	}
	var out []Violation
	for _, check := range source.VerifySources(g) {
		if check.Status == source.StatusVerified {
			continue
		}
		out = append(out, Violation{
			Check:   "check_specification_sources_current",
			ID:      check.SourceID,
			Message: check.Message,
		})
	}
	return orderedSourceViolations(out)
}

func checkSourceLinksResolve(g *ontology.Graph) []Violation {
	if g == nil {
		return nil
	}
	var out []Violation
	for i, req := range g.Requirements {
		if len(req.SourceLinks) == 0 {
			continue
		}
		oneRequirement := *g
		oneRequirement.Requirements = g.Requirements[i : i+1]
		for _, check := range source.VerifyLinks(&oneRequirement) {
			if check.Status == source.StatusVerified {
				continue
			}
			out = append(out, Violation{
				Check:   "check_source_links_resolve",
				ID:      req.ID,
				Message: check.Message,
			})
		}
	}
	return orderedSourceViolations(out)
}

func checkCoverageDeclarationsJustified(g *ontology.Graph) []Violation {
	if g == nil {
		return nil
	}
	var out []Violation
	for _, req := range g.Requirements {
		if req.Coverage == nil {
			continue
		}
		if err := source.ValidateCoverage(req); err != nil {
			out = append(out, Violation{
				Check:   "check_coverage_declarations_justified",
				ID:      req.ID,
				Message: err.Error(),
			})
		}
	}
	return orderedSourceViolations(out)
}

func orderedSourceViolations(out []Violation) []Violation {
	sort.Slice(out, func(i, j int) bool {
		if out[i].ID != out[j].ID {
			return out[i].ID < out[j].ID
		}
		return out[i].Message < out[j].Message
	})
	return out
}

var _ = All.MustRegister("check_specification_sources_current", Invariant{
	Name:  "check_specification_sources_current",
	Canon: methodology.Domain,
	Claim: "every explicitly declared specification source has an ID, path, version, and hash, and its current bytes match that hash.",
	Rule:  "When specification_sources is non-empty, require complete metadata and compare each current file's SHA-256 with the declaration.",
	Why:   "A source link with missing or drifted bytes cannot be treated as current evidence.",
	Check: checkSpecificationSourcesCurrent,
})

var _ = All.MustRegister("check_source_links_resolve", Invariant{
	Name:  "check_source_links_resolve",
	Canon: methodology.Requirement,
	Claim: "every explicitly authored requirement source_link names a declared source and a real anchor.",
	Rule:  "For requirements with source_links, resolve source IDs and heading or line-range anchors structurally.",
	Why:   "A citation must point to bytes and a location that exist; this check makes no semantic-completeness claim.",
	Check: checkSourceLinksResolve,
})

var _ = All.MustRegister("check_coverage_declarations_justified", Invariant{
	Name:  "check_coverage_declarations_justified",
	Canon: methodology.Requirement,
	Claim: "every authored coverage qualification has its required rationale, profile, and source links.",
	Rule:  "Validate only explicit unsupported_recommendation, unreachable, or unverified declarations; verified and discrepancy are computed outcomes.",
	Why:   "Authored rationale and scope keep a qualified coverage claim honest without inferring its semantics.",
	Check: checkCoverageDeclarationsJustified,
})
