package main

import (
	"fmt"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/gate"
)

func cmdGate(args []string) error {
	fs := newFlagSet("gate")
	domain := fs.String("domain", "", "domain directory (default: "+defaultDomainRel+")")
	asJSON := fs.Bool("json", false, "emit machine-readable JSON")
	fs.Parse(args)

	if fs.NArg() < 1 {
		return fmt.Errorf("usage: hotam gate <target-anchor> [--domain <path>] [--json]")
	}
	target := fs.Arg(0)

	domainDir, err := resolveDomain(*domain)
	if err != nil {
		return err
	}
	g, err := loadDomainGraph(domainDir)
	if err != nil {
		return err
	}
	result := gate.SelectTier1(target, g)
	if *asJSON {
		if result.NodeIDs == nil {
			result.NodeIDs = []string{}
		}
		return printJSON(result)
	}
	printGateResult(result)
	return nil
}

func printGateResult(r gate.GateResult) {
	confident := "false"
	if r.Confident {
		confident = "true"
	}
	fmt.Printf("confident: %s\n", confident)
	fmt.Printf("node_ids: %s\n", strings.Join(r.NodeIDs, " "))
	fmt.Printf("reason: %s\n", r.Reason)
}
