---
id: 51
slug: global-agents-middle-layer
title: Machine-local config layer (.docket.local.yml) + all-local agent generation
status: done
priority: high
created: 2026-07-09
updated: 2026-07-10
depends_on: [50]
related: [45, 46, 48, 50]
adrs: [8, 15, 16, 17, 19, 20]
spec: docs/superpowers/specs/2026-07-09-global-agents-middle-layer-design.md
plan: docs/superpowers/plans/2026-07-09-global-agents-middle-layer.md
results: docs/results/2026-07-09-global-agents-middle-layer-results.md
trivial: false
auto_groomable: false
branch: feat/global-agents-middle-layer
pr: https://github.com/danielhanold/docket/pull/60
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-09-global-agents-middle-layer-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-09-global-agents-middle-layer-design.md) |
| Plan | [2026-07-09-global-agents-middle-layer.md](https://github.com/danielhanold/docket/blob/feat/global-agents-middle-layer/docs/superpowers/plans/2026-07-09-global-agents-middle-layer.md) |
| Results | [2026-07-09-global-agents-middle-layer-results.md](https://github.com/danielhanold/docket/blob/feat/global-agents-middle-layer/docs/results/2026-07-09-global-agents-middle-layer-results.md) |
| PR | [#60](https://github.com/danielhanold/docket/pull/60) |
| ADRs | [ADR-0008](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0008-agent-layer-generated-subagents.md), [ADR-0015](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0015-harness-portable-agent-config.md), [ADR-0016](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0016-harness-first-agent-config.md), [ADR-0017](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0017-cursor-dispatch-rule-full-agent-set.md), [ADR-0019](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0019-global-config-fence-classification.md), [ADR-0020](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0020-generated-agent-artifacts-machine-local.md) |
<!-- docket:artifacts:end -->

## Why

Live testing of change 0050 (2026-07-09, Daniel) surfaced that the global `agents:` block is
**dead in any repo that opts into per-repo generation**: change 0048's always-full-set pass
commits ALL wrappers resolved from `.docket.yml` + built-ins only, so agents without a
per-repo override are pinned to built-in Claude IDs, and those committed files shadow the
user-level wrappers carrying the global models. The 0050 docs promise "per-repo > global >
built-in, field-by-field" — false for `agents:` in opted-in repos, exactly the tested case.
A stopgap shadowing warning went into PR #59; this change ships the real semantics.

The grooming brainstorm (2026-07-09) identified the root tension: model/effort choices are
**machine** preferences, but 0048 forces them through **committed** files, where the
ADR-0019 fence correctly forbids the global layer from participating. Patching the committed
model (fall-through, seed command, docs-only — all examined and rejected in the spec) leaves
some variant of the shadowing problem alive; removing committed generation dissolves it.

## What changes

Per the linked spec: stop committing generated agent artifacts entirely, and add the
machine-scoped per-repo config file that 0050 deferred.

- **`.docket.local.yml`** (repo root, gitignored): machine-and-repo-scoped overrides for
  exactly the ADR-0019 global-able key set; fenced keys warned-and-ignored. Per-field
  precedence becomes **repo-local > repo-committed > global > built-in** (the `.env`
  pattern), resolved by `docket-config.sh --export`; skills' Step-0 interface unchanged.
- **All-local agent generation:** the per-repo pass still writes the full built-in agent
  set (ADR-0017's by-construction dispatch guarantee kept) plus the Cursor dispatch rule,
  but as **gitignored local files**, each field resolved through all four layers in one
  pass. Opt-in via `agents:`/`agent_harnesses:` in either the committed or the local file.
  The PR #59 stopgap warning is removed; the Cursor user-registry question is moot.
- **Managed `.gitignore` block** (`# docket:generated:start/end`) owned by
  `sync-agents.sh`, strictly docket-scoped.
- **Migration + `--check`:** first run in a 0048-era repo deletes tracked wrappers, writes
  the block, regenerates locally, and prints the single migration commit. `--check` becomes:
  block current + no tracked `docket-*` files (CI-meaningful) + local staleness (advisory).
- **Docs + ADR:** README/convention rewritten to the four-layer story; a build-time ADR
  supersedes ADR-0017's committed-generation model and updates ADR-0008/0016 — the
  clone-identical-committed-wrapper guarantee is consciously retired (solo-first call).
- **Rider (from 0050 results):** guard `prune_orphans`' empty `scan_dirs[@]` expansion in
  `sync-agents.sh` against macOS bash 3.2 under `set -u` (pre-existing hazard, explicitly
  deferred to "the next sync-agents change" — this one).

## Out of scope

- A seed command (rejected in favor of the local layer).
- New global-able keys beyond the ADR-0019 set, or any fence reclassification.
- Changes to `skills:` runtime semantics beyond the added resolution rung.
- Board/GitHub-mirror behavior changes; user-level pass changes.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

- **2026-07-09 (implement-next):** Groomed and claimed the same day, so the world has barely
  moved. Verified against `origin/main` @ `b331756`: change 0050 is `done` (PR #59 merged +
  terminal-published), satisfying `depends_on: [50]`; the PR #59 stopgap shadowing warning this
  change removes exists at `sync-agents.sh:361–368`; ADR-0019 (fence classification) is
  published and its global-able key set matches the spec's `.docket.local.yml` key list;
  `docket-config.sh` carries 0050's global rung (`GCFG=…/config.yml`, misplacement guard) ready
  for the fourth layer. Scope adjustment: folded in the bash-3.2 `prune_orphans` empty-array
  guard that 0050's results file explicitly deferred to "the next sync-agents change". No work
  done elsewhere to drop; spec stands as approved.
