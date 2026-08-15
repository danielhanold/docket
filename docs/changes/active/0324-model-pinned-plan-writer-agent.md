---
id: 324
slug: model-pinned-plan-writer-agent
title: 'Extract plan writing into a model-pinned internal agent'
status: implemented
priority: critical
type: feat
created: 2026-08-15
updated: 2026-08-15
depends_on: []
stacked_on:
related: [16, 17, 49, 96, 311, 315]
discovered_from: []
adrs: [8, 15, 16, 18, 44, 59, 64, 83, 94]
spec: docs/superpowers/specs/2026-08-15-model-pinned-plan-writer-agent-design.md
plan: docs/superpowers/plans/2026-08-15-model-pinned-plan-writer-agent.md
results: docs/results/2026-08-15-model-pinned-plan-writer-agent-results.md
trivial: false
auto_groomable:
branch: feat/model-pinned-plan-writer-agent
claimed_at: 2026-08-15T19:52:50Z
pr: https://github.com/danielhanold/docket/pull/209
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-15-model-pinned-plan-writer-agent-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-15-model-pinned-plan-writer-agent-design.md) |
| Plan | [2026-08-15-model-pinned-plan-writer-agent.md](https://github.com/danielhanold/docket/blob/feat/model-pinned-plan-writer-agent/docs/superpowers/plans/2026-08-15-model-pinned-plan-writer-agent.md) |
| Results | [2026-08-15-model-pinned-plan-writer-agent-results.md](https://github.com/danielhanold/docket/blob/feat/model-pinned-plan-writer-agent/docs/results/2026-08-15-model-pinned-plan-writer-agent-results.md) |
| PR | [#209](https://github.com/danielhanold/docket/pull/209) |
| ADRs | [ADR-0008](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0008-agent-layer-generated-subagents.md), [ADR-0015](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0015-harness-portable-agent-config.md), [ADR-0016](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0016-harness-first-agent-config.md), [ADR-0018](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0018-pluggable-skills-passthrough-degrade.md), [ADR-0044](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0044-autonomy-precedence-call-site-pre-specification.md), [ADR-0059](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0059-dispatch-capability-resolved-not-inferred-from-tool-name.md), [ADR-0064](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0064-shipped-agent-defaults-live-in-a-harness-indexed-sidecar.md), [ADR-0083](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0083-agent-worktree-scope-is-a-declared-frontmatter-fact.md), [ADR-0094](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0094-plan-authoring-is-a-pinned-internal-composition-agent.md) |
<!-- docket:artifacts:end -->

## Why

`docket-implement-next` currently authors the implementation plan at the orchestrator's own model
and effort. Lowering that pin to price routine orchestration more economically therefore lowers the
quality ceiling of the plan that guides every downstream build task.

Planning needs an independent model-and-effort boundary so the orchestrator can be tuned for its
coordination workload without coupling that choice to the judgment-heavy plan artifact.

## What changes

Add an internal, feature-worktree-scoped `docket-plan-writer` agent with independent shipped and
user-overridable model/effort settings. Keep planning inside `docket-implement-next` Step 4: the
parent prepares the worktree and context, dispatches the planner in the foreground, verifies its
committed plan artifact, and records the returned repo-relative path in `plan:`.

The plan writer continues to honor the resolved `skills.plan` binding, including custom plan
locations and the existing missing-skill fallback. Its plan commit persists the path for recovery;
the return is a non-terminal sub-step receipt, and the parent must attach the plan and continue into
the build. Update the agent/config documentation, dispatch and generator guards, resume contract,
and the Go embedded asset snapshot. Land this change before change 0315 so the Go migration's
claim-to-implemented workflow reconciles against the settled planning boundary.

## Out of scope

- Adding a public plan-writer skill or another workflow-role configuration key.
- Changing the default `skills.plan` binding, the build/review roles, or Step 4's lifecycle
  postcondition and top-level numbering.
- Moving plan judgment or harness-native agent dispatch into the Go engine.
- Changing change 0315's dependency graph or implementing any other Go-migration slice.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-08-15 — reconcile (implement-next)

Spec authored today; verified against current origin/main and it holds unchanged:
16 agent sources plus `agents/harness-defaults.yml` exist; `cmd/genassets` and the
`internal/assets/embedded/` tree from change 0311 are present, so the required
embedded-asset regeneration path is real. No related change has landed since the
spec was written; no scope adjustment needed. One coordination note: change 0308
(Go git adapter) is in-progress on a separate branch — this change touches no Go
runtime behavior, only regenerated embedded assets, so overlap is limited to a
possible mechanical regeneration collision at merge time.

## Run halted (resolved 2026-08-15 — see Halt resolution below)

**2026-08-15 — implement-next build halted on an invalidated scope premise (needs a human decision).**

The spec and plan both assert this change "touches no Go source behavior — only
regenerated embedded assets." That premise is **false**, discovered at the whole-suite
build gate. Adding a 17th shipped agent (`docket-plan-writer`) to
`agents/harness-defaults.yml` is, by the repo's own design, a **human-coordinated release
step**, not a mechanical asset regen.

Tasks 1–6 of the plan are committed and green (agent source, four-harness sidecar rows and
generated wrappers, the cursor dispatch fragment, the Step-4/convention/edge-paths/resume
contract, the docs, and the verify-run fixture). Two additional corrective commits closed
plan-under-anticipated fanout: `feat(0324): cursor dispatch fragment for docket-plan-writer
agent` and the count-guard sweep. What remains red under `go test ./...` needs work the
plan neither anticipated nor authorized, and one piece needs authority this autonomous run
does not have:

1. **`internal/config/defaults.go` — hand-authored Go source** (`builtinAgents()`) ships 16
   short names per harness; a `plan-writer` row (model + effort, all four harnesses) must be
   added. Explicitly outside the plan's "commit ONLY internal/assets/" scope.
2. **A NEW immutable versioned fixture tree — requires a docket release-versioning decision
   a human must make.** `TestBuiltinAgentsParityWithFrozenSidecar` byte-compares the live
   `agents/harness-defaults.yml` (now 17) against the FROZEN
   `testdata/repositories/v0.9.2/agents-harness-defaults.yml` (16). `testdata/README.md` and
   the test's own remedy string forbid editing `v0.9.2/` ("immutable input… a new upstream
   state gets a NEW versioned tree, never an edit"). Cutting that tree means **choosing the
   docket release version** the snapshot is named for, copying the cross-package tree,
   writing `PROVENANCE.md`, and re-pointing `sidecarPath` in `internal/config/defaults_test.go`.
   An autonomous run must not invent a release version and bake it into an immutable tree —
   a wrong or colliding version is unrecoverable.
3. **`TestBuiltinAgentsShape`** hardcodes "exactly the 16 canonical short names" → 16→17 +
   `plan-writer`.
4. **Golden refreezes** via `go test -update` in `internal/harness/{claude,codex,cursor,opencode}`
   to create the `plan-writer` goldens.

**What a human must decide before this can resume:** (a) the docket **release version** under
which to cut the new frozen fixture tree (`testdata/repositories/v<release>/`), and (b)
confirmation that this change's scope legitimately expands to include the Go-side
reconciliation (defaults.go + shape test + a new versioned fixture tree + four harness
golden refreezes) — or that the Go-side coupling should instead be split into its own
change. The spec's *Out of scope* line "Moving plan judgment or harness-native agent
dispatch into the Go engine" did not foresee that merely *shipping* an agent already touches
`internal/config` and the frozen-fixture release tripwire.

**Worktree state:** `feat/model-pinned-plan-writer-agent` at HEAD carries Tasks 1–6 + the two
corrective commits. The count-guard sweep (10 test files → 17) and the embedded-asset regen
are **present but uncommitted** in the feature worktree (a build worker returned BLOCKED
without committing; this run did not adopt them). A resume should re-derive or commit those
under its own authorship after the human resolves the release-version decision above.

## Halt resolution (2026-08-15 — human decision)

The two decisions the halt asked for are made; the scope premise is corrected in the spec and
plan, so this change is resumable:

- **Scope: fold into 0324.** The Go-side reconciliation of the shipped agent registry belongs to
  this change, as one finalizable unit — shipping the 17th agent is not separable from registering
  it. The spec's `## Go-migration isolation` section is rewritten accordingly; the plan's false
  "touches no Go source" constraint is replaced and a Go-reconciliation task added.
- **Release version: `0.9.3`.** The new immutable frozen fixture tree is
  `testdata/repositories/v0.9.3/`, **sparse** — only `agents-harness-defaults.yml` (17-agent byte
  copy) + a tree-wide `PROVENANCE.md`; every other frozen input stays on `v0.9.2/`. Only
  `sidecarPath` in `internal/config/defaults_test.go` re-points to `v0.9.3`. The Bash rollback tag
  stays `v0.9.2`.
- **Git tag ordering.** The `v0.9.3/` directory name and in-code version references land in this
  change; the actual **git tag `0.9.3` is cut only after 0324 merges and is confirmed working**.
- **Program map.** A cross-reference note is added to
  `docs/superpowers/specs/2026-08-12-go-migration-program-map.md` recording that post-sprint change
  0324 introduces the 17th shipped agent and the `v0.9.3` agent-registry release. No new sprint
  change is created (the reconciliation folds into 0324).

The `## Run halted` heading above is renamed (no longer a bare `## Run halted` line) so `verify-run`
no longer classifies this change as halted; the halt narrative is preserved as history.
