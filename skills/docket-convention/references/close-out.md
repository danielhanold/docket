# Close-out — the shared per-change sequence

> Single source for the close-out sequence a final transition (`done` or `killed`) runs:
> archive → re-render `## Artifacts` → cleanup → board. Every driver routes through this file:
> `docket-finalize-change`'s per-change close-out, `docket-status`'s merge sweep, and
> `change.complete` for a `results-artifact` change (the three `done` drivers), plus the kill
> callers — `docket-implement-next`'s reconcile-kill and `docket-new-change`'s proposed-kill. The
> sequence is one; only the failure posture differs per caller (table below). This file owns
> ordering and posture. `schema` gives request-file and result shapes; the capability catalog
> gives flags.

Contents: [The sequence](#the-sequence) · [Failure posture](#failure-posture--per-caller) · [Determinism invariant](#determinism-invariant)

## The sequence

All metadata writes happen in the metadata worktree (`metadata_worktree_path` in the `repository.prepare` context),
synced to the metadata remote (`metadata_remote`) before the first read; every commit pushes
immediately.

1. **Archive on `docket` first.** The final outcomes split here: a merged `done` runs the Go
   `finalize.closeout` transaction; a completed `results-artifact` change runs the Go
   `change.complete` transaction; `killed` runs the Go `change.kill` transaction (`finalize.closeout`
   covers neither of the other two).

   **Done drivers** (`docket-finalize-change`'s close-out, the `docket-status` merge sweep) archive
   through the typed transaction. There is **no caller-supplied date**: it derives the UTC archive
   date from the verified GitHub `mergedAt` **inside** the transaction (never `now()`):

   ```
   finalize.closeout  --id <id> [--input <notes.json>]   # resolve argv from the capability catalog
   ```

   `--input` carries only the optional authored closeout notes (`verification_outcomes`,
   `late_findings`; `-` for stdin); the `results:` file already sits on the metadata branch and is
   never passed. Trust the typed outcome: `done-archived` (or `stacked-merged` / `root-archived` for a
   stack) ⇒ the change is marked done and relocated to the dated archive path — idempotent if
   already archived, including across a day boundary. This ONE metadata commit atomically owns the
   archive move, the `## Artifacts` re-render, the re-stamp of **every metadata-resident back-link**
   (spec, plan, and results all live on the metadata ref), and the inline board render; a typed
   refusal or process failure writes nothing and aborts per the caller's posture, with **no partial
   caller-owned follow-up**. It still relocates the change file in its own step, so concurrent done
   drivers converge tree-identically (see *Determinism invariant*). Steps 2 and 4 below run no
   separate pass for this path.

   **Kill drivers** (`docket-implement-next`'s reconcile-kill, `docket-new-change`'s proposed-kill)
   drive the typed `change.kill` operation transaction. There is **no caller-supplied date**: it
   derives the UTC archive date from its own transaction clock **inside** the transaction (never a
   caller `now()`). Author the non-empty `## Why killed` section body and pin the exact record
   submitted for the kill — its `path` and opaque record `revision`, both from the caller's
   authoritative context read — into a bounded JSON request file (`-` for stdin):

   ```
   # request-file: { "change_id": <id>, "path": "<changes_dir>/active/<UTC-birth>-<id>-<slug>.md",
   #                 "revision": "<revision>", "why_killed": "<why>" }
   change.kill  --repo-dir <metadata_worktree_path> --request <request-file> --json   # resolve argv from the capability catalog
   ```

   Trust the typed outcome: `applied` ⇒ archived. Kill is **not idempotent**: a retry after a
   success finds the record already archived and is refused, writing nothing — read that refusal
   against the archived record, never as a failed kill. This ONE metadata commit
   atomically owns the archive move, the refreshed `updated:` date, the spliced `## Why killed`
   section, the `## Artifacts` re-render, the retargeted spec, plan, and results back-links, and the inline board render
   — so the step-2 re-render and step-4 board pass below carry **nothing** for the kill path, exactly
   as `finalize.closeout` owns them for the done path. A wrong `revision` or an illegal source status
   returns a typed refusal that writes nothing (a lost conflict-checked write is `contended`; see
   *Determinism invariant*).

   **Complete driver** (a `results-artifact` change — `docket-implement-next`'s results-artifact
   ending, or a person finishing one) drives the typed `change.complete` operation transaction,
   which owns its own archive exactly as kill does. Pin the record from the `status` operation
   (with `--json`) into a bounded JSON request file (`-` for stdin):

   ```
   # request-file: { "change_id": <id>, "path": "<record path>", "revision": "<revision>" }
   change.complete  --repo-dir <metadata_worktree_path> --request <request-file> --json   # resolve argv from the capability catalog
   ```

   It refuses, writing nothing, unless the change is a `results-artifact` change, `proposed` and
   build-ready or `in-progress`, with no `pr:`, and with a FINAL-valid results artifact attached.
   Trust the typed outcome: `completed` ⇒ done and archived, dated by the transaction's own UTC
   clock; `already-completed` ⇒ an earlier call already did it (a safe retry after a lost
   response). The one metadata commit owns the archive move, the cleared claim, the removed
   `## Run halted` section, the `## Artifacts` re-render, the retargeted back-links, and the inline
   board render — so steps 2 and 4 carry nothing for this path either. It touches no branch,
   worktree, or GitHub; cleanup (step 3) follows it.

2. **Artifact block + back-links — owned atomically by step 1, no separate caller commit.**
   Every close-out transaction — `finalize.closeout` (done), `change.kill` (kill), or
   `change.complete` (complete) — re-renders the archived record's `## Artifacts` block (relative
   same-branch rows, plus an absolute `Spec (merged)` row for a merged change) **and** retargets
   every metadata-resident `docket:backlink` block — spec, plan, and results — to the
   now-**archived** change path, **in the same step-1 metadata commit** as the archive.

   So **no separate caller re-render or back-link commit runs for any path** — the skill never
   invokes a facade renderer and never hand-edits a managed block. Nothing touches the integration
   branch. A typed refusal (malformed markers, missing artifact) leaves the file untouched and
   aborts per the caller's posture — surface it.

3. **Clean up the feature branch + worktree.**

   ```
   finalize.cleanup  --id <id>   # resolve argv from the capability catalog
   ```

   Trust the typed outcome. Ownership is proven **inside the transaction**: only
   workspaces and feature refs proven owned by *this* closed change are removed — the local ref
   only when its recorded tip is detached from every worktree AND contained in the verified merge
   chain, the remote ref only under an exact old-value lease with no open child PR still targeting
   it — never the metadata worktree, the primary tree, or any out-of-tree path. Any
   resource whose ownership it cannot prove is **retained**, not force-removed. A branch kept
   because deleting it could lose work (moved after the merge, commits off the integration branch,
   checked out elsewhere) is named in the `pending` result with the command to delete it by hand
   after review (`git branch -D <branch>`; `git push origin --delete <branch>` for the remote); if
   a live change now uses that name, it says so and advises no delete. The warning shows in
   `finalize cleanup` and sweep human output, and every full sweep retries until the branch is
   gone — except a branch a live change now uses, or a blocked or unknown workspace. A carried
   stacked child is retried like any other `done` change; until its work is proven on the
   integration branch it stays `pending` (`carry-unproven`) with no delete advice. A failure aborts per the caller's posture.

   **Complete path (no PR):** on a `done` `results-artifact` change, cleanup makes no GitHub call
   and never touches a remote ref (the branch was never published). It removes a clean feature
   worktree and deletes the local branch by its recorded name only when the branch has no commits
   of its own (its tip is contained in `origin/<integration_branch>`); uncommitted files, commits
   on the branch, or an unprovable probe keep everything and report `pending`. A change completed
   straight from `proposed` has no branch, and nothing present on this machine reads
   `already-clean`. A `pending` cleanup never undoes the completion; the full sweep
   retries cleanup while its worktree or local branch is still there, unless a live change now
   uses that branch or the workspace is blocked or unknown.

   **Kill path:** cleanup removes only a merged `done` change's resources. On a `killed` change it
   removes nothing and returns `no-op` with disposition `retained` and reason `killed-retained` —
   a success, not a failure, so the kill caller continues. Any feature worktree or branch a
   reconcile-killed change already had stays in place; remove it by hand if it is no longer wanted —
   the branch is the pre-kill `branch:` value (`<type>/<slug>` by default; the kill clears the
   field) and `git worktree list` finds the worktree. A `proposed`-kill has none.

4. **Board refresh — owned atomically by step 1, no separate pass.** Every close-out transaction
   renders the inline `BOARD.md` **inside its own step-1 metadata commit**, so **no separate Board
   pass runs**. The step-1 transaction is the sole writer; a caller never hand-renders the board,
   invokes a board renderer, or follows the typed mutation with a second board commit. `BOARD.md`
   is the live planning view and is never published to the integration branch.

## Failure posture — per caller

The posture on a failed step-1 transaction is the caller's (steps 2 and 4 are absorbed into
step 1):

| Caller | Posture |
|---|---|
| `docket-finalize-change` (single-change close-out) | **abort-and-report** — stop this change's close-out, surface the failure |
| `docket-status` merge sweep (bulk janitor) | **log-and-continue** — abandon the remainder of this change's close-out, move to the next change; the next sweep self-heals idempotently |
| `docket-implement-next` reconcile-kill | trust each exit code; a failure aborts the kill and is surfaced before returning to selection |
| `docket-new-change` proposed-kill | same as reconcile-kill — surface and stop; nothing else is in flight |
| `docket-implement-next` results-artifact ending | a `change.complete` refusal halts the run; a `pending` cleanup goes into the run report and never halts it |

**Step-failure propagation:** step 1's atomic transaction commits all of it **together** or
writes nothing (fail-closed), so no partial step-2 or step-4 follow-up is left to skip.
Step 3 (cleanup) follows the caller's own skill body: the sweep treats it as best-effort (log and
continue; a later pass self-heals); other callers keep their own posture (abort-and-report).

## Determinism invariant

Two agents both driving the same final transition converge through the step-1 transaction's
exact-revision conflict check: one applies and the other reads `contended` (a lost race), re-runs
the `repository.prepare` operation, and re-reads authority rather than racing a second write. The archive
date is the transaction's own UTC clock (never a caller `now()`), so a replay after a lost response
reuses the same dated filename. Every derived view (`## Artifacts` block, back-links, inline board)
is regenerated deterministically inside that one commit — on a rebase conflict in generated content,
**regenerate, never 3-way merge**.
