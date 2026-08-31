<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0377 — Migrate deferred Bash-facade workflow operations to native Go CLI verbs](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-31-0377-migrate-deferred-bash-facade-workflow-operations-to-native-g.md)**
<!-- docket:backlink:end -->

# Native Workflow Closure Before Bash Facade Deletion

## Summary

Change 0377 supplies the last native workflow capabilities required before change 0370 can delete
the frozen Bash facade. It does not port each Bash command one for one. Instead, it maps every
retained consumer to the Go capability that already owns the behavior, adds one missing repository
preparation operation, fills narrow health/context gaps, and cuts canonical skills plus generated
products over to the typed command surface.

The central rule is capability ownership. Board rendering, artifact-link rendering, ADR-index
rendering, stack resolution, terminal closeout, and status projection stay with their existing Go
owners. A skill never invokes a renderer or performs a second derived-view write after a typed
mutation. The Bash implementation remains present and frozen as the migration host and parity
oracle until 0370 removes it.

## Context and dependency boundary

Change 0369 migrated retained consumers only where an exact public Go operation already existed. It
deliberately left `preflight`, the Board pass, change-link rendering, ADR checks, and stack helpers
on the facade. Changes 0371 and 0372 then completed native dispatch and retired deferred feature
families without supplying these missing homes.

Change 0370 consequently halted before code was written: its deletion criterion cannot be met while
the retained workflow skills still execute the facade. Change 0377 is the predecessor that closes
that gap. Its dependency on 0372 is already satisfied; it is not stacked on 0370 and must merge to
the integration branch independently. Change 0370 remains run-halted until 0377 is merged and a
human explicitly resumes it.

The approved Go-migration architecture remains authoritative:

- Go v1 uses the fixed `docket` metadata branch and `.docket` worktree; main mode is gone.
- The GitHub Issues/Projects backlog mirror is dropped. Historical `issue:` and `github_project`
  values remain inert, and requesting the `github` board surface blocks mutations.
- Planning mutations use protocol-v1 typed operations and exact-version transactions.
- The inline board and other generated artifacts are derived views, never independent authority.
- Deferred product features are not reintroduced as part of deleting the facade.

## Goals

- Replace the shared Step-0 Bash preflight with one native, structured repository-preparation
  operation.
- Map every retained facade consumer to an existing or narrowly extended Go capability.
- Remove the routine Board pass and every routine direct-renderer call from maintained workflows.
- Make repository health and authorized repair cover deterministic board, artifact-link, and
  ADR-index drift.
- Cut canonical skills, references, generated assets, and maintained parent-facing instructions to
  the native CLI without a forwarding shim.
- Prove by shape and mutation that no maintained executable consumer requires the migrated facade
  operations.
- Leave the frozen Bash facade intact, green, and deletable by 0370.

## Non-goals

- Deleting `scripts/docket.sh`, `scripts/lib`, `scripts/run-tests.sh`, legacy tests, compatibility
  launchers, or runtime/configuration seams; change 0370 owns physical deletion.
- Resuming, re-dispatching, implementing, replanning, or opening a PR for change 0370.
- Reintroducing automatic capture, automated learning maintenance, terminal publication, the
  GitHub board mirror, main mode, or another deferred/retired capability.
- Creating a Go forwarding facade whose command names mirror the Bash operation table.
- Redesigning lifecycle policy, metadata topology, transaction semantics, selection, or branch
  stacking.
- Rewriting archived changes, Accepted ADR bodies, historical specs/results, or frozen v0.9.2
  artifacts.
- Release, tag, asset, fresh-host acceptance, or rollback work.

## Chosen architecture and rejected alternatives

### Chosen: capability-oriented closure

Each facade operation is classified by the capability it provides. Existing typed commands absorb
behavior they already own; only a genuinely missing independently callable capability receives a
new command. Skills consume structured protocol results rather than recreating shell orchestration.

This keeps one owner for each rule and makes 0377 a coherent closure change rather than a second
control-plane implementation.

### Rejected: one-for-one Go verbs

Adding Go commands named after every Bash renderer and stack helper would preserve the facade's
decomposition and split-write failure modes. It would duplicate public interfaces for behavior
already present inside typed transactions and maintenance/context operations.

### Rejected: one umbrella Go workflow facade

A single command dispatching all legacy operation names would merely translate the Bash facade into
Go. It would keep stringly routing, blur read versus mutation boundaries, and create a compatibility
layer that 0370 could not truthfully delete.

### Rejected: eliminate all local metadata preparation

Moving every authoring/read flow into new context APIs could eventually remove the need for a local
metadata worktree, but that is a larger workflow redesign. Current interactive skills and human
inspection still need a safe synchronized `.docket` checkout. One typed preparation operation is
the smaller, explicit boundary.

## Legacy capability disposition

| Legacy capability | Native disposition |
|---|---|
| `docket.sh preflight` | New `docket repository prepare --json` operation |
| Flat `env`/preflight output | Structured preparation context; any unused `env` route remains frozen for 0370 |
| Plain `docket-status` | `docket maintenance sweep --json`, then read-only `docket status --json` |
| `docket-status --board-only` | Removed from maintained workflows; board-visible mutations render atomically |
| `render-change-links` | Absorbed by typed change/finalize operations; exceptional drift uses repository health/repair |
| `render-adr-index` | Absorbed by typed ADR operations; exceptional drift uses repository health/repair |
| `adr-checks` | Required checks become repository-health findings; no separate ADR checker command |
| `stack-base` | `docket context implementation` supplies the typed effective-base result |
| `stack-children` | Existing/extended structured finalize or maintenance context supplies descendants |
| `stack-closeout` | Existing typed finalize operations and `docket maintenance sweep` own closeout |

The build derives the complete executable inventory from the current tree and classifies every site.
This table describes the behavioral mapping, not a fixed filename or call-count allowlist.

## `docket repository prepare`

### Purpose and interface

`docket repository prepare --repo-dir <dir> --json` is the sole shared Step-0 operation. Its
operation key is `repository.prepare`. The only caller input is the repository directory; all
configuration and topology facts are resolved from authoritative sources.

Preparation performs this ordered flow:

1. Discover the canonical repository and origin.
2. Pin the default branch, load the committed configuration, merge supported machine layers, and
   resolve the integration branch.
3. Gather the existing repository-classifier facts and classify topology once.
4. Require the fixed Go v1 `docket`-branch topology.
5. Ensure the healthy repository's `.docket` worktree exists, is correctly registered to the
   metadata branch, and has worktree-local hooks disabled.
6. Synchronize a clean, behind metadata worktree to the pinned remote metadata revision.
7. Return the exact context the workflow may carry forward.

It creates no planning record, makes no lifecycle decision, and pushes no remote mutation.

### Structured result

An applied or no-op protocol-v1 result contains typed fields for:

- canonical repository root and origin identity;
- default, integration, and metadata branch names plus their pinned revisions;
- fixed metadata worktree path;
- resolved changes, ADR, and results directories;
- supported finalize/test configuration;
- resolved workflow skill bindings and other settings that current maintained consumers actually
  read; and
- configuration diagnostics and repository-state notices.

The schema is a closed typed context derived from the consumer inventory, not a generic string map.
It does not emit shell assignments, quote values for a shell, expose an `eval` interface, or retain
legacy `DOCKET_*` transport variables merely because preflight once printed them. Human output is a
short redacted summary; agents consume JSON.

### State and refusal behavior

- A fresh repository refuses with the exact `docket repository init` remedy.
- A legacy repository refuses with the exact `docket repository migrate` remedy.
- A healthy remote whose local `.docket` worktree is absent may be attached idempotently.
- A clean local metadata worktree strictly behind the remote may fast-forward to the pinned remote
  revision.
- Dirty, ahead, diverged, foreign, ambiguously registered, or probe-unknown local state refuses
  without overwriting, rebasing, resetting, deleting, or adopting anything.
- Invalid or unsupported configuration refuses before local synchronization.
- Lost-response retry converges by re-reading current topology and revision state.

Preparation never implicitly initializes or migrates a repository. It may materialize only the
local checkout of an already-valid remote topology.

## Workflow data flow

Every operating skill begins with `repository prepare --json`, validates the protocol envelope, and
carries its typed context as literal values. It then invokes the narrow command for its work:

- New-change and grooming flows submit authored Markdown through `docket change create` and
  `docket change groom`. These operations atomically write the record, spec when applicable,
  artifact links, and inline board.
- Implementation uses `docket context implementation` for source bytes, exact versions,
  relationships, readiness, claim eligibility, and effective base before typed claim/reconcile
  mutations.
- Status invokes `docket maintenance sweep --json` for the mutating terminal/recovery half, then
  `docket status --json` for the fresh read-only backlog and health projection.
- ADR workflows use typed record/supersede/reverse operations, which own index rendering.
- Finalize uses typed context and finalize operations for merge, descendants, retargeting,
  terminal closeout, and cleanup.

No skill parses human output, reconstructs a branch from prose, invokes a renderer, edits a managed
block, or follows a typed mutation with a second board commit.

## Derived-view ownership

There remains exactly one pure renderer for each derived view. Board-visible app operations call a
shared app-layer inclusion path that renders the candidate after-state through the canonical board
renderer and includes `BOARD.md` in the same transaction file set. They do not contain private
copies of board grouping, ordering, readiness, dependency, or formatting logic.

A mutation-sensitive structural guard proves that an operation changing a board-authoritative
change record cannot land without the canonical board output in the same transaction. The guard is
keyed on mutation shape and common ownership, not a hand-maintained list of command names.

The same ownership rule applies to artifact links and the ADR index:

- change/finalize operations render artifact links from the candidate snapshot;
- ADR operations render the index from the candidate snapshot; and
- retries recompute all derived bytes from fresh authority rather than merging stale generated
  output.

There is no routine native Board-pass command. Direct hand-editing of authoritative metadata is an
exceptional repair case, not a second supported write architecture.

## Repository health and authorized repair

`docket repository check` remains read-only. It compares the pinned metadata corpus with canonical
rendering and emits stable findings for:

- stale or malformed inline board output;
- stale, missing, or malformed managed artifact-link blocks;
- stale or malformed ADR-index output;
- ADR identity, status, relationship, filename, and ledger consistency not already covered by the
  shared repository snapshot validator; and
- existing topology and frontmatter findings.

Each deterministic derived-view difference is marked mechanically repairable. Authored prose,
ambiguous markers, illegal ADR evolution, missing referenced artifacts, and other judgment-bearing
conditions remain manual-review errors.

`docket repository migrate` extends its existing preview/authorization model to repair the
mechanically repairable findings even when topology is already healthy. The preview identifies the
exact pinned source revision and file set; an authorized retry re-proves that revision and
recomputes canonical bytes. Marker order and balance are validated before any block replacement.
The operation never treats a failed read or renderer error as clean absence and never changes
authored content to make a check pass.

The approved GitHub-board decision is unchanged: historical `issue:` and `github_project` values
are inert, and a requested `github` board surface is an unsupported mutation configuration. No
GitHub mirror repair or write path is added.

## Stack and maintenance behavior

Stack behavior remains domain-owned:

- implementation context returns the tagged effective-base result and branch;
- finalize/maintenance context returns the descendant information its caller needs, extended only
  if a currently retained `stack-children` consumer lacks a typed field;
- typed finalize operations own retargeting and per-change terminal transitions; and
- `maintenance sweep` owns batch merged-change closeout, including completed stack promotion and
  item-isolated recovery.

No standalone `stack-base`, `stack-children`, or `stack-closeout` compatibility verbs are added.
Unknown branch facts, killed parents, missing parents, cycles, and failed PR probes retain typed
fail-closed outcomes; none silently falls back to the integration branch.

## Skill and generated-product migration

The cutover updates canonical executable instructions first, including operating skills, shared
convention/reference files, registered-agent parent instructions, and maintained operator material.
Generated embedded assets and harness products are then regenerated through their canonical
generator, and a second generation must be byte-clean.

The migration removes maintained executable dependence on `DOCKET_SCRIPTS_DIR`,
`DOCKET_BASH_PATH`, `docket.sh preflight`, the Board pass, direct renderers, ADR checks, and stack
helpers. Historical prose and frozen artifacts retain point-in-time truth.

New command implementations land before their skill call sites inside the same independently usable
PR. The old facade remains functional throughout the build, so the migration host can build its own
replacement. Existing installed old skills continue to work until installation is upgraded; the
normal post-merge source install updates the binary and generated skill products together. No
forwarding shim is introduced.

## Protocol and failure model

Every native operation returns a protocol-v1 envelope with a stable operation key, closed result or
disposition vocabulary, structured findings, and typed context/receipt data. Exit codes are a
presentation mapping, not the agent control interface.

Configuration errors, unsupported behavior, topology uncertainty, Git observation failures, dirty
or divergent worktrees, malformed managed markers, stale exact versions, and transaction
contention fail closed. A mutation either commits its complete primary-plus-derived file set or
writes nothing.

`maintenance sweep` retains its existing item-isolation contract: one failed item does not suppress
independent items, destructive suffixes are withheld after an unresolved prerequisite, and every
processed item has a structured disposition. Read-only `status`, `context`, and `repository check`
operations fabricate neither success nor absence when a required probe fails.

## Consumer seal

After rewiring, a whole-repository, shape-derived guard classifies executable facade dependencies in
canonical sources and generated products. It detects direct execution, variable-composed execution,
indirect delegation, sourced runtime dependencies, and generator-emitted calls.

The allowed categories are structural:

- the frozen Bash implementation and its parity/deletion tests;
- immutable archived changes, Accepted ADR history, specs, plans, and results; and
- frozen release artifacts.

An unknown executable site fails the guard. Exclusions are never a growing filename or count
allowlist. Mutation probes introduce each forbidden shape in maintained material and prove the
guard reddens. Removing the consumer population or bypassing canonical generation must also redden
through a non-vacuity floor derived from the maintained product shape.

## Verification strategy

Focused unit and app-layer tests cover repository classification, every preparation disposition,
typed JSON fields and redaction, canonical render inclusion, repository-health findings, repair
planning, context extensions, and structured error mapping.

Integration tests use local repositories and remotes to cover worktree attachment, clean-behind
synchronization, dirty/ahead/diverged refusal, exact-revision repair authorization, transaction
contention/retry, interrupted recovery, and semantic parity with the frozen Bash implementation
where it remains the oracle.

Test placement and runtime are requirements:

- long-running or real-repository scenarios go in the existing integration group;
- tests are split or sharded so no individual test runs longer than 60 seconds;
- unit packages do not absorb integration-scale fixtures that destabilize the ordinary suite;
- the whole gate is `go run ./cmd/docket development test` from the source checkout under review;
- `BUDGET WATCH:` and `PARALLEL-SENSITIVE:` findings are inspected; and
- any `SERIAL CONFIRMED OVER BUDGET:` finding is an authoritative breach that must be resolved.

Mutation evidence must cover the preparation gate, dirty/diverged safety, every derived-view wiring
invariant, consumer classification shapes, generated-product drift, and failed-probe-not-absence
behavior.

## Sequencing and recovery

The implementation sequence is:

1. Land and test repository preparation.
2. Fill only the health/context gaps proven by the consumer inventory.
3. Establish shared derived-view inclusion and its mutation guard.
4. Rewire canonical skills and references.
5. Regenerate products and prove deterministic parity.
6. Install the final consumer seal and run the whole suite.

Intermediate commits remain buildable because the frozen facade stays present until all call sites
move. A failed cutover leaves the old route available; it does not authorize a mixed forwarding
shim. After 0377 merges, change 0370 remains run-halted until a human runs its explicit quiescent
resume operation. The resumed reconcile re-proves the merged consumer seal before deletion.

## Acceptance criteria

1. `docket repository prepare --json` implements the approved structured Step-0 contract.
2. Preparation attaches or fast-forwards only a healthy, clean metadata worktree and refuses every
   destructive or uncertain state without overwriting user data.
3. No flat shell-assignment or `eval` interface is introduced.
4. Every maintained facade consumer is derived and behaviorally classified.
5. Plain status uses `maintenance sweep` plus read-only `status`; no mutating status facade remains.
6. Routine Board-pass and direct-renderer invocations are absent from maintained workflows.
7. Board-visible record mutations include canonical board output atomically through one shared
   renderer/inclusion path.
8. Typed change/finalize operations own artifact-link rendering; typed ADR operations own ADR-index
   rendering.
9. Repository check reports deterministic board, artifact-link, ADR-index, and ADR-ledger defects
   with correct repairability.
10. Repository migrate previews and repairs every approved mechanical defect against an exact pinned
    revision and refuses ambiguous/authored repairs.
11. Stack consumers use typed context/finalize/maintenance capabilities without compatibility verbs
    or integration-branch fallback.
12. Canonical skills, references, agents, and maintained operator instructions use native structured
    commands.
13. Generated embedded/harness products match their canonical sources after repeat generation.
14. A shape-derived, mutation-tested guard proves no maintained executable consumer requires the
    migrated facade operations and fails on unknowns.
15. Frozen Bash implementation, parity/deletion tests, immutable history, and frozen release
    artifacts remain present and unchanged except for mechanically required test integration.
16. GitHub board mirroring, main mode, and deferred product features remain absent or unsupported as
    settled upstream.
17. Change 0370 remains run-halted and unimplemented; no branch, plan, PR, or deletion work for it is
    performed by 0377.
18. Long-running tests use the integration group, no individual test exceeds 60 seconds, and the
    complete Go-native suite passes with no authoritative serial budget breach.

## Risks and mitigations

- **Preparation becomes another facade:** keep one closed operation with typed repository context,
  no sub-operation routing, and no generic string map.
- **A command forgets the board:** use one shared inclusion path plus a mutation-tested structural
  invariant over board-authoritative writes.
- **Repair rewrites authored history:** classify only canonical deterministic bytes as repairable;
  refuse malformed markers and illegal evolution.
- **Stack behavior is accidentally restated:** expose domain results through context rather than
  reimplementing graph rules in CLI or skills.
- **Installed skill/binary skew:** land commands before call sites, regenerate from canonical
  sources, retain the old facade through the migration, and update both channels through the normal
  source install.
- **Guard passes vacuously:** derive sites by executable shape, fail unknowns, and mutation-test both
  forbidden sites and population removal.
- **377 expands into 370:** forbid physical deletion and legacy-suite contraction; 377 ends at a
  green zero-maintained-consumer seal.
