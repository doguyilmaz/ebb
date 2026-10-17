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
