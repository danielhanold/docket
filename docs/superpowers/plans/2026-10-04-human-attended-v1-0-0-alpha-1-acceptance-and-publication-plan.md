<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0366 — v1.0.0-alpha.1 acceptance and publication (Claude Code)](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0366-human-attended-v1-0-0-rc1-acceptance-and-publication.md)**
<!-- docket:backlink:end -->
# v1.0.0-alpha.1 acceptance and publication — Plan

This change has no code to plan or build. Its plan is the human-attended protocol in the spec (`docs/superpowers/specs/2026-09-04-human-attended-v1-0-0-rc1-acceptance-and-publication-design.md` on the `docket` branch), executed phase by phase with the human present:

1. Phase 0 — before the cut: record dependency, open-PR, in-progress and no-loops decisions.
2. Phase 1 — cut the candidate from `origin/main`, claim 0366, freeze `main`.
3. Phase 2 — dispatch `release-candidate.yml` once; verify the evidence and checksums; keep one read-only copy.
4. Phase 3 — the Claude Code lifecycle in a throwaway test home, with a kill and resume; check the terminal predicate.
5. Phase 4 — at the human's explicit "publish": tag, draft, upload the six files, verify, publish as a pre-release not marked latest.
6. Phase 5 — the public install check from the real release URL.
7. Phase 6 — evidence bundle, results, build gate, PR, mark implemented, finalize.

This file exists because `change.mark-implemented` requires a linked plan. It was written during Phase 6, after Phases 0–5 had run. The evidence for each phase is in `docs/release/v1.0.0-alpha.1/`.

The build gate for the closeout PR runs on the head that carries this plan.
