<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0245 — Harden sync-agents wrapper generation and clear the 0192 findings](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-08-0245-harden-sync-agents-wrapper-generation-and-clear-the-0192-fin.md)**
<!-- docket:backlink:end -->

# Harden sync-agents wrapper generation and clear the 0192 findings — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Factor the three sync-agents wrapper emitters onto one shared parse helper, make the unmapped-harness-token path loud, clear change 0192's six unfixed review findings, and add the #0082 global-opt-in advisory.

**Architecture:** All production changes land in one file, `sync-agents.sh` (repo root, 1636 lines), plus three documentation files. The refactor (Task 1) is byte-identity-preserving: a new `parse_wrapper_source()` sets fixed `WSRC_*` globals that the three named emitters consume, while every emitter keeps its own serialization, its own skills-preamble sentence, and its own `inherit`/`auto` sentinel handling. Tasks 2 and 6 add behavior (a WARN, a `--check` advisory leg, an opt-in hint); Tasks 3–5 are finding-clearing edits with tests. Tests go into the existing `tests/test_sync_agents_opencode.sh` for the opencode/dispatch-block work, and into one new suite `tests/test_sync_agents_harness_gaps.sh` for the two new behaviors.

**Tech Stack:** POSIX-ish bash 3.2 (macOS system bash is the floor — no namerefs, no `declare -A` in `sync-agents.sh`), `sed`/`awk`/`grep`, the repo's own `tests/test_*.sh` assert-harness convention, `scripts/run-tests.sh` as the suite runner.

## Global Constraints

- **Bash floor is 3.2.** No namerefs (`local -n`), no associative arrays, no `${var^^}` in `sync-agents.sh`. Result passing is by fixed global, the repo's `RES_*` house pattern.
- **`log()` writes to stderr** (`sync-agents.sh:108`: `log(){ printf '%s\n' "sync-agents: $*" >&2; }`). Emitter stdout is redirected into the wrapper file, so **every** diagnostic added inside an emitter or its dispatch must go through `log`, never `printf` to stdout.
- **Byte-identity is Task 1's gate.** After Task 1, generated wrappers for all four harnesses must be byte-identical to before, and `tests/test_sync_agents.sh`, `_codex.sh`, `_cursor.sh`, `_opencode.sh` must pass **without being modified by Task 1**.
- **A new `tests/test_*.sh` file requires two more edits in the same commit** (`tests/test_runtime_budgets.sh` enforces both): a row in `tests/runtime-budgets.tsv` (`<path><TAB><integer seconds><TAB>parallel`, minimum 10), and a bump of `EXPECTED_TOTAL` in `tests/test_runtime_budgets.sh:28` by exactly that row's seconds, with a dated comment naming the new-test-file case.
- **Portability:** the PATH `grep` is ugrep and accepts constructs BSD `grep` rejects. Re-run any new grep-bearing assertion under `/usr/bin/grep` before committing.
- **`AGENTS_MD_DISPATCH_HARNESSES="codex opencode"`** (`sync-agents.sh:262`) is the single source for the dispatch-harness list. Task 4 makes the `--check` diagnostic read it; the *comment* at `:1225` that also hand-lists `(codex, opencode)` is **out of scope** (prose about the block, not an emitted string).
- **Verbatim spec wording for the #0082 hint** (Task 6) — the message text is fixed in that task, do not paraphrase.

---

## File Structure

| File | Responsibility | Tasks |
|---|---|---|
| `sync-agents.sh` (modify) | `parse_wrapper_source()`, `harness_has_named_emitter()`, `warn_unmapped_harness()`, the `--check` advisory leg, the head-prose reword, the derived diagnostic, the #0082 hint | 1, 2, 4, 6 |
| `tests/test_sync_agents_harness_gaps.sh` (create) | The two new behaviors: unmapped-token WARN + `--check` advisory (Task 2); the #0082 advisory's four-cell truth table (Task 6) | 2, 6 |
| `tests/runtime-budgets.tsv` (modify) | One row for the new suite | 2 |
| `tests/test_runtime_budgets.sh` (modify) | `EXPECTED_TOTAL` bump + dated rationale | 2 |
| `tests/test_sync_agents_opencode.sh` (modify) | Two-dispatch-harness fixture (Task 3); body/preamble asserts + effort-drop generation probe (Task 5) | 3, 5 |
| `docs/codex/setup.md` (modify) | Last-dispatch-harness caveat on the de-list note | 3 |
| `skills/docket-convention/references/agent-layer.md` (modify) | Opencode table cell → bare `.md` | 4 |

---

### Task 1: `parse_wrapper_source()` — one shared parse, byte-identical output

**Files:**
- Modify: `sync-agents.sh` — insert the helper just above the emitter registry comment at `:826`; rewrite the parse lines inside `emit_codex_toml` (`:858`), `emit_cursor_md` (`:917`), `emit_opencode_md` (`:973`)
- Test: no new test file. The gate is a build-time byte-identity snapshot plus the four **unmodified** existing suites.

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: `parse_wrapper_source <src-md>` → sets four globals, read by every later task that touches an emitter:
  - `WSRC_NAME` — the `name:` frontmatter value, falling back to `docket-$(short_name "$src")`
  - `WSRC_DESC` — `agent_description "$src"`
  - `WSRC_SKILLS_CSV` — the bracket-stripped `skills:` value
  - `WSRC_BODY` — post-frontmatter body, leading blank lines trimmed

- [ ] **Step 1: Snapshot the pre-refactor output of all four harnesses**

This is the gate, so it is captured *before* any edit. Run from the worktree root:

```bash
SNAP="$(mktemp -d "${TMPDIR:-/tmp}/wsrc-before-XXXXXX")"; echo "$SNAP" > /tmp/wsrc-snap-path
R="$SNAP/repo"; mkdir -p "$R"; git -C "$R" init -q
printf 'agent_harnesses: [claude, cursor, codex, opencode]\n' > "$R/.docket.yml"
( cd "$R" && DOCKET_HARNESS_ROOT="$SNAP/home" bash "$PWD/../../sync-agents.sh" >"$SNAP/gen.log" 2>&1 ) || true
( cd "$R" && find .claude .cursor .codex .opencode -type f | LC_ALL=C sort | tar -cf "$SNAP/before.tar" -T - )
tar -tf "$SNAP/before.tar" | wc -l
```

Expected: a non-zero file count (4 harnesses × 16 agents + dispatch rules). If it is zero, stop — the snapshot is vacuous and cannot gate anything.

- [ ] **Step 2: Add `parse_wrapper_source()` above the emitter registry**

Insert immediately **before** the `# --- per-harness emitter registry (change 0077) ---` comment at `sync-agents.sh:826`:

```bash
# --- shared wrapper-source parse (change 0245) -------------------------------
# The three named emitters (codex/cursor/opencode) each re-derived the same four values from the
# wrapper source with byte-identical sed/awk. A parse fix that reached one and missed its twins is
# exactly the defect class this removes (learnings: escape-ere-metacharacters-in-key).
#
# Scope is deliberately SOURCE-DERIVED FIELDS ONLY. Three things stay per-emitter and must not
# migrate here:
#   * serialization (TOML vs YAML frontmatter),
#   * the skills-preamble sentence, which differs by one phrase per harness
#     (learnings: consolidation-flattens-caller-variance — templating it flattens real variance),
#   * the `inherit`/`auto` sentinel handling, which is ASYMMETRIC BY DESIGN: codex tests
#     `!= "inherit"` at emit position, cursor/opencode normalize to empty up front, and claude's
#     emit() passes `inherit` through verbatim (0168 whole-branch review, IMPORTANT 2). Folding it
#     in here is the regression that review caught.
# emit() itself is untouched: it is a stream transform and parses no fields.
#
# Result convention is fixed globals (the RES_*/resolve_agent_layers house pattern), not stdout
# key=value (a subshell per call, and escaping a multi-line body is the fragility being removed)
# and not namerefs (bash 4.3+; docket's floor is 3.2).
parse_wrapper_source(){  # $1=src md -> sets WSRC_NAME WSRC_DESC WSRC_SKILLS_CSV WSRC_BODY
  local src="$1"
  WSRC_NAME="$(sed -n '/^name:/{s/^name:[[:space:]]*//;p;q;}' "$src")"
  [ -n "$WSRC_NAME" ] || WSRC_NAME="docket-$(short_name "$src")"
  WSRC_DESC="$(agent_description "$src")"
  WSRC_SKILLS_CSV="$(sed -n '/^skills:/{s/^skills:[[:space:]]*//;p;q;}' "$src" | sed -e 's/^\[//' -e 's/\][[:space:]]*$//' -e 's/[[:space:]]*$//')"
  # body = everything after the frontmatter closing --- , leading blank lines trimmed.
  WSRC_BODY="$(awk '/^---[[:space:]]*$/ && d<2 {d++; next} d>=2 {print}' "$src" | awk 'NF{p=1} p{print}')"
}
```

- [ ] **Step 3: Rewrite `emit_codex_toml`'s parse block**

In `emit_codex_toml` (now shifted down by the insert), replace these six lines —

```bash
  local name desc model effort skills_csv body dev esc
  name="$(sed -n '/^name:/{s/^name:[[:space:]]*//;p;q;}' "$src")"
  [ -n "$name" ] || name="docket-$(short_name "$src")"
  desc="$(agent_description "$src")"
```

...and the two further down...

```bash
  skills_csv="$(sed -n '/^skills:/{s/^skills:[[:space:]]*//;p;q;}' "$src" | sed -e 's/^\[//' -e 's/\][[:space:]]*$//' -e 's/[[:space:]]*$//')"
  # body = everything after the frontmatter closing --- , leading blank lines trimmed.
  body="$(awk '/^---[[:space:]]*$/ && d<2 {d++; next} d>=2 {print}' "$src" | awk 'NF{p=1} p{print}')"
```

— so that the function reads:

```bash
emit_codex_toml(){  # $1=src md  $2=model  $3=effort   (both FINAL resolved values)
  local src="$1" mo="$2" eo="$3"
  local name desc model effort skills_csv body dev esc
  parse_wrapper_source "$src"
  name="$WSRC_NAME"
  desc="$WSRC_DESC"
  skills_csv="$WSRC_SKILLS_CSV"
  body="$WSRC_BODY"
  # change 0168: FINAL resolved values (shipped sidecar ⊕ user layers). The source frontmatter is
  # no longer a default store, so there is nothing to fall back to: an unresolved field means the
  # wrapper is honestly UNPINNED and Codex applies its own default.
  model="$mo"
  effort="$eo"
```

Everything from `# developer_instructions text:` onward is untouched — including the `!= "inherit"` and `!= "auto"` tests at emit position, which stay exactly where they are.

- [ ] **Step 4: Rewrite `emit_cursor_md`'s parse block**

Same shape. The function becomes:

```bash
emit_cursor_md(){  # $1=src md  $2=model  $3=effort   (both FINAL resolved values)
  local src="$1" mo="$2" eo="$3"
  local name desc model effort skills_csv body
  parse_wrapper_source "$src"
  name="$WSRC_NAME"
  desc="$WSRC_DESC"
  skills_csv="$WSRC_SKILLS_CSV"
  body="$WSRC_BODY"
  # change 0168: FINAL resolved values (shipped sidecar ⊕ user layers). The source frontmatter is
  # no longer a default store, so there is nothing to fall back to. An agent with no cursor entry
  # in agents/harness-defaults.yml and no user override is emitted UNPINNED — which is the point:
  # falling back to the source frontmatter is exactly how a Claude model ID leaked into a Cursor
  # wrapper that could never honor it.
  model="$mo"
  effort="$eo"
  # Normalize the two "no pin" sentinels to empty, so the emit logic below has one shape to test.
  [ "$model" = "inherit" ] && model=""
  [ "$effort" = "auto" ] && effort=""
```

The two sentinel-normalization lines **stay in the emitter** — do not move them into the helper. Everything from `printf -- '---\n'` onward is untouched.

- [ ] **Step 5: Rewrite `emit_opencode_md`'s parse block**

`emit_opencode_md` declares no `name` (the filename is the identifier), so it consumes three of the four globals:

```bash
emit_opencode_md(){  # $1=src md  $2=model  $3=effort   (both FINAL resolved values)
  local src="$1" mo="$2" eo="$3"
  local desc model effort skills_csv body
  parse_wrapper_source "$src"
  desc="$WSRC_DESC"
  skills_csv="$WSRC_SKILLS_CSV"
  body="$WSRC_BODY"
  # change 0168: FINAL resolved values (shipped sidecar ⊕ user layers). The source frontmatter is
  # no longer a default store, so an unresolved field means the wrapper is honestly UNPINNED and
  # opencode applies its own default.
  model="$mo"
  effort="$eo"
  # Normalize the two "no pin" sentinels to empty, so the emit logic below has one shape to test.
  # `inherit` is a real Claude Code frontmatter value with no opencode equivalent, so it normalizes
  # here exactly as it does in emit_cursor_md/emit_codex_toml rather than passing through.
  [ "$model" = "inherit" ] && model=""
  [ "$effort" = "auto" ] && effort=""
```

`WSRC_NAME` is set and ignored here — harmless. Everything from `printf -- '---\n'` onward is untouched, including the `short_name "$src"` call inside the effort-drop WARN at what was `:997`.

- [ ] **Step 6: Verify byte-identity against the Step-1 snapshot**

```bash
SNAP="$(cat /tmp/wsrc-snap-path)"
R2="$SNAP/repo-after"; mkdir -p "$R2"; git -C "$R2" init -q
printf 'agent_harnesses: [claude, cursor, codex, opencode]\n' > "$R2/.docket.yml"
( cd "$R2" && DOCKET_HARNESS_ROOT="$SNAP/home-after" bash "$PWD/../../sync-agents.sh" >"$SNAP/gen2.log" 2>&1 ) || true
mkdir -p "$SNAP/unpacked"; tar -xf "$SNAP/before.tar" -C "$SNAP/unpacked"
diff -r "$SNAP/unpacked" <(cd "$R2" && tar -cf - .claude .cursor .codex .opencode | tar -xf - -C "$SNAP/after-x" 2>/dev/null; echo) 2>/dev/null || true
mkdir -p "$SNAP/after-x" && ( cd "$R2" && tar -cf - .claude .cursor .codex .opencode ) | tar -xf - -C "$SNAP/after-x"
diff -r "$SNAP/unpacked" "$SNAP/after-x" && echo "BYTE-IDENTICAL"
```

Expected: `BYTE-IDENTICAL`, zero diff lines. **A single diff line fails this task** — the refactor is defined as output-preserving; fix the emitter, do not adjust the snapshot.

- [ ] **Step 7: Run the four generation suites, unmodified**

```bash
bash tests/test_sync_agents.sh; bash tests/test_sync_agents_codex.sh
bash tests/test_sync_agents_cursor.sh; bash tests/test_sync_agents_opencode.sh
```

Expected: `ALL PASS` from each. `git status` must show **no** modification to any of those four files.

- [ ] **Step 8: Commit**

```bash
git add sync-agents.sh
git commit -m "refactor(0245): factor parse_wrapper_source out of the three emitters"
```

---

### Task 2: Loud unmapped harness tokens

**Files:**
- Modify: `sync-agents.sh` — add `harness_has_named_emitter()` and `warn_unmapped_harness()` next to the dispatch (just below `harness_ext`, above `emit_for_harness`); call the warner from the `*)` arm; add an advisory leg to `check_project_level`
- Create: `tests/test_sync_agents_harness_gaps.sh`
- Modify: `tests/runtime-budgets.tsv`, `tests/test_runtime_budgets.sh`

**Interfaces:**
- Consumes: nothing from Task 1 (independent region of the file).
- Produces: `harness_has_named_emitter <harness>` → rc 0 when the token has a named emitter (`claude|codex|cursor|opencode`), rc 1 otherwise. Task 6's test file reuses the same suite; nothing else calls it.
- Produces: global `WARNED_UNMAPPED` — a space-delimited, space-padded list of already-warned tokens.

- [ ] **Step 1: Write the failing test**

Create `tests/test_sync_agents_harness_gaps.sh`:

```bash
#!/usr/bin/env bash
# tests/test_sync_agents_harness_gaps.sh — the two gaps change 0245 closes in sync-agents.sh:
#   (1) an accepted-but-unmapped harness token silently got a Claude-shaped wrapper (#0142);
#   (2) a global agent_harnesses with no repo opt-in generated nothing and said nothing (#0082).
# Both are DIAGNOSTIC contracts, so every assertion here reads stderr, not a generated file.
# run: bash tests/test_sync_agents_harness_gaps.sh
set -u
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

REPO="$(cd "$(dirname "$0")/.." && pwd)"
WORK="$(mktemp -d "${TMPDIR:-/tmp}/docket-gaps-XXXXXX")"
trap 'rm -rf "$WORK"' EXIT

mk_repo(){  # $1=dest  $2=.docket.yml body
  mkdir -p "$1"; git -C "$1" init -q 2>/dev/null || true
  printf '%s\n' "$2" > "$1/.docket.yml"
}

# --- (1) unmapped token WARNs, once per harness per run ---------------------
RK="$WORK/kiro"; mk_repo "$RK" 'agent_harnesses: [claude, kiro]'
( cd "$RK" && DOCKET_HARNESS_ROOT="$WORK/home-kiro" bash "$REPO/sync-agents.sh" ) >"$WORK/kiro.out" 2>"$WORK/kiro.err" || true

assert "unmapped token 'kiro' produces a WARN" \
  'grep -q "WARN" "$WORK/kiro.err" && grep -q "kiro" "$WORK/kiro.err"'
assert "the WARN names the unverified Claude-shaped wrapper" \
  'grep -qi "claude-shaped" "$WORK/kiro.err"'
# 16 agents are generated per harness; a per-wrapper warn would print 16 lines.
assert "the WARN fires exactly once for the run, not once per wrapper" \
  '[ "$(grep -c "unverified for .kiro" "$WORK/kiro.err")" = "1" ]'
assert "the unmapped run still generates kiro wrappers (WARN, not refusal)" \
  '[ "$(ls "$RK"/.kiro/agents/docket-*.md 2>/dev/null | wc -l | tr -d " ")" = "16" ]'

# A named harness must stay silent — this is the discriminating half. A WARN that fired for every
# token would pass every assert above and still be wrong.
RC="$WORK/claudeonly"; mk_repo "$RC" 'agent_harnesses: [claude, codex, cursor, opencode]'
( cd "$RC" && DOCKET_HARNESS_ROOT="$WORK/home-claude" bash "$REPO/sync-agents.sh" ) >/dev/null 2>"$WORK/claude.err" || true
assert "no unmapped WARN for the four named harnesses" \
  '! grep -q "unverified for" "$WORK/claude.err"'

# --- (2) --check surfaces it as a NON-failing advisory ----------------------
( cd "$RK" && DOCKET_HARNESS_ROOT="$WORK/home-kiro" bash "$REPO/sync-agents.sh" --check ) >"$WORK/chk.out" 2>"$WORK/chk.err"; chk_rc=$?
assert "--check prints an advisory for the unmapped token" \
  'grep -q "advisory:" "$WORK/chk.err" && grep -q "kiro" "$WORK/chk.err"'
assert "--check advisory does NOT fail the run (rc unchanged)" '[ "'"$chk_rc"'" = "0" ]'

( cd "$RC" && DOCKET_HARNESS_ROOT="$WORK/home-claude" bash "$REPO/sync-agents.sh" --check ) >/dev/null 2>"$WORK/chk2.err" || true
assert "--check prints no unmapped advisory for named harnesses" \
  '! grep -q "advisory: harness" "$WORK/chk2.err"'

echo "---"; [ "$fail" = "0" ] && echo "ALL PASS" || echo "FAILURES"; exit $fail
```

- [ ] **Step 2: Register the new suite in the budget table**

Append to `tests/runtime-budgets.tsv`, keeping the file's existing ordering convention (rows are path-sorted — insert it between `tests/test_sync_agents_drift_docs.sh` and `tests/test_sync_agents_opencode.sh`):

```
tests/test_sync_agents_harness_gaps.sh	20	parallel
```

Then in `tests/test_runtime_budgets.sh:28`, change `EXPECTED_TOTAL=1415` to `EXPECTED_TOTAL=1435` and add this as the **first** comment line under it (above the existing `# 1405 -> 1415` line):

```
                    # 1415 -> 1435 (change 0245): the new-test-file case named below —
                    # tests/test_sync_agents_harness_gaps.sh brings its own row; it runs four full
                    # sync-agents generations (two real, two --check), sized to 20s against the
                    # 10s-15s the sibling sync-agents suites measure for one or two.
```

- [ ] **Step 3: Run the test to verify it fails**

```bash
bash tests/test_sync_agents_harness_gaps.sh
```

Expected: FAIL — `NOT OK - unmapped token 'kiro' produces a WARN`, `NOT OK - the WARN names the unverified Claude-shaped wrapper`, `NOT OK - the WARN fires exactly once for the run, not once per wrapper`, `NOT OK - --check prints an advisory for the unmapped token`. The "no WARN for named harnesses" asserts pass vacuously (nothing warns yet) — that is expected and is why the failing asserts above are the discriminating ones.

- [ ] **Step 4: Add the predicate and the warner**

In `sync-agents.sh`, insert directly **below** the `harness_ext(){ ... }` line (`:828`) and **above** the `# Dispatch to the harness-appropriate emitter.` comment:

```bash
# Does this harness token have a NAMED emitter, or does it fall through to the generic
# Claude-shaped one? Named once, used by both consumers — emit_for_harness's `*)` arm and
# check_project_level's advisory leg — so the two cannot drift into disagreeing about which
# tokens are supported (learnings: duplicated-gate-copies-the-whole-predicate).
harness_has_named_emitter(){  # $1=harness
  case "$1" in claude|codex|cursor|opencode) return 0;; *) return 1;; esac
}

# Space-padded list of harness tokens already warned about in THIS run. A run generates one wrapper
# per agent (16+), so a per-wrapper warn would bury the message under its own repetition; the
# emitters run in the main shell, not subshells, so a plain global is sufficient state.
WARNED_UNMAPPED=" "
warn_unmapped_harness(){  # $1=harness
  case "$WARNED_UNMAPPED" in *" $1 "*) return 0;; esac
  WARNED_UNMAPPED="$WARNED_UNMAPPED$1 "
  log "WARN harness '$1' has no named emitter — its wrappers are Claude-shaped and unverified for '$1', so the model and effort docket reports may never be honored (ADR-0060). Give '$1' its own emitter, or accept the unverified shape."
}
```

- [ ] **Step 5: Call the warner from the `*)` arm**

In `emit_for_harness`, change the catch-all arm from:

```bash
    *)        emit            "$1" "$3" "$4";;
```

to:

```bash
    *)        warn_unmapped_harness "$2"; emit "$1" "$3" "$4";;
```

Leave the existing five-line comment above it exactly as it is — it explains *why* the arm is dangerous, and the WARN is now its runtime half. Note the WARN also fires on the user-level pass (a presence-detected `~/.kiro/agents`); that is intended — the unverified shape must never generate silently, whichever pass emits it.

- [ ] **Step 6: Add the `--check` advisory leg**

In `check_project_level`, insert this block immediately **after** the `legacy=` committed-config-shape block and **before** the `# leg (c) — local staleness` comment:

```bash
  # Unmapped-harness advisory (change 0245). ADVISORY: reported, never fails CI — existing repos
  # that list such a token today keep working, and hard refusal is deliberately out of scope. Same
  # substance as the generation-time WARN, same predicate; a token may surface twice on this path
  # (leg (c)'s emit_wrapper reaches the `*)` arm's own once-per-harness WARN), which is accepted —
  # each is deduped, and suppressing one would couple the legs.
  local uh
  for uh in $HARNESSES; do
    harness_has_named_emitter "$uh" && continue
    log "advisory: harness '$uh' has no named emitter — its wrappers are Claude-shaped and unverified for '$uh' (ADR-0060). Not a check failure."
  done
```

- [ ] **Step 7: Run the test to verify it passes**

```bash
bash tests/test_sync_agents_harness_gaps.sh
bash /usr/bin/grep --version >/dev/null 2>&1 || true
PATH=/usr/bin:/bin bash tests/test_sync_agents_harness_gaps.sh
```

Expected: `ALL PASS` from both invocations. The second runs the asserts' greps under BSD `/usr/bin/grep`, per the global portability constraint.

- [ ] **Step 8: Run the budget guard and the four generation suites**

```bash
bash tests/test_runtime_budgets.sh
bash tests/test_sync_agents.sh; bash tests/test_sync_agents_codex.sh
bash tests/test_sync_agents_cursor.sh; bash tests/test_sync_agents_opencode.sh
```

Expected: `ALL PASS` from each. If `test_runtime_budgets.sh` reports `budgeted total: 1435s ... pinned at 1415s`, the `EXPECTED_TOTAL` edit in Step 2 was missed.

- [ ] **Step 9: Commit**

```bash
git add sync-agents.sh tests/test_sync_agents_harness_gaps.sh tests/runtime-budgets.tsv tests/test_runtime_budgets.sh
git commit -m "feat(0245): WARN and --check advisory for unmapped harness tokens"
```

---

### Task 3: The two-dispatch-harness fixture and the codex de-list caveat

**Files:**
- Modify: `tests/test_sync_agents_opencode.sh` — append the fixture after the existing de-list block (the current last assert before the `echo "---"` footer)
- Modify: `docs/codex/setup.md:52-54`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: nothing later tasks consume.

- [ ] **Step 1: Write the failing test**

In `tests/test_sync_agents_opencode.sh`, insert this block immediately **before** the final `echo "---"; [ "$fail" = "0" ] ...` line:

```bash
# --- two AGENTS.md-dispatch harnesses share ONE block (change 0245) ----------
# The discriminating fixture: every existing fixture above configures exactly ONE dispatch harness,
# and a single-owner fixture produces identical output under "is opencode listed" and "is ANY
# dispatch harness listed" — so a suite of them is green against code that never learned to share
# (learnings: shared-resource-keeps-first-owner-assumptions).
R3="$WORK/repo3"; mkdir -p "$R3"; git -C "$R3" init -q 2>/dev/null || true
printf 'agent_harnesses: [codex, opencode]\n' > "$R3/.docket.yml"
( cd "$R3" && DOCKET_HARNESS_ROOT="$WORK/home3" bash "$REPO/sync-agents.sh" >/dev/null 2>&1 ) || true
A3="$R3/AGENTS.md"
assert "two dispatch harnesses get the block EXACTLY once" \
  '[ "$(grep -c "docket:dispatch:start" "$A3")" = "1" ]'
assert "two dispatch harnesses: exactly one closing marker" \
  '[ "$(grep -c "docket:dispatch:end" "$A3")" = "1" ]'
assert "two dispatch harnesses: the block still lists every wrapper source once" \
  '[ "$(grep -c "^- \*\*docket-" "$A3")" = "16" ]'

# De-list ONE of the two: the block must SURVIVE. This is the assert no single-owner fixture reaches.
printf 'agent_harnesses: [opencode]\n' > "$R3/.docket.yml"
( cd "$R3" && DOCKET_HARNESS_ROOT="$WORK/home3" bash "$REPO/sync-agents.sh" >/dev/null 2>&1 ) || true
assert "de-listing codex leaves the block in place (opencode still targets it)" \
  'grep -q "docket:dispatch:start" "$A3"'

# De-list the LAST one: now it goes.
printf 'agent_harnesses: [claude]\n' > "$R3/.docket.yml"
( cd "$R3" && DOCKET_HARNESS_ROOT="$WORK/home3" bash "$REPO/sync-agents.sh" >/dev/null 2>&1 ) || true
assert "de-listing the LAST dispatch harness strips the block" \
  '! grep -q "docket:dispatch:start" "$A3" 2>/dev/null'
```

- [ ] **Step 2: Run the test**

```bash
bash tests/test_sync_agents_opencode.sh
```

Expected: `ALL PASS`. **This test is expected to pass on first run** — `repo_wants_agents_md_dispatch` already reads `AGENTS_MD_DISPATCH_HARNESSES`, so the behavior is correct and merely unguarded. If any of the six asserts fails, that is a real production defect in `sync_agents_md_dispatch`: stop and fix it in this task before continuing. A fixture that pins already-correct behavior is the deliverable here — the finding was "no two-dispatch-harness fixture exists anywhere in `tests/`", not "the behavior is wrong".

- [ ] **Step 3: Fix the codex setup.md de-list note**

In `docs/codex/setup.md`, replace lines 52-54:

```markdown
> Note: when Codex is de-listed from an opted-in repo, `sync-agents.sh` **removes** the
> `AGENTS.md` dispatch block (and prints a one-time commit notice). Your own `AGENTS.md`
> content outside the docket markers is preserved untouched.
```

with:

```markdown
> Note: because the block is shared with opencode, it is removed only when the **last**
> `AGENTS.md`-dispatch harness is de-listed. De-listing Codex from a repo that still targets
> opencode (or the reverse) leaves the block in place, correctly; de-listing the last one removes
> it and prints a one-time commit notice. Your own `AGENTS.md` content outside the docket markers
> is preserved untouched.
```

This mirrors the wording `docs/opencode/setup.md:59-62` already carries — the opencode doc was written after the block became shared and got the caveat; the incumbent codex doc kept the single-owner sentence.

- [ ] **Step 4: Verify the doc claim now matches the fixture**

```bash
grep -n "last" docs/codex/setup.md | head -3
bash tests/test_codex_runbook.sh
```

Expected: the note names "last"; `test_codex_runbook.sh` still passes (it asserts setup.md links the runbook — an unrelated region, but confirm no collateral damage).

- [ ] **Step 5: Commit**

```bash
git add tests/test_sync_agents_opencode.sh docs/codex/setup.md
git commit -m "test(0245): pin the shared dispatch block across two harnesses; fix the codex de-list caveat"
```

---

### Task 4: Derive the `--check` diagnostic; scope the head claim; fix the table cell

**Files:**
- Modify: `sync-agents.sh:1431` (the `--check` diagnostic) and `sync-agents.sh:1206` (the head prose inside `assemble_agents_md_dispatch`)
- Modify: `skills/docket-convention/references/agent-layer.md:131`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: nothing later tasks consume.

- [ ] **Step 1: Derive the diagnostic's harness list from the variable**

In `check_project_level`, the `else` arm currently hand-lists the harnesses:

```bash
      log "check: AGENTS.md carries a docket dispatch block but no AGENTS.md-dispatch harness (codex, opencode) is in agent_harnesses — run: bash sync-agents.sh and commit AGENTS.md"
```

Replace with:

```bash
      log "check: AGENTS.md carries a docket dispatch block but no AGENTS.md-dispatch harness ($(printf '%s' "$AGENTS_MD_DISPATCH_HARNESSES" | sed 's/ /, /g')) is in agent_harnesses — run: bash sync-agents.sh and commit AGENTS.md"
```

The variable sits at `:262`; the hand-written copy was the drift risk — a fifth dispatch harness would have left this message naming two.

- [ ] **Step 2: Scope the AGENTS.md head claim**

In `assemble_agents_md_dispatch`'s heredoc, replace this sentence:

```
validated model and reasoning effort for every one of these agents on every harness it supports, so
they are pinned out of the box; your config layers override either field per agent.
```

with:

```
validated model and reasoning effort for every one of these agents on the harnesses it ships
defaults for — Claude, Cursor, Codex and opencode — so they are pinned out of the box there; your
config layers override either field per agent, and set them for any other harness.
```

The old sentence claimed shipped defaults "on every harness it supports", which is false for an accepted-but-unmapped token: those wrappers are unpinned. **This changes committed bytes in consumer repos** — accepted: `--check` flags the staleness, the next run rewrites the block, which is the designed refresh path.

- [ ] **Step 3: Fix the agent-layer table cell**

In `skills/docket-convention/references/agent-layer.md:131`, the opencode row holds a full path where its three siblings hold a bare extension. Change:

```markdown
| opencode | `.opencode/agents/docket-<name>.md` | `model:` (`openrouter/<vendor>/<id>`) | `reasoningEffort:` (a provider model option, not a first-class field) | body preamble |
```

to:

```markdown
| opencode | `.md` | `model:` (`openrouter/<vendor>/<id>`) | `reasoningEffort:` (a provider model option, not a first-class field) | body preamble |
```

- [ ] **Step 4: Verify the block regenerates and `--check` flags the staleness**

```bash
W="$(mktemp -d)"; R="$W/r"; mkdir -p "$R"; git -C "$R" init -q
printf 'agent_harnesses: [codex]\n' > "$R/.docket.yml"
( cd "$R" && DOCKET_HARNESS_ROOT="$W/h" bash "$PWD/../../sync-agents.sh" >/dev/null 2>&1 )
grep -c "on every harness it supports" "$R/AGENTS.md" || true
grep -q "ships" "$R/AGENTS.md" && echo "REWORDED OK"
```

Expected: the old phrase count is `0`, and `REWORDED OK` prints.

Then verify the derived diagnostic fires with the interpolated list:

```bash
printf 'agent_harnesses: [claude]\n' > "$R/.docket.yml"
( cd "$R" && DOCKET_HARNESS_ROOT="$W/h" bash "$PWD/../../sync-agents.sh" --check ) 2>&1 | grep "dispatch block but no" || echo "(block already stripped — re-add it by hand to exercise the arm)"
```

Expected: when the arm fires, its text reads `(codex, opencode)` — same rendering as before, now derived.

- [ ] **Step 5: Run the affected suites**

```bash
bash tests/test_sync_agents_codex.sh; bash tests/test_sync_agents_opencode.sh
bash tests/test_sync_agents_drift_docs.sh
```

Expected: `ALL PASS` from each. `test_sync_agents_drift_docs.sh` guards several `agent-layer.md` claims; none of them pin the opencode cell's path, but it is the suite that would catch collateral damage from Step 3.

- [ ] **Step 6: Commit**

```bash
git add sync-agents.sh skills/docket-convention/references/agent-layer.md
git commit -m "fix(0245): derive the dispatch-harness diagnostic; scope the head claim; fix the agent-layer cell"
```

---

### Task 5: opencode body/preamble asserts and the effort-drop probe

**Files:**
- Modify: `tests/test_sync_agents_opencode.sh` — append after Task 3's block, before the footer
- Modify: `sync-agents.sh` — comment only, and only if the live probe contradicts it (see Step 5)

**Interfaces:**
- Consumes: Task 3's `$R3`/`$A3` fixtures are already scoped; this task creates its own `$R4`.
- Produces: nothing.

- [ ] **Step 1: Write the failing body/preamble asserts**

Append to `tests/test_sync_agents_opencode.sh`, before the footer:

```bash
# --- the opencode emitter's BODY and skills preamble (change 0245) -----------
# codex and cursor both assert their emitted body survives and carries the preamble; opencode had
# neither, so a regression to an empty prompt would have shipped green.
OC="$D/docket-status.md"
assert "opencode: emitted body is non-empty" \
  '[ -n "$(awk "/^---[[:space:]]*$/ && d<2 {d++; next} d>=2 {print}" "$OC" | tr -d "[:space:]")" ]'
assert "opencode: body preamble tells the child to LOAD its skills" \
  'grep -qiF "load these docket skills" "$OC"'
assert "opencode: preamble names the opencode skills directory" \
  'grep -qiF "opencode skills directory" "$OC"'
assert "opencode: preamble names the agent's own skill" 'grep -qF "docket-status" "$OC"'
assert "opencode: preamble names docket-convention"     'grep -qF "docket-convention" "$OC"'
assert "opencode: wrapper body survives verbatim"       'grep -qi "refresh docket state" "$OC"'
```

- [ ] **Step 2: Write the failing effort-drop generation assert**

Append immediately after:

```bash
# --- effort is DROPPED when no model resolves (change 0245) ------------------
# Docket's own half of the "a provider option with no provider selected has nothing to reach"
# rationale: pinned by test regardless of whether the opencode CLI is present to probe the other
# half. The fixture pins model to the `inherit` sentinel, which normalizes to "no model".
R4="$WORK/repo4"; mkdir -p "$R4"; git -C "$R4" init -q 2>/dev/null || true
cat > "$R4/.docket.yml" <<'YML'
agent_harnesses: [opencode]
agents:
  opencode:
    status: {model: inherit, effort: high}
YML
( cd "$R4" && DOCKET_HARNESS_ROOT="$WORK/home4" bash "$REPO/sync-agents.sh" ) >/dev/null 2>"$WORK/oc4.err" || true
F4="$R4/.opencode/agents/docket-status.md"
assert "opencode effort-drop: no model line is emitted"  '! grep -qE "^model:" "$F4"'
assert "opencode effort-drop: NO reasoningEffort key is emitted" '! grep -qE "^reasoningEffort:" "$F4"'
assert "opencode effort-drop: the drop is WARNed, not silent" \
  'grep -q "effort .high. dropped" "$WORK/oc4.err"'
```

- [ ] **Step 3: Run the tests to verify which fail**

```bash
bash tests/test_sync_agents_opencode.sh
```

Expected: the six body/preamble asserts **pass** (the emitter already does this — the finding was missing coverage) and the three effort-drop asserts **pass** (the branch at `sync-agents.sh:993-998` already implements it). If any fails, that is a real defect — fix the emitter in this task.

If the `effort .high. dropped` assert fails because the config shape is not consumed, check the resolved value by running `( cd "$R4" && DOCKET_HARNESS_ROOT=... bash sync-agents.sh )` by hand and reading stderr; adjust the fixture's flow-map spelling (no spaces inside `{…}`, no `#`) rather than weakening the assert.

- [ ] **Step 4: Probe the live opencode rationale**

The comment at `sync-agents.sh:964-968` asserts a claim about opencode's own behavior. The CLI is present on this machine (`/opt/homebrew/bin/opencode`), so probe it rather than reasoning about it:

```bash
opencode --version
mkdir -p "$R4/.opencode/agents" && opencode debug agent docket-status 2>&1 | head -30
```

Read the output for `options.reasoningEffort`. Two outcomes:

- **The probe confirms** `reasoningEffort` arrives under `options` and that an agent with no model has no provider to reach → leave the comment as written, and record the probe's version in the commit message.
- **The probe contradicts it, or `opencode debug agent` is unavailable / errors** → proceed to Step 5.

- [ ] **Step 5: Reword the comment if the probe did not confirm it**

Only if Step 4 did not confirm. In `sync-agents.sh`, change:

```
# reports it as `options.reasoningEffort`. That is also why effort is dropped when no model
# resolves — a provider option with no provider selected has nothing to reach.
```

to:

```
# reports it as `options.reasoningEffort`. Effort is dropped when no model resolves — that is
# DOCKET's design choice (a provider option with no provider selected has nothing to name), pinned
# by tests/test_sync_agents_opencode.sh's effort-drop asserts, not a verified opencode behavior.
```

The `options.reasoningEffort` forwarding claim itself stays either way — it was verified against opencode 1.18.11.

- [ ] **Step 6: Re-run and commit**

```bash
bash tests/test_sync_agents_opencode.sh
```

Expected: `ALL PASS`.

```bash
git add tests/test_sync_agents_opencode.sh sync-agents.sh
git commit -m "test(0245): pin the opencode body, preamble, and effort-drop contract"
```

---

### Task 6: The #0082 global-opt-in advisory

**Files:**
- Modify: `sync-agents.sh` — `project_level_pass` (`:1355`), replacing its bare early return
- Modify: `tests/test_sync_agents_harness_gaps.sh` — append the four-cell truth table

**Interfaces:**
- Consumes: `USER_HARNESSES_SET` (set by `resolve_global_agent_harnesses`, which `main` calls at `:1622`, before `project_level_pass` at `:1633` — so it is always resolved on the generation path) and `per_repo_opted_in` (`:222`).
- Produces: nothing.

- [ ] **Step 1: Write the failing test**

Append to `tests/test_sync_agents_harness_gaps.sh`, before the footer:

```bash
# --- (3) #0082: global agent_harnesses + no repo opt-in is no longer silent ---
# Four cells, because the hint must fire in exactly ONE of them. A test of only the firing cell
# cannot separate "fires correctly" from "fires always".
GH="$WORK/gcfg"; mkdir -p "$GH/docket"
printf 'agent_harnesses: [claude, cursor]\n' > "$GH/docket/config.yml"

run_cell(){  # $1=label  $2=global-set(1|0)  $3=repo .docket.yml body or empty
  local d="$WORK/cell-$1"; mkdir -p "$d"; git -C "$d" init -q 2>/dev/null || true
  [ -n "$3" ] && printf '%s\n' "$3" > "$d/.docket.yml"
  if [ "$2" = 1 ]; then
    ( cd "$d" && XDG_CONFIG_HOME="$GH" DOCKET_HARNESS_ROOT="$WORK/h-$1" bash "$REPO/sync-agents.sh" ) >/dev/null 2>"$WORK/cell-$1.err" || true
  else
    ( cd "$d" && XDG_CONFIG_HOME="$WORK/empty-xdg" DOCKET_HARNESS_ROOT="$WORK/h-$1" bash "$REPO/sync-agents.sh" ) >/dev/null 2>"$WORK/cell-$1.err" || true
  fi
}
mkdir -p "$WORK/empty-xdg"
HINT="has not opted in"

run_cell global-noopt 1 ''
assert "#0082: global set + no repo opt-in PRINTS the hint" \
  'grep -qF "'"$HINT"'" "$WORK/cell-global-noopt.err"'
assert "#0082: the hint names .docket.local.yml" \
  'grep -qF ".docket.local.yml" "$WORK/cell-global-noopt.err"'
assert "#0082: the hint names the global config path it read" \
  'grep -qF "docket/config.yml" "$WORK/cell-global-noopt.err"'

run_cell global-opted 1 'agent_harnesses: [claude]'
assert "#0082: global set + repo OPTED IN stays silent" \
  '! grep -qF "'"$HINT"'" "$WORK/cell-global-opted.err"'

run_cell noglobal-noopt 0 ''
assert "#0082: no global + no opt-in stays silent" \
  '! grep -qF "'"$HINT"'" "$WORK/cell-noglobal-noopt.err"'

run_cell noglobal-opted 0 'agent_harnesses: [claude]'
assert "#0082: no global + repo opted in stays silent" \
  '! grep -qF "'"$HINT"'" "$WORK/cell-noglobal-opted.err"'
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
bash tests/test_sync_agents_harness_gaps.sh
```

Expected: FAIL on the three `global-noopt` asserts (`#0082: global set + no repo opt-in PRINTS the hint`, `... names .docket.local.yml`, `... names the global config path it read`). The three silence asserts pass vacuously — expected, and exactly why the truth table has four cells.

- [ ] **Step 3: Add the hint**

In `sync-agents.sh`, replace `project_level_pass`'s first line:

```bash
project_level_pass() {  # built-in ⊕ local ⊕ committed ⊕ global -> <repo>/.<H>/agents for each H in HARNESSES
  project_wrappers_generated || return 0
```

with:

```bash
project_level_pass() {  # built-in ⊕ local ⊕ committed ⊕ global -> <repo>/.<H>/agents for each H in HARNESSES
  if ! project_wrappers_generated; then
    # #0082: a user-level agent_harnesses cannot drive per-repo generation — per-repo targeting is
    # deliberately repo-owned so the committed artifacts stay deterministic across every clone
    # (ADR-0019's coordination-key fence; change 0050). What was wrong was the SILENCE: the user
    # set a knob, ran the tool, and got neither wrappers nor a word. Generation path only — one
    # authoritative copy of the hint, at the moment the user acted and the no-op bit.
    if [ "${USER_HARNESSES_SET:-0}" = "1" ]; then
      log "global agent_harnesses is set (${XDG_CONFIG_HOME:-$HOME/.config}/docket/config.yml) but this repo has not opted in, so no per-repo wrappers were generated. To opt in, add 'agent_harnesses:' to .docket.local.yml (machine-local) or .docket.yml (committed)."
    fi
    return 0
  fi
```

Options 2 and 3 of #0082 — letting global config drive per-repo artifacts — stay rejected; this is advisory only and changes nothing about what gets generated.

- [ ] **Step 4: Run the test to verify it passes**

```bash
bash tests/test_sync_agents_harness_gaps.sh
PATH=/usr/bin:/bin bash tests/test_sync_agents_harness_gaps.sh
```

Expected: `ALL PASS` from both.

- [ ] **Step 5: Confirm `--check` was deliberately left alone**

```bash
grep -c "has not opted in" sync-agents.sh
```

Expected: `1`. A second copy in `check_project_level` is deliberately **not** added — the stub pins option 1 to the generation-time no-op, and one authoritative copy beats two drifting ones. If a future run wants it, `USER_HARNESSES_SET` is already resolved on that path via `validate_runner_config` (`:1561` → `:762`).

- [ ] **Step 6: Commit**

```bash
git add sync-agents.sh tests/test_sync_agents_harness_gaps.sh
git commit -m "feat(0245): hint when global agent_harnesses meets a non-opted-in repo"
```

---

### Task 7: Full-suite gate

**Files:** none modified — this task only runs and reports.

**Interfaces:**
- Consumes: every earlier task's commits.
- Produces: the green full-suite record.

- [ ] **Step 1: Run the whole suite**

```bash
scripts/run-tests.sh
```

Expected: every suite `PASS`, exit 0, and no `OVER BUDGET:` row for `tests/test_sync_agents_harness_gaps.sh`.

- [ ] **Step 2: If the new suite breaches its 20s budget**

Do **not** raise the ceiling — that is what the budget guard's counter C exists to catch. Instead reduce the suite's cost: the four `run_cell` invocations and the two `--check` runs each do a full 16-agent generation. Drop the `noglobal-opted` cell's *generation* to a `--check` run if needed, or narrow the fixtures' `agent_harnesses` to `[claude]` so fewer harnesses are emitted. Re-measure with `scripts/run-tests.sh -j 1 --timings /tmp/t.tsv` and read the row.

- [ ] **Step 3: Confirm the four generation suites were never modified**

```bash
git diff origin/main --stat -- tests/test_sync_agents.sh tests/test_sync_agents_codex.sh tests/test_sync_agents_cursor.sh
```

Expected: empty. `tests/test_sync_agents_opencode.sh` **is** expected to appear in a full `git diff --stat` — Tasks 3 and 5 extend it deliberately; the byte-identity gate's "unmodified" applies to Task 1's verification moment, and Task 1 touched none of them.

---

## Self-Review

**1. Spec coverage.**

| Spec section | Task |
|---|---|
| §1 `parse_wrapper_source()`, `WSRC_*` globals, per-emitter serialization/preamble/sentinels stay | Task 1 |
| §1 byte-identity gate, four suites unmodified | Task 1 Steps 1/6/7, Task 7 Step 3 |
| §2 `harness_has_named_emitter()` shared by both sites | Task 2 Step 4 |
| §2 `warn_unmapped_harness`, once per harness per run, via `log` | Task 2 Steps 4–5 |
| §2 non-failing `--check` advisory leg | Task 2 Step 6 |
| §2 behavioral predicate/dispatch agreement test | Task 2 Step 1 (named-harness silence asserts) |
| §3 codex setup.md last-harness caveat | Task 3 Step 3 |
| §3 two-dispatch-harness fixture in `test_sync_agents_opencode.sh` | Task 3 Step 1 |
| §3 head-claim reword (committed bytes accepted) | Task 4 Step 2 |
| §3 diagnostic derived from `AGENTS_MD_DISPATCH_HARNESSES` | Task 4 Step 1 |
| §3 opencode body/preamble asserts | Task 5 Step 1 |
| §3 effort-drop generation test + live probe with comment-reword fallback | Task 5 Steps 2, 4, 5 |
| §3 agent-layer.md opencode cell → bare `.md` | Task 4 Step 3 |
| §4 `project_level_pass` advisory, generation path only, `--check` copy deliberately absent | Task 6 Steps 3, 5 |
| Test plan: portability under `/usr/bin/grep` | Task 2 Step 7, Task 6 Step 4 |
| Assumptions 1, 2, 4, 5, 8, 9, 12, 13 | Encoded as comments in Tasks 1, 2, 5, 6 |

Assumptions 3 (no committed golden files), 6 (committed-bytes change accepted), 7 (fixture home), 10 (table cell in scope), and 11 (no new couplings) are satisfied by construction — no task adds a snapshot file, Task 4 Step 2 states the accepted cost, Task 3 uses the opencode suite, Task 4 Step 3 does the cell, and no task touches frontmatter.

**2. Placeholder scan.** Every code step carries the literal text to write. The two conditional branches (Task 5 Step 5's reword, Task 7 Step 2's budget relief) state their trigger condition and the exact remedy rather than deferring a decision.

**3. Type consistency.** `parse_wrapper_source` sets exactly `WSRC_NAME`, `WSRC_DESC`, `WSRC_SKILLS_CSV`, `WSRC_BODY`; Task 1 Steps 3–5 read those four names and no others (opencode reads three). `harness_has_named_emitter` is defined once in Task 2 Step 4 and called in Steps 5 and 6 under that spelling. `WARNED_UNMAPPED` is initialized and read in the same step. `EXPECTED_TOTAL` moves 1415 → 1435 in Task 2 Step 2 and is asserted nowhere else.
