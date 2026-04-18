---
name: bg-mistakeinator-record
context: bg
description: "Append a learned-mistake entry to `.bender/artifacts/mistakes.md`. Dedupes by `id:` so re-recording the same lesson is a no-op."
provides: [mistakes, record, artifact]
stages: [ghu, implement]
applies_to: [any]
---

# bg-mistakeinator-record

Append one new Mistake entry to the project-local artifact at
`.bender/artifacts/mistakes.md`. Entries are keyed by a stable `id:` slug;
re-recording an existing id is a no-op. The file is human-readable Markdown
with YAML frontmatter per entry — designed to be edited, curated, and
version-controlled alongside the codebase.

## Input

- **id**: stable, kebab-case slug (e.g. `user-service-swallow-errors`). Required.
- **scope**: repo-relative file path, module name, or `tag:<free-form>`. Required.
- **tags**: optional list of short tokens (e.g. `[errors, data-loss]`).
- **title**: one-line human-readable title. Required.
- **avoid**: the anti-pattern (what not to do). Required.
- **prefer**: counter-example (what to do instead). Optional.

## Output — one appended Markdown block

```markdown
---
id: <id>
scope: <scope>
tags: [<tags...>]
created: <RFC3339 timestamp>
---

## <title>

**Avoid:** <avoid>

**Prefer:** <prefer>
```

## Contract

- Append-only. Existing `id` → exit 0 with `already recorded`.
- Creates `.bender/artifacts/` if missing.
- Never rewrites previously-recorded entries.
- Malformed frontmatter anywhere in the file causes readers to treat the
  whole file as unavailable — keep the append atomic; never leave a
  half-written entry on disk.
