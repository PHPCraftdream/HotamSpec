package ontology

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

const (
	ConflictDETECTED      = "DETECTED"
	ConflictACKNOWLEDGED  = "ACKNOWLEDGED"
	ConflictDECIDEDPrefix = "DECIDED"
	ConflictREVISITPrefix = "REVISIT_WHEN"
	ConflictHELDPrefix    = "HELD"
)

var ConflictUnresolvedLifecycle = map[string]struct{}{
	ConflictDETECTED:     {},
	ConflictACKNOWLEDGED: {},
}

var conflictWhitespace = regexp.MustCompile(`\s+`)

func ConflictIdentity(axis, context string) string {
	normctx := conflictWhitespace.ReplaceAllString(strings.ToLower(strings.TrimSpace(context)), " ")
	sum := sha256.Sum256([]byte(axis + "\x00" + normctx))
	return "C-" + hex.EncodeToString(sum[:])[:8]
}

type Variant struct {
	ID       string `json:"id"`
	Behavior string `json:"behavior"`
	Implies  string `json:"implies"`
	Costs    string `json:"costs"`
}

type Conflict struct {
	ID               string    `json:"id"`
	Axis             string    `json:"axis"`
	Context          string    `json:"context"`
	Members          []string  `json:"members"`
	Resolver         string    `json:"resolver"`
	Lifecycle        string    `json:"lifecycle"`
	SharedAssumption *string   `json:"shared_assumption"`
	Derived          []string  `json:"derived"`
	RevisitMarker    string    `json:"revisit_marker"`
	DecidedBy        string    `json:"decided_by"`
	Variants         []Variant `json:"variants"`
	Signoff          *Signoff  `json:"signoff"`
	CreatedAt        string    `json:"created_at"`
	DecidedAt        string    `json:"decided_at"`
	DeclOrder        int       `json:"decl_order"`
	// SourceRefs lists supporting provenance references for this conflict
	// (a doc path, a ticket id, a decision record, ...) -- the same
	// free-form, unresolved-by-invariant shape Requirement.SourceRefs
	// already establishes (internal/ontology/requirement.go): no check_*
	// validates these entries resolve to anything real, mirroring
	// Requirement's own precedent rather than inventing a stricter rule for
	// this node type alone. Purely additive and optional (omitempty) -- a
	// Conflict that predates this field has no source_refs and its JSON is
	// unchanged.
	SourceRefs []string `json:"source_refs,omitempty"`
}

func (c Conflict) IsUnresolved() bool {
	_, ok := ConflictUnresolvedLifecycle[c.Lifecycle]
	return ok
}

func (c Conflict) IsDecided() bool {
	return strings.HasPrefix(c.Lifecycle, ConflictDECIDEDPrefix)
}

func (c Conflict) IsHeld() bool {
	return strings.HasPrefix(c.Lifecycle, ConflictHELDPrefix)
}
