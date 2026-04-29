// Package assets embeds the web UI files.
package assets

import "embed"

//go:embed web/*
var WebFS embed.FS
