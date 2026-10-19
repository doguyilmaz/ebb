package cli

import (
	"os"
	"syscall"
	"unsafe"
)

// interactive reports whether a person can answer on stdin: a terminal, not a
// pipe or /dev/null, which is a character device too.
func interactive() bool {
	var t syscall.Termios
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, os.Stdin.Fd(), syscall.TIOCGETA, uintptr(unsafe.Pointer(&t)))
	return errno == 0
}
