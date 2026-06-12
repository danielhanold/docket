---
id: 11
slug: github-issues-board-mirror
title: GitHub Issues mirror of the board — one-way visual sync, change files stay source of truth
status: proposed
priority: medium
created: 2026-06-12
updated: 2026-06-12
depends_on: []
related: [4, 10]
adrs: []
spec:
plan:
results:
trivial: false
branch:
pr:
blocked_by:
reconciled: false
---

## Why

Follow-up to the AgentRQ competitive review (2026-06-11) and the docket-vs-GitHub-Issues
comparison (2026-06-12). The agreed boundary rule: git-native agent mechanics stay in docket;
human-facing visibility belongs on GitHub's surface. The board is pure visibility — and the
status it surfaces worst in markdown (`implemented` — needs your merge, the human's one job)
is the one GitHub surfaces best: an open issue with a linked PR in an "awaiting merge" column.

All seven statuses map cleanly. The terminal states match GitHub's native close reasons —
`done` → closed as **completed**, `killed` → closed as **not planned** — and the five active
states ride on `status:` labels (or a Projects v2 single-select Status column, which gives a
real kanban view). Derived annotations become labels too: `needs-brainstorm`,
`waiting: needs-your-merge`, `waiting: not-yet-built`, `priority: <p>`.

## What changes

- Mirror sync in the Board pass (`docket-status`): upsert one GitHub issue per change via `gh`,
  setting state, close reason, labels, and body — alongside, not replacing, `BOARD.md`.
- New `issue:` frontmatter field (same shape as `pr:`) minted on first sync, making the upsert
  idempotent.
- Strictly one-way: each mirror issue carries a banner ("Generated mirror of
  `docs/changes/…` — edits and comments here are not read"). The sync never reads issue state
  back into change files.
- Best-effort semantics, same as the existing board rule: the sync needs network + `gh` auth,
  must never abort a build, and self-heals on the next pass. The committed `BOARD.md` remains
  the canonical, offline-safe view.
- Convention additions in `docket-convention`: the `issue:` field, the status→issue mapping,
  and the one-way rule.

## Out of scope

- Two-way sync of any kind — no reading comments, assignments, or label edits back from GitHub.
- Mirroring the Mermaid dependency graph onto native sub-issue/dependency relationships
  (write-heavy, least-consulted board element; possible later change).
- Requiring GitHub: repos on other remotes simply skip the mirror (knob in `.docket.yml`).
- Replacing `BOARD.md` — the mirror is additive.

## Open questions

- Plain issues + labels, or also auto-manage a Projects v2 board with a Status field? (Projects
  gives real columns but adds GraphQL surface and per-repo setup.)
- What lands in the issue body — full change body, or just title/frontmatter summary plus a link
  to the change file on the `docket` branch?
- Label namespace and collision policy in repos that already use `status:`-style labels.
- Does the sweep/finalize link the PR to the mirror issue (auto-close on merge), or is closing
  left entirely to the one-way sync to avoid two writers?

## Reconcile log
