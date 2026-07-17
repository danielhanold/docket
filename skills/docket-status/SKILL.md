---
name: docket-status
description: Use when you want to see or refresh the docket backlog — what is proposed, in progress, blocked, implemented, or done — by refreshing docket state, sweeping merged changes to done, and running health checks for stale claims, broken spec/plan/results links, and dependency stalls.
context: fork
agent: docket-status
---

# docket-status — the board & janitor

## Overview

`docket-status` gives you a queryable, up-to-date view of the backlog and keeps it clean. Four jobs: **report the backlog digest** (the `backlog <status> <count>` and `change <id> <status> <readiness> <slug>` lines — emitted in *every* configuration, board or no board, and **the channel you write your summary from**), refresh docket state (rendering each enabled board surface), sweep any `implemented` change whose PR merged into the archive, and run health checks (stale claims, broken links, dependency stalls). On a full pass it also self-heals the learnings index and surfaces two needs-you advisories — see *Learnings* below. The change files are the source of truth; any board is generated output, never edited by hand. All of this is sequenced by the deterministic orchestrator (contract: `scripts/docket-status.md`; change 0058) as one 8-step pass — this skill invokes it, trusts its exit code, surfaces its report, and applies the handful of judgment calls the script deliberately leaves in-model.

## When to use

- You want to know what is done, next, or stuck — or you suspect a stale board, stale/broken links, or a cleared blocker.
- A PR was merged via the GitHub button (not via `docket-finalize-change`) and the board is stale.
- `docket-implement-next` calls this at step 0 as a self-cleaning safety net before selecting the next change.

## Convention (load first — blocking)

Invoke the `docket-convention` skill via the Skill tool first — unless already invoked this session — and run its *Step-0 preamble*: `docket.sh preflight` as its own Bash call, reading the printed `KEY=value` block off stdout. `docket.sh docket-status` re-derives and re-checks the bootstrap gate + metadata-worktree sync itself, but the block gives you `$DOCKET_SCRIPTS_DIR` and the other exported values for the rest of this skill. Everything below uses the convention's vocabulary without redefinition.

## Mode choice

- **The user only wants to *see* the backlog** (no explicit refresh requested, nothing merged recently that you know of) ⇒ run the board-only pass: `--board-only`.
- **Everything else** — an explicit refresh/cleanup request, `docket-implement-next`'s step-0 safety net, or a post-merge cleanup after a PR merged via the GitHub button ⇒ run the full pass (no flag): board + merge sweep + health checks + judgment lines + learnings self-heal + integration sync.

## Run the orchestrator

```
"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh docket-status [--board-only]
```

Trust its exit code for the pass as a whole: `0` is the pass completing (`board off`, `pass ok`, findings, `sweep-failed`, `sweep-skipped`, `board *-failed`, and `judgment` lines on stdout are all normal outcomes, not errors); non-zero is a hard error — config export failure, an unusable `BOOTSTRAP` verdict or metadata worktree, a bad CLI argument, or a resolved `board_surfaces` that came back empty or with `none` combined with another surface (change 0071) — surface the stderr diagnostic and stop rather than improvising a fix. **The Board pass specifically is narrower than that**: key on its stdout report line, never its exit code alone (the convention's *Board refresh on status writes* is the single source of that contract). A pass that exits `0` but prints no `board …` line at all, or ends its Board pass on `board inline failed`, `board <token> unknown`, `board github failed`, or `board inline changed push-failed`, has ALSO failed at that step.

The script owns the mechanics of what it renders, sweeps, and checks — see `scripts/docket-status.md` for the full 8-step sequence, its output-line shapes, and its failure postures. Surface its report to the user in human terms (what's on the board, what got swept, what health checks flagged) rather than pasting the raw line-oriented output. Health checks stay warn-only — do not auto-fix findings unless the user explicitly asks.

## Read the report — it is the only channel you need

The report is **self-evidencing**: it always states what it did, so you never have to go looking for corroboration.

- **`board off`** — the repo sets `board_surfaces: []` and there is deliberately **no board**. This is a configuration, not a failure. Do not look for `BOARD.md`; it must not exist.
- **`backlog <status> <count>` + `change <id> <status> <readiness> <slug>`** — the backlog digest, emitted in **every** configuration. **This is your backlog-state channel.** Write the summary from these lines. On a full pass the digest is taken **after** the sweep, so it already accounts for everything this pass closed out: a change on a `swept` line is counted under `backlog done` and has no `change` line of its own. Never report a swept change as still awaiting merge.
- **`learnings disabled`** — `learnings.enabled: false`; exactly one such line per pass (never one per change). The ledger's read/write gate is closed for this run: no index render, no advisories, every existing `learnings/` file left byte-untouched. A configuration, not a failure.
- **`learnings index clean` / `learnings index changed pushed` / `learnings index changed push-failed`** — the derived index's self-heal, on a full pass only; see *Learnings* below for the write shape.
- **`learnings over-cap — needs curation (…)`** and **`learnings promotion-pending <n> — needs you`** — the two needs-you advisories. Surface them to the user; never auto-fix — consolidation and promotion are human acts.
- **`pass ok`** — the orchestrator ran to completion. It is always the last line of a successful pass.

Two rules follow, and they are not optional:

- **A thin report is the success case, not a symptom.** An empty sweep, no health findings, and `board off` together mean a healthy, board-less repo. The pass is complete. Do **not** re-run the orchestrator, trace it, or investigate — there is nothing to find.
- **Never probe `BOARD.md`.** With the board off it must not exist; with the board on, summarize from the digest lines rather than opening the file. Reading, rendering, or hand-writing `BOARD.md` is never part of this skill's job — `board-refresh.sh` is its only writer.

## Judgment follow-ups (stay in-model — the script does not do these)

Drive these off the report lines `docket.sh docket-status` emits; skip a category entirely if no matching line appeared.

- **`harvest <id> <path>` lines** — for each, run the harvest-learnings procedure (the *Harvest learnings* step in `docket-finalize-change`'s SKILL.md is its single source — invoke it by reference, do not reimplement it here). Best-effort: log and continue on failure, never abort the rest of the pass for it.
- **`judgment blocked <id> <text>` lines** — re-examine that change's `blocked_by:` free text; flag to the user if the referenced issue/PR/event appears resolved. This is judgment, not a git probe — never scripted.
- **`minted issue <id> <n>` / `minted project <owner> <n>` lines** — write the value back into the change file (`issue:`) or `.docket.yml` (`github_project: {owner, number}`) on `metadata_branch`, following normal push discipline (re-run `docket.sh preflight`, commit, push).
- **`github` mirror reachability** — only when `board_surfaces` includes `github`: warn on a change carrying an `issue:` whose mirror looks unreachable. Best-effort visibility flag, like the other checks — never auto-fix.

## Final summary

Close with a short human-facing summary: backlog state (counts/highlights, read from the digest lines — never from the board file), what was swept to done (if anything), and any health-check findings or judgment flags raised above. When the `inline` board is enabled, point the user at `BOARD.md` (or the GitHub mirror, if enabled) for the full picture rather than reproducing it inline. When the report says `board off`, there is no board to point at — the digest-derived summary **is** the deliverable, and that is the intended, complete outcome.

## Reference: what the board, sweep, and checks mean

The mechanics below live entirely in the orchestrator (contract: `scripts/docket-status.md`) — this is a compact map so a reader knows what the report lines refer to, not a restatement of how they work.

### Board

Renders each surface in `board_surfaces` (config; default `[inline]`) from the same one dependency-resolution pass, computed once. Readiness cells: a dependency-waiting change shows **⏳ waiting on #N — not yet built** or **⏳ waiting on #N — needs your merge**; a `proposed` change with no spec, not `trivial: true`, and not waiting shows **needs-brainstorm** — or **auto-groom blocked — needs you** when its body carries an `## Auto-groom blocked` section.

When `board_surfaces` includes `inline`, `board-refresh.sh` (contract: `scripts/board-refresh.md`) is the single gated writer of `BOARD.md`: it owns the surface gate and the atomic replace, wrapping the pure renderer `/render-board.sh` (contract: `scripts/render-board.md`) internally so nothing else ever touches the file; the orchestrator commits and pushes the result to `metadata_branch` only when it actually changed. This skill **never hand-edits `BOARD.md`, never hand-renders it, and never 3-way merges it**; on a rebase conflict, regenerate through `board-refresh.sh` — never a hand-merge — and continue. When `board_surfaces` omits `inline`, there is simply no board. Where present, `BOARD.md` is the live planning view and stays on `docket` — never published to the integration branch.

`github` is the one-way Issues + Projects v2 mirror (`github-mirror.sh`, mechanics in `skills/docket-convention/github-board-mirror.md`), best-effort — runs only when `board_surfaces` includes `github`; a fresh mint prints `issue-minted`/`project-minted` lines to record back into the change file / `.docket.yml`.

### Merge sweep

The bulk safety net: every `implemented` change whose PR has merged gets archived on `metadata_branch`, its terminal record published only if the repo has opted in (via `terminal-publish` onto the `integration_branch` in `docket`-mode), and its branch cleaned up, chaining the same close-out sequence (`terminal-close-out.md`) `docket-finalize-change` uses. Runs automatically at `docket-implement-next` step 0 and on any explicit non-`--board-only` `docket-status` invocation.

The rebase-onto-base + re-run-tests gate lives in `docket-finalize-change`'s merge step and is **finalize-only** — the sweep only archives PRs that are already merged, it never merges, so the gate has nothing to act on here.

**Sweep posture:** per-change failures **log the error and continue to the next change**. A failure before the archive step (`sync pull-failed`, `archive script-error`) or a `cleanup` failure retries cleanly next pass. A `sweep-failed` at `render-change-links` with reason **`skipped-publish`** (the renderer itself exited non-zero) — or at `terminal-publish` — **does** abandon the remainder of this change's close-out, leaving it **archived but its terminal record unpublished**, invisible to future detection (which only scans `active/*.md`); that's the one case needing a manual `docket.sh terminal-publish --id <id> --enabled true` follow-up. Reason **`commit-failed`** or **`push-failed`** (step 6a, change 0075) is instead **report-and-continue**: the close-out already completed (`terminal-publish` and `cleanup` ran; the pass still emits `swept`/`harvest`) — only the cosmetic `## Artifacts` block is stale, self-healing on the next pass. So never let a `sweep-failed render-change-links` line alone imply the record is unpublished — check the reason, and cross-check whether the same pass emitted `swept`/`harvest` for that id. Under `terminal_publish: false` the publish leg is a no-op that cannot fail, but `render-change-links` can still fail — the follow-up there is a manual re-render, not a publish. This is **deliberately divergent from `docket-finalize-change`'s** abort-and-report posture — the sequence is shared, the failure posture is not. A failed `docket.sh render-change-links` follow-on skips publish (a stale `## Artifacts` block is never published).

### Learnings

Runs on a **full pass only** — never under `--board-only` — gated FIRST on `learnings.enabled` (default `true`): disabled means the renderer is never invoked, no advisories are computed, and every existing `learnings/` file is left byte-untouched — a read/write gate, never a purge. Enabled, the pass self-heals the derived index (`learnings/README.md`, rendered by `render-learnings-index.sh` — contract: `scripts/render-learnings-index.md`) with the identical write shape the board uses: render, diff, commit only when the bytes actually changed, push with the bounded rebase-retry (regenerating through the renderer on a conflict, never a hand-merge). It then surfaces the two visibility-only advisories: `learnings over-cap — needs curation` once active findings (`retained` + `candidate`; a `promoted` finding no longer counts) exceed `learnings.cap`, and `learnings promotion-pending <n> — needs you` whenever any finding carries `promotion_state: candidate`. Populating `learnings/` is the harvest's job — see the `harvest <id> <path>` hook under *Judgment follow-ups* above.

### Health checks

Flag the following (do not auto-fix unless asked). Five mechanical, git-only, warn-only checks run via `docket.sh board-checks` against the shared dependency-resolution pass:

```
"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh board-checks --changes-dir <metadata working tree>/<changes_dir> \
  --metadata-branch <metadata_branch> --integration-branch origin/<integration_branch>
```

- **`broken-spec`** — `spec:` set (and not `trivial: true`) but the path does not resolve on the metadata branch.
- **`broken-plan-results`** — a `done` change's set `plan:`/`results:` does not resolve on the integration branch (link rot). An `implemented` change is never flagged — those files legitimately still live on the unmerged feature branch.
- **`dep-cycle`** — a `depends_on` cycle; one finding per change in the loop.
- **`stale-in-progress`** — an `in-progress` change whose feature branch exists but has had no commit in 3 days.
- **`merge-gate-stall`** — a build-ready change whose worst-unmet dependency is stuck at `implemented` (reason `"needs your merge"`).

Two judgment checks stay in-model, on top of the script: `blocked_by:` re-examination and `github` mirror reachability (see *Judgment follow-ups* above) — both warn-only, never auto-fix.
