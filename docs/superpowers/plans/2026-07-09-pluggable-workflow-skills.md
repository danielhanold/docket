# Pluggable Workflow Skills Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make docket's five hard-coded superpowers skill invocations (brainstorm, plan, build, review, finish) rebindable via a `skills:` map in `.docket.yml`, with an `auto` sentinel and a degrade-to-auto-on-missing rule, so a repo without the superpowers plugin works with zero config.

**Architecture:** `docket-config.sh --export` gains a nested `skills:` reader and emits five new `SKILL_*` variables (each defaulted to today's superpowers skill). The convention documents a new "Skill layer"; the four invoking skill bodies switch their invocation to the resolved skill and single-source the auto-fallback / missing-skill rule to the convention. Absent config is byte-identical to current behavior.

**Tech Stack:** Bash (`scripts/docket-config.sh`, awk/sed flat-YAML reader), hermetic bash test fixtures (`tests/test_*.sh`, temp repos + bare origins), Markdown skill bodies.

## Global Constraints

- **Absent `skills:` ⇒ byte-identical to pre-0049 behavior.** Every `SKILL_*` defaults to its superpowers skill; a repo with no `skills:` map (and no superpowers change elsewhere) resolves exactly today's five skills. (LEARNINGS #48: gate new behavior on explicit opt-in, never mere config presence; prove the minimal adopter is unaffected.)
- **Nested read, never bare leaf.** `skills:` leaves (`brainstorm`/`plan`/`build`/`review`/`finish`) are read *within the `skills:` block only* — never as bare top-level keys — so a future top-level `build:`/`review:` (e.g. #0044) cannot collide. (LEARNINGS #42.)
- **Whitespace-class safety.** The block reader must accept both tab- and space-indented config. Use `[[:space:]]`, never a literal-space class. Test tab-indented input explicitly. (LEARNINGS #46.)
- **Passthrough, unvalidated.** A `skills:` value is a skill name passed verbatim to the Skill tool; docket never validates it against a registry (ADR-0015 philosophy). Unknown *role keys* are warned-and-ignored (the `board_surfaces` posture) — a typo never aborts a run.
- **Emit order:** the five `SKILL_*` lines emit **before** `BOOTSTRAP`, which stays the last emitted line.
- **Prose references are out of scope.** The five superpowers mentions that are *not* invocations stay verbatim: `docket-implement-next` "re-brainstorming is a human act…", `docket-groom-next`/`docket-new-change` "do NOT continue to writing-plans", `docket-auto-groom` "do NOT invoke `superpowers:brainstorming`", `docket-status` "the same guard as `superpowers:finishing-a-development-branch`".
- **Existing gates untouched:** `docket-finalize-change`'s rebase-retest merge gate (`finalize.gate`) still validates the suite before any merge, regardless of the resolved build method.
- **Run a test file with** `bash tests/<file>.sh` (prints per-assert `ok -`/`NOT OK -`, ends `PASS`/`FAIL`, exit code = fail count). Full suite: `for t in tests/test_*.sh; do echo "== $t =="; bash "$t" || echo "FAILED: $t"; done`.

---

### Task 1: `skills:` resolution in `docket-config.sh` (+ contract + tests)

**Files:**
- Modify: `scripts/docket-config.sh` (add `yaml_block_body`; resolve five `SKILL_*`; emit them; warn on unknown role keys)
- Modify: `scripts/docket-config.md` (document the five vars + `skills:` parsing)
- Test: `tests/test_docket_config.sh` (update the count assertion; add skills fixtures)

**Interfaces:**
- Produces (emitted `KEY=value` lines, consumed by every skill's Step-0 `eval "$(docket-config.sh --export)"`): `SKILL_BRAINSTORM`, `SKILL_PLAN`, `SKILL_BUILD`, `SKILL_REVIEW`, `SKILL_FINISH`. Defaults respectively: `superpowers:brainstorming`, `superpowers:writing-plans`, `superpowers:subagent-driven-development`, `superpowers:requesting-code-review`, `superpowers:finishing-a-development-branch`.

- [ ] **Step 1: Update the count assertion + add failing skills fixtures**

In `tests/test_docket_config.sh`, change the line-count assertion (currently 13) to 18:

```bash
n="$(run "$tmp/c" --export | grep -c '=')"
assert "direct-pipe: 18 KEY=value lines emitted"       '[ "$n" -eq 18 ]'
```

Then append, immediately before the final `if [ "$fail" = 0 ]` block, new fixtures (G)–(J):

```bash
# --- (G) skills: absent -> five superpowers defaults (byte-identical behavior) ---
mkrepo "$tmp/g"
printf 'metadata_branch: main\n' > "$tmp/g/.docket.yml"
git -C "$tmp/g" add .docket.yml; git -C "$tmp/g" commit --quiet -m cfg; git -C "$tmp/g" push --quiet origin main
out="$(run "$tmp/g" --export)"; eval "$out"
assert "skills absent: BRAINSTORM default" '[ "$SKILL_BRAINSTORM" = superpowers:brainstorming ]'
assert "skills absent: PLAN default"       '[ "$SKILL_PLAN" = superpowers:writing-plans ]'
assert "skills absent: BUILD default"      '[ "$SKILL_BUILD" = superpowers:subagent-driven-development ]'
assert "skills absent: REVIEW default"     '[ "$SKILL_REVIEW" = superpowers:requesting-code-review ]'
assert "skills absent: FINISH default"     '[ "$SKILL_FINISH" = superpowers:finishing-a-development-branch ]'

# --- (H) skills: explicit overrides incl. `auto`, a custom name, and a partial map ---
mkrepo "$tmp/h"
cat > "$tmp/h/.docket.yml" <<'EOF'
metadata_branch: main
skills:
  build: auto
  review: my-org:custom-review
  brainstorm: superpowers:brainstorming
EOF
git -C "$tmp/h" add .docket.yml; git -C "$tmp/h" commit --quiet -m cfg; git -C "$tmp/h" push --quiet origin main
out="$(run "$tmp/h" --export)"; eval "$out"
assert "skills auto: BUILD is auto"         '[ "$SKILL_BUILD" = auto ]'
assert "skills custom: REVIEW verbatim"     '[ "$SKILL_REVIEW" = my-org:custom-review ]'
assert "skills partial: PLAN still default" '[ "$SKILL_PLAN" = superpowers:writing-plans ]'

# --- (I) skills: TAB-indented block parses (LEARNINGS #46 — whitespace class) ---
mkrepo "$tmp/i"
printf 'metadata_branch: main\nskills:\n\tplan: auto\n' > "$tmp/i/.docket.yml"
git -C "$tmp/i" add .docket.yml; git -C "$tmp/i" commit --quiet -m cfg; git -C "$tmp/i" push --quiet origin main
out="$(run "$tmp/i" --export)"; eval "$out"
assert "skills tab-indent: PLAN auto"       '[ "$SKILL_PLAN" = auto ]'

# --- (J) skills: unknown role key -> warned on stderr, ignored; known keys still resolve ---
mkrepo "$tmp/j"
printf 'metadata_branch: main\nskills:\n  bogus: x\n  plan: auto\n' > "$tmp/j/.docket.yml"
git -C "$tmp/j" add .docket.yml; git -C "$tmp/j" commit --quiet -m cfg; git -C "$tmp/j" push --quiet origin main
jerr="$(run "$tmp/j" --export 2>&1 >/dev/null)"
out="$(run "$tmp/j" --export 2>/dev/null)"; eval "$out"
assert "skills unknown key: warned on stderr"       'printf "%s" "$jerr" | grep -qi "unknown skills role"'
assert "skills unknown key: known PLAN still parsed" '[ "$SKILL_PLAN" = auto ]'
assert "skills unknown key: does not abort (exit 0)" '[ "$(run_rc "$tmp/j" --export)" -eq 0 ]'
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `bash tests/test_docket_config.sh`
Expected: FAIL — `NOT OK` on the new skills asserts (`SKILL_*` unset ⇒ empty) and on `18 KEY=value lines` (still 13).

- [ ] **Step 3: Add the nested-block reader**

In `scripts/docket-config.sh`, immediately after the `yaml_get() { … }` function (ends at the line with the closing `}` after the `sed … head -n1 …` pipeline), add:

```bash
# Emit the indented child lines of a top-level block key (block-style YAML only). Used for the
# nested `skills:` map (change 0049) so each leaf is read WITHIN the block via yaml_get — never as
# a bare top-level key, which a future top-level `build:`/`review:` could otherwise shadow.
# Comment-strips each line (matching yaml_get semantics) so a trailing comment on `skills:` or a
# full-line comment inside the block can't fool block detection. `[[:space:]]` => tab OR space.
yaml_block_body() {  # yaml_block_body <file> <top-level-key>  -> child lines on stdout
  [ -f "$1" ] || return 0
  awk -v parent="$2" '
    { line=$0; sub(/[[:space:]]*#.*/, "", line) }
    line ~ ("^" parent "[[:space:]]*:[[:space:]]*$") { inblk=1; next }
    inblk && line ~ /^[^[:space:]]/ { inblk=0 }
    inblk { print }
  ' "$1"
}
```

- [ ] **Step 4: Resolve and warn**

In `scripts/docket-config.sh`, immediately after the `board_surfaces` resolution block (the `if [ -z "$bs_raw" ] … fi`, ending at line ~107), add:

```bash
# --- skills: role-keyed pluggable workflow skills (change 0049) ---------------
# Nested block; each leaf read within the block only. Unset leaf => the superpowers default.
SKILLS_BLK="$(mktemp)"; yaml_block_body "$CFG" skills >"$SKILLS_BLK"
SKILL_BRAINSTORM="$(yaml_get "$SKILLS_BLK" brainstorm)"; SKILL_BRAINSTORM="${SKILL_BRAINSTORM:-superpowers:brainstorming}"
SKILL_PLAN="$(yaml_get "$SKILLS_BLK" plan)";             SKILL_PLAN="${SKILL_PLAN:-superpowers:writing-plans}"
SKILL_BUILD="$(yaml_get "$SKILLS_BLK" build)";           SKILL_BUILD="${SKILL_BUILD:-superpowers:subagent-driven-development}"
SKILL_REVIEW="$(yaml_get "$SKILLS_BLK" review)";         SKILL_REVIEW="${SKILL_REVIEW:-superpowers:requesting-code-review}"
SKILL_FINISH="$(yaml_get "$SKILLS_BLK" finish)";         SKILL_FINISH="${SKILL_FINISH:-superpowers:finishing-a-development-branch}"
# Unknown role keys: warn-and-ignore (a typo must never abort — the board_surfaces posture).
while IFS= read -r _role; do
  [ -n "$_role" ] || continue
  case " brainstorm plan build review finish " in
    *" $_role "*) ;;
    *) printf 'docket-config: warning: unknown skills role %s — ignored\n' "$_role" >&2 ;;
  esac
done < <(sed -n -E 's/^[[:space:]]*([[:alnum:]_-]+)[[:space:]]*:.*/\1/p' "$SKILLS_BLK")
rm -f "$SKILLS_BLK"
```

- [ ] **Step 5: Emit the five variables (before `BOOTSTRAP`)**

In the `--- emit ---` block, between `emit AUTO_GROOM "$AUTO_GROOM"` and `emit BOOTSTRAP "$BOOTSTRAP"`, insert:

```bash
  emit SKILL_BRAINSTORM "$SKILL_BRAINSTORM"
  emit SKILL_PLAN "$SKILL_PLAN"
  emit SKILL_BUILD "$SKILL_BUILD"
  emit SKILL_REVIEW "$SKILL_REVIEW"
  emit SKILL_FINISH "$SKILL_FINISH"
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `bash tests/test_docket_config.sh`
Expected: PASS (all `ok -`, final `PASS`, exit 0). Confirms defaults, `auto`, custom name, partial map, tab-indent, unknown-key warning, and the 18-line count.

- [ ] **Step 7: Update the script contract**

In `scripts/docket-config.md`, add the five `SKILL_*` keys to the emitted-`KEY=value` contract list, and add a short "skills:" paragraph to the Behavior section:

> **`skills:` (change 0049).** Reads the optional nested `skills:` block and emits `SKILL_BRAINSTORM`, `SKILL_PLAN`, `SKILL_BUILD`, `SKILL_REVIEW`, `SKILL_FINISH`. Each unset leaf defaults to its superpowers skill (`superpowers:brainstorming`, `superpowers:writing-plans`, `superpowers:subagent-driven-development`, `superpowers:requesting-code-review`, `superpowers:finishing-a-development-branch`); a set leaf is passed through verbatim (or the sentinel `auto`). Leaves are read *within the block* (never as bare top-level keys). An unknown role key under `skills:` is warned on stderr and ignored — never fatal.

Then run: `bash tests/test_script_contracts_coverage.sh`
Expected: PASS (contract coverage still satisfied).

- [ ] **Step 8: Commit**

```bash
git add scripts/docket-config.sh scripts/docket-config.md tests/test_docket_config.sh
git commit -m "feat(0049): resolve skills: map in docket-config.sh (five SKILL_* vars)"
```

---

### Task 2: Convention "Skill layer" section + `.docket.yml` example

**Files:**
- Modify: `skills/docket-convention/SKILL.md` (new "Skill layer" section; add `skills:` to the `.docket.yml` example block)
- Test: `tests/test_docket_config.sh` (extend the skill-wiring sentinel block to assert the convention documents the Skill layer)

**Interfaces:**
- Consumes: the `SKILL_*` variable names emitted by Task 1.
- Produces: the single-sourced auto-fallback artifact table + missing-skill rule that Tasks 3–4's skill bodies reference by name ("per the convention's *Skill layer*").

- [ ] **Step 1: Write the failing sentinel**

In `tests/test_docket_config.sh`, in the skill-wiring block (near the `CONV=…` asserts), add:

```bash
assert "convention documents the Skill layer" 'grep -qF "Skill layer" "$CONV"'
assert "convention names SKILL_ resolution vars" \
  'grep -qF "SKILL_BRAINSTORM" "$CONV" && grep -qF "SKILL_FINISH" "$CONV"'
assert "convention documents the auto sentinel + degrade rule" \
  'grep -qiF "degrade to auto" "$CONV"'
```

Run: `bash tests/test_docket_config.sh`
Expected: FAIL — `NOT OK` on the three new convention asserts.

- [ ] **Step 2: Add the `skills:` block to the `.docket.yml` example**

In `skills/docket-convention/SKILL.md`, inside the fenced `.docket.yml` example (the block that already shows `metadata_branch`, `agents:`, …), add after the `agents:` line group:

```yaml
skills:                      # pluggable workflow skills (change 0049); unset key = the superpowers default shown
  brainstorm: superpowers:brainstorming
  plan:       superpowers:writing-plans
  build:      superpowers:subagent-driven-development   # e.g. `auto` to build inline without SDD
  review:     superpowers:requesting-code-review
  finish:     superpowers:finishing-a-development-branch
```

- [ ] **Step 3: Add the "Skill layer" section**

In `skills/docket-convention/SKILL.md`, immediately after the "Agent layer" section (before "Directory layout"), add:

````markdown
### Skill layer — pluggable workflow skills (change 0049)

docket's workflow quality rests on five superpowers skill invocations. Each is a **pluggable
role**: the optional `skills:` map in `.docket.yml` rebinds it to a different skill, or to the
sentinel `auto` (no skill — the running agent performs the step inline at its own model). An unset
key defaults to the superpowers skill, so an absent `skills:` map is byte-identical to pre-0049
behavior.

| Role | Default skill | Invoked by | `auto` / fallback artifact — stop-point |
|---|---|---|---|
| brainstorm | `superpowers:brainstorming` | `docket-new-change` §2, `docket-groom-next` | a spec file at the configured spec path; stop at the spec |
| plan | `superpowers:writing-plans` | `docket-implement-next` §4 | a plan file on the feature branch, recorded in `plan:` |
| build | `superpowers:subagent-driven-development` | `docket-implement-next` §5 | the plan executed on the feature branch |
| review | `superpowers:requesting-code-review` | `docket-implement-next` §6 | a whole-branch review before the PR opens |
| finish | `superpowers:finishing-a-development-branch` | `docket-implement-next` §7; `docket-finalize-change` close-out | a pushed feature branch + open PR — never merged; stop |

- **Passthrough.** A value is a skill name passed verbatim to the Skill tool — docket never
  validates it against a registry (the ADR-0015 harness-neutral-passthrough philosophy; the
  passthrough is exactly what lets any third-party or in-repo skill plug in). Unknown *role keys*
  are warned-and-ignored (the `board_surfaces` posture — a typo never aborts a run).
- **`auto` sentinel.** No skill is invoked; the running agent does the step itself at whatever
  model it already runs at (the wrapper-resolved model for a subagent, the session model inline).
  The per-role fallback defines only the **final artifact / stop-point** (column 4) — never the
  method (no mandated TDD, dialogue shape, question cadence, or commit granularity).
- **Missing-skill rule — degrade to auto + warn.** If the resolved skill cannot be invoked at
  runtime (superpowers not installed, plugin unavailable, a typo'd custom name), the invoking skill
  **degrades to that role's `auto` fallback and warns prominently** — in the run output, and (for
  the build-time roles: plan/build/review/finish) as a note in the PR body. A repo with no
  superpowers plugin therefore works out of the box with zero config. This is softer than the
  autonomous abort-and-report rule because skill availability is a per-machine property, not a
  repo-state error — aborting would make docket unusable for exactly the users this serves.
- **Resolution** is deterministic via `docket-config.sh --export`, which emits `SKILL_BRAINSTORM`,
  `SKILL_PLAN`, `SKILL_BUILD`, `SKILL_REVIEW`, `SKILL_FINISH` (each defaulted when the key is unset).
  Consuming skill bodies read the variable from their Step-0 export; none re-parse YAML. Existing
  gates are untouched — `docket-finalize-change`'s rebase-retest merge gate (`finalize.gate`) still
  validates the suite before any merge regardless of the resolved build method.
````

- [ ] **Step 4: Run the tests to verify they pass**

Run: `bash tests/test_docket_config.sh && bash tests/test_convention_extraction.sh`
Expected: both PASS (new convention sentinels satisfied; convention-extraction unaffected).

- [ ] **Step 5: Commit**

```bash
git add skills/docket-convention/SKILL.md tests/test_docket_config.sh
git commit -m "feat(0049): document the Skill layer in docket-convention"
```

---

### Task 3: Wire the brainstorm sites to `SKILL_BRAINSTORM`

**Files:**
- Modify: `skills/docket-new-change/SKILL.md` (step 2 brainstorm invocation)
- Modify: `skills/docket-groom-next/SKILL.md` (the brainstorm invocation line)
- Test: `tests/test_groom_recap.sh` (update the brainstorm-line anchor); `tests/test_docket_config.sh` (add brainstorm wiring sentinel)

**Interfaces:**
- Consumes: `SKILL_BRAINSTORM` (Task 1); the convention "Skill layer" (Task 2).

- [ ] **Step 1: Update the groom-recap anchor + add wiring sentinels (failing)**

In `tests/test_groom_recap.sh`, replace the brainstorm-line locator (line ~31) so it no longer depends on the old literal:

```bash
brainstorm_line="$(grep -nF "resolved brainstorm skill" "$SKILL" | head -1 | cut -d: -f1)"
```

In `tests/test_docket_config.sh` skill-wiring block, add:

```bash
assert "new-change brainstorm uses SKILL_BRAINSTORM" \
  'grep -qF "SKILL_BRAINSTORM" "$REPO/skills/docket-new-change/SKILL.md"'
assert "groom-next brainstorm uses SKILL_BRAINSTORM" \
  'grep -qF "SKILL_BRAINSTORM" "$REPO/skills/docket-groom-next/SKILL.md"'
```

Run: `bash tests/test_groom_recap.sh; bash tests/test_docket_config.sh`
Expected: `test_groom_recap.sh` FAILs its "recap comes before the brainstorm invocation" assert (anchor now finds nothing → `brainstorm_line` empty); `test_docket_config.sh` FAILs the two new wiring asserts.

- [ ] **Step 2: Switch `docket-groom-next` brainstorm to the resolved skill**

In `skills/docket-groom-next/SKILL.md`, replace the sentence that begins "Then run `` `superpowers:brainstorming` `` WITH THE HUMAN, seeded with…" with:

> Then run the **resolved brainstorm skill** — `$SKILL_BRAINSTORM` from the Step-0 config export (default `superpowers:brainstorming`) — WITH THE HUMAN, seeded with the stub's body and its `## Open questions` — the open questions are the session's starting agenda. If it resolves to `auto` or cannot be invoked, apply the brainstorm auto-fallback per the convention's *Skill layer* (design inline with the human, warning prominently on unavailability) — the artifact is unchanged: a spec, then stop. STOP AT THE SPEC — do NOT continue to `superpowers:writing-plans` (planning is build-time, owned by `docket-implement-next`).

(The "do NOT continue to `superpowers:writing-plans`" prose reference is preserved verbatim.)

- [ ] **Step 3: Switch `docket-new-change` step 2 brainstorm to the resolved skill**

In `skills/docket-new-change/SKILL.md`, replace the opening of step 2 — "**Brainstorm** — run `` `superpowers:brainstorming` `` WITH THE HUMAN. This is the decision point." — with:

> 2. **Brainstorm** — run the **resolved brainstorm skill** — `$SKILL_BRAINSTORM` from the Step-0 config export (default `superpowers:brainstorming`) — WITH THE HUMAN. This is the decision point. If it resolves to `auto` or cannot be invoked, apply the brainstorm auto-fallback per the convention's *Skill layer* (design inline with the human, warning on unavailability); the artifact is unchanged: a spec, then stop.

Keep the remainder of step 2 (STOP AT THE SPEC, "do NOT continue to `writing-plans`", the `.docket/docs/superpowers/specs/…` path, `spec:` recording, and the `render-change-links.sh` regen) verbatim.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `bash tests/test_groom_recap.sh && bash tests/test_docket_config.sh`
Expected: both PASS — recap still precedes the (reworded) brainstorm line; both brainstorm sites name `SKILL_BRAINSTORM`.

- [ ] **Step 5: Commit**

```bash
git add skills/docket-new-change/SKILL.md skills/docket-groom-next/SKILL.md tests/test_groom_recap.sh tests/test_docket_config.sh
git commit -m "feat(0049): brainstorm sites invoke the resolved SKILL_BRAINSTORM"
```

---

### Task 4: Wire `docket-implement-next` (plan/build/review/finish) + `docket-finalize-change` finish

**Files:**
- Modify: `skills/docket-implement-next/SKILL.md` (steps 4/5/6/7 invocations)
- Modify: `skills/docket-finalize-change/SKILL.md` (the finish-skill invocation in the non-standard close-out)
- Test: `tests/test_docket_config.sh` (add plan/build/review/finish wiring sentinels)

**Interfaces:**
- Consumes: `SKILL_PLAN`, `SKILL_BUILD`, `SKILL_REVIEW`, `SKILL_FINISH` (Task 1); the convention "Skill layer" (Task 2).

- [ ] **Step 1: Add wiring sentinels (failing)**

In `tests/test_docket_config.sh` skill-wiring block, add:

```bash
IMPL="$REPO/skills/docket-implement-next/SKILL.md"
assert "implement-next plan uses SKILL_PLAN"     'grep -qF "SKILL_PLAN" "$IMPL"'
assert "implement-next build uses SKILL_BUILD"   'grep -qF "SKILL_BUILD" "$IMPL"'
assert "implement-next review uses SKILL_REVIEW" 'grep -qF "SKILL_REVIEW" "$IMPL"'
assert "implement-next finish uses SKILL_FINISH" 'grep -qF "SKILL_FINISH" "$IMPL"'
assert "finalize finish uses SKILL_FINISH" \
  'grep -qF "SKILL_FINISH" "$REPO/skills/docket-finalize-change/SKILL.md"'
```

Run: `bash tests/test_docket_config.sh`
Expected: FAIL — `NOT OK` on the five new wiring asserts.

- [ ] **Step 2: Switch step 4 (plan)**

In `skills/docket-implement-next/SKILL.md` step 4, replace "Run `` `superpowers:writing-plans` ``: it performs an intentional **cross-tree** step —" with:

> Run the **resolved plan skill** — `$SKILL_PLAN` from the Step-0 config export (default `superpowers:writing-plans`; on `auto` or unavailability, apply the plan auto-fallback per the convention's *Skill layer* — author the plan file yourself, warning prominently). The resolved plan step performs an intentional **cross-tree** step —

Leave the rest of the sentence (the cross-tree spec-read / plan-write explanation, the `plan:` recording, and the `render-change-links.sh` regen) verbatim.

- [ ] **Step 3: Switch step 5 (build)**

In `skills/docket-implement-next/SKILL.md` step 5, replace "`` `superpowers:subagent-driven-development` `` executes the plan task-by-task with TDD + per-task review." with:

> The **resolved build skill** — `$SKILL_BUILD` from the Step-0 config export (default `superpowers:subagent-driven-development`) — executes the plan task-by-task; SDD does TDD + per-task review. On `auto` or unavailability, apply the build auto-fallback per the convention's *Skill layer* (execute the plan on the feature branch, warning prominently) — the artifact is the executed plan; method is the agent's choice.

- [ ] **Step 4: Switch step 6 (review)**

In `skills/docket-implement-next/SKILL.md` step 6, replace the leading "`` `superpowers:requesting-code-review` `` (whole-branch);" with:

> The **resolved review skill** — `$SKILL_REVIEW` from the Step-0 config export (default `superpowers:requesting-code-review`) — whole-branch; on `auto` or unavailability, apply the review auto-fallback per the convention's *Skill layer* (a whole-branch review before the PR opens, warning prominently).

Leave the remainder of step 6 (re-read `LEARNINGS.md`, the `docket-adr` dispatch, `adrs:` update, `render-change-links.sh`) verbatim.

- [ ] **Step 5: Switch step 7 (finish)**

In `skills/docket-implement-next/SKILL.md` step 7, replace "Invoke `` `superpowers:finishing-a-development-branch` ``, DIRECTED to: push the feature branch and open a PR — do NOT merge — then stop." with:

> Invoke the **resolved finish skill** — `$SKILL_FINISH` from the Step-0 config export (default `superpowers:finishing-a-development-branch`) — DIRECTED to: push the feature branch and open a PR — do NOT merge — then stop. On `auto` or unavailability, apply the finish auto-fallback per the convention's *Skill layer* (push the branch and open the PR, never merging, then stop) and note the degrade in the PR body.

Leave "Pre-specifying the outcome keeps it non-interactive while reusing its push/PR mechanics." verbatim.

- [ ] **Step 6: Switch `docket-finalize-change` finish invocation**

In `skills/docket-finalize-change/SKILL.md`, in the "When a human is present, `` `superpowers:finishing-a-development-branch` `` can drive a **non-standard close-out**…" sentence, change only the *invocation* clause to the resolved skill, keeping the *provenance-guard* prose reference naming the concrete skill:

> When a human is present, the **resolved finish skill** — `$SKILL_FINISH` (default `superpowers:finishing-a-development-branch`) — can drive a **non-standard close-out** (keep the branch, discard it, or merge locally without a PR) — its merge/keep/discard chooser fits naturally at step 4. docket also borrows `superpowers:finishing-a-development-branch`'s **worktree provenance-guard**: only auto-remove a worktree whose path is under `.worktrees/<slug>` — never remove a worktree outside that known path.

- [ ] **Step 7: Run the tests to verify they pass**

Run: `bash tests/test_docket_config.sh`
Expected: PASS — all five wiring asserts satisfied; the pre-existing `/docket-config.sh` Step-0 sentinels still green.

- [ ] **Step 8: Commit**

```bash
git add skills/docket-implement-next/SKILL.md skills/docket-finalize-change/SKILL.md tests/test_docket_config.sh
git commit -m "feat(0049): implement-next + finalize invoke resolved plan/build/review/finish skills"
```

---

### Task 5: Close open questions + full regression

**Files:** (verification only; a fix here only if a wrapper directive names a superpowers skill)
- Possibly modify: `sync-agents.sh` / `agents/docket-*.md` (only if the grep below finds a superpowers skill name baked into a generated wrapper directive)

- [ ] **Step 1: Verify generated agent-wrapper directives name no superpowers skill (spec open question #2)**

Run:

```bash
grep -rn -E 'superpowers:(brainstorming|writing-plans|subagent-driven-development|requesting-code-review|finishing-a-development-branch)' \
  sync-agents.sh agents/ 2>/dev/null
```

Expected: no output (wrappers inject only `docket-convention` + a one-line directive; they name no superpowers skill). If there IS output, that wrapper directive must switch to the resolved-skill wording — make the minimal edit and re-run. If empty, no change (documented no-op).

- [ ] **Step 2: Confirm no other test counts the emitted-var set (LEARNINGS #42 sweep)**

Run:

```bash
grep -rn -E "grep -c '=' | -eq (13|18)|BOARD_SURFACES|AUTO_GROOM" tests/ | grep -v test_docket_config.sh
```

Expected: no test outside `test_docket_config.sh` asserts the emitted-line count. (If one appears, update it to 18.)

- [ ] **Step 3: Run the full suite**

Run:

```bash
for t in tests/test_*.sh; do echo "== $t =="; bash "$t" >/tmp/o 2>&1 || { echo "FAILED: $t"; tail -5 /tmp/o; }; done; echo "done"
```

Expected: every test prints `PASS`; no `FAILED:` lines.

- [ ] **Step 4: Commit (only if Step 1 required a wrapper edit)**

```bash
git add sync-agents.sh agents/
git commit -m "feat(0049): resolved-skill wording in generated agent-wrapper directives"
```

---

## Self-Review

**Spec coverage:**
- Config shape / five `SKILL_*` vars / defaults → Task 1 ✓
- `auto` sentinel passthrough + unknown-key warn-and-ignore → Task 1 (parse/emit/warn) + Task 2 (documented) ✓
- Auto fallback = final artifact only → Task 2 table (column 4), referenced by Tasks 3–4 ✓
- Missing-skill → degrade to auto + warn (run output + PR body for build roles) → Task 2 rule, wired at each site in Tasks 3–4 ✓
- Touched surfaces: `docket-config.sh`/`.md` (T1), convention (T2), new-change/groom-next (T3), implement-next/finalize (T4), tests (T1/T2/T3/T4) ✓
- #0044 relationship guard → recorded in the convention Skill layer + spec; no code change here ✓
- Open questions: availability probe = attempt-and-degrade (baked into the missing-skill rule wording); wrapper-directive no-op verified in T5; ADR recorded at implement-next step 6 (post-build, not a plan task) ✓

**Placeholder scan:** none — every step shows the exact edit, code, command, and expected result.

**Type/name consistency:** `SKILL_BRAINSTORM`/`SKILL_PLAN`/`SKILL_BUILD`/`SKILL_REVIEW`/`SKILL_FINISH` used identically across Tasks 1–4 and the sentinels; defaults spelled identically to the superpowers skill names everywhere.
