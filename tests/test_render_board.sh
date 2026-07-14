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
# TWO GUARDS DEFEND THAT INVARIANT AND NEITHER SUBSUMES THE OTHER — THAT IS THE DESIGN, NOT AN
# OVERSIGHT.
#   - GUARD 1 (here) is TOKEN-SCOPED and TARGET-BLIND. It prohibits the WRITE on a render-board.sh
#     invocation and on the pipeline that invocation feeds, so it never matches a filename and
#     cannot be evaded by renaming one — but it is STRUCTURALLY BLIND to a write that crosses a
#     STATEMENT boundary.
#   - REDIRECT_RE (defined further down) is WHOLE-FILE and TARGET-KEYED. It flattens the source
#     with `tr` and spans 200 characters, so it DOES see those cross-statement shapes — but it is
#     blind to every redirect that does not park a literal `/BOARD.md` a whitespace-bounded ` > `
#     away (no-space, `>>`, `>|`, `&>`, a variable target: all invisible to it AT ANY WIDTH).
# Each guard catches shapes the other misses. That is not a claim resting on this comment: the
# COMPLEMENTARITY block (below the REDIRECT_RE definition) PROVES it by mutation, in BOTH
# directions, against the real regex and the real function. DELETING EITHER GUARD, OR "SIMPLIFYING"
# ONE INTO THE OTHER, REOPENS A HOLE — and turns that block red on the way out.
#
# Guard 1 prohibits the WRITE instead of recognizing the write TARGET, so all of these die
# identically and none of them is a filename match:
#     >"$d/BOARD.md"   (no space — the IDIOMATIC shell form, which REDIRECT_RE cannot see)
#     >> / >|          (append and clobber-force)
#     > "$mw/$rel"     (VARIABLE target — the literal string "BOARD.md" appears nowhere near the
#                       redirect, so NO regex keyed on /BOARD\.md can reach it AT ANY WIDTH; this
#                       is what forecloses "just widen REDIRECT_RE" as a complete answer)
#     &> / &>> / >&f   (the merged-output forms — see stage 3; each one really does write the file)
#     | cat > "$f"     (a redirect ANYWHERE IN THE PIPELINE the renderer feeds — the pipeline IS
#     |& cat > "$f"     its stdout, so the token does NOT end at the `|`, nor at the `|&`, which is
#                       just `2>&1 |`; see stages 4 and 5)
# It never asks WHERE you are writing.
#
# WHAT IT ACTUALLY CHECKS, STATED WITHOUT OVERCLAIM — this is a bounded guard, and the bound is
# published here so nobody has to rediscover it: for each render-board.sh invocation TOKEN — the
# invocation AND THE PIPELINE IT FEEDS, cut at a `;`, an `&`, or a `||` — is there a surviving `>`
# once true fd dups are erased. That is a SOURCE-SYNTAX property of ONE TOKEN. It is not a
# filesystem fact, and it is NOT a property of the file. Two consequences, both real and both
# load-bearing:
#
#   * IT IS BLIND TO A WRITE THAT CROSSES A STATEMENT BOUNDARY. Every one of these really writes
#     BOARD.md — executed against a stub renderer, the bytes arrive — and every one is GREEN here,
#     because the renderer's own token ENDS at the `;` and the redirect belongs to a LATER
#     statement that the token never reaches:
#         { "$SCRIPTS_DIR"/render-board.sh --changes-dir "$d"; } > "$d/BOARD.md"
#         out=$("$SCRIPTS_DIR"/render-board.sh --changes-dir "$d"); printf '%s' "$out" > "$d/BOARD.md"
#         board(){ "$SCRIPTS_DIR"/render-board.sh --changes-dir "$d"; }; board > "$d/BOARD.md"
#     WIDENING THE TOKEN IS NOT THE FIX. A token that spans statements is a whole-file scan — which
#     is precisely what REDIRECT_RE already is, and REDIRECT_RE catches ALL THREE of these (it
#     flattens the file and spans 200 chars). Chasing them from here would just rebuild REDIRECT_RE
#     badly, inside a function that also has to stay narrow enough not to redden on the codebase's
#     real calls. THIS IS WHY BOTH GUARDS SHIP, and the COMPLEMENTARITY block asserts exactly this
#     pair of directions so the next author cannot delete one and keep the invariant.
#   * IT CANNOT SEE A WRITE THAT IS NOT SPELLED WITH A `>` ON THE TOKEN: a pipeline member that
#     writes without a redirect (`| tee f`), an fd opened on a prior line (`exec 3>"$f"` … `>&3`),
#     an operator that only exists at runtime (`eval`), or a `>` the scan never reached because it
#     mis-lexed a quote. Those are the KNOWN, ACCEPTED GAPS below.
#
# The call-site list is DERIVED FROM A find(1) SWEEP, never hand-maintained (ledger #64: never
# hand-list the call sites of an operation you are gating). The sweep covers scripts/lib/*.sh,
# which a flat scripts/*.sh glob would silently miss. board-refresh.sh — the one gated writer
# (change 0059: render to temp -> chmod -> rename) — is the ONLY allowlisted script.
#
# PIPELINE ORDER IS LOAD-BEARING:
#   1. STRIP COMMENTS FIRST — drop whole-line ones, THEN strip trailing ones. Prose that merely
#      NAMES the script is not an invocation. render-adr-index.sh, render-change-links.sh, and
#      render-board.sh itself mention it in comments ONLY, so this step is what keeps them out of
#      the scan, not a nicety. The trailing strip (a `#` preceded by whitespace, to end of line) is
#      equally load-bearing: whole-line stripping alone leaves
#      `render-board.sh ... 2>&2  # digest -> stdout only` with a bare `>` from the comment's
#      ARROW, turning the guard RED on a legitimate call — and this codebase's comment style is
#      full of `->` arrows, so that is a live trap, not a hypothetical.
#      TRADE-OFF, DISCLOSED NOT HIDDEN — THE SCAN IS LEXER-NAIVE ABOUT QUOTES. It honours `#`, `;`,
#      `&` and `||` as shell metacharacters WHEREVER they appear, including inside a QUOTED
#      ARGUMENT, where bash treats them as inert text. Each therefore truncates the token early and
#      can hide a redirect placed after it — these really write the file and all of them stay GREEN
#      (verified against a stub renderer):
#          render-board.sh --repo "a #b" > "$out"   (this strip eats from the ` #` to end of line)
#          render-board.sh --repo "a;b"  > "$out"   (stage 6 cuts the token at the `;`)
#          render-board.sh --repo "a&b"  > "$out"   (stage 6 cuts the token at the `&`)
#          render-board.sh --repo "a||b" > "$out"   (stage 4 rewrites the `||` to a `;`, then 6 cuts)
#      The fix would be a quote-aware lexer — a bash parser, out of all proportion to a test
#      sentinel — and the exposure is narrow: render-board.sh's only arguments are --changes-dir,
#      --repo and --format, none of which takes a value containing `#`, `;`, `&` or `|`. The strip,
#      in exchange, fixes a false positive that is REAL TODAY (the `->` arrow trap above). That is
#      the trade. The honest answer to all of it is the deferred filesystem-effect test (gaps below).
#   2. ONLY THEN JOIN backslash-continuations into logical lines. The tokenizer is line-oriented,
#      so a redirect parked on a continuation line hands it a first-line token with no `>` — a
#      clean pass. (REDIRECT_RE survives that shape for a different reason: it flattens the file
#      with `tr` first. Same shape, two independent mechanisms — which is the pair working as
#      intended, not one being redundant.)
#      WHY COMMENTS MUST BE STRIPPED BEFORE THE JOIN, NOT AFTER — this ordering is a FIX, and the
#      obvious order (join, then strip) is EXPLOITABLE. Bash comments are PHYSICAL-LINE scoped: a
#      trailing backslash does NOT continue a comment onto the next line. So in
#          # regenerate the board \
#          "$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" > "$out"
#      the second line REALLY EXECUTES and REALLY WRITES THE FILE (proven against a stub renderer).
#      Join-first folds it INTO the comment, and the comment drop then deletes BOTH lines — the
#      write is laundered and the guard passes clean. Stripping first deletes only the comment,
#      leaving the live invocation (and its `>`) standing alone. The continuation rows below still
#      go RED under this order because NEITHER of their lines is a comment: the join still happens,
#      it just no longer runs on text that a comment can swallow.
#      A FAIL-SAFE SIDE EFFECT OF THIS ORDER, so the next author is not surprised: dropping the
#      comments first lets the join reach ACROSS one. In
#          "$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" \
#          # not actually a continuation
#            > "$out"
#      bash removes the `\`-newline, then reads `#` as starting a comment that runs to end of line,
#      so the command ENDS there and the `> "$out"` line is a separate statement that never
#      receives the renderer's output. The guard deletes the comment, joins the two survivors and
#      calls it RED — a FALSE POSITIVE. The error direction of this order is therefore FAIL-SAFE:
#      it can only ever ADD a false alarm, never hide a write. That is why it is disclosed and
#      accepted rather than engineered around.
#   3. NORMALIZE `&>`/`&>>` TO `>`, BEFORE tokenizing. `&>file` and `&>>file` are WRITES (stdout+
#      stderr merged into the file). The tokenizer cuts on `;` and `&` (stage 6), so the `&` of
#      `&>` would END the token BEFORE its `>` — the write would vanish. Rewriting them to a bare
#      `>` first makes them ordinary writes.
#   4. NORMALIZE THE TWO `|`-COMPOUND OPERATORS, because the tokenizer's separators are `;` and `&`
#      and these two spellings smuggle an `&` (or an extra `|`) into a position that means the
#      OPPOSITE of what a raw scan would infer:
#        - `|&` IS A PIPELINE (`2>&1 |`), so it must STAY INSIDE the token: rewrite it to `|`.
#          Left alone, its `&` ENDS the token at stage 6 and every redirect downstream of it
#          vanishes — `render-board.sh ... |& cat > "$d/BOARD.md"` really writes the board, and it
#          was GREEN until this stage existed (verified against a stub renderer).
#        - `||` STARTS A NEW COMMAND, so the token must be CUT there: rewrite it to `;`. Left
#          alone, `|` does not cut (stage 5) and the OR-branch's redirect is read as the renderer's
#          — `out="$(render-board.sh ... 2>&2)" || echo failed > "$log"` writes nothing of the
#          renderer's, yet went RED. A guard that reddens on that is a guard someone deletes.
#      Both rewrites are pinned by battery rows; the `|&` one must precede nothing in particular,
#      but it must come BEFORE the tokenizer, and its position relative to the fd-dup erasure is
#      immaterial (checked both ways — the erasure preserves its right boundary).
#   5. ERASE ONLY TRUE fd DUPS, still before tokenizing. The erasure must key on the TARGET BEING A
#      WHOLE, REAL fd — not on the operator, and not merely on the target's PREFIX. `>&2`, `2>&1`,
#      `>&-` dup/close a descriptor (harmless), but `>&"$out"` and `>& file` are WRITES that share
#      the same `>&` spelling. So the erasure requires a digit run or `-` after the `&` AND A RIGHT
#      BOUNDARY after that: `[0-9]*>&([0-9]+|-)($|[[:space:];&|)}"])`. Both halves are bugs already
#      paid for:
#        * without the digit/`-` requirement (`[0-9-]*`), the class matches ZERO characters and
#          erases `>&"$out"` outright, laundering a file write into nothing.
#        * without the RIGHT BOUNDARY, the erasure is a PREFIX match: bash reads `>&word` with a
#          word that is not a valid fd as a merged WRITE, so `>&2board.md` really creates the file
#          `2board.md` — yet an unanchored `>&([0-9]+|-)` eats the `>&2` and leaves a bare
#          `board.md` with no `>` left to find (proven against a stub renderer). The boundary makes
#          the erasure fire only when the descriptor ENDS there. `|` is IN that boundary class, so
#          `2>&1| head` (no space) is still erased whole — a row pins it.
#      Erasing the true dups also keeps the `&` of `2>&2` from CUTTING the token mid-redirect and
#      leaving a dangling `>` that fires on the codebase's CORRECT invocation.
#   6. TOKENIZE PER INVOCATION, CUTTING AT `;` AND `&` — NEVER AT A BARE `|`. Two points, both
#      load-bearing:
#        - PER INVOCATION, not per line: a logical line carrying a clean call beside a rogue one
#          must not be whitewashed by the clean one (ledger #64). `;` and `&` begin genuinely NEW
#          commands, so they end the token — and so does a `||`, which stage 4 has already
#          rewritten INTO a `;` for exactly that reason.
#        - A BARE `|` DOES NOT END THE TOKEN, because A PIPELINE *IS* THE RENDERER'S STDOUT —
#          everything downstream of the `|` is where those bytes go, so a redirect anywhere in the
#          pipeline is a redirect OF the renderer. An earlier cut of this guard tokenized on
#          `[^;&|]*` and so read
#              "$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" | cat > "$d/BOARD.md"
#          as an invocation that ENDED at the `|`, with no `>` left in it — GREEN, on a line that
#          really writes BOARD.md (verified against a stub renderer). That is the pre-0059 anti-
#          pattern wearing a `| cat`.
#      The invocation is recognized at a WORD BOUNDARY — `(^|[^-[:alnum:]])render-board\.sh` — not
#      at a leading `/`: a PATH-resolved `render-board.sh ... > "$out"` (no directory part) writes
#      the file just as surely as `"$SCRIPTS_DIR"/render-board.sh` does, and keying on the slash
#      would miss it. The boundary still excludes a merely SIMILAR name (`my-render-board.sh` is
#      not this script).
#   7. ANY surviving `>` in a token is a file-directed redirect => violation. This rejects even a
#      stderr-to-file form (2>/dev/null) — deliberately conservative: the correct way to route
#      this renderer's stderr is the fd dup already in use (2>&2), and a guard that permits SOME
#      writes is a guard whose next author must relitigate which.
#
# KNOWN, ACCEPTED GAPS. Two kinds, and the difference matters:
#
# (I) NOT A GAP OF THE PAIR — Guard 1 misses it, REDIRECT_RE covers it:
#   0. THE STATEMENT-BOUNDARY CLASS (`{ …; } > f`, capture-then-write, a wrapper function). Spelled
#      out above. Guard 1 is GREEN on all three; REDIRECT_RE is RED on all three; the
#      COMPLEMENTARITY block asserts both halves. It is listed here so nobody "fixes" Guard 1 by
#      widening its token — the fix already exists, and it is the other guard.
#
# (II) GENUINE GAPS OF THE PAIR — real writes that NEITHER a token scan nor a target-keyed regex
#      reliably sees. They are disclosed, not fixed, and they share one answer (below):
#   a. A PIPELINE MEMBER THAT WRITES WITHOUT A `>`: `render-board.sh ... | tee f`. The pipeline is
#      INSIDE the token (stage 6), so any redirect in it dies — but `tee` needs no redirect to
#      write. Catching it would mean knowing WHICH COMMANDS WRITE, i.e. hand-maintaining a list of
#      writers, which is the same anti-pattern (ledger #64) as hand-listing call sites. Out of
#      reach for this scan on purpose.
#   b. fd INDIRECTION — a redirect OPENED ON A PRIOR LINE, in either of two shapes:
#        - `exec 3>"$out"`, then `render-board.sh ... >&3`. The file IS written, and the guard
#          stays green because `>&3` is — syntactically, locally, and correctly — an fd dup.
#        - `exec > "$out"`, then a BARE `render-board.sh ...` with NO redirect on it at all. Here
#          the invocation token carries no `>` whatsoever, so there is nothing for ANY tightening
#          of stages 3-7 to catch: the write lives entirely on the earlier `exec` line.
#      A scan of the invocation cannot know what an fd was opened to, or that stdout was rebound
#      out from under it; it would have to follow descriptors across statements, which is
#      interpretation, not grep.
#   c. THE REDIRECT OPERATOR ARRIVING BY EXPANSION OR eval:
#          r='>'; eval "\"\$SCRIPTS_DIR\"/render-board.sh --changes-dir \"\$d\" $r \"\$out\""
#      writes the file, and the source text carries no `>` on the invocation to find. Unreachable
#      by construction: the redirect does not exist as syntax until runtime.
#   d. A `;`, `&`, `||` OR `#` INSIDE A QUOTED ARGUMENT, cutting the token short of a real redirect
#      (the lexer-naivety trade-off under stage 1). Same root cause as (c): the scan reads text,
#      not a parse tree.
#
# (III) KNOWN FALSE POSITIVES, both FAIL-SAFE (they can only add an alarm, never hide a write), so
#      they are disclosed rather than engineered around:
#   e. A HEREDOC BODY that quotes the anti-pattern as PROSE. `cat > "$f" <<EOS` … a line reading
#      `render-board.sh ... > "$d/BOARD.md"` … `EOS` writes nothing of the renderer's, but neither
#      guard knows what a heredoc is: Guard 1 reads the body as source and goes RED (so does
#      REDIRECT_RE). No such heredoc exists in scripts/ today; if one is ever added, quote it so it
#      does not read as an invocation, or move the example into a comment.
#   f. The join reaching ACROSS a comment that bash would treat as ending the command (stage 2).
#
# The answer to the (II) gaps is the filesystem-effect test the design DEFERRED (run the
# orchestrator against a fixture and assert BOARD.md's bytes): it is syntax-independent but
# path-dependent, and it is the ONLY thing that can see a `| tee`, an fd opened elsewhere, a
# redirect conjured by eval, or a quote the scan mis-lexed. THAT IS WHY THE DEFERRED TEST STILL HAS
# A JOB with both guards in place.
#
# The battery below is the substance of this guard (ledger #64: a guard is code — mutation-test it
# before trusting it, or it is decoration). Every evasion above is injected into a fixture and MUST
# turn the guard RED; the controls — the codebase's real invocation among them — MUST keep it
# GREEN. Every row's verdict has been compared against FILESYSTEM TRUTH: each fixture was executed
# against a stub renderer and the target inspected for the renderer's bytes.

# Folds backslash-continuations into logical lines. awk, not sed: BSD sed does not portably treat
# `\n` in an s/// LHS as a newline, so the classic `:a; /\\$/N; s/\\\n//; ta` is a GNU-ism here.
# PLANNED TWIN: tests/test_docket_status.sh's flag sentinel tokenizes the same unit and needs the
# same fix (Guard 3 of change 0070). It does not exist yet — when it lands, the two copies must be
# kept in step.
join_continuations(){
  awk '{ while (sub(/\\$/, "")) { if ((getline nxt) > 0) { $0 = $0 nxt } else { break } } print }' "$1"
}

# 0 = clean; 1 = at least one render-board.sh invocation (or the pipeline it feeds) carries a
# file-directed redirect.
# Stage order per the block above: comments OUT first (a comment cannot continue across a trailing
# backslash, so joining first would let one SWALLOW a live invocation), THEN join, THEN normalize
# `&>`, THEN normalize the `|`-compounds (`|&` is a PIPELINE and must stay in the token; `||` is a
# NEW COMMAND and must cut it), THEN erase whole-and-only-whole fd dups, THEN tokenize per
# invocation at a word boundary.
# The tokenizer's `[^;&]*` cuts at `;` and `&` ONLY — NOT at a bare `|`: the pipeline a renderer
# feeds is its stdout, so `... | cat > f` must stay INSIDE the token (see stage 6; `[^;&|]*` here
# was a false-GREEN on a real BOARD.md write).
# BOUNDED BY CONSTRUCTION: this sees the invocation TOKEN, so a write that crosses a STATEMENT
# boundary (`{ ...; } > f`, capture-then-write, a wrapper function) is GREEN here and RED under
# REDIRECT_RE's whole-file scan. Do not widen the token to chase them — see COMPLEMENTARITY below.
# Diagnostics go to STDOUT so the real scan can name the offending script; the mutation battery
# routes them to /dev/null, since an EXPECTED red row is not an operator-actionable violation.
render_board_write_free(){
  local f="$1" violation=0 inv
  while IFS= read -r inv; do
    [ -n "$inv" ] || continue
    echo "  (render-board.sh invocation writes to a file in ${f##*/}: $inv)"
    violation=1
  done < <(
    join_continuations <(
      grep -v '^[[:space:]]*#' "$f" | sed 's/[[:space:]]#.*$//'
    ) \
      | sed 's/&>>\{0,1\}/>/g' \
      | sed 's/|&/|/g; s/||/;/g' \
      | sed -E 's/[0-9]*>&([0-9]+|-)($|[[:space:];&|)}"])/\2/g' \
      | grep -oE '[^;&]*(^|[^-[:alnum:]])render-board\.sh[^;&]*' \
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
#   B. REDIRECT_RE RED / GUARD 1 GREEN — the STATEMENT BOUNDARY. Guard 1 reads ONE INVOCATION
#      TOKEN, and a token ends at a `;`. So a write that puts the renderer in one statement and the
#      redirect in the NEXT is invisible to it: a brace group, a capture-then-write, a wrapper
#      function. All three really write BOARD.md (executed against a stub renderer — the bytes
#      arrive). REDIRECT_RE catches all three, because it flattens the whole file with `tr` and
#      spans 200 characters across the boundary Guard 1 stops at.
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
for onlyg1 in "$mut/comp-nospace-var.sh" "$mut/amp-redirect.sh"; do
  assert "complementarity A: guard1 flags ${onlyg1##*/} (a REAL write)" \
    '! render_board_write_free "$onlyg1" >/dev/null'
  assert "complementarity A: REDIRECT_RE is BLIND to ${onlyg1##*/} — so guard1 may not be deleted" \
    '! tr "\n" " " < "$onlyg1" | grep -Eq "$REDIRECT_RE"'
done

# --- direction B: shapes ONLY REDIRECT_RE can see (Guard 1's token ends at the statement boundary)
# Each of these three REALLY WRITES BOARD.md — the renderer's bytes reach the file — and each is
# GREEN under Guard 1. That is not a bug to be fixed here; it is the bound of a token-scoped scan,
# and it is why the whole-file scan below stays.
# B1: brace group — the redirect belongs to the GROUP, not to the invocation inside it.
printf '%s\n' '#!/usr/bin/env bash' \
  '{ "$SCRIPTS_DIR"/render-board.sh --changes-dir "$d"; } > "$d/BOARD.md"' \
  > "$mut/comp-brace-group.sh"
# B2: capture-then-write — the renderer's stdout is parked in a variable, then written by a
#     SECOND command. The invocation itself carries no redirect at all.
printf '%s\n' '#!/usr/bin/env bash' \
  'out=$("$SCRIPTS_DIR"/render-board.sh --changes-dir "$d"); printf "%s" "$out" > "$d/BOARD.md"' \
  > "$mut/comp-capture-write.sh"
# B3: a wrapper function — the invocation is in the body, the redirect is on the CALL.
printf '%s\n' '#!/usr/bin/env bash' \
  'board(){ "$SCRIPTS_DIR"/render-board.sh --changes-dir "$d"; }; board > "$d/BOARD.md"' \
  > "$mut/comp-wrapper-fn.sh"
for onlyre in "$mut/comp-brace-group.sh" "$mut/comp-capture-write.sh" "$mut/comp-wrapper-fn.sh"; do
  assert "complementarity B: REDIRECT_RE flags ${onlyre##*/} — so the script scan may not be deleted" \
    'tr "\n" " " < "$onlyre" | grep -Eq "$REDIRECT_RE"'
  assert "complementarity B: guard1 is BOUNDED — it does NOT see ${onlyre##*/} (write crosses a statement)" \
    'render_board_write_free "$onlyre" >/dev/null'
done

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
