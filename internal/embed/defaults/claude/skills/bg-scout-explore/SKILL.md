---
name: bg-scout-explore
context: bg
description: "Read-only codebase exploration: find symbol, find references, read file, list tree, grep. Persists a per-session cache so downstream agents don't re-read files."
provides: [scout, find-symbol, find-refs, read, tree, grep, cache]
stages: [plan, tdd, ghu, implement]
applies_to: [any]
---

# bg-scout-explore

Scout is the session's read-only front door. Every other agent — crafter, tester, reviewer, sentinel, benchmarker, surgeon, linter, architect, scribe — is expected to hit the scout cache before issuing its own Read/Grep/Glob calls. This is the single biggest lever for reducing token spend across a `/ghu` run: instead of N parallel agents each re-reading the same 20 files, scout reads them once, writes a digest, and the others replay from cache.

## Cache layout — write here, read from here

```
.bender/cache/scout/<session-id>/
├── index.json             # catalogue of everything scout has seen this session
├── symbols/<fqname>.json  # {"name": "...", "kind": "...", "file": "...", "line": N, "signature": "..."}
├── refs/<fqname>.json     # {"name": "...", "callsites": [{"file": "...", "line": N, "context": "..."}]}
├── modules/<path>.json    # {"path": "...", "files": N, "public_api": [...], "deps_in": [...], "deps_out": [...]}
├── grep/<sha1>.json       # {"pattern": "...", "scope": "...", "hits": [{"file", "line", "text"}]}
└── trees/<sha1>.json      # {"glob": "...", "paths": [...]}
```

`index.json` is the manifest every other agent reads first. Shape:

```json
{
  "schema_version": 1,
  "session_id": "<id>",
  "started_at": "<iso>",
  "symbols":  ["<fqname>", ...],
  "refs":     ["<fqname>", ...],
  "modules":  ["<path>", ...],
  "grep":     [{"pattern": "...", "scope": "...", "key": "<sha1>"}, ...],
  "trees":    [{"glob":    "...", "key": "<sha1>"}, ...]
}
```

Before issuing any expensive lookup, check the relevant array in `index.json`. Hit → read the cached file. Miss → do the real work, persist the result, append the new entry to `index.json`.

## Five operations

### 1. Find symbol
Locate the definition of a named symbol. Prefer the language index (Go: `gopls` / `go doc`; TS: tsserver; Python: rope or the project's LSP). Persist to `symbols/<fqname>.json`.

### 2. Find references
List every callsite of a symbol. Persist to `refs/<fqname>.json` with `(file, line, context)` triples.

### 3. Read file
Use the Read tool. Honor offset/limit when the caller asked for a slice. Cache the raw bytes + mtime; if mtime is unchanged on the next call, serve the cached bytes.

### 4. List tree
Use Glob with `**` patterns (e.g., `internal/**/*.go`). Persist the listing to `trees/<sha1-of-glob>.json`.

### 5. Grep
Use the Grep tool with a regex pattern and an optional path scope. Persist to `grep/<sha1-of-pattern+scope>.json`. Cap at 250 hits; if truncated, include `"truncated": true`.

## Operating principles

- **Zero write scope outside the cache directory**. The agent's `write_scope.allow: []` blocks everything except the cache writes (which the orchestrator explicitly permits for this skill).
- **Cache aggressively**. Subsequent identical lookups in the same session MUST be served from the on-disk cache, not re-executed.
- **Bound the result size**. Hundreds of matches → return the top 50 with a `"more_available": N` field.
- **Emit `file_changed` per cache write** so the viewer shows what scout indexed.
- **Never mutate `index.json` destructively**: append + dedupe; do not rewrite.
