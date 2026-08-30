# Terminal close-out — the shared per-change sequence

> Single source for the close-out sequence a terminal transition (`done` or `killed`) runs:
> archive → re-render `## Artifacts` → cleanup → board (terminal publication is deferred from Go v1).
> All four drivers route
> through this file: `docket-finalize-change`'s per-change close-out and `docket-status`'s merge
> sweep (the two `done` drivers), plus the kill callers — `docket-implement-next`'s reconcile-kill
> and `docket-new-change`'s proposed-kill (changes 0054/0055). The sequence is one; only the
> failure posture differs per caller (table below). This file owns ordering and posture; each
> script's mechanics live in its co-located contract (`scripts/<name>.md`).

Contents: [The sequence](#the-sequence-docket-mode) · [main-mode degradation](#main-mode-degradation) · [Failure posture](#failure-posture--per-caller) · [Determinism invariant](#determinism-invariant)

## The sequence (docket-mode)

All metadata writes happen in the metadata working tree (`.docket/`), synced to `origin/docket`
before the first read; every commit pushes immediately.

1. **Archive on `docket` first.** The two terminal outcomes split here: `done` runs the Go
   `finalize closeout` transaction; `killed` stays on the frozen Bash archiver (`finalize closeout`
   does not cover the `killed` outcome — change 0369).

   **Done drivers** (`docket-finalize-change`'s close-out, the `docket-status` merge sweep) archive
   through the typed transaction. There is **no caller-supplied date**: it derives the UTC archive
   date from the verified GitHub `mergedAt` **inside** the transaction (never `now()`), so nothing
   is computed or passed for it here:

   ```
   docket finalize closeout --id <id> [--input <notes.json>]
   ```

   `--input` carries only the optional authored closeout notes (`verification_outcomes`,
   `late_findings`; `-` for stdin); the merged `results:` file is read from the verified merge, not
   passed. Trust the typed outcome: `done-archived` (or `stacked-merged` / `root-archived` for a
   stack) ⇒ the change is marked done and relocated to the dated archive path — idempotent if
   already archived, including across a day boundary. This ONE metadata commit atomically owns the
   archive move, the `## Artifacts` re-render, the re-stamp of **every metadata-resident back-link
   including the spec** (which lives on the metadata ref), and the inline board render; a typed
   refusal or process failure writes nothing and aborts per the caller's posture, with **no partial
   caller-owned follow-up**. It still relocates the change file in its own step, so concurrent done
   drivers converge tree-identically (see *Determinism invariant*). The frozen step-2 re-render and
   step-5 board pass below still run for this path — they re-confirm what the transaction already
   landed and are idempotent no-ops (a no-diff re-render is success).

   **Kill drivers** (`docket-implement-next`'s reconcile-kill, `docket-new-change`'s proposed-kill)
   keep the FROZEN Bash archive leg. Compute the terminal date in **UTC** — the kill commit's date
   (`TZ=UTC git show -s --date=format-local:%Y-%m-%d <kill-sha>`). Never `now()`. Author the commit
   message and pass `--reason "<why>"` (`--results <path>` when a `results:` file arrived):

   ```
   "${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh archive-change --changes-dir .docket/<changes_dir> \
     --id <id> --outcome killed --date <UTC-date> [--results <path>] [--reason "<why>"] --message "<msg>"
   ```

   Trust the exit code: `0` ⇒ archived — an idempotent no-op if already archived, including across
   a day boundary (it reuses the existing dated filename). The script commits **the change file
   only** on `metadata_branch`, so the re-render and the board stay separate commits and
   concurrent archivers converge tree-identically (see *Determinism invariant*).

2. **Re-render the `## Artifacts` block — separate follow-on commit, pushed before cleanup.** Regenerate
   the block on the **archived** file (plan/results re-point to the integration branch at
   terminal state; the renderer is the block's sole writer):

   ```
   "${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh render-change-links \
     --change-file .docket/<changes_dir>/archive/<UTC-date>-<id>-<slug>.md --adrs-dir .docket/<adrs_dir>
   ```

   Commit as a separate follow-on metadata commit on `metadata_branch` and push `origin/docket`.
   **Stage by explicit path** — that tree is shared, so a bare `add -A` commits another agent's
   staged work under your message. This re-render re-points the durable `## Artifacts` block in
   place on the archived record; terminal publication is deferred from Go v1, so nothing is copied
   onto the integration branch. Never bundle it into the step-1 archive commit (which must stay
   change-file-only and byte-identical across concurrent archivers) — keeping the re-render a
   separate follow-on commit is what preserves that determinism.

   **The spec's back-link (change 0136)** re-points on the `active/ → archive/` move too, but only
   the **kill** path restamps it here. On the **done** path the step-1 `docket finalize closeout`
   transaction already owns this restamp atomically — it re-renders every metadata-resident
   back-link, the spec included, in the same metadata commit as the archive (change 0369; proven by
   `internal/app/finalize_closeout_test.go`'s `TestCloseoutBacklinkLegDocketMode` and
   `internal/app/finalize_closeout_integration_test.go`'s
   `TestIntegrationFinalizeCloseoutBacklinkLegDocketMode`) — so **no separate caller step runs for
   the done path**. On the **kill** path — which `finalize closeout` does not drive — re-stamp the
   spec's `docket:backlink` block in this same follow-on commit to point at the now-**archived**
   change path. Skip when there is no `spec:`; must-land:

   ```
   docket artifact backlink --repo-dir .docket \
     --artifact <spec-path> --change <changes_dir>/archive/<UTC-date>-<id>-<slug>.md
   ```

   The operation is the sole writer of the `docket:backlink` block; a typed refusal (malformed
   markers, missing artifact) leaves the file untouched — surface it, never hand-edit the block.

3. **Terminal publication (deferred).**
   terminal publication is deferred from Go v1 — `docket finalize closeout` is the complete automated closeout boundary.
   publication-deferral marking is deferred from Go v1 — existing `publish-deferred` markers remain as historical evidence.
   Step 1's supported Go metadata closeout — `docket finalize closeout` on the done path, the frozen
   `archive-change` leg on the kill path — is the whole automated closeout: no terminal record is
   copied onto the integration branch, and no `## Publish deferred` marker is ever written. A request
   that specifically requires *published* terminal artifacts on the integration branch stops
   **before** claiming that outcome, even when the metadata transaction itself succeeded. Existing
   published records and any existing `## Publish deferred` markers remain untouched historical
   evidence — the `publish-deferred` health check keeps them visible. The frozen Bash publisher is
   **not** a supported fallback, and an enabled `terminal_publish:` key activates nothing.

4. **Clean up the feature branch + worktree.**

   ```
   docket finalize cleanup --id <id>
   ```

   Trust the typed outcome. Ownership is proven **inside the transaction** (change 0369): only
   workspaces and feature refs proven owned by *this* terminal change are removed — the local ref
   only when its recorded tip is detached from every worktree AND contained in the verified merge
   chain, the remote ref only under an exact old-value lease with no open child PR still targeting
   it — never the `.docket/` metadata worktree, the primary tree, or any out-of-tree path. Any
   resource whose ownership it cannot prove is **retained**, not force-removed (so a kill leg whose
   branch never merged keeps its feature ref rather than losing it). A failure aborts per the
   caller's posture.

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

The sequence is shared; the posture on a non-zero exit from steps 1–2 is the caller's (step 3 is a
deferred no-op that cannot fail):

| Caller | Posture |
|---|---|
| `docket-finalize-change` (single-change close-out) | **abort-and-report** — stop this change's close-out, surface the failure |
| `docket-status` merge sweep (bulk janitor) | **log-and-continue** — abandon the remainder of this change's close-out, move to the next change; the next sweep self-heals idempotently |
| `docket-implement-next` reconcile-kill | trust each exit code; a failure aborts the kill and is surfaced before looping back to selection |
| `docket-new-change` proposed-kill | same as reconcile-kill — surface and stop; nothing else is in flight |

**Step-failure propagation:** a failed step 1 (archive) skips step 2 (re-render) for every caller.
A **no-diff re-render is success**: commit the block only when it actually changed; an unchanged
block (nothing to re-point) is not a failure. Step 3 (terminal publication) is a deferred no-op and
never fails. Steps 4–5 follow the caller's own skill body: the sweep treats both as best-effort
(log and continue; the board self-heals on the next pass); other callers keep their own posture
(e.g. `docket-new-change`'s post-kill Board pass is must-land).

## Determinism invariant

Two agents both driving the same terminal transition produce a byte-identical step-1 commit
(change-file-only, UTC terminal date, no `now()`); the loser re-runs `docket.sh preflight` and the
rebase resolves cleanly.
Everything else (re-render, board) is regenerated deterministically from the change files — on a
rebase conflict in generated content, **regenerate, never 3-way merge**.
