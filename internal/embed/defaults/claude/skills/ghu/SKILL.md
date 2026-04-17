---
name: ghu
user-invocable: true
argument-hint: "[--bg | --inline] [--from=<spec>] [--only=<task-id>] [--skip=<name>[,<name>...]] [--abort-on-failure]"
context: fg
description: "Execute the approved plan — implement, test, lint, review, and report. The only stage that writes code."
provides: [stage, execute]
stages: [ghu]
applies_to: [any]
inputs:
  - .bender/artifacts/specs/*.md
  - .bender/artifacts/plan/tasks-*.md
outputs:
  - .bender/artifacts/ghu/run-<timestamp>-report.md
---

# `/ghu` — Execute the Plan

Decompose the approved task list, dispatch work to specialised agents, and produce a final report. This is the **only** stage that writes source code.

## User Input

```text
$ARGUMENTS
```

## Execution Mode — READ FIRST

`/ghu` supports two execution modes, selected by flags in `$ARGUMENTS`:

- **`--bg` (default)** — Isolated-subagent mode. The workflow runs inside a forked `Agent` context so the main conversation is not polluted with file reads, tool outputs, and agent orchestration. This is the recommended mode for full runs.
- **`--inline`** — Inline mode. The workflow runs directly in the current conversation. Use this for debugging, short scoped runs (`--only=<task>`), or when you explicitly want to observe each step.

### Dispatcher (what the MAIN conversation does)

**Step 0 — Before doing anything else, parse `$ARGUMENTS` and branch:**

1. If `$ARGUMENTS` contains `--inline` → skip to the "Workflow" section below and execute it directly in this conversation.
2. Otherwise (default, or `--bg` explicitly) → delegate to a subagent:
   - Invoke the **Agent tool** exactly once with:
     - `subagent_type: general-purpose`
     - `run_in_background: true`
     - `description`: `"ghu background run"`
     - `prompt`: a self-contained message that includes (a) the full body of this SKILL.md from the "Workflow" section onward, (b) the user's `$ARGUMENTS` (with `--bg` stripped), and (c) the absolute working directory.
   - After launching, print to the user exactly:
     - The new session ID (generated before launch).
     - The target report path (`.bender/artifacts/ghu/run-<timestamp>-report.md`).
     - The viewer URL: `http://localhost:4317/sessions/<session-id>` — start the viewer with `bender server` (detached by default; `bender server stop` to stop).
     - A note that execution is running in the background and they will be notified on completion.
   - **Best-effort auto-open.** If the viewer is running (TCP probe of `127.0.0.1:4317` succeeds — use `nc -z localhost 4317` or equivalent), invoke the platform opener once and ignore failure: `open http://localhost:4317/sessions/<id>` on macOS, `xdg-open` on Linux, `powershell Start-Process` on Windows. If the probe fails, do not attempt to open — just print the URL plus a hint like `Run 'bender server' to launch the viewer.`.
   - **Exit the main turn.** Do NOT execute the Workflow section in the main conversation when delegating.

The main conversation's sole responsibility in `--bg` mode is to dispatch and report the launch. All orchestration, file writes, and agent invocations happen inside the forked subagent.

## Actor discipline — WHO emits WHICH events

This table is a rule, not a suggestion. Events that violate it break the viewer's agent threading and the `bender sessions validate` contract.

| Event type(s) | `actor.kind` | `actor.name` | `payload.agent` | Notes |
|---|---|---|---|---|
| `session_started` | `user` | `claude-code` | absent | Emitted once by the orchestrator on session start. |
| `stage_started`, `stage_completed`, `stage_failed`, `artifact_written` | `stage` | `ghu` | absent | Stage-owned lifecycle events. |
| `orchestrator_decision`, `session_completed` | `orchestrator` | `core` | absent | Orchestrator-owned. When the decision targets a specific agent, set `payload.dispatched_agent` (and `payload.dispatched_agents` for parallel dispatch). |
| `agent_started`, `agent_progress`, `agent_completed`, `agent_failed`, `agent_blocked` | `agent` | `<agent name>` | `<agent name>` | Worker-owned. |
| `skill_invoked`, `skill_completed`, `skill_failed` | `agent` | `<agent name>` | `<agent name>` | Worker-owned; plus `skill`. |
| `file_changed` | `agent` | `<agent name>` | `<agent name>` | The agent that wrote the file. |
| `finding_reported` | `agent` | `<reporter>` (reviewer/sentinel/benchmarker/linter) | `<reporter>` | Whichever agent surfaced the finding. |

Forbidden combinations:
- `payload.agent` on orchestrator/stage/user events — WRONG. Use `dispatched_agent` on orchestrator decisions instead.
- `actor.kind: "agent"` without a `payload.agent` matching `actor.name` — WRONG.
- Orchestrator-emitted events with `actor.kind: "agent"` — WRONG.

## Event emission discipline — STREAM, never batch

**Every** event in "Observability shape" MUST be appended to `.bender/sessions/<id>/events.jsonl` **the moment its trigger happens** — **one Bash tool call per event**, not a single `Write` at the end. The bender-ui viewer tails the file via fsnotify; batching collapses the timeline into one notification and the user sees `Waiting for events…` for the full run. This applies to both the orchestrator (main or `--bg` subagent) AND every worker subagent it dispatches — each worker emits its own events inside its own context using the same mechanism.

```bash
printf '%s\n' '<single-line JSON>' >> .bender/sessions/<id>/events.jsonl
```

Ordering: intent events (`skill_invoked`, `orchestrator_decision`, `agent_started`) append BEFORE the action; result events (`file_changed`, `artifact_written`, `skill_completed`, `agent_completed`, `stage_completed`, `session_completed`) append AFTER. `orchestrator_progress` + `agent_progress` append as their percent changes — not once at the end. Never buffer events and flush them with one `Write`.

## Pre-Execution Checks

Run any `hooks.before_ghu`.

## Workflow

1. **Resolve the source artifacts**:
   - The latest **approved** spec at `.bender/artifacts/specs/<slug>-<ts>.md`.
   - The matching task list at `.bender/artifacts/plan/tasks-<ts>.md`.
   - Optionally, test scaffolds at `.bender/artifacts/plan/tests/`.
   - If anything required is missing, print:
     - `error: missing required upstream artifact: <name>. Run /plan and /plan confirm first.`
     - Exit. Do **not** create a partial session.

2. **Detect TDD mode.** Glob `.bender/artifacts/plan/tests/**/*.md`. If the glob is non-empty AND every matched file's frontmatter has `status: approved`, set **`tdd_mode = true`** for the remainder of this run. Record the decision with an `orchestrator_decision` event (`decision_type: "execution_mode"`, payload `mode: "tdd"` or `mode: "plain"`, plus `scaffold_count`). Skip this step if there are no scaffolds (proceed in plain mode).

3. **Honor `--only=<task>`** to scope to a single task (same as `/implement`).

4. **Honor `--skip=<name>[,<name>...]`** to bypass agents or groups. Accepts the following names (aliases in parentheses):

   | Name | Alias(es) | What it skips |
   |---|---|---|
   | `linter` | `lint` | the linter agent + `bg-linter-*` skills |
   | `tester` | `test`, `tests` | the tester agent (all of `bg-tester-*` and `fg-tester-*`) |
   | `reviewer` | `review` | the reviewer agent's critique + PR summary |
   | `sentinel` | `security`, `sec` | the sentinel agent's security pass |
   | `benchmarker` | `perf`, `bench` | the benchmarker agent's perf pass |
   | `scribe` | `docs` | the scribe agent's doc + inline-comment updates |
   | `surgeon` | `refactor` | the surgeon agent's refactor pass |
   | `architect` | — | the architect validation pass during /ghu |
   | `review-sweep` | `reviews` | the whole `review-sweep` group (reviewer, sentinel, benchmarker, scribe) |

   Rules:
   - **`crafter` is not skippable.** `/ghu` that skips crafter produces nothing. Reject with `error: cannot skip crafter — /ghu has nothing to do without it.`
   - **`scout` is not skippable.** Its cache is what makes downstream agents token-efficient; skipping it forces every other agent to re-read the tree.
   - **In TDD mode**, `tester` skipping is rejected: the `tdd-cycle` group needs `bg-tester-scaffold` and `bg-tester-run`, so dropping the tester agent breaks the cycle. Print `error: --skip=tester is not compatible with TDD mode (approved scaffolds under .bender/artifacts/plan/tests/). Remove the scaffolds or drop --skip.`.
   - **Unknown names** are a hard error listing the valid names above.
   - Every resolved skip entry MUST emit an `orchestrator_decision` event with `decision_type: "skip"` and payload `{"target": "<canonical-name>", "reason": "user_requested", "alias": "<what the user typed>"}` so the viewer and `bender sessions validate` can show what was bypassed.

5. **Honor `--abort-on-failure`** to halt on the first task failure (default: continue and mark blocked).

6. **Create a session directory** under `.bender/sessions/<id>/`. Write `state.json` and append `session_started`, `stage_started`, `orchestrator_decision` (with `decision_type: "task_decomposition"`). The decomposition payload MUST carry the task list AND the dependency edges extracted from the tasks file: `{"decision_type":"task_decomposition","tasks":["T001","T002",...],"dependencies":[{"task":"T002","depends_on":["T001"]},...]}`. Tasks with no incoming edges are the first wave; everything else must wait for its prerequisites.

7. **Walk the execution graph**, which depends on `tdd_mode`:

   **Plain mode** (no approved scaffolds) — walks the `plain-cycle` + `review-sweep` groups from `.claude/groups.yaml`:
   ```
   scout (map codebase) → architect (validate approach)
   ↓
   surgeon (if refactor needed)
   ↓
   plain-cycle group (parallel, halt_on_failure=false):
     crafter → bg-crafter-implement   ∥   tester → bg-tester-write-and-run
   ↓
   linter (autofix + report)
   ↓
   review-sweep group (parallel, halt_on_failure=false):
     reviewer ∥ sentinel ∥ benchmarker ∥ scribe
   ↓
   final report
   ```

   **TDD mode** (approved scaffolds present) — walks the `tdd-cycle` group from `.claude/groups.yaml`:
   ```
   scout (map codebase) → architect (validate approach)
   ↓
   surgeon (if refactor needed)
   ↓
   tdd-cycle group (ordered, halt_on_failure):
     1. tester → bg-tester-scaffold
          • reads approved prose scaffolds under .bender/artifacts/plan/tests/
          • writes commented-out test stubs at the real test paths
            (sibling _test.go / .test.ts / …) with mock setup, subject construction, and
            asserts commented out plus `// TODO - implement the Code` markers
          • emit finding_reported(severity: info, category: "tdd_scaffolded")
     2. crafter → bg-crafter-implement
          • reads the stubs + source tasks, implements production code, activates the
            commented-out assertions / mock setup as it goes
          • may invoke bg-tester-run for its own inner feedback loop (not authoritative)
     3. tester → bg-tester-run
          • runs the resolved test command on the final state
          • suite MUST pass; red → finding_reported(severity: high, category: "test_failure")
            per failing test and halt (halt_on_failure)
          • on green → finding_reported(severity: info, category: "tdd_green") with
            tests_total + tests_passed + duration_ms
   ↓
   [REFACTOR] surgeon/crafter cleanup pass — tests stay green (re-run bg-tester-run after changes)
   ↓
   linter (autofix + report)
   ↓
   review-sweep group (parallel, halt_on_failure=false):
     reviewer ∥ sentinel ∥ benchmarker ∥ scribe
   ↓
   final report (flag the TDD mode in the summary header, cite the
   tdd_scaffolded and tdd_green findings)
   ```

   The `tdd-cycle` group is **ordered** so tester scaffolds first, crafter implements second, tester runs third. Switch the order at your own peril — the commented-out asserts only mean anything if crafter sees them before touching production code. The test-runner command comes from the constitution's Tests section, falling back to marker-file autodetect (see `bg-tester-run`).

   For each agent invocation (both modes):
   - Use the **Agent tool** with `subagent_type=<agent-name>` to invoke it (the agent definitions are at `.claude/agents/<name>.md`).
   - Pass the relevant task IDs, affected files, acceptance criteria, **and — in TDD mode — the paths of the scaffold files this agent should consume**.
   - **Token-efficient orientation.** Tell every downstream agent to consult `.bender/cache/scout/<session-id>/index.json` BEFORE issuing its own Read / Grep / Glob calls. Scout ran as the very first step of the graph so the cache is populated; worker agents should replay from disk instead of re-reading files. Cache miss for a lookup they need? Call `bg-scout-explore` to populate it rather than bypassing the cache.
   - **Attribution:** every event emitted during this invocation MUST carry `payload.agent: "<agent-name>"` so the viewer and `bender sessions validate` can thread events by responsible agent. This applies to `skill_invoked`, `skill_completed`, `skill_failed`, `file_changed`, `finding_reported`, `agent_progress`, plus `orchestrator_decision` events whose decision concerns a specific agent (use `payload.dispatched_agent`).
   - Emit `agent_started`, `agent_progress` (as the agent reports back), `agent_completed` / `agent_failed` / `agent_blocked`.

8. **Enforce write scopes**:
   - Each agent's write scope is declared in its frontmatter (`write_scope.allow` / `write_scope.deny`).
   - Before any file write, verify the path matches `allow` and does not match `deny`. If it doesn't, refuse and emit `agent_failed`.

9. **Serialize concurrent same-path writes**:
   - If two agent assignments target the same file path, dispatch them sequentially.

10. **Apply the failure policy**:
    - Default: a failed agent is marked blocked; siblings continue; the final report enumerates the blocker.
    - Strict (`--abort-on-failure`): halt pending agents on first failure.

11. **Write the final report** at `.bender/artifacts/ghu/run-<timestamp>-report.md` with frontmatter and the sections from `contracts/artifacts.md` §5. List the skipped agents/groups (from step 4) in the report header so the reviewer knows what did NOT run.

12. **Emit `session_completed`** with `status: completed | failed`, `duration_ms`, `agents_summary`, and `skipped` (array of canonical names that were bypassed via `--skip`).

13. **Print** the run summary: tasks attempted/completed, files changed, tests added, findings, blockers, skipped agents, report path.

## Observability shape — emit verbatim, do NOT invent fields

Same envelope as `/cry`, `/plan`, `/tdd`. Stage is **`ghu`** for every event.

### session_started
```json
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"user","name":"claude-code"},"type":"session_started","payload":{"command":"/ghu","invoker":"<$USER>","working_dir":"<abs path>","registered_projects":[],"parallelism":1}}
```

### stage_started
```json
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"stage","name":"ghu"},"type":"stage_started","payload":{"stage":"ghu","inputs":[".bender/artifacts/specs/<slug>-<ts>.md",".bender/artifacts/plan/tasks-<ts>.md"]}}
```

### orchestrator_decision (emit per task_decomposition, agent_assignment, graph_node_transition, parallel_dispatch, execution_mode, skip)
```json
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"orchestrator","name":"core"},"type":"orchestrator_decision","payload":{"decision_type":"task_decomposition","tasks":["T001","T002","..."],"dependencies":[{"task":"T002","depends_on":["T001"]}]}}
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"orchestrator","name":"core"},"type":"orchestrator_decision","payload":{"decision_type":"agent_assignment","dispatched_agent":"crafter","task_ids":["T004"]}}
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"orchestrator","name":"core"},"type":"orchestrator_decision","payload":{"decision_type":"graph_node_transition","from_node":"scout","to_node":"architect"}}
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"orchestrator","name":"core"},"type":"orchestrator_decision","payload":{"decision_type":"skip","target":"linter","alias":"lint","reason":"user_requested"}}
```

### orchestrator_progress (emit after every graph node transition)
Whole-session progress as an integer percentage `[0, 100]`. The orchestrator MUST emit one per completed major node so the viewer can render a session-level progress bar. `current_step` is the human-readable node name (e.g., `"scout"`, `"tdd-cycle: bg-tester-scaffold"`, `"review-sweep"`). Baseline points: scout=10, architect=20, crafter/tester cycle mid=50, linter=70, review-sweep=90, final report=100.
```json
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"orchestrator","name":"core"},"type":"orchestrator_progress","payload":{"percent":50,"current_step":"tdd-cycle: bg-crafter-implement","completed_nodes":["scout","architect","bg-tester-scaffold"],"remaining_nodes":["bg-tester-run","linter","review-sweep","report"]}}
```

### agent_started / agent_progress / agent_completed (or agent_failed / agent_blocked)

Every agent MUST emit at least one `agent_progress` event mid-run so the viewer can thread per-agent progress bars alongside the session-level one. `percent` is the agent's own 0..100 estimate of how far through ITS work it is (not the session-level number); `current_step` is a human-readable label. Emit at natural boundaries (e.g., after reading files, after each skill call, after each task). Long skill invocations (>5 seconds wall time) should emit a mid-skill progress too.
```json
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"agent","name":"crafter"},"type":"agent_started","payload":{"agent":"crafter","task_ids":["T004"]}}
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"agent","name":"crafter"},"type":"agent_progress","payload":{"agent":"crafter","percent":42,"current_step":"applying patch","completed":["read","plan"]}}
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"agent","name":"crafter"},"type":"agent_completed","payload":{"agent":"crafter","task_ids":["T004"],"duration_ms":<int>}}
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"agent","name":"crafter"},"type":"agent_failed","payload":{"agent":"crafter","task_ids":["T004"],"duration_ms":<int>,"error":"<human-readable reason>"}}
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"agent","name":"crafter"},"type":"agent_blocked","payload":{"agent":"crafter","task_ids":["T004"],"error":"blocked by finding from sentinel"}}
```

### skill_invoked / skill_completed / skill_failed (inside each agent)
```json
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"agent","name":"crafter"},"type":"skill_invoked","payload":{"skill":"bg-crafter-implement","agent":"crafter"}}
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"agent","name":"crafter"},"type":"skill_completed","payload":{"skill":"bg-crafter-implement","agent":"crafter","duration_ms":<int>,"outputs":["pkg/foo/bar.go"]}}
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"agent","name":"crafter"},"type":"skill_failed","payload":{"skill":"bg-crafter-verify-build","agent":"crafter","duration_ms":<int>,"error":"<stderr summary>"}}
```

### file_changed (one per modified file)

Required: `path`, `lines_added`, `lines_removed`, `agent`.

Optional but **highly recommended** so the viewer can render a Write-tool-style log entry (path + action pill + preview):

- `action`: `"create"` | `"modify"` | `"delete"` — what happened to the file.
- `preview`: first 15 lines of the written content as a single string (include the actual file bytes; strip trailing whitespace). Skip on deletes.
- `preview_line_count`: the integer number of lines in the `preview` string (for `+N more` rendering).
- `lines_total`: the final file's total line count after the write.

```json
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"agent","name":"crafter"},"type":"file_changed","payload":{"path":"pkg/foo/bar.go","action":"create","lines_added":58,"lines_removed":0,"lines_total":58,"preview_line_count":15,"preview":"package foo\n\nimport (\n\t\"fmt\"\n)\n\n...","agent":"crafter"}}
```

### finding_reported (from reviewer/sentinel/benchmarker)
```json
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"agent","name":"reviewer"},"type":"finding_reported","payload":{"finding_id":"R-001","severity":"medium","category":"review","title":"<one line>","description":"<full>","location":{"path":"pkg/foo/bar.go","line_start":12,"line_end":18}}}
```

### artifact_written (final report + per-agent outputs)
```json
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"stage","name":"ghu"},"type":"artifact_written","payload":{"path":".bender/artifacts/ghu/run-<ts>-report.md","stage":"ghu","checksum":"<sha256>","bytes":<int>}}
```

### stage_completed / session_completed
```json
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"stage","name":"ghu"},"type":"stage_completed","payload":{"stage":"ghu","inputs":["<spec path>","<tasks path>"],"outputs":["<run report path>"]}}
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"orchestrator","name":"core"},"type":"session_completed","payload":{"status":"completed","duration_ms":<int>,"agents_summary":[{"agent":"crafter","status":"completed"},{"agent":"tester","status":"completed"}]}}
```

### state.json (overwrite in place)
```json
{
  "schema_version": 1,
  "session_id": "<session_id>",
  "command": "/ghu",
  "started_at": "<iso>",
  "completed_at": "<iso, once terminal>",
  "status": "running|completed|failed",
  "source_artifacts": ["<spec path>","<tasks path>"],
  "skills_invoked": ["<skill names actually invoked>"],
  "files_changed": <int>,
  "findings_count": <int>
}
```

### Forbidden shortcuts
- `ts` / `event` / inlined payload fields / missing `schema_version|session_id|actor|payload` — all WRONG.
- Stage names other than `ghu` — WRONG.
- `kind` on `artifact_written` — WRONG.
- `session_resumed` — WRONG (always fresh sessions).

## Post-Execution

Run any `hooks.after_ghu`.

## Notes

- This is the only stage that mutates the source tree.
- Always emit events; the session log is how `bender sessions show/export` reconstructs the run.
- If a required tool is missing on PATH for a skill (per its `requires_tools`), emit `skill_failed` and continue with the next skill.
