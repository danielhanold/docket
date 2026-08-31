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
   drive the typed `docket change kill` transaction. There is **no caller-supplied date**: it
   derives the UTC archive date from its own transaction clock **inside** the transaction (never a
   caller `now()`). Author the non-empty `## Why killed` section body and pin the exact record
   submitted for the kill — its `path` and opaque entity `version`, both from the caller's
   authoritative context read — into a bounded JSON request file (`-` for stdin):

   ```
   # request-file: { "change_id": <id>, "path": "<changes_dir>/active/<UTC-birth>-<id>-<slug>.md",
   #                 "version": "<entity-version>", "why_killed": "<why>" }
   docket change kill --repo-dir .docket --input <request-file> --json
   ```

   Trust the typed outcome: `applied` ⇒ archived — an idempotent no-op if already archived,
   including across a day boundary (it reuses the existing dated filename). This ONE metadata commit
   atomically owns the archive move, the refreshed `updated:` date, the spliced `## Why killed`
   section, the `## Artifacts` re-render, the retargeted spec back-link, and the inline board render
   — so the step-2 re-render and step-5 board pass below carry **nothing** for the kill path, exactly
   as `finalize closeout` owns them for the done path. A wrong `version` or an illegal source status
   returns a typed refusal that writes nothing (a lost CAS race is `contended`; see
   *Determinism invariant*).

2. **Artifact block + spec back-link — owned atomically by step 1, no separate caller commit.**
   Both terminal transactions re-render the archived record's `## Artifacts` block (plan/results
   re-point to the integration branch at terminal state) **and** re-stamp every metadata-resident
   back-link — the spec's `docket:backlink` block included (change 0136) — retargeted to the
   now-**archived** change path, **in the same step-1 metadata commit** as the archive:

   - On the **done** path the step-1 `docket finalize closeout` transaction owns this restamp
     atomically (change 0369; proven by `internal/app/finalize_closeout_test.go`'s
     `TestCloseoutBacklinkLegDocketMode` and
     `internal/app/finalize_closeout_integration_test.go`'s
     `TestIntegrationFinalizeCloseoutBacklinkLegDocketMode`).
   - On the **kill** path the step-1 `docket change kill` transaction owns it identically — it
     re-renders the `## Artifacts` block and retargets the linked spec's `docket:backlink` block in
     its one commit.

   So **no separate caller re-render or back-link commit runs for either path** — the skill never
   invokes a facade renderer and never hand-edits a managed block. Terminal publication is deferred
   from Go v1, so nothing is copied onto the integration branch; in `docket` mode a spec that lives
   on the metadata ref is restamped in that same step-1 commit. A typed refusal (malformed markers,
   missing artifact) leaves the file untouched and aborts per the caller's posture — surface it,
   never hand-edit the block.

3. **Terminal publication (deferred).**
   terminal publication is deferred from Go v1 — `docket finalize closeout` is the complete automated closeout boundary.
   publication-deferral marking is deferred from Go v1 — existing `publish-deferred` markers remain as historical evidence.
   Step 1's supported Go metadata closeout — `docket finalize closeout` on the done path,
   `docket change kill` on the kill path — is the whole automated closeout: no terminal record is
   copied onto the integration branch, and the `## Publish deferred` marker is never written. A request
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

5. **Board refresh — owned atomically by step 1, no separate pass.** Both terminal transactions
   render the inline `BOARD.md` **inside their own step-1 metadata commit** — `docket finalize
   closeout` on the done path, `docket change kill` on the kill path — so **no separate Board pass
   runs**, and no skill ever hand-renders the board or double-commits it. The step-1 transaction is
   the sole writer; a caller neither invokes a board renderer nor follows the typed mutation with a
   second board commit. `BOARD.md` is the live planning view and is never published to the
   integration branch.

## main-mode degradation

In single-branch/`main`-mode the metadata working tree *is* the integration branch, so the step-1
archive commit is itself the terminal record: terminal publication stays a deferred no-op, and the
step-1 transaction re-points the `## Artifacts` block and every back-link in place within that same
commit (there is no separate step-2 commit). Step 4 is unchanged; step 5's board render rides the
step-1 commit.

The `terminal_publish` knob (change 0064) is likewise inert in `main`-mode — the mode guard already
makes the publish a no-op, so there is no surface for the knob to act on.

## Failure posture — per caller

The sequence is shared; the posture on a failed step-1 transaction is the caller's (steps 2 and 5
are absorbed into step 1 and carry no separate command; step 3 is a deferred no-op that cannot fail):

| Caller | Posture |
|---|---|
| `docket-finalize-change` (single-change close-out) | **abort-and-report** — stop this change's close-out, surface the failure |
| `docket-status` merge sweep (bulk janitor) | **log-and-continue** — abandon the remainder of this change's close-out, move to the next change; the next sweep self-heals idempotently |
| `docket-implement-next` reconcile-kill | trust each exit code; a failure aborts the kill and is surfaced before looping back to selection |
| `docket-new-change` proposed-kill | same as reconcile-kill — surface and stop; nothing else is in flight |

**Step-failure propagation:** step 1's atomic transaction owns the archive move, the `## Artifacts`
re-render, every back-link, and the inline board render **together** — it either commits the complete
set (fail-closed) or writes nothing, so there is no partial step-2 or step-5 follow-up left to skip.
Step 3 (terminal publication) is a deferred no-op and never fails. Step 4 (cleanup) follows the
caller's own skill body: the sweep treats it as best-effort (log and continue; a later pass
self-heals); other callers keep their own posture (abort-and-report).

## Determinism invariant

Two agents both driving the same terminal transition converge through the step-1 transaction's
exact-version CAS: one applies and the other reads `contended` (a lost race), re-runs
`docket repository prepare`, and re-reads authority rather than racing a second write. The archive
date is the transaction's own UTC clock (never a caller `now()`), so a replay after a lost response
reuses the same dated filename. Every derived view (`## Artifacts` block, back-links, inline board)
is regenerated deterministically inside that one commit — on a rebase conflict in generated content,
**regenerate, never 3-way merge**.
