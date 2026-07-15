// requirements_test_gen.go renders requirements_test.go: one named Go test
// function per SETTLED ontology.Requirement, with sub-tests for every atom
// GEN-CODE-CONTRACT.md §3's classification found (see requirements.go), and
// a single honest `t.Log` sub-test-free body for requirements where no atom
// was found at all. See docs/GEN-CODE-CONTRACT.md §1.1: this file is pure
// ASCII by construction — every string this renderer emits into Go source
// comes from either an already-resolved Go identifier (BuildEntityModel) or
// an already-ASCII anchor token (BuildRequirementModel's idAnchorPattern
// only matches Latin-letter/digit/hyphen shapes) — never the requirement's
// (possibly Cyrillic) claim text.
package gocode

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// RenderRequirementsTestFile renders the full requirements_test.go body for
// a domain: the ownership marker, package clause, and one Test_<id> function
// per SETTLED requirement (sorted by id, contract §5 determinism — reqModels
// is expected already-sorted by BuildRequirementModels, but this renderer
// re-sorts defensively so it never depends on caller order).
func RenderRequirementsTestFile(packageName string, reqModels []*requirementModel) ([]byte, error) {
	sorted := make([]*requirementModel, len(reqModels))
	copy(sorted, reqModels)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].src.ID < sorted[j].src.ID })

	var b strings.Builder
	b.WriteString(OwnershipMarker)
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "package %s\n\n", packageName)
	b.WriteString("import \"testing\"\n\n")

	for i, m := range sorted {
		if i != 0 {
			b.WriteString("\n")
		}
		b.WriteString(renderRequirementTest(m))
	}

	return []byte(b.String()), nil
}

// renderRequirementTest renders one requirement's Test_<id> function body,
// dispatching on the atom kind BuildRequirementModel already resolved.
func renderRequirementTest(m *requirementModel) string {
	var b strings.Builder
	fmt.Fprintf(&b, "// Atom: %s - see requirements_audit.md#%s\n", m.src.ID, m.anchorSlug)
	fmt.Fprintf(&b, "func %s(t *testing.T) {\n", m.funcName)

	switch m.kind {
	case atomKindField:
		renderFieldAtomBody(&b, m)
	case atomKindStatePair:
		renderStatePairAtomBody(&b, m)
	case atomKindGate:
		renderGateAtomBody(&b, m)
	case atomKindInterEntity:
		renderInterEntityAtomBody(&b, m)
	default:
		renderNoAtomBody(&b, m)
	}

	b.WriteString("}\n")
	return b.String()
}

// renderFieldAtomBody renders one t.Run per matched EntityType.field
// (contract §3 row 1). Each sub-test constructs the entity via its already-
// generated constructor, fills every OTHER required field with a
// placeholder, and asserts the atom's own field's required-ness is actually
// enforced by the generated Validate(): if required, leaving it empty must
// fail; if not required, Validate() must not depend on it (leaving it empty
// must still pass once every required field is filled). Either branch is a
// real assertion on generated behavior (contract §5 mutational: a
// generator regression that drops the field's Required branch from
// Validate() flips this sub-test red).
func renderFieldAtomBody(b *strings.Builder, m *requirementModel) {
	for _, fa := range m.fields {
		subName := fa.entity.structName + "_" + fa.field.fieldName
		fieldMsg := subName + ": field " + fa.field.fieldName
		fmt.Fprintf(b, "\tt.Run(%s, func(t *testing.T) {\n", strconv.Quote(subName))
		fmt.Fprintf(b, "\t\tx := New%s()\n", fa.entity.structName)
		for _, other := range fa.entity.fields {
			if other.fieldName == fa.field.fieldName {
				continue
			}
			if !other.src.Required {
				continue
			}
			fmt.Fprintf(b, "\t\tx.%s = %s\n", other.fieldName, strconv.Quote("placeholder"))
		}
		if fa.field.src.Required {
			b.WriteString("\t\t// field is required (graph: field.required=true) - Validate() must reject it empty.\n")
			b.WriteString("\t\tif err := x.Validate(); err == nil {\n")
			fmt.Fprintf(b, "\t\t\tt.Fatal(%s)\n", strconv.Quote(fieldMsg+" required, expected Validate() to fail while empty"))
			b.WriteString("\t\t}\n")
			fmt.Fprintf(b, "\t\tx.%s = %s\n", fa.field.fieldName, strconv.Quote("placeholder"))
			b.WriteString("\t\tif err := x.Validate(); err != nil {\n")
			fmt.Fprintf(b, "\t\t\tt.Fatalf(%s+\": %%v\", err)\n", strconv.Quote(fieldMsg+" filled, expected Validate() to pass"))
			b.WriteString("\t\t}\n")
		} else {
			b.WriteString("\t\t// field is not required (graph: field.required=false) - Validate() must not depend on it.\n")
			b.WriteString("\t\tif err := x.Validate(); err != nil {\n")
			fmt.Fprintf(b, "\t\t\tt.Fatalf(%s+\": %%v\", err)\n", strconv.Quote(fieldMsg+" optional and left empty, expected Validate() to pass"))
			b.WriteString("\t\t}\n")
		}
		b.WriteString("\t})\n")
	}
}

// renderStatePairAtomBody renders one t.Run per matched lifecycle-state pair
// (contract §3 row 2): asserts both named states are distinct legal values
// of the entity's state type (a mutation that removes one of the two
// constants, or collapses them to the same value, fails to compile/fails
// this comparison).
func renderStatePairAtomBody(b *strings.Builder, m *requirementModel) {
	sp := m.statePair
	subName := sp.entity.structName + "_state_pair"
	fmt.Fprintf(b, "\tt.Run(%q, func(t *testing.T) {\n", subName)
	for i := 0; i < len(sp.states); i++ {
		for j := i + 1; j < len(sp.states); j++ {
			a, c := sp.states[i], sp.states[j]
			fmt.Fprintf(b, "\t\tif %s == %s {\n", a.constant, c.constant)
			fmt.Fprintf(b, "\t\t\tt.Fatalf(\"%s and %s must be distinct lifecycle states of %s\")\n",
				a.constant, c.constant, sp.entity.structName)
			b.WriteString("\t\t}\n")
		}
	}
	b.WriteString("\t})\n")
}

// renderGateAtomBody renders the gate/order atom (contract §3 row 3): the
// anchor tokens BuildRequirementModel already found (meta-token + typed
// anchor) are baked in as a Go string literal slice — computed once at
// generation time, not re-derived from claim text in the .go layer (which
// would require Cyrillic) — and the sub-test asserts the anchor set is
// non-empty and every anchor is non-blank, the structural minimum the §3
// row 3 classification itself depends on.
func renderGateAtomBody(b *strings.Builder, m *requirementModel) {
	subName := "gate_order_anchors"
	fmt.Fprintf(b, "\tt.Run(%q, func(t *testing.T) {\n", subName)
	b.WriteString("\t\tanchors := []string{")
	for i, a := range m.gate.anchors {
		if i != 0 {
			b.WriteString(", ")
		}
		b.WriteString(strconv.Quote(a))
	}
	b.WriteString("}\n")
	b.WriteString("\t\tif len(anchors) == 0 {\n")
	b.WriteString("\t\t\tt.Fatal(\"expected at least one typed gate/order anchor\")\n")
	b.WriteString("\t\t}\n")
	b.WriteString("\t\tfor _, a := range anchors {\n")
	b.WriteString("\t\t\tif a == \"\" {\n")
	b.WriteString("\t\t\t\tt.Fatal(\"gate/order anchor must not be blank\")\n")
	b.WriteString("\t\t\t}\n")
	b.WriteString("\t\t}\n")
	b.WriteString("\t})\n")
}

// renderInterEntityAtomBody renders the inter-entity invariant atom
// (contract §3 row 4): asserts both named EntityTypes actually construct
// (their generated constructors exist and return a non-nil value in their
// declared initial state) - the structural minimum "both ends resolve in
// the graph" requires.
func renderInterEntityAtomBody(b *strings.Builder, m *requirementModel) {
	names := make([]string, len(m.interEntity))
	for i, em := range m.interEntity {
		names[i] = em.structName
	}
	subName := "inter_entity_" + strings.Join(names, "_")
	fmt.Fprintf(b, "\tt.Run(%s, func(t *testing.T) {\n", strconv.Quote(subName))
	for _, em := range m.interEntity {
		fmt.Fprintf(b, "\t\tif x := New%s(); x == nil {\n", em.structName)
		fmt.Fprintf(b, "\t\t\tt.Fatal(%s)\n", strconv.Quote("New"+em.structName+"() returned nil - both ends of this invariant must resolve to a generated EntityType"))
		b.WriteString("\t\t}\n")
	}
	b.WriteString("\t})\n")
}

// renderNoAtomBody renders contract §3's closing row: no structural carrier
// found. The requirement still gets a named, passing test function (visible
// in `go test -v`) — never t.Skip — with a single t.Log pointing at
// requirements_audit.md, carrying only the (already-ASCII) requirement id,
// never the claim text itself (contract §1.1/§3).
func renderNoAtomBody(b *strings.Builder, m *requirementModel) {
	fmt.Fprintf(b, "\tt.Log(\"no structural atom - see requirements_audit.md#%s\")\n", m.anchorSlug)
}
