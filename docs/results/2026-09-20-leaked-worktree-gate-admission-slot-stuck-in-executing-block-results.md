<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0439 — Leaked worktree gate-admission slot stuck in "executing" blocks finalize.rebase with a swallowed unavailable error](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-20-0439-leaked-worktree-gate-admission-slot-stuck-in-executing-block.md)**
<!-- docket:backlink:end -->
# Leaked worktree gate-admission slot stuck in "executing" blocks finalize.rebase with a swallowed unavailable error — Results

## Outcome

When a worktree gate-admission slot refuses a new gate, the refusal now carries a bounded, credential-free description of the incumbent through to both JSON and human output, instead of collapsing into a bare `unavailable` halt. The behavior change is entirely diagnostic — no admission, teardown, cancellation, cleanup, or epoch-fence policy was altered.

Delivered in five plan tasks plus two review fixes:

- `internal/gatedrive`: admission refusals (`worktree-busy`, `unresolved-execution`, `stale-run-epoch`) now attach an `IncumbentSnapshot` to the typed `OwnershipError`, projected from the exact record read under the slot's flock (`incumbentSnapshot`). It carries identity and route facts only — `Kind`, `State`, `DriveID`, `RawRunID`, `RawRunDir`, and a boolean `EpochOwned` — never the reservation token, owner generation, or the epoch id itself. The `ErrNotFound` inventory leg and the fail-closed read-error leg stay snapshot-free (an unreadable record has no trustworthy facts to project).
- `internal/app/gate_drive.go`: `mapDriveResult` maps a snapshot-bearing admission refusal to `Stage: worktree-admission`, the existing `incumbent-drive:<id>` / `incumbent-run:<id>` locator convention (each id validated — drive id via `gatedrive.ValidDriveID`, raw run id via a 32-lowercase-hex shape), and a per-kind credential-free remedy. Raw-run stop guidance renders only when a confirmed `RawRunDir` exists; driven and epoch-owned incumbents get continuation / `run.cancel` guidance and never a raw-stop suggestion. The blanket `worktree-busy` wording that equated occupancy with a running process was reworded to slot-occupancy wording.
- `internal/app/gate.go`: the raw `GateLaunch` refusal cause is derived from the refusal's own snapshot (`admissionRefusalCause`), not a post-refusal re-read of the slot — closing a decide-on-one-copy / locate-on-another TOCTOU. The review fix extended the same discipline to `rawStaleEpochRefusal`, which now projects its locator from the slot record it already loaded; the now-dead `incumbentLocator` re-reader was removed.
- `internal/app/finalize_rebase.go`: the local-gate seam carries the four diagnostic fields (`HaltReason/HaltMessage/HaltStage/HaltLocator` on `LocalGateResult`, `Reason/Message/Stage/Locator` on `GateReport`) through both the initial `Start` and continued `Advance` slices into JSON and `HumanText`. The coarse `unavailable` halt cause and the closed halt vocabulary are preserved; a genuinely detail-less halt keeps today's exact generic output; `GateReport.RunDir` is never overloaded with an incumbent path. An admission refusal is never turned into suite failure, gate evidence, a charged attempt, or retry authorization.
- `docs/concepts/run-gate.md`: a new "The worktree admission slot" section explains that a busy slot means occupied (not necessarily still running), that a raw run retains its slot until explicit teardown via `docket gate stop <run-dir> --reason <why>` (stopping a completed run settles its slot; the stop operation decides whether teardown is proven), that history cleanup assesses historical drives only, and that `gate recover` does not release a current raw admission slot.

No departures from the spec's scope: no new CLI command, configuration key, persistent schema change, lifecycle state, automatic recovery policy, retry layer, or ADR.

## Human testing

### Operator sees an actionable diagnostic when finalize meets an occupied worktree slot

This is the incident the change targets. Automated composition tests assert the mapped fields and the human line; a live end-to-end run is the one thing tests do not exercise.

Prerequisites: a registered feature worktree with a configured local gate.

1. In that worktree, launch a raw gate run (`docket gate launch … -- <a quick command>`) and let it reach a terminal (passed or failed) state without stopping it.
2. Attempt a `finalize` whose local gate targets the same worktree.
   Expected: finalize stays blocked, spawns no second process, and its JSON + human output name `worktree-busy`, an `incumbent-run:<id>` locator, and executable `docket gate observe`/`docket gate stop <run-dir> --reason <why>` guidance — not a bare "unavailable".
3. Run the reported `docket gate stop <run-dir> --reason <why>`, then retry finalize's gate.
   Expected: the raw slot is released, the retry admits, and the completed run's original result is preserved.

## Verification performed

- Full suite (`go run ./cmd/docket development test`) driven through the native build gate: green, 52/52 files, 0 failed, at the final head `9c318b5d55a29e9c3bed385b14ea48181fbc014e`. The first build run went red on a single gofmt normalization of a doc comment in `gate_drive.go` (no logic); repaired and re-certified green.
- Per-task focused verification ran through the gate driver with RED→GREEN and, for Tasks 1 and 4, an explicit mutation check that blanking the carried field reddens the relevant assertion.
- Whole-branch deep review: build evidence verified green at head; every safety invariant the change committed to (credential-free projection, snapshot-not-re-read, validated locators, no raw-stop for driven/epoch slots, `RunDir` not overloaded, refusal never becomes red/evidence/charge/retry, no dropped detail across the mapping seams) was confirmed. Three minor findings returned; two fixed in-branch (commit `9c318b5d`), one reported as follow-up.

## Findings and limitations

### Stale-run-epoch fence is dormant today

The `stale-run-epoch` refusal leg and `rawStaleEpochRefusal` carry the new snapshot/locator provenance, but the fence itself is currently inert (`RunEpochID` is `""` until the epoch-ownership work lands), so that leg's enriched diagnostic is exercised only by unit tests, not by a live epoch-owned slot. The projection and locator validation are correct by construction and covered by tests.

## Follow-ups

### Build recertify advisory-precheck path does not surface the rich incumbent diagnostic

`startBudgetedBuild`'s advisory precheck short-circuits a cleanly-read busy slot via `WorktreeAdmissionRefusal`, which returns a snapshot-free `OwnershipError`, so on that path `mapDriveResult` falls back to the generic `ownershipNextAction` message rather than the new incumbent locator/stage/per-kind remedy. This is not a regression (that path never had the rich diagnostic) and is outside this change's stated target (`finalize.rebase`, which uses `engine.Start` directly and does carry the snapshot). A follow-up could give the build recertify precheck the same enriched diagnostic. No change minted; capture deliberately with `docket change create` if desired.
