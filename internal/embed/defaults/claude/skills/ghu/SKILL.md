---
name: ghu
user-invocable: true
argument-hint: "[--inline | --bg] [--from=<spec>] [--only=<task-id>] [--skip=<name>[,<name>...]] [--abort-on-failure]"
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

- **`--inline` (default)** — Inline mode. The workflow runs directly in the current conversation so you see each stage stream live in the main chat (scout → architect → crafter → …). The web UI still tails `events.jsonl` in parallel. This is the recommended default, especially on memory-constrained hosts where backgrounded subagents amplify swap pressure and starve the live viewer.
- **`--bg`** — Isolated-subagent mode. The workflow runs inside a forked `Agent` context with `run_in_background: true`. Main chat stays silent until completion — useful when you deliberately want to free the main conversation for other work and only consume results via `http://localhost:4317/sessions/<id>` or notifications. Opt in explicitly.

### Dispatcher (what the MAIN conversation does)

**Step 0 — Before doing anything else, parse `$ARGUMENTS` and branch:**

1. If `$ARGUMENTS` contains `--bg` → **seed the session, then delegate**:
2. Otherwise (default — no `--bg`) → **run inline**: skip the seed-and-delegate block and proceed directly to the "Workflow" section below, executing it in the current conversation. The Workflow section itself owns session seeding in inline mode — do NOT pre-seed here.

### Worktree provisioning (MANDATORY — first action, no silent fall-back)

Before creating `.bender/sessions/<id>/`, writing `state.json`, or dispatching any
stage, this skill MUST provision a git worktree via the bender binary. Every file
the pipeline writes during this session lands inside that worktree; the main
working tree is never touched by the pipeline.

```bash
# 1. Generate <session-id> — same format as before (UTC timestamp + rand3).
SESSION_ID="$(date -u +%Y-%m-%dT%H-%M-%S)-$(head -c 6 /dev/urandom | od -An -tx1 | tr -d ' \n' | head -c 3)"

# 1a. Resolve the workflow id so /tdd → /ghu render as one timeline in the viewer.
#     Uses the current git branch as the grouping key; sessions started on the
#     same branch within 24h inherit the same workflow id.
WORKFLOW_KEY="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || true)"
WORKFLOW_ID=""
WORKFLOW_PARENT=""
if [[ -n "$WORKFLOW_KEY" ]] && command -v bender >/dev/null 2>&1; then
    WF_JSON="$(bender workflow resolve --key "$WORKFLOW_KEY" 2>/dev/null || true)"
    if [[ -n "$WF_JSON" ]]; then
        WORKFLOW_ID="$(printf '%s' "$WF_JSON" | sed -n 's/.*"workflow_id":"\([^"]*\)".*/\1/p')"
        WORKFLOW_PARENT="$(printf '%s' "$WF_JSON" | sed -n 's/.*"parent_session_id":"\([^"]*\)".*/\1/p')"
    fi
fi

# 2. Probe the bender binary.
WT_FLAGS=()
if [[ -n "$WORKFLOW_ID" ]]; then
    WT_FLAGS+=(--workflow-id "$WORKFLOW_ID")
fi
if [[ -n "$WORKFLOW_PARENT" ]]; then
    WT_FLAGS+=(--workflow-parent "$WORKFLOW_PARENT")
fi
if command -v bender >/dev/null 2>&1 && bender worktree --help >/dev/null 2>&1; then
    WT_OUT="$(bender worktree create "${WT_FLAGS[@]}" "$SESSION_ID")"
elif [[ -x .specify/extensions/worktree/scripts/bash/worktree.sh ]]; then
    # Fallback script does not yet carry workflow linkage; this is a soft
    # degradation — the viewer falls back to per-session timelines.
    WT_OUT="$(bash .specify/extensions/worktree/scripts/bash/worktree.sh create "$SESSION_ID")"
else
    printf 'error: bender v0.18.0+ is required for worktree-isolated sessions.\n' >&2
    printf 'Install: go install github.com/mayckol/ai-bender/cmd/bender@latest\n' >&2
    printf '    OR   drop .specify/extensions/worktree/scripts/ into the project.\n' >&2
    exit 1
fi

# 3. Parse the binary's two output lines:
#    worktree: <absolute path>
#    branch:   bender/session/<SESSION_ID>
WORKTREE_PATH="$(printf '%s\n' "$WT_OUT" | awk '/^worktree:/ {print $2}')"
SESSION_BRANCH="$(printf '%s\n' "$WT_OUT" | awk '/^branch:/ {print $2}')"

# 4. Adopt the worktree as the working directory for every subsequent tool call.
cd "$WORKTREE_PATH"

# 5. state.json and events.jsonl have ALREADY been populated by
#    `bender worktree create` — do not rewrite them here. The skill continues
#    with orchestrator_decision (task_decomposition) next, per the Observability
#    shape section below.
```

On refusal (exit code 10/11/12/13 from `bender worktree create`), abort the
session immediately — never fall back to writing in the main tree.

<!-- END WORKTREE PROVISIONING BLOCK — do not modify this comment;
     tests/integration/skills_initialisation_test.go grep for it. -->

Then continue the dispatch:

   - After the worktree is provisioned (steps 1–5 above), append `session_started` and `stage_started` to `.bender/sessions/<SESSION_ID>/events.jsonl` (one `printf` each — same envelope as "Observability shape" below). `state.json` has already been authored by the binary with schema v2; do NOT overwrite it.
   - Invoke the **Agent tool** exactly once with:
     - `subagent_type: general-purpose`
     - `run_in_background: true`
     - `description`: `"ghu background run"`
     - `prompt`: a self-contained message that includes (a) the full body of this SKILL.md **from the `## Actor discipline` section onward** (this is critical — `## Actor discipline`, `## Event emission discipline — STREAM, never batch`, `## Pre-Execution Checks`, `## Workflow`, and `## Observability shape` must ALL be in the subagent prompt; if you slice from `## Workflow` the subagent never sees the streaming rule and falls back to batch-at-end, leaving the viewer blank until completion), (b) the user's `$ARGUMENTS` (with `--bg` stripped), (c) the absolute working directory **set to `$WORKTREE_PATH` from the provisioning block** — the subagent's FIRST action is `cd "$WORKTREE_PATH"` so every tool call resolves inside the session worktree, AND (d) explicit `SESSION_ID=<session-id>` and `WORKTREE_PATH=<path>` lines plus a note that the session directory, v2 `state.json`, `session_started`, and `stage_started` have **already been written by the dispatcher** — the subagent must continue from `orchestrator_decision` (task_decomposition) onward and must NOT re-emit `session_started` / `stage_started` or recreate `state.json`. The subagent MUST obey the STREAM-never-batch rule: each of its own events is one `printf >> events.jsonl` call at the moment it happens, so the viewer sees updates live.
   - After launching, print to the user exactly:
     - The seeded session ID.
     - The target report path (`.bender/artifacts/ghu/run-<timestamp>-report.md`).
     - The viewer URL: `http://localhost:4317/sessions/<session-id>` — start the viewer with `bender server` (detached by default; `bender server stop` to stop).
     - A note that execution is running in the background and they will be notified on completion.
   - **Best-effort auto-open.** If the viewer is running (TCP probe of `127.0.0.1:4317` succeeds — use `nc -z localhost 4317` or equivalent), invoke the platform opener once and ignore failure: `open http://localhost:4317/sessions/<session-id>` on macOS, `xdg-open` on Linux, `powershell Start-Process` on Windows. Because the session directory was seeded above, the page lands on a live timeline (not a 404). If the probe fails, do not attempt to open — just print the URL plus a hint like `Run 'bender server' to launch the viewer.`.
   - **Exit the main turn.** Do NOT execute the Workflow section in the main conversation when delegating.

The main conversation's sole responsibility in `--bg` mode is to seed the session, dispatch, and report the launch. All remaining orchestration, file writes, and agent invocations happen inside the forked subagent.

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

## Parallel dispatch protocol — MUST OBEY

The pipeline walk produces batches of two or more ready nodes whenever
their dependencies leave them free to run concurrently (emergent
parallelism; see `.bender/pipeline.yaml`). Every such batch MUST be
dispatched as a **single concurrent message**, never one-per-turn.
Claude's `Agent` tool runs invocations in parallel only when multiple
tool-use blocks appear in the SAME assistant message. Dispatching one
agent, waiting for it, then dispatching the next silently serialises the
whole batch — which defeats the pipeline's purpose and is the single
biggest correctness-and-speed bug this section exists to prevent.

### Rule for `ordered: false` groups

1. Resolve the eligible agent set (apply `--skip` filters first).
2. Emit **one** `orchestrator_decision` event with
   `decision_type: "parallel_dispatch"`, `group: "<group-id>"`, and
   `dispatched_agents: ["<name>", ...]`.
3. In the **same assistant message**, issue one `Agent` tool call per
   dispatched agent — no prose between them, no intermediate tool calls,
   no awaiting one before the next.
4. Each dispatched agent streams its own events into the shared
   `events.jsonl`. Interleaved lines from different agents are expected
   and correct; never buffer to "sort them" at the end.
5. After every dispatched agent has returned (success, failure, blocked),
   resume walking the graph at the next node.

### Rule for `ordered: true` groups

Walk in declaration order, one agent per turn. `halt_on_failure: true`
means stop the entire run on the first failure. `halt_on_failure: false`
means emit `agent_failed` / `agent_blocked` and continue with the next
entry.

### Forbidden patterns

- One Agent call per assistant message when the group is `ordered:
  false`. This is the silent-serialisation bug.
- Awaiting agent A's result before starting agent B when both belong to
  the same `ordered: false` group.
- Skipping the `parallel_dispatch` `orchestrator_decision` event. The
  viewer uses it to lay out the fan-out in the flow diagram.
- Parallel-dispatching agents whose `write_scope.allow` globs overlap on
  the same file path. If two dispatched agents both claim the right to
  write the same path, fall back to sequential dispatch and emit
  `orchestrator_decision` with `decision_type: "parallel_dispatch_aborted"`,
  `reason: "write_scope_conflict"`, `conflicting_path: "<path>"`. Step 9
  (serialise concurrent same-path writes) is the runtime enforcement of
  this rule.

## Event emission discipline — STREAM, never batch

**Every** event in "Observability shape" MUST be appended to `.bender/sessions/<id>/events.jsonl` **the moment its trigger happens** — **one `bender event emit` call per event**, not a single `Write` at the end. The bender-ui viewer tails the file via fsnotify; batching collapses the timeline into one notification and the user sees `Waiting for events…` for the full run. This applies to both the orchestrator (main or `--bg` subagent) AND every worker subagent it dispatches — each worker emits its own events inside its own context using the same mechanism.

Use the binary helper (feature 007). It performs an atomic `O_APPEND | O_SYNC` write and fsync so fsnotify fires per event — critical when the subagent's cwd is a worktree:

```bash
bender event emit \
  --sessions-root "$SESSIONS_ROOT" \
  --session "$SESSION_ID" \
  --type orchestrator_progress \
  --actor-kind orchestrator \
  --actor-name core \
  --payload '{"percent":42,"current_step":"crafter","completed_nodes":3,"total_nodes":10}'
```

`$SESSIONS_ROOT` is the absolute path to the **main repo's** `.bender/sessions` (set it once at session start with `SESSIONS_ROOT="$REPO_ROOT/.bender/sessions"`). Passing `--sessions-root` explicitly is mandatory from inside a worktree because the skill's cwd is no longer the main repo.

Fallback when the binary is unavailable: call `.specify/scripts/bash/event-emit.sh` with the same flags. Do **not** use raw `printf >> events.jsonl` — batched writes defeat the streaming contract and are what caused the "flow view frozen" regression.

Ordering: intent events (`skill_invoked`, `orchestrator_decision`, `agent_started`) emit BEFORE the action; result events (`file_changed`, `artifact_written`, `skill_completed`, `agent_completed`, `stage_completed`, `session_completed`) emit AFTER. `orchestrator_progress` + `agent_progress` emit as their percent changes — not once at the end.

### Computing `orchestrator_progress.percent`

Replace any fixed baseline table (e.g. scout=10, architect=20) with a pure function of the DAG walk:

```
percent = round(100 * completed_nodes / total_nodes)
```

Emit one `orchestrator_progress` per node transition carrying the numeric `completed_nodes` and `total_nodes` alongside `percent`. This is what makes the scout-stage % (and every other stage %) reflect real progress rather than a hand-picked constant.

Scout-family agents additionally emit one `agent_progress` per internal sub-step (one scan/grep/read batch) so the per-agent bar advances smoothly even when the session-level walk is between transitions.

## Pre-Execution Checks

Run any `hooks.before_ghu`.

## Workflow

1. **Resolve the source artifacts**:
   - The latest **approved** spec at `.bender/artifacts/specs/<slug>-<ts>.md`. Every artifact in the plan set MUST have `status: approved` in its frontmatter; a set at `status: draft` is a plan that the user has not yet confirmed and `/ghu` MUST NOT touch it.
   - The matching task list at `.bender/artifacts/plan/tasks-<ts>.md`.
   - Optionally, test scaffolds at `.bender/artifacts/plan/tests/`.
   - If anything required is missing OR if the latest plan set is still at `status: draft`, print:
     - `error: missing required upstream artifact: <name>. Run /plan and /plan confirm first.`
     - Exit. Do **not** create a partial session.
   - If the latest `state.json` for a `/plan` session has `status: "awaiting_confirm"` and no newer `/plan confirm` session has flipped the set to approved, print the same error and exit. Implementation cannot begin while the plan is still under review.

2. **Load `.bender/pipeline.yaml`** — the declarative execution DAG. Reject
   any pipeline whose `schema_version` you don't recognise. The full schema
   and runtime semantics are defined in `specs/002-pipeline-config-move/contracts/pipeline-schema.md`
   — this SKILL intentionally doesn't duplicate that schema; just load, evaluate variables, and walk.

3. **Honor `--only=<task>`** to scope to a single task (same as `/implement`).

4. **Honor `--skip=<name>[,<name>...]`** to bypass pipeline nodes by agent name.
   Any node whose `agent` matches a skipped canonical name is treated as
   `when: false` (resolved-by-skip) for the rest of the walk. Canonical
   names (aliases in parentheses):

   | Name | Alias(es) | What it skips |
   |---|---|---|
   | `linter` | `lint` | every pipeline node whose `agent` is `linter` |
   | `tester` | `test`, `tests` | every node whose `agent` is `tester` |
   | `reviewer` | `review` | every node whose `agent` is `reviewer` |
   | `sentinel` | `security`, `sec` | every node whose `agent` is `sentinel` |
   | `benchmarker` | `perf`, `bench` | every node whose `agent` is `benchmarker` |
   | `scribe` | `docs` | every node whose `agent` is `scribe` |
   | `surgeon` | `refactor` | every node whose `agent` is `surgeon` |
   | `architect` | — | every node whose `agent` is `architect` |

   Rules:
   - **`crafter` is not skippable.** `/ghu` that skips crafter produces nothing. Reject with `error: cannot skip crafter — /ghu has nothing to do without it.`
   - **`scout` is not skippable.** Its cache is what makes downstream agents token-efficient; skipping it forces every other agent to re-read the tree.
   - **In TDD mode** (`tdd_mode == true`), `tester` skipping is rejected — the TDD branch (`tdd-scaffold → tdd-implement → tdd-verify`) needs the tester. Print `error: --skip=tester is not compatible with TDD mode (approved scaffolds under .bender/artifacts/plan/tests/). Remove the scaffolds or drop --skip.`.
   - **Unknown names** are a hard error listing the valid names above.
   - Every resolved skip entry MUST emit an `orchestrator_decision` event with `decision_type: "skip"` and payload `{"target": "<canonical-name>", "reason": "user_requested", "alias": "<what the user typed>"}` so the viewer and `bender sessions validate` can show what was bypassed.

5. **Honor `--abort-on-failure`** to halt on the first task failure (default: continue and mark blocked).

6. **Session directory + initial events.**
   - **`--bg` mode**: the dispatcher (see "Dispatcher" section above) has already provisioned the worktree via `bender worktree create`, populated a v2 `.bender/sessions/<SESSION_ID>/state.json`, and appended `session_started` + `stage_started`. The subagent's `cd "$WORKTREE_PATH"` is the first shell action; use the `SESSION_ID` and `WORKTREE_PATH` passed in the subagent prompt and do NOT recreate the directory, rewrite `state.json`, or re-emit those two events.
   - **`--inline` mode**: run the Worktree provisioning block from the Dispatcher section inline (same bash, same binary probe, same fallback). After `cd "$WORKTREE_PATH"`, append `session_started` + `stage_started` to `.bender/sessions/<SESSION_ID>/events.jsonl` and continue. `state.json` is already authored by the binary — do not rewrite it.
   - In **both** modes, append `orchestrator_decision` next (with `decision_type: "task_decomposition"`). The decomposition payload MUST carry the task list AND the dependency edges extracted from the tasks file: `{"decision_type":"task_decomposition","tasks":["T001","T002",...],"dependencies":[{"task":"T002","depends_on":["T001"]},...]}`. Tasks with no incoming edges are the first wave; everything else must wait for its prerequisites.

7. **Evaluate the pipeline's declared variables** in the order they appear
   under `variables:` in `.bender/pipeline.yaml`. Kinds:

   - **`glob_nonempty_with_status`**: execute the glob under the project
     root. If it matches no files → variable is `false`. If any matched
     file's YAML frontmatter lacks a `status` key whose value equals the
     declared `require_status`, the variable is `false`. Otherwise `true`.
   - **`plan_flag`**: open the latest approved plan under
     `.bender/artifacts/plan/plan-*.md` (or the plan artifact referenced by
     the pipeline's `source_artifacts`). Read the frontmatter key named by
     `flag`. Coerce to `true`/`false`. A missing key or missing plan file
     resolves to `false` (conservative).
   - **`literal`**: use the declared `value` verbatim.

   Once every variable has a concrete value, emit:

   ```json
   {"type":"orchestrator_decision","payload":{"decision_type":"pipeline_loaded","pipeline_id":"<pipeline.id>","nodes_total":<int>,"nodes_skipped_by_when":<int>,"max_concurrent":<int>,"variables":{"<name>":<value>, ...}}}
   ```

   For backwards-compatible viewers, also emit an `execution_mode` event
   derived from `tdd_mode` (`mode: "tdd"` or `mode: "plain"` plus
   `scaffold_count`).

8. **Walk the pipeline DAG.** Follow this algorithm verbatim — it mirrors
   `internal/pipeline/walker.go::DryRun` byte-for-byte so the viewer's
   flow diagram and the Go `bender doctor` dry-run both stay in sync with
   your walk.

   ```
   status := {}                       # node_id → "pending" | "skipped" | "resolved" | "failed" | "blocked"
   for each node in pipeline.nodes:
       status[node.id] = "skipped" if when(node) is false else "pending"

   while true:
       ready := nodes where status[n.id] == "pending" AND deps_satisfied(n)
       if ready is empty: break

       sort ready by (priority desc, id asc)
       batch := ready[:max_concurrent]

       if |batch| >= 2:
           if any two members of `batch` have overlapping write_scope.allow globs:
               emit orchestrator_decision(parallel_dispatch_aborted,
                    group=<inferred>, reason="write_scope_conflict",
                    conflicting_path=<glob>, fallback_order=[ids in priority order])
               dispatch batch members sequentially (one per subsequent turn)
           else:
               emit orchestrator_decision(parallel_dispatch,
                    group=<inferred>, dispatched_agents=[...], node_ids=[...])
               in the SAME assistant message, issue one Agent tool call per node
       else:
           emit orchestrator_decision(agent_assignment,
                dispatched_agent=batch[0].agent, node_id=batch[0].id)
           issue one Agent tool call

       await every dispatched subagent (interleaved events.jsonl writes are fine)
       for each dispatched node:
           on success → status[id] = "resolved"
           on agent_failed:
               status[id] = "failed"
               if node.halt_on_failure OR pipeline.halt_on_failure: stop walking
           on agent_blocked → status[id] = "blocked"

   emit stage_completed / stage_failed based on whether any required node is failed.
   ```

   `deps_satisfied(n)` rules:
   - `depends_mode: all-resolved` (default): every `n.depends_on` id is
     in `status` ∈ {`resolved`, `skipped`}.
   - `depends_mode: any-resolved`: at least one `n.depends_on` id is
     `resolved` (skipped deps don't count for `any-resolved` — they
     represent branches that were never active).

   Write-scope overlap rule: compare `write_scope.allow` globs pairwise
   using simple shell-glob equality or prefix subset. If both nodes could
   write the same path (e.g. both claim `**/*.go`), they must run
   sequentially. Emit `parallel_dispatch_aborted` per the observability
   section below and fall back to priority-ordered serial dispatch of
   just those two nodes.

   The dispatched Agent tool call for each node MUST:
   - Set `subagent_type` to the node's `agent`.
   - Carry the node id in the prompt so the subagent can attribute its
     events correctly.
   - Tell the subagent to consult
     `.bender/cache/scout/<session-id>/index.json` BEFORE issuing its own
     Read / Grep / Glob calls (token efficiency).
   - Instruct the subagent to emit `agent_started`, `agent_progress` (as
     it reports back), and a terminal event (`agent_completed` /
     `agent_failed` / `agent_blocked`) — all with
     `payload.agent: "<agent-name>"`.

9. **Enforce write scopes** at every file-write attempt:
   - Each agent's write scope is declared in its frontmatter (`write_scope.allow` / `write_scope.deny`).
   - Before any file write, verify the path matches `allow` and does not match `deny`. If it doesn't, refuse and emit `agent_failed`.

10. **Apply the failure policy**:
    - Default: a failed agent is marked blocked; siblings continue; the final report enumerates the blocker.
    - Strict (`--abort-on-failure`): halt the walk on the first failure, identical to a node-level `halt_on_failure: true`.

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

### orchestrator_decision (emit per pipeline_loaded, task_decomposition, agent_assignment, graph_node_transition, parallel_dispatch, parallel_dispatch_aborted, execution_mode, skip)
```json
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"orchestrator","name":"core"},"type":"orchestrator_decision","payload":{"decision_type":"pipeline_loaded","pipeline_id":"ghu-default","nodes_total":15,"nodes_skipped_by_when":0,"max_concurrent":8,"variables":{"tdd_mode":false,"plan_refactor":false}}}
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"orchestrator","name":"core"},"type":"orchestrator_decision","payload":{"decision_type":"task_decomposition","tasks":["T001","T002","..."],"dependencies":[{"task":"T002","depends_on":["T001"]}]}}
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"orchestrator","name":"core"},"type":"orchestrator_decision","payload":{"decision_type":"agent_assignment","dispatched_agent":"crafter","node_id":"plain-impl","task_ids":["T004"]}}
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"orchestrator","name":"core"},"type":"orchestrator_decision","payload":{"decision_type":"parallel_dispatch","group":"review-batch","dispatched_agents":["reviewer","sentinel","benchmarker"],"node_ids":["reviewer","sentinel","benchmarker"]}}
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"orchestrator","name":"core"},"type":"orchestrator_decision","payload":{"decision_type":"parallel_dispatch_aborted","group":"plain-batch","reason":"write_scope_conflict","conflicting_path":"**/*.go","fallback_order":["plain-impl","plain-tests"]}}
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"orchestrator","name":"core"},"type":"orchestrator_decision","payload":{"decision_type":"graph_node_transition","from_node":"scout","to_node":"architect"}}
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"orchestrator","name":"core"},"type":"orchestrator_decision","payload":{"decision_type":"execution_mode","mode":"plain","scaffold_count":0}}
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"orchestrator","name":"core"},"type":"orchestrator_decision","payload":{"decision_type":"skip","target":"linter","alias":"lint","reason":"user_requested"}}
```

### orchestrator_progress (emit after every graph node transition)
Whole-session progress as an integer percentage `[0, 100]`. The orchestrator MUST emit one per completed pipeline node so the viewer can render a session-level progress bar. `current_step` is the node id from `.bender/pipeline.yaml` (e.g., `"scout"`, `"tdd-implement"`, `"reviewer"`). Baseline points: scout=10, architect=20, crafter/tester implementation mid=50, linter=70, review trio=85, scribe=92, report=100.
```json
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"orchestrator","name":"core"},"type":"orchestrator_progress","payload":{"percent":50,"current_step":"tdd-implement","completed_nodes":["scout","architect","tdd-scaffold"],"remaining_nodes":["tdd-verify","linter","reviewer","sentinel","benchmarker","scribe","report"]}}
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

### state.json (authored by `bender worktree create`)

`state.json` is written by the binary (or the fallback script) during the
Worktree provisioning step, at schema v2, with `worktree`, `session_branch`,
`base_branch`, and `base_sha` populated. The orchestrator does NOT rewrite
this file during the session — it only updates the top-level `status`,
`completed_at`, `skills_invoked`, `files_changed`, and `findings_count`
fields at terminal transitions, preserving every v2 worktree field verbatim.

Full schema: `internal/session/state.go`'s `State` struct plus
`specs/004-worktree-flow/contracts/state-v2.schema.yaml`. The v1→v2 field
inventory and loader compatibility are documented in
`specs/004-worktree-flow/data-model.md`.

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
