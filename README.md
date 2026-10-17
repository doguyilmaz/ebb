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
ebb                         # overview: disk use, top findings (reuses a scan younger than an hour)
ebb scan [path]             # scan the whole data volume, or a folder (--threads, --gentle)
ebb findings                # what takes space, why, how safe it is to remove (--category)
ebb top [path] [--files]    # largest folders or files under a path
ebb find <pattern>          # glob or substring over names
ebb report --format html -o ebb.html   # also text, json, csv
ebb history · ebb diff      # past scans, and what grew between two of them
ebb version
```

Every command takes `--json`. Grant Full Disk Access to your terminal (System Settings → Privacy & Security) to let
ebb see app containers and Mail/Messages data; without it the report shows those bytes as "not visible".

State lives in `~/.local/state/ebb` (last scan, history snapshots), config in `~/.config/ebb/config.toml`, your own
rules in `~/.config/ebb/rules/*.toml`.
