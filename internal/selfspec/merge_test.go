package selfspec

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// domainGraphPath is the real, committed hotam-spec-self domain graph — THE
// point of Phase 0 (task #344, RAC-0) is proving the merge mechanism against
// real, messy, 300+-node production data, not a small synthetic fixture.
const domainGraphPath = "../../domains/hotam-spec-self/graph.json"

// wantRequirementCount is the full domains/hotam-spec-self/graph.json
// Requirement count as of task #345 (RAC-A): 253 SETTLED + 42 REJECTED + 6
// DRAFT = 301. TestMergeIntoGraph_AllRequirementsRegistered pins this exact
// number so a future requirement landing in the graph without a matching
// registry entry (or vice versa) fails loudly here instead of silently
// leaving the registry's coverage incomplete.
// 301 + 1: R-vendored-ontology-matches-engine-canon -- landed task #365
// (RAC2 Phase A) via `hotam sync-self`, the self-hosting anchor for
// check_ontology_vendor_current's orphan-enforcer gate (254 SETTLED + 42
// REJECTED + 6 DRAFT = 302).
// 302 + 1: R-opt-in-trigger-owns-its-own-obligations -- landed task #388
// (W0.1) via `hotam sync-self`, the self-hosting anchor for
// check_claim_authority_ratchet's orphan-enforcer gate (255 SETTLED + 42
// REJECTED + 6 DRAFT = 303).
// 303 + 2: task #395 (W1.3) via `hotam sync-self` rejected
// R-generations-inherit-doc-test-code in place (SETTLED -> REJECTED, an
// existing node's status flip -- not a node count change on its own) and
// added its two atomic successors, R-requirement-generation-mechanized-by-
// registry and R-entity-type-realized-by-go-symbol-never-generated (254
// SETTLED + 43 REJECTED + 6 DRAFT = 303, then +2 new SETTLED successors =
// 256 SETTLED + 43 REJECTED + 6 DRAFT = 305).
// 305 + 1: R-public-surface-authority-owns-its-own-obligations -- landed
// task #396 (W1.4) via `hotam sync-self`, the self-hosting anchor for
// check_public_surface_linked_or_marked/check_public_surface_authority_ratchet's
// orphan-enforcer gate (257 SETTLED + 43 REJECTED + 6 DRAFT = 306).
// 306 + 1: R-scenario-authority-owns-its-own-obligations -- landed task #397
// (W1.5) via `hotam sync-self`, the self-hosting anchor for
// check_scenario_quality/check_scenario_authority_ratchet's orphan-enforcer
// gate (258 SETTLED + 43 REJECTED + 6 DRAFT = 307).
const wantRequirementCount = 307

// TestMergeIntoGraph_ByteIdenticalRoundTrip is the entire point of Phase A
// (RAC-A, task #345, scaling Phase 0/RAC-0's proof to full coverage): load
// the REAL committed graph.json, run MergeIntoGraph over it (replacing EVERY
// registered requirement's structural fields with the registry's own
// values), re-serialize via loader.WriteGraph — REUSING the exact same
// canonical-marshal path graph.json is always written through, not a
// reimplementation of it — and assert the output is BYTE-IDENTICAL to the
// committed file. If the registry's mirrored structural fields do not
// exactly match what is already on disk, this test fails and prints a
// usable diff hint (first differing line, with context).
func TestMergeIntoGraph_ByteIdenticalRoundTrip(t *testing.T) {
	want, err := os.ReadFile(domainGraphPath)
	if err != nil {
		t.Fatalf("read %s: %v", domainGraphPath, err)
	}

	g, err := loader.LoadGraph(domainGraphPath)
	if err != nil {
		t.Fatalf("LoadGraph(%s): %v", domainGraphPath, err)
	}

	if len(Requirements.All()) == 0 {
		t.Fatal("selfspec.Requirements is empty — registered nothing, this test would be vacuous")
	}

	if err := MergeIntoGraph(g, Requirements); err != nil {
		t.Fatalf("MergeIntoGraph: %v", err)
	}

	got := writeGraphBytes(t, g)
	diffReport(t, domainGraphPath, string(got), string(want))
}

// TestMergeIntoGraph_AllRequirementsRegistered proves Phase A's full-coverage
// claim directly: every one of the graph's 301 Requirement nodes has a
// matching selfspec.Requirements entry, and the registry carries no more and
// no fewer than that — not merely that MergeIntoGraph succeeds on whatever
// subset happens to be registered (that weaker property is what Phase 0's
// original test proved; this test is Phase A's stronger claim).
func TestMergeIntoGraph_AllRequirementsRegistered(t *testing.T) {
	g, err := loader.LoadGraph(domainGraphPath)
	if err != nil {
		t.Fatalf("LoadGraph(%s): %v", domainGraphPath, err)
	}

	inGraph := make(map[string]bool, len(g.Requirements))
	for _, r := range g.Requirements {
		inGraph[r.ID] = true
	}

	registered := Requirements.All()
	if len(registered) != wantRequirementCount {
		t.Errorf("selfspec.Requirements has %d entries, want exactly %d (the full domains/hotam-spec-self/graph.json Requirement count)", len(registered), wantRequirementCount)
	}
	if len(g.Requirements) != wantRequirementCount {
		t.Fatalf("domains/hotam-spec-self/graph.json has %d Requirement nodes, want %d — wantRequirementCount is stale, update it (and re-run the codegen if the registry also needs to change)", len(g.Requirements), wantRequirementCount)
	}

	inRegistry := make(map[string]bool, len(registered))
	for _, r := range registered {
		inRegistry[r.ID] = true
		if !inGraph[r.ID] {
			t.Errorf("selfspec.Requirements has %q, absent from domains/hotam-spec-self/graph.json", r.ID)
		}
	}
	for id := range inGraph {
		if !inRegistry[id] {
			t.Errorf("domains/hotam-spec-self/graph.json has %q, missing from selfspec.Requirements", id)
		}
	}
}

// TestMergeIntoGraph_Idempotent proves running the merge twice produces
// exactly the same result as running it once: MergeIntoGraph replaces
// structural fields from the registry (a pure function of the registry
// state, unaffected by what was already in that field) and passes event
// fields through from whatever the graph currently holds, so a second pass
// over an already-merged graph must be a no-op.
func TestMergeIntoGraph_Idempotent(t *testing.T) {
	g, err := loader.LoadGraph(domainGraphPath)
	if err != nil {
		t.Fatalf("LoadGraph(%s): %v", domainGraphPath, err)
	}

	if err := MergeIntoGraph(g, Requirements); err != nil {
		t.Fatalf("MergeIntoGraph (first pass): %v", err)
	}
	once := writeGraphBytes(t, g)

	if err := MergeIntoGraph(g, Requirements); err != nil {
		t.Fatalf("MergeIntoGraph (second pass): %v", err)
	}
	twice := writeGraphBytes(t, g)

	diffReport(t, "idempotence (pass1 vs pass2)", string(twice), string(once))
}

// TestMergeIntoGraph_MissingRegistryIDIsError proves the narrow
// creation-forbidden contract: a registered ID absent from the graph is a
// hard error, never a silent skip or a graph mutation that invents a node.
func TestMergeIntoGraph_MissingRegistryIDIsError(t *testing.T) {
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-not-the-one-youre-looking-for"},
		},
	}
	// Requirements is the package-level, already-populated full registry
	// (all 301 domains/hotam-spec-self/graph.json requirement IDs); none of
	// them match the single node above, so MergeIntoGraph must fail on the
	// first registered ID it cannot find.
	if err := MergeIntoGraph(g, Requirements); err == nil {
		t.Fatal("MergeIntoGraph: want error when a registered ID is absent from the graph, got nil")
	}
}

// TestMergeIntoGraph_NilGraphIsError is the boundary-condition control.
func TestMergeIntoGraph_NilGraphIsError(t *testing.T) {
	if err := MergeIntoGraph(nil, Requirements); err == nil {
		t.Fatal("MergeIntoGraph(nil, Requirements): want error, got nil")
	}
}

// TestMergeIntoGraph_NilRegistryIsError proves the task #366 parameterization
// rejects a nil registry explicitly, rather than panicking on the first
// reg.All() call — a caller building a registry from an external source
// (e.g. `hotam sync-domain`'s subprocess-dumped registry) must get a clear
// error, not a crash, if that construction ever yields nil.
func TestMergeIntoGraph_NilRegistryIsError(t *testing.T) {
	g := &ontology.Graph{Requirements: []ontology.Requirement{{ID: "R-whatever"}}}
	if err := MergeIntoGraph(g, nil); err == nil {
		t.Fatal("MergeIntoGraph(g, nil): want error, got nil")
	}
}

// TestMergeIntoGraph_UnregisteredNodesUntouched proves a node whose ID is
// NOT in the registry is byte-for-byte unaffected by MergeIntoGraph — even
// though, as of Phase A (task #345), the registry covers all 301 real graph
// requirements, a caller can still construct a graph containing IDs the
// registry does not know about (e.g. a not-yet-landed proposal's synthetic
// node), and this test proves those stay untouched. It does NOT assert on
// the registered set's own before/after diff against the real graph: the
// whole point of this package is that the registry is a proven
// byte-identical mirror, so on a healthy registry that diff is legitimately
// zero — asserting it must be nonzero would make this test fail exactly when
// the codegen is doing its job correctly. Instead this test proves
// non-vacuity a different way: it builds a synthetic graph containing every
// registered ID PLUS one deliberately unregistered "control" node carrying a
// distinctive sentinel claim, runs MergeIntoGraph, and asserts the sentinel
// node's claim (and every other field) is untouched while at least one
// registered node's claim now matches the registry (not the synthetic
// placeholder claim it started with) — proving both halves of the contract:
// registered nodes ARE overwritten, unregistered nodes are NOT.
func TestMergeIntoGraph_UnregisteredNodesUntouched(t *testing.T) {
	all := Requirements.All()
	if len(all) == 0 {
		t.Fatal("selfspec.Requirements is empty — this test would be vacuous")
	}

	const sentinelClaim = "SENTINEL-CONTROL-CLAIM-must-survive-merge-untouched"
	const placeholderClaim = "SYNTHETIC-PLACEHOLDER-must-be-overwritten-by-registry"

	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-unregistered-control-node", Claim: sentinelClaim, Status: ontology.StatusDRAFT},
		},
	}
	for _, r := range all {
		// Every field starts as an obviously-wrong placeholder so a
		// passing test proves MergeIntoGraph actually overwrote it from
		// the registry, not that it happened to already match.
		g.Requirements = append(g.Requirements, ontology.Requirement{
			ID:     r.ID,
			Claim:  placeholderClaim,
			Status: ontology.StatusDRAFT,
		})
	}

	if err := MergeIntoGraph(g, Requirements); err != nil {
		t.Fatalf("MergeIntoGraph: %v", err)
	}

	byID := make(map[string]ontology.Requirement, len(g.Requirements))
	for _, r := range g.Requirements {
		byID[r.ID] = r
	}

	control := byID["R-unregistered-control-node"]
	if control.Claim != sentinelClaim {
		t.Errorf("unregistered control node was mutated by MergeIntoGraph: Claim = %q, want unchanged sentinel %q", control.Claim, sentinelClaim)
	}
	if control.Status != ontology.StatusDRAFT {
		t.Errorf("unregistered control node was mutated by MergeIntoGraph: Status = %q, want unchanged %q", control.Status, ontology.StatusDRAFT)
	}

	overwritten := 0
	for _, r := range all {
		got := byID[r.ID]
		if got.Claim == placeholderClaim {
			t.Errorf("registered requirement %q was NOT overwritten by MergeIntoGraph — still carries the synthetic placeholder claim", r.ID)
			continue
		}
		if got.Claim != r.Claim {
			t.Errorf("registered requirement %q: Claim = %q, want registry value %q", r.ID, got.Claim, r.Claim)
			continue
		}
		overwritten++
	}
	if overwritten != len(all) {
		t.Errorf("expected all %d registered requirements overwritten from the registry, got %d", len(all), overwritten)
	}
}

// writeGraphBytes writes g through loader.WriteGraph — REUSING the exact
// canonical-marshal path (internal/loader's SetEscapeHTML(false) + sorted-
// by-ID + 2-space-indent JSON encoder) that graph.json is always written
// through — into a fresh temp file, and returns the resulting bytes. This
// deliberately does NOT reimplement serialization; it calls the real,
// already-tested writer.
func writeGraphBytes(t *testing.T, g *ontology.Graph) []byte {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "graph.json")
	if err := loader.WriteGraph(path, g); err != nil {
		t.Fatalf("WriteGraph: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written graph: %v", err)
	}
	return data
}

// diffReport asserts got == want, and on failure prints byte counts plus the
// first differing line (with a few lines of surrounding context) so a real
// fidelity mismatch (a trailing-space difference, a nil-vs-[] slice, an
// escaped character) is immediately locatable instead of dumping the whole
// multi-hundred-KB file into the test log.
func diffReport(t *testing.T, name, got, want string) {
	t.Helper()
	if got == want {
		return
	}
	gotLines := strings.Split(got, "\n")
	wantLines := strings.Split(want, "\n")
	max := len(gotLines)
	if len(wantLines) < max {
		max = len(wantLines)
	}
	first := max
	for i := 0; i < max; i++ {
		if gotLines[i] != wantLines[i] {
			first = i
			break
		}
	}
	start := first - 3
	if start < 0 {
		start = 0
	}
	end := first + 5

	var b strings.Builder
	b.WriteString("\n=== byte-identity FAILED for " + name + " ===\n")
	b.WriteString("got bytes=" + strconv.Itoa(len(got)) + " want bytes=" + strconv.Itoa(len(want)) + "\n")
	b.WriteString("got lines=" + strconv.Itoa(len(gotLines)) + " want lines=" + strconv.Itoa(len(wantLines)) + "\n")
	b.WriteString("first differing line index: " + strconv.Itoa(first) + "\n")
	for i := start; i < end; i++ {
		gotLine := ""
		if i < len(gotLines) {
			gotLine = gotLines[i]
		}
		wantLine := ""
		if i < len(wantLines) {
			wantLine = wantLines[i]
		}
		marker := "  "
		switch {
		case i >= len(wantLines):
			marker = "G>"
		case i >= len(gotLines):
			marker = "W<"
		case gotLine != wantLine:
			marker = "* "
		}
		b.WriteString(marker + " line[" + strconv.Itoa(i) + "]\n    got:  " + truncate(gotLine, 200) + "\n    want: " + truncate(wantLine, 200) + "\n")
	}
	t.Error(b.String())
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
