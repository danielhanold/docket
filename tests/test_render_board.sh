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

**12 changes** — 🟢 1 in progress · 🟡 5 proposed · 🔴 1 blocked · ⚪ 1 deferred · 🔵 1 implemented · ✅ 2 done · 🗑️ 1 killed

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

## 🔵 Implemented — awaiting merge (1)

| # | Title | Priority | PR |
|---|-------|----------|----|
| [0008](active/0008-hotel.md) | Hotel feature | `high` | [#142](https://github.com/o/r/pull/142) |

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
  0010:::done
  0012:::done
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

# --- Guard 1: the repo-wide render-board.sh WRITE sentinel (change 0070) ----------------------
# THE INVARIANT, stated once: render-board.sh's stdout reaches a file through board-refresh.sh
# and NOTHING else.
#
# Every guard that came before encoded a PROXY for that sentence — "a --format digest flag is
# present" (tests/test_docket_status.sh), "a ` > ` appears near the string BOARD.md" (REDIRECT_RE,
# below) — and every known evasion is an exploit of the gap between the proxy and the invariant.
# This guard prohibits the WRITE instead of recognizing the write TARGET, so it never matches a
# filename and cannot be evaded by renaming one:
#     >"$d/BOARD.md"   (no space — the IDIOMATIC shell form, which REDIRECT_RE cannot see)
#     >> / >|          (append and clobber-force)
#     > "$mw/$rel"     (VARIABLE target — the literal string "BOARD.md" appears nowhere near the
#                       redirect, so NO regex keyed on /BOARD\.md can reach it AT ANY WIDTH; this
#                       is what forecloses "just widen REDIRECT_RE" as a complete answer)
#     &> / &>> / >&f   (the merged-output forms — see stage 3; each one really does write the file)
# all die identically. It never asks WHERE you are writing.
#
# WHAT IT ACTUALLY CHECKS, stated without overclaim: for each render-board.sh invocation TOKEN, is
# there a surviving `>` after fd dups are erased. That is a SOURCE-SYNTAX property of the token,
# not a filesystem fact — so it catches every redirect written ON the invocation, and it cannot
# see a write reached INDIRECTLY (through a pipe, or through an fd opened elsewhere). Those are
# named in KNOWN, ACCEPTED GAPS below rather than papered over.
#
# The call-site list is DERIVED FROM A find(1) SWEEP, never hand-maintained (ledger #64: never
# hand-list the call sites of an operation you are gating). The sweep covers scripts/lib/*.sh,
# which a flat scripts/*.sh glob would silently miss. board-refresh.sh — the one gated writer
# (change 0059: render to temp -> chmod -> rename) — is the ONLY allowlisted script.
#
# PIPELINE ORDER IS LOAD-BEARING:
#   1. JOIN backslash-continuations into logical lines. The tokenizer is line-oriented, so a
#      redirect parked on a continuation line hands it a first-line token with no `>` — a clean
#      pass. (REDIRECT_RE survived that shape only because it flattens the file with `tr` first;
#      that flattening was never incidental. Guard 1 subsumes the scan it replaces ONLY because
#      it joins.)
#   2. DROP whole-line comments, THEN STRIP TRAILING ones — prose that merely NAMES the script is
#      not an invocation. render-adr-index.sh, render-change-links.sh, and render-board.sh itself
#      mention it in comments ONLY, so this step is what keeps them out of the scan, not a nicety.
#      The trailing strip (a `#` preceded by whitespace, to end of line) is equally load-bearing:
#      whole-line stripping alone leaves `render-board.sh ... 2>&2  # digest -> stdout only` with a
#      bare `>` from the comment's ARROW, turning the guard RED on a legitimate call — and this
#      codebase's comment style is full of `->` arrows, so that is a live trap, not a hypothetical.
#      TRADE-OFF, DISCLOSED NOT HIDDEN: a `#` inside a QUOTED ARGUMENT on a render-board.sh
#      invocation line would truncate that line early and could hide a redirect placed after it.
#      No such invocation exists (render-board.sh's only arguments are --changes-dir, --repo, and
#      --format, none of which take a `#`), and the strip fixes a false positive that is real
#      today against a false negative that requires a flag this renderer does not have.
#   3. NORMALIZE `&>`/`&>>` TO `>`, THEN ERASE ONLY TRUE fd DUPS, both BEFORE tokenizing. Subtle
#      and mandatory, in this order:
#        - `&>file` and `&>>file` are WRITES (stdout+stderr merged into the file). The tokenizer
#          splits on `; & |`, so the `&` of `&>` would END the token BEFORE its `>` — the write
#          would vanish. Rewriting `&>`/`&>>` to a bare `>` first makes them ordinary writes.
#        - the fd-dup erasure must key on whether the TARGET IS A REAL fd, not on the operator:
#          `>&2`, `2>&1`, `>&-` dup/close a descriptor (harmless), but `>&"$out"` and `>& file`
#          are WRITES that share the same `>&` spelling. Hence `[0-9]*>&([0-9]+|-)` — a digit run
#          or `-` is REQUIRED after the `&`. (A `[0-9-]*` there matches ZERO characters and so
#          erases `>&"$out"`, laundering a real file write into nothing. That was the bug.)
#        Erasing the true dups first also keeps the `&` of `2>&2` from CUTTING the token
#        mid-redirect and leaving a dangling `>` that fires on the codebase's CORRECT invocation.
#   4. TOKENIZE PER INVOCATION, not per line: a logical line carrying a clean call beside a rogue
#      one must not be whitewashed by the clean one (ledger #64).
#   5. ANY surviving `>` in a token is a file-directed redirect => violation. This rejects even a
#      stderr-to-file form (2>/dev/null) — deliberately conservative: the correct way to route
#      this renderer's stderr is the fd dup already in use (2>&2), and a guard that permits SOME
#      writes is a guard whose next author must relitigate which.
#
# KNOWN, ACCEPTED GAPS — both are INDIRECT writes, invisible to any source scan of the invocation
# line, and both have the SAME answer (see below). They are disclosed, not fixed:
#   a. PIPE INTO A WRITER: `render-board.sh ... | tee f`. The tokenizer ends the invocation at the
#      `|`, so the writer on the far side is never examined.
#   b. fd INDIRECTION: `exec 3>"$out"` on one line, then `render-board.sh ... >&3` on another. The
#      file IS written, and the guard stays green because `>&3` is — syntactically, locally, and
#      correctly — an fd dup. A scan of the invocation cannot know what fd 3 was opened to; it
#      would have to follow the descriptor across statements, which is interpretation, not grep.
# The answer to BOTH is the filesystem-effect test the design DEFERRED (run the orchestrator
# against a fixture, assert BOARD.md's bytes): it is syntax-independent but path-dependent, and it
# earns its cost when a write path exists that a source scan cannot reach. Today none does.
#
# The battery below is the substance of this guard (ledger #64: a guard is code — mutation-test it
# before trusting it, or it is decoration). Every evasion above is injected into a fixture and MUST
# turn the guard RED; the fd-dup control — the codebase's real invocation — MUST keep it GREEN.

# Folds backslash-continuations into logical lines. awk, not sed: BSD sed does not portably treat
# `\n` in an s/// LHS as a newline, so the classic `:a; /\\$/N; s/\\\n//; ta` is a GNU-ism here.
# TWIN: an identical function lives in tests/test_docket_status.sh (Guard 3 tokenizes the same
# unit and inherits the same fix) — keep the two in step.
join_continuations(){
  awk '{ while (sub(/\\$/, "")) { if ((getline nxt) > 0) { $0 = $0 nxt } else { break } } print }' "$1"
}

# 0 = clean; 1 = at least one render-board.sh invocation carries a file-directed redirect.
render_board_write_free(){
  local f="$1" violation=0 inv
  while IFS= read -r inv; do
    [ -n "$inv" ] || continue
    echo "  (render-board.sh invocation writes to a file in ${f##*/}: $inv)"
    violation=1
  done < <(
    join_continuations "$f" \
      | grep -v '^[[:space:]]*#' \
      | sed 's/[[:space:]]#.*$//' \
      | sed 's/&>>\{0,1\}/>/g' \
      | sed -E 's/[0-9]*>&([0-9]+|-)//g' \
      | grep -oE '[^;&|]*/render-board\.sh[^;&|]*' \
      | grep '>' || true
  )
  return "$violation"
}

# --- the mutation battery: each row is a fixture the guard must judge correctly ---
mut="$tmp/mut"; mkdir -p "$mut"

# (1) spaced redirect — the ONLY shape REDIRECT_RE could already see. Establishes the baseline.
printf '%s\n' '#!/usr/bin/env bash' \
  '"$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" > "$d/BOARD.md"' > "$mut/spaced.sh"
assert "guard1 flags a spaced redirect into BOARD.md" \
  '! render_board_write_free "$mut/spaced.sh"'

# (2) NO SPACE — the idiomatic shell redirect, invisible to REDIRECT_RE's [[:space:]]>[[:space:]].
printf '%s\n' '#!/usr/bin/env bash' \
  '"$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" >"$d/BOARD.md"' > "$mut/nospace.sh"
assert "guard1 flags a no-space redirect into BOARD.md" \
  '! render_board_write_free "$mut/nospace.sh"'

# (3) append. Not special-cased: the board is a full regeneration, never an accumulation — but
#     this dies for the same reason everything else does, not because append is uniquely wrong.
printf '%s\n' '#!/usr/bin/env bash' \
  '"$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" >> "$d/BOARD.md"' > "$mut/append.sh"
assert "guard1 flags an append (>>) redirect into BOARD.md" \
  '! render_board_write_free "$mut/append.sh"'

# (4) clobber-force.
printf '%s\n' '#!/usr/bin/env bash' \
  '"$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" >| "$d/BOARD.md"' > "$mut/clobber.sh"
assert "guard1 flags a clobber-force (>|) redirect into BOARD.md" \
  '! render_board_write_free "$mut/clobber.sh"'

# (5) VARIABLE TARGET — the row that forecloses widening. docket-status.sh really does hold the
#     board path in a variable (`local rel="$CHANGES_DIR/BOARD.md"`), so a rogue write can carry
#     the literal "BOARD.md" nowhere near the redirect. Unreachable by any BOARD.md-keyed regex.
printf '%s\n' '#!/usr/bin/env bash' \
  'rel="$CHANGES_DIR/BOARD.md"' \
  '"$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" > "$mw/$rel"' > "$mut/vartarget.sh"
assert "guard1 flags a redirect to a VARIABLE target (no BOARD.md near the >)" \
  '! render_board_write_free "$mut/vartarget.sh"'

# (6) CONTINUATION LINE — the redirect sits on the second physical line. A line-oriented tokenizer
#     sees a first-line token with no `>` and passes it clean. This is why joining precedes
#     tokenizing, and why dropping the old flattened scan is only safe once it does.
printf '%s\n' '#!/usr/bin/env bash' \
  '"$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" \' \
  '  > "$f"' > "$mut/continuation.sh"
assert "guard1 flags a redirect parked on a continuation line" \
  '! render_board_write_free "$mut/continuation.sh"'

# (6b) CONTINUATION + NO SPACE — the union shape the design named as still-live under BOTH old
#      guards (evades the line-oriented tokenizer AND REDIRECT_RE's whitespace requirement).
printf '%s\n' '#!/usr/bin/env bash' \
  '"$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" \' \
  '  >"$mw/$rel"' > "$mut/continuation-nospace.sh"
assert "guard1 flags a no-space redirect on a continuation line (the union evasion)" \
  '! render_board_write_free "$mut/continuation-nospace.sh"'

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
  '! render_board_write_free "$mut/amp-redirect.sh"'

# (8) &>> — the appending twin of the above.
printf '%s\n' '#!/usr/bin/env bash' \
  '"$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" &>> "$out"' > "$mut/amp-append.sh"
assert "guard1 flags an &>> (merged, appending) redirect into a file" \
  '! render_board_write_free "$mut/amp-append.sh"'

# (9) >& file — same merge, the other spelling. Shares its operator with the HARMLESS fd dups
#     (>&2, >&-), which is exactly why the erasure must inspect the TARGET, not the operator.
printf '%s\n' '#!/usr/bin/env bash' \
  '"$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" >& "$out"' > "$mut/gtamp-spaced.sh"
assert "guard1 flags a >& FILE redirect (not an fd dup — the target is a path)" \
  '! render_board_write_free "$mut/gtamp-spaced.sh"'

# (10) >&"$out" — no space. The shape the old `[0-9-]*` erasure literally deleted: it matched the
#      empty string after the `&`, so a file write was rewritten into nothing and passed clean.
printf '%s\n' '#!/usr/bin/env bash' \
  '"$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" >&"$out"' > "$mut/gtamp-nospace.sh"
assert "guard1 flags a no-space >&\"\$out\" FILE redirect (the zero-width fd-dup erasure bug)" \
  '! render_board_write_free "$mut/gtamp-nospace.sh"'

# (11) A ROGUE REDIRECT SITTING BEFORE A TRAILING COMMENT. The other half of the trailing-comment
#      fix (see row 14): stripping trailing comments must NOT become a way to launder a real write.
#      The strip removes only what follows the `#`; the redirect precedes it and still dies.
printf '%s\n' '#!/usr/bin/env bash' \
  '"$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" > "$f"  # regenerate the board' \
  > "$mut/rogue-before-comment.sh"
assert "guard1 still flags a rogue redirect that sits BEFORE a trailing comment" \
  '! render_board_write_free "$mut/rogue-before-comment.sh"'

# (12) FALSE-POSITIVE CONTROL — the codebase's REAL invocation, copied verbatim from
#     scripts/docket-status.sh, fd dup and all. A guard that fires on this is a guard someone
#     disables; this row carries as much weight as every RED row above it.
printf '%s\n' '#!/usr/bin/env bash' \
  'if ! out="$("$SCRIPTS_DIR"/render-board.sh --changes-dir "$cd_dir" --format digest 2>&2)"; then' \
  '  return 1' \
  'fi' > "$mut/fddup.sh"
assert "guard1 stays GREEN on the real 2>&2 --format digest invocation (false-positive control)" \
  'render_board_write_free "$mut/fddup.sh"'

# (13) FALSE-POSITIVE CONTROL — a 2>&1 fd dup beside a COMMENT that spells out the old redirect.
#      Comment-stripping is load-bearing: three scripts name render-board.sh in comments only.
printf '%s\n' '#!/usr/bin/env bash' \
  '# historical (pre-0059): render-board.sh --changes-dir "$d" > "$d/BOARD.md"' \
  'out="$("$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" 2>&1)"' > "$mut/comment.sh"
assert "guard1 stays GREEN on an fd-dup call beside a comment naming the old redirect" \
  'render_board_write_free "$mut/comment.sh"'

# (14) FALSE-POSITIVE CONTROL — a legitimate call carrying a TRAILING comment whose prose contains
#      an ARROW. Whole-line comment stripping does not touch this line, so before the trailing
#      strip the comment's `->` left a `>` inside the token and the guard fired RED on a correct
#      invocation. This codebase's comments are full of `->` arrows; a guard that reddens on one
#      is a guard someone deletes. Row 11 is its mirror: the strip must not launder a real write.
printf '%s\n' '#!/usr/bin/env bash' \
  'out="$("$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" --format digest 2>&2)"  # digest -> stdout only' \
  > "$mut/trailing-comment.sh"
assert "guard1 stays GREEN on a legit call with a trailing '# ... -> ...' comment" \
  'render_board_write_free "$mut/trailing-comment.sh"'

# (15) FALSE-POSITIVE CONTROL — `>&-` closes a descriptor. It is not a write, it shares the `>&`
#      spelling with the file-writing forms in rows 9-10, and the fix must keep telling them
#      apart: the target here is `-`, not a path.
printf '%s\n' '#!/usr/bin/env bash' \
  'out="$("$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" --format digest 2>&2 >&-)"' \
  > "$mut/fdclose.sh"
assert "guard1 stays GREEN on a >&- fd close (a descriptor, not a file)" \
  'render_board_write_free "$mut/fdclose.sh"'

# --- the real scan: every script under scripts/ EXCEPT the one allowlisted writer ---
guard1_violation=0
scanned=0
while IFS= read -r s; do
  [ "${s##*/}" = "board-refresh.sh" ] && continue
  scanned=$((scanned + 1))
  render_board_write_free "$s" || guard1_violation=1
done < <(find "$REPO/scripts" -name '*.sh' -type f | sort)
assert "no script under scripts/ (except board-refresh.sh) writes render-board.sh's stdout to a file" \
  '[ "$guard1_violation" -eq 0 ]'

# Anti-vacuity: a scan over zero files passes for the wrong reason. Assert the sweep actually saw
# the tree, and that the allowlisted writer it skips really exists (a rename must not silently
# turn the allowlist into a no-op that hides the real writer).
assert "the write scan is not vacuous (it swept the scripts tree)" '[ "$scanned" -ge 10 ]'
assert "the allowlisted writer scripts/board-refresh.sh exists" \
  '[ -f "$REPO/scripts/board-refresh.sh" ]'

# --- negative sentinel: no skill body may redirect render-board.sh stdout straight into
# BOARD.md (the pre-0059 anti-pattern this task removes). Whitespace-normalize per file first
# since the old redirect could span physical lines. The guard regex:
#   render-board\.sh.{0,200}[[:space:]]>[[:space:]].{0,200}/BOARD\.md
# Design (each element defends a specific real shape in this codebase's prose):
#   - `.{0,200}` bounded any-char gaps (NOT `[^>]*`): the historical redirect's destination is
#     a bracket placeholder `<metadata working tree>/<changes_dir>/BOARD.md` whose `>` characters
#     and internal spaces a `[^>]*` class could never cross — so `[^>]*` was BLIND to the exact
#     reintroduction shape this sentinel exists to catch. `.` crosses placeholder `>`s freely.
#   - `[[:space:]]>[[:space:]]` a whitespace-bounded redirect operator: a real ` > ` redirect has
#     a space on both sides, whereas a placeholder's closing bracket is `tree>` / `dir>/` (letter
#     before `>`, or no space after) — so the porcelain guard line and every `<...>` placeholder
#     are structurally excluded.
#   - `/BOARD\.md` (slash required, not bare `BOARD.md`): a real redirect target is a PATH ending
#     in `/BOARD.md`; this rejects a flattened markdown blockquote (`\n> ` -> ` > `) that lands a
#     bare "BOARD.md" prose word within the window — blockquotes genuinely appear in
#     docket-status and docket-implement-next, so this is a live false-positive class, not
#     hypothetical.
# All five requirements + the blockquote case are verified empirically below and by the
# positive-control assertion. THE SAME REGEX is used for both the positive control and the
# across-skills scan, so weakening it (e.g. back to `[^>]*`) trips the positive control loudly.
REDIRECT_RE='render-board\.sh.{0,200}[[:space:]]>[[:space:]].{0,200}/BOARD\.md'

# Positive control ("test the test"): the historical bracket-placeholder redirect that WAS in
# this codebase pre-0059 MUST still be flagged by the guard. If a future edit weakens REDIRECT_RE
# so it can no longer cross placeholder brackets, this assertion fails — not the silent scan.
HISTORICAL_REDIRECT='"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/render-board.sh --changes-dir <metadata working tree>/<changes_dir> > <metadata working tree>/<changes_dir>/BOARD.md'
assert "guard regex flags the historical bracket-placeholder redirect (positive control)" \
  'printf "%s" "$HISTORICAL_REDIRECT" | tr "\n" " " | grep -Eq "$REDIRECT_RE"'

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
backlog implemented 1
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

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
