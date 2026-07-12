package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/PHPCraftdream/HotamSpecGo/internal/diagnose"
)

// cmdConfront implements `hotam confront <text> [--domain <path>] [--file
// <path>] [--json]`: the CONFRONT step of the mediation loop, made executable.
//
// CONFRONT checks a candidate claim (a draft the operator is about to propose)
// against the graph's SETTLED reality (duplicate guard) and REJECTED history
// (anti-relitigation guard) BEFORE anything is written. It reuses the SAME
// lexical-overlap engine as `hotam inspect` (internal/diagnose.Confront, backed
// by claimTokens / markerHits / the MinLexicalOverlap* thresholds) — no new
// scoring was invented for this command.
//
// The candidate text comes either from a positional argument (quoted) or from
// --file <path> (- = stdin) for long drafts. Output is human-readable by
// default; --json emits machine-readable ConfrontResult JSON. Exit code is
// ALWAYS 0: confront informs, it never gates (R-ai-presents-not-decides) — a
// high-overlap hit is a warning to the operator, not a block.
func cmdConfront(args []string) error {
	fs := newFlagSet("confront")
	domain := fs.String("domain", "", "domain directory (default: "+defaultDomainRel+")")
	file := fs.String("file", "", "read candidate text from this file (use \"-\" for stdin)")
	asJSON := fs.Bool("json", false, "emit machine-readable JSON")
	fs.Parse(args)

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

// formatConfrontReport renders the human-readable CONFRONT report. It always
// produces an explicit verdict — either the hits (grouped SETTLED then
// REJECTED, each with id/score/shared-tokens/claim, plus replaced_by for
// REJECTED) or the "clear to propose" green light. Silence is never the output:
// the operator must know the check ran and what it concluded.
func formatConfrontReport(r diagnose.ConfrontResult) string {
	var b strings.Builder
	b.WriteString("hotam confront — CONFRONT check (advisory; exit code always 0).\n")
	b.WriteString(fmt.Sprintf("candidate: %s\n", confrontCandidatePreview(r.Candidate)))
	b.WriteString("\n")
	if r.Clear {
		b.WriteString("no significant overlap with SETTLED or REJECTED — clear to propose.\n")
		return b.String()
	}
	if len(r.Settled) > 0 {
		b.WriteString(fmt.Sprintf("possible DUPLICATE of %d SETTLED requirement(s):\n", len(r.Settled)))
		for _, h := range r.Settled {
			writeConfrontHit(&b, &h)
		}
		b.WriteString("\n")
	}
	if len(r.Rejected) > 0 {
		b.WriteString(fmt.Sprintf("possible RE-LITIGATION of %d REJECTED requirement(s):\n", len(r.Rejected)))
		for _, h := range r.Rejected {
			writeConfrontHit(&b, &h)
			if len(h.ReplacedBy) > 0 {
				b.WriteString(fmt.Sprintf("     replaced by: %s (cite the replacement instead of re-deriving; see docs/gen/HISTORY.md)\n", strings.Join(h.ReplacedBy, ", ")))
			} else {
				b.WriteString("     no known REPLACES successor; see docs/gen/HISTORY.md for why it was rejected.\n")
			}
		}
	}
	return b.String()
}

func writeConfrontHit(b *strings.Builder, h *diagnose.ConfrontHit) {
	b.WriteString(fmt.Sprintf("  - %s (score %d): %s\n", h.ID, h.Score, h.Claim))
	b.WriteString(fmt.Sprintf("     shared tokens: [%s]\n", strings.Join(h.Shared, ", ")))
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
