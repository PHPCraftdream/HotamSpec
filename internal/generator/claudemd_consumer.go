package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/PHPCraftdream/HotamSpec/internal/diagnose"
	"github.com/PHPCraftdream/HotamSpec/internal/invariants"
	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// Lightweight CONSUMER crystal. The full profile (self-hosting domains)
// keeps the heavy template in claudemd.go byte-for-byte; the consumer profile
// renders this compact one instead: header + goals, the requirement list
// inline (or counts above the budget), a 3-line status, how-to-change, four
// rules, pointers. Optional blocks appear only when non-empty.

// ConsumerRequirementsBudget is the all-or-nothing character budget of the
// inline requirement list in the consumer crystal.
const ConsumerRequirementsBudget = 6000

// RequirementsMDWritten reports whether genSpec writes docs/gen/REQUIREMENTS.md.
// Full profile: whenever the graph is non-empty. Consumer profile: also
// dropped for code-authority + discipline:"full" domains, whose requirement
// text lives in spec/requirements.go and docs/gen/SPEC.md.
func RequirementsMDWritten(g *ontology.Graph, consumer bool) bool {
	if !RequirementsMDHasContent(g) {
		return false
	}
	if consumer && g.RequirementsAuthorityCode && g.Discipline == loader.DisciplineFull {
		return false
	}
	return true
}

// ConsumerHistoryMDHasContent: HISTORY.md has its own data only when a
// requirement was REJECTED or a conflict is decided/parked.
func ConsumerHistoryMDHasContent(g *ontology.Graph) bool {
	for _, r := range g.Requirements {
		if r.Status == ontology.StatusREJECTED {
			return true
		}
	}
	for _, c := range g.Conflicts {
		if c.IsDecided() || strings.HasPrefix(c.Lifecycle, ontology.ConflictREVISITPrefix) {
			return true
		}
	}
	return false
}

// ConsumerOpenMDHasContent: OPEN.md has its own data only when something is open.
func ConsumerOpenMDHasContent(g *ontology.Graph) bool {
	for _, r := range g.Requirements {
		if r.IsOpen() {
			return true
		}
	}
	for _, c := range g.Conflicts {
		if c.IsUnresolved() {
			return true
		}
	}
	return false
}

// ConsumerUnenforcedMDHasContent: UNENFORCED.md has its own data only when a
// SETTLED requirement is not ENFORCED or a DRAFT exists.
func ConsumerUnenforcedMDHasContent(g *ontology.Graph) bool {
	for _, r := range g.Requirements {
		if r.Status == ontology.StatusDRAFT {
			return true
		}
		if r.Status == ontology.StatusSETTLED && r.Enforcement != ontology.EnforcementENFORCED {
			return true
		}
	}
	return false
}

var consumerDocPathRE = regexp.MustCompile(`docs/gen/[A-Za-z-]+\.md`)

func collapseWS(s string) string { return strings.Join(strings.Fields(s), " ") }

// consumerLiveRequirements: every non-REJECTED requirement in narrative order.
func consumerLiveRequirements(g *ontology.Graph) []ontology.Requirement {
	var out []ontology.Requirement
	for _, r := range NarrativeOrder(g.Requirements, func(r ontology.Requirement) int { return r.DeclOrder }) {
		if r.Status != ontology.StatusREJECTED {
			out = append(out, r)
		}
	}
	return out
}

func consumerTestNames(verifiedBy []string) string {
	var names []string
	for _, v := range verifiedBy {
		if i := strings.LastIndex(v, ":"); i >= 0 {
			v = v[i+1:]
		}
		names = append(names, v)
	}
	switch {
	case len(names) == 0:
		return ""
	case len(names) <= 2:
		return strings.Join(names, ", ")
	}
	return fmt.Sprintf("%s, %s +%d", names[0], names[1], len(names)-2)
}

func consumerReqLine(r ontology.Requirement) string {
	claim := collapseWS(shortForm(collapseWS(r.Claim), collapseWS(r.Summary)))
	line := fmt.Sprintf("- %s — %s [%s]", r.ID, claim, flagFor(r.Enforcement))
	if r.Status == ontology.StatusDRAFT {
		line += " DRAFT"
	}
	if t := consumerTestNames(r.VerifiedBy); t != "" {
		line += " ← " + t
	}
	return line
}

// consumerFullTextPointer names where the full requirement text lives.
func consumerFullTextPointer(g *ontology.Graph, domainName string) string {
	switch {
	case RequirementsMDWritten(g, true):
		return fmt.Sprintf("`domains/%s/docs/gen/REQUIREMENTS.md`, `hotam req list --domain domains/%s`", domainName, domainName)
	case g.RequirementsAuthorityCode:
		return fmt.Sprintf("`domains/%s/docs/gen/SPEC.md`, `domains/%s/spec/requirements.go`, `hotam req list --domain domains/%s`", domainName, domainName, domainName)
	}
	return fmt.Sprintf("`hotam req list --domain domains/%s`", domainName)
}

// renderConsumerRequirements renders the requirement list block body.
func renderConsumerRequirements(g *ontology.Graph, domainName string) string {
	reqs := consumerLiveRequirements(g)
	if len(reqs) == 0 {
		return "## Requirements\n\n_(none yet)_"
	}
	lines := make([]string, len(reqs))
	size := 0
	for i, r := range reqs {
		lines[i] = consumerReqLine(r)
		size += utf8.RuneCountInString(lines[i]) + 1
	}
	if size <= ConsumerRequirementsBudget {
		head := fmt.Sprintf("## Requirements (%d) — id — claim [E enforced|S structural|P prose] ← verified_by", len(reqs))
		return head + "\n\n" + strings.Join(lines, "\n")
	}
	var e, s, p int
	var notEnforced []string
	for _, r := range reqs {
		switch flagFor(r.Enforcement) {
		case "E":
			e++
		case "S":
			s++
		default:
			p++
		}
		if r.Status == ontology.StatusDRAFT || r.Enforcement != ontology.EnforcementENFORCED {
			tag := flagFor(r.Enforcement)
			if r.Status == ontology.StatusDRAFT {
				tag += ",DRAFT"
			}
			notEnforced = append(notEnforced, r.ID+"["+tag+"]")
		}
	}
	out := []string{
		fmt.Sprintf("## Requirements (%d)", len(reqs)),
		"",
		fmt.Sprintf("E (enforced) %d · S (structural) %d · P (prose) %d. Inline list omitted: over %d chars.", e, s, p, ConsumerRequirementsBudget),
		"Full text: " + consumerFullTextPointer(g, domainName) + ".",
	}
	if len(notEnforced) == 0 {
		out = append(out, "DRAFT or not ENFORCED: none.")
	} else {
		out = append(out, fmt.Sprintf("DRAFT or not ENFORCED (%d): %s.", len(notEnforced), strings.Join(notEnforced, ", ")))
	}
	return strings.Join(out, "\n")
}

// renderConsumerStatus renders the 3-line status body.
func renderConsumerStatus(g *ontology.Graph, today string, violations []invariants.Violation) string {
	signals := diagnose.DiagnoseSignalsWithViolations(g, today, violations)
	top := "none — graph clean"
	if len(signals) > 0 {
		sig := signals[0]
		msg := collapseWS(shortForm(collapseWS(sig.Message), summaryForTarget(g, sig.Target)))
		msg = consumerDocPathRE.ReplaceAllString(msg, "hotam what-now")
		top = fmt.Sprintf("[P%d] `%s` — %s", sig.Priority, sig.Target, msg)
	}
	var settled, enforced, draft, debt int
	for _, r := range g.Requirements {
		if r.Status == ontology.StatusSETTLED {
			settled++
			if r.Enforcement == ontology.EnforcementENFORCED {
				enforced++
			}
			if r.IsCloseableDebt() {
				debt++
			}
		}
		if r.Status == ontology.StatusDRAFT {
			draft++
		}
	}
	return strings.Join([]string{
		"## Status",
		"",
		"- **top action:** " + top + " (all: `hotam what-now`)",
		fmt.Sprintf("- **debt:** %d/%d SETTLED ENFORCED · %d DRAFT · %d closeable", enforced, settled, draft, debt),
		fmt.Sprintf("- **violations:** %d (`hotam all-violations`)", len(violations)),
	}, "\n")
}

// renderConsumerHowToChange renders the change workflow, branching on the
// domain's requirements authority / discipline.
func renderConsumerHowToChange(g *ontology.Graph, domainName string) string {
	d := "domains/" + domainName
	var lines []string
	lines = append(lines, "## How to change")
	lines = append(lines, "")
	switch {
	case g.RequirementsAuthorityCode:
		lines = append(lines,
			"Requirements live in code (`requirements_authority: code`); `hotam land` refuses Requirement/Rejection JSON here.",
			"1. Edit the scenario test and the requirement literal in `"+d+"/spec/requirements.go`.",
			"2. `hotam sync-domain --domain "+d+"` (dry-run) prints the diff and its hash.",
			"3. Present the diff to the owner; the resolver decides.",
			"4. After approval: `hotam sync-domain --domain "+d+" --today YYYY-MM-DD --confirm-hash <hex>`.",
			"5. `hotam all-violations --domain "+d+"` must print 0.",
			"Conflicts, assumptions and other non-requirement nodes: a Proposed* JSON, then `hotam land <file.json> --domain "+d+" --today YYYY-MM-DD`.",
		)
	case g.Discipline == loader.DisciplineFull:
		lines = append(lines,
			"1. Write the scenario test (hotamspec recorder) that proves the behavior; its run also generates the requirement text.",
			"2. Draft a ProposedRequirement JSON citing it in `verified_by`; present it with the R-ids it touches; the resolver decides.",
			"3. After approval: `hotam land <file.json> --domain "+d+" --today YYYY-MM-DD`.",
			"4. `hotam all-violations --domain "+d+"` must print 0.",
		)
	default:
		lines = append(lines,
			"1. Draft a ProposedRequirement JSON (any Proposed* kind); present it with the R-ids it touches; the resolver decides.",
			"2. After approval: `hotam land <file.json> --domain "+d+" --today YYYY-MM-DD`.",
			"3. `hotam all-violations --domain "+d+"` must print 0.",
		)
	}
	return strings.Join(lines, "\n")
}

func renderConsumerRules() string {
	return strings.Join([]string{
		"## Rules",
		"",
		"1. The resolver decides: present options with their R-ids; never close a conflict or settle a requirement yourself.",
		"2. Cite R-ids (`R-…`/`C-…`/`A-…`), not vibes.",
		"3. Speak the domain's language, not the framework's, unless the human uses the framework's terms first.",
		"4. Never hand-edit `graph.json`, `docs/gen/` or this file: they are generated.",
		"",
		"Modality: when a claim asserts a hard always/never/must/must-not/only/any, in any language, write the ALL-CAPS token (ALWAYS, NEVER, MUST, MUST NOT, ONLY, ANY) into the claim text; `hotam confront` detects conflicts by them.",
	}, "\n")
}

func renderConsumerPointers(domainName string) string {
	return strings.Join([]string{
		"## Pointers",
		"",
		"`hotam what-now` — next actions · `hotam req show <id>` — one requirement · `hotam -h` — all commands · `domains/" + domainName + "/README.md` — domain guide.",
	}, "\n")
}

// consumerRejectedBlock renders the anti-relitigation block, or "" when
// nothing was rejected with a known replacement.
func consumerRejectedBlock(g *ontology.Graph) string {
	body := RenderRecentlyRejectedBlock(g)
	if strings.Contains(body, "_(no anti-relitigation entries") {
		return ""
	}
	body = strings.TrimPrefix(body, generatedHeaderComment+"\n\n")
	body = strings.ReplaceAll(body, " Full list: `spec/docs/gen/HISTORY.md`.", "")
	body = strings.ReplaceAll(body, "full history + WHY: `spec/docs/gen/HISTORY.md`, `hotam req show <id>`", "WHY: `hotam req show <id>`")
	return body
}

// consumerDomainCount counts domains/<name>/ directories (not "_"-prefixed).
func consumerDomainCount(repoRoot string, domainGraphs map[string]*ontology.Graph) int {
	entries, err := os.ReadDir(filepath.Join(repoRoot, "domains"))
	if err != nil {
		return len(domainGraphs)
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), "_") {
			n++
		}
	}
	return n
}

// renderConsumerCrystal renders the whole lightweight consumer crystal,
// through the durable-notes marker line.
func renderConsumerCrystal(g *ontology.Graph, domainName, repoRoot string, domainGraphs map[string]*ontology.Graph, today string, violations *ViolationsOverride, selfCrystalPath string) string {
	var viol []invariants.Violation
	if violations != nil {
		viol = violations.Violations
	} else {
		viol = invariants.AllViolations(g)
	}

	var parts []string
	add := func(name, body string) {
		if body != "" {
			parts = append(parts, WrapBlock(name, body))
		}
	}

	// Essence: goals/director/charter only when declared.
	m := loader.ResolveDomainPresentation(filepath.Join(repoRoot, "domains", domainName, "graph.json"))
	var essence []string
	if m.Charter != "" {
		essence = append(essence, "- **charter** — "+collapseWS(m.Charter))
	}
	if len(m.Goals) > 0 {
		essence = append(essence, "- **goals** — "+strings.Join(m.Goals, ", "))
	}
	if m.Director != "" {
		essence = append(essence, "- **director** — "+m.Director)
	}
	add("PROJECT-ESSENCE", strings.Join(essence, "\n"))

	if len(g.Stakeholders) > 0 {
		var st []string
		for _, s := range NarrativeOrder(g.Stakeholders, func(s ontology.Stakeholder) int { return s.DeclOrder }) {
			st = append(st, fmt.Sprintf("`%s` %s", s.ID, collapseWS(s.Name)))
		}
		add("STAKEHOLDERS", "Stakeholders: "+strings.Join(st, " · "))
	}

	add("REQUIREMENTS", renderConsumerRequirements(g, domainName))
	add("STATUS", renderConsumerStatus(g, today, viol))

	if consumerDomainCount(repoRoot, domainGraphs) > 1 {
		dm := renderDomainMapBlockWithViolations(repoRoot, domainGraphs, today, violations, selfCrystalPath)
		dm = strings.TrimPrefix(rewriteRepoAbsPaths(repoRoot, dm), generatedHeaderComment+"\n\n")
		add("DOMAIN-MAP", dm)
	}
	if g.ParentDeclared && g.Parent != "" {
		add("PARENT-PROJECT", "Parent project: `"+g.Parent+"`.")
	}
	add("RECENTLY-REJECTED", consumerRejectedBlock(g))

	add("HOW-TO-CHANGE", renderConsumerHowToChange(g, domainName))
	add("RULES", renderConsumerRules())
	add("POINTERS", renderConsumerPointers(domainName))

	header := consumerHeaderLine(repoRoot, domainName)
	var sb strings.Builder
	sb.WriteString(header + "\n\n")
	sb.WriteString(generatedHeaderComment + "\n\n")
	sb.WriteString(strings.Join(parts, "\n\n"))
	sb.WriteString("\n\n" + DurableNotesMarkerLine + "\n")
	return sb.String()
}
