.PHONY: build test test-race vet fmt fmt-check check gen

build:
	go build -o bin/hotam ./cmd/hotam

# -timeout 30m: cmd/hotam's e2e/killswitch tests spawn real `go build`/`go
# test` subprocesses (already kept to ONE shared binary build per process via
# testbinary_test.go's buildSharedHotamBinary — the per-test rebuild that
# used to dominate is already gone). The REMAINING wall-clock floor is a
# small set of tests that call t.Setenv (domain_flag_chain_test.go's
# TestCmdLand_OmittedDomainFallsThroughActiveDomainChain and siblings) and
# so cannot use t.Parallel (Go panics if t.Setenv races with a parallel
# sibling) — they run serially, each doing a real gen-spec regeneration /
# `land` verified_by gate against the full hotam-spec-self domain graph.
# Measured on this dev machine (2026-07, -count=1, no other load): TWO
# consecutive full `go test ./cmd/hotam/` runs took 1079s and 1174s
# (~18-19.6 min) — a 20m budget would leave only ~1-2 minutes of margin,
# not "large margin". 30m keeps real headroom without masking a genuine
# hang; if this package's wall-clock grows further, the next lever is
# de-serializing those t.Setenv tests (e.g. via HOTAM_SPEC_PROJECT_ROOT
# passed through cmd.Env to an exec'd subprocess instead of t.Setenv on the
# in-process test), not a bigger timeout.
test:
	go test -timeout 30m ./...

test-race:
	go test -race -timeout 30m ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

fmt-check:
	@files=$$(gofmt -l .); \
	if [ -n "$$files" ]; then \
		echo "gofmt would reformat:"; echo "$$files"; exit 1; \
	fi

gen:
	go run ./cmd/hotam gen-spec --domain domains/hotam-spec-self --claude-md CLAUDE.md

check: build fmt-check vet test-race
	@echo "All checks passed."
