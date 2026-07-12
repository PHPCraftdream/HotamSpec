package main

import (
	"fmt"

	"github.com/PHPCraftdream/HotamSpecGo/internal/proposal"
)

// cmdLand implements `hotam land`: the single-command pipeline that keeps
// graph.json and its rendered docs/gen/*.md in sync, closing the gap
// described by TaskList P1-4 — internal/proposal/apply.go's Apply() writes
// only graph.json + graph.lock and never regenerates docs, so every
// standalone `hotam apply-proposal` leaves the graph and docs/gen/CLAUDE.md/
// AGENTS.md diverged until someone remembers to run gen-spec by hand.
//
// land runs three steps in sequence, reusing the exact same code paths as
// the standalone commands (applyProposalFile / ApplyBatch / genSpec /
// allViolations) so its behavior is provably identical to running them one
// at a time:
//
//  1. apply the proposal — a single positional file (applyProposalFile) or a
//     whole directory of proposals applied atomically via --batch
//     (loadBatchDir + proposal.ApplyBatch). Strict decode; Apply/ApplyBatch
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

	// (a) apply: batch (--batch <dir>, atomic all-or-nothing) or a single
	// positional proposal file. Either way a successful return means the
	// graph on disk is structurally valid; only the docs remain stale until
	// step (b) runs.
	if *batchDir != "" {
		proposals, err := loadBatchDir(*batchDir)
		if err != nil {
			return fmt.Errorf("apply step failed, nothing landed: %w", err)
		}
		gp := graphPathForDomain(domainDir)
		if err := proposal.ApplyBatch(gp, *today, proposals); err != nil {
			return fmt.Errorf("apply step failed, nothing landed: %w", err)
		}
		fmt.Printf("applied batch of %d proposals to %s\n", len(proposals), relPathForDisplay(gp))
	} else {
		if fs.NArg() < 1 {
			return fmt.Errorf("usage: hotam land <proposal.json> --domain <path> --today YYYY-MM-DD [--batch <dir>] [--claude-md <path>]")
		}
		proposalFile := fs.Arg(0)
		p, gp, err := applyProposalFile(proposalFile, domainDir, *today)
		if err != nil {
			return fmt.Errorf("apply step failed, nothing landed: %w", err)
		}
		fmt.Printf("applied %s %s to %s\n", p.Kind(), p.TargetAnchor(), relPathForDisplay(gp))
	}

	// (b) gen-spec: regenerate docs/gen/*.md + graph.json from the graph
	// apply-proposal just wrote, so docs never drift from the graph they
	// describe.
	written, err := genSpec(domainDir, *claudeMD)
	if err != nil {
		return fmt.Errorf("proposal applied but doc regeneration failed: %w", err)
	}
	fmt.Printf("regenerated %d doc(s)\n", len(written))

	// (c) all-violations: safety-net re-verification, not the primary gate
	// (see the function doc above) — apply already rejected an
	// invariant-breaking write before anything touched disk.
	violations, err := allViolations(domainDir)
	if err != nil {
		return fmt.Errorf("proposal applied and docs regenerated but violation check failed to run: %w", err)
	}
	if len(violations) > 0 {
		for _, v := range violations {
			fmt.Printf("[%s] %s: %s\n", v.Check, v.ID, v.Message)
		}
		return fmt.Errorf("landed but graph invalid: %d invariant violation(s) found after gen-spec (apply already validated the graph before writing it — this signals drift introduced by gen-spec or a concurrent change, not a bad proposal)", len(violations))
	}

	fmt.Println("landed: graph applied, docs regenerated, 0 violations")
	return nil
}
