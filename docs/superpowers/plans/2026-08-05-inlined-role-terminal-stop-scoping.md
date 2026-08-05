<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0212 — An inlined role skill's terminal stop ends the whole run — scope docket-build's stop and enforce the run disposition](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0212-an-inlined-role-skill-s-terminal-stop-ends-the-whole-run-sco.md)**
<!-- docket:backlink:end -->

# Inlined Role Terminal Stop Scoping Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Scope every docket-owned skill body's terminal stop and second-person prohibitions to the role rather than to a caller that loaded the body inline, and bind `docket-implement-next`'s run to declaring one of its four terminal dispositions.

**Architecture:** Two prose levers plus two guards. Lever 1 edits four skill bodies (`docket-build`, `docket-review`, `docket-status`, `docket-build-task`) to add one canonical, mode-conditioned **scoping clause** beside each terminal stop or second-person prohibition, and records a no-hazard verdict for the two clean bodies (`docket-adr`, `docket-brainstorm`). A new positive-presence, **proximity-scoped** guard (`tests/test_inline_role_stop_scoping.sh`) asserts the clause sits *at* each hazard site, not merely somewhere in the file. Lever 2 adds a closing obligation to `docket-implement-next`'s *Terminal disposition* section and extends `tests/test_loop_continuation.sh` to assert it. Every touched skill file is under a live line/word budget in `tests/test_skill_size_budgets.sh`; the sequence is compress → re-measure → raise only what still exceeds.

**Tech Stack:** Bash 3.2-compatible shell test scripts (`tests/test_*.sh`, run by `for test in tests/test_*.sh; do "$DOCKET_BASH_PATH" "$test"; done`), Markdown skill bodies under `skills/`.

## Global Constraints

Copied verbatim from the spec and from `AGENTS.md`; every task's requirements implicitly include this section.

- **The scoping clause is two-sided and conditioned on invocation mode.** A generated wrapper injects the *same* skill body, so a one-sided "the caller continues" would be read by dispatched subagents whose turn genuinely does end. Both halves appear in every instance.
- **`docket-brainstorm` Step 3 is the house pattern** — it stops, then names the owner of the next step. Model the clause on it.
- **Vendored skills are out of scope.** Never edit `superpowers:writing-plans`, `superpowers:finishing-a-development-branch`, or anything else docket does not own.
- **The guard is positive-presence, never a negative vocabulary grep.** A negative grep forbidding unqualified "you stop" escapes by paraphrase; the existing `tests/test_role_skill_self_description.sh` header documents exactly that limitation of its own negative form.
- **Presence of the clause anywhere in a file is not presence of it at the stop** (0199's co-occurrence lesson). The guard must be proximity- or site-scoped.
- **Non-vacuity is mandatory in every guard**: the file the guard reads exists and is non-empty; a live PRESENCE assert runs through the same read; and a mutation-in-fixture probe proves the matcher fires. Confirm every mutation actually landed with a `grep -c` before and after — an in-place substitution that silently fails to match yields a green run with nothing mutated.
- **Budgets:** `tests/test_skill_size_budgets.sh` rows are `<relpath> <maxLines> <maxWords>`. Compress the touched section first, re-measure, then raise only rows the post-compression actual exceeds. A raise is set from the *measured* actual: lines → next multiple of 5, words → next multiple of 50; **if that lands within 25 words of the actual, take the multiple after it.**
- **A raise must be justified in-diff** (change 0201): name the `references/` file the prose was considered for and state why it cannot live there. `skills/docket-build/` and `skills/docket-review/` have **no `references/` tree at all** — for those, name the home that would have to be *created* and argue why creating it is wrong.
- **Behaviour-neutral slimming of unrelated prose is rejected** as the primary lever; it couples a fix to a refactor.
- **Shell rules (`AGENTS.md`):** never `producer | early-exiting-consumer` (`grep -q`, `head`) under `set -o pipefail` — capture into a variable first. A `grep` pattern leading with `--` must use `-e` or `-F --`. awk indent classes are `[^[:space:]]`, never `[^ ]`.
- **Cross-reference rule (`AGENTS.md`, ADR-0054):** a cross-reference in maintained source anchors on a symbol name or a **verbatim-quoted clause**, never on a line number.
- **Never touch docket metadata** from the feature worktree: no `.docket/`, no change file, no `BOARD.md`, no ADR, no learnings ledger.

## File Structure

| File | Responsibility |
|---|---|
| `skills/docket-build/SKILL.md` (modify) | Scope the line-11 terminal stop; label `## Output`'s disposition and `## Halting conditions`' `halted` as **role-scoped**, not the run disposition of the same name. |
| `skills/docket-review/SKILL.md` (modify) | Scope the H1/`## Halting` terminal stops and the `## Conduct` second-person prohibitions (never writes / never commits / never checks out / never dispatches / never runs the suite). |
| `skills/docket-status/SKILL.md` (modify) | Scope the `## Run the orchestrator` hard-error stop ("surface the stderr diagnostic and stop rather than improvising a fix"), which is inlined at `docket-implement-next` Step 0 under Tier A. |
| `skills/docket-build-task/SKILL.md` (modify) | Scope the `## Outcomes` terminal return and the `## Scope` second-person prohibitions; this body reaches a caller's context by wrapper preload (`agents/docket-build-*.md` carry `skills: [docket-build-task]`). |
| `skills/docket-implement-next/SKILL.md` (modify) | Lever 2: the *Terminal disposition* section gains a closing obligation on the agent. |
| `tests/test_inline_role_stop_scoping.sh` (create) | Lever 1's guard: per-file, per-site proximity-scoped positive presence, plus the recorded no-hazard verdicts for `docket-adr` and `docket-brainstorm`. |
| `tests/test_loop_continuation.sh` (modify) | Lever 2's guard: the closing obligation and the wrong-vocabulary rule. |
| `tests/test_skill_size_budgets.sh` (modify) | Budget rows re-derived from post-compression actuals, with in-diff justification comments. |

## The canonical scoping clause

Every instance uses one of exactly two shapes so a single guard matcher covers them all. Both carry the two literal anchors `loaded inline into a caller's context` and `dispatched as a subagent` — the guard keys on those two literals co-occurring within the site window.

**Shape A — terminal stop:**

```markdown
**Scope of this stop:** loaded inline into a caller's context, this stop ends this role only and
that caller continues to its own next step; dispatched as a subagent, your turn ends here.
```

**Shape B — second-person prohibitions:**

```markdown
**Scope of these prohibitions:** they bind this role's own conduct. When this body is
loaded inline into a caller's context they do not bind that caller, whose writes, commits, and
dispatches remain its own; dispatched as a subagent, they bind you for the whole turn.
```

Adapt only the leading bolded label and the role noun. Never drop either literal anchor, and never reduce either shape to one side.

**Both anchors must appear in lowercase, exactly as written above.** The guard matches them as case-sensitive fixed strings, so a sentence-initial `Loaded inline …` would not match. Shape B's `When this body is` prefix exists solely to keep the anchor mid-sentence; do not "tidy" it away.

---

### Task 1: Scope docket-build's stop and label its disposition role-scoped

**Files:**
- Modify: `skills/docket-build/SKILL.md` — the H1 paragraph (verbatim clause `Then you stop — review is not yours.`), `## Halting conditions` (verbatim clause `Every halt is the same disposition: stop, return`), `## Output` (verbatim clause `the terminal build disposition`)
- Modify: `tests/test_skill_size_budgets.sh` — the `skills/docket-build/SKILL.md` row, only if the post-edit actual exceeds `265 2400`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: the two literal anchors `loaded inline into a caller's context` and `dispatched as a subagent` present in `skills/docket-build/SKILL.md` within 6 lines of the clause `Then you stop`; the phrase `role-scoped` present in `## Output`. Task 5's guard table keys on exactly these.

**Context the implementer needs.** `docket-build` is invoked **inline via the Skill tool** by `docket-implement-next` Step 5. Its line-11 sentence *"Then you stop — review is not yours."* is the exact sentence that ended the 0206 run at the Step 5/6 boundary — the driver read "you" as itself. Separately, `## Halting conditions` says every halt returns `halted`, which is *also* the spelling of one of `docket-implement-next`'s four **run** dispositions; an inlined reader can mistake a build halt for a run halt. Labelling the build disposition role-scoped is what keeps the two vocabularies apart.

- [ ] **Step 1: Write the failing test**

Create nothing yet — Task 5 owns the guard. For this task the failing check is the budget test plus a direct presence check. Run first, to see the current state:

```bash
cd /Users/homer/dev/docket/.worktrees/an-inlined-role-skill-s-terminal-stop-ends-the-whole-run-sco
grep -c "loaded inline into a caller's context" skills/docket-build/SKILL.md
```

Expected: `0`.

- [ ] **Step 2: Measure the file before editing**

```bash
wc -l skills/docket-build/SKILL.md; wc -w skills/docket-build/SKILL.md
```

Expected: `262` lines, `2369` words (budget `265 2400` — **3 lines, 31 words** of margin). The clause will not fit; compression comes first.

- [ ] **Step 3: Compress the H1 paragraph and add Shape A**

The current paragraph reads:

```markdown
docket's build role, bound by `skills.build`. You are already running inside
`docket-implement-next` Step 5 with the plan written and the feature worktree cut. You read the
plan, route each task to a profile, dispatch one fresh worker per task, apply the escalation
protocol, and run the build gate. Then you stop — review is not yours.
```

Replace it with:

```markdown
docket's build role, bound by `skills.build`. You run inside `docket-implement-next` Step 5 with
the plan written and the worktree cut: read the plan, route each task to a profile, dispatch one
fresh worker per task, apply the escalation protocol, run the build gate. Then you stop — review
is not yours.

**Scope of this stop:** loaded inline into a caller's context, this stop ends the build role only
and that caller continues to its own next step; dispatched as a subagent, your turn ends here.
```

- [ ] **Step 4: Label the halt disposition role-scoped**

In `## Halting conditions`, the opening sentence currently reads:

```markdown
Every halt is the same disposition: stop, return `halted` — the change stays `in-progress` and the
worktree is preserved for inspection or resume — and report which condition below fired with its
concrete evidence (task, profile, SHA, command, or harness message).
```

Change the opening clause to name the scope:

```markdown
Every halt is the same **role-scoped** build disposition: stop, return `halted` — a build outcome,
not `docket-implement-next`'s run disposition of the same name — the change stays `in-progress`
and the worktree is preserved for inspection or resume — and report which condition below fired
with its concrete evidence (task, profile, SHA, command, or harness message).
```

- [ ] **Step 5: Label the Output disposition role-scoped**

In `## Output`, replace the list item `the terminal build disposition` with:

```markdown
the terminal build disposition (**role-scoped** — a build disposition, never a run disposition)
```

- [ ] **Step 6: Re-measure and derive the budget row**

```bash
wc -l skills/docket-build/SKILL.md; wc -w skills/docket-build/SKILL.md
```

If lines ≤ 265 **and** words ≤ 2400, make no budget edit and skip to Step 8. Otherwise derive the new row from the measured actual under the file's documented rounding rule: lines → the next multiple of 5 strictly above the actual; words → the next multiple of 50 strictly above the actual, and **if that lands within 25 words of the actual, take the multiple after it**.

- [ ] **Step 7: Raise the row with its in-diff justification**

Edit the `skills/docket-build/SKILL.md` row in `BUDGETS` to the derived numbers, and append a justification paragraph to the comment block above `BUDGETS`, immediately after the last existing entry, using this text with `<L>`/`<W>`/`<actualL>`/`<actualW>` replaced by the real numbers:

```bash
# skills/docket-build/SKILL.md's budget was raised 265/2400 -> <L>/<W> by change 0212, which added the
# mode-conditioned scoping clause beside the file's terminal stop. skills/docket-build/ has NO
# references/ tree, so change 0201's rule cannot be discharged by naming an existing file: the home
# that would have to be created is skills/docket-build/references/. Creating it is wrong here — the
# clause must fire at the exact moment a reader reads "Then you stop", and a rule sitting in an
# unread reference file cannot intervene at that moment. That is the same argument change 0137
# recorded for the convention's dispatch rule. The H1 paragraph was compressed first; set from the
# measured actual: <actualL> lines -> <L>, <actualW> words -> <W>.
```

- [ ] **Step 8: Verify**

```bash
grep -c "loaded inline into a caller's context" skills/docket-build/SKILL.md
grep -c "dispatched as a subagent" skills/docket-build/SKILL.md
"$DOCKET_BASH_PATH" tests/test_skill_size_budgets.sh
```

Expected: each `grep -c` prints `1`; the budget test prints `PASS`.

- [ ] **Step 9: Commit**

```bash
git add skills/docket-build/SKILL.md tests/test_skill_size_budgets.sh
git commit -m "fix(0212): scope docket-build's terminal stop and label its disposition role-scoped"
```

---

### Task 2: Scope docket-review's stops and its second-person prohibitions

**Files:**
- Modify: `skills/docket-review/SKILL.md` — `## Conduct` (append after its last bullet, verbatim clause `One shot at the dispatched rung`) and `## Halting` (verbatim clause `An unmet precondition or a blocking ambiguity is **abort-and-report**`)
- Modify: `tests/test_skill_size_budgets.sh` — the `skills/docket-review/SKILL.md` row, only if the post-edit actual exceeds `105 800`

**Interfaces:**
- Consumes: the two literal anchors defined in the *canonical scoping clause* section.
- Produces: both literal anchors present in `skills/docket-review/SKILL.md` within 6 lines of the clause `One shot at the dispatched rung` **and** within 6 lines of the clause `abort-and-report`. Task 5's guard requires two separate sites in this file.

**Context the implementer needs.** This is the **larger** half of lever 1, not the lighter one. `docket-review` is bound by `skills.review` and invoked at `docket-implement-next` Step 6. Its `## Conduct` prohibitions — never writes, never commits, never checks out, never dispatches, never runs the suite — inline-loaded into the driver's context would forbid the driver's own blocker-fix dispatch, its `docket-adr` dispatch, and Step 7's metadata writes. That is strictly worse than a stop sentence, because it silently disables work the driver *must* do. `## Scope` already says "the controller that dispatched you owns every write that follows from them", so partial scoping exists; the clause makes it explicit and two-sided. Current actual is 96 lines / 758 words against a `105 800` budget — 9 lines, 42 words of margin.

- [ ] **Step 1: Confirm the current state**

```bash
cd /Users/homer/dev/docket/.worktrees/an-inlined-role-skill-s-terminal-stop-ends-the-whole-run-sco
grep -c "loaded inline into a caller's context" skills/docket-review/SKILL.md
wc -l skills/docket-review/SKILL.md; wc -w skills/docket-review/SKILL.md
```

Expected: `0`, then `96` and `758`.

- [ ] **Step 2: Add Shape B to `## Conduct`**

Append this paragraph at the end of `## Conduct`, after the existing final bullet (the one beginning `- One shot at the dispatched rung.`):

```markdown
**Scope of these prohibitions:** they bind this review role's own conduct. When this body is
loaded inline into a caller's context they do not bind that caller, whose writes, commits, and
dispatches remain its own; dispatched as a subagent, they bind you for the whole turn.
```

- [ ] **Step 3: Add Shape A to `## Halting`**

Append this paragraph at the end of `## Halting`, after the existing final sentence:

```markdown
**Scope of this stop:** loaded inline into a caller's context, this stop ends the review role only
and that caller continues to its own next step; dispatched as a subagent, your turn ends here.
```

- [ ] **Step 4: Re-measure and raise only if needed**

```bash
wc -l skills/docket-review/SKILL.md; wc -w skills/docket-review/SKILL.md
```

If lines ≤ 105 and words ≤ 800, make no budget edit and skip to Step 6. Otherwise derive the row exactly as in Task 1 Step 6 (lines → next multiple of 5 above the actual; words → next multiple of 50 above the actual, pushed one multiple further if within 25 words).

- [ ] **Step 5: Raise the row with its in-diff justification**

```bash
# skills/docket-review/SKILL.md's budget was raised 105/800 -> <L>/<W> by change 0212, which scoped
# the file's ## Conduct prohibitions and its ## Halting stop to the review role. skills/docket-review/
# has NO references/ tree; the home that would have to be created is
# skills/docket-review/references/, and creating it is wrong for the same reason as docket-build's
# entry above — a prohibition-scoping rule must be read in the same breath as the prohibition it
# scopes. Set from the measured actual: <actualL> lines -> <L>, <actualW> words -> <W>.
```

- [ ] **Step 6: Verify**

```bash
grep -c "loaded inline into a caller's context" skills/docket-review/SKILL.md
"$DOCKET_BASH_PATH" tests/test_skill_size_budgets.sh
"$DOCKET_BASH_PATH" tests/test_role_skill_self_description.sh
```

Expected: `2` (one per site); both tests print `PASS`. The self-description test must stay green — the added clause contains no `superpowers:` reference, so it cannot trip that guard.

- [ ] **Step 7: Commit**

```bash
git add skills/docket-review/SKILL.md tests/test_skill_size_budgets.sh
git commit -m "fix(0212): scope docket-review's prohibitions and halting stop to the review role"
```

---

### Task 3: Settle the docket-status and docket-build-task verdicts

**Files:**
- Modify: `skills/docket-status/SKILL.md` — `## Run the orchestrator` (verbatim clause `surface the stderr diagnostic and stop rather than improvising a fix`)
- Modify: `skills/docket-build-task/SKILL.md` — `## Scope` (append after its last bullet, verbatim clause `If you were dispatched as an **escalated** worker`) and `## Outcomes` (verbatim clause `Return exactly one of three outcomes`)
- Modify: `tests/test_skill_size_budgets.sh` — the `skills/docket-build-task/SKILL.md` row, only if the post-edit actual exceeds `115 1000`

**Interfaces:**
- Consumes: the two literal anchors.
- Produces: both anchors within 6 lines of `stop rather than improvising a fix` in `skills/docket-status/SKILL.md`; both anchors within 6 lines of `Return exactly one of three outcomes` **and** within 6 lines of `If you were dispatched as an **escalated** worker` in `skills/docket-build-task/SKILL.md`.

**Context the implementer needs.** The spec left both verdicts open, to be settled at build time. Settle both as **edit**, for stated reasons:

- `docket-status` is dispatched as a subagent at `docket-implement-next` Step 0 *and* run **inline** under the convention's Tier A when dispatch is unavailable — a first-class equivalent path, not a degradation. Its hard-error stop is therefore loadable into the driver's context with no scoping at all. Budget margin is generous (96/2260 against `118 2393` — 22 lines, 133 words).
- `docket-build-task` reaches a caller's context by **wrapper preload**: `agents/docket-build-economy.md` and its three siblings carry `skills: [docket-build-task]`. It carries both hazard classes — `## Scope`'s never-write/never-push prohibitions and `## Outcomes`' terminal three-outcome return. It has the tightest margin in the set (111/959 against `115 1000` — 4 lines, 41 words), so compress before adding.

- [ ] **Step 1: Confirm the current state**

```bash
cd /Users/homer/dev/docket/.worktrees/an-inlined-role-skill-s-terminal-stop-ends-the-whole-run-sco
grep -c "loaded inline into a caller's context" skills/docket-status/SKILL.md skills/docket-build-task/SKILL.md
wc -l skills/docket-status/SKILL.md skills/docket-build-task/SKILL.md
wc -w skills/docket-status/SKILL.md skills/docket-build-task/SKILL.md
```

Expected: both counts `0`; `96` and `111` lines; `2260` and `959` words.

- [ ] **Step 2: Add Shape A to docket-status**

Immediately after the paragraph in `## Run the orchestrator` that ends with `has ALSO failed at that step.`, insert:

```markdown
**Scope of this stop:** loaded inline into a caller's context — the convention's Tier A path — this
stop ends the status role only and that caller continues to its own next step; dispatched as a
subagent, your turn ends here.
```

- [ ] **Step 3: Add Shape B to docket-build-task's `## Scope`**

Append at the end of `## Scope`, after the existing final bullet (the one beginning `- If you were dispatched as an **escalated** worker`):

```markdown
**Scope of these prohibitions:** they bind this worker's own conduct. When this body is
loaded inline into a caller's context they do not bind that caller, whose writes, commits, and
dispatches remain its own; dispatched as a subagent, they bind you for the whole turn.
```

- [ ] **Step 4: Add Shape A to docket-build-task's `## Outcomes`**

The section currently opens:

```markdown
Return exactly one of three outcomes. A missing or malformed outcome halts the build, so state it
plainly.
```

Replace that opening with:

```markdown
Return exactly one of three outcomes. A missing or malformed outcome halts the build, so state it
plainly.

**Scope of this return:** loaded inline into a caller's context, returning ends the worker role only
and that caller continues to its own next step; dispatched as a subagent, your turn ends here.
```

- [ ] **Step 5: Re-measure both files; compress docket-build-task if it exceeds**

```bash
wc -l skills/docket-build-task/SKILL.md; wc -w skills/docket-build-task/SKILL.md
wc -l skills/docket-status/SKILL.md; wc -w skills/docket-status/SKILL.md
```

`docket-status` has 133 words of headroom and will fit; make no budget edit for it. For `docket-build-task`, first compress `## Scope`'s third bullet, which currently reads:

```markdown
- Stay **inside the feature worktree, on its branch**, performing **no docket metadata operations**:
  never write to `.docket/`, the metadata branch, change files, ADRs, the board, or the
  learnings ledger; never push, force-push, `reset --hard`, or rebase. The controller and
  `docket-implement-next` own all of that.
```

to:

```markdown
- Stay **inside the feature worktree, on its branch**, performing **no docket metadata operations**:
  never write to `.docket/`, the metadata branch, change files, ADRs, the board, or the learnings
  ledger; never push, force-push, `reset --hard`, or rebase. The controller owns all of that.
```

Then re-measure. If the actual still exceeds `115 1000`, derive and raise the row exactly as in Task 1 Step 6, with this justification comment:

```bash
# skills/docket-build-task/SKILL.md's budget was raised 115/1000 -> <L>/<W> by change 0212, which
# scoped the worker contract's ## Scope prohibitions and its ## Outcomes return to the worker role.
# The body reaches a caller's context by wrapper preload (agents/docket-build-*.md carry
# skills: [docket-build-task]), so the hazard is real for this file. skills/docket-build-task/ has NO
# references/ tree; a created one could not intervene at the moment the return instruction is read.
# ## Scope's worktree bullet was compressed first; set from the measured actual: <actualL> -> <L>,
# <actualW> -> <W>.
```

- [ ] **Step 6: Verify**

```bash
grep -c "loaded inline into a caller's context" skills/docket-status/SKILL.md
grep -c "loaded inline into a caller's context" skills/docket-build-task/SKILL.md
"$DOCKET_BASH_PATH" tests/test_skill_size_budgets.sh
```

Expected: `1`, then `2`, then `PASS`.

- [ ] **Step 7: Commit**

```bash
git add skills/docket-status/SKILL.md skills/docket-build-task/SKILL.md tests/test_skill_size_budgets.sh
git commit -m "fix(0212): scope docket-status's and docket-build-task's stops and prohibitions"
```

---

### Task 4: Bind the run disposition in docket-implement-next

**Files:**
- Modify: `skills/docket-implement-next/SKILL.md` — `### Terminal disposition (driver contract)` (verbatim clause `The final report **enumerates** what happened`)
- Modify: `tests/test_loop_continuation.sh` — add asserts after the existing `--- SKILL.md: the four-disposition terminal contract ---` block
- Modify: `tests/test_skill_size_budgets.sh` — the `skills/docket-implement-next/SKILL.md` row, only if the post-edit actual exceeds `145 3800`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: the literal phrase `the run does not end until` and the literal phrase `is by construction an aborted run` in `skills/docket-implement-next/SKILL.md`. Task 5 does not read these; only `tests/test_loop_continuation.sh` does.

**Context the implementer needs.** The section today is *guidance to a driver* — it says what a driver does with each disposition, and nothing binds the running agent to declare one. The 0206 run closed with a **build** disposition ("Build disposition: complete — the plan is executed"), and that wrong vocabulary is by itself proof the run never reached its terminal step; it would have caught all four observed incidents (0109, 0194 twice, 0206). Two hard boundaries: state that `advanced` is claimable only when **Step 7's postcondition holds** **by pointing at Step 7** — never define Step 7's postcondition here, which is change 0203's surface; and add **no new runtime mechanism, status, or field** — the final report is model output, and the deterministic backstop is the `aborted-run` ledger. Current actual is 139 lines / 3728 words against `145 3800` — 6 lines, 72 words.

- [ ] **Step 1: Write the failing test**

Add this block to `tests/test_loop_continuation.sh`, immediately after the existing line `assert "SKILL enumerates skipped-with-reason" 'grep -Eqi "skipped with (its|the) reason" "$IMPL"'`:

```bash
# --- SKILL.md: the closing obligation on the AGENT (change 0212) ---
# The pre-0212 section was guidance to a DRIVER only; nothing bound the running agent to declare a
# disposition, and the 0206 run closed with a step-scoped "build disposition" instead. These asserts
# pin the obligation itself, not the table it sits under.
assert "SKILL obliges the agent to declare before the run ends" \
  'grep -qF -- "the run does not end until" "$IMPL"'
assert "SKILL names a non-conforming vocabulary an aborted run" \
  'grep -qF -- "is by construction an aborted run" "$IMPL"'
# advanced is gated on Step 7 BY POINTER — 0212 must not define Step 7's postcondition (that is
# change 0203's surface), so assert the pointer and not any postcondition wording.
assert "SKILL gates advanced on Step 7's postcondition by pointer" \
  'grep -Eqi "advanced.{0,80}Step 7|Step 7.{0,80}advanced" "$IMPL"'
```

- [ ] **Step 2: Run it to verify it fails**

```bash
cd /Users/homer/dev/docket/.worktrees/an-inlined-role-skill-s-terminal-stop-ends-the-whole-run-sco
"$DOCKET_BASH_PATH" tests/test_loop_continuation.sh
```

Expected: FAIL, with `NOT OK - SKILL obliges the agent to declare before the run ends` and the two asserts after it.

- [ ] **Step 3: Add the closing obligation**

In `### Terminal disposition (driver contract)`, insert this paragraph immediately **before** the existing final paragraph (`The final report **enumerates** what happened: …`):

```markdown
**The obligation is on the agent, not only the driver.** The run does not end until exactly one of
the four is declared: a final report that declares a step-scoped or invented disposition — a build
disposition, a review outcome, "complete" — is by construction an aborted run, whatever else it
reports. `advanced` is claimable only when **Step 7's postcondition** holds; that postcondition is
Step 7's to state, not this section's.
```

- [ ] **Step 4: Run the test to verify it passes**

```bash
"$DOCKET_BASH_PATH" tests/test_loop_continuation.sh
```

Expected: `PASS`.

- [ ] **Step 5: Prove the new asserts are non-vacuous by mutation**

```bash
cp skills/docket-implement-next/SKILL.md /tmp/0212-impl-backup.md
grep -c "the run does not end until" skills/docket-implement-next/SKILL.md
perl -0pi -e 's/The run does not end until/The run may end before/' skills/docket-implement-next/SKILL.md
grep -c "the run does not end until" skills/docket-implement-next/SKILL.md
"$DOCKET_BASH_PATH" tests/test_loop_continuation.sh
cp /tmp/0212-impl-backup.md skills/docket-implement-next/SKILL.md
grep -c "the run does not end until" skills/docket-implement-next/SKILL.md
rm -f /tmp/0212-impl-backup.md
```

Expected, in order: `1` (the mutation has a target), `0` (**the mutation landed** — if this prints `1` the substitution silently failed and the run below proves nothing), `FAIL` with `NOT OK - SKILL obliges the agent to declare before the run ends`, then `1` again after restore.

- [ ] **Step 6: Re-measure and raise only if needed**

```bash
wc -l skills/docket-implement-next/SKILL.md; wc -w skills/docket-implement-next/SKILL.md
```

If lines ≤ 145 and words ≤ 3800, make no budget edit and skip to Step 8. Otherwise derive the row as in Task 1 Step 6.

- [ ] **Step 7: Raise the row with its in-diff justification**

```bash
# skills/docket-implement-next/SKILL.md's budget was raised 145/3800 -> <L>/<W> by change 0212, which
# turned the *Terminal disposition* section from driver guidance into an obligation on the running
# agent. skills/docket-implement-next/references/edge-paths.md was the considered home and is wrong
# for it: edge-paths.md is read conditionally, at named edges (kill, resume, PR-body assembly),
# whereas this obligation must be in context on EVERY run at the moment the agent decides it is
# finished — the run that ends early never reaches a conditional read. Set from the measured actual:
# <actualL> lines -> <L>, <actualW> words -> <W>.
```

- [ ] **Step 8: Verify**

```bash
"$DOCKET_BASH_PATH" tests/test_loop_continuation.sh
"$DOCKET_BASH_PATH" tests/test_skill_size_budgets.sh
```

Expected: both `PASS`.

- [ ] **Step 9: Commit**

```bash
git add skills/docket-implement-next/SKILL.md tests/test_loop_continuation.sh tests/test_skill_size_budgets.sh
git commit -m "fix(0212): bind the run disposition as a closing obligation on the agent"
```

---

### Task 5: The proximity-scoped scoping-clause guard

**Files:**
- Create: `tests/test_inline_role_stop_scoping.sh`

**Interfaces:**
- Consumes: the literal anchors `loaded inline into a caller's context` and `dispatched as a subagent`, placed by Tasks 1–3 at the sites named in each of their *Produces* blocks.
- Produces: nothing later tasks consume.

**Context the implementer needs.** This guard is modelled on `tests/test_role_skill_self_description.sh` but inverts its polarity: **positive presence**, because a negative vocabulary grep forbidding unqualified "you stop" escapes by paraphrase — that file's own header documents the limitation. Two obligations carry over from change 0199 and from the learnings ledger:

1. **The file list is hand-maintained**, so it must match the sweep set the build actually landed. Both no-hazard verdicts are recorded here too, each with a live assert — a verdict recorded as a bare comment is not a verdict a future edit can violate.
2. **Presence of the clause anywhere in the file is not presence of it at the stop.** The site window is what makes this a real guard; a whole-file grep would pass with the clause parked in the frontmatter.

And from `marker-scoped-guard-needs-a-population-floor`: a guard whose scope is selected by matching text silently degrades to guarding *nothing* when the selector stops matching. Each site therefore asserts its **anchor is found** before asserting the clause is near it — a reworded stop reddens the guard and forces the table to be updated rather than passing vacuously.

- [ ] **Step 1: Write the guard**

Create `tests/test_inline_role_stop_scoping.sh`:

```bash
#!/usr/bin/env bash
# tests/test_inline_role_stop_scoping.sh — change 0212. A docket-owned skill body that can be LOADED
# INTO A CALLER'S CONTEXT must scope its terminal stops and its second-person prohibitions to the
# role, because "you" in an inlined body resolves to the caller. On 2026-08-05 a docket-implement-next
# run read docket-build's "Then you stop — review is not yours." as its own terminal boundary and
# ended at the Step 5/6 boundary with no review and no PR.
#
# POSITIVE-PRESENCE guard, deliberately not a negative vocabulary grep: a grep forbidding an
# unqualified "you stop" is line- and vocabulary-scoped and escapes by paraphrase (the header of
# tests/test_role_skill_self_description.sh documents that limitation of its own negative form).
#
# PROXIMITY-SCOPED: presence of the clause anywhere in the file is NOT presence of it at the stop
# (change 0199's co-occurrence lesson). Each site below is a (file, anchor) pair, and the clause must
# appear within WINDOW lines AFTER the anchor line.
#
# The SITES table is hand-maintained. If a swept body rewords a stop, its anchor stops matching and
# the existence assert reddens — deliberately, so the table is updated rather than guarding nothing.
# Run: bash tests/test_inline_role_stop_scoping.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if ( eval "$2" ); then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# Both halves of the two-sided, mode-conditioned clause. A wrapper injects the SAME body, so a
# one-sided "the caller continues" would be read by dispatched subagents whose turn genuinely ends.
INLINE_HALF="loaded inline into a caller's context"
DISPATCH_HALF="dispatched as a subagent"
WINDOW=6

# The line number of the first line matching a fixed-string anchor, or empty. Captured into a
# variable and sliced with parameter expansion — never `grep | head`, which under `set -o pipefail`
# SIGPIPEs the producer into an intermittent 141 (AGENTS.md).
anchor_line(){
  local hits first
  hits="$(grep -nF -- "$2" "$1" 2>/dev/null || true)"
  [ -n "$hits" ] || return 0
  first="${hits%%$'\n'*}"
  printf '%s' "${first%%:*}"
}

# Does the window starting at the anchor line carry BOTH halves of the clause?
clause_near(){
  local file="$1" line="$2" window text lo hi
  lo="$line"; hi=$(( line + WINDOW ))
  text="$(awk -v lo="$lo" -v hi="$hi" 'NR>=lo && NR<=hi' "$file")"
  case "$text" in *"$INLINE_HALF"*) ;; *) return 1 ;; esac
  case "$text" in *"$DISPATCH_HALF"*) ;; *) return 1 ;; esac
  return 0
}

# SITES: "<relpath>|<verbatim anchor clause>|<what the site is>". Verbatim-quoted clauses, never line
# numbers (AGENTS.md / ADR-0054). A PROHIBITION site anchors on the LAST bullet of its block, not the
# first, because the clause is appended after the block — anchoring on the first bullet would put the
# clause outside the window. No comment lines inside the heredoc: `read` would take one as a path.
SITES="
skills/docket-build/SKILL.md|Then you stop — review is not yours.|terminal stop
skills/docket-review/SKILL.md|One shot at the dispatched rung|second-person prohibitions
skills/docket-review/SKILL.md|An unmet precondition or a blocking ambiguity is **abort-and-report**|terminal stop
skills/docket-status/SKILL.md|stop rather than improvising a fix|hard-error stop (Tier A inline path)
skills/docket-build-task/SKILL.md|If you were dispatched as an **escalated** worker|second-person prohibitions
skills/docket-build-task/SKILL.md|Return exactly one of three outcomes|terminal return
"

while IFS='|' read -r rel anchor what; do
  [ -n "$rel" ] || continue
  f="$REPO/$rel"
  # Non-vacuity anchor #1: the file exists and is non-empty, or every assert below is meaningless.
  assert "swept body exists and is non-empty: $rel" '[ -s "$f" ]'
  [ -s "$f" ] || continue
  # Non-vacuity anchor #2 / population floor: the site anchor still matches. A reworded stop reddens
  # here instead of silently selecting an empty scope.
  ln="$(anchor_line "$f" "$anchor")"
  assert "$rel still carries its $what anchor" '[ -n "$ln" ]'
  [ -n "$ln" ] || continue
  # The property: the two-sided clause sits AT the site, not merely somewhere in the file.
  assert "$rel scopes its $what within $WINDOW lines" 'clause_near "$f" "$ln"'
done <<EOF
$SITES
EOF

# --- Recorded no-hazard verdicts (the sweep's deliverable is a per-file verdict, not an edit set) ---
# docket-adr: no terminal stop and no second-person prohibition; the body ends on a validation
# invocation. Asserted live rather than left as a comment, so a future edit that introduces a stop
# without scoping it reddens here.
ADR="$REPO/skills/docket-adr/SKILL.md"
assert "docket-adr exists and is non-empty" '[ -s "$ADR" ]'
assert "docket-adr still names itself (live presence, non-vacuity)" 'grep -qF -- "docket-adr" "$ADR"'
adr_stops="$(grep -icE "then you stop|your turn ends|never (writes|commits|dispatches)" "$ADR" || true)"
assert "docket-adr carries no unscoped stop or prohibition (found $adr_stops)" '[ "$adr_stops" -eq 0 ]'

# docket-brainstorm: its stop is the HOUSE PATTERN — it stops, then names the owner of the next step.
# Assert the naming half, which is the part that makes the stop safe.
BS="$REPO/skills/docket-brainstorm/SKILL.md"
assert "docket-brainstorm exists and is non-empty" '[ -s "$BS" ]'
bs_ln="$(anchor_line "$BS" "STOP AT THE SPEC")"
assert "docket-brainstorm still carries its stop anchor" '[ -n "$bs_ln" ]'
assert "docket-brainstorm's stop names the owner of the next step" \
  'grep -qF -- "owned by \`docket-implement-next\`" "$BS"'

if [ "$fail" != 0 ]; then
  echo "REMEDY: a docket-owned skill body loadable into a caller's context scopes its terminal stop"
  echo "        and its second-person prohibitions to the role, two-sided and conditioned on"
  echo "        invocation mode: \"$INLINE_HALF\" ... \"$DISPATCH_HALF\". Put the clause AT the site,"
  echo "        within $WINDOW lines of the anchor — not elsewhere in the file. If a stop was"
  echo "        reworded, update this file's SITES table in the same diff."
fi

# --- Non-vacuity anchor #3 (mutation-in-fixture): the matcher must FIRE on an unscoped site. ---
# Without this, a typo in either half makes every assert above permanently green — the inversion
# mirrored-guard-enforces-its-own-property warns about.
probe="$(mktemp)"
printf '%s\n' 'Then you stop — review is not yours.' 'Some other paragraph entirely.' > "$probe"
pl="$(anchor_line "$probe" "Then you stop — review is not yours.")"
assert "probe anchor is found (got '$pl')" '[ "$pl" = "1" ]'
assert "the matcher REJECTS an unscoped stop" '! clause_near "$probe" "$pl"'
# And it must ACCEPT a properly scoped one.
printf '%s\n' 'Then you stop — review is not yours.' '' \
  "**Scope of this stop:** $INLINE_HALF, this stop ends this role only and that caller continues to its own next step; $DISPATCH_HALF, your turn ends here." > "$probe"
pl="$(anchor_line "$probe" "Then you stop — review is not yours.")"
assert "the matcher ACCEPTS a scoped stop" 'clause_near "$probe" "$pl"'
# One-sided clauses must NOT satisfy it: the wrapper injects the same body, so both halves are
# load-bearing.
printf '%s\n' 'Then you stop — review is not yours.' '' \
  "**Scope of this stop:** $INLINE_HALF, the caller continues." > "$probe"
pl="$(anchor_line "$probe" "Then you stop — review is not yours.")"
assert "the matcher REJECTS a one-sided clause" '! clause_near "$probe" "$pl"'
# Presence far from the site must NOT satisfy it (the 0199 co-occurrence lesson, proved).
{ printf '%s\n' 'Then you stop — review is not yours.'
  for i in 1 2 3 4 5 6 7 8; do printf 'filler line %s\n' "$i"; done
  printf '%s\n' "$INLINE_HALF and $DISPATCH_HALF"; } > "$probe"
pl="$(anchor_line "$probe" "Then you stop — review is not yours.")"
assert "the matcher REJECTS a clause outside the site window" '! clause_near "$probe" "$pl"'
rm -f "$probe"

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
```

- [ ] **Step 2: Run it and verify it passes against the landed edits**

```bash
cd /Users/homer/dev/docket/.worktrees/an-inlined-role-skill-s-terminal-stop-ends-the-whole-run-sco
chmod +x tests/test_inline_role_stop_scoping.sh
"$DOCKET_BASH_PATH" tests/test_inline_role_stop_scoping.sh
```

Expected: `PASS`, with one `ok - ... still carries its ... anchor` and one `ok - ... scopes its ... within 6 lines` per site (six sites), plus the no-hazard and probe asserts.

- [ ] **Step 3: Mutation-test the guard against a real file**

Strip one landed clause and confirm the guard reddens on that site specifically:

```bash
cp skills/docket-build/SKILL.md /tmp/0212-build-backup.md
grep -c "$(printf '%s' "loaded inline into a caller's context")" skills/docket-build/SKILL.md
perl -0pi -e "s/\*\*Scope of this stop:\*\*[^\n]*\n[^\n]*\n//" skills/docket-build/SKILL.md
grep -c "loaded inline into a caller's context" skills/docket-build/SKILL.md
"$DOCKET_BASH_PATH" tests/test_inline_role_stop_scoping.sh
cp /tmp/0212-build-backup.md skills/docket-build/SKILL.md
grep -c "loaded inline into a caller's context" skills/docket-build/SKILL.md
rm -f /tmp/0212-build-backup.md
```

Expected, in order: `1`, then `0` (**the mutation landed** — a `1` here means the substitution silently failed and the run proves nothing; fix the pattern and redo), then `FAIL` naming `skills/docket-build/SKILL.md scopes its terminal stop within 6 lines`, then `1` after restore.

- [ ] **Step 4: Confirm the restore is byte-clean**

```bash
git diff --stat skills/docket-build/SKILL.md
"$DOCKET_BASH_PATH" tests/test_inline_role_stop_scoping.sh
```

Expected: no diff against the committed Task 1 state (empty output), and `PASS`.

- [ ] **Step 5: Commit**

```bash
git add tests/test_inline_role_stop_scoping.sh
git commit -m "test(0212): guard the mode-conditioned scoping clause at each swept stop site"
```

---

### Task 6: Whole-suite gate and drift check

**Files:**
- Modify: any file the gate reddens (no edits expected)

**Interfaces:**
- Consumes: every artifact from Tasks 1–5.
- Produces: a green full suite.

**Context the implementer needs.** `AGENTS.md`: *"Run the whole suite at the build gate, never only the tests the spec enumerated."* Three suite files are the likely collateral: `tests/test_skill_size_budgets.sh` (rows edited across three tasks — the last writer must not have clobbered an earlier raise), `tests/test_role_skill_self_description.sh` (its negative matcher reads the same four bodies), and `tests/test_comment_anchor_style.sh` (the new guard's SITES table must carry verbatim clauses, never `file:line` forms).

- [ ] **Step 1: Run the whole suite**

```bash
cd /Users/homer/dev/docket/.worktrees/an-inlined-role-skill-s-terminal-stop-ends-the-whole-run-sco
for test in tests/test_*.sh; do
  echo "=== $test"
  "$DOCKET_BASH_PATH" "$test" || echo "REDFILE: $test"
done
```

Expected: every file ends `PASS`; no `REDFILE:` lines.

- [ ] **Step 2: Confirm all three budget raises survived**

```bash
git diff origin/main -- tests/test_skill_size_budgets.sh
```

Expected: every row raised in Tasks 1, 3, and 4 is present with its justification comment. If a raise is missing, an earlier task's edit was overwritten — restore it and re-run the suite.

- [ ] **Step 3: Confirm the sweep is complete and its verdicts are all recorded**

```bash
for f in docket-build docket-review docket-status docket-build-task; do
  printf '%-22s %s\n' "$f" "$(grep -c "loaded inline into a caller's context" skills/$f/SKILL.md)"
done
grep -c 'skills/docket-' tests/test_inline_role_stop_scoping.sh
```

Expected: `docket-build 1`, `docket-review 2`, `docket-status 1`, `docket-build-task 2` — six sites, matching the six rows of the guard's SITES table plus the two no-hazard file paths.

- [ ] **Step 4: Commit only if Step 1 required a fix**

```bash
git add -A
git commit -m "fix(0212): resolve full-suite fallout from the scoping sweep"
```

If the suite was green with no edits, skip this step — do not create an empty commit.

---

## Self-Review

**1. Spec coverage.**

| Spec requirement | Task |
|---|---|
| Lever 1 — scope the stop, sweep six files, per-file verdict | Tasks 1, 2, 3 (four edits) + Task 5 (two recorded no-hazard verdicts, asserted live) |
| Clause is two-sided and mode-conditioned | *Canonical scoping clause* section; enforced by Task 5's "REJECTS a one-sided clause" probe |
| `docket-brainstorm` treated as the house pattern | Task 5's `docket-brainstorm's stop names the owner of the next step` assert |
| Positive-presence, proximity-scoped guard in the 0194/0198/0199 style | Task 5 |
| Guard's hand-maintained list matches the landed sweep set | Task 5's SITES table + Task 6 Step 3 cross-check |
| Non-vacuity anchors + mutation-in-fixture probe | Task 5 Steps 1, 3, 4 |
| Lever 2 — run-disposition obligation on the agent | Task 4 Step 3 |
| `advanced` gated on Step 7 **by pointer**, never defining it | Task 4 Step 3 wording + its by-pointer assert |
| No new runtime mechanism / status / field | No task creates one; Task 4's context states it |
| Extend `tests/test_loop_continuation.sh` | Task 4 Step 1 |
| Budgets: compress → re-measure → raise only what exceeds | Tasks 1, 2, 3, 4 (each has an explicit re-measure gate) |
| 0201 raise justification naming the `references/` home | Tasks 1, 2, 3, 4 justification comments |
| Vendored skills untouched | Global Constraints; no task names one |

**2. Placeholder scan.** No "TBD", no "add appropriate handling", no "similar to Task N" — each task repeats the code it needs. The `<L>`/`<W>` slots in the budget comments are outputs of a stated deterministic derivation (measure, then round by the file's documented rule), not decisions deferred to the implementer.

**3. Type consistency.** The two literal anchors are spelled identically in the *canonical scoping clause* section, in every task's insert text, and in Task 5's `INLINE_HALF` / `DISPATCH_HALF`. The helper names `anchor_line` and `clause_near` are used consistently within Task 5. The verbatim anchor clauses in Task 5's SITES table match the clauses named in Tasks 1–3's **Files** blocks, including the em dash in `Then you stop — review is not yours.`

## Risks the reviewer should look at

- **Task 5's `clause_near` window is 6 lines.** If a task's insert lands the clause further from its anchor than that, the guard reddens correctly but the fix is to move the clause, never to widen the window.
- **`anchor_line` uses capture-then-slice**, not `grep | head`, to stay clear of the SIGPIPE-141 shape `AGENTS.md` forbids. The `${hits%%$'\n'*}` form needs a real bash (`$DOCKET_BASH_PATH`), which is how the suite runs every test; a reviewer should confirm nothing invokes these tests under `sh`.
- **Three tasks edit `tests/test_skill_size_budgets.sh`.** Task 6 Step 2 is the cross-check that no raise was lost.
