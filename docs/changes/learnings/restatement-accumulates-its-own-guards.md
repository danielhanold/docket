---
slug: restatement-accumulates-its-own-guards
hook: "Deleting a restatement is never a one-file edit — tests grep the COPY, not the source, so the copy has quietly become load-bearing."
topics: [docs, testing, refactoring]
changes: [145]
created: 2026-07-28
updated: 2026-07-28
promotion_state: candidate
promoted_to:
---

## Apply
Before deleting a duplicated list, count, or paraphrase, **grep the test suite for the prose you are
about to remove** — not for the source it restates. Asserts reach into whichever copy was nearest
when they were written, so a restatement silently acquires dependents that no one intended and that
its own change's "no other test changes" claim will under-count.

When you find such a dependent, the fix is **relocation, not restoration**: repoint the assert at
the artifact that actually owns the content, or correct the paraphrase to the canonical term. Do not
re-add the deleted text to keep a grep green — that reinstates the very duplication you are removing.
Check that the *behavioral* invariant behind a repointed assert is covered somewhere real, so the
repoint is a move rather than a loss.

Read this as an argument for the deletion, not against it: content that N tests grep from a copy is
content whose ownership was already ambiguous.

## War story
- 2026-07-28 (#145, PR #135 — merged) — Removing a stale check-count-and-list restatement from
  `skills/docket-status/SKILL.md`. The spec claimed "no other test changes"; the plan corrected that
  once and was **still short by two**. Beyond the three asserts in `tests/test_board_checks.sh` it
  named, deleting the invocation block also reddened `tests/test_docket_metadata_branch.sh` (its
  vocabulary-presence loop — the deleted block held the file's only occurrence of "metadata working
  tree") and `tests/test_results_artifact.sh` (it grepped a sentence living inside a deleted bullet).
  Both were resolved by relocation: the paraphrase was corrected to the convention's canonical term,
  and the results assert was repointed at `scripts/board-checks.md`, which already carried the same
  carve-out reasoning — with the behavioral invariant independently covered by
  `test_board_checks.sh`'s `broken-plan-results silent for an implemented change` case.

  Two honesty notes from the same change worth copying. The guard shipped **naming its own
  limitation in its header comment**: re-adding the list under a *new* heading escapes a
  section-scoped guard, and the first commit had left that out of the comment, so a follow-up commit
  added it rather than letting a later reader over-trust it ([[marker-scoped-guard-needs-a-population-floor]]).
  And the mutation matrix included a cell where the *negative* assert goes green under a heading
  rename and only the non-vacuity anchor reddens — that inversion is the whole reason the anchor
  exists. The wider audit of this restatement class across `skills/` is #0154.
