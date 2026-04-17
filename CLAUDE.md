AGENTS.md

## Active Technologies
- Go 1.22+ + Cobra (command surface), Viper (settings precedence layering), `go-yaml` v3 (YAML parsing for agents/skills/groups/settings), Bubble Tea + Lip Gloss (multi-agent TUI renderer), `gorilla/websocket` (WebSocket sink), `net/http` (HTTP sink), `net` Unix domain sockets (local socket sink), `embed` (stdlib, embedded defaults). (001-ai-bender-pipeline)
- Local filesystem only. `.claude/` holds configuration; `artifacts/` holds per-stage outputs; `artifacts/.bender/sessions/<id>/` holds `state.json` snapshots and append-only `events.jsonl`; `artifacts/.bender/cache/` holds scout caches. No external database. (001-ai-bender-pipeline)
- Go 1.22+ (CLI + validator); Bun 1.x (bundler for UI assets, unchanged) + Cobra (CLI), `gopkg.in/yaml.v3` (YAML), stdlib `embed` / `io/fs` (asset embedding) (002-pipeline-config-move)
- Local filesystem. `.bender/pipeline.yaml` + `.bender/groups.yaml` as config; `.bender/sessions/` + `.bender/artifacts/` + `.bender/cache/` as runtime state (002-pipeline-config-move)

## Recent Changes
- 001-ai-bender-pipeline: Added Go 1.22+ + Cobra (command surface), Viper (settings precedence layering), `go-yaml` v3 (YAML parsing for agents/skills/groups/settings), Bubble Tea + Lip Gloss (multi-agent TUI renderer), `gorilla/websocket` (WebSocket sink), `net/http` (HTTP sink), `net` Unix domain sockets (local socket sink), `embed` (stdlib, embedded defaults).
