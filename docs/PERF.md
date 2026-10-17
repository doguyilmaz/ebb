# Performance

Measured on the reference Mac (Apple Silicon, 10 cores, macOS 26). Update with every engine change.

## tree

| Benchmark | Result |
|---|---|
| Build 1.1M nodes (100k dirs × 10 files) + Finish | 82 ms/op, 528 MB/op transient (slice growth) |
| Slice (depth 4, minShare 0.005, 4000 nodes) on that tree | 9 µs/op |
