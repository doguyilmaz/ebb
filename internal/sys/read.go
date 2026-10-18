package sys

import (
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"
)

// ReadSmall reads a regular file of at most max bytes without following a
// link at the path: the folders it reads from (cloned repositories, packages)
// are not the user's own, and a link to a device or a pipe there never ends.
func ReadSmall(path string, max int64) ([]byte, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("%s: not a regular file", path)
	}
	b, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > max {
		return nil, errors.New(path + ": larger than expected")
	}
	return b, nil
}
