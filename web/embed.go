// Package web embeds the built Vue single-page application.
package web

import "embed"

// Dist holds the Vite build output. The Makefile and Dockerfile build it before
// the Go binary is compiled.
//
//go:embed all:dist
var Dist embed.FS
