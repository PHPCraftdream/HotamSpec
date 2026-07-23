// authored_prose_snapshot.go holds checkAuthoredProseSnapshot (task #333,
// R4F-prose-lint, generalizing task #331/R4-process-why): a NARROW,
// ADVISORY-ONLY lint over authored prose text fields, scoped to exactly one
// smell — a point-in-time STATUS SNAPSHOT embedded in durable authored
// prose, the kind that inevitably goes stale because nothing regenerates
// prose. The design consult behind #331 found a real, confirmed instance:
// prat/gpsm-sm's Process.Why literally read "27 из 32 ФТ... ТЕКУЩЕЕ
// ПОЛОЖЕНИЕ на 2026-07-21" while the graph had since moved to 32/32 — the
// claim was true when written and false by the time anyone read it again.
// Task #333 (the fourth external review's §4.1 synthesis) found the SAME
// smell is not unique to Process.Why/Step.Why — it is a CLASS affecting any
// authored text field: manifest.json's `goals`/`charter` fields carry the
// identical risk (gpsm-sm's own `goals` field, before task #329's rewording,
// baked in a "32/32 SIGNED"-shaped snapshot). This file now scans BOTH
// families with the exact same two predicates — Process.Why/Step.Why (#331's
// original scope) AND manifest.json's goals/charter (#333's extension).
// `why`/`goals`/`charter` are meant to carry DURABLE rationale/description —
// a live tally belongs to a typed carrier instead (gate_signoffs / conflict
// lifecycle), rendered fresh on every `hotam gen-spec` by PIPELINE.md's
// generated "Live state" section (internal/generator/pipeline.go's
// renderPipelineLiveState) or DOMAIN-MAP's generated gates line.
//
// Deliberately NOT extended (yet) to Requirement.Claim/Conflict.Context: per
// the review's own caution (§4.1), those fields carry MORE legitimate risk
// of false positives — a Requirement's claim text can legitimately assert a
// count or a status word as part of its actual normative substance (e.g.
// "the assert MUST tally FAIL CLOSED... more than one distinct
// `pipeline_run`"), unlike Process.why/goals/charter's narrower
// "status-narrative" role. A scan of hotam-spec-self's own 297-requirement
// graph found zero real fires of the two precise predicates below (so
// extending there would currently be a safe no-op), but also found ~10
// claims that pair a bare digit with a status word in ordinary normative
// prose (e.g. R-status-single-command-summary's "shall provide a `status`
// command") — evidence the CLAIM text register is noisier than
// why/goals/charter's narrower "narrate current standing" role, so this file
// stays conservative and leaves claim/context untouched. See task #333's
// drafted (not landed) proposals/draft-R-authored-prose-no-live-tallies.json
// for the class-wide requirement this check enforces the engine-portion of.
//
// ADVISORY, NEVER A GATE: this is deliberately NOT registered via
// All.MustRegister, so it never appears in invariants.AllViolations, never
// blocks `hotam all-violations`'s exit code, and never blocks
// internal/proposal/apply.go's proposal-gate. It mirrors HonoredSkipWarnings
// (authored_links.go) — a plain function returning []Violation that lives in
// this package for its shared machinery (the Violation shape, the graph
// types) but is wired into cmd/hotam's non-blocking ADVISORY section
// (all_violations.go's printAdvisorySection) by name, exactly the same
// "advisory band" HonoredSkipWarnings and diagnose.ReflectOrphanEntityType
// already establish (see printAdvisorySection's own doc comment).
//
// Deliberately a SMALL, FIXED pattern set, not a general number/date
// heuristic: a broad "any digit near any date" rule would fire across
// hundreds of legitimate why/goals/claim entries that cite a real contract
// date, a real deadline, or an unrelated count with no snapshot claim
// attached — see this file's own test for the true-negative shapes the
// design named (a plain resolution date, a contract due-date, a prose count
// with no stage-token co-occurrence, gpsm-sm's REAL post-#329-reworded goals
// text). The whole point of ADVISORY here is that it must stay LOW-NOISE
// across 300+ existing graph nodes, or operators learn to ignore it.
package invariants

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// snapshotMarkerPhrases is the small, fixed, case-insensitive list of
// phrases that name a POINT-IN-TIME STATUS claim (as opposed to durable
// rationale). Matched case-insensitively against the lowercased text.
var snapshotMarkerPhrases = []string{
	"текущее положение",
	"по состоянию на",
	"as of",
	"current status",
}

// isoDatePattern matches an ISO-shaped date (\d{4}-\d{2}-\d{2}) — the exact
// shape gpsm-sm's stale sentence carried ("на 2026-07-21").
var isoDatePattern = regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)

// tallyPattern matches a "N из M" / "N of M" tally shape (Cyrillic "из" or
// English "of"), e.g. "27 из 32" — a live count claim, the second half of
// the (b) fire condition below when it co-occurs with one of the domain's
// own declared gate stage tokens.
var tallyPattern = regexp.MustCompile(`(?i)\d+\s+(из|of)\s+\d+`)

// checkAuthoredProseSnapshot fires (per the design's two independent
// conditions, either sufficient on its own) when a Process.Why, Step.Why, or
// manifest.json goals/charter text:
//
//	(a) contains a snapshotMarkerPhrases entry AND an ISO date
//	    (isoDatePattern) ANYWHERE in the same text (co-occurrence in the
//	    same field, not necessarily the same sentence — simpler to implement
//	    correctly, per the design consult), OR
//	(b) contains a tallyPattern match AND any stage token from the domain's
//	    OWN declared gate_stage_order (loader.ResolveGateStageOrder)
//	    anywhere in the same text.
//
// (b) is scoped to the domain's OWN declared stage vocabulary (never a
// hardcoded "P-G" family) — the engine does not know or care what a "gate"
// is for any particular domain's methodology (the same boundary
// checkGateSignoffMonotonic already draws), so a domain that has not
// declared gate_stage_order simply never fires condition (b) at all (an
// honest no-op, not a false negative — there is no domain-declared stage
// vocabulary to check a tally against).
//
// Honest no-op when g.DomainDir == "" (an in-memory fixture graph built
// without loader.LoadGraph) for condition (b)'s stage-token lookup, AND for
// the manifest goals/charter scan entirely (there is no manifest.json to
// read without a DomainDir) — mirrors checkGateSignoffMonotonic's identical
// guard; condition (a) over Process.Why/Step.Why does not depend on
// DomainDir and still evaluates for such graphs.
//
// AuthoredProseSnapshotWarnings is the exported entry point cmd/hotam's
// printAdvisorySection (all_violations.go) calls directly by name — mirrors
// HonoredSkipWarnings' identical exported-wrapper shape, since this check is
// never registered into the All registry (see this file's package doc
// comment for why).
func AuthoredProseSnapshotWarnings(g *ontology.Graph) []Violation {
	return checkAuthoredProseSnapshot(g)
}

func checkAuthoredProseSnapshot(g *ontology.Graph) []Violation {
	var stageTokens []string
	if g.DomainDir != "" {
		stageTokens = loader.ResolveGateStageOrder(filepath.Join(g.DomainDir, "graph.json"))
	}

	var out []Violation
	for _, p := range g.Processes {
		if v, fired := evalWhySnapshot(p.Why, stageTokens); fired {
			out = append(out, Violation{
				Check:   "check_authored_prose_snapshot",
				ID:      p.ID,
				Message: v,
			})
		}
		for _, step := range p.Steps {
			if v, fired := evalWhySnapshot(step.Why, stageTokens); fired {
				out = append(out, Violation{
					Check:   "check_authored_prose_snapshot",
					ID:      p.ID + " / step " + step.Name,
					Message: v,
				})
			}
		}
	}

	// Manifest goals/charter scan (task #333's extension): the domain's own
	// manifest.json sitting next to graph.json, resolved the exact same way
	// stageTokens above is (loader.ResolveDomainPresentation mirrors
	// loader.ResolveGateStageOrder's "read manifest, tolerate missing/
	// malformed, default to zero value" pattern) — an honest no-op when
	// g.DomainDir == "" (no manifest.json to read).
	if g.DomainDir != "" {
		pres := loader.ResolveDomainPresentation(filepath.Join(g.DomainDir, "graph.json"))
		for i, goal := range pres.Goals {
			if v, fired := evalWhySnapshot(goal, stageTokens); fired {
				out = append(out, Violation{
					Check:   "check_authored_prose_snapshot",
					ID:      "manifest.json goals[" + strconv.Itoa(i) + "]",
					Message: v,
				})
			}
		}
		if v, fired := evalWhySnapshot(pres.Charter, stageTokens); fired {
			out = append(out, Violation{
				Check:   "check_authored_prose_snapshot",
				ID:      "manifest.json charter",
				Message: v,
			})
		}
	}

	return out
}

// snapshotViolationMessage is the fixed message text this check reports —
// identical regardless of WHICH of the two fire conditions matched, since
// both name the same underlying smell (a live status claim frozen into
// durable prose).
const snapshotViolationMessage = "Authored prose contains a point-in-time status snapshot; live status belongs to typed carriers (gate_signoffs / conflict lifecycle) — PIPELINE.md's Live state section / DOMAIN-MAP's gates line render it fresh; keep this text durable rationale/description only."

// evalWhySnapshot evaluates both fire conditions against one text field
// (Process.Why/Step.Why or a manifest goals/charter entry), returning the
// violation message and true on either match.
func evalWhySnapshot(why string, stageTokens []string) (string, bool) {
	if why == "" {
		return "", false
	}
	lower := strings.ToLower(why)

	// Condition (a): a snapshot-marker phrase co-occurring with an ISO date.
	if isoDatePattern.MatchString(why) {
		for _, phrase := range snapshotMarkerPhrases {
			if strings.Contains(lower, phrase) {
				return snapshotViolationMessage, true
			}
		}
	}

	// Condition (b): a tally pattern co-occurring with a domain-declared
	// gate stage token.
	if len(stageTokens) > 0 && tallyPattern.MatchString(why) {
		for _, stage := range stageTokens {
			if stage == "" {
				continue
			}
			if strings.Contains(why, stage) {
				return snapshotViolationMessage, true
			}
		}
	}

	return "", false
}
