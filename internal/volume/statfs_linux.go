package volume

import "syscall"

func fromStatfs(st *syscall.Statfs_t) Breakdown {
	bs := uint64(st.Bsize)
	total := st.Blocks * bs
	return Breakdown{Capacity: total, Free: st.Bavail * bs, Used: total - st.Bavail*bs, DataUsed: (st.Blocks - st.Bfree) * bs}
}

func device(*syscall.Statfs_t) string { return "" }
