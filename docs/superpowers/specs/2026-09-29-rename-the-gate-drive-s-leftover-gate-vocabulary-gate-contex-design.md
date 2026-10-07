<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0477 — Finish the run-tracker rename (--gate-context, DOCKET_AGENT_GUARDIAN_GATE_KEY, dispatch_context)](../../changes/archive/2026-09-30-0477-rename-the-gate-drive-s-leftover-gate-vocabulary-gate-contex.md)**
<!-- docket:backlink:end -->

# Finish the run-tracker rename: design

Change 0477. Decision record: ADR-0129, rows 38e-38h and its Update of 2026-09-29 (change 0477 grooming).

## Goal

Change 0471 renamed the run gate to the run tracker as a hard cut, but kept three run-tracker spellings because ADR-0129 had no row for them. This change renames them as one hard cut with no aliases, so the retired "gate" and "dispatch context" words are gone from the run-tracker surface:

| ADR-0129 row | Kind | Old | New |
|---|---|---|---|
| 38e | flag | gate drive `--gate-context` (`gate drive start`, `gate drive prepare-scope`) | `--run-context` |
| 38f | env | `DOCKET_AGENT_GUARDIAN_GATE_KEY` | `DOCKET_AGENT_GUARDIAN_RUN_KEY` |
| 38g | key | `run.start` result JSON `dispatch_context` | `run_context` |
| 38h | stage | error-text prefix `rungate store` | `run-tracker store` (already renamed by 0471; recorded only) |

Success means: the same run-context token goes under one flag name (`--run-context`) to both `change claim` and the gate drive; the retired-vocabulary seal refuses every retired spelling at executable sites without a kept-namesake exception; and the whole suite is green.

## Background (traced on main at 32fd8adea, 2026-09-29)

- **The gate drive's flag already means "run context" everywhere except its name.** In `internal/cli/gate.go` both `gate drive start` and `gate drive prepare-scope` read the flag into a variable named `runContext`, and the gate-drive records store its hash as `RunContextHash` (JSON `run_context_hash` in the v2 gate-drive and gate-scope stores). So no persisted state carries the old name, and no storage reset is needed.
- **The guardian env var is private to one binary.** `SpawnAgentGuardian` (`internal/app/agent_guardian.go`) sets it on the guardian process it launches, and that process (the same binary) reads it at startup. The Go constant is already named `guardianRunKeyEnv`. Nothing outside that file names the variable, apart from 0471's plan and results and the seal.
- **`dispatch_context` is the last wire key named after the retired "dispatch context" concept (row 6).** It is the JSON tag on the `RunContext` field of the `run.start` result (`internal/app/runtracker_start.go`). The `run.start` schema descriptor is derived from that tag. The text output line already reads `run-started <key> <run-id> <run-context>`. No skill reads the JSON key; only `runtracker_start_resume_integration_test.go` asserts it.
- **The seal carries about 100 lines of special-case code for the kept flag.** In `internal/repoguard/retired_vocabulary_test.go`, row 12 retires `change claim --gate-context` but has to keep the gate drive's `--gate-context`. It does that through the `Kept` field and `scanKeptRows`, with the row-12-only binding logic `gateDriveBound`, `gateDriveFlagFile`, `gateDriveCommand`, the regexes `gateDriveHeadRe` / `gateDriveQualifierRe` / `gateDriveOpSpanRe`, and the helpers they use (`argvShaped`, `afterShellSeparator`, `enclosingCodeSpan`, `tokenSpans`). Row 12 is the only row that sets `Kept`.
- **`rungate store` is already sealed.** Row 38's `rungate` token row catches it. Only ADR-0129's table text lacked it, which rows 38h and the Update now fix.

## Design

### 1. Renames (hard cut, no aliases)

- **Row 38e.** Rename the flag registered by `gate drive start` and `gate drive prepare-scope` from `gate-context` to `run-context`, and every read of it. Rewrite both help strings so they no longer say "outer gate": the flag takes the `<run-context>` token printed by `run.start`, and links the drive (or the scope's nested drives) to the dispatched run. Update every refusal or hint message that names the flag, including the one in `runtracker_run_id_refusal.go` ("the <run-context> goes to the gate drive's --gate-context"). The old flag is simply unknown to the command, the same hard-cut behavior 0471 used; no custom refusal is added.
- **Row 38f.** Change the value of `guardianRunKeyEnv` to `DOCKET_AGENT_GUARDIAN_RUN_KEY`.
- **Row 38g.** Change the JSON tag on the `run.start` result's run-context field from `dispatch_context` to `run_context`. The schema descriptor follows from the tag.
- **Prose in maintained source.** Where Go comments or help text say "outer gate", "gate context" or "gate-context" and mean the run tracker or its run-context token, rewrite them in run-tracker words ("the run tracker", "the dispatched run", "run context"). Find the sites with a whole-repo grep, never a hand list. Leave the checkpoint sense of "gate" alone (ADR-0129 Decision 5): if a site means an outer suite gate, it stays. Starting points from grooming: `internal/app/gate_drive.go`, `internal/app/runtracker_start.go`, `internal/cli/gate.go`, and `internal/gatedrive/` (`drive.go`, `driver.go`, `run_waiting.go`, `scope.go`, `takeover.go`), plus test comments and failure messages in `internal/gatedrive/takeover_test.go` and `scope_test.go`.
- **Go identifiers** already use `RunContext`, so none need renaming unless the grep finds one.

### 2. Consumers changed in the same PR

- **Skills and their embedded copies** (`internal/assets/embedded/tree/...`): `skills/docket-build/SKILL.md`, `skills/docket-build/references/gate-caller-loop.md`, `skills/docket-build-task/SKILL.md`, and `skills/docket-implement-next/SKILL.md`. Every gate-drive `--gate-context` becomes `--run-context`. Where the prose distinguishes "gate-drive `--gate-context`" from the claim's `--run-context`, it collapses to one flag.
- **Generated dispatch material.** Change the `agent.enter` sentence in `internal/harness/dispatch.go` and in this repo's AGENTS.md managed block from "labeled for `change.claim --run-context` and gate-drive `--gate-context`" to one label: `--run-context` on `change.claim` and the gate drive. Change both in the same commit, so the managed block and its generator stay byte-identical.
- **Golden output.** Update the `gate.drive.start` and `gate.drive.prepare-scope` signatures in `internal/cli/capability_production_test.go` to `[--run-context <token>]`.

### 3. Retired-vocabulary seal

- **Row 12 becomes a plain row covering 38e too.** Drop its `Kept` hook from both entries. The `kindToken` entry retires `--gate-context` everywhere, and the `kindGoFlag` entry retires the flag-name literal `gate-context` everywhere. Label both entries `Row: "12, 38e"`, with `New: "--run-context"`.
- **Delete the row-12 special case:** `gateDriveBound`, `gateDriveFlagFile`, `gateDriveCommand`, the three `gateDrive*Re` regexes, their helpers, and their negative-control tests. Delete the generic `Kept` field and the `scanKeptRows` path too if no row uses them afterwards, because a mechanism with no users cannot be mutation-tested (same reasoning as ADR-0129 Decision 10). A later family that needs a kept-namesake exception can bring one back.
- **New rows.** Row 38f is a `kindToken` row for `DOCKET_AGENT_GUARDIAN_GATE_KEY`, with `New: "DOCKET_AGENT_GUARDIAN_RUN_KEY"`. Row 38g is a `kindJSONKey` row for `dispatch_context`, with `New: "run_context"`. Row 38h needs no new row, because row 38's `rungate` token already covers it.
- **Committed state stays.** The claim-receipt key `gate_context_hash` must stay legal. Row 38c's token boundary already excludes it; a negative control in the seal tests must prove that (add one if none exists).
- **Mutation-test every new or changed row.** Put the old spelling back at one executable site of the right kind: a skill argv line for `--gate-context`, the flag literal in `internal/cli/gate.go`, the env-var constant, and the struct tag. Each time, watch the seal fail and name the replacement. Then restore.
- **Update the header comments of `retired_vocabulary_test.go`**, which currently explain row 12's kept namesake.

### 4. Tests that pin the kept name are flipped, not deleted

- `internal/cli/run_tracker_rename_test.go`: replace the "gate drive must keep its --gate-context flag" assertion with one that checks `gate drive start` and `gate drive prepare-scope` register `run-context` and do not register `gate-context`. Extend the existing (command, new flag, old flag) table the same way.
- `internal/repoguard/gatedrive_scope_identity_test.go` and `gatedrive_run_id_thread_test.go`: switch the pinned skill and dispatch wording to `--run-context`, keeping each pin's mutation-tested shape.
- The message assertions in `internal/cli/gate_test.go`, `internal/app/gate_drive_test.go` and `internal/app/runtracker_run_id_refusal_test.go` expect `--run-context`.
- `internal/app/runtracker_start_resume_integration_test.go` expects `"run_context":`.
- The existing guardian spawn tests cover the env-var rename. Row 38f's seal pins the old spelling's absence.
- Run the whole suite at the build gate (`build.test_command`), not just these tests, and read the budget report.

### 5. Landing (human procedure, recorded in the results `**Human action:**`)

1. Merge with no dispatched run in flight. A run whose loaded skill text passes `--gate-context` fails against the new binary.
2. Run the post-merge binary rebuild immediately.
3. Restart coordinator sessions so they pick up the regenerated AGENTS.md dispatch block.
4. Re-run `docket install` in consumer repos.

Nothing persisted changes name, so unlike 0471 there is no storage reset and no old storage root to delete.

## Out of scope

- Aliases, a deprecation window or dual-spelling support (ADR-0129 Decision 2).
- The committed claim-receipt key `gate_context_hash` and the claim idempotency digest (Decision 3).
- The gate drive's own name, the `docket gate …` CLI noun, and every checkpoint sense of "gate" (Decision 5).
- Point-in-time records: archived changes, specs, plans, results, and ADR bodies other than ADR-0129's table and Update.
- Rows owned by changes 0472-0474, and the unrelated `gofmt` failure tracked by change 0478.

## Risks

- **Cutover.** This is the same hard-cut risk as 0471, limited to the flag: an in-flight run using old skill text breaks until it is restarted on the new install. The landing procedure covers it.
- **Rebase collisions** with 0472-0474, which append rows to the same seal table and ADR-0129. Whichever lands second rebases a small, append-only table.
