---
slug: predicate-must-ask-the-post-pass-state
hook: "A predicate consulted inside the pass that creates its subject must ask what the repo WILL have, not what it has — present tense on a virgin tree picks the wrong branch permanently."
topics: [shell, guards, idempotency]
changes: [242]
created: 2026-08-08
updated: 2026-08-08
promotion_state: candidate
promoted_to:
---

## Apply

When a decision inside a write pass depends on a file that the *same pass* creates, phrase the
predicate in the future tense: "will this repo have X when this pass finishes?" — derived from the
pass's own inputs (the configured harness set, the resolved plan) — never `[ -e X ]`.

Present tense is correct on every subsequent run and wrong exactly once: on the virgin tree, which
is the run that lays down the durable artifact. The wrong branch there is not self-healing, because
the next run sees the artifact the wrong branch created and now agrees with it.

The same shape appears wherever a `--check` leg and a write leg share a subject: gate the check on
the predicate the **writer** actually acts under, not on a weaker sibling — the weaker one demands
the artifact from repos the writer would never have written to.

## War story

- 2026-08-08 (#242, PR #186) — `claude_surface_target()` in `sync-agents.sh` asked whether the repo
  *has* an `AGENTS.md`. On a virgin `[claude, codex]` repo, `AGENTS.md` is created by the very write
  pass that resolves this target, so the present-tense question seeded a second real `CLAUDE.md`
  carrying a permanently duplicated block instead of the intended symlink. Made future-tense.
  In the same branch, the `--check` leg had to be re-gated from the weaker `gitignore_block_wanted`
  to `project_wrappers_generated` — the predicate the project-level pass actually writes under —
  because with `HARNESSES` defaulting to a set containing `claude`, the weak gate demanded a
  `CLAUDE.md` from every repo that merely had a docket branch.
