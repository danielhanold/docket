<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0145 — docket-status SKILL.md restates a stale check count and list the 0111 guard does not pin](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0145-docket-status-skill-md-restates-a-stale-check-count-and-list.md)**
<!-- docket:backlink:end -->

# docket-status SKILL.md stale check restatement — results
Change: #0145 · Branch: feat/docket-status-skill-md-restates-a-stale-check-count-and-list · PR: <url> · Plan: docs/superpowers/plans/2026-07-28-status-skill-stale-check-restatement-plan.md · ADRs: none

## Findings

**The restatement had test-side dependents the plan under-counted.** The plan's *Verified starting
state* corrected the spec's "no other test changes" claim once, and was still short by two. Beyond
the three asserts in `tests/test_board_checks.sh` it named, deleting the invocation block also
reddened one assert in `tests/test_docket_metadata_branch.sh` (the vocabulary-presence loop — the
deleted block held the file's only occurrence of "metadata working tree") and one in
`tests/test_results_artifact.sh` (it grepped a sentence that lived inside the deleted
`broken-plan-results` bullet). Both were resolved by relocating the invariant to where the content
actually lives rather than by re-adding text: a paraphrase in SKILL.md's Convention section was
corrected to the convention's canonical term, and the results assert was repointed at
`scripts/board-checks.md`, which already carries the same carve-out reasoning. The behavioral
invariant behind the repointed assert is independently covered by
`tests/test_board_checks.sh`'s `broken-plan-results silent for an implemented change` case.

The generalizable bit: **a restatement accumulates its own guards.** Deleting one is not a one-file
edit, because tests reach into the copy rather than into the source — which is itself an argument
for the change's thesis.

**The guard's named limitation is real and now shipped in the comment.** Whole-branch review
mutation-confirmed it: re-adding the check-id list under a *new* heading escapes the section-scoped
guard (the non-vacuity anchor catches a *rename of* `### Health checks`, not a new section
elsewhere). The plan recorded this and the first commit did not carry it into the guard's header
comment; a follow-up commit added it, so a later reader cannot read the guard as stronger than it
is. The file-wide alternative stays rejected — it would redden `### Merge sweep`'s legitimate
`publish-deferred` prose.

**Mutation evidence.** All five plan cells were run by the implementer and independently re-run by
both reviewers, matching predictions each time — including cell B, where the *negative* assert goes
green (the section extracts empty under a heading rename) and only the positive non-vacuity anchor
reddens. That inversion is why the anchor exists.

No ADR: the decision to remove rather than pin a fifth surface is recorded in the change and spec,
and sits inside conventions ADR-0012 and ADR-0054 already govern.

## Follow-ups

- **Change 0154** (minted by auto-capture): audit the rest of the `skills/` tree for the same
  restatement class — closed vocabularies, flag lists, and counts copied out of the contracts that
  own them. 0145 closed exactly one section in one file by design.
- Deferred minor, not blocking: the repointed assert in `tests/test_results_artifact.sh` greps a
  prose substring that sits on a line-wrap boundary in `scripts/board-checks.md`, so a reflow of
  that paragraph would redden it spuriously. Lateral rather than a regression (the assert it
  replaced had the same fragility against SKILL.md) and it fails loudly, never vacuously. Anchoring
  on the `broken-plan-results` section head instead would harden it.
- Deferred minor, not reachable today: the guard's `awk` extractor terminates on `/^(#|##|###) /`,
  so a fenced code block whose line begins `# ` inside `### Health checks` would shrink the guarded
  region silently. The rewritten section is fence-free, and the non-vacuity assert reddens loudly if
  the section ever empties.
