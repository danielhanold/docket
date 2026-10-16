<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0553 — Explain repository check and status findings in plain language](../changes/archive/2026-10-16-0553-explain-repository-check-and-status-findings-in-plain-langua.md)**
<!-- docket:backlink:end -->

# Explain repository check and status findings in plain language — Results

**Human action:** Not required to merge. One optional walkthrough is listed below for anyone who wants to see the new wording.

## Outcome

Every record-level finding from `docket repository check` and `docket status` now carries a plain-language message, an "is it harmful" impact line, and a remedy. The build finished at a3ee5f9b7 (10 plan tasks, green on the first full-suite run). All 10 review findings were fixed in-branch (17da9b7e9, 43a703d44, 9ca351f34, 84439f2f3). The first post-fix gate failed only because the disk filled up ("no space left on device"). After space was freed the run was resumed, and the full suite passed at 84439f2f3.

## Human actions and testing

- **Optional: see the new finding text.** Prerequisites: a scratch clone of any docket repository and the binary built from this branch. Steps: in the scratch clone's metadata worktree, change one change record's `priority:` to `urgent` and commit it locally, then run `docket repository check`. Expected: the finding gives a plain sentence about the unknown priority, a line saying whether it is harmful, and a remedy that names the groom revise priority edit. Cleanup: `git reset --hard HEAD~1` in that scratch metadata worktree.

## Verification performed

- Full suite (`go run ./cmd/docket development test`) green at a3ee5f9b7 and again at the final head 84439f2f3 (resumed run, attempt 1). Both budget reports had only BUDGET WATCH and PARALLEL-SENSITIVE screening lines, with no SERIAL CONFIRMED OVER BUDGET line.
- The post-fix gate at 84439f2f3 (drive 9c84c4ac) failed first. Every failure in its log was "no space left on device", so the fixes were kept, not reverted. The resumed gate passed at the same head.
- Review: deep tier (bumped from standard because the diff is 3019 lines). It returned 10 findings: 0 blocker, 3 important, 7 minor. All 10 were fixed. Finding 1 (status showed parse failures raw) was fixed in 17da9b7e9. Finding 2 (hand-edit remedies on mutation refusals) was fixed in 43a703d44. Finding 3 (remedies naming repair when the setup state would refuse it) was fixed in 9ca351f34. Findings 4-10 (wording and fixture details) were fixed together in 84439f2f3.
