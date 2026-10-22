<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0577 — Fix duplicate originHasBranch breaking combined-tag go vet](../../changes/active/0577-fix-duplicate-originhasbranch-breaking-combined-tag-go-vet.md)**
<!-- docket:backlink:end -->

# Fix duplicate originHasBranch breaking combined-tag go vet Implementation Plan

> **For agentic workers:** This plan is executed by `docket-build` (one task, routed to a build tier under the `docket-build-task` contract). Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `go vet -tags integration,e2e ./...` compile by renaming the integration-tagged test helper `originHasBranch` to `originHasRef`.

**Architecture:** `internal/app` declares `originHasBranch` twice behind different build tags: `finalize_e2e_test.go` (`//go:build e2e`) and `leak_check_private_integration_test.go` (`//go:build integration`). Each tag compiles alone; together they collide. Renaming the integration helper (which takes a full ref, so `originHasRef` is also the accurate name) at its declaration, doc comment, and every caller removes the clash. No behavior change.

**Tech Stack:** Go test files with build tags; `go vet`.

**Spec:** `docs/superpowers/specs/2026-10-22-fix-duplicate-originhasbranch-breaking-combined-tag-go-vet-design.md`

## Global Constraints

- New name is exactly `originHasRef`; signature unchanged: `func originHasRef(t *testing.T, origin, ref string) bool`.
- The e2e helper `originHasBranch` in `internal/app/finalize_e2e_test.go` keeps its name, signature, and behavior. Do not touch that file.
- No new suite check, CI step, test file, or shared helper. No other renames or cleanups in these files.
- Derive callers from a grep, not from the list below (AGENTS.md: never hand-list the sites of an identifier).

## Review Focus

- A missed integration-tagged caller: the integration tag alone would then fail to compile. Covered by the `-tags integration` vet in Step 4.
- An accidental edit to the e2e helper or its five callers: covered by the `-tags e2e` vet in Step 4 and the single-declaration grep in Step 4.
- None further: the change is a pure identifier rename with no runtime behavior.

---

### Task 1: Rename the integration-tag helper to originHasRef

**Files:**
- Modify: `internal/app/leak_check_private_integration_test.go` (the `originHasBranch` declaration and its doc comment, the call in `originRefTip`, and the calls in the leak-check tests)
- Modify: `internal/app/finalize_cleanup_integration_test.go` (one call, `originHasBranch(t, lw.origin, lw.featureRef)`)
- Do not modify: `internal/app/finalize_e2e_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `func originHasRef(t *testing.T, origin, ref string) bool` in `leak_check_private_integration_test.go`, same body as today (`for-each-ref --format=%(refname) <ref>`, returns `out == ref`).

- [ ] **Step 1: Confirm the failing check**

Run: `go vet -tags integration,e2e ./internal/app/`
Expected: exit 1 with `leak_check_private_integration_test.go:...: originHasBranch redeclared in this block`.

- [ ] **Step 2: Enumerate the sites to rename**

Run: `grep -n 'originHasBranch' internal/app/*.go`
Expected at plan time: the declaration plus 5 calls in `leak_check_private_integration_test.go`, 1 call in `finalize_cleanup_integration_test.go`, and the e2e declaration plus 5 calls in `finalize_e2e_test.go`. Rename every hit in files whose first line is `//go:build integration`; leave every hit in `finalize_e2e_test.go`. If any other file shows a hit, check its build tag the same way.

- [ ] **Step 3: Rename in the two integration files**

Replace `originHasBranch` with `originHasRef` at the declaration and every caller in `leak_check_private_integration_test.go` and `finalize_cleanup_integration_test.go`. Update the doc comment to start `// originHasRef reports whether the bare origin carries ref.` and keep its existing second sentence about probe failures failing the test (exact wording is the builder's call). Run `gofmt -l internal/app/` and expect no output.

- [ ] **Step 4: Verify**

Run each; all must exit 0:
- `go vet -tags integration,e2e ./...`
- `go vet -tags integration ./internal/app/`
- `go vet -tags e2e ./internal/app/`

Then run `grep -n 'func originHasBranch' internal/app/*.go` and expect exactly one hit, in `finalize_e2e_test.go`; and `grep -n 'originHasBranch' internal/app/leak_check_private_integration_test.go internal/app/finalize_cleanup_integration_test.go` and expect no hits.

Record the combined-vet command and its exit 0 in the results file's verification section as the one-time proof (spec Acceptance).

- [ ] **Step 5: Commit**

```bash
git add internal/app/leak_check_private_integration_test.go internal/app/finalize_cleanup_integration_test.go
git commit -m "fix: rename integration-tag originHasBranch to originHasRef so combined-tag vet compiles"
```

The whole suite (`build.test_command`, read from config) runs at the build gate after this task, per the spec's acceptance.
