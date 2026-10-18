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

## ui (Explore, real scan, 2834×1285 canvas at 2× DPR, sunburst of ~3,500 arcs)

| Change | Hover to next frame, median / p95 |
|---|---|
| redraw all shapes on hover | 16.8 / 24.1 ms |
| shapes on a base canvas, hover repaints only an overlay | 16.7 / 19.3 ms (median is the frame wait) |
