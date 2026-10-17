# ebb — design spec

Date: 2026-10-17 · Status: draft for review · Module: `github.com/doguyilmaz/ebb` · CLI: `ebb`

## 1. Goal

Answer "where did my disk go, and why" on macOS in seconds, and reclaim space safely — from the CLI, a web UI
(embedded or hosted, paired to the local port) or an AI agent over MCP.

Success criteria

- Full scan of the data volume (5.95M files, 410 GB on the reference Mac) finishes in ≈ 45–52 s, shows first results
  in ≤ 1 s, peaks ≈ 400 MB RSS, and leaves the machine responsive (throttled I/O, bounded workers). Measured in
  phase 1 (docs/PERF.md): the walk is kernel-bound; repeat scans are incremental via FSEvents.
- Every byte of `statfs` "used" is attributed: scanned, sealed system volume, other APFS volumes (VM, Preboot…),
  local snapshots, purgeable, or an explicit "not visible" bucket with the reason (permissions / Full Disk Access).
- Each finding says what it is, why it is large, what deleting it costs, and how safe it is.
- Nothing is deleted without a reviewed plan; every action is logged; trash actions are undoable.

Non-goals (this spec): Windows/Linux support beyond a generic walker used in tests and non-darwin builds; a native
GUI app; a TUI; running as root; mole's non-disk features (system "optimize", live CPU/network monitor, Touch ID
sudo).

Feature checklist (every item is specified below and scheduled in §15)

- Fast, non-blocking parallel scan with live progress (§4) · full byte attribution incl. hidden/system space (§4.4)
- Findings report: what/why/impact/safety/evidence + generated notes + user notes (§5, §6)
- Views: sunburst, treemap, icicle, list, sources Sankey, project ↔ package graph, history timeline (§10)
- Select-and-clean from UI/CLI/MCP with plan → approve → execute, trash/undo, oplog (§7)
- One-command presets for routine cleanups ("dev caches", "simulators", "all regenerable") (§6.3)
- App uninstall with leftovers; leftovers of already-removed apps (§6.2)
- Protect list (never touch) and ignore list (hide a finding) (§6.3)
- History, growth diff, exports JSON/CSV/HTML (§8, §10)
- Local API + embedded UI + hosted web app paired to the local port (§9, §10)
- MCP server + skill for AI agents, Ask-AI in the UI (§11)
- Daemon: scheduled gentle scans, FSEvents incremental rescans, low-space alert (§8)
- Single binary, Homebrew distribution (§15)

## 2. What the reference Mac told us

Measured 2026-10-17 (read-only `du`): CoreSimulator ≈85 GB (49 GB runtimes in `/Library`, 35 GB devices in
`~/Library`), bun cache 35 GB, Android SDK + AVD 32 GB, DerivedData 18 GB, gradle 16 GB, npm 16 GB. Nothing was
abnormal — dev tools accumulate and never evict. The rule set in §6 is shaped by this.

Accuracy trap found while designing: bun installs `node_modules` with `clonefile` on macOS and pnpm uses hardlinks,
so `du`-style sums double-count. ebb reports **allocated** and **unique** bytes separately (§4.3) and computes
reclaimable bytes from unique bytes only.

## 3. Architecture

One Go binary, same shape as lockstep (CLI + daemon + local web UI + MCP), with a shared `internal/app` facade that
the CLI, HTTP server and MCP server all call. Commands parse flags, call the facade and print; they hold no logic.

```
cmd/ebb/main.go                 os.Exit(cli.Main(os.Args[1:], os.Stdout, os.Stderr))
internal/
  walk/        parallel walker; walk_darwin.go (cgo getattrlistbulk), walk_generic.go (ReadDir+Lstat, !darwin + tests)
  tree/        pointer-free arena, aggregation, queries (children, top-n, path lookup, search, LOD slice)
  volume/      statfs, APFS container + volumes (diskutil -plist), local snapshots (tmutil), purgeable, firmlinks
  rules/       embedded TOML rule specs + user rules; generic path/pattern matcher
  detect/      detectors (one file each) producing findings; command runner is injected (sys.Env-style)
  finding/     Finding model, safety levels, evidence
  clean/       plan → approve → execute; trash/delete/command/manual; protected paths; oplog; undo
  history/     snapshots (gob+gzip), index, diff
  report/      JSON, CSV, self-contained HTML, terminal tables
  app/         facade: scan sessions, current tree, findings cache, plans, events
  server/      HTTP API on 127.0.0.1, bearer auth, pairing, SSE, embedded UI assets
  mcpserver/   hand-rolled MCP JSON-RPC over stdio (lockstep pattern)
  daemon/ svc/ launchd agent, schedules, FSEvents incremental rescans, low-space alert
  sys/ state/ config/   env abstraction + fakes, state files (WriteAtomic, 0600), config.toml
  assistant/   embedded SKILL.md + `ebb assistant install`
  api/         JSON contract types + tsgen (emits ui/src/api/types.gen.ts)
rules/         *.toml embedded via go:embed
ui/            React 19 + Vite 8 + TanStack Router/Query (lockstep stack), plain CSS tokens
```

Reuse from lockstep (copied and adapted; lockstep itself is untouched): server bootstrap + JSON/error helpers,
bearer auth (constant-time, `?token=` for EventSource), CORS + Private Network Access handling, pairing flow
(6-digit code, 5-min window, confirm only from loopback, `web_origins`; ebb additionally binds each confirmation to a page nonce and issues per-origin tokens), SSE hub, `go:embed dist`
asset handler with SPA fallback, MCP server shape (`protocolVersion "2025-06-18"`, `obj/str/boolp` schema helpers,
text-JSON results, `isError`), launchd service install/bootout, `sys.Env`/`sys.Fake`, `state.WriteAtomic`,
cobra `Main` + `--json` convention, testscript CLI tests, goreleaser + Homebrew tap. Extracting a shared module for
both tools is a separate follow-up once ebb stabilizes.

Differences from lockstep, on purpose

- **cgo on darwin.** getattrlistbulk, `NSFileManager trashItemAtURL`, FSEvents and volume capacity keys need system
  frameworks. `x/sys/unix` only carries the getattrlistbulk syscall number, no wrapper. Darwin release builds run on a
  macOS runner (clang `-arch arm64/x86_64`); other OS builds stay `CGO_ENABLED=0` with the generic walker.
- **Host header check** (accept only `127.0.0.1:<port>` / `localhost:<port>`) on top of lockstep's token + Origin
  rules, as defence against DNS rebinding for a tool that can delete files.
- **Generated TS types** from `internal/api` (`go generate`) instead of hand-written mirrors — one contract, no drift.

Paths (XDG, like lockstep): config `~/.config/ebb/config.toml`, user rules `~/.config/ebb/rules/*.toml`, state
`~/.local/state/ebb/` (`daemon.json`, `ui-token`, `oplog.jsonl`, `snapshots/`, `pending-pair.json`, `daemon.log`).
`EBB_HOME` re-roots everything; `EBB_SANDBOX=1` disables real commands, trash and OS service calls (tests, demos).
Default daemon port `47340`. Launchd label `dev.ebb.daemon`. Default hosted origin
`https://ebb.apps.doguyilmaz.com` (deployed the same way as lockstep's hosted UI).

## 4. Scanning engine

### 4.1 Walker

- Work queue of directories (mutex + cond, LIFO for locality), `N` workers, default `min(8, NumCPU)`, `--threads`.
- Darwin worker: `open(O_RDONLY|O_DIRECTORY)`, `getattrlistbulk` loop into a reused 256 KB per-worker buffer.
  Attributes: `CMN_NAME|OBJTYPE|FILEID|FLAGS|MODTIME|ERROR|RETURNED_ATTRS`, `FILE_LINKCOUNT|TOTALSIZE|ALLOCSIZE`,
  `DIR_MOUNTSTATUS`, extended `CMNEXT_CLONEID|EXT_FLAGS|CLONE_REFCNT` (`FSOPT_ATTR_CMN_EXTENDED|FSOPT_PACK_INVAL_ATTRS`). Buffer parsing
  lives in C next to the SDK headers; Go receives a packed array of fixed-size records + a name blob per directory.
- Never follows symlinks, never crosses mount points (`DIR_MNTSTATUS_MNTPOINT`), skips dataless (iCloud-evicted)
  bytes, records per-directory errors (EACCES, EPERM) with the byte-less path into an "inaccessible" list.
- Workers never touch the tree: they send one batch per directory over a channel to a single builder goroutine.
- I/O policy: interactive scans use `setiopolicy_np(IOPOL_TYPE_DISK, IOPOL_SCOPE_PROCESS, IOPOL_UTILITY)`, daemon and
  `--gentle` use `IOPOL_THROTTLE`. Cancellation via `context`. Progress event every 100 ms (dirs, files, bytes,
  errors, rate, sample path).
- Default root: `/System/Volumes/Data`. Paths are displayed through `/usr/share/firmlinks` (`/System/Volumes/Data/Users/x`
  → `/Users/x`). `ebb scan <path>` scans a subtree.

### 4.2 Tree (arena)

Struct-of-arrays, no pointers, so the GC never scans it: `parent, firstChild, nextSibling, nameOff []uint32`,
`nameLen []uint16`, `alloc, unique, logical []uint64`, `files []uint32`, `maxMtime []int64`, `flags []uint8`, plus a
`[]byte` name arena.

- Files under 1 MiB are not stored as nodes; their bytes and count fold into the parent's `small` aggregate. All
  directories and files ≥ 1 MiB are nodes. Listing a directory's small files is done live on demand (one ReadDir).
- Sizes propagate up the ancestor chain as batches arrive, so partial totals are always valid; readers take an
  `RWMutex` read lock, the builder a write lock per batch. Children are sorted by size when the scan finishes.
- Queries: children(path, limit), top-n files/dirs under path, find(glob|substring), path → node, LOD slice
  (`depth`, `minShare`, `maxNodes`) that merges tiny siblings into one "other" node — the browser never receives the
  whole tree.

### 4.3 Hardlinks and clones

- `nlink > 1`: a side table `inode → {nlink, alloc, nodes seen}` counts bytes once (first location) and marks the
  others shared. A target frees a hardlinked file only if all its links are inside the target.
- APFS clones (`EF_MAY_SHARE_BLOCKS`): the bulk pass also reads `ATTR_CMNEXT_CLONEID` and `ATTR_CMNEXT_CLONE_REFCNT`.
  Pure clones share a clone ID, so a side table `cloneID → {refs, alloc, nodes seen}` counts their unique bytes once,
  exactly like hardlinks; allocated bytes stay per copy (what `du` shows). A modified clone gets its own clone ID and
  counts fully as unique — a known overestimate the report notes. UI shows allocated vs unique; reclaimable = unique
  minus shared data that a copy outside the target keeps alive. (A `PRIVATESIZE` pass was measured and rejected:
  2.7× slower per entry, kernel-serialized as a post-pass — see `docs/PERF.md`.)

### 4.4 Volume accounting

`volume.Breakdown`: container capacity/free (statfs + `diskutil apfs list -plist`), sealed system volume, VM
(swap), Preboot/Recovery/Update, local snapshots (`tmutil listlocalsnapshots /`, count + dates), purgeable
(`NSURLVolumeAvailableCapacityForImportantUsageKey − NSURLVolumeAvailableCapacityKey`), scanned bytes, and
`not visible = used − attributed` with Full Disk Access status (probe a TCC-protected path) and the fix.

## 5. Findings

```go
type Finding struct {
    ID, RuleID, Title, Category, Owner string // Owner: "Xcode", "Bun", "Android Studio", …
    Safety   Safety   // regenerable | caution | user-data | system
    Bytes    uint64   // reclaimable (unique) bytes
    Targets  []Target // path, dev, ino, bytes
    Why      string   // why it is this large
    Impact   string   // what deleting costs
    Evidence []string // facts: "last used 214 days ago", "project folder no longer exists"
    Actions  []Action // default first
    Notes    string   // free-text note shown in the report
}
```

Report = volume breakdown + findings by bytes + "notes" section (not-visible bytes explained, accuracy caveats, what
was skipped and why) + top directories and files.

User notes: any finding or path can carry a user note (`~/.local/state/ebb/notes.json`, keyed by finding rule+path or
by path). Notes show in the UI side panel, the Findings view and every export, and survive rescans.

## 6. Rules and detectors

### 6.1 Rules and detectors

Data rules (TOML, embedded + user dir) cover fixed locations and name patterns:

```toml
[[rule]]
id = "bun.cache"
title = "Bun install cache"
owner = "Bun"
category = "package-cache"
paths = ["~/.bun/install/cache"]
safety = "regenerable"
why = "Bun keeps every version of every package it has installed; nothing is evicted automatically."
impact = "The next install re-downloads what it needs."
min_size = "500MB"
action = { kind = "command", argv = ["bun", "pm", "cache", "rm"], requires = "bun", fallback = "delete" }
```

Pattern rules use `match = { name = "node_modules", sibling = "package.json" }` style conditions, evaluated in one
pass over directory nodes. Categories: `package-cache, build-artifact, simulator, sdk, app-cache, logs, installer,
large-file, duplicate, stale-project, leftover, trash, snapshot, user-data`.

Detectors (Go, one file each, external commands through the injected runner) — v1 set:

| Detector | Source of truth | Default action |
|---|---|---|
| Simulators: devices unused N days, unavailable, old runtimes | `xcrun simctl list -j`, device dir mtimes | `simctl delete <udid>` / `simctl runtime delete <id>` |
| DerivedData per project, orphaned when workspace path is gone | `info.plist` WorkspacePath | delete |
| Xcode archives, device support, multiple Xcode.app versions | paths + Info.plist | trash |
| Android AVDs unused N days, old system images | `~/.android/avd/*.ini`, sdk dirs | delete |
| Package caches: bun, npm, pnpm, yarn, gradle, go, cargo, pip, CocoaPods, Homebrew | rules + `brew cleanup -n` | tool command, fallback delete |
| Stale projects: `node_modules`, `target`, `build`, `.gradle`, `Pods`, `.next`, `.expo` where the project is untouched N days | max mtime of project minus artifact dirs | delete |
| Package breakdown: package@version × copies × bytes across node_modules and bun cache | dir names + package.json | — (evidence for "why") |
| App leftovers: Containers / Application Support / Caches / Preferences for bundle IDs no longer installed | installed bundle IDs from /Applications Info.plist | trash |
| Docker: VM disk (allocated vs logical) | Docker.raw, `docker system df` | `docker system prune` (manual) |
| Large files ≥ 1 GiB not covered by another finding; installers (.dmg/.pkg/.xip) | tree | trash |
| Logs and crash reports | rules | delete |
| Trash size | `~/.Trash` | empty |
| Local snapshots | volume | manual `tmutil deletelocalsnapshots` |
| Duplicates (on demand): size → head/tail 64 KB → full SHA-256, files ≥ 1 MiB | tree + reads | trash (keep one) |
| Growth since previous snapshot | history | — |

"Unused N days" defaults to 30, configurable. Read-only user data (Photos, Messages, Mail, iOS backups, iCloud Drive)
is reported as `user-data` with guidance only — never as one-click deletion.

### 6.2 Apps and uninstall

The `apps` detector lists installed apps (`/Applications`, `~/Applications`, Setapp folder) with their footprint:
bundle size + leftovers matched by bundle ID and app name in `~/Library/{Containers, Group Containers, Application
Support, Caches, Preferences, Saved Application State, Logs, HTTPStorages, WebKit, LaunchAgents}`. Uninstall is a
plan: quit check (refuse while running), app bundle + leftovers → trash, LaunchAgents unloaded with
`launchctl bootout` first. System apps (`/System/Applications`, Apple-signed in `/Applications` such as Safari) are
never offered. App Store apps owned by root go through `manual` with the Finder hint. Leftovers of apps that are no
longer installed come from the same matcher (category `leftover`).

### 6.3 Presets, protect list, ignore list

Presets are named selections in the rule files, run with one command (`ebb clean --preset dev-caches`) or one click:

```toml
[[preset]]
id = "dev-caches"
title = "Developer caches"
categories = ["package-cache", "build-artifact"]
max_safety = "regenerable"
```

Built-in presets: `dev-caches`, `simulators` (unused devices + unavailable + old runtimes), `stale-projects`,
`logs`, `installers`, `all-regenerable`. Every preset still produces a plan; `--yes` skips only the CLI prompt for
regenerable items.

`[clean] protect = [globs]` in config.toml: never offered, never deleted (adds to the built-in protected list).
`ignore` (per finding rule+path, `~/.local/state/ebb/ignores.json`) hides a finding until its size grows by 50 %;
manageable in Settings.

## 7. Cleanup

Flow: select (findings or any tree nodes) → **plan** (server resolves each target to an action, bytes, safety,
warnings, blocking conditions such as "Xcode is running"; plan id expires in 10 min) → **approve** → **execute**
(streamed progress) → result with reclaimed bytes and undo handles. The tree is updated in place, no full rescan.

Action kinds: `command` (argv, no shell, timeout, sanitized env), `delete` (permanent), `trash`
(`NSFileManager trashItemAtURL`, resulting URL recorded), `empty-trash`, `manual` (root-owned or risky: the plan
shows the exact command to copy). Defaults by safety: regenerable → command/delete; caution → trash; user-data →
trash with per-item confirmation; system → manual only. The UI states that trash frees space only after emptying.

Guards, enforced at execute time, not just at plan time

- Re-`lstat` every target; refuse if dev/ino differ from the plan (TOCTOU).
- Protected list (`/System`, `/bin`, `/sbin`, `/usr` except `/usr/local`, `/Library/Apple`, `~/Library/Keychains`,
  `~` itself, every top-level home folder, iCloud sync roots via `EF_IS_SYNC_ROOT`, any mount point) is refused.
- Targets must be inside a scanned root; no symlink is followed; ebb refuses to run as root unless `--allow-root`.
- Every action appends to `oplog.jsonl` (`op_id, ts, plan_id, path, action, bytes, result, trash_url`). `ebb undo
  <op>` moves trashed items back. `ebb ops` / Activity view list history.

AI approvals: MCP can create plans freely. `ebb_cleanup_execute` runs without UI approval only when every item is
`regenerable` and `[mcp] auto_execute = "regenerable"` (default); anything else needs approval in the UI or
`ebb approve <plan>`.

## 8. History and growth

Every full scan saves a snapshot: directories ≥ 10 MB with path, alloc, unique, files, maxMtime (gob + gzip,
typically < 1 MB) plus volume breakdown. Index in `snapshots/index.json`; retention keeps the last 60 scans and one
per week beyond. `diff(a, b)` matches by path and returns growth/shrink lists; the UI draws a used-space timeline
and "what grew since <date>".

Daemon (`ebb daemon install|uninstall|status|run`): launchd agent running the server on the default port, a daily
gentle scan when on power and idle, FSEvents (cgo, CoreServices) to rescan only changed directories while the
server is up, and a notification (`osascript display notification`) when free space drops below 10 GB (configurable).
Known limitation: macOS TCC grants Full Disk Access per binary, so the daemon binary needs its own grant and
Homebrew upgrades may require re-granting; the UI detects and explains this.

## 9. HTTP API

`127.0.0.1:<port>`, `Authorization: Bearer <token>` (or `?token=` on `/events`), JSON, 1 MB body limit, errors
`{"error","code"?}`. Token-free: `GET /api/v1/health`, `POST /api/v1/pair`.

```
GET  /api/v1/status                         version, scan state, last scan, FDA status
POST /api/v1/scans {root?, threads?}        start (one active scan); GET /api/v1/scans/current; DELETE stops it
GET  /api/v1/events                         SSE: hello, scan.progress, scan.done,
                                            plan.progress, plan.done, findings.changed, history.changed
GET  /api/v1/volume
GET  /api/v1/tree?path&depth&minShare&maxNodes      LOD slice for charts
GET  /api/v1/ages?path                       bytes by last-modified age bucket
GET  /api/v1/children?path&limit&small=1
GET  /api/v1/search?q&limit
GET  /api/v1/findings?category&minBytes
GET  /api/v1/sources                        owner → category → location flows (Sankey)
GET  /api/v1/packages?limit                 package@version breakdown
POST /api/v1/duplicates {root, minSize}     on-demand job, results via events
GET  /api/v1/apps                           installed apps + leftovers footprint
GET  /api/v1/presets                        presets with current bytes
GET/PUT/DELETE /api/v1/notes                user notes ; GET/PUT/DELETE /api/v1/ignores
POST /api/v1/plans {targets|preset|app}     → plan; POST /plans/{id}/approve; POST /plans/{id}/execute
GET  /api/v1/ops ; POST /api/v1/ops/{id}/undo
GET  /api/v1/history ; GET /api/v1/history/diff?from&to
GET  /api/v1/report?format=json|csv|html
POST /api/v1/reveal {path}                  open -R in Finder
POST /api/v1/ask {path|findingId, question} headless `claude -p` (only when `claude` is on PATH)
GET/POST /api/v1/web, /web/allow, /web/deny, /pair/pending, /pair/confirm, /pair/reject   (lockstep pairing)
GET/POST /api/v1/settings ; /api/v1/daemon{,/install,/uninstall,/restart}
```

Every path parameter is cleaned, NUL-free, and must resolve inside a scanned root. Endpoints are documented in
`docs/API.md`; Go types in `internal/api` are the single contract and generate `ui/src/api/types.gen.ts`.

## 10. Web UI

Same app serves embedded (same origin, token via URL hash → sessionStorage) and hosted (pairs to the local port,
token in localStorage). English UI strings in v1.

Screens

1. **Overview** — stacked volume bar (system, apps, developer, user, snapshots, purgeable, not visible, free),
   reclaimable total, top findings, scan button with live progress, growth since last scan.
2. **Explore** — Sunburst (DaisyDisk-style) · Treemap (squarified, bundle-analyzer style) · Icicle · List
   (virtualized, sortable). Breadcrumb, click-to-zoom, hover details, color by category or file kind, allocated vs
   unique toggle. Side panel: size, files, last modified, owner/rule, safety, findings, actions (Reveal, Add to
   cleanup, Ask AI).
3. **Findings** — the report: grouped by category, each with why/impact/evidence/notes, checkboxes, selection tray
   ("N selected · X GB → Review plan").
4. **Sources** — Sankey owner → category → location; Packages tab: a bipartite graph projects ↔ package@version
   (edge = a copy in that project's node_modules, node size = bytes), plus the bun-cache version list — answers
   "which packages, in how many copies, eat node_modules and the cache". Apps tab: installed apps by total footprint
   with Uninstall.
5. **History** — used-space timeline, snapshot diff, growth list.
6. **Activity** — oplog with undo.
7. **Settings** — scan roots and exclusions, thresholds, rules, web access/pairing, daemon, MCP/skill install, FDA.

Cleanup sheet: plan review (actions, warnings, blockers) → confirm → progress → result. ⌘K palette for paths and
actions.

Guidance: a first-run Welcome (what ebb does, read-only scan, Full Disk Access explained), a live scan progress
screen, and a guided tour after the first scan (re-openable from the help button and ⌘K). Every concept has a short
explanation behind an (i) tooltip, all from one glossary. Charts animate with a keyed morph (zoom, metric switch,
refresh), show labels where they fit, a cursor tooltip and a ranked legend (also the table view); motion follows
`prefers-reduced-motion`. Overview adds cleanable space by category and an age breakdown.

Rendering: Canvas 2D for sunburst/treemap/icicle (SVG does not survive tens of thousands of shapes), layouts from
`d3-hierarchy`, Sankey from `d3-sankey`, timeline as hand-drawn SVG. Charts fetch LOD slices and refetch on zoom.
Hit-testing uses the computed layout rects. Styling: plain CSS with tokens (light/dark), like lockstep.

Exports: JSON (full report), CSV (findings + top dirs), HTML (self-contained file: inline data + inline treemap
script, no network).

## 11. MCP server and skill

`ebb mcp` (stdio). It talks to the running server when `daemon.json` points to a live one (shared scan state with
the UI); otherwise it runs the engine in-process. Tools (`ebb_*`, JSON text results):

`status, scan {root, wait}, overview, top {path, n, kind}, search {query}, findings {category, minBytes},
explain {path}, packages, apps, presets, history_diff {from, to}, cleanup_plan {targets | preset | app},
cleanup_execute {planId}, undo {opId}, note {target, text}, report {format}`.

`ebb assistant install` writes the embedded `SKILL.md` (how to investigate, how to plan cleanups safely, never
delete user-data without asking) into each AI tool and registers `ebb mcp` — lockstep's mechanism; lockstep then
syncs it across machines.

AI in the UI: "Ask" runs `claude -p` with tools disabled and sends only metadata (paths, sizes, rule facts, evidence),
never file contents; the answer is shown as markdown.

## 12. CLI

```
ebb                       overview (last snapshot if < 1 h old, else scans) + hint for `ebb serve --open`
ebb scan [path] [--threads N] [--gentle] [--json]
ebb top [path] [-n 20] [--files] ;  ebb find <pattern>
ebb findings [--category c] ;  ebb report [--format json|csv|html] [-o file]
ebb clean [finding-id|category…] [--preset id] [--dry-run] [--yes] ;  ebb approve <plan> ;  ebb ops ;  ebb undo <op>
ebb apps ;  ebb uninstall <app> [--dry-run] ;  ebb note <path|finding> <text> ;  ebb ignore <finding>
ebb protect add|rm|ls <glob>
ebb history ;  ebb diff [from] [to]
ebb serve [--port] [--open] ;  ebb web allow|deny|confirm ;  ebb daemon install|uninstall|status|run
ebb mcp ;  ebb assistant install ;  ebb version
```

Global flags `--json`, `--home`, `-v`. Notices go to stderr so `--json` stays clean.

## 13. Security

- Loopback bind only; bearer token (24 random bytes, 0600); Origin allowlist with pairing; Host header check; CORS
  never `*`; PNA header only when requested; no CSRF-able routes without the bearer header.
- Path validation on every path input; protected list and dev/ino re-check before any destructive action.
- Commands from rules run as argv without a shell; user rules can only add rules, never relax protections; the plan
  shows each command before it runs.
- No telemetry; nothing leaves the machine except the optional `claude -p` call, which sends metadata only.

## 14. Testing and verification

- Walker: fixtures in temp dirs with hardlinks, APFS clones (`cp -c`), symlink loops, permission-denied dirs, NFD
  names, long paths, sparse files; darwin walker results must equal the generic walker's; totals within 1 % of
  `du -skx` on sample dirs (after clone/hardlink adjustment).
- Benchmarks: synthetic 1M-entry tree; `ebb scan --profile` writes pprof; real-disk run recorded in `docs/PERF.md`
  (time, RSS via `/usr/bin/time -l`, CPU).
- Unit tests for tree queries/LOD, rules matcher, each detector (fake runner + fixture trees), clean (protected
  paths, TOCTOU refusal, trash vs delete, oplog, undo — in `EBB_SANDBOX`), history diff, API handlers (httptest),
  pairing (ported tests), MCP (JSON-RPC golden), tsgen.
- CLI end-to-end with testscript (`testdata/script/*.txtar`).
- UI: `bun test` for layout/LOD/format utils; Playwright audit against a sandboxed `ebb serve` like lockstep's e2e.
- `make check` = vet + test + gofmt + UI typecheck/tests; build = UI bundle → `go build` with embed.

## 15. Delivery phases

1. **Engine + report:** walker, tree, hardlink/clone accounting, volume breakdown, rules, detectors, findings,
   CLI (`scan/top/find/findings/report/history/diff`), snapshot on every full scan, JSON/CSV/HTML export, benchmarks.
2. **Server + UI + cleanup:** API, SSE, auth/pairing, embedded UI (Overview, Explore with all four views, Findings,
   Activity, Settings), plan/approve/execute, trash/delete/command, oplog, undo, presets, protect/ignore, notes.
3. **AI:** MCP server, skill + `assistant install`, Ask in UI.
4. **Over time:** History screen (timeline + diff), daemon (launchd, schedules, FSEvents, low-space alert), Sources
   Sankey, Packages graph, Apps + uninstall, duplicates, hosted deploy, goreleaser + Homebrew tap.

Each phase ends with the full test suite green and a real run on this Mac.

## 16. Handover

Any agent can resume from the repo alone:

- This spec is the source of truth for *what*; the implementation plan
  `docs/superpowers/plans/2026-10-17-ebb.md` is the source of truth for *how* and *how far*: tasks are checkboxes,
  ticked in the same change that completes them.
- Deviations from this spec are recorded in the plan's "Deviations" section with the reason, and the spec is
  updated to match — never left to drift.
- `make check` must pass before a task is ticked. A resuming agent runs it first, then continues at the first
  unticked task.
