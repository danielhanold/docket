---
slug: mirrored-guard-enforces-its-own-property
hook: "A guard copied from a working twin enforces the property its ORIGINAL author needed — probe it by execution before naming a broader rule in your diagnostic."
topics: [guards, spec, validation]
changes: [173]
created: 2026-07-31
updated: 2026-07-31
promotion_state: candidate
promoted_to:
---

## Apply
"Mirror the existing validator" is a cheap-sounding spec instruction that hides a real question:
*which* property does the existing validator actually test? A guard is written against the one
failure its author had in front of them, and its condition is usually narrower than the sentence
someone later writes to describe it. Copy the condition, inherit the narrowness — while the
diagnostic you copy alongside it goes on promising the broader rule.

The specific trap: a byte-for-byte `consumed != raw` comparison between a narrow-class read and a
permissive read is a test for **internal whitespace**, not for well-formedness in general. A value
that is malformed in a way both readers consume identically slides straight through.

Two moves before mirroring:

- **Execute the twin against the case you intend to reject**, not just the case it was written for.
  One probe run distinguishes "this guard covers it" from "this guard happens to agree on the
  fixtures the original author had."
- **Read the diagnostic's remedy text as a specification.** If the message tells the user to do X,
  the condition must actually fail when the user does not-X. A remedy naming a rule the guard cannot
  enforce is the tell that the mirror is incomplete — and it is visible at spec time, before any
  code is written.

Related: [[verify-the-claim]], [[guards-are-code]], [[printed-remedy-state-validity]].

## War story
- 2026-07-31 (#173, PR #142 — merged) — The spec directed the new `sync-agents.sh` validator to
  mirror `hd_validate`'s single `consumed != raw` leg *and* to reject a quoted value. Probing the
  real `hd_field`/`hd_field_raw` pair by execution showed those two instructions conflict: a quoted
  but space-free value (`{model: "claude-opus-5"}`) has `consumed == raw`, so the `!=` leg never
  fires. The mirrored comparison tests internal whitespace, and the diagnostic's own remedy ("write
  values unquoted") named a rule it could not enforce. Escalated and approved mid-build; shipped
  with an explicit quote leg (single quotes included) beside the byte-for-byte `!=` leg, and
  recorded as **ADR-0065**, whose rule generalizes to every `field`/`field_raw` validator pair in
  docket. Follow-up **#0180** applies the same leg back to `hd_validate` itself — the twin that was
  copied from was equally incomplete, which is exactly how the gap propagated.
