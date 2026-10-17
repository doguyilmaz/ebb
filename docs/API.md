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
| GET | `/volume` | — | `Breakdown` | 409 `never-scanned` |
| GET | `/tree?path&depth&minShare&maxNodes` | — | `SliceResponse` | 400 `outside-root`, 409 `never-scanned` |
| GET | `/children?path&limit&small=1` | — | `ChildrenResponse` | 400 `outside-root` |
| GET | `/search?q&limit` | — | `SearchResponse` | — |
| GET | `/findings?category&minBytes` | — | `FindingsResponse` | — |
| GET | `/report?format=json\|csv\|html\|text` | — | file | 400 `bad-format` |
| POST | `/reveal` | `PathRequest` | 204 | 400 `outside-root` |
| GET | `/history` | — | `HistoryResponse` | — |
| GET | `/history/diff?from&to` | — | `DiffResponse` | 404 |
| GET/POST | `/settings` | `Settings` | `Settings` | 400 |
| GET | `/presets` | — | `PresetView[]` | — |
| GET | `/events` | — | `text/event-stream` | 401 |

Paths are accepted in display form (`/Users/me/...`) or real form (`/System/Volumes/Data/Users/me/...`) and must lie
inside the scanned root.

## Events

`event: <name>` with JSON `data`: `hello`, `scan.progress` (`Progress`), `scan.done` (`Meta`), `findings.changed`,
`plan.progress`, `plan.done`, `history.changed`. A `: ping` comment is sent every 25 s.

## Pairing (hosted web app)

1. `GET /health` from the page tells "no server" apart from "server, origin not allowed" (`allowed`).
2. `POST /pair` with an `Origin` header and no token. Unknown origin → 403. Allowed origin not yet confirmed →
   `PairResponse {pending, code, expires, confirm}`; confirmed → `{token, device, version}`.
3. The person confirms on the machine: `ebb web confirm <code>` or the local UI
   (`GET /pair/pending`, `POST /pair/confirm {code}`, `POST /pair/reject`). Confirmation from a non-loopback origin
   is refused, so a page can never approve itself. Codes are 6 digits, valid 5 minutes, 5 attempts.
4. `GET /web`, `POST /web/allow {origin}`, `POST /web/deny {origin}` manage the list (`WebAccess`).
