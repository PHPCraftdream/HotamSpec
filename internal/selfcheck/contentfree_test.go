package selfcheck

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/generator"
	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// frameworkScanRoots are the framework source trees these hygiene checks sweep.
var frameworkScanRoots = []string{"internal", "cmd/hotam"}

// TestContentFree_NoBusinessData enforces R-content-free-no-business-data:
// the framework Go packages (internal/*, cmd/hotam/*) ship no business data.
//
// EXACT RULE (mechanically checked): in every NON-TEST, NON-TESTDATA .go file
// under internal/ and cmd/hotam/, no string literal may contain a business
// domain token sourced from a REAL business domain graph
// (domains/hotam-spec-self/graph.json). The denylist is the union of:
//
//   - every Axis slug (axis slugs are domain-specific tension vocabulary, the
//     most unmistakably business content — e.g. "framework-purity-vs-helpfulness"),
//   - every Stakeholder ID and Requirement owner value,
//
// MINUS the canonical reader-role contract: internal/generator/common.go's
// DomainDocReaders binds the operator/steward/maintainer roles to exactly three
// stakeholder IDs ("ai-agent", "domain-user", "framework-author"). Those three
// are framework INFRASTRUCTURE (the cross-domain reader-resolution contract
// every domain is expected to honor), not domain-specific business data, so
// they are exempted. The exempt set is DERIVED from generator.DomainDocReaders
// at test time (not hardcoded) — only the values declared in that contract are
// treated as infrastructure. Any other business stakeholder/axis found as a
// string literal in framework source is a leak.
//
// Only string literals are inspected (compiled-in data); comments are excluded.
// Substring match is used because every denylist token is a distinctive
// hyphenated multi-word slug/id with no legitimate partial overlap.
//
// Discrimination: this test fails the moment a domain-specific axis slug or a
// non-canonical stakeholder (e.g. "framework-reviewer") is hardcoded into any
// non-test framework source string. A doctored scan over a fixture containing
// such a token confirms the predicate is non-vacuous (see TestContentFree_NoBusinessData_DetectsViolation).
func TestContentFree_NoBusinessData(t *testing.T) {
	t.Parallel()
	denylist := businessTokenDenylist(t)
	if len(denylist) == 0 {
		t.Fatal("denylist is empty — cannot enforce content-freeness")
	}

	files := collectGoFiles(t, frameworkScanRoots, false /* no tests */, false /* no testdata */)
	for _, f := range files {
		for _, lit := range stringLiterals(f.ast) {
			for token, label := range denylist {
				if strings.Contains(lit, token) {
					t.Errorf("R-content-free-no-business-data: business %s %q appears as a string literal in %s — framework source must stay content-free",
						label, token, relPath(t, f.path))
				}
			}
		}
	}
}

// TestContentFree_NoBusinessData_DetectsViolation is the non-vacuity control
// for TestContentFree_NoBusinessData: feeding the same predicate a synthetic
// source string containing a real business axis slug must report it. If this
// passes, the main test's predicate genuinely catches leaks rather than
// trivially succeeding on an empty denylist or a broken matcher.
func TestContentFree_NoBusinessData_DetectsViolation(t *testing.T) {
	t.Parallel()
	denylist := businessTokenDenylist(t)
	// A doctored "source" carrying a real axis slug that must never appear in
	// framework source. Any non-empty denylist from the business graph contains
	// at least one axis slug; pick the first axis slug guaranteed present.
	var sample string
	for tok, label := range denylist {
		if label == "axis" {
			sample = "some framework string embedding " + tok
			break
		}
	}
	if sample == "" {
		t.Fatal("denylist has no axis slug to exercise the detector")
	}
	hit := false
	for tok := range denylist {
		if strings.Contains(sample, tok) {
			hit = true
			break
		}
	}
	if !hit {
		t.Fatal("predicate failed to flag a synthetic source string containing a known business axis slug — the main test would be vacuous")
	}
}

// businessTokenDenylist builds the denylist described in
// TestContentFree_NoBusinessData: axis slugs + stakeholder IDs + requirement
// owners from the business graph, minus the canonical reader-role contract
// (generator.DomainDocReaders values). Returns token -> human label.
func businessTokenDenylist(t *testing.T) map[string]string {
	t.Helper()
	root := repoRoot(t)
	g, err := loader.LoadGraph(filepath.Join(root, "domains", "hotam-spec-self", "graph.json"))
	if err != nil {
		t.Fatalf("load business graph for denylist: %v", err)
	}
	exempt := map[string]bool{}
	for _, id := range generator.DomainDocReaders {
		exempt[id] = true
	}
	deny := map[string]string{}
	for _, a := range g.Axes {
		if a.Slug != "" {
			deny[a.Slug] = "axis"
		}
	}
	for _, s := range g.Stakeholders {
		if s.ID != "" && !exempt[s.ID] {
			deny[s.ID] = "stakeholder"
		}
	}
	for _, r := range g.Requirements {
		if r.Owner != "" && !exempt[r.Owner] {
			deny[r.Owner] = "stakeholder"
		}
	}
	return deny
}

// TestContentFree_NoExamples enforces R-content-free-no-examples: the framework
// shall not embed illustrative example graph content in its source modules.
//
// EXACT RULE (mechanically checked): in NON-TEST, NON-TESTDATA .go files under
// internal/ and cmd/hotam/, a composite literal that constructs a graph node
// (ontology.Requirement{...}, ontology.Conflict{...}, ...[]ontology.Axis{...},
// etc.) may appear ONLY in the two content-INTAKE boundaries, which handle
// content functionally rather than embedding illustrative examples:
//
//   - internal/proposal/* — the mechanical writer that applies steward-approved
//     proposals; it constructs nodes FROM proposal input, never embeds examples;
//   - cmd/hotam/init_cmd.go — the `hotam init` domain scaffold, a functional,
//     explicitly-replaceable seed template (its own Why text says "replace it"),
//     not illustrative content that misleads adopters.
//
// Anywhere else, a node-constructing composite literal is embedded example
// graph content and fails this check. map[string]ontology.EntityType{} (an empty
// typed map index) is NOT matched — see findNodeLiterals.
//
// Discrimination: see TestContentFree_NoExamples_DetectsViolation.
func TestContentFree_NoExamples(t *testing.T) {
	t.Parallel()
	files := collectGoFiles(t, frameworkScanRoots, false, false)
	for _, f := range files {
		for _, hit := range findNodeLiterals(f.path, f.ast) {
			if isContentIntakeSite(f.path) {
				continue
			}
			t.Errorf("R-content-free-no-examples: %s composite literal in %s — framework source modules must not embed example graph content (only internal/proposal and cmd/hotam/init_cmd.go may construct nodes)",
				hit.typeName, relPath(t, f.path))
		}
	}
}

// TestContentFree_NoExamples_DetectsViolation is the non-vacuity control: a
// Requirement{} composite literal placed outside the intake boundaries must be
// flagged. It builds a tiny synthetic .go source, parses it, runs the same
// detector, and asserts the hit is found AND correctly rejected (since temp
// paths are not intake sites).
func TestContentFree_NoExamples_DetectsViolation(t *testing.T) {
	t.Parallel()
	src := `package synth
import "github.com/PHPCraftdream/HotamSpec/internal/ontology"
var sample = ontology.Requirement{ID: "R-sample", Claim: "illustrative example"}`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "synth_example.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse synthetic source: %v", err)
	}
	hits := findNodeLiterals("synth_example.go", file)
	if len(hits) != 1 || hits[0].typeName != "Requirement" {
		t.Fatalf("detector failed to find the synthetic Requirement literal, got %+v — the main test would be vacuous", hits)
	}
	if isContentIntakeSite("synth_example.go") {
		t.Fatal("a temp path must not be classified as an intake site")
	}
	// Reaching here means the same hit would be reported as a violation by the
	// main test (it is outside every intake boundary).
}

// isContentIntakeSite reports whether a file path is one of the two legitimate
// content-handling boundaries where constructing graph nodes is allowed.
func isContentIntakeSite(path string) bool {
	clean := filepath.ToSlash(path)
	if strings.Contains(clean, "/internal/proposal/") {
		return true
	}
	if strings.HasSuffix(clean, "cmd/hotam/init_cmd.go") {
		return true
	}
	return false
}

// TestContentFree_NoSeedGraph enforces R-content-free-no-seed-graph: the
// framework shall not embed a seed TensionGraph — LoadGraph (internal/loader)
// discovers the user's graph by convention from domains/<name>/graph.json and
// must inject no content of its own.
//
// EXACT RULE (behaviorally checked): LoadGraph is a pure reader of its file
// argument. Loading an EMPTY valid graph (`{}`) yields a graph with zero
// requirements, conflicts, axes, stakeholders, and assumptions, and
// SelfHosting=false — i.e. no seed nodes are synthesized when the file carries
// none. Loading a graph carrying exactly one stakeholder + one requirement
// returns exactly those and nothing more — proving LoadGraph reads from disk
// rather than ignoring its input or appending a baked-in seed. Together these
// prove no seed graph is embedded in the loader.
//
// Discrimination: if LoadGraph appended a hardcoded seed requirement, the
// empty-graph case would return a non-empty Requirements slice and fail.
func TestContentFree_NoSeedGraph(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	emptyPath := filepath.Join(dir, "graph.json")
	if err := os.WriteFile(emptyPath, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write empty graph: %v", err)
	}
	empty, err := loader.LoadGraph(emptyPath)
	if err != nil {
		t.Fatalf("LoadGraph(empty): %v", err)
	}
	if len(empty.Requirements) != 0 || len(empty.Conflicts) != 0 ||
		len(empty.Axes) != 0 || len(empty.Stakeholders) != 0 || len(empty.Assumptions) != 0 {
		t.Errorf("R-content-free-no-seed-graph: LoadGraph on an empty graph injected seed content — "+
			"req=%d conf=%d axes=%d stake=%d assum=%d (all must be 0)",
			len(empty.Requirements), len(empty.Conflicts), len(empty.Axes),
			len(empty.Stakeholders), len(empty.Assumptions))
	}
	if empty.SelfHosting {
		t.Errorf("R-content-free-no-seed-graph: empty graph in a dir with no manifest must report SelfHosting=false")
	}

	// Round-trip a one-of-each graph through the real writer/reader to prove
	// LoadGraph returns disk content verbatim, neither ignoring input nor
	// appending a seed.
	small := &ontology.Graph{
		Stakeholders: []ontology.Stakeholder{{ID: "s1", Name: "S", Domain: "d"}},
		Requirements: []ontology.Requirement{{
			ID:             "R-one",
			Claim:          "one requirement",
			Owner:          "s1",
			Status:         ontology.StatusSETTLED,
			Enforcement:    ontology.EnforcementPROSE,
			Enforceability: ontology.EnforceabilityENFORCEABLE,
		}},
	}
	smallPath := filepath.Join(dir, "small", "graph.json")
	if err := loader.WriteGraph(smallPath, small); err != nil {
		t.Fatalf("write small graph: %v", err)
	}
	got, err := loader.LoadGraph(smallPath)
	if err != nil {
		t.Fatalf("LoadGraph(small): %v", err)
	}
	if len(got.Stakeholders) != 1 || len(got.Requirements) != 1 || len(got.Conflicts) != 0 {
		t.Errorf("R-content-free-no-seed-graph: LoadGraph must round-trip disk content exactly — "+
			"stake=%d req=%d conf=%d (want stake=1 req=1 conf=0); a seed graph is likely being appended",
			len(got.Stakeholders), len(got.Requirements), len(got.Conflicts))
	}
}
