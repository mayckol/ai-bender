---
name: bg-crafter-verify-build
context: bg
description: "Run the project's build to verify the implementation compiles. The gating step before downstream agents (linter, reviewer) run."
provides: [code-generation, build, verify]
stages: [ghu, implement]
applies_to: [any]
---

# bg-crafter-verify-build

Run the project's build command (resolved from `artifacts/constitution.md` Build / CI section). Compilation is a hard gate: if the build fails, the task is blocked and no downstream agent runs against this task's output.

## Steps

1. Resolve the build command from the constitution:
   - Go: `go build ./...`
   - Node: `npm run build` / `pnpm build` / `yarn build`
   - Cargo: `cargo build`
   - Maven: `mvn compile`
   - Gradle: `./gradlew build`
   - Python: `python -m build` or `poetry build`
2. Run it. Capture stdout + stderr.
3. **On success**: emit `skill_completed` with the duration; allow downstream agents to proceed.
4. **On failure**: emit `finding_reported` with severity `high`, the failing command, and the first 20 lines of stderr. Mark this task blocked.

## Notes

- This is the single point at which "the change actually compiles" is verified. Other agents trust its outcome — never bypass it.
- If the build command isn't documented in the constitution and can't be inferred from the stack, emit a `medium`-severity finding asking the user to fill in the constitution's Build / CI section.
