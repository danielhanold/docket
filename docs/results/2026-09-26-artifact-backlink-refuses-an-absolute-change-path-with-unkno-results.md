<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0460 — artifact.backlink refuses an absolute --change path with unknown-change](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-26-0460-artifact-backlink-refuses-an-absolute-change-path-with-unkno.md)**
<!-- docket:backlink:end -->
# artifact.backlink refuses an absolute --change path with unknown-change — Results

**Human action:** None required. The fix is covered by automated tests; a reviewer only needs to read the PR diff.

## Outcome

`docket artifact backlink --change <path>` used to answer `unknown-change` when given an absolute path or an oddly spelled one (`./docs/...`, `a//b`, a `..` escape). That made it look like the change record was missing, when the real problem was the path's form. The command now checks `--change` the same way `--artifact` and the plan/results attach commands already check their paths:

- an absolute path is refused with reason `absolute-path`;
- an empty value, a `..` escape, or a non-canonical spelling is refused with reason `path-escape`;
- each message names the `--change` flag and shows the expected repo-relative form.

Only a well-formed repo-relative path that matches no record still returns `unknown-change`. Nothing is written when any of these refusals fires. Absolute paths are still not accepted, as the spec decided.

The `docket-implement-next` skill text that led an agent to pass an absolute path now says "repo-relative" at both places it passes `artifact.backlink` flags, and the embedded copy of that skill was regenerated.

## Verification performed

- New table test `TestArtifactBacklinkChangePathValidation` covers absolute, `..` escape, `./` spelling, interior `..`, trailing slash, empty and whitespace-only values. It failed before the fix (all reported `unknown-change`) and passes after it. The artifact file is checked byte-identical after each refusal.
- Mutation check: removing the absolute-path branch made the `absolute` case fail, as intended.
- The embedded-asset drift guard and harness golden tests pass after regeneration.
- The full suite runs at the build gate; its result is recorded in the PR's build-evidence block.
