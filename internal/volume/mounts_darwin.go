package volume

import "syscall"

const (
	mntReadOnly   = 0x1
	mntLocal      = 0x1000
	mntDontBrowse = 0x100000
)

func mounts() []mount {
	n, err := syscall.Getfsstat(nil, 2) // MNT_NOWAIT
	if err != nil || n == 0 {
		return nil
	}
	buf := make([]syscall.Statfs_t, n)
	if n, err = syscall.Getfsstat(buf, 2); err != nil {
		return nil
	}
	var out []mount
	for _, st := range buf[:n] {
		bs := uint64(st.Bsize)
		out = append(out, mount{on: cstr(st.Mntonname[:]), fs: cstr(st.Fstypename[:]), capacity: st.Blocks * bs, free: st.Bavail * bs,
			rdonly: st.Flags&mntReadOnly != 0, local: st.Flags&mntLocal != 0, hide: st.Flags&mntDontBrowse != 0})
	}
	return out
}

func cstr(b []int8) string {
	s := make([]byte, 0, len(b))
	for _, c := range b {
		if c == 0 {
			break
		}
		s = append(s, byte(c))
	}
	return string(s)
}
