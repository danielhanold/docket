---
slug: validate-the-whole-input-set-first
hook: "A tool that processes a list must validate every element before acting on any — otherwise a bad last argument surfaces only after the work on the earlier ones is already spent."
topics: [design, cli, validation]
changes: [185, 207]
created: 2026-08-01
updated: 2026-08-05
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
- 2026-08-05 (#207, PR #159) — The generative form of the same rule, where the per-item work is a
  **write** rather than a read. `sync-agents.sh` validated each `runner:` config inside its wrapper
  generation loop and `exit 1`'d on the first offender, leaving a zero-length wrapper (the caller's
  `>` had already truncated — [[atomic-generated-write]]) and every later agent stale. Two details
  worth carrying:

  **The validation must be ONE predicate, shared by the gate and the loop.** The rules already
  existed inline at the call site; the fix extracted them into a single predicate that owns the
  rules, their scope, their diagnostics, and their *ordering* (registration checked before
  required-model), then called it from a pre-flight pass above the loop. The in-loop copy was reduced
  to a can't-happen assertion for future call sites rather than deleted — a second, drifting
  spelling of the rule is exactly what the extraction exists to prevent, and the ordering itself
  turned out to be load-bearing: swapping the two checks reddened a test.

  **Do not narrow the enumeration to save time, and measure before you assume you need to.** The
  gate walks every (pass, agent, harness) triple, which doubles the config resolution — measured at
  **+0.54s** (0.82s → 1.36s, three-run averages), not guessed at. Narrowing the walk would put the
  rule's scope in a second place, reintroducing the drift the shared predicate eliminates. The
  posture this buys is deliberately stricter than before — one bad entry now refreshes **zero**
  wrappers, `nginx -t` semantics — on the reasoning that whatever was already generated was working,
  and a mixed directory of fresh, stale, and zero-length files is worse than no change at all.

  A gate placed above the loop also has to sit at the right *height*: this one goes below the
  legacy-config migration (so its resolution sees post-migration reality) but above the first write.
  And the validation-only mode (`--check`) must get the same leg, or the two modes disagree about
  what a valid config is.
