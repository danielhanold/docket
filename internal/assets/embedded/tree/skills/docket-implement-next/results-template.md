<!-- results-template.md — close-out artifact for a change. OPTIONAL: write one only when at least
     one is true: (a) the human must run interactive/manual checks at the merge gate beyond automated
     tests, (b) the build surfaced findings worth recording (incl. any that became ADRs), or
     (c) there are follow-ups / notable plan deviations. Otherwise skip it — the PR + green CI are the
     receipt. Authored in the feature worktree and committed on feat/<slug> (a build artifact, like the
     plan); keep build-receipt detail in the PR description, not here. -->
# <title> — results
Change: #<id> · Branch: feat/<slug> · PR: <url> · Plan: <path> · ADRs: <ids>

## Verify (human)

<!-- GENUINELY MANUAL checks for the merge gate — things no automated test can reach. Each item
     PENDING until checked. A fixed finding never belongs here: the fix plus the green suite is its
     verification, and the PR body's disposition table is where its outcome is read. -->
- [ ] …

## Findings

<!-- Discoveries during the build; note which became ADRs. Delete if none. -->

## Follow-ups

<!-- Deferred items / new proposed changes. Delete if none. -->
