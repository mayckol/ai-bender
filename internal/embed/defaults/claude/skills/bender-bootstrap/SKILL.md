---
name: bender-bootstrap
context: fg
description: "Fill the AI-required sections of artifacts/constitution.md (purpose, conventions, glossary) by reading the codebase. Archives the prior constitution."
provides: [stage, bootstrap, refine, constitution]
stages: [bootstrap]
applies_to: [any]
inputs:
  - artifacts/constitution.md
outputs:
  - artifacts/constitution.md
  - artifacts/constitution/<timestamp>.md
---

# `/bender-bootstrap` — Refine the Constitution

`bender init` populates the constitution's mechanical sections (stack, structure, tests, lint, build, dependencies) using heuristic file probes. The three sections it can't fill on its own — **Purpose**, **Conventions**, **Glossary** — are marked `_pending: true` and wait for this skill.

This skill reads the codebase, fills those three sections, archives the prior constitution under `artifacts/constitution/<timestamp>.md`, and writes the new one.

## Pre-Execution Checks

Run any `hooks.before_bootstrap` from `.specify/extensions.yml`.

## Workflow

### 1. Confirm the constitution exists and has pending sections

- If `artifacts/constitution.md` does not exist, print:
  - `error: artifacts/constitution.md not found. Run \`bender init\` first.`
  - Stop.
- Read the file. Locate every section whose body contains `_pending: true`. The expected ones are `## Purpose`, `## Conventions`, `## Glossary`. If none are pending, print "constitution already complete; nothing to do" and stop.

### 2. Create a session directory

- Path: `artifacts/.bender/sessions/<timestamp>-<rand3>/`.
- Write `state.json`: `{ schema_version: 1, command: "/bender-bootstrap", status: "running", started_at: "<iso>" }`.
- Append `session_started` and `stage_started` events to `events.jsonl`.

### 3. Fill `## Purpose` (if pending)

Read these sources, in order:
- `README.md` (or `README.rst`, `README.txt`) — primary signal.
- `package.json` `description`, `pyproject.toml` `[project] description`, `Cargo.toml` `package.description`, `go.mod` (sometimes a `// purpose:` comment), `composer.json` `description`.
- Top-level docs (`docs/README.md`, `docs/index.md`, `docs/intro.md`).
- The repo URL (org/repo) as a tiebreaker for naming.

Synthesize a single paragraph (3–6 sentences) describing:
- What the project is.
- Who uses it.
- The primary problem it solves.
- Any notable scope boundary (what it deliberately does NOT do).

Replace the `_pending: true` line under `## Purpose` with this paragraph.

### 4. Fill `## Conventions` (if pending)

Sample 10–30 representative source files (avoid generated code, tests, and vendored deps). Use `bg-scout-explore` if available, otherwise the Read tool. Identify:

- **Naming**: snake_case / camelCase / kebab-case / PascalCase, where each is used (variables vs types vs files).
- **Error handling**: language-appropriate. Examples: Go — `if err != nil { return fmt.Errorf("...: %w", err) }`; TypeScript — `Result<T, E>` types vs throwing; Python — exception classes vs return-tuple; Rust — `?` operator vs explicit match.
- **Dependency injection**: constructor injection / global / IoC container / functional currying.
- **Architectural pattern**: clean architecture, hexagonal, MVC, layered, microservices, monolith, event-driven. Pick one or "mixed" with a one-line justification.

Render as four bullet entries under `## Conventions`. Do not be exhaustive — capture the dominant pattern, not every exception.

### 5. Fill `## Glossary` (if pending)

Identify recurring domain terms — nouns that appear in:
- Public type names (exported structs/classes/interfaces).
- Public function/method names.
- README headings.
- Top-level package/module names.

Filter out generic CS terms (Service, Manager, Handler, Util, Config, Options) unless they have a specific domain meaning in this codebase.

Render as a bulleted list:
- `<Term>`: <one-line definition tied to how the codebase uses it>

Aim for 5–15 entries. If the codebase has none beyond CS terms, write `_no recurring domain terms detected_` and move on.

### 6. Archive and write

1. Move the current `artifacts/constitution.md` to `artifacts/constitution/<timestamp>.md` (use the same `YYYY-MM-DDTHH-MM-SS` format `bender init` uses; collision suffix `-1`, `-2`, … if needed).
2. Write the new constitution to `artifacts/constitution.md` with the three filled sections substituted in.
3. Update the frontmatter `created_at` and `tool_version` (mark `tool_version: bender-bootstrap-refined`).

### 7. Emit events

- `artifact_written` for both the archived old constitution and the new one (each with sha256 + byte count).
- `skill_completed` for each filled section.
- `stage_completed` and `session_completed` (status: completed) at the end.
- Update `state.json` to `status: completed` plus `files_changed: 2` and `findings_count: 0`.

### 8. Print

Tell the user what was filled, what was archived, and the next suggested command (typically `/cry "<your first feature request>"`).

## Post-Execution

Run any `hooks.after_bootstrap`.

## What you DO NOT do

- Modify the mechanical sections (Stack, Structure, Tests, Lint, Build/CI, Dependencies). Those are `bender init`'s domain — re-run `bender init` if they need refresh.
- Touch source code. Your job ends at the constitution.
- Skip archiving. Even if the prior constitution had only pending sections, archive it so the revision history stays unbroken.
