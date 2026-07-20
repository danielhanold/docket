---
slug: agent-shell-noop-reads-as-success
hook: "The agent's interactive shell is not bash and its grep may not be grep — a sweep can iterate zero items, a verification grep can match nothing, and both still print success."
topics: [shell, verification, environment]
changes: [109]
created: 2026-07-20
updated: 2026-07-20
promotion_state: candidate
promoted_to:
---

## Apply
An agent-authored one-off sweep or verification command runs in the **harness's interactive shell**,
not in a clean `bash` and not against POSIX tools. Two failure modes follow, and both are silent:

- **Word-splitting.** `for f in $FILES` over an unquoted multi-line scalar iterates zero times under
  zsh, which does not word-split unquoted parameters the way bash does. Run multi-file sweeps under
  an explicit `bash -c`, or feed the list through `while IFS= read -r f`.
- **Shadowed tools.** The shell may define a function or alias shadowing a standard tool (a `grep`
  function backed by `ugrep` strips the leading `./` from paths, so plan-authored filters anchored on
  `^\./…` exclude nothing). Verify with `command grep` / `git grep`, never the bare name.

The compounding rule is the one to remember: **zero iterations and zero matches are indistinguishable
from success.** A `sweep done` line printed after a loop says the script reached the end, not that the
loop ran; a grep filter that excludes nothing makes a clean tree look dirty (or a dirty one look
clean). Assert on the *effect* — count the files actually touched, `git diff --stat` the result — never
on the fact that the command exited 0.

## War story
- 2026-07-20 (#109, PR #112) — A rename sweep across 16 files. The first attempt's
  `for f in $FILES` ran under zsh, iterated **zero** times, touched nothing, and printed its
  unconditional `sweep done` success line; the run looked complete. Fixed by running the loop under
  `bash` explicitly. The same change hit the second face during verification: the harness shell's
  `grep` function (ugrep) drops the leading `./`, so the plan's `^\./docs/…` exclusions matched
  nothing and every historical artifact leaked into the results, making an actually-clean sweep read
  as dirty. Re-verified with `command grep` and cross-checked with `git grep`. Neither failure was a
  defect in any committed script — both are traps for the agent's own throwaway commands, which is
  exactly what a mechanical rename is made of.
