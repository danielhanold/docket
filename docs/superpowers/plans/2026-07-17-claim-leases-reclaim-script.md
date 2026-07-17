# Claim leases + reclaim script Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make an expired `in-progress` claim self-heal: stamp a claim lease (`claimed_at:`), detect expiry deterministically (independent of whether a feature branch was ever pushed), and flip the safe (no-branch) case back to build-ready `proposed` via a deterministic reclaim script — dissolving both the stuck-forever failure and the resume-claims-a-different-change trap.

**Architecture:** Four cooperating pieces, each defaulted in the spec's §7. (1) A `claimed_at:` UTC-ISO-8601 frontmatter field stamped by `docket-implement-next`'s claim + phase-boundary metadata commits, cleared on any transition out of `in-progress`. (2) A config-layered `reclaim:` block (`lease_ttl` hours, `auto`) resolved by `docket-config.sh --export`, mirroring the `learnings:` block. (3) A new deterministic `scripts/reclaim-claims.sh` (facade op `reclaim-claims`) that reclaims an expired-lease change **only when it has no feature branch** (the crashed-before-push blind spot — the provably collision-free case), under CAS/re-read discipline. (4) `docket-status` wiring: `board-checks.sh`'s `stale-in-progress` upgrades to also key on `claimed_at`+TTL, `docket-status` prints a state-valid recommended reclaim command, and mutation runs only under `reclaim.auto: true` (default off — additive to today's warn-only behavior).

**Tech Stack:** POSIX-ish bash (`set -uo pipefail`, no `set -e`), the shared `scripts/lib/docket-frontmatter.sh` helpers, git plumbing (no `gh`, no network in the mechanical scripts), hermetic bash test harnesses with `NOW`/`GIT` mock seams.

## Global Constraints

- **This repo IS docket.** All edits land on the feature branch `feat/claim-leases-reclaim-script` (cut from `origin/main`). The change file, BOARD, ADRs, and specs live on the `docket` metadata branch and are NOT touched here.
- **Shell portability (promoted AGENTS.md rule):** never `producer | early-exiting-consumer` under `pipefail` (capture into a var, then `grep <<<"$var"`); a `grep` pattern that leads with `--` must use `-e`/`--`; awk indent classes are `[^[:space:]]`. Scripts use `set -uo pipefail` (NOT `set -e`) to match `board-checks.sh`.
- **Frontmatter edits (promoted rule):** anchor every field edit to the first `---…---` block; quote any YAML scalar carrying a colon-space or boolean keyword.
- **Guards are code (promoted rule):** every new test guard must be mutation-tested (strip what it guards, watch it redden); key guards on syntactic shape, not enumerated spellings; run the WHOLE suite at the gate, never only the enumerated tests.
- **`claimed_at:` format is UTC ISO-8601 second-precision:** `YYYY-MM-DDTHH:MM:SSZ` (e.g. `2026-07-17T14:30:05Z`), produced by `date -u +%Y-%m-%dT%H:%M:%SZ`. It is machine-written and machine-read; `updated:`/`created:` stay date-only and human-facing.
- **`reclaim.lease_ttl` is an integer number of HOURS, default `72`** (≥ the existing 3-day `stale-in-progress` window). Validated like `learnings.cap` (non-negative integer, fail-closed). Converted to seconds (×3600) internally.
- **`reclaim.auto` is `true|false`, default `false`,** validated fail-closed like `terminal_publish`. Both `reclaim.*` keys are behavioral (NOT coordination-fenced) — they resolve through the full per-field layering (repo-local > repo-committed > global > built-in), exactly like `learnings.*` (spec §7-H).
- **Learnings that gate this build:** `config-knob-ship-end-to-end` (ship sample config + README + prose in the SAME change), `opt-in-signal-not-file-presence` (gate mutation on `reclaim.auto`, and regression-test that a non-opted-in repo mutates nothing), `presence-encoded-state` (clear `claimed_at`+`branch` on EVERY transition out of in-progress), `printed-remedy-state-validity` (the printed reclaim command must be valid in the exact state that produced it), `metadata-branch-invisible-to-suite` (reclaim operates on metadata-branch files — tests are hermetic via seams; record any metadata-branch-only verification in the results file), `conditional-mkdir-in-loop-aborts-run` / errexit hygiene (`|| continue` any per-item failure point), `check-plumbing-auto-discovery` (the facade does NOT auto-discover — `reclaim-claims` must be hand-added to `WRAPPED_OPS` + `docket.md`, verified by the sentinel), `transient-resource-lifecycle` (any scratch temp file self-heals + diagnostics captured before teardown), `escape-ere-metacharacters-in-key` (do not duplicate the date helper — put it once in the shared lib).
- **No `## Artifacts` regen in reclaim:** reclaim mutates only non-link-bearing fields (`status`/`branch`/`claimed_at`/`reconciled`/`updated`), so per the convention's field-write rule the Artifacts block is NOT regenerated (the spec §3-3's "refreshes the Artifacts block" is superseded — record this in the results file).

---

## File Structure

**New files:**
- `scripts/reclaim-claims.sh` — the deterministic reclaim sweep (core deliverable).
- `scripts/reclaim-claims.md` — its co-located contract (Purpose/Usage/Behavior/Exit codes/Invariants).
- `tests/test_reclaim_claims.sh` — hermetic tests (NOW/GIT seams), mirroring `tests/test_board_checks.sh`.

**Modified — scripts:**
- `scripts/lib/docket-frontmatter.sh` — add the shared `iso_to_epoch` helper (single source; both new + upgraded scripts use it).
- `scripts/docket-config.sh` — resolve + export `RECLAIM_LEASE_TTL` / `RECLAIM_AUTO` (mirror the `learnings:` block).
- `scripts/docket-config.md` — document the two keys.
- `scripts/docket.sh` — add `reclaim-claims` to `WRAPPED_OPS` + the usage comment.
- `scripts/docket.md` — add the `reclaim-claims` inventory-table row.
- `scripts/board-checks.sh` — add `--lease-ttl-hours N`; upgrade `stale-in-progress` to key on `claimed_at`+TTL and mark the reclaimable case.
- `scripts/board-checks.md` — document the upgrade + the `[reclaimable]` message marker.
- `scripts/docket-status.sh` — pass `--lease-ttl-hours`, print the state-valid remedy, invoke reclaim under `reclaim.auto`.
- `scripts/docket-status.md` — document the reclaim wiring.
- `scripts/archive-change.sh` — clear `claimed_at:` on terminal transition (presence-encoded-state).

**Modified — skills (mind `tests/test_skill_size_budgets.sh`):**
- `skills/docket-convention/SKILL.md` — add `claimed_at:` to the manifest, `## Reclaim log` to the body-sections list, and the sanctioned `in-progress → proposed` reverse edge (diagram + rules prose).
- `skills/docket-implement-next/SKILL.md` — stamp `claimed_at` at claim (Step 2) + re-stamp at phase-boundary commits (reconcile Step 3, `implemented` Step 7).

**Modified — ship end-to-end + tests:**
- `.docket.yml` — add the commented `reclaim:` block (this repo's own config doubles as the sample).
- `config.yml.example` — add the commented `reclaim:` block (global-layer sample).
- `README.md` — document the `reclaim:` knob + the reclaim command + the new lifecycle edge.
- `tests/test_docket_config.sh` — assert the new export keys + defaults + fail-closed validation.
- `tests/test_board_checks.sh` — new `stale-in-progress` cases (expired no-branch reclaimable; expired with-branch; not-expired).
- `tests/test_docket_facade.sh` — add `reclaim-claims` to the stub-helper list (routing test).
- `tests/test_docket_frontmatter.sh` — test `iso_to_epoch`.
- `tests/test_closeout.sh` — assert `claimed_at:` is cleared on archive.

---

## Task 1: `iso_to_epoch` shared helper

**Files:**
- Modify: `scripts/lib/docket-frontmatter.sh` (append a new helper after `has_section`, ~line 37)
- Test: `tests/test_docket_frontmatter.sh`

**Interfaces:**
- Produces: `iso_to_epoch "<YYYY-MM-DDTHH:MM:SSZ>"` → prints epoch seconds on stdout and returns 0 on success; prints nothing and returns 1 on parse failure. Portable across GNU and BSD/macOS `date`.

- [ ] **Step 1: Write the failing test.** Add to `tests/test_docket_frontmatter.sh` (follow its existing `assert`/sourcing pattern):

```bash
# --- iso_to_epoch: portable UTC ISO-8601 -> epoch ---
# Derive the oracle from the host's own date (GNU or BSD) so the test is host-portable —
# compare iso_to_epoch against that, never against a hardcoded epoch constant.
known="2026-07-17T12:00:00Z"
oracle="$(TZ=UTC date -u -d "$known" +%s 2>/dev/null || date -u -j -f '%Y-%m-%dT%H:%M:%SZ' "$known" +%s 2>/dev/null)"
got="$(iso_to_epoch "$known")"
assert "iso_to_epoch parses a UTC ISO-8601 timestamp" '[ -n "$got" ] && [ "$got" = "$oracle" ]'
assert "iso_to_epoch returns nonzero + empty on garbage" '! iso_to_epoch "not-a-timestamp" >/dev/null 2>&1'
assert "iso_to_epoch returns empty string on garbage" '[ -z "$(iso_to_epoch "not-a-timestamp" 2>/dev/null)" ]'
```

- [ ] **Step 2: Run it and confirm it fails.** Run: `bash tests/test_docket_frontmatter.sh`. Expected: the new asserts print `NOT OK` (function not defined ⇒ empty output / nonzero).

- [ ] **Step 3: Implement the helper** in `scripts/lib/docket-frontmatter.sh` (after `has_section(){ ... }`):

```bash
# iso_to_epoch ISO — convert a UTC ISO-8601 second-precision timestamp (YYYY-MM-DDTHH:MM:SSZ) to
# epoch seconds on stdout. Tries GNU date first, then BSD/macOS date. Returns 1 (empty stdout) on
# a parse failure — callers treat "no epoch" as "no positive evidence" (never as expired). Single
# source: both board-checks.sh and reclaim-claims.sh use it (do NOT duplicate — escape-ere twin rule).
iso_to_epoch(){
  local iso="$1" e
  e="$(date -u -d "$iso" +%s 2>/dev/null)"                         && { printf '%s' "$e"; return 0; }
  e="$(date -u -j -f '%Y-%m-%dT%H:%M:%SZ' "$iso" +%s 2>/dev/null)" && { printf '%s' "$e"; return 0; }
  return 1
}
```

- [ ] **Step 4: Run it and confirm it passes.** Run: `bash tests/test_docket_frontmatter.sh`. Expected: all asserts `ok`, exit 0.

- [ ] **Step 5: Commit.**

```bash
git add scripts/lib/docket-frontmatter.sh tests/test_docket_frontmatter.sh
git commit -m "feat(reclaim): add portable iso_to_epoch helper to the shared frontmatter lib"
```

---

## Task 2: `reclaim:` config resolution + export

**Files:**
- Modify: `scripts/docket-config.sh` (add a `reclaim:` block after the `learnings:` block, ~line 317; add two `emit` lines in the export section, ~line 384)
- Modify: `scripts/docket-config.md`
- Test: `tests/test_docket_config.sh`

**Interfaces:**
- Produces: two new `--export` lines — `RECLAIM_LEASE_TTL=<int hours, default 72>` and `RECLAIM_AUTO=<true|false, default false>` — resolved per-field (repo-local > repo-committed > global > built-in), read within a `reclaim:` block via `yaml_block_body` (never as bare top-level keys). Consumed by `docket-status.sh` (Task 8).

- [ ] **Step 1: Write the failing test.** Add to `tests/test_docket_config.sh` (follow its fixture pattern — a temp repo with a committed `.docket.yml`; reuse the existing helper that builds one). Assert defaults and overrides and fail-closed:

```bash
# reclaim defaults (no reclaim: block)
assert "RECLAIM_LEASE_TTL defaults to 72" 'echo "$out" | grep -qxF "RECLAIM_LEASE_TTL=72"'
assert "RECLAIM_AUTO defaults to false"   'echo "$out" | grep -qxF "RECLAIM_AUTO=false"'
# reclaim: block honored (write reclaim:\n  lease_ttl: 12\n  auto: true into the committed .docket.yml)
assert "RECLAIM_LEASE_TTL reads the block" 'echo "$out2" | grep -qxF "RECLAIM_LEASE_TTL=12"'
assert "RECLAIM_AUTO reads the block"      'echo "$out2" | grep -qxF "RECLAIM_AUTO=true"'
# fail-closed on garbage
assert "non-integer lease_ttl aborts nonzero" '! run_resolver_with "reclaim:\n  lease_ttl: soon\n" >/dev/null 2>&1'
assert "non-bool auto aborts nonzero"         '! run_resolver_with "reclaim:\n  auto: maybe\n" >/dev/null 2>&1'
```

(Match the exact fixture helpers already in `tests/test_docket_config.sh`; do not invent new harness names — read the file first and reuse its `mk_repo`/`resolve` equivalents.)

- [ ] **Step 2: Run it and confirm it fails.** Run: `bash tests/test_docket_config.sh`. Expected: the four `RECLAIM_*` asserts `NOT OK` (keys absent).

- [ ] **Step 3: Implement resolution** in `scripts/docket-config.sh`, immediately after the `learnings:` block ends (after the `LEARNINGS_CAP` validation `case`, ~line 317). Mirror the `learnings:` block EXACTLY (block parse + per-field precedence + fail-closed), and re-issue the EXIT trap to include the new temp files:

```bash
# --- reclaim: the claim-lease self-heal subsystem (change 0089) ----------------
# Nested block parsed exactly like learnings: — each leaf read WITHIN the block via yaml_block_body
# (never a bare top-level key: `auto` is a generic word a future block could shadow). BOTH keys are
# behavioral, NOT coordination-fenced (spec §7-H): they resolve through the full per-field layering
# repo-local > repo-committed > global > built-in, like learnings.* / auto_groom. lease_ttl is an
# integer number of HOURS (converted to seconds by the consumers); auto gates the ONLY mutating path.
RECLAIM_BLK="$(mktemp)";  yaml_block_body "$CFG"  reclaim >"$RECLAIM_BLK"
GRECLAIM_BLK="$(mktemp)"; yaml_block_body "$GCFG" reclaim >"$GRECLAIM_BLK"
LRECLAIM_BLK="$(mktemp)"; yaml_block_body "$LCFG" reclaim >"$LRECLAIM_BLK"
trap 'rm -f "$CFG" "$LEARN_BLK" "$GLEARN_BLK" "$LLEARN_BLK" "$RECLAIM_BLK" "$GRECLAIM_BLK" "$LRECLAIM_BLK"' EXIT
reclaim_key(){  # reclaim_key <leaf> <default> -> resolved value on stdout
  local v; v="$(yaml_get "$LRECLAIM_BLK" "$1")"
  [ -n "$v" ] || v="$(yaml_get "$RECLAIM_BLK" "$1")"
  [ -n "$v" ] || v="$(yaml_get "$GRECLAIM_BLK" "$1")"
  printf '%s' "${v:-$2}"
}
RECLAIM_LEASE_TTL="$(reclaim_key lease_ttl 72)"
RECLAIM_AUTO="$(reclaim_key auto false)"
case "$RECLAIM_LEASE_TTL" in
  ''|*[!0-9]*) die "unparseable config: reclaim.lease_ttl must be a non-negative integer (hours), got '$RECLAIM_LEASE_TTL'" ;;
esac
case "$RECLAIM_AUTO" in
  true|false) ;;
  *) die "unparseable config: reclaim.auto must be 'true' or 'false', got '$RECLAIM_AUTO'" ;;
esac
```

- [ ] **Step 4: Add the export lines** in the `emit` section (after `emit TERMINAL_PUBLISH "$TERMINAL_PUBLISH"`, ~line 384):

```bash
  emit RECLAIM_LEASE_TTL "$RECLAIM_LEASE_TTL"
  emit RECLAIM_AUTO "$RECLAIM_AUTO"
```

- [ ] **Step 5: Document** the two keys in `scripts/docket-config.md` alongside the other resolved keys (find the resolved-keys table/list and add `RECLAIM_LEASE_TTL` and `RECLAIM_AUTO` with their defaults and the "hours / behavioral, not fenced" notes).

- [ ] **Step 6: Run tests.** Run: `bash tests/test_docket_config.sh`. Expected: all `ok`, exit 0. Also run `bash tests/test_docket_facade.sh` (its `env` fixture asserts a clean resolve) — expected still green.

- [ ] **Step 7: Commit.**

```bash
git add scripts/docket-config.sh scripts/docket-config.md tests/test_docket_config.sh
git commit -m "feat(reclaim): resolve + export RECLAIM_LEASE_TTL and RECLAIM_AUTO"
```

---

## Task 3: `reclaim-claims.sh` — the deterministic reclaim sweep

**Files:**
- Create: `scripts/reclaim-claims.sh`
- Test: `tests/test_reclaim_claims.sh`

**Interfaces:**
- Produces: `reclaim-claims.sh --changes-dir DIR --lease-ttl-hours N [--remote R]`. Seams `GIT="${GIT:-git}"`, `NOW="${NOW:-$(date +%s)}"`. Sources `lib/docket-frontmatter.sh` (uses `field`, `int_field`, `iso_to_epoch`, `set_field`-style edits). For each `active/*.md` at `status: in-progress`: reclaim iff `claimed_at:` present AND `NOW - iso_to_epoch(claimed_at) > N*3600` AND no `feat/<slug>` ref (local `refs/heads/` OR `refs/remotes/origin/`). On reclaim: append `## Reclaim log`, `status→proposed`, clear `branch:`+`claimed_at:`, `reconciled→false`, `updated→<UTC today>`; commit CHANGE-FILE-ONLY + CAS-push; **re-read eligibility after any non-fast-forward** and skip if no longer eligible. Emits one report line per change: `reclaimed <id> <slug> (lease ${age}h, no branch)` or `skipped <id> raced` or nothing. Exit 0 on a clean sweep (including zero reclaims).

- [ ] **Step 1: Write the failing test** `tests/test_reclaim_claims.sh`. Model it on `tests/test_board_checks.sh` (read that file first for the exact fixture/seam idiom). Build a real temp git repo with `docs/changes/active/*.md` fixtures, drive `NOW`/`GIT` seams, and assert:

```bash
# Fixture helper writes an active change with a given status/branch/claimed_at.
# CASE A — expired lease, NO branch => reclaimed.
#   claimed_at = NOW - 100h (ttl 72) ; branch: feat/a but NO ref exists.
assert "expired + no branch is reclaimed to proposed" \
  '[ "$(field "$WT/docs/changes/active/0001-a.md" status)" = proposed ]'
assert "reclaim clears branch"      '[ -z "$(field "$WT/docs/changes/active/0001-a.md" branch)" ]'
assert "reclaim clears claimed_at"  '[ -z "$(field "$WT/docs/changes/active/0001-a.md" claimed_at)" ]'
assert "reclaim resets reconciled"  '[ "$(field "$WT/docs/changes/active/0001-a.md" reconciled)" = false ]'
assert "reclaim appends a Reclaim log section" 'grep -qF "## Reclaim log" "$WT/docs/changes/active/0001-a.md"'
assert "reclaim reports the change" 'printf "%s" "$out" | grep -qE "^reclaimed 1 "'
# CASE B — expired lease, branch REF EXISTS => NOT reclaimed (orphan/collision guard).
assert "expired + branch ref present is left in-progress" \
  '[ "$(field "$WT/docs/changes/active/0002-b.md" status)" = in-progress ]'
# CASE C — NOT expired (claimed_at = NOW - 1h) => NOT reclaimed.
assert "fresh lease is left in-progress" \
  '[ "$(field "$WT/docs/changes/active/0003-c.md" status)" = in-progress ]'
# CASE D — NO claimed_at (pre-migration) => NEVER reclaimed (no positive evidence).
assert "no claimed_at is never reclaimed" \
  '[ "$(field "$WT/docs/changes/active/0004-d.md" status)" = in-progress ]'
# CASE E — a proposed change is untouched.
assert "a proposed change is ignored" \
  '[ "$(field "$WT/docs/changes/active/0005-e.md" status)" = proposed ]'
```

Use a `GIT` seam wrapper (as `test_board_checks.sh` does) so "branch ref exists" is deterministic: CASE A's `feat/a` must resolve to NO ref; CASE B's `feat/b` must resolve to a ref. The cleanest is a REAL repo where you actually create `refs/heads/feat/b` for CASE B and leave CASE A's branch absent — then no `GIT` mock is needed for the ref probe (only `NOW` is mocked). Push target: create a bare origin and clone so `cas_push` succeeds.

- [ ] **Step 2: Run it and confirm it fails.** Run: `bash tests/test_reclaim_claims.sh`. Expected: fails (script missing).

- [ ] **Step 3: Implement `scripts/reclaim-claims.sh`.** Full script:

```bash
#!/usr/bin/env bash
# scripts/reclaim-claims.sh — deterministic claim-lease reclaim (change 0089). Sweeps active/*.md for
# in-progress changes whose claim lease (claimed_at:) is EXPIRED and that have NO feature branch (the
# crashed-before-push blind spot — the one case reclaim is provably collision-free and orphan-free),
# and flips them back to build-ready `proposed` so the queue self-heals. Git-only (no gh, no network);
# reads the metadata working tree it is pointed at. Mutation is the caller's choice (docket-status runs
# it only under reclaim.auto; a human runs `docket.sh reclaim-claims` explicitly). ADR-0012: a
# deterministic script, never model prose. ADR-0021: authors its own mechanical commit.
#
# Usage: reclaim-claims.sh --changes-dir DIR --lease-ttl-hours N [--remote R]
#   Reclaimable iff (1) status: in-progress, (2) claimed_at present AND NOW-claimed_at > N*3600,
#   AND (3) no feat/<slug> ref resolves (refs/heads OR refs/remotes/origin). A change with no
#   claimed_at is NEVER reclaimed (no positive evidence of expiry). Report: one line per change on
#   stdout — "reclaimed <id> <slug> (lease <age>h, no branch)" | "skipped <id> raced". Exit 0.
#   Mock seams: GIT="${GIT:-git}"; NOW="${NOW:-$(date +%s)}".
set -uo pipefail
GIT="${GIT:-git}"
NOW="${NOW:-$(date +%s)}"
CHANGES_DIR=""; TTL_HOURS=""; REMOTE="origin"
die(){ printf '%s\n' "reclaim-claims: $*" >&2; exit 1; }
while [ $# -gt 0 ]; do
  case "$1" in
    --changes-dir) CHANGES_DIR="$2"; shift ;;
    --lease-ttl-hours) TTL_HOURS="$2"; shift ;;
    --remote) REMOTE="$2"; shift ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) die "unknown argument: $1" ;;
  esac
  shift
done
[ -n "$CHANGES_DIR" ] || die "missing --changes-dir"
[ -d "$CHANGES_DIR" ] || die "changes dir not found: $CHANGES_DIR"
case "$TTL_HOURS" in ''|*[!0-9]*) die "missing/invalid --lease-ttl-hours" ;; esac

# shellcheck source=/dev/null
. "$(dirname "${BASH_SOURCE[0]}")/lib/docket-frontmatter.sh"

WT="$($GIT -C "$CHANGES_DIR" rev-parse --show-toplevel)" || die "not a git worktree: $CHANGES_DIR"
REL_ABS="$(cd "$CHANGES_DIR" && pwd -P)"; REL="${REL_ABS#"$WT"/}"
TTL_SECS=$(( TTL_HOURS * 3600 ))
TODAY="$(date -u +%Y-%m-%d)"

set_field(){ # set_field FILE KEY VALUE — first ---…--- block only (frontmatter-edit-anchor rule)
  local f="$1" k="$2" v="$3" t; t="$(mktemp)"
  sed -E "/^---$/,/^---$/ s|^($k:)[[:space:]]*.*|\1 $v|" "$f" > "$t" && mv "$t" "$f"
}
branch_exists(){ # $1 = branch name; 0 iff a local OR origin-tracking ref resolves
  local b="$1"; [ -n "$b" ] || return 1
  $GIT -C "$WT" show-ref --verify --quiet "refs/heads/$b"          && return 0
  $GIT -C "$WT" show-ref --verify --quiet "refs/remotes/origin/$b" && return 0
  return 1
}
cur_branch="$($GIT -C "$WT" rev-parse --abbrev-ref HEAD)"
cas_push(){ until $GIT -C "$WT" push "$REMOTE" "$cur_branch"; do
  $GIT -C "$WT" pull --rebase "$REMOTE" "$cur_branch" || die "rebase during push failed"; done; }

# eligible FILE -> prints "<age_hours>" and returns 0 iff reclaimable; returns 1 otherwise.
eligible(){
  local f="$1" status claimed epoch branch
  status="$(field "$f" status)"; [ "$status" = "in-progress" ] || return 1
  claimed="$(field "$f" claimed_at)"; [ -n "$claimed" ] || return 1   # no positive evidence
  epoch="$(iso_to_epoch "$claimed")" || return 1
  [ "$(( NOW - epoch ))" -gt "$TTL_SECS" ] || return 1                 # not expired
  branch="$(field "$f" branch)"
  branch_exists "$branch" && return 1                                  # has a branch => never here
  printf '%s' "$(( (NOW - epoch) / 3600 ))"; return 0
}

shopt -s nullglob
for f in "$WT/$REL/active/"*.md; do
  age="$(eligible "$f")" || continue
  id="$(int_field "$f" id)"; [ -n "$id" ] || continue
  base="$(basename "$f")"; slug="${base%.md}"; slug="${slug#*-}"
  # Append the dated Reclaim log entry (new body section; parallel to ## Reconcile log).
  { printf '\n## Reclaim log\n\n### %s — reclaimed by reclaim-claims.sh\n\n' "$TODAY"
    printf 'Claim lease expired (~%sh since claimed_at, TTL %sh) and no feature branch ref was found; '
    printf 'flipped in-progress → proposed so the change re-enters selection.\n' "$age" "$TTL_HOURS"
  } >> "$f"
  set_field "$f" status proposed
  set_field "$f" branch ""
  set_field "$f" claimed_at ""
  set_field "$f" reconciled false
  set_field "$f" updated "$TODAY"
  $GIT -C "$WT" add "$REL/active/$base"
  $GIT -C "$WT" commit -q -m "docket($(printf '%04d' "$id")): reclaim — expired lease, no branch; back to proposed" -- "$REL/active/$base" || die "commit failed for $id"
  # CAS: push; on non-ff, rebase then RE-READ eligibility — skip if a concurrent writer changed it.
  if ! $GIT -C "$WT" push "$REMOTE" "$cur_branch" 2>/dev/null; then
    $GIT -C "$WT" pull --rebase "$REMOTE" "$cur_branch" || die "rebase during push failed for $id"
    if ! eligible "$f" >/dev/null; then
      printf 'skipped %s raced\n' "$id"; continue   # advanced under us; our commit still rides the rebase
    fi
    cas_push
  fi
  printf 'reclaimed %s %s (lease %sh, no branch)\n' "$id" "$slug" "$age"
done
shopt -u nullglob
exit 0
```

Note the `## Reclaim log` `printf` ordering above is a plan sketch — implement the here-doc so `%s` args (`$age`, `$TTL_HOURS`, `$TODAY`) bind correctly; prefer a single `cat <<EOF`/`printf` with explicit args. Verify the appended text renders as intended before committing.

- [ ] **Step 4: Run it and confirm it passes.** Run: `chmod +x scripts/reclaim-claims.sh && bash tests/test_reclaim_claims.sh`. Expected: all `ok`, exit 0. Also assert a LATER fixture still processes after an earlier skip (errexit hygiene — `|| continue` proven).

- [ ] **Step 5: Write the contract** `scripts/reclaim-claims.md` (Purpose / Usage / Behavior / Exit codes / Invariants), following a sibling like `scripts/board-checks.md`. State the two eligibility conditions, the no-`claimed_at`⇒never rule, the no-branch narrowing (orphan/collision guard), the CAS re-read, and the report grammar.

- [ ] **Step 6: Commit.**

```bash
git add scripts/reclaim-claims.sh scripts/reclaim-claims.md tests/test_reclaim_claims.sh
git commit -m "feat(reclaim): deterministic reclaim-claims.sh sweep for expired no-branch leases"
```

---

## Task 4: Facade wiring (`docket.sh reclaim-claims`)

**Files:**
- Modify: `scripts/docket.sh:38` (the `WRAPPED_OPS` array) + the usage comment block (~line 26)
- Modify: `scripts/docket.md` (the inventory table)
- Modify: `tests/test_docket_facade.sh:14-16` (stub-helper list)

**Interfaces:**
- Produces: `docket.sh reclaim-claims [args]` routes verbatim to `scripts/reclaim-claims.sh`. The op name == script basename (facade invariant, enforced by the sentinel at `tests/test_docket_facade.sh:154-157`).

- [ ] **Step 1: Add the op** to `WRAPPED_OPS` in `scripts/docket.sh` (insert `reclaim-claims` — keep it a single space-separated token in the string):

```bash
WRAPPED_OPS="docket-status board-refresh archive-change terminal-publish cleanup-feature-branch github-mirror sync-integration-branch render-change-links render-adr-index render-learnings-index adr-checks board-checks reclaim-claims runner-dispatch setup-auto-approve"
```

- [ ] **Step 2: Add the usage line** in the `# Usage:` comment (near `board-checks`, ~line 24):

```
#   reclaim-claims [args]     reclaim expired-lease, no-branch in-progress claims back to proposed
```

- [ ] **Step 3: Add the inventory row** to `scripts/docket.md` (match the existing `| \`op\` | … |` table shape so the sentinel's `md_ops` grep picks it up):

```
| `reclaim-claims` | reclaim expired-lease, no-branch in-progress claims back to `proposed` |
```

- [ ] **Step 4: Add the stub** to `tests/test_docket_facade.sh`'s stub-helper `for h in …` list (line 14-16) so the routing test can exercise it:

```bash
         adr-checks board-checks reclaim-claims docket-config setup-auto-approve; do
```

- [ ] **Step 5: Run the facade tests.** Run: `bash tests/test_docket_facade.sh`. Expected: all `ok` — including the sentinel `every wrapped op maps to scripts/<op>.sh` (now that `scripts/reclaim-claims.sh` exists from Task 3) and `docket.sh op set == docket.md documented op set`.

- [ ] **Step 6: Commit.**

```bash
git add scripts/docket.sh scripts/docket.md tests/test_docket_facade.sh
git commit -m "feat(reclaim): route docket.sh reclaim-claims to the reclaim sweep"
```

---

## Task 5: `board-checks.sh` — upgrade `stale-in-progress` to key on the lease

**Files:**
- Modify: `scripts/board-checks.sh` (arg parsing ~line 19-34; the `stale-in-progress` block ~line 73-83)
- Modify: `scripts/board-checks.md`
- Test: `tests/test_board_checks.sh`

**Interfaces:**
- Consumes: `iso_to_epoch` (Task 1). New flag `--lease-ttl-hours N` (default 72 when absent, so standalone use stays sane).
- Produces: `stale-in-progress` findings that ALSO fire on `claimed_at`+TTL expiry. The reclaimable case (expired AND no branch ref) carries the stable marker `[reclaimable]` at the END of its message — the machine-readable contract `docket-status` (Task 6) keys on. Existing branch-idle finding is preserved; at most one `stale-in-progress` line per change.

- [ ] **Step 1: Write the failing tests** in `tests/test_board_checks.sh` (extend its existing in-progress fixtures; it already drives `NOW`). Add three cases:

```bash
# expired lease + NO branch ref => stale-in-progress carrying [reclaimable]
assert "expired no-branch lease is flagged reclaimable" \
  'printf "%s\n" "$out" | grep -E "^stale-in-progress\s+<id>\s" | grep -qF "[reclaimable]"'
# expired lease + branch ref EXISTS => stale-in-progress WITHOUT [reclaimable]
assert "expired with-branch lease is flagged, not reclaimable" \
  'printf "%s\n" "$out" | grep -E "^stale-in-progress\s+<id2>\s" | grep -qvF "[reclaimable]"'
# fresh lease, no branch => NO stale-in-progress finding
assert "fresh lease produces no stale finding" \
  '! printf "%s\n" "$out" | grep -qE "^stale-in-progress\s+<id3>\s"'
# regression: existing branch-idle >3d case still fires (unchanged)
```

(Use the file's real fixture helper + the tab-separated finding shape it already asserts; replace `<id>` etc. with the fixtures' ids.)

- [ ] **Step 2: Run and confirm failure.** Run: `bash tests/test_board_checks.sh`. Expected: the new asserts `NOT OK`.

- [ ] **Step 3: Add the flag** to `board-checks.sh` arg parsing:

```bash
    --lease-ttl-hours) LEASE_TTL_HOURS="$2"; shift ;;
```

and initialize near the other vars: `LEASE_TTL_HOURS="${LEASE_TTL_HOURS:-72}"` (after the parse loop, default when unset). Update the usage `# Usage:` comment to list it.

- [ ] **Step 4: Rewrite the `stale-in-progress` block** (lines 73-83) to key on the lease AND keep the branch-age signal, emitting at most one finding:

```bash
  # --- stale-in-progress: lease expired (claimed_at+TTL) OR branch idle >3 days ---
  # Complements the branch-age signal with a claimed_at signal that catches the crashed-BEFORE-branch
  # blind spot (branch ref absent). The reclaimable subset (expired AND no branch ref) carries the
  # trailing [reclaimable] marker — the machine contract docket-status keys on for its remedy print.
  if [ "$status" = "in-progress" ]; then
    branch="$(field "$f" branch)"
    claimed="$(field "$f" claimed_at)"
    has_branch=0
    if [ -n "$branch" ]; then
      if "$GIT" -C "$CHANGES_DIR" show-ref --verify --quiet "refs/heads/$branch" \
         || "$GIT" -C "$CHANGES_DIR" show-ref --verify --quiet "refs/remotes/origin/$branch"; then
        has_branch=1
      fi
    fi
    lease_secs="$(( LEASE_TTL_HOURS * 3600 ))"
    expired=0; age_h=""
    if [ -n "$claimed" ]; then
      cepoch="$(iso_to_epoch "$claimed")" || cepoch=""
      if [ -n "$cepoch" ] && [ "$(( NOW - cepoch ))" -gt "$lease_secs" ]; then
        expired=1; age_h="$(( (NOW - cepoch) / 3600 ))"
      fi
    fi
    if [ "$has_branch" = 1 ]; then
      ts="$("$GIT" -C "$CHANGES_DIR" log -1 --format=%ct "$branch" 2>/dev/null)"
      if [ -n "$ts" ] && [ "$(( NOW - ts ))" -gt "$(( 3*86400 ))" ]; then
        emit stale-in-progress "$id" "branch $branch idle >3 days (last commit $(( (NOW - ts) / 86400 ))d ago)"
      elif [ "$expired" = 1 ]; then
        emit stale-in-progress "$id" "claim lease expired ${age_h}h ago; branch $branch exists — needs your review (not auto-reclaimable)"
      fi
    elif [ "$expired" = 1 ]; then
      emit stale-in-progress "$id" "claim lease expired ${age_h}h ago; no feature branch — self-heal with docket.sh reclaim-claims [reclaimable]"
    fi
  fi
```

- [ ] **Step 5: Run and confirm pass.** Run: `bash tests/test_board_checks.sh`. Expected: all `ok`, including the preserved branch-idle regression.

- [ ] **Step 6: Document** in `scripts/board-checks.md`: the `--lease-ttl-hours` flag, the two `stale-in-progress` triggers, and the `[reclaimable]` marker as a stable machine-readable suffix.

- [ ] **Step 7: Commit.**

```bash
git add scripts/board-checks.sh scripts/board-checks.md tests/test_board_checks.sh
git commit -m "feat(reclaim): stale-in-progress keys on claim lease + marks reclaimable"
```

---

## Task 6: `docket-status.sh` — remedy print + opt-in reclaim mutation

**Files:**
- Modify: `scripts/docket-status.sh` (`health_checks` ~line 504-518; `main` full path ~line 658-682)
- Modify: `scripts/docket-status.md`
- Test: `tests/test_docket_status.sh`

**Interfaces:**
- Consumes: `RECLAIM_LEASE_TTL` / `RECLAIM_AUTO` (resolved into the shell by `docket_preflight`'s `eval`, Task 2); the `[reclaimable]` marker from board-checks (Task 5); `reclaim-claims.sh` (Task 3).
- Produces: (a) `health_checks` passes `--lease-ttl-hours "$RECLAIM_LEASE_TTL"` to `board-checks.sh`; (b) after health checks on the FULL path, when `RECLAIM_AUTO != true` and ≥1 `[reclaimable]` finding exists, prints ONE state-valid remedy line; (c) when `RECLAIM_AUTO = true`, invokes `reclaim-claims.sh` (mutating) and prints its report lines prefixed `reclaim …`. Never on `--board-only`.

- [ ] **Step 1: Write the failing tests** in `tests/test_docket_status.sh` (read its harness first; it stubs board-checks / mocks config). Assert:

```bash
# health_checks forwards the ttl
assert "board-checks is called with --lease-ttl-hours" 'printf "%s" "$bc_argv" | grep -qF -- "--lease-ttl-hours"'
# auto OFF + a [reclaimable] finding => remedy printed, NO mutation
assert "auto off prints the reclaim remedy" 'printf "%s\n" "$out" | grep -qF "docket.sh reclaim-claims"'
assert "auto off does NOT invoke reclaim-claims" '! printf "%s" "$reclaim_called" | grep -q 1'
# auto ON => reclaim-claims invoked, report surfaced; remedy line NOT printed
assert "auto on invokes reclaim-claims" 'printf "%s" "$reclaim_called_auto" | grep -q 1'
# --board-only never triggers either
assert "--board-only runs no reclaim" '! printf "%s\n" "$bo_out" | grep -qF "reclaim"'
```

Follow the file's existing seam approach for stubbing `board-checks.sh` / `reclaim-claims.sh` (e.g. a `SCRIPTS_DIR` stub dir, or the mock pattern already present). Mutation-test each new assert.

- [ ] **Step 2: Run and confirm failure.** Run: `bash tests/test_docket_status.sh`.

- [ ] **Step 3: Forward the TTL** in `health_checks` (line 510-512):

```bash
  "$SCRIPTS_DIR"/board-checks.sh \
    --changes-dir "$cd_dir" --metadata-branch "$metadata_branch" \
    --integration-branch "origin/$INTEGRATION_BRANCH" \
    --lease-ttl-hours "${RECLAIM_LEASE_TTL:-72}" 2>&2 | \
```

- [ ] **Step 4: Add a `reclaim_pass` helper** (near `health_checks`) and wire it into `main`'s FULL path only. `health_checks` currently streams findings to stdout; capture whether any `[reclaimable]` line appeared. Simplest robust approach: have `main` capture health output into a variable, print it, then branch:

```bash
# In main(), REPLACE the bare `health_checks` call on the full path with:
  local health_out
  health_out="$(health_checks)"
  [ -n "$health_out" ] && printf '%s\n' "$health_out"
  reclaim_pass "$health_out"
```

and define:

```bash
# reclaim_pass HEALTH_OUT — opt-in mutation OR a state-valid remedy line. Full path only.
# Keys the remedy on the SAME condition that gates the write (a [reclaimable] finding exists),
# so the printed command is valid in exactly the state that produced it (printed-remedy-state-validity).
reclaim_pass(){
  local health_out="$1" mw cd_dir
  printf '%s\n' "$health_out" | grep -qF "[reclaimable]" || return 0
  if [ "${RECLAIM_AUTO:-false}" = true ]; then
    mw="$(docket_metadata_worktree)"; cd_dir="$mw/$CHANGES_DIR"
    local line
    while IFS= read -r line; do [ -n "$line" ] && printf 'reclaim %s\n' "$line"; done \
      < <("$SCRIPTS_DIR"/reclaim-claims.sh --changes-dir "$cd_dir" --lease-ttl-hours "${RECLAIM_LEASE_TTL:-72}")
  else
    printf 'reclaim: %s expired-lease change(s) can self-heal — run: docket.sh reclaim-claims\n' \
      "$(printf '%s\n' "$health_out" | grep -cF "[reclaimable]")"
  fi
}
```

(Capture-then-grep — never `health_checks | grep -q` — honors the no-pipefail-SIGPIPE rule. Confirm `docket_metadata_worktree` is the same helper `health_checks` uses.)

- [ ] **Step 5: Run and confirm pass.** Run: `bash tests/test_docket_status.sh`. Expected: all `ok`.

- [ ] **Step 6: Document** in `scripts/docket-status.md`: the TTL forwarding, the opt-in `reclaim.auto` mutation, and the state-valid remedy line (full path only; never `--board-only`).

- [ ] **Step 7: Commit.**

```bash
git add scripts/docket-status.sh scripts/docket-status.md tests/test_docket_status.sh
git commit -m "feat(reclaim): docket-status prints reclaim remedy + runs reclaim under reclaim.auto"
```

---

## Task 7: `archive-change.sh` — clear `claimed_at` on terminal transition

**Files:**
- Modify: `scripts/archive-change.sh` (frontmatter section ~line 99-103; self-verification ~line 114-121)
- Test: `tests/test_closeout.sh`

**Interfaces:**
- Produces: on `done` or `killed`, the archived change has `claimed_at:` cleared (presence-encoded-state: every field encoding "claimed" is removed on the transition out). No-op when the field is absent (pre-migration changes).

- [ ] **Step 1: Write the failing test** in `tests/test_closeout.sh` (it already drives `archive-change.sh`). Seed a fixture change with `claimed_at: 2026-07-17T10:00:00Z`, archive it, and assert:

```bash
assert "archive clears claimed_at" '[ -z "$(field "$archived_file" claimed_at)" ]'
```

- [ ] **Step 2: Run and confirm failure.** Run: `bash tests/test_closeout.sh`.

- [ ] **Step 3: Clear the field** in `archive-change.sh` after `set_field "$dest" updated "$DATE"` (line 100):

```bash
set_field "$dest" claimed_at ""   # presence-encoded-state: drop the lease on the terminal transition
```

- [ ] **Step 4: Run and confirm pass.** Run: `bash tests/test_closeout.sh`. Expected: all `ok`. Confirm the idempotent-reuse path and existing postconditions stay green.

- [ ] **Step 5: Commit.**

```bash
git add scripts/archive-change.sh tests/test_closeout.sh
git commit -m "feat(reclaim): clear claimed_at on terminal archive transition"
```

---

## Task 8: `docket-implement-next` skill — stamp/refresh `claimed_at`

**Files:**
- Modify: `skills/docket-implement-next/SKILL.md` (Step 2 claim; Step 3 reconcile; Step 7 `implemented`)
- Test: `tests/test_skill_size_budgets.sh` (must stay green — keep edits terse)

**Interfaces:**
- Produces: skill instructions so a future run stamps `claimed_at: <UTC ISO-8601 now>` (via `date -u +%Y-%m-%dT%H:%M:%SZ`) into the change frontmatter at the claim commit, and re-stamps it at the reconcile and `implemented` metadata commits (the poor-man's heartbeat). The field is a plain frontmatter scalar — the skill adds it if absent (existing changes predate it), always in the first `---…---` block.

- [ ] **Step 1: Check the size budget headroom.** Run: `bash tests/test_skill_size_budgets.sh` and note the current margin for `docket-implement-next`. Keep the additions to a few clauses.

- [ ] **Step 2: Edit Step 2 (Claim).** In the sentence that sets `status: in-progress` + `branch:` + `updated:`, add `claimed_at:`:

> …if still `proposed`, set `status: in-progress` + `branch: feat/<slug>` + `updated: <UTC today>` + `claimed_at: <UTC ISO-8601 now>` (`date -u +%Y-%m-%dT%H:%M:%SZ`; add the field if the change predates it) — the claim lease `reclaim-claims` keys on…

- [ ] **Step 3: Edit the field-write rule + phase boundaries.** In the *field-write rule* enumeration (and Step 3 reconcile / Step 7 `implemented`), note that each phase-boundary metadata commit **re-stamps `claimed_at: <UTC ISO-8601 now>`** (a zero-cost heartbeat) — one added clause, not a new paragraph.

- [ ] **Step 4: Run the size budget + convention-extraction tests.** Run: `bash tests/test_skill_size_budgets.sh && bash tests/test_convention_extraction.sh`. Expected: green. If the budget reddens, tighten wording (do not raise the budget).

- [ ] **Step 5: Commit.**

```bash
git add skills/docket-implement-next/SKILL.md
git commit -m "docs(reclaim): implement-next stamps + refreshes the claimed_at lease"
```

---

## Task 9: `docket-convention` skill — manifest field, body section, lifecycle edge

**Files:**
- Modify: `skills/docket-convention/SKILL.md` (change manifest block; "Change body sections" list; "Lifecycle — seven states" diagram + rules)
- Test: `tests/test_skill_size_budgets.sh`, `tests/test_convention_extraction.sh`

**Interfaces:**
- Produces: the convention documents (a) the `claimed_at:` manifest field, (b) the `## Reclaim log` body section, (c) the sanctioned `in-progress → proposed` reverse edge (reclaim). This is the contract these code changes implement.

- [ ] **Step 1: Add the manifest field.** In the change-manifest YAML block, after `branch:`, add:

```yaml
claimed_at:               # UTC ISO-8601 claim lease (YYYY-MM-DDTHH:MM:SSZ); stamped at claim, refreshed at phase boundaries, cleared on leaving in-progress
```

- [ ] **Step 2: Add the body section.** In the "Change body sections" list, after the `## Reconcile log` entry, add one line:

> - `## Reclaim log` — dated entries appended by `reclaim-claims.sh` when an expired-lease, no-branch claim self-heals back to `proposed`.

- [ ] **Step 3: Add the reverse edge to the lifecycle diagram.** In the "Lifecycle — seven states" ASCII diagram, add the reclaim edge from `in-progress` back to `proposed` (a labeled arrow, e.g. `in-progress ──lease expired, no branch (reclaim)──▶ proposed`), and add one rules-prose sentence:

> **Reclaim edge (`in-progress → proposed`).** An `in-progress` change whose claim lease (`claimed_at:` + `reclaim.lease_ttl`) has expired AND that has no feature branch is flipped back to `proposed` by `reclaim-claims.sh` (opt-in via `reclaim.auto` or an explicit `docket.sh reclaim-claims`), clearing `branch:`/`claimed_at:` and resetting `reconciled: false` so a fresh reconcile runs on re-claim. The has-branch case is never auto-reclaimed (it may carry real work) — it stays flagged for a human.

- [ ] **Step 4: Run the guarding tests.** Run: `bash tests/test_skill_size_budgets.sh && bash tests/test_convention_extraction.sh`. Expected: green. Tighten wording if the budget reddens.

- [ ] **Step 5: Commit.**

```bash
git add skills/docket-convention/SKILL.md
git commit -m "docs(reclaim): document claimed_at, Reclaim log, and the in-progress→proposed edge"
```

---

## Task 10: Ship the knob end-to-end (sample config + README)

**Files:**
- Modify: `.docket.yml` (this repo's committed config = the sample)
- Modify: `config.yml.example` (global-layer sample)
- Modify: `README.md`
- Test: `tests/test_config_example.sh` (must stay green)

**Interfaces:**
- Produces: the `reclaim:` knob is documented everywhere a new knob must ship (config-knob-ship-end-to-end): commented in both sample configs, described in README, with the reclaim command + lifecycle edge surfaced.

- [ ] **Step 1: Add the commented block to `.docket.yml`** near the `finalize:` / `learnings:` blocks:

```yaml
# The reclaim subsystem (change 0089) — expired in-progress claims self-heal back to proposed.
# A crashed docket-implement-next leaves a change stuck at in-progress; a claim lease (claimed_at:,
# stamped at claim) + this TTL let a deterministic reclaim pass flip an EXPIRED, NO-BRANCH claim
# back to build-ready proposed. Detection is always on (docket-status flags + recommends);
# MUTATION is opt-in.
#   lease_ttl: generous hours, >= the 3-day stale-in-progress window (default 72).
#   auto:      false (default) => flag + recommend only. true => docket-status self-heals each pass.
# reclaim:
#   lease_ttl: 72
#   auto: false
```

- [ ] **Step 2: Add the same commented block to `config.yml.example`** (global-layer sample; keep it commented so a fresh copy resolves warning-free, matching the `learnings:` treatment).

- [ ] **Step 3: Document in `README.md`.** Find the config/knobs section (where `finalize` / `learnings` / `terminal_publish` are described) and add a `reclaim` entry: the lease model (`claimed_at` + `lease_ttl` hours, default 72), the `auto` opt-in (default off = warn-only), the `docket.sh reclaim-claims` command, and the new `in-progress → proposed` reclaim lifecycle edge (with the has-branch case left for a human). If README states the stuck-in-progress failure as unavoidable anywhere, relax that prose.

- [ ] **Step 4: Run the config-sample test.** Run: `bash tests/test_config_example.sh`. Expected: green (the reclaim block is commented, so the resolver still sees a warning-free file). If a doc/coverage test enumerates knobs, update it.

- [ ] **Step 5: Commit.**

```bash
git add .docket.yml config.yml.example README.md
git commit -m "docs(reclaim): ship the reclaim knob end-to-end (sample configs + README)"
```

---

## Task 11: Whole-suite gate + results notes

**Files:** none (verification task)

- [ ] **Step 1: Run the entire suite** (never only the enumerated tests — promoted rule). From the feature worktree root:

```bash
for t in tests/test_*.sh; do echo "== $t =="; bash "$t" || echo "FAILED: $t"; done
```

Expected: every test exits 0 (no `FAILED:` line, no `NOT OK`). Investigate any red — especially `test_docket_facade.sh` (sentinel), `test_script_contracts_coverage.sh` (every `scripts/<n>.sh` needs a `scripts/<n>.md` — Task 3 ships `reclaim-claims.md`), `test_skill_size_budgets.sh`, and `test_change_links_coverage.sh`.

- [ ] **Step 2: Confirm `test_script_contracts_coverage.sh` sees `reclaim-claims.md`.** If it enumerates scripts, the new script+contract pair must be covered. Run: `bash tests/test_script_contracts_coverage.sh`.

- [ ] **Step 3: Note for the results file (metadata-branch verification).** Record that reclaim operates on metadata-branch change files (invisible to the integration-branch suite — `metadata-branch-invisible-to-suite`), so its behavior is proven by the hermetic `tests/test_reclaim_claims.sh` seams, and that the `## Artifacts` block is intentionally NOT regenerated by reclaim (no link-bearing field mutated — supersedes spec §3-3). These go in the change's results file at close-out (Step 6.5 of docket-implement-next).

---

## Self-Review

**Spec coverage** (spec §6 build checklist ↔ tasks):
- `claimed_at:` added to manifest + stamped/refreshed by implement-next + cleared on close-out → Tasks 9, 8, 7. ✓
- Convention lifecycle reverse edge + `## Reclaim log` section → Task 9. ✓
- `reclaim:` block in docket-config + export → Task 2. ✓
- `scripts/reclaim-claims.sh` + `.md` + `docket.sh reclaim-claims` routing → Tasks 3, 4. ✓
- `stale-in-progress` upgraded + `docket-status` prints remedy + invokes under `reclaim.auto` → Tasks 5, 6. ✓
- `tests/test_reclaim_claims.sh` hermetic + regression tests (non-opted-in mutates nothing; expired-with-branch untouched) → Task 3 (CASE B/D/E) + Task 6 (auto-off no-mutation). ✓
- Ship end-to-end (sample `.docket.yml` + README) → Task 10. ✓

**Reconcile refinements folded in:** facade op `reclaim-claims` (not `reclaim`) → Task 4; `lease_ttl` integer hours → Tasks 2/global constraints. ✓

**Type/name consistency:** `iso_to_epoch` (Task 1) used identically in Tasks 3 + 5; `--lease-ttl-hours` flag name identical in `reclaim-claims.sh` (Task 3), `board-checks.sh` (Task 5), and the `docket-status.sh` forwarding (Task 6); `[reclaimable]` marker produced in Task 5, consumed in Task 6; `RECLAIM_LEASE_TTL`/`RECLAIM_AUTO` emitted in Task 2, read in Task 6. ✓

**Placeholder scan:** the `## Reclaim log` `printf` in Task 3 is explicitly flagged as a sketch to finalize during implementation (bind the `%s` args correctly) — not a silent placeholder.
