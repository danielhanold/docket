<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0218 — Fix review findings in-branch instead of minting a stub for every one](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-06-0218-fix-review-findings-in-branch-instead-of-minting-a-stub-for.md)**
<!-- docket:backlink:end -->

# Fix review findings in-branch instead of minting a stub for every one — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give `docket-implement-next` Step 6 a bounded fix loop that repairs review findings on the open branch before the PR opens, so a non-blocking finding stops being routed to the backlog as a stub.

**Architecture:** Three moving parts. (1) docket-build's `## Routing` rubric is extracted to `skills/docket-build/references/task-routing.md` so the fix loop and docket-build classify from one shared source instead of a restatement. (2) A new `skills/docket-implement-next/references/fix-loop.md` owns the fix-loop mechanics — character→profile routing, severity→posture, per-finding and batched tasks, the revert-and-record suite gate — read blockingly from Step 6 only when review returns findings. (3) A new `review.min_fix_severity` config knob, resolved as `REVIEW_MIN_FIX_SEVERITY`, sets the minimum severity that enters the loop, with `blocker` as the pre-0218 compat escape hatch.

**Tech Stack:** POSIX-ish bash 4.4+ (`scripts/docket-config.sh`), markdown skill contracts under `skills/`, and hand-rolled `assert`-based bash test files under `tests/`. No build system, no package manager. Every test file is run directly: `bash tests/test_x.sh`.

## Global Constraints

Copied from the spec and from this repo's standing rules — every task's requirements implicitly include this section.

- **`max` is never dispatched from the fix loop, for any severity.** The fix ladder truncates at `premium`. A max-character blocker halts; a max-character important/minor becomes a PR-body record.
- **The fix phase never restates the routing rubric.** It reads `skills/docket-build/references/task-routing.md`. Restatement drift is a documented learnings class (`restatement-accumulates-its-own-guards`).
- **`review.min_fix_severity`** values are exactly `minor` (default) | `important` | `blocker`. Resolved as `REVIEW_MIN_FIX_SEVERITY`. Global-able, **not** coordination-fenced. Fails closed on any other value.
- **Blockers are always fixed regardless of the knob** — a run cannot proceed past an unfixed blocker.
- **Bounded at two full-suite runs** in the fix phase: one after fixes land, one after a revert. Still-red halts. There is **no re-review round**.
- **No new run status and no new frontmatter field.** A halt leaves the change `in-progress` with `claimed_at` refreshed, per the convention's Tier C posture.
- **Guards are code** (`AGENTS.md`): every new assert must be mutation-tested — plant the defect, watch it redden, confirm the mutation actually landed with a `grep -c` before/after count, then revert. An assert that never saw red is untested code.
- **Write the assert that DETECTS the state you removed**, not one that confirms the wording you introduced (`assert-detects-removal-not-replacement`).
- **Absence asserts need a non-vacuity companion through the same extractor** — an `awk` section extractor that silently returns empty makes every negative grep pass.
- **A new config knob ships end-to-end in the same change**: resolver, `scripts/docket-config.md` contract row + export list, `.docket.example.yml`, README, and any prose that stated the now-relaxed rule as absolute (`config-knob-ship-end-to-end`).
- **Every `skills/**/*.md` needs a budget row** in `tests/test_skill_size_budgets.sh` — a completeness guard fails otherwise. Raise a budget in the same diff that grows a file, set from the *measured* actual: next multiple of 5 (lines) / 50 (words), and if that leaves ≤ ~1 line or ≤ 25 words of margin, take the multiple after. Record the derivation in the `BUDGETS` comment block.
- **Sentinels grep the copy, not the source.** Before deleting or moving prose, `grep -rn` the phrase across `tests/` and repoint any dependent assert at the artifact that now owns the content — relocation, never restoration.

## File Structure

**Created**

| File | Responsibility |
|---|---|
| `skills/docket-build/references/task-routing.md` | The consumer-neutral character→profile rubric, with its `max`/`premium` organizing principle and rationale. Two owners: docket-build's per-task routing and docket-implement-next's fix loop. |
| `skills/docket-implement-next/references/fix-loop.md` | Fix-loop mechanics: severity→posture table, per-finding vs batched tasks, the revert-and-record suite gate, the PR-body disposition table. |

**Modified**

| File | Change |
|---|---|
| `skills/docket-build/SKILL.md` | `## Routing` becomes a stub + pointer keeping the consumer-specific rules (plan override, routing line). |
| `skills/docket-implement-next/SKILL.md` | Step 6's triage paragraph becomes the fix-loop rule + blocking pointer. |
| `skills/docket-implement-next/references/edge-paths.md` | PR-body findings become a disposition table. |
| `skills/docket-implement-next/results-template.md` | `## Verify (human)` narrows to genuinely manual checks. |
| `skills/docket-convention/references/auto-capture.md` | Materiality bar gains the in-branch-fixable clause. |
| `scripts/docket-config.sh` | `review_key` + `REVIEW_MIN_FIX_SEVERITY` + validation + emit. |
| `scripts/docket-config.md` | Contract table row, prose paragraph, export-name list. |
| `.docket.example.yml` | The `review:` block. |
| `README.md` | The findings-destination paragraph and the knob. |
| `tests/test_docket_build.sh` | Rubric asserts repointed to the reference; extraction guards. |
| `tests/test_docket_review.sh` | Stale triage assert replaced; fix-loop guards; README assert. |
| `tests/test_docket_config.sh` | The `REVIEW_MIN_FIX_SEVERITY` block. |
| `tests/test_docket_example_yml.sh` | `map_for` + `classify_key` entries. |
| `tests/test_skill_size_budgets.sh` | Two new rows, budget raises, derivation comments. |

**Task order is a dependency chain:** Task 1 creates the shared rubric Task 3 points at; Task 2 creates the export Task 3's prose names; Task 5 documents what Tasks 2–4 built.

---

### Task 1: Extract the routing rubric to a shared reference

**Files:**
- Create: `skills/docket-build/references/task-routing.md`
- Modify: `skills/docket-build/SKILL.md:48-90` (the whole `## Routing` section)
- Modify: `tests/test_docket_build.sh:148-168` (the rubric asserts)
- Modify: `tests/test_skill_size_budgets.sh` (BUDGETS table + comment block)

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: the path `skills/docket-build/references/task-routing.md`, and the two literal phrases later tasks and guards anchor on — `**Build profile:**` (stays in SKILL.md) and `cannot walk back` (moves to the reference). Task 3's `fix-loop.md` links to this file by that exact relative-from-repo-root path.

**Why an extraction rather than a second copy:** the justification is *shared consumption*, not section weight. Per the `skill-extraction-and-stub-pointer` learning, the stub keeps the rule and the reference keeps rule + why; per `shared-resource-keeps-first-owner-assumptions`, the reference must be written for two owners from the start, and one guard must assert *both* consumers point at it — a single-owner assert passes against a file that never learned to share.

- [ ] **Step 1: Write the failing tests**

Append to `tests/test_docket_build.sh`, immediately after the existing routing-assert block that ends at line 168 (the `controller: the demoted top-rung triggers now name premium, not max` assert). First add the reference-file handle near the top of the file, beside the existing `WORKER=` line at line 8:

```bash
ROUTING="$REPO/skills/docket-build/references/task-routing.md"
```

Then the new asserts:

```bash
# --- change 0218: the routing rubric is EXTRACTED, with two owners ------------
# The rubric moved out of the controller so docket-implement-next's fix loop can classify a
# finding from the same source instead of restating it. These asserts are the multi-owner
# fixture the shared-resource-keeps-first-owner-assumptions learning demands: a guard that only
# checks docket-build still points at the file passes against a reference that never learned it
# has a second consumer.
assert "routing: the shared reference exists" '[ -f "$ROUTING" ]'
routing_body="$(cat "$ROUTING" 2>/dev/null)"
assert "routing: reference is non-vacuous (>= 20 lines)" \
  '[ "$(printf "%s\n" "$routing_body" | grep -c .)" -ge 20 ]'

# The rubric itself now lives in the reference, not the controller.
assert "routing: economy must be POSITIVELY established (in the reference)" \
  'grep -qE "^- \*\*\`economy\`\*\* — \*only when\*" <<<"$routing_body"'
assert "routing: named risk selects premium (in the reference)" \
  'grep -qiE "premium[^.]{0,200}(authentication|security boundar)" <<<"$routing_body"'
assert "routing: max's direct rubric is unresolved architecture + irreversible data only" \
  'grep -qiE "\*\*\`max\`\*\*[^.]{0,240}unresolved architecture" <<<"$routing_body" &&
   grep -qiE "\*\*\`max\`\*\*[^.]{0,240}irreversible" <<<"$routing_body"'
assert "routing: the demoted top-rung triggers name premium, not max" \
  '! grep -qiE "\*\*\`max\`\*\*[^.]{0,240}(authentication|security boundar|concurrency|release infrastructure)" <<<"$routing_body"'
# The RATIONALE moved with the rule — an extraction is behavior-neutral only if the why moved too.
assert "routing: the reference carries the max/premium organizing principle" \
  'grep -qiF -- "cannot walk back" <<<"$routing_body"'

# The controller keeps a stub + pointer, and no longer carries the rubric bullets itself.
assert "controller: Routing section points at the shared reference" \
  'grep -qF -- "references/task-routing.md" <<<"$ctrl_body"'
assert "controller: no longer restates the economy rubric bullet" \
  '! grep -qE "^- \*\*\`economy\`\*\* — \*only when\*" <<<"$ctrl_body"'
# The consumer-SPECIFIC rules stay in the controller: they are docket-build's, not the fix loop's.
assert "controller: keeps the plan-override rule" \
  'grep -qF -- "**Build profile:**" <<<"$ctrl_body"'
assert "controller: keeps the invalid-override halt" \
  'grep -qiE "invalid[^.]{0,80}halt" <<<"$ctrl_body"'

# TWO OWNERS. This is the assert a single-owner fixture cannot reach.
IMPL_FIX="$REPO/skills/docket-implement-next/references/fix-loop.md"
assert "routing: the reference names both of its consumers" \
  'grep -qiF -- "docket-build" <<<"$routing_body" &&
   grep -qiF -- "docket-implement-next" <<<"$routing_body"'
assert "routing: the reference does not describe itself as docket-build's alone" \
  '! grep -qiE "only consumer|sole consumer" <<<"$routing_body"'
```

Note: `IMPL_FIX` is declared here but only asserted against in Task 3, which creates `fix-loop.md`. Do **not** add a `[ -f "$IMPL_FIX" ]` assert in this task — it would be red for a reason unrelated to this task's deliverable. Task 3 adds that assert to `tests/test_docket_review.sh`, which is where the fix loop's guards belong.

- [ ] **Step 2: Run tests to verify they fail**

Run: `bash tests/test_docket_build.sh`
Expected: FAIL — `NOT OK - routing: the shared reference exists`, `NOT OK - routing: reference is non-vacuous`, every `routing:` assert reading the empty `$routing_body`, and `NOT OK - controller: Routing section points at the shared reference`. The two `! grep` negative asserts (`controller: no longer restates…`, `routing: the demoted top-rung…`) will pass vacuously at this point — that is expected and is exactly why the non-vacuity floor assert above exists.

- [ ] **Step 3: Create the shared reference**

Create `skills/docket-build/references/task-routing.md` with this exact content. The four rubric bullets and the organizing-principle paragraph are **moved verbatim** from `skills/docket-build/SKILL.md:60-88`; everything else is new framing that makes the file consumer-neutral.

```markdown
# task-routing — the character→profile rubric

The shared classification rubric behind docket's profile-routed work. **Two consumers read this
file**, and it is written for both:

- **`docket-build`** routes each plan task to a profile agent (`## Routing` in its `SKILL.md`).
- **`docket-implement-next`** routes each review finding in its Step 6 fix loop
  (`references/fix-loop.md`).

Neither restates it. What is classified differs — a plan task, a review finding — but the question
is identical: *how much reasoning investment does this piece of work need, and what happens if it
is got wrong?* Consumer-specific rules (a plan's `**Build profile:**` override, the fix loop's
`premium` ceiling, escalation ladders) belong to each consumer, not here.

## The rubric

Classify with a deliberate asymmetry — `economy` must be *positively* established, named risk
selects upward, and uncertainty defaults to `standard`.

The `max`/`premium` boundary has an organizing principle, not just a list: **`max` is for mistakes
the correction machinery cannot walk back.** Destroyed data cannot be un-destroyed by a retry, and
a wrong architectural call shapes every task after it; a patch-correctable bug is caught at the
suite gate or in review. Resolve edge cases by applying that test, not by extending the lists
below.

- **`max`** — **unresolved architecture** or an **irreversible data change** (a destructive
  migration, a backfill, anything that cannot be rolled back). Nothing else classifies here.
  Irreversibility is the test: a reversible or purely additive migration is *not* `max` — it is
  `premium`, or `standard` if it carries no consequential risk at all.
- **`premium`** — authentication or security boundaries, concurrency or locking, release
  infrastructure, or any consequential risk **explicitly named in the plan or spec text**. That last
  door is honored, not inferred: never articulate a new risk on your own — your classification is
  this closed list, so uncertainty still sinks to `standard`.
- **`standard`** — everything remaining; the default and the uncertainty sink. Deliberately includes
  hard-but-safe work: difficulty known ahead of time is handled by the consumer's own override, and
  difficulty discovered while working is handled by the `standard -> premium` escalation.
- **`economy`** — *only when* the work is fully specified, follows an established pattern, carries no
  consequential risk, and requires **no cross-file reasoning** — either localized to a couple of
  implementation files (tests do not count against locality), or a mechanical, pattern-identical
  edit repeated across many files whose instances do not interact and where a missed instance fails
  loudly (a grep, a validator) rather than silently. All four conditions must hold; doubt about any
  one of them means `standard`.

`max` is rare by construction. Its doors are narrow and each consumer states its own: docket-build
admits the two-item rubric above, an explicit plan override, and a `premium` escalation; the fix
loop admits **none of them** — it never dispatches `max` at all.
```

- [ ] **Step 4: Replace the controller's `## Routing` section with a stub + pointer**

In `skills/docket-build/SKILL.md`, replace lines 48–90 — the entire `## Routing` section from the `## Routing` heading through `Emit one concise routing line per task naming both the profile and its reason.` — with:

```markdown
## Routing

**Explicit override wins.** A plan task may carry a line of the form:

```markdown
**Build profile:** economy
```

A valid value (`economy`, `standard`, `premium`, `max`) is authoritative; record its use in that task's
routing line. An **invalid** value is a plan contract error: **halt** per *Halting conditions* and
surface it — never silently fall back to a default.

**Otherwise classify** using the shared character→profile rubric in
[`references/task-routing.md`](references/task-routing.md) — the same rubric
`docket-implement-next`'s Step 6 fix loop reads, which is why it lives in a file rather than here.
**Read it now (blocking) before routing your first task.** It carries the deliberate asymmetry
(`economy` positively established, uncertainty sinking to `standard`), the `max`/`premium`
organizing principle, and the four tier bullets. Never restate it in this file or in your dispatch
prose.

For docket-build specifically, `max` has exactly three doors: the rubric's two-item direct
classification, an explicit plan override, and a `premium` escalation.

Emit one concise routing line per task naming both the profile and its reason.
```

Leave `## Profiles` (lines 30–46) and `## Escalation` untouched — the escalation ladder is docket-build's own, not shared.

- [ ] **Step 5: Verify the move was behavior-neutral**

Byte-diff the moved bullets against the original section, so the extraction is provably a move and not a rewrite:

```bash
git show HEAD:skills/docket-build/SKILL.md \
  | awk '/^- \*\*`max`\*\*/,/^`max` is rare by construction/' > /tmp/routing-before.txt
awk '/^- \*\*`max`\*\*/,/^`max` is rare by construction/' \
  skills/docket-build/references/task-routing.md > /tmp/routing-after.txt
diff /tmp/routing-before.txt /tmp/routing-after.txt
```

Expected: differences confined to the `standard` bullet's second sentence (reworded to drop the docket-build-specific "the plan override covers…") and the closing `max is rare` paragraph (reworded for two consumers). The `max`, `premium`, and `economy` bullets must diff **clean**. If any of those three shows a diff, you rewrote rather than moved — restore the original wording.

- [ ] **Step 6: Add the budget row and raise**

In `tests/test_skill_size_budgets.sh`, add a row to the `BUDGETS` heredoc, placed alphabetically among the `skills/docket-build*` rows:

```
skills/docket-build/references/task-routing.md              XX  YYY
```

Measure first — `wc -l` and `wc -w` on the new file — then set `XX`/`YYY` per the Global Constraints rounding rule. Then append to the `BUDGETS` comment block, immediately before the `BUDGETS="` line:

```
# skills/docket-build/references/task-routing.md is a NEW row added by change 0218, which extracted
# docket-build's `## Routing` rubric to a shared reference so docket-implement-next's Step 6 fix loop
# could classify a finding from the same source rather than restate it. The justification for the
# extraction is SHARED CONSUMPTION, not section weight — the file has two owners, and a restated
# rubric is the documented drift class (restatement-accumulates-its-own-guards). Set per the
# rounding rule above from the measured actual: <L> lines -> XX, <W> words -> YYY.
# skills/docket-build/SKILL.md's budget was NOT lowered by the extraction: a budget is a ceiling,
# and lowering it to the new actual would redden the next unrelated edit for no invariant.
```

Substitute the real measured `<L>`/`<W>` and the chosen `XX`/`YYY`.

- [ ] **Step 7: Run tests to verify they pass**

Run: `bash tests/test_docket_build.sh && bash tests/test_skill_size_budgets.sh`
Expected: both print `PASS`.

- [ ] **Step 8: Mutation-test every new assert**

For each new assert, plant its defect, confirm the mutation landed with a count, run the test, confirm the specific assert reddens, revert. Concretely, at minimum:

```bash
# (a) the two-owner assert must bite
grep -c "docket-implement-next" skills/docket-build/references/task-routing.md   # expect >= 1
perl -0pi -e 's/docket-implement-next/docket-SOMETHINGELSE/g' skills/docket-build/references/task-routing.md
grep -c "docket-implement-next" skills/docket-build/references/task-routing.md   # expect 0 — mutation landed
bash tests/test_docket_build.sh | grep "NOT OK - routing: the reference names both"
git checkout skills/docket-build/references/task-routing.md

# (b) the rationale-moved assert must bite
grep -c "cannot walk back" skills/docket-build/references/task-routing.md        # expect 1
perl -0pi -e 's/cannot walk back/cannot be undone/g' skills/docket-build/references/task-routing.md
grep -c "cannot walk back" skills/docket-build/references/task-routing.md        # expect 0
bash tests/test_docket_build.sh | grep "NOT OK - routing: the reference carries the max/premium"
git checkout skills/docket-build/references/task-routing.md

# (c) the stub's pointer assert must bite
perl -0pi -e 's{references/task-routing\.md}{references/gone.md}g' skills/docket-build/SKILL.md
bash tests/test_docket_build.sh | grep "NOT OK - controller: Routing section points"
git checkout skills/docket-build/SKILL.md

# (d) the no-restatement assert must bite — reintroduce the deleted bullet
printf '\n- **`economy`** — *only when* the task is fully specified.\n' >> skills/docket-build/SKILL.md
bash tests/test_docket_build.sh | grep "NOT OK - controller: no longer restates"
git checkout skills/docket-build/SKILL.md
```

Each `grep "NOT OK - …"` must produce a line. If any produces nothing, that assert is not guarding what its name claims — fix the assert, not the test run.

- [ ] **Step 9: Commit**

```bash
git add skills/docket-build/references/task-routing.md skills/docket-build/SKILL.md \
        tests/test_docket_build.sh tests/test_skill_size_budgets.sh
git commit -m "refactor(0218): extract the routing rubric to a two-owner shared reference"
```

---

### Task 2: The `review.min_fix_severity` config knob

**Files:**
- Modify: `scripts/docket-config.sh:590` (after the `build:` block, before `# --- change_types + auto_capture`)
- Modify: `scripts/docket-config.sh:783` (the `emit BUILD_CHECKPOINT` line — insert directly after)
- Modify: `scripts/docket-config.md:130` (contract table), `:212` (prose), `:345` (export-name list)
- Modify: `.docket.example.yml:160-166` (insert a `review:` block after the `build:` block)
- Modify: `tests/test_docket_config.sh:1249` (insert the new block after the BUILD_CHECKPOINT block, before `# --- Change 0091 — auto_capture`)
- Modify: `tests/test_docket_example_yml.sh:118` (`map_for`), `:186` (`classify_key`)

**Interfaces:**
- Consumes: nothing from Task 1.
- Produces: the export `REVIEW_MIN_FIX_SEVERITY`, emitted **directly after `BUILD_CHECKPOINT`** and before `SKILL_BRAINSTORM`. Values: exactly `minor` | `important` | `blocker`. Task 3's Step 6 prose and Task 5's README both name this exact variable.

**Precedent:** this knob is a structural clone of `build.checkpoint` (change 0167) — a nested block read leaf-by-leaf through `config_block_get`, global-able, failing closed on garbage. Follow that shape exactly; do not invent a new one.

**One real hazard, and it needs its own guard:** the `skills:` block already has a `review:` *leaf* (`skills.review: docket-review`). `config_block_header` rejects any indented line (`scripts/docket-config.sh:171`), so the leaf cannot be read as a block header — but that is an invariant to *assert*, not to assume, because a future refactor of the header matcher would silently make `skills.review`'s value (`docket-review`) resolve as `review.min_fix_severity` and abort every run.

- [ ] **Step 1: Write the failing tests**

Insert into `tests/test_docket_config.sh` immediately after the `# --- (BLD-h) the contract doc documents it` block (which ends at line 1249) and before the `# --- Change 0091 — auto_capture` comment:

```bash
# ============================================================================
# Change 0218 — the review: block (REVIEW_MIN_FIX_SEVERITY)
# Structural clone of the build: block above. NOTE (guards-are-code (e)): clear the asserted var
# BEFORE each eval — an aborting run emits NOTHING, and eval "" would silently leave the previous
# case's value in place.
# ============================================================================

# --- (RMF-a) default when no layer sets the block -----------------------------
unset REVIEW_MIN_FIX_SEVERITY
mkrepo "$tmp/rmf-a"
out="$(run "$tmp/rmf-a" --export)"; eval "$out"
assert "REVIEW_MIN_FIX_SEVERITY defaults to minor" \
  'echo "$out" | grep -qxF "REVIEW_MIN_FIX_SEVERITY=minor"'

# --- (RMF-b) repo-committed block is honored ----------------------------------
unset REVIEW_MIN_FIX_SEVERITY
mkrepo "$tmp/rmf-b"
cat > "$tmp/rmf-b/.docket.yml" <<'EOF'
metadata_branch: main
review:
  min_fix_severity: important
EOF
git -C "$tmp/rmf-b" add .docket.yml; git -C "$tmp/rmf-b" commit --quiet -m cfg
git -C "$tmp/rmf-b" push --quiet origin main
out2="$(run "$tmp/rmf-b" --export)"; eval "$out2"
assert "REVIEW_MIN_FIX_SEVERITY reads the block" \
  'echo "$out2" | grep -qxF "REVIEW_MIN_FIX_SEVERITY=important"'

# --- (RMF-c) global-able (ADR-0019 — NOT coordination-fenced) -----------------
unset REVIEW_MIN_FIX_SEVERITY
mkrepo "$tmp/rmf-c"
mkdir -p "$tmp/rmf-c.xdg/docket"
cat > "$tmp/rmf-c.xdg/docket/config.yml" <<'EOF'
review:
  min_fix_severity: blocker
EOF
rmf_c_err="$(rung "$tmp/rmf-c.xdg" "$tmp/rmf-c" --export 2>&1 >/dev/null)"
out="$(rung "$tmp/rmf-c.xdg" "$tmp/rmf-c" --export 2>/dev/null)"; eval "$out"
assert "review.min_fix_severity is global-able (not fenced)" \
  '[ "$REVIEW_MIN_FIX_SEVERITY" = "blocker" ]'
assert "no fence warning for review.min_fix_severity" \
  '! printf "%s" "$rmf_c_err" | grep -qi "review.*per-repo-only"'

# --- (RMF-d) repo-local layer wins over repo-committed ------------------------
unset REVIEW_MIN_FIX_SEVERITY
mkrepo "$tmp/rmf-d"
cat > "$tmp/rmf-d/.docket.yml" <<'EOF'
metadata_branch: main
review:
  min_fix_severity: blocker
EOF
git -C "$tmp/rmf-d" add .docket.yml; git -C "$tmp/rmf-d" commit --quiet -m cfg
git -C "$tmp/rmf-d" push --quiet origin main
printf 'review:\n  min_fix_severity: minor\n' > "$tmp/rmf-d/.docket.local.yml"
out="$(run "$tmp/rmf-d" --export)"; eval "$out"
assert "local layer beats repo-committed for review.min_fix_severity" \
  '[ "$REVIEW_MIN_FIX_SEVERITY" = "minor" ]'

# --- (RMF-e) SHADOW GUARD — a bare min_fix_severity: OUTSIDE the review: block -
unset REVIEW_MIN_FIX_SEVERITY
mkrepo "$tmp/rmf-e"
cat > "$tmp/rmf-e/.docket.yml" <<'EOF'
metadata_branch: main
some_future_block:
  min_fix_severity: blocker
EOF
git -C "$tmp/rmf-e" add .docket.yml; git -C "$tmp/rmf-e" commit --quiet -m cfg
git -C "$tmp/rmf-e" push --quiet origin main
out="$(run "$tmp/rmf-e" --export)"; eval "$out"
assert "a foreign block's min_fix_severity: does not shadow review.min_fix_severity" \
  '[ "$REVIEW_MIN_FIX_SEVERITY" = "minor" ]'

# --- (RMF-f) THE skills.review COLLISION GUARD --------------------------------
# `skills:` already carries a `review:` LEAF. config_block_header rejects indented lines, so the
# leaf can never be read as a `review:` BLOCK header — but that is the invariant this knob's
# correctness rests on, so assert it rather than assume it. Without this, a future relaxation of
# the header matcher would resolve skills.review's value (`docket-review`) as
# review.min_fix_severity and abort every run in every repo that sets a review skill.
unset REVIEW_MIN_FIX_SEVERITY
mkrepo "$tmp/rmf-f"
cat > "$tmp/rmf-f/.docket.yml" <<'EOF'
metadata_branch: main
skills:
  review: docket-review
EOF
git -C "$tmp/rmf-f" add .docket.yml; git -C "$tmp/rmf-f" commit --quiet -m cfg
git -C "$tmp/rmf-f" push --quiet origin main
out="$(run "$tmp/rmf-f" --export)"; eval "$out"
assert "skills.review's leaf does not shadow the review: block" \
  '[ "$REVIEW_MIN_FIX_SEVERITY" = "minor" ]'
assert "skills.review still resolves normally alongside the review: block" \
  '[ "$SKILL_REVIEW" = "docket-review" ]'

# --- (RMF-g) fail closed on garbage -------------------------------------------
assert "non-enum min_fix_severity aborts nonzero" \
  '! run_resolver_with "review:\n  min_fix_severity: critical\n" >/dev/null 2>&1'
rmf_g_err="$(run_resolver_with "review:\n  min_fix_severity: critical\n" 2>&1 >/dev/null)"
assert "unparseable review.min_fix_severity: mentions review.min_fix_severity" \
  'printf "%s" "$rmf_g_err" | grep -qF "review.min_fix_severity"'

# --- (RMF-h) export presence and POSITION -------------------------------------
out_rmf="$(run "$tmp/rmf-a" --export)"
out_rmf_plain="$(run "$tmp/rmf-a" --export --format plain)"
assert "REVIEW_MIN_FIX_SEVERITY is emitted" \
  'grep -q "^REVIEW_MIN_FIX_SEVERITY=" <<<"$out_rmf"'
assert "REVIEW_MIN_FIX_SEVERITY is emitted directly after BUILD_CHECKPOINT" \
  '[ "$(grep -n "^REVIEW_MIN_FIX_SEVERITY=" <<<"$out_rmf" | cut -d: -f1)" \
     = "$(( $(grep -n "^BUILD_CHECKPOINT=" <<<"$out_rmf" | cut -d: -f1) + 1 ))" ]'
assert "REVIEW_MIN_FIX_SEVERITY present in plain format too" \
  'grep -q "^REVIEW_MIN_FIX_SEVERITY=" <<<"$out_rmf_plain"'

# --- (RMF-i) the contract doc documents it ------------------------------------
assert "docket-config.md has a review.min_fix_severity table row" \
  'grep -qE "^\| \`review\.min_fix_severity\` \| \`minor\` \| yes \|" "$REPO/scripts/docket-config.md"'
assert "docket-config.md lists the export name" \
  'grep -q "^REVIEW_MIN_FIX_SEVERITY$" "$REPO/scripts/docket-config.md"'
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `bash tests/test_docket_config.sh 2>&1 | grep "NOT OK"`
Expected: every new `RMF-*` assert reddens — `REVIEW_MIN_FIX_SEVERITY defaults to minor` first, since no such export exists. `RMF-g`'s "aborts nonzero" assert will also fail, because an unknown key is currently ignored rather than rejected.

- [ ] **Step 3: Add the resolver block**

In `scripts/docket-config.sh`, insert immediately after the `build:` block's closing `esac` (line 593, right before the `# --- change_types + auto_capture` comment):

```bash
# --- review: the review-role knobs (change 0218) ------------------------------
# Nested block parsed exactly like build: — the leaf is read WITHIN the block via config_block_get,
# never as a bare top-level key. Two reasons here, not one: `min_fix_severity` is a generic-ish
# word another block could shadow, AND the `skills:` block already carries a `review:` LEAF. Only
# config_block_header's column-0 requirement keeps `skills.review: docket-review` from being read
# as this block's header; tests/test_docket_config.sh (RMF-f) pins that invariant from both sides.
# Behavioral, NOT coordination-fenced: it shapes BRANCH content (which findings get fixed in the
# diff a human reviews), never shared metadata, so it resolves through the full per-field layering
# repo-local > repo-committed > global > built-in, like build.checkpoint / reclaim.* / learnings.*.
# min_fix_severity is the MINIMUM finding severity that enters docket-implement-next's Step 6 fix
# loop. Blockers are always fixed regardless — a run cannot proceed past an unfixed blocker — so
# `blocker` means "fix nothing else", the pre-0218 record-everything behavior kept as a compat
# escape hatch. Fails CLOSED on anything else (the build.checkpoint / learnings.enabled
# precedent): silently defaulting a typo would either over-fix or under-fix a branch a human is
# about to merge, and both are invisible.
review_key(){  # review_key <leaf> <default> -> resolved value on stdout
  local v; v="$(config_block_get local review "$1")"
  [ -n "$v" ] || v="$(config_block_get committed review "$1")"
  [ -n "$v" ] || v="$(config_block_get global review "$1")"
  printf '%s' "${v:-$2}"
}
REVIEW_MIN_FIX_SEVERITY="$(review_key min_fix_severity minor)"
case "$REVIEW_MIN_FIX_SEVERITY" in
  minor|important|blocker) ;;
  *) die "unparseable config: review.min_fix_severity must be 'minor', 'important', or 'blocker', got '$REVIEW_MIN_FIX_SEVERITY'" ;;
esac
```

- [ ] **Step 4: Emit the export**

In `scripts/docket-config.sh`, insert directly after the `emit BUILD_CHECKPOINT "$BUILD_CHECKPOINT"` line:

```bash
  emit REVIEW_MIN_FIX_SEVERITY "$REVIEW_MIN_FIX_SEVERITY"
```

- [ ] **Step 5: Document it in the script contract**

Three edits to `scripts/docket-config.md`.

(a) Insert a table row directly after the `build.checkpoint` row at line 130:

```markdown
| `review.min_fix_severity` | `minor` | yes | read from the nested `review:` block; `minor`/`important`/`blocker`, anything else aborts; resolves repo-local > repo-committed > global; behavioral, not coordination-fenced — the minimum finding severity that enters `docket-implement-next`'s Step 6 fix loop |
```

(b) Insert a prose paragraph directly after the `build.checkpoint` paragraph at line 212:

```markdown
`review.min_fix_severity` (default `minor`, any layer) sets the lowest review-finding severity that
`docket-implement-next`'s Step 6 fix loop repairs in-branch. Blockers are always fixed regardless of
this value, so `blocker` means "fix nothing but blockers" — the pre-0218 record-everything behavior,
kept as a compat escape hatch. `important` fixes blockers and importants and records minors. Note
the nested read: the `skills:` block also has a `review:` key, and only `config_block_header`'s
column-0 requirement distinguishes the two.
```

(c) Insert `REVIEW_MIN_FIX_SEVERITY` into the export-name list at line 345, directly after `BUILD_CHECKPOINT`.

- [ ] **Step 6: Add the example-config block**

In `.docket.example.yml`, insert a `review:` block directly after the `build:` block (after line 166). The active value must equal the shipped default, or the byte-identity fidelity test in `tests/test_docket_example_yml.sh` reddens:

```yaml
review:
  # min_fix_severity — the lowest review-finding severity that docket-implement-next's Step 6 fix
  # loop repairs in-branch, on the open feature branch, before the PR opens. minor (default) => every
  # finding the reviewer returns is a fix candidate. important => minors are recorded in the PR body
  # instead of fixed. blocker => nothing but blockers is fixed, which is docket's pre-0218 behavior,
  # kept as a compat escape hatch. Blockers are ALWAYS fixed regardless of this value: a run cannot
  # proceed past an unfixed blocker. Routing a fix to a model profile is by the fix's CHARACTER, not
  # its severity, and never reaches the `max` profile; severity sets only what happens when a fix
  # fails. Anything other than these three values is a config error, not a silent fallback.
  min_fix_severity: minor
```

- [ ] **Step 7: Wire the example-config guards**

In `tests/test_docket_example_yml.sh`, add to `map_for` directly after the `BUILD_CHECKPOINT)` arm:

```bash
    REVIEW_MIN_FIX_SEVERITY) echo '^[[:space:]]+min_fix_severity:[[:space:]]*minor[[:space:]]*$' ;;
```

Add to `classify_key` directly after the `build.checkpoint)` arm:

```bash
    review.min_fix_severity)      echo 'resolved:REVIEW_MIN_FIX_SEVERITY' ;;
```

And add `review` to the block-header arm's alternation, which currently reads:

```bash
    finalize|learnings|reclaim|build|skills|runners|runners.codex|runners.opencode|auto_capture) echo 'elsewhere:HEADER' ;;
```

so that it becomes:

```bash
    finalize|learnings|reclaim|build|review|skills|runners|runners.codex|runners.opencode|auto_capture) echo 'elsewhere:HEADER' ;;
```

- [ ] **Step 8: Run tests to verify they pass**

Run: `bash tests/test_docket_config.sh && bash tests/test_docket_example_yml.sh`
Expected: both print `PASS`. If `test_docket_example_yml.sh` reports the fidelity diff, the example's active value does not match the shipped default — fix the example, never the resolver default.

- [ ] **Step 9: Mutation-test the new asserts**

```bash
# (a) the enum guard must reject a fourth value
grep -c "minor|important|blocker" scripts/docket-config.sh    # expect 1
perl -0pi -e 's/minor\|important\|blocker\) \;\;/minor|important|blocker|critical) ;;/' scripts/docket-config.sh
grep -c "minor|important|blocker|critical" scripts/docket-config.sh   # expect 1 — mutation landed
bash tests/test_docket_config.sh | grep "NOT OK - non-enum min_fix_severity aborts nonzero"
git checkout scripts/docket-config.sh

# (b) the POSITION assert must bite — move the emit line
perl -0pi -e 's/(  emit REVIEW_MIN_FIX_SEVERITY .*\n)//' scripts/docket-config.sh
perl -0pi -e 's/(  emit BOOTSTRAP )/  emit REVIEW_MIN_FIX_SEVERITY "\$REVIEW_MIN_FIX_SEVERITY"\n$1/' scripts/docket-config.sh
bash tests/test_docket_config.sh | grep "NOT OK - REVIEW_MIN_FIX_SEVERITY is emitted directly after"
git checkout scripts/docket-config.sh

# (c) the skills.review collision guard must bite — relax the header matcher
perl -0pi -e 's/\[\[ "\$line" != \[\[:space:\]\]\* && "\$line" == \*:\* \]\] \|\| return 1/[[ "$line" == *:* ]] || return 1/' scripts/docket-config.sh
bash tests/test_docket_config.sh | grep "NOT OK - skills.review's leaf does not shadow"
git checkout scripts/docket-config.sh
```

Each grep must produce a line. Mutation (c) is the important one: if it produces nothing, the collision guard is decorative.

- [ ] **Step 10: Commit**

```bash
git add scripts/docket-config.sh scripts/docket-config.md .docket.example.yml \
        tests/test_docket_config.sh tests/test_docket_example_yml.sh
git commit -m "feat(0218): add the review.min_fix_severity knob (REVIEW_MIN_FIX_SEVERITY)"
```

---

### Task 3: The fix loop — reference + Step 6 rewrite

**Files:**
- Create: `skills/docket-implement-next/references/fix-loop.md`
- Modify: `skills/docket-implement-next/SKILL.md:84` (the whole "**Triage the returned findings by severity.**" paragraph)
- Modify: `tests/test_docket_review.sh:148` (the stale assert) and the block around it
- Modify: `tests/test_skill_size_budgets.sh` (new row + raise + comments)

**Interfaces:**
- Consumes: `skills/docket-build/references/task-routing.md` (Task 1) — linked, never restated. `REVIEW_MIN_FIX_SEVERITY` (Task 2) — named in the Step 6 prose and in the reference.
- Produces: the path `skills/docket-implement-next/references/fix-loop.md` and the literal marker phrases Task 4 and Task 5's guards anchor on: `disposition table`, `revert`, `min_fix_severity`.

**Why a reference file rather than inline Step 6 prose:** the mechanics are heavy and *conditionally* read — a review that returns nothing never needs them — which is exactly the `skill-extraction-and-stub-pointer` test (heavy AND off the common path). The same-shaped precedents in this repo are `references/edge-paths.md`, `references/gate-failure.md`, and `references/auto-capture.md`. `skills/docket-implement-next/SKILL.md` also sits at 145/150 lines and 3800/3850 words, so inlining would force a large budget raise for prose most runs never read.

**Task profile note:** this is the change's normative core — a wrong rule here ships an autonomous loop that edits branches on the wrong terms. Route it at `premium`. **Build profile:** premium

- [ ] **Step 1: Write the failing tests**

In `tests/test_docket_review.sh`, **replace** the stale assert at line 148:

```bash
assert "controller: important/minor findings go to the PR body, never auto-fixed" \
  'grep -qE "important" <<<"$step6" && grep -qiE "PR body" <<<"$step6"'
```

with this block. Note the shape: the first assert **detects the state being removed** (`assert-detects-removal-not-replacement`), and it is scoped to `$step6`, whose non-vacuity is already anchored by the existing `controller: Step 6 was located` assert above it.

```bash
# --- change 0218: findings are FIXED in-branch, not recorded and re-minted ----
# The removed rule was "An `important` or `minor` finding is recorded in the PR body for the
# human's merge-time judgment, never auto-fixed". The assert that used to sit here confirmed the
# words "important" and "PR body" were present — both of which survive the rewrite, so it would
# have stayed green across the exact change it was meant to notice. Assert the NEGATIVE instead.
assert "controller: Step 6 no longer forbids auto-fixing non-blockers" \
  '! grep -qiE "never auto-fixed" <<<"$step6"'
assert "controller: Step 6 sends findings through a bounded in-branch fix loop" \
  'grep -qiF -- "fix loop" <<<"$step6"'
assert "controller: Step 6 points at the fix-loop reference (blocking read)" \
  'grep -qF -- "references/fix-loop.md" <<<"$step6"'
assert "controller: Step 6 names the severity threshold knob" \
  'grep -qF -- "REVIEW_MIN_FIX_SEVERITY" <<<"$step6"'
assert "controller: blockers still route through the docket-build-task contract" \
  'grep -qF -- "docket-build-task" <<<"$step6"'
assert "controller: no re-review round after fixes" \
  'grep -qiE "no re-review|never re-review" <<<"$step6"'

# --- the fix-loop reference itself --------------------------------------------
FIX="$REPO/skills/docket-implement-next/references/fix-loop.md"
assert "fix-loop: the reference exists" '[ -f "$FIX" ]'
fix_body="$(cat "$FIX" 2>/dev/null)"
assert "fix-loop: reference is non-vacuous (>= 30 lines)" \
  '[ "$(printf "%s\n" "$fix_body" | grep -c .)" -ge 30 ]'

# The routing axis DELEGATES — it must never restate the rubric it shares with docket-build.
assert "fix-loop: routes by character via the shared rubric" \
  'grep -qF -- "task-routing.md" <<<"$fix_body"'
assert "fix-loop: does not restate the rubric's economy bullet" \
  '! grep -qE "^- \*\*\`economy\`\*\* — \*only when\*" <<<"$fix_body"'

# The CEILING is the whole safety argument: no fix task may ever reach max, at any severity.
assert "fix-loop: never dispatches the max profile" \
  'grep -qiE "never[^.]{0,80}\`?max\`?|no fix task[^.]{0,60}max" <<<"$fix_body"'
assert "fix-loop: a max-character blocker halts" \
  'grep -qiE "max[^.]{0,120}halt" <<<"$fix_body"'

# Severity sets POSTURE only — the orthogonality claim, which is what keeps a minor finding from
# being handed to a cheap model just for being minor.
assert "fix-loop: severity selects the failure posture, not the profile" \
  'grep -qiE "severity[^.]{0,100}posture" <<<"$fix_body"'

# Task shape and commits.
assert "fix-loop: blockers and importants get one task per finding" \
  'grep -qiE "(one task per finding|per-finding task)" <<<"$fix_body"'
assert "fix-loop: minors batch by shared routed profile" \
  'grep -qiE "batch" <<<"$fix_body"'
assert "fix-loop: fixes run the docket-build-task contract" \
  'grep -qF -- "docket-build-task" <<<"$fix_body"'

# The suite gate: revert-and-record, bounded at two runs.
assert "fix-loop: re-runs the full suite after fixes land" \
  'grep -qiE "full[- ]suite" <<<"$fix_body"'
assert "fix-loop: a red re-run reverts the NON-BLOCKER fix commits" \
  'grep -qiE "revert[^.]{0,120}non-blocker|non-blocker[^.]{0,120}revert" <<<"$fix_body"'
assert "fix-loop: the gate is bounded at two suite runs" \
  'grep -qiE "two suite runs|at most two" <<<"$fix_body"'
assert "fix-loop: still-red after the revert halts" \
  'grep -qiE "still[- ]red[^.]{0,80}halt" <<<"$fix_body"'

# The knob, and the always-fix-blockers carve-out that makes it safe.
assert "fix-loop: reads the severity threshold from the resolved knob" \
  'grep -qF -- "REVIEW_MIN_FIX_SEVERITY" <<<"$fix_body"'
assert "fix-loop: blockers are fixed regardless of the threshold" \
  'grep -qiE "blocker[^.]{0,120}regardless" <<<"$fix_body"'

# The recording surface.
assert "fix-loop: the PR body carries a disposition table" \
  'grep -qiF -- "disposition table" <<<"$fix_body"'
for d in fixed deferred reverted recorded; do
  assert "fix-loop: the disposition table has a '$d' state" \
    'grep -qiE "\b'"$d"'\b" <<<"$fix_body"'
done
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `bash tests/test_docket_review.sh 2>&1 | grep "NOT OK"`
Expected: `NOT OK - controller: Step 6 no longer forbids auto-fixing non-blockers`, `NOT OK - fix-loop: the reference exists`, `NOT OK - fix-loop: reference is non-vacuous`, and every `fix-loop:` positive assert reading an empty `$fix_body`. The `! grep` negatives inside `$fix_body` pass vacuously — that is what the non-vacuity floor covers.

- [ ] **Step 3: Create the fix-loop reference**

Create `skills/docket-implement-next/references/fix-loop.md`:

```markdown
# fix-loop — repairing review findings in-branch

The mechanics behind `docket-implement-next` Step 6's bounded fix loop. **Read this before
dispatching the first fix task.** Loaded on demand from Step 6; sibling files are not auto-loaded
with the skill.

The loop runs **after review returns and before the PR opens**, on the branch that is already
green. The human's merge gate does not move: every auto-authored fix arrives inside the diff they
were going to read anyway. Nothing here relaxes `docket-review`'s read-only contract (ADR-0066) —
the reviewer stays a reviewer, and the fixing is the implementer's.

## Two orthogonal axes

**Character picks the profile. Severity picks only the failure posture.** Keeping these apart is
the design: a `minor` finding whose fix is genuinely subtle must not be handed to a cheap model for
being minor, and a `blocker` whose fix is a one-word typo must not burn a premium dispatch for
being a blocker.

- **Character → profile.** A finding is a very small work item with the diagnosis pre-written.
  Route it with the shared rubric in
  [`../../docket-build/references/task-routing.md`](../../docket-build/references/task-routing.md)
  — the same file `docket-build` routes plan tasks with. **Never restate that rubric here or in
  your dispatch prose.**
- **Severity → posture.** What happens when the fix does not land.

## The ceiling — no fix task is ever `max`

**No fix task dispatches the `max` profile, at any severity.** `premium` is
"consequential but correctable" — still walk-backable inside a reviewed diff. `max` is defined by
irreversibility, and an irreversible act must never happen to a branch as an unplanned side-quest
discovered at review time. This matches the pre-0218 blocker ladder (`standard` → `premium` →
halt), which also never reached `max`.

The rubric therefore doubles as the size ceiling; there is no separate knob for "too big to fix
in-branch". A **max-character blocker halts** — abort-and-report, the change stays `in-progress`
with `claimed_at` refreshed and the reason recorded. A **max-character important or minor** — rare
by construction, since unresolved architecture flagged as minor essentially does not occur —
becomes a line in the PR body for the human's merge-time judgment, **not** a follow-up change.

## The routing table

| Finding character | blocker | important | minor |
|---|---|---|---|
| `economy` | fix (→ 1 escalation) | fix (→ 1 escalation) | fix, batched (→ 1 escalation) |
| `standard` | fix (→ 1 escalation) | fix (→ 1 escalation) | fix (→ 1 escalation) |
| `premium` | fix (no retry — the next rung is `max`) | fix (no retry) | fix (no retry) |
| `max` | **halt** | PR-body record | PR-body record |

Escalation is docket-build's one-bounded-escalation rule, **truncated at `premium`**: an `economy`
fix retries once at `standard`, a `standard` fix once at `premium`, and a `premium` fix does not
retry at all. Failure after the allowance is exhausted follows the severity posture — a blocker
halts, an important or minor becomes a PR-body record naming the failure as the reason.

## The severity threshold

`REVIEW_MIN_FIX_SEVERITY` (from the Step-0 config export; `minor` by default) is the lowest
severity that enters this loop. `important` records minors instead of fixing them; `blocker` is the
pre-0218 record-everything behavior, kept as a compat escape hatch.

**Blockers are fixed regardless of the threshold** — a run cannot proceed past an unfixed blocker,
so the knob can never disarm the one gate that must not be disarmed. A finding below the threshold
takes the PR-body record path unchanged.

The reviewer's `unverified-build-state` blocker is the one finding you never hand to a worker: you
resolve it by re-running the suite yourself.

## Tasks, batching, commits

Every fix runs the **`docket-build-task`** contract (focused test → implement → verify →
self-review → one commit), dispatched by profile name, **foreground and sequential** — fixes share
one worktree, so two concurrent workers would collide.

- **Blockers and importants: one task per finding**, one commit each, the message naming the
  finding and the reasoning. Per-finding tasks buy failure isolation and a bisectable narrative,
  and blockers are rare enough that the extra dispatches cost nothing.
- **Minors: route each finding first, then batch** those sharing a profile into one task per
  profile — in practice a single `economy` batch. The batch's tier is its members' shared tier, so
  it is homogeneous by construction. One commit enumerating the findings it fixed; a failed batch
  falls back to recording its members.

**Track every fix commit's SHA and whether its task addressed a blocker.** The suite gate below
cannot run without that record.

## The suite gate — revert and record

Run the **whole suite once** after every fix task has landed, using the same command boundary
docket-build's gate uses, and refresh the build-evidence record from the result.

**Green** → proceed to Step 6.5 with the refreshed record.

**Red** → the loop must not leave the branch worse than the green build that entered it:

1. **Revert the non-blocker fix commits** by tracked SHA — the importants and minors. Blocker
   fixes stay: the run cannot proceed without them.
2. **Re-run the suite once.**
3. **Green** → proceed. The reverted findings are recorded unfixed in the PR body, which is the
   fallback they already had.
4. **Still red** → the blocker fixes are implicated, and there is no second repair chain:
   **halt** — abort-and-report, the change stays `in-progress` with the reason recorded.

**At most two suite runs** in this phase. **No re-review round** after fixes: remediation is
carried by each worker's own self-review, the suite gate, and the human reading every fix in the
PR diff.

## Recording — the PR-body disposition table

Findings reach the PR body as a **disposition table**, so the human sees at a glance what was done
about each one:

| State | Meaning |
|---|---|
| **fixed** | repaired in-branch; cite the commit SHA |
| **deferred** | below `REVIEW_MIN_FIX_SEVERITY`, or a max-character non-blocker; recorded for merge-time judgment |
| **reverted** | fixed, then rolled back by the suite gate; the finding stands |
| **recorded** | the fix was attempted and its escalation allowance was exhausted; name the failure |

## Auto-capture is narrower here

**A finding about this branch's own diff is never mintable** — it is fixed or it is recorded.
Minting from review survives only for genuinely distinct, beyond-the-branch work that independently
clears the materiality bar in
[`../../docket-convention/references/auto-capture.md`](../../docket-convention/references/auto-capture.md).
```

- [ ] **Step 4: Rewrite Step 6's triage paragraph**

In `skills/docket-implement-next/SKILL.md`, replace the entire paragraph at line 84 (beginning `**Triage the returned findings by severity.**` and ending `…findings and fixes alike are visible in the PR body.`) with:

```markdown
**Triage the returned findings, then FIX them in-branch.** Findings are repaired on the open branch before the PR opens — a bounded **fix loop**, not a stub for every one. `REVIEW_MIN_FIX_SEVERITY` (Step-0 export; `minor` by default) is the lowest severity that enters it, and blockers are fixed regardless of it. Routing is by the fix's CHARACTER via the shared rubric, never by its severity, and never reaches the `max` profile; every fix runs the `docket-build-task` contract. Before dispatching the first fix task, **read `references/fix-loop.md` now (blocking)** — it owns the routing table, the per-finding and batched task shapes, the revert-and-record suite gate (bounded at two runs; still-red halts), and the PR-body disposition table. The reviewer's `unverified-build-state` blocker is the one exception you resolve yourself, by re-running the suite. A finding that is genuinely distinct beyond-the-branch work still takes the auto-capture path above; a finding about this branch's own diff never does. There is **no re-review** round after fixes — remediation is carried by the worker's own self-review, the suite re-run, and the human reading every fix in the PR diff.
```

- [ ] **Step 5: Add the budget row and raise**

Measure both files:

```bash
wc -l -w skills/docket-implement-next/references/fix-loop.md skills/docket-implement-next/SKILL.md
```

In `tests/test_skill_size_budgets.sh`, add the new row to the `BUDGETS` heredoc, placed with the other `skills/docket-implement-next/` rows:

```
skills/docket-implement-next/references/fix-loop.md          XX  YYY
```

If `skills/docket-implement-next/SKILL.md` now exceeds `150 3850`, raise that row too, from the measured actual per the rounding rule. Then append to the comment block before `BUDGETS="`:

```
# skills/docket-implement-next/references/fix-loop.md is a NEW row added by change 0218, which gave
# Step 6 a bounded in-branch fix loop for review findings. The mechanics are heavy AND conditionally
# read — a review that returns no findings never needs them — which is the skill-extraction-and-stub-
# pointer test, and the same shape as this skill's existing edge-paths.md and the convention's
# auto-capture.md. Step 6 keeps the RULE (fix in-branch, character-routed, never max, threshold knob)
# and the blocking pointer; the reference keeps rule + why. Set per the rounding rule above from the
# measured actual: <L> lines -> XX, <W> words -> YYY.
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `bash tests/test_docket_review.sh && bash tests/test_skill_size_budgets.sh && bash tests/test_docket_build.sh`
Expected: all three print `PASS`. `test_docket_build.sh` is included because Task 1 left a `$IMPL_FIX` handle referring to the file this task creates.

- [ ] **Step 7: Mutation-test the new asserts**

```bash
# (a) the removal assert must bite — reintroduce the deleted rule
grep -c "never auto-fixed" skills/docket-implement-next/SKILL.md    # expect 0
perl -0pi -e 's/(There is \*\*no re-review\*\*)/An important or minor finding is never auto-fixed. $1/' skills/docket-implement-next/SKILL.md
grep -c "never auto-fixed" skills/docket-implement-next/SKILL.md    # expect 1 — mutation landed
bash tests/test_docket_review.sh | grep "NOT OK - controller: Step 6 no longer forbids"
git checkout skills/docket-implement-next/SKILL.md

# (b) the no-max ceiling assert must bite
grep -c "never" skills/docket-implement-next/references/fix-loop.md   # note the count
perl -0pi -e 's/\*\*No fix task dispatches the `max` profile, at any severity\.\*\*/A fix task may dispatch any profile./' skills/docket-implement-next/references/fix-loop.md
bash tests/test_docket_review.sh | grep "NOT OK - fix-loop: never dispatches the max profile"
git checkout skills/docket-implement-next/references/fix-loop.md

# (c) the no-restatement assert must bite
printf '\n- **`economy`** — *only when* the task is fully specified.\n' >> skills/docket-implement-next/references/fix-loop.md
bash tests/test_docket_review.sh | grep "NOT OK - fix-loop: does not restate"
git checkout skills/docket-implement-next/references/fix-loop.md

# (d) the two-run bound must bite
perl -0pi -e 's/\*\*At most two suite runs\*\*/Suite runs are unbounded/' skills/docket-implement-next/references/fix-loop.md
bash tests/test_docket_review.sh | grep "NOT OK - fix-loop: the gate is bounded at two suite runs"
git checkout skills/docket-implement-next/references/fix-loop.md

# (e) the blockers-regardless carve-out must bite
perl -0pi -e 's/\*\*Blockers are fixed regardless of the threshold\*\*/Blockers obey the threshold like everything else/' skills/docket-implement-next/references/fix-loop.md
bash tests/test_docket_review.sh | grep "NOT OK - fix-loop: blockers are fixed regardless"
git checkout skills/docket-implement-next/references/fix-loop.md
```

Every grep must produce a line.

- [ ] **Step 8: Commit**

```bash
git add skills/docket-implement-next/references/fix-loop.md \
        skills/docket-implement-next/SKILL.md \
        tests/test_docket_review.sh tests/test_skill_size_budgets.sh
git commit -m "feat(0218): fix review findings in-branch via a bounded Step 6 fix loop"
```

---

### Task 4: Narrow auto-capture and the recording surfaces

**Files:**
- Modify: `skills/docket-convention/references/auto-capture.md:16-20` (the `## Materiality bar` section)
- Modify: `skills/docket-implement-next/references/edge-paths.md:28` (the build-evidence / PR-body paragraph)
- Modify: `skills/docket-implement-next/results-template.md:10-13` (`## Verify (human)`)
- Modify: `tests/test_docket_review.sh` (append a section)
- Modify: `tests/test_skill_size_budgets.sh` (raise `auto-capture.md` / `edge-paths.md` rows only if measured over)

**Interfaces:**
- Consumes: Task 3's `fix-loop.md` (the disposition table is defined there; these files reference it, never redefine it).
- Produces: nothing later tasks depend on structurally. Task 5's README paragraph restates the *user-facing* consequence only.

- [ ] **Step 1: Write the failing tests**

Append to `tests/test_docket_review.sh`, after the Task 3 block:

```bash
# --- change 0218: auto-capture no longer absorbs this branch's own findings ---
AC="$REPO/skills/docket-convention/references/auto-capture.md"
ac_body="$(cat "$AC" 2>/dev/null)"
assert "auto-capture: reference is non-vacuous (>= 20 lines)" \
  '[ "$(printf "%s\n" "$ac_body" | grep -c .)" -ge 20 ]'
# Scoped to the Materiality bar SECTION, not the whole file: a whole-file grep would match the
# clause wherever it landed, including a passing mention in the mint paragraph, which is not where
# the bar is applied. The section extractor gets its own non-vacuity anchor for the same reason.
ac_bar="$(awk '/^## Materiality bar/{f=1;next} /^## /{f=0} f' "$AC")"
assert "auto-capture: the Materiality bar section was located (non-vacuity anchor)" \
  '[ -n "$ac_bar" ]'
assert "auto-capture: work fixable by a small in-branch edit fails the bar" \
  'grep -qiF -- "in-branch" <<<"$ac_bar"'

# --- the PR body records a disposition, not a wishlist ------------------------
EP="$REPO/skills/docket-implement-next/references/edge-paths.md"
ep_body="$(cat "$EP" 2>/dev/null)"
assert "edge-paths: reference is non-vacuous (>= 15 lines)" \
  '[ "$(printf "%s\n" "$ep_body" | grep -c .)" -ge 15 ]'
assert "edge-paths: the PR body carries the findings disposition table" \
  'grep -qiF -- "disposition table" <<<"$ep_body"'
assert "edge-paths: no longer parks importants/minors for merge-time judgment alone" \
  '! grep -qiF -- "left for merge-time judgment" <<<"$ep_body"'

# --- Verify (human) is manual checks only -------------------------------------
RT="$REPO/skills/docket-implement-next/results-template.md"
rt_body="$(cat "$RT" 2>/dev/null)"
assert "results-template: is non-vacuous (>= 15 lines)" \
  '[ "$(printf "%s\n" "$rt_body" | grep -c .)" -ge 15 ]'
assert "results-template: Verify (human) excludes fixed findings" \
  'grep -qiE "fixed finding[^.]{0,80}(never|not)|(never|not)[^.]{0,80}fixed finding" <<<"$rt_body"'
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `bash tests/test_docket_review.sh 2>&1 | grep "NOT OK"`
Expected: `NOT OK - auto-capture: work fixable by a small in-branch edit fails the bar`, `NOT OK - edge-paths: the PR body carries the findings disposition table`, `NOT OK - edge-paths: no longer parks importants/minors…`, `NOT OK - results-template: Verify (human) excludes fixed findings`. The three non-vacuity asserts must **pass** already — if one fails, the file handle is wrong.

- [ ] **Step 3: Narrow the materiality bar**

In `skills/docket-convention/references/auto-capture.md`, replace the `## Materiality bar` section body (lines 18–20) with:

```markdown
Mint only for *actionable follow-up work that would be its own change / PR*
("would a human file this as a `docket-new-change`?"). A build lesson → the **learnings** harvest;
drift inside the current change → the **reconcile log**; a bare observation → the run report.

**Work fixable by a small in-branch edit fails the bar** (change 0218). A review finding about the
diff currently on the branch is **never mintable** — it is fixed in-branch or recorded in the PR
body, per `docket-implement-next`'s fix loop. A stub costs a title, an id, a groom, a spec, a plan,
a branch, a PR, and a close-out; a dead line of code costs one deletion, and routing the second
through the machinery built for the first is what made the backlog self-generating. Minting from
review survives only for genuinely distinct, beyond-the-branch work that clears the bar on its own.
```

- [ ] **Step 4: Make the PR body a disposition table**

In `skills/docket-implement-next/references/edge-paths.md`, in the `**Build-evidence block (change 0170).**` paragraph, replace the clause

```
alongside the review outcome — the rung that reviewed, blockers fixed, and any important or minor findings left for merge-time judgment.
```

with

```
alongside the review outcome — the rung that reviewed, and the **findings disposition table** (change 0218): one row per finding, each marked fixed (with its commit SHA), deferred, reverted, or recorded. The table's states are defined in `fix-loop.md`; do not redefine them here.
```

Leave the rest of that paragraph — the durable-home sentence, the marker validation rule, and the expected-staleness rule — byte-untouched.

- [ ] **Step 5: Narrow `## Verify (human)`**

In `skills/docket-implement-next/results-template.md`, replace the `## Verify (human)` comment line

```
<!-- Interactive/manual checks for the merge gate. Each item PENDING until checked. -->
```

with

```
<!-- GENUINELY MANUAL checks for the merge gate — things no automated test can reach. Each item
     PENDING until checked. A fixed finding never belongs here: the fix plus the green suite is its
     verification, and the PR body's disposition table is where its outcome is read. -->
```

- [ ] **Step 6: Check the budgets**

```bash
wc -l -w skills/docket-convention/references/auto-capture.md \
         skills/docket-implement-next/references/edge-paths.md \
         skills/docket-implement-next/results-template.md
```

Current budgets are `45 450`, `35 450`, and `24 172` respectively. Raise only a row the measured actual now exceeds, per the rounding rule, with a derivation comment naming change 0218 and what the added prose is. Do **not** raise a row prophylactically.

- [ ] **Step 7: Run tests to verify they pass**

Run: `bash tests/test_docket_review.sh && bash tests/test_skill_size_budgets.sh`
Expected: both print `PASS`.

- [ ] **Step 8: Mutation-test the new asserts**

```bash
# (a) the materiality clause must be inside the Materiality bar SECTION, not merely in the file
grep -c "in-branch" skills/docket-convention/references/auto-capture.md   # note the count
perl -0pi -e 's/\*\*Work fixable by a small in-branch edit fails the bar\*\* \(change 0218\)\./Work is minted on its merits./' skills/docket-convention/references/auto-capture.md
bash tests/test_docket_review.sh | grep "NOT OK - auto-capture: work fixable by a small in-branch"
git checkout skills/docket-convention/references/auto-capture.md

# (b) the section extractor must be proven live — rename the heading and watch the ANCHOR redden,
#     which is the inversion the non-vacuity companion exists for
perl -0pi -e 's/^## Materiality bar$/## The bar/m' skills/docket-convention/references/auto-capture.md
bash tests/test_docket_review.sh | grep "NOT OK - auto-capture: the Materiality bar section was located"
git checkout skills/docket-convention/references/auto-capture.md

# (c) the disposition-table assert must bite
perl -0pi -e 's/findings disposition table/findings summary/' skills/docket-implement-next/references/edge-paths.md
bash tests/test_docket_review.sh | grep "NOT OK - edge-paths: the PR body carries the findings disposition table"
git checkout skills/docket-implement-next/references/edge-paths.md
```

Mutation (b) is the one that matters most: if renaming the heading does **not** redden the anchor, the extractor is not scoped where its name claims and assert (a) is a whole-file grep in disguise.

- [ ] **Step 9: Commit**

```bash
git add skills/docket-convention/references/auto-capture.md \
        skills/docket-implement-next/references/edge-paths.md \
        skills/docket-implement-next/results-template.md \
        tests/test_docket_review.sh tests/test_skill_size_budgets.sh
git commit -m "feat(0218): narrow auto-capture and turn PR-body findings into a disposition table"
```

---

### Task 5: Ship the knob and the new behavior in README

**Files:**
- Modify: `README.md:739` (the findings-destination paragraph in `### docket-review`)
- Modify: `tests/test_docket_review.sh:195-206` (the README assert block)

**Interfaces:**
- Consumes: `REVIEW_MIN_FIX_SEVERITY` (Task 2), the fix-loop rules (Task 3), the disposition table (Task 4).
- Produces: nothing.

**Why this is its own task and not folded into Task 2:** the `config-knob-ship-end-to-end` learning is specifically about the surfacing half being dropped, and the README paragraph it must correct depends on Tasks 3 and 4 having landed. Folding it into Task 2 would have documented behavior that did not exist yet.

- [ ] **Step 1: Write the failing tests**

In `tests/test_docket_review.sh`, append to the README block that ends at line 206:

```bash
# --- change 0218: README documents the in-branch fix loop and its knob -------
# Scoped to the docket-review section, whose extraction is already anchored above by
# "README: the docket-review section was located (non-vacuity anchor)". A whole-README grep for
# "min_fix_severity" would be satisfied by any passing mention anywhere in a 900-line file.
assert "README: the fix loop replaced the record-everything rule" \
  '! grep -qiF -- "go into the PR body for merge-time judgment" <<<"$rm_body"'
assert "README: documents the in-branch fix loop" \
  'grep -qiF -- "fix loop" <<<"$rm_body"'
assert "README: documents the min_fix_severity knob" \
  'grep -qF -- "min_fix_severity" <<<"$rm_body"'
assert "README: states that fix routing never reaches max" \
  'grep -qiE "never[^.]{0,80}\`?max\`?" <<<"$rm_body"'
assert "README: states blockers are fixed regardless of the knob" \
  'grep -qiE "blocker[^.]{0,120}regardless" <<<"$rm_body"'
```

If the existing block does not already define `rm_body` scoped to the `### docket-review` section, define it alongside `RM` using the same extractor the existing non-vacuity anchor implies:

```bash
rm_body="$(awk '/^### docket-review/{f=1;next} /^### /{f=0} f' "$RM")"
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `bash tests/test_docket_review.sh 2>&1 | grep "NOT OK"`
Expected: `NOT OK - README: the fix loop replaced the record-everything rule`, `NOT OK - README: documents the in-branch fix loop`, `NOT OK - README: documents the min_fix_severity knob`, `NOT OK - README: states that fix routing never reaches max`, `NOT OK - README: states blockers are fixed regardless of the knob`.

- [ ] **Step 3: Rewrite the findings-destination paragraph**

In `README.md`, replace the paragraph at line 739 — currently:

```
Findings come back severity-tiered, and each tier has a fixed destination: a **blocker** is fixed before the PR opens, as a synthetic task through the same `docket-build-task` contract that wrote the code; **important** and **minor** findings go into the PR body for merge-time judgment; and anything that is distinct follow-up work rather than this change's own defect becomes an auto-captured stub. The reviewer returns the finding list and a one-line verdict — nothing else, no prose report.
```

with:

```markdown
Findings come back severity-tiered, and since change 0218 they are **fixed on the branch** rather than recorded and re-minted. `docket-implement-next` runs a bounded **fix loop** after review returns and before the PR opens: each finding becomes a task through the same `docket-build-task` contract that wrote the code, committed into the diff the human was going to read anyway, so the merge gate does not move. The reviewer itself is unchanged — it returns the finding list and a one-line verdict, nothing else, and never fixes anything.

Two axes, deliberately kept apart. A fix's **character** picks the model profile, using the same [routing rubric](skills/docket-build/references/task-routing.md) `docket-build` applies to plan tasks — so a subtle one-line fix is not handed to a cheap model just for being labelled minor. Severity picks only the **failure posture**: a blocker that cannot be fixed halts the run, while an important or minor that cannot be fixed falls back to a line in the PR body. Fix routing **never reaches the `max` profile** at any severity: `max` means irreversible, and an irreversible act must not happen to a branch as an unplanned side-quest discovered at review time.

The loop is bounded. One full-suite run after the fixes land; if it goes red, the non-blocker fix commits are reverted and the suite runs once more — green proceeds with those findings recorded unfixed, still-red halts. Two suite runs, no second repair chain, and no re-review round. The branch can never end worse than the green build that entered the loop. Every finding's outcome reaches the PR body as a **disposition table** — fixed (with its SHA), deferred, reverted, or recorded.

`review.min_fix_severity` (default `minor`, settable in any config layer) is the lowest severity that enters the loop: `important` records minors instead of fixing them, and `blocker` restores the pre-0218 behavior as a compat escape hatch. **Blockers are fixed regardless of it** — the knob can never disarm the one gate that must not be disarmed. One consequence worth stating: a review finding about the branch's own diff is no longer auto-captured as a stub at all. It is fixed or it is recorded; only genuinely distinct, beyond-the-branch work still mints one.
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `bash tests/test_docket_review.sh`
Expected: `PASS`.

- [ ] **Step 5: Mutation-test the new asserts**

```bash
# (a) the negative assert must bite — reintroduce the removed sentence
grep -c "go into the PR body for merge-time judgment" README.md   # expect 0
perl -0pi -e 's/(Two axes, deliberately kept apart\.)/Important and minor findings go into the PR body for merge-time judgment. $1/' README.md
grep -c "go into the PR body for merge-time judgment" README.md   # expect 1 — mutation landed
bash tests/test_docket_review.sh | grep "NOT OK - README: the fix loop replaced"
git checkout README.md

# (b) the section scoping must be real — move the knob paragraph OUT of the docket-review section
#     and the assert must still redden, proving it is section-scoped and not a whole-file grep
perl -0pi -e 's/`review\.min_fix_severity` \(default `minor`/`review.SOMETHINGELSE` (default `minor`/' README.md
bash tests/test_docket_review.sh | grep "NOT OK - README: documents the min_fix_severity knob"
git checkout README.md
```

Both greps must produce a line.

- [ ] **Step 6: Commit**

```bash
git add README.md tests/test_docket_review.sh
git commit -m "docs(0218): document the in-branch fix loop and review.min_fix_severity"
```

---

## Self-review

**1. Spec coverage.**

| Spec section | Task |
|---|---|
| Bounded fix loop inside Step 6, extended phase not a new role skill | 3 |
| Two orthogonal axes (character→profile, severity→posture) | 3 |
| Routing table (final form), including `max` → halt / PR-body record | 3 |
| `max` never dispatched from the fix loop; rubric doubles as the size ceiling | 3 |
| Shared rubric extraction to `docket-build/references/task-routing.md`, stub + pointer | 1 |
| Per-finding tasks and commits for blockers/importants; minors batched per profile | 3 |
| All fixes run `docket-build-task`, escalation truncated at premium | 3 |
| Revert-and-record suite gate, bounded at two runs | 3 |
| No re-review | 3 |
| `review.min_fix_severity` knob → `REVIEW_MIN_FIX_SEVERITY`, global-able, not fenced | 2 |
| Sample config, README, relaxed prose in the same change | 2 (example + contract doc), 5 (README) |
| PR-body findings become a disposition table | 3 (definition), 4 (edge-paths wiring) |
| `## Verify (human)` shrinks to genuinely manual checks | 4 |
| Auto-capture materiality bar gains the in-branch clause | 4 |
| Testing notes: Step 6 prose, extraction stub + pointer, export-order guard, two-consumer rubric, materiality clause | 1, 2, 3, 4, 5 |

No gaps. Out-of-scope items (ADR-0066, retroactive backlog clearing, `docket-integration-repair`) have no task, correctly.

**2. Placeholder scan.** The only deliberately unfilled values are the budget numbers `XX`/`YYY` and the measured `<L>`/`<W>` in Tasks 1, 3, and 4 — these are *measurements of files that do not exist until the task runs*, and each carries the exact `wc` command and the rounding rule that determines them. Everything else is literal content.

**3. Type / name consistency.**

- `REVIEW_MIN_FIX_SEVERITY` — identical in Task 2 (resolver, emit, contract doc, tests), Task 3 (Step 6 prose, `fix-loop.md`, asserts), Task 5 (README uses the YAML path `review.min_fix_severity`, which the README assert greps for as `min_fix_severity` — consistent).
- `review_key` mirrors `build_key`'s signature exactly: `review_key <leaf> <default>`.
- `skills/docket-build/references/task-routing.md` — the same path in Task 1 (creation, stub pointer, asserts), Task 3 (`fix-loop.md`'s relative link `../../docket-build/references/task-routing.md`, resolving correctly from `skills/docket-implement-next/references/`), and Task 5 (README link, repo-root-relative).
- `skills/docket-implement-next/references/fix-loop.md` — Task 1 declares the `IMPL_FIX` handle without asserting on it; Task 3 creates it and asserts via its own `FIX` handle in `test_docket_review.sh`. Deliberate, and flagged in Task 1 Step 1.
- Disposition states `fixed` / `deferred` / `reverted` / `recorded` — defined once in `fix-loop.md` (Task 3), referenced without redefinition in `edge-paths.md` (Task 4) and README (Task 5), and asserted as a set in Task 3's loop.
