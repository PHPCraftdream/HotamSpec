package main

import (
	"fmt"
	"os"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
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
	domain := fs.String("domain", "", "domain directory containing graph.json (required)")
	today := fs.String("today", "", "date in YYYY-MM-DD format (required)")
	claudeMD := fs.String("claude-md", "", "path to CLAUDE.md for rune count (passed through to gen-spec)")
	batchDir := fs.String("batch", "", "apply every *.json proposal file in <dir> atomically in filename order (alternative to a single positional proposal file)")
	fs.Parse(args)

	if *domain == "" {
		return fmt.Errorf("--domain is required")
	}
	if *today == "" {
		return fmt.Errorf("--today is required (YYYY-MM-DD)")
	}

	domainDir, err := resolveDomain(*domain)
	if err != nil {
		return err
	}

	if *batchDir != "" {
		return cmdLandBatch(*batchDir, domainDir, *today, *claudeMD)
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("usage: hotam land <proposal.json> --domain <path> --today YYYY-MM-DD [--batch <dir>] [--claude-md <path>]")
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
	if err := confrontBeforeApply(domainDir, p); err != nil {
		return err
	}
	return landProposalValue(p, domainDir, *claudeMD, *today)
}

// cmdLandBatch is the --batch <dir> code path of `hotam land`: snapshot,
// apply every *.json in <dir> atomically, regen docs, re-verify, and roll
// back on failure. It is the batch counterpart to landProposalFile; the two
// share the same snapshot/genSpec/allViolations/rollback shape but differ in
// the apply step (ApplyBatch vs applyProposalValue).
func cmdLandBatch(batchDir, domainDir, today, claudeMDPath string) error {
	snapshot, err := snapshotGraphFiles(domainDir)
	if err != nil {
		return fmt.Errorf("pre-land snapshot failed, nothing landed: %w", err)
	}

	proposals, err := loadBatchDir(batchDir)
	if err != nil {
		return fmt.Errorf("apply step failed, nothing landed: %w", err)
	}
	// Advisory confront-at-gate (warn-only, never blocks): run AFTER every
	// proposal is parsed but BEFORE ApplyBatch mutates the graph, so all N
	// candidates are checked against the SAME starting snapshot the batch then
	// applies atomically.
	if err := confrontBatchSummary(domainDir, proposals); err != nil {
		return err
	}
	gp := graphPathForDomain(domainDir)
	if err := proposal.ApplyBatch(gp, today, proposals); err != nil {
		return fmt.Errorf("apply step failed, nothing landed: %w", err)
	}
	fmt.Printf("applied batch of %d proposals to %s\n", len(proposals), relPathForDisplay(gp))

	written, _, err := genSpec(domainDir, claudeMDPath, today, "")
	if err != nil {
		rerr := rollbackLand(domainDir, snapshot, claudeMDPath, today)
		return rolledBackError("doc regeneration failed", err, rerr)
	}
	fmt.Printf("regenerated %d doc(s)\n", len(written))

	violations, err := allViolations(domainDir)
	if err != nil {
		rerr := rollbackLand(domainDir, snapshot, claudeMDPath, today)
		return rolledBackError("violation check failed to run", err, rerr)
	}
	if len(violations) > 0 {
		for _, v := range violations {
			fmt.Printf("[%s] %s: %s\n", v.Check, v.ID, v.Message)
		}
		cause := fmt.Errorf("%d invariant violation(s) found after gen-spec (apply already validated the graph before writing it — this signals drift introduced by gen-spec or a concurrent change, not a bad proposal)", len(violations))
		rerr := rollbackLand(domainDir, snapshot, claudeMDPath, today)
		return rolledBackError("graph invalid after gen-spec", cause, rerr)
	}

	fmt.Println("landed: graph applied, docs regenerated, 0 violations")
	return nil
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
func landProposalFile(proposalFile, domainDir, claudeMDPath, today string) error {
	p, err := parseProposalFile(proposalFile)
	if err != nil {
		return fmt.Errorf("apply step failed, nothing landed: %w", err)
	}
	return landProposalValue(p, domainDir, claudeMDPath, today)
}

// landProposalValue runs the full land pipeline (snapshot, apply an
// already-parsed proposal, regen, re-verify, rollback-on-failure). It is the
// parse-free core shared by landProposalFile (which reads + parses the file)
// and the cmdLand single-file command branch (which parses once for an
// advisory confront check before landing). The transactional snapshot/rollback
// lives here so every caller that lands a single proposal shares one
// implementation.
func landProposalValue(p proposal.Proposal, domainDir, claudeMDPath, today string) error {
	snapshot, err := snapshotGraphFiles(domainDir)
	if err != nil {
		return fmt.Errorf("pre-land snapshot failed, nothing landed: %w", err)
	}

	gp, err := applyProposalValue(p, domainDir, today)
	if err != nil {
		return fmt.Errorf("apply step failed, nothing landed: %w", err)
	}
	fmt.Printf("applied %s %s to %s\n", p.Kind(), p.TargetAnchor(), relPathForDisplay(gp))

	written, _, err := genSpec(domainDir, claudeMDPath, today, "")
	if err != nil {
		rerr := rollbackLand(domainDir, snapshot, claudeMDPath, today)
		return rolledBackError("doc regeneration failed", err, rerr)
	}
	fmt.Printf("regenerated %d doc(s)\n", len(written))

	violations, err := allViolations(domainDir)
	if err != nil {
		rerr := rollbackLand(domainDir, snapshot, claudeMDPath, today)
		return rolledBackError("violation check failed to run", err, rerr)
	}
	if len(violations) > 0 {
		for _, v := range violations {
			fmt.Printf("[%s] %s: %s\n", v.Check, v.ID, v.Message)
		}
		cause := fmt.Errorf("%d invariant violation(s) found after gen-spec (apply already validated the graph before writing it — this signals drift introduced by gen-spec or a concurrent change, not a bad proposal)", len(violations))
		rerr := rollbackLand(domainDir, snapshot, claudeMDPath, today)
		return rolledBackError("graph invalid after gen-spec", cause, rerr)
	}

	fmt.Println("landed: graph applied, docs regenerated, 0 violations")
	return nil
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

	if _, _, err := genSpec(domainDir, claudeMDPath, today, ""); err != nil {
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
