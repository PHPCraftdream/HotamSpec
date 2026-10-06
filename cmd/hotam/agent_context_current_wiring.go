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

// This file wires the REAL check_agent_context_md_current implementation into
// internal/invariants' registry, patching the honest-no-op placeholder
// internal/invariants/agent_context_current.go registers
// (checkAgentContextMDCurrentUnwired) via registry.Update — the SAME mechanism
// claude_md_current_wiring.go already uses for
// check_domain_claude_md_current. See agent_context_current.go's own package
// doc comment for the full architectural rationale (internal/invariants must
// never import internal/generator — a real, mechanically-verified import
// cycle — so the real comparison logic, which needs
// generator.BuildAgentContextRootWithViolations AND this package's own
// resolveClaudeMDPath/repoRootForDomain/domainNameFromDir crystal-location
// logic, can only live here, where both are already reachable).
//
// init() order: Go guarantees internal/invariants' own init() (which
// registers the placeholder via agent_context_current.go's MustRegister call)
// runs before this package's init(), because cmd/hotam imports
// internal/invariants (dependency inits run first) — so
// invariants.All.Update below always finds an existing entry to patch.
func init() {
	invariants.All.Update("check_agent_context_md_current", *withCheckAgentContextMDCurrent(mustGetInvariant("check_agent_context_md_current")))
}

// withCheckAgentContextMDCurrent returns a copy of inv with its
// PostProcessCheck field patched to the real implementation
// (checkAgentContextMDCurrentReal), leaving every other field (Name, Canon,
// Claim, Rule, Why, ComparesOnDiskProjection) exactly as
// internal/invariants/agent_context_current.go declared them.
func withCheckAgentContextMDCurrent(inv invariants.Invariant) *invariants.Invariant {
	inv.PostProcessCheck = func(g *ontology.Graph, prior []invariants.Violation) []invariants.Violation {
		return checkAgentContextMDCurrentReal(g, prior, time.Now().Format("2006-01-02"))
	}
	// AsOf variant: AllViolationsAsOf (land/sync-self/sync-domain's post-write
	// check) threads the command's own --today through, so the fresh render is
	// computed AS OF the same date genSpec just wrote the file with — mirrors
	// withCheckDomainClaudeMDCurrent's identical rationale.
	inv.PostProcessCheckAsOf = checkAgentContextMDCurrentReal
	return &inv
}

var (
	agentContextAsOfRE  = regexp.MustCompile(`\(as of (\d{4}-\d{2}-\d{2})\)`)
	agentContextTodayRE = regexp.MustCompile(`--today (\d{4}-\d{2}-\d{2})`)
)

// agentContextStampedDate returns the generation date a committed AGENT-CONTEXT.md carries.
func agentContextStampedDate(committed []byte) (string, bool) {
	for _, re := range []*regexp.Regexp{agentContextAsOfRE, agentContextTodayRE} {
		if m := re.FindSubmatch(committed); m != nil {
			return string(m[1]), true
		}
	}
	return "", false
}

// checkAgentContextMDCurrentReal is the real check_agent_context_md_current
// logic: IF a domain's committed docs/gen/AGENT-CONTEXT.md exists, its WHOLE
// content (the projection carries no durable-notes tail, so there is nothing
// to split around — unlike CLAUDE.md, no SplitAtDurableNotesMarker) must be
// byte-identical to generator.BuildAgentContextRootWithViolations's output
// over the current graph, fed the publication-flavored violation set
// (invariants.PublicationViolationsFromPhaseOne of priorViolations) — the
// exact snapshot genSpec threads into the written file. That flavor, not
// priorViolations directly, is what makes the comparison honest: a full-set
// render would disagree with the committed file whenever an unrelated disk
// projection is transiently stale, even though a re-run would rewrite
// AGENT-CONTEXT.md byte-identically.
//
// HONEST NO-OPs (mirrors checkDomainClaudeMDCurrentReal's own
// opt-in-when-absent shape):
//   - g.DomainDir == "": an in-memory fixture graph never loaded via
//     loader.LoadGraph — no on-disk domain to check against at all.
//   - explicit language configurations (g.Languages non-empty): localized
//     AGENT-CONTEXT views are covered by check_language_outputs_current; this
//     check judges only the non-localized root projection.
//   - the file does not exist on disk: this domain has never had
//     AGENT-CONTEXT.md generated yet (consumer-profile domains never get it
//     at all) — nothing to be stale YET.
func checkAgentContextMDCurrentReal(g *ontology.Graph, priorViolations []invariants.Violation, today string) []invariants.Violation {
	if g.DomainDir == "" {
		return nil
	}

	// Explicit language configurations compare localized AGENT-CONTEXT views
	// through check_language_outputs_current; this root renderer has no
	// locale context and must not judge those files.
	if len(g.Languages) > 0 {
		return nil
	}
	agentContextPath := filepath.Join(g.DomainDir, "docs", "gen", "AGENT-CONTEXT.md")
	committed, err := os.ReadFile(agentContextPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return []invariants.Violation{{
			Check:   "check_agent_context_md_current",
			ID:      g.DomainDir,
			Message: fmt.Sprintf("could not read %s: %v", agentContextPath, err),
		}}
	}

	// The file stamps its own generation date (the unconditional counters line
	// "... (as of <date>)", and the freshness advisory's `--today <date>`), so
	// judging it as of the calendar day would flag a content-identical file
	// every midnight. Compare as of the date the file itself carries.
	if date, ok := agentContextStampedDate(committed); ok {
		today = date
	}

	publication := invariants.PublicationViolationsFromPhaseOne(priorViolations)

	domainName := domainNameFromDir(g.DomainDir)
	repoRoot := repoRootForDomain(g.DomainDir)
	domainGraphs := map[string]*ontology.Graph{domainName: g}
	consumer := loader.ResolveGenProfile(graphPathForDomain(g.DomainDir)) == loader.GenProfileConsumer
	claudeMDPath := resolveClaudeMDPath(g.DomainDir, "")

	// Same charCount fixpoint genSpec embeds (mirrors
	// checkDomainClaudeMDCurrentReal's identical call shape), so the
	// budget-line number matches what genSpec wrote into the committed file.
	override := &generator.ViolationsOverride{For: g, Violations: publication}
	charCount, err := generator.ComputeCrystalCharCountFixpointWithViolations(g, domainName, repoRoot, domainGraphs, today, consumer, override, claudeMDPath)
	if err != nil {
		return []invariants.Violation{{
			Check:   "check_agent_context_md_current",
			ID:      g.DomainDir,
			Message: fmt.Sprintf("could not compute a fresh render of %s to compare against: %v", agentContextPath, err),
		}}
	}
	fresh := generator.BuildAgentContextRootWithViolations(g, domainName, charCount, today, consumer, publication, repoRoot)

	if string(committed) != fresh {
		return []invariants.Violation{{
			Check: "check_agent_context_md_current",
			ID:    g.DomainDir,
			Message: fmt.Sprintf(
				"%s does not match what a fresh `hotam gen-spec` run produces right now -- it is either stale (the "+
					"domain's graph, requirements, or debt/pulse state changed since it was last regenerated) or was "+
					"edited by hand despite its own do-not-edit banner; the whole file is generated content. Re-run "+
					"`hotam gen-spec --domain %s --claude-md %s` to regenerate it.",
				agentContextPath, g.DomainDir, claudeMDPath),
		}}
	}
	return nil
}
