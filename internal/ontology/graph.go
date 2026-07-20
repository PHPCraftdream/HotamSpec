package ontology

// CurrentSchemaVersion is the schema_version written into every graph.json by
// the loader's canonical writer (loader.marshalCanonical / WriteGraph) and the
// maximum version the loader accepts. graph.json is a single-repo internal
// format — a plain integer, not semver.
//
// Bump this ONLY for a STRUCTURAL format change (a new top-level field, a
// changed field shape) that requires a real migration step. When you bump it:
//  1. Add a migration case to the version switch in loader.LoadGraph.
//  2. Update any byte-identity / round-trip fixtures that hardcode the old
//     byte layout.
//
// Do NOT bump for content changes to domain graphs — only for format changes.
//
// v3 added the additive OPTIONAL Requirement fields implemented_by and
// verified_by (mirrors the v1→v2 bump for blocked_on).
const CurrentSchemaVersion = 3

type Graph struct {
	// SchemaVersion is the graph.json format version this graph was written
	// with. Populated by the loader (LoadGraph) and stamped by the writer
	// (marshalCanonical); always CurrentSchemaVersion on round-trip. Serialized
	// as json:"schema_version" so it round-trips through graph.json.
	SchemaVersion int              `json:"schema_version"`
	Axes          []Axis           `json:"axes"`
	Stakeholders  []Stakeholder    `json:"stakeholders"`
	Assumptions   []Assumption     `json:"assumptions"`
	Requirements  []Requirement    `json:"requirements"`
	Conflicts     []Conflict       `json:"conflicts"`
	Operators     []Operator       `json:"operators"`
	Processes     []Process        `json:"processes"`
	Goals         []Goal           `json:"goals"`
	EntityTypes   []EntityType     `json:"entity_types"`
	Entities      []EntityInstance `json:"entities"`
	SelfHosting   bool             `json:"self_hosting"`
	// DomainDir is the filesystem path of the domain directory this graph was
	// loaded from (the resolved --domain path, i.e. the parent dir of
	// graph.json). Populated by the loader at LoadGraph time; deliberately
	// unserialized (json:"-") so it never round-trips through graph.json or
	// DisallowUnknownFields. Lets domain-scoped checks resolve files relative
	// to the graph actually being checked instead of a CWD-based project-root
	// search, which resolves THIS framework's own root for external domains.
	DomainDir string `json:"-"`
	// Discipline is the domain's manifest.json "discipline" field
	// (loader.ResolveDiscipline), populated by the loader at LoadGraph time
	// exactly like SelfHosting above -- deliberately unserialized (json:"-"):
	// discipline lives in manifest.json, not graph.json, so it never
	// round-trips through this struct's own JSON encoding. "" (the zero
	// value) is the long-standing soft-discipline default; loader.DisciplineFull
	// ("full") opts a domain into check_settled_requires_scenario's real gate
	// (PLAN-scenario-generated-spec.md §2 D4, task W2.1). A graph built
	// in-memory by a test fixture (never through loader.LoadGraph) leaves
	// this "" -- an honest no-op, the same convention DomainDir/SelfHosting
	// already establish for synthetic graphs.
	Discipline string `json:"-"`
	// ManifestExists, ParentDeclared, and Parent carry the domain's
	// manifest.json "parent" field (loader.ResolveParent), populated by the
	// loader at LoadGraph time exactly like Discipline above -- deliberately
	// unserialized (json:"-"): parent lives in manifest.json, not graph.json,
	// so it never round-trips through this struct's own JSON encoding.
	//
	// D6 (PLAN-scenario-generated-spec.md §2 D6) makes "parent" MANDATORY for
	// any domain that HAS a manifest.json -- but a domain with NO
	// manifest.json at all (the shape of countless synthetic test-fixture
	// graphs across this codebase that build an ontology.Graph{DomainDir:
	// someTempDir, ...} directly, in Go code, without ever running `hotam
	// init` or writing a manifest.json) never had the chance to declare a
	// parent, so ManifestExists distinguishes that HONEST NO-OP case (mirrors
	// every sibling resolver's own missing-manifest-is-the-soft-default
	// convention) from the real check_project_parent_declared VIOLATION case
	// (a manifest exists yet never declared parent):
	//
	//   - ManifestExists == false: no manifest.json next to graph.json at
	//     all. check_project_parent_declared is a no-op -- there is no
	//     manifest for a "parent" key to be missing FROM.
	//   - ManifestExists == true, ParentDeclared == false: a manifest.json
	//     exists but its "parent" key is absent (or the manifest is malformed,
	//     or the value is a non-string/non-null type). THIS is the violation
	//     D6 exists to catch.
	//   - ManifestExists == true, ParentDeclared == true, Parent == "": the
	//     key is present with JSON null (or an explicit empty string) -- the
	//     EXPLICIT root-domain declaration ("I have considered this and I
	//     have no parent"). No violation.
	//   - ManifestExists == true, ParentDeclared == true, Parent != "": the
	//     key is present with a non-empty string naming the parent domain --
	//     a valid child-domain declaration. No violation.
	//
	// A graph built in-memory by a test fixture (never through
	// loader.LoadGraph) leaves all three at the zero value
	// (ManifestExists=false, ParentDeclared=false, Parent="") -- the honest
	// no-op case above, the same convention DomainDir/SelfHosting already
	// establish for synthetic graphs.
	ManifestExists bool   `json:"-"`
	ParentDeclared bool   `json:"-"`
	Parent         string `json:"-"`
}

func (g *Graph) IsEmpty() bool {
	return len(g.Axes) == 0 &&
		len(g.Stakeholders) == 0 &&
		len(g.Assumptions) == 0 &&
		len(g.Requirements) == 0 &&
		len(g.Conflicts) == 0 &&
		len(g.Operators) == 0 &&
		len(g.Processes) == 0 &&
		len(g.Goals) == 0 &&
		len(g.EntityTypes) == 0 &&
		len(g.Entities) == 0
}

func AxisSlugs(g *Graph) map[string]struct{} {
	out := make(map[string]struct{}, len(g.Axes))
	for _, a := range g.Axes {
		out[a.Slug] = struct{}{}
	}
	return out
}

func StakeholderIDs(g *Graph) map[string]struct{} {
	out := make(map[string]struct{}, len(g.Stakeholders))
	for _, s := range g.Stakeholders {
		out[s.ID] = struct{}{}
	}
	return out
}

func AssumptionIDs(g *Graph) map[string]struct{} {
	out := make(map[string]struct{}, len(g.Assumptions))
	for _, a := range g.Assumptions {
		out[a.ID] = struct{}{}
	}
	return out
}

func RequirementIDs(g *Graph) map[string]struct{} {
	out := make(map[string]struct{}, len(g.Requirements))
	for _, r := range g.Requirements {
		out[r.ID] = struct{}{}
	}
	return out
}

func OperatorIDs(g *Graph) map[string]struct{} {
	out := make(map[string]struct{}, len(g.Operators))
	for _, op := range g.Operators {
		out[op.ID] = struct{}{}
	}
	return out
}

func ProcessIDs(g *Graph) map[string]struct{} {
	out := make(map[string]struct{}, len(g.Processes))
	for _, p := range g.Processes {
		out[p.ID] = struct{}{}
	}
	return out
}

func GoalIDs(g *Graph) map[string]struct{} {
	out := make(map[string]struct{}, len(g.Goals))
	for _, go_ := range g.Goals {
		out[go_.ID] = struct{}{}
	}
	return out
}

func EntityTypeSlugs(g *Graph) map[string]struct{} {
	out := make(map[string]struct{}, len(g.EntityTypes))
	for _, et := range g.EntityTypes {
		out[et.Slug] = struct{}{}
	}
	return out
}

func EntityIDs(g *Graph) map[string]struct{} {
	out := make(map[string]struct{}, len(g.Entities))
	for _, e := range g.Entities {
		out[e.ID] = struct{}{}
	}
	return out
}

func RequirementByID(g *Graph, rid string) (Requirement, bool) {
	for _, r := range g.Requirements {
		if r.ID == rid {
			return r, true
		}
	}
	return Requirement{}, false
}

func AssumptionByID(g *Graph, aid string) (Assumption, bool) {
	for _, a := range g.Assumptions {
		if a.ID == aid {
			return a, true
		}
	}
	return Assumption{}, false
}
