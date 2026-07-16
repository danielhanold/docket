---
slug: shared-git-config-mutation
hook: "When a helper mutates SHARED git config on a frequently-run path, only touch a value when the tool's own rule requires it."
topics: [git, config, concurrency]
changes: [63]
created: 2026-07-11
updated: 2026-07-11
promotion_state: retained
promoted_to:
---

## Apply
When a helper mutates SHARED git config (common/global) on a frequently-run path,
only touch a value when the tool's own rule requires it (here: relocate `core.bare` only when
`true`, per git); leave harmless defaults untouched, and assume a concurrent loop may be
mutating the same key. Also: enabling `extensions.worktreeConfig` must precede any `--worktree`
write and roll back on a failed follow-on write — fail-closed ordering for multi-step config changes.

## War story
- 2026-07-11 (#63, PR #72) — 0063 disabled hooks on docket worktrees by relocating a conflicting
  common-config git value, and an early draft relocated `core.bare` unconditionally. Because
  `git init`/`clone` write `core.bare=false` into common config on essentially every repo, that
  fired on docket-status's most-run path — and since docket runs concurrent autonomous loops, a
  concurrent `--unset core.bare` raced one loop into the rollback branch, transiently re-enabling
  hooks.
