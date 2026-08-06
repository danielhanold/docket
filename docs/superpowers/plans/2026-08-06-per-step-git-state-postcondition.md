<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0203 — Define the per-step git-state postcondition docket-implement-next now names but never states](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0203-define-the-per-step-git-state-postcondition-docket-implement.md)**
<!-- docket:backlink:end -->

# Per-step git-state postcondition Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give `docket-implement-next`'s orphan term `git-state postcondition` a referent by stating one per-step postcondition table for Steps 2–7, prefaced by a governing sentence that denies an intermediate certificate ever ends a run, and guard it mechanically.

**Architecture:** Three surfaces, all in the docket repo itself. (1) `skills/docket-implement-next/SKILL.md` gains a `### Step postconditions` subsection immediately **before** `### Terminal disposition`, and its §5 clause is repaired to point at it. (2) `tests/test_skill_size_budgets.sh`'s row for that file is re-derived from the post-edit measured actual, with an in-diff justification comment. (3) `tests/test_loop_continuation.sh` — which already owns the terminal-contract prose asserts over this same file — gains the presence guard, proximity-scoped on §5, with a mutation probe per new matcher.

**Tech Stack:** Markdown prose (skill bodies), Bash 4.4+ test scripts using the repo's `assert(){ ... eval ... }` house harness, `grep -E`, `awk` section extraction, `mktemp` fixtures.

## Global Constraints

- **Docs-only change.** No new field, status, run record, or runtime mechanism is introduced (spec §3, Out of scope). No script under `scripts/` is modified.
- **Every table row is a condition over git** — refs, commits, frontmatter fields, and the committed build-evidence record. Stated once as a property of the whole table, not repeated per row.
- **`tests/test_loop_continuation.sh:36-38` is a live constraint.** It asserts `advanced.{0,80}Step 7|Step 7.{0,80}advanced` over `SKILL.md`. Any compression of the *Terminal disposition* sentence must keep `Step 7` within 80 characters of `advanced`.
- **Phrase greps read a whitespace-collapsed haystack** (learnings: `phrase-grep-over-wrapped-prose`, change 0218) — `flatten(){ tr -s '[:space:]' ' '; }`, the exact idiom already used at `tests/test_docket_review.sh:193`. `awk` section extractors keep their newlines.
- **Every new matcher gets its own mutation probe** (learnings: `assert-detects-removal-not-replacement`, `mirrored-guard-enforces-its-own-property`, AGENTS.md `guards-are-code`). Copying a matcher is not inheriting its property — probe by execution.
- **Non-vacuity anchors follow `tests/test_role_skill_self_description.sh`**, not `test_loop_continuation.sh`'s current style: a file-exists + non-empty anchor and a live presence assert through the same read, per spec assumption 5.
- **Budget row derivation rule** (`tests/test_skill_size_budgets.sh` header): lines → next multiple of 5, words → next multiple of 50; **if that multiple lands within 25 words of the measured actual, take the multiple after.** The same near-zero-margin reasoning applies to the line axis — the comment block records raising twice for one line of headroom.
- **Measured baseline (2026-08-06, post-0212):** `skills/docket-implement-next/SKILL.md` = **145 lines / 3844 words** against budget row `150 3850`. Re-measure; never trust this number after editing.

---

## File Structure

| File | Responsibility | Action |
|---|---|---|
| `skills/docket-implement-next/SKILL.md` | The contract. Holds the new `### Step postconditions` section and the repaired §5 clause. | Modify |
| `tests/test_skill_size_budgets.sh` | The budget row `skills/docket-implement-next/SKILL.md 150 3850` (line 428) + its in-diff justification comment block. | Modify |
| `tests/test_loop_continuation.sh` | The mechanical companion — presence + proximity guard for the new section. Already owns this file's terminal-contract asserts (0088, 0212). | Modify |

No files are created. No `references/` file is added — a common-path rule parked in a conditionally-read reference is absent precisely when it must fire (spec assumption 1).

---

### Task 1: State the postconditions in SKILL.md and re-derive the budget row

**Files:**
- Modify: `skills/docket-implement-next/SKILL.md` — insert a new `### Step postconditions` subsection immediately before `### Terminal disposition` (currently line 99); repair the §5 clause (currently line 74); repoint the *Terminal disposition* pointer sentence (currently lines 118–119).
- Modify: `tests/test_skill_size_budgets.sh:428` (the budget row) and its comment block above.
- Test: `tests/test_skill_size_budgets.sh` (existing — this task's oracle).

**Interfaces:**
- Consumes: nothing from an earlier task.
- Produces: the exact heading string `### Step postconditions`; the six row-label prefixes `| 2 Claim`, `| 3 Reconcile`, `| 4 Worktree + plan`, `| 5 Build`, `| 6 Review + ADRs`, `| 7 PR + stop`; the governing-sentence phrases `certify a **step**, never the run` and `the only postcondition that also completes the run is Step 7's`; and the §5 pointer phrase `see *Step postconditions*`. **Task 2's matchers grep for exactly these strings** — if any is reworded here, Task 2's asserts must be reworded to match.

- [ ] **Step 1: Inventory the suite's existing asserts over this file before editing it**

`restatement-accumulates-its-own-guards`: eleven test files grep this SKILL.md. Compression must not silently delete prose another guard asserts.

Run from the worktree root:

```bash
grep -rn 'docket-implement-next/SKILL.md\|IMPL=' tests/*.sh | grep -v test_skill_size_budgets
```

Expected: hits in `test_artifact_backlink_coverage.sh`, `test_board_refresh_on_transition.sh`, `test_closeout.sh`, `test_composition_wiring.sh`, `test_docket_config.sh`, `test_dispatch_capability.sh`, `test_docket_metadata_branch.sh`, `test_docket_review.sh`, `test_loop_continuation.sh`, `test_learnings_ledger.sh`, `test_results_artifact.sh`, `test_skill_facade_wiring.sh`. Read each matcher that targets Step 5, Step 7, or *Terminal disposition* prose. **Do not delete or reword any sentence one of them matches.** Note especially `test_board_refresh_on_transition.sh:21`, which requires `run the Board pass (best-effort` to appear **at least 3 times**, and `test_learnings_ledger.sh:55,57`, which require `learnings/README.md` and `learnings.enabled` at least twice each.

- [ ] **Step 2: Record the pre-edit measurement**

```bash
wc -l -w skills/docket-implement-next/SKILL.md
grep -n 'docket-implement-next/SKILL.md' tests/test_skill_size_budgets.sh | tail -1
```

Expected: `145 3844` and row `skills/docket-implement-next/SKILL.md                      150 3850`. If either differs, use the observed values — the branch may have moved.

- [ ] **Step 3: Insert the `### Step postconditions` section**

Insert immediately **before** the line `### Terminal disposition (driver contract)`. Write exactly:

```markdown
### Step postconditions

Each step below is complete only when its row holds — read from **git**, never from a sub-skill's report or its own narration. The conditions are **cumulative**: each holds in addition to every earlier step's. These certify a **step**, never the run. **Once a change is claimed, and absent a `halted` disposition or a Step-3 kill, the only postcondition that also completes the run is Step 7's** — a satisfied intermediate row is never licence to stop. A run that ends any other way ends on a **disposition**, not on a postcondition.

| Step | Complete only when |
|---|---|
| 2 Claim | `status: in-progress` + `branch:` + `claimed_at:` committed on `metadata_branch` **and landed** (local tip == remote tip). |
| 3 Reconcile | `reconciled: true` and a dated `## Reconcile log` entry landed on `metadata_branch` — or, on the kill path, the change archived. |
| 4 Worktree + plan | Step 3's push SHA-confirmed **before** the branch is cut; then the plan file **and** its `docket:backlink` stamp committed on `feat/<slug>`, **and** `plan:` landed on `metadata_branch` — a two-tree conjunction, both refs read. |
| 5 Build | the executed plan committed on `feat/<slug>`, with a build-evidence record at `result: green` whose `head_sha` **equals branch HEAD** (the conjunct that makes a sub-skill's report git-checkable). |
| 6 Review + ADRs | that record still green at `head_sha` == HEAD **after** any fix commits, and every ADR the run produced landed in `adrs:`. Known-weak row: on a clean review this reduces to Step 5's, because whether a reviewer ran is not a fact about git. |
| 7 PR + stop | the branch pushed (`origin/feat/<slug>` resolves), the PR open, and `status: implemented` + `pr:` landed on `metadata_branch`; `results:` set **iff** a results file and its backlink stamp are committed on `feat/<slug>`. |

Steps 0, 1 and 6.5 get no row: 0 produces nothing scoped to this change, 1 is a pure read, and 6.5 is optional — its artifact rides in Step 7's `iff` conjunct.
```

- [ ] **Step 4: Repair the §5 clause so the term has a referent**

In Step 5 (currently line 74), replace exactly:

```
Emitting that log line discharges the suppression obligation only; the step is not complete until its git-state postcondition holds.
```

with:

```
Emitting that log line discharges the suppression obligation only; the step is not complete until its git-state postcondition holds — see *Step postconditions*.
```

This is the whole repair: the sentence stays (it is the only producer-side occurrence of the term on the common path, and 0113's budget rationale argues in-diff that it must fire at the moment of action — spec §5), and gains a pointer.

- [ ] **Step 5: Repoint the *Terminal disposition* sentence at the table**

In *Terminal disposition*, replace exactly:

```
`advanced` is claimable only when **Step 7's postcondition** holds; that postcondition is
Step 7's to state, not this section's.
```

with:

```
`advanced` is claimable only when **Step 7's postcondition** holds — stated in *Step postconditions* above, not here.
```

This keeps `Step 7` within 80 characters of `advanced` (the Global Constraint), keeps the pointer 0212's guard asserts, and saves a line now that the referent exists.

- [ ] **Step 6: Re-measure and re-derive the budget row**

```bash
wc -l -w skills/docket-implement-next/SKILL.md
```

Take the measured actual. Apply the documented rounding rule: lines → next multiple of 5, words → next multiple of 50; **if the chosen multiple lands within 25 words (or ~1–2 lines) of the actual, take the multiple after.** Edit the row at `tests/test_skill_size_budgets.sh:428` in place, preserving its column alignment:

```
skills/docket-implement-next/SKILL.md                      <LINES> <WORDS>
```

Then append a justification comment to the block above the table — after the existing 0212 entry — stating: (a) the change and what it added; (b) that the compression of the *Terminal disposition* pointer sentence was taken first and how much it recovered; (c) the measured pre- and post-edit actuals; (d) the derivation from the actual per the rounding rule; and (e) **why the prose cannot live in `skills/docket-implement-next/references/edge-paths.md`** — that file is read CONDITIONALLY, only once a run already knows it hit a named edge, whereas a postcondition table is read on the **common path at every step boundary**, so a rule parked there is unread precisely when it must intervene. Model the wording on the existing 0212 entry immediately above it.

- [ ] **Step 7: Run the budget test — the task's oracle**

```bash
bash tests/test_skill_size_budgets.sh
```

Expected: `PASS`. If it reports the `docket-implement-next` row over budget, the re-derivation in Step 6 was wrong — recompute from `wc`, do not pad the number arbitrarily.

- [ ] **Step 8: Prove no other guard was broken by the compression**

```bash
for t in tests/test_loop_continuation.sh tests/test_board_refresh_on_transition.sh \
         tests/test_learnings_ledger.sh tests/test_composition_wiring.sh \
         tests/test_dispatch_capability.sh tests/test_docket_review.sh \
         tests/test_results_artifact.sh tests/test_closeout.sh; do
  echo "== $t"; bash "$t" | tail -1
done
```

Expected: `PASS` from each. `test_loop_continuation.sh` in particular must still pass — Step 5's edit sits inside the file it reads and its `advanced`/`Step 7` proximity assert is the one at risk.

- [ ] **Step 9: Commit**

```bash
git add skills/docket-implement-next/SKILL.md tests/test_skill_size_budgets.sh
git commit -m "docs(0203): state the per-step git-state postcondition table

Gives the orphan term a referent: one table for Steps 2-7 before Terminal
disposition, prefaced by the cumulative/step-not-run governing sentence, with
the SKILL.md 5 clause repaired to point at it. Budget row re-derived from the
post-edit measured actual."
```

---

### Task 2: Guard the section, its rows, and the §5 pointer

**Files:**
- Modify: `tests/test_loop_continuation.sh` — append a change-0203 block after the existing change-0212 block (currently ends line 38), before the `--- SKILL.md: per-step-exit mappings ---` block.
- Test: `tests/test_loop_continuation.sh` itself (the guard's mutation probes are its own oracle).

**Interfaces:**
- Consumes: from Task 1 — the heading `### Step postconditions`, the six row labels `2 Claim` / `3 Reconcile` / `4 Worktree + plan` / `5 Build` / `6 Review + ADRs` / `7 PR + stop`, the governing phrases `certify a **step**, never the run` and `the only postcondition that also completes the run is Step 7's`, and the §5 pointer `see *Step postconditions*`. `IMPL` is already defined at line 12 of this file.
- Produces: nothing consumed downstream.

- [ ] **Step 1: Write the guard block**

Insert into `tests/test_loop_continuation.sh` immediately after the change-0212 block (the assert ending `"SKILL gates advanced on Step 7's postcondition by pointer"`) and before the `# --- SKILL.md: per-step-exit mappings ---` comment:

```bash
# --- SKILL.md: the per-step postcondition table (change 0203) ---
# 0113 added the clause "the step is not complete until its git-state postcondition holds" and
# defined no postcondition anywhere; the term occurred exactly once in skills/. These asserts pin
# both halves of the repair: the DEFINING section, and — separately — that §5's producer-side
# clause actually points at it. A file-level "the term occurs more than once" count would be the
# 0199 co-occurrence weakness: it proves nothing about §5, which is where the reader hits the term.
#
# Phrase asserts read a whitespace-collapsed haystack so a pure re-flow of the table's prose never
# reddens a policy assert (learnings: phrase-grep-over-wrapped-prose, change 0218). The idiom is
# the same one-liner as tests/test_docket_review.sh:193 — deliberately re-stated rather than
# extracted, at 1 line it is cheaper to read here than to follow a pointer.
flatten(){ tr -s '[:space:]' ' '; }

# Non-vacuity anchor #1: the file every assert below reads must exist and be non-empty, or the
# whole block passes for reasons unrelated to the property (style borrowed from
# tests/test_role_skill_self_description.sh, NOT from this file's own thinner precedent).
assert "SKILL.md exists and is non-empty" '[ -s "$IMPL" ]'

# The defining section.
assert "SKILL states a Step postconditions section" 'grep -qF -- "### Step postconditions" "$IMPL"'

# Every step the table claims to cover has a row. A missing row is the exact 0113 defect (Step 4
# received no rider) and is what this loop exists to catch.
for row in "2 Claim" "3 Reconcile" "4 Worktree + plan" "5 Build" "6 Review + ADRs" "7 PR + stop"; do
  assert "the postcondition table has a row for Step $row" 'grep -qF -- "| $row |" "$IMPL"'
done

# The governing sentence — a step certificate is NEVER a run certificate. This is the half the
# 0206 evidence bought: that run satisfied Step 5's postcondition at the moment it died.
assert "SKILL states the postconditions certify a step, not the run" \
  'flatten < "$IMPL" | grep -qF -- "certify a **step**, never the run"'
assert "SKILL states only Step 7's postcondition also completes the run" \
  'flatten < "$IMPL" | grep -qF -- "the only postcondition that also completes the run is Step 7'"'"'s"'
assert "SKILL states the conditions are cumulative" \
  'flatten < "$IMPL" | grep -qiE "cumulative"'

# PROXIMITY-SCOPED producer assert (learnings: specified-but-unreachable). The contract's producer
# is §5's clause; anchoring only on the defining section would leave the term orphaned exactly
# where a reader meets it. Extract the Step 5 region with awk — the extractor MUST keep newlines
# or the slice becomes the whole file — then flatten only the slice.
step5_region(){ awk '/^### Step 5 — Build/{f=1; next} f && /^### /{exit} f' "$IMPL"; }
s5="$(mktemp)"; step5_region > "$s5"
# Non-vacuity anchor #2: a live PRESENCE assert through the same extraction. If the Step 5 heading
# is renamed, this reddens instead of the proximity assert below going quietly green on an empty
# slice.
assert "the Step 5 region extracts and is non-empty" '[ -s "$s5" ]'
assert "the Step 5 region is really Step 5 (names the build role)" \
  'grep -qF -- "SKILL_BUILD" "$s5"'
assert "SKILL.md 5's git-state postcondition clause points at the table" \
  'flatten < "$s5" | grep -qE "git-state postcondition.{0,120}Step postconditions"'
rm -f "$s5"

# --- Mutation proofs: one per matcher introduced above (guards-are-code;
# assert-detects-removal-not-replacement; mirrored-guard-enforces-its-own-property). A matcher
# that has never been shown to go RED against the state it forbids is untested code.
probe="$(mktemp)"
# (a) the heading matcher fires on the heading and not on a near-miss.
printf '%s\n' '### Step postcondition' > "$probe"   # singular — a real typo shape
assert "the heading matcher rejects a singular near-miss" '! grep -qF -- "### Step postconditions" "$probe"'
# (b) the row-label matcher fires only on a table row, not on prose naming the step.
printf '%s\n' 'Step 4 Worktree + plan is where the plan file is written.' > "$probe"
assert "the row matcher ignores prose that merely names a step" '! grep -qF -- "| 4 Worktree + plan |" "$probe"'
printf '%s\n' '| 4 Worktree + plan | something holds. |' > "$probe"
assert "the row matcher fires on a real table row" 'grep -qF -- "| 4 Worktree + plan |" "$probe"'
# (c) the flattened phrase matcher survives a line wrap — the property flatten() exists for.
printf 'These\ncertify a **step**,\nnever the run.\n' > "$probe"
assert "the flattened phrase matcher survives a hard wrap" \
  'flatten < "$probe" | grep -qF -- "certify a **step**, never the run"'
# (d) THE load-bearing one: the proximity assert must go RED on the pre-0203 state — the clause
# present with no pointer. This is the state 0203 removed, not the wording it introduced.
printf '%s\n' 'the step is not complete until its git-state postcondition holds. docket-build routes each task.' > "$probe"
assert "the proximity matcher REJECTS the orphan clause (the pre-0203 defect)" \
  '! { flatten < "$probe" | grep -qE "git-state postcondition.{0,120}Step postconditions"; }'
printf '%s\n' 'the step is not complete until its git-state postcondition holds — see *Step postconditions*.' > "$probe"
assert "the proximity matcher ACCEPTS the repaired clause" \
  'flatten < "$probe" | grep -qE "git-state postcondition.{0,120}Step postconditions"'
# (e) and it must not be satisfied by a far-apart co-occurrence elsewhere in the same region.
printf 'git-state postcondition holds.%s Step postconditions\n' "$(printf ' x%.0s' $(seq 1 130))" > "$probe"
assert "the proximity matcher rejects a distant co-occurrence" \
  '! { flatten < "$probe" | grep -qE "git-state postcondition.{0,120}Step postconditions"; }'
rm -f "$probe"
```

- [ ] **Step 2: Run the guard — it must pass against Task 1's prose**

```bash
bash tests/test_loop_continuation.sh
```

Expected: `PASS`, with the new `ok - ` lines present. If the six row asserts fail, the row labels in `SKILL.md` and the loop have drifted — fix the **loop** to match the prose Task 1 actually wrote (the prose is the contract; the guard follows it).

- [ ] **Step 3: Prove the guard bites on the real file, not only on fixtures**

Fixture probes prove the matcher; this proves the *wiring*. Temporarily delete the heading from the real file and confirm the suite reddens:

```bash
cp skills/docket-implement-next/SKILL.md /tmp/impl.bak
sed -i '' 's/^### Step postconditions$/### Step postcondition/' skills/docket-implement-next/SKILL.md
bash tests/test_loop_continuation.sh | grep -c '^NOT OK'
cp /tmp/impl.bak skills/docket-implement-next/SKILL.md
bash tests/test_loop_continuation.sh | tail -1
```

Expected: a non-zero `NOT OK` count while mutated, then `PASS` after restore. Confirm `git status` shows `skills/docket-implement-next/SKILL.md` clean before continuing — a left-behind mutation is a silent contract change.

- [ ] **Step 4: Commit**

```bash
git add tests/test_loop_continuation.sh
git commit -m "test(0203): guard the Step postconditions table and 5's pointer

Presence asserts for the section, one per Step 2-7 row, and the cumulative /
step-not-run governing sentence; plus a proximity-scoped assert that 5's
producer-side clause actually points at the table, rather than a file-level
occurrence count that would prove nothing about 5. Each matcher carries its
own mutation probe, and the proximity probe reddens on the pre-0203 orphan
clause specifically."
```

---

## Verification

- [ ] Full suite green, run once as the build gate: `bash tests/run_all.sh` (or the repo's documented runner — check `AGENTS.md` / `tests/` for the entry point).
- [ ] `grep -c 'git-state postcondition' skills/docket-implement-next/SKILL.md` returns **1** — §5's clause only; the new `### Step postconditions` section deliberately does not repeat the term. Non-orphanhood is not pinned by an occurrence count but by the proximity assert in `tests/test_loop_continuation.sh`, which requires §5's clause to point at *Step postconditions*.
- [ ] `wc -l -w skills/docket-implement-next/SKILL.md` is at or under the re-derived budget row.
- [ ] `git diff origin/main...HEAD --stat` touches exactly three files: the SKILL.md and the two test files.

## Self-Review notes

- **Spec coverage.** §1 one-table → Task 1 Step 3. §2 governing sentence → Task 1 Step 3 (its second paragraph) + Task 2's three phrase asserts. §3 enforceability/prose-presence guard, proximity-scoped, borrowing role-self-description anchors → Task 2 Steps 1–3. §4 Step 4 gets a row at full strength (SHA-compare + backlink stamp + two-tree conjunction) → the `4 Worktree + plan` row. §5 clause repaired not deleted → Task 1 Step 4. §6 size budget, compress-first-then-re-derive → Task 1 Steps 5–6. Named residual (Step 6 vacuity) → stated in the `6 Review + ADRs` row itself. Steps 0/1/6.5 exclusion → the trailing sentence after the table.
- **Known deviation from the spec, deliberate:** the spec anticipated a compression pass over "Step 7's enumerated prose." Task 1 compresses only the *Terminal disposition* pointer sentence. Step 7's prose is matched by asserts in `test_artifact_backlink_coverage.sh`, `test_results_artifact.sh` and `test_closeout.sh` (Task 1 Step 1 inventories them), and `restatement-accumulates-its-own-guards` says the copy has become load-bearing. `size-target-is-direction` covers the residual: take the budget raise rather than cut prose another guard holds.
