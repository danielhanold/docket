<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0312 — Planning mutations, inline board, and ADRs](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-17-0312-planning-mutations-board-and-adrs.md)**
<!-- docket:backlink:end -->

# Planning mutations, inline board, and ADRs — results
Change: #312 · Branch: feat/planning-mutations-board-and-adrs · PR: opened at close-out · Plan: docs/superpowers/plans/2026-08-16-planning-mutations-board-and-adrs.md · ADRs: none

## Verify (human)

<!-- No genuinely-manual merge-gate checks: the full suite (121/121, incl. both repository modes and the real-git concurrency matrix) plus the golden-provenance asserts cover this slice end to end. -->
- [ ] None — automated coverage is sufficient; the PR diff + green suite are the receipt.

## Findings

- **internal/render + internal/app slice landed as specified.** Ten typed operations (`change create/groom/block/defer/kill`, `learning record/update`, `adr record/supersede/reverse`) plus the pure `internal/render` package (fence-aware source-preserving section editor, canonical record serializers, artifact/backlink/board/ADR-index renderers with goldens frozen-with-provenance from the Bash renderers) and the `internal/cli` command families. Every operation lands its source mutation + all affected v1-owned derived views as one atomic transaction; creation is idempotent; retries recompute allocation and derived bytes from fresh state.
- **Deep review (docket-review-deep) returned 0 blockers, 1 important, 1 minor — both fixed in-branch:**
  - *important* — `change kill` hard-failed `internal-error` when a linked spec existed but carried no `docket:backlink` block. Fixed by probing `specDoc.Block("backlink")` and skipping the spec mutation when absent, matching the documented absent-spec contract. (commit `3b163f5d`)
  - *minor* — `change block/defer/kill/groom` used a bare `SetField("updated", …)` that `internal-error`s on a record lacking `updated:`, less tolerant than the ADR ops on the same corpus. Fixed by reusing the existing `upsertField` helper across the three ops; the `docket:artifacts` `ReplaceBlock` block-present assumption (identical to the ADR ops) is now documented in a code comment at each site. (commit `64c4eddf`)
- **No ADRs produced.** The reviewer confirmed no implementation decision rose to a novel architecture choice beyond the approved spec; the candidate-snapshot rebuild, board-surface `github` fence, and exact-blob version pinning are all within the 0309/0310/0312 design.
- **gofmt gate fix.** Three files (`internal/app/change_groom_test.go`, `internal/app/learning_ops_test.go`, `internal/render/record.go`) were not gofmt-clean and reddened `test_go_toolchain`; corrected by `gofmt -w`. (commit `8fbd905d`)

## Follow-ups

- **Environment / build-gate notes for the maintainer (not code defects in this change):**
  - The full suite must be launched with `/opt/homebrew/bin` on PATH so the docket **Bash scripts** (which use the Bash-4 `mapfile` builtin) resolve Bash 4+ rather than macOS `/bin/bash` 3.2; a detached gate subprocess whose login shell dropped that PATH produced spurious `mapfile: command not found` failures across ~17 shell test files. The passing gate run used `scripts/run-tests.sh -j 3` under the homebrew Bash with PATH pinned.
  - At full default parallelism this machine saturated (per-file wall-clock budgets blown ~10x, `OVER BUDGET` on ~24 files); `-j 3` runs clean at 121/121. `test_gate_run_stop` (a timing-sensitive `--stop` barrier test of **unmodified baseline** `scripts/gate-run.sh`, untouched by this change) is flaky under parallel load — green 141/141 in isolation and in the final full run; worth de-flaking independently of this slice.
