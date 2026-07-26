package invariants

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	ontologyvendor "github.com/PHPCraftdream/HotamSpec/internal/ontology/vendor"
)

// vendoredOntologyDirRelPath is where `hotam vendor-ontology`
// (cmd/hotam/vendor_ontology.go) writes the vendored copies, relative to a
// domain's own directory -- kept as a named constant here (rather than
// re-deriving filepath.Join("spec", "hotamontology") inline at each call
// site) so the write side and the check side visibly agree on one literal
// path, mirroring vendoredRecorderRelPath's identical role in
// recorder_check.go.
const vendoredOntologyDirRelPath = "spec/hotamontology"

// checkOntologyVendorCurrent is the mechanical actuality gate for the
// vendored minimal Requirement + Registry ontology mirror (task #365, RAC2
// Phase A): IF a domain has ever vendored the mirror (a file sits at
// spec/hotamontology/requirement.go and/or spec/hotamontology/registry.go),
// its content -- after stripping the do-not-edit banner `hotam
// vendor-ontology` always stamps on top -- MUST sha256-match the engine's
// OWN canonical source (internal/ontology/canon/requirement.go and
// registry.go, reached here via ontologyvendor.RequirementBodyForHash() /
// RegistryBodyForHash(), themselves go:embed's of those exact files -- see
// internal/ontology/canon/embed.go). Modeled directly on
// checkRecorderCurrent (recorder_check.go) -- same shape, same two failure
// classes it does not try to distinguish (stale vs. hand-edited), same
// honest-no-op posture.
//
// The two files are checked INDEPENDENTLY: a domain may have vendored one
// without the other (e.g. mid-upgrade, or a hand-deleted file) is still
// reported precisely (only the missing/drifted file's own violation fires),
// rather than one combined check that could mask which half actually
// drifted.
//
// This is an HONEST NO-OP for a domain that has never vendored the mirror at
// all (no file at either path): vendoring the ontology mirror is opt-in,
// exactly like the scenario recorder -- a domain that has not yet adopted
// Go-code-authored requirements is not lying about anything by not having a
// vendored copy on disk.
func checkOntologyVendorCurrent(g *ontology.Graph) []Violation {
	if g.DomainDir == "" {
		// No on-disk domain to check against (an in-memory fixture graph
		// built without ever going through loader.LoadGraph) -- honest
		// no-op, mirroring checkRecorderCurrent's identical guard.
		return nil
	}
	var violations []Violation
	violations = append(violations, checkVendoredOntologyFile(
		g.DomainDir, "requirement.go",
		ontologyvendor.RequirementBodyForHash(),
	)...)
	violations = append(violations, checkVendoredOntologyFile(
		g.DomainDir, "registry.go",
		ontologyvendor.RegistryBodyForHash(),
	)...)
	return violations
}

// checkVendoredOntologyFile checks one vendored file (fileName, one of
// "requirement.go"/"registry.go") under <domainDir>/spec/hotamontology/
// against wantBody, the engine's own canonical (post-banner) source for that
// same file. Shared by checkOntologyVendorCurrent for both files so the
// read/banner-strip/hash-compare logic exists exactly once.
func checkVendoredOntologyFile(domainDir, fileName, wantBody string) []Violation {
	vendoredPath := filepath.Join(domainDir, filepath.FromSlash(vendoredOntologyDirRelPath), fileName)
	data, err := os.ReadFile(vendoredPath)
	if err != nil {
		if os.IsNotExist(err) {
			// This file never vendored into this domain -- opt-in, not a
			// violation (see checkOntologyVendorCurrent's doc comment).
			return nil
		}
		return []Violation{{
			Check:   "check_ontology_vendor_current",
			ID:      domainDir,
			Message: fmt.Sprintf("could not read vendored ontology file at %s: %v", vendoredPath, err),
		}}
	}

	body, ok := ontologyvendor.StripBanner(string(data))
	if !ok {
		return []Violation{{
			Check: "check_ontology_vendor_current",
			ID:    domainDir,
			Message: fmt.Sprintf(
				"%s does not start with the expected `hotam vendor-ontology` do-not-edit banner -- "+
					"it was hand-created or hand-edited rather than produced by `hotam vendor-ontology`; "+
					"re-run `hotam vendor-ontology --domain %s` to restore a genuine vendored copy",
				vendoredPath, domainDir),
		}}
	}

	gotHash := sha256Hex(body)
	wantHash := sha256Hex(wantBody)
	if gotHash != wantHash {
		return []Violation{{
			Check: "check_ontology_vendor_current",
			ID:    domainDir,
			Message: fmt.Sprintf(
				"%s (sha256 %s) does not match the engine's current canonical ontology mirror "+
					"(internal/ontology/canon/%s, sha256 %s) -- either the vendored copy is stale "+
					"(re-run `hotam vendor-ontology --domain %s` to pick up the current canon) or it was hand-edited "+
					"despite the do-not-edit banner (restore it via the same command)",
				vendoredPath, gotHash, fileName, wantHash, domainDir),
		}}
	}
	return nil
}

var _ = All.MustRegister("check_ontology_vendor_current", Invariant{
	Name:  "check_ontology_vendor_current",
	Canon: methodology.Domain,
	Claim: "a domain's vendored spec/hotamontology/{requirement.go,registry.go}, if present, are byte-identical (post-banner) to the engine's own canonical ontology mirror.",
	Rule: "IF a file exists at <domainDir>/spec/hotamontology/requirement.go and/or spec/hotamontology/registry.go, each MUST start with the exact " +
		"`hotam vendor-ontology` do-not-edit banner (internal/ontology/vendor's Banner), and the body following that banner MUST sha256-match " +
		"internal/ontology/canon's own current content for that file (ontologyvendor.RequirementBodyForHash() / RegistryBodyForHash(), themselves " +
		"go:embed's of those exact files). A domain with NO file at either path is an honest no-op -- vendoring the ontology mirror is " +
		"opt-in per the domain's own adoption of Go-code-authored requirements, not mandatory for every domain unconditionally.",
	Why: "the vendored ontology mirror is Go code a consumer domain's spec/ module COMPILES AND IMPORTS to declare its own Requirement literals " +
		"in code (task #365, RAC2 Phase A) -- unlike a generated Markdown doc (drift there is merely stale prose), a stale or hand-edited " +
		"vendored Requirement/Registry shape can silently diverge from the engine's own ontology.Requirement (e.g. a hand-edit that drops a " +
		"field or renames a JSON tag would corrupt the round-trip the moment that domain's registry is later projected onto its graph.json). " +
		"Modeled directly on check_recorder_current (recorder_check.go): both are graph-generic invariants that read g.DomainDir's own " +
		"filesystem state (not just the in-memory graph), both are honest no-ops when the relevant on-disk artifact was never created, and " +
		"both exist to catch a real drift class no purely in-memory graph check could ever see.",
	Check: checkOntologyVendorCurrent,
})
