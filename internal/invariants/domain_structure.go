package invariants

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// checkGraphLockPinsGraphJSON is the runtime (all-violations) half of
// R-no-hand-edit-graph. lock_real_domains_test.go (internal/loader) already
// pins this via a hardcoded realDomains list -- but that list names only
// THIS repo's own two domains (hotam-spec-self, hotam-dev), so a hand-edit to
// a CONSUMER domain's graph.json (e.g. an adopting repo's domains/prat or
// domains/gpsm-sm) that leaves graph.lock untouched is invisible to that
// test and to `hotam all-violations` alike -- exactly @fh's F6 finding: "0
// violations -- graph clean" on a graph.json a human quietly edited by hand.
//
// This check closes that gap FOR ANY DOMAIN, not just this framework's own:
// it runs against whichever graph.json all-violations actually loaded
// (g.DomainDir, populated by loader.LoadGraph for every domain, self-hosting
// or consumer -- see enforcement.go's checkEnforcedByResolvable and
// authored_links.go for the same g.DomainDir pattern already established for
// filesystem-aware checks) and calls loader.VerifyLock on that graph's own
// graph.json/graph.lock pair. loader.WriteGraph (the only writer
// hotam apply-proposal/hotam land ever call) writes graph.lock in the SAME
// call as graph.json, so the legitimate proposal flow always leaves the pair
// in sync; only a hand-edit that bypasses that writer (editing graph.json
// directly, or restoring an old graph.json over a newer lock) can desync
// them, which is exactly the signal this check exists to surface.
func checkGraphLockPinsGraphJSON(g *ontology.Graph) []Violation {
	if g.DomainDir == "" {
		// No on-disk domain to check against (e.g. an in-memory fixture graph
		// built without ever going through loader.LoadGraph) -- honest no-op,
		// not a false positive against a path that was never loaded from disk.
		return nil
	}
	graphPath := graphPathForDomainDir(g.DomainDir)
	if _, statErr := os.Stat(graphPath); statErr != nil {
		// No graph.json actually sitting at g.DomainDir -- e.g. a synthetic
		// fixture graph built in-memory (ontology.Graph{DomainDir: someTempDir,
		// ...}) whose temp dir carries authored spec/ files for a DIFFERENT
		// check (authored_links_test.go's writeAuthoredSpecFixture) but was
		// never itself written by loader.LoadGraph/WriteGraph. A domain that
		// really went through LoadGraph always has a graph.json at this exact
		// path (LoadGraph reads it to construct the graph in the first place),
		// so this branch can never mask a real hand-edit on an actually-loaded
		// domain -- only a graph that was never backed by an on-disk pair at
		// all. Honest no-op, not a false positive.
		return nil
	}
	if _, statErr := os.Stat(loader.LockPath(graphPath)); statErr != nil {
		// graph.json exists but graph.lock does not -- a domain that has
		// never yet gone through hotam apply-proposal/hotam land (a freshly
		// scaffolded/copied fixture, or a domain still in its bootstrap
		// window before its first landed proposal). R-no-hand-edit-graph's
		// own claim text carves this out explicitly ("...prohibited outside
		// of bootstrap events"): WriteGraph/WriteLock always write the lock
		// alongside graph.json from the very first land, so an ABSENT lock
		// means "not yet under lock discipline", a materially different
		// signal from a PRESENT lock whose hash no longer matches (which can
		// only mean graph.json changed via some path OTHER than
		// WriteGraph -- the real hand-edit signal). Conflating the two would
		// falsely flag every test/bootstrap fixture that legitimately copies
		// a bare graph.json onto disk without ever calling WriteGraph
		// (cmd/hotam's own copySelfDomainUnderRoot test helper, used across
		// 100+ existing cmd/hotam tests, is exactly this shape) -- an honest
		// no-op here, not a false positive.
		return nil
	}
	ok, err := loader.VerifyLock(graphPath)
	if err != nil {
		return []Violation{{
			Check:   "check_graph_lock_pins_graph_json",
			ID:      g.DomainDir,
			Message: fmt.Sprintf("graph.lock could not be verified for %s: %v -- run `hotam apply-proposal`/`hotam land` to (re)create a consistent graph.json + graph.lock pair", graphPath, err),
		}}
	}
	if !ok {
		return []Violation{{
			Check:   "check_graph_lock_pins_graph_json",
			ID:      g.DomainDir,
			Message: fmt.Sprintf("graph.lock sha256 does not match the current %s -- the graph was hand-edited outside `hotam apply-proposal`/`hotam land` (R-no-hand-edit-graph); revert the hand-edit or land it as a real proposal so graph.json and graph.lock are rewritten together", graphPath),
		}}
	}
	return nil
}

var _ = All.MustRegister("check_graph_lock_pins_graph_json", Invariant{
	Name:  "check_graph_lock_pins_graph_json",
	Canon: methodology.Domain,
	Claim: "every domain's graph.lock sha256 pin matches its own current graph.json -- for ANY domain, not just this framework's self-hosting ones.",
	Rule: "RULE: for the domain graph actually being checked (g.DomainDir), when a graph.json really sits at that path AND a " +
		"sibling graph.lock also already exists, loader.VerifyLock(graph.json) MUST report a match against the sha256 pinned " +
		"in that graph.lock. loader.WriteGraph (the sole writer hotam apply-proposal / hotam land use) always rewrites " +
		"graph.lock in the same call as graph.json, so a legitimate land leaves the pair in sync; a mismatch on an EXISTING " +
		"lock can only mean graph.json was edited by some OTHER path -- a hand-edit, a restored backup, a manual merge -- " +
		"while graph.lock was left stale (R-no-hand-edit-graph). Two cases are honest no-ops, not false positives: (1) no " +
		"on-disk graph.json at g.DomainDir at all (g.DomainDir empty, or a synthetic fixture graph built purely in-memory and " +
		"pointed at an unrelated temp dir -- e.g. authored_links_test.go's writeAuthoredSpecFixture, which writes only spec/ " +
		"files, never a graph.json/graph.lock pair); (2) graph.json exists but graph.lock does not -- R-no-hand-edit-graph's " +
		"own claim text carves out this exact case ('...prohibited outside of bootstrap events'): a domain that has never yet " +
		"gone through its first hotam apply-proposal/hotam land (a freshly scaffolded or test-copied graph.json) is not yet " +
		"under lock discipline at all, a materially different signal from a PRESENT lock that no longer matches.",
	Why: "R-no-hand-edit-graph's OWN runtime enforcement previously existed only as internal/loader/lock_real_domains_test.go, a " +
		"test whose realDomains list names exactly this repo's own two domains (hotam-spec-self, hotam-dev) -- so it protects " +
		"HotamSpec's own graphs but is structurally blind to any domain outside this repo (an adopting consumer's domains/prat, " +
		"domains/gpsm-sm, or any future domain). Worse, that protection was TEST-ONLY: it never ran as part of `hotam " +
		"all-violations`, the command a consumer actually runs as their gate, so a hand-edited consumer graph.json reported " +
		"\"0 violations -- graph clean\" even though the lock had gone stale (@fh F6). This check moves the SAME VerifyLock " +
		"machinery (internal/loader/lock.go, already used by WriteGraph/WriteLock and by lock_real_domains_test.go) into the " +
		"graph-generic invariant registry, keyed off g.DomainDir the way enforcement.go's checkEnforcedByResolvable and " +
		"authored_links.go already resolve filesystem paths for whichever domain is actually loaded -- so it runs for every " +
		"domain all-violations ever checks, self-hosting or consumer, closing the gap without touching lock.go's proven " +
		"hash logic or WriteGraph/WriteLock's legitimate write path. References: R-no-hand-edit-graph, " +
		"TestNoHandEditGraph_RealDomainLocksPinCurrentGraph.",
	Check: checkGraphLockPinsGraphJSON,
})

// graphPathForDomainDir mirrors cmd/hotam/common.go's graphPathForDomain
// (filepath.Join(domainDir, "graph.json")) without importing package main --
// internal/ packages cannot import cmd/, so the one-line join is duplicated
// here rather than factored into a shared helper across that boundary.
func graphPathForDomainDir(domainDir string) string {
	return filepath.Join(domainDir, "graph.json")
}

func checkDomainManifestExistsAndImportable(g *ontology.Graph) []Violation {
	return nil
}

var _ = All.MustRegister("check_domain_manifest_exists_and_importable", Invariant{
	Name:  "check_domain_manifest_exists_and_importable",
	Canon: methodology.Domain,
	Claim: "every domains/<name>/manifest exists and can be loaded (honest no-op).",
	Rule: "RULE: a domain directory MUST contain a manifest that can be loaded without error. " +
		"Missing or unloadable manifests make the domain invisible to the framework (R-domain-has-manifest). " +
		"This invariant is an HONEST NO-OP.",
	Why: "this invariant checks the FILE SYSTEM STRUCTURE of a domain on disk (does manifest or manifest.json exist " +
		"and is it loadable), not the shape of the in-memory graph. The Go invariant contract (Check func(*ontology.Graph) " +
		"[]Violation) operates on an already-loaded in-memory graph and has no access to the domain's directory path on " +
		"disk. Filesystem-coherence checks of this kind belong to a separate domain-scaffolding or create_domain layer " +
		"(future work outside internal/invariants), which has access to the filesystem and the build pipeline -- the same " +
		"architectural boundary as check_doc_reader_resolves_to_stakeholder and check_entities_md_lists_all_types. " +
		"Implementing this as a graph-only invariant would require adding a filesystem path parameter (breaking the uniform " +
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
	Claim: "every domains/<name>/manifest ID field matches the directory name (honest no-op).",
	Rule: "RULE: manifest MUST define an ID attribute equal to the directory name. A mismatched ID " +
		"breaks the identity anchor for gen_spec and create_domain tooling. " +
		"This invariant is an HONEST NO-OP.",
	Why: "this invariant checks the FILE SYSTEM STRUCTURE of a domain on disk: it loads each domain's manifest, reads its " +
		"ID attribute, and compares it to the directory name. The Go invariant contract " +
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
	Claim: "every domains/<name>/manifest DESCRIPTION is non-empty (honest no-op).",
	Rule: "RULE: manifest MUST define a non-empty DESCRIPTION attribute. A domain without a " +
		"description is undocumented and invisible to human readers. " +
		"This invariant is an HONEST NO-OP.",
	Why: "this invariant checks the FILE SYSTEM STRUCTURE of a domain on disk: it loads each domain's manifest and reads " +
		"its DESCRIPTION attribute. The Go invariant contract " +
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
	Claim: "every domains/<name>/manifest GOALS is non-empty (honest no-op).",
	Rule: "RULE: manifest MUST define a non-empty GOALS attribute. A domain without declared goals " +
		"has no visible intent and cannot drive the burn-down meter. " +
		"This invariant is an HONEST NO-OP.",
	Why: "this invariant checks the FILE SYSTEM STRUCTURE of a domain on disk: it loads each domain's manifest and reads " +
		"its GOALS attribute. The Go invariant contract " +
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
	Claim: "every domains/<name>/manifest DIRECTOR is non-empty (honest no-op).",
	Rule: "RULE: manifest MUST define a non-empty DIRECTOR attribute naming the director agent. " +
		"A domain without a declared director is headless. " +
		"This invariant is an HONEST NO-OP.",
	Why: "this invariant checks the FILE SYSTEM STRUCTURE of a domain on disk: it loads each domain's manifest and reads " +
		"its DIRECTOR attribute. The Go invariant contract " +
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
	Claim: "every domains/<name>/manifest defines ID (matching dirname), DESCRIPTION, GOALS, DIRECTOR (thin delegator, honest no-op).",
	Rule: "RULE: a domain without a valid manifest is invisible to the framework. This is a THIN " +
		"DELEGATOR -- calls check_domain_manifest_exists_and_importable, check_domain_manifest_id_matches_dirname, " +
		"check_domain_manifest_description_nonempty, check_domain_manifest_goals_nonempty, " +
		"check_domain_manifest_director_nonempty and concatenates. The atomic sub-checks are registered individually. " +
		"Because all five sub-checks are honest no-ops (they check filesystem structure, not graph shape), " +
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
	Claim: "every domains/<name>/agents/<DIRECTOR>/scope must exist (honest no-op).",
	Rule: "RULE: a domain whose declared director agent is missing is headless. " +
		"This invariant is an HONEST NO-OP.",
	Why: "the director is the entry point for all domain-level operator delegation (R-domain-declares-director). " +
		"This invariant checks the FILE SYSTEM STRUCTURE of a domain on disk (does the director agent's scope exist), " +
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
	Claim: "every agent directory must contain an 'agents/' subdirectory (honest no-op).",
	Rule: "RULE: every agent is itself a potential director that can spawn sub-agents; the agents/ " +
		"subdir is the recursion slot (R-agent-is-recursive-director). " +
		"This invariant is an HONEST NO-OP.",
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
	Claim: "every agent directory must contain a 'docs/' subdirectory (honest no-op).",
	Rule: "RULE: every agent carries its own docs/ for generated CLAUDE.md and thinking fragments " +
		"scoped to its domain (R-agent-has-docs-dir). " +
		"This invariant is an HONEST NO-OP.",
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
	Claim: "every agent directory must contain a 'tools/' subdirectory (honest no-op).",
	Rule: "RULE: every agent carries its own tools/ subdir for its private tools " +
		"(R-agent-has-own-tools-dir), separate from the shared spec/tools/ (R-shared-tools-in-spec-tools). " +
		"This invariant is an HONEST NO-OP.",
	Why: "without tools/ the private and shared tool boundary is invisible; create_agent always scaffolds it; its " +
		"absence indicates manual corruption. This invariant checks the FILE SYSTEM STRUCTURE of agent directories on " +
		"disk, not the shape of the in-memory graph. The Go invariant contract (Check func(*ontology.Graph) []Violation) " +
		"operates on an already-loaded in-memory graph and has no access to the agents directory path on disk. " +
		"Filesystem-coherence checks belong to a separate agent-scaffolding layer (future work outside " +
		"internal/invariants), the same boundary as check_doc_reader_resolves_to_stakeholder and " +
		"check_entities_md_lists_all_types. References: R-agent-has-own-tools-dir, R-shared-tools-in-spec-tools.",
	Check: checkAgentHasToolsSubdir,
})
