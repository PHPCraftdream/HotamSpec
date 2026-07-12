package query

import (
	"sort"
	"strings"

	"github.com/PHPCraftdream/HotamSpecGo/internal/ontology"
)

// summaryPreviewLen is how many runes of Claim are kept as the fallback
// preview when Summary is empty (roughly the "first ~80 characters" asked
// for by the req-list spec).
const summaryPreviewLen = 80

// ListItem is one row of `hotam req list` / `hotam req search`: just
// enough to recognize the requirement and decide whether to look closer.
type ListItem struct {
	ID      string `json:"id"`
	Summary string `json:"summary"`
	Status  string `json:"status"`
	Owner   string `json:"owner"`
}

// ListFilter narrows `hotam req list`. Empty fields are unfiltered.
type ListFilter struct {
	Status      string
	Owner       string
	Enforcement string
}

func requirementPreview(r ontology.Requirement) string {
	if r.Summary != "" {
		return r.Summary
	}
	runes := []rune(r.Claim)
	if len(runes) <= summaryPreviewLen {
		return r.Claim
	}
	return string(runes[:summaryPreviewLen]) + "…"
}

func toListItem(r ontology.Requirement) ListItem {
	return ListItem{
		ID:      r.ID,
		Summary: requirementPreview(r),
		Status:  r.Status,
		Owner:   r.Owner,
	}
}

// List returns compact rows for every Requirement matching filter, sorted
// by id for a stable, diffable listing.
func List(g *ontology.Graph, filter ListFilter) []ListItem {
	out := make([]ListItem, 0, len(g.Requirements))
	for _, r := range g.Requirements {
		if filter.Status != "" && r.Status != filter.Status {
			continue
		}
		if filter.Owner != "" && r.Owner != filter.Owner {
			continue
		}
		if filter.Enforcement != "" && r.Enforcement != filter.Enforcement {
			continue
		}
		out = append(out, toListItem(r))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// SearchResult is one ranked hit from Search.
type SearchResult struct {
	ID      string `json:"id"`
	Summary string `json:"summary"`
	Status  string `json:"status"`
	Owner   string `json:"owner"`
	Rank    int    `json:"rank"`
}

// Search ranks: match in id > claim > why (case-insensitive substring),
// each field checked independently so a requirement matching in both id
// and why still ranks by its best (lowest-numbered) hit. Ties break by id
// for determinism.
func Search(g *ontology.Graph, text string) []SearchResult {
	needle := strings.ToLower(strings.TrimSpace(text))
	if needle == "" {
		return nil
	}
	const (
		rankID = iota
		rankClaim
		rankWhy
		rankNone
	)
	var out []SearchResult
	for _, r := range g.Requirements {
		rank := rankNone
		if strings.Contains(strings.ToLower(r.ID), needle) {
			rank = rankID
		} else if strings.Contains(strings.ToLower(r.Claim), needle) {
			rank = rankClaim
		} else if strings.Contains(strings.ToLower(r.Why), needle) {
			rank = rankWhy
		} else if strings.Contains(strings.ToLower(r.Summary), needle) {
			rank = rankWhy
		}
		if rank == rankNone {
			continue
		}
		out = append(out, SearchResult{
			ID:      r.ID,
			Summary: requirementPreview(r),
			Status:  r.Status,
			Owner:   r.Owner,
			Rank:    rank,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Rank != out[j].Rank {
			return out[i].Rank < out[j].Rank
		}
		return out[i].ID < out[j].ID
	})
	return out
}
