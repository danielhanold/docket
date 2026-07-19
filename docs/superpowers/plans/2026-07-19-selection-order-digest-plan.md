# Selection-order backlog digest Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give `docket-implement-next` a deterministic, single-read selection queue by adding one `ready` line to `render-board.sh --format digest` and a write-free `docket-status.sh --digest-only` entry point to reach it.

**Architecture:** Three layers, bottom-up. (1) `render-board.sh`'s existing digest projection gains one `ready <id> <id> …` line — build-ready ids in selection order — computed from the readiness pass it already runs, so no new readiness logic and no new file reads. (2) `docket-status.sh` gains `--digest-only`, a path that resolves config and runs the existing `backlog_pass`, then exits — no preflight sync, no sweep, no board render, no commit, no push. (3) `skills/docket-implement-next/SKILL.md` Step 1 takes its ordered candidate set from the `ready` line, confirming build-readiness by reading the one change file before claiming.

**Tech Stack:** Bash 3.2-compatible shell (macOS/BSD + GNU), the repo's hand-rolled `assert`-based test scripts under `tests/`, markdown contracts under `scripts/*.md`.

## Global Constraints

- **Shell portability:** Bash 3.2 / BSD userland compatible. No `declare -A` additions outside existing patterns, no GNU-only `sort`/`sed`/`date` flags. (See the promoted `shell-portability` finding.)
- **`set -uo pipefail` is already active in both scripts** — every new pipeline must tolerate it; never `cmd | grep` where a SIGPIPE can fire.
- **`render-board.sh` determinism is load-bearing:** same change files ⇒ identical bytes. **No wall-clock reads** in the new code — `priority` and `created` are static frontmatter.
- **The markdown projection must stay byte-identical.** The `ready` line lives inside the `if [ "$FORMAT" = digest ]` block, which `exit 0`s before the markdown emission.
- **The digest is report output, never a board surface.** It is never persisted, committed, or written to `BOARD.md`.
- **Readiness has exactly one owner:** `readiness()` in `scripts/lib/docket-frontmatter.sh`, reached via `digest_readiness()`. Do not reimplement or re-derive it.
- **Selection order is `priority` (`critical` > `high` > `medium` > `low`) → `created` (ascending) → `id` (ascending).** An unset or unrecognized `priority` is `medium` (the convention's default).
- **The `ready` line is ALWAYS emitted** — bare (`ready`, no ids) when the queue is empty. Absence means *no queue was produced*, never *nothing is ready*.

---

### Task 1: The `ready` line in `render-board.sh --format digest`

**Files:**
- Modify: `scripts/render-board.sh` (header comment block lines 7–14; the digest block ending at the `exit 0` around line 135)
- Modify: `scripts/render-board.md` (usage table row + the "Digest projection" section around lines 99–115)
- Test: `tests/test_render_board.sh` (digest battery starting at the `--- change 0069: --format digest` banner, ~line 1451)

**Interfaces:**
- Consumes: `digest_readiness FILE ID STATUS` (existing, in `scripts/render-board.sh`), which returns the exact token `build-ready` for a build-ready `proposed` change; `rows_sorted STATUS` (existing) emitting `id<TAB>file` lines; `field FILE KEY` (existing, from `scripts/lib/docket-frontmatter.sh`).
- Produces: a final digest line matching `^ready( [0-9]+)*$`, emitted **after** all `change …` lines, consumed by Task 2's `--digest-only` path and Task 3's skill Step 1.

- [ ] **Step 1: Write the failing tests**

Open `tests/test_render_board.sh`. Find the digest golden block — it begins with the line `digest_golden="$tmp/digest-golden.txt"` and the heredoc `cat > "$digest_golden" <<'EOF'`. Add `ready 2` as the **last line of the heredoc**, immediately after `change 13 implemented finalize-blocked mike`, so the block reads:

```bash
digest_golden="$tmp/digest-golden.txt"
cat > "$digest_golden" <<'EOF'
backlog in-progress 1
backlog proposed 5
backlog blocked 1
backlog deferred 1
backlog implemented 2
backlog done 2
backlog killed 1
change 1 in-progress - alpha
change 2 proposed build-ready bravo
change 3 proposed needs-brainstorm charlie
change 4 proposed auto-groom-blocked delta
change 5 proposed waiting-on-3-unbuilt echo
change 6 proposed waiting-on-8-needs-merge foxtrot
change 7 blocked - golf
change 8 implemented - hotel
change 9 deferred - india
change 13 implemented finalize-blocked mike
ready 2
EOF
```

(The main fixture has exactly one build-ready change, id 2 — the other four `proposed` changes are `needs-brainstorm`, `auto-groom-blocked`, and two `waiting`.)

Then append the new `ready` battery. Put it directly after the existing assert block `(f) archive rollups only …` and before `(g) an unknown --format value …`:

```bash
# --- change 0094: the `ready` line (build-ready ids in selection order) ---
# The `ready` line is a SECOND consumer of the same readiness pass: its membership is exactly the
# set of `change … proposed build-ready …` lines, so it can never disagree with them. What it adds
# is ORDER — priority > created > id — which the id-ascending `change` lines deliberately do not
# carry.

# (i) position + shape: `ready` is the LAST line, and it lists exactly the build-ready ids.
assert "ready is the final digest line" \
  '[ "$(tail -n 1 "$digest_out")" = "ready 2" ]'
assert "ready line matches the ready grammar" \
  'grep -qE "^ready( [0-9]+)*$" "$digest_out"'
assert "exactly one ready line is emitted" \
  '[ "$(grep -c "^ready" "$digest_out")" -eq 1 ]'

# (ii) membership parity with the change lines — the anti-disagreement invariant. Derive the
#      expected set from the `change` lines themselves rather than restating it, so a readiness
#      change that moves a `change` line moves this assert with it.
exp_ready="$(awk '$2=="proposed" && $3=="build-ready" {print $1}' <(grep "^change " "$digest_out") | sort -n | tr '\n' ' ')"
got_ready="$(sed -n 's/^ready //p' "$digest_out" | tr -s ' ' | sed 's/ *$//' | tr ' ' '\n' | sort -n | tr '\n' ' ')"
assert "ready membership equals the build-ready change lines" '[ "$exp_ready" = "$got_ready" ]'

# (iii) ORDERING, on a dedicated fixture. Three bands prove the three sort keys independently.
ord="$tmp/ord"; mkdir -p "$ord/active" "$ord/archive"
# id 30: medium, oldest  -> age beats id (before 31)
# id 31: medium, newest  -> loses on age
# id 32: critical, newest -> priority beats age AND id (first overall)
# id 33: high, newest     -> second overall
# id 34: low, oldest      -> last, despite being the oldest (priority outranks age)
# id 35: (no priority:)   -> defaults to medium; same created as 30 -> tie falls to LOWEST id (30 < 35)
write_ord(){ # write_ord ID PRIORITY CREATED SLUG
  local pri_line="priority: $2"
  [ -n "$2" ] || pri_line=""
  cat > "$ord/active/00$1-$4.md" <<EOF
---
id: $1
slug: $4
title: $4
status: proposed
$pri_line
created: $3
updated: $3
depends_on: []
spec: docs/superpowers/specs/x.md
trivial: false
---

## Why
x
EOF
}
write_ord 30 medium   2026-01-01 mike
write_ord 31 medium   2026-06-01 november
write_ord 32 critical 2026-06-01 oscar
write_ord 33 high     2026-06-01 papa
write_ord 34 low      2026-01-01 quebec
write_ord 35 ""       2026-01-01 romeo

ord_out="$tmp/ord-digest.txt"
bash "$SCRIPT" --changes-dir "$ord" --format digest > "$ord_out" 2>/dev/null
assert "ordering fixture: exact selection order (priority > created > id)" \
  '[ "$(sed -n "s/^ready //p" "$ord_out")" = "32 33 30 35 31 34" ]'
assert "ordering: critical outranks an older medium"  '[ "$(sed -n "s/^ready //p" "$ord_out" | cut -d" " -f1)" = "32" ]'
assert "ordering: low sorts last despite being oldest" '[ "$(sed -n "s/^ready //p" "$ord_out" | awk "{print \$NF}")" = "34" ]'
assert "ordering: an absent priority: defaults to medium (35 sits in the medium band)" \
  '[ "$(sed -n "s/^ready //p" "$ord_out" | cut -d" " -f4)" = "35" ]'
assert "ordering: exact tie (same priority+created) falls to the LOWEST id" \
  '[ "$(sed -n "s/^ready //p" "$ord_out" | cut -d" " -f3,4)" = "30 35" ]'

# (iv) TOTALITY — the empty queue emits a BARE `ready`, never no line (sole-channel lesson: a
#      missing line must mean "no queue was produced", never "nothing is ready").
mt="$tmp/empty-ready"; mkdir -p "$mt/active" "$mt/archive"
cat > "$mt/active/0040-sierra.md" <<'EOF'
---
id: 40
slug: sierra
title: sierra
status: proposed
priority: medium
created: 2026-01-01
updated: 2026-01-01
depends_on: []
trivial: false
---

## Why
no spec, not trivial -> needs-brainstorm, so the ready queue is EMPTY
EOF
mt_out="$tmp/empty-ready-digest.txt"
bash "$SCRIPT" --changes-dir "$mt" --format digest > "$mt_out" 2>/dev/null
assert "empty build-ready set still emits a ready line" 'grep -q "^ready$" "$mt_out"'
assert "the empty ready line is bare (no trailing space)" '[ "$(grep "^ready" "$mt_out")" = "ready" ]'
assert "empty-queue digest still reports the change line" \
  'grep -qxF "change 40 proposed needs-brainstorm sierra" "$mt_out"'

# (v) a wholly empty backlog still emits the ready line (stdout is never empty — change 0069).
none="$tmp/no-changes"; mkdir -p "$none/active" "$none/archive"
none_out="$tmp/no-changes-digest.txt"
bash "$SCRIPT" --changes-dir "$none" --format digest > "$none_out" 2>/dev/null
assert "wholly empty backlog still emits a bare ready line" '[ "$(cat "$none_out")" = "ready" ]'

# (vi) determinism — the digest (ready line included) is byte-stable across runs.
ord_out2="$tmp/ord-digest2.txt"
bash "$SCRIPT" --changes-dir "$ord" --format digest > "$ord_out2" 2>/dev/null
assert "digest with a ready line is byte-identical across runs" 'diff -u "$ord_out" "$ord_out2"'

# (vii) the ready line does NOT leak into the markdown projection.
ord_md="$tmp/ord-board.md"
bash "$SCRIPT" --changes-dir "$ord" --format markdown > "$ord_md" 2>/dev/null
assert "markdown projection carries no ready line" '! grep -q "^ready" "$ord_md"'
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `bash tests/test_render_board.sh 2>&1 | grep "NOT OK"`

Expected: FAIL — several `NOT OK` lines, including `NOT OK - --format digest matches the digest golden byte-for-byte` (the golden now has a `ready 2` line the renderer does not emit), `NOT OK - ready is the final digest line`, and `NOT OK - empty build-ready set still emits a ready line`.

- [ ] **Step 3: Implement the `ready` line**

In `scripts/render-board.sh`, inside the `if [ "$FORMAT" = digest ]; then` block, insert the new code **between** the `done < <( … )` that closes the `change`-line loop and the block's `exit 0`. The block's tail becomes:

```bash
  done < <(
    for st in in-progress proposed blocked deferred implemented; do
      rows_sorted "$st"
    done | sort -t$'\t' -k1,1n
  )
  # --- the `ready` line (change 0094) -------------------------------------------------------
  # The build-ready QUEUE, in the convention's deterministic selection order: priority
  # (critical > high > medium > low) -> created (ascending) -> id (ascending). Membership is
  # exactly the set digest_readiness() already reported as `build-ready`, so this line can never
  # disagree with the `change` lines above; what it adds is ORDER, which those id-ascending lines
  # deliberately do not carry. Both sort keys are STATIC frontmatter — no wall-clock read — so the
  # renderer stays deterministic and the golden byte-compare holds.
  #
  # ALWAYS EMITTED, bare when the queue is empty: absence of this line means NO QUEUE WAS PRODUCED
  # (an older render-board, or a render failure), never "nothing is ready". A consumer that cannot
  # tell those apart has merely moved the silence somewhere quieter.
  ready_ids=""
  while IFS=$'\t' read -r rid; do
    [ -n "$rid" ] || continue
    ready_ids="$ready_ids $rid"
  done < <(
    while IFS=$'\t' read -r id f; do
      [ -n "$id" ] || continue
      [ "$(digest_readiness "$f" "$id" proposed)" = build-ready ] || continue
      # An unset or unrecognized priority is `medium` — the convention's documented default.
      case "$(field "$f" priority)" in
        critical) prank=0 ;;
        high)     prank=1 ;;
        low)      prank=3 ;;
        *)        prank=2 ;;
      esac
      printf '%s\t%s\t%s\n' "$prank" "$(field "$f" created)" "$id"
    done < <(rows_sorted proposed) | sort -t$'\t' -k1,1n -k2,2 -k3,3n | cut -f3
  )
  printf 'ready%s\n' "$ready_ids"
  exit 0
fi
```

Notes for the implementer:
- `rows_sorted proposed` already restricts the loop to `proposed` changes, so `digest_readiness` is called with the literal `proposed` and its `build-ready` return is the whole filter.
- `sort -t$'\t' -k1,1n -k2,2 -k3,3n` sorts priority numerically, `created` lexicographically (ISO dates sort correctly as text), then id numerically. `cut -f3` keeps only the id.
- `printf 'ready%s\n' "$ready_ids"` yields a bare `ready` when `ready_ids` is empty, and `ready 32 33 …` otherwise, because each id was accumulated with a leading space.

- [ ] **Step 4: Update the script header comment**

In `scripts/render-board.sh`, extend the `--format` line of the usage block (lines 10–14) so the digest's documented shape includes the new line:

```bash
#   --format markdown (default) emits the BOARD.md markdown; --format digest emits the
#   line-oriented backlog digest (`backlog <status> <count>` + `change <id> <status> <readiness>
#   <slug>` + a final `ready <id> …` queue line, change 0094) — a second projection of the same
#   dependency/readiness pass, consumed by docket-status.sh's report. The digest is REPORT OUTPUT,
#   NOT a board surface: it is never persisted, committed, or written to BOARD.md.
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `bash tests/test_render_board.sh 2>&1 | tail -5; bash tests/test_render_board.sh 2>&1 | grep -c "NOT OK"`

Expected: PASS — the `NOT OK` count is `0`.

- [ ] **Step 6: Update the contract doc**

In `scripts/render-board.md`:

Update the `--format` row of the flags table (~line 30) to read:

```markdown
| `--format markdown\|digest` | no | Output projection. `markdown` (default) emits the board. `digest` emits the line-oriented backlog digest (change 0069) plus its trailing `ready` queue line (change 0094). Any other value is an argument error (exit 2). |
```

In the **Digest projection (`--format digest`)** section (~lines 99–115), after the existing description of the emitted lines, add:

```markdown
**The `ready` line (change 0094).** The digest's final line is always `ready [<id> …]` — the
**build-ready queue in selection order**: `priority` (`critical` > `high` > `medium` > `low`) →
`created` (ascending) → `id` (ascending), the convention's *Build-readiness & selection* order. An
unset or unrecognized `priority` is treated as `medium`.

Its membership is exactly the set of changes the `change` lines report as `proposed build-ready`,
because it is computed from the same `digest_readiness()` call — the line can never disagree with
them. What it adds is **order**, which the id-ascending `change` lines deliberately do not carry.
Both sort keys are static frontmatter, so the renderer performs no wall-clock read and stays
deterministic.

The line is **always emitted**, bare (`ready`, no ids) when nothing is build-ready. Absence of a
`ready` line therefore means **no queue was produced** — an older `render-board.sh`, or a failed
render — and never "nothing is ready". Consumers must treat the two cases differently:
`docket-implement-next` Step 1 falls back to walking `active/` on absence, but reports `drained` on
a bare line.
```

- [ ] **Step 7: Run the full suite and commit**

Run: `bash tests/test_render_board.sh 2>&1 | grep -c "NOT OK"` (expect `0`)

```bash
git add scripts/render-board.sh scripts/render-board.md tests/test_render_board.sh
git commit -m "feat(0094): emit a selection-order \`ready\` line in the render-board digest"
```

---

### Task 2: `docket-status.sh --digest-only` — the write-free entry point

**Files:**
- Modify: `scripts/docket-status.sh` (usage header lines 7–16; the flag-parsing `while` loop ~lines 29–42; `main()` at the end of the file)
- Modify: `scripts/docket-status.md` (usage synopsis + flags table ~lines 16–29; the report-line table ~line 311)
- Test: `tests/test_docket_status.sh` (append a new battery at the end of the file, before the final exit/summary block)

**Interfaces:**
- Consumes: `backlog_pass` (existing, in `scripts/docket-status.sh`) — runs `render-board.sh --format digest` and prints its lines, including Task 1's `ready` line; `docket_metadata_worktree` (existing, from `scripts/lib/docket-root.sh`, which anchors the path itself); the `CONFIG_EXPORT_CMD` mock seam.
- Produces: `docket-status.sh --digest-only`, reachable from a skill as `docket.sh docket-status --digest-only` (the facade already `exec`s wrapped ops with `"$@"`, so **no `docket.sh` change is needed**). Emits `backlog …`, `change …`, and `ready …` lines and **no `board …` line**; exits 0.

- [ ] **Step 1: Write the failing tests**

Append this battery to `tests/test_docket_status.sh`, immediately before the file's final `exit $fail` line:

```bash
# ── change 0094: --digest-only, the write-free selection read ────────────────────────────────
# The digest is how docket-implement-next Step 1 acquires its ordered candidate set. It is a READ:
# it must not sync, commit, push, render the board, or move HEAD. --board-only was not reusable —
# it commits and pushes BOARD.md, and a selection read must not be a write.

dg="$tmp/digest-only"; mkdir -p "$dg/work/.docket/docs/changes/active" "$dg/work/.docket/docs/changes/archive"
cat > "$dg/work/.docket/docs/changes/active/0050-tango.md" <<'EOF'
---
id: 50
slug: tango
title: tango
status: proposed
priority: high
created: 2026-02-01
updated: 2026-02-01
depends_on: []
spec: docs/superpowers/specs/t.md
trivial: false
---

## Why
x
EOF
cat > "$dg/work/.docket/docs/changes/active/0051-uniform.md" <<'EOF'
---
id: 51
slug: uniform
title: uniform
status: proposed
priority: critical
created: 2026-03-01
updated: 2026-03-01
depends_on: []
spec: docs/superpowers/specs/u.md
trivial: false
---

## Why
x
EOF

cat > "$tmp/fixture-digest.sh" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' \
  'BOOTSTRAP=PROCEED' \
  'METADATA_BRANCH=docket' \
  'INTEGRATION_BRANCH=main' \
  'DOCKET_MODE=docket' \
  'METADATA_WORKTREE=.docket' \
  'CHANGES_DIR=docs/changes' \
  'ADRS_DIR=docs/adrs' \
  'RESULTS_DIR=docs/results' \
  'BOARD_SURFACES=inline'
EOF

# A git stub that RECORDS every invocation, so "it never synced" is proven by evidence rather than
# by the absence of a visible symptom.
cat > "$tmp/spy-git.sh" <<EOF
#!/usr/bin/env bash
printf '%s\n' "\$*" >> "$tmp/git-calls.txt"
exit 0
EOF
chmod +x "$tmp/spy-git.sh"
: > "$tmp/git-calls.txt"

dg_out="$tmp/digest-only-out.txt"
(cd "$dg/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-digest.sh" GIT="$tmp/spy-git.sh" \
  "$SCRIPT" --digest-only >"$dg_out" 2>"$tmp/digest-only-err.txt")
dgrc=$?

assert "--digest-only exits 0" '[ "$dgrc" -eq 0 ]'
assert "--digest-only emits the ready line"    'grep -qE "^ready( [0-9]+)*$" "$dg_out"'
assert "--digest-only emits change lines"      'grep -q "^change 50 proposed build-ready tango" "$dg_out"'
assert "--digest-only emits backlog rollups"   'grep -q "^backlog proposed 2" "$dg_out"'
assert "--digest-only ready order is critical-first (51 before 50)" \
  '[ "$(sed -n "s/^ready //p" "$dg_out")" = "51 50" ]'

# The load-bearing half: it is a READ.
assert "--digest-only emits NO board line" '! grep -q "^board " "$dg_out"'
assert "--digest-only writes no BOARD.md" '[ ! -e "$dg/work/.docket/docs/changes/BOARD.md" ]'
assert "--digest-only never invokes git at all (no fetch/pull/commit/push)" \
  '[ ! -s "$tmp/git-calls.txt" ]'
assert "--digest-only emits no pass ok (it is not a pass)" '! grep -q "^pass ok" "$dg_out"'
assert "--digest-only stdout is non-empty" '[ -s "$dg_out" ]'

# Mutual exclusion: the two flags are opposite postures (a read vs. a committing write).
(cd "$dg/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-digest.sh" GIT="$tmp/spy-git.sh" \
  "$SCRIPT" --digest-only --board-only >/dev/null 2>"$tmp/dg-both-err.txt")
bothrc=$?
assert "--digest-only with --board-only exits 2" '[ "$bothrc" -eq 2 ]'
assert "the mutual-exclusion error names both flags" \
  'grep -q -- "--digest-only" "$tmp/dg-both-err.txt" && grep -q -- "--board-only" "$tmp/dg-both-err.txt"'
# Order-independence: a flag pair that only rejects in one order is a half-closed gate.
# NB: capture the status into a NAMED variable. `assert ... '[ "$?" -eq 2 ]'` would evaluate `$?`
# inside assert's own eval, where it reports the previous command in THAT scope — the assert would
# be measuring itself and would pass for the wrong reason.
(cd "$dg/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-digest.sh" GIT="$tmp/spy-git.sh" \
  "$SCRIPT" --board-only --digest-only >/dev/null 2>/dev/null)
revrc=$?
assert "--board-only with --digest-only also exits 2 (order-independent)" '[ "$revrc" -eq 2 ]'

# Totality on an EMPTY backlog: stdout is still non-empty and the ready line is still there.
dge="$tmp/digest-empty"; mkdir -p "$dge/work/.docket/docs/changes/active" "$dge/work/.docket/docs/changes/archive"
dge_out="$tmp/digest-empty-out.txt"
(cd "$dge/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-digest.sh" GIT="$tmp/spy-git.sh" \
  "$SCRIPT" --digest-only >"$dge_out" 2>/dev/null)
assert "--digest-only on an empty backlog still emits a bare ready line" \
  '[ "$(cat "$dge_out")" = "ready" ]'

# Bootstrap stays fail-closed on this path too — a read must not silently report an empty backlog
# for a repo that was never migrated.
cat > "$tmp/fixture-digest-stop.sh" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' \
  'BOOTSTRAP=STOP_MIGRATE' \
  'METADATA_BRANCH=docket' \
  'INTEGRATION_BRANCH=main' \
  'DOCKET_MODE=docket' \
  'METADATA_WORKTREE=.docket' \
  'CHANGES_DIR=docs/changes' \
  'ADRS_DIR=docs/adrs' \
  'RESULTS_DIR=docs/results' \
  'BOARD_SURFACES=inline'
EOF
(cd "$dg/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-digest-stop.sh" GIT="$tmp/spy-git.sh" \
  "$SCRIPT" --digest-only >"$tmp/dg-stop-out.txt" 2>"$tmp/dg-stop-err.txt")
stoprc=$?
assert "--digest-only is fail-closed on a non-PROCEED bootstrap verdict" '[ "$stoprc" -ne 0 ]'
assert "--digest-only emits no ready line when the bootstrap gate rejects" \
  '! grep -q "^ready" "$tmp/dg-stop-out.txt"'

# --help documents the flag (the skill's author has to be able to find it).
assert "--help mentions --digest-only" '"$SCRIPT" --help 2>&1 | grep -q -- "--digest-only"'
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `bash tests/test_docket_status.sh 2>&1 | grep "NOT OK"`

Expected: FAIL — `NOT OK - --digest-only exits 0` (the flag is currently an unknown argument, exit 2), plus the whole battery below it.

- [ ] **Step 3: Add the flag and the mutual-exclusion gate**

In `scripts/docket-status.sh`, update the usage header (the `# Usage:` block, lines 7–16) to:

```bash
# Usage: docket-status.sh [--board-only] [--digest-only] [--must-land] [--repo OWNER/REPO]
#                          [--project OWNER/NUMBER] [--auto-create-project] [--project-owner OWNER]
#   --board-only           only regenerate the board surfaces; skip sweep/health passes
#   --digest-only          WRITE-FREE READ (change 0094): resolve config, emit the backlog digest
#                          (rollups + `change` lines + the `ready` queue line) and exit. No worktree
#                          sync, no sweep, no health checks, no board render, no commit, no push,
#                          and no `board …` line. Mutually exclusive with --board-only.
#   --must-land            (with --board-only) retry a push-failed board write in-script and
#                          map the outcome to the exit code (0 = board landed); see docket-status.md
```

Then update the flag parser. Change the `BOARD_ONLY=0 MUST_LAND=0 …` initializer line to add `DIGEST_ONLY=0`, add the case arm, and add the exclusion check right after the `while` loop:

```bash
BOARD_ONLY=0 DIGEST_ONLY=0 MUST_LAND=0 REPO_FLAG="" PROJECT_FLAG="" AUTO_CREATE_PROJECT=0 PROJECT_OWNER=""
usage(){ sed -n '2,18p' "${BASH_SOURCE[0]}"; }
while [ $# -gt 0 ]; do
  case "$1" in
    --board-only) BOARD_ONLY=1 ;;
    --digest-only) DIGEST_ONLY=1 ;;
    --must-land) MUST_LAND=1 ;;
    --repo) REPO_FLAG="$2"; shift ;;
    --project) PROJECT_FLAG="$2"; shift ;;
    --auto-create-project) AUTO_CREATE_PROJECT=1 ;;
    --project-owner) PROJECT_OWNER="$2"; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "docket-status: unknown argument: $1" >&2; exit 2 ;;
  esac; shift
done
# Opposite postures: --digest-only is a write-free READ, --board-only commits and pushes BOARD.md.
# Rejected in BOTH orders — a gate that only closes one way is not a gate.
if [ "$DIGEST_ONLY" = 1 ] && [ "$BOARD_ONLY" = 1 ]; then
  echo "docket-status: --digest-only and --board-only are mutually exclusive (a write-free read vs. a committing board write)" >&2
  exit 2
fi
```

**Note the `usage()` range change** from `'2,15p'` to `'2,18p'` — the header grew by three lines, and the `--help` assert reads it.

- [ ] **Step 4: Add the `digest_only_pass` function**

In `scripts/docket-status.sh`, add this function immediately **after** `backlog_pass` (whose closing `}` precedes the `# detect_merged` banner):

```bash
# digest_only_pass — the write-free selection read (change 0094). docket-implement-next Step 1
# acquires its ordered candidate set here, so this path must be a READ in the strict sense: it
# resolves config and runs the backlog pass, and does nothing else.
#
# It deliberately does NOT call docket_preflight. Preflight FETCHES AND `pull --rebase`s the
# metadata worktree — a working-tree mutation that can move HEAD, which would make a "read" a
# write. That costs nothing here: the calling skill runs this AFTER its own Step-0 preflight, so
# the tree is already freshly synced and a second sync would be pure redundancy. The digest is a
# snapshot of the change files as it finds them, which is exactly the contract Step 1 wants.
#
# The bootstrap verdict is still enforced FAIL-CLOSED. A repo that was never migrated has no
# metadata worktree, and reporting a cheerful empty backlog for it would hand the selector a
# `ready` line meaning "nothing is ready" when the truth is "this repo is not set up" — the exact
# two-cases-one-signal collapse the always-emitted ready line exists to prevent.
digest_only_pass(){
  local cfg
  cfg="$(${CONFIG_EXPORT_CMD:-"$SCRIPTS_DIR"/docket-config.sh --export})" \
    || { echo "docket-status: config export failed" >&2; return 1; }
  eval "$cfg"
  case "${BOOTSTRAP:-}" in
    PROCEED) : ;;
    STOP_MIGRATE)  echo "docket-status: repo not migrated — run migrate-to-docket.sh" >&2; return 1 ;;
    CREATE_ORPHAN) echo "docket-status: fresh repo — run docket.sh bootstrap to create the docket branch" >&2; return 1 ;;
    *) echo "docket-status: unknown bootstrap verdict '${BOOTSTRAP:-}'" >&2; return 1 ;;
  esac
  backlog_pass
}
```

- [ ] **Step 5: Wire it into `main()`**

In `scripts/docket-status.sh`, make `digest_only_pass` the **first thing `main()` does** — before `docket_preflight`, so the read path never reaches the sync:

```bash
main(){
  # change 0094: the write-free read short-circuits BEFORE docket_preflight (which syncs).
  if [ "$DIGEST_ONLY" = 1 ]; then
    digest_only_pass || exit 1
    exit 0
  fi
  docket_preflight "$SCRIPTS_DIR" || exit 1
  if [ "$MUST_LAND" = 1 ]; then
```

Leave the rest of `main()` untouched.

- [ ] **Step 6: Run the tests to verify they pass**

Run: `bash tests/test_docket_status.sh 2>&1 | grep -c "NOT OK"`

Expected: PASS — `0`.

- [ ] **Step 7: Update the contract doc**

In `scripts/docket-status.md`:

Update the usage synopsis (~line 16):

```
docket-status.sh [--board-only] [--digest-only] [--must-land] [--repo OWNER/REPO]
                  [--project OWNER/NUMBER] [--auto-create-project] [--project-owner OWNER]
docket-status.sh -h | --help
```

Add a flags-table row directly after the `--board-only` row:

```markdown
| `--digest-only` | **Write-free read** (change 0094). Resolve config, enforce the bootstrap verdict fail-closed, emit the backlog digest — `backlog` rollups, `change` lines, and the trailing `ready` queue line — and exit 0. Runs **no** metadata-worktree sync (it does not call `docket_preflight`), no sweep, no health checks, no learnings pass, no board render, no commit and no push, and emits **no `board …` line** and no `pass ok`. Mutually exclusive with `--board-only` (exit 2, in either order): a selection read must not be a write. This is the entry point `docket-implement-next` Step 1 uses to acquire its ordered candidate set. |
```

Add a report-line table row after the `change <id> …` row (~line 312):

```markdown
| `ready [<id> …]` | The **build-ready queue in selection order** (`priority` → `created` → `id`), from `render-board.sh`'s digest (change 0094). Emitted on every path that runs the backlog pass — the full report, `--board-only`, and `--digest-only`. **Always present**, bare when nothing is build-ready; its absence means no queue was produced (an older `render-board.sh`, or a failed render), never "nothing is ready". Membership always equals the `change` lines reporting `proposed build-ready`. |
```

Finally, add a short section documenting the path. Put it directly after the "**4. Backlog pass**" section:

```markdown
**4a. Digest-only path (`--digest-only`, change 0094).** A separate, write-free entry point that
runs `digest_only_pass` and exits before anything else — including `docket_preflight`. It resolves
config, enforces the bootstrap verdict fail-closed, and runs the same step-4 backlog pass. It is
deliberately **not** built on `--board-only`: that flag commits and pushes `BOARD.md`, and a
selection read must not be a write.

Skipping the preflight sync is what makes the read strict — preflight fetches and rebases the
metadata worktree, which can move `HEAD`. Nothing is lost: `docket-implement-next` runs this
**after** its own Step-0 preflight, so the tree is already synced. The digest is a snapshot of the
change files as of the moment it runs; taking it before Step 0's sweep would list already-merged
changes, which is why Step 1 orders it after.
```

- [ ] **Step 8: Run the affected suites and commit**

Run:
```bash
bash tests/test_docket_status.sh 2>&1 | grep -c "NOT OK"
bash tests/test_render_board.sh 2>&1 | grep -c "NOT OK"
bash tests/test_script_contracts_coverage.sh 2>&1 | grep -c "NOT OK"
bash tests/test_docket_facade.sh 2>&1 | grep -c "NOT OK"
```
Expected: `0` from each.

```bash
git add scripts/docket-status.sh scripts/docket-status.md tests/test_docket_status.sh
git commit -m "feat(0094): add write-free docket-status --digest-only selection read"
```

---

### Task 3: Rewire `docket-implement-next` Step 1 onto the `ready` line

**Files:**
- Modify: `skills/docket-implement-next/SKILL.md` (Step 1, lines 31–37)
- Modify: `tests/test_skill_size_budgets.sh` (the `skills/docket-implement-next/SKILL.md` budget row, line 25)
- Test: `tests/test_skill_facade_wiring.sh` (append a producer-anchored sentinel battery)

**Interfaces:**
- Consumes: Task 2's `docket.sh docket-status --digest-only`, emitting a `ready [<id> …]` line.
- Produces: the final deliverable — Step 1 prose that acquires an ordered candidate set from the digest while the convention's *Build-readiness & selection* definition stays authoritative.

**Posture that must survive the rewrite** (spec §4 — a human diff read of before/after is required, and it is Step 5 of this task):
1. The convention's *Build-readiness & selection* definition **stays the authority**. Step 1 gains an *acquisition path*; it does not delegate the *definition* to a script.
2. The id-allowlist paragraph's posture is **untouched** — "a filter, never a dependency override", skipped-with-reason, "never aborts the run".

- [ ] **Step 1: Write the failing sentinels**

Append to `tests/test_skill_facade_wiring.sh`, before its final exit line:

```bash
# ── change 0094: Step 1 acquires its candidate set from the digest's `ready` line ─────────────
# specified-but-unreachable: a contract with a PRODUCER and a CONSUMER needs at least one assert
# anchored on the producer. Here the producer is render-board.sh's ready line and the write-free
# reader is docket-status.sh --digest-only; asserting only that the SKILL mentions a ready line
# would pass identically in a world where nothing ever emits one. So: pin both ends.
impl="$REPO/skills/docket-implement-next/SKILL.md"

# PRODUCER end — the line really is emitted, and the read path really is write-free.
assert "producer: render-board.sh emits a ready line" \
  'grep -q "printf .ready" "$REPO/scripts/render-board.sh"'
assert "producer: docket-status.sh implements --digest-only" \
  'grep -q -- "--digest-only) DIGEST_ONLY=1" "$REPO/scripts/docket-status.sh"'
assert "producer: the digest-only path short-circuits before docket_preflight" \
  'awk "/^main\(\)/,/docket_preflight/" "$REPO/scripts/docket-status.sh" | grep -q "digest_only_pass"'

# CONSUMER end — Step 1 names the exact invocation a reader must run.
assert "skill Step 1 names the --digest-only invocation" \
  'grep -q -- "docket-status --digest-only" "$impl"'
assert "skill Step 1 names the ready line as its candidate source" \
  'grep -q "\`ready\`" "$impl"'
assert "skill Step 1 keeps the change files authoritative (accelerator, not sole channel)" \
  'grep -qi "accelerator" "$impl"'
assert "skill Step 1 documents the no-ready-line fallback to walking active/" \
  'grep -qi "fall back" "$impl"'

# POSTURE — the two things spec section 4 says must not be lost in the rewrite.
assert "skill Step 1 still defers the DEFINITION to the convention" \
  'grep -q "Build-readiness & selection" "$impl"'
assert "skill allowlist is still a filter, never a dependency override" \
  'grep -q "never a dependency override" "$impl"'
assert "skill allowlist still skips with a reason and never aborts" \
  'grep -q "skipped with its reason" "$impl" && grep -q "never aborts the run" "$impl"'
assert "skill Step 1 still routes an empty queue to drained" \
  'grep -q "drained" "$impl"'
```

- [ ] **Step 2: Run to verify they fail**

Run: `bash tests/test_skill_facade_wiring.sh 2>&1 | grep "NOT OK"`

Expected: FAIL on the consumer-end asserts (`skill Step 1 names the --digest-only invocation`, `… names the ready line …`, `… accelerator …`, `… fall back …`). The producer-end asserts should already PASS (Tasks 1–2 landed them) — if a producer assert fails, stop: the earlier task regressed.

- [ ] **Step 3: Rewrite Step 1 in the skill**

In `skills/docket-implement-next/SKILL.md`, replace lines 31–37 (the whole `### Step 1 — Select` section, from the heading through the "Empty queue → `drained`" paragraph) with:

```markdown
### Step 1 — Select

Build-readiness and ranking are defined by the convention's **Build-readiness & selection** section — that definition is the authority here, and this step only describes how to ACQUIRE the set it defines.

**Acquisition.** Run `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh docket-status --digest-only` — a write-free read, and only AFTER Step 0's `docket-status` dispatch and metadata re-sync (the digest is a snapshot; taken pre-sweep it lists already-merged changes). Its final `ready <id> <id> …` line is the build-ready queue already in the convention's deterministic order — whose final tie-break is LOWEST `id`, so two implementers (if ever run concurrently) converge on the same winner and never claim the same change. The `change <id> <status> <readiness> <slug>` lines carry the skip reasons (`needs-brainstorm`, `auto-groom-blocked`, `waiting-on-<N>-unbuilt`, `waiting-on-<N>-needs-merge`).

The digest is an **accelerator, not the sole channel**: the change files stay authoritative. Read the top candidate's change file and CONFIRM build-readiness before claiming — a stale digest then costs a re-pick, never a bad build. If the file disagrees with the digest, drop that candidate, take the next, and REPORT the disagreement (digest/file drift is a signal, not noise). A **bare `ready`** line means the queue is empty; **no `ready` line at all** means no queue was produced (an older `render-board`, a failed render) — fall back to walking `active/` yourself, applying the convention's definition and order, and say so in the run report as a degradation to investigate.

**Scope (id allowlist).** With no argument the candidate set is the whole `ready` queue. A caller may pass an **id allowlist** — `docket-implement-next 90,92,94` (a single id `90` is the degenerate case) — and selection is then **restricted to that set**, preserving the queue's order *within* it. The allowlist is a filter, **never a dependency override**: a scoped id that is not currently build-ready+claimable — needs-brainstorm, already `in-progress`, or waiting on an unmerged `depends_on` — is **skipped with its reason** (read it off that id's `change` line), never force-built, and never aborts the run.

**Empty queue → `drained`.** If no candidate in scope is build-ready+claimable, build nothing and end the run with the **`drained`** disposition (see *Terminal disposition*) — the driver's stop signal.
```

- [ ] **Step 4: Raise the size budget in the same diff**

Check the new actuals:

```bash
wc -l -w skills/docket-implement-next/SKILL.md
```

In `tests/test_skill_size_budgets.sh`, edit line 25. The pre-change row is:

```
skills/docket-implement-next/SKILL.md                      140 2845
```

Raise both numbers to the new actuals **plus ~10% headroom**, keeping the column alignment. With the Step 1 rewrite the file grows to roughly 131 lines / 2960 words, so the row becomes:

```
skills/docket-implement-next/SKILL.md                      145 3260
```

Set the two numbers from the `wc` output you actually observed (`ceil(lines * 1.10)` and `ceil(words * 1.10)`), not from these estimates. The guard explicitly permits an in-diff raise; this is that raise.

- [ ] **Step 5: Human diff read of the Step 1 rewrite (spec §4 — REQUIRED, not optional)**

Run: `git diff -- skills/docket-implement-next/SKILL.md`

Read the before/after and confirm, in writing, each of these. The sentinels in Step 1 of this task pin the *phrases*; they cannot see *posture* — this read is the only thing that can (`consolidation-flattens-caller-variance`).

1. The convention's *Build-readiness & selection* definition is still the **authority**; Step 1 added an acquisition path and did **not** move the definition into a script.
2. The allowlist is still **a filter, never a dependency override**; a not-ready scoped id is still **skipped with its reason** and still **never aborts the run**.
3. The lowest-`id` tie-break rationale (concurrent implementers converge, never collide) survived.
4. `drained` still fires on an empty queue in scope.

If any of the four reads weaker than before, fix the prose before committing.

- [ ] **Step 6: Run the suites to verify they pass**

Run:
```bash
bash tests/test_skill_facade_wiring.sh 2>&1 | grep -c "NOT OK"
bash tests/test_skill_size_budgets.sh 2>&1 | grep -c "NOT OK"
bash tests/test_skill_handoff_precedence.sh 2>&1 | grep -c "NOT OK"
bash tests/test_loop_continuation.sh 2>&1 | grep -c "NOT OK"
```
Expected: `0` from each.

- [ ] **Step 7: Commit**

```bash
git add skills/docket-implement-next/SKILL.md tests/test_skill_size_budgets.sh tests/test_skill_facade_wiring.sh
git commit -m "feat(0094): rewire implement-next Step 1 onto the digest ready line"
```

---

### Task 4: Whole-suite verification

**Files:** none modified — this task only runs and reports.

**Interfaces:**
- Consumes: everything Tasks 1–3 produced.
- Produces: a green suite, or a named failing test.

- [ ] **Step 1: Run the entire test suite**

Run, in **one foreground invocation** (it takes roughly 10 minutes — do not background it):

```bash
cd /Users/homer/dev/docket/.worktrees/selection-order-digest && for t in tests/test_*.sh; do echo "=== $t ==="; bash "$t" 2>&1 | grep -E "NOT OK" || echo "  (clean)"; done
```

Expected: every test reports `(clean)`.

- [ ] **Step 2: Fix or escalate**

If any test reports `NOT OK`, root-cause it before proceeding — a failure here is a real regression in a shared script (`render-board.sh` and `docket-status.sh` are both widely consumed). Do not weaken an assert to make it pass.

- [ ] **Step 3: Confirm the end-to-end path on the real repo**

The suite is hermetic and sees only fixtures (`metadata-branch-invisible-to-suite`), so prove the real invocation works against the live metadata worktree:

```bash
cd /Users/homer/dev/docket && "${DOCKET_SCRIPTS_DIR:?}"/docket.sh docket-status --digest-only | tail -3
git -C /Users/homer/dev/docket/.docket status --porcelain
```

Expected: a `ready …` line listing real build-ready ids (94 will be absent — it is `in-progress`), and a clean `git status` proving the read wrote nothing. Record the observed `ready` line in the results file.
