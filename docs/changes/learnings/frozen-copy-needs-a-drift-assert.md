---
slug: frozen-copy-needs-a-drift-assert
hook: 'A copy of a live file in testdata/ detects nothing by itself: read a maintained file where it lives, or keep a deliberate old sample with its source in PROVENANCE.md.'
topics: [testing, fixtures, drift]
changes: [305, 548]
created: 2026-08-13
updated: '2026-10-16'
promotion_state: candidate
promoted_to:
---

## Apply

A copy of a live file in `testdata/` asserts **nothing** by itself. Nobody diffs it. The live original can be edited freely and every test stays green, because the tests read the copy. The copy *looks* like drift detection while performing none, and the illusion is strongest in exactly the repos disciplined enough to keep fixtures at all.

Choose deliberately, and write the choice down:

- **The file is still maintained and the test should follow it:** don't copy it. Read it where it lives. A test that checks a shipped file then fails whenever an edit changes what it checks, with no copy to keep in step. In docket, no copy of a shipped file lives under `testdata/` (see `testdata/README.md`).
- **The test needs an old shape that must not follow the original:** keep it as a sample, and record where it came from (release, commit, date) in the folder's `PROVENANCE.md`, never in a folder name, so nobody later "fixes" it by syncing.

Silence is a third option that is neither: a copy that drifts and reports nothing. Mutation-test the choice like any guard ([[guards-are-code]]): edit the live file and watch the suite redden. A copy nobody compares is a correspondence guard with **zero** loops ([[correspondence-guard-runs-one-way]]).

Related: [[verify-the-claim]]: a copy is a document asserting a fact about another artifact, and is not an oracle for it.

## War story
- 2026-08-13 (#305, PR #205) — the change froze two live files into `testdata/repositories/v0.9.2/`:
  the repo's own config (the `docket-self` fixture) and `agents/harness-defaults.yml`, the sidecar
  the Go built-in agent registry mirrors sixteen×four model/effort pairs from. Both were described
  as drift detection; neither compared itself to its original, so an edit to either live file would
  have left the suite green while the vendored Go table quietly diverged from the shipped YAML.
  Review caught it (important-severity finding #2); the fix (`e492a751`) adds byte-equality asserts
  in both directions of the parity chain — Go table ↔ frozen sidecar ↔ live sidecar — so a live
  edit now forces a deliberate new versioned tree.
