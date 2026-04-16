---
name: bg-scribe-update-docs
context: bg
description: "Update README, API docs (godoc/JSDoc/OpenAPI), changelog, and constitution glossary to reflect the change set."
provides: [scribe, docs, readme, api-docs, changelog, glossary]
stages: [ghu, implement]
applies_to: [any]
---

# bg-scribe-update-docs

Keep the project's documentation synchronized with the code that landed this run. **Honor `CLAUDE.md` rules**: no obvious or redundant prose. Only doc changes that add real signal.

## Four targets

### 1. README
Update Installation, Usage, and Examples sections when the change introduces or modifies a user-visible feature. Keep the existing tone. Do not rewrite sections that haven't changed.

### 2. API docs
For every changed public symbol, update its doc comment / spec entry:
- Go: package-level and exported-symbol godoc comments.
- TypeScript / JavaScript: JSDoc on exported declarations.
- Python: docstrings on public functions and classes.
- REST / GraphQL: OpenAPI / SDL files if the project uses them.

### 3. Changelog
Append one entry per merged change set, using the project's existing format (Keep a Changelog, Conventional Commits, etc.). Do not invent a new format.

### 4. Constitution glossary
If the change introduces new domain terms (new types, new concepts surfacing in public APIs), add them to `.bender/artifacts/constitution.md` Glossary section: `term → one-line definition`.

## What you DO NOT do

- Modify production code. Your write scope only allows docs and markdown.
- Update specs or artifacts at `.bender/artifacts/specs/**` / `.bender/artifacts/plan/**` (those have their own owners).
