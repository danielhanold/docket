<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0237 — Prose levers fail to hold the step boundary — give the disposition contract a consumer](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0237-prose-levers-fail-to-hold-the-step-boundary-add-a-mechanical.md)**
<!-- docket:backlink:end -->

# Design — a mechanical consumer for the terminal-disposition contract

Change 0237. Supersedes and absorbs change 0236 (killed into this one).

## Problem

An autonomous `docket-implement-next` run is defined by seven steps ending in an open PR. Six times
it has executed some prefix of them and reported success:

| # | Change | Boundary | Lineage |
|---|---|---|---|
| 1 | 0109 | — | original |
| 2–3 | 0194 (×2) | Step 7 seam | 0113 |
| 4 | 0206 | — | 0113 |
| 5 | 0231 (filed as 0236) | Step 4/5 — plan → build | 0096 → 0113 |
| 6 | 0235 (filed as 0237) | Step 5/6 — build → review | 0212 |

Every remedy docket has shipped for this family is **prose addressed to the agent that is failing**:
0096's call-site pre-specification (ADR-0044), 0113's split §5 sentence and the *Step postconditions*
table, 0212's mode-conditioned scoping clause (ADR-0069), and the four-value terminal-disposition
contract. Instances 5 and 6 each occurred with every applicable lever present, correctly worded, and
applicable by its own terms.

Instance 6 is the sharpest datum because it is the **first live run to exercise 0212's fixed path**.
`/docket-implement-next 235` (agent `ac44ea6eb7832da86`, 2026-08-07) ran Steps 0–5 in full — six
build commits, suite green at `files=87 passed=87 failed=0 asserts=6347` — then ended its turn at the
Step 5/6 boundary with the branch unpushed, no review, no PR, and the manifest still `in-progress`.
Its closing line was *"Build disposition (role-scoped): green"* — `docket-build`'s vocabulary, not the
driver's, which is precisely the independently-checkable tell 0212 introduced. Nothing checked it.

So this change is not a seventh point-fix. It is a response to the **fix strategy**.

### The structural gap, stated once

**The terminal-disposition contract has a producer and no consumer.** `advanced` is claimable only
when Step 7's postcondition holds — a statement entirely readable from git, that no code reads.

Evidence carried forward from 0236's grooming attempt (reconstructed git signature of instance 5,
change 0231):

| Fact | Evidence |
|---|---|
| Plan committed on the feature branch | `71d64d33`, 11:35:37 -0400 |
| `plan:` landed on `metadata_branch` | `2107f884`, 11:36:02 -0400 |
| Build commits at the moment of return | none — next branch commit `20b70b62` at 11:43:01, after the human's resume |
| Branch pushed / `pr:` | no / unset |
| Disposition declared in the fork's report | **none** |

Step 4's postcondition held in full, on both trees. The run stopped on a *satisfied intermediate
row* — the stop `docket-implement-next` forbids twice in prose. Both rules were live; both were
violated in one report.

The one non-prose lever, `board-checks.sh`'s `aborted-run`, could not catch instance 5 either: leg A
was blind (`plan:` *was* set), leg B's horizon is 12h against a 15-minute-old claim, and leg C's 2h
branch-idle floor had not elapsed when the human resumed after ~8 minutes. Change 0219 has since
added leg D (Step 7 seam, `pr:` set while `status: in-progress`, time-free), which closes a real gap
but not this one.

## Decisions

Settled with the human during grooming, 2026-08-07.

1. **0236 and 0237 are one change.** They were filed as two step boundaries (4/5 and 5/6) of one
   family. Both stubs independently concluded the family shares a root cause, and both proposed the
   same class of remedy. A per-boundary split reproduces the family's own failure mode — each fix
   misses the boundary it did not name. 0236 is killed into this change; its settled evidence is
   carried above.
2. **The mechanism is a git-only checker plus a caller at a seam docket owns.** Not a fourth prose
   rider, and not a self-report the failing agent authors.
3. **Cross-harness is a hard constraint.** A Claude Code `Stop` hook was investigated and confirmed
   available (command-type, exit 2 blocks the turn end and feeds stderr back). It is **rejected as
   this change's mechanism** because it covers exactly one harness and is the only candidate whose
   code docket does not own. It is deferred to its own stub.
4. **When the gate fires, the run is given one bounded chance to finish**, and a legitimate halt
   must be **written into git** rather than narrated. This is what makes a `halted` disposition
   verifiable — 0236's critic named self-declared `halted` as the load-bearing untrusted input, and
   requiring a git act rather than a claim resolves it without trusting the worker.

## Design

### 1. `verify-run.sh` — the consumer

A new deterministic script behind the facade: `docket.sh verify-run <id>`. Git and filesystem only;
no network, no `gh`, no harness. It evaluates **Step 7's postcondition** for one change and reports a
verdict.

Conjuncts, all read with the anchored `fm_field` (ADR-0057), from the metadata working tree:

| Conjunct | Read |
|---|---|
| status advanced | `status: implemented` |
| PR recorded | `pr:` non-empty |
| branch delivered | `origin/<branch:>` resolves |

Verdict vocabulary, one report line on stdout — the same report-line house pattern the Board pass
uses, where **callers key on the line, never on the exit code**:

- `run-complete <id>` — every conjunct holds.
- `run-halted <id>` — a `## Run halted` record is present; the run ended deliberately.
- `run-incomplete <id> <unmet conjuncts>` — one or more conjuncts unmet.
- `run-unclaimed <id>` — the change is not `in-progress` and not `implemented`; there is no run to
  verify (`proposed` after a reclaim, `deferred`, or archived).

Exit codes: **0 whenever a verdict was produced**, non-zero only when the check itself could not run
(unreadable change file, unresolvable config, not a repo). This follows the learnings finding
`exit-code-encodes-a-non-failure`: `run-incomplete` is a finding, not a script failure, and a bare
non-zero consumer must not read it as one.

**What it deliberately does not do.** It flips no status, releases no claim, writes no file, and
shells no `gh`. Pure reader, matching `board-checks.sh`'s contract and `aborted-run`'s advisory
posture. The *only* thing that acts on a verdict is the caller in §2.

**No time floor.** This is the whole point of the script's existence, and it is only sound because of
where it is called: at a seam where the child process has already returned, nothing is still moving,
so "stopped" and "still working" are not ambiguous. `board-checks.sh` cannot make that assumption and
therefore keeps its floors — see §4.

### 2. `runner-dispatch.sh` — the caller

`runner-dispatch.sh` currently ends with `exec "$DOCKET_BASH_PATH" "$ADAPTER" …`, handing off and
never regaining control. It becomes a call-and-return, and on return runs the gate.

This is the seam docket **owns**. Every non-Claude harness (`codex`, `cursor`, `opencode`, and any
future adapter) delegates through it, so one edit covers all of them with no harness cooperation,
no hook, and no `settings.json`.

**Which change to check — the snapshot diff.** No session identity, no marker file, no new
frontmatter field:

1. Before handing off, record the set of change ids currently `in-progress`.
2. Hand off; the adapter runs foreground as today.
3. On return, re-sync the metadata working tree (a fresh `docket.sh preflight`, per the convention's
   re-sync rule and the `cas-re-read-fresh-origin` finding — the "after" read must come from fresh
   origin state, never from the local tree the child just wrote).
4. Re-read the in-progress set. Any id **not** in the before-set is this run's claim.
5. Run `verify-run <id>` on each.

A change another agent already held was in the before-set and is ignored, so concurrent runs do not
cross-fire. A run that claimed nothing (`drained`, or `contended` where the CAS was lost) yields an
empty diff and the gate is a no-op.

**Agent gating — the gate engages only for `--agent implement-next`.** This is load-bearing, not an
optimization: a `build-*` delegation leaves its change `in-progress` **by design** (the build role
does not reach Step 7), so running the gate on one would fire on every healthy build. `status`,
`adr`, `review-*`, `finalize-change` and `auto-groom` delegations are likewise out of scope. An
unrecognised agent is a no-op, never a guess.

**Action on `run-incomplete` — one bounded re-dispatch.** Re-invoke the same adapter once, with the
change id and the unmet conjuncts as task context. This mirrors `docket-build`'s one-escalation-per-
task rule, which is docket's existing precedent for "give it exactly one more chance, then stop."
After the second return, re-run `verify-run`:

- `run-complete` / `run-halted` → report and exit with the adapter's own exit code.
- `run-incomplete` again → **abort-and-report**, loud and non-zero, naming the change and the still-
  unmet conjuncts. The change stays `in-progress` with its claim intact; `aborted-run` remains the
  standing backstop.

`run-halted` never re-dispatches — a halt means a human is needed, and spending a second full agent
run on it is waste.

**Exit-code preservation.** With `exec` gone, `runner-dispatch.sh` must propagate the adapter's exit
code verbatim when the gate takes no action, so no existing caller observes a behavior change. Only
the two-strikes abort introduces a new non-zero, and only on a path that is presently silent.

### 3. `## Run halted` — the escape that cannot be narrated

A new change-body section, in the same family as `## Auto-groom blocked` and `## Finalize blocked`:
presence-encoded state, dated entry, written by the halting run, naming why the run stopped needing a
human.

A run that genuinely cannot proceed clears the gate by **writing this section and committing it** —
a git act, observable to `verify-run` and to any later reader. It cannot be satisfied by a sentence
in a completion report, which is exactly the property every prior lever lacked.

Per the `presence-encoded-state` finding, every transition out must remove it: **removal is owned by
`docket-implement-next`'s Step 2 claim**, which is the only transition back into a live run. Stated
once, in that skill, and not restated elsewhere.

Board rendering of the section is **out of scope** — `aborted-run` already surfaces the change, and a
new board cell is scope this change does not need.

### 4. What `board-checks.sh` does *not* become

The first draft of this design had `aborted-run`'s four legs delegate to the shared postcondition
definition. **Rejected.** The legs do not each evaluate the Step 7 postcondition; they evaluate four
different partial evidences with three different floors, and change 0219 rewrote that block hours
before this design was written. Refactoring a freshly-landed, explicitly cost-sensitive path
(change 0176) for zero behavior change is risk without return.

`board-checks.sh` is therefore **untouched**. The board keeps its floors, because a board pass looks
at a repository and genuinely cannot distinguish a stopped run from a live one — change 0219's own
branch, mid-build at the moment this spec was written, was byte-indistinguishable in git from an
aborted run.

`board-checks.md`'s `## Not covered` paragraph gains one sentence pointing at `verify-run` as the
floor-free check available at a dispatch seam. Documentation only.

## Scope

**In:**
- `scripts/verify-run.sh` + `scripts/verify-run.md` (the co-located contract every script carries).
- Facade registration: `WRAPPED_OPS` in `scripts/docket.sh` **and** the operations table in
  `scripts/docket.md` — the sentinel test greps both, so they land together.
- `scripts/runner-dispatch.sh`: `exec` → call-and-return, snapshot diff, agent gate, one bounded
  re-dispatch, exit-code preservation. `scripts/runner-dispatch.md` updated to match.
- `## Run halted` in the convention's *Change body sections* list; its removal rule in
  `docket-implement-next` Step 2.
- One sentence in `scripts/board-checks.md`'s `## Not covered`.
- Tests: `tests/test_verify_run.sh` (new) and extensions to the runner-dispatch suite covering the
  snapshot diff, the agent gate, the bounded re-dispatch, and exit-code preservation.

**Out:**
- Any Claude Code `Stop` / `SubagentStop` hook, and any `settings.json` or installer work. Files as
  its own stub — it is the only path that catches a **Claude** run at the moment it stops, which is
  where all six incidents occurred, so it should be tracked, not forgotten.
- Any change to `board-checks.sh`'s legs or floors (§4).
- Any new config knob. The gate is unconditional for `implement-next` delegations and bounded at one
  re-dispatch, matching `aborted-run`'s hardcoded-horizon precedent.
- Any status flip or claim release. The advisory posture holds; only the re-dispatch acts, and it
  acts by running an agent, not by editing state.
- A seventh prose rider anywhere in the skill bodies.

## Risks

- **The gate does not cover Claude.** Claude Code dispatches subagents itself, so `runner-dispatch.sh`
  is not on that path. Claude runs are covered only by `aborted-run`'s floors, exactly as today. This
  is the honest cost of the cross-harness constraint, and it means the six observed incidents would
  each still have gone uncaught at the moment they happened. What the change buys is that the
  contract acquires a real consumer, reachable by hand and by every non-Claude runner — and the
  deferred hook stub becomes a two-line wiring job onto an oracle that already exists.
- **Removing `exec` changes process semantics** — signal delivery and the process tree both shift.
  Exit-code preservation is specified; signal behavior needs a deliberate check at build time.
- **One re-dispatch is a real cost.** A false `run-incomplete` spends a full agent run. The snapshot
  diff and the `implement-next` agent gate exist to keep the false-positive rate near zero; the
  two-strikes cap keeps the worst case bounded at exactly one wasted run.
- **`## Run halted` is a new presence-encoded section**, and the family's failure mode is stale
  presence (the `presence-encoded-state` finding, and the re-arm rule `## Auto-groom blocked`
  needed). Its removal must land in the same change as its writer, not after.
