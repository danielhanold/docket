---
name: docket-status
description: Use when you want to see or refresh the docket backlog — what is proposed, in progress, blocked, implemented, or done — by regenerating the BOARD.md board, sweeping merged changes to done, or running health checks for stale claims, broken spec/plan/results links, and dependency stalls.
---

# docket-status — the board & janitor

## Overview

`docket-status` gives you a queryable, up-to-date view of the backlog and keeps it clean. It has three jobs: render `BOARD.md` from the change files, sweep any `implemented` change whose PR merged into the archive, and run health checks that flag stale claims, broken links, and dependency stalls. The change files are the source of truth; `BOARD.md` is always generated output, never edited by hand.

## When to use

- You want to know what is done, what is next, or what is stuck.
- A PR was merged via the GitHub button (not via `docket-finalize-change`) and the board is stale.
- `docket-implement-next` calls this at step 0 as a self-cleaning safety net before selecting the next change.
- You suspect spec, plan, or results links are stale or broken.
- The board shows a change as waiting but you think the blocker has cleared.
- You want to see the Mermaid dependency graph to understand build order.

## Convention (load first — blocking)

Before anything else in this skill, invoke the `docket-convention` skill via the Skill tool — unless it was already invoked earlier in this session and its content is in context. Everything below uses its vocabulary (build-ready, metadata working tree, terminal-publish, the `DOCKET`/`LIVE` bootstrap probes, …) without redefinition; no step below is executable without the convention loaded.

## Shared dependency-resolution pass

Computed once per `docket-status` run; both the board and the health checks consume the same result — never recomputed.

For every change, resolve each id in its `depends_on`:

- Target status `done` → **satisfied**
- Target status `implemented` (PR open, not yet merged) → **NOT satisfied**; reason = `"needs your merge"`
- Target any other active status, or id missing → **NOT satisfied**; reason = `"not yet built"`

A change with all deps satisfied (or none) is **dependency-clear**. A change with at least one unsatisfied dep is **dependency-waiting**, carrying the worst unmet reason for display (`"needs your merge"` > `"not yet built"`).

Readiness cells the board renders from this pass: a dependency-waiting change shows **⏳ waiting on #N — not yet built** or **⏳ waiting on #N — needs your merge** (never build-ready; waiting takes precedence over a missing spec). A `proposed` change that is not waiting, has no spec, and is not `trivial: true` shows **needs-brainstorm** — or **auto-groom blocked — needs you** when its body carries an `## Auto-groom blocked` section.

## Where the board, sweep, and checks operate

Run the convention's *Step-0 preamble* — config export (`eval "$("${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket-config.sh --export)"`), `BOOTSTRAP` verdict, metadata-working-tree ensure + sync. All three passes read and write in that tree on `metadata_branch`, pushed to its remote immediately; the passes below say "`.docket/`" / "`origin/docket`" for the common (`docket`-mode) case.

## Board

The Board pass renders **each surface listed in `board_surfaces`** (config; default `[inline]`). It scans `<changes_dir>/active/` and `archive/`, parses each file's frontmatter, and applies the dependency-resolution pass above **once**, then drives the enabled surfaces from that single result. `board_surfaces: []` makes the whole pass a no-op (the change files remain the source of truth); an unknown token is warned-and-ignored; a non-GitHub remote drops `github`.

**`inline` surface** (the default). Regenerate `BOARD.md` by invoking the deterministic
`"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/render-board.sh --changes-dir <metadata working tree>/<changes_dir> > <metadata working
tree>/<changes_dir>/BOARD.md` (in `docket`-mode the metadata working tree is `.docket/`; pass
`--repo <owner>/<repo>` so `pr:` cells hyperlink). The script owns *how* to render — its contract
(`scripts/render-board.md`) documents the emitted structure, offline (no `gh`, no network) and
deterministically (same change files ⇒ identical bytes); the skill owns *when* to render and the
commit discipline. `BOARD.md` is the **live planning view and stays on `docket`** — it is **never**
published to the integration branch (the one metadata file terminal-publish never copies). **Never
hand-edit `BOARD.md`, never merge it.** Commit it and push `origin/docket`. On a `pull --rebase`
conflict in `BOARD.md` during the push loop, **regenerate, never 3-way merge**: discard the conflict
markers (either side — they invert under rebase anyway), **re-run `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/render-board.sh`** to
rebuild `BOARD.md` from the change files, `git add` it, then `git rebase --continue`. Dropping
`inline` forfeits this offline-safe view — the documented tradeoff of a GitHub-only board.

**`github` surface** — the one-way Issues + Projects v2 mirror (per the convention's *GitHub board mirror* definition; mechanics in `skills/docket-convention/github-board-mirror.md`). Invoke the deterministic `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/github-mirror.sh` against the change files, **best-effort**: it needs network + `gh` auth, it never aborts the pass, and it self-heals next run. Point `--changes-dir` at the **metadata working tree** (`.docket/<changes_dir>` in `docket`-mode) — never the integration-branch checkout, where `active/` is pruned (the script warns if it detects that wrong tree, but the run still misses the live backlog). The script upserts one issue per change (keyed on `issue:`), reconciles the `docket:` label set, sets close state/reason, and best-effort-syncs Projects; on a fresh mint it prints `issue-minted <id> <number>` lines — record each into the change file's `issue:` on `metadata_branch` (the script does no git writes). **Projects auto-create is opt-in:** when `github_project` is unset, pass `--auto-create-project` (owner defaults to the integration repo's owner; override with `--project-owner`) — the script mints a private board, prints `project-minted <owner> <number>`, which you record as `github_project: {owner, number}` in `.docket.yml` on the default branch (the first-sync write-back); when `github_project` is set, pass it as `--project <owner>/<number>` instead. Both metadata writes follow the normal push discipline; a `gh`/network failure logs and continues.

**No churny timestamp.** Counts convey freshness; a generated-at line would churn on every run.

## Merge sweep

The bulk safety net: sweep every `implemented` change whose PR has merged into the archive. Runs automatically at `docket-implement-next` step 0, and whenever you invoke `docket-status` explicitly after merging via the GitHub button. The sweep is a **terminal-transition driver** — like `docket-finalize-change`, on each swept change it both archives on `metadata_branch` and, in `docket`-mode, publishes the terminal record onto the integration branch.

> **Note (the gate is finalize-only).** The rebase-onto-base + re-run-tests gate (change 0015) lives in `docket-finalize-change`'s merge step — the only place docket itself merges. The sweep **only archives PRs that are already merged**; it never performs a merge, so a pre-merge gate has nothing to act on here. A PR merged via the GitHub button bypasses the gate by nature — outside docket's control.

For each `implemented` change:

1. **Determine its PR** — use `pr:`; if empty, fall back to `gh pr list --head feat/<slug>`.
2. **Ask gh whether that PR is merged.** Not merged → skip.
3. **Merged → ARCHIVE IDEMPOTENTLY:**

   a. `git pull --rebase` on `metadata_branch` (in `docket`-mode, `git -C .docket pull --rebase origin docket`); re-read `status`.
      Already `done` (or already under `archive/`) → no-op, continue.

   b. **Compute the merge date in UTC** — use `gh`'s `mergedAt`, or
      `TZ=UTC git show -s --date=format-local:%Y-%m-%d <merge-sha>`. Never `now()`.

   c–e. **Close out via the shared sequence** — run the convention's terminal close-out
   (**read `../docket-convention/references/terminal-close-out.md` now — blocking**) with
   `--outcome done` and the UTC merge date from step b, through its **cleanup** step (steps 1–4:
   archive, re-render, terminal-publish onto the integration branch in `docket`-mode, cleanup —
   the reference owns invocations, ordering, and the `main`-mode degradation). Its step 5 (board)
   is covered by this same `docket-status` run's own Board pass — do not re-render per change.

   **Sweep posture (steps c–e):** the sweep is a bulk janitor draining N changes — on any non-zero
   exit, **log it, abandon the remainder of this change's close-out, and continue to the next
   change**; the next sweep self-heals idempotently. A failed
   `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/render-change-links.sh` follow-on
   commit **skips publish** (a stale `## Artifacts` block is never published). This posture is
   **deliberately divergent from `docket-finalize-change`'s** abort-and-report — the sequence is
   shared, the failure posture is not. Determinism: concurrent archivers produce byte-identical
   change-file-only commits; `BOARD.md` is regenerated separately, never hand-merged.

   h. **Harvest learnings (best-effort)** — invoke the harvest procedure (the *Harvest learnings* step in `docket-finalize-change`, its single source) for the swept change. Its idempotency probe makes a sweep racing `docket-finalize-change` a safe no-op. Best-effort like the board: log and continue on failure — never abort the sweep for it.

**Sync the integration checkout (best-effort).** Once after all swept changes are archived — not once per swept change — run `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/sync-integration-branch.sh --integration-branch <integration_branch>`. Same best-effort, FF-only helper finalize runs — it fast-forwards the clone's local `<integration_branch>` checkout so the symlinked skills track the just-swept merges. Omitting it would leave swept close-outs stale. Best-effort like the board: never aborts the sweep; a no-op in `main`-mode.

## Health checks

Flag the following (do not auto-fix unless asked). Board and health checks share the one dependency-resolution pass computed above — it is not re-run (it is now literally `resolve_deps`, run inside the script below).

**Mechanical checks → `board-checks.sh`.** The five mechanical checks are deterministic git probes, so they live in a script, not in prose. Invoke:

```
"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/board-checks.sh --changes-dir <metadata working tree>/<changes_dir> \
  --metadata-branch <metadata_branch> --integration-branch origin/<integration_branch>
```

(in `docket`-mode the metadata working tree is `.docket/`, so `--changes-dir .docket/<changes_dir> --metadata-branch docket`; resolve `plan:`/`results:` against `origin/<integration_branch>` — those files never live on `docket`. In `main`-mode pass `--metadata-branch <integration_branch> --integration-branch origin/<integration_branch>`; both link checks then resolve on the same content). The script sources the shared helper, calls `resolve_deps` once, and prints one finding per line on stdout — TAB-separated `<check-id>\t<change-id>\t<message>`, `check-id` ∈ `{broken-spec, broken-plan-results, dep-cycle, stale-in-progress, merge-gate-stall}`. **Surface each finding line as a warning.** A clean tree prints nothing. The script is **git-only** (no `gh`, no network) and **warn-only** (it never auto-fixes); `--strict` makes it exit non-zero on any finding, for a future CI gate. What the five cover:

- **`broken-spec`** — `spec:` set (and not `trivial: true`) but the path does not resolve on the metadata branch.
- **`broken-plan-results`** — a `done` change's set `plan:`/`results:` does not resolve on the integration branch (link rot). An `implemented` change is never flagged — those files legitimately still live on the unmerged feature branch.
- **`dep-cycle`** — a `depends_on` cycle; one finding per change in the loop.
- **`stale-in-progress`** — an `in-progress` change whose feature branch exists but has had no commit in **3 days** (the current fixed default). A just-claimed change with a `branch:` value but no branch yet created is **not** stale.
- **`merge-gate-stall`** — a build-ready change whose worst-unmet dependency is stuck at `implemented` (reason `"needs your merge"`), naming the blocking `#N`. Surfaces that a single merge unblocks downstream work.

**Model-driven checks (judgment — stay in-model, on top of the script):**

- **`blocked` changes whose blocker may have cleared** — re-examine `blocked_by:` free text; flag if the referenced issue/PR/event appears resolved. Judgment, not a git probe — never scripted.
- **`github` mirror reachability** — runs only when `board_surfaces` includes `github` (skipped otherwise): warn on a change carrying an `issue:` whose mirror is unreachable (the upsert is best-effort and self-heals; this is only a visibility flag). Like the other checks it only warns — it never auto-fixes, and a best-effort refresh is allowed to lose a race.
