AGENTS.md

## Active Technologies
- Go 1.22+ + Cobra (command surface), Viper (settings precedence layering), `go-yaml` v3 (YAML parsing for agents/skills/groups/settings), Bubble Tea + Lip Gloss (multi-agent TUI renderer), `gorilla/websocket` (WebSocket sink), `net/http` (HTTP sink), `net` Unix domain sockets (local socket sink), `embed` (stdlib, embedded defaults). (001-ai-bender-pipeline)
- Local filesystem only. `.claude/` holds configuration; `artifacts/` holds per-stage outputs; `artifacts/.bender/sessions/<id>/` holds `state.json` snapshots and append-only `events.jsonl`; `artifacts/.bender/cache/` holds scout caches. No external database. (001-ai-bender-pipeline)
- Go 1.22+ (CLI + validator); Bun 1.x (bundler for UI assets, unchanged) + Cobra (CLI), `gopkg.in/yaml.v3` (YAML), stdlib `embed` / `io/fs` (asset embedding) (002-pipeline-config-move)
- Local filesystem. `.bender/pipeline.yaml` + `.bender/groups.yaml` as config; `.bender/sessions/` + `.bender/artifacts/` + `.bender/cache/` as runtime state (002-pipeline-config-move)
- Go 1.26.2 (per `go.mod`) (003-init-optional-skills)
- Go 1.26.2 (unchanged from 004) (005-worktree-followups)
- Local filesystem only (unchanged). (005-worktree-followups)
- Go 1.26.2 (per `go.mod`). + `cobra` (CLI), `huh` v1.0.0 (TTY forms — already in use, drives the multiple-choice prompt), `gopkg.in/yaml.v3` (existing config layer), stdlib `embed`, `os`, `bufio` (non-interactive stdin path), `golang.org/x/term` (TTY detection). (006-plan-clarifications)
- Local filesystem only. Clarifications artifact at `.bender/artifacts/plan/clarifications-<timestamp>.md`. Reuse index implicit (re-scanned from prior `clarifications-*.md` files matching the source capture). (006-plan-clarifications)
- Go 1.26.2 (per `go.mod`); TypeScript/React for the UI bundle. + Cobra (CLI), huh v1.0.0 (TTY forms), `gopkg.in/yaml.v3` (manifest + pipeline), Bubble Tea + Lip Gloss (TUI), `fsnotify` (tail), stdlib `embed` (defaults), `net/http` + SSE (live stream). Frontend: React 18, Bun bundler, no new UI deps. (007-flow-scout-init-fixes)
- Local filesystem only. `.bender/sessions/<id>/state.json` + `events.jsonl` (append-only) in the **main repo root** even when `/ghu` runs inside a worktree. `.bender/selection.yaml` for init-time preferences. `.specify/extensions.yml` for hook registry. (007-flow-scout-init-fixes)

## Recent Changes
- 001-ai-bender-pipeline: Added Go 1.22+ + Cobra (command surface), Viper (settings precedence layering), `go-yaml` v3 (YAML parsing for agents/skills/groups/settings), Bubble Tea + Lip Gloss (multi-agent TUI renderer), `gorilla/websocket` (WebSocket sink), `net/http` (HTTP sink), `net` Unix domain sockets (local socket sink), `embed` (stdlib, embedded defaults).
