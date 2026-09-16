<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0413 — Finalize rebase gate cannot clear derived embedded-asset manifest collisions](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0413-finalize-rebase-gate-cannot-clear-derived-embedded-asset-man.md)**
<!-- docket:backlink:end -->

# Change 0413 — Regenerate embedded assets during finalize rebase

## Necessity

Still necessary, checked against main `0842c1900c8b3cc1b87cfd6cdc80f6d77f5b3bf5` on 2026-09-16. The stub's fixed two-dispatch premise is obsolete: change 0349 made admission durable/configurable, and 0419 raised the default to 10. Neither distinguishes generated conflicts. `mapBegunRebase` and `mapContinuedRebase` still send every conflict through resolver admission; the resolver edits hunks and cannot regenerate the bundle under its current charter.

An isolated Git fixture using the current `cmd/genassets`, one independent main-side authored edit, and 11 feature commits produced 11 successive conflicts solely in `internal/assets/embedded/manifest.json`. Regenerating at each stop completed the rebase, preserved both authored edits, and passed the generator's `-check`. This reproduces the collision; budget exhaustion follows from the current admission code, rather than from an end-to-end finalize run.

## Fix

Add one narrowly scoped generated-bundle path to the existing Go finalize rebase controller.

- Recognize only Docket's own embedded bundle: `internal/assets/embedded/manifest.json` and its `tree/` payload. Require the Docket module identity and existing generator contract in the feature workspace; a similarly named file in another repository is not eligible. No generic generated-file detection.
- Before returning a conflict or applying resolver-budget exhaustion, inspect the live unmerged set. When only eligible bundle outputs remain, regenerate with the feature workspace's existing `cmd/genassets` from that stopped commit's merged authored roots. Stage only the resulting bundle changes, including deletions, and continue through the existing Git rebase primitives. Repeat only as Git advances to subsequent stopped commits.
- For mixed authored/generated conflicts, retain normal reserved resolver dispatch for the authored decisions. Have the resolver leave bundle outputs for the controller. After validating the resolver report and resolved authored inputs, regenerate the bundle before staging and continuing. Never generate from unresolved authored inputs or hand-merge hashes.
- Generated-only stops allocate no resolver reservation and consume no resolver budget, including after the last permitted authored resolution. Real resolver reservations remain charged exactly once and are never refunded.
- Keep ownership checks, the workspace operation lock, and interruption handling in the existing rebase flow/receipt. Do not fabricate resolver reports or reservation tokens for deterministic work. Re-entry must inspect the stopped commit and Git state before another mutation; ambiguous progress or generation failure blocks through the existing failure path. Do not loop on an unchanged stopped commit.
- Preserve the post-rebase whole-suite gate and publishing checks. Regeneration is conflict resolution, not permission to merge or bypass validation.

## Scope and validation

Primary edits belong in the existing finalize rebase code, focused tests, and the resolver/finalize instructions needed for the mixed-conflict handoff; regenerate their shipped assets normally. Reuse the existing generator unchanged. No generator registry, hooks framework, new public command/configuration, separate state store, squashing, blanket budget increase, or unrelated finalize repair.

Add a real-Git regression with more generated-only stops than the configured resolver limit: it must finish with zero reservations, preserved authored edits, and a clean bundle drift check. Cover mixed conflicts charging only authored dispatches (including a final allowed dispatch followed by generated-only stops), generation failure without continuation, interrupted re-entry without duplicate continuation, and ineligible repositories remaining on the normal path. Reverting the fast path must make the regression fail. At implementation time, run the full configured build/finalize suite through the source runner and inspect its budget report.
