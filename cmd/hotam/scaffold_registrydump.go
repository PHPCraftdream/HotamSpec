package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	registrydumpvendor "github.com/PHPCraftdream/HotamSpec/internal/registrydump/vendor"
)

// cmdScaffoldRegistrydump implements `hotam scaffold-registrydump --domain
// <dir>`: it writes <domainDir>/spec/registrydump/main.go, a minimal Go
// program that imports the domain's own spec/ module root package (expected
// to declare `var Requirements = hotamontology.New[hotamontology.Requirement]()`,
// mirroring internal/selfspec.Requirements' shape one-for-one — see
// R-domain-founded-in-wave-order step 6 in the root CLAUDE.md) plus the
// vendored spec/hotamontology package, and prints
// `json.Marshal(Requirements.All())` to stdout.
//
// This is the domain-side half of the module-boundary bridge `hotam
// sync-domain` (task #366) needs: sync-domain itself lives in the ENGINE's
// own Go module and can never import a consumer domain's spec/ module
// directly (NEW-2-bis — no cross-module `replace`), so it instead spawns
// `go run ./registrydump` INSIDE the domain's own spec/ module (the same
// module-boundary-crossing principle internal/gate/test_exec.go already uses
// to execute a domain's verified_by tests) and reads the JSON it prints back
// on stdout.
//
// Modeled directly on `hotam vendor-ontology` (cmd/hotam/vendor_ontology.go)
// for its two hard requirements: (1) spec/go.mod must already exist — this
// command never mints a new Go module on the caller's behalf; (2)
// spec/hotamontology/ must already be vendored (`hotam vendor-ontology` run
// first) — registrydump's own generated source imports it directly, so
// scaffolding it before the ontology mirror exists would produce a Go file
// that cannot compile. Idempotent: re-running always overwrites
// spec/registrydump/main.go unconditionally with the current template.
func cmdScaffoldRegistrydump(args []string) error {
	fs := newFlagSet("scaffold-registrydump")
	domain := fs.String("domain", "", "domain directory (default: "+defaultDomainRel+")")
	fs.Parse(args)

	domainDir, err := resolveDomain(*domain)
	if err != nil {
		return err
	}

	written, err := scaffoldRegistrydump(domainDir)
	if err != nil {
		return err
	}
	fmt.Println(relPathForDisplay(written))
	return nil
}

// scaffoldRegistrydump does the actual write and returns the path written,
// so tests can call it directly without going through flag parsing / stdout.
//
// Requires <domainDir>/spec/go.mod (the domain's own spec/ Go module must
// already exist — same hard requirement as vendorOntology/vendorRecorder)
// AND <domainDir>/spec/hotamontology/requirement.go (the vendored ontology
// mirror `hotam vendor-ontology` writes — registrydump's generated source
// imports this package by its module-relative path, so scaffolding it first
// would leave a non-compiling stub).
func scaffoldRegistrydump(domainDir string) (string, error) {
	specDir := filepath.Join(domainDir, "spec")
	specGoMod := filepath.Join(specDir, "go.mod")
	modulePath, err := readGoModModulePath(specGoMod)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("scaffold-registrydump: %s does not exist -- the domain's spec/ tree must already be its own Go module (its own go.mod) before registrydump can be scaffolded into it; see PLAN-authored-spec-discipline.md §3", specGoMod)
		}
		return "", fmt.Errorf("scaffold-registrydump: %w", err)
	}

	vendoredOntology := filepath.Join(specDir, "hotamontology", "requirement.go")
	if _, err := os.Stat(vendoredOntology); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("scaffold-registrydump: %s does not exist -- run `hotam vendor-ontology --domain %s` first so registrydump's generated source has a spec/hotamontology package to import", vendoredOntology, domainDir)
		}
		return "", fmt.Errorf("scaffold-registrydump: stat %s: %w", vendoredOntology, err)
	}

	target := filepath.Join(specDir, "registrydump", "main.go")
	withStakeholders := declaresStakeholders(specDir)
	if withStakeholders {
		vendoredStk := filepath.Join(specDir, "hotamontology", "stakeholder.go")
		if _, err := os.Stat(vendoredStk); err != nil {
			return "", fmt.Errorf("scaffold-registrydump: spec/ declares Stakeholders but %s is missing -- re-run `hotam vendor-ontology --domain %s` to vendor the Stakeholder mirror", vendoredStk, domainDir)
		}
	}
	content := []byte(registrydumpSource(modulePath, withStakeholders))
	if err := writeFileMkdir(target, content); err != nil {
		return "", fmt.Errorf("scaffold-registrydump: %w", err)
	}
	return target, nil
}

// registrydumpBanner re-exports registrydumpvendor.Banner under this file's
// existing local name -- the do-not-edit banner prepended to the generated
// spec/registrydump/main.go, the same rhetorical shape as
// internal/ontology/vendor.Banner / internal/recorder/vendor.Banner applied
// to a GENERATED (not vendored-from-canon) file: hand-editing it will be
// silently discarded the next time `hotam scaffold-registrydump` runs. The
// literal moved into internal/registrydump/vendor (a shared leaf package)
// so internal/gate/model_scan.go's unified generated-file exclusion can
// recognize it too, without internal/gate importing cmd/hotam (a cycle --
// see that new package's own doc comment for the full reasoning).
const registrydumpBanner = registrydumpvendor.Banner

// registrydumpTemplate is the generated main.go body, minus the banner.
// {{MODULE}} is substituted with the domain's spec/ module's own declared
// module path (read from spec/go.mod) so the import statement resolves
// regardless of what name the domain author chose for their module.
const registrydumpTemplate = `package main

import (
	"encoding/json"
	"fmt"
	"os"

	spec "{{MODULE}}"
)

// main prints json.Marshal(spec.Requirements.All()) to stdout -- the exact
// []hotamontology.Requirement slice ` + "`hotam sync-domain`" + ` reads back and
// projects onto this domain's graph.json.
func main() {
	data, err := json.Marshal(spec.Requirements.All())
	if err != nil {
		fmt.Fprintf(os.Stderr, "registrydump: marshal Requirements.All(): %v\n", err)
		os.Exit(1)
	}
	os.Stdout.Write(data)
}
`

// registrydumpEnvelopeTemplate is the variant for a domain that ALSO declares
// `var Stakeholders = hotamontology.New[hotamontology.Stakeholder]()`: stdout
// is the JSON envelope {"requirements":[...],"stakeholders":[...]} instead of
// a bare Requirement array. sync-domain accepts both (a leading '[' is the
// legacy array, '{' the envelope), so a domain without Stakeholders keeps the
// legacy template and an already-scaffolded old main.go keeps working.
const registrydumpEnvelopeTemplate = `package main

import (
	"encoding/json"
	"fmt"
	"os"

	spec "{{MODULE}}"
)

// main prints {"requirements": spec.Requirements.All(), "stakeholders":
// spec.Stakeholders.All()} -- the envelope ` + "`hotam sync-domain`" + ` reads back and
// projects onto this domain's graph.json.
func main() {
	data, err := json.Marshal(struct {
		Requirements any ` + "`json:\"requirements\"`" + `
		Stakeholders any ` + "`json:\"stakeholders\"`" + `
	}{spec.Requirements.All(), spec.Stakeholders.All()})
	if err != nil {
		fmt.Fprintf(os.Stderr, "registrydump: marshal registries: %v\n", err)
		os.Exit(1)
	}
	os.Stdout.Write(data)
}
`

// declaresStakeholders reports whether the domain's spec/ root package
// declares a `Stakeholders` registry variable (best-effort source scan of
// non-test *.go files directly in specDir).
func declaresStakeholders(specDir string) bool {
	files, _ := filepath.Glob(filepath.Join(specDir, "*.go"))
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		if data, err := os.ReadFile(f); err == nil && stakeholdersDeclRe.Match(data) {
			return true
		}
	}
	return false
}

var stakeholdersDeclRe = regexp.MustCompile(`(?m)^(var\s+|\s+)Stakeholders\s*(=|\*?hotamontology\.)`)

// registrydumpSource returns the full byte-for-byte content
// spec/registrydump/main.go should hold for a domain whose spec/ module's
// own go.mod declares modulePath; withStakeholders selects the envelope
// variant.
func registrydumpSource(modulePath string, withStakeholders bool) string {
	tpl := registrydumpTemplate
	if withStakeholders {
		tpl = registrydumpEnvelopeTemplate
	}
	return registrydumpBanner + "\n" + strings.ReplaceAll(tpl, "{{MODULE}}", modulePath)
}

// readGoModModulePath reads the `module <path>` directive out of the go.mod
// at path, returning just <path>. Returns an os.IsNotExist-satisfying error
// when path itself does not exist (propagated to the caller unchanged so it
// can distinguish "no go.mod yet" from "go.mod exists but has no module
// directive" / other read errors).
func readGoModModulePath(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("scan %s: %w", path, err)
	}
	return "", fmt.Errorf("%s has no `module <path>` directive", path)
}
