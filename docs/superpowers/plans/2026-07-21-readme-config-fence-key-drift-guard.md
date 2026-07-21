# README Config Fence Key Drift Guard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add section `(9) README CONFIG FENCE KEY CORRESPONDENCE` to `tests/test_docket_example_yml.sh`, guarding every `yaml` fence in `README.md` against key drift from `.docket.example.yml` — with a derived (never enumerated) fence set, existence-only asserts by default, and opt-in value equality.

**Architecture:** One new section appended to an existing bash test file, built from five small shell/awk helpers (`fence_openers`, `fence_body`, `fence_marker`, `is_pseudo_key`, `scan_fences`). `scan_fences` emits one *finding* line per problem and the section asserts each finding-kind is empty, so failures name the fence line and the offending key. Helpers take their markdown path as an **argument** so marker-grammar tests can scan a temporary fixture instead of mutating the real README. A blocking prerequisite widens `flatten_yaml`'s key class to admit hyphens.

**Tech Stack:** bash (`set -uo pipefail`), POSIX awk, BSD-compatible grep/sed. No new dependencies.

## Global Constraints

- **Sections `(1)`–`(8)` are byte-untouched** except the two-line `flatten_yaml` widening in Task 1. Do not refactor `(8)` into `(9)`.
- **`.docket.example.yml` content is not modified** by any task.
- **The fence count literal is `9`** and the discovery regex is whitespace-tolerant (`^[[:space:]]*```yaml[[:space:]]*$`). These are a **matched pair** — a column-0 regex plus a literal `8` is a green suite that permanently excludes fence 576. Never "fix" a count mismatch by lowering the literal to match a broken regex.
- **Portability:** no `\t` literals inside `grep -E` (BSD grep does not interpret them) — use a `TAB9="$(printf '\t')"` variable. No awk interval expressions (`{n,m}`); use `substr()`.
- **Shell:** the file is `#!/usr/bin/env bash`. Run every verification with explicit `bash`, never the agent's interactive shell — a `for f in $VAR` loop iterates zero times under zsh and still prints success (`agent-shell-noop-reads-as-success`).
- **Assert idiom:** `assert "<description with observed value>" '<test expression>'`. The description must carry the *observed* value so CI output is diagnosable.
- **Every task ends with the full suite green:** `bash tests/test_docket_example_yml.sh` → exit 0. **Baseline before any change: 138 `ok -`, 0 `NOT OK`.**
- **Verified ground truth** (re-confirmed against `main` at `5ed3d8c` during reconcile): `README.md` carries exactly **9** yaml fences at lines **209, 234, 264, 289, 310, 407, 433, 576, 594**; fence **576 is indented two spaces**. `.docket.example.yml` flattens to **30** paths and contains **no** hyphenated active key.

---

## File Structure

| File | Responsibility | Change |
|---|---|---|
| `tests/test_docket_example_yml.sh` | The whole guard. `flatten_yaml` widening (Task 1); new section `(9)` appended after `(8)`, immediately before the final `exit $fail` (Tasks 2–6). | Modify |
| `README.md` | Carries the `<!-- docket:config-fence: values -->` marker on the `reclaim:` fence. | Modify (Task 6, one line) |
| `docs/superpowers/plans/2026-07-21-readme-config-fence-key-drift-guard.md` | This plan. | Created |

**Placement:** section `(9)` goes at the **end** of `tests/test_docket_example_yml.sh`, after `(8)`'s last assert (the canonical-reference link target check) and **before** the final `exit $fail` line.

**Reused from earlier in the file:** `assert`, `REPO`, `EX`, `README` (set by `(7)`), `tmp` (the `mktemp -d` scratch dir with its `trap`), `flatten_yaml`, and `ex_flat` (set by `(8)` as `flatten_yaml < "$EX"`). Reusing `ex_flat` is deliberate — if `(8)` is ever moved above `(9)`, `ex_flat` goes empty and **every** fence key fails loudly rather than silently passing.

---

### Task 1: Widen `flatten_yaml`'s key class to admit hyphens

**Why this is first and blocking:** `flatten_yaml`'s key regex is `[A-Za-z_][A-Za-z0-9_]*` — no hyphen — so it silently drops `implement-next:` from README fences 289 and 310. Left alone, Task 5's floor 3 ships **RED on correct README prose** (raw=11 vs flat=10 on both fences).

**The trap this task exists to avoid:** `flatten_yaml` carries the key class **twice**. Widening only the first is a half-fix that **no floor in section (9) can catch** — verified: half-widened and fully-widened both yield 3 paths on the hyphenated fixture and `flat=11` on fences 289/310, so every count-based assert passes either way. The difference is only visible in the extracted **value**: half-widened, `agents.default.implement-next` comes back carrying the **entire raw line**. That is why Step 1 asserts the value, not the path count.

**Files:**
- Modify: `tests/test_docket_example_yml.sh` (`flatten_yaml` body — shape test and value strip; `:782` and `:785` as of `5ed3d8c`, but **locate them by their code**, not by line number)
- Test: same file (new asserts appended at the end, before `exit $fail`)

**Interfaces:**
- Produces: `flatten_yaml` accepting keys matching `[A-Za-z_][A-Za-z0-9_-]*`, emitting `path<TAB>value` with the value correctly stripped for hyphenated keys. Every later task consumes this.

- [x] **Step 1: Write the failing test**

Append to the very end of `tests/test_docket_example_yml.sh`, **immediately before** the final `exit $fail` line:

```bash
# --- (9) README CONFIG FENCE KEY CORRESPONDENCE -------------------------------
TAB9="$(printf '\t')"

# PREREQUISITE GUARD (change 0108, Task 1). flatten_yaml's key class must admit HYPHENS, because
# README fences 289/310 carry `implement-next:` under agents.default. The class appears TWICE in
# flatten_yaml — the shape test and the value strip — and widening only the shape test is a
# half-fix that NOTHING ELSE IN THIS FILE CATCHES: the path count is identical either way
# (3 paths on the fixture below, 11 on fences 289/310), so every count-based floor passes. The
# half-fix is visible ONLY in the extracted VALUE, which comes back as the whole raw line. That
# is why this asserts the value and not just the path.
hyph_fix="$(printf 'agents:\n  default:\n    implement-next: { model: x, effort: y }\n')"
hyph_out="$(printf '%s\n' "$hyph_fix" | flatten_yaml)"
hyph_paths="$(printf '%s\n' "$hyph_out" | grep -c .)"
hyph_val="$(printf '%s\n' "$hyph_out" | awk -F"$TAB9" '$1=="agents.default.implement-next"{print $2}')"
assert "(9) flatten_yaml keeps a HYPHENATED key as its own path (got $hyph_paths paths, want 3)" \
  '[ "$hyph_paths" = "3" ]'
assert "(9) flatten_yaml STRIPS a hyphenated key from its value — half-fix guard; widening only the shape test leaves the whole raw line here and no count-based floor can see it (got [$hyph_val])" \
  '[ "$hyph_val" = "{ model: x, effort: y }" ]'
```

- [x] **Step 2: Run test to verify it fails**

Run: `bash tests/test_docket_example_yml.sh 2>&1 | grep -E '^NOT OK'`

Expected: **2** failures — the path assert reports `got 2 paths, want 3` and the value assert reports `got []`. (Un-widened, `implement-next:` fails the shape test entirely, so the path never exists.)

- [x] **Step 3: Widen both occurrences**

In `flatten_yaml`, change the **shape test** from:

```awk
      if (line !~ /^[[:space:]]*[A-Za-z_][A-Za-z0-9_]*:/) next
```

to:

```awk
      if (line !~ /^[[:space:]]*[A-Za-z_][A-Za-z0-9_-]*:/) next
```

and the **value strip** from:

```awk
      val = line; sub(/^[[:space:]]*[A-Za-z_][A-Za-z0-9_]*:[[:space:]]*/, "", val)
```

to:

```awk
      val = line; sub(/^[[:space:]]*[A-Za-z_][A-Za-z0-9_-]*:[[:space:]]*/, "", val)
```

Then update `(8)`'s standing comment about the narrow key regex so it no longer contradicts the code. Change the comment text `([A-Za-z_][A-Za-z0-9_]*:)` in the "SAFETY NET for the flattener's deliberately narrow key regex" block to `([A-Za-z_][A-Za-z0-9_-]*:)`. **This is a comment-only edit inside `(8)`** — the sole permitted touch to that section, because leaving it stale would document a hazard the code no longer has.

- [x] **Step 4: Run tests to verify they pass and the widening is behavior-neutral**

Run: `bash tests/test_docket_example_yml.sh; echo "EXIT=$?"`

Expected: `EXIT=0`, **140** `ok -` lines (138 baseline + 2 new), **0** `NOT OK`.

Then prove behavior-neutrality for `(1)`–`(8)` explicitly:

```bash
bash tests/test_docket_example_yml.sh | grep -E '^ok - \(8\)' 
```

Expected: `(8)` reports `snippet flattened key count is exactly 5` and `raw=5 flattened=5` — unchanged from baseline. The example still flattens to 30 paths (no hyphenated active key exists in it), so `(8)`'s `>= 20` floor and exact-5 count are untouched.

- [x] **Step 5: Verify the mutation is load-bearing (half-fix must redden)**

Temporarily revert **only the value strip** to the hyphen-free form, leaving the shape test widened, then run:

```bash
bash tests/test_docket_example_yml.sh 2>&1 | grep -E '^NOT OK'
```

Expected: exactly **one** failure — the value assert, reporting `got [    implement-next: { model: x, effort: y }]`. The path assert stays green, proving the value assert is the only guard for this defect. **Restore the widening** and re-run to green before committing.

- [x] **Step 6: Commit**

```bash
git add tests/test_docket_example_yml.sh
git commit -m "test(0108): widen flatten_yaml's key class to admit hyphens

flatten_yaml dropped `implement-next:` from README fences 289/310 because
its key class excluded hyphens. Widened at BOTH occurrences — the shape
test and the value strip — since widening only the shape test leaves the
entire raw line as the extracted value, which no count-based floor can
detect. Guarded by a value assert, not a path count.

Behavior-neutral for sections (1)-(8): the example carries no hyphenated
active key, so ex_flat stays at 30 paths."
```

---

### Task 2: Fence discovery + the exact-count floor

**Files:**
- Modify: `tests/test_docket_example_yml.sh` (section `(9)`)

**Interfaces:**
- Produces: `fence_openers <markdown-path>` → one `startline<TAB>indent` line per yaml fence; `fence_body <markdown-path> <startline> <indent>` → the fence's body with its base indent stripped. Both take the markdown path as an **argument** (Task 5's fixture depends on this).

- [x] **Step 1: Write the failing test**

Append to section `(9)`, after Task 1's asserts:

```bash
# FENCE DISCOVERY — DERIVED, NEVER ENUMERATED. The stub that proposed this change listed the
# unguarded fences by line number and its list was ALREADY WRONG on arrival (it omitted the
# reclaim: fence). A hand-written fence list is an enumerated floor that ages directly into the
# gap it was written to close, so the set is scanned out of the README instead: every yaml fence
# is in scope BY DEFAULT, and a new config fence is guarded the day it is written.
#
# The opener regex is WHITESPACE-TOLERANT and the closer is matched at the SAME indent, because
# fence 576 (skills: / brainstorm:) is a list-item continuation indented two spaces. A
# column-0-anchored regex structurally cannot see it — that is not hypothetical, it is the bug
# this change's own design draft shipped, which is why mutation-testing it is Step 5 below.
fence_openers(){
  awk '
    /^[[:space:]]*```yaml[[:space:]]*$/ && !inf { inf=1; ind=match($0,/[^[:space:]]/)-1; start=NR; next }
    inf && /^[[:space:]]*```[[:space:]]*$/ && match($0,/[^[:space:]]/)-1==ind { printf "%d\t%d\n", start, ind; inf=0 }
  ' "$1"
}

# Body of the fence opening at line $2 with indent $3, with that base indent stripped so nested
# keys keep their RELATIVE indent (flatten_yaml dots by indentation). substr() rather than an
# awk interval expression: {0,n} is not portable across awk implementations.
fence_body(){
  awk -v s="$2" -v ind="$3" '
    NR <= s { next }
    $0 ~ /^[[:space:]]*```[[:space:]]*$/ && match($0,/[^[:space:]]/)-1==ind { exit }
    { print substr($0, ind+1) }
  ' "$1"
}

# NON-VACUITY FLOOR 1 — the population itself. (9) iterates a DISCOVERED set, so its real failure
# mode is discovering ZERO fences and sailing through green. An EXACT count also catches the
# opposite direction (an undocumented fence added without keys in the example). The remedy is
# inline in the message so it survives into CI output.
fence_count="$(fence_openers "$README" | grep -c .)"
assert "(9) README yaml fence count is exactly 9 — floor against discovery going silently empty, ceiling against an unguarded new fence; if you ADDED a config fence, bump this literal AND ensure its keys are in .docket.example.yml in the same commit; if this dropped to 8, check that the fence regex is still whitespace-tolerant (fence 576 is indented) before touching the literal (got $fence_count)" \
  '[ "$fence_count" = "9" ]'
```

- [x] **Step 2: Run test to verify it passes**

Run: `bash tests/test_docket_example_yml.sh 2>&1 | grep -E 'fence count'`

Expected: `ok - (9) README yaml fence count is exactly 9 ... (got 9)`

- [x] **Step 3: Verify discovery finds the right fences**

Run:

```bash
bash -c 'cd "$(git rev-parse --show-toplevel)" && awk "
  /^[[:space:]]*\`\`\`yaml[[:space:]]*\$/ && !inf { inf=1; ind=match(\$0,/[^[:space:]]/)-1; start=NR; next }
  inf && /^[[:space:]]*\`\`\`[[:space:]]*\$/ && match(\$0,/[^[:space:]]/)-1==ind { printf \"%d\t%d\n\", start, ind; inf=0 }
" README.md'
```

Expected exactly:

```
209	0
234	0
264	0
289	0
310	0
407	0
433	0
576	2
594	0
```

Note fence **576 has indent 2** — that is the one a column-0 regex misses.

- [x] **Step 4: Mutation test — regress the regex to column-0**

Temporarily change the `fence_openers` opener pattern from `/^[[:space:]]*```yaml[[:space:]]*$/` to `/^```yaml[[:space:]]*$/`, then run:

```bash
bash tests/test_docket_example_yml.sh 2>&1 | grep -E 'fence count'
```

Expected: `NOT OK - (9) README yaml fence count is exactly 9 ... (got 8)`.

This is the design draft's own bug pinned as a test. **Restore the whitespace-tolerant regex** and re-run to green.

- [x] **Step 5: Commit**

```bash
git add tests/test_docket_example_yml.sh
git commit -m "test(0108): derive the README yaml fence set with an exact-count floor

Fence discovery is scanned out of the README, never enumerated: the stub's
hand-written fence list was already missing the reclaim: fence when filed.
Opener is whitespace-tolerant and the closer matches at the same indent so
the indented fence 576 (skills:/brainstorm:) is seen; a column-0 regex
finds only 8 and the count floor reddens on it."
```

---

### Task 3: The existence assert — query-by-key against the example

**Files:**
- Modify: `tests/test_docket_example_yml.sh` (section `(9)`)

**Interfaces:**
- Consumes: `fence_openers`, `fence_body` (Task 2); `flatten_yaml`, `ex_flat` (Task 1 / section `(8)`).
- Produces: `is_pseudo_key <key>` → exit 0 if the example carries that key as a *commented* pseudo-key; `scan_fences <markdown-path>` → one finding line per problem, `miss <fence-line> <path>` in this task. Later tasks extend `scan_fences` with more finding kinds.

- [x] **Step 1: Write the failing test**

Append to section `(9)`, after Task 2:

```bash
# ANCHOR: .docket.example.yml, ONE HOP. Sections (2a)/(2b)/(2c) already bind the example to the
# resolver in BOTH directions and prove it a faithful superset of everything the code reads, so
# going through the example inherits resolver coverage transitively and keeps a single anchor
# per artifact instead of two competing ones.
#
# ASSERT: EXISTENCE-ONLY by default. This is what makes one check applicable to all nine fences.
# Most of them deliberately show NON-DEFAULT values to illustrate opting in (auto_capture: true,
# terminal_publish: true, metadata_branch: main, and the two layered-config samples), so a
# value-equality assert would go spuriously RED against correct prose. Value equality is opt-in
# per fence — see the values marker in Task 6.
#
# DIRECTION (correspondence-guard-runs-one-way): this iterates the README's fence keys and proves
# fence ⊆ example. The reverse loop is deliberately ABSENT and is NOT an oversight — "every
# example key appears in the README" is the fourth all-keys surface change 0101 deleted. Do not
# add it.
#
# KEY RESOLUTION — QUERY-BY-KEY, not build-a-set. Two README fences use agents: and
# agent_harnesses: actively, but section (3) requires those keys to ship COMMENTED in the
# example, so a naive `path IN flatten_yaml(example)` reddens against correct prose. Resolution:
#   - top-level segment ACTIVE in the example  => the FULL dotted path must match;
#   - top-level segment only a COMMENTED pseudo-key => acceptance stops at the top-level segment,
#     because a commented key has no nested body to match against.
# Building a pseudo-key SET by regex is rejected: `^#[[:space:]]*[A-Za-z_]+:` also matches the
# example's prose comments (`# exceptions:` :22, `# scope: any layer` in many places, `# line:`
# :179), which would silently accept anything.
#
# is_pseudo_key matches the key LITERALLY via index()==1 rather than interpolating it into an
# ERE. That is strictly stronger than escaping it (escape-ere-metacharacters-in-key): there is no
# regex for a metacharacter to leak into at all.
#
# RESIDUAL HOLE, documented rather than closed: a future fence key whose NAME collides with a
# prose-comment word would be silently accepted (`scope:` would match `# scope: any layer`, and
# anchoring is no defense since that comment starts at column 0). No collision exists among
# today's 12 top-level fence keys. It is not closed because the only tight closure is an explicit
# two-key allowlist, which is exactly the enumerated floor this change exists to avoid.
is_pseudo_key(){
  awk -v k="$1" '
    { line=$0
      if (line !~ /^#/) next
      sub(/^#[[:space:]]*/, "", line)
      if (index(line, k ":") == 1) { found=1; exit } }
    END { exit(found?0:1) }
  ' "$EX"
}

ex9_paths="$(printf '%s\n' "$ex_flat" | cut -f1)"

# scan_fences <markdown-path> — emits one finding per line; EMPTY OUTPUT MEANS CLEAN.
# Takes the path as an argument (not the $README global) so the marker tests in Task 5 can scan a
# temporary fixture instead of mutating the real README.
scan_fences(){
  local md="$1" line ind body flatout p pv top
  while IFS="$TAB9" read -r line ind; do
    [ -n "$line" ] || continue
    body="$(fence_body "$md" "$line" "$ind")"
    flatout="$(printf '%s\n' "$body" | flatten_yaml)"
    while IFS="$TAB9" read -r p pv; do
      [ -n "$p" ] || continue
      top="${p%%.*}"
      if grep -Fxq "$top" <<<"$ex9_paths"; then
        grep -Fxq "$p" <<<"$ex9_paths" || echo "miss $line $p"
      elif is_pseudo_key "$top"; then :
      else echo "miss $line $p"; fi
    done <<FLAT
$flatout
FLAT
  done <<OPENERS
$(fence_openers "$md")
OPENERS
}

findings9="$(scan_fences "$README")"
f9_miss="$(printf '%s\n' "$findings9" | grep '^miss ' | sed 's/^miss //' | tr '\n' ' ')"
assert "(9) every README config-fence key exists in .docket.example.yml (fence-line + key path shown; ${f9_miss:-none missing})" \
  '[ -z "$f9_miss" ]'
```

- [x] **Step 2: Run test to verify it passes on correct prose**

Run: `bash tests/test_docket_example_yml.sh 2>&1 | grep -E 'every README config-fence key'`

Expected: `ok - (9) every README config-fence key exists in .docket.example.yml (... none missing)`

This is the load-bearing green: all 9 fences resolve — 24 distinct paths, 18 via active keys (full dotted path) and 6 via the `agents`/`agent_harnesses` pseudo-key path.

- [x] **Step 3: Mutation test — plant a phantom key**

Insert a bogus key into the `auto_capture` fence (line 264). Run:

```bash
bash -c 'cp README.md /tmp/README.bak && sed "265a\\
phantom_key: yes" /tmp/README.bak > README.md && bash tests/test_docket_example_yml.sh 2>&1 | grep -E "config-fence key"; cp /tmp/README.bak README.md'
```

Expected: `NOT OK - (9) every README config-fence key exists ... (264 phantom_key)`

Confirm `README.md` is restored: `git diff --stat README.md` must be empty.

- [x] **Step 4: Verify the pseudo-key branch is genuinely exercised**

The `elif is_pseudo_key` branch must not be dead code. Run:

```bash
bash -c 'awk "/^# agents:\$/{print \"agents pseudo-key present\"} /^# agent_harnesses:/{print \"agent_harnesses pseudo-key present\"}" .docket.example.yml'
```

Expected: both lines print. These are what `agents.default.implement-next`, `agents.claude.status` and `agent_harnesses` resolve through — without that branch the existence assert would report 6 spurious misses.

- [x] **Step 5: Commit**

```bash
git add tests/test_docket_example_yml.sh
git commit -m "test(0108): assert every README config-fence key exists in the example

Existence-only by default, so the check applies to all nine fences
including the ones deliberately showing non-default values. Keys resolve
query-by-key: full dotted path when the top-level key is active in the
example, top-level only when it is a commented pseudo-key (agents,
agent_harnesses), which section (3) requires to ship commented.

is_pseudo_key matches literally via index()==1 rather than building an
ERE, so no metacharacter can leak into the pattern."
```

---

### Task 4: Non-vacuity floors — per-fence non-empty, and raw-vs-flattened

**Files:**
- Modify: `tests/test_docket_example_yml.sh` (section `(9)`)

**Interfaces:**
- Consumes: `scan_fences` (Task 3).
- Produces: `empty <fence-line>` and `drop <fence-line> raw=N flat=M` findings.

- [x] **Step 1: Write the failing test**

In `scan_fences`, insert the two floors **between** the `flatout=` assignment and the `while IFS="$TAB9" read -r p pv` loop:

```bash
    flat="$(printf '%s\n' "$flatout" | grep -c .)"
    if [ "$flat" -eq 0 ]; then echo "empty $line"; continue; fi
    raw="$(printf '%s\n' "$body" | grep -vE '^[[:space:]]*$' | grep -vcE '^[[:space:]]*#')"
    [ "$raw" = "$flat" ] || echo "drop $line raw=$raw flat=$flat"
```

Add `flat` and `raw` to the function's `local` declaration, so the first line of `scan_fences` reads:

```bash
  local md="$1" line ind body flatout flat raw p pv top
```

Then append the two asserts after the existing `f9_miss` assert:

```bash
# NON-VACUITY FLOOR 2 — a fence that flattens to ZERO paths contributes nothing to the existence
# loop above, so it would be silently unguarded rather than reported.
f9_empty="$(printf '%s\n' "$findings9" | grep '^empty ' | sed 's/^empty //' | tr '\n' ' ')"
assert "(9) every config fence flattens to at least one key (fence lines listed; ${f9_empty:-none empty})" \
  '[ -z "$f9_empty" ]'

# NON-VACUITY FLOOR 3 — SAFETY NET for flatten_yaml's deliberately narrow key class. A key spelled
# outside [A-Za-z_][A-Za-z0-9_-]* is silently REJECTED by the flattener rather than flagged, and
# because the existence loop iterates POST-filter output, a dropped line is invisible to it. Cross-
# check structurally: every non-blank, non-full-line-comment line in a fence must survive
# flattening into exactly one path.
f9_drop="$(printf '%s\n' "$findings9" | grep '^drop ' | sed 's/^drop //' | tr '\n' ' ')"
assert "(9) the flattener drops no key-shaped line in any fence (raw content lines vs flattened, per fence; ${f9_drop:-none dropped})" \
  '[ -z "$f9_drop" ]'
```

- [x] **Step 2: Run tests to verify they pass**

Run: `bash tests/test_docket_example_yml.sh 2>&1 | grep -E 'flattens to at least|drops no key-shaped'`

Expected: both `ok -`, reporting `none empty` and `none dropped`.

- [x] **Step 3: Mutation test — regress `flatten_yaml`'s key class**

Temporarily revert **both** occurrences of `[A-Za-z0-9_-]*:` in `flatten_yaml` to `[A-Za-z0-9_]*:`, then run:

```bash
bash tests/test_docket_example_yml.sh 2>&1 | grep -E '^NOT OK'
```

Expected: floor 3 reddens with **`289 raw=11 flat=10 310 raw=11 flat=10`**, alongside Task 1's two asserts. This is the pairing that makes Task 1 non-optional: without the widening, this floor ships RED on correct prose.

**Restore the widening** and re-run to green.

- [x] **Step 4: Verify floor 2 can fire**

Floor 2 has no natural trigger in the README, so prove it is reachable rather than dead:

```bash
bash -c 'printf "# Fixture\n\n\`\`\`yaml\n# only a comment\n\`\`\`\n" > /tmp/empty-fence.md && cat /tmp/empty-fence.md'
```

Confirm by reasoning against the code: the body is a single full-line comment, `flatten_yaml` skips `^[[:space:]]*#`, so `flat=0` and the `empty` finding fires before the `raw` comparison. The `continue` is what stops a zero-key fence from also reporting a spurious `drop`.

- [x] **Step 5: Commit**

```bash
git add tests/test_docket_example_yml.sh
git commit -m "test(0108): add per-fence non-vacuity floors

Floor 2: a fence flattening to zero paths reddens instead of silently
contributing nothing to the existence loop. Floor 3: every non-blank,
non-comment line in a fence must survive flattening into exactly one
path, so a key spelled outside the flattener's key class is flagged
rather than dropped.

Floor 3 is what the Task 1 widening exists to keep green: hyphen-free,
fences 289/310 report raw=11 flat=10."
```

---

### Task 5: Marker grammar — `ignore`, hard-fail on malformed, fixture-tested

**Files:**
- Modify: `tests/test_docket_example_yml.sh` (section `(9)`)

**Interfaces:**
- Consumes: `fence_openers`, `scan_fences`.
- Produces: `fence_marker <markdown-path> <startline> <fence-indent>` → `NONE` | `TOKEN <ignore|values>` | `BAD <reason>`; `marker <fence-line> <reason>` findings; `ignore`-marked fences skipped entirely.

- [x] **Step 1: Write the failing test**

Add `fence_marker` immediately after `fence_body` in section `(9)`:

```bash
# MARKER GRAMMAR. Two markers attach to a fence:
#   <!-- docket:config-fence: ignore -->   not .docket.yml schema — skip this fence entirely
#   <!-- docket:config-fence: values -->   also assert value equality against the example
#
# ATTACHMENT is the NEAREST PRECEDING NON-BLANK line, not strictly the line above. Fence 576
# forces this: it is a list-item continuation preceded by a blank line, and a column-0 HTML
# comment there would terminate the enclosing list. So the marker may carry leading whitespace,
# and must sit at AT LEAST its fence's own indent.
#
# AN UNKNOWN OR MALFORMED TOKEN IS A HARD FAIL, never warned-and-ignored, because the two mistake
# directions are ASYMMETRIC: a typo'd `ignore` fails safe (the fence is still checked and
# reddens, loudly), but a typo'd `values`, a typo'd marker name, or a bare
# `<!-- docket:config-fence -->` fails OPEN AND SILENT — value coverage evaporates with no signal,
# which is precisely the drift class this change exists to end. Any line matching
# docket:config-fence that does not match the exact grammar reddens.
#
# AT MOST ONE MARKER PER FENCE; a second reddens rather than one silently winning.
fence_marker(){
  awk -v s="$2" -v find="$3" '
    NR >= s { exit }
    $0 !~ /^[[:space:]]*$/ { prev2 = prev1; prev1 = $0 }
    END {
      if (prev1 !~ /docket:config-fence/) { print "NONE"; exit }
      if (prev2 ~ /docket:config-fence/)  { print "BAD duplicate-marker"; exit }
      mind = match(prev1, /[^[:space:]]/) - 1
      if (mind < find) { print "BAD marker-indent-below-fence"; exit }
      if (prev1 ~ /^[[:space:]]*<!--[[:space:]]+docket:config-fence:[[:space:]]+(ignore|values)[[:space:]]+-->[[:space:]]*$/) {
        t = prev1
        sub(/^.*docket:config-fence:[[:space:]]*/, "", t)
        sub(/[[:space:]]*-->.*$/, "", t)
        print "TOKEN " t
      } else { print "BAD malformed-marker" }
    }
  ' "$1"
}
```

In `scan_fences`, add marker handling as the **first thing inside the opener loop**, before `body=` is computed:

```bash
    marker="$(fence_marker "$md" "$line" "$ind")"
    case "$marker" in
      NONE)      token="" ;;
      "TOKEN "*) token="${marker#TOKEN }" ;;
      "BAD "*)   echo "marker $line ${marker#BAD }"; continue ;;
      *)         echo "marker $line unparseable"; continue ;;
    esac
    [ "$token" = "ignore" ] && continue
```

Extend the `local` line to:

```bash
  local md="$1" line ind body flatout flat raw p pv top marker token
```

Append the marker assert and the fixture tests after the floor asserts:

```bash
f9_marker="$(printf '%s\n' "$findings9" | grep '^marker ' | sed 's/^marker //' | tr '\n' ' ')"
assert "(9) every docket:config-fence marker parses (fence-line + reason; ${f9_marker:-none malformed})" \
  '[ -z "$f9_marker" ]'

# The ignore path has ZERO exercise in the README — all nine fences today are config fences — so
# without a fixture it would ship with its only branch untested. Assert it POSITIVELY on a
# temporary fixture rather than by adding a real ignored fence to the README. This is why the
# helpers above take a markdown path as an ARGUMENT instead of reading $README.
fx9="$tmp/fence-fixture.md"
printf '# Fixture\n\n<!-- docket:config-fence: ignore -->\n```yaml\nnot_a_docket_key: true\n```\n' > "$fx9"
fx9_count="$(fence_openers "$fx9" | grep -c .)"
assert "(9) fixture scaffold is valid — one discoverable fence (got $fx9_count)" '[ "$fx9_count" = "1" ]'
fx9_marker="$(fence_marker "$fx9" 4 0)"
assert "(9) an ignore marker parses to its token (got $fx9_marker)" '[ "$fx9_marker" = "TOKEN ignore" ]'
fx9_findings="$(scan_fences "$fx9")"
assert "(9) an ignore-marked fence is skipped entirely — its non-schema key raises nothing (got [${fx9_findings}])" \
  '[ -z "$fx9_findings" ]'

# ...and the SAME fixture without the marker must report the key, so the assert above is proven to
# be the marker working rather than the fixture being invisible to the scanner.
fx9b="$tmp/fence-fixture-unmarked.md"
printf '# Fixture\n\n```yaml\nnot_a_docket_key: true\n```\n' > "$fx9b"
fx9b_findings="$(scan_fences "$fx9b")"
assert "(9) the same fence WITHOUT the ignore marker does report its key — proves the skip is the marker, not an invisible fixture (got [${fx9b_findings}])" \
  '[ "$fx9b_findings" = "miss 3 not_a_docket_key" ]'
```

- [x] **Step 2: Run tests to verify they pass**

Run: `bash tests/test_docket_example_yml.sh 2>&1 | grep -E '^(ok|NOT OK) - \(9\)'`

Expected: all `(9)` asserts `ok -`, including `an ignore-marked fence is skipped entirely (got [])` and the unmarked control reporting `miss 3 not_a_docket_key`.

- [x] **Step 3: Mutation test — malformed marker tokens**

Run each of these against a temporary README copy and confirm the marker assert reddens:

```bash
bash -c 'cp README.md /tmp/README.bak
for m in "<!-- docket:config-fence: valeus -->" "<!-- docket:config-fence -->" "<!-- docket:confg-fence: values -->"; do
  awk -v M="$m" "NR==263{print M} {print}" /tmp/README.bak > README.md
  printf "%s => " "$m"
  bash tests/test_docket_example_yml.sh 2>&1 | grep -E "marker parses" | head -1
done
cp /tmp/README.bak README.md'
```

Expected:
- `valeus` → `NOT OK ... (265 malformed-marker)`
- bare `<!-- docket:config-fence -->` → `NOT OK ... (265 malformed-marker)`
- `confg-fence` (typo'd marker *name*) → `ok ... (none malformed)` — this one is **correctly** invisible, since it does not match `docket:config-fence` at all and so is just an HTML comment. Note it and move on; the fence itself is still existence-checked, which is the fail-safe direction.

Then the duplicate-marker case:

```bash
bash -c 'cp README.md /tmp/README.bak
awk "NR==263{print \"<!-- docket:config-fence: values -->\"; print \"<!-- docket:config-fence: values -->\"} {print}" /tmp/README.bak > README.md
bash tests/test_docket_example_yml.sh 2>&1 | grep -E "marker parses"
cp /tmp/README.bak README.md'
```

Expected: `NOT OK ... (266 duplicate-marker)`

Confirm `git diff --stat README.md` is empty afterward.

- [x] **Step 4: Run the full suite**

Run: `bash tests/test_docket_example_yml.sh; echo "EXIT=$?"`

Expected: `EXIT=0`, 0 `NOT OK`.

- [x] **Step 5: Commit**

```bash
git add tests/test_docket_example_yml.sh
git commit -m "test(0108): parse config-fence markers, hard-failing on malformed tokens

Markers attach as the nearest preceding non-blank line (fence 576 is an
indented list continuation, so a column-0 comment there would terminate
the list) and must sit at least at their fence's indent. An unknown or
malformed token is a hard fail rather than warned-and-ignored: a typo'd
`values` otherwise fails open and silent, evaporating value coverage with
no signal.

The ignore path has no exercise in the README, so it is asserted
positively on a temporary fixture, plus an unmarked control proving the
skip is the marker rather than an invisible fixture."
```

---

### Task 6: Opt-in value equality, applied to the `reclaim:` fence

**Files:**
- Modify: `tests/test_docket_example_yml.sh` (section `(9)`)
- Modify: `README.md` (one line inserted at 233)

**Interfaces:**
- Consumes: `fence_marker`'s `values` token, `ex_flat`.
- Produces: `value <fence-line> <path> readme=X example=Y` findings.

- [x] **Step 1: Write the failing test**

In `scan_fences`, extend the active-key branch of the path loop. Replace:

```bash
      if grep -Fxq "$top" <<<"$ex9_paths"; then
        grep -Fxq "$p" <<<"$ex9_paths" || echo "miss $line $p"
```

with:

```bash
      if grep -Fxq "$top" <<<"$ex9_paths"; then
        if ! grep -Fxq "$p" <<<"$ex9_paths"; then
          echo "miss $line $p"
        elif [ "$token" = "values" ]; then
          exval="$(awk -F"$TAB9" -v k="$p" '$1==k{print $2; exit}' <<<"$ex_flat")"
          [ "$pv" = "$exval" ] || echo "value $line $p readme=$pv example=$exval"
        fi
```

Extend the `local` line to:

```bash
  local md="$1" line ind body flatout flat raw p pv top marker token exval
```

Append the assert after the marker assert:

```bash
# VALUE EQUALITY IS OPT-IN, and it is not lost where it is SOUND. Section (8) keeps it on fence
# 209 (the per-repo snippet, which documents shipped defaults); this marker adds it to the
# reclaim: fence, whose lease_ttl: 72 / auto: false are also shipped defaults and SHOULD redden if
# the defaults move. The other seven fences stay existence-only, because they deliberately
# illustrate non-default values. Fence 209 is therefore double-covered by (8) (existence + values)
# and (9) (existence only); that overlap is accepted rather than special-cased — (8)'s fence is
# simply left unmarked, and since no unmarked fence gets a value assert, no special-casing exists.
f9_value="$(printf '%s\n' "$findings9" | grep '^value ' | sed 's/^value //' | tr '\n' ' ')"
assert "(9) values-marked fences match the example exactly (${f9_value:-none mismatched})" \
  '[ -z "$f9_value" ]'
```

- [x] **Step 2: Run test to verify it passes trivially (no marker in the README yet)**

Run: `bash tests/test_docket_example_yml.sh 2>&1 | grep -E 'values-marked'`

Expected: `ok - (9) values-marked fences match the example exactly (none mismatched)` — vacuously true, because no fence carries the marker yet. Step 3 gives it a population.

- [x] **Step 3: Add the marker to the `reclaim:` fence in `README.md`**

`README.md` line 233 is currently blank, sitting between the last bullet (232) and the fence opener (234). Replace that blank line with the marker:

```bash
bash -c 'awk "NR==233{print \"<!-- docket:config-fence: values -->\"; next} {print}" README.md > /tmp/README.new && mv /tmp/README.new README.md'
```

Verify the result:

```bash
sed -n '231,239p' README.md
```

Expected:

```
- **Run it by hand anytime** with `docket.sh reclaim-claims`, whether or not `reclaim.auto` is set.
- **A change with a branch is left to a human.** It might carry real, unpushed work, so reclaim never touches it — it stays flagged instead.
<!-- docket:config-fence: values -->
```yaml
reclaim:
  lease_ttl: 72   # hours; >= docket-status's 3-day stale-in-progress window
  auto: false     # true => docket-status self-heals eligible claims each pass
```
```

**Critical:** the marker must land at line 233, **outside** section `(8)`'s span (203–224, terminated by the `### Reclaiming stale claims` heading at 225), so `snippet_section()` is unperturbed.

- [x] **Step 4: Run the full suite**

Run: `bash tests/test_docket_example_yml.sh; echo "EXIT=$?"`

Expected: `EXIT=0`, 0 `NOT OK`. In particular `(8)`'s asserts must be unchanged (`snippet flattened key count is exactly 5`, `raw=5 flattened=5`) — proof the marker did not leak into `(8)`'s section span.

- [x] **Step 5: Mutation test — drift a shipped default, and prove the marker is what catches it**

Drift each `reclaim` default in turn and confirm the value assert reddens:

```bash
bash -c 'cp README.md /tmp/README.bak
sed "s|^  lease_ttl: 72|  lease_ttl: 96|" /tmp/README.bak > README.md
bash tests/test_docket_example_yml.sh 2>&1 | grep -E "values-marked"
sed "s|^  auto: false|  auto: true|" /tmp/README.bak > README.md
bash tests/test_docket_example_yml.sh 2>&1 | grep -E "values-marked"
cp /tmp/README.bak README.md'
```

Expected:
- `NOT OK ... (234 reclaim.lease_ttl readme=96 example=72)`
- `NOT OK ... (234 reclaim.auto readme=true example=false)`

Now the **control** — the same drift with the marker removed must be **silent**, proving the marker is load-bearing rather than the drift being caught by something else:

```bash
bash -c 'cp README.md /tmp/README.bak
sed "/docket:config-fence: values/d" /tmp/README.bak | sed "s|^  lease_ttl: 72|  lease_ttl: 96|" > README.md
bash tests/test_docket_example_yml.sh 2>&1 | grep -E "values-marked"
cp /tmp/README.bak README.md'
```

Expected: `ok - (9) values-marked fences match the example exactly (none mismatched)` — silent, because unmarked fences are existence-only.

Confirm `git diff --stat README.md` shows only the single added marker line.

- [x] **Step 6: Commit**

```bash
git add tests/test_docket_example_yml.sh README.md
git commit -m "test(0108): opt in the reclaim: fence to value equality

Value equality stays where it is sound. Section (8) keeps it on the
per-repo snippet; this adds it to the reclaim: fence, whose lease_ttl: 72
and auto: false are shipped defaults that should redden if the defaults
move. The remaining seven fences stay existence-only because they
deliberately illustrate non-default values.

The marker sits at README:233, outside section (8)'s span (203-224), so
snippet_section() is unperturbed."
```

---

## Self-Review

**Spec coverage:**

| Spec section | Task |
|---|---|
| §1 Fence discovery — derived, default-in, whitespace-tolerant | Task 2 |
| §1 Marker grammar — attachment, hard fail, at-most-one | Task 5 |
| §2 Anchor — the example, one hop | Task 3 (comment + `ex_flat` reuse) |
| §3 Assert — existence-only | Task 3 |
| §3 Value equality opt-in, applied to `reclaim:` | Task 6 |
| §4 Key resolution — query-by-key, residual hole documented | Task 3 |
| §5 Nested paths + `flatten_yaml` widening at BOTH occurrences | Task 1 |
| §6 Floor 1 exact count = 9 | Task 2 |
| §6 Floor 2 per-fence non-empty | Task 4 |
| §6 Floor 3 raw-vs-flattened | Task 4 |
| §6 Mutation 1 phantom key | Task 3 Step 3 |
| §6 Mutation 2 column-0 regex | Task 2 Step 4 |
| §6 Mutation 3 hyphen-free key class | Task 4 Step 3 |
| §6 Mutation 4 marker parse + positive `ignore` on a fixture | Task 5 Steps 1, 3 |
| A9 reverse loop deliberately absent | Task 3 (comment) |

**Beyond the spec, added because the build verified they were needed:**

- **Task 1 Step 5** — the half-fix mutation. The spec states floor 3 cannot catch a half-widening; this was confirmed empirically (half-fix and full-fix both yield 3 paths on the fixture and `flat=11` on fences 289/310), so the value assert is the only guard and is mutation-tested as such.
- **Task 5's unmarked control fixture** — proves the `ignore` skip is the marker working, not the fixture being invisible to the scanner. Without it, "ignore-marked fence raises nothing" passes vacuously if `scan_fences` never sees the fixture at all (`backstop-must-compute-not-reenumerate`: mutation-test the population, not only the suppression).
- **Task 6 Step 5's control** — proves the `values` marker is load-bearing by showing the identical drift is silent without it.

**Type/name consistency:** `fence_openers`, `fence_body`, `fence_marker`, `is_pseudo_key`, `scan_fences`, `TAB9`, `ex9_paths`, `findings9`, and the finding prefixes `miss`/`empty`/`drop`/`marker`/`seen`/`value` are spelled identically in every task. `scan_fences`'s `local` list grows across Tasks 3→4→5→6 and its final form is `md line ind body flatout flat raw p pv top marker token seen_token exval`.

**Verification status:** every helper in this plan was prototyped and run against the real `README.md` and `.docket.example.yml` before the plan was written. The clean run reports 9 fences and zero findings; mutations 1–4, the duplicate-marker case, the `values` drift cases, and the `ignore` fixture all produce the expected findings. The plan's asserts are therefore known-satisfiable, not drafts (`plan-supplied-test-code-is-unverified`).

## Corrections applied during execution

- **Task 3 Step 1's piped `grep -Fxq` form was corrected mid-branch by commit `7b32c2b`, and Task 6 Step 1's code blocks above have been updated to match.** `ex9_paths` is a captured variable, and `grep -Fxq` exits as soon as it matches; feeding it via `printf '%s\n' "$ex9_paths" | grep -Fxq ...` under this file's `set -uo pipefail` is exactly the producer-piped-into-early-exiting-consumer hazard AGENTS.md's Shell section forbids — the still-writing `printf` can take SIGPIPE (141), and under `pipefail` that 141 can replace `grep`'s real exit status on the pipeline. The shipped code has used the here-string form (`grep -Fxq "$x" <<<"$var"`) since that commit, which landed right after Task 3 and before Tasks 4–6 were built on top of it; this plan document itself was never reconciled until this correction, so it kept showing the pre-fix piped form as if it were still current. Recorded here so a future reader trusts the *why* (a real pipefail hazard, not a style preference), not just the corrected spelling.
- **The `values` marker's population floors (NON-VACUITY FLOOR 4 and FLOOR 5 in the shipped `(9)` section) were added after this change's whole-branch review, not as part of the original Task 6 design above.** The review proved that deleting the `values` marker line from README, or moving it one non-blank line earlier (still well-formed, but no longer attached to the `reclaim:` fence), left `f9_value` fully green with no floor to catch it — green for a reason other than the property it claims, the exact fail-open-and-silent mode this whole section exists to end. The fix: `scan_fences` now emits a `seen <fence-line> <token>` record for every fence it reaches, before any marker-driven `continue`, giving (a) an exact count of fences visited and (b) a floor that at least one fence carries the `values` token; a further reconciliation assert compares the whole README's `docket:config-fence` line count against how many fences actually consumed a marker, catching an orphaned marker (one separated from its fence by other content) that `fence_marker` silently returns `NONE` for. All four scenarios were mutation-tested in a scratch copy outside the worktree and confirmed to redden.
- **A second re-review found the FLOOR 4 "at least one fence is values-marked" count was itself a displacement gap, closed in a follow-up fix pass (change 0108's fix-2).** Moving the `values` marker from the `reclaim:` fence onto fence 209 (the per-repo snippet, which also documents shipped defaults) left the count-based floor green — fence 209 absorbs the marker — while `reclaim.lease_ttl` drifted undetected. The fix added a positive control (`fx9e`) that drifts `reclaim.lease_ttl` in a fixture and demands a `value` finding naming that key specifically, regardless of which fence currently carries the marker; the same pass also tightened the `fx9c`/`fx9d` fixture asserts to exact-match the finding string rather than mere presence, and corrected this section's own stale finding-prefix/`local`-list inventory (the omission of `seen` and `exval`→`seen_token exval`, above).
