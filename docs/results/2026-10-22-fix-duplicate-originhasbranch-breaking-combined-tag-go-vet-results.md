<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0577 — Fix duplicate originHasBranch breaking combined-tag go vet](../changes/archive/2026-10-22-0577-fix-duplicate-originhasbranch-breaking-combined-tag-go-vet.md)**
<!-- docket:backlink:end -->

# Fix duplicate originHasBranch breaking combined-tag go vet: results

**Human action:** None needed. The fix only renames a test helper, and the full suite passed.

## Outcome

The integration-tag test helper `originHasBranch` in `internal/app/leak_check_private_integration_test.go` is now called `originHasRef`. The rename covers its declaration, its doc comment and every caller, including the one in `finalize_cleanup_integration_test.go`. The e2e helper of the same name in `finalize_e2e_test.go` is unchanged. Running `go vet` with both tags together now compiles. Nothing behaves differently.

## Verification performed

- One-time proof with the combined tags: `go vet -tags integration,e2e ./internal/app/` exited 1 before the fix (`originHasBranch redeclared`). After the fix, `go vet -tags integration,e2e ./...` exits 0.
- `go vet -tags integration ./internal/app/` and `go vet -tags e2e ./internal/app/` both still exit 0.
- `internal/app` now declares `originHasBranch` only once, in `finalize_e2e_test.go`.
- The whole suite (`go run ./cmd/docket development test`) passed at the feature head through the build gate. The budget report contained only `BUDGET WATCH` screening notes, with no breaches.
- The lean-tier review ran and returned no findings.
