# Encode the disabled board positively — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make an unresolved `--surfaces` value a loud exit-2 wiring error instead of a silent board-disabling no-op, by encoding the deliberate off-state positively as the reserved token `none`.

**Architecture:** Three parts. (1) A **positive sentinel**: `docket-config.sh` never emits an empty `BOARD_SURFACES` — `board_surfaces: []` resolves to `none`; `board-refresh.sh` exits 2 on an empty value and no-ops on `none`; `docket-status.sh`'s `board_pass` treats an empty value as a fatal resolver failure and maps `none` to its existing `board off` line. (2) **One Board pass**: the 8 duplicated Board-pass call sites across 6 skill/reference files collapse into `docket.sh docket-status --board-only`, so no surfaces value crosses the skill/script boundary at all. (3) **Guards**: a structural sentinel forbidding the retired spellings in skill prose, and a narrowing (never a deletion) of the existing transition guard onto the new canonical spelling.

**Tech Stack:** Bash 3.2-compatible shell scripts (`scripts/*.sh`), markdown skill prose (`skills/*/SKILL.md`), and a self-contained bash test harness (`tests/test_*.sh` — no framework, no `tests/lib/`; each test is a standalone script with an inline `assert(){ if eval "$2"; ...}` and `exit $fail`).

## Global Constraints

- **Repo:** all paths below are relative to the feature worktree `/Users/homer/dev/docket/.worktrees/board-surfaces-unset-vs-empty`. Branch: `feat/board-surfaces-unset-vs-empty`, cut from `origin/main` @ `edb37f9`.
- **Never touch docket metadata on this branch** — no edits to `docs/changes/**`, `docs/adrs/**`, or `BOARD.md`. Only `scripts/`, `skills/`, `tests/`, `docs/superpowers/plans/`, and (if written) `docs/results/`.
- **Bash portability:** target BSD (macOS) *and* GNU tools. `sed -E`, `awk` `match()/RSTART/RLENGTH`, `grep -oE/-qxF/-hoF/-cF` are proven safe in this repo; `grep -P` is not. Scripts use `set -uo pipefail` (never `-e`).
- **`none` is reserved and exclusive.** `--surfaces "none inline"` is a contradiction → exit 2, never a silent winner-pick.
- **`board_surfaces: []` semantics are UNCHANGED** — a disabled repo still renders nothing, commits nothing, and its pre-existing `BOARD.md` (if any) is left **byte-identical**. Only the internal *encoding* changes (`""` → `none`). The user-facing YAML spelling `[]` stays exactly as documented in `README.md`.
- **Guards are code (LEARNINGS, ADR-0031).** Every new assert must be **mutation-tested**: re-introduce the retired shape into the REAL tree (not only a fixture), watch the assert go RED, revert. An assert that stays green under its own mutation is a defect, not a curiosity. Never RETIRE an existing guard claiming subsumption — **narrow** it to the property that is still load-bearing.
- **Guard scope discrimination (ADR-0030):** a prose guard forbids **invocations**, never **nouns**. Descriptive mentions of `board-refresh.sh` (e.g. `skills/docket-convention/SKILL.md`'s "Derived-view script family", `skills/docket-status/SKILL.md`'s ownership prose) are PERMITTED and must stay green.
- **Assert on emitted lines, not eval'd variables**, wherever a config export is under test — `eval "$out"` leaves a stale value behind when a run emits nothing, making "emitted nothing" and "emitted the right thing" indistinguishable (LEARNINGS, stale-state).
- **Run one test:** `bash tests/test_<name>.sh` (prints `ok - …` / `NOT OK - …`; exits non-zero on any failure). **Run the suite:** `for f in tests/test_*.sh; do echo "== $f"; bash "$f" || echo "FAILED: $f"; done` — ~10 minutes; run it in the FOREGROUND.
- **Commit after every task.** Never `git add -A` (the worktree shares a repo with other trees) — stage the exact paths.

---

## File Structure

| File | Responsibility | Change |
|---|---|---|
| `scripts/docket-config.sh` | config resolver; sole emitter of `BOARD_SURFACES` | Normalize an empty resolved value to `none` (single choke point after the machine-fence filter) |
| `scripts/docket-config.md` | resolver contract | Document `[]` → `none`, and the never-empty invariant |
| `scripts/board-refresh.sh` | gated, atomic `BOARD.md` writer | Empty value → exit 2; `none` → no-op exit 0; `none` + any other token → exit 2 |
| `scripts/board-refresh.md` | writer contract | Rewrite the `--surfaces` row, token-gate table, error table, exit codes |
| `scripts/docket-status.sh` | orchestrator; `board_pass` | Empty `BOARD_SURFACES` → fatal exit 2; `none` → existing `board off` line |
| `scripts/docket-status.md` | orchestrator contract | Re-document `board off` as keyed on `none` |
| `skills/docket-convention/SKILL.md` | the shared contract | Board-pass paragraph → the facade call; drop `$BOARD_SURFACES` |
| `skills/docket-convention/references/terminal-close-out.md` | terminal close-out | 1 call site → facade |
| `skills/docket-new-change/SKILL.md` | | 3 call sites → facade; board commit splits out |
| `skills/docket-groom-next/SKILL.md` | | 1 call site → facade |
| `skills/docket-auto-groom/SKILL.md` | | 1 call site → facade |
| `skills/docket-finalize-change/SKILL.md` | | 1 call site → facade |
| `skills/docket-implement-next/SKILL.md` | | 1 call site → facade |
| `tests/test_docket_config.sh` | | Re-key `[]` asserts to `none`; add never-empty assert |
| `tests/test_board_refresh.sh` | | Re-key sections 2 + 5; add exit-2 and exclusivity cases |
| `tests/test_docket_status.sh` | | Re-key empty fixtures to `none`; add fatal-on-empty case |
| `tests/test_board_refresh_on_transition.sh` | | **Narrow** (not delete) onto the new canonical spelling |
| `tests/test_skill_facade_wiring.sh` | | **New Layer-3 structural sentinel** for the retired spellings |

**Not touched:** `scripts/render-board.sh` (pure renderer, unchanged since 0059), `tests/test_render_board.sh` (`REDIRECT_RE` + the write sentinel — ADR-0031 forbids collapsing them; they must stay GREEN, unmodified), `README.md` (the `[]` YAML spelling is unchanged), `.docket.yml`.

---

### Task 1: `docket-config.sh` never emits an empty `BOARD_SURFACES`

**Files:**
- Modify: `scripts/docket-config.sh:197-216`
- Modify: `scripts/docket-config.md:83`
- Test: `tests/test_docket_config.sh:101-107`, `:387-397`

**Interfaces:**
- Produces: the resolved `BOARD_SURFACES` value is **never the empty string**. `board_surfaces: []` (from any layer), and any layer combination whose tokens all get filtered out, resolve to the single token `none`. Every downstream task consumes this.

- [ ] **Step 1: Write the failing tests**

In `tests/test_docket_config.sh`, replace the section-(D) block (currently lines 101-107) with:

```bash
# --- (D) board_surfaces: [] -> the reserved `none` token, distinct from unset (change 0071) ----
# 0071 inverts the polarity: an EMPTY BOARD_SURFACES no longer means "board disabled" — it means
# "nobody resolved this", and every consumer now treats it as a wiring bug. The deliberate
# off-state is encoded POSITIVELY as `none`. Asserted on the EMITTED LINE, not on an eval'd
# variable: `eval "$out"` after a run that emitted nothing leaves the PREVIOUS case's value in
# place, so a variable assert cannot tell "emitted nothing" from "emitted the right thing".
mkrepo "$tmp/d"
printf 'metadata_branch: main\nboard_surfaces: []\n' > "$tmp/d/.docket.yml"
git -C "$tmp/d" add .docket.yml; git -C "$tmp/d" commit --quiet -m cfg
git -C "$tmp/d" push --quiet origin main
out="$(run "$tmp/d" --export)"
assert "board []: emits BOARD_SURFACES=none"           'printf "%s\n" "$out" | grep -qxF "BOARD_SURFACES=none"'
assert "board []: never emits an empty BOARD_SURFACES" '! printf "%s\n" "$out" | grep -qxF "BOARD_SURFACES="'
eval "$out"
assert "board []: BOARD_SURFACES is the none token"     '[ "$BOARD_SURFACES" = none ]'

# --- (D2) the never-empty invariant holds across layers (change 0071) --------------------------
# A global layer whose ONLY token is `github` is machine-fenced (0050) and filtered to nothing.
# That filtered-to-empty path is a second way the resolver used to emit "" — it must also land
# on `none`, or the sentinel has a hole exactly where the fence bites.
mkrepo "$tmp/d2"
mkdir -p "$tmp/d2.xdg/docket"
printf 'board_surfaces: [github]\n' > "$tmp/d2.xdg/docket/config.yml"
out="$(rung "$tmp/d2.xdg" "$tmp/d2" --export 2>/dev/null)"
assert "board fenced-to-empty: emits BOARD_SURFACES=none" \
  'printf "%s\n" "$out" | grep -qxF "BOARD_SURFACES=none"'
```

And in section (N) (currently line 395-397), replace the `[]`-honored assert:

```bash
printf 'board_surfaces: []\n' > "$tmp/n.xdg/docket/config.yml"
out="$(rung "$tmp/n.xdg" "$tmp/n" --export 2>/dev/null)"
assert "0050 N: global [] honored (board disabled, encoded as none)" \
  'printf "%s\n" "$out" | grep -qxF "BOARD_SURFACES=none"'
eval "$out"
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `bash tests/test_docket_config.sh`
Expected: `NOT OK - board []: emits BOARD_SURFACES=none`, `NOT OK - board []: never emits an empty BOARD_SURFACES`, `NOT OK - board fenced-to-empty: emits BOARD_SURFACES=none`, `NOT OK - 0050 N: global [] honored…`. Everything else `ok`.

- [ ] **Step 3: Implement — normalize at the single choke point**

In `scripts/docket-config.sh`, the board block currently ends at line 216 (`fi`). Append the normalizer immediately after that closing `fi`, so it catches BOTH the `[]` path and the fenced-to-empty path:

```bash
fi
# Change 0071 — the positive sentinel. BOARD_SURFACES is NEVER emitted empty. `board_surfaces: []`
# (and any layer combination whose tokens all get filtered out, e.g. a global `[github]` dropped by
# the machine-scope fence) resolves to the reserved token `none`. Empty therefore has exactly one
# meaning left downstream: *nobody resolved this* — a wiring bug, which board-refresh.sh and
# docket-status.sh now reject loudly instead of silently treating as "board disabled". `none` is
# reserved and exclusive; no real surface may ever be named `none`.
[ -n "$BOARD_SURFACES" ] || BOARD_SURFACES="none"
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `bash tests/test_docket_config.sh`
Expected: all `ok -`, exit 0. (The `direct-pipe: 19 KEY=value lines emitted` assert must still pass — no key was added or removed.)

- [ ] **Step 5: Mutation-test the never-empty assert**

Temporarily comment out the `[ -n "$BOARD_SURFACES" ] || BOARD_SURFACES="none"` line, run `bash tests/test_docket_config.sh`, and confirm the three new asserts go RED. Restore the line and confirm GREEN.

- [ ] **Step 6: Update the contract**

In `scripts/docket-config.md:83`, replace the `board_surfaces` row's derivation cell:

```
| `board_surfaces` | `inline` | yes, minus `github` | YAML list `[a, b]` stripped of brackets/commas; **`[]` → the reserved token `none`** (change 0071 — an empty value is NEVER emitted; empty means "unresolved", a wiring bug); a `github` token arriving from either machine-scoped layer (repo-local or global) is dropped (Stage 2c), and a list left empty by that drop also resolves to `none` |
```

Add to the same file's Invariants section (append a bullet):

```
- **`BOARD_SURFACES` is never emitted empty** (change 0071). The deliberate off-state is the
  positive token `none`; an empty value is reserved for "unresolved" and is a wiring bug every
  consumer rejects with exit 2.
```

- [ ] **Step 7: Commit**

```bash
git add scripts/docket-config.sh scripts/docket-config.md tests/test_docket_config.sh
git commit -m "feat(0071): docket-config.sh never emits an empty BOARD_SURFACES ([] -> none)"
```

---

### Task 2: `board-refresh.sh` — empty exits 2, `none` no-ops, `none` is exclusive

**Files:**
- Modify: `scripts/board-refresh.sh:9-17` (usage comment), `:41-43` (missing-flag guard), `:45-59` (token gate)
- Modify: `scripts/board-refresh.md:15-21` (usage/flags), `:32-40` (token-gate table), `:69-72` (errors), `:74-81` (exit codes)
- Test: `tests/test_board_refresh.sh:63-67` (section 2), `:83-89` (section 5), plus new sections

**Interfaces:**
- Consumes: `BOARD_SURFACES` from Task 1 — never empty, `none` when disabled.
- Produces: `board-refresh.sh --surfaces ""` → **exit 2**, stderr `board-refresh: empty --surfaces value (unresolved config?); pass --surfaces none to disable the board`. `--surfaces none` → **exit 0**, stdout `board-refresh: board disabled (none) — no-op`, `BOARD.md` untouched. `--surfaces "none inline"` → **exit 2**, stderr `board-refresh: 'none' is exclusive — it cannot be combined with other surfaces: none inline`. The missing-flag guard (exit 2) is UNCHANGED.

- [ ] **Step 1: Write the failing tests**

In `tests/test_board_refresh.sh`, replace section 2 (lines 63-67) with:

```bash
# --- 2: --surfaces "" (empty value) -> exit 2 (change 0071: the polarity reversal) -------------
# Was: a no-op exit 0, byte-identical to a deliberately disabled repo. That ambiguity is the whole
# bug — an agent whose $BOARD_SURFACES never resolved sent "" and got a SILENT stale board with a
# success exit code. Empty now means exactly one thing: nobody resolved this.
rm -f "$tmp/BOARD.md"
"$SCRIPT" --changes-dir "$tmp" --surfaces "" >"$work/out2" 2>"$work/err2"; rc2=$?
assert "empty surfaces: exit 2 (wiring bug, not a configuration)" '[ "$rc2" -eq 2 ]'
assert "empty surfaces: BOARD.md not created" '[ ! -e "$tmp/BOARD.md" ]'
assert "empty surfaces: names the unresolved-config cause on stderr" \
  'grep -qF "empty --surfaces value" "$work/err2"'
assert "empty surfaces: points at the positive off-token on stderr" \
  'grep -qF "none" "$work/err2"'
assert "empty surfaces: no leftover temp files in changes dir" '[ "$(count_files)" -eq 0 ]'

# --- 2b: --surfaces "none" -> the deliberate off-state: no-op, exit 0 --------------------------
rm -f "$tmp/BOARD.md"
"$SCRIPT" --changes-dir "$tmp" --surfaces "none" >"$work/out2b" 2>"$work/err2b"; rc2b=$?
assert "none: exit 0" '[ "$rc2b" -eq 0 ]'
assert "none: BOARD.md not created" '[ ! -e "$tmp/BOARD.md" ]'
assert "none: announces the deliberate no-op on stdout" \
  'grep -qF "board-refresh: board disabled (none)" "$work/out2b"'
assert "none: no leftover temp files in changes dir" '[ "$(count_files)" -eq 0 ]'

# --- 2c: `none` is RESERVED and EXCLUSIVE — a contradiction exits 2, never picks a winner ------
rm -f "$tmp/BOARD.md"
"$SCRIPT" --changes-dir "$tmp" --surfaces "none inline" >"$work/out2c" 2>"$work/err2c"; rc2c=$?
assert "none+inline: exit 2 (contradiction)" '[ "$rc2c" -eq 2 ]'
assert "none+inline: BOARD.md not written" '[ ! -e "$tmp/BOARD.md" ]'
assert "none+inline: says none is exclusive on stderr" 'grep -qF "exclusive" "$work/err2c"'
# order-independent: the contradiction is a property of the token SET, not of token order
rm -f "$tmp/BOARD.md"
"$SCRIPT" --changes-dir "$tmp" --surfaces "inline none" >"$work/out2d" 2>"$work/err2d"; rc2d=$?
assert "inline+none (reversed order): exit 2" '[ "$rc2d" -eq 2 ]'
assert "inline+none (reversed order): BOARD.md not written" '[ ! -e "$tmp/BOARD.md" ]'
```

Replace section 5 (lines 83-89) with the same truncation trap re-keyed onto `none`, plus its exit-2 twin:

```bash
# --- 5: truncation-trap regression — a pre-existing BOARD.md survives a DISABLED run byte-for-byte
# Change 0059's entire point, carried over unchanged except for the encoding (`""` -> `none`).
# Non-negotiable: disabling the board must never create, truncate, or delete a prior BOARD.md.
rm -f "$tmp/BOARD.md"
printf '# Stale Board\n\nDo not touch.\n' > "$tmp/BOARD.md"
cp "$tmp/BOARD.md" "$work/known-board.md"
"$SCRIPT" --changes-dir "$tmp" --surfaces "none" >"$work/out5" 2>"$work/err5"; rc5=$?
assert "truncation trap (none): exit 0" '[ "$rc5" -eq 0 ]'
assert "truncation trap (none): pre-existing BOARD.md untouched (byte-identical)" \
  'diff -u "$work/known-board.md" "$tmp/BOARD.md"'

# --- 5b: the exit-2 paths must ALSO leave a pre-existing BOARD.md byte-identical ---------------
# A loud failure that truncates the board on its way out would trade a silent stale board for a
# loud destroyed one. Both rejection paths (empty value, exclusivity violation) are asserted.
"$SCRIPT" --changes-dir "$tmp" --surfaces "" >"$work/out5b" 2>"$work/err5b"; rc5b=$?
assert "empty surfaces: exit 2 leaves a pre-existing BOARD.md byte-identical" \
  '[ "$rc5b" -eq 2 ] && diff -u "$work/known-board.md" "$tmp/BOARD.md"'
"$SCRIPT" --changes-dir "$tmp" --surfaces "none inline" >"$work/out5c" 2>"$work/err5c"; rc5c=$?
assert "none+inline: exit 2 leaves a pre-existing BOARD.md byte-identical" \
  '[ "$rc5c" -eq 2 ] && diff -u "$work/known-board.md" "$tmp/BOARD.md"'
rm -f "$tmp/BOARD.md"
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `bash tests/test_board_refresh.sh`
Expected: `NOT OK - empty surfaces: exit 2 …` (it currently exits 0), `NOT OK - none: announces the deliberate no-op …` (it currently prints `inline disabled — no-op`), `NOT OK - none+inline: exit 2 (contradiction)` (it currently renders the board!), etc. Sections 1, 3, 4, 6, 7, 8 stay `ok`.

- [ ] **Step 3: Implement the token gate**

In `scripts/board-refresh.sh`, replace the usage comment lines 9-17 with:

```bash
# Usage: board-refresh.sh --changes-dir DIR --surfaces "TOKENS" [--repo OWNER/REPO]
#   --changes-dir DIR   required; the metadata working tree (active/, archive/, BOARD.md live here).
#   --surfaces "TOKENS"  required AS A FLAG, and its value must be NON-EMPTY (change 0071).
#                        Space-separated tokens (the caller's resolved $BOARD_SURFACES, verbatim).
#                        A missing flag is a wiring bug (exit 2). An EMPTY VALUE is also a wiring
#                        bug (exit 2) — it means the caller never resolved its config. The
#                        deliberate off-state is the reserved, exclusive token `none` (no-op,
#                        exit 0, BOARD.md never created or truncated).
#   --repo OWNER/REPO   optional; forwarded verbatim to render-board.sh.
# Only the exact `inline` token enables a render+write. Unknown tokens (typos, `github`) warn to
# stderr and are ignored — they never abort and never block `inline` from taking effect.
```

Replace the `SURFACES_SET` guard (lines 41-43) with the missing-flag guard PLUS the new empty-value guard:

```bash
[ "$SURFACES_SET" -eq 1 ] || {
  printf 'board-refresh: missing --surfaces (pass --surfaces none to disable the board)\n' >&2; exit 2;
}
# Change 0071 — the positive sentinel. An empty VALUE is a wiring bug, not a configuration: it is
# what an unresolved `$BOARD_SURFACES` degrades to, and treating it as "board disabled" is how a
# stale board used to ship behind a success exit code. The deliberate off-state must SAY so.
[ -n "$SURFACES" ] || {
  printf 'board-refresh: empty --surfaces value (unresolved config?); pass --surfaces none to disable the board\n' >&2
  exit 2
}
```

Replace the token gate (lines 45-59) with:

```bash
# Tokenize --surfaces. `none` is the reserved, EXCLUSIVE off-token: it disables every surface and
# may not be combined with any other token (a contradiction exits 2 rather than silently picking a
# winner). Only the exact `inline` token enables a render+write; unknown tokens warn and are
# ignored (a typo must never abort a build).
inline_enabled=0
none_seen=0
other_seen=0
for tok in $SURFACES; do
  case "$tok" in
    none)   none_seen=1 ;;
    inline) inline_enabled=1; other_seen=1 ;;
    github) other_seen=1 ;;
    *) printf 'board-refresh: unknown surface token ignored: %s\n' "$tok" >&2; other_seen=1 ;;
  esac
done

if [ "$none_seen" -eq 1 ] && [ "$other_seen" -eq 1 ]; then
  printf "board-refresh: 'none' is exclusive — it cannot be combined with other surfaces: %s\n" "$SURFACES" >&2
  exit 2
fi

if [ "$none_seen" -eq 1 ]; then
  printf 'board-refresh: board disabled (none) — no-op\n'
  exit 0
fi

if [ "$inline_enabled" -eq 0 ]; then
  printf 'board-refresh: inline disabled — no-op\n'
  exit 0
fi
```

Note: the `inline_enabled -eq 0` branch is still reached by a `github`-only or unknown-token-only value — that path is unchanged (a no-op exit 0), which is what keeps section 3 and section 6 green.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `bash tests/test_board_refresh.sh`
Expected: all `ok -`, exit 0.

- [ ] **Step 5: Mutation-test the new guards**

Three mutations, each run against `bash tests/test_board_refresh.sh`, each reverted after:
1. Delete the `[ -n "$SURFACES" ] || {…exit 2…}` guard → the empty-surfaces asserts must go RED.
2. Delete the `none_seen && other_seen` exclusivity block → the `none+inline` asserts must go RED.
3. Change the `none)` arm to fall through to `inline_enabled=1` → the truncation-trap assert must go RED.

If any mutation leaves the suite GREEN, the assert is decoration — fix the assert before continuing.

- [ ] **Step 6: Update the contract**

In `scripts/board-refresh.md`, update the `--surfaces` flag row (line 21):

```
| `--surfaces "TOKENS"` | **yes, as a flag, with a NON-EMPTY value** | The caller's already-resolved `$BOARD_SURFACES`, verbatim: space-separated tokens (e.g. `"inline"`, `"inline github"`, `"github"`, or `"none"`). The flag being **absent** is a wiring bug (exit 2). An **empty value** is ALSO a wiring bug (exit 2, change 0071) — it is what an unresolved config variable degrades to. The deliberate off-state is the reserved token **`none`** (no-op, exit 0), which is **exclusive**: combining it with any other token exits 2. |
```

Update the token-gate table (lines 35-40):

```
| Tokens (example) | Action |
|---|---|
| `"inline"` | Render via `render-board.sh` and replace `BOARD.md`. |
| `"inline github"` | Same as above — `github` is irrelevant to this script. |
| `"github"` | No-op: `BOARD.md` is left completely untouched. |
| `"none"` | Deliberate off-state: no-op, exit 0. `BOARD.md` is never created, written, truncated, or deleted. |
| `"none inline"` (any mix) | **Exit 2** — `none` is exclusive; a contradiction is never resolved silently. |
| `""` (empty) | **Exit 2** — a wiring bug (unresolved config), never a configuration. |
```

Update the error table (line 71) and exit codes (lines 78-80):

```
| Missing `--surfaces` flag | stderr | `board-refresh: missing --surfaces (pass --surfaces none to disable the board)` |
| Empty `--surfaces` value | stderr | `board-refresh: empty --surfaces value (unresolved config?); pass --surfaces none to disable the board` |
| `none` combined with another token | stderr | `board-refresh: 'none' is exclusive — it cannot be combined with other surfaces: <tokens>` |
```

```
| 0 | Either `BOARD.md` was rendered and written (`inline` present), or the run was a deliberate no-op (`none`, or `inline` simply absent) — both are success. |
| 2 | Argument/wiring error: `--changes-dir` missing or not a directory, `--surfaces` flag absent, `--surfaces` value **empty**, `none` combined with another token, or an unrecognized flag. |
```

Add to the file's Invariants section:

```
- **A rejection never writes.** Every exit-2 path (missing flag, empty value, `none` contradiction)
  leaves a pre-existing `BOARD.md` byte-identical — a loud failure must not trade a stale board for
  a destroyed one.
```

- [ ] **Step 7: Commit**

```bash
git add scripts/board-refresh.sh scripts/board-refresh.md tests/test_board_refresh.sh
git commit -m "feat(0071): board-refresh.sh exits 2 on an empty --surfaces; none is the exclusive off-token"
```

---

### Task 3: `docket-status.sh` — `board_pass` asserts the resolver produced a value

**Files:**
- Modify: `scripts/docket-status.sh:41-58` (`board_pass`)
- Modify: `scripts/docket-status.md:55-56`, `:167`
- Test: `tests/test_docket_status.sh` — the `write_board_fixture` call sites at ~`:301`, `:728`, `:773`, `:861-873`, `:1338`, `:1509`

**Interfaces:**
- Consumes: `BOARD_SURFACES` from Task 1 (never empty; `none` when disabled) and `board-refresh.sh` from Task 2.
- Produces: `docket.sh docket-status --board-only` — the ONE Board-pass entry point every skill calls in Task 4. Its stdout report lines are the caller's contract: `board off` (disabled) · `board inline clean` (no change) · `board inline changed pushed` (committed + pushed) · `board inline changed push-failed`. An empty `BOARD_SURFACES` is now a **fatal exit 2**, not a `board off`.

- [ ] **Step 1: Write the failing tests**

In `tests/test_docket_status.sh`, the `board_pass` empty-surfaces case (currently ~lines 300-312) becomes the FATAL case, and a new `none` case takes over the `board off` contract. Replace that block with:

```bash
# Change 0071: an EMPTY BOARD_SURFACES is no longer "board off" — it is an unresolved-config
# wiring bug, and the orchestrator that every skill's Board pass now routes through must fail
# LOUDLY rather than silently skip the board. This is the reference implementation of the
# polarity reversal: the guard that used to read "no surfaces => disabled" is now an assertion
# that the resolver produced a value at all.
write_board_fixture ""
(cd "$tmp/board-case/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-board.sh" "$SCRIPT" --board-only >"$tmp/board-run3.txt" 2>"$tmp/board-run3-err.txt")
rc=$?
assert "board_pass empty-surfaces run exits 2 (fatal wiring bug)" '[ $rc -eq 2 ]'
assert "board_pass empty-surfaces names the unresolved config on stderr" \
  'grep -qF "BOARD_SURFACES" "$tmp/board-run3-err.txt"'
assert "board_pass empty-surfaces NEVER reports 'board off'" \
  '! grep -qxF "board off" "$tmp/board-run3.txt"'
assert "board_pass empty-surfaces emits no inline board line" \
  '! grep -q "board inline" "$tmp/board-run3.txt"'

# --- the deliberate off-state (`none`) keeps TODAY's byte-identical `board off` report ----------
write_board_fixture none
(cd "$tmp/board-case/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-board.sh" "$SCRIPT" --board-only >"$tmp/board-run3n.txt" 2>"$tmp/board-run3n-err.txt")
rc=$?
assert "board_pass none run exits zero" '[ $rc -eq 0 ]'
assert "board_pass none emits a positive 'board off' line" \
  'grep -qxF "board off" "$tmp/board-run3n.txt"'
assert "board_pass none emits no inline board line" \
  '! grep -q "board inline" "$tmp/board-run3n.txt"'
```

Then re-key every OTHER disabled-board fixture from `""` to `none` — these all assert the board-off REPORT contract (`board off` + digest + `pass ok`), which `none` must preserve byte-for-byte:
- `:728` — `'BOARD_SURFACES='` → `'BOARD_SURFACES=none'`
- `:773` — `'BOARD_SURFACES='` → `'BOARD_SURFACES=none'`
- `:861-873` — the helper's `$1` default and its call site → pass `none`
- `:1338` — `write_board_fixture ""` → `write_board_fixture none`
- `:1509` — `write_board_fixture ""` → `write_board_fixture none`

Update the comment at `:1315` and `:1401` to say the board-off repo is encoded `none` (`board_surfaces: []` → `BOARD_SURFACES=none`). Leave every assertion's EXPECTED OUTPUT unchanged — `board off`, the digest lines, and `pass ok` must all still appear. That invariance IS the test: the off-state's observable behavior did not change, only its encoding.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `bash tests/test_docket_status.sh`
Expected: `NOT OK - board_pass empty-surfaces run exits 2 (fatal wiring bug)` (it currently exits 0 and prints `board off`), `NOT OK - board_pass none emits a positive 'board off' line` (the `none` token currently falls into the unknown-token arm and warns). The re-keyed `none` fixtures fail the same way.

- [ ] **Step 3: Implement `board_pass`**

In `scripts/docket-status.sh`, replace `board_pass` (lines 41-58) with:

```bash
board_pass(){
  local surfaces="${BOARD_SURFACES:-}"
  # Change 0071 — the polarity reversal, at its reference implementation. This guard used to read
  # `[ -n "$surfaces" ] || { echo "board off"; return 0; }` — i.e. an UNRESOLVED config produced
  # the DISABLED behavior, silently, with a success exit code. That is the bug. docket-config.sh
  # now never emits an empty BOARD_SURFACES (the off-state is the positive token `none`), so an
  # empty value here means exactly one thing: nobody resolved this. Fail closed and loudly —
  # main() runs board_pass FIRST, so a hard exit here never reaches `pass ok`.
  if [ -z "$surfaces" ]; then
    echo "docket-status: BOARD_SURFACES is empty — config was never resolved (a wiring bug). The deliberate off-state is 'none'." >&2
    exit 2
  fi
  # `none` is the reserved, EXCLUSIVE off-token: it disables every surface. Its report line is
  # byte-identical to the pre-0071 `board off` — a disabled repo's output must not change.
  local tok
  for tok in $surfaces; do
    if [ "$tok" = none ]; then
      if [ "$surfaces" != none ]; then
        echo "docket-status: 'none' is exclusive — it cannot be combined with other surfaces: $surfaces" >&2
        exit 2
      fi
      echo "board off"
      return 0
    fi
  done
  local mw
  if [ "${DOCKET_MODE:-}" = docket ]; then mw="${METADATA_WORKTREE:-.docket}"; else mw="."; fi
  local cd_dir="$mw/$CHANGES_DIR"
  for tok in $surfaces; do
    case "$tok" in
      inline) board_pass_inline "$mw" "$cd_dir" ;;
      github) board_pass_github "$cd_dir" ;;
      *) echo "docket-status: unknown board surface '$tok'" >&2 ;;
    esac
  done
}
```

`board_pass_inline` and `board_pass_github` are UNCHANGED — `board_pass_inline` already passes a **literal** `--surfaces inline` to `board-refresh.sh` (it is already immune to the trigger; only `board_pass`'s guard was not).

- [ ] **Step 4: Run the tests to verify they pass**

Run: `bash tests/test_docket_status.sh`
Expected: all `ok -`, exit 0.

- [ ] **Step 5: Mutation-test the fatal guard**

Revert the guard to the old `[ -n "$surfaces" ] || { echo "board off"; return 0; }` in the REAL script, run `bash tests/test_docket_status.sh`, confirm the empty-surfaces asserts go RED (this is the regression the whole change exists to prevent — it MUST redden). Restore.

- [ ] **Step 6: Update the contract**

In `scripts/docket-status.md`, update line 55-56:

```
**3. Board pass**, once per surface token in the space-separated `BOARD_SURFACES` config value.
The reserved token **`none`** is the deliberate off-state and emits a positive `board off` line
(change 0069) — never silence. An **empty** `BOARD_SURFACES` is a wiring bug, not a
configuration: the pass exits 2 with a diagnostic (change 0071), because `docket-config.sh` never
emits an empty value and an unresolved config must never masquerade as a disabled board.
```

And the report-line table at line 167:

```
| `board off` | `BOARD_SURFACES` is the reserved token `none` — the board is deliberately disabled (`board_surfaces: []`); no surface was rendered and nothing was committed. Positive evidence of a deliberate skip, never silence. |
```

Add to Exit codes: `| 2 | … or `BOARD_SURFACES` was empty / `none` was combined with another surface (a wiring bug — change 0071). |`

- [ ] **Step 7: Commit**

```bash
git add scripts/docket-status.sh scripts/docket-status.md tests/test_docket_status.sh
git commit -m "feat(0071): board_pass asserts the resolver produced a value; none maps to board off"
```

---

### Task 4: Collapse the 8 Board-pass call sites into one facade call

**Files:**
- Modify: `skills/docket-convention/SKILL.md:212` (the "Board refresh on status writes" paragraph)
- Modify: `skills/docket-convention/references/terminal-close-out.md:82`
- Modify: `skills/docket-new-change/SKILL.md:43`, `:51`, `:59`
- Modify: `skills/docket-groom-next/SKILL.md:71`
- Modify: `skills/docket-auto-groom/SKILL.md:56`
- Modify: `skills/docket-finalize-change/SKILL.md:55`
- Modify: `skills/docket-implement-next/SKILL.md:86`

**Interfaces:**
- Consumes: `docket.sh docket-status --board-only` from Task 3 — it self-resolves config, gates on surfaces, renders through `board-refresh.sh`, and commits + pushes `BOARD.md` with its own rebase-retry loop.
- Produces: the ONLY permitted Board-pass spelling in prose, which Task 6's sentinel enforces:
  `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh docket-status --board-only`
- **No surfaces value crosses the skill/script boundary anymore.** The skills also stop hand-rolling `git add`/`commit`/`push` for `BOARD.md` — the orchestrator owns that, including the diff-only decision (it commits only when `BOARD.md` actually changed) and the rebase-retry.

- [ ] **Step 1: Re-derive the call-site list by grep (never by hand)**

Run, from the feature worktree:

```bash
grep -rn -E '\-\-surfaces|BOARD_SURFACES|docket\.sh board-refresh' skills/
```

Expected: 8 `--surfaces` sites across 6 files (auto-groom 1, terminal-close-out 1, finalize 1, groom-next 1, implement-next 1, new-change 3), plus the descriptive `$BOARD_SURFACES` mention in `docket-convention/SKILL.md:212`. Every one of them must be gone by the end of this task **except** pure NOUN mentions of `board-refresh.sh` (`docket-convention/SKILL.md:238`'s "Derived-view script family"; `docket-status/SKILL.md:57,80`), which ADR-0030 explicitly permits and which must stay.

- [ ] **Step 2: Rewrite the convention's Board-pass paragraph**

`skills/docket-convention/SKILL.md:212` — replace the "Board refresh on status writes" paragraph with:

```markdown
**Board refresh on status writes.** Any skill that writes a change's `status:` refreshes the board immediately after — the **Board pass** — by invoking the one facade call `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh docket-status --board-only`. That orchestrator owns the whole decision: it resolves config itself (fail-closed), gates on the enabled surfaces, renders the `inline` surface through the gated `board-refresh.sh` writer, runs the `github` mirror upsert (best-effort), and commits + pushes `BOARD.md` on `metadata_branch` **only if it actually changed** — a separate commit from the `status:` write, with its own rebase-retry. **No surfaces value is ever passed by a skill**: the caller never resolves, spells, or forwards one, which is what makes an unresolved config impossible to mistake for a disabled board. The pass reports its outcome on stdout — `board off` (the board is deliberately disabled, `board_surfaces: []`), `board inline clean` (nothing changed), `board inline changed pushed`, or `board inline changed push-failed` — and **callers key on that report line, never on the exit code**. A repo with `board_surfaces: []` renders and commits nothing, and a pre-existing `BOARD.md` is left untouched rather than truncated. The board is a derived view and must never trail the change files.
```

- [ ] **Step 3: Rewrite the 5 must-land call sites**

Each of these is a **must-land** Board pass (retry until the board lands). The canonical replacement text — adapt only the surrounding sentence, never the command:

> invoke `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh docket-status --board-only` — the single Board-pass entry point; it renders, commits, and pushes `BOARD.md` itself (a separate commit, only if the board changed). **Must-land:** key on the stdout report line, not the exit code — re-invoke until it reports `board inline changed pushed`, `board inline clean`, or `board off`; on `board inline changed push-failed`, re-run `docket.sh preflight` and invoke it again.

Apply to:
- `skills/docket-new-change/SKILL.md:43` (step 5, Brainstorm mode). **Also split the board commit out** of the change+spec commit: the step now (a) commits the change + spec and pushes to `origin/docket` as the must-land content commit, THEN (b) runs the Board pass, which makes its own separate board commit. This aligns `docket-new-change` with the separate-board-commit rule every other skill already follows (the convention mandates it so a claim CAS stays byte-identical across concurrent agents; `docket-new-change` was the outlier).
- `skills/docket-new-change/SKILL.md:51` (Scan mode) — same split: commit the stubs and push, THEN run the Board pass.
- `skills/docket-new-change/SKILL.md:59` (proposed-kill) — keep the "must-land Board pass" phrasing (a test anchors on it).
- `skills/docket-groom-next/SKILL.md:71`
- `skills/docket-auto-groom/SKILL.md:56`
- `skills/docket-finalize-change/SKILL.md:55` — keep its `BOARD.md` "is **never** published to the integration branch" sentence verbatim (a test anchors on `is **never** published`).

- [ ] **Step 4: Rewrite the 2 best-effort call sites**

- `skills/docket-implement-next/SKILL.md:86` (the "Best-effort board refresh" section) — replace the invocation + hand-rolled git with:

```markdown
The Board pass this skill runs after its own status writes (claim, reconcile-kill, `implemented`) is **best-effort**: invoke `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh docket-status --board-only` — the single Board-pass entry point, which renders, commits, and pushes `BOARD.md` itself (a separate commit from the `status:` write, only if the board changed) — then **log whatever report line it prints and continue**; never abort the build for it. A `board off` line means the repo disabled the board and there is nothing to do; `board inline clean` means the render didn't change anything. The build's correctness rests on the change-file CAS, not the board; any residual staleness self-heals at the next must-land Board pass (the next change's Step 0 `docket-status`, a manual `docket-status`, or finalize). The board is always a **separate commit** from the `status:` write (keeping the claim CAS byte-identical across concurrent agents).
```

Keep the section heading `### Best-effort board refresh` and the three `run the Board pass (best-effort` call-site phrases elsewhere in the file EXACTLY as they are — `tests/test_board_refresh_on_transition.sh` anchors on both (`grep -c "run the Board pass (best-effort" >= 3`).

- `skills/docket-convention/references/terminal-close-out.md:82` — replace the invocation with the same facade call, preserving the surrounding best-effort/must-land per-caller posture that the reference already defines.

- [ ] **Step 5: Verify the retired spellings are gone**

Run:

```bash
grep -rn -E '\-\-surfaces|BOARD_SURFACES|docket\.sh board-refresh' skills/ ; echo "exit=$?"
```

Expected: **no matches**, `exit=1`. (Pure noun mentions of `board-refresh.sh` — no `docket.sh` prefix, no `--surfaces` — remain in `docket-convention/SKILL.md:238` and `docket-status/SKILL.md`; they do not match this pattern.)

Then confirm the new spelling landed in all 6 rewired files:

```bash
grep -rlF 'docket.sh docket-status --board-only' skills/ | sort
```

Expected exactly: `skills/docket-auto-groom/SKILL.md`, `skills/docket-convention/SKILL.md`, `skills/docket-convention/references/terminal-close-out.md`, `skills/docket-finalize-change/SKILL.md`, `skills/docket-groom-next/SKILL.md`, `skills/docket-implement-next/SKILL.md`, `skills/docket-new-change/SKILL.md`.

- [ ] **Step 6: Commit**

```bash
git add skills/
git commit -m "feat(0071): collapse 8 Board-pass call sites into docket.sh docket-status --board-only"
```

---

### Task 5: Narrow the transition guard onto the new canonical spelling

**Files:**
- Modify: `tests/test_board_refresh_on_transition.sh:36-56`

**Interfaces:**
- Consumes: the rewired prose from Task 4.
- Produces: the guard that keeps every status-writing skill wired to a **gated** board write.

**Why narrow, not delete (ADR-0031 / LEARNINGS).** This test's loop currently asserts each of the 5 status-writing skills (a) names `docket.sh board-refresh`, (b) states "only if BOARD.md changed", and (c) mentions `git status --porcelain`. Task 4 legitimately invalidates all three — the skills no longer invoke the writer, and the diff-only decision moved INTO the orchestrator. Deleting the guard is how the hole reopens. The property that is still load-bearing is: **every status-writing skill has a Board site that routes the board write through the deterministic gated pipeline — never a hand-render, never a raw redirect, never a bare "docket-status will get to it eventually" delegation.** Re-key onto that. The diff-only property it used to assert in prose is now asserted where it actually lives — `tests/test_docket_status.sh`'s "board_pass second (clean) run reports clean" (the orchestrator does not commit an unchanged board).

- [ ] **Step 1: Rewrite the guard's second half**

Replace lines 31-56 of `tests/test_board_refresh_on_transition.sh` with:

```bash
# --- change 0059 Task 3, NARROWED by change 0071 -----------------------------------------------
# 0059 asserted that every Board-pass caller named `board-refresh.sh` and hand-stated the
# diff-only commit rule. 0071 collapses all 8 call sites into ONE facade call
# (`docket.sh docket-status --board-only`): the orchestrator now owns the render, the diff-only
# decision, the commit, and the push, and NO surfaces value crosses the skill/script boundary.
# The prose clauses 0059 anchored on are therefore gone BY DESIGN.
#
# This guard is NARROWED, never deleted (ADR-0031: deleting a sentinel is how the guarded hole
# reopens). The property that is still load-bearing: every status-writing skill routes its board
# write through the deterministic gated pipeline at its Board site — never a hand-render, never a
# raw redirect, never a bare "docket-status will get to it eventually" delegation.
#
# The diff-only rule 0059 asserted in PROSE is now asserted where it actually executes:
# tests/test_docket_status.sh ("board_pass second (clean) run reports clean") proves the
# orchestrator does not commit an unchanged board.
BOARD_PASS_CALL="docket.sh docket-status --board-only"

# E. The convention still names board-refresh.sh as the gated inline writer (a NOUN mention —
# permitted by ADR-0030, and load-bearing: it is what documents the single write choke point).
assert "convention names board-refresh.sh (the gated inline writer)" \
  'grep -q "board-refresh.sh" skills/docket-convention/SKILL.md'

# E2. The convention defines the ONE Board-pass call, and states the report-line contract that
# replaced the hand-rolled diff check.
assert "convention defines the single Board-pass facade call" \
  "grep -qF \"$BOARD_PASS_CALL\" skills/docket-convention/SKILL.md"
assert "convention states the stdout report-line contract (not an exit code)" \
  'grep -qF "board inline changed pushed" skills/docket-convention/SKILL.md'

CALLERS=(
  skills/docket-new-change/SKILL.md
  skills/docket-groom-next/SKILL.md
  skills/docket-auto-groom/SKILL.md
  skills/docket-finalize-change/SKILL.md
  skills/docket-implement-next/SKILL.md
)

for f in "${CALLERS[@]}"; do
  name="$(basename "$(dirname "$f")")"
  assert "$name routes its Board site through the single facade call" \
    "grep -qF \"$BOARD_PASS_CALL\" \"$f\""
  # The retired shapes must be GONE: a skill that still spells a surfaces value is a skill that
  # can still send an unresolved one.
  assert "$name no longer spells a surfaces value at its Board site" \
    "! grep -qE '\-\-surfaces|BOARD_SURFACES' \"$f\""
done
```

Keep asserts A-D (lines 13-29) exactly as they are — they anchor on phrases Task 4 deliberately preserved.

- [ ] **Step 2: Run the test**

Run: `bash tests/test_board_refresh_on_transition.sh`
Expected: all `ok -`, exit 0.

- [ ] **Step 3: Mutation-test the narrowed guard**

In the REAL tree, revert ONE skill's Board site to the old `docket.sh board-refresh --changes-dir .docket/<changes_dir> --surfaces "$BOARD_SURFACES"` spelling. Run `bash tests/test_board_refresh_on_transition.sh` and confirm BOTH asserts for that skill go RED (the facade call is missing AND a surfaces value reappeared). Restore.

- [ ] **Step 4: Commit**

```bash
git add tests/test_board_refresh_on_transition.sh
git commit -m "test(0071): narrow the transition guard onto the single Board-pass facade call"
```

---

### Task 6: The structural sentinel — no retired spelling may return to skill prose

**Files:**
- Modify: `tests/test_skill_facade_wiring.sh` (add a Layer 3 after the existing Layer 2, before `exit $fail`)

**Interfaces:**
- Consumes: the rewired prose from Task 4.
- Produces: the completeness guard the spec requires — grep-derived and mutation-tested, so the call-site list can never silently regrow. This is what turns "we fixed 8 sites today" into "a 9th cannot appear."

**Why here.** `tests/test_skill_facade_wiring.sh` (change 0072) already owns exactly the scope this sentinel needs — `skills/*/SKILL.md` + `skills/docket-convention/references/*.md`, glob-derived, with a proven code-unit extractor (indented fences included, per LEARNINGS) and canonical-form stripping. It is an INDEPENDENT scan added alongside the existing ones, not a widening of any of them (ADR-0031).

**Scope discrimination (ADR-0030), restated because it is load-bearing.** Clauses 1 and 2 (`BOARD_SURFACES`, `--surfaces`) are forbidden across the WHOLE file: neither string has any legitimate descriptive use in skill prose (the YAML key is the lowercase `board_surfaces`, a different string, and it stays). Clause 3 forbids an **invocation** (`docket.sh board-refresh`) and is checked in code units only — the bare NOUN `board-refresh.sh` is permitted and is asserted PRESENT in the convention by Task 5.

- [ ] **Step 1: Write the failing sentinel**

Append to `tests/test_skill_facade_wiring.sh`, immediately before the final `exit $fail`:

```bash
# ---- Layer 3: the board-surfaces sentinel (change 0071) ----------------------------------------
# 0071 removed every surfaces value from skill prose: the Board pass is now the single facade call
# `docket.sh docket-status --board-only`, and the orchestrator self-resolves its config. This
# sentinel is what keeps it that way — the call-site list is grep-derived, so it can never
# silently regrow (LEARNINGS: derive gated call-site lists by grep, never by hand; guard the
# list's completeness with a sentinel, not with review attention).
#
# SCOPE DISCRIMINATION (ADR-0030), and why the three clauses differ:
#   * `BOARD_SURFACES` and `--surfaces` are forbidden across the WHOLE file. Neither has any
#     legitimate descriptive use in skill prose — the config KEY is the lowercase `board_surfaces`
#     (a different string, deliberately untouched). Whole-file (not code-unit) scoping is the
#     stronger guard here and costs nothing.
#   * `docket.sh board-refresh` is forbidden as an INVOCATION, in code units. The bare noun
#     `board-refresh.sh` is PERMITTED (the convention's "Derived-view script family" and
#     docket-status's ownership prose both legitimately NAME it; test_board_refresh_on_transition.sh
#     asserts the convention still does). Forbidding the noun would over-scope into prose whose job
#     is to describe the mechanism — the exact rejected alternative in ADR-0030.
B_SURF_VAR='BOARD_SURFACES'          # the shell variable, in any spelling ($X, "$X", ${X})
B_SURF_FLAG='--surfaces'             # the retired flag
B_REFRESH_CALL='docket.sh board-refresh'   # the retired direct invocation

board_pass_files=0
for f in "${SCOPE[@]}"; do
  rel="${f#$REPO/}"
  units="$(extract_code_units "$f")"

  assert "no BOARD_SURFACES variable survives anywhere in $rel" \
    '! grep -qF "$B_SURF_VAR" "$f"'
  assert "no --surfaces flag survives anywhere in $rel" \
    '! grep -qF "$B_SURF_FLAG" "$f"'
  assert "no direct board-refresh invocation in code units of $rel" \
    '! printf "%s" "$units" | grep -qF "$B_REFRESH_CALL"'

  grep -qF 'docket.sh docket-status --board-only' "$f" && board_pass_files=$((board_pass_files + 1))
done

# NON-VACUITY (LEARNINGS: a guard that parses nothing passes everything — assert the unit count the
# extractor found, not just its verdict). The sweep above is only meaningful if the corpus is real
# and the canonical Board-pass call is actually PRESENT where it belongs: the 6 rewired files
# (5 status-writing skills + the convention + terminal-close-out = 7 files; docket-status/SKILL.md
# and docket-adr/SKILL.md have no Board site of their own).
assert "the in-scope corpus is non-empty (the sentinel actually scanned files)" \
  '[ "${#SCOPE[@]}" -ge 8 ]'
assert "the canonical Board-pass call is present in every rewired file (found $board_pass_files)" \
  '[ "$board_pass_files" -eq 7 ]'
```

Note: verify the expected `board_pass_files` count against the Task 4 Step 5 grep output before hardcoding `7`; if `terminal-close-out.md` or the convention carries the call more than once the count is per-FILE, so 7 is the file count, not the occurrence count.

- [ ] **Step 2: Run to verify it passes on the rewired tree**

Run: `bash tests/test_skill_facade_wiring.sh`
Expected: all `ok -`, exit 0 (Task 4 already removed every retired spelling).

- [ ] **Step 3: Mutation-test all three clauses — in the REAL tree**

LEARNINGS is explicit: a fixture battery only samples shapes you already thought of; inject the regression into the actual guarded file. For EACH mutation, run `bash tests/test_skill_facade_wiring.sh`, confirm the named assert goes **RED**, then revert:

1. Add `--surfaces "$BOARD_SURFACES"` back into `skills/docket-groom-next/SKILL.md`'s Board site → the `--surfaces` AND `BOARD_SURFACES` asserts for that file must redden.
2. Add a `docket.sh board-refresh --changes-dir X --surfaces inline` invocation inside a fenced code block in `skills/docket-finalize-change/SKILL.md` → the direct-invocation assert must redden.
3. Add the same invocation inside an **indented** (list-item) fenced block in `skills/docket-convention/references/terminal-close-out.md` → it must ALSO redden. This is the exact extractor bug 0072 shipped and fixed (a column-0 fence anchor silently dropped every indented fence); this mutation proves the extractor still sees indented fences.
4. Delete the Board-pass call from `skills/docket-auto-groom/SKILL.md` → the `board_pass_files` count assert must redden (proving the presence half is not vacuous).

If ANY mutation leaves the suite green, the assert is decoration — fix it before continuing.

- [ ] **Step 4: Commit**

```bash
git add tests/test_skill_facade_wiring.sh
git commit -m "test(0071): structural sentinel — no surfaces value may return to skill prose"
```

---

### Task 7: Full-suite sweep + contract coverage

**Files:**
- Modify: whatever the suite reddens (expected candidates: `tests/test_convention_extraction.sh`, `tests/test_composition_wiring.sh`, `tests/test_script_contracts_coverage.sh`)

- [ ] **Step 1: Run the whole suite, in the FOREGROUND**

```bash
for f in tests/test_*.sh; do echo "== $f"; bash "$f" >/tmp/0071-suite.log 2>&1 || echo "FAILED: $f"; done
```

Better (keeps per-test output): run it once capturing everything, ~10 minutes, ONE foreground call:

```bash
for f in tests/test_*.sh; do echo "===== $f"; bash "$f" 2>&1 | grep -E "^NOT OK|^ok" | grep -c "^ok" >/dev/null; bash "$f" >/dev/null 2>&1 || echo "FAILED: $f"; done
```

Simplest and sufficient: `for f in tests/test_*.sh; do bash "$f" >/dev/null 2>&1 || echo "FAILED: $f"; done`

Expected: no `FAILED:` lines.

- [ ] **Step 2: Triage every failure against the base**

A RED test is a hypothesis, not a verdict (LEARNINGS, environment family). For each `FAILED:` test, re-run the IDENTICAL test against unmodified `origin/main` (`git stash` is not enough across a worktree — use `git -C /Users/homer/dev/docket show origin/main:tests/<name>.sh`  into a temp file, or check out a scratch worktree) and byte-compare the failing assert sets. Fix only the differential that this change actually caused; record any environment-bound failure rather than "fixing" it.

- [ ] **Step 3: Fix the real fallout**

Likely: a convention-extraction or composition-wiring test anchored on a phrase Task 4 rewrote. Re-key the anchor onto the surviving load-bearing phrase — **narrow, never delete** (ADR-0031).

- [ ] **Step 4: Verify the disabled-board promise end-to-end**

The single most important behavioral invariant of this change is that a disabled repo's `BOARD.md` is never touched. Prove it directly:

```bash
bash tests/test_board_refresh.sh 2>&1 | grep -i "truncation trap"
```

Expected: `ok - truncation trap (none): exit 0`, `ok - truncation trap (none): pre-existing BOARD.md untouched (byte-identical)`, and both exit-2 twins green.

- [ ] **Step 5: Commit**

```bash
git add -u tests/
git commit -m "test(0071): re-key suite anchors onto the consolidated Board pass"
```

---

## Self-Review

**Spec coverage.**
- Spec §1 "positive sentinel" — `docket-config.sh` never empty (Task 1); `board-refresh.sh` `--surfaces` required-as-flag preserved, empty exits 2, `none` no-ops, `none` exclusive, unknown tokens still warn-and-ignore (Task 2); `docket-status.sh` `board_pass` `none` arm + assertion (Task 3). ✅
- Spec §2 "one Board pass" — 8 sites → `docket.sh docket-status --board-only` (Task 4); `docket-new-change`'s board commit splits out (Task 4 Step 3); must-land keys on the stdout report line, `docket-implement-next` stays best-effort (Task 4 Steps 3-4). ✅
- Spec §3 "guards" — structural sentinel, grep-derived + mutation-tested (Task 6); `docket-config.sh` never-empty asserted incl. every layer combination (Task 1 Step 1, case D2); `board-refresh.sh` exit-2/exclusivity/no-op/truncation-trap (Task 2 Step 1). ✅
- Spec §4 "ADR" — recorded at step 6 of `docket-implement-next` via the `docket-adr` subagent, NOT on this branch (an ADR is metadata; it lives on `metadata_branch`). The reversal to record: change 0059's *"an explicit empty value means 'no surfaces configured'"* → **a deliberate off-state must be encoded positively; absence and emptiness are reserved for error.** `relates_to: [28]`. A SECOND decision surfaced during planning and should be raised with it or as its own ADR: the diff-only/porcelain commit rule migrated from skill PROSE into the orchestrator, which narrowed (never deleted) the 0059 transition guard — see Task 5. ✅
- Spec Out-of-scope — `board_surfaces: []` YAML semantics unchanged (Global Constraints); `$SKILL_*` family untouched; `github-mirror.sh` untouched; no stale-board retrofit. ✅

**Placeholder scan.** No TBDs. Every code step carries the actual code. Task 7's Step 2 names the technique rather than the exact fix because the fallout set is not knowable until the suite runs — that is a triage procedure, not a placeholder.

**Type consistency.** The report lines (`board off`, `board inline clean`, `board inline changed pushed`, `board inline changed push-failed`) are produced by `board_pass`/`board_pass_inline` (Task 3, unchanged strings) and consumed by the prose contract (Task 4) and the sentinel (Tasks 5, 6) under exactly those spellings. The token `none` and the exit code `2` are consistent across Tasks 1, 2, 3. The canonical prose call is byte-identical in Tasks 4, 5, and 6: `docket.sh docket-status --board-only`.
