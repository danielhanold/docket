<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0193 — Default the build and review roles to docket-build and docket-review](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-02-0193-default-build-review-roles-to-docket-owned-skills.md)**
<!-- docket:backlink:end -->

# Default the build and review roles to docket-build and docket-review — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Flip docket's two built-in workflow-skill defaults from the superpowers skills to docket's own `docket-build` and `docket-review`, and remove this repo's now-redundant `skills:` pin so it dogfoods the shipped default rather than an override on top of it.

**Architecture:** The whole behavioral change is two string literals in `scripts/docket-config.sh`'s `skill_role` call block — that function is the single place the default is decided, and every consumer reads the exported `SKILL_BUILD` / `SKILL_REVIEW`. Everything else in this plan is the end-to-end surface that must move with it: the repo's own config, the canonical `.docket.example.yml` mirror, four prose documents, and the test assertions that currently pin the *opt-in* posture this change deletes. Task order is chosen so the full suite is green at every task boundary — each task moves a producer and its guards together.

**Tech Stack:** Bash (POSIX-ish, run under `$DOCKET_BASH_PATH`), YAML config read by hand-rolled `grep`/`awk` block readers, Markdown docs, a hand-rolled `assert`-based shell test suite in `tests/test_*.sh`.

## Global Constraints

- **Run the whole suite at the build gate, never only the tests this plan enumerates** (AGENTS.md). The suite command is the repo's detected Bash-suite shape:
  ```bash
  for test in tests/test_*.sh; do "$DOCKET_BASH_PATH" "$test"; done
  ```
- **`grep` in your shell is ugrep, not BSD grep.** Verification greps in this repo must be run as `/usr/bin/grep` when portability matters; a green ugrep result is not evidence (learnings: `agent-shell-noop-reads-as-success`).
- **Never `producer | grep -q`** under `set -o pipefail` — capture into a variable, then `grep <<<"$var"` (AGENTS.md).
- **A guard is code: mutation-test it.** For every assert this plan inverts, confirm it goes red against the *pre-change* state before believing the post-change green (AGENTS.md; learnings: `assert-detects-removal-not-replacement`).
- **Assert the negative, not the replacement.** Where this change *removes* a claim ("the shipped default stays SDD"), the guard must detect the removed state's absence, not merely confirm the new wording's presence.
- **Cross-references in maintained source anchor on a symbol name or a verbatim-quoted clause — never a line number** (AGENTS.md, ADR-0054). Line numbers appear in this plan for navigation only; do not write any into the files you edit.
- **Do not touch `docs/adrs/`.** ADR-0063 and ADR-0066 need `## Update` notes recording that their opt-in-default *consequence* was overtaken, but ADRs live on the `docket` metadata branch, not this feature branch. That is delivered out-of-band by `docket-implement-next`'s step-6 `docket-adr` dispatch, riding the change's existing `adrs: [63, 66]` (learnings: `adr-update-delivery`). Adding ADR files here would be a metadata write in the feature worktree — forbidden.
- **Historical records are immutable**: archived changes, specs, plans, results, and existing ADR *bodies* keep whatever was true when written. Do not sweep them for the old default names.

## File Structure

| File | Responsibility in this change |
|---|---|
| `scripts/docket-config.sh` | **The only behavioral edit.** The `skill_role build …` / `skill_role review …` default arguments, plus the block comment above them that calls the fallback "the superpowers default". |
| `.docket.yml` | This repo's overrides. Loses its whole `skills:` block and the comment paragraph introducing it; keeps the sibling `build:` block. |
| `.docket.example.yml` | The canonical cross-harness reference. Its `skills:` template values, and the `build:` block header prose that frames `docket-build` as "the alternative … selected via `skills: build:`". |
| `scripts/docket-config.md` | The resolver's contract. Its `skills:` paragraph enumerates the five built-in defaults. |
| `README.md` | User-facing. The role table plus the two role sections (`### docket-build`, `### docket-review`) whose framing is "opt-in alternative". |
| `skills/docket-convention/SKILL.md` | The shared contract: the `.docket.yml` sample block and the *Skill layer* role table. |
| `skills/docket-implement-next/SKILL.md` | Three restatements of the defaults in steps 5 and 6, including the rung-default sentence that uses SDD's record-less-ness as its example. |
| `tests/test_docket_config.sh` | Default-resolution assertions (absent-block case, and the global-layer merge case). |
| `tests/test_docket_example_yml.sh` | The mirror guard mapping each export key to the value the example must document. |
| `tests/test_docket_build.sh` | Asserts the repo opts in and that the shipped default is still SDD — both premises this change deletes. |
| `tests/test_docket_review.sh` | Same shape for review: a dogfood-via-`.docket.yml` assert and a shipped-default-unchanged assert. |

---

### Task 1: Flip the resolver defaults

The behavioral core. Everything downstream is documentation of what this task decides.

**Files:**
- Modify: `scripts/docket-config.sh` — the `skill_role` call block and its preceding comment
- Test: `tests/test_docket_config.sh` (case G, case L), `tests/test_docket_build.sh` (the shipped-default assert)

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: `SKILL_BUILD=docket-build` and `SKILL_REVIEW=docket-review` on the `--export` surface when no config layer sets those leaves. Tasks 2–4 document exactly these two strings.

- [ ] **Step 1: Write the failing tests**

Three assert sites. In `tests/test_docket_config.sh`, case **(G)** currently reads:

```bash
assert "skills absent: BUILD default"      '[ "$SKILL_BUILD" = superpowers:subagent-driven-development ]'
assert "skills absent: REVIEW default"     '[ "$SKILL_REVIEW" = superpowers:requesting-code-review ]'
```

Replace those two lines with the new expectations *plus* explicit negatives, so the guard detects the state being removed rather than only confirming the replacement:

```bash
assert "skills absent: BUILD default"      '[ "$SKILL_BUILD" = docket-build ]'
assert "skills absent: REVIEW default"     '[ "$SKILL_REVIEW" = docket-review ]'
# The old superpowers defaults must be GONE, not merely shadowed — a resolver that emitted both
# (or fell through to SDD on some layer path) would satisfy a presence-only assert.
assert "skills absent: BUILD default is no longer SDD" \
  '[ "$SKILL_BUILD" != superpowers:subagent-driven-development ]'
assert "skills absent: REVIEW default is no longer superpowers review" \
  '[ "$SKILL_REVIEW" != superpowers:requesting-code-review ]'
```

Also fix the case-(G) section header comment, which currently claims five superpowers defaults:

```bash
# --- (G) skills: absent -> the five built-in defaults (three superpowers, two docket-owned) ---
```

In the same file, case **(L)** asserts an unset role falls through to the built-in default:

```bash
assert "0050 L: skills merge — unset role stays default"     '[ "$SKILL_BUILD" = superpowers:subagent-driven-development ]'
```

becomes:

```bash
assert "0050 L: skills merge — unset role stays default"     '[ "$SKILL_BUILD" = docket-build ]'
```

In `tests/test_docket_build.sh`, this block asserts the very posture the change reverses:

```bash
# The SHIPPED cross-harness default must stay SDD — the opt-in is this repo's, not everyone's.
# Anchored on the resolver, which is what actually decides the default.
sdd_default="$(grep -E 'SKILL_BUILD=|skill_role build' "$REPO/scripts/docket-config.sh")"
assert "shipped skills.build default is still superpowers SDD" \
  'grep -qF -- "superpowers:subagent-driven-development" <<<"$sdd_default"'
```

Its *premise* is deleted, but what it **guards** — that the shipped default is whatever docket intends, checked against the resolver rather than against prose — still holds, so invert it rather than delete it (learnings: `test-premise-deleted-not-regated`):

```bash
# The SHIPPED cross-harness default is now docket-build (change 0193) — every repo gets the
# profile-routed build with no opt-in. Anchored on the resolver, which is what actually decides
# the default, and asserted in BOTH directions so a revert to SDD reddens here rather than
# silently restoring the retired opt-in posture.
build_default="$(grep -E 'SKILL_BUILD=|skill_role build' "$REPO/scripts/docket-config.sh")"
assert "resolver's build default line was located (non-vacuity anchor)" '[ -n "$build_default" ]'
assert "shipped skills.build default is docket-build" \
  'grep -qF -- "docket-build" <<<"$build_default"'
assert "shipped skills.build default is no longer superpowers SDD" \
  '! grep -qF -- "superpowers:subagent-driven-development" <<<"$build_default"'
```

Add the matching resolver-anchored pair for review to `tests/test_docket_review.sh`, immediately above its existing `DY="$REPO/.docket.yml"` line — review previously had no resolver-anchored default guard at all, only an example-config one:

```bash
# The shipped cross-harness default is now docket-review (change 0193). Anchored on the resolver,
# both directions, mirroring the build guard in tests/test_docket_build.sh.
review_default="$(grep -E 'SKILL_REVIEW=|skill_role review' "$REPO/scripts/docket-config.sh")"
assert "resolver's review default line was located (non-vacuity anchor)" '[ -n "$review_default" ]'
assert "shipped skills.review default is docket-review" \
  'grep -qF -- "docket-review" <<<"$review_default"'
assert "shipped skills.review default is no longer superpowers review" \
  '! grep -qF -- "superpowers:requesting-code-review" <<<"$review_default"'
```

- [ ] **Step 2: Run the tests to verify they fail**

Run:
```bash
"$DOCKET_BASH_PATH" tests/test_docket_config.sh
"$DOCKET_BASH_PATH" tests/test_docket_build.sh
"$DOCKET_BASH_PATH" tests/test_docket_review.sh
```
Expected: all three FAIL. `test_docket_config.sh` fails on `skills absent: BUILD default` and `skills absent: REVIEW default` (still resolving to the superpowers names) and on the case-(L) merge assert. `test_docket_build.sh` fails on `shipped skills.build default is docket-build`. `test_docket_review.sh` fails on `shipped skills.review default is docket-review`. If any of them *passes* here, the edit did not land — re-check with `grep -c` before continuing.

- [ ] **Step 3: Flip the two defaults**

In `scripts/docket-config.sh`, the `skill_role` call block currently reads:

```bash
SKILL_BUILD="$(skill_role build superpowers:subagent-driven-development)"
SKILL_REVIEW="$(skill_role review superpowers:requesting-code-review)"
```

Change to:

```bash
SKILL_BUILD="$(skill_role build docket-build)"
SKILL_REVIEW="$(skill_role review docket-review)"
```

Leave `SKILL_BRAINSTORM`, `SKILL_PLAN`, and `SKILL_FINISH` exactly as they are — the other three roles keep their superpowers defaults.

The comment three lines above the `skill_role()` definition now misstates the fallback:

```bash
# Nested block; each leaf read within the block only. Per-key precedence:
# per-repo leaf > global leaf > the superpowers default.
```

Change its last clause to name the actual fallback without re-listing values (a list here would be a fifth restatement to keep in sync):

```bash
# Nested block; each leaf read within the block only. Per-key precedence:
# per-repo leaf > global leaf > the role's built-in default (superpowers for brainstorm/plan/
# finish, docket's own docket-build/docket-review for build/review since change 0193).
```

- [ ] **Step 4: Run the tests to verify they pass**

Run:
```bash
"$DOCKET_BASH_PATH" tests/test_docket_config.sh
"$DOCKET_BASH_PATH" tests/test_docket_build.sh
"$DOCKET_BASH_PATH" tests/test_docket_review.sh
```
Expected: all three PASS.

- [ ] **Step 5: Mutation-test the new negatives**

The negative asserts are the ones that can rot silently, so prove each one *can* fail. Temporarily revert `SKILL_BUILD`'s default to `superpowers:subagent-driven-development`, confirm the count actually changed, re-run, then restore:

```bash
grep -c 'skill_role build docket-build' scripts/docket-config.sh   # expect 1
perl -0pi -e 's/skill_role build docket-build/skill_role build superpowers:subagent-driven-development/' scripts/docket-config.sh
grep -c 'skill_role build docket-build' scripts/docket-config.sh   # expect 0 — the mutation LANDED
"$DOCKET_BASH_PATH" tests/test_docket_build.sh                     # expect FAIL
git checkout -- scripts/docket-config.sh
grep -c 'skill_role build docket-build' scripts/docket-config.sh   # expect 1 again
```

A mutation that leaves the suite green is a defect in the guard, not a pass. Repeat the same before/after count check for the review default if either negative assert looks fragile.

- [ ] **Step 6: Run the whole suite**

Run:
```bash
for test in tests/test_*.sh; do "$DOCKET_BASH_PATH" "$test"; done
```
Expected: every test PASSes. Task 1 changes only the resolver's default arguments, so `.docket.example.yml`'s mirror guard is still self-consistent (it compares the example against its own hardcoded expectations, not against the live export value) — that pair moves together in Task 2.

- [ ] **Step 7: Commit**

```bash
git add scripts/docket-config.sh tests/test_docket_config.sh tests/test_docket_build.sh tests/test_docket_review.sh
git commit -m "feat(0193): default skills.build to docket-build and skills.review to docket-review"
```

---

### Task 2: Move the canonical example config and its mirror guard

`.docket.example.yml` is the cross-harness reference every consuming repo copies from; its `skills:` block ships the defaults as live values, so it must state the new ones.

**Files:**
- Modify: `.docket.example.yml` — the `skills:` block values, and the `build:` block's header prose
- Test: `tests/test_docket_example_yml.sh` (the `map_for` regex table), `tests/test_docket_review.sh` (the shipped-default-in-example assert)

**Interfaces:**
- Consumes: the two default strings Task 1 produced (`docket-build`, `docket-review`).
- Produces: an example config whose documented defaults match the resolver, which is exactly what `test_docket_example_yml.sh`'s completeness loop checks.

- [ ] **Step 1: Write the failing tests**

In `tests/test_docket_example_yml.sh`, the `map_for` case arms:

```bash
    SKILL_BUILD)           echo '^[[:space:]]+build:[[:space:]]*superpowers:subagent-driven-development' ;;
    SKILL_REVIEW)          echo '^[[:space:]]+review:[[:space:]]*superpowers:requesting-code-review' ;;
```

become:

```bash
    SKILL_BUILD)           echo '^[[:space:]]+build:[[:space:]]*docket-build[[:space:]]*$' ;;
    SKILL_REVIEW)          echo '^[[:space:]]+review:[[:space:]]*docket-review[[:space:]]*$' ;;
```

The trailing `[[:space:]]*$` anchor matters: without it, `build:      docket-build-economy` would satisfy the pattern, so the anchor is what makes this assert about the *value* rather than a prefix of it.

In `tests/test_docket_review.sh`, this assert states the posture being reversed:

```bash
# The SHIPPED default must NOT move — the example config is the cross-harness default surface.
assert "the shipped default review binding is unchanged in the example config" \
  'grep -qE "^ +review: +superpowers:requesting-code-review$" "$REPO/.docket.example.yml"'
```

The block guards a real property — the example config is the cross-harness default surface and must agree with the resolver — so invert it, both directions:

```bash
# The example config is the cross-harness default surface, so it must state the SHIPPED default,
# which change 0193 moved to docket-review. Both directions: a revert of the example alone would
# leave it disagreeing with the resolver, which is exactly the drift this pair exists to catch.
assert "the example config states the shipped docket-review default" \
  'grep -qE "^ +review: +docket-review$" "$REPO/.docket.example.yml"'
assert "the example config no longer ships the superpowers review default" \
  '! grep -qE "^ +review: +superpowers:requesting-code-review$" "$REPO/.docket.example.yml"'
```

- [ ] **Step 2: Run the tests to verify they fail**

Run:
```bash
"$DOCKET_BASH_PATH" tests/test_docket_example_yml.sh
"$DOCKET_BASH_PATH" tests/test_docket_review.sh
```
Expected: both FAIL — `completeness: SKILL_BUILD present in example` and `completeness: SKILL_REVIEW present in example` in the first, `the example config states the shipped docket-review default` in the second.

- [ ] **Step 3: Update the example config**

In `.docket.example.yml`, the `skills:` block:

```yaml
skills:
  brainstorm: superpowers:brainstorming
  plan:       superpowers:writing-plans
  build:      superpowers:subagent-driven-development
  review:     superpowers:requesting-code-review
  finish:     superpowers:finishing-a-development-branch
```

becomes:

```yaml
skills:
  brainstorm: superpowers:brainstorming
  plan:       superpowers:writing-plans
  build:      docket-build
  review:     docket-review
  finish:     superpowers:finishing-a-development-branch
```

The comment paragraph above that block says unset keys "default to the superpowers skills shown, so an absent block is byte-identical to superpowers-everywhere". Both halves are now false. Replace those two clauses (keep the rest of the paragraph — the `auto` sentinel, verbatim passthrough, the degrade-with-warning rule, and the unknown-role rule are all unchanged):

```
# skills — (change 0049) rebinds any of the five workflow invocation points — brainstorm / plan /
# build / review / finish — to a different skill (the name passes to the Skill tool verbatim,
# unvalidated) or to `auto` (no skill; the running agent performs the step inline at its own
# model — e.g. `build: auto` to build with no fan-out at all). Unset keys default to the values
# shown: superpowers for brainstorm / plan / finish, and docket's own build and review roles
# since change 0193. To run the superpowers engine everywhere instead, set `build:
# superpowers:subagent-driven-development` and `review: superpowers:requesting-code-review`
# explicitly. A resolved-but-unavailable skill degrades to that role's `auto` fallback with a
# prominent warning. Unknown role keys are warned + ignored. Shape and per-role fallback
# artifacts: docket-convention "Skill layer".
```

Note the `build: auto` example lost its "without SDD" phrasing, which no longer describes what you are opting out of.

Then the `build:` block header, which frames `docket-build` as a selectable alternative:

```
# Knobs for docket's own build role, docket-build (change 0167) — the lean, profile-routed
# alternative to superpowers:subagent-driven-development, selected via `skills: build:` below.
# Inert unless that role is bound to docket-build.
```

becomes:

```
# Knobs for docket's own build role, docket-build (change 0167) — the lean, profile-routed build
# that has been the shipped default for `skills: build:` since change 0193. Inert if you rebind
# that role away from docket-build.
```

- [ ] **Step 4: Run the tests to verify they pass**

Run:
```bash
"$DOCKET_BASH_PATH" tests/test_docket_example_yml.sh
"$DOCKET_BASH_PATH" tests/test_docket_review.sh
```
Expected: both PASS.

- [ ] **Step 5: Mutation-test the anchored regex**

Prove the new `[[:space:]]*$` anchor is doing work, and that the mutation lands:

```bash
grep -cE '^ +build: +docket-build$' .docket.example.yml    # expect 1
perl -0pi -e 's/^  build:      docket-build$/  build:      docket-build-economy/m' .docket.example.yml
grep -cE '^ +build: +docket-build$' .docket.example.yml    # expect 0 — mutation LANDED
"$DOCKET_BASH_PATH" tests/test_docket_example_yml.sh        # expect FAIL
git checkout -- .docket.example.yml
```

- [ ] **Step 6: Run the whole suite**

Run:
```bash
for test in tests/test_*.sh; do "$DOCKET_BASH_PATH" "$test"; done
```
Expected: every test PASSes.

- [ ] **Step 7: Commit**

```bash
git add .docket.example.yml tests/test_docket_example_yml.sh tests/test_docket_review.sh
git commit -m "docs(0193): ship the new build/review defaults in the example config"
```

---

### Task 3: Drop this repo's redundant `skills:` pin

With the defaults flipped, docket's own `.docket.yml` override is a pin over the shipped value — the exact thing the change exists to remove, so the repo dogfoods the default rather than a duplicate of it.

**Files:**
- Modify: `.docket.yml` — delete the `skills:` block and its introducing comment paragraph
- Test: `tests/test_docket_build.sh` (the repo-opts-in assert), `tests/test_docket_review.sh` (the dogfood-via-`.docket.yml` assert)

**Interfaces:**
- Consumes: the resolver defaults from Task 1 — this task is only safe because an absent `skills:` block now resolves to the same two values.
- Produces: a `.docket.yml` with no `skills:` block; its `build: checkpoint: true` block is untouched and still asserted.

- [ ] **Step 1: Write the failing tests**

In `tests/test_docket_build.sh`:

```bash
assert "repo's skills: block extraction is non-vacuous" '[ -n "$skills_blk" ]'
assert "repo opts skills.build in to docket-build" \
  'grep -qE "^[[:space:]]+build:[[:space:]]+docket-build[[:space:]]*$" <<<"$skills_blk"'
```

The premise (this repo pins the role) is deleted, but the property it guards — *this repo actually runs `docket-build`* — still holds and is now delivered by the default. Invert to assert the absence of the pin, which is the state this change creates:

```bash
# Change 0193 made docket-build the shipped default, so this repo no longer pins skills.build —
# it genuinely runs the default rather than an override that happens to match. Asserting the
# block is ABSENT is what detects a re-added pin silently reintroducing the duplication.
assert "repo no longer pins skills.build (it runs the shipped default)" '[ -z "$skills_blk" ]'
```

Leave the two `build_blk` / `checkpoint` asserts immediately below completely alone — that is a different block of `.docket.yml` and it stays.

In `tests/test_docket_review.sh`:

```bash
DY="$REPO/.docket.yml"
assert "this repo dogfoods docket-review via .docket.yml" \
  'awk "/^skills:/{f=1;next} /^[a-z_]+:/{f=0} f" "$DY" | grep -qE "^ +review: +docket-review$"'
```

Same inversion. Note this line pipes an `awk` producer into `grep -q` — a `pipefail` SIGPIPE hazard (AGENTS.md); the replacement captures first:

```bash
DY="$REPO/.docket.yml"
dy_skills="$(awk "/^skills:/{f=1;next} /^[a-z_]+:/{f=0} f" "$DY")"
# Change 0193: docket-review is the shipped default, so this repo stops pinning it. The dogfood
# is now "we run what we ship", and the assert that proves it is the pin's ABSENCE.
assert "this repo no longer pins skills.review (it runs the shipped default)" '[ -z "$dy_skills" ]'
```

- [ ] **Step 2: Run the tests to verify they fail**

Run:
```bash
"$DOCKET_BASH_PATH" tests/test_docket_build.sh
"$DOCKET_BASH_PATH" tests/test_docket_review.sh
```
Expected: both FAIL — the `skills:` block is still present in `.docket.yml`, so both emptiness asserts fail.

- [ ] **Step 3: Delete the block from `.docket.yml`**

Remove this comment paragraph and the block beneath it in their entirety:

```yaml
# This repo dogfoods docket's own build AND review roles. Build (change 0167): each plan task is
# routed to a named economy/standard/premium profile agent instead of SDD's implementer+reviewer
# pairs. Review (change 0170): one bounded read-only whole-branch reviewer behind three pinned
# rungs, selected one above the build. The SHIPPED cross-harness defaults are unchanged —
# superpowers:subagent-driven-development and superpowers:requesting-code-review; see
# .docket.example.yml.
skills:
  build: docket-build
  review: docket-review
```

Replace it with a one-line note explaining the *absence*, so a future reader does not re-add the pin thinking it was dropped by accident:

```yaml
# No `skills:` block: since change 0193 docket-build and docket-review are the SHIPPED defaults,
# so this repo dogfoods them by running the default rather than pinning a copy of it.
```

Keep the `build:` block that follows (`checkpoint: true`) exactly as it is — `build.checkpoint` is a genuine override of the shipped `false`, unrelated to this change.

Verify only the intended block left:

```bash
/usr/bin/grep -n 'skills:\|checkpoint:' .docket.yml
```
Expected: no `skills:` line; `checkpoint: true` still present.

- [ ] **Step 4: Run the tests to verify they pass**

Run:
```bash
"$DOCKET_BASH_PATH" tests/test_docket_build.sh
"$DOCKET_BASH_PATH" tests/test_docket_review.sh
```
Expected: both PASS.

- [ ] **Step 5: Verify the resolver still yields the right values for THIS repo**

The point of the task is that removing the pin changes nothing about what this repo runs. Prove it directly rather than trusting the asserts:

```bash
"$DOCKET_SCRIPTS_DIR"/docket.sh env | /usr/bin/grep -E '^SKILL_(BUILD|REVIEW)='
```
Expected: exactly `SKILL_BUILD=docket-build` and `SKILL_REVIEW=docket-review`. If either shows a superpowers name, Task 1 did not land or a machine-local `.docket.local.yml` is shadowing it — stop and report rather than re-adding the pin.

- [ ] **Step 6: Run the whole suite**

Run:
```bash
for test in tests/test_*.sh; do "$DOCKET_BASH_PATH" "$test"; done
```
Expected: every test PASSes. Pay particular attention to any test that reads `.docket.yml` generically (config-layer and facade tests) — removing a top-level block can change what a block-scanning `awk` reads next.

- [ ] **Step 7: Commit**

```bash
git add .docket.yml tests/test_docket_build.sh tests/test_docket_review.sh
git commit -m "chore(0193): drop this repo's redundant skills: pin"
```

---

### Task 4: Retire the opt-in framing across the prose

Four documents describe the roles as opt-in alternatives to a superpowers default. Each needs its framing inverted while keeping the genuinely still-true content: how to opt *back* to superpowers, and the install/fresh-session requirement.

**Files:**
- Modify: `README.md` (role table; `### docket-build`; `### docket-review`)
- Modify: `scripts/docket-config.md` (the `skills:` paragraph)
- Modify: `skills/docket-convention/SKILL.md` (config sample; *Skill layer* role table)
- Modify: `skills/docket-implement-next/SKILL.md` (steps 5 and 6)
- Test: no new test file; the existing README asserts in `tests/test_docket_build.sh` and `tests/test_docket_review.sh` must stay green

**Interfaces:**
- Consumes: the two default strings from Task 1 and the example-config wording from Task 2.
- Produces: nothing consumed by a later task — this is the final task.

- [ ] **Step 1: Confirm which existing asserts constrain this prose**

Before editing, know what greps the text you are about to rewrite (learnings: `restatement-accumulates-its-own-guards` — asserts reach into whichever copy was nearest when written):

```bash
/usr/bin/grep -rn 'rm_body\|"\$RM"\|rvsec' tests/test_docket_build.sh tests/test_docket_review.sh
```

The constraints that must survive your edit:
- `README says how to opt back into SDD` greps the README for the literal `superpowers:subagent-driven-development`. **Keep that string in the README** — the opt-out instruction is still true and still wanted.
- `README states the shipped-defaults boundary for the profiles` greps case-insensitively for `docket-build` within 200 characters of `Claude Code, Cursor, and Codex`. Do not split that sentence across a longer gap.
- The `docket-review` section asserts run against `rvsec`, extracted by `awk "/^### docket-review/{f=1;next} /^### /{f=0} f"`. **Do not rename the `### docket-review` heading** — the extraction is anchored on it, and the non-vacuity anchor requires `build-evidence` to remain inside the section.
- Within that section, `one full-suite run when` and `three only when both` must both survive verbatim.

- [ ] **Step 2: Update the README role table**

```markdown
| `build` | `superpowers:subagent-driven-development` | `docket-implement-next` — execute the plan with TDD |
| `review` | `superpowers:requesting-code-review` | `docket-implement-next` — whole-branch review before the PR |
```

becomes:

```markdown
| `build` | `docket-build` | `docket-implement-next` — execute the plan task-by-task via profile-routed workers |
| `review` | `docket-review` | `docket-implement-next` — whole-branch review before the PR |
```

The paragraph below the table opens "Unset keys default to the superpowers skills above — an absent `skills:` map is byte-identical to superpowers-everywhere." Replace that sentence (keep the rest of the paragraph, which covers the `auto` degrade and points at docket-convention):

```markdown
Unset keys default to the skills shown above: superpowers for `brainstorm`, `plan`, and `finish`, and docket's own `docket-build` and `docket-review` for the two roles docket has since taken ownership of. To run the superpowers engine at every point, set `build: superpowers:subagent-driven-development` and `review: superpowers:requesting-code-review` explicitly.
```

Also fix the sentence introducing the table, which calls superpowers "the default engine":

```markdown
docket is a lifecycle wrapper around a workflow engine, and superpowers is the default engine for three of the five roles — docket owns the `build` and `review` roles itself. Each of the **five workflow invocation points is a pluggable role**: an optional `skills:` map in any config layer rebinds a role to a different skill (the name is passed to the Skill tool verbatim) or to the sentinel `auto` (no skill — the running agent performs the step inline at its own model).
```

- [ ] **Step 3: Reframe the README `### docket-build` section**

Its opening paragraph currently frames the role as an opt-in alternative. Replace the first paragraph with:

```markdown
The `build` role — the step in `docket-implement-next` that turns a written plan into commits — runs **`docket-build`**, docket's own build engine and the shipped default since change 0193. It dispatches one fresh worker per task and does no review of its own, leaving `skills.review` as docket's sole review gate: `T` nested runs on the clean path — one worker per task — plus one full-suite gate the controller runs itself as a Bash command rather than as a nested agent, against the `2T + 2` of `superpowers:subagent-driven-development`, the per-task implementer/reviewer pair it replaced. Each escalation adds at most one more nested run.
```

The "Select it in any config layer" block and its `skills: build: docket-build` example now document a no-op. Replace the block and the sentence after it with the opt-*out* direction:

```markdown
Nothing to select — it is the default. To opt back out to the superpowers engine, set it in any config layer:

```yaml
skills:
  build: superpowers:subagent-driven-development
```
```

The next paragraph begins "Opting in also means re-running `install.sh` and starting a fresh session". That requirement is now *stronger*, not weaker — every upgrading repo inherits the role and needs the generated profile agents. Reword its opening while keeping the rest of the paragraph intact:

```markdown
Because `docket-build` is now the default, a repo upgrading docket must re-run `install.sh` and start a fresh session: the installer is what generates the four profile agents and links the two build skills onto this machine, and every harness registers agent definitions only at process start
```

(continue with the existing text from "— until the harness hosting your session has, …" unchanged).

- [ ] **Step 4: Reframe the README `### docket-review` section**

Keep the `### docket-review` heading exactly as is. Replace its first paragraph:

```markdown
The `review` role — the step in `docket-implement-next` that reads the finished branch before the PR opens — runs **`docket-review`**, the shipped default since change 0193: one read-only reviewer contract behind three pinned rung wrappers — `docket-review-lean`, `docket-review-standard`, `docket-review-deep` — that share the contract and differ only in model and effort. The reviewer reads `git diff`, `git log`, and the tree; it never writes, never commits, never checks out, never dispatches a subagent, and never runs the test suite. It gets one shot at the rung it was dispatched at: there is no reviewer escalation ladder.
```

Replace the "Select it in any config layer" block the same way as Task 4 Step 3:

```markdown
Nothing to select — it is the default. To opt back out, set it in any config layer:

```yaml
skills:
  review: superpowers:requesting-code-review
```
```

Then the closing paragraph of the section, which is entirely about the retired asymmetric posture:

```markdown
The binding posture is deliberately asymmetric. The **shipped cross-harness default stays `superpowers:requesting-code-review`** — `.docket.example.yml` is unchanged, so nobody inherits this by upgrading. This repository dogfoods it by opting in through its own committed `.docket.yml`, and opting back out anywhere is one line: `skills: review: superpowers:requesting-code-review`. As with `docket-build`, opting in means re-running `install.sh` and starting a fresh session, since the three rung wrappers are generated by the installer and every harness registers agent definitions only at process start.
```

becomes:

```markdown
The binding posture used to be deliberately asymmetric — both roles shipped opt-in while this repository dogfooded them through its own committed `.docket.yml`. Change 0193 ended that: `docket-build` and `docket-review` are the shipped cross-harness defaults, `.docket.example.yml` states them as such, and this repository now carries no `skills:` block at all, so it runs exactly what everyone else does. As with `docket-build`, inheriting the role on upgrade means re-running `install.sh` and starting a fresh session, since the three rung wrappers are generated by the installer and every harness registers agent definitions only at process start.
```

The paragraph beginning "The rung is chosen **deterministically as one above the build**" and everything through the build-evidence discussion is unchanged — those describe mechanics, not binding posture.

- [ ] **Step 5: Update `scripts/docket-config.md`**

Its `skills:` paragraph reads "resolves **repo-local > repo-committed > global > superpowers default**" and then enumerates all five built-ins. Update both the chain label and the enumeration:

```markdown
**`skills:` (change 0049).** Reads the optional nested `skills:` block and emits
`SKILL_BRAINSTORM`, `SKILL_PLAN`, `SKILL_BUILD`, `SKILL_REVIEW`, `SKILL_FINISH`. Each leaf
resolves **repo-local > repo-committed > global > built-in default** — the repo-local
`.docket.local.yml`'s `skills:` block wins if the leaf is set there, else the per-repo
`.docket.yml`'s `skills:` block, else the global `config.yml`'s `skills:` block, else the
built-in default (`superpowers:brainstorming`, `superpowers:writing-plans`, `docket-build`,
`docket-review`, `superpowers:finishing-a-development-branch` — the build and review roles
became docket-owned defaults in change 0193); a set leaf is passed through verbatim (or the
sentinel `auto`). Leaves are read *within the block* (never as bare top-level keys). An unknown
role key under `skills:`, in any of the three layers, is warned on stderr and ignored — never
fatal.
```

- [ ] **Step 6: Update `skills/docket-convention/SKILL.md`**

Two sites. The `.docket.yml` config sample:

```yaml
  build:      superpowers:subagent-driven-development   # e.g. `auto` to build inline without SDD
  review:     superpowers:requesting-code-review
```

becomes:

```yaml
  build:      docket-build   # e.g. `auto` to build inline with no fan-out
  review:     docket-review
```

And the *Skill layer* role table:

```markdown
| build | `superpowers:subagent-driven-development` | `docket-implement-next` §5 | the plan executed on the feature branch |
| review | `superpowers:requesting-code-review` | `docket-implement-next` §6 | a whole-branch review before the PR opens, over a branch whose build evidence is green |
```

becomes:

```markdown
| build | `docket-build` | `docket-implement-next` §5 | the plan executed on the feature branch |
| review | `docket-review` | `docket-implement-next` §6 | a whole-branch review before the PR opens, over a branch whose build evidence is green |
```

The sentence introducing the section — "An unset key defaults to the superpowers skill — an absent map is byte-identical to pre-0049 behavior." — is now wrong on both halves. Replace with:

```markdown
An unset key defaults to the skill shown — superpowers for `brainstorm`/`plan`/`finish`, docket's own for `build`/`review` (change 0193).
```

- [ ] **Step 7: Update `skills/docket-implement-next/SKILL.md`**

Three sites. In step 5, the parenthetical `(default `superpowers:subagent-driven-development`)` becomes `(default `docket-build`)`, and the trailing clause "SDD does TDD + per-task review" no longer describes the default — replace it with "docket-build routes each task to a profile agent and gates on one full-suite run".

In step 6's review paragraph, `(default `superpowers:requesting-code-review`)` becomes `(default `docket-review`)`.

In step 6's rung-selection paragraph, this sentence uses SDD's record-less-ness as its example:

```markdown
When the resolved build skill emits **no build record at all** — the shipped default `superpowers:subagent-driven-development` emits none — the rung defaults to `docket-review-standard`, matching the uncertainty sink `standard` is in docket-build's own routing.
```

The **rule** survives and must not be dropped; only its example is stale, since the default build skill now *does* emit a record. Rewrite so the rule stands on its own:

```markdown
When the resolved build skill emits **no build record at all** — as any build role rebound away from `docket-build` may well do, `superpowers:subagent-driven-development` among them — the rung defaults to `docket-review-standard`, matching the uncertainty sink `standard` is in docket-build's own routing.
```

- [ ] **Step 8: Verify no stale opt-in framing survives**

```bash
/usr/bin/grep -rn 'opt-in alternative\|shipped default stays\|stays the shipped default\|byte-identical to superpowers-everywhere' README.md scripts/ skills/ .docket.example.yml .docket.yml
```
Expected: no matches. Then confirm the deliberately-retained opt-out strings are still there:

```bash
/usr/bin/grep -c 'superpowers:subagent-driven-development' README.md   # expect >= 1
/usr/bin/grep -c 'superpowers:requesting-code-review' README.md        # expect >= 1
```

- [ ] **Step 9: Check the skill size budgets**

`tests/test_skill_size_budgets.sh` gates SKILL.md sizes and you edited two of them:

```bash
"$DOCKET_BASH_PATH" tests/test_skill_size_budgets.sh
```
Expected: PASS. These edits are roughly size-neutral; if the budget is exceeded, tighten your own new wording rather than raising the budget.

- [ ] **Step 10: Run the whole suite**

Run:
```bash
for test in tests/test_*.sh; do "$DOCKET_BASH_PATH" "$test"; done
```
Expected: every test PASSes. Prose edits in this repo commonly redden vocabulary-presence and sentinel guards in tests you did not plan to touch (`test_convention_extraction.sh`, `test_skill_handoff_precedence.sh`, `test_composition_wiring.sh`, `test_dispatch_capability.sh` all grep skill prose). If one reddens, resolve it by **relocation** — repoint the assert at the artifact that owns the content, or correct the paraphrase — never by re-adding deleted text just to keep a grep green (learnings: `restatement-accumulates-its-own-guards`).

- [ ] **Step 11: Commit**

```bash
git add README.md scripts/docket-config.md skills/docket-convention/SKILL.md skills/docket-implement-next/SKILL.md
git commit -m "docs(0193): retire the opt-in framing for the build and review roles"
```

---

## Self-review

**Spec coverage.** The change body's five bullets map to tasks as follows: the resolver flip → Task 1; the `.docket.yml` removal → Task 3; the documentation sweep (`.docket.example.yml`, `README.md`, `scripts/docket-config.md`, both SKILL.md files) → Tasks 2 and 4; the test updates (`test_docket_config.sh`, `test_docket_example_yml.sh`, `test_docket_build.sh`, `test_docket_review.sh`) → distributed into the task that breaks each one, per the green-at-every-boundary rule; the ADR bullet → deliberately **not** a task here, delivered by the step-6 `docket-adr` dispatch onto the metadata branch (see Global Constraints).

**Open question.** The change's open question — whether a Codex/Cursor-shaped install still resolves sensibly on the new default — is answered by content that already exists and is already guarded: the README states all three harnesses ship complete sixteen-agent blocks in `agents/harness-defaults.yml`, and `tests/test_docket_build.sh` asserts that boundary sentence. No new work; Task 4 must simply not weaken that sentence, which Step 1 of that task calls out explicitly.

**Placeholder scan.** Every step names exact files and shows the exact before/after text. No "update the docs as appropriate" steps remain.

**Consistency.** The two produced strings — `docket-build` and `docket-review` — are used identically in all four tasks, in the resolver, the example config, the four prose documents, and every assert.
