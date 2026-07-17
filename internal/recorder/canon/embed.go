package hotamspec

import _ "embed"

// Source is this package's OWN hotamspec.go file, embedded verbatim at
// build time. It exists so a DIFFERENT package (internal/generator's
// recorder-vendoring code) can obtain the exact canonical bytes without
// re-reading the file from an assumed relative path at runtime (fragile:
// would break the moment the binary runs from a different working
// directory or the repo layout shifts) and without go:embed's own
// restriction against a "../" pattern reaching across a package boundary
// (go:embed patterns may not contain ".." -- see `go doc embed`). By
// embedding the file INSIDE its own package and exporting the result as a
// plain string, any importer gets a build-time-frozen, byte-exact copy no
// matter where the importing code runs from.
//
// This must stay a single, whole-file embed of hotamspec.go ONLY (not a
// directory glob) so Source's content is exactly the recorder's own public
// API surface -- this file (embed.go) and hotamspec_test.go are
// deliberately excluded: the former is vendoring plumbing, not part of the
// API a consumer domain's tests call, and the latter is this package's own
// test suite, never meant to ship into a consumer's spec/hotamspec/.
//
//go:embed hotamspec.go
var Source string
