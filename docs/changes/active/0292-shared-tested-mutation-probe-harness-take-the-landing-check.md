---
id: 292
slug: shared-tested-mutation-probe-harness-take-the-landing-check
title: 'Shared, tested mutation-probe harness — take the landing check out of each plan author''s care'
status: proposed
priority: high
type: feat
created: 2026-08-11
updated: 2026-08-11
depends_on: []
related: []
discovered_from: [260]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

**Trigger** — the #0260 close-out harvest. The plan's mutation-probe harness shipped defective three
separate times in one change, each the same root cause: a `grep -c` landing-check whose counter
literal cannot change across its own mutation, so a mutation that landed perfectly reported
`MUTATION DID NOT LAND`. Three workers each caught their own instance independently. Counting the
drain, that is the **fifth** recurrence of the plan-supplied-probe class across **four** changes —
#0286 (two defects in supplied test code), #0281 (Task 1 Probe 1 was not a valid probe), and #0260
(three probe-harness instances).

**Opportunity** — docket has no shared, tested mutation-probe harness. Every plan author hand-writes
the probe scaffold — mutate, verify the mutation landed, run the target asserts, restore from a
backup copy, report — and hand-writes the landing check that proves the mutation actually took. That
scaffold is fixed and mechanical; only the mutation and the target asserts vary. A committed helper
(e.g. `scripts/mutation-probe.sh` plus its `scripts/mutation-probe.md` contract, with its own guard
in `tests/`) would take the mutation and the target as parameters, own the landing check and the
restore, and be cited by name from plans instead of transcribed into them.

**Independent value** — it stands with #0260 fully reverted. The defect class is not about #0260's
subject matter; it is about who authors probe scaffolding. The plan author is structurally the
person least able to verify it, because no implementation exists yet to run it against. A tested
template moves the check from per-plan care to a place that is executed on every suite run. It also
retires several recurring hazards already in the learnings ledger — restore-from-a-backup-copy (not
`git checkout --`), `bash -n` the mutated artifact on deletion-shaped mutations, and confirming the
mutation landed by diffing rather than by exit code.

**Boundary** — build the harness, its script contract, and a guard proving the harness itself
reddens when its landing check is defeated; then point the plan-authoring guidance at it. It stops
there. It does **not** retrofit existing merged plans (those are frozen build records), does not
change what any current test asserts, and does not attempt to auto-generate mutations or infer what
a probe should target — the plan still names the mutation and the target asserts.

**Reason for deferral** — #0260 is a documentation-and-guards change to finalize's dispatch tiering
and gate-failure prose; it changed no script. Adding a new executable helper, its contract, and its
guard is a different kind of work in different files, and folding it in would have expanded a branch
whose whole scope was prose and sentinels. It also needs its own design pass: the harness's
parameter surface is the entire question, and five recurrences give real evidence about what that
surface has to cover.

## Open questions

- **Backlog review 2026-09-02 (Bash→Go migration)** — still valid for Docket Go; needs regrooming against the Go tree. Re-target: the proposed home (`scripts/mutation-probe.sh` + `.md`, `grep -c` landing checks) is deleted and no Go successor exists. Options: a `docket development` subcommand that mutates / `go test`s / restores, or a prose template in the plan-writer / build-task guidance. The recurrence evidence is all Bash-era — re-check whether the class still fires on Go-native plans before building.

