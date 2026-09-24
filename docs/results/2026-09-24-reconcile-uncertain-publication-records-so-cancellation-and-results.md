<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0444 — Reconcile uncertain publication records so cancellation and resume can finish](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0444-reconcile-uncertain-publication-records-so-cancellation-and.md)**
<!-- docket:backlink:end -->
# Reconcile uncertain publication records so cancellation and resume can finish — Results

**Human action:** No required human action before merge. Automated unit, production-boundary, and race tests cover the behavior. One optional hardening follow-up, making the workspace adapter refuse a moved head, is listed under Known issues for human triage.

## Outcome

A PR or workspace publish can fail in a way where Docket does not know if the external effect landed, for example when a transport error hits after the request was sent. The run gate journals such an admission as `uncertain`. Before this change an uncertain entry stayed uncertain for good. `run.cancel` stayed at `cancellation-pending` and a successful attributed closeout stayed at `completion-unaccounted`, even after a later identical retry had published the exact same thing and succeeded.

Now each publication admission journals a typed identity descriptor. For a PR that is repository, head branch, head commit, base, and digests of the title and body. For a workspace publish it is repository dir, remote, feature ref, and head commit. Only digests of authored prose are stored, never the raw text. An uncertain entry is settled to `completed` only when a later retry in the same epoch journal has an identical descriptor, completed, and verified its postcondition. "Verified" means the adapter returned applied or no-op, and for a workspace publish the head it actually pushed equals the requested head. A retry that was refused, contended, or failed internally, or that pushed a moved head, never counts as proof. Settlement is a compare-and-swap write under the epoch lock. It runs in exactly two places: run-cancellation teardown and the attributed successful closeout (keyed `run.gate-verdict`). An AST-based guard test pins those two callers. Read-only paths (`run.verify`, the unattributed observe verdict, and resume quiescence checks) never write.

## Verification performed

- Every plan task and review fix followed TDD, with RED/GREEN runs through the gate driver. Each new guard was mutation-tested: dropping the verified requirement, verifying every result, matching outside the lock, swallowing the settlement write failure, removing the cancel or closeout wiring, and adding an unauthorized caller all turned the relevant tests red.
- Race tests (`-race -count=3`) cover concurrent settlement, racing completion callbacks, and new admissions. No admission is lost and no entry is downgraded. Interruption tests show a failed settlement write leaves the cancel pending, and a repeat cancel converges to `cancelled`.
- A deep whole-branch review found 1 blocker, 2 important, and 1 minor finding. All four were fixed in-branch. The blocker was that unverified retries counted as proof. The important findings were the workspace moved-head case and indirect acceptance-3 coverage. The minor finding was stale comments. The PR body has the disposition table.
- Full-suite build gate `go run ./cmd/docket development test` passed before review (53/53 files). The branch is re-certified after the fixes and this results file; the PR build-evidence block has the head and result. The budget report showed only parallel `BUDGET WATCH:` screening lines and no `SERIAL CONFIRMED OVER BUDGET:` line.

## Known issues and follow-ups

- **Workspace adapter does not refuse a moved head (suggested hardening, not a defect).** If a commit lands on the feature branch between the app-level head check and the push, `WorkspacePublish` pushes the newer head. This change makes sure such a publish can never settle an uncertain entry, because the entry is journaled unverified. The push itself still happens, though. Passing an expected head into `workspace.PublishRequest`, so the adapter refuses a moved head the way the PR path's `ExpectedHead` already does, would prevent it. Impact is low, the race window is small, and the gate outcome is already safe. Next action: human triage; capture it with `docket change create` if wanted.
