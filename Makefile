VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X github.com/doguyilmaz/ebb/internal/cli.Version=$(VERSION)

build:
	go build -ldflags "$(LDFLAGS)" -o bin/ebb ./cmd/ebb

test:
	go test ./...

check:
	go vet ./...
	go test ./...
	@out=$$(gofmt -l $$(find . -name '*.go' -not -path './ui/*')); test -z "$$out" || (echo "$$out"; exit 1)

bench:
	go test -run '^$$' -bench . -benchmem ./internal/walk ./internal/tree

.PHONY: build test check bench
