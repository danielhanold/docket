# migrate-to-docket.sh $PWD-target — results
Change: #3 · Branch: feat/migration-tool-pwd-target · PR: <set at PR open> · Plan: docs/superpowers/plans/2026-06-04-migration-tool-pwd-target.md · ADRs: none

## Verify (human)

Automated + behavioral coverage is green (the 4 new assertions in `test_docket_metadata_branch.sh`, plus `test_sync_convention` / `test_results_artifact` / `test_link_skills`; `bash -n` clean). The whole-branch review *executed* the load-bearing paths — `$PWD`→toplevel resolution from a subdir, the `/dev/tty` confirm, the `--yes` non-tty bypass, the outside-a-repo `die`, and that seed/prune/idempotency still run on the resolved root. So this is light:

- [ ] Skim the README **"Migrating an existing repo"** section on GitHub — the run-from-within-the-target-repo usage + the confirm/`--yes` note read correctly and render.

## Findings

- **No ADR** — purely an ergonomics fix to the migration tool; the branch model (ADR-0001/0002) is untouched.
- **A third repo hit the gap:** `~/dev/markhaus` was migrated 2026-06-04 via the `/tmp`-patched-`cd` workaround — confirmed the motivation at reconcile; native `$PWD` targeting supersedes it. Design unchanged.
- **Review found 2 minors:** fixed one (suppress the raw `/dev/tty: Device not configured` on a no-tty abort, so the clean "aborted" message stands alone — commit `5249dfb`); deferred one (the `-y` test-assertion regex is loose but proven non-vacuous by a revert test).

## Follow-ups

- **Adjacent migration-tool gaps (deliberately out of scope here):** `migrate-to-docket.sh` still (1) does **not** flip `.docket.yml` `metadata_branch: main → docket` (a lingering `main` silently pins single-branch mode), and (2) does **not** fast-forward the local integration branch after its transient-worktree push. Both are hand-done today (see project memory `docket-migration-tool-not-distributed`). Worth a future change — arguably the higher-value follow-up now that `$PWD` targeting lands.
- **Deferred minor:** tighten the `-y` assertion regex in `tests/test_docket_metadata_branch.sh`.
