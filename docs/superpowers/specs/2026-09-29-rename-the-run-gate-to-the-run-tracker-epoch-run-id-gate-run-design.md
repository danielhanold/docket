<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0471 — Rename the run gate to the run tracker (epoch → run id, gate-* → run-*)](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0471-rename-the-run-gate-to-the-run-tracker-epoch-run-id-gate-run.md)**
<!-- docket:backlink:end -->

# Run tracker rename (ADR-0129 family (a)) — design

**Change:** 0471 · **Date:** 2026-09-29 · **Status:** groomed · **Decision record:** ADR-0129 (amended in place 2026-09-29 at this grooming)

## Problem

ADR-0129 settled a collision-free docket vocabulary. The run gate is not a checkpoint: it is bookkeeping for a dispatched run, and "epoch" hides that it names a run and its id. This change applies ADR-0129's family (a) rows (1–38 and 38a–38d), which rename the run gate to the **run tracker** across its whole surface: operation ids, CLI verbs and flags, report lines, codes, failure stages, Go identifiers, file names, local storage, skills, agents, generated dispatch material and docs.

Grooming traced the code (2026-09-29) and found what the umbrella table did not cover:

- Four run-tracker names that the table missed, now rows 38a–38d.
- The admission slot record (`internal/gatedrive/admission.go`, `admissionRecord`) has no JSON tags, so its on-disk key is the Go field name `RunEpochID`. A plain Go rename would silently change the key: old slots would decode with no owning run and the run fence would be lost without an error.
- The claim receipts committed on the `docket` branch carry `gate_context_hash`, and the claim idempotency digest hashes that key name. Committed history cannot be rewritten.
- `dispatch_epoch` in the run-tracker record is a real Unix timestamp (it sits beside `created_at`), not a run id.

## Decisions (settled at grooming)

1. **Scope is ADR-0129 rows 1–38 and 38a–38d, hard cut, no aliases.** The ADR is the naming authority. Old spellings are refused once this lands.
2. **Four added names (rows 38a–38d):** `agent enter --run-gate-key` → `--run-key`; the `run.start` result JSON key `epoch` → `run_id`; the `change.claim` request key `gate_context` → `run_context`; the env var `DOCKET_AGENT_GUARDIAN_EPOCH` → `DOCKET_AGENT_GUARDIAN_RUN_ID`.
3. **Local run-tracker storage is reset to new names, not migrated** (ADR-0129 Decision 3 as amended). Each store whose persisted names change moves to a new root, so the new binary never reads old state and starts empty. There is no migration code and no upgrade-detection guard. The only precondition is that nothing is in flight at upgrade, which this change already requires.
4. **Committed state keeps its spelling.** The claim receipts' `gate_context_hash` key and the claim idempotency digest payload are unchanged. Their Go fields are renamed and keep the old key through an explicit JSON tag.
5. **Every file named after a retired term is renamed**: Go source and test files, the six run-tracker integration shards (with their test-function prefixes and budget rows), the concept page and the cursor-rules asset.
6. **"Gate" stays where it means a checkpoint:** `gate-failed` (the suite gate failed), the `gate-scope` run participant kind (a gate-drive scope), the gate drive, the gate run, the `docket gate …` noun, the `gatelifecycle` shard (gate launch/stop) and the two `gatedrive_*` shards.
7. **Landing is a human procedure**, with no new tooling. No operation lists live runs, and run records marked `active` on disk include dead and never-dispatched runs, so a machine check would mislead.
8. **0471 creates the retired-vocabulary table** in `internal/repoguard` (ADR-0129 Decision 10). No other family has landed. If one lands first, 0471 appends to that family's table instead.

## Design

### 1. Wire and prose renames

Apply ADR-0129 rows 7–37 and 38a–38d to every wire surface, and rows 1–6 to prose. Old spellings are refused outright; nothing accepts both.

Go identifiers follow their row's term, derived by a whole-repo grep and never from a hand list. For example: `EpochRecord` → `RunRecord`, `RunEpochID` → `RunID`, `GateContextHash` → `RunContextHash`, `EpochLaunchGate` → `RunLaunchGate`, `EpochRevokedFunc` → `RunRevokedFunc`, `ClassifyRunEpochError` → `ClassifyRunIDError`. Identifiers whose "gate" means the run tracker (for example `GateRecord`, `GateStoreError`, `RunGateBeforeResult`, `RunGateAsset`) are renamed too. Identifiers whose "gate" means a checkpoint keep it.

Sites known at grooming. This list is a starting point for the plan's grep, not the population:

- **CLI and app:** the `run` command group and its flag definitions (`internal/cli`), including `change claim --gate-context` and `agent enter --run-gate-key` / `--run-epoch`; `internal/app` run-tracker files; the `gatedrive` flags `--run-epoch` on `gate drive start` and `gate drive prepare-scope`.
- **Asset-free command table:** the `"run gate-before"`, `"run gate-verdict"` and `"run gate-claim"` entries in `internal/cli/install.go`.
- **Generated dispatch material:** `internal/harness/dispatch.go` (`dispatchPreamble`, `RunGate`, `DispatchInterior`, `CodexDispatchInterior`, `CodexRootEntryClause`), the `cursor-rules/run-gate.md` source and its embedded copy, and its callers (`internal/reposeed/plan.go`, `internal/harness/cursor/cursor.go`, `internal/cli/install.go`, `internal/app/repository_init.go`). The embedded tree and `manifest.json` are regenerated (`go generate`, `cmd/genassets`), never hand-edited.
- **This repo's `AGENTS.md`:** the managed `docket:dispatch` block is regenerated from the generator, never hand-edited.
- **Skills and embedded copies:** `skills/docket-implement-next/SKILL.md` and `references/edge-paths.md`; `skills/docket-build/SKILL.md` and `references/gate-caller-loop.md`; `skills/docket-build-task/SKILL.md`; `skills/docket-finalize-change/references/gate-failure.md` (its `run.cancel … --epoch` remedy becomes `--run-id`).
- **Docs:** `docs/concepts/run-gate.md` (renamed, section 3), `docs/concepts/build-profiles-and-gate.md`, `docs/comparison/ai-native-sdlc-playbook.md`, `docs/reference/glossary.md` (the entries for these rows, per ADR-0129 §"Family changes"), `docs/reference/harness/validation-runbook.md`.
- **Tests:** repoguard wording pins (for example `prose_contracts_test.go`, `root_entry_dispatch_test.go`, `skill_handoff_sites_test.go`, the renamed `gatedrive_run_id_thread_test.go`), byte budgets in `repoguard/budgets_test.go`, and the golden `capabilities` / `schema` output.

Stored wording changes going forward: the run-tracker record's `Disposition` text and a gate drive's `LastCause` are written with the new tokens (`run-done …`, `run-record-unreadable`). Nothing branches on either value.

### 2. Storage reset

The local stores under `<git-common-dir>/docket/` move to new roots:

| Store | Old | New |
|---|---|---|
| Run records | `rungate/<key>/` with `epoch.json`, `epoch.lock` | `run-tracker/<key>/` with `run.json`, `run.lock` |
| Run-record keys | `epoch_id`, `gate_key`, `dispatch_epoch` | `run_id`, `run_key`, `dispatched_at` |
| Resume locks | `rungate-resume/<change-id>/epoch.lock` | `run-tracker-resume/<change-id>/run.lock` |
| Admission slots | `gate-admission/v1`, implicit key `RunEpochID` | `gate-admission/v2`, explicit JSON tags on every field, key `run_id` |
| Gate scopes | `gate-scopes/v1`, keys `run_epoch_id`, `gate_context_hash` | `gate-scopes/v2`, keys `run_id`, `run_context_hash` |
| Gate drives | `gate-drives/v1`, key `gate_context_hash` | `gate-drives/v2`, key `run_context_hash` |

Rules:

- **New roots are the version boundary.** The new binary knows only the new roots, never reads the old ones, and starts with empty state. Record-level `schema` fields keep their current values.
- **No migration code, no detection guard.** The old roots are left inert. Nothing references them, so no admission slot can point at a run the new store cannot find.
- **Explicit tags everywhere in admission v2.** `admissionRecord` gains an explicit `json:"…"` tag on every field, so a future Go rename can never silently change a persisted key.
- **Other persisted structs follow the same rule.** The plan greps every persisted struct in these stores for retired terms. Any store with a retired persisted key not in the table above moves to a new root the same way, and the plan lists it.
- **Committed state keeps its spelling.** `changeClaimReceipt`, `claimDigestPayload` and the claim-proof scanner struct keep the key `gate_context_hash`, with their Go fields renamed to `RunContextHash`. The request key `gate_context` (row 38c) is a different, wire-level key and is renamed to `run_context`.
- **A change halted `in-progress` before the upgrade resumes** through the existing path for a change with no run record (change 0463). The renamed end-to-end test that covers it (today `TestIntegrationGateArmEpochlessResumeEndToEnd0382`) must keep covering that case.
- **Other readers follow the constants.** `gate history cleanup`, `PruneGateRecords`, `docket gate observe` and repository checks resolve the store roots through the renamed constants, and the tests' fixture paths move with them.

### 3. File renames

Use `git mv` so history follows each file.

**`internal/app` (36 files).** The `rungate_` prefix becomes `runtracker_` everywhere, and the rest of the name changes where listed:

| Old | New |
|---|---|
| `rungate_before.go` | `runtracker_start.go` |
| `rungate_before_integration_test.go` | `runtracker_start_integration_test.go` |
| `rungate_before_resume_integration_test.go` | `runtracker_start_resume_integration_test.go` |
| `rungate_cancel.go` | `runtracker_cancel.go` |
| `rungate_cancel_helpers_test.go` | `runtracker_cancel_helpers_test.go` |
| `rungate_cancel_integration_test.go` | `runtracker_cancel_integration_test.go` |
| `rungate_claim.go` | `runtracker_continue.go` |
| `rungate_claim_integration_test.go` | `runtracker_continue_integration_test.go` |
| `rungate_complete.go` | `runtracker_complete.go` |
| `rungate_complete_helpers_test.go` | `runtracker_complete_helpers_test.go` |
| `rungate_complete_integration_test.go` | `runtracker_complete_integration_test.go` |
| `rungate_continuation.go` | `runtracker_continuation.go` |
| `rungate_epoch.go` | `runtracker_run_record.go` |
| `rungate_epoch_helpers_test.go` | `runtracker_run_record_helpers_test.go` |
| `rungate_epoch_integration_test.go` | `runtracker_run_record_integration_test.go` |
| `rungate_epoch_refusal.go` | `runtracker_run_id_refusal.go` |
| `rungate_epoch_refusal_test.go` | `runtracker_run_id_refusal_test.go` |
| `rungate_epochless_resume_e2e_integration_test.go` | `runtracker_no_run_record_resume_e2e_integration_test.go` |
| `rungate_fence.go` | `runtracker_fence.go` |
| `rungate_fence_helpers_test.go` | `runtracker_fence_helpers_test.go` |
| `rungate_fence_integration_test.go` | `runtracker_fence_integration_test.go` |
| `rungate_gate.go` | `runtracker_launch_gate.go` |
| `rungate_gate_integration_test.go` | `runtracker_launch_gate_integration_test.go` |
| `rungate_ownership_integration_test.go` | `runtracker_ownership_integration_test.go` |
| `rungate_production_census_integration_test.go` | `runtracker_production_census_integration_test.go` |
| `rungate_publication.go` | `runtracker_publication.go` |
| `rungate_publication_integration_test.go` | `runtracker_publication_integration_test.go` |
| `rungate_publication_settle_paths_integration_test.go` | `runtracker_publication_settle_paths_integration_test.go` |
| `rungate_publication_settle_paths_test.go` | `runtracker_publication_settle_paths_test.go` |
| `rungate_publication_test.go` | `runtracker_publication_test.go` |
| `rungate_store.go` | `runtracker_store.go` |
| `rungate_store_helpers_test.go` | `runtracker_store_helpers_test.go` |
| `rungate_store_integration_test.go` | `runtracker_store_integration_test.go` |
| `rungate_verdict.go` | `runtracker_verdict.go` |
| `rungate_verdict_helpers_test.go` | `runtracker_verdict_helpers_test.go` |
| `rungate_verdict_integration_test.go` | `runtracker_verdict_integration_test.go` |

`rungate_gate.go` holds the run launch gate (row 37's `run-launch-gate`), hence `launch_gate`.

**`internal/gatedrive` and `internal/repoguard`:**

| Old | New |
|---|---|
| `internal/gatedrive/driver_epochfence_test.go` | `internal/gatedrive/driver_runfence_test.go` |
| `internal/gatedrive/epoch_gate_test.go` | `internal/gatedrive/run_launch_gate_test.go` |
| `internal/gatedrive/epoch_test.go` | `internal/gatedrive/slot_run_fence_test.go` |
| `internal/gatedrive/takeover_epoch_test.go` | `internal/gatedrive/takeover_run_test.go` |
| `internal/repoguard/gatedrive_run_epoch_thread_test.go` | `internal/repoguard/gatedrive_run_id_thread_test.go` |

**Pages:**

| Old | New |
|---|---|
| `docs/concepts/run-gate.md` | `docs/concepts/run-tracker.md` (fix every inbound link) |
| `cursor-rules/run-gate.md` | `cursor-rules/run-tracker.md` (the embedded copy, `manifest.json` and the asset constant follow) |

Consumer repos are unaffected by the cursor-rules rename: Cursor always installs the composed rule as `docket-dispatch.mdc`.

**Integration shards.** Each shard file, its `SHARD_PREFIX`, the Go test-function prefix it selects, its header comment and its `tests/runtime-budgets.tsv` row rename together. Budget values are unchanged. Test counts at grooming are listed; the plan re-counts at build start.

| Old shard | New shard | Old test prefix | New test prefix | Tests |
|---|---|---|---|---|
| `test_go_integration_app_gatearm.sh` | `test_go_integration_app_runstart.sh` | `TestIntegrationGateArm` | `TestIntegrationRunStart` | 39 |
| `test_go_integration_app_gatecancel.sh` | `test_go_integration_app_runcancel.sh` | `TestIntegrationGateCancel` | `TestIntegrationRunCancel` | 44 |
| `test_go_integration_app_gatecompletion.sh` | `test_go_integration_app_runcompletion.sh` | `TestIntegrationGateCompletion` | `TestIntegrationRunCompletion` | 31 |
| `test_go_integration_app_gateepoch.sh` | `test_go_integration_app_runrecord.sh` | `TestIntegrationGateEpoch` | `TestIntegrationRunRecord` | 36 |
| `test_go_integration_app_gatefence.sh` | `test_go_integration_app_runfence.sh` | `TestIntegrationGateFence` | `TestIntegrationRunFence` | 30 |
| `test_go_integration_app_gateverdict.sh` | `test_go_integration_app_runverdict.sh` | `TestIntegrationGateVerdict` | `TestIntegrationRunVerdict` | 32 |

No `TestIntegrationRun*` prefix existed at grooming, so the new prefixes cannot capture unrelated tests.

**Not renamed:**

- `tests/test_go_integration_app_gatelifecycle.sh` and its `TestIntegrationGateLifecycle` tests: gate launch/stop, the gate-run sense.
- `tests/test_go_integration_gatedrive_process.sh` and `tests/test_go_integration_gatedrive_race.sh`.
- `internal/app/repocheck_*.go` and `tests/test_go_integration_app_repocheck.sh`: "epoch" only appears inside "r-epoch-eck".

### 4. Retired-vocabulary seal

Create the retired-vocabulary table in `internal/repoguard` (ADR-0129 Decision 10). It maps each retired token to its replacement:

- every wire token in rows 7–37 and 38a–38d;
- row 38's retired unversioned storage names (`rungate`, `rungate-resume`, `epoch.json`, `epoch.lock`) as path-construction literals, so no future code can read the old roots again. The `v1` → `v2` bumps are pinned by the store tests instead.

The seal follows the shape-class pattern of `internal/repoguard/absence_test.go` (`TestNoRetiredBashControlPlane`) and scans the same surfaces: `ExecutableSurface`, `AlwaysLoadedSurface` (AGENTS.md/CLAUDE.md, `agents/**`, `cursor-rules/**`), and generator output including prose. It matches:

- a retired flag or CLI verb in an argv or a flag definition;
- a retired operation id in a catalog-resolution or dispatch site;
- a retired token in a closed-vocabulary, verdict-parsing or code-constant site;
- a retired storage name in a path-construction site.

It does not match passing mentions in frozen records, ADR-0129 or history. Every failure names the replacement.

Shape boundaries the seal must respect:

- `gate_context` (retired) must not match inside `gate_context_hash` (committed, kept).
- `run cancel --epoch` (retired) must not match `cmd/releasepkg --source-epoch` or `SOURCE_DATE_EPOCH` (real Unix epochs, kept).
- `gate-failed`, `gate-scope`, `gatedrive-*` and `idempotent-suite-gate` stay legal.

Any fixture that has to carry a retired value gets a bounded exclusion, and the exclusion itself is mutation-tested (see the learning `frozen-fixture-corpus-trips-repo-wide-scans`).

### 5. Landing

Landing is a human procedure. The results file's `**Human action:**` and the PR body say:

1. **Merge only when no dispatched implement-next run is in flight** (drain or cancel first).
2. **Run the post-merge binary rebuild immediately** (the AGENTS.md rule).
3. **Restart open coordinator sessions.** Their loaded CLAUDE.md still names `run.gate-before`.
4. **Re-run `docket install` in every consumer repo.** Their generated dispatch material still names the old tokens.
5. **On each machine, switch to the new binary only when no implement-next or finalize run is in flight in any docket repo.** The new binary starts the run tracker's stores empty. Afterwards the old roots may be deleted by hand: `.git/docket/rungate`, `.git/docket/rungate-resume`, `.git/docket/gate-admission/v1`, `.git/docket/gate-scopes/v1`, `.git/docket/gate-drives/v1`.

If step 1 or step 5 is skipped, the failure is loud, not silent:

- A coordinator's `run.gate-verdict` no longer resolves from the catalog, so the coordinator stops.
- New starts and claims with old flags are refused.
- An in-flight child's `gate drive advance` uses no renamed flag, but its drive lives in a `v1` root the new binary does not read, so the new binary reports the drive as not found.

Recovery uses `run verdict`, `run start --resume <id>` and `run cancel --run-id` with the new binary, then re-dispatches.

Building 0471 itself is safe. The build's gate drive uses the installed (old) binary, while the suite runs from source. No shell test calls the installed binary with the old names; at grooming the only hit in `tests/` was a comment. 0471's own finalize also runs on the old binary, and the rebuild happens after finalize returns.

## Testing

- **The full suite** through `build.test_command`. Read the budget report even when green.
- **Seal mutation tests:** for each shape class, reintroduce one retired token at an executable site and watch the seal go red naming the replacement. Include a non-vacuity floor per shape class and a negative control for each shape boundary in section 4.
- **Admission tag guard:** a test fails if any `admissionRecord` field lacks an explicit JSON tag. Mutation: drop one tag, and the test goes red.
- **Committed-key decode test:** a claim receipt written with `gate_context_hash` decodes into `RunContextHash`, and claim proof still finds it. Mutation: change the tag, and the test goes red.
- **Storage reset:**
  - A fixture repo holding only old roots (a `rungate/<key>/epoch.json` marked active, and a `gate-admission/v1` slot owned by that run) lets `run start` admit a new run in that worktree, and leaves the old files byte-for-byte untouched.
  - A change `in-progress` with no run record resumes through the change-0463 path.
- **Golden output:** `capabilities --json` and `schema --json` show only the new names.
- **Shard integrity:** the completeness contract (`tests/test_go_integration_contract.sh`) stays green, and each renamed shard runs exactly the number of tests its old shard ran, so no test silently drops out.
- **Whole-repo grep:** grep for every retired token and phrase in rows 1–38 and 38a–38d. Show each remaining hit is either a kept name (Decision 6, committed state), a point-in-time record, or ADR-0129.

## Out of scope

- Aliases, a deprecation window or dual-spelling support (ADR-0129 Decision 2).
- Renaming config keys, agent names, frontmatter fields, or committed metadata (the claim receipts' `gate_context_hash` and the claim digest).
- Migrating old run-tracker state, an upgrade-detection guard, a live-run listing operation, or deleting old roots automatically.
- Editing point-in-time records: archived changes, results, specs, plans. ADR-0129 was amended in place at grooming with explicit human authorization, and 0471's build does not edit it further.
- Rows owned by changes 0472–0474 or by change 0469.

## Coordination

- **0469** (opaque-name renames) touches the same run-tracker files. Its own grooming decides whether it takes `depends_on: [471]`; 0471 does not wait on it.
- **0472 and 0474** append their rows to the retired-vocabulary table 0471 creates. 0473 has no wire tokens and adds nothing. If a sibling lands first, the reconcile pass switches 0471 from "create" to "append".
