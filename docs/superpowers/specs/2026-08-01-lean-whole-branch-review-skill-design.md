<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0170 — Lean Docket-owned whole-branch review skill](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0170-lean-whole-branch-review-skill.md)**
<!-- docket:backlink:end -->

# Lean whole-branch review skill + suite-once evidence chain — design

**Change:** 0170 · **Date:** 2026-08-01 · **Groomed:** interactive brainstorm (Daniel + session)

## Context

Change 0167 replaced Superpowers SDD with docket's own build role: `docket-build` routes each
plan task to a profile-pinned worker, performs no per-task review and no final review of its
own, and ends with a single full-suite gate (red → one bounded integration-repair ladder).
It deliberately preserved exactly one independent review: `docket-implement-next` Step 6's
`skills.review` role, still bound to the default `superpowers:requesting-code-review`.

That default leaves docket's only remaining review boundary outside its control — session
model/effort, recursive reviewer/fix subagents, verbose reports — and its checklist ("All
tests passing?") re-runs the full suite on the exact branch state the build gate just tested.
With a ~10-minute suite, one change currently pays for **three** full runs:

1. the build gate (mandatory, owns the repair ladder),
2. the Step 6 reviewer's re-run (same branch state — pure duplication),
3. finalize's `gate: local` post-rebase run (a genuinely new state — the only justified re-run).

This change designs the replacement reviewer **and** the evidence chain that removes runs 2
and (conditionally) 3.

## Goals

- One bounded, read-only, whole-branch reviewer with an explicit model and effort, returning
  structured findings to the controller and never fixing anything.
- The full suite runs **exactly once** during implementation (the build gate), certified by a
  durable evidence block.
- Finalize's post-rebase run becomes conditional: skipped only when the state it would test is
  provably identical to an already-tested state.
- No new fix machinery: blocker remediation reuses the 0167 `docket-build-task` worker contract.

## Non-goals

- Per-task review (removed by 0167; stays removed).
- Reviewer-side fixes or any recursive reviewer/fix agents.
- Changes to the build profiles, rubric, or TDD contract (0167/0184).
- A second review round after blocker fixes.

## Pipeline — before and after

```
TODAY (per change: 3 full-suite runs, ~30 min)

docket-implement-next
  Step 5: docket-build ──────────► SUITE RUN #1 (build gate; red → repair ladder)
  Step 6: superpowers review ────► SUITE RUN #2 (same branch state — pure duplication)
  Step 7: open PR ── stop
                     ...human approves...
docket-finalize-change
  rebase onto main ─────────────► SUITE RUN #3 (post-rebase state — always)


AFTER 0170 (per change: 1 run clean-path, 2 worst-case)

docket-implement-next
  Step 5: docket-build ──────────► SUITE RUN #1 (unchanged, sole implementation home)
             └─ records EVIDENCE: {command, result, HEAD SHA, timestamp}
  Step 6: docket-review (NEW) ───► reads diff + EVIDENCE, runs NO suite
             ├─ blockers → one fix task (docket-build-task contract)
             │     └─ fix commits landed? → suite re-runs ONCE, evidence refreshes
             └─ important/minor → PR body
  Step 7: open PR + evidence block in PR body ── stop
                     ...human approves...
docket-finalize-change
  rebase onto main
     ├─ no-op rebase AND evidence green AND SHA matches ──► SKIP suite
     └─ anything else ─────────────────────────────────────► suite runs
```

Division of labor: the **reviewer finds**, the **controller decides and routes**, the
**build-task worker fixes**. The reviewer's read-only purity is what makes its verdict
trustworthy — it has no incentive to under-report what it would otherwise have to fix.

## Why the suite lives in the build gate, not the reviewer

Record this rationale in `README.md` at build time — it is the design's central placement
decision and the question a newcomer will ask first.

1. **The suite answers the build's question.** "Does what I assembled work together?" is a
   completion check on the build — workers run focused tests only, so cross-task integration
   breakage is a build defect. Review asks a different question: "is this good?" — judgment
   over the diff, which is what the pinned reviewer model is paid for. A test run needs no
   model; it is deterministic bash.
2. **The repair machinery lives on the build side.** 0167 gave the gate its red-suite answer:
   one synthetic integration-repair task through the worker ladder. A suite run inside a
   reviewer that is forbidden to fix would have to hand failures back out, re-enter build
   machinery, and then face "does the fixed branch need re-review?" — the build→review→build
   loop this change exists to kill. Suite-in-build means review only ever starts from a
   known-green branch: one pass, one direction, no cycles.
3. **Gate-first ordering is cheaper on failure.** A red suite discovered after the expensive
   whole-branch read wastes the entire review on a branch that was never shippable. Cheap
   deterministic check first, expensive judgment second — the same reason CI runs before a
   human looks at a PR.
4. **The evidence chain follows naturally.** The thing that last mutated the branch certifies
   it. The build mutates; review consumes the certificate; finalize re-checks only when the
   state genuinely changes (rebase onto a moved base).

The mental model: the suite is not "at the end of build" so much as it is **the boundary
between build and review**, owned by the side that can fix a failure — exactly like CI status
checks on a PR, with the reviewer as the human-style reviewer who reads the diff and trusts
the green check.

## Component 1 — the `docket-review` skill

A new skill implementing docket's review role, invocable via `skills.review`.

**Dispatch shape.** `docket-implement-next` Step 6 dispatches the pinned wrapper agent once,
**foreground** (the parent actively blocks). Foreground is forced by three facts: the findings
are load-bearing for the very next step (Step 7 cannot open the PR before triage); a read-only
agent has no git-state channel, so the in-context return is its only output path (the same
shape as finalize's rebase-resolver and integration-repair dispatches); and ADR-0024 rules out
background-and-yield (a forked child has no notification channel — the caller would read a
half-done run as `completed`). The cost is bounded: unlike the old default, this reviewer runs
no suite — the parent blocks for one model read of the diff, not ten minutes of bash.

**Foreground ≠ inline — the pin survives the dispatch.** Foreground/background is a
*scheduling* axis (does the parent block on the child's return); inline/dispatched is an
*execution* axis (parent's own context vs. a separate subagent). This reviewer is a
foreground **dispatch**: a fresh subagent spun up from the `docket-review` wrapper, which
carries the resolved `model:`/`effort:` pin — so the review runs at opus-5/medium regardless
of the model `docket-implement-next` itself is running at, exactly as the build workers and
`docket-adr` run at their own pins today. The parent's model reaches the review only on the
degraded **inline** path (`skills.review: auto`, or Tier C's human-authorized inline
fallback), which is precisely why that path is authorized-or-halt with a loud warning: a
degraded binding drops the pin along with the discipline.

**Inputs (in the dispatch prompt):** branch name and base (`origin/<integration_branch>`);
the change's PM-altitude context (title, `## Why`, `## What changes`); the relevant learnings
hooks the controller already pulls at Step 6; and the current **evidence block**.

**Conduct:**
- Read-only: may run read-only commands (`git diff`, `git log`, `git show`, greps); never
  writes files, never commits, never checks out (shared-worktree rule), never dispatches
  subagents, and **never runs the test suite**.
- Verifies the evidence block's `head_sha` equals the branch HEAD it is reviewing. A missing,
  malformed, or stale block is reported as a blocker finding (`unverified-build-state`) — not
  a reason to run the suite itself.
- Reviews the whole branch diff for correctness, design soundness, contract violations, and
  test-coverage gaps the suite cannot see. It does not re-litigate profile routing or TDD
  mechanics (the build's own discipline).
- Returns the finding list (schema below) and stops. An empty list is a valid, expected return.

## Component 2 — wrapper agent + pins

One generated wrapper, `agents/docket-review.md`, joining the roster via the existing
`agents/docket-*.md` glob (post-0184: thirteen wrappers → **fourteen**).

- **Wraps the review skill only — no `docket-convention` injection.** Precedent: the
  `docket-build-*` profile workers, which perform no docket metadata operations. The reviewer
  is read-only over the feature branch; it touches no docket metadata.
- **Claude pin: `claude-opus-5` / `medium`** (settled during grooming: strong model for the
  sole independent review; medium effort as the cost point). Codex and Cursor rows mirror
  that tier in `agents/harness-defaults.yml`, with concrete model ids chosen at build time
  against the post-0184 table (verify at reconcile — PR #147 must have merged; see Coupling).
- Carries the standard abort-and-report posture of autonomous wrappers.

**Binding posture (mirrors 0167):** the shipped cross-harness default for `skills.review`
stays `superpowers:requesting-code-review`; this repository dogfoods `docket-review` via its
committed `.docket.yml`. Tier C (authorized-or-halt) dispatch posture from change 0137 is
inherited unchanged — an explicitly configured `auto` authorizes inline review; any other
resolved value that cannot dispatch is abort-and-report.

## Component 3 — the build-evidence block

The suite-once contract, produced by the build side, consumed twice.

**Fields:** `command` (the exact full-suite command run), `result` (`green` — a red result
never reaches the block; red enters the repair path), `head_sha` (the branch HEAD the run
tested), `ran_at` (UTC ISO-8601).

**Lifecycle:**
1. `docket-build`'s gate already emits "full-suite command and result" in its output; it now
   also emits the structured evidence line the controller captures in-context.
2. Any post-gate commit that lands during Step 6 (a blocker fix) triggers exactly one suite
   re-run by the controller; fresh evidence replaces the old.
3. At Step 7 the controller writes the block durably into the **PR body**, marker-bounded
   (`<!-- docket:build-evidence:start -->` / `<!-- docket:build-evidence:end -->`), alongside
   the review outcome (blockers fixed, important/minor findings).
4. `docket-finalize-change` reads the PR body's block for the skip predicate (Component 5).

The PR body is the right durable home: docket already writes it, finalize already reads the
PR, and it needs no new file or branch write. The marker-block edit pattern follows the
existing house rule (sole-writer, marker-bounded, never hand-edited).

## Component 4 — controller changes (`docket-implement-next` Step 6)

- Before dispatching review, the controller validates the evidence in-context: present, green,
  `head_sha` == branch HEAD. If not (a build-contract violation), it re-runs the gate once to
  mint fresh evidence rather than dispatching a review of an uncertified branch.
- Dispatch the reviewer (foreground, DIRECTED per the autonomy-precedence rule), then triage:
  - **blocker** → one synthetic fix task covering all blockers, run through the
    `docket-build-task` contract on the ladder `standard → premium → halt` (mirroring
    integration-repair; no new machinery). If its commits land, re-run the full suite once and
    refresh the evidence. A red re-run **halts** (abort-and-report, change stays `in-progress`
    with the reason recorded) — no second repair chain, no re-review.
  - **`unverified-build-state`** blocker (the reviewer's backstop finding) → resolved by the
    controller re-running the suite, never by a worker task.
  - **important / minor** → recorded in the PR body for the human's merge-time judgment; never
    auto-fixed.
  - **distinct follow-up work** → the existing auto-capture path, unchanged (classify, mint via
    `mint-stub`, carry the running `--minted` count).
- No re-review after fixes: remediation is verified by the worker's self-review plus the green
  suite re-run, and both findings and fixes are visible in the PR body.

## Component 5 — finalize's conditional suite skip

In `docket-finalize-change`'s `gate: local` path, immediately after the rebase step:

**Skip the post-rebase suite run only when ALL hold:**
1. the rebase was a **no-op** — the feature branch was already based on the current
   `origin/<integration_branch>` tip (tip is an ancestor of the branch HEAD; HEAD unchanged by
   the rebase), and
2. the PR body contains a parseable evidence block with `result: green`, and
3. the block's `head_sha` equals the branch HEAD being merged.

Anything else — missing/malformed block, SHA mismatch, an actual rebase — runs the suite
exactly as today. **The posture fails toward running:** any doubt costs ten minutes, never a
broken main. A skip is logged loudly in finalize's output (one line naming the matched SHA)
so the decision is auditable. `gate: ci`, `both`, and `off` are untouched.

Net effect: one suite run per change when review is clean and main has not moved; two
otherwise; never three.

## Finding schema

Each finding:

| field | content |
|---|---|
| `severity` | `blocker` · `important` · `minor` |
| `location` | `file:line` (or `file` for file-level findings) |
| `summary` | one sentence — the defect claim |
| `rationale` | why it is real: the failure scenario or violated contract |
| `suggested_fix` | optional, one sentence — a hint for the fix worker, never a patch |

Severity meanings: **blocker** = would ship a real defect (wrong behavior, broken contract,
data loss); **important** = should be addressed but survivable in an open PR (missing edge
coverage, fragile pattern); **minor** = style/polish. The reviewer returns the list plus a
one-line overall verdict (`clean` / `N findings: B blocker, I important, M minor`) and
nothing else — no prose report.

## Coupling and ripple surfaces

- **`depends_on: [167, 184]`.** 0167 is `done`. 0184 (four-tier ladder, PR #147) is
  `implemented` — this change edits the same roster surfaces, so it builds only after #147
  merges. Its reconcile pass re-verifies the pin table and wrapper count against merged main.
- Roster/count surfaces (the 0167/0184 ripple list): `.docket.example.yml` (`agents.claude`
  mirror + commented `codex:`/`cursor:` mirrors), README wrapper-count prose,
  `skills/docket-convention/SKILL.md` wrapper enumeration, `cursor-rules/dispatch/` fragments,
  and the tests asserting the roster, example-yml key count, and per-skill size budgets
  (`test_sync_agents*.sh`, `test_docket_example_yml.sh`, `test_skill_size_budgets.sh`,
  `test_dispatch_capability.sh`).
- Skill-prose surfaces: `docket-build` (gate emits the evidence line), `docket-implement-next`
  Step 6/7 (dispatch + triage + PR-body block), `docket-finalize-change` (skip predicate),
  `docket-convention` (role table row for review; wrapper count).
- **README.md**: document the suite-placement rationale (the section above) and the evidence
  chain.
- **ADR at build time**: docket owns the review role — one bounded read-only reviewer, the
  evidence chain, and the suite-once placement; relates to ADR-0063 (build-role twin).

## Settled during grooming (decision log)

1. Suite's implementation-phase home: **build gate only**; reviewer consumes evidence, never
   re-runs. (Daniel, 2026-08-01)
2. Reviewer pin: **opus-5 / medium** on Claude; other harnesses mirror the tier.
3. Findings: **severity-tiered routing** — blockers fixed pre-PR via the build-task contract,
   important/minor to the PR body, follow-ups auto-captured.
4. Finalize skip: **folded into 0170** (not a separate stub) — the evidence contract and its
   consumer ship together.
5. Foreground dispatch: settled on data-flow + ADR-0024 grounds (see Component 1).
