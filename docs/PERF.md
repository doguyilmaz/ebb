# Performance

Measured on the reference Mac (Apple Silicon, 10 cores, macOS 26). Update with every engine change.

## tree

| Benchmark | Result |
|---|---|
| Build 1.1M nodes (100k dirs × 10 files) + Finish | 82 ms/op, 528 MB/op transient (slice growth) |
| Slice (depth 4, minShare 0.005, 4000 nodes) on that tree | 9 µs/op |

## walk (`~/Library`, 869k files, warm cache, 8 workers)

| Reader | Time |
|---|---|
| getattrlistbulk + clone IDs (default) | 7.0 s |
| ReadDir + Lstat (generic) | 11.3 s |
| getattrlistbulk + `ATTR_CMNEXT_PRIVATESIZE` per entry | 18.8 s (rejected) |
| getattrlistbulk + private-size post-pass on 121k clones | +8.1 s, does not scale with threads (rejected) |

Clone IDs (`ATTR_CMNEXT_CLONEID` + `CLONE_REFCNT`) cost ≈ 19 % over the plain bulk pass and let pure clones be
deduplicated like hardlinks. CPU profile: ~90 % of time is kernel work: directory `open` by full path (~45 %) and
`getattrlistbulk` (~30 %); Go-side work is negligible. Next lever: open children with `openat` relative to the
parent descriptor (Task 13).

### openat and searchfs (2026-10-17)

- Opening subfolders with `openat` relative to the parent's descriptor (kept open while its children are queued,
  budget half the process fd limit, path fallback beyond it): 8.6 → 8.0 s on `~/Library` (880k files, warm, back to
  back). Profile afterwards: `openat` ~49 %, `getattrlistbulk` ~41 %, Go ~2 %. The cost is the kernel's per-vnode
  work, not path lookup, so no language or FFI change helps.
- `searchfs(2)` catalog scan, as a read-only probe: works without root and returns sizes but runs on one thread:
  117 s for the whole data volume (7.06M entries) versus ~45 s for the parallel walk. Not worth pursuing.
- Worker count is not settled: runs at 8/10/14 workers swung between 14 and 19 s with the cache state; measure
  interleaved, several rounds, before changing the default. The real lever for repeat scans is FSEvents (Task 30).

### incremental rescans (FSEvents, 2026-10-18)

Same Mac, data volume, ebb serving (watcher live), a few minutes of normal use between scans:

| | Total | Folders listed again |
|---|---|---|
| full scan (baseline that stores the event ID) | 52 s | all 1.05M |
| "scan again", first version | 10.7 s | 416 (8.2 s: whole-tree re-sort + quadratic clearing of big folders) |
| "scan again", targeted sort + linear clearing | **2.6 s** | 208 in 1.2 s; the rest is detectors, saving the tree and a snapshot |

FSEvents replay after ebb was off: ~3.6 h of changes = 30k folder events in 11 s, then the update as above.

### dropped FSEvents (2026-11-11)

The live stream drops events (`UserDropped`, flags 0x3, path = the stream root) when a burst of changes outruns it:
16 in about an hour of builds and e2e runs on this Mac, 11 for 150k folders created in 29 s. The history macOS keeps
on disk still holds every change (replayed afterwards: all 150k folders, no drop flags). Until now one drop marked the
change set lost, so every "scan again" and every daemon check read everything (28-85 s instead of ~1.5 s) until the
next full scan. `KernelDropped`, `IDsWrapped` and `RootChanged` were never seen and still mean a full scan.

Now the watcher restarts the stream from the last event it received before the drop and the history fills the gap. The
set is not handed out while that replay runs. Two things measured on the way:

- A replay started while the burst is still going comes back with drops of its own after a few thousand events (the
  last second or two of history is not on disk yet). So the restart waits until drops have stopped for 5 s (30 s at
  most), and a dropped replay is tried again from where it got to. At most 8 recoveries in 10 minutes; past that, or
  when macOS reports one of the unrecoverable drops, the set is lost as before and the daemon log says why.
- Replays after the burst were clean every time (131k-264k events in 1.3-2.9 s, no drop flags).

Sandbox, 44k folders created and rewritten by 4 writers over 15-32 s (about 220k-440k directory events), watcher
following the real data volume from a fresh event ID:

| | 15-18 s burst | 32 s burst |
|---|---|---|
| before | `lost: true` 7-10 s into the burst | `lost: true` |
| after | `lost: false`, ready within 15 s after the burst (3 recoveries in the logged run); the set holds all folders | `lost: false`, ready within 15 s after the burst |

A lost set costs a full scan (28-85 s); an intact one a ~1.5 s update.

## full scan (`/System/Volumes/Data`: 5.95M files, 1.04M folders, 410.6 GB, no Full Disk Access)

| Build | Walk | Total (incl. detectors, saving) | Max RSS |
|---|---|---|---|
| first version | 48.6 s | 85.2 s | 491 MB |
| O(1) `Reclaimable` (shared-group LCA cache) + 256 MiB soft memory limit | 42.8 s | 44.6 s | 395 MB |
| same, `--threads 16` | 43.7 s | 45.1 s | 406 MB |
| after detector fixes (run-to-run variance) | 50.4 s | 52.3 s | 411 MB |

The walk is kernel-bound: ~165 s of system CPU across 8 workers. 16 workers add kernel CPU without finishing
sooner, so 8 stays the default. Detectors and persistence take ~2 s. The tree holds 1.07M nodes (~100 MB) plus 225k
APFS clone groups. `Reclaimable` used to scan every shared group per call (36 s spent in the stale-project
detector); it now reads a per-node cache computed once per write in O(groups × depth): 1.4 s → 0.9 ms per 1,000 calls.
| `GOGC=50` instead of the fixed memory limit (scales with disk size) | 41.6 s | 43.4 s | 345 MB |

## history diff (real snapshots, 2026-11-11)

`Explaining` compared every pair of changes and built a string for each comparison. It now sorts the changes by
path once: everything below a folder is one run of that order, found with two binary searches, and a max tree
gives the largest change of the run in each direction. Same result, byte-identical API responses.

| Comparison | Changes in / kept | `Explaining` before | after | `GET /api/v1/history/diff` before | after |
|---|---|---|---|---|---|
| a week (the Overview default) | 9,147 / 4,993 | 2.1 s | 7 ms | 1.87–1.98 s | 31–37 ms |
| oldest snapshot (3.5 weeks) | 15,054 / 8,369 | 6.2 s | 11 ms | 5.23–5.37 s | 40–48 ms |

`BenchmarkExplaining` (10k synthetic changes, deep paths, mixed signs): 7–10 ms/op, 2.8 MB/op, 10k allocs/op; the
pairwise version took 3.7 s/op, 9.6 GB/op, 69M allocs/op on the same data. The Overview waits for the week
comparison before it shows anything, so the window went from ~2 s of empty background to the shell and page at
170–290 ms (headless Chrome, warm server, dark scheme).

### loading the saved scan (18.5 MB `last.tree`)

Every caller of `Last` that found nothing in memory read its own copy of the tree, including the watcher that
starts with the server, so even one request loaded it twice. Callers now wait for the read under way. Peak RSS of a
cold server answering N `/status` requests at once:

| N | before | after |
|---|---|---|
| 1 | 246–250 MB | 128–145 MB |
| 4 | 587–599 MB | 146 MB |
| 8 | 976–1054 MB | 144–145 MB |

The slowest answer is now the time of one read whatever N is: 0.45–0.47 s against 0.49–0.54 s for N = 1 in the same
interleaved runs, and up to 0.62 s for N = 8 before.

### opening the window (2026-11-11)

Until the page was ready the window painted only its background, after a frame of the "Looking for ebb" screen.
index.html now carries a splash (the mark and the name) that stays until the first page is on screen. Headless
Chrome, warm sandbox server with the real scan, dark scheme, every animation frame sampled:

| | first contentful paint | page | frames with only the background before the page | "Looking for ebb" frames |
|---|---|---|---|---|
| before | 84–256 ms | 183–232 ms | 8–10 | 1–2 |
| after | 40–148 ms (the first paint) | 159–275 ms | 0 | 0 |
| before, CPU slowed 4× | 588–632 ms | 531–562 ms | 7–10 | 4 |
| after, CPU slowed 4× | 96–132 ms (the first paint) | 463–484 ms | 0 | 0 |

## ui (Explore, real scan, 2834×1285 canvas at 2× DPR, sunburst of ~3,500 arcs)

| Change | Hover to next frame, median / p95 |
|---|---|
| redraw all shapes on hover | 16.8 / 24.1 ms |
| shapes on a base canvas, hover repaints only an overlay | 16.7 / 19.3 ms (median is the frame wait) |
