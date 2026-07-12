package generator

import (
	"sort"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

var entityCoveringChecks = []string{
	"check_entity_type_lifecycle_wellformed",
	"check_entity_instance_state_in_lifecycle",
	"check_entity_instance_required_fields",
	"check_entity_instance_id_prefix",
	"check_entity_instance_refs_resolve",
	"check_entity_field_kind_known",
	"check_typed_anchors_entity",
}

func EntitiesMDHasContent(g *ontology.Graph) bool {
	return len(g.EntityTypes) > 0
}

func renderEntityTypeMermaid(et ontology.EntityType) []string {
	lc := et.Lifecycle
	lines := []string{"```mermaid", "stateDiagram-v2"}
	for _, s := range lc.States {
		if s.IsInitial() {
			lines = append(lines, "    [*] --> "+s.Name)
			break
		}
	}
	for _, t := range lc.Transitions {
		lines = append(lines, "    "+t.Src+" --> "+t.Dst+" : "+t.Event)
	}
	for _, s := range lc.States {
		if !s.IsInitial() {
			lines = append(lines, "    "+s.Name+": "+s.Name+" ("+s.Kind+")")
		} else {
			lines = append(lines, "    "+s.Name+": "+s.Name+" ("+s.Kind+")")
		}
	}
	lines = append(lines, "```")
	return lines
}

func renderEntityLifecycleSummary(et ontology.EntityType) []string {
	lc := et.Lifecycle
	stateParts := make([]string, 0, len(lc.States))
	for _, s := range lc.States {
		stateParts = append(stateParts, "`"+s.Name+"` ("+s.Kind+")")
	}
	transParts := make([]string, 0, len(lc.Transitions))
	for _, t := range lc.Transitions {
		transParts = append(transParts, "`"+t.Event+"`")
	}
	cyclicStr := "false"
	if lc.Cyclic {
		cyclicStr = "true"
	}
	transStr := "_(none)_"
	if len(transParts) > 0 {
		transStr = strings.Join(transParts, ", ")
	}
	return []string{
		"- States: " + strings.Join(stateParts, ", "),
		"- Transitions: " + transStr,
		"- Cyclic: " + cyclicStr,
	}
}

func renderEntityFieldsTable(et ontology.EntityType) []string {
	if len(et.Fields) == 0 {
		return []string{"_(no fields declared)_"}
	}
	lines := []string{
		"| name | kind | required | ref_target |",
		"|------|------|----------|------------|",
	}
	for _, f := range et.Fields {
		ref := f.RefTarget
		reqStr := "false"
		if f.Required {
			reqStr = "true"
		}
		lines = append(lines, "| "+f.Name+" | "+f.Kind+" | "+reqStr+" | "+ref+" |")
	}
	return lines
}

func renderEntityInstancesTable(g *ontology.Graph, slug string) []string {
	var instances []ontology.EntityInstance
	for _, e := range g.Entities {
		if e.EntityType == slug {
			instances = append(instances, e)
		}
	}
	if len(instances) == 0 {
		return []string{"_(no instances declared)_"}
	}
	etMap := map[string]ontology.EntityType{}
	for _, et := range g.EntityTypes {
		etMap[et.Slug] = et
	}
	et, etOK := etMap[slug]
	var fieldNames []string
	if etOK {
		for _, f := range et.Fields {
			fieldNames = append(fieldNames, f.Name)
		}
	}

	headerParts := append([]string{"id", "state"}, fieldNames...)
	sepParts := make([]string, len(headerParts))
	for i, h := range headerParts {
		dashes := len(h)
		if dashes < 3 {
			dashes = 3
		}
		sepParts[i] = strings.Repeat("-", dashes)
	}
	lines := []string{
		"| " + strings.Join(headerParts, " | ") + " |",
		"| " + strings.Join(sepParts, " | ") + " |",
	}
	sortedInstances := make([]ontology.EntityInstance, len(instances))
	copy(sortedInstances, instances)
	sort.Slice(sortedInstances, func(i, j int) bool { return sortedInstances[i].ID < sortedInstances[j].ID })
	for _, inst := range sortedInstances {
		row := []string{inst.ID, inst.State}
		for _, fn := range fieldNames {
			val, _ := inst.FieldValue(fn)
			row = append(row, val)
		}
		lines = append(lines, "| "+strings.Join(row, " | ")+" |")
	}
	return lines
}

func entityStateConflictSuspects(g *ontology.Graph) []latentSuspect {
	typeBySlug := map[string]ontology.EntityType{}
	for _, et := range g.EntityTypes {
		typeBySlug[et.Slug] = et
	}
	var suspects []latentSuspect

	processDestinations := func(p ontology.Process, slug string) map[string]struct{} {
		et, ok := typeBySlug[slug]
		if !ok {
			return map[string]struct{}{}
		}
		transitionsByEvent := map[string]ontology.Transition{}
		for _, t := range et.Lifecycle.Transitions {
			transitionsByEvent[t.Event] = t
		}
		terminalOrQuiescent := map[string]struct{}{}
		for _, s := range et.Lifecycle.States {
			if s.IsTerminal() {
				terminalOrQuiescent[s.Name] = struct{}{}
			}
		}
		dests := map[string]struct{}{}
		for _, step := range p.Steps {
			if step.Invokes == "" || !strings.Contains(step.Invokes, ".") {
				continue
			}
			parts := strings.SplitN(step.Invokes, ".", 2)
			s := parts[0]
			event := parts[1]
			if s != slug {
				continue
			}
			t, ok := transitionsByEvent[event]
			if ok {
				if _, isTQ := terminalOrQuiescent[t.Dst]; isTQ {
					dests[t.Dst] = struct{}{}
				}
			}
		}
		return dests
	}

	for _, et := range g.EntityTypes {
		slug := et.Slug
		var ps []ontology.Process
		for _, p := range g.Processes {
			for _, de := range p.DrivesEntities {
				if de == slug {
					ps = append(ps, p)
					break
				}
			}
		}
		for i := 0; i < len(ps); i++ {
			for j := i + 1; j < len(ps); j++ {
				a, b := ps[i], ps[j]
				da := processDestinations(a, slug)
				db := processDestinations(b, slug)
				if len(da) == 0 || len(db) == 0 {
					continue
				}
				disjoint := true
				for k := range da {
					if _, ok := db[k]; ok {
						disjoint = false
						break
					}
				}
				if !disjoint {
					continue
				}
				left, right := a.ID, b.ID
				if right < left {
					left, right = right, left
				}
				daSorted := mapKeysSorted(da)
				dbSorted := mapKeysSorted(db)
				hint := "both drive entity '" + slug + "' but to disjoint resting states: " + strings.Join(daSorted, ", ") + " vs " + strings.Join(dbSorted, ", ") + " — likely conflict on axis behavioral-" + slug + "-state"
				suspects = append(suspects, latentSuspect{Left: left, Right: right, Hint: hint})
			}
		}
	}
	sort.Slice(suspects, func(i, j int) bool {
		if suspects[i].Left != suspects[j].Left {
			return suspects[i].Left < suspects[j].Left
		}
		return suspects[i].Right < suspects[j].Right
	})
	return suspects
}

func mapKeysSorted(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func BuildEntities(g *ontology.Graph, domainName string) string {
	sourceHint := "from the active domain's `graph.json`"
	if domainName != "" {
		sourceHint = "from `domains/" + domainName + "/graph.json`"
	}

	lines := []string{Banner, ReaderHeaderLine("ENTITIES", g), ""}
	lines = append(lines, "# Entities")
	lines = append(lines, "")
	lines = append(lines, "> Generated by `hotam gen-spec` "+sourceHint+". Do not hand-edit.")
	lines = append(lines, "")

	if len(g.EntityTypes) == 0 {
		lines = append(lines, "_(no entity types declared in this domain — the §Entity aspect is opt-in.)_")
		lines = append(lines, "")
		return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n") + "\n"
	}

	sortedTypes := make([]ontology.EntityType, len(g.EntityTypes))
	copy(sortedTypes, g.EntityTypes)
	sort.Slice(sortedTypes, func(i, j int) bool { return sortedTypes[i].Slug < sortedTypes[j].Slug })

	for _, et := range sortedTypes {
		lines = append(lines, "## "+et.Slug)
		lines = append(lines, "")
		if et.Description != "" {
			lines = append(lines, et.Description)
			lines = append(lines, "")
		}

		lines = append(lines, "### Lifecycle")
		lines = append(lines, "")
		lines = append(lines, renderEntityTypeMermaid(et)...)
		lines = append(lines, "")
		lines = append(lines, renderEntityLifecycleSummary(et)...)
		lines = append(lines, "")

		lines = append(lines, "### Fields")
		lines = append(lines, "")
		lines = append(lines, renderEntityFieldsTable(et)...)
		lines = append(lines, "")

		lines = append(lines, "### Covered by")
		lines = append(lines, "")
		for _, checkName := range entityCoveringChecks {
			lines = append(lines, "- `"+checkName+"`")
		}
		lines = append(lines, "")

		lines = append(lines, "### Instances")
		lines = append(lines, "")
		lines = append(lines, renderEntityInstancesTable(g, et.Slug)...)
		lines = append(lines, "")
	}

	lines = append(lines, "## Entity-state tensions")
	lines = append(lines, "")
	suspects := entityStateConflictSuspects(g)
	if len(suspects) == 0 {
		lines = append(lines, "_(no entity-state tensions surfaced — clean)_")
		lines = append(lines, "")
	} else {
		for _, s := range suspects {
			lines = append(lines, "- `"+s.Left+"` × `"+s.Right+"` — "+s.Hint)
		}
		lines = append(lines, "")
	}

	return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n") + "\n"
}

func renderEntityDerivedConstitutionSection(g *ontology.Graph) string {
	if len(g.EntityTypes) == 0 {
		return ""
	}
	enforcerStrs := make([]string, len(entityCoveringChecks))
	for i, c := range entityCoveringChecks {
		enforcerStrs[i] = "`" + c + "`"
	}
	enforcerStr := strings.Join(enforcerStrs, ", ")
	lines := []string{"**Entity-derived requirements**", ""}

	sortedTypes := make([]ontology.EntityType, len(g.EntityTypes))
	copy(sortedTypes, g.EntityTypes)
	sort.Slice(sortedTypes, func(i, j int) bool { return sortedTypes[i].Slug < sortedTypes[j].Slug })

	for _, et := range sortedTypes {
		lines = append(lines, "- **R-entity-"+et.Slug+"** — *"+et.Description+"* [STRUCTURAL·entity · §Entity] [enforced_by: "+enforcerStr+"]")
	}
	lines = append(lines, "")
	return strings.Join(lines, "\n")
}
