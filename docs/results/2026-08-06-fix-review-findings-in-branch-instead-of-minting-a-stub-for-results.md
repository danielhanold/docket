<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0218 — Fix review findings in-branch instead of minting a stub for every one](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0218-fix-review-findings-in-branch-instead-of-minting-a-stub-for.md)**
<!-- docket:backlink:end -->

# Fix review findings in-branch instead of minting a stub for every one — results
Change: #0218 · Branch: feat/fix-review-findings-in-branch-instead-of-minting-a-stub-for · PR: (opened at step 7) · Plan: docs/superpowers/plans/2026-08-06-fix-review-findings-in-branch.md · ADRs: 66, 70

## Verify (human)

- [ ] Decide whether the fix loop's **five non-blocker fix tasks per run** cap is the number you want. It was chosen against auto-capture's 3-mints precedent, and this very run would have hit it (see Findings) — so the first number docket ships is one this run already exceeded.
- [ ] Decide whether Tier C's authorizing knob for a **fix** dispatch should stay `skills.build: auto`. Fix workers run `docket-build-task` at docket-build's profiles, so it is the closest existing knob, but there is no `skills.fix` key and the mapping is an inference rather than a stated contract.

## Findings

**This change was dog-fooded on itself at the human's instruction.** The deep review of this branch returned **12 findings (0 blocker, 6 important, 6 minor)**, and instead of the pre-0218 rule (record them in the PR body, mint stubs for the rest), they were run through the fix loop this change builds. All 12 were fixed in-branch across 7 commits, the suite gate went green on its first run, and **0 stubs were minted**. Under the old rule this run would have merged with 12 unfixed findings and minted up to 3 stubs — the exact pattern (changes 0197, 0200, 0220 sitting unbuilt in `active/`) the change exists to end.

**The self-test found four real defects in the change's own design.** Each is a rule the change shipped wrong and the loop then corrected — the strongest available evidence that the loop does something a PR-body record would not have:

- **The blocker floor (→ ADR-0070, commit `c2055a47`).** Character routing silently weakened the one gate 0218 insists must never weaken. Pre-0218 every blocker fix ran `standard` then escalated to `premium` before halting; under pure character routing a blocker whose fix *looks* mechanical routes `economy`, escalates once to `standard`, and halts — `premium` is never tried. The ceiling matched the old ladder; the floor did not, and the file claimed equivalence it did not have. Fixed by giving blockers a floor at `standard`, stated explicitly as the one deliberate exception to the character/severity orthogonality. Recorded as **ADR-0070** together with the never-`max` ceiling, since the two bounds are one decision surface.
- **A green branch could ship a `result: red` evidence block (commit `e0e0f371`).** The red path's post-revert re-run said only "Green → proceed", never revoking the red record the first run wrote. The PR body's `docket:build-evidence` block — which `docket-finalize-change` reads — would have carried `result: red` for a branch that was green. Fails safe (finalize just runs the suite) but publishes a false record.
- **The revert had no ordering and no conflict posture (commit `eeaa27c7`).** Nothing fixed the order of blocker/important/minor tasks, so a blocker fix landing after an important fix in the same region would make the revert conflict, leaving an autonomous run with a half-applied revert in a shared worktree. The "the branch can never end worse than the green build that entered it" guarantee rested entirely on that revert succeeding. Fixed with blockers-first ordering plus a halt posture that restores the worktree.
- **"Bounded" bounded everything except the work (commit `3f494679`).** Escalations and suite runs were capped; the number of fix tasks was not. A deep review of a large diff returns ten-plus findings, so Step 6 could expand without limit.

**Three guards in this change were decoration, each caught by mutation-testing rather than by running the suite.** All three would have passed forever while guarding nothing:

- The relocated 0184 routing guard could only ever match a trigger sharing a line with the `max` token — the rubric is hard-wrapped and `grep` is line-based, so most of the defect surface was invisible. This was true of the pre-extraction original too. Replaced with a bounded `awk` slice plus a non-vacuity companion.
- The plan's `skills.review` collision fixture was decorative: `config_block_header` rejects the leaf for **two** independent reasons (indented **and** valued), so relaxing either left the fixture green — and relaxing both also left it green, since the would-be block has no `min_fix_severity` leaf. Replaced with a coexistence fixture and an indented-valueless fixture, the latter being the shape only the column-0 check rejects. The plan's prose asserting "only the column-0 requirement" was factually wrong and was corrected in the resolver comment and `scripts/docket-config.md`.
- The materiality-bar assert stayed green after deleting the rule-bearing clause, because the next sentence independently matched. Replaced with a proximity shape.
- The disposition-state loop confirmed vocabulary, not the table: all four state words occur in surrounding prose, so deleting every table row left it green. Replaced with row-shaped asserts inside an extracted slice.

**The plan itself carried defects the workers had to correct.** Its mutation commands interpolated shell `$` as Perl variables and silently corrupted a target file (one rewrote a guard to `[[ "" == *:* ]]`); its `git checkout` restore steps destroyed uncommitted work and errored on untracked files; it expected `NOT OK -` from a suite whose `assert` prints `FAIL -`, so its verification greps would have silently found nothing; and its Task 1 Step 5 byte-diff instruction conflicted with its own Task 1 Step 3 content. Every worker read the line back after mutating rather than trusting exit status, which is what caught these.

**One second-order consequence surfaced late.** Answering the `unverified-build-state` question (that self re-run sits *outside* the two-run bound, because a run that consumed it could otherwise revert but never re-verify) raised Step 6's real ceiling and made README's freshly-corrected suite-run arithmetic an undercount. Corrected in the same commit rather than shipping a numeric contradiction between two paragraphs of the same section.

## Follow-ups

- **`README.md` prose accuracy is not machine-checked beyond two keyed phrases.** The suite-run arithmetic can drift again if the fix loop's bound or finalize's skip conditions change, without reddening anything. No stub minted — it is a property of an existing guard's reach, not distinct beyond-the-branch work.
- **`m3` and `m4`'s new prose in `fix-loop.md` is unguarded** (the `unverified-build-state` bound answer and the Tier C dispatch clause). A later edit can drop either without a test noticing.
- **Budget margins on this change's files are thin.** `skills/docket-implement-next/SKILL.md` sits at 3844/3850 words, and `skills/docket-build/references/task-routing.md` at 48/500 lines-words against 50/500. The next edit to either will likely need a raise.
- **No stubs were minted this run** (`AUTO_CAPTURE_ENABLED=true`, cap 3, 0 consumed, 0 policy-suppressed, 0 dedup skips). Every discovery above is either fixed in this branch or a property of it, which is precisely what 0218's narrowed materiality bar now requires.
