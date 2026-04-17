# Refactor — declarative pipeline + `.bender/` config move

Status: **proposed**. Target version: **v0.17.0**. Tracking branch: `main`.

This document enumerates every required change to land two linked shifts:

1. **Bender-owned configuration moves out of `.claude/` into `.bender/`.**
   `.claude/` stays reserved for Claude Code native artefacts (agents + skills).
   `.bender/` becomes the single home for everything bender-specific — both
   config (`pipeline.yaml`, `groups.yaml`) and runtime state (`sessions/`,
   `artifacts/`, `cache/`).

2. **`/ghu` and `/implement` stop interpreting a prose graph and start
   walking a declarative DAG from `.bender/pipeline.yaml`.** Parallelism
   becomes emergent (two ready nodes with disjoint deps = single-message
   `Agent` fan-out). `priority` only breaks ties when `ready > max_concurrent`.

---

## 1. Why

### 1.1 Scope cleanup

Today `.claude/groups.yaml` sits alongside `.claude/agents/*.md` and
`.claude/skills/*/SKILL.md`. Only the latter two are Claude Code native
conventions. `groups.yaml` is bender-invented — Claude Code itself doesn't
read it. Pinning it to `.claude/` muddles the contract:

| Directory | Owner | Consumer |
|---|---|---|
| `.claude/agents/` | Claude Code convention | Claude runtime |
| `.claude/skills/` | Claude Code convention | Claude runtime |
| `.claude/groups.yaml` | bender (today) | LLM-side `/ghu` prose only |
| `.bender/sessions/` | bender | bender viewer + CLI |
| `.bender/artifacts/` | bender | `/cry`, `/plan`, `/ghu` outputs |
| `.bender/cache/` | bender | scout cross-agent cache |

Relocating config to `.bender/` gives a clean invariant: **if bender owns
it, it lives under `.bender/`.**

### 1.2 The prose-graph problem

The current `/ghu` SKILL.md describes execution as ASCII art:

```
plain-cycle group (parallel, halt_on_failure=false):
  crafter → bg-crafter-implement   ∥   tester → bg-tester-write-and-run
```

The `∥` is visual, not executable. Claude's `Agent` tool only parallelises
when multiple tool-use blocks land in **one assistant message**. One-per-
turn dispatch silently serialises — which is what we've been seeing in real
runs. v0.16.0 partially addressed this with an explicit "Parallel Dispatch
Protocol" section, but the graph itself still lives as prose the subagent
must interpret.

Declarative `pipeline.yaml` flips that: the graph becomes a structure the
orchestrator **walks** (via a well-defined algorithm), not a story it must
understand. Dependencies, parallelism, priority, conditionals, and per-node
failure policy are all data. A Go validator (`bender doctor`) catches
cycles, missing deps, broken agent/skill references, malformed `when`
expressions — things no amount of prose review can guarantee.

---

## 2. Current state — what's in use, what isn't

### 2.1 `groups.yaml` consumers today

| Consumer | File | What it does |
|---|---|---|
| `bender doctor` | `internal/doctor/doctor.go:103-116` | Loads groups, counts them, reports the number. Does **not** validate selectors against the catalog. |
| `/ghu` + `/implement` SKILL prose | `skills/ghu/SKILL.md`, `skills/implement/SKILL.md` | Prose tells the LLM "walk the `<group>` group from `.claude/groups.yaml`", so Claude reads and enumerates at runtime. |
| Agent prose (descriptive) | `agents/crafter.md:24`, `agents/tester.md:24,32` | Reference `tdd-cycle` group name in personality docs. Not functional. |
| Integration test | `tests/integration/content_present_test.go:91` | Parses the shipped file and verifies canonical entries. |

**Zero Go runtime code dispatches from groups.yaml.** The file's only real
purpose is to be read by the LLM while executing `/ghu`-family SKILLs.
That's why this migration is low-risk — no runtime dispatcher needs to
change, just path strings.

### 2.2 After the refactor

| File | New path | Primary consumer |
|---|---|---|
| `pipeline.yaml` (new) | `<project>/.bender/pipeline.yaml` | Go (`pipeline.LoadFromFS` + `pipeline.Validate` + `pipeline.DryRun`) **and** SKILL prose for runtime walk |
| `groups.yaml` | `<project>/.bender/groups.yaml` (moved) | SKILL prose only; retained as the "named skill bundles" escape hatch for pattern/tag-driven fan-outs |

`groups.yaml` is not deleted. With the pipeline handling the DAG, groups
becomes a pure skill-selector convenience — useful if we ever add a node
type that fans out over a `select: { patterns: ["bg-sentinel-*"] }` match
set without naming every skill explicitly. That's a v2 concern; for v1 the
pipeline enumerates nodes explicitly.

---

## 3. Target file layout

After `bender sync-defaults --force` against a fresh project:

```
<project>/
  .claude/
    agents/*.md
    skills/*/SKILL.md
  .bender/
    pipeline.yaml           ← NEW: declarative DAG
    groups.yaml             ← MOVED from .claude/groups.yaml
    sessions/               ← runtime (unchanged)
    artifacts/              ← runtime (unchanged)
    cache/                  ← runtime (unchanged)
```

`.bender/` is always writeable by bender and by the LLM during stage runs.
Config files (`pipeline.yaml`, `groups.yaml`) sit at the top level;
runtime state lives in subdirectories. Flat is better than nested while
there are only two config files.

---

## 4. File-by-file change list

### 4.1 Embed restructure

Move the shipped templates out of `internal/embed/defaults/claude/` into a
new sibling `internal/embed/defaults/bender/`:

| From | To |
|---|---|
| `internal/embed/defaults/claude/groups.yaml` | `internal/embed/defaults/bender/groups.yaml` |
| `internal/embed/defaults/claude/pipeline.yaml` *(currently lives here after v0.16.x WIP)* | `internal/embed/defaults/bender/pipeline.yaml` |

The `internal/embed/defaults/claude/` tree keeps `agents/` and `skills/`
— unchanged. Only the two bender-owned YAML files relocate.

### 4.2 `internal/embed/defaults.go`

The embed FS root is `defaults/`, exposed via `FS()`. Both subtrees
(`claude/*` and the new `bender/*`) already ride under that root — no
changes to the embed plumbing itself. The comment that says "materialises
the `defaults/claude/` tree" gets broadened to cover `bender/` too.

### 4.3 `internal/workspace/scaffold.go`

`Scaffold` currently walks `defaults/claude/` and writes to `<root>/.claude/`.
Extend it to also walk `defaults/bender/` and write to `<root>/.bender/`.
Two options:

**A.** Two passes, one per subtree. Simplest — keep destination logic
in `destinationFor` and dispatch per top-level prefix.

**B.** One walk over `defaults/` with per-prefix destination routing.
Smaller diff; cleaner.

Preferred: **B**. The helper becomes:

```go
func destinationFor(projectRoot, embeddedPath string) string {
    switch {
    case strings.HasPrefix(embeddedPath, "claude/"):
        rel := strings.TrimPrefix(embeddedPath, "claude/")
        return filepath.Join(projectRoot, ".claude", filepath.FromSlash(rel))
    case strings.HasPrefix(embeddedPath, "bender/"):
        rel := strings.TrimPrefix(embeddedPath, "bender/")
        return filepath.Join(projectRoot, ".bender", filepath.FromSlash(rel))
    }
    return ""  // ignored — unknown prefix
}
```

Preservation semantics (untouched user-edited files) stay identical —
just applied per tree.

### 4.4 `internal/group/loader.go`

Change the default read path in `LoadFromFS`'s doc/comment from
`claude/groups.yaml` to `bender/groups.yaml`. The function already takes
`base string`, so the signature doesn't change — only doctor's call site.

### 4.5 `internal/pipeline/loader.go`

Already parametric on `base`. No signature change. Doctor's call site
switches from `"claude/pipeline.yaml"` to `"bender/pipeline.yaml"`.

### 4.6 `internal/doctor/doctor.go`

Two call sites to rebind:

- **Line 105** — `group.LoadFromFS(embedded.FS(), "claude/groups.yaml")`
  → `group.LoadFromFS(embedded.FS(), "bender/groups.yaml")`
- **Line 111** — `group.LoadFromFS(userFS, "groups.yaml")` — the
  `userFS` here is rooted at `<project>/.claude`. Change it to be
  rooted at `<project>/.bender` for config lookups, or keep userFS at
  the project root and pass `".bender/groups.yaml"`. Pick whichever
  preserves the existing mapping cleanly.

Similarly the new pipeline check (added in v0.16.x) needs its user-side
path updated to `.bender/pipeline.yaml`.

Practical approach: introduce a **second** user FS rooted at
`<project>/.bender/` alongside the existing `<project>/.claude/` FS.
Callers pass the right one based on what they're loading. Keeps each
callsite explicit and untangles the two trees in the resolver.

### 4.7 `bender init` / `bender sync-defaults`

Inherit the new layout automatically once `workspace.Scaffold` is
updated. The CLI surfaces don't need touching.

### 4.8 `/ghu` SKILL.md — **major rewrite**

File: `internal/embed/defaults/claude/skills/ghu/SKILL.md`. Currently
~385 lines with prose graphs, group-walking instructions, and the
Parallel Dispatch Protocol section introduced in v0.16.0.

Changes:

1. **Replace Workflow §7 ("Walk the execution graph")** — instead of the
   plain-mode / TDD-mode ASCII graphs, instruct the subagent to:

   - Read `.bender/pipeline.yaml`.
   - Resolve every declared `variables` entry into a concrete value:
     - `kind: glob_nonempty_with_status` — run the glob, verify every
       matched file has the required frontmatter `status`, set the
       variable to `true`/`false` accordingly.
     - `kind: plan_flag` — open the latest approved plan artifact,
       read the named frontmatter key, coerce to bool.
     - `kind: literal` — use the declared value.
   - Walk the pipeline via the algorithm below (also included verbatim
     in the SKILL body, since this is prompt text the subagent reads).

2. **Add a "Pipeline walk algorithm" section** with pseudocode identical
   to `internal/pipeline/walker.go::DryRun`, so the LLM's walk matches
   the Go dry-run preview byte-for-byte. Includes:
   - How to compute `ready` (all deps in `resolved`, `when` true).
   - How to order by priority then id for stable fan-out.
   - How to cap batches at `max_concurrent`.
   - How to emit `orchestrator_decision(parallel_dispatch)` before
     dispatching ≥2 nodes.

3. **Keep the Parallel Dispatch Protocol section** — still the
   mechanical rule the subagent obeys when `|batch| > 1`.

4. **Drop prose group references** — remove `plain-cycle`,
   `tdd-cycle`, `review-sweep`, `docs-sweep` mentions in the Workflow
   section. They become pipeline node groups identifiable by the shared
   `depends_on` pattern, not named constructs.

5. **`--skip=<name>` table** — reinterpret skipped targets as
   pipeline-level node filters. Same canonical names
   (`crafter`, `tester`, `scribe`, `linter`, …). The orchestrator drops
   any node whose `agent` matches a skipped name, then re-validates
   dependencies before walking (a skipped node is "resolved", not
   blocking — matches existing `when`-skip semantics).

6. **Observability shape** — add `orchestrator_decision.decision_type`
   values used by the walker: `pipeline_loaded` (with `nodes_total`,
   `nodes_skipped_by_when`, `max_concurrent`), existing
   `parallel_dispatch`, existing `parallel_dispatch_aborted`,
   existing `skip`, existing `agent_assignment`.

### 4.9 `/implement` SKILL.md — **minor rewrite**

File: `internal/embed/defaults/claude/skills/implement/SKILL.md`. Same
walk as `/ghu` with task scoping:

- After loading `.bender/pipeline.yaml`, apply a node filter matching
  the resolved task's `agent_hints`. Nodes whose `agent` doesn't match
  are dropped (treated as `when: false`).
- Walk the residual DAG identically.

Line 45 currently says "Run the same execution graph as `/ghu`, pruned
to the single task and its direct review/lint follow-ups". Rewrite to:
"Load `.bender/pipeline.yaml`; narrow candidate nodes to the ones
implied by the resolved task; walk per the algorithm in
`.claude/skills/ghu/SKILL.md`."

### 4.10 Agent prose touch-ups

- **`agents/crafter.md:24`** — replace "the `tdd-cycle` group is active"
  with "TDD-mode pipeline nodes (`tdd-scaffold → tdd-implement →
  tdd-verify`) are active". Purely descriptive; agent behavior
  unchanged.

- **`agents/tester.md:24`** — replace "runs as the first step of the
  `tdd-cycle` group during `/ghu` / `/implement`" with "runs as the
  `tdd-scaffold` node in the pipeline (depends_on: architect, surgeon)".

- **`agents/tester.md:32`** — replace "runs as the third step of
  `tdd-cycle` (after crafter)" with "runs as the `tdd-verify` node
  (depends_on: tdd-implement)".

Worker skills (`bg-scout-*`, `bg-crafter-*`, `bg-tester-*`,
`bg-linter-*`, `bg-reviewer-*`, `bg-sentinel-*`, `bg-benchmarker-*`,
`bg-scribe-*`, `bg-architect-*`, `bg-surgeon-*`) **don't need
updating** — they execute work and emit events, with no awareness of
orchestration. Pipeline-layer concerns stay in the orchestrator
SKILLs.

### 4.11 Integration tests

- `tests/integration/init_test.go:115` — `mustExist` path flips from
  `.claude/groups.yaml` to `.bender/groups.yaml`. Add a check that
  `.bender/pipeline.yaml` also exists.

- `tests/integration/init_test.go:168`,
  `tests/integration/sync_defaults_test.go:17,37` — update the
  `custom` path string to the new location.

- `tests/integration/content_present_test.go:91` — the embed read
  path flips from `"claude/groups.yaml"` to `"bender/groups.yaml"`.

### 4.12 README

- **Line 64** — `.claude/{skills,agents}, .claude/groups.yaml,
  .bender/config.yaml, .bender/artifacts/constitution.md` → flip to
  `.claude/{skills,agents}, .bender/groups.yaml, .bender/pipeline.yaml,
  .bender/artifacts/constitution.md`.
- **Line 405** — "Default groups (named selectors in
  `.claude/groups.yaml`)" → "Default groups (named selectors in
  `.bender/groups.yaml`)".
- **Line 438** — filesystem diagram relocation.
- **Line 503** — edit instructions for overriding groups.

### 4.13 `internal/embed/defaults.go`

Module-level comment at line 4: "project's `.claude/` (skills, agents,
groups.yaml)" → "project's `.claude/` (skills, agents) and `.bender/`
(groups.yaml, pipeline.yaml)".

---

## 5. Migration for existing client projects

Two projects are already registered and synced: `ui-test`, `api-test`.
They carry the **old** layout (`.claude/groups.yaml`, no
`pipeline.yaml`).

Migration flow:

1. `cd <project>` then `bender sync-defaults --force`. This:
   - Writes the new `.bender/groups.yaml` template.
   - Writes the new `.bender/pipeline.yaml` template.
   - Leaves the **old** `.claude/groups.yaml` in place (sync-defaults
     only scaffolds *forward*; it never removes files).
2. Optional: delete the stale `.claude/groups.yaml` manually. The
   loader no longer reads it; leaving it behind is cosmetic lint,
   not a runtime hazard.
3. Run `bender doctor` to confirm `status: healthy`.

Document this in the README's upgrade notes for v0.17.0.

The probe harness (`parallel-probe`, `probe-alpha/β/γ`) that was
installed into both projects during v0.16.0 testing stays valid —
those agents + skills live under `.claude/` and don't care about the
config relocation.

---

## 6. What's deferred to v2

- **Pattern/tag-driven pipeline nodes.** Adding an optional `group:` or
  `select:` shorthand on a node, so one pipeline entry can fan out over
  every skill matching `bg-sentinel-*` without naming each. Needs the
  orchestrator walk to materialise synthetic child nodes at
  dispatch-time. Not required today — every current fan-out names its
  nodes explicitly.
- **Per-task crafter fan-out.** Multiple concurrent `crafter(T001)` +
  `crafter(T002)` when `depends_on` sets are disjoint. Needs runtime
  same-path write-scope collision detection (Workflow §9 declares the
  rule but today has no enforcement). The `parallel_dispatch_aborted`
  event already reserves the slot.
- **UI flow diagram from live pipeline.yaml.** Today the viewer has a
  hard-coded pipeline in `ui/src/client/lib/pipeline.ts`. Replace with
  an API endpoint that serves the parsed pipeline; the `PipelineFlow`
  component renders whatever the server returns.
- **Deleting `.claude/groups.yaml` from existing client projects**.
  Harmless to leave; a v2 cleanup command (`bender migrate
  --remove-legacy-config`) could sweep it.

---

## 7. Release plan

| Step | Output |
|---|---|
| 1 | Land §4.1 – §4.6 (embed move + loader rebinds). Run Go tests. |
| 2 | Land §4.8 – §4.10 (SKILL + agent prose rewrites). |
| 3 | Land §4.11 – §4.13 (tests + README). Run `make test`. |
| 4 | Build + release `v0.17.0`. |
| 5 | Reinstall locally (`go install ...`). |
| 6 | Re-sync `ui-test` and `api-test`: `bender sync-defaults --force`. |
| 7 | `bender doctor --project ui-test` / `--project api-test` — both must report `status: healthy`. |
| 8 | Re-run `/parallel-probe` in one of the projects to confirm the new SKILL still obeys the Parallel Dispatch Protocol. Verdict should remain `PARALLEL`. |

---

## 8. Non-goals

- Rewriting worker skills (`bg-*`) — they stay stack- and
  orchestration-agnostic.
- Changing the event schema — existing events are sufficient; only the
  set of `orchestrator_decision.decision_type` values expands.
- Adding new `when` operators — equality (`==` / `!=`) covers every
  current branching need.
- Cross-pipeline support (multiple pipelines per project, keyed by
  slash command) — not needed while `/ghu` and `/implement` share one
  graph. Leave for v2 if a real case arises.
