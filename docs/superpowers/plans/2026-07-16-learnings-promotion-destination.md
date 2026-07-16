# Learnings Promotion Destination — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn docket's single 490-line `LEARNINGS.md` into a findings directory plus a rendered, derived index, with a human-gated promotion valve that lets the ledger actually shrink, and a wholesale on/off switch.

**Architecture:** `<changes_dir>/learnings/` holds one curated finding file per lesson/family (bare `<slug>.md`), plus a generated `README.md` index rendered by a new sole-writer script `render-learnings-index.sh` (the exact analog of `render-adr-index.sh`). Readers load the small index and pull only relevant findings. The harvest (still close-out-only, per ADR-0005) creates or extends findings and marks must-fire rules `candidate`; a human graduates them to `AGENTS.md` and flips `promoted`, which drops them out of the paid surface and the cap's view. Two new config keys (`learnings.enabled`, `learnings.cap`) gate the whole subsystem.

**Tech Stack:** Bash (`set -uo pipefail`), `scripts/lib/docket-frontmatter.sh` (grep/sed/awk frontmatter reads — **no YAML loader**), markdown skill/convention prose, shell-based structural test suite (`tests/test_*.sh`).

## Global Constraints

Copied verbatim from the spec + the repo's learned conventions. **Every task's requirements implicitly include this section.**

- **No real YAML loader.** Parse frontmatter only via `scripts/lib/docket-frontmatter.sh` (`field` / `list_field` / `int_field` / `has_section`). Spec §4.1.
- **The renderer is pure**: reads finding files, emits the index to **STDOUT**, performs **no git writes**, no network, no `gh`. Callers redirect + commit. Spec §4.2.
- **The renderer is the sole writer** of `learnings/README.md` (ADR-0012 script-vs-model boundary). No skill ever hand-constructs or patches the index.
- **The renderer needs no enabled/disabled awareness** — *callers* gate on `learnings.enabled`, exactly as `render-board.sh` stays pure while `board-refresh.sh` gates it. Spec §4.2.
- **Determinism:** same finding files ⇒ byte-identical output.
- **ADR-0005 is preserved in substance:** harvest only at close-out, single writer, single moment, ledger never published to the integration branch. Spec §2.6, §3.
- **`promotion_state` default is `retained`** — a *positive* off-state; absence/emptiness are reserved for error (ADR-0032).
- **Fence classification:** both `learnings.enabled` and `learnings.cap` are **global-able** (ADR-0019) — they resolve through all four layers (`.docket.local.yml` > `.docket.yml` > global > built-in). Do **not** fence them. Spec §4.5.
- **`grep` for a `--flag`** must use `grep -E -e "<pat>"` or `grep -qF -- "<pat>"` (shell-portability family; a bare ERE leading with `--` is parsed as an option).
- **Never `producer | early-exiting-consumer`** (`grep -q`, `head`) under `pipefail` — capture into a variable first (pipefail family).
- **awk whitespace classes** must be `[^[:space:]]`, never `[^ ]` (tab-indented input; shell-portability family).
- **Every new grep sentinel must be mutation-tested** — delete/alter the clause it guards and watch it redden. A mutation that leaves an assert green is a defect, not a fact to explain away (guards-are-code family).
- **Key guards on syntactic SHAPE, never an enumerated list of spellings** (guards-are-code (g)).
- **Derive site lists from a whole-repo grep, never hand-list them** (enumerated-floor family).
- **Run the WHOLE suite** at the build gate, never only the enumerated tests (enumerated-floor (c)).
- **Metadata-branch files are invisible to the integration-branch suite** — verify them at build time and record in the results file (LEARNINGS #6). Never write a repo test asserting a `learnings/` finding file exists.
- **Each task must leave the whole suite GREEN** — a task that changes prose a sentinel guards updates that sentinel in the SAME task (LEARNINGS #45: the intermediate state is itself buildable and testable).

## File Structure

**Integration branch (ships via this PR):**

| File | Responsibility |
|---|---|
| `scripts/render-learnings-index.sh` | CREATE — pure, deterministic index renderer; sole writer of `learnings/README.md`. |
| `scripts/render-learnings-index.md` | CREATE — its contract (Purpose / Usage / Behavior / Exit codes / Invariants). |
| `scripts/docket-config.sh` | MODIFY — resolve the nested `learnings:` block; emit `LEARNINGS_ENABLED` + `LEARNINGS_CAP`. |
| `scripts/docket.sh` | MODIFY — add `render-learnings-index` to `WRAPPED_OPS`. |
| `scripts/docket-status.sh` | MODIFY — enable-gated index self-heal render + commit-only-if-changed. |
| `skills/docket-convention/SKILL.md` | MODIFY — rewrite *Learnings ledger*; directory layout; `.docket.yml` schema block. |
| `skills/docket-finalize-change/SKILL.md` | MODIFY — rewrite the *Harvest learnings* step (single source). |
| `skills/docket-status/SKILL.md` | MODIFY — index self-heal + two advisories + once-per-pass disabled note. |
| `skills/docket-implement-next/SKILL.md` | MODIFY — two-step read contract (plan time + review). |
| `skills/docket-groom-next/SKILL.md` | MODIFY — two-step read contract (before brainstorm). |
| `.docket.yml` | MODIFY — commented `learnings:` sample block with both keys + defaults. |
| `README.md` | MODIFY — new first-class "Learnings" feature section (orientation + pointer). |
| `AGENTS.md` | CREATE — the promotion destination (recommended neutral spelling; none exists today). |
| `tests/test_render_learnings_index.sh` | CREATE — renderer determinism/idempotency/rendering tests. |
| `tests/test_learnings_ledger.sh` | MODIFY — rewrite guards for the new contract. |
| `tests/test_docket_config.sh` | MODIFY — `LEARNINGS_ENABLED`/`LEARNINGS_CAP` resolution + layering. |

**Metadata branch (`docket`) — build-time data writes, NOT in this PR, verified in the results file:**

| Path | Responsibility |
|---|---|
| `docs/changes/learnings/<slug>.md` | The migrated finding files (one per family/standalone entry). |
| `docs/changes/learnings/README.md` | The rendered index. |
| `docs/changes/LEARNINGS.md` | Reduced to a pointer stub. |

## Known spec deviations (carry into the results file)

1. **`config.yml.example` gets NO `learnings:` block.** Spec §4.8/§6.13 says to add one. Against the real file: its header states *"Only the two harness/model keys are shown here — see README -> Configuration for every other key"*, and **ADR-0039** pins it as a **documented mirror of the `agents/docket-*.md` wrapper defaults**, nothing else. `finalize:`, `board_surfaces`, `terminal_publish`, and `skills:` are all likewise absent from it. Adding `learnings:` would contradict the file's own scoping sentence and ADR-0039's Decision. **Honor the spec's intent** (LEARNINGS #49 — a knob ships end-to-end) through the surfaces that actually carry knobs: the commented `.docket.yml` sample, the convention's schema block, and README → Configuration. Record in the results file; leave the re-scope to the human.
2. **Ledger is 490 lines / 33 top-level entries**, not the spec's 491 (trailing-newline off-by-one). Cosmetic; the cap-breach premise is unaffected.

---

### Task 1: The index renderer + contract

**Files:**
- Create: `scripts/render-learnings-index.sh`
- Create: `scripts/render-learnings-index.md`
- Test: `tests/test_render_learnings_index.sh`

**Interfaces:**
- Consumes: `scripts/lib/docket-frontmatter.sh` — `field FILE KEY`, `list_field FILE KEY` (both already exist; `field` returns the raw scalar **with quotes intact**).
- Produces: `render-learnings-index.sh --learnings-dir DIR` → index markdown on STDOUT; exit 0 on success, 2 on usage/missing-dir.

**Critical implementation note (do not skip):** `field()` does **not** strip surrounding quotes. `hook:` is REQUIRED to be quoted (it carries a colon-space — YAML-scalar family). So the renderer **must dequote** `hook` or the index ships literal `"` characters. A test pins this.

- [ ] **Step 1: Write the failing test**

Create `tests/test_render_learnings_index.sh`:

```bash
#!/usr/bin/env bash
# tests/test_render_learnings_index.sh — guards change 0067's index renderer.
# render-learnings-index.sh is the SOLE writer of <changes_dir>/learnings/README.md: pure
# (stdout only, no git), deterministic (same inputs => identical bytes), offline.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
R="$REPO/scripts/render-learnings-index.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

SB="$(mktemp -d)"; trap 'rm -rf "$SB"' EXIT
LD="$SB/learnings"; mkdir -p "$LD"

mkfinding(){ # mkfinding SLUG HOOK TOPICS CHANGES STATE PROMOTED_TO
  cat >"$LD/$1.md" <<EOF
---
slug: $1
hook: "$2"
topics: [$3]
changes: [$4]
created: 2026-06-17
updated: 2026-07-16
promotion_state: $5
promoted_to: ${6:-}
---

## Apply
The rule for $1.

## War story
- 2026-07-14 (#72, PR #79) — something happened.
EOF
}

mkfinding guards-are-code "A guard is code: mutation-test it, or it is decoration." "testing, sentinels" "14, 15" retained
mkfinding pipefail "Never producer | early-exiting-consumer under pipefail." "shell" "11" candidate
mkfinding yaml-scalar "Quote any scalar carrying a colon-space." "config" "5" promoted "AGENTS.md"

out="$("$R" --learnings-dir "$LD")"; rc=$?

# (a) contract basics
assert "exits 0 on a valid dir" '[ "$rc" = "0" ]'
assert "writes nothing into the learnings dir (stdout-only, no git)" '[ ! -e "$LD/README.md" ]'
assert "missing --learnings-dir exits 2" '"$R" >/dev/null 2>&1; [ "$?" = "2" ]'
assert "nonexistent dir exits 2" '"$R" --learnings-dir "$SB/nope" >/dev/null 2>&1; [ "$?" = "2" ]'

# (b) DEQUOTE — hook is required to be quoted; the index must not carry the quote bytes
assert "hook is dequoted in the index" '! printf "%s" "$out" | grep -qF "\"A guard is code"'
assert "hook text renders" 'printf "%s" "$out" | grep -qF "A guard is code: mutation-test it, or it is decoration."'

# (c) grouping by PRIMARY topic (first tag); remaining tags render inline
assert "primary topic group header present" 'printf "%s" "$out" | grep -qE "^## testing$"'
assert "secondary tag renders inline" 'printf "%s" "$out" | grep -qF "· also: sentinels"'
assert "a finding appears exactly once" '[ "$(printf "%s" "$out" | grep -cF "(guards-are-code.md)")" = "1" ]'

# (d) candidate marker
assert "candidate carries the needs-promotion marker" \
  'printf "%s" "$out" | grep -F "pipefail.md" | grep -qF "⟨needs promotion⟩"'
assert "retained carries no marker" \
  'printf "%s" "$out" | grep -F "guards-are-code.md" | grep -vqF "⟨needs promotion⟩"'

# (e) promoted findings leave the topic groups for the compressed appendix
assert "Promoted appendix present" 'printf "%s" "$out" | grep -qE "^## Promoted$"'
assert "promoted finding renders with its target" \
  'printf "%s" "$out" | grep -qF "[yaml-scalar](yaml-scalar.md) → AGENTS.md"'
assert "promoted finding is NOT in a topic group" \
  '[ "$(printf "%s" "$out" | grep -cF "yaml-scalar.md")" = "1" ]'
assert "promoted hook does not tax the hint surface" \
  '! printf "%s" "$out" | grep -qF "Quote any scalar carrying a colon-space."'

# (f) determinism / idempotency
out2="$("$R" --learnings-dir "$LD")"
assert "byte-identical across runs" '[ "$out" = "$out2" ]'

# (g) empty dir is a valid, non-crashing render
ED="$SB/empty"; mkdir -p "$ED"
eout="$("$R" --learnings-dir "$ED")"; erc=$?
assert "empty dir exits 0" '[ "$erc" = "0" ]'
assert "empty dir still renders the header" 'printf "%s" "$eout" | grep -qF "# Learnings"'

# (h) README.md in the dir is excluded from the corpus
printf '# Learnings\n' >"$LD/README.md"
out3="$("$R" --learnings-dir "$LD")"
assert "README.md is not treated as a finding" '[ "$out3" = "$out" ]'
rm -f "$LD/README.md"

# (i) corpus-size assert — prove the renderer SAW the findings (a parser that reads
#     nothing passes everything; guards-are-code family)
assert "index counts all 3 findings" \
  '[ "$(printf "%s" "$out" | grep -cE "^- \[")" = "3" ]'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
```

- [ ] **Step 2: Run test to verify it fails**

Run: `bash tests/test_render_learnings_index.sh`
Expected: FAIL — every assert errors because `scripts/render-learnings-index.sh` does not exist.

- [ ] **Step 3: Write the renderer**

Create `scripts/render-learnings-index.sh`:

```bash
#!/usr/bin/env bash
# scripts/render-learnings-index.sh — deterministic, idempotent renderer for the learnings index
# (<changes_dir>/learnings/README.md), change 0067. The exact analog of render-adr-index.sh (0030)
# and render-board.sh (0022): reads the finding files and emits the index to STDOUT byte-for-byte.
# No git writes (the caller redirects + commits), offline (no gh, no git, no network). Same finding
# files => identical bytes. Reuses lib/docket-frontmatter.sh.
#
# PURE BY DESIGN: this script has no learnings.enabled awareness. The CALLERS gate on it — exactly
# as render-board.sh stays pure while board-refresh.sh/docket-status.sh own the write decision.
#
# Usage: render-learnings-index.sh --learnings-dir DIR
set -uo pipefail

LEARNINGS_DIR=""
while [ $# -gt 0 ]; do
  case "$1" in
    --learnings-dir) LEARNINGS_DIR="$2"; shift ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) printf 'render-learnings-index: unknown argument: %s\n' "$1" >&2; exit 2 ;;
  esac
  shift
done
[ -n "$LEARNINGS_DIR" ] || { printf 'render-learnings-index: missing --learnings-dir\n' >&2; exit 2; }
[ -d "$LEARNINGS_DIR" ] || { printf 'render-learnings-index: learnings dir not found: %s\n' "$LEARNINGS_DIR" >&2; exit 2; }

# shellcheck source=/dev/null
source "$(dirname "${BASH_SOURCE[0]}")/lib/docket-frontmatter.sh"

# `field` returns the RAW scalar — quotes intact. `hook` is REQUIRED to be quoted (it carries a
# colon-space; YAML-scalar family), so it must be dequoted here or the index ships quote bytes.
dequote(){ local v="$1"; v="${v#\"}"; v="${v%\"}"; v="${v#\'}"; v="${v%\'}"; printf '%s' "$v"; }

declare -A F_HOOK F_TOPICS F_STATE F_TO
SLUGS=""
mapfile -t FILES < <(find "$LEARNINGS_DIR" -maxdepth 1 -name '*.md' ! -name 'README.md' 2>/dev/null | sort)
for f in "${FILES[@]}"; do
  slug="$(field "$f" slug)"; [ -n "$slug" ] || continue
  F_HOOK["$slug"]="$(dequote "$(field "$f" hook)")"
  F_TOPICS["$slug"]="$(list_field "$f" topics)"
  state="$(field "$f" promotion_state)"
  # Positive off-state (ADR-0032): an unset/unknown state is NOT silently "retained" for the
  # purposes of the hint surface — it renders as retained, but only `promoted` leaves the groups
  # and only `candidate` earns the marker, so an empty value degrades to the safe, visible tier.
  F_STATE["$slug"]="${state:-retained}"
  F_TO["$slug"]="$(field "$f" promoted_to)"
  SLUGS+="$slug"$'\n'
done

primary_topic(){ # primary_topic SLUG -> first tag, or "uncategorized"
  local t; t="$(printf '%s' "${F_TOPICS[$1]}" | awk '{print $1}')"
  printf '%s' "${t:-uncategorized}"
}
rest_topics(){ # rest_topics SLUG -> "b, c" (empty when only one tag)
  printf '%s' "${F_TOPICS[$1]}" | awk '{ for(i=2;i<=NF;i++){ printf "%s%s", (i>2 ? ", " : ""), $i } }'
}

# --- partition: promoted findings leave the paid surface for the appendix -------------------
ACTIVE=""; PROMOTED=""
while IFS= read -r s; do
  [ -n "$s" ] || continue
  if [ "${F_STATE[$s]}" = "promoted" ]; then PROMOTED+="$s"$'\n'; else ACTIVE+="$s"$'\n'; fi
done <<<"$SLUGS"

# --- topic buckets (derived, sorted — no hand-listed topic set) -----------------------------
TOPICS_SEEN=""
while IFS= read -r s; do
  [ -n "$s" ] || continue
  TOPICS_SEEN+="$(primary_topic "$s")"$'\n'
done <<<"$ACTIVE"
TOPICS_SORTED="$(printf '%s' "$TOPICS_SEEN" | sed '/^$/d' | sort -u)"

row(){ # row SLUG
  local s="$1" line rest marker=""
  [ "${F_STATE[$s]}" = "candidate" ] && marker=" ⟨needs promotion⟩"
  line="- [$s]($s.md) — ${F_HOOK[$s]}"
  rest="$(rest_topics "$s")"
  [ -n "$rest" ] && line+=" · also: $rest"
  printf '%s%s\n' "$line" "$marker"
}

printf '# Learnings — the build loop'"'"'s memory\n\n'
printf 'One curated finding per file; this index is the hint surface. Load it, then read only the findings that bear on the change at hand. Generated by `render-learnings-index.sh` — do not hand-edit. Contract: docket-convention, "Learnings ledger".\n'

if [ -n "$TOPICS_SORTED" ]; then
  while IFS= read -r topic; do
    [ -n "$topic" ] || continue
    printf '\n## %s\n\n' "$topic"
    while IFS= read -r s; do
      [ -n "$s" ] || continue
      [ "$(primary_topic "$s")" = "$topic" ] && row "$s"
    done <<<"$(printf '%s' "$ACTIVE" | sed '/^$/d' | sort)"
  done <<<"$TOPICS_SORTED"
fi

PROMOTED_SORTED="$(printf '%s' "$PROMOTED" | sed '/^$/d' | sort)"
if [ -n "$PROMOTED_SORTED" ]; then
  printf '\n## Promoted\n\n'
  printf 'Graduated to an always-in-context agent-instructions file. Kept as the rule'"'"'s receipt, the harvest'"'"'s dedup memory, and a one-line-reversible demotion path — they no longer count against the cap.\n\n'
  while IFS= read -r s; do
    [ -n "$s" ] || continue
    printf -- '- [%s](%s.md) → %s\n' "$s" "$s" "${F_TO[$s]}"
  done <<<"$PROMOTED_SORTED"
fi
```

- [ ] **Step 4: Run test to verify it passes**

Run: `bash tests/test_render_learnings_index.sh`
Expected: `PASS` — every assert `ok - …`.

- [ ] **Step 5: Mutation-test the new sentinels (guards-are-code — mandatory)**

Prove each assert can redden. Run each mutation, confirm `NOT OK`, then revert:
1. Delete the `dequote` call on `F_HOOK` → the "hook is dequoted" assert must redden.
2. Change `[ "${F_STATE[$s]}" = "promoted" ]` to `= "nope"` → the Promoted-appendix asserts must redden.
3. Remove the `! -name 'README.md'` filter → assert (h) must redden.
4. Drop the `sort` from `FILES`/`TOPICS_SORTED` on a multi-finding dir → determinism assert must still pass (`find | sort` is the determinism source); if it does NOT redden when you inject `shuf`, the determinism assert is vacuous — fix it.

Record the four outcomes; a mutation that stays green is a defect to fix, never to explain away.

- [ ] **Step 6: Write the contract**

Create `scripts/render-learnings-index.md`, following `scripts/render-adr-index.md`'s exact section structure (read it first — Purpose / Usage / Behavior / Exit codes / Invariants). It must state:
- **Purpose:** deterministic, offline renderer for `<changes_dir>/learnings/README.md`; the sole writer of that file (ADR-0012); a member of the derived-view script family alongside `render-board.sh`, `render-adr-index.sh`, `render-change-links.sh`.
- **Usage:** `render-learnings-index.sh --learnings-dir DIR`; reached from a consuming repo as `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh render-learnings-index --learnings-dir DIR`.
- **Behavior:** reads every `*.md` except `README.md` via `lib/docket-frontmatter.sh`; groups active findings under their **primary topic** (first `topics:` tag), rendering remaining tags inline as `· also: …`; marks `candidate` findings `⟨needs promotion⟩`; moves `promoted` findings out of the topic groups into a compressed trailing `## Promoted` group (`- [slug](slug.md) → <promoted_to>`). Dequotes `hook`. Emits to STDOUT only.
- **Exit codes:** `0` success (including an empty dir); `2` usage error / missing dir.
- **Invariants:** no git writes, no network; same finding files ⇒ byte-identical bytes; a finding appears exactly once; **no `learnings.enabled` awareness — the callers gate** (the `render-board.sh` precedent).

- [ ] **Step 7: Verify the contract-coverage test passes**

Run: `bash tests/test_script_contracts_coverage.sh`
Expected: `PASS` — every `scripts/<name>.sh` has a co-located `scripts/<name>.md`. (This test derives its corpus; the new contract satisfies it.)

- [ ] **Step 8: Commit**

```bash
git add scripts/render-learnings-index.sh scripts/render-learnings-index.md tests/test_render_learnings_index.sh
git commit -m "feat(0067): add render-learnings-index.sh — the derived learnings index renderer"
```

---

### Task 2: Config — the `learnings:` block

**Files:**
- Modify: `scripts/docket-config.sh` (resolution near the `skills:` block, ~line 242; emit block ~line 329)
- Test: `tests/test_docket_config.sh`

**Interfaces:**
- Consumes: `yaml_block_body FILE KEY` and `yaml_get FILE KEY` (both exist in `docket-config.sh`).
- Produces: exported `LEARNINGS_ENABLED` (`true`|`false`) and `LEARNINGS_CAP` (non-negative integer) in `docket-config.sh --export` output, consumed by skills as literals.

**Critical design decision (do not skip):** mirror `finalize:`'s **shape** (a nested block) but `skills:`' **parsing** (`yaml_block_body`). `finalize.gate` is read by bare leaf-key (`yaml_get "$CFG" gate`), which works only because `gate`/`test_command` are unusual words. `enabled` and `cap` are generic: a bare `yaml_get "$CFG" enabled` would match an `enabled:` leaf under **any** future block (or a top-level one). Read each leaf **within** the `learnings:` block — the reason the `skills:` block reader exists (see its comment: "never as a bare top-level key, which a future top-level `build:`/`review:` could otherwise shadow"). This is ADR-worthy; flag it for Task 11's ADR.

- [ ] **Step 1: Write the failing tests**

Append to `tests/test_docket_config.sh` (match the file's existing fixture/assert idiom — read it first; it builds sandbox repos and `eval`s the export). Add a section:

```bash
# --- change 0067: the learnings: block ---------------------------------------
# NOTE (guards-are-code (e)): clear the asserted vars BEFORE each eval — an aborting
# run emits NOTHING, and eval "" would silently leave the previous case's value in place.

# (a) defaults when no layer sets the block
unset LEARNINGS_ENABLED LEARNINGS_CAP
mkrepo_with_cfg ""            # existing helper: seeds .docket.yml with the given body
eval "$(run_export)"
assert "learnings.enabled defaults to true" '[ "$LEARNINGS_ENABLED" = "true" ]'
assert "learnings.cap defaults to 300" '[ "$LEARNINGS_CAP" = "300" ]'

# (b) repo-committed block is honored
unset LEARNINGS_ENABLED LEARNINGS_CAP
mkrepo_with_cfg "learnings:
  enabled: false
  cap: 120
"
eval "$(run_export)"
assert "repo learnings.enabled honored" '[ "$LEARNINGS_ENABLED" = "false" ]'
assert "repo learnings.cap honored" '[ "$LEARNINGS_CAP" = "120" ]'

# (c) BOTH keys are global-able (ADR-0019 — NOT fenced)
unset LEARNINGS_ENABLED LEARNINGS_CAP
mkrepo_with_cfg ""
mkglobal_cfg "learnings:
  enabled: false
  cap: 42
"
eval "$(run_export)"
assert "learnings.enabled is global-able (not fenced)" '[ "$LEARNINGS_ENABLED" = "false" ]'
assert "learnings.cap is global-able (not fenced)" '[ "$LEARNINGS_CAP" = "42" ]'
assert "no fence warning for learnings keys" '! printf "%s" "$err" | grep -qi "learnings.*per-repo-only"'

# (d) repo-local layer wins over repo-committed
unset LEARNINGS_ENABLED LEARNINGS_CAP
mkrepo_with_cfg "learnings:
  cap: 120
"
mklocal_cfg "learnings:
  cap: 7
"
eval "$(run_export)"
assert "local layer beats repo-committed for cap" '[ "$LEARNINGS_CAP" = "7" ]'

# (e) SHADOW GUARD — a bare `enabled:`/`cap:` OUTSIDE the learnings: block must not leak in.
#     This is the whole reason the block is read via yaml_block_body.
unset LEARNINGS_ENABLED LEARNINGS_CAP
mkrepo_with_cfg "some_future_block:
  enabled: false
  cap: 9
"
eval "$(run_export)"
assert "a foreign block's enabled: does not shadow learnings.enabled" '[ "$LEARNINGS_ENABLED" = "true" ]'
assert "a foreign block's cap: does not shadow learnings.cap" '[ "$LEARNINGS_CAP" = "300" ]'

# (f) fail closed on garbage (the terminal_publish precedent)
unset LEARNINGS_ENABLED LEARNINGS_CAP
mkrepo_with_cfg "learnings:
  enabled: yes
"
out="$(run_export 2>&1)"; rc=$?
assert "unparseable learnings.enabled dies loudly" '[ "$rc" != "0" ] && printf "%s" "$out" | grep -qF "learnings.enabled"'

unset LEARNINGS_ENABLED LEARNINGS_CAP
mkrepo_with_cfg "learnings:
  cap: lots
"
out="$(run_export 2>&1)"; rc=$?
assert "non-integer learnings.cap dies loudly" '[ "$rc" != "0" ] && printf "%s" "$out" | grep -qF "learnings.cap"'
```

Adapt the helper names (`mkrepo_with_cfg` / `mkglobal_cfg` / `mklocal_cfg` / `run_export`) to whatever `tests/test_docket_config.sh` actually defines — **read the file and reuse its real idiom**; do not invent helpers.

- [ ] **Step 2: Run test to verify it fails**

Run: `bash tests/test_docket_config.sh`
Expected: FAIL — `LEARNINGS_ENABLED`/`LEARNINGS_CAP` are unset/empty.

- [ ] **Step 3: Implement resolution**

In `scripts/docket-config.sh`, immediately after the `skills:` resolution block (after the `SKILL_FINISH=` line and its unknown-role-key warn loop), add:

```bash
# --- learnings: the findings ledger subsystem (change 0067) --------------------
# Nested block, mirroring finalize:'s SHAPE but the skills: block's PARSING. Each leaf is read
# WITHIN the block via yaml_block_body — never as a bare top-level key. finalize.gate gets away
# with a bare leaf read because `gate`/`test_command` are unusual words; `enabled` and `cap` are
# generic, so a bare read would let ANY block's (or a future top-level) `enabled:` shadow this one.
# Per-key precedence: repo-local > repo-committed > global > built-in.
# ADR-0019 fence: BOTH keys are global-able. A machine-local disable only OMITS an enrichment
# write — it never writes conflicting state, so there is no "which ledger is authoritative"
# question, and the index self-heals on any enabled render.
LEARN_BLK="$(mktemp)";  yaml_block_body "$CFG"  learnings >"$LEARN_BLK"
GLEARN_BLK="$(mktemp)"; yaml_block_body "$GCFG" learnings >"$GLEARN_BLK"
LLEARN_BLK="$(mktemp)"; yaml_block_body "$LCFG" learnings >"$LLEARN_BLK"
learn_key(){  # learn_key <leaf> <default> -> resolved value on stdout
  local v; v="$(yaml_get "$LLEARN_BLK" "$1")"
  [ -n "$v" ] || v="$(yaml_get "$LEARN_BLK" "$1")"
  [ -n "$v" ] || v="$(yaml_get "$GLEARN_BLK" "$1")"
  printf '%s' "${v:-$2}"
}
LEARNINGS_ENABLED="$(learn_key enabled true)"
LEARNINGS_CAP="$(learn_key cap 300)"
# Fail closed on garbage (the terminal_publish precedent): silently defaulting a typo would
# either tax every read or silently disable the subsystem — both against intent. `yes`/`no` are
# rejected deliberately (YAML-scalar family: they are boolean keywords under a real loader but
# arrive here as literal strings).
case "$LEARNINGS_ENABLED" in
  true|false) ;;
  *) die "unparseable config: learnings.enabled must be 'true' or 'false', got '$LEARNINGS_ENABLED'" ;;
esac
case "$LEARNINGS_CAP" in
  ''|*[!0-9]*) die "unparseable config: learnings.cap must be a non-negative integer, got '$LEARNINGS_CAP'" ;;
esac
```

Then add to the emit block (next to `emit FINALIZE_GATE`):

```bash
  emit LEARNINGS_ENABLED "$LEARNINGS_ENABLED"
  emit LEARNINGS_CAP "$LEARNINGS_CAP"
```

Add both temp files to the existing `trap … EXIT` cleanup list so no `mktemp` leaks (match how `SKILLS_BLK` is cleaned up — read that code and follow it exactly).

- [ ] **Step 4: Run test to verify it passes**

Run: `bash tests/test_docket_config.sh`
Expected: `PASS`.

- [ ] **Step 5: Mutation-test the shadow guard (the load-bearing one)**

Swap `learn_key`'s block-scoped reads for a bare `yaml_get "$CFG" enabled` and re-run. Assert (e) MUST redden. If it stays green, the guard is vacuous — fix it before proceeding. Revert the mutation.

- [ ] **Step 6: Verify the live export**

Run: `bash scripts/docket-config.sh --export --format plain | grep -E -e '^LEARNINGS_'`
Expected exactly:
```
LEARNINGS_ENABLED=true
LEARNINGS_CAP=300
```
(This repo sets no `learnings:` block, so both are built-in defaults.)

- [ ] **Step 7: Commit**

```bash
git add scripts/docket-config.sh tests/test_docket_config.sh
git commit -m "feat(0067): resolve the learnings: config block (enabled + cap), block-scoped"
```

---

### Task 3: Facade wiring

**Files:**
- Modify: `scripts/docket.sh:36` (`WRAPPED_OPS`)
- Test: `tests/test_docket_facade.sh`

**Interfaces:**
- Consumes: Task 1's `scripts/render-learnings-index.sh`.
- Produces: `docket.sh render-learnings-index --learnings-dir DIR` dispatches to the script.

- [ ] **Step 1: Write the failing test**

Read `tests/test_docket_facade.sh` and follow its existing idiom. Add:

```bash
assert "facade dispatches render-learnings-index" \
  'printf "%s" "$("$REPO/scripts/docket.sh" render-learnings-index --learnings-dir "$LD" 2>/dev/null)" | grep -qF "# Learnings"'
assert "render-learnings-index is listed in the rejection help text" \
  '"$REPO/scripts/docket.sh" bogus-op 2>&1 | grep -qF "render-learnings-index"'
```

Seed `$LD` as a temp learnings dir with one finding file (reuse Task 1's `mkfinding` shape).

- [ ] **Step 2: Run test to verify it fails**

Run: `bash tests/test_docket_facade.sh`
Expected: FAIL — `docket: unknown operation: render-learnings-index`.

- [ ] **Step 3: Add the op**

In `scripts/docket.sh` line 36, append the token to `WRAPPED_OPS` (the `for _o in $WRAPPED_OPS` loop auto-dispatches; no `case` arm needed):

```bash
WRAPPED_OPS="docket-status board-refresh archive-change terminal-publish cleanup-feature-branch github-mirror sync-integration-branch render-change-links render-adr-index render-learnings-index adr-checks board-checks runner-dispatch"
```

- [ ] **Step 4: Run test to verify it passes**

Run: `bash tests/test_docket_facade.sh`
Expected: `PASS`.

- [ ] **Step 5: Commit**

```bash
git add scripts/docket.sh tests/test_docket_facade.sh
git commit -m "feat(0067): wire render-learnings-index into the docket.sh facade"
```

---

### Task 4: The convention — the single source

**Files:**
- Modify: `skills/docket-convention/SKILL.md` — the *Learnings ledger* section, the Directory-layout block, and the `.docket.yml` schema block
- Test: `tests/test_learnings_ledger.sh` (update the convention asserts **in this task** — they hardcode the old contract and WILL redden otherwise)

**Interfaces:**
- Consumes: nothing (prose).
- Produces: the vocabulary every later task's prose references — `learnings/`, `promotion_state`, `retained | candidate | promoted`, `learnings.enabled`, `learnings.cap`, "will the agent know to search for this?".

The existing asserts that this task breaks (verified by reading `tests/test_learnings_ledger.sh`):
- `convention names the ledger path` → `grep -qF "LEARNINGS.md"`
- `convention states the ~300-line soft cap` → `grep -qF "~300 lines"`
- `directory layout lists LEARNINGS.md` → `grep -qF "LEARNINGS.md            # curated"`

- [ ] **Step 1: Rewrite the Directory-layout block**

In `skills/docket-convention/SKILL.md`, replace the `LEARNINGS.md` line in the layout block with:

```
  LEARNINGS.md            # pointer stub → learnings/ (the pre-0067 single-file ledger)
  learnings/              # curated build-loop findings; harvested at close-out (see "Learnings ledger")
    <slug>.md             # one finding per lesson/family — living files, extended on re-hit
    README.md             # GENERATED index (render-learnings-index.sh); never hand-edited
```

- [ ] **Step 2: Rewrite the *Learnings ledger* section**

Replace the whole `### Learnings ledger` section body. It must carry, as the single source (ADR-0003):

```markdown
### Learnings ledger

`<changes_dir>/learnings/` — the project's **build-loop memory**: one curated finding per file, on
`metadata_branch` only (never published to the integration branch). A **finding** is one lesson or one
consolidated family. `LEARNINGS.md` remains as a pointer stub to the pre-0067 single-file ledger.

**Structure — index + detail.** The finding *files* are curated prose, written only by the harvest and
by human curation — **never regenerated**. The *index* (`learnings/README.md`) is a **derived view**,
rendered by `render-learnings-index.sh` (its sole writer, ADR-0012), which joins the derived-view script
family. That split is the whole design: readers pay for a small hint surface, not for history.

**Finding-file frontmatter:**

```yaml
---
slug: guards-are-code
hook: "A guard is code — mutation-test it or it is decoration."   # QUOTED (carries a colon-space)
topics: [testing, sentinels]        # first tag is the PRIMARY grouping topic
changes: [14, 15, 21]               # provenance + the harvest's idempotency key
created: 2026-06-17
updated: 2026-07-16
promotion_state: retained           # retained | candidate | promoted  (default retained, ADR-0032)
promoted_to:                        # set only when promoted: the agent-instructions file it graduated into
---

## Apply
<the distilled, actionable rule>

## War story
- 2026-07-14 (#72, PR #79) — <what happened>. …
```

**Read contract — pay per relevance.** Gated on `learnings.enabled`; when `false`, readers perform
**zero** learnings reads:
1. Load `learnings/README.md` (the index) always — a small, grouped hint surface.
2. Read only the finding files whose index line (hook + topics) bears on the change at hand.

**Readers:** `docket-implement-next` at plan time and at review; `docket-groom-next` before a brainstorm.

**Writing:** only the harvest at close-out appends (single source: the *Harvest learnings* step in
`docket-finalize-change`; `docket-status`'s sweep invokes it by reference). The harvest **creates** a
new finding or **extends** an existing one (append a dated `## War story` entry, add the change id to
`changes:`, bump `updated:`) — it **never merges two distinct findings**, which is human-gated curation.
Zero findings is normal; kills are not harvested.

**Promotion — the shrink valve.** Tiering criterion: *"will the agent know to search for this?"* A rule
that must fire **unprompted** graduates; a war story stays in retrieval. The harvest sets
`promotion_state: candidate` on `metadata_branch` and **never touches the integration branch**
(ADR-0005). A human lands the graduation in the integration-branch agent-instructions file
(`AGENTS.md`/`CLAUDE.md`, symlink-aware; `AGENTS.md` is the neutral spelling when neither exists) and
flips `promoted` + `promoted_to:`. A promoted finding leaves the topic groups for a compressed
`## Promoted` appendix and **stops counting against the cap** — but its file is **kept**, never deleted:
it is the graduated rule's receipt, the harvest's dedup memory against re-minting a duplicate, and a
one-line-reversible demotion path.

**Capacity.** `learnings.cap` (default 300) counts **active findings** (`retained` + `candidate`) — not
raw lines, and not promoted ones. Past the cap the loop **flags** `learnings over-cap — needs curation`
through the digest's needs-you channel; it **never auto-merges its own memory**. Consolidation and
promotion are human acts.

**Off switch.** `learnings.enabled: false` makes the whole subsystem a no-op **read/write gate, never a
purge**: readers skip, the harvest no-ops with a one-line note, `docket-status` skips the advisories and
the index self-heal, and `render-learnings-index.sh` is never invoked. Existing `learnings/` files are
left byte-untouched, and re-enabling resumes from them.
```

- [ ] **Step 3: Add the config keys to the `.docket.yml` schema block**

In the convention's `.docket.yml` example block, after the `finalize:` block, add:

```yaml
learnings:                   # the build-loop memory subsystem (change 0067)
  enabled: true              # default. false = whole subsystem off (read/write gate, never a purge)
  cap: 300                   # default. active-finding count past which the harvest flags "needs curation"
```

- [ ] **Step 4: Update the guards this task breaks**

In `tests/test_learnings_ledger.sh`, replace section (a) with asserts keyed on the NEW contract. Anchor each to the **unique phrase its clause owns** and confirm `grep -c` == 1 where the phrase must be singular:

```bash
# (a) the convention contract — single source
assert "convention has the Learnings ledger section" 'grep -qF "### Learnings ledger" "$CONV"'
assert "convention names the findings directory" 'grep -qF "<changes_dir>/learnings/" "$CONV"'
assert "convention names the generated index as derived" \
  'grep -qF "render-learnings-index.sh" "$CONV"'
assert "convention states the tiering criterion" \
  'grep -qF "will the agent know to search for this?" "$CONV"'
assert "convention states the cap counts active findings" \
  'grep -qF "counts **active findings**" "$CONV"'
assert "convention states the off switch is a gate, not a purge" \
  'grep -qF "read/write gate, never a" "$CONV"'
assert "convention pins the promotion_state enum" \
  'grep -qF "retained | candidate | promoted" "$CONV"'
assert "directory layout lists the learnings dir" \
  'grep -qE "^  learnings/ +# curated build-loop findings" "$CONV"'
assert "convention keeps the LEARNINGS.md stub pointer" 'grep -qF "pointer stub" "$CONV"'
```

- [ ] **Step 5: Run the test**

Run: `bash tests/test_learnings_ledger.sh`
Expected: `PASS` for section (a). Sections (b)/(c)/(d) may still pass (they guard files this task has not touched yet) — if any redden, that is a real signal; fix in the owning task.

- [ ] **Step 6: Mutation-test each new convention assert**

For each assert added in Step 4: delete the exact clause it guards from `SKILL.md`, run the test, confirm `NOT OK`, restore. **All nine must redden.** A green mutation is a defect (guards-are-code). Also confirm each anchor is singular: `grep -c "<phrase>" skills/docket-convention/SKILL.md` == 1 — if a phrase appears twice, the assert is double-guarded ((b)) and must be re-anchored.

- [ ] **Step 7: Commit**

```bash
git add skills/docket-convention/SKILL.md tests/test_learnings_ledger.sh
git commit -m "docs(0067): rewrite the convention's Learnings ledger contract — findings + derived index"
```

---

### Task 5: The harvest — `docket-finalize-change`

**Files:**
- Modify: `skills/docket-finalize-change/SKILL.md:72` (step 2.5, *Harvest learnings*)
- Test: `tests/test_learnings_ledger.sh` section (b)

**Interfaces:**
- Consumes: Task 4's convention vocabulary.
- Produces: the harvest procedure's single source, referenced by `docket-status`'s sweep (Task 6).

The existing assert this task breaks: `finalize has the idempotency probe` → `grep -qF "already cites"`.

- [ ] **Step 1: Rewrite step 2.5**

Replace the whole `2.5 **Harvest learnings.**` paragraph with:

```markdown
2.5 **Harvest learnings.** Gated on `learnings.enabled` (from the Step-0 config export): when `false`, print exactly one line — `learnings disabled — harvest skipped` — and go to step 3 (never silently; a reader must be able to tell "harvested zero" from "skipped because disabled"). When enabled: distill this change's close-out signals — PR review comments, merge-gate feedback, `results:` findings — into zero or more **findings** under `<changes_dir>/learnings/` (shape per the convention's *Learnings ledger*). For each lesson, either **create** `learnings/<slug>.md` or **extend** the existing family finding whose slug already covers the class — append a dated `## War story` entry with `(#<id>, PR #<n>)` provenance, add this change's id to `changes:`, bump `updated:`. Never merge two existing distinct findings — that is human-gated curation. Set `promotion_state: candidate` on any finding whose rule must fire **unprompted** (*"will the agent know to search for this?"*). Zero findings is normal. **Idempotency probe:** skip if some finding file's `changes:` list already contains this change's id — read via `lib/docket-frontmatter.sh`'s `list_field`, never a bare numeric grep (a bare id can match a PR number or a date). Then re-render the index — `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh render-learnings-index --learnings-dir .docket/<changes_dir>/learnings > .docket/<changes_dir>/learnings/README.md` — and commit the finding file(s) + index together as **its own commit** on `metadata_branch` (never bundled with the archive commit), only if the render actually changed bytes, and push. Kills are not harvested. This step is the harvest procedure's single source; `docket-status`'s sweep invokes it by reference.
```

- [ ] **Step 2: Update the guard**

In `tests/test_learnings_ledger.sh` section (b), replace the `already cites` assert and add the gate assert:

```bash
assert "finalize carries the harvest step" \
  'grep -qF "Harvest learnings" "$REPO/skills/docket-finalize-change/SKILL.md"'
assert "finalize's idempotency probe keys on the changes: list" \
  'grep -qF "already contains this change" "$REPO/skills/docket-finalize-change/SKILL.md"'
assert "finalize gates the harvest on learnings.enabled" \
  'grep -qF "learnings disabled — harvest skipped" "$REPO/skills/docket-finalize-change/SKILL.md"'
assert "finalize re-renders the index through the facade" \
  'grep -qF "docket.sh render-learnings-index" "$REPO/skills/docket-finalize-change/SKILL.md"'
assert "status sweep invokes the harvest by reference" \
  'grep -qF "Harvest learnings" "$REPO/skills/docket-status/SKILL.md" && grep -qF "docket-finalize-change" "$REPO/skills/docket-status/SKILL.md"'
```

- [ ] **Step 3: Run the test**

Run: `bash tests/test_learnings_ledger.sh`
Expected: `PASS`.

- [ ] **Step 4: Run the facade-wiring guard**

Run: `bash tests/test_skill_facade_wiring.sh`
Expected: `PASS` — the new `docket.sh render-learnings-index` invocation must use the canonical
`"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh` prefix (ADR-0030 discriminates on the
invocation prefix). If it reddens, fix the prose's invocation spelling — never the guard.

- [ ] **Step 5: Mutation-test the four new asserts**

Delete each guarded clause in turn; confirm `NOT OK`; restore. All four must redden.

- [ ] **Step 6: Commit**

```bash
git add skills/docket-finalize-change/SKILL.md tests/test_learnings_ledger.sh
git commit -m "docs(0067): rewrite the harvest step — findings, index re-render, enable gate"
```

---

### Task 6: `docket-status` — self-heal + advisories

**Files:**
- Modify: `skills/docket-status/SKILL.md`
- Modify: `scripts/docket-status.sh`
- Test: `tests/test_learnings_ledger.sh`, `tests/test_docket_status.sh`

**Interfaces:**
- Consumes: `LEARNINGS_ENABLED`/`LEARNINGS_CAP` (Task 2); `docket.sh render-learnings-index` (Tasks 1+3).
- Produces: the index self-heal render + two needs-you advisories.

**Read `scripts/docket-status.sh` first** and follow its existing structure: how the Board pass renders → diffs → commits-only-if-changed → pushes with a rebase-retry, and how the digest emits `change <id> <status>` lines. The learnings render is the **exact same shape** as the board render. Do not invent a new pattern.

- [ ] **Step 1: Write the failing tests**

In `tests/test_docket_status.sh`, following the file's existing sandbox idiom, add:

```bash
# --- change 0067: the learnings index self-heal + advisories -------------------
# (a) enabled: a stale index is re-rendered and committed
assert "status re-renders a stale learnings index" \
  'printf "%s" "$out" | grep -qE "^learnings index (clean|changed)"'

# (b) disabled: exactly one note, and the renderer is NEVER invoked
assert "disabled emits exactly one learnings-disabled note" \
  '[ "$(printf "%s" "$out_disabled" | grep -cF "learnings disabled")" = "1" ]'
assert "disabled never invokes the renderer" \
  '! printf "%s" "$trace_disabled" | grep -qF "render-learnings-index"'
assert "disabled leaves an existing finding file byte-untouched" \
  '[ "$(cat "$LD/seeded.md")" = "$SEEDED_BYTES" ]'

# (c) over-cap advisory (needs-you channel, ADR-0028)
assert "over-cap surfaces the needs-you advisory" \
  'printf "%s" "$out_overcap" | grep -qF "learnings over-cap — needs curation"'
assert "under-cap emits no over-cap advisory" \
  '! printf "%s" "$out" | grep -qF "over-cap"'

# (d) promotion-pending advisory
assert "a candidate finding surfaces promotion-pending" \
  'printf "%s" "$out_candidate" | grep -qF "learnings promotion-pending 1"'

# (e) the cap counts ACTIVE findings — a promoted finding must not count
assert "promoted findings do not count toward the cap" \
  '! printf "%s" "$out_promoted_over" | grep -qF "over-cap"'
```

Seed each fixture with real finding files (reuse Task 1's `mkfinding` shape) and a `.docket.yml`
carrying the relevant `learnings:` block. **Fixture realism (green-suite family):** the sandbox's
`SCRIPTS_DIR` must carry the **real** `render-learnings-index.sh` and its `lib/` — a mock dir
omitting it would route every test through the best-effort degrade branch and leave the happy path
untested while the suite reads green.

- [ ] **Step 2: Run to verify it fails**

Run: `bash tests/test_docket_status.sh`
Expected: FAIL — no learnings lines are emitted.

- [ ] **Step 3: Implement the pass in `scripts/docket-status.sh`**

Add a learnings pass mirroring the board pass's write decision. Gate FIRST on `LEARNINGS_ENABLED`
(short-circuit before `cap` is consulted):

```bash
# --- learnings pass (change 0067) --------------------------------------------
# Gated FIRST on learnings.enabled — the gate short-circuits before cap is consulted, and the
# renderer is never invoked when disabled. Same write decision as the board pass: render to a
# temp, diff, commit ONLY if the bytes changed, push with the existing rebase-retry.
# The disabled note is deliberate, not silence (ADR-0032 positive off-state + the sole-channel
# totality learning): "no line" is indistinguishable from success.
learnings_pass(){
  if [ "${LEARNINGS_ENABLED:-true}" != "true" ]; then
    printf 'learnings disabled\n'
    return 0
  fi
  local ldir="$mw/$CHANGES_DIR/learnings"
  [ -d "$ldir" ] || { printf 'learnings index skipped (no learnings dir)\n'; return 0; }
  local tmp; tmp="$(mktemp)"
  if ! "$SCRIPTS_DIR"/render-learnings-index.sh --learnings-dir "$ldir" >"$tmp" 2>/dev/null; then
    rm -f "$tmp"; printf 'learnings index failed\n'; return 0
  fi
  if [ -f "$ldir/README.md" ] && cmp -s "$tmp" "$ldir/README.md"; then
    rm -f "$tmp"; printf 'learnings index clean\n'
  else
    mv "$tmp" "$ldir/README.md"
    printf 'learnings index changed\n'
    # commit + push via the SAME helper the board pass uses — read it and reuse; do not
    # hand-roll a second commit path.
  fi
  learnings_advisories "$ldir"
}
```

Then the advisories, counting **active** findings via the frontmatter lib (never a bare grep):

```bash
# Cap counts ACTIVE findings (retained + candidate) — promoted ones are the shrink valve and
# must not count. Read promotion_state through the frontmatter lib, keyed on shape.
learnings_advisories(){
  local ldir="$1" f state active=0 candidates=0
  while IFS= read -r f; do
    [ -n "$f" ] || continue
    state="$(field "$f" promotion_state)"; state="${state:-retained}"
    [ "$state" = "promoted" ] && continue
    active=$((active + 1))
    [ "$state" = "candidate" ] && candidates=$((candidates + 1))
  done < <(find "$ldir" -maxdepth 1 -name '*.md' ! -name 'README.md' 2>/dev/null | sort)
  if [ "$active" -gt "${LEARNINGS_CAP:-300}" ]; then
    printf 'learnings over-cap — needs curation (%d active, cap %d)\n' "$active" "${LEARNINGS_CAP:-300}"
  fi
  [ "$candidates" -gt 0 ] && printf 'learnings promotion-pending %d — needs you\n' "$candidates"
  return 0
}
```

Wire `learnings_pass` into the full pass (NOT `--board-only`, which is the board's dedicated entry
point). Source `lib/docket-frontmatter.sh` if the script does not already.

- [ ] **Step 4: Run to verify it passes**

Run: `bash tests/test_docket_status.sh`
Expected: `PASS`.

- [ ] **Step 5: Update `skills/docket-status/SKILL.md`**

Document the learnings pass: the enable gate, the once-per-pass `learnings disabled` note, the
index self-heal (derived view, commit-only-if-changed), and the two needs-you advisories. Keep the
harvest **by reference** to `docket-finalize-change` (do not restate it — ADR-0003 single source).

- [ ] **Step 6: Add the reader/advisory guards**

In `tests/test_learnings_ledger.sh`:

```bash
assert "status documents the learnings enable gate" \
  'grep -qF "learnings disabled" "$REPO/skills/docket-status/SKILL.md"'
assert "status documents the index self-heal as a derived view" \
  'grep -qF "render-learnings-index" "$REPO/skills/docket-status/SKILL.md"'
assert "status documents both needs-you advisories" \
  'grep -qF "over-cap" "$REPO/skills/docket-status/SKILL.md" && grep -qF "promotion-pending" "$REPO/skills/docket-status/SKILL.md"'
```

- [ ] **Step 7: Mutation-test**

Delete each guarded clause; confirm `NOT OK`; restore. Additionally mutate the **code**: make
`learnings_pass` ignore the gate (drop the `!= "true"` early return) and confirm test (b)'s
"disabled never invokes the renderer" reddens. If it stays green, the fixture is not exercising the
branch (green-suite family) — fix the fixture.

- [ ] **Step 8: Commit**

```bash
git add scripts/docket-status.sh skills/docket-status/SKILL.md tests/test_docket_status.sh tests/test_learnings_ledger.sh
git commit -m "feat(0067): docket-status learnings pass — gated self-heal render + two advisories"
```

---

### Task 7: The readers

**Files:**
- Modify: `skills/docket-implement-next/SKILL.md` (Step 4 plan time; Step 6 review)
- Modify: `skills/docket-groom-next/SKILL.md` (before the brainstorm)
- Test: `tests/test_learnings_ledger.sh` section (c), `tests/test_auto_groom.sh`

**Interfaces:**
- Consumes: Task 4's convention read contract.
- Produces: the two-step read at all three hot moments.

The existing asserts this task breaks:
- `implement-next reads the ledger at plan time and review` → `[ "$(grep -cF "LEARNINGS.md" …)" -ge 2 ]`
- `groom-next reads the ledger in scan-context` → `grep -qF "LEARNINGS.md" …`
- `tests/test_auto_groom.sh` asserts `LEARNINGS.md` in `skills/docket-auto-groom/SKILL.md` — **check it** (`grep -n "LEARNINGS" tests/test_auto_groom.sh`) and update in this task if it reddens.

- [ ] **Step 1: Rewrite the implement-next read lines**

In `skills/docket-implement-next/SKILL.md` Step 4, replace the `LEARNINGS.md` read sentence with:

```markdown
Alongside the spec, read the learnings index `<changes_dir>/learnings/README.md` from the same metadata working tree, then read the individual finding files whose hook + topics bear on this change — past lessons inform the plan. Skip both steps entirely when `learnings.enabled` is `false`.
```

In Step 6, replace the review read sentence with:

```markdown
Re-read the learnings index `<changes_dir>/learnings/README.md` first and pull the findings relevant to what this change touched, so past lessons feed the review (skipped entirely when `learnings.enabled` is `false`).
```

- [ ] **Step 2: Rewrite the groom-next read line**

In `skills/docket-groom-next/SKILL.md`, replace the `LEARNINGS.md` read sentence with:

```markdown
Read the learnings index `<changes_dir>/learnings/README.md` BEFORE the brainstorm and pull any findings whose hook bears on the stub, so the conversation is informed by adjacent work and past lessons (skipped entirely when `learnings.enabled` is `false`).
```

- [ ] **Step 3: Update the guards**

Replace section (c) of `tests/test_learnings_ledger.sh`:

```bash
# (c) the readers — the two-step index-first read contract, at all three hot moments
assert "implement-next reads the index at plan time AND review" \
  '[ "$(grep -cF "learnings/README.md" "$REPO/skills/docket-implement-next/SKILL.md")" -ge 2 ]'
assert "implement-next gates its reads on learnings.enabled" \
  '[ "$(grep -cF "learnings.enabled" "$REPO/skills/docket-implement-next/SKILL.md")" -ge 2 ]'
assert "groom-next reads the index before the brainstorm" \
  'grep -qF "learnings/README.md" "$REPO/skills/docket-groom-next/SKILL.md"'
assert "groom-next gates its read on learnings.enabled" \
  'grep -qF "learnings.enabled" "$REPO/skills/docket-groom-next/SKILL.md"'
# No reader may still point at the retired single-file ledger as a READ target.
for sk in docket-implement-next docket-groom-next; do
  assert "$sk no longer reads the retired LEARNINGS.md" \
    '! grep -qE "read .*LEARNINGS\.md" "$REPO/skills/$sk/SKILL.md"'
done
```

- [ ] **Step 4: Run the tests**

Run: `bash tests/test_learnings_ledger.sh && bash tests/test_auto_groom.sh`
Expected: `PASS` both. If `test_auto_groom.sh` reddens on its `LEARNINGS.md` assert, update that
assert to the index path in this task (it is this task's breakage).

- [ ] **Step 5: Mutation-test**

Delete each guarded read sentence; confirm `NOT OK`; restore. Confirm the `-ge 2` count asserts are
not satisfied by a single line mentioning the path twice — `grep -c` counts LINES, so verify the two
matches are on genuinely different lines (`grep -nF "learnings/README.md" <file>`).

- [ ] **Step 6: Commit**

```bash
git add skills/docket-implement-next/SKILL.md skills/docket-groom-next/SKILL.md tests/test_learnings_ledger.sh tests/test_auto_groom.sh
git commit -m "docs(0067): readers load the index and pull by relevance, gated on learnings.enabled"
```

---

### Task 8: Surfacing — `.docket.yml`, README, `AGENTS.md`

**Files:**
- Modify: `.docket.yml`
- Modify: `README.md`
- Create: `AGENTS.md`
- Test: `tests/test_learnings_ledger.sh`

**Interfaces:**
- Consumes: Task 2's keys, Task 4's vocabulary.
- Produces: the promotion destination (`AGENTS.md`) that Task 10's migration marks candidates against.

**Scope note:** `config.yml.example` is deliberately **excluded** — see *Known spec deviations* #1.

- [ ] **Step 1: Add the commented sample block to `.docket.yml`**

After the `finalize:` block, add:

```yaml
# The learnings subsystem (change 0067) — the build loop's memory. Findings live in
# <changes_dir>/learnings/ (one curated finding per file) with a GENERATED README.md index;
# groom/plan/review load the index and pull only relevant findings. Both keys are global-able
# (settable in ~/.config/docket/config.yml or .docket.local.yml).
#   enabled: false turns the WHOLE subsystem off — readers skip (zero token cost), the harvest
#     no-ops with a one-line note, and the index is never rendered. It is a read/write GATE, never
#     a purge: existing learnings/ files are left untouched and re-enabling resumes from them.
#     Note: because the harvest is one-shot at close-out, a change that closes on a disabled
#     machine is permanently un-harvested — a deliberate, bounded cost.
#   cap: the ACTIVE-finding count (retained + candidate; promoted ones don't count) past which
#     docket flags "needs curation". docket never auto-merges its own memory — you curate.
# This repo runs the defaults, so the block stays commented out.
#
# learnings:
#   enabled: true
#   cap: 300
```

- [ ] **Step 2: Create `AGENTS.md`**

The promotion destination. Neutral spelling (neither `AGENTS.md` nor `CLAUDE.md` exists on `main`
today — verified). It must be genuinely useful from day one, not an empty placeholder:

```markdown
# AGENTS.md — always-in-context rules for this repo

Rules that must fire **unprompted**. This file is the graduation destination for docket's learnings
findings: when a lesson passes the tiering criterion — *"will the agent know to search for this?"* —
a human promotes it here and flips its finding to `promotion_state: promoted`. Everything that is a
*war story* rather than a *rule* stays in `docs/changes/learnings/` on the `docket` branch and is
pulled by relevance, not loaded here.

Promotion is human-gated by construction: the harvest proposes (`candidate`), a human disposes. See
the docket-convention skill's *Learnings ledger* section for the mechanics — this file deliberately
does not restate them.

## Shell

- Never `producer | early-exiting-consumer` (`grep -q`, `head`, `head -n1`) under `set -o pipefail`
  — the producer takes SIGPIPE and the 141 becomes an intermittent failure. Capture into a variable
  first, then `grep <<<"$var"`.
- `grep` for a pattern that leads with `--` must declare it: `grep -E -e "<pat>"` or
  `grep -qF -- "<pat>"`. A bare leading `--` is parsed as an option (exit 2) — and inside a negated
  assert (`! grep …`), that error inverts into a permanently green, vacuous guard.
- awk indent classes are `[^[:space:]]`, never `[^ ]` — a literal-space class silently drops
  tab-indented input.

## Frontmatter and generated blocks

- Anchor a frontmatter-field edit to the first `---…---` block, never a bare column-0 line match:
  docket's own change/ADR files discuss `status:`/`updated:` in body prose.
- Quote any hand-authored YAML scalar carrying a colon-space or a boolean keyword
  (`on/off/yes/no/true/false`). Today's grep/awk reader tolerating it is not evidence it is
  well-formed.
- Before rewriting a marker-delimited managed block, validate marker **order and balance** — refuse
  on dangling/out-of-order/nested markers and leave the file untouched. Presence alone is not
  enough; an unbounded range consumes to EOF and eats the user's content.

## Guards and tests

- A guard is code: mutation-test it — strip the thing it guards, watch it redden — or it is
  decoration. A mutation that leaves an assert green is a defect until proven otherwise.
- Key a guard on syntactic **shape**, never an enumerated list of spellings. The spelling you miss
  is the target file's own house idiom.
- Never hand-list the sites of a literal or an operation you are gating — derive them from a
  whole-repo grep, then sort them into prose vs executable. Only the executable ones can violate a
  gate, and a docs-shaped reading skips right past them.
- Run the whole suite at the build gate, never only the tests the spec enumerated.
```

- [ ] **Step 3: Add the README section**

In `README.md`, add a first-class feature section. Factual register; **orientation + pointer only** —
it must NOT restate mechanics the convention owns (prose restating another file's fact is a drift
surface no sentinel can catch; verify-the-claim family). Place it near the other feature sections:

```markdown
## Learnings — the loop's memory

The repo gets smarter as changes ship. Every change that reaches `done` distills its close-out
signals — PR review comments, merge-gate feedback, results findings — into a curated **finding**
(zero is normal, and kills are never harvested).

- **Findings + a rendered index.** One file per lesson or consolidated family under
  `docs/changes/learnings/` on the metadata branch, plus a generated `README.md` index.
- **Pay per relevance.** Groom, plan, and review load the index — a small hint surface — and pull
  only the findings that bear on the change at hand, instead of paying for the whole history on
  every run.
- **Human-gated promotion.** A rule that must fire unprompted graduates into `AGENTS.md`/`CLAUDE.md`,
  where it is always in context; the finding then stops taxing the retrieval surface. docket
  proposes the candidate — it never edits your always-in-context file, and never auto-merges its
  own memory.
- **Controls.** `learnings.enabled` turns the subsystem off wholesale (a read/write gate, never a
  purge); `learnings.cap` sets the active-finding count past which docket flags "needs curation".

Mechanics — the finding schema, the harvest's write moments, the promotion states — live in the
`docket-convention` skill's *Learnings ledger* section, which is their single source.
```

- [ ] **Step 4: Add the surfacing guards**

In `tests/test_learnings_ledger.sh`:

```bash
# (e) end-to-end surfacing — LEARNINGS #49: a knob is not done when it merely works
assert "the sample .docket.yml carries the learnings block" \
  'grep -qE "^# learnings:$" "$REPO/.docket.yml"'
assert "the sample documents both keys" \
  'grep -qE "^#   enabled: true$" "$REPO/.docket.yml" && grep -qE "^#   cap: 300$" "$REPO/.docket.yml"'
assert "README presents learnings as a feature" 'grep -qF "## Learnings — the loop" "$REPO/README.md"'
assert "README points at the convention rather than restating mechanics" \
  'grep -qF "single source" "$REPO/README.md"'
assert "AGENTS.md exists as the promotion destination" '[ -f "$REPO/AGENTS.md" ]'
assert "AGENTS.md states the tiering criterion" \
  'grep -qF "will the agent know to search for this?" "$REPO/AGENTS.md"'
```

- [ ] **Step 5: Run the tests + mutation-test**

Run: `bash tests/test_learnings_ledger.sh && bash tests/test_config_example.sh`
Expected: `PASS` both (`test_config_example.sh` must stay green — this task does not touch
`config.yml.example`).
Then mutation-test each new assert (delete the clause, confirm `NOT OK`, restore).

- [ ] **Step 6: Commit**

```bash
git add .docket.yml README.md AGENTS.md tests/test_learnings_ledger.sh
git commit -m "docs(0067): surface the learnings subsystem — sample config, README, AGENTS.md"
```

---

### Task 9: Whole-suite gate + anti-restatement sweep

**Files:**
- Modify: `tests/test_learnings_ledger.sh` (section (d) sentinels)

**Interfaces:**
- Consumes: every prior task.
- Produces: a green whole suite — the merge gate's precondition.

- [ ] **Step 1: Re-point the anti-restatement sentinels**

Section (d) currently scans for `"build-loop memory"` and `"compression, not destruction"` in the
convention and asserts no operating skill restates them. `compression, not destruction` is **gone**
from the new contract (the cap now flags rather than auto-merges). Re-point to phrases the NEW
contract owns, and keep the anti-restatement discipline (ADR-0003 single source):

```bash
# (d) anti-restatement sentinels — contract phrases live ONLY in the convention
for s in "build-loop memory" "will the agent know to search for this?"; do
  assert "convention contains sentinel: $s" 'grep -qF "$s" "$CONV"'
  for sk in "${OPERATING[@]}"; do
    f="$REPO/skills/$sk/SKILL.md"
    assert "$sk does not restate: $s" '[ -f "$f" ] && ! grep -qF "$s" "$f"'
  done
done
```

**Careful:** Task 5's harvest step quotes the tiering criterion. If it does, that is a genuine
restatement — either drop it from finalize's prose (preferred: reference the convention) or exclude
finalize from that sentinel's loop with an explicit comment saying why. Decide by reading, not
guessing; do **not** loosen the sentinel silently.

- [ ] **Step 2: Run the WHOLE suite (one foreground run)**

Run every test, not just the enumerated ones (enumerated-floor (c) — an out-of-goal regression is
exactly what the tests outside the goal set exist to catch):

```bash
cd /Users/homer/dev/docket/.worktrees/learnings-promotion-destination
fails=""; for t in tests/test_*.sh; do
  if ! out="$(bash "$t" 2>&1)"; then fails="$fails $t"; fi
done
printf 'FAILED:%s\n' "${fails:-  (none)}"
```

Expected: `FAILED:   (none)`.

- [ ] **Step 3: Triage any red test**

For each failure, decide: a real regression (fix the code) or a sentinel legitimately dated by this
change (NARROW it to the property that is still load-bearing — **never delete it**; deleting is how
the guarded hole reopens). If a red test looks environment-bound, re-run the identical test against
unmodified `origin/main` and byte-compare the failing sets before calling it env-bound (environment
family) — record the differential.

- [ ] **Step 4: Commit**

```bash
git add tests/test_learnings_ledger.sh
git commit -m "test(0067): re-point the anti-restatement sentinels to the new contract"
```

---

### Task 10: Migrate the ledger (metadata branch — build-time data write)

**Files:**
- Create (on `docket`, in `/Users/homer/dev/docket/.docket/`): `docs/changes/learnings/<slug>.md` × N
- Create: `docs/changes/learnings/README.md` (rendered, never hand-written)
- Modify: `docs/changes/LEARNINGS.md` → pointer stub

**Interfaces:**
- Consumes: Task 1's renderer, Task 4's schema.
- Produces: the acceptance proof, recorded in the results file (Task 11).

**This is NOT a feature-branch commit.** These files live on `metadata_branch` and are invisible to
the integration-branch suite (LEARNINGS #6). Commit them in `/Users/homer/dev/docket/.docket/` on
`docket`, pushed immediately. Never `git add` them in the feature worktree.

- [ ] **Step 1: Convert each family entry to a finding file**

Read `/Users/homer/dev/docket/.docket/docs/changes/LEARNINGS.md` (490 lines, 33 top-level entries).
Each consolidated **family** entry becomes one finding whose `changes:` carries every cited id and
whose `## War story` carries the dated `(#<id>, PR #<n>)` sub-entries **verbatim**. The families
present (derive this list from the file — do not trust this enumeration; it is a floor):
`guards-are-code`, `sole-channel`, `enumerated-floor`, `moving-base`, `verify-the-claim`,
`environment`, `green-suite-untested-branch`, `shell-portability`, `yaml-scalar`,
`adr-update-delivery`, `pipefail`, `idempotency-keying`.

Each standalone dated entry becomes its own finding file with a slug describing its class.

Frontmatter per Task 4's schema. `hook:` is the entry's distilled `Apply:` rule, **quoted**.
`topics:` first tag is the primary group.

- [ ] **Step 2: Mark graduation candidates**

Criterion-first (**not** a fixed list): set `promotion_state: candidate` on findings whose `Apply:`
rule must fire **unprompted** — e.g. pipefail's "never `producer | early-exiting-consumer`", the
frontmatter-anchor rule, the `grep -E -e` rule. A war story about one change's specific bug is
`retained`. The rules already written into `AGENTS.md` in Task 8 are the natural candidate set —
keep the two consistent.

- [ ] **Step 3: Render the index**

```bash
cd /Users/homer/dev/docket
scripts/render-learnings-index.sh --learnings-dir .docket/docs/changes/learnings \
  > .docket/docs/changes/learnings/README.md
```

**Never hand-write this file** — the renderer is its sole writer. Verify it is non-empty and lists
every finding.

- [ ] **Step 4: Reduce `LEARNINGS.md` to a pointer stub**

Do NOT delete it (LEARNINGS #20 — leave a stub + pointer so name-based cross-refs still resolve):

```markdown
<!-- LEARNINGS.md — the pre-0067 single-file ledger. Superseded by the findings directory. -->

# Learnings

→ **Moved to [`learnings/`](learnings/).** Start at [`learnings/README.md`](learnings/README.md) —
the generated index — and read only the findings relevant to the change at hand.

The single-file ledger this replaced grew to 490 lines with no way to shrink; change 0067 gave it an
index + a promotion valve. Git history keeps every pre-migration byte. Contract: the
`docket-convention` skill's *Learnings ledger* section.
```

- [ ] **Step 5: Verify the migration (the acceptance proof)**

```bash
cd /Users/homer/dev/docket
echo "findings:  $(find .docket/docs/changes/learnings -maxdepth 1 -name '*.md' ! -name 'README.md' | wc -l)"
echo "index lines: $(wc -l < .docket/docs/changes/learnings/README.md)"
echo "stub lines:  $(wc -l < .docket/docs/changes/LEARNINGS.md)"
# every finding must appear in the index exactly once (or in ## Promoted)
for f in .docket/docs/changes/learnings/*.md; do
  b="$(basename "$f")"; [ "$b" = README.md ] && continue
  n="$(grep -cF "($b)" .docket/docs/changes/learnings/README.md)"
  [ "$n" = "1" ] || echo "MISCOUNT: $b appears $n times"
done
# idempotency: a second render must be byte-identical
scripts/render-learnings-index.sh --learnings-dir .docket/docs/changes/learnings > /tmp/idx2
cmp .docket/docs/changes/learnings/README.md /tmp/idx2 && echo "render is idempotent"
```

Expected: every finding counted once; `render is idempotent`; no `MISCOUNT` lines.
**No lesson may be lost** — byte-check that each family's war-story sub-entries survived.

- [ ] **Step 6: Commit on the metadata branch**

```bash
git -C /Users/homer/dev/docket/.docket add docs/changes/learnings docs/changes/LEARNINGS.md
git -C /Users/homer/dev/docket/.docket commit -m "docket(0067): migrate the ledger to findings + rendered index"
git -C /Users/homer/dev/docket/.docket push origin docket
```

---

### Task 11: Results file + ADR

**Files:**
- Create: `docs/results/2026-07-16-learnings-promotion-destination-results.md` (feature branch)

**Interfaces:**
- Consumes: every task's findings.
- Produces: the merge-gate record.

- [ ] **Step 1: Author the results file**

From `docs/results/results-template.md`. It MUST record:
- **The migration proof** (Task 10's counts + idempotency check) — metadata-branch artifacts are
  invisible to the suite, so this file is their only receipt (LEARNINGS #6).
- **Spec deviation #1** — `config.yml.example` deliberately not touched (ADR-0039 + the file's own
  scoping sentence); intent honored via `.docket.yml` + convention + README. Flag for the human.
- **Spec deviation #2** — 490 lines / 33 entries, not 491.
- **The block-scoped config parse** — why `learnings:` uses `yaml_block_body` rather than
  finalize's bare leaf read.
- **Mutation-test outcomes** for every new sentinel (which mutations reddened; any that did not, and
  what was fixed).
- **Manual/interactive checks for the human at the merge gate:** the real harvest, a real promotion
  flip, and a `learnings.enabled: false` sweep are behavioral, metadata-branch operations the suite
  cannot reach.

- [ ] **Step 2: Record the ADR**

The ADR is recorded via the `docket-adr` subagent (the implementer's Step 6), **not** hand-written
here. It carries the six decisions in spec §5, `relates_to: [5]`, `change: 67`. Additionally record
the **block-scoped config parse** decision from Task 2 if the ADR author judges it non-obvious.
ADR-0005 stays `Accepted` and gains a dated `## Update` pointing forward; keep `5` in the change's
`adrs:` so terminal-publish re-copies it atomically (ADR-update-delivery family).

- [ ] **Step 3: Commit**

```bash
git add docs/results/2026-07-16-learnings-promotion-destination-results.md
git commit -m "results(0067): build receipt — migration proof, spec deviations, mutation outcomes"
```

---

## Self-Review

**1. Spec coverage.**

| Spec section | Task |
|---|---|
| §4.1 findings dir + frontmatter schema | 4 (schema), 10 (migration) |
| §4.2 rendered derived index + Promoted appendix + candidate marker | 1 |
| §4.3 pay-per-relevance read contract | 4 (contract), 7 (readers) |
| §4.4 harvest writes findings + idempotency probe | 5 |
| §4.5 config surface, enable gate, cap + escalation, fence class | 2, 6, 8 |
| §4.6 promotion valve + keep-the-file + self-heal | 1 (appendix), 4 (contract), 6 (self-heal), 8 (AGENTS.md) |
| §4.7 migration | 10 |
| §4.8 prose/skill/test/README/AGENTS.md sites | 4, 5, 6, 7, 8 (config.yml.example deliberately excluded — deviation #1) |
| §5 one ADR, relates_to ADR-0005 | 11 |
| §6 assumptions | honored throughout; #2's "mirrors finalize:" refined to block-scoped parsing (Task 2) |
| §7 risks | recorded in the results file (11) |

**2. Placeholder scan.** No TBD/TODO. Every code step carries real code. Two steps deliberately say
"read the existing file and follow its idiom" (Task 2's config-test helpers, Task 6's commit helper)
— these are *reuse* instructions with named targets, not placeholders: inventing parallel helpers
there would be the defect.

**3. Type consistency.** `--learnings-dir` is the flag everywhere (Tasks 1, 3, 5, 6, 10).
`LEARNINGS_ENABLED`/`LEARNINGS_CAP` are the exported names everywhere (Tasks 2, 6). `promotion_state`
values are `retained | candidate | promoted` everywhere (Tasks 1, 4, 6, 10). `promoted_to:` is the
target field everywhere. `render-learnings-index` is the facade op token in Tasks 3, 5, 6.
