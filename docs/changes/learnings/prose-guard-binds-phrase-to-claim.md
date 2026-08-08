---
slug: prose-guard-binds-phrase-to-claim
hook: "A guard that asserts a phrase is PRESENT survives a rewrite that keeps the words and drops the claim — bind the phrase to what it is asserted about, with a bounded gap."
topics: [testing, guards, docs]
changes: [224, 242]
created: 2026-08-07
updated: 2026-08-08
promotion_state: candidate
promoted_to:
---

## Apply
Guarding a normative sentence in prose usually gets written as "the phrase is in the file":

```
grep -q "completed successfully" "$skill"
```

That pins vocabulary, not meaning. It stays green through any edit that keeps the words somewhere
in the document while severing them from the thing they were asserted about — moving the phrase to
a different section, attaching it to a different subject, or demoting a rule to an example. The
words are the *evidence*; the claim is the **binding** between them.

Bind the phrase to its subject with a single bounded gap:

```
grep -qE "recorded status[^.]{0,80}completed successfully" "$skill"
```

The same applies to a guard asserting that two names are classified differently: adjacency
(`name_a` near `name_b`) is satisfied by a list that mentions both. Bind **each** name to its own
classification separately, so a rewrite that merges the two categories reddens.

Keep the gap bounded and small. An unbounded `.*` re-binds across paragraphs and re-admits exactly
the drift you were guarding against; a multi-hundred-character gap is unbounded in practice. Bound
it to about the length of the clause you mean, and remember the gap runs over hard-wrapped prose —
match a whitespace-collapsed haystack ([[phrase-grep-over-wrapped-prose]]) or the binding breaks on
a pure re-flow.

The pre-review check: read the guard and ask *what edit would keep this green while destroying the
rule?* If you can name one in under a minute, the guard is on the phrase, not the claim. Prose
sibling of [[assert-pins-outcome-not-mechanism]] — same defect, evidence-vs-meaning, on text
instead of behavior.

## War story
- 2026-08-07 (#224, PR #174) — three of the change's five review findings were this one shape.
  `completed successfully` was asserted as a bare phrase rather than bound to *the recorded status*
  it qualifies; the two non-verdict names were asserted as adjacent to each other rather than each
  bound to its own classification. Every one of them would have survived a rewrite that kept the
  vocabulary and dropped the binding — which, for a change whose entire subject was a contract
  sentence, would have left the guard defending nothing. Fixed by binding each claim with a single
  bounded gap. Three separate instances in one branch is the signal that this is a default authoring
  habit, not a slip: the natural way to guard a sentence is to grep for it.

- 2026-08-08 (#242, PR #186) — a convention-pointer assert flattened the entire SKILL.md and matched
  `verify-run` and `once` from unrelated paragraphs; it stayed green with the guarded sentence
  deleted (mutation-proven). Fixed by binding the window to the *Composition* paragraph and adding an
  anchor-existence assert, so a renamed paragraph fails loudly instead of silently matching nothing —
  the companion rule to binding the phrase: **also assert the window you bound it to still exists.**
