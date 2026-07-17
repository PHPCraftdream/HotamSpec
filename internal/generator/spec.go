// spec.go renders docs/gen/SPEC.md: the generated NORMATIVE TEXT projection
// PLAN-scenario-generated-spec.md §2 D2/§3 W1.3 names — the successor stage
// to the authored-spec discipline's own projections (traceability.go/
// models.go/coverage.go): a requirement's claim (still the short AUTHORED
// intent from graph.json, D2 — never invented here) followed by the
// GENERATED prose narrative of its verified_by scenario(s), rendered from
// the ACTUAL Given/When/Then/Value steps a real, passing `go test` run just
// recorded via internal/recorder/canon's hotamspec API
// (PLAN-scenario-generated-spec.md §1's "text incarnates the actually-run
// test", never a second, independently-writable source of truth).
//
// Unlike traceability.go/models.go/coverage.go (pure functions of the graph
// plus a read-only AST scan), BuildSpec is NOT a pure function of the graph
// alone: it EXECUTES every verified_by test that carries a recorder-based
// scenario, exactly once, via internal/gate.RunVerifiedByTestRecording
// (W1.2) — "one run gives both the assert AND the artifact the narrative is
// built from" (PLAN §1). This is deliberately expensive (a real `go test`
// compile+run per verified_by entry) — the task's own framing: "текст
// рождается только из проходящего теста", not a claim generated for free.
//
// This file is read-only over the graph and re-EXECUTES the domain's own
// authored code via `go test` purely to observe what a real scenario
// narrated; it never mutates the graph, never writes to the domain's spec/
// tree, and is not itself an enforcement gate (a future W2.3
// check_spec_md_current is the mechanical staleness gate; this generator
// only renders what a fresh run reports NOW).
package generator

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// specArtifact is one parsed hotamspec.Artifact (internal/recorder/canon's
// JSON shape) read back from a RunVerifiedByTestRecording call — this
// package's own local decode target so spec.go does not need to import
// internal/recorder/canon (gate already avoids that coupling for the same
// reason, see gate.RecordedArtifact's doc comment: canon is vendored into
// consumer domains, never imported cross-module by the engine).
type specArtifact struct {
	ReqID   string             `json:"req_id"`
	Test    string             `json:"test"`
	Title   string             `json:"title"`
	Steps   []specArtifactStep `json:"steps"`
	Verdict string             `json:"verdict"`
}

type specArtifactStep struct {
	Kind   string           `json:"kind"`
	Desc   string           `json:"desc"`
	Values []specArtifactKV `json:"values,omitempty"`
	Passed bool             `json:"passed,omitempty"`
}

type specArtifactKV struct {
	Key   string `json:"k"`
	Value string `json:"v"`
}

// specTestOutcome is one verified_by (file:test) entry's recording outcome
// for one requirement — either a narrated scenario (one or more artifacts,
// verdict pass) or an honest reason no narrative could be rendered (test has
// no hotamspec.Scenario, the run did not pass, or execution could not be
// proven at this nesting level). Exactly one of Artifacts/Problem is
// meaningful: Problem == "" iff at least one Artifact narrates this entry.
type specTestOutcome struct {
	entry     string // raw "file:test" verified_by entry
	artifacts []specArtifact
	problem   string // non-empty: why no scenario narrative exists for this entry
}

// specRow is one rendered requirement section: its own id/claim (the D2
// short authored intent, taken verbatim from the graph, never invented
// here) plus every verified_by entry's recording outcome.
type specRow struct {
	req      ontology.Requirement
	outcomes []specTestOutcome
}

// BuildSpec renders docs/gen/SPEC.md: for every requirement, its short
// authored claim (graph.json's own text — D2's "claim остаётся коротким
// авторским intent") followed by the GENERATED normative narrative of its
// verified_by scenario(s) — Given/When/Then/Value steps rendered from a
// REAL, currently-passing `go test` run recorded via
// internal/gate.RunVerifiedByTestRecording (PLAN-scenario-generated-spec.md
// §2 D1/D2, task W1.3).
//
// Three honest outcomes per requirement, never blurred together:
//
//   - Narrated: at least one verified_by entry produced at least one
//     hotamspec.Artifact with verdict "pass". Its Given/When/Then/Value
//     steps are rendered as the requirement's normative body.
//   - No scenario: the requirement carries verified_by entries, the test(s)
//     resolve and PASS (gate.RunVerifiedByTest-shaped proof), but produced
//     NO hotamspec.Artifact — an ordinary (pre-W1.1-style) Go test with no
//     recorder narration. Shown honestly as "no scenario recorded", never
//     silently omitted or invented text.
//   - Without verified_by at all: shown in its own honest section, mirroring
//     TRACEABILITY.md's own "prose / roadmap-debt" partition — a SETTLED
//     requirement need not yet carry a scenario, but SPEC.md must say so
//     plainly rather than pretend a narrative exists.
//
// Determinism/byte-identical (task W1.3's own mandate): requirements are
// sorted by ID (not DeclOrder — SPEC.md's normative narrative is meant to be
// looked up by anchor, not read as founding history), each requirement's
// verified_by entries are rendered in their own declared (graph) order, and
// every artifact's steps render in RECORD order with canonically-rendered
// values (internal/recorder/canon's own renderValue guarantee, W1.1) — two
// BuildSpec calls against an unchanged spec/ tree produce byte-identical
// output because RunVerifiedByTestRecording's artifact bytes are themselves
// byte-identical across repeated runs of the same scenario (that guarantee
// is W1.1's, this generator only renders what it is handed, unmodified,
// never re-sorting or re-deriving a step's own fields).
func BuildSpec(g *ontology.Graph) string {
	lines := []string{Banner, ReaderHeaderLine("SPEC", g), ""}
	lines = append(lines, "# SPEC.md — generated normative text (Hotam-Spec)")
	lines = append(lines, "")
	lines = append(lines,
		"Generated from this domain's `graph.json` claims plus REAL, currently-"+
			"passing `go test` runs of every `verified_by` entry, recorded via the "+
			"`hotamspec` scenario API (PLAN-scenario-generated-spec.md §1/§2 D1/D2, "+
			"task W1.3): the normative body under each requirement is not hand-"+
			"written prose — it is the Given/When/Then/Value narrative a real test "+
			"run just produced. `graph.json` remains the bookkeeping layer (id, "+
			"short authored claim, status); this document is the derived "+
			"projection, never the other way around. Not an enforcement gate "+
			"itself — a future `check_spec_md_current` (W2.3) is the mechanical "+
			"staleness floor; this generator only renders what the CURRENT run "+
			"reports.")
	lines = append(lines, "")

	if g.IsEmpty() {
		lines = append(lines, EmptyNotice)
		lines = append(lines, "")
		return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n") + "\n"
	}

	specRoot := gate.SpecRootForGraph(g)

	// R-authored-spec-layer-progression's own sort key elsewhere is
	// DeclOrder (founding/narrative order); SPEC.md is deliberately sorted
	// by ID instead — a normative-text reference is looked up by anchor
	// (R-anchor-everything), not read start-to-end as founding history, and
	// an ID sort is also what keeps two runs byte-identical regardless of
	// any future DeclOrder renumbering that does not itself change which
	// requirements exist.
	reqs := make([]ontology.Requirement, len(g.Requirements))
	copy(reqs, g.Requirements)
	sort.Slice(reqs, func(i, j int) bool { return reqs[i].ID < reqs[j].ID })

	var withScenario, withoutVerifiedBy []ontology.Requirement
	rows := make(map[string]specRow, len(reqs))
	for _, r := range reqs {
		if len(r.VerifiedBy) == 0 {
			withoutVerifiedBy = append(withoutVerifiedBy, r)
			continue
		}
		withScenario = append(withScenario, r)
		coverFile := firstImplementedByFile(r)
		row := specRow{req: r}
		for _, entry := range r.VerifiedBy {
			row.outcomes = append(row.outcomes, recordVerifiedByEntry(specRoot, r.ID, entry, coverFile))
		}
		rows[r.ID] = row
	}

	narratedCount := 0
	for _, r := range withScenario {
		for _, o := range rows[r.ID].outcomes {
			if len(o.artifacts) > 0 {
				narratedCount++
				break
			}
		}
	}

	lines = append(lines,
		"**"+strconv.Itoa(len(withScenario))+" requirement(s) carry `verified_by`; "+
			strconv.Itoa(narratedCount)+" have at least one recorded scenario narrative; "+
			strconv.Itoa(len(withoutVerifiedBy))+" carry no `verified_by` yet (no code carrier, honest gap).**")
	lines = append(lines, "")
	lines = append(lines, "---")
	lines = append(lines, "")

	lines = append(lines, "## Requirements with a verified_by scenario")
	lines = append(lines, "")
	if len(withScenario) == 0 {
		lines = append(lines, "_None in this domain yet — the scenario-generated-spec layer (PLAN-scenario-generated-spec.md §3 W1.3) has not been started for this domain._")
		lines = append(lines, "")
	} else {
		for _, r := range withScenario {
			lines = append(lines, renderSpecRequirement(rows[r.ID])...)
		}
	}

	lines = append(lines, "## Without a scenario (no verified_by — honest gap)")
	lines = append(lines, "")
	lines = append(lines,
		"Requirements with no `verified_by` entry at all: SETTLED without a code "+
			"carrier is honest roadmap debt (mirrors TRACEABILITY.md/COVERAGE.md's "+
			"own partition), never silently claimed narrated.")
	lines = append(lines, "")
	if len(withoutVerifiedBy) == 0 {
		lines = append(lines, "_None — every requirement in this domain carries at least one `verified_by` entry._")
		lines = append(lines, "")
	} else {
		lines = append(lines, "| id | status | claim |")
		lines = append(lines, "|---|---|---|")
		for _, r := range withoutVerifiedBy {
			lines = append(lines, "| `"+r.ID+"` | "+Cell(r.Status)+" | "+Cell(r.Claim)+" |")
		}
		lines = append(lines, "")
	}

	return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n") + "\n"
}

// firstImplementedByFile returns the file half of r's first implemented_by
// entry (best-effort coverPkgFile input for
// RunVerifiedByTestRecording — implemented_by and verified_by are
// independent lists, not index-paired, per
// PLAN-authored-spec-discipline.md §4/§12; a requirement with more than one
// implemented_by symbol still only needs ONE file in that symbol's own
// package to point -coverpkg at the right import path). Returns "" when r
// carries no implemented_by at all or the entry does not parse as
// "file:symbol" — RunVerifiedByTestRecording treats an empty coverPkgFile as
// "skip coverage collection", never an error, so a requirement missing
// implemented_by still gets its scenario narrated, just without a coverage
// profile (coverage-proof enforcement is W2.2's job, not this generator's).
func firstImplementedByFile(r ontology.Requirement) string {
	if len(r.ImplementedBy) == 0 {
		return ""
	}
	file, _, ok := gate.ParseFileColonSymbol(strings.TrimSpace(r.ImplementedBy[0]))
	if !ok {
		return ""
	}
	return file
}

// recordVerifiedByEntry runs ONE verified_by entry via
// gate.RunVerifiedByTestRecording (a single real `go test` invocation) and
// classifies the outcome honestly: a non-empty problem string names EXACTLY
// why no scenario narrative could be rendered for this entry, so
// renderSpecRequirement never has to guess or paper over a gap.
func recordVerifiedByEntry(specRoot, reqID, entry, coverFile string) specTestOutcome {
	out := specTestOutcome{entry: entry}
	file, testName, ok := gate.ParseFileColonSymbol(strings.TrimSpace(entry))
	if !ok {
		out.problem = "malformed verified_by entry (expected file:symbol)"
		return out
	}

	result := gate.RunVerifiedByTestRecording(specRoot, file, testName, coverFile)
	switch {
	case result.Skipped:
		out.problem = "not executed at this nesting level (recursion guard honored) — " + result.InfraWarning
		return out
	case result.Err != nil:
		out.problem = "could not be executed: " + result.Err.Error()
		return out
	case result.CompileFailed:
		out.problem = "package does not compile"
		return out
	case !result.Passed:
		out.problem = "test does not currently pass"
		return out
	}

	var artifacts []specArtifact
	for _, a := range result.Artifacts {
		var parsed specArtifact
		if err := json.Unmarshal(a.RawJSON, &parsed); err != nil {
			// A malformed artifact from a passing run should be structurally
			// impossible (internal/recorder/canon's writeArtifact only ever
			// emits its own Artifact shape) — treat it as "no narrative"
			// rather than fail the whole document, since the test itself did
			// pass and that verdict must not be hidden by a rendering bug.
			continue
		}
		if parsed.Verdict != "pass" {
			continue
		}
		artifacts = append(artifacts, parsed)
	}
	out.artifacts = artifacts
	if len(artifacts) == 0 {
		out.problem = "test passes but recorded no hotamspec scenario (plain go test, no narrative to render)"
	}
	return out
}

// renderSpecRequirement renders one requirement's full SPEC.md section: an
// H2 heading naming the id, its short authored claim (D2 — copied verbatim
// from the graph, never rewritten), then one subsection per verified_by
// entry — either the entry's recorded scenario(s) (Given/When/Then/Value, in
// record order) or an honest one-line reason no narrative exists.
func renderSpecRequirement(row specRow) []string {
	var lines []string
	lines = append(lines, "## `"+row.req.ID+"`", "")
	lines = append(lines, "**Claim:** "+Cell(row.req.Claim), "")
	lines = append(lines, "**Status:** "+Cell(row.req.Status)+" · **Enforcement:** "+Cell(row.req.Enforcement), "")

	for i, o := range row.outcomes {
		lines = append(lines, "### `"+o.entry+"`", "")
		if o.problem != "" {
			lines = append(lines, "_No scenario narrative: "+Cell(o.problem)+"._", "")
			continue
		}
		for _, art := range o.artifacts {
			lines = append(lines, renderSpecArtifact(art)...)
		}
		_ = i
	}

	return lines
}

// renderSpecArtifact renders one hotamspec.Artifact as prose: its title,
// then each recorded Step in call order — Given/When/Then/Value — with
// Given/Value's key/value facts rendered as an inline "key=value" list
// (already canonically rendered by internal/recorder/canon's renderValue at
// RECORD time, W1.1 — this function does no further formatting of the
// value strings themselves, only the surrounding Markdown shape) and Then's
// recorded Passed outcome shown explicitly (every artifact this function
// ever receives already has verdict "pass" — recordVerifiedByEntry filters
// non-pass artifacts out before this is called — but a per-step outcome is
// still worth narrating: a Scenario can carry a Then step that itself
// reports false while an EARLIER Then's failure is what actually flipped
// t.Failed(); rendering each step's own recorded Passed value keeps the
// narrative honest about exactly which assertion(s) held).
func renderSpecArtifact(art specArtifact) []string {
	var lines []string
	lines = append(lines, "**"+Cell(art.Title)+"**", "")
	for _, step := range art.Steps {
		switch step.Kind {
		case "given":
			lines = append(lines, "- Given "+Cell(step.Desc)+renderSpecFacts(step.Values))
		case "when":
			lines = append(lines, "- When "+Cell(step.Desc))
		case "then":
			outcome := "held"
			if !step.Passed {
				outcome = "FAILED"
			}
			lines = append(lines, "- Then "+Cell(step.Desc)+" — **"+outcome+"**")
		case "value":
			lines = append(lines, "- Value"+renderSpecFacts(step.Values))
		default:
			lines = append(lines, "- "+Cell(step.Kind)+": "+Cell(step.Desc)+renderSpecFacts(step.Values))
		}
	}
	lines = append(lines, "")
	return lines
}

// renderSpecFacts renders a Step's Values as an inline " (k1=v1, k2=v2)"
// suffix, in the exact order the artifact already carries them (call order —
// never re-sorted, see internal/recorder/canon's kv doc comment for why
// Given/Value's facts are an ordered slice, not a map). Returns "" for an
// empty Values (a Given/Value step with no supporting facts), so a bare
// description does not grow a trailing " ()".
func renderSpecFacts(values []specArtifactKV) string {
	if len(values) == 0 {
		return ""
	}
	parts := make([]string, 0, len(values))
	for _, v := range values {
		parts = append(parts, v.Key+"="+v.Value)
	}
	return " (" + Cell(strings.Join(parts, ", ")) + ")"
}
