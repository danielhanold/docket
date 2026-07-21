# Terminal close-out — the shared per-change sequence

> Single source for the close-out sequence a terminal transition (`done` or `killed`) runs:
> archive → re-render `## Artifacts` → terminal-publish → cleanup → board. All four drivers route
> through this file: `docket-finalize-change`'s per-change close-out and `docket-status`'s merge
> sweep (the two `done` drivers), plus the kill callers — `docket-implement-next`'s reconcile-kill
> and `docket-new-change`'s proposed-kill (changes 0054/0055). The sequence is one; only the
> failure posture differs per caller (table below). This file owns ordering and posture; each
> script's mechanics live in its co-located contract (`scripts/<name>.md`).

Contents: [The sequence](#the-sequence-docket-mode) · [main-mode degradation](#main-mode-degradation) · [Failure posture](#failure-posture--per-caller) · [Determinism invariant](#determinism-invariant)

## The sequence (docket-mode)

All metadata writes happen in the metadata working tree (`.docket/`), synced to `origin/docket`
before the first read; every commit pushes immediately.

1. **Archive on `docket` first.** Compute the terminal date in **UTC** — the merge commit's date
   for `done` (`gh`'s `mergedAt`, or `TZ=UTC git show -s --date=format-local:%Y-%m-%d <merge-sha>`),
   the kill commit's date for `killed`. Never `now()`. Author the commit message, pass
   `--results <path>` when a `results:` file arrived via the merge, `--reason "<why>"` on a kill:

   ```
   "${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh archive-change --changes-dir .docket/<changes_dir> \
     --id <id> --outcome <done|killed> --date <UTC-date> [--results <path>] [--reason "<why>"] --message "<msg>"
   ```

   Trust the exit code: `0` ⇒ archived — an idempotent no-op if already archived, including across
   a day boundary (it reuses the existing dated filename). The script commits **the change file
   only** on `metadata_branch`, so the re-render and the board stay separate commits and
   concurrent archivers converge tree-identically (see *Determinism invariant*).

2. **Re-render the `## Artifacts` block — follow-on commit, pushed BEFORE publish.** Regenerate
   the block on the **archived** file (plan/results re-point to the integration branch at
   terminal state; the renderer is the block's sole writer):

   ```
   "${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh render-change-links \
     --change-file .docket/<changes_dir>/archive/<UTC-date>-<id>-<slug>.md --adrs-dir .docket/<adrs_dir>
   ```

   Commit as a separate follow-on metadata commit on `metadata_branch` and push `origin/docket`.
   **Ordering is load-bearing:** `terminal-publish.sh` copies the change file *from
   `origin/docket`* — publishing before this commit lands would publish the stale block onto the
   integration branch, defeating the re-point on the exact surface it targets. Never bundle this
   into the step-1 archive commit (which must stay change-file-only and byte-identical across
   concurrent archivers).

3. **Publish the terminal record.** Reached only after the step-2 commit is on `origin/docket`.
   **Gated by `TERMINAL_PUBLISH`** (change 0064) — pass `<terminal_publish>`, the resolved config's
   `TERMINAL_PUBLISH` value from Step 0's `preflight`/`env` block, straight through:

   ```
   "${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh terminal-publish --id <id> --outcome <done|killed> \
     --integration-branch <integration_branch> --metadata-branch docket \
     --changes-dir <changes_dir> --adrs-dir <adrs_dir> --message "<msg>" \
     --enabled <terminal_publish>
   ```

   Copies the archived change file + its `spec:` (if set) + the **`Accepted`** ADRs in `adrs:`
   from `origin/docket` onto the integration branch in one dedicated commit — the only flow of
   metadata onto the code line. Trust the exit code; its reuse-existing-file idempotency makes two
   drivers racing on the same change a safe no-op.

   When `terminal_publish` is `false` — **the default** since change 0084 — the script is a
   **no-op that exits 0**: the record stays on `docket`, and a suppressed publish is *success* —
   it does NOT trip the skip-publish guard, so steps 4–5 still run. Callers pass the flag and
   keep trusting the exit code; no caller branches on the knob itself.

   **When the publish is expected but does NOT complete — mark it (change 0083).** If
   `terminal_publish` is `true` and this is docket-mode, but the publish is consciously deferred
   (a human gate) or blocked (a wall the run cannot pass, e.g. a protected-branch push denial),
   the driver appends the durable `## Publish deferred` marker before reporting:

   ```
   "${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh mark-publish-deferred --mode add \
     --change-file .docket/<changes_dir>/archive/<UTC-date>-<id>-<slug>.md \
     --reason <deferred|blocked> --detail "<short single-line why>" \
     --date <UTC-date> --integration-branch <integration_branch> --id <id>
   ```

   Commit and push it on `metadata_branch` like any other metadata write. Autonomous callers
   still abort-and-report — the marker makes that abort **durable and self-describing** instead
   of living only in a chat thread, which is precisely how #0043's record went missing for eight
   days with every health check reporting clean. `mark-publish-deferred.sh` **replaces** an
   existing section rather than appending a second, so re-marking is safe.

   **Never mark under suppression.** When `terminal_publish` is `false`, or in `main`-mode, the
   publish is legitimately a no-op that exits 0 — that is *success*, not a deferral, and no
   marker is written. **Never mark on a successful publish**, and never remove the marker by
   hand: `terminal-publish.sh` clears it itself on the success path, so the state stays
   presence-encoded.

4. **Clean up the feature branch + worktree.**

   ```
   "${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh cleanup-feature-branch --slug <slug>
   ```

   Trust the exit code. The provenance guard lives in the script: only worktrees resolving under
   `.worktrees/<slug>` are removed — never the `.docket/` metadata worktree or any out-of-tree
   path.

5. **Board refresh.** Run the Board pass — the single facade call
   `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh docket-status --board-only` (a
   must-land caller adds `--must-land`) — which resolves config itself, gates on the enabled
   surfaces, renders `inline` through the gated `board-refresh.sh` writer, and commits + pushes
   `BOARD.md` on `metadata_branch` itself, always a **separate commit** from the archive commits
   above, only if the board actually changed. Key on its stdout report line, never the exit code —
   the report-line vocabulary, retry classification, and (for `--must-land`) the bounded retry and
   exit-code mapping live in the script contract (`scripts/docket-status.md`); a missing `board …`
   line, or a non-zero exit from this call, is ALSO a failure — never proceed as if the board
   landed just because nothing complained. React per the caller's own Board posture (its skill
   body is authoritative — the step 1–3 table below does not govern step 5): **must-land /
   abort-and-report** callers (`docket-finalize-change`; `docket-new-change`'s proposed-kill)
   stop and surface it; **best-effort / log-and-continue** callers (the `docket-status` merge
   sweep; `docket-implement-next`'s reconcile-kill) log it and move on, trusting a later pass to
   self-heal. `BOARD.md` is the live planning view and is never published to the integration
   branch.

## main-mode degradation

In single-branch/`main`-mode the metadata working tree *is* the integration branch, so the step-1
archive commit is itself the terminal record: `terminal-publish.sh` is a no-op (its own mode-guard
fires), and the step-2 renderer still runs once to re-point the block in place, committed before
cleanup. Steps 4–5 are unchanged.

The `terminal_publish` knob (change 0064) is likewise inert in `main`-mode — the mode guard already
makes the publish a no-op, so there is no surface for the knob to act on.

## Failure posture — per caller

The sequence is shared; the posture on a non-zero exit from steps 1–3 is the caller's:

| Caller | Posture |
|---|---|
| `docket-finalize-change` (single-change close-out) | **abort-and-report** — stop this change's close-out, surface the failure |
| `docket-status` merge sweep (bulk janitor) | **log-and-continue** — abandon the remainder of this change's close-out, move to the next change; the next sweep self-heals idempotently |
| `docket-implement-next` reconcile-kill | trust each exit code; a failure aborts the kill and is surfaced before looping back to selection |
| `docket-new-change` proposed-kill | same as reconcile-kill — surface and stop; nothing else is in flight |

**The skip-publish guard (all callers):** a failed step 1 skips steps 2–3; a **failed step-2
commit/push skips step 3** — a stale `## Artifacts` block must never be published. A **no-diff
re-render is success**: commit the block only when it actually changed; an unchanged block
(nothing to re-point) is not a failure and proceeds to publish — the skip-publish guard fires on
a *failed* commit/push, never on an empty diff. Steps 4–5
follow the caller's own skill body: the sweep treats both as best-effort (log and continue; the
board self-heals on the next pass); other callers keep their own posture (e.g.
`docket-new-change`'s post-kill Board pass is must-land).

## Determinism invariant

Two agents both driving the same terminal transition produce a byte-identical step-1 commit
(change-file-only, UTC terminal date, no `now()`); the loser re-runs `docket.sh preflight` and the
rebase resolves cleanly.
Everything else (re-render, board) is regenerated deterministically from the change files — on a
rebase conflict in generated content, **regenerate, never 3-way merge**.
