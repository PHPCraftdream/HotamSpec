package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/PHPCraftdream/HotamSpec/internal/paths"
)

// cmdUse implements `hotam use <domain-name>` — the tiny command that sets the
// active-domain preference for the CURRENT project. It records
// {"active_domain": "<name>"} in the <project-root>/.hotam-spec-project marker
// file, so a subsequent bare `hotam <command>` (no --domain) targets the chosen
// domain (tier 3 of resolveDomain's active-domain chain, see common.go).
//
// It refuses to point the active domain at nothing: <root>/domains/<name>/graph.json
// must exist. The marker write succeeds even if the file did not exist before,
// so `hotam use` also promotes a domains/-native-marker project (one that
// resolves project-root via R3, never carried a .hotam-spec-project file) into
// one that also carries an active-domain preference going forward.
func cmdUse(args []string) error {
	fs := newFlagSet("use")
	fs.Parse(args)

	if fs.NArg() < 1 {
		return fmt.Errorf("usage: hotam use <domain-name>")
	}
	domainName := fs.Arg(0)

	root, err := paths.ProjectRootOrRaise()
	if err != nil {
		return err
	}

	domainDir := filepath.Join(root, "domains", domainName)
	if _, err := os.Stat(graphPathForDomain(domainDir)); err != nil {
		return fmt.Errorf("refusing to switch active domain: %s not found (expected a scaffolded domain at %s)", domainName, domainDir)
	}

	markerPath := filepath.Join(root, paths.MarkerFilename)
	if err := paths.WriteActiveDomain(markerPath, domainName); err != nil {
		return fmt.Errorf("write %s: %w", markerPath, err)
	}

	fmt.Printf("active domain set to %q (domains/%s)\n", domainName, domainName)
	return nil
}
