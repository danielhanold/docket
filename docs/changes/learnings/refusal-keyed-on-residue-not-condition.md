---
slug: refusal-keyed-on-residue-not-condition
hook: "A validator that keys on the residue a bad input leaves behind misses the inputs that leave no residue and fires on legal ones that happen to look like it — detect the condition itself, upstream of whatever normalization eats the evidence."
topics: [guards, config, validation]
changes: [276]
created: 2026-08-09
updated: 2026-08-09
promotion_state: candidate
promoted_to:
---

## Apply
When you must refuse an input that some earlier stage has already mangled, the cheap-looking
predicate is the **residue**: the artifact the mangling leaves behind in the value you happen to be
holding. It is wrong in both directions at once, and both directions are silent:

- **False negative.** The residue only appears for *some* spellings of the bad input. A YAML comment
  truncating a quoted scalar leaves an unbalanced leading quote; the same truncation on an
  **unquoted** scalar leaves nothing at all. Keyed on the quote, the validator exports a fragment
  while every document promises a hard error.
- **False positive.** A legal input can carry the residue for unrelated reasons — a value that
  simply *begins* with a quote character. If the validator runs at every entry point (a config
  resolver at Step 0 of every skill), that false positive does not degrade one operation; it bricks
  the tool for that repo.

The fix is positional, not cleverer pattern-matching: **read the raw leaf before the normalization
step that destroys the evidence**, and test the condition there. Then bound the blast radius —
refuse only in the layer that actually wins resolution, so a repo that has already fixed its config
is never held hostage by a stale lower-precedence one.

Where a spelling genuinely cannot be distinguished (the source language is itself ambiguous), record
the residual gap explicitly rather than papering it with a predicate that guesses.

Related: [[byte-pattern-guard-matches-a-spelling]] (the pattern-vs-property version),
[[guard-keyed-on-presence-not-provenance]] (the layered-precedence version).

## War story
- 2026-08-09 (#276, PR #190) — Dummy mode's `persona:` refusal for a `#`-bearing value keyed on the
  unbalanced leading quote that comment-truncation leaves in a *quoted* scalar. It therefore missed
  the unquoted form entirely — a truncated fragment was exported silently while the spec, the script
  contract, and the README all promised a hard error — and it aborted on a legal persona whose text
  merely began with a quote. Because `docket-config.sh` runs at every skill's Step 0, that second
  limb would have made the tool unusable in such a repo. Re-keyed to read the raw leaf **before**
  normalization can eat the `#`, and scoped so a `#`-broken persona in an overridden layer is not
  fatal. One shape stays unresolvable and was recorded rather than guessed at: `persona:#foo` with no
  space after the colon resolves to empty and falls through to the shipped default, because YAML
  itself is ambiguous there.
