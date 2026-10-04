<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0515 — Retire the finalize repair sign-off so a green repair merges](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-04-0515-make-finalize-merge-honor-the-repair-sign-off-block-when-an.md)**
<!-- docket:backlink:end -->
# Retire the finalize repair sign-off so a green repair merges — Results

**Human action:** None required before merge. After it merges and the binary is reinstalled, restart running Claude sessions before the next finalize run so no run mixes the old and new finalize contracts.

## Outcome

Before this change, when finalize's integration repair turned a red rebased suite green, the finalize skill still refused to merge: an autonomous run recorded a `repair-needs-signoff` block and halted until a human ran `finalize clear-block`, and an attended run prompted for a go-ahead. That rule lived only in skill text; the binary never enforced it, and the docs also promised that auto-detect skipped any change carrying a `## Finalize blocked` section, which the binary never did either.

Now a repair that turns the rebased suite green publishes and merges like any other green change, on autonomous and attended runs alike. The repair stays visible: the finalize run's report names what broke and the repair commits, and the archived record's `## Closeout notes` keeps the same facts (a refused note never stops closeout). Approval stays the repository's own policy: if branch protection requires approvals and dismisses stale approvals on new commits, the repair push removes the approval and the merge waits for a fresh one.

`## Finalize blocked` is now a visible note only. The binary's never-active blocked-note code is removed (`finalizeBlockedMap`, the `finalize-blocked` selection skip, and the merge's marker term), so a note can never stop selection or merge, and the next finalize run retries the change. `finalize block` and `finalize clear-block` themselves are unchanged.

If a finalize run publishes a repair and then halts before merging (for example a denied merge), its `## Finalize blocked` note names the repair with an `Authored repair:` remedy, and the later run that merges carries that into the closeout notes, so the repair is never lost from the archived record. If closeout refuses a notes request, the run retries once without notes and its report names what was dropped.

ADR-0139 ("Finalize adds no human gate of its own") records the decision; ADR-0010 and ADR-0008 carry dated update notes pointing to it.

Beyond the spec's file list, the build also corrected the same stale claims in `skills/docket-convention/SKILL.md`, `agents/docket-rebase-resolver.md` (and its generated goldens), `docs/install/models-and-effort.md`, and `docs/comparison/ai-native-sdlc-playbook.md`.

## Verification performed

- Each build task ran its focused tests with mutation probes: reintroducing a `HasFinalizeBlocked` skip in selection, or a note refusal in `finalize.merge`, turns the new tests red; dropping the closeout notes from the end-to-end repair test turns it red; restoring the old step 6 sign-off prose turns the new `align_0515` prose sentinel red.
- `TestE2EConflictAndRepair` now drives conflict, resolver, red suite, repair, green re-gate, evidence, publish, merge, and closeout with no `finalize block` or `clear-block`.
- The embedded asset bundle and all four harness goldens were regenerated.
- A new guard walks every file under `skills/` and `agents/` and fails on any `repair-needs-signoff` occurrence (mutation-tested).
- A deep-tier whole-branch review returned 1 important and 6 minor findings; all 7 were fixed in-branch (cross-run repair visibility, the dropped-notes report, two guard gaps, a stale test comment, a wrong repair-budget figure in the comparison doc, and guidance for a persistently blocked change).

## Known issues and follow-ups

- `internal/install/legacydata/docket-rebase-resolver.md` still mentions the "auto-repair sign-off". It is a frozen legacy install fixture, so it was deliberately left alone; it describes an old installed file, not current behavior. No action suggested.
- The rule that carries an `Authored repair:` note into a later closeout is skill prose; no binary check enforces that an agent follows it. Suggested next action: none unless a missed note is observed.
