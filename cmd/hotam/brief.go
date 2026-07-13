package main

import (
	"fmt"
	"time"

	"github.com/PHPCraftdream/HotamSpec/internal/query"
)

// cmdBrief implements `hotam brief <anchor-id> [--domain <path>] [--today
// YYYY-MM-DD] [--json]`: a single-call orientation aggregator for any anchor
// (Requirement, Conflict, or Assumption). It composes the full card +
// one-hop neighborhood + freshness (for Requirements), replacing the 3-4
// separate round-trips (`hotam req show` + `hotam req context` + `hotam req
// related` + `hotam due`) an agent previously needed. Read-only; exit code
// is 0 on success, 1 on error (unknown anchor, bad graph).
func cmdBrief(args []string) error {
	fs := newFlagSet("brief")
	domain := fs.String("domain", "", "domain directory (default: "+defaultDomainRel+")")
	todayFlag := fs.String("today", "", "date in YYYY-MM-DD format (default: system date)")
	asJSON := fs.Bool("json", false, "emit machine-readable JSON")
	fs.Parse(args)

	if fs.NArg() < 1 {
		return fmt.Errorf("usage: hotam brief <anchor-id> [--domain <path>] [--today YYYY-MM-DD] [--json]")
	}
	id := fs.Arg(0)

	domainDir, err := resolveDomain(*domain)
	if err != nil {
		return err
	}
	today := *todayFlag
	if today == "" {
		today = time.Now().Format("2006-01-02")
	}

	g, err := loadDomainGraph(domainDir)
	if err != nil {
		return err
	}

	card, err := query.Brief(g, id, today)
	if err != nil {
		return err
	}

	if *asJSON {
		return printJSON(card)
	}
	fmt.Println(query.FormatBrief(card))
	return nil
}
