package gate

import (
	"encoding/base64"
	"encoding/json"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/docbundle"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func specDocumentFixture(t *testing.T) (*ontology.Graph, map[string]SpecRow) {
	t.Helper()
	localized := func(en, ru, zh string) ontology.LocalizedText {
		return ontology.LocalizedText{"en": en, "ru": ru, "zh": zh}
	}
	selected := ontology.CaseDefinition{ID: "selected-final-case", ClauseIDs: []string{"input.bom"}, AtomIDs: []string{"R-input"}, Test: "spec/model/input_test.go:TestInput"}
	hidden := ontology.CaseDefinition{ID: "hidden-case", ClauseIDs: []string{"input.bom"}, AtomIDs: []string{"R-input"}, Test: selected.Test}
	input := &ontology.ObservedValue{Kind: "bytes", Encoding: "base64", Bytes: base64.StdEncoding.EncodeToString([]byte("\ufeffn: -0.0"))}
	expected := &ontology.ObservedValue{Kind: "object", Fields: map[string]ontology.ObservedValue{"n": {Kind: "float", FloatBits: "8000000000000000"}}}
	observation := specObservation{Name: "technical-parse-fingerprint", Passed: true, RawInput: input, RawActual: expected, RawExpected: expected}
	hiddenText := "do not promote this observation"
	hiddenObservation := specObservation{Name: "hidden-observation", Passed: true, RawInput: &ontology.ObservedValue{Kind: "text", Text: &hiddenText}, RawActual: expected, RawExpected: expected}
	graph := &ontology.Graph{
		Languages: []string{"en", "ru", "zh"}, DefaultLanguage: "en", SelfExecutingAtoms: true,
		Requirements: []ontology.Requirement{
			{ID: "R-grammar", AtomKind: "rule", ImplementedBy: []string{"spec/model/grammar/grammar.go:Box.Grammar"}, ClauseLinks: []ontology.ClauseLink{{ClauseID: "grammar.pair"}}},
			{ID: "R-input", AtomKind: "rule", ImplementedBy: []string{"spec/model/input/input.go:Box.Input"}, ClauseLinks: []ontology.ClauseLink{{ClauseID: "input.bom"}}, Cases: []ontology.CaseDefinition{hidden, selected}},
		},
		Conformance: &ontology.ConformanceConfig{
			RuleCases: true,
			Clauses:   []ontology.SourceClause{{ID: "input.bom"}, {ID: "grammar.pair"}},
			DocumentSections: []ontology.DocumentSection{
				{ID: "language.grammar", Order: 30, TitleTexts: localized("§ 4 Grammar", "§ 4 Грамматика", "§ 4 文法"), Blocks: []ontology.NormativeBlock{{ID: "text.grammar", TextRef: "normative.Grammar", Role: "normative", ClauseIDs: []string{"grammar.pair"}}}},
				{ID: "language.input", ParentID: "language.definitions", Order: 1, TitleTexts: localized("§ 3.1 Input", "§ 3.1 Вход", "§ 3.1 输入"), Blocks: []ontology.NormativeBlock{{ID: "text.input", TextRef: "normative.Input", Role: "normative", ClauseIDs: []string{"input.bom"}, Examples: []ontology.DocumentExample{{CaseID: selected.ID, ComparisonNames: []string{observation.Name}}}}}},
				{ID: "language.definitions", Order: 20, TitleTexts: localized("§ 3 Definitions", "§ 3 Определения", "§ 3 定义"), Blocks: []ontology.NormativeBlock{{ID: "text.definition", TextRef: "normative.Definition", Role: "informative"}}},
			},
		},
	}
	rows := map[string]SpecRow{
		"R-input": {req: graph.Requirements[1], outcomes: []specTestOutcome{{passed: true, artifacts: []specArtifact{
			{Case: &hidden, Verdict: "pass", Observations: []specObservation{hiddenObservation}},
			{Case: &selected, Verdict: "pass", Observations: []specObservation{observation}},
		}}}},
	}
	data := &specDocumentSnapshot{
		texts: map[string]ontology.LocalizedText{
			"normative.Input": localized(
				"The parser MUST skip exactly one leading BOM (U+FEFF); any other BOM is content. Negative zero retains its sign. See § 4 and [grammar](#text.grammar).",
				"Парсер MUST пропустить ровно один ведущий BOM (U+FEFF); прочие BOM являются содержимым. Отрицательный ноль сохраняет знак. См. § 4 и [грамматику](#text.grammar).",
				"解析器 MUST 跳过恰好一个前导 BOM (U+FEFF)；其他 BOM 是内容。负零保留符号。见 § 4 和[文法](#text.grammar)。"),
			"normative.Grammar":    localized("    pair = key, ':', value;\n\nA pair MUST have a key.", "    pair = key, ':', value;\n\nПара MUST иметь ключ.", "    pair = key, ':', value;\n\n键值对 MUST 包含键。"),
			"normative.Definition": localized("A document is a sequence of Unicode code points.", "Документ — последовательность кодовых точек Unicode.", "文档是 Unicode 码点序列。"),
		},
		examples: make(map[string][]specObservation),
	}
	if err := collectSpecDocumentExamples(graph, rows, map[string][]string{selected.ID: {observation.Name}}, data.examples); err != nil {
		t.Fatal(err)
	}
	rows[specDocumentRowKey] = SpecRow{document: data}
	return graph, rows
}

func TestSpecDocumentFollowsAuthoredSectionsAndOnlySelectedExamples(t *testing.T) {
	graph, rows := specDocumentFixture(t)
	documents, err := BuildSpecDocumentsFromRows(graph, rows)
	if err != nil {
		t.Fatal(err)
	}
	root := documents["SPEC.en.md"]
	definitions := strings.Index(root, "## § 3 Definitions")
	input := strings.Index(root, "### § 3.1 Input")
	grammar := strings.Index(root, "## § 4 Grammar")
	if definitions < 0 || input <= definitions || grammar <= input {
		t.Fatalf("authored hierarchy/order was replaced by method order:\n%s", root)
	}
	if strings.Count(root, "The parser MUST skip exactly one leading BOM") != 1 || !strings.Contains(root, "    pair = key, ':', value;") {
		t.Fatalf("normative text was repeated or grammar was omitted:\n%s", root)
	}
	for _, forbidden := range []string{"hidden-case", "do not promote this observation", "technical-parse-fingerprint", "8000000000000000", "UTF-8:", "Executed checks", "Domain operation", "R-input", "rule-R"} {
		if strings.Contains(root, forbidden) {
			t.Errorf("reader document leaked unselected/technical journal content %q", forbidden)
		}
	}
	if !strings.Contains(root, "```ktav\n\ufeffn: -0.0\n```") || !strings.Contains(root, "n: -0.0") || !strings.Contains(root, "```json5") {
		t.Fatalf("literal Ktav input or signed semantic JSON5 result was lost:\n%s", root)
	}
	if !strings.Contains(root, "[§ 4](#language.grammar)") || !strings.Contains(root, "[grammar](#text.grammar)") {
		t.Fatalf("canonical section/block references are absent:\n%s", root)
	}
	shard := documents["spec/en/model/input.md"]
	if !strings.Contains(shard, "[grammar](../../../SPEC.en.md#text.grammar)") || strings.Contains(shard, "A pair MUST have a key") || !strings.Contains(shard, "../../../SPEC.en.md") {
		t.Fatalf("focused view is incomplete or has a dangling cross-package reference:\n%s", shard)
	}
	layout, err := docbundle.NewLayout(graph.Languages, graph.DefaultLanguage)
	if err != nil {
		t.Fatal(err)
	}
	target, err := layout.EvidenceCasePath("en", "R-input", "selected-final-case")
	if err != nil {
		t.Fatal(err)
	}
	relative, err := specDocumentRelative("docs/gen/SPEC.en.md", target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(root, "input.bom: ") || !strings.Contains(root, "]("+relative+")") {
		t.Fatalf("explicit clause witness cannot be discovered from its block:\n%s", root)
	}
}

func TestSpecDocumentAnchorsAndCanonicalLinksRemainValidAcrossLocales(t *testing.T) {
	graph, rows := specDocumentFixture(t)
	documents, err := BuildSpecDocumentsFromRows(graph, rows)
	if err != nil {
		t.Fatal(err)
	}
	anchors := regexp.MustCompile(`<a id="([^"]+)"></a>`)
	var canonical []string
	for _, language := range graph.Languages {
		root := documents["SPEC."+language+".md"]
		var current []string
		for _, match := range anchors.FindAllStringSubmatch(root, -1) {
			current = append(current, match[1])
		}
		if canonical == nil {
			canonical = current
		} else if !reflect.DeepEqual(canonical, current) {
			t.Fatalf("locale %s changes canonical identities: %v != %v", language, current, canonical)
		}
		for key, document := range documents {
			if !strings.Contains(key, "/"+language+"/") && key != "SPEC."+language+".md" {
				continue
			}
			for _, match := range regexp.MustCompile(`\]\(([^)]*)#([^)]+)\)`).FindAllStringSubmatch(document, -1) {
				targetKey := key
				if match[1] != "" {
					targetKey = filepath.ToSlash(filepath.Clean(filepath.Join(filepath.Dir(key), filepath.FromSlash(match[1]))))
				}
				if !strings.Contains(documents[targetKey], `<a id="`+match[2]+`"></a>`) {
					t.Errorf("%s has an unresolved canonical link %s#%s", key, targetKey, match[2])
				}
			}
		}
	}
	graph.Requirements[1].ID = "R-renamed-method"
	graph.Requirements[1].ImplementedBy[0] = "spec/model/input/input.go:Box.Renamed"
	rows["R-renamed-method"] = rows["R-input"]
	delete(rows, "R-input")
	rendered, err := BuildSpecFromRowsForLanguage(graph, rows, "en")
	if err != nil {
		t.Fatal(err)
	}
	var after []string
	for _, match := range anchors.FindAllStringSubmatch(rendered, -1) {
		after = append(after, match[1])
	}
	if !reflect.DeepEqual(canonical, after) {
		t.Fatalf("method identity changes document anchors: %v != %v", canonical, after)
	}
}

func TestSpecDocumentRefusesIncompleteOrAmbiguousPublication(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		change func(*ontology.Graph, map[string]SpecRow)
	}{
		{"missing text", func(g *ontology.Graph, rows map[string]SpecRow) {
			delete(rows[specDocumentRowKey].document.texts, "normative.Grammar")
		}},
		{"missing translation", func(g *ontology.Graph, rows map[string]SpecRow) {
			delete(rows[specDocumentRowKey].document.texts["normative.Grammar"], "zh")
		}},
		{"unknown clause", func(g *ontology.Graph, rows map[string]SpecRow) {
			g.Conformance.DocumentSections[0].Blocks[0].ClauseIDs = []string{"unknown"}
		}},
		{"omitted norm", func(g *ontology.Graph, rows map[string]SpecRow) {
			g.Conformance.Clauses = append(g.Conformance.Clauses, ontology.SourceClause{ID: "omitted"})
		}},
		{"unmatched example", func(g *ontology.Graph, rows map[string]SpecRow) {
			g.Conformance.DocumentSections[1].Blocks[0].Examples = []ontology.DocumentExample{{CaseID: "unknown", ComparisonNames: []string{"technical-parse-fingerprint"}}}
		}},
		{"unwitnessed example", func(g *ontology.Graph, rows map[string]SpecRow) {
			g.Requirements[1].Cases[1].ClauseIDs = []string{"grammar.pair"}
		}},
		{"unknown fragment", func(g *ontology.Graph, rows map[string]SpecRow) {
			rows[specDocumentRowKey].document.texts["normative.Grammar"]["en"] += "\n[absent](#absent)"
		}},
		{"ambiguous ordering", func(g *ontology.Graph, rows map[string]SpecRow) { g.Conformance.DocumentSections[0].Order = 20 }},
		{"shared text duplicated", func(g *ontology.Graph, rows map[string]SpecRow) {
			g.Conformance.DocumentSections[0].Blocks[0].TextRef = "normative.Input"
		}},
		{"parent cycle", func(g *ontology.Graph, rows map[string]SpecRow) {
			g.Conformance.DocumentSections[2].ParentID = "language.input"
		}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			graph, rows := specDocumentFixture(t)
			scenario.change(graph, rows)
			if documents, err := BuildSpecDocumentsFromRows(graph, rows); err == nil || documents != nil {
				t.Fatalf("invalid document returned a partial publication: err=%v, documents=%v", err, documents)
			}
		})
	}
}

func TestSpecDocumentSelectedFailureCannotBeHiddenByPassingSibling(t *testing.T) {
	graph, rows := specDocumentFixture(t)
	outcome := rows["R-input"].outcomes[0]
	failed := outcome.artifacts[1]
	failed.Verdict = "fail"
	failed.Observations = append([]specObservation(nil), failed.Observations...)
	failed.Observations[0].Passed = false
	outcome.failedArtifacts = []specArtifact{failed}
	row := rows["R-input"]
	row.outcomes = []specTestOutcome{outcome}
	rows["R-input"] = row
	if err := collectSpecDocumentExamples(graph, rows, map[string][]string{"selected-final-case": {"technical-parse-fingerprint"}}, make(map[string][]specObservation)); err == nil {
		t.Fatal("a passing selected example hid a failed comparison with the same case identity")
	}
	outcome.failedArtifacts = nil
	outcome.artifacts[1].Observations = append([]specObservation(nil), outcome.artifacts[1].Observations...)
	wrong := "unexpected"
	outcome.artifacts[1].Observations[0].RawActual = &ontology.ObservedValue{Kind: "text", Text: &wrong}
	row.outcomes = []specTestOutcome{outcome}
	rows["R-input"] = row
	if err := collectSpecDocumentExamples(graph, rows, map[string][]string{"selected-final-case": {"technical-parse-fingerprint"}}, make(map[string][]specObservation)); err == nil {
		t.Fatal("a recorded passed label hid an actual/expected mismatch")
	}
}

func TestSpecDocumentLeavesEvidenceExactAndCodeExamplesLiteral(t *testing.T) {
	graph, rows := specDocumentFixture(t)
	data := rows[specDocumentRowKey].document
	before, err := json.Marshal(data.examples["selected-final-case"])
	if err != nil {
		t.Fatal(err)
	}
	literal := "~~~ktav\n[not a reference](#absent)\n§ 4\n~~~\n\n`[also literal](#absent)`\n\n    [grammar literal](#absent)"
	data.texts["normative.Definition"]["en"] += "\n\n" + literal
	documents, err := BuildSpecDocumentsFromRows(graph, rows)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(documents["SPEC.en.md"], literal) {
		t.Fatal("fenced, inline or indented literal content was interpreted as document links")
	}
	after, err := json.Marshal(data.examples["selected-final-case"])
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("reader projection changed the exact evidence values")
	}
}

func TestSpecDocumentInlineCodeUsesExactDelimiterRuns(t *testing.T) {
	for _, literal := range []string{
		"``literal ``` § 4 [not a reference](#unknown) literal``",
		"```literal `` § 4 [not a reference](#unknown) literal```",
		"``literal ` § 4 [not a reference](#unknown) literal``",
		"``literal ```` § 4 [not a reference](#unknown) literal``",
	} {
		t.Run(literal, func(t *testing.T) {
			graph, rows := specDocumentFixture(t)
			rows[specDocumentRowKey].document.texts["normative.Definition"]["en"] += "\n\n" + literal
			documents, err := BuildSpecDocumentsFromRows(graph, rows)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(documents["SPEC.en.md"], literal) {
				t.Fatal("a shorter/longer internal backtick run changed literal inline code")
			}
		})
	}
	graph, rows := specDocumentFixture(t)
	rows[specDocumentRowKey].document.texts["normative.Definition"]["en"] += "\n\n``unmatched `literal [not a reference](#unknown)` § 4"
	documents, err := BuildSpecDocumentsFromRows(graph, rows)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(documents["SPEC.en.md"], "``unmatched `literal [not a reference](#unknown)` [§ 4](#language.grammar)") {
		t.Fatal("an unmatched opener hid later exact inline code or a prose reference")
	}
	rows[specDocumentRowKey].document.texts["normative.Definition"]["en"] += "\n\n``unmatched [invalid prose reference](#unknown)"
	if documents, err := BuildSpecDocumentsFromRows(graph, rows); err == nil || documents != nil {
		t.Fatal("an unmatched delimiter hid an invalid prose reference from publication validation")
	}
}

func TestSpecDocumentCollectsAuthoredTextWithoutExecutableRequirements(t *testing.T) {
	root := writeAtomPipelineFixture(t, map[string]string{
		"manifest.json": `{"self_hosting":false}`,
		"spec/model/normative/normative.go": `package normative
 type Text string
 // >>>>> lang=en
 // A document consists of Unicode code points.
 // >>>>> lang=ru
 // Документ состоит из кодовых точек Unicode.
 // >>>>> lang=zh
 // 文档由 Unicode 码点组成。
 const Definition Text = "language.definition"
 `,
	})
	graph := &ontology.Graph{DomainDir: root, Languages: []string{"en", "ru", "zh"}, DefaultLanguage: "en", Conformance: &ontology.ConformanceConfig{DocumentSections: []ontology.DocumentSection{{
		ID: "language.definition", TitleTexts: ontology.LocalizedText{"en": "Definitions", "ru": "Определения", "zh": "定义"},
		Blocks: []ontology.NormativeBlock{{ID: "text.definition", TextRef: "spec/model/normative/normative.go:Definition", Role: "informative"}},
	}}}}
	index, err := NewAtomSourceIndexForGraph(graph)
	if err != nil {
		t.Fatal(err)
	}
	rows := CollectSpecRowsFromSnapshot(graph, &AtomExecutionSnapshot{SourceIndex: index})
	documents, err := BuildSpecDocumentsFromRows(graph, rows)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(documents["SPEC.ru.md"], "Документ состоит из кодовых точек Unicode.") || strings.Contains(documents["SPEC.ru.md"], "A document consists") {
		t.Fatalf("informative-only snapshot lost authored locale text: %s", documents["SPEC.ru.md"])
	}
}

func TestSpecDocumentExplicitEmptyCaseScopeCannotBecomeABroadWitness(t *testing.T) {
	declared := ontology.CaseDefinition{ID: "no-witness", ClauseIDs: []string{}}
	merged, err := mergeRecordedCase(declared, ontology.CaseDefinition{ID: declared.ID})
	if err != nil {
		t.Fatal(err)
	}
	if merged.ClauseIDs == nil {
		t.Fatal("explicit non-witness scope became absent/inferable")
	}
	declared.ClauseIDs = []string{"whole-paragraph"}
	if _, err := mergeRecordedCase(declared, ontology.CaseDefinition{ID: declared.ID, ClauseIDs: []string{}}); err == nil {
		t.Fatal("an explicit recorded non-witness was upgraded to broad declaration scope")
	}
}

func TestSpecDocumentMalformedSelectedValuesRejectTheWholeBundle(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		change func(*specObservation)
	}{
		{"input encoding", func(o *specObservation) {
			o.RawInput = &ontology.ObservedValue{Kind: "bytes", Encoding: "hex", Bytes: "/w=="}
		}},
		{"input base64", func(o *specObservation) {
			o.RawInput = &ontology.ObservedValue{Kind: "bytes", Encoding: "base64", Bytes: "not-base64"}
		}},
		{"expected metadata", func(o *specObservation) {
			o.RawExpected = &ontology.ObservedValue{Kind: "integer", Integer: "7", FloatBits: "0000000000000000"}
			o.RawActual = o.RawExpected
		}},
		{"actual metadata", func(o *specObservation) { o.RawActual = &ontology.ObservedValue{Kind: "bool"} }},
		{"missing expected", func(o *specObservation) { o.RawExpected = nil }},
		{"missing actual", func(o *specObservation) { o.RawActual = nil }},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			graph, rows := specDocumentFixture(t)
			data := rows[specDocumentRowKey].document
			observation := data.examples["selected-final-case"][0]
			scenario.change(&observation)
			data.examples["selected-final-case"][0] = observation
			row := rows["R-input"]
			row.outcomes[0].artifacts[1].Observations[0] = observation
			rows["R-input"] = row
			if err := collectSpecDocumentExamples(graph, rows, map[string][]string{"selected-final-case": {"technical-parse-fingerprint"}}, make(map[string][]specObservation)); err == nil {
				t.Fatal("collection accepted an invalid selected input/expected/actual value")
			}
			if documents, err := BuildSpecDocumentsFromRows(graph, rows); err == nil || documents != nil {
				t.Fatal("malformed selected metadata became a partial or counterfeit reader publication")
			}
		})
	}
}

func TestSpecDocumentOpaqueBytesRemainExplicitAndLinkToExactCaseEvidence(t *testing.T) {
	for _, role := range []string{"input", "expected", "nested expected"} {
		t.Run(role, func(t *testing.T) {
			graph, rows := specDocumentFixture(t)
			data := rows[specDocumentRowKey].document
			opaque := &ontology.ObservedValue{Kind: "bytes", Encoding: "base64", Bytes: base64.StdEncoding.EncodeToString([]byte{0xff, 0x00})}
			observation := data.examples["selected-final-case"][0]
			switch role {
			case "input":
				observation.RawInput = opaque
				code := "InvalidUtf8"
				observation.RawExpected = &ontology.ObservedValue{Kind: "text", Text: &code}
			case "expected":
				observation.RawExpected = opaque
			case "nested expected":
				observation.RawExpected = &ontology.ObservedValue{Kind: "object", Fields: map[string]ontology.ObservedValue{"payload": *opaque}}
			}
			observation.RawActual = observation.RawExpected
			row := rows["R-input"]
			row.outcomes[0].artifacts[1].Observations[0] = observation
			rows["R-input"] = row
			data.examples = make(map[string][]specObservation)
			if err := collectSpecDocumentExamples(graph, rows, map[string][]string{"selected-final-case": {"technical-parse-fingerprint"}}, data.examples); err != nil {
				t.Fatal(err)
			}
			before, err := json.Marshal(data.examples["selected-final-case"])
			if err != nil {
				t.Fatal(err)
			}
			documents, err := BuildSpecDocumentsFromRows(graph, rows)
			if err != nil {
				t.Fatal(err)
			}
			layout, err := docbundle.NewLayout(graph.Languages, graph.DefaultLanguage)
			if err != nil {
				t.Fatal(err)
			}
			root := documents["SPEC.en.md"]
			if !strings.Contains(root, "> **Opaque bytes (not UTF-8).**") {
				t.Fatal("a valid opaque value was not explicitly distinguished from Unicode text/JSON5")
			}
			if strings.Contains(root, opaque.Bytes) || strings.Contains(root, "FF 00") {
				t.Fatal("opaque reader prose leaked raw base64/hex data instead of linking evidence")
			}
			json5Blocks := regexp.MustCompile("(?s)```json5\\n(.*?)\\n```")
			for _, block := range json5Blocks.FindAllStringSubmatch(root, -1) {
				if strings.Contains(block[1], "null") || strings.Contains(block[1], opaque.Bytes) || strings.Contains(block[1], "FF 00") {
					t.Fatal("opaque bytes were replaced by counterfeit JSON5 or a technical byte dump")
				}
			}
			for _, language := range graph.Languages {
				target, err := layout.EvidenceCasePath(language, "R-input", "selected-final-case")
				if err != nil {
					t.Fatal(err)
				}
				base := "docs/gen/SPEC." + language + ".md"
				relative, err := specDocumentRelative(base, target)
				if err != nil {
					t.Fatal(err)
				}
				document := documents["SPEC."+language+".md"]
				// This link is part of the explanatory byte block, not merely
				// the separate clause inventory: it points to this exact case.
				explanation := regexp.MustCompile(`(?m)^> .*$`).FindString(document)
				if !strings.Contains(explanation, "]("+relative+")") {
					t.Fatalf("%s opaque value lacks its exact localized case evidence link", language)
				}
			}
			after, err := json.Marshal(data.examples["selected-final-case"])
			if err != nil {
				t.Fatal(err)
			}
			if string(before) != string(after) {
				t.Fatal("opaque reader prose changed exact machine comparison evidence")
			}
		})
	}
}

func TestSpecDocumentExplicitComparisonSelectionExcludesTechnicalRecordsWithoutChangingEvidence(t *testing.T) {
	graph, rows := specDocumentFixture(t)
	data := rows[specDocumentRowKey].document
	semantic := data.examples["selected-final-case"][0]
	secret := "never-expose-this-diagnostic-source"
	truth := true
	span := ontology.ObservedValue{Kind: "array", Items: []ontology.ObservedValue{{Kind: "integer", Integer: "0"}, {Kind: "integer", Integer: "9"}}}
	diagnostic := specObservation{
		Name: "recorded-span-proof", Passed: true,
		RawInput: &ontology.ObservedValue{Kind: "object", Fields: map[string]ontology.ObservedValue{
			"line": {Kind: "integer", Integer: "7"}, "source": {Kind: "text", Text: &secret}, "span": span,
		}},
		RawExpected: &ontology.ObservedValue{Kind: "bool", Bool: &truth},
		RawActual:   &ontology.ObservedValue{Kind: "bool", Bool: &truth},
	}
	row := rows["R-input"]
	// Put the technical record first: selection cannot be positional clipping.
	row.outcomes[0].artifacts[1].Observations = []specObservation{diagnostic, semantic}
	rows["R-input"] = row
	normativeOffsetRule := "An error's byte offset counts UTF-8 bytes, not Unicode code points."
	data.texts["normative.Grammar"]["en"] += "\n\n" + normativeOffsetRule
	before, err := json.Marshal(row.outcomes[0].artifacts[1].Observations)
	if err != nil {
		t.Fatal(err)
	}
	data.examples = make(map[string][]specObservation)
	if err := collectSpecDocumentExamples(graph, rows, map[string][]string{"selected-final-case": {semantic.Name}}, data.examples); err != nil {
		t.Fatal(err)
	}
	documents, err := BuildSpecDocumentsFromRows(graph, rows)
	if err != nil {
		t.Fatal(err)
	}
	root := documents["SPEC.en.md"]
	if !strings.Contains(root, "n: -0.0") || strings.Contains(root, secret) || strings.Contains(root, "recorded-span-proof") || strings.Contains(root, "span: [") {
		t.Fatal("the selected semantic comparison was lost or its unselected technical sibling was promoted")
	}
	if !strings.Contains(root, normativeOffsetRule) {
		t.Fatal("authored normative byte-offset semantics were suppressed with execution diagnostics")
	}
	layout, err := docbundle.NewLayout(graph.Languages, graph.DefaultLanguage)
	if err != nil {
		t.Fatal(err)
	}
	target, err := layout.EvidenceCasePath("en", "R-input", "selected-final-case")
	if err != nil {
		t.Fatal(err)
	}
	relative, err := specDocumentRelative("docs/gen/SPEC.en.md", target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(root, "[Example evidence]("+relative+")") {
		t.Fatal("the selected example lost its exact case evidence link")
	}
	after, err := json.Marshal(rows["R-input"].outcomes[0].artifacts[1].Observations)
	if err != nil {
		t.Fatal(err)
	}
	preservedSpan := rows["R-input"].outcomes[0].artifacts[1].Observations[0].RawInput.Fields["span"]
	if string(before) != string(after) || !ontology.EqualObservedValues(&span, &preservedSpan) {
		t.Fatal("reader comparison selection changed or removed the exact typed span evidence")
	}
}

func TestSpecDocumentComparisonSelectorsFailClosedOnMissingAmbiguousOrFailedRecords(t *testing.T) {
	for _, scenario := range []string{"missing", "duplicate selector", "empty selector", "duplicate record", "failed selected", "ambiguous execution"} {
		t.Run(scenario, func(t *testing.T) {
			graph, rows := specDocumentFixture(t)
			data := rows[specDocumentRowKey].document
			semantic := data.examples["selected-final-case"][0]
			selector := &graph.Conformance.DocumentSections[1].Blocks[0].Examples[0]
			row := rows["R-input"]
			switch scenario {
			case "missing":
				selector.ComparisonNames = []string{"absent-comparison"}
			case "duplicate selector":
				selector.ComparisonNames = []string{semantic.Name, semantic.Name}
			case "empty selector":
				selector.ComparisonNames = nil
			case "duplicate record":
				row.outcomes[0].artifacts[1].Observations = []specObservation{semantic, semantic}
				data.examples["selected-final-case"] = []specObservation{semantic, semantic}
			case "failed selected":
				passingSibling := semantic
				passingSibling.Name = "passing-sibling-comparison"
				failed := semantic
				failed.Passed = false
				row.outcomes[0].artifacts[1].Observations = []specObservation{passingSibling, failed}
				data.examples["selected-final-case"] = []specObservation{passingSibling, failed}
			case "ambiguous execution":
				row.outcomes[0].artifacts = append(row.outcomes[0].artifacts, row.outcomes[0].artifacts[1])
			}
			rows["R-input"] = row
			selections := map[string][]string{selector.CaseID: selector.ComparisonNames}
			err := collectSpecDocumentExamples(graph, rows, selections, make(map[string][]specObservation))
			if err == nil {
				t.Fatal("an invalid named comparison selector acquired reader proof")
			}
			if scenario == "ambiguous execution" {
				// Collection is the snapshot boundary: it must not silently
				// choose an identical-looking execution from duplicate origins.
				data.err = err
			}
			if documents, err := BuildSpecDocumentsFromRows(graph, rows); err == nil || documents != nil {
				t.Fatal("invalid named comparison selection returned a partial reader bundle")
			}
		})
	}
}

func TestSpecDocumentControlCharactersUseVisibleEscapesWithoutNormalizingEvidence(t *testing.T) {
	graph, rows := specDocumentFixture(t)
	data := rows[specDocumentRowKey].document
	source := "a: 1\nb: 2\r\nc: 3\rd: 4"
	observation := data.examples["selected-final-case"][0]
	observation.RawInput = &ontology.ObservedValue{Kind: "bytes", Encoding: "base64", Bytes: base64.StdEncoding.EncodeToString([]byte(source))}
	observation.RawExpected = &ontology.ObservedValue{Kind: "object", Fields: map[string]ontology.ObservedValue{
		"a": {Kind: "integer", Integer: "1"}, "b": {Kind: "integer", Integer: "2"}, "c": {Kind: "integer", Integer: "3"}, "d": {Kind: "integer", Integer: "4"},
	}}
	observation.RawActual = observation.RawExpected
	row := rows["R-input"]
	row.outcomes[0].artifacts[1].Observations[0] = observation
	rows["R-input"] = row
	data.examples = make(map[string][]specObservation)
	if err := collectSpecDocumentExamples(graph, rows, map[string][]string{"selected-final-case": {observation.Name}}, data.examples); err != nil {
		t.Fatal(err)
	}
	before, err := json.Marshal(row.outcomes[0].artifacts[1].Observations)
	if err != nil {
		t.Fatal(err)
	}
	documents, err := BuildSpecDocumentsFromRows(graph, rows)
	if err != nil {
		t.Fatal(err)
	}
	root := documents["SPEC.en.md"]
	if strings.ContainsRune(root, '\r') || !strings.Contains(root, "```json5\n\"a: 1\\nb: 2\\r\\nc: 3\\rd: 4\"\n```") {
		t.Fatal("mixed CRLF/CR input was invisible, normalized, or falsely presented as a literal Ktav fence")
	}
	layout, err := docbundle.NewLayout(graph.Languages, graph.DefaultLanguage)
	if err != nil {
		t.Fatal(err)
	}
	target, err := layout.EvidenceCasePath("en", "R-input", "selected-final-case")
	if err != nil {
		t.Fatal(err)
	}
	relative, err := specDocumentRelative("docs/gen/SPEC.en.md", target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(root, "[exact example evidence]("+relative+")") {
		t.Fatal("escaped source notation cannot be traced back to exact original bytes")
	}
	after, err := json.Marshal(rows["R-input"].outcomes[0].artifacts[1].Observations)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) || data.examples["selected-final-case"][0].RawInput.Bytes != base64.StdEncoding.EncodeToString([]byte(source)) {
		t.Fatal("control-character presentation normalized the machine comparison input")
	}
}
