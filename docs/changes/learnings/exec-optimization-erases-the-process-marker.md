---
slug: exec-optimization-erases-the-process-marker
hook: "A marker embedded in a shell `-c` string is not a process identifier — a single simple command is EXEC'd, so the shell's argv is replaced and the marker vanishes from the command line `pgrep -f` reads."
topics: [testing, fixtures, shell, process]
changes: [282]
created: 2026-08-10
updated: 2026-08-10
promotion_state: candidate
promoted_to:
---

## Apply

A fixture that needs to find a process it started later — to assert it is alive, to kill it, to prove
a guard noticed it — usually tags it with a marker and looks it up by command line:

```sh
/bin/sh -c 'sleep 30 # my-canary' &
pgrep -f my-canary          # finds nothing
```

**The marker is gone before the first probe.** When a shell is given `-c` with a *single simple
command*, it does not fork — it **execs** the command in place as an optimization. The process that
survives is `sleep 30`; the shell's original argv, comment and all, no longer exists anywhere the
kernel will show you. `pgrep -f` reads `/proc`-equivalent command lines, so it reads `sleep 30`.

Three ways out, in order of preference:

1. **Put the marker in the executed program's own arguments**, where it survives the exec:
   `/bin/sh -c 'sleep 30' my-canary` (sets `$0`), or give the target a real argument it keeps.
2. **Defeat the exec optimization** by making the command non-simple — `sh -c 'sleep 30; :'` forks,
   so the shell stays and keeps its argv. Works, but it encodes a shell implementation detail; write
   the reason in a comment or the next reader deletes the `; :`.
3. **Do not identify by command line at all** — record the pid or pgid at launch and check identity
   from that, which is what production code should be doing anyway.

**The meta-rule, which is the expensive half.** This defect surfaced as a mutation test that came back
**green**. The mutation was applied, the guard did not redden, and the natural reading is "the guard is
fine / the invariant is untestable." The real reading was that the fixture could never observe the
thing in the first place, so it would have passed against *any* mutation. **A mutation test that does
not redden is a statement about the fixture at least as often as about the guard** — before concluding
an invariant is unguardable, prove the fixture can see the state it is asserting on. Sibling of
[[plan-supplied-test-code-is-unverified]] and [[assert-pins-outcome-not-mechanism]]; the parent rule is
[[guards-are-code]].

## War story
- 2026-08-10 (#282, PR #191) — the plan's pre-record canary was
  `/bin/sh -c 'sleep 30 # gate-run-canary'`, used to prove a launch helper would not signal a process
  group it had not identity-checked. `pgrep -f gate-run-canary` matched nothing, because the shell
  exec'd `sleep 30` and dropped the comment with its own argv. The mutation test **passed under the
  plan's own fixture** and only reddened once the fixture was repaired — meaning the guard for a
  blocker-severity defect would have shipped permanently green. A second fixture defect in the same
  change had the same shape from a different direction: a post-record wedge read its pgid via a glob
  over random `mktemp` names and usually picked an earlier, already-dead run, so it too asserted on a
  process that was not the one under test.
