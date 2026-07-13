package main

import (
	"fmt"
	"os"

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
	// contract.
	if len(violations) > 0 {
		if !*asJSON {
			fmt.Fprintf(os.Stderr, "%d violation(s) found\n", len(violations))
		}
		os.Exit(1)
	}
	if !*asJSON {
		fmt.Println("0 violations — graph clean")
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
