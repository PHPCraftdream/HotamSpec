package proposal

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

var slugPattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

func validationError(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}

func (p ProposedRequirement) validate() error {
	if strings.TrimSpace(p.ID) == "" {
		return validationError("'id' is required for a Requirement proposal.")
	}
	if strings.TrimSpace(p.Claim) == "" {
		return validationError("'claim' is required and must be non-empty.")
	}
	if strings.TrimSpace(p.Owner) == "" {
		return validationError("'owner' is required and must be non-empty.")
	}
	if strings.TrimSpace(p.Status) == "" {
		return validationError("'status' is required and must be non-empty.")
	}
	if err := validateEnforcedByClearSentinel(p.EnforcedBy); err != nil {
		return err
	}
	if (strings.TrimSpace(p.LastReviewedAt) != "" || strings.TrimSpace(p.ReviewAfter) != "") &&
		len(trimNonEmpty(p.Evidence)) == 0 {
		return validationError(
			"'evidence' is required when 'last_reviewed_at' or 'review_after' is set on a " +
				"Requirement proposal — freshness fields are evidence-gated regardless of entry " +
				"point (ProposedRequirement or ProposedReviewMark); without it a stamp is " +
				"indistinguishable from an administrative date backfill riding along a routine " +
				"content edit (R-review-mark-carries-evidence).")
	}
	return nil
}

// validateEnforcedByClearSentinel enforces that the "<clear>" sentinel, when
// used, is the ONLY entry in enforced_by — it cannot be combined with real
// enforcer names and cannot be repeated. See clearSentinel / mutate.go.
func validateEnforcedByClearSentinel(enforcedBy []string) error {
	seen := 0
	for _, e := range enforcedBy {
		if e == clearSentinel {
			seen++
		}
	}
	if seen > 0 {
		if len(enforcedBy) != 1 {
			return validationError(
				"enforced_by contains the %q clear-sentinel alongside other entries; "+
					"use exactly [\"%s\"] to clear, or list real enforcers — not both.",
				clearSentinel, clearSentinel)
		}
	}
	return nil
}

func (p ProposedConflictTransition) validate() error {
	if strings.TrimSpace(p.ConflictID) == "" {
		return validationError("'conflict_id' is required and must be non-empty.")
	}
	newLifecycle := strings.TrimSpace(p.NewLifecycle)
	if newLifecycle == "" {
		return validationError("'new_lifecycle' is required and must be non-empty.")
	}
	decidedBy := strings.TrimSpace(p.DecidedBy)
	if strings.HasPrefix(newLifecycle, ontology.ConflictDECIDEDPrefix) && decidedBy == "" {
		return validationError(
			"new_lifecycle starts with DECIDED but decided_by is empty. " +
				"A DECIDED transition requires a human decider.")
	}
	if strings.HasPrefix(newLifecycle, ontology.ConflictHELDPrefix) {
		if decidedBy == "" {
			return validationError(
				"new_lifecycle starts with HELD but decided_by is empty. " +
					"A HELD transition requires a human signoff.")
		}
		distinct := map[string]struct{}{}
		for _, v := range p.Variants {
			distinct[v.ID] = struct{}{}
		}
		if len(distinct) < 2 {
			return validationError(
				"new_lifecycle starts with HELD but fewer than 2 distinct " +
					"Variant ids were supplied (requires >= 2).")
		}
	}
	return nil
}

func (p ProposedRejection) validate() error {
	if strings.TrimSpace(p.RequirementID) == "" {
		return validationError("'requirement_id' is required for a Rejection proposal.")
	}
	if strings.TrimSpace(p.Reason) == "" {
		return validationError("'reason' is required and must be non-empty.")
	}
	return nil
}

func (p ProposedConflictMemberUpdate) validate() error {
	if strings.TrimSpace(p.ConflictID) == "" {
		return validationError("'conflict_id' is required for a ConflictMemberUpdate proposal.")
	}
	add := trimNonEmpty(p.AddMembers)
	rem := trimNonEmpty(p.RemoveMembers)
	if len(add) == 0 && len(rem) == 0 {
		return validationError(
			"at least one of 'add_members' / 'remove_members' must be non-empty " +
				"(a ConflictMemberUpdate with neither is a no-op).")
	}
	return nil
}

func (p ProposedConflict) validate() error {
	axis := strings.TrimSpace(p.Axis)
	if axis == "" {
		return validationError("'axis' is required and must be non-empty.")
	}
	context := strings.TrimSpace(p.Context)
	if context == "" {
		return validationError("'context' is required and must be non-empty.")
	}
	members := trimNonEmpty(p.Members)
	seen := map[string]struct{}{}
	for _, m := range members {
		seen[m] = struct{}{}
	}
	if len(seen) < 2 {
		return validationError(
			"'members' must contain at least two DISTINCT requirement ids.")
	}
	for _, m := range members {
		if !strings.HasPrefix(m, "R-") {
			return validationError("member %q must be an R-... requirement id.", m)
		}
	}
	steward := strings.TrimSpace(p.Steward)
	if steward == "" {
		return validationError("'steward' is required and must be non-empty.")
	}
	initialLifecycle := strings.TrimSpace(p.InitialLifecycle)
	if initialLifecycle == "" {
		initialLifecycle = ontology.ConflictDETECTED
	}
	decidedBy := strings.TrimSpace(p.DecidedBy)
	if strings.HasPrefix(initialLifecycle, ontology.ConflictDECIDEDPrefix) && decidedBy == "" {
		return validationError(
			"initial_lifecycle starts with DECIDED but decided_by is empty. " +
				"Materializing a conflict already-DECIDED requires a human decider.")
	}
	if !strings.HasPrefix(initialLifecycle, ontology.ConflictDECIDEDPrefix) && initialLifecycle != ontology.ConflictDETECTED {
		return validationError(
			"initial_lifecycle must be 'DETECTED' or start with 'DECIDED(...)'. " +
				"Other lifecycles are reached via a separate ConflictTransition.")
	}
	return nil
}

func (p ProposedOperatorBudget) validate() error {
	operatorID := strings.TrimSpace(p.OperatorID)
	if operatorID == "" {
		return validationError("'operator_id' is required for an OperatorBudget proposal.")
	}
	if !strings.HasPrefix(operatorID, "OP-") {
		return validationError("'operator_id' must start with 'OP-'; got %q.", operatorID)
	}
	if p.NewLimit < 0 {
		return validationError("'new_limit' must be >= 0; got %d.", p.NewLimit)
	}
	newMeasure := strings.TrimSpace(p.NewMeasure)
	if _, ok := ontology.BudgetMeasures[newMeasure]; !ok {
		return validationError("'new_measure' must be a valid budget measure; got %q.", newMeasure)
	}
	return nil
}

func (p ProposedAxis) validate() error {
	slug := strings.TrimSpace(p.Slug)
	if slug == "" {
		return validationError("'slug' is required for an Axis proposal.")
	}
	if !slugPattern.MatchString(slug) {
		return validationError(
			"'slug' must be kebab-case (lowercase letters, digits, hyphens, "+
				"starting with a letter); got %q.", slug)
	}
	if strings.TrimSpace(p.Description) == "" {
		return validationError("'description' is required and must be non-empty.")
	}
	return nil
}

func (p ProposedStakeholder) validate() error {
	if strings.TrimSpace(p.ID) == "" {
		return validationError("'id' is required for a Stakeholder proposal.")
	}
	if strings.TrimSpace(p.Name) == "" {
		return validationError("'name' is required and must be non-empty.")
	}
	if strings.TrimSpace(p.Domain) == "" {
		return validationError("'domain' is required and must be non-empty.")
	}
	return nil
}

func (p ProposedAssumption) validate() error {
	id := strings.TrimSpace(p.ID)
	if id == "" {
		return validationError("'id' is required for an Assumption proposal.")
	}
	if !strings.HasPrefix(id, "A-") {
		return validationError("'id' must start with 'A-'; got %q.", id)
	}
	if strings.TrimSpace(p.Statement) == "" {
		return validationError("'statement' is required and must be non-empty.")
	}
	status := strings.TrimSpace(p.Status)
	if _, ok := ontology.AssumptionStates[status]; !ok {
		return validationError("'status' must be a valid assumption state; got %q.", status)
	}
	if strings.TrimSpace(p.Owner) == "" {
		return validationError("'owner' is required and must be non-empty.")
	}
	return nil
}

func (p ProposedAssumptionTransition) validate() error {
	if strings.TrimSpace(p.AssumptionID) == "" {
		return validationError("'assumption_id' is required for an AssumptionTransition proposal.")
	}
	newStatus := strings.TrimSpace(p.NewStatus)
	if _, ok := ontology.AssumptionStates[newStatus]; !ok {
		return validationError("'new_status' must be a valid assumption state; got %q.", newStatus)
	}
	if strings.TrimSpace(p.Reason) == "" {
		return validationError(
			"'reason' is required and must be non-empty — an assumption status " +
				"change with no recorded reason is drift, not a decision.")
	}
	decidedBy := strings.TrimSpace(p.DecidedBy)
	if (newStatus == ontology.AssumptionDEAD ||
		newStatus == ontology.AssumptionHOLDS ||
		newStatus == ontology.AssumptionIMPLEMENTS) && decidedBy == "" {
		return validationError(
			"'decided_by' is required when new_status is %q: a transition that "+
				"reduces live signal needs a named human signoff.", newStatus)
	}
	return nil
}

// isEntityTypeCreateAttempt reports whether p carries EITHER a description
// OR states -- the two fields that are REQUIRED for a CREATE (a new
// EntityType with no description or no states is never valid) and that an
// UPDATE proposal (see ProposedEntityType.mutate) MUST leave empty. It
// governs which validate() branch runs below:
//   - true  -> full CREATE-shape validation (unchanged from before UPDATE
//     existed), so a genuine create with a missing/malformed description or
//     states list is still rejected here, byte-identically to before.
//   - false -> validate() defers the create-vs-update call to mutate(),
//     which has graph access. This is a SHAPE check only: validate() cannot
//     know whether p.Slug actually names an existing EntityType. If it does,
//     mutate()'s UPDATE path runs (fields-only append). If it does not,
//     mutate()'s CREATE path runs anyway and produces an EntityType with no
//     states/description -- which the 'states must be non-empty' invariant
//     (internal/invariants) rejects downstream, so this branch can never
//     silently create a malformed EntityType.
func isEntityTypeCreateAttempt(p ProposedEntityType) bool {
	return strings.TrimSpace(p.Description) != "" || len(p.States) != 0
}

func (p ProposedEntityType) validate() error {
	slug := strings.TrimSpace(p.Slug)
	if slug == "" {
		return validationError("'slug' is required for an EntityType proposal.")
	}
	if !slugPattern.MatchString(slug) {
		return validationError(
			"'slug' must be kebab-case; got %q.", slug)
	}
	if !isEntityTypeCreateAttempt(p) {
		// Not attempting to CREATE (no description, no states): either a
		// fields-only UPDATE, or a malformed proposal with neither shape --
		// mutate() makes the authoritative call once it has graph access
		// (see isEntityTypeCreateAttempt doc comment above).
		if len(p.Fields) == 0 {
			return validationError(
				"an EntityType proposal must supply either 'description'+'states' " +
					"(to create a new EntityType) or 'fields' (to append fields to an " +
					"existing one) -- got neither.")
		}
		for _, f := range p.Fields {
			if _, ok := ontology.EntityFieldKinds[f.Kind]; !ok {
				return validationError("field kind %q is not valid.", f.Kind)
			}
		}
		return nil
	}
	if strings.TrimSpace(p.Description) == "" {
		return validationError("'description' is required and must be non-empty.")
	}
	if len(p.States) == 0 {
		return validationError("'states' must be a non-empty list of states.")
	}
	stateNames := map[string]struct{}{}
	initialCount := 0
	for _, s := range p.States {
		if strings.TrimSpace(s.Name) == "" {
			return validationError("each state must have a non-empty name.")
		}
		if _, ok := ontology.StateKinds[s.Kind]; !ok {
			return validationError("state kind %q is not valid.", s.Kind)
		}
		stateNames[s.Name] = struct{}{}
		if s.Kind == ontology.StateKindInitial {
			initialCount++
		}
	}
	if initialCount != 1 {
		return validationError("exactly one state must have kind='initial'; found %d.", initialCount)
	}
	for _, t := range p.Transitions {
		if _, ok := stateNames[t.Src]; !ok {
			return validationError("transition src %q is not a declared state name.", t.Src)
		}
		if _, ok := stateNames[t.Dst]; !ok {
			return validationError("transition dst %q is not a declared state name.", t.Dst)
		}
	}
	for _, f := range p.Fields {
		if _, ok := ontology.EntityFieldKinds[f.Kind]; !ok {
			return validationError("field kind %q is not valid.", f.Kind)
		}
	}
	return nil
}

func (p ProposedProcess) validate() error {
	id := strings.TrimSpace(p.ID)
	if id == "" {
		return validationError("'id' is required for a Process proposal.")
	}
	if !strings.HasPrefix(id, "PR-") {
		return validationError("'id' must start with 'PR-'; got %q.", id)
	}
	if len(p.Steps) == 0 {
		return validationError("'steps' must be a non-empty list of steps.")
	}
	declaredRoles := map[string]struct{}{}
	for _, r := range trimNonEmpty(p.RolesRequired) {
		declaredRoles[r] = struct{}{}
	}
	usedRoles := map[string]struct{}{}
	for i, s := range p.Steps {
		name := strings.TrimSpace(s.Name)
		if name == "" {
			return validationError("step %d: 'name' is required and must be non-empty.", i)
		}
		role := strings.TrimSpace(s.RequiresRole)
		if role == "" {
			return validationError("step %q: 'requires_role' is required and must be non-empty.", name)
		}
		if strings.TrimSpace(s.Why) == "" {
			return validationError("step %q: 'why' is required and must be non-empty.", name)
		}
		if _, ok := declaredRoles[role]; !ok {
			return validationError(
				"step %q requires role %q which is not listed in 'roles_required' %v -- "+
					"every step's requires_role must be declared explicitly in roles_required "+
					"(no implicit roles, R-process-roles-declared).", name, role, sortedRoleList(declaredRoles))
		}
		usedRoles[role] = struct{}{}
	}
	for r := range declaredRoles {
		if _, ok := usedRoles[r]; !ok {
			return validationError(
				"'roles_required' lists role %q but no step requires it -- "+
					"roles_required must match exactly the set of roles used by steps "+
					"(no undemanded roles).", r)
		}
	}
	return nil
}

func sortedRoleList(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (p ProposedReviewMark) validate() error {
	if strings.TrimSpace(p.RequirementID) == "" {
		return validationError("'requirement_id' is required for a ReviewMark proposal.")
	}
	if len(trimNonEmpty(p.Evidence)) == 0 {
		return validationError(
			"'evidence' is required for a ReviewMark proposal — it is the attestation " +
				"the steward inspects to confirm the review was substantive; without it a mark " +
				"is indistinguishable from an administrative date backfill (R-review-mark-" +
				"carries-evidence).")
	}
	return nil
}

// Validate runs a proposal's own kind-specific validation (the same check
// internal/proposal.Apply runs internally before mutating a graph), exposed
// so callers can validate a proposal BEFORE writing it to disk (e.g. `hotam
// propose`) without duplicating any per-kind rule. It performs no disk I/O
// and does not mutate any graph.
func Validate(p Proposal) error {
	a, ok := p.(actor)
	if !ok {
		return fmt.Errorf("proposal kind %q does not implement validation", p.Kind())
	}
	return a.validate()
}

func trimNonEmpty(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		t := strings.TrimSpace(s)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}
