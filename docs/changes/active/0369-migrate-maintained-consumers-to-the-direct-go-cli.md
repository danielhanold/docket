---
id: 369
slug: 'migrate-maintained-consumers-to-the-direct-go-cli'
title: 'Migrate maintained consumers to the direct Go CLI'
status: 'in-progress'
priority: 'critical'
type: 'refactor'
created: '2026-08-29'
updated: '2026-08-29'
depends_on: [318]
stacked_on:
related: [317, 322, 326, 361, 366, 367, 370]
discovered_from: [318]
adrs: [14, 29, 30, 33, 36, 74, 99]
spec: 'docs/superpowers/specs/2026-08-29-migrate-maintained-consumers-to-the-direct-go-cli-design.md'
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'refactor/migrate-maintained-consumers-to-the-direct-go-cli'
pr:
blocked_by:
reconciled: false
claimed_at: '2026-08-29T23:33:57Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-29-migrate-maintained-consumers-to-the-direct-go-cli-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-29-migrate-maintained-consumers-to-the-direct-go-cli-design.md) |
| ADRs | [ADR-0014](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0014-consuming-repo-script-resolution.md), [ADR-0029](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0029-docket-facade-routing-and-config-presentation.md), [ADR-0030](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0030-facade-wiring-guard-discriminates-on-invocation-prefix.md), [ADR-0033](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0033-cursor-auto-run-trust-at-facade.md), [ADR-0036](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0036-codex-agents-md-dispatch-block-committed-machine-neutral.md), [ADR-0074](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0074-build-gate-verdict-is-tri-state-runner-defined-non-failure-exit-is-a-halt.md), [ADR-0099](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0099-one-metadata-topology-for-go-v1.md) |
<!-- docket:artifacts:end -->

## Why

The Go command surface cannot become the only supported control plane while skills, agents, generators, workflows, setup checks, and operator instructions still route through the legacy Bash facade. Migrating these maintained consumers is a repo-wide but reviewable boundary that must land before deletion.

## What changes

- Derive a whole-repository executable-site inventory and classify every active, generated,
  legacy, historical, and unknown occurrence.
- Rewrite maintained skills, agents, canonical generators, generated dispatch blocks, workflows,
  setup/health checks, validators, active instructions, and executable examples to invoke the
  PATH-resolved public Go CLI and consume JSON where machines interpret results.
- Regenerate products from canonical sources and prove deterministic, machine-neutral output and
  representative fresh-process loading.
- Leave the Bash facade/runtime/old runner and their tests behaviorally frozen and green, but prove
  that they have no maintained callers.
- Replace the facade-wiring rule with a fail-closed, shape-derived, mutation-tested no-new-callers
  guard and record the direct-Go architecture through the ADR workflow.

## Out of scope

Physical facade/runtime/configuration/test deletion (0370); a replacement forwarding shim; missing
Go capability invention; raw Git/GitHub mutation as a substitute for Docket; release publication,
rollback, or real-host acceptance (0366); post-cutover board work (0367); and rewrites of immutable
historical or frozen records.

## Design decisions

This is a sequential merged-main dependency on 0318, not a stacked branch. The intermediate state
must be independently green and usable. Process-start-loaded artifacts receive honest generator and
hermetic fresh-process evidence; live external harness reload remains human truth. Unknown inventory
or guard classification fails closed. The old implementation remains present but ceases to be a
supported integration contract.

## Run halted

### 2026-08-29

**Halted at reconcile (Step 3) — 2026-08-29, by an autonomous `docket-implement-next` run.**

## Disposition: halted — a design precondition is unmet on the reconciled base

Change 0369 migrates every maintained executable consumer off the Bash facade (`scripts/docket.sh`, `DOCKET_SCRIPTS_DIR`) to the PATH-resolved public Go `docket` CLI. Its central assumption (spec *Assumptions*: "The public Go CLI already exposes the lifecycle operations and structured contracts maintained consumers need") is **false on the current base**. The spec's own *Public Go boundary* and *Failure handling* make this an explicit stop, not a scope adjustment: "If a consumer requires behavior not exposed by the public Go CLI, implementation halts for reconciliation rather than recreating it in a consumer or shim," and inventing Go capability or a forwarding shim are declared non-goals. This run therefore halts and hands back to a human rather than inventing verbs, adding a shim, or shipping a partial migration that fails acceptance criteria 4, 13, and 23.

## Evidence (facade op → public Go verb, verified against the installed `docket` CLI)

**Absorbed — no blocker.** The Go mutating transactions render these internally (they write `BOARD.md` / the `## Artifacts` block inside their own commit), so maintained consumers simply drop the separate call: `board-refresh`, `render-change-links`, `adr-checks`, `board-checks`.

**Cleanly mapped — no blocker.** `env`/`preflight` config read → `docket config`; `archive-change`/`stack-closeout` → `docket finalize closeout`; `cleanup-feature-branch` → `docket finalize cleanup`; `reclaim-claims` → `docket change reclaim` / `docket maintenance sweep`; `verify-run` → `docket run verify`; `gate-before`/`gate-verdict` → `docket run gate-before`/`gate-verdict`; `render-artifact-backlink` → `docket artifact backlink`; `docket-status` read → `docket status` + `docket maintenance sweep`.

**Missing — needed by maintained consumers, not absorbed into any lifecycle transaction, and forbidden to invent in 0369:**

1. **`runner-dispatch`** — the parent-harness delegation adapter that `sync-agents.sh` bakes into the generated runner-shim wrapper (`docket.sh runner-dispatch --launch/--observe`) and that reviewer-rung `--worktree` delegation depends on. `docket gate` exposes launch/drive/observe/stop/recover/cleanup — **no delegation verb**. This is a maintained *generated product*; it cannot be regenerated to call a Go verb that does not exist.
2. **`mint-stub`** — the CAS-correct auto-capture mint (a `proposed` needs-brainstorm stub with `discovered_from:`/`type:` set, one per call). `docket change create` is the authored-markdown create path, not the mint. Auto-capture is a maintained runtime path in `docket-implement-next` (reconcile/review) and the finalize/status harvest.
3. **`render-adr-index`** (used by the `docket-adr` skill) and **`render-learnings-index`** (used by the harvest) — there is no `docket adr index` / `docket learning index` verb. Whether `docket adr record/supersede/reverse` and `docket learning record/update` re-render their index internally (the way change transactions re-render the board) is **unconfirmed**; the migration's own rule is to fail closed on that uncertainty, and it cannot be resolved without reading and proving each transaction's internals.
4. **`terminal-publish`** and **`mark-publish-deferred`** — used by the `docket-adr` and `docket-status` skills; no direct public Go verb (finalize's publish/closeout act on feature heads and archival, not the marker/terminal-record-copy operations these skills invoke).

Standalone board refresh (`docket.sh docket-status --board-only`, invoked by 6 skills and the reconcile-kill path) also has no public Go standalone verb; `docket status` is read-only. Whether every maintained caller's board pass is fully covered by transaction-embedded rendering is part of the same fail-closed uncertainty.

## Why this is halt, not a partial build

The acceptance criteria are all-or-nothing for the migration boundary: #4 "Every maintained consumer invokes the public CLI directly from PATH," #13 "The frozen implementation has no maintained executable caller," #23 "a final whole-repository scan finds no unclassified executable-shaped facade/helper invocation." Migrating only the consumers that have Go verbs leaves `docket-adr`, `docket-auto-groom`, `docket-groom-next`, `docket-new-change`, the `docket-convention` Step-0/board-pass, `docket-status`, and the `sync-agents.sh`-generated runner shim still routing through the facade for the operations above — so criteria #4/#13/#23 cannot be met, and the no-new-callers guard (#14) cannot be made fail-closed clean.

## Recommended human action

Land the missing **public** Go CLI surface first, as prerequisite work outside 0369's non-goals, then re-dispatch 0369. At minimum: a public delegation verb replacing `runner-dispatch`; a public auto-capture mint verb replacing `mint-stub`; public `terminal-publish` and `mark-publish-deferred` (or a confirmed finalize/maintenance path that subsumes them); and a written confirmation (with tests) that ADR-index, learnings-index, and standalone board refresh are re-rendered inside the relevant Go transactions so the `render-adr-index`/`render-learnings-index`/`--board-only` calls can be dropped rather than replaced. Alternatively, re-scope 0369 via `docket-new-change` to migrate only the already-covered consumer set and explicitly defer the facade-only operations (and their consumers) to a follow-up — but that is a human re-brainstorm decision, which this autonomous skill cannot make.

## Run state

Claimed `in-progress` on branch `refactor/migrate-maintained-consumers-to-the-direct-go-cli`; **no feature branch was cut and no code was written** — the halt is at reconcile, before planning. `reconciled` remains `false`. The claim lease and this marker are the only mutations; the reclaim lease self-heals the claim if left. A human resumes with `docket change resume-halted --id 369 --acknowledge-quiescent` after landing the prerequisite Go surface, or kills/re-scopes the change.
