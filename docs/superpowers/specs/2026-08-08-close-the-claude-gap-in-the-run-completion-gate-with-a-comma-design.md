<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0242 — Close the Claude gap in the run-completion gate with a caller-side verify in the dispatch rules](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0242-close-the-claude-gap-in-the-run-completion-gate-with-a-comma.md)**
<!-- docket:backlink:end -->

# Design — close the Claude gap in the run-completion gate at the caller's seam

Change 0242. Wires the one uncovered dispatch path onto the oracle change 0237 built.

> **Revision history, same day (2026-08-08).** Draft 1 (Stop hook) was rejected as too heavy —
> preserved under *Rejected* as the escalation path. Draft 2 (gate in "the generated per-harness
> dispatch rules") was halted at reconcile: its premise was false for Claude — no generated
> parent-facing surface exists there, deliberately, per ADR-0024. This draft settles the missing
> piece: docket now creates that surface. The slug retains the word "hook"; slugs are filenames
> and do not chase design pivots.

## Problem

Change 0237 gave the terminal-disposition contract its missing consumer — `docket.sh verify-run`,
a pure git reader of Step 7's postcondition — and a caller at the dispatch seam docket owns,
`runner-dispatch.sh`. Every harness whose autonomous runs are CLI-driven (`codex`, `cursor`,
`opencode`, future adapters) dispatches through it and is covered.

The uncovered surface is **Claude interactive sessions**: a human types
`/docket-implement-next` (or a `/loop` over it) and the harness forks the skill itself via
native `context: fork` frontmatter — `runner-dispatch.sh` is not on that path. All six observed
instances of the half-run family (0109, 0194 ×2, 0206, 0231, 0235) happened on exactly that
path; it is covered today only by `board-checks.sh`'s `aborted-run` legs and their 2h/12h
floors.

The structural fact the fix must respect: any check placed **inside** the skill or its wrapper
is executed by the same agent that is failing — an agent that stops at a step boundary also
stops before its own "now verify yourself" step. The check must run in the context that regains
control **after** the failing agent has stopped: the **parent session**, at the moment the
fork's report returns.

**The reconcile finding this draft resolves.** The parent reads a harness-specific
always-in-context surface, and docket generates into it for two harness sets only —
`.cursor/rules/docket-dispatch.mdc` (cursor) and the `AGENTS.md` `docket:dispatch` block
(codex/opencode). Claude has **neither**: ADR-0024 solved Claude's routing problem natively
(`context: fork` — "no generated file, no hook, no CLAUDE.md routing"), so no parent-facing
Claude surface was ever built. Verified live during grooming (2026-08-08, this repo): docket's
own root has `AGENTS.md` but no `CLAUDE.md`, and an interactive Claude session loaded
**neither** — so beyond the gate, even the promoted learnings tier (rules that "must fire
unprompted") currently never reaches Claude sessions. The gate and the learnings have the same
delivery problem; this change fixes the surface once, for both.

## Decisions

Settled with the human during grooming, 2026-08-08 — including the post-reconcile
re-brainstorm.

1. **The mechanism is a caller-side verify on fork return, carried by the parent-facing
   instruction surface** — the `docket:dispatch` block `sync-agents.sh` already assembles,
   extended with the gate and delivered to every enabled harness's parent, **including
   Claude's** (Decision 2). The parent runs 0237's gate shape: snapshot before dispatch, verify
   after return, one bounded re-dispatch. Ships with the sync consuming repos already run;
   covers every invocation shape (one-offs and each `/loop` iteration, which is itself a fork
   return).
2. **Docket creates the missing Claude surface, and the policy is one physical file.** Claude
   Code's documented always-loaded surface is `CLAUDE.md`; per the
   `harness-behavior-is-mode-and-version-scoped` finding, delivery targets that documented
   surface, never "maybe this version also reads `AGENTS.md`". When `claude` is an enabled
   harness:
   - `CLAUDE.md` exists (file or symlink) → target its `realpath`.
   - `CLAUDE.md` absent, `AGENTS.md` present → **create `CLAUDE.md` as a committed symlink to
     `AGENTS.md`** — one physical instructions file; Claude loads everything `AGENTS.md`
     carries (the gate block *and* the promoted learnings), identically to codex/opencode.
     Chosen over a generated `@AGENTS.md`-import stub (import support is itself
     harness-version-scoped) and over a standalone block-only `CLAUDE.md` (leaves the learnings
     undelivered and forks the gate text into a second physical file).
   - Neither exists → create a real `CLAUDE.md` seeded with only the managed block.
   The block is written **once per distinct physical file** (`realpath` dedupe) — a symlinked
   pair never gets two diverging copies (the `decide-and-act-on-the-same-copy` posture).
3. **Routing stays native; the surface carries only the gate.** `context: fork` frontmatter is
   untouched — it is the one harness-enforced link in the chain, it guarantees the parent/child
   separation the gate's soundness rests on, and replacing it with prose routing would
   reintroduce the inline-at-session-model defect ADR-0024 killed. This change therefore
   **amends nothing in ADR-0024's decision**: that ADR governs how invocations *route*; a new
   parallel ADR (produced by this change) governs what the parent *does after a routed run
   returns* — a surface question 0024 never faced, since in 2026-07 there was no oracle to
   consult. The new ADR also records the symlink policy of Decision 2.
4. **A Claude Code `Stop`/`SubagentStop` hook remains rejected** (recorded under *Rejected*;
   weight: machine-wide interception of all sessions including non-docket work; coupling to
   unowned, version-mobile harness surface). It remains the worked-out escalation path if the
   caller-side gate is observed degrading.
5. **No loop-specific mechanism** (each `/loop` iteration is a fork return — covered), **no
   headless-Claude runner adapter** (the human's subscription does not cover headless
   invocations), **no re-scope away from Claude** (all six incidents are the Claude path; a
   gate that reaches only the already-gated harnesses manufactures false coverage — the
   reconcile's own verdict).
6. **The gate mirrors 0237's shape wherever it can**: same oracle, same snapshot-diff
   discriminator, same one-bounded-re-dispatch cap, same `run-halted`-never-re-dispatches rule.
   No second verdict logic exists anywhere.

## Design

### 1. The caller-side gate, as managed-block text

The gate procedure, addressed to the parent session, phrased harness-neutrally (the same text
serves every harness's parent):

1. **Before dispatching**: record the current in-progress set —
   `docket.sh verify-run --in-progress-ids`.
2. **Dispatch** and block on the return, as the surrounding dispatch prose already directs.
3. **After return**: re-run `docket.sh verify-run --in-progress-ids`; any id not in the
   before-set is this run's claim (the discriminator `runner-dispatch.sh` uses — a foreign
   concurrent claim was in the before-set and is ignored). Empty diff (`drained`, lost CAS) →
   the gate is a no-op. An explicitly-passed id may be verified directly; the snapshot still
   runs — two cheap commands, one uniform procedure.
4. **Verify**: `docket.sh verify-run <id>`, keying on the report line:
   - `run-complete` / `run-unclaimed` → done.
   - `run-halted` → done; never re-dispatch (a halt means a human is needed).
   - `run-incomplete` → **re-dispatch the same skill once**, passing the change id and the
     unmet conjuncts as task context. After the second return, verify again: complete/halted →
     report; `run-incomplete` again → **stop and report loudly**, naming the change and the
     unmet conjuncts. The change stays `in-progress` with its claim intact; the `aborted-run`
     legs remain the backstop. Never a third dispatch.

Every step is a single facade command whose execution (or absence) is visible in the parent's
transcript. Brevity is a requirement, not a style preference — the block rides always-loaded
context in every enabled harness.

### 2. Delivery — targets resolved per enabled harness, written once per physical file

`sync-agents.sh` resolves the target set from `agent_harnesses`:

| Enabled harness | Parent-facing target |
|---|---|
| `codex` / `opencode` | `AGENTS.md` `docket:dispatch` block (existing machinery, block extended) |
| `claude` | `CLAUDE.md` per Decision 2 — existing file's `realpath`, else create the symlink → `AGENTS.md`, else create seeded `CLAUDE.md` |
| `cursor` | `.cursor/rules/docket-dispatch.mdc` (existing rule, gate appended from the same template source) |

Then dedupe by `realpath` and write the managed block once per distinct physical file. The gate
text has **one template source**; per-harness files assemble from it (the build must confirm
this single-sourcing so the gate cannot fork into silently diverging variants — the
`consolidation-flattens-caller-variance` and `duplicated-gate-copies-the-whole-predicate`
findings).

Repo combos this logic covers, conditioned on enabled harnesses: `AGENTS.md` only (symlink gets
created when `claude` is enabled — docket's own repo is this case today); `CLAUDE.md` symlinked
to `AGENTS.md` (resolved to one write); `CLAUDE.md` only; neither (seeded). Harnesses whose
autonomous paths run through `runner-dispatch.sh` keep that gate too; double coverage is
harmless — `verify-run` is a pure reader, and a parent re-checking a runner-gated run sees
`run-complete`.

The convention's *Composition* prose (caller "verifies the child's git-state transition") gains
a pointer naming the managed-block gate as that obligation's mechanical form for interactive
dispatch — a sentence, not a restatement.

### 3. Trust posture — stated honestly

The gate is an instruction a model follows, and the family exists because prose degrades. It is
differently positioned from the six failed levers on three axes: addressed to the **parent**
(the non-failing agent, whose remaining job is short), **a handful of mechanical commands**
rather than a multi-step behavioral contract, and **transcript-verifiable** — a degraded gate
shows as a missing command, not a plausible summary. The hard-mechanical alternative was
rejected on weight with eyes open; *Rejected* below is the escalation path.

### 4. What this change does not touch

`verify-run.sh`, `runner-dispatch.sh`, and `board-checks.sh` are consumed as-is. `context:
fork` frontmatter and ADR-0024's routing decision are untouched. No hook, no `settings.json`,
no installer work. No metadata write, status flip, or claim release by the gate.

## Scope

**In:**
- The gate procedure in the shared dispatch-block template; assembly into the `AGENTS.md`
  block, the cursor rule, and the Claude surface.
- The Claude-surface resolution/creation logic in `sync-agents.sh` (Decision 2), including the
  `realpath` dedupe and the committed symlink for the AGENTS-only combo.
- A new parallel ADR: parent-facing gate surface vs ADR-0024's routing; the symlink policy.
- The one-sentence pointer in the convention's *Composition* prose.
- Tests: sync-agents suite coverage for the generated block (snapshot command, verify command,
  the one-re-dispatch bound, the run-halted no-re-dispatch rule — anchored per
  `prose-guard-binds-phrase-to-claim` / `specified-but-unreachable`) and for the surface logic
  (each repo combo × harness set, symlink creation, `realpath` dedupe writing exactly once).
  Live parent compliance is external truth with no in-repo oracle — a named
  human-verification item per `external-truth-needs-a-human-checkpoint`.

**Out:**
- Any Claude Code hook (see *Rejected*), any `settings.json` or installer work.
- Any loop-specific mechanism; any headless-Claude runner adapter; any re-scope away from
  Claude (Decision 5).
- Any change to `verify-run.sh` / `runner-dispatch.sh` / `board-checks.sh` / `context: fork`
  routing; any new config knob; any status flip or claim release; any re-derivation of verdict
  logic.

## Rejected — the command-type Stop hook (draft 1, same day)

For the record, and as the worked-out escalation path should the caller-side gate degrade:
a `scripts/claude-stop-hook.sh` registered user-level for `Stop` + `SubagentStop` via
`ensure-docket-env.sh`; self-gating four-leg sequence (repo gate → transcript-derived session
gate → attribution with `claimed_at`-epoch fallback via
`verify-run --in-progress-ids --with-claimed-at` → verdict); exit 2 blocks the stop and feeds
the unmet conjuncts to the still-alive agent; capped at one block per session×change; fail-open
on any internal error; blocking build-time re-probe of the hook protocol at the current CC
version. Rejected 2026-08-08: machine-wide interception of all sessions including non-docket
work, and coupling to unowned, version-mobile harness surface, outweigh the hard block — given
that a transcript-verifiable caller-side gate can reach the same oracle at the same seam-moment
once the parent-facing surface exists.

## Risks

- **The gate is model-followed prose.** Mitigations in §3; residual risk accepted explicitly,
  with the hook as the recorded escalation. The `aborted-run` floors stand regardless.
- **Always-in-context real estate.** The block must stay a few lines or it degrades the context
  it rides in — brevity is a build requirement.
- **A committed symlink is a new repo-root artifact.** Fine on macOS/Linux git; a Windows
  checkout without symlink support would materialize it as a text file. Accepted for a
  solo-maintainer macOS project; the ADR records the constraint so a future contributor-facing
  repo can revisit.
- **A second dispatch spends a full agent run on a false `run-incomplete`.** Same bound and
  mitigation as 0237: exact attribution via the snapshot diff; the cap holds the worst case to
  one wasted run.
- **Surface-creation logic touches repos docket doesn't own the contents of.** The block write
  is marker-bounded and the symlink is created only when `CLAUDE.md` is absent — never
  overwriting an existing file; the suite's combo matrix is the guard.
