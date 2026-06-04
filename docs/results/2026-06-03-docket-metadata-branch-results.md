# docket metadata branch — results
Change: #2 · Branch: feat/docket-metadata-branch · PR: <set at PR open> · Plan: docs/superpowers/plans/2026-06-03-docket-metadata-branch.md · ADRs: 1, 2

## Verify (human)

Automated coverage is green (`test_docket_metadata_branch` 40 ok, `test_sync_convention`, `test_results_artifact`, `test_link_skills` all pass; `sync-convention.sh --check` clean). These need a human eye at the merge gate:

- [ ] **Render the README docket-mode section on GitHub** — confirm the artifact-location table renders as a table and there's no broken markdown / stale "rough edge" / "pseudo-database" language.
- [ ] **Skim `migrate-to-docket.sh` for safety.** The whole-branch review ran it end-to-end in throwaway repos (fresh / interrupted-unpushed / already-migrated — all converged or aborted correctly), but it has **never been run against this repo**. Eyeball: idempotent (split mutation/push), tolerant `git rm --ignore-unmatch`, `ls-tree` probes, aborts if `origin/docket` exists, no force-push, only `rm -rf` of a `mktemp` dir.
- [ ] **Confirm the rollout seatbelt is in place** — `.docket.yml` on `main` pins `metadata_branch: main` (commit `ec55688`). This is what keeps docket's *own* skills working after this PR flips the default to `docket`. Without it, every docket skill would hit the bootstrap guard and STOP on merge.
- [ ] **Sanity-read the convention-block diff** once — it's synced byte-identical into all five skills, so an error here propagates everywhere (the test asserts sync + the load-bearing content, but a human read is cheap insurance).

## Findings

- **Recorded as ADRs:** ADR-0001 (orphan `docket` branch + `.docket/` worktree + publish-by-copy-not-merge) and ADR-0002 (docket-mode default + refuse-and-migrate bootstrap + terminal-publish single-sourced in finalize).
- **Build-discovered correctness fix:** the §7.0 `LIVE` probe (and migration) referenced a repo-root `BOARD.md`, but the board lives at `<changes_dir>/BOARD.md`. Fixed in the convention block + the spec (commits `4878db0` feature-side, `40edc0d` spec-side).
- **Build decision (→ ADR-0002):** the terminal-publish procedure is single-sourced in `docket-finalize-change`; `docket-new-change`, `docket-implement-next`, `docket-status`, and `docket-adr` reference it rather than duplicating the git sequence (it's an operational procedure, not a convention contract).
- **Reconcile was a currency check:** the only world-movement since proposal was a user-added `.gitignore` (`b2a75ae`); folded in (extend, not create).
- **Test suite hardened post-review:** the originally-vacuous `main`-mode backward-compat assertion was replaced with 5 real, proven-non-vacuous assertions (each flips to NOT OK if its degradation clause is deleted).

## Follow-ups

- **Dogfood: migrate this repo to docket-mode** (a separate `docket-new-change` → `docket-implement-next` cycle, explicitly out of scope for 0002). When ready: remove the `metadata_branch: main` pin from `.docket.yml` and run `migrate-to-docket.sh`. This is the first real-repo exercise of the migration path.
- **Deferred minor (reviewer):** the board-path handling is asymmetric — `migrate-to-docket.sh` tolerates a repo-root `BOARD.md` fallback while the §7.0 guard hard-pins `<changes_dir>/BOARD.md`. Purely theoretical (the layout fixes the board under `changes_dir`); only worth reconciling if a repo-root board layout ever becomes supported.
