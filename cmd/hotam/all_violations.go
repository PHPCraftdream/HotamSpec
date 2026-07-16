package main

import (
	"fmt"
	"os"

	"github.com/PHPCraftdream/HotamSpec/internal/diagnose"
	"github.com/PHPCraftdream/HotamSpec/internal/invariants"
)

func cmdAllViolations(args []string) error {
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
	// sibling: advisory is presented, never enforced).
	if len(violations) > 0 {
		if !*asJSON {
			fmt.Fprintf(os.Stderr, "%d violation(s) found\n", len(violations))
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

// printAdvisorySection prints a non-blocking "ADVISORY" section after a clean
// (0 blocking violations) all-violations run: signals like orphan-detail
// (diagnose.ReflectOrphanEntityType) that are informational for the steward,
// never a gate. It never affects the exit code and never appears in --json
// output (see cmdAllViolations' --json branch above). No-ops silently when
// there is nothing advisory to report — an empty advisory section would be
// noise, not signal.
func printAdvisorySection(domainDir string) error {
	g, err := loadDomainGraph(domainDir)
	if err != nil {
		return err
	}
	orphans := diagnose.ReflectOrphanEntityType(g)
	if len(orphans) == 0 {
		return nil
	}
	fmt.Println()
	fmt.Println("ADVISORY (non-blocking):")
	for _, f := range orphans {
		fmt.Printf("[%s] %s: %s\n", f.Condition, f.Target, f.Imperative)
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
