<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0580 — Release v1.0.0: stable release with version-free install docs](../../changes/active/0580-release-v1-0-0-stable-release-with-version-free-install-docs.md)**
<!-- docket:backlink:end -->

# Release v1.0.0 — Plan

This change is run by hand in an attended session, following the spec's protocol. This file exists because `change.mark-implemented` requires a linked plan; the evidence for each phase is in `docs/release/v1.0.0/`.

1. **Phase 0 — Docs to `main`.** Version-free install docs merged before the cut; pre-cut state recorded.
2. **Phase 1 — Cut the candidate.** Candidate = `origin/main`; claim, workspace, reconcile, this plan; freeze rule from here on.
3. **Phase 2 — Package once.** `release-candidate.yml` at the candidate as `v1.0.0`; verify evidence and checksums; read-only copy.
4. **Phase 3 — Coverage statement.** No harness test, no public-install check (human decision).
5. **Phase 4 — Publish.** Notes approved, human's "publish"; tag, draft release, assets, publish as Latest, latest-link check.
6. **Phase 6 — Closeout.** Evidence bundle, build gate, PR, mark implemented, finalize, binary rebuild, status.
