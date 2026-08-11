<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0275 — Run gate has no runnable path for slash-command or backgrounded implement-next dispatch](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0275-run-gate-has-no-runnable-path-for-slash-command-or-backgroun.md)**
<!-- docket:backlink:end -->

# Run gate — a runnable path for detached implement-next dispatch

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give the caller-side run gate a runnable verification path for a dispatch the session did not foreground-block on — a backgrounded agent it launched itself, or a slash-command run it first learns of from a completion notification.

**Architecture:** Prose-only. `cursor-rules/run-gate.md` is the single authored source of the gate text; `sync-agents.sh`'s `assemble_run_gate()` splices it verbatim into the Cursor rule and the committed `AGENTS.md` dispatch block (`CLAUDE.md` is the same physical file). This change edits that one template — reframing the ordering obligation, scoping step 2's backgrounding prohibition, and appending a `### Detached dispatch` section — then moves the guard suite and the committed `AGENTS.md` block with it. No script changes: `verify-run.sh` already exposes `--in-progress-ids --with-claimed-at` and `--iso-to-epoch`, and `runner-dispatch.sh` already implements the three-filter attribution this section ports into prose.

**Tech Stack:** Bash (the `tests/*.sh` assert-function house style), markdown templates, `sync-agents.sh` generator.

## Global Constraints

Copied verbatim from `AGENTS.md`, the spec, and the change's reconcile log. Every task's requirements implicitly include this section.

- **Never `producer | early-exiting-consumer`** (`grep -q`, `head`, `head -n1`) under `set -o pipefail` — capture into a variable, then `grep <<<"$var"`. `tests/test_sync_agents_run_gate.sh` runs `set -uo pipefail`.
- **A `grep` pattern that leads with `--` must declare it**: `grep -E -e "<pat>"` or `grep -qF -- "<pat>"`. A bare leading `--` parses as an option (exit 2), and inside a negated assert that error inverts into a permanently green, vacuous guard.
- **A guard is code: mutation-test it** — strip the thing it guards, watch it redden — or it is decoration. Every mutation in this plan uses a `cp` backup (`cp "$f" "$f.bak"` … `mv "$f.bak" "$f"`), **never** `git checkout -- <file>`, which restores to HEAD and destroys the uncommitted edit under test (learning: `mutation-restore-needs-a-backup-copy`).
- **Prove every mutation landed before interpreting its result** — `grep -c` before and after. A `sed`/`perl` substitution that fails to match exits 0 having changed nothing, and a green run then reads as "the guard is vacuous" when no mutation was ever applied.
- **Write the assert that DETECTS the state you removed**, not one that confirms the wording you just introduced (learning: `assert-detects-removal-not-replacement`).
- **Bind a prose phrase to the claim it is asserted about**, with a bounded window — never a bare phrase-presence grep over the whole file (learning: `prose-guard-binds-phrase-to-claim`). A window needs a **named terminator** and a non-vacuity assert that the window is not the whole file (learning: `section-slice-needs-a-named-terminator`).
- **Mutation-test each conjunct of a widened clause separately** — delete only the added part and confirm something reddens (learning: `guard-the-widened-clause`).
- **No apostrophe inside an assert pattern.** `assert(){ … eval "$2" … }` receives its expression as a **single-quoted** string; an apostrophe in the pattern terminates that string. Word template prose so the asserted substrings contain none.
- **Cross-references anchor on a symbol name or a verbatim-quoted clause, never a line number.** `tests/test_comment_anchor_style.sh` rejects the filename-plus-line-number form.
- **The gate is native-dispatch-only.** Its commands are only `docket.sh preflight` and `docket.sh verify-run`; its re-dispatch is a harness-native named-agent dispatch whose retry context rides the dispatch prompt. Change 0277 moved *delegated* briefs onto a `--brief-file` channel for `runner-dispatch.sh` and adapters refuse brief-file-plus-argv — none of that reaches this gate, and no prose added here may be written as a facade invocation with trailing argv.
- **No observation loop.** The detached path regains control exactly once, at the notification. It must not grow a poll loop, so change 0286's taught `gate-run --observe` loop shape is not imported here.
- **`cursor-rules/dispatch.head.md` is NOT edited.** Its item 2 carries the same "never background it and never poll" sentence and, on the Cursor surface, is spliced above the gate — but its directive governs every docket agent dispatch, not just implement-next runs. The Detached section instead states its own precondition in its heading, so the gate is self-consistent on both surfaces from its own text.
- **Suite command:** `scripts/run-tests.sh` (the resolved `finalize.test_command`). A trailing `OVER BUDGET:` line for a file this change touched is a finding to act on, not noise.

---

## File Structure

| File | Responsibility |
|---|---|
| `cursor-rules/run-gate.md` | **Modify.** The single authored source of the gate text. Grows the reframed header, the scoped step 2, and the `### Detached dispatch` section. 25 lines → 43. |
| `tests/test_sync_agents_run_gate.sh` | **Modify.** Raises the brevity bound with recorded rationale, re-points the two step-2 asserts at the scoped wording, and adds the detached-section guard block. |
| `AGENTS.md` | **Modify (generated).** The committed `docket:dispatch` managed block, regenerated so its embedded gate matches the template. `CLAUDE.md` is a symlink to this same physical file — one regeneration serves both. |
| `tests/runtime-budgets.tsv` | **Read, and modify only if the measurement demands it.** Row `tests/test_sync_agents_run_gate.sh 10 parallel`. |
| `docs/results/2026-08-11-run-gate-detached-dispatch-path-results.md` | **Create (Task 3).** Records the deliberate bound raise, the mutation evidence, and the budget margin as a number. |

---

### Task 1: Amend the gate template, and move its guard and the committed block with it

The template edit, the guard changes, and the `AGENTS.md` regeneration are **one commit**. They cannot be split: growing the template past 25 lines reddens the brevity assert, changing step 2's wording reddens the two step-2 asserts, and changing the template at all reddens the currency assert. Any intermediate commit is a red suite.

**Files:**
- Modify: `cursor-rules/run-gate.md`
- Modify: `tests/test_sync_agents_run_gate.sh`
- Modify: `AGENTS.md` (generated — never hand-edited)
- Test: `tests/test_sync_agents_run_gate.sh`

**Interfaces:**
- Consumes: `sync-agents.sh`'s `assemble_run_gate()` (`cat "$CURSOR_RULES_SRC/run-gate.md"`), `assemble_agents_md_dispatch`, `_docket_gi_current_block`, and the `DISPATCH_START` / `DISPATCH_END` marker variables — all reached by sourcing `sync-agents.sh` in a subshell, exactly as the test's existing currency leg does.
- Produces: a 43-line `cursor-rules/run-gate.md`; a `GATE_LINES` bound of `43` that the two verbatim-slice checks and the `slice_gate` awk reader both key off automatically.

- [ ] **Step 1: Read the three files you are about to change, end to end**

Read `cursor-rules/run-gate.md` (25 lines), `tests/test_sync_agents_run_gate.sh` (226 lines), and — with `grep -n "docket:dispatch" AGENTS.md` — locate the managed block in `AGENTS.md`. Do not edit `AGENTS.md` by hand at any point; it is regenerated in Step 8.

Note the three existing window slicers you will reuse the shape of:

```bash
STEP2="${G#*verify-run --in-progress-ids}"; STEP2="${STEP2%%After the return*}"
STEP3="${G#*After the return}"; STEP3="${STEP3%%report line*}"
```

`flat()` collapses every whitespace run to one space before matching, so an asserted substring may span a line break in the template.

- [ ] **Step 2: Write the failing guards — the raised bound**

In `tests/test_sync_agents_run_gate.sh`, replace the brevity block (the comment history plus the `gate text is at most 25 lines` assert) with:

```bash
# --- brevity: the block rides always-loaded context in every harness (spec Risks) ---
GATE_LINES="$(grep -c "" "$GATE_SRC" 2>/dev/null || echo 0)"
# Bound raised 14 -> 18 (change 0242 review, finding 1): every command must carry the convention's
# mandatory facade prefix, since `docket.sh` is on no PATH. Correctness over the plan-time guess.
# Raised 18 -> 23 (finding 3): the symmetric metadata re-sync — two `preflight` commands at full
# facade spelling, plus the sentence saying why both reads must be fresh.
# Raised 23 -> 25 (finding 4): the multi-candidate abort clause in step 3 — the cardinality rule
# scripts/runner-dispatch.sh enforces ("this run claims at most one") had no prose counterpart.
# Raised 25 -> 43 (change 0275): the gate had NO runnable path for a dispatch the session did not
# foreground-block on — the shape a human actually launches (`/docket-implement-next <id>`), where
# steps 1-3 are structurally unrunnable because no before-snapshot exists. The `### Detached
# dispatch` section is that path, and it cannot be shorter than the two branches it must keep
# apart: one that CAN attribute (before-set + DISPATCH_EPOCH, all three filters, re-dispatch
# allowed) and one that CANNOT (verify-and-report, never re-dispatch). Collapsing them is the
# defect — an unattributed re-dispatch lands on a change a live agent is holding. The raise is
# deliberate and priced: an always-loaded block earns its length only by being runnable, and 18
# lines is what the observed-and-ungated dispatch shape costs.
assert "gate text is at most 43 lines" '[ "$GATE_LINES" -ge 1 ] && [ "$GATE_LINES" -le 43 ]'
```

- [ ] **Step 3: Write the failing guards — the reframed ordering obligation**

The header window's named terminator is step 1's opener, `Before dispatching`. Insert this immediately after the existing `G="$(flat "$GATE_SRC" 2>/dev/null)"` line:

```bash
# The gate's framing must survive notification-driven control flow: a session HANDED a completion
# report never "reports" in the sense the old heading meant, so "verify before you report" bound
# nothing on the very path change 0275 exists to cover. Bound to the HEADER window (everything
# before step 1's opener), not to $G — the words recur in the steps below.
HEADW="${G%%Before dispatching*}"
assert "gate: the header window exists to be asserted about" \
  '[ -n "$HEADW" ] && [ "$HEADW" != "$G" ]'
assert "gate: the obligation is stated against RELAYING, not against reporting" \
  '[[ "$HEADW" == *"before you relay it"* ]]'
assert "gate: a completion notification is named as the CHILD claim, not the session output" \
  '[[ "$HEADW" == *"the CHILD"*"claim, not your report"*"before relaying"* ]]'
```

- [ ] **Step 4: Write the failing guards — step 2 scoped, and its routing arm**

Replace the two existing step-2 behavioral asserts (`gate: step 2 carries its own foreground/blocking claim` and `gate: step 2 forbids backgrounding and polling`) with the three below. Keep the long explanatory comment above them intact, and append the scoping note. The non-vacuity assert `gate: step 2 exists to be asserted about` stays exactly as it is.

```bash
# Change 0275: the prohibition is SCOPED rather than dropped. A blanket "never background it" on
# the same page as the Detached section countermands it, and the prohibition is mutation-tested —
# so it now binds the path it can bind (a dispatch this session issues and can block on) and
# routes every other shape to the named section instead of forbidding it.
assert "gate: step 2 scopes foreground-and-block to a dispatch this session can block on" \
  '[[ "$STEP2" == *"can block on it"*"foreground"*"block on the return"* ]]'
assert "gate: step 2 forbids backgrounding and polling on that path" \
  '[[ "$STEP2" == *"never background it"*"never poll"* ]]'
assert "gate: step 2 routes a dispatch it did not block on to the detached section" \
  '[[ "$STEP2" == *"harness"*"backgrounds for you"*"Detached dispatch"* ]]'
```

- [ ] **Step 5: Write the failing guards — the detached section**

Append this block immediately after the existing `gate: run-incomplete re-dispatches exactly ONCE, then stops` assert, before the `--- rendered into BOTH surfaces ---` banner:

```bash
# --- the detached path: the dispatch shape the gate had no runnable procedure for (change 0275) --
# Bound to the DETACHED window, never to $G. Every phrase below also has a legitimate home in
# steps 1-4 — "verify-run", "stop and report", "never re-dispatch" — so a whole-file window is
# satisfied with the entire section deleted (mutation-proven vacuous). The section is last in the
# template, so the window runs to EOF and its opening heading is the named terminator; the
# non-vacuity assert below is what proves the heading was found at all.
DETACHED="${G#*### Detached dispatch}"
assert "gate: the detached section exists to be asserted about" \
  '[ -n "$DETACHED" ] && [ "$DETACHED" != "$G" ]'
# The section must announce WHICH dispatches it governs. Without this it reads as an alternative
# the caller may choose, and a caller who can foreground-block would be free to pick the weaker
# path; it is a fallback for a shape the caller does not control, not an option.
assert "gate: the detached section is scoped by whether the session foreground-blocked" \
  '[[ "$DETACHED" == *"you did not foreground-block"* ]]'
# Branch A — attributable. All three filters, each asserted as its OWN conjunct: the epoch filter
# was added at the design gate on top of the set difference, and a guard that pins only the pair
# leaves the added part free (learnings: guard-the-widened-clause).
assert "gate: detached branch A captures BOTH the before-snapshot and a dispatch epoch" \
  '[[ "$DETACHED" == *"before-snapshot"*"date -u +%s"*"DISPATCH_EPOCH"* ]]'
assert "gate: detached branch A reads the claim instant from the oracle, not from prose" \
  '[[ "$DETACHED" == *"verify-run --in-progress-ids --with-claimed-at"* ]]'
assert "gate: detached branch A requires all three filters, named" \
  '[[ "$DETACHED" == *"ALL THREE filters"*"absent from the before-set"*"parses"*"DISPATCH_EPOCH"* ]]'
assert "gate: detached branch A keeps step 3 cardinality — two or more candidates abort" \
  '[[ "$DETACHED" == *"one survivor"*"step 4"* ]] && [[ "$DETACHED" == *"two or more"*"stop and report"* ]]'
# Branch B — unattributable. The never-re-dispatch rule AND the reason it holds: without the
# reason a later editor reads it as caution and relaxes it, and the failure it prevents is the one
# unrecoverable move in the whole gate.
assert "gate: detached branch B is named as unattributed mode" \
  '[[ "$DETACHED" == *"unattributed mode"*"No before-set exists"* ]]'
assert "gate: unattributed mode never re-dispatches, and says why a timestamp cannot attribute" \
  '[[ "$DETACHED" == *"re-stamped at every phase boundary"*"looks fresh"*"Never re-dispatch"* ]]'
assert "gate: a prose id from the child is a hint to verify, never attribution authority" \
  '[[ "$DETACHED" == *"the notification names"*"never authority"* ]]'
```

- [ ] **Step 6: Run the test to verify it fails**

Run: `bash tests/test_sync_agents_run_gate.sh`

Expected: **FAIL**, with `NOT OK` on every assert added in Steps 3–5 (the header, the scoped step 2, and all nine detached asserts) — the template still carries none of that text. The brevity assert and the two currency asserts stay green (`43` is a weaker bound than `25`, and the template has not moved yet). This is the reading that proves the guards detect the absent state rather than confirming text you already wrote.

- [ ] **Step 7: Rewrite the template**

Replace the **entire contents** of `cursor-rules/run-gate.md` with exactly this. It is 43 lines; do not reflow it, because the brevity bound is exact.

````markdown
## Run gate — verify a dispatched implement-next run before you relay it

A dispatched run that stops early returns a report that reads as success, and a completion
notification is the CHILD's claim, not your report. Do not trust either; read git before relaying
an outcome as your own. Docket's helper facade is not on `PATH`: run each command below verbatim,
expansion included. `verify-run` only reads local metadata, so both snapshots must be taken from
FRESH ORIGIN state — re-sync on BOTH sides, or a claim abandoned by an earlier session shows up
only in the after-read and is attributed to this run.

1. **Before dispatching** `docket-implement-next`, re-sync the metadata worktree with
   `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh preflight`, then snapshot the claimed
   set: `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh verify-run --in-progress-ids`.
2. When you issue the dispatch and can block on it, dispatch **foreground** and block on the
   return: never background it and never poll. A dispatch you background — or one the harness
   backgrounds for you — is not covered here; use **Detached dispatch** below.
3. **After the return**, re-sync again with
   `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh preflight` and re-run
   `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh verify-run --in-progress-ids`. Any id
   absent from the snapshot is this run's claim; an empty diff (drained, or a lost claim race) ends
   the gate. If MORE THAN ONE id is new, stop and report: this run claims at most one change, so at
   least one of them is a concurrent run's and none can be told apart — never re-dispatch onto a
   change another agent may be holding.
4. Run `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh verify-run <id>` and key on its
   report line, never its exit code:
   - `run-complete` / `run-unclaimed` — done.
   - `run-halted` — done; **never re-dispatch** a halt, which means a human is needed.
   - `run-incomplete` — re-dispatch the same agent **once**, passing the id and the unmet
     conjuncts; verify again; if still incomplete, stop and report loudly. Never a third dispatch.

### Detached dispatch — you did not foreground-block, whoever backgrounded it

- **You issued it after running tool calls:** take the step-1 before-snapshot AND `date -u +%s` as
  `DISPATCH_EPOCH` before launching. At the notification, re-sync, then run
  `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh verify-run --in-progress-ids
  --with-claimed-at` and keep only ids passing ALL THREE filters: absent from the before-set,
  `claimed_at` parses, and `claimed_at` >= `DISPATCH_EPOCH`. Exactly one survivor → step 4
  unchanged; none → done; two or more → stop and report, as in step 3.
- **Slash-command or notification-first launch — unattributed mode.** No before-set exists, and a
  timestamp alone cannot attribute: `claimed_at` is re-stamped at every phase boundary, so a
  concurrent run claimed before your window looks fresh too. Verify and report ONLY — `verify-run
  <id>` on any id the notification names (a prose id is a hint, never authority), else on each
  current in-progress id, reporting every verdict. **Never re-dispatch** here: that needs all
  three filters, and re-dispatching onto a change a live agent holds is the one unrecoverable move.
````

Confirm the length before going further:

```bash
grep -c "" cursor-rules/run-gate.md   # must print exactly 43
```

If it prints anything else, the paste reflowed — fix it rather than adjusting the bound.

- [ ] **Step 8: Regenerate the committed `AGENTS.md` block**

A bare `bash sync-agents.sh` is a **no-op in this repo**: `check_project_level`'s surface leg is gated on `per_repo_opted_in`, which reads `agent_harnesses:`/`agents:` from `.docket.yml` or `.docket.local.yml`, and the committed `.docket.yml` carries neither. The opt-in lives in the gitignored `.docket.local.yml`, which does not exist in a fresh worktree. Use the recipe the test's currency leg documents:

```bash
# .docket.local.yml is gitignored; back up any pre-existing one rather than clobbering it.
[ -e .docket.local.yml ] && cp .docket.local.yml .docket.local.yml.bak
printf 'agent_harnesses: [claude, cursor]\n' > .docket.local.yml
bash sync-agents.sh
rm -f .docket.local.yml
[ -e .docket.local.yml.bak ] && mv .docket.local.yml.bak .docket.local.yml
git status --porcelain
```

Expected from `git status --porcelain`: `M AGENTS.md` plus the two files you edited by hand. The generator also writes `.cursor/rules/docket-dispatch.mdc`, `.cursor/agents/docket-*.md` and `.claude/agents/docket-*.md`, but all three globs are **gitignored** (`.gitignore` lines for `.claude/agents/docket-*.md`, `.cursor/agents/docket-*.md`, `.cursor/rules/docket-dispatch.mdc`), so none appears. `CLAUDE.md` is a symlink to `AGENTS.md` — the same physical file — so it neither appears nor needs regenerating separately.

If any **other** tracked file shows as modified, stop and report rather than committing it: this task's diff is exactly three files.

- [ ] **Step 9: Run the test to verify it passes**

Run: `bash tests/test_sync_agents_run_gate.sh`

Expected: `ALL PASS`, with the currency assert (`this repo's committed AGENTS.md block is current`) green — that assert prints a `diff -u` on failure, so a red there names the exact drift.

- [ ] **Step 10: Mutation-test the raised bound**

The bound is only real if it can redden. Prove each mutation landed with a count.

```bash
cp cursor-rules/run-gate.md /tmp/rg.bak
grep -c "" cursor-rules/run-gate.md                      # 43
printf 'x\n' >> cursor-rules/run-gate.md
grep -c "" cursor-rules/run-gate.md                      # 44 — mutation landed
bash tests/test_sync_agents_run_gate.sh | grep -F "gate text is at most"
mv /tmp/rg.bak cursor-rules/run-gate.md
grep -c "" cursor-rules/run-gate.md                      # 43 — restored
```

Expected: `NOT OK - gate text is at most 43 lines` while mutated. (The rendered-verbatim asserts also redden, because `slice_gate` reads `GATE_LINES` from the mutated template — that is the same bound doing its job through a second reader, not a separate finding.)

- [ ] **Step 11: Mutation-test the detached section — whole, then conjunct by conjunct**

Four separate probes. Each one: back up with `cp`, mutate, **prove it landed with `grep -c`**, run the test, restore with `mv`. Never `git checkout --` — the file is edited and uncommitted, so that restores to HEAD and destroys Step 7's work.

**M1 — the whole section.** Delete from the `### Detached dispatch` heading to EOF:

```bash
cp cursor-rules/run-gate.md /tmp/rg.bak
awk '/^### Detached dispatch/{exit} {print}' /tmp/rg.bak > cursor-rules/run-gate.md
grep -c "Detached dispatch" cursor-rules/run-gate.md     # was 2, must now be 1 (step 2 still refers to it)
bash tests/test_sync_agents_run_gate.sh | grep -F "NOT OK"
mv /tmp/rg.bak cursor-rules/run-gate.md
```

Expected: every one of the nine detached asserts reddens, starting with `NOT OK - gate: the detached section exists to be asserted about`.

**M2 — only the third filter.** Branch A's epoch conjunct was added on top of the set difference at the design gate; deleting just it must still redden something:

```bash
cp cursor-rules/run-gate.md /tmp/rg.bak
grep -c -e "--with-claimed-at" cursor-rules/run-gate.md  # 1
perl -0pi -e 's/, and `claimed_at` >= `DISPATCH_EPOCH`//' cursor-rules/run-gate.md
diff /tmp/rg.bak cursor-rules/run-gate.md                # MUST show the deletion; empty means it never landed
bash tests/test_sync_agents_run_gate.sh | grep -F "NOT OK"
mv /tmp/rg.bak cursor-rules/run-gate.md
```

Expected: `NOT OK - gate: detached branch A requires all three filters, named` — and *only* that one from the branch-A group, which is what proves the conjunct is guarded on its own rather than riding the pair.

**M3 — the never-re-dispatch rule.** Replace branch B's prohibition with a permissive twin, so the mutation is a *reintroduced defect* rather than a deletion:

```bash
cp cursor-rules/run-gate.md /tmp/rg.bak
perl -0pi -e 's/\*\*Never re-dispatch\*\* here: that needs all/You may re-dispatch here once, using all/' cursor-rules/run-gate.md
diff /tmp/rg.bak cursor-rules/run-gate.md                # MUST show the substitution
bash tests/test_sync_agents_run_gate.sh | grep -F "NOT OK"
mv /tmp/rg.bak cursor-rules/run-gate.md
```

Expected: `NOT OK - gate: unattributed mode never re-dispatches, and says why a timestamp cannot attribute`.

**M4 — step 2's scoping, reverted to the blanket wording.** This is the pre-change state, so it is the state the new assert must detect:

```bash
cp cursor-rules/run-gate.md /tmp/rg.bak
perl -0pi -e 's/2\. When you issue the dispatch and can block on it, dispatch \*\*foreground\*\* and block on the\n   return: never background it and never poll\. A dispatch you background — or one the harness\n   backgrounds for you — is not covered here; use \*\*Detached dispatch\*\* below\./2. Dispatch **foreground** and block on the return; never background it and never poll./' cursor-rules/run-gate.md
diff /tmp/rg.bak cursor-rules/run-gate.md                # MUST show the substitution
bash tests/test_sync_agents_run_gate.sh | grep -F "NOT OK"
mv /tmp/rg.bak cursor-rules/run-gate.md
```

Expected: `NOT OK - gate: step 2 scopes foreground-and-block to a dispatch this session can block on` and `NOT OK - gate: step 2 routes a dispatch it did not block on to the detached section`, while `gate: step 2 forbids backgrounding and polling on that path` stays green — the blanket wording still contains that phrase, which is precisely why the two new asserts had to be written.

If any probe leaves the suite green, that assert is decoration: fix the assert, not the reading.

- [ ] **Step 12: Confirm the tree is back to the intended state, then re-run**

```bash
grep -c "" cursor-rules/run-gate.md                      # 43
bash tests/test_sync_agents_run_gate.sh
```

Expected: `43`, then `ALL PASS`. A mutation left behind here would be committed in the next step.

- [ ] **Step 13: Commit**

```bash
git add cursor-rules/run-gate.md tests/test_sync_agents_run_gate.sh AGENTS.md
git commit -m "fix(0275): give the run gate a runnable detached-dispatch path

The gate assumed one dispatch shape — before-snapshot, foreground dispatch,
after-snapshot, diff. A slash-command launch backgrounds the run inside the
same user turn, so steps 1-3 are structurally unrunnable and only the child's
own prose named the id: the evidence class the gate exists to distrust.

Scope step 2's prohibition to a dispatch the session issues and can block on,
add a Detached dispatch section with an attributable branch (before-set plus
DISPATCH_EPOCH, all three filters, step 4 unchanged) and an unattributed
branch (verify-and-report, never re-dispatch), and reframe the obligation
against relaying rather than reporting. Brevity bound raised 25 -> 43 with
recorded rationale; AGENTS.md regenerated via the documented recipe."
```

---

### Task 2: Re-measure the budgeted file and record its margin

`tests/test_sync_agents_run_gate.sh` carries a row in `tests/runtime-budgets.tsv` (`10`, `parallel`). Task 1 added roughly a dozen asserts to it. They are pure shell string matches over an already-loaded variable, so the expected movement is nil — but a budget row this change touched must be re-measured and reported as a **number**, never as "did not trip the budget check" (learning: `budget-headroom-is-spent-before-it-is-breached`).

**Files:**
- Modify (only if the measurement demands it): `tests/runtime-budgets.tsv`
- Test: `scripts/run-tests.sh`

**Interfaces:**
- Consumes: Task 1's committed test file.
- Produces: the measured serial and contended readings, carried into Task 3's results file.

- [ ] **Step 1: Measure the file standalone, worst of three**

```bash
for i in 1 2 3; do /usr/bin/time -p bash tests/test_sync_agents_run_gate.sh >/dev/null; done
```

Expected: roughly 3.5s real per run (the pre-change reading was 3.455s). Record the **worst** of the three — the table's own rule prices a row off the worst standalone serial reading, not the run-of-the-day one.

- [ ] **Step 2: Decide the row**

Apply the table's stated rule — next multiple of 5, plus a 5s margin — to the worst reading. At ~3.5s that yields 10, which is the row's current value: **leave `tests/runtime-budgets.tsv` untouched** and record the margin as a number.

Raise the row only if the worst reading is at or above 10s. If you do raise it, edit the row in place and re-seed `EXPECTED_TOTAL` per the table's own instructions in `tests/runtime-budgets.tsv`.

- [ ] **Step 3: Run the whole suite**

Run: `scripts/run-tests.sh`

Expected: `fail=0`. Two specific things to read for:

1. **No `OVER BUDGET:` line naming `tests/test_sync_agents_run_gate.sh`.** A trailing `OVER BUDGET:` line does not fail the run, so nothing else will catch it — it is a finding to act on for a file this change touched.
2. **`tests/test_sync_agents_runners.sh` at ~184s against its 60s ceiling is PRE-EXISTING**, tracked as change #0280, and is **not** this change's to fix. Leave it alone; note it in the results file as pre-existing so a reader does not attribute it here. Its `! grep -qi "never background it" "$G"` assert targets `emit_shim`'s generated shim, a different file from the gate template — it must stay green, and a red there means an edit escaped into the wrong population.

- [ ] **Step 4: Commit only if the row moved**

If Step 2 left `tests/runtime-budgets.tsv` untouched, there is nothing to commit — say so and move on.

```bash
git add tests/runtime-budgets.tsv
git commit -m "test(0275): re-budget the run-gate guard row against its measured reading"
```

---

### Task 3: Write the results file

**Files:**
- Create: `docs/results/2026-08-11-run-gate-detached-dispatch-path-results.md`

**Interfaces:**
- Consumes: Task 1's mutation evidence, Task 2's measurements.
- Produces: the `results:` artifact the change's frontmatter will point at.

- [ ] **Step 1: Author the results file**

Copy the structure of `docs/changes/results-template.md` and fill in:

- **What was built** — the scoped step 2, the two-branch Detached section, the reframed obligation, and the fact that the deliverable is prose plus its guard (no script changed; `verify-run.sh` and `runner-dispatch.sh` already carried the oracle).
- **The deliberate brevity-bound raise, 25 → 43**, with the rationale recorded in the test's history comment quoted here too — an always-loaded context block grew by 72%, and that is a cost a future reader is entitled to see priced rather than discovered.
- **Mutation evidence** — the four probes from Task 1 Step 11 plus the bound probe from Step 10, each with the assert that reddened.
- **Budget margin as a number** — the worst standalone reading against the row's 10s ceiling, and whether the row moved. Name `tests/test_sync_agents_runners.sh` (~184s vs a 60s ceiling) as **pre-existing, tracked as #0280, untouched here**.
- **Human verification** — one item no in-repo test can be an oracle for: whether a live parent session, handed a completion notification from a slash-command-launched run, actually follows the unattributed branch and declines to re-dispatch. The suite can prove the text is present and reachable on every surface; it cannot prove a model obeys it. This is the same class of item change 0242's results file recorded for the foreground path.
- **Deliberate non-goals** — `cursor-rules/dispatch.head.md` was left alone on purpose (its item 2 governs every docket agent dispatch, not just implement-next runs), and the detached path deliberately grows no polling loop.

- [ ] **Step 2: Stamp the back-link and commit**

```bash
"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh render-artifact-backlink \
  --artifact-file docs/results/2026-08-11-run-gate-detached-dispatch-path-results.md \
  --change-file ../../.docket/docs/changes/active/0275-run-gate-has-no-runnable-path-for-slash-command-or-backgroun.md
git add docs/results/2026-08-11-run-gate-detached-dispatch-path-results.md
git commit -m "docs(0275): results — the detached-dispatch path and its deliberate bound raise"
```

Use absolute paths for both flags if the relative form does not resolve from the worktree root.

---

## Self-Review

**Spec coverage.**

| Spec section | Task |
|---|---|
| §1 Keep foreground primary, scope step 2, adjust the two step-2 asserts | Task 1, Steps 4, 7, 11-M4 |
| §2 Detached section — branch A (before-set + epoch, three filters, step 4 unchanged, cardinality) | Task 1, Steps 5, 7 |
| §2 Detached section — branch B (unattributed, verify-and-report, never re-dispatch, prose id is a hint) | Task 1, Steps 5, 7 |
| §3 Reframe the ordering obligation against relaying | Task 1, Steps 3, 7 |
| §4 Raise the 25-line bound deliberately, rationale in the history comment | Task 1, Steps 2, 10 |
| §4 Regenerate via the documented recipe (bare sync-agents is a no-op) | Task 1, Step 8 |
| §4 Extend the guard and mutation-test it | Task 1, Steps 5, 11 |
| Reconcile addendum 1 — `dispatch.head.md` not edited; section self-scopes | Global Constraints; Task 1 Step 5's scoping assert |
| Reconcile addendum 2 — native-dispatch-only, no facade argv | Global Constraints; the template adds no facade re-dispatch |
| Reconcile addendum 3 — no observation loop | Global Constraints; the section is notification-driven |
| Budget re-measurement | Task 2 |

**Placeholder scan.** No `TBD`, no "add appropriate error handling", no "similar to Task N". Every assert, every mutation command, and the full 43-line template are written out verbatim.

**Type consistency.** Variable names used across steps: `GATE_SRC`, `GATE_LINES`, `G`, `STEP2`, `STEP3` (pre-existing); `HEADW`, `DETACHED` (new, defined in Steps 3 and 5 before use). The asserted substrings in Steps 3–5 each appear verbatim in the Step 7 template — checked pairwise: `before you relay it`, `the CHILD`, `claim, not your report`, `before relaying`, `can block on it`, `block on the return`, `never background it`, `never poll`, `harness`, `backgrounds for you`, `Detached dispatch`, `you did not foreground-block`, `before-snapshot`, `date -u +%s`, `DISPATCH_EPOCH`, `verify-run --in-progress-ids --with-claimed-at` (spans a line break; `flat()` collapses it), `ALL THREE filters`, `absent from the before-set`, `parses`, `one survivor`, `step 4`, `two or more`, `stop and report`, `unattributed mode`, `No before-set exists`, `re-stamped at every phase boundary`, `looks fresh`, `Never re-dispatch`, `the notification names`, `never authority`. None contains an apostrophe.
