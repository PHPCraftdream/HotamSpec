package generator

import (
	"fmt"
	"strings"
	"time"

	"github.com/PHPCraftdream/HotamSpec/internal/diagnose"
	"github.com/PHPCraftdream/HotamSpec/internal/freshness"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// Canon: §Graph — AGENT-CONTEXT.md is the compact per-domain agent-boot
// digest (R-constitution-is-index's sibling for the docs/gen/ side): a
// single file under ~15KB that gives an agent the live-state pulse, the
// top-N what-now actions, an id+flag-only constitution index, and
// SETTLED/DRAFT/REJECTED/OVERDUE counters, with a pointer to `hotam req
// show <id>` for full detail and to the existing full REQUIREMENTS.md /
// TENSIONS.md / etc. as reference-only. It deliberately reuses the exact
// building blocks the full docs already use (BuildLiveState from
// livestate.go, DiagnoseSignals from internal/diagnose, and the
// buildConstitutionIndexModel/clusterIndexItems clustering from
// claudemd_constitutionindex.go) rather than inventing new counting logic —
// see TaskList P2-1.

// agentContextWhatNowLimit is the number of top what-now actions surfaced in
// AGENT-CONTEXT.md (N=10 per the P2-1 task spec).
const agentContextWhatNowLimit = 10

// BuildAgentContext renders docs/gen/AGENT-CONTEXT.md for one domain: a
// compact agent-boot digest, target < 15KB. claudeMDCharCount feeds the
// reused LIVE-STATE budget line exactly as it does for the root crystal
// (see BuildLiveState); domainName qualifies the `hotam req show <id>`
// pointer and the full-MD reference paths.
func BuildAgentContext(g *ontology.Graph, domainName string, claudeMDCharCount int) string {
	if domainName == "" {
		domainName = "hotam-spec-self"
	}

	lines := []string{
		generatedHeaderComment,
		"",
		"# AGENT-CONTEXT.md — compact agent boot digest (" + domainName + ")",
		"",
		"This file is the compact entry point for an agent session — target < 15KB. It is a live-state pulse, a top-action shortlist, and an id+flag-only constitution index, NOT a substitute for the full reference docs. Full claims/WHY/assumptions, tension detail, and rejection history remain in the other docs/gen/*.md files below — those are reference-only, load them on demand, not at every boot.",
		"",
	}

	lines = append(lines, BuildLiveState(g, claudeMDCharCount))
	lines = append(lines, "")

	lines = append(lines, renderAgentContextWhatNow(g)...)
	lines = append(lines, "")

	lines = append(lines, renderAgentContextCounters(g)...)
	lines = append(lines, "")

	lines = append(lines, renderAgentContextConstitutionIndex(g)...)
	lines = append(lines, "")

	lines = append(lines, "## Details on demand", "")
	lines = append(lines, fmt.Sprintf("- One requirement's full claim + WHY + assumptions: `hotam req show <id> --domain domains/%s`.", domainName))
	lines = append(lines, fmt.Sprintf("- Full requirement roster: `domains/%s/docs/gen/REQUIREMENTS.md`.", domainName))
	lines = append(lines, fmt.Sprintf("- Tension clusters: `domains/%s/docs/gen/TENSIONS.md`.", domainName))
	lines = append(lines, fmt.Sprintf("- Rejection history: `domains/%s/docs/gen/HISTORY.md`.", domainName))
	lines = append(lines, fmt.Sprintf("- Enforcement gap detail: `domains/%s/docs/gen/UNENFORCED.md`.", domainName))
	lines = append(lines, fmt.Sprintf("- Framework-internal atoms: `domains/%s/docs/gen/FRAMEWORK-INVARIANTS.md`.", domainName))
	lines = append(lines, "- Review-freshness detail (which ids, how overdue): `hotam due --domain domains/"+domainName+"`.")

	return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n") + "\n"
}

// renderAgentContextWhatNow renders the "## Top actions" section: the first
// agentContextWhatNowLimit signals from diagnose.DiagnoseSignals, the same
// ranked list `hotam what-now` prints, reused verbatim rather than
// re-deriving priority ordering here.
func renderAgentContextWhatNow(g *ontology.Graph) []string {
	out := []string{fmt.Sprintf("## Top actions (what-now, top %d)", agentContextWhatNowLimit), ""}
	signals := diagnose.DiagnoseSignals(g)
	if len(signals) == 0 {
		out = append(out, "_(none — graph clean)_")
		return out
	}
	end := agentContextWhatNowLimit
	if end > len(signals) {
		end = len(signals)
	}
	for _, s := range signals[:end] {
		out = append(out, fmt.Sprintf("- [P%d] %s on `%s` — %s", s.Priority, bandLabel[s.Priority], s.Target, s.Message))
	}
	if len(signals) > end {
		out = append(out, "", fmt.Sprintf("_(showing %d of %d — full list: `hotam what-now`)_", end, len(signals)))
	}
	return out
}

// renderAgentContextCounters renders the "## Status counters" section:
// SETTLED / DRAFT / REJECTED status totals plus OVERDUE (via
// internal/freshness — the same classifier `hotam due` uses, not a
// hand-rolled recount).
func renderAgentContextCounters(g *ontology.Graph) []string {
	var settled, draft, rejected int
	for _, r := range g.Requirements {
		switch r.Status {
		case ontology.StatusSETTLED:
			settled++
		case ontology.StatusDRAFT:
			draft++
		case ontology.StatusREJECTED:
			rejected++
		}
	}

	today := time.Now().Format("2006-01-02")
	classified := freshness.ClassifyGraph(g, today)
	overdue := 0
	for _, c := range classified {
		if c.Status == freshness.Overdue {
			overdue++
		}
	}

	return []string{
		"## Status counters",
		"",
		fmt.Sprintf("SETTLED %d · DRAFT %d · REJECTED %d · OVERDUE %d (as of %s)", settled, draft, rejected, overdue, today),
	}
}

// renderAgentContextConstitutionIndex renders the "## Constitution index"
// section: id + enforcement-flag only (no claim text), reusing
// buildConstitutionIndexModel/clusterIndexItems — the exact model that
// backs CLAUDE.md's own CONSTITUTION block (claudemd_constitutionindex.go)
// — rather than a second independent index builder.
func renderAgentContextConstitutionIndex(g *ontology.Graph) []string {
	out := []string{
		"## Constitution index (id + flag only — [E] ENFORCED · [S] STRUCTURAL · [P] PROSE)",
		"",
	}
	categories := buildConstitutionIndexModel(g)
	if len(categories) == 0 {
		out = append(out, "_No SETTLED requirements yet._")
		return out
	}
	for _, category := range categories {
		items := clusterIndexItems(category.Requirements)
		out = append(out, fmt.Sprintf("**%s** — %s", category.Label, strings.Join(items, " · ")))
		out = append(out, "")
	}
	return out
}
