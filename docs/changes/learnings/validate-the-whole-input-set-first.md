---
slug: validate-the-whole-input-set-first
hook: "A tool that processes a list must validate every element before acting on any — otherwise a bad last argument surfaces only after the work on the earlier ones is already spent."
topics: [design, cli, validation]
changes: [185]
created: 2026-08-01
updated: 2026-08-01
promotion_state: retained
promoted_to:
---

## Apply
When a script takes a set of targets and does something expensive per target, the natural loop
shape — resolve, validate, and process each target in turn — makes the run's failure point depend
on argument order. A typo in the last path is reported only after every earlier path has already
been processed, and the user pays the full cost of the run to learn the invocation was wrong.

Split the loop: resolve and check every target up front (existence, readability, whatever the
per-item precondition is), fail with all the bad ones named, and only then enter the expensive
pass. The cost of the extra pass is a few stat calls; the cost of not doing it is the entire run.

The rule sharpens as the per-item work gets more expensive or more externally visible — profiling
a test suite, uploading, deleting, migrating. It is the batch-level form of the same instinct
behind [[decide-and-act-on-the-same-copy]]: settle everything you can decide before the point
where the action becomes irreversible or costly.

## War story
- 2026-08-01 (#185, PR #146) — `profile-asserts.sh` accepts a list of test files and runs each one,
  timestamping its output. The first implementation validated each path inside the run loop. A typo
  in the **last** argument surfaced only after the earlier files had already been profiled — several
  minutes of suite time spent on a run that was going to abort anyway. Fixed in a follow-up commit
  by validating the whole test set before profiling any of it.
