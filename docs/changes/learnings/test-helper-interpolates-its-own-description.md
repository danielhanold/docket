---
slug: test-helper-interpolates-its-own-description
hook: "A helper that interpolates a caller-supplied description into a double-quoted string executes any backticks in it — a guard's own harness can mutate the tree it is testing."
topics: [testing, shell, guards]
changes: [212]
created: 2026-08-05
updated: 2026-08-05
promotion_state: candidate
promoted_to:
---

## Apply

In a shell test helper, a description, label, or anchor string is **data**. The moment it reaches
`"$1"` inside a double-quoted context — `echo "ok: $1"`, `printf "... $1 ..."`, `msg="… $1 …"` — the
shell expands it, and a backtick in the caller's string is command substitution that **runs**. The
caller wrote prose; the harness ran a command.

- Print caller-supplied text with `printf '%s'` (single-quoted format, value as an argument), never
  by interpolating it into a double-quoted string.
- This matters most in prose-anchored guards, where the anchors *are* excerpts of the file under
  test and inline-code spans are the house style — so backticks are not an exotic input, they are
  the expected one.
- Forbidding backticks in anchors is a mitigation at one call site, not a fix: the `assert` idiom is
  copied across sibling test files, so the next author inherits the defect with none of the warning.
  Harden the helper.
- The blast radius is not a red test. Command substitution runs with the worker's cwd and
  credentials, so the harness can revert, delete, or push. Treat "my test suite executed part of my
  test data" as a data-loss class, not a formatting bug.

## War story

- 2026-08-05 (#212, PR #161) — `tests/test_inline_role_stop_scoping.sh`'s `assert` helper
  interpolated its description into a double-quoted string. One of the anchors under test was an
  excerpt of skill prose containing a backticked `git checkout .`. Running the suite
  **command-substituted it**, and the working tree reverted the fix worker's own uncommitted edits
  mid-build — silently, from inside the thing whose entire job is to observe without changing.
  Two things make this worth a finding rather than a shrug. First, the input was not adversarial or
  unusual: the file under test is skill documentation, its anchors are quoted sentences from it, and
  inline code in those sentences is the norm — so the "hostile" input is simply the ordinary one.
  Second, the fix that shipped was the *narrow* one — a comment forbidding backticks in anchors at
  that one call site — while the `assert` idiom is shared across the sibling test files, every one
  of which still interpolates. The rule inherited by the next author is "don't put backticks in
  your anchors," which is unenforced and unexplained, rather than "the helper doesn't evaluate its
  arguments." Carried as an explicit follow-up on 0212 rather than fixed in-branch.
