// Package web embeds the built Vue frontend.
package web

import "embed"

// Dist holds the Vite build output. Empty until `npm run build` has run.
//
//go:embed all:dist
var Dist embed.FS
