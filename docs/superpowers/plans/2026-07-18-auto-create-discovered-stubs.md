# Auto-create discovered stubs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an `auto_capture` config knob that lets docket's autonomous single-change skills mint `proposed` needs-brainstorm stubs for follow-up work they discover mid-run — with `discovered_from:` provenance — instead of asking a human who isn't there.

**Architecture:** Three layers, split on the ADR-0012 script-vs-model boundary. (1) `docket-config.sh` resolves a new global-able boolean `auto_capture` and exports `AUTO_CAPTURE`. (2) A new deterministic `scripts/mint-stub.sh` does the whole mechanical mint — dedup check, id allocation, stub write from the change template, `discovered_from:` population, and the CAS push — reached through the `docket.sh` facade. (3) Skill prose in `docket-convention` (one shared definition) plus one-line hooks in `docket-implement-next`, `docket-finalize-change`, and `docket-status` decides *what* is material and calls the helper. Default-off: with `auto_capture` unset, every path is byte-identical to today.

**Tech Stack:** Bash (POSIX-leaning, `set -uo pipefail`), git plumbing, markdown skill/contract prose. No new dependencies. Tests are hand-rolled bash assert scripts under `tests/`.

## Global Constraints

Copied verbatim from the spec and from constraints verified at reconcile. Every task's requirements implicitly include this section.

- **Config key:** `auto_capture`, boolean, default `false`. Exported as `AUTO_CAPTURE`.
- **Fence classification: global-able** — resolves repo-local (`.docket.local.yml`) > repo-committed (`.docket.yml`) > global (`~/.config/docket/config.yml`) > built-in `false`. It must NOT appear in the ADR-0019 coordination-key fence list.
- **Default-off is byte-identical to pre-change.** A repo that never sets the key mints zero stubs and keeps every existing report line unchanged (learnings: `opt-in-signal-not-file-presence` — gate on the explicit key, never on config-file presence).
- **Per-invocation cap: 3**, a hardcoded constant, not a config key. Overflow is **surfaced in the run report, never silently dropped**.
- **Dedup:** cheap case-insensitive match of the proposed slug/title against existing **active** change files only. On a match, skip the mint and report the skip.
- **Materiality bar (model-side judgment):** mint only for *actionable follow-up work that would be its own change / PR*. A build-loop lesson → the learnings harvest. In-scope drift for the current change → the reconcile log. A bare observation → prose only.
- **Mint sites:** `docket-implement-next` (reconcile + review passes) and the `docket-finalize-change` / `docket-status` harvest. **`docket-auto-groom` is NOT a mint site** and must not be wired — it would break its provable-termination invariant and create an `auto_groom` × `auto_capture` growth loop. Interactive skills (`docket-new-change`, `docket-groom-next`) are out of scope.
- **Metadata-worktree writes only.** The mint must never touch the running change's `status:`/`branch:`/`pr:` state, and never writes to a feature branch.
- **Ship end-to-end** (learnings: `config-knob-ship-end-to-end`): the knob lands in `config.yml.example`, the `docket-convention` `.docket.yml` schema block, README, and the authoritative fence table in `scripts/docket-config.md` — in this same change.
- **Every new top-level `scripts/<name>.sh` needs a co-located `scripts/<name>.md` contract** (`tests/test_script_contracts_coverage.sh`).
- **Every new facade op must be added to BOTH `WRAPPED_OPS` in `scripts/docket.sh` and the inventory table in `scripts/docket.md`** — `tests/test_docket_facade.sh` asserts the two sets are equal.
- **Skill size budgets** (`tests/test_skill_size_budgets.sh`): every `skills/**/*.md` is pinned at landed actuals + ~10%. Headroom at plan time — implement-next 127/140 L, 2641/2845 W; finalize-change 132/160 L, 2266/2699 W; status 107/118 L, 2175/2393 W; convention 294/317 L, 4769/5104 W. If prose pushes a file over, **raise that budget row in the same diff** (the test explicitly sanctions this; silent regrowth is what it forbids).
- **Shell rules from AGENTS.md** (always-in-context, non-negotiable): never `producer | grep -q`/`head` under `pipefail` — capture into a variable then `grep <<<"$var"`; anchor every frontmatter-field edit to the first `---…---` block, never a bare column-0 match; validate marker order/balance before rewriting a marker-delimited block.
- **CAS discipline** (learnings: `cas-re-read-fresh-origin`): after a non-fast-forward push rejection, re-derive state from **fresh origin** (`fetch` + `reset --hard <remote>/<branch>`), never by re-reading the working tree you just wrote. Distinguish a lost race (retry) from a real git failure (`die`).
- **Suite command:** `for f in tests/test_*.sh; do echo "== $f"; bash "$f" || echo "FAILED: $f"; done` — run the WHOLE suite at the build gate, never only the enumerated tests (AGENTS.md).

---

### Task 1: `auto_capture` config resolution + export

Adds the knob to the resolver, the authoritative fence table, and the emit-interface contract. Ends with a green `tests/test_docket_config.sh`.

**Files:**
- Modify: `scripts/docket-config.sh` (resolution after the `AUTO_GROOM` line ~195; emit after `emit AUTO_GROOM` ~397)
- Modify: `scripts/docket-config.md` (fence table ~106; emit list ~272; the two interface counts ~284)
- Test: `tests/test_docket_config.sh` (two count assertions at ~143 and ~402; new `auto_capture` block appended after the change-0089 `reclaim:` block ~827+)

**Interfaces:**
- Consumes: nothing from earlier tasks (this is the first).
- Produces: the exported variable **`AUTO_CAPTURE`** (value exactly `true` or `false`), emitted by `docket-config.sh --export` in both `shell` and `plain` formats, positioned **immediately after `AUTO_GROOM`**. Tasks 3 and 4 refer to it by that name; Task 2's script never reads it (the skill passes the decision down).

- [ ] **Step 1: Write the failing test — resolution, layering, and fail-closed validation**

Append to `tests/test_docket_config.sh`, after the change-0089 `reclaim:` block (which ends near line 925, before any final `exit $fail`). It reuses the file's existing `mkrepo` / `run` / `rung` / `assert` helpers exactly as the reclaim block does.

```bash
# --- Change 0091 — auto_capture (global-able boolean, default false) ---------------------------
# Mirrors auto_groom's four-layer resolution, but fails CLOSED on a non-boolean (the reclaim.auto /
# learnings.enabled / terminal_publish precedent): silently defaulting a typo to `false` would make
# an opted-in repo quietly stop capturing, which is invisible rather than loud.

# (AC-a) default
mkrepo "$tmp/ac-a"
out_ac="$(run "$tmp/ac-a" --export 2>/dev/null)"
assert "AUTO_CAPTURE defaults to false" 'echo "$out_ac" | grep -qxF "AUTO_CAPTURE=false"'

# (AC-b) repo-committed .docket.yml wins over the built-in
mkrepo "$tmp/ac-b"
printf 'auto_capture: true\n' > "$tmp/ac-b/.docket.yml"
git -C "$tmp/ac-b" add .docket.yml >/dev/null 2>&1
git -C "$tmp/ac-b" commit -qm "cfg" >/dev/null 2>&1
git -C "$tmp/ac-b" push -q origin HEAD:main >/dev/null 2>&1
out_ac_b="$(run "$tmp/ac-b" --export 2>/dev/null)"
assert "AUTO_CAPTURE reads repo .docket.yml" 'echo "$out_ac_b" | grep -qxF "AUTO_CAPTURE=true"'

# (AC-c) global layer is honored (NOT fenced) and emits no per-repo-only warning
mkrepo "$tmp/ac-c"
mkdir -p "$tmp/ac-c.xdg/docket"
printf 'auto_capture: true\n' > "$tmp/ac-c.xdg/docket/config.yml"
ac_c_out="$(rung "$tmp/ac-c.xdg" "$tmp/ac-c" --export 2>/dev/null)"
ac_c_err="$(rung "$tmp/ac-c.xdg" "$tmp/ac-c" --export 2>&1 >/dev/null)"
assert "auto_capture is global-able (not fenced)" 'echo "$ac_c_out" | grep -qxF "AUTO_CAPTURE=true"'
assert "no fence warning for auto_capture" '! printf "%s" "$ac_c_err" | grep -qi "auto_capture.*per-repo-only"'

# (AC-d) repo-local .docket.local.yml outranks repo-committed AND global
mkrepo "$tmp/ac-d"
mkdir -p "$tmp/ac-d.xdg/docket"
printf 'auto_capture: false\n' > "$tmp/ac-d.xdg/docket/config.yml"
printf 'auto_capture: false\n' > "$tmp/ac-d/.docket.yml"
git -C "$tmp/ac-d" add .docket.yml >/dev/null 2>&1
git -C "$tmp/ac-d" commit -qm "cfg" >/dev/null 2>&1
git -C "$tmp/ac-d" push -q origin HEAD:main >/dev/null 2>&1
printf 'auto_capture: true\n' > "$tmp/ac-d/.docket.local.yml"
ac_d_out="$(rung "$tmp/ac-d.xdg" "$tmp/ac-d" --export 2>/dev/null)"
assert "repo-local auto_capture outranks repo-committed and global" \
  'echo "$ac_d_out" | grep -qxF "AUTO_CAPTURE=true"'

# (AC-e) fail closed on garbage, and the diagnostic names the key
mkrepo "$tmp/ac-e"
printf 'auto_capture: maybe\n' > "$tmp/ac-e/.docket.yml"
git -C "$tmp/ac-e" add .docket.yml >/dev/null 2>&1
git -C "$tmp/ac-e" commit -qm "cfg" >/dev/null 2>&1
git -C "$tmp/ac-e" push -q origin HEAD:main >/dev/null 2>&1
assert "non-bool auto_capture aborts nonzero" '! run "$tmp/ac-e" --export >/dev/null 2>&1'
ac_e_err="$(run "$tmp/ac-e" --export 2>&1 >/dev/null)"
assert "unparseable auto_capture: names auto_capture" \
  'printf "%s" "$ac_e_err" | grep -qF "auto_capture"'

# (AC-f) emit ORDER is pinned: AUTO_CAPTURE immediately follows AUTO_GROOM (the contract in
# scripts/docket-config.md lists them adjacently; a reordering there is a silent contract break).
ac_f="$(run "$tmp/ac-a" --export 2>/dev/null | grep -n '^AUTO_' | cut -d: -f1 | tr '\n' ' ')"
ac_g_line="$(printf '%s' "$ac_f" | awk '{print $1}')"
ac_c_line="$(printf '%s' "$ac_f" | awk '{print $2}')"
assert "AUTO_CAPTURE is emitted directly after AUTO_GROOM" '[ "$ac_c_line" -eq "$(( ac_g_line + 1 ))" ]'
```

Also update the two existing interface-count assertions in the SAME step — adding one emitted line moves both:

```bash
# line ~143
assert "direct-pipe: 24 KEY=value lines emitted"       '[ "$n" -eq 24 ]'
# line ~402
assert "0050 E': still 24 KEY=value lines with global layer" '[ "$n50" -eq 24 ]'
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_docket_config.sh 2>&1 | grep -c "NOT OK"`
Expected: a non-zero count. The `AUTO_CAPTURE=…` assertions fail (the variable is never emitted) and both count assertions fail (23 emitted, 24 expected).

- [ ] **Step 3: Add the resolution + validation to `scripts/docket-config.sh`**

Insert immediately AFTER the existing `AUTO_GROOM=` line (currently line 195) and BEFORE the `# change 0064: coordination-key fenced` comment block:

```bash
# change 0091: auto_capture — gates autonomous mid-run capture of discovered follow-up work into
# proposed needs-brainstorm stubs. Global-able (ADR-0019): like auto_groom it gates a LOCAL-RUN
# behavior producing ordinary backlog commits, never coordination state, so per-machine divergence
# is the benign "machine A captures, machine B does not" — never a split backlog. Unlike auto_groom
# it fails CLOSED on a non-boolean (the reclaim.auto / learnings.enabled precedent): defaulting a
# typo to `false` would silently stop capture in a repo that opted in, an invisible failure.
AUTO_CAPTURE="$(lcl auto_capture)"; AUTO_CAPTURE="${AUTO_CAPTURE:-$(yaml_get "$CFG" auto_capture)}"; AUTO_CAPTURE="${AUTO_CAPTURE:-$(gbl auto_capture)}"; AUTO_CAPTURE="${AUTO_CAPTURE:-false}"
case "$AUTO_CAPTURE" in
  true|false) ;;
  *) die "unparseable config: auto_capture must be 'true' or 'false', got '$AUTO_CAPTURE'" ;;
esac
```

- [ ] **Step 4: Add the emit line**

In the emit block (currently ~line 397), insert directly after `emit AUTO_GROOM "$AUTO_GROOM"`:

```bash
  emit AUTO_CAPTURE "$AUTO_CAPTURE"
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `bash tests/test_docket_config.sh 2>&1 | grep -c "NOT OK"`
Expected: `0`.

- [ ] **Step 6: Prove the guard bites (mutation test — AGENTS.md "a guard is code")**

Temporarily change the resolution default to `AUTO_CAPTURE="${AUTO_CAPTURE:-true}"`, re-run `bash tests/test_docket_config.sh`, and confirm the `AUTO_CAPTURE defaults to false` assertion goes red. Then temporarily delete the `case … esac` validation and confirm `non-bool auto_capture aborts nonzero` goes red. **Revert both mutations.**

Expected: each mutation reddens at least one assertion; the file is byte-identical to Step 4 afterward (`git diff` shows only the intended additions).

- [ ] **Step 7: Update `scripts/docket-config.md` — fence table, emit list, and both counts**

(a) In the per-key table, insert a row directly after the `auto_groom` row:

```markdown
| `auto_capture` | `false` | yes | resolves repo-local > repo-committed > global; fails closed on a non-boolean (change 0091) |
```

(b) In the emit-interface list, insert `AUTO_CAPTURE` directly after `AUTO_GROOM`.

(c) Update the count sentence beneath that list from:

```markdown
23 lines in `shell` format; 24 in `plain` format, with `REPO_ROOT` inserted directly
```

to:

```markdown
24 lines in `shell` format; 25 in `plain` format, with `REPO_ROOT` inserted directly
```

(d) In the Stage 2b'/2c prose listing which keys resolve through the full layering — the sentence reading `finalize.test_command`, `auto_groom`, `board_surfaces`, and each `skills:` leaf. — add `auto_capture`:

```markdown
`finalize.test_command`, `auto_groom`, `auto_capture`, `board_surfaces`, and each `skills:` leaf.
```

- [ ] **Step 8: Verify no fence-list contamination**

Run: `grep -n "auto_capture" scripts/docket-config.sh scripts/docket-config.md`
Expected: `auto_capture` appears in the resolution block, the emit line, the table row, the emit list, and the layering sentence — and **never** inside the coordination-key fence enumeration (the list containing `metadata_branch`, `integration_branch`, `changes_dir`, `adrs_dir`, `results_dir`, `github_project`, `terminal_publish`). Confirm by reading the fence bullet in Stage 2c.

- [ ] **Step 9: Run the whole suite**

Run: `for f in tests/test_*.sh; do echo "== $f"; bash "$f" >/tmp/o 2>&1 || { echo "FAILED: $f"; grep "NOT OK" /tmp/o | head -5; }; done`
Expected: no `FAILED:` lines. In particular `tests/test_consuming_repo_scripts.sh` and `tests/test_docket_facade.sh` stay green (this task adds no op).

- [ ] **Step 10: Commit**

```bash
git add scripts/docket-config.sh scripts/docket-config.md tests/test_docket_config.sh
git commit -m "feat(0091): resolve + export auto_capture (global-able, fail-closed boolean)"
```

---

### Task 2: `mint-stub.sh` — the deterministic mint (dedup, allocate, write, CAS)

The whole mechanical half of the ADR-0012 split, plus its contract and facade wiring. Ends with a new green `tests/test_mint_stub.sh` driving a real temp git repo with a bare origin.

**Files:**
- Create: `scripts/mint-stub.sh`
- Create: `scripts/mint-stub.md`
- Modify: `scripts/docket.sh` (the `# Usage:` op list ~line 25; `WRAPPED_OPS` ~line 38)
- Modify: `scripts/docket.md` (inventory table, after the `reclaim-claims` row ~line 56)
- Test: `tests/test_mint_stub.sh`

**Interfaces:**
- Consumes: nothing from Task 1 at runtime — the script never reads `AUTO_CAPTURE`; the calling skill gates on it and only then invokes the helper. (Keeping the gate model-side means the script has exactly one job and stays trivially testable.)
- Produces: the facade op **`docket.sh mint-stub`** with this exact contract, which Task 3's skill prose invokes verbatim:

```
mint-stub.sh --changes-dir DIR --title TITLE --body-file FILE --discovered-from ID
             [--slug SLUG] [--minted N] [--cap N] [--remote R] [--template PATH]
```

  - `--changes-dir` — path to the metadata worktree's changes dir (e.g. `.docket/docs/changes`).
  - `--title` — the stub's title line.
  - `--body-file` — a file whose contents become the stub body verbatim; **must start with `## Why`**.
  - `--discovered-from` — the originating change id (populates `discovered_from: [ID]`).
  - `--slug` — optional; derived from `--title` when omitted (lowercase, non-alphanumerics → `-`, squeezed, trimmed, truncated to 60 chars).
  - `--minted` — how many stubs this invocation has already minted (default `0`); with `--cap` (default `3`) it enforces the per-invocation cap.
  - Exit codes: **0** minted · **3** duplicate, nothing written · **4** cap reached, nothing written · **1** error.
  - Stdout report lines (one, exactly): `minted <id> <slug>` · `skipped duplicate <slug> (matches #<id>)` · `skipped cap-reached (cap <n>, minted <n>)`.
  - Mock seams: `GIT="${GIT:-git}"`, `TODAY="${TODAY:-$(date -u +%Y-%m-%d)}"`.

- [ ] **Step 1: Write the failing test**

Create `tests/test_mint_stub.sh`. The `new_repo` helper is lifted from `tests/test_reclaim_claims.sh` (a real temp repo with a **bare** origin parked on `docket`, so the CAS push actually lands — a mock that omits the push would route every test through an untested branch; learnings: `green-suite-untested-branch`).

```bash
#!/usr/bin/env bash
# tests/test_mint_stub.sh — verifies change 0091: scripts/mint-stub.sh, the deterministic
# discovered-work stub mint. Hermetic: a temp repo with a local *bare* origin parked on the docket
# branch so the CAS push actually lands; TODAY is mocked so the created/updated stamps are stable.
# Run: bash tests/test_mint_stub.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$REPO/scripts/mint-stub.sh"
TEMPLATE="$REPO/skills/docket-new-change/change-template.md"
# shellcheck source=/dev/null
. "$REPO/scripts/lib/docket-frontmatter.sh"   # field / list_field / int_field for the assertions
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }
git_quiet(){ git "$@" >/dev/null 2>&1; }
FIXED_DAY=2026-07-18

new_repo(){
  local root origin work
  root="$(mktemp -d)"; origin="$root/origin.git"; work="$root/work"
  git_quiet init --bare "$origin"
  git_quiet clone "$origin" "$work"
  git -C "$work" config user.email t@t; git -C "$work" config user.name t
  git_quiet -C "$work" checkout --orphan docket
  git_quiet -C "$work" rm -rf . || true
  mkdir -p "$work/docs/changes/active" "$work/docs/changes/archive"
  echo baseline > "$work/docs/changes/.keep"
  git -C "$work" add -A; git_quiet -C "$work" commit -m "docket baseline"
  git_quiet -C "$work" push -u origin docket
  printf '%s\n' "$work"
}

# mkchange WORK DIR FILE ID SLUG TITLE — write a minimal change file into active/ or archive/.
mkchange(){
  local work="$1" dir="$2" file="$3" id="$4" slug="$5" title="$6"
  cat > "$work/docs/changes/$dir/$file" <<EOF
---
id: $id
slug: $slug
title: $title
status: proposed
---

## Why
seed
EOF
}

body(){ local f; f="$(mktemp)"; printf '## Why\n\n%s\n\n## What changes\n\n- thing\n' "$1" > "$f"; printf '%s' "$f"; }

run_mint(){ # run_mint WORK [extra args...]
  local work="$1"; shift
  TODAY="$FIXED_DAY" "$SCRIPT" --changes-dir "$work/docs/changes" --template "$TEMPLATE" "$@"
}

# --- (A) happy path: allocates max+1, writes the stub, pushes, reports ------------------------
W="$(new_repo)"
mkchange "$W" active  0007-alpha.md 7 alpha "Alpha"
mkchange "$W" archive 2026-07-01-0012-beta.md 12 beta "Beta"
git -C "$W" add -A; git_quiet -C "$W" commit -m seed; git_quiet -C "$W" push origin docket
B="$(body 'discovered while building #91')"
outA="$(run_mint "$W" --title "Cap the widget" --body-file "$B" --discovered-from 91 2>&1)"; rcA=$?
NEW="$W/docs/changes/active/0013-cap-the-widget.md"
assert "A: exit 0"                 '[ "$rcA" -eq 0 ]'
assert "A: reports minted 13"      '[ "$outA" = "minted 13 cap-the-widget" ]'
assert "A: file created at max+1"  '[ -f "$NEW" ]'
assert "A: id field is 13"         '[ "$(int_field "$NEW" id)" = "13" ]'
assert "A: slug field"             '[ "$(field "$NEW" slug)" = "cap-the-widget" ]'
assert "A: title field"            '[ "$(field "$NEW" title)" = "Cap the widget" ]'
assert "A: status proposed"        '[ "$(field "$NEW" status)" = "proposed" ]'
assert "A: discovered_from set"    '[ "$(list_field "$NEW" discovered_from)" = "91" ]'
assert "A: created stamped"        '[ "$(field "$NEW" created)" = "$FIXED_DAY" ]'
assert "A: updated stamped"        '[ "$(field "$NEW" updated)" = "$FIXED_DAY" ]'
assert "A: needs-brainstorm (no spec)" '[ -z "$(field "$NEW" spec)" ]'
assert "A: not trivial"            '[ "$(field "$NEW" trivial)" = "false" ]'
assert "A: auto_groomable left unset" '[ -z "$(field "$NEW" auto_groomable)" ]'
assert "A: body carried through"   'grep -qF "discovered while building #91" "$NEW"'
assert "A: Artifacts markers present" \
  'grep -qF "docket:artifacts:start" "$NEW" && grep -qF "docket:artifacts:end" "$NEW"'
assert "A: pushed to origin" \
  '[ "$(git -C "$W" rev-parse HEAD)" = "$(git -C "$W" rev-parse origin/docket)" ]'
assert "A: working tree clean after mint" '[ -z "$(git -C "$W" status --porcelain)" ]'
assert "A: commit touched ONLY the new change file" \
  '[ "$(git -C "$W" show --name-only --format= HEAD | grep -c .)" -eq 1 ]'

# --- (B) dedup against an ACTIVE slug (case-insensitive) --------------------------------------
W2="$(new_repo)"
mkchange "$W2" active 0004-cap-the-widget.md 4 cap-the-widget "Cap The Widget"
git -C "$W2" add -A; git_quiet -C "$W2" commit -m seed; git_quiet -C "$W2" push origin docket
B2="$(body dup)"
outB="$(run_mint "$W2" --title "CAP the WIDGET" --body-file "$B2" --discovered-from 91 2>&1)"; rcB=$?
assert "B: exit 3 on duplicate"    '[ "$rcB" -eq 3 ]'
assert "B: reports the match"      '[ "$outB" = "skipped duplicate cap-the-widget (matches #4)" ]'
assert "B: no new file"            '[ "$(ls "$W2/docs/changes/active" | grep -c .)" -eq 1 ]'
assert "B: no new commit"          '[ -z "$(git -C "$W2" status --porcelain)" ]'

# --- (B2) an ARCHIVED slug is NOT a duplicate (dedup is active-only, by spec §5) ---------------
W2b="$(new_repo)"
mkchange "$W2b" archive 2026-07-01-0004-cap-the-widget.md 4 cap-the-widget "Cap The Widget"
git -C "$W2b" add -A; git_quiet -C "$W2b" commit -m seed; git_quiet -C "$W2b" push origin docket
B2b="$(body notdup)"
outB2="$(run_mint "$W2b" --title "Cap the widget" --body-file "$B2b" --discovered-from 91 2>&1)"; rcB2=$?
assert "B2: archived slug does not block the mint" '[ "$rcB2" -eq 0 ]'
assert "B2: minted 5"                              '[ "$outB2" = "minted 5 cap-the-widget" ]'

# --- (C) cap ---------------------------------------------------------------------------------
W3="$(new_repo)"
B3="$(body capped)"
outC="$(run_mint "$W3" --title "Fourth thing" --body-file "$B3" --discovered-from 91 --minted 3 2>&1)"; rcC=$?
assert "C: exit 4 at cap"        '[ "$rcC" -eq 4 ]'
assert "C: reports cap-reached"  '[ "$outC" = "skipped cap-reached (cap 3, minted 3)" ]'
assert "C: nothing written"      '[ "$(ls "$W3/docs/changes/active" | grep -c .)" -eq 0 ]'
outC2="$(run_mint "$W3" --title "Third thing" --body-file "$B3" --discovered-from 91 --minted 2 2>&1)"; rcC2=$?
assert "C: under the cap still mints" '[ "$rcC2" -eq 0 ]'

# --- (D) CAS: a competing writer takes the id first; the retry RE-ALLOCATES from fresh origin ---
# The competing commit must DIVERGE the same contended path (an id), or the retry branch is never
# exercised (learnings: green-suite-untested-branch).
W4="$(new_repo)"
mkchange "$W4" active 0007-alpha.md 7 alpha "Alpha"
git -C "$W4" add -A; git_quiet -C "$W4" commit -m seed; git_quiet -C "$W4" push origin docket
OTHER="$(mktemp -d)/other"; git_quiet clone "$(git -C "$W4" remote get-url origin)" "$OTHER"
git -C "$OTHER" config user.email o@o; git -C "$OTHER" config user.name o
git_quiet -C "$OTHER" checkout docket
mkchange "$OTHER" active 0008-competitor.md 8 competitor "Competitor"
git -C "$OTHER" add -A; git_quiet -C "$OTHER" commit -m "competing mint"; git_quiet -C "$OTHER" push origin docket
B4="$(body race)"
outD="$(run_mint "$W4" --title "Raced thing" --body-file "$B4" --discovered-from 91 2>&1)"; rcD=$?
assert "D: exit 0 after the race"      '[ "$rcD" -eq 0 ]'
assert "D: re-allocated to 9, not 8"   '[ "$outD" = "minted 9 raced-thing" ]'
assert "D: file is 0009-raced-thing"   '[ -f "$W4/docs/changes/active/0009-raced-thing.md" ]'
assert "D: stale 0008 name not left behind" '[ ! -f "$W4/docs/changes/active/0008-raced-thing.md" ]'
assert "D: competitor survived"        '[ -f "$W4/docs/changes/active/0008-competitor.md" ]'
assert "D: converged with origin" \
  '[ "$(git -C "$W4" rev-parse HEAD)" = "$(git -C "$W4" rev-parse origin/docket)" ]'

# --- (E) argument validation ------------------------------------------------------------------
W5="$(new_repo)"
assert "E: missing --title fails"       '! run_mint "$W5" --body-file "$(body x)" --discovered-from 91 >/dev/null 2>&1'
assert "E: missing --body-file fails"   '! run_mint "$W5" --title T --discovered-from 91 >/dev/null 2>&1'
assert "E: missing --discovered-from fails" '! run_mint "$W5" --title T --body-file "$(body x)" >/dev/null 2>&1'
assert "E: non-numeric --discovered-from fails" \
  '! run_mint "$W5" --title T --body-file "$(body x)" --discovered-from nine >/dev/null 2>&1'
BAD="$(mktemp)"; printf 'no heading here\n' > "$BAD"
assert "E: body not starting with ## Why fails" \
  '! run_mint "$W5" --title T --body-file "$BAD" --discovered-from 91 >/dev/null 2>&1'
assert "E: a rejected run writes nothing" '[ "$(ls "$W5/docs/changes/active" | grep -c .)" -eq 0 ]'

exit $fail
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_mint_stub.sh 2>&1 | tail -5`
Expected: failure — the script does not exist yet (`No such file or directory`, every assert `NOT OK`).

- [ ] **Step 3: Write `scripts/mint-stub.sh`**

```bash
#!/usr/bin/env bash
# scripts/mint-stub.sh — deterministic discovered-work stub mint (change 0091). The MECHANICAL half
# of auto-capture: the calling skill judges WHAT is material (and gates on AUTO_CAPTURE); this script
# does the mint — cheap active-slug dedup, id allocation (max+1 across active/ + archive/), stub write
# from the change template with discovered_from: populated, and the compare-and-swap push. Git-only
# (no gh, no network beyond the remote). ADR-0012: a deterministic script, never model prose.
# ADR-0021: authors its own mechanical commit.
#
# Usage: mint-stub.sh --changes-dir DIR --title TITLE --body-file FILE --discovered-from ID
#                     [--slug SLUG] [--minted N] [--cap N] [--remote R] [--template PATH]
#   Mints exactly ONE stub per invocation. --minted is how many stubs THIS skill invocation has
#   already minted; at --cap (default 3) the mint is refused so the caller can surface the overflow.
#   Report (exactly one line, stdout):
#     minted <id> <slug>
#     skipped duplicate <slug> (matches #<id>)
#     skipped cap-reached (cap <n>, minted <n>)
#   Exit codes: 0 minted | 3 duplicate | 4 cap reached | 1 error.
#   Mock seams: GIT="${GIT:-git}"; TODAY="${TODAY:-$(date -u +%Y-%m-%d)}".
set -uo pipefail
GIT="${GIT:-git}"
TODAY="${TODAY:-$(date -u +%Y-%m-%d)}"
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHANGES_DIR=""; TITLE=""; BODY_FILE=""; FROM=""; SLUG=""; MINTED=0; CAP=3; REMOTE="origin"
TEMPLATE="$SELF_DIR/../skills/docket-new-change/change-template.md"
die(){ printf '%s\n' "mint-stub: $*" >&2; exit 1; }
while [ $# -gt 0 ]; do
  case "$1" in
    --changes-dir) CHANGES_DIR="$2"; shift ;;
    --title) TITLE="$2"; shift ;;
    --body-file) BODY_FILE="$2"; shift ;;
    --discovered-from) FROM="$2"; shift ;;
    --slug) SLUG="$2"; shift ;;
    --minted) MINTED="$2"; shift ;;
    --cap) CAP="$2"; shift ;;
    --remote) REMOTE="$2"; shift ;;
    --template) TEMPLATE="$2"; shift ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) die "unknown argument: $1" ;;
  esac
  shift
done
[ -n "$CHANGES_DIR" ] || die "missing --changes-dir"
[ -d "$CHANGES_DIR" ] || die "changes dir not found: $CHANGES_DIR"
[ -n "$TITLE" ]       || die "missing --title"
[ -n "$BODY_FILE" ]   || die "missing --body-file"
[ -f "$BODY_FILE" ]   || die "body file not found: $BODY_FILE"
[ -f "$TEMPLATE" ]    || die "change template not found: $TEMPLATE"
case "$FROM"   in ''|*[!0-9]*) die "missing/invalid --discovered-from (want a change id)" ;; esac
case "$MINTED" in ''|*[!0-9]*) die "invalid --minted" ;; esac
case "$CAP"    in ''|*[!0-9]*) die "invalid --cap" ;; esac
# The body is the model's prose; pin only its entry shape so a malformed stub can never land.
head1="$(head -n1 "$BODY_FILE")"
case "$head1" in "## Why"*) ;; *) die "body file must start with '## Why'" ;; esac

# shellcheck source=/dev/null
. "$SELF_DIR/lib/docket-frontmatter.sh"   # field / int_field

# slugify TEXT — lowercase, non-alphanumerics -> '-', squeeze, trim, cap at 60 chars.
slugify(){
  printf '%s' "$1" | tr '[:upper:]' '[:lower:]' \
    | sed -E 's/[^a-z0-9]+/-/g; s/^-+//; s/-+$//' | cut -c1-60
}
[ -n "$SLUG" ] || SLUG="$(slugify "$TITLE")"
[ -n "$SLUG" ] || die "could not derive a slug from --title"

# Cap first: it is the cheapest refusal and must not depend on repo state.
if [ "$MINTED" -ge "$CAP" ]; then
  printf 'skipped cap-reached (cap %s, minted %s)\n' "$CAP" "$MINTED"
  exit 4
fi

WT="$($GIT -C "$CHANGES_DIR" rev-parse --show-toplevel)" || die "not a git worktree: $CHANGES_DIR"
REL_ABS="$(cd "$CHANGES_DIR" && pwd -P)"; REL="${REL_ABS#"$WT"/}"
cur_branch="$($GIT -C "$WT" rev-parse --abbrev-ref HEAD)"

# dup_of SLUG — print the id of an ACTIVE change whose slug OR title slugifies to SLUG; empty if none.
# Active-only by spec §5: an archived near-name is history, not a live duplicate.
dup_of(){
  local want="$1" f fslug ftitle
  shopt -s nullglob
  for f in "$WT/$REL/active/"*.md; do
    fslug="$(field "$f" slug)"
    ftitle="$(field "$f" title)"
    if [ "$(slugify "$fslug")" = "$want" ] || [ "$(slugify "$ftitle")" = "$want" ]; then
      int_field "$f" id; shopt -u nullglob; return 0
    fi
  done
  shopt -u nullglob
  return 1
}

# next_id — max `id:` across active/ + archive/, plus one. 1 when the backlog is empty.
next_id(){
  local f v max=0
  shopt -s nullglob
  for f in "$WT/$REL/active/"*.md "$WT/$REL/archive/"*.md; do
    v="$(int_field "$f" id)"; [ -n "$v" ] || continue
    [ "$v" -gt "$max" ] && max="$v"
  done
  shopt -u nullglob
  printf '%s' "$(( max + 1 ))"
}

# write_stub ID — render the stub from the template and stage+commit it. Frontmatter scalars are
# rewritten ONLY inside the first ---…--- block (AGENTS.md frontmatter-anchor rule); the template's
# commented body scaffolding is replaced wholesale by the model's body, and the empty ## Artifacts
# marker block is emitted verbatim (render-change-links.sh remains its sole writer).
write_stub(){
  local id="$1" pad file tmp
  pad="$(printf '%04d' "$id")"
  file="$WT/$REL/active/$pad-$SLUG.md"
  tmp="$(mktemp)"
  # frontmatter: everything up to and including the SECOND '---'
  awk 'NR==1&&$0=="---"{print;next} /^---$/{print;exit} {print}' "$TEMPLATE" > "$tmp"
  sed -E -i.bak \
    -e "s|^(id:).*|\1 $id|" \
    -e "s|^(slug:).*|\1 $SLUG|" \
    -e "s|^(title:).*|\1 $TITLE|" \
    -e "s|^(created:).*|\1 $TODAY|" \
    -e "s|^(updated:).*|\1 $TODAY|" \
    -e "s|^(discovered_from:).*|\1 [$FROM]|" \
    "$tmp" && rm -f "$tmp.bak"
  {
    cat "$tmp"
    printf '\n## Artifacts\n\n'
    printf '<!-- docket:artifacts:start (generated — do not hand-edit) -->\n'
    printf '<!-- docket:artifacts:end -->\n\n'
    cat "$BODY_FILE"
  } > "$file"
  rm -f "$tmp"
  $GIT -C "$WT" add "$REL/active/$pad-$SLUG.md"                                  || die "git add failed for $id"
  $GIT -C "$WT" commit -q -m "docket($pad): auto-capture stub discovered from #$FROM" \
       -- "$REL/active/$pad-$SLUG.md"                                            || die "commit failed for $id"
  printf '%s' "$file"
}

dup_id="$(dup_of "$SLUG")" && {
  printf 'skipped duplicate %s (matches #%s)\n' "$SLUG" "$dup_id"
  exit 3
}

id="$(next_id)"
cur_file="$(write_stub "$id")"

# Bounded CAS retry. On EVERY non-fast-forward: fetch + reset --hard to the fresh remote tip, then
# RE-DERIVE both the dedup verdict and the next id from that origin state — never from the working
# tree we just wrote (which would read back our own stub and re-mint the same id forever). reset
# --hard is safe only because we push per mint: the local branch carries at most this one commit.
result=exhausted
for attempt in 1 2 3 4 5; do
  if $GIT -C "$WT" push "$REMOTE" "$cur_branch" >/dev/null 2>&1; then
    result=pushed; break
  fi
  $GIT -C "$WT" fetch "$REMOTE" >/dev/null 2>&1 \
    || die "fetch during CAS failed (attempt $attempt)"
  $GIT -C "$WT" reset --hard "$REMOTE/$cur_branch" >/dev/null 2>&1 \
    || die "reset --hard during CAS failed (attempt $attempt; $REMOTE/$cur_branch unreachable or missing)"
  # A concurrent writer may have minted this very stub while we raced.
  dup_id="$(dup_of "$SLUG")" && {
    printf 'skipped duplicate %s (matches #%s)\n' "$SLUG" "$dup_id"
    exit 3
  }
  id="$(next_id)"
  cur_file="$(write_stub "$id")"
done
case "$result" in
  pushed) printf 'minted %s %s\n' "$id" "$SLUG" ;;
  *) die "push did not converge after 5 attempts" ;;
esac
exit 0
```

- [ ] **Step 4: Make it executable and run the test**

```bash
chmod +x scripts/mint-stub.sh
bash tests/test_mint_stub.sh
```
Expected: every line `ok - …`, exit 0. If `sed -i.bak` misbehaves on this platform, replace with the portable `sed … > "$tmp2" && mv "$tmp2" "$tmp"` form used by `reclaim-claims.sh`'s `set_field` — **do not** leave a GNU-only `sed -i` in the tree (AGENTS.md shell-portability).

- [ ] **Step 5: Prove the CAS retry is not a vacuous branch (mutation test)**

Temporarily replace the retry body's `id="$(next_id)"` with a no-op (reuse the stale `id`), then run only fixture D:

Run: `bash tests/test_mint_stub.sh 2>&1 | grep "D:"`
Expected: `D: re-allocated to 9, not 8` goes `NOT OK` — proving the fixture actually exercises re-allocation rather than passing through a clean first push. **Revert the mutation** and re-run to green.

- [ ] **Step 6: Wire the facade — `scripts/docket.sh`**

(a) In the `# Usage:` block, insert after the `reclaim-claims` line:

```bash
#   mint-stub [args]          mint one discovered-work stub (auto-capture; CAS-correct)
```

(b) Append `mint-stub` to `WRAPPED_OPS`:

```bash
WRAPPED_OPS="docket-status board-refresh archive-change terminal-publish cleanup-feature-branch github-mirror sync-integration-branch render-change-links render-adr-index render-learnings-index adr-checks board-checks reclaim-claims runner-dispatch mint-stub"
```

- [ ] **Step 7: Wire the facade docs — `scripts/docket.md`**

Insert a row after the `reclaim-claims` row in the inventory table:

```markdown
| `mint-stub` | `mint-stub.sh` | mint one discovered-work stub into `active/` with `discovered_from:` provenance (auto-capture, change 0091) |
```

- [ ] **Step 8: Write the contract — `scripts/mint-stub.md`**

```markdown
# scripts/mint-stub.sh — contract

## Purpose

The deterministic, mechanical half of **auto-capture** (change 0091). When an autonomous skill
judges that it has discovered genuine follow-up work, this script performs the mint: a cheap
active-slug dedup check, id allocation, the stub write from the change template with
`discovered_from:` provenance, and a compare-and-swap push onto the metadata branch.

The split is the ADR-0012 script-vs-model boundary: the **model decides what is material** (and
gates on `AUTO_CAPTURE`); the **script does the mint**. Every mint site therefore shares one
CAS-correct implementation instead of N hand-rolled copies. Per ADR-0021 it authors its own
formulaic commit.

## Usage

```
mint-stub.sh --changes-dir DIR --title TITLE --body-file FILE --discovered-from ID
             [--slug SLUG] [--minted N] [--cap N] [--remote R] [--template PATH]
```

Reached from a skill through the facade: `docket.sh mint-stub …`.

| Flag | Required | Meaning |
|---|---|---|
| `--changes-dir` | yes | the metadata worktree's changes dir (e.g. `.docket/docs/changes`) |
| `--title` | yes | the stub's title |
| `--body-file` | yes | file whose contents become the stub body verbatim; **must start with `## Why`** |
| `--discovered-from` | yes | originating change id; populates `discovered_from: [ID]` |
| `--slug` | no | derived from `--title` when omitted |
| `--minted` | no | stubs already minted by THIS skill invocation (default `0`) |
| `--cap` | no | per-invocation cap (default `3`) |
| `--remote` | no | default `origin` |
| `--template` | no | default `../skills/docket-new-change/change-template.md` |

## Behavior

1. **Validate** every argument; a malformed body (no leading `## Why`) is rejected before any write.
2. **Cap check** — `--minted >= --cap` refuses immediately (exit 4), before touching repo state.
3. **Dedup** — case-insensitive slugified match of the proposed slug against every **active** change's
   `slug:` and `title:`. Archived changes are deliberately NOT scanned: archived work is history, not
   a live duplicate. On a match: exit 3, nothing written.
4. **Allocate** — id = max `id:` across `active/` + `archive/`, plus one.
5. **Write** — render the stub from the change template: frontmatter scalars rewritten inside the
   first `---…---` block only, an empty `## Artifacts` marker block (the block's sole writer remains
   `render-change-links.sh`), then the caller's body verbatim. The stub is an ordinary
   needs-brainstorm change: `status: proposed`, no `spec:`, `trivial: false`, `auto_groomable` left
   **unset** so it inherits the repo default — exactly like a scan-mode stub.
6. **Commit + CAS push** — stages and commits the ONE new change file, then pushes with a bounded
   5-attempt retry. On every non-fast-forward it fetches, `reset --hard`s to the fresh remote tip,
   and **re-derives both the dedup verdict and the next id from that origin state** — never from the
   working tree it just wrote. A concurrent writer that minted the same slug meanwhile turns the run
   into a duplicate skip. `reset --hard` is safe only because the script pushes per mint, so the local
   branch never carries more than this one commit.

Exactly one report line goes to stdout; the caller surfaces it.

## Exit codes

| Code | Meaning | Report line |
|---|---|---|
| 0 | stub minted and pushed | `minted <id> <slug>` |
| 3 | duplicate; nothing written | `skipped duplicate <slug> (matches #<id>)` |
| 4 | per-invocation cap reached; nothing written | `skipped cap-reached (cap <n>, minted <n>)` |
| 1 | usage/git error (diagnostic on stderr) | — |

## Invariants

- **Metadata-worktree writes only.** It touches exactly one new file under `active/` and never the
  originating change's `status:`/`branch:`/`pr:`, never `BOARD.md`, never a feature branch.
- **One stub per invocation.** Multi-stub capture is the caller looping, incrementing `--minted`.
- **Never merges or edits an existing change.** A near-duplicate is skipped, never amended.
- **No `gh`, no network beyond the git remote.** Offline-safe apart from the push.
- **The commit is formulaic** (`docket(<id>): auto-capture stub discovered from #<n>`) and touches a
  single path, keeping it trivially reviewable in history.

## Mock seams

`GIT` (default `git`), `TODAY` (default `date -u +%Y-%m-%d`). `tests/test_mint_stub.sh` drives a real
temp repo with a bare origin so the CAS push genuinely lands.
```

- [ ] **Step 9: Run the facade + contract-coverage tests**

Run: `bash tests/test_docket_facade.sh && bash tests/test_script_contracts_coverage.sh && bash tests/test_consuming_repo_scripts.sh`
Expected: all `ok`, exit 0. `test_docket_facade.sh` asserts `docket.sh op set == docket.md documented op set` — a mismatch here means Step 6 and Step 7 disagree.

- [ ] **Step 10: Run the whole suite**

Run: `for f in tests/test_*.sh; do echo "== $f"; bash "$f" >/tmp/o 2>&1 || { echo "FAILED: $f"; grep "NOT OK" /tmp/o | head -5; }; done`
Expected: no `FAILED:` lines.

- [ ] **Step 11: Commit**

```bash
git add scripts/mint-stub.sh scripts/mint-stub.md scripts/docket.sh scripts/docket.md tests/test_mint_stub.sh
git commit -m "feat(0091): mint-stub.sh — deterministic discovered-work stub mint + facade op"
```

---

### Task 3: Skill wiring — one shared definition, three thin hooks

The model half. The definition lands **once** in `docket-convention` (so the three call sites cannot drift apart — learnings: `consolidation-flattens-caller-variance`); each mint site gets a short hook that references it.

**Files:**
- Modify: `skills/docket-convention/SKILL.md` (`.docket.yml` schema block ~line 20; new *Auto-capture* definition placed immediately after the *Autonomous grooming* section)
- Modify: `skills/docket-implement-next/SKILL.md` (step 3 reconcile, step 6 review, the final-report enumeration)
- Modify: `skills/docket-finalize-change/SKILL.md` (step 2.5, alongside the harvest)
- Modify: `skills/docket-status/SKILL.md` (the `harvest <id> <path>` hook under *Judgment follow-ups*)
- Modify: `tests/test_skill_size_budgets.sh` (raise only the budget rows this task actually pushes over)

**Interfaces:**
- Consumes: `AUTO_CAPTURE` from Task 1 (read from the Step-0 config export, never re-parsed from YAML) and the `docket.sh mint-stub` op + exit codes from Task 2.
- Produces: the shared convention section **"Auto-capture (shared definition)"**, which the three skills reference by name and never restate.

- [ ] **Step 1: Add `auto_capture` to the convention's `.docket.yml` schema block**

In `skills/docket-convention/SKILL.md`, in the fenced `.docket.yml` sample, insert directly after the `auto_groom:` line:

```yaml
auto_capture: false          # autonomous mid-run capture of discovered follow-up work into stubs
```

- [ ] **Step 2: Add the shared *Auto-capture* definition**

Insert as a new section immediately AFTER the *Autonomous grooming (shared definition)* section and BEFORE *Learnings ledger*:

```markdown
### Auto-capture (shared definition)

`auto_capture` (default `false`, global-able) governs what an **autonomous** skill does with genuine
follow-up work it discovers mid-run. Disabled, the model reports it in prose and moves on. Enabled,
the model mints it as an ordinary `proposed` needs-brainstorm stub with `discovered_from:` set —
capture fidelity, **not** autonomy: every minted stub still waits at the human's groom gate.

**Mint sites** are the autonomous *single-change* skills: `docket-implement-next` (reconcile and
review discoveries) and the `docket-finalize-change` / `docket-status` harvest (close-out findings).
**`docket-auto-groom` is never a mint site** — a minted stub is itself autonomous-eligible, so
minting would break its provable-termination invariant and make `auto_groom` × `auto_capture` a
backlog-growth loop. Interactive skills already mint with a human present.

**Materiality bar** — mint only for *actionable follow-up work that would be its own change / PR*
("would a human file this as a `docket-new-change`?"). A lesson about how to build → the **learnings**
harvest. Drift inside the current change → the **reconcile log** or the current work. A bare
observation → the run report only.

**The mint itself is deterministic** (ADR-0012 — the model judges *what*, the script does the mint):
`"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh mint-stub --changes-dir .docket/<changes_dir>
--title <title> --body-file <file> --discovered-from <this change's id> --minted <n so far>` — one
stub per call, contract in `scripts/mint-stub.md`. It owns dedup, id allocation, the template write,
and the CAS push; **exit 3** = duplicate skipped, **exit 4** = per-invocation cap (3) reached. Every
skip and every capped overflow is **surfaced in the run report, never silently dropped**. Minting is
a metadata-worktree write only — it never touches the running change's own claim/branch/PR state.
```

- [ ] **Step 3: Hook `docket-implement-next`**

(a) In **Step 3 — Reconcile**, append to the paragraph ending `…set `reconciled: true`; commit and push on `metadata_branch` (in `docket`-mode, `origin/docket`).`:

```markdown
When `AUTO_CAPTURE` is `true` (Step-0 export), adjacent follow-up work this pass surfaces is minted per the convention's *Auto-capture* shared definition instead of only being noted.
```

(b) In **Step 6 — Review + ADRs**, append after the ADR dispatch sentence:

```markdown
Review findings that are distinct follow-up work — not this change's own fixes — are likewise minted per *Auto-capture* when enabled.
```

(c) In the **Terminal disposition** section's final paragraph, extend the enumeration sentence:

```markdown
The final report **enumerates** what happened: the change built (if any), each change **skipped with its reason** (needs-brainstorm / already `in-progress` / waiting on an unmerged `depends_on` / outside the id allowlist), any stubs **auto-captured** (plus every dedup skip and any cap overflow), and which disposition ended the run.
```

- [ ] **Step 4: Hook `docket-finalize-change`**

In step **2.5 Harvest learnings**, append one sentence to the end of that step:

```markdown
Separately from the harvest: when `AUTO_CAPTURE` is `true`, close-out findings that are distinct **follow-up work** rather than build-loop lessons are minted as stubs per the convention's *Auto-capture* shared definition (a finding can be a lesson, a stub, or neither — never both by default), committed and pushed independently of the harvest commit; every mint, dedup skip, and cap overflow is reported.
```

- [ ] **Step 5: Hook `docket-status`**

In the `harvest <id> <path>` bullet under *Judgment follow-ups*, append:

```markdown
The same step's auto-capture leg (convention: *Auto-capture*) applies here too, with the cap scoped **per swept change**; also best-effort — log and continue.
```

- [ ] **Step 6: Measure against the budgets**

Run:
```bash
for f in skills/docket-convention/SKILL.md skills/docket-implement-next/SKILL.md \
         skills/docket-finalize-change/SKILL.md skills/docket-status/SKILL.md; do
  printf '%-50s %4s L %5s W\n' "$f" "$(wc -l < "$f" | tr -d ' ')" "$(wc -w < "$f" | tr -d ' ')"
done
bash tests/test_skill_size_budgets.sh
```
Expected: the convention grows by roughly 20 lines / 300 words and will likely exceed its `317 / 5104` row; the three hooks are one sentence each and should stay inside their rows.

- [ ] **Step 7: Raise only the budget rows that actually went over**

For each file the test reddens, edit its row in the `BUDGETS` table of `tests/test_skill_size_budgets.sh` to the **new actual + ~10%**, matching the table's existing convention. Do not touch rows that are still green, and do not pre-emptively pad. Example shape (use the real measured numbers, not these):

```
skills/docket-convention/SKILL.md                          347 5600
```

Then re-run: `bash tests/test_skill_size_budgets.sh`
Expected: all `ok`, including `every skills/**/*.md has a budget row`.

- [ ] **Step 8: Verify auto-groom was NOT wired (the excluded-site guard)**

Run: `grep -rn "mint-stub\|Auto-capture\|auto_capture" skills/`
Expected: hits in `docket-convention`, `docket-implement-next`, `docket-finalize-change`, `docket-status` — and **zero** hits in `skills/docket-auto-groom/`. A hit there is a spec violation (§3), not a nice-to-have.

- [ ] **Step 9: Run the whole suite**

Run: `for f in tests/test_*.sh; do echo "== $f"; bash "$f" >/tmp/o 2>&1 || { echo "FAILED: $f"; grep "NOT OK" /tmp/o | head -5; }; done`
Expected: no `FAILED:` lines. Watch `tests/test_composition_wiring.sh`, `tests/test_skill_facade_wiring.sh`, and `tests/test_convention_extraction.sh` — all three read skill prose and are the likely tripwires for a wording change.

- [ ] **Step 10: Commit**

```bash
git add skills/docket-convention/SKILL.md skills/docket-implement-next/SKILL.md \
        skills/docket-finalize-change/SKILL.md skills/docket-status/SKILL.md \
        tests/test_skill_size_budgets.sh
git commit -m "feat(0091): wire auto-capture into the convention + the three autonomous mint sites"
```

---

### Task 4: Ship the knob end-to-end — sample config + README

The surfacing half of `config-knob-ship-end-to-end`: a knob that only works is not done. Nothing here is optional.

**Files:**
- Modify: `config.yml.example` (after the change-0089 `reclaim:` block, ~line 29)
- Modify: `README.md` (`.docket.yml` block ~line 200; global-config block ~line 264; `.docket.local.yml` block ~line 291; a new section after *Reclaiming stale claims* ~line 236)

**Interfaces:**
- Consumes: the key name, default, fence classification, cap, and mint-site list settled in Tasks 1–3. Every value documented here must match the resolver and the convention **exactly** — this is the surface a human reads before opting in.
- Produces: no code interface.

- [ ] **Step 1: Add the commented block to `config.yml.example`**

Insert after the `reclaim:` block (which ends at the commented `#   auto: false`), keeping the file's comment-then-commented-YAML house style:

```yaml
# Auto-capture (change 0091) — mid-run discovered work becomes a proposed stub instead of prose.
# When an AUTONOMOUS run surfaces genuine follow-up work (implement-next's reconcile/review, a
# close-out finding), there is no human to ask. With auto_capture: true the skill mints a
# needs-brainstorm stub with discovered_from: provenance; you still gate it at groom time, so this
# buys capture fidelity, not autonomy. Bounded: a cheap active-slug dedup check and a hardcoded cap
# of 3 stubs per invocation, with any overflow reported rather than dropped. docket-auto-groom is
# deliberately never a mint site. Global-able, so this is a legitimate place to set it.
# Leave commented to keep docket's shipped default (false).
#
# auto_capture: true
```

- [ ] **Step 2: Add the key to all three README config blocks**

In the `.docket.yml` block, after the `auto_groom:` line:

```yaml
auto_capture: false          # autonomous capture of discovered follow-up work into proposed stubs
```

In the **global config** (`~/.config/docket/config.yml`) sample block, after its `auto_groom: false` line:

```yaml
auto_capture: false
```

In the **`.docket.local.yml`** sample block, after its `auto_groom: false` line:

```yaml
auto_capture: false
```

- [ ] **Step 3: Add the README section**

Insert immediately after the *Reclaiming stale claims (`reclaim`)* section and before *Workflow roles — the `skills:` map*:

```markdown
### Capturing discovered work (`auto_capture`)

Agents constantly surface follow-up work mid-task: a reconcile pass notices an adjacent gap, a build
uncovers a latent bug, a close-out finding implies a next step. With a human in the room the model
asks. In an unattended run there is nobody to ask, so that work is mentioned in prose that scrolls
away — and lost.

`auto_capture: true` closes that gap. An autonomous skill that identifies genuine follow-up work
mints it as an ordinary `proposed` needs-brainstorm stub, with `discovered_from:` recording which
change surfaced it. Nothing is designed, built, or merged — you still gate every stub at groom time.
It buys **capture fidelity, not autonomy**.

- **Off by default.** With `auto_capture` unset or `false`, behavior is exactly as before.
- **Where it fires.** `docket-implement-next` (reconcile and review) and the
  `docket-finalize-change` / `docket-status` close-out harvest. `docket-auto-groom` deliberately
  never mints — a stub it created would be its own next input, so grooming could grow the queue it
  exists to drain.
- **What gets minted.** Only work that would be its own change/PR. Build-loop lessons go to
  [learnings](#learnings--the-loops-memory); drift inside the current change goes to its reconcile
  log.
- **Bounded.** A cheap dedup check against active changes, plus a cap of 3 stubs per invocation.
  Overflow is reported in the run output, never silently dropped.
- **Global-able.** Set it per-repo, in your global config, or in `.docket.local.yml`.

```yaml
auto_capture: true
```

Minted stubs appear on the board as ordinary `needs-brainstorm` work and flow into
`docket-groom-next`'s queue like anything else you filed by hand.
```

- [ ] **Step 4: Check the README table of contents**

Run: `sed -n '15,32p' README.md`
Expected: if the ToC enumerates the sections around *Reclaiming stale claims*, add a matching entry for *Capturing discovered work (`auto_capture`)* with an anchor of `#capturing-discovered-work-auto_capture`. If the ToC is coarser than that level, leave it alone.

- [ ] **Step 5: Verify every documented value matches the implementation**

Run:
```bash
grep -n "auto_capture" README.md config.yml.example scripts/docket-config.sh \
     scripts/docket-config.md skills/docket-convention/SKILL.md
```
Expected: the default is stated as `false` everywhere; the cap is stated as `3` everywhere it appears; the mint-site list is identical in README and the convention; no document claims `docket-auto-groom` mints. (Learnings: `verify-the-claim` — a doc asserting a fact about another artifact is not an oracle.)

- [ ] **Step 6: Run the whole suite**

Run: `for f in tests/test_*.sh; do echo "== $f"; bash "$f" >/tmp/o 2>&1 || { echo "FAILED: $f"; grep "NOT OK" /tmp/o | head -5; }; done`
Expected: no `FAILED:` lines. `tests/test_config_example.sh` runs `config.yml.example` through the real resolver — a commented block must stay invisible to the parser and produce no warnings.

- [ ] **Step 7: Verify the default-off path end-to-end (the non-adopter regression, spec "Verification notes")**

```bash
cd /tmp && rm -rf ac-probe && mkdir ac-probe && cd ac-probe
git init -q . && git commit -q --allow-empty -m init
"${DOCKET_SCRIPTS_DIR}"/docket.sh env 2>/dev/null | grep AUTO_CAPTURE
```
Expected: `AUTO_CAPTURE=false` in a repo with no `.docket.yml` at all — the opt-in is the key, never the file's presence. Then confirm nothing was written: `git -C /tmp/ac-probe status --porcelain` is empty. Clean up with `rm -rf /tmp/ac-probe`.

- [ ] **Step 8: Commit**

```bash
git add README.md config.yml.example
git commit -m "docs(0091): ship auto_capture end-to-end — sample config + README"
```

---

## Build-time verification (not automatable in the suite)

The hermetic suite sees only its fixtures and the integration-branch checkout — it cannot see the
`docket` metadata branch where stubs actually land (learnings: `metadata-branch-invisible-to-suite`).
Do these against the real tree at the build gate and record them in the results file's `## Findings`:

1. **A real mint.** In the live `.docket/` worktree, dry-run the mint against a **throwaway clone**
   of `origin/docket` (never the live branch): confirm the stub lands with the right id, that
   `discovered_from:` is populated, and that the board renders it as `needs-brainstorm`.
2. **The dedup path fires.** Point a mint at an existing active slug and watch exit 3 + the skip
   line — proving it is not a swallowed no-op.
3. **The global layer actually takes effect.** Set `auto_capture: true` in
   `~/.config/docket/config.yml` (temporarily) in a repo that also carries committed generated
   artifacts, and confirm `docket.sh env` reports `AUTO_CAPTURE=true` — a value can resolve
   correctly and still be shadowed (learnings: `config-layer-write-and-read-hazards`). Restore the
   file afterward.
4. **Nothing leaked onto the feature branch.** `git diff --stat origin/main..HEAD` must show only
   `scripts/`, `skills/`, `tests/`, `README.md`, `config.yml.example`, and the plan/results files —
   never a change file, `BOARD.md`, or an ADR.
