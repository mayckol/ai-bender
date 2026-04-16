---
name: bender-bootstrap
user-invocable: true
context: fg
description: "Fill the AI-required sections of .bender/artifacts/constitution.md (purpose, conventions, glossary) by reading the codebase. Archives the prior constitution."
provides: [stage, bootstrap, refine, constitution]
stages: [bootstrap]
applies_to: [any]
inputs:
  - .bender/artifacts/constitution.md
outputs:
  - .bender/artifacts/constitution.md
  - .bender/artifacts/constitution/<timestamp>.md
---

# `/bender-bootstrap` — Refine the Constitution

`bender init` populates the constitution's mechanical sections (stack, structure, tests, lint, build, dependencies) using heuristic file probes. The three sections it can't fill on its own — **Purpose**, **Conventions**, **Glossary** — are marked `_pending: true` and wait for this skill.

This skill reads the codebase, fills those three sections, archives the prior constitution under `.bender/artifacts/constitution/<timestamp>.md`, and writes the new one.

## Pre-Execution Checks

Run any `hooks.before_bootstrap` from `.specify/extensions.yml`.

## Workflow

### 1. Confirm the constitution exists and has pending sections

- If `.bender/artifacts/constitution.md` does not exist, print:
  - `error: .bender/artifacts/constitution.md not found. Run \`bender init\` first.`
  - Stop.
- Read the file. Locate every section whose body contains `_pending: true`. The expected ones are `## Purpose`, `## Conventions`, `## Glossary`. If none are pending, print "constitution already complete; nothing to do" and stop.

### 2. Create a session directory

- Path: `.bender/sessions/<timestamp>-<rand3>/`.
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

1. Move the current `.bender/artifacts/constitution.md` to `.bender/artifacts/constitution/<timestamp>.md` (use the same `YYYY-MM-DDTHH-MM-SS` format `bender init` uses; collision suffix `-1`, `-2`, … if needed).
2. Write the new constitution to `.bender/artifacts/constitution.md` with the three filled sections substituted in.
3. Update the frontmatter `created_at` and `tool_version` (mark `tool_version: bender-bootstrap-refined`).

### 7. Emit events

Use the exact shapes in "Observability shape" below.

Order:
1. `session_started`
2. `stage_started` (stage = `bootstrap`)
3. `skill_invoked` + matching `skill_completed` for each filled section (`fill_purpose`, `fill_conventions`, `fill_glossary`)
4. `artifact_written` for the archived old constitution (first — the one you're about to overwrite)
5. `artifact_written` for the new refined constitution
6. `stage_completed`
7. `session_completed`

Then update `state.json` with `status: completed`, `completed_at`, `files_changed: 2`, `findings_count: 0`.

### 8. Print

Tell the user what was filled, what was archived, and the next suggested command (typically `/cry "<your first feature request>"`).

## Observability shape — emit verbatim, do NOT invent fields

Same envelope as every other slash command. Stage is **`bootstrap`** for every event.

### session_started
```json
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"user","name":"claude-code"},"type":"session_started","payload":{"command":"/bender-bootstrap","invoker":"<$USER>","working_dir":"<abs path>"}}
```

### stage_started
```json
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"stage","name":"bootstrap"},"type":"stage_started","payload":{"stage":"bootstrap","inputs":[".bender/artifacts/constitution.md"]}}
```

### skill_invoked / skill_completed (one pair per filled section)
```json
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"stage","name":"bootstrap"},"type":"skill_invoked","payload":{"skill":"fill_purpose","agent":"bootstrap"}}
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"stage","name":"bootstrap"},"type":"skill_completed","payload":{"skill":"fill_purpose","agent":"bootstrap","duration_ms":<int>,"outputs":[]}}
```

### artifact_written (two: archived old, then new current)
```json
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"stage","name":"bootstrap"},"type":"artifact_written","payload":{"path":".bender/artifacts/constitution/<ts>.md","stage":"bootstrap","checksum":"<sha256 of archived>","bytes":<int>}}
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"stage","name":"bootstrap"},"type":"artifact_written","payload":{"path":".bender/artifacts/constitution.md","stage":"bootstrap","checksum":"<sha256 of new>","bytes":<int>}}
```

### stage_completed / session_completed
```json
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"stage","name":"bootstrap"},"type":"stage_completed","payload":{"stage":"bootstrap","inputs":[".bender/artifacts/constitution.md"],"outputs":[".bender/artifacts/constitution/<ts>.md",".bender/artifacts/constitution.md"]}}
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"orchestrator","name":"core"},"type":"session_completed","payload":{"status":"completed","duration_ms":<int>,"agents_summary":[]}}
```

### state.json (overwrite in place)
```json
{
  "schema_version": 1,
  "session_id": "<session_id>",
  "command": "/bender-bootstrap",
  "started_at": "<iso>",
  "completed_at": "<iso, once terminal>",
  "status": "running|completed|failed",
  "source_artifacts": [".bender/artifacts/constitution.md"],
  "skills_invoked": ["fill_purpose","fill_conventions","fill_glossary"],
  "files_changed": 2,
  "findings_count": 0
}
```

### Forbidden shortcuts
- `ts` / `event` / inlined payload fields / missing `schema_version|session_id|actor|payload` — all WRONG.
- Stage names like `bootstrap_constitution` — WRONG. Use `bootstrap`.
- `kind` field on `artifact_written` (values like `archived_constitution`, `constitution`) — WRONG. The contract is `{path, stage, checksum, bytes}`. Claude can distinguish the two `artifact_written` events by their `payload.path`.

## Post-Execution

Run any `hooks.after_bootstrap`.

## What you DO NOT do

- Modify the mechanical sections (Stack, Structure, Tests, Lint, Build/CI, Dependencies). Those are `bender init`'s domain — re-run `bender init` if they need refresh.
- Touch source code. Your job ends at the constitution.
- Skip archiving. Even if the prior constitution had only pending sections, archive it so the revision history stays unbroken.
