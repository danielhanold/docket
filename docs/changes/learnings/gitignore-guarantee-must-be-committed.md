---
slug: gitignore-guarantee-must-be-committed
hook: "An 'every clone / never committed' guarantee must rest on a committed repo .gitignore entry, never a per-machine user-global ignore."
topics: [git, gitignore, guarantees]
changes: [27]
created: 2026-06-19
updated: 2026-06-19
promotion_state: retained
promoted_to:
---

## Apply
An "every clone / never committed" guarantee must rest on a committed repo `.gitignore` entry, never a
per-machine user-global ignore — and when a change *generates* such a file, add the ignore in the same
change so the guarantee ships with the feature instead of silently depending on each dev's box.

## War story
- 2026-06-19 (#27, PR #39) — A change promised its locally-written file (`.claude/settings.local.json`)
  would "never be committed onto collaborators," but on the build machine that guarantee only held
  because a *user-global* excludesfile (`~/.config/git/ignore`) ignored it — the repo `.gitignore` did
  not. Reconcile caught it; unfixed, a collaborator without that global ignore could have committed the
  file, defeating the change's whole point.
