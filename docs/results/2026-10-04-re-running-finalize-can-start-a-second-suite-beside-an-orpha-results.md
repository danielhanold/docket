<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0497 — Re-running finalize can start a second suite beside an orphaned one](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-04-0497-re-running-finalize-can-start-a-second-suite-beside-an-orpha.md)**
<!-- docket:backlink:end -->
# Re-running finalize can start a second suite beside an orphaned one — Results

**Human action:** None required to merge. One optional walkthrough below checks the new halt
message by hand; automated tests already cover it.

## Outcome

When a gate's supervisor process dies alone (SIGKILL or a crash), part of the test suite can keep
running. Until now nothing told the human, and a quick re-run of finalize started a second suite in
the same checkout beside the leftover one.

Now, when a gate drive halts because its supervisor died, docket runs the existing read-only
leftover check (`ProbeLeftover`) on that run. If the check clearly finds a leftover, the halt
carries an informational `tree-survives:<drive>:<pgid>` finding:

- The gate drive record stores it (`last_finding`), and the drive document shows it as `finding`
  (HALTED documents only). Human text prints a `finding:` line.
- Finalize puts it in the gate report's `teardown_finding` field. Its halt message now says the
  supervisor died and, when a suite is still running, tells the human to wait until
  `pgrep -lg <pgid>` prints nothing before re-running finalize. Other halt messages are unchanged.
- The build skill text tells the controller to copy the finding into its halt report, so it
  reaches `## Run halted`.

This is visibility only. The halt outcome and cause, and finalize's disposition, reason, result,
and halt cause, are the same for every probe answer. Nothing new refuses, blocks, or signals.
The glossary entry is retitled "Leftover-suite finding `tree-survives`", and ADR-0134 has a dated
Update note on the metadata branch.

## Human actions and testing

### Optional — read the new finalize halt messages

Why: to see the exact wording a human gets after a supervisor death, without killing a real gate.

Prerequisites: a Go toolchain and this branch checked out.

1. Run `timeout --kill-after=10s 10m go test -count=1 -v -run 'TestFinalizeGateHaltMessage' ./internal/app/`
   Expected: PASS. The subtests show the `supervisor-died` message with and without the
   "still running as process group <pgid>" clause.
2. Run `timeout --kill-after=10s 10m go test -count=1 -v -run 'TestDeathHaltFinding' ./internal/gatedrive/`
   Expected: PASS for every probe answer (`leftover`, `none`, `unclear`, error) under both
   `signaled` and `vanished`; only the `leftover` rows carry a finding.

## Verification performed

- Full suite (`go run ./cmd/docket development test`) through the build gate: green.
  The run printed 29 `BUDGET WATCH` screening lines and no `SERIAL CONFIRMED OVER BUDGET` line.
- Mutation checks, each turning its test red and restored green: removing the probe call;
  letting a finding change the cause; trusting the answer when the probe errors; showing the
  finding on non-HALTED documents; dropping the `teardown_finding` fallback; letting a finding
  change finalize's halt cause; removing the human-text `finding:` line; loosening the pgid check.
  The plan's literal version of the "probe error" mutation did not compile; it was re-run in a
  compiling form.
- Whole-branch review (deep tier): no findings.

## Known issues and follow-ups

- **A supervisor death within the first 30-second slice gives no warning.** The launching process
  is still alive then, so the leftover check reads "unclear" and the halt carries no finding.
  Same as before this change. Accepted loss in the spec; no action suggested.
- **`evidence.recertify` never reports a leftover**, for the same reason (it drives every slice in
  one process). Accepted loss; no action suggested.
- **The warning is not a guard.** A human who re-runs finalize without waiting still starts a
  second suite, as before. Intended (no new blocking gates).
