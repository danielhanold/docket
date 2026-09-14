<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0425 — Restore native Codex dispatch for Multi-Agent V2 Docket coordinators](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0425-restore-native-codex-dispatch-for-multi-agent-v2-docket-coor.md)**
<!-- docket:backlink:end -->
# Restore native Codex dispatch for Multi-Agent V2 Docket coordinators — Results

## Outcome

Codex's generated Docket role definitions now use the harness's top-level native named-agent dispatch for all 17 registered roles. The implementation adds explicit role-aware feature binding, immutable assignment and private payload validation, resource-closure checks, first-response gate receipt validation, feature-revision artifact diagnostics, and a disposable native-acceptance fixture generator. Other harness routes remain unchanged, and ADR-0119 supersedes ADR-0114's automatic Codex `agent.enter` decision while retaining its ownership invariant.

The full source verification found one in-scope integration defect: the repository's exhaustive process-exit guard had not registered the two new developer command entrypoints. Commit `e861d4968c04621ed82b18aa096bee36a28c5b1f` adds only those exact `main` packages to the guard's allowlist and documents their process-exit contracts.

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
