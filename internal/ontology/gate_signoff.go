package ontology

// GateSignoff is one domain-declared gate-passage FACT attached to a
// Requirement: "this Requirement passed (or was explicitly deferred at) gate
// stage X, in pipeline run Y." It exists so a domain that runs its
// Requirements through a staged review methodology (e.g. prat/gpsm-sm's
// P-G0..P-G4 gates) has ONE typed carrier on the graph recording per-stage
// passage, instead of that fact being reconstructed from prose scattered
// across History entries, ticket bodies, and commit messages (the drift this
// type closes — see PLAN-hotamspec-adoption.md's gate-signoff review).
//
// Stage is a plain string, NOT a hardcoded enum of engine constants. The
// engine must not know the specific stage names or their order — that is
// ONE domain's methodology (prat/gpsm-sm's P-G0..P-G4), while this same
// engine also serves domains with a different staged-gate vocabulary or no
// gate discipline at all (hotam-spec-self, hotam-dev). A domain that wants
// its gate stages validated for monotonic order declares that order as DATA
// in its own manifest.json ("gate_stage_order": [...]) — see
// internal/loader.ResolveGateStageOrder. A domain that never declares
// gate_stage_order gets no ordering check at all (honest no-op,
// R-uncrystallizable-automated: the engine will not invent a stage order no
// domain declared).
//
// It reuses the existing Signoff payload (DecidedBy/Date/Verbatim/
// Instrument/ChosenVariant) for the human-provenance half of the record,
// exactly the same "payload, not a new node type" pattern Signoff itself
// already establishes for Conflict/Assumption decisions (see signoff.go).
// Signoff is a pointer here (mirrors Conflict.Signoff/Assumption.Signoff)
// rather than embedded: a GateSignoff can exist in State=DEFERRED with no
// human decision yet recorded (Signoff == nil), and a named field reads
// clearer than an embedded pointer at the call sites that construct or
// inspect a GateSignoff.
const (
	GateSignoffStateSigned   = "SIGNED"
	GateSignoffStateDeferred = "DEFERRED"
)

var GateSignoffStates = map[string]struct{}{
	GateSignoffStateSigned:   {},
	GateSignoffStateDeferred: {},
}

type GateSignoff struct {
	// Stage names the gate this signoff records passage (or deferral) of, in
	// whatever vocabulary the owning domain's manifest.json gate_stage_order
	// declares (e.g. "P-G1"). Opaque to the engine.
	Stage string `json:"stage"`
	// State is one of GateSignoffStateSigned ("SIGNED": the requirement
	// passed this stage) or GateSignoffStateDeferred ("DEFERRED": the
	// requirement's passage of this stage was explicitly postponed, with
	// DeferredReason recording why).
	State string `json:"state"`
	// DeferredReason is REQUIRED (non-empty) when State is DEFERRED — a
	// deferral with no recorded reason is drift, not a decision (mirrors
	// ProposedAssumptionTransition's identical requirement for its own
	// Reason field). May reference a Conflict node id (the `C-[0-9a-f]{8}`
	// shape ConflictIdentity produces) when the deferral is because an open
	// Conflict blocks this stage; when it does, the referenced Conflict MUST
	// exist in the graph (see check_gate_signoff_deferred_conflict_resolves).
	DeferredReason string `json:"deferred_reason,omitempty"`
	// Evidence lists supporting artifact references for this signoff (test
	// run output, review notes, ...) — same free-form shape as
	// Requirement.Evidence.
	Evidence []string `json:"evidence,omitempty"`
	// PipelineRun names the pipeline execution this signoff belongs to — the
	// unit the monotonicity invariant (check_gate_signoff_monotonic) checks
	// stage order WITHIN: a re-run of the pipeline starts a fresh
	// PipelineRun, and stage order is only meaningful relative to signoffs
	// sharing the same run.
	PipelineRun string `json:"pipeline_run"`
	// Signoff carries the human provenance (decided_by/date/verbatim/
	// instrument/chosen_variant) for a SIGNED gate passage. nil is valid for
	// a DEFERRED entry that has no decision to attribute yet.
	Signoff *Signoff `json:"signoff,omitempty"`
}

// GateSignoffs is the Requirement-level field carrying every declared
// GateSignoff for that requirement, across however many stages/pipeline runs
// apply. omitempty preserves byte-identical JSON for every existing
// Requirement that has never used this field.
