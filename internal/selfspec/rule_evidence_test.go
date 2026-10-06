package selfspec

import (
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/registry"
)

func TestRuleHoldsKeepsSupportingMethodAsSeparateEvidenceRelation(t *testing.T) {
	model := `package model

type Box struct { value int }
// >>>>> lang=en
// positive values satisfy the validation rule
// >>>>> lang=ru
// положительные значения удовлетворяют правилу проверки
// >>>>> lang=zh
// 正值满足验证规则
func (b Box) Valid() bool { return b.value > 0 }

// >>>>> lang=en
// value reports the stored sample
// >>>>> lang=ru
// значение сообщает сохранённый образец
// >>>>> lang=zh
// 值报告已存储的样本
func (b Box) Value() int { return b.value }
`
	root := atomDiscoveryFixture(t, multilingualRuleManifest, model, `package model

import (
	"testing"
	"example.test/domain/hotamspec"
)

func TestRuleHold(t *testing.T) {
	box := Box{value: 1}
	supportContext := hotamspec.CaseContext{ID: "support-value", AtomIDs: []string{"R-box-value"}}
	support := hotamspec.Fact(t, box.Value, 1, hotamspec.WithInput(1), hotamspec.WithCase(supportContext))
	holdContext := hotamspec.CaseContext{ID: "validity-rule", AtomIDs: []string{"R-box-valid"}, Operation: "validate", Producer: "adapter"}
	hotamspec.Holds(t, box.Valid, support, hotamspec.WithCase(holdContext))
}
`)
	discovered, err := DiscoverAtoms(root, root, registry.New[ontology.Requirement]())
	if err != nil {
		t.Fatal(err)
	}
	requirement, ok := discovered.Get("R-box-valid")
	if !ok {
		t.Fatal("Holds primary atom was not discovered")
	}
	if len(requirement.ImplementedBy) != 2 {
		t.Fatalf("Holds implemented_by links lost the primary or support method: %v", requirement.ImplementedBy)
	}
	wantRelation := ontology.Relation{Kind: "depends_on", Target: "R-box-value"}
	found := false
	for _, relation := range requirement.Relations {
		if relation == wantRelation {
			found = true
		}
	}
	if !found {
		t.Fatalf("supporting method was not retained as a separate relation: %v", requirement.Relations)
	}
	if strings.Contains(requirement.ClaimTexts["en"], "stored sample") {
		t.Fatalf("rule norm absorbed a supporting sample phrase: %q", requirement.ClaimTexts["en"])
	}
}
