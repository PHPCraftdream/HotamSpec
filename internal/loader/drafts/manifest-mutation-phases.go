//go:build ignore

// DRAFT — NOT LANDED (task #341, R5-manifest-object). This file is the Phase 2/3
// architecture note for the manifest-as-typed-object migration; it is excluded
// from compilation (//go:build ignore) so it never enters the build and serves
// only as a durable plan, exactly as task #340's R-debt-ratchet-no-growth.go
// draft stayed a ready-to-paste registry literal until a resolver-approved
// landing wave.
//
// CONTEXT. Phase 1 (this task, #341) landed ONLY the byte-identity foundation:
// the typed DomainManifest struct + LoadManifest/WriteManifest + a
// byte-identical round-trip proof on the two real committed manifests
// (internal/loader/manifest.go). It proved the "object is primary, JSON is a
// projection" discipline — the same discipline the requirements-as-code wave
// (#343-#347) already established for Requirements via internal/selfspec +
// hotam sync-self — can be applied to manifest.json. Phase 1 deliberately did
// NOT flip authority, add a mutation layer, or block hand-editing.
//
// PHASE 2 — typed mutation layer (ProposedManifestChange). Mirror
// internal/proposal's shape for graph nodes: a typed Proposed*-style object
// with Validate/Mutate/Apply methods that edit a DomainManifest through
// field-typed accessors (SetPurpose, AddGoal, DeclareParent, …) instead of raw
// map[string]interface{} JSON surgery. Each mutation round-trips through
// LoadManifest → mutate → WriteManifest, inheriting Phase 1's byte-identity
// guarantee. This is the manifest analogue of how MergeIntoGraph applies
// registered Requirements onto a graph.
//
// PHASE 3 — hand-edit guard (R-no-hand-edit-manifest). The R-no-hand-edit-graph
// analogue: refuse a direct hand-edit of manifest.json that bypasses the typed
// path. Today graph.json is protected by graph.lock (check_graph_lock_pins_*)
// — manifest.json has no such pin. Phase 3 would add a manifest.lock content
// hash (or reuse the existing ResolveDiscipline one-way-door concern already
// documented in loader.go) plus a check that fails closed when manifest.json's
// bytes diverge from what the typed path last wrote. This makes manifest.json
// editing a conscious, reviewed diff — the same discipline graph.json already
// enforces.
//
// LANDING these is a separate, resolver-approved step per phase, NOT part of
// task #341. When Phase 2 lands, delete this note (or move it into a landed
// requirement's doc comment).
package loader
