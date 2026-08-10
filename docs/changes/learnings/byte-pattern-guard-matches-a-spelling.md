---
slug: byte-pattern-guard-matches-a-spelling
hook: "A grep-based ban or shape predicate matches a spelling, not the property — bound it on both sides, and where equivalent spellings survive, assert the limitation in the guard's header instead of writing it in a comment."
topics: [guards, grep, testing]
changes: [246, 276, 286]
created: 2026-08-08
updated: 2026-08-10
promotion_state: candidate
promoted_to:
---

## Apply
Every guard built from `grep` is a **byte pattern standing in for a property**, and the two are
almost never the same extent. The gap opens in both directions at once, and each direction fails
green:

- **Too narrow — the pattern covers one spelling of the property.** A scanner has no awareness of
  the quoting that produced its input. In bash, `"\\b"` and `'\b'` deliver the identical two bytes
  to grep, so a ban written against the escaped source form leaves the unescaped form entirely
  unguarded while the guard's name and its diagnostic both claim to ban the *construct*.
- **Too wide — the pattern has no boundary, so incidental text satisfies it.** A shape predicate
  looking for a config key `agents` is satisfied by a script that merely contains its own
  hyphenated *name* (`sync-agents.sh` logging `"sync-agents: $*"`). Strip every real reader out of
  the consumer and the guard stays green: it was never testing what it claimed.

Three moves:

1. **Bound both sides explicitly**, and pick the class by asking what characters can legitimately
   abut the token. `[^[:alnum:]_-]` and `[^[:alnum:]_]` differ by exactly the hyphen, and the
   hyphen is what admits a tool's own kebab-case name.
2. **Mutation-test by deletion, not by addition.** Add a violating line and watch it redden proves
   the pattern matches something. Delete every legitimate site and watch it *stay* green is what
   exposes an unfalsifiable anchor.
3. **When a spelling survives on purpose, assert the limitation rather than commenting it.** A
   header that says "this class covers only the escaped form" is prose that ages; an assert that
   the count of surviving sites is what you think it is cannot. **Compute that count** — do not
   write it — per [[enumerated-floor]] and [[backstop-must-compute-not-reenumerate]]; a written
   figure is a guess, and estimates in this territory are routinely off.

Related: [[escape-ere-metacharacters-in-key]] (the pattern built *from* a key needs the inverse
care), [[correspondence-guard-runs-one-way]].

## War story
- 2026-08-08 (#246, PR #179) — Both directions, in one change, both found at review rather than by
  the suite. **Too narrow:** the new portability class banned the word-boundary escapes `\\b`,
  `\\<`, `\\>` in their double-quoted source spelling. The single-backslash spelling delivers the
  same bytes to grep and was unguarded, so the guard's header overclaimed. The fix deliberately did
  *not* widen scope mid-review — it corrected the header to state the limitation, **asserted** that
  limitation rather than commenting it, made the site count computed rather than written, and filed
  the residual work as #0262. The computed figure justified itself immediately: review estimated
  ~42 surviving sites, the measurement returned 47 before the change's own additions and 48 after.
  **Too wide:** the `elsewhere:` shape predicate carried no left boundary, so one of its six
  entries was unfalsifiable — `agents` matched `sync-agents.sh`'s own logging of its name. Closed
  with a left boundary class of `[^[:alnum:]_-]`; the reviewer's own suggested class omitted the
  hyphen and would have re-admitted exactly the match being removed.
- 2026-08-09 (#276, PR #190) — **Too narrow again, this time in the guard written to close a class
  that had just cost two merge-gate reds.** The new repo-wide pipe-shape guard enumerated its
  early-exiting consumers as `grep` and `head`. `awk … exit` closes stdin exactly the same way, and
  **three live sites survived the sweep — two of them inside the very file that had gone red twice**.
  The enumeration was written from the two spellings that had actually failed, which is the most
  seductive version of this mistake: the sample that motivated the guard becomes the guard's
  definition of the property. Re-keyed on shape across `grep`, `head`, `awk … exit`, `sed … q`, and
  `read`, with an optional path prefix, and its `KNOWN IMPRECISION` header widened to state what the
  predicate actually delivers.

- 2026-08-10 (#286, PR #192) — the too-wide direction, at its smallest scale: `grep -qF -- "|| true"`
  over an extracted fence stayed green with `|| true` deleted from the executable line, because the
  fence's own explanatory comment quotes the literal it explains. When a guard reads a document that
  *discusses* the thing it guards, prose and invocation are byte-identical — the assert must strip
  comments and match only executable lines. The build filed it as an accepted residual; review
  disagreed and proved the repair by A/B against the mutated file (old expression green, new red).
