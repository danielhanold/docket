---
slug: section-slice-needs-a-named-terminator
hook: "A generic /^## / terminator ends the slice at the first heading-shaped line — including one inside a fenced example — so name the terminator, and assert the terminator exists."
topics: [testing, guards, markdown]
changes: [226, 224, 246]
created: 2026-08-07
updated: 2026-08-08
promotion_state: retained
promoted_to:
---

## Apply
Slicing one markdown section for assertion is normally written as "from my heading to the next
heading":

```
awk '/^## My section/{f=1;next} /^## /{f=0} f'
```

The terminator is a **shape**, not an identity, so it fires on the first line that merely *looks*
like a heading. Any fenced code block, quoted template, or worked example containing a `## ` line
silently truncates the slice — and truncation makes asserts **pass vacuously**: the fields they
guard were simply not in the text that was searched. Nothing about the output says the slice ended
early.

Two rules:

1. **Name the terminator** — `/^## Per discovery/` rather than `/^## /` — so the slice ends at the
   section you actually mean.
2. **Assert the terminator exists.** A named terminator that no longer matches (someone renamed the
   next section) does not error; the slice silently widens to end-of-file, and every assert inside
   it stays green for the wrong reason. The existence check is what converts a rename from a silent
   widening into a red test.

The general form: any range extraction keyed on a pattern needs a guard that the pattern still
resolves — the same reasoning as [[marker-block-range-edit]], applied to reads instead of writes.
Both failure directions here are green-passing, which is why it needs an explicit assert rather
than care.

## War story
- 2026-08-07 (#226, PR #168) — A plan-supplied assert sliced `## What a captured discovery says`
  with a generic `/^## /` terminator to check the five fields a captured discovery must carry. That
  section embeds a fenced example of a stub body, whose first line is the literal `## Why` that
  `mint-stub.sh` requires. The slice therefore ended before any of the five fields, and the asserts
  passed against an empty-of-the-relevant-content string. Fixed with the named terminator
  `/^## Per discovery/`; review then correctly required an existence assert on that terminator,
  since a later rename would widen the slice to end-of-file with every assert still green. The
  section under test was *about* stub bodies, so the collision was not a freak accident — a
  document that documents markdown is exactly where heading-shaped lines appear inside examples.
- 2026-08-07 (#224, PR #174) — a re-hit one day later, with an aggravating detail: the slice used
  the generic `/^#+ /` terminator inside a block whose **own comment cited this finding by name**.
  Citing a learning is not applying it — the author had the rule in hand, wrote the reference, and
  still shipped the shape. Fixed to the named `/^### Gate execution posture$/` plus the existence
  assert, in this suite rather than only in the sibling. The replacement was verified empirically to
  yield a byte-identical slice (47 non-blank lines both ways), so it hardened the guard with zero
  behavioral drift — worth doing whenever a terminator is tightened, since a silently *narrower*
  slice would quietly stop covering what the asserts claim to cover.
- 2026-08-08 (#246, PR #179) — **Two ways a *named* terminator still fails**, both live in one file,
  neither visible to the existence assert this finding already prescribes.
  (a) **The name is prefix-weak.** The terminator literal `claude-opus-5` is a prefix of cursor's
  `claude-opus-5-high`, so deleting claude's own last row closed the range on *cursor's* block
  instead. The existence assert stayed green (the terminator still matched — just the wrong
  occurrence), and so did the guard over the slice's last line, because the over-wide slice's final
  row is still a `build-max:` row. A terminator needs a **boundary class**, not merely a name;
  identity requires that nothing longer also matches.
  (b) **The name is a hand-written literal that content later grows past.** The round-trip slice
  ended on a cursor `finalize-change` literal that sat above cursor's own build rows and above the
  entire opencode block. Change 0192 shipped opencode and nothing noticed that sixteen rows had
  stopped reaching the resolver under test — the terminator still existed, still matched, still
  named a real line, and the slice it produced had quietly become a minority of the thing it claimed
  to round-trip. The fix in both cases was to stop writing the anchor down: derive it (the last
  harness block's sidecar-derived `build-max` row) and assert the derivation covers the shipped
  harness headers. Rule 1 of this finding says *name* the terminator; this adds the next turn of the
  screw — **a name is an identity claim, so bound it and derive it**, or the slice silently narrows
  every time the file grows.
