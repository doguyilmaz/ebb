//go:build !darwin

package clean

// composed leaves s as it is: other systems' file systems tell the composed
// and decomposed spellings of a name apart.
func composed(s string) string { return s }
