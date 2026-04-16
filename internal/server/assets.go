// Package server hosts the local bender-ui HTTP viewer.
//
// Client assets under ./assets/ are produced by `cd ui && bun run build:embed`
// and are embedded into the bender binary so `bender server` needs no runtime
// dependency on Bun, Node, or the source tree. The Bun-based development
// server under ui/ continues to work for fast iteration; the Go server ships
// the same UI as a single-binary install.
package server

import "embed"

//go:embed assets/index.html assets/client.js assets/styles.css
var clientFS embed.FS
