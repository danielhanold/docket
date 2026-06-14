---
id: 11
slug: github-issues-board-mirror
title: GitHub board mirror — selectable board surfaces, one-way Issues + Projects mirror
status: in-progress
priority: medium
created: 2026-06-12
updated: 2026-06-14
depends_on: []
related: [4, 10]
adrs: []
spec: docs/superpowers/specs/2026-06-14-github-issues-board-mirror-design.md
plan:
results:
trivial: false
branch: feat/github-issues-board-mirror
pr:
blocked_by:
reconciled: true
---

## Why

Follow-up to the AgentRQ competitive review (2026-06-11) and the docket-vs-GitHub-Issues
comparison (2026-06-12). The boundary rule: git-native agent mechanics stay in docket;
human-facing visibility belongs on GitHub's surface. The board is pure visibility — and the
status it surfaces worst in markdown (`implemented` — needs your merge, the human's one job)
is the one GitHub surfaces best: an open issue with a linked PR in an "awaiting merge" view.

The brainstorm widened this from "BOARD.md, optionally mirrored" to a cleaner model: the board
is a **derived view rendered on zero or more selectable surfaces**. The change files (+ git)
are always the source of truth; every surface — `inline` included — is regenerated output,
never read back. A repo picks the offline-safe inline board, the GitHub mirror, both, or none.

## What changes

- **`board_surfaces` knob in `.docket.yml`** — a list selecting which derived board views to
  render: `inline` (BOARD.md, offline-safe) and/or `github` (Issues + Projects mirror). Default
  `[inline]` (backward-compatible; GitHub strictly opt-in). `[]` disables the board entirely —
  git history is then the only record, still fully authoritative. Unknown tokens warn-and-ignore;
  a non-GitHub remote silently drops `github`.
- **The Board pass (`docket-status`) becomes render-each-enabled-surface.** The existing
  "regenerate the board on every status write" invariant generalizes to "refresh each enabled
  surface"; "offline-safe canonical view" becomes a property of the `inline` surface.
- **The `github` surface — one-way Issues + Projects v2 mirror**, best-effort (needs network +
  auth, never aborts a build, self-heals next pass), additive:
  - One issue per change, upserted via a new per-change `issue:` frontmatter field (shape of
    `pr:`) minted on first sync. State + close reason map all seven statuses
    (`done`→completed, `killed`→not-planned; active states stay open).
  - A Projects v2 board, **auto-created if not configured** — private, under the repo owner,
    with a Status single-select field; its `{owner, number}` is written back to `.docket.yml`
    (`github_project:`) for idempotency. Projects is the optional half: missing `project` token
    scope or any GraphQL failure → skip Projects, still mirror Issues + labels.
  - Issue body is a visibility pointer — one-way banner, a frontmatter digest, `## Why` distilled
    to a sentence or two, and **hrefs to every relevant artifact** (change file on `docket`, spec,
    each ADR, plus plan/results once they exist) — never the full body (no source-of-truth dup).
  - Labels live under a **`docket:` namespace** (`docket:status/*`, `docket:priority/*`,
    `docket:waiting/*`, `docket:readiness/*`); docket touches only labels it minted.
  - **Closing is sync-owned.** The PR *references* the issue (linked-PR awaiting-merge view) but
    never `Closes #N`; the Board-pass sync stays the sole writer of open/closed state and reason.
  - **Implemented as a deterministic script** (`scripts/github-mirror.sh`) the Board pass invokes,
    not agent-constructed `gh`/GraphQL calls — idempotent external writes need reproducibility and
    testable command construction (the `inline` surface stays agent-prose).
- **Convention additions (`docket-convention`)** — `board_surfaces`, `github_project`, the
  `issue:` field, the status→issue/close-reason mapping, the `docket:` label namespace, the
  one-way rule, and the generalized "each enabled surface" board invariants.

## Out of scope

- Two-way sync of any kind — no reading comments, assignments, or label edits back from GitHub.
- Mirroring the Mermaid dependency graph onto native sub-issue/dependency relationships
  (write-heavy, least-consulted board element; possible later change).
- Per-day timeseries / charts on the GitHub surface (0010's analytics territory).
- Auto-creating the project as public or under a non-owner account — private, repo owner only.

## Reconcile log

- 2026-06-14 — Reconciled at claim. Spec is hours old (same-day groom); no changes shipped
  since, `origin/main` unchanged, so no scope drift to fold in. Verified against current code:
  (1) no `scripts/` dir exists yet — `scripts/github-mirror.sh` is net-new; (2) the test harness
  is **sentinel-grep** against skill prose + scripts (`tests/*.sh`, `assert` = `grep`), so the
  mirror's test asserts the script's **command construction against a mocked `gh`**, not live
  GitHub effects (matches the spec §4.4 / the LEARNINGS note on metadata-branch artifacts);
  (3) the existing `test_board_refresh_on_transition.sh` pins the convention heading
  **"Board refresh on status writes"** and a `>=3` count of best-effort Board-pass clauses in
  `docket-implement-next` — the generalization to "each enabled surface" must PRESERVE that
  heading, and the PR→issue touch-up must not reduce the board-pass clause count. No design
  change; proceeding to plan.
