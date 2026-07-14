package generator

import (
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
)

func TestBuildThinkingDocsCountMatchesSections(t *testing.T) {
	t.Parallel()
	sections := methodology.Sections.All()
	docs := BuildThinkingDocs()
	if len(docs) != len(sections) {
		t.Fatalf("BuildThinkingDocs: got %d docs, want %d sections", len(docs), len(sections))
	}
}

func TestBuildThinkingDocsSemanticCompleteness(t *testing.T) {
	t.Parallel()
	docs := BuildThinkingDocs()
	for _, s := range methodology.Sections.All() {
		key := topicSlug(s.Slug)
		doc, ok := docs[key]
		if !ok {
			t.Errorf("missing thinking doc for section %q (key %q)", s.Slug, key)
			continue
		}
		if doc == "" {
			t.Errorf("empty thinking doc for section %q", s.Slug)
			continue
		}
		if !strings.Contains(doc, s.Canon) {
			t.Errorf("thinking doc %q missing Canon text", s.Slug)
		}
		if !strings.Contains(doc, s.Narrative) {
			t.Errorf("thinking doc %q missing Narrative text", s.Slug)
		}
		if !strings.Contains(doc, s.Why) {
			t.Errorf("thinking doc %q missing Why text", s.Slug)
		}
		if !strings.Contains(doc, s.Slug) {
			t.Errorf("thinking doc %q missing Slug heading", s.Slug)
		}
	}
}

func TestBuildToolDocsCountMatchesTools(t *testing.T) {
	t.Parallel()
	tools := methodology.Tools.All()
	docs := BuildToolDocs(false)
	if len(docs) != len(tools) {
		t.Fatalf("BuildToolDocs: got %d docs, want %d tools", len(docs), len(tools))
	}
}

func TestBuildToolDocsSemanticCompleteness(t *testing.T) {
	t.Parallel()
	docs := BuildToolDocs(false)
	for _, tool := range methodology.Tools.All() {
		doc, ok := docs[tool.Command]
		if !ok {
			t.Errorf("missing tool doc for command %q", tool.Command)
			continue
		}
		if doc == "" {
			t.Errorf("empty tool doc for command %q", tool.Command)
			continue
		}
		if !strings.Contains(doc, tool.Canon) {
			t.Errorf("tool doc %q missing Canon text", tool.Command)
		}
		if !strings.Contains(doc, tool.Purpose) {
			t.Errorf("tool doc %q missing Purpose text", tool.Command)
		}
		if !strings.Contains(doc, tool.Command) {
			t.Errorf("tool doc %q missing Command heading", tool.Command)
		}
	}
}
