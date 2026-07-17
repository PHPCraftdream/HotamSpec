package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/paths"
	"github.com/PHPCraftdream/HotamSpec/internal/proposal"
)

// cmdLand implements `hotam land`: the single-command pipeline that keeps
// graph.json and its rendered docs/gen/*.md in sync, closing the gap
// described by TaskList P1-4 — internal/proposal/apply.go's Apply() writes
// only graph.json + graph.lock and never regenerates docs, so every
// standalone `hotam apply-proposal` leaves the graph and docs/gen/CLAUDE.md/
// AGENTS.md diverged until someone remembers to run gen-spec by hand.
//
// land runs three steps in sequence, reusing the exact same code paths as
// the standalone commands (applyProposalValue / ApplyBatch / genSpec /
// allViolations) so its behavior is provably identical to running them one
// at a time:
//
//  1. apply the proposal — a single positional file (parseProposalFile +
//     applyProposalValue) or a whole directory of proposals applied atomically
//     via --batch (loadBatchDir + proposal.ApplyBatch). Strict decode;
//     Apply/ApplyBatch
//     reject the write outright if the mutated graph would introduce new
//     invariant violations — see internal/proposal/apply.go.
//  2. regenerate docs/gen/*.md + graph.json for the domain from the newly
//     written graph.
//  3. run all-violations again as a safety-net verification pass. Step 1
//     already guarantees the graph was valid at the moment it was written,
//     so this step is NOT the thing standing between an invalid graph and
//     disk — it exists to catch drift introduced by gen-spec itself (a
//     rendering bug, a stale generator) or by anything else that touched
//     the graph between steps 1 and 3. If it finds violations anyway, land
//     still exits non-zero so the caller cannot mistake "applied" for
//     "graph is currently valid".
//
// The whole pipeline is TRANSACTIONAL with respect to graph.json + the
// generated docs: before step 1 writes anything, land snapshots the current
// on-disk graph.json + graph.lock. If step 2 or step 3 then fails AFTER step
// 1 already wrote a new graph.json, land restores those two files from the
// snapshot and re-runs genSpec — because genSpec is a pure function of
// graph.json (it calls loadDomainGraph fresh every time and deterministically
// regenerates every doc), restoring the graph and re-rendering restores the
// docs too. The domain is therefore never left with a new graph.json next to
// stale (pre-land) docs; a failure rolls back to a mutually-consistent
// pre-land state instead (R-land-is-transactional).
//
// In batch mode step 1 applies N proposals to one in-memory graph and
// regenerates docs exactly once (one gen-spec, one all-violations), not N
// times — the whole point of the batch flag for the ~200-proposal waves.
func cmdLand(args []string) error {
	fs := newFlagSet("land")
	domain := fs.String("domain", "", "domain directory containing graph.json (default: active-domain chain — HOTAM_DOMAIN env, then .hotam-spec-project marker, then "+defaultDomainRel+")")
	today := fs.String("today", "", "date in YYYY-MM-DD format (required)")
	claudeMD := fs.String("claude-md", "", "path to CLAUDE.md for rune count (passed through to gen-spec)")
	batchDir := fs.String("batch", "", "apply every *.json proposal file in <dir> atomically in filename order (alternative to a single positional proposal file)")
	ackConflict := fs.String("ack-conflict", "", "cite an existing Conflict node (C-...) whose members cover a semantic conflict the gate detected — overrides the semantic-conflict refusal")
	decisionRef := fs.String("decision-ref", "", "free-text reference to where a human decision was recorded (ticket, meeting, steward+date) — overrides the semantic-conflict refusal and is persisted in the requirement's History")
	asJSON := fs.Bool("json", false, "emit machine-readable JSON")
	fs.Parse(args)

	if *today == "" {
		return fmt.Errorf("--today is required (YYYY-MM-DD)")
	}

	domainDir, err := resolveDomain(*domain)
	if err != nil {
		return err
	}

	if *batchDir != "" {
		result, err := cmdLandBatch(*batchDir, domainDir, *today, *claudeMD, *asJSON)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(result)
		}
		return nil
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("usage: hotam land <proposal.json> --domain <path> --today YYYY-MM-DD [--batch <dir>] [--claude-md <path>] [--json]")
	}
	// Parse the proposal ONCE so the advisory confront-at-gate (warn-only,
	// never blocks — R-ai-presents-not-decides) runs BEFORE the land outcome,
	// then the same parsed value drives the land pipeline. A parse failure is
	// wrapped as "apply step failed" exactly as landProposalFile did when it
	// owned the parse, preserving the existing error contract.
	proposalFile := fs.Arg(0)
	p, err := parseProposalFile(proposalFile)
	if err != nil {
		return fmt.Errorf("apply step failed, nothing landed: %w", err)
	}
	if err := confrontBeforeApply(domainDir, p, *asJSON); err != nil {
		return err
	}
	ackOpts := landAckOptions{AckConflict: *ackConflict, DecisionRef: *decisionRef}
	result, err := landProposalValue(p, domainDir, *claudeMD, *today, ackOpts, *asJSON)
	if err != nil {
		return err
	}
	if *asJSON {
		return printJSON(result)
	}
	return nil
}

// cmdLandBatch is the --batch <dir> code path of `hotam land`: snapshot,
// apply every *.json in <dir> atomically, regen docs, re-verify, and roll
// back on failure. It is the batch counterpart to landProposalFile; the two
// share the same snapshot/genSpec/allViolations/rollback shape but differ in
// the apply step (ApplyBatch vs applyProposalValue).
//
// The semantic-conflict gate runs in TWO halves on the batch path. The
// BLOCKING half — opposite-marker detection (diagnose.IsBlockingHit) — now
// runs INSIDE internal/proposal.ApplyBatch via an injected
// proposal.ConflictChecker (batchConflictChecker, built here in cmd/hotam
// using diagnose.IsBlockingHit, since internal/proposal cannot import
// internal/diagnose directly — R-core-periphery-import-ratchet): each
// ProposedRequirement is confronted against the rolling in-memory graph (so
// it catches contradictions against pre-existing state AND against earlier
// items of the same batch), and ANY blocking hit aborts the ENTIRE batch
// atomically (disk untouched). The OVERRIDE half — --ack-conflict /
// --decision-ref — does NOT
// run in batch mode: those flags are inherently per-proposal, but batch mode
// processes a directory of files with no per-file flag mechanism; a batch
// item that trips the blocking gate must be pulled out and landed
// individually via `hotam land`/`hotam apply-proposal` (single-file) with an
// explicit ack. The advisory confront-at-gate summary (confrontBatchSummary)
// also still runs as visibility-only.
func cmdLandBatch(batchDir, domainDir, today, claudeMDPath string, asJSON bool) (*LandResult, error) {
	// Resolve the effective crystal path once (see landProposalValue for the
	// rationale): the forward genSpec and every rollback re-render in this
	// batch path must agree on whether the root crystal is in scope.
	claudeMDPath = resolveClaudeMDPath(domainDir, claudeMDPath)
	snapshot, err := snapshotGraphFiles(domainDir)
	if err != nil {
		return nil, fmt.Errorf("pre-land snapshot failed, nothing landed: %w", err)
	}

	proposals, err := loadBatchDir(batchDir)
	if err != nil {
		return nil, fmt.Errorf("apply step failed, nothing landed: %w", err)
	}
	// Advisory confront-at-gate (warn-only, never blocks): run AFTER every
	// proposal is parsed but BEFORE ApplyBatch mutates the graph, so all N
	// candidates are checked against the SAME starting snapshot the batch then
	// applies atomically.
	if err := confrontBatchSummary(domainDir, proposals, asJSON); err != nil {
		return nil, err
	}
	gp := graphPathForDomain(domainDir)
	if err := proposal.ApplyBatch(gp, today, proposals, batchConflictChecker, batchProvenanceChecker(domainDir)); err != nil {
		return nil, fmt.Errorf("apply step failed, nothing landed: %w", err)
	}
	fmt.Fprintf(landOut(asJSON), "applied batch of %d proposals to %s\n", len(proposals), relPathForDisplay(gp))

	written, _, err := genSpec(domainDir, claudeMDPath, today, "", false)
	if err != nil {
		rerr := rollbackLand(domainDir, snapshot, claudeMDPath, today)
		return nil, rolledBackError("doc regeneration failed", err, rerr)
	}
	fmt.Fprintf(landOut(asJSON), "regenerated %d doc(s)\n", len(written))

	violations, err := allViolations(domainDir)
	if err != nil {
		rerr := rollbackLand(domainDir, snapshot, claudeMDPath, today)
		return nil, rolledBackError("violation check failed to run", err, rerr)
	}
	if len(violations) > 0 {
		for _, v := range violations {
			fmt.Fprintf(landOut(asJSON), "[%s] %s: %s\n", v.Check, v.ID, v.Message)
		}
		cause := fmt.Errorf("%d invariant violation(s) found after gen-spec (apply already validated the graph before writing it — this signals drift introduced by gen-spec or a concurrent change, not a bad proposal)", len(violations))
		rerr := rollbackLand(domainDir, snapshot, claudeMDPath, today)
		return nil, rolledBackError("graph invalid after gen-spec", cause, rerr)
	}

	fmt.Fprintln(landOut(asJSON), "landed: graph applied, docs regenerated, 0 violations")
	return &LandResult{
		Landed:          true,
		GraphPath:       gp,
		RegeneratedDocs: len(written),
		Violations:      []string{},
	}, nil
}

// landProposalFile runs the full land pipeline (snapshot, apply, regen,
// re-verify, rollback-on-failure) against a single already-written proposal
// file. It is a thin read+parse wrapper over parseProposalFile +
// landProposalValue. Shared by cmdLand's single-file mode and `hotam propose`'s
// --land flag, so both paths are provably identical — the transactional
// snapshot/rollback logic lives in exactly one place (landProposalValue).
//
// No confront check runs here (or inside landProposalValue): `hotam propose
// --land` runs its OWN confront in runPropose before calling this function, so
// embedding one here would double-print the report on that path. The direct
// `hotam land <file>` path runs its confront at the cmdLand command level
// before calling landProposalValue.
//
// ackOpts threads the --ack-conflict / --decision-ref flags through to the
// semantic-conflict gate inside landProposalValue.
func landProposalFile(proposalFile, domainDir, claudeMDPath, today string, ackOpts landAckOptions, asJSON bool) (*LandResult, error) {
	p, err := parseProposalFile(proposalFile)
	if err != nil {
		return nil, fmt.Errorf("apply step failed, nothing landed: %w", err)
	}
	return landProposalValue(p, domainDir, claudeMDPath, today, ackOpts, asJSON)
}

// resolveClaudeMDPath returns the effective root-crystal path a land operation
// should regenerate. An explicit non-empty path always wins (operator
// override). Otherwise, if domainDir resolves to a real project root (via
// repoRootForDomain) that already carries a root CLAUDE.md or a
// .hotam-spec-project marker, land defaults to regenerating
// <repoRoot>/CLAUDE.md automatically — closing the class of bug where
// graph.json + docs/gen/*.md are fresh but the crystal an agent actually boots
// from is stale. A domain NOT linked to any such project root (e.g. an isolated
// test fixture, or a bare domain nobody has run init-project / hotam use on)
// gets no auto-write — this only activates where a crystal convention already
// exists.
//
// The CLAUDE.md / marker checks are DIRECT os.Stat calls at repoRoot itself,
// not an upward walk, so this function is not vulnerable to the ancestor-
// marker contamination shape (a stray marker several levels above) — only
// repoRootForDomain's own internal tier-2 ProjectRootOrRaise() call walks
// upward, and only for non-domains/<name> layouts.
//
// A second gate (review-7 R7-b, fixing task #134's own regression) then
// confirms the domain actually being landed is the one the root crystal
// speaks FOR before auto-writing — the crystal's Role/identity content is
// domain-specific ("Operator of `<domain>`"), so auto-regenerating it from a
// non-active domain would silently hijack the root crystal's identity to
// whichever domain happened to run `land` last. Auto-write proceeds only
// when one of these holds:
//
//  1. repoRoot == domainDir — the bare/single-domain-is-root tier-3 layout,
//     where there is only ever one domain and nothing to disambiguate.
//  2. Exactly one directory exists under <repoRoot>/domains/ — a
//     single-domain project is unambiguous even with no recorded
//     active-domain preference.
//  3. The domain being landed (domainNameFromDir) matches the active domain
//     resolveActiveDomainName resolves for repoRoot (HOTAM_DOMAIN env, then
//     the .hotam-spec-project marker's active_domain, then the legacy
//     default name) — this is the genuinely-active-domain case.
//
// Everything else (2+ domains under repoRoot/domains/, landing domain not the
// active one) returns "" — same as the no-convention case — so the operator
// must pass an explicit --claude-md to update the crystal from a non-active
// domain's content.
func resolveClaudeMDPath(domainDir, explicit string) string {
	if explicit != "" {
		return explicit
	}
	repoRoot := repoRootForDomain(domainDir)
	candidate := filepath.Join(repoRoot, "CLAUDE.md")
	hasConvention := false
	if _, err := os.Stat(candidate); err == nil {
		hasConvention = true
	} else if _, err := os.Stat(filepath.Join(repoRoot, paths.MarkerFilename)); err == nil {
		hasConvention = true
	}
	if !hasConvention {
		return ""
	}
	if !isActiveOrUnambiguousDomain(repoRoot, domainDir) {
		return ""
	}
	return candidate
}

// isActiveOrUnambiguousDomain reports whether landing domainDir is safe to
// auto-write repoRoot's crystal for — either because domainDir IS repoRoot
// (nothing to disambiguate), because it is the only domain under
// <repoRoot>/domains/ (unambiguous even absent a recorded preference), or
// because it matches the active domain name resolveActiveDomainName resolves
// for repoRoot.
func isActiveOrUnambiguousDomain(repoRoot, domainDir string) bool {
	if repoRoot == domainDir {
		return true
	}
	domainsRoot := filepath.Join(repoRoot, "domains")
	entries, err := os.ReadDir(domainsRoot)
	if err == nil {
		dirCount := 0
		for _, e := range entries {
			if e.IsDir() {
				dirCount++
			}
		}
		if dirCount == 1 {
			return true
		}
	}
	activeName, _ := resolveActiveDomainName(repoRoot)
	return domainNameFromDir(domainDir) == activeName
}

// landProposalValue runs the full land pipeline (semantic-conflict gate,
// snapshot, apply an already-parsed proposal, regen, re-verify,
// rollback-on-failure). It is the parse-free core shared by landProposalFile
// (which reads + parses the file) and the cmdLand single-file command branch
// (which parses once for an advisory confront check before landing). The
// transactional snapshot/rollback lives here so every caller that lands a
// single proposal shares one implementation.
//
// The semantic-conflict gate runs FIRST, before the snapshot: if it refuses
// (a ProposedRequirement whose claim carries an opposite marker against an
// existing SETTLED requirement, with no ack supplied), nothing is mutated. See
// semanticConflictGate for the signal definition and R-ai-presents-not-decides
// / R-decided-needs-human-signoff for why this requires a recorded decision
// rather than auto-resolving.
func landProposalValue(p proposal.Proposal, domainDir, claudeMDPath, today string, ackOpts landAckOptions, asJSON bool) (*LandResult, error) {
	// Semantic-conflict gate: refuse to land a ProposedRequirement whose claim
	// carries an opposite marker against an existing SETTLED requirement,
	// unless the operator supplied --ack-conflict or --decision-ref. Runs
	// BEFORE the snapshot so a refusal leaves the graph untouched. hadConflict
	// is reused below to gate appendAckHistory: the audit trail is written only
	// when a real conflict was detected, not merely because ack flags were
	// passed (prevents a false "semantic conflict acknowledged" entry on a
	// non-conflicting land with --decision-ref).
	hadConflict, err := semanticConflictGate(domainDir, p, ackOpts)
	if err != nil {
		return nil, err
	}

	// Provenance gate (opt-in, task #158): refuse to land a SETTLED
	// ProposedRequirement with incomplete provenance (source_refs,
	// last_reviewed_at, review_after) when this domain's manifest.json sets
	// require_provenance: true. A no-op for every domain that does not opt
	// in — see provenanceGate's doc comment. Runs BEFORE the snapshot so a
	// refusal leaves the graph untouched, same placement as the
	// semantic-conflict gate above.
	if err := provenanceGate(domainDir, p); err != nil {
		return nil, err
	}

	// Resolve the effective crystal path ONCE so every downstream step — the
	// forward genSpec AND any rollback re-render — agrees on whether the root
	// crystal is in scope for this land. An explicit --claude-md wins; absent
	// that, a domain linked to a real project root (CLAUDE.md or marker
	// present) auto-regenerates the crystal so docs/gen and the boot crystal
	// can never diverge again (see resolveClaudeMDPath).
	claudeMDPath = resolveClaudeMDPath(domainDir, claudeMDPath)
	snapshot, err := snapshotGraphFiles(domainDir)
	if err != nil {
		return nil, fmt.Errorf("pre-land snapshot failed, nothing landed: %w", err)
	}

	gp, err := applyProposalValue(p, domainDir, today)
	if err != nil {
		return nil, fmt.Errorf("apply step failed, nothing landed: %w", err)
	}
	fmt.Fprintf(landOut(asJSON), "applied %s %s to %s\n", p.Kind(), p.TargetAnchor(), relPathForDisplay(gp))

	// Persist the human-decision audit trail (--ack-conflict / --decision-ref)
	// AFTER apply wrote the new/updated requirement but BEFORE regen renders
	// the docs, so the History entry appears in the generated output. Guarded
	// on hadConflict (a real signal was found) AND ackOpts.hasAck(): ack flags
	// without a real conflict must NOT write a false audit entry. A failure
	// here triggers the same rollback as any other post-apply failure.
	if hadConflict && ackOpts.hasAck() {
		if err := appendAckHistory(gp, p, today, ackOpts); err != nil {
			rerr := rollbackLand(domainDir, snapshot, claudeMDPath, today)
			return nil, rolledBackError("ack history append failed", err, rerr)
		}
	}

	written, _, err := genSpec(domainDir, claudeMDPath, today, "", false)
	if err != nil {
		rerr := rollbackLand(domainDir, snapshot, claudeMDPath, today)
		return nil, rolledBackError("doc regeneration failed", err, rerr)
	}
	fmt.Fprintf(landOut(asJSON), "regenerated %d doc(s)\n", len(written))

	violations, err := allViolations(domainDir)
	if err != nil {
		rerr := rollbackLand(domainDir, snapshot, claudeMDPath, today)
		return nil, rolledBackError("violation check failed to run", err, rerr)
	}
	if len(violations) > 0 {
		for _, v := range violations {
			fmt.Fprintf(landOut(asJSON), "[%s] %s: %s\n", v.Check, v.ID, v.Message)
		}
		cause := fmt.Errorf("%d invariant violation(s) found after gen-spec (apply already validated the graph before writing it — this signals drift introduced by gen-spec or a concurrent change, not a bad proposal)", len(violations))
		rerr := rollbackLand(domainDir, snapshot, claudeMDPath, today)
		return nil, rolledBackError("graph invalid after gen-spec", cause, rerr)
	}

	fmt.Fprintln(landOut(asJSON), "landed: graph applied, docs regenerated, 0 violations")
	return &LandResult{
		Landed:          true,
		Anchor:          p.TargetAnchor(),
		GraphPath:       gp,
		RegeneratedDocs: len(written),
		Violations:      []string{},
	}, nil
}

// LandResult is the --json envelope for a successful `hotam land` (or
// `hotam propose --land`) invocation: whether the graph was applied, the
// anchor landed (empty for batch mode), the graph path written, how many docs
// were regenerated, and any violations found. On the failure path no
// LandResult is produced — the error is returned to main.go which routes it to
// stderr and exits non-zero, consistently with every other command.
type LandResult struct {
	Landed          bool     `json:"landed"`
	Anchor          string   `json:"anchor,omitempty"`
	GraphPath       string   `json:"graph_path,omitempty"`
	RegeneratedDocs int      `json:"regenerated_docs"`
	Violations      []string `json:"violations"`
}

// landOut returns the writer for operational land messages. When asJSON is
// true, messages go to os.Stderr so stdout carries exactly one JSON document;
// otherwise they go to os.Stdout (byte-identical to pre-JSON behavior).
func landOut(asJSON bool) *os.File {
	if asJSON {
		return os.Stderr
	}
	return os.Stdout
}

// graphSnapshot captures the pre-apply bytes of a domain's graph.json and
// graph.lock so a failed land can be rolled back. The Present flags
// distinguish "file did not exist before land" (rollback must REMOVE it) from
// "file existed and was empty" — a first-ever land into a brand-new domain
// has both absent, while copySelfDomain fixtures copy graph.json but no
// graph.lock.
type graphSnapshot struct {
	graphBytes   []byte
	graphPresent bool
	lockBytes    []byte
	lockPresent  bool
}

// snapshotGraphFiles reads the current on-disk bytes of the target domain's
// graph.json + graph.lock via the same path helpers apply-proposal uses
// (graphPathForDomain / loader.LockPath). A not-yet-existing file is recorded
// as absent rather than an error; any other read failure is fatal so land
// never starts mutating a domain it cannot later restore.
func snapshotGraphFiles(domainDir string) (*graphSnapshot, error) {
	gp := graphPathForDomain(domainDir)
	lp := loader.LockPath(gp)
	s := &graphSnapshot{}

	switch gData, gErr := os.ReadFile(gp); {
	case gErr == nil:
		s.graphBytes = gData
		s.graphPresent = true
	case os.IsNotExist(gErr):
		s.graphPresent = false
	default:
		return nil, fmt.Errorf("read pre-land graph %s: %w", gp, gErr)
	}

	switch lData, lErr := os.ReadFile(lp); {
	case lErr == nil:
		s.lockBytes = lData
		s.lockPresent = true
	case os.IsNotExist(lErr):
		s.lockPresent = false
	default:
		return nil, fmt.Errorf("read pre-land lock %s: %w", lp, lErr)
	}

	return s, nil
}

// rollbackLand restores a domain's graph.json + graph.lock to the bytes held
// in snap, then re-runs genSpec so the rendered docs match the restored
// graph. Because genSpec deterministically regenerates every doc from
// graph.json (it reloads it fresh every call), this is sufficient to return
// the whole domain — graph AND docs — to its pre-land state. It returns nil
// only when both the file restore AND the re-render succeed; a non-nil return
// means the graph files WERE restored (restoreGraphFile runs before the
// re-genSpec) but the doc re-render failed, so the caller must surface that
// loudly alongside the original failure rather than swallow either.
func rollbackLand(domainDir string, snap *graphSnapshot, claudeMDPath, today string) error {
	gp := graphPathForDomain(domainDir)
	lp := loader.LockPath(gp)

	if err := restoreGraphFile(gp, snap.graphPresent, snap.graphBytes); err != nil {
		return err
	}
	if err := restoreGraphFile(lp, snap.lockPresent, snap.lockBytes); err != nil {
		return err
	}

	if _, _, err := genSpec(domainDir, claudeMDPath, today, "", false); err != nil {
		return fmt.Errorf("rollback doc regen: %w", err)
	}
	return nil
}

// restoreGraphFile writes data back to path (0o644, matching
// loader.atomicWriteFile's mode) or, when the pre-land state was absent,
// removes path — ignoring an already-absent file so rollback is idempotent.
func restoreGraphFile(path string, present bool, data []byte) error {
	if !present {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %s: %w", path, err)
		}
		return nil
	}
	return writeFileMkdir(path, data)
}

// rolledBackError reports that the land failed at stage and was rolled back
// to the pre-land state. When rollbackErr != nil the rollback itself was
// incomplete (graph.json + graph.lock were restored, but the doc re-render
// failed), and both the original cause and the rollback failure are surfaced
// so neither is swallowed — the caller still sees a "rolled back" message and
// knows the graph files on disk are the pre-land ones.
func rolledBackError(stage string, cause, rollbackErr error) error {
	if rollbackErr != nil {
		return fmt.Errorf("%s, rolled back to pre-land state but rollback incomplete (graph.json+lock restored, doc regen failed): %w; rollback error: %v",
			stage, cause, rollbackErr)
	}
	return fmt.Errorf("%s, rolled back to pre-land state: %w", stage, cause)
}
