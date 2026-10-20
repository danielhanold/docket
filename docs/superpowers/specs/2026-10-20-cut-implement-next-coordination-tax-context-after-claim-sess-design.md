# Stop implement-next tripping over its own bookkeeping — design

Change 0360 — Stop implement-next tripping over its own bookkeeping

## Why

An implement-next run spends a steady share of its turns on bookkeeping between its own writes, not on the change. Change #360's stub (2026-08-27) listed about fifteen such costs. Most were fixed by later changes (see *Already fixed — do not rebuild*). What remains was measured on the last 15 `docket-implement-next` runs (transcripts from 2026-10-17 to 2026-10-20; 14 of them claimed a change):

- **The revision the agent passes is the wrong one.** Each record write takes `--revision <blob id of the change record>`. The claim's receipt carries `committed_revision`, which is the metadata **commit** id (`res.AppliedCommit`; ADR-0129 defines every `*_revision` result key as a commit id). The skill tells the agent the claim "returns the new record revision", so 13 of 14 claimed runs passed the commit id to `change.reconcile` and got a bare `contended`.
- **The documented way to re-read the revision refuses.** The agent then calls `context.implementation --id <id>`, which refuses every in-progress change: `invalid-state` / `not-ready-not-proposed (state "in-progress")` (14 of 14 runs). `selectContextChange` runs `domain.ClaimEligibility` → `EvaluateReadiness`, which returns `ReadyNotProposed` for any non-`proposed` record. Yet two skill edits from 2026-10-06 (b559fd4d7, 440f16fae) route the post-plan and results-checkpoint re-reads through it. The skill's own named path also lists "a named resume of an in-progress change" as going straight to `context.implementation --id`, so the documented resume entry hits the same refusal.
- **So every write is preceded by a hand-rolled lookup.** Runs settle on `docket repository prepare … >/dev/null; R=$(git -C .docket rev-parse HEAD:<record path>)` before each write — 6.7 `repository.prepare` calls per run on average. Since #549 every docket write already fast-forwards the local metadata checkout, and every typed read (`status`, `context.implementation`) fetches and pins origin itself, so nearly all of those re-syncs do nothing.
- **Four claim refreshes protect nothing.** The skill calls `change.refresh-claim` before and after the plan dispatch and the build. The lease's only consumer is reclaim (`domain.EvaluateReclaim`, the sweep's `maintenance.go` worklist, `change.reclaim`), and reclaim refuses whenever the recorded or conventional feature branch exists locally or remotely, or a live workspace or gate run exists. The branch exists from `workspace.prepare` onward, which runs minutes after claim; claim and reconcile both stamp `claimed_at` before that. Nothing else reads `claimed_at` (no board cell, no status field). Only implement-next calls the operation.
- **`evidence.verify` fails in 13 of 15 runs.** The skill says `evidence.record` "returns the immutable typed record" and then to run `evidence.verify --record <request-file>`; agents save the JSON receipt and pass it, and verify answers `missing-evidence: the record file carries no build-evidence block` (it wants the receipt's `block` text). That call also repeats a check `pr.publish` and `change.mark-implemented` each make themselves (`evidence.Verify(req.EvidenceRecord, req.Head)` in `pr_publish.go` and `change_implemented.go`).
- **Smaller costs:** `docket schema change.reconcile` (no `--operation`) is refused as an unknown command — agents tried that form 10 times in 15 runs; Step 7 loads the 109-line `references/edge-paths.md` on every PR run for its last section only; the review-tier size bump diffs `origin/<integration_branch>...HEAD`, so a stacked child counts its parent's lines; nothing says reconcile's log entry is the body only, or what to send when nothing changed; `repository.prepare` returns an always-empty `skills: {}` that nothing reads.

The point is not to make a run skip safety checks. It is to stop the run failing on its own bookkeeping and re-deriving facts the system already keeps current.

## Goals

- A run's writes succeed first time: no self-inflicted `contended`, no `context.implementation` refusal for the run's own claimed change, no failed `evidence.verify`.
- About two `repository.prepare` calls per run instead of about seven.
- The resume entry the skill documents actually works.
- Every skill sentence about receipts, leases, re-syncs, and evidence matches what the binary does.

## Non-goals and anti-goals

- **No new receipt field.** Considered and rejected at grooming: the run looks the revision up instead (see 1). A `record_revision` field would save one local read per write, but it adds a new term (the key `revision` already means a commit id in run-tracker results, per ADR-0129) for a protection nothing needs.
- **No conflict checks removed.** Every write keeps its exact-revision check and exact-lease push. Claim eligibility, stacked-base resolution, and "no metadata writes on the feature branch" are unchanged. Accepted at grooming: because the run looks the revision up just before each post-claim write (1), that check guards the moment between the lookup and the write, not everything since the run's previous write. That is what today's runs already do by hand, and nothing else writes an in-progress record without changing its status (which reconcile, halt, and mark-implemented refuse) or moving it (kill), while claim and the run tracker prevent a second run on the same change.
- **No Go `docket implement` driver.** The fixes below remove the busywork a driver was meant to hide.
- **No gate "couldn't run" result.** Exit 126/127 under `/bin/sh -c` still reads `FAILED`. The stub's trigger (agent-supplied gate argv) no longer exists: owners take no argv since #374/#489. Out of scope until it is seen again.
- **No change to the parent's check of the plan-writer's work.** A dispatched agent's "done" is not proof; the check is one read and stays.
- **No change to `docket config --repo-dir`'s local-config lookup** (it reads the given folder, not the main checkout). Not hitting any run today; left out deliberately.
- **No change to selection.** `context.implementation` with no `--id` still selects only build-ready `proposed` changes.

## Design

### 1. Revision lookups: read the local copy before each write

Before every write to its own change record after the claim — `change.reconcile`, `change.halt`, `change.kill` on a reconcile-kill, `workspace.prepare` (which compares `--revision` without writing), the revision handed to the plan-writer for its `change.attach-plan`, the producing-change revision handed to `docket-adr` (see 6a), each `change.attach-results` checkpoint, and `change.mark-implemented` — the run reads the record's current revision from the local metadata checkout, in the same shell call as the write (or immediately before a dispatch that carries it):

```
R=$(git -C <metadata_worktree_path> rev-parse HEAD:<record path>)
```

`<record path>` is the change's `path` from the pre-claim bundle; it does not move while the change is active. The skill states plainly that every receipt's `committed_revision` is the commit id and is never a `--revision` value, and removes the sentences saying claim and refresh "return the new record revision".

**On `contended`:** re-sync once (`repository.prepare`), look the revision up again, and retry the write once — except `change.reconcile`, which keeps its existing rule (re-read fresh context and re-author the request, since its content depends on the record). A second `contended` follows the step's existing stop rule. This covers the rare case where #549's best-effort follow left the local checkout behind (dirty, ahead, or diverged), and it guards the same lookup-to-write window the non-goals accept.

The claim itself still uses the revision from the pre-claim bundle, as today. A resume (`change.resume-halted`) uses the revision from the context bundle (see 2).

### 2. `context.implementation --id` serves the run's in-progress change

With an explicit `--id` naming an `in-progress` change, `context.implementation` returns the normal bundle instead of refusing:

- `change` (path, source, revision, summary), `spec`, `related`, `effective_base`, `adrs`, `learnings`, `workflow`, `warnings` — computed as for a proposed change.
- `workflow.feature_branch` carries the recorded `branch:` for an in-progress change (today it is always the branch a claim *would* mint, `domain.MintBranch(type, branch_prefix, slug)`, pinned by `TestContextImplementationFeatureBranchHonorsMintPrefix`; keep that for proposed changes and update the field comment, which says the context is produced before the claim).
- `halt.run_halted` reports the `## Run halted` marker. Today it is readable only on a proposed record carrying a leftover marker (`TestContextImplementationReportsHalt`); the in-progress records that `change.halt` actually marks are refused.
- `readiness` and `claim_eligible: false` with `claim_refusal` carrying today's reason (`not-ready-not-proposed`). A plain named run on an in-progress change therefore reads "not claimable" from the bundle and ends exactly as it does now (surfaced refusal, `drained`); only a resume proceeds.
- Add the record's `claimed_at` and `reconciled` values if the builder finds the resume path needs them from the bundle rather than the source bytes (the source bytes already carry both — builder's call).

Every other non-`proposed` status (`blocked`, `deferred`, `implemented`, `stacked-merged`, `done`, `killed`) refuses exactly as today, and a `proposed` change keeps today's readiness refusals. No `--id`: unchanged. Update the code comment "The explicit change must be claimable exactly as a claim requires" to state the new rule.

Skill side: the named path's documented resume entry now works as written; the post-plan and results-checkpoint "re-read via `context.implementation`" sentences become the local lookup from 1 (one rule for every post-claim revision).

### 3. Delete the claim refreshes and `change.refresh-claim`

- Remove every `change.refresh-claim` call from `docket-implement-next` (Step 4 before the plan dispatch and after it, Step 5 before and after the build) and from `references/results-artifact.md` ("Refresh the claim … at each phase boundary").
- Delete the operation: the `docket change refresh-claim` CLI command, its capability-catalog entry, its schema registration, `ChangeRefreshClaim` and the `refresh` branch of `changeClaimOp` in `internal/app/change_claim.go`, their tests, the glossary entry, and any install allow rule naming it (`internal/cli/install.go`). **Keep `domain.RefreshClaim`** (rename it if the builder prefers): `change.reconcile`'s claimed_at patch step and `change.resume-halted` both call it, and that stamping stays. Whole-repo grep for `refresh-claim` / `RefreshClaim` / `refresh_claim`; point-in-time records (archived changes, specs, plans) stay as written. Check `internal/repoguard/retired_vocabulary_test.go` — retire the token there if that guard's pattern calls for it.
- `claimed_at` is still stamped by `change.claim`, `change.reconcile`, and `change.resume-halted`, and cleared on leaving `in-progress`. Fix the wording to match: the convention's manifest comment (`claimed_at: … stamped at claim; refreshed at phase boundaries; …`); the field-write rule's "the transaction re-stamps `claimed_at` … on every mutation" (it does not: attach-* and mark-implemented leave it untouched) and its "the skill also refreshes the claim explicitly … around the long plan and build dispatches" sentence; the paragraph under SKILL.md's *Step postconditions* table saying rows 3, 4, 6 and 7 carry "the `claimed_at` re-stamp" (rows 4 and 7 never re-stamp); and the "stays `in-progress` with `claimed_at` refreshed" halt phrases (SKILL.md Step 5, `references/fix-pass.md`).
- The `reclaim.lease_ttl` key, `change.reclaim`, and the sweep's reclaim are unchanged.
- **Accepted at grooming — the cross-clone case.** `workspace.prepare` creates the feature branch locally; it reaches the remote only at `workspace.publish` (Step 7). From another clone, reclaim therefore sees no branch during plan and build, and only the lease protects the change. Without refreshes the lease runs from the reconcile stamp, so another clone could reclaim a running change only if the run took more than `reclaim.lease_ttl` (72h by default) between reconcile and publishing, and someone ran `change reclaim` there or had `reclaim.auto: true` (off by default). Runs take hours; a build that itself ran past the TTL was already exposed with refreshes. Folding a re-stamp into attach-plan/attach-results was considered and declined.

### 4. Re-sync only where it matters

`repository.prepare` runs:

1. at startup (unchanged), and after `maintenance.preflight` on the selection path (unchanged);
2. once after any dispatched agent that writes the change record returns — the plan-writer (`change.attach-plan`) and `docket-adr` (`adr.record` with the producing change, 6a) — before the parent checks that agent's work from git (one rule for both);
3. once after a `contended` write, per 1;
4. on a contended claim, before re-selecting (unchanged).

Remove the others: the Step 4 "local metadata tip must equal the remote tip" SHA check after claim (an `applied` receipt already proves the exact-lease push landed — reword postcondition row 2's "landed (local tip == remote tip)" to name the applied receipt), the "re-sync immediately before reading" the spec, the re-syncs around the build, and before each results checkpoint and its re-read of the results file. Typed reads fetch on their own; plain-file reads of `.docket` follow every docket write since #549. Keep one sentence in the skill saying why (so a later edit does not re-add them).

### 5. Reconcile wording

Step 3 says: when nothing changed (spec, stack base, related changes, and scope all hold), send `{id, revision, reconcile_log_entry}` with a one-line body such as "Unchanged against `<integration tip>`". Owned sections, spec sections, and relations are included only when reality requires a change. The `reconcile_log_entry` is the **body only** — the operation writes the `### YYYY-MM-DD` heading (`appendReconcileLog`); mirror the halt prose's "section body only" wording. No Go change.

### 6. Evidence wording

Step 6: `evidence.record`'s receipt carries the evidence as its `block` field. Write that text to a file; that file is the `--evidence` input for `pr.publish` and `change.mark-implemented`. Delete the standalone `evidence.verify` call after `evidence.record` — the record call already refuses anything but a `PASSED` drive at that head, and both consuming commands re-verify the bytes. "Repeat the record → verify chain" after review fixes becomes "repeat the record step". Keep the *Validate the build evidence* head check at the start of Step 6 as written.

### 6a. docket-adr records the ADR number on the change itself

`adr.record` already takes an optional producing change `{id, path, revision}` (`ADRProducingChange` in `internal/app/adr_ops.go`): when supplied, the same transaction checks that exact revision, appends the new id to the change's `adrs:`, and re-renders `## Artifacts`. The `docket-adr` skill already says to supply it from implement-next Step 6; implement-next never does, and instead writes `adrs:` with a second `change.reconcile`.

Step 6 now looks the revision up (1) and passes `{id, path, revision}` in each `docket-adr` dispatch payload; `docket-adr` passes it through as the request's `change`. Delete the Step 6 "record that number in the change's `adrs:` relation through a `change.reconcile`" write. After the dispatch returns, re-sync once (4.2) and check that the returned number is in `adrs:` (postcondition row 6 is unchanged). A `contended` from `adr.record` is handled by the child's existing refusal posture; the parent re-syncs, looks the revision up again, and re-dispatches once, then follows Step 6's existing stop rule. Several ADRs in one run are dispatched one at a time, each with a fresh lookup. No Go change.

### 7. Review size bump uses the change's own base

Step 6's tier modifier ("more than 1500 changed lines … from `git diff --shortstat origin/<integration_branch>...HEAD`") diffs against the change's resolved effective base instead: the bundle's `effective_base` — `origin/<integration_branch>` for an ordinary change, the parent's branch for a stacked one (the same base `workspace.prepare` cut from). Skill prose only; there is no Go implementation of the rule.

### 8. Leftovers riding along

- **Delete `skills` from `repository.prepare`.** Remove `PrepareSkills`, the `Skills` field on `PrepareContext`, and its constructor line. The schema is reflected from `RepositoryPrepareResult` through its single registry row, so the descriptor follows the struct. Update the test that builds a populated `PrepareSkills` to prove the human output redacts it (`internal/cli/repository_test.go`). Nothing reads the field; config refuses every `skills.*` key. Keep that config-side refusal as it is.
- **Move PR-body assembly into Step 7.** Move the *PR-body assembly (Step 7)* section of `references/edge-paths.md` into SKILL.md Step 7 and drop Step 7's "read `references/edge-paths.md` now (blocking)" for it; edge-paths keeps its rare-edge sections and their conditional loads.
- **`docket schema <id>`.** Accept one optional positional argument as the operation id, equivalent to `--operation <id>`; both together must agree or refuse as invalid arguments. Update the catalog signature (`[<id>] [--operation <id>]` or the house form) and the command help.

## Already fixed — do not rebuild

From the stub, now handled elsewhere: named runs skip the maintenance preflight (#448); build and finalize gates run the configured command as one `/bin/sh -c` string and take no argv (#374, #489); `evidence.record` reads the owner's own test command (#517); results land on the metadata branch, so they never stale evidence (#530); operational config reads `.docket.local.yml` from the main checkout; request schemas are authoritative and probes are not the discovery path (#399, #520, #521); dummy-mode, auto-capture, and gate-execution references are gone (#502); prepare output is one JSON line; reconcile never required section rewrites; build-worker prompts are already small (median about 1.1k characters).

## Proof

- **Go:** unit and integration tests for `context.implementation --id` on an in-progress change (bundle returned, `claim_eligible: false`, `claim_refusal: not-ready-not-proposed`, `feature_branch` set, `halt.run_halted` true when marked) and that every other non-proposed status still refuses; a mutation check (restore the old refusal → the new test reddens). Tests that `change.refresh-claim` is gone from the CLI, catalog, and schema registry. `repository.prepare` JSON has no `skills` key. `docket schema <id>` equals `docket schema --operation <id>`, and a mismatch refuses.
- **Skills:** run `go generate ./internal/assets/` so the embedded copies match; existing skill guards stay green. Whole-repo grep shows no remaining `refresh-claim` instruction, no "returns the new record revision" claim, no `evidence.verify` call in implement-next, and no Step 6 `change.reconcile` writing `adrs:`; the `docket-adr` dispatch payload names the producing change.
- **Whole suite** at the build gate, as always.
- **After merge (optional, for the human):** re-run the transcript tally used for this spec on the next several implement-next runs; expect no self-inflicted `contended`, no context refusal, no failed `evidence.verify`, and about two prepares per run.

## Decisions worth an ADR

- `context.implementation --id` serves the run's in-progress change; selection stays proposed-only (amends the "claimable exactly as a claim requires" contract).
- The claim lease is stamped only by claim, reconcile, and resume-halted; there is no explicit refresh, because reclaim already refuses while a branch or workspace exists.
- Post-claim revisions are looked up from the local metadata checkout, not carried in receipts; the exact-revision check guards the lookup-to-write window.

The builder decides which of these need an ADR through the normal Step 6 path.

## Left to the builder

- Exact wording and placement of every skill edit, within the rules above.
- Whether `claimed_at`/`reconciled` join the context bundle (2).
- How `docket schema` spells the positional argument in its signature and help.
- Test placement and names.
