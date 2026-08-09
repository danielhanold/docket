<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0258 — Guard the config-suite's enumerated claims: export order and rung pairs](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-09-0258-guard-the-config-suite-s-enumerated-claims-export-order-and.md)**
<!-- docket:backlink:end -->

# Guard the config-suite's enumerated claims: export order and rung pairs — design

Change: docs/changes/active/0258-guard-the-config-suite-s-enumerated-claims-export-order-and.md
Consolidates #0123 (export-list order) and #0125 (rung-pair completeness): the same
guard-or-delete meta-question about a prose-only enumerated claim, ruled once, applied twice.

## Problem (verified 2026-08-07)

1. **Export order.** `scripts/docket-config.md` §Emit sells sequence as contract ("printed as
   `KEY=value` lines to stdout in this order"; "The last line is always `BOOTSTRAP=…`"; "33 lines in `shell` format; 34
   in `plain`"), and the R7 guard's own comment says pipe consumers may rely on it. But the
   fenced 34-entry list (between the "in this order" paragraph and the "33 lines" paragraph) is
   pinned only by per-key *presence* greps plus two runtime-only adjacency clusters (R7 around
   `FINALIZE_REQUIRE_PR_APPROVAL`; the AUTO_GROOM→CHANGE_TYPES→AUTO_CAPTURE_* identity cluster).
   A doc-side reorder, or a runtime reorder outside those clusters, stays green.
2. **Rung pairs.** Section S of `tests/test_docket_config.sh` pins all six ordered rung pairs of
   the three-layer `finalize.test_command` chain (s4–s9), but the "six pairs" enumeration lives
   only in the section header comment. Nothing derives the rung count from the resolver
   (`config_scalar_get`'s `committed`/`global`/`local` dispatch, read via `lcl`/`gbl` and the
   chain at `FINALIZE_TEST_COMMAND=`), so a fourth layer takes ordered pairs 6 → 12 and six
   cells go silently unpinned. The #0125 blocker resolved: ADR-0054 ruled "convert, do not
   close" — guarded source-shape anchors are a supported idiom.

## Posture ruling (applies to both legs)

**Guard the claim; do not delete or downgrade it.** For leg 1 the doc's ordering claim is real
consumer contract (R7's stated rationale) — re-specifying the list as unordered would delete a
promise callers already lean on. For leg 2 the completeness claim is the only thing standing
between a future fourth layer and six unpinned masking cells. Both guards derive their expected
side from the resolver, never from a second hand-maintained enumeration (`enumerated-floor`,
`backstop-must-compute-not-reenumerate`).

## Design

Both guards land as new sections of the `tests/test_docket_config*.sh` family, using the suite's
existing `mkrepo`/`run`/`rung` fixture helpers (one small fixture repo, one `--export` run per
format). They are written **corpus-indifferent** for the #0251 split: no `${BASH_SOURCE[0]}`
whole-file scans; anything that must scan test source iterates the family glob
`tests/test_docket_config*.sh` exactly as 0251's population guard does.

### Leg 1 — doc fence vs emission: whole-sequence equality

- **Extract the doc side.** Locate the `### Emit` heading in `scripts/docket-config.md` (a
  quoted-clause anchor per ADR-0054 — never a line number), take the first fenced block after
  it, and reduce each line to its first whitespace-delimited token (this strips the
  `REPO_ROOT … (plain format only — see below)` annotation). Control assert first: the
  extraction is non-empty and contains `DOCKET_MODE` and `BOOTSTRAP`
  (`marker-scoped-guard-needs-a-population-floor` — an anchor that stops matching must redden,
  not empty-compare green).
- **Extract the runtime side.** Keys of `--export --format plain` output (`cut -d= -f1`) and of
  default `shell` output, from one fixture repo.
- **The verdict is sequence equality, not membership:** plain keys must equal the doc token
  sequence byte-for-byte and in order (single string/diff compare); shell keys must equal the
  doc sequence with the `REPO_ROOT` token removed. One compare is inherently two-way — a
  reorder, removal, addition, or count-stable rename on *either* side reddens.
- **Numeral prose:** derive `plain_count`/`shell_count` from the same extractions and grep the
  doc for the literal sentence fragment `"${shell_count} lines in \`shell\` format; ${plain_count} in \`plain\`"` —
  so growing the list forces the prose numerals to move with it.
- The existing R7 / AUTO_* adjacency asserts **stay**: they pin change-scoped runtime claims on
  their own fixtures; the new guard subsumes their ordering coverage but removing them would
  rewrite landed changes' witnesses for no benefit.

### Leg 2 — rung-pair completeness derived from the resolver

- **Derive the layer set** from `config_scalar_get`'s dispatch: the case arms that call
  `config_scalar_from_lines … "${CONFIG_LINES_<LAYER>[@]}"` (symbol-anchored grep on
  `scripts/docket-config.sh`; the `*)` die arm excluded). Control asserts: set size n ≥ 3 and
  the three known layers (`committed`, `global`, `local`) all present — a floor plus known
  members, not an exact-set pin, so a fourth layer *grows* the set rather than reddening the
  control, and a pattern that quietly stops matching cannot empty-compare green.
- **Compute expected pairs** = all ordered pairs over the derived set: n·(n−1) pairs (6 today).
- **Declare the pinned side machine-readably:** each section-S masking fixture gains one
  adjacent marker line, `# RUNG_PAIR: <auto-holder>-><real-holder>` (e.g.
  `# RUNG_PAIR: local->committed` on s4), replacing the header comment's prose enumeration as
  the claim of record (the header keeps narrative, reworded to point at the guard).
- **The verdict is set equality:** the set of `RUNG_PAIR:` markers collected across the family
  glob `tests/test_docket_config*.sh` must equal the computed ordered-pair set exactly —
  duplicates, gaps, and unknown layer names all redden. Count equality falls out of set
  equality; no `>= 6` floor, no hand-written "6".
- Residual (accepted): a marker line could survive deletion of its fixture body. The marker sits
  inside the fixture block so the natural edit removes both; a lying orphan marker is the same
  class of residual as any labeled assert and is left to review.

### Mutation proof (both guards; run and record per suite convention)

- Leg 1: swap two adjacent entries in the doc fence → red; delete one entry → red; count-stable
  rename of one entry → red; comment out one `emit` line in the resolver → red (already red via
  presence asserts, but confirm the *new* guard also fires); stale numeral in the prose
  sentence → red.
- Leg 2: delete one s-fixture (with its marker) → red; duplicate a marker → red; simulate the
  fourth layer by adding a `staging)` dispatch arm (with a `CONFIG_LINES_STAGING` clause) to a
  scratch copy of `docket-config.sh` fed to the extraction → expected pairs go 6 → 12 → red;
  deleting the `RUNG_PAIR` grep's entire population (all markers) → red via set inequality, not
  a vacuous pass.

### Docs

`scripts/docket-config.md` needs no content change (the guard pins what it already says).
Section-S header comment reworded from claiming the enumeration to citing the guard.

## Out of scope (unchanged from stub)

- Adding config layers or keys; changing emission order.
- The #0251 population-floor/sharding rework of the same file — build-time coordination only.

## Assumptions

1. **Guard, don't re-specify — both legs.** Chosen: the house bias (ADR-0054 "convert, do not
   close"; the correspondence-guard learnings family), plus leg-1's order claim being live
   consumer contract per R7's own comment. Rejected: declaring the doc list unordered — deletes
   an explicit documented promise consumers may already lean on. Rejected: closing #0125's
   claim as a documented non-goal — leaves the known 6→12 silent-gap failure mode open for the
   cost of ~20 test lines.
2. **Leg 1 compares runtime output, not resolver source.** Chosen: `--export` output is the
   consumer-facing artifact and the suite already runs it; a source-grep of `emit` lines would
   re-derive what the run states directly and miss conditional emission (REPO_ROOT is
   format-conditional). Rejected: extending per-pair adjacency asserts to all 34 keys — O(n)
   hand-maintained asserts is the enumerated-floor shape this change exists to retire.
3. **Fence extraction anchors on the `### Emit` heading + first following fence, with a
   non-empty/sentinel control assert.** ADR-0054-compliant (quoted clause, greppable, drift
   mechanically visible). Rejected: line-number anchors (forbidden); rejected: fingerprinting
   the fence by its first line only (a prepended entry would dodge the anchor's intent).
4. **Both format sequences are pinned (plain = fence; shell = fence minus REPO_ROOT), and the
   prose numerals are derived-checked.** Rejected: pinning plain only — the doc's 33/34 claim
   and REPO_ROOT placement rule are exactly the subtle part a future edit gets wrong.
5. **Leg 2's expected side derives from `config_scalar_get`'s dispatch arms.** Chosen: it is the
   single choke point every layer read funnels through (`lcl`/`gbl` are one-line wrappers; the
   committed read calls it directly), so a fourth layer cannot land without adding an arm.
   Rejected: counting reads on the `FINALIZE_TEST_COMMAND=` chain line — pins one key's chain
   shape, breaks on an innocent refactor to multiple lines, and misses a layer added elsewhere.
   Rejected: the weaker "three readers still exist" floor #0125 offered — reddens on addition
   but never ties fixture coverage to the pair count, leaving the actual claim unguarded.
6. **Pinned side = per-fixture `RUNG_PAIR:` markers, set-equality against the computed pair
   set, collected over the family glob.** Chosen: self-registers when layers grow (author adds
   fixtures + markers; guard computes the target), split-indifferent under #0251, and set
   equality subsumes count. Rejected: counting `mkrepo "$tmp/s[0-9]"` calls or assert-name
   patterns — incidental naming, silently miscounts. Accepted residual: an orphaned marker
   without a live fixture (documented in Design; same trust class as assert labels).
7. **Existing adjacency asserts (R7, AUTO_* cluster) stay despite redundancy.** Chosen: they are
   landed changes' mutation witnesses on their own fixtures; deleting them re-opens those
   changes' proofs to save nothing. Rejected: consolidation — churn without coverage gain.
8. **Placement in the `test_docket_config*.sh` family; #0251 coupling is build-time, not a
   dependency — reciprocal `related: [251]` recorded.** 0251's spec (assumptions 7/9) already
   rules the collision: whichever lands second rebases; its glob-corpus population guard is
   indifferent to where 0258's asserts land; its two-way split targets ~25–30s per shard with
   headroom for these legs. Constraint honored here: no `BASH_SOURCE` whole-file scans — leg
   2's marker collection iterates the family glob. Rejected: `depends_on` either way — both
   designs are valid against the un-split file and against the split corpus.
9. **Dependency state:** `depends_on: []` — nothing blocks. #0123/#0125 are killed-consolidated
   parents; 0114/ADR-0054 is done and is what licenses the leg-2 source-shape anchor.
