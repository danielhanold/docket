<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0468 — Rename colliding docket terms and retire obsolete glossary entries](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-29-0468-rename-colliding-docket-terms-and-retire-obsolete-glossary-e.md)**
<!-- docket:backlink:end -->
# Rename colliding docket terms and retire obsolete glossary entries — Results

**Human action:** None needed to merge. Read the wording in the PR diff to confirm the new names ("PR handoff", "shared-setting guard", "final status", "archived record", "close-out") read well where they appear. The wire-token renames are left for changes 0471–0474.

## Outcome

Some docket words had more than one meaning. This change settles every rename in one decision record and applies the renames that are prose only:

- **ADR-0129, "Collision-free docket vocabulary"**, is on the `docket` branch. It holds the naming rules and the full 66-row rename table, with each row assigned to the change that owns it.
- **Coordination-key fence → shared-setting guard**, in docs, skills and the `internal/config` comments. Go identifiers and the `fenced-setting-ignored` code stay; change 0474 renames that code.
- **Human merge gate → PR handoff.** The "merge gate", "rebase-retest gate" and "test gate" aliases are gone; **finalize gate** and **suite gate** remain.
- **Change-lifecycle "terminal" → final.** "Terminal status" is now "final status", "terminal record" is "archived record", "terminal sweep" is "merged-PR sweep", and "terminal close-out" is "close-out". This covers docs, skills and Go comments or messages. Where "terminal" means a run or process has finished, it stays.
- **"Autonomous-eligible"** is now part of the auto-groomable entry.
- **Obsolete terms.** The glossary has a new section for retired features: runner delegation, the runner shim, `runtime.bash` and terminal publish.

The build ran with no changes to identifiers, test names or wire tokens. It regenerated the embedded skill and agent copies and the harness goldens for the `docket-implement-next` description.

## Verification performed

- Each task ran its focused tests through the gate driver, and all passed. These covered assets, repoguard prose contracts, harness goldens, config, and the app, domain and reposetup packages. Vet and gofmt were also clean.
- A whole-repo scan for the old phrases in rows 60–65 was re-run after the edits. Every remaining hit was sorted into one of three groups:
  - process-level "terminal";
  - frozen fixtures and point-in-time records;
  - comments owned by change 0474.
- A glossary anchor check found 237 in-page links and none dangling.
- A token-count check confirmed that no wire token owned by a family change was altered.
- A deep whole-branch review returned 0 blockers, 2 important findings and 2 minor findings, and all four were fixed in the branch (commit ff5eab634):
  - the `internal/config` comments now say "shared-setting guard";
  - user-facing CLI help and messages that said "terminal change" now say "final change";
  - "Final status" and "Close-out" are now in the glossary index;
  - the last finalize-gate alias in `docket-status` is gone.
- The full-suite build gate certifies the final head. Its evidence is in the PR body.
- The suite run reported one budget breach: `SERIAL CONFIRMED OVER BUDGET` for `tests/test_go_integration_app_rebaserecovery.sh` (61s solo against a 60s threshold). This change only edits prose, so that test was already slow before it. It does not fail the run.

## Known issues and follow-ups

### Fixed after review: `.docket.yml` wording

The two lifecycle "terminal records" comments in this repo's own `.docket.yml` now read "published archived records" and "final records". `TestFixtureDocketSelf` compares that file byte for byte with a frozen copy, so the fixture was re-cut as `testdata/repositories/v0.9.8/` and the test now points at it. No key or value changed.

### Fixed after review: leftover lifecycle "terminal child" comments

The comments and test messages that said "terminal child" or "terminal outcomes" for a finished stacked change now say "final": `internal/app/finalize_merge.go`, `internal/domain/stackcloseout.go`, `internal/app/finalize_retarget_test.go` and `internal/app/finalize_e2e_test.go`. The `skipped-terminal` token, its constant, and the test function name `TestRetargetChildrenSkipsTerminalChildren` are unchanged; change 0474 owns them (row 59). Process-level "terminal" (a finished run) and the finalize "terminal half" stay, per Decision 6.

### Fixed after review: change 0469 drops rows now owned by ADR-0129

Change 0469's rename table no longer lists "gate key / dispatch context" (rows 5 and 6) or "dispatch tiers A / B / C + carve-out" (rows 48–52). Its out-of-scope section now points to ADR-0129 for them.
