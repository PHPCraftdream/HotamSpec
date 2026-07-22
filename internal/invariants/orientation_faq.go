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
// field AT ALL contributes ZERO violations regardless of how sparse its
// crystal is — exactly like check_settled_requires_scenario (no
// discipline:"full" = no scenario obligation), check_spec_md_current (no
// committed SPEC.md = no staleness), and check_recorder_current (no
// vendored recorder = no drift). Declaring the list is the explicit,
// per-domain opt-in that turns this check on; "no committed opt-in = no
// lie".
//
// FAIL CLOSED once a domain HAS opted in (distinct from the honest no-op
// above, which only covers "never opted in"): a manifest.json that is
// malformed JSON but whose raw bytes show the domain tried to declare
// "orientation_faq" fires a violation rather than silently losing the whole
// check; a raw list entry that fails per-entry validation (not an object, or
// an empty "question") fires a violation rather than silently shrinking the
// checked list; a "keywords" list whose every element is blank fires a
// violation rather than passing as if zero keywords had been required; and a
// "link" target must be a real, NON-EMPTY file, whose content contains at
// least one of the entry's own keywords when the entry declares any, rather
// than merely existing on disk. A domain never loses its checked property
// through an unrelated typo, an unnoticed empty file, or a boilerplate
// keywords list nobody actually populated.
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
// list (resolved via loader.ResolveOrientationFAQDiagnostic, itself an
// honest no-op for Entries when the field is absent), the answer MUST be
// reachable from the generated crystal in at most ONE hop, by EITHER
// signal:
//
//   - KEYWORDS INLINE: every non-blank keyword in the entry's Keywords list
//     appears (case-insensitive) in the crystal's text — the answer is
//     present inline, zero hops. A Keywords list that is non-empty in JSON
//     but whose every element is blank/whitespace does NOT count as "no
//     keywords declared"; it fires its own violation instead (see below).
//   - LINK ONE-HOP: the entry's Link (a repo-root-relative path) appears in
//     the crystal's text (as a markdown `[text](path)` link OR as a bare
//     path string) AND resolves to a REAL, NON-EMPTY, existing file under
//     the repo root — the answer is exactly one hop away, at a real file
//     that actually holds content. When the entry ALSO declares real
//     keywords, the linked file's content must contain at least one of
//     them (a minimal relevance check); a link-only entry (no keywords)
//     only needs the file to be non-empty.
//
// Violations, beyond a failed signal above:
//   - An entry with NEITHER Keywords nor Link non-empty (no checkable
//     contract at all).
//   - An entry whose Keywords list is present but every element is blank.
//   - A manifest.json that fails to parse as JSON but whose raw bytes show
//     the domain tried to declare "orientation_faq" (fail-closed on
//     malformed-but-declared-intent — see loader.OrientationFAQDiagnostic).
//   - A raw "orientation_faq" list entry that loader-level per-entry
//     validation dropped (not a JSON object, or an empty "question").
//
// The check is an HONEST NO-OP only for a domain whose manifest.json truly
// carries no "orientation_faq" field at all, and for a domain whose crystal
// path resolves to "" (no crystal convention on disk, checked only once the
// domain has a non-empty, non-dropped-only entries list).
func checkOrientationFAQAnswered(g *ontology.Graph) []Violation {
	if g.DomainDir == "" {
		// No on-disk domain (an in-memory fixture graph built without
		// loader.LoadGraph) — honest no-op, mirroring check_recorder_current's
		// / check_spec_md_current's identical DomainDir guard.
		return nil
	}
	diag := loader.ResolveOrientationFAQDiagnostic(filepath.Join(g.DomainDir, "graph.json"))
	entries := diag.Entries

	// FAIL CLOSED on a malformed manifest that shows every sign the domain
	// TRIED to declare orientation_faq: manifest.json exists, does NOT parse
	// as JSON (so loader.ResolveOrientationFAQ — deliberately, per its own
	// documented tolerant-resolver contract shared with every sibling
	// resolver — returned nil), but the raw bytes contain the literal
	// "orientation_faq" field name. A domain that declared this contract
	// cannot silently lose it to an unrelated JSON typo elsewhere in the
	// same file; that is exactly the gap between "never opted in" (honest
	// no-op) and "opted in, then broke" (a violation) this check exists to
	// close.
	if diag.ManifestExists && !diag.ManifestParsed && diag.ManifestDeclaresIntent {
		manifestPath := filepath.Join(g.DomainDir, "manifest.json")
		return []Violation{{
			Check: "check_orientation_faq_answered",
			ID:    manifestPath,
			Message: fmt.Sprintf(
				"manifest %s contains an \"orientation_faq\" declaration but the file does not parse as valid JSON — "+
					"the domain evidently tried to declare an orientation contract and lost it to a JSON error; "+
					"fix the manifest so it parses, or remove the orientation_faq field entirely if it was never intended",
				manifestPath),
		}}
	}

	// FAIL CLOSED on dropped entries: a well-parsed "orientation_faq" list
	// whose per-entry validation (loader.ResolveOrientationFAQDiagnostic)
	// silently dropped one or more raw entries (not a JSON object, or an
	// empty "question" field) must not be reported as if the shortened list
	// were the whole story — each dropped entry fires its own violation so a
	// resolver sees exactly which raw entries need fixing.
	var out []Violation
	for _, d := range diag.Dropped {
		out = append(out, Violation{
			Check: "check_orientation_faq_answered",
			ID:    fmt.Sprintf("%s#orientation_faq[%d]", g.DomainDir, d.Index),
			Message: fmt.Sprintf(
				"orientation_faq entry at index %d could not be loaded (%s): raw entry %s — "+
					"fix or remove this entry so it declares a checkable question",
				d.Index, d.Reason, d.Raw),
		})
	}

	if len(entries) == 0 {
		// This domain has not opted into the orientation_faq layer (or every
		// entry it declared was invalid and already reported above) —
		// honest no-op for the "nothing checkable" case itself ("no
		// committed opt-in = no lie"), the same shape
		// check_settled_requires_scenario / check_spec_md_current /
		// check_recorder_current already establish for their own opt-in
		// fields.
		return out
	}
	crystalPath := resolveCrystalPath(g.DomainDir)
	if crystalPath == "" {
		// The domain declared an orientation contract but has no generated
		// crystal on disk to satisfy it from — every declared question is
		// unreachable. Report one violation per entry so a resolver can see
		// the full break, rather than a single "no crystal" line that
		// hides how many questions that blocks. Dropped-entry violations
		// (if any) collected above are still reported alongside these.
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
		out = append(out, Violation{
			Check:   "check_orientation_faq_answered",
			ID:      crystalPath,
			Message: fmt.Sprintf("could not read crystal %s: %v", crystalPath, err),
		})
		return out
	}
	crystalText := strings.ToLower(string(crystalBytes))
	repoRoot := resolveCrystalRepoRoot(g.DomainDir)

	for _, e := range entries {
		// normalizedKeywords holds only the non-blank, trimmed keywords —
		// used for both the "declares keywords but all blank" violation and
		// the actual inline-presence / linked-file-relevance checks below,
		// so a keyword list that is present-but-entirely-whitespace is never
		// silently treated as "no keywords declared".
		var normalizedKeywords []string
		for _, kw := range e.Keywords {
			if t := strings.TrimSpace(kw); t != "" {
				normalizedKeywords = append(normalizedKeywords, t)
			}
		}
		declaresKeywords := len(e.Keywords) > 0
		hasKeywords := len(normalizedKeywords) > 0
		hasLink := strings.TrimSpace(e.Link) != ""

		// An entry with neither signal declares no checkable contract.
		if !declaresKeywords && !hasLink {
			out = append(out, Violation{
				Check:   "check_orientation_faq_answered",
				ID:      e.Question,
				Message: fmt.Sprintf("orientation_faq question %q declares neither keywords nor a link — it has no checkable answer contract", e.Question),
			})
			continue
		}

		// A keywords list that was DECLARED (non-empty in JSON) but every
		// element is blank/whitespace is its own violation, by the same
		// logic as "declares neither keywords nor a link": a keywords list
		// with no real keyword in it is not a checkable contract, and must
		// not be silently treated as "no keywords declared, fall through to
		// link" — the domain explicitly tried to declare inline-answer
		// keywords and failed to declare any real ones.
		if declaresKeywords && !hasKeywords && !hasLink {
			out = append(out, Violation{
				Check:   "check_orientation_faq_answered",
				ID:      e.Question,
				Message: fmt.Sprintf("orientation_faq question %q declares keywords but all are blank — it has no checkable answer contract", e.Question),
			})
			continue
		}

		satisfied := false

		// Signal (a): all (non-blank) keywords present inline in the crystal
		// (zero hops). Requires at least one real keyword to have been
		// checked — hasKeywords is already false for an all-blank list, so
		// this signal never fires bogus-true for that case.
		if hasKeywords {
			allPresent := true
			for _, kw := range normalizedKeywords {
				if !strings.Contains(crystalText, strings.ToLower(kw)) {
					allPresent = false
					break
				}
			}
			if allPresent {
				satisfied = true
			}
		}

		// Signal (b): the link appears in the crystal AND resolves to a
		// real, NON-EMPTY existing file under the repo root (exactly one
		// hop), AND — when the entry also declares real keywords — the
		// linked file's own content contains at least one of them (a
		// minimal relevance check: an existing-but-empty or existing-but-
		// unrelated file must not count as "the answer is one hop away").
		// Checked even when keywords already satisfied inline, so a broken
		// link still surfaces as its own signal when BOTH signals were
		// declared — but it only BLOCKS satisfaction here, never overrides
		// an inline keyword pass.
		linkResolves := false
		if hasLink {
			link := strings.TrimSpace(e.Link)
			if strings.Contains(crystalText, strings.ToLower(link)) {
				target := filepath.Join(repoRoot, filepath.FromSlash(link))
				if linkBytes, statErr := os.ReadFile(target); statErr == nil {
					linkText := strings.ToLower(string(linkBytes))
					if strings.TrimSpace(linkText) != "" {
						if !hasKeywords {
							// Link-only entry: non-empty content is the only
							// checkable signal (nothing to cross-check it
							// against).
							linkResolves = true
						} else {
							for _, kw := range normalizedKeywords {
								if strings.Contains(linkText, strings.ToLower(kw)) {
									linkResolves = true
									break
								}
							}
						}
					}
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
		// resolver can fix the crystal, the linked file, or the manifest
		// entry directly.
		var missing []string
		if hasKeywords {
			missing = append(missing, "none of the declared keywords appear inline in the crystal")
		}
		if hasLink {
			link := strings.TrimSpace(e.Link)
			target := filepath.Join(repoRoot, filepath.FromSlash(link))
			switch {
			case !strings.Contains(crystalText, strings.ToLower(link)):
				missing = append(missing, fmt.Sprintf("the crystal contains no reference to link %q", link))
			default:
				linkBytes, statErr := os.ReadFile(target)
				switch {
				case statErr != nil:
					missing = append(missing, fmt.Sprintf("link %q appears in the crystal but resolves to no real file at %s", link, target))
				case strings.TrimSpace(string(linkBytes)) == "":
					missing = append(missing, fmt.Sprintf("link %q resolves to %s, but that file is empty", link, target))
				case hasKeywords:
					missing = append(missing, fmt.Sprintf("link %q resolves to %s, but that file's content contains none of the declared keywords", link, target))
				}
			}
		}
		out = append(out, Violation{
			Check: "check_orientation_faq_answered",
			ID:    e.Question,
			Message: fmt.Sprintf(
				"orientation_faq question %q is not answerable from the crystal %s in <=1 hop: %s — "+
					"either add the answer inline (keywords) to the crystal or point a one-hop link at a real, non-empty, relevant file that holds the answer",
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
		"non-blank keyword in the entry's \"keywords\" list appears (case-insensitive) in the crystal's text, i.e. the answer " +
		"is present with ZERO hops; OR (b) LINK ONE-HOP — the entry's \"link\" (a repo-root-relative path, the SAME " +
		"convention the crystal's own cross-references use, e.g. \"domains/<name>/docs/gen/PIPELINE.md\") appears in the " +
		"crystal's text (as a markdown [text](path) link OR a bare path string) AND resolves to a REAL, NON-EMPTY, EXISTING " +
		"FILE under the repo root whose content contains at least one of the entry's own keywords when the entry declares " +
		"any (a minimal relevance check; a link-only entry needs only non-empty content). A chain of links (crystal -> index " +
		"-> answer) does NOT satisfy this check by design: the answer must be at most ONE hop from the crystal. FAIL CLOSED " +
		"beyond the opt-in boundary: an entry declaring NEITHER keywords nor a link fires a violation (no checkable " +
		"contract); a \"keywords\" list present but entirely blank fires a violation (not silently treated as absent); a " +
		"manifest.json that fails to parse as JSON but whose raw bytes contain the literal \"orientation_faq\" field name " +
		"fires a violation (the domain evidently tried to declare the contract and lost it to an unrelated JSON error, " +
		"rather than silently becoming a no-op); a raw list entry that per-entry validation drops (not a JSON object, or an " +
		"empty \"question\") fires a violation naming its index, rather than silently shrinking the checked list. A domain " +
		"with no crystal on disk fires one violation per declared question.",
	Why: "R-orientation-faq-answerable makes 'an AI agent orients in this project fast' a MECHANICALLY CHECKABLE property " +
		"rather than a hope: the domain DECLARES, in its own manifest, the basic onboarding questions a freshly-spawned " +
		"operator must be able to answer, and this invariant PROVES each declared answer is reachable from the crystal the " +
		"agent actually boots from — never silently absent, never buried behind a chain of pointers an agent would have to " +
		"follow blind. This is the orientation showcase property made drift-proof the same way every other check_* in this " +
		"package makes its own property drift-proof: a resolver who edits the crystal, moves a doc, or rewords a section in a " +
		"way that orphans a declared answer sees it in `hotam all-violations` immediately, not after a confused demo session. " +
		"The opt-in shape (honest no-op when the list is absent) is deliberate: orientability-as-invariant is a domain's own " +
		"public promise, not an engine-wide mandate — exactly the boundary discipline:\"full\" / the committed SPEC.md / the " +
		"vendored recorder already draw between 'the engine guarantees the structural floor for every domain' and 'a domain " +
		"opts into a stricter contract by declaring it'. FAIL CLOSED once opted in is the other half of that same honesty: " +
		"a domain that DID declare orientation_faq must not be able to silently lose the guarantee to a malformed manifest, " +
		"an empty or irrelevant linked file, an all-blank keywords list, or a quietly-dropped malformed entry — each of " +
		"those is a domain that BELIEVES it proved orientability while the check actually verified nothing, which is worse " +
		"than never having opted in at all.",
	Check: checkOrientationFAQAnswered,
})
