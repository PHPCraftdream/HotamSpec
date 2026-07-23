// process_why_snapshot.go holds checkProcessWhySnapshotProse (task #331,
// R4-process-why): a NARROW, ADVISORY-ONLY lint over Process.Why/Step.Why
// text, scoped to exactly one smell — a point-in-time STATUS SNAPSHOT
// embedded in authored prose, the kind that inevitably goes stale because
// nothing regenerates prose. The design consult that produced this check
// found a real, confirmed instance: prat/gpsm-sm's Process.Why literally
// read "27 из 32 ФТ... ТЕКУЩЕЕ ПОЛОЖЕНИЕ на 2026-07-21" while the graph had
// since moved to 32/32 — the claim was true when written and false by the
// time anyone read it again. `why` is meant to carry DURABLE rationale (why
// this stage exists, why this order) — a live tally belongs to a typed
// carrier instead (gate_signoffs / conflict lifecycle), rendered fresh on
// every `hotam gen-spec` by PIPELINE.md's generated "Live state" section
// (internal/generator/pipeline.go's renderPipelineLiveState).
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
// already establish (see printAdvisorySection's own doc comment). Scoped
// ONLY to Process.Why and Step.Why — deliberately NOT Requirement.Why, which
// this task's design explicitly leaves untouched.
//
// Deliberately a SMALL, FIXED pattern set, not a general number/date
// heuristic: a broad "any digit near any date" rule would fire across
// hundreds of legitimate why entries that cite a real contract date, a real
// deadline, or an unrelated count with no snapshot claim attached — see this
// file's own test for the three explicit true-negative shapes the design
// named (a plain resolution date, a contract due-date, a prose count with no
// stage-token co-occurrence). The whole point of ADVISORY here is that it
// must stay LOW-NOISE across 300+ existing graph nodes, or operators learn
// to ignore it.
package invariants

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// snapshotMarkerPhrases is the small, fixed, case-insensitive list of
// phrases that name a POINT-IN-TIME STATUS claim (as opposed to durable
// rationale). Matched case-insensitively against the lowercased why text.
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

// checkProcessWhySnapshotProse fires (per the design's two independent
// conditions, either sufficient on its own) when a Process.Why or Step.Why
// text:
//
//   (a) contains a snapshotMarkerPhrases entry AND an ISO date
//       (isoDatePattern) ANYWHERE in the same why text (co-occurrence in the
//       same field, not necessarily the same sentence — simpler to implement
//       correctly, per the design consult), OR
//   (b) contains a tallyPattern match AND any stage token from the domain's
//       OWN declared gate_stage_order (loader.ResolveGateStageOrder)
//       anywhere in the same why text.
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
// without loader.LoadGraph) for condition (b)'s stage-token lookup — mirrors
// checkGateSignoffMonotonic's identical guard; condition (a) does not depend
// on DomainDir and still evaluates for such graphs.
//
// ProcessWhySnapshotWarnings is the exported entry point cmd/hotam's
// printAdvisorySection (all_violations.go) calls directly by name — mirrors
// HonoredSkipWarnings' identical exported-wrapper shape, since this check is
// never registered into the All registry (see this file's package doc
// comment for why).
func ProcessWhySnapshotWarnings(g *ontology.Graph) []Violation {
	return checkProcessWhySnapshotProse(g)
}

func checkProcessWhySnapshotProse(g *ontology.Graph) []Violation {
	var stageTokens []string
	if g.DomainDir != "" {
		stageTokens = loader.ResolveGateStageOrder(filepath.Join(g.DomainDir, "graph.json"))
	}

	var out []Violation
	for _, p := range g.Processes {
		if v, fired := evalWhySnapshot(p.Why, stageTokens); fired {
			out = append(out, Violation{
				Check:   "check_process_why_snapshot_prose",
				ID:      p.ID,
				Message: v,
			})
		}
		for _, step := range p.Steps {
			if v, fired := evalWhySnapshot(step.Why, stageTokens); fired {
				out = append(out, Violation{
					Check:   "check_process_why_snapshot_prose",
					ID:      p.ID + " / step " + step.Name,
					Message: v,
				})
			}
		}
	}
	return out
}

// snapshotViolationMessage is the fixed message text this check reports —
// identical regardless of WHICH of the two fire conditions matched, since
// both name the same underlying smell (a live status claim frozen into
// durable prose).
const snapshotViolationMessage = "Process why contains a point-in-time status snapshot; live status belongs to typed carriers (gate_signoffs / conflict lifecycle) — PIPELINE.md's Live state section renders it fresh; keep `why` durable rationale only."

// evalWhySnapshot evaluates both fire conditions against one why text,
// returning the violation message and true on either match.
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
