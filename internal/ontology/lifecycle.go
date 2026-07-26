package ontology

import (
	"errors"
	"strings"
)

const (
	StateKindInitial   = "initial"
	StateKindNormal    = "normal"
	StateKindTerminal  = "terminal"
	StateKindQuiescent = "quiescent"
)

var StateKinds = map[string]struct{}{
	StateKindInitial:   {},
	StateKindNormal:    {},
	StateKindTerminal:  {},
	StateKindQuiescent: {},
}

type State struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
	Why  string `json:"why"`
}

func (s State) IsInitial() bool {
	return s.Kind == StateKindInitial
}

func (s State) IsTerminal() bool {
	return s.Kind == StateKindTerminal || s.Kind == StateKindQuiescent
}

type Transition struct {
	Src             string  `json:"src"`
	Dst             string  `json:"dst"`
	Event           string  `json:"event"`
	Guard           string  `json:"guard"`
	GuardAssumption *string `json:"guard_assumption"`
	Why             string  `json:"why"`
}

type Lifecycle struct {
	Slug         string       `json:"slug"`
	States       []State      `json:"states"`
	Transitions  []Transition `json:"transitions"`
	Cyclic       bool         `json:"cyclic"`
	PrefixStates []string     `json:"prefix_states"`
}

func (l Lifecycle) StateNames() map[string]struct{} {
	out := make(map[string]struct{}, len(l.States))
	for _, s := range l.States {
		out[s.Name] = struct{}{}
	}
	return out
}

func (l Lifecycle) Initial() (State, error) {
	for _, s := range l.States {
		if s.IsInitial() {
			return s, nil
		}
	}
	return State{}, errors.New(l.Slug + ": no initial state")
}

func (l Lifecycle) prefixSet() map[string]struct{} {
	out := make(map[string]struct{}, len(l.PrefixStates))
	for _, p := range l.PrefixStates {
		out[p] = struct{}{}
	}
	return out
}

func (l Lifecycle) Matches(value string) (State, bool) {
	prefixes := l.prefixSet()
	for _, s := range l.States {
		if _, ok := prefixes[s.Name]; ok {
			if value == s.Name || strings.HasPrefix(value, s.Name+"(") {
				return s, true
			}
		} else if value == s.Name {
			return s, true
		}
	}
	return State{}, false
}

func (l Lifecycle) TransitionFor(fromState, event string) (Transition, bool) {
	for _, t := range l.Transitions {
		if t.Src == fromState && t.Event == event {
			return t, true
		}
	}
	return Transition{}, false
}

func (l Lifecycle) CanTransition(fromState, toState string) bool {
	for _, t := range l.Transitions {
		if t.Src == fromState && t.Dst == toState {
			return true
		}
	}
	return false
}

var RequirementStatusLifecycle = Lifecycle{
	Slug: "requirement-status",
	States: []State{
		{Name: "DRAFT", Kind: StateKindInitial},
		{Name: "SETTLED", Kind: StateKindNormal},
		{Name: "OPEN", Kind: StateKindNormal},
		{Name: "REJECTED", Kind: StateKindTerminal},
	},
	Transitions: []Transition{
		{Src: "DRAFT", Dst: "SETTLED", Event: "accept"},
		{Src: "DRAFT", Dst: "REJECTED", Event: "reject"},
		{Src: "DRAFT", Dst: "OPEN", Event: "accept-with-hole"},
		{Src: "SETTLED", Dst: "REJECTED", Event: "withdraw"},
		{Src: "SETTLED", Dst: "OPEN", Event: "reopen-question"},
		{Src: "OPEN", Dst: "SETTLED", Event: "resolve-question"},
		{Src: "OPEN", Dst: "REJECTED", Event: "reject-question"},
	},
	PrefixStates: []string{"OPEN"},
}

// RequirementProofLifecycle is the state machine backing Requirement.State()
// (task #370, RAC3-B): whether a Requirement's normative Claim is actually
// PROVEN by real, currently-executed evidence, as opposed to merely
// declared. This is deliberately a SEPARATE Lifecycle value from
// RequirementStatusLifecycle above -- that one governs the editorial
// workflow of the Claim text itself (DRAFT -> SETTLED -> ... -> REJECTED,
// decided by a human resolver via a ProposedRequirement/transition); this
// one governs a MECHANICALLY COMPUTED verdict over the SAME requirement's
// verified_by carrier (see internal/selfspec/requirement_state.go,
// task #369's RAC3-A drift/execution machinery), never resolver-decided and
// never stored on the node itself -- it is re-derived fresh on every read,
// exactly like Requirement.IsCloseableDebt() derives from Enforcement +
// Enforceability rather than being its own stored field.
//
//   - NO_CARRIER: the requirement declares no verified_by entry at all --
//     nothing to prove or disprove (mirrors
//     selfspec.RequirementInClaimDerivationScope's own "len(VerifiedBy) == 0"
//     half, and check_settled_requires_scenario's "missing carrier" branch).
//   - UNVERIFIED: verified_by is declared but the freshest attempt to run it
//     could not produce a verdict at all -- every entry fails to parse/
//     resolve, or the run was Skipped (gate's recursion guard) or hit an
//     infrastructure Err (gate.TestRunResult) -- "declared, never actually
//     proven, for reasons short of a real red bar."
//   - FAILING: verified_by resolved and ran, but at least one entry's `go
//     test` invocation did not pass (CompileFailed or Passed==false) -- a
//     real, currently-red bar.
//   - STALE: every verified_by entry runs and passes, but the requirement's
//     committed Claim no longer matches what a fresh re-derivation from
//     those passing runs would produce right now -- the exact drift
//     check_claim_matches_scenario (internal/invariants/claim_scenario_current.go)
//     detects; State() surfaces the SAME verdict as a first-class Requirement
//     state instead of only as a violation a caller must separately query.
//   - PROVEN: verified_by resolved, ran, passed, AND the committed Claim
//     currently matches its fresh re-derivation -- the healthy terminal
//     state: real evidence, currently in agreement with what is claimed.
//
// This Lifecycle is CYCLIC (proof freshness can regress the moment a
// verified_by test's scenario text changes without a re-sync, or a passing
// test starts failing) -- there is no single terminal state a requirement
// "graduates" to permanently, mirroring ConflictLifecycle's own Cyclic:true
// shape (a DECIDED conflict can return to DETECTED when its revisit_marker
// condition fires) rather than RequirementStatusLifecycle's acyclic shape.
var RequirementProofLifecycle = Lifecycle{
	Slug: "requirement-proof",
	States: []State{
		{Name: "NO_CARRIER", Kind: StateKindInitial, Why: "no verified_by entry declared -- nothing to prove or disprove yet"},
		{Name: "UNVERIFIED", Kind: StateKindNormal, Why: "verified_by declared but the freshest attempt produced no verdict (unresolvable entry, Skipped, or infra Err)"},
		{Name: "FAILING", Kind: StateKindNormal, Why: "verified_by resolved and ran, but at least one entry does not currently pass"},
		{Name: "STALE", Kind: StateKindNormal, Why: "every verified_by entry passes, but the committed Claim no longer matches a fresh re-derivation (drift)"},
		{Name: "PROVEN", Kind: StateKindQuiescent, Why: "every verified_by entry passes AND the committed Claim matches a fresh re-derivation right now"},
	},
	Transitions: []Transition{
		{Src: "NO_CARRIER", Dst: "UNVERIFIED", Event: "verified-by-declared", Why: "a verified_by entry is added but not yet resolvable/run"},
		{Src: "UNVERIFIED", Dst: "FAILING", Event: "run-resolves-and-fails", Why: "the entry now resolves and runs, but does not pass"},
		{Src: "UNVERIFIED", Dst: "STALE", Event: "run-resolves-and-passes-stale-claim", Why: "the entry now resolves and passes, but Claim has drifted"},
		{Src: "UNVERIFIED", Dst: "PROVEN", Event: "run-resolves-and-passes-fresh-claim", Why: "the entry now resolves, passes, and Claim already matches"},
		{Src: "FAILING", Dst: "UNVERIFIED", Event: "carrier-becomes-unresolvable", Why: "a previously-running entry is renamed/removed and no longer resolves"},
		{Src: "FAILING", Dst: "STALE", Event: "fix-makes-it-pass-stale-claim", Why: "the implementation is fixed, the test passes again, but Claim was not re-derived"},
		{Src: "FAILING", Dst: "PROVEN", Event: "fix-makes-it-pass-fresh-claim", Why: "the implementation is fixed, the test passes, and Claim already matches"},
		{Src: "STALE", Dst: "PROVEN", Event: "resync-claim", Why: "hotam sync-domain re-derives Claim from the currently-passing scenario"},
		{Src: "STALE", Dst: "FAILING", Event: "regression-after-drift", Why: "the test starts failing while Claim was already stale"},
		{Src: "PROVEN", Dst: "STALE", Event: "scenario-title-edited-without-resync", Why: "a verified_by test's recorded scenario text changes without re-running hotam sync-domain"},
		{Src: "PROVEN", Dst: "FAILING", Event: "regression", Why: "a previously-passing verified_by entry starts failing"},
	},
	Cyclic: true,
}

var ConflictLifecycle = Lifecycle{
	Slug: "conflict-lifecycle",
	States: []State{
		{Name: "DETECTED", Kind: StateKindInitial},
		{Name: "ACKNOWLEDGED", Kind: StateKindNormal},
		{Name: "DECIDED", Kind: StateKindQuiescent},
		{Name: "REVISIT_WHEN", Kind: StateKindQuiescent},
		{Name: "HELD", Kind: StateKindQuiescent},
	},
	Transitions: []Transition{
		{Src: "DETECTED", Dst: "ACKNOWLEDGED", Event: "resolver-acknowledge"},
		{Src: "ACKNOWLEDGED", Dst: "DECIDED", Event: "resolver-decide", Guard: "rationale or derived requirement recorded"},
		{Src: "ACKNOWLEDGED", Dst: "REVISIT_WHEN", Event: "resolver-park", Guard: "revisit condition recorded"},
		{Src: "ACKNOWLEDGED", Dst: "HELD", Event: "resolver-hold", Guard: "decided_by recorded and >=2 variants attached"},
		{Src: "DECIDED", Dst: "DETECTED", Event: "condition-fires", Guard: "revisit_marker condition holds"},
		{Src: "REVISIT_WHEN", Dst: "DETECTED", Event: "condition-fires", Guard: "parked condition holds"},
		{Src: "HELD", Dst: "DECIDED", Event: "resolver-choose-variant", Guard: "rationale names the chosen variant"},
	},
	PrefixStates: []string{"DECIDED", "REVISIT_WHEN", "HELD"},
	Cyclic:       true,
}
