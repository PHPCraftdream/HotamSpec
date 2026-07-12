.PHONY: build test test-race vet fmt fmt-check check gen

build:
	go build -o bin/hotam ./cmd/hotam

test:
	go test ./...

test-race:
	go test -race ./...

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
