# ebb local API

Base `http://127.0.0.1:<port>/api/v1` (default port 47340). JSON bodies, 1 MB limit. Every route except
`GET /health` and `POST /pair` needs `Authorization: Bearer <token>`; tokens are never accepted in a URL. The
event stream, which a browser opens without headers, takes `?ticket=` from `POST /events/ticket` (30 s, one use,
standing for the token that asked for it). Errors are
`Error {error, code?}` with a matching HTTP status. Types are defined in `internal/api/types.go` and generated into
`ui/src/api/types.gen.ts`; this table names them.

Requests are accepted only with a loopback `Host` header (`127.0.0.1`, `localhost`, `::1`), any port. Cross-origin
callers must be on the allowed origins list and paired (see Pairing).

| Method | Path | Request | Response | Errors |
|---|---|---|---|---|
| GET | `/health` | none | `Health` | none |
| GET | `/status` | none | `Status` | none |
| POST | `/scans` | `ScanRequest` | `ScanState` (202) | 409 `scan-running`, 409 `cleaning` |
| GET | `/scans/current` | none | `ScanState` | none |
| DELETE | `/scans/current` | none | `ScanState` (202; `scan.done` follows with `stopped: true`, the last result stays) | 409 `no-scan` |
| GET | `/volume` | none | `Breakdown` | 409 `never-scanned` |
| GET | `/volumes` | none | `Disk[]` (the startup disk first, scanned through its data volume, then every disk the Finder shows; scan one with `POST /scans {root: path}`) | none |
| GET | `/tree?path&depth&minShare&maxNodes` | none | `SliceResponse` | 400 `outside-root`, 409 `never-scanned` |
| GET | `/ages?path` | none | `AgesResponse` (bytes by last-modified bucket, relative to the scan time) | 400 `outside-root`, 409 `never-scanned` |
| GET | `/explain?path` | none | `Explain {item, findings, biggest}` | 400 `outside-root`, 404 |
| GET | `/files?path&minSize&olderThanDays&limit` | none | `FilesResponse` (stored files ≥ 1 MB, largest first, optionally untouched for N days) | 400 `outside-root` |
| GET | `/children?path&limit&small=1` | none | `ChildrenResponse` | 400 `outside-root` |
| GET | `/search?q&limit` | none | `SearchResponse` | none |
| GET | `/sources` | none | `Flows {nodes, links, total}` (place → owner → kind over the biggest folders of home and of the scanned disk) | 409 `never-scanned` |
| GET | `/packages` | none | `Packages {projects, nodeModules, packages, cache, cacheBytes}` (node_modules packages across projects with copies and versions; Bun's cache per package) | 409 `never-scanned` |
| GET | `/apps` | none | `AppsResponse {apps, note?}` (apps in /Applications and ~/Applications as the last scan saw them, biggest first, each with its Library data; uninstall with `POST /plans {app}`) | 409 `never-scanned` |
| POST | `/duplicates` | `DuplicatesRequest {path?, minSize?}` | `DuplicatesState` (202; events `duplicates.progress`, `duplicates.done`) | 400 `outside-root`, 409 `scan-running`/`cleaning`/`searching` |
| GET | `/duplicates` | none | `DuplicatesState {running, progress, result, error}` (the last result survives restarts) | none |
| DELETE | `/duplicates/current` | none | `DuplicatesState` (202) | 409 `no-search` |
| GET | `/findings?category&minBytes&ignored=1` | none | `FindingsResponse` | none |
| GET | `/report?format=json\|csv\|html\|text` | none | file | 400 `bad-format` |
| POST | `/reveal` | `PathRequest` | 204 | 400 `outside-root` |
| POST | `/ask` | `AskRequest {path \| findingId, question}` | `AskResponse {answer}` (Markdown from the person's own `claude -p`, one turn, no tools; only metadata is sent) | 409 `ask-unavailable`, 400 `outside-root` |
| POST | `/choose-folder` | `ChooseRequest {purpose: scan\|protect}` | `ChooseResponse {path}` (empty when cancelled; shows the macOS chooser on this Mac) | 400, 409 `sandboxed` |
| GET | `/history` | none | `HistoryResponse` | none |
| GET | `/history/diff?from&to` | none | `DiffResponse` (`from` is null while there is no earlier scan of the same folder; empty IDs mean the newest and the one before) | 404 |
| GET/POST | `/settings` | `Settings` (fields left out keep their value) | `Settings` | 400 |
| GET | `/daemon` | none | `Daemon` (installed, running or waiting for another ebb to close, what it did last, log path) | none |
| POST | `/daemon/install\|restart\|uninstall` | none | `Daemon` (202 when the server answering is the service itself; it acts right after) | 400, 409 `unsupported` |
| GET | `/presets` | none | `PresetView[]` | none |
| POST | `/plans` | `PlanRequest {findingIds?, paths?, preset?, app?, source?}` (`app`: name or bundle ID to uninstall; Apple's apps are refused) | `Plan` | 400, 409 `never-scanned` |
| GET | `/plans` | none | `PendingPlans {plans}` (made by an agent or the CLI, waiting for the person's approval, oldest first) | none |
| GET | `/plans/{id}` | none | `Plan` | 404 `no-plan` |
| POST | `/plans/{id}/approve` | none | `Plan` (approval only; whoever made the plan runs it) | 404 `no-plan` |
| POST | `/plans/{id}/execute` | none | `PlanResult` (events `plan.progress` per item, `plan.done`) | 404, 409 `needs-approval`, 409 `scan-running`, 409 `cleaning`, 410 `plan-expired` |
| GET | `/ops?limit` | none | `OpsResponse` | none |
| POST | `/ops/{id}/undo` | none | 204 | 400 |
| GET/POST | `/notes` | `NoteRequest {key, text}` (empty text removes) | `{key: text}` | 400 |
| GET/POST | `/ignores` | `IgnoreRequest {findingId}` | `{findingId: time}` | none |
| DELETE | `/ignores/{id}` | none | `{findingId: time}` | none |
| POST | `/events/ticket` | none | `{ticket}` | 401 |
| GET | `/events?ticket` | none | `text/event-stream` | 401 |

Plans expire after 10 minutes and run once. A plan needs approval unless every runnable item is `regenerable` and
`[mcp] auto_execute = "regenerable"` (the default). Execute re-checks every item: it must still be the same file
system object (device and inode) the plan saw, pass the guard (protected locations, scanned root, your protect
list, case and symlink variants included) and its owner app must not be running. `GET /findings?ignored=1` also
returns ignored findings, flagged.

Routes that manage web access (`GET /web`, `/pair/*`, `/web/allow`, `/web/deny`) and the background service
(`/daemon/*` POSTs) accept only this machine's own token, never a token issued to a paired page.

A plan item is also blocked when the last scan does not vouch for it: a path it never saw, a mounted disk or a
folder holding one, anything inside a cloud-synced folder or ebb's own settings and history. A cleanup and a scan
never run at the same time. Only one cleanup runs at a time.

Paths are accepted in display form (`/Users/me/...`) or real form (`/System/Volumes/Data/Users/me/...`) and must lie
inside the scanned root.

## Events

`event: <name>` with JSON `data`: `hello`, `scan.progress` (`Progress`), `scan.done` (`Meta`), `findings.changed`,
`plan.progress`, `plan.done`, `history.changed`, `plans.changed` (a plan from an agent or the CLI waits for approval,
or no longer does). A `: ping` comment is sent every 25 s.

## Pairing (hosted web app)

1. `GET /health` from the page tells "no server" apart from "server, origin not allowed" (`allowed`).
2. The page makes a random nonce (≥ 24 characters), keeps it and sends `POST /pair` `PairRequest {nonce}` with
   its `Origin` header and no token. Unknown origin → 403; another page already waiting → 409 `pair-busy`.
   Otherwise → `PairResponse {pending, code, expires, confirm}`; repeating the same nonce returns the same code.
3. The person confirms on the machine: `ebb web confirm <code>` or the local UI
   (`GET /pair/pending`, `POST /pair/confirm {code}`, `POST /pair/reject`). Confirmation from a non-loopback origin
   is refused, so a page can never approve itself. Codes are 6 digits, valid 5 minutes, 5 attempts.
4. The page repeats `POST /pair` with the same nonce and receives `{token, device, version}`: a token issued to
   that origin (only its hash is stored), once. An `Origin` header alone never releases a token: any local process
   can send one.
5. `GET /web`, `POST /web/allow {origin}`, `POST /web/deny {origin}` manage the list (`WebAccess`); deny also
   revokes every token issued to that origin.
