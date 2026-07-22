---
name: docket-finalize-change
description: Use when a change's PR is approved or merged and you want to close it out to done promptly rather than waiting for the safety-net sweep — merging if approved, verifying the merge landed, archiving the change, cleaning up its branch and worktree, and refreshing the board. The human's closing bookend; mirrors docket-new-change.
---

# docket-finalize-change — close out a change (human)

## Overview

`docket-finalize-change` is the human's deliberate close-out for a change at the merge gate: merge the approved PR, then drive the **`done`** terminal transition — harvest learnings, archive, publish terminal records if the repo opted in, clean up the branch and worktree, refresh the board. It reuses the same idempotent archive-and-publish flow as `docket-status`'s safety-net sweep, so it is safe to run even if the sweep already ran.

## When to use

- A PR was approved (merge + close out in one step), or was merged via the GitHub button and you want it archived now rather than at the next sweep.
- You are closing out one or several merged changes and want branch/worktree cleanup + a board refresh in one pass.

## Convention (load first — blocking)

Invoke `docket-convention` first (unless already loaded this session) and follow its **Step-0 preamble (every operating skill)**: load the convention, then run `docket.sh preflight` as its own Bash call and read the printed `KEY=value` block off stdout (it resolves config, enforces the bootstrap verdict fail-closed, and syncs the metadata working tree). Everything below uses its vocabulary without redefinition.

### The durable root (change 0075)

Every step of this skill **after** the merge gate's suite run — the merge, the metadata writes,
the archive, `terminal-publish`, `cleanup-feature-branch.sh`, and the Board pass — runs from the
durable root: the absolute main-worktree path the Step-0 `preflight` block prints as `REPO_ROOT=`.
Prefix those Bash calls with `cd <that path>` (or target them with `git -C <that path>`). A
correctness requirement, not hygiene: cleanup removes `.worktrees/<slug>`, and a CWD inside it is
orphaned — the agent's **next** Bash call cannot start, stranding the run after the destructive
step landed (`cleanup-feature-branch.sh` refuses when the caller's CWD is at or inside the target,
as the backstop). Two plausible derivations are both wrong: never as `dirname` of
`METADATA_WORKTREE` (in `main`-mode that *is* the repo root, so `dirname` yields its **parent**),
and never `git rev-parse --show-toplevel` (a linked worktree returns itself, not the main one).
**The merge gate's suite run is the exception** — it happens in the feature worktree, which is
where it belongs; only the close-out steps move to the durable root.

## Selection

Given an explicit change id or an **id allowlist**, OR auto-detect.

**Explicit id** (`docket-finalize-change <id>`) — never prompts (an explicit id is unambiguous). The rebase-retest correctness gate still runs. The explicit id is itself the human authorization, so **an explicit id overrides `require_pr_approval`**: it merges even an unapproved PR. The approval policy governs only the auto-detect path.

**Id allowlist** (`docket-finalize-change 90,92,94`) — generalizes the explicit-id form; a single id is the degenerate case. The set bounds *which changes are eligible*; the run still merges only the best-ordered one of them (see *Ordering* below). **Naming the ids IS the authorization** the *attended* multi-candidate prompt would otherwise have collected, so an allowlist never prompts **for selection** and overrides `require_pr_approval` exactly as a single explicit id does (an *attended* run can still prompt for repair sign-off — a different guard, see *Sign-off on auto-authored repairs*). (No driver-run path prompts — see the matrix; what naming the ids adds is the approval override and the *Finalize blocked* override below.) A scoped id that is not eligible is **skipped with its reason**, never force-merged, and never aborts the run. Unset ⇒ every eligible `implemented` change is a candidate.

**Auto-detect** — already-merged PRs are archived silently (idempotent, unchanged). For the
rest, classify every `implemented` candidate and act per this matrix:

| Candidate | Behavior |
|---|---|
| Not git-mergeable (`CLOSED`, `DRAFT`, or a conflict the gate's resolver reported **ambiguous** — `mergeable: CONFLICTING` alone is *not* this; it deprioritizes, see *Ordering*) | **Surface, do not merge** |
| `FINALIZE_REQUIRE_PR_APPROVAL` is `true` AND unapproved (`reviewDecision != APPROVED`) | **Surface, do not merge** |
| **Exactly one eligible** candidate | **Run the full flow — gate + merge + finalize — with NO prompt** |
| **More than one eligible** candidate | **Driver/autonomous run: NO prompt** — take the *Ordering* head. **Attended run** closing out several at once: **Prompt** — list them and confirm the batch (the blast-radius guard) |

"Eligible" = git-mergeable AND (`FINALIZE_REQUIRE_PR_APPROVAL` is `false` OR approved). The ambiguity count is over *eligible* candidates only: an unapproved PR when `FINALIZE_REQUIRE_PR_APPROVAL` is `true` is surfaced-not-merged and does **not** count toward the prompt. Git-conflict *resolution* is delegated to the rebase-retest gate below; selection's "surface, do not merge" covers only states the gate can't act on. **The multi-candidate prompt is an interactive-*batch* guard, superseded by *One merge per invocation* below** — a single merge is never a batch — so it governs only an attended run closing out several changes at once; a driver or autonomous run selects by *Ordering* and never prompts.

**Ordering — by mergeability, not priority.** The goal is to close out as many changes as possible per drain, so selection maximizes each attempt's chance of success. Among eligible candidates, take the head of:

1. **`depends_on` order** — a hard correctness constraint, not a preference: a dependency is satisfied only at `done`, so a dependent never merges ahead of its dependency however mergeable it looks.
2. **GitHub's `mergeable` field** — `MERGEABLE` first: `CONFLICTING` **deprioritizes, never excludes**. It sorts *last* among eligible candidates (a conflicted PR is the most expensive to close out, which is exactly what mergeability ordering is for), and resolution stays delegated to the gate's `docket-rebase-resolver`. Only a conflict the **gate** can't act on — the resolver reporting it ambiguous — marks *Finalize blocked*, via the abort-and-report set it already belongs to.
3. **Smallest diff first** — `changedFiles`, then `additions + deletions`: cheaper to re-test, less likely to redden the suite after rebase, and lands the most changes before any halt.
4. **`priority` → `created` → lowest id** — the final tiebreak. Priority is *demoted*, not deleted: it still encodes human intent and guarantees a total, reproducible order.

Probe with `gh pr view <n> --json mergeable,mergeStateStatus,changedFiles,additions,deletions`. **GitHub computes `mergeable` lazily** — the first query returns `UNKNOWN` and only *triggers* the computation — so poll, bounded, and treat a still-`UNKNOWN` result as "attempt it": the rebase-retest gate is the real arbiter, and a wrong guess costs one gate run, not correctness. Do **not** build pairwise file-overlap ranking — measured against this repo's real backlog on 2026-07-18, it discriminates nothing and costs O(n) extra `gh` calls per invocation. Revisit only on evidence.

**Re-selection replaces sequencing.** Each invocation re-derives "best next" against the **current** `origin/<integration_branch>`, so no precomputed order can go stale — the direct answer to the moving base, since every merge moves it and an order authored before the first merge is a prediction about a base that no longer exists by the third.

The per-change steps below run for each selected change; step 5 (Board) runs once at the end.

## Terminal disposition (driver contract)

Every run ends by declaring exactly **one** of four dispositions — the **same four words** `docket-implement-next` uses, so one driver keys on both skills without knowing which it is driving:

| Disposition | Meaning | Driver action |
|---|---|---|
| `advanced` | **A close-out advanced** — this run merged a change and closed it out, **or** it archived an already-merged PR (the human clicked Merge; real close-out work ran, just no merge by this run). Either way the per-change steps ran to completion. | continue |
| `contended` | Another writer got there first — the `docket-status` sweep archived it between selection and close-out; the archive is an idempotent no-op, so **nothing merged**. Qualifier: if *this* run performed the merge, it is `advanced` regardless of who archived it — the no-op archive alone is not the signal. | continue — re-select next |
| `drained` | No eligible `implemented` change in scope. | **stop** |
| `halted` | Any abort-and-report point fired, **or** every member of a non-empty eligible set needs a human. | **stop + surface** |

The driver's decision is binary: **continue on `advanced`/`contended`, stop on `drained`/`halted`.** The contract is **driver-agnostic** — `/loop`, cron, a scheduled agent, or a human re-typing the command are all equally valid; docket owns the contract, never the driver.

**One merge per invocation.** A run merges **exactly one** change and exits `advanced`; it **never batches**. Consecutive close-outs come from the driver re-invoking, not from an in-run loop — which is also what keeps the blast-radius posture the multi-candidate prompt was protecting, and why that prompt is superseded on the driver path. Archiving several **already-merged** changes in one run does not violate this rule: no merge occurred, so there is no blast radius to bound.

**Every abort-and-report point maps to `halted`.** The set enumerated in *abort-and-report points (the full set)* below is unchanged — this **names** existing behavior and adds none.

**A blocked-but-non-empty set is `halted`, never `drained`.** There *is* work; it just needs a human. `drained` must keep meaning "genuinely nothing to do," or the driver's stop signal loses its meaning. Resolving the boundary exactly: a candidate that was **in scope but skipped for any human-requiring reason** — unapproved under `require_pr_approval`, not git-mergeable, or carrying `## Finalize blocked` — **counts toward the non-empty set and yields `halted`**. `drained` requires that no `implemented` change was in scope at all.

The final report **enumerates** the change merged (if any), each change **skipped with its reason** (outside the id allowlist / not git-mergeable / unapproved under `require_pr_approval` / already carrying `## Finalize blocked` **on the auto-detect path only — a named id overrides that skip** / waiting on an unmerged `depends_on`), and which disposition ended the run.

## Per-change steps

1. **Check the PR** (`gh`). Already merged → straight to step 2. Approved + mergeable but not merged → merge it into `<integration_branch>` (resolved from `.docket.yml`; not hard-coded `main`). An explicit id IS the merge decision (and overrides `require_pr_approval`); under auto-detect, follow the Selection matrix. **Before the merge lands, run *The rebase-retest merge gate* below** (unless `finalize.gate` is `off`). The merge itself, and every step after it through the close-out, works from the repo's main worktree (see above).

2. **Verify the merge landed** on the integration branch. If the change carries a `results:` file, this is the moment to append interactive-verification outcomes and any late findings to it, post-merge.

2.5 **Harvest learnings.** Gated on `learnings.enabled` (from the Step-0 config export): when `false`, print exactly one line — `learnings disabled — harvest skipped` — and go to step 3 (never silently; "harvested zero" must be distinguishable from "skipped because disabled"). When enabled: distill this change's close-out signals — PR review comments, merge-gate feedback, `results:` findings — into zero or more **findings** under `<changes_dir>/learnings/` (shape per the convention's *Learnings ledger* and its reference). **Create** `learnings/<slug>.md` or **extend** the existing family finding (a dated `## War story` entry with `(#<id>, PR #<n>)` provenance, this change's id added to `changes:`, `updated:` bumped) — never merge two existing distinct findings. Set `promotion_state: candidate` on any finding whose rule must fire **unprompted**. Zero findings is normal; kills are not harvested. **Idempotency probe:** skip if some finding file's `changes:` list already contains this change's id — read via `lib/docket-frontmatter.sh`'s `list_field`, never a bare numeric grep (a bare id can match a PR number or a date). Then re-render the index atomically — a failed render must never truncate the last-good index (direct `>` truncates on open), so render to a same-directory temp file and replace only on success: `tmp=$(mktemp .docket/<changes_dir>/learnings/.render-index.XXXXXX) && "${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh render-learnings-index --learnings-dir .docket/<changes_dir>/learnings > "$tmp" && [ -s "$tmp" ] && mv "$tmp" .docket/<changes_dir>/learnings/README.md || rm -f "$tmp"`. On success, commit the finding file(s) + index together as **its own commit** on `metadata_branch` (never bundled with the archive commit), only if the render changed bytes, and push. On a render failure (no `mv`), commit the finding file(s) alone — the sweep refreshes the index next pass — and surface the failure rather than reporting the harvest as clean. This step is the harvest procedure's single source; `docket-status`'s sweep invokes it by reference. Separately from the harvest: when `AUTO_CAPTURE_ENABLED` is `true`, close-out findings that are distinct **follow-up work** rather than build-loop lessons are classified and minted as stubs per the convention's *Auto-capture* shared definition (a finding can be a lesson, a stub, or neither — never both by default), committed and pushed independently of the harvest commit; every mint, dedup skip, and cap overflow is reported.

3. **Archive → re-render → publish.** This is the shared terminal close-out sequence — **the single source is `skills/docket-convention/references/terminal-close-out.md`; follow it exactly, steps 1–3.** Finalize-only facts the reference doesn't carry: compute the merge date in **UTC** via `gh`'s `mergedAt` (never `now()`); pass `--results <path>` to `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh archive-change` when a results file arrived via the merge; the sequence's re-render step is `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh render-change-links` (sole writer of the archived file's `## Artifacts` block, committed as a follow-on and pushed before publish reads it); this skill's posture on any non-zero exit is **abort-and-report** (stop this change's close-out, surface the failure — see the reference's *Failure posture* table).

4. **Clean up** — from the repo's main worktree (see above): invoke `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh cleanup-feature-branch --slug <slug>`; trust the exit code.

5. **Board** — run the must-land Board pass: `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh docket-status --board-only --must-land` — a non-zero exit means the board did not land; STOP and surface it (abort-and-report). The board is the live planning view and is **never** published to the integration branch.

6. **Sync the integration checkout (best-effort)** — once at the end of the run: `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh sync-integration-branch --integration-branch <integration_branch>`. FF-only, guarded, never aborts or alters the close-out; every skip is a normal exit 0.

## The rebase-retest merge gate

Guards step 1's merge — the **only** place docket itself merges. Every value below is read from the Step-0 `preflight` export block (`FINALIZE_GATE`, `FINALIZE_TEST_COMMAND`, `FINALIZE_REQUIRE_PR_APPROVAL`), never by parsing `.docket.yml`; the block documents what each key means and where it's set:

```yaml
finalize:
  gate: local                 # local (default) | ci | both | off
  test_command:               # OPTIONAL override; unset => the agent auto-detects the suite
  require_pr_approval: false  # default false. true => the auto-detect path refuses to merge
                              #   an unapproved PR (reviewDecision != APPROVED), surfacing instead.
```

`gate` defaults to **`local`**; `ci` validates GitHub checks; `both` requires local **and** CI green; **`off`** is the documented opt-out — merge trusting the PR's own CI, with no rebase and no re-test (today's pre-gate behavior).

`require_pr_approval` validates *human sign-off* (`gate` validates *correctness*); the skill reads its resolved value as **`FINALIZE_REQUIRE_PR_APPROVAL`** from the Step-0 export block — the sole channel, resolving repo-local > repo-committed > global > built-in `false` (change 0102) — governing only auto-detect; an explicit id always overrides it. `true` ⇒ refuses to merge a PR not `APPROVED`, surfacing it instead. Approval must come from a **human** reviewer — a co-maintainer, or the maintainer if eligible on someone else's PR. See ADR-0011 (consent model) and ADR-0043 (why bot-approval was retired).

**Flow** (runs before `gh pr merge`):

1. `gate == off` → merge trusting the PR's own CI; skip the rest of the gate.
2. **Rebase** `feat/<slug>` onto `origin/<integration_branch>`. On conflict, dispatch the `docket-rebase-resolver` subagent (foreground, at the model/effort its wrapper resolves) to reconcile every hunk until the rebase completes; an **ambiguous conflict** it can't resolve aborts the rebase and the gate **abort-and-reports**.
3. **Determine the suite:** `test_command` override, else auto-detect. Under `local`/`both` with no detectable suite and no `test_command`, **abort-and-report** — this fires only when the suite is *undetectable*; a detected suite that runs clean (even one with zero tests) is green and proceeds.
   For the detected Bash-suite shape (`tests/test_*.sh`), run every test with the configured runtime. An explicit `FINALIZE_TEST_COMMAND` is user-authored shell text: execute that text unchanged, without prefixing or rewriting it, while leaving the exported `DOCKET_BASH_PATH` available to the command's environment. This is the command boundary:

<!-- configured-bash-finalize:start -->
```bash
if [ -n "${FINALIZE_TEST_COMMAND:-}" ]; then
  eval "$FINALIZE_TEST_COMMAND"
else
  for test in tests/test_*.sh; do
    "$DOCKET_BASH_PATH" "$test"
  done
fi
```
<!-- configured-bash-finalize:end -->

4. **Validate per `gate`:**
   - `local` runs the suite in the worktree **before any push**.
   - `ci` pushes `--force-with-lease` then polls `gh pr checks`; `both` does both.
   - On **red**, dispatch `docket-integration-repair` (foreground, at the model/effort its wrapper resolves) — it root-causes and writes a minimal fix in at most two attempts. Green → apply the sign-off rule below. Stuck / cannot reach green → **abort-and-report**. Red or absent CI checks under `ci`/`both` also **abort-and-report**.
5. **Push** `--force-with-lease` if rebased and not already pushed; a lease rejected by a concurrent push → **abort-and-report**.
6. `gh pr merge` — **without** `--admin` whenever the PR is already `APPROVED`, or the integration
   branch's protection requires a pull request but **zero** approvals (docket's single-maintainer
   default — see the README's finalize/merge section). `--admin` remains available only on the
   pre-existing explicit-id / attended paths, where a sole maintainer chooses to force past an
   otherwise-unsatisfiable required review → the existing close-out (harvest → archive →
   terminal-publish → cleanup → board).

### The two agents (split at rebase-completion)

`docket-rebase-resolver` resolves conflicts *during* the rebase and never runs tests; `docket-integration-repair` owns the **red suite** *after* the rebase lands, regardless of cause. Neither wraps a skill (only `docket-convention`); both are dispatched **foreground at the model/effort its wrapper resolves** — never a literal tier. An authored repair from `docket-integration-repair` is what fires the sign-off rule below; pure conflict resolution does not.

### Sign-off on auto-authored repairs

A repair is code the human's approval predated, so it never merges unseen:

- **Interactive finalize**: force-push the repaired branch, report the repair diff + what broke, and **prompt** for go-ahead before `gh pr merge`.
- **Autonomous finalize**: cannot prompt, so it force-pushes the repair and follows **abort-and-report** — STOP, do not merge; the human reviews the pushed repair on the PR and re-runs finalize to merge.

### abort-and-report points (the full set)

Each leaves the **PR open** and the change **`implemented`**: an ambiguous rebase conflict · `local`/`both` with no detectable suite and no `test_command` override · repair cannot reach green in ≤2 attempts · `ci`/`both` with red or absent CI checks · a `--force-with-lease` rejected by a concurrent push · any repair under **autonomous** finalize (sign-off) · **a harness or permission classifier denying the merge itself** (the branch-protection recipe in the README is what makes `gh pr merge` land unattended; if a denial still fires, that is a `halted`, not a retry loop).

**Where the reason surfaces.** The subagent returns its diagnosis in-context; finalize relays it to the human (interactive) or the dispatching caller (autonomous), records it durably as a **comment on the PR** (`gh pr comment`) — a human returning later reads exactly why the auto-merge stopped — **and appends the `## Finalize blocked` marker to the change file** (see below). Every abort-and-report point does all three; the comment is the narrative, the marker is the state.

### `## Finalize blocked` — marking a change that needs a human

A gate failure is recorded as a `## Finalize blocked` body section on the change file — a **metadata write** on `metadata_branch` like any other field write, appended in the metadata working tree and pushed immediately. The heading itself is **bare**, never dated (`has_section` is a whole-line match, so `## Finalize blocked — 2026-07-18` would not be detected); the date lives inside the section body instead, exactly as the live `## Auto-groom blocked` instances already do. It is deliberately **not a new status** and **not a reuse of `blocked`**: the change really *is* `implemented` with an open PR, and an eighth status would flatten the six distinct abort reasons into one label while forcing changes to the lifecycle diagram, the board renderer, the GitHub mirror's seven-state mapping, and the health checks. Shape mirrors the proven `## Auto-groom blocked`: a dated entry naming **which** reason fired and what the human must do. A **re-mark REPLACES the existing section, never appends a second heading** — the marker is state, not a log, and a retry that fails again must leave exactly one section (record successive attempts as dated bullets *inside* it).

- **Auto-detect selection skips** any **unmerged** change already carrying the section — without this a re-run re-selects the same known-bad change forever and the drain never progresses past it. **An already-merged PR is archived regardless of the marker**: the silent-archive path is idempotent close-out work, not a merge decision, so a change the human merged by hand must never be stranded on the board by a stale marker. **An explicitly named id or allowlist member overrides the skip**: naming the id is the human's "I looked at it, retry" signal, exactly as the explicit id is itself the human authorization everywhere else in this file. Without the override the marker would deadlock — a skipped change can never be finalized, so the clearing rule below could never fire.
- **A `CONFLICTING` PR is NOT marked at selection time** — the gate's `docket-rebase-resolver` usually resolves it, so marking it up front would strand a fixable PR as human-blocked. Marking happens only where every other abort-and-report reason does: at the gate.
- **A successful finalize removes the section.** Unlike auto-groom's human-only re-arm, the condition is machine-verifiable (the gate passed), so requiring a human to delete it would strand stale markers on changes that are fine. That removal is a **live-path obligation** — an `implemented` change that stays `implemented` must not carry a stale needs-you cell — **not** a guarantee about archived files: nothing strips the section at close-out, so on an out-of-band human merge it rides into `archive/` verbatim, and nothing needs to. **Every automated reader is scoped to a change short of `done`** (the board cell, the auto-detect skip, the `stale-finalize-blocked` check, and the mirror's readiness label — the one reader that also scans `archive/`, whose `readiness_label` has no `done` arm), so archiving retires the marker's meaning whether or not the section survives; what the archived section keeps is its **human** reader, the record of why the change stalled. That is the presence-encoded-state rule **discharged, not waived** — removal is its usual *means*; the *end* is that no reader is left misinformed. Do **not** add strip-on-archive to satisfy it literally: it destroys that record and puts body surgery into the shared terminal primitive for zero observable gain.
- The board renders a change carrying it as **`finalize blocked — needs you`**, parallel to `auto-groom blocked — needs you`.

## Where finishing-a-development-branch fits

When a human is present — the **one** exception to the *Skill layer*'s autonomy-precedence rule — the resolved finish skill (`$SKILL_FINISH`, default `superpowers:finishing-a-development-branch`) can drive a non-standard close-out (keep, discard, or merge locally without a PR); its chooser fits at step 4. On the autonomous path the finish skill is **not invoked at all**: docket's own steps 1–6 drive the close-out and the rebase-retest gate governs the merge, so there is no chooser to meet. The gate is independent of the skill either way; docket also borrows its provenance-guard (only auto-remove a worktree under `.worktrees/<slug>`).

## Terminal publish (docket-mode)

The shared procedure — documented in `skills/docket-convention/references/terminal-close-out.md` — copies a change's terminal records from `origin/docket` onto the integration branch. It runs on every terminal transition (`done`, via step 3 and `docket-status`'s sweep; or `killed`, via the killing skill), distinguished by a publish token (`<id>` for a change, `adr-<NN>` for a standalone/status-changed ADR). **Skipped entirely in `main`-mode** — the archive move is itself the terminal record there — **and skipped whenever `terminal_publish` is `false`** (default; change 0084).

The copy-set: the archived change file, its `spec:` if set, and each `adrs:` entry whose ADR is `Accepted` (`Proposed`/draft ADRs are skipped). `BOARD.md` is **never** published. Mechanics — `checkout origin/docket -- <copy-set>`, the CAS push, self-verify, teardown — are owned by `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh terminal-publish --id <id> --enabled <terminal_publish>` (or `docket.sh terminal-publish --adr <NN> --enabled <terminal_publish>` for the ADR-only path — `<terminal_publish>` is the resolved `TERMINAL_PUBLISH` from the Step-0 `preflight` block); see `scripts/terminal-publish.md`.

**Headless publish degradation.** On a **headless** run with `terminal_publish: true`, an agent permission classifier can deny the records-push. The denial does **not** fail the run: archive + cleanup + board already landed; finalize skips retrying the push and surfaces one line — `terminal-publish blocked (auto-mode push denial) — run docket.sh terminal-publish --id <id> (and --adr <NN> for any published ADR) manually`. Do not stop there: apply the close-out reference's mark step (`--mode add --reason blocked`) to the archived change file too — a chat-only deferral is the #0043 failure mode. **Attended** runs are unaffected. Version-defense (change 0062's spike observed no headless denial, but the classifier is not a docket-owned contract).
