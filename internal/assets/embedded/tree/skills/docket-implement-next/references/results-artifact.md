# results-artifact — the ending for a change that delivers findings, not code

The `docket-implement-next` ending for a change whose `context.implementation` bundle reports
`deliverable: results-artifact`. **Read this right after `workspace.prepare` returns.** It replaces
the rest of Step 4 and Steps 5–7; selection, claim, reconcile, and `workspace.prepare` already ran
as for a `pr` change. Loaded on demand from Step 4; sibling files are not auto-loaded with the
skill.

## What this ending skips

No plan, no build, no PR. Skip `workspace.commit-spec` — the branch must keep no commits of its
own, or cleanup keeps it — plus the plan-writer dispatch, the build role, the build gate, evidence
records, the branch diff review, `workspace.publish`, `pr.publish`, and `change.mark-implemented`
(it refuses a `results-artifact` change). The spec is the plan.

**Refresh the claim** (the `change.refresh-claim` operation, at a revision re-read after a re-sync)
at each phase boundary below, as the `pr` path does: before the work, before review, and before
completion. A refresh `contended` stops new work.

## 1. Do the work

Do what the spec describes, in the feature worktree. **Never commit** to the branch, never publish
it, and never modify the primary checkout. Anything worth keeping goes into the results artifact:
the findings, the numbers, and the commands or short scripts that produced them, inline, so a reader
can re-run them. Delete everything else from the worktree before cleanup.

If the work turns out to need a code change, or something must be kept as a file the results
artifact cannot carry (a long script, a large data file), **halt** through the `change.halt`
operation and say so in the halt report: a person decides whether it becomes a follow-up change with
a PR. End the run `halted`.

## 2. Human steps

When the change or spec says a step needs a human, or the run finds that one does, do the work you
can and halt (`change.halt`) when you reach that step — at the step, not earlier — naming it in the
halt report, so the work so far stays in the worktree for the person. A person finishes the change
by doing the steps, attaching the results artifact, and running the `change.complete` operation.

## 3. Write and attach the results artifact

Under Step 6.5's existing rules, unchanged: the template, the `**Human action:**` statement, the
checkpoint lifecycle and mechanics (`change.attach-results`, same path on every attach), Known
issues and follow-ups, and the backlog match at final consolidation. Step 6.5's PR-body and
gate-evidence references do not apply.

## 4. Review

Re-read the learnings index `<changes_dir>/learnings/README.md` first and pull the hooks relevant
to the work (skipped when `learnings.enabled` is `false`). The reviewer touches no docket metadata,
so everything it reads travels inline.

<!-- docket:feature-dispatch:start targets=docket-review-standard -->
Dispatch `docket-review-standard` foreground — the default tier when there is no build record — and tell it to review in its results-artifact mode, then stop. Its dispatch payload contains:
Feature worktree: <absolute canonical feature-worktree root>
It also contains, inline, the spec body, the results artifact's text, the change title and scope, and the relevant learnings hooks. When the `docket-review` skill is missing, apply the convention's missing-skill rule: review the results inline against the same checks, warning prominently, and dispatch no reviewer. An undispatchable review tier — established only per the convention's *Dispatch-capability resolution* — makes the run **`halt`**.
<!-- docket:feature-dispatch:end -->

## 5. Fix pass

Revise the results artifact yourself for every finding at or above `review.min_fix_severity` (read
through the `diagnostic.config` operation; blockers always), capped by `review.max_fix_tasks`, then
re-attach it at the same path. No fix workers, no suite gate, no second review. Findings below the
threshold go into the results artifact's Known issues and follow-ups; distinct follow-up work goes
into the final report, as on the `pr` path. `## Verification performed` names the review that ran
and how its findings ended.

## 6. Complete, then clean up

1. Run the `change.complete` operation with a request file holding `{change_id, path, revision}`,
   pinned from the `status` operation's `--json` output after a re-sync. `completed` (or
   `already-completed` on a lost-response retry) means the change is `done` and archived; any
   refusal halts, naming its reason.
2. Run the `finalize.cleanup` operation with `--id <id>`. It removes a clean worktree and a local
   branch with no commits of its own, and never touches a remote ref. A `pending` cleanup (files
   left in the worktree, commits on the branch) goes into the run report; it **never halts the run
   and never undoes the completion**.

End the run `advanced`. The final report names the results path and the cleanup disposition, plus
the usual skipped changes and follow-up work with their backlog verdicts.
