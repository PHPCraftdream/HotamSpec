// coverage.go renders docs/gen/COVERAGE.md: the generated projection
// PLAN-authored-spec-discipline.md §7 names as the honest picture of
// authored-spec discipline for a domain -- which SETTLED+ENFORCEABLE
// requirements have a real implemented_by+verified_by (or enforced_by)
// carrier versus roadmap debt, and which LAYER (models -> fields -> methods
// -> tests, §4/§8 step 3-6, R-authored-spec-layer-progression) the domain's
// authored spec/ tree currently sits at.
//
// This file is read-only over the graph and the filesystem: the carrier
// breakdown re-resolves each requirement's implemented_by/verified_by via
// the same gate.ResolveSpecSymbol/ResolveSpecTest resolver
// traceability.go uses, and the layer counts reuse models.go's own
// go/ast scan (ScanModelLayerCounts) so the two projections never disagree
// about what a domain's spec/ tree declares. It never mutates the graph and
// is not itself an enforcement gate (that remains
// internal/invariants/authored_links.go's job; a doc projection must not be
// a second source of truth for pass/fail).
package generator

import (
	"strconv"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// BuildCoverage renders docs/gen/COVERAGE.md: for every SETTLED requirement,
// whether it carries a real carrier (authored implemented_by+verified_by,
// re-resolved against gate.SpecRootForGraph(g); or engine enforced_by) versus
// roadmap debt (SETTLED, ENFORCEABLE, no carrier yet) versus permanent
// discipline (SETTLED, INHERENTLY_PROSE) -- plus the domain's current
// authored-spec layer per PLAN-authored-spec-discipline.md §4/§8 step 3-6,
// read from the same go/ast scan models.go performs.
func BuildCoverage(g *ontology.Graph) string {
	lines := []string{Banner, ReaderHeaderLine("COVERAGE", g), ""}
	lines = append(lines, "# COVERAGE.md — authored-spec discipline coverage (Hotam-Spec)")
	lines = append(lines, "")
	lines = append(lines,
		"Generated from this domain's `graph.json` plus its authored `spec/` tree "+
			"(PLAN-authored-spec-discipline.md §4/§7): which SETTLED requirements have "+
			"a real carrier (authored `implemented_by`+`verified_by`, re-resolved here "+
			"the same way TRACEABILITY.md does; or engine `enforced_by`) versus honest "+
			"roadmap debt versus permanent discipline, and which LAYER (models -> "+
			"fields -> methods -> tests, §4/§8 step 3-6) this domain's authored spec/ "+
			"tree currently sits at, read from the same `go/ast` scan MODELS.md "+
			"performs. Not an enforcement gate itself — "+
			"`internal/invariants/authored_links.go` is the actual mechanical floor; "+
			"this doc only reports its verdict for orientation "+
			"(R-authored-spec-projections-are-derived).")
	lines = append(lines, "")

	if g.IsEmpty() {
		lines = append(lines, EmptyNotice)
		lines = append(lines, "")
		return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n") + "\n"
	}

	specRoot := gate.SpecRootForGraph(g)
	reqs := NarrativeOrder(g.Requirements, func(r ontology.Requirement) int { return r.DeclOrder })

	var settled []ontology.Requirement
	for _, r := range reqs {
		if r.Status == ontology.StatusSETTLED {
			settled = append(settled, r)
		}
	}

	// The carrier split gates on Enforcement == ENFORCED FIRST (the actual
	// disjunctive gate, R-enforced-requires-enforcer-or-authored-link,
	// promotes a requirement to ENFORCED only when a real carrier exists),
	// THEN splits authored vs engine by which half of the disjunction it
	// used. Everything NOT ENFORCED is SETTLED debt, split by Enforceability
	// (mirrors UNENFORCED.md's own split) -- so a STRUCTURAL requirement that
	// happens to carry a stray enforced_by (e.g. a INHERENTLY_PROSE
	// discipline note referencing a related check_* for context) still lands
	// in permanent-discipline, not engine-carrier, since it never reached
	// ENFORCED.
	var authoredCarrier, engineCarrier, roadmapDebt, permanentDiscipline []ontology.Requirement
	for _, r := range settled {
		switch {
		case r.Enforcement == ontology.EnforcementENFORCED && authoredLinksAllResolve(specRoot, r):
			authoredCarrier = append(authoredCarrier, r)
		case r.Enforcement == ontology.EnforcementENFORCED && len(r.EnforcedBy) > 0:
			engineCarrier = append(engineCarrier, r)
		case r.Enforceability == ontology.EnforceabilityINHERENTLY_PROSE:
			permanentDiscipline = append(permanentDiscipline, r)
		default:
			roadmapDebt = append(roadmapDebt, r)
		}
	}

	layer, layerErr := ScanModelLayerCounts(g)

	lines = append(lines,
		"**"+strconv.Itoa(len(settled))+" SETTLED requirement(s): "+
			strconv.Itoa(len(authoredCarrier))+" authored-carrier, "+
			strconv.Itoa(len(engineCarrier))+" engine-carrier, "+
			strconv.Itoa(len(roadmapDebt))+" roadmap-debt, "+
			strconv.Itoa(len(permanentDiscipline))+" permanent discipline.**")
	lines = append(lines, "")
	lines = append(lines, "---")
	lines = append(lines, "")

	lines = append(lines, "## Layer — where this domain's authored spec/ tree currently sits")
	lines = append(lines, "")
	lines = append(lines,
		"Per PLAN-authored-spec-discipline.md §4 (refined by "+
			"R-authored-spec-layer-progression, §8 step 3-6): a domain is founded "+
			"general-before-specific — ALL models before any one model's fields, ALL "+
			"fields before any one model's methods, ALL methods before tests. Counts "+
			"below are read from the same `go/ast` scan MODELS.md performs "+
			"(`ScanModelLayerCounts`).")
	lines = append(lines, "")
	if layerErr != nil {
		lines = append(lines, "_Layer could not be determined: "+layerErr.Error()+"_")
		lines = append(lines, "")
	} else if layer.Files == 0 {
		lines = append(lines, "**Layer 0 — no authored spec/ yet.** No authored model file was found "+
			"(ordinary domain: nothing under `spec/`; self-hosting domain: no requirement "+
			"names an engine file via `implemented_by`/`verified_by` yet). "+
			"PLAN-authored-spec-discipline.md §8 step 3 has not started.")
		lines = append(lines, "")
	} else {
		lines = append(lines, "| files scanned | objects (models) | fields | methods | typed errors |")
		lines = append(lines, "|---|---|---|---|---|")
		lines = append(lines, "| "+strconv.Itoa(layer.Files)+" | "+strconv.Itoa(layer.Objects)+" | "+
			strconv.Itoa(layer.Fields)+" | "+strconv.Itoa(layer.Methods)+" | "+strconv.Itoa(layer.Errors)+" |")
		lines = append(lines, "")
		lines = append(lines, "**"+layerVerdict(layer)+"**")
		lines = append(lines, "")
	}

	lines = append(lines, "## Authored-carrier (implemented_by + verified_by both resolve)")
	lines = append(lines, "")
	lines = append(lines,
		"SETTLED requirements with a real authored `spec/` symbol AND a real, "+
			"non-vacuous, non-skipped test — the authored path of the disjunctive "+
			"ENFORCED gate (R-enforced-requires-enforcer-or-authored-link).")
	lines = append(lines, "")
	lines = append(lines, renderCoverageAuthoredTable(specRoot, authoredCarrier))

	lines = append(lines, "## Engine-carrier (enforced_by, no authored link)")
	lines = append(lines, "")
	lines = append(lines,
		"SETTLED requirements proven by the engine mechanism (a `check_*` invariant "+
			"or repo-wide `Test*` function named in `enforced_by`) — the OTHER path of "+
			"the same disjunctive gate. Typical for a domain's own methodology/"+
			"framework requirements whose \"code\" IS the engine.")
	lines = append(lines, "")
	if len(engineCarrier) == 0 {
		lines = append(lines, "_None in this domain._")
		lines = append(lines, "")
	} else {
		lines = append(lines, "| id | enforced_by | claim |")
		lines = append(lines, "|---|---|---|")
		for _, r := range engineCarrier {
			lines = append(lines, "| `"+r.ID+"` | "+Cell(strings.Join(r.EnforcedBy, ", "))+" | "+Cell(r.Claim)+" |")
		}
		lines = append(lines, "")
	}

	lines = append(lines, "## Roadmap debt (SETTLED, ENFORCEABLE, no carrier yet)")
	lines = append(lines, "")
	lines = append(lines,
		"Honest debt, not a silent gap (PLAN-authored-spec-discipline.md §5): a "+
			"requirement may be SETTLED without code, but neither an authored link nor "+
			"an engine enforcer proves it yet. A stale/orphaned `implemented_by` or "+
			"`verified_by` entry (symbol renamed or deleted after the link was made) "+
			"also lands here, not in the authored-carrier table above.")
	lines = append(lines, "")
	if len(roadmapDebt) == 0 {
		lines = append(lines, "_None — every SETTLED+ENFORCEABLE requirement in this domain has a carrier._")
		lines = append(lines, "")
	} else {
		lines = append(lines, "| id | enforcement | owner | implemented_by | verified_by | claim |")
		lines = append(lines, "|---|---|---|---|---|---|")
		for _, r := range roadmapDebt {
			implCell := "—"
			if len(r.ImplementedBy) > 0 {
				implCell = renderTraceabilityLinks(resolveTraceabilityLinks(specRoot, r.ImplementedBy, true))
			}
			verifCell := "—"
			if len(r.VerifiedBy) > 0 {
				verifCell = renderTraceabilityLinks(resolveTraceabilityLinks(specRoot, r.VerifiedBy, false))
			}
			lines = append(lines, "| `"+r.ID+"` | "+r.Enforcement+" | `"+r.Owner+"` | "+implCell+" | "+verifCell+" | "+Cell(r.Claim)+" |")
		}
		lines = append(lines, "")
	}

	lines = append(lines, "## Permanent discipline (SETTLED, INHERENTLY_PROSE — not debt)")
	lines = append(lines, "")
	lines = append(lines,
		"Requirements no `check_*` could ever mechanically verify, by design — not "+
			"counted against this domain's coverage.")
	lines = append(lines, "")
	if len(permanentDiscipline) == 0 {
		lines = append(lines, "_None yet tagged._")
		lines = append(lines, "")
	} else {
		lines = append(lines, "| id | enforcement | owner | claim |")
		lines = append(lines, "|---|---|---|---|")
		for _, r := range permanentDiscipline {
			lines = append(lines, "| `"+r.ID+"` | "+r.Enforcement+" | `"+r.Owner+"` | "+Cell(r.Claim)+" |")
		}
		lines = append(lines, "")
	}

	return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n") + "\n"
}

// authoredLinksAllResolve reports whether r carries at least one
// implemented_by entry AND at least one verified_by entry, and EVERY entry
// in both lists resolves against specRoot -- the same "real, both halves
// present" bar the authored path of the disjunctive ENFORCED gate
// (R-enforced-requires-enforcer-or-authored-link) requires. A requirement
// with a link that no longer resolves (stale/orphaned after a later edit) is
// deliberately NOT counted as authored-carrier here — it surfaces instead in
// the roadmap-debt table so staleness is visible, not hidden behind a
// carrier count that no longer reflects the code on disk.
func authoredLinksAllResolve(specRoot string, r ontology.Requirement) bool {
	if len(r.ImplementedBy) == 0 || len(r.VerifiedBy) == 0 {
		return false
	}
	for _, l := range resolveTraceabilityLinks(specRoot, r.ImplementedBy, true) {
		if !l.resolved {
			return false
		}
	}
	for _, l := range resolveTraceabilityLinks(specRoot, r.VerifiedBy, false) {
		if !l.resolved {
			return false
		}
	}
	return true
}

// renderCoverageAuthoredTable renders the authored-carrier table shared by
// BuildCoverage, re-resolving each row's links exactly like
// renderTraceabilityLinks does so its "resolves" verdict never diverges from
// TRACEABILITY.md's.
func renderCoverageAuthoredTable(specRoot string, rows []ontology.Requirement) string {
	if len(rows) == 0 {
		return "_None yet — no SETTLED requirement in this domain carries a fully-resolving `implemented_by`+`verified_by` pair (PLAN-authored-spec-discipline.md §3 has not been completed for any requirement)._\n"
	}
	var b strings.Builder
	b.WriteString("| id | implemented_by | verified_by | claim |\n")
	b.WriteString("|---|---|---|---|\n")
	for _, r := range rows {
		implCell := renderTraceabilityLinks(resolveTraceabilityLinks(specRoot, r.ImplementedBy, true))
		verifCell := renderTraceabilityLinks(resolveTraceabilityLinks(specRoot, r.VerifiedBy, false))
		b.WriteString("| `" + r.ID + "` | " + implCell + " | " + verifCell + " | " + Cell(r.Claim) + " |\n")
	}
	return b.String()
}

// layerVerdict renders a one-line plain-English summary of which layer
// PLAN-authored-spec-discipline.md §4 considers this domain "at": models
// exist but no fields yet is layer 1, fields but no methods is layer 2,
// methods but nothing further (tests are counted separately, via
// verified_by presence on SETTLED requirements above, not here — a method
// count alone cannot tell whether a test exists) is layer 3-in-progress.
// This is a coarse, honest signal ("at least this far"), not a strict gate —
// the mechanical floor (internal/invariants/authored_links.go) is the actual
// enforcement point.
func layerVerdict(c ModelLayerCounts) string {
	switch {
	case c.Objects == 0:
		return "Layer 0-in-progress — spec/ files exist but no exported object (model) parses yet."
	case c.Fields == 0 && c.Methods == 0:
		return "Layer 1 — models exist (skeleton), no fields or methods yet."
	case c.Methods == 0:
		return "Layer 2 — models and fields exist, no methods (operations/invariants) yet."
	default:
		return "Layer 3 or later — models, fields, and methods all exist. See the authored-carrier table below for which SETTLED requirements this domain has actually proven with tests (layer 4)."
	}
}
