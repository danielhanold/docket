---
slug: frozen-copy-needs-a-drift-assert
hook: "A fixture frozen as a copy of a live file detects drift only if a byte-equality assert ties the two — the copy alone promises the check and performs none."
topics: [testing, fixtures, drift]
changes: [305]
created: 2026-08-13
updated: 2026-08-13
promotion_state: candidate
promoted_to:
---

## Apply
Freezing a copy of a live file into `testdata/` is a real technique: it pins the shape a test was
written against, so the test keeps asserting one thing while the live file evolves. But the copy
by itself asserts **nothing**. Nobody diffs it. The live original can be edited freely and every
test stays green, because the tests read the frozen copy — which is the whole point, and also the
trap. The freeze *looks* like drift detection while performing none, and the illusion is strongest
in exactly the repos disciplined enough to freeze fixtures at all.

A frozen copy needs one of two things, chosen deliberately and written down:

- a **byte-equality assert** against the live original, when the copy is meant to track it. A live
  edit then reddens the suite and forces the author to mint a **new versioned fixture tree** on
  purpose, rather than silently invalidating the old one; or
- an explicit statement that the copy is a **historical snapshot** that must not track the
  original — in which case name the version it snapshots, so nobody later "fixes" it by syncing.

Both are one-line-cheap, and the choice between them is the fixture's actual contract. Silence
means the third option, which is neither: a copy that drifts and reports nothing. Mutation-test it
like any guard ([[guards-are-code]]) — edit the live file, watch the suite redden — and note the
direction problem this shares with [[correspondence-guard-runs-one-way]]: a copy nobody compares
is a correspondence guard with **zero** loops, not one.

Related: [[verify-the-claim]] — the frozen copy is a document asserting a fact about another
artifact, and is not an oracle for it.

## War story
- 2026-08-13 (#305, PR #205) — the change froze two live files into `testdata/repositories/v0.9.2/`:
  the repo's own config (the `docket-self` fixture) and `agents/harness-defaults.yml`, the sidecar
  the Go built-in agent registry mirrors sixteen×four model/effort pairs from. Both were described
  as drift detection; neither compared itself to its original, so an edit to either live file would
  have left the suite green while the vendored Go table quietly diverged from the shipped YAML.
  Review caught it (important-severity finding #2); the fix (`e492a751`) adds byte-equality asserts
  in both directions of the parity chain — Go table ↔ frozen sidecar ↔ live sidecar — so a live
  edit now forces a deliberate new versioned tree.
