---
name: docket-status
description: Use when you want to see or refresh the docket backlog — what is proposed, in progress, blocked, implemented, or done — by regenerating the BOARD.md board, sweeping merged changes to done, or running health checks for stale claims, broken spec/plan/results links, and dependency stalls.
---

# docket-status — the board & janitor

## Overview

`docket-status` gives you a queryable, up-to-date view of the backlog and keeps it clean. It has three jobs: render `BOARD.md` from the change files, sweep any `implemented` change whose PR merged into the archive, and run health checks that flag stale claims, broken links, and dependency stalls. The change files are the source of truth; `BOARD.md` is always generated output, never edited by hand. Since change 0058 all of this is sequenced by the deterministic `scripts/docket-status.sh` orchestrator (contract: `scripts/docket-status.md`) — this skill's job is to invoke it, trust its exit code, surface its report, and apply the handful of judgment calls the script deliberately leaves in-model.

## When to use

- You want to know what is done, what is next, or what is stuck.
- A PR was merged via the GitHub button (not via `docket-finalize-change`) and the board is stale.
- `docket-implement-next` calls this at step 0 as a self-cleaning safety net before selecting the next change.
- You suspect spec, plan, or results links are stale or broken.
- The board shows a change as waiting but you think the blocker has cleared.
- You want to see the Mermaid dependency graph to understand build order.

## Convention (load first — blocking)

Before anything else in this skill, invoke the `docket-convention` skill via the Skill tool — unless it was already invoked earlier in this session and its content is in context. Everything below uses its vocabulary (build-ready, metadata working tree, terminal-publish, the `DOCKET`/`LIVE` bootstrap probes, …) without redefinition; no step below is executable without the convention loaded.

## Step 0 — config

Run the convention's *Step-0 preamble*: `eval "$("${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket-config.sh --export)"`. `docket-status.sh` re-derives and re-checks this itself (bootstrap gate, metadata-worktree sync), but the eval here gives you `$DOCKET_SCRIPTS_DIR` and the other exported vars for the rest of this skill.

## Mode choice

- **The user only wants to *see* the backlog** (no explicit refresh requested, nothing merged recently that you know of) ⇒ run the board-only pass: `--board-only`.
- **Everything else** — an explicit refresh/cleanup request, `docket-implement-next`'s step-0 safety net, or a post-merge cleanup after a PR merged via the GitHub button ⇒ run the full pass (no flag): board + merge sweep + health checks + judgment lines + integration sync.

## Run the orchestrator

```
"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket-status.sh [--board-only]
```

Trust its exit code: `0` is the pass completing (findings, `sweep-failed`, `sweep-skipped`, `board *-failed`, and `judgment` lines on stdout are normal outcomes, not errors); non-zero is a hard error (config export failure, an unusable `BOOTSTRAP` verdict, an unusable metadata worktree, or a bad CLI argument) — surface the stderr diagnostic and stop rather than improvising a fix.

The script owns the mechanics of what it renders, sweeps, and checks — see `scripts/docket-status.md` for the full 7-step sequence, its output-line shapes, and its failure postures. Surface its report to the user in human terms (what's on the board, what got swept, what health checks flagged) rather than pasting the raw line-oriented output.

## Judgment follow-ups (stay in-model — the script does not do these)

Drive these off the report lines `docket-status.sh` emits; skip a category entirely if no matching line appeared.

- **`harvest <id> <path>` lines** — for each, run the harvest-learnings procedure (the *Harvest learnings* step in `docket-finalize-change`'s SKILL.md is its single source — invoke it by reference, do not reimplement it here). Best-effort: log and continue on failure, never abort the rest of the pass for it.
- **`judgment blocked <id> <text>` lines** — re-examine that change's `blocked_by:` free text; flag to the user if the referenced issue/PR/event appears resolved. This is judgment, not a git probe — never scripted.
- **`minted issue <id> <n>` / `minted project <owner> <n>` lines** — write the value back into the change file (`issue:`) or `.docket.yml` (`github_project: {owner, number}`) on `metadata_branch`, following normal push discipline (pull --rebase, commit, push).
- **`github` mirror reachability** — only when `board_surfaces` includes `github`: warn on a change carrying an `issue:` whose mirror looks unreachable. Best-effort visibility flag, like the other checks — never auto-fix.

## Final summary

Close with a short human-facing summary: board state (counts/highlights, not the raw file), what was swept to done (if anything), and any health-check findings or judgment flags raised above. Point the user at `BOARD.md` (or the GitHub mirror, if enabled) for the full picture rather than reproducing it inline.

## Reference: what the board, sweep, and checks mean

The mechanics below live entirely in `scripts/docket-status.sh` (contract: `scripts/docket-status.md`) — this is a short map so a reader knows what the report lines refer to, not a restatement of how they work.

- **Board.** Renders each surface in `board_surfaces` (config; default `[inline]`) from the same one dependency-resolution pass, computed once. `inline` regenerates `BOARD.md` deterministically via `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/render-board.sh` (contract: `scripts/render-board.md`) and commits/pushes it to `metadata_branch` — **never hand-edit `BOARD.md`, never 3-way merge it**; on a rebase conflict, regenerate and continue. `github` is the one-way Issues + Projects v2 mirror (`github-mirror.sh`), best-effort.
- **Merge sweep.** The bulk safety net: every `implemented` change whose PR has merged gets archived, its terminal record published (in `docket`-mode), and its branch cleaned up, chaining the same ADR-0035 close-out scripts `docket-finalize-change` uses. Runs automatically at `docket-implement-next` step 0 and on any explicit non-`--board-only` `docket-status` invocation. Per-change failures log and move on; the next sweep self-heals idempotently.
- **Health checks.** Five mechanical, git-only, warn-only checks (`broken-spec`, `broken-plan-results`, `dep-cycle`, `stale-in-progress`, `merge-gate-stall`) run via `board-checks.sh` against the same dependency-resolution pass. Two judgment checks — `blocked_by:` re-examination and `github` mirror reachability — stay in-model; see *Judgment follow-ups* above.
