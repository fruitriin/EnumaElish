BINARY := ccchain
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: build test test-integration test-fixture test-all bench vet clean all check

# Default: full quality gate
all: check

# Quality gate (build + vet + all tests)
check: vet test-all build
	@echo "=== All checks passed ==="

build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/ccchain

# Unit tests only
test:
	go test ./...

# Integration tests (default ruleset, ~220 cases)
test-integration:
	go test ./internal/eval/ -run TestIntegration -v

# Fixture combination tests (commands × rulesets)
test-fixture:
	go test ./internal/eval/ -run TestFixture -v

# All tests
test-all:
	go test ./... -count=1

bench:
	go test -bench=. -benchmem ./internal/dsl/ ./internal/eval/ ./internal/shell/

vet:
	go vet ./...

clean:
	rm -f $(BINARY)

# Cross-compile for release
.PHONY: release
release:
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-darwin-amd64 ./cmd/ccchain
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)-darwin-arm64 ./cmd/ccchain
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-linux-amd64 ./cmd/ccchain
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)-linux-arm64 ./cmd/ccchain
