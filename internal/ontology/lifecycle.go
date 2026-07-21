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
