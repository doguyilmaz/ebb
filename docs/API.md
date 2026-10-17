# ebb local API

Base `http://127.0.0.1:<port>/api/v1` (default port 47340). JSON bodies, 1 MB limit. Every route except
`GET /health` and `POST /pair` needs `Authorization: Bearer <token>` (or `?token=` on `/events`). Errors are
`Error {error, code?}` with a matching HTTP status. Types are defined in `internal/api/types.go` and generated into
`ui/src/api/types.gen.ts`; this table names them.

Requests are accepted only with a loopback `Host` header (`127.0.0.1`, `localhost`, `::1`), any port. Cross-origin
callers must be on the allowed origins list and paired (see Pairing).

| Method | Path | Request | Response | Errors |
|---|---|---|---|---|
| GET | `/health` | — | `Health` | — |
| GET | `/status` | — | `Status` | — |
| POST | `/scans` | `ScanRequest` | `ScanState` (202) | 409 `scan-running` |
| GET | `/scans/current` | — | `ScanState` | — |
| DELETE | `/scans/current` | — | `ScanState` (202; `scan.done` follows with `stopped: true`, the last result stays) | 409 `no-scan` |
| GET | `/volume` | — | `Breakdown` | 409 `never-scanned` |
| GET | `/tree?path&depth&minShare&maxNodes` | — | `SliceResponse` | 400 `outside-root`, 409 `never-scanned` |
| GET | `/ages?path` | — | `AgesResponse` (bytes by last-modified bucket, relative to the scan time) | 400 `outside-root`, 409 `never-scanned` |
| GET | `/explain?path` | — | `Explain {item, findings, biggest}` | 400 `outside-root`, 404 |
| GET | `/files?path&minSize&olderThanDays&limit` | — | `FilesResponse` (stored files ≥ 1 MB, largest first, optionally untouched for N days) | 400 `outside-root` |
| GET | `/children?path&limit&small=1` | — | `ChildrenResponse` | 400 `outside-root` |
| GET | `/search?q&limit` | — | `SearchResponse` | — |
| GET | `/findings?category&minBytes&ignored=1` | — | `FindingsResponse` | — |
| GET | `/report?format=json\|csv\|html\|text` | — | file | 400 `bad-format` |
| POST | `/reveal` | `PathRequest` | 204 | 400 `outside-root` |
| POST | `/choose-folder` | `ChooseRequest {purpose: scan\|protect}` | `ChooseResponse {path}` (empty when cancelled; shows the macOS chooser on this Mac) | 400, 409 `sandboxed` |
| GET | `/history` | — | `HistoryResponse` | — |
| GET | `/history/diff?from&to` | — | `DiffResponse` (`from` is null while there is no earlier scan of the same folder; empty IDs mean the newest and the one before) | 404 |
| GET/POST | `/settings` | `Settings` | `Settings` | 400 |
| GET | `/presets` | — | `PresetView[]` | — |
| POST | `/plans` | `PlanRequest {findingIds?, paths?, preset?, source?}` | `Plan` | 400, 409 `never-scanned` |
| GET | `/plans/{id}` | — | `Plan` | 404 `no-plan` |
| POST | `/plans/{id}/approve` | — | `Plan` | 404 `no-plan` |
| POST | `/plans/{id}/execute` | — | `PlanResult` (events `plan.progress` per item, `plan.done`) | 404, 409 `needs-approval`, 410 `plan-expired` |
| GET | `/ops?limit` | — | `OpsResponse` | — |
| POST | `/ops/{id}/undo` | — | 204 | 400 |
| GET/POST | `/notes` | `NoteRequest {key, text}` (empty text removes) | `{key: text}` | 400 |
| GET/POST | `/ignores` | `IgnoreRequest {findingId}` | `{findingId: time}` | — |
| DELETE | `/ignores/{id}` | — | `{findingId: time}` | — |
| GET | `/events` | — | `text/event-stream` | 401 |

Plans expire after 10 minutes and run once. A plan needs approval unless every runnable item is `regenerable` and
`[mcp] auto_execute = "regenerable"` (the default). Execute re-checks every item: it must still be the same file
system object (device and inode) the plan saw, pass the guard (protected locations, scanned root, your protect
list, case and symlink variants included) and its owner app must not be running. `GET /findings?ignored=1` also
returns ignored findings, flagged.

Routes that manage web access (`/pair/*`, `/web/allow`, `/web/deny`) accept only this machine's own token, never a
token issued to a paired page.

Paths are accepted in display form (`/Users/me/...`) or real form (`/System/Volumes/Data/Users/me/...`) and must lie
inside the scanned root.

## Events

`event: <name>` with JSON `data`: `hello`, `scan.progress` (`Progress`), `scan.done` (`Meta`), `findings.changed`,
`plan.progress`, `plan.done`, `history.changed`. A `: ping` comment is sent every 25 s.

## Pairing (hosted web app)

1. `GET /health` from the page tells "no server" apart from "server, origin not allowed" (`allowed`).
2. The page makes a random nonce (≥ 24 characters), keeps it, and sends `POST /pair` `PairRequest {nonce}` with
   its `Origin` header and no token. Unknown origin → 403; another page already waiting → 409 `pair-busy`.
   Otherwise → `PairResponse {pending, code, expires, confirm}`; repeating the same nonce returns the same code.
3. The person confirms on the machine: `ebb web confirm <code>` or the local UI
   (`GET /pair/pending`, `POST /pair/confirm {code}`, `POST /pair/reject`). Confirmation from a non-loopback origin
   is refused, so a page can never approve itself. Codes are 6 digits, valid 5 minutes, 5 attempts.
4. The page repeats `POST /pair` with the same nonce and receives `{token, device, version}` — a token issued to
   that origin (only its hash is stored), once. An `Origin` header alone never releases a token: any local process
   can send one.
5. `GET /web`, `POST /web/allow {origin}`, `POST /web/deny {origin}` manage the list (`WebAccess`); deny also
   revokes every token issued to that origin.
