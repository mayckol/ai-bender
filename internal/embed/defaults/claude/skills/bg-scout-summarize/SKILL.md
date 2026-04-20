---
name: bg-scout-summarize
context: bg
description: "Summarise a module: file count, public API surface, inbound/outbound dependencies. Returns a digest, not full source. Persists to the scout cache for reuse."
provides: [scout, summarize, cache]
stages: [plan, tdd, ghu, implement]
applies_to: [any]
---

# bg-scout-summarize

When asked for a module summary, return a compact digest — never the full source. Other agents read the digest to decide whether they need to fetch individual files. This is the main reason a parallel /ghu run doesn't blow the context budget: 20 files read once by scout, digested, and replayed from disk for the five agents that need orientation.

## Cache location

Persist at `.bender/cache/scout/<session-id>/modules/<path>.json` (same directory scheme as `bg-scout-explore`). Register the module in the session's `index.json` under `modules`. On a cache hit with the tree hash unchanged since the entry was written, serve the cached digest and skip re-parsing.

## Output shape (JSON in `modules/<path>.json`)

```json
{
  "schema_version": 1,
  "module": "<import path or directory>",
  "tree_hash": "<sha1 of sorted file mtimes>",
  "files":   { "total": 0, "source": 0, "test": 0, "generated": 0 },
  "public_api": [
    { "name": "<exported symbol>", "kind": "function|type|const|var", "signature": "<one line>" }
  ],
  "deps_in":  ["<module that imports this one>"],
  "deps_out": ["<module this one imports>"],
  "notes":    "<anything non-obvious>"
}
```

## Operating principles

- **Cache by module path + tree hash**. Re-summarising on cache hit is wasted I/O and wasted tokens.
- **Compact**: the whole digest should be ≤ 50 entries in `public_api` for a typical module. If a module is so large the digest exceeds that, recursively summarise its children modules first and link them from the parent's `notes`.
- **Only read public API surface**. Private helpers never appear in the digest — that's what individual file reads are for.

## Progress emission (feature 007)

Emit one `agent_progress` event per module summarised, via `bender event emit`. This keeps the UI's per-agent bar advancing while summarise is crunching modules rather than idling at 0% until completion:

```bash
bender event emit \
  --sessions-root "$SESSIONS_ROOT" \
  --session "$SESSION_ID" \
  --type agent_progress \
  --actor-kind agent \
  --actor-name scout \
  --payload '{"agent":"scout","percent":60,"current_step":"summarise internal/event"}'
```

`percent` is monotonically non-decreasing and reaches `100` before the agent returns.
