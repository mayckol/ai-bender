// Package embed exposes the embedded `defaults/` tree as a fs.FS.
//
// The materializer in internal/workspace walks this tree, mirroring it into the project's `.claude/`
// (skills, agents, groups.yaml) and `.bender/` (config.yaml, from config.yaml.tmpl).
package embed

import (
	"embed"
	"io/fs"
)

//go:embed all:defaults
var raw embed.FS

// FS returns a sub-filesystem rooted at "defaults", so callers see paths like "claude/skills/...".
func FS() fs.FS {
	sub, err := fs.Sub(raw, "defaults")
	if err != nil {
		// embed.FS guarantees the subtree exists at compile time.
		panic(err)
	}
	return sub
}
