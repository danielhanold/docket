<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0512 — Release v1.0.0-alpha.2: prove and publish Cursor support](../../changes/archive/2026-10-08-0512-release-v1-0-0-alpha-2-prove-and-publish-cursor-support.md)**
<!-- docket:backlink:end -->

# v1.0.0-alpha.2 acceptance and publication (Cursor) — plan

This change is a human-attended release protocol, not a build. It is run by the operator in an
attended session and never by `docket-implement-next`.

The plan is the spec's protocol, executed in order:
`docs/superpowers/specs/2026-10-08-release-v1-0-0-alpha-2-prove-and-publish-cursor-support-design.md`.

1. Phase 0 — before the cut (done: 0543 merged, Cursor isolation dry run passed, no loops, no open PRs).
2. Phase 1 — cut the candidate at `ec4c2b1841954c2d6a3dd018e1133ee762861137`, claim, reconcile, attach this plan.
3. Phase 2 — package once with the release-candidate workflow; verify `evidence.json` and checksums exactly.
4. Phase 3 — the Cursor lifecycle in an isolated test home, with the kill and resume; record `harness/cursor.md`.
5. Phase 4 — publish `v1.0.0-alpha.2` as a pre-release at the human's explicit "publish".
6. Phase 5 — public install check with `--harness cursor`.
7. Phase 6 — closeout: gate, evidence, results, PR, then mark implemented, then finalize.

The evidence bundle lands under `docs/release/v1.0.0-alpha.2/` on this change's feature branch.
