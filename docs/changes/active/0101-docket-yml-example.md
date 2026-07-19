---
id: 101
slug: docket-yml-example
title: .docket.yml.example — the canonical all-comprehensive config reference
status: implemented
priority: medium
created: 2026-07-19
updated: 2026-07-19
depends_on: []
related: [81]
discovered_from: []
adrs: [19, 39, 48]
spec: docs/superpowers/specs/2026-07-19-docket-yml-example-design.md
plan: docs/superpowers/plans/2026-07-19-docket-yml-example.md
results: docs/results/2026-07-19-docket-yml-example-results.md
trivial: false
auto_groomable:
branch: feat/docket-yml-example
claimed_at: 2026-07-19T22:18:54Z
pr: https://github.com/danielhanold/docket/pull/109
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-19-docket-yml-example-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-19-docket-yml-example-design.md) |
| Plan | [2026-07-19-docket-yml-example.md](https://github.com/danielhanold/docket/blob/feat/docket-yml-example/docs/superpowers/plans/2026-07-19-docket-yml-example.md) |
| Results | [2026-07-19-docket-yml-example-results.md](https://github.com/danielhanold/docket/blob/feat/docket-yml-example/docs/results/2026-07-19-docket-yml-example-results.md) |
| PR | [#109](https://github.com/danielhanold/docket/pull/109) |
| ADRs | [ADR-0019](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0019-global-config-fence-classification.md), [ADR-0039](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0039-config-example-mirrors-wrapper-defaults.md), [ADR-0048](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0048-docket-yml-example-invariants.md) |
<!-- docket:artifacts:end -->

## Why

docket's config documentation is scattered across three drifting surfaces — this repo's own
`.docket.yml` (expansive but incomplete), `config.yml.example` (the change-0081 global
starter, with duplicated reclaim/auto_capture prose), and the script contracts. No single
file shows a user every key, its default, its documentation, and which layer (repo,
`.docket.local.yml`, global) may set it. Users have nowhere authoritative to copy from.

## What changes

A new committed `.docket.yml.example` at the repo root becomes the canonical reference,
Helm-values style: every key active at its shipped default with full per-key documentation
and a scope tag (repo-only coordination-fenced vs any-layer). Presence-sensitive keys
(`agents:`, `agent_harnesses:`) ship commented with a loud marker. A new `auto` sentinel
(≡ unset) for `finalize.test_command` and `github_project` lets those defaults ship
explicitly. `config.yml.example` is deleted; `install.sh` scaffolds a minimal pointer-only
global config instead; this repo's `.docket.yml` slims to its set values + pointer; the
README retargets. A new test file enforces example = resolver defaults (fidelity),
key completeness — over both the exported keys and the four non-exported schema keys
(`github_project`, `agents`, `agent_harnesses`, `finalize.require_pr_approval`) — and the
relocated ADR-0039 agents-mirror equality. A new ADR supersedes ADR-0039. The example's header states the standing rule: every new config flag lands in
this file — value + docs — in the same PR.

## Out of scope

- Codegen of the example from the resolver (manual mirror + tests is the accepted trade-off).
- New config keys or behavior changes beyond the `auto` sentinel.
- Validating harness model IDs in the commented codex/cursor blocks.
- Moving the authoritative scope-classification table out of `scripts/docket-config.md`.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-07-19 — reconciled against origin/main @ current tip

Spec re-read against CURRENT code. **Every assumption still holds**, no scope drop: `config.yml.example`,
`tests/test_config_example.sh`, `scripts/ensure-global-config.sh`/`.md`, the repo's own `.docket.yml`,
and the README setup/Configuration sections are all present and unchanged on `origin/main`. Change 0081
(the surface being consolidated) is archived done; ADR-0039 is `Accepted` and still names
`config.yml.example` as the mirror — so the supersede-with-a-new-ADR step is still exactly right.

Two sharpenings fold in — both refine deliverables, neither invalidates the design:

1. **The `auto` sentinel for `github_project` cannot be proven by the fidelity test as specced.**
   `github_project` is *not* in the resolver's export surface: `scripts/docket-config.sh` only
   coordination-fences it (`:169`), and the value is consumed by `scripts/github-mirror.sh` through the
   Board pass's `--project` flag, with a `project-minted` write-back into `.docket.yml`. So the spec's
   test 1 (`--export --format plain` byte-identical with vs. without the file) is blind to it, and test
   2's export-key → YAML-path mapping has no key to map. The sentinel must be implemented where the
   value is actually read/written-back, and covered by a dedicated assertion rather than the
   export-diff. `finalize.test_command` is unaffected — it *is* exported, so the export-diff proves that
   half of decision 5 exactly as written.

2. **Completeness (test 2) needs a second, explicit list for non-exported schema keys.** Four keys are
   part of the config schema but have no export key: `github_project`, `agents:`, `agent_harnesses:`
   (all consumed by `sync-agents.sh` / the mirror) and **`finalize.require_pr_approval`** — a
   *model-read* key, read only by `skills/docket-finalize-change/SKILL.md`, present-but-commented in this
   repo's `.docket.yml` and implemented in no script. An export-key-driven completeness check would
   silently under-cover all four. `require_pr_approval` is the sharpest case: the spec's own key
   inventory (§Body grouping) never names it, so without this note the canonical reference would ship
   missing a real key on day one — precisely the drift the change exists to end.

Both are folded into the plan as test-shape and key-inventory requirements. No open questions reopened.
