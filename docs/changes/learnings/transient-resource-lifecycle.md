---
slug: transient-resource-lifecycle
hook: "A fixed-name scratch resource must self-heal from an interrupted run's leftover, and teardown must not eat the diagnostics the failure path still has to read."
topics: [shell, scripts, cleanup]
changes: [62]
created: 2026-07-17
updated: 2026-07-17
promotion_state: retained
promoted_to:
---

## Apply
A script's transient scratch resource — a worktree, a lock dir, a temp checkout — has two failure
modes that only appear on the unhappy path, so the happy-path suite never sees either.

**Self-heal before create, don't just clean up after.** If the resource has a FIXED name, a run
interrupted before teardown leaves it behind, and every FUTURE run then dies on "already exists" —
one crash wedges the tool permanently. Force-remove any leftover immediately before creating it
(`git worktree remove --force` / `rm -rf` then create), rather than trusting that the last run got
to its own teardown. Cleanup-on-exit is best-effort by nature; the create path is the only place
you control.

**Order teardown after the failure path reads.** Teardown deletes the very files the error branch
needs to explain itself (a captured stderr log, an exit-code file). Capture the diagnostic into a
shell variable BEFORE calling teardown, then report — or the failure surfaces with an empty reason
exactly when the reason matters most. Trace each error path to see what it reads and when teardown
runs relative to it; `trap`-based teardown makes the ordering easy to get wrong because it fires
where you are not looking.

Both need a test that INTERRUPTS: a leftover-resource fixture for the self-heal, and a forced
failure asserting the diagnostic reached the output.

## War story
- 2026-07-17 (#62, PR #94) — `setup-auto-approve.sh` builds a transient worktree under a fixed name
  to commit the workflow template onto the integration branch. Two bugs, both only on the unhappy
  path: (1) a run interrupted before teardown left the worktree behind, and because the name is
  fixed, `worktree add` then failed for **every subsequent run** — the tool wedged itself until a
  human hand-removed the directory; fixed by force-removing any leftover before `worktree add -B`,
  with a leftover fixture to pin it. (2) `teardown` deleted `.push.err` before the push-failure path
  read it, so a rejected push (the workflow-OAuth-scope case the script exists to explain) would
  have reported its hint from an already-deleted file; fixed by capturing the message before
  teardown. Both were invisible to a green suite: every passing test ran to completion, which is
  precisely the case neither bug occurs in.
