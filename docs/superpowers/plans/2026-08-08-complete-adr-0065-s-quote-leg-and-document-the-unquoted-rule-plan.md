<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0255 — Complete ADR-0065's quote leg and document the unquoted rule](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0255-complete-adr-0065-s-quote-leg-and-document-the-unquoted-rule.md)**
<!-- docket:backlink:end -->

# Complete ADR-0065's quote leg and document the unquoted rule — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close ADR-0065's quote leg in `hd_validate`, make a `#` inside an `agents:` entry's flow map a loud validation failure in **both** validators instead of a silent truncation, and state the resulting one-sentence rule at the five places a user actually reads before writing config.

**Architecture:** Three surfaces, no new abstractions. (1) `scripts/lib/harness-defaults.sh` gets the quote leg its twin `sync-agents.sh` already has, copied inline — the library header forbids coupling the shipped-data reader to the user-config readers, and helper extraction is explicitly deferred to change #0256. (2) Both files gain a *pre-strip view* of an entry line plus a small pure-shell predicate over it, because both readers strip `#` before any validator sees the value; the strip order is deliberately left alone so legitimate trailing and full-line comments keep working. (3) One identical sentence lands as a comment at five documentation points of use.

**Tech Stack:** Bash (3.2-compatible), awk, the repo's own `tests/*.sh` assert-harness suite driven by `scripts/run-tests.sh`.

## Global Constraints

Copied verbatim from the spec and from `AGENTS.md` (always-in-context repo rules). Every task's requirements implicitly include this section.

- **Bash 3.2 compatibility is mandatory.** `sync-agents.sh` maintains two code paths (`BASH_VERSINFO[0] -lt 4` and the bash-4 `_LAYER_BODY_CACHE` path) and **both must be changed together** so they cannot disagree. No associative arrays, no `${var^^}`, no `mapfile` in new code.
- **Never `producer | early-exiting-consumer`** (`grep -q`, `head`, `head -n1`) — the suite runs under `set -o pipefail` and the producer takes SIGPIPE, turning into an intermittent 141. Capture into a variable first, then `grep <<<"$var"`.
- **awk indent classes are `[^[:space:]]`, never `[^ ]`** — a literal-space class silently drops tab-indented input (the suite has a tab-indented `.docket.local.yml` fixture that must keep passing).
- **Existing diagnostic strings stay byte-identical.** The string `is not a bare scalar — the reader consumes only '…'; write model/effort values unquoted and space-free` must not change in either file. Tests in `tests/test_sync_agents_validator.sh` pin it, and the two validators are specified to emit the same sentence.
- **A guard is code: mutation-test it.** After every new assert, revert the production edit it guards, confirm the assert goes RED, restore, confirm GREEN. Confirm each mutation actually landed with `grep -c` before and after — a substitution that silently fails to match yields a green run with nothing mutated.
- **Cross-references in maintained source anchor on a symbol name or a verbatim-quoted clause, never a line number.** `tests/test_comment_anchor_style.sh` rejects the filename-plus-line-number form. Write "the reference implementation in `sync-agents.sh`'s `validate_user_agent_values`", never "sync-agents.sh:606".
- **Run the whole suite at the build gate**, not just the tests this plan names: `scripts/run-tests.sh` (this repo's resolved `finalize.test_command`).
- **Use `/usr/bin/grep`, not bare `grep`, in new test asserts that pin a regex** — PATH `grep` here is ugrep and accepts syntax BSD grep rejects. The existing asserts in `tests/test_sync_agents_validator.sh` already do this; match the file's local idiom in each file you touch. **This binds mutation-test landing checks too:** `grep -c "case \"$raw\" in"` through PATH `grep` treats the `$` as an anchor and reports `0` on an *unmutated* file, which reads exactly like a landed mutation. Always `/usr/bin/grep -cF` for a landing check.
- **Never raise an existing row in `tests/runtime-budgets.tsv`.** `tests/test_runtime_budgets.sh` pins the table's TOTAL (`EXPECTED_TOTAL`) precisely so that a quiet per-row raise reddens, and its remedy text refuses the raise by name: *"A ceiling describes what a file already costs, so it moves when the file is re-shaped, not when it gets slower: shard the file, or move its new assertions into a shard with room."* The total legitimately moves in exactly two cases, and this change uses the first: **a new test file brings its own row**. Re-seed `EXPECTED_TOTAL` in the same diff with a dated comment naming the case, following the precedent comments already in that file. A row may never exceed the hard 60s ceiling, and the table's floor for a small file is 10s.
- The one sentence added by Task 4, byte-identical at all five sites:
  `Write model/effort values unquoted and space-free; \`#\` cannot appear inside the \`{…}\` flow map.`

---

## File Structure

| File | Responsibility | Task |
|---|---|---|
| `scripts/lib/harness-defaults.sh` | Modify: quote leg in `hd_validate`; `keep_comments` mode on `_hd_block`/`_hd_entry_line`; new `_hd_flow_map_has_comment`; `#` leg in `hd_validate` | 1, 2 |
| `tests/test_harness_defaults_flow_map.sh` | **Create**: every new `hd_validate` probe this change adds — quote fire probes (Task 1), `#` fire/ignore probes and the reader assert (Task 2) | 1, 2 |
| `tests/runtime-budgets.tsv` | Modify: add the new shard's own row (the guard's permitted new-test-file case) | 1 |
| `tests/test_runtime_budgets.sh` | Modify: re-seed `EXPECTED_TOTAL` by the new row, naming the case | 1 |
| `sync-agents.sh` | Modify: `keep_comments` 5th arg on `harness_agent_line` (both bash paths); new `flow_map_has_comment`; `#` leg in `validate_user_agent_values` | 3 |
| `tests/test_sync_agents_validator.sh` | Modify: `#` fire/ignore probes (0173's quote probes untouched) | 3 |
| `README.md` | Modify: the two `agents:` config examples | 4 |
| `skills/docket-convention/SKILL.md` | Modify: the `.docket.yml` schema block's `agents:` comment | 4 |
| `skills/docket-convention/references/agent-layer.md` | Modify: the example `agents:` block | 4 |
| `.docket.example.yml` | Modify: the `agents:` intro comment | 4 |

Task order matters: Task 1 and Task 2 both edit `hd_validate`'s per-entry loop, so Task 2 builds on Task 1's committed state. Task 3 is independent of 1–2 (different file) but is placed after them so the `#` predicate's shape is settled once before being duplicated. Task 4 depends on 1–3 only in that it documents the rules they enforce.

---

### Task 1: The quote leg in `hd_validate`

ADR-0065's rule, applied to the twin the rule was generalized *from*. A quoted but space-free value (`{model: "claude-opus-5"}`) has `consumed == raw` under both `hd_field` and `hd_field_raw`, so the existing `[ "$v" != "$raw" ]` leg structurally cannot see it and the quote characters ride into the emitted pin.

**Files:**
- Modify: `scripts/lib/harness-defaults.sh` — the `elif [ "$v" != "$raw" ]` branch inside `hd_validate`'s `for k in model effort` loop
- Create: `tests/test_harness_defaults_flow_map.sh` — a new shard holding every `hd_validate` probe this change adds
- Modify: `tests/runtime-budgets.tsv` — add the new shard's row
- Modify: `tests/test_runtime_budgets.sh` — re-seed `EXPECTED_TOTAL`

**Why a new shard rather than more asserts in `tests/test_harness_defaults_validator.sh`:** that file sits at 49.5s against a 50s row (measured), so its own margin is already gone, and every assert here costs a full `hd_validate` sweep (~3.3s). Adding this change's ~10 new sweeps to it would breach both its row and the hard 60s ceiling, and raising the row is exactly what the budget guard refuses. A new file is the guard's own first sanctioned case and the one with repeated precedent in `EXPECTED_TOTAL`'s comment history.

**Interfaces:**
- Consumes: `hd_field`, `hd_field_raw`, `hd_validate` (all existing, unchanged signatures)
- Produces: nothing new for later tasks. Task 2 edits the same `hd_validate` loop and must not disturb this branch's condition.

- [ ] **Step 1: Write the failing tests**

Create `tests/test_harness_defaults_flow_map.sh`. Its harness preamble mirrors `tests/test_harness_defaults_validator.sh`'s exactly — same `set -uo pipefail`, `REPO`, `assert`, library source, `HD`/`SRC`, `T`/`mut` scaffolding, and the same trailing `rm -rf "$T"` / `PASS`/`FAIL` epilogue — so the two shards stay recognizably one family:

```bash
#!/usr/bin/env bash
# tests/test_harness_defaults_flow_map.sh — value-level probes for hd_validate's two change-0255
# legs: the ADR-0065 quote leg, and the `#`-inside-the-flow-map leg. A separate shard from
# test_harness_defaults_validator.sh purely on cost: every assert here is one full hd_validate
# sweep (~3.3s), and that file's 50s row has no margin left.
# Run: bash tests/test_harness_defaults_flow_map.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

. "$REPO/scripts/lib/harness-defaults.sh"
HD="$REPO/agents/harness-defaults.yml"
SRC="$REPO/agents"

T="$(mktemp -d "${TMPDIR:-/tmp}/hd-flow-map.XXXXXX")"
mut(){ cp "$HD" "$T/hd.yml"; }
```

Then the quote probes. The probe already in `test_harness_defaults_validator.sh` uses a value **with spaces**, which the `!=` leg already catches; these use space-free values, which it cannot. Leave that existing probe where it is — do not move it.

```bash
# 0255 / ADR-0065: a quoted but SPACE-FREE value. `"claude-opus-5"` has consumed == raw under both
# readers, so the `!=` leg above structurally CANNOT see it — the quotes would ride into the
# emitted pin verbatim while the diagnostic's own remedy tells the user to write them unquoted.
# This pair is what pins the explicit quote leg, and it is the twin of the probe change 0173 added
# to tests/test_sync_agents_validator.sh for the user-config validator.
mut; sed -i.bak 's|^    adr:.*|    adr:                   { model: "claude-opus-5", effort: low }|' "$T/hd.yml"
assert "reject: double-quoted SPACE-FREE scalar" '! hd_validate "$T/hd.yml" "$SRC" 2>/dev/null'
dq_diag="$(hd_validate "$T/hd.yml" "$SRC" 2>&1 || true)"
assert "reject: double-quoted space-free diagnostic names the remedy" \
  'grep -q "unquoted and space-free" <<<"$dq_diag"'

mut; sed -i.bak "s|^    adr:.*|    adr:                   { model: 'claude-opus-5', effort: low }|" "$T/hd.yml"
assert "reject: single-quoted SPACE-FREE scalar" '! hd_validate "$T/hd.yml" "$SRC" 2>/dev/null'
```

Close the file with the same epilogue the sibling shard uses:

```bash
rm -rf "$T"

[ "$fail" = 0 ] && echo "PASS" || echo "FAIL"
exit "$fail"
```

Make it executable (`chmod +x`) to match its siblings.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `bash tests/test_harness_defaults_flow_map.sh`
Expected: FAIL — all three asserts print `NOT OK`, because `hd_validate` currently returns 0 for a quoted space-free value.

- [ ] **Step 3: Add the quote leg**

In `scripts/lib/harness-defaults.sh`, inside `hd_validate`, change the single-leg condition to the two-leg form. The `!=` leg stays **byte-for-byte** (ADR-0065's decision clause) and the diagnostic string is **unchanged**:

```bash
        elif [ "$v" != "$raw" ] || case "$raw" in '"'*|"'"*) true;; *) false;; esac; then
          # Two legs (ADR-0065). The `!=` leg catches anything the value class cannot express (an
          # embedded space). The quote leg catches what `!=` structurally CANNOT see: a quoted but
          # space-free value has v == raw, so the quotes would ride into the emitted pin verbatim
          # while this diagnostic's own remedy text tells the user to write them unquoted. Single
          # quotes included — the remedy says "unquoted", not "double-unquoted". Copied by value
          # from the reference implementation in sync-agents.sh's `validate_user_agent_values`;
          # this library's header forbids coupling the shipped-data reader to the user-config
          # readers, and extracting a shared helper is change #0256's scope, not this one's.
          echo "harness-defaults: $h/$a '$k' value '$raw' is not a bare scalar — the reader consumes only '$v'; write model/effort values unquoted and space-free" >&2; rc=1
```

Keep the existing comment lines that the new comment replaces only where they are redundant — the sentence about the empty clip blaming ABSENCE is still true and should survive somewhere in this block.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `bash tests/test_harness_defaults_flow_map.sh`
Expected: `PASS`, exit 0, with every assert `ok`.

- [ ] **Step 5: Mutation-test the new asserts**

Note the landing check uses `/usr/bin/grep -cF`: PATH `grep` is ugrep, which reads the `$` in the pattern as an anchor and reports `0` on an **unmutated** file — indistinguishable from a landed mutation, and the exact false-green this step exists to rule out.

```bash
cd /Users/homer/dev/docket/.worktrees/complete-adr-0065-s-quote-leg-and-document-the-unquoted-rule
/usr/bin/grep -cF 'case "$raw" in' scripts/lib/harness-defaults.sh              # expect 1
cp scripts/lib/harness-defaults.sh "${TMPDIR:-/tmp}/hd-backup.sh"               # NOT `git checkout --`: that restores HEAD and destroys this task's uncommitted work
perl -pi -e "s/ \|\| case \"\\\$raw\" in '\"'\*\|\"'\"\*\) true;; \*\) false;; esac//" scripts/lib/harness-defaults.sh
/usr/bin/grep -cF 'case "$raw" in' scripts/lib/harness-defaults.sh             # expect 0 — proves the mutation LANDED
bash tests/test_harness_defaults_flow_map.sh; echo "rc=$?"                      # expect FAIL / rc=1
cp "${TMPDIR:-/tmp}/hd-backup.sh" scripts/lib/harness-defaults.sh && rm "${TMPDIR:-/tmp}/hd-backup.sh"
bash tests/test_harness_defaults_flow_map.sh; echo "rc=$?"                      # expect PASS / rc=0
```

If the landing check reports 1 both times, the mutation did not land — fix the pattern and re-run. A green suite under a mutation that never applied is not evidence.

- [ ] **Step 6: Give the new shard its budget row**

The new file has no row yet, so `tests/test_runtime_budgets.sh` assertion (2) — completeness, both directions — fails until you add one. Measure first:

```bash
time bash tests/test_harness_defaults_flow_map.sh
```

Add the row to `tests/runtime-budgets.tsv` in the file's existing sort position, tab-separated, third column `parallel`:

```
tests/test_harness_defaults_flow_map.sh	<seconds>	parallel
```

Size `<seconds>` from the measurement **with headroom for Task 2**, which appends roughly six more `hd_validate` sweeps to this same file (~20s more). Sizing it once here avoids a second move of the pinned total inside one change. The table's floor is 10s and the hard ceiling is 60s; stay under 60 with real margin.

Then re-seed `EXPECTED_TOTAL` in `tests/test_runtime_budgets.sh` by exactly that row's value, adding a comment line at the TOP of the existing precedent list in the same style:

```
EXPECTED_TOTAL=<1365 + row>
                    # 1365 -> <new> (change 0255): the new-test-file case named below —
                    # tests/test_harness_defaults_flow_map.sh brings its own row, measured
                    # standalone at <measured>s and sized to <row>s to cover the `#`-leg probes
                    # task 2 appends to the same file.
```

Record the measured number in the commit message — a tolerance constant with no recorded measurement is wrong in both directions on any other machine.

**Do not touch the `tests/test_harness_defaults_validator.sh` row.** This change adds no assertions to that file, so its cost is unchanged.

- [ ] **Step 7: Run the whole suite**

Run: `scripts/run-tests.sh`
Expected: every file passes, `tests/test_runtime_budgets.sh` included. No `OVER BUDGET:` line for either harness-defaults shard.

- [ ] **Step 8: Commit**

```bash
git add scripts/lib/harness-defaults.sh tests/test_harness_defaults_flow_map.sh tests/runtime-budgets.tsv tests/test_runtime_budgets.sh
git commit -m "fix(0255): add ADR-0065's quote leg to hd_validate"
```

---

### Task 2: The `#`-inside-the-flow-map leg in `harness-defaults.sh`

`_hd_block` strips comments (`sub(/#.*/,"",nc)`) before either field reader runs, so `{model: c#5}` truncates to `c` with `v == raw == c` and passes every existing leg. The strip order stays as it is — reordering it would break legitimate trailing and full-line comments across every layer for a corner no real model ID exercises — so the corner is caught instead by validating a **pre-strip view** of the entry line.

**Files:**
- Modify: `scripts/lib/harness-defaults.sh` — `_hd_block`, `_hd_entry_line`, new `_hd_flow_map_has_comment`, `hd_validate`'s per-entry loop
- Test: `tests/test_harness_defaults_flow_map.sh` (the shard Task 1 created)

**Interfaces:**
- Consumes: `_hd_block(file, harness)`, `_hd_entry_line(file, harness, agent)` — both from Task 0 state, both extended here
- Produces:
  - `_hd_block "$file" "$harness" [keep_comments]` — `keep_comments=1` prints entry lines RAW (comments intact); any other value or omission preserves today's comment-stripped output exactly
  - `_hd_entry_line "$file" "$harness" "$agent" [keep_comments]` — forwards the flag
  - `_hd_flow_map_has_comment "$raw_entry_line"` — returns 0 when the line carries a `#` inside its `{…}` flow map, 1 otherwise. Task 3 duplicates this predicate by value into `sync-agents.sh` as `flow_map_has_comment`; the two bodies must stay identical apart from the name.

**The firing rule, stated once (both tasks implement exactly this):** the check *applies* only when the entry's first `{` precedes any `#` on the line — a `#` before the first `{`, or no `{` at all, never fires. When it applies, take the substring after that first `{`; it **fires** iff that substring contains a `#` before its first `}`, **or** contains a `#` and no `}` at all. Consequences: a trailing comment after `}` stays legal; a full-line comment never reaches the entry matcher; a commented-out map (`status: # {model: c#5}`) does **not** fire even though the comment contains a `#` after a `{`.

- [ ] **Step 1: Write the failing tests**

Append to `tests/test_harness_defaults_flow_map.sh`, after Task 1's quote probes and before the closing `rm -rf "$T"` epilogue.

```bash
# 0255: a `#` INSIDE the flow map. _hd_block strips comments before either reader runs, so
# `{ model: c#5 }` truncates to `c` with v == raw == c and slides past every leg above — the same
# silent-truncation class the quote leg exists to close, in a corner the value-comparison legs
# structurally cannot see. The strip ORDER is deliberately unchanged (reordering it would break
# legitimate trailing and full-line comments everywhere); the corner is caught on a PRE-STRIP view.
mut; sed -i.bak 's|^    adr:.*|    adr:                   { model: c#5, effort: low }|' "$T/hd.yml"
assert "reject: '#' inside the flow map" '! hd_validate "$T/hd.yml" "$SRC" 2>/dev/null'
h_diag="$(hd_validate "$T/hd.yml" "$SRC" 2>&1 || true)"
assert "reject: '#' diagnostic names the flow map, not bareness" \
  'grep -q "inside the flow map" <<<"$h_diag"'
# The remedy for a truncating `#` is NOT "write it unquoted" — a diagnostic that blames the wrong
# cause is the defect the split diagnostics in this file exist to prevent.
assert "reject: '#' diagnostic does not blame quoting" \
  '! grep -q "unquoted and space-free" <<<"$h_diag"'

# Ignore probes — the carve-outs. Over-rejection here would hard-abort generation on config styles
# used throughout .docket.example.yml and this very sidecar.
mut; sed -i.bak 's|^    adr:.*|    adr:                   { model: claude-opus-5, effort: low }   # a trailing note|' "$T/hd.yml"
assert "accept: a trailing comment AFTER the closing brace is legal" \
  'hd_validate "$T/hd.yml" "$SRC" 2>/dev/null'

# A commented-out map is field-absent post-strip. In the SIDECAR that is already an error — but the
# CORRECT one (missing field), not a truncation complaint. Pinning which diagnostic fires is the
# point: the `#` leg must stay silent here.
mut; sed -i.bak 's|^    adr:.*|    adr:                   # { model: c#5, effort: low }|' "$T/hd.yml"
c_diag="$(hd_validate "$T/hd.yml" "$SRC" 2>&1 || true)"
assert "accept: a commented-out map does not fire the '#' leg" \
  '! grep -q "inside the flow map" <<<"$c_diag"'
assert "reject: a commented-out map is a MISSING-field error instead" \
  'grep -q "missing a non-empty" <<<"$c_diag"'

# The raw view must select the SAME line the stripped view does, or the two disagree about WHICH
# entry is being judged — the failure mode that makes a duplicated gate diverge on exactly the
# inputs it was written to catch.
mut; sed -i.bak 's|^    adr:.*|    adr:                   { model: claude-opus-5, effort: low }   # note|' "$T/hd.yml"
assert "reader: keep_comments view returns the same entry, comments intact" \
  '[ "$(_hd_entry_line "$T/hd.yml" claude adr 1)" != "$(_hd_entry_line "$T/hd.yml" claude adr)" ] &&
   grep -q "^    adr:" <<<"$(_hd_entry_line "$T/hd.yml" claude adr 1)" &&
   grep -q "note" <<<"$(_hd_entry_line "$T/hd.yml" claude adr 1)"'
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `bash tests/test_harness_defaults_flow_map.sh`
Expected: FAIL. The `reject: '#' inside the flow map` asserts fail (validator returns 0 today) and the `keep_comments` reader assert fails (the flag is ignored, so both calls return identical stripped text). The two accept-probes may already pass — that is expected; they are the over-rejection floor for Step 3, and a guard that is green before AND after the change is only meaningful as a floor.

- [ ] **Step 3: Add the `keep_comments` mode and the predicate**

In `scripts/lib/harness-defaults.sh`, replace `_hd_block` and `_hd_entry_line` with the flag-bearing forms. Boundary logic keeps keying on the stripped view `nc` in **both** modes, which is what guarantees the two views select the identical set of lines:

```bash
# Print the body lines under `  <harness>:` (four-space-indented entries).
# keep_comments=1 prints each entry line RAW (comments intact); anything else strips them, which is
# what every reader wants. Block-boundary logic keys on the STRIPPED view in both modes, so the two
# views can never select a different set of lines — only a different rendering of the same ones.
_hd_block(){ # $1=file $2=harness [$3=keep_comments]
  [ -f "$1" ] || return 0
  awk -v h="$2" -v keep="${3:-0}" '
    { nc=$0; sub(/#.*/,"",nc) }
    nc ~ "^  "h"[[:space:]]*:[[:space:]]*$" { inb=1; next }
    inb && nc ~ /^  [A-Za-z0-9._-]+[[:space:]]*:/ { inb=0 }
    inb && nc ~ /^    [A-Za-z0-9._-]+[[:space:]]*:/ { print (keep == "1" ? $0 : nc) }
  ' "$1"
}
```

```bash
_hd_entry_line(){ # $1=file $2=harness $3=agent [$4=keep_comments]
  local block line
  block="$(_hd_block "$1" "$2" "${4:-0}")"
  [ -n "$block" ] || return 0
  line="$(grep -E "^    $3[[:space:]]*:" <<<"$block" || true)"
  [ -n "$line" ] || return 0
  printf '%s' "${line%%$'\n'*}"                # first match only; no `| head` under pipefail
}
```

Add the predicate immediately after `_hd_entry_line`:

```bash
# Does <raw entry line> carry a `#` INSIDE its `{…}` flow map? Returns 0 (fires) when it does.
#
# Comments are stripped before either field reader sees a line, so a `#` inside the flow map
# truncates the entry silently — `{ model: c#5 }` becomes `{ model: c` and every value-comparison
# leg agrees the value is fine. Such a `#` is OUT OF CONTRACT; the strip order is deliberately
# unchanged, and this predicate makes the corner a loud refusal instead.
#
# The rule, exactly: it APPLIES only when the entry's first `{` precedes any `#` on the line (a `#`
# before the first `{`, or no `{` at all, never fires). It then FIRES iff, after that `{`, a `#`
# appears before the first `}`, or a `#` appears with no `}` at all (a commented-away closing brace
# is the same truncation). So a trailing comment after `}`, a full-line comment, and a commented-out
# map all stay legal.
#
# Twin: `flow_map_has_comment` in sync-agents.sh — same body, different name. Duplicated by value on
# purpose: this library's header forbids coupling the shipped-data reader to the user-config
# readers, and extracting the shared helper is change #0256's scope.
_hd_flow_map_has_comment(){ # $1=raw entry line
  local l="$1" after
  case "$l" in *'{'*) : ;; *) return 1 ;; esac
  case "${l%%\{*}" in *'#'*) return 1 ;; esac          # a `#` before the first `{` never fires
  after="${l#*\{}"
  case "$after" in *'#'*) : ;; *) return 1 ;; esac     # no `#` after the `{` at all
  case "$after" in *'}'*) : ;; *) return 0 ;; esac     # `#` present, no `}` at all -> truncation
  case "${after%%\}*}" in *'#'*) return 0 ;; esac      # `#` before the first `}` -> truncation
  return 1
}
```

- [ ] **Step 4: Wire the leg into `hd_validate`**

Add `rawline` to `hd_validate`'s `local` declaration list, then insert the check inside the per-entry `while IFS= read -r line` loop — after `a` is computed and the wrapper-source check runs, before the `fields` extraction:

```bash
      rawline="$(_hd_entry_line "$f" "$h" "$a" 1)"
      if _hd_flow_map_has_comment "$rawline"; then
        echo "harness-defaults: $h/$a entry contains '#' inside the flow map — comments cannot appear inside {…}; docket strips them before parsing" >&2; rc=1
      fi
```

Wrapper emission continues to read the stripped line; the corner is rejected at validation, never silently emitted.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `bash tests/test_harness_defaults_flow_map.sh`
Expected: `PASS`, exit 0.

- [ ] **Step 6: Mutation-test the new asserts**

Run each mutation, confirming with `/usr/bin/grep -cF` that it landed, that the suite reddens, and that restoring makes it green again. Back up with `cp`, never `git checkout --` (that restores HEAD and would destroy Task 1's committed-but-then-modified state and any uncommitted work).

1. Delete the `if _hd_flow_map_has_comment` block from `hd_validate` → the fire probes must redden.
2. Change `_hd_flow_map_has_comment`'s first-`{`-precedes-`#` guard (`case "${l%%\{*}" in *'#'*) return 1 ;; esac`) to `return 1` unconditionally → nothing fires; the fire probes redden.
3. Delete that same guard line entirely → the commented-out-map accept-probe must redden (it now over-fires). This is the assert that proves the carve-out is real rather than incidental.
4. Change `_hd_block`'s `print (keep == "1" ? $0 : nc)` back to `print nc` → the `keep_comments` reader assert reddens.

If any mutation leaves the suite green, that assert is decoration — fix the assert before proceeding.

- [ ] **Step 7: Run the whole suite**

Run: `scripts/run-tests.sh`
Expected: all green. Pay attention to `tests/test_harness_defaults.sh` and `tests/test_sync_agents_defaults.sh` — they consume `_hd_block`/`_hd_entry_line` through their default (no-flag) path, which must be byte-identical to today's behavior.

Check the shard's measured cost against the row Task 1 sized:

```bash
time bash tests/test_harness_defaults_flow_map.sh
```

If it still fits the row, change nothing — that is the intended outcome of Task 1 sizing with headroom. If it does not fit, adjust **that one new row** and `EXPECTED_TOTAL` together, and update Task 1's `EXPECTED_TOTAL` comment so it states the final size. This is still the single new-test-file case being sized before it ever merges, not a raise of an existing ceiling — but it must stay under the 60s hard ceiling. If it cannot, stop and report rather than reaching for a serial pin.

- [ ] **Step 8: Commit**

```bash
git add scripts/lib/harness-defaults.sh tests/test_harness_defaults_flow_map.sh
git commit -m "fix(0255): reject a '#' inside an agents: flow map in hd_validate"
```

(Add `tests/runtime-budgets.tsv tests/test_runtime_budgets.sh` to the `git add` only if Step 7 required resizing the row.)

---

### Task 3: The `#`-inside-the-flow-map leg in `sync-agents.sh`

The same corner in the user-config validator, where it matters most: `install.sh` runs `sync-agents.sh`, and the global `~/.config/docket/config.yml` layer is read for every repo on the machine.

**Files:**
- Modify: `sync-agents.sh` — `harness_agent_line` (both bash paths), new `flow_map_has_comment`, `validate_user_agent_values`
- Test: `tests/test_sync_agents_validator.sh`

**Interfaces:**
- Consumes: `_hd_flow_map_has_comment`'s body from Task 2 (copied by value, renamed); `harness_agent_line(file, harness, agent, under_agents)`; `section_body`; `_LAYER_BODY_CACHE`
- Produces: `harness_agent_line "$file" "$harness" "$agent" "$under_agents" [keep_comments]` — a 5th positional arg; `1` returns the entry line WITHOUT the comment strip. `flow_map_has_comment "$raw_entry_line"` — identical semantics to `_hd_flow_map_has_comment`.

- [ ] **Step 1: Write the failing tests**

Add to `tests/test_sync_agents_validator.sh`, immediately after the single-quoted-value block (the `HROOT173S` fixture) and before the `a genuinely MISSING value` block. Note this file's local idiom: `/usr/bin/grep`, herestrings, `make_sandbox` / `rm -rf "$SBX"` per fixture.

```bash
# -- 0255: a `#` INSIDE the flow map. harness_agent_line strips comments on BOTH bash paths before
#    either reader runs, so `{ model: c#5 }` truncates to `c` and every value-comparison leg agrees
#    it is fine — the silent truncation this validator family exists to close, in the one corner the
#    value legs structurally cannot see. Generation must abort before any wrapper is written. --
make_sandbox
HROOT255H="$(mktemp -d)"; mkdir -p "$HROOT255H/.claude"
printf 'agents:\n  default:\n    status: { model: c#5, effort: high }\n' > "$SBX/.docket.yml"
h_err="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT255H" bash "$SYNC" 2>&1 >/dev/null )"; h_rc=$?
assert "0255 validator: '#' inside the flow map exits non-zero" '[ "$h_rc" != "0" ]'
assert "0255 validator: '#' diagnostic names the harness/agent" '/usr/bin/grep -qF "default/status" <<<"$h_err"'
assert "0255 validator: '#' diagnostic names the flow map" '/usr/bin/grep -qF "inside the flow map" <<<"$h_err"'
assert "0255 validator: '#' diagnostic names the layer file" '/usr/bin/grep -qF ".docket.yml" <<<"$h_err"'
# A diagnostic that blames the wrong cause is the defect the split messages here exist to prevent:
# "write it unquoted" is not the remedy for a truncating `#`.
assert "0255 validator: '#' diagnostic does not blame quoting" \
  '! /usr/bin/grep -qF "unquoted and space-free" <<<"$h_err"'
assert "0255 validator: '#' value writes no wrapper" '[ ! -e "$SBX/.claude/agents/docket-status.md" ]'
rm -rf "$SBX" "$HROOT255H"

# -- the carve-outs. Over-rejecting either of these would hard-abort generation on the documented,
#    legitimate comment styles used throughout .docket.example.yml and agent-layer.md's own example. --
make_sandbox
HROOT255T="$(mktemp -d)"; mkdir -p "$HROOT255T/.claude"
printf 'agents:\n  default:\n    status: { model: claude-opus-5, effort: high }   # trailing note\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT255T" bash "$SYNC" >/dev/null 2>&1 ); t_rc=$?
assert "0255 validator: a trailing comment AFTER the brace still generates (rc=0)" '[ "$t_rc" = "0" ]'
assert "0255 validator: and the trailing-comment wrapper IS written" \
  '[ "$(fm_anchored "$SBX/.claude/agents/docket-status.md" model)" = "claude-opus-5" ]'
rm -rf "$SBX" "$HROOT255T"

# A commented-out map is field-absent post-strip, which is LEGAL in user config (every field is
# optional) — and it is the natural workaround for this very gate, so it must not fire.
make_sandbox
HROOT255C="$(mktemp -d)"; mkdir -p "$HROOT255C/.claude"
printf 'agents:\n  default:\n    status: # { model: c#5, effort: high }\n    adr: { model: claude-opus-5, effort: low }\n' > "$SBX/.docket.yml"
c_err="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT255C" bash "$SYNC" 2>&1 >/dev/null )"; c_rc=$?
assert "0255 validator: a commented-out map still generates (rc=0)" '[ "$c_rc" = "0" ]'
assert "0255 validator: and it fires no flow-map complaint" \
  '! /usr/bin/grep -qF "inside the flow map" <<<"$c_err"'
assert "0255 validator: and the sibling entry still resolves" \
  '[ "$(fm_anchored "$SBX/.claude/agents/docket-adr.md" model)" = "claude-opus-5" ]'
rm -rf "$SBX" "$HROOT255C"
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `bash tests/test_sync_agents_validator.sh`
Expected: FAIL — the six `'#' inside the flow map` asserts fail (generation currently succeeds with a truncated `c` pin). The three carve-out asserts already pass; they are the over-rejection floor for Step 4.

- [ ] **Step 3: Add the `keep_comments` arg to `harness_agent_line`**

Both bash paths change together. Replace the bash-3.2 path's `sed`/`grep`/`head` chain with a single awk pass that matches on the stripped view and prints whichever view the caller asked for — the same technique Task 2 used, and the reason the two modes can never select different lines:

```bash
# Print the `agents.<harness>.<agent>` entry line from <file>. under_agents=1 => the harness map is
# nested under a top-level `agents:` key (.docket.yml); 0 => the harness map is the whole file (global).
# keep_comments=1 returns the line WITHOUT the comment strip, for validate_user_agent_values' check
# that no `#` sits inside the flow map; every other caller wants the stripped default. Both bash
# paths match on the STRIPPED view in both modes, so they cannot select different lines.
harness_agent_line() {  # $1=file $2=harness $3=agent $4=under_agents(0|1) [$5=keep_comments]
  local key body line stripped sub hbody matched keep="${5:-0}"
  if [ "${BASH_VERSINFO[0]}" -lt 4 ]; then
    [ -f "$1" ] || return 0
    if [ "$4" = "1" ]; then sub="$(section_body agents < "$1")"; else sub="$(cat "$1")"; fi
    hbody="$(printf '%s\n' "$sub" | section_body "$2" || true)"
    matched="$(awk -v a="$3" -v keep="$keep" '
      { nc=$0; sub(/#.*/,"",nc) }
      nc ~ ("^[[:space:]]*" a "[[:space:]]*:") { print (keep == "1" ? $0 : nc); exit }
    ' <<<"$hbody")"
    printf '%s\n' "$matched"
    return 0
  fi
  key="${1}"$'\x1f'"${2}"$'\x1f'"${4}"
  body="${_LAYER_BODY_CACHE[$key]-}"
  while IFS= read -r line || [ -n "$line" ]; do
    stripped="${line%%#*}"
    if [[ $stripped =~ ^[[:space:]]*${3}[[:space:]]*: ]]; then
      if [ "$keep" = "1" ]; then printf '%s' "$line"; else printf '%s' "$stripped"; fi
      return 0
    fi
  done <<<"$body"
}
```

The bash-4 path works unchanged because `section_body` prints values raw — comment-stripping is documented there as the caller's job — so the layer-body cache already retains comments.

Note the trailing-newline difference between the two paths (`printf '%s\n'` vs `printf '%s'`) is pre-existing and harmless: every caller reads through `$( … )`, which strips trailing newlines. Do not "fix" it in this change.

- [ ] **Step 4: Add the predicate and wire the leg in**

Add `flow_map_has_comment` next to `harness_agent_line` — body identical to Task 2's `_hd_flow_map_has_comment`, with a comment naming the twin:

```bash
# Does <raw entry line> carry a `#` INSIDE its `{…}` flow map? Returns 0 (fires) when it does.
#
# Comments are stripped before either field reader sees a line, so a `#` inside the flow map
# truncates the entry silently — `{ model: c#5 }` becomes `{ model: c` and every value-comparison
# leg agrees the value is fine. Such a `#` is OUT OF CONTRACT; the strip order is deliberately
# unchanged (reordering it would break the legitimate trailing and full-line comments used across
# every layer), and this predicate makes the corner a loud refusal instead.
#
# The rule, exactly: it APPLIES only when the entry's first `{` precedes any `#` on the line (a `#`
# before the first `{`, or no `{` at all, never fires). It then FIRES iff, after that `{`, a `#`
# appears before the first `}`, or a `#` appears with no `}` at all. So a trailing comment after
# `}`, a full-line comment, and a commented-out map all stay legal.
#
# Twin: `_hd_flow_map_has_comment` in scripts/lib/harness-defaults.sh — same body, different name.
# Duplicated by value on purpose: that library's header forbids coupling the shipped-data reader to
# these user-config readers, and extracting the shared helper is change #0256's scope.
flow_map_has_comment() {  # $1=raw entry line
  local l="$1" after
  case "$l" in *'{'*) : ;; *) return 1 ;; esac
  case "${l%%\{*}" in *'#'*) return 1 ;; esac
  after="${l#*\{}"
  case "$after" in *'#'*) : ;; *) return 1 ;; esac
  case "$after" in *'}'*) : ;; *) return 0 ;; esac
  case "${after%%\}*}" in *'#'*) return 0 ;; esac
  return 1
}
```

Then in `validate_user_agent_values`, add `rawline` to the `local` list and insert the check immediately after the existing `line="$(harness_agent_line "$f" "$h" "$a" 1)"` / `[ -n "$line" ] || continue` pair, before the `for k in model effort runner` loop:

```bash
        rawline="$(harness_agent_line "$f" "$h" "$a" 1 1)"
        if flow_map_has_comment "$rawline"; then
          log "$h/$a entry contains '#' inside the flow map — comments cannot appear inside {…}; docket strips them before parsing ($f)"
          rc=1
        fi
```

The check sits **inside** the existing dead-config carve-outs (the harness skip and the `[ -f "$AGENTS_SRC/docket-$a.md" ] || continue`), so a `#` in config that generates nothing still cannot hard-fail a repo — the same reasoning that already exempts the pre-0046 flat shape.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `bash tests/test_sync_agents_validator.sh`
Expected: `PASS`, exit 0.

- [ ] **Step 6: Mutation-test the new asserts**

Back up with `cp`, mutate, `grep -c` to prove the mutation landed, run, restore, re-run:

1. Delete the `if flow_map_has_comment` block from `validate_user_agent_values` → the six fire asserts redden.
2. Delete `flow_map_has_comment`'s `case "${l%%\{*}" in *'#'*) return 1 ;; esac` guard → the commented-out-map carve-out asserts redden.
3. Revert `harness_agent_line`'s bash-4 branch to always `printf '%s' "$stripped"` → the fire asserts redden (the validator sees a stripped line and finds no `#`).
4. Revert the bash-3.2 awk to `print nc` unconditionally, then force that path by running the file under a bash 3.2 binary if one is available (`/bin/bash` on macOS); if none is, state so explicitly in the results file rather than claiming the path was exercised. An unexercised path is an unverified path.

- [ ] **Step 7: Run the whole suite**

Run: `scripts/run-tests.sh`
Expected: all green. `harness_agent_line` is on the hot resolution path for every layer and every agent, so watch `tests/test_sync_agents.sh`, `tests/test_sync_agents_defaults.sh`, `tests/test_sync_agents_cursor.sh`, `tests/test_sync_agents_codex.sh`, `tests/test_sync_agents_opencode.sh`, and `tests/test_sync_agents_runners.sh` in particular — including the tab-indented `.docket.local.yml` fixture in the over-rejection floor, which is what the `[^[:space:]]`-class rule protects.

- [ ] **Step 8: Commit**

```bash
git add sync-agents.sh tests/test_sync_agents_validator.sh
git commit -m "fix(0255): reject a '#' inside an agents: flow map in the user-config validator"
```

---

### Task 4: The rule, at the five points of use

The finding behind the docs half is that the rule appears **nowhere a user looks before tripping the gate** — `grep -rn unquoted README.md skills/docket-convention/ .docket.example.yml` returns zero hits today. One identical sentence, reusing the diagnostic's own wording so the docs and the error message stay mutually recognizable.

**Files:**
- Modify: `README.md` (two `agents:` examples)
- Modify: `skills/docket-convention/SKILL.md` (the `.docket.yml` schema block's `agents:` line)
- Modify: `skills/docket-convention/references/agent-layer.md` (the example `agents:` block)
- Modify: `.docket.example.yml` (the `agents:` intro comment)
- Test: `tests/test_sync_agents_drift_docs.sh`

**Interfaces:**
- Consumes: the diagnostics authored in Tasks 1–3
- Produces: nothing consumed by later tasks

**The sentence, byte-identical at all five sites** (rendered as a comment in each file's own comment syntax):

> Write model/effort values unquoted and space-free; `#` cannot appear inside the `{…}` flow map.

- [ ] **Step 1: Write the failing test**

Add to the doc-sentinel section of `tests/test_sync_agents_drift_docs.sh` (this file already owns the README/doc sentinels for `sync-agents.sh`). Anchor each assert to the **block** the rule must appear in, not to a whole-file grep — a file-wide match for the phrase would stay green if the sentence drifted into unrelated prose.

```bash
# ---- 0255: the unquoted / no-`#` rule is stated at the five points of use --------------------
# The finding this guards: before change 0255 the rule appeared NOWHERE a user reads before
# tripping the gate — the gate self-described only once tripped. Each assert is scoped to the
# `agents:` example block in its file, so the sentence drifting into unrelated prose reddens.
DOCS_REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
rule_re='unquoted and space-free'

readme_global="$(sed -n '/^# ~\/.config\/docket\/config.yml/,/^```$/p' "$DOCS_REPO/README.md")"
assert "0255 docs: README global config.yml example states the rule" \
  '/usr/bin/grep -qF "$rule_re" <<<"$readme_global"'

readme_local="$(sed -n '/^# <repo>\/.docket.local.yml/,/^```$/p' "$DOCS_REPO/README.md")"
assert "0255 docs: README .docket.local.yml example states the rule" \
  '/usr/bin/grep -qF "$rule_re" <<<"$readme_local"'

skill_agents_line="$(/usr/bin/grep -n '^agents:' "$DOCS_REPO/skills/docket-convention/SKILL.md" || true)"
assert "0255 docs: convention SKILL.md agents: schema line states the rule" \
  '/usr/bin/grep -qF "$rule_re" <<<"$skill_agents_line"'

layer_example="$(sed -n '/^agents:  */,/^```$/p' "$DOCS_REPO/skills/docket-convention/references/agent-layer.md")"
assert "0255 docs: agent-layer.md example block states the rule" \
  '/usr/bin/grep -qF "$rule_re" <<<"$layer_example"'

example_intro="$(sed -n '/^# agents — per-skill subagent model\/effort/,/^# agents:$/p' "$DOCS_REPO/.docket.example.yml")"
assert "0255 docs: .docket.example.yml agents: intro states the rule" \
  '/usr/bin/grep -qF "$rule_re" <<<"$example_intro"'

# Non-vacuity: every slice above must be non-empty, or a renamed heading turns all five asserts
# into vacuous greens against nothing.
assert "0255 docs: every doc slice is non-empty" \
  '[ -n "$readme_global" ] && [ -n "$readme_local" ] && [ -n "$skill_agents_line" ] &&
   [ -n "$layer_example" ] && [ -n "$example_intro" ]'
```

Before writing the asserts, run each `sed`/`grep` extraction by hand and confirm it returns the block you intended. A slice regex that matches nothing produces a guard that can only ever fail — or, inverted, one that can only ever pass.

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_sync_agents_drift_docs.sh`
Expected: FAIL on the five rule asserts, PASS on the non-vacuity assert (which proves the slices resolve).

- [ ] **Step 3: Add the sentence at all five sites**

`README.md` — the `~/.config/docket/config.yml` fenced example, on the `agents:` line's comment:

```yaml
agents:                      # agent model/effort defaults (same agents: shape as .docket.yml)
                             # Write model/effort values unquoted and space-free; `#` cannot
                             # appear inside the `{…}` flow map.
  default:
    implement-next: { model: claude-opus-5, effort: medium }
```

`README.md` — the `<repo>/.docket.local.yml` fenced example, same shape:

```yaml
agents:                       # Write model/effort values unquoted and space-free; `#` cannot
                              # appear inside the `{…}` flow map.
  default:
    implement-next: { model: claude-opus-5, effort: medium }
```

`skills/docket-convention/SKILL.md` — the `.docket.yml` schema block, keeping the `agents:` entry to one line:

```yaml
agents:                      # harness-first per-skill subagent model/effort — write values unquoted and space-free, no `#` inside the `{…}` flow map; see "Agent layer" below
```

`skills/docket-convention/references/agent-layer.md` — inside the example `agents:` block, alongside the sibling rule comments already there:

```yaml
  # Write model/effort values unquoted and space-free; `#` cannot appear inside the `{…}` flow map
  # — docket strips comments before parsing, so an in-map `#` truncates the value. Both validators
  # refuse it rather than shipping a clipped pin.
```

`.docket.example.yml` — inside the `agents:` intro comment paragraph.

**Placement constraint, load-bearing:** `tests/test_docket_example_yml.sh` discovers this file's intentionally-commented keys by taking the line **immediately following** a `# scope: …` tag. The intro currently ends:

```
# PRESENCE-SENSITIVE: uncommenting this key changes behavior even at these default values.
# scope: any layer (.docket.yml, .docket.local.yml, or global config.yml)
# agents:
```

Insert the new line **earlier in the paragraph** — a good spot is right after the sentence about `effort: auto` — and never between the `# scope:` tag and `# agents:`. Inserting there would make the new line the discovered "key" and break the commented-key extraction. Also keep the new line out of the `/^# agents:$/,/^runners:$/` region, which has its own doubly-commented-block guard.

```
# Write model/effort values unquoted and space-free; `#` cannot appear inside the `{…}` flow map —
# docket strips comments before parsing, so an in-map `#` truncates the value and generation is
# refused rather than shipping a clipped pin.
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `bash tests/test_sync_agents_drift_docs.sh`
Expected: `PASS`, exit 0.

- [ ] **Step 5: Verify the finding's own grep now answers**

```bash
cd /Users/homer/dev/docket/.worktrees/complete-adr-0065-s-quote-leg-and-document-the-unquoted-rule
/usr/bin/grep -rn "unquoted and space-free" README.md skills/docket-convention/ .docket.example.yml
```

Expected: five hits — two in `README.md`, one in `skills/docket-convention/SKILL.md`, one in `skills/docket-convention/references/agent-layer.md`, one in `.docket.example.yml`. This is the literal command from the change's `## Why` that returned zero hits before.

- [ ] **Step 6: Mutation-test the doc asserts**

Delete the sentence from one site at a time (five runs), confirming with `grep -c` that the deletion landed and that `tests/test_sync_agents_drift_docs.sh` reddens for that site specifically. A single whole-file grep would stay green after four of the five deletions — that is the failure this per-site scoping exists to prevent.

- [ ] **Step 7: Run the whole suite**

Run: `scripts/run-tests.sh`
Expected: all green. `tests/test_docket_example_yml.sh` is the one to watch — it enforces the sidecar-to-example mirror equality, the scope-tag key discovery, and the commented-block guards described in Step 3.

- [ ] **Step 8: Commit**

```bash
git add README.md skills/docket-convention/SKILL.md skills/docket-convention/references/agent-layer.md .docket.example.yml tests/test_sync_agents_drift_docs.sh
git commit -m "docs(0255): state the unquoted / no-'#' rule at its five points of use"
```

---

## Self-review notes

**Spec coverage.** §1 quote leg → Task 1. §2 `#` corner, both validators, with the exact firing rule and its carve-outs → Tasks 2 and 3. §3 documentation at five points of use → Task 4. §4 tests → the test steps inside each of the four tasks. Assumption 9 ("recorded in spec, docs, and code comments; no ADR update") → the block comments in Tasks 2 and 3 plus Task 4's sentence; no ADR task is included, per that assumption's explicit decision. Assumption 7 (existing diagnostics stay byte-identical) → stated in Global Constraints and enforced by the untouched 0173 asserts.

**Deliberate non-goals, restated so no task drifts into them.** No shared helper extracted between the two files (deferred to #0256). No change to the strip order. No widening of what values are legal. No vendor model allowlist.

**Plan correction, 2026-08-08 (recorded rather than silently rewritten).** This plan's first draft told Tasks 1 and 2 to add their `hd_validate` probes to `tests/test_harness_defaults_validator.sh` and to *raise that file's budget row*. The Task 1 worker returned `BLOCKED` against it, correctly: `tests/test_runtime_budgets.sh` pins the table's TOTAL precisely so a per-row raise reddens, and its remedy text refuses the raise by name, directing instead to "shard the file, or move its new assertions into a shard with room." The worker also measured the file at **49.5s against its 50s row** — its margin was already gone before this change added anything. The plan now routes every new probe into a new shard, `tests/test_harness_defaults_flow_map.sh`, which brings its own row: the guard's own first sanctioned case, with repeated precedent in `EXPECTED_TOTAL`'s comment history. The same worker also caught that the original mutation landing check used PATH `grep` (ugrep) with an unescaped `$`, which reports `0` on an unmutated file and would have made every mutation test a false green; the Global Constraints now require `/usr/bin/grep -cF` there.

**Known risk to surface at review.** The new shard is sized once, in Task 1, with headroom for the probes Task 2 appends, so the pinned total moves exactly once inside this change. If Task 2's measurement overruns that row, Task 2 Step 7 resizes the same new row rather than touching any existing one — and if it cannot fit under the 60s hard ceiling, the plan directs a stop-and-report instead of a serial pin.
