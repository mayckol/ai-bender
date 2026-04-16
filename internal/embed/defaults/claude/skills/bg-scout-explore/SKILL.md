---
name: bg-scout-explore
context: bg
description: "Read-only codebase exploration: find symbol, find references, read file, list tree, grep. Caches results per session."
provides: [scout, find-symbol, find-refs, read, tree, grep]
stages: [ghu, implement]
applies_to: [any]
---

# bg-scout-explore

Other agents delegate code lookups to scout instead of grepping the tree themselves. Cache aggressively under `artifacts/.bender/cache/` so parallel agents in the same session share discoveries.

## Five operations

### 1. Find symbol
Locate the definition of a named symbol. Use the language's index (Go: `gopls`/`go doc`; TS: tsserver; Python: rope or simple AST). Cache by `artifacts/.bender/cache/symbol/<name>.json`.

### 2. Find references
List every callsite of a symbol. Cache by `artifacts/.bender/cache/refs/<name>.json`.

### 3. Read file
Use the Read tool. Honor offset/limit when the caller asked for a slice. Cache by `path + mtime`.

### 4. List tree
Use Glob with `**` patterns (e.g., `internal/**/*.go`). Cache the listing.

### 5. Grep
Use the Grep tool with a regex pattern and an optional path scope. Cache by `pattern + scope`.

## Operating principles

- **Zero write scope**. Any write attempt is a bug; the agent's `write_scope.allow: []` enforces it.
- **Cache aggressively**. Subsequent identical lookups in the same session must hit cache, not re-search.
- **Bound the result size**. For grep/list returning hundreds of matches, return the top 50 with a note that more are available.
