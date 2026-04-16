#!/usr/bin/env bash
# gen-default-skills.sh — one-time generator that writes every default worker skill markdown
# under internal/embed/defaults/claude/skills/<name>/SKILL.md.
#
# Skills authored by hand (slash commands and the bender-doctor wrapper) are NOT touched.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SKILLS_DIR="$ROOT/internal/embed/defaults/claude/skills"

# write_skill <name> <context> <stages> <applies_to> <provides> <description> <body>
write_skill() {
    local name="$1" ctx="$2" stages="$3" applies="$4" provides="$5" desc="$6" body="$7"
    local dir="$SKILLS_DIR/$name"
    mkdir -p "$dir"
    cat > "$dir/SKILL.md" <<EOF
---
name: $name
context: $ctx
description: $desc
provides: [$provides]
stages: [$stages]
applies_to: [$applies]
---

# $name

$body
EOF
}

############################
# fg-bootstrap-detect-* (10) — refine constitution sections marked pending after \`bender init\`.
############################
write_skill fg-bootstrap-detect-stack         fg bootstrap any "detect, stack"        "Detect the project's primary language, package manager, and frameworks." \
    "Use file probes (manifests, lock files) and extension counts to identify the stack. Update \`artifacts/constitution.md\` Stack section."
write_skill fg-bootstrap-detect-structure     fg bootstrap any "detect, structure"    "Detect the project's folder layout, entry points, and module boundaries." \
    "Walk top-level directories. Identify entry points (main.go, index.js, app.py, etc.). Update Structure section."
write_skill fg-bootstrap-detect-tester        fg bootstrap any "detect, tests"        "Detect the test framework, conventions, and coverage tooling." \
    "Inspect dev dependencies (package.json devDependencies, pyproject testing, go.mod tests). Update Tests section."
write_skill fg-bootstrap-detect-linter        fg bootstrap any "detect, lint"         "Detect linters, formatters, and pre-commit hooks." \
    "Look for .golangci.yml, .eslintrc.*, .prettierrc.*, .pre-commit-config.yaml. Update Lint section."
write_skill fg-bootstrap-detect-build         fg bootstrap any "detect, build"        "Detect build tooling and containerization." \
    "Look for Makefile, Dockerfile, build scripts, language-specific build tools. Update Build / CI section."
write_skill fg-bootstrap-detect-purpose       fg bootstrap any "detect, purpose"      "Extract the project's purpose from README, manifests, and top-level docs." \
    "Read README.md, package.json description, pyproject.toml description, etc. Summarise the purpose in one paragraph. Update Purpose section."
write_skill fg-bootstrap-detect-conventions   fg bootstrap any "detect, conventions"  "Identify naming patterns, error-handling style, DI approach, and architectural pattern." \
    "Sample 10-30 representative source files. Identify recurring patterns: naming (snake/camel/kebab), error handling (panic/wrap/Result), DI (constructor injection/global/IoC container), architecture (clean/hex/MVC/layered)."
write_skill fg-bootstrap-detect-glossary      fg bootstrap any "detect, glossary"     "Identify recurring domain terms and the project's ubiquitous language." \
    "Scan public APIs, type names, and README headings for recurring nouns. Build a glossary mapping term → one-line definition."
write_skill fg-bootstrap-detect-cicd          fg bootstrap any "detect, cicd"         "Detect the CI provider and deployment targets." \
    "Look for .github/workflows/, .gitlab-ci.yml, .circleci/, Jenkinsfile, vercel.json, fly.toml, Procfile, k8s/Helm manifests. Update Build / CI section."
write_skill fg-bootstrap-detect-dependencies  fg bootstrap any "detect, dependencies" "List direct dependencies with versions and outdated flags." \
    "Parse the manifest, list direct deps with versions. (Outdated detection is best-effort; offline runs may skip.)"

############################
# fg-cry-* (5)
############################
write_skill fg-cry-classify-issue   fg cry any            "classify"     "Classify a free-form request into bug | feature | performance | architectural." \
    "Pick the single best label based on the request. Default to \`feature\` when ambiguous."
write_skill fg-cry-bug              fg cry bug            "capture, bug" "Frame the request as a bug: reproduction steps, expected vs actual." \
    "Section the capture artifact: Reproduction Steps, Expected Behaviour, Actual Behaviour, Suspected Root Cause (if obvious)."
write_skill fg-cry-feature          fg cry feature        "capture, feature" "Frame the request as a feature: outcomes, user types, success criteria." \
    "Section the capture artifact: Desired Outcomes, Affected User Types, Acceptance Criteria, Out of Scope."
write_skill fg-cry-performance      fg cry performance    "capture, performance" "Frame the request as a performance issue: workload, target, constraint." \
    "Section the capture artifact: Workload Description, Current Behaviour, Target Metric, Constraints."
write_skill fg-cry-architectural    fg cry architectural  "capture, architecture" "Frame the request as an architectural change: motivation, scope, blast radius." \
    "Section the capture artifact: Motivation, Affected Modules, Migration Concerns, Backward-Compatibility Impact."

############################
# fg-plan-* (7)
############################
write_skill fg-plan-decompose-tasks   fg plan any "plan, decompose" "Decompose the spec into a numbered, dependency-ordered task list." \
    "Produce \`artifacts/plan/tasks-<ts>.md\` with T001+ tasks, agent_hints, depends_on (no cycles), affected_files, acceptance criteria."
write_skill fg-plan-data-model        fg plan any "plan, data-model" "Author the data-model artifact: entities, fields, relationships, validation, state transitions." \
    "Produce \`artifacts/plan/data-model-<ts>.md\` mirroring the entities introduced in the spec."
write_skill fg-plan-api-contract      fg plan any "plan, api"        "Author the API contract when an externally consumable interface is involved." \
    "Produce \`artifacts/plan/api-contract-<ts>.md\` documenting endpoints / CLI grammar / GraphQL types as appropriate to the project."
write_skill fg-plan-sequence-diagram  fg plan architectural "plan, sequence" "Render a sequence diagram for cross-module flows in the plan." \
    "Use Mermaid \`sequenceDiagram\` syntax inside the data-model or api-contract artifact."
write_skill fg-plan-risk-assessment   fg plan any "plan, risk"       "Author the risk assessment: severity × likelihood × mitigation per risk." \
    "Produce \`artifacts/plan/risk-assessment-<ts>.md\` with Identified Risks (table) and Open Risks (bullet list)."
write_skill fg-plan-affected-files    fg plan any "plan, files"      "List the files each task is expected to touch (informs scout caching and write-conflict prevention)." \
    "Per task, list affected_files as glob patterns. Include both existing files to modify and new files to create."
write_skill fg-plan-migration-strategy fg plan architectural "plan, migration" "Author a migration strategy when the change is breaking." \
    "Section: Pre-conditions, Migration Steps (ordered), Rollback Plan, Communication Plan."

############################
# fg-tester-* (7) — TDD scaffold mode
############################
write_skill fg-tester-scaffold-structure fg tdd any "tdd, scaffold"     "Mirror the source tree under artifacts/plan/tests/ with one scaffold per source file needing coverage." \
    "Inspect the plan's affected_files. For each, create artifacts/plan/tests/<source-path>-<ts>.md."
write_skill fg-tester-propose-cases      fg tdd any "tdd, propose"      "Propose test cases beyond the user's seed scenarios, drawing from the plan and risk assessment." \
    "For each acceptance criterion in the plan, propose at least one positive and one negative case."
write_skill fg-tester-pattern-unit       fg tdd any "tdd, unit"         "Apply the unit-test pattern: isolation, fast, no I/O." \
    "Document each unit case as: Preconditions / Steps / Expected outcome. Prose only."
write_skill fg-tester-pattern-integration fg tdd any "tdd, integration" "Apply the integration-test pattern: real adapters where they cross a boundary." \
    "Document each integration case as: Setup (real adapters), Inputs, Expected behaviour. Prose only."
write_skill fg-tester-pattern-contract   fg tdd any "tdd, contract"     "Apply the contract-test pattern: producer/consumer compatibility checks." \
    "Document each contract case as: Producer / Consumer / Schema / Compatibility expectation."
write_skill fg-tester-pattern-e2e        fg tdd any "tdd, e2e"          "Apply the end-to-end pattern: full pipeline against a fixture environment." \
    "Document each e2e case as: Workload / Environment / Assertions on user-observable outcomes."
write_skill fg-tester-coverage-targets   fg tdd any "tdd, coverage"     "Compute the coverage target for the change set." \
    "Read the project's coverage tool (from constitution). Target ≥80% line coverage for new code; flag gaps."

############################
# bg-crafter-* (7)
############################
write_skill bg-crafter-apply-patch      bg ghu any "code-generation" "Apply a minimum-diff patch implementing the task." \
    "Read the affected files, write the smallest change that satisfies the acceptance criteria. No refactors outside scope."
write_skill bg-crafter-check-data-model bg ghu any "check, data-model" "Verify that the implementation honors the planned data model." \
    "Compare the implementation's types/structs/schemas to artifacts/plan/data-model-<ts>.md. Emit findings on divergence."
write_skill bg-crafter-check-api-contract bg ghu any "check, api"      "Verify that the implementation honors the planned API contract." \
    "Compare exposed endpoints / CLI surface / types to artifacts/plan/api-contract-<ts>.md. Emit findings on divergence."
write_skill bg-crafter-check-interface-impl bg ghu any "check, interface" "Verify that interfaces are implemented by the right concrete types." \
    "For each declared interface, find implementing types and check method signatures match. Emit findings on partial implementations."
write_skill bg-crafter-run-build         bg ghu any "code-generation" "Run the project's build to verify the change compiles." \
    "Use the build tool from the constitution (e.g., go build, npm run build, cargo build). Emit a finding on failure."
write_skill bg-crafter-run-generate      bg ghu any "code-generation" "Run code generation (go generate, codegen scripts) when the change requires it." \
    "Detect generators from the constitution / Makefile. Run them; verify generated outputs are committed."
write_skill bg-crafter-resolve-conflicts bg ghu any "code-generation, conflicts" "Resolve merge or write-scope conflicts when multiple tasks touch overlapping files." \
    "Sequence writes to overlapping paths; emit findings if a true logical conflict (not just textual) is detected."

############################
# bg-tester-* (6)
############################
write_skill bg-tester-write-unit         bg ghu any "test, unit"        "Author unit tests for new code." \
    "One test file per production file under test. Cover acceptance criteria + edge cases enumerated by tester agent."
write_skill bg-tester-write-integration  bg ghu any "test, integration" "Author integration tests for boundary-crossing code." \
    "Use real adapters where the boundary lives (DB, HTTP, filesystem). Avoid mocks at the boundary itself."
write_skill bg-tester-write-fixture      bg ghu any "test, fixture"     "Author or update test fixtures." \
    "Fixtures live under tests/fixtures/ or testdata/. Keep them small and human-readable."
write_skill bg-tester-run-suite          bg ghu any "test, run"         "Run the test suite and report failures." \
    "Use the project's test command (from constitution). Emit findings on failure with the failing test name + one-line diagnosis."
write_skill bg-tester-run-coverage       bg ghu any "test, coverage"    "Run the project's coverage tool and report numbers." \
    "Use the coverage tool from the constitution. Report total + per-package coverage."
write_skill bg-tester-diff-coverage      bg ghu any "test, coverage"    "Compute coverage on the diff (changed lines only)." \
    "Compare changed lines against the coverage report; flag any uncovered new code."

############################
# bg-linter-* (5)
############################
write_skill bg-linter-detect-config      bg ghu any "lint, config"     "Detect the project's lint configuration." \
    "Resolve linter from the constitution; verify config file exists; report config path."
write_skill bg-linter-run                bg ghu any "lint, run"        "Run the configured linter against changed files (or the whole tree on demand)." \
    "Default scope: changed files. On --all: whole tree. Capture stdout/stderr."
write_skill bg-linter-autofix            bg ghu any "lint, autofix"    "Apply safe autofixes only (formatting, import sorting)." \
    "Use the linter's --fix or formatter command. Refuse to apply semantic changes."
write_skill bg-linter-report-unfixable   bg ghu any "lint, report"     "Emit findings for warnings/errors the linter cannot autofix." \
    "Group by severity; cite file path + line range; do not modify code."
write_skill bg-linter-format             bg ghu any "lint, format"     "Run the project's formatter (gofmt, prettier, ruff format, rustfmt, etc.)." \
    "Run with no special flags; commit only formatting changes."

############################
# bg-reviewer-* (5)
############################
write_skill bg-reviewer-vs-spec          bg ghu any "review, spec"          "Verify changes match the approved spec." \
    "Read each spec section; for each, check the implementation honors it. Emit findings citing spec section + code location."
write_skill bg-reviewer-vs-constitution  bg ghu any "review, constitution"  "Verify changes respect the project constitution." \
    "Read artifacts/constitution.md; check naming/error/DI/architecture conventions are honored. Emit findings with constitution rule cited."
write_skill bg-reviewer-idioms           bg ghu any "review, idioms"        "Check changes against language/framework idioms." \
    "Use the constitution's stack to know which idioms apply (e.g., Go: use error wrapping; React: use hooks). Emit findings on idiomatic violations."
write_skill bg-reviewer-test-quality     bg ghu any "review, tests"         "Critique the new tests authored this run." \
    "Check: coverage of acceptance criteria, edge cases, isolation, naming. Emit findings on gaps."
write_skill bg-reviewer-pr-summary       bg ghu any "review, pr-summary"    "Generate a PR description for the changes." \
    "Sections: Summary, Why, Changes, Tests, Findings (severity ≥ medium). Write to artifacts/ghu/reviews/<ts>/pr-summary.md."

############################
# fg/bg-architect-* (5)
############################
write_skill fg-architect-boundary-analysis fg plan architectural "architect, boundaries" "Identify module boundaries and the cross-boundary contracts." \
    "List modules + their public APIs. Map cross-module calls. Flag implicit coupling."
write_skill fg-architect-dep-graph         fg plan architectural "architect, deps"        "Build a dependency graph of modules; flag cycles." \
    "Walk imports across the codebase. Render a Mermaid graph in the data-model artifact. Reject cycles."
write_skill fg-architect-pattern-fit       fg plan architectural "architect, patterns"    "Pick an architectural pattern (clean, hex, MVC, layered) appropriate to the change." \
    "Justify the choice in the spec. Document the pattern's roles and the layers' responsibilities."
write_skill bg-architect-validate-boundaries bg ghu architectural "architect, validate"   "Validate that the implementation respects the planned boundaries." \
    "Re-walk imports during /ghu; flag any new edges that cross boundaries the plan said to keep separate."
write_skill bg-architect-detect-drift      bg ghu architectural "architect, drift"        "Detect drift between the implementation and the planned data model / API contract." \
    "Compare resolved types/endpoints to artifacts/plan/. Emit findings on undocumented additions."

############################
# bg-scribe-* (5)
############################
write_skill bg-scribe-update-readme        bg ghu any "scribe, readme"      "Update README to reflect the change set." \
    "Update Installation, Usage, and Examples sections as needed. Keep the existing tone."
write_skill bg-scribe-update-api-docs      bg ghu any "scribe, api-docs"    "Update API docs (godoc, JSDoc, OpenAPI, etc.) for changed public surface." \
    "For each changed public symbol, update its doc comment / spec entry."
write_skill bg-scribe-update-changelog     bg ghu any "scribe, changelog"   "Append a changelog entry for the change set." \
    "Use the project's existing changelog format. One entry per merged change set."
write_skill bg-scribe-inline-comments      bg ghu any "scribe, comments"    "Add inline comments explaining non-obvious why." \
    "Per CLAUDE.md: no comments that restate what the code does. Only add comments that explain non-obvious why, edge cases, or business rules."
write_skill bg-scribe-update-glossary      bg ghu any "scribe, glossary"    "Add new domain terms introduced by this change to the constitution glossary." \
    "If new types/concepts entered the codebase, add them to the constitution's Glossary section."

############################
# bg-scout-* (6)
############################
write_skill bg-scout-find-symbol         bg ghu any "scout, find-symbol"   "Find the definition of a symbol." \
    "Search by exact name in the index. Cache the result under artifacts/.bender/cache/symbol/<name>."
write_skill bg-scout-find-references     bg ghu any "scout, find-refs"     "Find all references to a symbol." \
    "Search for usages; cache the result for the session."
write_skill bg-scout-read-file           bg ghu any "scout, read"          "Read a file (or a line range) and cache the result." \
    "Use Read; honor offset/limit if requested. Cache by file path + mtime."
write_skill bg-scout-list-tree           bg ghu any "scout, tree"          "List a directory tree with optional depth limit." \
    "Use Glob with ** patterns. Cache the listing."
write_skill bg-scout-grep                bg ghu any "scout, grep"          "Run a regex search across the project." \
    "Use Grep; cache the result by pattern + path scope."
write_skill bg-scout-summarize-module    bg ghu any "scout, summarize"     "Summarise a module: file count, public API surface, inbound dependencies." \
    "Cache the summary; subsequent lookups in the same session are served from cache."

############################
# bg-sentinel-* (6)
############################
write_skill bg-sentinel-scan-secrets      bg ghu any "security, secrets"   "Scan for committed secrets." \
    "Look for API key patterns, private keys, hardcoded passwords. Severity ≥ high if exploitable; medium if test-only."
write_skill bg-sentinel-scan-deps-cve     bg ghu any "security, cve"       "Cross-reference dependencies against known CVEs." \
    "Best-effort offline check; if a CVE database is bundled, use it. Emit findings with CVE id + affected version."
write_skill bg-sentinel-audit-inputs      bg ghu any "security, validation" "Audit input validation paths for SQL injection, command injection, SSRF, etc." \
    "Trace user input from entry points to sinks. Flag any unsanitized path."
write_skill bg-sentinel-check-auth-paths  bg ghu any "security, auth"      "Verify authentication and authorization checks on protected endpoints." \
    "Identify protected endpoints from the spec; verify each has auth/authz before reaching the business logic."
write_skill bg-sentinel-check-crypto-usage bg ghu any "security, crypto"   "Verify crypto usage: algorithms, key sizes, salts, IVs." \
    "Flag deprecated algorithms (MD5/SHA1 for hashing, DES, ECB). Flag reused IVs and missing salts."
write_skill bg-sentinel-report-findings   bg ghu any "security, report"    "Aggregate sentinel findings into artifacts/ghu/security/<ts>/." \
    "One file per finding (id-based). Final summary in the run report."

############################
# bg-benchmarker-* (5)
############################
write_skill bg-benchmarker-find-hotpaths     bg ghu performance "perf, hotpaths"  "Identify likely hot paths in the changed code." \
    "Inspect loops, allocations, sync primitives. Mark each as info-severity \"needs measurement\" if not directly measurable."
write_skill bg-benchmarker-detect-n-plus-one bg ghu performance "perf, n+1"       "Detect N+1 query patterns in DB code." \
    "Trace queries inside loops; emit findings citing the loop + query."
write_skill bg-benchmarker-allocation-audit  bg ghu performance "perf, alloc"     "Audit allocation patterns for unnecessary heap pressure." \
    "Flag allocations inside hot loops; suggest pooling or reuse where applicable."
write_skill bg-benchmarker-run-benchmarks    bg ghu performance "perf, run"       "Run the project's benchmarks if configured." \
    "Use the constitution's benchmark command. Capture results; compare against baseline if available."
write_skill bg-benchmarker-compare-baseline  bg ghu performance "perf, baseline"  "Compare current benchmarks against the baseline." \
    "Regression vs baseline → severity high; absolute slow → severity medium with measurement."

############################
# bg-surgeon-* (5)
############################
write_skill bg-surgeon-extract-function       bg ghu architectural "refactor, extract"      "Extract a chunk of code into a new function." \
    "Verify behaviour preserved; tests must pass before and after."
write_skill bg-surgeon-rename-symbol          bg ghu any           "refactor, rename"       "Rename a symbol across the codebase." \
    "Update every reference; run tests to verify behaviour preserved."
write_skill bg-surgeon-split-module           bg ghu architectural "refactor, split"        "Split a too-large module into smaller modules." \
    "Define the split lines; move code; update imports; verify tests pass."
write_skill bg-surgeon-consolidate-duplication bg ghu any          "refactor, consolidate"  "Consolidate duplicated code into a shared helper." \
    "Identify the duplication; verify the helper preserves behaviour for every caller."
write_skill bg-surgeon-verify-behavior-preserved bg ghu any        "refactor, verify"       "Verify a refactor preserves observable behaviour by running tests before and after." \
    "Run the test suite before the refactor (must be green); run again after; require identical pass/fail set."

echo "wrote default worker skills under $SKILLS_DIR"
ls "$SKILLS_DIR" | wc -l
