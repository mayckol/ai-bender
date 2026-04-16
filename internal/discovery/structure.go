package discovery

import (
	"path/filepath"
)

// DetectStructure reports significant top-level folders and likely entry points.
func DetectStructure(p Probe) StructureInfo {
	var s StructureInfo
	known := map[string]bool{
		"cmd": true, "internal": true, "pkg": true,
		"src": true, "app": true, "apps": true, "packages": true,
		"lib": true, "libs": true, "service": true, "services": true,
		"api": true, "web": true, "frontend": true, "backend": true,
		"tests": true, "test": true, "docs": true, "scripts": true,
	}
	for _, dir := range p.TopLevelDirs() {
		base := filepath.Base(dir)
		if known[base] {
			s.Folders = append(s.Folders, base)
		}
	}
	for _, f := range p.TopLevelFiles() {
		switch f {
		case "main.go", "index.js", "index.ts", "app.py", "main.py", "Main.java", "main.rs":
			s.EntryPoints = append(s.EntryPoints, f)
		}
	}
	if p.HasAnyUnder("cmd") {
		s.EntryPoints = append(s.EntryPoints, "cmd/<bin>/main.go")
	}
	return s
}
