package invariants

import (
	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/registry"
)

type Violation struct {
	Check   string `json:"check"`
	ID      string `json:"id"`
	Message string `json:"message"`
}

type Invariant struct {
	Name        string
	Canon       *methodology.Section
	Claim       string
	Rule        string
	Why         string
	Check       func(*ontology.Graph) []Violation
	IsDelegator bool

	// ComparesOnDiskProjection marks a check that compares the graph against
	// an on-disk artifact regenerated ONLY by `hotam gen-spec` (not by the
	// graph mutation itself) — e.g. check_spec_md_current (docs/gen/SPEC.md)
	// and check_domain_claude_md_current (CLAUDE.md). Such a check is
	// structurally unusable as a pre/post-mutation diff signal inside
	// internal/proposal/apply.go's applyToGraph: the "after" state it reads
	// is the IN-MEMORY graph immediately following a mutation that has not
	// yet been written to disk or run through gen-spec, so the committed
	// projection is GUARANTEED to disagree with a fresh render right after
	// any substantive mutation — not a real signal, pure noise (worse: the
	// violationKey dedup collapses to the same Check+ID before and after
	// regardless, so it either never blocks anything if already-stale before
	// the mutation, or always false-blocks if it was fresh before). See
	// AllViolationsForProposalGate, which filters these out, and
	// internal/proposal/apply.go's applyToGraph, the only caller of that
	// filtered view. Every other invariant (the structural floor: real
	// symbols, real tests, real links, graph-shape rules) is unaffected —
	// AllViolations (used by `all-violations`/`status`/internal/diagnose)
	// keeps reporting ComparesOnDiskProjection checks exactly as before,
	// since staleness debt there is a real, wanted signal.
	ComparesOnDiskProjection bool

	// PostProcessCheck, when non-nil, marks this Invariant as a POST-PROCESS
	// check: AllViolations/AllViolationsForProposalGate run it in a SECOND
	// phase, strictly after every ordinary (non-post-process) Check has
	// already completed for the same graph, passing that phase's full
	// violation list as the second argument. A post-process check's Check
	// field is left nil (see checkDomainClaudeMDCurrentUnwired's own doc
	// comment in claude_md_current.go for why PostProcessCheck itself
	// defaults to an honest no-op rather than doing real work until
	// cmd/hotam's init() wires the real implementation via registry.Update).
	//
	// This exists for exactly one reason: check_domain_claude_md_current
	// needs a byte-fresh render of the domain's own CLAUDE.md, and that
	// render embeds the SAME violation set every other check in the current
	// AllViolations(g) pass is being checked against (the LIVE-STATE/
	// DOMAIN-MAP pulse lines derive from it). Computing that set by calling
	// AllViolations(g) again from INSIDE a normal Check func would recurse
	// unboundedly (a harder problem than any subprocess-recursion guard
	// elsewhere in this package solves — there is no process boundary here).
	// Running as a post-process phase, fed the already-completed phase-1
	// result as a plain argument, breaks the cycle without ever re-entering
	// AllViolations. See internal/generator/claudemd.go's ViolationsOverride
	// doc comment for the render-side half of this mechanism.
	PostProcessCheck func(*ontology.Graph, []Violation) []Violation
}

var All = registry.New[Invariant]()
