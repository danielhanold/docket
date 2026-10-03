<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0496 — Add `docket repository repair` and stop flagging empty claimed_at](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0496-drop-final-claimed-at-is-reported-repairable-but-nothing-can.md)**
<!-- docket:backlink:end -->
# Add `docket repository repair` and stop flagging empty claimed_at — Results

**Human action:** Assessment pending until review finishes. Expect one optional check: run the new repair command on this repository after merge.

## Outcome

`docket repository check` used to report 269 `drop-final-claimed-at` warnings. Every archived change record carries an empty `claimed_at:` key, which is docket's normal "no claim" form, but the check's repair planner treated the bare key as a leftover claim stamp. Because any finding makes `check` exit 1, the command could never come back clean. The planner now decides on the parsed value: an empty key (bare, `''`, `""`, `null`, `~`) is not a finding. A real timestamp or a malformed value is still reported. The 269 warnings go away with no record rewritten.

The remedy those warnings gave also pointed nowhere. On an already-migrated repository, `migrate` applied no frontmatter repairs, and its derived-view repair (board, `## Artifacts` blocks, ADR index) sat behind a flag named for frontmatter. This change adds `docket repository repair` (catalog operation `repository.repair`):

- Without `--yes` it previews every mechanically repairable finding `check` reports, both frontmatter fixes and derived-view re-renders. Anything that needs a person is listed as manual review.
- With `--yes` it applies frontmatter fixes first, then re-renders derived views from the repaired records. Everything lands in one commit on `docket` (subject `docket: repository repair`), published under an exact lease on the previewed tip. If the tip moved since the preview, the result is `contended` and nothing is overwritten.
- A repository that has not been migrated is refused, with a pointer to `docket repository migrate`.

`migrate` now only migrates. On a migrated repository it writes nothing, whatever flags are passed, and prints `repository already migrated; for mechanical repairs run docket repository repair`. Remedies in `check` and `status`, and the docket-adr, docket-status, and docket-convention skills, now name the new command; skills use the catalog id.

## Known issues and follow-ups

### The committed-ignore remedy still names `migrate`

When the committed `.gitignore` entry docket expects is missing, `repository check` reports a `committed-ignore-invalid` finding whose remedy says to re-run `docket repository migrate`. On a migrated repository `migrate` no longer writes anything, so following that hint does not restore the entry. This finding is not mechanically repairable, so `repository repair` does not cover it either. Confirmed by reading the code; low impact. Workaround: add the ignore entry by hand. Suggested next action: reword that remedy in a small follow-up change.
