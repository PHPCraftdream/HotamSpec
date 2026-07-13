package main

import "github.com/PHPCraftdream/HotamSpec/internal/methodology"

// This file is the ONLY place cmd/hotam reaches into internal/methodology's
// Tool registry to attach a real Run function. internal/methodology must stay
// free of any dependency on cmd/hotam (it is package main; methodology is an
// importable library package other tools, e.g. the generator, also depend
// on) — so tools_data.go declares every Implemented Tool with Run: nil, and this
// file patches those entries in place via registry.Update once cmd/hotam
// itself (which already links in every cmd* function below) is loaded.
//
// Architecture note (see ticket P1-6 / TaskList #19): this is option (a)
// from the ticket brief. The alternative — giving internal/methodology a
// function-value field populated by literal closures at declaration time —
// was rejected because Tool.Run's whole point is "this actually invokes the
// CLI command", and the CLI commands (cmdGenSpec, cmdWhatNow, ...) live in
// package main by construction (main.go's flag/arg handling, os.Exit, etc.).
// Importing cmd/hotam from internal/methodology to reach them would be
// backwards (a leaf library depending on the top-level binary) and importing
// internal/methodology from cmd/hotam already happens today for the doc
// generator, so wiring the other direction here — main reaching down into
// methodology's registry after both packages are loaded — is the only
// acyclic option. registry.Update (internal/registry/registry.go) was added
// alongside this file to make that patch possible without exposing the
// registry's internal map.
//
// init() order: Go guarantees internal/methodology's own init() (which
// populates Tools via tools_data.go's MustRegister calls) runs before this
// package's init(), because cmd/hotam imports internal/methodology
// (dependency inits run first). So every name Update() touches below is
// already registered by the time this runs.
func init() {
	wireToolRun("gen_spec", cmdGenSpec)
	wireToolRun("what_now", cmdWhatNow)
	wireToolRun("apply_proposal", cmdApplyProposal)
	wireToolRun("gate", cmdGate)
	wireToolRun("all_violations", cmdAllViolations)
	wireToolRun("req", cmdReq)
	wireToolRun("due", cmdDue)
	wireToolRun("status", cmdStatus)
	wireToolRun("inspect", cmdInspect)
	wireToolRun("confront", cmdConfront)
	wireToolRun("land", cmdLand)
}

// wireToolRun patches the Run field of the already-registered Tool named
// name, leaving every other field (Command, Canon, Purpose, Status) exactly
// as tools_data.go declared it. It panics (via registry.Update) if name was
// never registered — a mismatch here is a wiring bug (an Implemented tool renamed
// or removed in tools_data.go without updating this file, or vice versa),
// and TestToolWiring_EveryImplementedToolHasRun in tool_wiring_test.go exists to
// catch the inverse mistake: an Implemented tool that this file forgot to wire.
func wireToolRun(name string, run func(args []string) error) {
	tool, ok := methodology.Tools.Get(name)
	if !ok {
		panic("tool_wiring: no such registered tool " + name)
	}
	updated := *tool
	updated.Run = run
	methodology.Tools.Update(name, updated)
}
