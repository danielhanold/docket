<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0117 — Deferred ADR-publish visibility — detect an unpublished ADR with a computed board-checks finding](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-07-28-0117-deferred-adr-publish-visibility-decide-whether-docket-adr-s.md)**
<!-- docket:backlink:end -->

# Unpublished-ADR detection (`adr-unpublished`) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add one computed, git-only health check — `adr-unpublished` — to `scripts/board-checks.sh` that reports an ADR whose publish onto the integration branch was due but did not happen, or that drifted after publication.

**Architecture:** `board-checks.sh` gains two optional arguments (`--adrs-dir DIR`, `--terminal-publish`) and one new check block that walks the ADR directory, resolves each ADR's blob on the metadata branch and on the integration branch with `git rev-parse --verify -q <ref>:<path>`, and emits under a due rule keyed on the ADR's `status:` and its `change:` back-link's lifecycle status. `docket-status.sh` passes the two new arguments through from resolved config, supplying `--terminal-publish` only when `terminal_publish: true` **and** docket-mode. Report only — no writer, no marker, no healer.

**Tech Stack:** Bash (the repo's `DOCKET_BASH_PATH` runtime), `git` plumbing only (no `gh`, no network), the shared `scripts/lib/docket-frontmatter.sh` helpers, and the repo's hand-rolled `assert`-based test suites.

## Global Constraints

Copied verbatim from the spec and the reconcile log; every task's requirements implicitly include these.

- **Git-only and offline.** Both arms probe **local branch refs** via git plumbing. No network, no `gh`. (spec §4.3)
- **Report only.** No healer, no re-publisher, no auto-fix, no marker, no writer, no removal path. (spec §2, §7)
- **Warn-only.** Findings never change the exit status; `--strict` remains the only path to exit 1. (`board-checks.sh` existing contract)
- **One check-id, two messages** — `adr-unpublished`, following the `stale-in-progress` precedent, *not* two ids for one condition. (spec §4.3)
- **The due rule** (spec §4.2), exactly:
  | ADR shape | Due when |
  |---|---|
  | No `change:` back-link (standalone), `status: Accepted` | immediately |
  | Has a `change:` back-link | that change's status is `done` or `killed` |
  | Already present on the integration branch | always (bytes must match, whatever the status) |
- **The gate** (spec §4.4): the check emits nothing unless **both** `terminal_publish: true` **and** docket-mode.
- **ADR-0049 rule, not to be weakened:** *structural columns carry only script-derived or shape-validated values; untrusted text belongs in the message column.*
- **`fm_field`, never `field`, for `status:` and `change:`** — `change:` may be **absent**, and `field()` would fall through to body prose. (`board-checks.sh`'s own `fd_type` line sets this precedent with the same reasoning.)
- **Zero findings on this repo today.** After the build, running the check against the live repo must emit nothing: ADR-0023 (`change: 44`, `blocked`) and ADR-0060 (`change: 135`, `implemented`) are both correctly not-due, and no ADR present on both branches drifts. (reconcile log)
- **Four-site registration** (spec §4.5): `BOARD_CHECK_IDS` in `scripts/lib/docket-frontmatter.sh`, `board-checks.sh`'s `check-id ∈ {…}` header, `scripts/board-checks.md`'s `**`<id>`**` section, `scripts/docket-status.md`'s single `check <check-id>` row — **plus** the one hardcoded literal the derived guard does not absorb: `tests/test_board_checks.sh`'s `[ "${#BOARD_CHECK_IDS[@]}" = 12 ]` → `13`.

---

## File Structure

| File | Change | Responsibility |
|---|---|---|
| `scripts/board-checks.sh` | Modify | Argument parsing + validation for `--adrs-dir` / `--terminal-publish`; the new `adr-unpublished` check block; the `check-id ∈ {…}` header enumeration. |
| `scripts/lib/docket-frontmatter.sh` | Modify (1 line) | `BOARD_CHECK_IDS` gains `adr-unpublished`. |
| `scripts/board-checks.md` | Modify | Usage table rows for the two flags; a `**`adr-unpublished`**` section under *Check enumeration*. |
| `scripts/docket-status.md` | Modify (1 row) | The closed `check <check-id>` enumeration gains `adr-unpublished`. |
| `scripts/docket-status.sh` | Modify | `health_checks()` passes `--adrs-dir`, and `--terminal-publish` only under the two-leg gate. |
| `tests/test_board_checks.sh` | Modify | The `12` → `13` count literal; a new `adr-unpublished` fixture block covering the due rule, both arms, and the script-side gate leg. |
| `tests/test_docket_status.sh` | Modify | The caller-side gate leg: argument-log asserts that main-mode / `TERMINAL_PUBLISH=false` omit `--terminal-publish`, and docket-mode + true passes it. |

---

### Task 1: The check — arguments, gate, due rule, `missing` arm, and full registration

Registration lands **in this task**, not later: the moment `board-checks.sh` emits a new literal check-id, `tests/test_board_checks.sh`'s derived set-compares redden. A task must leave the suite green.

**Files:**
- Modify: `scripts/board-checks.sh` (header enumeration ~line 12; arg loop ~lines 28-51; new check block after the `FILES` walk closes at ~line 313)
- Modify: `scripts/lib/docket-frontmatter.sh:294-296` (`BOARD_CHECK_IDS`)
- Modify: `scripts/board-checks.md` (Usage table ~lines 12-26; *Check enumeration* ~line 41)
- Modify: `scripts/docket-status.md:355` (the `check <check-id>` row)
- Test: `tests/test_board_checks.sh` (the `12` literal at ~line 1096; new fixture block inserted **before** the `# --- registration:` comment at ~line 997)

**Interfaces:**
- Consumes: `field`, `fm_field`, `int_field`, `padded_id_from_file`, `emit`, `docket_status_is_terminal`, and the `STATUS_OF` map populated by `resolve_deps` — all already in scope at the insertion point.
- Produces: the `adr-unpublished` check-id with two message shapes; the `--adrs-dir DIR` and `--terminal-publish` CLI arguments consumed by Task 3's `docket-status.sh` passthrough.

- [ ] **Step 1: Write the failing test**

Insert this block into `tests/test_board_checks.sh` immediately **before** the line `# --- registration: the check-id is documented everywhere it must be (correspondence guard) ------`.

```bash
# ============================ adr-unpublished (missing arm) ============================
# The due rule (spec §4.2) decides whether an ADR absent from the integration branch is a gap.
# Every negative row is asserted, not just the positive: a check that fires on everything absent
# is the naive formulation the ADR-0023/ADR-0060 data points exist to rule out.
read -r AU AU_ORIGIN <<<"$(new_repo)"
mkdir -p "$AU/docs/adrs"

# --- change files the due rule resolves `change:` against (on the docket checkout) ---
cat > "$AU/docs/changes/archive/2026-07-01-0060-done-change.md" <<'EOF'
---
id: 60
slug: done-change
title: A change that reached done
status: done
priority: medium
depends_on: []
trivial: true
---
EOF
cat > "$AU/docs/changes/archive/2026-07-02-0061-killed-change.md" <<'EOF'
---
id: 61
slug: killed-change
title: A change that was killed
status: killed
priority: medium
depends_on: []
trivial: true
---
EOF
cat > "$AU/docs/changes/active/0062-implemented-change.md" <<'EOF'
---
id: 62
slug: implemented-change
title: A change still at the merge gate
status: implemented
priority: medium
depends_on: []
trivial: true
---
EOF

# --- ADRs. Only adr_pub is committed to BOTH branches; the rest live on docket only. ---
# 10: standalone (no change:), Accepted        -> DUE now, absent      => finding
# 11: change: 60 (done)                        -> DUE, absent          => finding
# 12: change: 61 (killed)                      -> DUE, absent          => finding
# 13: change: 62 (implemented)                 -> NOT due (ADR-0060 shape) => silent
# 14: standalone, status: Superseded by ADR-10 -> NOT Accepted, absent  => silent
# 15: change: 99 (no such change file)         -> unresolvable          => silent
write_adr(){ # write_adr NUM STATUS CHANGE
  local num="$1" st="$2" ch="$3"
  { printf -- '---\nid: %s\nslug: adr-%s\ntitle: ADR %s\nstatus: %s\ndate: 2026-07-01\n' \
      "$((10#$num))" "$((10#$num))" "$((10#$num))" "$st"
    printf 'supersedes: []\nreverses: []\nrelates_to: []\n'
    [ -n "$ch" ] && printf 'change: %s\n' "$ch"
    printf -- '---\n\n## Context\n\nc\n\n## Decision\n\nd\n\n## Consequences\n\nq\n'
  } > "$AU/docs/adrs/${num}-adr-$((10#$num)).md"
}
write_adr 0010 Accepted ""
write_adr 0011 Accepted 60
write_adr 0012 Accepted 61
write_adr 0013 Accepted 62
write_adr 0014 "Superseded by ADR-10" ""
write_adr 0015 Accepted 99
echo "# index" > "$AU/docs/adrs/README.md"
git -C "$AU" add -A; git_quiet -C "$AU" commit -m "adr fixtures"; git_quiet -C "$AU" push origin docket

auout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AU/docs/changes" \
  --adrs-dir "$AU/docs/adrs" --terminal-publish \
  --metadata-branch docket --integration-branch main 2>/dev/null)"

assert "adr-unpublished fires for a STANDALONE Accepted ADR absent from the integration branch (ADR-0010)" \
  'has_finding "$auout" adr-unpublished "?"'
assert "adr-unpublished fires for a change-tied ADR whose change is DONE (ADR-0011, cid 60)" \
  'has_finding "$auout" adr-unpublished 60'
assert "adr-unpublished fires for a change-tied ADR whose change is KILLED (ADR-0012, cid 61)" \
  'has_finding "$auout" adr-unpublished 61'
assert "adr-unpublished SILENT for a change-tied ADR whose change is IMPLEMENTED (ADR-0013 — the live ADR-0060 shape)" \
  '! has_finding "$auout" adr-unpublished 62'
assert "adr-unpublished SILENT for a non-Accepted ADR absent from the integration branch (ADR-0014)" \
  '[ "$(grep -c -- "ADR-0014" <<<"$auout")" -eq 0 ]'
assert "adr-unpublished SILENT for an ADR whose change: resolves to no change file (ADR-0015)" \
  '[ "$(grep -c -- "ADR-0015" <<<"$auout")" -eq 0 ]'
assert "adr-unpublished names the ADR number in the message column (ADR-0010)" \
  '[ "$(grep -c -- "ADR-0010" <<<"$auout")" -eq 1 ]'
assert "adr-unpublished skips README.md (never reported as an ADR)" \
  '[ "$(grep -ci -- "README" <<<"$auout")" -eq 0 ]'
# ADR-0049: the change-id column carries a shape-validated value only. `?` is the existing
# fallback for "no usable id" (padded_id_from_file), reused here rather than widening the column
# to admit an ADR reference — the ADR number rides the message column, which is the last field of
# the caller's `read` and therefore harmless.
au10="$(grep -E "$(printf '^adr-unpublished\t\\?\t')" <<<"$auout")"
assert "the standalone ADR's change-id column is the validated '?' fallback, not an ADR reference" \
  '[ -n "$au10" ]'
assert "adr-unpublished message names the integration branch" 'grep -qF -- "main" <<<"$au10"'

# --- the SCRIPT-SIDE gate leg: no --terminal-publish => the check is entirely silent ---
augateout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AU/docs/changes" \
  --adrs-dir "$AU/docs/adrs" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "gate: without --terminal-publish the check emits NOTHING (spec §4.4)" \
  '[ "$(grep -c "^adr-unpublished" <<<"$augateout")" -eq 0 ]'
# ...and the whole check is opt-in on --adrs-dir too, so every pre-existing caller is unaffected.
aunodir="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AU/docs/changes" --terminal-publish \
  --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "gate: without --adrs-dir the check emits NOTHING" \
  '[ "$(grep -c "^adr-unpublished" <<<"$aunodir")" -eq 0 ]'
assert "board-checks still exits 0 with adr-unpublished findings (warn-only)" \
  'NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AU/docs/changes" --adrs-dir "$AU/docs/adrs" \
     --terminal-publish --metadata-branch docket --integration-branch main >/dev/null 2>&1'
assert "a missing --adrs-dir path is rejected up front (exit 2), never silently skipped" \
  '! bash "$SCRIPT" --changes-dir "$AU/docs/changes" --adrs-dir "$AU/docs/adrs-nope" \
     --terminal-publish --metadata-branch docket --integration-branch main >/dev/null 2>&1'
```

Then change the count literal at `tests/test_board_checks.sh:1096-1097` from:

```bash
assert "BOARD_CHECK_IDS holds the 12 check-ids board-checks.sh emits" \
  '[ "${#BOARD_CHECK_IDS[@]}" = 12 ]'
```

to:

```bash
# 13 since change 0117 added adr-unpublished. This literal is the ONE hand-edit the derived
# set-compares below do not absorb (verified at 0117's reconcile) — bump it with every new id.
assert "BOARD_CHECK_IDS holds the 13 check-ids board-checks.sh emits" \
  '[ "${#BOARD_CHECK_IDS[@]}" = 13 ]'
```

- [ ] **Step 2: Run test to verify it fails**

Run: `bash tests/test_board_checks.sh 2>&1 | grep -c "^NOT OK"`
Expected: a non-zero count. The `--adrs-dir` argument is unknown, so the script exits 2 and every new assert fails; the `13` count assert fails against the 12-entry array; the derived set-compares still pass (nothing new is emitted yet).

- [ ] **Step 3: Write minimal implementation**

**3a.** `scripts/board-checks.sh` — extend the header block. Replace the `Usage:` line and the `check-id ∈ {…}` enumeration (lines ~8-15) with:

```bash
# Usage: board-checks.sh --changes-dir DIR --metadata-branch BR --integration-branch BR [--strict]
#                         [--lease-ttl-hours N] [--adrs-dir DIR] [--terminal-publish]
#   Findings: TAB-separated  <check-id>\t<change-id>\t<message>  on stdout, sorted by (check-id, change-id).
#     check-id ∈ {adr-unpublished, board-row-dropped, broken-spec, broken-plan-results, dep-cycle,
#                 field-domain, publish-deferred, stale-in-progress, merge-gate-stall,
#                 stale-finalize-blocked, merged-orphan, unknown-commit-ref, malformed-id}
#     The set above is declared in lib/docket-frontmatter.sh as BOARD_CHECK_IDS and pinned to it,
#     to board-checks.md, and to docket-status.md by tests/test_board_checks.sh — edit all four.
```

**3b.** Same file — add the two arguments. In the `while` loop add two cases, and add the two variables to the initialization line:

```bash
CHANGES_DIR=""; METADATA_BRANCH=""; INTEGRATION_BRANCH=""; STRICT=0
ADRS_DIR=""; ADR_GATE=0
```

```bash
    --adrs-dir) ADRS_DIR="$2"; shift ;;
    --terminal-publish) ADR_GATE=1 ;;
```

`ADR_GATE`, deliberately not `TERMINAL_PUBLISH`: the caller (`docket-status.sh`) carries a shell variable by that name, and a same-named internal would silently inherit it if it were ever exported — the gate must come from argv alone.

Add the validation beside the existing `[ -n "$CHANGES_DIR" ]` block:

```bash
# --adrs-dir is OPTIONAL (the check is opt-in), but a path that was SUPPLIED and does not exist is
# a caller error, never a silent skip: a typo'd dir would make the check vacuously green forever.
if [ -n "$ADRS_DIR" ] && [ ! -d "$ADRS_DIR" ]; then
  printf 'board-checks: adrs dir not found: %s\n' "$ADRS_DIR" >&2; exit 2
fi
```

**3c.** Same file — insert the check block immediately **after** the `for f in "${FILES[@]}"; do … done` walk closes (after the `publish-deferred` block's `done`, before the `# --- board-row-dropped:` loop). `resolve_deps` has already populated `STATUS_OF`.

```bash
# --- adr-unpublished: an ADR whose publish onto the integration branch is DUE but did not happen,
# or that drifted after publication (change 0117). Computed, not marked: unlike publish-deferred,
# this needs nothing at all from the run that went wrong — which is the whole point, since the
# failure mode being closed is that NOBODY NOTICED. The ADR corpus has no marker seam to hang a
# marker on anyway: an ADR file is never moved (no archive moment) and an Accepted ADR is immutable
# except its status: line. See ADR-0051's boundary — that decision declined a detector-AND-HEALER
# over CHANGE records; this is a read-only report over ADRs and reverses nothing.
#
# Gated twice, both legs required (spec §4.4): --adrs-dir supplied AND --terminal-publish passed.
# The caller passes --terminal-publish only under `terminal_publish: true` AND docket-mode; under
# the default `false` the ledger deliberately lives on the metadata branch only, so an ungated
# check would fire on every ADR forever, and in main-mode the two refs coincide so the comparison
# is vacuous.
if [ -n "$ADRS_DIR" ] && [ "$ADR_GATE" = 1 ]; then
  # Repo-relative path prefix for the ADR dir, derived from git itself rather than from the
  # config value: the script is handed a FILESYSTEM path (as with --changes-dir) but must probe
  # refs, which are addressed repo-relative. --show-prefix is worktree-root-relative, which is
  # exactly what `<ref>:<path>` wants, and it needs no network.
  adr_prefix="$("$GIT" -C "$ADRS_DIR" rev-parse --show-prefix 2>/dev/null)"
  mapfile -t ADR_FILES < <(find "$ADRS_DIR" -maxdepth 1 -name '*.md' ! -name 'README.md' 2>/dev/null | sort)
  for af in "${ADR_FILES[@]}"; do
    a_num="$(padded_id_from_file "$af")"
    [ "$a_num" = '?' ] && continue          # not a numbered ADR file; adr-checks.sh owns naming hygiene
    a_rel="${adr_prefix}$(basename "$af")"
    # fm_field, never field: `change:` is legitimately ABSENT on a standalone ADR, and field()
    # would fall through and read body prose as its value.
    a_status="$(fm_field "$af" status)"
    a_change="$(fm_field "$af" change)"
    a_change_id=""
    case "$a_change" in
      ''|*[!0-9]*) ;;                        # absent, or not a bare integer -> unresolvable
      *) a_change_id="$(( 10#$a_change ))" ;;
    esac
    # ADR-0049: the change-id column carries only script-derived or shape-validated values. The
    # validated change id when there is one; otherwise `?`, the same fallback padded_id_from_file
    # already uses for a file whose id is unusable. The ADR number rides the MESSAGE column, which
    # is the last field of the caller's `read` and cannot shift a field.
    a_cid="${a_change_id:-?}"
    m_blob="$("$GIT" -C "$ADRS_DIR" rev-parse --verify -q "$METADATA_BRANCH:$a_rel" 2>/dev/null)"
    i_blob="$("$GIT" -C "$ADRS_DIR" rev-parse --verify -q "$INTEGRATION_BRANCH:$a_rel" 2>/dev/null)"

    if [ -n "$i_blob" ]; then
      # Present on the integration branch => due FOREVER, whatever its status. This row is what
      # catches an un-re-published status flip, and it is deliberately status-blind: an ADR
      # published while Accepted must keep tracking its bytes after it is Superseded or Reversed.
      # (stale arm lands in Task 2.)
      continue
    fi

    # Absent on the integration branch. Never expected there unless the publish trigger has fired.
    [ "$a_status" = "Accepted" ] || continue
    if [ -n "$a_change" ]; then
      # Change-tied: due only once its change reached a TERMINAL status. An unresolvable
      # change: value stays silent — absence of a resolvable link is not evidence of a gap.
      [ -n "$a_change_id" ] || continue
      docket_status_is_terminal "${STATUS_OF[$a_change_id]:-}" || continue
    fi
    emit adr-unpublished "$a_cid" "ADR-$a_num is due on $INTEGRATION_BRANCH but absent — publish it (docket.sh terminal-publish --adr $a_num)"
  done
fi
```

**3d.** `scripts/lib/docket-frontmatter.sh:294` — add the id to the array, keeping alphabetical order:

```bash
BOARD_CHECK_IDS=(adr-unpublished board-row-dropped broken-plan-results broken-spec dep-cycle
                 field-domain malformed-id merge-gate-stall merged-orphan publish-deferred
                 stale-finalize-blocked stale-in-progress unknown-commit-ref)
```

**3e.** `scripts/board-checks.md` — update Usage. Replace the fenced usage block with:

```
board-checks.sh --changes-dir DIR --metadata-branch BR --integration-branch BR [--strict]
                 [--lease-ttl-hours N] [--adrs-dir DIR] [--terminal-publish]
```

and append two rows to the flag table:

```markdown
| `--adrs-dir DIR` | no | Path to the flat ADR directory. Enables the `adr-unpublished` check (with `--terminal-publish`). A supplied path that does not exist is an error (exit 2), never a silent skip. |
| `--terminal-publish` | no | Opens the `adr-unpublished` gate. The caller passes it only when `terminal_publish: true` **and** docket-mode; absent, the check emits nothing. |
```

Then add this section under *Check enumeration*, after the `**`publish-deferred`**` section:

```markdown
**`adr-unpublished`** — An ADR whose publish onto the integration branch was due but did not
happen. Gated: emits nothing unless `--adrs-dir` is supplied **and** `--terminal-publish` is
passed. Walks `<adrs-dir>/*.md` (excluding `README.md` and any file whose basename yields no
padded id), and for each resolves the blob on `--metadata-branch` and on `--integration-branch`
via `git rev-parse --verify -q <ref>:<path>` — local refs only, no network. The due rule: a
standalone `Accepted` ADR is due immediately; a `change:`-tied ADR is due once that change reaches
`done` or `killed`; an ADR already present on the integration branch is due always, whatever its
status. An ADR that is neither `Accepted` nor already published is never expected there, and an
unresolvable `change:` stays silent. The change-id column carries the validated `change:` id when
there is one and `?` otherwise (ADR-0049); the ADR number is always named in the message. Report
only — this check writes nothing and heals nothing.
```

**3f.** `scripts/docket-status.md:355` — add `adr-unpublished` to the `∈ {…}` set on that single row (keep it one row; the extractor pins the count at exactly 1).

- [ ] **Step 4: Run test to verify it passes**

Run: `bash tests/test_board_checks.sh 2>&1 | tail -30`
Expected: no `NOT OK` lines. Then confirm the whole registration guard sees the new id:

Run: `bash tests/test_board_checks.sh 2>&1 | grep -E "check-id SET|BOARD_CHECK_IDS"`
Expected: every line begins `ok -`.

- [ ] **Step 5: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/deferred-adr-publish-visibility-decide-whether-docket-adr-s
git add scripts/board-checks.sh scripts/board-checks.md scripts/docket-status.md \
        scripts/lib/docket-frontmatter.sh tests/test_board_checks.sh
git commit -m "feat(0117): adr-unpublished check — missing arm, due rule, gate, four-site registration"
```

---

### Task 2: The `stale` arm — present on both branches, bytes differ

**Files:**
- Modify: `scripts/board-checks.sh` (the `if [ -n "$i_blob" ]; then … continue; fi` clause added in Task 1)
- Modify: `scripts/board-checks.md` (the `**`adr-unpublished`**` section — document the second message)
- Test: `tests/test_board_checks.sh` (append to the `adr-unpublished` block from Task 1)

**Interfaces:**
- Consumes: `m_blob`, `i_blob`, `a_num`, `a_cid` from Task 1's loop body.
- Produces: the second `adr-unpublished` message shape. No new check-id, so no registration change.

- [ ] **Step 1: Write the failing test**

Append to `tests/test_board_checks.sh`, at the end of Task 1's `adr-unpublished` block:

```bash
# ============================ adr-unpublished (stale arm) ============================
# An ADR present on BOTH branches whose bytes differ — the un-re-published status flip. A marker
# structurally cannot catch this: nothing failed at publish time, the file simply moved on.
# Fixture shape matters (green-suite-untested-branch): ADR-0020 is published and IDENTICAL, so the
# arm must distinguish drift from mere presence rather than firing on every published ADR.
write_adr 0020 Accepted ""
write_adr 0021 Accepted ""
git -C "$AU" add -A; git_quiet -C "$AU" commit -m "adrs 20,21 on docket"; git_quiet -C "$AU" push origin docket
# Publish both onto main verbatim, then drift ONLY 0021 on docket (a status flip).
git_quiet -C "$AU" checkout main
git_quiet -C "$AU" checkout docket -- docs/adrs/0020-adr-20.md docs/adrs/0021-adr-21.md
git -C "$AU" add -A; git_quiet -C "$AU" commit -m "publish adrs 20,21"; git_quiet -C "$AU" push origin main
git_quiet -C "$AU" checkout docket
sed -i.bak 's/^status: Accepted/status: Superseded by ADR-20/' "$AU/docs/adrs/0021-adr-21.md"
rm -f "$AU/docs/adrs/0021-adr-21.md.bak"
git -C "$AU" add -A; git_quiet -C "$AU" commit -m "flip adr 21 status"; git_quiet -C "$AU" push origin docket

stout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AU/docs/changes" \
  --adrs-dir "$AU/docs/adrs" --terminal-publish \
  --metadata-branch docket --integration-branch main 2>/dev/null)"

assert "adr-unpublished fires the STALE arm for a published ADR whose bytes drifted (ADR-0021)" \
  '[ "$(grep -c -- "ADR-0021" <<<"$stout")" -eq 1 ]'
assert "the stale finding is an adr-unpublished line (one check-id, two messages)" \
  'grep -E "$(printf "^adr-unpublished\t")" <<<"$stout" | grep -qF -- "ADR-0021"'
assert "adr-unpublished SILENT for a published ADR whose bytes MATCH (ADR-0020)" \
  '[ "$(grep -c -- "ADR-0020" <<<"$stout")" -eq 0 ]'
# Status-blindness: ADR-0021 is no longer Accepted, yet it is still due because it is already
# published. An Accepted-only gate on this arm would silence exactly the case it exists to catch.
st21="$(grep -E "$(printf '^adr-unpublished\t\\?\t')" <<<"$stout" | grep -- "ADR-0021")"
assert "the stale message is DISTINCT from the missing message (says differs/re-publish, not absent)" \
  '! grep -qF -- "absent" <<<"$st21"'
assert "the stale message names both branches" \
  'grep -qF -- "docket" <<<"$st21" && grep -qF -- "main" <<<"$st21"'
```

- [ ] **Step 2: Run test to verify it fails**

Run: `bash tests/test_board_checks.sh 2>&1 | grep "^NOT OK"`
Expected: the five new asserts fail — Task 1's block `continue`s on every ADR present on the integration branch, so `ADR-0021` produces no finding at all.

- [ ] **Step 3: Write minimal implementation**

In `scripts/board-checks.sh`, replace Task 1's placeholder clause:

```bash
    if [ -n "$i_blob" ]; then
      # Present on the integration branch => due FOREVER, whatever its status. This row is what
      # catches an un-re-published status flip, and it is deliberately status-blind: an ADR
      # published while Accepted must keep tracking its bytes after it is Superseded or Reversed.
      # (stale arm lands in Task 2.)
      continue
    fi
```

with:

```bash
    if [ -n "$i_blob" ]; then
      # Present on the integration branch => due FOREVER, whatever its status. Deliberately
      # status-blind: an ADR published while Accepted must keep tracking its bytes after it is
      # Superseded or Reversed, and an Accepted-only gate here would silence exactly the
      # un-re-published status flip this arm exists to catch — the case a marker structurally
      # cannot see, because nothing FAILED at publish time.
      #
      # Blob-SHA equality, not a byte-by-byte diff: git already content-addresses both sides, so
      # the compare is one rev-parse each and needs no working-tree read. A missing m_blob (the
      # ADR is on the integration branch but not committed on the metadata branch) has nothing to
      # compare against, so it stays silent rather than guessing.
      if [ -n "$m_blob" ] && [ "$m_blob" != "$i_blob" ]; then
        emit adr-unpublished "$a_cid" "ADR-$a_num differs between $METADATA_BRANCH and $INTEGRATION_BRANCH — re-publish it (docket.sh terminal-publish --adr $a_num)"
      fi
      continue
    fi
```

Then extend the `**`adr-unpublished`**` section in `scripts/board-checks.md` with a sentence naming the second arm:

```markdown
Two arms share the one check-id (the `stale-in-progress` precedent): **missing** — due but absent
on the integration branch; and **stale** — present on both branches with differing blob SHAs, the
un-re-published status flip. An ADR present on the integration branch but not committed on the
metadata branch has nothing to compare against and stays silent.
```

- [ ] **Step 4: Run test to verify it passes**

Run: `bash tests/test_board_checks.sh 2>&1 | grep -c "^NOT OK"`
Expected: `0`.

- [ ] **Step 5: Commit**

```bash
git add scripts/board-checks.sh scripts/board-checks.md tests/test_board_checks.sh
git commit -m "feat(0117): adr-unpublished stale arm — status-blind blob-SHA drift on published ADRs"
```

---

### Task 3: `docket-status.sh` passthrough and the caller-side gate leg

**Files:**
- Modify: `scripts/docket-status.sh:695-710` (`health_checks`)
- Test: `tests/test_docket_status.sh` (the `health_checks` block at ~lines 1144-1166)

**Interfaces:**
- Consumes: Task 1's `--adrs-dir DIR` and `--terminal-publish` arguments.
- Produces: nothing later tasks depend on.

- [ ] **Step 1: Write the failing test**

Append to `tests/test_docket_status.sh` immediately after the existing assert `"health_checks: invokes board-checks.sh with expected flags"`:

```bash
# --- change 0117: the ADR-check arguments, and the CALLER-SIDE leg of its gate ----------------
# The script-side leg (--terminal-publish absent => silent) is pinned in tests/test_board_checks.sh.
# THIS is the other leg: docket-status passes the flag only under terminal_publish:true AND
# docket-mode. Both negatives are asserted, not just the positive — a gate that is only ever tested
# open is a gate nothing proves is closed.
assert "0117: health_checks passes --adrs-dir" \
  'grep -Eq -- "--adrs-dir \./?docs/adrs" "$health_log"'
assert "0117 gate(main-mode): --terminal-publish is NOT passed even with TERMINAL_PUBLISH=true" \
  '! grep -q -- "--terminal-publish" "$health_log"'

health_log_dk="$tmp/health-calls-docket.log"; : > "$health_log_dk"
health_out_dk="$( cd "$health_dir" && \
  DOCKET_MODE=docket CHANGES_DIR=docs/changes ADRS_DIR=docs/adrs \
  INTEGRATION_BRANCH=main METADATA_BRANCH=docket TERMINAL_PUBLISH=true \
  SCRIPTS_DIR="$tmp/mock-health" HEALTH_LOG="$health_log_dk" \
  bash -c '. "'"$SCRIPT"'"; health_checks' )"
assert "0117 gate(docket-mode + terminal_publish:true): --terminal-publish IS passed" \
  'grep -q -- "--terminal-publish" "$health_log_dk"'

health_log_off="$tmp/health-calls-off.log"; : > "$health_log_off"
health_out_off="$( cd "$health_dir" && \
  DOCKET_MODE=docket CHANGES_DIR=docs/changes ADRS_DIR=docs/adrs \
  INTEGRATION_BRANCH=main METADATA_BRANCH=docket TERMINAL_PUBLISH=false \
  SCRIPTS_DIR="$tmp/mock-health" HEALTH_LOG="$health_log_off" \
  bash -c '. "'"$SCRIPT"'"; health_checks' )"
assert "0117 gate(docket-mode + terminal_publish:false): --terminal-publish is NOT passed" \
  '! grep -q -- "--terminal-publish" "$health_log_off"'

health_log_unset="$tmp/health-calls-unset.log"; : > "$health_log_unset"
health_out_unset="$( cd "$health_dir" && \
  DOCKET_MODE=docket CHANGES_DIR=docs/changes ADRS_DIR=docs/adrs \
  INTEGRATION_BRANCH=main METADATA_BRANCH=docket \
  SCRIPTS_DIR="$tmp/mock-health" HEALTH_LOG="$health_log_unset" \
  bash -c '. "'"$SCRIPT"'"; health_checks' )"
assert "0117 gate(TERMINAL_PUBLISH unset): no unbound-variable crash, flag not passed" \
  '! grep -q -- "--terminal-publish" "$health_log_unset"'
```

The first main-mode assert requires `TERMINAL_PUBLISH=true` to be in scope for the original invocation, so also add `TERMINAL_PUBLISH=true` and `ADRS_DIR=docs/adrs` to the environment of the existing `health_out="$( cd "$health_dir" && …` invocation — that is what makes the main-mode negative meaningful rather than vacuous (it must be the *mode* leg that closes the gate, not a missing knob).

- [ ] **Step 2: Run test to verify it fails**

Run: `bash tests/test_docket_status.sh 2>&1 | grep "^NOT OK"`
Expected: the `--adrs-dir` assert and the docket-mode positive assert fail; `health_checks` passes neither argument yet.

- [ ] **Step 3: Write minimal implementation**

Replace the `board-checks.sh` invocation in `scripts/docket-status.sh`'s `health_checks()` with:

```bash
  # change 0117: the adr-unpublished check is opt-in on --adrs-dir, and its gate opens only when
  # terminal_publish is true AND we are in docket-mode. BOTH legs are resolved HERE, not in
  # board-checks.sh: mode is this script's knowledge, and in main-mode the metadata and integration
  # refs coincide so the comparison is vacuous. "${TERMINAL_PUBLISH:-false}" guards a stale or
  # mocked config export that does not emit the key (the change-0064 unbound-variable shape).
  local adr_args=()
  if [ -n "${ADRS_DIR:-}" ]; then
    adr_args+=(--adrs-dir "$mw/$ADRS_DIR")
    if [ "${TERMINAL_PUBLISH:-false}" = true ] && [ "${DOCKET_MODE:-}" = docket ]; then
      adr_args+=(--terminal-publish)
    fi
  fi
  "$DOCKET_BASH_PATH" "$SCRIPTS_DIR"/board-checks.sh \
    --changes-dir "$cd_dir" --metadata-branch "$metadata_branch" \
    --integration-branch "origin/$INTEGRATION_BRANCH" \
    --lease-ttl-hours "${RECLAIM_LEASE_TTL:-72}" "${adr_args[@]}" 2>&2 | \
```

Note `"${adr_args[@]}"` on an empty array is safe here because the script does not run under `set -u` for this expansion path — but write it as `${adr_args[@]+"${adr_args[@]}"}` if the suite reddens with an unbound-variable error on bash 3.2.

- [ ] **Step 4: Run test to verify it passes**

Run: `bash tests/test_docket_status.sh 2>&1 | grep -c "^NOT OK"`
Expected: `0`.

- [ ] **Step 5: Commit**

```bash
git add scripts/docket-status.sh tests/test_docket_status.sh
git commit -m "feat(0117): docket-status passes --adrs-dir and gates --terminal-publish on mode + knob"
```

---

### Task 4: Live-repo verification and mutation proof

The hermetic suite runs against the integration-branch checkout and cannot see the real ADR ledger on `docket` — the `metadata-branch-invisible-to-suite` finding's exact shape. So the real-corpus behavior is verified at build time, against the real tree, and recorded.

**Files:**
- Create: nothing in the repo. Findings are recorded in the change's results file (written by the caller at close-out).
- Modify: nothing.

**Interfaces:**
- Consumes: the finished check from Tasks 1-3.
- Produces: the verification record the results file carries.

- [ ] **Step 1: Run the finished check against the real repo, expecting ZERO findings**

```bash
bash /Users/homer/dev/docket/.worktrees/deferred-adr-publish-visibility-decide-whether-docket-adr-s/scripts/board-checks.sh \
  --changes-dir /Users/homer/dev/docket/.docket/docs/changes \
  --adrs-dir /Users/homer/dev/docket/.docket/docs/adrs \
  --terminal-publish \
  --metadata-branch origin/docket --integration-branch origin/main
```

Expected: **no `adr-unpublished` line.** ADR-0023 (`change: 44`, `blocked`) and ADR-0060 (`change: 135`, `implemented`) are both correctly not-due; the 58 ADRs present on both branches have identical blobs. Record the exact output.

- [ ] **Step 2: Prove the path is not a swallowed no-op — positive control for the `missing` arm**

Zero findings is indistinguishable from a check that never ran. Copy the metadata tree to a throwaway dir, flip a change to `done` so its ADR becomes due, and watch the finding fire:

```bash
T=$(mktemp -d); cp -R /Users/homer/dev/docket/.docket/docs "$T/docs"
sed -i.bak 's/^status: implemented/status: done/' \
  "$T/docs/changes/active/0135-cursor-agent-wrapper-contract.md"
bash /Users/homer/dev/docket/.worktrees/deferred-adr-publish-visibility-decide-whether-docket-adr-s/scripts/board-checks.sh \
  --changes-dir "$T/docs/changes" --adrs-dir "$T/docs/adrs" --terminal-publish \
  --metadata-branch origin/docket --integration-branch origin/main | grep adr-unpublished
```

Expected: exactly one `adr-unpublished` line naming `ADR-0060`, with change-id column `135`. (The `--adrs-dir` here is inside a copy, so `rev-parse --show-prefix` must still resolve — if the copy is outside a git worktree the command errors; in that case run the control by copying into a scratch clone of the repo instead, and note which route was used.)

- [ ] **Step 3: Positive control for the `stale` arm**

```bash
printf '\n<!-- drift -->\n' >> "$T/docs/adrs/0051-publish-deferred-marker-not-branch-diff-detector.md"
```

Re-run the same command. Expected: an additional `adr-unpublished` line naming `ADR-0051` with the *differs / re-publish* message. Then `rm -rf "$T"`.

If the copy route cannot resolve `--show-prefix`, this control is what proves the stale arm on real bytes; run it inside a scratch clone rather than skipping it.

- [ ] **Step 4: Run the full suite, in the foreground, once**

```bash
cd /Users/homer/dev/docket/.worktrees/deferred-adr-publish-visibility-decide-whether-docket-adr-s
bash tests/run-all.sh
```

Expected: green. This is a single foreground run — never backgrounded.

- [ ] **Step 5: Verify the portability trap and commit nothing new if clean**

The suite's `grep` may be `ugrep` in this shell, which accepts patterns `/usr/bin/grep` rejects. Re-run the two greps this change introduces under the real BSD tool:

```bash
/usr/bin/grep -c "^adr-unpublished" /dev/null; echo "bsd grep ok"
```

and confirm nothing in the new test block uses a bounded repetition or a GNU-only flag. If a portability defect surfaces, fix it and commit; otherwise this task adds no commit and its output feeds the results file.

---

## Self-Review

**Spec coverage.** §4.1 placement → Task 1 (3c, in `board-checks.sh`). §4.2 due rule, all three rows plus both negatives → Task 1 Step 1 asserts + 3c. §4.3 two arms, one check-id → Task 1 (missing) and Task 2 (stale). §4.4 both gate legs → Task 1 (script-side, both flags) and Task 3 (caller-side mode + knob, with the unset case). §4.5 four-site registration plus the `12`→`13` literal → Task 1 (3d, 3e, 3f, and the count edit). §5's open question (the `<change-id>` column) → resolved in Task 1 as the validated `change:` id or ADR-0049's existing `?` fallback, with the ADR number in the message column; the rule is not weakened. §6 testing, including the offline requirement (`rev-parse` on local refs only) and the real-corpus verification the hermetic suite cannot do → Tasks 1, 2, 4. §7 out-of-scope items are respected: no healer, no change-record set-diff, nothing published.

**Placeholder scan.** Every step carries the literal code or command to run and its expected result. No TBD, no "add error handling", no "similar to Task N".

**Type consistency.** `ADRS_DIR` / `ADR_GATE` / `adr_prefix` / `a_rel` / `a_num` / `a_status` / `a_change` / `a_change_id` / `a_cid` / `m_blob` / `i_blob` are introduced in Task 1 and reused unchanged in Task 2. The check-id string is `adr-unpublished` in all seven surfaces. `adr_args` is local to Task 3.
