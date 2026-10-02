// Package dambreak embeds the frontend assets and benchmark data so the
// server binary is fully self-contained.
package dambreak

import "embed"

// WebFS embeds the frontend assets (vanilla JS/HTML/CSS, no build step).
// The files live under ui/ at the repo root and are served as-is.
// `all:ui` covers the css/, js/ and fonts/ subfolders.
//
//go:embed all:ui
var WebFS embed.FS

// BenchmarkFS embeds the benchmark data files (placeholder until verified).
//
//go:embed all:benchmark
var BenchmarkFS embed.FS
