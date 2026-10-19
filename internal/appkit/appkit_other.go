//go:build !darwin

package appkit

func Bundled() bool      { return false }
func Run(func())         {}
func Notify(_, _ string) {}
