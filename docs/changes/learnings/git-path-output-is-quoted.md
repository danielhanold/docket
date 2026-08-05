---
slug: git-path-output-is-quoted
hook: "Git plumbing renders paths C-quoted by default — read them NUL-delimited, and never round-trip NUL-delimited output through command substitution, which strips the delimiter."
topics: [git, shell, scripts]
changes: [202]
created: 2026-08-05
updated: 2026-08-05
promotion_state: candidate
promoted_to:
---

## Apply
Every git command that prints a path — `ls-tree --name-only`, `diff --name-only`, `ls-files`,
`status` — renders it through `core.quotePath`, which defaults to **true**. A path containing a
double quote, a backslash, a control character, or **any non-ASCII byte** comes back wrapped in
double quotes with escapes. A script that then looks up that literal string in another git command
gets a miss, and in a *presence* check the miss is not a crash — it is a confident wrong answer
pointed in whichever direction the surrounding `if` happens to face. Nothing in the repo has such a
path, so nothing ever reddens; the first user with an accented filename gets the false report.

The fix is `-z` (NUL-delimited, quoting off). It brings its own trap, and it is worse than the one
it replaces:

- **`$(…)` strips NUL bytes.** `paths=$(git ls-tree -r -z …)` followed by
  `while … read -r -d '' p; do … done <<<"$paths"` reads **zero** records — every delimiter is gone,
  `read -d ''` hits EOF, the loop body never runs, and the function returns its
  nothing-found answer for **every** input. Green suite, dead check. So the capture-then-here-string
  shape that is idiomatic elsewhere is *unavailable* here, not merely discouraged.
- Consume from a redirect instead: `while IFS= read -r -d '' p; do … done < <(git ls-tree -r -z …)`.
  This also disposes of the reason the capture shape usually exists — the `producer | while … done`
  subshell, where an in-loop `return 0` exits only the subshell. The hazard is the **pipe**, not the
  loop, and a process-substituted redirect has no pipe to be trapped by.
- **Re-audit the downstream sanitizers.** The old encoding was load-bearing for code you did not
  touch: C-quoting guaranteed an embedded newline arrived as the two characters `\n`, so a
  sanitizer could legitimately handle only TAB and CR. Under `-z` the raw LF reaches it. When a
  producer's encoding changes, every consumer's escaping premise expires with it
  ([[shared-resource-keeps-first-owner-assumptions]]).

Pin the behavior with a fixture that sets `core.quotePath=true` **in the fixture repo itself**, so
a developer's global `core.quotePath=false` cannot silently disarm the mutation that proves it.

Related: [[shell-portability]], [[agent-shell-noop-reads-as-success]] (a sweep that iterates zero
items still prints success).

## War story
- 2026-08-05 (#202, PR #158) — `board-checks.sh`'s `branch_only_artifact` enumerated a feature
  branch's files with `git ls-tree -r --name-only` and looked each path up with `git_has`. Any
  non-ASCII path came back C-quoted, the lookup failed, and the check reported an **inherited**
  file as branch-only — a false positive in a check whose entire value is that it is believable.
  The rewrite to `-z` + `read -r -d ''` had to explicitly *reject* the capture-then-here-string
  shape the existing comment argued for, and that comment's "race" rationale turned out to be wrong
  on its own terms as well; it was corrected rather than carried forward. The change shipped
  knowing that the `sanitize` helper downstream still escapes only TAB and CR — recorded as a
  follow-up rather than fixed in-branch, since `-z` can now deliver a raw LF into a TSV record.
