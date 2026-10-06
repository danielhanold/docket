<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0532 — Implement private visibility for PRs, commits, and shipped files](../changes/active/0532-private-visibility-keep-docket-out-of-prs-commits-and-shippe.md)**
<!-- docket:backlink:end -->

# Implement private visibility for PRs, commits, and shipped files — Results

**Human action:** No action is required to merge. One optional end-to-end walkthrough in a scratch private repository is described below for anyone who wants to see the leak check refuse a push.

## Outcome

Before this change, a private-visibility repository still left docket traces on its shared remote: marker blocks and a change line in PR descriptions, marker-bearing PR comments from finalize, and change ids or docket words in commit messages, the shipped spec copy, and code.

Now, in a private repository:

- PR descriptions are exactly the authored prose. The spec copy ships without its change line.
- A finalize block is recorded on the change only, with no PR comment. Backlink repointing after merge skips private repositories.
- A **blocking leak check** runs before every feature-branch push (`workspace publish`, `finalize publish`) and every PR create or edit (`pr publish`). It scans each outgoing commit's message and added lines and paths (per commit, so text added and later removed is still caught), the feature branch name, and the PR title and description. A hit returns a `leak-detected` refusal naming the commit, file, line and matched text, and nothing is pushed. The decision is recorded as ADR-0144.
- `leak_check.match_word: false` lets the bare word "docket" through for codebases where it is a domain word. Markers, trailers, paths, `dckt`, and zero-padded change references are still blocked.
- `repository check`, `repository prepare` and `workspace publish` report a `docket`/`dckt` branch or a `refs/docket/` ref on origin (report only). Pushes now refuse any ref outside `refs/heads/`, so finalize's internal refs can never reach the remote.
- Skills and worker agents carry a writing rule for private repositories, and every feature-branch writer, including finalize's rebase resolver and integration-repair worker, now receives the repository's visibility in its dispatch.

Shared repositories are unchanged and never run the scanner.

## Human actions and testing

### Optional — see the leak check refuse a push

Lets you watch the check work in a real private repository. Integration tests already cover this.

Prerequisites: a docket build from this branch, and a scratch repository initialized in private mode with a bare `origin`.

1. Groom and claim a change, then add a commit to its feature worktree whose subject ends in `(0001)`.
   Expected: `docket workspace publish --id <id> --head <sha> --json` returns `leak-detected` and lists that commit, and `git ls-remote origin` shows no feature branch.
2. Reword the commit to remove the id and publish again.
   Expected: the publish is applied.

Cleanup: delete the scratch repository.

## Verification performed

- Full suite (`go run ./cmd/docket development test`) green at the final head after the review fixes, recorded as build evidence. Three PARALLEL-SENSITIVE screening lines on the first gate run, for shards this change does not touch; no serial-confirmed breach.
- Every task and fix added mutation-tested coverage: removing the scanner call before each push site turns a test red, and the shared-mode no-scan behavior is pinned.
- Whole-branch review (deep tier): 3 findings (1 blocker, 2 important), all fixed in-branch; full table in the PR body.

## Known issues and follow-ups

### Raised test-shard budget

The `app_workflowlifecycle` integration shard grew from about 55s to about 71s, measured serially, because of the new private-mode leak tests. Its budget in `tests/runtime-budgets.tsv` was raised to 80s. This is confirmed and has no user impact. If the shard keeps growing, splitting it is the natural next step. In-scope note, no backlog verdict.
