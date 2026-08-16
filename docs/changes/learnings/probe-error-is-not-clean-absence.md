---
slug: probe-error-is-not-clean-absence
hook: "A probe that errors and a probe that cleanly reports 'not there' are different answers — collapsing them makes the destructive branch the one that fires when the system is least understood."
topics: [cleanup, error-handling, resources]
changes: [309]
created: 2026-08-16
updated: 2026-08-16
promotion_state: candidate
promoted_to:
---

## Apply
Cleanup code almost always asks a registry a yes/no question — *is this thing still registered?
still mounted? still referenced?* — and then branches: unregister it properly, or, if it was never
registered, just delete the leftovers directly. The bug is in how the *error* return is folded into
that boolean. Written the obvious way,

```go
list, err := ListWorktrees()
if err != nil {
    return false   // "not registered"
}
```

an error and a clean not-found produce the same answer, and the answer they produce is the one that
authorizes the direct delete. So a transient failure — a lock contended, a binary missing from PATH,
a partially-written state file — routes straight to the branch that destroys the evidence and
orphans the registration, and it does so *precisely* when the system's state is least knowable.
The registration then survives with nothing left on disk behind it, which is often exactly the state
no reclaim path can repair: the pruner is looking for a directory that a cleanup already removed.

The rule: **a probe has three outcomes, not two — present, cleanly absent, and unknown — and
"unknown" never shares a branch with "absent" when the other branch is destructive.** On unknown,
*retain*. Leave the candidate in place, emit a warning that names it as pending, and let a later
pass ask again with better luck. Retaining costs a stale directory and one log line; deleting on a
bad read costs a resource leak no automated path can reclaim. The asymmetry is the whole argument,
and it holds every time cleanup is involved, because cleanup is the one context where the cheap
answer to a failed question is also the irreversible one.

Two corollaries worth stating, because they are where this gets missed in review:

- **The type system will not help you here.** `(bool, error)` collapsed to `bool` at a single call
  site is a one-line, entirely idiomatic-looking mistake, and every test that exercises the happy
  path and the genuinely-absent path passes. The test that catches it is the one that *injects a
  probe failure* and asserts the resource still exists afterward. If a cleanup path has no such
  test, assume it has this bug.
- **"It's just a transient error, it'll retry" is backwards.** The retry reruns the cleanup, which
  re-probes, which may succeed — but the leak already happened on the first pass, and the second
  pass now sees a registration with no directory, which is a different and worse input than it
  started with. Errors during cleanup compound state; they do not defer it.

Related: [[absent-target-certifies-permission]] — the same collapse one layer up, where a missing
config layer is read as a fully-defaulted one and absence produces the most permissive verdict.

## War story
- 2026-08-16 (#309, PR #211 — merged) — The isolated metadata transaction engine registers a real
  detached git worktree per transaction under `<common-dir>/docket/transactions/`, and its cleanup
  path asked `worktreeRegistered` before deciding how to reclaim a candidate. That helper treated
  **any** `ListWorktrees` error as "not registered," and the caller then removed the candidate
  directory directly — leaving a `.git/worktrees/<name>` entry with no worktree behind it, which
  `PruneAbandoned` could never reclaim because it scans for directories. Every test passed: the
  registered path worked, the truly-unregistered path worked, and the one input nobody had written
  a case for was `git worktree list` failing. It was caught by the deep whole-branch review as the
  single *important* finding on a 44-file, ~10k-line branch — the other three were minor — which is
  the part worth remembering: this class does not look like a bug in the diff. It looks like
  ordinary error handling, it sits in the least-read function in the change, and it only manifests
  as a leak on a day when something unrelated is already going wrong. The fix separated the list
  error from a clean not-found and retained on uncertainty with a `cleanup-pending` warning, plus a
  regression test that injects the list failure (commit `5f9cacbe`).
