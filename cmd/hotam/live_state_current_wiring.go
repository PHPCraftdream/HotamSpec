package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/PHPCraftdream/HotamSpec/internal/generator"
	"github.com/PHPCraftdream/HotamSpec/internal/invariants"
	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// This file wires the REAL check_live_state_md_current implementation into
// internal/invariants' registry, patching the honest-no-op placeholder
// internal/invariants/live_state_current.go registers
// (checkLiveStateMDCurrentUnwired) via registry.Update — the SAME mechanism
// agent_context_current_wiring.go already uses for
// check_agent_context_md_current. See live_state_current.go's own package
// doc comment for the full architectural rationale (internal/invariants must
// never import internal/generator — a real, mechanically-verified import
// cycle — so the real comparison logic, which needs
// generator.BuildLiveStateWithViolationsRoot AND this package's own
// graphPathForDomain/repoRootForDomain/domainNameFromDir crystal-location
// logic, can only live here, where both are already reachable).
//
// init() order: Go guarantees internal/invariants' own init() (which
// registers the placeholder via live_state_current.go's MustRegister call)
// runs before this package's init(), because cmd/hotam imports
// internal/invariants (dependency inits run first) — so
// invariants.All.Update below always finds an existing entry to patch.
func init() {
	invariants.All.Update("check_live_state_md_current", *withCheckLiveStateMDCurrent(mustGetInvariant("check_live_state_md_current")))
}

// withCheckLiveStateMDCurrent returns a copy of inv with its
// PostProcessCheck field patched to the real implementation
// (checkLiveStateMDCurrentReal), leaving every other field (Name, Canon,
// Claim, Rule, Why, ComparesOnDiskProjection) exactly as
// internal/invariants/live_state_current.go declared them.
func withCheckLiveStateMDCurrent(inv invariants.Invariant) *invariants.Invariant {
	inv.PostProcessCheck = func(g *ontology.Graph, prior []invariants.Violation) []invariants.Violation {
		return checkLiveStateMDCurrentReal(g, prior, time.Now().Format("2006-01-02"))
	}
	// AsOf variant: AllViolationsAsOf (land/sync-self/sync-domain's post-write
	// check) threads the command's own --today through — mirrors
	// withCheckAgentContextMDCurrent's identical rationale.
	inv.PostProcessCheckAsOf = checkLiveStateMDCurrentReal
	return &inv
}

// liveStateGeneratedRE matches the generation-date stamp
// internal/generator/livestate.go renders into every live-state.md
// (`- **generated:** YYYY-MM-DD`); the file self-carries the pin the
// stamped-date comparison below needs.
var liveStateGeneratedRE = regexp.MustCompile(`- \*\*generated:\*\* (\d{4}-\d{2}-\d{2})`)

// liveStateStampedDate returns the generation date a committed live-state.md
// carries.
func liveStateStampedDate(committed []byte) (string, bool) {
	if m := liveStateGeneratedRE.FindSubmatch(committed); m != nil {
		return string(m[1]), true
	}
	return "", false
}

// checkLiveStateMDCurrentReal is the real check_live_state_md_current logic:
// IF a domain's committed docs/gen/live-state.md exists AND the standalone
// projection is not withheld for this graph, its WHOLE content must be
// byte-identical to generator.BuildLiveStateWithViolationsRoot's output over
// the current graph, fed the publication-flavored violation set
// (invariants.PublicationViolationsFromPhaseOne of priorViolations) — the
// exact snapshot genSpec threads into the written file (mirrors
// checkAgentContextMDCurrentReal; the P3-13 change established that flavor as
// THE single selector for comparative renders).
//
// HONEST NO-OPs (mirrors checkAgentContextMDCurrentReal's own
// opt-in-when-absent shape):
//   - g.DomainDir == "": an in-memory fixture graph never loaded via
//     loader.LoadGraph — no on-disk domain to check against at all.
//   - explicit language configurations (g.Languages non-empty): genSpec
//     never writes the standalone live-state.md for those domains
//     (gen_spec.go's !localizedConfigured gate) and localized live-state
//     views are covered by check_language_outputs_current; this check judges
//     only the non-localized root projection.
//   - generator.LiveStateMDHasContent(g) is false: the domain graph is
//     genuinely empty (task #364's conditional-write gate) so genSpec never
//     writes the file — a domain that never gets the projection must be an
//     honest no-op, not a violation. (A stale live-state.md left over from
//     before the graph was emptied is out of scope here: the same run's
//     structure checks judge the docs/gen tree's other projections.)
//   - the file does not exist on disk: nothing to be stale.
func checkLiveStateMDCurrentReal(g *ontology.Graph, priorViolations []invariants.Violation, today string) []invariants.Violation {
	if g.DomainDir == "" {
		return nil
	}

	if len(g.Languages) > 0 {
		return nil
	}

	if !generator.LiveStateMDHasContent(g) {
		return nil
	}

	liveStatePath := filepath.Join(g.DomainDir, "docs", "gen", "live-state.md")
	committed, err := os.ReadFile(liveStatePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return []invariants.Violation{{
			Check:   "check_live_state_md_current",
			ID:      g.DomainDir,
			Message: fmt.Sprintf("could not read %s: %v", liveStatePath, err),
		}}
	}

	// The render depends on `today` (freshness signals: OVERDUE counts vs
	// review_after, the `--today` advisory text), so judging as of the
	// calendar day would flag a content-identical file every midnight. The
	// file stamps its own generation date (`- **generated:** <date>`, added
	// by this same review finding); compare as of the date the file carries.
	if date, ok := liveStateStampedDate(committed); ok {
		today = date
	}

	publication := invariants.PublicationViolationsFromPhaseOne(priorViolations)

	domainName := domainNameFromDir(g.DomainDir)
	repoRoot := repoRootForDomain(g.DomainDir)
	domainGraphs := map[string]*ontology.Graph{domainName: g}
	consumer := loader.ResolveGenProfile(graphPathForDomain(g.DomainDir)) == loader.GenProfileConsumer
	claudeMDPath := resolveClaudeMDPath(g.DomainDir, "")

	// Same charCount fixpoint genSpec embeds (mirrors
	// checkAgentContextMDCurrentReal's identical call shape), so the
	// budget-line number matches what genSpec wrote into the committed file.
	override := &generator.ViolationsOverride{For: g, Violations: publication}
	charCount, err := generator.ComputeCrystalCharCountFixpointWithViolations(g, domainName, repoRoot, domainGraphs, today, consumer, override, claudeMDPath)
	if err != nil {
		return []invariants.Violation{{
			Check:   "check_live_state_md_current",
			ID:      g.DomainDir,
			Message: fmt.Sprintf("could not compute a fresh render of %s to compare against: %v", liveStatePath, err),
		}}
	}
	fresh := generator.BuildStandaloneLiveStateRoot(g, domainName, charCount, today, publication, repoRoot)

	if string(committed) != fresh {
		return []invariants.Violation{{
			Check: "check_live_state_md_current",
			ID:    g.DomainDir,
			Message: fmt.Sprintf(
				"%s does not match what a fresh `hotam gen-spec` run produces right now -- it is either stale (the "+
					"domain's graph, requirements, or debt/pulse state changed since it was last regenerated) or was "+
					"edited by hand despite its own do-not-edit banner; the whole file is generated content. Re-run "+
					"`hotam gen-spec --domain %s --claude-md %s` to regenerate it.",
				liveStatePath, g.DomainDir, claudeMDPath),
		}}
	}
	return nil
}
