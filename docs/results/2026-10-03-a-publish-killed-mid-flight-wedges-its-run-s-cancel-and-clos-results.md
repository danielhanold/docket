<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0494 — A publish killed mid-flight wedges its run's cancel and closeout](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0494-a-publish-killed-mid-flight-wedges-its-run-s-cancel-and-clos.md)**
<!-- docket:backlink:end -->
# A publish killed mid-flight wedges its run's cancel and closeout — results

**Human action:** None required to merge. One optional walkthrough below exercises a real killed publish end to end.

## Outcome

`pr.publish` and `workspace.publish` now hold a per-entry kernel lock (`publish-<token>.lock` beside the run record) from before their `admitted` journal entry is written until after the outcome is written. A single classifier (`classifyAdmittedMutation` / `accountAdmittedMutations`) decides whether a journal entry blocks cancel, terminal repair, resume, and the success closeout:

- `completed` is accounted.
- `uncertain`, or `admitted` whose lock probes free, is accounted with the informational finding `mutation-abandoned:<op>`.
- `admitted` with a held lock, a missing lock file, a probe error, or no `lock_token` still blocks as `mutation-pending:<op>`, as before.

`settleUncertainPublications` first rewrites a dead publisher's `admitted` entry to `uncertain`, so a later verified identical retry still settles it as `mutation-settled:<op>`. A lock failure at admission never refuses the publish; the entry is journaled without a token and behaves as before.

Tasks: typed `process.ProbeLock`; publish-lock primitives and classifier with the additive `AdmittedMutation.LockToken`; a real SIGKILL process test; admission holding the lock across the remote call; cancel/terminal-repair/resume and settlement; the success closeout; comments, glossary (`mutation-abandoned`) and run-tracker concept paragraph.

0444's no-retry tests were rewritten and renamed to the new rule, never deleted (for example `TestIntegrationRunCancelUncertainWithoutRetryIsAbandoned`, `...CompleteSuccessfulRunAbandonsUncertainWithoutRetry`).

## Verification performed

- Each task ran RED then GREEN focused tests, and the plan's mutation checks (held read as free, missing read as free, dropped admitted-to-uncertain step, lock released before the outcome write, refusal path not releasing) each turned the named test red and passed once restored.
- The runcancel, runfence, runcompletion, runverdict, runstart and concurrency (-race) integration shards passed during the build.
- The full suite runs at the build gate; its evidence lives in the PR body.

## Human actions and testing

- **Optional — kill a real publish and cancel.** Prerequisites: a scratch repository with docket installed and a tracked run that reaches `workspace.publish`. Steps: start the run with `run.start`, interrupt the process (Ctrl-C or `kill -9`) while `workspace.publish` is pushing, then run `run.cancel --key <key> --reason test`. Expected: disposition `cancelled` with finding `mutation-abandoned:workspace.publish`, and `run.start --resume <id>` admits one replacement. Cleanup: delete the scratch branch and repository.

## Known issues and follow-ups

- **A dropped publish callback reads as a dead publisher.** If code holding a publish's `done` callback lets it become unreachable before calling it, garbage collection closes the lock file and the entry reads as abandoned. Production callers call it immediately after the remote work; two racing tests needed `runtime.KeepAlive`. Confirmed as a test hazard, not seen in production. Suggested action: keep this in mind when adding new journaled publications.
- **Entries written before this change** carry no `lock_token` and still wedge the old way until their publisher writes an outcome. None exist on the build machine.
