package generator

import "testing"

// TestModuleDocstringResolvesEveryModule guards the TaskList P2-3 merge:
// internal/methodology is now the only source of module-doc prose (the old
// internal/generator/module_docs_data.go hand-maintained duplicate is gone).
// Every module key in ModuleOrder must resolve to non-empty text, either via
// methodology.Overview ("__init__") or a methodology.Sections entry (every
// other key, through moduleSectionSlug).
func TestModuleDocstringResolvesEveryModule(t *testing.T) {
	t.Parallel()
	for _, me := range ModuleOrder {
		doc := ModuleDocstring(me.Mod)
		if doc == "" {
			t.Errorf("ModuleDocstring(%q) is empty", me.Mod)
		}
	}
}

// TestModuleSectionSlugNoDuplicates guards against two legacy module keys
// silently resolving to the same methodology.Sections entry, which would
// reintroduce a form of duplication (two "different" module docs rendering
// identical content) even after the module_docs_data.go merge.
func TestModuleSectionSlugNoDuplicates(t *testing.T) {
	t.Parallel()
	seenSlug := make(map[string]string)
	for mod, slug := range moduleSectionSlug {
		if prevMod, ok := seenSlug[slug]; ok {
			t.Errorf("slug %q claimed by both module %q and %q", slug, prevMod, mod)
		}
		seenSlug[slug] = mod
	}
}
