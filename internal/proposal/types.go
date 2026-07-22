package proposal

import (
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

const (
	KindRequirement          = "Requirement"
	KindConflictTransition   = "ConflictTransition"
	KindRejection            = "Rejection"
	KindConflict             = "Conflict"
	KindOperatorBudget       = "OperatorBudget"
	KindAxis                 = "Axis"
	KindStakeholder          = "Stakeholder"
	KindAssumption           = "Assumption"
	KindAssumptionTransition = "AssumptionTransition"
	KindConflictMemberUpdate = "ConflictMemberUpdate"
	KindEntityType           = "EntityType"
	KindReviewMark           = "ReviewMark"
	KindProcess              = "Process"
	KindGateSignoffBatch     = "GateSignoffBatch"
	KindAssumptionRewrite    = "AssumptionRewrite"
)

type Proposal interface {
	Kind() string
	TargetAnchor() string
}

type actor interface {
	validate() error
	mutate(g *ontology.Graph, today string) error
}

type ProposedRequirement struct {
	ID             string              `json:"id"`
	Claim          string              `json:"claim"`
	Owner          string              `json:"owner"`
	Status         string              `json:"status"`
	Why            string              `json:"why"`
	Assumptions    []string            `json:"assumptions"`
	Relations      []ontology.Relation `json:"relations"`
	Enforcement    string              `json:"enforcement"`
	EnforcedBy     []string            `json:"enforced_by"`
	MTag           string              `json:"m_tag"`
	Enforceability string              `json:"enforceability"`
	Summary        string              `json:"summary"`
	CreatedAt      string              `json:"created_at"`
	SettledAt      string              `json:"settled_at"`
	LastReviewedAt string              `json:"last_reviewed_at"`
	ReviewAfter    string              `json:"review_after"`
	Evidence       []string            `json:"evidence"`
	SourceRefs     []string            `json:"source_refs"`
	BlockedOn      string              `json:"blocked_on"`
	ImplementedBy  []string            `json:"implemented_by"`
	VerifiedBy     []string            `json:"verified_by"`
}

func (p ProposedRequirement) Kind() string         { return KindRequirement }
func (p ProposedRequirement) TargetAnchor() string { return p.ID }

type ProposedConflictTransition struct {
	ConflictID       string             `json:"conflict_id"`
	NewLifecycle     string             `json:"new_lifecycle"`
	DecidedBy        string             `json:"decided_by"`
	RevisitMarker    string             `json:"revisit_marker"`
	SharedAssumption string             `json:"shared_assumption"`
	Derived          []string           `json:"derived"`
	Variants         []ontology.Variant `json:"variants"`
	Date             string             `json:"date"`
	Verbatim         string             `json:"verbatim"`
	Instrument       string             `json:"instrument"`
	ChosenVariant    string             `json:"chosen_variant"`
	// SourceRefs, when non-empty, REPLACES the target Conflict's existing
	// SourceRefs (same "empty preserves, non-empty replaces" idiom Derived/
	// Variants above already use on this same proposal kind) -- lets a
	// transition attach provenance for the decision it is recording (e.g.
	// the doc/ticket that documents a DECIDED lifecycle) without a separate
	// proposal kind.
	SourceRefs []string `json:"source_refs"`
}

func (p ProposedConflictTransition) Kind() string         { return KindConflictTransition }
func (p ProposedConflictTransition) TargetAnchor() string { return p.ConflictID }

type ProposedRejection struct {
	RequirementID string   `json:"requirement_id"`
	Reason        string   `json:"reason"`
	ReplacedBy    []string `json:"replaced_by"`
}

func (p ProposedRejection) Kind() string         { return KindRejection }
func (p ProposedRejection) TargetAnchor() string { return p.RequirementID }

type ProposedConflict struct {
	Axis             string   `json:"axis"`
	Context          string   `json:"context"`
	Members          []string `json:"members"`
	Resolver         string   `json:"resolver"`
	SharedAssumption string   `json:"shared_assumption"`
	Note             string   `json:"note"`
	InitialLifecycle string   `json:"initial_lifecycle"`
	DecidedBy        string   `json:"decided_by"`
	SourceRefs       []string `json:"source_refs"`
}

func (p ProposedConflict) Kind() string { return KindConflict }
func (p ProposedConflict) TargetAnchor() string {
	return ontology.ConflictIdentity(p.Axis, p.Context)
}

type ProposedOperatorBudget struct {
	OperatorID string `json:"operator_id"`
	NewLimit   int    `json:"new_limit"`
	NewMeasure string `json:"new_measure"`
	Why        string `json:"why"`
}

func (p ProposedOperatorBudget) Kind() string         { return KindOperatorBudget }
func (p ProposedOperatorBudget) TargetAnchor() string { return p.OperatorID }

// ProposedAxis is a CREATE-or-UPDATE proposal for an Axis node.
//
// CREATE (p.Slug not yet in the graph): unchanged since Axis's introduction
// -- description is required, a duplicate slug is rejected via errDuplicate.
//
// UPDATE (p.Slug already names an Axis in the graph): REPLACES the existing
// Axis.Description with p.Description (coalesceStr's "empty preserves,
// non-empty replaces" idiom -- the same one ProposedRequirement.mutate
// already uses for Why/Summary/etc). A HistoryEntry recording the
// description diff is appended, mirroring the History-on-mutation pattern
// ProposedRequirement/ProposedEntityType/ProposedProcess already use -- see
// ProposedAxis.mutate's doc comment in mutate.go.
type ProposedAxis struct {
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Why         string `json:"why"`
}

func (p ProposedAxis) Kind() string         { return KindAxis }
func (p ProposedAxis) TargetAnchor() string { return "Axis:" + p.Slug }

type ProposedStakeholder struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Domain string `json:"domain"`
	Why    string `json:"why"`
}

func (p ProposedStakeholder) Kind() string         { return KindStakeholder }
func (p ProposedStakeholder) TargetAnchor() string { return p.ID }

type ProposedAssumption struct {
	ID         string   `json:"id"`
	Statement  string   `json:"statement"`
	Status     string   `json:"status"`
	Owner      string   `json:"owner"`
	Why        string   `json:"why"`
	CreatedAt  string   `json:"created_at"`
	SourceRefs []string `json:"source_refs"`
}

func (p ProposedAssumption) Kind() string         { return KindAssumption }
func (p ProposedAssumption) TargetAnchor() string { return p.ID }

type ProposedAssumptionTransition struct {
	AssumptionID string `json:"assumption_id"`
	NewStatus    string `json:"new_status"`
	Reason       string `json:"reason"`
	DecidedBy    string `json:"decided_by"`
	Date         string `json:"date"`
	Verbatim     string `json:"verbatim"`
	Instrument   string `json:"instrument"`
}

func (p ProposedAssumptionTransition) Kind() string         { return KindAssumptionTransition }
func (p ProposedAssumptionTransition) TargetAnchor() string { return p.AssumptionID }

// ProposedAssumptionRewrite is a CLEAN REWRITE of an EXISTING Assumption's
// Statement -- distinct from ProposedAssumptionTransition, which changes
// Status and appends a "[STATUS] reason" suffix onto Statement as a SIDE
// EFFECT of a status decision (see ProposedAssumptionTransition.mutate).
// A rewrite REPLACES Statement outright with NewStatement, touching no
// other field, for the case where an assumption's WORDING needs correcting
// (a typo, an ambiguity, a scope clarification) with no status change
// involved at all.
//
// Reason is REQUIRED (non-empty): a rewrite with no recorded reason is
// silent, unaudited drift of what the assumption even claims -- the exact
// failure mode ProposedAssumptionTransition's own Reason requirement
// already guards against for status changes (see validate.go), applied
// here to content changes instead. A HistoryEntry recording the statement
// diff (old -> new) plus Reason is appended to the Assumption's History on
// every apply -- see ProposedAssumptionRewrite.mutate's doc comment in
// mutate.go for why this is non-negotiable, not merely a convention.
type ProposedAssumptionRewrite struct {
	AssumptionID string `json:"assumption_id"`
	NewStatement string `json:"new_statement"`
	Reason       string `json:"reason"`
}

func (p ProposedAssumptionRewrite) Kind() string         { return KindAssumptionRewrite }
func (p ProposedAssumptionRewrite) TargetAnchor() string { return p.AssumptionID }

type ProposedConflictMemberUpdate struct {
	ConflictID    string   `json:"conflict_id"`
	AddMembers    []string `json:"add_members"`
	RemoveMembers []string `json:"remove_members"`
	DecidedBy     string   `json:"decided_by"`
}

func (p ProposedConflictMemberUpdate) Kind() string         { return KindConflictMemberUpdate }
func (p ProposedConflictMemberUpdate) TargetAnchor() string { return p.ConflictID }

type EntityTypeState struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
	Why  string `json:"why"`
}

type EntityTypeTransition struct {
	Src   string `json:"src"`
	Dst   string `json:"dst"`
	Event string `json:"event"`
}

type EntityTypeField struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Required  bool   `json:"required"`
	RefTarget string `json:"ref_target"`
}

type ProposedEntityType struct {
	Slug        string                 `json:"slug"`
	Description string                 `json:"description"`
	Why         string                 `json:"why"`
	States      []EntityTypeState      `json:"states"`
	Transitions []EntityTypeTransition `json:"transitions"`
	Cyclic      bool                   `json:"cyclic"`
	Fields      []EntityTypeField      `json:"fields"`
}

func (p ProposedEntityType) Kind() string         { return KindEntityType }
func (p ProposedEntityType) TargetAnchor() string { return "EntityType:" + p.Slug }

// ProposedReviewMark is a minimal, single-purpose proposal for stamping an
// EXISTING requirement's freshness metadata (last_reviewed_at, review_after,
// evidence) without going through the general-purpose ProposedRequirement
// patch path. It exists because backfilling review metadata across 275
// requirements via ProposedRequirement would require re-stating every other
// field just to avoid an unintended coalesce, and because a review-mark is
// conceptually a distinct act (the resolver re-affirmed a claim is still
// true) from a content edit — it deserves its own typed node so
// `hotam due` / freshness tooling can distinguish "reviewed" history from
// "content changed" history if it ever needs to (mirrors
// ProposedAssumptionTransition's read-attention-only "record it" shape).
type ProposedReviewMark struct {
	RequirementID string   `json:"requirement_id"`
	ReviewedAt    string   `json:"reviewed_at"`
	ReviewAfter   string   `json:"review_after"`
	Evidence      []string `json:"evidence"`
}

func (p ProposedReviewMark) Kind() string         { return KindReviewMark }
func (p ProposedReviewMark) TargetAnchor() string { return p.RequirementID }

// ProposedStep mirrors ontology.Step's shape for the wire format (the same
// pattern EntityTypeState/EntityTypeTransition already use to decouple the
// proposal JSON shape from the landed ontology type).
type ProposedStep struct {
	Name         string `json:"name"`
	RequiresRole string `json:"requires_role"`
	Invokes      string `json:"invokes"`
	Why          string `json:"why"`
}

// ProposedProcess is a CREATE-or-UPDATE proposal for a Process node (the
// §Process opt-in behavioral aspect: a Lifecycle + ordered Steps +
// roles_required + drives_entities, internal/ontology/process.go).
//
// CREATE (p.ID not yet in the graph): unchanged since task #199 -- a
// duplicate ID is rejected via errDuplicate, not merged.
//
// UPDATE (p.ID already names a Process in the graph, since task #212):
// mirrors ProposedEntityType's UPDATE mode (ffa4977) -- deliberately narrow.
// p.Steps and p.DrivesEntities are APPENDED to the existing lists (never
// redefining, removing, or reordering an existing step/slug); p.RolesRequired
// is UNIONED into the existing list; p.Why, if non-empty, REPLACES the
// existing Why (a correction, not an append). See ProposedProcess.mutate's
// doc comment in mutate.go for the full UPDATE contract.
//
// Lifecycle is NOT author-supplied on either CREATE or UPDATE: every Process
// in this codebase (the one worked example, PR-closed-loop, in
// domains/hotam-spec-self/graph.json) uses the single shared
// ontology.ProcessLifecycle (READY/RUNNING/BLOCKED/DONE/ABANDONED) -- there is
// no second Process lifecycle shape anywhere in the graph or the ontology
// package to choose between, so mutate() always stamps
// ontology.ProcessLifecycle rather than accepting an author-supplied one
// (avoids inventing a second, untested lifecycle-authoring wire format for a
// aspect that has exactly one instance in the wild); an UPDATE never touches
// an already-landed Process's Lifecycle at all.
type ProposedProcess struct {
	ID             string         `json:"id"`
	Steps          []ProposedStep `json:"steps"`
	RolesRequired  []string       `json:"roles_required"`
	DrivesEntities []string       `json:"drives_entities"`
	Why            string         `json:"why"`
}

func (p ProposedProcess) Kind() string         { return KindProcess }
func (p ProposedProcess) TargetAnchor() string { return p.ID }

// GateSignoffEntry is one transition inside a ProposedGateSignoffBatch: "this
// Requirement's gate_signoffs gains one new ontology.GateSignoff entry."
// Stage/State/DeferredReason/Evidence/PipelineRun mirror
// ontology.GateSignoff's own fields (see internal/ontology/gate_signoff.go);
// DecidedBy/Date/Verbatim/Instrument mirror ontology.Signoff's fields (see
// internal/ontology/signoff.go) — the same wire-shape-decoupling pattern
// ProposedConflictTransition already uses to carry Signoff's fields flat on
// the proposal rather than nesting an ontology.Signoff literal in the JSON.
type GateSignoffEntry struct {
	RequirementID  string   `json:"requirement_id"`
	Stage          string   `json:"stage"`
	State          string   `json:"state"`
	DeferredReason string   `json:"deferred_reason"`
	Evidence       []string `json:"evidence"`
	PipelineRun    string   `json:"pipeline_run"`
	DecidedBy      string   `json:"decided_by"`
	Date           string   `json:"date"`
	Verbatim       string   `json:"verbatim"`
	Instrument     string   `json:"instrument"`
}

// ProposedGateSignoffBatch is a MULTI-TARGET proposal: it applies N
// GateSignoffEntry transitions, each appending ONE new ontology.GateSignoff
// to the named Requirement's GateSignoffs list, in a SINGLE apply-proposal —
// the shape a staged-gate methodology (e.g. prat/gpsm-sm's P-G0..P-G4) needs
// when an entire wave of requirements clears the same stage together (32
// requirements signing off P-G1 in one sitting should not require 32
// separate single-target proposals). It is intentionally its OWN Proposal
// kind (rather than, say, N separate ProposedRequirement UPDATEs folded into
// one JSON array) because "one Kind, one TargetAnchor, one atomic mutate"
// matches how apply-proposal / ApplyBatch already treat every OTHER Proposal
// value — see mutate.go's ProposedGateSignoffBatch.mutate for how each entry
// is applied to its own Requirement inside ONE mutate() call, and
// history.go's summarizeFieldDiff extension for how each affected
// Requirement gets its own HistoryEntry recording the GateSignoffs diff.
//
// TargetAnchor() joins every entry's RequirementID (comma-separated, stable
// input order, no de-duplication) rather than picking one — every other
// Proposal kind's TargetAnchor names the ONE node it targets; this kind
// targets several, so the join is the honest representation for the log
// lines / lock-file notes that read TargetAnchor() (see apply.go's
// `p.Kind() + " " + p.TargetAnchor() + " applied"`).
type ProposedGateSignoffBatch struct {
	Entries []GateSignoffEntry `json:"entries"`
}

func (p ProposedGateSignoffBatch) Kind() string { return KindGateSignoffBatch }
func (p ProposedGateSignoffBatch) TargetAnchor() string {
	ids := make([]string, len(p.Entries))
	for i, e := range p.Entries {
		ids[i] = e.RequirementID
	}
	return strings.Join(ids, ",")
}
