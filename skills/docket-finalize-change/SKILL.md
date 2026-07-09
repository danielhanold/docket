---
name: docket-finalize-change
description: Use when a change's PR is approved or merged and you want to close it out to done promptly rather than waiting for the safety-net sweep — merging if approved, verifying the merge landed, archiving the change, cleaning up its branch and worktree, and refreshing the board. The human's closing bookend; mirrors docket-new-change.
---

# docket-finalize-change — close out a change (human)

## Overview

`docket-finalize-change` is the human's deliberate close-out for a change that has reached the merge gate. It mirrors `docket-new-change` — the opening bookend that starts a change's life — by providing the closing bookend that ends it. Rather than waiting for the safety-net merge-sweep that `docket-status` and `docket-implement-next` run in bulk, finalize handles one or more specific changes now: merging the approved PR into the integration branch, then driving the **`done`** terminal transition — harvesting learnings, archiving the change on `metadata_branch`, publishing its terminal records onto the integration branch (`docket`-mode), cleaning up the branch and worktree, and refreshing the board. It reuses the same idempotent archive-and-publish flow as `docket-status`'s safety-net sweep, so it is safe to run even if the sweep already ran first.

`done` is one of docket's two **terminal transitions** — the other is `killed` (driven by the producer from `proposed`, or the implementer from `in-progress` via reconcile). The publish-to-integration step is a **shared procedure** owned by this skill (see *Terminal publish (docket-mode)* below); the kill origins and `docket-status`'s sweep reference it. This skill is its single source.

## When to use

- A PR was approved and you want to merge it and close the change in one step.
- A PR was merged via the GitHub button and you want to archive the change immediately rather than waiting for the next `docket-status` or `docket-implement-next` sweep.
- You want to clean up the feature branch and worktree after a merge and refresh the board in one pass.
- You are closing out multiple merged changes at once after a sprint or review cycle.

## Convention (load first — blocking)

Before anything else in this skill, invoke the `docket-convention` skill via the Skill tool — unless it was already invoked earlier in this session and its content is in context. Everything below uses its vocabulary (build-ready, metadata working tree, terminal-publish, the `DOCKET`/`LIVE` bootstrap probes, …) without redefinition; no step below is executable without the convention loaded.

Resolve config + the bootstrap verdict deterministically: `eval "$("${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket-config.sh --export)"` (fail-closed; read-only). Act on `BOOTSTRAP` — `PROCEED` to continue; `STOP_MIGRATE` to refuse-and-point at `migrate-to-docket.sh`; `CREATE_ORPHAN` to opt into `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket-config.sh --bootstrap` (fresh repo only).

## Selection

Given an explicit change id, OR auto-detect.

**Explicit id** (`docket-finalize-change <id>`) — never prompts (an explicit id is
unambiguous). The rebase-retest correctness gate still runs. The explicit id is itself the
human authorization, so **an explicit id overrides `require_pr_approval`**: it merges even an
unapproved PR. The approval policy governs only the auto-detect path; merging an unapproved PR
simply requires being explicit about it.

**Auto-detect** — already-merged PRs are archived silently (idempotent, unchanged). For the
rest, classify every `implemented` candidate and act per this matrix:

| Candidate | Behavior |
|---|---|
| Not git-mergeable (`CLOSED`, `DRAFT`, or a GitHub-reported conflict the gate can't act on) | **Surface, do not merge** |
| `require_pr_approval: true` AND unapproved (`reviewDecision != APPROVED`) | **Surface, do not merge** — the policy gate; report it so you know docket saw it and why it skipped |
| **Exactly one eligible** candidate | **Run the full flow — gate + merge + finalize — with NO prompt** |
| **More than one eligible** candidate | **Prompt**: list them and confirm the batch (the blast-radius guard) |

"Eligible" = git-mergeable AND (`require_pr_approval: false` OR approved). The ambiguity count
is over *eligible* candidates only: under `require_pr_approval: true` an unapproved PR is
surfaced-not-merged and does **not** count toward the prompt. Git-conflict *resolution* is
delegated to the rebase-retest gate (it rebases onto base and dispatches
`docket-rebase-resolver`); selection's "surface, do not merge" covers only states the gate
can't act on (draft/closed/flatly un-mergeable).

The prompt exists **only to guard the bulk-merge blast radius** — more than one eligible
target at once. The common case, one obvious target the human deliberately invoked finalize on,
merges with **no prompt**.

The per-change steps below run for each selected change.

## Per-change steps

**Steps 1–4 run per selected change** (check → verify → harvest → archive → clean up), exactly mirroring `docket-status`'s per-change archive loop (which invokes the same harvest by reference). **Step 5 (Board) runs once after all selected changes are processed** — it is wholesale and idempotent, so a single regen at the end is correct and avoids redundant regenerations.

1. **Check the PR** (`gh`). Already merged → straight to archive. Approved + mergeable but not merged → **merge it into the integration branch** — `gh pr merge --merge "$pr" --repo … ` (or the team's merge mode) targeting the change's PR against **`<integration_branch>`** (resolved from `.docket.yml`; default `auto` → `origin/HEAD`, fallback `main`), **not hard-coded `main`** (a GitFlow repo merges into `develop`). Invoking finalize on an **explicit change id** IS the merge decision (and overrides `require_pr_approval`) — the gate is respected; under **auto-detect**, follow the Selection matrix above — a single eligible candidate merges with **no prompt**, and finalize prompts **only when more than one** is eligible. **Before the merge lands, run *The rebase-retest merge gate* below** (unless `finalize.gate` is `off`) — it brings the feature branch up to base, validates the integrated result, and only then proceeds to `gh pr merge`. Then continue. (Merging the PR is the only thing that lands plan + results + code on the integration branch — they ride the merge, not the terminal-publish.)

2. **Verify the merge landed on the integration branch** (optionally: tests green on the merged result).

   > **Close-out (optional).** If the change carries a `results:` file, this is the moment to append interactive-verification **outcomes** and any late findings to it — on the integration branch (where the results *file* now lives, via the merge), post-merge. The results file is the durable record of what was hand-verified at the gate.

2.5 **Harvest learnings.** Distill this change's close-out signals — PR review comments (`gh pr view <pr> --comments`), merge-gate feedback, and the `results:` file's findings — into **zero or more** entries at the top of `<changes_dir>/LEARNINGS.md` in the metadata working tree (format per the convention's *Learnings ledger*; provenance `(#<id>, PR #<n>)`). Zero entries is normal — most changes teach nothing new; harvest only what generalizes beyond this change. **Idempotency probe:** skip the change entirely if the ledger already cites `(#<id>` — this is what makes a sweep racing finalize a no-op. If the file exceeds the convention's soft cap, this harvest also distills per the convention's rules. Commit the ledger as its **own commit** on `metadata_branch` (never bundled with the archive commit, which must stay byte-identical across concurrent archivers) and push. Kills are not harvested. This step is the harvest procedure's **single source**; `docket-status`'s sweep invokes it by reference.

3. **Archive (idempotent)** — in the **metadata working tree** (the `.docket/` worktree in `docket`-mode; the primary working tree on the integration branch in `main`-mode), synced to its remote first.

   **Compute the merge date in UTC** — use `gh`'s `mergedAt`, or `TZ=UTC git show -s --date=format-local:%Y-%m-%d <merge-sha>`. Never `now()`. Author the commit message, then invoke the archive primitive:

   ```
   "${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/archive-change.sh --changes-dir .docket/<changes_dir> --id <id> --outcome done --date <merge-date> [--results <path>] --message "<msg>"
   ```

   **Trust the exit code:** `0` ⇒ archived (idempotent no-op if it was already archived — including across a day boundary, since it reuses the existing dated filename); non-zero ⇒ **abort-and-report**. The script owns the mechanics (see `scripts/archive-change.md`); the one fact the steps below rely on is that it commits **the change file only** on `metadata_branch` — so the board (step 5) and the re-point (below) stay separate commits and concurrent archivers converge tree-identically.

   This is **step 1 of the terminal publish** below (archive-on-`docket`-first). Once `archive-change.sh` returns 0, **re-point the block before publishing** — order matters because `terminal-publish.sh` copies the change file *from `origin/docket`*, so the re-pointed block must already be there. In `docket`-mode:
   1. Invoke the renderer to regenerate the change's `## Artifacts` block on the archived file: `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/render-change-links.sh --change-file .docket/<changes_dir>/archive/<merge-date>-<id>-<slug>.md --adrs-dir .docket/<adrs_dir>` (plan/results now re-point to the integration branch at `done`; the renderer is the sole writer of the block). Commit this as a **follow-on metadata commit** on `metadata_branch` and **push `origin/docket`** — so the re-pointed block is on `origin/docket` before the publish reads it.
   2. Then **invoke the *Terminal publish (docket-mode)* procedure with outcome `done`** (token `T = <id>`) to copy the now-re-pointed terminal records from `origin/docket` onto the integration branch.

   In `main`-mode the metadata working tree *is* the integration branch, so this archive commit is itself the terminal record and terminal-publish is **skipped**; the renderer still runs once (after `archive-change.sh` returns 0) to re-point the block in place, committed before the branch/worktree clean-up step.

4. **Clean up** — remove the merged feature branch + worktree by invoking `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/cleanup-feature-branch.sh --slug <slug>`; **trust the exit code** (the provenance guard and self-verification live in the script). The invariant it enforces: only worktrees resolving under `.worktrees/<slug>` are removed — never the `.docket/` metadata worktree or any out-of-tree path.

5. **Board** — regenerate `BOARD.md` (`docket-status`'s Board pass) in the metadata working tree and commit + push it on `metadata_branch` (in `docket`-mode, `origin/docket`) as a separate commit from the archive commits above. The board is the **live planning view and stays on `metadata_branch`** — it is never published to the integration branch.

6. **Sync the integration checkout (best-effort)** — once at the very end of the run (after the board step, so a batch finalize fast-forwards once after all its merges): `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/sync-integration-branch.sh --integration-branch <integration_branch>`. This fast-forwards the clone's local `<integration_branch>` checkout to the tip the merges just pushed, keeping the skills symlinked from it current (change 0029). It is **best-effort like the board** (per the convention's Branch model): FF-only, guarded (on-branch + clean + true-FF), and it **never aborts or alters the close-out** — every skip (wrong branch, dirty tree, non-FF, fetch failure) is a normal exit 0. A no-op in `main`-mode where the metadata working tree already *is* the integration checkout.

**Note:** This archive procedure is **identical** to `docket-status`'s merge-sweep archive — same UTC merge date, same change-file-only commit, same reuse-existing-file idempotency, same terminal-publish invocation. Both skills describe the same operation; they must not diverge.

## The rebase-retest merge gate

Guards step 1's merge — the **only** place docket itself merges. It validates the
*merged result*, not just the PR head: a PR that is behind base can pass its own CI
and still break the integration branch on a semantic conflict git auto-merges cleanly.
Configured by `.docket.yml`:

```yaml
finalize:
  gate: local                 # local (default) | ci | both | off
  test_command:               # OPTIONAL override; unset => the agent auto-detects the suite
  require_pr_approval: false  # default false. true => the auto-detect path refuses to merge
                              #   an unapproved PR (reviewDecision != APPROVED), surfacing instead.
```

`gate` defaults to **`local`** (gate on, validating against the repo's local suite);
`ci` validates GitHub checks; `both` requires local **and** CI green; **`off`** is the
documented opt-out — merge trusting the PR's own CI, with no rebase and no re-test (the
pre-gate behavior).
`test_command` is normally unset — auto-detect the suite by inspecting the repo
(Makefile, `package.json` scripts, a `tests/` dir, CI config); the override is used
verbatim only when auto-detection guesses wrong.

`require_pr_approval` defaults to **`false`** — approval is never a selection-time
blocker (the single-human-friendly default: the author pushes their own PR and so
cannot approve it on GitHub at all). Set it **`true`** to make the **auto-detect path**
refuse to merge an unapproved PR (`reviewDecision != APPROVED`), surfacing it instead of
merging — `gate` validates *correctness*, `require_pr_approval` validates *human sign-off*.
It governs **only the auto-detect path**: an explicit `docket-finalize-change <id>` always
overrides it (the explicit id is itself the human authorization the gate asks for), and the
rebase-retest correctness gate still runs regardless.

The gate operates in the change's feature worktree (`.worktrees/<slug>`) if it still
exists, else a transient worktree on `feat/<slug>` provisioned and torn down like
terminal-publish's `pub-<T>` tree.

**Flow** (runs before `gh pr merge`):

1. `gate == off` → merge trusting the PR's own CI (no rebase, no re-test); skip the rest of the gate.
2. **Rebase** `feat/<slug>` onto `origin/<integration_branch>`. On a clean rebase,
   continue. On conflict, **dispatch the `docket-rebase-resolver` subagent**
   (foreground, at the model/effort its wrapper resolves) to reconcile every hunk
   until the rebase completes; if it reports an **ambiguous conflict** it cannot
   resolve, the rebase is aborted and the gate **aborts-and-reports**.
3. **Determine the suite:** the `test_command` override, else auto-detect. Under
   `local`/`both` with **no detectable suite and no `test_command`**, **abort-and-report** —
   the gate is on but has nothing to validate; the reason names the remedy (set
   `test_command` to point at the suite, or `gate: off` to opt out). This fires only when the
   suite is **undetectable**: a *detected* suite that runs clean — even one with zero tests —
   is green and proceeds.
4. **Validate per `gate`:**
   - `local` → run the suite in the worktree **before any push**.
   - `ci` → push `--force-with-lease`, then poll `gh pr checks`.
   - `both` → local first, then push + CI.
   On green, continue. On **red**, **dispatch the `docket-integration-repair` subagent**
   (foreground, at the model/effort its wrapper resolves) — it owns every red-test
   outcome, root-causes it, and writes a minimal fix in **at most two attempts**. If it
   reaches green, apply the **sign-off rule** below. If it is **stuck / cannot reach
   green**, **abort-and-report**. A `ci`/`both` run with **red or absent CI checks**
   also **aborts-and-reports**.
5. **Push** `--force-with-lease` if the branch was rebased and not already pushed; a
   **lease rejected by a concurrent push** → **abort-and-report**.
6. `gh pr merge` → the existing close-out (harvest → archive → terminal-publish →
   cleanup → board).

The rebase makes the feature sit on top of base, so the eventual `gh pr merge` is
conflict-free — validating the rebased branch validates what actually lands. `local`
runs the suite **before** the force-push so a broken rebase is never force-pushed;
`ci` validates after the push (CI runs on the pushed branch); `both` does both.

### The two agents (split at rebase-completion)

Conflict resolution and semantic repair are different shapes — a bounded reconciliation
versus open-ended debugging — so they are two dedicated wrappers
(`agents/docket-rebase-resolver.md` ①, `agents/docket-integration-repair.md` ②), each
wrapping **no skill** (loading only `docket-convention`), both carrying abort-and-report,
each dispatched **foreground at the model/effort its wrapper resolves** (never a literal
tier in this prose — the wrapper + layered config are the single source). The boundary is
**the rebase completing**: ① resolves conflicts *during* the rebase and never runs tests;
② owns the **red suite** *after* the rebase lands, regardless of cause (base drift or a
bad ① resolution). ①'s report is **conflicts resolved**; ②'s is an **authored repair** —
and an authored repair is what fires the sign-off rule.

### Sign-off on auto-authored repairs

A ② repair is code the human's approval predated, so it **never merges unseen** —
reconciling with the agent layer's abort-and-report rule for autonomous subagents:

- **Interactive finalize** (a human is attending the session): force-push `--force-with-lease` the repaired branch, **report the repair diff + what broke**, and **prompt** for go-ahead before `gh pr merge` (the interactive sign-off).
- **Autonomous finalize** (running as its own subagent, no human to ask): it **cannot** prompt, so it **force-pushes the repair and follows abort-and-report** — STOP, do not merge. The human reviews the pushed repair on the PR and re-runs finalize to merge.

Pure ① conflict resolution does **not** trigger sign-off — it completes the merge the
human already intended and flows through the normal merge path.

### abort-and-report points (the full set)

Each leaves the **PR open** and the change **`implemented`**, surfacing a clear reason:
an ambiguous rebase conflict ① gives up on · `local`/`both` with no detectable suite and
no `test_command` override · ② cannot reach green in ≤2 attempts · `ci`/`both` with red or
absent CI checks · a `--force-with-lease` rejected by a concurrent push · any ② repair
under **autonomous** finalize (sign-off).

**Where the reason surfaces.** The resolver/repair subagent returns its diagnosis to
finalize in-context (the subagent contract — that is what "stop and surface" means for a
dispatched agent); finalize relays it in its own abort-and-report output — to the human in
an interactive session, or to the dispatching caller when finalize itself runs
autonomously. Because an autonomous return is ephemeral, finalize also records the reason
durably as a **comment on the PR** (`gh pr comment`): the change stays `implemented` with
the PR open, so a human returning to it reads exactly why the auto-merge stopped (what
failed, the agent's hypothesis, what it tried). For a ② repair the force-pushed commit is
itself part of that durable record on the PR.

## Where finishing-a-development-branch fits

When a human is present, the **resolved finish skill** — `$SKILL_FINISH` (default `superpowers:finishing-a-development-branch`) — can drive a **non-standard close-out** (keep the branch, discard it, or merge locally without a PR) — its merge/keep/discard chooser fits naturally at step 4. On `auto` or unavailability this optional close-out has no auto artifact — the human drives the keep/discard/merge-locally choice directly; finalize's core rebase-retest merge gate (`finalize.gate`) is independent of the finish skill and still governs any actual merge. docket also borrows `superpowers:finishing-a-development-branch`'s **worktree provenance-guard**: only auto-remove a worktree whose path is under `.worktrees/<slug>` — never remove a worktree outside that known path.

## Terminal publish (docket-mode)

The shared procedure that copies a change's terminal records from `origin/docket` onto the integration branch. **This skill is its single source** — `docket-status`'s sweep (for `done`), the producer's proposed-kill and the implementer's reconcile-kill (for `killed`), and `docket-adr` (for a standalone or status-changed ADR) all invoke *this* procedure rather than restating it.

It runs on **every** terminal transition. A terminal transition is `done` (driven by this skill — step 3 above, and `docket-status`'s sweep) or `killed` (driven by the killing skill). Two entry shapes, distinguished by a **publish token `T`** used to name the throwaway branch:

- **Change publish** (`done` or `killed`): `T = <id>`. The copy-set is built from the change manifest (below); **step 1 (archive-first) applies**. This path is executed by `terminal-publish.sh` (see below).
- **ADR-only publish** (a standalone or supersession/reversal ADR, from `docket-adr`): `T = adr-<NN>`. The copy-set is the single ADR file; **step 1 is skipped** (there is no change file to archive). This path is handled by `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/terminal-publish.sh --adr <NN>` — the single executor of both publish shapes — over its single-ADR copy-set (mechanics: `scripts/terminal-publish.md`).

**Skipped entirely in `main`-mode.** It is guarded on `metadata_branch == docket`; in `main`-mode there is no `docket` branch — the metadata working tree *is* the integration branch, so the archive move there (step 3 above, or the kill clauses in `docket-new-change` / `docket-implement-next`) is itself the terminal record. The **archive-move contract is identical in both modes**: the dated `archive/<DATE>-<id>-<slug>.md` filename (UTC merge/kill-commit date, per the convention) and the reuse-existing-file idempotency rule apply to the `main`-mode archive too; only the tree it runs in differs.

**The copy-set** is the archived change file (always present), its `spec:` file when set, and each `adrs:` entry **whose ADR is `Accepted`** — `Proposed`/draft ADRs are skipped (the **`Accepted` gate**, applied at copy time). `BOARD.md` is **never** published. For an ADR-only publish (`T = adr-<NN>`) it is the single ADR file. `terminal-publish.sh` assembles this list from the manifest; finalize relies only on those facts — see `scripts/terminal-publish.md` for the assembly details.

### The change-publish path (`T = <id>`)

Run the two scripts in order — **archive-first ordering is load-bearing**: step 1 archives the change on `origin/docket` so the copy step can copy the *archived* path:

1. **Archive on `docket` first** — `archive-change.sh` (the same invocation as *Per-change steps* step 3 above, run against the `.docket/` tree synced to `origin/docket`). **Trust the exit code** (`0` ⇒ archived; non-zero ⇒ abort-and-report); mechanics in `scripts/archive-change.md`. (For a `killed` change there is usually no merged PR, so plan/results may not exist — a kill publishes only what is on `docket`: the change file, plus its `spec:`/`adrs:` if set.)
2. **Publish** — `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/terminal-publish.sh --id <id> --outcome <done|killed> --integration-branch <integration_branch> --metadata-branch <metadata_branch> --changes-dir <changes_dir> --adrs-dir <adrs_dir> --message "<msg>"`. **Trust the exit code** (`0` ⇒ the copy-set landed and self-verified on `origin/<integration_branch>`; non-zero ⇒ abort-and-report). The publish is a **copy, not a branch merge** — the script `checkout origin/docket -- <copy-set>` onto the integration branch (applying the `Accepted` gate); `BOARD.md` is never published, and plan/results/code already arrived via the PR (`done`) or do not exist (`killed`). The script is a no-op in `main`-mode (`metadata_branch == integration_branch`). (Mechanics: `scripts/terminal-publish.md`.)

`terminal-publish.sh` is the **executor of both publish shapes** (`--id` for change publish, `--adr` for ADR-only); the ADR-only variant runs over its single-ADR copy-set with step 1 skipped.

The script owns the mechanics — provisioning a transient `pub-<T>` worktree on the integration branch, copying the records from `origin/docket`, the CAS (compare-and-swap) push with its same-file-race retry, the post-push self-verify, the teardown, and the re-run safety (a sweep that races finalize on the same change is a safe no-op). Finalize does not restate them: see `scripts/terminal-publish.md` for the full contract.
