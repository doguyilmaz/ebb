// Package units formats byte counts the way Finder does (decimal units).
package units

import "strconv"

func Bytes(n uint64) string {
	if n < 1000 {
		return strconv.FormatUint(n, 10) + " B"
	}
	units := []string{"kB", "MB", "GB", "TB", "PB"}
	v := float64(n) / 1000
	i := 0
	for v >= 1000 && i < len(units)-1 {
		v /= 1000
		i++
	}
	s := strconv.FormatFloat(v, 'f', 1, 64)
	if len(s) > 2 && s[len(s)-2:] == ".0" {
		s = s[:len(s)-2]
	}
	return s + " " + units[i]
}
