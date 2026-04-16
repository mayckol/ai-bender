// Package constitution renders artifacts/constitution.md from discovery output.
//
// The current constitution always lives at artifacts/constitution.md. When a new one is written,
// the prior file (if any) is moved into artifacts/constitution/<timestamp>.md so revisions accumulate.
package constitution

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"text/template"
	"time"

	"github.com/mayckol/ai-bender/internal/artifact"
	"github.com/mayckol/ai-bender/internal/discovery"
	"github.com/mayckol/ai-bender/internal/version"
)

const (
	currentRel  = "artifacts/constitution.md"
	revisionDir = "artifacts/constitution"
)

// Render generates the constitution markdown for a discovery Result. Used by Write but exposed
// for tests that want to inspect the bytes without touching the filesystem.
func Render(r discovery.Result, now time.Time) ([]byte, error) {
	var buf bytes.Buffer
	data := struct {
		CreatedAt    string
		Version      string
		Result       discovery.Result
		PendingFlags pendingFlags
	}{
		CreatedAt:    now.UTC().Format(time.RFC3339),
		Version:      version.Version,
		Result:       r,
		PendingFlags: derivePendingFlags(r),
	}
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("constitution: render: %w", err)
	}
	return buf.Bytes(), nil
}

// Write renders and atomically installs the new constitution at projectRoot, archiving any prior
// constitution under artifacts/constitution/<timestamp>.md. Returns the absolute path written.
func Write(projectRoot string, r discovery.Result, now time.Time) (string, error) {
	if err := os.MkdirAll(filepath.Join(projectRoot, "artifacts"), 0o755); err != nil {
		return "", err
	}
	currentPath := filepath.Join(projectRoot, currentRel)
	if err := archivePrior(projectRoot, currentPath, now); err != nil {
		return "", err
	}
	body, err := Render(r, now)
	if err != nil {
		return "", err
	}
	if err := writeFileAtomic(currentPath, body); err != nil {
		return "", err
	}
	return currentPath, nil
}

func archivePrior(projectRoot, currentPath string, now time.Time) error {
	if _, err := os.Stat(currentPath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	if err := os.MkdirAll(filepath.Join(projectRoot, revisionDir), 0o755); err != nil {
		return err
	}
	base := artifact.FormatTime(now)
	exists := func(name string) bool {
		_, err := os.Stat(filepath.Join(projectRoot, revisionDir, name+".md"))
		return err == nil
	}
	suffix, err := artifact.WithCollisionSuffix(base, exists)
	if err != nil {
		return err
	}
	target := filepath.Join(projectRoot, revisionDir, suffix+".md")
	return os.Rename(currentPath, target)
}

func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".constitution.*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

type pendingFlags struct {
	Purpose      bool
	Stack        bool
	Structure    bool
	Tests        bool
	Lint         bool
	Build        bool
	CICD         bool
	Dependencies bool
	Conventions  bool
	Glossary     bool
}

func derivePendingFlags(r discovery.Result) pendingFlags {
	return pendingFlags{
		Purpose:      true,
		Conventions:  true,
		Glossary:     true,
		Stack:        r.Stack.IsZero(),
		Structure:    r.Structure.IsZero(),
		Tests:        r.Tests.IsZero(),
		Lint:         r.Lint.IsZero(),
		Build:        r.Build.IsZero(),
		CICD:         r.CICD.IsZero(),
		Dependencies: len(r.Dependencies) == 0,
	}
}

var tmpl = template.Must(template.New("constitution").Funcs(template.FuncMap{
	"join": func(sep string, items []string) string {
		out := ""
		for i, s := range items {
			if i > 0 {
				out += sep
			}
			out += s
		}
		return out
	},
}).Parse(constitutionTmpl))

const constitutionTmpl = `---
created_at: {{.CreatedAt}}
tool_version: {{.Version}}
---

# Project Constitution

## Purpose
{{if .PendingFlags.Purpose}}_pending: true — run ` + "`/cry`" + ` or ` + "`/bender-bootstrap`" + ` to fill this in._{{else}}{{end}}

## Stack
{{- if .PendingFlags.Stack}}
_pending: true — no manifest detected._
{{- else}}
- Language: {{.Result.Stack.Language}}
- Package manager: {{.Result.Stack.PackageManager}}
{{- if .Result.Stack.Frameworks}}
- Frameworks: {{join ", " .Result.Stack.Frameworks}}
{{- end}}
{{- end}}

## Structure
{{- if .PendingFlags.Structure}}
_pending: true — no recognisable folder layout._
{{- else}}
{{- if .Result.Structure.Folders}}
- Folders: {{join ", " .Result.Structure.Folders}}
{{- end}}
{{- if .Result.Structure.EntryPoints}}
- Entry points: {{join ", " .Result.Structure.EntryPoints}}
{{- end}}
{{- end}}

## Tests
{{- if .PendingFlags.Tests}}
_pending: true — no test framework detected._
{{- else}}
- Framework: {{.Result.Tests.Framework}}
{{- if .Result.Tests.Conventions}}
- Conventions: {{.Result.Tests.Conventions}}
{{- end}}
{{- if .Result.Tests.CoverageTool}}
- Coverage: {{.Result.Tests.CoverageTool}}
{{- end}}
{{- end}}

## Lint
{{- if .PendingFlags.Lint}}
_pending: true — no linter or formatter detected._
{{- else}}
{{- if .Result.Lint.Linters}}
- Linters: {{join ", " .Result.Lint.Linters}}
{{- end}}
{{- if .Result.Lint.Formatters}}
- Formatters: {{join ", " .Result.Lint.Formatters}}
{{- end}}
- Pre-commit: {{.Result.Lint.PreCommit}}
{{- end}}

## Build / CI
{{- if and .PendingFlags.Build .PendingFlags.CICD}}
_pending: true — no build tool or CI provider detected._
{{- else}}
{{- if not .PendingFlags.Build}}
- Build tool: {{.Result.Build.Tool}}
- Makefile: {{.Result.Build.HasMakefile}}
- Dockerfile: {{.Result.Build.HasDockerfile}}
{{- end}}
{{- if not .PendingFlags.CICD}}
- CI: {{.Result.CICD.Provider}}
{{- if .Result.CICD.DeploymentTargets}}
- Deployment: {{join ", " .Result.CICD.DeploymentTargets}}
{{- end}}
{{- end}}
{{- end}}

## Conventions
_pending: true — naming / error-handling / DI / architecture pattern (run ` + "`/bender-bootstrap`" + `)._

## Glossary
_pending: true — recurring domain terms (run ` + "`/bender-bootstrap`" + `)._

## Dependencies
{{- if .PendingFlags.Dependencies}}
_pending: true — no dependency manifest parsed._
{{- else}}
{{- range .Result.Dependencies}}
- {{.Name}} {{.Version}}
{{- end}}
{{- end}}

## Pending Items
{{- range .Result.Pending}}
- {{.}}
{{- end}}
`
