package main

import (
	"fmt"
	"os"

	"github.com/PHPCraftdream/HotamSpec/internal/diagnose"
	"github.com/PHPCraftdream/HotamSpec/internal/invariants"
)

func cmdAllViolations(args []string) error {
	// Compile-cache cleanup (gate.CleanupCompileCache) is centralized in
	// main() -- reachable from every subcommand, not just this one. See
	// main()'s own doc comment for why.
	fs := newFlagSet("all-violations")
	domain := fs.String("domain", "", "domain directory (default: "+defaultDomainRel+")")
	asJSON := fs.Bool("json", false, "emit machine-readable JSON")
	fs.Parse(args)

	domainDir, err := resolveDomain(*domain)
	if err != nil {
		return err
	}
	violations, err := allViolations(domainDir)
	if err != nil {
		return err
	}
	if *asJSON {
		// --json stays exactly []invariants.Violation for backward
		// compatibility (TestCmdAllViolations_JSON): advisory signals
		// (never blocking, never part of this exit-code contract) are a
		// plain-text-only addition below, not folded into this shape.
		if violations == nil {
			violations = []invariants.Violation{}
		}
		if err := printJSON(violations); err != nil {
			return err
		}
	} else {
		for _, v := range violations {
			fmt.Printf("[%s] %s: %s\n", v.Check, v.ID, v.Message)
		}
	}
	// Exit code is identical with or without --json: 1 if any violations,
	// 0 if clean. --json changes the OUTPUT FORMAT, never the exit-code
	// contract. Advisory signals (below) NEVER affect this exit code —
	// they are informational, not a gate (R-ai-presents-not-decides'
	// sibling: advisory is presented, never enforced). The advisory section
	// is printed on BOTH the clean and the violations-found path (not only
	// when clean): an honored verified_by recursion-guard skip must stay
	// visible even in a run that ALSO reports unrelated real violations
	// elsewhere in the graph — @fh's "honored-skip must not be silent"
	// re-review does not carve out an exception for "the run was already
	// going to fail anyway".
	if len(violations) > 0 {
		if !*asJSON {
			fmt.Fprintf(os.Stderr, "%d violation(s) found\n", len(violations))
			if err := printAdvisorySection(domainDir); err != nil {
				return err
			}
		}
		os.Exit(1)
	}
	if !*asJSON {
		fmt.Println("0 violations — graph clean")
		if err := printAdvisorySection(domainDir); err != nil {
			return err
		}
	}
	return nil
}

// printAdvisorySection prints a non-blocking "ADVISORY" section: signals like
// orphan-detail (diagnose.ReflectOrphanEntityType), honored verified_by
// recursion-guard skips (invariants.HonoredSkipWarnings -- @fh's
// "honored-skip must not be silent" re-review: a Skipped RunVerifiedByTest
// result must never look identical to a genuinely proven entry), and a
// Process/Step why containing a point-in-time status snapshot
// (invariants.ProcessWhySnapshotWarnings, task #331/R4-process-why -- a
// point-in-time claim like "27 of 32 SIGNED as of 2026-07-21" belongs to
// PIPELINE.md's generated Live state section, never frozen into authored
// prose nothing regenerates) that are informational for the resolver, never
// a gate. Called on BOTH the clean and the violations-found path in
// cmdAllViolations (see its own comment) so a warning is never hidden behind
// an unrelated blocking violation. It never affects the exit code and never
// appears in --json output (see cmdAllViolations' --json branch above).
// No-ops silently when there is nothing advisory to report — an empty
// advisory section would be noise, not signal.
func printAdvisorySection(domainDir string) error {
	g, err := loadDomainGraph(domainDir)
	if err != nil {
		return err
	}
	orphans := diagnose.ReflectOrphanEntityType(g)
	skips := invariants.HonoredSkipWarnings(g)
	snapshots := invariants.ProcessWhySnapshotWarnings(g)
	if len(orphans) == 0 && len(skips) == 0 && len(snapshots) == 0 {
		return nil
	}
	fmt.Println()
	fmt.Println("ADVISORY (non-blocking):")
	for _, f := range orphans {
		fmt.Printf("[%s] %s: %s\n", f.Condition, f.Target, f.Imperative)
	}
	for _, v := range skips {
		fmt.Printf("[%s] %s: %s\n", v.Check, v.ID, v.Message)
	}
	for _, v := range snapshots {
		fmt.Printf("[%s] %s: %s\n", v.Check, v.ID, v.Message)
	}
	return nil
}

func allViolations(domainDir string) ([]invariants.Violation, error) {
	g, err := loadDomainGraph(domainDir)
	if err != nil {
		return nil, err
	}
	return invariants.AllViolations(g), nil
}
