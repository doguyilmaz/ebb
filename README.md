# ebb

See where your Mac's disk space went, why, and win it back safely.

ebb is one Go binary: a fast parallel scanner, a findings report that explains what is large and whether it is safe
to remove, a local web UI, and an MCP server for AI agents. Design: `docs/superpowers/specs/2026-10-17-ebb-design.md`.

## Build from source

```sh
make build      # bin/ebb
make check      # vet, tests, gofmt
```

Requires Go 1.26 and Xcode command line tools (cgo on macOS).

## Commands

```sh
ebb version
```
