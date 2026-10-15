<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0551 — Check out a change built on another machine into its own docket worktree](../../changes/archive/2026-10-15-0551-check-out-a-change-built-on-another-machine-into-its-own-doc.md)**
<!-- docket:backlink:end -->

# Check out a change built elsewhere into its own docket worktree — design

## Problem

Finalize works only on the machine whose docket created the change's feature workspace. In every other situation it stops, and the remedy is hand surgery.

Observed in a downstream repository: change 21 was `implemented` with an open PR, and its branch was checked out at `.worktrees/feat-aws-mcp-connect-claude-code-harness-support` by a tool other than docket. Docket expected `.worktrees/aws-mcp-connect-claude-code-harness-support` with an ownership manifest under `.git/docket/workspaces/`, and neither existed. `workspace inspect` reported `foreign`. `finalize rebase` returned `blocked` / `workspace-probe-failed` ("failed to start git"). The foreign worktree was clean, at the recorded head, and identical to the remote branch, so nothing stood in the way except docket's ownership rules. No `## Finalize blocked` marker could be recorded, because `finalize.block` needs an attempt token and the failed rebase never issued one.

Tracing the code turned up three defects:

1. **Misleading error.** `FinalizeRebase` calls `RebaseState` in `rc.wsDir` before `requireReadyWorkspace`. For an `absent` or `foreign` inspection, `wsDir` is the intended path, which doesn't exist. Git can't start there, so the real cause (no docket worktree) is reported as a probe failure.
2. **No way to finalize a change built on another machine.** `workspace prepare`'s fresh-allocation arm requires the local branch, the remote branch, the target path, and the registration all to be absent. Once the branch has been pushed, no docket operation can create a workspace for it. `finalize.merge` also requires `HeadsAgree` to include the local workspace head (`in.reqHead == in.localHead`), so even `finalize.gate: off` cannot merge.
3. **No supported way for a human to get a pushed change onto a second machine.** The obvious tools (`gh pr checkout`, `git worktree add`) create exactly the foreign state that blocks finalize.

## Goal

A single command, `docket workspace checkout --id <id>`, puts an `implemented` change into its own docket-owned worktree at `.worktrees/<slug>` on any machine. It works whether the branch exists only on the remote, also as a local branch, or is held by a foreign worktree. It never loses work and never acts on an unproven state. The finalize skill runs the same command before any step that reads the workspace, so finalize repairs this situation itself.

## Design

### The `workspace.checkout` operation

A new catalog operation, `workspace.checkout` (effects: `local-write`), spelled `docket workspace checkout`. It is separate from `workspace prepare` on purpose. Prepare starts a fresh build and requires the branch to be absent. Checkout takes over a pushed branch and requires the branch to exist at a proven head. The two have opposite safety rules, so they don't share a command.

**Inputs.** `--id <id>` (required). Optionally `--revision <record-revision>` and `--head <commit>`, given together or not at all. The finalize skill passes both, taken from `context finalize`. A human passes only the id.

**Status gate.** The change must be `implemented`. Any other status refuses `blocked` with reason `not-implemented` and touches nothing. An `in-progress` change may still have a live run pushing to its branch, and there is no proven head to check against, so it is out of scope.

**The expected head.** The operation resolves one commit, the *expected head*, and acts only when every source agrees:

- the open PR for the recorded `branch:` (exactly one, targeting the change's effective base, the same check `finalize.rebase` makes), its head commit;
- the remote feature branch, freshly fetched;
- `--head`, when supplied.

If any disagree, the result is `contended`, reason `head-mismatch`, with nothing touched. When `--revision` is supplied and the record has moved, the result is `contended`, reason `revision-drift`. The workspace target (slug, effective base, feature ref) comes from the existing `loadWorkspaceContext` / `resolveWorkspaceTarget` path, so the effective-base rule exists in one place only, and stacked changes resolve their parent branch as they already do.

**Outcomes.** All work happens under the existing per-workspace operation lock. The local state is classified with the existing `Inspect`, refined by the checks below.

| Local state | Result |
|---|---|
| Owned workspace, `ready`, at the expected head | `existing`, no-op |
| Owned workspace, `dirty-owned` or head ≠ expected | `blocked` (`workspace-dirty` / `local-head-mismatch`), untouched; the message names the path |
| Owned workspace in any other state (`mismatch`, `branch-missing`, `allocating`, `cleaned`) | `blocked` (`workspace-not-ready`), untouched. Repairing an owned-but-broken workspace is out of scope. |
| Nothing local: no manifest, no local branch, no path, no registration | **attach** (below), `attached` |
| Local branch, not checked out in any worktree, tip == expected head | **attach** without moving the branch, `attached` |
| Local branch, not checked out, tip is a strict ancestor of the expected head | **attach** after fast-forwarding the branch, `attached` |
| Local branch, not checked out, tip ahead of or diverged from the expected head | `blocked` (`local-branch-diverged`), untouched; the message gives the local tip, the expected head, and a remedy |
| Branch checked out in the **primary** worktree | `blocked` (`branch-in-primary-checkout`), untouched; remedy: switch the main checkout to another branch |
| Branch checked out in a foreign linked worktree that passes **every** removal proof | remove it, then **attach**, `attached` plus a `foreign-worktree-removed` finding naming the removed path |
| Foreign linked worktree that fails any removal proof | `blocked` (`foreign-worktree-unsafe`), untouched; the message names the path, the failed proof, and the exact command to resolve it |
| `.worktrees/<slug>` is occupied by something not registered as a worktree | `blocked` (`target-path-occupied`), untouched |
| Any probe error (git failure, unreadable manifest, unreadable status) | `unknown` / external-failed. A probe error is never read as absence in either direction. |

**Foreign-worktree removal proofs.** All of these must hold, each read fresh immediately before removal:

1. The registration is a linked worktree, not the primary worktree and not the metadata worktree.
2. It is attached to exactly the recorded feature ref, not detached.
3. Its HEAD equals the expected head (so it also equals the remote branch and the PR head).
4. It has no tracked modifications, no staged changes, and no untracked files (ignored files do not count and are removed with the folder).
5. It has no rebase, merge, cherry-pick, revert, or bisect in progress.
6. It is not locked (`git worktree lock`).
7. Its path exists, and it carries no docket manifest naming a different owner.

Removal uses the existing clean-only removal path (`RemoveWorktreeClean`, no force), so git itself refuses a worktree that turned dirty after the check. Ignored files (build output, dependencies) are deleted with the folder. Git ignores them, so they are not work, and the finding says so.

**Attach.** This reuses `workspace prepare`'s manifest, lock, and two-phase allocation:

1. Publish an `allocating` manifest for the canonical path `.worktrees/<slug>`, recording the change's feature ref, its effective base ref, and the merge-base of the expected head and the fetched base tip as `BaseCommit`.
2. If the local branch is absent, create it at the expected head. If it is a strict ancestor, move it with a compare-and-swap ref update from the inspected tip to the expected head. If the CAS fails, return `contended` and leave the manifest `allocating`, which a later run can resume.
3. Add a branch-attached worktree at the canonical path for the existing branch (no `-B`, no reset, no force).
4. Reinspect the registration, ref, and HEAD, then advance the manifest to `ready`.

A crash in the middle leaves an `allocating` manifest. A rerun of `workspace checkout` resumes the missing suffix under the same ancestry rule `prepare`'s resume arm uses. After attach, `Inspect` classifies the workspace as `ready`, so `finalize.rebase`, `finalize.merge`, and `finalize.cleanup` treat it exactly like a workspace docket created from scratch. That includes cleanup deleting the local branch after merge, which it already does only when the branch's tip is contained in the verified merge chain.

**Result.** It uses the protocol-v1 envelope like the other workspace operations: id, disposition (`existing` | `attached` | `blocked` | `contended` | `unknown`), reason, message, path, head, and findings (`foreign-worktree-removed`, `local-branch-fast-forwarded`). The schema descriptor is registered so `docket schema --operation workspace.checkout` returns it.

### Adopting a branch docket didn't create (ADR)

`workspace prepare` refuses any branch it didn't create ("pre-Go work is never adopted"), because a fresh build has nothing to prove the branch against. `workspace checkout` deliberately adopts a branch, and may remove a foreign worktree, because an `implemented` change has a head proven by the PR and the remote branch together, and every action it takes is checked against that head. Record this as an ADR: *docket may adopt or replace unowned local state for a change only when that state is proven identical to, or an ancestor of, the change's PR-and-remote-agreed head, and never removes anything that isn't clean.* This keeps a later change from "fixing" checkout back to prepare's rule, and from extending adoption to states that don't have that proof.

### `finalize.rebase` check order

In `FinalizeRebase`'s fresh-attempt path, run `requireReadyWorkspace` before `RebaseState`, so that an `absent`, `foreign`, or `mismatch` workspace reports `workspace-not-ready` with the inspection's `Detail` in the message (e.g. "no workspace manifest; local feature branch exists"). Git then never runs in a directory that may not exist. Foreign-in-progress detection still precedes the dirty check, so a rebase this call didn't start is never reported as merely dirty: `RebaseState` runs only when the workspace is registered (`ready` or `dirty-owned`). The `ready` / `dirty-owned` distinction then follows. The message for `workspace-not-ready` names `docket workspace checkout --id <id>` as the remedy when the state is `absent` or `foreign`.

### Finalize skill

Add one step to `skills/docket-finalize-change/SKILL.md`, immediately after *Authoritative context* and before the first step that reads the workspace. Run `workspace.checkout` with `--id <id> --revision <revision> --head <head>` from the context. Run it in every gate mode, because `finalize.merge` needs the local head even when `finalize.gate: off`. Outcome mapping: `existing` / `attached` → continue (name a `foreign-worktree-removed` finding in the final report); `contended` → re-read `context finalize` (the run's disposition is `contended`); `blocked` → `halted` with the message relayed verbatim; `unknown` → `halted`. Regenerate the embedded skill assets.

### Guide

Add a short section to the guide (`docs/guide/landing-changes.md`) on working with a change built on another machine. Run `docket workspace checkout --id <id>` to get it locally in its own worktree. Don't use `gh pr checkout` or a hand-made `git worktree add`, because those create a worktree docket can't use. Finalize performs the same checkout automatically. Describe current behavior only.

## Testing

Integration tests in `internal/workspace` with temp-repo fixtures (bare remote and clone), plus app-layer tests for the operation envelope:

- built elsewhere (remote branch only) → `attached`, `ready` manifest, `Inspect` = `ready`;
- local branch at the expected head, not checked out → `attached`, ref unchanged;
- local branch behind → `attached`, branch fast-forwarded, `local-branch-fast-forwarded` finding;
- local branch ahead / diverged → `blocked` `local-branch-diverged`, ref and filesystem byte-identical;
- branch checked out in the primary worktree → `blocked` `branch-in-primary-checkout`;
- clean foreign worktree at the expected head → removed, `attached`, `foreign-worktree-removed` naming the old path;
- one test per removal proof (dirty tracked file, staged change, untracked file, rebase in progress, locked, head ≠ expected, detached) → `blocked` `foreign-worktree-unsafe`, worktree intact. Mutation-test each: removing that proof's check must turn its test red;
- PR head ≠ remote head, or `--head` ≠ either → `contended` `head-mismatch`, nothing touched;
- `--revision` drift → `contended` `revision-drift`;
- non-`implemented` change → `blocked` `not-implemented`;
- injected probe error at each probe → `unknown`, nothing touched;
- interrupted attach (manifest left `allocating`) → rerun resumes to `ready`;
- existing owned `ready` workspace → `existing`, no Git mutation;
- `finalize.rebase` with an absent workspace and with a foreign workspace → `blocked` `workspace-not-ready` carrying the inspection detail, never `workspace-probe-failed`;
- end-to-end: checkout, then `finalize.merge` with `finalize.gate: off` sees agreeing local, remote, and PR heads.

Run the whole suite at the build gate.

## Out of scope

- Continuing an `in-progress` change on another machine (no proven head; a live run may own the branch).
- Repairing an owned workspace that is broken (`mismatch`, `branch-missing`, a stale `cleaned` tombstone).
- Changing `workspace prepare` in any way.
- Cleaning up foreign worktrees that don't hold the change's branch.
