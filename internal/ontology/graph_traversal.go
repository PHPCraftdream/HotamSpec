package ontology

import (
	"fmt"
	"sort"
	"strings"
)

const GenericAssumptionThreshold = 8

type LatentSuspect struct {
	Left  string `json:"left"`
	Right string `json:"right"`
	Hint  string `json:"hint"`
}

type LatentCluster struct {
	Assumptions  []string        `json:"assumptions"`
	Requirements []string        `json:"requirements"`
	Pairs        []LatentSuspect `json:"pairs"`
}

func pairKey(a, b string) string {
	if a > b {
		a, b = b, a
	}
	return a + "\x00" + b
}

func ReplacesMap(g *Graph) map[string][]string {
	tmp := map[string][]string{}
	for _, r := range g.Requirements {
		for _, rel := range r.Relations {
			if rel.Kind == "replaces" {
				tmp[rel.Target] = append(tmp[rel.Target], r.ID)
			}
		}
	}
	out := make(map[string][]string, len(tmp))
	for k, v := range tmp {
		out[k] = v
	}
	return out
}

func RequirementsOnAssumption(g *Graph, aid string) []Requirement {
	var out []Requirement
	for _, r := range g.Requirements {
		for _, a := range r.Assumptions {
			if a == aid {
				out = append(out, r)
				break
			}
		}
	}
	return out
}

func ConflictsOnAssumption(g *Graph, aid string) []Conflict {
	var out []Conflict
	for _, c := range g.Conflicts {
		if c.SharedAssumption != nil && *c.SharedAssumption == aid {
			out = append(out, c)
		}
	}
	return out
}

func DeadAssumptions(g *Graph) []Assumption {
	var out []Assumption
	for _, a := range g.Assumptions {
		if a.Status == AssumptionDEAD {
			out = append(out, a)
		}
	}
	return out
}

func UncertainAssumptions(g *Graph) []Assumption {
	var out []Assumption
	for _, a := range g.Assumptions {
		if a.Status == AssumptionUNCERTAIN {
			out = append(out, a)
		}
	}
	return out
}

func ConflictsByAxis(g *Graph) map[string][]Conflict {
	out := map[string][]Conflict{}
	for _, c := range g.Conflicts {
		out[c.Axis] = append(out[c.Axis], c)
	}
	return out
}

func MembersPairSet(g *Graph) map[string]struct{} {
	pairs := map[string]struct{}{}
	for _, c := range g.Conflicts {
		ms := c.Members
		for i := 0; i < len(ms); i++ {
			for j := i + 1; j < len(ms); j++ {
				pairs[pairKey(ms[i], ms[j])] = struct{}{}
			}
		}
	}
	return pairs
}

func AssumptionReferenceCounts(g *Graph) map[string]int {
	counts := map[string]int{}
	for _, r := range g.Requirements {
		if r.Status == StatusREJECTED {
			continue
		}
		for _, aID := range r.Assumptions {
			counts[aID]++
		}
	}
	return counts
}

func lessStringSlice(a, b []string) bool {
	la, lb := len(a), len(b)
	m := la
	if lb < m {
		m = lb
	}
	for i := 0; i < m; i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return la < lb
}

func DependencyChains(g *Graph) [][]string {
	ids := map[string]struct{}{}
	for _, r := range g.Requirements {
		ids[r.ID] = struct{}{}
	}
	deps := map[string]map[string]struct{}{}
	rdeps := map[string]map[string]struct{}{}
	for _, r := range g.Requirements {
		deps[r.ID] = map[string]struct{}{}
		rdeps[r.ID] = map[string]struct{}{}
	}
	for _, r := range g.Requirements {
		for _, rel := range r.Relations {
			if rel.Kind == "depends_on" {
				if _, ok := ids[rel.Target]; !ok {
					continue
				}
				deps[r.ID][rel.Target] = struct{}{}
				rdeps[rel.Target][r.ID] = struct{}{}
			}
		}
	}

	var roots []string
	for rid := range ids {
		if len(rdeps[rid]) == 0 && len(deps[rid]) > 0 {
			roots = append(roots, rid)
		}
	}
	sort.Strings(roots)

	chainSet := map[string][]string{}
	walk := func(root string) {
		type frame struct {
			node string
			path []string
		}
		stack := []frame{{root, []string{root}}}
		for len(stack) > 0 {
			f := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			var children []string
			for c := range deps[f.node] {
				children = append(children, c)
			}
			sort.Strings(children)
			var unseen []string
			inPath := map[string]bool{}
			for _, p := range f.path {
				inPath[p] = true
			}
			for _, c := range children {
				if !inPath[c] {
					unseen = append(unseen, c)
				}
			}
			if len(unseen) == 0 {
				if len(f.path) >= 2 {
					rev := make([]string, len(f.path))
					for i, p := range f.path {
						rev[len(f.path)-1-i] = p
					}
					chainSet[strings.Join(rev, "\x00")] = rev
				}
				continue
			}
			for _, c := range unseen {
				next := make([]string, 0, len(f.path)+1)
				next = append(next, f.path...)
				next = append(next, c)
				stack = append(stack, frame{c, next})
			}
		}
	}
	for _, root := range roots {
		walk(root)
	}

	out := make([][]string, 0, len(chainSet))
	for _, c := range chainSet {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return lessStringSlice(out[i], out[j]) })
	return out
}

func IndependentSubgraphs(g *Graph) [][]string {
	parent := map[string]string{}
	for _, r := range g.Requirements {
		parent[r.ID] = r.ID
	}
	var find func(string) string
	find = func(x string) string {
		for parent[x] != x {
			parent[x] = parent[parent[x]]
			x = parent[x]
		}
		return x
	}
	union := func(a, b string) {
		ra, rb := find(a), find(b)
		if ra == rb {
			return
		}
		hi, lo := ra, rb
		if ra > rb {
			hi, lo = rb, ra
		}
		parent[hi] = lo
	}

	ids := map[string]struct{}{}
	for rid := range parent {
		ids[rid] = struct{}{}
	}
	for _, r := range g.Requirements {
		for _, rel := range r.Relations {
			if rel.Kind == "depends_on" {
				if _, ok := ids[rel.Target]; !ok {
					continue
				}
				union(r.ID, rel.Target)
			}
		}
	}

	comps := map[string][]string{}
	for rid := range ids {
		root := find(rid)
		comps[root] = append(comps[root], rid)
	}

	out := make([][]string, 0, len(comps))
	for _, members := range comps {
		cp := append([]string(nil), members...)
		sort.Strings(cp)
		out = append(out, cp)
	}
	sort.Slice(out, func(i, j int) bool { return lessStringSlice(out[i], out[j]) })
	return out
}

type latentRecord struct {
	minCount  int
	signature []string
	left      string
	right     string
}

func latentPairRecords(g *Graph) []latentRecord {
	already := MembersPairSet(g)
	refCounts := AssumptionReferenceCounts(g)

	var reqs []Requirement
	for _, r := range g.Requirements {
		if r.Status != StatusREJECTED {
			reqs = append(reqs, r)
		}
	}

	var records []latentRecord
	for i := 0; i < len(reqs); i++ {
		for j := i + 1; j < len(reqs); j++ {
			a, b := reqs[i], reqs[j]
			aSet := map[string]struct{}{}
			for _, x := range a.Assumptions {
				aSet[x] = struct{}{}
			}
			var shared []string
			for _, x := range b.Assumptions {
				if _, ok := aSet[x]; ok {
					shared = append(shared, x)
				}
			}
			if len(shared) == 0 {
				continue
			}
			var specific []string
			for _, aID := range shared {
				if refCounts[aID] < GenericAssumptionThreshold {
					specific = append(specific, aID)
				}
			}
			if len(specific) == 0 {
				continue
			}
			if _, ok := already[pairKey(a.ID, b.ID)]; ok {
				continue
			}
			left, right := a.ID, b.ID
			if left > right {
				left, right = right, left
			}
			minCount := refCounts[specific[0]]
			for _, aID := range specific[1:] {
				if refCounts[aID] < minCount {
					minCount = refCounts[aID]
				}
			}
			sigSorted := append([]string(nil), specific...)
			sort.Strings(sigSorted)
			records = append(records, latentRecord{minCount, sigSorted, left, right})
		}
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].minCount != records[j].minCount {
			return records[i].minCount < records[j].minCount
		}
		if records[i].left != records[j].left {
			return records[i].left < records[j].left
		}
		return records[i].right < records[j].right
	})
	return records
}

func LatentConnectorSuspects(g *Graph) []LatentSuspect {
	records := latentPairRecords(g)
	out := make([]LatentSuspect, 0, len(records))
	for _, rec := range records {
		out = append(out, LatentSuspect{
			Left:  rec.left,
			Right: rec.right,
			Hint:  "shares assumption(s): " + strings.Join(rec.signature, ", "),
		})
	}
	return out
}

func LatentConnectorClusters(g *Graph) []LatentCluster {
	records := latentPairRecords(g)
	grouped := map[string][]latentRecord{}
	for _, rec := range records {
		key := strings.Join(rec.signature, "\x00")
		grouped[key] = append(grouped[key], rec)
	}

	type clusterEntry struct {
		minCount int
		cluster  LatentCluster
	}
	var clusters []clusterEntry
	for _, recs := range grouped {
		memberSet := map[string]struct{}{}
		var pairs []LatentSuspect
		for _, rec := range recs {
			memberSet[rec.left] = struct{}{}
			memberSet[rec.right] = struct{}{}
			pairs = append(pairs, LatentSuspect{
				Left:  rec.left,
				Right: rec.right,
				Hint:  "shares assumption(s): " + strings.Join(rec.signature, ", "),
			})
		}
		var members []string
		for m := range memberSet {
			members = append(members, m)
		}
		sort.Strings(members)
		clusterMin := recs[0].minCount
		for _, rec := range recs[1:] {
			if rec.minCount < clusterMin {
				clusterMin = rec.minCount
			}
		}
		clusters = append(clusters, clusterEntry{
			minCount: clusterMin,
			cluster: LatentCluster{
				Assumptions:  recs[0].signature,
				Requirements: members,
				Pairs:        pairs,
			},
		})
	}
	sort.Slice(clusters, func(i, j int) bool {
		if clusters[i].minCount != clusters[j].minCount {
			return clusters[i].minCount < clusters[j].minCount
		}
		return lessStringSlice(clusters[i].cluster.Assumptions, clusters[j].cluster.Assumptions)
	})
	out := make([]LatentCluster, 0, len(clusters))
	for _, c := range clusters {
		out = append(out, c.cluster)
	}
	return out
}

func EntityStateConflictSuspects(g *Graph) []LatentSuspect {
	typeBySlug := map[string]EntityType{}
	for _, et := range g.EntityTypes {
		typeBySlug[et.Slug] = et
	}

	processDestinations := func(p Process, slug string) map[string]struct{} {
		et, ok := typeBySlug[slug]
		if !ok {
			return map[string]struct{}{}
		}
		transitionsByEvent := map[string]Transition{}
		for _, t := range et.Lifecycle.Transitions {
			transitionsByEvent[t.Event] = t
		}
		terminal := map[string]struct{}{}
		for _, s := range et.Lifecycle.States {
			if s.IsTerminal() {
				terminal[s.Name] = struct{}{}
			}
		}
		dests := map[string]struct{}{}
		for _, step := range p.Steps {
			if step.Invokes == "" || !strings.Contains(step.Invokes, ".") {
				continue
			}
			parts := strings.SplitN(step.Invokes, ".", 2)
			s, event := parts[0], parts[1]
			if s != slug {
				continue
			}
			if t, ok := transitionsByEvent[event]; ok {
				if _, isTerm := terminal[t.Dst]; isTerm {
					dests[t.Dst] = struct{}{}
				}
			}
		}
		return dests
	}

	var suspects []LatentSuspect
	for _, et := range g.EntityTypes {
		slug := et.Slug
		var ps []Process
		for _, p := range g.Processes {
			for _, de := range p.DrivesEntities {
				if de == slug {
					ps = append(ps, p)
					break
				}
			}
		}
		for i := 0; i < len(ps); i++ {
			for j := i + 1; j < len(ps); j++ {
				a, b := ps[i], ps[j]
				da := processDestinations(a, slug)
				db := processDestinations(b, slug)
				if len(da) == 0 || len(db) == 0 {
					continue
				}
				disjoint := true
				for d := range da {
					if _, ok := db[d]; ok {
						disjoint = false
						break
					}
				}
				if !disjoint {
					continue
				}
				left, right := a.ID, b.ID
				if left > right {
					left, right = right, left
				}
				das := sortedKeys(da)
				dbs := sortedKeys(db)
				hint := fmt.Sprintf(
					"both drive entity '%s' but to disjoint resting states: %v vs %v — likely conflict on axis behavioral-%s-state",
					slug, das, dbs, slug,
				)
				suspects = append(suspects, LatentSuspect{Left: left, Right: right, Hint: hint})
			}
		}
	}
	sort.Slice(suspects, func(i, j int) bool {
		if suspects[i].Left != suspects[j].Left {
			return suspects[i].Left < suspects[j].Left
		}
		return suspects[i].Right < suspects[j].Right
	})
	return suspects
}

func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
