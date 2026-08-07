<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0234 — Split gate-execution.md: probe evidence should not sit on a blocking-read surface](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0234-split-gate-execution-md-probe-evidence-should-not-sit-on-a-b.md)**
<!-- docket:backlink:end -->

# Split `gate-execution.md`: probe evidence off the blocking-read surface — results

Change: #0234 · Branch: `feat/split-gate-execution-md-probe-evidence-should-not-sit-on-a-b` · PR: see change file · Plan: `docs/superpowers/plans/2026-08-07-split-gate-execution-md-probe-evidence-should-not-sit-on-a-b-plan.md` · ADRs: none

## Verify (human)

- [ ] **Re-probe trigger still reads correctly.** The four harness version strings deliberately stayed
      on the blocking-read surface (spec A3) while the durations and probe design moved. Confirm that
      reading only `skills/docket-build/references/gate-execution.md` still tells you *when* to
      re-probe — that is the one property the split could plausibly have broken, and no test can
      assert it.

## Findings

The whole-branch review (`docket-review-standard`) returned five findings — 0 blocker, 1 important,
4 minor. All five were fixed in-branch; the PR body carries the disposition table. Two are worth
recording beyond that table:

- **The split was guarded in one direction only** (important, `45cfb7d4`). The three planned guards
  all asserted *removal* from the kept surface — that `## Method` is gone, that the pointer exists,
  that the sibling is non-vacuous. None asserted the evidence actually *landed*: deleting `## Method`
  and the ladder from the sibling left it above the `>= 40` floor with every assert green and the
  evidence gone from the repo. This is `correspondence-guard-runs-one-way` applied to a *file move*
  rather than to two sets, which is a shape the finding's existing war stories do not cover.

- **The reviewer's suggested pattern would not have fired** (minor, `bd507aa0`). Finding 5 proposed
  `returned in \*\*[0-9]+s\*\*` to catch duration figures regrowing on the kept surface. Validated
  against the real text it matches only three of the four narratives — the `claude` one line-wraps
  between "returned in" and `**19s**`, and `grep` is line-oriented, so a paste of exactly that
  sentence would have slipped past the guard written to catch it. The adopted pattern keys on the
  figure shape alone (`\*\*[0-9]+s\*\*`). A suggested fix in a review finding is unverified code, the
  same way `plan-supplied-test-code-is-unverified` says plan-supplied test code is.

Two smaller notes: a `perl -0pi -e 's/^…$/…/m'` mutation silently made no substitution and produced a
false "the guard did not redden" reading (two workers hit this independently — confirm the mutation
landed before believing the guard's response); and the plan's own verification greps were written
`^(NOT )?ok` while the runner prints `NOT OK` uppercase, so a RED line was invisible to the filter
rather than reported.

## Follow-ups

- **The evidence row's word budget was re-set twice in one branch** (971 → 1000, then → 1050 after an
  in-branch review fix added a sentence took the actual to 999/1000). No action needed, but it is a
  worked example of the rounding rule's within-25 clause firing on a file that changed *after* its
  row was set — the rule assumes the actual is final at the moment the row is written, and in a run
  with a fix loop it is not.

Nothing was auto-captured: every finding was about this branch's own diff, which the fix loop's
narrower auto-capture rule makes non-mintable by construction.
