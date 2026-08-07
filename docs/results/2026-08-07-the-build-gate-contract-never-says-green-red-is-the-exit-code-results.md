<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0224 — The build gate contract never says green/red is the exit code, so an output-shape match passes as a gate](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-07-0224-the-build-gate-contract-never-says-green-red-is-the-exit-code.md)**
<!-- docket:backlink:end -->

# The build gate's verdict is the exit status — results

Change: #0224 · Branch: feat/the-build-gate-contract-never-says-green-red-is-the-exit-code · PR: (opened at Step 7) · Plan: docs/superpowers/plans/2026-08-07-the-build-gate-contract-never-says-green-red-is-the-exit-plan.md · ADRs: 0074

## Verify (human)

- [ ] Read the new § *The build gate* verdict paragraph in `skills/docket-build/SKILL.md` and confirm it says what you want the gate's tri-state to be. This change hands the *non-failure* judgment to the resolved runner's own documented contract rather than to docket — an obligation docket cannot enforce mechanically, and the one part of this diff no test can check for you.
- [ ] Confirm you accept that `scripts/run-tests.sh` exit 3 and exit 4 now read as a **halt** rather than red. The spec (assumption 4) had accepted the opposite as a residual; review reversed it. ADR-0074 records the reversal, but the call is yours.

## Findings

- **ADR-0074 — the build gate's verdict is tri-state, not binary.** Review established that the spec's accepted residual was not benign: with `finalize.test_command: scripts/run-tests.sh` and `configured-bash-finalize` propagating the exit code verbatim, a run with **zero failing tests** (exit 3 = "produced no result at all", exit 4 = "green but slow") would have fallen through to `**Red** → turn the failure into exactly one synthetic integration-repair task`. That is the same manufacture-a-repair-task harm the adjacent configuration-gap carve-out already refuses. The contract now states green / halt / red without naming a single exit code, preserving the no-taxonomy posture that is the whole point of the change.

- **The plan's own mutation probe was defective, and would have produced a false clean run.** Its `perl` deletion used a literal-space `quotemeta`, which cannot match a phrase that wraps mid-line — and the one guarded phrase that wraps is `(a)`, the central `green if and only if the resolved suite command exits zero`. It reported `before=1 after=1`. The `before/after` count check caught it as `MUTATION DID NOT LAND` rather than as a passing guard, which is exactly what that check exists for. Replaced with a whitespace-insensitive form. This is the `phrase-grep-over-wrapped-prose` learning striking the **verification** step rather than the assert — the second face that finding's own war story records.

- **A guard can pin a phrase's presence while losing the claim's binding.** Three of the five review findings were this one shape: `completed successfully` asserted as a bare phrase rather than bound to *the recorded status*; the two non-verdict names asserted as adjacent to each other rather than each bound to its classification. All would survive a rewrite that kept the words and dropped the meaning. Fixed by binding each claim with a single bounded gap.

- **The slice terminator was a shape, not a name** — `/^#+ /` — in a block whose own comment cited `section-slice-needs-a-named-terminator`, the learning that says to name it and assert it exists. Now `/^### Gate execution posture$/` with an existence assert in this suite rather than only in the sibling. Verified empirically to yield a byte-identical slice (47 non-blank lines both ways), so the change is a hardening with no behavioral drift.

## Follow-ups

None minted. Two candidates were considered and deliberately not filed:

- **`docket-finalize-change`'s prose stays silent on the verdict rule** (spec assumption 2). Not captured: the spec's own rationale rejects the restatement — finalize's `configured-bash-finalize` block already *is* the exit-status test, and restating one rule in two skills grows a second set of guards over the copy (`restatement-accumulates-its-own-guards`). Filing it would be backlog churn against a decision already argued.
- **No runner-side obligation was written.** ADR-0074's cost section names it plainly: the gate's correctness now depends on a resolved runner documenting which of its non-zero exits are non-failures, and docket does not enforce that. Deliberately left as a stated cost rather than a change — writing a rule about what runners should exit is explicitly out of this change's scope, and `scripts/run-tests.md` already defers exit-code semantics here.
