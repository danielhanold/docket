<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0463 — Resume gate-armed line is ambiguous when no epoch exists — dispatch context gets passed as --run-epoch](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0463-resume-gate-armed-line-is-ambiguous-when-no-epoch-exists-dis.md)**
<!-- docket:backlink:end -->
# Resume gate-armed line is ambiguous when no epoch exists — Results

**Human action:** Review the PR. Look closely at one design point before merging: `run cancel` now accepts a resumed run's verified change id as cancel authority. Nothing else needs a human.

## Outcome

Before this change, resuming a change whose earlier run was never armed (`docket run gate-before implement-next --resume <id>` with no prior run epoch) printed a two-token line, `gate-armed <key> <dispatch-context>`. The documented line has three tokens, `gate-armed <key> <epoch> <dispatch-context>`. Both values are 32-hex strings, so the parent read the dispatch context as the epoch and passed it as `--run-epoch`. The gate then refused with a generic `invalid-request`, and the resumed run finished with no epoch linkage and no cancel coverage. That is what happened on change 0382.

What changed:

- **A resume with no prior epoch now mints one.** The epoch is bound to the change id and the verified feature worktree, so every armed gate carries an epoch and the `gate-armed` line always has three tokens.
- **An unknown `--run-epoch` gets a named refusal,** `unknown-run-epoch`, with a hint that names which token of the armed line goes where. This covers `gate drive start`, `gate drive prepare-scope` (which checks only that the epoch exists, not that it is live), and `agent enter`. `agent enter` now checks before it launches anything, including when `--run-epoch` is passed without `--run-gate-key`. A mismatched epoch there reports the existing `stale-run-epoch`.
- **Changes beyond the spec, made because of review findings:**
  - `run cancel` accepts the resume-verified owner. That is a gate record attributed to a change, with no claim-binding file, and the change must match the epoch's own change id. Without this, the new resume epoch could never be cancelled, and every later `--resume` of that change was stuck behind `resume-active-run`. Epochs from the older cancelled-run replacement path can now be cancelled too. An unconfirmed claim reservation still refuses `claim-unconfirmed`.
  - An epochless resume now refuses `resume-active-run` when a live epoch already owns the feature worktree, before it mints anything. This stops two live epochs from blocking all fenced mutations (such as PR publish) in one worktree.

## Human actions and testing

### Important — sign off on the widened `run cancel` authority

ADR-0111 made a confirmed claim binding the authority for cancelling a run. This change adds a second accepted shape: a gate record attributed to change N with no binding file, cancelling an epoch whose change id is N. Without it, resumed runs cannot be cancelled at all. It is still a policy change on a fail-closed boundary, and no separate ADR records it. If you skip this review, the risk is that the widening covers a record shape you did not intend.

1. Read the authority step of `runCancel` in `internal/app/rungate_cancel.go`, and `TestIntegrationGateCancelRunCancelResumeAuthorityFailsClosed` in `internal/app/rungate_before_resume_integration_test.go`.
   Expected: the resume shape is accepted only when no binding file exists and the attributed id equals the epoch's change id. Every other case refuses as before.
2. Decide whether this needs an ADR update note. If it does, record one with `docket adr` after merging.

## Verification performed

- The full build suite (`go run ./cmd/docket development test`) passed on the pre-review head `d9d0c4ac`: 54 of 54 files. The post-review certification run on the final head is recorded in the PR's build-evidence block.
- Every task and review fix ran a focused RED/GREEN cycle through the gate driver. Each new guard was mutation-checked: stripping the fix turned its test red.
- Review used the deep rung and returned 1 blocker, 3 important and 1 minor finding. All were fixed in the branch (see the PR's disposition table). There was no second review round after the fixes.

## Post-review changes (2026-09-28)

A human review of the widened `run cancel` authority found three problems. The branch was then rebased onto `main` (one test moved to `testsupport.TempDir` after change 0462 removed `gateTempDir`), and two were fixed:

- **A stray claim no longer brings back the cancel wedge.** A claim made under a resume arm's dispatch context used to write its reservation before refusing, or could claim a different change. Either left the resume epoch uncancellable (`claim-unconfirmed` or `claim-mismatch`). `change claim` now refuses a resume context as `gate-context-conflict` before it writes anything. The resume-verified shape is one predicate, `GateRecord.resumeAttributed`, shared by claim, cancel, and the verdict.
- **Concurrent resume arms are serialized.** The race below was not small: 12 of 12 simultaneous arms each minted a live epoch in test. A per-change resume lock (under `<git-common-dir>/docket/rungate-resume/<id>`) now covers the arm from the prior-epoch check through the bind, so exactly one arms and the rest refuse `resume-active-run`.
- **Kept on purpose: an undispatched resume arm blocks the next resume until cancelled.** The resume epoch is bound when armed so that a second agent cannot enter the worktree before the first reaches its first gate step. Binding later would reopen that window, and the dispatch context is never stored, so a repeat arm cannot reprint it. The earlier resume-after-cancel path already behaves this way. Nothing records whether an agent is using an epoch, so the `resume-active-run` refusal now names both remedies: cancel if it was never dispatched or its agent exited, `run gate-verdict` if it is still running.

## Known issues and follow-ups

### A small race window remains between concurrent epochless resumes

**Fixed after review** (see *Post-review changes*): a per-change resume lock now serializes resume arms. The original finding follows.

Two `--resume` arms of the same unarmed change started at the same instant can both pass the new worktree-owner check and each mint an epoch. Epoch locks are per gate key, not per change. If that happens, later resumes refuse as `resume-epoch-unreadable`, and fenced mutations in that worktree refuse until one epoch is cancelled with `docket run cancel`. After that cancel, the change's own later resumes still see both epochs and keep refusing. This is suspected, not observed, and needs a true simultaneous double arm. Suggested next step: a follow-up change adding a check-after-bind re-scan, in which a racer that sees two owners unbinds its own epoch.

### Cancelled epochs that were never superseded still count as live

`FindEpochByChange` counts a cancelled epoch that was never superseded as live. This predates the change and only matters after the double-mint race above. It needs no action unless that race is seen in practice.
