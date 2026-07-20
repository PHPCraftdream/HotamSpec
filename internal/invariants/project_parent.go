package invariants

import (
	"fmt"
	"path/filepath"

	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// checkProjectParentDeclared is the manifest.parent gate (PLAN-scenario-
// generated-spec.md §2 D6, task W6.1). D6 makes a domain's manifest.json
// "parent" field MANDATORY for any domain that HAS a manifest.json -- sub-
// project inheritance becomes machine-readable: manifest.parent names the
// parent domain for a child, or is explicitly JSON null for a root domain,
// but it MUST be declared either way. The field's semantics are resolved by
// loader.ResolveParent and surfaced on the loaded graph as the triple
// g.ManifestExists / g.ParentDeclared / g.Parent:
//
//   - g.ManifestExists == false: no manifest.json exists next to graph.json
//     at all. There is no manifest for a "parent" key to be missing FROM --
//     HONEST NO-OP (see the WHY THIS IS NOT AN OPT-IN section below for why
//     this differs from simply bailing on g.DomainDir == "").
//   - g.ManifestExists == true, g.ParentDeclared == false: a manifest.json
//     exists but its "parent" key is absent (ResolveParent also returns this
//     for malformed JSON or a non-string/non-null value). This IS the
//     VIOLATION: D6 makes the field obligatory once a domain has a manifest
//     at all.
//   - g.ManifestExists == true, g.ParentDeclared == true, g.Parent == "":
//     the key is PRESENT with JSON null (or an explicit empty string). This
//     is a VALID declaration that the domain is a ROOT -- the "I have
//     considered this and have no parent" case D6 reserves explicit JSON
//     null for. No violation.
//   - g.ManifestExists == true, g.ParentDeclared == true, g.Parent != "":
//     the key is PRESENT with a non-empty string naming the parent domain.
//     This is a VALID declaration that the domain is a CHILD of g.Parent.
//     No violation.
//
// This check fires ONE violation when g.ManifestExists && !g.ParentDeclared,
// naming the domain and instructing the steward to add "parent": null (root)
// or "parent": "<parent-domain-name>" (child) to manifest.json.
//
// WHY THIS IS NOT AN OPT-IN / HONEST-NO-OP CHECK IN THE check_settled_requires_scenario
// SENSE, BUT DOES still bail via g.ManifestExists: the package's opt-in
// manifest-field checks (check_settled_requires_scenario, gated on
// discipline:"full") are honest no-ops because the METHODOLOGY only
// recommends the behavior and lets a steward migrate per-domain -- a domain
// that has a manifest.json and simply hasn't opted in stays silently clean.
// D6 is different: "обязательное поле" (mandatory field) -- for any domain
// that HAS a manifest.json, the obligation is universal, not opt-in; there is
// no discipline:"full"-style flag to gate on. What DOES bail is
// g.ManifestExists == false: a graph with no manifest.json at all was never
// given the chance to declare parent in the first place (this is the shape
// of countless synthetic test-fixture graphs across this codebase that build
// &ontology.Graph{DomainDir: someTempDir, ...} directly, in Go code, without
// ever running `hotam init` or writing a manifest.json -- treating their
// total absence of a manifest as a violation would be a false positive
// exactly like firing check_graph_lock_pins_graph_json against a path that
// was never loaded from disk). This is DIFFERENT from a bare `g.DomainDir ==
// ""` bail (which the sibling check_graph_lock_pins_graph_json uses): many
// existing test fixtures in this codebase DO set a real DomainDir (a real
// t.TempDir() path) while never writing a manifest.json there, so gating on
// DomainDir alone would still fire false positives against them; gating on
// ManifestExists (resolved by actually trying to read manifest.json)
// correctly no-ops for exactly that population while still firing for any
// REAL on-disk domain (which always has a manifest.json, by construction --
// `hotam init`/`init-project` always create one).
//
// DOMAIN-LEVEL, NOT PER-REQUIREMENT: this check evaluates exactly ONCE per
// all-violations run against g (it asks a question about the manifest, not
// about any individual requirement), matching the shape of
// check_graph_lock_pins_graph_json rather than the per-requirement
// check_settled_requires_scenario.
func checkProjectParentDeclared(g *ontology.Graph) []Violation {
	if !g.ManifestExists {
		// No manifest.json at all next to graph.json -- there is no manifest
		// for a "parent" key to be missing FROM. Honest no-op: this is the
		// shape of a synthetic in-memory/test-fixture graph (a real domain
		// always has a manifest.json, created by `hotam init`/`init-project`).
		return nil
	}
	if g.ParentDeclared {
		// The "parent" key is present in manifest.json -- either JSON null
		// (an explicit root-domain declaration) or a string naming the parent
		// domain (a child-domain declaration). Both are valid declarations;
		// D6's obligation is satisfied. No violation.
		//
		// F5 (task W7.2, @fx finding F5): SELF-REFERENCE DETECTION. A parent
		// value that names THIS domain's own name (e.g. gpsm-sm's manifest
		// declaring "parent": "gpsm-sm") is a self-reference -- a domain
		// cannot be its own parent. This is the ONE parent-VALUE check that
		// does NOT require reaching into a sibling domain's filesystem (it
		// only needs filepath.Base(g.DomainDir), the domain's own directory
		// name), so it is architecturally sound to enforce here without
		// introducing the cross-domain-filesystem pattern no other invariant
		// in this codebase uses. A self-referencing parent fires a violation.
		if g.Parent != "" && g.DomainDir != "" {
			ownName := filepath.Base(g.DomainDir)
			if g.Parent == ownName {
				return []Violation{{
					Check: "check_project_parent_declared",
					ID:    g.DomainDir,
					Message: fmt.Sprintf(
						"manifest.json declares \"parent\": %q for %s, but that is this domain's OWN name -- "+
							"a domain cannot be its own parent; use \"parent\": null (root domain) or "+
							"\"parent\": \"<a-different-domain-name>\" (child domain)",
						g.Parent, ownName),
				}}
			}
		}
		//
		// HONESTY BOUNDARY -- VALUE NOT RESOLVED AGAINST REAL DOMAINS (F5,
		// task W7.2): this check validates that the parent key IS declared
		// (presence) and that it is not a SELF-REFERENCE (parent == own
		// name), but it does NOT validate that a NON-SELF parent value
		// names a REAL sibling domain (e.g. "parent": "nonexistent-domain"
		// or an A<->B cycle passes silently here). Every other filesystem-
		// aware invariant in this codebase (check_graph_lock_pins_graph_json,
		// check_recorder_current, check_spec_md_current) reads from the
		// domain's OWN directory only; validating a sibling's existence
		// (../<parent-name>/graph.json) would be architecturally new (no
		// existing invariant reaches into a sibling domain's filesystem),
		// and would break the existing test-fixture pattern (project_parent
		// _test.go's temp-dir fixtures declare parent values pointing to
		// real domain names like "hotam-spec-self" that do not exist as
		// siblings inside the temp dir). The crystal's
		// RenderParentProjectBlock (internal/generator/claudemd.go)
		// renders whatever parent value is declared, unvalidated --
		// a steward reading the crystal sees the declaration as-is.
		// This scoping is deliberately LOUDLY documented here (matching
		// this codebase's convention of never leaving an honesty boundary
		// silent -- see checkScenarioExecutesImpl's own doc comment and
		// the INHERENTLY_PROSE exemption's doc comment); a future wave
		// MAY add cross-domain sibling validation as a separate invariant
		// with its own anchoring and test fixtures.
		return nil
	}
	return []Violation{{
		Check: "check_project_parent_declared",
		ID:    g.DomainDir,
		Message: fmt.Sprintf(
			"manifest.json lacks a \"parent\" key for %s -- D6 makes parent a MANDATORY field once a domain has a manifest at "+
				"all: add \"parent\": null (this is a root domain) or \"parent\": \"<parent-domain-name>\" (this is a child domain) "+
				"to manifest.json so sub-project inheritance is machine-readable",
			g.DomainDir),
	}}
}

var _ = All.MustRegister("check_project_parent_declared", Invariant{
	Name:  "check_project_parent_declared",
	Canon: methodology.Domain,
	Claim: "every domain that HAS a manifest.json declares a \"parent\" field in it (PLAN-scenario-generated-spec.md §2 D6): " +
		"either JSON null (an explicit root-domain declaration -- this domain has no parent) or a non-empty string naming the " +
		"parent domain (a child-domain declaration). A manifest.json that exists but has no \"parent\" key at all is a " +
		"violation -- D6 makes the field mandatory once a manifest exists, not opt-in. A domain with NO manifest.json at all " +
		"is an honest no-op (it was never given the chance to declare parent).",
	Rule: "RULE: for the domain graph actually being checked, resolved by loader.ResolveParent into the triple " +
		"g.ManifestExists / g.ParentDeclared / g.Parent. IF g.ManifestExists == false (no manifest.json next to graph.json at " +
		"all -- the shape of countless synthetic test-fixture graphs across this codebase that build &ontology.Graph{DomainDir: " +
		"someTempDir, ...} directly, in Go code, without ever running `hotam init`), this check is a HONEST NO-OP: there is no " +
		"manifest for a \"parent\" key to be missing FROM. OTHERWISE (a manifest.json genuinely exists), IF g.ParentDeclared " +
		"is true -- the \"parent\" key is present in that manifest, whether JSON null (an explicit root-domain declaration) or " +
		"a non-empty string naming the parent domain (a child-domain declaration) -- this check fires ZERO violations: both " +
		"are valid declarations, D6's obligation is satisfied. OTHERWISE (g.ManifestExists is true but g.ParentDeclared is " +
		"false -- the \"parent\" key is absent from an EXISTING manifest.json; ResolveParent also returns this for malformed " +
		"JSON or a non-string value), this check fires ONE violation naming the domain and instructing the steward to add " +
		"\"parent\": null (root) or \"parent\": \"<parent-domain-name>\" (child). This is NOT an opt-in / honest-no-op check " +
		"like check_settled_requires_scenario (which no-ops for every domain that HAS a manifest but has not declared " +
		"discipline:\"full\"): D6 makes parent MANDATORY (\"обязательное поле\") for any domain that has a manifest at all, so " +
		"there is no per-domain opt-in flag to gate on once a manifest exists -- the ManifestExists bail is not an opt-in, it " +
		"is the same missing-manifest-is-the-soft-default convention every sibling resolver (ResolveDiscipline/ResolveGenProfile/" +
		"ResolveRequireProvenance) already follows. It is DOMAIN-LEVEL (fires once per all-violations run against g, asking a " +
		"question about the manifest, not about any individual requirement), matching check_graph_lock_pins_graph_json's shape " +
		"rather than the per-requirement shape.",
	Why: "PLAN-scenario-generated-spec.md §2 D6 (task W6.1): sub-project inheritance becomes machine-readable. Before W6.1 " +
		"the relationship between a domain and its parent (the recursion / sub-project hierarchy D6 is about) existed only " +
		"implicitly -- a manifest.json either named no parent at all or named one informally, and nothing distinguished " +
		"\"has no parent because it is a root\" from \"has no parent because no one bothered to declare it\". D6 collapses " +
		"that ambiguity by making manifest.parent a MANDATORY field once a manifest exists: JSON null is the EXPLICIT, " +
		"deliberate declaration of rootness (\"I have considered this and I have no parent\"), a non-empty string is a child " +
		"declaration, and absence-from-an-existing-manifest is a violation. The crystal's \"Parent project\" section " +
		"(internal/generator/claudemd.go RenderParentProjectBlock) reads this resolved state so a domain's place in the " +
		"sub-project hierarchy is visible in every generated crystal, not just inferable. This check is the gate that makes " +
		"the field actually obligatory: without it, a manifest.json that simply never declared parent would pass " +
		"all-violations clean, and the crystal section would silently fall back to a placeholder -- exactly the kind of quiet " +
		"invisibility R-anchor-everything's generative law exists to prevent. The g.ManifestExists bail (rather than a bare " +
		"g.DomainDir == \"\" check) matters in practice: this codebase's test suite has many fixtures that set a real " +
		"DomainDir (a t.TempDir() path) while never writing a manifest.json there, and gating on DomainDir alone would " +
		"produce false-positive violations against every one of them; every REAL on-disk domain always has a manifest.json " +
		"(created by `hotam init`/`init-project`), so gating on manifest existence correctly separates the two populations. " +
		"init-project's born-obligated wiring (a SEPARATE later task, W6.2) will scaffold new sub-projects with parent + " +
		"discipline:full + a vendored recorder from birth; this check is the MECHANISM (the manifest field, the invariant, " +
		"the crystal section) that the wiring will populate, so the obligation is enforceable the moment any domain with a " +
		"manifest is loaded, regardless of how its manifest was authored. References: loader.ResolveParent (the resolution), " +
		"ontology.Graph.ManifestExists/ParentDeclared/Parent (the surfaced fields), RenderParentProjectBlock (the crystal " +
		"section).",
	Check: checkProjectParentDeclared,
})
