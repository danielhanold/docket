---
slug: defaulted-param-hides-caller-wiring
hook: "A parameter the callee defaults makes the caller's wiring invisible — assert the RESOLVED non-default value, or deleting the argument reddens nothing."
topics: [testing, cli, defaults]
changes: [202]
created: 2026-08-05
updated: 2026-08-05
promotion_state: candidate
promoted_to:
---

## Apply
When a caller passes `--x "$X"` and the callee reads `${X:-<default>}`, the argument is
**behaviorally invisible** on any repo running the default. Delete it from the caller and the suite
stays green, the tool keeps working, and the regression surfaces only for the users who configured
a non-default value — the exact population the knob exists for. The fallback that makes the callee
robust is what makes the wiring untestable.

The natural assert does not close it either. Asserting that the callee was invoked with
`--x docs/results` passes identically in three different worlds: the caller forwards the resolved
config, the caller hardcodes the literal, or the caller passes nothing and only the callee's
fallback supplies it. **Set a non-default value in the fixture and assert *that* value arrives.**
Anything else pins the string, not the plumbing.

Two companions:

- A call site whose arguments are asserted one-by-one is an [[enumerated-floor]] — the argument
  nobody listed is the one that rots. Prefer deriving the expected set from the callee's own
  parser, or at minimum assert the full argument vector rather than a per-flag grep.
- The reason to bother is the same one that justifies any guard: a gate that is only ever tested
  open is a gate nothing proves is closed ([[guards-are-code]]).

## War story
- 2026-08-05 (#202, PR #158) — `docket-status.sh`'s `health_checks` call had an assert for every
  argument it passes **except** `--results-dir`; deleting that argument reddened nothing. The
  callee's `${RESULTS_DIR:-docs/results}` fallback meant a repo configuring a non-default
  `results_dir` would have had leg A's results arm scanning a nonexistent directory forever, with a
  permanently green suite. The fix pins the **resolved** value by driving the fixture with a
  non-default `RESULTS_DIR` — asserting the default string would have passed against the broken
  wiring. The finding had been raised and consciously deferred at #0113's merge gate; it took its
  own change to close, which is the ordinary cost of deferring a coverage hole rather than the
  exception.
