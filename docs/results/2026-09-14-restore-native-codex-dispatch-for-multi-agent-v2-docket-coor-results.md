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
- The native fixture generator commits discovered build/finalize gate configuration, obtains metadata versions from typed `status`, runs `go test ./...`, verifies build readiness and a clean primary, and records complete source, binary, pin, command, HEAD, and file-hash evidence in its manifest.

Permanent regression coverage includes the original review overlay cases, real Git workspace ownership/ref/stack/terminal cases, descendant committed-path checks, review ref/evidence/dirty checks, full wired resolver and repair entry against real finalize receipts, constructible schema output, and a successful end-to-end native fixture preparation. The review overlay and focused package/tagged integration checks passed before this results update. The complete configured source suite is run from the clean commit containing this results record; its final-head receipt remains in the external launch-kit verification evidence to avoid a self-referential commit hash.

The first complete post-review run found two suite-registration defects in the new tests: a default-tag real-Git file used the reserved `_integration_test.go` suffix, and the tagged finalize entry case did not match a declared shard prefix. Renaming the file and placing the tagged case under the existing finalize-rebase shard corrected the integration contract; `gofmt` also normalized the expanded reference guard before the final rerun.

## Verification performed

- The configured source command `go run ./cmd/docket development test` passed at `e861d4968c04621ed82b18aa096bee36a28c5b1f`: 46 of 46 suite files passed, with 399 assertions and no failed result markers.
- The repaired `TestProcessExitSitesAreAllowlisted` passed both normally and under race instrumentation; `tests/test_go_toolchain.sh` passed all six result markers.
- Generated source consistency checks passed: `go run ./cmd/genassets -repo "$PWD" -check` and `go run ./cmd/gendispatch -repo "$PWD" -check`.
- The implementation pass also recorded passing focused Go packages, tagged workflow integration coverage, Linux `codexcontract` compilation, source capability/schema inspection, and literal Bash/zsh first-response capture cases.

## Findings and limitations

### Parallel budget screenings

The final source suite reported seven `BUDGET WATCH` findings at parallelism 11, all at consecutive parallel-overrun streak 2 of 5. No `SERIAL CONFIRMED OVER BUDGET` breach was reported. The same classes of contention-sensitive findings were present in the clean-base baseline and do not establish a serial budget regression.

### Native acceptance remains a separate gate

This manual bootstrap verification proves the source suite and deterministic generated contracts; it is not an attributed ImplementNext gate and does not prove that a fresh Codex app loads the candidate definitions, resources, executable, or exact operator model settings. Independent whole-branch review and the production-asset native acceptance run remain required before dogfooding change 424.
