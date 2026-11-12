GO ?= go
BIN := bin/shardwave

# Pinned so the laptop and CI run the same linter. After `brew install golangci-lint`:
#   make check GOLANGCI=golangci-lint
GOLANGCI_VERSION := v2.15.0
GOLANGCI ?= $(GO) run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_VERSION)

.PHONY: all check build test vet lint fmt tidy windows run-a run-b clean

all: check

# What CI runs. Run it before handing code over for review.
check: vet lint test

# NOTE: `build`, `run-a`, `run-b`, `windows` fail until cmd/shardwave exists (Sprint 001, step 7).
build:
	$(GO) build -o $(BIN) ./cmd/shardwave

# -race is always on.
test:
	$(GO) test -race -count=1 ./...

vet:
	$(GO) vet ./...

# Linters plus formatting check, per .golangci.yml.
lint:
	$(GOLANGCI) run

# Rewrites files: gofmt, gofumpt, goimports.
fmt:
	$(GOLANGCI) fmt

tidy:
	$(GO) mod tidy

windows:
	GOOS=windows GOARCH=amd64 $(GO) build -o dist/shardwave.exe ./cmd/shardwave

# Two nodes on localhost. Keys land in .shardwave/ (gitignored).
run-a: build
	$(BIN) node --listen 127.0.0.1:7000 --peers 127.0.0.1:7001 --key .shardwave/a.key

run-b: build
	$(BIN) node --listen 127.0.0.1:7001 --peers 127.0.0.1:7000 --key .shardwave/b.key

clean:
	rm -rf bin dist
