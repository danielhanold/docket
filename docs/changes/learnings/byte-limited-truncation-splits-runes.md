---
slug: byte-limited-truncation-splits-runes
hook: "A byte-offset cut through text produces malformed output — back every truncation off to a character boundary."
topics: [diagnostics, encoding]
changes: [313]
created: 2026-08-16
updated: 2026-08-16
promotion_state: candidate
promoted_to:
---

## Apply

Any bound expressed in **bytes** — a stderr excerpt, a bounded detail string, a log field cap —
must back its cut off to a **character boundary** before emitting. A raw byte slice of UTF-8 text
splits a multibyte rune whenever the limit lands mid-sequence, and the result is a diagnostic that
is subtly malformed exactly in the cases a human is reading it to debug something. The failure is
input-dependent, so it survives every test whose fixture happens to be ASCII: test it with a
multibyte fixture positioned so the limit falls inside a rune, not merely near it.

The byte limit itself is usually the right bound (it caps memory and transport size) — the fix is
never to switch to counting characters, only to move the cut backward to the nearest boundary and
leave the limit, the redaction, and the truncation marker unchanged.

## War story

- 2026-08-16 (#313, PR #213) — the deep review's one minor finding: `githubcli/failure.go`'s
  `stderrExcerpt` and `workspace/inspect.go`'s `boundedDetail` both cut at a raw byte offset, so a
  `gh` stderr excerpt or a workspace-inspection detail containing non-ASCII text could end in a
  broken rune. Fixed in-branch with a shared `trimToRuneBoundary` (`unicode/utf8`) plus a focused
  test at each site; limits, redaction, and marker text untouched.
