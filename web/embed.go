// Package web embeds the built SvelteKit static assets for single-binary distribution.
package web

import "embed"

// Assets contains the SvelteKit production build output.
// The build/ directory is created by `pnpm run build` in the web/ directory.
//
//go:embed all:build
var Assets embed.FS
