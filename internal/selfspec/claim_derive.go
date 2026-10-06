// claim_derive.go implements task #369 (RAC3-A, Phase A): deriving a
// Requirement's Claim text FROM its verified_by test(s)' recorded scenario
// description(s), the same real-`go test`-execution machinery
// cmd/hotam/gen_spec.go's `--spec` flag already uses to render docs/gen/
// SPEC.md (gate.RunVerifiedByTestRecording). Before this task, Claim was
// always a hand-authored string in a domain's spec/requirements.go Go
// literal (or a hand-edited graph.json field) — this file makes the text
// itself a DERIVED projection of the actually-run test, for any domain that
// has opted into discipline:"full", exactly the same one-way opt-in gate
// check_settled_requires_scenario (internal/invariants/scenario_discipline.go)
// already established for "every SETTLED requirement needs a real scenario
// carrier."
//
// DESIGN (resolver-settled, see the task brief this file implements):
//
//  1. Claim = the CONCATENATION of every verified_by test's recorded
//     hotamspec.NewScenario(t, id, description) description string, in
//     verified_by's own declared order. A single verified_by entry is the
//     trivial n=1 case of the same rule — not a separately coded branch.
//     Within one verified_by entry, a title repeated by two or more
//     subscenarios contributes only ONCE (first-occurrence order kept) —
//     task #389/W0.2, see deriveClaimFromVerifiedBy's own doc comment for
//     why this dedup is scoped to a single entry and deliberately NOT
//     applied across two different verified_by entries.
//  2. Separator: a single space (" "). Chosen over a newline because Claim
//     is graph.json's own short, single-line "normative claim" field
//     (SPEC.md's own doc comment: "claim остаётся коротким авторским
//     intent") — every existing Claim value in every domain's graph.json is
//     one line, and TRACEABILITY.md/COVERAGE.md/REQUIREMENTS.md render Claim
//     inside Markdown table cells (internal/generator's Cell helper flattens
//     embedded newlines to spaces anyway, so a literal newline here would be
//     silently collapsed downstream, making a real "\n" separator produce
//     indistinguishable output from a plain space while being harder to
//     reason about at the point of derivation). A space keeps the derived
//     Claim visually a single flowing sentence when a requirement's two (or
//     more) scenario titles are written to read as a continuation of one
//     another (see the real pilot precedent this task's brief cites,
//     PRAT-hotam/domains/prat/spec/model/brd_package_test.go's
//     R-brd-integrity-zero-blockers, whose two verified_by scenario titles
//     are each already a complete, independently-readable clause).
//  3. Only IN SCOPE for a Requirement whose domain has discipline:"full" AND
//     whose own Enforceability is NOT INHERENTLY_PROSE — mirrors
//     checkSettledRequiresScenario's exact two gates (domain-level
//     discipline:full check, requirement-level INHERENTLY_PROSE exemption).
//     A Requirement outside this scope keeps whatever Claim its registry
//     literal already declares, unmodified — this is what makes the whole
//     mechanism an ADDITIVE opt-in, not a universal behavior change for
//     every domain that has adopted the Go-authoring path (RAC2) but not yet
//     the scenario discipline (RAC3).
//  4. A Requirement in scope but carrying NO verified_by entries at all is
//     left untouched too (there is nothing to derive from) — that gap is
//     check_settled_requires_scenario's own job to flag once the
//     requirement is SETTLED, not this function's.
package selfspec

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/registry"
)

// ClaimDerivationSeparator is the literal string joining two consecutive
// verified_by scenario descriptions into one derived Claim — see this file's
// own package doc comment, point 2, for why a single space (not a newline)
// was chosen. Exported so a caller/test can build the exact expected string
// without duplicating the literal.
const ClaimDerivationSeparator = " "

// DeriveClaimsFromScenarios mutates reg IN PLACE (via registry.Registry.
// Update): for every registered Requirement in scope (see this file's own
// package doc comment for the exact scope rule — discipline:"full" AND
// Enforceability != INHERENTLY_PROSE AND at least one verified_by entry),
// its Claim field is REPLACED with the concatenation of every verified_by
// test's recorded hotamspec scenario description(s), in verified_by's own
// declared order, joined by ClaimDerivationSeparator.
//
// specRoot is the resolved root implemented_by/verified_by file paths are
// joined onto (gate.SpecRootForGraph's own convention — a plain domainDir
// for an ordinary consumer domain, or the engine repository root for a
// self-hosting one); selfHosting mirrors g.SelfHosting, needed only to keep
// this function's signature self-contained (it does not require a full
// *ontology.Graph).
//
// COST: real, one `go test` subprocess execution per verified_by entry of
// every in-scope Requirement (gate.RunVerifiedByTestRecording) — the exact
// same per-entry price docs/gen/SPEC.md's own generation already pays. This
// function is therefore called ONLY from the sync-domain write path
// (cmd/hotam/sync_domain.go), which already pays an equivalent per-
// verified_by-entry cost via its own genSpec(..., includeSpec=true) call
// immediately afterward — never from the cheap, default `gen-spec` path.
//
// A verified_by entry whose test cannot be resolved, does not compile, does
// not currently pass, or recorded no hotamspec scenario at all contributes
// NOTHING to the concatenation for that entry (an honest gap, mirroring
// gate.BuildSpecFromRows' own "no scenario narrative" case) — it does NOT
// abort derivation for the whole Requirement, so a Requirement with one
// narrating entry and one plain (non-scenario) entry still derives a Claim
// from the narrating entry alone, exactly reflecting what a fresh
// `gen-spec --spec` run would itself render for that requirement's normative
// text.
//
// Returns the set of Requirement IDs whose Claim was actually changed by
// this call (old != new) — callers that want to know whether this pass did
// anything (e.g. to skip needless work) can inspect len() rather than
// re-diffing the whole registry themselves.
func DeriveClaimsFromScenarios(reg *registry.Registry[ontology.Requirement], specRoot string, selfHosting bool, discipline string) []string {
	if manifest, err := loader.LoadManifest(filepath.Join(specRoot, "manifest.json")); err == nil && manifest.SelfExecutingAtoms {
		return nil
	}
	if discipline != loader.DisciplineFull {
		// Domain-level honest no-op — mirrors checkSettledRequiresScenario's
		// own top-of-function guard exactly (scenario_discipline.go).
		return nil
	}

	var changed []string
	for _, r := range reg.All() {
		if !RequirementInClaimDerivationScope(r) {
			continue
		}
		derived, ok := deriveClaimFromVerifiedBy(specRoot, selfHosting, r.VerifiedBy, nil)
		if !ok || derived == r.Claim {
			continue
		}
		r.Claim = derived
		reg.Update(r.ID, r)
		changed = append(changed, r.ID)
	}
	return changed
}

// RequirementInClaimDerivationScope reports whether r's Claim is a candidate
// for derivation AT ALL, independent of the domain's own discipline (callers
// that already know discipline:full holds, e.g. DeriveClaimsFromScenarios
// above, still need this per-requirement half of the gate; a caller that
// wants to know "would THIS requirement be derived if its domain opted in"
// without re-deriving anything can call this directly too, e.g. the
// companion drift-detection invariant, check_claim_matches_scenario).
//
// Mirrors checkSettledRequiresScenario's INHERENTLY_PROSE exemption exactly:
// a Requirement honestly tagged Enforceability == INHERENTLY_PROSE is never
// a derivation candidate (the same residual "no snapshot gate could ever
// mechanically check this" category), and a Requirement with no verified_by
// entries at all has nothing to derive from. Deliberately does NOT gate on
// Status == SETTLED: a DRAFT requirement that already carries verified_by
// entries and is not INHERENTLY_PROSE derives its Claim exactly the same way
// a SETTLED one would — the derivation rule is about WHERE the text comes
// from, not about the requirement's own lifecycle stage (SETTLED-only
// enforcement is check_settled_requires_scenario's own, separate concern).
func RequirementInClaimDerivationScope(r ontology.Requirement) bool {
	if r.Enforceability == ontology.EnforceabilityINHERENTLY_PROSE {
		return false
	}
	return len(r.VerifiedBy) > 0
}

// deriveClaimFromVerifiedBy runs every verifiedBy entry (in its own declared
// order) via gate.RunVerifiedByTestRecording and concatenates every
// recorded scenario's title (hotamspec.NewScenario's own description
// argument, carried through as gate's RecordedArtifact -> the Title field of
// its decoded Artifact JSON) with ClaimDerivationSeparator. ok is false only
// when NOT ONE entry produced any narrated scenario at all (nothing to
// derive a Claim from) — the caller then leaves the Requirement's existing
// Claim untouched rather than overwriting it with an empty string.
//
// DEDUPLICATION SCOPE (task #389, W0.2 — resolver-settled, see this file's
// own CHANGELOG entry for the full worked example): titles are deduplicated
// ONLY within the artifacts produced by a SINGLE verified_by entry (one
// `go test` run of one test function/subtest tree), preserving first-
// occurrence order — never GLOBALLY across two DIFFERENT verified_by
// entries. Rationale:
//
//   - WITHIN one entry, a repeated title is a mechanical artifact, not
//     signal: one test function that calls hotamspec.NewScenario(t, id,
//     "same wording") several times (e.g. once per loop-driven sub-case,
//     or once per t.Run table row sharing the same narrated intent) proves
//     the SAME sentence-worthy behavior repeatedly, not several distinct
//     behaviors — concatenating it N times (the bug this task fixes; see
//     the real R-gate-pg0-source-ready example this task's brief captured
//     verbatim, where one test's four identically-titled subscenarios
//     quadrupled a single sentence in the derived Claim) never added
//     information, only noise.
//   - ACROSS two DIFFERENT verified_by entries, a repeated title is a
//     distinct, worth-keeping signal instead of noise: it means two
//     independently-declared tests happen to narrate identically, which is
//     either (a) intentional — the SAME behavior is proven from two angles
//     and the requirement's author wants both entries listed as
//     independent proof, or (b) a genuine duplicate-coverage smell in the
//     requirement's own verified_by list. Silently collapsing that
//     cross-entry repeat would hide (b) instead of surfacing it, and would
//     make Claim's word count an unreliable proxy for "how many
//     independent verified_by entries actually contributed text" — a
//     property callers (and human readers comparing verified_by's length
//     to Claim's clause count) may reasonably lean on. Cross-entry
//     duplication is therefore left visible on purpose, not swallowed here.
func deriveClaimFromVerifiedBy(specRoot string, selfHosting bool, verifiedBy []string, atoms *gate.AtomSourceIndex) (claim string, ok bool) {
	var titles []string
	for _, entry := range verifiedBy {
		file, testName, parsedOK := gate.ParseFileColonSymbol(strings.TrimSpace(entry))
		if !parsedOK {
			continue
		}
		result := gate.RunVerifiedByTestRecording(specRoot, file, testName, "")
		if result.Skipped || result.Err != nil || result.CompileFailed || !result.Passed {
			continue
		}
		var entryTitles []string
		seen := make(map[string]bool, len(result.Artifacts))
		for _, art := range result.Artifacts {
			title, verdictOK := scenarioArtifactTitleIfPass(art.RawJSON)
			if !verdictOK {
				continue
			}
			if strings.TrimSpace(title) == "" {
				// Atoms carry no recorded title: their text is derived from the method source.
				if atoms == nil {
					continue
				}
				atom, err := gate.DecodeAtomArtifact(art.RawJSON)
				if err != nil || atom.Mode == "" {
					continue
				}
				if title, err = atoms.DeriveClaim(atom); err != nil || strings.TrimSpace(title) == "" {
					continue
				}
			}
			if seen[title] {
				continue
			}
			seen[title] = true
			entryTitles = append(entryTitles, title)
		}
		titles = append(titles, entryTitles...)
	}
	if len(titles) == 0 {
		return "", false
	}
	return strings.Join(titles, ClaimDerivationSeparator), true
}

// scenarioArtifact is this file's own local decode target for one
// gate.RecordedArtifact's RawJSON bytes — the same shape internal/gate/
// spec_build.go's own unexported specArtifact decodes (this package cannot
// reach that unexported type across the package boundary, so it declares an
// equivalent local shape rather than widening gate's public API for a
// single extra field read). Only ReqID/Title/Verdict are needed here (unlike
// specArtifact, which also decodes Steps for full narrative rendering).
type scenarioArtifact struct {
	ReqID   string `json:"req_id"`
	Title   string `json:"title"`
	Verdict string `json:"verdict"`
}

// scenarioArtifactTitleIfPass decodes raw (one gate.RecordedArtifact.RawJSON)
// and returns its Title, but only when the artifact's own recorded verdict is
// "pass" — mirroring internal/gate/spec_build.go's recordVerifiedByEntry,
// which filters out any non-"pass" artifact before it ever reaches
// SPEC.md's rendered narrative (a Scenario can be constructed inside a test
// whose OTHER assertions fail, producing a "fail"-verdict artifact that must
// never be trusted as proof of anything). A malformed/undecodable artifact
// (structurally impossible for a genuine recorder-produced file — see
// gate.RunVerifiedByTestRecording's own readArtifacts/looksLikeRecorderArtifact
// shape validation — but handled defensively here exactly as
// recordVerifiedByEntry itself does) is treated as "no title", not an error.
func scenarioArtifactTitleIfPass(raw []byte) (title string, ok bool) {
	var parsed scenarioArtifact
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", false
	}
	if parsed.Verdict != "pass" {
		return "", false
	}
	return parsed.Title, true
}
