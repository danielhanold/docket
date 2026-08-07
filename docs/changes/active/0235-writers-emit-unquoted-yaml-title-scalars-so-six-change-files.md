---
id: 235
slug: writers-emit-unquoted-yaml-title-scalars-so-six-change-files
title: Writers emit unquoted YAML title scalars, so six change files fail to parse
status: proposed
priority: medium
type: fix
created: 2026-08-07
updated: 2026-08-07
depends_on: []
related: []
discovered_from: [234]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

Six change files on `docket` have frontmatter that a real YAML parser rejects
(`mapping values are not allowed here`). Every one is an unquoted `title:` scalar carrying a colon:

| id | title fragment |
|---|---|
| 0121 | `The manifest's elsewhere: check proves...` |
| 0173 | `...a model ID containing / or :` (trailing colon, end of value) |
| 0211 | `...stops after the build: commits on an unpushed branch...` |
| 0217 | `Clear change 0202's three minor findings: dead guard...` |
| 0219 | `aborted-run's sixth signature: PR opened and pr: written...` |
| 0234 | `Split gate-execution.md: probe evidence should not sit...` |

**docket already knows this rule and still writes violations.** AGENTS.md carries the promoted
`yaml-scalar` finding ("Quote any hand-authored YAML scalar carrying a colon-space or a boolean
keyword. Today's grep/awk reader tolerating it is not evidence it is well-formed"), ADR-0065 records
it, and `board-checks.sh`'s `scalar-form` check (change 0191) *detects* it. The loop mints a
malformed file and then reports the violation back to itself, warn-only, forever.

The gap is that the rule is scoped to **hand-authored** scalars while the highest-volume writer is a
**script**. `mint-stub.sh`'s `set_field` writes `print key ": " val` — byte-for-byte by deliberate
design, so that a model-authored title is never corrupted by sed/awk metacharacters (its B1 comment
is explicit about this). That protection is against *shell and regex* injection. It does nothing
about *YAML syntax*: a title containing `: ` produces a file no YAML consumer can read. The write
path has no quoting step at all.

`scalar-form` also has a hole of its own, which is why the count above is six and the check reports
five: it tests for `': '` (colon-space) and for bare booleans, but **not for a colon at the end of
the value**. 0173's title ends in `/ or :` — invalid YAML, undetected.

Impact today is bounded but not zero: docket's own readers are grep/awk (`docket-frontmatter.sh`)
and tolerate it, which is exactly the false comfort AGENTS.md warns about. Any consumer that
actually parses — the GitHub mirror, an external tool, a future reader — sees a broken file, and the
failure mode is a parse abort over the whole manifest rather than one bad field.

## What changes

- **Quote at the write boundary.** `set_field` (and any sibling writer of a free-text scalar) emits
  a properly quoted YAML scalar when the value needs it. Byte-for-byte fidelity of the value must
  survive — the B1 property is not to be traded away, and quoting must handle a value that itself
  contains quotes.
- **Repair the six existing files**, active and archive alike. An archived file is immutable as a
  *record*, but a syntactically broken one is not a record anyone can read.
- **Close the `scalar-form` gap**: a value ending in `:` is as invalid as one containing `': '`.
- **Guard it.** A test that parses every change file's frontmatter as YAML would have caught all six
  at once and is the real backstop; the per-leg `scalar-form` unit assertions are the diagnostic.

## Out of scope

- The `field-domain` finding on change 0189 (`title contains '|'`, which injects board columns).
  Adjacent and also a title-sanitization issue, but a different failure — a rendering corruption,
  not a parse error — with its own existing check.
- Rewording any title for style. This is a syntax fix; titles keep their words.
- Introducing a YAML library dependency into the runtime read path. The readers stay grep/awk;
  only the *writer* and the *guard* are in question.

## Open questions

- Quote-vs-reject at the write boundary: does `mint-stub` quote silently, or refuse a title that
  needs quoting and make the caller reword? Silent quoting is friendlier and matches "the script
  does the mint"; refusal keeps titles readable in the raw file.
- Which quoting form — single-quoted (only `'` needs doubling) is simpler than double-quoted for
  prose that may contain backslashes.
- Does the guard belong in the suite (hermetic, sees only fixtures) or in `board-checks` (sees the
  live tree)? The six real violations live where the suite cannot see them, which is precisely how
  they survived.
- Do the interactive creation paths (`docket-new-change`'s template fill) share the same hole, or
  does a human-authored title get quoted by the model per AGENTS.md today?
