package invariants

import (
	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func checkDomainManifestExistsAndImportable(g *ontology.Graph) []Violation {
	return nil
}

var _ = All.MustRegister("check_domain_manifest_exists_and_importable", Invariant{
	Name:  "check_domain_manifest_exists_and_importable",
	Canon: methodology.Domain,
	Claim: "every domains/<name>/manifest exists and can be loaded (honest no-op in the Go port).",
	Rule: "RULE (original Python): a domain directory MUST contain a manifest.py that can be loaded without error. " +
		"Missing or unloadable manifests make the domain invisible to the framework (R-domain-has-manifest). " +
		"The Python source walked the domains directory, found each domain subdirectory, checked manifest.py exists, " +
		"and attempted importlib.util.spec_from_file_location + exec_module to confirm the module loads. " +
		"This invariant is an HONEST NO-OP in the Go port.",
	Why: "this invariant checks the FILE SYSTEM STRUCTURE of a domain on disk (does manifest.py or manifest.json exist " +
		"and is it loadable), not the shape of the in-memory graph. The Go invariant contract (Check func(*ontology.Graph) " +
		"[]Violation) operates on an already-loaded in-memory graph and has no access to the domain's directory path on " +
		"disk. Filesystem-coherence checks of this kind belong to a separate domain-scaffolding or create_domain layer " +
		"(future work outside internal/invariants), which has access to the filesystem and the build pipeline -- the same " +
		"architectural boundary as check_doc_reader_resolves_to_stakeholder and check_entities_md_lists_all_types. " +
		"Porting this as a graph-only invariant would require adding a filesystem path parameter (breaking the uniform " +
		"Check signature) or re-deriving the manifest from graph data (which defeats the purpose of checking the file). " +
		"References: R-domain-has-manifest.",
	Check: checkDomainManifestExistsAndImportable,
})

func checkDomainManifestIdMatchesDirname(g *ontology.Graph) []Violation {
	return nil
}

var _ = All.MustRegister("check_domain_manifest_id_matches_dirname", Invariant{
	Name:  "check_domain_manifest_id_matches_dirname",
	Canon: methodology.Domain,
	Claim: "every domains/<name>/manifest ID field matches the directory name (honest no-op in the Go port).",
	Rule: "RULE (original Python): manifest.py MUST define an ID attribute equal to the directory name. A mismatched ID " +
		"breaks the identity anchor for gen_spec and create_domain tooling. " +
		"This invariant is an HONEST NO-OP in the Go port.",
	Why: "this invariant checks the FILE SYSTEM STRUCTURE of a domain on disk: it loads each domain's manifest.py via " +
		"importlib, reads its ID attribute, and compares it to the directory name. The Go invariant contract " +
		"(Check func(*ontology.Graph) []Violation) operates on an already-loaded in-memory graph and has no access to the " +
		"domain's directory path or manifest module on disk. Filesystem-coherence checks belong to a separate " +
		"domain-scaffolding layer (future work outside internal/invariants), the same boundary as " +
		"check_doc_reader_resolves_to_stakeholder and check_entities_md_lists_all_types. " +
		"References: R-domain-has-manifest.",
	Check: checkDomainManifestIdMatchesDirname,
})

func checkDomainManifestDescriptionNonempty(g *ontology.Graph) []Violation {
	return nil
}

var _ = All.MustRegister("check_domain_manifest_description_nonempty", Invariant{
	Name:  "check_domain_manifest_description_nonempty",
	Canon: methodology.Domain,
	Claim: "every domains/<name>/manifest DESCRIPTION is non-empty (honest no-op in the Go port).",
	Rule: "RULE (original Python): manifest.py MUST define a non-empty DESCRIPTION attribute. A domain without a " +
		"description is undocumented and invisible to human readers. " +
		"This invariant is an HONEST NO-OP in the Go port.",
	Why: "this invariant checks the FILE SYSTEM STRUCTURE of a domain on disk: it loads each domain's manifest.py via " +
		"importlib and reads its DESCRIPTION attribute. The Go invariant contract " +
		"(Check func(*ontology.Graph) []Violation) operates on an already-loaded in-memory graph and has no access to the " +
		"domain's directory path or manifest module on disk. Filesystem-coherence checks belong to a separate " +
		"domain-scaffolding layer (future work outside internal/invariants), the same boundary as " +
		"check_doc_reader_resolves_to_stakeholder and check_entities_md_lists_all_types. " +
		"References: R-domain-has-manifest.",
	Check: checkDomainManifestDescriptionNonempty,
})

func checkDomainManifestGoalsNonempty(g *ontology.Graph) []Violation {
	return nil
}

var _ = All.MustRegister("check_domain_manifest_goals_nonempty", Invariant{
	Name:  "check_domain_manifest_goals_nonempty",
	Canon: methodology.Domain,
	Claim: "every domains/<name>/manifest GOALS is non-empty (honest no-op in the Go port).",
	Rule: "RULE (original Python): manifest.py MUST define a non-empty GOALS attribute. A domain without declared goals " +
		"has no visible intent and cannot drive the burn-down meter. " +
		"This invariant is an HONEST NO-OP in the Go port.",
	Why: "this invariant checks the FILE SYSTEM STRUCTURE of a domain on disk: it loads each domain's manifest.py via " +
		"importlib and reads its GOALS attribute. The Go invariant contract " +
		"(Check func(*ontology.Graph) []Violation) operates on an already-loaded in-memory graph and has no access to the " +
		"domain's directory path or manifest module on disk. Filesystem-coherence checks belong to a separate " +
		"domain-scaffolding layer (future work outside internal/invariants), the same boundary as " +
		"check_doc_reader_resolves_to_stakeholder and check_entities_md_lists_all_types. " +
		"References: R-domain-has-manifest.",
	Check: checkDomainManifestGoalsNonempty,
})

func checkDomainManifestDirectorNonempty(g *ontology.Graph) []Violation {
	return nil
}

var _ = All.MustRegister("check_domain_manifest_director_nonempty", Invariant{
	Name:  "check_domain_manifest_director_nonempty",
	Canon: methodology.Domain,
	Claim: "every domains/<name>/manifest DIRECTOR is non-empty (honest no-op in the Go port).",
	Rule: "RULE (original Python): manifest.py MUST define a non-empty DIRECTOR attribute naming the director agent. " +
		"A domain without a declared director is headless. " +
		"This invariant is an HONEST NO-OP in the Go port.",
	Why: "this invariant checks the FILE SYSTEM STRUCTURE of a domain on disk: it loads each domain's manifest.py via " +
		"importlib and reads its DIRECTOR attribute. The Go invariant contract " +
		"(Check func(*ontology.Graph) []Violation) operates on an already-loaded in-memory graph and has no access to the " +
		"domain's directory path or manifest module on disk. Filesystem-coherence checks belong to a separate " +
		"domain-scaffolding layer (future work outside internal/invariants), the same boundary as " +
		"check_doc_reader_resolves_to_stakeholder and check_entities_md_lists_all_types. " +
		"References: R-domain-declares-director.",
	Check: checkDomainManifestDirectorNonempty,
})

func checkDomainManifestValid(g *ontology.Graph) []Violation {
	var out []Violation
	out = append(out, checkDomainManifestExistsAndImportable(g)...)
	out = append(out, checkDomainManifestIdMatchesDirname(g)...)
	out = append(out, checkDomainManifestDescriptionNonempty(g)...)
	out = append(out, checkDomainManifestGoalsNonempty(g)...)
	out = append(out, checkDomainManifestDirectorNonempty(g)...)
	return out
}

var _ = All.MustRegister("check_domain_manifest_valid", Invariant{
	Name:  "check_domain_manifest_valid",
	Canon: methodology.Domain,
	Claim: "every domains/<name>/manifest defines ID (matching dirname), DESCRIPTION, GOALS, DIRECTOR (thin delegator, honest no-op in the Go port).",
	Rule: "RULE (original Python): a domain without a valid manifest is invisible to the framework. This is a THIN " +
		"DELEGATOR -- calls check_domain_manifest_exists_and_importable, check_domain_manifest_id_matches_dirname, " +
		"check_domain_manifest_description_nonempty, check_domain_manifest_goals_nonempty, " +
		"check_domain_manifest_director_nonempty and concatenates. The atomic sub-checks are registered individually. " +
		"Because all five sub-checks are honest no-ops in the Go port (they check filesystem structure, not graph shape), " +
		"this delegator is also an honest no-op.",
	Why: "the manifest is the stable identity anchor for a domain; missing or mismatched fields make the domain " +
		"undiscoverable by gen_spec and create_domain tooling (R-domain-has-manifest). The Go invariant contract " +
		"(Check func(*ontology.Graph) []Violation) operates on an already-loaded in-memory graph and has no access to " +
		"the domain's directory path or manifest module on disk. Filesystem-coherence checks belong to a separate " +
		"domain-scaffolding layer (future work outside internal/invariants), the same boundary as " +
		"check_doc_reader_resolves_to_stakeholder and check_entities_md_lists_all_types. " +
		"References: R-domain-has-manifest.",
	Check:       checkDomainManifestValid,
	IsDelegator: true,
})

func checkDomainDirectorExists(g *ontology.Graph) []Violation {
	return nil
}

var _ = All.MustRegister("check_domain_director_exists", Invariant{
	Name:  "check_domain_director_exists",
	Canon: methodology.Domain,
	Claim: "every domains/<name>/agents/<DIRECTOR>/scope must exist (honest no-op in the Go port).",
	Rule: "RULE (original Python): a domain whose declared director agent is missing is headless. The Python source " +
		"loaded each domain's manifest.py, read the DIRECTOR attribute, and checked that " +
		"domains/<name>/agents/<DIRECTOR>/scope.py exists on disk. " +
		"This invariant is an HONEST NO-OP in the Go port.",
	Why: "the director is the entry point for all domain-level operator delegation (R-domain-declares-director). " +
		"This invariant checks the FILE SYSTEM STRUCTURE of a domain on disk (does the director agent's scope.py exist), " +
		"not the shape of the in-memory graph. The Go invariant contract (Check func(*ontology.Graph) []Violation) " +
		"operates on an already-loaded in-memory graph and has no access to the domain's directory path or manifest " +
		"module on disk. Filesystem-coherence checks belong to a separate domain-scaffolding layer (future work outside " +
		"internal/invariants), the same boundary as check_doc_reader_resolves_to_stakeholder and " +
		"check_entities_md_lists_all_types. References: R-domain-declares-director.",
	Check: checkDomainDirectorExists,
})

func checkAgentHasAgentsSubdir(g *ontology.Graph) []Violation {
	return nil
}

var _ = All.MustRegister("check_agent_has_agents_subdir", Invariant{
	Name:  "check_agent_has_agents_subdir",
	Canon: methodology.Agent,
	Claim: "every agent directory must contain an 'agents/' subdirectory (honest no-op in the Go port).",
	Rule: "RULE (original Python): every agent is itself a potential director that can spawn sub-agents; the agents/ " +
		"subdir is the recursion slot (R-agent-is-recursive-director). The Python source walked the spec/agents or " +
		"domains/<name>/agents/director/agents directory, found each agent subdirectory (identified by scope.py), and " +
		"checked that an agents/ subdirectory exists inside it. " +
		"This invariant is an HONEST NO-OP in the Go port.",
	Why: "without the agents/ subdir the recursive delegation pattern collapses -- create_agent always scaffolds it; " +
		"its absence indicates manual corruption. This invariant checks the FILE SYSTEM STRUCTURE of agent directories " +
		"on disk, not the shape of the in-memory graph. The Go invariant contract (Check func(*ontology.Graph) []Violation) " +
		"operates on an already-loaded in-memory graph and has no access to the agents directory path on disk. " +
		"Filesystem-coherence checks belong to a separate agent-scaffolding layer (future work outside " +
		"internal/invariants), the same boundary as check_doc_reader_resolves_to_stakeholder and " +
		"check_entities_md_lists_all_types. References: R-agent-is-recursive-director.",
	Check: checkAgentHasAgentsSubdir,
})

func checkAgentHasDocsSubdir(g *ontology.Graph) []Violation {
	return nil
}

var _ = All.MustRegister("check_agent_has_docs_subdir", Invariant{
	Name:  "check_agent_has_docs_subdir",
	Canon: methodology.Agent,
	Claim: "every agent directory must contain a 'docs/' subdirectory (honest no-op in the Go port).",
	Rule: "RULE (original Python): every agent carries its own docs/ for generated CLAUDE.md and thinking fragments " +
		"scoped to its domain (R-agent-has-docs-dir). The Python source walked the agents directory, found each agent " +
		"subdirectory (identified by scope.py), and checked that a docs/ subdirectory exists inside it. " +
		"This invariant is an HONEST NO-OP in the Go port.",
	Why: "without docs/ the agent cannot receive generated shared-docs links; create_agent always scaffolds it; its " +
		"absence indicates manual corruption. This invariant checks the FILE SYSTEM STRUCTURE of agent directories on " +
		"disk, not the shape of the in-memory graph. The Go invariant contract (Check func(*ontology.Graph) []Violation) " +
		"operates on an already-loaded in-memory graph and has no access to the agents directory path on disk. " +
		"Filesystem-coherence checks belong to a separate agent-scaffolding layer (future work outside " +
		"internal/invariants), the same boundary as check_doc_reader_resolves_to_stakeholder and " +
		"check_entities_md_lists_all_types. References: R-agent-has-docs-dir.",
	Check: checkAgentHasDocsSubdir,
})

func checkAgentHasToolsSubdir(g *ontology.Graph) []Violation {
	return nil
}

var _ = All.MustRegister("check_agent_has_tools_subdir", Invariant{
	Name:  "check_agent_has_tools_subdir",
	Canon: methodology.Agent,
	Claim: "every agent directory must contain a 'tools/' subdirectory (honest no-op in the Go port).",
	Rule: "RULE (original Python): every agent carries its own tools/ subdir for its private tools " +
		"(R-agent-has-own-tools-dir), separate from the shared spec/tools/ (R-shared-tools-in-spec-tools). The Python " +
		"source walked the agents directory, found each agent subdirectory (identified by scope.py), and checked that a " +
		"tools/ subdirectory exists inside it. " +
		"This invariant is an HONEST NO-OP in the Go port.",
	Why: "without tools/ the private and shared tool boundary is invisible; create_agent always scaffolds it; its " +
		"absence indicates manual corruption. This invariant checks the FILE SYSTEM STRUCTURE of agent directories on " +
		"disk, not the shape of the in-memory graph. The Go invariant contract (Check func(*ontology.Graph) []Violation) " +
		"operates on an already-loaded in-memory graph and has no access to the agents directory path on disk. " +
		"Filesystem-coherence checks belong to a separate agent-scaffolding layer (future work outside " +
		"internal/invariants), the same boundary as check_doc_reader_resolves_to_stakeholder and " +
		"check_entities_md_lists_all_types. References: R-agent-has-own-tools-dir, R-shared-tools-in-spec-tools.",
	Check: checkAgentHasToolsSubdir,
})
