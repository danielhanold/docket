# Archive decay — rolling digest for the always-loaded board — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Keep docket's always-loaded `BOARD.md` flat in size as the archive grows, by giving `render-board.sh` a count-based archive recency window with a per-month digest for older `done`, and pruning the mermaid graph to only the done nodes an active change actually depends on.

**Architecture:** Change is confined to the single deterministic renderer `scripts/render-board.sh` (STDOUT only — the caller `board-refresh.sh` still owns the atomic `BOARD.md` write), its contract `scripts/render-board.md`, and the golden test `tests/test_render_board.sh`. No config knob, no caller change, no new file, no change to any archived record. Two independent output changes: (1) mermaid pruning — universal, alters every board with an unreferenced done id; (2) archive-table window — inert below the threshold (byte-identical archive table for small repos), collapsing older `done` only above it.

**Tech Stack:** Bash (renderer already requires bash 4+ — uses `declare -A`, `mapfile`), sourced helper `scripts/lib/docket-frontmatter.sh` (`field`, `int_field`, `list_field`, `pad`, `resolve_deps`).

## Global Constraints

- **Determinism is load-bearing.** Same change files → identical bytes. Every input must derive from the change files (sort order, filename `YYYY-MM-DD` prefixes, `depends_on` sets) — **no wall-clock**. This is why the recency window is count-based, not time-based. The golden byte-compare in `tests/test_render_board.sh` and the "commit only if bytes changed" board gate both depend on it.
- **Rendered views only.** No archived change file, spec, or ADR is summarized-in-place, rewritten, or deleted. This edits renderer output only.
- **`render-board.sh` runs under `set -uo pipefail`** (NOT `set -e`). Guard every associative-array read with `:-`; declare arrays before use so `set -u` never trips on an empty expansion.
- **Window constant:** `ARCHIVE_RECENT=15`, a single named constant at the top of `render-board.sh`.
- **Killed entries never collapse** — every `killed` archive row renders verbatim regardless of age; only `done` collapses into the digest.
- **Shell house rules (AGENTS.md):** never `producer | early-exiting-consumer` under pipefail (capture into a var first); a `grep` pattern that leads with `--` must use `-e`/`-F --`; a guard is code — the golden/assertions are the regression guard.
- **Commit style:** conventional, scoped to the change id, e.g. `feat(0093): <summary>`.

---

### Task 1: Prune the mermaid graph to depends_on-referenced done nodes

**Files:**
- Modify: `scripts/render-board.sh` (mermaid block, currently the `# --- mermaid ---` section near the end, ~L200-222 on `origin/main`)
- Modify: `scripts/render-board.md` (the **Dependency graph (Mermaid)** behavior paragraph)
- Test: `tests/test_render_board.sh` (update the existing hand-authored golden's mermaid block)

**Interfaces:**
- Consumes: `list_field FILE depends_on` (space-separated bare parent ids), `field FILE status`, `int_field FILE id`, `pad ID` (zero-pads to 4 digits), the `ARCFILES` array, and `rows_sorted STATUS` — all already present in the script.
- Produces: unchanged script signature. The mermaid block now emits `:::done` only for referenced done ids and emits the `classDef done` line only when at least one such node remains.

**Why the referenced set must be collected in the edge loop (not from `resolve_deps`):** `resolve_deps`'s `DEP_ON[id]` records only the *worst unmet* dependency; a **done** dependency is *satisfied* and is skipped there. The mermaid needs exactly the done deps, so the referenced-id set is collected during the edge-emitting loop, which already iterates every `depends_on` value.

- [ ] **Step 1: Update the existing golden to the post-pruning expected output**

In `tests/test_render_board.sh`, inside the hand-authored `golden` heredoc, the mermaid block currently reads:

```
  0010:::done
  0012:::done
  classDef done fill:#d3f9d8;
```

Delete the `  0012:::done` line so it reads:

```
  0010:::done
  classDef done fill:#d3f9d8;
```

Rationale: the fixture's active change `0002-bravo` has `depends_on: [10]`, so `0010` is referenced and keeps its `:::done` styling; **nothing** in the fixture depends on `0012`, so it is dropped. `0010` remains, so `classDef done` stays. No other golden line changes (the 3-entry archive table is well under the window and stays byte-identical).

- [ ] **Step 2: Run the test to verify it now fails against the un-pruned renderer**

Run: `bash tests/test_render_board.sh`
Expected: FAIL — the assert `"rendered output matches the golden byte-for-byte"` reports a diff showing the current renderer still emits `  0012:::done` while the golden no longer does.

- [ ] **Step 3: Implement the mermaid pruning in `scripts/render-board.sh`**

Replace the entire `# --- mermaid ---` block. The current block is:

```bash
# --- mermaid ---
printf '\n```mermaid\ngraph TD\n'
# emit all active changes in ascending numeric id order
while IFS=$'\t' read -r id f; do
  [ -n "$id" ] || continue
  local_deps="$(list_field "$f" depends_on)"
  if [ -n "$local_deps" ]; then
    for dep in $local_deps; do printf '  %s --> %s\n' "$(pad "$dep")" "$(pad "$id")"; done
  else
    printf '  %s\n' "$(pad "$id")"
  fi
done < <(
  for st in in-progress proposed blocked deferred implemented; do
    rows_sorted "$st"
  done | sort -t$'\t' -k1,1n
)
# done nodes (ascending id); killed omitted
mapfile -t DONE_IDS < <(for f in "${ARCFILES[@]}"; do
  [ "$(field "$f" status)" = "done" ] && { v="$(int_field "$f" id)"; [ -n "$v" ] && printf '%s\n' "$v"; }; done | sort -n)
for id in "${DONE_IDS[@]}"; do [ -n "$id" ] && printf '  %s:::done\n' "$(pad "$id")"; done
printf '  classDef done fill:#d3f9d8;\n```\n'
```

Replace it with:

```bash
# --- mermaid ---
printf '\n```mermaid\ngraph TD\n'
# Emit all active changes in ascending numeric id order; record every id referenced by an active
# change's depends_on (padded form as the key) so done nodes can be pruned to referenced-only
# below. A DONE dependency is *satisfied* in resolve_deps and skipped there, so the referenced set
# must be collected here, in the loop that already reads every depends_on value. (change 0093)
declare -A REFERENCED
while IFS=$'\t' read -r id f; do
  [ -n "$id" ] || continue
  local_deps="$(list_field "$f" depends_on)"
  if [ -n "$local_deps" ]; then
    for dep in $local_deps; do
      REFERENCED["$(pad "$dep")"]=1
      printf '  %s --> %s\n' "$(pad "$dep")" "$(pad "$id")"
    done
  else
    printf '  %s\n' "$(pad "$id")"
  fi
done < <(
  for st in in-progress proposed blocked deferred implemented; do
    rows_sorted "$st"
  done | sort -t$'\t' -k1,1n
)
# Done nodes (ascending id): style :::done ONLY for a done id an active change depends on;
# unreferenced done ids carry no edge and are dropped. Killed omitted entirely. Emit the classDef
# line only when at least one :::done node remains (no dangling def). (change 0093)
mapfile -t DONE_IDS < <(for f in "${ARCFILES[@]}"; do
  [ "$(field "$f" status)" = "done" ] && { v="$(int_field "$f" id)"; [ -n "$v" ] && printf '%s\n' "$v"; }; done | sort -n)
done_shown=0
for id in "${DONE_IDS[@]}"; do
  [ -n "$id" ] || continue
  [ -n "${REFERENCED["$(pad "$id")"]:-}" ] || continue
  printf '  %s:::done\n' "$(pad "$id")"; done_shown=1
done
[ "$done_shown" -eq 1 ] && printf '  classDef done fill:#d3f9d8;\n'
printf '```\n'
```

Key points: keying `REFERENCED` by the `pad`-normalized (4-digit) form makes the edge and the done-node lookup use the same canonical key; `${REFERENCED[...]:-}` satisfies `set -u`; the final `printf '```\n'` closes the fence unconditionally while `classDef` is now conditional.

- [ ] **Step 4: Run the test to verify the golden now matches**

Run: `bash tests/test_render_board.sh`
Expected: PASS — `"rendered output matches the golden byte-for-byte"` and the idempotence assert are both `ok`.

- [ ] **Step 5: Update the contract paragraph in `scripts/render-board.md`**

Replace the **Dependency graph (Mermaid)** paragraph. Current text:

```
**Dependency graph (Mermaid).** After the active sections, emits a fenced `mermaid` block with a
`graph TD`. Each active change is a node (ID padded to four digits). Changes with `depends_on:`
emit `PARENT --> CHILD` edges; standalone changes emit a bare node. Done changes from archive are
listed with `:::done` and a `classDef done fill:#d3f9d8;` rule. Killed archive entries are omitted
from the graph.
```

Replace with:

```
**Dependency graph (Mermaid).** After the active sections, emits a fenced `mermaid` block with a
`graph TD`. Each active change is a node (ID padded to four digits). Changes with `depends_on:`
emit `PARENT --> CHILD` edges; standalone changes emit a bare node. A done change from the archive
is styled `:::done` **only when an active change's `depends_on` references it** (so it already
appears as an edge parent); unreferenced done ids — floating, edgeless nodes — are dropped. The
`classDef done fill:#d3f9d8;` rule is emitted only when at least one `:::done` node remains. Killed
archive entries are omitted from the graph. This pruning is universal: it changes every board that
has an unreferenced done id, not only large archives.
```

- [ ] **Step 6: Commit**

```bash
git add scripts/render-board.sh scripts/render-board.md tests/test_render_board.sh
git commit -m "feat(0093): prune mermaid done nodes to depends_on-referenced only"
```

---

### Task 2: Archive recency window + per-month digest for older done

**Files:**
- Modify: `scripts/render-board.sh` (add the `ARCHIVE_RECENT` constant near the top; rewrite the `# --- archive ---` block at the end of the script)
- Modify: `scripts/render-board.md` (the **Archive section** behavior paragraph)
- Test: `tests/test_render_board.sh` (add a dedicated large-archive fixture with content + determinism assertions)

**Interfaces:**
- Consumes: `ARCFILES` array, `ARC_COUNT[done]` / `ARC_COUNT[killed]`, `field FILE status`, `field FILE title`, `int_field FILE id`, `pad ID` — all already present.
- Produces: unchanged signature. The archive `<details>` now renders a verbatim window plus, when older done exist, an "Older done (collapsed)" `| Month | Done |` digest.

**Design note (intermediate state is buildable):** this task is a self-contained rewrite of the one archive block. After Task 1 the suite is green; this task keeps the existing golden green (its 3-entry archive is inert — table byte-identical, no digest) and adds a new fixture. The archive-table window and the mermaid pruning from Task 1 are orthogonal: a done id can be collapsed in the archive table yet still styled in the mermaid if an active change depends on it (they answer different questions).

- [ ] **Step 1: Write the failing large-archive fixture test**

In `tests/test_render_board.sh`, immediately **before** the final block:

```bash
if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
```

insert this new fixture + assertions (it nests under `$tmp` so the existing `trap 'rm -rf "$tmp"' EXIT` cleans it — do NOT add a second `trap`, which would replace the first):

```bash
# ── large-archive fixture: recency window + per-month done digest (change 0093) ───────────────
# 18 done across three months + 2 killed; one active change depends on a done id (0060).
# Window is 15, so the 12 June + 3 May done render verbatim and the 3 April done collapse.
big="$tmp/big"; mkdir -p "$big/active" "$big/archive"

cat > "$big/active/0100-mainline.md" <<'EOF'
---
id: 100
slug: mainline
title: Mainline
status: in-progress
priority: high
depends_on: [60]
spec: docs/superpowers/specs/2026-07-01-mainline.md
branch: feat/mainline
EOF

mkarc(){ # mkarc DATE ID STATUS  -> writes $big/archive/<DATE>-<PADID>-c<ID>.md
  local d="$1" id="$2" st="$3"
  printf -- '---\nid: %s\nslug: c%s\ntitle: Change %s\nstatus: %s\npriority: medium\ndepends_on: []\n---\n' \
    "$id" "$id" "$id" "$st" > "$big/archive/${d}-$(printf '%04d' "$id")-c${id}.md"
}

d=1; for id in $(seq 60 71); do mkarc "$(printf '2026-06-%02d' "$d")" "$id" done; d=$((d+1)); done
mkarc 2026-05-01 50 done; mkarc 2026-05-02 51 done; mkarc 2026-05-03 52 done
mkarc 2026-04-01 40 done; mkarc 2026-04-02 41 done; mkarc 2026-04-03 42 done
mkarc 2026-06-20 90 killed   # newest row overall — verbatim, never collapses
mkarc 2026-03-15 30 killed   # oldest row overall — verbatim, never collapses

big_out="$tmp/big-out.md"
bash "$SCRIPT" --changes-dir "$big" --repo o/r > "$big_out" 2>/dev/null
brc=$?
assert "big: large-archive render exits 0" '[ "$brc" -eq 0 ]'

# (a) verbatim window — every killed row (any age) + the 15 most-recent done, listed individually.
assert "big: newest killed (0090) verbatim" 'grep -qF -- "| [0090](archive/2026-06-20-0090-c90.md) | Change 90 | 2026-06-20 |" "$big_out"'
assert "big: oldest killed (0030) verbatim" 'grep -qF -- "| [0030](archive/2026-03-15-0030-c30.md) | Change 30 | 2026-03-15 |" "$big_out"'
assert "big: a recent done (0071) verbatim" 'grep -qF -- "| [0071](archive/2026-06-12-0071-c71.md) | Change 71 | 2026-06-12 |" "$big_out"'
assert "big: the 15th-newest done (0050) still verbatim" 'grep -qF -- "| [0050](archive/2026-05-01-0050-c50.md) | Change 50 | 2026-05-01 |" "$big_out"'

# (b) collapsed set — older done appear ONLY as a per-month digest row; killed never collapses.
assert "big: April done collapsed to a per-month digest row (3 done)" 'grep -qF -- "| [2026-04](archive/) | 3 done |" "$big_out"'
assert "big: the Older-done sub-block header is present" 'grep -qF -- "**Older done (collapsed)**" "$big_out"'
assert "big: a collapsed done (0040) has NO verbatim archive row" '! grep -qF -- "archive/2026-04-01-0040-c40.md" "$big_out"'
assert "big: exactly one per-month digest row (only April collapses)" '[ "$(grep -cE "\(archive/\) \| [0-9]+ done \|" "$big_out")" -eq 1 ]'

# (c) mermaid — :::done only for the referenced done id (0060); every other done dropped.
assert "big: referenced done 0060 is styled :::done" 'grep -qxF -- "  0060:::done" "$big_out"'
assert "big: unreferenced verbatim done 0071 NOT in the mermaid" '! grep -qxF -- "  0071:::done" "$big_out"'
assert "big: unreferenced collapsed done 0040 NOT in the mermaid" '! grep -qxF -- "  0040:::done" "$big_out"'
assert "big: exactly one :::done node in the graph" '[ "$(grep -cF -- ":::done" "$big_out")" -eq 1 ]'

# (d) determinism — a second render is byte-identical.
big_out2="$tmp/big-out2.md"
bash "$SCRIPT" --changes-dir "$big" --repo o/r > "$big_out2" 2>/dev/null
assert "big: re-render is byte-identical (determinism)" 'diff -u "$big_out" "$big_out2"'
```

- [ ] **Step 2: Run the test to verify the new assertions fail**

Run: `bash tests/test_render_board.sh`
Expected: FAIL — the un-windowed renderer lists every April done verbatim and emits no digest, so `"big: April done collapsed..."`, `"big: exactly one per-month digest row..."`, `"big: a collapsed done (0040) has NO verbatim archive row"`, and the mermaid `:::done` count/exclusion asserts report `NOT OK`.

- [ ] **Step 3: Add the `ARCHIVE_RECENT` constant to `scripts/render-board.sh`**

Directly after the `GIT="${GIT:-git}"` line near the top of the script, add:

```bash
# Count-based recency window over the archive: the archive table lists every killed entry plus the
# ARCHIVE_RECENT most-recent `done` entries verbatim; older `done` collapse into a per-month digest.
# Count-based (not time-based) keeps the renderer deterministic — same change files, identical bytes.
ARCHIVE_RECENT=15
```

- [ ] **Step 4: Rewrite the archive block in `scripts/render-board.sh`**

Replace the entire `# --- archive ---` block. The current block is:

```bash
# --- archive ---
ndone=${ARC_COUNT[done]:-0}; nkilled=${ARC_COUNT[killed]:-0}
if [ $(( ndone + nkilled )) -gt 0 ]; then
  em=""; lbl=""
  [ "$ndone" -gt 0 ] && { em+="✅"; lbl="done"; }
  [ "$nkilled" -gt 0 ] && { em+="🗑️"; [ -n "$lbl" ] && lbl="$lbl + killed" || lbl="killed"; }
  printf '\n<details><summary>%s Archive — %s (%d)</summary>\n\n' "$em" "$lbl" "$(( ndone + nkilled ))"
  printf '| # | Title | Merged |\n|---|-------|--------|\n'
  # sort archive rows: date desc, then id desc. Key = "<date>\t<id>\t<file>".
  while IFS=$'\t' read -r date id f; do
    [ -n "$id" ] || continue
    printf '| [%s](archive/%s) | %s | %s |\n' "$(pad "$id")" "$(basename "$f")" "$(field "$f" title)" "$date"
  done < <(
    for f in "${ARCFILES[@]}"; do
      base="$(basename "$f")"; d="${base:0:10}"; id="$(int_field "$f" id)"
      printf '%s\t%s\t%s\n' "$d" "$id" "$f"
    done | sort -t$'\t' -k1,1r -k2,2nr
  )
  printf '\n</details>\n'
fi
```

Replace it with:

```bash
# --- archive ---
ndone=${ARC_COUNT[done]:-0}; nkilled=${ARC_COUNT[killed]:-0}
if [ $(( ndone + nkilled )) -gt 0 ]; then
  em=""; lbl=""
  [ "$ndone" -gt 0 ] && { em+="✅"; lbl="done"; }
  [ "$nkilled" -gt 0 ] && { em+="🗑️"; [ -n "$lbl" ] && lbl="$lbl + killed" || lbl="killed"; }
  printf '\n<details><summary>%s Archive — %s (%d)</summary>\n\n' "$em" "$lbl" "$(( ndone + nkilled ))"
  printf '| # | Title | Merged |\n|---|-------|--------|\n'
  # Partition the date-desc / id-desc sorted rows: the verbatim window = every killed row (any age)
  # plus the first ARCHIVE_RECENT done rows in sort order (killed and recent done interleave by
  # date, unchanged shape); the collapsed set = older done rows, tallied into a per-YYYY-MM digest.
  # Killed never collapses. The status is carried in the sort tuple so the loop can partition
  # without re-reading the file. Sort keys (date field 1 desc, id field 2 num desc) are unchanged.
  # (change 0093)
  done_seen=0
  declare -A MONTH_DONE; month_order=()
  while IFS=$'\t' read -r date id st f; do
    [ -n "$id" ] || continue
    if [ "$st" = "done" ]; then
      done_seen=$(( done_seen + 1 ))
      if [ "$done_seen" -gt "$ARCHIVE_RECENT" ]; then
        ym="${date:0:7}"
        [ -n "${MONTH_DONE[$ym]:-}" ] || month_order+=("$ym")
        MONTH_DONE["$ym"]=$(( ${MONTH_DONE[$ym]:-0} + 1 ))
        continue
      fi
    fi
    printf '| [%s](archive/%s) | %s | %s |\n' "$(pad "$id")" "$(basename "$f")" "$(field "$f" title)" "$date"
  done < <(
    for f in "${ARCFILES[@]}"; do
      base="$(basename "$f")"; d="${base:0:10}"; id="$(int_field "$f" id)"; st="$(field "$f" status)"
      printf '%s\t%s\t%s\t%s\n' "$d" "$id" "$st" "$f"
    done | sort -t$'\t' -k1,1r -k2,2nr
  )
  if [ "${#month_order[@]}" -gt 0 ]; then
    printf '\n**Older done (collapsed)**\n\n'
    printf '| Month | Done |\n|-------|------|\n'
    for ym in "${month_order[@]}"; do
      printf '| [%s](archive/) | %d done |\n' "$ym" "${MONTH_DONE[$ym]}"
    done
  fi
  printf '\n</details>\n'
fi
```

Key points: the sort producer now emits a 4-field tuple (`date`, `id`, `status`, `file`) but the sort keys (`-k1,1r -k2,2nr`) are unchanged, so ordering is identical. `done_seen` counts only done rows (killed skip the counter and always print). `month_order` (an insertion-ordered indexed array) is built newest-first because the stream is date-desc, so the digest is deterministic and newest bucket first. The `${#month_order[@]} -gt 0` guard means the digest sub-block — and thus every byte after the verbatim table — is absent when nothing collapses, so a small archive renders byte-identically to the pre-change output.

- [ ] **Step 5: Run the full test to verify all assertions pass**

Run: `bash tests/test_render_board.sh`
Expected: PASS — the existing golden byte-compare and idempotence asserts stay `ok` (3-entry archive is inert), and every `big:` assert reports `ok`. Final line: `PASS`.

- [ ] **Step 6: Update the contract paragraph in `scripts/render-board.md`**

Replace the **Archive section** paragraph. Current text:

```
**Archive section.** If `archive/` contains any `*.md` files, emits a collapsible `<details>` block
with a `| # | Title | Merged |` table. Rows are sorted by merged date descending, then by ID
descending. The `#` cell links to `archive/<filename>`. The merged date is the first ten characters
of the archive filename (the `YYYY-MM-DD` prefix).
```

Replace with:

```
**Archive section.** If `archive/` contains any `*.md` files, emits a collapsible `<details>`
block. The `| # | Title | Merged |` table lists a **verbatim window** — every `killed` entry (any
age) plus the `ARCHIVE_RECENT` (default 15) most-recent `done` entries — sorted by merged date
descending, then by ID descending; the `#` cell links to `archive/<filename>` and the merged date
is the first ten characters of the filename (the `YYYY-MM-DD` prefix). `done` entries older than
the window are **not** listed individually: they collapse into an "Older done (collapsed)"
`| Month | Done |` digest, one row per `YYYY-MM` bucket (newest first, each linking to the
`archive/` directory), keeping the always-loaded board flat as the archive grows. `killed` never
collapses. When the archive's `done` count is at or below `ARCHIVE_RECENT`, no digest is emitted
and the **archive table is byte-identical to the pre-window renderer** — the window is inert until
it is needed. (The mermaid pruning above is separate and universal, not inert.) The window is
count-based, not time-based, so the renderer stays deterministic — same change files, identical
bytes.
```

- [ ] **Step 7: Run the whole test suite (not just this test) and commit**

Run the repo's full suite to catch any cross-test regression (e.g. board-refresh / docket-status tests that render the board), then commit:

```bash
bash tests/test_render_board.sh
# plus any repo-level runner, e.g.:  bash tests/run-all.sh   (use whatever the repo provides)
git add scripts/render-board.sh scripts/render-board.md tests/test_render_board.sh
git commit -m "feat(0093): archive recency window + per-month done digest"
```

Expected: all green.

---

## Self-Review

**Spec coverage:**
- Count-based recency window over `done`, constant `ARCHIVE_RECENT=15` → Task 2, Steps 3-4. ✔
- Per-month digest of older `done`, `| Month | Done |`, newest first, linking `archive/` → Task 2, Step 4 + assert (b). ✔
- Killed always verbatim, digest done-only → Task 2, Step 4 (killed skip the counter) + asserts. ✔
- Inert below threshold: archive table byte-identical for small archives → Task 2 (existing golden stays green) + Step 6 contract note. ✔
- Mermaid `:::done` only for depends_on-referenced done ids; `classDef` only when a node remains → Task 1. ✔
- Existing golden updated (drop `0012:::done`), not asserted unchanged → Task 1, Step 1. ✔
- Large-archive fixture with >15 done across months + killed, asserting (a)-(d) → Task 2, Step 1. ✔
- Contract updated: Archive section + Dependency graph paragraphs → Task 1 Step 5, Task 2 Step 6. ✔
- No config knob, no new file, no caller change, no archived record touched → honored throughout. ✔
- Out of scope (GitHub mirror, learnings index decay, ARCHIVE.md, analytics) → untouched. ✔

**Placeholder scan:** none — every code step shows complete code; every run step shows the command and expected result.

**Type consistency:** `REFERENCED` keyed by `pad`-form in both write and read (Task 1); `MONTH_DONE`/`month_order` and `done_seen` used consistently (Task 2); the 4-field sort tuple's keys match the unchanged `-k1,1r -k2,2nr`. `ARCHIVE_RECENT` defined once (Task 2 Step 3), read in Task 2 Step 4.
