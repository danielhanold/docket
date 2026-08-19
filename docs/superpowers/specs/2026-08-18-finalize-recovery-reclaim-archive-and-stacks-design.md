<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0316 — Finalize, recovery, reclaim, archive, and stacks](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-19-0316-finalize-recovery-reclaim-archive-and-stacks.md)**
<!-- docket:backlink:end -->

# Finalize, recovery, reclaim, archive, and stacks

**Change:** 0316 · **Type:** feat · **Priority:** critical · **Date:** 2026-08-18 ·
**Status:** Focused design; waiting on declared bootstrap prerequisites

## Purpose and boundary

This change completes the terminal half of Docket's retained agent-first lifecycle. A supported
repository can take a verified `implemented` change through its current effective-base gate,
merge it exactly once, record only a merge that authoritative Git and GitHub facts prove, archive
the resulting terminal records, close a stack when its root reaches the integration branch, and
clean only Docket-owned resources. Interrupted runs resume from durable Git, GitHub, metadata,
workspace, gate, and ownership facts instead of a hidden workflow session.

The approved [Go migration program map](2026-08-12-go-migration-program-map.md) and
[architecture](2026-08-12-go-migration-architecture-design.md) are fixed upstream constraints.
This design does not reopen their one-binary, agent-first, one-shot JSON, transaction, external-
effect, compatibility, deferred-capability, packaging, or hard-cutover decisions.

Change 0316 starts from the verified PR and `implemented` record delivered by 0315, after 0322 and
0326 have completed the migration-host bootstrap and supported-config preconditions. It owns
finalize context and selection, the local rebase/retest composition, resolver and repair report
inputs, rewritten-head publication, verified merge, terminal metadata closeout, stack closeout,
explicit and automatic reclaim, persistent halted/finalize-blocked recovery, merged-PR maintenance,
terminal artifact-link repair, and ownership-safe cleanup. It does not rebuild behavior from
changes 0305 through 0315 or either bootstrap prerequisite. Changes 0317 and 0318 retain release
acceptance and self-hosting cutover.

## Independently deliverable result

The landed Go foundations are consumed as dependencies:

- `internal/config` already resolves both repository modes, `finalize.gate`, the suite command,
  approval policy, reclaim TTL/auto policy, and the Go-v1 unsupported-capability fence.
- `internal/domain` already owns the lifecycle vocabulary, `MarkStackedMerged`, `MarkDone`,
  `EvaluateReclaim`/`Reclaim`, dependency and stack graphs, and ADR-0092's effective-base rule.
- `internal/document`, `internal/render`, and `internal/repository/transaction` already own
  loss-preserving patches, marker balance, boards, artifact blocks/backlinks, exact-version
  mutations, semantic retry, full-snapshot validation, and expected-ref pushes.
- `internal/gitcli` already owns repository identity, exact refs and objects, ancestry, worktrees,
  changed paths, exact-path commits, and lease pushes.
- `internal/workspace` already owns long-lived feature-workspace manifests, exact inspection,
  non-forcing publication, clean worktree removal, and cleaned tombstones.
- `internal/githubcli` already owns typed PR identity/versioning and probe/act/verify publication.
- `internal/process` already owns durable gate launch/observe/stop/recover with exact terminal
  outcomes; `internal/evidence` owns exact-head green evidence.
- The 0315 application and skill path already owns context through `implemented`, including
  workspace preparation, evidence production, PR publication, and read-only run verification.

Change 0316's independently reviewable deliverable is:

- narrow Git and GitHub mechanics for an ownership-proven rebase, exact force-with-lease rewrite,
  child-PR retarget, expected-head merge, merge verification, and exact branch deletion;
- application/CLI checkpoints that sequence those effects without making Go an agent dispatcher or
  storing a central phase machine;
- atomic terminal transactions for ordinary, stacked, and stack-root outcomes in both repository
  modes, with repairable generated-link follow-up where two refs cannot commit atomically;
- explicit halt/resume/reclaim and maintenance operations whose destructive legs fail closed on
  absent proof or an errored probe;
- revised `docket-finalize-change`, resolver, repair, status, and implement-next assets that keep
  judgment and authored reports in Claude while Go owns the mechanics and postconditions;
- ownership-safe retention cleanup for terminal gate runs, feature workspaces, and branches; and
- hermetic failure-injection tests proving that a crash or lost response after every irreversible
  effect converges on retry without a false `done`, duplicate merge/comment, unsafe deletion, or
  stranded stack.

With those prerequisites done, this slice is independently useful before release packaging:
disposable repositories in both metadata modes can complete the retained claim-to-terminal
lifecycle through Go. Live four-harness and release-artifact acceptance remains 0317; the remaining
self-hosting, Bash-removal, publication, and hard-cutover proof remains 0318.

## Implementation launch precondition — declared and externally owned

Change 0316 does not bootstrap the runtime it needs. The migration program supplies that runtime in
three explicit stages:

1. Changes 0322 and 0326 are implemented and finalized through explicitly resolved skill and helper
   paths from an immutable `v0.9.2` checkout—not the current Go-based named implementer and not a
   tagged `install.sh` run that leaves existing skill links untouched. Change 0322 wires exact
   legacy user-level adoption and makes the source `install.sh` delegate to the Go development
   installer. Change 0326 disables the active deferred configuration requests that would otherwise
   require Go to return `unsupported-config`.
2. From a clean checkout at a reviewed `origin/main` commit containing both changes, the migration
   owner runs `go run ./cmd/docket development install --source <checkout>`. That transient process
   is authorized only to perform machine installation; it must not invoke a shared-state
   transaction command.
3. The installed PATH-resolved binary must pass `docket install check`, `command -v docket` must
   identify that installed binary rather than an ad-hoc `go install` output, and
   `docket diagnostic config --repo-dir <checkout> --for-mutation --json` must report mutation
   allowed. The host is then restarted so it reloads the installed agents and dispatch material.

Only after 0322 and 0326 are `done` and all three stages pass may `docket-implement-next 316` start.
It invokes the installed binary for `context`, `change`, `workspace`, `evidence`, `pr`, `run`, and
later `finalize`; it never substitutes `docket.sh` or a temporary build. A missing/ambiguous binary,
installation drift, active unsupported capability, failed host reload, or dependency still in
flight is a genuine precondition failure and aborts before claim, plan, branch, workspace,
metadata mutation, or PR creation.

The previously reported configuration blockers are corrected here precisely: repository-local
`.docket.local.yml` agent model/effort pins request the deferred per-repository routing capability;
global `~/.config/docket/config.yml` pins are supported and do not block. The other blockers are
repository-local `auto_capture.enabled` and committed `build.checkpoint`,
`finalize.skip_results_only_delta`, and `terminal_publish`. Change 0326 turns off only those active
requests and does not weaken the capability fence.

Unit and application tests may call Go package seams directly, while protocol and end-to-end tests
build an explicit temporary executable and run it only against disposable repositories with
hermetic supported configuration. Those product-under-test binaries still never control Docket's
own shared metadata.

## Chosen architecture

Claude remains the workflow controller. Go exposes small checkpoint operations whose promises are
independently observable:

```text
Go-v1 finalize asset under an explicit disposable-repository test host
  |
  +--> context finalize              revision-consistent change/PR/stack/gate facts
  +--> finalize retarget-children    attended override only; probe/act/verify each PR
  +--> finalize rebase               owned rebase at exact head onto exact effective base
  |      `--> rebase-continue/abort   resolver edits; Go owns Git state transitions
  +--> gate launch/observe            landed native local gate
  +--> evidence record                landed exact-head gate evidence
  +--> finalize publish               exact force-with-lease + existing-PR evidence update
  +--> finalize block/clear-block     durable human-needed state around the gate
  +--> finalize merge                 expected-head GitHub merge + authoritative reprobe
  +--> finalize closeout              verified terminal metadata transaction
  +--> finalize cleanup               terminal links, runs, workspaces, and exact refs
  `--> maintenance sweep              already-merged recovery + cleanup + policy reclaim
```

No operation interprets a child-agent return as proof. Resolver and repair agents author conflict
or diagnosis reports and may edit the feature workspace; Go verifies the live Git index, rebase
state, commits, gate record, remote refs, PR snapshot, and metadata before the next effect.

This is deliberately a set of resumable checkpoints rather than a central `finalize advance`
command. A central phase machine would duplicate the already-authoritative repository, PR,
workspace, gate, and metadata states, make response-loss recovery depend on a second cursor, and
move human sign-off and repair judgment into Go. The opposite alternative—letting skills run Git
and `gh` directly while Go only archives afterward—would leave the riskiest irreversible effects
outside the typed and tested boundary. The chosen middle keeps policy and prose agent-owned while
making every mechanical promise probeable.

## Package and command boundaries

Application orchestration composes landed packages inward:

```text
internal/cli  -->  internal/app
                       |
                       +--> domain + repository/transaction + document/render
                       +--> workspace --> gitcli
                       +--> process + evidence
                       `--> githubcli
```

- `internal/cli` parses typed flags/request files and presents one protocol-v1 result. It contains
  no lifecycle, Git, GitHub, stack, reclaim, or cleanup policy.
- `internal/app` pins authoritative context, checks cross-package preconditions, calls one named
  effect or metadata operation, and maps closed outcomes. It does not retain a workflow cursor or
  dispatch an agent.
- `internal/domain` receives typed branch/PR/merge facts and remains free of filesystems,
  documents, Git, GitHub, processes, and clocks except injected values.
- `internal/gitcli`, `internal/githubcli`, and `internal/workspace` gain only the named primitives
  required below. No generic command runner, checkout/reset/clean, arbitrary force push, arbitrary
  branch delete, or unversioned PR mutation is exported.
- Metadata writes stay in `repository/transaction.Engine` and patch only owned fields/sections and
  derived blocks. A cross-ref update on the integration ref uses the same isolated-worktree,
  explicit-path, request-receipt, and expected-ref principles; it is not a shared checkout edit.

The public operation names settled by this slice are shown below. The examples use `docket` as the
eventual executable name. Product protocol tests send the same argv to the explicitly built
temporary executable described above; they use neither a bare PATH lookup nor `docket.sh`
dispatch. The implementation host uses the installed PATH-resolved binary proven by the launch
precondition; temporary test binaries remain confined to disposable repositories.

```text
docket context finalize [--id <change-id>] [--allowlist <id,id,...>]

docket finalize retarget-children --id <id> --version <entity-version> --input <request-file>
docket finalize rebase --id <id> --version <entity-version> --head <oid>
docket finalize rebase-continue --id <id> --attempt <token> --input <report-file>
docket finalize rebase-abort --id <id> --attempt <token> --input <report-file>
docket finalize publish --id <id> --attempt <token> --head <oid> --evidence <record-file>
docket finalize block --id <id> --version <entity-version> --input <report-file>
docket finalize clear-block --id <id> --version <entity-version> --head <oid>
docket finalize merge --id <id> --version <entity-version> --head <oid> [--admin]
docket finalize closeout --id <id>
docket finalize cleanup --id <id>

docket change halt --id <id> --version <entity-version> --input <report-file>
docket change resume-halted --id <id> --version <entity-version> --acknowledge-quiescent
docket change reclaim --id <id> --version <entity-version>

docket maintenance sweep
docket gate cleanup <absolute-run-dir>
```

Scalar identities travel as flags. Authored conflict, repair, halt, block, or retarget authorization
travels through a bounded request file or stdin, never command-line interpolation. Exact flag
grouping may follow the landed Cobra patterns, but these operation meanings remain stable for the
shipped skills.

## Authoritative finalize context and selection

`context finalize` is read-only. It pins one metadata revision, builds one complete snapshot, and
then probes Git/GitHub with explicit repository and branch identities derived from that snapshot.
For each in-scope change it returns at least:

- canonical change path, exact bytes, entity version, status, priority/date, branch, PR, plan,
  optional results, and finalize/run marker state;
- the resolved effective-base outcome, exact remote base ref/head, current local/remote feature
  heads, workspace ownership/cleanliness, and any existing owned rebase state;
- the PR number, version, state, draft/approval/mergeability facts, exact head/base, diff size,
  body evidence verdict, merged timestamp, merge commit, and current comments needed for recovery;
- dependencies plus direct and transitive stack relationships, each descendant's lifecycle and PR
  destination, and the complete current set of open child PRs;
- resolved local/off gate policy, suite command, observation budget, approval policy, reclaim
  policy, repository mode, metadata/integration refs, and capability notices; and
- a typed candidate/skip reason rather than an omitted or guessed fact.

An explicit `--id` inspects exactly that record. Naming an id is the human merge authorization and
therefore overrides `finalize.require_pr_approval` and the auto-detect skip for an existing
`## Finalize blocked` section; it does not override malformed state, wrong PR identity, an unsafe
stack, an unprovable effect, or the separate sign-off required after an authored repair.

Without an id, selection retains one merge per invocation. Already-merged PRs are closeout work,
not merge candidates, and may be recovered directly. Open candidates are ordered by dependency
eligibility, GitHub `MERGEABLE` before `CONFLICTING`/still-unknown, smaller changed-file and line
counts, then priority, creation date, and id. `CONFLICTING` reaches the rebase resolver; draft,
closed, approval-blocked, malformed, and already finalize-blocked candidates are surfaced with
stable reasons. An allowlist bounds the same selection without changing its ordering.

GitHub probe errors are `unknown`, never clean absence. A read-only context may report capability
warnings, but every later mutation reruns capability preflight and its own authoritative checks.

## Open-child gate and retargeting

The child set comes from `domain.StackChildren`/`StackDescendantsParentFirst` over the current
snapshot, never the parent's rendered artifact table. Before merging a parent, finalize reprobes
every open child's PR and applies the retained gate:

- an autonomous run with any open child halts and records the exact child list;
- an attended run may proceed only after the human explicitly authorizes the exact versioned set;
- authorization for one set does not cover a child added concurrently; and
- a child already `stacked-merged` or `done` does not block.

`finalize retarget-children` accepts the exact authorized child IDs and PR versions from context.
It resolves the parent's own effective base and probe/act/verifies each open child PR onto that
branch before the parent merge. It never relies on GitHub's branch-deletion behavior. A retry
adopts an already-retargeted exact PR; a new child, changed PR version, ambiguous PR, or failed
probe returns `contended`/`unknown` and no parent merge may start.

The operation changes no `stacked_on:` field. That field continues to describe where the child was
designed; ADR-0092 and the parent's later lifecycle state determine the child's next effective base.

## Rebase and local gate

For supported `finalize.gate: local`, `finalize rebase` performs these checks before Git mutation:

1. reload current metadata and require the exact `implemented` record/version and verified PR;
2. resolve the effective base unconditionally and fetch its exact remote head;
3. inspect the manifest-owned feature workspace and require the expected clean local/remote/PR
   feature head plus the PR's exact base;
4. create an ownership-scoped rebase receipt keyed by repository identity, change id, original
   head, original remote head, base ref, and base head; and
5. rebase the feature branch onto that exact base head through a named Git operation.

The receipt lives with the existing workspace ownership state. It is an effect receipt, not a
workflow phase machine: its only purpose is to prove which rewrite may later be continued,
aborted, or force-with-lease published. Owned temporary refs retain the original and base commits
across interruption. Live Git rebase state plus those refs and the manifest are authoritative; a
pathname, branch name, reflog message, or child report alone is not.

Closed rebase dispositions are `unchanged`, `rebased`, `conflicted`, `contended`, `blocked`, and
`failed`. A response-lost success is recovered by inspecting the exact owned receipt, temporary
refs, workspace branch/head, ancestry, and absence of an in-progress rebase. A foreign/malformed
rebase, dirty worktree, moved base, changed remote feature head, or errored probe is retained and
blocked, never reset or adopted.

On conflict, the resolver may inspect and edit only the returned feature workspace. It does not
run `git add`, `rebase --continue`, `rebase --abort`, push, or tests directly. Each
`rebase-continue` request supplies a structured resolver report and the exact attempt token. Go
validates the reported repository-relative paths, compares them with live unmerged/index state,
stages only those explicit paths, continues non-interactively, and returns the next conflict or a
verified completion. An ambiguous report routes through `rebase-abort`, which proves the owned
attempt, aborts, and verifies restoration to the recorded original head before finalize is marked
blocked.

After a completed rebase:

- if the rebase was a no-op and the existing PR body carries parseable green evidence for the
  exact current head, the local suite may be skipped with that exact permit named in the result;
- otherwise the full suite command already resolved by the 0315/build path is launched and
  observed through the native gate, in the feature workspace, within the observation budget;
- a passed terminal run produces evidence only through the landed `evidence record` operation;
- failed is repair work; running at budget exhaustion, signaled, stopped, vanished, malformed, or
  unavailable is a halt, not a fabricated red result; and
- the deferred results-only strict-ancestor exemption is not implemented. Any evidence head other
  than the exact current head runs the suite.

`finalize.gate: off` retains its explicit opt-out meaning: no rebase and no local retest. The merge
operation still verifies current metadata, workspace/remote/PR identity, destination, stack gate,
and the merge result. `ci` and `both` remain deferred capability requests and fail before mutation,
as settled by 0305.

## Conflict and repair reports

Resolver and repair returns are authored inputs, not authority. Their versioned JSON envelope
contains the change and attempt IDs, disposition, bounded technical summary, touched/conflicted
paths, observed head/base, and a recommended human action. A repair report additionally names the
commits it claims and whether it is `repaired` or `stuck`.

Go verifies every mechanical claim:

- a resolved conflict means no unmerged entry remains after the owned continue operation;
- a repair commit must be on the exact owned feature branch, descend from the rebased head, and
  match the live changed-path set;
- every repair changes `HEAD`, invalidates old evidence, and requires the whole local gate again;
- the repair agent gets at most two attempts per finalize invocation, enforced by the skill; and
- no report can authorize merge, ref deletion, metadata transition, or inline substitution for a
  missing resolver/repair dispatch.

The existing resolver/repair split remains: the rebase resolver owns conflicts until rebase
completion and never tests; the integration repair agent owns a red post-rebase suite and authors
the minimal fix. Dispatch unavailability follows the existing carve-out and halts. Pure conflict
resolution does not introduce the separate repair sign-off. An authored repair does: an autonomous
finalize publishes the verified repaired head/evidence, records `repair-needs-signoff`, and stops;
an attended finalize asks the human before merge.

## Rewritten-head publication and PR evidence

The landed `workspace publish` intentionally refuses rewritten history, so finalize adds a narrow
rewrite operation instead of weakening that general service.

`finalize publish` consumes the owned rebase receipt, exact current clean workspace head, original
remote head, and canonical evidence bytes. It:

1. probes the remote feature ref;
2. returns no-op if it already equals the intended head;
3. otherwise pushes exactly that head under `--force-with-lease` against the receipt's exact old
   remote head—never an unqualified force;
4. reprobes until it proves equality, observes a different head as contention, or returns unknown;
5. reprobes the existing PR by number/head/base and expected PR version;
6. loss-preservingly replaces only the Docket build-evidence block with the exact current-head
   green record; and
7. probe/act/verifies the updated PR snapshot without creating another PR.

A crash after the push but before the PR update is an ordinary replay: the remote-head probe adopts
the rewrite, then the PR update resumes. A crash after the PR update adopts both promises. Unknown
never authorizes a second mutation or merge. Authored PR prose, title, reviews, comments, and every
other body byte remain untouched.

## Persistent halt and finalize-blocked state

`finalize block` leaves the PR open and the change `implemented`. It first ensures exactly one
versioned PR comment carrying a Docket-owned attempt marker and the bounded authored report, then
upserts one bare `## Finalize blocked` section in an exact-version metadata transaction. The
section records UTC date, stable reason, attempt identity, verified Git/GitHub facts, comment URL,
and concrete remedy. A re-mark replaces the section interior or appends an attempt inside it; it
never creates a second heading. Board output changes atomically with the marker.

The comment effect is idempotent by its owned attempt marker. A crash after commenting but before
metadata reuses the comment and finishes the marker transaction. A comment probe failure is
unknown and does not write a marker claiming the comment exists.

Auto-detect skips an unmerged marked change. An explicit id is the human retry signal. The marker
is cleared only by `finalize clear-block`, which reprobes an exact current head, valid local-gate
evidence (unless gate is off), published remote ref, and matching open PR before transactionally
removing the section and rerendering the board. If a PR was merged out of band, closeout may archive
the historical marker unchanged; no active reader remains for it.

Implementation-run halts use a separate `## Run halted` section because the source state and
recovery are different. `change halt` upserts one bounded authored report on an `in-progress`
change without clearing its branch, lease, workspace, or evidence. `run verify` gains the closed
`run-halted` verdict. `context implementation --id` reports its durable checkpoints.

`change resume-halted` is human-authorized recovery of the same work, not a second claim. It
requires the exact marked record and explicit acknowledgement that the prior worker is quiescent,
reprobes the owned branch/workspace and any live gate state, refreshes the claim, removes the marker,
and preserves every valid checkpoint. It never discards, resets, adopts, or reassigns a workspace
whose writer may still be live. A fresh claim after reclaim continues to use the landed claim
action's owned marker removal.

## Merge and authoritative verification

`finalize merge` rechecks all merge conjuncts immediately before the external effect:

- the exact change remains `implemented` with the same canonical PR;
- local workspace, remote feature ref, PR head, and requested head agree;
- the PR is open, non-draft, and targets the freshly resolved effective base;
- local gate policy is satisfied by exact evidence or the explicit off policy;
- approval policy is satisfied for auto-detect, or an explicit id supplied human authorization;
- no unretargeted open child PR exists; and
- no newer finalize-blocked reason or metadata version superseded the request.

It invokes the GitHub merge operation with merge-commit semantics and an exact expected head. It
never requests branch deletion. `--admin` is accepted only on an attended, explicitly named run as
separate human authorization; it is never inferred from an explicit id, approval absence, or a
permission error.

After any success, timeout, cancellation, or lost response, Go reprobes the PR. A verified merge
result contains the PR number/version, exact head and base at merge, GitHub `mergedAt` in UTC, and
the merge commit object ID. The Git adapter fetches the destination and proves the reported merge
commit reachable from its current remote tip. An open PR is not merged; a differently headed or
based merge is contended/invalid; an unobservable result is unknown. None permits closeout.

An already-merged exact PR is a no-op verified result, whether the merge was performed by this run,
a prior response-lost run, or a human. Docket never attempts compensating rollback of a merge.

## Terminal metadata transaction and archive

`finalize closeout --id` takes no caller-supplied `done` boolean or archive date. It reloads
metadata, reprobes the recorded PR and destination ref, and derives the UTC archive date from the
verified GitHub `mergedAt`.

For an ordinary change whose verified merge destination is the integration branch, one metadata
transaction:

1. applies `domain.MarkDone` only after the merge-commit reachability proof;
2. stamps `updated:` from the UTC merge date and clears the claim while preserving historical
   branch/PR/artifact fields;
3. relocates the record from `active/` to the canonical dated archive path;
4. rerenders its artifact block and metadata-resident spec backlink to that archive path;
5. rerenders the inline board; and
6. validates, commits explicit paths, and pushes all metadata outputs atomically.

The archive move reuses the relocation/file-plan machinery already exercised by 0312's kill path;
it does not copy that implementation into a second permissive writer. A renderer, marker,
validation, lease, or push failure produces no remotely partial metadata outcome. A response-lost
success is a no-op when the canonical archive record and transaction receipt already exist.

Terminal publishing is not part of this flow. No archived change, spec, or ADR is copied onto the
integration branch, and no `## Publish deferred` marker is created. Automatic learning harvest,
index refresh, promotion, and follow-up capture are likewise absent.

## Stack merge and root closeout

The governing invariant remains: `done` means the change's code is reachable from the integration
branch.

If an implemented stacked change's verified PR destination is its live parent's branch,
`finalize closeout` applies `MarkStackedMerged` in place, clears any stale successful-gate marker,
and atomically rerenders the board. It does not archive, delete the feature branch, clean the
workspace, or refresh terminal backlinks. A stacked PR retargeted and merged directly into the
integration branch takes the ordinary `done` path instead.

When a stack root reaches the integration branch, closeout derives the full descendant set from
the authoritative graph and verifies the chain of merged PR destinations that carried every
`stacked-merged` descendant into the root. One metadata transaction then marks and archives the
root plus every proven carried descendant, using the root's verified UTC merge date for every
archive filename, rerenders each artifact/spec backlink, and renders one board over the final
population. No descendant is promoted from a rendered table or from a branch name alone.

The transaction is all-or-nothing for the proven closeout set. If a required PR/destination fact is
unknown or contradictory, the already-merged root remains recoverable rather than writing false
descendant `done` records. Maintenance can also invoke the same operation for an archived root with
active `stacked-merged` descendants, so a legacy or externally interrupted closeout has an explicit
repair path. Per-descendant terminal link/cleanup failures occur after the metadata transaction and
remain independently retryable; they do not roll terminal state backward.

## Terminal artifact backlinks across repository modes

The terminal record moves from `active/` to `archive/`, so generated backlinks in linked artifacts
must point to the new canonical path.

- In `main` mode, the change, spec, plan, results, and board share one ref; the closeout transaction
  can include every affected managed block atomically.
- In `docket` mode, the archived record, spec, and board live on the metadata ref while merged plan
  and results live on the integration ref. The metadata transaction lands first. A follow-up
  isolated integration-ref operation patches only the existing `docket:backlink` blocks in the
  verified plan/results paths, commits explicit paths with a stable closeout request trailer, and
  pushes with an exact expected-ref lease.

The second leg is generated-link maintenance, not terminal publishing: it copies no metadata
record and edits no authored bytes. A lost response is recovered from the remote block bytes and
request trailer. A failed or contended leg leaves the change truthfully `done` and emits a typed
`terminal-backlink-pending` health/maintenance finding; the explicit cleanup/sweep path retries it.
The operation never hand-edits a merged artifact's authored content.

## Reclaim

`change reclaim` is an exact-version metadata operation over one `in-progress` record. It uses the
configured TTL and injected UTC clock, and requires all of the following:

- `EvaluateLease` reports strictly expired; missing, empty, malformed, future, exactly-at-boundary,
  and non-in-progress stamps are not expiry;
- both the recorded and conventional feature branch are cleanly absent locally and remotely;
- no Docket-owned workspace is allocating/ready/dirty/ambiguous for the change;
- no owned gate run or other live ownership record indicates work still exists; and
- every Git/workspace/process probe succeeded. Unknown never shares the absent branch.

Only then does the transaction apply the landed `Reclaim` action, append one dated
`## Reclaim log` entry with the previous claim and proof summary, return the change to `proposed`,
clear branch/claim, set `reconciled: false`, rerender links/board, validate, commit, and push.

Explicit reclaim is available regardless of `reclaim.auto`. `maintenance sweep` attempts it only
when the resolved auto policy is true. A live branch, workspace, gate, missing/malformed lease, or
probe error is a stable skipped reason and causes no cleanup, delete, reset, or marker removal.

## Merged-PR maintenance sweep

`docket status` remains read-only. Mutation is explicit through `docket maintenance sweep`; the
status agent may invoke that operation before its later read.

One sweep pins an initial inventory and processes items in deterministic order, reloading fresh
authority before every mutation. It:

1. finds active `implemented` changes whose recorded PRs are already merged;
2. closes stacked children before ancestors, then invokes the same verified `finalize closeout`
   operation so a root can carry all descendants;
3. retries terminal backlink repair and ownership-safe cleanup for archived/done records and
   completed stacks;
4. invokes explicit reclaim for eligible records only when `reclaim.auto` is true; and
5. reports every applied, no-op, contended, blocked, unknown, and failed item as structured sorted
   entries rather than collapsing the run to one Boolean.

The sweep never merges an open PR, overrides approval, retargets children without human
authorization, repairs code, edits authored results/learnings, or treats a finalize marker as a
reason to ignore a PR already merged out of band. Per-item failures do not stop independent items,
but a destructive suffix for that item never runs after an unknown prerequisite.

Transaction-engine abandoned-worktree pruning and native `gate recover` keep their landed
semantics and commands. Maintenance may compose their reported inventories, but this change does
not duplicate or broaden their ownership rules.

## Ownership-safe cleanup and run retention

Cleanup begins only after durable closeout, except an explicitly aborted owned rebase restoring
its own original head. It is a retryable suffix, never evidence that merge/archive succeeded.

For each terminal change, `finalize cleanup`:

1. reloads the archived/stacked state and verified merge destination;
2. repairs terminal backlinks first when needed;
3. calls the landed clean workspace removal using the existing ownership manifest facts, not a
   base recomputed from the now-terminal record;
4. deletes the local feature ref only when the exact recorded tip is detached from every worktree
   and the verified merge chain contains it;
5. deletes the remote feature ref only under an exact old-value lease and only after a fresh probe
   proves no open child PR still targets it; and
6. retains the cleaned tombstone for replay and health attribution.

`stacked-merged` changes retain workspace and branches until their root reaches the integration
branch. Root cleanup handles every carried descendant independently after the atomic stack
closeout. An out-of-band parent merge with unretargeted children archives truthfully but retains the
parent branch and reports `children-retarget-required`; uncertainty costs a stale branch, never a
closed child PR or lost review history.

Probe failure, dirty/untracked/conflicted bytes, moved refs, malformed/foreign manifests, live
locks, unproven ancestry, or an exact-lease rejection returns cleanup pending and preserves the
resource. No operation calls global worktree prune, force-removes a checkout, recursively deletes
by pathname, or touches the primary, metadata, transaction, sibling feature, or foreign worktree.

`docket gate cleanup` adds the retention policy deliberately left to this change by 0314. It
removes one exact private run directory only after validating ownership, a terminal record, no live
lock/group, and either durable exact-head evidence or a persisted halt/finalize report that no
longer needs the logs. Failed, vanished, ambiguous, or unreported diagnostic runs are retained.
Finalize cleanup calls it for successful closed gates; maintenance retries safe pending cases.

## Recovery and idempotency matrix

Each irreversible boundary is ordered probe → act → verify → record, with a specific replay:

| Interrupted after | Durable authority on retry | Recovery |
|---|---|---|
| Rebase starts/conflicts | workspace manifest, owned refs, live Git rebase state | return same attempt/conflict; continue or verified abort |
| Rebase completes | owned receipt/refs plus exact workspace head/base | adopt completed rewrite; never rebase a different head |
| Force-with-lease push | authoritative remote feature ref | adopt exact head or report contention/unknown |
| PR evidence update | versioned PR body and exact head/base | adopt exact managed block; never recreate PR |
| Child PR retarget | versioned child PR base | adopt each exact retarget; block on changed set/version |
| PR merge | GitHub merged snapshot plus reachable merge commit | return verified already-merged; never merge twice |
| Metadata closeout push | canonical archive/stack state plus transaction receipt | return no-op or resume same semantic operation |
| Integration backlink push | remote managed blocks plus request trailer | adopt or retry exact generated-only patch |
| Worktree removal | workspace tombstone plus Git registration | return already-clean; retain on unknown |
| Local/remote ref delete | exact local/remote ref probes and deletion lease | return already-absent only on clean absence |
| Gate-run directory delete | absence plus owned cleanup receipt/tombstone | return no-op; foreign absence is not adopted |

No retry keys success on a clean working tree, a cached context, a process exit alone, a branch
name, a local commit, a child completion message, or the existence of a metadata field. The probe
always asks the authority that owns the promised postcondition.

## Skill and embedded-asset revisions

This change authors and embeds only the Go-v1 workflow assets needed for the retained terminal
path. They are exercised explicitly as product-under-test inputs; their source bytes are not
authority to replace the installed, verified implementation-host binary:

- `docket-finalize-change` becomes the Claude-owned sequencer for the operations and sign-off rules
  in this spec, retaining one merge per invocation and its `advanced`/`contended`/`drained`/`halted`
  driver contract.
- `docket-rebase-resolver` edits conflicts and returns the structured report; it never runs Git
  rebase mechanics or tests.
- `docket-integration-repair` diagnoses a red rebased suite, authors at most the bounded repair,
  and returns the structured report; Go and the controller verify and rerun the gate.
- `docket-status` explicitly invokes maintenance when requested, then reads status; the human
  `docket status` command remains read-only.
- `docket-implement-next` and `run verify` gain the persistent halt/resume path without changing
  the already-landed claim-to-implemented checkpoints.
- Focused references, command manifests, agent templates, and embedded assets are regenerated
  together so the installed protocol is internally consistent. Bootstrap installation and
  self-hosting authority remain outside this change.

Go v1 uses the fixed approved roles. The change does not add skill rebinding, per-repository model
routing, cross-harness delegation, inline resolver/repair substitution, autonomous capture, or
learning harvest.

## Failure, concurrency, and security rules

- Every metadata operation performs capability preflight, reloads fresh origin, and submits exact
  entity versions. A semantic retry reapplies intent only while its preconditions still hold.
- Git/GitHub effects use explicit repository, ref, PR, head, base, and expected-version identities.
  `unknown` never becomes permission for merge, overwrite, create, retarget, or delete.
- The rebase rewrite may force only through the exact old-value lease recorded by the owned
  attempt. No other operation gains a force-push primitive.
- Authored reports are bounded valid UTF-8, cross the CLI through files/stdin, and never enter Git
  or `gh` argument strings. Returned diagnostics redact report bodies, credentials, environments,
  credential-bearing URLs, and unbounded stderr/logs.
- Marker-bound blocks and sections validate whole-population order/balance before replacement.
  Unknown frontmatter and unowned authored bytes remain byte-identical.
- Cleanup treats present, cleanly absent, and unknown as three outcomes. Only cleanly absent can
  certify an already-completed destructive suffix.
- The decision and action use the same authoritative copy: a gate over metadata/ref/PR bytes never
  inspects one checkout and acts on another ref.
- A merge is never rolled back. Post-merge failures leave a verified recoverable closeout; cleanup
  and generated-link repair stay independent suffixes.
- Child-agent returns are hints. Missing/ambiguous dispatch follows the existing resolver/repair
  carve-out, records durable human-needed state, and never authorizes inline self-approval.

## Testing strategy

### Pure domain and application tests

- Finalize eligibility/ordering covers dependency order, mergeability bands, diff tiebreaks,
  approval policy, explicit-id override, finalize markers, and deterministic nil-safe results.
- Merge/closeout tests cover ordinary, stacked-parent, direct-integration retarget, root carry,
  missing/cyclic/killed stacks, destination mismatch, archive dates from `mergedAt`, and every
  legal/illegal source status.
- Halt/resume/reclaim tests cover exact TTL boundaries, missing/malformed claims, worker-quiescence
  acknowledgement, marker replacement/removal, and every ownership/probe refusal.
- Every command emits exactly one protocol-v1 document with closed result/disposition/reason fields;
  no automation parses prose or process exit codes for semantic verdicts.

### Real-Git and transaction tests

Disposable repositories with bare remotes cover both `main` and `docket` metadata modes, ordinary
and multi-level stacks, and independently mutable feature/metadata/integration refs. Tests inject
interruption before and after every rebase, continue, abort, lease push, archive push, backlink
push, worktree removal, and branch deletion boundary.

The matrix proves exact-old rewrite leases, concurrent base/head movement, same-entity contention,
response-loss receipts, atomic multi-record stack closeout, byte preservation outside owned
patches, and absence of writes to primary/metadata/transaction/sibling indexes. Each cleanup probe
has an injected-error test that proves the resource is retained.

### Protocol-faithful GitHub tests

An executable fake receives production argv/stdin/environment and models current nested GitHub
JSON for PR view/list/edit/comment/merge. It covers lazy/unknown mergeability, approval/draft/state,
child retarget, exact version contention, comment deduplication, expected-head merge, mergedAt and
merge-commit verification, response loss, auth/permission denial, timeout, malformed output, and
unknown post-effect state. No ordinary test creates or merges a maintainer-repository PR.

### Gate, repair, and retention tests

Real supervised processes cover passed, failed, signaled, stopped, vanished, budget-expired, and
malformed states. Exact-evidence/no-op-rebase skip is positive- and negative-tested; a moved head,
real rebase, stale evidence, or any strict-ancestor-only evidence runs the full suite. Resolver
fixtures create real multi-conflict rebases, and repair fixtures prove every repair invalidates
evidence and requires a fresh gate. Gate cleanup retains every live, failed-without-report,
ambiguous, foreign, and probe-error case.

### End-to-end and mutation tests

The end-to-end matrix builds `./cmd/docket` to an explicit temporary path, isolates global and
repository-local configuration, starts from 0315's verified `implemented` state in a disposable
repository, and proves:

- ordinary local-gate finalize to archive and cleanup in both repository modes;
- rebase conflict resolution, red-suite repair/sign-off halt, retry, merge, and closeout;
- response loss after each external effect converging without duplicates or false state;
- child merge to `stacked-merged`, root merge carrying all descendants to `done`, and safe branch
  retention/retargeting with open children;
- out-of-band merge recovered by maintenance; and
- expired no-work reclaim plus persistent implementation halt/resume.

A separate negative fixture loads the capability requests that blocked Docket before 0326—
repository-local `agents.*`, `auto_capture.enabled`, `build.checkpoint`,
`finalize.skip_results_only_delta`, and `terminal_publish`—and proves every mutating 0316 operation
returns `unsupported-config` before a metadata, Git, GitHub, workspace, gate, or cleanup effect.
The test invokes the temporary binary by absolute path and also establishes that no test depends on
a `docket` executable being present on `PATH` or on unsupported verbs in `docket.sh`.

Every safety guard has a mutation that removes or corrupts its premise and observes the operation
refuse before mutation. The build runs the whole resolved suite and treats every `OVER BUDGET:`
line as a finding requiring action.

Live release binaries and direct Claude/Codex/Cursor/OpenCode acceptance remain 0317. Docket's own
full self-hosting proof, remaining configuration cleanup, production-Bash removal, and cutover
rehearsal remain 0318.

## Explicit ownership exclusions

This change does not:

- reimplement configuration, document parsing, domain snapshot/graph basics, Git discovery/object
  reads, metadata transaction semantics, status rendering, installation, planning mutations,
  workspace/PR/evidence foundations, native process supervision, or claim-to-implemented workflow
  behavior owned by changes 0305 through 0315;
- add CI or combined finalize gates, results-only gate skipping, terminal publishing, automatic
  learning harvest/index/promotion, auto-capture, auto-groom, build checkpoints, GitHub backlog
  mirroring, per-repository routing, skill rebinding, cross-harness delegation, or a Bash fallback;
- add release archives, checksums, downloader/upgrade work, or live four-harness acceptance from
  0317;
- implement 0322's source bootstrap/legacy adoption or 0326's early configuration contraction;
- make Go self-hosting authoritative, remove production Bash/tests, publish the release, or perform
  hard cutover from 0318;
- install or activate the migration host's Go `docket`, teach `docket.sh` the Go command tree,
  mutate Docket's live metadata with a source-built binary, or bypass the unsupported-capability
  refusal;
- introduce a daemon, database, generic workflow cursor, public Go API, arbitrary Git/`gh` runner,
  force-push/delete escape hatch, automatic merge rollback, or new lifecycle status; or
- edit merged plan/results authored content, infer stack state from rendered links, or mark a
  change `done` from PR prose, a branch name, local time, or an unverified merge response.

## Acceptance criteria

1. No implementation run starts until 0322 and 0326 are `done`, the reviewed-source development
   bootstrap has installed the PATH-resolved binary, `install check` and the four-layer mutation
   diagnostic pass, and the host has reloaded the installed assets. The bootstrap process itself
   never runs shared-state transaction commands.
2. An explicitly built temporary Go-v1 executable, invoked by absolute path against a disposable
   supported-configuration repository, can take one verified `implemented` change through the
   configured local/off gate, exact merge, terminal metadata transaction, generated-link repair,
   and owned cleanup without direct skill-owned Git, GitHub, metadata, or process mechanics.
3. Resolver and repair agents retain authorship and judgment, while every Git/index/head/gate/PR
   claim they return is mechanically verified before publication or merge.
4. Every irreversible effect has a postcondition probe and replay path; interruption or response
   loss after any boundary converges without duplicate PR comments/merges, false `done`, unsafe ref
   overwrite/delete, or hidden phase state.
5. Ordinary, stacked, retargeted, and root-closeout outcomes preserve the invariant that only code
   reachable from the integration branch is `done`; carried descendants close atomically and open
   child PRs are never stranded by branch deletion.
6. Explicit reclaim and maintenance reclaim mutate only a strictly expired, branchless,
   workspace-less, gate-less claim whose absence probes all succeeded; unknown state is retained.
7. Persistent `Run halted` and `Finalize blocked` records make human-needed recovery durable,
   idempotent, and explicitly resumable without losing or racing an existing workspace.
8. Both repository modes pass the real-Git claim-to-terminal, crash/retry, stack, maintenance,
   backlink, and cleanup matrices under the whole resolved suite.
9. With a pre-0326 negative configuration fixture, that same temporary executable names the
   repository-local `agents.*`, `auto_capture.enabled`, `build.checkpoint`,
   `finalize.skip_results_only_delta`, and `terminal_publish` blockers and performs no mutation;
   global model/effort pins remain supported, and no product test requires `docket.sh` to implement
   Go verbs.
10. The implementation contains no behavior allocated to changes 0305–0315, 0317–0318, 0322, or
   0326 and does not reintroduce any capability the approved Go v1 architecture deferred or
   dropped.
