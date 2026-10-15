<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0551 — Check out a change built on another machine into its own docket worktree](../changes/active/0551-check-out-a-change-built-on-another-machine-into-its-own-doc.md)**
<!-- docket:backlink:end -->

# Check out a change built on another machine into its own docket worktree — results

**Human action:** Optional. The full suite passed on the final head. One Important check is worth doing before merge: try a real checkout of a change built on another machine.

## Outcome

`docket workspace checkout --id <id>` is new. It puts an `implemented` change into its own docket-owned worktree at `.worktrees/<slug>` on any machine. It first checks that the open PR and the freshly fetched remote branch agree on the head, plus `--head` when that is given. Then it does one of these:

- attaches a branch that exists only on the remote;
- reuses a local branch that is already at that head;
- fast-forwards a local branch that is behind. It attaches the worktree first, so git's own in-use check runs before any ref moves.
- removes a foreign worktree that holds the branch, but only when a fresh set of removal proofs passes and git's clean-only remove succeeds.

Anything else is refused with a plain remedy and leaves everything untouched. The finalize skill now runs the checkout before any step that reads the workspace, in every gate mode. `finalize rebase` now reports a missing or foreign workspace as `workspace-not-ready` with the real reason, instead of `workspace-probe-failed`. ADR-0149 records when docket may adopt local state it did not create. The guide and the finalize concept doc describe the new command.

## Human actions and testing

- **Important — check out a change built elsewhere, on a real second clone.** Why: the integration tests use temporary repositories and a fake PR adapter. A run against GitHub has not been done.
  - Prerequisites: an `implemented` change with an open PR, and a second clone of the repo that has never built that change.
  - Steps: in the second clone, run `docket workspace checkout --id <id> --json`. Expect `disposition: attached` and a worktree at `.worktrees/<slug>` whose HEAD equals the PR head (`git -C .worktrees/<slug> rev-parse HEAD`). Run the same command again and expect `existing`.
  - Variant: first run `gh pr checkout <n>` in a separate `git worktree add` directory, leave that worktree clean, then run the checkout. Expect `attached` plus a `foreign-worktree-removed` finding, and expect the old directory to be gone.
  - Cleanup: `git worktree remove .worktrees/<slug>`, then `rm -rf .git/docket/workspaces/<slug>` in that clone. Delete the local branch if you don't need it.
  - If skipped: the GitHub PR-probe path is covered only by the fake adapter.
- **Optional — finalize on the second machine.** In that clone, run `docket-finalize-change` for the change (gate `off` or `local`). Expect the checkout to run first and finalize to proceed normally.

## Verification performed

- The full suite (`go run ./cmd/docket development test`) passed after the build, and again on the final head after the review fixes.
- Each new guard was mutation-tested by its worker: the removal proofs, the head agreement checks, divergence, the primary-checkout refusal, the resume ancestry check, the attach-before-fast-forward ordering, the remedy gating and the disposition fill. Each mutation turned its test red.
- Budget report: no `SERIAL CONFIRMED OVER BUDGET` lines. Several pre-existing `BUDGET WATCH` / `PARALLEL-SENSITIVE` lines appeared in parallel runs, none of them for the new checkout shards.
- Review: a deep review ran and returned 7 findings (0 blocker, 2 important, 5 minor). Six were fixed in-branch. One, about other code this branch did not change, is reported below.

## Known issues and follow-ups

- **`evidence recertify` and finalize's forward-refresh path still give the old `workspace-not-ready` message.** When: you run `docket evidence recertify` for an implemented change on a machine without its docket workspace, for example after a follow-up commit made on a second machine. What you see: "the feature workspace is "absent", not the clean registered feature state", with no reason and no remedy. Impact: low. The workaround is to run `docket workspace checkout --id <id>` first. Confirmed by reading the code (`internal/app/evidence_recertify.go`). The forward-refresh arm needs an owned receipt, so it is rarely reached. Suggested next action: reuse `workspaceNotReadyMessage`'s logic there. No existing change fits (checked 27). This is a new change for a human to capture, related to #551.
- **Ignored files in a removed foreign worktree are deleted.** Build output and dependencies in a foreign worktree that checkout removes go with the folder. The finding says so. This is by design (ADR-0149) and is not a defect.
