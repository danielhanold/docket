---
id: 7
slug: github-board-mirror-boundary
title: GitHub board mirror — one-way, change-files-authoritative, driven by a deterministic script
status: Accepted
date: 2026-06-14
supersedes: []
reverses: []
relates_to: [1]
change: 11
---

## Context

Change 0011 adds a GitHub surface to docket's board: each change mirrored to a GitHub issue
(and a Projects v2 item), so the status humans must act on — `implemented`, needs your merge —
shows up where GitHub renders it best (an open issue with a linked PR in an "awaiting merge"
view). Two questions had to be settled before any code, because both set precedents the rest of
docket will inherit:

1. **Direction of authority.** docket's whole model is that change files on the metadata branch
   are the single source of truth (ADR-0001). A GitHub surface is bidirectional by temptation —
   issues invite comments, label edits, status drags on a Projects board. Letting any of that
   flow back into change files would create a second writer and dissolve the source-of-truth
   guarantee.

2. **Execution model.** Every existing docket board operation is local, side-effect-free, and
   idempotent-by-regeneration (`inline` rewrites `BOARD.md` from the change files), so it is
   safely expressed as agent-executed skill prose. The mirror is the opposite: idempotent,
   **side-effectful writes to an external API** (create-or-update issues, reconcile labels,
   move project items) that the test suite cannot observe (it only sees the integration-branch
   checkout). Agent-constructed `gh`/GraphQL calls carry LLM variance — duplicate issues, label
   drift, a wrong close reason, a second project — on writes that are externally visible and not
   cheaply reversible.

## Decision

1. **The mirror is strictly one-way and the change files stay authoritative.** GitHub state is
   derived output, never read back: no comments, labels, assignments, or column drags flow into
   change files. The Board-pass sync is the **sole writer** of issue open/closed state and close
   reason (`done` → completed, `killed` → not planned). A PR may *reference* its mirror issue
   (a plain `#N` link, for the linked-PR view) but never `Closes #N`, which would make GitHub a
   second writer that cannot express `killed → not planned`.

2. **The `github` surface is implemented as a deterministic script** (`scripts/github-mirror.sh`)
   that the Board pass invokes — not agent-constructed calls. The script owns the external-write
   mechanics; it is idempotent (keyed on the per-change `issue:` field), best-effort (degrades on
   missing network / auth / `project` scope, never aborts a build), and testable (a mock-`gh`
   `--dry-run` test asserts command construction). The `inline` surface stays agent-prose — only
   `github` gets the script, a scoped departure from docket's all-prose model, not a wholesale
   shift.

## Consequences

- The source-of-truth guarantee survives a human-facing surface: GitHub is a window, not a back
  door. The cost is that edits made on GitHub are silently ignored (a banner on every mirror
  issue says so), and two-way features (triage from GitHub) are permanently out of scope here.
- External writes are reproducible and unit-testable without a live GitHub, at the price of a new
  execution model (a shell script) for docket to maintain alongside its skills — justified only
  because the writes are side-effectful and externally visible; future local/derived board work
  should stay agent-prose.
- The script does no git writes: it emits `issue-minted <id> <number>` lines and the Board pass
  persists `issue:` (and the first-sync `github_project` write-back into `.docket.yml`), keeping
  all metadata commits under the existing push discipline.
- Idempotency rests on the `issue:` field and the configured `github_project`; if those are lost,
  a re-sync can duplicate the issue/project — the same reliance the `pr:` field already carries.
