<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0496 — Add `docket repository repair` and stop flagging empty claimed_at](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-03-0496-drop-final-claimed-at-is-reported-repairable-but-nothing-can.md)**
<!-- docket:backlink:end -->
# Add `docket repository repair` and stop flagging empty claimed_at — Results

**Human action:** Nothing is required to merge. After merge, one optional check confirms that `repository check` is clean on this repository and that the new repair command only previews.

## Outcome

`docket repository check` used to report 269 `drop-final-claimed-at` warnings. Every archived change record carries an empty `claimed_at:` key, which is docket's normal "no claim" form, but the check's repair planner treated the bare key as a leftover claim stamp. Because any finding makes `check` exit 1, the command could never come back clean. The planner now decides on the parsed value: an empty key (bare, `''`, `""`, `null`, `~`) is not a finding. A real timestamp or a malformed value is still reported. The 269 warnings go away with no record rewritten.

The remedy those warnings gave also pointed nowhere. On an already-migrated repository, `migrate` applied no frontmatter repairs, and its derived-view repair (board, `## Artifacts` blocks, ADR index) sat behind a flag named for frontmatter. This change adds `docket repository repair` (catalog operation `repository.repair`):

- Without `--yes` it previews every mechanically repairable finding `check` reports, both frontmatter fixes and derived-view re-renders. Anything that needs a person is listed as manual review.
- With `--yes` it applies frontmatter fixes first, then re-renders derived views from the repaired records. Everything lands in one commit on `docket` (subject `docket: repository repair`), published under an exact lease on the previewed tip. If the tip moved since the preview, the result is `contended` and nothing is overwritten.
- A repository that has not been migrated is refused, with a pointer to `docket repository migrate`.

`migrate` now only migrates. On a migrated repository it writes nothing, whatever flags are passed, and prints `repository already migrated; for mechanical repairs run docket repository repair`. Remedies in `check` and `status`, and the docket-adr, docket-status, and docket-convention skills, now name the new command; skills use the catalog id.

## Human actions and testing

### Optional — confirm the false alarm is gone and repair only previews

Why: it shows on the real repository that the 269 warnings are gone, and that the new command writes nothing unless `--yes` is passed.

Prerequisite: this change is merged and the installed `docket` binary is rebuilt from `main`. Work from the primary checkout, `/Users/homer/dev/docket`.

1. Run `docket repository check --repo-dir /Users/homer/dev/docket`.
   Expected: no `drop-final-claimed-at` lines. If the repository has no other findings, the command exits 0.
2. Run `docket repository repair --repo-dir /Users/homer/dev/docket` (without `--yes`).
   Expected: a preview of any repairable findings, or a no-op message. `git -C .docket log -1 origin/docket` shows the same commit before and after.
3. Run `docket repository migrate --repo-dir /Users/homer/dev/docket --repair-frontmatter`.
   Expected: `repository already migrated; for mechanical repairs run docket repository repair`, and nothing is written.

## Verification performed

- Each of the seven plan tasks ran its own focused tests and mutation checks. A new real-git integration shard, `tests/test_go_integration_app_reporepair.sh`, covers seven repair scenarios.
- The full suite (`go run ./cmd/docket development test`) passed at the build head. The budget report had screening lines (`PARALLEL-SENSITIVE`, `BUDGET WATCH`) for existing shards only, none for this change's shard, and no serial-confirmed breach.
- A whole-branch deep review returned two findings, both fixed in-branch:
  - The docket-adr skill told agents to run the repair with `--yes`. It now previews only, and a human authorizes the apply.
  - The repair planner had its own copy of the decoder's empty-value rule. Both now call one shared helper, `document.EmptyValue`, and every clause of it is mutation-tested.
- The final gate ran again at the head that contains these results; see the build-evidence block in the PR.
- ADR-0136 records the decision.

## Known issues and follow-ups

### The committed-ignore remedy still names `migrate`

When the committed `.gitignore` entry docket expects is missing, `repository check` reports a `committed-ignore-invalid` finding whose remedy says to re-run `docket repository migrate`. On a migrated repository `migrate` no longer writes anything, so following that hint does not restore the entry. This finding is not mechanically repairable, so `repository repair` does not cover it either. Confirmed by reading the code; low impact. Workaround: add the ignore entry by hand. Suggested next action: reword that remedy in a small follow-up change.
