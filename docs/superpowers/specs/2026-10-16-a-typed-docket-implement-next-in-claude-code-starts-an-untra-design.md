<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0556 — A typed /docket-implement-next in Claude Code starts an untracked run](../../changes/active/0556-a-typed-docket-implement-next-in-claude-code-starts-an-untra.md)**
<!-- docket:backlink:end -->

# Change 0556: say when an implement run is untracked, and document the tracked ways to start one

Design approved by Daniel on 2026-10-16.

## Goal

Nobody should end up with an untracked implement-next run without knowing it, and the docs should name the ways to start one that the run tracker does track. A typed `/docket-implement-next` in Claude Code keeps working exactly as it does today, and stays visible in Claude's `/` menu.

This change makes two edits. The implementer's final report says plainly when its run was untracked (Part A). The maintained docs say to start implement-next with a plain request or `/loop` (Part B). It does not try to make the typed command tracked; *Alternatives considered* records why.

**Known limit (accepted).** The note rides the final report, so a run that dies before writing one (the costly case) shows no note. Part B's docs carry that case: they teach the tracked starts before anyone needs recovery. Covering it inside the run would mean changing the parent-facing dispatch blocks or the run tracker's output, which this change deliberately leaves alone.

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

The session can only read the outcome with `run.verdict --unattributed`, which observes and authorizes nothing. ADR-0084 already names this state: a slash-command launch has no attribution and runs in unattributed mode, never re-dispatching. This change makes that state visible to the human instead of silent.

### History

- **0242** gave Claude a parent-facing run-completion check.
- **0275** found the slash-command launch had no runnable path for that check and settled it as observe-only (ADR-0084).
- **0345** proposed a four-harness redesign so command entry would be tracked. It was killed on 2026-10-16 as outdated: Codex is unsupported, OpenCode ships untested, the run-gate vocabulary was renamed, and only the typed slash path turned out to be affected.

Daniel drives docket with prose and `/loop`, not typed commands.

## Decision

Keep the typed command as it is, visible and forked. Make an untracked run say so in its own final report (Part A). Point the docs at the tracked starts: a plain request in the session, or `/loop` (Part B).

## What changes

### Part A: the untracked-run note in the implementer's final report

In `skills/docket-implement-next/SKILL.md`, extend the final-report contract in *Terminal disposition (driver contract)*: the paragraph beginning "The final report **enumerates** what happened".

**When it applies.** The dispatch prompt carried **neither a run-context token nor a run key**. That covers:

- a typed or model-initiated skill invocation;
- a parent that received `run-untracked` and dispatched keyless;
- any direct agent launch that skipped `run.start`.

It does **not** apply when the prompt carried either one:

- **A run context:** the ordinary tracked dispatch.
- **A run key:** the parent-facing blocks describe the `run-retry-once` re-dispatch as "keeping the same key". The `run-continue` re-dispatch carries "the explicit change id, the continuation id, and the same key". Neither promises the run context. Those runs are tracked by key, so telling the human otherwise would be false.

A child that had a run context and failed to pass it to the claim is change 0541's problem.

Step 2's existing sentence equates "no token" with "a `run-untracked` or ungated run" (*Verify the run.* paragraph). That ignores keyed re-dispatches. The note's condition must not inherit that gap. Whether to also correct that sentence is the builder's call.

**What the note says.** It is written in plain words, covering what happened, whether it matters, and what to do:

- **What:** this run was started without a run context, so the run tracker is not tracking it.
- **Whether it matters:** if it stops partway, docket cannot retry it, resume it, or cancel it automatically; a run that finished is unaffected.
- **What to do:** to get a tracked run next time, start implement-next by asking the session in plain words (for example "implement 556") or with `/loop docket-implement-next`, so the session starts the tracked run first. Name the usual cause: invoking the skill directly, such as a typed `/docket-implement-next`.

**Who reads it.** In the case this targets, the forked child's final report goes to the main session, which then reports to the human. The note is therefore a separate, clearly marked line addressed to the human. It asks the session receiving the report to pass it on as written, not summarize it away. That is an informational request inside the child's own report, not authority and not a change to the parent-facing blocks.

The exact wording is the builder's. Keep the skill prose harness-neutral (the convention's rule for normative skill prose). The skill already names `/loop` in *Terminal disposition*, so naming it again is consistent.

**What the note must never do.** These are anti-goals:

- change the declared disposition;
- stop, delay, or fail the run;
- refuse or alter the claim;
- start a run, mint a key, or try to recover attribution on the child's side.

A key the child minted and reported back would be authority taken from child prose, which the run tracker forbids. The run key is the tracker's only handle (ADR-0133). This is visibility only. Docket's standing rule is that new checks report and never block.

### Part B: the docs name the tracked starts

Docs describe current behavior only: no change numbers or PR references (ADR links are fine).

- **`docs/guide/building-without-supervision.md`.** In the section that recommends `/loop docket-implement-next`, say to start implement-next with a plain request or `/loop`, both of which the run tracker tracks. Say that a typed `/docket-implement-next` in Claude Code goes straight to the implementer, so that run is untracked. Say what that costs: no automatic retry, resume, or cancel if it stops partway.
- **`docs/concepts/run-tracker.md`.** Where *The moving parts* describes **Starting**, add that a tracked start needs the session to take a turn first. In Claude Code, a typed `/docket-implement-next` skips that, so the run is untracked and can only be read unattributed.
- **`docs/install/models-and-effort.md`.** The *two invocation paths* table recommends skill-invoke "for everything else". Add the exception for implement-next: a skill-invoke of `docket-implement-next` (typed, or the model invoking the skill) runs untracked, so start implement-next with a plain request or `/loop`. Leave the existing table rows, and the `completed (forked execution)` phrase a prose-contract sentinel pins, intact.
- **`README.md`.** The quick-start line "**Drain** the backlog to open PRs — `docket-implement-next` (or `/loop` it hands-free)." reads as if the bare command is the way in. Reword it to a plain request or `/loop`. Keep the sentinel-pinned phrase "The right model for each step." intact.
- **Anything else.** Find every other maintained doc that presents `/docket-implement-next` or a skill-invoke of it as a way to start a run, by whole-repository grep, not by this list, and give it the same treatment. Exclude point-in-time records: archived changes, results files, specs, plans, and Accepted ADRs.

## Constraints

- The skill frontmatter (`context: fork`, `agent: docket-implement-next`) stays unchanged. The command stays visible in Claude's `/` menu (no `user-invocable: false`).
- No change to the run tracker binary, `change.claim`, the parent-facing dispatch blocks, the agent wrappers, or any harness adapter.
- No new ADR, and no edits to ADR-0024, ADR-0026, or ADR-0084. ADR-0084 already describes the slash launch as unattributed; this change only makes that visible.
- After editing the skill, regenerate the embedded assets (`go generate ./internal/assets/`) so release installs carry the change.
- **Size budget.** `skills/docket-implement-next/SKILL.md` sits exactly at its `TestSkillSizeBudgets` ceiling (215 lines / 8411 words, `internal/repoguard/budgets_test.go`), so any added word reddens the suite. Keep the note to about two sentences in the final-report paragraph. Raise that file's ceiling by exactly the added count, with a reason comment in the file's existing re-baseline style. This is the routine path, as 0531 and 0532 did. If the note cannot stay that short, follow 0448's pattern: put the full note in `references/edge-paths.md` behind a blocking pointer at its trigger. A pointer costs words too, so the ceiling still moves by exactly the pointer's size. Never trim unrelated SKILL.md prose to make room.

## Alternatives considered

**Route the typed command through the main session (0345's Claude row).** Drop `context: fork` so the typed command lands in the main session, which would start a tracked run and dispatch the agent. This was rejected. It rests on unproven Claude Code behavior, it reaches into every harness, and it would serve a path Daniel does not use. Research findings:

- The skill body is about 57 KB (about 14k tokens), and the agent wrapper preloads it (`skills: [docket-implement-next, docket-convention]`). Without the fork, Claude Code's documentation says "the rendered `SKILL.md` content enters the conversation as a single message". The main session would load the whole implementer procedure and could build inline at the session's model, which is what ADR-0024's fork exists to prevent. The skill would have to split into a short entry plus a worker body that the agent still receives in full.
- Sibling reference files are not auto-loaded; only `SKILL.md` is (learning `skill-extraction-and-stub-pointer`). A split that moves the worker body into a reference depends on the agent reading it.
- A separate worker skill can't be hidden from the main model. Claude Code's sub-agent documentation says: "You can't preload skills that set `disable-model-invocation: true`". Whether `user-invocable: false` still allows preloading is undocumented.
- A Claude-only command file can't take over the name: when "A skill and a file in `.claude/commands/`" collide, "The skill" wins.
- The skill folder is shared by every harness (installed by symlink for Claude, Cursor, OpenCode, and Codex). Any split also changes what Cursor's implementer loads, and Cursor is supported and tested.

**Hide the typed command (`user-invocable: false`).** This was rejected. The note in Part A already covers accidental use. Whether a hidden skill can still be preloaded into the agent is undocumented. And change 0503 would like typed commands to stay usable.

**Pre-render shell injection or a vendor hook.** 0345 analyzed these. They could hand the child a run context, but they do not give the main session the key or make it run the verdict. That is half the tracking and new harness-specific machinery.

**Document `@docket-implement-next`.** Not done: the agent @-mention is not widely known and is untested for tracking.

## Tests and verification

- Add a prose-contract sentinel for the Part A note in `internal/repoguard/prose_contracts_test.go`, following its existing `skills/docket-implement-next/SKILL.md` entries (or the `edge-paths.md` file, if the note lands there). Anchor it on verbatim clauses of the note: its condition (neither a run context nor a run key), its untracked statement, and its remedy. Mutation-test it: strip the note, or drop the run-key exclusion, and watch the sentinel go red.
- Whether the Part B doc statements also get sentinels is the builder's call. If added, follow the same mutation discipline.
- Regenerate embedded assets, and keep the asset inventory and golden tests green.
- Run the full suite through the configured `build.test_command`.
- No live native acceptance is required. An optional human walkthrough for the results file: run a typed `/docket-implement-next` in a throwaway repository and confirm the final report carries the note.

## Left to the builder

- The note's exact wording and its placement within the final-report paragraph.
- Exact placement and wording within each named doc page.
- The sentinel's anchor phrases.
- Whether docs get sentinels.

## Related

- **Change 0541** (proposed): a child given a run context can still claim without it. That is a different gap, out of scope here. If 0541 changes `change.claim`'s interface (for example an explicit untracked flag), whichever change lands second reconciles Step 2's wording and this note's condition.
  - The spec review also noticed that the parent-facing blocks do not say whether a `run-retry-once` or `run-continue` re-dispatch carries the run context, though the Codex adapter's clause does. A retried child without it would claim untracked under a tracked key. That belongs with 0541's grooming, not here.
- **Change 0503** (deferred): session auto-naming. It excluded typed slash commands on the belief that they did not work in docket. They do run; Daniel simply drives by prose, and this change documents the tracking difference.
- **ADRs:** ADR-0024 (the fork), ADR-0026 (the two invocation paths), ADR-0084 (unattributed mode for slash launches), and ADR-0133 (the run key is the only handle).

## Out of scope

- Making a typed `/docket-implement-next` tracked, or changing its fork frontmatter or visibility.
- Codex, Cursor, and OpenCode launch mechanics. The skill note is harness-neutral and applies wherever a run lacks a run context; nothing harness-specific changes.
- The other forked skills (`docket-status`, `docket-adr`, `docket-auto-groom`), which the run tracker does not track.
- Recovering attribution after the fact, or any change to the run tracker, claim, or dispatch blocks.
