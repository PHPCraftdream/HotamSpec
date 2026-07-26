package main

import (
	"fmt"
	"os"
	"path/filepath"

	ontologyvendor "github.com/PHPCraftdream/HotamSpec/internal/ontology/vendor"
)

// cmdVendorOntology implements `hotam vendor-ontology --domain <dir>`: it
// copies the engine's canonical minimal Requirement + Registry ontology
// mirror (internal/ontology/canon/requirement.go + registry.go, embedded at
// build time via internal/ontology/vendor's RequirementSource/
// RegistrySource) into <domainDir>/spec/hotamontology/requirement.go and
// <domainDir>/spec/hotamontology/registry.go, both banner-stamped
// do-not-edit.
//
// This is a SEPARATE step from `hotam gen-spec`, not folded into it --
// modeled directly on `hotam vendor-recorder` (cmd/hotam/vendor_recorder.go)
// for the identical reason: vendoring only makes sense for a domain that
// already has an authored spec/ Go module (PLAN-authored-spec-discipline.md
// §3), and it must never attempt to write a Go source file into a directory
// that is not (yet, or ever going to be) a Go module. `hotam vendor-ontology`
// requires the caller to have already created spec/ (with its own go.mod)
// before it will write anything -- see the explicit go.mod existence check
// below.
//
// Re-running this command is always safe and idempotent: it overwrites both
// vendored files unconditionally with the current canon, which is also
// exactly how a domain picks up a NEWER canon after an engine upgrade --
// re-run vendor-ontology, the vendored copies advance,
// check_ontology_vendor_current goes back to green.
//
// Task #365 scope note: this command is infrastructure ONLY -- it vendors
// the minimal type + registry a consumer domain's spec/ module can import.
// It does NOT implement `hotam sync-domain` (projecting a domain's own
// Requirement registry onto its graph.json) and does NOT touch
// internal/proposal/apply.go's self-hosting lock -- those are separate,
// later tasks (#366, #367).
func cmdVendorOntology(args []string) error {
	fs := newFlagSet("vendor-ontology")
	domain := fs.String("domain", "", "domain directory (default: "+defaultDomainRel+")")
	fs.Parse(args)

	domainDir, err := resolveDomain(*domain)
	if err != nil {
		return err
	}

	written, err := vendorOntology(domainDir)
	if err != nil {
		return err
	}
	for _, path := range written {
		fmt.Println(relPathForDisplay(path))
	}
	return nil
}

// vendorOntology does the actual writes and returns the paths written, so
// tests can call it directly without going through flag parsing / stdout.
//
// Requires <domainDir>/spec/go.mod to already exist -- same hard requirement
// as vendorRecorder (cmd/hotam/vendor_recorder.go), for the identical
// reason: minting a NEW Go module is an authoring decision belonging to
// whoever founds the domain's spec/ tree, not something this command should
// silently decide on the caller's behalf.
func vendorOntology(domainDir string) ([]string, error) {
	specDir := filepath.Join(domainDir, "spec")
	specGoMod := filepath.Join(specDir, "go.mod")
	if _, err := os.Stat(specGoMod); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("vendor-ontology: %s does not exist -- the domain's spec/ tree must already be its own Go module (its own go.mod) before the ontology mirror can be vendored into it; see PLAN-authored-spec-discipline.md §3", specGoMod)
		}
		return nil, fmt.Errorf("vendor-ontology: stat %s: %w", specGoMod, err)
	}

	targetDir := filepath.Join(specDir, "hotamontology")
	reqTarget := filepath.Join(targetDir, "requirement.go")
	regTarget := filepath.Join(targetDir, "registry.go")

	if err := writeFileMkdir(reqTarget, []byte(ontologyvendor.RequirementSource())); err != nil {
		return nil, fmt.Errorf("vendor-ontology: %w", err)
	}
	if err := writeFileMkdir(regTarget, []byte(ontologyvendor.RegistrySource())); err != nil {
		return nil, fmt.Errorf("vendor-ontology: %w", err)
	}
	return []string{reqTarget, regTarget}, nil
}
