<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0170 — Lean Docket-owned whole-branch review skill](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-01-0170-lean-whole-branch-review-skill.md)**
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

**Dispatch shape.** `docket-implement-next` Step 6 dispatches the selected rung's wrapper
agent once (rung selection: Component 2), **foreground** (the parent actively blocks). Foreground is forced by three facts: the findings
are load-bearing for the very next step (Step 7 cannot open the PR before triage); a read-only
agent has no git-state channel, so the in-context return is its only output path (the same
shape as finalize's rebase-resolver and integration-repair dispatches); and ADR-0024 rules out
background-and-yield (a forked child has no notification channel — the caller would read a
half-done run as `completed`). The cost is bounded: unlike the old default, this reviewer runs
no suite — the parent blocks for one model read of the diff, not ten minutes of bash.

**Foreground ≠ inline — the pin survives the dispatch.** Foreground/background is a
*scheduling* axis (does the parent block on the child's return); inline/dispatched is an
*execution* axis (parent's own context vs. a separate subagent). This reviewer is a
foreground **dispatch**: a fresh subagent spun up from the selected `docket-review-*` rung
wrapper, which carries the resolved `model:`/`effort:` pin — so the review runs at its rung's
pin regardless of the model `docket-implement-next` itself is running at, exactly as the
build workers and `docket-adr` run at their own pins today. The parent's model reaches the review only on the
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

## Component 2 — reviewer rungs + pins

Three generated wrappers — `agents/docket-review-lean.md`, `-standard.md`, `-deep.md` —
joining the roster via the existing `agents/docket-*.md` glob (post-0184: thirteen wrappers
→ **sixteen**). Same shape, differing only in pin, mirroring how the build profiles share
`docket-build-task`.

**Rung selection — the "one above the build" rule.** The controller computes the build's
overall complexity as the **highest profile any task routed to or escalated to** — read
deterministically from `docket-build`'s stable output lines (task-to-profile selection;
escalation), with an escalation counting as the tier escalated *to*. Mapping:

| build's highest profile | reviewer rung |
|---|---|
| economy | lean |
| standard | standard |
| premium or max | deep (the cap merges the top two) |

No fourth rung ships: under this mapping a reviewer-economy rung is unreachable — it would
be a dead wrapper maintained forever. One optional modifier: a whole-branch diff above a
size threshold (set at build time) bumps the rung by one — the single selection signal
independent of the build's self-assessment, bounding the cost of a rubric misjudgment.
Selection is a deterministic rule over the build record, never model judgment, and the
chosen rung + reason is one line in the run output.

**Pins — the rungs get their own table, not the build's shifted one.** A literal +1 on the
build pins would price the common case (standard build) at premium's pin everywhere; instead
the reviewer ladder prices review work directly, reusing build-ladder pairs where they fit.
Settled 2026-08-01 against the merged post-0184 `agents/harness-defaults.yml`:

| rung | claude | cursor | codex |
|---|---|---|---|
| review-lean | `claude-sonnet-5` / high | `cursor-grok-4.5-medium` / auto | `gpt-5.6-terra` / medium |
| review-standard | `claude-opus-5` / medium | `cursor-grok-4.5-high` / auto | `gpt-5.6-terra` / high |
| review-deep | `claude-opus-5` / high | `claude-opus-5-high` / auto | `gpt-5.6-sol` / medium |

Per-harness reasoning: **claude** — standard and deep sit exactly on build-premium's and
build-max's pins; lean deviates from a literal +1 because sonnet-5/high is cheaper than
opus-5/low and the build-economy comment's reason for avoiding smaller models (contract
fumbles halt the build) does not apply to a read-and-return reviewer. **cursor** — pure +1:
the three rungs take build-standard's, build-premium's, and build-max's IDs verbatim; effort
stays `auto` per the block's variant-encoded rule. **codex** — pairs are roles (the block's
own doctrine): lean takes build-standard's `terra/medium`; standard runs `terra/high`
(chosen over build-premium's `sol/low` — same model as lean at higher reasoning, keeping the
common-case review on the mid model); deep takes build-max's `sol/medium`. Invariant on
every harness: **review-deep equals the build-max pin** — the cap rung never reviews below
the strength the riskiest build work was built with. Reconcile re-verifies these rows against
`origin/main`'s table at build time.

- **Each wrapper wraps the review skill only — no `docket-convention` injection.** Precedent:
  the `docket-build-*` profile workers, which perform no docket metadata operations. The
  reviewer is read-only over the feature branch; it touches no docket metadata.
- All three carry the standard abort-and-report posture of autonomous wrappers.
- **No reviewer escalation ladder**: one shot at the selected rung. A reviewer that cannot
  complete aborts-and-reports; it never re-dispatches itself upward.

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
- Compute the reviewer rung per Component 2's rule (highest routed-or-escalated build
  profile, +1-style mapping, optional diff-size bump) and log the selection + reason as one
  line.
- Dispatch the selected rung's reviewer (foreground, DIRECTED per the autonomy-precedence
  rule), then triage:
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

- **`depends_on: [167, 184]`.** Both are `done` and merged — verified at reconcile
  (2026-08-01). The pin table above was re-verified row-by-row against merged
  `origin/main:agents/harness-defaults.yml`; every row holds, including the cross-harness
  invariant **review-deep == build-max** (claude `claude-opus-5`/high, cursor
  `claude-opus-5-high`, codex `gpt-5.6-sol`/medium). Wrapper count on merged main is
  **thirteen**, so the target is **sixteen** as designed.
- Skill-prose surfaces: `docket-build` (gate emits the evidence line), `docket-implement-next`
  Step 6/7 (dispatch + triage + PR-body block), `docket-finalize-change` (skip predicate),
  `docket-convention` (role table row for review; wrapper count).
- **README.md**: document the suite-placement rationale (the section above) and the evidence
  chain.
- **ADR at build time**: docket owns the review role — one bounded read-only reviewer, the
  evidence chain, and the suite-once placement; relates to ADR-0063 (build-role twin).

### Reconciled ripple list (2026-08-01 — verified against merged main)

No generator code changes: `sync-agents.sh` (repo **root**, not `scripts/`) and
`scripts/lib/harness-defaults.sh` are entirely glob-driven over `agents/docket-*.md`, and
`link-skills.sh` globs `skills/*/`. Short names derive as `${basename#docket-}` minus `.md`,
so the sidecar keys are `review-lean` / `review-standard` / `review-deep`. New/edited surfaces:

| Surface | Edit |
|---|---|
| `agents/docket-review-{lean,standard,deep}.md` | new — `skills: [docket-review]`, no convention injection |
| `agents/harness-defaults.yml` | **+9 rows** (3 agents × claude/cursor/codex) |
| `cursor-rules/dispatch/docket-review-*.md` | new — 3 fragments |
| `skills/docket-review/SKILL.md` | new |
| `skills/docket-build/SKILL.md` | evidence line in the gate/output |
| `skills/docket-implement-next/SKILL.md` | Step 6 rung selection + evidence + triage; Step 7 PR block |
| `skills/docket-finalize-change/SKILL.md` | conditional post-rebase skip in the gate flow's step 4 |
| `skills/docket-convention/SKILL.md` | wrapper count/enumeration + Skill-layer review row + dispatch tier rows |
| `.docket.example.yml` | "thirteen agents" prose + 3 commented mirror blocks (+3 rows each) |
| `.docket.yml` | `skills: review: docket-review` (this repo's dogfood opt-in) |
| `README.md` | two count sentences, `## Skills` catalog row, new `### docket-review` section |

**Hard gates the groom did not name — each fails the suite if missed:**

1. **`hd_validate` completeness.** Every shipped harness block must carry an entry for every
   `agents/docket-*.md`. Three wrappers without all nine sidecar rows fail generation *before
   any wrapper is written*.
2. **`test_dispatch_capability.sh` reverse correspondence** (the sharpest constraint). It
   greps all `skills/**/*.md` for `` `<name>` …subagent `` and `resolved (build|review) skill`,
   and its site-coverage assert is an **exact** `-eq 5`. Naming the three rung wrappers as
   dispatch targets in skill prose adds three sites: they need `check_site` rows, matching
   tier rows in the convention's tier table, and the floor raised to 8. `PENDING_TIER` is
   explicitly forbidden as a parking spot.
3. **`test_finalize_gate.sh`** asserts the convention says "thirteen" and not "twelve"
   (lines 140-141) → becomes sixteen/thirteen; and asserts the exact phrase
   **"six skills get a wrapper"** (line 144) → becomes seven, since `docket-review` is a
   wrapper-bearing skill. The same file forbids `opus|sonnet|haiku|fable` and `xhigh`
   literals in finalize's own prose.
4. **`test_docket_build.sh`** bans the bare words `low` / `medium` / `high` from every
   `agents/docket-*.md` and `cursor-rules/dispatch/docket-*.md`. The review wrappers and
   fragments must describe their rung without those tokens.
5. **`test_skill_size_budgets.sh`** auto-discovers `skills/**/*.md` and fails on any file
   with no budget row: one **new row** for `skills/docket-review/SKILL.md` plus **four
   raises**, because the files to edit sit at 3-4 lines of headroom
   (`docket-build` 247/250, `docket-convention` 361/365 and ~33 words,
   `docket-finalize-change` 189/193, `docket-implement-next` 135/147 and ~55 words).
   Budget rounding follows that file's own stated convention.
6. **Roster counts**: `test_sync_agents.sh` (13 → 16, twice), `test_sync_agents_cursor.sh`,
   `test_sync_agents_codex.sh` (one equality + two `-ge` floors),
   `test_cursor_dispatch_rule.sh` fragment floor, and `test_readme_skill_catalog.sh`'s
   forward/reverse check between `skills/*/` and the README catalog table.

**Two roster hand-lists to settle deliberately** (neither reddens on its own):
`test_sync_agents.sh`'s `AUTONOMOUS` list (scopes the per-agent convention-injection assert)
and `test_skill_fork_dispatch.sh`'s `FORKED`/`EXCLUDED` lists — `docket-review` belongs in
neither today, so the build must place it consciously rather than by omission.

**Pre-existing prose defect to fix in passing:** `skills/docket-convention/SKILL.md`'s
convention-injection sentence reads "every wrapper except **four**" while naming five
(`docket-brainstorm-consultant` + the four `docket-build-*` workers) — an off-by-one left by
0184's fourth profile. This change edits that exact sentence, so it is corrected here rather
than captured separately; with the three review wrappers it becomes eight.

## Settled during grooming (decision log)

1. Suite's implementation-phase home: **build gate only**; reviewer consumes evidence, never
   re-runs. (Daniel, 2026-08-01)
2. Reviewer pin: initially a single **opus-5 / medium** reviewer; revised same day to the
   three-rung ladder (decision 6) with opus-5/medium kept as the common-case (standard-rung)
   pin.
3. Findings: **severity-tiered routing** — blockers fixed pre-PR via the build-task contract,
   important/minor to the PR body, follow-ups auto-captured.
4. Finalize skip: **folded into 0170** (not a separate stub) — the evidence contract and its
   consumer ship together.
5. Foreground dispatch: settled on data-flow + ADR-0024 grounds (see Component 1).
6. Tiered review (Daniel's "one above the build" method, 2026-08-01): three reviewer rungs
   selected deterministically from the build's highest routed-or-escalated profile, with the
   rungs' own pin table and an optional diff-size bump. Four mirrored tiers rejected — the
   bottom rung is unreachable under the +1 mapping (a dead wrapper). Evidence block stays in
   the PR body, not a temp file — finalize is a cross-session/cross-machine consumer, and
   `.superpowers/` checkpoint files are gitignored, opt-in, and transient.
