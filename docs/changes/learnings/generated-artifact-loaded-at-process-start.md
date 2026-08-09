---
slug: generated-artifact-loaded-at-process-start
hook: "An artifact the harness loads once at process start cannot be validated by the session that edited it — the running process is still holding the old copy."
topics: [environment, testing, agents]
changes: [271]
created: 2026-08-09
updated: 2026-08-09
promotion_state: candidate
promoted_to:
---

## Apply
Some artifacts are read by the harness **once, at process start**, and never re-read: agent
definitions and their generated wrappers, skill registries, shell profiles, loaded env. When a
change edits one of those, the session that made the edit is the **worst possible place to test
it** — it is still executing against the snapshot taken before the file existed in its new form,
and it will observe the *old* behavior no matter how correct the new file is.

Two failure directions, and both look like evidence:

- **False red.** You edit the wrapper, dispatch, watch the old payload go out, and "fix" a file
  that was already correct.
- **False green.** The old copy happens to do the right thing for the case you tried, and you
  report a payload change as verified when nothing exercised it.

So: when a change writes a start-time-loaded artifact, say so **in the results file's human-verify
section**, name the restart as a precondition of the test, and do not let the run claim the payload
is runtime-validated. A hermetic test over the *generator* (does `sync-agents.sh` write the bytes
we intend?) is real and is what an autonomous run can honestly deliver; a test over the *loader*
requires a fresh process and therefore a human. Distinguish the two in the record rather than
letting the generator's green stand in for the loader's.

The diagnostic that settles it in one shot: edit the file on disk to a value that could not
possibly come from anywhere else, dispatch once in the same session, and read what actually went
out.

## War story
- 2026-08-09 (#271, PR #188 — merged) — The change fixed the generated dispatch shim's payload
  (making the child's task text a required slot rather than an optional tail) and could not
  validate it from the session that made it. Claude Code loads agent definitions at process start:
  a wrapper edited on disk to dispatch `--runner opencode2` still dispatched `--runner opencode` on
  the very next call in the same session — measured on this branch, which is what turned a
  suspicion into a stated precondition. The shim payload therefore shipped with the generator's
  tests green and the loader path **not runtime-validated**, recorded as such rather than implied
  to be covered. Same shape as the delegation verdicts this change shipped `unverified`: the honest
  move is to name what the run could not measure ([[external-truth-needs-a-human-checkpoint]]),
  not to let an adjacent green imply it.
