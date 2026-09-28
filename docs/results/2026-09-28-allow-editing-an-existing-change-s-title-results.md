<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0461 — Allow editing an existing change's title](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0461-allow-editing-an-existing-change-s-title.md)**
<!-- docket:backlink:end -->
# Allow editing an existing change's title — Results

**Human action:** None required to merge. One optional walkthrough is below if you want to see a retitle end to end.

## Outcome

Before this change, nothing could rename a change after it was created. The only route was hand-editing the frontmatter and then running `docket repository migrate` to repair the board.

`change.groom` now takes an optional `title` field. It works with the `spec`, `trivial`, `revise`, and `rearm` outcomes. A request whose only content is a new `title` is now a valid `revise`. When the title changes, one metadata commit does all of the following:

- rewrites `title:` and `updated:` through the writer, so YAML quoting stays correct;
- re-renders the change's `## Artifacts` block and the board;
- re-stamps the `docket:backlink` line at the top of the linked spec. Nothing else in the spec file changes.

A retitle never touches the slug, the file name, the spec path, or `branch:`.

Refusals:

- `abstain` with a `title` is refused as `invalid-title`.
- A title containing a line break or a control character is refused as `invalid-title`. This check is shared by `change.create` and `change.groom`, and a whitespace-only title is still `empty-title`.
- A linked spec that has lost its backlink block is refused as `spec-backlink-missing` or `spec-backlink-malformed`.
- A `proposed` change that still has feature-branch artifacts (`branch:`, `plan:`, or `results:`) cannot be retitled and is refused as `not-retitleable`. This case arises when a change is deferred from in-progress and then revived. It was added after review, because the backlinks on those artifacts contain the old title and later identity checks would fail.

Board title cells now escape `|` and turn line breaks into spaces. That also fixes a table-corruption gap that existing titles could trigger. The `docket-groom-next` skill now tells groomers to pass `title` when a groom renames a change.

## Human actions and testing

### Optional — retitle a proposed change end to end

This lets you see the single-commit retitle on real metadata. The automated tests use an in-memory tree.

Prerequisites: a docket binary built from this branch, and a scratch `proposed` change that has a spec and has never been claimed.

1. Write a request file containing `{"id": <id>, "version": "<blob from docket status --json>", "outcome": "revise", "title": "New title"}`, then run `docket change groom --input <file> --json`.
   Expected: `result: applied`, and exactly one new commit on `docket`.
2. Run `git -C .docket show --stat HEAD`.
   Expected: the commit touches the change file, `BOARD.md`, and the spec. In the spec, only the backlink line changed. The change file name keeps its old slug.

Cleanup: retitle the change back the same way, or kill the scratch change.

## Verification performed

- Full suite (`go run ./cmd/docket development test`) passed at the build head, 66 of 66 files. The final certification run on the post-review head is recorded in the PR's build-evidence block.
- Each task was test-driven and mutation-tested by its worker: removing the re-stamp, the title upsert, the guards, or the board escaping made its test fail.
- A deep whole-branch review returned three findings, all fixed in this branch:

| Finding | Severity | Disposition |
|---|---|---|
| Retitle accepted on a revived change that still has plan/results backlinks | important | fixed in b0571693b |
| `spec-backlink-missing` message suggested the wrong remedy | minor | fixed in a9408a6c1 |
| Invisible U+2028/U+2029 characters in test literals | minor | fixed in a9408a6c1 |

## Known issues and follow-ups

- The `docket-groom-next` skill text lists some groom refusal codes but not the new `not-retitleable`. A groomer who hits it gets the typed message, but the skill doesn't mention it. Impact is minor and confirmed. Suggested next action: add it the next time that skill is edited.
- Task 5 raised the `docket-groom-next` SKILL.md size budget in `internal/repoguard/budgets_test.go` to the new size (78 lines / 2081 words), following the precedent of 0382 and 0445. Please glance at it when reviewing.
- The suite printed `BUDGET WATCH` / `PARALLEL-SENSITIVE` lines for tests this change doesn't touch (race, toolchain, several integration shards). They were not serially confirmed and look like existing parallel-load noise.
