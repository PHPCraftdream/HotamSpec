package proposal

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func defaultStr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func coalesceStr(newVal, defaultVal, oldVal string) string {
	if newVal == defaultVal {
		return oldVal
	}
	return newVal
}

func coalesceSlice(newVal, oldVal []string) []string {
	if len(newVal) == 0 {
		return oldVal
	}
	cp := make([]string, len(newVal))
	copy(cp, newVal)
	return cp
}

// clearSentinel is the explicit opt-in marker a proposal author writes to mean
// "clear this field to empty" — exactly "<clear>" (or, for a slice field, the
// single-element slice ["<clear>"]). It exists because both coalesceSlice and
// coalesceStr treat an empty incoming value as "leave the old value alone"
// (partial-update / patch semantics, asserted by
// TestApply_Requirement_UpdateAppendsHistory), so without an explicit signal
// there is no way for a ProposedRequirement UPDATE to empty an already-set
// field. The wave-2 enforced_by rebind NEEDS that for enforced_by: downgrading
// a requirement from ENFORCED to PROSE/STRUCTURAL must be able to drop its
// (now-misleading) stale enforcer list, and the convention for
// PROSE/STRUCTURAL requirements in this graph is enforced_by == []. The
// wave-6 blocked_on close-the-loop fix reuses the SAME literal for
// blocked_on: once a requirement is marked feature-blocked debt, clearing
// blocked_on back to "" (once the blocking feature ships) needs the same
// escape from patch semantics — one reserved literal for the whole proposal
// system is simpler to remember than a family of near-identical sentinels.
// The sentinel is consumed during apply (it never reaches graph.json); for
// enforced_by, a proposal that pairs it with other entries is rejected by
// validateEnforcedByClearSentinel.
const clearSentinel = "<clear>"

// resolveEnforcedBy is the enforced_by-specific coalesce: a single-element
// ["<clear>"] clears to an empty slice; any other non-empty value replaces;
// empty preserves the old value (patch semantics).
func resolveEnforcedBy(newVal, oldVal []string) []string {
	if len(newVal) == 1 && newVal[0] == clearSentinel {
		return []string{}
	}
	return coalesceSlice(newVal, oldVal)
}

// resolveImplementedBy is the implemented_by-specific coalesce, mirroring
// resolveEnforcedBy exactly: a single-element ["<clear>"] clears to an empty
// slice; any other non-empty value replaces; empty preserves the old value
// (patch semantics). implemented_by carries path-qualified `file:symbol`
// entries pointing into the domain's authored spec/ layer (see
// internal/ontology/requirement.go).
func resolveImplementedBy(newVal, oldVal []string) []string {
	if len(newVal) == 1 && newVal[0] == clearSentinel {
		return []string{}
	}
	return coalesceSlice(newVal, oldVal)
}

// resolveVerifiedBy is the verified_by-specific coalesce, mirroring
// resolveEnforcedBy exactly: a single-element ["<clear>"] clears to an empty
// slice; any other non-empty value replaces; empty preserves the old value
// (patch semantics). verified_by carries path-qualified `file:test` entries
// (see internal/ontology/requirement.go).
func resolveVerifiedBy(newVal, oldVal []string) []string {
	if len(newVal) == 1 && newVal[0] == clearSentinel {
		return []string{}
	}
	return coalesceSlice(newVal, oldVal)
}

// resolveBlockedOn is the blocked_on-specific coalesce, mirroring
// resolveEnforcedBy's shape for a scalar string field: the sentinel
// "<clear>" clears to ""; any other non-empty value replaces; empty
// preserves the old value (patch semantics). This is what lets an UPDATE
// proposal close the feature-blocked -> closeable-now lifecycle loop once
// the blocking feature ships — see IsCloseableDebtNow / IsFeatureBlockedDebt
// in internal/ontology/requirement.go.
func resolveBlockedOn(newVal, oldVal string) string {
	if newVal == clearSentinel {
		return ""
	}
	return coalesceStr(newVal, "", oldVal)
}

func coalesceRelations(newVal, oldVal []ontology.Relation) []ontology.Relation {
	if len(newVal) == 0 {
		return oldVal
	}
	cp := make([]ontology.Relation, len(newVal))
	copy(cp, newVal)
	return cp
}

func findRequirementIndex(g *ontology.Graph, id string) int {
	for i, r := range g.Requirements {
		if r.ID == id {
			return i
		}
	}
	return -1
}

func findConflictIndex(g *ontology.Graph, id string) int {
	for i, c := range g.Conflicts {
		if c.ID == id {
			return i
		}
	}
	return -1
}

func findAssumptionIndex(g *ontology.Graph, id string) int {
	for i, a := range g.Assumptions {
		if a.ID == id {
			return i
		}
	}
	return -1
}

func findOperatorIndex(g *ontology.Graph, id string) int {
	for i, op := range g.Operators {
		if op.ID == id {
			return i
		}
	}
	return -1
}

func findEntityTypeIndex(g *ontology.Graph, slug string) int {
	for i, et := range g.EntityTypes {
		if et.Slug == slug {
			return i
		}
	}
	return -1
}

func findProcessIndex(g *ontology.Graph, id string) int {
	for i, p := range g.Processes {
		if p.ID == id {
			return i
		}
	}
	return -1
}

func containsRelation(rels []ontology.Relation, target ontology.Relation) bool {
	for _, r := range rels {
		if r.Kind == target.Kind && r.Target == target.Target {
			return true
		}
	}
	return false
}

func (p ProposedRequirement) mutate(g *ontology.Graph, today string) error {
	idx := findRequirementIndex(g, p.ID)
	if idx >= 0 {
		existing := g.Requirements[idx]
		old := snapshotFrom(existing)

		applied := existing
		applied.Claim = p.Claim
		applied.Owner = p.Owner
		applied.Status = p.Status
		applied.Why = coalesceStr(p.Why, "", existing.Why)
		applied.Assumptions = coalesceSlice(p.Assumptions, existing.Assumptions)
		// Enforcement coalesce: an empty incoming value preserves the old
		// (patch semantics); any explicit value — including PROSE, the
		// CREATE-path default — REPLACES. The previous form
		// coalesceStr(defaultStr(p.Enforcement, PROSE), PROSE, old) made PROSE
		// unreachable on UPDATE (it was indistinguishable from "omitted", both
		// collapsed to the default and preserved the old value), so a
		// downgrade ENFORCED -> PROSE was impossible. Passing "" as the
		// defaultVal keeps "omitted == preserve" while letting an explicit
		// PROSE/STRUCTURAL/ENFORCED take effect.
		applied.Enforcement = coalesceStr(p.Enforcement, "", existing.Enforcement)
		applied.EnforcedBy = resolveEnforcedBy(p.EnforcedBy, existing.EnforcedBy)
		applied.ImplementedBy = resolveImplementedBy(p.ImplementedBy, existing.ImplementedBy)
		applied.VerifiedBy = resolveVerifiedBy(p.VerifiedBy, existing.VerifiedBy)
		applied.Relations = coalesceRelations(p.Relations, existing.Relations)
		// Enforceability coalesce: same asymmetry bug the Enforcement field
		// above was already fixed for (see that field's comment). The old
		// form wrapped p.Enforceability in defaultStr(..., ENFORCEABLE)
		// before coalescing against the ENFORCEABLE sentinel, so an explicit
		// "ENFORCEABLE" (the CREATE-path default value) was
		// indistinguishable from "omitted" and always collapsed to
		// preserving the old value — an UPDATE proposal could never flip
		// INHERENTLY_PROSE -> ENFORCEABLE (found by task W4.2, prat batch 1:
		// R-agent-mode-four-typed and 3 siblings could not actually flip via
		// `hotam apply-proposal`). Passing p.Enforceability raw with ""
		// as the omitted-sentinel keeps "omitted == preserve" while letting
		// an explicit ENFORCEABLE/INHERENTLY_PROSE/NOT_APPLICABLE take effect.
		applied.Enforceability = coalesceStr(p.Enforceability, "", existing.Enforceability)
		applied.MTag = coalesceStr(p.MTag, "", existing.MTag)
		applied.Summary = coalesceStr(p.Summary, "", existing.Summary)
		applied.CreatedAt = coalesceStr(p.CreatedAt, "", existing.CreatedAt)
		// settled_at records WHEN the requirement was first decided SETTLED --
		// it must be stamped once, on the DRAFT/other -> SETTLED transition,
		// and otherwise preserved. An UPDATE proposal always resends the
		// current status (validation requires a non-empty status), so
		// "p.Status == SETTLED" alone can't distinguish a real transition
		// from a content-only edit of an already-SETTLED requirement;
		// checking existing.Status too is what makes that distinction.
		if p.SettledAt != "" {
			applied.SettledAt = p.SettledAt
		} else if p.Status == ontology.StatusSETTLED && existing.Status != ontology.StatusSETTLED {
			applied.SettledAt = today
		}
		applied.LastReviewedAt = coalesceStr(p.LastReviewedAt, "", existing.LastReviewedAt)
		applied.ReviewAfter = coalesceStr(p.ReviewAfter, "", existing.ReviewAfter)
		applied.Evidence = coalesceSlice(p.Evidence, existing.Evidence)
		applied.SourceRefs = coalesceSlice(p.SourceRefs, existing.SourceRefs)
		applied.BlockedOn = resolveBlockedOn(p.BlockedOn, existing.BlockedOn)

		summary := summarizeFieldDiff(old, snapshotFrom(applied))
		if summary != "" {
			applied.History = append(applied.History, ontology.HistoryEntry{
				At:      today,
				Summary: summary,
			})
		}
		g.Requirements[idx] = applied
		return nil
	}

	// blocked_on's clear-sentinel presumes an EXISTING value to clear; a
	// brand-new requirement has nothing to clear, so the sentinel here is not
	// a harmless no-op — it is either a copy-pasted UPDATE proposal
	// misapplied as a CREATE, or an operator who misunderstands the
	// convention and would otherwise get the literal string "<clear>"
	// silently written into blocked_on. mutate() (not validate()) is the
	// right place for this check: validate() is a pure proposal-shape check
	// with no graph access, so it cannot tell create from update; only here,
	// having just taken the idx < 0 (create) branch, do we know for certain.
	if p.BlockedOn == clearSentinel {
		return validationError(
			"blocked_on is %q (the clear-sentinel) on a CREATE proposal for %q — "+
				"a new requirement has no existing blocked_on to clear; omit "+
				"blocked_on or set a real value.", clearSentinel, p.ID)
	}

	created := defaultStr(p.CreatedAt, today)
	settledAt := p.SettledAt
	if p.Status == ontology.StatusSETTLED && settledAt == "" {
		settledAt = today
	}
	newReq := ontology.Requirement{
		ID:             p.ID,
		Claim:          p.Claim,
		Owner:          p.Owner,
		Status:         p.Status,
		Why:            p.Why,
		Assumptions:    append([]string(nil), p.Assumptions...),
		Relations:      append([]ontology.Relation(nil), p.Relations...),
		Enforcement:    defaultStr(p.Enforcement, ontology.EnforcementPROSE),
		EnforcedBy:     append([]string(nil), p.EnforcedBy...),
		ImplementedBy:  append([]string(nil), p.ImplementedBy...),
		VerifiedBy:     append([]string(nil), p.VerifiedBy...),
		MTag:           p.MTag,
		Enforceability: defaultStr(p.Enforceability, ontology.EnforceabilityENFORCEABLE),
		Summary:        p.Summary,
		CreatedAt:      created,
		SettledAt:      settledAt,
		LastReviewedAt: p.LastReviewedAt,
		ReviewAfter:    p.ReviewAfter,
		Evidence:       append([]string(nil), p.Evidence...),
		SourceRefs:     append([]string(nil), p.SourceRefs...),
		BlockedOn:      p.BlockedOn,
	}
	g.Requirements = append(g.Requirements, newReq)
	return nil
}

// cloneGraph returns a deep copy of g via a JSON marshal/unmarshal round-trip.
// This is a simple, correct-by-construction clone (every ontology type here
// is a plain data struct that already round-trips through graph.json) — no
// hand-written deep-copy code to keep in sync as fields are added. It is
// used ONLY for in-memory simulation (SimulateRequirementResult); it never
// touches disk. DomainDir (json:"-") does not round-trip, but
// SimulateRequirementResult's caller (provenanceGate) never reads it off the
// simulated copy, so that is not a concern here.
func cloneGraph(g *ontology.Graph) (*ontology.Graph, error) {
	data, err := json.Marshal(g)
	if err != nil {
		return nil, fmt.Errorf("clone graph: marshal: %w", err)
	}
	var out ontology.Graph
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("clone graph: unmarshal: %w", err)
	}
	return &out, nil
}

// SimulateRequirementResult predicts what a ProposedRequirement's target
// requirement will look like AFTER mutate() applies it, WITHOUT touching g or
// disk. It exists so a gate can inspect the POST-MERGE field values (what
// will actually land) rather than the raw incoming proposal — which matters
// because mutate()'s coalesce* helpers treat an empty field on an UPDATE as
// "leave the existing value unchanged", not "clear it": naively checking p's
// raw fields for emptiness would misjudge an UPDATE that omits an
// already-set field on purpose.
//
// It deep-copies g (cloneGraph), applies the SAME mutate() logic this file
// already uses for a real apply to the copy, and returns the resulting
// ontology.Requirement looked up by p.ID. It deliberately skips the
// invariant-diff/violation-checking machinery applyToGraph wraps around
// mutate() — callers that need SimulateRequirementResult only need the
// post-merge field values, not a validity verdict, so running the heavier
// invariant sweep here would be wasted work.
func SimulateRequirementResult(g *ontology.Graph, today string, p ProposedRequirement) (ontology.Requirement, error) {
	cp, err := cloneGraph(g)
	if err != nil {
		return ontology.Requirement{}, err
	}
	if err := p.mutate(cp, today); err != nil {
		return ontology.Requirement{}, fmt.Errorf("simulate requirement result: %w", err)
	}
	idx := findRequirementIndex(cp, p.ID)
	if idx < 0 {
		return ontology.Requirement{}, fmt.Errorf("simulate requirement result: %q not found in simulated graph after mutate", p.ID)
	}
	return cp.Requirements[idx], nil
}

func (p ProposedConflictTransition) mutate(g *ontology.Graph, today string) error {
	idx := findConflictIndex(g, p.ConflictID)
	if idx < 0 {
		return errNotFound("conflict_id", p.ConflictID)
	}
	c := g.Conflicts[idx]
	c.Lifecycle = strings.TrimSpace(p.NewLifecycle)
	c.DecidedBy = strings.TrimSpace(p.DecidedBy)
	c.RevisitMarker = p.RevisitMarker
	if strings.TrimSpace(p.SharedAssumption) != "" {
		sa := strings.TrimSpace(p.SharedAssumption)
		c.SharedAssumption = &sa
	}
	if len(p.Derived) > 0 {
		c.Derived = append([]string(nil), p.Derived...)
	}
	if len(p.Variants) > 0 {
		c.Variants = append([]ontology.Variant(nil), p.Variants...)
	}
	if len(p.SourceRefs) > 0 {
		c.SourceRefs = append([]string(nil), p.SourceRefs...)
	}
	isDecision := strings.HasPrefix(c.Lifecycle, ontology.ConflictDECIDEDPrefix) ||
		strings.HasPrefix(c.Lifecycle, ontology.ConflictHELDPrefix)
	if isDecision && c.DecidedBy != "" {
		date := defaultStr(p.Date, today)
		c.Signoff = &ontology.Signoff{
			DecidedBy:     c.DecidedBy,
			Date:          date,
			Verbatim:      p.Verbatim,
			Instrument:    defaultStr(p.Instrument, ontology.SignoffInstrumentPersonal),
			ChosenVariant: p.ChosenVariant,
		}
		c.DecidedAt = date
	}
	g.Conflicts[idx] = c
	return nil
}

func (p ProposedRejection) mutate(g *ontology.Graph, today string) error {
	idx := findRequirementIndex(g, p.RequirementID)
	if idx < 0 {
		return errNotFound("requirement_id", p.RequirementID)
	}
	r := g.Requirements[idx]
	edgeOnly := r.Status == ontology.StatusREJECTED && normDash(r.Why) == normDash(p.Reason)
	if !edgeOnly {
		r.Status = ontology.StatusREJECTED
		newWhy := p.Reason
		if r.Why != "" {
			newWhy = p.Reason + " — (was: " + r.Why + ")"
		}
		r.Why = newWhy
		g.Requirements[idx] = r
	}
	for _, succID := range p.ReplacedBy {
		sid := strings.TrimSpace(succID)
		if sid == "" {
			continue
		}
		sidx := findRequirementIndex(g, sid)
		if sidx < 0 {
			return errNotFound("replaced_by successor", sid)
		}
		succ := g.Requirements[sidx]
		edge := ontology.Relation{Kind: "replaces", Target: p.RequirementID}
		if !containsRelation(succ.Relations, edge) {
			succ.Relations = append(succ.Relations, edge)
			g.Requirements[sidx] = succ
		}
	}
	return nil
}

func normDash(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "--", "—"))
}

func (p ProposedConflict) mutate(g *ontology.Graph, today string) error {
	id := ontology.ConflictIdentity(p.Axis, p.Context)
	if findConflictIndex(g, id) >= 0 {
		return errDuplicate("Conflict", id)
	}
	axisSlugs := ontology.AxisSlugs(g)
	if _, ok := axisSlugs[strings.TrimSpace(p.Axis)]; !ok {
		return errNotDeclared("axis", p.Axis)
	}
	members := trimNonEmpty(p.Members)
	idx := ontology.BuildIndex(g)
	for _, m := range members {
		r, ok := idx.RequirementByID[m]
		if !ok {
			return errNotFound("member requirement", m)
		}
		if r.Owner == strings.TrimSpace(p.Resolver) {
			return errResolverOwnsMember(p.Resolver, m)
		}
	}
	if len(g.Stakeholders) > 0 {
		if _, ok := ontology.StakeholderIDs(g)[strings.TrimSpace(p.Resolver)]; !ok {
			return errNotDeclared("resolver", p.Resolver)
		}
	}
	lifecycle := strings.TrimSpace(p.InitialLifecycle)
	if lifecycle == "" {
		lifecycle = ontology.ConflictDETECTED
	}
	c := ontology.Conflict{
		ID:         id,
		Axis:       strings.TrimSpace(p.Axis),
		Context:    p.Context,
		Members:    members,
		Resolver:   strings.TrimSpace(p.Resolver),
		Lifecycle:  lifecycle,
		CreatedAt:  today,
		SourceRefs: append([]string(nil), p.SourceRefs...),
	}
	if strings.TrimSpace(p.DecidedBy) != "" {
		c.DecidedBy = strings.TrimSpace(p.DecidedBy)
	}
	if strings.TrimSpace(p.SharedAssumption) != "" {
		sa := strings.TrimSpace(p.SharedAssumption)
		c.SharedAssumption = &sa
	}
	if strings.HasPrefix(lifecycle, ontology.ConflictDECIDEDPrefix) {
		c.DecidedAt = today
	}
	g.Conflicts = append(g.Conflicts, c)
	return nil
}

func (p ProposedOperatorBudget) mutate(g *ontology.Graph, today string) error {
	idx := findOperatorIndex(g, p.OperatorID)
	if idx < 0 {
		return errNotFound("operator_id", p.OperatorID)
	}
	g.Operators[idx].ContextBudget = ontology.ContextBudget{
		Limit:   p.NewLimit,
		Measure: strings.TrimSpace(p.NewMeasure),
	}
	return nil
}

func findAxisIndex(g *ontology.Graph, slug string) int {
	for i, a := range g.Axes {
		if a.Slug == slug {
			return i
		}
	}
	return -1
}

// mutate implements CREATE for a new Axis (p.Slug not yet in g) and UPDATE
// for an already-existing one -- see ProposedAxis's doc comment in types.go.
func (p ProposedAxis) mutate(g *ontology.Graph, today string) error {
	slug := strings.TrimSpace(p.Slug)
	if idx := findAxisIndex(g, slug); idx >= 0 {
		existing := g.Axes[idx]
		oldDescription := existing.Description
		existing.Description = coalesceStr(p.Description, "", existing.Description)
		if existing.Description != oldDescription {
			existing.History = append(existing.History, ontology.HistoryEntry{
				At:      today,
				Summary: "description: " + abbrev(oldDescription, 150) + "→" + abbrev(existing.Description, 150),
			})
		}
		g.Axes[idx] = existing
		return nil
	}
	if strings.TrimSpace(p.Description) == "" {
		return validationError("'description' is required for a new Axis %q.", slug)
	}
	g.Axes = append(g.Axes, ontology.Axis{
		Slug:        slug,
		Description: p.Description,
	})
	return nil
}

func (p ProposedStakeholder) mutate(g *ontology.Graph, today string) error {
	id := strings.TrimSpace(p.ID)
	if _, ok := ontology.StakeholderIDs(g)[id]; ok {
		return errDuplicate("Stakeholder", id)
	}
	g.Stakeholders = append(g.Stakeholders, ontology.Stakeholder{
		ID:     id,
		Name:   p.Name,
		Domain: p.Domain,
	})
	return nil
}

func (p ProposedAssumption) mutate(g *ontology.Graph, today string) error {
	id := strings.TrimSpace(p.ID)
	if _, ok := ontology.AssumptionIDs(g)[id]; ok {
		return errDuplicate("Assumption", id)
	}
	g.Assumptions = append(g.Assumptions, ontology.Assumption{
		ID:         id,
		Statement:  p.Statement,
		Status:     strings.TrimSpace(p.Status),
		Owner:      strings.TrimSpace(p.Owner),
		CreatedAt:  defaultStr(p.CreatedAt, today),
		SourceRefs: append([]string(nil), p.SourceRefs...),
	})
	return nil
}

// mutate is a CLEAN REWRITE of an existing Assumption's Statement -- see
// ProposedAssumptionRewrite's doc comment in types.go for how this differs
// from ProposedAssumptionTransition (which changes Status and appends a
// suffix onto Statement as a side effect). A HistoryEntry recording the
// before/after Statement (plus Reason) is ALWAYS appended here, unlike the
// summarizeFieldDiff-gated History appends elsewhere in this file --
// validate() already requires NewStatement non-empty (see validate.go), so
// mutate() never reaches this point with a no-op rewrite, and unconditional
// appending is what makes the History trail non-negotiable: a caller cannot
// silently rewrite Statement and have the History write skipped because the
// diff happened to look empty under some future refactor of the diff
// machinery (unlike Requirement/Axis's diff-gated History, which tolerates a
// no-op UPDATE producing no entry).
func (p ProposedAssumptionRewrite) mutate(g *ontology.Graph, today string) error {
	idx := findAssumptionIndex(g, p.AssumptionID)
	if idx < 0 {
		return errNotFound("assumption_id", p.AssumptionID)
	}
	a := g.Assumptions[idx]
	oldStatement := a.Statement
	a.Statement = p.NewStatement
	a.History = append(a.History, ontology.HistoryEntry{
		At:      today,
		Summary: "statement: " + abbrev(oldStatement, 150) + "→" + abbrev(a.Statement, 150) + "; reason: " + p.Reason,
	})
	g.Assumptions[idx] = a
	return nil
}

func (p ProposedAssumptionTransition) mutate(g *ontology.Graph, today string) error {
	idx := findAssumptionIndex(g, p.AssumptionID)
	if idx < 0 {
		return errNotFound("assumption_id", p.AssumptionID)
	}
	a := g.Assumptions[idx]
	newStatus := strings.TrimSpace(p.NewStatus)
	a.Statement = a.Statement + " — [" + newStatus + "] " + p.Reason
	a.Status = newStatus
	if strings.TrimSpace(p.DecidedBy) != "" {
		date := defaultStr(p.Date, today)
		a.Signoff = &ontology.Signoff{
			DecidedBy:  strings.TrimSpace(p.DecidedBy),
			Date:       date,
			Verbatim:   p.Verbatim,
			Instrument: defaultStr(p.Instrument, ontology.SignoffInstrumentPersonal),
		}
		a.DecidedAt = date
	}
	g.Assumptions[idx] = a
	return nil
}

func (p ProposedConflictMemberUpdate) mutate(g *ontology.Graph, today string) error {
	idx := findConflictIndex(g, p.ConflictID)
	if idx < 0 {
		return errNotFound("conflict_id", p.ConflictID)
	}
	c := g.Conflicts[idx]
	removeSet := map[string]struct{}{}
	for _, m := range trimNonEmpty(p.RemoveMembers) {
		removeSet[m] = struct{}{}
	}
	var kept []string
	for _, m := range c.Members {
		if _, gone := removeSet[m]; !gone {
			kept = append(kept, m)
		}
	}
	existingAfter := map[string]struct{}{}
	for _, m := range kept {
		existingAfter[m] = struct{}{}
	}
	for _, m := range trimNonEmpty(p.AddMembers) {
		if _, present := existingAfter[m]; !present {
			kept = append(kept, m)
			existingAfter[m] = struct{}{}
		}
	}
	distinct := map[string]struct{}{}
	for _, m := range kept {
		distinct[m] = struct{}{}
	}
	if len(distinct) < 2 {
		return errTooFewMembers(p.ConflictID, len(distinct))
	}
	c.Members = kept
	g.Conflicts[idx] = c
	return nil
}

// mutate applies every entry of the batch to its own Requirement, appending
// ONE new ontology.GateSignoff to that Requirement's GateSignoffs list per
// entry — all within this single mutate() call, so ApplyBatch/Apply write
// the graph exactly once for the whole wave of transitions (see
// ProposedGateSignoffBatch's doc comment in types.go for why this is its
// own Proposal kind rather than N separate proposals).
//
// Each entry's requirement_id MUST resolve to an EXISTING Requirement —
// unlike ProposedRequirement, a GateSignoffBatch entry never CREATES a
// Requirement; it only records a gate-passage fact against one that already
// exists. A single unresolvable requirement_id aborts the WHOLE mutate()
// call (an error return here means g is left partially mutated for entries
// processed before the failing one — but applyToGraph's caller (Apply/
// ApplyBatch) never persists a graph after mutate() returns an error, so a
// partially-mutated in-memory g is never written to disk; this mirrors
// every other multi-step mutate() in this file, e.g. ProposedRejection's
// per-successor loop, which has the same in-memory-only-on-error property).
//
// A HistoryEntry is appended to each AFFECTED Requirement recording its own
// GateSignoffs diff, via the SAME summarizeFieldDiff/snapshotFrom machinery
// ProposedRequirement.mutate and ProposedReviewMark.mutate already use —
// gate_signoffs is just one more field summarizeFieldDiff now diffs (see
// history.go), so no bespoke history-writing code lives here.
func (p ProposedGateSignoffBatch) mutate(g *ontology.Graph, today string) error {
	for i, e := range p.Entries {
		idx := findRequirementIndex(g, e.RequirementID)
		if idx < 0 {
			return fmt.Errorf("entry %d: %w", i, errNotFound("requirement_id", e.RequirementID))
		}
		r := g.Requirements[idx]
		old := snapshotFrom(r)

		var sp *ontology.Signoff
		if strings.TrimSpace(e.DecidedBy) != "" {
			sp = &ontology.Signoff{
				DecidedBy:     strings.TrimSpace(e.DecidedBy),
				Date:          defaultStr(e.Date, today),
				Verbatim:      e.Verbatim,
				Instrument:    defaultStr(e.Instrument, ontology.SignoffInstrumentPersonal),
				ChosenVariant: "",
			}
		}
		r.GateSignoffs = append(r.GateSignoffs, ontology.GateSignoff{
			Stage:          strings.TrimSpace(e.Stage),
			State:          strings.TrimSpace(e.State),
			DeferredReason: e.DeferredReason,
			Evidence:       append([]string(nil), e.Evidence...),
			PipelineRun:    strings.TrimSpace(e.PipelineRun),
			Signoff:        sp,
		})

		summary := summarizeFieldDiff(old, snapshotFrom(r))
		if summary != "" {
			r.History = append(r.History, ontology.HistoryEntry{
				At:      today,
				Summary: summary,
			})
		}
		g.Requirements[idx] = r
	}
	return nil
}

func (p ProposedReviewMark) mutate(g *ontology.Graph, today string) error {
	idx := findRequirementIndex(g, p.RequirementID)
	if idx < 0 {
		return errNotFound("requirement_id", p.RequirementID)
	}
	r := g.Requirements[idx]
	old := snapshotFrom(r)

	r.LastReviewedAt = defaultStr(p.ReviewedAt, today)
	if strings.TrimSpace(p.ReviewAfter) != "" {
		r.ReviewAfter = p.ReviewAfter
	}
	if len(p.Evidence) > 0 {
		r.Evidence = coalesceSlice(p.Evidence, r.Evidence)
	}

	summary := summarizeFieldDiff(old, snapshotFrom(r))
	if summary != "" {
		r.History = append(r.History, ontology.HistoryEntry{
			At:      today,
			Summary: summary,
		})
	}
	g.Requirements[idx] = r
	return nil
}

// mutate implements CREATE for a new EntityType (p.Slug not yet in g) and a
// minimal, deliberately narrow UPDATE for an already-existing one.
//
// UPDATE (p.Slug already names an EntityType in g): p.Fields are APPENDED to
// the existing EntityType.Fields -- never replacing or redefining an
// existing field (errFieldAlreadyExists if a name collides). States,
// Transitions, Description and Why must all be empty on an UPDATE
// (errEntityTypeUpdateShape otherwise) -- this first iteration intentionally
// does not support editing lifecycle/description/why of an already-landed
// EntityType, only adding new fields to it (e.g. a new reference field
// pointing at another EntityType). A HistoryEntry is appended recording the
// appended field names, mirroring the History-on-mutation pattern
// ProposedRequirement/ProposedReviewMark already use.
//
// CREATE (p.Slug not yet in g): unchanged from before this UPDATE path
// existed -- byte-identical behavior.
func (p ProposedEntityType) mutate(g *ontology.Graph, today string) error {
	slug := strings.TrimSpace(p.Slug)
	if idx := findEntityTypeIndex(g, slug); idx >= 0 {
		if len(p.States) != 0 || len(p.Transitions) != 0 ||
			strings.TrimSpace(p.Description) != "" || strings.TrimSpace(p.Why) != "" {
			return errEntityTypeUpdateShape(slug)
		}
		existing := g.EntityTypes[idx]
		existingNames := make(map[string]struct{}, len(existing.Fields))
		for _, f := range existing.Fields {
			existingNames[f.Name] = struct{}{}
		}
		newFields := make([]ontology.EntityField, 0, len(p.Fields))
		addedNames := make([]string, 0, len(p.Fields))
		for _, f := range p.Fields {
			if _, dup := existingNames[f.Name]; dup {
				return errFieldAlreadyExists(slug, f.Name)
			}
			newFields = append(newFields, ontology.EntityField{
				Name:      f.Name,
				Kind:      f.Kind,
				Required:  f.Required,
				RefTarget: f.RefTarget,
			})
			addedNames = append(addedNames, f.Name)
			existingNames[f.Name] = struct{}{}
		}
		existing.Fields = append(existing.Fields, newFields...)
		if len(addedNames) > 0 {
			existing.History = append(existing.History, ontology.HistoryEntry{
				At:      today,
				Summary: "fields added: [" + strings.Join(addedNames, ", ") + "]",
			})
		}
		g.EntityTypes[idx] = existing
		return nil
	}
	states := make([]ontology.State, 0, len(p.States))
	for _, s := range p.States {
		states = append(states, ontology.State{Name: s.Name, Kind: s.Kind, Why: s.Why})
	}
	transitions := make([]ontology.Transition, 0, len(p.Transitions))
	for _, t := range p.Transitions {
		transitions = append(transitions, ontology.Transition{Src: t.Src, Dst: t.Dst, Event: t.Event})
	}
	fields := make([]ontology.EntityField, 0, len(p.Fields))
	for _, f := range p.Fields {
		fields = append(fields, ontology.EntityField{
			Name:      f.Name,
			Kind:      f.Kind,
			Required:  f.Required,
			RefTarget: f.RefTarget,
		})
	}
	g.EntityTypes = append(g.EntityTypes, ontology.EntityType{
		Slug:        slug,
		Description: p.Description,
		Lifecycle: ontology.Lifecycle{
			Slug:        slug + "-lifecycle",
			States:      states,
			Transitions: transitions,
			Cyclic:      p.Cyclic,
		},
		Fields: fields,
		Why:    p.Why,
	})
	return nil
}

// mutate implements CREATE for a new Process (p.ID not yet in g) and a
// minimal, deliberately narrow UPDATE for an already-existing one (mirrors
// ProposedEntityType's UPDATE mode, ffa4977).
//
// UPDATE (p.ID already names a Process in g):
//   - p.Steps are APPENDED to the end of the existing Process.Steps --
//     never redefining, removing, or reordering an existing step
//     (errStepAlreadyExists if a Name collides with an existing step). Each
//     appended step is validated the same way validate() already validates
//     a CREATE's steps (non-empty name/requires_role/why, and
//     requires_role declared in THIS proposal's roles_required) --
//     validate() already runs that check uniformly for both CREATE and
//     UPDATE (see isProcessStepsShapePresent's doc comment in validate.go).
//   - p.RolesRequired (the roles used by the NEWLY appended steps) are
//     UNIONED into the existing Process.RolesRequired, not replaced --
//     check_process_roles_declared (internal/invariants/scope_process.go)
//     checks the FULL post-mutation Process.RolesRequired against the FULL
//     post-mutation Process.Steps (old + new), so a role a pre-existing
//     step already declared must stay declared, and a role a newly
//     appended step declares must become newly declared too. A role
//     already present in the existing list is left as-is (no duplicate).
//   - p.DrivesEntities are APPENDED to the existing Process.DrivesEntities
//     -- never removing or reordering the existing list
//     (errDrivesEntityAlreadyExists if a slug is already present).
//   - p.Why, if non-empty, REPLACES the existing Process.Why (a correction
//     of the process's own rationale, not an append -- coalesceStr's
//     established "empty preserves, non-empty replaces" idiom, the same
//     one ProposedRequirement.mutate already uses for its own Why field).
//     An empty p.Why leaves the existing Why untouched.
//   - A HistoryEntry is appended recording what changed (step names added,
//     drives_entities slugs added, and/or "why updated"), mirroring the
//     History-on-mutation pattern ProposedEntityType's UPDATE mode uses.
//
// drives_entities referential integrity is checked HERE (not in validate()),
// on BOTH CREATE and UPDATE: validate() has no graph access, so it cannot
// know which EntityType slugs are declared in the target domain -- the same
// split ProposedConflict.mutate already uses for its member/resolver lookups.
// Each slug in p.DrivesEntities MUST resolve to a declared EntityType.slug in
// g.entity_types (mirrors check_process_drives_existing_entities,
// internal/invariants/scope_process.go) so a bad slug is rejected here with a
// clear message instead of landing and only being caught later by the
// invariant sweep applyToGraph runs after mutate.
//
// The Process always carries the single shared ontology.ProcessLifecycle --
// see ProposedProcess's doc comment in types.go for why no author-supplied
// lifecycle is accepted (CREATE only; UPDATE never touches Lifecycle).
//
// CREATE (p.ID not yet in g): unchanged from before this UPDATE path
// existed -- byte-identical behavior.
func (p ProposedProcess) mutate(g *ontology.Graph, today string) error {
	id := strings.TrimSpace(p.ID)
	declaredEntityTypes := ontology.EntityTypeSlugs(g)
	for _, slug := range trimNonEmpty(p.DrivesEntities) {
		if _, ok := declaredEntityTypes[slug]; !ok {
			return errNotDeclared("drives_entities EntityType", slug)
		}
	}
	if idx := findProcessIndex(g, id); idx >= 0 {
		existing := g.Processes[idx]

		existingStepNames := make(map[string]struct{}, len(existing.Steps))
		for _, s := range existing.Steps {
			existingStepNames[s.Name] = struct{}{}
		}
		newSteps := make([]ontology.Step, 0, len(p.Steps))
		addedStepNames := make([]string, 0, len(p.Steps))
		for _, s := range p.Steps {
			name := strings.TrimSpace(s.Name)
			if _, dup := existingStepNames[name]; dup {
				return errStepAlreadyExists(id, name)
			}
			newSteps = append(newSteps, ontology.Step{
				Name:         name,
				RequiresRole: strings.TrimSpace(s.RequiresRole),
				Invokes:      s.Invokes,
				Why:          s.Why,
			})
			addedStepNames = append(addedStepNames, name)
			existingStepNames[name] = struct{}{}
		}

		existingEntitySlugs := make(map[string]struct{}, len(existing.DrivesEntities))
		for _, slug := range existing.DrivesEntities {
			existingEntitySlugs[slug] = struct{}{}
		}
		newEntitySlugs := make([]string, 0, len(p.DrivesEntities))
		for _, slug := range trimNonEmpty(p.DrivesEntities) {
			if _, dup := existingEntitySlugs[slug]; dup {
				return errDrivesEntityAlreadyExists(id, slug)
			}
			newEntitySlugs = append(newEntitySlugs, slug)
			existingEntitySlugs[slug] = struct{}{}
		}

		existingRoles := make(map[string]struct{}, len(existing.RolesRequired))
		for _, r := range existing.RolesRequired {
			existingRoles[r] = struct{}{}
		}
		mergedRoles := append([]string{}, existing.RolesRequired...)
		for _, r := range trimNonEmpty(p.RolesRequired) {
			if _, dup := existingRoles[r]; !dup {
				mergedRoles = append(mergedRoles, r)
				existingRoles[r] = struct{}{}
			}
		}

		existing.Steps = append(existing.Steps, newSteps...)
		existing.DrivesEntities = append(existing.DrivesEntities, newEntitySlugs...)
		existing.RolesRequired = mergedRoles

		var changes []string
		if len(addedStepNames) > 0 {
			changes = append(changes, "steps added: ["+strings.Join(addedStepNames, ", ")+"]")
		}
		if len(newEntitySlugs) > 0 {
			changes = append(changes, "drives_entities added: ["+strings.Join(newEntitySlugs, ", ")+"]")
		}
		if strings.TrimSpace(p.Why) != "" && strings.TrimSpace(p.Why) != existing.Why {
			changes = append(changes, "why updated")
		}
		existing.Why = coalesceStr(p.Why, "", existing.Why)
		if len(changes) > 0 {
			existing.History = append(existing.History, ontology.HistoryEntry{
				At:      today,
				Summary: strings.Join(changes, "; "),
			})
		}
		g.Processes[idx] = existing
		return nil
	}
	// CREATE (id not found above): a brand-new Process must have at least
	// one step. validate() cannot enforce this by itself any more -- an
	// empty p.Steps is also the valid shape for an UPDATE that appends only
	// drives_entities (see isProcessStepsShapePresent's doc comment in
	// validate.go) -- so the "steps required" check for a genuine CREATE
	// has to live here, where mutate() already knows (via findProcessIndex
	// above) that id does NOT name an existing Process.
	if len(p.Steps) == 0 {
		return validationError("'steps' must be a non-empty list of steps.")
	}
	steps := make([]ontology.Step, 0, len(p.Steps))
	for _, s := range p.Steps {
		steps = append(steps, ontology.Step{
			Name:         strings.TrimSpace(s.Name),
			RequiresRole: strings.TrimSpace(s.RequiresRole),
			Invokes:      s.Invokes,
			Why:          s.Why,
		})
	}
	g.Processes = append(g.Processes, ontology.Process{
		ID:             id,
		Lifecycle:      ontology.ProcessLifecycle,
		Steps:          steps,
		RolesRequired:  trimNonEmpty(p.RolesRequired),
		DrivesEntities: trimNonEmpty(p.DrivesEntities),
		Why:            p.Why,
	})
	return nil
}
