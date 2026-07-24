<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0136 — Artifact back-links — a generated link at the top of every artifact pointing to the change](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0136-artifact-backlinks.md)**
<!-- docket:backlink:end -->

# Artifact back-links — results
Change: #136 · Branch: feat/artifact-backlinks · PR: <opened at close-out> · Plan: docs/superpowers/plans/2026-07-24-artifact-backlinks.md · ADRs: none

## Verify (human)

Automated tests cover the mechanics (whole suite green, 62/62). The items below are the real-runtime
behaviors the hermetic suite cannot fully exercise (LEARNINGS `metadata-branch-invisible-to-suite`),
plus the interactive back-link stamps that only fire in a live skill run:

- [ ] After merge, confirm the published **spec** on `docket` and (under `terminal_publish: true`) the
      **plan/results** on `main` carry a `docket:backlink` block pointing at the archived change path
      (`docs/changes/archive/…-0136-…md`) — the close-out re-render (terminal-close-out step 2 + the
      terminal-publish fold-in).
- [ ] On the next real `docket-new-change` / `docket-groom-next` run, confirm the spec is stamped with
      a `docket:backlink` block in the same spec-write commit.
- [ ] Confirm the `docket-implement-next` PR-body back-link line renders (skill-side; no automated
      coverage — the sentinel only checks the instruction is present, not that a PR carries it).

## Findings

- **No ADRs.** Every design decision (docket post-write stamp, uniform `metadata_branch` target,
  durability tiering, the terminal-publish fold-in, `terminal_publish: false` stamp-once) was already
  settled in the spec; the build followed it faithfully, so nothing rose to an architecture decision.
- **Presentation call resolved (spec left it open).** The block body is
  `> ↩ **[Change NNNN — <title>](<url>)**` in GitHub mode and
  `> ↩ **Change NNNN — <title>** — \`<relpath>\`` in bare-path fallback.
- **Real-artifact dogfood.** The renderer was run against the real change file on the metadata branch
  to stamp this change's own **results** back-link (see the block at the top of this file) — a live
  check beyond the hermetic fixtures.
- **Self-referential-artifact edge (discovered by the dogfood; reported, not filed).** The renderer
  replaces **every** `docket:backlink` marker region it finds (the same awk replace-vs-insert as
  `render-change-links.sh`). An artifact whose *body* embeds the literal marker strings — as **this
  change's own plan does**, because its subject matter is the block format (Task 1's block-shape
  example and the embedded test goldens) — therefore has those regions overwritten instead of a block
  inserted at the top. So the plan for 0136 is **intentionally left unstamped**. This is a narrow edge:
  a normal spec/plan/results never contains the literal marker comments, and the sibling
  `render-change-links.sh` has the identical behavior without issue (change files are docket-controlled).
  Left as report-only rather than an auto-captured stub — hardening it (e.g. a top-of-file positional
  heuristic) is speculative for real artifacts and would implicate both renderers; a human can file it
  at the merge gate if wanted.
- **Minor (reported, not filed).** `render-artifact-backlink.sh` mirrors `render-change-links.sh`'s
  in-place `mv` without an explicit awk-exit / `-s` non-emptiness guard before the `mv`
  (LEARNINGS `atomic-generated-write`). This is the sibling's established, proven idiom (awk over a
  valid block file + an existing artifact does not fail-empty), so it was kept behavior-consistent
  rather than diverged; a hardening pass would belong to both renderers together, not this change.

## Follow-ups

- **Build-method degradation (notable plan deviation).** This run's runtime had no subagent-dispatch
  (Task) tool, so `superpowers:subagent-driven-development` could not dispatch fresh implementer/
  reviewer subagents. The plan was executed inline with SDD's TDD discipline (test-first → verify-fail
  → implement → verify-pass → commit per task) and reviewed as a whole-branch self-review — the
  Skill-layer missing-skill fallback. No behavioral impact on the deliverable; noted for the reviewer.
- The one-time back-fill over already-terminal changes' artifacts stays deliberately out of scope
  (spec); the block appears naturally on the next relevant write.
