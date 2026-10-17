---
name: ebb
description: Use when someone asks why their Mac's disk is full, what takes space, how to free space, or wants caches, build output, simulators, logs or old installers cleaned up, or mentions ebb. Investigates with ebb's MCP tools (or the ebb CLI) and plans cleanups the person reviews before anything is removed.
---

# ebb: where the disk space went, and getting it back safely

ebb scans the disk once (later scans only re-read folders that changed), explains every large item and cleans through
reviewed plans. Use its MCP tools (`ebb_*`); without MCP, the `ebb` CLI does the same.

## Investigate

1. `ebb_status`. No scan yet, or the last one is old: `ebb_scan` (it waits; later scans take seconds).
2. `ebb_overview`: how the disk splits (macOS, apps, what can be cleaned, everything else, not visible, free) and the
   largest findings.
3. Go deeper only where it matters:
   - `ebb_findings` lists what ebb found, each with **why** it is there, what removing it **costs**, and its **safety**:
     `regenerable` (a tool rebuilds it), `caution` (usually safe, look first), `user-data` (the person's own files),
     `system` (never removed).
   - `ebb_explain {path}` for any folder: sizes, the findings covering it, what is biggest inside.
   - `ebb_top {path, kind: "dirs" | "files", olderThanDays}` for the biggest things somewhere, `ebb_search` by name.
   - `ebb_history_diff` for what grew since the previous scan.
4. Answer in the person's terms: the few biggest causes, what each is, and what removing it would cost.

"Not visible" space is what the scan could not read (other users, apps' private data, local snapshots); Full Disk
Access for the app that runs ebb shows more of it. "Purgeable" space is freed by macOS itself.

## Clean safely

- **Always plan first**: `ebb_cleanup_plan` with `findingIds`, a `preset` (`ebb_presets`, e.g. developer caches) or
  `paths`. Show the plan: each item, its action (Trash, delete, the tool's own command, or manual), size and safety.
- **Then** `ebb_cleanup_execute {planId}`. Plans made only of rebuildable caches run directly. Anything else returns
  "needs approval": tell the person to approve it in ebb's window or with `ebb approve <id>`, then execute again.
  Never try to get around an approval.
- Never remove `user-data` or anything the person did not clearly choose. Prefer presets and findings over raw paths;
  raw paths always go to the Trash.
- Items that went to the Trash can be put back with `ebb_undo {opId}` (the op ids are in the execute result).
- Leave a note (`ebb_note`, `finding:<id>` or `path:<path>`) when the person decides to keep something, so it is not
  suggested again without context.

## CLI equivalents

| MCP | CLI |
|---|---|
| ebb_scan | `ebb scan [path]` (`--full` reads everything) |
| ebb_overview / ebb_findings | `ebb overview`, `ebb findings` |
| ebb_top | `ebb top [path]` |
| ebb_search | `ebb find <pattern>` |
| ebb_history_diff | `ebb diff` |
| ebb_cleanup_plan + execute | `ebb clean <finding-id> --dry-run`, then `ebb clean <finding-id>` |
| ebb_undo | `ebb undo [op-id]` |
| ebb_report | `ebb report --format text\|json\|csv\|html` |

`ebb serve --open` opens the window, where the person can see everything and approve plans.
