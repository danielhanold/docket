<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0522 — Share the JSON-key rules between internal/cli and internal/app](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0522-share-the-json-key-rules-between-internal-cli-and-internal-a.md)**
<!-- docket:backlink:end -->
# Share the JSON-key rules between internal/cli and internal/app — Results

**Human action:** None needed. This is a behavior-preserving refactor, and the existing cross-check tests cover it.

## Outcome

The code that lists the JSON keys a request file accepts now lives in one place. `internal/app` exports `RequestJSONKeys`, and both it and the private `requiredJSONKeys` (the same keys filtered to `docket:"required"` fields) run on one shared walk. The CLI's separate copy (`internal/cli/requestkeys.go`) is gone. The unknown-key refusal message ("accepted keys: …") and the published-schema cross-check tests now call `app.RequestJSONKeys`. The two comments that pointed readers at the old CLI copy now name the shared function. Nothing changes for users: the refusal text and `docket schema` output are the same as before.

## Verification performed

- Each build task ran focused tests. The moved unit tests (`TestRequestJSONKeysReconcile`, `TestRequestJSONKeysSkipsAndPromotes`) and a new `TestRequiredJSONKeysFiltersTheSharedWalk` pass in `internal/app`. Two mutations of the shared walk (removing embedded-struct promotion, and removing the required filter) turned them red. `TestPublishedRequestIsTheDecodedJSONFile` and the unknown-key refusal test pass in `internal/cli`.
- A whole-repo grep finds no remaining `requestJSONKeys` in Go source.
- The full suite ran through the build gate at the final head.
