BINARY := ccchain
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: build test bench vet clean all

all: vet test build

build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/ccchain

test:
	go test ./...

bench:
	go test -bench=. -benchmem ./internal/dsl/

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
