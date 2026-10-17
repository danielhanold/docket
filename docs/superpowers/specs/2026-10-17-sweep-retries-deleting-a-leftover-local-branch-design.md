<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0562 — Sweep retries deleting a leftover local branch](../../changes/active/0562-sweep-retries-deleting-a-leftover-local-branch.md)**
<!-- docket:backlink:end -->

# Sweep retries a leftover local branch, and names a branch it keeps — design

Change #562, groomed 2026-10-17.

## Why

When `finalize cleanup` removes a done change's worktree but the local branch delete then fails, the full maintenance sweep never tries again. Before dispatching cleanup for an old done change, the sweep's snapshot assessment (`sweepAssessLocalRefLeg` in `internal/app/maintenance_assess.go`) asks only whether some worktree still has the feature branch checked out. It never asks whether the branch exists. So the most common leftover goes unseen: workspace tombstone clean, remote branch gone, local branch still there. The sweep reports `snapshot-no-work` on every run. Change #559 found this for results-artifact changes, and PR changes have the same gap. The #390 sweep design already called for this check ("Record exact local/remote feature-ref presence from the inventories"; dispatch when "a ref remains after an already-clean workspace"), but only the remote half was built.

There is a second, smaller problem. When cleanup deliberately keeps a branch because deleting it could lose work, the message ("the local feature ref moved off the merged head; it is retained") names no branch and gives no next step. The sweep's one-line summary shows only counts, so the message sits unread in JSON.

`finalize cleanup --id N` run directly already retries the local delete on every run (`finalizeCleanupLocalRef`, `finalizeCleanupLocalRefNoPR`). Its retry needs no change. The only change it gets is the new warning wording below.

## Goals

1. The full sweep dispatches a normal cleanup for a done change whose recorded local feature branch still exists, for both `pr` and `results-artifact` deliverables.
2. When cleanup keeps a branch on purpose, the warning names the exact branch and tells the human to review it and delete it by hand, with the exact git command.
3. That warning is visible in the human-readable output of both `finalize cleanup` and `maintenance sweep`, not only in JSON.
4. The docs say again that the sweep retries a leftover local branch.

## Non-goals

- Changing when closeout first tries cleanup, or any cleanup safety proof (tip, ancestry, worktree-detached, exact-lease).
- Branches of `killed` or `stacked-merged` changes. Both are deliberately kept by design and get no warning.
- Screening a kept branch out before dispatch (option B, rejected). The decided behavior (option A) is that every full sweep re-dispatches cleanup for a change whose branch still exists, and the warning repeats until the human deletes the branch. A leftover remote branch already behaves this way today.
- The implementation-scope sweep (preflight). It defers historical cleanups and stays unchanged, including its deferred count.
- Stacked children whose PR merged into their parent's branch ("carried descendants"). In the normal stacked flow, cleanup always refuses them with `destination-mismatch` and keeps both branches. Nothing has failed: docket has no step that deletes them yet. That gap belongs to its own change (see *Related*). This change leaves them exactly as they are today, with no new dispatch and no new warning.

## Anti-goals

- A failed or partial local-branch listing must never read as "branch absent". It makes the local-ref leg unknown, the same posture as `remoteHeadsErr`.
- No per-change git or GitHub probe added to the assessment. The local-branch inventory is one read per sweep invocation, gathered only when there are done candidates, exactly like the existing shared inventories.
- No "delete it by hand" advice for a transient failure the next retry will probably fix.

## Design

### 1. Shared local-branch inventory

- Add a `gitcli.Client` method that lists every local branch head (`refs/heads/*`) with its object id in one git process, for example `git for-each-ref --format='%(refname) %(objectname)' refs/heads/`. It returns a map keyed by full ref, parsed and validated strictly like `ListRemoteHeads`: malformed lines, duplicates, or invalid object ids are an error, never a partial map.
- `gatherSweepSharedFacts` reads it once next to `ListRemoteHeads` / `ListWorktrees` into new `sweepSharedFacts` fields (a `localHeads` map plus a `localHeadsErr`). The existing "no git client or repository is wired" failure sets the new error as well.

### 2. Local-ref leg keyed on presence

`sweepAssessLocalRefLeg` marks work when the feature ref is present in `localHeads`. It marks the leg unknown when `localHeadsErr` is set. Presence covers the old "checked out in a worktree" case, since a checked-out branch exists. Whether the worktree-list check stays as a second signal is the builder's call, as long as no case that is actionable today stops being actionable. The leg applies to both deliverables, as today.

**Carried descendants are excluded from the new presence trigger.** A done PR record whose PR merged into a branch other than the integration branch must not become actionable merely because its local branch exists. Its cleanup would refuse `destination-mismatch` on every sweep and add GitHub calls for nothing. The batched PR read (`SweepPRSetResult.Facts`, which the sweep currently drops after taking the bodies) already carries each PR's `BaseRef`. Use it where gathered. Where it is not gathered (a private repository skips the batch) or a number is unresolved, treat any done record carrying `stacked_on:` as a possible carried descendant and skip the presence trigger for it. That is today's behavior, never a new dispatch. The existing worktree-checkout and remote-ref triggers stay exactly as they are for these records. The exact mechanism is the builder's call; the rule is "no new dispatch for a record whose cleanup will refuse `destination-mismatch`".

### 3. Loud, named warning for a branch kept on purpose

These cleanup reasons mean "kept because deleting could lose work, or because something else holds it". Each one's finding message names the branch and gives the remedy:

| Reason | Path | Message shape |
|---|---|---|
| `recorded-tip-moved` | local, PR | `local branch <b> was kept: it moved off the merged commit. Review it, then delete it by hand (git branch -D <b>).` |
| `tip-not-in-merge-chain` | local, PR and results-artifact | `local branch <b> was kept: it has commits that are not on <integration>. Review it, then delete it by hand (git branch -D <b>).` |
| `branch-checked-out` | local, PR and results-artifact | `local branch <b> was kept: it is checked out in another worktree. Review that worktree, then delete the branch by hand (git branch -D <b>).` |
| `remote-ref-moved` | remote, PR only | `remote branch <b> was kept: it moved after the merge. Review it, then delete it by hand (git push origin --delete <b>).` |

`<b>` is the recorded short branch name (for example `feat/foo`).

**Reused branch name — no delete advice.** Nothing stops a later change from minting the same branch name as an old done change. When any non-final record in the same snapshot (proposed, in-progress, blocked, deferred, implemented, stacked-merged) records the same branch, the warning drops the delete command and says the branch is in use instead, for example `local branch feat/foo was kept: change #N is now using it.` (The same goes for the remote form.) This check reads the snapshot cleanup already loads (`closeoutContext.snap`) and adds no probe. *Designer's call*, so the warning never tells a human to force-delete another change's live branch.

The exact wording follows the house finding style: what happened, why it matters, the remedy, in plain words. Reason codes and the `pending` disposition stay as they are. The `docket-status` skill relays finding text, so the wording is the deliverable.

The following stay quiet, with today's retryable wording: every probe or fetch failure (`ref-probe-failed`, `ancestry-probe-failed`, `worktree-list-probe-failed`, `remote-ref-probe-failed`, `child-pr-probe-failed`), `local-ref-delete-failed` (the transient case #559 found, which the retry fixes), and `remote-lease-rejected` (a lost race; the next sweep sees `remote-ref-moved` and warns then). `children-retarget-required` keeps its own disposition and message, because deleting that branch would break an open child PR.

### 4. Visible in human output

- `CleanupOpResult.HumanText` also prints each kept-branch warning, the way it already prints `workspace-remnant`.
- **The sweep entry must carry the warning text.** Today `sweepRunCleanup` copies only the cleanup's top-level `Message` (a generic sentence on a pending PR cleanup) and `Reason` (only `findings[0].Code`) into the entry, so a kept-branch finding never reaches the entry. Do it the way `workspace-remnant` already is: append each kept-branch finding's message to `entry.Message`, joined with `"; "`. Leave `entry.Reason` as today, and if `Reason` is empty, set it to the kept-branch code. The JSON entry's fields are unchanged; only the message text grows. *Designer's call*, matching the existing remnant precedent rather than adding a field.
- `MaintenanceResult.HumanText` appends one line per kept-branch warning, naming the change id and the warning, after the count summary. When both the local and the remote branch are kept, which is the usual result of committing and pushing after the merge, both lines appear. How the sweep recognizes a kept-branch finding (a reason-code set, or a marker on the finding) is the builder's call. The set is the four reasons in the table above.

### 5. Docs

- `skills/docket-convention/references/close-out.md` ("the sweep retries a leftover worktree only") and `docs/reference/glossary.md` (the `maintenance.sweep` entry, "a completed `results-artifact` change's leftover worktree"): restore the promise that the sweep also retries a leftover local branch, for every done change. Mention the named kept-branch warning where the docs describe cleanup's `pending` outcomes. Docs describe current behavior only, with no change citations. Run `go generate ./internal/assets/` after skill edits.

## Constraints

- Every sweep dispatch still goes through a fresh prepare plus `FinalizeCleanup` with all its live proofs. The inventory screens for work and never authorizes a delete.
- The #390 request-budget tests (`maintenance_traffic_integration_test.go` and similar) still hold: adding done records whose branch is absent adds no per-item request, and the new inventory is one local git process per invocation.

## Testing

- Carried descendant: a done record whose batched PR facts show a non-integration `BaseRef`, with its local branch present, gets no new dispatch. The same holds for a done `stacked_on:` record when the batch is not gathered. Both keep today's assessment outcome.
- Reused branch name: a kept-branch warning for a done record whose branch a live record also names carries no delete command.
- Sweep entry: a pending PR cleanup with a `pr-backlink-pending` finding ahead of a kept-branch finding still carries the kept-branch text in `entry.Message`, and a local-plus-remote kept pair carries both.
- Assessment: local branch present with a clean tombstone and no remote leads to dispatch, for both deliverables. Local branch absent leads to `snapshot-no-work` as today. A failed local-branch inventory makes the leg unknown (`snapshot-unknown`), never no-work. A similarly named branch (`feat/foo-2` against `feat/foo`) does not match. Mutation-test the presence check: strip it and watch the leftover-branch test go red.
- Parser: an empty repository with no heads gives an empty map. A malformed line, a duplicate, or a bad object id gives an error.
- Cleanup messages: each of the four kept-on-purpose reasons carries the branch name and the git command, and the quiet reasons do not. A `HumanText` test for `finalize cleanup` and for `maintenance sweep` shows the warning line.
- End to end (integration fixture): close out a change, make the local delete fail once (or recreate the branch after cleanup), then run a full sweep. The branch is deleted. Commit to the branch after the merge, then sweep. The branch is kept and the sweep's human output names it.

## Related

- #559 found the gap (its results file, "The sweep does not retry a leftover local branch").
- #390 designed the snapshot assessment this change completes.
- #567, "Cleanup deletes a stacked child's branches once its code reaches main", holds the carried-descendant gap this change deliberately leaves alone.

## Left to the builder

The gitcli method name and parsing helper, whether to keep the worktree check alongside presence, how kept-branch findings are tagged for the sweep summary, and the final message wording within the shape above.
