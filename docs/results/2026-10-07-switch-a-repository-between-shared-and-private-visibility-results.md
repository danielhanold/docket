<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0533 — Switch a repository between shared and private visibility](../changes/active/0533-switch-a-repository-between-shared-and-private-visibility.md)**
<!-- docket:backlink:end -->

# Switch a repository between shared and private visibility — Results

**Human action:** Assessment pending: the whole-branch review returned findings that are being fixed in-branch now.

## Outcome

Build complete and green at c5562829d (12 plan tasks plus one repair). `docket repository set-visibility <shared|private>` exists with preview, `--yes`, resume, and live-run refusal. `repository check` reports, and `repository repair` re-stamps, absolute artifact backlinks as relative links. The whole-branch review is in its fix pass.

## Verification performed

- Full suite (`go run ./cmd/docket development test`): attempt 1 red on the upgrade-guide finding table (no row for the new `artifact-backlink-stale` code); fixed by one added guide row; attempt 2 green at c5562829d. Screening lines only (PARALLEL-SENSITIVE, SERIAL CONFIRMATION DUE for `test_go_race.sh`); no serial-confirmed budget breach.
- Whole-branch review (deep tier) returned 12 findings, persisted below before the fix pass:
  1. Blocker: going shared with `cursor` in `agent_harnesses` wedges, because the add commit tries to commit the ignored `.cursor/rules/docket-dispatch.mdc`.
  2. Important: going private leaves the generated Cursor rule behind; with `--remove-shared-files` it becomes an untracked docket-named file.
  3. Important: re-running `set-visibility private` refuses once origin's docket branch has moved on (publish judged against origin's current tip).
  4. Important: `set-visibility shared` on an already-shared repository is not a no-op.
  5. Important: the new test shard's runtime budget is 150s; repository policy says split into sibling shards.
  6. Important: the ADR the spec expects was not recorded.
  7. Minor: the identity-keys stop reports `applied`.
  8. Minor: the preview's commit file list is not the real one.
  9. Minor: the dirty-file precondition checks all candidate paths, not the pending commit's.
  10. Minor: the commit-path set is a third hand-written list of surface paths.
  11. Minor: an unreadable private layout is read as "no store".
  12. Minor: a round trip re-encodes a hand-formatted `.docket.yml`.
