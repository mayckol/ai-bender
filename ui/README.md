# bender-ui

Real-time viewer for ai-bender sessions.

Reads `.bender/sessions/<id>/state.json` + `events.jsonl` from a project's
workspace and streams live events to the browser via Server-Sent Events. The
file layout is the single source of truth — this server is a read-only tail
and renderer, not a database.

## Run

```bash
bun install
bun run dev -- --project /path/to/your/project
# or
BENDER_UI_PROJECT=/path/to/your/project bun run dev
```

Default port: **4317** (override with `BENDER_UI_PORT`).

Open `http://localhost:4317/`:
- `/` — session list for the project
- `/sessions/:id` — live timeline for one session

## Auto-open from /ghu --bg

When `/ghu --bg` dispatches, the SKILL.md instructs Claude Code to print the
viewer URL. If this server is already running, the URL is clickable; if not,
start the server and refresh.

## HTTP API

| Route | Purpose |
|---|---|
| `GET /api/sessions` | Session list (same shape as `bender sessions list`). |
| `GET /api/sessions/:id` | Full snapshot `{state, events}` (same shape as `bender sessions export`). |
| `GET /api/sessions/:id/stream` | SSE: `snapshot` event first, then `event`/`state-patch` for each append/rewrite. |
| `GET /api/sessions/:id/report` | Markdown run report (if the session produced one under `.bender/artifacts/ghu/`). |

## Test

```bash
bun test
```
