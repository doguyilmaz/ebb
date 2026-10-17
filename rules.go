// Package ebb embeds the built-in cleanup rules; the code lives in internal/.
package ebb

import "embed"

//go:embed rules/*.toml
var Rules embed.FS
