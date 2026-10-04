<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0505 — Share one unsupported-key matcher between the example-config test and the docs guard](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-04-0505-share-one-unsupported-key-matcher-between-the-example-config.md)**
<!-- docket:backlink:end -->
# Share one unsupported-key matcher between the example-config test and the docs guard — Results

**Human action:** None needed beyond the normal PR review. This is a test-only refactor with no change to how docket behaves for users.

## Outcome

Two repository guards check that configuration keys docket no longer supports do not appear where people would copy them: one scans the example config file `.docket.example.yml`, the other scans the user-facing docs. Each guard had its own copy of the same matching code, and the two had to be kept in sync by hand. Both now call one shared function, `config.UnsupportedKeyShapes`, in `internal/config/unsupported_key_shapes.go`.

The shared matcher also closes a gap both copies had. A key commented out inside a block that was already commented out, such as `#   # terminal_publish: true`, used to pass both guards. It is now flagged. No current file in the example config or the docs trips the wider match.

The function lives in non-test code, so about 50 lines of matcher code are now compiled into the `docket` binary. They have no runtime effect: the regular expressions are built only when a test calls the function. A separate package would not have avoided this, because it would hit the same import cycle that caused the duplication. The spec accepted this cost.

## Verification performed

- New unit test `TestUnsupportedKeyShapes` covers every unsupported key in the schema registry, nested-comment cases, and supported-key and prose-heading negatives. Mutation checks confirmed that reverting the nested-comment fix, or deleting a family of matchers, turns it red.
- Each guard gained one nested-comment case, and each was confirmed red against the old single-marker pattern.
- Whole-branch review (standard tier): 1 minor finding. The first fix failed the suite's gofmt check and was reverted. A formatted fix was then committed after the run and the finding is resolved (see Known issues). Full table in the PR body.
- The full suite (`go run ./cmd/docket development test`) passed at the build gate. Certification of the final head is in the PR's build-evidence block.

## Known issues and follow-ups

None open.

### Resolved after the run: one nested-comment probe did not test what its name said

In `TestUnsupportedKeyShapes`, the probe `#   #     adr: { model: x, runner: codex }` matches through the flow-mapping `{`/`,` branch, which worked before this change too, so it alone would stay green if the nested-comment fix were reverted for leaf keys. A follow-up commit adds the line-start leaf probe `#   #     runner: codex`, gofmt-clean. Mutation check: changing the leaf shape's comment marker from `*` back to `?` turns both this probe and `#   #   cap: 5` red. The full suite passed after the commit (77/77 files).
