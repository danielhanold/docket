# Terminal-publish gap — mark the deferral, stop the checker lying — results

Change: #83 · Branch: feat/terminal-publish-gap-detection · PR: <url> · Plan: docs/superpowers/plans/2026-07-21-terminal-publish-gap-detection.md · ADRs: 51

## Verify (human)

Nothing interactive is required — the whole suite is green (`SUITE rc=0` across `tests/test_*.sh`)
and the live backlog was verified at build time (see *Findings*). Three judgment calls deserve your
eye at the merge gate:

- [ ] **The write side is a documented rule, not an enforced code path.** `board-checks.sh` reads a
      marker that only a *compliant driver* writes. The sweep's `sweep-failed … terminal-publish`
      branch now marks itself in code, and finalize's headless-publish degradation and the
      close-out reference both instruct drivers to mark — but a terminal record that goes missing
      via a path writing no marker (a hard crash between archive and publish) is still invisible.
      This is the accepted cost of "mark, don't detect"; it is now stated plainly in
      `scripts/board-checks.md` rather than merely implied. Confirm you accept the asymmetry —
      **ADR-0051** records the reasoning.
- [ ] **A failed publish after the marker is cleared leaves a window.** The removal is committed and
      pushed on the metadata branch *before* the copy-set is read, because the copy-set is read from
      `origin/docket` and removing afterwards would publish a "publish not completed" marker onto
      `main` with nothing to correct it there. If the publish then fails, the marker is already
      gone; the driver's defer path re-marks (`--mode add` replaces rather than appends). A
      rollback-re-add inside the script was considered and declined — a failure path inside a
      failure path. Confirm you prefer that trade to the alternative.
- [ ] **A permanently-denied removal push is not self-healing.** It leaves a committed-but-unpushed
      local commit while `origin` still carries the marker; recovery is a manual `git push` of the
      metadata branch. The re-run now refuses loudly rather than publishing a marker-carrying
      record, so it fails safe — but it does not fix itself. Documented in
      `scripts/terminal-publish.md`.

## Findings

**Four per-task reviews and a whole-branch review found 1 Critical + 6 Important issues in work that
was green at every step.** Every one was a way for a passing guard to prove less than it claimed —
the same class as change #0104's, and worth reading as a set.

**Critical — the plan's own prescribed code made `--mode remove` mutate files it should not touch.**
`mark-publish-deferred.sh`'s remove path ran strip → trim-trailing-blanks → `mv` unconditionally,
even when no marker was found, so any file ending in more than one newline was silently rewritten on
a no-op. The contract promised "`0` = the file now matches the requested state, **including a no-op
remove**". The test meant to catch it — `remove with no marker changes nothing` — passed **vacuously**,
because the fixture never ended in extra blank lines. Fixed with an exact `grep -qxF` precondition
before any temp-file work. A later round found the first fix (a `cmp -s` gate) still rewrote a file
lacking a trailing newline, falsifying the contract's explicit "byte-untouched, not merely
line-equivalent" claim.

**Important — the check-id registration guard had teeth for only 10 of 12 ids.** The guard derived
the emitted set with `grep -oE '^[[:space:]]*emit [a-z-]+'` — anchored on **line position**, so it
silently missed the `cond || emit …` idiom used twice in `board-checks.sh` (`broken-spec`,
`broken-plan-results`). The reviewer proved it: removing `broken-spec` from `docket-status.md`'s
enumeration left the guard **green**. This is the guard written specifically to prevent a repeat of
change #0098 shipping an unregistered check-id. Re-keyed on call *shape*. A later round found two
more holes in the same guard: a **tautological** arm (`grep -qF -- "$c" "$BCSH"` can never fail,
since `$emitted` was derived by grepping `$BCSH`) and a **count** rather than **set** comparison —
blind to a rename, proven by misspelling an id in the header only and watching 12 = 12 still pass.
Now an exact `comm -3` set compare, matching `test_docket_facade.sh`'s idiom for the same class.

**Important — the gate read the local working tree while the publish read the remote tip.**
`terminal-publish.sh` decided whether to clear the marker by inspecting `$META_WORKTREE/$change_path`
but built its copy-set from `$metaref` eight lines later. With a stale, missing, or mis-resolved
metadata worktree the block was skipped **with no diagnostic** and the script **exited 0 having
published a marker-carrying record onto `main`** — the precise gap this change exists to close.
Reproduced live. The decision is now authoritative with respect to the copy that will actually be
published, and an unresolvable local file is a hard error rather than a silent skip.

**Important — a rebase failure wedged the shared `.docket` worktree.** The CAS retry's rebase path
called `die` with no `git rebase --abort`, leaving the *real, shared* metadata worktree mid-rebase
with a detached HEAD and a `UU` conflict — every subsequent docket operation failing until a human
intervened. Reproduced with a concurrent writer. Now aborted — and, after a further finding, aborted
**conditionally**: only a rebase this script's own `pull --rebase` started, because `.docket` is
shared with concurrent autonomous loops and aborting someone else's rebase destroys their state.

**Important — two guards added *by a fix round* were themselves decoration.** The HEAD-on-branch
guard and the fail-closed postcondition could each be deleted outright with the suite still green,
and the fix report claimed "every guard added in this pass is load-bearing" — which was false. This
is the repo's own standard (`guards-are-code`) failing on the very pass that was applying it. Both
are now driven into their `die` branch by dedicated fixtures (mutation signatures `MHEAD=5`,
`MPOST=4`).

**Important — the sole writer of a durable record could silently truncate it.** The render block
`{ cat "$tmp.2"; printf …; } > "$tmp.3" || die` checks only the **last** command's status. A failed
base copy (ENOSPC/EIO) would leave `$tmp.3` holding only the marker section, skip the `die`, and
`mv` it over the archived change record — whole body gone, exit 0. Split, plus a size postcondition
(documented in both code and contract as a gross-truncation check, not a proof of fidelity).

**Important — unvalidated `--id` corrupted the record permanently, then published clean.**
`--detail`, `--date` and `--reason` were shape-validated; `--id` and `--integration-branch` were not,
and both are interpolated into the rendered body. An injected column-0 `## Fake heading` inside the
marker section made `--mode remove` terminate early, leaving a tail **unremovable by the tool that
wrote it** — and `terminal-publish.sh`, which checks only that the heading is gone, published it
with exit 0. Both are now validated at intake by shape.

**Fixed a pre-existing fail-open guard while we were in the file.** `terminal-publish.sh`'s
postcondition block used `printf … | grep -q …` under `set -o pipefail`. Line 320 pipes the full
integration-branch `ls-tree`, so a consuming repo with a few thousand tracked files crosses the pipe
buffer and gets an intermittent false `postcondition: … missing`; worse, the worktree-survival check
`… | grep -q "pub-$T" && die` **fails open** — a SIGPIPE 141 makes the `&&` not fire and skips the
guard entirely. The branch's headline guarantee rests on that block. Converted to here-strings.

**Live verification (the suite cannot see the metadata branch).** Per `metadata-branch-invisible-to-suite`,
the check was run against the real `.docket` tree: **zero** `publish-deferred` findings (no false
positives across the live backlog), and the detection path was then proven live by marking a
throwaway copy of an archived record and watching the finding fire — fittingly, on **#0043**, the
change whose eight-day invisible gap motivated this work.

## Plan deviations

- **The plan mandated a `teardown_tmp(){ :; }` no-op helper.** Dropped by controller resolution
  before Task 3 — the `trap 'rm -rf "$tmpd"' EXIT` already covers the only provisioned resource, so
  the helper was dead code a reviewer would rightly flag.
- **Two Task 3 fixtures had to be corrected, not merely extended.** The main-mode suppression case
  passed a `docket`-checked-out worktree with `--metadata-branch main` — incoherent, and it made
  that mutation a **false negative** (main-mode died at "no archived change file" before ever
  reaching the block under test). Separately, one test had to get its own fixture because it was
  dying on a `pub-` worktree leaked by an earlier test rather than on its own guard.
- **`skills/docket-convention/references/terminal-close-out.md` exceeded its size budget.** The row
  was raised (147/1238 → 173/1458) per `test_skill_size_budgets.sh`'s own documented remedy —
  insertions only, nothing trimmed from unrelated content (`size-target-is-direction`).
- **The sweep's mark commits and pushes.** Beyond the plan, justified by a mutation showing an
  uncommitted marker dirties the shared metadata worktree and breaks the *next* change's
  `pull --rebase` — worse than the gap it records. Best-effort: it cannot alter the sweep's control
  flow or its emitted report lines.

## Follow-ups

- **#117** — deferred **ADR**-publish visibility. `docket-adr`'s publish path sits behind the same
  protected-`main` wall but has no archive seam and an immutability rule that complicates a body
  marker. Deliberately omitted here (spec §5); auto-captured.
- **#118** — whether the sweep's **skip-publish** path (a failed `## Artifacts` re-render) should
  also mark. From a detection standpoint it leaves the identical archived-but-unpublished state; the
  current "nothing was deferred yet" rationale in `scripts/docket-status.md` is the weakest sentence
  in the doc set. Auto-captured.
- **Not filed, noted:** `scripts/docket-status.sh`'s pre-existing artifacts commit uses a
  pathspec-less `git commit` in the shared `.docket`; the new mark adopts the stricter
  `commit … -- <path>` idiom. Worth aligning the older call site someday.
