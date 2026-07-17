package loader

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

type graphDTO struct {
	SchemaVersion int                       `json:"schema_version"`
	Axes          []ontology.Axis           `json:"axes"`
	Stakeholders  []ontology.Stakeholder    `json:"stakeholders"`
	Assumptions   []ontology.Assumption     `json:"assumptions"`
	Requirements  []ontology.Requirement    `json:"requirements"`
	Conflicts     []ontology.Conflict       `json:"conflicts"`
	Operators     []ontology.Operator       `json:"operators"`
	Processes     []ontology.Process        `json:"processes"`
	Goals         []ontology.Goal           `json:"goals"`
	EntityTypes   []ontology.EntityType     `json:"entity_types"`
	Entities      []ontology.EntityInstance `json:"entities"`
}

func LoadGraph(path string) (*ontology.Graph, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load graph: read %s: %w", path, err)
	}

	// Version probe: decode just schema_version with a lenient decoder (no
	// DisallowUnknownFields) so a genuinely newer format is reported with a
	// clear, actionable error instead of the opaque "json: unknown field"
	// that the strict decoder below would emit for the new top-level fields a
	// future version would carry.
	var probe struct {
		SchemaVersion int `json:"schema_version"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, fmt.Errorf("load graph: decode %s: %w", path, err)
	}
	sv := probe.SchemaVersion
	if sv == 0 {
		// Missing/zero schema_version → treat as the current version for
		// backward compatibility with pre-version graph.json files.
		sv = ontology.CurrentSchemaVersion
	}
	switch sv {
	case ontology.CurrentSchemaVersion:
		// today's format — proceed to the strict decode below.
	case 1:
		// v1 is a real prior version (the graph.json format before the additive
		// Requirement.blocked_on field landed). It requires NO data
		// transformation: blocked_on is a purely additive OPTIONAL field
		// (`json:"blocked_on,omitempty"`), so a v1 file that simply lacks it
		// decodes losslessly into the v2 Requirement struct as the Go
		// zero-value "" — there is nothing for DisallowUnknownFields to reject
		// (v1 data has no field the v2 struct does not recognize). Proceed to
		// the same strict decode below. A genuine data-SHAPE change in a future
		// version (a renamed/removed field, a changed shape) would need real
		// transformation code inserted here before the strict decode.
	case 2:
		// v2 is a real prior version (the graph.json format before the additive
		// Requirement.implemented_by and Requirement.verified_by fields landed).
		// It requires NO data transformation: implemented_by/verified_by are
		// purely additive OPTIONAL fields (`json:"implemented_by,omitempty"` /
		// `json:"verified_by,omitempty"`), so a v2 file that simply lacks them
		// decodes losslessly into the v3 Requirement struct as nil slices —
		// there is nothing for DisallowUnknownFields to reject (v2 data has no
		// field the v3 struct does not recognize). Proceed to the same strict
		// decode below. A genuine data-SHAPE change in a future version (a
		// renamed/removed field, a changed shape) would need real
		// transformation code inserted here before the strict decode.
	default:
		if sv > ontology.CurrentSchemaVersion {
			return nil, fmt.Errorf(
				"load graph: %s: schema_version %d is newer than this hotam binary supports (max %d) — upgrade hotam or downgrade the graph",
				path, sv, ontology.CurrentSchemaVersion)
		}
		// sv < 1 is unreachable: 1 is the first versioned format. A missing
		// schema_version is normalized to CurrentSchemaVersion above; a real
		// prior version lower than 1 does not exist.
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var dto graphDTO
	if err := dec.Decode(&dto); err != nil {
		return nil, fmt.Errorf("load graph: decode %s: %w", path, err)
	}
	g := &ontology.Graph{
		SchemaVersion: ontology.CurrentSchemaVersion,
		Axes:          dto.Axes,
		Stakeholders:  dto.Stakeholders,
		Assumptions:   dto.Assumptions,
		Requirements:  dto.Requirements,
		Conflicts:     dto.Conflicts,
		Operators:     dto.Operators,
		Processes:     dto.Processes,
		Goals:         dto.Goals,
		EntityTypes:   dto.EntityTypes,
		Entities:      dto.Entities,
		SelfHosting:   resolveSelfHosting(path),
		DomainDir:     filepath.Dir(path),
		Discipline:    ResolveDiscipline(path),
	}
	if err := validateGraph(g); err != nil {
		return nil, fmt.Errorf("load graph: %s: %w", path, err)
	}
	return g, nil
}

func WriteGraph(path string, g *ontology.Graph) error {
	if g == nil {
		return fmt.Errorf("write graph: nil graph")
	}
	data, err := marshalCanonical(g)
	if err != nil {
		return fmt.Errorf("write graph: marshal: %w", err)
	}
	if err := atomicWriteFile(path, data); err != nil {
		return fmt.Errorf("write graph: %s: %w", path, err)
	}
	if err := WriteLock(path, ""); err != nil {
		return fmt.Errorf("write graph: lock: %w", err)
	}
	return nil
}

func marshalCanonical(g *ontology.Graph) ([]byte, error) {
	dto := graphDTO{
		SchemaVersion: ontology.CurrentSchemaVersion,
		Axes:          sortedCopy(g.Axes, func(a ontology.Axis) string { return a.Slug }),
		Stakeholders:  sortedCopy(g.Stakeholders, func(s ontology.Stakeholder) string { return s.ID }),
		Assumptions:   sortedCopy(g.Assumptions, func(a ontology.Assumption) string { return a.ID }),
		Requirements:  sortedCopy(g.Requirements, func(r ontology.Requirement) string { return r.ID }),
		Conflicts:     sortedCopy(g.Conflicts, func(c ontology.Conflict) string { return c.ID }),
		Operators:     sortedCopy(g.Operators, func(o ontology.Operator) string { return o.ID }),
		Processes:     sortedCopy(g.Processes, func(p ontology.Process) string { return p.ID }),
		Goals:         sortedCopy(g.Goals, func(gl ontology.Goal) string { return gl.ID }),
		EntityTypes:   sortedCopy(g.EntityTypes, func(et ontology.EntityType) string { return et.Slug }),
		Entities:      sortedCopy(g.Entities, func(e ontology.EntityInstance) string { return e.ID }),
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(dto); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func atomicWriteFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
		return fmt.Errorf("mkdir %s: %w", dir, mkErr)
	}
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("create %s: %w", tmp, err)
	}
	cleanup := func() { _ = os.Remove(tmp) }
	if _, err := f.Write(data); err != nil {
		f.Close()
		cleanup()
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		cleanup()
		return fmt.Errorf("sync %s: %w", tmp, err)
	}
	if err := f.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		cleanup()
		return fmt.Errorf("rename %s -> %s: %w", tmp, path, err)
	}
	return nil
}

func resolveSelfHosting(graphPath string) bool {
	manifestPath := filepath.Join(filepath.Dir(graphPath), "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return false
	}
	var m struct {
		SelfHosting bool `json:"self_hosting"`
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return false
	}
	return m.SelfHosting
}

// GenProfileFull and GenProfileConsumer are the two accepted values for the
// gen-spec profile (R-gen-spec-profile). gen-spec's full output (~90 docs/gen
// files per domain) is correct for a self-hosting domain developing Hotam-Spec
// itself; an external business consumer needs only the subset that is not
// pure framework-self-documentation noise.
const (
	GenProfileFull     = "full"
	GenProfileConsumer = "consumer"
)

// ResolveGenProfile reads the optional "gen_profile" field from the
// manifest.json sitting next to graph.json, mirroring resolveSelfHosting's
// exact pattern (read manifest, tolerate a missing file, tolerate malformed
// JSON, default when absent). Returns GenProfileFull ("full") for every
// absent/missing-field/malformed/unrecognized case — preserving 100%
// backward compatibility with every manifest.json in this repo and in the
// wild that predates the profile feature (they carry no gen_profile field, so
// they keep resolving to the full output set, byte-identical to before).
func ResolveGenProfile(graphPath string) string {
	manifestPath := filepath.Join(filepath.Dir(graphPath), "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return GenProfileFull
	}
	var m struct {
		GenProfile string `json:"gen_profile"`
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return GenProfileFull
	}
	switch m.GenProfile {
	case GenProfileConsumer, GenProfileFull:
		return m.GenProfile
	default:
		return GenProfileFull
	}
}

// ResolveRequireProvenance reads the optional "require_provenance" field from
// the manifest.json sitting next to graph.json, mirroring resolveSelfHosting's
// exact pattern (read manifest, tolerate a missing file, tolerate malformed
// JSON, default when absent/malformed). Returns false ("provenance NOT
// required") for every absent/missing-field/malformed case — preserving 100%
// backward compatibility with every manifest.json in this repo and in the
// wild that predates the provenance-gate feature (they carry no
// require_provenance field, so they keep landing requirements exactly as
// before, with no gate applied). Opting in is an explicit, per-domain choice
// (`"require_provenance": true` in manifest.json) — see
// cmd/hotam/provenance_gate.go for what the flag then enforces.
func ResolveRequireProvenance(graphPath string) bool {
	manifestPath := filepath.Join(filepath.Dir(graphPath), "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return false
	}
	var m struct {
		RequireProvenance bool `json:"require_provenance"`
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return false
	}
	return m.RequireProvenance
}

// DisciplineFull is the one recognized non-empty value of manifest.json's
// optional "discipline" field (PLAN-scenario-generated-spec.md §2 D4, task
// W2.1). An empty/absent/unrecognized value means "soft discipline" — today's
// long-standing behavior, unchanged: check_settled_requires_scenario
// (internal/invariants/scenario_discipline.go) is an honest no-op for such a
// domain, exactly like every optional-field NO-OP contract in this file
// (ResolveGenProfile, ResolveRequireProvenance). Only "full" turns that same
// check into a real, per-SETTLED-requirement gate.
//
// ONE-WAY SEMANTICS (steward decision, PLAN-scenario-generated-spec.md §2 D4):
// flipping a domain's manifest.json from discipline:"" to discipline:"full"
// is meant to be a ONE-WAY door — once a domain has migrated its SETTLED
// requirements to carry real enforced_by / (implemented_by+verified_by+
// scenario) coverage and declared discipline:"full", silently flipping it
// back to "" would be a silent DOWNGRADE of a promise already made public in
// the domain's own manifest (exactly the kind of quiet regression
// R-no-hand-edit-graph and check_graph_lock_pins_graph_json exist to catch
// for graph.json itself). This engine version does NOT yet mechanically
// enforce the one-way property for manifest.json (manifest.json, unlike
// graph.json, has no graph.lock-style content pin today) — ResolveDiscipline
// is a pure READER, deliberately as small and honest as ResolveGenProfile's
// own precedent. The one-way discipline is, for now, a DOCUMENTED convention
// (this comment + PLAN-scenario-generated-spec.md §2 D4) plus ordinary code
// review / R-no-hand-edit-graph-adjacent scrutiny of manifest.json diffs —
// the same honesty-over-mechanism boundary check_verified_by_test_has_teeth's
// own doc comment draws between "the structural floor" and "the mirror
// audit". A future wave MAY harden this into a real mechanical gate (e.g. a
// manifest.lock pin, or a check that refuses an all-violations run whose
// manifest.json shows discipline flipping from "full" to "" relative to the
// last landed commit) — tracked as follow-up, not silently promised here.
const DisciplineFull = "full"

// ResolveDiscipline reads the optional "discipline" field from the
// manifest.json sitting next to graph.json, mirroring resolveSelfHosting's /
// ResolveGenProfile's exact pattern (read manifest, tolerate a missing file,
// tolerate malformed JSON, default when absent/unrecognized). Returns "" (the
// soft-discipline default) for every absent/missing-field/malformed/
// unrecognized case — preserving 100% backward compatibility with every
// manifest.json in this repo and in the wild that predates the discipline
// field (they carry no discipline field, so they keep resolving to "",
// meaning check_settled_requires_scenario stays an honest no-op for them,
// byte-identical to before this field existed). Only the single literal
// value "full" (DisciplineFull) is recognized; any other non-empty string
// (a typo, a future value not yet supported) is treated the same as absent —
// deliberately, so a malformed opt-in can never silently masquerade as a
// real one.
func ResolveDiscipline(graphPath string) string {
	manifestPath := filepath.Join(filepath.Dir(graphPath), "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return ""
	}
	var m struct {
		Discipline string `json:"discipline"`
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return ""
	}
	if m.Discipline == DisciplineFull {
		return DisciplineFull
	}
	return ""
}

// DomainPresentation carries the optional DOMAIN-MAP presentation fields of a
// domain's manifest.json: purpose (one-line description), goals (bullet list),
// and director (the accountable steward role/name). All three are optional —
// a manifest without them (every manifest predating task #210) yields the
// zero value, and the DOMAIN-MAP renderer falls back to em-dash placeholders,
// exactly as before.
type DomainPresentation struct {
	Purpose  string   `json:"purpose"`
	Goals    []string `json:"goals"`
	Director string   `json:"director"`
}

// ResolveDomainPresentation reads the optional "purpose"/"goals"/"director"
// fields from the manifest.json sitting next to graph.json, mirroring
// resolveSelfHosting's exact pattern (read manifest, tolerate a missing file,
// tolerate malformed JSON, default when absent). Returns the zero
// DomainPresentation for every absent/missing-field/malformed case —
// preserving 100% backward compatibility with every manifest.json in this
// repo and in the wild that predates these fields (task #210: the DOMAIN-MAP
// purpose/goals/director now live on disk in the domain's own manifest, not
// in a hardcoded engine-side table).
func ResolveDomainPresentation(graphPath string) DomainPresentation {
	manifestPath := filepath.Join(filepath.Dir(graphPath), "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return DomainPresentation{}
	}
	var m DomainPresentation
	if err := json.Unmarshal(data, &m); err != nil {
		return DomainPresentation{}
	}
	return m
}

func validateGraph(g *ontology.Graph) error {
	var errs []string
	add := func(format string, args ...any) {
		errs = append(errs, fmt.Sprintf(format, args...))
	}

	for i, a := range g.Axes {
		if a.Slug == "" {
			add("axes[%d]: empty slug", i)
		}
	}
	for i, s := range g.Stakeholders {
		if s.ID == "" {
			add("stakeholders[%d]: empty id", i)
		}
	}
	for i, a := range g.Assumptions {
		if a.ID == "" {
			add("assumptions[%d]: empty id", i)
		}
		if a.Status != "" {
			if _, ok := ontology.AssumptionStates[a.Status]; !ok {
				add("assumptions[%d] %s: invalid status %q", i, a.ID, a.Status)
			}
		}
	}
	for i, r := range g.Requirements {
		if r.ID == "" {
			add("requirements[%d]: empty id", i)
		}
		if r.Status != "" {
			if _, ok := ontology.RequirementStatusLifecycle.Matches(r.Status); !ok {
				add("requirements[%d] %s: invalid status %q", i, r.ID, r.Status)
			}
		}
		if r.Enforcement != "" {
			if _, ok := ontology.EnforcementLevels[r.Enforcement]; !ok {
				add("requirements[%d] %s: invalid enforcement %q", i, r.ID, r.Enforcement)
			}
		}
		if r.Enforceability != "" {
			if _, ok := ontology.EnforceabilityKinds[r.Enforceability]; !ok {
				add("requirements[%d] %s: invalid enforceability %q", i, r.ID, r.Enforceability)
			}
		}
		for j, rel := range r.Relations {
			if _, ok := ontology.RelationKinds[rel.Kind]; !ok {
				add("requirements[%d] %s: relations[%d]: invalid kind %q", i, r.ID, j, rel.Kind)
			}
		}
	}
	for i, c := range g.Conflicts {
		if c.ID == "" {
			add("conflicts[%d]: empty id", i)
		}
		if c.Axis == "" {
			add("conflicts[%d] %s: empty axis", i, c.ID)
		}
		if c.Lifecycle != "" {
			if _, ok := ontology.ConflictLifecycle.Matches(c.Lifecycle); !ok {
				add("conflicts[%d] %s: invalid lifecycle %q", i, c.ID, c.Lifecycle)
			}
		}
	}
	for i, op := range g.Operators {
		if op.ID == "" {
			add("operators[%d]: empty id", i)
		}
		if op.ContextBudget.Measure != "" {
			if _, ok := ontology.BudgetMeasures[op.ContextBudget.Measure]; !ok {
				add("operators[%d] %s: invalid budget measure %q", i, op.ID, op.ContextBudget.Measure)
			}
		}
	}
	for i, p := range g.Processes {
		if p.ID == "" {
			add("processes[%d]: empty id", i)
		}
	}
	for i, gl := range g.Goals {
		if gl.ID == "" {
			add("goals[%d]: empty id", i)
		}
		if gl.TargetState.Kind != "" {
			if _, ok := ontology.TargetKinds[gl.TargetState.Kind]; !ok {
				add("goals[%d] %s: invalid target_state.kind %q", i, gl.ID, gl.TargetState.Kind)
			}
		}
	}
	for i, et := range g.EntityTypes {
		if et.Slug == "" {
			add("entity_types[%d]: empty slug", i)
		}
	}
	for i, e := range g.Entities {
		if e.ID == "" {
			add("entities[%d]: empty id", i)
		}
		if e.EntityType == "" {
			add("entities[%d] %s: empty entity_type", i, e.ID)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("graph validation failed:\n  %s", strings.Join(errs, "\n  "))
	}
	return nil
}
