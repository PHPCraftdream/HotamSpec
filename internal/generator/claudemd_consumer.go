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
	"github.com/PHPCraftdream/HotamSpec/internal/docbundle"
	"github.com/PHPCraftdream/HotamSpec/internal/gate"
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

func consumerReqLine(g *ontology.Graph, r ontology.Requirement) string {
	claim := collapseWS(shortForm(collapseWS(requirementClaim(g, r)), collapseWS(r.Summary)))
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
		return fmt.Sprintf("`%s`, `hotam req list --domain domains/%s`", localizedDomainDocPath(g, domainName, "REQUIREMENTS.md"), domainName)
	case g.RequirementsAuthorityCode:
		return fmt.Sprintf("`%s`, `domains/%s/spec/requirements.go`, `hotam req list --domain domains/%s`", localizedDomainDocPath(g, domainName, "SPEC.md"), domainName, domainName)
	}
	return fmt.Sprintf("`hotam req list --domain domains/%s`", domainName)
}

// renderConsumerRequirements renders the requirement list block body.
func renderConsumerRequirements(g *ontology.Graph, domainName string) string {
	reqs := consumerLiveRequirements(g)
	if len(reqs) == 0 {
		return serviceText(g, "## Requirements") + "\n\n" + serviceText(g, "_(none yet)_")
	}
	lines := make([]string, len(reqs))
	size := 0
	for i, r := range reqs {
		lines[i] = consumerReqLine(g, r)
		size += utf8.RuneCountInString(lines[i]) + 1
	}
	if size <= ConsumerRequirementsBudget {
		head := serviceText(g, "## Requirements (%d) — id — claim [E enforced|S structural|P prose] ← verified_by", len(reqs))
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
		serviceText(g, "## Requirements (%d)", len(reqs)),
		"",
		serviceText(g, "E (enforced) %d · S (structural) %d · P (prose) %d. Inline list omitted: over %d chars.", e, s, p, ConsumerRequirementsBudget),
		serviceText(g, "Full text: %s.", consumerFullTextPointer(g, domainName)),
	}
	groups := make(map[string][]ontology.Requirement)
	for _, r := range reqs {
		groups[gate.SpecPackage(r)] = append(groups[gate.SpecPackage(r)], r)
	}
	packages := make([]string, 0, len(groups))
	for pkg := range groups {
		packages = append(packages, pkg)
	}
	sort.Strings(packages)
	out = append(out, "", serviceText(g, "| Package | Requirements | E | S | P |"), "|---|---:|---:|---:|---:|")
	var layout docbundle.Layout
	if g.SelfExecutingAtoms {
		var err error
		layout, err = docbundle.NewLayout(g.Languages, g.DefaultLanguage)
		if err != nil {
			panic(err)
		}
	}
	for _, pkg := range packages {
		var enforced, structural, prose int
		for _, r := range groups[pkg] {
			switch flagFor(r.Enforcement) {
			case "E":
				enforced++
			case "S":
				structural++
			default:
				prose++
			}
		}
		link := localizedDomainDocPath(g, domainName, "SPEC.md")
		if g.SelfExecutingAtoms {
			shardPath, pathErr := layout.SpecShardPath(renderLanguage(g), pkg+".md")
			if pathErr != nil {
				panic(pathErr)
			}
			link = "domains/" + domainName + "/" + filepath.ToSlash(shardPath)
		}
		out = append(out, fmt.Sprintf("| [%s](%s) | %d | %d | %d | %d |", pkg, link, len(groups[pkg]), enforced, structural, prose))
	}
	if len(notEnforced) == 0 {
		out = append(out, serviceText(g, "DRAFT or not ENFORCED: none."))
	} else {
		out = append(out, serviceText(g, "DRAFT or not ENFORCED (%d): %s.", len(notEnforced), strings.Join(notEnforced, ", ")))
	}
	return strings.Join(out, "\n")
}

// renderConsumerStatus renders the 3-line status body.
func renderConsumerStatus(g *ontology.Graph, today string, violations []invariants.Violation) string {
	signals := diagnose.DiagnoseSignalsWithViolations(g, today, violations)
	top := serviceText(g, "none — graph clean")
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
		serviceText(g, "## Status"),
		"",
		serviceText(g, "- **top action:** %s (all: `hotam what-now`)", top),
		serviceText(g, "- **debt:** %d/%d SETTLED ENFORCED · %d DRAFT · %d closeable", enforced, settled, draft, debt),
		serviceText(g, "- **violations:** %d (`hotam all-violations`)", len(violations)),
	}, "\n")
}

// renderConsumerHowToChange renders the change workflow, branching on the
// domain's requirements authority / discipline.
func renderConsumerHowToChange(g *ontology.Graph, domainName string) string {
	d := "domains/" + domainName
	var lines []string
	lines = append(lines, serviceText(g, "## How to change"))
	lines = append(lines, "")
	switch {
	case g.SelfExecutingAtoms:
		lines = append(lines,
			serviceText(g, "Requirements come from documented methods and executed values (`self_executing_atoms: true`)."),
			serviceText(g, "1. Edit the documented value method and `hotamspec.Fact(t, object.Method, want)` test; use `hotamspec.Holds(t, object.Predicate, evidence...)` for relations."),
			serviceText(g, "2. Keep `%s/spec/requirements.go` for REJECTED entries and explicit overrides only.", d),
			serviceText(g, "3. `hotam sync-domain --domain %s` prints the diff and hash; present them to the owner.", d),
			serviceText(g, "4. After approval: `hotam sync-domain --domain %s --today YYYY-MM-DD --confirm-hash <hex>`.", d),
			serviceText(g, "5. `hotam gen-spec --domain %s --spec`, then `hotam all-violations --domain %s` must print 0.", d, d),
		)
	case g.RequirementsAuthorityCode:
		lines = append(lines,
			serviceText(g, "Requirements live in code (`requirements_authority: code`); `hotam land` refuses Requirement/Rejection JSON here."),
			serviceText(g, "1. Edit the scenario test and the requirement literal in `%s/spec/requirements.go`.", d),
			serviceText(g, "2. `hotam sync-domain --domain %s` (dry-run) prints the diff and its hash.", d),
			serviceText(g, "3. Present the diff to the owner; the resolver decides."),
			serviceText(g, "4. After approval: `hotam sync-domain --domain %s --today YYYY-MM-DD --confirm-hash <hex>`.", d),
			serviceText(g, "5. `hotam all-violations --domain %s` must print 0.", d),
			serviceText(g, "Conflicts, assumptions and other non-requirement nodes: a Proposed* JSON, then `hotam land <file.json> --domain %s --today YYYY-MM-DD`.", d),
		)
	case g.Discipline == loader.DisciplineFull:
		lines = append(lines,
			serviceText(g, "1. Write the scenario test (hotamspec recorder) that proves the behavior; its run also generates the requirement text."),
			serviceText(g, "2. Draft a ProposedRequirement JSON citing it in `verified_by`; present it with the R-ids it touches; the resolver decides."),
			serviceText(g, "3. After approval: `hotam land <file.json> --domain %s --today YYYY-MM-DD`.", d),
			serviceText(g, "4. `hotam all-violations --domain %s` must print 0.", d),
		)
	default:
		lines = append(lines,
			serviceText(g, "1. Draft a ProposedRequirement JSON (any Proposed* kind); present it with the R-ids it touches; the resolver decides."),
			serviceText(g, "2. After approval: `hotam land <file.json> --domain %s --today YYYY-MM-DD`.", d),
			serviceText(g, "3. `hotam all-violations --domain %s` must print 0.", d),
		)
	}
	return strings.Join(lines, "\n")
}

func renderConsumerRules(g *ontology.Graph) string {
	return strings.Join([]string{
		serviceText(g, "## Rules"),
		"",
		serviceText(g, "1. The resolver decides: present options with their R-ids; never close a conflict or settle a requirement yourself."),
		serviceText(g, "2. Cite R-ids (`R-…`/`C-…`/`A-…`), not vibes."),
		serviceText(g, "3. Speak the domain's language, not the framework's, unless the human uses the framework's terms first."),
		serviceText(g, "4. Never hand-edit `graph.json`, `docs/gen/` or this file: they are generated."),
		"",
		serviceText(g, "Modality: when a claim asserts a hard always/never/must/must-not/only/any, in any language, write the ALL-CAPS token (ALWAYS, NEVER, MUST, MUST NOT, ONLY, ANY) into the claim text; `hotam confront` detects conflicts by them."),
	}, "\n")
}

func renderConsumerPointers(g *ontology.Graph, domainName string) string {
	return strings.Join([]string{
		serviceText(g, "## Pointers"),
		"",
		serviceText(g, "`hotam what-now` — next actions · `hotam req show <id>` — one requirement · `hotam -h` — all commands · `domains/%s/README.md` — domain guide.", domainName),
	}, "\n")
}

// consumerRejectedBlock renders the anti-relitigation block, or "" when
// nothing was rejected with a known replacement.
func consumerRejectedBlock(g *ontology.Graph) string {
	body := RenderRecentlyRejectedBlock(g)
	emptyNotice := serviceText(g, "_(no anti-relitigation entries — nothing recently rejected.)_")
	if strings.Contains(body, emptyNotice) {
		return ""
	}
	body = strings.TrimPrefix(body, serviceText(g, generatedHeaderComment)+"\n\n")
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
		essence = append(essence, serviceText(g, "- **charter** — %s", collapseWS(m.Charter)))
	}
	if len(m.Goals) > 0 {
		essence = append(essence, serviceText(g, "- **goals** — %s", strings.Join(m.Goals, ", ")))
	}
	if m.Director != "" {
		essence = append(essence, serviceText(g, "- **director** — %s", m.Director))
	}
	add("PROJECT-ESSENCE", strings.Join(essence, "\n"))

	if len(g.Stakeholders) > 0 {
		var st []string
		for _, s := range NarrativeOrder(g.Stakeholders, func(s ontology.Stakeholder) int { return s.DeclOrder }) {
			st = append(st, fmt.Sprintf("`%s` %s", s.ID, collapseWS(s.Name)))
		}
		add("STAKEHOLDERS", serviceText(g, "Stakeholders: %s", strings.Join(st, " · ")))
	}

	add("REQUIREMENTS", renderConsumerRequirements(g, domainName))
	add("STATUS", renderConsumerStatus(g, today, viol))

	if consumerDomainCount(repoRoot, domainGraphs) > 1 {
		dm := renderDomainMapBlockWithLanguage(g.RenderLanguage, repoRoot, domainGraphs, today, violations, selfCrystalPath)
		dm = strings.TrimPrefix(rewriteRepoAbsPaths(repoRoot, dm), serviceText(g, generatedHeaderComment)+"\n\n")
		add("DOMAIN-MAP", dm)
	}
	if g.ParentDeclared && g.Parent != "" {
		add("PARENT-PROJECT", serviceText(g, "Parent project: `%s`.", g.Parent))
	}
	rejected := consumerRejectedBlock(g)
	if rejected != "" {
		historyPath := "domains/" + domainName + "/" + localizedDocumentPath(g, "docs/gen/HISTORY.md")
		rejected = strings.ReplaceAll(rejected, "spec/docs/gen/HISTORY.md", historyPath)
		add("RECENTLY-REJECTED", rejected)
	}

	add("HOW-TO-CHANGE", renderConsumerHowToChange(g, domainName))
	add("RULES", renderConsumerRules(g))
	add("POINTERS", renderConsumerPointers(g, domainName))

	header := consumerHeaderLine(repoRoot, domainName)
	var sb strings.Builder
	sb.WriteString(header + "\n\n")
	sb.WriteString(serviceText(g, generatedHeaderComment) + "\n\n")
	sb.WriteString(strings.Join(parts, "\n\n"))
	sb.WriteString("\n\n" + DurableNotesMarkerLine + "\n")
	return sb.String()
}
