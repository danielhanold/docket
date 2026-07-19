---
slug: model-authored-values-are-untrusted-input
hook: "A value a model wrote is untrusted input to a script — and a helper copied from a sibling inherits that sibling's assumption that its values were generated constants."
topics: [shell, scripts, injection, sed]
changes: [91]
created: 2026-07-19
updated: 2026-07-19
promotion_state: candidate
promoted_to:
---

## Apply
When a script writes a value that was authored by a **model in prose** (a title, a summary, a
reason), treat it as untrusted input, not as a program constant. Two distinct failure modes, both
reached by ordinary English:

- **Replacement-side reinterpretation.** `sed "s|^title:.*|title: $VAL|"` reinterprets `&` and `\1`
  *in the replacement*. `&` is unremarkable inside a real title ("Cleanup & dedupe"), so this
  silently produces mangled or titleless files that are then committed and pushed. Escaping the
  *pattern* is not enough — this is the other side. Write through a mechanism that does not
  reinterpret: `awk`'s `ENVIRON[...]`, or a heredoc, never string-interpolated `sed`.
- **Structural injection.** A multi-line value injects whole lines into the structured region it
  lands in. In frontmatter this is a lifecycle attack: a reader whose `field()` returns the *first*
  match lets an injected `trivial: true` win over the template's later `trivial: false` — and
  `trivial` reads as build-ready, so an ungroomed stub skips the human grooming gate entirely.
  Validate at argument intake by **shape** (reject control characters), not by enumerating bad
  strings.

**Copying a helper copies its code, not its preconditions.** Before lifting a `set_field`-style
routine from a sibling script, ask what that sibling ever passes it. A routine that is provably
safe writing generated constants (a status, an ISO date, a branch name) becomes unsafe the moment
your call site passes free text. The precondition lived in the sibling's call sites, not in the
helper, so it does not travel with the copy — restate it or replace the mechanism.

## War story
- 2026-07-19 (#91, PR #104) — `mint-stub.sh` mints a backlog stub from a title an autonomous skill
  wrote. Both modes fired on one argument. (a) The first implementation interpolated the title into
  a `sed` replacement, copied verbatim from `reclaim-claims.sh`'s `set_field` — safe there because
  that script only ever writes generated constants, unsafe here where the value is English; an `&`
  mangled the title and the broken change file was pushed. Rewritten to `awk` + `ENVIRON`. (b) A
  multi-line `--title` injected arbitrary frontmatter, and because `field()` returns the first
  match, an injected `trivial: true` landed *ahead* of the template's `trivial: false` — a bypass of
  the grooming gate reached by an argument, caught in review before merge. Fixed by rejecting
  control characters at intake, mutation-proved (17 assertions redden with the guard stripped).
  Neither was visible to a fully green suite: no fixture used a title with punctuation. See
  [[green-suite-untested-branch]], [[escape-ere-metacharacters-in-key]] (the pattern-side twin), and
  [[guards-are-code]].
