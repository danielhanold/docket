# disable-worktree-hooks.sh — skip git hooks on a docket-owned worktree, idempotently

## Purpose

Points a docket-owned worktree's git-hook lookup at an empty, docket-owned directory, so every
commit into that worktree skips the repo's shared hook framework (pre-commit.com, husky, lefthook).
docket makes many machine-generated bookkeeping commits into worktrees on the orphan `docket` branch
(no `.pre-commit-config.yaml`) and onto the integration branch (its own docs); a shared `pre-commit`
hook would hard-fail or run against commits it was never meant to guard. This helper disables the
hook *mechanism* — framework-agnostically — by construction, so no per-commit flag can be forgotten.

Scope is metadata bookkeeping only. Feature-branch code worktrees are never passed to this helper,
so the team's code-quality hooks still fire on real code headed to a PR (change 0063).

Invoked by `docket-status.sh` (the persistent `.docket` worktree), `migrate-to-docket.sh` (its
transient seed/prune worktrees), and `terminal-publish.sh` (its transient publish worktree),
immediately after each `git worktree add`. Idempotent and self-healing — a repeat call is a clean
no-op, so existing installs are fixed on the next docket run.

## Usage

```
disable-worktree-hooks.sh --worktree DIR
```

- `--worktree DIR` — the docket-owned worktree to disable hooks on (required).

**Mock seam:** `GIT="${GIT:-git}"`.

## Behavior

1. **Resolve the empty hooks dir.** `<git-common-dir>/docket/empty-hooks`, resolved to an absolute
   path (`cd DIR && cd "$(git rev-parse --git-common-dir)" && pwd -P`) and created with `mkdir -p`.
   Absolute so `core.hooksPath` never resolves relative to a worktree root; a real empty directory
   avoids "hooksPath does not exist" surprises in git and in a framework's own `core.hooksPath`
   checks. Living under `.git/`, it is never tracked and never leaks into a commit.
2. **worktreeConfig safety (first enable only).** If `extensions.worktreeConfig` is not already
   `true`, enable it (git requires the extension enabled before any `--worktree` write can happen at
   all), then detect a pre-existing **common-config** `core.worktree`/`core.bare` value and relocate
   it to the main worktree's per-worktree config. If a value is present and cannot be relocated
   safely, roll back the enable (unset `extensions.worktreeConfig`) and exit 1 (fail-closed), so the
   extension is not left enabled with a value stranded in common config. In virtually all repos these
   keys are unset (or only `core.bare` is set, to its default) so this is a no-op or single-value
   path; the rollback fully restores that case. (The pathological case of *both* keys set in common
   config with a write failing on the second is left as-is — negligibly rare, since `core.worktree`
   in common config is itself unusual.)
3. **Set the worktree-scoped hooks path.** `git -C DIR config --worktree core.hooksPath <empty>`.
   `--worktree` replaces rather than appends, so re-running never duplicates the entry.

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Hooks disabled on DIR — or already disabled (idempotent no-op). |
| 1 | DIR missing/not a worktree, common git dir unresolvable, or an unsafe-to-relocate `core.worktree`/`core.bare` blocked enabling. |
| 2 | Usage error (missing `--worktree`, unknown flag). |

## Invariants

- **Worktree-scoped, never global.** Only the passed worktree's `core.hooksPath` is set; the main
  working tree and every feature worktree keep running the team's hooks. The behavior test asserts a
  main-worktree commit still fails after the helper runs.
- **Idempotent.** A repeat call re-writes the same value under `--worktree`; there is never a
  duplicate `core.hooksPath` entry and no error. This is what makes it self-healing at every
  create/ensure site.
- **Local-only.** Touches only `.git/config` and `.git/worktrees/<wt>/config.worktree` plus a dir
  under `.git/`. Never the remote, teammates' clones, or the committed `.docket.yml`.
- **Fail-closed on the worktreeConfig caveat.** Rather than risk silently unsetting `core.worktree`/
  `core.bare` for linked worktrees, it relocates-or-refuses; on a failed relocation the tentative
  `extensions.worktreeConfig` enable is rolled back, fully restoring the original state for the
  no-value and single-value cases (the universal ones). A simultaneous two-value relocation whose
  second write fails is not atomically unwound — a deliberately accepted, negligibly-rare gap
  (`core.worktree` in common config is itself unusual), not a claim of full multi-value atomicity.
