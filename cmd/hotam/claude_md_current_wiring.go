package main

import (
	"fmt"
	"os"
	"time"

	"github.com/PHPCraftdream/HotamSpec/internal/generator"
	"github.com/PHPCraftdream/HotamSpec/internal/invariants"
	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// This file wires the REAL check_domain_claude_md_current implementation
// into internal/invariants' registry, patching the honest-no-op placeholder
// internal/invariants/claude_md_current.go registers
// (checkDomainClaudeMDCurrentUnwired) via registry.Update — the SAME
// mechanism cmd/hotam/tool_wiring.go already uses to patch methodology.Tools'
// Run field from a nil placeholder. See claude_md_current.go's own package
// doc comment for the full architectural rationale (internal/invariants must
// never import internal/generator — a real, mechanically-verified import
// cycle — so the real comparison logic, which needs
// generator.RenderClaudeMDFromTemplateWithViolations AND this package's own
// resolveClaudeMDPath/repoRootForDomain/domainNameFromDir crystal-location
// logic, can only live here, where both are already reachable).
//
// init() order: Go guarantees internal/invariants' own init() (which
// registers the placeholder via claude_md_current.go's MustRegister call)
// runs before this package's init(), because cmd/hotam imports
// internal/invariants (dependency inits run first) — so
// invariants.All.Update below always finds an existing entry to patch.
func init() {
	invariants.All.Update("check_domain_claude_md_current", *withCheckDomainClaudeMDCurrent(mustGetInvariant("check_domain_claude_md_current")))
}

// mustGetInvariant fetches the already-registered placeholder Invariant by
// name, panicking (a wiring bug, not a runtime condition) if it is missing —
// mirrors tool_wiring.go's wireToolRun's identical "no such registered X"
// panic shape.
func mustGetInvariant(name string) invariants.Invariant {
	inv, ok := invariants.All.Get(name)
	if !ok {
		panic("claude_md_current_wiring: no such registered invariant " + name)
	}
	return *inv
}

// withCheckDomainClaudeMDCurrent returns a copy of inv with its
// PostProcessCheck field patched to the real implementation
// (checkDomainClaudeMDCurrentReal), leaving every other field (Name, Canon,
// Claim, Rule, Why, ComparesOnDiskProjection) exactly as
// internal/invariants/claude_md_current.go declared them.
func withCheckDomainClaudeMDCurrent(inv invariants.Invariant) *invariants.Invariant {
	inv.PostProcessCheck = checkDomainClaudeMDCurrentReal
	return &inv
}

// checkDomainClaudeMDCurrentReal is the real check_domain_claude_md_current
// logic: IF a domain's resolved crystal path (the SAME resolution
// `hotam land`/`hotam gen-spec` already use — resolveClaudeMDPath with no
// explicit override) carries a committed CLAUDE.md, its GENERATED portion
// (everything up to and including generator.DurableNotesMarkerLine) must be
// byte-identical to a fresh render computed via
// generator.RenderClaudeMDFromTemplateWithViolations, fed priorViolations
// (this AllViolations pass's own phase-1 result — see
// internal/invariants/invariant.go's PostProcessCheck doc comment for why
// this avoids recursing back into AllViolations for the same graph). Content
// after the marker line (the operator's own durable notes) is never read for
// comparison — see splitGeneratedPart's own doc comment.
//
// HONEST NO-OPs (mirrors check_spec_md_current's own opt-in-when-absent
// shape, internal/invariants/spec_md_current.go):
//   - g.DomainDir == "": an in-memory fixture graph never loaded via
//     loader.LoadGraph — no on-disk domain to check against at all.
//   - resolveClaudeMDPath resolves to "": the project has not adopted the
//     crystal convention (no root CLAUDE.md, no .hotam-spec-project marker) —
//     see resolveClaudeMDPath/crystalConventionExists's own doc comments.
//   - the resolved path does not exist on disk: this domain's project HAS
//     adopted the convention in general, but this specific domain has never
//     had its crystal generated yet (a fresh `hotam init-project` before the
//     first `hotam gen-spec --claude-md` run, for example) — nothing to be
//     stale YET, distinct from check_spec_md_current's F3 special case (a
//     discipline:full domain's SPEC.md absence IS a violation because
//     discipline:full is an explicit one-way promise); CLAUDE.md carries no
//     equivalent opt-in flag a domain could have declared and then violated
//     by omission, so absence here stays a plain no-op regardless of
//     discipline.
//   - the committed file has no generator.DurableNotesMarkerLine at all: not
//     a template-shaped file (e.g. a hand-authored README some project
//     already had at that path before ever adopting `hotam gen-spec
//     --claude-md`) — comparing it against a generated template would be
//     comparing two unrelated things, not detecting staleness.
func checkDomainClaudeMDCurrentReal(g *ontology.Graph, priorViolations []invariants.Violation) []invariants.Violation {
	if g.DomainDir == "" {
		return nil
	}
	claudeMDPath := resolveClaudeMDPath(g.DomainDir, "")
	if claudeMDPath == "" {
		return nil
	}
	committed, err := os.ReadFile(claudeMDPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return []invariants.Violation{{
			Check:   "check_domain_claude_md_current",
			ID:      g.DomainDir,
			Message: fmt.Sprintf("could not read %s: %v", claudeMDPath, err),
		}}
	}

	committedGenerated, _, ok := generator.SplitAtDurableNotesMarker(string(committed))
	if !ok {
		// Not a template-shaped file -- honest no-op, see doc comment above.
		return nil
	}

	domainName := domainNameFromDir(g.DomainDir)
	repoRoot := repoRootForDomain(g.DomainDir)
	domainGraphs := map[string]*ontology.Graph{domainName: g}
	consumer := loader.ResolveGenProfile(graphPathForDomain(g.DomainDir)) == loader.GenProfileConsumer
	today := time.Now().Format("2006-01-02")

	// selfCrystalPath == claudeMDPath: this check just confirmed a real file
	// exists at claudeMDPath (the os.ReadFile above succeeded), so the
	// SELF-ENTRY SPECIAL CASE's tautology genuinely holds here too — see
	// renderDomainMapBlockWithViolations's doc comment. Without this, the
	// fresh render's DOMAIN-MAP self-entry would fall through to a real
	// os.Stat on claudeMDPath, which — since the committed file DOES exist
	// on disk right now — would actually agree with the committed content in
	// THIS specific call ordering; the mismatch this fixes is about
	// determinism across DIFFERENT render call sites (this check's render
	// vs genSpec's own write-time render, which runs its os.Stat BEFORE the
	// file exists), not about this one call site in isolation — passing it
	// here keeps every RenderClaudeMDFromTemplateWithViolations call site in
	// this codebase using the same deterministic rule.
	override := &generator.ViolationsOverride{For: g, Violations: priorViolations}
	charCount, err := generator.ComputeCrystalCharCountFixpointWithViolations(g, domainName, repoRoot, domainGraphs, today, consumer, override, claudeMDPath)
	if err != nil {
		return []invariants.Violation{{
			Check:   "check_domain_claude_md_current",
			ID:      g.DomainDir,
			Message: fmt.Sprintf("could not compute a fresh render of %s to compare against: %v", claudeMDPath, err),
		}}
	}
	fresh := generator.RenderClaudeMDFromTemplateWithViolations(g, domainName, repoRoot, charCount, domainGraphs, today, consumer, override, claudeMDPath)
	freshGenerated, _, freshOK := generator.SplitAtDurableNotesMarker(fresh)
	if !freshOK {
		// Should be unreachable -- every template this package renders
		// carries generator.DurableNotesMarkerLine by construction. Surfaced
		// as a violation (not a panic) so a future template change that
		// somehow drops the marker is caught as data, not a crash.
		return []invariants.Violation{{
			Check:   "check_domain_claude_md_current",
			ID:      g.DomainDir,
			Message: "a fresh render of this domain's CLAUDE.md carries no durable-notes marker line at all -- this is an engine bug, not a domain problem; report it",
		}}
	}

	if committedGenerated != freshGenerated {
		return []invariants.Violation{{
			Check: "check_domain_claude_md_current",
			ID:    g.DomainDir,
			Message: fmt.Sprintf(
				"%s's generated portion (everything up to and including the durable-notes marker line) does not match "+
					"what a fresh `hotam gen-spec --claude-md` run produces right now -- it is either stale (the domain's "+
					"graph, requirements, or debt/pulse state changed since the crystal was last regenerated) or was "+
					"edited by hand despite its own do-not-edit banner; content BELOW the marker line (durable notes) is "+
					"never compared and is safe. Re-run `hotam gen-spec --domain %s --claude-md %s` to regenerate it.",
				claudeMDPath, g.DomainDir, claudeMDPath),
		}}
	}
	return nil
}
