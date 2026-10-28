# v1.0.0 — decisions

## Phase 0 (2026-10-27)
- Version-free install docs: PR #446 (plain, untracked docs PR), merged to `main` as 0cf3ab4de2fd81c5799106830d1e28ab60ad3109. `README.md` and `docs/install/install.md` download from `releases/latest/download` with no `--version`; `docs/release/upgrading-from-bash.md` §3 drops `VERSION=` and points at `latest`, §1 says "a later release". No other pinned install command in maintained docs.
- Docs PR gated by the full suite: SUITE files=91 passed=91 failed=0, `test_go_integration_bashupgrade` ok=4 notok=0; no OVER BUDGET line. A first run failed 2 files on a stale ignored `.agents` symlink in the primary checkout (environment, not source); the human had it deleted and the suite was re-run green in a clean worktree.
- Human attested: no /loop or autonomous docket session running against this repo. Decided by: Daniel, 2026-10-27.
- No change other than 0580 in-progress; `gh pr list --state open` returned nothing. Held PRs: none.

## Candidate
- SHA: 0cf3ab4de2fd81c5799106830d1e28ab60ad3109 (ct 1793147258) "docs: point install commands at the latest release"
- Contains the Phase 0 docs merge: `git merge-base --is-ancestor 0cf3ab4de 0cf3ab4de` true.
- `git ls-remote origin refs/heads/main`: 0cf3ab4de2fd81c5799106830d1e28ab60ad3109
- `docket status --json`: zero error findings, zero warning findings; no in-progress change before the claim.
- `gh pr list --state open`: (empty)

## Source added since rc.2
- `git log --oneline v1.0.0-rc.2..0cf3ab4de`: 8 commits (rc.2 evidence commits, the logo 095ec9327, the docs merge 0cf3ab4de)
- `git diff --stat v1.0.0-rc.2 0cf3ab4de -- . ':!docs'`: README.md (7+, 3-), assets/docket-lockup-light.png (new); 2 files changed

## Phase 2 (2026-10-27)
- Workflow run 40609036031 at the candidate: success, every job green; evidence and checksums verified; no STOP lines. BUDGET WATCH lines recorded in candidate/run.txt (screening only).

## Phase 3 — coverage statement
No harness test and no public-install check, by the human's decision of 2026-10-27 (Claude Code and Cursor tested enough through rc.2). The candidate is gated by the full suite in Phase 2. The downloader is unchanged since rc.1, whose public-install check proved no quarantine attribute and a valid signature.
