<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0258 — Guard the config-suite's enumerated claims: export order and rung pairs](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-09-0258-guard-the-config-suite-s-enumerated-claims-export-order-and.md)**
<!-- docket:backlink:end -->

# Guard the config-suite's enumerated claims: export order and rung pairs — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace two prose-only enumerated claims in the config surface with computed guards — the doc's export-order fence pinned by whole-sequence equality against the resolver's real emission, and the "six ordered rung pairs" claim pinned by set equality against a pair set derived from the resolver's own layer dispatch.

**Architecture:** Both guards land as new sections appended to `tests/test_docket_config.sh`, after the existing `(GOB-g)` section and before the closing `0174 template integrity` assert. Each derives its **expected** side from the real artifact (the resolver's `--export` output for leg 1; `config_scalar_get`'s case arms for leg 2) and never from a second hand-maintained list. Leg 2 additionally converts the `(S4-S9)` header comment's prose enumeration into six machine-readable per-fixture marker lines, which become the claim of record.

**Tech Stack:** Bash 4.4+, POSIX `awk`/`sed`/`grep` (BSD-compatible — no GNU/gawk extensions), the suite's existing `mkrepo`/`rung`/`assert` fixture helpers, `scripts/run-tests.sh` as the gate.

## Global Constraints

Copied verbatim from the spec and `AGENTS.md`; every task's requirements implicitly include these.

- **Corpus-indifferent to #0251's split.** No `${BASH_SOURCE[0]}` whole-file scans. Anything that must scan test source iterates the family glob `tests/test_docket_config*.sh`.
- **ADR-0054 anchoring.** Cross-references anchor on a **symbol name** or a **verbatim-quoted clause**, never a line number. `tests/test_comment_anchor_style.sh` enforces the filename-plus-line-number form.
- **Never `producer | early-exiting-consumer`** (`grep -q`, `head`, `head -n1`) under `set -o pipefail` — capture into a variable first, then `grep <<<"$var"`.
- **`grep` for a pattern that leads with `--` must declare it**: `grep -qF -- "<pat>"`.
- **awk indent classes are `[^[:space:]]`, never `[^ ]`.**
- **BSD awk only.** No 3-argument `match(str, re, arr)`, no `gensub`, no `asort` — those are gawk-only and the suite must pass on macOS.
- **A guard is code: mutation-test it** — strip the thing it guards, watch it redden — or it is decoration. Mutation-test the **population**, not only the suppression.
- **Key a guard on syntactic shape**, never an enumerated list of spellings.
- **The existing R7 and AUTO_* adjacency asserts stay** (spec assumption 7). Do not delete or consolidate them.
- **Out of scope:** adding config layers or keys; changing emission order; #0251's population-floor/sharding rework.
- **No new test file.** Everything lands in `tests/test_docket_config.sh`, so `tests/runtime-budgets.tsv` needs no new row and `EXPECTED_TOTAL` must not move. Keep the added runtime small — leg 1 costs one `mkrepo` plus two resolver runs; leg 2 costs zero fixtures.
- **Suite command:** whatever `finalize.test_command` resolves to — today `scripts/run-tests.sh`. Read it there, never from a second copy.

---

## Amendment (2026-08-09) — the focused proof harness, and the state Task 1 resumes from

Two corrections to the tasks below, made after two runs halted inside Task 1. **They change no
assert, no guard, and no design** — only how the mutation proofs are executed and what Task 1
starts from.

**1. Use a focused proof harness for every mutation proof.** `tests/test_docket_config.sh` is a
single monolithic script, so `bash tests/test_docket_config.sh` re-runs all ~490 asserts —
**measured at 68 s per run**. Task 1 as originally written serialises seven such runs and Task 2
six, which is what exhausted the dispatch window twice. Every mutation proof below therefore runs
against a throwaway harness built from the file's helper preamble plus the change-0258 sections —
**measured at 1 s**, with the eight `0258 L1` assert lines byte-identical to the full run.

Build it, from the worktree root, and **rebuild it after any edit to the change-0258 blocks in**
`tests/test_docket_config.sh`:

```bash
{ sed -n '1,116p' tests/test_docket_config.sh
  sed -n '/^# Change 0258 leg 1 /,$p' tests/test_docket_config.sh | sed -n '/^assert "0174 template integrity/q;p'
  echo 'exit $fail'
} > tests/.focus-0258.sh
bash tests/.focus-0258.sh 2>&1 | grep -E '0258|NOT OK'
```

Lines 1–116 are the preamble through `rung()` — `assert`, `mkrepo`, `ensure_test_runtime`, `rung`,
`$REPO`, `$tmp` — which is everything both legs consume. The harness must live **inside
`tests/`** (`$REPO` derives from `${BASH_SOURCE[0]}`) and must **not** be named
`test_docket_config*.sh`, or it would join leg 2's family glob and double every marker. The
leading `.` keeps it out of the suite runner's own glob.

Constraints on its use:
- The harness is a **proof accelerator, not the gate**. Each task still ends with **one** full
  `bash tests/test_docket_config.sh` run before its commit, and Task 3's `scripts/run-tests.sh`
  gate is unchanged.
- Leg-2 proofs need no rebuild between mutations: the marker collection and the layer derivation
  both read files from disk, so mutating `tests/test_docket_config.sh` or
  `scripts/docket-config.sh` is observed by the harness as-is.
- **Delete `tests/.focus-0258.sh` before every clean-check and commit.** It is untracked, so it
  will show in `git status --porcelain` and must never be committed.

**2. Task 1 resumes from work already in the tree.** A prior run left
`tests/test_docket_config.sh` **modified but uncommitted** with the Task 1 Step 1 block already
applied verbatim — verified green (eight `0258 L1` asserts, zero `NOT OK`) against the full suite
on 2026-08-09. **Adopt that diff; do not rewrite it.** Confirm it first with
`git diff -- tests/test_docket_config.sh` and check it matches Step 1's block. `scripts/docket-config.md`
and `scripts/docket-config.sh` are clean — every earlier mutation was restored.

**3. Task 1 Step 2's expected count is wrong.** It says "nine `ok - 0258 L1 …` lines"; the Step 1
block contains exactly **eight** asserts, and eight is the correct expectation. Corrected in place
below.

---

## File Structure

| File | Change | Responsibility |
|---|---|---|
| `tests/test_docket_config.sh` | Modify — append two sections after `(GOB-g)`, before the `0174 template integrity` assert | Leg 1: `emit_fence_tokens` extractor + doc-vs-emission sequence equality + prose-numeral check. Leg 2: layer derivation from `config_scalar_get` + expected-pair computation + marker collection + set equality. |
| `tests/test_docket_config.sh` | Modify — the `(S4-S9)` block | Add one `# RUNG_PAIR:` marker line to each of the six fixtures s4–s9; reword the section header comment from *stating* the enumeration to *citing* the guard. |
| `scripts/docket-config.md` | **No content change.** The guard pins what the doc already says. | — |

Both new sections are appended to the same file and share nothing, so they could be two files' worth of work; they are two tasks because a reviewer can reject one while approving the other.

---

## Task 1: Leg 1 — doc fence vs emission, whole-sequence equality

**Files:**
- Modify: `tests/test_docket_config.sh` — append a new section immediately after the `(GOB-g)` section's last assert (`"GOB-g: emitted between REVIEW_MAX_FIX_TASKS and SKILL_BRAINSTORM"`) and before the `"0174 template integrity: the shared template is unmutated after the full run"` assert.
- Read-only reference: `scripts/docket-config.md` (the `### Emit` section), `scripts/docket-config.sh`.

**Interfaces:**
- Consumes: the suite's existing file-scope helpers — `assert <label> <eval-string>`, `mkrepo <dir>`, `rung <xdg-dir> <repo-dir> [args...]`, and the `$REPO` / `$tmp` variables. All are already defined near the top of the file.
- Produces: a shell function `emit_fence_tokens()` (no arguments; prints one key token per line to stdout) and the variables `doc_plain_keys`, `doc_shell_keys`, `emit_plain_keys`, `emit_shell_keys`, `l1_plain_n`, `l1_shell_n`, `l1_sentence`. Task 2 uses none of these; the names are listed so Task 2 does not collide with them.

**Why this shape (from the spec, do not re-derive):** the doc's `### Emit` section sells sequence as contract ("printed as `KEY=value` lines to stdout in this order"), and `(R7)`'s own comment cites that promise as its rationale — but the 34-entry fence was pinned only by per-key *presence* greps plus two adjacency clusters, so a doc-side reorder stayed green. One string compare is inherently two-way: a reorder, addition, removal, or count-stable rename on **either** side reddens.

- [ ] **Step 1: Write the failing guard**

Append this block to `tests/test_docket_config.sh` at the location named under **Files**. Note the comment block deliberately never begins a line with `# RUNG_PAIR:` — that is Task 2's marker literal and a stray one here would poison Task 2's collection.

```bash
# ============================================================================
# Change 0258 leg 1 — the doc's export fence vs the resolver's real emission
# ============================================================================
# scripts/docket-config.md's `### Emit` section sells SEQUENCE as contract ("printed as
# `KEY=value` lines to stdout in this order"), and (R7) above cites that promise as the reason
# its own adjacency assert exists. Until this guard the fence itself was pinned only by
# per-key PRESENCE greps plus two adjacency clusters (R7; the AUTO_GROOM -> CHANGE_TYPES ->
# AUTO_CAPTURE_* identity cluster), so a doc-side reorder stayed green. Those stay: they are
# their own changes' mutation witnesses on their own fixtures.
#
# The verdict here is whole-sequence equality, not membership. One string compare is
# inherently two-way -- a reorder, an addition, a removal, or a count-stable rename on EITHER
# side reddens.
#
# The doc side anchors on the `### Emit` heading and the first fenced block after it (ADR-0054:
# a quoted-clause anchor, never a line number), reducing each fence line to its first
# whitespace-delimited token so the `REPO_ROOT ... (plain format only -- see below)` annotation
# is stripped. An anchor that silently stops matching yields an EMPTY extraction, which would
# compare green against nothing -- so the control asserts pin the population first.
emit_fence_tokens(){  # first fenced block after the `### Emit` heading; first token per line
  awk '
    /^### Emit[[:space:]]*$/ { seen = 1; next }
    seen && /^```/           { if (infence) exit; infence = 1; next }
    infence && NF            { print $1 }
  ' "$REPO/scripts/docket-config.md"
}
doc_plain_keys="$(emit_fence_tokens)"
doc_shell_keys="$(grep -v '^REPO_ROOT$' <<<"$doc_plain_keys")"

assert "0258 L1 control: the \`### Emit\` fence extraction is non-empty" \
  '[ -n "$doc_plain_keys" ]'
assert "0258 L1 control: the extracted fence contains DOCKET_MODE" \
  'grep -qx DOCKET_MODE <<<"$doc_plain_keys"'
assert "0258 L1 control: the extracted fence contains BOOTSTRAP" \
  'grep -qx BOOTSTRAP <<<"$doc_plain_keys"'
assert "0258 L1 control: dropping REPO_ROOT shortened the shell sequence by exactly one" \
  '[ "$(grep -c . <<<"$doc_plain_keys")" -eq "$(( $(grep -c . <<<"$doc_shell_keys") + 1 ))" ]'

mkrepo "$tmp/l1"
mkdir -p "$tmp/l1.xdg/docket"
cat > "$tmp/l1/.docket.yml" <<'EOF'
metadata_branch: main
integration_branch: main
EOF
git -C "$tmp/l1" add .docket.yml; git -C "$tmp/l1" commit --quiet -m cfg
git -C "$tmp/l1" push --quiet origin main
emit_plain_keys="$(rung "$tmp/l1.xdg" "$tmp/l1" --export --format plain | cut -d= -f1)"
emit_shell_keys="$(rung "$tmp/l1.xdg" "$tmp/l1" --export | cut -d= -f1)"

assert "0258 L1 control: the resolver emitted a non-empty key sequence" \
  '[ -n "$emit_plain_keys" ] && [ -n "$emit_shell_keys" ]'
assert "0258 L1: plain emission order equals the doc fence, in order and entry for entry" \
  '[ "$emit_plain_keys" = "$doc_plain_keys" ]'
assert "0258 L1: shell emission order equals the doc fence minus REPO_ROOT" \
  '[ "$emit_shell_keys" = "$doc_shell_keys" ]'

# The doc states the two counts in prose as well; derive them from the same extraction so
# growing the fence forces the numerals to move with it.
l1_plain_n="$(grep -c . <<<"$doc_plain_keys")"
l1_shell_n="$(grep -c . <<<"$doc_shell_keys")"
l1_sentence="$l1_shell_n lines in \`shell\` format; $l1_plain_n in \`plain\`"
assert "0258 L1: the doc's line-count prose tracks the fence ($l1_shell_n/$l1_plain_n)" \
  'grep -qF -- "$l1_sentence" "$REPO/scripts/docket-config.md"'
```

- [ ] **Step 2: Run it and confirm every new assert is GREEN against unmodified sources**

This guard pins a claim that is **true today**, so the first run must pass. A red run here means the extraction is broken, not that the doc is wrong — debug the extractor, do not edit the doc.

Run the **full** file once here — this is the baseline confirmation, so it is worth the 68 s — then
build the focused harness per the Amendment and confirm it reproduces the same eight lines.

```bash
cd /Users/homer/dev/docket/.worktrees/guard-the-config-suite-s-enumerated-claims-export-order-and
bash tests/test_docket_config.sh 2>&1 | grep -E '0258 L1|NOT OK'
```
Expected: **eight** `ok - 0258 L1 …` lines, no `NOT OK`. (The Step 1 block contains eight asserts;
this step originally said nine — see Amendment item 3.) Every later proof in this task runs against
the harness instead.

- [ ] **Step 3: Mutation-prove it — doc-side reorder**

Swap two adjacent entries in the `### Emit` fence of `scripts/docket-config.md` (e.g. `ADRS_DIR` and `RESULTS_DIR`), run the guard, then restore.

```bash
cp scripts/docket-config.md /tmp/dc-md.bak
perl -0pi -e 's/^ADRS_DIR\nRESULTS_DIR$/RESULTS_DIR\nADRS_DIR/m' scripts/docket-config.md
bash tests/.focus-0258.sh 2>&1 | grep '0258 L1'   # focused harness — see the Amendment
cp -f /tmp/dc-md.bak scripts/docket-config.md
```
Expected while mutated: `NOT OK - 0258 L1: plain emission order equals the doc fence…` **and** the shell-format assert. Record both lines.

- [ ] **Step 4: Mutation-prove it — doc-side deletion, and the prose numerals**

Delete one fence entry, run, restore.

```bash
cp scripts/docket-config.md /tmp/dc-md.bak
perl -0pi -e 's/^LEARNINGS_CAP\n//m' scripts/docket-config.md
bash tests/.focus-0258.sh 2>&1 | grep '0258 L1'   # focused harness — see the Amendment
cp -f /tmp/dc-md.bak scripts/docket-config.md
```
Expected while mutated: both sequence asserts red **and** `NOT OK - 0258 L1: the doc's line-count prose tracks the fence (32/33)` — the numerals derive from the extraction, so a shortened fence makes the doc's own sentence stale.

Then prove the numeral leg alone: with the fence untouched, edit only the prose sentence's `33` to `32`, run, restore.

```bash
cp scripts/docket-config.md /tmp/dc-md.bak
perl -pi -e 's/^33 lines in `shell` format; 34 in `plain`/32 lines in `shell` format; 34 in `plain`/' scripts/docket-config.md
bash tests/.focus-0258.sh 2>&1 | grep '0258 L1'   # focused harness — see the Amendment
cp -f /tmp/dc-md.bak scripts/docket-config.md
```
Expected while mutated: only the prose assert reddens; both sequence asserts stay green.

- [ ] **Step 5: Mutation-prove it — count-stable rename, and a runtime-side change**

Rename one fence entry without changing the count, run, restore.

```bash
cp scripts/docket-config.md /tmp/dc-md.bak
perl -0pi -e 's/^BOARD_SURFACES$/BOARD_SURFACE/m' scripts/docket-config.md
bash tests/.focus-0258.sh 2>&1 | grep '0258 L1'   # focused harness — see the Amendment
cp -f /tmp/dc-md.bak scripts/docket-config.md
```
Expected while mutated: both sequence asserts red, prose assert green (count unchanged) — this is the mutation that the pre-existing presence greps and the count-only `(E')` guard both survive, so it is the one that proves the new guard earns its place.

Then the runtime side: comment out one `emit` call in `scripts/docket-config.sh` (find it with `grep -n 'LEARNINGS_CAP' scripts/docket-config.sh` and comment the emit line, not the resolution line), run, restore.

```bash
cp scripts/docket-config.sh /tmp/dc-sh.bak
# comment out the single line that emits LEARNINGS_CAP
bash tests/.focus-0258.sh 2>&1 | grep '0258 L1'   # focused harness — see the Amendment
cp -f /tmp/dc-sh.bak scripts/docket-config.sh
```
Expected while mutated: both sequence asserts red (other pre-existing asserts will also redden — that is fine; confirm the **new** guard fires).

- [ ] **Step 6: Mutation-prove the anti-vacuity control — a broken anchor must redden, not empty-compare green**

Rename the heading the extractor anchors on, run, restore.

```bash
cp scripts/docket-config.md /tmp/dc-md.bak
perl -pi -e 's/^### Emit$/### Emitted values/' scripts/docket-config.md
bash tests/.focus-0258.sh 2>&1 | grep '0258 L1'   # focused harness — see the Amendment
cp -f /tmp/dc-md.bak scripts/docket-config.md
```
Expected while mutated: `NOT OK - 0258 L1 control: the \`### Emit\` fence extraction is non-empty` plus the two sentinel controls — the guard fails loudly instead of silently comparing an empty doc side against an empty expectation.

- [ ] **Step 7: Verify the file is byte-restored, then commit**

Delete the focused harness first, then re-run the **full** file once so the commit is gated on the
real suite file and not on the accelerator:

```bash
cd /Users/homer/dev/docket/.worktrees/guard-the-config-suite-s-enumerated-claims-export-order-and
rm -f tests/.focus-0258.sh
bash tests/test_docket_config.sh 2>&1 | grep -E '0258 L1|NOT OK'
git status --porcelain
```
Expected: eight `ok - 0258 L1 …` lines, no `NOT OK`; and ` M tests/test_docket_config.sh` as the
only entry. If `scripts/docket-config.md` or `scripts/docket-config.sh` show as modified, restore them with `git checkout --` before committing. If `tests/.focus-0258.sh` still shows, delete it — it is never committed.

```bash
git add tests/test_docket_config.sh
git commit -m "test(0258): pin the export fence's whole sequence against real emission

The doc's \`### Emit\` list sold order as contract but was pinned only by
per-key presence greps and two adjacency clusters. Compare the extracted
fence to the resolver's emitted key sequence in both formats, and derive
the doc's own line-count numerals from the same extraction."
```

---

## Task 2: Leg 2 — rung-pair completeness computed from the resolver

**Files:**
- Modify: `tests/test_docket_config.sh` — the `(S4-S9)` section (header comment plus one added marker line per fixture s4–s9), and a new section appended after Task 1's block, still before the `0174 template integrity` assert.
- Read-only reference: `scripts/docket-config.sh` (the `config_scalar_get` function).

**Interfaces:**
- Consumes: `assert <label> <eval-string>` and `$REPO`, as in Task 1. Does **not** consume anything Task 1 produced — no fixtures, no variables.
- Produces: the variables `rp_layers`, `rp_n`, `rp_expected`, `rp_pinned`, and the loop variable `rp_l`. No functions.

**Why this shape (from the spec, do not re-derive):** section `(S4-S9)` pins all six ordered rung pairs of the three-layer `finalize.test_command` chain, but the "six pairs" enumeration lives only in the header comment. `config_scalar_get` is the single choke point every layer read funnels through (`lcl`/`gbl` are one-line wrappers; the committed read calls it directly), so a fourth layer cannot land without adding an arm — deriving from those arms means the expected pair count grows from 6 to 12 on its own and the guard reddens until six new fixtures exist.

- [ ] **Step 1: Add the six marker lines to the `(S4-S9)` fixtures**

In `tests/test_docket_config.sh`, insert one marker line immediately below each fixture's own descriptive comment and immediately above its `mkrepo` call. The pairs are written as *(rung holding `auto`) → (rung holding the real command)* and are read off the existing assert labels — do not invent them:

| Fixture | Existing assert label | Marker line to insert |
|---|---|---|
| s4 | `0106 s4: local auto masks committed real command` | `# RUNG_PAIR: local->committed` |
| s5 | `0106 s5: committed auto masks global real command` | `# RUNG_PAIR: committed->global` |
| s6 | `0106 s6: global auto does NOT wipe committed real command` | `# RUNG_PAIR: global->committed` |
| s7 | `0112 s7: committed auto does NOT wipe local real command` | `# RUNG_PAIR: committed->local` |
| s8 | `0112 s8: global auto does NOT wipe local real command (committed key absent)` | `# RUNG_PAIR: global->local` |
| s9 | `0112 s9: local auto masks global real command (committed key absent)` | `# RUNG_PAIR: local->global` |

For example, s4 becomes:

```bash
# (s4) FORWARD, lcl() path: .docket.local.yml `auto` over a committed real command.
# RUNG_PAIR: local->committed
mkrepo "$tmp/s4"
```

Exactly six such lines may exist in the whole `tests/test_docket_config*.sh` family. Verify:

```bash
grep -hcE '^# RUNG_PAIR: ' tests/test_docket_config*.sh
```
Expected: `6`.

- [ ] **Step 2: Reword the `(S4-S9)` header comment so the markers, not the prose, are the claim of record**

Replace these four lines of the section header —

```
# 0106 pinned three of the six ordered rung pairs; 0112 completes the matrix with s7/s8/s9.
# Writing each pair as (rung holding `auto` -> rung holding the real command):
#   forward (higher `auto` masks lower real):  local->committed s4 | committed->global s5 | local->global s9
#   reverse (lower `auto` must NOT wipe higher real): global->committed s6 | committed->local s7 | global->local s8
```

— with:

```
# 0106 pinned three of the ordered rung pairs; 0112 completed the matrix with s7/s8/s9.
# Each fixture below carries a machine-readable marker line naming its pair as
# (rung holding `auto` -> rung holding the real command). Change 0258's leg-2 guard, near the
# end of this file, computes the expected ordered-pair set from `config_scalar_get`'s layer
# dispatch and asserts set equality against those markers -- so the markers, not this comment,
# are the claim of record, and a FOURTH config layer grows the expected set from 6 pairs to 12
# and reddens the guard until six new fixtures exist.
# The forward cases (a higher `auto` masks a lower real command) are s4, s5 and s9; the reverse
# cases (a lower `auto` must NOT wipe a higher real command) are s6, s7 and s8.
```

Leave the rest of the header — the collapse-placement narrative and the s7-discriminating-power paragraph — byte-untouched.

- [ ] **Step 3: Write the guard**

Append this block to `tests/test_docket_config.sh` after Task 1's leg-1 block, still before the `0174 template integrity` assert. **No line in this block may begin with `# RUNG_PAIR:`** — it would register as a seventh marker and redden the guard against itself.

```bash
# ============================================================================
# Change 0258 leg 2 — rung-pair completeness, computed from the resolver
# ============================================================================
# Section (S4-S9) above pins the ordered rung pairs of the three-layer finalize.test_command
# chain. Until this guard the "six pairs" claim lived only in that section's header comment,
# so a FOURTH config layer would take the ordered-pair count from 6 to 12 and leave six
# masking cells silently unpinned with nothing to say so.
#
# The EXPECTED side is derived from `config_scalar_get`'s layer dispatch in
# scripts/docket-config.sh -- the single choke point every layer read funnels through, since
# `lcl`/`gbl` are one-line wrappers over it and the committed read calls it directly, so a
# fourth layer cannot land without adding an arm. The `*)` die arm is excluded by the
# lowercase-name shape of the match.
#
# The PINNED side is declared by the per-fixture marker lines added to s4-s9, collected across
# the `tests/test_docket_config*.sh` family glob -- never a ${BASH_SOURCE[0]} whole-file scan,
# so change 0251's split of this file cannot blind the collection.
#
# The verdict is SET equality: a gap, a duplicate, and an unknown layer name all redden, and
# count equality falls out of it (no hand-written "6", no ">= 6" floor).
#
# Accepted residual (spec, Design): a marker line could outlive deletion of its fixture body.
# The marker sits inside the fixture block so the natural edit removes both; a lying orphan is
# the same trust class as a lying assert label and is left to review.
rp_layers="$(sed -n '/^config_scalar_get()/,/^}/p' "$REPO/scripts/docket-config.sh" \
  | grep -E '^[[:space:]]*[a-z_]+\)[[:space:]]+config_scalar_from_lines' \
  | sed -E 's/^[[:space:]]*([a-z_]+)\).*/\1/' | LC_ALL=C sort)"
rp_n="$(grep -c . <<<"$rp_layers")"

assert "0258 L2 control: config_scalar_get dispatches at least three config layers (n=$rp_n)" \
  '[ "$rp_n" -ge 3 ]'
for rp_l in committed global local; do
  assert "0258 L2 control: layer $rp_l is dispatched by config_scalar_get" \
    'grep -qx "$rp_l" <<<"$rp_layers"'
done

# All ordered pairs over the derived layer set: n*(n-1) of them.
rp_expected="$(awk '{ a[NR] = $0 }
  END { for (i = 1; i <= NR; i++) for (j = 1; j <= NR; j++) if (i != j) print a[i] "->" a[j] }' \
  <<<"$rp_layers" | LC_ALL=C sort)"
rp_pinned="$(grep -hE '^# RUNG_PAIR: ' "$REPO"/tests/test_docket_config*.sh \
  | sed -E 's/^# RUNG_PAIR: //' | LC_ALL=C sort)"

assert "0258 L2 control: the family glob yielded a non-empty pinned pair population" \
  '[ -n "$rp_pinned" ]'
assert "0258 L2 control: $rp_n layers imply $(( rp_n * (rp_n - 1) )) ordered pairs" \
  '[ "$(grep -c . <<<"$rp_expected")" -eq "$(( rp_n * (rp_n - 1) ))" ]'
assert "0258 L2: the pinned rung pairs are exactly the resolver's ordered-pair set" \
  '[ "$rp_pinned" = "$rp_expected" ]'
```

- [ ] **Step 4: Run it and confirm every new assert is GREEN**

```bash
bash tests/.focus-0258.sh 2>&1 | grep '0258 L2'   # focused harness — see the Amendment
```
Expected: seven `ok - 0258 L2 …` lines, no `NOT OK`. If the set-equality assert is red, print both sides to see which pair is missing or doubled:
```bash
diff <(printf '%s\n' "$rp_expected") <(printf '%s\n' "$rp_pinned")
```
(Add that as a temporary debug line inside the file if needed; remove it before committing.)

- [ ] **Step 5: Mutation-prove it — a deleted fixture**

Delete the s7 fixture together with its marker (the comment block, the marker line, the `mkrepo`, the config writes, and the assert), run, restore.

```bash
cp tests/test_docket_config.sh /tmp/tdc.bak
# delete the entire (s7) block including its `# RUNG_PAIR:` line
bash tests/.focus-0258.sh 2>&1 | grep '0258 L2'   # focused harness — see the Amendment
cp -f /tmp/tdc.bak tests/test_docket_config.sh
```
Expected while mutated: `NOT OK - 0258 L2: the pinned rung pairs are exactly the resolver's ordered-pair set` — five pinned pairs against six expected.

- [ ] **Step 6: Mutation-prove it — a duplicated marker, and a total population wipe**

Duplicate one marker line, run, restore:

```bash
cp tests/test_docket_config.sh /tmp/tdc.bak
perl -0pi -e 's/^# RUNG_PAIR: local->committed\n/# RUNG_PAIR: local->committed\n# RUNG_PAIR: local->committed\n/m' tests/test_docket_config.sh
bash tests/.focus-0258.sh 2>&1 | grep '0258 L2'   # focused harness — see the Amendment
cp -f /tmp/tdc.bak tests/test_docket_config.sh
```
Expected while mutated: the set-equality assert reddens — `sort` (not `sort -u`) keeps the duplicate, so seven pinned entries never equal six expected.

Then delete **all six** markers, run, restore:

```bash
cp tests/test_docket_config.sh /tmp/tdc.bak
perl -ni -e 'print unless /^# RUNG_PAIR: /' tests/test_docket_config.sh
bash tests/.focus-0258.sh 2>&1 | grep '0258 L2'   # focused harness — see the Amendment
cp -f /tmp/tdc.bak tests/test_docket_config.sh
```
Expected while mutated: **both** `NOT OK - 0258 L2 control: the family glob yielded a non-empty pinned pair population` and the set-equality assert — an emptied population fails loudly rather than passing vacuously.

- [ ] **Step 7: Mutation-prove it — simulate the fourth config layer**

This is the failure mode the guard exists for. Add a `staging)` arm to `config_scalar_get` in a scratch copy of the resolver and point the extraction at it. Because `rp_layers` reads `"$REPO/scripts/docket-config.sh"` directly, mutate the real file and restore it:

```bash
cp scripts/docket-config.sh /tmp/dc-sh.bak
perl -0pi -e 's/^(    local\)     config_scalar_from_lines "\$2" "\$\{CONFIG_LINES_LOCAL\[@\]\}" ;;\n)/$1    staging)   config_scalar_from_lines "\$2" "\${CONFIG_LINES_STAGING[@]}" ;;\n/m' scripts/docket-config.sh
grep -n 'staging)' scripts/docket-config.sh   # confirm the arm landed
bash tests/.focus-0258.sh 2>&1 | grep '0258 L2'   # focused harness — see the Amendment
cp -f /tmp/dc-sh.bak scripts/docket-config.sh
```
Expected while mutated: `0258 L2 control: config_scalar_get dispatches at least three config layers (n=4)` stays **green** (the floor grows, it does not redden — that is deliberate), `0258 L2 control: 4 layers imply 12 ordered pairs` stays green, and `NOT OK - 0258 L2: the pinned rung pairs are exactly the resolver's ordered-pair set` fires: six markers against twelve required pairs.

Note: the mutated resolver will also break unrelated asserts because `CONFIG_LINES_STAGING` is undeclared under `set -u`. Only the leg-2 lines matter for this proof; restore immediately.

- [ ] **Step 8: Mutation-prove the derivation anchor**

Rename the function the extraction anchors on, run, restore:

```bash
cp scripts/docket-config.sh /tmp/dc-sh.bak
perl -pi -e 's/^config_scalar_get\(\)/config_scalar_fetch()/' scripts/docket-config.sh
bash tests/.focus-0258.sh 2>&1 | grep '0258 L2'   # focused harness — see the Amendment
cp -f /tmp/dc-sh.bak scripts/docket-config.sh
```
Expected while mutated: the `at least three config layers (n=0)` control and all three known-layer controls redden — a broken anchor cannot empty-compare green.

- [ ] **Step 9: Verify byte-restoration, then commit**

Delete the focused harness first, then re-run the **full** file once so the commit is gated on the
real suite file:

```bash
rm -f tests/.focus-0258.sh
bash tests/test_docket_config.sh 2>&1 | grep -E '0258 L|NOT OK'
git status --porcelain
git diff --stat
```
Expected: eight `0258 L1` plus seven `0258 L2` `ok` lines, no `NOT OK`; and only `tests/test_docket_config.sh` modified. Restore any incidentally-modified `scripts/` file with `git checkout -- scripts/`, and delete `tests/.focus-0258.sh` if it still shows — it is never committed.

```bash
git add tests/test_docket_config.sh
git commit -m "test(0258): compute the rung-pair claim instead of enumerating it in prose

Derive the expected ordered-pair set from config_scalar_get's layer
dispatch and assert set equality against per-fixture markers collected
over the test_docket_config*.sh family glob. A fourth config layer now
takes the expected set from 6 pairs to 12 and reddens until the six new
masking fixtures exist."
```

---

## Task 3: Full-suite gate and budget check

**Files:**
- Read-only: `tests/runtime-budgets.tsv` (row `tests/test_docket_config.sh` — budget `55`, `parallel`).
- Modify (only if the gate demands it): nothing is expected to change here.

**Interfaces:**
- Consumes: the finished state of Tasks 1 and 2.
- Produces: the build-evidence record for the branch.

- [ ] **Step 1: Confirm the suite command from its single source**

```bash
grep -n 'test_command' /Users/homer/dev/docket/.docket.yml
```
Expected: `scripts/run-tests.sh`. Use exactly that — never a second copy of the command.

- [ ] **Step 2: Run the whole suite**

```bash
cd /Users/homer/dev/docket/.worktrees/guard-the-config-suite-s-enumerated-claims-export-order-and
scripts/run-tests.sh
```
Expected: exit `0`, `PASS` for every file.

- [ ] **Step 3: Read the budget report for `tests/test_docket_config.sh`**

A trailing `OVER BUDGET:` block is advisory (exit stays `0`) — and therefore a finding to act on, because nothing else will catch it.

- If `tests/test_docket_config.sh` is **not** listed over budget: done, no action.
- If it **is** listed: do **not** raise the number in `tests/runtime-budgets.tsv` — that is explicitly forbidden by `tests/README.md`. Instead check whether the added cost is the leg-1 fixture. Leg 2 adds no fixture and no resolver run, so it cannot be the cause. If leg 1 is the cause, the cheapest remedy inside this change's scope is to drop the `mkrepo "$tmp/l1"` fixture and reuse an existing main-mode fixture for the two `rung` calls; if that still breaches, record the measurement in the results file and leave the retune to #0251, which owns the budget regime for this file.

- [ ] **Step 4: Confirm no unintended file is dirty**

```bash
git status --porcelain
git diff origin/main --stat
```
Expected: `tests/test_docket_config.sh` and `docs/superpowers/plans/…` only. `scripts/docket-config.md` and `scripts/docket-config.sh` must be **unchanged** — every mutation in Tasks 1 and 2 was restored.

- [ ] **Step 5: Commit anything the gate required**

If Steps 3–4 produced no edits, there is nothing to commit and this step is a no-op. Otherwise:

```bash
git add -A tests/
git commit -m "test(0258): keep the config suite inside its runtime budget"
```

---

## Self-Review

**1. Spec coverage.**

| Spec element | Task |
|---|---|
| Leg 1 doc-side extraction at the `### Emit` anchor, first token per line | Task 1 Step 1 (`emit_fence_tokens`) |
| Leg 1 control assert: non-empty, contains `DOCKET_MODE` and `BOOTSTRAP` | Task 1 Step 1 |
| Leg 1 runtime side: `--export --format plain` and default `shell`, one fixture | Task 1 Step 1 |
| Leg 1 verdict: sequence equality both formats, shell = fence minus `REPO_ROOT` | Task 1 Step 1 |
| Leg 1 prose numerals derived and grepped | Task 1 Step 1 |
| R7 / AUTO_* adjacency asserts stay | Global Constraints; nothing in any task touches them |
| Leg 2 layer derivation from `config_scalar_get` arms, `*)` excluded | Task 2 Step 3 |
| Leg 2 controls: n ≥ 3 floor plus three known members | Task 2 Step 3 |
| Leg 2 expected pairs = n·(n−1) | Task 2 Step 3 |
| Leg 2 per-fixture `RUNG_PAIR:` markers replacing the prose enumeration | Task 2 Steps 1–2 |
| Leg 2 verdict: set equality over the family glob | Task 2 Step 3 |
| Mutation proof leg 1: reorder, delete, count-stable rename, resolver emit removal, stale numeral | Task 1 Steps 3–5 |
| Mutation proof leg 2: deleted fixture, duplicated marker, simulated fourth layer, wiped population | Task 2 Steps 5–7 |
| Corpus-indifference to #0251 (glob, no `BASH_SOURCE`) | Global Constraints; Task 2 Step 3 |
| Docs: no `docket-config.md` content change; header comment reworded | File Structure; Task 2 Step 2 |

No gaps.

**2. Placeholder scan.** Every code step carries the literal code to paste; every run step carries the exact command and the exact expected output. The one conditional branch (Task 3 Step 3's over-budget path) names both outcomes and the decision rule rather than deferring it.

**3. Type consistency.** Task 1 produces `emit_fence_tokens`, `doc_plain_keys`, `doc_shell_keys`, `emit_plain_keys`, `emit_shell_keys`, `l1_plain_n`, `l1_shell_n`, `l1_sentence`; Task 2 produces `rp_layers`, `rp_n`, `rp_expected`, `rp_pinned`, `rp_l`. Disjoint prefixes (`l1_`/`doc_`/`emit_` vs `rp_`), no collision with each other or with the file's existing `ac_*`, `gob_*`, `out7*`, `n50` names. The marker literal `# RUNG_PAIR: ` is spelled identically in Task 2 Step 1's table, Step 3's collection grep, and every mutation command.
