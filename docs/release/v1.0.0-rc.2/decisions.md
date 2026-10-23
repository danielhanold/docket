# v1.0.0-rc.2 — decisions

## Phase 0 (2026-10-23)
- Human attested: no /loop or autonomous docket session running against this repo. Decided by: Daniel, 2026-10-23.
- No change other than 0579 in-progress; `gh pr list --state open` returned nothing. Held PRs: none.

## Candidate
- SHA: 4a64bc3bbb66f0368cc3e8d30d4031a596922f45 (ct 1792716396) "docs: cover missing prepare context, stray-marker leftovers, and non-git directories in docket-export"
- `git ls-remote origin refs/heads/main`: 4a64bc3bbb66f0368cc3e8d30d4031a596922f45
- `docket status --json`: zero error findings, zero warning findings; no in-progress change before the claim.

## Source added since rc.1
- `git log --oneline v1.0.0-rc.1..4a64bc3bb`: 267 commits
- `git diff --stat v1.0.0-rc.1 4a64bc3bb -- . ':!docs'`: 485 files changed, 34223 insertions(+), 7408 deletions(-)

## Phase 2 (2026-10-23)
- Workflow run 39952716960 at the candidate: success, every job green; evidence and checksums verified; no STOP lines. BUDGET WATCH lines recorded in candidate/run.txt (screening only).

## Phase 3 — coverage statement
No harness test and no public-install check, by the human's decision of 2026-10-23. rc.2 is the build the human tests with. The candidate is gated by the full suite in Phase 2. The downloader is unchanged since rc.1, whose public-install check proved no quarantine attribute and a valid signature.

## Phase 4 approval (2026-10-23)
- Human (Daniel) approved release-notes.md and gave the explicit "publish". origin/main re-probed: still the candidate.
