// orientation_faq.go holds check_orientation_faq_answered: the MECHANICAL
// orientability gate for a domain that has declared an "orientation_faq"
// list in its manifest.json. Making "an AI agent (or a new human reader)
// orients in this project fast" a CHECKABLE property — not a hope or a
// feeling — is the whole point: each declared basic onboarding question
// (what is this project? what is the requirement lifecycle? who decides
// what? what is currently blocked? where is the full requirements list?)
// MUST be reachable from the generated crystal in at most ONE hop, or the
// domain is failing its own declared orientation contract and
// all-violations reports it the same way it reports any other broken
// invariant.
//
// HONEST NO-OP (the same shape every sibling opt-in check already
// establishes): a domain whose manifest.json carries NO "orientation_faq"
// field contributes ZERO violations regardless of how sparse its crystal is
// — exactly like check_settled_requires_scenario (no discipline:"full" =
// no scenario obligation), check_spec_md_current (no committed SPEC.md =
// no staleness), and check_recorder_current (no vendored recorder = no
// drift). Declaring the list is the explicit, per-domain opt-in that turns
// this check on; "no committed opt-in = no lie".
package invariants

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/paths"
)

// resolveCrystalRepoRoot mirrors cmd/hotam's repoRootForDomain tier-1 rule
// (cmd/hotam/gen_spec.go: when domainDir's parent is literally "domains",
// repoRoot is domainDir's grandparent) plus the same paths.ProjectRootOrRaise
// fallback for non-conforming layouts, plus domainDir itself as the minimal
// root when even that fails. Kept here (rather than imported from cmd/hotam,
// which is package main and therefore unimportable) so this check resolves
// the SAME repo root gen-spec resolves when it writes the crystal — the
// one shared fact the Link-resolution base below depends on.
func resolveCrystalRepoRoot(domainDir string) string {
	if filepath.Base(filepath.Dir(domainDir)) == "domains" {
		return filepath.Dir(filepath.Dir(domainDir))
	}
	if root, err := paths.ProjectRootOrRaise(); err == nil {
		return root
	}
	return domainDir
}

// resolveCrystalPath returns the path of the GENERATED crystal a freshly-
// spawned agent actually reads when orienting to this domain — the file
// whose text this check searches for inline answers and one-hop links.
//
// It mirrors cmd/hotam.resolveClaudeMDPath's INTENT (a consumer domain
// under <repoRoot>/domains/ gets its OWN local crystal at
// <domainDir>/CLAUDE.md; the active/unambiguous domain gets the repo-root
// CLAUDE.md) via a robust EXISTING-FILE priority that needs none of the
// active-domain / marker / env machinery resolveClaudeMDPath itself relies
// on (that machinery lives in package main and is not importable here):
//
//  1. <domainDir>/CLAUDE.md — a consumer domain's local crystal. By
//     generation convention (task A2) the active domain NEVER gets a local
//     crystal (it writes the root one), so this file existing is a reliable
//     signal that THIS domain's orientation text lives here.
//  2. <repoRoot>/CLAUDE.md — the active domain's root crystal.
//
// Returns "" when neither exists (a bare test fixture or an un-adopted
// project with no crystal convention) — an HONEST NO-OP for this check, the
// same boundary resolveClaudeMDPath's crystalConventionExists gate already
// draws.
//
// PRIORITY ORDER (local before root) is correct for BOTH real cases in this
// repo and matches generation byte-for-byte: the active domain
// (hotam-spec-self) has NO local CLAUDE.md so it falls through to root;
// every consumer domain (e.g. hotam-dev) HAS a local CLAUDE.md so it is
// checked there. The only divergence from resolveClaudeMDPath would be an
// active domain that ALSO carried a stale local CLAUDE.md — a state
// generation itself never produces, so it cannot arise in a clean repo.
func resolveCrystalPath(domainDir string) string {
	local := filepath.Join(domainDir, "CLAUDE.md")
	if _, err := os.Stat(local); err == nil {
		return local
	}
	root := filepath.Join(resolveCrystalRepoRoot(domainDir), "CLAUDE.md")
	if _, err := os.Stat(root); err == nil {
		return root
	}
	return ""
}

// checkOrientationFAQAnswered is the mechanical orientability gate. For
// every entry in the domain's declared manifest.json "orientation_faq"
// list (resolved via loader.ResolveOrientationFAQ, itself an honest no-op
// when the field is absent), the answer MUST be reachable from the
// generated crystal in at most ONE hop, by EITHER signal:
//
//   - KEYWORDS INLINE: every substring in the entry's Keywords list appears
//     (case-insensitive) in the crystal's text — the answer is present
//     inline, zero hops.
//   - LINK ONE-HOP: the entry's Link (a repo-root-relative path) appears in
//     the crystal's text (as a markdown `[text](path)` link OR as a bare
//     path string) AND resolves to a REAL EXISTING FILE under the repo
//     root — the answer is exactly one hop away, at a real file.
//
// An entry with NEITHER Keywords nor Link non-empty fires a violation
// directly (the entry declares no checkable contract at all). An entry whose
// Keywords are absent AND whose Link is missing-from-crystal OR points at a
// nonexistent file fires a violation. The check is an HONEST NO-OP for a
// domain with no "orientation_faq" list (loader returns nil) and for a
// domain whose crystal path resolves to "" (no crystal convention on disk).
func checkOrientationFAQAnswered(g *ontology.Graph) []Violation {
	if g.DomainDir == "" {
		// No on-disk domain (an in-memory fixture graph built without
		// loader.LoadGraph) — honest no-op, mirroring check_recorder_current's
		// / check_spec_md_current's identical DomainDir guard.
		return nil
	}
	entries := loader.ResolveOrientationFAQ(filepath.Join(g.DomainDir, "graph.json"))
	if len(entries) == 0 {
		// This domain has not opted into the orientation_faq layer —
		// honest no-op ("no committed opt-in = no lie"), the same shape
		// check_settled_requires_scenario / check_spec_md_current /
		// check_recorder_current already establish for their own opt-in
		// fields.
		return nil
	}
	crystalPath := resolveCrystalPath(g.DomainDir)
	if crystalPath == "" {
		// The domain declared an orientation contract but has no generated
		// crystal on disk to satisfy it from — every declared question is
		// unreachable. Report one violation per entry so a steward can see
		// the full break, rather than a single "no crystal" line that
		// hides how many questions that blocks.
		out := make([]Violation, 0, len(entries))
		for _, e := range entries {
			out = append(out, Violation{
				Check: "check_orientation_faq_answered",
				ID:    e.Question,
				Message: fmt.Sprintf(
					"orientation_faq question %q cannot be answered: the domain has no generated crystal "+
						"(neither %s/CLAUDE.md nor the repo-root CLAUDE.md exists) — run `hotam gen-spec --domain %s` to generate one",
					e.Question, g.DomainDir, g.DomainDir),
			})
		}
		return out
	}
	crystalBytes, err := os.ReadFile(crystalPath)
	if err != nil {
		return []Violation{{
			Check:   "check_orientation_faq_answered",
			ID:      crystalPath,
			Message: fmt.Sprintf("could not read crystal %s: %v", crystalPath, err),
		}}
	}
	crystalText := strings.ToLower(string(crystalBytes))
	repoRoot := resolveCrystalRepoRoot(g.DomainDir)

	var out []Violation
	for _, e := range entries {
		hasKeywords := len(e.Keywords) > 0
		hasLink := strings.TrimSpace(e.Link) != ""

		// An entry with neither signal declares no checkable contract.
		if !hasKeywords && !hasLink {
			out = append(out, Violation{
				Check:   "check_orientation_faq_answered",
				ID:      e.Question,
				Message: fmt.Sprintf("orientation_faq question %q declares neither keywords nor a link — it has no checkable answer contract", e.Question),
			})
			continue
		}

		satisfied := false

		// Signal (a): all keywords present inline in the crystal (zero hops).
		if hasKeywords {
			allPresent := true
			for _, kw := range e.Keywords {
				if strings.TrimSpace(kw) == "" {
					continue
				}
				if !strings.Contains(crystalText, strings.ToLower(strings.TrimSpace(kw))) {
					allPresent = false
					break
				}
			}
			if allPresent {
				satisfied = true
			}
		}

		// Signal (b): the link appears in the crystal AND resolves to a real
		// existing file under the repo root (exactly one hop). Checked even
		// when keywords already satisfied, so a misresolvable link still
		// surfaces as its own signal when BOTH signals were declared — but
		// it only BLOCKS satisfaction here, never overrides a keyword pass.
		linkResolves := false
		if hasLink {
			link := strings.TrimSpace(e.Link)
			if strings.Contains(crystalText, strings.ToLower(link)) {
				target := filepath.Join(repoRoot, filepath.FromSlash(link))
				if _, statErr := os.Stat(target); statErr == nil {
					linkResolves = true
				}
			}
			if linkResolves {
				satisfied = true
			}
		}

		if satisfied {
			continue
		}

		// Diagnose WHY this entry failed, naming the missing signal(s) so a
		// steward can fix the crystal or the manifest entry directly.
		var missing []string
		if hasKeywords {
			missing = append(missing, "none of the declared keywords appear inline in the crystal")
		}
		if hasLink {
			link := strings.TrimSpace(e.Link)
			if !strings.Contains(crystalText, strings.ToLower(link)) {
				missing = append(missing, fmt.Sprintf("the crystal contains no reference to link %q", link))
			} else {
				target := filepath.Join(repoRoot, filepath.FromSlash(link))
				missing = append(missing, fmt.Sprintf("link %q appears in the crystal but resolves to no real file at %s", link, target))
			}
		}
		out = append(out, Violation{
			Check: "check_orientation_faq_answered",
			ID:    e.Question,
			Message: fmt.Sprintf(
				"orientation_faq question %q is not answerable from the crystal %s in <=1 hop: %s — "+
					"either add the answer inline (keywords) to the crystal or point a one-hop link at a real file that holds the answer",
				e.Question, crystalPath, strings.Join(missing, "; ")),
		})
	}
	return out
}

var _ = All.MustRegister("check_orientation_faq_answered", Invariant{
	Name:  "check_orientation_faq_answered",
	Canon: methodology.Domain,
	Claim: "for every question a domain declares in its manifest.json orientation_faq list, the answer is reachable from the generated crystal in at most one hop.",
	Rule: "for each entry in a domain's manifest.json \"orientation_faq\" list (an HONEST NO-OP when the field is absent — " +
		"\"no committed opt-in = no lie\", the same shape check_settled_requires_scenario / check_spec_md_current / " +
		"check_recorder_current establish for their own opt-in fields), AT LEAST ONE of two satisfaction signals MUST hold " +
		"against the domain's generated crystal (the active domain's repo-root CLAUDE.md, or a consumer domain's own " +
		"<domainDir>/CLAUDE.md — the SAME file resolveClaudeMDPath writes during gen-spec/land): (a) KEYWORDS INLINE — every " +
		"substring in the entry's \"keywords\" list appears (case-insensitive) in the crystal's text, i.e. the answer is " +
		"present with ZERO hops; OR (b) LINK ONE-HOP — the entry's \"link\" (a repo-root-relative path, the SAME convention " +
		"the crystal's own cross-references use, e.g. \"domains/<name>/docs/gen/PIPELINE.md\") appears in the crystal's text " +
		"(as a markdown [text](path) link OR a bare path string) AND resolves to a REAL EXISTING FILE under the repo root. " +
		"A chain of links (crystal -> index -> answer) does NOT satisfy this check by design: the answer must be at most ONE " +
		"hop from the crystal. An entry declaring NEITHER keywords nor a link fires a violation directly (no checkable " +
		"contract). A domain with no crystal on disk fires one violation per declared question.",
	Why: "R-orientation-faq-answerable makes 'an AI agent orients in this project fast' a MECHANICALLY CHECKABLE property " +
		"rather than a hope: the domain DECLARES, in its own manifest, the basic onboarding questions a freshly-spawned " +
		"operator must be able to answer, and this invariant PROVES each declared answer is reachable from the crystal the " +
		"agent actually boots from — never silently absent, never buried behind a chain of pointers an agent would have to " +
		"follow blind. This is the orientation showcase property made drift-proof the same way every other check_* in this " +
		"package makes its own property drift-proof: a steward who edits the crystal, moves a doc, or rewords a section in a " +
		"way that orphans a declared answer sees it in `hotam all-violations` immediately, not after a confused demo session. " +
		"The opt-in shape (honest no-op when the list is absent) is deliberate: orientability-as-invariant is a domain's own " +
		"public promise, not an engine-wide mandate — exactly the boundary discipline:\"full\" / the committed SPEC.md / the " +
		"vendored recorder already draw between 'the engine guarantees the structural floor for every domain' and 'a domain " +
		"opts into a stricter contract by declaring it'.",
	Check: checkOrientationFAQAnswered,
})
