# Change 0556: document the tracked ways to start implement-next

Change 0556 — A typed /docket-implement-next in Claude Code starts an untracked run

Design approved by Daniel on 2026-10-16, and revised the same day to a docs-only change after a behavioral-risk review.

## Goal

The maintained docs should name the ways to start implement-next that the run tracker tracks: a plain request in the session, or `/loop`. They should also say plainly that a typed `/docket-implement-next` in Claude Code runs untracked. The typed command keeps working exactly as it does today and stays visible in Claude's `/` menu.

This is a docs-only change. No skill, agent wrapper, dispatch block, harness adapter, or run-tracker code changes. *Alternatives considered* records why: both making the typed command tracked and labeling the untracked run inside the skill were weighed and rejected.

## Background and evidence

### What happens today

The run tracker (`docs/concepts/run-tracker.md`) only works when the session that launches the implementer takes a turn first. The parent runs `run.start`, gets a run key and a run context, puts the run context in the dispatch prompt, and runs `run.verdict <key>` when the worker returns. The implementer passes the run context to `change.claim --run-context`, which records proof binding the claim to that run. Those parent steps live in the parent-facing instructions (the managed `docket:dispatch` block in `CLAUDE.md`/`AGENTS.md`, and `cursor-rules/run-tracker.md`). Only a main session reading them can follow them.

`skills/docket-implement-next/SKILL.md` declares `context: fork` and `agent: docket-implement-next` (ADR-0024). Claude Code's skill documentation describes the fork: "Claude Code starts a new subagent of the type set in the `agent` field and gives it the skill content as its prompt." A typed `/docket-implement-next` therefore goes straight to the implementer. The main session never takes a turn, so nothing runs `run.start`.

### Live check, 2026-10-16

This used Claude Code 2.1.301, a fresh interactive session per check, and the installed `docket` at `ffa75abfa`. There were two throwaway repositories, each with a local bare repository as `origin` and one trivial build-ready change (id 1). Each check was run once.

- **Typed `/docket-implement-next 1`.** The parent transcript holds only the typed command, with no assistant turn. The session spawned a subagent with `agentType: docket-implement-next`, `requestShape: background`. Its prompt was the skill body followed by `ARGUMENTS: 1`, with no run context. It claimed with `docket change claim --id 1 --revision <rev> --json`, without `--run-context`. The claim commit's `Docket-Result` carries `gate_context_hash: ""`, so the run is untracked. The run then continued normally (reconcile, plan, worktree) until the session was closed.
- **`/loop docket-implement-next 1`.** `/loop` hands the text to the main session as an ordinary prompt. The main session ran `docket run start implement-next`, dispatched `docket-implement-next` through native agent dispatch with `Run context: <token>` in the prompt, and scheduled its loop wakeup. The claim commit carries a non-empty `gate_context_hash`, so the run is tracked.

Plain prose requests ("implement 556") take the same path as `/loop`. Change 0541 found every claim in this repository since 2026-10-04 carrying a context hash, except one caused by a child dropping a token it had been given.

### What an untracked run costs

An untracked run still builds to a PR; the work itself is unaffected. What is lost is the run tracker's recovery. If the run stops partway, there is no key, so:

- no `run-retry-once` re-dispatch,
- no `run-continue` continuation,
- no `run.cancel`.

The session can only read the outcome with `run.verdict --unattributed`, which observes and authorizes nothing. ADR-0084 already names this state: a slash-command launch has no attribution and runs in unattributed mode, never re-dispatching.

### History

- **0242** gave Claude a parent-facing run-completion check.
- **0275** found the slash-command launch had no runnable path for that check and settled it as observe-only (ADR-0084).
- **0345** proposed a four-harness redesign so command entry would be tracked. It was killed on 2026-10-16 as outdated: Codex is unsupported, OpenCode ships untested, the run-gate vocabulary was renamed, and only the typed slash path turned out to be affected.

Daniel drives docket with prose and `/loop`, not typed commands.

### What the docs say today

The maintained docs never warn about the typed path, and one of them steers toward it:

- **`docs/install/models-and-effort.md`.** The *two invocation paths* table presents skill-invoke (`/docket-status`, or the model auto-invoking the skill) and agent-dispatch as equivalent for the pinned run. It closes with "**skill-invoke for everything else**". For implement-next, a skill-invoke is the untracked path.
- **`README.md`.** The quick start says "**Drain** the backlog to open PRs — `docket-implement-next` (or `/loop` it hands-free)." That reads as if the bare command is the way in.
- **`docs/guide/building-without-supervision.md`.** It recommends `/loop docket-implement-next` and links the run tracker, but does not say a typed command bypasses it.
- **`docs/concepts/run-tracker.md`.** It explains `run.start` but not that a launch which skips the session's turn skips the tracker.

## Decision

Change only the maintained docs. Keep the typed command as it is: visible, forked, and untracked. Add no instructions to any skill or agent.

## What changes

Docs describe current behavior only: no change numbers or PR references (ADR links are fine).

- **`docs/install/models-and-effort.md`.** In the *two invocation paths* section, add the implement-next exception. A skill-invoke of `docket-implement-next` (typed, or the model invoking the skill) runs untracked, so start implement-next with a plain request in the session or `/loop`. Leave the existing table rows, and the `completed (forked execution)` phrase a prose-contract sentinel pins, intact.
- **`docs/guide/building-without-supervision.md`.** In the section that recommends `/loop docket-implement-next`, say to start implement-next with a plain request or `/loop`, both of which the run tracker tracks. Say that a typed `/docket-implement-next` in Claude Code goes straight to the implementer, so that run is untracked. Say what that costs: no automatic retry, resume, or cancel if it stops partway.
- **`docs/concepts/run-tracker.md`.** Where *The moving parts* describes **Starting**, add that a tracked start needs the session to take a turn first. In Claude Code, a typed `/docket-implement-next` skips that, so the run is untracked and can only be read unattributed.
- **`README.md`.** Reword the quick-start drain line to a plain request or `/loop`. Keep the sentinel-pinned phrase "The right model for each step." intact.
- **Anything else.** Find every other maintained doc that presents `/docket-implement-next`, or a skill-invoke of it, as a way to start a run, by whole-repository grep, not by this list, and give it the same treatment. Exclude point-in-time records: archived changes, results files, specs, plans, and Accepted ADRs.

## Constraints

- No edits to any `skills/**`, `agents/**`, `cursor-rules/**`, harness adapter, run-tracker, or claim code. No new ADR, and no edits to ADR-0024, ADR-0026, or ADR-0084.
- The skill frontmatter (`context: fork`, `agent: docket-implement-next`) stays unchanged. The command stays visible in Claude's `/` menu.
- The edited docs are not part of the embedded asset set (`internal/assets/embedded` holds only agents, cursor-rules, and skills), so no asset regeneration is needed. If the builder's grep turns up a target inside the embedded set, the *no skill or agent edits* constraint wins: report it as a follow-up instead of editing it.
- Do not document the `@docket-implement-next` agent mention; it is not widely known and is untested for tracking.

## Alternatives considered

**Label the untracked run in the implementer's final report.** This was the first approved design. It was rejected after a behavioral-risk review on 2026-10-16:

1. **It shows up only when it doesn't matter.** The note would ride the final report, so it would appear only on runs that finished. The costly case, a run dying partway, writes no report and would get no note.
2. **Its risk lands on every run.** The implementer skill is read by every implement run on every harness, tracked or not. So any behavioral nudge from the new text would apply to all runs, while the note could only help the rare untracked one. Making two sentences safe took six explicit rules:
   - report-only, never a reason to stop;
   - never self-track, so no `run.start`, `run.verdict`, or `run.cancel`, and no searching for or borrowing a run context or key;
   - advice for next time, not a request to re-run;
   - no look-alike run-tracker or disposition tokens;
   - Step 2's claim and the *Verify the run.* paragraph left untouched;
   - harness-neutral wording.

   Needing that many rules signals the risk.
3. **It protects a path nobody here uses.** Daniel starts runs with prose or `/loop`, and the docs reach anyone else without an agent re-reading a warning on every run.
4. **It costs words in a skill already at its size cap.** `skills/docket-implement-next/SKILL.md` sits exactly at its `TestSkillSizeBudgets` ceiling (215 lines / 8411 words).

The specific behavioral risks found are recorded here so nobody re-proposes the note without them:

- A child "fixing" its untracked state by running `run.start` would leave a live run whose key no session holds. Because one checkout carries at most one live run, that could block the human's next tracked start or resume there until someone finds the key and cancels it.
- A main session could read "start implement-next with a plain request" as a to-do and launch another run. After an `advanced` run, that builds the next change.
- The abort-and-report rule could read "untracked" as an unmet precondition and stop the run.

**Warn from the run tracker instead (future option).** If in-run visibility is ever wanted, the right home is the run tracker, not skill prose. For example, a plain note when a session reads an untracked run with `run.verdict --unattributed`. That is deterministic code with no agent interpretation, and it would also cover a run that died partway. It is not needed now; it would be its own change.

**Route the typed command through the main session (0345's Claude row).** Drop `context: fork` so the typed command lands in the main session, which would start a tracked run and dispatch the agent. This was rejected. It rests on unproven Claude Code behavior, it reaches into every harness, and it would serve a path Daniel does not use. Research findings:

- The skill body is about 57 KB (about 14k tokens), and the agent wrapper preloads it (`skills: [docket-implement-next, docket-convention]`). Without the fork, Claude Code's documentation says "the rendered `SKILL.md` content enters the conversation as a single message". The main session would load the whole implementer procedure and could build inline at the session's model, which is what ADR-0024's fork exists to prevent. The skill would have to split into a short entry plus a worker body that the agent still receives in full.
- Sibling reference files are not auto-loaded; only `SKILL.md` is (learning `skill-extraction-and-stub-pointer`). A split that moves the worker body into a reference depends on the agent reading it.
- A separate worker skill can't be hidden from the main model. Claude Code's sub-agent documentation says: "You can't preload skills that set `disable-model-invocation: true`". Whether `user-invocable: false` still allows preloading is undocumented.
- A Claude-only command file can't take over the name: when "A skill and a file in `.claude/commands/`" collide, "The skill" wins.
- The skill folder is shared by every harness (installed by symlink for Claude, Cursor, OpenCode, and Codex). Any split also changes what Cursor's implementer loads, and Cursor is supported and tested.

**Hide the typed command (`user-invocable: false`).** This was rejected. Whether a hidden skill can still be preloaded into the agent is undocumented, and change 0503 would like typed commands to stay usable.

**Pre-render shell injection or a vendor hook.** 0345 analyzed these. They could hand the child a run context, but they do not give the main session the key or make it run the verdict. That is half the tracking and new harness-specific machinery.

**Document `@docket-implement-next`.** Not done: the agent @-mention is not widely known and is untested for tracking.

## Tests and verification

- Run the full suite through the configured `build.test_command`. Existing doc sentinels must stay green, in particular the pinned README and `models-and-effort.md` phrases.
- Whether the new doc statements get prose-contract sentinels is the builder's call. If added, follow the existing doc-contract tests' pattern and mutation-test each one: strip the statement and watch it redden.
- A reading review confirms the edited pages describe only current behavior and carry no change numbers.
- No live native acceptance is required.

## Left to the builder

- Exact placement and wording within each named page.
- Whether to add doc sentinels.

## Related

- **Change 0541** (proposed): a child given a run context can still claim without it. That is a different gap, out of scope here.
  - Noticed during this groom: the parent-facing blocks do not say whether a `run-retry-once` or `run-continue` re-dispatch carries the run context, though the Codex adapter's clause does. A retried child without it would claim untracked under a tracked key. That belongs with 0541's grooming.
- **Change 0503** (deferred): session auto-naming. It excluded typed slash commands on the belief that they did not work in docket. They do run; Daniel simply drives by prose, and this change documents the tracking difference.
- **ADRs:**
  - ADR-0024 (the fork);
  - ADR-0026 (the two invocation paths that `models-and-effort.md` documents);
  - ADR-0084 (unattributed mode for slash launches);
  - ADR-0133 (the run key is the only handle, cited by the rejected self-tracking risk).

## Out of scope

- Any change to skills, agent wrappers, dispatch blocks, harness adapters, the run tracker, or `change.claim`. That includes an untracked-run note in the skill and a run-tracker warning.
- Making a typed `/docket-implement-next` tracked, or changing its fork frontmatter or visibility.
- Codex, Cursor, and OpenCode launch mechanics.
- The other forked skills (`docket-status`, `docket-adr`, `docket-auto-groom`), which the run tracker does not track.
