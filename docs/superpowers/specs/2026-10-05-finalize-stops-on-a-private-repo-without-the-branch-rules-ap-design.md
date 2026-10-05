<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0525 — Finalize stops on a private repo without the branch-rules API, and leaves half-removed workspaces](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-05-0525-finalize-stops-on-a-private-repo-without-the-branch-rules-ap.md)**
<!-- docket:backlink:end -->

# Finalize merges on a plan without branch rules, and finishes a removal Git started

Groomed 2026-10-05 with Daniel. Two small fixes to existing finalize code, plus one visibility fix
found while tracing. Both fixes came out of the alpha.1 acceptance (change 0366). Tracing showed the
stub's explanation was wrong for both, so this spec starts from what actually happened.

## Problem

### 1. The merge-method picker stops on a plan that has no branch rules

Before merging, `finalize.merge` chooses a merge method (change 0336). `MergePullRequest`
(`internal/githubcli/merge.go`) reads the methods the repository allows (`probeRepoMergeMethods`)
and the active rules on the PR's base branch (`probeBranchMergeRules`, in
`internal/githubcli/mergemethod.go`). It intersects the two sets and picks rebase, else merge
commit, else squash. 0336's spec says a failed request counts as unknown, and unknown stops the
merge: finalize reports `merge-probe-unknown` and makes no merge call.

On 0366's private fixture repository (`danielhanold/docket-accept-v1-0-0-alpha-1-claude`), the
branch-rules request fails every time. Re-run on 2026-10-05:

```text
$ gh api repos/danielhanold/docket-accept-v1-0-0-alpha-1-claude/rules/branches/main
stdout: {"message":"Upgrade to GitHub Pro or make this repository public to enable this feature.",
         "documentation_url":"https://docs.github.com/rest/repos/rules#get-rules-for-a-branch",
         "status":"403"}
stderr: gh: Upgrade to GitHub Pro or make this repository public to enable this feature. (HTTP 403)
exit:   1
```

Read the plain way, this response is an answer, not a failed read. Branch rules don't exist on this
plan for private repositories, so no rule can restrict the merge method. The repository endpoint
on the same repository succeeds and allows all three methods. Today every free-plan private
repository stops at this step, and the human has to merge by hand.

### 2. A removal Git started and couldn't finish leaves a workspace cleanup can never clear

`finalize cleanup` removes the feature checkout through `workspace.Cleanup`
(`internal/workspace/cleanup.go`). For a ready workspace, `cleanupReady` proves the checkout is
clean (`dirtyPaths`, which runs `git status`) and then calls the non-forcing
`gitcli.RemoveWorktreeClean` (`git worktree remove -- <path>`). Any non-zero exit with
`KindCommandFailed` is classified as "git refused the non-forcing removal (workspace not clean)",
which is blocked, and the manifest stays at the `ready` phase.

The misclassification is in that last step. `git worktree remove` first runs its own clean check.
It then deletes the directory tree and, **even if that delete fails**, deletes its administrative
directory, which is the registration. Git's source says "continue on even if ret is non-zero,
there's no going back from here". It exits non-zero after doing so. Reproduced in a scratch repo
on 2026-10-05: with one subdirectory made undeletable, `git worktree remove` printed
`error: failed to delete '<path>': Permission denied` and exited 255. The registration was gone,
and part of the tree was left behind.

That is what happened to 0366:

- The remnant held 36 files and a `.git` pointer to an administrative directory that no longer
  existed.
- Its `.DS_Store` was written at 21:28 local, the minute cleanup ran. Finder writing into the
  folder during the delete is the likely cause of the failure.
- `.DS_Store` is in this repository's `.gitignore`, so neither Docket's clean check nor Git's saw
  it. It was not the blocker.
- On every re-run, `cleanupReady` finds the recorded path unregistered and returns blocked
  ("recorded path is not registered"). Nothing can move it forward.
- Because the workspace leg never succeeds, `finalize cleanup` also retains the local and remote
  feature branches. The human deleted the folder and both branches by hand.

### 3. A blocked cleanup doesn't say why

`finalizeCleanupWorkspace` (`internal/app/finalize_cleanup.go`) throws away
`CleanupResult.BlockedBy`, the bounded list of reasons or paths `workspace.Cleanup` already collects.
It reports only "the feature workspace is not a clean, ready checkout; it is retained". The 0366
session had to inspect the folder by hand to find out what had happened.

## Decisions (settled at grooming)

1. **Only GitHub's plan-gate answer lets the merge proceed.** A `403` whose message is GitHub's
   "upgrade or make this repository public" text means the branch has no rules. Every other failure
   still stops as unknown. 0336's rule that a failed read is unknown otherwise stands.
2. **When Git has already started deleting, Docket finishes the delete.** If the finish also fails,
   Docket marks the workspace cleaned anyway, removes the branches, and reports the leftover path.
   An ADR records this as the one case where Docket deletes a folder by path.
3. **No special case for OS files.** An untracked `.DS_Store` in a repository that doesn't ignore it
   keeps blocking safely: nothing is touched, and the human deletes it and re-runs. The blocked
   finding now names what blocked it.

## Design

### 1. Recognize the plan-gate answer (`internal/githubcli/mergemethod.go`)

When the branch-rules `gh api` call exits non-zero, `probeBranchMergeRules` tries to decode stdout
as GitHub's error body, `{"message": string, "status": string}`. It treats the result as **branch
rules unavailable** only when all three hold:

- the body decodes;
- `status` is exactly `"403"`;
- `message` matches GitHub's plan-gate wording: it starts with `Upgrade to GitHub ` and ends with
  ` or make this repository public to enable this feature.` This covers the Pro and Team variants.

In that case the probe returns the all-methods set (`rebase`, `merge` and `squash` all true) and
reports that rules were unavailable. Any other non-zero exit stays exactly as today, a `KindExternal`
failure that `MergePullRequest` maps to `MergeUnknown`. That includes an unreadable body, a different
status, a different message, a 404, a 5xx, and a transport failure.

If GitHub ever rewords the message, Docket falls back to today's stop. A missed match fails safe.

`MergeResult` gains a boolean, `BranchRulesUnavailable`. It is set whenever the plan-gate answer was
used, on every outcome reached after the policy probes, including `method-unavailable`. The
repository probe and the selection order are unchanged, and so are `--admin`, exact-head matching,
the authoritative reprobe, and the refusal to retry a lower-priority method. GitHub still enforces
any constraint at merge time; a rejection there stays an ordinary `denied`.

### 2. Report it visibly (`internal/app/finalize_merge.go`, `skills/docket-finalize-change/SKILL.md`)

When `MergeResult.BranchRulesUnavailable` is set, the `finalize.merge` result appends one warning
finding:

- code `branch-rules-unavailable`
- message: "GitHub does not offer branch rules for this repository on its plan; the merge method
  was chosen from the repository settings alone"

It is a note, never a stop. The disposition, result and method fields are unchanged.

The finalize skill sends one `late_findings` entry to `finalize.closeout` when the merge result
carries that finding. Closeout notes then record it permanently. This follows the skill's existing
rule for a run that authored a repair.

`docs/guide/landing-changes.md` step 4 gains one sentence: on a private repository whose GitHub plan
has no branch rules, the method comes from the repository settings alone.

### 3. Finish a removal Git already started (`internal/workspace/cleanup.go`)

In `cleanupReady`, when `RemoveWorktreeClean` fails with `KindCommandFailed`, Docket re-reads the
worktree list (`ListWorktrees`) before classifying:

- **The list can't be read:** a `failed` error return, as today. A probe error is never read as
  absence.
- **The recorded path is still registered:** Git refused before deleting anything. Blocked,
  byte-untouched, as today. The reason now reads "git refused the non-forcing removal".
- **The recorded path is no longer registered:** Git passed its own clean check and started its
  delete, so its registration is already gone. Docket finishes the delete with `os.RemoveAll` on the
  manifest's recorded path. That path was already proven to be this repository's
  `<primary>/.worktrees/<slug>` by `ownsManifest`. Docket then advances the manifest from ready to
  the cleaned tombstone and returns `cleaned`.
  - If `os.RemoveAll` fails, Docket still advances the manifest to cleaned, since the checkout is no
    longer a Git checkout. It returns `cleaned` with a new `CleanupResult.Remnant` field holding the
    leftover path.

This is safe because of what the leftover can contain: copies of tracked files at a head finalize
already verified as merged, gitignored files, and anything written during Git's delete. Git was
already deleting all of it under the same clean proof. Finishing the delete only completes an
action Git had committed to.

Comment updates:

- The "never an administrative directory by pathname" comment in `cleanup.go`, and the "recursively
  deletes by pathname" sentence in `finalize_cleanup.go`'s file comment, name this exception.
- `gate cleanup`'s comment "never a recursive pathname delete" refers to run logs and stays as it is.

**ADR (recorded at build through `docket-adr`):** "Cleanup finishes a worktree removal Git already
committed to". It relates to ADR-0035 (teardown is fail-closed, never half-destructive). This is
the one case where Docket deletes a directory by path: a removal Git began after passing its own
clean check, shown by the registration being gone. ADR-0035 rejected exactly that kind of
half-finished destruction.

**Known limit, accepted:** if the Docket process dies in the moment between Git's failed removal
and Docket's finish, the manifest stays `ready` with an unregistered path. That is today's stuck
state, and the human deletes the folder by hand. No durable "removing" marker is added for a window
that small.

### 4. Branches and the remnant note (`internal/app/finalize_cleanup.go`)

`finalizeCleanupWorkspace` treats `cleaned` the same with or without `Remnant`: the workspace leg is
done, and the local and remote branch legs run under their existing proofs. The local-ref proof
requires that no worktree has the branch checked out, which holds once the registration is gone.

When `Remnant` is set, cleanup appends a warning finding:

- code `workspace-remnant`
- message: "Git removed the worktree but part of the folder could not be deleted; delete <path> by
  hand"

The disposition stays `cleaned`.

### 5. Name what blocked a workspace (`internal/app/finalize_cleanup.go`)

The `workspace-blocked` finding's message appends `res.BlockedBy`, already capped at eight entries by
`boundedReasons`. For example: "the feature workspace is not a clean, ready checkout (.DS_Store); it
is retained". There is no other change to the blocked path.

### 6. Tests

Every new assert is mutation-checked: remove the behavior it guards and watch the assert fail.

- **Plan-gate probe** (`internal/githubcli`, fake `gh`):
  - The plan-gate 403 body plus exit 1 merges with the repository's methods and sets
    `BranchRulesUnavailable`.
  - A squash-only repository with the plan-gate answer selects squash.
  - Each of these still returns `MergeUnknown` with a failure: a 403 with any other message, a 404
    body, a non-JSON stdout, and a status other than `"403"` with the plan-gate message.
  - The Team wording ("Upgrade to GitHub Team or make this repository public…") also matches.
- **Finalize merge** (`internal/app`): a merged result from a plan-gated probe carries exactly one
  `branch-rules-unavailable` warning, and its disposition is unchanged.
- **Workspace cleanup** (`internal/workspace`, real Git in a temp repo, failure induced the way the
  2026-10-05 reproduction did it):
  - **Finish succeeds:** for example, the undeletable directory is made deletable again before
    Docket's finish, or a seam injects one failed removal. The folder is gone, the manifest is
    cleaned, and the result is `cleaned` with no `Remnant`.
  - **Finish fails:** the manifest is cleaned and the result is `cleaned` with `Remnant` set to the
    path.
  - **Git refuses while the path is still registered:** for example, an untracked file appears
    after Docket's check. The result is blocked, and the workspace and manifest are byte-untouched.
  - **The re-list errors:** a failed error return, nothing deleted.
- **Finalize cleanup** (`internal/app`):
  - After a finish-fails remnant, the local and remote branch legs still run, and the result
    carries one `workspace-remnant` warning.
  - An untracked, non-ignored file blocks cleanup and the `workspace-blocked` message names it.

## Out of scope

- Every other finalize gate, and what a real branch rule allows.
- Any other unreadable branch-rules response. Those keep stopping as unknown.
- Ignoring or deleting OS files such as `.DS_Store` or `Thumbs.db`.
- A durable "removing" manifest phase, or recognizing a remnant left before this change. The one
  known remnant, 0366's, was cleaned by hand.
- Workspaces with uncommitted tracked changes, which still block.
- The publish step's identical clean check.

## Acceptance

- On a private free-plan repository, `finalize.merge` merges using the repository's settings and
  its result carries `branch-rules-unavailable`. Closeout notes record it.
- Any other branch-rules failure still reports `merge-probe-unknown` and makes no merge call.
- When `git worktree remove` fails after Git removed the registration, one `finalize cleanup` run
  ends `cleaned` and removes the local and remote branches. The leftover folder is either deleted
  or named in a `workspace-remnant` warning.
- A Git refusal while the path is still registered remains blocked and touches nothing, and a
  blocked cleanup names what blocked it.
- The ADR is recorded, and the whole suite passes.
