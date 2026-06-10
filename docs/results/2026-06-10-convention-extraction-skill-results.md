# Extract the shared convention into a docket-convention skill — results
Change: #5 · Branch: feat/convention-extraction-skill · PR: see `pr:` in the change manifest · Plan: docs/superpowers/plans/2026-06-10-convention-extraction-skill.md · ADRs: 3

## Verify (human)

Automated coverage is green (all 5 test files pass; `test_convention_extraction.sh` alone carries 93 asserts, both directions). These need a human eye at the merge gate:

- [x] **Fresh-session load check (spec §8):** VERIFIED 2026-06-10 (pre-merge, via temporary symlinks pointing ~/.claude/skills at this branch's worktree). Fresh session, `/docket-status`: transcript shows `Skill(docket-status)` immediately followed by `Skill(docket-convention)`, before any metadata read — the blocking Step-0 load held with nothing pre-loaded in context.
- [ ] Same check on `docket-new-change`'s trivial path (any throwaway idea; kill it after).
- [ ] Skim `skills/docket-convention/SKILL.md` once end-to-end — it is now the single copy of the contract; a transcription defect here propagates everywhere (the build verified byte-fidelity by diff, but the human gate is the right place for a sanity read).

## Findings

- **A pre-existing convention restatement was hiding outside the markers.** `docket-implement-next` Step 1 restated the full Build-readiness & selection definition in its own words ("satisfied = `done`" instead of the convention's "satisfied when it reaches `done`") — which is exactly why the sentinel sweep missed it. Found by the whole-branch review, removed in `3a37d75`; Step 1 now references the convention's definition and keeps only its operational additions. Recorded as the "accepted gap" consequence in ADR-0003: the sentinel tripwire is sampling, not parsing.
- **Sentinel coverage extended 7 → 10 at build time** (added `zero-padded to 4 digits`, `PM-altitude proposal`, `must never trail the change files` — manifest/body/board-rule sections were uncovered). All ten verified collision-free against the slimmed skills.
- **Three older change-guard tests grepped the five skills for convention content** (results: field, metadata_branch default, board-refresh rule…). The plan only anticipated removing their sync-check asserts; the build also repointed those content asserts to the single source, preserving each test's guard intent (`9fe6002`).
- **YAML constraint on the settled description wording:** the spec's frontmatter description contained a `: ` which is invalid inside an unquoted YAML scalar; one colon became an em-dash (noted in spec §3).

## Follow-ups

- When a future change adds or reworks a convention section, add a collision-checked sentinel for it in `tests/test_convention_extraction.sh` (the reference-never-restate rule's tripwire only covers what it samples — ADR-0003).
