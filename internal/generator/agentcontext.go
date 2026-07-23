package generator

import (
	"fmt"
	"strings"

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
//
// today (YYYY-MM-DD) is threaded through explicitly rather than computed
// internally via time.Now(), so two renders with the same today produce
// byte-identical output regardless of wall-clock date (R-... idempotency;
// see gen-spec's --today flag and TestBuildAgentContext_TodayIsInjectable /
// TestBuildAgentContext_SameTodayIsByteIdentical).

// agentContextWhatNowLimit is the number of top what-now actions surfaced in
// AGENT-CONTEXT.md (N=10 per the P2-1 task spec).
const agentContextWhatNowLimit = 10

// BuildAgentContext renders docs/gen/AGENT-CONTEXT.md for one domain: a
// compact agent-boot digest, target < 15KB. claudeMDCharCount feeds the
// reused LIVE-STATE budget line exactly as it does for the root crystal
// (see BuildLiveState); domainName qualifies the `hotam req show <id>`
// pointer and the full-MD reference paths. consumer selects the same
// Constitution-index categorization scheme CLAUDE.md's own CONSTITUTION
// block uses (buildConstitutionIndexModel — id-prefix bucketing for FULL,
// enforcement-tier bucketing for CONSUMER; see
// claudemd_constitutionindex.go's consumerCategoryOrder doc comment).
func BuildAgentContext(g *ontology.Graph, domainName string, claudeMDCharCount int, today string, consumer bool) string {
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

	lines = append(lines, BuildLiveState(g, domainName, claudeMDCharCount, today))
	lines = append(lines, "")

	lines = append(lines, renderAgentContextWhatNow(g, today)...)
	lines = append(lines, "")

	lines = append(lines, renderAgentContextCounters(g, today)...)
	lines = append(lines, "")

	lines = append(lines, renderAgentContextConstitutionIndex(g, consumer)...)
	lines = append(lines, "")

	lines = append(lines, "## Details on demand", "")
	lines = append(lines, fmt.Sprintf("- One requirement's full claim + WHY + assumptions: `hotam req show <id> --domain domains/%s`.", domainName))
	lines = append(lines, fmt.Sprintf("- Full requirement roster: `domains/%s/docs/gen/REQUIREMENTS.md`.", domainName))
	lines = append(lines, fmt.Sprintf("- Tension clusters: `domains/%s/docs/gen/TENSIONS.md`.", domainName))
	lines = append(lines, fmt.Sprintf("- Rejection history: `domains/%s/docs/gen/HISTORY.md`.", domainName))
	lines = append(lines, fmt.Sprintf("- Enforcement gap detail: `domains/%s/docs/gen/UNENFORCED.md`.", domainName))
	lines = append(lines, fmt.Sprintf("- Framework-internal atoms: `domains/%s/docs/gen/FRAMEWORK-INVARIANTS.md`.", domainName))
	lines = append(lines, "- Review-freshness detail (which ids, how overdue): `hotam due --domain domains/"+domainName+"`.")
	lines = append(lines, "")

	lines = append(lines, renderAgentContextDocsGenIndex(domainName)...)

	return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n") + "\n"
}

// renderAgentContextWhatNow renders the "## Top actions" section: the first
// agentContextWhatNowLimit signals from diagnose.DiagnoseSignals, the same
// ranked list `hotam what-now` prints, reused verbatim rather than
// re-deriving priority ordering here.
func renderAgentContextWhatNow(g *ontology.Graph, today string) []string {
	out := []string{fmt.Sprintf("## Top actions (what-now, top %d)", agentContextWhatNowLimit), ""}
	signals := diagnose.DiagnoseSignals(g, today)
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
func renderAgentContextCounters(g *ontology.Graph, today string) []string {
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

// renderAgentContextDocsGenIndex renders the "## docs/gen/ file index" section:
// a which-do-I-actually-need-to-read categorization of every top-level file
// this domain's docs/gen/ holds (R-domain-owns-docs-gen's manifest), split into
// MANDATORY (named directly in this domain's CLAUDE.md boot text — an agent
// following ORIENT/LOCATE reads these essentially every session), REFERENCE
// (thinking/tools + the remaining topic docs — read on demand for a specific
// task, never at boot), and ARCHIVAL (change-log / self-contained-snapshot
// material with no boot-time consumer). Added per external-review task #83:
// the review's complaint was that an agent has no way to tell boot-critical
// docs/gen files from optional/historical ones without reading every one; this
// closes that gap in the generator source (not a hand-edit of the output) so
// the index regenerates and can never drift from the actual boot text above.
// graph.json is explicitly ARCHIVAL/self-contained here, NOT deadweight: its
// existence is mandated by R-drift-structurally-impossible ("The generated
// docs/gen/*.md and graph.json shall equal the regeneration of the current
// graph, byte-for-byte") and verified by
// TestBuildGraphJSON_ByteIdenticalToFixture — it is a read-only, portable,
// human-browsable snapshot of the domain graph co-located with its own
// markdown shadow, not something any tool reads back at runtime.
func renderAgentContextDocsGenIndex(domainName string) []string {
	base := "domains/" + domainName + "/docs/gen/"
	return []string{
		"## docs/gen/ file index (which files do I actually need to read?)",
		"",
		"MANDATORY (named directly in this domain's CLAUDE.md boot text — read essentially every session):",
		"- `AGENT-CONTEXT.md` (this file) — compact boot digest.",
		"- `" + base + "REQUIREMENTS.md` — full requirement roster (LOCATE step).",
		"- `" + base + "TENSIONS.md` — conflict clusters (LOCATE step).",
		"- `" + base + "UNENFORCED.md` — enforcement-gap detail behind the top-action line.",
		"- `" + base + "FRAMEWORK-INVARIANTS.md` — framework-internal atoms behind the Constitution index.",
		"",
		"REFERENCE (load on demand for a specific task, not at boot):",
		"- `" + base + "CONSTITUTION.md`, `GLOSSARY.md`, `REPO-MAP.md` — narrative expansions of sections already summarized above.",
		"- `" + base + "atoms-operator.md`, `atoms-substrate.md`, `atoms-discipline.md`, `atoms-check.md` — per-category atom detail.",
		"- `" + base + "live-state.md` — the same pulse this file's Live-state section already carries, standalone.",
		"- `" + base + "OPEN.md` — open-question detail behind the OPEN status.",
		"- `" + base + "thinking/<slug>.md` — one deep-dive per §-section, loaded only when a §-anchor needs its full Canon/Narrative/Why.",
		"- `" + base + "tools/INDEX.md` — entry point for the tool-docs directory: splits the registry into Implemented (real commands) vs Planned (methodology surface only).",
		"- `" + base + "tools/<tool>.md` — one purpose doc per tool, loaded only when working with that tool.",
		"",
		"ARCHIVAL (historical/self-contained — read only when investigating past decisions, never at boot):",
		"- `" + base + "HISTORY.md` — REJECTED + DECIDED change-log; anti-relitigation lookup only.",
		"- `" + base + "graph.json` — read-only regenerated snapshot of this domain's graph.json, kept byte-identical by R-drift-structurally-impossible for archival/portability; no tool reads it back.",
	}
}

// renderAgentContextConstitutionIndex renders the "## Constitution index"
// section: id + enforcement-flag only (no claim text), reusing
// buildConstitutionIndexModel/clusterIndexItems — the exact model that
// backs CLAUDE.md's own CONSTITUTION block (claudemd_constitutionindex.go)
// — rather than a second independent index builder.
func renderAgentContextConstitutionIndex(g *ontology.Graph, consumer bool) []string {
	out := []string{
		"## Constitution index (id + flag only — [E] ENFORCED · [S] STRUCTURAL · [P] PROSE)",
		"",
	}
	categories := buildConstitutionIndexModel(g, consumer)
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
