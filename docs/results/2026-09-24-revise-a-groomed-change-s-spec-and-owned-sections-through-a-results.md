<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0445 — Revise a groomed change's spec and owned sections through a typed operation](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0445-revise-a-groomed-change-s-spec-and-owned-sections-through-a.md)**
<!-- docket:backlink:end -->
# Revise a groomed change's spec and owned sections through a typed operation — Results

**Human action:** No action is required before merge. One optional walkthrough below lets you try the new revise path by hand.

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

Review found problems that were fixed before the PR opened, and two of the fixes add to the design:

- **Spec-body revises need a version pin.** A revise that replaces the spec body must also send `spec_path` (the change's `spec:` value) and `spec_version` (the spec file's git blob id, for example `git -C .docket rev-parse HEAD:<spec_path>`). If the spec changed after you read it, the call returns `contended` and writes nothing, so a concurrent edit can no longer be silently overwritten. If either field is missing, the request is refused with `empty-spec_path` or `empty-spec_version`. If the path is not the change's linked spec, it is refused with `spec-path-mismatch`.
- **Identical revises are a no-op.** If the revise would not change the record or the spec (for example, a same-day revise with identical text), the call returns `no-op`. Before the fix, the engine failed it.
- **`spec_markdown` cannot contain a backlink block.** On both `spec` and `revise`, `spec_markdown` that includes a `docket:backlink` block is refused with `invalid-spec_markdown`. The operation stamps that block itself, and a second copy would corrupt the spec file.

Departure from the plan: the skill-size word budgets in `internal/repoguard/budgets_test.go` were raised: docket-groom-next from 1650 to 1889 words (raised in steps as the review fixes added wording), docket-new-change from 1700 to 1706. The plan prescribed the new prose word for word, and without the raise the guard would fail.

## Human actions and testing

### Optional — revise a groomed change by hand

Run this if you want to see the new path work end to end. It changes metadata, so use a scratch repository and never the real docket backlog.

Prerequisites: a docket binary built from this branch (`go build -o /tmp/docket-445 ./cmd/docket`) and a disposable repository where docket is initialized and has one `proposed` change with a linked spec.

1. Write a request file `{"id":<id>,"version":"<record blob from docket status --json>","path":"<change path>","outcome":"revise","sections":[{"heading":"## Why","action":"replace","markdown":"New why text."}]}` and run `/tmp/docket-445 change groom --input req.json --json`.
   Expected: `result` is `applied`, and the change's `## Why` now holds the new text.
2. Run the same request again with the new version.
   Expected: `result` is `no-op`.
3. Send `spec_markdown` without `spec_path` and `spec_version`.
   Expected: `invalid-input`, with `empty-spec_path` and `empty-spec_version` findings.

Cleanup: delete the scratch repository.

## Verification performed

Each build task and each review fix ran its own tests through the gate driver, both failing (RED) and passing (GREEN). The new guards were mutation-checked: when a guard was removed, its test failed. The guards covered are the revise gate, the only-changed-paths declarations, the spec version pin (including a real-git race test where a stale version now returns `contended`), the backlink refusal, and the receipt spec_path.

The first full-suite build gate failed because the embedded skill copies were stale after the skill edits. They were regenerated, and the second run passed (54/54 files). Deep-rung review then returned 6 findings: 1 blocker, 2 important, 3 minor. All six were fixed in-branch, and the full suite ran again on the final head before the PR opened.

## Known issues and follow-ups

### A spec_path equal to the change record's own path gives a generic error

If a caller sends the change file's own path as `spec_path`, the engine rejects the request as a duplicate expectation (`failed`, `invalid-input`). It does not return a typed shape finding. Only a malformed request can trigger this, and nothing is written. Status: confirmed, low impact. Suggested next action: add a shape check that refuses this case with a clear finding code, if it ever matters.

### Skill word budgets were raised rather than trimmed

The docket-groom-next and docket-new-change skill word ceilings were raised to fit the new revise instructions. Status: deliberate. If you would rather keep the skills shorter, trimming the prose is a reasonable follow-up.
