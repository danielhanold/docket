<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0577 — Fix duplicate originHasBranch breaking combined-tag go vet](../../changes/active/0577-fix-duplicate-originhasbranch-breaking-combined-tag-go-vet.md)**
<!-- docket:backlink:end -->

# Fix duplicate originHasBranch breaking combined-tag go vet — design

## Why

`go vet -tags integration,e2e ./...` fails to compile the `internal/app` test package with one error, `originHasBranch redeclared in this block`. Two test helpers in that package share the name, each behind a different build tag:

- `internal/app/finalize_e2e_test.go` (`//go:build e2e`) declares `originHasBranch(t, origin, branch)`. It takes a short branch name (`feat/x`), probes `refs/heads/<branch>` with `tryGit`, and treats any git error as "absent". All five callers are in that file.
- `internal/app/leak_check_private_integration_test.go` (`//go:build integration`, added 2026-10-06 with the private-mode leak check) declares `originHasBranch(t, origin, ref)`. It takes a full ref (`refs/heads/feat/x`), probes it with `for-each-ref`, and fails the test on any probe error other than a missing ref. Its sibling `originRefTip` and the leak-check tests in that file call it, and so does `internal/app/finalize_cleanup_integration_test.go`.

Each tag compiles fine on its own, and the suite only ever vets them separately: `tests/test_go_finalize_e2e.sh` runs `go vet -tags e2e ./internal/app/`, and `tests/test_go_integration_contract.sh` runs `go vet -tags integration` over the integration packages. No suite row, CI workflow, editor config, or doc runs both tags together. The clash was found by a reviewer who ran the combined vet by hand, and it was recorded as a known issue. The harm is small, but anyone who vets both tags together hits a compile error that has nothing to do with their work.

The integration helper is also misnamed: it checks a ref, not a branch.

## Design (decided at grooming)

Rename the integration-tag helper from `originHasBranch` to `originHasRef`, at its declaration and at every caller, and update its doc comment to the new name. The e2e helper keeps its name and its behavior. No behavior changes, no new file, and no shared helper.

Find the callers with a grep for the identifier across `internal/app`'s integration-tagged files, not from the list above. At grooming they were the declaration, `originRefTip`, and the leak-check tests in `leak_check_private_integration_test.go`, plus one call in `finalize_cleanup_integration_test.go`.

Grooming checked the rename with `go vet -overlay` on renamed copies of the two integration files, leaving the tree untouched. With both tags, vet passed on `./internal/app/` and on `./...`. It also passed with each tag alone, and the combined run reported no other error.

The check that fails before the fix is the combined vet itself. No new test file is needed.

## Acceptance

- `go vet -tags integration,e2e ./...` exits 0.
- `go vet -tags integration ./internal/app/` and `go vet -tags e2e ./internal/app/` still exit 0.
- `internal/app` declares `originHasBranch` exactly once, in `finalize_e2e_test.go`.
- The whole suite (`build.test_command`) passes.
- The results file's verification section records the combined-vet run as the one-time proof.

## Non-goals

- **No new suite check or CI step for the combined tags (decided at grooming).** Nothing runs both tags together, so a guard would turn a harmless future clash into a failed build. A future clash gets found the same way this one was.
- **No shared helper (decided at grooming).** The two helpers stay separate, and the e2e helper keeps treating probe errors as "absent". Unifying them means more churn for a risk the e2e test setup already catches.
- No other renames or cleanups in these test files.

## Left to the builder

- The exact wording of the renamed helper's doc comment.
