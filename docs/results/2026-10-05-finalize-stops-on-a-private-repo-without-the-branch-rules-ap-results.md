<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0525 — Finalize stops on a private repo without the branch-rules API, and leaves half-removed workspaces](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-05-0525-finalize-stops-on-a-private-repo-without-the-branch-rules-ap.md)**
<!-- docket:backlink:end -->
# Finalize stops on a private repo without the branch-rules API, and leaves half-removed workspaces — Results

**Human action:** No action is required before merge. One optional walkthrough is offered below for anyone who wants to see the private-repository merge path work against a real GitHub repository.

## Outcome

Two finalize failures from the alpha.1 acceptance run are fixed, and a third gap in what a blocked cleanup reports is closed.

- **Private repositories without branch rules now merge.** Before merging, finalize checks which merge methods the repository and its base branch allow. On a private repository whose GitHub plan has no branch rules, GitHub answers the branch-rules request with a 403 saying "Upgrade to GitHub Pro or make this repository public to enable this feature." Finalize used to treat that as an unreadable policy and stopped with `merge-probe-unknown`. It now reads that exact answer as "this branch has no rules", picks the method from the repository settings alone, merges, and adds a `branch-rules-unavailable` warning to the merge result. The finalize skill copies that warning into the closeout notes. Any other failed branch-rules read (a different message, a different status, a 404, a 5xx, unreadable output) still stops as unknown.
- **A worktree removal Git started is now finished.** `git worktree remove` first runs its own clean check. If deleting the folder then fails partway (for example, Finder writing a `.DS_Store` during the delete), Git still drops its registration and exits non-zero. Docket used to read this as "Git refused", leave the workspace marked ready, and block on every later run, keeping both feature branches. Docket now lists the worktrees again after a failed removal. If the path is still registered, Git really did refuse, and the workspace stays blocked and untouched. If the registration is gone, Docket finishes deleting the recorded `.worktrees/<slug>` folder, marks the workspace cleaned, and removes the branches as usual. If the folder still cannot be deleted, cleanup finishes anyway and reports the leftover path in a `workspace-remnant` warning. The finalize cleanup result and the maintenance sweep both show that warning. ADR-0140 records this as the one case where Docket deletes a folder by path, a narrow exception to ADR-0035.
- **A blocked cleanup names what blocked it.** The `workspace-blocked` message now lists the paths or reasons, for example "(stray-notes.txt)".

## Verification performed

- Each task was built test-first with focused tests. Each new assertion was mutation-checked: the guarded behavior was removed, the test went red, and the code was restored. This covers the plan-gate match (status, message, and every result that carries the flag), the merge warning, the finish, remnant, and re-list-error branches of cleanup, each check that decides whether a path is still registered, the sweep's remnant reporting, and the blocked-message contents.
- The cleanup tests run real Git in temporary repositories. They break Git's delete partway through with a read-only, gitignored directory, which reproduces the 2026-10-05 failure (exit 255 after the registration was removed).
- The full suite (`go run ./cmd/docket development test`) passed through the build gate.
- Whole-branch review (deep tier): 4 findings (1 important, 3 minor), all fixed in-branch. The full table is in the PR body.

## Known issues and follow-ups

### A remnant warning is reported once, not on later runs

**When it occurs:** Cleanup finished a removal Git had started, but some of the folder could not be deleted. **What you see:** That cleanup run, whether from finalize or from the maintenance sweep, reports a `workspace-remnant` warning naming the folder. A later cleanup of the same change returns `already-clean` and does not mention the folder again, because the cleaned record does not store the leftover path. **Impact:** A leftover `.worktrees/<slug>` folder, which may hold gitignored files, can sit unnoticed if nobody reads the first warning. **Status:** Confirmed by design. The spec ruled out a durable record of a half-removed folder. **Workaround:** Delete the folder named in the warning. `ls .worktrees` shows any folder with no matching worktree. **Suggested next action:** None unless leftover folders turn up in practice.

### A crash between Git's failed removal and Docket's finish leaves today's stuck state

**When it occurs:** The Docket process dies in the moment after `git worktree remove` fails and before Docket finishes the delete. **What you see:** Cleanup keeps reporting the workspace as blocked, as before this change. **Impact:** The same manual cleanup as today: delete the folder and both branches by hand. **Status:** Accepted at grooming (ADR-0140). The window is very small.
