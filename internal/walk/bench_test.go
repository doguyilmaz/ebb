//go:build darwin

package walk

import (
	"context"
	"os"
	"testing"
)

// BenchmarkScanHome scans EBB_BENCH_ROOT with each reader configuration.
func BenchmarkScanHome(b *testing.B) {
	root := os.Getenv("EBB_BENCH_ROOT")
	if root == "" {
		b.Skip("set EBB_BENCH_ROOT")
	}
	for _, c := range []struct {
		name string
		r    func() reader
	}{
		{"bulk", func() reader { return newBulkReader() }},
		{"generic", func() reader { return genericReader{} }},
	} {
		b.Run(c.name, func(b *testing.B) {
			for b.Loop() {
				benchReader = c.r()
				res, err := Scan(context.Background(), Options{Root: root}, 0, nil)
				benchReader = nil
				if err != nil {
					b.Fatal(err)
				}
				b.ReportMetric(float64(res.Files), "files")
			}
		})
	}
}
