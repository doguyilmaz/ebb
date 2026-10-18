<p align="center"><img src="assets/brand/icon-spectrum-1024.png" width="128" alt=""></p>
<h1 align="center">ebb</h1>
<p align="center">See where your Mac's disk space went and win it back safely.</p>
<p align="center"><a href="https://ebb.doguyilmaz.com">ebb.doguyilmaz.com</a></p>

ebb is a single binary for macOS 12 or later. It has a fast parallel scanner and findings that explain what is large and whether it is safe to remove. It also has a local web window, a background service, a CLI and an MCP server for AI agents. Nothing leaves your Mac.

## Install

```sh
brew install doguyilmaz/tap/ebb
```

Or download the archive from [Releases](https://github.com/doguyilmaz/ebb/releases/latest) (one binary for Apple silicon and Intel) and put `ebb` on your PATH.

## Start

```sh
ebb serve --open      # the window, on 127.0.0.1:47340
ebb daemon install    # or keep it in the background: starts at login, keeps scans fresh, warns when space runs low
ebb                   # the overview in the terminal
```

The first scan reads the whole data volume, in about a minute for 6 million files. Later scans read only the folders that changed, usually in seconds.

### Full Disk Access

Without Full Disk Access, macOS hides app containers and Mail, Messages and Safari data. ebb shows that space as "Not visible". Grant it in System Settings → Privacy & Security → Full Disk Access to whatever runs ebb:

- **Without the background service:** your terminal.
- **With the background service:** the ebb binary itself. Settings shows its exact path. Homebrew installs every version to a new path, so add it again after an upgrade.

## What it finds

- **Caches that rebuild themselves:** npm, Bun, pnpm, Yarn, Cargo, Go, Gradle, CocoaPods, pip and more. Also browser caches and the build folders of projects untouched for months.
- **Xcode:** DerivedData, archives, device support, unavailable simulators, old runtimes and docsets.
- **Other tools:** Docker data, old IDE versions, logs, installers in Downloads, Trash on other volumes and local snapshots.
- **Apps:** what each app keeps in your Library and what removed apps left behind.
- **Your files:** duplicates and the largest and oldest files.

Each finding says why it is there, what removing it costs, how safe it is and how to remove it (the tool's own command when there is one). The window adds:

- the disk as a sunburst, treemap, list or file table;
- what changed since yesterday, last week or last month;
- where space comes from and which packages your projects share.

## Safety

- **Plan first.** Every cleanup is a plan before anything happens. Each item shows its action (Trash, delete, the tool's command or a manual step), its size, its safety and anything blocking it.
- **What it never touches:**
  - anything outside the scanned folder;
  - protected paths: system folders, the top-level home folders, Keychains, iCloud and other cloud storage folders and your protect list;
  - anything that changed since the plan was made;
  - an app while it is running.
- **Undo.** Your own files go to the Trash. `ebb undo` puts them back. `ebb ops` lists everything ebb did.
- **No root.** ebb refuses to run as root.
- **Agents can only plan.** An AI agent can make a plan, but only a person approves it, in the window or with `ebb approve`. Plans made only of rebuildable caches are the exception.

## Commands

```sh
ebb scan [path]                  # the data volume or one folder (--gentle throttles I/O)
ebb findings                     # what takes space, why, how safe (--category)
ebb top [path] [--files]         # largest folders or files
ebb find <pattern>               # glob or substring over names
ebb apps · ebb uninstall <app>   # apps with their Library data; uninstall to the Trash
ebb dupes [--min 10MiB]          # identical files in your own folders
ebb clean <finding|category> [--preset dev-caches] [--path p] [--dry-run] [--yes]
ebb ops · ebb undo [op]          # what ebb did; put the last trashed item back
ebb history · ebb diff           # past scans and what grew between two
ebb report --format html -o ebb.html   # also text, json, csv
ebb note · ebb ignore · ebb protect add|rm|ls <glob>
ebb daemon install|status|restart|uninstall
ebb approve <plan>               # review and approve a plan an agent made
ebb web allow|deny|confirm|ls    # which web pages may use this machine's ebb
```

Every command takes `--json`.

## AI agents

`ebb assistant install` gives AI tools the ebb skill and registers `ebb mcp`, through [lockstep](https://github.com/doguyilmaz/lockstep) when it is installed. Agents can scan, read findings, explain any path and plan cleanups. When the window or the service runs, agents share its scan and their plans appear there for approval.

## Hosted window

[ebb.apps.doguyilmaz.com](https://ebb.apps.doguyilmaz.com) is the same window served as a static page. It runs in your browser and talks to ebb on your own machine after you confirm a pairing code there. It works in Chrome, Edge and Firefox. Safari does not let a hosted page reach loopback. To host your own copy, see [docs/DEPLOY.md](docs/DEPLOY.md).

## Files

- `~/.local/state/ebb`: the last scan, history and the operation log.
- `~/.config/ebb/config.toml`: settings.
- `~/.config/ebb/rules/*.toml`: your own rules, the same format as [rules/](rules). ebb always asks before acting on them.

## Not done on purpose

- **Stripping app language files or thinning universal binaries.** It breaks code signatures for little space.
- **RAM "cleaning", maintenance scripts and privacy erasers.** They are not about disk space and rarely help.
- **Running as root or touching the system volume.**

## Build

```sh
make build    # bin/ebb, with the web UI (Go 1.26, Xcode command line tools, Bun)
make check    # vet, tests, gofmt, UI typecheck and tests
make e2e      # the window in Chrome against a sandboxed demo home
```

[API](docs/API.md) · [Performance](docs/PERF.md) · [Deploy](docs/DEPLOY.md) · MIT License
