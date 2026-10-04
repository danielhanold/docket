<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0521 — Mark every nested required request field in the schema, and fix the stale schema docs](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0521-finish-schema-operation-documentation-outcomes-md-flag-only.md)**
<!-- docket:backlink:end -->
# Mark every nested required request field in the schema, and fix the stale schema docs — Results

**Human action:** None needed before merge. The change only adds schema markers, a test, and doc wording, and the full suite passed on the final head.

## Outcome

`docket schema --operation <id>` now marks nested request fields as required wherever the validator always refuses them when blank. That covers the ADR `target` and `successor` objects and their `id`/`path`/`revision` pins on `adr.supersede` and `adr.reverse`, the producing `change` pins on ADR requests (required only when `change` is sent), `sections[].heading` and `sections[].intent` on `change.groom` and `learning.update`, and the `children[]` pins on `finalize.retarget-children`. Validator behavior and finding codes did not change.

`docs/reference/outcomes.md` and the docket-convention `close-out.md` reference (plus its embedded copy) no longer claim that every operation publishes a request shape. A flag-only operation shows only its result shape, and its flags come from `docket capabilities --json`.

## Verification performed

- A new test, `TestNestedRequiredTagMatchesValidator`, starts from a valid request for each operation with nested fields, blanks one field at a time, and checks that tagged fields are refused and untagged fields accepted. The field list comes from reflection, not a hand-written list.
- Mutation probes, each of which turned the test red: removing the tag from `ADRTarget.path`; removing the tag from `AuthorizedChild.pr_revision`; adding a tag to `SectionEditRequest.markdown`; dropping the `change.reconcile` fixture; deleting the `successor.request_id` exemption.
- Focused tests for `internal/app`, the schema and finalize tests in `internal/cli`, and `internal/assets` / `internal/repoguard` passed.
- After the review fixes, `docket schema --json` output was compared before and after the shared-helper refactor and was byte-identical.
- The full suite (`go run ./cmd/docket development test`) passed through the build gate. It printed four `PARALLEL-SENSITIVE` screening lines for slow tests this change does not touch (`test_go_finalize_e2e.sh`, `test_go_integration_app_changeruntime.sh`, `test_go_integration_app_merge.sh`, `test_go_race.sh`) and no `SERIAL CONFIRMED OVER BUDGET` line.
- Whole-branch review (standard tier): 2 minor findings, both fixed in-branch; full table in the PR body.

## Known issues and follow-ups

- **`successor.request_id` is published as required but ignored.** When an agent sends an `adr.supersede` or `adr.reverse` request, the schema says `successor.request_id` is required, but the operation ignores it because the outer `request_id` controls idempotency. Impact: harmless; an agent that sends it has it ignored. Confirmed, accepted by design, and exempted in the new test with a staleness check. Fixing it would mean splitting `ADRRecordRequest` into two types. No action suggested.
- **`close-out.md` is at its size budget.** The reference now sits at exactly 141 lines and 1349 words, its guarded ceiling. Any future addition there must trim elsewhere or raise the budget deliberately. Confirmed; no action needed now.
- **A fourth copy of the JSON-key rules remains in `internal/cli`.** The review fix moved the struct-field-to-JSON-key rules in `internal/app` into one helper, `jsonFieldKey`. `requestJSONKeys` in `internal/cli` still carries its own copy. If the rules change in one package only, the CLI's published-request guard could disagree with the schema; it would fail loudly rather than pass silently. Confirmed; low impact. Suggested next action: a small refactor to have `internal/cli` reuse an exported form of the helper. No existing change fits (checked 23); a new change a human captures, related to #520.
