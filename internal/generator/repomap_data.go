package generator

import "strings"

// repoMapFrameworkAndToolsContent holds the domain-independent "Framework
// body" and "Tools" sections of REPO-MAP.md — these describe internal/ and
// cmd/hotam/, which are the same regardless of which domain the doc is
// generated for.
var repoMapFrameworkAndToolsContent = strings.Join([]string{
	"### Repository Map",
	"",
	"**Framework body** (`internal/ontology/`)",
	"",
	"- `internal/ontology/assumption.go` — a claim with its OWN lifecycle (the root of context drift).",
	"- `internal/ontology/axis.go` — controlled vocabulary of tension dimensions.",
	"- `internal/ontology/conflict.go` — the first-class connector NODE, a held property of the discipline (not its headline; J1, commit b2c58c8).",
	"- `internal/ontology/entity.go` — domain-declared business concept with its own lifecycle.",
	"- `internal/ontology/graph.go` — the tension graph store and its traversal helpers.",
	"- `internal/ontology/graph_traversal.go` — traversal helpers over the tension graph.",
	"- `internal/ontology/lifecycle.go` — the generic state-machine value-type (framework keystone).",
	"- `internal/ontology/operator.go` — the acting facet of a Stakeholder (M20: NEW TYPE).",
	"- `internal/ontology/process.go` — opt-in behavioral aspect (M12).",
	"- `internal/ontology/requirement.go` — a business requirement as a node in the tension graph.",
	"- `internal/ontology/signoff.go` — the frozen provenance record of a human steward decision.",
	"- `internal/ontology/stakeholder.go` — who owns requirements and stewards conflicts.",
	"- `internal/invariants/` — structural form of the tension graph (the check_* layer).",
	"- `internal/diagnose/` — the operator's next-action diagnosis (what_now equivalent).",
	"- `internal/proposal/` — structured operator-→-steward change proposals + the mechanical apply writer.",
	"- `internal/loader/` — reads a domain's graph.json into an in-memory Graph.",
	"- `internal/generator/` — regenerates docs/gen/ from the graph (the gen-spec engine).",
	"- `internal/gate/` — T1 tiered LAND gate: select a targeted test subset instead of the full suite.",
	"- `internal/paths/` — project-root resolution for the consumer's data directory.",
	"",
	"**Tools** (`cmd/hotam/`, dispatched by `hotam <command>` / `go run ./cmd/hotam <command>`)",
	"",
	"- `cmd/hotam/apply_proposal.go` — mechanical writer for steward-approved JSON proposals.  →  R-tool-apply-proposal",
	"- `cmd/hotam/gate_cmd.go` — T1 tiered LAND gate: select a targeted enforcer subset instead of the full suite.  →  R-tool-gate",
	"- `cmd/hotam/gen_spec.go` — regenerates docs/gen/ from the executable model (methodology + graph), making drift structurally impossible.  →  R-tool-gen-spec",
	"- `cmd/hotam/all_violations.go` — print all invariant violations; exit 1 if any.",
	"- `cmd/hotam/what_now.go` — derives the prioritized next correct action from any graph state, making being-lost structurally impossible.  →  R-tool-what-now",
	"",
	"Not yet ported to the Go CLI (Python-era tools with no `hotam` subcommand yet): attention, attention_hook,",
	"audit_atomicity, audit_tensions, claude_md_diff_watch, closure, confront, context, context_producer,",
	"create_agent, create_axis, create_domain, create_entity_type, delegate, emit_cipher, gate_status,",
	"hotam_req, invoke_agent, land, mark_revisit_evaluated, review, setup_context_hook, setup_hooks,",
	"spawn_agent, spawn_log_isolation_status, ticket_comment, ticket_create, ticket_edit, ticket_list,",
	"ticket_move, ticket_show, update_baseline.",
}, "\n")

// domainGraphPyRole returns the one-line role text for domains/<name>/graph.json
// in the "Domain content" section. The Go port has no per-domain graph.py
// source file to introspect (domains carry graph.json instead), so the
// known domains' role text is captured here directly; any other domain
// falls back to generic phrasing.
func domainGraphPyRole(domainName string) string {
	switch domainName {
	case "hotam-spec-self":
		return "Hotam-Spec modeling itself — the meta-domain (the framework's own design)."
	default:
		return "content graph of domain '" + domainName + "'."
	}
}

// GenDocEntry describes one file written into a domain's docs/gen/ during
// this run, for the REPO-MAP.md "Generated docs" section.
type GenDocEntry struct {
	Filename string
	Content  string
}

// mdTitle extracts the first H1/H2 heading from generated Markdown content as
// a short title — mirrors Python's _md_title(): take the first line starting
// with '#', drop everything up to (and including) the last " — ", then strip
// a trailing " (...)" suffix.
func mdTitle(content string) string {
	for _, line := range strings.Split(content, "\n") {
		stripped := strings.TrimSpace(strings.TrimLeft(line, "#"))
		if strings.HasPrefix(line, "#") && stripped != "" {
			parts := strings.Split(stripped, " — ")
			title := parts[len(parts)-1]
			if idx := strings.Index(title, " ("); idx >= 0 {
				title = title[:idx]
			}
			return strings.TrimSpace(title)
		}
	}
	return ""
}
