package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/paths"
)

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	fs.SetOutput(os.Stderr)
	return fs
}

// defaultDomainRel is the legacy relative path "domains/<name>" used in
// --domain flag-help strings across this package (gen-spec, status, req, …).
// It stays accurate as a help-text description because tier 4 (legacy default)
// still resolves to exactly this path for this repository's own self-hosting
// workflow and for any project with no recorded active-domain preference.
const defaultDomainRel = "domains/hotam-spec-self"

// defaultDomainName is the tier-4 (legacy) domain NAME used when neither the
// HOTAM_DOMAIN env var nor a marker-file active_domain preference yields a
// name. It is the domain-name component of defaultDomainRel above.
const defaultDomainName = "hotam-spec-self"

// resolveDomain resolves the target domain directory for a command, applying a
// 4-tier active-domain resolution order. The project root is resolved ONCE via
// paths.ProjectRootOrRaise(); the domain NAME is then picked by
// resolveActiveDomainName and joined as <root>/domains/<name>.
//
// Resolution tiers (highest priority first):
//
//  1. Explicit --domain <path> (domainFlag != ""): resolved verbatim via
//     filepath.Abs. No project-root resolution — byte-identical to
//     pre-active-domain behavior so every script/flag-based call is
//     unaffected. The sole exception is an advisory (never fatal) N5 hint on
//     stderr when domainFlag has no path separator AND the resolved path
//     does not exist — a likely sign the caller passed a domain NAME (as
//     `hotam use` accepts) where a PATH was expected; see the inline comment
//     at the hint's call site for the exact conditions.
//  2. HOTAM_DOMAIN env var: a domain NAME resolved as <root>/domains/<name>.
//     Emits one stderr notice ("resolved domain: <name> (via HOTAM_DOMAIN env)").
//  3. active_domain recorded in the .hotam-spec-project marker file
//     (paths.ReadActiveDomain). Emits one stderr notice
//     ("resolved domain: <name> (via .hotam-spec-project marker)").
//  4. Legacy default defaultDomainName ("hotam-spec-self"): SILENT, so this
//     repository's own everyday bare-command usage and any test relying on the
//     silent default see zero new output.
//
// Tiers 2 and 3 emit a single stderr line (never stdout) so an agent is never
// surprised by which domain a bare command silently targeted ("honesty over
// magic"), while the unsurprising tier-1 and tier-4 cases stay noise-free.
func resolveDomain(domainFlag string) (string, error) {
	if domainFlag != "" {
		abs, err := filepath.Abs(domainFlag)
		if err != nil {
			return "", fmt.Errorf("resolve --domain %q: %w", domainFlag, err)
		}
		// N5 advisory hint (never fatal, never blocks): a bare NAME with no
		// path separator that does not exist on disk is almost certainly a
		// user passing an active-domain NAME (the kind `hotam use` accepts)
		// where this flag expects a PATH — e.g. --domain hotam-spec-self
		// instead of --domain domains/hotam-spec-self. Without this, the
		// resulting error surfaces later as a confusing "no such file"
		// buried inside a graph-load path. A single-segment name that DOES
		// exist (a legitimately named directory checked out at CWD) must
		// NOT get a spurious hint, so this only fires when both conditions
		// hold.
		if !strings.ContainsAny(domainFlag, "/\\") {
			if _, statErr := os.Stat(abs); statErr != nil {
				fmt.Fprintf(os.Stderr, "hint: --domain %q looks like a domain NAME, not a path, and no such directory exists here — did you mean %q?\n", domainFlag, "domains/"+domainFlag)
			}
		}
		return abs, nil
	}
	root, err := paths.ProjectRootOrRaise()
	if err != nil {
		return "", err
	}
	name, source := resolveActiveDomainName(root)
	if source != "" {
		fmt.Fprintf(os.Stderr, "resolved domain: %s (via %s)\n", name, source)
	}
	return filepath.Join(root, "domains", name), nil
}

// resolveActiveDomainName picks the active domain NAME for a resolved project
// root, returning the name and a human-readable source describing where it came
// from. A non-empty source means a genuinely new magic-resolution path (tiers 2
// or 3) fired and should be reported on stderr; source == "" means the silent
// tier-4 legacy default applies and must produce no notice.
//
// Priority order:
//
//  1. HOTAM_DOMAIN env var (paths.EnvActiveDomain), trimmed of whitespace —
//     source "HOTAM_DOMAIN env".
//  2. active_domain in the <root>/.hotam-spec-project marker — source
//     ".hotam-spec-project marker".
//  3. Legacy default defaultDomainName — source "" (silent).
func resolveActiveDomainName(root string) (name, source string) {
	if env := strings.TrimSpace(os.Getenv(paths.EnvActiveDomain)); env != "" {
		return env, "HOTAM_DOMAIN env"
	}
	markerPath := filepath.Join(root, paths.MarkerFilename)
	if n, ok := paths.ReadActiveDomain(markerPath); ok {
		return n, ".hotam-spec-project marker"
	}
	return defaultDomainName, ""
}

func graphPathForDomain(domainDir string) string {
	return filepath.Join(domainDir, "graph.json")
}

func loadDomainGraph(domainDir string) (*ontology.Graph, error) {
	gp := graphPathForDomain(domainDir)
	g, err := loader.LoadGraph(gp)
	if err != nil {
		return nil, err
	}
	return g, nil
}

func writeFileMkdir(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir for %s: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// writeFilesParallel writes each (path, content) pair in paths[i]/contents[i]
// to disk concurrently — one goroutine per file, each touching only its own
// path, so there is no write collision to guard against. It mirrors
// invariants.AllViolations' indexed-slice-then-merge shape: results land in
// a pre-sized slot per index (never via append from a goroutine, which would
// race), so the caller can rebuild output in the original, deterministic
// order after wg.Wait() rather than in whatever order goroutines happen to
// finish.
//
// It returns the first error encountered, selected deterministically by
// index (lowest i wins) rather than by goroutine completion order, so a
// failure is reproducible across runs even though the writes themselves are
// concurrent.
func writeFilesParallel(paths []string, contents [][]byte) error {
	errs := make([]error, len(paths))
	var wg sync.WaitGroup
	for i := range paths {
		wg.Add(1)
		go func(idx int, p string, data []byte) {
			defer wg.Done()
			errs[idx] = writeFileMkdir(p, data)
		}(i, paths[i], contents[i])
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func domainNameFromDir(domainDir string) string {
	return filepath.Base(domainDir)
}

func relPathForDisplay(path string) string {
	cwd, err := os.Getwd()
	if err != nil {
		return path
	}
	rel, err := filepath.Rel(cwd, path)
	if err != nil {
		return path
	}
	return rel
}
