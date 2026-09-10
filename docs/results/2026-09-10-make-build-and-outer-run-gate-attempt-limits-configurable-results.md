<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0421 — Make build and outer run gate attempt limits configurable](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-10-0421-make-build-and-outer-run-gate-attempt-limits-configurable.md)**
<!-- docket:backlink:end -->
# Make build and outer run gate attempt limits configurable — Results

## Outcome

Two independently configurable positive-integer attempt limits were added and wired end to end:

- `build.max_attempts` (built-in default 4): the initial full-suite run plus up to three
  repair-and-rerun cycles. Enforced by a new durable per-`(repo, change, phase)` suite-attempt
  budget store (`internal/gatedrive/suitebudget.go`) reserved atomically inside `StartDrive` for
  build-owned drives, before any launch, so re-observation, recovery, takeover, and continuation
  never double-charge and no repair worker can bypass the cap.
- `run.max_attempts` (built-in default 2): the initial implementation run plus up to one eligible
  retry. Enforced by generalizing the outer gate's binary `retry-consumed` O_EXCL marker into a
  counted per-attempt marker budget snapshotted into the `GateRecord` at mint (schema **v4**;
  legacy v3 records fail closed).

Both limits count total attempts including the initial attempt (a value of 1 disables retries;
only positive integers are valid), resolve through the normal config precedence layers, and are
propagated to their owners through typed context (`PrepareBuild.MaxAttempts`) and the mint
snapshot. Attempt usage and exhaustion diagnostics were surfaced. Durable accounting, attribution,
continuation, and explicit-halt semantics are preserved: waiting and continuation consume no
additional attempt, and an outer retry cannot override a build halt requiring human help. Maintained
workflow instructions, generated gate surfaces, and the repoguard budget pins were updated
consistently, and the config leaves mirror the change-0349 `finalize.resolver_max_attempts`
precedent.

No material departures from the spec.

## Verification performed

- Full suite gate driven inline via the build-owned gate driver
  (`go run ./cmd/docket development test`): **green** at the certified head. Build-evidence record
  minted from the observed run directory and verified against HEAD.
- Deep whole-branch review (rung selected as `docket-review-deep`: no build record from the
  resumed build defaulted the rung to `standard`, and the 2,392-line whole-branch diff bumped it
  one step to `deep`). Review confirmed: config plumbing mirrors the 0349 precedent; store-level
  CAS discipline for both the outer per-attempt marker and the new suite-budget store is sound;
  legacy v3 fails closed; `PrepareBuild.MaxAttempts` asserts a resolved non-default value; the
  `budgets_test.go` re-baseline is documented at exact counts; guard/mutation probes are recorded.
- Finding 2 (below) fixed in-branch and re-certified by the same green suite gate over the combined
  head.

## Findings and limitations

### Repeat diagnostic observation of a quiescent incomplete run advances the retry counter at `run.max_attempts >= 3` (important; deferred to human merge-time judgment)

Because the outer gate derives the current attempt purely from the persisted marker count
(`attempt = 1 + GateRetryUsage`) and the v4 design targets a distinct `retry-consumed-<n>` marker
per count, repeated `gate-verdict <key>` observations of the *same* unchanged quiescent
run-incomplete record — with no new dispatch actually completing — each grant a successive
`gate-retry-once` until the budget is spent. Empirically confirmed at limit 4: three successive
observations returned `gate-retry-once` (usage 1 → 2 → 3) before the terminal `gate-stop`.

Impact is bounded and errs safe: the hard cap always holds — at most `run.max_attempts - 1` markers
exist over a record's life, so the total number of authorized dispatches can never exceed
`run.max_attempts`. The shipping default (2) is immune, because after the single grant the second
observation already reads usage 1 and stops. The concurrent-observation case is correctly handled
(one CAS winner per completed attempt). The gap is the **sequential** repeat at `limit >= 3`, where
a diagnostic re-read (documented as routine) spuriously consumes budget.

A proper fix binds the attempt number to a distinct dispatch epoch rather than the raw marker count,
under the resume-verified `AttributedID`/empty-`BoundRequestID` shape where no per-dispatch binding
exists (continuity there is `RunVerify`'s job). That resolves an open design question the change
deliberately deferred and modifies the ship-once attribution/concurrency model, so it is left for
human merge-time judgment rather than an autonomous review-time change that could destabilize the
tested concurrency and resume guarantees. No safety violation ships in the interim.

### Build suite-attempt budget spans the change lifetime across outer retries (minor; fixed)

The build suite-attempt budget key `{repo, change, "build"}` carries no outer-attempt/epoch
dimension, so it persists across outer-gate `gate-retry-once` re-dispatches of the same change: an
outer retry inherits the remaining build budget and cannot acquire more build repairs. This is
intended and errs safe, but was undocumented. A doc-comment note was added to
`reserveBuildSuiteAttempt` stating the budget is keyed to the change lifetime and is intentionally
not refreshed by an outer retry (commit `a224c44e`).

## Follow-ups

### Consider binding the outer retry counter to a dispatch epoch (from the important finding above)

If the sequential repeat-observation behavior at `run.max_attempts >= 3` is deemed undesirable, a
follow-up change should introduce a per-dispatch epoch signal into the resume-verified verdict path
so that re-observation of an unchanged attempt returns `gate-stop`/`no-attributable-claim` rather
than advancing the counter, and pin the assumption with integration coverage across distinct
per-dispatch claim bindings. Captured here for deliberate human triage; not minted automatically.
