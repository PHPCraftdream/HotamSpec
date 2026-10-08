package gate

import (
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func TestNormativeIncludeRefusesUndeclaredSourceConstant(t *testing.T) {
	rules := ontology.LocalizedText{}
	fragments := ontology.LocalizedText{"en": "Shared English.", "ru": "Общий русский текст.", "zh": "共同中文文本。"}
	for _, language := range []string{"en", "ru", "zh"} {
		rules[language] = "Rule.\n\ninclude: spec/model/text.go:Shared\n\ninclude: spec/model/text.go:Missing"
	}
	index, err := NewAtomSourceIndexForGraph(writeNormativeIncludeFixture(t, rules, fragments))
	if err == nil || index != nil {
		t.Fatalf("undeclared normative text returned a usable source snapshot: index=%v err=%v", index, err)
	}
	for _, context := range []string{"rule.go:", "Box.Rule language \"", "spec/model/text.go:Missing", "documented constant in the source snapshot"} {
		if !strings.Contains(err.Error(), context) {
			t.Errorf("missing-include diagnostic lost authored context %q: %v", context, err)
		}
	}
}

func TestNormativeIncludePreservesFencedKtavAcrossLanguages(t *testing.T) {
	tests := []struct {
		name string
		code string
	}{
		{"backticks", "```ktav\ninclude: value\n```"},
		{"tildes", "~~~ktav\ninclude: value\n~~~"},
		{"shorter_backticks_are_content", "````ktav\n```\ninclude: value\n````"},
		{"shorter_tildes_are_content", "~~~~ktav\n~~~\ninclude: value\n~~~~"},
		{"inline_backticks_are_content", "```ktav\n```inline code```\ninclude: value\n```"},
		{"different_marker_is_content", "~~~ktav\n```\ninclude: value\n~~~"},
		{"four_space_closer_is_content", "```ktav\n    ```\ninclude: value\n```"},
		{"tab_indented_closer_is_content", "~~~ktav\n\t~~~\ninclude: value\n~~~"},
		{"indented_opener_and_longer_closer", "   ~~~ktav\ninclude: value\n  ~~~~~ \t"},
		{"empty_backtick_code", "```ktav\n```"},
		{"empty_tilde_code", "~~~ktav\n~~~"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rules := ontology.LocalizedText{
				"en": "The shared rule applies.\n\ninclude: spec/model/text.go:Shared",
				"ru": "Применяется общее правило.\n\ninclude: spec/model/text.go:Shared",
				"zh": "适用共同规则。\n\ninclude: spec/model/text.go:Shared",
			}
			fragments := ontology.LocalizedText{
				"en": "An authored Ktav example.\n" + test.code + "\nThe example ends here.",
				"ru": "Авторский пример Ktav.\n" + test.code + "\nПример завершён.",
				"zh": "作者提供的 Ktav 示例。\n" + test.code + "\n示例结束。",
			}
			index, err := NewAtomSourceIndexForGraph(writeNormativeIncludeFixture(t, rules, fragments))
			if err != nil {
				t.Fatal(err)
			}
			source, err := index.ResolveLink("spec/model/rule.go:Box.Rule")
			if err != nil {
				t.Fatal(err)
			}
			for _, language := range []string{"en", "ru", "zh"} {
				want := strings.Replace(rules[language], "include: spec/model/text.go:Shared", fragments[language], 1)
				if got := source.Phrases[language]; got != want {
					t.Errorf("%s: fenced source changed:\ngot  %q\nwant %q", language, got, want)
				}
			}
			if source.Phrase != source.Phrases["ru"] {
				t.Fatalf("default-language phrase differs from its expanded document: %q", source.Phrase)
			}
		})
	}
}

func TestNormativeIncludeUsesMarkdownFenceBoundaries(t *testing.T) {
	tests := []struct {
		name   string
		before string
	}{
		{"long_fence", "````ktav\n```\ninclude: value\n````"},
		{"inline_backticks", "```ktav\n```inline code```\ninclude: value\n```"},
		{"tilde_fence", "~~~ktav\ninclude: value\n~~~~ \t"},
		{"empty_fence", "~~~ktav\n~~~"},
		{"invalid_backtick_info_is_not_an_opener", "```inline code```"},
		{"four_space_indent_is_not_an_opener", "    ```ktav"},
		{"two_markers_are_not_an_opener", "~~ktav"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rules := ontology.LocalizedText{}
			fragments := ontology.LocalizedText{"en": "Shared English.", "ru": "Общий русский текст.", "zh": "共同中文文本。"}
			for _, language := range []string{"en", "ru", "zh"} {
				rules[language] = "Rule.\n\n" + test.before + "\n\ninclude: spec/model/text.go:Shared"
			}
			index, err := NewAtomSourceIndexForGraph(writeNormativeIncludeFixture(t, rules, fragments))
			if err != nil {
				t.Fatal(err)
			}
			source, err := index.ResolveLink("spec/model/rule.go:Box.Rule")
			if err != nil {
				t.Fatal(err)
			}
			for _, language := range []string{"en", "ru", "zh"} {
				want := "Rule.\n\n" + test.before + "\n\n" + fragments[language]
				if got := source.Phrases[language]; got != want {
					t.Errorf("%s: fence boundary changed include expansion:\ngot  %q\nwant %q", language, got, want)
				}
			}
		})
	}
}

func TestNormativeIncludeOpenFenceKeepsFollowingSourceLiteral(t *testing.T) {
	rules := ontology.LocalizedText{}
	fragments := ontology.LocalizedText{}
	for _, language := range []string{"en", "ru", "zh"} {
		rules[language] = "Rule.\n\ninclude: spec/model/text.go:Shared\n\ninclude: spec/model/text.go:Missing"
		fragments[language] = "Shared example.\n~~~ktav\ninclude: value"
	}
	index, err := NewAtomSourceIndexForGraph(writeNormativeIncludeFixture(t, rules, fragments))
	if err != nil {
		t.Fatal(err)
	}
	source, err := index.ResolveLink("spec/model/rule.go:Box.Rule")
	if err != nil {
		t.Fatal(err)
	}
	for _, language := range []string{"en", "ru", "zh"} {
		want := "Rule.\n\nShared example.\n~~~ktav\ninclude: value\n\ninclude: spec/model/text.go:Missing"
		if got := source.Phrases[language]; got != want {
			t.Errorf("%s: an unmatched inserted fence did not stay literal through EOF:\ngot  %q\nwant %q", language, got, want)
		}
	}
}

func TestNormativeIncludeRejectsActiveNestedDirective(t *testing.T) {
	tests := []struct {
		name   string
		before string
	}{
		{"plain", "Nested rule."},
		{"after_backtick_close", "```ktav\ninclude: value\n```"},
		{"after_tilde_close", "~~~ktav\ninclude: value\n~~~~"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rules := ontology.LocalizedText{}
			fragments := ontology.LocalizedText{}
			for _, language := range []string{"en", "ru", "zh"} {
				rules[language] = "Rule.\n\ninclude: spec/model/text.go:Shared"
				fragments[language] = test.before + "\n\ninclude: spec/model/text.go:Nested"
			}
			index, err := NewAtomSourceIndexForGraph(writeNormativeIncludeFixture(t, rules, fragments))
			if err == nil || index != nil {
				t.Fatalf("active nested directive returned a usable document: index=%v err=%v", index, err)
			}
			for _, context := range []string{"rule.go:", "Box.Rule language \"", "spec/model/text.go:Shared", "must not contain nested includes", "spec/model/text.go:Nested"} {
				if !strings.Contains(err.Error(), context) {
					t.Errorf("nested-include diagnostic lost authored context %q: %v", context, err)
				}
			}
		})
	}
}

func TestNormativeIncludeMissingLanguageFailsWithConstantContext(t *testing.T) {
	rules := ontology.LocalizedText{}
	for _, language := range []string{"en", "ru", "zh"} {
		rules[language] = "Rule.\n\ninclude: spec/model/text.go:Shared"
	}
	fragments := ontology.LocalizedText{"en": "English shared text.", "zh": "共同中文文本。"}
	index, err := NewAtomSourceIndexForGraph(writeNormativeIncludeFixture(t, rules, fragments))
	if err == nil || index != nil {
		t.Fatalf("missing shared translation returned a usable document: index=%v err=%v", index, err)
	}
	for _, context := range []string{"text.go:", "constant Shared", "language \"ru\"", "missing language block"} {
		if !strings.Contains(err.Error(), context) {
			t.Errorf("missing-language diagnostic lost authored context %q: %v", context, err)
		}
	}
}

func writeNormativeIncludeFixture(t *testing.T, rules, fragments ontology.LocalizedText) *ontology.Graph {
	t.Helper()
	ruleSource := "package model\n\ntype Box struct{}\n"
	fragmentSource := "package model\n\ntype Text string\n"
	for _, language := range []string{"en", "ru", "zh"} {
		if text, ok := rules[language]; ok {
			ruleSource += "// >>>>> lang=" + language + "\n"
			for _, line := range strings.Split(text, "\n") {
				ruleSource += "// " + line + "\n"
			}
		}
		if text, ok := fragments[language]; ok {
			fragmentSource += "// >>>>> lang=" + language + "\n"
			for _, line := range strings.Split(text, "\n") {
				fragmentSource += "// " + line + "\n"
			}
		}
	}
	ruleSource += "func (Box) Rule() bool { return true }\n"
	fragmentSource += "const Shared Text = \"shared\"\n"
	root := writeAtomPipelineFixture(t, map[string]string{
		"manifest.json":      `{"self_hosting":false}`,
		"spec/model/rule.go": ruleSource,
		"spec/model/text.go": fragmentSource,
	})
	return &ontology.Graph{DomainDir: root, Languages: []string{"en", "ru", "zh"}, DefaultLanguage: "ru"}
}
