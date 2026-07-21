---
slug: decide-and-act-on-the-same-copy
hook: "Decide from the copy you will actually act on — a gate that inspects the local worktree while the action reads a remote ref can pass on bytes that are never the ones published."
topics: [git, scripts, gates]
changes: [83]
created: 2026-07-21
updated: 2026-07-21
promotion_state: candidate
promoted_to:
---

## Apply
When a script *decides* something about a file and then *acts* on that file, both halves must read
the **same copy**. The dangerous shape is subtle because both reads look correct in isolation:

```sh
[ -f "$META_WORKTREE/$change_path" ] && has_marker "$META_WORKTREE/$change_path" && clear_marker   # decides from the LOCAL worktree
git checkout "$metaref" -- "$change_path"                                                          # acts on the REMOTE ref
```

The local worktree and the remote ref agree almost always, which is exactly why this ships. They
diverge when the worktree is stale, when the path is mis-resolved, when a concurrent agent has
moved the file, or when the worktree is simply missing — and in every one of those cases the gate
evaluates against bytes that are not the bytes the action will use. Worse, the common failure is a
**silent skip**: a missing local file makes the predicate false, the guarded block is skipped with
no diagnostic, and the script exits 0 having done the thing the gate existed to prevent.

Two rules follow:

1. **Resolve the decision against the copy the action will consume.** If the action reads
   `origin/<branch>`, the gate reads `git show origin/<branch>:<path>`, not the working tree.
   Route both through one variable so they cannot drift apart later.
2. **An unresolvable input to a gate is a hard error, never a false predicate.** `[ -f "$x" ]`
   returning false conflates "the condition genuinely does not hold" with "I could not check" —
   and those need opposite responses. Distinguish them explicitly and `die` on the second.

The same shape appears wherever a decision is made about a moving target: see
[[cas-re-read-fresh-origin]] for the retry-loop face of it (re-reading your own pending write) and
[[moving-base]] for the design-time face (planning against a snapshot the base has left behind).

## War story
- 2026-07-21 (#83, PR #114) — Found in the very script the change exists to make trustworthy.
  `terminal-publish.sh` decided whether to clear the `publish-deferred` marker by inspecting
  `$META_WORKTREE/$change_path`, then built its copy-set from `$metaref` **eight lines later**. With
  a stale, missing, or mis-resolved metadata worktree the clear was skipped with no diagnostic and
  the script **exited 0 having published a marker-carrying record onto `main`** — a terminal record
  on the integration branch permanently announcing that its own publish never completed. That is
  precisely the gap #83 was written to close, reproduced live inside #83's own implementation. The
  decision is now authoritative with respect to the copy that will actually be published, and an
  unresolvable local file is a hard error rather than a silent skip. Worth noting *why* it survived
  review this long: the two reads are eight lines apart and each is individually idiomatic — the
  defect exists only in the relationship between them, which no single-line review catches.
