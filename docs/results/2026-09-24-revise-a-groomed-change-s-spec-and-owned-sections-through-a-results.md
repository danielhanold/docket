<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0445 — Revise a groomed change's spec and owned sections through a typed operation](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0445-revise-a-groomed-change-s-spec-and-owned-sections-through-a.md)**
<!-- docket:backlink:end -->
# Revise a groomed change's spec and owned sections through a typed operation — Results

**Human action:** Assessment pending: review has not run yet.

## Outcome

Before this change, the only way to adjust a change after grooming was to hand-edit its files and commit them directly. That skipped the version check and the `updated:` stamp that every other metadata write gets.

`change.groom` now accepts a third outcome, `revise`. It works only on a `proposed` change that is already groomed, meaning it has a linked spec or was marked trivial. A revise request can replace the whole spec body (the file stays at its existing path and keeps its backlink block), splice the change's owned sections (Why / What changes / Out of scope / Open questions), or both. Everything lands in one transaction, pinned to the exact version. A revise never writes `spec:` or `trivial:`, so it cannot turn a spec'd change into a trivial one or the other way round. You can revise a change as many times as you like while it stays `proposed`.

The operation refuses, and writes nothing, in these cases:

- `not-revisable`: the change is not groomed yet, or is not `proposed`.
- `spec-not-linked`: the request sends spec text for a change that has no spec.
- `spec-file-missing`: the linked spec file is absent.
- `empty-revise`: the request carries no effective edit.

The result now reports its `outcome` and prints `change NNNN revised — …`.

The `docket-groom-next` skill now sends an explicit id for an already-groomed change to a revise flow. It used to report an error and suggest clearing `spec:` by hand. `docket-new-change` now points to that same flow for adjusting a spec right after it lands.

Departure from the plan: the skill-size word budgets in `internal/repoguard/budgets_test.go` were raised: docket-groom-next from 1650 to 1813 words, docket-new-change from 1700 to 1706. The plan prescribed the new prose word for word, and without the raise the guard would fail.

## Verification performed

Each task ran its own tests through the gate driver, both failing (RED) and passing (GREEN). The gate tests were mutation-checked: when the guarded condition was removed, the matching test failed. The internal/app package, the change-authoring integration slice, internal/cli, and internal/repoguard all passed. The full suite runs as the build gate on this head.

## Known issues and follow-ups

### No plan-level assertion on the revise receipt's spec path

The receipt now records the existing spec path when a revise replaces the spec body. None of the plan-level tests asserts that value, though. The applied-path integration test builds its own receipt. If the value regressed, the audit receipt would show an empty spec path for a spec revise, and no test would fail. Status: suspected gap, not an observed defect. Suggested next step: add one assertion in `change_groom_test.go`.
