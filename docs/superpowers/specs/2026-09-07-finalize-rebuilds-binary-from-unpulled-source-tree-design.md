<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0346 — Finalize's post-merge binary rebuild runs against an unpulled source tree](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0346-finalize-rebuilds-binary-from-unpulled-source-tree.md)**
<!-- docket:backlink:end -->

# Verified post-merge binary rebuild

Approved design for change 0346, 2026-09-07.

## Problem and current context

The repository requires a development reinstall after a PR merges into main. A successful install from an old primary checkout can produce an old binary with a new timestamp, so install success alone does not satisfy that requirement.

Change 0388 is now done. Its native `repository.sync-integration` operation safely advances a clean primary checkout and the finalize workflow invokes it once after the batch's closeout and cleanup attempts. It deliberately skips unsafe checkouts and reports failures independently of completed lifecycle work. Change 0346 therefore addresses the remaining ordering and verification gap around the repository's rebuild policy; it does not reimplement sync.

This design was inspected against source at 9cce7f9838e6f85f1b98b2676ea87e838aa505ee and fetched main at 0d1a7e1b36af36c1f805952cd3836e84dbad5b6f. The intervening diff affects metadata ownership, not the inspected finalize, sync, or install paths. Change 0340 already stamps development binary identity. Its fallback to unknown identity remains valid for ordinary developer installs, but cannot count as proof of a successful post-merge refresh. Change 0392 preserves installation through newer configuration schemas.

## Chosen approach and ownership

Use the existing sync, install, and version operations, plus read-only Git proofs. This is a workflow and repository-policy correction with regression guards. It introduces no production CLI operation, flag, configuration key, installer mode, or alternate source checkout.

The maintained finalize skill owns the generic ordering: repository-required post-merge build/install work follows the end-of-run integration sync, and its result is reported separately. It must not hard-code Docket's source path or impose binary installation on consuming repositories. The concrete Docket rebuild requirement remains in `AGENTS.md`, under `Rebuild the binary after a merge to main`; `CLAUDE.md` remains its existing symlink. Keep that rule concise and update its condition and proof together.

Every Docket operation is resolved through the capability catalog. Read structured results using the corresponding schema; do not turn human messages or exit status alone into authority. No shell facade or raw rebuild fallback is introduced.

## Rebuild sequence

1. Retain the actual merge commit IDs from authoritative, verified PR merge facts for integration merges handled by the run, including already-merged recovery. A feature branch's original head is not a substitute: rebase and squash may change commit identity. A stacked-only merge into a parent does not trigger this repository's main-merge rebuild rule.
2. Let the existing once-per-batch integration sync run after closeout and cleanup, including their possible integration backlink repairs. Do not add per-change sync calls or rebuild before this suffix. An equivalent directly attended merge follows the same sequence using the catalog-resolved sync operation once.
3. Authorize a rebuild only for sync disposition `advanced` or `already-current` with valid repository and target facts. `skipped`, `refused`, `failed`, missing facts, malformed output, or an unobservable outcome leaves the rebuild incomplete. A successful command exit on a deliberate skip is insufficient.
4. Inspect the exact source directory the repository policy supplies to install, `/Users/homer/dev/docket`. Its canonical identity must match the synced primary checkout. Require the expected integration branch, no unfinished Git operation, and a clean index/worktree including non-ignored untracked files. Read and retain its full commit ID S; require equality with the successful sync's target and after commit IDs. Unknown or changed state does not authorize installation.
5. Prove every retained main merge commit is an ancestor of S in that source repository. Use `git merge-base --is-ancestor` against immutable commit IDs, distinguishing a negative ancestry answer from a failed probe. Neither permits rebuild. This proves the source used by install, not merely a remote-tracking ref or feature worktree.
6. Invoke the catalog-resolved `development.install` with the existing source argument. A single successful rebuild may satisfy every verified main merge in the batch. Do not change the normal installer or silently retry through another build method.
7. After install succeeds, recheck that the source branch, cleanliness, and HEAD still match the observations for S. Read the identity from the installed executable at the binary destination the installation actually updated, using its version operation. Require its full, clean commit identity to equal S. Unknown, dirty, mismatching, or unreadable identity is verification failure. Do not accept a timestamp, version label, short-prefix comparison, or the identity of an older running process as proof.

The success report includes the verified installed commit. Source movement or a failed post-install observation is reported honestly as an unverified rebuild; an installation may already have changed, so do not claim it was untouched or attempt a rollback. These checks detect observed source changes; they do not introduce a lock or claim atomic exclusion against arbitrary concurrent edits to the source tree.

## Failure behavior

Keep the verified merged change `done` and report `binary rebuild incomplete` separately, naming the failed condition and source path. A failed rebuild never reverses a merge, revives a terminal change, writes a finalize-blocked marker, or rewrites frozen results or closeout notes. Preserve any earlier workflow halt.

When the source cannot safely sync, do not stash, switch branches, reset, discard files, or build from another checkout. State the specific obstacle and the valid recovery sequence: resolve the reported source state, rerun integration sync, then repeat the proof/install/identity sequence. Do not offer the bare install command as a sufficient remedy for stale source. Installation and post-install verification failures similarly retain their actual diagnostics and require a verified retry.

## Validation and acceptance

Add focused Go regression guards in the existing repository-guard package for the maintained policy and finalize sequencing. Locate relevant executable instruction sites by a whole-repository search, distinguishing authored instructions from generated copies and frozen records. Bind checks to the section and ordering/conditional claims, not mere token presence. Mutation-test removal or inversion of the success-disposition gate, source ancestry proof, post-install exact identity check, end-of-run ordering, and separate incomplete-report requirement. Disable Go test caching for mutation probes and prove each mutation actually landed before interpreting its result.

Reuse the existing Go sync tests for fast-forward behavior and unsafe/unknown-state handling, and the development install identity tests for stamping. Do not build a duplicate sync engine or a test-only rebuild workflow. Review the authored procedure against concrete cases: a clean source behind a merge; already-current source; dirty, detached, other-branch, local-ahead, or diverged source; successful sync whose target excludes a retained merge; failed Git probe; failed install; source movement; unknown/dirty/mismatching binary identity; batch and already-merged recovery; and a consuming repository with no rebuild requirement. Each case must have an explicit permitted action and report under the prose above. The guards prove the maintained instruction contract, not universal obedience by every agent harness.

Regenerate embedded assets through the normal generation path after changing the shipped skill. Run the entire configured build suite through the source-entered Go runner and act on authoritative serial budget breaches. Do not add standalone Bash tests, alter historical records, or raise budgets to conceal growth.

Acceptance: the normal clean-checkout path syncs before rebuilding and reports the installed commit containing the merge; every unsafe or unverifiable path reports the outstanding rebuild without presenting completed merge/closeout work as failed.

## Relations and alternatives

Set `depends_on: [388]` (already done), `related: [283, 340, 392]`, retain `discovered_from: [342]`, and cite ADR-0099 and ADR-0104. Change 0283's future instructions-file slim must preserve this repository-specific behavioral requirement. No stacking is needed. Main-mode is removed under ADR-0099, so it is not a compatibility branch to add.

A temporary clean checkout would add source ownership and cleanup work and can change the source path used by development asset links. An installer-wide freshness restriction would interfere with intentional installs of older or dirty development trees; a new optional guarded-install mode would add a public interface and rollout burden. Neither is necessary to close this workflow defect now that native sync and build identity exist. A warning after blindly rebuilding was rejected because it still performs the stale rebuild the policy should prevent.
