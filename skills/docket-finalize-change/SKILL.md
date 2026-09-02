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
- `failed` with `reason: gate-failed` — the rebase completed but the local suite is **red** at the rebased head. Enter the repair path (step 5).
- `contended` — a lost race (the base or remote head moved, or the record version drifted). Re-read `context.finalize`; the disposition is `contended`, never `halted`.
- `blocked` — retained foreign rebase state, a moved base, an unresolved effective base, a dirty workspace, or a precondition failure the receipt was **not** written for. A human is needed: `halted`.

**Resolver loop (skill-enforced ≤2 attempts).** On `conflicted`, dispatch `docket-rebase-resolver` (foreground, at the model/effort its wrapper resolves) naming the feature worktree in the payload. It edits only the conflicted regions in the returned workspace and returns a versioned `ResolverReport` (fields in step 3 of *The two agents* in `references/gate-failure.md`) — it never runs the rebase mechanics or the suite. Feed that report back with the `finalize.rebase-continue` operation with `--id <id> --attempt <attempt> --input <report>`, which validates the reported paths against the live unmerged set (paths outside it → refusal `report-not-resolved`), stages exactly them, and continues. A continue may return `conflicted` again (the next commit's conflict), `unchanged`/`rebased` (completed, gate composed), or `failed`/`gate-failed` (completed, suite red → step 5). **The skill counts resolver dispatches and allows at most two.** A resolver that returns `disposition: stuck`, a conflict still unresolved after the second dispatch, or a resolver dispatch that is unavailable (the carve-out below) is `halted`: run the `finalize.rebase-abort` operation with `--id <id> --attempt <attempt> --input <report>` to restore the recorded original head under the owned attempt, then record the `## Finalize blocked` marker (step 8) and stop. `rebase-abort` verifies restoration; a failed restoration is itself `halted`.

### 4. The local gate

The gate is composed into `finalize.rebase`/`rebase-continue`: a completed rebase runs the full resolved suite unless the rebase was a **no-op and** the PR body carries **green** build-evidence for the **exact** current head **whose recorded command equals the resolved `finalize.test_command`**, in which case the run is skipped and the permit named in the gate report. A **skipped** (`build-gate-off`) build-evidence record never waives finalize's gate — the build role's gate policy is independent of finalize's, so skipped build evidence, or green evidence recorded against a different command, forces the suite to run. There is no strict-ancestor or results-only skip. A passing gate records its evidence through the landed `evidence.record` seam and returns the block in the rebase document; a red gate returns `failed`/`gate-failed` (step 5).

`docket gate` owns the gate's mechanics — the supervised run outliving any foreground call, the durable run directory, completion established from that artifact, and the bounded `gate_observation_budget` that fails closed when spent. **One clause of `docket-build`'s *Gate execution posture* it cannot own is yours to obey:** whether you may **yield** while the gate runs is decided by *your own* dispatch posture, never by the gate's. Only a top-level session agent, able to receive a resumption signal, may yield and then make short observations. Running as a dispatched or forked child you have no such channel, so you may **never** yield — observe by *blocking* instead, in repeated short foreground reads, control never handed back to your caller mid-gate. `gate.observe` is a single read-only report and cannot tell which you are; a child that yields here parks until a human notices.

### 5. Repair a red gate

A red suite after the rebase is repair work, regardless of cause. Dispatch `docket-integration-repair` (foreground, at the model/effort its wrapper resolves) naming the feature worktree. It root-causes the red tests, authors a **bounded** fix in at most two attempts, commits it on the feature branch, and returns a report naming its claimed commits and `repaired`/`stuck`; it never runs the rebase, merges, or transitions metadata. Then re-run the gate on the repaired head yourself: the `gate.launch` operation with `--root <run-root> --cwd <feature worktree> -- <resolved suite>`, then the `gate.observe` operation with `<run-dir>`, under the gate-execution posture `docket-build` owns (its `references/gate-execution.md`, including the observation budget). On a `passed` terminal observation whose head equals the repaired head, the `evidence.record` operation with `--id <id> --run <absolute-run-dir> --head <repaired head>` returns the immutable block — there is **no** agent-supplied `passed` boolean; a failed/running/stopped/vanished/malformed/head-mismatched run produces none, and a repair that cannot reach green in two attempts, or a repair dispatch that is unavailable (the carve-out below), is `halted`.

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
- `root-archived` — a stack root reached integration and every carried descendant is proven: one transaction archives the root and every descendant using the root's merge date, one board render over the final population. One unproven descendant leaves the root recoverable with zero descendant writes.
- `already` — the promised terminal state already exists (a response-lost success): a keyed no-op, never a duplicate transition.
- `children-retarget-required` — a descendant is not yet stacked-merged; return to step 2 (attended) or `halted` (autonomous).
- `contended` / `blocked` / `unknown` — a lost race, an illegal source status or destination mismatch, or an unobservable probe; re-read context (`contended`) or stop (`halted`). In `docket` mode the metadata transaction lands first and a separate integration-ref leg patches only the existing `docket:backlink` blocks; a failed leg leaves the change truthfully `done` with a `terminal-backlink-pending` finding that a retry recovers — cleanup (step 10) repairs it, never a reason to redo the merge.

### 10. Cleanup — Docket-owned resources only

The `finalize.cleanup` operation with `--id <id>`. An ordered, independently retryable suffix: repair a pending backlink first, remove the workspace from manifest facts, delete the local branch only when its exact recorded tip is worktree-detached and contained in the verified merge chain, and delete the remote ref only under the exact lease after a fresh probe shows no open child PR targets it. Every probe treats present, cleanly absent, and unknown as three outcomes — an injected or real probe error retains the resource with a pending result and `children-retarget-required`/a retention reason, never a destroy. A non-terminal change refuses (the one pre-terminal exception is restoring an aborted owned rebase). Stacked-merged changes retain their workspace and branches until the root closes. Cleanup failure never unwinds the merge — the merge is never rolled back.

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
