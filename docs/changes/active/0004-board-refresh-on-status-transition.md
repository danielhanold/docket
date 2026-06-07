---
id: 4
slug: board-refresh-on-status-transition
title: BOARD.md goes stale during a build — refresh it on status transitions (claim / implemented), not only at Step 0
status: implemented
priority: medium
created: 2026-06-07
updated: 2026-06-07
depends_on: []
related: [2]
adrs: []
spec: docs/superpowers/specs/2026-06-07-board-refresh-on-status-transition-design.md
plan: docs/superpowers/plans/2026-06-07-board-refresh-on-status-transition.md
results:
trivial: false
branch: feat/board-refresh-on-status-transition
pr: https://github.com/danielhanold/docket/pull/5
blocked_by:
reconciled: true
---

## Why

`BOARD.md` lags the change lifecycle for the entire duration of an autonomous build. The
change file's `status:` field — the source of truth — transitions correctly, but the
generated board is not refreshed alongside those transitions, so anyone reading the board
sees a stale picture from before work started.

Concrete evidence (observed dogfooding docket on `~/dev/markhaus`, change #0021):

- `docket-implement-next` regenerates the board exactly once, at **Step 0** (the
  `docket-status` call **before** selection/claim).
- It then mutates `status:` twice without touching the board: `proposed → in-progress` at
  **claim** (Step 2) and `in-progress → implemented` at **PR open** (Step 7). Neither step
  regenerates `BOARD.md`.
- Net effect: from the moment a change is claimed, through reconcile + plan + build + review
  + PR, and even after it reaches `implemented`, the board keeps showing the **pre-claim**
  snapshot — e.g. the change rendered as `build-ready` under *Proposed* the whole time. In
  the markhaus run the board only became correct because the operator regenerated it by hand
  at the end; a "pure" run leaves it stale until the *next* `docket-status` Board pass
  (typically the next `docket-implement-next` Step 0, or a manual `docket-status`).

The terminal transitions are already covered — `docket-finalize-change` and the
`docket-status` merge-sweep both run the Board pass on `done`/`killed`. The gap is the
**non-terminal** transitions the implementer performs itself (`in-progress`, `implemented`),
which have no board refresh.

This is a **visibility / observability gap, not a correctness bug.** Claiming is a
compare-and-swap on the change *file* (`pull --rebase` → re-read `status:` → proceed only if
still `proposed`), never on the board, so a stale board cannot cause a double-claim. But it
does mean the board — docket's at-a-glance answer to "what's happening right now?" — is
wrong for the entire build window: a human watching a long autonomous drain (or a second
agent glancing at the board) sees claimed/in-flight/PR-open work still advertised as
proposed and build-ready.

## What changes

Make the board reflect the non-terminal status transitions that `docket-implement-next`
performs, so it is correct *during* a build, not only before and (eventually) after.

Likely shape (to be settled at brainstorm): have `docket-implement-next` regenerate
`BOARD.md` (the existing `docket-status` Board pass — the single source of board-generation
logic, never a bespoke second renderer) immediately after the **claim** write (Step 2) and
after the **`implemented`** write (Step 7), pushed to `metadata_branch` in the same flow as
the status edit. Equivalently, factor the Board pass so any skill that mutates a change's
`status:` can cheaply re-render the board, keeping the "change file is truth, BOARD.md is a
derived view" invariant intact and the renderer DRY.

The conflict story already exists and should be reused: the Board pass regenerates wholesale
and, on a `pull --rebase` collision in `BOARD.md`, rebuilds from the change files rather than
3-way merging — so adding more board regens under concurrency stays safe.

## Out of scope

- **Changing the lifecycle** or adding new statuses. `in-progress` correctly spans
  plan/build/review; this change is only about the *board reflecting* the existing states.
- **Reconcile-time board churn.** Reconcile (Step 3) does not change `status:`, so it needs
  no board refresh; don't add gratuitous regens for steps that don't move status.
- **Terminal transitions.** `done`/`killed` board refresh already works via
  `docket-finalize-change` / merge-sweep — leave it as-is.
- Live/streaming board updates beyond status-transition points (no file-watcher, no push on
  every sub-step).

## Open questions

- **Is the lag actually a defect, or accepted eventual-consistency?** The board is a derived
  view; one view is "it's fine for it to trail until the next `docket-status`." Decide
  whether live-on-transition is worth the extra commits/pushes per build.
- **Where does the regen live?** Inline in `docket-implement-next` (call the `docket-status`
  Board pass after the claim/implemented writes), or a small shared "regen board" step every
  status-mutating skill invokes? The latter is DRY but touches more skills.
- **Cost under concurrency.** Each added board regen is another commit + push to
  `origin/<metadata_branch>` and another rebase/regenerate opportunity. Is two extra board
  pushes per change acceptable, or should the regen be best-effort / coalesced?
- **Do other skills have the same gap?** Audit every place a `status:` is written
  (`blocked`, `deferred`, revive) for whether the board is refreshed there too — fix the
  class, not just the two `docket-implement-next` sites.
- **Should `docket-status`'s health checks flag board/source drift** (board shows a status
  that disagrees with the change file) as a safety net regardless of who regenerates?

## Reconcile log

### 2026-06-07 — brainstormed + reconciled (spec written same day)

Spec: `docs/superpowers/specs/2026-06-07-board-refresh-on-status-transition-design.md`.
Verified the spec's anchors against the live skills — no drift (the spec is hours old):
claim (implement-next Step 2), reconcile-kill (Step 3), `implemented` (Step 7), new-change
proposed-kill sub-path, terminal-publish's "BOARD.md is never published", docket-status
health checks, and the `sync-convention.sh` canonical block all present as described.

All five **Open questions** are resolved by the spec:

1. **Defect vs eventual-consistency** → fix it (live-on-transition), but cheaply.
2. **Where the regen lives** → one terse invariant in the canonical `## Convention` block
   (synced ×5) + inline Board-pass calls at each status-write site; no heavyweight shared
   "regen" step. Renderer stays single-sourced in docket-status; board stays a *separate*
   commit (bundling rejected — it breaks the claim-CAS determinism).
3. **Cost under concurrency** → the refreshes implement-next runs inline (claim, reconcile-kill,
   `implemented`) are **best-effort / non-fatal** (bounded retry, then log-and-continue);
   self-heal at the next must-land Board pass.
4. **Other skills with the gap** → the class is bigger than the two named sites: the **two
   `killed` origins** (new-change proposed-kill, implement-next reconcile-kill) also skip the
   board (they invoke only terminal-publish, which never touches BOARD.md). **Four** sites get
   a Board pass. `blocked`/`deferred`/revive have no driving skill → covered by the tripwire.
5. **Drift tripwire** → yes; add a board/source drift warning to docket-status health checks.

Scope/altitude unchanged; no code exists yet, so nothing to drop or fold in. Build-ready.
