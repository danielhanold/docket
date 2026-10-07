<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0505 — Share one unsupported-key matcher between the example-config test and the docs guard](../../changes/archive/2026-10-04-0505-share-one-unsupported-key-matcher-between-the-example-config.md)**
<!-- docket:backlink:end -->

# Share one unsupported-key matcher between the example-config test and the docs guard — design

## Problem

Two guards fail when a config key the schema registry marks unsupported is spelled where it should not be:

- `TestExampleSchemaCorrespondence` (`internal/config/example_correspondence_test.go`), Direction C, scans `.docket.example.yml`, using `exampleUnsupportedKeyShapes`.
- `TestLivingDocsAlignment` (`internal/repoguard/docs_alignment_test.go`) scans the living docs, using `unsupportedKeyShapes`.

The two shape-derivation functions are logic-identical copies (the names of the function, the shape struct, and the segment-class constant differ; the regexps do not). The config copy says why it exists: "the living docs guard, which cannot be imported here without an import cycle". That is accurate. `repoguard` imports `config`, so config's internal test cannot import `repoguard`. The repoguard copy also lives in a `_test.go` file, so nothing could import it anyway.

Both copies share one gap. The two line-start shapes allow at most one `#` before the key (`(?:#[ \t]*)?`), so a key commented out inside an already-commented block (`#   # terminal_publish: true`, or `#   #   cap: 5` in a mixed block) is not matched.

## Decision

### One copy, exported from `internal/config`

Move the derivation into a new non-test file in package `config`, next to `SettingPaths()`, the registry it reads:

```go
// internal/config/unsupported_key_shapes.go
type UnsupportedKeyShape struct {
	Name string         // the shape's display name, e.g. "skills.build", "skills:", "cap:"
	Re   *regexp.Regexp
}

func UnsupportedKeyShapes(paths []SettingPath) []UnsupportedKeyShape
```

- The body is the existing derivation, unchanged except for the nested-comment fix below. Keep the `paths` parameter, which the repoguard copy already takes, so callers and unit tests can pass a registry.
- `TestExampleSchemaCorrespondence` (`directionCViolations` and its non-vacuity block) calls `UnsupportedKeyShapes(SettingPaths())`. Delete `exampleUnsupportedKeyShapes`, `exampleKeyShape`, and `exampleKeySegClass`, along with the "mirrors repoguard's … import cycle" comment.
- `TestLivingDocsAlignment` calls `config.UnsupportedKeyShapes(config.SettingPaths())`, and `scanUnsupportedKeys` takes `[]config.UnsupportedKeyShape`. Delete `unsupportedKeyShapes`, `keyShape`, and `keySegClass`. The citation scanner is unaffected, because `citationShapes` uses its own anonymous struct.
- No new package. A separate leaf package was rejected. Config's internal test cannot import a package that imports `config`, so a leaf package could not use `SettingPath` and every caller would have to translate the registry into a second type. The accepted cost: about 50 lines that only tests call are compiled into the binary. They have no runtime effect, because regexps are compiled when the function is called, never at init.

### Close the nested-comment gap

In the two line-start shapes, change the optional single comment marker `(?:#[ \t]*)?` to `(?:#[ \t]*)*`:

- top-level key shape: ``(?m)(?:^[ \t]*(?:#[ \t]*)*(?:-[ \t]+)?|`)<top>:``
- mixed-block leaf shape: `(?m)(?:^[ \t]*(?:#[ \t]*)*|[{,][ \t]*)<leaf>:`

Before: `#   # terminal_publish: true` and `#   #   cap: 5` pass both guards. After: both are flagged. Dotted-path shapes are unaffected.

Side effect: a markdown heading spelled `## <unsupported-top-key>:` is now matched in the living docs. That spelling is already banned there. At grooming time (main after 0464's merge), the only multi-`#` key-shaped lines in `.docket.example.yml` and the living docs are `# scope:` tags and prose headings (`## Reconcile:`, `### Readiness:`). None is an unsupported key, so no current file newly fails. Re-verify at reconcile.

Update repoguard's `# Limits` doc comment to say that commented and nested-commented YAML keys are matched.

## Tests

- New `internal/config/unsupported_key_shapes_test.go` unit-tests `UnsupportedKeyShapes` directly:
  - Registry-derived population: every unsupported `SettingPaths()` entry, spelled as a key, is matched. This lifts the probe both guards carry today; the guards may keep their own copies.
  - Nested-comment probes: `#   # terminal_publish: true`, `# # skills:`, `#   #   cap: 5`, and a flow-mapping leaf inside a nested comment.
  - Negatives: the existing supported-text negatives (`build:`, `review:`, `plan:`, `finalize.gate: local`, `learnings.enabled`, `auto_groomable: true`), plus a nested-comment supported key (`#   # gate: local`) and a prose heading (`## Reconcile: x`).
- Guard-level non-vacuity gains one nested case each. Add `"#   # terminal_publish: true"` to `TestLivingDocsAlignment`'s flagged list, and add a planted nested-comment unsupported key to Direction C's planted cases in `TestExampleSchemaCorrespondence`.
- Mutation check: revert `*` to `?` in either shape and the nested probes must go red. Delete a shape family and the population probe must go red.
- The whole suite (`build.test_command`) at the build gate.

## Out of scope

- Which keys count as unsupported, and the registry itself.
- What either guard scans beyond this matcher: citations, `maskCode`, the structural extractor, the agents-table check, and refused values of supported keys (still review-only, as 0464 decided).
- Reworking either guard's existing non-vacuity beyond the nested additions.
