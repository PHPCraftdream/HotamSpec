package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// cmdInit implements `hotam init <dir> [--name <domain-name>] [--profile
// consumer|full] [--require-provenance]` — the scaffold command that closes
// the "applicability to external projects" gap (TaskList P1-7 / applicability
// score 3/10): before
// this command existed, a team wanting to adopt Hotam-Spec in ITS OWN
// repository had to hand-write a graph.json from scratch (see
// docs/QUICKSTART-CONSUMER.md step 2's `cat > graph.json <<'EOF' ... EOF`
// bootstrap) with no scaffold and no e2e proof the `hotam` binary even works
// outside this repo's own checkout.
//
// init creates the minimal on-disk shape a domain needs to be immediately
// usable:
//
//   - <dir>/graph.json — a genuinely EMPTY graph (0 axes/stakeholders/
//     requirements/conflicts/assumptions/operators/processes/goals/
//     entity_types/entities). Task #364 retired the earlier auto-seeded
//     Stakeholder ("owner") + SETTLED Requirement ("R-domain-exists") this
//     scaffold used to write: an empty graph already passes every
//     structural invariant by construction (R-empty-content-wellformed,
//     internal/invariants/empty_content_test.go), so `all-violations` is 0
//     the instant the domain is created with no need for a worked example
//     to make that true — and a business-empty graph is also what
//     `hotam gen-spec` now (same task) treats as ZERO docs/gen/ output
//     (no REQUIREMENTS.md/OPEN.md/etc, no docs/gen/ directory at all): a
//     genuinely fresh domain should carry no generated files until the
//     adopter models something real, rather than a permanent worked-example
//     artifact nobody asked for that they would otherwise have to reject.
//   - <dir>/docs/gen/ — created empty; `hotam gen-spec --domain <dir>`
//     populates it (init deliberately does NOT call gen-spec itself, so
//     `hotam init` stays a pure scaffold step and the doc-generation step
//     stays observable/separate, matching QUICKSTART-CONSUMER.md's own
//     step-by-step structure).
//   - <dir>/manifest.json — {"self_hosting": false, "gen_profile":
//     "consumer", "parent": null}, so internal/loader.resolveSelfHosting reads
//     a real, explicit value instead of silently defaulting via a missing
//     file, and ResolveGenProfile resolves to the consumer profile —
//     matching init-project's own default so both onboarding paths are
//     consistent (R8-e: --profile full overrides this to the heavier
//     full-profile output set for a domain that needs
//     framework-self-hosting-style docs). "parent": null is initDomain's
//     own unconditional default (PLAN-scenario-generated-spec.md §2 D6,
//     task W6.1/W6.2): a domain scaffolded via bare `hotam init` with no
//     wrapping project and no --parent flag is, by definition, a ROOT domain
//     unless told otherwise, so it satisfies check_project_parent_declared
//     (internal/invariants/project_parent.go) the instant `hotam init`
//     returns — no migration window, matching the domain's own born-clean
//     discipline (task #364: it is born with zero nodes, which already
//     passes every structural invariant). --parent <name> overrides this default to a
//     child declaration ("parent": "<name>") for a caller founding a domain
//     that IS a sub-project of an existing one; this flag does NOT validate
//     that a domain named <name> actually exists on disk (matching how
//     --domain flags elsewhere in this codebase don't validate against a
//     live filesystem lookup either — that is out of scope for this command).
//     --require-provenance additionally sets "require_provenance": true in
//     this same manifest, so internal/loader.ResolveRequireProvenance (task
//     #158) reports true from the very first `hotam land` in this domain —
//     no hand-editing manifest.json after scaffolding required (R12-b).
//     All overrides (--profile, --require-provenance, --parent) are composed
//     into ONE manifest write when any is set, so combining them never lets
//     one flag's write silently discard another's.
//   - <dir>/README.md — a short pointer back at the graph + the `hotam`
//     commands to run next, so a directory listing alone orients a human.
//
// domainDir passed on the command line need not live anywhere near this
// repository or contain a domains/ ancestor — resolveDomain(--domain) and
// this function both take the path as-is (filepath.Abs, no upward marker
// search), which is exactly what an external project's own repo root
// requires (see external_e2e_test.go, which builds the hotam binary into
// an os.MkdirTemp directory OUTSIDE this repo's working tree and drives
// the full init -> apply-proposal -> land -> req -> what-now -> gen-spec
// -> all-violations sequence from there).
func cmdInit(args []string) error {
	fs := newFlagSet("init")
	name := fs.String("name", "", "domain name (default: the last path segment of <dir>)")
	profile := fs.String("profile", "", "gen-spec profile: consumer|full (default: consumer, matching init-project; full produces the heavier framework-self-hosting doc set)")
	todayFlag := fs.String("today", "", "date in YYYY-MM-DD format (default: system date) — reserved for reproducible/byte-identical scaffolding (currently unused: task #364 removed the seed Requirement this flag used to date-stamp)")
	requireProvenance := fs.Bool("require-provenance", false, "require source_refs/last_reviewed_at/review_after on every SETTLED requirement landed into this domain (writes require_provenance: true into manifest.json; see internal/loader.ResolveRequireProvenance)")
	parentFlag := fs.String("parent", "", "name of this domain's parent domain (default: none — this is a root domain, written as \"parent\": null; PLAN-scenario-generated-spec.md §2 D6). Not validated against a live filesystem lookup.")
	if err := parseCommandFlags(fs, args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("usage: hotam init <dir> [--name <domain-name>] [--profile consumer|full] [--today YYYY-MM-DD] [--require-provenance] [--parent <parent-domain-name>]")
	}
	rawDir := fs.Arg(0)

	switch *profile {
	case "", loader.GenProfileConsumer, loader.GenProfileFull:
		// valid — empty falls through to initDomain's consumer default.
	default:
		return fmt.Errorf("--profile must be %q, %q, or empty (default consumer), got %q", loader.GenProfileConsumer, loader.GenProfileFull, *profile)
	}

	domainDir, err := filepath.Abs(rawDir)
	if err != nil {
		return fmt.Errorf("resolve <dir> %q: %w", rawDir, err)
	}

	domainName := *name
	if domainName == "" {
		domainName = filepath.Base(domainDir)
	}

	today := *todayFlag
	if today == "" {
		today = time.Now().Format("2006-01-02")
	}

	written, err := initDomain(domainDir, domainName, today)
	if err != nil {
		return err
	}

	// initDomain defaults to the consumer gen-spec profile (matching
	// init-project), require_provenance omitted (false), and "parent": null
	// (this domain is a root, per D6 — see initDomain's own doc comment).
	// --profile full, --require-provenance, and/or --parent <name> override
	// those defaults. All three flags are resolved together into ONE final
	// manifest write, rather than independent blind overwrites layered on
	// top of each other — a prior version of this code rewrote the WHOLE
	// manifest.json for --profile full alone, which would have silently
	// discarded --require-provenance if it wrote independently. Composing
	// here means the flags can never clobber each other, however combined.
	if *profile == loader.GenProfileFull || *requireProvenance || *parentFlag != "" {
		manifestProfile := loader.GenProfileConsumer
		if *profile == loader.GenProfileFull {
			manifestProfile = loader.GenProfileFull
		}
		manifest := fmt.Sprintf("{\"self_hosting\": false, \"gen_profile\": %q", manifestProfile)
		if *requireProvenance {
			manifest += ", \"require_provenance\": true"
		}
		if *parentFlag != "" {
			manifest += fmt.Sprintf(", \"parent\": %q", *parentFlag)
		} else {
			manifest += ", \"parent\": null"
		}
		manifest += "}\n"

		manifestPath := filepath.Join(domainDir, "manifest.json")
		if err := writeFileMkdir(manifestPath, []byte(manifest)); err != nil {
			return err
		}
	}

	for _, p := range written {
		fmt.Println(relPathForDisplay(p))
	}
	fmt.Printf("initialized domain %q at %s\n", domainName, relPathForDisplay(domainDir))
	fmt.Println("next: hotam gen-spec --domain " + rawDir)
	return nil
}

// seedReviewCadenceDays was the review interval applied to initDomain's old
// auto-seeded requirement's review_after, measured from `today` (the
// domain's scaffold date) — 180 days (~6 months), long enough that a
// freshly-scaffolded domain didn't immediately trip freshness/what-now's
// DUE-SOON lookahead (internal/freshness.DueSoonWindowDays == 30 days),
// short enough that a domain left completely untouched for a long time
// still eventually surfaced its seed requirement for review. Task #364
// retired that auto-seed (initDomain now writes a genuinely empty graph),
// so this constant is unused in production code today; it is kept
// (alongside addDaysLocal below) for test fixtures that reconstruct the old
// seed's exact shape when a test's real subject needs non-empty domain
// content (e.g. exercising gen-spec's consumer-vs-full profile behavior,
// which an empty graph would trivially skip end to end).
const seedReviewCadenceDays = 180

// initDomain performs the actual scaffold and returns every path it wrote,
// in write order, so cmdInit and external_e2e_test.go can both assert on
// exactly what landed on disk. It refuses to overwrite an existing
// graph.json (initializing on top of a real domain would silently discard
// it), but tolerates (and creates) an otherwise-empty target directory.
//
// today (YYYY-MM-DD) is threaded through for API/call-site stability with
// cmdInit/initProject (both pin it for reproducible scaffolding elsewhere in
// their own output) and with the many existing tests that already pass a
// pinned date. Task #364 retired the auto-seeded Requirement whose
// last_reviewed_at/review_after this parameter used to seed (see
// seedReviewCadenceDays/addDaysLocal below, now unused in THIS function but
// kept — package-private, zero cost — for test fixtures that want to
// replicate the old seed's freshness shape); an empty graph has no
// Requirement to carry a freshness field at all, so `today` is currently a
// reserved/unused parameter here, not a silent behavior change for any
// caller.
func initDomain(domainDir, domainName, today string) ([]string, error) {
	_ = today // see doc comment above — reserved, currently unused now that no seed Requirement exists.
	graphPath := graphPathForDomain(domainDir)
	if _, err := os.Stat(graphPath); err == nil {
		return nil, fmt.Errorf("refusing to init: %s already exists", graphPath)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("stat %s: %w", graphPath, err)
	}

	// A genuinely empty graph (task #364): 0 axes/stakeholders/requirements/
	// conflicts/assumptions/operators/processes/goals/entity_types/entities.
	// ontology.Graph{}.IsEmpty() is true by construction, and an empty graph
	// passes every structural invariant (R-empty-content-wellformed) — no
	// seed content is needed to make a freshly-scaffolded domain
	// invariant-clean.
	g := &ontology.Graph{}

	if err := loader.WriteGraph(graphPath, g); err != nil {
		return nil, fmt.Errorf("write %s: %w", graphPath, err)
	}
	written := []string{graphPath, loader.LockPath(graphPath)}

	manifestPath := filepath.Join(domainDir, "manifest.json")
	if err := writeFileMkdir(manifestPath, []byte("{\"self_hosting\": false, \"gen_profile\": \"consumer\", \"parent\": null}\n")); err != nil {
		return nil, err
	}
	written = append(written, manifestPath)

	genDir := filepath.Join(domainDir, "docs", "gen")
	if err := os.MkdirAll(genDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", genDir, err)
	}

	readmePath := filepath.Join(domainDir, "README.md")
	readme := fmt.Sprintf(readmeTemplate, domainName)
	if err := writeFileMkdir(readmePath, []byte(readme)); err != nil {
		return nil, err
	}
	written = append(written, readmePath)

	return written, nil
}

// addDaysLocal returns the YYYY-MM-DD date `days` days after date. On parse
// failure it returns date unchanged. This mirrors internal/freshness's own
// unexported addDays helper — that one is package-private to
// internal/freshness, so cmd/hotam carries this small local equivalent
// rather than exporting cross-package date arithmetic for a single use.
func addDaysLocal(date string, days int) string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}
	return t.AddDate(0, 0, days).Format("2006-01-02")
}

const readmeTemplate = `# %[1]s — a Hotam-Spec domain

Scaffolded by ` + "`hotam init`" + `. This directory holds one Hotam-Spec domain:
a graph of Requirements, Stakeholders, Conflicts and Assumptions in
` + "`graph.json`" + `, plus generated readable views under ` + "`docs/gen/`" + `.

## Next steps

` + "```bash" + `
# render readable docs/gen/*.md from graph.json
hotam gen-spec --domain .

# confirm the graph is structurally sound (should print "0 violations")
hotam all-violations --domain .

# see the next correct action
hotam what-now --domain .
` + "```" + `

This domain starts genuinely empty (0 nodes) — ` + "`hotam gen-spec`" + ` writes
nothing under ` + "`docs/gen/`" + ` until you land your first real content (a
Stakeholder, a Requirement, ...); an empty graph is well-formed by
construction (R-empty-content-wellformed), not an error to work around.

## Founding order (general before specific, authored by hand)

Hotam-Spec is an authored-executable-specification discipline: an agent who
understands the domain writes its model, behavior, and tests BY HAND; the
engine holds the structural floor and generates only derived projections
(REQUIREMENTS/MODELS/TRACEABILITY/COVERAGE.md), never the authored code
itself. Grow a new domain in this order, never bottom-up from requirements
(R-domain-founded-in-wave-order):

1. Purpose, stakeholders, terminology — manifest purpose/goals + Stakeholders + Axes.
2. Process and its scenarios — a Process node naming stages, roles, drives_entities.
3. Object model, by hand (` + "`spec/model/`" + `) — all aggregates/value-objects and their fields.
4. Operations, policies, invariants, by hand (` + "`spec/application/`" + `, ` + "`spec/policy/`" + `).
5. Executable tests, by hand (` + "`spec/tests/`" + `, or beside the code they exercise).
6. Requirements, linked to that code+tests via ` + "`implemented_by`" + `/` + "`verified_by`" + `
   (R-spec-link-embodied-vs-proven) — ENFORCED requires both, real and resolvable.
7. Assumptions and conflicts.
8. Generated documentation and coverage audit — ` + "`hotam gen-spec`" + `.

Close every step with ` + "`hotam gen-spec`" + ` and read
` + "`docs/gen/PIPELINE.md`" + ` — the generated domain overview
(R-domain-overview-projection), your second document after the Domain Map.

## Making changes

The graph is never hand-edited.

- **Requirements in code** (manifest ` + "`requirements_authority: code`" + `, the
  ` + "`hotam init-project`" + ` default): edit the scenario test and the requirement
  literal in ` + "`spec/requirements.go`" + `, run ` + "`hotam sync-domain --domain .`" + `
  (dry-run: prints the diff and a hash), show the diff to the owner, then
  ` + "`hotam sync-domain --domain . --today YYYY-MM-DD --confirm-hash <hex>`" + `.
  Finish with ` + "`hotam all-violations --domain .`" + ` = 0. ` + "`hotam land`" + ` refuses
  Requirement/Rejection JSON on such a domain.
- **Everything else** (conflicts, assumptions, and requirements on a domain
  without code authority): ` + "`hotam land <proposal.json> --domain . --today YYYY-MM-DD`" + `
  (apply + regenerate docs + re-verify in one step; fails closed — writes
  nothing — if the change would introduce a new invariant violation). See
  PROPOSAL-REFERENCE.md in the Hotam-Spec repo for the JSON shape of every
  proposal kind.
`
