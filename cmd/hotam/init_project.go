package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/PHPCraftdream/HotamSpec/internal/paths"
)

// defaultInitProjectDomain is the domain name `hotam init-project` scaffolds
// under <dir>/domains/ when --domain is omitted. It is deliberately a neutral,
// multi-domain-friendly name rather than the dir's basename (which would
// duplicate the project name: ./acme/domains/acme) or the framework's own
// self-modeling domain name "hotam-spec-self" (which would be confusing for an
// external business project). init-project records this name as the project's
// active-domain preference in the marker file, so a bare `hotam <command>`
// (no --domain) run from inside the project resolves to domains/<name> via
// resolveDomain's tier-3 marker resolution (cmd/hotam/common.go).
const defaultInitProjectDomain = "main"

// cmdInitProject implements `hotam init-project <dir> [--domain <name>]
// [--today YYYY-MM-DD]` — the single onboarding command that bootstraps an
// EXTERNAL business project's full Hotam-Spec layout in one call. Where
// `hotam init <dir>` scaffolds only one bare domain, init-project additionally
// creates the project-root marker (<dir>/.hotam-spec-project) and renders the
// root crystal (CLAUDE.md/AGENTS.md/GEMINI.md) plus every docs/gen/* view, so a
// brand-new adopter lands on a fully working project the instant the command
// returns — "we say HOW, the business builds WHAT using our HOW".
//
// It is a pure composition of the two existing primitives — initDomain (the
// per-domain scaffold) and genSpec (the doc/crystal generator) — plus a
// marker-file write. It does not reimplement either: a future change to domain
// scaffolding or doc generation flows through init-project for free.
func cmdInitProject(args []string) error {
	fs := newFlagSet("init-project")
	domainName := fs.String("domain", "", "name of the base domain to scaffold under <dir>/domains/ (default: "+defaultInitProjectDomain+")")
	todayFlag := fs.String("today", "", "date in YYYY-MM-DD format (default: system date) — embedded in freshness/status lines of the generated docs and root crystal; pin this for reproducible/byte-identical regeneration")
	fs.Parse(args)

	if fs.NArg() < 1 {
		return fmt.Errorf("usage: hotam init-project <dir> [--domain <name>] [--today YYYY-MM-DD]")
	}
	rawDir := fs.Arg(0)

	dir, err := filepath.Abs(rawDir)
	if err != nil {
		return fmt.Errorf("resolve <dir> %q: %w", rawDir, err)
	}

	domainNameResolved := *domainName
	if domainNameResolved == "" {
		domainNameResolved = defaultInitProjectDomain
	}

	today := *todayFlag
	if today == "" {
		today = time.Now().Format("2006-01-02")
	}

	written, err := initProject(dir, domainNameResolved, today)
	if err != nil {
		return err
	}
	for _, p := range written {
		fmt.Println(relPathForDisplay(p))
	}

	domainRel := filepath.Join("domains", domainNameResolved)
	fmt.Printf("initialized project at %s (base domain %q under %s)\n", relPathForDisplay(dir), domainNameResolved, domainRel)
	fmt.Println("next steps:")
	fmt.Printf("  cd %s\n", rawDir)
	fmt.Printf("  # draft a ProposedRequirement JSON (see PROPOSAL-REFERENCE.md), then:\n")
	fmt.Printf("  hotam land <proposal.json> --domain %s --today %s\n", domainRel, today)
	fmt.Printf("  hotam what-now --domain %s\n", domainRel)
	fmt.Printf("  hotam all-violations --domain %s\n", domainRel)
	// The marker now records the active domain (paths.WriteActiveDomain at
	// scaffold time), so a bare `hotam <command>` with no --domain resolves to
	// domains/<name> from inside the project (resolveDomain tier-3 marker
	// resolution). --domain <path> remains available for scripts/explicitness.
	fmt.Printf("  # the marker records the active domain, so a bare `hotam <command>` (no --domain)\n")
	fmt.Printf("  # works from inside %s; --domain %s remains available for scripts/explicitness.\n", rawDir, domainRel)
	return nil
}

// initProject performs the full project bootstrap and returns every path it
// wrote, in write order, so cmdInitProject and the init-project tests can both
// assert on exactly what landed on disk (mirroring initDomain's return-list-of-
// written-paths convention). It refuses to overwrite an existing project:
//   - <dir>/.hotam-spec-project already exists → a project was already
//     bootstrapped here (re-running init-project would clobber its crystal).
//   - <dir>/CLAUDE.md already exists → the project root already holds a crystal
//     (either a prior init-project or a hand-maintained one), which gen-spec
//     would silently overwrite.
//
// Both checks mirror initDomain's own "refusing to init: %s already exists"
// discipline for graph.json — same spirit (never silently destroy existing
// work), new guard points appropriate to a project-root scaffold.
func initProject(dir, domainName, today string) ([]string, error) {
	// (1) Refuse to overwrite an existing project. Check both guard points
	// BEFORE writing anything, so a refusal leaves the target untouched.
	markerPath := filepath.Join(dir, paths.MarkerFilename)
	if _, err := os.Stat(markerPath); err == nil {
		return nil, fmt.Errorf("refusing to init-project: %s already exists (project already bootstrapped at %s)", markerPath, dir)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("stat %s: %w", markerPath, err)
	}
	claudeMDPath := filepath.Join(dir, "CLAUDE.md")
	if _, err := os.Stat(claudeMDPath); err == nil {
		return nil, fmt.Errorf("refusing to init-project: %s already exists (project root already holds a crystal)", claudeMDPath)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("stat %s: %w", claudeMDPath, err)
	}

	// (2) Scaffold the base domain at <dir>/domains/<name> via the EXISTING
	// initDomain — do not reimplement domain scaffolding.
	domainDir := filepath.Join(dir, "domains", domainName)
	written, err := initDomain(domainDir, domainName)
	if err != nil {
		return nil, err
	}

	// (2b) initDomain already writes gen_profile: consumer into the
	// manifest (R8-e: unified with init-project's own historical default), so
	// no second manifest write is needed here. genSpec's profile parameter is
	// passed "" below so it resolves from that manifest, making the
	// consumer-file-count reduction visible immediately on the very first
	// gen-spec inside init-project's own pipeline.

	// (3) Write the project-root marker, recording the scaffolded domain as the
	// active-domain preference. paths.WriteActiveDomain writes
	// {"active_domain": "<name>"} as 2-space-indented JSON with a trailing
	// newline. internal/paths R4 resolution (searchMarkerFileUpward) only ever
	// checks file EXISTENCE via os.Stat — it never parses content — so this
	// JSON payload is purely additive and does not change project-root
	// detection. It lets a bare `hotam <command>` resolve to domains/<name>
	// from inside the project (resolveDomain tier 3).
	if err := paths.WriteActiveDomain(markerPath, domainName); err != nil {
		return written, fmt.Errorf("write %s: %w", markerPath, err)
	}
	written = append(written, markerPath)

	// (4) Generate the root crystal + all domain docs via the EXISTING genSpec.
	// claudeMDPath points at <dir>/CLAUDE.md so the crystal (CLAUDE.md +
	// AGENTS.md + GEMINI.md) lands at the project root and docs/gen/* lands
	// under <dir>/domains/<name>/docs/gen/. repoRootForDomain (gen_spec.go)
	// derives the repo root from the domains/<name> layout, so the DOMAIN-MAP
	// block lists the scaffolded domain correctly with no extra plumbing.
	genWritten, _, err := genSpec(domainDir, claudeMDPath, today, "")
	if err != nil {
		return written, err
	}
	written = append(written, genWritten...)

	return written, nil
}
