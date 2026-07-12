package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/PHPCraftdream/HotamSpecGo/internal/proposal"
)

func cmdApplyProposal(args []string) error {
	fs := newFlagSet("apply-proposal")
	domain := fs.String("domain", "", "domain directory containing graph.json (required)")
	today := fs.String("today", "", "date in YYYY-MM-DD format (required)")
	fs.Parse(args)

	if fs.NArg() < 1 {
		return fmt.Errorf("usage: hotam apply-proposal <proposal.json> --domain <path> --today YYYY-MM-DD")
	}
	if *domain == "" {
		return fmt.Errorf("--domain is required")
	}
	if *today == "" {
		return fmt.Errorf("--today is required (YYYY-MM-DD)")
	}
	proposalFile := fs.Arg(0)

	domainDir, err := resolveDomain(*domain)
	if err != nil {
		return err
	}
	p, gp, err := applyProposalFile(proposalFile, domainDir, *today)
	if err != nil {
		return err
	}
	fmt.Printf("applied %s %s to %s\n", p.Kind(), p.TargetAnchor(), relPathForDisplay(gp))
	fmt.Println("docs not regenerated; run hotam gen-spec or use hotam land")
	return nil
}

// applyProposalFile reads a proposal JSON file from disk, strictly decodes
// it, and applies it to domainDir's graph.json (internal/proposal.Apply
// already re-verifies invariants against the mutated graph BEFORE writing —
// see internal/proposal/apply.go — so a successful return here means the
// graph on disk is structurally valid, just not yet re-rendered into
// docs/gen/). It returns the decoded proposal and the graph path that was
// written, so callers (cmdApplyProposal, cmdLand) can report what happened
// without re-parsing.
func applyProposalFile(proposalFile, domainDir, today string) (proposal.Proposal, string, error) {
	data, err := os.ReadFile(proposalFile)
	if err != nil {
		return nil, "", fmt.Errorf("read proposal %s: %w", proposalFile, err)
	}
	p, err := parseProposal(data)
	if err != nil {
		return nil, "", err
	}
	gp := graphPathForDomain(domainDir)
	if err := proposal.Apply(gp, today, p); err != nil {
		return nil, "", err
	}
	return p, gp, nil
}

func parseProposal(data []byte) (proposal.Proposal, error) {
	var probe struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, fmt.Errorf("parse proposal kind: %w", err)
	}
	switch probe.Kind {
	case proposal.KindRequirement:
		var p proposal.ProposedRequirement
		return p, unmarshalProposal(data, &p)
	case proposal.KindConflictTransition:
		var p proposal.ProposedConflictTransition
		return p, unmarshalProposal(data, &p)
	case proposal.KindRejection:
		var p proposal.ProposedRejection
		return p, unmarshalProposal(data, &p)
	case proposal.KindConflict:
		var p proposal.ProposedConflict
		return p, unmarshalProposal(data, &p)
	case proposal.KindOperatorBudget:
		var p proposal.ProposedOperatorBudget
		return p, unmarshalProposal(data, &p)
	case proposal.KindAxis:
		var p proposal.ProposedAxis
		return p, unmarshalProposal(data, &p)
	case proposal.KindStakeholder:
		var p proposal.ProposedStakeholder
		return p, unmarshalProposal(data, &p)
	case proposal.KindAssumption:
		var p proposal.ProposedAssumption
		return p, unmarshalProposal(data, &p)
	case proposal.KindAssumptionTransition:
		var p proposal.ProposedAssumptionTransition
		return p, unmarshalProposal(data, &p)
	case proposal.KindConflictMemberUpdate:
		var p proposal.ProposedConflictMemberUpdate
		return p, unmarshalProposal(data, &p)
	case proposal.KindEntityType:
		var p proposal.ProposedEntityType
		return p, unmarshalProposal(data, &p)
	case proposal.KindReviewMark:
		var p proposal.ProposedReviewMark
		return p, unmarshalProposal(data, &p)
	default:
		return nil, fmt.Errorf("unknown proposal kind %q (expected one of: %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)",
			probe.Kind,
			proposal.KindRequirement, proposal.KindConflictTransition, proposal.KindRejection,
			proposal.KindConflict, proposal.KindOperatorBudget, proposal.KindAxis,
			proposal.KindStakeholder, proposal.KindAssumption, proposal.KindAssumptionTransition,
			proposal.KindConflictMemberUpdate, proposal.KindEntityType, proposal.KindReviewMark)
	}
}

// unmarshalProposal decodes a proposal JSON object into target with a strict
// decoder: any field the target struct does not declare a json tag for is a
// hard error (DisallowUnknownFields), so a stale camelCase key or a typo'd
// snake_case key fails loudly instead of silently leaving the Go field at
// its zero value. The top-level "kind" selector field is legal on every
// proposal but is not part of any Proposed* struct, so it is stripped
// before the strict decode runs.
func unmarshalProposal(data []byte, target any) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return fmt.Errorf("parse proposal fields: %w", err)
	}
	delete(fields, "kind")

	stripped, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("parse proposal fields: %w", err)
	}

	dec := json.NewDecoder(bytes.NewReader(stripped))
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		return fmt.Errorf("parse proposal fields: %w", err)
	}
	return nil
}
