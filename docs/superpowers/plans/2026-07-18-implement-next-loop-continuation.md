# Loop continuation — implement-next re-invocation contract — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `docket-implement-next` a driver-agnostic re-invocation contract — every run declares one of four terminal dispositions and selection accepts an id allowlist — plus README documentation naming `/loop` as the recommended drain driver.

**Architecture:** Prose-only change. Two additions to `skills/docket-implement-next/SKILL.md` (a four-disposition terminal contract with per-step-exit mappings, and id-set scoping in Step 1), one README documentation subsection, one new sentinel test, and one raised size-budget row. **No shell scripts change.** The change is self-referential (it edits the very skill running the build); edits land on the feature branch's repo source, so the running installed copy is untouched.

**Tech Stack:** Markdown (skill prose + README); Bash sentinel test in the repo's standalone `tests/test_*.sh` style (no runner — each test is `bash tests/<file>.sh`).

## Global Constraints

- **No scripts change.** Only `skills/docket-implement-next/SKILL.md`, `README.md`, `tests/test_skill_size_budgets.sh`, and the new `tests/test_loop_continuation.sh` are touched.
- **Four disposition tokens, verbatim:** `advanced`, `contended`, `drained`, `halted`. Driver rule: **continue on `advanced`/`contended`, stop on `drained`/`halted`.**
- **Disposition→exit mapping (from spec §3.1):** Step 7 PR opened → `advanced`; Step 2 lost claim CAS → `contended`; Step 1 empty queue → `drained`; Step 3 fundamentally-invalidated / hard error → `halted`.
- **Id-set scoping is an allowlist, never a dependency override** (spec §3.2). Unset ⇒ whole build-ready backlog (byte-identical to today). A scoped non-build-ready member is skipped with its reason, never force-built, never aborts the run.
- **Size-budget guard (change 0085):** `tests/test_skill_size_budgets.sh` caps `skills/docket-implement-next/SKILL.md` at 119 lines / 2451 words. Growth requires RAISING that row **in the same diff** (the guard permits an in-diff raise). Set the new caps to the post-edit actuals + ~10%.
- **`/loop` is recommended, not verified-supported** (spec §6 degrade + learning `harness-behavior-is-mode-and-version-scoped`): the docs frame `/loop` as the recommended driver and tell the reader to confirm it composes in their own harness. The live harness spike is deferred to a follow-up; the disposition contract stands regardless of driver.
- Sentinels are sampling, not parsing (learning `foundational-test-discipline`) — the new test pairs with the whole-branch review, it does not replace it.

---

### Task 1: Terminal disposition contract + id-set scoping in SKILL.md

**Files:**
- Create: `tests/test_loop_continuation.sh` (SKILL.md asserts only in this task; README asserts added in Task 2)
- Modify: `skills/docket-implement-next/SKILL.md` (Step 1, Step 2, Step 3, and a new `### Terminal disposition (driver contract)` subsection)
- Modify: `tests/test_skill_size_budgets.sh` (raise the `docket-implement-next/SKILL.md` budget row)

**Interfaces:**
- Produces: the four disposition tokens and the `### Terminal disposition (driver contract)` heading in SKILL.md that Task 2's README doc cross-references; the `tests/test_loop_continuation.sh` file that Task 2 extends.

- [ ] **Step 1: Write the failing sentinel test (SKILL.md portion)**

Create `tests/test_loop_continuation.sh` with exactly:

```bash
#!/usr/bin/env bash
# tests/test_loop_continuation.sh — guards change 0088 (loop continuation: docket-implement-next as
# a driver-agnostic re-invocation contract). Asserts the four-disposition terminal contract, the
# per-step-exit mappings, id-set scoping (SKILL.md), and the README /loop drain-pattern doc.
# Sentinels are sampling, not parsing (learnings: foundational-test-discipline) — pair with the
# whole-branch review; this test does not replace it.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

IMPL="$REPO/skills/docket-implement-next/SKILL.md"

# --- SKILL.md: the four-disposition terminal contract ---
assert "SKILL has a Terminal disposition section" 'grep -Eqi "Terminal disposition" "$IMPL"'
for d in advanced contended drained halted; do
  tok="\`$d\`"
  assert "SKILL names disposition $d (code-formatted)" 'grep -qF "$tok" "$IMPL"'
done
# The binary driver rule — both halves must be present (non-vacuous).
assert "SKILL states continue-on advanced/contended" 'grep -Eqi "continue on .{0,4}advanced" "$IMPL"'
assert "SKILL states stop-on drained/halted" 'grep -Eqi "stop on .{0,4}drained" "$IMPL"'
# Skipped-with-reasons enumeration.
assert "SKILL enumerates skipped-with-reason" 'grep -Eqi "skipped with (its|the) reason" "$IMPL"'

# --- SKILL.md: per-step-exit mappings ---
assert "SKILL ties a lost claim race to contended (Step 2)" 'grep -Eqi "claim (CAS|race)" "$IMPL"'
assert "SKILL ties the empty queue to drained (Step 1)" 'grep -Eqi "empty queue|no candidate|nothing .{0,20}build-ready" "$IMPL"'

# --- SKILL.md: id-set scoping ---
assert "SKILL documents an id allowlist" 'grep -Eqi "allowlist" "$IMPL"'
assert "SKILL shows the comma-separated id-set form" 'grep -Eq "docket-implement-next 90,92,94" "$IMPL"'
assert "SKILL states the allowlist is not a dependency override" 'grep -Eqi "never a dependency override" "$IMPL"'

# --- Non-vacuity / mutation proof: the code-formatted disposition grep actually bites. ---
probe="$(mktemp)"; printf 'plain advanced word, no code formatting\n' > "$probe"
assert "the code-formatted disposition grep is non-vacuous" '! grep -qF "\`advanced\`" "$probe"'
rm -f "$probe"

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
```

- [ ] **Step 2: Run the test — verify it fails**

Run: `bash tests/test_loop_continuation.sh`
Expected: FAIL — several `NOT OK` lines (no Terminal disposition section, no code-formatted tokens, no allowlist prose) and final `FAIL`, exit 1.

- [ ] **Step 3: Edit SKILL.md Step 1 — id-set scoping + the `drained` empty-queue exit**

In `skills/docket-implement-next/SKILL.md`, replace the Step 1 body paragraph:

```
Among `active/` changes, select per the convention's **Build-readiness & selection** definition: build-ready `proposed` changes only, ranked by its deterministic order — whose final tie-break is LOWEST `id`, so two implementers (if ever run concurrently) converge on the same winner and never claim the same change. Pick the top, or accept an explicit id passed by the caller. Skip `in-progress`, `blocked`, `deferred`, and not-build-ready stubs.
```

with:

```
Among `active/` changes, select per the convention's **Build-readiness & selection** definition: build-ready `proposed` changes only, ranked by its deterministic order — whose final tie-break is LOWEST `id`, so two implementers (if ever run concurrently) converge on the same winner and never claim the same change. Skip `in-progress`, `blocked`, `deferred`, and not-build-ready stubs.

**Scope (id allowlist).** With no argument the candidate set is the whole build-ready backlog (byte-identical to today). A caller may pass an **id allowlist** — `docket-implement-next 90,92,94` (a single id `90` is the degenerate case) — and selection is then **restricted to that set**, with the same deterministic order applied *within* it. The allowlist is a filter, **never a dependency override**: a scoped id that is not currently build-ready+claimable — needs-brainstorm, already `in-progress`, or waiting on an unmerged `depends_on` — is **skipped with its reason**, never force-built, and never aborts the run.

**Empty queue → `drained`.** If no candidate in scope is build-ready+claimable, build nothing and end the run with the **`drained`** disposition (see *Terminal disposition*) — the driver's stop signal.
```

- [ ] **Step 4: Edit SKILL.md Step 2 — classify the lost claim race as `contended`**

In Step 2, replace the sentence:

```
The arbiter is the re-read (abort if no longer `proposed`), not that any single push succeeds. No worktree yet.
```

with:

```
The arbiter is the re-read (abort if no longer `proposed`), not that any single push succeeds — and that abort is the **`contended`** disposition (see *Terminal disposition*): a lost claim CAS race is a normal, continue-able outcome a driver re-selects past, **never `halted`**. No worktree yet.
```

- [ ] **Step 5: Edit SKILL.md Step 3 — classify the fundamentally-invalidated stop as `halted`**

In Step 3, replace the second escape-hatch bullet:

```
- Design **FUNDAMENTALLY invalidated** (not just scope-adjustable) → STOP and escalate to the human. This skill cannot re-brainstorm alone; re-brainstorming is a human act handled by `superpowers:brainstorming` + `docket-new-change`.
```

with:

```
- Design **FUNDAMENTALLY invalidated** (not just scope-adjustable) → STOP and escalate to the human — end the run with the **`halted`** disposition (see *Terminal disposition*), the driver's stop-and-surface signal. Any hard error that prevents reaching a PR is likewise `halted`. This skill cannot re-brainstorm alone; re-brainstorming is a human act handled by `superpowers:brainstorming` + `docket-new-change`.
```

- [ ] **Step 6: Edit SKILL.md — add the `### Terminal disposition (driver contract)` subsection**

Insert this new subsection **immediately after** Step 7's closing line (`**STOP.** The change stays in `active/` as `implemented` until a human merges it, or approves `docket-finalize-change` to merge it.`) and **before** `### Best-effort board refresh`:

```
### Terminal disposition (driver contract)

Every run ends by declaring exactly **one** of four dispositions, so any driver — a human re-typing the command, the built-in `/loop`, a cron/scheduled agent, or #0008's fan-out — keys on the outcome instead of parsing prose:

| Disposition | Meaning | Driver action |
|---|---|---|
| `advanced` | Built a change → PR opened (Step 7 reached). | continue |
| `contended` | Selected a change but lost the claim CAS (Step 2); **nothing built**. | continue — re-select next |
| `drained` | No build-ready+claimable change in scope (Step 1's empty queue). | **stop** |
| `halted` | Stopped needing a human — fundamentally-invalidated design (Step 3) or a hard error. | **stop + surface** |

The driver's decision is binary: **continue on `advanced`/`contended`, stop on `drained`/`halted`.** The contract is **driver-agnostic** — it names run outcomes, not any one driver's mechanics; `/loop` is *recommended*, not required (see the README drain-pattern doc).

The final report **enumerates** what happened: the change built (if any), each change **skipped with its reason** (needs-brainstorm / already `in-progress` / waiting on an unmerged `depends_on` / outside the id allowlist), and which disposition ended the run.
```

- [ ] **Step 7: Run the sentinel test — verify it passes**

Run: `bash tests/test_loop_continuation.sh`
Expected: all `ok - ` lines, final `PASS`, exit 0.

- [ ] **Step 8: Raise the size-budget row (measure, then set)**

Measure the edited file:

Run: `wc -l -w skills/docket-implement-next/SKILL.md`
Then in `tests/test_skill_size_budgets.sh`, edit the row:

```
skills/docket-implement-next/SKILL.md                      119 2451
```

to the **measured actuals + ~10% (ceil)** — e.g. if `wc` reports `132 2430`, set `145 2673`. Use the real measured numbers, not this example.

- [ ] **Step 9: Run the size-budget test — verify it passes**

Run: `bash tests/test_skill_size_budgets.sh`
Expected: `skills/docket-implement-next/SKILL.md within line budget` and `within word budget` both `ok`; final `PASS`, exit 0.

- [ ] **Step 10: Commit**

```bash
git add skills/docket-implement-next/SKILL.md tests/test_loop_continuation.sh tests/test_skill_size_budgets.sh
git commit -m "feat(docket-implement-next): four-disposition contract + id-set scoping (0088)"
```

---

### Task 2: `/loop` drain-pattern documentation in README.md

**Files:**
- Modify: `README.md` (new `### Draining hands-free with \`/loop\`` subsection after the "Quickstart: the daily loop" section)
- Modify: `tests/test_loop_continuation.sh` (append the README asserts)

**Interfaces:**
- Consumes: the four disposition tokens and the driver rule established in Task 1's SKILL.md.

- [ ] **Step 1: Append the README asserts to the failing test**

In `tests/test_loop_continuation.sh`, insert **before** the `# --- Non-vacuity` block:

```bash
README="$REPO/README.md"

# --- README: the /loop drain-pattern doc ---
assert "README documents the /loop whole-backlog drain" 'grep -Eq "/loop docket-implement-next$|/loop docket-implement-next[^0-9]" "$README"'
assert "README documents the /loop id-set drain" 'grep -Eq "/loop docket-implement-next 90,92,94" "$README"'
assert "README states the driver never merges" 'grep -Eqi "never merges" "$README"'
assert "README names all four dispositions" 'for d in advanced contended drained halted; do grep -qiF "$d" "$README" || exit 1; done'
```

- [ ] **Step 2: Run the test — verify the new README asserts fail**

Run: `bash tests/test_loop_continuation.sh`
Expected: FAIL — the four README asserts show `NOT OK` (SKILL.md asserts still `ok`), final `FAIL`, exit 1.

- [ ] **Step 3: Add the README subsection**

In `README.md`, insert this subsection **immediately after** the line `In short: **you** create and merge; **docket** grooms, implements, and closes out. \`docket-status\` keeps the board honest in between.` and **before** the following `---`:

```

### Draining hands-free with `/loop`

`docket-implement-next` ends every run by declaring one of four **dispositions** — `advanced` (built a change → PR), `contended` (lost a claim race, nothing built), `drained` (nothing build-ready in scope), or `halted` (needs a human). A driver keys on these: **continue on `advanced`/`contended`, stop on `drained`/`halted`.** The contract is driver-agnostic — a human re-typing the command works as well as any loop runner.

The recommended driver is the built-in **`/loop`**, which forks a fresh implementer each iteration so the heavy build stays in the fork and the loop context stays small:

- `/loop docket-implement-next` — self-paced; drains the whole build-ready backlog, stopping on `drained`.
- `/loop docket-implement-next 90,92,94` — drains only that id set (deterministic order within it); a scoped change that is not build-ready — needs-brainstorm, already in progress, or waiting on an unmerged dependency — is skipped with its reason.

Budget and iteration caps are `/loop`'s own mechanism; docket does not reimplement them. The driver **never merges** — the human merge gate is untouched, so a dependency only clears between drains via a human merge; a scoped change waiting on an unmerged dependency is skipped this drain, not waited on. Confirm `/loop` composes cleanly with the forked skill in your own harness before relying on it unattended — harness behavior is version- and mode-scoped.
```

- [ ] **Step 4: Run the test — verify it passes**

Run: `bash tests/test_loop_continuation.sh`
Expected: all `ok - `, final `PASS`, exit 0.

- [ ] **Step 5: Commit**

```bash
git add README.md tests/test_loop_continuation.sh
git commit -m "docs(docket): /loop drain-pattern for the implement-next disposition contract (0088)"
```

---

### Task 3: Whole-suite verification

**Files:** none modified — verification only.

- [ ] **Step 1: Run the full test suite**

Run:
```bash
fail=0; for t in tests/test_*.sh; do bash "$t" >"/tmp/$(basename "$t").out" 2>&1 || { echo "FAIL: $t"; tail -5 "/tmp/$(basename "$t").out"; fail=1; }; done; echo "suite fail=$fail"
```
Expected: `suite fail=0` (every test green — in particular `test_loop_continuation.sh` and `test_skill_size_budgets.sh`).

- [ ] **Step 2: If any test is red, fix and re-run**

Root-cause per `superpowers:systematic-debugging`; a red `test_skill_size_budgets.sh` means the budget row wasn't raised to the measured actuals (redo Task 1 Step 8). Re-run Step 1 until `suite fail=0`.

## Self-Review notes

- **Spec §3.1 (disposition report):** Task 1 Steps 3–6 (the four tokens, the table, the binary rule, the enumeration) + the per-step-exit mappings in Steps 3/4/5.
- **Spec §3.2 (id-set scoping):** Task 1 Step 3 (allowlist, comma form, within-set order, skip-with-reason, not-a-dependency-override).
- **Spec §3.3 (/loop doc):** Task 2 Step 3 (both `/loop` forms, never-merges, budget-caps-are-/loop's, self-paced stop-on-drained).
- **Spec §6 (degrade):** the README doc frames `/loop` as recommended and tells the reader to confirm composition in their harness; the live spike is a deferred follow-up (recorded in the results file at Step 6.5, not in the plan tasks).
- **Spec §7 (testing):** `tests/test_loop_continuation.sh` asserts the disposition mapping and id-set vocabulary; the size-budget guard rides along; the whole-branch review carries the rest.
