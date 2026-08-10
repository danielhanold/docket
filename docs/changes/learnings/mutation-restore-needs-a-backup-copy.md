---
slug: mutation-restore-needs-a-backup-copy
hook: "`git checkout -- <file>` restores to HEAD, not to your uncommitted edit — as a mutation-test restore step it silently destroys the work being tested and produces a meaningless reading."
topics: [testing, git, mutation]
changes: [226, 231, 270]
created: 2026-08-07
updated: 2026-08-10
promotion_state: candidate
promoted_to:
---

## Apply
A mutation test is three steps: break the guarded thing, run the assert, restore. The reflexive
restore idiom is `git checkout -- <file>` — and it is wrong for the case that matters, because the
mutation is nearly always performed on a file you have **just edited and not yet committed**.
`git checkout --` restores to **HEAD**, so it discards the edit under test along with the mutation.

What follows is worse than losing work, because the run keeps going. The suite now executes against
the pre-change file, so the asserts fail en masse — a large, confident, entirely meaningless red
reading. The natural next move is to debug the *asserts*, which are fine.

**Back up and copy back:**

```
cp "$f" "$f.bak"; <mutate>; <run assert>; mv "$f.bak" "$f"
```

Restore-to-backup is correct whether or not the edit is committed, so it is the idiom to write into
plans unconditionally rather than the one to reach for only when you remember the state. Two
adjacent tells that the reading is bogus rather than real: the failure count is far larger than the
mutation's blast radius, and asserts unrelated to the mutated claim are among the failures.
Related: [[plan-supplied-test-code-is-unverified]] — this arrived as plan-supplied *procedure*,
which gets even less scrutiny than plan-supplied code.

## War story
- 2026-08-07 (#226, PR #168) — The plan's Step 6/7 mutation procedure specified
  `git checkout -- <file>` as the restore. The file was a reference the task had just rewritten and
  not committed, so the restore reverted the rewrite mid-mutation-test; the following run produced
  roughly 40 failures against the old file's content and briefly read as a genuine regression in
  the change. Later tasks in the same plan used a `cp` backup and behaved correctly. The rewrite was
  recoverable from the editor state, but the diagnostic cost was the real damage.
- 2026-08-07 (#231, PR #170) — The same idiom arrived in a *different* plan five days later, again
  as the restore step in every mutation block, again over prose files the task had just edited and
  not committed. This time it was caught before the first probe ran: every task staged its edit with
  `git add` before mutating, so `git checkout --` restored the staged edit rather than HEAD. Staging
  is a second correct answer, and a cheap one when the worker is committing per task anyway — but it
  works only because `git checkout -- <file>` reads the index, so it silently reverts to being wrong
  the moment a plan step mutates before staging. The `cp` backup has no such precondition. The
  recurrence is the point: a plan-supplied restore idiom does not get re-scrutinized per plan.
- 2026-08-10 (#270, PR #193) — the **application** failure mode, sibling to this finding's restore
  failure mode: the same three-step procedure, broken at step 1 instead of step 3. A fix worker
  re-ran a mutation probe with a `perl -0pi` one-liner that died on a syntax error *before writing
  anything*. The subsequent run came back all-green — which, read naively, is the worst available
  reading: "the guard survived its mutation, so the guard is vacuous," when in fact no mutation had
  been applied and the guard was fine. Nothing in the output distinguishes "mutated and still green"
  from "never mutated." The procedure now gates every probe on `git diff` showing the changed line
  before believing any reading. Generalize: **a mutation test must prove the mutation landed before
  it interprets the result** — assert on the diff, or on a checksum change, never on the mutating
  command's exit code alone (a `perl`/`sed` one-liner that fails to match its pattern exits 0 having
  changed nothing). Restore correctness and application correctness are two independent failure modes
  of one procedure, and each produces a confidently wrong reading in the opposite direction: a broken
  restore fabricates a red, a broken mutation fabricates a green.
