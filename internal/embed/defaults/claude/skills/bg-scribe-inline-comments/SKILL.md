---
name: bg-scribe-inline-comments
context: bg
description: "Add inline comments only where they add real signal: non-obvious why, business rules, edge cases, side effects."
provides: [scribe, comments]
stages: [ghu, implement]
applies_to: [any]
---

# bg-scribe-inline-comments

Per CLAUDE.md: **never add comments that restate what the code does.** Only add comments that explain non-obvious *why*, hidden constraints, edge cases, or business rules a future reader would otherwise miss.

## Rules

1. **Skip the obvious**. `// loop over users` next to `for _, u := range users` is noise. Delete don't add.
2. **Skip the redundant**. If a function name and its types already convey the intent, the comment is redundant.
3. **Add real signal** in these cases:
   - A workaround for a specific bug (`// workaround for upstream issue #1234`).
   - A hidden constraint (`// MUST be called before Init() because the registry caches the result`).
   - A subtle invariant (`// callers rely on this returning a copy, not the underlying slice`).
   - Business rule that isn't obvious from the code (`// CNPJ check digit uses module 11 modified for Brazilian fiscal IDs`).
4. **Avoid attribution and timestamps**. No "added by", no "as of YYYY-MM-DD" — git history covers that.

## Output

Add the comments inline with the code change set. No standalone "comment-only" commits unless the user asked for one.
