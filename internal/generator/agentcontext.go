package generator

import (
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/diagnose"
	"github.com/PHPCraftdream/HotamSpec/internal/freshness"
	"github.com/PHPCraftdream/HotamSpec/internal/invariants"
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
// AgentContextMDHasContent reports whether AGENT-CONTEXT.md carries real
// domain content — i.e. whether the graph is non-empty (task #364: withheld
// entirely from the docs/gen/ write set when the domain has zero axes/
// stakeholders/requirements/conflicts/assumptions/operators/processes/goals/
// entity_types/entities, mirroring the conditional-write pattern
// DECISIONS.md/ENTITIES.md/TENSIONS.md/PIPELINE.md/MODELS.md already use). A
// genuinely empty domain's AGENT-CONTEXT.md would carry nothing but a
// zeroed-out LIVE-STATE block and empty index sections — no different in
// kind from every other file in this set that this same task gates.
func AgentContextMDHasContent(g *ontology.Graph) bool {
	return !g.IsEmpty()
}

func BuildAgentContext(g *ontology.Graph, domainName string, claudeMDCharCount int, today string, consumer bool) string {
	return buildAgentContext(g, domainName, claudeMDCharCount, today, consumer, invariants.AllViolations(g), "")
}

// BuildAgentContextRoot is BuildAgentContext with the repo root threaded
// through for absolute-path normalization.
func BuildAgentContextRoot(g *ontology.Graph, domainName string, claudeMDCharCount int, today string, consumer bool, repoRoot string) string {
	return BuildAgentContextRootWithViolations(g, domainName, claudeMDCharCount, today, consumer, invariants.AllViolations(g), repoRoot)
}

// BuildAgentContextRootWithViolations renders the locale view using the
// caller's shared violation snapshot, avoiding a second per-locale scan.
func BuildAgentContextRootWithViolations(g *ontology.Graph, domainName string, claudeMDCharCount int, today string, consumer bool, violations []invariants.Violation, repoRoot string) string {
	return rewriteRepoAbsPaths(repoRoot, buildAgentContext(g, domainName, claudeMDCharCount, today, consumer, violations, repoRoot))
}

func buildAgentContext(g *ontology.Graph, domainName string, claudeMDCharCount int, today string, consumer bool, violations []invariants.Violation, repoRoot string) string {
	if domainName == "" {
		domainName = "hotam-spec-self"
	}

	tensionsWritten := TensionsMDHasContent(g)

	lines := []string{
		serviceText(g, generatedHeaderComment),
		"",
		serviceText(g, "# AGENT-CONTEXT.md — compact agent boot digest (%s)", domainName),
		"",
		serviceText(g, "This file is the compact entry point for an agent session — target < 15KB. It is a live-state pulse, a top-action shortlist, and an id+flag-only constitution index, NOT a substitute for the full reference docs. Full claims/WHY/assumptions, tension detail, and rejection history remain in the other docs/gen/*.md files below — those are reference-only, load them on demand, not at every boot."),
		"",
	}

	lines = append(lines, BuildLiveStateWithViolations(g, domainName, claudeMDCharCount, today, violations))
	lines = append(lines, "")

	lines = append(lines, renderAgentContextWhatNow(g, today, violations)...)
	lines = append(lines, "")

	lines = append(lines, renderAgentContextCounters(g, today)...)
	lines = append(lines, "")

	lines = append(lines, renderAgentContextConstitutionIndex(g, consumer)...)
	lines = append(lines, "")

	lines = append(lines, serviceText(g, "## Details on demand"), "")
	lines = append(lines, serviceText(g, "- One requirement's full claim + WHY + assumptions: `hotam req show <id> --domain domains/%s`.", domainName))
	lines = append(lines, serviceText(g, "- Full requirement roster: `%s`.", localizedDomainDocPath(g, domainName, "REQUIREMENTS.md")))
	if tensionsWritten {
		lines = append(lines, serviceText(g, "- Tension clusters: `%s`.", localizedDomainDocPath(g, domainName, "TENSIONS.md")))
	}
	lines = append(lines, serviceText(g, "- Rejection history: `%s`.", localizedDomainDocPath(g, domainName, "HISTORY.md")))
	lines = append(lines, serviceText(g, "- Enforcement gap detail: `%s`.", localizedDomainDocPath(g, domainName, "UNENFORCED.md")))
	lines = append(lines, serviceText(g, "- Framework-internal atoms: `%s`.", localizedDomainDocPath(g, domainName, "FRAMEWORK-INVARIANTS.md")))
	lines = append(lines, serviceText(g, "- Review-freshness detail (which ids, how overdue): `hotam due --domain domains/%s`.", domainName))
	lines = append(lines, "")

	lines = append(lines, renderAgentContextDocsGenIndex(g, domainName, tensionsWritten)...)

	return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n") + "\n"
}

func localizedDomainDocPath(g *ontology.Graph, domainName, filename string) string {
	return "domains/" + domainName + "/" + localizedDocumentPath(g, "docs/gen/"+filename)
}

// renderAgentContextWhatNow renders the "## Top actions" section: the first
// agentContextWhatNowLimit signals from diagnose.DiagnoseSignals, the same
// ranked list `hotam what-now` prints, reused verbatim rather than
// re-deriving priority ordering here.
func renderAgentContextWhatNow(g *ontology.Graph, today string, violations []invariants.Violation) []string {
	out := []string{serviceText(g, "## Top actions (what-now, top %d)", agentContextWhatNowLimit), ""}
	signals := diagnose.DiagnoseSignalsWithViolations(g, today, violations)
	if len(signals) == 0 {
		out = append(out, serviceText(g, "_(none — graph clean)_"))
		return out
	}
	end := agentContextWhatNowLimit
	if end > len(signals) {
		end = len(signals)
	}
	for _, signal := range signals[:end] {
		out = append(out, serviceText(g, "- [P%d] %s on `%s` — %s", signal.Priority, serviceText(g, bandLabel[signal.Priority]), signal.Target, signal.Message))
	}
	if len(signals) > end {
		out = append(out, "", serviceText(g, "_(showing %d of %d — full list: `hotam what-now`)_", end, len(signals)))
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
		serviceText(g, "## Status counters"),
		"",
		serviceText(g, "SETTLED %d · DRAFT %d · REJECTED %d · OVERDUE %d (as of %s)", settled, draft, rejected, overdue, today),
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
//
// tensionsWritten gates whether the TENSIONS.md entry appears at all: a domain
// with no conflict/axis nodes does not write TENSIONS.md, so listing it as
// MANDATORY would be a dead link (task #361). PIPELINE.md and MODELS.md are
// NOT listed here at all (they are REFERENCE-level or absent, tracked in
// REPO-MAP.md's "not written" lines instead).
func renderAgentContextDocsGenIndex(g *ontology.Graph, domainName string, tensionsWritten bool) []string {
	domainPath := func(filename string) string {
		return "domains/" + domainName + "/" + localizedDocumentPath(g, "docs/gen/"+filename)
	}
	projectPath := func(filename string) string {
		return localizedDocumentPath(g, "framework/"+filename)
	}
	thinkingPattern := strings.TrimSuffix(localizedDocumentPath(g, "docs/gen/thinking/slug.md"), "slug.md") + "<slug>.md"
	toolPattern := strings.TrimSuffix(projectPath("tools/tool.md"), "tool.md") + "<tool>.md"
	mandatory := []string{
		serviceText(g, "## docs/gen/ file index (which files do I actually need to read?)"),
		"",
		serviceText(g, "MANDATORY (named directly in this domain's CLAUDE.md boot text — read essentially every session):"),
		serviceText(g, "- `%s` (this file) — compact boot digest.", domainPath("AGENT-CONTEXT.md")),
		serviceText(g, "- `%s` — full requirement roster (LOCATE step).", domainPath("REQUIREMENTS.md")),
	}
	if tensionsWritten {
		mandatory = append(mandatory, serviceText(g, "- `%s` — conflict clusters (LOCATE step).", domainPath("TENSIONS.md")))
	}
	mandatory = append(mandatory,
		serviceText(g, "- `%s` — enforcement-gap detail behind the top-action line.", domainPath("UNENFORCED.md")),
		serviceText(g, "- `%s` — framework-internal atoms behind the Constitution index.", domainPath("FRAMEWORK-INVARIANTS.md")),
		"",
		serviceText(g, "REFERENCE (load on demand for a specific task, not at boot):"),
		serviceText(g, "- `%s`, `%s` — narrative expansions of sections already summarized above.", domainPath("CONSTITUTION.md"), domainPath("REPO-MAP.md")),
		serviceText(g, "- `%s` — methodology controlled vocabulary (project-shared).", projectPath("GLOSSARY.md")),
		serviceText(g, "- `%s`, `%s`, `%s`, `%s` — per-category atom detail.", domainPath("atoms-operator.md"), domainPath("atoms-substrate.md"), domainPath("atoms-discipline.md"), domainPath("atoms-check.md")),
		serviceText(g, "- `%s` — the same pulse this file's Live-state section already carries, standalone.", domainPath("live-state.md")),
		serviceText(g, "- `%s` — open-question detail behind the OPEN status.", domainPath("OPEN.md")),
		serviceText(g, "- `%s` — one deep-dive per §-section, loaded only when a §-anchor needs its full Canon/Narrative/Why.", thinkingPattern),
		serviceText(g, "- `%s` — entry point for the tool-docs directory: splits the registry into Implemented (real commands) vs Planned (methodology surface only).", projectPath("tools/INDEX.md")),
		serviceText(g, "- `%s` — one purpose doc per tool, loaded only when working with that tool.", toolPattern),
		"",
		serviceText(g, "ARCHIVAL (historical/self-contained — read only when investigating past decisions, never at boot):"),
		serviceText(g, "- `%s` — REJECTED + DECIDED change-log; anti-relitigation lookup only.", domainPath("HISTORY.md")),
		serviceText(g, "- `domains/%s/graph.json` — read-only regenerated snapshot of this domain's graph.json, kept byte-identical by R-drift-structurally-impossible for archival/portability; no tool reads it back.", domainName),
	)
	return mandatory
}

// renderAgentContextConstitutionIndex renders the "## Constitution index"
// section: id + enforcement-flag only (no claim text), reusing
// buildConstitutionIndexModel/clusterIndexItems — the exact model that
// backs CLAUDE.md's own CONSTITUTION block (claudemd_constitutionindex.go)
// — rather than a second independent index builder.
func renderAgentContextConstitutionIndex(g *ontology.Graph, consumer bool) []string {
	out := []string{
		serviceText(g, "## Constitution index (id + flag only — [E] ENFORCED · [S] STRUCTURAL · [P] PROSE)"),
		"",
	}
	categories := buildConstitutionIndexModel(g, consumer)
	if len(categories) == 0 {
		out = append(out, serviceText(g, "_No SETTLED requirements yet._"))
		return out
	}
	for _, category := range categories {
		items := clusterIndexItems(category.Requirements)
		out = append(out, serviceText(g, "**%s** — %s", serviceText(g, category.Label), strings.Join(items, " · ")))
		out = append(out, "")
	}
	return out
}
