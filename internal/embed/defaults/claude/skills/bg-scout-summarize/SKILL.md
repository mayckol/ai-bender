---
name: bg-scout-summarize
context: bg
description: "Summarise a module: file count, public API surface, inbound/outbound dependencies. Returns a digest, not full source."
provides: [scout, summarize]
stages: [ghu, implement]
applies_to: [any]
---

# bg-scout-summarize

When asked for a module summary, return a compact digest — never the full source. Other agents use the digest to decide whether they need to read individual files.

## Output shape

```yaml
module: <import path or directory>
files:
  total: <int>
  by_kind: { source: N, test: N, generated: N }
public_api:
  - name: <exported symbol>
    kind: function | type | const | var
    signature: <one line>
inbound_deps:
  - <module that imports this one>
outbound_deps:
  - <module this one imports>
notes: <anything non-obvious>
```

## Operating principles

- **Cache by module path + tree hash**. Re-summarising on cache hit is wasted I/O.
- **Compact**: the whole digest should be ≤ 50 lines for a typical module. If a module is so large the digest exceeds that, recursively summarise its children instead.
