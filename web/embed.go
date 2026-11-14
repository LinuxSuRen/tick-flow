// Package web embeds the built frontend so the single server binary
// serves both the API and the UI. dist is a build artifact; .gitkeep keeps
// the embed valid in a fresh checkout before the first web build.
package web

import "embed"

// Assets holds the compiled SPA.
//
//go:embed all:dist
var Assets embed.FS
