package invariants

import (
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func TestCheckDomainManifestExistsAndImportable_Noop(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}}
	if vs := runCheck(t, "check_domain_manifest_exists_and_importable", g); len(vs) != 0 {
		t.Fatalf("check_domain_manifest_exists_and_importable is an honest no-op in the Go port; expected no violations, got %v", vs)
	}
}

func TestCheckDomainManifestIdMatchesDirname_Noop(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}}
	if vs := runCheck(t, "check_domain_manifest_id_matches_dirname", g); len(vs) != 0 {
		t.Fatalf("check_domain_manifest_id_matches_dirname is an honest no-op in the Go port; expected no violations, got %v", vs)
	}
}

func TestCheckDomainManifestDescriptionNonempty_Noop(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}}
	if vs := runCheck(t, "check_domain_manifest_description_nonempty", g); len(vs) != 0 {
		t.Fatalf("check_domain_manifest_description_nonempty is an honest no-op in the Go port; expected no violations, got %v", vs)
	}
}

func TestCheckDomainManifestGoalsNonempty_Noop(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}}
	if vs := runCheck(t, "check_domain_manifest_goals_nonempty", g); len(vs) != 0 {
		t.Fatalf("check_domain_manifest_goals_nonempty is an honest no-op in the Go port; expected no violations, got %v", vs)
	}
}

func TestCheckDomainManifestDirectorNonempty_Noop(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}}
	if vs := runCheck(t, "check_domain_manifest_director_nonempty", g); len(vs) != 0 {
		t.Fatalf("check_domain_manifest_director_nonempty is an honest no-op in the Go port; expected no violations, got %v", vs)
	}
}

func TestCheckDomainManifestValid_DelegatesToNoopSubchecks(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}}
	if vs := runCheck(t, "check_domain_manifest_valid", g); len(vs) != 0 {
		t.Fatalf("check_domain_manifest_valid delegates to five no-op sub-checks; expected no violations, got %v", vs)
	}
}

func TestCheckDomainDirectorExists_Noop(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}}
	if vs := runCheck(t, "check_domain_director_exists", g); len(vs) != 0 {
		t.Fatalf("check_domain_director_exists is an honest no-op in the Go port; expected no violations, got %v", vs)
	}
}

func TestCheckAgentHasAgentsSubdir_Noop(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}}
	if vs := runCheck(t, "check_agent_has_agents_subdir", g); len(vs) != 0 {
		t.Fatalf("check_agent_has_agents_subdir is an honest no-op in the Go port; expected no violations, got %v", vs)
	}
}

func TestCheckAgentHasDocsSubdir_Noop(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}}
	if vs := runCheck(t, "check_agent_has_docs_subdir", g); len(vs) != 0 {
		t.Fatalf("check_agent_has_docs_subdir is an honest no-op in the Go port; expected no violations, got %v", vs)
	}
}

func TestCheckAgentHasToolsSubdir_Noop(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}}
	if vs := runCheck(t, "check_agent_has_tools_subdir", g); len(vs) != 0 {
		t.Fatalf("check_agent_has_tools_subdir is an honest no-op in the Go port; expected no violations, got %v", vs)
	}
}
