package main

import (
	"fmt"
	"os"

	"github.com/PHPCraftdream/HotamSpecGo/internal/invariants"
)

func cmdAllViolations(args []string) error {
	fs := newFlagSet("all-violations")
	domain := fs.String("domain", "", "domain directory (default: "+defaultDomainRel+")")
	fs.Parse(args)

	domainDir, err := resolveDomain(*domain)
	if err != nil {
		return err
	}
	violations, err := allViolations(domainDir)
	if err != nil {
		return err
	}
	for _, v := range violations {
		fmt.Printf("[%s] %s: %s\n", v.Check, v.ID, v.Message)
	}
	if len(violations) > 0 {
		fmt.Fprintf(os.Stderr, "%d violation(s) found\n", len(violations))
		os.Exit(1)
	}
	fmt.Println("0 violations — graph clean")
	return nil
}

func allViolations(domainDir string) ([]invariants.Violation, error) {
	g, err := loadDomainGraph(domainDir)
	if err != nil {
		return nil, err
	}
	return invariants.AllViolations(g), nil
}
