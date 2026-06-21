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

## Where the board, sweep, and checks operate

Resolve config + the bootstrap verdict deterministically: `eval "$("${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket-config.sh --export)"` (fail-closed; read-only). Act on `BOOTSTRAP` — `PROCEED` to continue; `STOP_MIGRATE` to refuse-and-point at `migrate-to-docket.sh`; `CREATE_ORPHAN` to opt into `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket-config.sh --bootstrap` (fresh repo only).

All three passes read and write in the **metadata working tree** on `metadata_branch`, pushed to its remote immediately. In `docket`-mode that tree is the persistent `.docket/` worktree parked on `docket` — ensure it (state-specific create per the convention's Branch model, idempotent) and **sync it to `origin/docket` before any read** (`git -C .docket fetch origin docket && git -C .docket pull --rebase origin docket`); pushes target `origin/docket`. In single-branch/`main`-mode this degrades to the primary working tree on the integration branch (no `.docket/`): `git pull --rebase` and push on `origin/<metadata_branch>` (which equals `origin/<integration_branch>` there). The passes below say "`.docket/`" / "`origin/docket`" for the common (`docket`-mode) case; read those as the metadata working tree / `origin/<metadata_branch>` in `main`-mode.

## Board

The Board pass renders **each surface listed in `board_surfaces`** (config; default `[inline]`). It scans `<changes_dir>/active/` and `archive/`, parses each file's frontmatter, and applies the dependency-resolution pass above **once**, then drives the enabled surfaces from that single result. `board_surfaces: []` makes the whole pass a no-op (the change files remain the source of truth); an unknown token is warned-and-ignored; a non-GitHub remote drops `github`.

**`inline` surface** (the default). Regenerate `BOARD.md` by invoking the deterministic
`"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/render-board.sh --changes-dir <metadata working tree>/<changes_dir> > <metadata working
tree>/<changes_dir>/BOARD.md` (in `docket`-mode the metadata working tree is `.docket/`; pass
`--repo <owner>/<repo>` so `pr:` cells hyperlink). The script owns *how* to render — it reproduces
the *Structure* below byte-for-byte from the change files, offline (no `gh`, no network) and
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

### Structure (in order)

`render-board.sh` is the executable source of this structure (change 0022); the prose
below documents what it emits and the dependency-resolution it shares with the sweep and the
health checks.

1. **Count summary** — one line, e.g.:

   `**12 changes** — 🟢 2 in progress · 🟡 3 proposed · 🔴 1 blocked · ⚪ 1 deferred · 🔵 2 implemented · ✅ 3 done`

2. **Emoji-grouped `##` sections** per status with live counts in the heading, e.g. `## 🟢 In progress (2)`. Omit a section if its count is zero.

3. **Per-group tables** with columns relevant to the status (id · title · priority chip · spec/pr links · readiness). Readiness rules:
   - A dependency-waiting change renders **⏳ waiting on #N — not yet built** or **⏳ waiting on #N — needs your merge** (from the shared pass); it is never shown as build-ready, and this **takes precedence over a missing spec** (a stub that also waits renders as waiting).
   - A `proposed` change that is **not** dependency-waiting, with no spec and not `trivial: true`, renders **needs-brainstorm** — unless its body carries an `## Auto-groom blocked` section, in which case it renders **auto-groom blocked — needs you** (the autonomous groomer abstained; a human must resolve or re-arm it).

4. **Mermaid dependency graph** built from `depends_on` edges; `done` nodes tinted with `classDef done fill:#d3f9d8;`. Renders on GitHub and Markhaus (a Markdown viewer that bundles Mermaid); degrades gracefully in plain CommonMark.

5. **Collapsible `<details>` archive section** for both terminal states (done and killed).

### Example — abbreviated rendered `BOARD.md`

````markdown
# Backlog

**5 changes** — 🟢 1 in progress · 🟡 1 proposed · 🔵 1 implemented · ✅ 1 done · 🗑️ 1 killed

## 🟢 In progress (1)
| # | Title | Priority | Spec | Branch |
|---|-------|----------|------|--------|
| [0007](active/0007-quicklook-interactions.md) | Quick Look interactions | `high` | [spec](../superpowers/specs/2026-05-30-quicklook.md) | `feat/quicklook-interactions` |

## 🟡 Proposed (1)
| # | Title | Priority | Readiness |
|---|-------|----------|-----------|
| [0009](active/0009-export-pdf.md) | Export to PDF | `medium` | ⏳ waiting on #7 — not yet built |

## 🔵 Implemented — awaiting merge (1)
| # | Title | Priority | PR |
|---|-------|----------|----|
| [0008](active/0008-onboarding-tour.md) | Onboarding tour | `medium` | [#142](https://github.com/o/r/pull/142) |

```mermaid
graph TD
  0007 --> 0009
  0004:::done
  classDef done fill:#d3f9d8;
```

<details><summary>✅🗑️ Archive — done + killed (1)</summary>

| # | Title | Merged |
|---|-------|--------|
| [0004](archive/2026-05-30-0004-quicklook-extension.md) | Quick Look extension | 2026-05-30 |

</details>
````

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

   c. **Archive (delegated to `archive-change.sh`).** Author the commit message, determine whether a `results:` file exists (it arrived via the PR merge → pass `--results <path>`), then invoke the archive primitive — the same call `docket-finalize-change`'s step 3 uses:
      ```
      "${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/archive-change.sh --changes-dir .docket/<changes_dir> --id <id> --outcome done --date <merge-date> [--results <path>] --message "<msg>"
      ```
      The script owns the dated `archive/<merge-date>-<id>-<slug>.md` move (with reuse-existing-file idempotency, including across a day boundary), the `status: done` / `updated: <merge-date>` / `results:` writes, the **change-file-only** commit, and the push-with-rebase-retry on `origin/docket`, plus fail-closed self-verification the old hand-rolled path lacked. **Per-change failure posture (below): trust the exit code — `0` ⇒ archived; non-zero ⇒ log and move to the next change.**

   d. **Re-render the `## Artifacts` block (follow-on commit, before publish).** After `archive-change.sh` returns `0`, regenerate the block on the **archived** file: `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/render-change-links.sh --change-file .docket/<changes_dir>/archive/<merge-date>-<id>-<slug>.md --adrs-dir .docket/<adrs_dir>` (plan/results re-point to the integration branch at `done`; the renderer is the sole writer of the block). Commit this as a **separate follow-on metadata commit** on `docket` and **push `origin/docket`** — it must land on `origin/docket` **before** the publish below, because `terminal-publish.sh` copies the change file *from `origin/docket`*; publishing before the re-render lands would copy the stale block (the #0035 footgun). It is a separate commit, never bundled into the script-owned archive commit, which must stay change-file-only and byte-identical across concurrent archivers (the determinism invariant).

   e. **Publish the terminal record (`docket`-mode).** Reached **only if the step-d re-render commit landed on `origin/docket`**: `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/terminal-publish.sh --id <id> --outcome done --integration-branch <integration_branch> --metadata-branch docket --changes-dir <changes_dir> --adrs-dir <adrs_dir> --message "<msg>"` — copies the now-re-pointed terminal records from `origin/docket` onto the integration branch. The script's reuse-existing-file idempotency makes a sweep racing `docket-finalize-change` on the same change a safe no-op. In `main`-mode the metadata working tree *is* the integration branch, so the step-c archive commit is itself the terminal record and `terminal-publish.sh` is a no-op (its own mode-guard fires); the step-d renderer still runs once to re-point the block in place.

   **Per-change failure posture (steps c–e).** The sweep is a **bulk best-effort safety net** run unattended — its other steps (cleanup `g`, harvest `h`, the integration sync) are already explicitly log-and-continue. The three delegated archive steps take the **same** posture: on a non-zero exit from `archive-change.sh`, the renderer follow-on commit/push, **or** `terminal-publish.sh`, **log it, abandon the remainder of this change's close-out, and continue to the next change.** "Abandon the remainder" carries the #0035 guard — a failed archive skips the re-render and publish, and a **failed re-render commit skips publish**, so a stale `## Artifacts` block is never published. The next sweep self-heals idempotently (each script is a reuse-existing / byte-identical no-op on the already-done portion and re-attempts the rest). This is **deliberately divergent from `docket-finalize-change`'s step 3**, whose `non-zero ⇒ abort-and-report` fits a single-change close-out, not a janitor draining N changes — the sequence is shared, the failure posture is not.

   g. **Remove the merged feature branch + worktree**, provenance-guarded: invoke `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/cleanup-feature-branch.sh --slug <slug>` — only removes a worktree resolving under `.worktrees/<slug>`, never the `.docket/` metadata worktree or any out-of-tree path; the same guard as `superpowers:finishing-a-development-branch`. Trust the exit code.

   h. **Harvest learnings (best-effort)** — invoke the harvest procedure (the *Harvest learnings* step in `docket-finalize-change`, its single source) for the swept change. Its idempotency probe makes a sweep racing `docket-finalize-change` a safe no-op. Best-effort like the board: log and continue on failure — never abort the sweep for it.

**Sync the integration checkout (best-effort).** Once after all swept changes are archived — not once per swept change — run `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/sync-integration-branch.sh --integration-branch <integration_branch>`. Same best-effort, FF-only helper finalize runs (change 0029) — it fast-forwards the clone's local `<integration_branch>` checkout so the symlinked skills track the just-swept merges. Omitting it would leave swept close-outs stale. Best-effort like the board: never aborts the sweep; a no-op in `main`-mode.

**Determinism invariant.** Two agents both reading `implemented` produce a byte-identical add (change-file-only, UTC merge date, no `now()`). The loser's `pull --rebase` resolves cleanly because both adds are identical. `BOARD.md` is regenerated separately, never hand-merged.

**Note:** This archive procedure is **identical** to `docket-finalize-change`'s per-change archive — same UTC merge date, same change-file-only commit, same reuse-existing-file idempotency, same terminal-publish invocation. Both skills describe the same operation; they must not diverge.

## Health checks

Flag the following (do not auto-fix unless asked). Board and health checks share the one dependency-resolution pass computed above — it is not re-run (it is now literally `resolve_deps`, run inside the script below).

**Mechanical checks → `board-checks.sh` (change 0023).** The five mechanical checks are deterministic git probes, so they live in a script, not in prose. Invoke:

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
- **Board/source drift** — runs **per enabled surface** (skipped entirely when `board_surfaces: []`). For `inline`: render the board in-memory from the change files (reusing the shared dependency-resolution pass) and compare it to the committed `BOARD.md`; if any change's rendered status or placement disagrees, **warn** naming the change(s) (a writer skipped the board-refresh invariant). For `github`: warn on a change carrying an `issue:` whose mirror is unreachable (the upsert is best-effort and self-heals; this is only a visibility flag). Like the other checks it only warns — it never auto-fixes; the Board pass in this same `docket-status` run re-renders the enabled surfaces and heals the drift regardless. A best-effort refresh is allowed to lose a race. (Retiring/downgrading this `inline` drift check once rendering is deterministic is change **0024**.)
