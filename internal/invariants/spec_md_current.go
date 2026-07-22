// spec_md_current.go holds check_spec_md_current
// (PLAN-scenario-generated-spec.md §3 W2.3): the mechanical staleness gate
// for docs/gen/SPEC.md, closing the lifecycle gap W2.3 also fixed on the
// write side (cmd/hotam/gen_spec.go's genSpec: a default `hotam gen-spec`
// run, without --spec, no longer DELETES a committed SPEC.md as an orphan —
// see genSpec's own exemptFromCleanup doc comment — but it also does not
// regenerate it, since BuildSpec/CollectSpecRows execute a real `go test`
// per verified_by entry and that cost is deliberately opt-in). This check is
// the read-side complement: once a domain HAS a committed SPEC.md, it must
// actually match what a fresh `gen-spec --spec` run would produce right now
// — an edited-by-hand or simply out-of-date SPEC.md is exactly as dishonest
// as a stale REQUIREMENTS.md/TRACEABILITY.md would be, just for a document
// whose content happens to be expensive to regenerate rather than free.
package invariants

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// specMDRelPath is where genSpec (cmd/hotam/gen_spec.go) writes SPEC.md,
// relative to a domain's own directory — kept as a named constant here (the
// same discipline recorder_check.go's vendoredRecorderRelPath already
// establishes) so the write side and the check side visibly agree on one
// literal path.
const specMDRelPath = "docs/gen/SPEC.md"

// checkSpecMDCurrent is the mechanical actuality gate for docs/gen/SPEC.md.
//
// COMPARISON METHOD — full re-render, not a hash/marker embedded in the
// file: unlike checkRecorderCurrent (a sha256 compare against a fixed,
// cheap-to-recompute canonical source), there is no cheaper honest substitute
// here — SPEC.md's own content IS the record of a real `go test` execution
// (PLAN-scenario-generated-spec.md §1: "text incarnates the actually-run
// test"), so the only way to know whether it is CURRENT is to actually run
// the verified_by tests again and compare the freshly rendered Markdown,
// byte for byte, against what is committed. This check therefore calls
// gate.CollectSpecRows (the SAME data-collection pass BuildSpecFromRows
// itself performs — see internal/gate/spec_build.go's LAYERING doc comment
// for why this lives in internal/gate rather than internal/generator, which
// internal/invariants cannot import without closing a real import cycle)
// followed by gate.BuildSpecFromRows, and does a plain string comparison
// against the committed file's bytes.
//
// HONEST NO-OP — no committed SPEC.md at all: mirrors checkRecorderCurrent's
// own opt-in shape (that check's doc comment: "a domain that has not yet
// adopted scenarios is not lying about anything by not having a vendored
// copy on disk") and every other authored-projection check's honest-no-op
// convention. A domain that has never run `gen-spec --spec` (every domain in
// this wave except the prat pilot per PLAN-scenario-generated-spec.md's own
// wave ordering — gpsm-sm and hotam-spec-self have not adopted the
// scenario-generated-spec layer yet) has no SPEC.md file to be stale, so
// this check contributes zero violations for it, regardless of how bare its
// verified_by/implemented_by fields are — exactly the same non-mandatory
// posture check_settled_requires_scenario's own discipline:full gate already
// establishes for the wider scenario layer.
//
// COST — real, paid once per AllViolations call that reaches this check (one
// gate.CollectSpecRows pass, i.e. one real `go test` subprocess per
// verified_by entry with a resolvable scenario — the SAME cost
// check_scenario_executes_impl and check_verified_by_test_passes already
// accept as the price of proving execution rather than merely inferring it
// from source shape). Deliberately NOT cached to disk across process runs
// (b014a63: the cross-process disk verdict cache was removed as the root of
// a forge vector — a world-writable, offline-computable cache file let an
// attacker fake a green verdict) — the in-process memoization
// gate.RunVerifiedByTestRecording's own callers already rely on (and the
// recursion guard below) is the only caching this check inherits.
//
// RECURSION GUARD — inherited unchanged from gate.RunVerifiedByTestRecording
// itself (the same env-nonce mechanism check_scenario_executes_impl and
// check_verified_by_test_passes already depend on): when this check's own
// gate.CollectSpecRows call is itself running INSIDE a go-test subprocess
// gate.RunVerifiedByTestRecording spawned (e.g. `go test ./internal/...`
// reaching TestRegistryComplete_AllViolationsOnRealGraphDoesNotPanic, which
// calls AllViolations, which reaches this check, which would otherwise spawn
// yet another nested `go test`), every RunVerifiedByTestRecording call this
// check triggers reports Skipped instead of recursing — this check treats a
// Skipped-dominated CollectSpecRows result the same way BuildSpecFromRows
// itself would (an honest "not executed at this nesting level" outcome
// baked into the rendered text alongside everything else), so no special
// double-run handling is needed here: the SAME two-sided honesty
// (Skipped is not silently promoted to a "current" verdict, but also not a
// hard failure at the nested level) that RunVerifiedByTestRecording's own
// contract already provides propagates through CollectSpecRows/
// BuildSpecFromRows unchanged, and the OUTER, non-nested `go test
// ./internal/invariants/...` process is the one that actually proves
// currency.
func checkSpecMDCurrent(g *ontology.Graph) []Violation {
	if g.DomainDir == "" {
		// No on-disk domain to check against (an in-memory fixture graph
		// built without ever going through loader.LoadGraph) -- honest
		// no-op, mirroring checkRecorderCurrent's identical guard.
		return nil
	}
	specPath := filepath.Join(g.DomainDir, filepath.FromSlash(specMDRelPath))
	committed, err := os.ReadFile(specPath)
	if err != nil {
		if os.IsNotExist(err) {
			// F3 (task W7.2, @fx finding F3): a discipline:full domain that
			// has no docs/gen/SPEC.md at all is NOT an honest no-op -- a
			// discipline:full domain has made the explicit, one-way promise
			// (PLAN-scenario-generated-spec.md §2 D4) that its normative text
			// is generated from real scenario test runs; a missing SPEC.md
			// means that promise's artifact is entirely absent, which is a
			// violation, not an "hasn't adopted yet" state. Before F3, this
			// branch was an unconditional honest no-op regardless of
			// discipline -- a discipline:full domain with SPEC.md deleted
			// (accidentally or maliciously) kept claiming the discipline
			// while its entire generated normative text was gone, undetected
			// by all-violations. A domain WITHOUT discipline:full keeps the
			// existing honest no-op (it never promised a SPEC.md).
			if g.Discipline == loader.DisciplineFull {
				return []Violation{{
					Check: "check_spec_md_current",
					ID:    g.DomainDir,
					Message: fmt.Sprintf(
						"docs/gen/SPEC.md does not exist for %s, but this domain has declared discipline:\"full\" -- "+
							"a discipline:full domain's normative text must be generated from real scenario test runs; "+
							"run `hotam gen-spec --domain %s --spec` to generate it",
						g.DomainDir, g.DomainDir),
				}}
			}
			// This domain has never adopted the scenario-generated-spec
			// layer (no `gen-spec --spec` run yet) and is not discipline:full
			// -- honest no-op, see doc comment above.
			return nil
		}
		return []Violation{{
			Check:   "check_spec_md_current",
			ID:      g.DomainDir,
			Message: fmt.Sprintf("could not read %s: %v", specPath, err),
		}}
	}

	fresh := gate.BuildSpecFromRows(g, gate.CollectSpecRows(g))
	if string(committed) != fresh {
		return []Violation{{
			Check: "check_spec_md_current",
			ID:    g.DomainDir,
			Message: fmt.Sprintf(
				"%s does not match what a fresh `hotam gen-spec --spec` run produces right now -- it is either stale "+
					"(the domain's graph, implemented_by/verified_by links, or the code they point at changed since SPEC.md "+
					"was last regenerated) or was edited by hand despite its own do-not-edit banner; re-run "+
					"`hotam gen-spec --domain %s --spec` to regenerate it from the current, real, passing `go test` output",
				specPath, g.DomainDir),
		}}
	}
	return nil
}

var _ = All.MustRegister("check_spec_md_current", Invariant{
	Name:                     "check_spec_md_current",
	ComparesOnDiskProjection: true,
	Canon:                    methodology.Domain,
	Claim: "a domain's committed docs/gen/SPEC.md, if present, is byte-identical to what a fresh `hotam gen-spec --spec` run " +
		"(gate.CollectSpecRows + gate.BuildSpecFromRows, a real `go test` execution of every verified_by entry) produces " +
		"right now; a domain with no SPEC.md yet is an honest no-op.",
	Rule: "IF a file exists at <domainDir>/docs/gen/SPEC.md, its bytes MUST equal gate.BuildSpecFromRows(g, " +
		"gate.CollectSpecRows(g)) computed against the CURRENT graph and the CURRENT state of the domain's authored spec/ " +
		"tree (a real, freshly-executed `go test` run per verified_by entry -- not a cached or assumed prior result). A " +
		"domain with NO file at that path is an honest NO-OP -- adopting the scenario-generated-spec layer (via " +
		"`hotam gen-spec --spec`) is opt-in, not mandatory for every domain unconditionally, mirroring " +
		"check_recorder_current's identical opt-in shape for the vendored recorder.",
	Why: "PLAN-scenario-generated-spec.md §3 W2.3 fixes a lifecycle bug this task's own predecessor left open: a default " +
		"`hotam gen-spec` (no --spec) used to treat a committed SPEC.md as an ORPHAN and DELETE it, because SPEC.md was only " +
		"ever written under --spec and the default run's stale-file cleanup pass did not know SPEC.md was a first-class, " +
		"KNOWN projection rather than leftover cruft (cmd/hotam/gen_spec.go's genSpec now exempts SPEC.md from that cleanup " +
		"via exemptFromCleanup -- see its own doc comment). That write-side fix alone would leave a real gap open: a " +
		"domain could land a graph/code change that invalidates SPEC.md's narrative (an implementation edited, a " +
		"verified_by test's assertions changed, a requirement's claim reworded) and `hotam all-violations` would report " +
		"nothing wrong, because no other check ever reads SPEC.md's own committed content and compares it against reality. " +
		"This check closes that gap the same way check_recorder_current closes the analogous gap for the vendored " +
		"recorder -- read the committed artifact, recompute what it SHOULD be right now, and fire if they diverge. " +
		"COMPARISON METHOD is necessarily a full re-render (not a cheap hash-of-source check like " +
		"check_recorder_current's) because SPEC.md's content is not a pure function of source text alone -- it is the " +
		"recorded narrative of an ACTUALLY EXECUTED test run (PLAN §1's own framing), so proving it is current requires " +
		"actually re-running that same execution, exactly the same cost check_scenario_executes_impl and " +
		"check_verified_by_test_passes already accept elsewhere in this package. RELATION TO check_recorder_current: both " +
		"are domain-wide, filesystem-aware, honest-no-op-when-absent invariants (methodology.Domain canon) that read " +
		"g.DomainDir directly rather than the in-memory graph alone; check_recorder_current guards the Go CODE a domain " +
		"compiles and runs (spec/hotamspec/hotamspec.go), while this check guards the derived DOCUMENT that code's test " +
		"runs produce (docs/gen/SPEC.md) -- a stale recorder could silently change what a scenario records, while a stale " +
		"SPEC.md silently shows a reader (human or agent) normative text that no longer matches what the domain's tests " +
		"actually prove today.",
	Check: checkSpecMDCurrent,
})
