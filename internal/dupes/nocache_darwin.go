package dupes

import (
	"os"
	"syscall"
)

func noCache(f *os.File) { _, _, _ = syscall.Syscall(syscall.SYS_FCNTL, f.Fd(), syscall.F_NOCACHE, 1) }
