<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0589 — Make export its own command: docket export <type> <change_id>](../changes/active/0589-make-export-its-own-command-docket-export-type-change-id.md)**
<!-- docket:backlink:end -->

# Results — Make export its own command: docket export <type> <change_id>

**Human action:** None needed before merge. One optional check is listed below if you want to see the new command work by hand.

## Outcome

Exporting is now its own command: `docket export <type> [<id>]`, where `<type>` is `change`, `spec`, `plan`, or `results`. Its capability-catalog id is `export`. It writes the same clean copy that `docket open … --export` used to write. The file name, the default destination, `--output-path`, `--warnings`, and the export fields in the JSON result are all unchanged. The new command opens the copy only when it is written to the default destination without `--print`. With `--output-path` or `--print`, it prints the path and opens nothing (`launched: false`). The result no longer carries `target` or `target_kind`, and the result type is registered in the schema registry under `export`.

`docket open` no longer accepts `--export`, `--output-path`, or `--warnings`. There is no alias, so passing any of them gets the CLI's ordinary unknown-flag error. `ValidateOpenExport` and the `invalid-option` reason are gone. `open` and `export` share the change resolution, the artifact-path check, and the opener call rather than each keeping a copy.

The `docket-export` skill (and its embedded copy), `docs/reference/cli.md`, `docs/reference/glossary.md`, and the skill's prose sentinel now use the new command.

## Human actions and testing

- **Optional — export a spec by hand.**
  - Prerequisites: a `docket` binary built from this branch (`go run ./cmd/docket` from the worktree works) and any change that has a spec, for example 589.
  - Steps:
    1. Run `go run ./cmd/docket export spec 589 --print --json`. Expect `result: applied`, an `exported_path` under `~/Downloads` (or your home directory on other systems), and `launched: false`.
    2. Run `go run ./cmd/docket open spec 589 --export`. Expect an unknown-flag error.
  - Cleanup: delete the exported file.

## Verification performed

- The full suite (`go run ./cmd/docket development test`) passed at the final head, after the review fix.
- Workers mutation-tested three guards: the launch condition, the `exported_path` JSON tag, and the `--warnings` usage check. They also mutation-tested open's flag rejection and the `docket-export` skill sentinel (`TestAlignmentContracts`). Every mutation turned its test red.
- The catalog byte-budget test (`TestCapabilitiesPayloadWithinByteBudget`) is green without raising the ceiling, because removing open's three flags offset the new `export` entry.
- Review: standard tier. It found one minor issue, which was fixed: the direct-call reuse of the `invalid-target` reason for a bad `--warnings` value now has a comment explaining why.
- Budget screening note, unrelated to this change: the suite flagged `tests/test_go_integration_app_cleanup.sh` as over its serial budget (87s solo against a 60s threshold) on the first gate run. It ran green. This change does not touch that test.
