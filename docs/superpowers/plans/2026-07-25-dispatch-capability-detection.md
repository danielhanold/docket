<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0137 — Claude Code dispatch-capability detection: name-based probing silently drops SDD build and review discipline](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-07-27-0137-forked-claude-code-skills-assume-absent-task-dispatch.md)**
<!-- docket:backlink:end -->

# Dispatch-Capability Detection Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make docket's dispatch-capability detection honest — resolve a dispatch *capability* instead of looking for a specifically-named tool — and define a tiered posture for what an autonomous run does when dispatch is genuinely unavailable.

**Architecture:** Three layers, in dependency order. A **gating live spike** (Task 1) answers the change's open question before any product code exists, because one of its outcomes cancels the design. A **normative rule + tier table** lands in `skills/docket-convention/SKILL.md` (Task 2), the single source every operating skill loads at Step 0. **Consuming sites** then name their tier at the point of dispatch (Task 3), so the rule has a producer and not just a definition. Finally the two factually-wrong prose sites are corrected and a **shape-based negative guard** keeps them corrected (Task 4), and the whole suite plus guard mutation-testing runs as the gate (Task 5).

**Tech Stack:** Markdown skill prose; hermetic Bash test suite (`tests/test_*.sh`, run with `$DOCKET_BASH_PATH`); no runtime code, no dependencies.

> **The test code below was smoke-run against the real tree before this plan was saved** (learnings: `plan-supplied-test-code-is-unverified` — test code a plan hands you is unverified code, not an oracle). Confirmed at plan time: all five `check_site` anchors resolve to a real paragraph; the five tier asserts are red for the right reason (the markers do not exist yet); the negative guard's whole-repo grep returns exactly four hits, two Cursor-scoped and two offenders — the two sites Task 4 corrects; the population floors and the positive control pass. You should still re-run each step's *verify it fails* step rather than trusting this note.

## Global Constraints

- **Spec:** `.docket/docs/superpowers/specs/2026-07-25-dispatch-capability-detection-design.md` on the `docket` branch (read it from the metadata worktree; it is not on this branch). Its *Reconcile addendum* carries four build-facing constraints and is as binding as the Decision.
- **Harness-neutral by construction.** Every normative sentence is stated by **capability**, never by tool name, and never by harness. A tool name may appear only inside a diagnostic string or a dated historical record. This is the change's whole point — a Claude-Code-flavored rule fails the change.
- **`AGENTS.md` binds (always-in-context rules).** Specifically: a guard is code and must be mutation-tested; key a guard on syntactic **shape**, never an enumerated list of spellings; **never hand-list the sites** of a literal you are gating — derive them from a whole-repo grep; run the **whole suite** at the build gate; a cross-reference anchors on a symbol name or a verbatim-quoted clause, **never a line number** (`tests/test_comment_anchor_style.sh` enforces this — name the site the way this bullet does, e.g. `references/agent-layer.md`'s *Both invocation paths land on the same pinned wrapper* paragraph, never a filename-plus-line-number pointer at it).
- **Immutable records stay immutable.** An `Accepted` ADR changes only its `status:` line; `docs/adrs/0024-claude-context-fork-skill-dispatch.md`'s wrong `Task` sentence is corrected by an appended dated `## Update`, **never** an edit. Archived change files and `docs/results/` files are point-in-time records — never rewrite them to satisfy a guard.
- **ADR authoring is NOT a plan task.** docket's own Step 6 dispatches the `docket-adr` subagent for the new ADR and for ADR-0024's `## Update`; both land on the `docket` branch and publish from there. Do not create or edit anything under `docs/adrs/` on this feature branch.
- **Size budgets have ~zero headroom.** `tests/test_skill_size_budgets.sh` caps `skills/docket-convention/SKILL.md` at 354 lines / 5850 words (actual: 347 / 5848) and `skills/docket-implement-next/SKILL.md` at 147 / 3315 (actual: 135 / 3307). Growing either **requires** raising its row in the same diff, with a justification appended to the comment block above `BUDGETS` — the guard's own sanctioned escape hatch (precedent: changes 0127, 0102).
- **Test runtime:** run every test as `"$DOCKET_BASH_PATH" tests/<name>.sh` (this machine: `/opt/homebrew/bin/bash`). Never assume `/bin/bash`.
- **Suite shape:** the detected suite is `for t in tests/test_*.sh; do "$DOCKET_BASH_PATH" "$t"; done`. A new test file is auto-discovered by that glob — no registration step.
- **Commit on the feature branch only.** `feat/forked-claude-code-skills-assume-absent-task-dispatch`, in the worktree `.worktrees/forked-claude-code-skills-assume-absent-task-dispatch`. Never commit docket metadata (change files, `BOARD.md`, ADRs) here.

---

### Task 1: Gating live spike — does dispatch resolve on both invocation paths?

**This task can cancel the change.** Run it first, before any product edit. The spec's *Gating branch on the spike*: if a real `context: fork` child categorically lacks dispatch, Tier C would halt **every** forked build — bricking `/docket-implement-next`, the path ADR-0024 names first-class — so the change **stops and reports to the human** instead of shipping that posture. Do not resolve that silently, and do not "fix" it by weakening Tier C on your own authority.

**Files:**
- Create: `docs/results/2026-07-25-dispatch-capability-detection-results.md`

**Interfaces:**
- Produces: the results file, with its `## Live dispatch spike` section fully written. Later tasks append to this same file; Task 5 finishes it. The spike's verdict (GO / STOP) is what Task 2 depends on.

- [ ] **Step 1: Probe path A — an agent-dispatched child**

Dispatch a subagent (`general-purpose` is fine) with this exact prompt, and capture its reply verbatim:

```
Report facts about YOUR OWN runtime. Do not speculate, do not read files.
1. Do you have a tool that dispatches a subagent? If your tool list is
   partially deferred behind a search tool, SEARCH IT before answering.
2. Name the dispatch tool exactly as it appears to you, or say NONE.
3. Attempt one trivial dispatch: ask a child to reply with the single
   word NESTED_OK. Report the child's literal reply, or the exact error.
4. Do you have a Skill tool? If so, is
   `superpowers:subagent-driven-development` present? Is
   `superpowers:requesting-code-review` present?
5. Do you have a tool for asking the human a question? Name it or say NONE.
Answer as five numbered lines. Facts only.
```

- [ ] **Step 2: Probe path B — a real `context: fork` child**

The fork path is what a directly-invoked docket skill takes. It cannot be reached by dispatching an agent — a fork happens when a skill carrying `context: fork` frontmatter is invoked via the Skill tool. Reach it by invoking a forked docket skill and having it report its own dispatch surface. Use `docket-status` (Tier A, read-mostly, safe to run — it is the same sweep docket's own Step 0 runs, and it is idempotent):

Invoke the skill `docket-status` via the Skill tool with an argument instructing it to, **in addition to** its normal pass, report the five facts from Step 1 about its own runtime.

Record which mechanism you used and note the ambiguity honestly if the run's identity as a genuine fork cannot be confirmed from inside — per the `harness-behavior-is-mode-and-version-scoped` learning, an unverified premise must be labeled as unverified, not rounded up.

- [ ] **Step 3: Capture the harness version**

```bash
claude --version
```

Record the exact output. A finding with no version attached will later be read as universal and be wrong.

- [ ] **Step 4: Write the results file with the spike findings verbatim**

Create `docs/results/2026-07-25-dispatch-capability-detection-results.md`. The house template is `skills/docket-implement-next/results-template.md` in this repo — read it and follow its section order, adding the spike section below. Fill every bracket with what actually happened; **quote the probe replies verbatim**, do not summarize them:

```markdown
# Change 0137 — dispatch-capability detection: results

## Live dispatch spike (gating)

**Harness version:** <exact `claude --version` output>
**Date (UTC):** 2026-07-25
**Scope caveat:** these findings are scoped to the harness version and invocation
mode recorded here (learnings: `harness-behavior-is-mode-and-version-scoped`).
docket's suite is hermetic Bash and cannot dispatch a subagent, so this spike is
the only runtime evidence — there is no standing regression test behind it.

### Path A — agent-dispatched child

<the five numbered lines, verbatim>

### Path B — forked skill child

**How the fork was reached:** <exact mechanism>
**Confirmed a genuine fork:** <yes / no / unverifiable from inside — say which>

<the five numbered lines, verbatim>

### Verdict

<GO — dispatch resolves on both paths, Tier C ships as designed>
<or: STOP — a fork categorically lacks dispatch; halt and report>
```

- [ ] **Step 5: Act on the verdict**

- **GO** — dispatch resolves (or resolves after searching a deferred surface) on both paths: continue to Task 2.
- **STOP** — a genuine fork categorically lacks dispatch: **stop the build here**. Do not start Task 2. Report to the human: what was probed, the verbatim evidence, and the two options the spec names (steer users toward agent-dispatch, or relax Tier C). The change stays `in-progress`.

- [ ] **Step 6: Commit**

```bash
git add docs/results/2026-07-25-dispatch-capability-detection-results.md
git commit -m "spike(0137): live dispatch probe over both invocation paths"
```

---

### Task 2: The capability-resolution rule and tiered posture in docket-convention

**Files:**
- Create: `tests/test_dispatch_capability.sh`
- Modify: `skills/docket-convention/SKILL.md` (new section, placed immediately after *Harness-native recovery after sandbox or permission denial* — the adjacent section of the same shape: a normative, harness-neutral rule for what an agent does when the runtime withholds something)
- Modify: `tests/test_skill_size_budgets.sh` (raise the `skills/docket-convention/SKILL.md` row + justify)

**Interfaces:**
- Produces: the section heading `### Dispatch-capability resolution (change 0137)` and the three tier labels `**A — deterministic**`, `**B — adversarial**`, `**C — discipline**`. Tasks 3 and 4 assert against these exact strings; do not rename them.
- Produces: `tests/test_dispatch_capability.sh` with the helper `assert(){ ... }` and `fail` accumulator following the house pattern in `tests/test_composition_wiring.sh`. Tasks 3 and 4 append sections to this same file.

- [ ] **Step 1: Write the failing test**

Create `tests/test_dispatch_capability.sh`:

```bash
#!/usr/bin/env bash
# tests/test_dispatch_capability.sh — guards change 0137 (dispatch-capability detection).
#   - the convention carries a capability-resolution rule: resolve (incl. deferred tool surfaces),
#     then attempt one trivial dispatch; only a failed attempt or a policy denial proves absence
#   - the rule is stated by CAPABILITY: an absent tool NAME is explicitly not sufficient evidence
#   - the tiered unavailability posture (A deterministic / B adversarial / C discipline) is present
#   - Tier C is drawn AGAINST the Skill layer's missing-skill rule, not layered on top of it
#   - every CONSUMING dispatch site names its tier (producer coverage, not definition-only)
#   - no live docket prose gates a decision on a literal tool name (shape-scoped, no allowlist)
# Sentinels are sampling, not parsing (learnings: foundational-test-discipline) — pair with the
# whole-branch review.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

CONV="$REPO/skills/docket-convention/SKILL.md"

# --- the rule exists and is stated by capability -------------------------------------------------
assert "convention: has a dispatch-capability resolution section" \
  'grep -qiE "^#+ .*[Dd]ispatch-capability resolution" "$CONV"'
assert "convention: resolution includes searching deferred/lazily-loaded tool surfaces" \
  'grep -qiE "deferred or lazily-loaded" "$CONV"'
assert "convention: inconclusive resolution escalates to one trivial dispatch attempt" \
  'grep -qiE "attempt(ed)? one trivial dispatch" "$CONV"'
assert "convention: only a failed attempt or a policy denial establishes unavailability" \
  'grep -qiE "failed attempt.{0,40}policy denial" "$CONV"'
# The load-bearing negative: a missing tool NAME is explicitly not evidence. Deleting this
# sentence is exactly the regression the change exists to prevent, so it gets its own assert.
assert "convention: an absent tool NAME is explicitly insufficient evidence" \
  'grep -qiE "absence of a specifically-named tool never" "$CONV"'
assert "convention: a tool name is a diagnostic, never a decision input" \
  'grep -qiE "never a decision input" "$CONV"'

# --- the tiered posture --------------------------------------------------------------------------
for tier in "A — deterministic" "B — adversarial" "C — discipline"; do
  assert "convention: tier present: $tier" 'grep -qF -- "$tier" "$CONV"'
done
assert "convention: Tier A is a first-class equivalent path, not a degradation" \
  'grep -qiE "first-class equivalent path" "$CONV"'
assert "convention: Tier B routes to the existing abstain" \
  'grep -qiE "[Aa]bstain" "$CONV"'
assert "convention: Tier C is authorized-or-halt" \
  'grep -qiE "authorized-or-halt" "$CONV"'
assert "convention: Tier C names an explicitly configured auto as the authorization" \
  'grep -qiE "explicitly configured .?auto.? is the human" "$CONV"'
assert "convention: Tier C halt adds no new status or field" \
  'grep -qiE "[Nn]o new status, no new field" "$CONV"'

# --- the boundary against the pre-existing missing-skill rule ------------------------------------
# Both rules must coexist and be DISTINGUISHED; if the missing-skill rule vanished, Tier C would
# have silently replaced it (a scope change this change does not authorize).
assert "convention: the missing-skill rule still exists" \
  'grep -qiE "[Mm]issing-skill rule" "$CONV"'
assert "convention: Tier C is distinguished from the missing-skill rule" \
  'grep -qiE "cannot be \*\*invoked\*\*.{0,200}cannot \*\*dispatch\*\*" "$CONV"'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
```

- [ ] **Step 2: Run test to verify it fails**

Run: `"$DOCKET_BASH_PATH" tests/test_dispatch_capability.sh`
Expected: FAIL — every assert `NOT OK` (the section does not exist yet), final line `FAIL`.

- [ ] **Step 3: Write the convention section**

In `skills/docket-convention/SKILL.md`, insert this **immediately after** the final paragraph of the section headed `### Harness-native recovery after sandbox or permission denial` and **before** `### Agent layer — model/effort-pinned subagents (change 0016)`:

```markdown
### Dispatch-capability resolution (change 0137)

A dispatch-dependent step — a composition dispatch, or a role skill that dispatches internally — may be declared unavailable only after the agent has, in order: (1) **resolved** a subagent-dispatch mechanism, **including searching any deferred or lazily-loaded tool surface** the harness exposes, since a partially-loaded tool set makes absence observable without anything having been resolved; and (2) **attempted one trivial dispatch**, if resolution was inconclusive. Only a **failed attempt** or an explicit **policy denial** establishes unavailability. **The absence of a specifically-named tool never does** — the rule is stated by capability, and a tool name is a diagnostic string, **never a decision input**. A failure diagnostic MAY report what was searched for; naming it there commits docket to nothing, the same posture the README takes toward the fork transcript path.

When dispatch is genuinely unavailable the kinds are **not** equivalent, so the posture is tiered:

| Tier | Dispatch | Posture |
|---|---|---|
| **A — deterministic** | the `docket-status` and `docket-adr` composition dispatches | Run the same work **inline** — a **first-class equivalent path**, neither a degradation nor a warning, because the contract is git state on `metadata_branch`, not an in-context return. Every obligation holds unchanged: re-sync before reading, derive from fresh origin, never adopt or commit another agent's uncommitted files. |
| **B — adversarial** | the `docket-auto-groom-critic` gate | **Abstain**, per *Autonomous grooming*. Self-critique by the agent that drafted the spec is not an adversarial gate, and the abstain is a path that skill already owns. |
| **C — discipline** | the `build` and `review` role skills | **Authorized-or-halt.** An **explicitly configured `auto` is the human's** authorization to run inline; any other resolved value that cannot dispatch is **abort-and-report**, leaving the change `in-progress` with `claimed_at` refreshed and the halt reason recorded. **No new status, no new field** — the reclaim lease self-heals an abandoned claim. |

Tier C neither replaces nor softens the *Skill layer*'s **missing-skill rule**: a skill that cannot be **invoked** still degrades to `auto` + warn, while a skill that was invoked and then cannot **dispatch** is Tier C. Two conditions, two postures, one symptom in the run log.
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `"$DOCKET_BASH_PATH" tests/test_dispatch_capability.sh`
Expected: PASS.

- [ ] **Step 5: Raise the size budget in the same diff**

Measure the grown file, then edit the row:

```bash
wc -l skills/docket-convention/SKILL.md; wc -w skills/docket-convention/SKILL.md
"$DOCKET_BASH_PATH" tests/test_skill_size_budgets.sh
```

The budget test will report `NOT OK` for the convention row. In `tests/test_skill_size_budgets.sh`, set the `skills/docket-convention/SKILL.md` row to the **measured actual, rounded up** to the next multiple of 5 for lines and 10 for words — never a padded round number, so the next growth stays conscious. Append to the comment block above `BUDGETS`, in the same voice as the 0127 and 0102 entries, a justification naming: change 0137, the dispatch-capability resolution rule + tier table, that the rule must live in `SKILL.md` rather than a reference because it fires exactly when an agent is about to wrongly conclude dispatch is absent (a rule in an unread reference cannot prevent that), and the old → new numbers.

- [ ] **Step 6: Run both tests to verify green**

Run: `"$DOCKET_BASH_PATH" tests/test_skill_size_budgets.sh && "$DOCKET_BASH_PATH" tests/test_dispatch_capability.sh`
Expected: both PASS.

- [ ] **Step 7: Commit**

```bash
git add skills/docket-convention/SKILL.md tests/test_dispatch_capability.sh tests/test_skill_size_budgets.sh
git commit -m "feat(0137): capability-resolution rule + tiered unavailability posture in the convention"
```

---

### Task 3: Wire the tiers at every consuming dispatch site

The rule must have a **producer**, not only a definition (learnings: `specified-but-unreachable` — every sentinel anchored on a definition passes whether or not any procedural path reaches it). This task makes each dispatch site name its tier, and extends the guard to iterate the **consuming** sites with a population floor (learnings: `correspondence-guard-runs-one-way`, `marker-scoped-guard-needs-a-population-floor`).

**Files:**
- Modify: `skills/docket-implement-next/SKILL.md` (Step 0's `docket-status` dispatch; Step 5's build invocation; Step 6's review invocation and `docket-adr` dispatch)
- Modify: `skills/docket-auto-groom/SKILL.md` (Step 3's critic dispatch)
- Modify: `tests/test_dispatch_capability.sh` (append the consumer-coverage section)
- Modify: `tests/test_skill_size_budgets.sh` (raise the `skills/docket-implement-next/SKILL.md` row + justify)

**Interfaces:**
- Consumes: the tier labels from Task 2 (`A — deterministic`, `B — adversarial`, `C — discipline`) and the section name *Dispatch-capability resolution*.
- Produces: the literal marker `Tier A`, `Tier B`, `Tier C` at each consuming site, each within the same paragraph as that site's dispatch sentence.

- [ ] **Step 1: Write the failing test (append to `tests/test_dispatch_capability.sh`)**

Insert this **before** the final `if [ "$fail" = 0 ]` line:

```bash
# --- producer coverage: every CONSUMING dispatch site names its tier ----------------------------
# Anchored on the consuming skill sections, never an allowlist of tiers (learnings:
# correspondence-guard-runs-one-way). Each row: "<file>|<anchor regex>|<expected tier>". The anchor
# is the site's own dispatch sentence, so a tier marker parked in an unrelated paragraph does not
# satisfy it (learnings: marker-scoped-guard-needs-a-population-floor — attachment, not presence).
IMPL="$REPO/skills/docket-implement-next/SKILL.md"
AUTOGROOM="$REPO/skills/docket-auto-groom/SKILL.md"

# Print the single paragraph (blank-line-delimited block) containing the first anchor match.
para_with(){ awk -v pat="$2" 'BEGIN{RS="";} $0 ~ pat {print; exit}' "$1"; }

seen=0
# NOTE: the tier is expanded into the assert expression at call time. `assert` runs `eval "$2"`,
# so a `$3` left inside that string would resolve to *assert's* third positional parameter (unset
# under `set -u`), not this function's — a real trap, caught while writing this plan.
check_site(){ # $1 file  $2 anchor regex  $3 expected tier  $4 label
  local p tier label; p="$(para_with "$1" "$2")"; tier="$3"; label="$4"
  echo "seen $(basename "$(dirname "$1")")/$(basename "$1") $tier"  # per-site record, before any skip
  seen=$((seen+1))
  assert "$label: dispatch site found" '[ -n "$p" ]'
  assert "$label: names $tier at the dispatch site" "grep -qF -- \"$tier\" <<<\"\$p\""
}

check_site "$IMPL"      "dispatch the .?docket-status.? subagent" "Tier A" "implement-next §0 docket-status"
check_site "$IMPL"      "docket-adr.? subagent"                  "Tier A" "implement-next §6 docket-adr"
check_site "$IMPL"      "resolved build skill"                   "Tier C" "implement-next §5 build"
check_site "$IMPL"      "resolved review skill"                  "Tier C" "implement-next §6 review"
check_site "$AUTOGROOM" "docket-auto-groom-critic"               "Tier B" "auto-groom §3 critic"

# Population floor: the scanner must have REACHED all five sites. A renamed heading or a moved
# paragraph would otherwise silently shrink the guard's scope to nothing and still print PASS.
assert "consumer coverage: all five dispatch sites were scanned (floor)" '[ "$seen" -eq 5 ]'
```

- [ ] **Step 2: Run test to verify it fails**

Run: `"$DOCKET_BASH_PATH" tests/test_dispatch_capability.sh`
Expected: FAIL — the five `seen …` records print, the `dispatch site found` asserts pass, and every `names Tier …` assert is `NOT OK`.

- [ ] **Step 3: Wire `docket-implement-next`**

In `skills/docket-implement-next/SKILL.md`:

**Step 0** — append to the paragraph that ends `— the contract is **git state, not an in-context return**.`:

```markdown
 If no dispatch mechanism resolves per the convention's *Dispatch-capability resolution*, run the same sweep **inline (Tier A)** — an equivalent path, not a degradation.
```

**Step 5** — append to the end of the Step 5 paragraph:

```markdown
 If the resolved skill is invocable but **cannot dispatch** (established only per the convention's *Dispatch-capability resolution* — never from a tool name), this is **Tier C, authorized-or-halt**: an explicitly configured `auto` authorizes the inline path above; any other resolved value is abort-and-report, leaving the change `in-progress` with the halt reason recorded.
```

**Step 6** — append to the end of the Step 6 paragraph, after the `docket-adr` sentence:

```markdown
 The review role is **Tier C** on unavailable dispatch, on the same authorized-or-halt terms as step 5; the `docket-adr` dispatch is **Tier A** and runs inline instead, its git-state contract unchanged.
```

- [ ] **Step 4: Wire `docket-auto-groom`**

In `skills/docket-auto-groom/SKILL.md`, Step 3 — append to the end of the critic-pass paragraph:

```markdown
 If no dispatch mechanism resolves per the convention's *Dispatch-capability resolution*, the critic gate is **Tier B**: the groom **abstains** for that stub (exit 3) rather than self-reviewing — an author cannot be their own adversarial gate.
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `"$DOCKET_BASH_PATH" tests/test_dispatch_capability.sh`
Expected: PASS, with five `seen …` records printed.

- [ ] **Step 6: Raise the implement-next budget in the same diff**

```bash
wc -l skills/docket-implement-next/SKILL.md; wc -w skills/docket-implement-next/SKILL.md
"$DOCKET_BASH_PATH" tests/test_skill_size_budgets.sh
```

Raise the `skills/docket-implement-next/SKILL.md` row to the measured actual rounded up (lines → next 5, words → next 10) and extend the comment-block justification to name change 0137's Tier A/C wiring at the four implement-next dispatch sites. If `skills/docket-auto-groom/SKILL.md` also exceeds its row, raise it the same way; if it still fits (it had real headroom), leave it untouched.

- [ ] **Step 7: Run both tests to verify green**

Run: `"$DOCKET_BASH_PATH" tests/test_skill_size_budgets.sh && "$DOCKET_BASH_PATH" tests/test_dispatch_capability.sh`
Expected: both PASS.

- [ ] **Step 8: Commit**

```bash
git add skills/docket-implement-next/SKILL.md skills/docket-auto-groom/SKILL.md tests/test_dispatch_capability.sh tests/test_skill_size_budgets.sh
git commit -m "feat(0137): name the tier at every consuming dispatch site"
```

---

### Task 4: Correct the two wrong prose sites and guard the shape

Exactly **two** live prose sites are factually wrong; four other mentions are Cursor-scoped and **correct**, and a blanket rename would introduce new errors. The guard must therefore key on **shape**, not a hand-maintained exception list (`AGENTS.md`: never hand-list the sites of a literal you are gating; learnings: `enumerated-floor`).

**The shape, derived from a whole-repo grep rather than assumed:** in live prose (`skills/**/*.md` + `README.md`) every occurrence of the word `Task` is a dispatch-tool reference, and each is legitimate **only when its own line is Cursor-scoped** (Cursor genuinely documents a Task tool). The two wrong sites are wrong *precisely because* they are Claude-Code-scoped (`@docket-status`, `context: fork`) yet name Cursor's tool. That is a mechanical, mutation-testable predicate with no allowlist. `docs/adrs/` is excluded from the line rule — an `Accepted` ADR is immutable, and ADR-0024's wrong sentence is corrected by an appended `## Update` delivered on the `docket` branch (docket's Step 6), not here.

**Files:**
- Modify: `skills/docket-convention/references/agent-layer.md` (the *Both invocation paths land on the same pinned wrapper* paragraph)
- Modify: `README.md` (the **Agent-dispatch** row of the two-invocation-paths table)
- Modify: `tests/test_dispatch_capability.sh` (append the negative shape guard)

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: the negative guard's `seen <path>:<lineno>` records and its population floor.

- [ ] **Step 1: Re-derive the site list from a whole-repo grep**

Do not trust this plan's list — regenerate it, then sort the hits into Cursor-scoped and not:

```bash
grep -rn '\bTask\b' --include='*.md' skills/ README.md
```

Expected: four hits — two whose line also says `Cursor` (correct, leave alone) and two that do not (`agent-layer.md`'s *Both invocation paths* paragraph and `README.md`'s **Agent-dispatch** table row). If the counts differ from that, stop and report the drift rather than adapting the guard to whatever you found.

- [ ] **Step 2: Write the failing test (append to `tests/test_dispatch_capability.sh`)**

Insert this **before** the final `if [ "$fail" = 0 ]` line:

```bash
# --- negative guard: no live prose gates a decision on a literal tool name -----------------------
# Shape, not an allowlist (AGENTS.md: never hand-list the sites of a literal you are gating). In
# live prose every `Task` occurrence is a dispatch-tool reference; it is legitimate only where the
# line is Cursor-scoped, since Cursor documents a Task tool and Claude Code does not. docs/adrs/ is
# out of scope: an Accepted ADR is immutable and is corrected by an appended dated `## Update`.
mentions="$(cd "$REPO" && grep -rn '\bTask\b' --include='*.md' skills/ README.md 2>/dev/null)"
offenders=""; cursor_scoped=0; total=0
while IFS= read -r line; do
  [ -n "$line" ] || continue
  total=$((total+1))
  echo "seen ${line%%:*}:$(cut -d: -f2 <<<"$line")"          # per-hit record, before any skip
  if grep -qi 'cursor' <<<"$line"; then
    cursor_scoped=$((cursor_scoped+1))
  else
    offenders="$offenders
$line"
  fi
done <<EOF
$mentions
EOF

assert "no live prose names a dispatch tool outside a Cursor-scoped line" \
  '[ -z "$(printf %s "$offenders" | tr -d "[:space:]")" ]'
[ -z "$(printf %s "$offenders" | tr -d '[:space:]')" ] || printf 'offending lines:%s\n' "$offenders"
# Population floor: the scan must have reached real content. Zero hits would pass the assert above
# vacuously — a path typo or a moved file must redden, not silently guard nothing.
assert "negative guard: scan reached live prose (floor: >=2 Cursor-scoped mentions)" \
  '[ "$cursor_scoped" -ge 2 ]'
assert "negative guard: scan is non-empty" '[ "$total" -ge 2 ]'

# Positive control: the guard must REPORT a planted violation, whatever the real tree looks like
# (learnings: marker-scoped-guard-needs-a-population-floor — coverage, not population).
ctl="$(mktemp -d)"; trap 'rm -rf "$ctl"' EXIT
mkdir -p "$ctl/skills/x"
printf 'A forked skill-invoke and an explicit agent dispatch (a `Task` naming the wrapper) are one.\n' \
  > "$ctl/skills/x/SKILL.md"
: > "$ctl/README.md"
ctl_hits="$(cd "$ctl" && grep -rn '\bTask\b' --include='*.md' skills/ README.md 2>/dev/null | grep -vi cursor)"
assert "negative guard: positive control — a planted non-Cursor Task line IS detected" \
  '[ -n "$ctl_hits" ]'
```

- [ ] **Step 3: Run test to verify it fails**

Run: `"$DOCKET_BASH_PATH" tests/test_dispatch_capability.sh`
Expected: FAIL — `no live prose names a dispatch tool outside a Cursor-scoped line` is `NOT OK`, and the two offending lines print. The floor and positive-control asserts already pass.

- [ ] **Step 4: Correct the `agent-layer.md` site**

In `skills/docket-convention/references/agent-layer.md`, in the paragraph beginning **Both invocation paths land on the same pinned wrapper**, replace:

```
and an explicit agent dispatch (`@docket-status`, or a `Task` naming the wrapper) resolve to the
```

with:

```
and an explicit agent dispatch (`@docket-status`, or a dispatch naming the wrapper) resolve to the
```

- [ ] **Step 5: Correct the `README.md` site**

In `README.md`, in the two-invocation-paths table, replace the **Agent-dispatch** row's How cell:

```
| **Agent-dispatch** | `@docket-status`, or a `Task` dispatch naming the wrapper | The **identical** pinned run, drillable live in the TUI | One dispatch turn of overhead |
```

with:

```
| **Agent-dispatch** | `@docket-status`, or a subagent dispatch naming the wrapper | The **identical** pinned run, drillable live in the TUI | One dispatch turn of overhead |
```

Leave the following paragraph's `Cursor users are always on the drillable path: the generated dispatch rule routes a direct invocation through a real `Task` dispatch.` **untouched** — it is Cursor-scoped and correct.

- [ ] **Step 6: Run the test to verify it passes**

Run: `"$DOCKET_BASH_PATH" tests/test_dispatch_capability.sh`
Expected: PASS. The `seen …` records now show two hits, both Cursor-scoped.

- [ ] **Step 7: Verify the pre-existing doc sentinels still hold**

These two files are covered by an existing guard whose asserts anchor on neighbouring clauses:

Run: `"$DOCKET_BASH_PATH" tests/test_skill_fork_dispatch.sh`
Expected: PASS — in particular `README names both invocation paths into the pinned wrapper` and `agent-layer reference states both paths land on the same pinned wrapper`, neither of which mentions `Task`.

- [ ] **Step 8: Commit**

```bash
git add skills/docket-convention/references/agent-layer.md README.md tests/test_dispatch_capability.sh
git commit -m "fix(0137): state the dispatch path by capability, not a Claude-Code tool name"
```

---

### Task 5: Mutation-test the new guards, run the whole suite, finish the results file

**Files:**
- Modify: `docs/results/2026-07-25-dispatch-capability-detection-results.md`

**Interfaces:**
- Consumes: everything above.
- Produces: the completed results file. docket's Step 6.5 sets the `results:` field on the metadata branch — not here.

- [ ] **Step 1: Mutation-test each new guard (a guard is code — `AGENTS.md`)**

For each mutation: apply it, run `"$DOCKET_BASH_PATH" tests/test_dispatch_capability.sh`, confirm it **reddens**, then revert with `git checkout -- <file>`. Record the observed result for each.

1. Delete the `absence of a specifically-named tool never` sentence from the convention → the name-gate assert reddens.
2. Delete the whole `Tier C` table row → the tier + authorized-or-halt asserts redden.
3. Delete the *missing-skill rule* mention from the Skill layer → the boundary assert reddens.
4. Remove `Tier A` from implement-next's Step 0 dispatch paragraph only → that one site's assert reddens, the other four stay green (per-site isolation).
5. Move `Tier B` in auto-groom out of the critic paragraph into the Step 4 exit list → the attachment assert reddens (presence alone must not satisfy it).
6. Rename implement-next's `resolved build skill` phrase → the `dispatch site found` assert reddens **and** the population floor (`seen -eq 5`) reddens.
7. Re-introduce `` a `Task` naming the wrapper `` into `agent-layer.md` → the negative guard reddens and prints the offending line.
8. Point the negative guard's grep at a non-existent directory → the floor asserts redden (proving the scan is not vacuous).

Any mutation that leaves the suite green is a defect in the guard — fix the guard before proceeding.

- [ ] **Step 2: Run the whole suite**

`AGENTS.md`: run the whole suite at the build gate, never only the tests the spec enumerated. Run it as **one foreground call** — never backgrounded:

```bash
for t in tests/test_*.sh; do
  out="$("$DOCKET_BASH_PATH" "$t" 2>&1)" || true
  printf '%s %s\n' "$(printf '%s' "$out" | tail -n1)" "$t"
done
```

Expected: every line begins `PASS`. Investigate any `FAIL` before continuing; a pre-existing failure must be confirmed against the unmodified base (learnings: `environment`) and reported, not absorbed.

- [ ] **Step 3: Complete the results file**

Append to `docs/results/2026-07-25-dispatch-capability-detection-results.md`:

- `## Guard mutation matrix` — the eight mutations from Step 1, each with the assert that reddened.
- `## Suite` — the whole-suite result and its test count.
- `## Deviations from the spec` — at minimum: the negative guard is **shape-scoped** (a Cursor-scoped-line predicate) rather than the spec's *"exclude the four Cursor-scoped mentions"* **allowlist**, because `AGENTS.md` forbids hand-listing the sites of a gated literal and an allowlist ages into the gap it was written to close (learnings: `enumerated-floor`). Same coverage, no maintenance list. Also record the budget-row raises with old → new numbers.
- `## Follow-ups` — the `skill-fallback-degrades-discipline` learning still records #0136's cause as *"the run's runtime exposed no subagent-dispatch (Task) tool at all"*, now known to be a likely false negative. The ledger's only writer is the close-out harvest, so the correction rides finalize — flag it here so the harvest does not miss it.
- `## Manual checks for the merge gate` — anything the human should verify by hand, or "none".

- [ ] **Step 4: Verify no metadata leaked onto the feature branch**

```bash
git diff --stat origin/main...HEAD
```

Expected: only `docs/superpowers/plans/`, `docs/results/`, `skills/`, `tests/`, and `README.md` paths. **Zero** entries under `docs/changes/` or `docs/adrs/` — those live on the `docket` branch (feature-branch invariant).

- [ ] **Step 5: Commit**

```bash
git add docs/results/2026-07-25-dispatch-capability-detection-results.md
git commit -m "docs(0137): results — spike findings, mutation matrix, deviations"
```
