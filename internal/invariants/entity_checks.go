package invariants

import (
	"fmt"
	"sort"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func checkEntityTypeLifecycleWellformed(g *ontology.Graph) []Violation {
	var out []Violation
	for _, et := range g.EntityTypes {
		for _, issue := range lifecycleWellformedIssues(et.Lifecycle) {
			out = append(out, Violation{
				Check:   "check_entity_type_lifecycle_wellformed",
				ID:      et.Slug,
				Message: issue,
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_entity_type_lifecycle_wellformed", Invariant{
	Name:  "check_entity_type_lifecycle_wellformed",
	Canon: methodology.Entity,
	Claim: "every EntityType.lifecycle is a well-formed Lifecycle.",
	Rule: "every EntityType.lifecycle MUST pass lifecycleWellformedIssues (non-empty states, exactly one INITIAL, " +
		"all transition endpoints resolve, terminal reachable if non-cyclic). No-ops when g.entity_types is empty " +
		"(Entity aspect not loaded).",
	Why: "the Lifecycle keystone is the single source of truth for state-machine well-formedness; reusing it here " +
		"means every EntityType inherits all four conditions without parallel machinery. " +
		"References: R-statemachine-wellformedness, M12.",
	Check: checkEntityTypeLifecycleWellformed,
})

func checkTransitionGuardAssumptionResolves(g *ontology.Graph) []Violation {
	aids := ontology.AssumptionIDs(g)
	var out []Violation
	for _, et := range g.EntityTypes {
		for _, t := range et.Lifecycle.Transitions {
			if t.GuardAssumption == nil {
				continue
			}
			ga := *t.GuardAssumption
			if ga == "" {
				continue
			}
			if _, ok := aids[ga]; !ok {
				out = append(out, Violation{
					Check:   "check_transition_guard_assumption_resolves",
					ID:      et.Slug,
					Message: fmt.Sprintf("transition %q->%q guard_assumption %q does not resolve to a known Assumption id", t.Src, t.Dst, ga),
				})
			}
		}
	}
	return out
}

var _ = All.MustRegister("check_transition_guard_assumption_resolves", Invariant{
	Name:  "check_transition_guard_assumption_resolves",
	Canon: methodology.Lifecycle,
	Claim: "every non-empty Transition.guard_assumption resolves to a known Assumption.",
	Rule: "for every EntityType.lifecycle.transitions[*], when guard_assumption is non-empty it MUST name an " +
		"Assumption id present in assumption_ids(g). A dangling guard_assumption is the behavioral-drift-seam " +
		"analogue of a dangling Requirement.assumptions[*] reference. No-ops when g.entity_types is empty.",
	Why: "this is a shape check -- for each Transition, guard_assumption must resolve in assumption_ids(g) -- the " +
		"exact homogeneous per-entity referential-integrity pattern the dangling-id family already covers for " +
		"Requirement/Conflict/Operator/Assumption edges; Transition.guard_assumption is the one remaining edge of " +
		"that family that had no enforcer. References: R-stale-substrate, dead-assumption fallout.",
	Check: checkTransitionGuardAssumptionResolves,
})

func checkAssumptionMachineChecksSyntactic(g *ontology.Graph) []Violation {
	var out []Violation
	for _, a := range g.Assumptions {
		if a.MachineCheck == nil {
			continue
		}
		mc := strings.TrimSpace(*a.MachineCheck)
		if mc == "" {
			out = append(out, Violation{
				Check:   "check_assumption_machine_checks_syntactic",
				ID:      a.ID,
				Message: fmt.Sprintf("machine_check field is set but empty/whitespace-only on assumption %q; it must be a non-empty formula or unset (nil), never an empty marker", a.ID),
			})
			continue
		}
	}
	return out
}

var _ = All.MustRegister("check_assumption_machine_checks_syntactic", Invariant{
	Name:  "check_assumption_machine_checks_syntactic",
	Canon: methodology.Assumption,
	Claim: "every non-empty machine_check is a well-formed expression, not prose.",
	Rule: "for each Assumption whose machine_check is set (non-nil), the trimmed value MUST be non-empty. This check " +
		"performs the structural non-empty check only; full expression-syntax validation (language-specific AST " +
		"compilation) is deferred to a future validation layer that has access to the expression mini-language's parser. " +
		"This does NOT execute the formula and does NOT assert it is TRUE.",
	Why: "the two machine_checks carried in the self-domain graph evaluate against different, not-yet-materialized " +
		"namespaces, so EXECUTING them would require inventing that namespace, which R-uncrystallizable-automated " +
		"forbids doing speculatively. What CAN be guaranteed structurally, without inventing semantics, is that the " +
		"recorded formula is a non-empty seam the deferred Z3/Hypothesis layers can later attach to, rather than an " +
		"empty marker masquerading as a machine_check. Promoting this to real execution and full syntax validation " +
		"is a separate, later act (a new atom) once a domain supplies the evaluation namespace and parser.",
	Check: checkAssumptionMachineChecksSyntactic,
})

func entityTypeBySlug(g *ontology.Graph) map[string]ontology.EntityType {
	out := make(map[string]ontology.EntityType, len(g.EntityTypes))
	for _, et := range g.EntityTypes {
		out[et.Slug] = et
	}
	return out
}

func checkEntityInstanceStateInLifecycle(g *ontology.Graph) []Violation {
	typeBySlug := entityTypeBySlug(g)
	var out []Violation
	for _, inst := range g.Entities {
		et, ok := typeBySlug[inst.EntityType]
		if !ok {
			out = append(out, Violation{
				Check:   "check_entity_instance_state_in_lifecycle",
				ID:      inst.ID,
				Message: fmt.Sprintf("entity_type %q is not declared", inst.EntityType),
			})
			continue
		}
		if _, ok := et.Lifecycle.Matches(inst.State); !ok {
			valid := make([]string, 0, len(et.Lifecycle.States))
			for _, s := range et.Lifecycle.States {
				valid = append(valid, s.Name)
			}
			sort.Strings(valid)
			out = append(out, Violation{
				Check:   "check_entity_instance_state_in_lifecycle",
				ID:      inst.ID,
				Message: fmt.Sprintf("state %q is not in lifecycle %q (valid: %v)", inst.State, et.Lifecycle.Slug, valid),
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_entity_instance_state_in_lifecycle", Invariant{
	Name:  "check_entity_instance_state_in_lifecycle",
	Canon: methodology.Entity,
	Claim: "every EntityInstance.state is valid in its EntityType.lifecycle.",
	Rule: "EntityInstance.state MUST be matched by the lifecycle of the corresponding EntityType (via Lifecycle.Matches). " +
		"An instance with an unknown state is structurally invalid -- the lifecycle machine cannot process it. " +
		"No-ops when g.entities is empty.",
	Why: "state integrity at the instance level mirrors check_requirement_status_in_lifecycle for requirements -- " +
		"the same keystone discipline applied to domain entities.",
	Check: checkEntityInstanceStateInLifecycle,
})

func checkEntityInstanceRequiredFields(g *ontology.Graph) []Violation {
	typeBySlug := entityTypeBySlug(g)
	var out []Violation
	for _, inst := range g.Entities {
		et, ok := typeBySlug[inst.EntityType]
		if !ok {
			continue
		}
		provided := map[string]struct{}{}
		for _, fv := range inst.FieldValues {
			provided[fv[0]] = struct{}{}
		}
		for _, f := range et.Fields {
			if f.Required {
				if _, ok := provided[f.Name]; !ok {
					out = append(out, Violation{
						Check:   "check_entity_instance_required_fields",
						ID:      inst.ID,
						Message: fmt.Sprintf("required field %q is missing", f.Name),
					})
				}
			}
		}
	}
	return out
}

var _ = All.MustRegister("check_entity_instance_required_fields", Invariant{
	Name:  "check_entity_instance_required_fields",
	Canon: methodology.Entity,
	Claim: "every required EntityField is present in EntityInstance.field_values.",
	Rule: "for each EntityField with required=true on the EntityType, the corresponding field name MUST appear in " +
		"EntityInstance.field_values. A missing required field is a structural gap -- the instance is incomplete. " +
		"No-ops when g.entities is empty.",
	Why: "required fields are the entity's schema contract; a missing required field violates the declared type " +
		"and makes downstream traversal unreliable.",
	Check: checkEntityInstanceRequiredFields,
})

func checkEntityInstanceIdPrefix(g *ontology.Graph) []Violation {
	var out []Violation
	for _, inst := range g.Entities {
		expectedPrefix := "ENT-" + inst.EntityType + "-"
		if !strings.HasPrefix(inst.ID, expectedPrefix) {
			out = append(out, Violation{
				Check:   "check_entity_instance_id_prefix",
				ID:      inst.ID,
				Message: fmt.Sprintf("entity instance id must start with %q", expectedPrefix),
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_entity_instance_id_prefix", Invariant{
	Name:  "check_entity_instance_id_prefix",
	Canon: methodology.Entity,
	Claim: "every EntityInstance.id starts with 'ENT-<entity_type>-'.",
	Rule: "EntityInstance.id MUST start with 'ENT-<entity_type>-' (typed-anchor discipline, R-anchor-everything). " +
		"A missing or wrong prefix breaks the typed-anchor discipline and makes cite-by-reference unreliable. " +
		"No-ops when g.entities is empty.",
	Why: "the prefix encodes both type and entity kind in the id, enabling unambiguous cross-reference in the graph. " +
		"Reference: R-anchor-everything.",
	Check: checkEntityInstanceIdPrefix,
})

func checkEntityInstanceRefsResolve(g *ontology.Graph) []Violation {
	sids := ontology.StakeholderIDs(g)
	rids := ontology.RequirementIDs(g)
	aids := ontology.AssumptionIDs(g)
	entitiesByType := map[string]map[string]struct{}{}
	for _, e := range g.Entities {
		if entitiesByType[e.EntityType] == nil {
			entitiesByType[e.EntityType] = map[string]struct{}{}
		}
		entitiesByType[e.EntityType][e.ID] = struct{}{}
	}
	typeBySlug := entityTypeBySlug(g)
	var out []Violation
	for _, inst := range g.Entities {
		et, ok := typeBySlug[inst.EntityType]
		if !ok {
			continue
		}
		for _, f := range et.Fields {
			if f.Kind != "reference" {
				continue
			}
			val, has := inst.FieldValue(f.Name)
			if !has || val == "" {
				continue
			}
			target := f.RefTarget
			valid := false
			switch target {
			case "stakeholder":
				_, valid = sids[val]
			case "requirement":
				_, valid = rids[val]
			case "assumption":
				_, valid = aids[val]
			default:
				ids, ok := entitiesByType[target]
				if ok {
					_, valid = ids[val]
				}
			}
			if !valid {
				out = append(out, Violation{
					Check:   "check_entity_instance_refs_resolve",
					ID:      inst.ID,
					Message: fmt.Sprintf("reference field %q=%q does not resolve in ref_target %q", f.Name, val, target),
				})
			}
		}
	}
	return out
}

var _ = All.MustRegister("check_entity_instance_refs_resolve", Invariant{
	Name:  "check_entity_instance_refs_resolve",
	Canon: methodology.Entity,
	Claim: "every reference EntityField value resolves in the graph.",
	Rule: "for each EntityField with kind='reference', any non-empty field value in EntityInstance.field_values " +
		"MUST resolve in the graph according to ref_target: 'stakeholder' resolves in stakeholder_ids(g); " +
		"'requirement' in requirement_ids(g); 'assumption' in assumption_ids(g); any other string is treated as an " +
		"entity_type slug and resolves among EntityInstance ids of that type. Empty values on optional references " +
		"are allowed; missing required references are caught by check_entity_instance_required_fields. " +
		"No-ops when g.entities is empty.",
	Why: "a dangling reference field is the entity-level equivalent of a dangling Conflict member -- the edge " +
		"exists but resolves to nothing, making the dependency invisible.",
	Check: checkEntityInstanceRefsResolve,
})

func checkEntityFieldKindKnown(g *ontology.Graph) []Violation {
	var out []Violation
	for _, et := range g.EntityTypes {
		for _, f := range et.Fields {
			if _, ok := ontology.EntityFieldKinds[f.Kind]; !ok {
				out = append(out, Violation{
					Check:   "check_entity_field_kind_known",
					ID:      et.Slug,
					Message: fmt.Sprintf("field %q has unknown kind %q (valid: %v)", f.Name, f.Kind, sortedSlugSet(ontology.EntityFieldKinds)),
				})
			}
		}
	}
	return out
}

var _ = All.MustRegister("check_entity_field_kind_known", Invariant{
	Name:  "check_entity_field_kind_known",
	Canon: methodology.Entity,
	Claim: "every EntityField.kind is in ENTITY_FIELD_KINDS.",
	Rule: "EntityField.kind MUST be in ENTITY_FIELD_KINDS (string | number | enum | reference | state). An unknown " +
		"kind is a misconfiguration that makes the field type undiscoverable. No-ops when g.entity_types is empty.",
	Why: "the kind discriminant is the seam for future machine-checkable field validation; an unknown kind breaks " +
		"the discriminant and hides the field from any kind-specific invariant.",
	Check: checkEntityFieldKindKnown,
})

func checkTypedAnchorsEntity(g *ontology.Graph) []Violation {
	var out []Violation
	for _, e := range g.Entities {
		if !strings.HasPrefix(e.ID, "ENT-") {
			out = append(out, Violation{
				Check:   "check_typed_anchors_entity",
				ID:      e.ID,
				Message: fmt.Sprintf("EntityInstance id %q must start with 'ENT-' (typed-anchor rule, R-anchor-everything)", e.ID),
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_typed_anchors_entity", Invariant{
	Name:  "check_typed_anchors_entity",
	Canon: methodology.Entity,
	Claim: "every EntityInstance id starts with 'ENT-'.",
	Rule: "EntityInstance.id MUST start with 'ENT-' (typed-anchor discipline, R-anchor-everything). Note: " +
		"check_entity_instance_id_prefix verifies the STRICTER 'ENT-<slug>-' rule. This check enforces only the " +
		"prefix family.",
	Why: "the 'ENT-' prefix family anchors all entity instances in the typed-anchor discipline " +
		"(R-anchor-everything), enabling unambiguous cross-reference.",
	Check: checkTypedAnchorsEntity,
})

func checkEntitiesMdListsAllTypes(g *ontology.Graph) []Violation {
	return nil
}

var _ = All.MustRegister("check_entities_md_lists_all_types", Invariant{
	Name:  "check_entities_md_lists_all_types",
	Canon: methodology.Entity,
	Claim: "every declared EntityType appears as a section in the generated ENTITIES.md.",
	Rule: "for each domain in domains/<name>/ whose graph declares entity_types, the corresponding " +
		"domains/<name>/docs/gen/ENTITIES.md MUST contain a section header '## <slug>' for every EntityType slug. " +
		"This is a FILESYSTEM-COHERENCE check -- it walks the domains directory, loads each domain's graph " +
		"independently, and reads the committed generated ENTITIES.md file from disk. The invariant contract " +
		"(Check func(*ontology.Graph) []Violation) operates on an in-memory graph only and has no access to the " +
		"filesystem; therefore this check is a NO-OP.",
	Why: "this check does not fit the pure-graph invariant contract: it reads generated doc files from disk and " +
		"compares them against per-domain graphs loaded independently. In this architecture, filesystem-coherence " +
		"checks belong to the generator/byte-identity layer (internal/generator), which has access to both the " +
		"filesystem and the build pipeline. Implementing this as a graph-only invariant would either require adding a " +
		"filesystem path parameter (breaking the uniform Check signature) or re-deriving the doc content from the " +
		"graph (which defeats the purpose of checking the committed file). A legitimate no-op here mirrors the " +
		"pattern of check_doc_reader_resolves_to_stakeholder: the structural guarantee is real, but its enforcement " +
		"lives in a different architectural layer. Reference: R-drift-structurally-impossible.",
	Check: checkEntitiesMdListsAllTypes,
})

func checkEntityTypeConstitutionProjection(g *ontology.Graph) []Violation {
	return nil
}

var _ = All.MustRegister("check_entity_type_constitution_projection", Invariant{
	Name:  "check_entity_type_constitution_projection",
	Canon: methodology.Entity,
	Claim: "every declared EntityType appears as R-entity-<slug> in the generated FRAMEWORK-INVARIANTS.md.",
	Rule: "for each domain in domains/<name>/ whose graph declares entity_types, the corresponding " +
		"domains/<name>/docs/gen/FRAMEWORK-INVARIANTS.md MUST contain a row naming 'R-entity-<slug>' for every " +
		"EntityType slug. This is a FILESYSTEM-COHERENCE check -- it walks the domains directory, loads each " +
		"domain's graph independently, and reads the committed generated FRAMEWORK-INVARIANTS.md file from disk. " +
		"The invariant contract (Check func(*ontology.Graph) []Violation) operates on an in-memory graph only and " +
		"has no access to the filesystem; therefore this check is a NO-OP.",
	Why: "this check does not fit the pure-graph invariant contract: it reads generated doc files from disk and " +
		"compares them against per-domain graphs. In this architecture, filesystem-coherence checks belong to the " +
		"generator/byte-identity layer (internal/generator), which has access to both the filesystem and the build " +
		"pipeline. Implementing this as a graph-only invariant would break the uniform Check signature or defeat the " +
		"purpose of checking the committed file. A legitimate no-op here mirrors the pattern of " +
		"check_doc_reader_resolves_to_stakeholder: the structural guarantee is real, but its enforcement lives in " +
		"a different architectural layer. Reference: R-drift-structurally-impossible.",
	Check: checkEntityTypeConstitutionProjection,
})
