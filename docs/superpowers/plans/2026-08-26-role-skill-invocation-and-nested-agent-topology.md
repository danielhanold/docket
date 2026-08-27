<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0355 — Build/review roles are skill-invoked that fan out to profile agents — Step 5 'dispatch' vocabulary invites an agent-not-found misfire](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-27-0355-build-review-roles-are-skill-invoked-that-fan-out-to-profile.md)**
<!-- docket:backlink:end -->
# Role-Skill Invocation and Nested-Agent Topology Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use the resolved build skill (docket-build) to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make every resolved `skills.*` role binding unambiguously a skill invocation — never a same-name agent dispatch — and make the built-in build/review fan-out conditional on the built-in binding, so a rejected `docket-build`/`docket-review` agent attempt can never read as Tier-C evidence or add an unwanted Docket review to a custom binding.

**Architecture:** Documentation/wording clarification to two skill bodies plus three mutation-tested shell prose guards. One generic rule lands in `docket-convention`'s Skill layer; `docket-implement-next` Step 5 re-frames the build seam as a role invocation with `docket-build`-conditional profile fan-out; Step 6 makes the Docket reviewer-rung dispatch conditional on the `docket-review` binding. Each prose edit is pinned by guards that detect the *removed* state, not merely confirm the new wording.

**Tech Stack:** Markdown skill bodies; bash test guards (`tests/test_*.sh`, house `assert`/`flat` idioms); `scripts/run-tests.sh` suite.

**Spec:** `docs/superpowers/specs/2026-08-26-role-skill-invocation-and-nested-agent-topology-design.md` (on the `docket` metadata branch; synchronized copy at `.docket/docs/superpowers/specs/…` in the primary tree).

## Global Constraints

- Exactly five maintained source files change (spec "Source changes"): `skills/docket-convention/SKILL.md`, `skills/docket-implement-next/SKILL.md`, `tests/test_skill_handoff_precedence.sh`, `tests/test_docket_build.sh`, `tests/test_docket_review.sh`. Plus this plan file — nothing else. In particular do NOT touch `AGENTS.md`, `CLAUDE.md`, `internal/harness/dispatch.go`, `sync-agents.sh`, generated fixtures, `skills/docket-build/SKILL.md`, `skills/docket-review/SKILL.md`, or any agent wrapper — the parent-facing managed dispatch block must stay byte-untouched (spec acceptance criterion 6).
- Do NOT add a `docket-build` or `docket-review` agent wrapper. The absence is the design.
- Never relax the Tier-C authorized-or-halt invariant — only clarify its trigger. A genuinely unavailable *required nested* dispatch (a profile worker, a selected rung, a custom skill's own required dispatch) still halts.
- Guard discipline (AGENTS.md + learnings): every new assert must be mutation-tested (strip the guarded thing, watch it redden); one bounded gap per ERE pattern, never two stacked (`stacked-gap-regex-hangs-instead-of-failing`); match whitespace-flattened haystacks for phrase asserts (`phrase-grep-over-wrapped-prose`); bind phrases to their claims, not bare vocabulary (`prose-guard-binds-phrase-to-claim`); write negative asserts that detect the removed state (`assert-detects-removal-not-replacement`).
- Mutation-test procedure (every probe): `cp "$f" "$f.bak"` first; apply the mutation; **prove it landed** with `/usr/bin/grep -cF` over a whitespace-flattened copy (`tr -s '[:space:]' ' ' < "$f"`) before/after — PATH `grep` is ugrep and `$` in fixed strings misreads through it; run the focused test and confirm the *named* assert reddens; restore with `mv "$f.bak" "$f"` — never `git checkout --` (`mutation-restore-needs-a-backup-copy`).
- The paragraphs being edited in `skills/docket-implement-next/SKILL.md` are single physical lines (one paragraph per line). Plain string replacement works, but flatten-count anyway when proving mutations landed.
- The house `assert` helper `eval`s its condition string; escape backticks inside double-quoted pattern segments as `` \` `` exactly as the existing asserts in these files do (`test-helper-interpolates-its-own-description`).
- The suite command is whatever `finalize.test_command` resolves to (it is `bash scripts/run-tests.sh` in this repo — read it from config, never a second copy) and must be run whole at the end, watching for `BUDGET WATCH:` / `SERIAL CONFIRMED OVER BUDGET:` lines.

## File Structure

- Modify: `skills/docket-implement-next/SKILL.md` — Step 5 paragraph (the single line beginning `**Refresh the claim** (\`docket change refresh-claim\`) immediately before this long build dispatch`) and Step 6's review-invocation paragraph (the single line beginning `The **resolved review skill** — \`$SKILL_REVIEW\``).
- Modify: `tests/test_docket_build.sh` — append a new "Step-5 seam" guard section slicing implement-next's Step 5.
- Modify: `tests/test_docket_review.sh` — extend the existing `step6` controller section (around its `step6="$(awk …)"` slice).
- Modify: `skills/docket-convention/SKILL.md` — one new bullet in `### Skill layer — pluggable workflow skills (change 0049)`.
- Modify: `tests/test_skill_handoff_precedence.sh` — two new group-1 asserts on the Skill-layer slice; one new per-site assert in the group-2 loop; fixtures proving non-vacuity.

Task order is chosen so every commit is suite-green: the generic per-site dispatch-framing ban (Task 3) would redden against today's Step 5 line, so Step 5 is fixed first (Task 1).

---

### Task 1: Step 5 — build-role invocation with `docket-build`-conditional fan-out, plus the build seam guard

**Files:**
- Modify: `skills/docket-implement-next/SKILL.md` (the `### Step 5 — Build` paragraph)
- Test: `tests/test_docket_build.sh` (append new section at end of file, before the final `[ "$fail" -eq 0 ]` line)

**Interfaces:**
- Consumes: existing `assert`/`flat` helpers and `$REPO` in `tests/test_docket_build.sh`.
- Produces: Step 5 prose whose invocation line carries no `long <word> dispatch` framing — Task 3's generic per-site ban depends on this; the exact Step 5 sentences quoted below — the guards match them verbatim, so prose and pattern must move together.

- [ ] **Step 1: Write the failing guards**

Append to `tests/test_docket_build.sh`, immediately before the closing `[ "$fail" -eq 0 ] && echo "ALL OK" || echo "FAILURES"` / `exit "$fail"` lines:

```bash
# --- change 0355: the Step-5 seam in docket-implement-next -----------------------------------
# The build role is a SKILL INVOCATION whose profile fan-out belongs to the docket-build binding;
# a custom skills.build value owns its own topology. Guards detect the removed state (the
# role-invocation-as-dispatch framing observed live on change 0351), not merely the new wording.
IMPL="$REPO/skills/docket-implement-next/SKILL.md"
step5="$(awk '/^### Step 5 — Build/{f=1;next} /^### Step 6 — Review/{f=0} f' "$IMPL")"
assert "seam: Step 5 was located (non-vacuity anchor)" '[ -n "$step5" ]'
# Named terminator + existence assert (learnings: section-slice-needs-a-named-terminator): a
# renamed Step-6 heading must redden here, not silently widen the slice to EOF.
assert "seam: the named Step-5 slice terminator exists" 'grep -q "^### Step 6 — Review" "$IMPL"'
step5_flat="$(flat "$step5")"
# Detect the removed state: Step 5 must never again call the role invocation a build dispatch.
assert "seam: the role-invocation-as-dispatch framing is absent (no 'long build dispatch')" \
  '! grep -qiE "long build dispatch" <<<"$step5_flat"'
# The invocation itself stays directed — sigil bound to the marker with one bounded gap.
assert "seam: \$SKILL_BUILD is invoked with the DIRECTED-to marker" \
  'grep -qE "SKILL_BUILD[^.]{0,160}DIRECTED to:" <<<"$step5_flat"'
# docket-build is the CONDITION that owns Docket profile routing (bound, single gap).
assert "seam: the docket-build binding owns the profile-worker fan-out" \
  'grep -qiE "when the value is \`docket-build\`[^.]{0,160}build-profile worker" <<<"$step5_flat"'
# The custom-binding branch adds no Docket topology (bound, single gap).
assert "seam: a custom build skill owns its own topology" \
  'grep -qiE "custom build skill owns its own[^.]{0,80}topology" <<<"$step5_flat"'
assert "seam: no Docket profile dispatches are added to a custom binding" \
  'grep -qiE "adds no docket profile dispatch" <<<"$step5_flat"'
# A rejected same-name role-agent attempt is the wrong operation, never the Tier-C trigger.
assert "seam: a rejected same-name docket-build attempt is not Tier-C evidence" \
  'grep -qiE "same-name \`docket-build\` agent[^.]{0,120}wrong operation" <<<"$step5_flat"'
# Tier C still attaches — but at the REQUIRED NESTED dispatch boundary, not the role noun.
assert "seam: Tier C keys on a required nested dispatch" \
  'grep -qiE "required nested dispatch[^.]{0,240}Tier C" <<<"$step5_flat"'
```

- [ ] **Step 2: Run the guards to verify the new ones fail for the intended reason**

Run: `bash tests/test_docket_build.sh 2>&1 | grep -E "NOT OK|ALL OK|FAILURES"`
Expected: FAILURES — exactly these red: `no 'long build dispatch'` (the phrase is present today), `docket-build binding owns the profile-worker fan-out`, both custom-topology asserts, `same-name docket-build attempt`, and `required nested dispatch` (none of that prose exists yet). Green already: slice located, terminator exists, `DIRECTED to:` bound. If a different set reddens, stop and reconcile the patterns before touching prose.

- [ ] **Step 3: Rewrite the Step 5 paragraph**

In `skills/docket-implement-next/SKILL.md`, replace the entire single-line paragraph that currently begins `**Refresh the claim** (\`docket change refresh-claim\`) immediately before this long build dispatch` (the only body line under `### Step 5 — Build`) with the following, as ONE line (shown wrapped here for readability — join into a single physical line, single spaces between sentences):

```markdown
**Refresh the claim** (`docket change refresh-claim`) immediately before this long **build-role
invocation** and again when it returns — a refresh `contended` stops a new invocation but never
cancels a build already running. The **resolved build skill** — `$SKILL_BUILD` from the Step-0
config export (default `docket-build`) — is invoked **DIRECTED to:** execute the plan
task-by-task and stop at the executed plan. A resolved `skills.build` value names a skill to
invoke, never a same-name agent to dispatch (convention, *Skill layer*): when the value is
`docket-build`, that invoked skill directs the controller to dispatch one named build-profile
worker per plan task and gates on one full-suite run; a custom build skill owns its own
execution and dispatch topology, and the driver adds no Docket profile dispatch on top of it. A
rejected attempt to dispatch a same-name `docket-build` agent is the wrong operation, never
capability evidence — resume at the skill invocation; no metadata or worktree state changed
because that probe was refused. **Proceed through the build — the deliverable is the executed
plan, never the decision about how to execute it.** Separately, and without ever relaxing the
first: answer any choice it poses from resolved config, surface none, and log one line naming
the role and skill if you suppressed a hand-off. Emitting that log line discharges the
suppression obligation only; the step is not complete until its git-state postcondition holds —
see *Step postconditions*. On `auto` or unavailability, apply the build auto-fallback per the
convention's *Skill layer* (execute the plan on the feature branch, warning prominently) — the
artifact is the executed plan; method is the agent's choice. If the invoked skill's genuinely
**required nested dispatch** — a build-profile worker, or a custom skill's own required dispatch
— is unavailable (established only per the convention's *Dispatch-capability resolution* — never
from a tool name), the build role is **Tier C, authorized-or-halt**: an explicitly configured
`auto` authorizes the inline path above; any other resolved value is abort-and-report, leaving
the change `in-progress` with `claimed_at` refreshed and the halt reason recorded.
```

Content preserved from the old paragraph: refresh-claim bracketing, the `DIRECTED to:` direction, proceed-through-the-build, choice suppression + log line, postcondition pointer, auto/missing fallback, Tier-C authorized-or-halt consequence. Changed: "long build dispatch" → "long **build-role invocation**"; the standalone "docket-build routes each task to a profile agent and gates on one full-suite run." sentence is absorbed into the new `docket-build`-conditional sentence; "invocable but **cannot dispatch**" → the required-nested-dispatch trigger; the wrong-operation sentence is new.

- [ ] **Step 4: Run both affected test files to verify they pass**

Run: `bash tests/test_docket_build.sh 2>&1 | tail -3` and `bash tests/test_skill_handoff_precedence.sh 2>&1 | tail -3`
Expected: `ALL OK` from both (the precedence guard's `DIRECTED to:` marker and mention classifier must still hold on the rewritten line — the word "skill" and the marker both survive).

- [ ] **Step 5: Mutation-test the new guards (spec Verification probes 1 and 2)**

For each probe: `cp skills/docket-implement-next/SKILL.md skills/docket-implement-next/SKILL.md.bak`, mutate, prove landed (`tr -s '[:space:]' ' ' < skills/docket-implement-next/SKILL.md | /usr/bin/grep -cF "<mutated phrase>"` before/after), run `bash tests/test_docket_build.sh`, confirm the NAMED assert is the one that reddens, then `mv skills/docket-implement-next/SKILL.md.bak skills/docket-implement-next/SKILL.md` and re-run to green.

1. **Restore the framing:** replace `long **build-role invocation**` with `long build dispatch`. Expected red: `seam: the role-invocation-as-dispatch framing is absent`.
2. **Delete the custom-topology clause:** remove `; a custom build skill owns its own execution and dispatch topology, and the driver adds no Docket profile dispatch on top of it`. Expected red: both custom-topology asserts (and only those).
3. **Delete the wrong-operation sentence:** remove the sentence starting `A rejected attempt to dispatch a same-name`. Expected red: `same-name docket-build attempt is not Tier-C evidence`.
4. **Delete the conditional sentence:** remove `when the value is \`docket-build\`, that invoked skill directs the controller to dispatch one named build-profile worker per plan task and gates on one full-suite run`. Expected red: `docket-build binding owns the profile-worker fan-out`.

- [ ] **Step 6: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/build-review-roles-are-skill-invoked-that-fan-out-to-profile
git add skills/docket-implement-next/SKILL.md tests/test_docket_build.sh
git commit -m "fix(implement-next): Step 5 is a build-role invocation; docket-build owns the profile fan-out"
```

---

### Task 2: Step 6 — reviewer-rung dispatch conditional on the `docket-review` binding, plus the review seam guard

**Files:**
- Modify: `skills/docket-implement-next/SKILL.md` (the `$SKILL_REVIEW` paragraph inside `### Step 6 — Review + ADRs`)
- Test: `tests/test_docket_review.sh` (extend the existing controller section that computes `step6=`)

**Interfaces:**
- Consumes: the existing `step6="$(awk "/^### Step 6 — Review/{f=1;next} /^### Step 6.5/{f=0} f" "$IMPL")"` slice, `assert`, and `$IMPL` in `tests/test_docket_review.sh`; the existing `flat` helper if present — if `tests/test_docket_review.sh` has no `flat()` helper, add the same three-line helper used by `tests/test_docket_build.sh` near the top.
- Produces: Step 6 prose in which `Dispatch the selected rung wrapper` is governed by `When \`$SKILL_REVIEW\` is \`docket-review\`` — Task 3's generic ban and these guards both read it; the rung-selection paragraph (highest-profile rule, diff-size modifier) is byte-untouched.

- [ ] **Step 1: Write the failing guards**

In `tests/test_docket_review.sh`, directly after the existing block of `step6` asserts (after the `no re-review` assert, before whatever section follows), insert:

```bash
# --- change 0355: rung dispatch is docket-review's topology, not a universal post-step -------
# A resolved skills.review value is invoked as a skill; ONLY the built-in docket-review binding
# gets the deterministic Docket rung dispatch. A custom binding returns its own findings and
# receives no additional Docket review. Guards bind the dispatch sentence to its condition so
# deleting the condition (re-unconditionalizing the dispatch) reddens.
assert "controller: the named Step-6 slice terminator exists" 'grep -q "^### Step 6.5" "$IMPL"'
step6_flat="$(flat "$step6")"
assert "controller: \$SKILL_REVIEW remains a directed skill invocation" \
  'grep -qE "SKILL_REVIEW[^.]{0,160}DIRECTED to:" <<<"$step6_flat"'
assert "controller: rung dispatch is conditional on the docket-review binding" \
  'grep -qiE "is \`docket-review\`\*\*[^.]{0,120}rung wrapper" <<<"$step6_flat"'
assert "controller: the rung fan-out is the binding's topology, not a universal post-step" \
  'grep -qiE "\`docket-review\`.s (own )?topology" <<<"$step6_flat"'
assert "controller: a custom review binding receives no additional Docket rung" \
  'grep -qiE "dispatch \*\*no\*\* docket reviewer rung in addition" <<<"$step6_flat"'
assert "controller: the auto fallback dispatches no reviewer" \
  'grep -qiE "warning prominently.[^.]{0,60}dispatch no reviewer" <<<"$step6_flat"'
```

Note the escaping: inside the eval'd single-quoted condition, backticks are written `` \` `` within double-quoted pattern text, exactly like the file's existing `docket-review-$r` asserts. `.s` in the topology pattern matches the flattened `'s` possessive without quoting headaches. One bounded gap per pattern.

- [ ] **Step 2: Run to verify the new guards fail for the intended reason**

Run: `bash tests/test_docket_review.sh 2>&1 | grep -E "NOT OK|ALL OK|FAILURES"`
Expected: FAILURES — red: the conditional-rung assert, the topology assert, the no-additional-rung assert, and the auto-no-reviewer assert (that prose does not exist yet). Green already: terminator exists, `SKILL_REVIEW … DIRECTED to:`. A different red set means a pattern is wrong — fix patterns first.

- [ ] **Step 3: Rewrite the `$SKILL_REVIEW` paragraph**

In `skills/docket-implement-next/SKILL.md`, replace the single-line paragraph beginning `The **resolved review skill** — \`$SKILL_REVIEW\`` (the fourth paragraph under `### Step 6 — Review + ADRs`; leave the evidence, rung-*selection*, and fix-loop paragraphs untouched) with the following, joined into ONE physical line:

```markdown
The **resolved review skill** — `$SKILL_REVIEW` from the Step-0 config export (default
`docket-review`) — is invoked **DIRECTED to:** review the whole branch against its base and
return its findings, then stop, answering any choice it poses from resolved config and never
surfacing one — log one line naming the role and skill if you suppressed a hand-off. A resolved
`skills.review` value names a skill to invoke, never a same-name agent to dispatch; the fan-out
below is `docket-review`'s topology, not a universal post-step. **When `$SKILL_REVIEW` is
`docket-review`**, dispatch the selected rung wrapper by name, foreground, passing it the branch
and base ref, the change's title and scope, the relevant learnings hooks, and the evidence
record. **When `$SKILL_REVIEW` names any other invocable skill**, consume that custom review's
whole-branch findings directly and dispatch **no** Docket reviewer rung in addition — a custom
review binding owns its own topology. On `auto` or unavailability, apply the review
auto-fallback per the convention's *Skill layer* (a whole-branch review before the PR opens,
warning prominently) and dispatch no reviewer. Name the **feature worktree** in any rung
dispatch payload: a reviewer reached through a runner delegation receives its worktree through
the facade's `--worktree` flag, and a delegated dispatch that names none is refused. Re-read the
learnings index `<changes_dir>/learnings/README.md` first and pull the findings relevant to what
this change touched (skipped entirely when `learnings.enabled` is `false`). For any non-obvious
decision made during implementation, **dispatch the `docket-adr` subagent** (foreground, at the
model/effort its wrapper resolves) — once per decision; it assigns the number, updates the
index, commits the ADR on `origin/docket`, publishes it onto the integration branch on
acceptance if the repo has opted in, and **returns the number**. After re-syncing `.docket/`,
record that number in the change's `adrs:` relation through a `docket change reconcile`
relations update — the transaction re-checks the exact version and re-renders the `## Artifacts`
block and inline board atomically, so the skill never hand-renders a derived view. Review
findings that are distinct follow-up work — not this change's own fixes — are likewise
classified and minted per *Auto-capture* when `AUTO_CAPTURE_ENABLED` is `true`, carrying the
running `--minted` count forward from the reconcile pass; a policy-suppressed candidate is
reported and does not increment it. On unavailable dispatch — established only per the
convention's *Dispatch-capability resolution*, never from a tool name, and never from a rejected
same-name role-agent attempt — a genuinely required nested dispatch (the selected rung for the
`docket-review` binding, or a custom skill's own required dispatch) makes the review role
**Tier C** on the same authorized-or-halt terms as step 5; the `docket-adr` dispatch is
**Tier A**, running inline instead with its git-state contract unchanged.
```

Changed vs. the current paragraph: the invoke-not-dispatch sentence is new; `Dispatch the selected rung wrapper` gained the `When \`$SKILL_REVIEW\` is \`docket-review\`` condition; the custom-binding and auto branches are explicit; the Tier-C sentence names the required-nested-dispatch trigger and excludes the same-name attempt. Everything else (worktree naming, learnings re-read, ADR dispatch, auto-capture) is byte-preserved.

- [ ] **Step 4: Run the affected test files to verify they pass**

Run: `bash tests/test_docket_review.sh 2>&1 | tail -3` and `bash tests/test_skill_handoff_precedence.sh 2>&1 | tail -3` and `bash tests/test_docket_build.sh 2>&1 | tail -3`
Expected: `ALL OK` from all three (the review file's pre-existing step6 asserts — three rung names, `highest profile`, fix loop, `REVIEW_MIN_FIX_SEVERITY`, `docket gate drive` — must all still pass; the rung names now appear via the selection paragraph and the conditional dispatch).

- [ ] **Step 5: Mutation-test (spec Verification probe 3)**

Backup-copy procedure as in Task 1 Step 5, target file `skills/docket-implement-next/SKILL.md`, focused test `bash tests/test_docket_review.sh`:

1. **Make the rung dispatch unconditional again:** replace `**When \`$SKILL_REVIEW\` is \`docket-review\`**, dispatch the selected rung wrapper` with `Dispatch the selected rung wrapper` (deleting the condition). Expected red: `rung dispatch is conditional on the docket-review binding`.
2. **Delete the custom-binding sentence** (the sentence starting `**When \`$SKILL_REVIEW\` names any other invocable skill**`). Expected red: `custom review binding receives no additional Docket rung`.
3. **Delete `and dispatch no reviewer`** from the auto branch. Expected red: `the auto fallback dispatches no reviewer`.

Prove each mutation landed via flattened `/usr/bin/grep -cF` counts; restore from `.bak`; re-run to green after each.

- [ ] **Step 6: Commit**

```bash
git add skills/docket-implement-next/SKILL.md tests/test_docket_review.sh
git commit -m "fix(implement-next): Step 6 rung dispatch is conditional on the docket-review binding"
```

---

### Task 3: Convention Skill layer — the generic role-binding boundary, plus the generic invocation guard

**Files:**
- Modify: `skills/docket-convention/SKILL.md` (`### Skill layer — pluggable workflow skills (change 0049)` bullet list)
- Test: `tests/test_skill_handoff_precedence.sh`

**Interfaces:**
- Consumes: Tasks 1–2's Step 5/6 prose (the per-site ban below runs over those invocation lines and passes only because Task 1 removed the framing); the file's existing `$LAYER` slice, `$MARKER`, `$SITE_RE`, and site-classification loop.
- Produces: the convention's generic rule sentence — quoted verbatim by the group-1 patterns below.

- [ ] **Step 1: Write the failing guards**

Three edits to `tests/test_skill_handoff_precedence.sh`:

(a) After the existing group-1 asserts (below `Skill layer names the call-site marker`), add:

```bash
# Change 0355: the Skill layer states the generic role-binding boundary — a resolved binding is
# INVOKED as a skill (never dispatched as a same-name agent), and nested agent dispatch belongs
# to the invoked skill's own contract. Flattened haystack; one bounded gap per pattern.
LAYER_FLAT="$(tr -s '[:space:]' ' ' <<<"$LAYER")"
assert "Skill layer: a resolved binding names a skill to invoke, not a same-name agent" \
  'grep -qiE "names a skill to \*\*invoke\*\*[^.]{0,80}not a same-name agent" <<<"$LAYER_FLAT"'
assert "Skill layer: nested agent dispatch belongs to the invoked skill.s contract" \
  'grep -qiE "nested (named-)?agent dispatch[^.]{0,80}invoked skill" <<<"$LAYER_FLAT"'
```

(b) Inside the group-2 loop, immediately after the existing `autonomous role invocation pre-specifies its outcome` assert (still inside the invocation branch, after the mention `continue`), add:

```bash
  # Change 0355: an invocation line must not frame the ROLE invocation itself as a dispatch
  # ("this long build dispatch" — the observed 0351 misfire bait). Genuine nested-dispatch prose
  # ("dispatch the selected rung wrapper") is untouched: the ban keys on the removed framing
  # shape, not on the word dispatch. Step 4's "long plan dispatch" line is a real subagent
  # dispatch AND a sigil MENTION, so the mention branch above already exempts it.
  assert "$rel:$lno role invocation is not framed as a dispatch" \
    '! grep -qiE "long [a-z-]+ dispatch" <<<"$text"'
```

(c) In the non-vacuity block at the bottom, after the `BRACED` asserts, add:

```bash
# The dispatch-framing ban must catch the observed defect shape (change 0351's Step-5 line).
FRAMED='Refresh the claim immediately before this long build dispatch — the resolved build skill `$SKILL_BUILD` is invoked **DIRECTED to:** execute the plan.'
assert "the framing ban is non-vacuous (the 0351 defect line is caught)" \
  'grep -qiE "long [a-z-]+ dispatch" <<<"$FRAMED"'
assert "the framing ban permits genuine nested-dispatch prose" \
  '! grep -qiE "long [a-z-]+ dispatch" <<<"Dispatch the selected rung wrapper by name, foreground"'
```

- [ ] **Step 2: Run to verify group 1 fails for the intended reason**

Run: `bash tests/test_skill_handoff_precedence.sh 2>&1 | grep -E "NOT OK|ALL OK|FAILURES"`
Expected: FAILURES — red: both new group-1 asserts (the convention rule is not written yet). Green: every per-site framing ban (Tasks 1–2 already cleaned the three invocation lines) and both fixtures. If a per-site ban is red, a Step 5/6 line still carries the framing — fix the prose, not the guard.

- [ ] **Step 3: Add the convention bullet**

In `skills/docket-convention/SKILL.md`, in the `### Skill layer` bullet list, insert this bullet immediately after the `**Passthrough.**` bullet (it refines what a passed-through value *is*), as one bullet (may hard-wrap like its neighbors):

```markdown
- **Role binding is a skill invocation.** A resolved `skills.<role>` value names a skill to
  **invoke**, not a same-name agent to dispatch; any nested agent dispatch belongs to the
  invoked skill's own contract, and the driver never infers or adds a topology from the role
  noun or the configured skill name. A rejected attempt to dispatch a nonexistent same-name role
  agent is the wrong operation — neither this section's missing-skill condition nor
  *Dispatch-capability resolution*'s Tier-C evidence, which attaches only at a required nested
  dispatch the invoked skill's contract actually reaches.
```

- [ ] **Step 4: Run to verify everything passes**

Run: `bash tests/test_skill_handoff_precedence.sh 2>&1 | tail -3`
Expected: `ALL OK`. Also confirm the floors still hold in the output (`checked >= 4`, marker-bearing `>= 3`) — the convention edit adds no `$SKILL_` sigil line, so the population is unchanged.

- [ ] **Step 5: Mutation-test (spec Verification probe 4, both halves, plus the site ban)**

Backup-copy procedure as before:

1. Target `skills/docket-convention/SKILL.md`: delete the first half of the rule (`names a skill to **invoke**, not a same-name agent to dispatch` → `names a skill`). Expected red: `names a skill to invoke, not a same-name agent`. Restore.
2. Target `skills/docket-convention/SKILL.md`: delete `any nested agent dispatch belongs to the invoked skill's own contract` (the clause, keeping the rest). Expected red: `nested agent dispatch belongs to the invoked skill.s contract`. Restore.
3. Target `skills/docket-implement-next/SKILL.md`: re-insert `long build dispatch` framing on the Step-5 invocation line (replace `long **build-role invocation**`). Expected red in THIS file: the per-site `role invocation is not framed as a dispatch` assert for that line (and `tests/test_docket_build.sh`'s seam assert, if also run). Restore.

Prove landed / restore / re-green per the Global Constraints procedure each time.

- [ ] **Step 6: Commit**

```bash
git add skills/docket-convention/SKILL.md tests/test_skill_handoff_precedence.sh
git commit -m "fix(convention): a resolved role binding is a skill invocation, never a same-name agent dispatch"
```

---

### Task 4: Whole-suite gate and scope verification

**Files:**
- No new edits. Verification only.

- [ ] **Step 1: Verify the change's file scope is exactly the spec's**

Run: `git diff --name-only 6e74df71ff786f7910391c3d4ff1e5a0b79373c8..HEAD`
Expected: exactly six paths — the five source files from Global Constraints plus `docs/superpowers/plans/2026-08-26-role-skill-invocation-and-nested-agent-topology.md`. Anything else (especially `AGENTS.md`, `CLAUDE.md`, anything under `agents/` or `internal/`) violates spec acceptance criterion 6 — revert it.

- [ ] **Step 2: Confirm no same-name wrapper appeared**

Run: `ls agents/docket-build.md agents/docket-review.md 2>&1`
Expected: both `No such file or directory` — the absence is the design (spec Out of scope).

- [ ] **Step 3: Run the full suite**

Run the command `finalize.test_command` resolves to (this repo: `bash scripts/run-tests.sh`), whole — never only the touched files.
Expected: suite green. Read the summary `SUITE` line, not a piped exit code. If a `BUDGET WATCH:`/`PARALLEL-SENSITIVE:` line names one of the three touched test files, confirm serially per `scripts/run-tests.md`; a `SERIAL CONFIRMED OVER BUDGET:` line is an authoritative breach to fix (the added guards are cheap greps — a breach means something is wrong, likely a backtracking pattern; check for stacked gaps).

- [ ] **Step 4: Final read-through against the spec's acceptance criteria**

Check each of the spec's eight acceptance criteria against the diff: (1) non-`auto` bindings described as skill invocations — Steps 5/6 + convention bullet; (2) built-in topologies conditional — Step 5 `when the value is`, Step 6 `When … is docket-review`; (3) custom bindings get no extra fan-out — both custom sentences; (4) rejected same-name attempt cannot trigger Tier C — Step 5 sentence, Step 6 `never from a rejected same-name role-agent attempt`, convention bullet; (5) genuine required-nested unavailability still Tier C — both Tier-C sentences retained; (6) managed dispatch block byte-untouched — Step 1 of this task; (7) guards mutation-proven — Tasks 1–3 Step 5s; (8) suite green, no authoritative budget breach — Step 3. No commit unless all eight hold.
