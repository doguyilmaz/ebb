package walk

import "syscall"

func statFields(st *syscall.Stat_t) fields {
	return fields{dev: uint64(st.Dev), ino: st.Ino, alloc: uint64(st.Blocks) * 512, logical: uint64(st.Size),
		nlink: uint32(st.Nlink), mtime: st.Mtimespec.Sec, flags: st.Flags}
}
