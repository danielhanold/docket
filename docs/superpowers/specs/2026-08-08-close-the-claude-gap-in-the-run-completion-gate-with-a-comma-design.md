<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0242 — Close the Claude gap in the run-completion gate with a caller-side verify in the dispatch rules](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0242-close-the-claude-gap-in-the-run-completion-gate-with-a-comma.md)**
<!-- docket:backlink:end -->

# Design — close the Claude gap in the run-completion gate at the caller's seam

Change 0242. Wires the one uncovered dispatch path onto the oracle change 0237 built.

> **Supersedes the same-day Stop-hook draft of this spec** (first committed 2026-08-08, same
> path). The hook design is recorded under *Rejected* below; the change's slug retains the word
> "hook" because slugs are filenames and do not chase design pivots.

## Problem

Change 0237 gave the terminal-disposition contract its missing consumer — `docket.sh verify-run`,
a pure git reader of Step 7's postcondition — and a caller at the dispatch seam docket owns,
`runner-dispatch.sh`. Every harness whose autonomous runs are CLI-driven (`codex`, `cursor`,
`opencode`, future adapters) dispatches through it and is covered.

The uncovered surface is **Claude interactive sessions**: a human types
`/docket-implement-next` (or a `/loop` over it) and the harness dispatches the skill as a fork
itself — `runner-dispatch.sh` is not on that path. All six observed instances of the half-run
family (0109, 0194 ×2, 0206, 0231, 0235) happened on exactly that path; it is covered today only
by `board-checks.sh`'s `aborted-run` legs and their 2h/12h floors.

The structural fact the fix must respect: any check placed **inside** the skill is executed by
the same agent that is failing — an agent that stops at a step boundary also stops before its own
"now verify yourself" step, which is the class of self-addressed prose that has failed six times.
The check must run in a context that regains control **after** the failing agent has stopped. On
the interactive path, that context is the **parent session that dispatched the fork**.

## Decisions

Settled with the human during grooming, 2026-08-08 (registration/attribution/cap decisions from
the same session's hook draft carry forward where they still apply).

1. **The mechanism is a caller-side verify on fork return, carried by the generated dispatch
   rules.** The agent layer already generates, per harness, the dispatch rule that routes a
   directly-invoked `docket-implement-next` to its pinned wrapper (`sync-agents.sh`, into each
   harness's agent-instructions file). That rule — read by the *parent* session, not the fork —
   grows the gate: snapshot before dispatch, verify after return, one bounded re-dispatch. This
   ships with the repo's existing sync, covers every invocation shape (one-offs and every
   `/loop` iteration, each of which is itself a fork return), and needs no per-machine or
   per-prompt discipline.
2. **A Claude Code `Stop`/`SubagentStop` hook is rejected** (recorded fully under *Rejected*).
   It was the only variant with a hard mechanical block, but the human judged it too heavy: with
   user-level registration it intercepts every turn end and every subagent completion machine-wide
   — including all non-docket work — and it couples to harness surface docket does not own
   (transcript format, hook protocol, settings schema), a coupling the
   `harness-behavior-is-mode-and-version-scoped` finding warns is version-fragile.
3. **No loop-specific mechanism.** A documented verify line for `/loop` prompts was considered
   and dropped: each loop iteration is a fork return, so the dispatch-rule gate already covers
   it, and `/loop` exists in exactly one harness. A second statement of the same rule in a second
   place is the restatement pattern the learnings ledger warns about.
4. **A headless-Claude runner adapter is rejected on cost grounds**: the human's Claude
   subscription does not cover headless (`claude -p`) invocations, so routing Claude runs
   through `runner-dispatch.sh` is not available.
5. **The gate mirrors 0237's shape wherever it can**: same oracle, same snapshot-diff
   discriminator, same one-bounded-re-dispatch cap, same `run-halted`-never-re-dispatches rule.
   No second verdict logic exists anywhere.

## Design

### 1. The caller-side gate, as dispatch-rule text

The generated dispatch rule for `docket-implement-next` (per harness, written by
`sync-agents.sh` from its template source) gains the gate procedure, addressed to the parent
session:

1. **Before dispatching**: record the current in-progress set —
   `docket.sh verify-run --in-progress-ids`.
2. **Dispatch the fork** and block on its return, as the rule already directs.
3. **After return**: re-run `docket.sh verify-run --in-progress-ids`; any id not in the
   before-set is this run's claim (the same discriminator `runner-dispatch.sh` uses — a foreign
   concurrent claim was in the before-set and is ignored). An empty diff (`drained`, lost CAS)
   ends the gate as a no-op. An explicitly-passed id may be verified directly, but the snapshot
   still runs — it is two cheap commands, and the uniform procedure keeps the rule short.
4. **Verify**: `docket.sh verify-run <id>`, keying on the report line:
   - `run-complete` / `run-unclaimed` → done.
   - `run-halted` → done; never re-dispatch (a halt means a human is needed).
   - `run-incomplete` → **re-dispatch the same skill once**, passing the change id and the
     unmet conjuncts as task context. After the second return, verify again: complete/halted →
     report; `run-incomplete` again → **stop and report loudly**, naming the change and the
     unmet conjuncts. The change stays `in-progress` with its claim intact; the `aborted-run`
     legs remain the backstop. Never a third dispatch.

The rule text is short, imperative, and mechanical — every step is a single facade command whose
execution (or absence) is visible in the parent's transcript.

### 2. Trust posture — stated honestly

This gate is an instruction a model follows, and the family exists because prose degrades. It is
nonetheless differently positioned from the six failed levers, on three axes: it is addressed to
the **parent** (the non-failing agent, whose remaining job is short), it is **a handful of
mechanical commands** rather than a multi-step behavioral contract, and its execution is
**transcript-verifiable** — a degraded gate shows as a missing command, not as a plausible
summary. The hard-mechanical alternative (the hook) was rejected on weight, with eyes open; if
the caller-side gate is ever observed degrading, the hook design under *Rejected* is the
escalation path, already worked out.

### 3. Delivery — where the text lives and how it syncs

- The rule template lives wherever `sync-agents.sh` sources dispatch-rule text today (build-time
  reconcile locates the exact file; the build follows the existing generation path, adding no
  new mechanism).
- `sync-agents.sh` regenerates each configured harness's instruction file
  (`agent_harnesses:`), so the gate reaches consuming repos through the sync they already run.
  Harnesses beyond Claude get the same rule; on their interactive paths it is equally
  applicable, and on their autonomous paths `runner-dispatch.sh` remains the enforcing seam —
  the two gates share the oracle, so double coverage is harmless (`verify-run` is a pure
  reader; the runner's gate acts first, and a parent re-checking afterward sees
  `run-complete`).
- The convention's *Composition* prose (caller "verifies the child's git-state transition")
  gains a pointer naming the dispatch-rule gate as that obligation's mechanical form for
  interactive dispatch — a sentence, not a restatement of the procedure.

### 4. What this change does not touch

`verify-run.sh`, `runner-dispatch.sh`, and `board-checks.sh` are consumed as-is — 0237's
surface, unchanged. No hook, no `settings.json`, no `ensure-docket-env.sh` work. No metadata
write, status flip, or claim release by the gate: it reads, re-dispatches at most once, and
reports.

## Scope

**In:**
- The gate procedure added to the `docket-implement-next` dispatch-rule template;
  regeneration of the per-harness instruction files via `sync-agents.sh`.
- The one-sentence pointer in the convention's *Composition* prose.
- Tests: sentinel coverage in the sync-agents suite asserting the generated rule carries the
  gate (snapshot command, verify command, the one-re-dispatch bound, the run-halted
  no-re-dispatch rule) — anchored per the `prose-guard-binds-phrase-to-claim` and
  `specified-but-unreachable` findings. Live compliance (a real parent session executing the
  gate) is external truth with no in-repo oracle — routed to a named human-verification item
  per the `external-truth-needs-a-human-checkpoint` finding.

**Out:**
- Any Claude Code hook (see *Rejected*), any `settings.json` or installer work.
- Any loop-specific documentation or mechanism (Decision 3).
- Any headless-Claude runner adapter (Decision 4).
- Any change to `verify-run.sh` / `runner-dispatch.sh` / `board-checks.sh`; any new config
  knob; any status flip or claim release; any re-derivation of verdict logic.

## Rejected — the command-type Stop hook (this spec's first draft, same day)

For the record, and as the worked-out escalation path should the caller-side gate degrade:
a `scripts/claude-stop-hook.sh` registered user-level for `Stop` + `SubagentStop` via
`ensure-docket-env.sh`; self-gating four-leg sequence (repo gate → transcript-derived session
gate → attribution with `claimed_at`-epoch fallback via
`verify-run --in-progress-ids --with-claimed-at` → verdict); exit 2 blocks the stop and feeds
the unmet conjuncts to the still-alive agent; capped at one block per session×change; fail-open
on any internal error; blocking build-time re-probe of the hook protocol at the current CC
version. Rejected 2026-08-08: machine-wide interception of all sessions including non-docket
work, and coupling to unowned, version-mobile harness surface, outweigh the hard block — given
that a transcript-verifiable caller-side gate reaches the same oracle at the same seam-moment.

## Risks

- **The gate is model-followed prose.** Mitigations are §2's three axes; residual risk is
  accepted explicitly, with the hook as the recorded escalation. The `aborted-run` floors stand
  regardless.
- **Dispatch-rule real estate.** Instruction files are always-in-context; the gate must stay a
  few lines, or it degrades the context it rides in. The build should treat brevity as a
  requirement, not a style preference.
- **A second dispatch spends a full agent run on a false `run-incomplete`.** Same bound and same
  mitigation as 0237: the snapshot diff keeps attribution exact, and the cap holds the worst
  case to one wasted run.
- **Rule-template drift across harnesses.** The build must confirm `sync-agents.sh` regenerates
  every configured harness's file from one template source, so the gate cannot fork into
  per-harness variants silently.
