package gate

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func TestSpecLocalizedAtomSteps(t *testing.T) {
	for _, defaultLanguage := range []string{"ru", "en"} {
		t.Run("default_"+defaultLanguage, func(t *testing.T) {
			root := writeAtomPipelineFixture(t, map[string]string{
				"manifest.json": `{"self_hosting":false}`,
				"spec/model/atoms.go": `package model
type Role string
const (
// >>>>> lang=ru
// переводчик
// >>>>> lang=en
// translator
RoleTranslator Role = "переводчик"
)
type Box struct{}
// >>>>> lang=ru
// Значение
// >>>>> lang=en
// Value
func (Box) Value() Role { return RoleTranslator }
// >>>>> lang=ru
// Условие соблюдено
// >>>>> lang=en
// The condition holds
func (Box) Holds() bool { return true }
// >>>>> lang=ru
// Правило выполнено
// >>>>> lang=en
// The rule applies
func (Box) Rule() int { return 7 }
`,
			})
			g := &ontology.Graph{DomainDir: root, Languages: []string{"ru", "en"}, DefaultLanguage: defaultLanguage, Conformance: &ontology.ConformanceConfig{RuleCases: true}}
			index, err := NewAtomSourceIndexForGraph(g)
			if err != nil {
				t.Fatal(err)
			}
			tests := []struct {
				name, mode           string
				steps                []AtomStep
				thenRU, thenEN       string
				givenRU, givenEN     string
				foreignRU, foreignEN []string
			}{
				{"fact_constant", "fact", []AtomStep{{Subject: "example.test/pipeline/model.Box.Value", Value: "переводчик"}}, "- Тогда Значение — переводчик.", "- Then Value — translator.", "", "", []string{"Value — translator"}, []string{"Значение", "переводчик"}},
				{"holds_constant_evidence", "holds", []AtomStep{{Subject: "example.test/pipeline/model.Box.Holds", Value: "true"}, {Subject: "example.test/pipeline/model.Box.Value", Value: "переводчик"}}, "- Тогда Условие соблюдено.", "- Then The condition holds.", "- Дано Значение — переводчик.", "- Given Value — translator.", []string{"The condition holds", "The rule applies", "translator"}, []string{"Условие соблюдено", "Правило выполнено", "переводчик"}},
				{"rule_constant_evidence", "rule", []AtomStep{{Subject: "example.test/pipeline/model.Box.Rule", Value: "7"}, {Subject: "example.test/pipeline/model.Box.Value", Value: "переводчик"}}, "- Тогда Правило выполнено.", "- Then The rule applies.", "- Дано Значение — переводчик.", "- Given Value — translator.", []string{"The rule applies", "translator"}, []string{"Правило выполнено", "переводчик"}},
			}
			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					a := AtomArtifact{ReqID: "R-" + tc.name, Mode: tc.mode, Verdict: "pass", Steps: tc.steps}
					if tc.mode == "rule" {
						a.Case = &AtomCaseContext{ID: "case-rule"}
					}
					steps, err := humanizeAtomSteps(index, a)
					if err != nil {
						t.Fatal(err)
					}
					raw, err := json.Marshal(steps[0])
					if err != nil {
						t.Fatal(err)
					}
					var restored specArtifactStep
					if err = json.Unmarshal(raw, &restored); err != nil {
						t.Fatal(err)
					}
					if restored.Texts != nil {
						t.Fatal("localized texts unexpectedly entered serialized artifact schema")
					}
					g.Requirements = []ontology.Requirement{{ID: a.ReqID, Claim: "Source", ClaimTexts: ontology.LocalizedText{"ru": "Русская норма", "en": "English norm"}, Status: ontology.StatusSETTLED, VerifiedBy: []string{"spec/model/atoms_test.go:TestAtom"}}}
					localizedSteps, err := humanizeAtomSteps(index, a)
					if err != nil {
						t.Fatal(err)
					}
					rows := map[string]SpecRow{a.ReqID: {req: g.Requirements[0], outcomes: []specTestOutcome{{entry: g.Requirements[0].VerifiedBy[0], artifacts: []specArtifact{{ReqID: a.ReqID, Mode: tc.mode, Verdict: "pass", Steps: localizedSteps}}}}}}
					for _, lang := range []string{"ru", "en"} {
						doc, err := BuildSpecFromRowsForLanguage(g, rows, lang)
						if err != nil {
							t.Fatal(err)
						}
						then, given, foreignThen, foreignGiven := tc.thenRU, tc.givenRU, "- Then "+strings.TrimPrefix(tc.thenEN, "- Then "), tc.givenEN
						if lang == "en" {
							then, given, foreignThen, foreignGiven = tc.thenEN, tc.givenEN, "- Тогда "+strings.TrimPrefix(tc.thenRU, "- Тогда "), tc.givenRU
						}
						foreignPhrases := tc.foreignRU
						if lang == "en" {
							foreignPhrases = tc.foreignEN
						}
						for _, foreign := range foreignPhrases {
							if strings.Contains(doc, foreign) {
								t.Fatalf("foreign phrase %q in %s document:\n%s", foreign, lang, doc)
							}
						}
						if !strings.Contains(doc, then) || (given != "" && !strings.Contains(doc, given)) {
							t.Fatalf("wrong %s narrative, expected then=%q given=%q foreign=%q/%q:\n%s", lang, then, given, foreignThen, foreignGiven, doc)
						}
						shards, err := BuildSpecDocumentsFromRows(g, rows)
						if err != nil {
							t.Fatal(err)
						}
						shard, ok := shards["spec/"+lang+"/model.md"]
						if !ok {
							t.Fatalf("missing localized shard for %s", lang)
						}
						if !strings.Contains(shard, then) || (given != "" && !strings.Contains(shard, given)) || strings.Contains(shard, foreignThen) || (foreignGiven != "" && strings.Contains(shard, foreignGiven)) {
							t.Fatalf("wrong %s shard:\n%s", lang, shard)
						}
					}
				})
			}
			failed := AtomArtifact{ReqID: "R-failed", Mode: "rule", Verdict: "fail", Case: &AtomCaseContext{ID: "failed"}, Steps: []AtomStep{{Subject: "example.test/pipeline/model.Box.Rule", Value: "7"}, {Subject: "example.test/pipeline/model.Box.Value", Value: "переводчик"}}}
			failedSteps, err := humanizeAtomSteps(index, failed)
			if err != nil {
				t.Fatal(err)
			}
			failedReq := ontology.Requirement{ID: failed.ReqID, Claim: "Source", ClaimTexts: ontology.LocalizedText{"ru": "Русская норма", "en": "English norm"}, Status: ontology.StatusSETTLED, VerifiedBy: []string{"spec/model/atoms_test.go:TestAtom"}}
			fg := *g
			fg.Requirements = []ontology.Requirement{failedReq}
			fr := map[string]SpecRow{failed.ReqID: {req: failedReq, outcomes: []specTestOutcome{{entry: failedReq.VerifiedBy[0], failedArtifacts: []specArtifact{{ReqID: failed.ReqID, Mode: "rule", Verdict: "fail", Steps: failedSteps}}}}}}
			for _, lang := range []string{"ru", "en"} {
				doc, err := BuildSpecFromRowsForLanguage(&fg, fr, lang)
				if err != nil {
					t.Fatal(err)
				}
				then, given, foreign := "- Тогда Правило выполнено.", "- Дано Значение — переводчик.", "- Then The rule applies."
				if lang == "en" {
					then, given, foreign = "- Then The rule applies.", "- Given Value — translator.", "- Тогда Правило выполнено."
				}
				if !strings.Contains(doc, then) || !strings.Contains(doc, given) || (lang == "en" && !strings.Contains(doc, "FAILED")) || (lang == "ru" && !strings.Contains(doc, "ОШИБКА")) || strings.Contains(doc, foreign) {
					t.Fatalf("failed locale narrative mismatch %s:\n%s", lang, doc)
				}
				shards, err := BuildSpecDocumentsFromRows(&fg, fr)
				if err != nil {
					t.Fatal(err)
				}
				shard, ok := shards["spec/"+lang+"/model.md"]
				if !ok || !strings.Contains(shard, given) || strings.Contains(shard, foreign) {
					t.Fatalf("failed shard mismatch %s:\n%s", lang, shard)
				}
			}
		})
	}
}

func TestLocalizedGivenSingleLanguagePrimaryText(t *testing.T) {
	for _, lang := range []string{"ru", "en"} {
		steps := []specArtifactStep{{Kind: "given", Desc: map[string]string{"ru": "Значение — переводчик.", "en": "Value — translator."}[lang], Subject: "Box.Value", Value: "7"}}
		got, err := renderSpecSteps(steps, "")
		if err != nil {
			t.Fatal(err)
		}
		want := map[string]string{"ru": "- Given Значение — переводчик.", "en": "- Given Value — translator."}[lang]
		if len(got) != 1 || got[0] != want {
			t.Fatalf("%s primary rendering=%q want %q", lang, got, want)
		}
		steps[0].Texts = ontology.LocalizedText{"ru": "Русский", "zh": "中文"}
		if _, err = renderSpecSteps(steps, "en"); err == nil {
			t.Fatal("empty localized map should refuse missing locale")
		}
		steps[0].Texts = ontology.LocalizedText{"en": " "}
		if _, err = renderSpecSteps(steps, "en"); err == nil {
			t.Fatal("whitespace localized text should refuse")
		}
	}
}

func TestRenderSpecStepsLegacySubjectBytes(t *testing.T) {
	got, err := renderSpecSteps([]specArtifactStep{{Subject: "Box.Value", Value: "7"}}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "- `Box.Value` — 7" {
		t.Fatalf("legacy rendering changed: %#v", got)
	}
}

func TestRenderSpecStepsMissingLocalizedAtomLanguage(t *testing.T) {
	_, err := renderSpecSteps([]specArtifactStep{{Kind: "then", Desc: "Русский", Texts: ontology.LocalizedText{"ru": "Русский"}, Passed: true}}, "en")
	if err == nil || !strings.Contains(err.Error(), `language "en"`) {
		t.Fatalf("missing locale error=%v", err)
	}
}
