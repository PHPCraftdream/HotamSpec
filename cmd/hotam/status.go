package main

import (
	"fmt"
	"time"

	"github.com/PHPCraftdream/HotamSpec/internal/diagnose"
	"github.com/PHPCraftdream/HotamSpec/internal/freshness"
	"github.com/PHPCraftdream/HotamSpec/internal/invariants"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// cmdStatus implements `hotam status [--domain <path>] [--today YYYY-MM-DD]
// [--json]`: a single-shot compact summary that folds together what an
// agent would otherwise have to reconstruct by separately running
// `what-now` (top action + debt), `due` (freshness), and `all-violations`
// (invariant count) — three graph loads and three command invocations
// reduced to one, so an agent doesn't burn context re-deriving the same
// picture piecemeal (review task P2-8 / TaskList #80).
//
// status is advisory, like due/inspect/confront: it never gates. Exit code
// is always 0, even when violations > 0 or debt is high — `all-violations`
// already IS the hard gate command for violations; status only reports the
// count alongside everything else so an agent can decide what to do next
// without a second round-trip.
//
// It is pure composition, not reimplementation: every number below comes
// from calling the SAME functions the standalone commands call
// (diagnose.DiagnoseSignals for the top action + debt counts — the same
// computation formatSignals/BuildLiveState use — freshness.ClassifyGraph
// for overdue/never-reviewed, invariants.AllViolations for the violation
// count), on ONE loaded graph, so status can never silently drift from what
// `what-now`/`due`/`all-violations` independently report.
func cmdStatus(args []string) error {
	fs := newFlagSet("status")
	domain := fs.String("domain", "", "domain directory (default: "+defaultDomainRel+")")
	todayFlag := fs.String("today", "", "date in YYYY-MM-DD format (default: system date)")
	asJSON := fs.Bool("json", false, "emit machine-readable JSON")
	fs.Parse(args)

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

	report := buildStatusReport(g, today)

	if *asJSON {
		return printJSON(report)
	}
	fmt.Println(formatStatusReport(report))
	return nil
}

// StatusReport is the JSON/text-rendered shape of `hotam status`'s output.
// Field names are flat and explicit so an agent can consume them in one
// pass without further processing (R-agent-never-lost's single-shot-pulse
// counterpart to the LIVE-STATE crystal block).
type StatusReport struct {
	Today              string `json:"today"`
	TopAction          string `json:"top_action"`
	SettledCount       int    `json:"settled_count"`
	EnforcedCount      int    `json:"enforced_count"`
	CloseableDebtCount int    `json:"closeable_debt_count"`
	OverdueCount       int    `json:"overdue_count"`
	NeverReviewedCount int    `json:"never_reviewed_count"`
	ViolationCount     int    `json:"violation_count"`
	NodeCount          int    `json:"node_count"`
}

// buildStatusReport aggregates the same signals/counts what-now, due, and
// all-violations independently compute, on a single already-loaded graph.
// See TestBuildStatusReport_MatchesWhatNowDueAllViolations for the
// consistency proof that guards against this ever drifting from those
// commands' own logic.
func buildStatusReport(g *ontology.Graph, today string) StatusReport {
	signals := diagnose.DiagnoseSignals(g)
	topAction := "none — graph clean"
	if len(signals) > 0 {
		topAction = formatSingleSignal(signals[0])
	}

	var settledCount, enforcedCount, closeableDebtCount int
	for _, r := range g.Requirements {
		if r.Status != ontology.StatusSETTLED {
			continue
		}
		settledCount++
		if r.Enforcement == ontology.EnforcementENFORCED {
			enforcedCount++
		}
		if r.IsCloseableDebt() {
			closeableDebtCount++
		}
	}

	classified := freshness.ClassifyGraph(g, today)
	var overdueCount, neverReviewedCount int
	for _, c := range classified {
		switch c.Status {
		case freshness.Overdue:
			overdueCount++
		case freshness.NeverReviewed:
			neverReviewedCount++
		}
	}

	violations := invariants.AllViolations(g)

	nodeCount := len(g.Requirements) + len(g.Conflicts) + len(g.Assumptions)

	return StatusReport{
		Today:              today,
		TopAction:          topAction,
		SettledCount:       settledCount,
		EnforcedCount:      enforcedCount,
		CloseableDebtCount: closeableDebtCount,
		OverdueCount:       overdueCount,
		NeverReviewedCount: neverReviewedCount,
		ViolationCount:     len(violations),
		NodeCount:          nodeCount,
	}
}

func formatStatusReport(r StatusReport) string {
	var out string
	out += fmt.Sprintf("hotam status — pulse as of %s\n", r.Today)
	out += fmt.Sprintf("top action:  %s\n", r.TopAction)
	out += fmt.Sprintf("debt:        %d/%d SETTLED ENFORCED · %d closeable debt (ENFORCEABLE, still PROSE/STRUCTURAL)\n", r.EnforcedCount, r.SettledCount, r.CloseableDebtCount)
	out += fmt.Sprintf("freshness:   %d overdue · %d never-reviewed\n", r.OverdueCount, r.NeverReviewedCount)
	out += fmt.Sprintf("violations:  %d\n", r.ViolationCount)
	out += fmt.Sprintf("graph:       %d nodes (req+conflict+assumption)\n", r.NodeCount)
	return out
}
