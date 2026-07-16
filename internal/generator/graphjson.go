package generator

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func copyStrings(src []string) []string {
	out := make([]string, len(src))
	copy(out, src)
	return out
}

func BuildGraphJSON(g *ontology.Graph) (string, error) {
	reqs := NarrativeOrder(g.Requirements, func(r ontology.Requirement) int { return r.DeclOrder })
	conflicts := NarrativeOrder(g.Conflicts, func(c ontology.Conflict) int { return c.DeclOrder })
	assumptions := NarrativeOrder(g.Assumptions, func(a ontology.Assumption) int { return a.DeclOrder })
	stakeholders := NarrativeOrder(g.Stakeholders, func(s ontology.Stakeholder) int { return s.DeclOrder })

	reqMaps := make([]map[string]any, 0, len(reqs))
	for _, r := range reqs {
		history := make([]map[string]any, 0, len(r.History))
		for _, h := range r.History {
			history = append(history, map[string]any{
				"at":         h.At,
				"decided_by": h.DecidedBy,
				"summary":    h.Summary,
			})
		}
		reqMaps = append(reqMaps, map[string]any{
			"assumptions":      copyStrings(r.Assumptions),
			"claim":            r.Claim,
			"enforced_by":      copyStrings(r.EnforcedBy),
			"enforcement":      r.Enforcement,
			"evidence":         copyStrings(r.Evidence),
			"history":          history,
			"id":               r.ID,
			"implemented_by":   copyStrings(r.ImplementedBy),
			"last_reviewed_at": r.LastReviewedAt,
			"owner":            r.Owner,
			"review_after":     r.ReviewAfter,
			"source_refs":      copyStrings(r.SourceRefs),
			"status":           r.Status,
			"verified_by":      copyStrings(r.VerifiedBy),
			"why":              r.Why,
		})
	}

	conflictMaps := make([]map[string]any, 0, len(conflicts))
	for _, c := range conflicts {
		conflictMaps = append(conflictMaps, map[string]any{
			"axis":      c.Axis,
			"context":   c.Context,
			"id":        c.ID,
			"lifecycle": c.Lifecycle,
			"members":   copyStrings(c.Members),
			"steward":   c.Steward,
		})
	}

	assumptionMaps := make([]map[string]any, 0, len(assumptions))
	for _, a := range assumptions {
		assumptionMaps = append(assumptionMaps, map[string]any{
			"id":        a.ID,
			"owner":     a.Owner,
			"statement": a.Statement,
			"status":    a.Status,
		})
	}

	stakeholderMaps := make([]map[string]any, 0, len(stakeholders))
	for _, s := range stakeholders {
		stakeholderMaps = append(stakeholderMaps, map[string]any{
			"domain": s.Domain,
			"id":     s.ID,
			"name":   s.Name,
		})
	}

	var nodeIDs []string
	nodeIDs = append(nodeIDs, ids(reqs)...)
	nodeIDs = append(nodeIDs, conflictIDs(conflicts)...)
	nodeIDs = append(nodeIDs, assumptionIDs(assumptions)...)
	nodeIDs = append(nodeIDs, stakeholderIDs(stakeholders)...)
	sort.Strings(nodeIDs)

	payload := map[string]any{
		"generated_from": "domains/<active>/graph.json",
		"note":           "READ-ONLY generated snapshot. Source of truth is the domain's graph.json; edit the graph only via `hotam apply-proposal` (R-no-hand-edit-graph, R-per-node-json-store REJECTED).",
		"schema_version": ontology.CurrentSchemaVersion,
		"requirements":   reqMaps,
		"conflicts":      conflictMaps,
		"assumptions":    assumptionMaps,
		"stakeholders":   stakeholderMaps,
		"node_ids":       nodeIDs,
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(payload); err != nil {
		return "", err
	}
	return strings.TrimRight(buf.String(), " \t\r\n") + "\n", nil
}

func ids(reqs []ontology.Requirement) []string {
	out := make([]string, len(reqs))
	for i, r := range reqs {
		out[i] = r.ID
	}
	return out
}

func conflictIDs(conflicts []ontology.Conflict) []string {
	out := make([]string, len(conflicts))
	for i, c := range conflicts {
		out[i] = c.ID
	}
	return out
}

func assumptionIDs(assumptions []ontology.Assumption) []string {
	out := make([]string, len(assumptions))
	for i, a := range assumptions {
		out[i] = a.ID
	}
	return out
}

func stakeholderIDs(stakeholders []ontology.Stakeholder) []string {
	out := make([]string, len(stakeholders))
	for i, s := range stakeholders {
		out[i] = s.ID
	}
	return out
}
