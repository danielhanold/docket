---
slug: best-effort-helper-on-a-sole-deliverable-path
hook: "Reusing a deliberately best-effort helper on a path where its output IS the deliverable converts a soft failure into a silent exit 0."
topics: [design, shell, contracts]
changes: [94]
created: 2026-07-19
updated: 2026-07-19
promotion_state: retained
promoted_to:
---

## Apply
"Best-effort" is a property of the **caller's** needs, never of the helper. A helper written to
degrade quietly is correct where its output is *incidental* — a board write that must not abort a
sweep, an advisory that must not fail a build. Reuse that same helper on a path where its output is
the **only** thing the path exists to deliver, and its virtue inverts: the swallowed failure now
becomes **exit 0 with empty stdout**, which every caller reads as success-with-nothing-to-report.

So when you route a new path through an existing helper, ask what the helper's failure mode means
*here*, not what it meant where it was written. Where the output is the sole deliverable, an empty
result **is** the failure and must exit non-zero with a diagnostic. Both postures can coexist in one
helper — keep the soft path soft for the incidental callers and add the hard gate on the sole-
deliverable path; that is a better answer than making the helper strict for everyone.

Note the detection asymmetry: fail-open holes are near-invisible to tests, because the natural
assert ("it exits 0") passes in exactly the broken case. Expect review, not the suite, to find them.

## War story
- 2026-07-19 (#94, PR #108) — `docket-status --digest-only` delegated to the deliberately
  best-effort `backlog_pass`, and so exited **0 with zero stdout** in two distinct failure cases:
  when the metadata worktree was missing (a fresh clone of a migrated repo), and when the render
  itself failed. A selection read that emits nothing would have been consumed as "empty backlog" —
  i.e. `drained`, stop — rather than as an error. Both were closed to exit non-zero with a
  diagnostic, while `backlog_pass` stayed best-effort for the report paths, where a failed digest
  must never abort a board write or a sweep. **Both holes were found by whole-branch review, not by
  the suite** — the tests asserted a clean exit, which is precisely what the bug produced. Related:
  [[sole-channel]], and the exit-status disambiguation this forced is recorded in ADR-0047.
