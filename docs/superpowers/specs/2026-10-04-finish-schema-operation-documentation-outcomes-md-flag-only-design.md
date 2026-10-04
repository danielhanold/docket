<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0521 — Mark every nested required request field in the schema, and fix the stale schema docs](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0521-finish-schema-operation-documentation-outcomes-md-flag-only.md)**
<!-- docket:backlink:end -->

# Mark every nested required request field in the schema, and fix the stale schema docs — Design

## Problem

Change 0520 (ADR-0138) made `docket schema --operation <id>` publish exactly the JSON file each operation reads, and nothing for flag-only operations. Two gaps were left:

1. **Docs.** `docs/reference/outcomes.md` says `docket schema --operation <id>` "shows one operation's request and result shape". About 23 flag-only operations now show no request. `skills/docket-convention/references/close-out.md` (and its embedded copy under `internal/assets/embedded/tree/`) repeats the claim: "each operation's request and result shape comes from the `schema` operation."
2. **Nested required fields.** The schema marks a field required from its `docket:"required"` tag. `reflectFields` already recurses into nested `internal/app` structs and reports the tag at every depth, but no nested request field carries the tag. The validator still refuses these fields when empty, so an agent reading the schema cannot tell they are mandatory:

| Operation(s) | Nested field | Validator today | Schema today |
|---|---|---|---|
| `adr.supersede`, `adr.reverse` | `target` (object) | missing → refused (its fields are empty) | not required |
| `adr.supersede`, `adr.reverse` | `target.id`, `target.path`, `target.revision` | refused when empty (`invalid-target-id`, `empty-target-path`, `empty-target-revision`) | not required |
| `adr.supersede`, `adr.reverse` | `successor` (object) | missing → refused (`empty-title`, …) | not required |
| `adr.record`, `successor` of supersede/reverse | `change.id`, `change.path`, `change.revision` | refused when empty **if `change` is sent** | not required |
| `change.groom`, `learning.update` | `sections[].heading`, `sections[].intent` | refused when empty (`invalid-section-heading`, `invalid-section-intent`) | not required |
| `finalize.retarget-children` | `children[].id`, `children[].pr_number`, `children[].pr_revision` | refused when empty (`invalid-child_id`, `invalid-child_pr_number`, `empty-child_pr_revision`) | not required |

## The rule

A request field, at any depth, carries `docket:"required"` exactly when the validator always refuses the request if that field alone is empty or zero. A container (object or list) follows the same rule: it is tagged only if leaving it out is refused. A required field inside an optional container means "required whenever that container is sent". Each element of a list is held to its element type's tags.

Fields required only under a condition stay untagged, because the schema has no conditional vocabulary and this change adds none. Example: `sections[].markdown` is required only for `intent: replace`.

## Code changes

All in `internal/app`; tags only, no validator behavior changes:

- `ADRReplaceRequest`: tag `target` and `successor`.
- `ADRTarget`: tag `id`, `path`, `revision`.
- `ADRProducingChange`: tag `id`, `path`, `revision`. `ADRRecordRequest.change` (a pointer) stays optional.
- `SectionEditRequest` (shared by `change.groom` and `learning.update`): tag `heading`, `intent`. `markdown` stays untagged.
- `AuthorizedChild`: tag `id`, `pr_number`, `pr_revision`. `RetargetChildrenInput.children` stays optional (an empty list is accepted).

**Left as is, on purpose:** `successor.request_id` is published as required (it is `ADRRecordRequest`'s own tag), but supersede and reverse ignore it because the outer `request_id` governs. This direction is harmless: an agent that sends it has it ignored. Fixing it would mean splitting `ADRRecordRequest` into a content type plus a wrapper, because request decoding is strict (`DisallowUnknownFields`). Record this in the results file as a known, accepted mismatch. Do not fix it.

## Test

`TestRequiredTagMatchesValidator` (`internal/app/schema_tags_test.go`) compares only top-level keys through the `invalid-<key>` / `empty-<key>` finding-code join. Nested codes do not follow one spelling (`invalid-target-id` vs `invalid-child_pr_number`), so the nested proof must not depend on code names.

Add a nested check driven by field zeroing:

- For each operation whose bound request has a nested object or list-of-objects field, start from a **known-valid fixture**. It passes the operation's shape validator with zero findings and populates every nested container, including optional ones such as `change` and `sections`.
- For each nested field, and for each container field at any depth, zero **that field alone** and run the shape validator:
  - tagged `docket:"required"` → at least one finding;
  - untagged → zero findings.
- Walk the fields by reflection over the same shape `reflectFields` uses, so the set of fields checked is derived and never hand-listed.
- Completeness: every bound request type with a nested struct field is covered by a fixture or exempted with a stated reason, the same way the existing top-level check handles exemptions. `ADRRecordRequest`'s nested `request_id` under `successor` is the one expected exemption (ignored by design, see above). The planner chooses the exemption's form: a field-level skip with a reason is fine.
- Adjust the existing flat `adr.reverse` / `adr.supersede` cases so the newly tagged top-level `target` and `successor` keys are proven by the zeroing check rather than the code join.

Mutation-test the guard and record the probes in the results file. Each of these must turn the test red:

- removing the tag from `ADRTarget.path`;
- removing the tag from `AuthorizedChild.pr_revision`;
- adding a tag to `SectionEditRequest.markdown`;
- dropping one fixture.

## Docs

- `docs/reference/outcomes.md`: replace the opening claim. An operation that reads a JSON file (`--request`, `--input`, or `--body`) shows that file's request shape plus its result shape. A flag-only operation shows only its result shape, and its flags are listed in `docket capabilities --json` (`signature`). Add one sentence: a required field inside an optional object is required only when that object is sent.
- `skills/docket-convention/references/close-out.md` and its embedded copy: reword the closing clause the same way (request shape from `schema` when the operation reads a JSON file, flags from the capability catalog). Edit the source and regenerate or sync the embedded copy through the repo's normal path. Do not hand-diverge the two.
- Docs describe current behavior only: no change or PR citations.

## Out of scope

- Changing validator behavior, finding codes, or which values come from flags versus JSON.
- New schema vocabulary (conditional-required, "ignored").
- Bumping `schema_version`. Adding a required marker to an existing field is a documentation-accuracy fix of the published shape, not a new shape.
- Splitting `ADRRecordRequest` to fix `successor.request_id`.

## Dependencies

Depends on #520: `declareJSONFile`, the current registry bindings (`RetargetChildrenInput`, …), and `TestRequiredTagMatchesValidator` exist only on its branch. Build after it merges.
