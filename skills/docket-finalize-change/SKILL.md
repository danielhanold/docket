---
name: docket-finalize-change
description: Use when a change's PR is approved or merged and you want to close it out to done promptly rather than waiting for the safety-net sweep — merging if approved, verifying the merge landed, archiving the change, cleaning up its branch and worktree, and refreshing the board. The human's closing bookend; mirrors docket-new-change.
---

# docket-finalize-change — close out a change (human)

## Overview

`docket-finalize-change` is the human's deliberate close-out for a change that has reached the merge gate: merging the approved PR into the integration branch, then driving the **`done`** terminal transition — harvesting learnings, archiving, publishing terminal records onto the integration branch if the repo has opted in (`docket`-mode), cleaning up the branch and worktree, and refreshing the board. It reuses the same idempotent archive-and-publish flow as `docket-status`'s safety-net sweep, so it is safe to run even if the sweep already ran.

## When to use

- A PR was approved and you want to merge it and close the change in one step.
- A PR was merged via the GitHub button and you want to archive the change immediately rather than waiting for the next `docket-status` or `docket-implement-next` sweep.
- You want to clean up the feature branch and worktree after a merge and refresh the board in one pass.
- You are closing out multiple merged changes at once after a sprint or review cycle.

## Convention (load first — blocking)

Invoke `docket-convention` first (unless already loaded this session) and follow its **Step-0 preamble (every operating skill)**: load the convention, then run `docket.sh preflight` as its own Bash call and read the printed `KEY=value` block off stdout (it resolves config, enforces the bootstrap verdict fail-closed, and syncs the metadata working tree). Everything below uses its vocabulary without redefinition.

### The durable root (change 0075)

Every step of this skill **after** the merge gate's suite run — the merge, the metadata writes,
the archive, `terminal-publish`, `cleanup-feature-branch.sh`, and the Board pass — runs from the
durable root: the absolute main-worktree path the Step-0 `preflight` block prints as `REPO_ROOT=`.
Prefix those Bash calls with `cd <that path>` (or target them with `git -C <that path>`).

This is not hygiene, it is a correctness requirement: cleanup removes `.worktrees/<slug>`, and
`git worktree remove --force` **succeeds** while the agent's own CWD is inside that directory —
the process merely orphans its CWD, and the agent's **next** Bash call then cannot start (`cd: no
such file or directory`), stranding the run after the destructive step has already landed. A child
process cannot change its parent's CWD, so no script can fix this; only the skill can. The
script-side guard is the backstop: `cleanup-feature-branch.sh` now refuses (before any destructive
step) when the caller's CWD is at or inside the target.

Two derivations look plausible and are both wrong: never as `dirname` of `METADATA_WORKTREE` (in
`main`-mode `METADATA_WORKTREE` *is* the repo root, so `dirname` yields the repo's **parent**),
and never from `git rev-parse --show-toplevel` (from a linked worktree that returns the linked
worktree, not the main one).

**The merge gate's suite run is the exception** — it happens in the feature worktree, which is
where it belongs. Only the close-out steps below move to the durable root.

## Selection

Given an explicit change id, OR auto-detect.

**Explicit id** (`docket-finalize-change <id>`) — never prompts (an explicit id is unambiguous). The rebase-retest correctness gate still runs. The explicit id is itself the human authorization, so **an explicit id overrides `require_pr_approval`**: it merges even an unapproved PR. The approval policy governs only the auto-detect path.

**Auto-detect** — already-merged PRs are archived silently (idempotent, unchanged). For the
rest, classify every `implemented` candidate and act per this matrix:

| Candidate | Behavior |
|---|---|
| Not git-mergeable (`CLOSED`, `DRAFT`, or a GitHub-reported conflict the gate can't act on) | **Surface, do not merge** |
| `require_pr_approval: true` AND unapproved (`reviewDecision != APPROVED`) | **Surface, do not merge** — the policy gate |
| **Exactly one eligible** candidate | **Run the full flow — gate + merge + finalize — with NO prompt** |
| **More than one eligible** candidate | **Prompt**: list them and confirm the batch (the blast-radius guard) |

"Eligible" = git-mergeable AND (`require_pr_approval: false` OR approved). The ambiguity count is over *eligible* candidates only: an unapproved PR under `require_pr_approval: true` is surfaced-not-merged and does **not** count toward the prompt. Git-conflict *resolution* is delegated to the rebase-retest gate below; selection's "surface, do not merge" covers only states the gate can't act on.

The per-change steps below run for each selected change; step 5 (Board) runs once at the end.

## Per-change steps

1. **Check the PR** (`gh`). Already merged → straight to step 2. Approved + mergeable but not merged → merge it into `<integration_branch>` (resolved from `.docket.yml`; not hard-coded `main`). An explicit id IS the merge decision (and overrides `require_pr_approval`); under auto-detect, follow the Selection matrix. **Before the merge lands, run *The rebase-retest merge gate* below** (unless `finalize.gate` is `off`). The merge itself, and every step after it through the close-out, works from the repo's main worktree (see above).

2. **Verify the merge landed** on the integration branch. If the change carries a `results:` file, this is the moment to append interactive-verification outcomes and any late findings to it, post-merge.

2.5 **Harvest learnings.** Gated on `learnings.enabled` (from the Step-0 config export): when `false`, print exactly one line — `learnings disabled — harvest skipped` — and go to step 3 (never silently; a reader must be able to tell "harvested zero" from "skipped because disabled"). When enabled: distill this change's close-out signals — PR review comments, merge-gate feedback, `results:` findings — into zero or more **findings** under `<changes_dir>/learnings/` (shape per the convention's *Learnings ledger*). For each lesson, either **create** `learnings/<slug>.md` or **extend** the existing family finding whose slug already covers the class — append a dated `## War story` entry with `(#<id>, PR #<n>)` provenance, add this change's id to `changes:`, bump `updated:`. Never merge two existing distinct findings — that is human-gated curation. Set `promotion_state: candidate` on any finding whose rule must fire **unprompted** — the tiering criterion is the convention's to state, not this step's to restate. Zero findings is normal. **Idempotency probe:** skip if some finding file's `changes:` list already contains this change's id — read via `lib/docket-frontmatter.sh`'s `list_field`, never a bare numeric grep (a bare id can match a PR number or a date). Then re-render the index — direct `>` truncates on open, so a failed render would empty the index before its exit code is even checked, and an emptied index is a byte change that then looks committable; render to a same-directory temp file first and only replace the index once the render has actually succeeded: `tmp=$(mktemp .docket/<changes_dir>/learnings/.render-index.XXXXXX) && "${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh render-learnings-index --learnings-dir .docket/<changes_dir>/learnings > "$tmp" && [ -s "$tmp" ] && mv "$tmp" .docket/<changes_dir>/learnings/README.md || rm -f "$tmp"` — the same atomic-write discipline `docket-status.sh`'s own `learnings_regen_index` uses for the sweep's re-render, so a render failure here never truncates or corrupts the last-good index. On success, commit the finding file(s) + index together as **its own commit** on `metadata_branch` (never bundled with the archive commit), only if the render actually changed bytes, and push. On a render failure (the `mv` did not happen), commit the finding file(s) alone — the last-good index survives untouched and `docket-status`'s own sweep will refresh it next pass — and surface the render failure rather than reporting the harvest as clean. Kills are not harvested. This step is the harvest procedure's single source; `docket-status`'s sweep invokes it by reference.

3. **Archive → re-render → publish.** This is the shared terminal close-out sequence — **the single source is `skills/docket-convention/references/terminal-close-out.md`; follow it exactly, steps 1–3.** Finalize-only facts the reference doesn't carry: compute the merge date in **UTC** via `gh`'s `mergedAt` (never `now()`); pass `--results <path>` to `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh archive-change` when a results file arrived via the merge; the sequence's re-render step is `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh render-change-links` (sole writer of the archived file's `## Artifacts` block, committed as a follow-on and pushed before publish reads it); this skill's posture on any non-zero exit is **abort-and-report** (stop this change's close-out, surface the failure — see the reference's *Failure posture* table).

4. **Clean up** — from the repo's main worktree (see above): invoke `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh cleanup-feature-branch --slug <slug>`; trust the exit code.

5. **Board** — invoke `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh docket-status --board-only` — the single Board-pass entry point; it renders, commits, and pushes `BOARD.md` itself on `metadata_branch`, a separate commit from the archive commits above, only if the board actually changed. **Must-land:** key on the stdout report line, never the exit code — `board inline changed push-failed` is the only retryable line; every other report line (`board inline changed pushed`, `board inline clean`, `board off`, `board github ok`, `board github failed`) is terminal. On `board inline changed push-failed`, re-run `docket.sh preflight` and invoke it again, bounded to 3 attempts total; if it still reports `board inline changed push-failed`, STOP and surface the failure. The board is the live planning view and is **never** published to the integration branch.

6. **Sync the integration checkout (best-effort)** — once at the end of the run: `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh sync-integration-branch --integration-branch <integration_branch>`. FF-only, guarded, never aborts or alters the close-out; every skip is a normal exit 0.

## The rebase-retest merge gate

Guards step 1's merge — the **only** place docket itself merges. Configured by `.docket.yml`:

```yaml
finalize:
  gate: local                 # local (default) | ci | both | off
  test_command:               # OPTIONAL override; unset => the agent auto-detects the suite
  require_pr_approval: false  # default false. true => the auto-detect path refuses to merge
                              #   an unapproved PR (reviewDecision != APPROVED), surfacing instead.
```

`gate` defaults to **`local`**; `ci` validates GitHub checks; `both` requires local **and** CI green; **`off`** is the documented opt-out — merge trusting the PR's own CI, with no rebase and no re-test (today's pre-gate behavior).

`require_pr_approval` validates *human sign-off* (`gate` validates *correctness*); it governs only the auto-detect path — an explicit id always overrides it.

**Flow** (runs before `gh pr merge`):

1. `gate == off` → merge trusting the PR's own CI; skip the rest of the gate.
2. **Rebase** `feat/<slug>` onto `origin/<integration_branch>`. On conflict, dispatch the `docket-rebase-resolver` subagent (foreground, at the model/effort its wrapper resolves) to reconcile every hunk until the rebase completes; an **ambiguous conflict** it can't resolve aborts the rebase and the gate **abort-and-reports**.
3. **Determine the suite:** `test_command` override, else auto-detect. Under `local`/`both` with no detectable suite and no `test_command`, **abort-and-report** — this fires only when the suite is *undetectable*; a detected suite that runs clean (even one with zero tests) is green and proceeds.
4. **Validate per `gate`:**
   - `local` runs the suite in the worktree **before any push**.
   - `ci` pushes `--force-with-lease` then polls `gh pr checks`; `both` does both.
   - On **red**, dispatch `docket-integration-repair` (foreground, at the model/effort its wrapper resolves) — it root-causes and writes a minimal fix in at most two attempts. Green → apply the sign-off rule below. Stuck / cannot reach green → **abort-and-report**. Red or absent CI checks under `ci`/`both` also **abort-and-report**.
5. **Push** `--force-with-lease` if rebased and not already pushed; a lease rejected by a concurrent push → **abort-and-report**.
6. `gh pr merge` → the existing close-out (harvest → archive → terminal-publish → cleanup → board).

### The two agents (split at rebase-completion)

`docket-rebase-resolver` resolves conflicts *during* the rebase and never runs tests; `docket-integration-repair` owns the **red suite** *after* the rebase lands, regardless of cause. Neither wraps a skill (only `docket-convention`); both are dispatched **foreground at the model/effort its wrapper resolves** — never a literal tier. An authored repair from `docket-integration-repair` is what fires the sign-off rule below; pure conflict resolution does not.

### Sign-off on auto-authored repairs

A repair is code the human's approval predated, so it never merges unseen:

- **Interactive finalize**: force-push the repaired branch, report the repair diff + what broke, and **prompt** for go-ahead before `gh pr merge`.
- **Autonomous finalize**: cannot prompt, so it force-pushes the repair and follows **abort-and-report** — STOP, do not merge; the human reviews the pushed repair on the PR and re-runs finalize to merge.

### abort-and-report points (the full set)

Each leaves the **PR open** and the change **`implemented`**: an ambiguous rebase conflict · `local`/`both` with no detectable suite and no `test_command` override · repair cannot reach green in ≤2 attempts · `ci`/`both` with red or absent CI checks · a `--force-with-lease` rejected by a concurrent push · any repair under **autonomous** finalize (sign-off).

**Where the reason surfaces.** The subagent returns its diagnosis in-context; finalize relays it to the human (interactive) or the dispatching caller (autonomous), and also records it durably as a **comment on the PR** (`gh pr comment`) — a human returning later reads exactly why the auto-merge stopped.

## Where finishing-a-development-branch fits

When a human is present, the resolved finish skill — `$SKILL_FINISH` (default `superpowers:finishing-a-development-branch`) — can drive a non-standard close-out (keep, discard, or merge locally without a PR); its chooser fits at step 4. finalize's core rebase-retest gate is independent of it and still governs any actual merge. docket also borrows its worktree provenance-guard: only auto-remove a worktree under `.worktrees/<slug>`.

## Terminal publish (docket-mode)

The shared procedure — documented in `skills/docket-convention/references/terminal-close-out.md` — that copies a change's terminal records from `origin/docket` onto the integration branch. It runs on every terminal transition (`done`, driven by step 3 above and `docket-status`'s sweep; or `killed`, driven by the killing skill), distinguished by a publish token `T` (`<id>` for a change publish, `adr-<NN>` for a standalone/status-changed ADR).

**Skipped entirely in `main`-mode** (guarded on `metadata_branch == docket`) — there the archive move is itself the terminal record — **and skipped whenever `terminal_publish` is `false`** (the default since change 0084; opt in per-repo with `terminal_publish: true`).

The copy-set: the archived change file, its `spec:` if set, and each `adrs:` entry whose ADR is `Accepted` (`Proposed`/draft ADRs are skipped). `BOARD.md` is **never** published. Mechanics — `checkout origin/docket -- <copy-set>`, the CAS push, self-verify, teardown — are owned by `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh terminal-publish --id <id> --enabled <terminal_publish>` (or `docket.sh terminal-publish --adr <NN> --enabled <terminal_publish>` for the ADR-only path — `<terminal_publish>` is the resolved config's `TERMINAL_PUBLISH` value from the Step-0 `preflight` block); see `scripts/terminal-publish.md`.
