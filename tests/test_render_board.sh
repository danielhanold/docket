#!/usr/bin/env bash
# tests/test_render_board.sh — verifies change 0022: deterministic BOARD.md rendering.
# A fixture changes/ tree spanning every status is rendered and byte-compared to a hand-authored
# golden; a second render must be byte-identical (idempotence). Also asserts the docket-status
# inline-surface wiring. Run: bash tests/test_render_board.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$REPO/scripts/render-board.sh"
SKILL="$REPO/skills/docket-status/SKILL.md"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

assert "script exists and is executable" '[ -x "$SCRIPT" ]'

tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/active" "$tmp/archive"

cat > "$tmp/active/0001-alpha.md" <<'EOF'
---
id: 1
slug: alpha
title: Alpha feature
status: in-progress
priority: high
depends_on: []
spec: docs/superpowers/specs/2026-06-10-alpha.md
branch: feat/alpha
EOF
cat > "$tmp/active/0002-bravo.md" <<'EOF'
---
id: 2
slug: bravo
title: Bravo feature
status: proposed
priority: medium
depends_on: [10]
spec: docs/superpowers/specs/2026-06-10-bravo.md
EOF
cat > "$tmp/active/0003-charlie.md" <<'EOF'
---
id: 3
slug: charlie
title: Charlie feature
status: proposed
priority: medium
depends_on: []
spec:
EOF
cat > "$tmp/active/0004-delta.md" <<'EOF'
---
id: 4
slug: delta
title: Delta feature
status: proposed
priority: low
depends_on: []
spec:
---

## Auto-groom blocked

2026-06-12 — abstained.
EOF
cat > "$tmp/active/0005-echo.md" <<'EOF'
---
id: 5
slug: echo
title: Echo feature
status: proposed
priority: medium
depends_on: [3]
spec: docs/superpowers/specs/2026-06-10-echo.md
EOF
cat > "$tmp/active/0006-foxtrot.md" <<'EOF'
---
id: 6
slug: foxtrot
title: Foxtrot feature
status: proposed
priority: medium
depends_on: [8]
spec: docs/superpowers/specs/2026-06-10-foxtrot.md
EOF
cat > "$tmp/active/0007-golf.md" <<'EOF'
---
id: 7
slug: golf
title: Golf feature
status: blocked
priority: medium
depends_on: []
blocked_by: upstream API frozen until Q3
EOF
cat > "$tmp/active/0008-hotel.md" <<'EOF'
---
id: 8
slug: hotel
title: Hotel feature
status: implemented
priority: high
depends_on: []
pr: https://github.com/o/r/pull/142
EOF
cat > "$tmp/active/0009-india.md" <<'EOF'
---
id: 9
slug: india
title: India feature
status: deferred
priority: low
depends_on: []
EOF
cat > "$tmp/active/0013-mike.md" <<'EOF'
---
id: 13
slug: mike
title: Mike feature
status: implemented
priority: high
depends_on: []
pr: https://github.com/o/r/pull/151
---

## Finalize blocked

2026-07-18 — ambiguous rebase conflict; resolve by hand and re-run.
EOF
cat > "$tmp/archive/2026-06-15-0010-juliet.md" <<'EOF'
---
id: 10
slug: juliet
title: Juliet feature
status: done
priority: medium
depends_on: []
EOF
cat > "$tmp/archive/2026-06-16-0012-lima.md" <<'EOF'
---
id: 12
slug: lima
title: Lima feature
status: done
priority: medium
depends_on: []
EOF
cat > "$tmp/archive/2026-06-14-0011-kilo.md" <<'EOF'
---
id: 11
slug: kilo
title: Kilo feature
status: killed
priority: low
depends_on: []
EOF

# Hand-authored golden — the executable form of docket-status Board -> Structure.
golden="$tmp/golden.md"
cat > "$golden" <<'EOF'
# Backlog

**13 changes** — 🟢 1 in progress · 🟡 5 proposed · 🔴 1 blocked · ⚪ 1 deferred · 🔵 2 implemented · ✅ 2 done · 🗑️ 1 killed

## 🟢 In progress (1)

| # | Title | Priority | Spec | Branch |
|---|-------|----------|------|--------|
| [0001](active/0001-alpha.md) | Alpha feature | `high` | [spec](../superpowers/specs/2026-06-10-alpha.md) | `feat/alpha` |

## 🟡 Proposed (5)

| # | Title | Priority | Readiness |
|---|-------|----------|-----------|
| [0002](active/0002-bravo.md) | Bravo feature | `medium` | build-ready |
| [0003](active/0003-charlie.md) | Charlie feature | `medium` | needs-brainstorm |
| [0004](active/0004-delta.md) | Delta feature | `low` | auto-groom blocked — needs you |
| [0005](active/0005-echo.md) | Echo feature | `medium` | ⏳ waiting on #3 — not yet built |
| [0006](active/0006-foxtrot.md) | Foxtrot feature | `medium` | ⏳ waiting on #8 — needs your merge |

## 🔴 Blocked (1)

| # | Title | Priority | Blocked by |
|---|-------|----------|------------|
| [0007](active/0007-golf.md) | Golf feature | `medium` | upstream API frozen until Q3 |

## ⚪ Deferred (1)

| # | Title | Priority |
|---|-------|----------|
| [0009](active/0009-india.md) | India feature | `low` |

## 🔵 Implemented — awaiting merge (2)

| # | Title | Priority | PR | Readiness |
|---|-------|----------|----|-----------|
| [0008](active/0008-hotel.md) | Hotel feature | `high` | [#142](https://github.com/o/r/pull/142) |  |
| [0013](active/0013-mike.md) | Mike feature | `high` | [#151](https://github.com/o/r/pull/151) | finalize blocked — needs you |

```mermaid
graph TD
  0001
  0010 --> 0002
  0003
  0004
  0003 --> 0005
  0008 --> 0006
  0007
  0008
  0009
  0013
  0010:::done
  classDef done fill:#d3f9d8;
```

<details><summary>✅🗑️ Archive — done + killed (3)</summary>

| # | Title | Merged |
|---|-------|--------|
| [0012](archive/2026-06-16-0012-lima.md) | Lima feature | 2026-06-16 |
| [0010](archive/2026-06-15-0010-juliet.md) | Juliet feature | 2026-06-15 |
| [0011](archive/2026-06-14-0011-kilo.md) | Kilo feature | 2026-06-14 |

</details>
EOF

rendered="$tmp/out.md"
bash "$SCRIPT" --changes-dir "$tmp" --repo o/r > "$rendered" 2>/dev/null
assert "rendered output matches the golden byte-for-byte" 'diff -u "$golden" "$rendered"'

# idempotence: a second render is byte-identical to the first
rendered2="$tmp/out2.md"
bash "$SCRIPT" --changes-dir "$tmp" --repo o/r > "$rendered2" 2>/dev/null
assert "render is idempotent (re-run is byte-identical)" 'diff -u "$rendered" "$rendered2"'

# PR cell: the docket convention is that pr: holds the FULL URL (#8 Hotel above, exercised by the
# golden). Also cover the bare-number fallback in a focused fixture (renders the same #N link via --repo).
bare="$(mktemp -d)"; mkdir -p "$bare/active" "$bare/archive"
cat > "$bare/active/0001-bare.md" <<'EOF'
---
id: 1
slug: bare
title: Bare PR
status: implemented
priority: medium
depends_on: []
pr: 77
EOF
bareout="$(bash "$SCRIPT" --changes-dir "$bare" --repo o/r 2>/dev/null)"
assert "pr: full URL renders [#N](url) without double-wrapping (Hotel #8 in the golden)" \
  'grep -qF "[#142](https://github.com/o/r/pull/142)" "$rendered"'
assert "pr: bare number falls back to [#N](built-url) via --repo" \
  'printf "%s" "$bareout" | grep -qF "[#77](https://github.com/o/r/pull/77)"'
rm -rf "$bare"

# --- change 0087: the `## Finalize blocked` cell on the implemented table -----------------------
# Positive and negative in one render: 0013 carries the section, 0008 does not. The golden already
# byte-checks both; these focused asserts name the invariant so a golden re-blessing cannot quietly
# drop it (learnings: guards-are-code).
assert "a marked implemented change renders the finalize-blocked cell" \
  'grep -qF "| finalize blocked — needs you |" "$rendered"'
assert "an unmarked implemented change renders an empty readiness cell" \
  'grep -qF "| [#142](https://github.com/o/r/pull/142) |  |" "$rendered"'
assert "the implemented table carries the Readiness column" \
  'grep -qF "| # | Title | Priority | PR | Readiness |" "$rendered"'

# Digest parity (change 0069's invariant: the digest can never disagree with the board).
digest="$(bash "$SCRIPT" --changes-dir "$tmp" --format digest 2>/dev/null)"
assert "digest reports finalize-blocked for the marked change" \
  'grep -qF "change 13 implemented finalize-blocked mike" <<<"$digest"'
assert "digest reports - for the unmarked implemented change" \
  'grep -qF "change 8 implemented - hotel" <<<"$digest"'

# Non-vacuity: the marker predicate must key on the section, not on status alone. A copy of the
# marked fixture with the section stripped must render the EMPTY cell.
nomark="$(mktemp -d)"; mkdir -p "$nomark/active" "$nomark/archive"
sed '/## Finalize blocked/,$d' "$tmp/active/0013-mike.md" > "$nomark/active/0013-mike.md"
nomarkout="$(bash "$SCRIPT" --changes-dir "$nomark" --repo o/r 2>/dev/null)"
assert "stripping the section drops the cell (predicate is non-vacuous)" \
  '! grep -qF "finalize blocked — needs you" <<<"$nomarkout"'
rm -rf "$nomark"

# --- A PROSE MENTION IS NOT A SECTION (review finding, change 0087) ---------------------------
# `has_section` was `grep -qF` — an unanchored substring match over the whole file — while its
# docblock promised a whole-line match. Change files routinely *mention* these markers inline in
# prose (change 0087's own file mentions both), so every such change false-positived as "blocked".
# Both markers, at the board AND digest level, in one render: 0021 is a `proposed` change with no
# spec that only talks about `## Auto-groom blocked` (must read `needs-brainstorm`), 0022 is an
# `implemented` change that only talks about `## Finalize blocked` (must read as an empty cell).
prose="$(mktemp -d)"; mkdir -p "$prose/active" "$prose/archive"
cat > "$prose/active/0021-quebec.md" <<'EOF'
---
id: 21
slug: quebec
title: Quebec feature
status: proposed
priority: medium
depends_on: []
spec:
---

## Design

- A stub the groomer abstains on gets a dated `## Auto-groom blocked` body section so the
  abstention is self-describing at the change.
EOF
cat > "$prose/active/0022-romeo.md" <<'EOF'
---
id: 22
slug: romeo
title: Romeo feature
status: implemented
priority: medium
depends_on: []
pr: https://github.com/o/r/pull/161
---

## Design

- A gate failure is marked with a dated `## Finalize blocked` section mirroring the proven
  `## Auto-groom blocked` marker — presence-encoded, never an eighth status.
EOF
proseout="$(bash "$SCRIPT" --changes-dir "$prose" --repo o/r 2>/dev/null)"
prosedigest="$(bash "$SCRIPT" --changes-dir "$prose" --format digest 2>/dev/null)"
assert "a prose mention of ## Auto-groom blocked does not render the blocked cell" \
  '! grep -qF "auto-groom blocked — needs you" <<<"$proseout"'
assert "the prose-mentioning proposed change still reads needs-brainstorm" \
  'grep -qF "| [0021](active/0021-quebec.md) | Quebec feature | \`medium\` | needs-brainstorm |" <<<"$proseout"'
assert "a prose mention of ## Finalize blocked does not render the blocked cell" \
  '! grep -qF "finalize blocked — needs you" <<<"$proseout"'
assert "the prose-mentioning implemented change renders an empty readiness cell" \
  'grep -qF "| [#161](https://github.com/o/r/pull/161) |  |" <<<"$proseout"'
assert "digest agrees: prose mention is needs-brainstorm, not auto-groom-blocked" \
  'grep -qF "change 21 proposed needs-brainstorm quebec" <<<"$prosedigest"'
assert "digest agrees: prose mention is -, not finalize-blocked" \
  'grep -qF "change 22 implemented - romeo" <<<"$prosedigest"'
rm -rf "$prose"

# --- docket-status inline-surface wiring sentinels (the SKILL is code on main) ---
# Since change 0058 the docket-status Board pass lives in scripts/docket-status.sh, not this
# SKILL — the SKILL only *describes* the inline surface (naming render-board.sh) and delegates to
# the orchestrator. Change 0059 therefore does NOT edit this SKILL; the gated-write wiring
# (board_pass_inline -> board-refresh.sh) is asserted in tests/test_docket_status.sh instead. These
# two sentinels are unchanged from main.
assert "docket-status inline surface names render-board.sh" \
  'grep -qF "/render-board.sh" "$SKILL"'
assert "docket-status keeps the regenerate-don't-3-way-merge rule" \
  'grep -qiF "never 3-way merge" "$SKILL"'

# --- Guard 1: the render-board.sh WRITE sentinel (change 0070) --------------------------------
# THE INVARIANT: render-board.sh's stdout reaches a file through board-refresh.sh and NOTHING else.
# SCOPE — exactly what the sweep below covers, no more: EVERY *.sh under scripts/ (recursively, so
# scripts/lib/ is included) AND every root-level *.sh (install.sh, link-skills.sh,
# migrate-to-docket.sh, sync-agents.sh). None of the root four invokes the renderer TODAY — which is
# why the list is DERIVED FROM A find(1) SWEEP and never hand-maintained (ledger #64: never
# hand-list the call sites of an operation you are gating). tests/ is deliberately NOT swept: this
# battery's own fixtures are full of the anti-pattern. board-refresh.sh — the one gated writer
# (change 0059: render to temp -> chmod -> rename) — is the ONLY allowlisted script.
#
# WHY TWO GUARDS: Guard 1 is TOKEN-SCOPED and TARGET-BLIND; REDIRECT_RE (further down) is WHOLE-FILE
# and TARGET-KEYED; neither subsumes the other. The COMPLEMENTARITY block below REDIRECT_RE's
# definition is the CANONICAL statement of that split and PROVES it by mutation in both directions,
# against the real regex and the real function. It is not restated here.
#
# Guard 1 prohibits the WRITE instead of recognizing the write TARGET — it never asks WHERE you are
# writing — so all of these die identically and none is a filename match (each one's mechanism is in
# the stage named beside it; every one really writes the file, stub-renderer verified):
#     >"$d/BOARD.md"                 no space — the IDIOMATIC shell form REDIRECT_RE cannot see
#     >> / >|                        append and clobber-force
#     > "$mw/$rel"                   VARIABLE target: the literal "BOARD.md" sits nowhere near the
#                                    redirect, so NO regex keyed on /BOARD\.md reaches it AT ANY
#                                    WIDTH — this is what forecloses "just widen REDIRECT_RE"
#     &> / &>> / >&f                 the merged-output forms (stage 3)
#     | cat > "$f" / |& cat > "$f"   a redirect anywhere in the PIPELINE the renderer feeds: the
#                                    pipeline IS its stdout (stages 4, 6)
#     out=$(render-board.sh …)       CAPTURE-THEN-WRITE: stdout parked in a VARIABLE and written by a
#     printf '%s' "${out:-}" > "$f"  LATER statement, so the invocation token carries no redirect at
#                                    all (stage 8 — the shape TWO live-tree probes found GREEN in the
#                                    very script this guard protects, and the most realistic
#                                    regression here: docket-status.sh ALREADY captures the digest
#                                    into `out` and ALREADY holds the board path in a variable)
#
# WHAT IT CHECKS, WITHOUT OVERCLAIM. Two source-syntax questions per file; clean only if both say no:
#   Q1 (THE TOKEN, stages 6-7). For each render-board.sh invocation TOKEN — the invocation AND THE
#      PIPELINE IT FEEDS, cut at `;`, `&` or `||` — is there a surviving `>` once true fd dups are
#      erased?
#   Q2 (THE TAINT, stage 8). For each VARIABLE whose value comes from a render-board.sh command
#      substitution — the stdout, ONE HOP on — does ANY statement in the file carry that variable's
#      value into a file-directed redirect (the SAME `>`-survives test as Q1, over the SAME
#      normalized source, so the two cannot drift)?
# Both are SOURCE-SYNTAX properties, not filesystem facts. Three consequences, all load-bearing:
#   * Q2 IS ONE HOP, NOT A DATAFLOW ANALYSIS. It follows a capture into a variable and stops: a
#     function parameter, a second variable or an `eval` loses the taint. DISCLOSED, NOT CHASED —
#     (II)(g) states the residual bound exactly.
#   * Q2 IS PER-FILE AND SCOPE-BLIND ON PURPOSE — a variable tainted in one function and redirected in
#     another is a violation here, though `local` would have made them different variables. That
#     OVER-approximates in the FAIL-SAFE direction; scope-awareness means parsing function bodies, the
#     same "rebuild a bash parser inside a test sentinel" trap stage 1 names. It IS per-VARIABLE: a
#     DIFFERENT variable redirected in a file that also captures the renderer is GREEN (rows 20b, 20e).
#   * IT IS BLIND TO A WRITE CROSSING A STATEMENT BOUNDARY WITH NO VARIABLE CARRYING THE BYTES — a
#     brace group, a wrapper function. Both really write BOARD.md and both are GREEN here; REDIRECT_RE
#     catches them. DO NOT WIDEN THE TOKEN TO CHASE THEM — see gap (I)(0), which spells out the shapes
#     and the reason.
#
# PIPELINE ORDER IS LOAD-BEARING:
#   1. STRIP COMMENTS FIRST — whole-line ones, THEN trailing ones. Prose that merely NAMES the script
#      is not an invocation: render-adr-index.sh, render-change-links.sh and render-board.sh itself
#      mention it in comments ONLY, so this is what keeps them out of the scan. The trailing strip (a
#      `#` preceded by whitespace, to end of line) is equally load-bearing: without it,
#      `render-board.sh ... 2>&2  # digest -> stdout only` keeps a bare `>` from the comment's ARROW
#      and the guard reddens on a LEGITIMATE call — and this codebase's comments are full of `->`.
#      TRADE-OFF, DISCLOSED: THE SCAN IS LEXER-NAIVE ABOUT QUOTES. It honours `#`, `;`, `&` and `||`
#      as metacharacters wherever they appear, including inside a QUOTED ARGUMENT where bash treats
#      them as inert text; each truncates the token early and can hide a redirect after it. These
#      really write the file and all stay GREEN (stub-renderer verified):
#          render-board.sh --repo "a #b" > "$out"   (this strip eats from the ` #` to end of line)
#          render-board.sh --repo "a;b"  > "$out"   (stage 6 cuts the token at the `;`)
#          render-board.sh --repo "a&b"  > "$out"   (stage 6 cuts the token at the `&`)
#          render-board.sh --repo "a||b" > "$out"   (stage 4 rewrites `||` to `;`, then 6 cuts)
#      The fix is a quote-aware lexer — a bash parser, out of all proportion to a test sentinel — and
#      the exposure is narrow: render-board.sh takes only --changes-dir, --repo and --format, none of
#      which carries `#`, `;`, `&` or `|` in its value; the strip, in exchange, fixes a false positive
#      that is REAL TODAY. The honest answer to all of it is the deferred filesystem-effect test.
#   2. ONLY THEN JOIN backslash-continuations. The tokenizer is line-oriented, so a redirect parked on
#      a continuation line hands it a first-line token with no `>` — a clean pass. (REDIRECT_RE
#      survives that shape by a different mechanism: it flattens the file with `tr`.)
#      COMMENTS MUST BE STRIPPED BEFORE THE JOIN — the obvious order is EXPLOITABLE. Bash comments are
#      PHYSICAL-LINE scoped, so a trailing backslash continues NOTHING. In
#          # regenerate the board \
#          "$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" > "$out"
#      line 2 REALLY EXECUTES and REALLY WRITES (stub-renderer verified). Join-first folds it INTO the
#      comment and the comment drop deletes BOTH — the write is laundered, the guard passes clean.
#      Strip-first deletes only the comment and leaves the live invocation standing (row 11b pins it).
#      FAIL-SAFE SIDE EFFECT of this order: dropping comments first lets the join reach ACROSS one, so
#          "$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" \
#          # not actually a continuation
#            > "$out"
#      — where bash ends the command AT the comment, so the `> "$out"` line never receives the
#      renderer's output — is called RED. A FALSE POSITIVE (gap III(f)); the error direction of the
#      ordering is thus fail-safe: it can only ADD an alarm, never hide a write.
#   3. NORMALIZE `&>`/`&>>` TO `>` BEFORE tokenizing. Both are WRITES (stdout+stderr merged into the
#      file), but the tokenizer cuts on `&` (stage 6), so the `&` of `&>` would END the token before
#      its `>` and the write would vanish.
#   4. NORMALIZE THE TWO `|`-COMPOUNDS — each smuggles an `&` (or an extra `|`) into a position that
#      means the OPPOSITE of what a raw scan infers:
#        - `|&` IS A PIPELINE (`2>&1 |`) and must STAY INSIDE the token: rewrite to `|`. Left alone
#          its `&` ends the token at stage 6 and every redirect downstream vanishes —
#          `render-board.sh ... |& cat > "$d/BOARD.md"` really writes the board and was GREEN until
#          this stage existed (stub-renderer verified).
#        - `||` STARTS A NEW COMMAND and must CUT the token: rewrite to `;`. Left alone, `|` does not
#          cut (stage 6) and the OR-branch's redirect is read as the renderer's —
#          `out="$(render-board.sh ... 2>&2)" || echo failed > "$log"` writes nothing of the
#          renderer's yet went RED, and a guard that reddens on an error-handling idiom gets deleted.
#      Both are pinned by battery rows. Each must precede the tokenizer; the `|&` rewrite's position
#      relative to the fd-dup erasure is immaterial (checked both ways — the erasure preserves its
#      right boundary).
#   5. ERASE ONLY TRUE fd DUPS, still before tokenizing, keyed on THE TARGET BEING A WHOLE, REAL fd —
#      not on the operator, and not on the target's PREFIX. `>&2`, `2>&1`, `>&-` dup/close a
#      descriptor (harmless); `>&"$out"` and `>& file` are WRITES sharing the same `>&` spelling. So
#      the erasure demands a digit run or `-` after the `&` AND A RIGHT BOUNDARY:
#      `[0-9]*>&([0-9]+|-)($|[[:space:];&|)}"])`. Both halves are bugs already paid for:
#        * without the digit/`-` requirement (`[0-9-]*`), the class matches ZERO characters and erases
#          `>&"$out"` outright, laundering a file write into nothing.
#        * without the RIGHT BOUNDARY it is a PREFIX match: bash reads `>&word` with a word that is
#          not a valid fd as a merged WRITE, so `>&2board.md` really creates the file `2board.md` —
#          yet an unanchored `>&([0-9]+|-)` eats the `>&2` and leaves a bare `board.md` with no `>`
#          left to find (stub-renderer verified). `|` is IN the boundary class, so `2>&1| head` (no
#          space) is still erased whole — row 17b pins it.
#      Erasing true dups also keeps the `&` of `2>&2` from CUTTING the token mid-redirect and leaving
#      a dangling `>` that fires on the codebase's CORRECT invocation.
#   6. TOKENIZE PER INVOCATION, CUTTING AT `;` AND `&` — NEVER AT A BARE `|`:
#        - PER INVOCATION, not per line: a logical line carrying a clean call beside a rogue one must
#          not be whitewashed by the clean one (ledger #64). `;` and `&` begin genuinely NEW commands,
#          and so does `||`, which stage 4 has already rewritten INTO a `;`.
#        - A BARE `|` DOES NOT END THE TOKEN, because A PIPELINE *IS* THE RENDERER'S STDOUT: whatever
#          is downstream is where the bytes go, so a redirect anywhere in it is a redirect OF the
#          renderer. An earlier cut tokenized on `[^;&|]*` and read
#              "$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" | cat > "$d/BOARD.md"
#          as an invocation ENDING at the `|`, with no `>` left — GREEN, on a line that really writes
#          BOARD.md. That is the pre-0059 anti-pattern wearing a `| cat`.
#      The invocation is recognized at a WORD BOUNDARY — `(^|[^-[:alnum:]])render-board\.sh` — not at
#      a leading `/`: a PATH-resolved `render-board.sh ... > "$out"` writes the file just as surely,
#      and keying on the slash would miss it. The boundary still excludes a merely similar name
#      (`my-render-board.sh` is not this script).
#   7. ANY surviving `>` in a token is a file-directed redirect => violation. This rejects even a
#      stderr-to-file form (`2>/dev/null`) — deliberately conservative: the right way to route this
#      renderer's stderr is the fd dup already in use (`2>&2`), and a guard that permits SOME writes
#      is a guard whose next author must relitigate which. THE SAME RULE AND THE SAME CONSERVATISM
#      APPLY TO Q2's STATEMENTS; gap (III)(h) discloses the identical false positive on the taint
#      side, rather than exempting `>/dev/null` on one half of the guard and not the other.
#   8. TAINT — FOLLOW THE STDOUT ONE HOP INTO A VARIABLE, THEN RE-ASK STAGE 7 THERE. Stages 6-7 read
#      the INVOCATION, but the stdout need not leave through it: it can be CAPTURED and written by a
#      LATER statement, and then the invocation token carries no `>` at all. TWO LIVE-TREE PROBES, not
#      imagination, shaped this stage. The first injected
#          if ! out="$("$SCRIPTS_DIR"/render-board.sh … 2>&2)"; printf "%s" "$out" >"$mw/$rel"; then
#      into the real scripts/docket-status.sh: the suite went GREEN while the board was really being
#      written. BOTH shipped guards missed it — stages 6-7 because the token is cut at the `;`,
#      REDIRECT_RE because there is no whitespace-bounded ` > ` and no literal `/BOARD.md` (the target
#      is the variable `$rel`). The SECOND probe was run against THE FIRST CUT OF THIS STAGE and came
#      back GREEN TOO: one character of the write changed —
#          printf "%s" "${out:-}" >"$mw/$rel"      # ${out:-}, not $out
#      — walks past a use pattern that only matched bare `$out` and `${out}`. That spelling is not
#      exotic: `${var:-}` IS THE HOUSE IDIOM OF THE VERY FILE THIS GUARD PROTECTS (14 uses in
#      scripts/docket-status.sh). A guard that is green on the most realistic regression in the file
#      it protects, written in that file's own idiom, is decoration (ledger #64). Hence BOTH patterns
#      below are keyed on the SHAPE of the syntax, never on a list of spellings someone thought of.
#      TWO SUB-STEPS, over the SAME normalized source stages 1-5 produced:
#        (a) NAME THE TAINTED VARIABLES: `NAME=` or `NAME+=`, an optional `(` (array capture), an
#            optional quote, then a COMMAND SUBSTITUTION — `$(`… or a BACKTICK… — reaching
#            `render-board.sh` before it closes. That covers `out="$(…)"`, `out=$(…)`, ``out=`…` ``,
#            `out=($(…))`, `out+=$(…)`, the `local`/`readonly`/`export` forms (they carry `NAME=`
#            inside them), the `if ! out="$(…)"` form the real script uses, and a capture THROUGH A
#            PIPELINE (`out="$(render-board.sh … | tail -n +2)"` — the pipeline is inside the
#            substitution, so it needs no special case). Rows 19-19l pin every one. The honest bound
#            is "a capture spelled `NAME=`/`NAME+=` with the substitution FIRST in the value", NOT
#            "every spelling" — see (II)(g) for what escapes.
#        (b) RE-ASK STAGE 7 ON EVERY STATEMENT CARRYING THAT NAME'S VALUE. Cut the normalized source
#            into statements with the SAME `[^;&]*` tokenizer stage 6 uses, keep the ones that USE the
#            variable, apply the SAME rule: a surviving `>` is a write. The use pattern is
#            `[$][{]?NAME` followed by a NON-IDENTIFIER character (or end of line) — the name after an
#            optional brace, required to END there. Being shape-based, it catches EVERY parameter
#            expansion of the tainted name in one stroke: `$out`, `${out}`, `${out:-}`,
#            `${out:-default}`, `${out//x/y}`, `${out:0:99}`, `${out[@]}`, `${out^^}`, `${out%.md}` —
#            in each, the operator following the name IS the non-identifier character the pattern
#            demands. The name-END requirement is what stops `$outfile`/`${outfile}` inheriting
#            `$out`'s taint (row 20d). NOT covered, by design: `${!out}` indirection, and `${#out}`
#            (a LENGTH — it carries no board bytes, so covering it would be a false positive).
#            Sharing the tokenizer and the `>` rule with Q1 is the point: `printf … >f`, `echo … >>f`,
#            `>|f`, `cat <<< "$out" > f`, `&>`/`>&file` all die by machinery that already normalizes
#            and erases, so THE TWO PATHS CANNOT DRIFT APART.
#      Its bound is Q2's bound: ONE HOP, PER-FILE, SCOPE-BLIND — (II)(g).
#
# KNOWN, ACCEPTED GAPS. Two kinds, and the difference matters:
#
# (I) NOT A GAP OF THE PAIR — Guard 1 misses it, REDIRECT_RE covers it:
#   0. THE NO-VARIABLE STATEMENT-BOUNDARY CLASS. Both really write BOARD.md (stub-renderer verified):
#          { "$SCRIPTS_DIR"/render-board.sh --changes-dir "$d"; } > "$d/BOARD.md"
#          board(){ "$SCRIPTS_DIR"/render-board.sh --changes-dir "$d"; }; board > "$d/BOARD.md"
#      The bytes cross the statement boundary with NO variable carrying them, so Q1's token ends at
#      the `;` and stage 8's taint has nothing to follow: Guard 1 is GREEN on both, REDIRECT_RE is RED
#      on both, and the COMPLEMENTARITY block asserts both halves. Widening Q1's token to chase them
#      would make it a whole-file scan — REDIRECT_RE rebuilt worse, inside a function that must also
#      stay narrow enough not to redden on the codebase's real calls. Listed here so nobody "fixes"
#      Guard 1 that way: the fix already exists, and it is the other guard.
#      (CAPTURE-THEN-WRITE WAS THE THIRD MEMBER AND IS NO LONGER IN IT. A live-tree probe found it
#      GREEN under BOTH guards — REDIRECT_RE only ever caught the *literal* `> "$d/BOARD.md"`
#      spelling, never the variable-target one a real regression in docket-status.sh would use. Stage
#      8 now catches it in every parameter-expansion spelling of the USE and every `NAME=`/`NAME+=`
#      spelling of the CAPTURE — that is the bound stated in 8(a)-(b) and (II)(g), NOT a claim of
#      totality. The COMPLEMENTARITY block scores the literal spelling BOTH-RED and the
#      variable-target one Guard-1-only.)
#
# (II) GENUINE GAPS OF THE PAIR — real writes NEITHER a token scan nor a target-keyed regex reliably
#      sees. Disclosed, not fixed; they share one answer (below):
#   a. A PIPELINE MEMBER THAT WRITES WITHOUT A `>`: `render-board.sh ... | tee f`. The pipeline is
#      INSIDE the token, so any redirect in it dies — but `tee` needs no redirect. Catching it means
#      knowing WHICH COMMANDS WRITE, i.e. hand-maintaining a list of writers: the same anti-pattern
#      (ledger #64) as hand-listing call sites. Out of reach on purpose.
#   b. fd INDIRECTION — a redirect OPENED ON A PRIOR LINE, in two shapes:
#        - `exec 3>"$out"`, then `render-board.sh ... >&3`: the file IS written, and the guard stays
#          green because `>&3` is — syntactically, locally, correctly — an fd dup.
#        - `exec > "$out"`, then a BARE `render-board.sh ...`: the token carries no `>` whatsoever, so
#          no tightening of stages 3-7 could catch it; the write lives on the earlier `exec` line.
#      A scan of the invocation cannot know what an fd was opened to, or that stdout was rebound out
#      from under it: interpretation, not grep.
#   c. THE REDIRECT OPERATOR ARRIVING BY EXPANSION OR eval:
#          r='>'; eval "\"\$SCRIPTS_DIR\"/render-board.sh --changes-dir \"\$d\" $r \"\$out\""
#      writes the file, and no `>` exists on the invocation to find. Unreachable by construction: the
#      redirect is not syntax until runtime.
#   d. A `;`, `&`, `||` OR `#` INSIDE A QUOTED ARGUMENT, cutting the token short of a real redirect
#      (stage 1's lexer-naivety trade-off). Same root cause as (c): the scan reads text, not a parse
#      tree.
#   g. STAGE 8's RESIDUAL BOUND, stated exactly so it is not mistaken for coverage. The taint now
#      follows the stdout through EVERY parameter-expansion spelling of a tainted variable's USE and
#      every `NAME=`/`NAME+=` spelling of its CAPTURE. What it still cannot follow is a SECOND HOP,
#      plus two captures that are not assignments at all. Every one below really writes the board;
#      every one is GREEN:
#          out=$(render-board.sh …); emit(){ printf '%s' "$1" > "$f"; }; emit "$out"
#              — the value leaves through a FUNCTION PARAMETER; `$1` is not `$out`.
#          out=$(render-board.sh …); b="$out"; printf '%s' "$b" > "$f"
#              — COPIED into an untainted second variable.
#          out=$(render-board.sh …); eval "printf '%s' \"\$out\" > \"\$f\""
#              — the use and the redirect exist only at runtime; cf. (c).
#          mapfile -t out < <(render-board.sh …)   /   read -r out < <(render-board.sh …)
#              — the capture is a COMMAND'S SIDE EFFECT, not an assignment: there is no `NAME=` for
#                8(a) to name, so the variable is never tainted at all.
#          out="prefix$(render-board.sh …)"
#              — the substitution is not FIRST in the value, so 8(a)'s `NAME=`-anchored pattern does
#                not reach it. (Anchoring "anywhere in the value" would let the pattern run across an
#                unrelated assignment sharing the logical line — a false positive traded for a shape
#                nothing in this codebase writes.)
#      Closing these means propagating taint through assignments, parameters, expansions and command
#      side effects: a bash dataflow analyzer inside a test sentinel — the same "out of all
#      proportion" call stage 1 makes about a quote-aware lexer, with the same answer (below). What
#      stage 8 DOES buy is the ONE HOP a real regression actually takes — the shape TWO live-tree
#      probes found, in the script the guard protects.
#
# (III) KNOWN FALSE POSITIVES, all FAIL-SAFE (they can only add an alarm, never hide a write), so they
#      are disclosed rather than engineered around:
#   e. A HEREDOC BODY quoting the anti-pattern as PROSE: `cat > "$f" <<EOS` … `render-board.sh ... >
#      "$d/BOARD.md"` … `EOS` writes nothing of the renderer's, but neither guard knows what a heredoc
#      is — Guard 1 reads the body as source and goes RED (so does REDIRECT_RE). No such heredoc
#      exists in the swept scripts today; if one is added, quote it so it does not read as an
#      invocation, or move the example into a comment.
#   f. The join reaching ACROSS a comment that bash would treat as ending the command (stage 2).
#   h. A TAINTED VARIABLE'S STATEMENT CARRYING A NON-FILE `>` — the Q2 twin of stage 7's disclosed
#      conservatism, named so the two halves are treated alike. `printf '%s\n' "$out" | grep -c
#      "^change " 2>/dev/null` writes NONE of the renderer's bytes to a file, yet the `>` of
#      `2>/dev/null` survives the fd-dup erasure (it is no dup — the target is a path) and Q2 reddens,
#      exactly as Q1 reddens on `render-board.sh … 2>/dev/null`. Exempting `>/dev/null` was rejected
#      for the reason stage 7 gives: `/dev/null` is a path like any other, and a guard that permits
#      SOME writes is a guard whose next author must relitigate which. Route the renderer's stderr
#      with the fd dup the codebase already uses (`2>&2`) and neither half fires. (Row 20c is the
#      GREEN counterpart: a tainted variable merely piped into `grep -qc`, with NO redirect at all,
#      is clean — it is the `2>/dev/null` that reddens, not the inspection.)
#
# The answer to the (II) gaps is the filesystem-effect test the design DEFERRED (run the orchestrator
# against a fixture and assert BOARD.md's bytes): syntax-independent but path-dependent, and the ONLY
# thing that can see a `| tee`, an fd opened elsewhere, a redirect conjured by eval, a SECOND taint
# hop, or a quote the scan mis-lexed. THAT IS WHY THE DEFERRED TEST STILL HAS A JOB with both guards
# in place.
#
# The battery below is the substance of this guard (ledger #64: a guard is code — mutation-test it
# before trusting it, or it is decoration). Every evasion above is injected into a fixture and MUST
# turn the guard RED; the controls — the codebase's real invocation among them — MUST keep it GREEN.
# Every row's verdict has been compared against FILESYSTEM TRUTH: each fixture was executed against a
# stub renderer and the target inspected for the renderer's bytes.

# Folds backslash-continuations into logical lines. awk, not sed: BSD sed does not portably treat
# `\n` in an s/// LHS as a newline, so the classic `:a; /\\$/N; s/\\\n//; ta` is a GNU-ism here.
# TWIN (SHIPPED): tests/test_docket_status.sh defines a BYTE-IDENTICAL join_continuations for its flag
# sentinel (Guard 3 of change 0070), which tokenizes the same unit and needed the same fix. The two
# copies are deliberate duplication — each test file is standalone and shares no library, the way each
# defines its own `assert` — so an edit to either must be mirrored in the other.
join_continuations(){
  awk '{ while (sub(/\\$/, "")) { if ((getline nxt) > 0) { $0 = $0 nxt } else { break } } print }' "$1"
}

# normalize_source — STAGES 1-5 of the block above, factored out so that BOTH halves of Guard 1
# (Q1, the invocation token; Q2, the taint) read THE SAME normalized text. This is not tidiness: if
# the taint pass kept its own copy of the `&>` normalization or the fd-dup erasure, the two would
# drift, and a shape closed on one path would silently reopen on the other. ONE NORMALIZER, ONE
# REDIRECT VOCABULARY, TWO QUESTIONS ASKED OF IT.
# Order: comments OUT first (a comment cannot continue across a trailing backslash, so joining first
# would let one SWALLOW a live invocation), THEN join continuations, THEN normalize `&>`/`&>>` to a
# bare `>`, THEN normalize the `|`-compounds (`|&` is a PIPELINE and must stay in the token; `||` is
# a NEW COMMAND and must cut it), THEN erase whole-and-only-whole fd dups.
normalize_source(){
  join_continuations <(
    grep -v '^[[:space:]]*#' "$1" | sed 's/[[:space:]]#.*$//'
  ) \
    | sed 's/&>>\{0,1\}/>/g' \
    | sed 's/|&/|/g; s/||/;/g' \
    | sed -E 's/[0-9]*>&([0-9]+|-)($|[[:space:];&|)}"])/\2/g'
}

# 0 = clean; 1 = the renderer's stdout reaches a file — EITHER on the invocation (or the pipeline it
# feeds), OR one hop later, out of a variable that captured it.
# Q1, THE INVOCATION TOKEN (stages 6-7). The tokenizer's `[^;&]*` cuts at `;` and `&` ONLY — NOT at
# a bare `|`: the pipeline a renderer feeds IS its stdout, so `... | cat > f` must stay INSIDE the
# token (see stage 6; `[^;&|]*` here was a false-GREEN on a real BOARD.md write).
# Q2, THE TAINT (stage 8). A capture (`out="$(render-board.sh …)"`, in any of its spellings) puts no
# `>` on the invocation at all, so Q1 is structurally blind to it — and a LIVE-TREE PROBE proved
# that mattered: injecting `…; printf "%s" "$out" >"$mw/$rel"; …` into the REAL
# scripts/docket-status.sh left this suite GREEN while the board was really written. Q2 names the
# captured variable, then re-asks Q1's "is there a surviving `>`" of every statement that USES it —
# the same tokenizer, over the same normalized text. Word-bounded, so `$outfile` does not inherit
# `$out`'s taint; per-file and scope-blind, which OVER-approximates in the fail-safe direction.
# STILL BOUNDED BY CONSTRUCTION: a write that crosses a statement boundary carrying NO variable
# (`{ ...; } > f`, a wrapper function) is GREEN here and RED under REDIRECT_RE's whole-file scan;
# and a taint that escapes through a function parameter, a second variable or an `eval` is GREEN
# under both. Do not widen the token to chase the first pair — see COMPLEMENTARITY below.
# Diagnostics go to STDOUT so the real scan can name the offending script; the mutation battery
# routes them to /dev/null, since an EXPECTED red row is not an operator-actionable violation.
render_board_write_free(){
  local f="$1" violation=0 inv norm var stmt
  norm="$(normalize_source "$f")"

  # Q1 — the invocation token (and the pipeline it feeds) may carry no surviving `>`.
  while IFS= read -r inv; do
    [ -n "$inv" ] || continue
    echo "  (render-board.sh invocation writes to a file in ${f##*/}: $inv)"
    violation=1
  done < <(
    printf '%s\n' "$norm" \
      | grep -oE '[^;&]*(^|[^-[:alnum:]])render-board\.sh[^;&]*' \
      | grep '>' || true
  )

  # Q2 — no variable that CAPTURED the renderer's stdout may carry it into a redirect. The taint
  # list is DERIVED (`NAME=` + optional quote + `$(` … `render-board.sh` before any `)`), never
  # hand-written; a capture through a pipeline needs no special case because the pipeline lives
  # INSIDE the `$(…)`. The use pattern is word-bounded so `$outfile` never inherits `$out`'s taint.
  while IFS= read -r var; do
    [ -n "$var" ] || continue
    while IFS= read -r stmt; do
      [ -n "$stmt" ] || continue
      echo "  (render-board.sh output captured in \$$var is written to a file in ${f##*/}: $stmt)"
      violation=1
    done < <(
      printf '%s\n' "$norm" \
        | grep -oE "[^;&]*[\$][{]?$var([^[:alnum:]_]|\$)[^;&]*" \
        | grep '>' || true
    )
  done < <(
    printf '%s\n' "$norm" \
      | grep -oE '[A-Za-z_][A-Za-z0-9_]*\+?=\(?["'"'"']?(\$\([^)]*|`[^`]*)render-board\.sh' \
      | sed -E 's/\+?=.*$//' \
      | sort -u
  )
  return "$violation"
}

# --- the mutation battery: each row is a fixture the guard must judge correctly ---
mut="$tmp/mut"; mkdir -p "$mut"

# (1) spaced redirect — the ONLY shape REDIRECT_RE could already see. Establishes the baseline.
printf '%s\n' '#!/usr/bin/env bash' \
  '"$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" > "$d/BOARD.md"' > "$mut/spaced.sh"
assert "guard1 flags a spaced redirect into BOARD.md" \
  '! render_board_write_free "$mut/spaced.sh" >/dev/null'

# (2) NO SPACE — the idiomatic shell redirect, invisible to REDIRECT_RE's [[:space:]]>[[:space:]].
printf '%s\n' '#!/usr/bin/env bash' \
  '"$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" >"$d/BOARD.md"' > "$mut/nospace.sh"
assert "guard1 flags a no-space redirect into BOARD.md" \
  '! render_board_write_free "$mut/nospace.sh" >/dev/null'

# (3) append. Not special-cased: the board is a full regeneration, never an accumulation — but
#     this dies for the same reason everything else does, not because append is uniquely wrong.
printf '%s\n' '#!/usr/bin/env bash' \
  '"$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" >> "$d/BOARD.md"' > "$mut/append.sh"
assert "guard1 flags an append (>>) redirect into BOARD.md" \
  '! render_board_write_free "$mut/append.sh" >/dev/null'

# (4) clobber-force.
printf '%s\n' '#!/usr/bin/env bash' \
  '"$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" >| "$d/BOARD.md"' > "$mut/clobber.sh"
assert "guard1 flags a clobber-force (>|) redirect into BOARD.md" \
  '! render_board_write_free "$mut/clobber.sh" >/dev/null'

# (5) VARIABLE TARGET — the row that forecloses widening. docket-status.sh really does hold the
#     board path in a variable (`local rel="$CHANGES_DIR/BOARD.md"`), so a rogue write can carry
#     the literal "BOARD.md" nowhere near the redirect. Unreachable by any BOARD.md-keyed regex.
printf '%s\n' '#!/usr/bin/env bash' \
  'rel="$CHANGES_DIR/BOARD.md"' \
  '"$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" > "$mw/$rel"' > "$mut/vartarget.sh"
assert "guard1 flags a redirect to a VARIABLE target (no BOARD.md near the >)" \
  '! render_board_write_free "$mut/vartarget.sh" >/dev/null'

# (6) CONTINUATION LINE — the redirect sits on the second physical line. A line-oriented tokenizer
#     sees a first-line token with no `>` and passes it clean. This is why joining precedes
#     tokenizing, and why dropping the old flattened scan is only safe once it does.
printf '%s\n' '#!/usr/bin/env bash' \
  '"$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" \' \
  '  > "$f"' > "$mut/continuation.sh"
assert "guard1 flags a redirect parked on a continuation line" \
  '! render_board_write_free "$mut/continuation.sh" >/dev/null'

# (6b) CONTINUATION + NO SPACE — the union shape the design named as still-live under BOTH old
#      guards (evades the line-oriented tokenizer AND REDIRECT_RE's whitespace requirement).
printf '%s\n' '#!/usr/bin/env bash' \
  '"$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" \' \
  '  >"$mw/$rel"' > "$mut/continuation-nospace.sh"
assert "guard1 flags a no-space redirect on a continuation line (the union evasion)" \
  '! render_board_write_free "$mut/continuation-nospace.sh" >/dev/null'

# (7)-(10) THE MERGED-OUTPUT FORMS. Every one of these REALLY WRITES THE FILE — verified by
#     running each against a stub renderer and stat'ing the target — and every one of them slipped
#     past the guard's first cut, for two independent reasons worth naming separately:
#       - `&>` / `&>>`: the tokenizer's `[^;&|]*` treats the `&` as a command separator, so the
#         token ENDED before the `>` and the write disappeared. Fixed by normalizing to a bare `>`
#         ahead of tokenizing (pipeline stage 3).
#       - `>& file`: the fd-dup erasure keyed on the OPERATOR `>&` rather than on whether the
#         target is a real descriptor, and its `[0-9-]*` happily matched ZERO characters — so
#         `>&"$out"`, a file write, was erased outright. Fixed by REQUIRING a digit run or `-`.
#     These four rows are the reason the erasure regex may never be loosened back to `[0-9-]*`.

# (7) &> — stdout+stderr merged into a FILE.
printf '%s\n' '#!/usr/bin/env bash' \
  '"$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" &> "$out"' > "$mut/amp-redirect.sh"
assert "guard1 flags an &> (merged stdout+stderr) redirect into a file" \
  '! render_board_write_free "$mut/amp-redirect.sh" >/dev/null'

# (8) &>> — the appending twin of the above.
printf '%s\n' '#!/usr/bin/env bash' \
  '"$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" &>> "$out"' > "$mut/amp-append.sh"
assert "guard1 flags an &>> (merged, appending) redirect into a file" \
  '! render_board_write_free "$mut/amp-append.sh" >/dev/null'

# (9) >& file — same merge, the other spelling. Shares its operator with the HARMLESS fd dups
#     (>&2, >&-), which is exactly why the erasure must inspect the TARGET, not the operator.
printf '%s\n' '#!/usr/bin/env bash' \
  '"$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" >& "$out"' > "$mut/gtamp-spaced.sh"
assert "guard1 flags a >& FILE redirect (not an fd dup — the target is a path)" \
  '! render_board_write_free "$mut/gtamp-spaced.sh" >/dev/null'

# (10) >&"$out" — no space. The shape the old `[0-9-]*` erasure literally deleted: it matched the
#      empty string after the `&`, so a file write was rewritten into nothing and passed clean.
printf '%s\n' '#!/usr/bin/env bash' \
  '"$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" >&"$out"' > "$mut/gtamp-nospace.sh"
assert "guard1 flags a no-space >&\"\$out\" FILE redirect (the zero-width fd-dup erasure bug)" \
  '! render_board_write_free "$mut/gtamp-nospace.sh" >/dev/null'

# (10b) >&2board.md — the fd-dup erasure's RIGHT edge. bash reads `>&word` with a word that is not
#      a valid descriptor as a merged WRITE, so this really creates the file `2board.md` (verified
#      against a stub renderer). An erasure anchored only on its LEFT (`>&([0-9]+|-)` with nothing
#      after) is a PREFIX match: it ate the `>&2`, left a bare `board.md`, and the token had no `>`
#      left to find — GREEN on a real write. The boundary in stage 5 is what closes this: the
#      descriptor must END at the match. Rows 12 and 15 are its counterweight — `2>&2` and `>&-`
#      must stay GREEN, so the fix may not simply stop erasing dups.
printf '%s\n' '#!/usr/bin/env bash' \
  '"$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" >&2board.md' > "$mut/gtamp-fdword.sh"
assert "guard1 flags >&2board.md (fd-dup erasure must be right-anchored, not a prefix match)" \
  '! render_board_write_free "$mut/gtamp-fdword.sh" >/dev/null'

# (11) A ROGUE REDIRECT SITTING BEFORE A TRAILING COMMENT. The other half of the trailing-comment
#      fix (see row 14): stripping trailing comments must NOT become a way to launder a real write.
#      The strip removes only what follows the `#`; the redirect precedes it and still dies.
printf '%s\n' '#!/usr/bin/env bash' \
  '"$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" > "$f"  # regenerate the board' \
  > "$mut/rogue-before-comment.sh"
assert "guard1 still flags a rogue redirect that sits BEFORE a trailing comment" \
  '! render_board_write_free "$mut/rogue-before-comment.sh" >/dev/null'

# (11b) THE COMMENT-BACKSLASH LAUNDERING SHAPE — the row that PINS THE PIPELINE ORDER. A comment
#      whose line ends in a backslash, with a real, redirecting invocation beneath it. Bash
#      comments are PHYSICAL-LINE scoped, so the backslash continues NOTHING: line 2 executes and
#      really writes "$out" (verified against a stub renderer — the file appears).
#      Under the original order (JOIN, then drop comments) the join folded line 2 INTO the comment
#      and the comment drop deleted both — a live write vanished and the guard passed clean. This
#      is the whole reason stage 1 strips comments BEFORE stage 2 joins. If a future edit "tidies"
#      the pipeline back into the intuitive join-first order, THIS row is what goes green and tells
#      them.
printf '%s\n' '#!/usr/bin/env bash' \
  '# regenerate the board \' \
  '"$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" > "$out"' > "$mut/comment-backslash.sh"
assert "guard1 flags a live redirect laundered under a comment line ending in a backslash" \
  '! render_board_write_free "$mut/comment-backslash.sh" >/dev/null'

# (11c) BARE, PATH-RESOLVED INVOCATION — no directory part. Every other row spells the call
#      `"$SCRIPTS_DIR"/render-board.sh`, and a tokenizer keyed on `/render-board.sh` reads the
#      LEADING SLASH as mandatory — so a script that puts scripts/ on PATH (or cd's into it) and
#      calls the renderer by bare name evades the guard entirely while writing the file exactly as
#      hard. Stage 6's word boundary `(^|[^-[:alnum:]])` accepts start-of-line and the space here,
#      while still refusing a merely similar name like `my-render-board.sh`.
printf '%s\n' '#!/usr/bin/env bash' \
  'render-board.sh --changes-dir "$d" > "$out"' > "$mut/barepath.sh"
assert "guard1 flags a bare PATH-resolved render-board.sh (no leading slash) with a redirect" \
  '! render_board_write_free "$mut/barepath.sh" >/dev/null'

# (11d)-(11h) THE PIPELINE FAMILY. A pipeline IS the renderer's stdout: whatever sits downstream of
#      the `|` is where the bytes go. The guard's earlier tokenizer (`[^;&|]*`) ENDED the
#      invocation at the `|`, so it never saw the redirect on the far side and called a real
#      BOARD.md write GREEN. Cutting at `;` and `&` but NEVER at a bare `|` (stage 6) keeps the
#      whole pipeline inside the token, and every redirect in it dies like any other. Row 11h is
#      the `|&` spelling of the same idea — it is `2>&1 |`, a PIPELINE, and its `&` used to end the
#      token one character before the write it was carrying. Each row below really writes its
#      target — verified against a stub renderer.

# (11d) THE CRITICAL ROW — a `| cat` between the renderer and the redirect. Nothing else about the
#      write changed: BOARD.md still receives the renderer's bytes.
printf '%s\n' '#!/usr/bin/env bash' \
  '"$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" | cat > "$d/BOARD.md"' \
  > "$mut/pipeline-literal.sh"
assert "guard1 flags a redirect on the FAR SIDE of a pipeline (| cat > BOARD.md)" \
  '! render_board_write_free "$mut/pipeline-literal.sh" >/dev/null'

# (11e) the same, no space before the target — the two evasions composed.
printf '%s\n' '#!/usr/bin/env bash' \
  '"$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" | cat >"$f"' \
  > "$mut/pipeline-nospace.sh"
assert "guard1 flags a no-space redirect on the far side of a pipeline (| cat >\"\$f\")" \
  '! render_board_write_free "$mut/pipeline-nospace.sh" >/dev/null'

# (11f) a longer pipeline, appending. The `>` may sit arbitrarily far downstream; the token runs to
#      the next `;` or `&`, so distance buys nothing.
printf '%s\n' '#!/usr/bin/env bash' \
  '"$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" | sed s/x/y/ >> "$f"' \
  > "$mut/pipeline-append.sh"
assert "guard1 flags an appending redirect further down a pipeline (| sed ... >> \"\$f\")" \
  '! render_board_write_free "$mut/pipeline-append.sh" >/dev/null'

# (11g) a spaced ` > ` into a literal /BOARD.md, split across a continuation. BOTH guards see this
#      one — REDIRECT_RE because it flattens the file, Guard 1 because it joins. An OVERLAP row,
#      kept because the two mechanisms are independent: it must not become the only thing either
#      guard is trusted for (that is what the COMPLEMENTARITY block is for).
printf '%s\n' '#!/usr/bin/env bash' \
  '"$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" \' \
  '  > "$d/BOARD.md"' > "$mut/continuation-literal.sh"
assert "guard1 flags a spaced /BOARD.md redirect parked on a continuation line" \
  '! render_board_write_free "$mut/continuation-literal.sh" >/dev/null'

# (11h) `|&` — THE PIPE-BOTH-STREAMS SPELLING. `cmd |& reader` is bash shorthand for
#      `cmd 2>&1 | reader`: a PIPELINE, so the renderer's stdout flows on to the reader and any
#      redirect the reader carries is a redirect OF the renderer. It really writes BOARD.md
#      (verified against a stub renderer). It was GREEN before stage 4 existed, for a reason that
#      has nothing to do with pipelines: the tokenizer cuts at `&`, and `|&` HAS an `&` — so the
#      token ended one character before the `>` it was carrying. Stage 4 rewrites `|&` to a bare
#      `|`, restoring it to the pipeline it always was. Row 18 is its mirror: `||` also carries a
#      `|`, means the OPPOSITE (a new command), and must CUT the token.
printf '%s\n' '#!/usr/bin/env bash' \
  '"$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" |& cat > "$d/BOARD.md"' \
  > "$mut/pipe-amp.sh"
assert "guard1 flags a redirect behind a |& pipe (|& cat > BOARD.md)" \
  '! render_board_write_free "$mut/pipe-amp.sh" >/dev/null'

# (11i) the same, with a VARIABLE target and an fd dup sitting right against the `|&`. Pins that
#      the fd-dup erasure (stage 5) and the `|&` rewrite (stage 4) compose in either order: the
#      erasure preserves the `|` it uses as a right boundary, so the pipeline survives it.
printf '%s\n' '#!/usr/bin/env bash' \
  '"$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" 2>&1|& cat > "$f"' \
  > "$mut/pipe-amp-fddup.sh"
assert "guard1 flags a |& redirect to a variable target with an adjacent fd dup (2>&1|& cat > \"\$f\")" \
  '! render_board_write_free "$mut/pipe-amp-fddup.sh" >/dev/null'

# (12) FALSE-POSITIVE CONTROL — the codebase's REAL invocation, copied verbatim from
#     scripts/docket-status.sh, fd dup and all. A guard that fires on this is a guard someone
#     disables; this row carries as much weight as every RED row above it.
printf '%s\n' '#!/usr/bin/env bash' \
  'if ! out="$("$SCRIPTS_DIR"/render-board.sh --changes-dir "$cd_dir" --format digest 2>&2)"; then' \
  '  return 1' \
  'fi' > "$mut/fddup.sh"
assert "guard1 stays GREEN on the real 2>&2 --format digest invocation (false-positive control)" \
  'render_board_write_free "$mut/fddup.sh" >/dev/null'

# (13) FALSE-POSITIVE CONTROL — a 2>&1 fd dup beside a COMMENT that spells out the old redirect.
#      Comment-stripping is load-bearing: three scripts name render-board.sh in comments only.
printf '%s\n' '#!/usr/bin/env bash' \
  '# historical (pre-0059): render-board.sh --changes-dir "$d" > "$d/BOARD.md"' \
  'out="$("$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" 2>&1)"' > "$mut/comment.sh"
assert "guard1 stays GREEN on an fd-dup call beside a comment naming the old redirect" \
  'render_board_write_free "$mut/comment.sh" >/dev/null'

# (14) FALSE-POSITIVE CONTROL — a legitimate call carrying a TRAILING comment whose prose contains
#      an ARROW. Whole-line comment stripping does not touch this line, so before the trailing
#      strip the comment's `->` left a `>` inside the token and the guard fired RED on a correct
#      invocation. This codebase's comments are full of `->` arrows; a guard that reddens on one
#      is a guard someone deletes. Row 11 is its mirror: the strip must not launder a real write.
printf '%s\n' '#!/usr/bin/env bash' \
  'out="$("$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" --format digest 2>&2)"  # digest -> stdout only' \
  > "$mut/trailing-comment.sh"
assert "guard1 stays GREEN on a legit call with a trailing '# ... -> ...' comment" \
  'render_board_write_free "$mut/trailing-comment.sh" >/dev/null'

# (15) FALSE-POSITIVE CONTROL — `>&-` closes a descriptor. It is not a write, it shares the `>&`
#      spelling with the file-writing forms in rows 9-10, and the fix must keep telling them
#      apart: the target here is `-`, not a path.
printf '%s\n' '#!/usr/bin/env bash' \
  'out="$("$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" --format digest 2>&2 >&-)"' \
  > "$mut/fdclose.sh"
assert "guard1 stays GREEN on a >&- fd close (a descriptor, not a file)" \
  'render_board_write_free "$mut/fdclose.sh" >/dev/null'

# (16) FALSE-POSITIVE CONTROL — a PIPELINE WITH NO REDIRECT. Rows 11d-11i pull the whole pipeline
#      into the token, and the price of that would be a guard that reddens on every legitimate
#      `render-board.sh | <reader>`. It does not: the rule is still "a surviving `>`", and a
#      pipeline that merely READS the renderer's stdout has none. Piping the digest into a reader
#      is a shape docket-status.sh is entitled to grow.
printf '%s\n' '#!/usr/bin/env bash' \
  'n="$("$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" --format digest | grep -c change)"' \
  > "$mut/pipeline-clean.sh"
assert "guard1 stays GREEN on a pipeline with no redirect (| grep -c)" \
  'render_board_write_free "$mut/pipeline-clean.sh" >/dev/null'

# (17) FALSE-POSITIVE CONTROL — `2>&1 | head`: an fd dup before a pipe. It is erased by stage 5 and
#      the surviving token holds no `>`. NOTE what this row does NOT pin: the SPACE after `2>&1` is
#      already a right boundary, so this fixture stays GREEN even if `|` were dropped from the
#      boundary class. Row 17b is the one that actually pins the `|`.
printf '%s\n' '#!/usr/bin/env bash' \
  'out="$("$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" 2>&1 | head -1)"' \
  > "$mut/pipeline-fddup.sh"
assert "guard1 stays GREEN on a 2>&1 fd dup piped into a reader (2>&1 | head)" \
  'render_board_write_free "$mut/pipeline-fddup.sh" >/dev/null'

# (17b) FALSE-POSITIVE CONTROL — `2>&1| head`, NO SPACE. Here the `|` IS the only right boundary the
#      fd-dup erasure (stage 5) can match on, so this row genuinely pins `|` inside the boundary
#      class `($|[[:space:];&|)}"])`. Drop the `|` from that class and the erasure no longer fires:
#      the `>` of `2>&1` survives into the token and the guard reddens on a call that writes
#      nothing. Row 17's spaced twin cannot detect that mutation; this one can.
printf '%s\n' '#!/usr/bin/env bash' \
  'out="$("$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" 2>&1| head -1)"' \
  > "$mut/pipeline-fddup-nospace.sh"
assert "guard1 stays GREEN on a no-space fd dup against a pipe (2>&1| head — pins | in the boundary class)" \
  'render_board_write_free "$mut/pipeline-fddup-nospace.sh" >/dev/null'

# (18) FALSE-POSITIVE CONTROL — `|| echo failed > "$log"`. THE FALSE POSITIVE THE PIPELINE FIX
#      INTRODUCED: once the tokenizer stopped cutting at `|` (so a real `| cat > f` write could be
#      seen), it also stopped cutting at `||` — and `||` is not a pipeline at all. It starts a NEW
#      COMMAND, whose redirect has nothing to do with the renderer: the `> "$log"` here writes the
#      string "failed", never the renderer's stdout (verified against a stub renderer — no board
#      bytes reach any file). The guard nonetheless read it as a redirect of the renderer and went
#      RED. Stage 4 rewrites `||` to a `;` so the tokenizer cuts there, exactly as it would for any
#      other new command. An error-handling idiom that reddens the guard is how a guard gets
#      deleted; this row is the reason it will not.
printf '%s\n' '#!/usr/bin/env bash' \
  'out="$("$SCRIPTS_DIR"/render-board.sh --changes-dir "$cd_dir" --format digest 2>&2)" || echo failed > "$log"' \
  > "$mut/or-else.sh"
assert "guard1 stays GREEN on a || error branch whose redirect belongs to another command" \
  'render_board_write_free "$mut/or-else.sh" >/dev/null'

# (18b) the same with no whitespace anywhere around the `||`, and the fd dup pressed right against
#      it. Pins that stage 4's rewrite and stage 5's erasure compose in either order.
printf '%s\n' '#!/usr/bin/env bash' \
  'out="$("$SCRIPTS_DIR"/render-board.sh --changes-dir "$cd_dir" 2>&1)"||echo failed > "$log"' \
  > "$mut/or-else-nospace.sh"
assert "guard1 stays GREEN on a no-space || error branch (\"...\"||echo failed > \"\$log\")" \
  'render_board_write_free "$mut/or-else-nospace.sh" >/dev/null'

# (19)-(19f) THE CAPTURE-THEN-WRITE CLASS — STAGE 8's ROWS. Found by a LIVE-TREE PROBE, not by
#      imagination: the shape in row 19 was injected into the REAL scripts/docket-status.sh, the
#      suite said PASS, and the board was really written. BOTH shipped guards were blind — Q1
#      because the invocation token is cut at the `;` and the write lives in the NEXT statement,
#      REDIRECT_RE because there is no whitespace-bounded ` > ` and no literal `/BOARD.md` anywhere
#      near the operator (the target is the variable `$rel`).
#      This is the most realistic regression shape this codebase has, and that is not rhetoric:
#      docket-status.sh ALREADY captures the renderer's stdout into `out` (backlog_pass) and ALREADY
#      holds the board path in a variable (`local rel="$CHANGES_DIR/BOARD.md"`, board_pass_inline).
#      A future edit reaches row 19 by adding ONE statement to a function that already has both
#      halves in scope. Every row below really writes the renderer's bytes to a file — verified by
#      executing the fixture against a stub renderer that prints known bytes and grepping the tree
#      for them. Rows 20-20d are the controls that keep stage 8 from becoming a nuisance.

# (19) THE LIVE-TREE SHAPE, verbatim. Capture in the `if` condition, write in the same condition
#      list, one `;` over. Q1's token ends at that `;`.
printf '%s\n' '#!/usr/bin/env bash' \
  'if ! out="$("$SCRIPTS_DIR"/render-board.sh --changes-dir "$cd_dir" --format digest 2>&2)"; printf "%s" "$out" >"$mw/$rel"; then' \
  '  return 1' \
  'fi' > "$mut/taint-live-probe.sh"
assert "guard1 flags the LIVE-TREE capture-then-write probe (if ! out=\$(...); printf ... >\"\$mw/\$rel\")" \
  '! render_board_write_free "$mut/taint-live-probe.sh" >/dev/null'

# (19b) the same class spread over LINES, with the literal board path. `out=$(…)` unquoted capture,
#      the write several statements later. Distance buys nothing: the taint is per-file.
printf '%s\n' '#!/usr/bin/env bash' \
  'out=$("$SCRIPTS_DIR"/render-board.sh --changes-dir "$d")' \
  'echo "rendered" >&2' \
  'printf '"'"'%s'"'"' "$out" > "$d/BOARD.md"' > "$mut/taint-later-line.sh"
assert "guard1 flags a capture written out on a LATER line (out=\$(...) ... printf > BOARD.md)" \
  '! render_board_write_free "$mut/taint-later-line.sh" >/dev/null'

# (19c) `local out="$(…)"` — the declaration form the real script uses — appended with `>>`. Stage
#      8 keys on `NAME=`, so `local`, `readonly`, `export` and a bare assignment are all one case.
printf '%s\n' '#!/usr/bin/env bash' \
  'board_pass_inline(){' \
  '  local out="$("$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" --format digest 2>&2)"' \
  '  echo "$out" >> "$f"' \
  '}' \
  'board_pass_inline' > "$mut/taint-local-append.sh"
assert "guard1 flags a 'local out=\$(...)' capture appended into a file (echo \"\$out\" >> \"\$f\")" \
  '! render_board_write_free "$mut/taint-local-append.sh" >/dev/null'

# (19d) CAPTURE THROUGH A PIPELINE — the renderer's stdout is filtered before it lands in the
#      variable, and the variable is then written. The pipeline lives INSIDE the `$(…)`, so the
#      taint pattern needs no special case for it; this row is what proves that.
printf '%s\n' '#!/usr/bin/env bash' \
  'out="$("$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" | tail -n +2)"' \
  'printf '"'"'%s'"'"' "$out" > "$f"' > "$mut/taint-pipeline.sh"
assert "guard1 flags a capture THROUGH A PIPELINE that is then written (out=\$(... | tail); printf > f)" \
  '! render_board_write_free "$mut/taint-pipeline.sh" >/dev/null'

# (19e) `cat <<< "$out" > "$f"` — a here-string writer. The `<<<` carries `<`, not `>`, so it does
#      not confuse the redirect test; the write is the `>` that follows. Pins that stage 8 reuses
#      the `>`-survives rule rather than pattern-matching on `printf`/`echo` — a writer allowlist
#      would be the ledger-#64 anti-pattern (hand-listing) all over again.
printf '%s\n' '#!/usr/bin/env bash' \
  'out="$("$SCRIPTS_DIR"/render-board.sh --changes-dir "$d")"' \
  'cat <<< "$out" > "$f"' > "$mut/taint-herestring.sh"
assert "guard1 flags a here-string write of a captured board (cat <<< \"\$out\" > \"\$f\")" \
  '! render_board_write_free "$mut/taint-herestring.sh" >/dev/null'

# (19f) `readonly out=$(…)` + `${out}` brace expansion + `>|` clobber-force + a VARIABLE target.
#      Every axis at once, and none of it is near a literal BOARD.md — REDIRECT_RE cannot see this
#      row AT ANY WIDTH, so it is stage 8 or nothing.
printf '%s\n' '#!/usr/bin/env bash' \
  'readonly out=$("$SCRIPTS_DIR"/render-board.sh --changes-dir "$d")' \
  'printf '"'"'%s'"'"' "${out}" >| "$mw/$rel"' > "$mut/taint-brace-clobber.sh"
assert "guard1 flags a readonly capture written via \${out} with >| to a variable target" \
  '! render_board_write_free "$mut/taint-brace-clobber.sh" >/dev/null'

# (20) FALSE-POSITIVE CONTROL — THE CODEBASE'S REAL CAPTURE AND ITS REAL USE, copied from
#      scripts/docket-status.sh's backlog_pass VERBATIM (not invented: an invented control proves
#      nothing about the code that ships). The digest IS captured into `out` — so `out` is tainted —
#      and the value is then printed to STDOUT, never redirected. This row is the whole reason stage
#      8 tests the STATEMENTS THAT USE the variable rather than the mere existence of a capture: a
#      guard that reddens on backlog_pass is a guard that gets deleted the same day.
printf '%s\n' '#!/usr/bin/env bash' \
  'backlog_pass(){' \
  '  local mw' \
  '  mw="$(docket_metadata_worktree)"' \
  '  local cd_dir="$mw/$CHANGES_DIR"' \
  '  local out' \
  '  if ! out="$("$SCRIPTS_DIR"/render-board.sh --changes-dir "$cd_dir" --format digest 2>&2)"; then' \
  '    echo "docket-status: backlog digest failed; continuing without it" >&2' \
  '    return 0' \
  '  fi' \
  '  [ -n "$out" ] && printf '"'"'%s\n'"'"' "$out"' \
  '  return 0' \
  '}' > "$mut/taint-real-usage.sh"
assert "guard1 stays GREEN on backlog_pass VERBATIM — a real capture printed to stdout, never redirected" \
  'render_board_write_free "$mut/taint-real-usage.sh" >/dev/null'

# (20b) FALSE-POSITIVE CONTROL — A DIFFERENT VARIABLE IS REDIRECTED IN THE SAME FILE. The file DOES
#      capture the renderer (so a per-FILE taint would redden), but the redirect carries `$other`,
#      which never touched render-board.sh. The taint is per-VARIABLE; this row is what pins that.
printf '%s\n' '#!/usr/bin/env bash' \
  'out="$("$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" --format digest 2>&2)"' \
  'printf '"'"'%s\n'"'"' "$out"' \
  'other="x"; printf "%s" "$other" > "$f"' > "$mut/taint-other-var.sh"
assert "guard1 stays GREEN when a DIFFERENT variable is redirected beside a clean capture (per-variable taint)" \
  'render_board_write_free "$mut/taint-other-var.sh" >/dev/null'

# (20c) FALSE-POSITIVE CONTROL — a tainted variable READ in a test and a grep, with no redirect
#      anywhere. Inspecting the digest is exactly what docket-status.sh is entitled to grow; only a
#      `>` is a write.
printf '%s\n' '#!/usr/bin/env bash' \
  'out="$("$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" --format digest 2>&2)"' \
  'if [ -n "$out" ] && printf "%s" "$out" | grep -qc "^change "; then echo yes; fi' \
  > "$mut/taint-compare-only.sh"
assert "guard1 stays GREEN on a tainted variable merely compared and grepped (no redirect)" \
  'render_board_write_free "$mut/taint-compare-only.sh" >/dev/null'

# (20d) FALSE-POSITIVE CONTROL — THE NAME-PREFIX TRAP. `out` is tainted; `outfile` is a DIFFERENT
#      variable that merely starts with the same letters, and redirecting IT is innocent. Without a
#      word boundary after the name, `$outfile` would inherit `$out`'s taint and this correct script
#      would go red. Drop the `([^[:alnum:]_]|$)` from the taint-use pattern and this row tells you.
printf '%s\n' '#!/usr/bin/env bash' \
  'out="$("$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" --format digest 2>&2)"' \
  'outfile="$d/log.txt"' \
  'printf "%s" "$outfile" > "$f"' > "$mut/taint-prefix-var.sh"
assert "guard1 stays GREEN when \$outfile (a name-prefix of the tainted \$out) is redirected" \
  'render_board_write_free "$mut/taint-prefix-var.sh" >/dev/null'

# (19g)-(19l) THE SPELLING BATTERY FOR STAGE 8 — THE SECOND LIVE-TREE PROBE'S ROWS. The first cut of
#      stage 8 matched only a bare `$out` / `${out}` USE and only a `$(…)` CAPTURE, so it was a list
#      of spellings, not a rule about shape — and SIX one-hop spellings walked straight past it while
#      really writing the board. Each row below was executed against a stub renderer printing known
#      bytes, and those bytes were found in a file afterwards; each was GREEN under the first cut.
#      They are the reason the patterns are keyed on syntax SHAPE (`[$][{]?NAME` + name-END for the
#      use; `NAME=`/`NAME+=` + optional `(` + optional quote + `$(`-or-backtick for the capture).

# (19g) `${out:-}` — THE DEFAULT-EXPANSION WRITE, AND THE ROW THAT MATTERS MOST: `${var:-}` IS THE
#      HOUSE IDIOM OF THE VERY FILE THIS GUARD PROTECTS. scripts/docket-status.sh spells parameter
#      expansions `${VAR:-}` FOURTEEN times (`grep -c '\${[A-Za-z_][A-Za-z0-9_]*:-'`), so this is not
#      an exotic evasion — it is what a docket-status regression would look like written IN THE
#      FILE'S OWN STYLE. A live-tree probe proved it: the guard's own probe shape, with `$out` changed
#      to `${out:-}` (ONE character), left the suite GREEN while the board was really written.
printf '%s\n' '#!/usr/bin/env bash' \
  'out="$("$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" --format digest 2>&2)"' \
  'printf "%s" "${out:-}" >"$mw/$rel"' > "$mut/taint-default-expansion.sh"
assert "guard1 flags a \${out:-} default-expansion write (the guarded file's OWN \${var:-} idiom)" \
  '! render_board_write_free "$mut/taint-default-expansion.sh" >/dev/null'

# (19h) BACKTICK CAPTURE. The older command-substitution spelling; bash still honours it, and a
#      `$(`-only capture pattern never sees it.
printf '%s\n' '#!/usr/bin/env bash' \
  'out=`"$SCRIPTS_DIR"/render-board.sh --changes-dir "$d"`' \
  'printf '"'"'%s'"'"' "$out" > "$f"' > "$mut/taint-backtick.sh"
assert "guard1 flags a BACKTICK capture that is then written (out=\`render-board.sh …\`)" \
  '! render_board_write_free "$mut/taint-backtick.sh" >/dev/null'

# (19i) ARRAY CAPTURE — `out=($(…))`, written back out with `"${out[@]}"`. Evades a capture pattern
#      that demands the `$(` sit immediately after the `=`, AND a use pattern that demands the name be
#      followed by `}` (here it is followed by `[`).
printf '%s\n' '#!/usr/bin/env bash' \
  'out=($("$SCRIPTS_DIR"/render-board.sh --changes-dir "$d"))' \
  'printf '"'"'%s\n'"'"' "${out[@]}" > "$f"' > "$mut/taint-array.sh"
assert "guard1 flags an ARRAY capture written via \${out[@]} (out=(\$(…)))" \
  '! render_board_write_free "$mut/taint-array.sh" >/dev/null'

# (19j) `out+=$(…)` — APPEND-ASSIGNMENT capture. `NAME=` alone does not match `NAME+=`.
printf '%s\n' '#!/usr/bin/env bash' \
  'out=""' \
  'out+=$("$SCRIPTS_DIR"/render-board.sh --changes-dir "$d")' \
  'printf '"'"'%s'"'"' "$out" > "$f"' > "$mut/taint-append-assign.sh"
assert "guard1 flags an append-assignment capture (out+=\$(…)) that is then written" \
  '! render_board_write_free "$mut/taint-append-assign.sh" >/dev/null'

# (19k) `${out//x/y}` — SUBSTITUTION EXPANSION. The bytes are still the renderer's; only some of them
#      are rewritten on the way to the file.
printf '%s\n' '#!/usr/bin/env bash' \
  'out="$("$SCRIPTS_DIR"/render-board.sh --changes-dir "$d")"' \
  'printf '"'"'%s'"'"' "${out//proposed/done}" > "$f"' > "$mut/taint-substitution.sh"
assert "guard1 flags a \${out//x/y} substitution-expansion write" \
  '! render_board_write_free "$mut/taint-substitution.sh" >/dev/null'

# (19l) `${out:0:99}` — SLICE EXPANSION, with `>|` to a variable target for good measure. A truncated
#      board is still a written board.
printf '%s\n' '#!/usr/bin/env bash' \
  'out="$("$SCRIPTS_DIR"/render-board.sh --changes-dir "$d")"' \
  'printf '"'"'%s'"'"' "${out:0:99}" >| "$mw/$rel"' > "$mut/taint-slice.sh"
assert "guard1 flags a \${out:0:99} slice write (>| to a variable target)" \
  '! render_board_write_free "$mut/taint-slice.sh" >/dev/null'

# (20e) FALSE-POSITIVE CONTROL — THE SAME EXPANSION STYLE, A DIFFERENT VARIABLE. Row 20b pins the
#      per-variable taint for a bare `$other`; this one pins it for the `${other:-}` spelling that
#      row 19g made the guard sensitive to. Widening the USE pattern must not turn "any `${…:-}`
#      redirected in a file that captures the renderer" into a violation: the bytes here never touched
#      render-board.sh (verified — the stub's bytes reach no file).
printf '%s\n' '#!/usr/bin/env bash' \
  'out="$("$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" --format digest 2>&2)"' \
  'printf '"'"'%s\n'"'"' "$out"' \
  'other="x"; printf "%s" "${other:-}" > "$f"' > "$mut/taint-other-var-default.sh"
assert "guard1 stays GREEN when a DIFFERENT variable is written in the \${other:-} style (per-variable taint)" \
  'render_board_write_free "$mut/taint-other-var-default.sh" >/dev/null'

# (20f) FALSE-POSITIVE CONTROL — THE NAME-PREFIX TRAP, BRACED. Row 20d pins `$outfile`; the use
#      pattern now accepts an optional `{` before the name, so `${outfile}` is the spelling that
#      widening could plausibly have broken. The name must still END at the match.
printf '%s\n' '#!/usr/bin/env bash' \
  'out="$("$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" --format digest 2>&2)"' \
  'outfile="$d/log.txt"' \
  'printf "%s" "${outfile}" > "$f"' > "$mut/taint-prefix-var-braced.sh"
assert "guard1 stays GREEN when \${outfile} (BRACED name-prefix of the tainted \$out) is redirected" \
  'render_board_write_free "$mut/taint-prefix-var-braced.sh" >/dev/null'

# --- the real scan: every *.sh under scripts/ AND every root-level *.sh, EXCEPT the one allowlisted
#     writer. The root four (install.sh, link-skills.sh, migrate-to-docket.sh, sync-agents.sh) do not
#     invoke the renderer today — they are swept anyway, because the point of a DERIVED sweep is that
#     it keeps holding when that changes. tests/ is NOT swept: this battery's fixtures live there and
#     every one of them is the anti-pattern.
guard1_violation=0
scanned=0
while IFS= read -r s; do
  [ "${s##*/}" = "board-refresh.sh" ] && continue
  scanned=$((scanned + 1))
  render_board_write_free "$s" || guard1_violation=1
done < <( { find "$REPO/scripts" -name '*.sh' -type f; find "$REPO" -maxdepth 1 -name '*.sh' -type f; } | sort )
assert "no script under scripts/ or the repo root (except board-refresh.sh) writes render-board.sh's stdout to a file" \
  '[ "$guard1_violation" -eq 0 ]'

# Anti-vacuity: a scan over zero files passes for the wrong reason. Assert the sweep actually saw
# the tree, and that the allowlisted writer it skips really exists (a rename must not silently
# turn the allowlist into a no-op that hides the real writer).
assert "the write scan is not vacuous (it swept the scripts tree)" '[ "$scanned" -ge 10 ]'
assert "the sweep reaches the ROOT-LEVEL scripts too (not just scripts/)" \
  '[ -f "$REPO/install.sh" ] && [ -f "$REPO/sync-agents.sh" ] && [ "$scanned" -ge 20 ]'
assert "the allowlisted writer scripts/board-refresh.sh exists" \
  '[ -f "$REPO/scripts/board-refresh.sh" ]'

# --- Guard 2: REDIRECT_RE — the WHOLE-FILE, TARGET-KEYED sentinel (re-derived by change 0070) --
# It scans TWO domains and it is load-bearing in BOTH — do not narrow it to one:
#   1. skills/*/SKILL.md PROSE — no skill body may show the pre-0059 anti-pattern
#      `render-board.sh ... > .../BOARD.md`.
#   2. scripts/docket-status.sh SHELL — kept from change 0069, and NOT retired by 0070. Guard 1
#      (the write sentinel above) is the invocation token + the pipeline it feeds + a one-hop
#      taint (stage 8) on a variable that captures the renderer's stdout — so it DOES catch
#      capture-then-write, `out=$(render-board.sh ...); printf ... > f` (see the OVERLAP row in
#      the COMPLEMENTARITY block below). What REDIRECT_RE alone still catches is the NO-VARIABLE
#      statement-boundary class: a write whose bytes cross a STATEMENT boundary carried in no
#      variable at all — `{ render-board.sh ...; } > f` (a brace group) or a wrapper function
#      whose CALL is redirected. Each really writes the board. THIS regex catches them, because
#      it flattens the file with `tr` and spans 200 characters across the boundary Guard 1's
#      token-plus-one-hop-taint stops at. The COMPLEMENTARITY block below PROVES both directions
#      by mutation. Neither guard subsumes the other; deleting either reopens a hole. ONE GUARD,
#      ONE HOLE.
#
# The regex:  render-board\.sh.{0,200}[[:space:]]>[[:space:]].{0,200}/BOARD\.md
# Each element defends a specific real shape in this codebase:
#   - `.{0,200}` bounded any-char gaps (NOT `[^>]*`): the historical redirect's destination is a
#     bracket placeholder `<metadata working tree>/<changes_dir>/BOARD.md`, whose `>` characters
#     and internal spaces a `[^>]*` class could never cross — so `[^>]*` was BLIND to the exact
#     reintroduction shape this sentinel exists to catch. `.` crosses placeholder `>`s freely.
#   - `[[:space:]]>[[:space:]]` whitespace-bounded operator: a real ` > ` redirect in PROSE has a
#     space on both sides, whereas a placeholder's closing bracket is `tree>` / `dir>/` (letter
#     before `>`, or no space after) — so every `<...>` placeholder is structurally excluded.
#   - `/BOARD\.md` (slash required, not bare `BOARD.md`): rejects a flattened markdown blockquote
#     (`\n> ` -> ` > `) that lands a bare "BOARD.md" prose word inside the window. Blockquotes
#     genuinely appear in docket-status and docket-implement-next: a LIVE false-positive class,
#     asserted below rather than merely described in a comment.
#
# WHY IT STAYS NARROW: the shapes that force it to be narrow (spaces around `>`, a literal target)
# are prose hazards, and the shapes a bash script has (`>"$f"`, `>>`, `>|`, `&>`, a VARIABLE
# target) are ones it can never see AT ANY WIDTH. That is Guard 1's job, and Guard 1 does it for
# EVERY script rather than the one that happened to be named. DO NOT widen this regex to chase
# shell forms, and DO NOT delete it as redundant — it is the only net under the NO-VARIABLE
# statement-boundary class.
#
# THE SAME REGEX serves the positive control and both scans, so weakening it (e.g. back to
# `[^>]*`) trips the positive control loudly rather than silently hollowing out a scan.
REDIRECT_RE='render-board\.sh.{0,200}[[:space:]]>[[:space:]].{0,200}/BOARD\.md'

# Positive control ("test the test"): the historical bracket-placeholder redirect that WAS in
# this codebase pre-0059 MUST still be flagged by the guard. If a future edit weakens REDIRECT_RE
# so it can no longer cross placeholder brackets, this assertion fails — not the silent scan.
HISTORICAL_REDIRECT='"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/render-board.sh --changes-dir <metadata working tree>/<changes_dir> > <metadata working tree>/<changes_dir>/BOARD.md'
assert "guard regex flags the historical bracket-placeholder redirect (positive control)" \
  'printf "%s" "$HISTORICAL_REDIRECT" | tr "\n" " " | grep -Eq "$REDIRECT_RE"'

# Negative control (change 0070): a flattened markdown blockquote must NOT trip the regex. This is
# the false-positive class that keeps it narrow — assert it, don't just describe it. The
# `/BOARD\.md` slash requirement is what saves this string: the prose word is a bare "BOARD.md".
BLOCKQUOTE_PROSE='Run render-board.sh to regenerate the board. > Never hand-edit BOARD.md — it is generated.'
assert "guard regex does NOT flag a flattened markdown blockquote (false-positive control)" \
  '! printf "%s" "$BLOCKQUOTE_PROSE" | tr "\n" " " | grep -Eq "$REDIRECT_RE"'

# Negative scan: no CURRENT skill body redirects render-board.sh stdout into BOARD.md.
redirect_found=0
for f in "$REPO"/skills/*/SKILL.md; do
  if tr '\n' ' ' < "$f" | grep -Eq "$REDIRECT_RE"; then
    echo "  (direct render-board.sh -> BOARD.md redirect found in: $f)"
    redirect_found=1
  fi
done
assert "no skills/*/SKILL.md redirects render-board.sh stdout directly into BOARD.md" \
  '[ "$redirect_found" -eq 0 ]'

# Anti-vacuity: the skills glob must be non-empty, or the scan above passes for the wrong reason.
# Uses a glob array (not `ls | wc -l`, which parses `ls` output; and not `set --`, which would
# clobber assert()'s own positional parameters since the assertion runs via `eval "$2"` inside
# that function) so an unmatched glob still fires this assertion instead of silently passing.
skill_md_files=( "$REPO"/skills/*/SKILL.md )
assert "the skills/*/SKILL.md scan is not vacuous" \
  '[ "${#skill_md_files[@]}" -ge 5 ]'

# Same regex, now also over scripts/docket-status.sh (change 0069). That script gained a LEGITIMATE
# render-board.sh call (`--format digest`, read-only, piped into its report), and the sentinel in
# tests/test_docket_status.sh that polices it only tokenizes the FLAG — so
# `render-board.sh --changes-dir "$1" --format digest > "$1/BOARD.md"` satisfies it while writing
# the very file board-refresh.sh is supposed to own. This scan guards the WRITE, not the flag; the
# two catch different holes, so both stay.
status_redirect=0
if tr '\n' ' ' < "$REPO/scripts/docket-status.sh" | grep -Eq "$REDIRECT_RE"; then
  echo "  (direct render-board.sh -> BOARD.md redirect found in: scripts/docket-status.sh)"
  status_redirect=1
fi
assert "scripts/docket-status.sh never redirects render-board.sh stdout into BOARD.md" \
  '[ "$status_redirect" -eq 0 ]'

# Anti-vacuity: the scanned path must exist AND be non-empty, or the scan above passes for the
# wrong reason (a missing OR empty/truncated file makes `tr`'s redirection produce no input, grep
# sees no input, and status_redirect never gets set — a silent, wrong-reason PASS rather than a
# real result). `-s` (not `-f`) so a truncated-to-zero file cannot slip a vacuous pass through.
assert "scripts/docket-status.sh exists (the scan above is not vacuous)" \
  '[ -s "$REPO/scripts/docket-status.sh" ]'

# --- COMPLEMENTARITY: neither guard subsumes the other. PROVEN, not asserted (change 0070) -----
# THIS BLOCK IS THE DECISION RECORD FOR WHY TWO GUARDS SHIP, and it is written as executable
# mutations rather than prose because the prose was WRONG THREE TIMES. The design originally said
# Guard 1 would SUBSUME the REDIRECT_RE scan over scripts/docket-status.sh, and that the scan could
# therefore be retired. MUTATION TESTING DISPROVED THAT. Each guard catches shapes the other
# cannot, so DELETING EITHER ONE REOPENS A HOLE, and neither may be "simplified" into the other.
#
# The two directions, each pinned below against the REAL $REDIRECT_RE (not a copy — a copy drifts
# out of agreement with the thing it is supposed to be proving about) and the REAL shipped
# render_board_write_free (not a reimplementation):
#
#   A. GUARD 1 RED / REDIRECT_RE GREEN — target-blindness. REDIRECT_RE is keyed on a literal
#      `/BOARD.md` sitting a whitespace-bounded ` > ` away. Take that away and it sees nothing, AT
#      ANY WIDTH: a no-space redirect to a VARIABLE target carries no `BOARD.md` near the operator
#      at all, and an `&>` carries no whitespace-bounded ` > `. Both really write the file. Only
#      Guard 1 can see them, because Guard 1 never asks what the target is called.
#
#   B. REDIRECT_RE RED / GUARD 1 GREEN — the STATEMENT BOUNDARY, WITH NO VARIABLE CARRYING THE
#      BYTES. Guard 1's Q1 reads ONE INVOCATION TOKEN, and a token ends at a `;`. So a write that
#      puts the renderer in one statement and the redirect in the NEXT is invisible to it — UNLESS
#      the bytes travel between the two in a VARIABLE, which Q2's taint now follows. Two shapes
#      carry them with no variable at all, and those are the ones that remain: a BRACE GROUP and a
#      WRAPPER FUNCTION. Both really write BOARD.md (executed against a stub renderer — the bytes
#      arrive). REDIRECT_RE catches both, because it flattens the whole file with `tr` and spans 200
#      characters across the boundary Q1 stops at.
#      WHAT USED TO BE HERE AND IS NOT ANY MORE: CAPTURE-THEN-WRITE. The paragraph below reserved
#      the right to flip a direction-B row to RED if Guard 1 ever learned to see across a statement
#      boundary without reddening the real calls. Stage 8 did exactly that, so the row MOVED — it is
#      now scored in the BOTH-RED section — and it earned the move the hard way: a live-tree probe
#      showed the pair was NOT actually covering the class. REDIRECT_RE only ever caught the LITERAL
#      `> "$d/BOARD.md"` spelling; the spelling a real regression in docket-status.sh would use
#      (`>"$mw/$rel"` — variable target, no space) was GREEN under BOTH guards. Direction A now
#      carries that exact probe as a Guard-1-only row.
#
# WHY NOT JUST WIDEN GUARD 1 TO COVER (B)? Because a token that spans statements IS a whole-file
# scan — it would be REDIRECT_RE, rebuilt worse, inside a function that must also stay narrow
# enough not to redden on the codebase's real calls. The repo's own ledger rule is the one that
# applies: ONE GUARD, ONE HOLE — when a mutation slips past a guard, add an INDEPENDENT scan rather
# than widening the first, and never delete a sentinel, because deleting a sentinel is how the
# guarded hole reopens.
#
# ONE MORE THING ABOUT DIRECTION B, so it is not misread as a ban on improvement: its GREEN half is
# a CHARACTERIZATION of Guard 1's bound, not a requirement that Guard 1 stay bounded forever. If
# someone genuinely teaches Guard 1 to see across a statement boundary WITHOUT reddening the
# codebase's real calls, flip that row to RED and keep the REDIRECT_RE half. What may NOT happen —
# the thing this block exists to prevent — is deleting REDIRECT_RE's script scan while these three
# shapes are caught by nothing else.
#
# If a future author deletes REDIRECT_RE's script scan as "redundant", direction B goes red here.
# If they narrow Guard 1's token, direction A (and the pipeline rows above) go red. Either way the
# suite says so, naming the fixture — instead of production discovering it.

# --- direction A: shapes ONLY Guard 1 can see (REDIRECT_RE is structurally blind) ---
# A1: no-space redirect to a VARIABLE target. The literal "BOARD.md" appears nowhere near the `>`.
printf '%s\n' '#!/usr/bin/env bash' \
  '"$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" >"$mw/$rel"' > "$mut/comp-nospace-var.sh"
# A2: `&>` — a merged-output write. No whitespace-bounded ` > ` anywhere for REDIRECT_RE to match.
#     ($mut/amp-redirect.sh, row 7, reused: the battery already proves Guard 1 reds it.)
# A3: THE LIVE-TREE PROBE (row 19) — capture-then-write to a VARIABLE target. This is the shape that
#     was GREEN under BOTH guards until stage 8 landed, and it is here to record WHY REDIRECT_RE was
#     never the answer to it: the target is `"$mw/$rel"`, so no `/BOARD.md` sits near the operator at
#     ANY width, and there is no whitespace-bounded ` > ` either. Only Guard 1 can reach it.
# A4: the `readonly` + `${out}` + `>|` + variable-target composite (row 19f) — same reasoning.
for onlyg1 in "$mut/comp-nospace-var.sh" "$mut/amp-redirect.sh" "$mut/taint-live-probe.sh" \
              "$mut/taint-brace-clobber.sh"; do
  assert "complementarity A: guard1 flags ${onlyg1##*/} (a REAL write)" \
    '! render_board_write_free "$onlyg1" >/dev/null'
  assert "complementarity A: REDIRECT_RE is BLIND to ${onlyg1##*/} — so guard1 may not be deleted" \
    '! tr "\n" " " < "$onlyg1" | grep -Eq "$REDIRECT_RE"'
done

# --- direction B: shapes ONLY REDIRECT_RE can see (Guard 1 carries NO variable across the boundary)
# Each of these two REALLY WRITES BOARD.md — the renderer's bytes reach the file — and each is GREEN
# under Guard 1: the invocation token holds no `>`, and no VARIABLE captures the stdout for stage
# 8's taint to follow. That is not a bug to be fixed here; it is the bound of a token-scoped scan
# with a one-hop taint, and it is why the whole-file scan below stays.
# B1: brace group — the redirect belongs to the GROUP, not to the invocation inside it.
printf '%s\n' '#!/usr/bin/env bash' \
  '{ "$SCRIPTS_DIR"/render-board.sh --changes-dir "$d"; } > "$d/BOARD.md"' \
  > "$mut/comp-brace-group.sh"
# B2: a wrapper function — the invocation is in the body, the redirect is on the CALL.
printf '%s\n' '#!/usr/bin/env bash' \
  'board(){ "$SCRIPTS_DIR"/render-board.sh --changes-dir "$d"; }; board > "$d/BOARD.md"' \
  > "$mut/comp-wrapper-fn.sh"
for onlyre in "$mut/comp-brace-group.sh" "$mut/comp-wrapper-fn.sh"; do
  assert "complementarity B: REDIRECT_RE flags ${onlyre##*/} — so the script scan may not be deleted" \
    'tr "\n" " " < "$onlyre" | grep -Eq "$REDIRECT_RE"'
  assert "complementarity B: guard1 is BOUNDED — it does NOT see ${onlyre##*/} (no variable crosses the statement)" \
    'render_board_write_free "$onlyre" >/dev/null'
done

# --- the OVERLAP column: CAPTURE-THEN-WRITE, now caught by BOTH. It USED to be a direction-B row
# (REDIRECT_RE-only), and moving it is the honest bookkeeping of what stage 8 changed. It is scored
# BOTH-RED rather than deleted for two reasons: it records that the migration really happened (a
# future author reading this block sees the shape, not a hole in the numbering), and it keeps the
# LITERAL spelling under REDIRECT_RE's watch even if stage 8 is ever narrowed.
# THE OVERLAP IS ONLY ON THIS SPELLING. The VARIABLE-TARGET spelling of the very same class — the
# live-tree probe, row A3 — is Guard 1's ALONE. So the pair is still not a subsumption in either
# direction: A holds (4 rows, the probe among them), B holds (2 rows).
printf '%s\n' '#!/usr/bin/env bash' \
  'out=$("$SCRIPTS_DIR"/render-board.sh --changes-dir "$d"); printf "%s" "$out" > "$d/BOARD.md"' \
  > "$mut/comp-capture-write.sh"
assert "complementarity OVERLAP: guard1 (stage 8 taint) now flags comp-capture-write.sh" \
  '! render_board_write_free "$mut/comp-capture-write.sh" >/dev/null'
assert "complementarity OVERLAP: REDIRECT_RE also flags comp-capture-write.sh (the literal spelling)" \
  'tr "\n" " " < "$mut/comp-capture-write.sh" | grep -Eq "$REDIRECT_RE"'

# --- malformed id is skipped (active + archive), renderer still succeeds ---
printf -- '---\nid: abc\nslug: bad\ntitle: Bad Active\nstatus: proposed\npriority: low\ndepends_on: []\n---\n' > "$tmp/active/0099-bad.md"
printf -- '---\nid: nope\nslug: badarc\ntitle: Bad Archive\nstatus: done\npriority: low\ndepends_on: []\n---\n' > "$tmp/archive/2026-06-01-0098-badarc.md"
mout="$("$SCRIPT" --changes-dir "$tmp" 2>/tmp/render-board-stderr.$$)"; mrc=$?
assert "render-board exits 0 with a malformed-id file present" '[ "$mrc" -eq 0 ]'
assert "render-board skips malformed active row (title absent)"  '! printf "%s" "$mout" | grep -q "Bad Active"'
assert "render-board skips malformed archive row (title absent)" '! printf "%s" "$mout" | grep -q "Bad Archive"'
rm -f "$tmp/active/0099-bad.md" "$tmp/archive/2026-06-01-0098-badarc.md" /tmp/render-board-stderr.$$

# --- change 0069: --format digest (the line-oriented backlog projection) ---
# The digest is a SECOND projection of the dependency/readiness pass render-board.sh already
# runs — same source of truth as the board's Readiness cell, machine-parseable instead of prose.
# It is report output, never a board surface: docket-status.sh pipes it through without writing.

# (a) regression guard: the DEFAULT (no --format) output is byte-identical to the golden.
#     This is the load-bearing guarantee of the whole change — the digest must not perturb the
#     markdown path by a single byte. ("$rendered" was produced from the golden compare above.)
defaulted="$tmp/out-default.md"
bash "$SCRIPT" --changes-dir "$tmp" --repo o/r > "$defaulted" 2>/dev/null
assert "default output is byte-identical to the golden after --format lands" \
  'diff -u "$golden" "$defaulted"'

# (b) an explicit --format markdown is byte-identical to the default.
explicit="$tmp/out-markdown.md"
bash "$SCRIPT" --changes-dir "$tmp" --repo o/r --format markdown > "$explicit" 2>/dev/null
assert "--format markdown is byte-identical to the default" 'diff -u "$defaulted" "$explicit"'

# (c) the digest's exact shape, byte-compared to a hand-authored golden. Rollups first (fixed
#     status order, non-zero only), then one `change` line per ACTIVE change, ascending id.
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
digest_out="$tmp/digest-out.txt"
bash "$SCRIPT" --changes-dir "$tmp" --repo o/r --format digest > "$digest_out" 2>/dev/null
drc=$?
assert "--format digest exits 0" '[ "$drc" -eq 0 ]'
assert "--format digest matches the digest golden byte-for-byte" 'diff -u "$digest_golden" "$digest_out"'

# (d) each readiness band individually (named asserts so a break names the band it broke).
assert "digest: build-ready token"            'grep -qxF "change 2 proposed build-ready bravo" "$digest_out"'
assert "digest: needs-brainstorm token"       'grep -qxF "change 3 proposed needs-brainstorm charlie" "$digest_out"'
assert "digest: auto-groom-blocked token"     'grep -qxF "change 4 proposed auto-groom-blocked delta" "$digest_out"'
assert "digest: waiting-on-N-unbuilt token"   'grep -qxF "change 5 proposed waiting-on-3-unbuilt echo" "$digest_out"'
assert "digest: waiting-on-N-needs-merge token" 'grep -qxF "change 6 proposed waiting-on-8-needs-merge foxtrot" "$digest_out"'
assert "digest: readiness is - for a non-proposed change" 'grep -qxF "change 1 in-progress - alpha" "$digest_out"'

# (e) the digest carries NO markdown board (it is a projection, not the board).
assert "digest emits no board markdown" '! grep -qF "# Backlog" "$digest_out"'
assert "digest emits no mermaid graph"  '! grep -qF "mermaid" "$digest_out"'

# (f) archive rollups only — archived changes get no `change` line (the digest is the ACTIVE backlog).
assert "digest: archived changes get no change line" '! grep -qE "^change (10|11|12) " "$digest_out"'

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
exp_ready="$(awk '$3=="proposed" && $4=="build-ready" {print $2}' <(grep "^change " "$digest_out") | sort -n | tr '\n' ' ')"
got_ready="$(sed -n 's/^ready //p' "$digest_out" | tr -s ' ' | sed 's/ *$//' | tr ' ' '\n' | sort -n | tr '\n' ' ')"
assert "ready membership equals the build-ready change lines" '[ "$exp_ready" = "$got_ready" ]'

# (iii) ORDERING, on a dedicated fixture. Three bands prove the three sort keys independently.
ord="$tmp/ord"; mkdir -p "$ord/active" "$ord/archive"
# id 30: medium, oldest  -> age beats id (before 31)
# id 31: medium, newest  -> loses on age to 30, but still beats an unstamped peer (36)
# id 32: critical, newest -> priority beats age AND id (first overall)
# id 33: high, newest     -> second overall
# id 34: low, oldest      -> last, despite being the oldest (priority outranks age)
# id 35: (no priority:)   -> defaults to medium; same created as 30 -> tie falls to LOWEST id (30 < 35)
# id 36: medium, NO created: line at all -> unknown age sorts LAST within its priority band, after
#         every dated medium peer (30, 35, 31) — an unstamped change must never preempt dated work.
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

# id 36 deliberately omits `created:` (and `updated:`) entirely — write_ord always emits a
# (possibly-empty-valued) created: line, so this fixture is written by hand to get the true
# absent-field case `field()` returns empty for.
cat > "$ord/active/0036-sierra2.md" <<'EOF'
---
id: 36
slug: sierra2
title: sierra2
status: proposed
priority: medium
depends_on: []
spec: docs/superpowers/specs/x.md
trivial: false
---

## Why
x
EOF

ord_out="$tmp/ord-digest.txt"
bash "$SCRIPT" --changes-dir "$ord" --format digest > "$ord_out" 2>/dev/null
assert "ordering fixture: exact selection order (priority > created > id)" \
  '[ "$(sed -n "s/^ready //p" "$ord_out")" = "32 33 30 35 31 36 34" ]'
assert "ordering: critical outranks an older medium"  '[ "$(sed -n "s/^ready //p" "$ord_out" | cut -d" " -f1)" = "32" ]'
assert "ordering: low sorts last despite being oldest" '[ "$(sed -n "s/^ready //p" "$ord_out" | awk "{print \$NF}")" = "34" ]'
assert "ordering: an absent priority: defaults to medium (35 sits in the medium band)" \
  '[ "$(sed -n "s/^ready //p" "$ord_out" | cut -d" " -f4)" = "35" ]'
assert "ordering: exact tie (same priority+created) falls to the LOWEST id" \
  '[ "$(sed -n "s/^ready //p" "$ord_out" | cut -d" " -f3,4)" = "30 35" ]'
# An unset created: is a DECIDED RULE, not an accident: unknown age sorts LAST within its priority
# band, so an unstamped change never preempts dated work. Isolate the medium band's three dated ids
# (30, 35, 31) plus the unstamped 36 in their EMITTED order (not a fixed expected string) and assert
# 36 is last among them — this fails under the old behavior, where an empty `created:` sorts before
# every real date and 36 would jump to the FRONT of the band instead.
med_band="$(sed -n "s/^ready //p" "$ord_out" | tr ' ' '\n' | grep -E '^(30|31|36)$' | tr '\n' ' ')"
assert "ordering: a change with no created: line sorts LAST within its priority band" \
  '[ "$(printf "%s" "$med_band" | awk "{print \$NF}")" = "36" ]'

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
# Positive control: prove $ord_md is really a rendered board before trusting the negative assert
# below — an empty file from a failed render would ALSO pass "no ready line" for the wrong reason.
assert "the markdown render actually produced a board" 'grep -q "^# Backlog" "$ord_md"'
assert "markdown projection carries no ready line" '! grep -q "^ready" "$ord_md"'

# (g) an unknown --format value is an argument error (exit 2), like any other bad flag.
bash "$SCRIPT" --changes-dir "$tmp" --format bogus >/dev/null 2>"$tmp/fmt-err.txt"
frc=$?
assert "unknown --format exits 2" '[ "$frc" -eq 2 ]'
assert "unknown --format names the flag on stderr" 'grep -qi "format" "$tmp/fmt-err.txt"'

# (h) the digest is a READ-ONLY projection: it writes no BOARD.md (that file is board-refresh.sh's
#     alone) and creates no git state. The BOARD.md check is the one that bites — nothing ever
#     `git init`s $tmp, so the git-free half is true before render-board.sh is even invoked and
#     would stay green even if the digest branch wrote the board.
assert "digest run writes no BOARD.md into the changes dir" '[ ! -e "$tmp/BOARD.md" ]'
assert "digest run leaves the fixture dir git-free" '[ ! -d "$tmp/.git" ]'

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
depends_on: [40]
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
assert "big: a referenced done that is ALSO collapsed (0040, April) is still styled :::done" 'grep -qxF -- "  0040:::done" "$big_out"'
assert "big: unreferenced verbatim done 0071 NOT in the mermaid" '! grep -qxF -- "  0071:::done" "$big_out"'
assert "big: an unreferenced collapsed done (0041, April) is NOT in the mermaid" '! grep -qxF -- "  0041:::done" "$big_out"'
assert "big: exactly one :::done node in the graph" '[ "$(grep -cF -- ":::done" "$big_out")" -eq 1 ]'

# (d) determinism — a second render is byte-identical.
big_out2="$tmp/big-out2.md"
bash "$SCRIPT" --changes-dir "$big" --repo o/r > "$big_out2" 2>/dev/null
assert "big: re-render is byte-identical (determinism)" 'diff -u "$big_out" "$big_out2"'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
