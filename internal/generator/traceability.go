// traceability.go renders docs/gen/TRACEABILITY.md: the generated projection
// PLAN-authored-spec-discipline.md §7 names as the navigation/trace surface
// for the authored-spec discipline (§4's requirement -> implemented_by
// (file:symbol) -> verified_by (file:test) schema). It exists so an agent or
// human can find, for any requirement carrying authored links, WHERE that
// requirement is embodied and WHERE it is proven without grepping graph.json
// by hand -- and so the same resolution the mechanical gate performs
// (internal/gate/spec_resolver.go, internal/invariants/authored_links.go) is
// visible as a human-readable status (resolves / orphaned) instead of only
// failing a check silently.
//
// This file is read-only over the graph: it re-resolves each
// implemented_by/verified_by entry via gate.ResolveSpecSymbol/ResolveSpecTest
// (the same resolver the invariants layer uses) purely to report status —
// it never mutates the graph and is not itself an enforcement gate (that
// remains internal/invariants/authored_links.go's job; a doc projection must
// not be a second source of truth for pass/fail).
package generator

import (
	"strconv"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// traceabilityRow is one rendered requirement row: its own id/claim plus the
// resolved status of every implemented_by and verified_by entry it carries.
type traceabilityRow struct {
	req            ontology.Requirement
	implementedRes []traceabilityLink
	verifiedRes    []traceabilityLink
}

// traceabilityLink is one implemented_by/verified_by entry plus whether it
// resolved against the domain's spec root.
type traceabilityLink struct {
	raw      string
	resolved bool
	detail   string // short reason when not resolved (parse error / not found)
	// hasScenario is set only for a verified_by entry: whether the AST-only
	// scan (gate.ResolveSpecTest's HasScenario, PLAN-scenario-generated-
	// spec.md §3 W1.4) found a `hotamspec.NewScenario(...)` call in the
	// test body -- a CHEAP, always-on signal (no `go test` execution),
	// distinct from verdict below.
	hasScenario bool
	// verdict is the REAL, executed outcome (ScenarioVerdictsFromRows,
	// gen-spec --spec only) -- "" when no verdicts map was supplied to
	// BuildTraceability (the default, cheap gen-spec path never executes
	// tests to fill this in).
	verdict string
}

// BuildTraceability renders docs/gen/TRACEABILITY.md: for every requirement
// carrying a non-empty implemented_by or verified_by, a row naming the
// requirement, its implemented_by (file:symbol) entries, its verified_by
// (file:test) entries, and each entry's resolution status (resolves /
// orphaned) against gate.SpecRootForGraph(g) -- the same self-hosting-aware
// root internal/invariants/authored_links.go resolves against, so an
// engine-facing requirement (g.SelfHosting) resolves its
// "internal/ontology/lifecycle.go:Lifecycle"-shaped entries against the
// engine repository root, and an ordinary domain resolves its
// "spec/model/risk.go:NewRisk"-shaped entries against its own domainDir.
//
// Requirements with NEITHER field populated are listed separately, split by
// SETTLED+ENFORCED-via-enforced_by (engine-enforced) vs everything else
// (prose/roadmap-debt with no code carrier yet) -- so the doc is an honest
// full partition of the roster, not just a spotlight on the authored-linked
// minority.
//
// Scenario column (PLAN-scenario-generated-spec.md §3 W1.4): every
// verified_by entry additionally reports whether it carries a
// hotamspec-recorder scenario -- CHEAPLY, via gate.ResolveSpecTest's
// AST-only HasScenario detection (a `hotamspec.NewScenario(...)` call in
// the test body), which BuildTraceability already gets for free from the
// SAME resolveTraceabilityLinks call this function always made -- so the
// default (no --spec) `gen-spec` gains this column at ZERO extra cost, no
// `go test` execution. verdicts is OPTIONAL (nil on every pre-existing
// caller and on a default `gen-spec` run): when supplied (only
// `gen-spec --spec`, populated once via generator.CollectSpecRows +
// generator.ScenarioVerdictsFromRows and shared with BuildSpec/BuildCoverage
// in the SAME invocation, see cmd/hotam/gen_spec.go), each linked
// requirement's verified_by cell additionally names the REAL recorded
// verdict (narrated+pass / narrated-but-another-entry-not-passing /
// no-narrative) instead of only the AST guess.
func BuildTraceability(g *ontology.Graph, verdicts ...map[string]ScenarioVerdict) string {
	var verdictMap map[string]ScenarioVerdict
	if len(verdicts) > 0 {
		verdictMap = verdicts[0]
	}

	lines := []string{Banner, ReaderHeaderLine("TRACEABILITY", g), ""}
	lines = append(lines, "# TRACEABILITY.md — requirement -> implemented_by -> verified_by (Hotam-Spec)")
	lines = append(lines, "")
	lines = append(lines,
		"Generated from `implemented_by`/`verified_by` on each requirement in this domain's "+
			"`graph.json` (PLAN-authored-spec-discipline.md §4/§7). Each authored link is "+
			"RE-RESOLVED here (same resolver the mechanical gate uses — "+
			"internal/gate/spec_resolver.go) purely for display: `resolves` means the named "+
			"file:symbol / file:test was found by parsing that file; `ORPHANED` means it was "+
			"not (stale reference, typo, or renamed/deleted symbol) — the mechanical gate "+
			"(internal/invariants/authored_links.go) is the actual enforcement point, this doc "+
			"only reports its verdict for navigation. The `scenario` column (PLAN-scenario-"+
			"generated-spec.md §3 W1.4) is a CHEAP, AST-only signal (no test execution) that a "+
			"verified_by test's body calls `hotamspec.NewScenario(...)`; a `verdict` in "+
			"parentheses (narrated, pass / narrated, another entry not passing / no narrative "+
			"recorded) only appears "+
			"when this doc was generated via `hotam gen-spec --spec`, which actually EXECUTES "+
			"every verified_by test to record it (real, but expensive) — a default `gen-spec` "+
			"run never pays that cost and this doc says so plainly rather than guessing.")
	lines = append(lines, "")

	if g.IsEmpty() {
		lines = append(lines, EmptyNotice)
		lines = append(lines, "")
		return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n") + "\n"
	}

	specRoot := gate.SpecRootForGraph(g)
	reqs := NarrativeOrder(g.Requirements, func(r ontology.Requirement) int { return r.DeclOrder })

	var linked []traceabilityRow
	var engineEnforced, prose []ontology.Requirement
	for _, r := range reqs {
		hasImpl := len(r.ImplementedBy) > 0
		hasVerif := len(r.VerifiedBy) > 0
		if !hasImpl && !hasVerif {
			if r.Status == ontology.StatusSETTLED && r.Enforcement == ontology.EnforcementENFORCED && len(r.EnforcedBy) > 0 {
				engineEnforced = append(engineEnforced, r)
			} else {
				prose = append(prose, r)
			}
			continue
		}
		verifiedRes := resolveTraceabilityLinks(specRoot, r.VerifiedBy, false)
		if v, ok := verdictMap[r.ID]; ok {
			applyScenarioVerdict(verifiedRes, v)
		}
		linked = append(linked, traceabilityRow{
			req:            r,
			implementedRes: resolveTraceabilityLinks(specRoot, r.ImplementedBy, true),
			verifiedRes:    verifiedRes,
		})
	}

	lines = append(lines,
		"**"+strconv.Itoa(len(linked))+" requirement(s) carry authored links; "+
			strconv.Itoa(len(engineEnforced))+" are engine-enforced (enforced_by, no authored carrier); "+
			strconv.Itoa(len(prose))+" are prose/roadmap-debt (no code carrier yet).**")
	lines = append(lines, "")
	lines = append(lines, "---")
	lines = append(lines, "")

	lines = append(lines, "## Authored-linked requirements")
	lines = append(lines, "")
	if len(linked) == 0 {
		lines = append(lines, "_No requirement in this domain carries an `implemented_by` or `verified_by` entry yet — the authored-spec layer (PLAN-authored-spec-discipline.md §3) has not been started for this domain._")
		lines = append(lines, "")
	} else {
		lines = append(lines, "| id | status | implemented_by | verified_by | claim |")
		lines = append(lines, "|---|---|---|---|---|")
		for _, row := range linked {
			implCell := renderTraceabilityLinks(row.implementedRes)
			verifCell := renderTraceabilityLinks(row.verifiedRes)
			lines = append(lines, "| `"+row.req.ID+"` | "+Cell(row.req.Status)+" | "+implCell+" | "+verifCell+" | "+Cell(row.req.Claim)+" |")
		}
		lines = append(lines, "")
	}

	lines = append(lines, "## Engine-enforced (enforced_by, no authored carrier)")
	lines = append(lines, "")
	lines = append(lines,
		"SETTLED+ENFORCED requirements proven by the engine mechanism (a `check_*` invariant or "+
			"repo-wide `Test*` function named in `enforced_by`) rather than a domain-authored "+
			"`spec/` symbol+test pair. Typical for a domain's own methodology/framework "+
			"requirements (`hotam-spec-self`) whose \"code\" IS the engine.")
	lines = append(lines, "")
	if len(engineEnforced) == 0 {
		lines = append(lines, "_None in this domain._")
		lines = append(lines, "")
	} else {
		lines = append(lines, "| id | enforced_by | claim |")
		lines = append(lines, "|---|---|---|")
		for _, r := range engineEnforced {
			lines = append(lines, "| `"+r.ID+"` | "+Cell(strings.Join(r.EnforcedBy, ", "))+" | "+Cell(r.Claim)+" |")
		}
		lines = append(lines, "")
	}

	lines = append(lines, "## Prose / roadmap-debt (no code carrier yet)")
	lines = append(lines, "")
	lines = append(lines,
		"Requirements with no `implemented_by`/`verified_by` AND no `enforced_by` — honest "+
			"discipline/roadmap-debt per PLAN-authored-spec-discipline.md §5: a requirement may "+
			"be SETTLED without code, but is not yet traceable to a real carrier.")
	lines = append(lines, "")
	if len(prose) == 0 {
		lines = append(lines, "_None in this domain._")
		lines = append(lines, "")
	} else {
		lines = append(lines, "| id | status | enforcement | claim |")
		lines = append(lines, "|---|---|---|---|")
		for _, r := range prose {
			lines = append(lines, "| `"+r.ID+"` | "+Cell(r.Status)+" | "+Cell(r.Enforcement)+" | "+Cell(r.Claim)+" |")
		}
		lines = append(lines, "")
	}

	return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n") + "\n"
}

// resolveTraceabilityLinks parses each raw "file:symbol"/"file:test" entry
// and re-resolves it against specRoot using the same gate.ResolveSpecSymbol
// (isSymbol == true, for implemented_by) / gate.ResolveSpecTest (isSymbol ==
// false, for verified_by) the mechanical invariants use, so this doc's
// resolves/ORPHANED verdict never diverges from the actual gate's.
func resolveTraceabilityLinks(specRoot string, raw []string, isSymbol bool) []traceabilityLink {
	out := make([]traceabilityLink, 0, len(raw))
	for _, entry := range raw {
		trimmed := strings.TrimSpace(entry)
		file, symbol, ok := gate.ParseFileColonSymbol(trimmed)
		if !ok {
			out = append(out, traceabilityLink{raw: trimmed, resolved: false, detail: "malformed (expected file:symbol)"})
			continue
		}
		if isSymbol {
			result, err := gate.ResolveSpecSymbol(specRoot, file, symbol)
			if err != nil {
				out = append(out, traceabilityLink{raw: trimmed, resolved: false, detail: "parse error"})
				continue
			}
			out = append(out, traceabilityLink{raw: trimmed, resolved: result.Found()})
			continue
		}
		result, err := gate.ResolveSpecTest(specRoot, file, symbol)
		if err != nil {
			out = append(out, traceabilityLink{raw: trimmed, resolved: false, detail: "parse error"})
			continue
		}
		detail := ""
		if result.Found && !result.HasTeeth {
			detail = "no teeth"
		} else if result.Found && result.HasSkip {
			detail = "unconditional skip"
		}
		// HasScenario comes from the SAME AST walk ResolveSpecTest already
		// performed above (gate.SpecTestResult.HasScenario, W1.4) -- no
		// second parse, no test execution: a verified_by cell's scenario
		// signal is free relative to what this function already computed
		// for the resolves/ORPHANED verdict.
		out = append(out, traceabilityLink{raw: trimmed, resolved: result.Found, detail: detail, hasScenario: result.Found && result.HasScenario})
	}
	return out
}

// applyScenarioVerdict overlays the REAL, executed ScenarioVerdict (gen-spec
// --spec only, ScenarioVerdictsFromRows) onto verifiedRes's cells IN PLACE,
// so renderTraceabilityLinks can render the actually-recorded outcome
// instead of only the AST guess -- every verified_by cell for the SAME
// requirement shares one ScenarioVerdict (the verdict is a per-requirement
// reduction across all its verified_by entries, see ScenarioVerdictsFromRows'
// own doc comment), so the same v is stamped onto every resolving link.
func applyScenarioVerdict(verifiedRes []traceabilityLink, v ScenarioVerdict) {
	for i := range verifiedRes {
		if !verifiedRes[i].resolved {
			continue
		}
		switch {
		case v.Narrated && v.AllEntriesPass:
			verifiedRes[i].verdict = "narrated, pass"
		case v.Narrated:
			verifiedRes[i].verdict = "narrated, another entry not passing"
		default:
			verifiedRes[i].verdict = "no narrative recorded"
		}
	}
}

// renderTraceabilityLinks renders one cell of the authored-linked table: each
// entry as a clickable-looking backticked path, tagged ✓ when it resolved
// and ORPHANED (plus a short reason) when it did not, joined with line
// breaks (<br>) so a requirement with multiple entries stays one table row.
// A resolving verified_by entry additionally carries its scenario signal
// (W1.4): "scenario" when the cheap AST scan found `hotamspec.NewScenario`
// in the test body, plus a real recorded verdict in parentheses when one was
// supplied (applyScenarioVerdict, gen-spec --spec only) -- an
// implemented_by entry never sets hasScenario/verdict (zero values), so this
// branch is silently skipped for that table, keeping this one render
// function shared without a caller-side conditional.
func renderTraceabilityLinks(links []traceabilityLink) string {
	if len(links) == 0 {
		return "—"
	}
	parts := make([]string, 0, len(links))
	for _, l := range links {
		cell := "`" + l.raw + "`"
		switch {
		case !l.resolved:
			reason := l.detail
			if reason == "" {
				reason = "not found"
			}
			cell += " — **ORPHANED** (" + reason + ")"
		case l.detail != "":
			cell += " — resolves (" + l.detail + ")"
		default:
			cell += " — resolves"
		}
		if l.resolved {
			if l.hasScenario {
				cell += " · scenario"
				if l.verdict != "" {
					cell += " (" + l.verdict + ")"
				}
			} else if l.verdict != "" {
				// A verdicts map was supplied but this entry's own AST scan
				// found no hotamspec.NewScenario call -- report the verdict
				// alone (it will read "no narrative recorded", consistent
				// with the AST guess) rather than silently dropping it.
				cell += " · " + l.verdict
			}
		}
		parts = append(parts, cell)
	}
	return strings.Join(parts, "<br>")
}
