<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0515 — Retire the finalize repair sign-off so a green repair merges](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0515-make-finalize-merge-honor-the-repair-sign-off-block-when-an.md)**
<!-- docket:backlink:end -->
# Retire the finalize repair sign-off so a green repair merges — Results

**Human action:** Assessment pending until review completes.

## Outcome

Before this change, when finalize's integration repair turned a red rebased suite green, the finalize skill still refused to merge: an autonomous run recorded a `repair-needs-signoff` block and halted until a human ran `finalize clear-block`, and an attended run prompted for a go-ahead. That rule lived only in skill text; the binary never enforced it, and the docs also promised that auto-detect skipped any change carrying a `## Finalize blocked` section, which the binary never did either.

Now a repair that turns the rebased suite green publishes and merges like any other green change, on autonomous and attended runs alike. The repair stays visible: the finalize run's report names what broke and the repair commits, and the archived record's `## Closeout notes` keeps the same facts (a refused note never stops closeout). Approval stays the repository's own policy: if branch protection requires approvals and dismisses stale approvals on new commits, the repair push removes the approval and the merge waits for a fresh one.

`## Finalize blocked` is now a visible note only. The binary's never-active blocked-note code is removed (`finalizeBlockedMap`, the `finalize-blocked` selection skip, and the merge's marker term), so a note can never stop selection or merge, and the next finalize run retries the change. `finalize block` and `finalize clear-block` themselves are unchanged.

ADR-0139 ("Finalize adds no human gate of its own") records the decision; ADR-0010 and ADR-0008 carry dated update notes pointing to it.

Beyond the spec's file list, the build also corrected the same stale claims in `skills/docket-convention/SKILL.md`, `agents/docket-rebase-resolver.md` (and its generated goldens), `docs/install/models-and-effort.md`, and `docs/comparison/ai-native-sdlc-playbook.md`.

## Verification performed

- Each build task ran its focused tests with mutation probes: reintroducing a `HasFinalizeBlocked` skip in selection, or a note refusal in `finalize.merge`, turns the new tests red; dropping the closeout notes from the end-to-end repair test turns it red; restoring the old step 6 sign-off prose turns the new `align_0515` prose sentinel red.
- `TestE2EConflictAndRepair` now drives conflict, resolver, red suite, repair, green re-gate, evidence, publish, merge, and closeout with no `finalize block` or `clear-block`.
- The embedded asset bundle and all four harness goldens were regenerated.

## Known issues and follow-ups

- `internal/install/legacydata/docket-rebase-resolver.md` still mentions the "auto-repair sign-off". It is a frozen legacy install fixture, so it was deliberately left alone; it describes an old installed file, not current behavior. No action suggested.
