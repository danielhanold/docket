---
slug: gitignore-guarantee-must-be-committed
hook: "An ignore-related guarantee — never-committed OR always-committed — must rest on a durable committed .gitignore construct, never a per-machine excludesfile or a one-time git add -f."
topics: [git, gitignore, guarantees]
changes: [27, 305]
created: 2026-06-19
updated: 2026-08-13
promotion_state: retained
promoted_to:
---

## Apply
An "every clone / never committed" guarantee must rest on a committed repo `.gitignore` entry, never a
per-machine user-global ignore — and when a change *generates* such a file, add the ignore in the same
change so the guarantee ships with the feature instead of silently depending on each dev's box.

The **inverse** guarantee has the same shape and the same failure mode. When a file must be *tracked*
but sits under a pattern the repo ignores — a fixture named like the machine-local config it imitates,
a build output kept on purpose — `git add -f` gets that one file in and guarantees nothing about the
next one: the pattern still matches, so a sibling added later is silently skipped and the fixture tree
ships incomplete. The durable form is a committed **negation**, placed in a nested `.gitignore` beside
the files it rescues rather than appended to the ignore that swallows them, and deliberately **outside
any marker-managed block** — a generated block gets rewritten wholesale, and a rewrite that hoists or
drops the negation neuters it without touching the line anyone would think to check.

Whichever direction the guarantee runs, verify it by asking git rather than by reading the file:
`git check-ignore -v <path>` names the rule that actually decides, and `git status --porcelain` on a
freshly-created sibling shows whether a *new* file is covered — the case `add -f` never answers.

## War story
- 2026-06-19 (#27, PR #39) — A change promised its locally-written file (`.claude/settings.local.json`)
  would "never be committed onto collaborators," but on the build machine that guarantee only held
  because a *user-global* excludesfile (`~/.config/git/ignore`) ignored it — the repo `.gitignore` did
  not. Reconcile caught it; unfixed, a collaborator without that global ignore could have committed the
  file, defeating the change's whole point.
- 2026-08-13 (#305, PR #205) — the inverse direction. The repo-wide ignore for `.docket.local.yml`
  also matched the repository-local config **fixtures** under `testdata/repositories/`, which exist
  precisely to imitate that filename. They were tracked only because a one-time `git add -f` had
  forced them in; any fixture added later would have been skipped in silence, and the frozen tree
  would have shipped a hole no test could see. Review caught it (important-severity finding #3);
  the durable fix (`551c5bd8`, hardened in `0aabf00a`) is a nested `testdata/repositories/.gitignore`
  negation kept outside the managed docket gitignore block, so a block rewrite cannot hoist past it.
  Residual, recorded in the results file: the negation's effectiveness was confirmed by two manual
  `git check-ignore` probes and no automated guard reproves it if the ignore layout changes.
