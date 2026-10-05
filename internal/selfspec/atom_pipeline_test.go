package selfspec

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/recorder/vendor"
	"github.com/PHPCraftdream/HotamSpec/internal/registry"
)

const multilingualRuleManifest = `{"self_hosting":false,"self_executing_atoms":true,"parent":null,"languages":["en","ru","zh"],"default_language":"ru","conformance":{"rule_cases":true}}`

const plainAtomManifest = `{"self_hosting":false,"self_executing_atoms":true,"parent":null}`

const multilingualRuleModel = `package model

type Box struct { value int }

// >>>>> lang=en
// returned values follow the selected rule
// >>>>> lang=ru
// возвращаемые значения следуют выбранному правилу
// >>>>> lang=zh
// 返回值遵循所选规则
func (b Box) Value() int { return b.value }

// >>>>> lang=en
// alias values follow the selected rule
// >>>>> lang=ru
// значения-псевдонимы следуют выбранному правилу
// >>>>> lang=zh
// 别名值遵循所选规则
func (b Box) Alias() int { return b.value }
`

func atomDiscoveryFixture(t *testing.T, manifest, model, test string) string {
	t.Helper()
	return atomSourceFixture(t, map[string]string{
		"manifest.json":               manifest,
		"spec/hotamspec/hotamspec.go": vendor.Source(),
		"spec/model/value.go":         model,
		"spec/model/value_test.go":    test,
	})
}

func TestDiscoverAtomsRuleCasesPreserveClaimsMetadataAndSourceOrder(t *testing.T) {
	root := atomDiscoveryFixture(t, multilingualRuleManifest, multilingualRuleModel, `package model

import (
	"testing"
	"example.test/domain/hotamspec"
)

func TestRuleCases(t *testing.T) {
	for _, sample := range []struct { name string; value int }{{"one", 11}, {"two", 22}} {
		t.Run(sample.name, func(t *testing.T) {
			box := Box{value: sample.value}
			ctx := hotamspec.CaseContext{
				ID: "case-" + sample.name,
				AtomIDs: []string{"R-box-value", "R-box-alias"},
				Profile: "profile-" + sample.name,
				Target: "adapter",
				Operation: "decode-" + sample.name,
				Producer: "adapter-" + sample.name,
			}
			hotamspec.Fact(t, box.Value, sample.value, hotamspec.WithInput(sample.value), hotamspec.WithCase(ctx))
			hotamspec.Fact(t, box.Alias, sample.value, hotamspec.WithInput(sample.value), hotamspec.WithCase(ctx))
		})
	}
}
`)

	discovered, err := DiscoverAtoms(root, registry.New[ontology.Requirement]())
	if err != nil {
		t.Fatal(err)
	}
	requirements := discovered.All()
	if len(requirements) != 2 {
		t.Fatalf("discovered %d atoms, want the two source methods", len(requirements))
	}
	if requirements[0].ID != "R-box-value" || requirements[1].ID != "R-box-alias" {
		t.Fatalf("source order = [%s, %s], want Value before Alias", requirements[0].ID, requirements[1].ID)
	}
	for _, requirement := range requirements {
		if requirement.AtomKind != "rule" {
			t.Errorf("%s AtomKind = %q, want rule", requirement.ID, requirement.AtomKind)
		}
		if requirement.Claim != requirement.ClaimTexts["ru"] {
			t.Errorf("%s primary Claim %q differs from default-language text %q", requirement.ID, requirement.Claim, requirement.ClaimTexts["ru"])
		}
		if len(requirement.ClaimTexts) != 3 {
			t.Errorf("%s ClaimTexts has %d locales, want en/ru/zh", requirement.ID, len(requirement.ClaimTexts))
		}
		if strings.Contains(requirement.Claim, "11") || strings.Contains(requirement.Claim, "22") {
			t.Errorf("rule claim contains witness data: %q", requirement.Claim)
		}
		if len(requirement.Cases) != 2 {
			t.Fatalf("%s cases = %d, want both executed samples", requirement.ID, len(requirement.Cases))
		}
		for _, caseDef := range requirement.Cases {
			if caseDef.Input == nil || caseDef.Expected == nil {
				t.Errorf("%s case %s lost independently recorded input/expected values: %+v", requirement.ID, caseDef.ID, caseDef)
			}
			if !reflect.DeepEqual(caseDef.AtomIDs, []string{"R-box-value", "R-box-alias"}) {
				t.Errorf("case %s shared atom links = %v", caseDef.ID, caseDef.AtomIDs)
			}
			if caseDef.Target != "adapter" || caseDef.Operation != "decode-"+strings.TrimPrefix(caseDef.ID, "case-") || caseDef.Producer != "adapter-"+strings.TrimPrefix(caseDef.ID, "case-") {
				t.Errorf("case %s metadata not preserved: %+v", caseDef.ID, caseDef)
			}
			if !strings.HasPrefix(caseDef.Test, "spec/model/value_test.go:TestRuleCases/") {
				t.Errorf("case %s test address = %q, want file-qualified executed subtest", caseDef.ID, caseDef.Test)
			}
		}
	}
	base := requirements[0]
	overrides := registry.New[ontology.Requirement]()
	overrides.MustRegister(base.ID, ontology.Requirement{
		ID: base.ID, Owner: "reviewer", AtomKind: "rule",
		ClaimTexts: base.ClaimTexts, Strength: "MUST",
	})
	withOverrides, err := DiscoverAtoms(root, overrides)
	if err != nil {
		t.Fatal(err)
	}
	overridden, ok := withOverrides.Get(base.ID)
	if !ok {
		t.Fatalf("override lost discovered requirement %s", base.ID)
	}
	if overridden.Owner != "reviewer" || overridden.Strength != "MUST" ||
		overridden.AtomKind != "rule" || !reflect.DeepEqual(overridden.ClaimTexts, base.ClaimTexts) ||
		len(overridden.Cases) != 2 {
		t.Fatalf("override erased derived atom/case metadata: %+v", overridden)
	}
	conflicting := registry.New[ontology.Requirement]()
	conflicting.MustRegister(base.ID, ontology.Requirement{
		ID: base.ID,
		Cases: []ontology.CaseDefinition{{
			ID:       base.Cases[0].ID,
			Expected: &ontology.ObservedValue{Kind: "conflicting-oracle"},
		}},
	})
	if _, err := DiscoverAtoms(root, conflicting); err == nil || !strings.Contains(err.Error(), "conflicting expected") {
		t.Fatalf("recorded want must not overwrite an inconsistent authored case oracle, got %v", err)
	}
}

func TestDiscoverAtomsRuleHoldsRetainsFalseAndTrueOracles(t *testing.T) {
	model := `package model

type Box struct { value int }
// >>>>> lang=en
// positive values satisfy the rule
// >>>>> lang=ru
// положительные значения удовлетворяют правилу
// >>>>> lang=zh
// 正值满足规则
func (b Box) Valid() bool { return b.value > 0 }
`
	root := atomDiscoveryFixture(t, multilingualRuleManifest, model, `package model

import (
	"testing"
	"example.test/domain/hotamspec"
)

func TestValidityCases(t *testing.T) {
	for _, sample := range []struct { name string; value int; want bool }{
		{"below", 0, false},
		{"above", 2, true},
	} {
		t.Run(sample.name, func(t *testing.T) {
			box := Box{value: sample.value}
			ctx := hotamspec.CaseContext{
				ID: "validity-" + sample.name,
				AtomIDs: []string{"R-box-valid"},
				Profile: "signed",
				Target: "adapter",
				Operation: "validate",
				Producer: "adapter-v1",
			}
			hotamspec.Holds(t, box.Valid,
				hotamspec.WithInput(sample.value),
				hotamspec.WithCase(ctx),
				hotamspec.Expect(sample.want),
			)
		})
	}
}
`)
	discovered, err := DiscoverAtoms(root, registry.New[ontology.Requirement]())
	if err != nil {
		t.Fatal(err)
	}
	requirement, ok := discovered.Get("R-box-valid")
	if !ok || requirement.AtomKind != "rule" || len(requirement.Cases) != 2 {
		t.Fatalf("Holds rule cases not discovered: %+v", requirement)
	}
	wantByID := map[string]bool{"validity-below": false, "validity-above": true}
	for _, caseDef := range requirement.Cases {
		want, exists := wantByID[caseDef.ID]
		if !exists || caseDef.Expected == nil || caseDef.Expected.Kind != "bool" || caseDef.Expected.Bool == nil || *caseDef.Expected.Bool != want {
			t.Errorf("case %s lost exact bool want (including false): %+v", caseDef.ID, caseDef.Expected)
		}
		if caseDef.Input == nil {
			t.Errorf("case %s lost WithInput value", caseDef.ID)
		}
	}
}

func TestDiscoverAtomsKeepsLegacyFactConflict(t *testing.T) {
	root := atomDiscoveryFixture(t, plainAtomManifest, `package model

type Box struct { value int }
// the value is exact
func (b Box) Value() int { return b.value }
`, `package model

import (
	"testing"
	"example.test/domain/hotamspec"
)

func TestTwoValues(t *testing.T) {
	first := Box{value: 1}
	second := Box{value: 2}
	hotamspec.Fact(t, first.Value, 1)
	hotamspec.Fact(t, second.Value, 2)
}
`)
	_, err := DiscoverAtoms(root, registry.New[ontology.Requirement]())
	if err == nil || !strings.Contains(err.Error(), "legacy executed values") {
		t.Fatalf("different legacy Fact values should conflict; got %v", err)
	}
}

func TestDiscoverAtomsRejectsRuleWithoutOwnTrigger(t *testing.T) {
	root := atomDiscoveryFixture(t, plainAtomManifest, `package model

import "os"

type Box struct { value int }
// values follow the rule
func (b Box) Value() int {
	_ = os.WriteFile("ran", []byte("method executed"), 0o644)
	return b.value
}
`, `package model

import (
	"testing"
	"example.test/domain/hotamspec"
)

func TestRule(t *testing.T) {
	box := Box{value: 1}
	ctx := hotamspec.CaseContext{ID: "case-1"}
	hotamspec.Fact(t, box.Value, 1, hotamspec.WithInput(1), hotamspec.WithCase(ctx))
}
`)
	_, err := DiscoverAtoms(root, registry.New[ontology.Requirement]())
	if err == nil || !strings.Contains(err.Error(), "conformance.rule_cases") {
		t.Fatalf("rule artifact without its own trigger should be rejected, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, "spec", "model", "ran")); !os.IsNotExist(statErr) {
		t.Fatalf("rule method executed without conformance.rule_cases; stat error=%v", statErr)
	}
}

func TestDiscoverAtomsRejectsCaseIDDescriptorCollision(t *testing.T) {
	root := atomDiscoveryFixture(t, multilingualRuleManifest, multilingualRuleModel, `package model

import (
	"testing"
	"example.test/domain/hotamspec"
)

func TestCollision(t *testing.T) {
	for _, sample := range []struct { name string; value int }{{"one", 1}, {"two", 2}} {
		t.Run(sample.name, func(t *testing.T) {
			box := Box{value: sample.value}
			ctx := hotamspec.CaseContext{ID: "same-case", AtomIDs: []string{"R-box-value"}, Operation: "decode"}
			hotamspec.Fact(t, box.Value, sample.value, hotamspec.WithInput(sample.value), hotamspec.WithCase(ctx))
		})
	}
}
`)
	_, err := DiscoverAtoms(root, registry.New[ontology.Requirement]())
	if err == nil || !strings.Contains(err.Error(), "same-case") || !strings.Contains(err.Error(), "conflicting") {
		t.Fatalf("same case ID with different executed descriptors should reject, got %v", err)
	}
}

func TestDiscoverAtomsReportsOrphanedRecorderCall(t *testing.T) {
	root := atomDiscoveryFixture(t, plainAtomManifest, `package model

type Box struct{}
// the value is exact
func (b Box) Value() int { return 1 }
`, `package model

import (
	"testing"
	"example.test/domain/hotamspec"
)

func TestConditionalAtom(t *testing.T) {
	if false {
		hotamspec.Fact(t, (Box{}).Value, 1)
	}
}
`)
	_, err := DiscoverAtoms(root, registry.New[ontology.Requirement]())
	if err == nil || !strings.Contains(err.Error(), "produced no recording") {
		t.Fatalf("orphaned atom call should remain visible, got %v", err)
	}
}

func TestAtomSourceMultilingualBlocksAndRuleClaims(t *testing.T) {
	root := atomSourceFixture(t, map[string]string{
		"spec/model/value.go": multilingualRuleModel,
	})
	graph := &ontology.Graph{
		DomainDir: root, Languages: []string{"en", "ru", "zh"}, DefaultLanguage: "ru",
		Conformance: &ontology.ConformanceConfig{RuleCases: true},
	}
	index, err := gate.NewAtomSourceIndexForGraph(graph)
	if err != nil {
		t.Fatal(err)
	}
	source, err := index.Resolve("example.test/domain/model.Box.Value")
	if err != nil {
		t.Fatal(err)
	}
	if source.PhrasePositions["en"].Line != 6 || source.PhrasePositions["ru"].Line != 8 || source.PhrasePositions["zh"].Line != 10 {
		t.Fatalf("localized source positions = %v, want phrase lines en=6, ru=8, zh=10", source.PhrasePositions)
	}
	artifact, err := gate.DecodeAtomArtifact([]byte(`{"req_id":"R-box-value","test":"TestRuleCases/one","mode":"rule","title":"","verdict":"pass","steps":[{"subject":"example.test/domain/model.Box.Value","value":"11"}],"case":{"id":"case-one"}}`))
	if err != nil {
		t.Fatal(err)
	}
	claims, err := index.DeriveClaims(artifact)
	if err != nil {
		t.Fatal(err)
	}
	if claims["en"] != "Returned values follow the selected rule." {
		t.Fatalf("English rule claim = %q", claims["en"])
	}
	if claims["ru"] != "Возвращаемые значения следуют выбранному правилу." {
		t.Fatalf("Russian rule claim = %q", claims["ru"])
	}
	if !strings.HasPrefix(claims["zh"], "返回值遵循所选规则") {
		t.Fatalf("Chinese rule claim = %q", claims["zh"])
	}
	primary, err := index.DeriveClaim(artifact)
	if err != nil || primary != claims["ru"] {
		t.Fatalf("primary claim = %q, %v; want default language %q", primary, err, claims["ru"])
	}
	artifact.Steps[0].Value = "22"
	changed, err := index.DeriveClaims(artifact)
	if err != nil || !reflect.DeepEqual(claims, changed) {
		t.Fatalf("rule claim changed with sample actual: before=%v after=%v err=%v", claims, changed, err)
	}
	artifact.Title = "a sample-derived title"
	if _, err := index.DeriveClaim(artifact); err == nil || !strings.Contains(err.Error(), "conflicts with derived claim") {
		t.Fatalf("title conflicting with normative text should reject, got %v", err)
	}
}

func TestAtomSourceMultilingualDiagnosticsHaveMethodLanguageAndLine(t *testing.T) {
	cases := []struct {
		name string
		doc  string
		line string
		lang string
	}{
		{"unknown", "// >>>>> lang=en\n// english\n// >>>>> lang=fr\n// français\n// >>>>> lang=ru\n// русский\n", "value.go:5:", `language "fr"`},
		{"duplicate", "// >>>>> lang=en\n// english\n// >>>>> lang=en\n// duplicate\n// >>>>> lang=ru\n// русский\n", "value.go:5:", `language "en"`},
		{"empty", "// >>>>> lang=en\n// >>>>> lang=ru\n// русский\n", "value.go:3:", `language "en"`},
		{"outside", "// prose outside\n// >>>>> lang=en\n// english\n// >>>>> lang=ru\n// русский\n", "value.go:3:", "Box.Value"},
		{"multiple-paragraphs", "// >>>>> lang=en\n// first phrase\n//\n// second phrase\n// >>>>> lang=ru\n// русская фраза\n", "value.go:6:", `language "en"`},
		{"missing", "// >>>>> lang=en\n// english\n", "value.go:3:", `language "ru"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := "package model\ntype Box struct{}\n" + tc.doc + "func (b Box) Value() int { return 1 }\n"
			root := atomSourceFixture(t, map[string]string{"spec/model/value.go": source})
			graph := &ontology.Graph{DomainDir: root, Languages: []string{"en", "ru"}, DefaultLanguage: "en"}
			_, err := gate.NewAtomSourceIndexForGraph(graph)
			if err == nil {
				t.Fatal("invalid multilingual source accepted")
			}
			for _, part := range []string{tc.line, "Box.Value", tc.lang} {
				if !strings.Contains(err.Error(), part) {
					t.Errorf("diagnostic %q missing %q", err, part)
				}
			}
		})
	}
}

func TestAtomSourcePlainDocInMultilingualModeReportsMissingTranslationOnUse(t *testing.T) {
	root := atomSourceFixture(t, map[string]string{
		"spec/model/value.go": "package model\ntype Box struct{}\n// plain single-language phrase\nfunc (b Box) Value() int { return 1 }\n",
	})
	graph := &ontology.Graph{DomainDir: root, Languages: []string{"en", "ru"}, DefaultLanguage: "en"}
	index, err := gate.NewAtomSourceIndexForGraph(graph)
	if err != nil {
		t.Fatal(err)
	}
	artifact := gate.AtomArtifact{ReqID: "R-box-value", Mode: "fact", Verdict: "pass", Steps: []gate.AtomStep{{Subject: "example.test/domain/model.Box.Value", Value: "1"}}}
	_, err = index.DeriveClaim(artifact)
	if err == nil || !strings.Contains(err.Error(), "value.go:3:") || !strings.Contains(err.Error(), `language "en"`) {
		t.Fatalf("plain multilingual atom should report exact missing locale at source, got %v", err)
	}
}

func TestAtomSourceSingleExplicitLocaleAcceptsPlainDoc(t *testing.T) {
	root := atomSourceFixture(t, map[string]string{
		"spec/model/value.go": "package model\ntype Box struct{}\n// число точно задано\nfunc (b Box) Value() int { return 7 }\n",
	})
	graph := &ontology.Graph{DomainDir: root, Languages: []string{"ru"}}
	index, err := gate.NewAtomSourceIndexForGraph(graph)
	if err != nil {
		t.Fatal(err)
	}
	artifact := gate.AtomArtifact{ReqID: "R-box-value", Mode: "fact", Verdict: "pass", Steps: []gate.AtomStep{{Subject: "example.test/domain/model.Box.Value", Value: "7"}}}
	claim, err := index.DeriveClaim(artifact)
	if err != nil || claim != "Число точно задано — 7." {
		t.Fatalf("single-locale plain doc claim = %q, %v", claim, err)
	}
}

func TestAtomSourceRuleRequiresRuleCasesTrigger(t *testing.T) {
	root := atomSourceFixture(t, map[string]string{"spec/model/value.go": multilingualRuleModel})
	graph := &ontology.Graph{DomainDir: root, Languages: []string{"en", "ru", "zh"}, DefaultLanguage: "ru"}
	index, err := gate.NewAtomSourceIndexForGraph(graph)
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := gate.DecodeAtomArtifact([]byte(`{"req_id":"R-box-value","test":"TestRule","mode":"rule","verdict":"pass","steps":[{"subject":"example.test/domain/model.Box.Value","value":"1"}],"case":{"id":"case-one"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := index.DeriveClaim(artifact); err == nil || !strings.Contains(err.Error(), "conformance.rule_cases") {
		t.Fatalf("rule mode without its own trigger should reject, got %v", err)
	}
}
