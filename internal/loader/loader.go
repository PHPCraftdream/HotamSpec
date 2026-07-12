package loader

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/PHPCraftdream/HotamSpecGo/internal/ontology"
)

type graphDTO struct {
	Axes         []ontology.Axis           `json:"axes"`
	Stakeholders []ontology.Stakeholder    `json:"stakeholders"`
	Assumptions  []ontology.Assumption     `json:"assumptions"`
	Requirements []ontology.Requirement    `json:"requirements"`
	Conflicts    []ontology.Conflict       `json:"conflicts"`
	Operators    []ontology.Operator       `json:"operators"`
	Processes    []ontology.Process        `json:"processes"`
	Goals        []ontology.Goal           `json:"goals"`
	EntityTypes  []ontology.EntityType     `json:"entity_types"`
	Entities     []ontology.EntityInstance `json:"entities"`
}

func LoadGraph(path string) (*ontology.Graph, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load graph: read %s: %w", path, err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var dto graphDTO
	if err := dec.Decode(&dto); err != nil {
		return nil, fmt.Errorf("load graph: decode %s: %w", path, err)
	}
	g := &ontology.Graph{
		Axes:         dto.Axes,
		Stakeholders: dto.Stakeholders,
		Assumptions:  dto.Assumptions,
		Requirements: dto.Requirements,
		Conflicts:    dto.Conflicts,
		Operators:    dto.Operators,
		Processes:    dto.Processes,
		Goals:        dto.Goals,
		EntityTypes:  dto.EntityTypes,
		Entities:     dto.Entities,
		SelfHosting:  resolveSelfHosting(path),
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
		Axes:         sortedCopy(g.Axes, func(a ontology.Axis) string { return a.Slug }),
		Stakeholders: sortedCopy(g.Stakeholders, func(s ontology.Stakeholder) string { return s.ID }),
		Assumptions:  sortedCopy(g.Assumptions, func(a ontology.Assumption) string { return a.ID }),
		Requirements: sortedCopy(g.Requirements, func(r ontology.Requirement) string { return r.ID }),
		Conflicts:    sortedCopy(g.Conflicts, func(c ontology.Conflict) string { return c.ID }),
		Operators:    sortedCopy(g.Operators, func(o ontology.Operator) string { return o.ID }),
		Processes:    sortedCopy(g.Processes, func(p ontology.Process) string { return p.ID }),
		Goals:        sortedCopy(g.Goals, func(gl ontology.Goal) string { return gl.ID }),
		EntityTypes:  sortedCopy(g.EntityTypes, func(et ontology.EntityType) string { return et.Slug }),
		Entities:     sortedCopy(g.Entities, func(e ontology.EntityInstance) string { return e.ID }),
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
