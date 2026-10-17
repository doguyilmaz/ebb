//go:build !darwin

package volume

func capacity(string) (uint64, uint64, bool) { return 0, 0, false }
