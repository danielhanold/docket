<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0368 — Recover a run halted before its workspace was allocated](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-19-0368-resume-halted-preallocation-recovery.md)**
<!-- docket:backlink:end -->
# Recover a run halted before its workspace was allocated — Results

## Outcome

A run that halted during reconciliation — after claim but before any `workspace.prepare` —
left an in-progress change with a fresh claim and a durable `## Run halted` marker but no
workspace manifest. Workspace inspection collapsed that proven absence into the `foreign`
state, so `change resume-halted` refused it with `workspace-writer-active`, and reclaim was
inapplicable (it requires a strictly expired lease and a missing branch).

This change adds a distinct `workspace.StateAbsent` classification, split out of `StateForeign`.
`classifyAbsentSlot` proves absence fail-closed: the local feature ref is absent AND the target
path is absent (via `Lstat`, so a dangling symlink counts as present) AND there is no blocking or
unresolved worktree registration; any probe error returns a typed `Failure` rather than reading as
clean absence. `resume-halted` was rebuilt around a closed admission set (`classifyResumeAdmission`)
that routes `StateAbsent` through a remote-feature-branch absence proof and refuses every
unrecognized state on a default arm. On success it preserves the existing claim and recorded
branch, removes only the `## Run halted` marker, and leaves workspace allocation to the ordinary
later `prepare` step. `StateAbsent` is carried with pre-change behavior through every workspace-state
consumer (reclaim, maintenance-assess, repair; evidence-recertify and finalize-rebase correctly
fall to their existing closed-default refusal arms). Fresh-allocation inventory checks were
tightened to fail closed on a stale registration at the target path, a registration on the feature
ref elsewhere, and an unresolvable registration.

## Verification performed

- Full suite green through the configured build gate (`go run ./cmd/docket development test`):
  49 files, 411 asserts, 0 failed, wall 332s. Build evidence recorded and verified at the
  branch head.
- The integration completeness contract (`tests/test_go_integration_contract.sh`) initially
  reddened: the new end-to-end regression test was named `TestIntegrationResumeHaltedPreallocation`,
  which matched no shard runner. After the finalize rebase onto base change 0434 — which split
  the `internal/app` change shard into the disjoint `TestIntegrationChangeAuthoring` and
  `TestIntegrationChangeRuntime` prefixes — this branch's three new tests were renamed onto
  `TestIntegrationChangeRuntime…` to register with `tests/test_go_integration_app_changeruntime.sh`,
  the shard that now owns the repair and halt/resume group. Contract green afterward.
- End-to-end regression `TestIntegrationChangeRuntimeResumeHaltedPreallocation` drives the real
  claim → halt → teardown → resume → prepare cycle through the REAL workspace service (no fake
  inspection) and asserts nothing is allocated by the resume itself (no manifest, no path, no
  republished remote ref) while the record's claim, status, and branch are preserved.
- Review (docket-review-standard): 0 blockers. One important finding — the spec-promised
  failed-probe refusal coverage was missing — was fixed in-branch (see Follow-ups/PR body).

## Findings and limitations

### Budget-watch screening (not a breach)

The build gate's budget report emitted `BUDGET WATCH` lines (parallel-overrun streak 1/5) for
several integration shards and `test_go_race`/`test_go_toolchain` under `-j11`. These are
screening findings on parallel wall-clock, not a `SERIAL CONFIRMED OVER BUDGET` breach. The new
integration test adds real-git work to `internal/app`; noted here rather than trimming assertions.

## Follow-ups

### Run-gate worktree admission not rebound to a resume's replacement epoch

While resuming this change, `gate drive start` refused the reserved-replacement run epoch
(`stale-run-epoch`, "an in-flight run owns this worktree") because the worktree's gate-admission
record still named the superseded predecessor epoch (released) and was never rebound to the
reserved replacement. The drive only started when the predecessor epoch was presented. This looks
related to the "verdict-path gate recovery never binds the run epoch's worktree" class. Out of
scope for change 0368; suggest capturing as a separate change against the run-gate admission
rebind path.
