# Complete the `finalize.test_command` cross-layer masking matrix — results

Change: #112 · Branch: feat/pin-the-reverse-cross-layer-masking-for-the-committed-over-l · PR: <url> · Plan: docs/superpowers/plans/2026-07-21-reverse-cross-layer-masking-matrix.md · ADRs: none

## Verify (human)

Nothing interactive is required — the change is test-only, `tests/test_docket_config.sh` is at
248 ok / 0 NOT OK, and the full repo sweep ran 53 files with `fail=0`. Two judgment calls deserve
your eye at the merge gate:

- [ ] **`s8` and `s9` are matrix-completeness witnesses, not uniquely-discriminating ones.** Only
      `s7` reddens under a mutation that leaves every other assert green (M3, the
      committed-rung-specific clear). `s8`'s witness is shared with `s6`, and `s9`'s with `s4`/`s5`.
      The spec chose this openly (A2): `guards-are-code` requires that a guard redden when the thing
      it guards is broken, not that it be the sole detector, and demanding uniqueness would force
      contrived mutations modelling no plausible refactor. The section header comment states the
      limitation rather than overselling it. Confirm you accept two fixtures earned on completeness
      rather than unique discriminating power.
- [ ] **The six-pair completeness claim is unguarded prose.** With three rungs there are exactly six
      ordered pairs and all six are now pinned — but that enumeration lives only in the section
      header comment. Nothing derives the rung count from code, so a future fourth config layer
      would take the pair count to 12, leave six cells unpinned, and make the comment false with
      zero test failures. Captured as change #125 rather than fixed here (see *Follow-ups*).
      Confirm you agree it was correctly deferred.

## Findings

**All three per-task reviews and the whole-branch review came back clean — no Critical or Important
findings anywhere on the branch.** That is unusual enough to be worth explaining rather than
celebrating: the fixtures were fully specified in the plan, and the plan's own values were checked
against the running code before any subagent was dispatched. The verification effort went into
*proving the guards fire*, not into repairing them.

**The mutation matrix is the actual deliverable.** All 18 cells (M1/M2/M3 × `s4`–`s9`) were run
against the real resolver and every cell matched prediction:

| mutation | s4 | s5 | s6 | s7 | s8 | s9 | totals |
|---|---|---|---|---|---|---|---|
| M1 per-layer collapse, before the chain | RED | RED | green | green | green | RED | 245 ok / 3 |
| M2 blanket "any rung says `auto` ⇒ unset" | green | green | RED | RED | RED | green | 245 ok / 3 |
| M3 committed-rung-specific clear | green | green | green | **RED** | green | green | 247 ok / 1 |

M3 is the load-bearing row and the entire justification for the change: it reddens `s7` **alone**
across all 248 asserts, meaning every one of change 0106's five asserts stays green under a
refactor that would silently drop a real repo's local `test_command`. That is precisely the hole
0106's review predicted and could not close from inside its own scope.

**The whole-branch reviewer rebuilt the matrix independently rather than trusting the report.** It
copied the resolver and test file into an isolated tree, re-applied M1/M2/M3 by content-anchoring,
and re-ran all four states — confirming the deltas suite-wide rather than only within section S.
That is the `guards-are-code` rule "treat a surviving mutant as a defect until proven otherwise —
re-derive the anchor's count yourself, never trust an implementer's narrative" applied to a report
that happened to be accurate.

**`s7` is not a re-pin of existing coverage.** The reviewer checked it against the pre-existing `L2`
fixture (`tests/test_docket_config.sh:503`), which also asserts `make local-test` resolving from the
local rung — but with the committed key *absent* rather than set to `auto`. `L2` is untouched by M3;
`s7` reddens. Distinct scenarios, and the near-collision was worth ruling out explicitly.

**The `s8` comment's stated reason was verified true, not assumed.** The comment explains that
`.docket.yml` is kept (with the key absent) for consistency with the main-mode shape, *not* because
omitting it would break resolution. The reviewer built the fixture both ways and ran the resolver:
without the file, `FINALIZE_TEST_COMMAND=make local-test` with `BOOTSTRAP=CREATE_ORPHAN`; as
shipped, the same value with `DOCKET_MODE=main` / `BOOTSTRAP=PROCEED`. Both halves of the claim hold.
This matters because a fixture comment encoding a false reason is the exact failure `verify-the-claim`
exists to stop.

**A forward claim written before its subject existed was checked once it did.** Task 1's header
paragraph asserted that `s8` and `s9` would share `s6`'s and `s4`/`s5`'s mutations respectively —
a claim about fixtures that did not yet exist. Verified true against the completed matrix at Task 3.

**Reconcile absorbed real drift.** Spec A8 predicted concurrent resolver work as the only realistic
drift and it happened: change 0102 (`finalize.require_pr_approval`) merged on 2026-07-21, after this
spec was authored, shifting the chain `:194 → :195` and the collapse `:201 → :202`. The spec and the
change body were corrected at reconcile. 0102's own follow-up commit had already re-anchored the
section-S header, so the header needed only its scope retitled — the build was told explicitly not
to "restore" the stale numbers.

**Plan-mandated finding, adjudicated rather than suppressed.** The whole-branch reviewer raised the
fixture duplication (`s7`/`s8`/`s9` are near-copies of each other and of `s4`–`s6`) as a finding
requiring adjudication, since spec A3 explicitly rejected extraction into a table-driven loop.
Having read all six fixtures, the reviewer **agreed with A3**: the non-uniformity is real (two-phase
writes, controls on some cells only, a committed-file rewrite in `s5`), and a table would need
escape hatches. Recorded here so the trade is visible rather than invisible.

**One accepted Minor, deliberately not fixed.** The plan's Task 1 line citations (`:1029`/`:1105`)
are off by one against the real file (the header is at `:1030`). The implementer content-anchored
and no error resulted. Left as-is: the plan is an archived record of intent, not a live contract,
and post-hoc editing a committed plan to correct a stale line number is exactly the treadmill that
makes line-number anchors questionable. It stands as evidence for change #114's open question
instead.

## Follow-ups

Two stubs auto-captured from the whole-branch review (`auto_capture` enabled), both
`discovered_from: [112]`:

- **#125 — decide whether the rung-pair completeness claim should be mechanically enforced.** The
  six-pair enumeration is prose in a comment; nothing derives the rung count from code, so a fourth
  config layer would silently leave six cells unpinned. This is the `correspondence-guard-runs-one-way`
  shape — the matrix is proven only in the direction "for each pair the author enumerated, a fixture
  exists." Genuinely hard to close well: an enforcement guard must read the resolver's source shape,
  which is the brittle-anchor failure mode **change #114** is currently weighing, and a
  hand-maintained pair list is an `enumerated-floor` that ages into the gap it was written to close.
  Should be groomed together with #114.
- **#126 — apply the poison-value prelude uniformly to every resolver `eval` in the config suite.**
  Auditing poison coverage surfaced that the `L2` fixture (`tests/test_docket_config.sh:500`) evaluates
  resolver output with no `FINALIZE_TEST_COMMAND=__poison__` line, unlike the section-S convention.
  Pre-existing and outside this change's diff, so correctly left alone — but it is a latent
  stale-value hazard of exactly the kind the prelude exists to prevent. The stub requires the fix be
  *proven* by demonstrating the hazard before adding the line, so it does not ship decoration.

No plan deviations. The three tasks executed as written; no step's expected output needed adjusting.
