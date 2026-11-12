VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X github.com/doguyilmaz/ebb/internal/cli.Version=$(VERSION)

ui:
	cd ui && bun install --frozen-lockfile && bun run build

build: ui
	MACOSX_DEPLOYMENT_TARGET=12.0 \
	  CGO_CFLAGS="$${CGO_CFLAGS:--O2 -g} -mmacosx-version-min=12.0" \
	  CGO_LDFLAGS="$${CGO_LDFLAGS:--O2 -g} -mmacosx-version-min=12.0" \
	  go build -ldflags "$(LDFLAGS)" -o bin/ebb ./cmd/ebb

test:
	go test ./...

check:
	go vet ./...
	go test ./...
	@out=$$(gofmt -l $$(find . -name '*.go' -not -path './ui/*')); test -z "$$out" || (echo "$$out"; exit 1)
	cd ui && bun run typecheck && bun test
	@if [ -f macos/Package.swift ]; then swift test --package-path macos; fi

e2e:
	scripts/e2e.sh

bench:
	go test -run '^$$' -bench . -benchmem ./internal/walk ./internal/tree

native: ui
	scripts/native-app.sh

native-check:
	swift test --package-path macos

native-smoke:
	scripts/native-smoke.sh

.PHONY: ui build test check e2e bench native native-check native-smoke
