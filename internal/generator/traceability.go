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
	"github.com/PHPCraftdream/HotamSpec/internal/localization"
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
	// test body -- a CHEAP, always-on signal (no `go test` execution). This
	// is the ONLY scenario signal this document renders: BuildTraceability
	// is a pure, mode-independent function of the graph plus a read-only AST
	// scan, so this cell renders byte-identically on every `gen-spec` run
	// regardless of --spec. The REAL, executed narrative and pass/fail state
	// lives in docs/gen/SPEC.md (hotam gen-spec --spec), whose own freshness
	// is separately enforced by check_spec_md_current -- see this file's
	// package doc comment.
	hasScenario bool
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
// SAME resolveTraceabilityLinks call this function always made -- so
// `gen-spec` gains this column at ZERO extra cost, no `go test` execution.
// This is the ONLY scenario signal this document ever renders: BuildTraceability
// is a pure function of the graph plus a read-only AST scan, byte-identical
// on every `gen-spec` run regardless of mode. The REAL, executed narrative
// and pass/fail state lives in docs/gen/SPEC.md (rendered only by
// `hotam gen-spec --spec`, whose freshness is separately enforced by
// check_spec_md_current) -- this document never overlays that real outcome,
// so it stays mode-independent (a routine `hotam land` regeneration always
// calls this function the same way SPEC.md's --spec run does, and both must
// produce the identical committed bytes).
// TraceabilityMDHasContent reports whether TRACEABILITY.md carries real
// domain content — i.e. whether the graph is non-empty (task #364: withheld
// entirely from the docs/gen/ write set when the domain has zero axes/
// stakeholders/requirements/conflicts/assumptions/operators/processes/goals/
// entity_types/entities, mirroring the conditional-write pattern
// DECISIONS.md/ENTITIES.md/TENSIONS.md/PIPELINE.md/MODELS.md already use). A
// genuinely empty domain has nothing to render but the EmptyNotice
// placeholder BuildTraceability already falls back to (g.IsEmpty()) — so a
// fresh, unmodeled domain now produces ZERO files under docs/gen/, not a
// directory full of calm-but-empty placeholders.
func TraceabilityMDHasContent(g *ontology.Graph) bool {
	return !g.IsEmpty()
}

func BuildTraceability(g *ontology.Graph) string {
	lines := docHeaderLines("TRACEABILITY", g)
	lines = append(lines, serviceText(g, "# TRACEABILITY.md — requirement -> implemented_by -> verified_by (Hotam-Spec)"))
	lines = append(lines, "")
	lines = append(lines,
		serviceText(g, "Generated from `implemented_by`/`verified_by` on each requirement in this domain's `graph.json` (PLAN-authored-spec-discipline.md §4/§7).")+" "+
			serviceText(g, "Each authored link is RE-RESOLVED here (same resolver the mechanical gate uses — internal/gate/spec_resolver.go) purely for display: `resolves` means the named file:symbol / file:test was found by parsing that file; `ORPHANED` means it was not (stale reference, typo, or renamed/deleted symbol) — the mechanical gate (internal/invariants/authored_links.go) is the actual enforcement point, this doc only reports its verdict for navigation.")+" "+
			serviceText(g, "The `scenario` column (PLAN-scenario-generated-spec.md §3 W1.4) is a CHEAP, AST-only signal (no test execution) that a verified_by test's body calls `hotamspec.NewScenario(...)` — this is the only scenario signal this document ever renders, so it stays byte-identical on every `gen-spec` run.")+" "+
			serviceText(g, "The REAL, executed narrative — Given/When/Then/Value steps from an actually-passing `go test` run — lives in `docs/gen/SPEC.md`, generated only by `hotam gen-spec --spec` (real, but expensive: a full compile+run per verified_by entry); that file's own freshness is separately enforced by `check_spec_md_current`, so this document does not need to (and must not) overlay its outcome here."),
	)
	lines = append(lines, "")

	if g.IsEmpty() {
		lines = append(lines, localizedEmptyNotice(g))
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

	lines = append(lines, serviceText(g, "## Authored-linked requirements"))
	lines = append(lines, "")
	if len(linked) == 0 {
		lines = append(lines, serviceText(g, "_No requirement in this domain carries an `implemented_by` or `verified_by` entry yet — the authored-spec layer (PLAN-authored-spec-discipline.md §3) has not been started for this domain._"))
		lines = append(lines, "")
	} else {
		lines = append(lines, serviceText(g, "| id | status | implemented_by | verified_by | claim |"))
		lines = append(lines, "|---|---|---|---|---|")
		for _, row := range linked {
			implCell := renderTraceabilityLinks(row.implementedRes, g.RenderLanguage)
			verifCell := renderTraceabilityLinks(row.verifiedRes, g.RenderLanguage)
			lines = append(lines, "| `"+row.req.ID+"` | "+Cell(serviceText(g, row.req.Status))+" | "+implCell+" | "+verifCell+" | "+Cell(requirementClaim(g, row.req))+" |")
		}
		lines = append(lines, "")
	}

	lines = append(lines, serviceText(g, "## Engine-enforced (enforced_by, no authored carrier)"))
	lines = append(lines, "")
	lines = append(lines, serviceText(g, "SETTLED+ENFORCED requirements proven by the engine mechanism (a `check_*` invariant or repo-wide `Test*` function named in `enforced_by`) rather than a domain-authored `spec/` symbol+test pair. Typical for a domain's own methodology/framework requirements (`hotam-spec-self`) whose \"code\" IS the engine."))
	lines = append(lines, "")
	if len(engineEnforced) == 0 {
		lines = append(lines, serviceText(g, "_None in this domain._"))
		lines = append(lines, "")
	} else {
		lines = append(lines, serviceText(g, "| id | enforced_by | claim |"))
		lines = append(lines, "|---|---|---|")
		for _, r := range engineEnforced {
			lines = append(lines, "| `"+r.ID+"` | "+Cell(strings.Join(r.EnforcedBy, ", "))+" | "+Cell(requirementClaim(g, r))+" |")
		}
		lines = append(lines, "")
	}

	lines = append(lines, serviceText(g, "## Prose / roadmap-debt (no code carrier yet)"))
	lines = append(lines, "")
	lines = append(lines, serviceText(g, "Requirements with no `implemented_by`/`verified_by` AND no `enforced_by` — honest discipline/roadmap-debt per PLAN-authored-spec-discipline.md §5: a requirement may be SETTLED without code, but is not yet traceable to a real carrier."))
	lines = append(lines, "")
	if len(prose) == 0 {
		lines = append(lines, serviceText(g, "_None in this domain._"))
		lines = append(lines, "")
	} else {
		lines = append(lines, serviceText(g, "| id | status | enforcement | claim |"))
		lines = append(lines, "|---|---|---|---|")
		for _, r := range prose {
			lines = append(lines, "| `"+r.ID+"` | "+Cell(localizedStatus(g, r.Status))+" | "+Cell(localizedStatus(g, r.Enforcement))+" | "+Cell(requirementClaim(g, r))+" |")
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

// renderTraceabilityLinks renders one cell of the authored-linked table: each
// entry as a clickable-looking backticked path, tagged ✓ when it resolved
// and ORPHANED (plus a short reason) when it did not, joined with line
// breaks (<br>) so a requirement with multiple entries stays one table row.
// A resolving verified_by entry additionally carries its scenario signal
// (W1.4): "scenario" when the cheap AST scan found `hotamspec.NewScenario`
// in the test body -- an implemented_by entry never sets hasScenario (zero
// value), so this branch is silently skipped for that table, keeping this
// one render function shared without a caller-side conditional. This is the
// only scenario signal ever rendered here (no real executed verdict overlay
// -- see BuildTraceability's own doc comment for why), so this function's
// output is a pure function of its links argument, mode-independent.
func renderTraceabilityLinks(links []traceabilityLink, language string) string {
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
				reason = localization.Text(language, "not found")
			}
			cell += " — **" + localization.Text(language, "ORPHANED") + "** (" + reason + ")"
		case l.detail != "":
			cell += " — " + localization.Text(language, "resolves") + " (" + l.detail + ")"
		default:
			cell += " — " + localization.Text(language, "resolves")
		}
		if l.resolved && l.hasScenario {
			cell += " · " + localization.Text(language, "scenario")
		}
		parts = append(parts, cell)
	}
	return strings.Join(parts, "<br>")
}
