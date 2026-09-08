---
name: docket-finalize-change
description: Use when a change's PR is approved or merged and you want to close it out to done promptly rather than waiting for the safety-net sweep — merging if approved, verifying the merge landed, archiving the change, cleaning up its branch and worktree, and refreshing the board. The human's closing bookend; mirrors docket-new-change.
---

# docket-finalize-change — close out a change (Claude-owned sequencer)

## Overview

`docket-finalize-change` drives a verified `implemented` change through its terminal half: it reads one authoritative finalize context, retargets any authorized open children, rebases the feature branch onto its current effective base through the resolver/repair loop, runs the local gate, publishes the rebased head with its evidence, merges the PR exactly once against an authoritative verification, archives the terminal records, and cleans only Docket-owned resources. The skill is the **workflow controller**: every mechanical effect is one named Docket operation (its argv resolved from the capability catalog) that reloads fresh authority, submits exact identities, and returns one protocol-v1 document with a closed disposition. The skill never merges, rebases, deletes, force-pushes, or writes metadata by hand — it sequences the operations and keys on their tokens.

**Closeout notes ride the invocation, not a pause.** The caller may include already-known
verification outcomes or late findings in the finalize request; step 9 routes them into the
closeout operation's structured request. The skill never pauses after merge and never asks a new
mid-run question — it records context supplied when finalize was invoked, nothing more. Post-merge
observations belong in the terminal change record's `## Closeout notes` section, never appended to
the frozen merged `results:` file.

## When to use

- A PR was approved (merge + close out in one step), or was merged out of band and you want it archived — with branch/worktree cleanup and a board refresh — now rather than at the next `maintenance.sweep`.

## Convention (load first — blocking)

Invoke `docket-convention` first (unless already loaded this session) and follow its **Step-0 preamble (every operating skill)**: load the convention, run the capability bootstrap, then run the `repository.prepare` operation with `--repo-dir <dir> --json` as its own Bash call and validate the protocol-v1 envelope, carrying its typed context values forward as literals (it resolves config, enforces the bootstrap verdict fail-closed, and ensures + syncs the metadata working tree). Everything below uses its vocabulary — build-ready, entity version, effective base, integration branch, terminal transition — without redefinition.

## How every operation is invoked

Each effect is one named operation — its argv resolved from the capability catalog, never hard-coded — that emits exactly one protocol-v1 JSON document; automation keys on the document's closed `result`/`disposition`/`reason` fields, **never** the process exit code and never prose. Scalar identities (`--id`, `--version`, `--head`, `--attempt`, `--pr-number`) ride on flags; the exact `--version` is always the opaque entity version the authoritative context read returned for that record. Authored content — a resolver report, a repair's evidence, an authored block report — crosses the CLI through a request file or stdin (`--input`, `--evidence`), never a command-line argument and never into a Git or `gh` argument string. Read a returned `unknown` as "state unobservable, retain": it never authorizes a merge, overwrite, create, retarget, or delete. Read a `contended` as a lost race the next context read resolves. Read an `unsupported-config` as a repo requesting a deferred capability: every mutating operation returns it before any effect, and the run stops with `halted` naming the blockers.

## Selection — from the authoritative context

Read the candidate set once with the `context.finalize` operation (read-only; no metadata write, no Git mutation):

- **Explicit id** — the `context.finalize` operation with `--id <id>`. Inspects exactly that record even when it carries a skip reason, reporting it as a candidate with an `override_note`. A named id **is** the human authorization: it overrides the `approval-required` and `finalize-blocked` skip reasons (an explicit id is the "I looked at it, merge/retry" signal). It never overrides a real blocker — `malformed`, `pr-closed`, `dependency-unmerged`, `not-implemented`, `pr-unknown`, `draft` — each of which surfaces as its own closed skip-reason token and yields `halted`, never a forced merge.
- **Id allowlist** — the `context.finalize` operation with `--allowlist <ids>` bounds membership without reordering the survivors; naming the ids is the same authorization a single explicit id carries. A scoped id that is not eligible is surfaced with its skip reason, never force-merged.
- **Auto-detect** — the `context.finalize` operation with neither flag applies the selection policy. `context.finalize` returns candidates already ordered by `SelectFinalizeQueue` — merged-recovery first (closeout work with no merge to perform), then dependency-eligible open PRs, `MERGEABLE` before `CONFLICTING`/`UNKNOWN`, then smaller changed-files and diff lines, then priority/created/id. Take the head. Every skipped candidate is surfaced with its closed skip-reason token — nothing is omitted or guessed.

**Re-selection replaces sequencing.** Each invocation re-reads `context.finalize` against the current integration tip, so no precomputed order goes stale — every merge moves the base.

From the chosen candidate, carry forward: the exact `version`, the verified feature `head`, the resolved effective base, the canonical PR number, the descendant relations with each child's lifecycle and PR destination, the open-child PR set, and the resolved gate/approval/repo-mode policy. No later step re-reads the change file — the context bundle is the single authority every operation keys on.

## Terminal disposition (driver contract)

Every run ends by declaring exactly **one** of four dispositions — the **same four words** `docket-implement-next` uses, so one driver keys on both skills without knowing which it is driving:

| Disposition | Meaning | Driver action |
|---|---|---|
| `advanced` | A close-out advanced — this run merged a change and closed it out, **or** it archived an already-merged PR (real close-out work ran, just no merge by this run). | continue |
| `contended` | Another writer got there first — a concurrent sweep or finalize archived or merged between the context read and the effect, so an operation returned `contended` and **nothing this run merged remains to redo**. If *this* run performed the merge, it is `advanced` regardless of who archived it. | continue — re-select next |
| `drained` | `context.finalize` returned no eligible `implemented` candidate in scope. | **stop** |
| `halted` | Any abort-and-report point fired (see below), **or** every member of a non-empty eligible set needs a human. | **stop + surface** |

The driver's decision is binary: **continue on `advanced`/`contended`, stop on `drained`/`halted`.** The contract is **driver-agnostic** — Docket owns the contract, never the driver.

**One merge per invocation.** A run merges **exactly one** change through the `finalize.merge` operation and exits `advanced`; it **never batches**. Consecutive close-outs come from the driver re-invoking, not from an in-run loop. Archiving several already-merged changes (each a merged-recovery candidate with no merge to perform) does not violate this rule: no merge occurred, so there is no blast radius to bound.

**A blocked-but-non-empty set is `halted`, never `drained`.** There is work; it just needs a human. A candidate in scope but skipped for any human-requiring reason — a real skip-reason token, or a `## Finalize blocked` marker on the auto-detect path (a named id overrides that skip) — counts toward the non-empty set and yields `halted`. `drained` requires that `context.finalize` surfaced no `implemented` candidate at all.

The final report enumerates the change merged (if any), each change skipped with its closed reason, and the disposition that ended the run.

**Dummy mode** is a *deferred capability* in the Go runtime: `dummy_mode.enabled` is rejected at the config gate, so a repo that sets it cannot mutate at all and this skill never runs with it on. Treat it as unavailable — do not calibrate prose to `DUMMY_MODE_PERSONA` and do not author an `### In plain terms` block. When a human asks for plainer language in-session, simply write plainer language; that is an ordinary request, not this setting.

## The sequence

The steps below run for the one selected change. Each is one operation; read its document and route on the token.

### 1. Authoritative context

The `context.finalize` operation with `[--id <id> | --allowlist <ids>]`. Read-only. Select per *Selection* above. A candidate whose only skip reason is `approval-required` or `finalize-blocked` on an explicitly named or allowlisted run carries an `override_note` and proceeds; any other skip reason on the selected candidate is `halted`.

### 2. Retarget authorized children (only when open children exist)

If the candidate has open child PRs targeting this change's branch, they must be retargeted onto this change's effective base **before** the merge, or the merge refuses with `not-mergeable`/an open-children conjunct.

- **Attended run:** author the exact authorized child set the human confirmed into a request file — `{ID, PRNumber, PRVersion}` per child, taken from the context bundle — and run the `finalize.retarget-children` operation with `--id <id> --version <version> --input <file>`. It probes/acts/verifies each authorized PR onto the effective base and adopts an already-retargeted exact PR as a no-op. A child open in the live graph but **absent from the authorized set** returns `contended` with zero edits — a new child appeared; re-read context. Version drift, an ambiguous head, or a probe error returns `contended`/`unknown` and enables no parent merge. It writes no metadata and never touches `stacked_on:`.
- **Autonomous run:** retargeting an open child re-points work the human never authorized. An autonomous run does **not** author a child set: an eligible change carrying open unauthorized children is `halted` — record a `## Finalize blocked` marker (step 8's mechanism) and stop. Terminal children (`stacked-merged`/`done`) neither block nor retarget.

### 3. Rebase onto the effective base (resolver loop)

The `finalize.rebase` operation with `--id <id> --version <version> --head <feature head>`. It writes an ownership-scoped receipt before any Git mutation, rebases the feature branch onto the exact effective base under owned refs, then composes the local gate. Route on `disposition`:

- `unchanged` / `rebased` — the rebase completed and the gate was satisfied (skipped on exact-head PR evidence for a no-op, or run and passed). Carry forward the returned `attempt` token, the resulting `head`, and the gate report's `evidence` block (present when the suite ran; otherwise reuse the exact-head PR-body evidence). Go to step 6.
- `conflicted` — the rebase stopped at the reported `unmerged_paths`. Enter the resolver loop below.
- `waiting` with `reason: gate-waiting` — the rebase completed and the local suite is still running under the detached supervisor; the owned receipt now carries the drive continuation. Re-run the **identical** `finalize.rebase` invocation (same `--id --version --head`); it recovers the completed rebase from the owned receipt and advances the **same** drive — on `passed` it mints the evidence block and returns `unchanged`/`rebased` exactly as a single-slice run does. Never re-enter through `gate drive advance`, never mint a run root, never carry the continuation yourself — the receipt does. Repeat until a terminal disposition; the driver's observation budget bounds the loop, and a budget expiry surfaces as `blocked` with `reason: gate-halted`. A `waiting` returned by `finalize.rebase-continue` re-enters the same way: through `finalize.rebase`, never another `rebase-continue`.
- `failed` with `reason: gate-failed` — the rebase completed but the local suite is **red** at the rebased head. Enter the repair path (step 5).
- `contended` — a lost race (the base or remote head moved, or the record version drifted). Re-read `context.finalize`; the disposition is `contended`, never `halted`.
- `blocked` — retained foreign rebase state, a moved base, an unresolved effective base, a dirty workspace, or a precondition failure the receipt was **not** written for. A human is needed: `halted`.

<!-- docket:feature-dispatch:start targets=docket-rebase-resolver -->
**Resolver loop (Go-enforced budget).** On `conflicted`, first run the `finalize.resolver-reserve` operation with `--id <id> --attempt <attempt>` — it durably admits one resolver dispatch under the owned attempt, before anything is launched, and is the ONLY thing that authorizes a dispatch. Route on its `disposition`:

- `reserved` — authorizes exactly ONE resolver dispatch. Its payload includes the returned `reservation` token and contains:
Dispatch `docket-rebase-resolver` foreground at the model/effort its wrapper resolves.
Feature worktree: <absolute canonical feature-worktree root>
The resolver echoes the token back as the report's `resolver_reservation` field. It edits only the conflicted regions in the returned workspace and returns a versioned `ResolverReport` (fields in step 3 of *The two agents* in `references/gate-failure.md`) — it never runs the rebase mechanics or the suite.
- `pending` — a reservation is already outstanding: dispatch NOTHING new. If the child it belongs to is yours and identifiable, wait for it or feed its matching report to `finalize.rebase-continue`; if you cannot establish dispatch ownership, this is `halted` with the reservation retained.
- `exhausted` (`resolver-budget-exhausted`) — the configured `finalize.resolver_max_attempts` budget (default 3) is spent: `halted` via the abort flow below. Raising the limit takes effect on the next explicit finalize attempt, never this receipt.
- `blocked` / `contended` — `blocked` is `halted` (`resolver-budget-unavailable` marks a pre-budget receipt: the abort flow remains available, a resolver dispatch does not); `contended` is a lost race — re-read `context.finalize` and re-enter.

Feed the resolver's report back with the `finalize.rebase-continue` operation with `--id <id> --attempt <attempt> --input <report>`, which validates the reported paths against the live unmerged set (paths outside it → refusal `report-not-resolved`), stages exactly them, and continues. A continue may return `conflicted` again (the next commit's conflict — return to the reserve step above; the next dispatch requires a fresh reservation), `unchanged`/`rebased` (completed, gate composed), `failed`/`gate-failed` (completed, suite red → step 5), or `blocked` with `resolver-budget-exhausted` (the last permitted continuation surfaced another conflict → enter the abort flow). Never count resolver dispatches yourself, and never abort-and-restart to replenish a budget.

A resolver that returns `disposition: stuck`, or a resolver dispatch that is unavailable (the carve-out below), is `halted` through the abort flow. **Abort flow:** before running the `finalize.rebase-abort` operation on a workspace that might still hold a live resolver child, establish that child's completion through the harness — inability to establish it is itself `halted`, with the workspace retained. Then run the `finalize.rebase-abort` operation with `--id <id> --attempt <attempt> --input <report>` to restore the recorded original head under the owned attempt, record the `## Finalize blocked` marker (step 8), and stop. `rebase-abort` verifies restoration; a failed restoration is itself `halted`.
<!-- docket:feature-dispatch:end -->

### 4. The local gate

The gate is composed into `finalize.rebase`/`rebase-continue`: a completed rebase runs the full resolved suite unless the rebase was a **no-op and** the PR body carries **green** build-evidence for the **exact** current head **whose recorded command equals the resolved `finalize.test_command`**, in which case the run is skipped and the permit named in the gate report. A **skipped** (`build-gate-off`) build-evidence record never waives finalize's gate — the build role's gate policy is independent of finalize's, so skipped build evidence, or green evidence recorded against a different command, forces the suite to run. There is no strict-ancestor or results-only skip. A passing gate records its evidence through the landed `evidence.record` seam and returns the block in the rebase document; a red gate returns `failed`/`gate-failed` (step 5).

`docket gate` owns the gate's mechanics — the supervised run outliving any foreground call, the durable run directory, completion established from that artifact, and the bounded `gate_observation_budget` that fails closed when spent. **One clause of `docket-build`'s *Gate execution posture* it cannot own is yours to obey:** whether you may **yield** while the gate runs is decided by *your own* dispatch posture, never by the gate's. Only a top-level session agent, able to receive a resumption signal, may yield and then make short observations. Running as a dispatched or forked child you have no such channel, so you may **never** yield — observe by *blocking* instead, in repeated short foreground reads, control never handed back to your caller mid-gate. `gate.observe` is a single read-only report and cannot tell which you are; a child that yields here parks until a human notices.

### 5. Repair a red gate

<!-- docket:feature-dispatch:start targets=docket-integration-repair -->
A red suite after the rebase is repair work, regardless of cause.
Dispatch `docket-integration-repair` foreground at the model/effort its wrapper resolves. Its payload contains:
Feature worktree: <absolute canonical feature-worktree root>
Red-test root cause, bounded two-attempt feature-branch fix, and claimed commits plus `repaired`/`stuck`; it never rebases, merges, or transitions metadata. Then re-run the gate on the repaired head: `gate.launch` with `--root <run-root> --cwd <feature worktree> -- <resolved suite>`, then `gate.observe` with `<run-dir>`, under `docket-build`'s gate-execution posture. On a `passed` terminal observation whose head equals repaired head, `evidence.record --id <id> --run <absolute-run-dir> --head <repaired head>` returns the immutable block — no agent-supplied `passed` boolean; a failed/running/stopped/vanished/malformed/head-mismatched run produces none, and a repair that cannot reach green in two attempts, or unavailable repair dispatch, is `halted`.
<!-- docket:feature-dispatch:end -->

### 6. Sign-off on an authored repair

A repair is code the human's PR review never saw, so it never merges unseen:

- **Autonomous run:** cannot prompt. Record the sign-off requirement durably and **stop**: the `finalize.block` operation with `--id <id> --version <version> --pr-number <n> --attempt <attempt> --reason repair-needs-signoff --head <repaired head> --input <block report>`. This ensures the owned PR comment (idempotent by the attempt marker), then upserts the single `## Finalize blocked` section naming the reason. Disposition `halted` — the human reviews the pushed repair on the PR and re-runs finalize.
- **Attended run:** publish the repaired head (step 7), then **prompt** the human with the repair diff and what broke before merging. On go-ahead, clear the block and merge.

A pass with **no** authored repair (an exact-head-evidence skip, or a clean rebase whose suite passed first try) skips this step entirely.

### 7. Publish the rebased head

The `finalize.publish` operation with `--id <id> --attempt <attempt> --head <head> --evidence <evidence file>`. It probes the remote first (a no-op when already at `head`), pushes exactly `head` under the receipt's exact old-value lease, then converges the PR build-evidence block onto that head — authored prose, title, and every other body byte stay byte-identical. It never creates a second PR. A reprobe `unknown` returns `rewrite-unknown`/`pr-probe-failed` and stops with no second mutation (`halted`); a moved remote returns `rewrite-contended` (re-read context, `contended`); an attempt token not matching the receipt is refused before any push. On the repair path, follow publish with the `finalize.clear-block` operation with `--id <id> --version <version> --head <repaired head> --pr-number <n>`, which removes the marker only after reprobing the exact current head, valid gate evidence, the published remote ref, and the matching open PR.

### 8. Merge exactly once

The `finalize.merge` operation with `--id <id> --version <version> --head <head>`. It reloads fresh authority and rechecks every merge conjunct immediately before the effect — implemented, PR identity, heads agree, base is the effective base, gate satisfied, approval satisfied, no open children, not superseded — and refuses with that conjunct's closed token (`not-mergeable`, `pr-not-open`, `unresolved-base`, a child conjunct, …) issuing **no** merge call when any fails. Before the effect it selects the best merge method the repository settings and the base branch's active rules permit, in the fixed order rebase → merge commit → squash, and attempts exactly that one; the document's `method` field reports the attempted method (absent on already-merged recovery). A cleanly observed empty permitted set refuses `blocked` with reason `merge-method-unavailable` before any merge — fix the repository or branch-rule merge settings; it is not `merge-denied` and is never retried with another method. It merges at the exact expected head, never requests a branch delete, and verifies the merge authoritatively: a reprobe returns the exact `mergedAt`/merge-commit facts and a Git fetch proves the merge commit reachable from the destination tip. An open PR on reprobe is not merged; a different head/base is `contended`; an unobservable result is `unknown` — none permits closeout. An already-merged exact PR is a verified no-op regardless of who merged it, never a second merge.

`--admin` is honored **only** on an attended, explicitly-named run where a sole maintainer chooses to force past an otherwise-unsatisfiable required review; it is never inferred from an approval absence or a permission error, and a `merge-denied` stays `denied` (`halted`). A named id overrides the `approval-required` and `finalize-blocked` skips (step 1); it never overrides malformed state, a wrong PR identity, an unsafe stack, or the repair sign-off.

### 9. Closeout — archive the terminal records

Every mutating Go transaction re-renders `BOARD.md` in the same commit as the record it reflects, so the board needs no separate pass and stays fresh by construction. The board is the live planning view and is **never** published to the integration branch.

The `finalize.closeout` operation with `--id <id> [--input <request-file>]`. When the finalize invocation
supplied verification outcomes or late findings, translate that prose into the two structured
lists — `verification_outcomes` and `late_findings`, each an array of strings — write them as a
bounded JSON request file, and pass it via `--input`; closeout renders them under `## Closeout
notes` in the same transaction that archives the record, and an identical-notes retry replays as
`already` while different notes against a terminal record are refused (`terminal-notes-frozen`).
When no notes were supplied, call the unchanged no-input form and archive immediately — there is
no post-merge pause or second user step. No caller-supplied done boolean or archive date: it reloads metadata, reprobes the PR and its destination, derives the UTC archive date from the verified `mergedAt`, and applies one atomic transaction. Route on `disposition`:

- `done-archived` — an ordinary change merged to the integration branch: marked `done` (only after the merge-commit reachability proof), relocated to the dated archive path, artifact block + spec backlink + inline board rerendered, validated, committed by explicit path, lease-pushed.
- `stacked-merged` — the PR merged into a live parent's branch: marked `stacked-merged` in place, not archived, branch and workspace retained until the root lands.
- `root-archived` — a stack root reached integration and every carried descendant is proven: its chain of merged PR destinations establishes the carry relationship AND its merged work is verified still present in Git — reachable in the pinned integration history, or exact-content at the root's merge result — since a merged destination alone is a relationship, never proof the content shipped. One transaction archives the root and every descendant using the root's merge date, one board render over the final population. One unproven descendant leaves the root recoverable with zero descendant writes.
- `already` — the promised terminal state already exists (a response-lost success): a keyed no-op, never a duplicate transition.
- `children-retarget-required` — a descendant is not yet stacked-merged; return to step 2 (attended) or `halted` (autonomous).
- `contended` / `blocked` / `unknown` — a lost race, an illegal source status or destination mismatch, or an unobservable probe; re-read context (`contended`) or stop (`halted`). In `docket` mode the metadata transaction lands first and a separate integration-ref leg patches only the existing `docket:backlink` blocks; a failed leg leaves the change truthfully `done` with a `terminal-backlink-pending` finding that a retry recovers — cleanup (step 10) repairs it, never a reason to redo the merge.

### 10. Cleanup — Docket-owned resources only

The `finalize.cleanup` operation with `--id <id>`. An ordered, independently retryable suffix: repair a pending backlink first, remove the workspace from manifest facts, delete the local branch only when its exact recorded tip is worktree-detached and contained in the verified merge chain, and delete the remote ref only under the exact lease after a fresh probe shows no open child PR targets it. Every probe treats present, cleanly absent, and unknown as three outcomes — an injected or real probe error retains the resource with a pending result and `children-retarget-required`/a retention reason, never a destroy. A non-terminal change refuses (the one pre-terminal exception is restoring an aborted owned rebase). Stacked-merged changes retain their workspace and branches until the root closes. Cleanup failure never unwinds the merge — the merge is never rolled back.

### 11. Integration sync — best-effort, end-of-run

After the batch's closeout and cleanup attempts (including already-merged recovery and any pending-retained cleanup), run the `repository.sync-integration` operation once (resolve its argv from the capability catalog) with `--repo-dir <primary checkout path from the Step-0 `repository.prepare` context>` and `--json`. Use the primary-checkout path from that Step-0 prepare context, not a feature or `.docket` worktree — a path that survives feature-worktree removal, since cleanup (step 10) may already have deleted the change's own worktree. Surface its outcome separately from the merge/closeout report. Posture: **best-effort.** A sync refusal or failure is reported but never undoes or replaces a completed merge, closeout, or cleanup result, never writes a `## Finalize blocked` marker, and never suppresses unrelated work. If the batch halted after earlier verified merges, still run this suffix when the repository context is valid and execution is not cancelled, and preserve the original halt verdict — the sync outcome never changes it. Never run it through a failed bootstrap or an unknown repository identity. A stacked child never makes its parent branch the target: the operation always resolves the configured integration branch itself. Do **not** add this sync to the per-change merge or closeout steps (8–9); it is the whole run's single end-of-run suffix, run once regardless of how many changes the run closed out.

### 12. Repository-required post-merge rebuild — after sync, verified

Some repositories' agent-instruction files require rebuilding or reinstalling a tool after a merge
to the integration branch. When such a requirement applies, it runs **after** step 11's integration
sync — never before it, and never per-change inside steps 8–9 — and a rebuild is authorized only
when that sync's document reported disposition `advanced` or `already-current` with valid
`primary_path` / `integration_branch` / target facts. A `skipped`, `refused`, or `failed` sync,
missing facts, malformed output, or an unobservable outcome leaves the rebuild not performed — a
clean process exit on a deliberate skip is not authorization.

Retain the actual merge-commit ids from step 8's authoritative merge verification (including
already-merged recovery); a feature branch's original head is never a substitute — rebase and
squash change commit identity. A stacked-only merge into a live parent's branch does not trigger an
integration-merge rebuild rule.

Before installing, verify the exact source directory the repository policy names for the install:

- Its canonical identity must be the synced primary checkout, on the expected integration branch,
  with no unfinished Git operation and a clean index and worktree including non-ignored untracked
  files. Read and retain its full HEAD commit id `S`; require `S` to equal the sync document's
  target/after commit ids. Unknown or changed state does not authorize installation.
- Prove every retained merge commit is contained in that source: run
  `git merge-base --is-ancestor <merge-commit> <S>` in the source repository, treating a
  negative answer and a failed probe as two distinct outcomes — neither permits the rebuild. This
  proves the source install reads, not merely a remote-tracking ref or a feature worktree.

Then run the repository's named install operation (argv resolved from the capability catalog) with
its stated source argument; a single successful rebuild may satisfy every verified integration
merge in the batch, and a failed install is never silently retried through another build method.
After a successful install, re-check that the source branch, cleanliness, and HEAD still equal the
observations for `S`, then read the installed executable's own identity — the version operation of
the binary at the destination the install actually updated. Require its full, clean commit
identity to equal `S` exactly. A timestamp, a version label, a short-prefix comparison, or the
identity of an older running process proves nothing; unknown, dirty, mismatching, or unreadable
identity is verification failure. The success report names the verified installed commit; observed
source movement is reported honestly as an unverified rebuild — an installation may already have
changed, so never claim it was untouched and never attempt a rollback.

**Failure posture — report separately, reverse nothing.** Any unmet condition keeps every verified
merged change `done` and is reported separately as **binary rebuild incomplete**, naming the failed
condition and the source path. A failed rebuild never reverses a merge, revives a terminal change,
writes a `## Finalize blocked` marker, rewrites frozen records or closeout notes, or changes an
earlier halt verdict. Never stash, switch branches, reset, discard files, or build from another
checkout to force the rebuild. State the specific obstacle and the recovery sequence: resolve the
reported source state, rerun integration sync, then repeat the proof, install, and identity check —
the bare install command alone is never a sufficient remedy for stale source. A repository whose
instructions carry no such requirement skips this step entirely.

## Identity repair checkpoint

Two skip reasons from `context.finalize` name a mismatch between the recorded `branch:` and the PR's identity rather than an ordinary blocker. Each is `halted` for a non-interactive caller and a human-gated repair for an attended one. **Never reconstruct a branch name and never search for a likely branch or PR** — the only names offered come from the recorded field and the exact PR the prober read.

- **`branch-pr-head-mismatch`** — the recorded `branch:` and the exact PR's reported head disagree. Present the evidence — change id + version, the recorded `branch:`, the exact PR number and state, and the reported head — and offer exactly three choices:
  - **Trust the PR** — adopt the PR's head as the record: the `change.repair-identity` operation with `--id N --expect-version V --adopt-pr-head --expect-pr M --expect-head H`.
  - **Trust the record** — keep `branch:` and re-point the record at the correct PR the human supplies: the `change.repair-identity` operation with `--id N --expect-version V --adopt-pr <ref> --expect-branch B`.
  - **Abort** — no writes.
- **`branch-missing`** — the recorded `branch:` resolves to no remote ref. Offer **only** the exact PR's reported head (the repair op itself proves that remote branch exists); confirm it or abort. Never search for a likely branch or PR.

After a successful repair, **reload and re-probe from scratch** — run the `context.finalize` operation with `--id N` again before any finalize effect; the repaired record is authority only once re-read. A `stale-evidence` / `workspace-conflict` / `pr-unknown` / `candidate-branch-absent` refusal from the repair op is reported to the human **verbatim** and stops the flow — it is never retried around.

**Non-interactive callers** (implement-next's finalize sweep) never repair autonomously: they `halt` with the structured evidence for a human to resolve.

## Sign-off, abort, and the blocked marker

The full abort-and-report set, the two-agent split, the sign-off rule, and the `## Finalize blocked` marker's write shape and lifecycle live in **`references/gate-failure.md`** — **read it at any abort** (a conflict, a red gate, an unavailable dispatch, a denied merge) before recording or reporting. Every abort-and-report point maps to `halted`, leaves the PR open and the change `implemented`, and records the `## Finalize blocked` marker via the `finalize.block` operation (comment first, then the single upserted section); the `finalize.clear-block` operation removes it after a successful reprobe.

## Dispatch unavailability — the carve-out

Both gate dispatches (`docket-rebase-resolver`, `docket-integration-repair`) sit outside the convention's dispatch-tier table by an explicit carve-out. Read the convention's *Dispatch-capability resolution* section for when unavailability is established at all — resolution first, then one trivial attempt, and **never** from a tool name. A conflicted rebase whose resolver cannot be dispatched, or a red gate whose repair cannot be dispatched, takes the carve-out posture: `halted`, exactly as an ambiguous conflict or a stuck repair does. Neither agent is ever substituted inline — reconciling hunks, or authoring a repair, in the same run that then merges that work is the self-approval shape the carve-out forbids.
