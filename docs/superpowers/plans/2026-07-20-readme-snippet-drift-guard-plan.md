# README Snippet Drift Guard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add one numbered section — `(8) README SNIPPET CORRESPONDENCE` — to `tests/test_docket_yml_example.sh` that proves the README's five-key `.docket.yml` snippet agrees with `.docket.yml.example`, and that the section's pointer to the example resolves.

**Architecture:** Two small shell helpers (a fenced-block extractor scoped to one README heading, and a generic indent-stack YAML flattener producing `path<TAB>value` lines), then three assert groups over them: a non-vacuity count floor, a forward-only correspondence loop (`snippet ⊆ example`, values equal), and a section-scoped pointer-target check. All of it appends to the existing suite after section `(7)`; no other file changes.

**Tech Stack:** POSIX-ish bash 3.2 (macOS default) + awk + sed. No new dependencies. The suite runs `set -uo pipefail` with **no `-e`**, and reports through the existing `assert(){ if eval "$2"; ...}` helper, accumulating into `fail`.

## Global Constraints

- **Target file:** `tests/test_docket_yml_example.sh` — the new section is appended **after** the existing `--- (7) README + dogfooding ---` block and **before** the final `exit $fail`.
- **Forward direction only.** The correspondence loop iterates the **snippet's** keys and asserts each exists in the example with an equal value. It **must not** iterate the example's keys. The reverse loop is a completeness assert — precisely the fourth all-keys surface change 0101 deleted. This is a deliberate departure from the `correspondence-guard-runs-one-way` learning and **must be written into the test as a comment** (Task 2), so a later reader does not "fix" it.
- **Non-vacuity floor is an exact count, not `>= 1`.** Currently **5** flattened paths: `metadata_branch`, `integration_branch`, `board_surfaces`, `finalize`, `finalize.gate`.
- **Bash 3.2 portability:** no associative arrays, no `mapfile`, no `declare -A`, no GNU-only `sed -i` / `grep -P`.
- **Never pipe into the correspondence `while read` loop.** A pipe puts the loop in a subshell and the accumulated failure variables vanish; feed it with a heredoc so it runs in the current shell.
- **Mutation-prove every assert before calling a task done** — strip or break the guarded property, watch the specific assert print `NOT OK`, then restore and confirm `ok`. A guard that never fired is decoration.
- Existing variables already in scope from earlier sections and **reused, not redefined**: `REPO`, `EX`, `README`, `assert`, `fail`.
- No changes to `README.md` or `.docket.yml.example` themselves — they are currently honest, so the new section must go **green on first run**.

## Verified against the real tree

The helper implementations below were executed against `README.md` and `.docket.yml.example` at `origin/main` (commit `3e26790`) while this plan was written. Their exact observed output:

```
snippet flat:                       example flat (excerpt):
metadata_branch<TAB>docket          metadata_branch<TAB>docket
integration_branch<TAB>auto         integration_branch<TAB>auto
board_surfaces<TAB>[inline]         finalize<TAB>
finalize<TAB>                       finalize.gate<TAB>local
finalize.gate<TAB>local             runners.codex.sandbox<TAB>workspace-write
```

The flattener already resolves the example's **three-level** path `runners.codex.sandbox`, which is the genericity proof the spec asks for — it is not hardcoded to the one known nested path.

---

### Task 1: Extraction + flattening helpers, with the non-vacuity floor

**Files:**
- Modify: `tests/test_docket_yml_example.sh` (append a new section after the `(7)` block, before `exit $fail`)

**Interfaces:**
- Consumes: `REPO`, `EX`, `README`, `assert`, `fail` — all already defined earlier in the file.
- Produces, for Tasks 2 and 3:
  - `readme_snippet()` — stdout: the raw lines inside the first ```` ```yaml ```` fence under the `### \`.docket.yml\` — per-repo settings` heading.
  - `flatten_yaml()` — stdin: YAML lines; stdout: one `path<TAB>value` line per key, dotted by indentation, comments and blanks dropped.
  - `sn_flat` — the flattened snippet (5 lines today).
  - `ex_flat` — the flattened example (whole file).

- [ ] **Step 1: Write the failing test**

Append to `tests/test_docket_yml_example.sh`, immediately after the `(7) README + dogfooding` block's last assert (the `repo .docket.yml keeps its set values` assert) and before `exit $fail`:

```bash
# --- (8) README SNIPPET CORRESPONDENCE ---------------------------------------
# The README carries a small illustrative .docket.yml snippet (change 0101 cut it down from a
# full all-keys sample). Nothing tested it against the canonical example, so its values could
# drift silently and its pointer could rot. This section closes that (change 0107).
#
# $README is already set by (7) above.

# Extract the fenced YAML block under the per-repo-settings heading. Scoped to that ONE heading:
# a whole-file grep would happily match some other snippet if this section were renamed or moved.
readme_snippet(){
  awk '
    /^### `\.docket\.yml` — per-repo settings$/ { inseg=1; next }
    inseg && /^### / { exit }
    inseg && /^```yaml$/ && !seen { infence=1; seen=1; next }
    infence && /^```$/ { exit }
    infence { print }
  ' "$README"
}

# Flatten block-mapping YAML to "path<TAB>value" lines, dotting by INDENTATION rather than
# hardcoding the one nested path we happen to know about (finalize.gate). An indent stack, so
# depth is generic: it resolves the example's three-level runners.codex.sandbox correctly.
# Deliberately NOT a general YAML parser — it covers exactly the block-mapping subset these two
# files use (scalar and inline-list values, full-line and trailing comments). Do not grow it.
flatten_yaml(){
  awk '
    { line = $0
      if (line ~ /^[[:space:]]*$/) next
      if (line ~ /^[[:space:]]*#/) next
      sub(/[[:space:]]+#.*$/, "", line)
      sub(/[[:space:]]+$/, "", line)
      if (line !~ /^[[:space:]]*[A-Za-z_][A-Za-z0-9_]*:/) next
      ind = match(line, /[^ ]/) - 1
      key = line; sub(/^[[:space:]]*/, "", key); sub(/:.*$/, "", key)
      val = line; sub(/^[[:space:]]*[A-Za-z_][A-Za-z0-9_]*:[[:space:]]*/, "", val)
      while (depth > 0 && indents[depth] >= ind) depth--
      depth++; indents[depth] = ind; keys[depth] = key
      path = keys[1]
      for (i = 2; i <= depth; i++) path = path "." keys[i]
      printf "%s\t%s\n", path, val
    }'
}

sn_flat="$(readme_snippet | flatten_yaml)"
ex_flat="$(flatten_yaml < "$EX")"

# NON-VACUITY FLOOR. The forward loop below iterates the snippet's keys, so its real failure mode
# is iterating an EMPTY set: rename the heading, retitle the fence, or move the section, and
# extraction yields nothing while every assert sails through proving nothing. An EXACT count (not
# ">= 1") is the right bar — it also reddens if the snippet quietly grows back toward being the
# all-keys mirror change 0101 deleted. If you intentionally added a snippet key, bump this number
# in the same commit AND add the key to .docket.yml.example.
sn_count="$(printf '%s\n' "$sn_flat" | grep -c .)"
assert "(8) snippet extraction found exactly 5 keys (non-vacuity floor; got $sn_count)" \
  '[ "$sn_count" = "5" ]'
assert "(8) example flattened non-empty (guard against a silently empty comparison side)" \
  '[ "$(printf "%s\n" "$ex_flat" | grep -c .)" -ge 20 ]'
```

- [ ] **Step 2: Run the test to verify the new asserts FIRE (mutation, not a code bug)**

Both new asserts must be shown capable of failing. Mutation A — break the fence so extraction finds nothing:

```bash
cd /Users/homer/dev/docket/.worktrees/guard-the-readme-config-snippet-against-docket-yml-example-d
cp README.md /tmp/README.bak
sed -i '' 's/^```yaml$/```YAML-BROKEN/' README.md
bash tests/test_docket_yml_example.sh 2>&1 | grep '(8)'
```

Expected: `NOT OK - (8) snippet extraction found exactly 5 keys (non-vacuity floor; got 0)`

Restore, then Mutation B — rename the heading:

```bash
cp /tmp/README.bak README.md
sed -i '' 's/^### `\.docket\.yml` — per-repo settings$/### `.docket.yml` — settings/' README.md
bash tests/test_docket_yml_example.sh 2>&1 | grep '(8)'
```

Expected: again `NOT OK - ... (got 0)` — proves the extractor is anchored on the heading, not merely on the first fence in the file.

Restore: `cp /tmp/README.bak README.md`

- [ ] **Step 3: Verify the honest tree is GREEN**

```bash
bash tests/test_docket_yml_example.sh 2>&1 | grep '(8)'
```

Expected, exactly:

```
ok - (8) snippet extraction found exactly 5 keys (non-vacuity floor; got 5)
ok - (8) example flattened non-empty (guard against a silently empty comparison side)
```

- [ ] **Step 4: Prove the flattener is generic, not accidentally passing on one hardcoded path**

Add a temporary three-level key to the example, then run the flattener directly and read its output:

```bash
printf 'probe:\n  nested:\n    deep: yes\n' >> .docket.yml.example
bash -c '
bash -c '
  flatten_yaml(){ awk '"'"'
    { line = $0
      if (line ~ /^[[:space:]]*$/) next
      if (line ~ /^[[:space:]]*#/) next
      sub(/[[:space:]]+#.*$/, "", line); sub(/[[:space:]]+$/, "", line)
      if (line !~ /^[[:space:]]*[A-Za-z_][A-Za-z0-9_]*:/) next
      ind = match(line, /[^ ]/) - 1
      key = line; sub(/^[[:space:]]*/, "", key); sub(/:.*$/, "", key)
      val = line; sub(/^[[:space:]]*[A-Za-z_][A-Za-z0-9_]*:[[:space:]]*/, "", val)
      while (depth > 0 && indents[depth] >= ind) depth--
      depth++; indents[depth] = ind; keys[depth] = key
      path = keys[1]; for (i = 2; i <= depth; i++) path = path "." keys[i]
      printf "%s\t%s\n", path, val
    }'"'"' ; }
  flatten_yaml < .docket.yml.example | grep -E "probe|runners\.codex\.sandbox"
'
```

Expected output includes both `runners.codex.sandbox<TAB>workspace-write` and `probe.nested.deep<TAB>yes` — dotted to full depth.

Now revert the probe key:

```bash
git checkout .docket.yml.example
git diff --stat   # expect: only tests/test_docket_yml_example.sh modified
```

- [ ] **Step 5: Commit**

```bash
git add tests/test_docket_yml_example.sh
git commit -m "test(0107): extract + flatten the README config snippet, with a non-vacuity floor"
```

---

### Task 2: The forward correspondence loop (`snippet ⊆ example`, values equal)

**Files:**
- Modify: `tests/test_docket_yml_example.sh` (extend section `(8)`)

**Interfaces:**
- Consumes: `sn_flat`, `ex_flat`, `assert` from Task 1.
- Produces: nothing further; Task 3 is independent of this loop's variables.

- [ ] **Step 1: Write the failing test**

Append to section `(8)`, directly below Task 1's asserts:

```bash
# DIRECTION: this loop iterates the SNIPPET's keys and proves snippet ⊆ example, values equal.
# It deliberately does NOT iterate the example's keys, and the missing reverse loop is NOT an
# oversight — do not "fix" it.
#
# The correspondence-guard-runs-one-way learning (harvested from change 0101) says: name the
# direction you iterate, then write the other one. That rule assumes the two sets stand in a
# CORRESPONDENCE. These two do not. The README snippet is a deliberate PROPER SUBSET — a small
# illustrative taste — while .docket.yml.example is the canonical all-keys reference. So the
# reverse loop here would assert "every key in the example appears in the README", which is a
# completeness check that re-creates the fourth all-keys surface change 0101 existed to delete.
# Writing it would undo the change that motivated this guard.
#
# The orphan direction that actually bit 0101 — a documented key no real surface carries — is
# still covered here: a snippet key absent from the example fails the existence assert below.
# The asymmetry is safe BECAUSE of the subset relation, which was not true of 0101's
# export-keys-vs-example guards.
#
# Fed by a HEREDOC, never a pipe: a pipe runs the loop in a subshell and both accumulator
# variables come back empty, so every mismatch would silently pass.
sn_missing=""
sn_mismatched=""
while IFS="$(printf '\t')" read -r sn_path sn_val; do
  [ -n "$sn_path" ] || continue
  ex_hit="$(printf '%s\n' "$ex_flat" | awk -F'\t' -v p="$sn_path" '$1==p{print "1"; exit}')"
  if [ -z "$ex_hit" ]; then
    sn_missing="$sn_missing $sn_path"
    continue
  fi
  ex_val="$(printf '%s\n' "$ex_flat" | awk -F'\t' -v p="$sn_path" '$1==p{print $2; exit}')"
  if [ "$ex_val" != "$sn_val" ]; then
    sn_mismatched="$sn_mismatched $sn_path(README='$sn_val'!=example='$ex_val')"
  fi
done <<SNIPPET_KEYS
$sn_flat
SNIPPET_KEYS

assert "(8) every README snippet key exists in the example (${sn_missing:-none missing})" \
  '[ -z "$sn_missing" ]'
assert "(8) every README snippet value equals the example's (${sn_mismatched:-none mismatched})" \
  '[ -z "$sn_mismatched" ]'
```

- [ ] **Step 2: Run the test to verify both asserts FIRE**

Mutation A — a drifted **value** in the README snippet:

```bash
cd /Users/homer/dev/docket/.worktrees/guard-the-readme-config-snippet-against-docket-yml-example-d
cp README.md /tmp/README.bak
sed -i '' 's/^board_surfaces: \[inline\]     # derived board views/board_surfaces: [inline, github]     # derived board views/' README.md
bash tests/test_docket_yml_example.sh 2>&1 | grep '(8)'
```

Expected: `NOT OK - (8) every README snippet value equals the example's ( board_surfaces(README='[inline, github]'!=example='[inline]'))`
The existence assert stays `ok` — the key still exists; only its value drifted.

Restore, then Mutation B — a snippet **key that is not in the example**:

```bash
cp /tmp/README.bak README.md
sed -i '' 's/^board_surfaces: \[inline\]/phantom_key: yes\nboard_surfaces: [inline]/' README.md
bash tests/test_docket_yml_example.sh 2>&1 | grep '(8)'
```

Expected: `NOT OK - (8) every README snippet key exists in the example ( phantom_key)`, **and** `NOT OK` on the count floor (now 6) — both firing is correct.

Restore: `cp /tmp/README.bak README.md`

- [ ] **Step 3: Verify the honest tree is GREEN**

```bash
bash tests/test_docket_yml_example.sh 2>&1 | grep '(8)'
```

Expected: all four `(8)` lines `ok`, with `none missing` / `none mismatched` in the messages.

- [ ] **Step 4: Confirm the rationale comment is present and specific**

```bash
grep -c "do not \"fix\" it" tests/test_docket_yml_example.sh
grep -c "PROPER SUBSET" tests/test_docket_yml_example.sh
```

Expected: `1` for each. The comment is a required deliverable of the spec, not decoration — a future reader who has internalized the one-way learning must find the reasoning at the site.

- [ ] **Step 5: Commit**

```bash
git add tests/test_docket_yml_example.sh
git commit -m "test(0107): forward-only correspondence loop for the README snippet keys"
```

---

### Task 3: The pointer assert, plus whole-suite verification

**Files:**
- Modify: `tests/test_docket_yml_example.sh` (close out section `(8)`)

**Interfaces:**
- Consumes: `REPO`, `README`, `assert` — all pre-existing.
- Produces: nothing; this is the last section of the file, followed by the untouched `exit $fail`.

- [ ] **Step 1: Write the failing test**

Append to section `(8)`, below Task 2's asserts:

```bash
# POINTER: the section's link to the canonical reference must resolve to a real file. Scoped to
# this section's body, NOT a whole-file grep — the README names .docket.yml.example in several
# other places (the tooling list, the layered-config prose), so an unscoped match would stay green
# even after THIS section's own link rotted.
snippet_section(){
  awk '
    /^### `\.docket\.yml` — per-repo settings$/ { inseg=1; next }
    inseg && /^### / { exit }
    inseg { print }
  ' "$README"
}
sn_ptr="$(snippet_section | sed -nE 's/.*\[`?\.docket\.yml\.example`?\]\(([^)]+)\).*/\1/p' | head -n1)"
assert "(8) the section links to the canonical reference" '[ -n "$sn_ptr" ]'
assert "(8) canonical-reference link target exists (${sn_ptr:-<no link>})" \
  '[ -n "$sn_ptr" ] && [ -f "$REPO/$sn_ptr" ]'
```

- [ ] **Step 2: Run the test to verify both asserts FIRE**

Mutation A — point the link at a nonexistent path:

```bash
cd /Users/homer/dev/docket/.worktrees/guard-the-readme-config-snippet-against-docket-yml-example-d
cp README.md /tmp/README.bak
sed -i '' 's|\[`\.docket\.yml\.example`\](\.docket\.yml\.example)|[`.docket.yml.example`](docs/.docket.yml.example)|' README.md
bash tests/test_docket_yml_example.sh 2>&1 | grep '(8) canonical'
```

Expected: `NOT OK - (8) canonical-reference link target exists (docs/.docket.yml.example)`

Restore, then Mutation B — remove the link entirely from this section (keeping the plain-text mention):

```bash
cp /tmp/README.bak README.md
sed -i '' 's|\*\*\[`\.docket\.yml\.example`\](\.docket\.yml\.example) is the canonical|**`.docket.yml.example` is the canonical|' README.md
bash tests/test_docket_yml_example.sh 2>&1 | grep '(8) the section links'
```

Expected: `NOT OK - (8) the section links to the canonical reference` — proving the assert is scoped to this section and not satisfied by the README's other mentions of the same filename.

Restore: `cp /tmp/README.bak README.md`

- [ ] **Step 3: Verify the whole file is byte-clean and the honest tree is GREEN**

```bash
git diff --stat
bash tests/test_docket_yml_example.sh; echo "EXIT=$?"
```

Expected: `git diff --stat` shows **only** `tests/test_docket_yml_example.sh`. The suite prints `EXIT=0` and the six `(8)` asserts are all `ok`:

```
ok - (8) snippet extraction found exactly 5 keys (non-vacuity floor; got 5)
ok - (8) example flattened non-empty (guard against a silently empty comparison side)
ok - (8) every README snippet key exists in the example (none missing)
ok - (8) every README snippet value equals the example's (none mismatched)
ok - (8) the section links to the canonical reference
ok - (8) canonical-reference link target exists (.docket.yml.example)
```

- [ ] **Step 4: Run the full repo suite**

Section `(8)` reuses `$README` and appends after `(7)`; confirm nothing earlier regressed and no sibling suite reads this file.

```bash
bash tests/run_all.sh 2>&1 | tail -25
```

Expected: the same pass/fail profile as `origin/main` — no NEW failures. If `tests/run_all.sh` does not exist, run `for t in tests/test_*.sh; do echo "== $t"; bash "$t" >/dev/null 2>&1 || echo "FAILED $t"; done` and compare against the same loop run on a clean `origin/main` checkout.

- [ ] **Step 5: Commit**

```bash
git add tests/test_docket_yml_example.sh
git commit -m "test(0107): assert the README section's canonical-reference pointer resolves"
```

---

## Self-review

**Spec coverage.**

| Spec requirement | Task |
|---|---|
| New numbered section `(8) README SNIPPET CORRESPONDENCE`, placed after `(7)` | 1 (created), 2–3 (extended) |
| Extract the fenced block under the per-repo-settings heading | 1 |
| Flatten to dotted paths generically, both sides through the same flattener | 1 |
| Assert (1): each snippet key path exists in the example | 2 |
| Assert (2): each snippet value equals the example's | 2 |
| Pointer assert: link target resolves to a real file | 3 |
| Forward-only, with the reasoning written into the test as a comment | 2 |
| Non-vacuity floor as an **exact** count (5) | 1 |
| Mutation bar: value drift reddens | 2, Step 2 Mutation A |
| Mutation bar: snippet key absent from example reddens | 2, Step 2 Mutation B |
| Mutation bar: broken heading/fence reddens | 1, Step 2 Mutations A and B |
| Mutation bar: bad pointer path reddens | 3, Step 2 Mutation A |
| Mutation bar: consistent nested key stays green / flattener generic | 1, Step 4 |
| No codegen, no reverse direction, no broader README audit | out of scope — not planned |

**Placeholder scan.** Every step carries literal shell, a literal command, and its expected output. No TBDs, no "add error handling", no "similar to Task N".

**Type consistency.** Helper and variable names are used identically across tasks: `readme_snippet()`, `flatten_yaml()`, `sn_flat`, `ex_flat`, `sn_count`, `sn_missing`, `sn_mismatched`, `snippet_section()`, `sn_ptr`. Task 3's `snippet_section()` is deliberately a second, differently-bounded extractor (whole section body, not the fence) and does not shadow Task 1's `readme_snippet()`.

**Ordering note.** Task 1 must land before Tasks 2 and 3 — both depend on `sn_flat` / `ex_flat` and on the section header existing. Tasks 2 and 3 are independent of each other and could be reviewed in either order.
