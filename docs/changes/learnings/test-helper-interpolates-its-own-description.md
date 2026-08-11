---
slug: test-helper-interpolates-its-own-description
hook: "A backtick in test SOURCE runs when the shell reaches that line — in a double-quoted literal, in an eval'd condition, in an unquoted heredoc — so a guard's own anchors can mutate the tree it is testing."
topics: [testing, shell, guards]
changes: [212, 221]
created: 2026-08-05
updated: 2026-08-11
promotion_state: candidate
promoted_to:
---

## Apply

In a shell test file, a description, label, or anchor string is **data** — but the shell does not
know that. A backtick in test source is command substitution, and it **runs when the shell reaches
that line**, before `assert` is ever called. The caller wrote prose; the shell ran a command.

The executing vector is **source evaluation**, not the helper printing its argument. This was
disproven by probe (0221): parameter expansion does **not** re-trigger command substitution, so a
backtick held in a variable's *value* is inert through `echo "ok - $1"`, through
`printf "… $1 …"`, and through expansion inside an eval'd single-quoted condition. A helper
hardened with `printf '%s'` and nothing else would have prevented **none** of this.

The positions that actually execute:

- **A double-quoted literal in test source** — a description written `assert "… \`cmd\` …" …`, or
  data assigned `SITES="… \`cmd\` …"`. Multi-line double-quoted assignments are where 0212's
  incident lived, which is why a line-local scanner cannot find this class.
- **An eval'd condition**, in two spellings with the same fate: an *unescaped* backtick typed inside
  a single-quoted condition (source quoting protects the first evaluation; `eval` strips that
  protection), and an *escaped* backtick inside a double-quoted string — the escape is consumed at
  source evaluation, so a bare backtick reaches `$2` and runs at the eval. That second spelling also
  fires through a plain assignment (`cond="… \"\`cmd\`\" …"; assert "d" "$cond"`), which is why
  0221's rule is position-independent rather than keyed on an argument index (ADR-0091).
- **An unquoted-delimiter heredoc body** (`<<EOF`), which the shell also substitutes. `<<'EOF'` is
  inert.

What to do:

- Carry verbatim clauses and anchors in **single-quoted literals** or **quoted-delimiter heredocs**.
  Where a pattern needs a literal backtick beside a `$var`, concatenate a file-local `BT='` + a
  single-quoted backtick — inert by construction.
- Escape backticks inside assert conditions: `'grep -qF "\`span\`" "$f"'` is the house idiom and is
  safe, because eval sees the backslash and treats the backtick as literal.
- Print caller-supplied text with `printf '%s'` (single-quoted format, value as an argument). This
  is correct hygiene and prevents echo-flag and escape surprises — it is just **not** the fix for
  this hazard, and treating it as the fix is how the original diagnosis went wrong.
- This matters most in prose-anchored guards, where the anchors *are* excerpts of the file under
  test and inline-code spans are the house style — so backticks are not an exotic input, they are
  the expected one.
- A per-file comment forbidding backticks is a mitigation at one call site, not a fix: the `assert`
  idiom is copied across ~100 sibling files, so the next author inherits the defect with none of the
  warning. Enforcement has to be mechanical and it has to run **before** the first test file is
  executed — detection after execution is not prevention.
- The blast radius is not a red test. Command substitution runs with the worker's cwd and
  credentials, so the harness can revert, delete, or push. Treat "my test suite executed part of my
  test data" as a data-loss class, not a formatting bug.

## War story

- 2026-08-05 (#212, PR #161) — `tests/test_inline_role_stop_scoping.sh` carried an anchor that was
  an excerpt of skill prose containing a backticked `git checkout .`. Running the suite
  **command-substituted it**, and the working tree reverted the fix worker's own uncommitted edits
  mid-build — silently, from inside the thing whose entire job is to observe without changing.
  Two things make this worth a finding rather than a shrug. First, the input was not adversarial or
  unusual: the file under test is skill documentation, its anchors are quoted sentences from it, and
  inline code in those sentences is the norm — so the "hostile" input is simply the ordinary one.
  Second, the fix that shipped was the *narrow* one — a comment forbidding backticks in anchors at
  that one call site — while the `assert` idiom is shared across the sibling test files. The rule
  inherited by the next author was "don't put backticks in your anchors," which is unenforced and
  unexplained. Carried as an explicit follow-up on 0212 rather than fixed in-branch.

- 2026-08-11 (#221) — the follow-up, and it corrected this file's own diagnosis. Grooming's probe
  showed the helper's `echo "ok - $1"` is provably **not** the executing vector, so the originally
  proposed fix (harden the helper with `printf '%s'`) would have shipped a change that prevented
  nothing while looking like a fix. The real fix landed at source: a checker
  (`scripts/check-test-source-hygiene.sh`) that `scripts/run-tests.sh` runs as a **synchronous
  preflight** over every target before the first launch, aborting the run with zero test files
  executed. Two lessons beyond the mechanism. First, the guard shipped its own silent failure: its
  line-continuation handler treated a backslash-newline as an escape rather than a **splice**,
  consuming the first character of the next line — which made the eval rule inert on the
  house two-line `assert … \` spelling, 55% of the suite's call sites, while every fixture passed
  and a documented mutation probe reported green. It was caught in whole-branch review, not by the
  suite. Every red fixture had written its hazard on one physical line, so the missed spelling was
  the codebase's own idiom — the exact shape AGENTS.md warns about. Second, a guard for a defect in
  the *oracle* cannot be validated by the oracle: a green suite is nearly no evidence, and the
  probes that actually found things were fixture-level mutation probes plus one differential capture
  proving a refactor was verdict-preserving.
