<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0115 — Extend the board-row-dropped invariant to archive/ files](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0115-extend-the-board-row-dropped-invariant-to-archive-files.md)**
<!-- docket:backlink:end -->

# Archive-side `board-row-dropped` Invariant Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Widen the `board-row-dropped` health check so it covers `archive/` files as well as `active/` ones, using one generalized predicate and no new check-id.

**Architecture:** `board-checks.sh`'s `renders_row` predicate gains the directory as its first argument and branches to the status set the renderer actually iterates for that directory — `docket_status_is_active` for `active/`, `docket_status_is_terminal` for `archive/` — above a hoisted, shared "id must be a usable integer" clause. The population site in the file walk drops its `fd_active = 1` guard. Suppression needs no new code: `malformed-id` and the `field-domain` `status` arm already run over `archive/` files and are both genuine archive-side drop causes.

**Tech Stack:** Bash 3.2-compatible shell (BSD/macOS + GNU), the repo's hand-rolled `assert`/`has_finding` test harness in `tests/test_board_checks.sh`. No new dependencies.

## Global Constraints

- **No new check-id.** `board-row-dropped` is widened, never split. Do **not** touch the check-id enumerations in `scripts/board-checks.sh`'s header, `lib/docket-frontmatter.sh`'s `BOARD_CHECK_IDS`, `scripts/board-checks.md`'s section list, or `scripts/docket-status.md` — change 0111 guards those as a mirror and any edit reddens its guard.
- **No change to `scripts/render-board.sh`.** The renderer is this change's oracle, not its subject. (One real renderer bug is documented in Task 1's notes and captured as separate follow-up work — do not fix it here.)
- **The active-side message stays byte-identical.** Only the archive branch gets new text.
- **Shell portability.** `PATH` `grep` on this machine is ugrep and accepts constructs BSD `grep` rejects; verify any new grep with `/usr/bin/grep`. Never use a literal `\t` inside `grep -E` — use the repo's `"$(printf '^x\ty\t')"` idiom. Bash 3.2: no `unset arr[-1]`, no associative-array tricks beyond what the file already uses.
- **Test values in this plan were verified against running code** on 2026-07-27 at `origin/main` `0da1c0aa`. Where a value is empirically confirmed the step says so; treat anything unmarked as still needing a run.

---

## File Structure

| File | Responsibility | Change |
|---|---|---|
| `scripts/board-checks.sh` | The predicate, the population site, the two messages, the explanatory comments | Modify |
| `tests/test_board_checks.sh` | Archive-side cases T1–T7, plus the one shipped assertion that inverts | Modify |
| `scripts/board-checks.md` | The `board-row-dropped` contract section — replace its "covers `active/` only" paragraph | Modify |

---

### Task 1: Widen the predicate and its population site

**Files:**
- Modify: `scripts/board-checks.sh` (the `renders_row` comment + function, ~lines 97–118; the `fd_active` derivation, ~line 128; the population site, ~line 136)
- Test: `tests/test_board_checks.sh` (the change-0104 block, ~lines 595–702)

**Interfaces:**
- Consumes: `docket_status_is_active` and `docket_status_is_terminal` from `scripts/lib/docket-frontmatter.sh` (already sourced at line 54). Both take one STATUS argument and exit 0 on membership.
- Produces: `renders_row DIR_KIND ID STATUS` — a **three**-argument predicate (was two). `DIR_KIND` is the literal string `active` or `archive`. Every later task calls this signature.

**Background a fresh implementer needs.** The board's count line says `**N changes**`, where N counts every `*.md` under `active/` plus every `*.md` under `archive/` with no id or status filter. The tables that follow are rendered by narrower passes. When a file joins the count but no table accounts for it, the board silently disagrees with itself. `board-row-dropped` (change 0104) catches that on the `active/` side; this task extends it to `archive/`.

Two distinct archive-side failures exist, and both must fire:

- **Case A — a non-terminal status in `archive/`** (e.g. `implemented`). Reachable for real: `archive-change.sh` does its `git mv` *before* the status flip and the commit, so an interrupted run leaves exactly this state.
- **Case B — a terminal-status archive file with no usable integer id.** The archive summary count keys on raw status alone, so the file *is* counted, while no row identifying it renders.

**Empirically verified baseline (2026-07-27).** A fixture with `archive/2026-06-16-0080-misfiled.md` at `status: implemented` beside a healthy `archive/2026-06-16-0081-good.md` at `status: done` renders `**2 changes**` above `<summary>✅ Archive — done (1)</summary>` — and `board-checks.sh` emits **nothing**. That silence is the gap this task closes.

**Note for the implementer — do not act on this, it is context.** Case B's real renderer behavior differs from what the spec predicted. The spec says the no-id row is dropped by the renderer's `[ -n "$id" ] || continue` guard. It is not: the archive sort feeder joins its fields with TAB and reads them back with `IFS=$'\t'`, and because TAB is an IFS *whitespace* character, an empty id field **collapses** and shifts every later field left — `id` receives the status, `st` receives the file path. The guard therefore never sees an empty id, and a corrupt row (`| [0000](archive/) |  | 2026-06-16 |`) renders instead. The file is still unaccounted for — a row identifying nothing is not the file's row — so the predicate below is correct either way and needs no adjustment. This is a genuine `render-board.sh` bug, deliberately **out of scope** here (Global Constraints forbid touching the renderer) and captured as separate follow-up work. It is why this task's message says "no row **identifying** it" rather than "no row rendered".

- [ ] **Step 1: Invert the one shipped assertion whose premise this change deletes**

In `tests/test_board_checks.sh`, case **(d)** currently asserts that `archive/` is exempt. That premise is exactly what this change removes, so the block is **inverted and kept**, never re-gated to green and never deleted — it guards a real mechanism (the no-id archive drop, case B) and simply guarded it backwards.

Replace the whole case (d) block:

```bash
# (d) CASE B — an archive/ file with NO id: field at all, at a terminal status. The archive summary
#     count keys on the raw status, so this file IS counted there, while no row IDENTIFYING it ever
#     renders. Nothing enumerated explains it: malformed-id needs a non-empty raw id value, and
#     `done` is a legal status so field-domain passes it. Only the computed invariant sees it.
#     (Before change 0115 this block asserted the OPPOSITE, under the premise that archive/ was
#     exempt from the invariant. That premise is what 0115 deletes.)
read -r I _ < <(new_repo)
printf -- '---\nslug: archnoid\ntitle: Arch no id\nstatus: done\npriority: medium\ndepends_on: []\n---\n' \
  > "$I/docs/changes/archive/2026-06-16-0073-archnoid.md"
iout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$I/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "board-row-dropped fires for an archive/ file with no id: field (0073)" \
  'has_finding "$iout" board-row-dropped 0073'
assert "malformed-id does NOT fire for it (there is no raw id value to report)" \
  '! has_finding "$iout" malformed-id 0073'
assert "field-domain does NOT explain it (done is a legal status)" \
  '! has_finding "$iout" field-domain 0073'
```

The change-id is `0073` (zero-padded), not `73`: with no frontmatter id the check falls back to `padded_id_from_file`, which strips the archive date prefix from `2026-06-16-0073-archnoid.md` and yields `0073`.

- [ ] **Step 2: Add the archive-side cases T1–T5**

Append immediately after case (g) (the false-suppression guard, which ends with the id-79 `field-domain` assert), before the `merged-orphan / unknown-commit-ref` banner:

```bash
# ============ archive-side board-row-dropped (change 0115) ============
# The invariant is SINGULAR — one `total`, one set of tables — so it is widened, not split: no new
# check-id. renders_row now takes the directory and reads the status set the renderer actually
# iterates for it (DOCKET_STATUSES_ACTIVE vs DOCKET_STATUSES_TERMINAL, via the shared
# docket_status_is_* helpers), above a hoisted "id must be usable" clause.

# (T1) CASE A, block open: a non-terminal status in archive/, beside a healthy done sibling. The
# archive block opens (the sibling is terminal) so the misfiled row DOES print — but under a
# <summary> count that excludes it, because that count reads terminal statuses only. Count and
# tables disagree, which is the whole invariant.
read -r AA _ < <(new_repo)
printf -- '---\nid: 80\nslug: misfiled\ntitle: Misfiled\nstatus: implemented\npriority: medium\ndepends_on: []\n---\n' \
  > "$AA/docs/changes/archive/2026-06-16-0080-misfiled.md"
printf -- '---\nid: 81\nslug: good\ntitle: Good\nstatus: done\npriority: medium\ndepends_on: []\n---\n' \
  > "$AA/docs/changes/archive/2026-06-16-0081-good.md"
aaout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AA/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "board-row-dropped fires for a NON-TERMINAL status in archive/ (80)" \
  'has_finding "$aaout" board-row-dropped 80'
assert "the healthy done sibling in archive/ draws no finding (81)" \
  '! has_finding "$aaout" board-row-dropped 81'
assert "the archive misfile is NOT explained by field-domain (implemented is a legal status)" \
  '! has_finding "$aaout" field-domain 80'

# (T2) CASE A, block closed: the same misfiled file with NO terminal sibling. The entire archive
# block is gated on the terminal counts, so it never opens and the row appears NOWHERE at all.
# Distinct from T1 in the rendered outcome; identical in the accounting failure.
read -r AB _ < <(new_repo)
printf -- '---\nid: 82\nslug: alone\ntitle: Alone\nstatus: implemented\npriority: medium\ndepends_on: []\n---\n' \
  > "$AB/docs/changes/archive/2026-06-16-0082-alone.md"
about="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AB/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "board-row-dropped fires when the archive block never opens at all (82)" \
  'has_finding "$about" board-row-dropped 82'

# (T3) FALSE-POSITIVE GUARD for the ARCHIVE_RECENT window. 16 well-formed done files: the renderer
# shows 15 verbatim and REDIRECTS the 16th into the per-month "Older done (collapsed)" digest.
# Collapse is a redirect, not a discard — the file is still in the summary count and still
# represented in the digest — so the predicate, which is written against ACCOUNTING rather than
# against verbatim row emission, must be blind to it. A predicate "tightened" toward row emission
# would fire on every done file past the 16th; this assert is what stops that from creeping back in.
# Asserted on the SPECIFIC id the window pushes out (not "no findings at all", which would pass
# vacuously): sort is date-desc, so the oldest date (2026-06-01, id 101) is the one that collapses.
# Verified against the running renderer: 15 verbatim rows + 1 collapsed.
read -r AC _ < <(new_repo)
for i in $(seq 1 16); do
  acd="$(printf '2026-06-%02d' "$i")"; acid=$(( 100 + i ))
  printf -- '---\nid: %s\nslug: c%s\ntitle: C%s\nstatus: done\npriority: medium\ndepends_on: []\n---\n' \
    "$acid" "$acid" "$acid" > "$AC/docs/changes/archive/$acd-0$acid-c$acid.md"
done
acout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AC/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "the collapsed done file draws NO board-row-dropped finding (101 — ARCHIVE_RECENT redirects, never discards)" \
  '! has_finding "$acout" board-row-dropped 101'
assert "nor does the newest verbatim done row (116)" \
  '! has_finding "$acout" board-row-dropped 116'

# (T4) the other member of DOCKET_STATUSES_TERMINAL: a killed archive file is healthy and silent.
# Its value is covering `killed`, not anything about collapse (killed never collapses).
read -r AD _ < <(new_repo)
printf -- '---\nid: 84\nslug: abandoned\ntitle: Abandoned\nstatus: killed\npriority: medium\ndepends_on: []\n---\n' \
  > "$AD/docs/changes/archive/2026-06-16-0084-abandoned.md"
adout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AD/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "a killed archive file draws NO board-row-dropped finding (84)" \
  '! has_finding "$adout" board-row-dropped 84'
```

T5 is case (d), already written in Step 1.

- [ ] **Step 3: Run the new tests and watch them fail for the right reason**

Run: `bash tests/test_board_checks.sh 2>&1 | /usr/bin/grep -n 'NOT OK'`

Expected failures — exactly these five, and nothing else:
- `board-row-dropped fires for an archive/ file with no id: field (0073)`
- `board-row-dropped fires for a NON-TERMINAL status in archive/ (80)`
- `board-row-dropped fires when the archive block never opens at all (82)`

The T3 and T4 negatives, and the `! has_finding` asserts in (d) and T1, pass **already** — the check does not fire on `archive/` at all yet. That is expected and is precisely why they cannot be the only asserts: a negative assert against a check that never fires is vacuous. Their job begins once the population site is widened in Step 4.

If any assert outside this list fails, stop and report — the fixtures have collided with an existing case.

- [ ] **Step 4: Widen the predicate and the population site**

In `scripts/board-checks.sh`, replace the `renders_row` comment block and function (currently ~lines 97–118) with:

```bash
# renders_row DIR_KIND ID STATUS — exit 0 iff render-board.sh would account for a file in DIR_KIND
# ('active' or 'archive') carrying this (int_field-validated) ID and this raw STATUS. This is the
# COMPUTED half of the board-row-dropped invariant (change 0104; widened to archive/ by 0115): it
# mirrors the renderer's own bucketing rather than re-enumerating the conditions the other checks
# already name, so a drop path ADDED TO THE RENDERER is noticed here without anyone editing this
# script. Three clauses, each anchored to real renderer behavior:
#   1. The id gate, hoisted because it is ONE condition holding in BOTH directories: the renderer
#      requires a usable integer id to emit an identifying row on either side. A file without one
#      is still counted in `total`, so it is unaccounted for.
#   2. active/  -> the renderer calls print_section once per DOCKET_STATUSES_ACTIVE member and
#      buckets on the RAW `status:` read, so a status outside that set lands in a bucket nothing
#      iterates. The live case is a TERMINAL status sitting in active/ — legal status, wrong
#      directory (the `sweep-failed <id> archive <reason>` state: status flipped, archive move
#      failed).
#   3. archive/ -> the archive block's open gate and its <summary> count both come from the
#      per-status archive tally read over DOCKET_STATUSES_TERMINAL, so a NON-terminal status is
#      counted in `total` and joins no summary. The live case is the mirror image: an interrupted
#      archive-change.sh, whose `git mv` precedes its status flip.
# Membership is read from the SHARED arrays via docket_status_is_active / docket_status_is_terminal,
# never a list restated here. That matters twice over: the active set (five names) and the full
# vocabulary (seven) are DIFFERENT sets and the difference IS the drop path, and since change 0116
# single-sourced the renderer's own vocabularies the renderer reads these very arrays too — so both
# arms are backed by the same source the consumer reads, not by a comment-asserted correspondence.
renders_row(){
  local rr_dir="$1" rr_id="$2" rr_st="$3"
  [ -n "$rr_id" ] || return 1
  case "$rr_dir" in
    active)  docket_status_is_active   "$rr_st" ;;
    archive) docket_status_is_terminal "$rr_st" ;;
    *) return 1 ;;
  esac
}
```

Then harden the `dir_kind` derivation and widen the population site. Replace (~line 128):

```bash
  fd_active=0; case "$f" in */active/*) fd_active=1 ;; esac
```

with:

```bash
  # Anchored on "$CHANGES_DIR" rather than a bare */active/* glob: an unanchored pattern
  # misclassifies every file when CHANGES_DIR itself contains an `active` path component.
  dir_kind=archive; case "$f" in "$CHANGES_DIR"/active/*) dir_kind=active ;; esac
```

and replace the population site (~line 136):

```bash
  if [ "$fd_active" = 1 ] && ! renders_row "$id" "$status"; then DROPPED["$cid"]=1; fi
```

with:

```bash
  if ! renders_row "$dir_kind" "$id" "$status"; then DROPPED["$cid"]=1; fi
```

`fd_active` has no other reader — confirm with `/usr/bin/grep -n fd_active scripts/board-checks.sh` and expect zero hits after the edit. (`fd_slug`, `fd_priority`, `fd_title`, `fd_type` are unrelated names; do not touch them.)

- [ ] **Step 5: Give the archive branch its own message**

The emitted message must name the archive pass so the reader knows which way the file is misfiled, stay honest about both sub-cases, and describe the invariant rather than enumerate causes. Replace the single `emit` in the `board-row-dropped` loop (~line 336) with a directional pair. The **active string is byte-identical to what ships today** — do not reword it.

```bash
for drop_id in "${!DROPPED[@]}"; do
  [ -n "${EXPLAINED[$drop_id]:-}" ] && continue
  # The message names the two SUPPRESSING arms specifically, not "field-domain" wholesale: a change
  # can legitimately carry a field-domain finding (a piped title, say) AND this one, because that
  # finding does not account for a dropped row. Saying "no field-domain finding explains it" next to
  # a visible field-domain finding on the same id would read as a contradiction.
  # Two strings, one per direction: the direction is what tells the reader which way the file is
  # misfiled, and it is the reason this is a widened check rather than a second check-id.
  if [ "${DROPPED_DIR[$drop_id]:-active}" = archive ]; then
    emit board-row-dropped "$drop_id" "counted in the board total but not accounted for by the archive pass (no row identifying it, or a summary count that excludes it); no malformed-id or field-domain status finding accounts for the drop"
  else
    emit board-row-dropped "$drop_id" "counted in the board total but rendered in no section; no malformed-id or field-domain status finding accounts for the drop"
  fi
done
```

This needs a companion map. Declare it beside `DROPPED` (~line 62):

```bash
declare -A EXPLAINED DROPPED DROPPED_DIR      # change-id -> 1 / -> dir kind; drive board-row-dropped
```

and record the direction at the population site, so Step 4's line becomes:

```bash
  if ! renders_row "$dir_kind" "$id" "$status"; then DROPPED["$cid"]=1; DROPPED_DIR["$cid"]="$dir_kind"; fi
```

- [ ] **Step 6: Run the full suite**

Run: `bash tests/test_board_checks.sh 2>&1 | /usr/bin/grep -c '^ok'` then `bash tests/test_board_checks.sh 2>&1 | /usr/bin/grep 'NOT OK'`

Expected: zero `NOT OK` lines. If the id-79 or id-71 cases redden, the `EXPLAINED` wiring was disturbed — revert Step 5 and re-apply it without touching the suppression `continue`.

- [ ] **Step 7: Verify against this repo's real archive, not only fixtures**

The hermetic suite never sees real history. Run the check over the actual metadata worktree:

```bash
bash scripts/board-checks.sh --changes-dir /Users/homer/dev/docket/.docket/docs/changes \
  --metadata-branch docket --integration-branch main | /usr/bin/grep '^board-row-dropped' || echo "clean"
```

Expected: `clean`. This repo's `archive/` was swept at reconcile — 111 files, every one terminal-status with a valid integer id — so a hit here is a genuinely misfiled file, not a test failure. Report it rather than "fixing" the predicate.

- [ ] **Step 8: Commit**

```bash
git add scripts/board-checks.sh tests/test_board_checks.sh
git commit -m "fix(0115): widen board-row-dropped to archive/ files

renders_row takes the directory and reads the status set the renderer
iterates for it, above a hoisted id clause. Catches a non-terminal status
in archive/ (an interrupted archive-change.sh) and a terminal archive file
with no usable id — the two states no enumerated check can see."
```

---

### Task 2: Pin the suppression analysis on the archive side

**Files:**
- Test: `tests/test_board_checks.sh` (append after Task 1's T4 block)

**Interfaces:**
- Consumes: `renders_row DIR_KIND ID STATUS` and the widened population site from Task 1.
- Produces: nothing later tasks call.

**Why this is its own task.** Task 1 proves the invariant *fires* on the archive side. This proves it *stays quiet* when another finding already explains the drop — a separate claim a reviewer could reject independently. The design asserts no new suppression code is needed, because both suppressing arms (`malformed-id`, and `field-domain`'s `status` arm) already run over `archive/` files without a directory gate. These two tests are what turn that reading into evidence.

- [ ] **Step 1: Write the two suppression tests**

```bash
# (T6) suppression by malformed-id on the archive side. A non-integer id is a genuine archive drop
# cause, so the enumerated finding accounts for it and the backstop stays quiet.
read -r AE _ < <(new_repo)
printf -- '---\nid: nope\nslug: badarch\ntitle: Bad arch id\nstatus: done\npriority: medium\ndepends_on: []\n---\n' \
  > "$AE/docs/changes/archive/2026-06-16-0085-badarch.md"
aeout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AE/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "malformed-id fires for a non-integer id in archive/ (0085)" \
  'has_finding "$aeout" malformed-id 0085'
assert "board-row-dropped is suppressed when malformed-id explains the archive drop (0085)" \
  '! has_finding "$aeout" board-row-dropped 0085'

# (T7) suppression by the field-domain `status` arm on the archive side. A status outside the
# seven-name vocabulary is outside DOCKET_STATUSES_TERMINAL too, so it explains the archive drop.
# EXACTLY ONE finding, for the same reason case (b) asserts it on the active side: DROPPED is
# written by the computed predicate and EXPLAINED by the field-domain status arm, at independent
# sites — so deleting that arm's EXPLAINED marker reddens this with a second finding.
read -r AF _ < <(new_repo)
printf -- '---\nid: 86\nslug: weird\ntitle: Weird status\nstatus: finished\npriority: medium\ndepends_on: []\n---\n' \
  > "$AF/docs/changes/archive/2026-06-16-0086-weird.md"
afout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AF/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
n86="$(printf '%s' "$afout" | /usr/bin/grep -c .)"
assert "an out-of-vocabulary archive status yields exactly ONE finding (86)" '[ "$n86" = 1 ]'
assert "and that one finding is field-domain, not board-row-dropped (86)" \
  'has_finding "$afout" field-domain 86'
assert "board-row-dropped is suppressed when field-domain explains the archive drop (86)" \
  '! has_finding "$afout" board-row-dropped 86'
```

- [ ] **Step 2: Run and confirm green**

Run: `bash tests/test_board_checks.sh 2>&1 | /usr/bin/grep 'NOT OK'`
Expected: no output. These pass on first run — the suppression machinery is already directory-agnostic, which is the claim being pinned. Their value is regression protection and the mutation evidence in Task 3, not a red-to-green transition.

If `n86` is not 1, print `$afout` and check whether the fixture drew an unrelated finding (a `slug`/`priority` violation would add one). Adjust the fixture, never the assert.

- [ ] **Step 3: Commit**

```bash
git add tests/test_board_checks.sh
git commit -m "test(0115): pin archive-side suppression by malformed-id and field-domain status"
```

---

### Task 3: Mutation matrix

**Files:**
- Modify: `scripts/board-checks.sh` and `tests/test_board_checks.sh` — **temporarily**, one mutation at a time, each reverted before the next
- Create: nothing

**Interfaces:**
- Consumes: everything from Tasks 1 and 2.
- Produces: the evidence table pasted into the results file at Step 3 below.

**Why.** A backstop has two halves — the code that *creates* an entry and the code that *suppresses* an already-explained one — and a suppression assert passes **vacuously** when the invariant never computes at all. So the population must be mutation-tested, not only the suppression. One mutation per *independent clause*: the predicate has three, and a single blanket mutation would let two of them pass vacuously.

**Run each mutation, record which asserts redden, then `git checkout -- <file>` before the next.** Verify the revert with `git diff --quiet` each time.

- [ ] **Step 1: Run the five mutations**

| # | Mutation | Must redden | Must stay green |
|---|---|---|---|
| M1 | In `renders_row`, make the `archive` arm `return 0` unconditionally | T1 (80), T2 (82) | **T5 / case (d) (0073)** — see the trap below |
| M2 | Delete the hoisted `[ -n "$rr_id" ] \|\| return 1` line | T5 (0073) **and** the active-side no-id case (a) (0070) | T1, T2 |
| M3 | Restore the active-only guard: `if [ "$dir_kind" = active ] && ! renders_row …` | T1, T2, **and** T5 together | the active cases (a), (f), (g) |
| M4 | Delete `EXPLAINED["$cid"]=1` in the `malformed-id` block | T6 (0085) gains a second finding | — |
| M5 | Delete `EXPLAINED["$cid"]=1` in the `field-domain` `status` arm | T7 (86) gains a second finding, **and** the pre-existing active-side `n71 = 1` pair | — |

**M1 is the trap — read before running it.** T5 is a **no-id** file. The hoisted id clause kills it *before* the directory switch runs, so widening the archive status arm leaves T5 green. An implementer who expects T5 to redden here will misread a working harness as broken, or worse, "fix" the predicate until it does. T5 staying green under M1 is the **correct** result and is what proves the two clauses are independent.

**M3 is the population-deletion mutation** — the one that catches a backstop whose entries are never created. It must redden all three archive cases at once.

**M5's second entry is expected collateral, not a defect.** An implementer checking "only the listed tests went red" would otherwise trip on it.

For each: apply the edit, run `bash tests/test_board_checks.sh 2>&1 | /usr/bin/grep 'NOT OK'`, record the exact assert names, then revert.

- [ ] **Step 2: Confirm the tree is clean**

Run: `git diff --stat` — expected: empty. Then `bash tests/test_board_checks.sh 2>&1 | /usr/bin/grep -c 'NOT OK'` — expected: `0`.

If any mutation's observed result differs from the table, **stop and report it** rather than adjusting the table to match. A cell that does not behave as predicted means either the predicate's clauses are not independent or an assert is decoration; both are findings, not bookkeeping.

- [ ] **Step 3: Record the matrix**

Keep the observed results — every assert name that reddened per mutation. They go into the results file in Task 4, as the evidence that each clause is load-bearing.

No commit for this task; it produces evidence, not a diff.

---

### Task 4: Documentation and results

**Files:**
- Modify: `scripts/board-checks.md` (the `board-row-dropped` section)
- Create: `docs/results/2026-07-27-extend-the-board-row-dropped-invariant-to-archive-files-results.md`

**Interfaces:**
- Consumes: the mutation matrix from Task 3.
- Produces: nothing.

- [ ] **Step 1: Read the current contract section**

Run: `/usr/bin/grep -n 'board-row-dropped' scripts/board-checks.md`

Locate the paragraph stating the check covers `active/` only — it currently documents this exact gap as follow-up work, which is what makes it stale the moment Task 1 lands.

- [ ] **Step 2: Replace that paragraph**

Write the two-sided predicate, the live-trigger list for each direction, and an explicit note that `ARCHIVE_RECENT` collapse is **not** a drop. Do not add a check-id anywhere. Cover:

- The predicate: a usable integer id, plus membership in the status set the renderer iterates for that directory — `DOCKET_STATUSES_ACTIVE` for `active/`, `DOCKET_STATUSES_TERMINAL` for `archive/`.
- `active/` live triggers: no `id:` field at all; a terminal status in `active/`.
- `archive/` live triggers: a non-terminal status in `archive/`; a terminal file with no usable id.
- Suppression: `malformed-id`, and `field-domain` on `status` — both already directory-agnostic. `slug`/`priority`/`title`/`type` deliberately do not suppress.
- The collapse note: the recency window **redirects** a row into the per-month digest; the file stays in the summary count, so the predicate — written against accounting, not verbatim row emission — is blind to it by design and must stay that way.

- [ ] **Step 3: Verify the check-id guard still passes**

Run: `bash tests/test_board_checks.sh 2>&1 | /usr/bin/grep 'NOT OK'`
Expected: no output. Change 0111's enumeration guard compares `board-checks.md`'s section list against `BOARD_CHECK_IDS` as a **set in both directions**, so an accidental new heading in the section you just rewrote reddens it. If it does, you added a check-id — remove it.

- [ ] **Step 4: Write the results file**

Create `docs/results/2026-07-27-extend-the-board-row-dropped-invariant-to-archive-files-results.md` from `docs/results/results-template.md`. It must record:

- The **mutation matrix** from Task 3 with observed results, including M1's deliberate T5-stays-green cell and M5's expected collateral.
- The **reconcile deltas**: change 0116 had landed, so the spec's "mirror by convention" caveat was obsolete and **T9 was dropped** rather than deferred — the correspondence it would have asserted is now structural, since the renderer reads `DOCKET_STATUSES_TERMINAL` itself.
- The **renderer bug found while verifying case B**: the archive sort feeder's TAB-joined fields are read back with `IFS=$'\t'`, and because TAB is IFS-whitespace an empty id collapses the field and shifts every later one, defeating the renderer's own id guard and printing a corrupt `| [0000](archive/) | | <date> |` row. Out of scope here; captured as follow-up. Note that the **active side is unaffected**, because its id guard runs before the TAB-join.
- The **real-archive verification** from Task 1 Step 7 (hermetic fixtures do not exercise real history).
- The one **shipped assertion that inverted** (case (d)) and why inverting was correct rather than re-exempting `archive/`.

- [ ] **Step 5: Commit**

```bash
git add scripts/board-checks.md docs/results/2026-07-27-extend-the-board-row-dropped-invariant-to-archive-files-results.md
git commit -m "docs(0115): two-sided board-row-dropped contract + results"
```

---

## Self-Review

**Spec coverage.** Decision (widen, no new check-id) → Task 1 Step 4. The predicate's three clauses → Task 1 Step 4, mutation-pinned in Task 3. Population site → Task 1 Step 4. Not-derived-from-`ARCHIVE_RECENT` → T3. Suppression (no new code) → Task 2. Message → Task 1 Step 5. Rejected "wrong directory" framing → survives only as the message's remedy hint, per Task 1 Step 5. Test plan T1–T7 → Tasks 1–2; **T9 dropped at reconcile** (0116 landed); T8 → Task 3. Existing tests that change: case (d) inverts → Task 1 Step 1; case (f)'s `L` fixture stays green unchanged → covered by Task 1 Step 6's full-suite run. Documentation → Task 4. Assumption 9(ii) (`dir_kind` glob anchoring) → Task 1 Step 4; 9(i) stays out of scope.

**Placeholder scan.** No TBDs; every test and every edit is written out in full. Task 4 Step 2 specifies the prose by required content rather than verbatim text — deliberate, since it is documentation whose wording is the author's, and each required element is enumerated.

**Type consistency.** `renders_row` is three-argument (`DIR_KIND ID STATUS`) everywhere after Task 1. `dir_kind` replaces `fd_active` at both its derivation and its single use; Task 1 Step 4 includes the zero-hits check. `DROPPED_DIR` is declared beside `DROPPED` and written at the one population site, read only in the emit loop. Fixture ids are unique across the file: 80, 81, 82, 84, 85, 86, and 101–116 do not collide with the existing 70–79 block or the 10/11/12/50/51/52/53/60 fixtures elsewhere.
