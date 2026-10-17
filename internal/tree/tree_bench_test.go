package tree

import (
	"strconv"
	"testing"
)

func synthetic(dirs, files int) *Tree {
	tr := New("/r")
	tr.Write(func(w *Writer) {
		parent := tr.Root()
		for i := 0; i < dirs; i++ {
			if i%100 == 0 {
				parent = w.AddDir(tr.Root(), []byte("top"+strconv.Itoa(i)), 1, 0)
			}
			d := w.AddDir(parent, []byte("dir"+strconv.Itoa(i)), int64(i), 0)
			for j := 0; j < files; j++ {
				w.AddFile(d, []byte("file"+strconv.Itoa(j)), KindFile, uint64(j+1)<<20, uint64(j+1)<<20, uint64(j+1)<<20, int64(j), 0)
			}
		}
	})
	return tr
}

func BenchmarkBuild1M(b *testing.B) {
	for b.Loop() {
		synthetic(100_000, 10).Finish()
	}
}

func BenchmarkSlice(b *testing.B) {
	tr := synthetic(100_000, 10)
	tr.Finish()
	b.ResetTimer()
	for b.Loop() {
		tr.Slice(tr.Root(), 4, 0.005, 4000)
	}
}
