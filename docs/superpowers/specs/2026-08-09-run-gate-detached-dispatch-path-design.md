<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0275 — Run gate has no runnable path for slash-command or backgrounded implement-next dispatch](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0275-run-gate-has-no-runnable-path-for-slash-command-or-backgroun.md)**
<!-- docket:backlink:end -->

# Run gate — a runnable path for slash-command / backgrounded implement-next dispatch

**Change:** 0275 · **Type:** fix · **Priority:** high
**Discovered from:** 0271 (observed live on the 0271 run, 2026-08-09)

## Problem

The run gate promoted into the generated dispatch block (single source:
`cursor-rules/run-gate.md`, spliced by `sync-agents.sh` into `AGENTS.md`/`CLAUDE.md` and the
cursor rule) assumes one dispatch shape: before-snapshot → foreground dispatch → after-snapshot →
diff → `verify-run <id>`. When a human launches the run as a slash command
(`/docket-implement-next 271`) the dispatch happens in the same user turn and (by harness
default) in the background, so the session regains control only at the completion notification.
Steps 1–3 are structurally unrunnable on that path: no before-snapshot exists and there is
nothing to diff. Only the agent's prose happening to name the id made verification possible on
0271 — precisely the evidence class the gate exists to distrust.

## Design

Amend the gate template only. No script changes: the oracle already carries everything the
detached path needs — `verify-run --in-progress-ids --with-claimed-at` and `--iso-to-epoch`
were built (0271) so `runner-dispatch.sh`'s own gate could attribute a claim by **claim instant
vs dispatch epoch** with no before-set. This change ports that same attribution shape to the
parent-session gate prose.

### 1. Keep the foreground path primary — with one scoped wording change to step 2

Steps 1–4 stay the primary path for a dispatch the session itself performs and blocks on. One
edit is required (critic finding, second pass): step 2's blanket "never background it and never
poll" would countermand the new Detached section's first branch on the same page. Scope the
prohibition: foreground-and-block when the session dispatches and can block; a dispatch the
session backgrounds — or the harness backgrounds by default — enters the Detached section
instead. The two step-2 asserts in `tests/test_sync_agents_run_gate.sh` ("step 2 carries its own
foreground/blocking claim", "step 2 forbids backgrounding and polling") are adjusted in the same
commit to pin the scoped wording.

### 2. Add a named **Detached dispatch** section

Applies when the session did not foreground-block on the dispatch — a backgrounded agent it
launched itself, or a run it first learns of from a completion notification.

- **Session-issued backgrounded dispatch** (the session precedes the Agent call with tool calls):
  the session CAN run the full step-1 before-snapshot before launching, and MUST — plus one
  `date -u +%s` for `DISPATCH_EPOCH`. At the notification: re-sync, then
  `docket.sh verify-run --in-progress-ids --with-claimed-at`, and attribute with the runner
  gate's full **three-filter** rule — (1) not in the before-set, (2) `claimed_at` parses,
  (3) `claimed_at` >= `DISPATCH_EPOCH`. Exactly one survivor → `verify-run <id>` and step 4's
  verdict table unchanged, bounded re-dispatch included. Zero → done (drained, or complete —
  optionally `verify-run <id>` on any id the notification names). Two or more → stop and
  report, exactly as step 3's multi-candidate rule.
- **Slash-command launch / notification-first — unattributed mode.** The dispatch happened in
  the same user turn that requested it, so no before-set exists; a timestamp alone cannot
  attribute, because `claimed_at` is re-stamped at every phase boundary and a live foreign run
  (a concurrent loop claimed before our window) looks fresh too — the before-set is the filter
  that excludes it, and it is exactly the one this path lacks. So this mode is
  **verify-and-report only**: if the notification names an id, run `verify-run <id>` on it — a
  prose id is a hint worth verifying, never attribution authority; otherwise run `verify-run`
  across the current in-progress set and report every verdict. The session **never
  re-dispatches** in unattributed mode — re-dispatch requires the full three-filter attribution
  (before-snapshot captured), because re-dispatching onto a change a live agent holds is the one
  unrecoverable move. This mirrors the runner facade's own detached seam, whose `--observe` is
  observe-only for this same reason.

### 3. Fix the ordering wording

"Verify before you report" cannot bind a session that was *handed* the report by a notification.
Reword the gate's framing: the child's completion notification is the **child's claim**, not the
session's report — the session verifies before **relaying** that claim to the human as an
outcome. Same obligation, stated so it survives notification-driven control flow.

### 4. Delivery

- Edit `cursor-rules/run-gate.md` (sole source; sync-agents' `assemble_run_gate` splices it into
  every surface). `tests/test_sync_agents_run_gate.sh` enforces a mutation-tested **25-line
  brevity bound** (raise history in the test: 14→18→23→25) because the block rides always-loaded
  context in every harness; the detached addition cannot fit, so the build **deliberately raises
  the bound** in the same commit, recording the raise and its rationale in the test's history
  comment, and keeps the addition as tight as the existing steps. The verbatim-slice checks key
  on the template's line count and move with it.
- Regeneration: a bare `sync-agents.sh` run in this repo is a **no-op** (the harness opt-in
  lives in the gitignored `.docket.local.yml`); use the regeneration recipe documented in
  `tests/test_sync_agents_run_gate.sh`'s currency leg (`agent_harnesses: [claude, cursor]` in
  `.docket.local.yml`, then run sync-agents) so the committed `AGENTS.md` block actually updates
  and the currency assert stays green.
- Extend `tests/test_sync_agents_run_gate.sh`: assert the block carries the detached section
  (before-snapshot + epoch for session-issued background dispatch, `--with-claimed-at`, the
  never-re-dispatch-in-unattributed-mode rule) and mutation-test per house rules — strip the
  detached section from the template, watch the guard redden.

## Out of scope

- Any change to `verify-run.sh` / `runner-dispatch.sh` — their gate already has this shape.
- Harness hooks (`Stop`/`SubagentStop`) — rejected by 0242 as too heavy; unchanged here.
- New claim-time metadata (e.g. a run-manifest file) — a new writer and new shared state for a
  problem the existing `claimed_at` stamp already answers.

## Assumptions

1. **Attribution mechanism: the runner gate's full three-filter rule, ported as prose.**
   Chosen because the oracle (`--with-claimed-at`, `--iso-to-epoch`) and the filter semantics
   already exist and are proven in `runner-dispatch.sh`'s gate. Critic-revised: an epoch-only
   filter (two of three) is NOT sufficient for re-dispatch — `claimed_at` is re-stamped at every
   phase boundary, so a live foreign run claimed before the window can survive the epoch filter
   as a single false-positive candidate; the before-set is the filter that excludes it. So
   re-dispatch requires all three filters; without a before-set the path is verify-and-report
   only (the runner facade's `--observe` seam is observe-only for the same reason). Rejected:
   harness completion hooks (0242 rejected: machine-wide interception, couples to surface docket
   does not own); a claim-time manifest file (new writer, new state, duplicates `claimed_at`).
2. **The dividing line is session-issued vs same-turn dispatch, not facade vs no-facade.**
   Critic-revised: if a session can run any pre-dispatch tool call it can run the full
   before-snapshot, and if it cannot, it cannot run `date` either — so epoch capture is scoped
   to dispatches the session itself issues (backgrounded Agent calls), and a slash-command /
   notification-first launch enters unattributed mode by default. Correct under either
   resolution of the harness's actual same-turn behavior. Rejected: claiming a lone `date` call
   is runnable where the snapshot is not (contradicts the stub's own observation).
   Post-re-check repair (critic's prescription, applied verbatim): retained step 2's blanket
   "never background it" would countermand the Detached section's first branch, and the
   prohibition is mutation-tested — so step 2's prohibition is scoped (foreground when the
   session dispatches and can block; a backgrounded dispatch enters the Detached section), the
   two step-2 asserts move with it, and §1 no longer claims "verbatim".
3. **Re-dispatch requires mechanical attribution.** In unattributed mode the gate verifies and
   reports but never re-dispatches. Chosen as the conservative floor: the gate's own step 3
   already refuses to act on ambiguous ownership, and a prose id from the child's report is the
   evidence class the gate exists to distrust. Rejected: allowing one re-dispatch on a
   prose-named id (turns the untrusted report back into a decision input).
4. **Prose-only change; the template plus its guard suite is the deliverable.** No new script
   capability is needed. Critic-revised scope additions: the build must confront
   `test_sync_agents_run_gate.sh`'s 25-line brevity bound (raise it deliberately, with recorded
   rationale — the bound exists because the block is always-loaded context) and use the test's
   documented regeneration recipe, since a bare `sync-agents.sh` run in this repo is a no-op.
   Rejected: a `docket.sh` "gate" verb that mechanizes the whole sequence — a larger design not
   required to close the observed gap; propose separately if the prose gate proves error-prone.
5. **Foreground remains the stated primary path.** The detached section is an addition, not a
   replacement, so existing harness behavior and the runner-side gate are untouched. Rejected:
   scoping the gate to foreground only and telling the human what to run instead (leaves the
   actually-observed dispatch shape ungated).
6. **Dependency state:** none. 0242 (gate surface) and 0271 (epoch machinery) are both `done`;
   recorded as `related:`. No active change touches `cursor-rules/run-gate.md`
   (0273's mention is a results-file citation, not an edit).

## Reconcile addendum — 2026-08-11

Verified against current `main` at claim time; the design above stands unchanged. Three
clarifications the build must carry:

1. **`cursor-rules/dispatch.head.md` item 2 is a second countermanding site**, not enumerated in
   §4's delivery list. It carries the same "never background it and never poll" sentence and, on
   the Cursor surface, is spliced *above* the gate. It is deliberately **not edited** — its
   directive governs every docket agent dispatch, not just implement-next runs, and widening it
   would loosen a rule that legitimately binds elsewhere. Instead the Detached section states its
   own precondition explicitly in its opening sentence — it governs a dispatch that was **not**
   foreground-blocked, whoever backgrounded it — so the gate reads consistently on both surfaces
   from its own text, with no cross-file dependency.
2. **The gate is native-dispatch-only.** Change 0277 moved delegated task briefs off argv onto a
   `--brief-file` channel for the `runner-dispatch.sh` facade, and adapters refuse the
   both-channels shape. The caller-side gate never invokes that facade — only `docket.sh preflight`
   and `docket.sh verify-run` — and its re-dispatch is a harness-native named-agent dispatch whose
   retry context rides the dispatch prompt. The detached prose inherits this: it must never be
   written as a facade invocation carrying trailing argv.
3. **No observation loop.** The detached path regains control exactly once, at the completion
   notification. It must not grow a poll loop, so change 0286's taught `gate-run --observe` loop
   shape (`scripts/gate-run.md`, "The caller's loop") is not a dependency and must not be imported.
