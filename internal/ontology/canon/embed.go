package hotamontology

import _ "embed"

// RequirementSource is requirement.go, embedded verbatim at build time --
// same rationale as internal/recorder/canon/embed.go's Source: a DIFFERENT
// package (internal/ontology/vendor) needs the exact canonical bytes without
// re-reading the file from an assumed relative path at runtime, and without
// go:embed's own restriction against a "../" pattern reaching across a
// package boundary.
//
//go:embed requirement.go
var RequirementSource string

// RegistrySource is registry.go, embedded verbatim at build time -- same
// rationale as RequirementSource above, kept as a second, separate embed
// (not a directory glob) so this package's own embed.go and any future test
// file stay excluded from what actually ships into a consumer domain's
// vendored spec/hotamontology/.
//
//go:embed registry.go
var RegistrySource string

// StakeholderSource is stakeholder.go, embedded verbatim -- same rationale as
// RegistrySource (separate embed, canon files only).
//
//go:embed stakeholder.go
var StakeholderSource string
