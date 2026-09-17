# Checked receipt consumption

Capture each catalog-resolved operation once, with `--json`, preserving original
stdout, stderr and exit status through terminal transport return. Validate using
`agent.check-receipt` before choosing another operation. A successful checker verifies
receipt shape and expected identity; only the backend establishes current ownership.

Workers pass their pinned `--assignment` and `--sha256`. Coordinators without worker
assignments instead pass `--run-root <established-absolute-root>`. Never invent an
assignment or substitute a new root. Both modes pass `--operation`, `--stdout`,
`--stderr`, and `--exit-code`. From the original start or continuation context, also
pass `--expected-drive-id` whenever known and `--expected-phase` when exposed.
Coordinator facade claims require both expectations. Resolve flags from the candidate
catalog and schema; these checks grant no authority and read no private drive files.

| Producing operation | Credential in original JSON | Meaning and next action |
|---|---|---|
| `gate.drive.start` / `advance` | `drive.generation` | Owner. On WAITING, advance the same `drive.drive_id`, or hand off before departure. |
| `gate.drive.handoff` | `drive.generation` | Single-use handoff token. Direct recovery claims once; an outer-gated departure returns to the parent for keyed verdict and facade continuation. Never redeem through both paths. |
| `gate.drive.claim` / authorized `takeover` | `drive.generation` | Fresh owner. Advance the same drive; do not claim again. |
| `run.gate-claim`, `decision: gate-claimed` | top-level `generation` | Fresh owner. Redemption already happened. Advance top-level `drive_id` under that generation; top-level `phase` identifies recovered work. |

The checked receipt preserves those paths and adds `authority` (`owner` or `handoff`)
and `next_operation`. A successful facade claim's next operation is
`gate.drive.advance`, including terminal claims needing the full receipt and
`raw_run_dir`. This observes existing execution; never start a replacement suite,
claim again, or demand the predecessor handoff token or parent's capability.

A facade `decision: gate-stop` is a refusal despite `result: applied` and exit zero.
The checker classifies it as `halt`, without authority or a next operation.
PASSED, FAILED and HALTED remain distinct; only trustworthy FAILED feeds repair.
Missing/malformed output or identity mismatch uses the existing blocked/halt posture.
Keep original captures protected; diagnostics and tracked notes contain no credentials.

Claim/takeover closes the old scope. Recovered controllers consume terminal evidence
without the old scope's final acknowledgement. Later tests need fresh quiescent scopes.
Uninterrupted workers retain their existing commit, active-input validation, then final
acknowledgement order; no writes follow acknowledgement. Terminal owner generations
remain available for that path. A successful claim is not completed workflow evidence.
