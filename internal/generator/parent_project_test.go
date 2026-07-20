package generator

import (
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// TestRenderParentProjectBlock_RootNull proves the GREEN root case of the
// tri-state (PLAN-scenario-generated-spec.md §2 D6): g.ParentDeclared == true
// and g.Parent == "" (manifest.json "parent": null) renders the explicit
// root-domain declaration, not a child line and not the not-declared
// placeholder.
func TestRenderParentProjectBlock_RootNull(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{ParentDeclared: true, Parent: ""}
	out := RenderParentProjectBlock(g)
	if !strings.Contains(out, "### Parent project") {
		t.Errorf("missing section heading, got:\n%s", out)
	}
	if !strings.Contains(out, "This is a root domain (no parent).") {
		t.Errorf("root declaration not rendered for parent:null, got:\n%s", out)
	}
	if strings.Contains(out, "Parent: `") {
		t.Errorf("root case must not render a child Parent line, got:\n%s", out)
	}
	if strings.Contains(out, "not yet declared") {
		t.Errorf("root case must not render the not-declared placeholder, got:\n%s", out)
	}
}

// TestRenderParentProjectBlock_ChildString proves the GREEN child case:
// g.ParentDeclared == true and g.Parent == "<name>" renders "Parent: `<name>`.".
func TestRenderParentProjectBlock_ChildString(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{ParentDeclared: true, Parent: "hotam-spec-self"}
	out := RenderParentProjectBlock(g)
	if !strings.Contains(out, "Parent: `hotam-spec-self`.") {
		t.Errorf("child declaration not rendered for parent:\"hotam-spec-self\", got:\n%s", out)
	}
	if strings.Contains(out, "root domain") {
		t.Errorf("child case must not render the root line, got:\n%s", out)
	}
	if strings.Contains(out, "not yet declared") {
		t.Errorf("child case must not render the not-declared placeholder, got:\n%s", out)
	}
}

// TestRenderParentProjectBlock_AbsentPlaceholder proves the (rare, violation-
// generating) absent case renders an HONEST placeholder naming the invariant,
// not empty output and not a misleading root/child line. In a live domain this
// state already fires a check_project_parent_declared violation; the renderer
// points a crystal reader at the actionable fix rather than leaving a blank
// section.
func TestRenderParentProjectBlock_AbsentPlaceholder(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{ParentDeclared: false, Parent: ""} // key absent
	out := RenderParentProjectBlock(g)
	if !strings.Contains(out, "### Parent project") {
		t.Errorf("missing section heading even in the absent case, got:\n%s", out)
	}
	if !strings.Contains(out, "check_project_parent_declared") {
		t.Errorf("absent case must name the invariant in its placeholder, got:\n%s", out)
	}
	if strings.Contains(out, "root domain") {
		t.Errorf("absent case must not masquerade as a root declaration, got:\n%s", out)
	}
	if strings.Contains(out, "Parent: `") {
		t.Errorf("absent case must not render a child Parent line, got:\n%s", out)
	}
}

// TestRenderClaudeMDFromTemplate_ParentProjectBlockPresent proves the new
// PARENT-PROJECT sentinel block is wired into the BUSINESS bucket of the full
// root crystal, immediately after DOMAIN-MAP (it is a manifest-derived section
// like DOMAIN-MAP, not a graph-derived one).
func TestRenderClaudeMDFromTemplate_ParentProjectBlockPresent(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	repoRoot := t.TempDir()
	out := RenderClaudeMDFromTemplate(g, "hotam-spec-self", repoRoot, 4200, nil, "2026-07-12", false)
	begin := "<!-- PARENT-PROJECT:BEGIN -->"
	end := "<!-- PARENT-PROJECT:END -->"
	bi, ei := strings.Index(out, begin), strings.Index(out, end)
	if bi == -1 || ei == -1 || ei < bi {
		t.Fatalf("PARENT-PROJECT sentinels missing or mis-ordered (begin=%d end=%d)", bi, ei)
	}
	// the block must sit AFTER DOMAIN-MAP's END sentinel and BEFORE CONSTITUTION's BEGIN.
	domainMapEnd := strings.Index(out, "<!-- DOMAIN-MAP:END -->")
	constitutionBegin := strings.Index(out, "<!-- CONSTITUTION:BEGIN -->")
	if domainMapEnd == -1 || constitutionBegin == -1 {
		t.Fatalf("DOMAIN-MAP/CONSTITUTION sentinels missing; cannot assert ordering")
	}
	if bi < domainMapEnd {
		t.Errorf("PARENT-PROJECT BEGIN (%d) must come after DOMAIN-MAP END (%d)", bi, domainMapEnd)
	}
	if ei > constitutionBegin {
		t.Errorf("PARENT-PROJECT END (%d) must come before CONSTITUTION BEGIN (%d)", ei, constitutionBegin)
	}
}
