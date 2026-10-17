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
deduplicated like hardlinks. CPU profile: ~90 % of time is kernel work — directory `open` by full path (~45 %) and
`getattrlistbulk` (~30 %); Go-side work is negligible. Next lever: open children with `openat` relative to the
parent descriptor (Task 13).

## full scan (`/System/Volumes/Data`: 5.95M files, 1.04M folders, 410.6 GB, no Full Disk Access)

| Build | Walk | Total (incl. detectors, saving) | Max RSS |
|---|---|---|---|
| first version | 48.6 s | 85.2 s | 491 MB |
| O(1) `Reclaimable` (shared-group LCA cache) + 256 MiB soft memory limit | 42.8 s | 44.6 s | 395 MB |
| same, `--threads 16` | 43.7 s | 45.1 s | 406 MB |
| after detector fixes (run-to-run variance) | 50.4 s | 52.3 s | 411 MB |

The walk is kernel-bound: ~165 s of system CPU across 8 workers, and 16 workers add kernel CPU without finishing
sooner, so 8 stays the default. Detectors and persistence take ~2 s. The tree holds 1.07M nodes (~100 MB) plus 225k
APFS clone groups. `Reclaimable` used to scan every shared group per call (36 s spent in the stale-project
detector); it now reads a per-node cache computed once per write in O(groups × depth): 1.4 s → 0.9 ms per 1,000 calls.
| `GOGC=50` instead of the fixed memory limit (scales with disk size) | 41.6 s | 43.4 s | 345 MB |
