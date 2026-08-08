<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0242 — Close the Claude gap in the run-completion gate with a command-type Stop hook](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0242-close-the-claude-gap-in-the-run-completion-gate-with-a-comma.md)**
<!-- docket:backlink:end -->

# Design — close the Claude gap in the run-completion gate with a command-type Stop hook

Change 0242. Wires the one uncovered harness onto the oracle change 0237 built.

## Problem

Change 0237 gave the terminal-disposition contract its missing consumer — `docket.sh verify-run`,
a pure git reader of Step 7's postcondition — and a caller at the dispatch seam docket owns,
`runner-dispatch.sh`. That covers `codex`, `cursor`, `opencode`, and every future adapter.

It does not cover **Claude**, because Claude Code dispatches subagents itself and
`runner-dispatch.sh` is not on that path. All six observed instances of the half-run family
(0109, 0194 ×2, 0206, 0231, 0235) happened under Claude; Claude runs today are covered only by
`board-checks.sh`'s `aborted-run` legs and their 2h/12h floors. 0237 deferred this deliberately —
a harness hook covers exactly one harness and is the only candidate whose code docket does not
own — but with the oracle built, closing it is a wiring job, not a design problem.

The mechanism was investigated and confirmed available during 0237's grooming (2026-08-07,
Claude Code 2.1.x): `Stop` and `SubagentStop` are live hook events; a **command**-type hook
receives `session_id`, `transcript_path`, `cwd`, and `hook_event_name` as JSON on stdin, and
**exit 2 blocks the stop and feeds stderr back to the agent** — the block signal doubles as the
continue-instruction, with the agent still alive to act on it. A prompt-type hook is another model
reading prose and would reintroduce the exact defect this family is about; only the command type
qualifies.

## Decisions

Settled with the human during grooming, 2026-08-08.

1. **Registration is user-level, via the install seam.** One entry in `~/.claude/settings.json`
   per machine, written idempotently by `ensure-docket-env.sh` — the same seam that injects
   `DOCKET_SCRIPTS_DIR`. It covers every docket repo on the machine, including future ones, with
   no per-repo drift; the hook script self-gates to a fast exit 0 everywhere else. The per-repo
   alternatives were rejected: a committed `.claude/settings.json` registers a Claude-specific
   hook for every teammate without opt-in and puts harness-specific config on the integration
   branch; a gitignored `settings.local.json` is N repos × M machines of drift, and a repo whose
   entry is missing silently loses the gate — the wrong failure shape for a net whose purpose is
   catching silent failures.
2. **Attribution is transcript-derived.** The transcript at `transcript_path` is written by the
   harness, not the agent — evidence the failing agent cannot forget to produce. A run-stamped
   marker (the stub's other candidate) is a prose obligation on exactly the agent class 0237
   stopped trusting, plus a new presence-encoded artifact with removal obligations. A bare
   time-window check cross-fires: this machine runs concurrent autonomous loops alongside
   interactive sessions in the same repos, and a session stopping while a foreign loop is mid-run
   must not be blocked on a claim it does not own.
3. **The livelock bound is one block per session×change, then allow.** Mirrors 0237's
   one-bounded-re-dispatch precedent (itself mirroring `docket-build`'s one escalation per task).
   After the cap, the stop is allowed and the change stays `in-progress` for the standing
   backstops. Block-until-halted was rejected: an agent that cannot act on the stderr instruction
   loops the session indefinitely, with no recourse but editing `settings.json` mid-wedge.
4. **This is a Claude-specific adapter, not a shared stop-gate core.** The harness-neutral part
   already exists — `verify-run` is the oracle — so the adapter is thin by construction. A second
   harness with a comparable stop event writes its own adapter then, informed by a real second
   data point rather than an abstraction extrapolated from one example.

## Design

### 1. `claude-stop-hook.sh` — the adapter

A new script behind the facade's directory (`scripts/claude-stop-hook.sh` + co-located
`scripts/claude-stop-hook.md` contract), registered for **both** `Stop` and `SubagentStop` —
the same script serves both; `hook_event_name` on stdin distinguishes them. Both events are
needed: a forked `docket-implement-next` run ends at `SubagentStop`, an inline one at `Stop`.

It is a translator, not a second oracle: it decides *whether* and *for which id* to invoke
`docket.sh verify-run`, and maps the verdict onto the hook protocol. It re-derives none of
0237's verdict logic.

**Gate sequence.** Each leg exits 0 fast when it fails — the machine-wide common case (a turn
ending in a non-docket repo) must cost milliseconds:

1. **Repo gate.** `cwd` is inside a docket repo — a cheap filesystem/git probe before any full
   config resolution.
2. **Session gate.** The session's own transcript (at `transcript_path`) shows a
   `docket-implement-next` invocation. Absent → this is not a run-driver session; exit 0.
3. **Attribution.** Extract the claimed change id from the transcript. Fallback when the id
   cannot be parsed: the in-progress ids whose `claimed_at` epoch postdates the transcript's
   first entry, read via `verify-run --in-progress-ids --with-claimed-at` (the reader built for
   exactly this consumer shape — the hook compares integers and owns no frontmatter parsing).
   Nothing attributable → exit 0.
4. **Verdict.** `docket.sh verify-run <id>`, keying on the report line per the house pattern:
   - `run-complete` / `run-halted` / `run-unclaimed` → exit 0.
   - `run-incomplete` → **exit 2**, stderr naming the change id and the unmet conjuncts
     (`status` / `pr` / `branch`) as the continue-instruction to the still-alive agent.

**The cap.** At most one exit-2 block per session×change, tracked by a small state file keyed on
`session_id` (session ids are unique and never reused, so the presence-encoded lifecycle is
trivial — no removal transition can go stale; the file lives in a tmp location and dies with the
machine's tmp hygiene). Cap already spent → allow the stop. `## Run halted` remains the
legitimate mid-session escape at any point: writing and committing it flips the verdict to
`run-halted`, which passes the gate — the same cannot-be-narrated escape 0237 built.

**Fail-open invariant.** Any internal failure of the hook itself — unreadable stdin, missing
transcript, unresolvable config, absent `DOCKET_SCRIPTS_DIR`, `verify-run` exiting 2 — is
exit 0, never exit 2. A machine-wide hook must never be able to wedge unrelated sessions; the
runner-side gate and the `aborted-run` legs remain the standing backstops. (Per the
`exit-code-encodes-a-non-failure` finding read in reverse: here the *hook's* non-zero is an
instruction to the harness, so it must be reserved for the one deliberate case.)

### 2. Registration — `ensure-docket-env.sh`

The existing installer helper grows an idempotent upsert of the hook entry into user-level
`~/.claude/settings.json`: matchers for `Stop` and `SubagentStop`, command referencing the
script through `$DOCKET_SCRIPTS_DIR` (already injected into the settings `env` by the same
helper — no hardcoded path, so a moved clone keeps working after re-install). Idempotency
matches the helper's existing posture: re-running install converges, never duplicates.

Per-machine rollout is therefore: run `install.sh` on that machine — the same act that makes any
docket helper reachable there. No per-repo step exists.

### 3. Build-time spike (blocking)

Per the `harness-behavior-is-mode-and-version-scoped` finding, the 0237-era confirmation is an
observation scoped to the version and mode it was seen in. Before the adapter is built, re-probe
at the current Claude Code version, in the exact dispatch modes docket uses (forked skill and
interactive session):

- exit 2 blocks the stop and feeds stderr back, for **both** `Stop` and `SubagentStop`;
- the stdin JSON carries the documented fields;
- the transcript format supports the session-gate grep and the id extraction.

A failed probe is a design input, not a build obstacle to route around — it goes back to the
human.

## Scope

**In:**
- `scripts/claude-stop-hook.sh` + `scripts/claude-stop-hook.md`.
- The registration upsert in `ensure-docket-env.sh` (+ its doc).
- The build-time re-probe spike, recorded in the results file.
- Tests: a hermetic suite driving the gate sequence, transcript attribution and its
  claimed-at fallback, the cap, verdict mapping, and the fail-open posture via fixture stdin
  JSON and fixture transcripts. Live hook firing (a real Claude session blocking on exit 2) is
  external truth with no in-repo oracle — routed to a named human-verification item per the
  `external-truth-needs-a-human-checkpoint` finding.

**Out:**
- Any second harness's stop event, and any shared stop-gate abstraction (Decision 4).
- Any change to `verify-run.sh`, `runner-dispatch.sh`, or `board-checks.sh` — the oracle and the
  runner seam are 0237's shipped surface, consumed as-is.
- Any status flip, claim release, or metadata write by the hook. It is a pure reader plus an
  exit code.
- Any prompt-type hook, and any re-derivation of verdict logic outside `verify-run`.
- Any new config knob. The gate is unconditional for attributable implement-next sessions,
  bounded at one block — matching 0237's no-knob precedent.

## Risks

- **The hook couples to harness surface docket does not own.** Transcript format, hook protocol,
  and settings schema can all move under a Claude Code upgrade. Mitigations: the blocking spike
  pins the version the build validates against; the fail-open invariant turns any future format
  drift into a silently-absent net (exactly today's status quo) rather than wedged sessions; and
  the runner-side gate plus `aborted-run` legs stay standing regardless.
- **Transcript grep is attribution by string-matching.** A session that merely *discusses*
  docket-implement-next could false-positive the session gate; the attribution leg then finds
  either a real claimed id (in which case verifying it is correct regardless of how the session
  was classified) or nothing (exit 0). The failure cost is one spurious verify-run read, not a
  block.
- **Machine-wide firing.** Every turn end on the machine pays the repo-gate probe. The gate
  sequence is ordered cheapest-first to keep that cost to a few milliseconds; the spike should
  confirm the observed overhead is imperceptible.
- **A blocked stop spends agent turns in an already-failing session.** Bounded at one block; the
  stderr instruction names the unmet conjuncts so the continuation is directed, not a blind
  retry.
