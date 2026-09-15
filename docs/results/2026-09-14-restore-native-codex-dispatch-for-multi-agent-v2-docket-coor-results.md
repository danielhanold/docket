<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0425 — Restore native Codex dispatch for Multi-Agent V2 Docket coordinators](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0425-restore-native-codex-dispatch-for-multi-agent-v2-docket-coor.md)**
<!-- docket:backlink:end -->
# Restore native Codex dispatch for Multi-Agent V2 Docket coordinators — Results

## Outcome

Codex's generated Docket role definitions now use the harness's top-level native named-agent dispatch for all 17 registered roles. The implementation adds explicit role-aware feature binding, immutable assignment and private payload validation, resource-closure checks, first-response gate receipt validation, feature-revision artifact diagnostics, and a disposable native-acceptance fixture generator. Other harness routes remain unchanged, and ADR-0119 supersedes ADR-0114's automatic Codex `agent.enter` decision while retaining its ownership invariant.

The full source verification found one in-scope integration defect: the repository's exhaustive process-exit guard had not registered the two new developer command entrypoints. Commit `e861d4968c04621ed82b18aa096bee36a28c5b1f` adds only those exact `main` packages to the guard's allowlist and documents their process-exit contracts.

## Independent-review repair

Commit `0424126409ebd68b4455bbbce8dbff0d49a20705` repairs all seven findings from the first phase-4 review while preserving its report and reproduction overlay:

- Active plan/results reads now require the registered owning workspace for active in-progress or implemented changes, pin its immutable HEAD, reject ownership/ref drift, honor stacked effective bases, and return terminal reads to the integration revision.
- Active child checks prove the current HEAD descends from entry and validate both committed and uncommitted paths against assigned ownership. Build, resolver, and repair entry require a separately hashed private payload.
- The `agent.check-inputs` schema now publishes versioned assignment and worker-payload document shapes, including nested root identity/fingerprint fields and closed role, phase, mode, and payload-kind vocabularies. Installed Codex references define the controller construction order and child entry envelope.
- Review assignments require equal full entry/review HEADs plus a declared hashed evidence resource; entry remains clean and branch-pinned. Resolver entry uses the owned conflict reservation before ordinary workspace checks, while repair entry requires the exact owned finalize attempt with no live rebase.
- The native fixture generator commits discovered build/finalize gate configuration, obtains metadata versions from typed `status`, runs `go test ./...`, verifies build readiness and a clean primary, and records source, binary, pin, command, HEAD, and file-hash evidence in its manifest.

Permanent regression coverage includes the original review overlay cases, real Git workspace ownership/ref/stack/terminal cases, descendant committed-path checks, review ref/evidence/dirty checks, full wired resolver and repair entry against real finalize receipts, constructible schema output, and a successful end-to-end native fixture preparation. The review overlay and focused package/tagged integration checks passed before this results update. The complete configured source suite is run from the clean commit containing this results record; its final-head receipt remains in the external launch-kit verification evidence to avoid a self-referential commit hash.

The first complete post-review run found two suite-registration defects in the new tests: a default-tag real-Git file used the reserved `_integration_test.go` suffix, and the tagged finalize entry case did not match a declared shard prefix. Renaming the file and placing the tagged case under the existing finalize-rebase shard corrected the integration contract; `gofmt` also normalized the expanded reference guard before the final rerun.

The corrected complete configured run passed at `28f0ee5cb40f6c5c69d46c215d5b486df6c44b0a`: 46 of 46 suite files, 399 assertions, zero failures, and a 271-second wall time. Its budget report classified `test_go_finalize_e2e.sh` as parallel-sensitive after a 23-second solo measurement; seven other parallel-overrun screenings were deferred by the one-confirmation-per-run limit. No `SERIAL CONFIRMED OVER BUDGET` breach was reported. This results-only commit is the final attachment checkpoint, so the same complete configured command is rerun here and recorded in the external final-head receipt.

## Second independent-review repair

Commit `c93624c73394b3d40bce622240bb8d18147dcfd8` resolves the two findings from the next phase-4 review:

- Resolver entry now compares the assignment's primary/common repository identity, canonical feature path, change path, feature ref, pinned metadata revision, and root filesystem identity with the conflict workspace selected by the owned finalize attempt. It then rechecks the metadata pin before accepting the owned reservation. A valid detached conflict workspace still passes, while another registered detached worktree at the same commit is refused.
- The native fixture manifest now merges the renderer's explicit output hashes with Git's tracked-file hashes. It re-reads every explicit output immediately before manifest publication and rejects divergent collisions. All inventory-derived `.codex/agents/docket-*.toml` definitions remain represented even under the repository's normal ignored-agent layout.

Permanent regressions retain the reviewer's full real-Git cross-worktree reproduction alongside the valid-workspace and wrong-reservation controls. Fixture coverage derives all expected role paths from the embedded agent inventory, compares each manifest hash with the rendered file, and mutation-tests post-render drift and conflicting hashes. The supplied second-review overlay, focused default-tag packages, and the full wired resolver/repair integration test passed before this results update. Generated assets remained byte-identical at 70 entries with bundle SHA-256 `45d4ab5115646a92b68f37f39b5b8e877d82b75830aa93659c624e392cd6984a`. The complete configured source suite is run from the clean commit containing this updated results record and retained in the external launch-kit verification evidence.

## Verification performed

### Initial native acceptance handoff repair

The first prepared candidate run stopped before planner execution. Its saved planner payload used bare `docket`, embedded `--repo-dir`, and omitted `--json`; the strict pinned-command validator correctly refused it. Successful `agent.check-inputs` now returns the canonical assignment-only `entry_argv`, built by the same function that validates payloads. Controllers copy this result after freezing the final assignment rather than guessing from optional catalog flags. A regression failed on the missing result before the repair and passed afterward; negative cases retain refusal of bare executables, embedded repository flags and omitted JSON output.

The local-only fixture's later `run.verify` reported `repository-unresolved` during GitHub discovery because the controller had not recorded a durable halt. The generated controller resource now requires catalog/schema-resolved `change.halt` and verification before returning on checker or child failure; the gate's fail-closed verdict is unchanged. A bounded read-only instruction application check produced the pinned command and the correct halt/epoch handling. This is not a native acceptance rerun. The failed fixture, private payload and active epoch remain preserved; no resume or redispatch was performed. A rebuilt candidate requires fresh source review and native acceptance before 424 dogfood.

- The configured source command `go run ./cmd/docket development test` passed at `e861d4968c04621ed82b18aa096bee36a28c5b1f`: 46 of 46 suite files passed, with 399 assertions and no failed result markers.
- The repaired `TestProcessExitSitesAreAllowlisted` passed both normally and under race instrumentation; `tests/test_go_toolchain.sh` passed all six result markers.
- Generated source consistency checks passed: `go run ./cmd/genassets -repo "$PWD" -check` and `go run ./cmd/gendispatch -repo "$PWD" -check`.
- The implementation pass also recorded passing focused Go packages, tagged workflow integration coverage, Linux `codexcontract` compilation, source capability/schema inspection, and literal Bash/zsh first-response capture cases.

## Findings and limitations

### Parallel budget screenings

The final source suite reported seven `BUDGET WATCH` findings at parallelism 11, all at consecutive parallel-overrun streak 2 of 5. No `SERIAL CONFIRMED OVER BUDGET` breach was reported. The same classes of contention-sensitive findings were present in the clean-base baseline and do not establish a serial budget regression.

### Native acceptance remains a separate gate

This manual bootstrap verification proves the source suite and deterministic generated contracts; it is not an attributed ImplementNext gate and does not prove that a fresh Codex app loads the candidate definitions, resources, executable, or exact operator model settings. Independent whole-branch review and the production-asset native acceptance run remain required before dogfooding change 424.
