package volume

import (
	"strings"
	"syscall"
)

func fromStatfs(st *syscall.Statfs_t) Breakdown {
	bs := uint64(st.Bsize)
	total := st.Blocks * bs
	return Breakdown{Capacity: total, Free: st.Bavail * bs, Used: total - st.Bavail*bs, DataUsed: (st.Blocks - st.Bfree) * bs}
}

func device(st *syscall.Statfs_t) string {
	b := make([]byte, 0, len(st.Mntfromname))
	for _, c := range st.Mntfromname {
		if c == 0 {
			break
		}
		b = append(b, byte(c))
	}
	return strings.TrimPrefix(string(b), "/dev/")
}
