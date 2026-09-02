---
name: docket-status
description: Use when you want to see or refresh the docket backlog — what is proposed, in progress, blocked, implemented, or done — by refreshing docket state, sweeping merged changes to done, and running health checks for stale claims, broken spec/plan/results links, and dependency stalls.
context: fork
agent: docket-status
---

# docket-status — the board & janitor

## Overview

`docket-status` gives you a queryable, up-to-date view of the backlog and keeps it clean. Four jobs: **report the backlog digest** (the `backlog <status> <count>` and `change <id> <status> <readiness> <slug>` lines, plus a trailing `ready [<id> …]` build-ready queue — emitted in *every* configuration, board or no board, and **the channel you write your summary from**), refresh docket state (rendering each enabled board surface), sweep any `implemented` change whose PR merged into the archive, and run health checks (stale claims, broken links, dependency stalls). On a full pass it also self-heals the learnings index and surfaces two needs-you advisories — see *Learnings* below. The change files are the source of truth; any board is generated output, never edited by hand. All of this runs through the native `maintenance.sweep` (the mutation) and `status` (the write-free read) operations — this skill resolves each argv from the capability catalog, invokes them, keys on their typed protocol-v1 dispositions, surfaces their report, and applies the handful of judgment calls the operations deliberately leave in-model. The per-line shapes and failure postures stay documented in `scripts/docket-status.md`.

## When to use

- You want to know what is done, next, or stuck — or you suspect a stale board, stale/broken links, or a cleared blocker.
- A PR was merged via the GitHub button (not via `docket-finalize-change`) and the board is stale.

## Convention (load first — blocking)

Invoke the `docket-convention` skill via the Skill tool first — unless already invoked this session — and run its *Step-0 preamble*: capability bootstrap, then the `repository.prepare` operation with `--repo-dir <dir> --json`, validating the protocol-v1 envelope and carrying its typed context forward. Prepare enforces the bootstrap gate and syncs the metadata working tree fail-closed; its typed context gives you the resolved repo/branch/dir values the rest of this skill needs. Everything below uses the convention's vocabulary without redefinition.

## Mode choice

- **The user only wants to *see* the backlog** (no explicit refresh requested, nothing merged recently that you know of) ⇒ run the write-free read alone: the `status` operation (resolve argv from the capability catalog) with `--json`. It never merges, archives, reclaims, or renders a board.
- Implementation scope (`--scope implementation`) is the startup-preflight scope: current merged-work recovery plus reclaim gating, with independent historical cleanup retries deferred and counted in `deferred_historical_cleanups`. It is owned by the `maintenance.preflight` operation, which `docket-implement-next` runs inline at its Step 0 — not a mode of this skill. This skill's two modes are the see-only read and the explicit `--scope full` refresh/cleanup.
- **An explicit refresh/cleanup request** — or a post-merge cleanup after a PR merged via the GitHub button ⇒ run the `maintenance.sweep` operation with `--scope full --json` first (merge sweep + historical cleanup retries + health checks + judgment lines + integration sync), then read the refreshed state with the `status` operation and `--json`.

## Maintenance sweep — the merged-PR recovery mutation (only when asked)

The **read is separate from the mutation.** The human `status` read stays read-only: it
reports the backlog and never merges, archives, reclaims, or cleans up. When the caller explicitly
asks to refresh or clean up — a post-merge cleanup, an out-of-band merge to recover — run the `maintenance.sweep` operation (resolve argv from the capability catalog, scope per *Mode choice*) **before** the read, then read the refreshed state.

The `maintenance.sweep` operation pins an initial inventory, walks it in deterministic order, and reloads
fresh authority before **every** mutation. It closes out each `implemented` change whose PR merged
(stacked children before ancestors, then the root carrying its descendants), retries pending
backlink-repair and cleanup suffixes, and attempts an eligible reclaim only when `reclaim.auto` is
true. It emits one structured entry per item with a closed disposition — `applied` | `noop` |
`contended` | `blocked` | `unknown` | `failed` | `skipped`, each with its stable reason — never a
collapsed boolean. It never merges an open PR, never overrides approval, never retargets an
unauthorized child, and never edits authored results; a destructive suffix never runs after an
`unknown` prerequisite, and one item's failure never stops the independent items. Surface the
per-item report; act only on the `blocked`/`failed`/`unknown` entries a human must see.

`--scope` is a closed vocabulary. `full` — the default when omitted — is the whole worklist, including cleanup retries for every `done`/`stacked-merged` record. `implementation` is the implementation-startup preflight: it still schedules every current merged-implemented closeout, the cleanup suffix such a closeout carries in the same invocation, and the `reclaim.auto`-gated reclaims, but it does not enqueue an independent cleanup for a record that was already terminal at the pinned inventory. Those are reported as a deferred COUNT — a population deliberately not probed, never evidence anything is dirty or blocked; explicit full maintenance owns those retries, and a failed or interrupted suffix stays recoverable through `--scope full` or the targeted finalize cleanup.

## Run the pass

On a **refresh/cleanup** pass, run the mutation first, then the read; on a **see-only** pass, run the read alone. Resolve each operation's argv from the capability catalog:

```
maintenance.sweep  --scope <full|implementation> --json   # mutation, scope per Mode choice
status             --json                                  # write-free read over the refreshed state
```

Validate each protocol-v1 envelope and key on its typed **disposition**, never an exit code. The sweep emits one structured entry per item with a closed disposition (`applied` | `noop` | `contended` | `blocked` | `unknown` | `failed` | `skipped`), and the read returns the structured backlog plus any health findings. A `blocked` / `failed` / `unknown` sweep entry, or a read whose envelope carries an error disposition — a config-resolution failure, an unusable bootstrap verdict or metadata worktree, a bad argument — is a hard error: surface the diagnostic and stop rather than improvising a fix.

**Scope of this stop:** if you invoked this skill yourself — the convention's Tier A path — this
stop ends only the status role and you continue to your own next step; only an agent whose entire
assignment is this role ends its turn here.

There is **no** separate board pass to key on: every board-authoritative typed mutation re-renders `BOARD.md` in the same metadata commit as the record it reflects, so the sweep leaves the board current and a plain `status` read writes nothing. The commands own the mechanics of what they sweep and check — see `scripts/docket-status.md` for the output-line shapes and failure postures. Surface the report to the user in human terms (what's on the board, what got swept, what health checks flagged) rather than pasting the raw line-oriented output. Health checks stay warn-only — do not auto-fix findings unless the user explicitly asks.

## Completion barrier — observe the sweep to its terminal result

Starting the sweep command is not finishing it. If the shell tool's foreground window expires and the harness moves the still-running sweep to the background, that is a liveness transition, not completion — and not a failed sweep. You stay responsible for that exact task: keep the task identity the harness returned and collect that task's terminal result through the harness's native observation/wait mechanism, and never start a second shell watcher, never poll the output file's size, never sleep-and-tail, never re-run the sweep, and never return a success report while the process remains unobserved. An output file turning nonempty, metadata commits appearing on `origin/docket`, elapsed time, or some separate command succeeding are not completion signals.

Only the sweep's actual terminal protocol-v1 envelope completes the command. Validate it and every entry: `protocol_version` `1`, `operation` `maintenance.sweep`, a `scope` equal to the one you requested, and each entry's closed disposition. Retain the original structured output — a harness result handle or a task-local output artifact — and extract the compact summary plus any problem entries in one read/parse rather than reopening the full output repeatedly; stdout is the JSON document, and any progress diagnostics belong on a separate channel. Only after that terminal validation run the post-sweep read: the `status` operation (resolve argv from the capability catalog) with `--json`. A read taken after a failed or unvalidated sweep is diagnostic only — label it as diagnostic; it can never authorize selection.

The envelope's top-level `applied` means some work applied, never that every item succeeded. A `blocked`, `failed`, or `unknown` entry — or a `contended` entry on work this preflight required — is a failed preflight even under an `applied` envelope: surface it per *Read the report* below and stop, per the *Scope of this stop* rule above. Intentional policy skips (`reclaim-auto-disabled`) and genuine no-ops remain non-errors — never collapse arbitrary `skipped` reasons into success. On cancellation, request cancellation of the exact owned task where the harness supports it, then observe its termination; never broadly kill processes, abandon a watcher, or spawn a replacement sweep while the prior one may still run. If quiescence cannot be established, halt naming the exact live task identity and preserve its output — an explicit failure here beats claiming an orphan-free exit.

When a caller dispatched this skill, the final report must name: the resolved scope, the sweep's terminal envelope result, every problem entry, where the original sweep and status outputs live, and the post-sweep metadata revision (`git -C <metadata worktree dir> rev-parse HEAD`). The prose re-summary is never a second authority — the caller verifies against the originals.

## Read the report — it is the only channel you need

The report is **self-evidencing**: it always states what it did, so you never have to go looking for corroboration.

- **`board off`** — the repo sets `board_surfaces: []` and there is deliberately **no board**. This is a configuration, not a failure. Do not look for `BOARD.md`; it must not exist.
- **`backlog <status> <count>` + `change <id> <status> <readiness> <slug>` + `ready [<id> …]`** — the backlog digest, emitted in **every** configuration. **This is your backlog-state channel.** Write the summary from these lines. On a full pass the digest is taken **after** the sweep, so it already accounts for everything this pass closed out: a change on a `swept` line is counted under `backlog done` and has no `change` line of its own. Never report a swept change as still awaiting merge. `ready` is the trailing line, always emitted: the build-ready queue in selection order (priority → created → id), bare when nothing is ready.
- **learnings (deferred)** — the pass emits no learnings self-heal or advisory lines. automated learnings-index rendering, capacity, and promotion are deferred from Go v1, so the pass reads nothing and writes nothing under `learnings/` and every existing `learnings/` file stays byte-untouched. See *Learnings* below.
- **`pass ok`** — the orchestrator ran to completion. It is always the last line of a successful pass.

Two rules follow, and they are not optional:

- **A thin report is the success case, not a symptom.** An empty sweep, no health findings, and `board off` together mean a healthy, board-less repo. The pass is complete. Do **not** re-run the orchestrator, trace it, or investigate — there is nothing to find.
- **Never probe `BOARD.md`.** With the board off it must not exist; with the board on, summarize from the digest lines rather than opening the file. Reading, rendering, or hand-writing `BOARD.md` is never part of this skill's job — `board-refresh.sh` is its only writer.

## Judgment follow-ups (stay in-model — the script does not do these)

Drive these off the report lines the `maintenance.sweep` and `status` operations emit; skip a category entirely if no matching line appeared.

- **`harvest <id> <path>` lines** — for each, note the id in the pass report — automated learnings harvest is deferred from Go v1 — record or update findings by editing `learnings/` files directly. The absence of a harvest is never a sweep failure, and the pass never fabricates an empty harvest result.
- **`stacked-merged` / `promote-failed` / `stack-carried-failed` lines, or a `check stack-invalid` / `check stack-parent-killed` finding** — **read [`../docket-convention/references/stacked-changes.md`](../docket-convention/references/stacked-changes.md) now (blocking)** before explaining or acting on one: it owns what the state means, why nothing was archived, and which remedies are a human's rather than a retry's.
- **`judgment blocked <id> <text>` lines** — re-examine that change's `blocked_by:` free text; flag to the user if the referenced issue/PR/event appears resolved. This is judgment, not a git probe — never scripted.
- **`minted issue <id> <n>` / `minted project <owner> <n>` lines** — write the value back into the change file (`issue:`) or `.docket.yml` (`github_project: {owner, number}`) on `metadata_branch`, following normal push discipline (re-run the `repository.prepare` operation to re-sync, commit, push). **Stage by explicit path** — that tree is shared, so a bare `add -A` commits another agent's staged work under your message. <!-- docket:config-read-channel: write-back -->
- **`github` mirror reachability** — only when `board_surfaces` includes `github`: warn on a change carrying an `issue:` whose mirror looks unreachable. Best-effort visibility flag, like the other checks — never auto-fix.

## Final summary

Close with a short human-facing summary: backlog state (counts/highlights, read from the digest lines — never from the board file), what was swept to done (if anything), and any health-check findings or judgment flags raised above. When the `inline` board is enabled, point the user at `BOARD.md` (or the GitHub mirror, if enabled) for the full picture rather than reproducing it inline. When the report says `board off`, there is no board to point at — the digest-derived summary **is** the deliverable, and that is the intended, complete outcome.

**Dummy mode:** when `DUMMY_MODE_ENABLED` is `true` (Step-0 export), write this summary and every other part of the run's `reports` calibrated to `DUMMY_MODE_PERSONA`, per the convention's *Dummy mode* shared definition.

## Reference: what the board, sweep, and checks mean

The mechanics below live entirely in the orchestrator (contract: `scripts/docket-status.md`) — this is a compact map so a reader knows what the report lines refer to, not a restatement of how they work.

### Board

Renders each surface in `board_surfaces` (config; default `[inline]`) from the same one dependency-resolution pass, computed once. Readiness cells: a dependency-waiting change shows **⏳ waiting on #N — not yet built** or **⏳ waiting on #N — needs your merge**; a `proposed` change with no spec, not `trivial: true`, and not waiting shows **needs-brainstorm** — or **auto-groom blocked — needs you** when its body carries an `## Auto-groom blocked` section.

When `board_surfaces` includes `inline`, `board-refresh.sh` (contract: `scripts/board-refresh.md`) is the single gated writer of `BOARD.md`: it owns the surface gate and the atomic replace, wrapping the pure renderer `/render-board.sh` (contract: `scripts/render-board.md`) internally so nothing else ever touches the file; the orchestrator commits and pushes the result to `metadata_branch` only when it actually changed. This skill **never hand-edits `BOARD.md`, never hand-renders it, and never 3-way merges it**; on a rebase conflict, regenerate through `board-refresh.sh` — never a hand-merge — and continue. When `board_surfaces` omits `inline`, there is simply no board. Where present, `BOARD.md` is the live planning view and stays on `docket` — never published to the integration branch.

`github` is the one-way Issues + Projects v2 mirror (`github-mirror.sh`, mechanics in `skills/docket-convention/github-board-mirror.md`), best-effort — runs only when `board_surfaces` includes `github`; a fresh mint prints `issue-minted`/`project-minted` lines to record back into the change file / `.docket.yml`. <!-- docket:config-read-channel: write-back -->

### Merge sweep

The bulk safety net: every `implemented` change whose PR has merged gets archived on `metadata_branch` and its branch cleaned up, chaining the same close-out sequence (`terminal-close-out.md`) `docket-finalize-change` uses. terminal publication is deferred from Go v1, so no terminal record is copied onto the `integration_branch`. Runs inside the `maintenance.preflight` operation at implementation scope (`docket-implement-next` Step 0 runs that operation inline), and in full scope on any explicit refresh/cleanup invocation.

The rebase-onto-base + re-run-tests gate lives in `docket-finalize-change`'s merge step and is **finalize-only** — the sweep only archives PRs that are already merged, it never merges, so the gate has nothing to act on here.

**Sweep posture:** per-change failures **log the error and continue to the next change**. A failure before the archive step (`sync pull-failed`, `archive script-error`) or a `cleanup` failure retries cleanly next pass. A `sweep-failed` at the artifact-links render **does** abandon the remainder of this change's close-out, leaving its `## Artifacts` block stale, which no later sweep resumes (the sweep only scans `active/*.md`); the follow-up there is the exceptional-drift repair — the `repository.check` operation surfaces the `artifact-links-stale` finding and authorized `docket repository migrate` re-renders the block on the archived file. terminal publication is deferred from Go v1, so the sweep has **no publish leg**: it copies no terminal record onto the `integration_branch` and writes no `## Publish deferred` marker, though the `publish-deferred` health check below still surfaces any pre-existing marker on every later pass. Reason **`commit-failed`** or **`push-failed`** (step 6a, change 0075) — or **`blocked-wedged-tree`** (change 0247: the shared metadata worktree was mid-rebase/merge, so step 6a committed nothing) — is instead **report-and-continue**: the close-out already completed (`cleanup` ran; the pass still emits `swept`/`harvest`) — only the cosmetic `## Artifacts` block is stale, self-healing on the next pass. This is **deliberately divergent from `docket-finalize-change`'s** abort-and-report posture — the sequence is shared, the failure posture is not.

### Learnings

**Learnings (deferred).** automated learnings-index rendering is deferred from Go v1 — existing `learnings/README.md` bytes are preserved, not refreshed; automated learnings capacity and promotion are deferred from Go v1 — ledger curation is human-directed. The pass reads nothing and writes nothing under `learnings/`; an enabled `learnings.enabled` key gates *reads* elsewhere and activates no automation here.

### Health checks

Flag what the pass reports (do not auto-fix unless asked): mechanical, git-only, warn-only checks over stale claims, broken spec/plan/results links, and dependency stalls. This skill never runs the checker directly — it invokes the `maintenance.sweep` / `status` operations, which run it. The closed check-id set and each check's meaning live where they are owned: the per-check sections of `scripts/board-checks.md`, and the `check <check-id>` report-line row in `scripts/docket-status.md`.

Two judgment checks stay in-model, on top of the script: `blocked_by:` re-examination and `github` mirror reachability (see *Judgment follow-ups* above) — both warn-only, never auto-fix.
