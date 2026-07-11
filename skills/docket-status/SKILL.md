---
name: docket-status
description: Use when you want to see or refresh the docket backlog — what is proposed, in progress, blocked, implemented, or done — by regenerating the BOARD.md board, sweeping merged changes to done, or running health checks for stale claims, broken spec/plan/results links, and dependency stalls.
---

# docket-status — the board & janitor

## Overview

`docket-status` gives you a queryable, up-to-date view of the backlog and keeps it clean. It has three jobs: render `BOARD.md` from the change files, sweep any `implemented` change whose PR merged into the archive, and run health checks that flag stale claims, broken links, and dependency stalls. The change files are the source of truth; `BOARD.md` is always generated output, never edited by hand. Since change 0058 all of this is sequenced by the deterministic orchestrator (contract: `scripts/docket-status.md`) — this skill's job is to invoke it, trust its exit code, surface its report, and apply the handful of judgment calls the script deliberately leaves in-model.

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

The script owns the mechanics of what it renders, sweeps, and checks — see `scripts/docket-status.md` for the full 7-step sequence, its output-line shapes, and its failure postures. Surface its report to the user in human terms (what's on the board, what got swept, what health checks flagged) rather than pasting the raw line-oriented output. Health checks stay warn-only — do not auto-fix findings unless the user explicitly asks.

## Judgment follow-ups (stay in-model — the script does not do these)

Drive these off the report lines `docket-status.sh` emits; skip a category entirely if no matching line appeared.

- **`harvest <id> <path>` lines** — for each, run the harvest-learnings procedure (the *Harvest learnings* step in `docket-finalize-change`'s SKILL.md is its single source — invoke it by reference, do not reimplement it here). Best-effort: log and continue on failure, never abort the rest of the pass for it.
- **`judgment blocked <id> <text>` lines** — re-examine that change's `blocked_by:` free text; flag to the user if the referenced issue/PR/event appears resolved. This is judgment, not a git probe — never scripted.
- **`minted issue <id> <n>` / `minted project <owner> <n>` lines** — write the value back into the change file (`issue:`) or `.docket.yml` (`github_project: {owner, number}`) on `metadata_branch`, following normal push discipline (pull --rebase, commit, push).
- **`github` mirror reachability** — only when `board_surfaces` includes `github`: warn on a change carrying an `issue:` whose mirror looks unreachable. Best-effort visibility flag, like the other checks — never auto-fix.

## Final summary

Close with a short human-facing summary: board state (counts/highlights, not the raw file), what was swept to done (if anything), and any health-check findings or judgment flags raised above. Point the user at `BOARD.md` (or the GitHub mirror, if enabled) for the full picture rather than reproducing it inline.

## Reference: what the board, sweep, and checks mean

The mechanics below live entirely in the orchestrator (contract: `scripts/docket-status.md`) — this is a compact map so a reader knows what the report lines refer to, not a restatement of how they work.

### Board

Renders each surface in `board_surfaces` (config; default `[inline]`) from the same one dependency-resolution pass, computed once. Readiness cells: a dependency-waiting change shows **⏳ waiting on #N — not yet built** or **⏳ waiting on #N — needs your merge**; a `proposed` change with no spec, not `trivial: true`, and not waiting shows **needs-brainstorm** — or **auto-groom blocked — needs you** when its body carries an `## Auto-groom blocked` section.

`inline` regenerates `BOARD.md` deterministically via `/render-board.sh` (contract: `scripts/render-board.md`) and commits/pushes it to `metadata_branch`. **Never hand-edit `BOARD.md`, never 3-way merge it**; on a rebase conflict, regenerate via `/render-board.sh` and continue. `BOARD.md` is the live planning view and stays on `docket` — never published to the integration branch.

`github` is the one-way Issues + Projects v2 mirror (`github-mirror.sh`, mechanics in `skills/docket-convention/github-board-mirror.md`), best-effort — runs only when `board_surfaces` includes `github`; a fresh mint prints `issue-minted`/`project-minted` lines to record back into the change file / `.docket.yml`.

### Merge sweep

The bulk safety net: every `implemented` change whose PR has merged gets archived on `metadata_branch`, its terminal record published (via `terminal-publish` onto the `integration_branch` in `docket`-mode), and its branch cleaned up, chaining the same close-out sequence (`terminal-close-out.md`) `docket-finalize-change` uses. Runs automatically at `docket-implement-next` step 0 and on any explicit non-`--board-only` `docket-status` invocation.

The rebase-onto-base + re-run-tests gate lives in `docket-finalize-change`'s merge step and is **finalize-only** — the sweep only archives PRs that are already merged, it never merges, so the gate has nothing to act on here.

**Sweep posture:** per-change failures **log the error, abandon the remainder of this change's close-out, and continue to the next change**; the next sweep self-heals idempotently **only for a failure before the archive step** (`sync pull-failed` or `archive script-error`) or a `cleanup` failure — those retry cleanly next pass. A `sweep-failed` at `render-change-links` or `terminal-publish` leaves the change **archived but its terminal record unpublished**, and no later sweep resumes it (detection only scans `active/*.md`); it needs a manual `terminal-publish.sh --id <id>` follow-up. This is **deliberately divergent from `docket-finalize-change`'s** abort-and-report posture — the sequence is shared, the failure posture is not. A failed `/render-change-links.sh` follow-on skips publish (a stale `## Artifacts` block is never published).

### Health checks

Flag the following (do not auto-fix unless asked). Five mechanical, git-only, warn-only checks run via `/board-checks.sh` against the shared dependency-resolution pass:

```
"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/board-checks.sh --changes-dir <metadata working tree>/<changes_dir> \
  --metadata-branch <metadata_branch> --integration-branch origin/<integration_branch>
```

- **`broken-spec`** — `spec:` set (and not `trivial: true`) but the path does not resolve on the metadata branch.
- **`broken-plan-results`** — a `done` change's set `plan:`/`results:` does not resolve on the integration branch (link rot). An `implemented` change is never flagged — those files legitimately still live on the unmerged feature branch.
- **`dep-cycle`** — a `depends_on` cycle; one finding per change in the loop.
- **`stale-in-progress`** — an `in-progress` change whose feature branch exists but has had no commit in 3 days.
- **`merge-gate-stall`** — a build-ready change whose worst-unmet dependency is stuck at `implemented` (reason `"needs your merge"`).

Two judgment checks stay in-model, on top of the script: `blocked_by:` re-examination and `github` mirror reachability (see *Judgment follow-ups* above) — both warn-only, never auto-fix.
