<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0203 — Define the per-step git-state postcondition docket-implement-next now names but never states](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0203-define-the-per-step-git-state-postcondition-docket-implement.md)**
<!-- docket:backlink:end -->

# Define the per-step git-state postcondition — results

Change: #0203 · Branch: feat/define-the-per-step-git-state-postcondition-docket-implement · PR: (see change `pr:`) · Plan: docs/superpowers/plans/2026-08-06-per-step-git-state-postcondition.md · ADRs: none

## Verify (human)

- [ ] Read the `### Step postconditions` table as a **user of the skill**, not as a diff. The whole change is prose that only works if an agent mid-run reads it and acts differently. No automated check can tell you whether the six rows are the ones you would want checked at each step boundary — that judgment is the deliverable.
- [ ] Confirm the governing sentence's read-scoping clause is the rule you want. It now says each row is read **as of the close of its own step**, so a later commit moving branch HEAD leaves an earlier row's `head_sha` stale by design. This is the load-bearing sentence: it is what keeps the table consistent with `references/edge-paths.md`'s expected-staleness rule, and getting it wrong makes the `advanced` disposition unreachable on any run that writes a results file — including this one.

## Findings

Six review findings (rung: `docket-review-standard`), all fixed in-branch; none became an ADR. The PR body's disposition table carries the per-finding outcome and commit SHAs. Three are worth recording beyond that table:

1. **The blocker was a real unreachability bug in the contract, not a wording nit.** As first written, cumulativity plus row 6's `head_sha == HEAD` conjunct made Step 7's postcondition unsatisfiable on the Step-6.5 path — a results commit moves HEAD after the evidence is minted, so the row could never hold at Step 7, and Step 7's row is the sole licence for `advanced`. `references/edge-paths.md` already said that staleness is EXPECTED, so the new table contradicted an existing rule in the same skill. Worth noting that **this very change wrote a results file**, so the defect would have fired on its own run.

2. **The fix generalized past both suggested remedies, correctly.** The reviewer proposed scoping row 6 or exempting it. The fix worker found row 5 had the identical defect (its `head_sha` conjunct is unsatisfiable at Step 6 the moment a fix commit lands — which is precisely why row 6 says "after any fix commits") and qualified the governing sentence once instead, at 30 words and zero exceptions.

3. **"Read from git, never from a sub-skill's report" was literally false for two rows.** The build-evidence record is `docket-build`'s *output*, not a git artifact — under the default `BUILD_CHECKPOINT: false` nothing is persisted at all, and the `true` ledger lives under the gitignored `.superpowers/`. Its durable home is the PR body. Only the `head_sha == HEAD` conjunct is a git fact, which is exactly why the spec called that conjunct load-bearing. The header is now qualified rather than the rows softened.

**Notable plan deviations:**

- The plan's `## Verification` block asserted `grep -c 'git-state postcondition'` would return **2**. It returns **1** — the new section deliberately never repeats the term. Corrected in the plan (finding 6). The consequence is that the proximity-scoped assert in `tests/test_loop_continuation.sh` is the *only* thing pinning that §5's clause is no longer orphaned; it is mutation-proved against the exact pre-change orphan state.
- The plan supplied its guard block as literal shell. Two lines of it could not be used as written: `flatten < f | grep -q` is AGENTS.md's producer-piped-into-early-exiting-consumer hazard under `set -o pipefail` (`tr` takes SIGPIPE, the pipeline returns 141, the assert goes intermittently red), and a comment anchored as `tests/test_docket_review.sh:193` is the filename-plus-line form `tests/test_comment_anchor_style.sh` rejects. Both were caught by execution, not by reading — `plan-supplied-test-code-is-unverified` earning itself.
- The budget row moved twice, not once: `150 3850` → `165 4250` (Task 1) → `165 4300` (fix 2), each re-derived from a fresh `wc` measurement per the rounding rule, with the justification comment extended in place rather than duplicated. The final file measures 162 lines / 4285 words.
- The plan's remaining `## Verification` bullet claiming the branch "touches exactly three files" is stale — the branch now also carries this results file. Left as written: the plan is a point-in-time record.

## Follow-ups

None minted. Every finding was about this branch's own diff, which `fix-loop.md` makes explicitly non-mintable — they were fixed, not deferred to new changes.

One ambient observation, recorded rather than minted because it is not material enough on its own and was not surfaced *by* this change: the suite's test scripts do not share an output convention — some end with a `PASS`/`FAIL` line, some do not — so the exit code is the only reliable oracle for a runner. Anyone writing a suite wrapper should key on `$?`, never on a trailing marker.
