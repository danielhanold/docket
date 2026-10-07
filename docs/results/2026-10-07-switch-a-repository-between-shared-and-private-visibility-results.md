<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0533 — Switch a repository between shared and private visibility](../changes/active/0533-switch-a-repository-between-shared-and-private-visibility.md)**
<!-- docket:backlink:end -->

# Switch a repository between shared and private visibility — Results

**Human action:** Yes. Review the PR, try one round-trip switch on a throwaway clone before relying on it, and after the merge run `docket repository repair` in this repository to re-stamp about 286 absolute spec backlinks.

## Outcome

A repository's visibility used to be fixed at `init`. There is now one supported way to move an existing repository between modes:

```
docket repository set-visibility <shared|private> [--yes] [--metadata-remote <url>] [--delete-shared-branch] [--remove-shared-files] [--repo-dir <dir>]
```

- Without `--yes` it prints a preview: each phase that will run, each flag's effect, any integration-branch commit with its exact message and file list, and warnings. `--yes` applies it, pinned to what the preview saw. A re-run resumes from the first unfinished phase. It refuses while any run, gate drive, or worktree lock is live.
- The metadata history is published unchanged under the other branch name (`docket` on origin, `dckt` on the local bare remote), so every record, claim receipt, and idempotency replay survives. This is recorded as ADR-0147.
- Integration-branch edits are committed locally, never pushed, under exactly `Remove docket configurations from repository` or `Add docket configurations to repository`.
- Switching to the mode a repository already has only aligns the local `visibility` value.
- `docket repository check` now reports, and `docket repository repair` now fixes, spec, plan, and results files whose backlink to their change is an absolute GitHub link instead of a relative one. The switch preview warns about such links but never refuses.

Departures from the spec, all deliberate:

- Phases that touch remotes run before the state-folder rename. The spec's order would have blocked the rename.
- The metadata worktree is removed and re-attached rather than moved with `git worktree move`.
- Going private saves personal keys from `.docket.local.yml` as `.git/dckt/local-keys.yml`; going shared restores them.
- Going shared refuses only when origin's `docket` branch holds unrelated or diverged history. A branch left behind by an earlier switch is fast-forwarded. As the spec read, a repository switched private without `--delete-shared-branch` could never switch back.
- The identity-keys stop on the way to shared returns a needs-review refusal carrying the push remedy, not `applied`.
- Open PRs carrying docket text are listed only when going private. No private Cursor rule file exists in the current code, so going shared has nothing to remove there.

## Human actions and testing

### Important — Repair absolute spec backlinks after the merge

About 286 spec files on this repository's `docket` branch still link back to their change with an absolute `https://github.com/.../blob/docket/...` link. They work today but would break if this repository ever switched to private with `--delete-shared-branch`. Nothing breaks if you skip it, but `repository check` will report them as warnings until you do.

Prerequisites: this change merged, and the installed `docket` binary rebuilt from `main`.

1. Run `docket repository repair` in `/Users/homer/dev/docket`.
   Expected: a preview listing the artifact files it will re-stamp (about 286) plus any stale `## Artifacts` blocks, ending in a confirmation request.
2. Run `docket repository repair --yes`.
   Expected: one metadata commit pushed to `origin/docket`.
3. Run `docket repository check`.
   Expected: no `artifact-backlink-stale` findings.

### Important — Try one round trip on a throwaway clone

The switch moves real data (remotes, the state folder, the metadata worktree). The automated tests use fixture repositories; nothing has run against a real GitHub remote. Skip it and the first real use is the test.

Prerequisites: a scratch GitHub repository you can delete, initialized with `docket repository init` and with at least one change created.

1. `docket repository set-visibility private`.
   Expected: a preview naming each phase and warning that teammates' later writes to `origin/docket` will not be seen. Nothing changes on disk.
2. `docket repository set-visibility private --yes --remove-shared-files`.
   Expected: `docket status` still lists the change; `.git/dckt/` exists; the clone root has no `.docket/` folder and no `.docket.yml`; `git log -1` on the current branch shows `Remove docket configurations from repository`, not pushed.
3. `docket repository set-visibility shared --yes`.
   Expected: `docket status` still lists the change; `.docket/` is back; `git log -1` shows `Add docket configurations to repository`, not pushed; the output prints where the bare remote backup was kept.

Cleanup: delete the scratch repository and its bare remote folder printed in step 3.

## Verification performed

- Full suite (`go run ./cmd/docket development test`) green at the final head. Earlier: attempt 1 went red because the upgrade guide's finding table had no row for the new backlink finding; one guide row fixed it.
- The last suite run printed screening lines only (PARALLEL-SENSITIVE, BUDGET WATCH for `test_go_integration_release.sh` and `test_go_toolchain.sh`, SERIAL CONFIRMATION DUE for `test_go_finalize_e2e.sh`). None are serial-confirmed breaches, and none of those files changed on this branch.
- The new integration tests are split across three shards (`test_go_integration_app_repovisibility.sh`, `_private.sh`, `_shared.sh`), measured serially at 42s, 67s, and 61s, with budget rows 50, 75, and 70.
- On a throwaway clone of this repository, `repository check` reported 286 `artifact-backlink-stale` findings, and the `repository repair` preview listed them for re-stamping with no manual-review entries. Nothing was pushed.
- Workers mutation-tested the guards: the mode-vocabulary check on commit subjects, the dirty-path refusal, the exact-tip check before deleting origin's branch, the live-run refusal, and the repo-only key filter on the config fold.
- Whole-branch review (deep tier): 12 findings (1 blocker, 5 important, 6 minor), all fixed in-branch, and the ADR finding fixed by recording ADR-0147. Full table in the PR body.
- The backlog match for the follow-up below compared titles of all 24 proposed and deferred changes, not their full bodies.

## Known issues and follow-ups

### A crash in the middle of the final commit step can leave one file needing a hand cleanup

If the process dies inside the going-shared commit step, after an instructions file is written but before it is recorded for the commit, the next preview refuses with "commit or set aside these edits" and names that file. This is suspected from reading the code, not observed. Interrupting between steps (what the tests cover) cannot hit it. Workaround: `git stash` that file and re-run; the switch writes it again. Suggested next action: none unless it is seen in practice.

### A hand-formatted `.docket.yml` without a `visibility:` line gets reformatted

Going shared keeps the committed `.docket.yml` byte-for-byte only when it already declares every setting the switch writes. A file with no `visibility:` line is rewritten with `visibility: shared` added, which can also reformat it. Confirmed by design: leaving the key out would let a global config decide the mode. The diff shows up in the local `Add docket configurations to repository` commit for review before you push.

### The docket-status skill does not list the new backlink finding codes

The docket-status skill lists the derived-view drift codes it reports (`board-stale`, `adr-index-stale`, `artifact-links-stale` and their `-malformed` forms), but not the new `artifact-backlink-stale` / `artifact-backlink-malformed` / `artifact-backlink-shared` codes. Nothing fails because of it; an agent reading the skill just will not recognize them by name. Suggested next action: add the codes to the skill's list. No existing change fits (checked 24); capture a new change.
