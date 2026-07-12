package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/PHPCraftdream/HotamSpecGo/internal/loader"
	"github.com/PHPCraftdream/HotamSpecGo/internal/ontology"
)

// cmdInit implements `hotam init <dir> [--name <domain-name>]` — the
// scaffold command that closes the "applicability to external projects"
// gap (TaskList P1-7 / applicability score 3/10): before this command
// existed, a team wanting to adopt Hotam-Spec in ITS OWN repository had to
// hand-write a graph.json from scratch (see docs/QUICKSTART-CONSUMER.md
// step 2's `cat > graph.json <<'EOF' ... EOF` bootstrap) with no scaffold
// and no e2e proof the `hotam` binary even works outside this repo's own
// checkout.
//
// init creates the minimal on-disk shape a domain needs to be immediately
// usable:
//
//   - <dir>/graph.json — a graph with one seed Stakeholder and one seed
//     SETTLED Requirement (both PROSE-enforced, so `all-violations` is 0
//     the instant the domain is created — no dangling owner references,
//     no unenforceable-but-claimed-ENFORCED nodes). An empty graph (no
//     nodes at all) also passes every invariant, as verified in the e2e
//     test below and used by cmdGate/cmdWhatNow's own fixtures, but a
//     single worked stakeholder+requirement pair gives an adopter
//     something to `hotam req show` / `hotam what-now` against
//     immediately instead of a wall of "nothing here yet".
//   - <dir>/docs/gen/ — created empty; `hotam gen-spec --domain <dir>`
//     populates it (init deliberately does NOT call gen-spec itself, so
//     `hotam init` stays a pure scaffold step and the doc-generation step
//     stays observable/separate, matching QUICKSTART-CONSUMER.md's own
//     step-by-step structure).
//   - <dir>/manifest.json — {"self_hosting": false}, so
//     internal/loader.resolveSelfHosting reads a real, explicit value
//     instead of silently defaulting via a missing file.
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
	fs.Parse(args)

	if fs.NArg() < 1 {
		return fmt.Errorf("usage: hotam init <dir> [--name <domain-name>]")
	}
	rawDir := fs.Arg(0)

	domainDir, err := filepath.Abs(rawDir)
	if err != nil {
		return fmt.Errorf("resolve <dir> %q: %w", rawDir, err)
	}

	domainName := *name
	if domainName == "" {
		domainName = filepath.Base(domainDir)
	}

	written, err := initDomain(domainDir, domainName)
	if err != nil {
		return err
	}
	for _, p := range written {
		fmt.Println(relPathForDisplay(p))
	}
	fmt.Printf("initialized domain %q at %s\n", domainName, relPathForDisplay(domainDir))
	fmt.Println("next: hotam gen-spec --domain " + rawDir)
	return nil
}

// initDomain performs the actual scaffold and returns every path it wrote,
// in write order, so cmdInit and external_e2e_test.go can both assert on
// exactly what landed on disk. It refuses to overwrite an existing
// graph.json (initializing on top of a real domain would silently discard
// it), but tolerates (and creates) an otherwise-empty target directory.
func initDomain(domainDir, domainName string) ([]string, error) {
	graphPath := graphPathForDomain(domainDir)
	if _, err := os.Stat(graphPath); err == nil {
		return nil, fmt.Errorf("refusing to init: %s already exists", graphPath)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("stat %s: %w", graphPath, err)
	}

	seedOwnerID := "owner"
	seedRequirementID := "R-domain-exists"

	g := &ontology.Graph{
		Stakeholders: []ontology.Stakeholder{
			{
				ID:     seedOwnerID,
				Name:   "Domain Owner",
				Domain: "owns this domain's seed requirement; replace or extend via Stakeholder proposals",
			},
		},
		Requirements: []ontology.Requirement{
			{
				ID:             seedRequirementID,
				Claim:          fmt.Sprintf("The %q domain shall exist as a valid, invariant-clean Hotam-Spec graph.", domainName),
				Owner:          seedOwnerID,
				Status:         ontology.StatusSETTLED,
				Why:            "Seed requirement created by `hotam init` as a worked example — replace it (via a Rejection + a real proposal) with your domain's first actual requirement.",
				Enforcement:    ontology.EnforcementPROSE,
				Enforceability: ontology.EnforceabilityINHERENTLY_PROSE,
			},
		},
	}

	if err := loader.WriteGraph(graphPath, g); err != nil {
		return nil, fmt.Errorf("write %s: %w", graphPath, err)
	}
	written := []string{graphPath, loader.LockPath(graphPath)}

	manifestPath := filepath.Join(domainDir, "manifest.json")
	if err := writeFileMkdir(manifestPath, []byte("{\"self_hosting\": false}\n")); err != nil {
		return nil, err
	}
	written = append(written, manifestPath)

	genDir := filepath.Join(domainDir, "docs", "gen")
	if err := os.MkdirAll(genDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", genDir, err)
	}

	readmePath := filepath.Join(domainDir, "README.md")
	readme := fmt.Sprintf(readmeTemplate, domainName, seedRequirementID)
	if err := writeFileMkdir(readmePath, []byte(readme)); err != nil {
		return nil, err
	}
	written = append(written, readmePath)

	return written, nil
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

# browse the seed requirement
hotam req show %[2]s --domain .
` + "```" + `

## Making changes

The graph is never hand-edited. Every change goes through
` + "`hotam apply-proposal <proposal.json> --domain . --today YYYY-MM-DD`" + `
(or ` + "`hotam land`" + ` to apply + regenerate docs + re-verify in one step),
which fails closed — writes nothing — if the change would introduce a new
invariant violation. See PROPOSAL-REFERENCE.md in the Hotam-Spec repo for
the full JSON shape of every proposal kind.
`
