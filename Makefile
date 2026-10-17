VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X github.com/doguyilmaz/ebb/internal/cli.Version=$(VERSION)

ui:
	cd ui && bun install --frozen-lockfile && bun run build

build: ui
	go build -ldflags "$(LDFLAGS)" -o bin/ebb ./cmd/ebb

test:
	go test ./...

check:
	go vet ./...
	go test ./...
	@out=$$(gofmt -l $$(find . -name '*.go' -not -path './ui/*')); test -z "$$out" || (echo "$$out"; exit 1)
	cd ui && bun run typecheck && bun test

e2e:
	scripts/e2e.sh

bench:
	go test -run '^$$' -bench . -benchmem ./internal/walk ./internal/tree

.PHONY: ui build test check e2e bench
