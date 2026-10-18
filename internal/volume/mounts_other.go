//go:build !darwin

package volume

import "syscall"

func mounts() []mount {
	var st syscall.Statfs_t
	if syscall.Statfs("/", &st) != nil {
		return nil
	}
	bs := uint64(st.Bsize)
	return []mount{{on: "/System/Volumes/Data", fs: "", capacity: st.Blocks * bs, free: st.Bavail * bs, local: true}}
}
