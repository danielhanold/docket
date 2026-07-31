---
id: 65
slug: bare-scalar-validation-needs-an-explicit-quote-leg
title: A bare-scalar validator needs an explicit quote leg — raw-vs-consumed comparison is a whitespace test, not a bare-scalar test
status: Accepted
date: 2026-07-31
supersedes: []
reverses: []
relates_to: [58, 15]
change: 173
---

## Context

docket has two YAML value readers that parse a flow map of the shape
`agent: {model: x, effort: y}`. Each ships as a pair: a `field` reader returning what docket can
actually *consume* (character class `[^,}[:space:]]+`), and a `field_raw` companion returning what
a YAML parser would see (`[^,}]*`, trailing whitespace trimmed). The pair convention is
established — `scripts/lib/docket-frontmatter.sh` has `field`/`field_raw` (ADR-[[0058]]),
`scripts/lib/harness-defaults.sh` has `hd_field`/`hd_field_raw` (change 0168), and change 0173
added `field_of`/`field_of_raw` to `sync-agents.sh`.

The established validation idiom, from `hd_validate` (change 0168), is a single leg: reject the
entry when `consumed != raw`, with a diagnostic telling the user to "write model/effort values
unquoted and space-free". Change 0173's spec directed its new validator to mirror that message
shape and that idiom.

Probing by execution against the real `hd_field`/`hd_field_raw` — not reasoning from the source —
showed the idiom does not enforce what its message claims:

| entry | consumed | raw | `consumed != raw` fires? |
|---|---|---|---|
| `{model: "quoted-model"}` | `"quoted-model"` | `"quoted-model"` | **no** |
| `{model: two words}` | `two` | `two words` | yes |
| `{model: anthropic/claude-opus-5}` | `anthropic/claude-opus-5` | same | no (correct — it round-trips) |

A quoted but space-free value has `consumed == raw`, so the `!=` leg never fires and the quote
characters ride through into the emitted model pin verbatim. The `!=` leg is precisely "the value
contains internal whitespace" — it is not, and structurally cannot be, a general bare-scalar test,
because a quoted value is identical under both readers. The remedy text named a rule ("unquoted")
that the validator could not enforce.

## Decision

A validator that claims a value is a **bare scalar** carries two legs, not one:

1. the existing `consumed != raw` comparison — kept byte-for-byte — which detects internal
   whitespace; and
2. an explicit **quote leg**: a raw value whose first character is `"` or `'` is not a bare
   scalar and is rejected. Single-quote coverage is required, not just double.

Implemented in `validate_user_agent_values` in `sync-agents.sh` under change 0173.

The rule generalizes: raw-vs-consumed comparison is a whitespace test, so any bare-scalar claim
built only on it is incomplete. Every `field`/`field_raw` validator pair in docket, present and
future, needs the quote leg beside the comparison.

## Consequences

- The rule applies to the whole reader-pair family, so a future pair inherits a stated
  requirement rather than rediscovering the gap by probe.
- ADR-[[0015]] (model IDs are opaque passthrough, no vendor allowlist) is respected: the quote leg
  judges the value's *lexical shape*, never its content. `anthropic/claude-opus-5` and any other
  vendor-shaped ID still pass unexamined.
- The identical gap is still live in `hd_validate` (`scripts/lib/harness-defaults.sh`). It was
  deliberately left outside change 0173's scope and recorded as follow-up work, so this ADR
  documents a rule the codebase does not yet apply uniformly.
- Cost: the validator now rejects input the previous code silently ignored. A quoted value in any
  config layer previously fell through to a default and generation succeeded; it now aborts
  generation before any wrapper is written. That is the intended trade — a loud abort over a
  silently corrupted model pin — but it is a behavior change for any repo carrying a quoted value
  today.
