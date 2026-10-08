package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/diagnose"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/proposal"
)

// cmdConfront implements `hotam confront <text> [--domain <path>] [--file
// <path>] [--proposal <path>] [--json]`: the CONFRONT step of the mediation
// loop, made executable.
//
// Text candidates receive lexical advisory evidence. Requirement proposals also
// carry authored links and explicit unresolved Conflict carriers; proposal mode
// includes shared-assumption/axis structural signals.
//
// Positional text, --file (- for stdin), and --proposal are mutually exclusive.
// Confront itself informs and returns zero; apply/land enforce formal carriers.
func cmdConfront(args []string) error {
	fs := newFlagSet("confront")
	domain := fs.String("domain", "", "domain directory (default: "+defaultDomainRel+")")
	file := fs.String("file", "", "read candidate text from this file (use \"-\" for stdin)")
	proposalPath := fs.String("proposal", "", "confront proposal metadata, formal Conflict carriers and shared-assumption/axis signals")
	asJSON := fs.Bool("json", false, "emit machine-readable JSON")
	if err := parseCommandFlags(fs, args); err != nil {
		return err
	}

	if *proposalPath != "" {
		if *file != "" || fs.NArg() > 0 {
			return fmt.Errorf("confront: pass either --proposal OR (--file OR a positional text argument), not a mix")
		}
		return cmdConfrontProposal(*proposalPath, *domain, *asJSON)
	}

	candidate, err := readConfrontCandidate(*file, fs.Args())
	if err != nil {
		return err
	}

	domainDir, err := resolveDomain(*domain)
	if err != nil {
		return err
	}
	g, err := loadDomainGraph(domainDir)
	if err != nil {
		return err
	}

	result := diagnose.Confront(g, candidate)

	if *asJSON {
		return printJSON(result)
	}
	fmt.Print(formatConfrontReport(result))
	return nil
}

// cmdConfrontProposal parses a proposal, runs the shared confrontation API
// (including Requirement metadata links and formal carriers when available),
// and additionally runs the existing structural checks for assumptions and
// axes. The operator sees the evidence report and structural section together.
// --proposal always renders the structural verdict, even when clear, so it is
// explicit that both checks ran.
func cmdConfrontProposal(proposalPath, domainFlag string, asJSON bool) error {
	p, err := parseProposalFile(proposalPath)
	if err != nil {
		return err
	}
	domainDir, err := resolveDomain(domainFlag)
	if err != nil {
		return err
	}
	g, err := loadDomainGraph(domainDir)
	if err != nil {
		return err
	}

	lexical := confrontProposal(g, p)

	structural := structuralConfrontForProposal(g, p)

	if asJSON {
		return printJSON(confrontProposalEnvelope{
			Lexical:    lexical,
			Structural: structural,
		})
	}
	fmt.Print(formatConfrontReport(lexical))
	fmt.Print(formatStructuralConfrontReport(structural))
	return nil
}

func confrontProposal(g *ontology.Graph, p proposal.Proposal) diagnose.ConfrontResult {
	if requirement, ok := p.(proposal.ProposedRequirement); ok {
		return diagnose.ConfrontRequirement(g, requirementFromProposal(requirement))
	}
	return diagnose.Confront(g, proposeConfrontText(p))
}

// structuralConfrontForProposal dispatches to the right structural check based
// on the proposal kind. A ProposedRequirement runs the shared-assumption
// cluster check (its Assumptions field). A ProposedConflict runs BOTH the
// shared-assumption check (if it carries a SharedAssumption) and the
// axis-co-reference check (its Axis field). Every other kind — Rejection,
// Stakeholder, Assumption, EntityType, etc. — has no assumption list and no
// axis, so the structural result is a clean no-op (Clear=true, empty slices).
func structuralConfrontForProposal(g *ontology.Graph, p proposal.Proposal) diagnose.StructuralConfrontResult {
	switch v := p.(type) {
	case proposal.ProposedRequirement:
		return diagnose.StructuralConfrontForRequirement(g, v.Assumptions)
	case proposal.ProposedConflict:
		var cAssumptions []string
		if strings.TrimSpace(v.SharedAssumption) != "" {
			cAssumptions = []string{v.SharedAssumption}
		}
		return diagnose.StructuralConfrontForConflict(g, v.Axis, cAssumptions)
	default:
		return diagnose.StructuralConfrontResult{
			SharedAssumptionHits: []diagnose.Candidate{},
			AxisCoReferenceHits:  []diagnose.Candidate{},
			Clear:                true,
		}
	}
}

// confrontProposalEnvelope is the --json output shape for `hotam confront
// --proposal`: the lexical ConfrontResult under "lexical" and the structural
// StructuralConfrontResult under "structural", so machine consumers get both
// signals in one payload without a second round-trip.
type confrontProposalEnvelope struct {
	Lexical    diagnose.ConfrontResult           `json:"lexical"`
	Structural diagnose.StructuralConfrontResult `json:"structural"`
}

// formatStructuralConfrontReport renders the human-readable structural-hits
// section for `hotam confront --proposal`. It always produces an explicit
// verdict — the hits (grouped by shared-assumption clusters then axis
// co-reference groups, each with members + evidence) or the "no structural
// tension detected" green line. Silence is never the output: the operator
// invoked --proposal specifically for the richer check and must know what the
// structural side concluded, not wonder whether it ran.
func formatStructuralConfrontReport(r diagnose.StructuralConfrontResult) string {
	var b strings.Builder
	b.WriteString("structural confront — shared-assumption / axis-co-reference checks (advisory; exit code always 0).\n")
	if r.Clear {
		b.WriteString("no structural tension detected — candidate joins no shared-assumption cluster and co-references no existing axis.\n")
		return b.String()
	}
	if len(r.SharedAssumptionHits) > 0 {
		b.WriteString(fmt.Sprintf("candidate would JOIN %d shared-assumption cluster(s):\n", len(r.SharedAssumptionHits)))
		for i := range r.SharedAssumptionHits {
			writeStructuralCandidate(&b, &r.SharedAssumptionHits[i])
		}
	}
	if len(r.AxisCoReferenceHits) > 0 {
		b.WriteString(fmt.Sprintf("candidate would CO-REFERENCE %d existing axis group(s):\n", len(r.AxisCoReferenceHits)))
		for i := range r.AxisCoReferenceHits {
			writeStructuralCandidate(&b, &r.AxisCoReferenceHits[i])
		}
	}
	return b.String()
}

func writeStructuralCandidate(b *strings.Builder, c *diagnose.Candidate) {
	b.WriteString(fmt.Sprintf("  - %s: members [%s]\n", c.ID, strings.Join(c.Members, ", ")))
	b.WriteString(fmt.Sprintf("     %s\n", c.Evidence))
}

// readConfrontCandidate resolves the candidate text from either --file <path>
// or the positional args (joined with spaces). Exactly one source must be
// non-empty; a mix or a total absence is a usage error. (- as a stdin marker is
// deliberately NOT supported: the project's reorderFlagsFirst heuristic treats
// a lone "-" as a positional, so "hotam confront --file -" would silently split
// the "-" away from its flag and misparse. Long drafts go to a real file path.)
func readConfrontCandidate(file string, positional []string) (string, error) {
	if file != "" {
		if len(positional) > 0 {
			return "", fmt.Errorf("confront: pass either --file OR a positional text argument, not both")
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return "", fmt.Errorf("read --file %s: %w", file, err)
		}
		return string(data), nil
	}
	if len(positional) == 0 {
		return "", fmt.Errorf("confront requires candidate text: pass it as a quoted positional argument, or via --file <path>")
	}
	return strings.Join(positional, " "), nil
}

// formatConfrontReport renders lexical/metadata suspicions and explicit formal
// Conflict carriers separately so advisory evidence is never presented as a
// semantic verdict.
func formatConfrontReport(r diagnose.ConfrontResult) string {
	var b strings.Builder
	b.WriteString("hotam confront — lexical/metadata suspicions are advisory; only an explicit unresolved Conflict carrier is a gate blocker.\n")
	b.WriteString(fmt.Sprintf("candidate: %s\n\n", confrontCandidatePreview(r.Candidate)))
	if r.Clear {
		b.WriteString("no lexical, metadata-linked, or unresolved formal-conflict evidence found.\n")
		return b.String()
	}
	if len(r.Settled) > 0 {
		b.WriteString(fmt.Sprintf("possible overlap with %d SETTLED requirement(s):\n", len(r.Settled)))
		for _, h := range r.Settled {
			writeConfrontHit(&b, &h)
		}
		b.WriteString("\n")
	}
	if len(r.Rejected) > 0 {
		b.WriteString(fmt.Sprintf("possible overlap with %d REJECTED requirement(s):\n", len(r.Rejected)))
		for _, h := range r.Rejected {
			writeConfrontHit(&b, &h)
			if len(h.ReplacedBy) > 0 {
				b.WriteString(fmt.Sprintf("     replaced by: %s (see docs/gen/HISTORY.md)\n", strings.Join(h.ReplacedBy, ", ")))
			} else {
				b.WriteString("     no known REPLACES successor; see docs/gen/HISTORY.md.\n")
			}
		}
		b.WriteString("\n")
	}
	if len(r.FormalConflicts) > 0 {
		b.WriteString(fmt.Sprintf("unresolved formal Conflict carrier(s): %d (these are the only semantic-gate blockers)\n", len(r.FormalConflicts)))
		for _, h := range r.FormalConflicts {
			writeConfrontHit(&b, &h)
		}
	}
	return b.String()
}

func writeConfrontHit(b *strings.Builder, h *diagnose.ConfrontHit) {
	b.WriteString(fmt.Sprintf("  - %s [%s; confidence=%s] (score %d): %s\n", h.ID, h.Classification, h.Confidence, h.Score, h.Claim))
	b.WriteString(fmt.Sprintf("     shared tokens: [%s]\n", strings.Join(h.Shared, ", ")))
	for _, reason := range h.Reasons {
		b.WriteString("     reason: " + reason + "\n")
	}
	if len(h.ConflictIDs) > 0 {
		b.WriteString(fmt.Sprintf("     conflicts: [%s]\n", strings.Join(h.ConflictIDs, ", ")))
	}
}

// confrontCandidatePreview renders a single-line preview of the candidate text,
// collapsed to one space-separated line and rune-truncated so a multi-line
// --file draft does not blow up the report header.
func confrontCandidatePreview(s string) string {
	oneLine := strings.Join(strings.Fields(s), " ")
	return runeTruncatePreview(oneLine, 100)
}

func runeTruncatePreview(s string, keep int) string {
	if len([]rune(s)) <= keep {
		return s
	}
	return string([]rune(s)[:keep]) + "…"
}
