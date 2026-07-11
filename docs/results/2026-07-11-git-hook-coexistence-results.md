# git-hook coexistence — results
Change: #63 · Branch: feat/git-hook-coexistence · PR: <set at PR open> · Plan: docs/superpowers/plans/2026-07-11-git-hook-coexistence.md · ADRs: 25 (relates ADR-0001)

## Verify (human)

No interactive/manual checks required — the hermetic suite covers the behavior end-to-end (34/34 green, including an always-failing-hook fixture that proves the disable is real, worktree-scoped, idempotent, and non-vacuous). Optional spot-check if desired:
- [ ] In a repo using pre-commit/husky/lefthook, confirm a docket bookkeeping commit (e.g. a `docket-status` board refresh) succeeds while a normal code commit still runs the team's hooks.

## Findings

- **New helper + invariant → ADR-0025.** The mechanism (worktree-scoped `core.hooksPath` → an empty docket-owned dir, via `extensions.worktreeConfig`, applied only to docket-owned worktrees; feature/code worktrees never touched) is recorded as ADR-0025. Establishes the invariant: any new site that creates a docket-owned worktree must call `disable-worktree-hooks.sh` after `git worktree add`; no code worktree may ever be passed to it.
- **Review fix — fail-closed ordering.** git requires `extensions.worktreeConfig` enabled *before* any `--worktree` write, so the safety branch enables first, then relocates a needing-relocation common value, and **rolls back the enable** if relocation fails (exit 1) — never leaving the extension on with a value stranded in common config.
- **Review fix — the ubiquitous `core.bare=false` trap.** `git init`/`clone` write `core.bare=false` into common config on essentially every repo, so an early draft relocated it on nearly every first run: stderr noise on docket-status's most-run path, needless mutation of the user's primary config, and — because docket supports concurrent autonomous loops — a race where a concurrent `--unset core.bare` drove the second loop into the rollback branch, transiently re-enabling hooks. Fixed by relocating `core.bare` only when it is `true` (git's own rule); the harmless default is left in place. Regression-tested with an explicit no-op-enable case.
- **Plan-vs-reality (from reconcile, confirmed in build):** `docket-config.sh --bootstrap` is worktree-free (`create_orphan` uses `commit-tree`), so it is deliberately NOT a hook-disable site — the following `ensure_and_sync_worktree` handles and self-heals it. terminal-publish uses a worktree-scoped disable on its transient `pub-$T` worktree (not a per-commit `-c`) so both the publish commit and the CAS `rebase --continue` replay skip hooks. migrate-to-docket disables hooks on all three of its transient worktrees (orphan seed, top-up, integration prune).

## Follow-ups

- **Accepted limitation (not a blocker):** the two-value safety case — *both* `core.worktree` and `core.bare=true` present in common config with the second relocation write failing — is not atomically unwound (the first key is not restored). Deliberately accepted as negligibly rare (`core.worktree` in common config is itself unusual) and documented honestly in `scripts/disable-worktree-hooks.md`. A future hardening change could make multi-value relocation atomic if a real repo ever hits it.
