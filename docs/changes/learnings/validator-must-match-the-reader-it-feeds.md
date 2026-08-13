---
slug: validator-must-match-the-reader-it-feeds
hook: "A hand-enumerated validation predicate drifts from the parser it feeds — the builder then renders a document its own reparse refuses; derive the rejected set from the reader's rules and fuzz the round trip."
topics: [serialization, validation, fuzzing, yaml]
changes: [306]
created: 2026-08-13
updated: 2026-08-13
promotion_state: candidate
---

## Apply
When a writer validates values before serializing them, the accept set must be the *reader's* accept
set, not a hand-listed one. Enumerating "the illegal characters" from memory reproduces the obvious
cases and misses the library's own extra refusals, so the writer emits bytes it cannot read back.
Route every validation site through one shared predicate (never two parallel enumerations that must
be kept in sync), and prove the correspondence with a round-trip fuzz target rather than examples —
the failing input is the one nobody thought to write down.

## War story
- 2026-08-13 (#306, PR #206) — `FuzzValueRoundTrip` crashed on `String("")`: validation
  rejected only C0 controls plus DEL, but go-yaml v3.0.4's reader also refuses C1 controls, so the
  builder could render a document its own reparse rejected. Fixed by routing `Value.validate` and
  `validBlockContent` through a single `illegalTextRune` predicate covering all Unicode `Cc` plus
  U+FFFE/U+FFFF; review then extended it to U+2028/U+2029, which the YAML scanner treats as line
  breaks and which would otherwise corrupt a value silently. The minimized crasher is committed as a
  seed under `internal/document/testdata/fuzz/FuzzValueRoundTrip/`. Two lessons in one: the fuzz
  target found what the example tests could not, and the *second* miss (U+2028) was found only
  because a human-grade review re-asked the same question after the first fix.
