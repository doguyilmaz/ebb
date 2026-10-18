//go:build !darwin

package dupes

import "os"

func noCache(*os.File) {}
