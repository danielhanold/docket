---
slug: section-slice-needs-a-named-terminator
hook: "A generic /^## / terminator ends the slice at the first heading-shaped line — including one inside a fenced example — so name the terminator, and assert the terminator exists."
topics: [testing, guards, markdown]
changes: [226]
created: 2026-08-07
updated: 2026-08-07
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
