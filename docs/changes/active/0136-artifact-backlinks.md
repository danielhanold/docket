---
id: 136
slug: artifact-backlinks
title: Artifact back-links — a generated link at the top of every artifact pointing to the change
status: implemented
priority: medium
type: feat
created: 2026-07-23
updated: 2026-07-24
depends_on: []
related: [35]
discovered_from: []
adrs: [12]
spec: docs/superpowers/specs/2026-07-23-artifact-backlinks-design.md
plan: docs/superpowers/plans/2026-07-24-artifact-backlinks.md
results: docs/results/2026-07-24-artifact-backlinks-results.md
trivial: false
auto_groomable:
branch: feat/artifact-backlinks
claimed_at: 2026-07-24T05:04:09Z
pr: https://github.com/danielhanold/docket/pull/124
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-23-artifact-backlinks-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-23-artifact-backlinks-design.md) |
| Plan | [2026-07-24-artifact-backlinks.md](https://github.com/danielhanold/docket/blob/feat/artifact-backlinks/docs/superpowers/plans/2026-07-24-artifact-backlinks.md) |
| Results | [2026-07-24-artifact-backlinks-results.md](https://github.com/danielhanold/docket/blob/feat/artifact-backlinks/docs/results/2026-07-24-artifact-backlinks-results.md) |
| PR | [#124](https://github.com/danielhanold/docket/pull/124) |
| ADRs | [ADR-0012](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0012-docket-status-script-vs-model-boundary.md) |
<!-- docket:artifacts:end -->

## Why

Change 0035 made the change file a hub that links **out** to its spec, plan, results, ADRs, and PR
via a generated `## Artifacts` block. But there is no link **back**: a reviewer reading a spec, a
plan, a results file, or a PR has no clickable path home to the change that owns it, and must
hand-search the backlog or the board. At every human review stop — spec review, the merge gate,
results review — this friction repeats. The forward direction is solved; the reciprocal is the
missing half.

## What changes

Add the reciprocal of 0035: a generated, marker-bounded **back-link block** at the very top of each
artifact docket touches, pointing to the change file. It is the mirror image — same sole-writer /
script-vs-model discipline (ADR-0012), same GitHub-blob-or-bare-path rendering, same
frontmatter-is-truth stance.

- A new deterministic renderer, `render-artifact-backlink.sh`, stamps a `docket:backlink` block at
  the top of an artifact, built from the change's frontmatter. It is the block's sole writer; skills
  never hand-edit it.
- **Uniform target:** every back-link points to the change on `metadata_branch` at its current
  canonical path (`active/…` while live, `archive/…` once terminal), so `terminal_publish` changes
  only whether the close-out re-render fires, never the link target.
- **Scope:** spec, plan, results, and the PR body. The two superpowers-authored artifacts (spec,
  plan) are included via a docket post-write stamp — docket never patches the vendored superpowers
  skills. ADRs are excluded (already back-referenced by `change:` frontmatter + the index).
- **Durability tiers by where the artifact lives:** spec (on docket) and the PR body (via `gh`) are
  always durable — re-rendered at close-out. plan/results (on the code line) are made durable by
  folding their re-render into `terminal-publish.sh`'s existing integration-branch commit when
  `terminal_publish: true` — no additional commit. When `terminal_publish: false`, plan/results are
  stamped once at creation and accepted to go stale after archive.

Full design — the block format, the renderer contract, all call sites, the terminal-publish
extension, and the testing approach — is in the linked spec.

## Out of scope

- A one-time back-fill pass over artifacts of already-terminal changes (the block appears naturally
  on the next relevant write).
- ADR body back-links (already covered).
- Durable plan/results back-links under `terminal_publish: false` (deliberately stamp-once).
- Any change to BOARD.md or the forward `## Artifacts` block.
- URL schemes beyond GitHub blob + bare-path fallback.

## Open questions

None outstanding — mechanism, uniform target, durability tiering, the terminal-publish fold-in, and
the `terminal_publish: false` behavior are all resolved in the spec. One presentation call (the
exact back-link text/glyph) is recorded there and open at build time.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-07-24 — reconcile before build

Re-validated the design against current `main`. Every dependency the spec rests on is present and
matches the spec's description:

- `scripts/render-change-links.sh` (change 0035, the forward-block renderer this change mirrors)
  exists with the sole-writer / marker-block / GitHub-blob-or-bare-path idioms the new renderer will
  reuse (`field`/`list_field` from `lib/docket-frontmatter.sh`, `blob()`, the awk replace-vs-insert
  split). Change 0035 is `done` (`archive/2026-06-21-0035-artifact-links.md`).
- `scripts/terminal-publish.sh` still provisions one transient integration-branch worktree (`pub`)
  and makes exactly one publish commit under `--enabled true`, gated by both the mode guard and the
  `terminal_publish` knob (changes 0064/0084) — the single commit the plan/results re-stamp folds
  into, exactly as decision 5 assumes. Recent work on this file (0083 marker-clear, 0084 loud-no-op,
  0064 opt-in) is consistent with the spec and does not disturb the fold-in point.
- `skills/docket-convention/references/terminal-close-out.md` carries the archive → re-render →
  publish → cleanup → board sequence the spec extends (spec re-render in step 2, plan/results
  re-render inside terminal-publish in step 3, PR-body re-render best-effort).
- Call sites named by the spec are present: `docket-new-change §2` and the kill close-out already
  call `render-change-links` after a spec write; `docket-implement-next §4/§7`, `docket-groom-next`,
  and `docket-auto-groom` exist as described. ADR-0012 (script-vs-model boundary) is Accepted.

No scope change. Body and spec are accurate as written; no work has been done elsewhere. No distinct
follow-up work surfaced that meets the auto-capture materiality bar — the spec's out-of-scope items
(one-time back-fill, ADR body back-links, durable plan/results under `terminal_publish: false`) are
deliberate non-goals already recorded, not newly discovered work. The one open presentation call
(exact back-link text/glyph, `↩ Change NNNN — <title>`) is resolved at build time.
