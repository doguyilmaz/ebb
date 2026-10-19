//go:build !darwin

package cli

// The background service is macOS only, so elsewhere ebb open never asks.
func interactive() bool { return false }
