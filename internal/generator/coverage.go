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
	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// BuildCoverage renders docs/gen/COVERAGE.md: for every SETTLED requirement,
// whether it carries a real carrier (authored implemented_by+verified_by,
// re-resolved against gate.SpecRootForGraph(g); or engine enforced_by) versus
// roadmap debt (SETTLED, ENFORCEABLE, no carrier yet) versus permanent
// discipline (SETTLED, INHERENTLY_PROSE) -- plus the domain's current
// authored-spec layer per PLAN-authored-spec-discipline.md §4/§8 step 3-6,
// read from the same go/ast scan models.go performs, EXTENDED
// (PLAN-scenario-generated-spec.md §3 W1.4) with a fifth rung on that same
// ladder -- objects -> fields -> methods -> tests -> SCENARIOS -- and a
// ratchet counter of how many SETTLED+verified_by requirements still lack a
// scenario narrative. Both additions are CHEAP (AST-only, gate.ResolveSpecTest's
// HasScenario) on a default `gen-spec`; verdicts is OPTIONAL (nil on every
// pre-existing caller) and, when supplied (gen-spec --spec only, the same
// generator.ScenarioVerdictsFromRows map BuildTraceability accepts), overlays
// the REAL executed verdict alongside the AST count.
//
// DISCIPLINE-EXEMPT SECTION (PLAN-scenario-generated-spec.md §2 D4/§5): for
// a discipline:full domain only (g.Discipline == loader.DisciplineFull), the
// doc additionally renders a "Discipline-exempt (inherently prose, no
// carrier)" section naming the SETTLED subset tagged
// Enforceability=INHERENTLY_PROSE that lacks a carrier -- the exact set
// check_settled_requires_scenario (internal/invariants/scenario_discipline.go)
// exempts from its carrier demand via its INHERENTLY_PROSE branch, surfaced
// here so that exemption is visible rather than silently folded into
// "0 violations". A non-discipline:full domain produces no such section at
// all (the exemption is dormant there -- the same honest-no-op shape
// loader.DisciplineFull's own doc comment establishes).
func BuildCoverage(g *ontology.Graph, verdicts ...map[string]ScenarioVerdict) string {
	var verdictMap map[string]ScenarioVerdict
	if len(verdicts) > 0 {
		verdictMap = verdicts[0]
	}
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
	ratchet := computeScenarioRatchet(g, verdictMap)

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
			"R-authored-spec-layer-progression, §8 step 3-6), extended by "+
			"PLAN-scenario-generated-spec.md §1 with a fifth rung: a domain is founded "+
			"general-before-specific — ALL models before any one model's fields, ALL "+
			"fields before any one model's methods, ALL methods before tests, ALL tests "+
			"before scenario narratives. The first five columns are read from the same "+
			"`go/ast` scan MODELS.md performs (`ScanModelLayerCounts`); the `scenarios` "+
			"column is the CHEAP AST-only count of verified_by tests whose body calls "+
			"`hotamspec.NewScenario(...)` (gate.ResolveSpecTest's HasScenario, no test "+
			"execution) — see the ratchet counter below for the honest remaining tail.")
	lines = append(lines, "")
	if layerErr != nil {
		lines = append(lines, "_Layer could not be determined: "+layerErr.Error()+"_")
		lines = append(lines, "")
	} else if layer.Files == 0 && ratchet.total == 0 {
		lines = append(lines, "**Layer 0 — no authored spec/ yet.** No authored model file was found "+
			"(ordinary domain: nothing under `spec/`; self-hosting domain: no requirement "+
			"names an engine file via `implemented_by`/`verified_by` yet). "+
			"PLAN-authored-spec-discipline.md §8 step 3 has not started.")
		lines = append(lines, "")
	} else {
		lines = append(lines, "| files scanned | objects (models) | fields | methods | typed errors | scenarios |")
		lines = append(lines, "|---|---|---|---|---|---|")
		lines = append(lines, "| "+strconv.Itoa(layer.Files)+" | "+strconv.Itoa(layer.Objects)+" | "+
			strconv.Itoa(layer.Fields)+" | "+strconv.Itoa(layer.Methods)+" | "+strconv.Itoa(layer.Errors)+" | "+
			strconv.Itoa(ratchet.withScenario)+" |")
		lines = append(lines, "")
		lines = append(lines, "**"+layerVerdict(layer, ratchet)+"**")
		lines = append(lines, "")
	}

	lines = append(lines, "## Scenario ratchet — SETTLED requirements still without a narrative")
	lines = append(lines, "")
	lines = append(lines,
		"PLAN-scenario-generated-spec.md §2 D4/§5: the honest remaining tail of SETTLED "+
			"requirements that carry at least one `verified_by` entry but no test body yet "+
			"calls `hotamspec.NewScenario(...)` — a RATCHET, not a promise: this count is "+
			"reported so it is visible and trending down, never silently hidden. A "+
			"requirement with NO `verified_by` at all is roadmap debt already reported above "+
			"(and in TRACEABILITY.md's prose/roadmap-debt section), not counted again here — "+
			"this ratchet only tracks requirements that DO have a test but that test does not "+
			"yet narrate.")
	lines = append(lines, "")
	lines = append(lines, "**"+strconv.Itoa(ratchet.withScenario)+"/"+strconv.Itoa(ratchet.total)+
		" SETTLED requirement(s) with `verified_by` carry a scenario narrative; "+
		strconv.Itoa(ratchet.withoutScenario)+" remain in the ratchet tail.**")
	lines = append(lines, "")
	if ratchet.withoutScenario == 0 {
		if ratchet.total == 0 {
			lines = append(lines, "_No SETTLED requirement in this domain carries `verified_by` yet — the ratchet has nothing to count._")
		} else {
			lines = append(lines, "_None — every SETTLED requirement with `verified_by` in this domain already carries a scenario narrative._")
		}
		lines = append(lines, "")
	} else {
		lines = append(lines, "| id | verified_by | claim |")
		lines = append(lines, "|---|---|---|")
		for _, r := range ratchet.tailRows {
			lines = append(lines, "| `"+r.ID+"` | "+Cell(strings.Join(r.VerifiedBy, ", "))+" | "+Cell(r.Claim)+" |")
		}
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

	// Discipline-exempt (inherently prose, no carrier) -- rendered ONLY for a
	// discipline:full domain. This section names the exact SETTLED subset
	// check_settled_requires_scenario (internal/invariants/scenario_discipline.go)
	// exempts from its carrier demand via the INHERENTLY_PROSE branch: a
	// requirement tagged Enforceability=INHERENTLY_PROSE that ALSO lacks a
	// carrier (no enforced_by, no implemented_by+verified_by+scenario). For a
	// non-discipline:full domain this section MUST NOT appear at all -- the
	// exemption is dormant there, so naming it would be noise (the same
	// honest-no-op shape loader.DisciplineFull's own doc comment draws; see
	// coverage.go's computeScenarioRatchet for the established tone).
	if g.Discipline == loader.DisciplineFull {
		var disciplineExempt []ontology.Requirement
		for _, r := range settled {
			if r.Enforceability == ontology.EnforceabilityINHERENTLY_PROSE &&
				requirementLacksDisciplineCarrier(specRoot, r) {
				disciplineExempt = append(disciplineExempt, r)
			}
		}
		lines = append(lines, "## Discipline-exempt (inherently prose, no carrier)")
		lines = append(lines, "")
		lines = append(lines,
			"ONLY for a discipline:full domain (PLAN-scenario-generated-spec.md §2 D4 / §5 risks: «категория "+
				"«inherently prose» сжимается при миграции per-requirement; любое остаточное исключение видно в "+
				"COVERAGE, не прячется»): the residual subset of SETTLED requirements tagged "+
				"`enforceability=INHERENTLY_PROSE` that lack ANY carrier (no `enforced_by` engine mechanism, no "+
				"`implemented_by`+`verified_by`+scenario authored path) — exactly the set "+
				"`check_settled_requires_scenario` exempts from its carrier demand so a domain can reach a clean "+
				"0 violations without bolting an architectural/temporal-order claim onto a fake mechanism (the same "+
				"class as R-domain-founded-in-wave-order, deliberately left honestly PROSE). The exemption is "+
				"surfaced HERE so it is visible, never folded silently into \"0 violations\". A member of this set "+
				"MAY also appear in the permanent-discipline table above; this section exists specifically to name "+
				"the discipline:full gate's actively-exempted subset, distinct from the broader INHERENTLY_PROSE "+
				"roster. This section never appears for a non-discipline:full domain — the exemption is dormant there.")
		lines = append(lines, "")
		if len(disciplineExempt) == 0 {
			lines = append(lines, "_None — no INHERENTLY_PROSE SETTLED requirement in this discipline:full domain lacks a carrier (the exemption is dormant)._")
			lines = append(lines, "")
		} else {
			lines = append(lines, "| id | owner | claim |")
			lines = append(lines, "|---|---|---|")
			for _, r := range disciplineExempt {
				lines = append(lines, "| `"+r.ID+"` | `"+r.Owner+"` | "+Cell(r.Claim)+" |")
			}
			lines = append(lines, "")
		}
	}

	return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n") + "\n"
}

// requirementLacksDisciplineCarrier mirrors
// checkSettledRequiresScenario's carrier test (internal/invariants/
// scenario_discipline.go): the INVERSE of "would pass without the
// INHERENTLY_PROSE exemption" -- TRUE when the requirement satisfies NEITHER
// the engine path (enforced_by non-empty) NOR the authored path
// (implemented_by non-empty AND verified_by non-empty AND at least one
// verified_by entry resolves to a test whose body calls
// hotamspec.NewScenario(...)). Used by BuildCoverage's discipline-exempt
// section to name exactly the SETTLED+INHERENTLY_PROSE subset the gate is
// actively exempting (a requirement that already has a carrier would not
// have fired a violation anyway, so it is not "actively exempted" -- only
// the carrier-less subset is). Reuses resolveTraceabilityLinks's per-entry
// hasScenario signal (the same AST-only signal computeScenarioRatchet uses
// and the gate's anyVerifiedByEntryHasScenario re-derives independently).
func requirementLacksDisciplineCarrier(specRoot string, r ontology.Requirement) bool {
	if len(r.EnforcedBy) > 0 {
		return false
	}
	if len(r.ImplementedBy) == 0 || len(r.VerifiedBy) == 0 {
		return true
	}
	for _, l := range resolveTraceabilityLinks(specRoot, r.VerifiedBy, false) {
		if l.hasScenario {
			return false
		}
	}
	return true
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
// PLAN-authored-spec-discipline.md §4 (extended by PLAN-scenario-generated-
// spec.md §1's fifth rung) considers this domain "at": models exist but no
// fields yet is layer 1, fields but no methods is layer 2, methods but
// nothing further (tests are counted separately, via verified_by presence
// on SETTLED requirements above, not here — a method count alone cannot
// tell whether a test exists) is layer 3-in-progress, and — new for W1.4 —
// once ratchet.total > 0 (at least one SETTLED requirement carries
// verified_by) the verdict additionally reports layer 4 (tests exist, none
// narrate yet) versus layer 5-in-progress (at least one narrates, ratchet
// tail remains) versus layer 5 complete (ratchet tail is zero). This is a
// coarse, honest signal ("at least this far"), not a strict gate — the
// mechanical floor (internal/invariants/authored_links.go, and W2's future
// check_settled_requires_scenario) is the actual enforcement point.
func layerVerdict(c ModelLayerCounts, ratchet scenarioRatchet) string {
	switch {
	case c.Objects == 0:
		return "Layer 0-in-progress — spec/ files exist but no exported object (model) parses yet."
	case c.Fields == 0 && c.Methods == 0:
		return "Layer 1 — models exist (skeleton), no fields or methods yet."
	case c.Methods == 0:
		return "Layer 2 — models and fields exist, no methods (operations/invariants) yet."
	case ratchet.total == 0:
		return "Layer 3 or later — models, fields, and methods all exist. See the authored-carrier table below for which SETTLED requirements this domain has actually proven with tests (layer 4)."
	case ratchet.withScenario == 0:
		return "Layer 4 — models, fields, methods, and tests (verified_by) all exist for at least one SETTLED requirement; none of them narrate a scenario yet (layer 5 has not started, see the ratchet counter above)."
	case ratchet.withoutScenario == 0:
		return "Layer 5 — every SETTLED requirement carrying verified_by also narrates a scenario (ratchet tail is zero)."
	default:
		return "Layer 5-in-progress — at least one SETTLED requirement narrates a scenario, and the ratchet tail above names the rest still to migrate."
	}
}

// scenarioRatchet is the reduction computeScenarioRatchet returns: how many
// SETTLED requirements carry at least one verified_by entry (total), how
// many of THOSE already have at least one entry whose test body calls
// `hotamspec.NewScenario` (withScenario, AST-only by default; overridden by
// a supplied ScenarioVerdict's real Narrated flag when verdicts is
// non-nil — the REAL, executed signal takes precedence over the AST guess
// for the SAME reason applyScenarioVerdict overlays TRACEABILITY.md's
// cells), and the honest remaining tail (withoutScenario) plus the actual
// requirement rows in that tail (tailRows, DeclOrder-sorted like every other
// COVERAGE.md table) for the ratchet section's own listing.
type scenarioRatchet struct {
	total           int
	withScenario    int
	withoutScenario int
	tailRows        []ontology.Requirement
}

// computeScenarioRatchet walks every SETTLED requirement carrying at least
// one verified_by entry and classifies it narrated/not: by default (verdicts
// == nil, every pre-existing/default `gen-spec` caller) via the CHEAP
// AST-only gate.ResolveSpecTest.HasScenario signal (any ONE verified_by
// entry narrating is enough to count the requirement as "with scenario");
// when verdicts is supplied (gen-spec --spec, ScenarioVerdictsFromRows) the
// REAL executed Narrated flag is used instead, since it is strictly more
// trustworthy than the AST guess for a requirement whose test the caller
// already actually ran. tailRows is built in the SAME NarrativeOrder
// (DeclOrder) every other COVERAGE.md/TRACEABILITY.md table uses, so a
// SETTLED requirement's position in the ratchet table matches its founding
// position elsewhere.
func computeScenarioRatchet(g *ontology.Graph, verdicts map[string]ScenarioVerdict) scenarioRatchet {
	specRoot := gate.SpecRootForGraph(g)
	reqs := NarrativeOrder(g.Requirements, func(r ontology.Requirement) int { return r.DeclOrder })

	var ratchet scenarioRatchet
	for _, r := range reqs {
		if r.Status != ontology.StatusSETTLED || len(r.VerifiedBy) == 0 {
			continue
		}
		ratchet.total++

		narrated := false
		if v, ok := verdicts[r.ID]; ok {
			narrated = v.Narrated
		} else {
			for _, l := range resolveTraceabilityLinks(specRoot, r.VerifiedBy, false) {
				if l.hasScenario {
					narrated = true
					break
				}
			}
		}

		if narrated {
			ratchet.withScenario++
		} else {
			ratchet.withoutScenario++
			ratchet.tailRows = append(ratchet.tailRows, r)
		}
	}
	return ratchet
}
