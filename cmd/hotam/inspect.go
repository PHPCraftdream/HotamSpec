package main

import (
	"fmt"

	"github.com/PHPCraftdream/HotamSpecGo/internal/diagnose"
)

// cmdInspect implements `hotam inspect [--domain <path>] [--json] [--limit N]`:
// a detailed, on-demand ADVISORY listing of semantic-conflict candidates —
// pairs/clusters of nodes that MIGHT be in tension, each with the evidence
// that flagged them and a fixed recommendation to consider a ProposedConflict.
//
// all-violations==0 only proves STRUCTURAL correctness (invariants package);
// whether two natural-language SETTLED claims actually contradict each
// other is a semantic question no structural check answers. `what-now`
// already surfaces a terse cluster signal + a freshness signal at
// PLatentConnector/PAdvisory priority (internal/diagnose/signal.go); this
// command does NOT duplicate that — it is the detailed drill-down a steward
// or agent reaches for on demand, backed by the SAME underlying detectors
// (internal/diagnose.AllCandidates) plus additional heuristics not surfaced
// in the terse what-now view (lexical claim overlap, axis co-reference).
//
// Exit code is ALWAYS 0: inspect informs, it never gates (R-ai-presents-not-decides).
func cmdInspect(args []string) error {
	fs := newFlagSet("inspect")
	domain := fs.String("domain", "", "domain directory (default: "+defaultDomainRel+")")
	asJSON := fs.Bool("json", false, "emit machine-readable JSON")
	limit := fs.Int("limit", 20, "max candidates to print (0 = unlimited)")
	fs.Parse(args)

	domainDir, err := resolveDomain(*domain)
	if err != nil {
		return err
	}
	g, err := loadDomainGraph(domainDir)
	if err != nil {
		return err
	}

	candidates := diagnose.AllCandidates(g)
	total := len(candidates)
	shown := candidates
	if *limit > 0 && len(shown) > *limit {
		shown = shown[:*limit]
	}

	if *asJSON {
		return printJSON(shown)
	}
	fmt.Print(formatInspectReport(shown, total))
	return nil
}

func formatInspectReport(shown []diagnose.Candidate, total int) string {
	if total == 0 {
		return "hotam inspect — no conflict candidates found (advisory; exit code always 0).\n"
	}
	out := fmt.Sprintf("hotam inspect — %d conflict candidate(s) found (showing %d), advisory only:\n\n", total, len(shown))
	for i, c := range shown {
		out += fmt.Sprintf("%d. [%s] %s\n", i+1, c.Heuristic, c.ID)
		out += fmt.Sprintf("   members: %v\n", c.Members)
		out += fmt.Sprintf("   evidence: %s\n", c.Evidence)
		if c.Score > 0 {
			out += fmt.Sprintf("   score: %d\n", c.Score)
		}
		out += fmt.Sprintf("   %s\n\n", c.Recommendation)
	}
	if total > len(shown) {
		out += fmt.Sprintf("… and %d more (raise --limit to see them).\n", total-len(shown))
	}
	return out
}
