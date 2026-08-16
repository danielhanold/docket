---
slug: frozen-corpus-covers-what-it-contains
hook: "A frozen real-world corpus covers what it actually contains, not what the plan assumed — inventory it before designing coverage on top of it, and record the empty categories where the fixture lives."
topics: [fixtures, testing, planning]
changes: [310]
created: 2026-08-16
updated: 2026-08-16
promotion_state: retained
---

## Apply

When a plan pins a real snapshot (a tag, an exported production tree, a captured API response) as a
semantic fixture, the plan is asserting coverage it has not verified: it names the paths the corpus
will exercise before anyone has enumerated what the snapshot holds. Real data is lumpy — whole
categories the plan takes for granted can be **absent**, and a corpus test written over an empty
category is green and vacuous.

So, in order:

1. **Inventory the snapshot first**, as its own step, before writing a single assertion against it.
   Count the entities per category the plan claims coverage for.
2. **Re-scope the corpus test to what is there** — the paths the real content genuinely exercises —
   and keep the absent categories covered by constructed fixtures (fakes, hand-built trees), which
   is what they were always the right tool for.
3. **Write the scope down beside the fixture**, not only in the results file: a `PROVENANCE.md` in
   the corpus directory naming the counts, the empty categories, and where their coverage actually
   lives. The next reader's default assumption is that a real corpus covers everything real; only a
   note at the fixture corrects it.
4. Treat legitimate-but-surprising properties of the snapshot (a config that resolves with
   diagnostics, a schema version behind head) as **part of the oracle**, asserted explicitly, rather
   than as noise to be filtered — they are the reason a real corpus is worth freezing at all.

The failure this prevents is the quiet one: a suite that reports a real-world corpus as covering the
feature, while the feature's hardest semantics were never fed a single row.

## War story

- 2026-08-16 (#310, PR #212) — the read-only status/health slice froze docket's own `v0.9.3` tag as
  a semantic corpus, on the plan's assumption that a real docket repo would exercise the whole
  status projection end to end. The tag is metadata-branch content and carries **terminal records
  only** — 9 archived changes and 5 Accepted ADRs, with no active changes, no learnings ledger, and
  no stacked changes. So the corpus exercised the complete-corpus inventory and health/validation
  path against an *empty active projection*: readiness, the ready queue, and effective-base
  semantics were never touched by it, and stayed covered by the fake-reader application tests in
  `status_test.go`. The reconciliation was recorded in the results file and in
  `testdata/repositories/v0.9.3/status-corpus/PROVENANCE.md`. The same pass found the frozen
  production `.docket.yml` resolves with 4 config diagnostics (3 deferred-capability errors, 1
  deferred-setting notice) — kept in the 6-finding oracle as an assertion rather than filtered out.
