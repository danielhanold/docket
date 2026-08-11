<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0260 — Tier finalize's in-context dispatches and name the push-denial posture](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-11-0260-tier-finalize-s-in-context-dispatches-and-name-the-push-deni.md)**
<!-- docket:backlink:end -->

# Tier finalize's in-context dispatches and name the push-denial posture — results

**Change:** 0260 · **Date:** 2026-08-11 · **Type:** fix (docs + test rewiring, no behavior change)
**ADR produced:** [ADR-0086](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0086-in-context-gating-dispatch-carved-out-of-the-tier-taxonomy.md)
**Follow-up minted:** #0291

## What shipped

Three surfaces, four plan-task commits plus four fix commits:

- **`skills/docket-convention/SKILL.md`** — a carve-out paragraph immediately after the A/B/C
  dispatch-capability tier table. `docket-rebase-resolver` and `docket-integration-repair` sit
  *outside* the taxonomy, not in a fourth row, because their contract is an in-context report gating
  the merge rather than git state on `metadata_branch`. Posture on genuine unavailability is
  finalize's pre-existing abort-and-report; inline substitution is forbidden as the same
  self-approval shape Tier B rejects for the critic.
- **`skills/docket-finalize-change/references/gate-failure.md`** — the single canonical site marker
  (one clause per agent noun), plus two new members in *abort-and-report points (the full set)*:
  dispatch-unavailability, and a harness or permission denial of the gate's post-rebase
  `--force-with-lease` push, conditioned on the convention's *Harness-native recovery* retry first.
  The stale "six distinct abort reasons" count was de-numeralized rather than re-counted.
- **`tests/`** — the two sites became ordinary `check_site` rows at posture label `carve-out`;
  `PENDING_TIER` shrank to empty but survives as an empty-pinned guard; the coherence loop now
  dispatches on posture-label *shape* and fails on an unrecognised label; floors re-derived.

The change is documentation and guards only. No script changed, and no runtime behavior moved — what
moved is what an agent holding a failed dispatch is told to do, and what the suite will let a future
editor quietly undo.

## Verification

Full suite green at `07adf73f`: `SUITE files=105 passed=105 failed=0 asserts=8439 wall=185s`.

The only `OVER BUDGET:` line is `test_sync_agents_runners` at 185s against a 60s ceiling — the
**pre-existing** #0280 breach, untouched by this change. `tests/test_dispatch_capability.sh` (1s) and
`tests/test_finalize_gate.sh` (0s) both sit well inside their 10s rows, so
`tests/runtime-budgets.tsv` needed no raise.

`tests/test_skill_size_budgets.sh` did need raises, because the added prose is the deliverable:
`skills/docket-convention/SKILL.md` 6450 → 6650 words and 380 → 385 lines;
`gate-failure.md` 900 → 1150 words and 35 → 40 lines. Each carries a per-raise rationale in that
file's house style. The line axes were raised in the fix loop after review pointed out that the
original 377/380 and 33/35 margins reproduced exactly the near-zero failure mode that file's own
preamble forbids.

## Review — six findings, all dispositioned

Reviewed at the **deep** rung (highest profile routed was `premium`, for the integration-repair
round; the 720-line diff was under the 1500-line bump threshold).

| # | Severity | Finding | Disposition |
|---|---|---|---|
| 1 | important | The coherence loop's new shape filter `continue`d past carve-out rows, replacing a *derived* cross-file property with a hand-list — a third carve-out site could have been invisible to the convention | **fixed** — `2c06ebc5` |
| 2 | minor | The paragraph's central claim was pinned by a bare token; negating the posture kept it green | **fixed** — `3d3c40ec` |
| 3 | minor | The de-numeralization guard enumerated spellings and stopped one short of the member count | **fixed** — `3d3c40ec` |
| 4 | minor | New proximity asserts ran line-scoped over paragraph records, so a pure re-flow reddened policy asserts | **fixed** — `07adf73f` |
| 5 | minor | Both raised budget rows kept near-zero *line* headroom | **fixed** — `3d3c40ec` |
| 6 | minor | At both gate steps the dispatch verb precedes the blocking read of the file that owns the posture | **minted** — #0291 |

Finding 6 was deliberately **not** fixed in-branch. The fix edits
`skills/docket-finalize-change/SKILL.md`'s step 2 and step 5 dispatch sentences — the exact
placement the spec's **Assumption 3** audited and declined, to avoid two marker sites to keep in
agreement. An autonomous run reversing a human-audited assumption inside a branch scoped to honour
it is the wrong move, so it went to the backlog with the argument written down.

## What the guards actually caught

Three of the six findings are the same defect class, and it is worth naming: **an assert that passes
by coincidence rather than by construction.**

- Finding 2's predicate matched a token that appeared once anywhere in the paragraph.
- Finding 3's enumerated `(six|seven|eight|nine)` would have gone permanently green at "ten".
- Finding 4's proximity asserts passed only because both target paragraphs happened to land as
  single unwrapped lines.

None of these would have failed. All three would have stopped noticing.

Two more were caught earlier, by the workers themselves, in the plan's own supplied test code:

- The push-denial assert's 120-character proximity window reached from a pre-existing member's
  `--force-with-lease` to a *different* pre-existing member's `denying` at ~123 characters. It
  passed today by three characters of luck.
- The dispatch-unavailable asserts, scoped to the whole file, were satisfied by the site-marker
  paragraph added in the same task — so deleting the enumeration member they exist to guard left
  them green. Verified empirically, then re-scoped to the enumeration section with its own
  non-vacuity companion.

## Plan defects found during the build

The plan's asserts were sound; its **probe harness** was not, in the same way three times — a
`grep -c` landing-check whose counter literal cannot change across its own mutation, producing a
spurious `MUTATION DID NOT LAND`. Task 1's `probe()` counted `carve-out` for every probe (only probe
A moves it); Task 2's probe I counted `distinct abort reasons` (1 before, 1 after); Task 3's probe N
expected `docket-newthing` at 1 when the literal already appeared once in a comment (correct
evidence is 1→2). Each worker caught its own instance and reported it, and the next brief carried the
warning forward.

That is the failure mode inverted: the earlier lesson was a mutation that never landed reading as a
guard that held. Here the harness cried wolf instead — noisy rather than dangerous, but the same
root cause, and it now has three more instances behind it. The probe template, not the author's
care, is where the counter belongs.

## For the human at the merge gate

Nothing manual to run — the suite covers it. Two things worth your eye in the diff:

1. **The carve-out is a taxonomy decision**, recorded as ADR-0086 and relating to (not superseding)
   ADR-0059. If you disagree that an in-context return channel is what disqualifies these two
   dispatches from Tiers A and C, that is the paragraph to push back on.
2. **Finding 6 / #0291** asks whether `gate-failure.md` being blocking-loaded *after* the dispatch
   verb is good enough. This change assumed yes, on the spec's audited assumption. The stub argues
   the other side.
