package proposal

import "github.com/PHPCraftdream/HotamSpecGo/internal/ontology"

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
	ID             string
	Claim          string
	Owner          string
	Status         string
	Why            string
	Assumptions    []string
	Relations      []ontology.Relation
	Enforcement    string
	EnforcedBy     []string
	MTag           string
	Enforceability string
	Summary        string
	CreatedAt      string
	SettledAt      string
	LastReviewedAt string
	ReviewAfter    string
	Evidence       []string
	SourceRefs     []string
}

func (p ProposedRequirement) Kind() string { return KindRequirement }
func (p ProposedRequirement) TargetAnchor() string { return p.ID }

type ProposedConflictTransition struct {
	ConflictID       string
	NewLifecycle     string
	DecidedBy        string
	RevisitMarker    string
	SharedAssumption string
	Derived          []string
	Variants         []ontology.Variant
	Date             string
	Verbatim         string
	Instrument       string
	ChosenVariant    string
}

func (p ProposedConflictTransition) Kind() string { return KindConflictTransition }
func (p ProposedConflictTransition) TargetAnchor() string { return p.ConflictID }

type ProposedRejection struct {
	RequirementID string
	Reason        string
	ReplacedBy    []string
}

func (p ProposedRejection) Kind() string { return KindRejection }
func (p ProposedRejection) TargetAnchor() string { return p.RequirementID }

type ProposedConflict struct {
	Axis             string
	Context          string
	Members          []string
	Steward          string
	SharedAssumption string
	Note             string
	InitialLifecycle string
	DecidedBy        string
}

func (p ProposedConflict) Kind() string { return KindConflict }
func (p ProposedConflict) TargetAnchor() string {
	return ontology.ConflictIdentity(p.Axis, p.Context)
}

type ProposedOperatorBudget struct {
	OperatorID string
	NewLimit   int
	NewMeasure string
	Why        string
}

func (p ProposedOperatorBudget) Kind() string { return KindOperatorBudget }
func (p ProposedOperatorBudget) TargetAnchor() string { return p.OperatorID }

type ProposedAxis struct {
	Slug        string
	Description string
	Why         string
}

func (p ProposedAxis) Kind() string { return KindAxis }
func (p ProposedAxis) TargetAnchor() string { return "Axis:" + p.Slug }

type ProposedStakeholder struct {
	ID     string
	Name   string
	Domain string
	Why    string
}

func (p ProposedStakeholder) Kind() string { return KindStakeholder }
func (p ProposedStakeholder) TargetAnchor() string { return p.ID }

type ProposedAssumption struct {
	ID        string
	Statement string
	Status    string
	Owner     string
	Why       string
	CreatedAt string
}

func (p ProposedAssumption) Kind() string { return KindAssumption }
func (p ProposedAssumption) TargetAnchor() string { return p.ID }

type ProposedAssumptionTransition struct {
	AssumptionID string
	NewStatus    string
	Reason       string
	DecidedBy    string
	Date         string
	Verbatim     string
	Instrument   string
}

func (p ProposedAssumptionTransition) Kind() string { return KindAssumptionTransition }
func (p ProposedAssumptionTransition) TargetAnchor() string { return p.AssumptionID }

type ProposedConflictMemberUpdate struct {
	ConflictID    string
	AddMembers    []string
	RemoveMembers []string
	DecidedBy     string
}

func (p ProposedConflictMemberUpdate) Kind() string { return KindConflictMemberUpdate }
func (p ProposedConflictMemberUpdate) TargetAnchor() string { return p.ConflictID }

type EntityTypeState struct {
	Name string
	Kind string
	Why  string
}

type EntityTypeTransition struct {
	Src   string
	Dst   string
	Event string
}

type EntityTypeField struct {
	Name      string
	Kind      string
	Required  bool
	RefTarget string
}

type ProposedEntityType struct {
	Slug        string
	Description string
	Why         string
	States      []EntityTypeState
	Transitions []EntityTypeTransition
	Cyclic      bool
	Fields      []EntityTypeField
}

func (p ProposedEntityType) Kind() string { return KindEntityType }
func (p ProposedEntityType) TargetAnchor() string { return "EntityType:" + p.Slug }
