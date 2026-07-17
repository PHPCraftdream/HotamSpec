package main

import (
	"fmt"
	"os"
	"path/filepath"

	recordervendor "github.com/PHPCraftdream/HotamSpec/internal/recorder/vendor"
)

// cmdVendorRecorder implements `hotam vendor-recorder --domain <dir>`: it
// copies the engine's canonical hotamspec scenario-recorder source
// (internal/recorder/canon/hotamspec.go, embedded at build time via
// internal/recorder/vendor's Source) into
// <domainDir>/spec/hotamspec/hotamspec.go, banner-stamped do-not-edit.
//
// This is a SEPARATE step from `hotam gen-spec`, not folded into it
// (PLAN-scenario-generated-spec.md §2 D1, task W1.1), because vendoring only
// makes sense for a domain that already has an authored spec/ Go module
// (PLAN-authored-spec-discipline.md §3): gen-spec runs unconditionally for
// EVERY domain, including ones with no spec/ tree at all (a domain still at
// the manifest/purpose stage of R-domain-founded-in-wave-order's 8 steps),
// and it must never attempt to write a Go source file into a directory that
// is not (yet, or ever going to be) a Go module. `hotam vendor-recorder`
// requires the caller to have already created spec/ (with its own go.mod,
// per PLAN-authored-spec-discipline.md's authored-tree convention) before
// it will write anything -- see the explicit go.mod existence check below.
//
// Re-running this command is always safe and idempotent: it overwrites
// spec/hotamspec/hotamspec.go unconditionally with the current canon (the
// same "regenerate, never hand-merge" contract every other `hotam gen-spec`
// output already has), which is also exactly how a domain picks up a NEWER
// canon after an engine upgrade -- re-run vendor-recorder, the vendored copy
// advances, check_recorder_current goes back to green.
func cmdVendorRecorder(args []string) error {
	fs := newFlagSet("vendor-recorder")
	domain := fs.String("domain", "", "domain directory (default: "+defaultDomainRel+")")
	fs.Parse(args)

	domainDir, err := resolveDomain(*domain)
	if err != nil {
		return err
	}

	written, err := vendorRecorder(domainDir)
	if err != nil {
		return err
	}
	fmt.Println(relPathForDisplay(written))
	return nil
}

// vendorRecorder does the actual write and returns the path written, so
// tests can call it directly without going through flag parsing / stdout.
//
// Requires <domainDir>/spec/go.mod to already exist -- vendoring into a
// spec/ tree that is not yet its own Go module would produce a Go file with
// nowhere to be compiled, silently misleading a caller into thinking the
// recorder is usable when `go build` would immediately fail. This is a
// deliberate hard requirement, not a "create go.mod too" convenience,
// because minting a NEW Go module (choosing a module path, a go directive
// version) is an authoring decision belonging to whoever founds the domain's
// spec/ tree (R-domain-founded-in-wave-order step 3), not something this
// command should silently decide on the caller's behalf.
func vendorRecorder(domainDir string) (string, error) {
	specDir := filepath.Join(domainDir, "spec")
	specGoMod := filepath.Join(specDir, "go.mod")
	if _, err := os.Stat(specGoMod); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("vendor-recorder: %s does not exist -- the domain's spec/ tree must already be its own Go module (its own go.mod) before the recorder can be vendored into it; see PLAN-authored-spec-discipline.md §3", specGoMod)
		}
		return "", fmt.Errorf("vendor-recorder: stat %s: %w", specGoMod, err)
	}

	target := filepath.Join(specDir, "hotamspec", "hotamspec.go")
	content := []byte(recordervendor.Source())
	if err := writeFileMkdir(target, content); err != nil {
		return "", fmt.Errorf("vendor-recorder: %w", err)
	}
	return target, nil
}
