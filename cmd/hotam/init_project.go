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
// BORN FULLY OBLIGATED (PLAN-scenario-generated-spec.md §2 D6, task W6.2):
// the scaffolded base domain is immediately (a) "parent": null in its
// manifest.json (it is the root of the new external project, within the
// framework's own hierarchy — see W6.1's manifest.parent mechanism), (b)
// "discipline": "full" in its manifest.json, UNCONDITIONALLY — not an opt-in
// flag the way --require-provenance is, because a brand-new project has no
// prior soft-discipline history to migrate FROM ("у новых проектов нет
// миграционного окна, обязанность действует с рождения" — no migration
// window, the obligation acts from birth), and (c) equipped with a real
// spec/go.mod + the vendored hotamspec scenario recorder
// (spec/hotamspec/hotamspec.go), so the discipline it is born under is
// immediately actionable, not just declared. The seed requirement initDomain
// writes (R-domain-exists) already satisfies check_settled_requires_scenario
// under discipline:full via the INHERENTLY_PROSE exemption (it carries
// Enforceability: INHERENTLY_PROSE with no carrier) — see
// internal/invariants/scenario_discipline.go — so the freshly-scaffolded
// domain passes a REAL `hotam all-violations` run with ZERO manual
// follow-up, proving the "no migration window" promise holds from the very
// first command.
//
// It is a pure composition of the two existing primitives — initDomain (the
// per-domain scaffold) and genSpec (the doc/crystal generator) — plus a
// marker-file write and the vendored-recorder scaffold (vendorRecorder,
// cmd/hotam/vendor_recorder.go). It does not reimplement any of them: a
// future change to domain scaffolding, doc generation, or recorder vendoring
// flows through init-project for free.
func cmdInitProject(args []string) error {
	fs := newFlagSet("init-project")
	domainName := fs.String("domain", "", "name of the base domain to scaffold under <dir>/domains/ (default: "+defaultInitProjectDomain+")")
	todayFlag := fs.String("today", "", "date in YYYY-MM-DD format (default: system date) — embedded in freshness/status lines of the generated docs and root crystal; pin this for reproducible/byte-identical regeneration")
	requireProvenance := fs.Bool("require-provenance", false, "require source_refs/last_reviewed_at/review_after on every SETTLED requirement landed into the base domain (writes require_provenance: true into its manifest.json; see internal/loader.ResolveRequireProvenance)")
	fs.Parse(args)

	if fs.NArg() < 1 {
		return fmt.Errorf("usage: hotam init-project <dir> [--domain <name>] [--today YYYY-MM-DD] [--require-provenance]")
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

	written, err := initProject(dir, domainNameResolved, today, *requireProvenance)
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
	fmt.Printf("  # found the domain skeleton-first (R-domain-founded-in-wave-order): land the\n")
	fmt.Printf("  # skeleton (purpose/goals + a ProposedProcess naming stages and roles + Axes)\n")
	fmt.Printf("  # BEFORE the first ProposedRequirement — see %s/README.md \"Founding order\".\n", domainRel)
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
func initProject(dir, domainName, today string, requireProvenance bool) ([]string, error) {
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
	written, err := initDomain(domainDir, domainName, today)
	if err != nil {
		return nil, err
	}

	// (2b) initDomain already writes gen_profile: consumer and "parent": null
	// into the manifest (R8-e: unified with init-project's own historical
	// default; "parent": null per W6.1/D6 — this base domain has no parent
	// WITHIN the framework's own hierarchy, it IS the root of the new
	// external project). This step layers "discipline": "full" into the SAME
	// composed manifest write, UNCONDITIONALLY — not gated behind a flag the
	// way --require-provenance is below. PLAN-scenario-generated-spec.md §2
	// D6 (task W6.2): "init-project создаёт под-проект СРАЗУ с
	// discipline: full ... у новых проектов нет миграционного окна,
	// обязанность действует с рождения" — a brand-new project scaffolded by
	// init-project has no prior soft-discipline history to migrate FROM, so
	// there is nothing to opt into; it is simply born under the stricter
	// discipline check_settled_requires_scenario enforces
	// (internal/invariants/scenario_discipline.go). --require-provenance
	// additionally adds "require_provenance": true while preserving
	// initDomain's own self_hosting/gen_profile/parent fields verbatim — one
	// single follow-up write, never a second independent blind overwrite
	// (R12-b; see cmdInit's own composition comment for the landmine this
	// avoids when combining flags). genSpec's profile parameter is passed ""
	// below so it resolves from that manifest, making the consumer-file-count
	// reduction visible immediately on the very first gen-spec inside
	// init-project's own pipeline.
	manifestPath := filepath.Join(domainDir, "manifest.json")
	manifest := "{\"self_hosting\": false, \"gen_profile\": \"consumer\", \"parent\": null, \"discipline\": \"full\""
	if requireProvenance {
		manifest += ", \"require_provenance\": true"
	}
	manifest += "}\n"
	if err := writeFileMkdir(manifestPath, []byte(manifest)); err != nil {
		return written, fmt.Errorf("write %s: %w", manifestPath, err)
	}

	// (2c) Scaffold the vendored hotamspec scenario recorder from birth —
	// the second half of "born fully obligated" (PLAN-scenario-generated-
	// spec.md §2 D6, task W6.2): a domain born discipline:"full" needs a
	// real spec/ Go module and a vendored recorder available immediately,
	// not after a manual follow-up step. Two sub-steps, in order:
	//
	//   (i) write <domainDir>/spec/go.mod — a fresh, minimal Go module.
	//       Module-naming convention (module <domain-name>-spec, go 1.25)
	//       mirrors the established shape already used by every consumer
	//       domain in the sibling PRAT-hotam repo (domains/prat/spec/go.mod,
	//       domains/gpsm-sm/spec/go.mod), for consistency across the whole
	//       framework's consumer domains.
	//   (ii) call the EXISTING vendorRecorder(domainDir) (cmd/hotam/
	//        vendor_recorder.go) — do NOT reimplement the vendoring logic,
	//        it already copies the canonical recorder source (internal/
	//        recorder/canon/hotamspec.go), banner-stamped do-not-edit, into
	//        spec/hotamspec/hotamspec.go. It REQUIRES spec/go.mod to exist
	//        first (see its own doc comment), hence (i) before (ii).
	specGoModPath := filepath.Join(domainDir, "spec", "go.mod")
	specGoMod := fmt.Sprintf("module %s-spec\n\ngo 1.25\n", domainName)
	if err := writeFileMkdir(specGoModPath, []byte(specGoMod)); err != nil {
		return written, fmt.Errorf("write %s: %w", specGoModPath, err)
	}
	written = append(written, specGoModPath)

	recorderPath, err := vendorRecorder(domainDir)
	if err != nil {
		return written, fmt.Errorf("vendor recorder into %s: %w", domainDir, err)
	}
	written = append(written, recorderPath)

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
	//
	// includeSpec=true (task W7.2, @fx finding F3 fix): this base domain is
	// ALWAYS scaffolded discipline:"full" (step 2b above), and F3's fix to
	// check_spec_md_current now fires a violation for ANY discipline:full
	// domain with no docs/gen/SPEC.md on disk -- so init-project must render
	// SPEC.md from birth, or the freshly-scaffolded domain would immediately
	// fail its own first `all-violations` run and violate D6's "no migration
	// window" promise (the exact contract task #273/W6.2 exists to prove).
	// A brand-new domain has zero verified_by entries at this point (only the
	// INHERENTLY_PROSE seed requirement), so this BuildSpec pass is a fast,
	// near-empty render -- no real `go test` recording work exists to do yet.
	genWritten, _, err := genSpec(domainDir, claudeMDPath, today, "", true)
	if err != nil {
		return written, err
	}
	written = append(written, genWritten...)

	return written, nil
}
