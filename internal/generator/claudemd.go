package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/PHPCraftdream/HotamSpec/internal/diagnose"
	"github.com/PHPCraftdream/HotamSpec/internal/invariants"
	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// Canon: §Graph — root CLAUDE.md generation (R-claude-md-template-driven).
//
// The root CLAUDE.md.template.txt is a
// fixed header plus exactly two placeholder lines, "<!-- mind -->" and
// "<!-- business -->", substituted with the MIND bucket (domain-agnostic
// "how to think" layer: OPERATOR-ROLE, MEDIATION-LOOP, EMBEDDED-THINKING,
// EMBEDDED-TOOLS, OPERATOR-RECURSION) and the BUSINESS bucket
// (domain-specific "what this claims" layer: LIVE-STATE, DOMAIN-MAP,
// CONSTITUTION, AGENT-MAP, CONCEPT-MAP, RECENTLY-REJECTED) respectively.
// Every individual block still carries its own BEGIN/END sentinel-comment
// pair (WrapBlock); MIND/BUSINESS are concatenations of those blocks.
//
// Under the FULL profile the template places <!-- mind --> BEFORE
// <!-- business --> (methodology-first — the engine's own self-hosting
// domains want the operator seed ahead of domain state). Under the CONSUMER
// profile (external business domains) the placeholder ORDER FLIPS —
// <!-- business --> before <!-- mind --> — via claudeMDTemplateConsumer, so
// the domain's own essence (purpose/stakeholders/live-state/…) is the first
// thing a freshly-booted operator reads, with the Hotam-Spec methodology
// seed appearing as a compact "how we work" block afterward (external
// review P1: a consumer-profile crystal opening with ~178 lines of
// framework methodology before any domain content buries the "what is this
// project" answer past the first screen). RenderBusinessContent already
// reorders the BUSINESS bucket's OWN internals for consumer (task A3); this
// template split additionally reorders the two BUCKETS relative to each
// other and swaps the file's opening header line — both gated the same way,
// by the same consumer bool, so the two reorders (intra-bucket, inter-bucket)
// compose into one coherent "business first, methodology second" document.
const mindPlaceholder = "<!-- mind -->"
const businessPlaceholder = "<!-- business -->"

// claudeMDTemplate is embedded verbatim from CLAUDE.md.template.txt.
// Embedding it here — rather than reading an external file — keeps root
// CLAUDE.md generation self-contained within this package and avoids a
// second file-ownership surface at repo root while still producing
// byte-identical output.
// bootDeepDiveClause is the boot line's "Deep-dives: ..." sentence, isolated
// as its own sentinel token (deepDiveClauseSentinel) so RenderClaudeMDFromTemplate
// can drop it cleanly under the consumer profile (which never writes
// docs/gen/thinking/ — see genSpec's `if !consumer { thinkingDocs := ... }`
// gate in cmd/hotam/gen_spec.go) instead of leaving a dangling pointer at a
// directory that was never generated.
const deepDiveClauseSentinel = " Deep-dives: `spec/docs/thinking/`."

// claudeMDHeaderSentinel is the FULL-profile file title, isolated as its own
// token so it can be swapped out wholesale for claudeMDConsumerHeaderPlaceholder
// under the consumer profile without touching any other template byte. It is
// never a legitimate substring of generated content, so a plain ReplaceAll is
// safe (mirrors deepDiveClauseSentinel's isolation pattern).
const claudeMDHeaderSentinel = "# CLAUDE.md — Hotam-Spec framework"

// claudeMDTemplate is the FULL-profile template (self_hosting domains:
// hotam-spec-self, hotam-dev, and every other non-consumer domain): fixed
// framework-identity header, then MIND (methodology) before BUSINESS
// (domain state) — preserved byte-identical to the pre-task-E2 layout.
const claudeMDTemplate = claudeMDHeaderSentinel + "\n" +
	"\n" +
	"**Hotam-Spec** — executable memory and discipline for a human + LLM-agent fleet: understand, evolve, protect, and support a shared model over time. Contradictory requirements are one of its properties — held open as tension-graph nodes, never silently discarded. License: MIT OR Apache-2.0.\n" +
	"\n" +
	"Boot: Role + Mediation-loop blocks below = operating seed." + deepDiveClauseSentinel + "\n" +
	"\n" +
	mindPlaceholder + "\n" +
	"\n" +
	businessPlaceholder + "\n" +
	"\n" +
	DurableNotesMarkerLine + "\n"

// claudeMDTemplateConsumer is the CONSUMER-profile template (external
// business domains: gpsm-sm, prat, …): the domain-identity header line
// (claudeMDHeaderSentinel, later replaced with the domain's own name by
// RenderClaudeMDFromTemplate) leads, then <!-- business --> renders BEFORE
// <!-- mind -->, and a short transition sentence introduces the methodology
// seed as the file's closing "how we work" section rather than its opener.
// Every other line — the Hotam-Spec tagline, the boot sentence, the
// trailing durable-notes marker — is preserved verbatim from the full
// template so the two profiles diverge ONLY in header text and placeholder
// order, nothing else.
const claudeMDTemplateConsumer = claudeMDHeaderSentinel + "\n" +
	"\n" +
	"**Hotam-Spec** — executable memory and discipline for a human + LLM-agent fleet: understand, evolve, protect, and support a shared model over time. Contradictory requirements are one of its properties — held open as tension-graph nodes, never silently discarded. License: MIT OR Apache-2.0.\n" +
	"\n" +
	"Boot: Role + Mediation-loop blocks below = operating seed." + deepDiveClauseSentinel + "\n" +
	"\n" +
	businessPlaceholder + "\n" +
	"\n" +
	"General Hotam-Spec discipline applied below:\n" +
	"\n" +
	mindPlaceholder + "\n" +
	"\n" +
	DurableNotesMarkerLine + "\n"

const generatedHeaderComment = "<!-- (generated by `hotam gen-spec` — do not hand-edit) -->"

// DurableNotesMarkerLine is the literal marker line every CLAUDE.md template
// (claudeMDTemplate, claudeMDTemplateConsumer) ends with: everything from the
// start of the file THROUGH this line (inclusive) is generator-owned and
// regenerated verbatim on every `hotam gen-spec` run; everything AFTER it is
// meant to be the operator's own durable-notes tail, preserved untouched
// across regenerations per the template's own promise ("Anything you write
// below this line survives every regeneration verbatim") — see
// cmd/hotam/gen_spec.go's genSpec, which reads the PRE-EXISTING file at
// claudeMDPath (if any) via SplitAtDurableNotesMarker and re-appends its tail
// to every fresh render before writing, so the promise is actually honored
// (prior to this being wired up, genSpec always overwrote the file wholesale,
// silently discarding any tail an operator had written — a real gap the
// template's own promise was not actually keeping). Exported (rather than a
// private literal duplicated at each of the two template definitions AND
// wherever a caller needs to locate the boundary — e.g. cmd/hotam/gen_spec.go
// at the write site, and internal/invariants/claude_md_current.go's real
// implementation, wired in from cmd/hotam, at the check site) so every reader
// of this boundary visibly agrees on one literal, the same discipline
// specMDRelPath/vendoredRecorderRelPath already establish for their own
// write-side/check-side path agreement.
const DurableNotesMarkerLine = "<!-- Anything you write below this line survives every regeneration verbatim. Use this space for durable notes, reminders, or context that the generator should never touch. -->"

// SplitAtDurableNotesMarker splits content at the FIRST line equal to
// DurableNotesMarkerLine (exact match after trimming only the line's own
// trailing \r, so a Windows-checked-out file with CRLF line endings still
// matches): generatedPart is everything up to and including that marker line
// (with its own trailing newline, if the source had one); tail is everything
// strictly after it, verbatim, including its own leading newline if present.
// ok reports whether the marker was found at all — when false, generatedPart
// is the whole of content and tail is "" (the caller decides what that means:
// genSpec's write side treats "not found" as "no tail to preserve, nothing to
// carry forward"; the check side treats it as "not a template-shaped file,"
// see claude_md_current.go for how the real implementation uses this).
func SplitAtDurableNotesMarker(content string) (generatedPart, tail string, ok bool) {
	idx := strings.Index(content, DurableNotesMarkerLine)
	if idx < 0 {
		return content, "", false
	}
	end := idx + len(DurableNotesMarkerLine)
	// Include the marker line's own trailing newline (LF or CRLF) in
	// generatedPart, not tail, so a file with NO authored tail at all
	// (generatedPart == the whole file, tail == "") round-trips exactly —
	// the common case today (see this const's own doc comment: no live
	// CLAUDE.md in this repo currently carries a tail).
	if end < len(content) && content[end] == '\r' {
		end++
	}
	if end < len(content) && content[end] == '\n' {
		end++
	}
	return content[:end], content[end:], true
}

// RenderOperatorRoleBlock renders the OPERATOR-ROLE block content (without
// sentinels): the resident seed's identity, parametrized by the active
// domain's name and its SETTLED requirement count.
func RenderOperatorRoleBlock(g *ontology.Graph, scopeLabel string) string {
	if scopeLabel == "" {
		scopeLabel = "(no domain yet)"
	}
	atomCount := 0
	for _, r := range g.Requirements {
		if r.Status == ontology.StatusSETTLED {
			atomCount++
		}
	}
	lines := []string{
		generatedHeaderComment,
		"",
		"### Role (the resident seed)",
		"",
		fmt.Sprintf(
			"Operator of `%s` (%d SETTLED). Guardian: **spec** (`domains/%s/graph.json`) ↔ **tests** (`check_*`/`Test_*`) ↔ **business** (resolver decisions). Drift between layers = top signal.",
			scopeLabel, atomCount, scopeLabel,
		),
		"",
		"**Default register (R-speak-domain-register-by-default):** speak in the language of the LAYER you are working in, and AVOID a higher layer's language when the current layer's own terms will do. Layers, bottom-up: (1) the active consumer domain's own language (its concepts, names, stages, roles); (2) the methodology-constitution's own terms; (3) the Hotam engine's internals (the graph, `R-…`/`C-…`/`A-…`/`OP-…` anchors, `SETTLED`/`ENFORCED`/`DRAFT` statuses, `check_*` invariants, `hotam` commands). Escalate to a higher layer ONLY when (a) the human explicitly asks to switch to it, or (b) the human's own message already uses that higher layer's terms — otherwise stay at the current layer, and lead with its essence, not the framework's name. (Inside the mediation loop's TRANSLATE/PRESENT/LAND steps anchor-citation stays mandatory — R-speak-by-reference.)",
		"",
		"Confront every input against graph reality BEFORE writing. Cite anchors (`R-…`/`C-…`/`A-…`/`OP-…`), never vibes (R-speak-by-reference). Present, never decide — resolver decides; never close a Conflict silently (R-ai-presents-not-decides, R-decided-needs-human-signoff).",
		"",
		"**Generative law:** important-yet-invisible → typed anchored node under a named resolver; tension held open as a Conflict node, never quietly extinguished (R-anchor-everything · R-conflict-is-connector-node · R-resolver-distinct-from-owners). Every RULE below is a projection of this law.",
	}
	return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n")
}

// RenderMediationLoopBlock renders the MEDIATION-LOOP block content (without
// sentinels): a static six-step input-processing loop, no graph/filesystem
// dependency.
func RenderMediationLoopBlock() string {
	return mediationLoopText
}

// RenderEmbeddedThinkingBlock renders the EMBEDDED-THINKING block content
// (without sentinels): one RULE line per §-section, each sourcing its rule
// text from Section.Canon (internal/methodology/sections_data.go) — the same
// registry BuildThinkingDocs reads from, so Canon is the single source of
// truth for both the inlined RULE and the full thinking-doc on disk.
//
// R-operator-crystal-embeds-thinking-distilled demands exactly this: a real
// RULE sentence per topic embedded inline (the operator sees the gist without
// fetching), plus a path link to the full Canon/Narrative/Why at
// domains/<domainName>/docs/gen/thinking/<slug>.md. The old version rendered
// only a bare slug-list ("§Agent · §Assumption · ...") whose "One RULE per
// §-section" label was therefore false — zero rule text appeared. Sourcing
// from Canon makes the label literally true and adds no new source of truth
// (the thinking/*.md docs are themselves generated FROM this same registry's
// Canon/Narrative/Why via BuildThinkingDocs).
//
// Each Canon is guarded through shortForm(canon, "") (firstWholeSentence) as a
// defensive measure against future Canon drift into multi-sentence text: the
// current Canon values are already single standalone sentences, so this is
// pure future-proofing, not truncation of anything today. It degrades
// gracefully (whole-sentence short form, never mid-word "..." stub,
// R-crystal-carries-short-form) if a future Canon value grows verbose.
//
// consumer gates the "full Canon/Narrative/Why at
// domains/<domainName>/docs/gen/thinking/<slug>.md" clause: genSpec never
// writes docs/gen/thinking/*.md under the consumer profile (`if !consumer {
// thinkingDocs := ... }` in cmd/hotam/gen_spec.go), so pointing at that path
// for a consumer-profile domain would be a broken link. When consumer is
// true the intro line ends cleanly after "per §-section" instead.
func RenderEmbeddedThinkingBlock(domainName string, consumer bool) string {
	if domainName == "" {
		domainName = "hotam-spec-self"
	}
	sections := methodology.Sections.All()
	intro := "One RULE per §-section."
	if !consumer {
		intro = "One RULE per §-section; full Canon/Narrative/Why at `domains/" + domainName + "/docs/gen/thinking/<slug>.md` (slug = lowercased §-name)."
	}
	lines := []string{
		generatedHeaderComment,
		"",
		"### Methodology — how to think",
		"",
		intro,
		"",
	}
	for _, s := range sections {
		rule := shortForm(s.Canon, "")
		lines = append(lines, "- **"+s.Slug+"** — "+rule)
	}
	return strings.Join(lines, "\n")
}

// RenderEmbeddedToolsBlock renders the EMBEDDED-TOOLS block content
// (without sentinels). It is a compact pointer-only reference: it reports the
// Implemented and Planned tool counts (computed from methodology.Tools) and
// directs the operator to `hotam -h` (the full command list with usage,
// generated from the same registry), `hotam status --json` (structured pulse),
// `hotam req` / `hotam brief` (agentic access), and docs/gen/tools/INDEX.md /
// internal/methodology/tools_data.go (the complete registry) for detail.
//
// consumer drops the internal/methodology/tools_data.go source-file reference
// (a framework SOURCE FILE that does not exist in an external consumer's
// project), keeping the docs/gen/tools/INDEX.md pointer (a real file in both
// profiles). Same pattern as BuildToolDocsIndex's consumer branch (task #144).
func RenderEmbeddedToolsBlock(consumer bool) string {
	tools := methodology.Tools.All()
	implementedCount := 0
	plannedCount := 0
	for _, t := range tools {
		if t.Status == methodology.Implemented {
			implementedCount++
		} else {
			plannedCount++
		}
	}

	registryRef := "spec/docs/gen/tools/INDEX.md, internal/methodology/tools_data.go."
	if consumer {
		registryRef = "spec/docs/gen/tools/INDEX.md."
	}

	lines := []string{
		generatedHeaderComment,
		"",
		fmt.Sprintf(
			"### Tool reference — %d Implemented, %d Planned. Full list with usage: `hotam -h`. Structured pulse: `hotam status --json`. Agentic access: `hotam req` / `hotam brief`. Complete registry: "+registryRef,
			implementedCount, plannedCount,
		),
	}
	return strings.Join(lines, "\n")
}

// RenderOperatorRecursionBlock renders the OPERATOR-RECURSION block content
// (without sentinels): sub-operator spawning as a capability, parametrized
// by the active domain name (used in the spawn-path example command).
func RenderOperatorRecursionBlock(domainName string) string {
	if domainName == "" {
		domainName = "hotam-spec-self"
	}
	return strings.Replace(operatorRecursionTemplate, "domains/hotam-spec-self/agents", "domains/"+domainName+"/agents", 1)
}

// RenderMindContent renders the MIND bucket: the domain-agnostic "how to
// think" layer. Order: OPERATOR-ROLE, MEDIATION-LOOP, EMBEDDED-THINKING,
// EMBEDDED-TOOLS, OPERATOR-RECURSION.
//
// consumer is threaded through to RenderEmbeddedThinkingBlock and
// RenderEmbeddedToolsBlock — the other three blocks carry no consumer-gated
// content.
func RenderMindContent(g *ontology.Graph, domainName string, consumer bool) string {
	parts := []string{
		WrapBlock("OPERATOR-ROLE", RenderOperatorRoleBlock(g, domainName)),
		WrapBlock("MEDIATION-LOOP", RenderMediationLoopBlock()),
		WrapBlock("EMBEDDED-THINKING", RenderEmbeddedThinkingBlock(domainName, consumer)),
		WrapBlock("EMBEDDED-TOOLS", RenderEmbeddedToolsBlock(consumer)),
		WrapBlock("OPERATOR-RECURSION", RenderOperatorRecursionBlock(domainName)),
	}
	return strings.Join(parts, "\n")
}

// shortForm renders a meaningful short form of text per
// R-crystal-carries-short-form: it prefers an explicit summary when one is
// provided, otherwise falls back to the first whole sentence of text. It NEVER
// mechanically truncates mid-word with an ellipsis — the resolver verdict
// ("All that gets truncated must not be truncated but have a short version")
// declares mid-word stubs an illusion of knowledge. A sentence boundary is a
// '.', '!', or '?' followed by whitespace or end-of-string; with no boundary
// the whole trimmed text is returned.
func shortForm(text, summary string) string {
	if s := strings.TrimSpace(summary); s != "" {
		return s
	}
	return firstWholeSentence(text)
}

// firstWholeSentence returns the leading sentence of text: everything up to
// and including the first sentence terminator ('.', '!', '?') that is
// followed by whitespace or the end of the string. When no such boundary
// exists the whole trimmed text is returned unchanged. This replaces the
// mechanical rune-count truncation (mid-word "..." stubs) that
// R-crystal-carries-short-form prohibits.
func firstWholeSentence(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		switch runes[i] {
		case '.', '!', '?':
			if i == len(runes)-1 {
				return text
			}
			if next := runes[i+1]; next == ' ' || next == '\t' || next == '\n' || next == '\r' {
				return string(runes[:i+1])
			}
		}
	}
	return text
}

// summaryForTarget returns the Summary of the Requirement whose ID equals
// target, if one exists in g; otherwise "" (so the caller falls back to the
// first-whole-sentence short form of the message). A Signal.Target is often a
// reflection axis or composite label rather than a Requirement ID, so a miss
// is the normal path and summary-priority applies only when the target
// resolves to a real Requirement carrying a non-empty Summary.
func summaryForTarget(g *ontology.Graph, target string) string {
	if target == "" {
		return ""
	}
	for _, r := range g.Requirements {
		if r.ID == target {
			return r.Summary
		}
	}
	return ""
}

// ViolationsOverride carries an already-computed invariants.AllViolations
// result for a SPECIFIC graph (For), to be substituted wherever that exact
// graph's own violations would otherwise be recomputed from scratch during a
// CLAUDE.md render. See domainPulse's doc comment for the full rationale —
// this exists solely so check_domain_claude_md_current
// (internal/invariants/claude_md_current.go) can render a byte-comparable
// fresh copy of a domain's crystal WITHOUT calling back into
// invariants.AllViolations for the graph it is itself currently being
// invoked from inside of (unbounded same-process recursion otherwise).
//
// For is compared by pointer identity against each candidate graph
// (RenderDomainMapBlock's domainGraphs values, RenderClaudeMDFromTemplate's
// own g) — callers that pre-populate domainGraphs with the SAME *ontology.Graph
// pointer as g (every existing genSpec call site already does exactly this,
// see cmd/hotam/gen_spec.go's `domainGraphs := map[string]*ontology.Graph{domainName: g}`)
// get the override applied to that one entry; every OTHER domain in the map
// (a different pointer) computes its own violations fresh, which is safe —
// it is a different graph, not a recursive call on g.
type ViolationsOverride struct {
	For        *ontology.Graph
	Violations []invariants.Violation
}

// forGraph returns v.Violations wrapped as a *ViolationsOverride scoped to
// dg, but ONLY when v is non-nil and v.For == dg (pointer identity); in
// every other case (v is nil, or dg is a different graph than the one the
// override was computed for) it returns nil, which downstream (domainPulse)
// means "compute fresh" — the safe default.
func (v *ViolationsOverride) forGraph(dg *ontology.Graph) *ViolationsOverride {
	if v == nil || dg == nil || v.For != dg {
		return nil
	}
	return v
}

// domainPulse mirrors _domain_pulse: a domain's open-action count plus a
// one-line rendering of its top-priority action, via the same
// DiagnoseSignals used for the root LIVE-STATE cipher. Returns (0, "") on a
// clean graph. The top action's message is rendered via shortForm
// (R-crystal-carries-short-form): summary-priority when the target resolves
// to a Requirement carrying a Summary, else the first whole sentence — never
// mechanical mid-word truncation.
//
// violations, when non-nil (a real pointer, even one wrapping a nil/empty
// slice — a nil *ViolationsOverride is the "not supplied" state, distinct
// from a supplied-but-empty result), is used INSTEAD of computing
// invariants.AllViolations(dg) internally — see
// diagnose.DiagnoseSignalsWithViolations's doc comment for why
// (check_domain_claude_md_current, internal/invariants/claude_md_current.go,
// must not re-enter invariants.AllViolations for the SAME graph it is
// currently being invoked from inside of). A plain []invariants.Violation
// parameter would be ambiguous here — invariants.AllViolations legitimately
// returns a nil slice for a clean graph (internal/invariants/
// all_violations.go's own "var out []Violation; return out"), so "nil"
// cannot double as "not supplied" — hence the pointer wrapper. Every other
// caller passes nil (the pointer itself), preserving the prior "compute
// fresh" behavior unchanged — this matters here specifically because
// RenderDomainMapBlock calls domainPulse once per domain under domains/,
// and the domain currently being rendered (g) is typically ALSO one of
// those entries (a domain's own CLAUDE.md lists itself in its Domain Map),
// so a caller rendering g's crystal must supply g's own precomputed
// violations for THAT entry while every sibling domain safely computes its
// own fresh (a different graph, not a recursive call).
//
// The "compute fresh" fallback (violations == nil) deliberately calls
// diagnose.DiagnoseSignalsExcludingDiskProjection, NOT plain
// diagnose.DiagnoseSignals — see AllViolationsExcludingDiskProjection's own
// doc comment (internal/invariants/all_violations.go) for the real,
// observed mutual-recursion bug this avoids: a sibling domain's fresh
// violation scan must never include check_spec_md_current/
// check_domain_claude_md_current, because either of those checks rendering
// ITS OWN crystal would in turn compute ITS siblings' pulses (including the
// domain currently being rendered here, reloaded from disk as a distinct
// graph pointer), and so on without bound. A DOMAIN-MAP sibling pulse line
// has never needed that specific signal anyway (it is a byte-comparison
// staleness check against a generated file, not a "top action" headline).
func domainPulse(dg *ontology.Graph, today string, violations *ViolationsOverride) (int, string) {
	var signals []diagnose.Signal
	if violations != nil {
		signals = diagnose.DiagnoseSignalsWithViolations(dg, today, violations.Violations)
	} else {
		signals = diagnose.DiagnoseSignalsExcludingDiskProjection(dg, today)
	}
	if len(signals) == 0 {
		return 0, ""
	}
	top := signals[0]
	imperative := strings.Join(strings.Fields(top.Message), " ")
	imperative = shortForm(imperative, summaryForTarget(dg, top.Target))
	return len(signals), fmt.Sprintf("[P%d] %s: %s", top.Priority, top.Target, imperative)
}

// RenderDomainMapBlock renders the DOMAIN-MAP block content (without
// sentinels): every domains/<name>/ directory (sorted, excluding those
// starting with "_"), its manifest presentation fields, SETTLED atom count,
// and open-action pulse (R-domain-map-shows-pulse).
//
// repoRoot is the repository root (the parent of domains/); domainGraphs
// optionally pre-supplies already-loaded graphs keyed by domain directory
// name (dedup with the caller's own load), falling back to loading
// domains/<name>/graph.json from disk when absent.
//
// This is a thin wrapper over renderDomainMapBlockWithViolations passing a
// nil override and an empty selfCrystalPath (every domain, including the
// active one if present among domainGraphs, computes its own violations
// fresh, and every domain's crystal-link line is determined by a real
// os.Stat — no self-referential write-in-progress to special-case) — the
// right default for every existing caller. See ViolationsOverride's doc
// comment for the one caller (check_domain_claude_md_current) that must
// supply a real override, and the SELF-ENTRY SPECIAL CASE comment inside
// this function's body for the one caller (genSpec's own crystal write)
// that must supply a real selfCrystalPath.
func RenderDomainMapBlock(repoRoot string, domainGraphs map[string]*ontology.Graph, today string) string {
	return renderDomainMapBlockWithViolations(repoRoot, domainGraphs, today, nil, "")
}

// renderDomainMapBlockWithViolations is RenderDomainMapBlock's core,
// additionally threading violations (when non-nil) to domainPulse via
// forGraph for whichever entry's graph pointer matches violations.For — see
// ViolationsOverride's doc comment — and selfCrystalPath (when non-empty)
// for the SELF-ENTRY SPECIAL CASE below (a real, found-in-practice
// time-of-check/time-of-write hazard around os.Stat on the crystal path
// currently being generated).
func renderDomainMapBlockWithViolations(repoRoot string, domainGraphs map[string]*ontology.Graph, today string, violations *ViolationsOverride, selfCrystalPath string) string {
	lines := []string{
		generatedHeaderComment,
		"",
		"### Domain Map",
		"",
	}

	domainsRoot := filepath.Join(repoRoot, "domains")
	entries, err := os.ReadDir(domainsRoot)
	if err != nil {
		lines = append(lines, "_(no domains yet — domains/ directory absent)_")
		return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n")
	}

	var domainDirs []string
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), "_") {
			continue
		}
		domainDirs = append(domainDirs, e.Name())
	}
	sort.Strings(domainDirs)

	if len(domainDirs) == 0 {
		lines = append(lines, "_(no domains yet)_")
		return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n")
	}

	for _, name := range domainDirs {
		// Presentation fields come from the domain's OWN manifest.json
		// (purpose/goals/director — task #210): a domain is a self-contained
		// directory, so its Domain Map identity lives on disk with it, never
		// in an engine-side table. A manifest without these fields (or a
		// missing/malformed manifest) yields empty values → em-dash
		// placeholders below, same rendering as before the fields existed.
		m := loader.ResolveDomainPresentation(filepath.Join(domainsRoot, name, "graph.json"))
		domainID := name
		description := m.Purpose
		goalsText := ""
		if len(m.Goals) > 0 {
			goalsText = strings.Join(m.Goals, ", ")
		}
		director := m.Director
		if description == "" {
			description = "—"
		}
		if goalsText == "" {
			goalsText = "—"
		}
		if director == "" {
			director = "—"
		}

		atomsCount := 0
		openActions := 0
		topActionLine := ""

		dg := domainGraphs[name]
		if dg == nil {
			gp := filepath.Join(domainsRoot, name, "graph.json")
			if loaded, err := loader.LoadGraph(gp); err == nil {
				dg = loaded
			}
		}
		if dg != nil {
			for _, r := range dg.Requirements {
				if r.Status == ontology.StatusSETTLED {
					atomsCount++
				}
			}
			openActions, topActionLine = domainPulse(dg, today, violations.forGraph(dg))
		}

		lines = append(lines, "### "+domainID)
		lines = append(lines, "- **purpose** — "+description)
		lines = append(lines, "- **goals** — "+goalsText)
		lines = append(lines, "- **director** — "+director)
		lines = append(lines, "- **path** — `domains/"+name+"/`")
		// Crystal link (task A2 router): when this domain carries a LOCAL
		// crystal at <domainDir>/CLAUDE.md on disk, surface a direct pointer
		// to it so the root crystal (the active domain's boot file) acts as a
		// router to its consumer-domain siblings. The active domain itself —
		// whose crystal lives at the repo ROOT, not under domains/ — has no
		// local file and so gets no link (exactly the "router to the OTHER
		// domains" shape the task asks for). A domain that has never been
		// generated gets no link either: honest "no committed file = no lie",
		// the same opt-in shape check_spec_md_current/check_recorder_current
		// use, so the router is self-populating as each domain is generated
		// and the root re-rendered.
		//
		// SELF-ENTRY SPECIAL CASE (bug found while implementing
		// check_domain_claude_md_current, E4): crystalPath is THIS entry's
		// candidate local-crystal path; selfCrystalPath (when non-empty) is
		// the ACTUAL path the crystal currently being rendered will be
		// written to (root OR local, whichever resolveClaudeMDPath decided —
		// the caller threads this through unchanged, this function makes no
		// root/local judgment of its own). When they match, the line is
		// forced present WITHOUT consulting the filesystem at all. os.Stat
		// here is otherwise a real side-effecting read of live filesystem
		// state at render time -- fine for every domain whose crystal this
		// render does NOT control, but for the domain THIS render IS
		// producing, it is a time-of-check/time-of-write hazard: the very
		// first time a domain's LOCAL crystal is generated,
		// <domainDir>/CLAUDE.md does not exist yet at the moment this render
		// runs (genSpec computes the render BEFORE writing it to disk) --
		// os.Stat fails, the line is omitted -- but the file DOES exist by
		// the time anything re-renders the SAME graph afterward (e.g.
		// check_domain_claude_md_current's own byte comparison, running
		// moments later), so the line appears the SECOND time, producing two
		// genuinely different byte sequences for what should be an
		// identical, deterministic render of the same graph (observed
		// directly: a captured byte-diff showed the committed CLAUDE.md
		// missing this exact line, differing by ~43 bytes from a fresh
		// re-render that found the just-written file). This entry's truth
		// value is a TAUTOLOGY, not a filesystem fact, precisely when
		// crystalPath == selfCrystalPath: whenever a domain's own crystal is
		// being rendered to THAT path at all, the crystal IS (or is about to
		// become) real content there by definition. The ACTIVE (root-crystal)
		// domain is unaffected either way -- its candidate crystalPath here
		// is always domains/<name>/CLAUDE.md, a path the active domain never
		// writes to (its crystal lives at repoRoot instead), so it can never
		// equal selfCrystalPath and always falls through to the real
		// (permanently-false, not racy) os.Stat check.
		crystalPath := filepath.Join(domainsRoot, name, "CLAUDE.md")
		if crystalPath == selfCrystalPath {
			lines = append(lines, "- **crystal** — `domains/"+name+"/CLAUDE.md`")
		} else if _, err := os.Stat(crystalPath); err == nil {
			lines = append(lines, "- **crystal** — `domains/"+name+"/CLAUDE.md`")
		}
		lines = append(lines, fmt.Sprintf("- **atoms-count** — %d SETTLED", atomsCount))
		if openActions > 0 {
			lines = append(lines, fmt.Sprintf("- **open actions** — %d (top: %s)", openActions, topActionLine))
		} else {
			lines = append(lines, "- **open actions** — 0 (graph clean)")
		}
		lines = append(lines, "")
	}

	return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n")
}

// RenderAgentMapBlock renders the AGENT-MAP block content (without
// sentinels). Static: no spec/agents/<name>/ directories exist
// (R-claude-md-consolidates-when-single-agent — sub-agent crystals
// materialize only at real spawn time), so the placeholder is the only
// reachable output; baked in claudemd_static.go alongside the other
// no-filesystem-equivalent blocks.
func RenderAgentMapBlock() string {
	return agentMapText
}

// RenderConceptMapBlock renders the CONCEPT-MAP block content (without
// sentinels). Static: the concept map projects glossary §-section terms
// against the framework's source tree, with no live AST scan wired up (see
// claudemd_static.go doc comment); the function carries no graph dependency.
//
// consumer is accepted for API symmetry; the consumer-profile omission (the
// entire CONCEPT-MAP block maps framework SOURCE FILE paths like
// internal/ontology/*.go that do not exist in an external consumer's project)
// is gated at the call site in RenderBusinessContent, which skips the
// WrapBlock entirely under consumer.
func RenderConceptMapBlock(consumer bool) string {
	return conceptMapText
}

// recentlyRejectedCap caps the CLAUDE.md RECENTLY-REJECTED block at a small
// taste of the anti-relitigation list (P2-1 compaction — the full list with
// full summaries lives in docs/gen/HISTORY.md, not duplicated into every
// regeneration of the root crystal).
const recentlyRejectedCap = 3

// rejectedReplacesRE matches the "REJECTED <dash> REPLACES" marker across
// em-dash, en-dash, double-dash, and single-hyphen variants.
var rejectedReplacesRE = regexp.MustCompile(`REJECTED\s*(?:—|–|--|-)\s*REPLACES`)

func hasRejectedReplacesMarker(why string) bool {
	return rejectedReplacesRE.MatchString(why)
}

// replacesMap mirrors graph.replaces_map: REJECTED-id -> ordered tuple of
// successor (carrier) ids, built from every Requirement's "replaces"
// relations, in graph declaration order (NOT raw JSON array order). The
// Go loader preserves source-file textual declaration order only via
// DeclOrder (see NarrativeOrder, the same pattern every other
// order-sensitive builder in this package uses).
func replacesMap(g *ontology.Graph) map[string][]string {
	out := make(map[string][]string)
	for _, r := range NarrativeOrder(g.Requirements, func(r ontology.Requirement) int { return r.DeclOrder }) {
		for _, rel := range r.Relations {
			if rel.Kind == "replaces" {
				out[rel.Target] = append(out[rel.Target], r.ID)
			}
		}
	}
	return out
}

// RenderRecentlyRejectedBlock renders the RECENTLY-REJECTED block content
// (without sentinels): REJECTED requirements with a known replacement
// (structural `replaces` edge OR legacy prose "REJECTED — REPLACES"
// marker), capped at recentlyRejectedCap entries (a small taste, P2-1
// compaction) in alphabetical-by-id order, with a pointer to the full
// history (docs/gen/HISTORY.md) beyond the cap.
func RenderRecentlyRejectedBlock(g *ontology.Graph) string {
	lines := []string{
		generatedHeaderComment,
		"",
		"### Recently rejected (anti-relitigation — summary)",
		"",
		"Before proposing an architectural change, check this isn't already REJECTED with a REPLACES successor — cite the replacement instead of re-deriving. Full list: `spec/docs/gen/HISTORY.md`.",
		"",
	}

	rmap := replacesMap(g)

	var rejected []ontology.Requirement
	for _, r := range g.Requirements {
		if r.Status != ontology.StatusREJECTED {
			continue
		}
		_, hasEdge := rmap[r.ID]
		if hasEdge || hasRejectedReplacesMarker(r.Why) {
			rejected = append(rejected, r)
		}
	}
	sort.Slice(rejected, func(i, j int) bool { return rejected[i].ID < rejected[j].ID })

	if len(rejected) == 0 {
		lines = append(lines, "_(no anti-relitigation entries — nothing recently rejected.)_")
		return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n")
	}

	total := len(rejected)
	shown := rejected
	if len(shown) > recentlyRejectedCap {
		shown = shown[:recentlyRejectedCap]
	}

	// P2-1 compaction: id-only (no WHY-derived summary text — that detail
	// lives in docs/gen/HISTORY.md, `hotam req show <id>`, and
	// AGENT-CONTEXT.md's pointer to it) to keep this root-crystal block
	// small regardless of how verbose an individual rejection's WHY prose
	// is.
	ids := make([]string, len(shown))
	for i, r := range shown {
		ids[i] = r.ID
	}
	lines = append(lines, fmt.Sprintf("**REJECTED (REPLACES known)** (%d) — %s", total, strings.Join(ids, " · ")))

	if total > recentlyRejectedCap {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf(
			"_(showing %d of %d, alphabetical by id — full history + WHY: `spec/docs/gen/HISTORY.md`, `hotam req show <id>`)_",
			len(shown), total,
		))
	}

	return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n")
}

// runeTruncateEllipsis truncates s to keep runes and appends the single
// Unicode ellipsis character "…".
func runeTruncateEllipsis(s string, keep int) string {
	r := []rune(s)
	if len(r) <= keep {
		return s
	}
	return strings.TrimRight(string(r[:keep]), " \t\r\n") + "…"
}

// RenderParentProjectBlock renders the PARENT-PROJECT block content (without
// sentinels): the active domain's place in the sub-project hierarchy, read
// from the manifest.json "parent" field resolved by loader.ResolveParent at
// LoadGraph time and surfaced on the loaded graph as g.ParentDeclared /
// g.Parent (PLAN-scenario-generated-spec.md §2 D6, task W6.1).
//
// This is a single-domain block (it renders the ACTIVE domain's parent, not a
// map over all domains) like LIVE-STATE / CONSTITUTION, NOT like DOMAIN-MAP
// (which walks every domains/<name>/ directory). It sits in the BUSINESS
// bucket directly after DOMAIN-MAP because, like DOMAIN-MAP, it is a
// manifest-derived section (purpose/goals/director/parent all live in
// manifest.json, not graph.json).
//
// The three manifest tri-states render as:
//
//   - g.ParentDeclared == false: the "parent" key is absent from manifest.json.
//     In a live domain this state is already firing a
//     check_project_parent_declared violation (so this branch should not be
//     the steady-state output for any real domain), but the renderer renders
//     an HONEST placeholder rather than empty output if it is reached, naming
//     the invariant so a reader of the crystal is pointed at the actionable
//     fix rather than seeing a blank section.
//   - g.ParentDeclared == true, g.Parent == "": JSON null -- the explicit
//     root-domain declaration. Renders "This is a root domain (no parent).".
//   - g.ParentDeclared == true, g.Parent != "": a non-empty string -- the
//     child-domain declaration. Renders "Parent: `<name>`.".
//
// g is expected to be the active domain's loaded graph (loader.LoadGraph
// populates ParentDeclared/Parent); a synthetic in-memory graph leaves both
// at the zero value (ParentDeclared=false, Parent="") and renders the
// placeholder branch -- harmless for the smoke/byte-identity fixtures, which
// do not assert this block's content.
func RenderParentProjectBlock(g *ontology.Graph) string {
	line := "### Parent project"
	var body string
	switch {
	case g.ParentDeclared && g.Parent != "":
		body = "Parent: `" + g.Parent + "`."
	case g.ParentDeclared:
		body = "This is a root domain (no parent)."
	default:
		body = "_(parent not yet declared — see check_project_parent_declared)_"
	}
	return strings.Join([]string{generatedHeaderComment, "", line, "", body}, "\n")
}

// RenderProjectEssenceBlock renders the PROJECT-ESSENCE block content
// (without sentinels): a compact purpose/goals/director summary for the
// CURRENT (active) domain only — the same three manifest fields
// RenderDomainMapBlock projects PER domain across the whole domains/ tree,
// narrowed here to just the active domain so a consumer-profile crystal
// opens with "what this project is" on the first screen.
//
// Source of truth: the active domain's own manifest.json, resolved via
// loader.ResolveDomainPresentation (the same loader already used by
// RenderDomainMapBlock for every sibling domain) — never a new data
// source. A manifest without these fields (or a missing/malformed
// manifest) yields em-dash placeholders, mirroring RenderDomainMapBlock's
// fallback shape exactly.
//
// repoRoot is the repository root (the parent of domains/); domainName is
// the active domain's directory name. Together they locate the manifest at
// domains/<domainName>/manifest.json — the same path ResolveDomainPresentation
// derives from its graphPath argument.
func RenderProjectEssenceBlock(repoRoot, domainName string) string {
	description := "—"
	goalsText := "—"
	director := "—"
	charter := ""
	if domainName != "" {
		m := loader.ResolveDomainPresentation(filepath.Join(repoRoot, "domains", domainName, "graph.json"))
		if m.Purpose != "" {
			description = m.Purpose
		}
		if len(m.Goals) > 0 {
			goalsText = strings.Join(m.Goals, ", ")
		}
		if m.Director != "" {
			director = m.Director
		}
		charter = m.Charter
	}
	lines := []string{
		generatedHeaderComment,
		"",
		"### Project essence",
		"",
		"- **purpose** — " + description,
	}
	if charter != "" {
		lines = append(lines, "- **charter** — "+charter)
	}
	lines = append(lines,
		"- **goals** — "+goalsText,
		"- **director** — "+director,
	)
	return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n")
}

// RenderStakeholdersBlock renders the STAKEHOLDERS block content (without
// sentinels): a compact who's-who table over the active domain's
// g.Stakeholders (id/name/domain), so a consumer-profile crystal opens
// with the project's roles on the first screen. Reuses the same graph
// slice BuildRequirements already projects into REQUIREMENTS.md's
// Stakeholders section (NarrativeOrder by DeclOrder, Cell for safe table
// interpolation) — no new data source.
//
// A graph with no Stakeholders renders an explicit empty marker (never
// nothing) so the sentinel pair always has honest inner content, matching
// AGENT-MAP / CONSTITUTION's empty-state contract.
func RenderStakeholdersBlock(g *ontology.Graph) string {
	lines := []string{
		generatedHeaderComment,
		"",
		"### Stakeholders & roles",
		"",
	}
	stakeholders := NarrativeOrder(g.Stakeholders, func(s ontology.Stakeholder) int { return s.DeclOrder })
	if len(stakeholders) == 0 {
		lines = append(lines, "_(no stakeholders declared in this domain yet.)_")
		return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n")
	}
	lines = append(lines, "| id | name | domain |")
	lines = append(lines, "|---|---|---|")
	for _, s := range stakeholders {
		lines = append(lines, "| `"+s.ID+"` | "+Cell(s.Name)+" | "+Cell(s.Domain)+" |")
	}
	return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n")
}

// RenderBusinessContent renders the BUSINESS bucket: the domain-specific
// "what this claims" layer. The block ORDER is profile-dependent:
//
//   - consumer == false (full profile, the default, backward-compatible
//     case): LIVE-STATE, DOMAIN-MAP, PARENT-PROJECT, CONSTITUTION,
//     AGENT-MAP, CONCEPT-MAP, RECENTLY-REJECTED — byte-identical to the
//     pre-consumer-profile-reorder output. The full profile targets the
//     engine's own self-hosting domains (hotam-spec-self / hotam-dev),
//     where the operational order (engine state first) is the one the
//     framework's own developers want.
//   - consumer == true (consumer profile, for external business domains
//     like gsm/prat): PROJECT-ESSENCE, STAKEHOLDERS, LIVE-STATE,
//     CONSTITUTION, DOMAIN-MAP, PARENT-PROJECT, AGENT-MAP,
//     RECENTLY-REJECTED. Opens with the project's essence (purpose/roles/
//     where-we-are/requirements-map) so a freshly-booted operator answers
//     "what is this project" and "what is blocked" from the first screen
//     of the crystal alone — the operational router (DOMAIN-MAP),
//     PARENT-PROJECT, AGENT-MAP and anti-relitigation history move below
//     the essence layer. CONCEPT-MAP stays omitted (it maps §-sections to
//     framework source-file paths that do not exist in an external
//     consumer's project — same gate as before, just stated here in the
//     order doc).
//
// consumer is threaded from cmd/hotam/gen_spec.go's profile resolution
// (R-gen-spec-profile) and propagates the same way through every
// consumer-aware renderer in this package.
//
// This is a thin wrapper over renderBusinessContentWithViolations passing a
// nil override and an empty selfCrystalPath — see ViolationsOverride's doc
// comment.
func RenderBusinessContent(g *ontology.Graph, domainName, repoRoot string, claudeMDCharCount int, domainGraphs map[string]*ontology.Graph, today string, consumer bool) string {
	return renderBusinessContentWithViolations(g, domainName, repoRoot, claudeMDCharCount, domainGraphs, today, consumer, nil, "")
}

// renderBusinessContentWithViolations is RenderBusinessContent's core,
// additionally threading violations (when non-nil) to BuildLiveState/
// RenderDomainMapBlock's violations-aware variants — see
// ViolationsOverride's doc comment. violations.For is expected to be g
// itself (the active domain this whole render is for) — LIVE-STATE always
// reports g's own pulse, never a sibling's, so the override applies
// unconditionally here (unlike domainPulse's forGraph, which must pick the
// matching entry out of several).
//
// selfCrystalPath (when non-empty) is threaded to
// renderDomainMapBlockWithViolations for its SELF-ENTRY SPECIAL CASE — see
// that function's own doc comment for the real, found-in-practice
// time-of-check/time-of-write os.Stat hazard this exists to avoid.
func renderBusinessContentWithViolations(g *ontology.Graph, domainName, repoRoot string, claudeMDCharCount int, domainGraphs map[string]*ontology.Graph, today string, consumer bool, violations *ViolationsOverride, selfCrystalPath string) string {
	// CRITICAL: branch FIRST, never call the plain (non-violations-aware)
	// BuildLiveState/RenderDomainMapBlock when violations != nil. An earlier
	// version of this function computed BOTH unconditionally ("compute a
	// default, then override") — but Go evaluates a plain function call
	// immediately regardless of whether its result is later discarded, so
	// that version called BuildLiveState (which calls diagnose.DiagnoseSignals,
	// which calls invariants.AllViolations(g)) EVERY time, even when a
	// override was supplied specifically to AVOID that call — reintroducing
	// the exact unbounded same-process recursion ViolationsOverride exists to
	// prevent (AllViolations -> check_domain_claude_md_current -> this
	// render -> BuildLiveState -> DiagnoseSignals -> AllViolations -> ...).
	// Caught by TestRenderClaudeMDFromTemplateWithViolations_NeverCallsPlainBuildLiveState
	// (claudemd_violations_override_test.go) and by
	// cmd/hotam/claude_md_current_test.go's end-to-end tests, which
	// deadlocked under go test's own goroutine-leak detection before this
	// fix.
	var liveState, domainMap string
	if violations != nil {
		liveState = BuildLiveStateWithViolations(g, domainName, claudeMDCharCount, today, violations.Violations)
		domainMap = renderDomainMapBlockWithViolations(repoRoot, domainGraphs, today, violations, selfCrystalPath)
	} else {
		liveState = BuildLiveState(g, domainName, claudeMDCharCount, today)
		domainMap = renderDomainMapBlockWithViolations(repoRoot, domainGraphs, today, nil, selfCrystalPath)
	}
	if !consumer {
		// Full profile: preserve byte-identical historical order.
		parts := []string{
			WrapBlock("LIVE-STATE", liveState),
			WrapBlock("DOMAIN-MAP", domainMap),
			WrapBlock("PARENT-PROJECT", RenderParentProjectBlock(g)),
			WrapBlock("CONSTITUTION", BuildConstitutionBlock(g, domainName)),
			WrapBlock("AGENT-MAP", RenderAgentMapBlock()),
			WrapBlock("CONCEPT-MAP", RenderConceptMapBlock(consumer)),
		}
		parts = append(parts, WrapBlock("RECENTLY-REJECTED", RenderRecentlyRejectedBlock(g)))
		return strings.Join(parts, "\n")
	}
	// Consumer profile: essence-first reorder. PROJECT-ESSENCE and
	// STAKEHOLDERS open the crystal; LIVE-STATE surfaces "where we are /
	// what's blocked"; CONSTITUTION carries the requirements map; then
	// the operational layer (DOMAIN-MAP router, PARENT-PROJECT, AGENT-MAP,
	// RECENTLY-REJECTED) follows. CONCEPT-MAP stays omitted under consumer.
	parts := []string{
		WrapBlock("PROJECT-ESSENCE", RenderProjectEssenceBlock(repoRoot, domainName)),
		WrapBlock("STAKEHOLDERS", RenderStakeholdersBlock(g)),
		WrapBlock("LIVE-STATE", liveState),
		WrapBlock("CONSTITUTION", BuildConstitutionBlock(g, domainName)),
		WrapBlock("DOMAIN-MAP", domainMap),
		WrapBlock("PARENT-PROJECT", RenderParentProjectBlock(g)),
		WrapBlock("AGENT-MAP", RenderAgentMapBlock()),
		WrapBlock("RECENTLY-REJECTED", RenderRecentlyRejectedBlock(g)),
	}
	return strings.Join(parts, "\n")
}

// consumerHeaderLine renders the consumer-profile crystal's opening header
// line: "# <domainName> — <purpose one-liner>" when the active domain's
// manifest carries a purpose (loader.ResolveDomainPresentation — the same
// source RenderProjectEssenceBlock reads), or bare "# <domainName>" when it
// does not (mirrors RenderProjectEssenceBlock's own "manifest absent/without
// these fields" fallback path, R-... honest-placeholder discipline, but a
// header line reads better degrading to just the name than to an em-dash).
// The purpose is passed through shortForm/firstWholeSentence (the same
// short-form discipline RenderOperatorRoleBlock/domainPulse already use for
// crystal one-liners, R-crystal-carries-short-form) so a multi-sentence
// manifest purpose does not blow the header out into a paragraph.
func consumerHeaderLine(repoRoot, domainName string) string {
	m := loader.ResolveDomainPresentation(filepath.Join(repoRoot, "domains", domainName, "graph.json"))
	purpose := strings.TrimSpace(m.Purpose)
	if purpose == "" {
		return "# " + domainName
	}
	return "# " + domainName + " — " + firstWholeSentence(purpose)
}

// RenderClaudeMDFromTemplate renders root CLAUDE.md by substituting the two
// template placeholder lines ("<!-- mind -->" / "<!-- business -->") in the
// active profile's template (claudeMDTemplate for full, claudeMDTemplateConsumer
// for consumer) with the rendered MIND and BUSINESS content respectively.
// Every other line of the template — including the trailing human-notes
// marker comment — is preserved byte-for-byte.
//
// claudeMDCharCount feeds the LIVE-STATE CRYSTAL_CHARS budget line (the
// resident crystal's own character count — supplied by the caller to avoid
// the fixpoint hazard of CLAUDE.md measuring its own about-to-be-written
// size; see BuildLiveState / livestate.go).
//
// today (YYYY-MM-DD) is threaded through explicitly rather than computed
// internally via time.Now(), so `hotam gen-spec --today <date>` (or any
// caller) can render a byte-reproducible crystal independent of wall-clock
// date — the property CI's regen-idempotency check needs.
//
// consumer selects the output profile (loader.GenProfileConsumer when true,
// full otherwise — mirrors genSpec's own `consumer` local in
// cmd/hotam/gen_spec.go). It gates:
//
//  0. (task E2) The TEMPLATE itself — claudeMDTemplateConsumer instead of
//     claudeMDTemplate — which flips the <!-- business --> / <!-- mind -->
//     placeholder order (business bucket opens the file under consumer,
//     methodology bucket closes it) — and the opening header line, swapped
//     from the fixed "# CLAUDE.md — Hotam-Spec framework" to
//     consumerHeaderLine's domain-first "# <domainName> — <purpose>" via a
//     single targeted string replace on claudeMDHeaderSentinel, same pattern
//     as the deep-dive-clause / spec-docs-thinking replaces below.
//  1. The "Deep-dives: `spec/docs/thinking/`" clause on the boot line, and
//     the "full Canon/Narrative/Why at .../thinking/<slug>.md" clause in the
//     EMBEDDED-THINKING intro (RenderEmbeddedThinkingBlock) — both dropped
//     when consumer is true, since genSpec never writes
//     docs/gen/thinking/*.md under the consumer profile. When consumer is
//     false (the default, backward-compatible case) both clauses render
//     exactly as before — byte-identical to pre-existing full-profile output.
//  2. The internal/methodology/tools_data.go source-file reference in
//     RenderEmbeddedToolsBlock — dropped under consumer (the file does not
//     exist in an external consumer's project).
//  3. The entire CONCEPT-MAP block (RenderBusinessContent) — omitted under
//     consumer (it maps §-sections to framework source-file paths that do not
//     exist in an external consumer's project).
//
// Every generated-content path in the template is rendered against a
// "spec/..." sentinel prefix (never a bare "docs/gen/..." — see
// mediationLoopText, the tool-reference line, and the RECENTLY-REJECTED
// pointer) and domain-qualified here via one targeted replace per sentinel,
// the same pattern RenderOperatorRecursionBlock uses for its own domain
// substitution. The two sentinel families ("spec/docs/thinking/" and
// "spec/docs/gen/") share the "spec/docs/" root but diverge at the next path
// segment (thinking/ vs gen/), so a single ReplaceAll of the narrower
// "spec/docs/gen/" prefix cannot also match "spec/docs/thinking/" — no
// double-processing risk between the two families.
// This is a thin wrapper over RenderClaudeMDFromTemplateWithViolations
// passing a nil override and an empty selfCrystalPath — see
// ViolationsOverride's doc comment.
func RenderClaudeMDFromTemplate(g *ontology.Graph, domainName, repoRoot string, claudeMDCharCount int, domainGraphs map[string]*ontology.Graph, today string, consumer bool) string {
	return RenderClaudeMDFromTemplateWithViolations(g, domainName, repoRoot, claudeMDCharCount, domainGraphs, today, consumer, nil, "")
}

// RenderClaudeMDFromTemplateWithViolations is RenderClaudeMDFromTemplate's
// core, additionally accepting an already-computed ViolationsOverride
// (nil for "compute fresh", the RenderClaudeMDFromTemplate default) and
// selfCrystalPath (the path THIS render is being written to, or about to
// be — empty string for "not writing anywhere in particular", the
// RenderClaudeMDFromTemplate default). See renderDomainMapBlockWithViolations's
// doc comment for selfCrystalPath's SELF-ENTRY SPECIAL CASE: without it, a
// domain's own DOMAIN-MAP self-entry can flicker its crystal-link line based
// on filesystem write-order timing rather than the graph alone, breaking
// byte-for-byte determinism between two renders of the same graph.
//
// This is the entry point check_domain_claude_md_current
// (internal/invariants/claude_md_current.go) calls, via internal/gate's
// re-export, supplying violations.For == g and violations.Violations ==
// the SAME violation slice every other check in the current
// invariants.AllViolations(g) pass is being checked against — see
// ViolationsOverride's doc comment for why a plain nil-slice parameter would
// be ambiguous and why calling plain RenderClaudeMDFromTemplate from inside
// that check would recurse.
func RenderClaudeMDFromTemplateWithViolations(g *ontology.Graph, domainName, repoRoot string, claudeMDCharCount int, domainGraphs map[string]*ontology.Graph, today string, consumer bool, violations *ViolationsOverride, selfCrystalPath string) string {
	effectiveDomain := domainName
	if effectiveDomain == "" {
		effectiveDomain = "hotam-spec-self"
	}
	mind := RenderMindContent(g, domainName, consumer)
	business := renderBusinessContentWithViolations(g, domainName, repoRoot, claudeMDCharCount, domainGraphs, today, consumer, violations, selfCrystalPath)

	// Template + placeholder order is profile-dependent (task E2): the full
	// profile keeps MIND before BUSINESS (claudeMDTemplate, byte-identical to
	// the pre-E2 layout); the consumer profile flips to BUSINESS before MIND
	// (claudeMDTemplateConsumer) so a freshly-booted operator on an external
	// business domain reads "what is this project" before the Hotam-Spec
	// methodology seed.
	activeTemplate := claudeMDTemplate
	if consumer {
		activeTemplate = claudeMDTemplateConsumer
	}

	srcLines := strings.Split(activeTemplate, "\n")
	outLines := make([]string, 0, len(srcLines))
	for _, line := range srcLines {
		switch strings.TrimSpace(line) {
		case mindPlaceholder:
			outLines = append(outLines, mind)
		case businessPlaceholder:
			outLines = append(outLines, business)
		default:
			outLines = append(outLines, line)
		}
	}
	out := strings.Join(outLines, "\n")

	// Consumer profile: swap the framework-identity header line for a
	// domain-first title carrying the active domain's own purpose (task E2 —
	// external review P1: the file must open with the domain's essence, not
	// "Hotam-Spec framework"). Falls back to the bare domain name when no
	// manifest purpose is on record (mirrors RenderProjectEssenceBlock's own
	// em-dash-placeholder fallback shape, but a header line reads better
	// without a bare em-dash, so it degrades to just the domain name).
	if consumer {
		out = strings.Replace(out, claudeMDHeaderSentinel, consumerHeaderLine(repoRoot, effectiveDomain), 1)
	}

	// Consumer profile: drop the boot line's deep-dive clause entirely (rather
	// than domain-qualify a path that was never written) so the line ends
	// cleanly after "operating seed." with no dangling "Deep-dives:" pointer.
	if consumer {
		out = strings.ReplaceAll(out, deepDiveClauseSentinel, "")
	} else {
		out = strings.ReplaceAll(out, "`spec/docs/thinking/`", "`domains/"+effectiveDomain+"/docs/gen/thinking/`")
	}

	// The three bare `docs/gen/...` links (bug 1) are rendered against the
	// `spec/docs/gen/...` sentinel prefix regardless of profile (all four
	// target files -- TENSIONS.md, REQUIREMENTS.md, HISTORY.md,
	// tools/INDEX.md -- are always written by genSpec under EITHER profile),
	// so this replace is unconditional.
	out = strings.ReplaceAll(out, "spec/docs/gen/", "domains/"+effectiveDomain+"/docs/gen/")

	return out
}

// maxCrystalFixpointIterations caps the render→measure→re-render loop that
// converges the resident crystal's self-referential CRYSTAL_CHARS measurement.
// The ONLY way the embedded number can shift the rendered length is a
// digit-count change (e.g. "9999"→"10000" is +1 char); convergence in ≤3
// iterations is guaranteed for any realistic crystal size, so 5 is a generous
// guard against a non-convergent (oscillating) measurement that should never
// occur in practice.
const maxCrystalFixpointIterations = 5

// ComputeCrystalCharCountFixpoint computes the fixpoint rune-count of the
// rendered root crystal. The resident crystal's LIVE-STATE block embeds its
// OWN character count (the CRYSTAL_CHARS budget line, R-context-budget-rule),
// so the measurement is self-referential: embedding a different count can
// shift the rendered length (a digit-count change like "9999"→"10000" adds
// one char), which changes the very number being embedded. The fixpoint is
// the count that, when embedded, produces a crystal whose rune count equals
// that count.
//
// It iterates render→measure (rune count via utf8.RuneCountInString, the
// convention every other CRYSTAL_CHARS call site uses)→re-render until the
// embedded number stops changing, capped at maxCrystalFixpointIterations; a
// non-convergence returns an error (oscillation should be impossible since
// only a small number's digit count can shift, but it is guarded anyway).
//
// The returned fixpoint is what every LIVE-STATE carrier (the root crystal
// AND docs/gen/AGENT-CONTEXT.md + live-state.md, which reuse BuildLiveState)
// must embed so the two artifacts agree regardless of the --claude-md flag
// (which gates only the crystal's DISK WRITE, not whether the measurement is
// computed). This replaces the former buggy mechanism that read the size of a
// stale pre-existing CLAUDE.md from the previous run — a read that made two
// consecutive gen-spec passes never converge and left AGENT-CONTEXT.md stuck
// at "0 chars" whenever --claude-md was omitted.
//
// consumer is threaded straight through to RenderClaudeMDFromTemplate: the
// consumer-profile crystal renders a few characters shorter (the dropped
// deep-dive clauses), so the fixpoint must be measured against the SAME
// profile genSpec will actually write, or the embedded CRYSTAL_CHARS count
// would disagree with the real rendered crystal under consumer.
// This is a thin wrapper over ComputeCrystalCharCountFixpointWithViolations
// passing a nil override and an empty selfCrystalPath — see
// ViolationsOverride's doc comment.
func ComputeCrystalCharCountFixpoint(g *ontology.Graph, domainName, repoRoot string, domainGraphs map[string]*ontology.Graph, today string, consumer bool) (int, error) {
	return ComputeCrystalCharCountFixpointWithViolations(g, domainName, repoRoot, domainGraphs, today, consumer, nil, "")
}

// ComputeCrystalCharCountFixpointWithViolations is
// ComputeCrystalCharCountFixpoint's core, threading violations AND
// selfCrystalPath through to RenderClaudeMDFromTemplateWithViolations on
// every fixpoint iteration (nil/"" for "compute fresh, no self-entry
// special-casing" — see ViolationsOverride's and
// renderDomainMapBlockWithViolations's doc comments respectively).
func ComputeCrystalCharCountFixpointWithViolations(g *ontology.Graph, domainName, repoRoot string, domainGraphs map[string]*ontology.Graph, today string, consumer bool, violations *ViolationsOverride, selfCrystalPath string) (int, error) {
	count := 0
	for i := 0; i < maxCrystalFixpointIterations; i++ {
		rendered := RenderClaudeMDFromTemplateWithViolations(g, domainName, repoRoot, count, domainGraphs, today, consumer, violations, selfCrystalPath)
		measured := utf8.RuneCountInString(rendered)
		if measured == count {
			return count, nil
		}
		count = measured
	}
	return count, fmt.Errorf("crystal char-count fixpoint did not converge after %d iterations (last measurement %d) — the resident crystal's self-referential size measurement is oscillating, which should be impossible since only a digit-count shift can change the rendered length", maxCrystalFixpointIterations, count)
}
