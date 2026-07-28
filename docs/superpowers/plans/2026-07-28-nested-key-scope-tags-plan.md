<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0122 — Nested keys' scope tags in .docket.example.yml are unguarded](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-07-28-0122-nested-keys-scope-tags-in-docket-example-yml-are-unguarded.md)**
<!-- docket:backlink:end -->

# Nested-key scope tags — guard implementation plan (change 0122)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rewrite the scope-tag guard in `tests/test_docket_example_yml.sh` so it evaluates keys at every indentation depth instead of only column 0, closing the masking hole that let change 0102 ship a wrongly-scoped `finalize.require_pr_approval` past a green suite.

**Architecture:** One `awk` pass, hoisted into a shell variable so the mutation self-tests execute literally the same program the real assert runs. The pass tracks `(line, depth, type)` per key, maintains an ancestor stack, and emits two things on stdout: one dotted path per uncovered scalar key, and a single trailing `COUNT <n>` line giving the number of keys it enumerated at depth > 0. The shell splits those two streams apart — the paths feed the emptiness assert, the count feeds an exact population floor. No new script, no new file: the guard has exactly one consumer and stays in place.

**Tech Stack:** POSIX `awk`, Bash 4+, the repo's existing `assert` helper in `tests/test_docket_example_yml.sh`.

## Global Constraints

- **POSIX awk only.** No `gensub`, no `asort`, no other GNU extensions. Depth is measured with `match($0, /^[[:space:]]*/)` + `RLENGTH`.
- **Indent classes are `[[:space:]]`, never a literal-space class** — a tab-indented block must not be silently dropped (AGENTS.md, and the convention the existing qualified extractor at `tests/test_docket_example_yml.sh:302-321` already follows).
- **Verify with `/usr/bin/awk` and `/usr/bin/grep`, never the PATH tools.** PATH `grep` on this machine is ugrep and accepts constructs BSD grep rejects; PATH `awk` may be gawk. A suite that only passes under the PATH tools is unverified.
- **Zero edits to `.docket.example.yml`.** The change is confined to the test file. If any task appears to require editing the example, stop — that is a design deviation, not an implementation detail.
- **The three sanctioned tag forms** are exactly: `scope: repo-only (coordination-fenced, ADR-0019)`, `scope: any layer`, `scope: local-only`.
- **Change 0102's two bespoke asserts (`tests/test_docket_example_yml.sh:517` and `:519`) are KEPT.** They test a different proposition (the *specific* tag value) than the general guard (that *some* sanctioned tag covers the key). Do not retire either.
- **Never mutate the real `.docket.example.yml` in a test.** Mutation self-tests copy it to `$tmp` first.

---

## File Structure

Only one file changes.

- **Modify:** `tests/test_docket_example_yml.sh` — the scope-tag guard block, currently lines **607-669**:
  - `:607-617` — the explanatory comment, which documents the forward-extension behavior this change deletes.
  - `:618-620` — the three tag-form presence asserts. **Already correct on `main`, including `local-only`.** Leave them alone.
  - `:621-663` — the `untagged_keys="$(awk ' … ')"` pass. Replaced wholesale.
  - `:664` — the assert label, which says "every ACTIVE **top-level** key".
  - `:665-669` — the operator-facing failure block, which prints `--- untagged top-level keys ---`.

Nothing outside that block moves. The `(2c)` orphan-key check, the classification manifest, and the qualified extractor are all out of scope.

---

## Reference: the verified guard program

Every task below uses this exact program text. **It has already been run against the real `.docket.example.yml` and against three mutations; the outputs quoted in the tasks are observed, not predicted.** Do not retype it from memory — copy it.

```awk
{
  content[NR] = $0
  is_active = ($0 ~ /^[[:space:]]*[A-Za-z_][A-Za-z0-9_]*[[:space:]]*:/)
  is_pseudo = ($0 ~ /^# (agent_harnesses|agents):/)
  is_banner = ($0 ~ /^#[[:space:]]*═══/)
  if (is_active || is_pseudo || is_banner) { nb++; bnd[nb] = NR }
  if (is_active) {
    nk++
    keyline[nk] = NR
    match($0, /^[[:space:]]*/); keydepth[nk] = RLENGTH
    rest = $0
    sub(/^[[:space:]]*[A-Za-z_][A-Za-z0-9_]*[[:space:]]*:/, "", rest)
    keytype[nk] = (rest ~ /^[[:space:]]*(#.*)?$/) ? "H" : "S"
    nm = $0
    sub(/^[[:space:]]*/, "", nm)
    sub(/[[:space:]]*:.*/, "", nm)
    keyname[nk] = nm
    bndidx[nk] = nb
  }
}
END {
  for (k = 1; k <= nk; k++) {
    idx = bndidx[k]
    prevB = (idx > 1) ? bnd[idx-1] : 0
    winStart = prevB + 1
    winEnd = keyline[k]
    own = 0
    for (l = winStart; l <= winEnd; l++) {
      if (content[l] ~ /scope: repo-only \(coordination-fenced, ADR-0019\)/) own = 1
      if (content[l] ~ /scope: any layer/) own = 1
      if (content[l] ~ /scope: local-only/) own = 1
    }
    while (top > 0 && sdepth[top] >= keydepth[k]) top--
    path = keyname[k]
    for (i = top; i >= 1; i--) path = sname[i] "." path
    covered = own
    if (!covered) { for (i = top; i >= 1; i--) if (sown[i]) { covered = 1; break } }
    if (!covered && keytype[k] == "S" && k > 1 && winStart == keyline[k] && prevB == keyline[k-1] && keydepth[k] == keydepth[k-1]) covered = effcov[k-1]
    effcov[k] = covered
    if (keydepth[k] > 0) nested++
    if (keytype[k] == "S" && !covered) print path
    top++; sdepth[top] = keydepth[k]; sname[top] = keyname[k]; sown[top] = own
  }
  print "COUNT " nested
}
```

How it implements the spec's four rules:

1. **Leaves carry the obligation** — the `print path` is gated on `keytype[k] == "S"`. A header is never reported for lacking a tag, but its `own` value is pushed onto the stack as `sown[top]`, so it can still *provide* coverage.
2. **Coverage = own tag, else nearest tagged ancestor** — `covered = own`, then the descending walk over `sown[i]`.
3. **A header's window never extends forward** — `winEnd = keyline[k]`, unconditionally, for headers and scalars alike. This is the single line that deletes the masking behavior; the old program branched here and set `winEnd = nextB - 1` for headers.
4. **Same-depth adjacency inheritance** — the `keydepth[k] == keydepth[k-1]` clause, which is the `changes_dir` / `adrs_dir` / `results_dir` group. Note this adds a depth check the old program did not have.

---

### Task 1: Replace the guard pass, its prose, its label, and its diagnostic

This is one task, not four, because the awk rewrite and the three prose sites assert the *same* proposition — a reviewer cannot sensibly approve a guard whose own comment describes the behavior it just deleted.

**Files:**
- Modify: `tests/test_docket_example_yml.sh:607-669`

**Interfaces:**
- Produces: shell variables `scope_guard_awk` (the hoisted program text), `scope_guard_out` (raw stdout), `nested_key_count` (the integer from the `COUNT` line), and `untagged_keys` (newline-separated dotted paths, empty on success). Tasks 2 and 3 consume `scope_guard_awk` and `nested_key_count`.

- [ ] **Step 1: Confirm the starting state is what this plan assumes**

Run:
```bash
cd /Users/homer/dev/docket/.worktrees/nested-keys-scope-tags-in-docket-example-yml-are-unguarded
/usr/bin/grep -n 'is_active = ($0 ~ /\^\[A-Za-z_\]' tests/test_docket_example_yml.sh
/usr/bin/grep -c 'scope: local-only form present' tests/test_docket_example_yml.sh
```

Expected: the first prints a hit at line **624** (the column-0 anchor still present); the second prints **1** (the `local-only` presence assert already exists — do not add a second one).

- [ ] **Step 2: Replace lines 607-669 with the new block**

Replace the whole region — from the `# Scope tags: both forms present, …` comment through the closing `fi` of the failure block — with the following. The three `assert "scope tag: … form present"` lines are carried over unchanged.

````bash
# Scope tags: all three forms present, and every ACTIVE SCALAR key at EVERY nesting depth is
# covered by a scope tag — a real per-key check, not just "the phrase occurs somewhere in the
# file" (which the three asserts below alone would only prove).
#
# The pass finds each active (uncommented) key's own preceding comment "window", bounded by the
# nearest neighbor above among: a section banner (# ═══...), another active key at ANY depth, or
# a commented pseudo-key (# agent_harnesses: / # agents:). Four rules (change 0122):
#
#   1. A SCALAR key (anything after the colon) must be covered. A HEADER key (a mapping opener
#      like `finalize:`, nothing after the colon) is never itself required to carry a tag — it
#      may PROVIDE one for its subtree, but a container has no scope of its own to assert.
#   2. Coverage = the key's own window carries a sanctioned tag, ELSE the nearest enclosing
#      header block's own window does. Both of the file's conventions are therefore legal:
#      finalize/learnings/reclaim tag each child individually, while auto_capture/runners/skills
#      tag the block header and let the children inherit.
#   3. A header's window is its OWN preceding comment lines ONLY, and is NEVER extended forward
#      into its body. This is the anti-masking rule. The pre-0122 guard did extend it forward,
#      so ANY ONE child's tag satisfied the header and no child was ever checked individually —
#      which is exactly how change 0102 shipped `finalize.require_pr_approval` carrying a
#      bespoke note claiming the OPPOSITE of its real scope, with the suite fully green.
#   4. A scalar key with a genuinely empty window (no comment lines of its own, immediately
#      adjacent to the previous key AT THE SAME DEPTH) inherits that key's coverage — this is
#      the changes_dir / adrs_dir / results_dir group, one shared comment block above all three.
#
# Failures are reported as DOTTED PATHS (`finalize.gate`), because a bare leaf name is ambiguous
# between learnings.enabled and auto_capture.enabled.
#
# The program is HOISTED into $scope_guard_awk rather than written inline, so the mutation
# self-tests below run LITERALLY THIS PROGRAM. A hand-copied second inline copy would be a
# guard that tests a different program than the one that ships (plan-supplied-test-code-is-
# unverified). The heredoc delimiter is single-quoted so awk's $0 is not shell-expanded.
assert "scope tag: repo-only form present"  'grep -qF "scope: repo-only (coordination-fenced, ADR-0019)" "$EX"'
assert "scope tag: any-layer form present"  'grep -qF "scope: any layer" "$EX"'
assert "scope tag: local-only form present" 'grep -qF "scope: local-only" "$EX"'
scope_guard_awk="$(cat <<'SCOPE_GUARD_AWK'
{
  content[NR] = $0
  is_active = ($0 ~ /^[[:space:]]*[A-Za-z_][A-Za-z0-9_]*[[:space:]]*:/)
  is_pseudo = ($0 ~ /^# (agent_harnesses|agents):/)
  is_banner = ($0 ~ /^#[[:space:]]*═══/)
  if (is_active || is_pseudo || is_banner) { nb++; bnd[nb] = NR }
  if (is_active) {
    nk++
    keyline[nk] = NR
    match($0, /^[[:space:]]*/); keydepth[nk] = RLENGTH
    rest = $0
    sub(/^[[:space:]]*[A-Za-z_][A-Za-z0-9_]*[[:space:]]*:/, "", rest)
    keytype[nk] = (rest ~ /^[[:space:]]*(#.*)?$/) ? "H" : "S"
    nm = $0
    sub(/^[[:space:]]*/, "", nm)
    sub(/[[:space:]]*:.*/, "", nm)
    keyname[nk] = nm
    bndidx[nk] = nb
  }
}
END {
  for (k = 1; k <= nk; k++) {
    idx = bndidx[k]
    prevB = (idx > 1) ? bnd[idx-1] : 0
    winStart = prevB + 1
    winEnd = keyline[k]
    own = 0
    for (l = winStart; l <= winEnd; l++) {
      if (content[l] ~ /scope: repo-only \(coordination-fenced, ADR-0019\)/) own = 1
      if (content[l] ~ /scope: any layer/) own = 1
      if (content[l] ~ /scope: local-only/) own = 1
    }
    while (top > 0 && sdepth[top] >= keydepth[k]) top--
    path = keyname[k]
    for (i = top; i >= 1; i--) path = sname[i] "." path
    covered = own
    if (!covered) { for (i = top; i >= 1; i--) if (sown[i]) { covered = 1; break } }
    if (!covered && keytype[k] == "S" && k > 1 && winStart == keyline[k] && prevB == keyline[k-1] && keydepth[k] == keydepth[k-1]) covered = effcov[k-1]
    effcov[k] = covered
    if (keydepth[k] > 0) nested++
    if (keytype[k] == "S" && !covered) print path
    top++; sdepth[top] = keydepth[k]; sname[top] = keyname[k]; sown[top] = own
  }
  print "COUNT " nested
}
SCOPE_GUARD_AWK
)"
# The pass emits two streams on one stdout: uncovered dotted paths, then a trailing COUNT line.
# Split them, so the COUNT line cannot make the emptiness assert unconditionally false.
scope_guard_out="$(awk "$scope_guard_awk" "$EX")"
nested_key_count="$(printf '%s\n' "$scope_guard_out" | sed -n 's/^COUNT //p')"
untagged_keys="$(printf '%s\n' "$scope_guard_out" | grep -v '^COUNT ')"
assert "scope tag: every ACTIVE SCALAR key at every depth is covered by a scope tag" \
  '[ -z "$untagged_keys" ]'
if [ -n "$untagged_keys" ]; then
  echo "--- keys with no scope tag (own or inherited), as dotted paths ---"
  printf '%s\n' "$untagged_keys"
  echo "---"
fi
````

- [ ] **Step 3: Run the suite and confirm the new assert is green on the unmodified file**

Run:
```bash
cd /Users/homer/dev/docket/.worktrees/nested-keys-scope-tags-in-docket-example-yml-are-unguarded
bash tests/test_docket_example_yml.sh 2>&1 | /usr/bin/grep -E 'scope tag:|NOT OK'
```

Expected: four `ok - scope tag: …` lines (three presence + the new coverage assert), and **no `NOT OK` lines anywhere in the suite**. If any `NOT OK` appears, stop and read it — the change is meant to require zero edits to `.docket.example.yml`, so a failure here is a defect in the pass, not a reason to retag the example.

- [ ] **Step 4: Confirm the deleted framing is really gone**

Run:
```bash
cd /Users/homer/dev/docket/.worktrees/nested-keys-scope-tags-in-docket-example-yml-are-unguarded
/usr/bin/grep -n 'top-level key is individually tagged\|untagged top-level keys\|extends its window forward' tests/test_docket_example_yml.sh
```

Expected: **no output** (exit 1). Each of those three strings is one of the now-false sites; a hit means a site was missed.

- [ ] **Step 5: Confirm no GNU-awk dependency crept in**

Run:
```bash
cd /Users/homer/dev/docket/.worktrees/nested-keys-scope-tags-in-docket-example-yml-are-unguarded
/usr/bin/awk "$(sed -n '/^scope_guard_awk=/,/^SCOPE_GUARD_AWK$/p' tests/test_docket_example_yml.sh | sed '1d;$d')" .docket.example.yml
```

Expected: exactly one line, `COUNT 17`. This runs the shipped program text under the system awk specifically.

- [ ] **Step 6: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/nested-keys-scope-tags-in-docket-example-yml-are-unguarded
git add tests/test_docket_example_yml.sh
git commit -m "fix(0122): evaluate scope tags at every depth, not just column 0

The guard anchored on /^[A-Za-z_][A-Za-z0-9_]*:/, so all 17 nested keys were
invisible to it, and a header's comment window extended forward through its
body — so any one child's tag satisfied the header and no child was checked
individually. That is how change 0102 shipped finalize.require_pr_approval
with a note asserting the opposite of its real scope, fully green.

Rules: scalars carry the obligation, headers may provide; coverage is own tag
else nearest tagged ancestor; a header's window never extends forward; same-
depth adjacency inheritance is retained. Failures report as dotted paths.
scope: local-only joins the accepted set. Zero edits to .docket.example.yml."
```

---

### Task 2: Exact-count population floor, emitted by the guard's own pass

Without this, the emptiness assert is satisfiable by a pass that enumerates **zero** nested keys — the `correspondence-guard-runs-one-way` vacuity, and the reason the count must come from the guard's own stdout rather than from any other extractor in the file.

**Files:**
- Modify: `tests/test_docket_example_yml.sh` — immediately after the failure block added in Task 1.

**Interfaces:**
- Consumes: `nested_key_count` from Task 1.

- [ ] **Step 1: Write the failing test**

Append directly below Task 1's `fi`:

```bash
# POPULATION FLOOR — EXACT, and emitted by the guard's OWN pass (change 0122).
# The emptiness assert above is green both when every key is covered AND when the pass enumerated
# nothing at all; only a floor distinguishes those. The count MUST come from $scope_guard_out —
# NOT from example_keys_raw's qualified extractor above, which would keep this green while the
# guard's own pass reached zero nested keys, i.e. exactly the vacuity this assert exists to catch.
#
# EXACT, not >=. An at-least floor of 15 is satisfied by the PRE-0102 file and would tolerate a
# regression that silently drops both runners.codex leaves. The 17: 3 finalize.*, 2 learnings.*,
# 2 reclaim.*, 2 auto_capture.*, runners.codex + its 2 leaves, 5 skills.*.
expected_nested_key_count=17
assert "scope tag: the pass enumerated exactly $expected_nested_key_count keys at depth > 0 (got ${nested_key_count:-0}; if you added or removed a nested key in .docket.example.yml, bump expected_nested_key_count in the same commit)" \
  '[ "${nested_key_count:-0}" = "$expected_nested_key_count" ]'
```

- [ ] **Step 2: Run it and verify it passes on the real file**

Run:
```bash
cd /Users/homer/dev/docket/.worktrees/nested-keys-scope-tags-in-docket-example-yml-are-unguarded
bash tests/test_docket_example_yml.sh 2>&1 | /usr/bin/grep 'enumerated exactly'
```

Expected: `ok - scope tag: the pass enumerated exactly 17 keys at depth > 0 (got 17; …)`

- [ ] **Step 3: Mutation-test the floor — prove it can go red**

Temporarily set `expected_nested_key_count=16`, re-run, confirm `NOT OK`, then set it back to `17` and confirm `ok` again.

Run:
```bash
cd /Users/homer/dev/docket/.worktrees/nested-keys-scope-tags-in-docket-example-yml-are-unguarded
sed -i.bak 's/^expected_nested_key_count=17$/expected_nested_key_count=16/' tests/test_docket_example_yml.sh
bash tests/test_docket_example_yml.sh 2>&1 | /usr/bin/grep 'enumerated exactly'
mv tests/test_docket_example_yml.sh.bak tests/test_docket_example_yml.sh
bash tests/test_docket_example_yml.sh 2>&1 | /usr/bin/grep 'enumerated exactly'
```

Expected: first invocation prints a `NOT OK - …` line; second prints `ok - …`. If the first is green, the assert is decoration — stop and fix it.

- [ ] **Step 4: Verify the floor is genuinely wired to the guard's own output**

Confirm by inspection that the assert reads `nested_key_count`, and that `nested_key_count` is derived from `scope_guard_out` and from nothing else.

Run:
```bash
cd /Users/homer/dev/docket/.worktrees/nested-keys-scope-tags-in-docket-example-yml-are-unguarded
/usr/bin/grep -n 'nested_key_count' tests/test_docket_example_yml.sh
```

Expected: exactly three hits — the assignment from `scope_guard_out`, the `expected_nested_key_count=17` line, and the assert. No hit may reference `example_keys_raw` or `example_keys`.

- [ ] **Step 5: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/nested-keys-scope-tags-in-docket-example-yml-are-unguarded
git add tests/test_docket_example_yml.sh
git commit -m "test(0122): exact 17-key nested population floor from the guard's own pass

The emptiness assert is equally green when every key is covered and when the
pass enumerated nothing. The count is read off the guard's own COUNT line, not
from the qualified extractor — reusing that would keep the floor green while
the guard itself reached zero nested keys. Exact rather than >=: a >= 15 floor
is satisfied by the pre-0102 file and tolerates dropping both runners.codex
leaves."
```

---

### Task 3: Guard-the-guard mutation self-tests

**Files:**
- Modify: `tests/test_docket_example_yml.sh` — immediately after Task 2's floor.

**Interfaces:**
- Consumes: `scope_guard_awk` from Task 1, and `$tmp` (the suite's existing `mktemp -d`, created at line 13 with an `EXIT` trap already in place — do not create a second temp dir or a second trap).

- [ ] **Step 1: Write the self-tests**

Append below Task 2's assert. Both deletions are anchored **by content**, never by line number, because the floor above explicitly anticipates the file gaining keys.

```bash
# GUARD-THE-GUARD (change 0122). The asserts above are green on a correct file; these prove the
# pass actually goes RED on the drift it exists to catch. Both run $scope_guard_awk — literally
# the program that ships — over a MUTATED COPY in $tmp. The real .docket.example.yml is never
# touched. Deletions are anchored on the KEY LINE'S CONTENT, not on a line number, because the
# population floor above explicitly anticipates this file gaining keys.
#
# drop_tag_above <file> <key-line-regex> — deletes the `scope:` comment line sitting immediately
# above the first line matching the regex. Emits the mutated file on stdout.
drop_tag_above(){
  awk -v pat="$2" '
    { b[NR] = $0 }
    END {
      for (i = 1; i <= NR; i++) {
        if (b[i+1] ~ pat && b[i] ~ /scope:/) continue
        print b[i]
      }
    }
  ' "$1"
}

# (a) A key that carries its OWN tag: finalize.gate. Under the PRE-0122 guard this mutation was
# green — the finalize: header's window extended forward and its two siblings' tags satisfied it.
drop_tag_above "$EX" '^  gate:' > "$tmp/mut-gate.yml"
mut_gate_out="$(awk "$scope_guard_awk" "$tmp/mut-gate.yml" | grep -v '^COUNT ')"
assert "guard-the-guard: dropping finalize.gate's own tag is REPORTED (got '${mut_gate_out}')" \
  '[ "$mut_gate_out" = "finalize.gate" ]'

# (b) A block whose children INHERIT: skills. Dropping the header's tag must report all five
# leaves, since none carries a tag of its own — this is the inheritance half of rule 2.
drop_tag_above "$EX" '^skills:' > "$tmp/mut-skills.yml"
mut_skills_out="$(awk "$scope_guard_awk" "$tmp/mut-skills.yml" | grep -v '^COUNT ' | sort | tr '\n' ' ')"
assert "guard-the-guard: dropping the skills: header tag reports all five leaves (got '${mut_skills_out}')" \
  '[ "$mut_skills_out" = "skills.brainstorm skills.build skills.finish skills.plan skills.review " ]'

# (c) THE ANTI-MASKING REGRESSION, reproduced. This is change 0102's exact bug: a finalize child
# whose window holds no sanctioned tag, while its two siblings remain tagged. The pre-0122 guard
# was GREEN here — which is how the bug shipped. Rule 3 is the only reason this is now red.
drop_tag_above "$EX" '^  require_pr_approval:' > "$tmp/mut-0102.yml"
mut_0102_out="$(awk "$scope_guard_awk" "$tmp/mut-0102.yml" | grep -v '^COUNT ')"
assert "guard-the-guard: the 0102 regression (an untagged finalize sibling) is REPORTED (got '${mut_0102_out}')" \
  '[ "$mut_0102_out" = "finalize.require_pr_approval" ]'

# Non-vacuity for the mutations themselves: a drop_tag_above that silently matched nothing would
# leave the copy identical to the original, and all three asserts would then be comparing the
# guard's clean (empty) output against a non-empty expectation — i.e. they'd fail loudly rather
# than pass falsely. But an inverted bug (the helper deleting too much) is silent, so pin the
# damage: each mutated copy must be EXACTLY one line shorter than the original.
for mf in mut-gate mut-skills mut-0102; do
  assert "guard-the-guard: $mf.yml differs from the original by exactly one deleted line" \
    '[ "$(( $(wc -l < "$EX") - $(wc -l < "$tmp/'"$mf"'.yml") ))" = "1" ]'
done
```

- [ ] **Step 2: Run and verify all self-tests pass**

Run:
```bash
cd /Users/homer/dev/docket/.worktrees/nested-keys-scope-tags-in-docket-example-yml-are-unguarded
bash tests/test_docket_example_yml.sh 2>&1 | /usr/bin/grep 'guard-the-guard'
```

Expected: six `ok - guard-the-guard: …` lines, no `NOT OK`. The three observed outputs are `finalize.gate`, the five sorted `skills.*` paths, and `finalize.require_pr_approval` — these are measured values, so a mismatch means the pass changed, not that the expectation is wrong.

- [ ] **Step 3: Confirm the real example file was not modified**

Run:
```bash
cd /Users/homer/dev/docket/.worktrees/nested-keys-scope-tags-in-docket-example-yml-are-unguarded
git status --porcelain .docket.example.yml
```

Expected: **no output.** Any output means a self-test wrote to the real file — a hard stop.

- [ ] **Step 4: Full suite, under the system tools**

Run the whole suite, plus a re-run with the system awk forced ahead of any PATH awk, to satisfy the portability constraint.

```bash
cd /Users/homer/dev/docket/.worktrees/nested-keys-scope-tags-in-docket-example-yml-are-unguarded
bash tests/test_docket_example_yml.sh 2>&1 | tail -20
echo "=== exit: $?"
bash tests/test_docket_example_yml.sh 2>&1 | /usr/bin/grep -c '^NOT OK'
```

Expected: the suite's own summary reports zero failures, and the `NOT OK` count is **0**.

- [ ] **Step 5: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/nested-keys-scope-tags-in-docket-example-yml-are-unguarded
git add tests/test_docket_example_yml.sh
git commit -m "test(0122): mutation self-tests over the hoisted guard program

Three mutations on \$tmp copies, run through \$scope_guard_awk so the self-test
exercises literally the shipping program: an own-tag key (finalize.gate), an
inheriting block (skills, all five leaves), and change 0102's exact regression
(an untagged finalize sibling), which the pre-0122 guard passed green. Anchored
by content, not line number. Plus a one-line-delta assert so a helper that
deletes too much cannot pass silently."
```

---

### Task 4: Full-suite regression sweep

The guard shares a file with the classification manifest and the `(2c)` orphan check; this task proves nothing else moved.

**Files:** none modified — verification only.

- [ ] **Step 1: Run the full repo suite**

This repo has **no aggregate runner** — no `Makefile`, no `tests/run-all.sh`. The suite is the set of `tests/test_*.sh` files, each run directly. Enumerate them under an explicit `bash` (never the harness's interactive shell, which is zsh and does not word-split an unquoted list the way bash does — a sweep that iterates zero times still prints its success line):

```bash
cd /Users/homer/dev/docket/.worktrees/nested-keys-scope-tags-in-docket-example-yml-are-unguarded
bash -c 'n=0; f=0; for t in tests/test_*.sh; do n=$((n+1)); if bash "$t" >/dev/null 2>&1; then :; else f=$((f+1)); echo "FAIL $t"; fi; done; echo "ran=$n failed=$f"'
```

Expected: `ran=` a count in the fifties (the current `tests/` population) and **`failed=0`**, with no `FAIL` lines. The `ran=` count is what makes this non-vacuous — a glob that matched nothing would otherwise print `ran=0 failed=0` and read as success.

This is a long run. Execute it in **one foreground call** with a generous timeout; do not background it.

- [ ] **Step 2: Confirm change 0102's two asserts survived**

Run:
```bash
cd /Users/homer/dev/docket/.worktrees/nested-keys-scope-tags-in-docket-example-yml-are-unguarded
bash tests/test_docket_example_yml.sh 2>&1 | /usr/bin/grep '0102:'
```

Expected: four `ok - 0102: …` lines, including `require_pr_approval carries the any-layer scope tag` and `the stale repo-committed-only note is gone`. Neither may be missing — the spec's assumption 5 keeps both.

- [ ] **Step 3: Confirm the diff is confined to one file**

Run:
```bash
cd /Users/homer/dev/docket/.worktrees/nested-keys-scope-tags-in-docket-example-yml-are-unguarded
git diff --stat origin/main -- . ':!docs/superpowers/plans'
```

Expected: exactly one file, `tests/test_docket_example_yml.sh`.

---

## Self-Review

**Spec coverage.** Every bullet in the spec's *What gets built* maps to a task: the awk rewrite implementing rules 1-4 → Task 1 Step 2; window boundaries generalized to any depth → the `is_active` regex in the hoisted program; `scope: local-only` in the accepted set → the third `own` test (the *presence* assert was already on `main`, recorded in Task 1 Step 1 so the implementer does not add a duplicate); the exact-count floor from the guard's own stdout → Task 2; the hoisted-program mutation self-tests → Task 3; the prose, label, and diagnostic rewrite → Task 1 Steps 2 and 4; POSIX-awk portability → Global Constraints plus Task 1 Step 5. Assumption 5's "keep both 0102 asserts" → Task 4 Step 2.

**One deliberate addition beyond the spec.** The spec names two mutation self-tests; the plan ships three. The third — dropping `require_pr_approval`'s tag — is the spec's own stated success criterion ("the one that makes the 0102 regression RED") and costs three lines. It is the only self-test that directly exercises rule 3, which the other two do not: `finalize.gate`'s mutation is caught by rule 1 as well. Verified red before this plan was written.

**Placeholder scan.** No TBDs. Every code step carries literal content; every verification step carries a runnable command and its expected output. All three expected mutation outputs are observed values from running the program against the real file, not predictions.

**Type consistency.** `scope_guard_awk`, `scope_guard_out`, `nested_key_count`, `untagged_keys`, `expected_nested_key_count`, and `drop_tag_above` are spelled identically at every definition and use site across Tasks 1-3.

**Known-fragile point, flagged for the implementer.** Task 3's `drop_tag_above` regexes (`^  gate:`, `^skills:`, `^  require_pr_approval:`) assume two-space indentation in `.docket.example.yml`, which is the file's current and consistent style. If a future reindent breaks them, the one-line-delta assert in Task 3 Step 1 reddens rather than failing open — that is the intended failure direction.
