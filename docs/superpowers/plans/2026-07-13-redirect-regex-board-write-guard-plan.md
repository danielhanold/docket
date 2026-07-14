# Harden the BOARD.md write guard — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a guard that prohibits the *write* itself, so no-space, `>>`, `>|`, `&>`, `|&`, variable-target, pipeline and continuation-line redirects of `render-board.sh`'s stdout all die identically — and mutation-test every one of them against filesystem truth.

**Architecture:** Three guards, derived from a single invariant — *`render-board.sh`'s stdout reaches a file through `board-refresh.sh` and nothing else.*

**Guard 1** (new, `tests/test_render_board.sh`) is a repo-wide, **token-scoped, target-blind** write sentinel: a bash function that, for any script, drops whole-line **and trailing** comments → **only then** joins backslash-continuations → normalizes `&>`/`&>>` to a bare `>` → normalizes the two `|`-compound operators (`|&` **is** a pipeline, so it becomes a bare `|` and stays *inside* the token; `||` **starts a new command**, so it becomes a `;` and *cuts* the token) → erases **only true** fd dups (`>&2`, `2>&1`, `>&-` — the target must be a digit run or `-` **and must END there**, so a file-writing `>&"$f"` or `>&2board.md` is *not* erased) → tokenizes per invocation at a **word boundary** (`(^|[^-[:alnum:]])render-board\.sh`, so a bare PATH-resolved call is seen too) → fails on any surviving `>`. It never matches the write *target*, so it cannot be evaded by renaming it.

**Guard 2** (`REDIRECT_RE`, `tests/test_render_board.sh`) is **whole-file and target-keyed**: it flattens a file with `tr` and looks for a whitespace-bounded ` > ` within 200 characters of a literal `/BOARD.md`. It is **KEPT, UNWIDENED, AND KEEPS BOTH OF ITS SCANS** — `skills/*/SKILL.md` prose *and* `scripts/docket-status.sh`.

**THE TWO GUARDS ARE COMPLEMENTARY, NOT NESTED — and that is the single most important fact in this plan.** The spec (and two earlier cuts of this plan) said Guard 1 would *subsume* `REDIRECT_RE`'s script scan, which could therefore be retired. **Mutation testing disproved that**, in both directions:

- **Guard 1 sees what `REDIRECT_RE` cannot.** A no-space redirect to a **variable** target (`>"$mw/$rel"`) carries no literal `BOARD.md` anywhere near the operator, and an `&>` carries no whitespace-bounded ` > ` at all. `REDIRECT_RE` is blind to both **at any width**; both really write the file.
- **`REDIRECT_RE` sees what Guard 1 cannot.** Guard 1 reads **one invocation token**, and a token ends at a `;`. So a write that crosses a **statement boundary** is invisible to it — a brace group (`{ render-board.sh …; } > f`), a capture-then-write (`out=$(render-board.sh …); printf … > f`), or a wrapper function. All three really write `BOARD.md` (executed against a stub renderer — the bytes arrive), all three are GREEN under Guard 1, and all three are RED under `REDIRECT_RE`, which flattens the file and spans the boundary Guard 1 stops at.

Widening Guard 1's token to cover the second class would make it a whole-file scan — i.e. `REDIRECT_RE`, rebuilt worse, inside a function that must also stay narrow enough not to redden on the codebase's real calls. The repo's own ledger rule governs: **one guard, one hole** — when a mutation slips past a guard, add an *independent* scan rather than widening the first; deleting a sentinel is how the guarded hole reopens. Both guards therefore ship, and Task 1's **COMPLEMENTARITY block** locks the decision by asserting both directions against the **real** `$REDIRECT_RE` and the **real** shipped function.

**Comments come out BEFORE the join, and that order is a correctness requirement, not a preference.** Bash comments are *physical-line* scoped: a trailing backslash does not continue a comment. So `# regenerate the board \` followed by a real, redirecting invocation is a **live write** — but a join-first pipeline folds the invocation into the comment and the comment drop then deletes both, laundering the write into nothing. Stripping first deletes only the comment and leaves the invocation standing.

**The token is cut at `;`, `&` and `||` — never at a bare `|`, because a pipeline IS the renderer's stdout.** An earlier cut tokenized on `[^;&|]*` and therefore read `render-board.sh … | cat > "$d/BOARD.md"` as an invocation *ending at the `|`*, with no `>` in it — **GREEN on a line that really writes `BOARD.md`**. Two corollaries, both paid for with bugs: `|&` (which *is* `2>&1 |`, a pipeline) smuggles an `&` that would end the token one character before the write it carries, so it is rewritten to `|`; and `||` (which is *not* a pipeline — it starts a new command) would otherwise leave the OR-branch's redirect inside the renderer's token, reddening the guard on `out="$(render-board.sh … 2>&2)" || echo failed > "$log"`, which writes nothing of the renderer's. An error-handling idiom that reddens a guard is how a guard gets deleted.

**What Guard 1 checks, stated without overclaim.** For each `render-board.sh` invocation token — the invocation *and the pipeline it feeds* — is there a surviving `>` once true fd dups are erased. That is a property of **source syntax on one token**, not of the filesystem and not of the file. Beyond the statement-boundary class (covered by Guard 2), four kinds of real write are out of its reach and are **disclosed as accepted gaps**: (a) a pipeline member that writes with no `>` at all (`| tee f`) — catching it would mean hand-maintaining a list of *which commands write*, the same anti-pattern (ledger #64) as hand-listing call sites; (b) a redirect through an fd opened on a prior line (`exec 3>"$f"` + `>&3`, or `exec > "$f"` + a *bare* invocation); (c) a redirect operator that only exists at runtime (`r='>'` + `eval`); (d) a `;`, `&`, `||` or `#` **inside a quoted argument**, which cuts the token short — the scan is lexer-naive about quotes, and fixing that means writing a bash parser. All four have the **same answer**: the filesystem-effect test this design deferred. *That is why the deferred test still has a job even with both guards in place.* Two known **false positives** run the other way and are **fail-safe** (they can only add an alarm, never hide a write): a heredoc body that quotes the anti-pattern as prose, and the join reaching *across* a comment that bash would treat as ending the command.

Guard 1 ships as a **function** precisely so the mutation battery calls the *same code* the real scan calls — a battery that tests a copy is decoration. **Guard 3** (`tests/test_docket_status.sh`) keeps the `--format digest` flag check and inherits the continuation-joining fix.

**Tech Stack:** Bash (`set -uo pipefail`), `awk` (continuation joining — portable across BSD/GNU, unlike `sed`'s `N`/`\n` handling), `grep -oE`, `find`. No production code changes; no new dependencies.

## Global Constraints

- **Test-only change.** `render-board.sh`, `board-refresh.sh`, and `docket-status.sh` are all correct today and MUST NOT be modified. Copied verbatim from the spec: *"No production code changes... This change is about the suite's ability to notice if a future one is not."* Any live-tree probe must revert through a `trap`, so a timeout cannot leave the tree dirty.
- **`REDIRECT_RE` is kept UNWIDENED and byte-identical**: `render-board\.sh.{0,200}[[:space:]]>[[:space:]].{0,200}/BOARD\.md`. Its positive control (`HISTORICAL_REDIRECT`) stays too, and **both of its scans stay** — `skills/*/SKILL.md` *and* `scripts/docket-status.sh`. Only its *comment* changes, plus two added assertions.
- **NEITHER GUARD MAY BE DELETED OR FOLDED INTO THE OTHER.** Each catches shapes the other misses; the COMPLEMENTARITY block proves it by mutation, in both directions, against the real regex and the real function. A future author who "simplifies" one into the other gets a red test naming the fixture.
- **Call-site lists are DERIVED, never hand-maintained** (ledger #64). Guard 1 enumerates scripts with `find "$REPO/scripts" -name '*.sh' -type f` — which covers `scripts/lib/*.sh`, invisible to a flat `scripts/*.sh` glob.
- **`board-refresh.sh` is the ONLY allowlisted writer**, matched by basename.
- **Every guard is mutation-tested against FILESYSTEM TRUTH** (ledger #64: *a guard is code — mutation-test it before trusting it, or it is decoration*). Every fixture is executed against a stub renderer and the target inspected for the renderer's bytes; the guard's verdict must agree with what actually happened on disk. Each evasion must turn the guard RED; every control must keep it GREEN.
- The false-positive control is the codebase's real invocation, copied verbatim from `scripts/docket-status.sh:172`:
  `out="$("$SCRIPTS_DIR"/render-board.sh --changes-dir "$cd_dir" --format digest 2>&2)"`
- Existing harness conventions in both test files are preserved: `set -uo pipefail`, `REPO=`, `fail=0`, `assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }`, and the `$tmp` mktemp dir with its `trap`.
- **The `tests/test_render_board.sh` diff is a PURE INSERTION vs `origin/main`** (zero `-` lines): Task 1 inserts, Task 2 rewrites only comments inside the block it owns.

---

### Task 1: Guard 1 — the repo-wide write sentinel, its mutation battery, and the complementarity lock

**Files:**
- Modify: `tests/test_render_board.sh` — insert the joiner, the guard function, the battery, and the real scan immediately BEFORE the existing `REDIRECT_RE` block (the comment starting `# --- negative sentinel:`). Task 2 rewrites that block's *comment*; this task does not touch it. A **second block** (the COMPLEMENTARITY lock) goes immediately AFTER that block, because it needs `$REDIRECT_RE` in scope.
- Test: `tests/test_render_board.sh` (the battery *is* the test — the guard function is the code under test).

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces:
  - `join_continuations <file>` → stdout: the file with backslash-continuations folded into logical lines. Task 3 defines an identical twin in `tests/test_docket_status.sh`.
  - `render_board_write_free <file>` → exit **0** = clean (no file-directed redirect on the invocation or the pipeline it feeds), exit **1** = violation (offending invocations echoed to stdout).
  - The COMPLEMENTARITY block, which is **Task 2's precondition**: it is what establishes that `REDIRECT_RE`'s script scan is load-bearing and must be KEPT.

- [ ] **Step 1: Write the failing test — the mutation battery + the real scan**

Insert this block into `tests/test_render_board.sh` immediately before the line `# --- negative sentinel: no skill body may redirect render-board.sh stdout straight into`.

The battery comes first *in the file* as well as first in time: it is the reason the guard is trustworthy, and a reader must meet it before the scan that relies on it.

```bash
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
```

Then append this SECOND block immediately AFTER the existing `# --- negative sentinel:` block (i.e. after its final assertion, `"scripts/docket-status.sh never redirects render-board.sh stdout into BOARD.md"`) and before `# --- malformed id is skipped`. It goes there, not with the battery, for one reason: it needs `$REDIRECT_RE` in scope so it can cross-check against **the real regex** rather than a copy that could drift out of agreement with the thing it is proving.

```bash
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
```

- [ ] **Step 2: Run the test to verify the battery has teeth**

Because `render_board_write_free` and `join_continuations` are pasted in Step 1 together with the battery, the honest red state to observe is a **mutation of the guard itself**. Neuter it twice, on a COPY of the test file (never the live one), and confirm the battery notices in both directions:

```bash
cd /Users/homer/dev/docket/.worktrees/redirect-regex-board-write-guard
# 1. guard never fires  -> every RED row must go NOT OK
#    render_board_write_free(){ return 0; }
# 2. guard always fires  -> every GREEN control must go NOT OK
#    render_board_write_free(){ return 1; }
```

Expected: with `return 0`, the **21** `guard1 flags …` rows and the **2** `complementarity A: guard1 flags …` rows print `NOT OK`, while every `guard1 stays GREEN …` control and every `complementarity B: REDIRECT_RE flags …` row still passes (they do not consult the stub). With `return 1`, the **9** `guard1 stays GREEN …` controls, the real scan, and the **3** `complementarity B: guard1 is BOUNDED …` rows print `NOT OK`. A guard that never fires — and a guard that always fires — are both caught.

Two further mutations prove the newest pipeline stages are load-bearing rather than decorative:

- **Delete the `| sed 's/|&/|/g; s/||/;/g'` stage** → exactly four rows go red: the two `|&` rows (a real write goes unseen) and the two `||` rows (a false positive on an error-handling idiom). Nothing else moves.
- **Drop `|` from the fd-dup right-boundary class** (`($|[[:space:];&|)}"])` → `($|[[:space:];&)}"])`) → exactly **one** row goes red: `2>&1| head` (no space). Its spaced twin `2>&1 | head` stays green — which is precisely why the no-space row exists, and why the spaced row's comment must not claim to pin the `|`.

- [ ] **Step 3: Run the test to verify it passes with the real guard**

Run: `cd /Users/homer/dev/docket/.worktrees/redirect-regex-board-write-guard && bash tests/test_render_board.sh 2>&1 | grep -E "guard1|complementarity|write scan|allowlisted|^(PASS|FAIL)"`

Expected — all **43** guard-1 and complementarity rows `ok -`, and the file ends `PASS`. Note that no `(render-board.sh invocation writes to a file ...)` diagnostics appear: the battery routes the guard's stdout to `/dev/null`, because an EXPECTED red row is not an operator-actionable violation and printing it makes a passing suite read like a failing one in CI. The real scan keeps its diagnostics — that is where an operator needs to know *which* script violated.

```
ok - guard1 flags a spaced redirect into BOARD.md
ok - guard1 flags a no-space redirect into BOARD.md
ok - guard1 flags an append (>>) redirect into BOARD.md
ok - guard1 flags a clobber-force (>|) redirect into BOARD.md
ok - guard1 flags a redirect to a VARIABLE target (no BOARD.md near the >)
ok - guard1 flags a redirect parked on a continuation line
ok - guard1 flags a no-space redirect on a continuation line (the union evasion)
ok - guard1 flags an &> (merged stdout+stderr) redirect into a file
ok - guard1 flags an &>> (merged, appending) redirect into a file
ok - guard1 flags a >& FILE redirect (not an fd dup — the target is a path)
ok - guard1 flags a no-space >&"$out" FILE redirect (the zero-width fd-dup erasure bug)
ok - guard1 flags >&2board.md (fd-dup erasure must be right-anchored, not a prefix match)
ok - guard1 still flags a rogue redirect that sits BEFORE a trailing comment
ok - guard1 flags a live redirect laundered under a comment line ending in a backslash
ok - guard1 flags a bare PATH-resolved render-board.sh (no leading slash) with a redirect
ok - guard1 flags a redirect on the FAR SIDE of a pipeline (| cat > BOARD.md)
ok - guard1 flags a no-space redirect on the far side of a pipeline (| cat >"$f")
ok - guard1 flags an appending redirect further down a pipeline (| sed ... >> "$f")
ok - guard1 flags a spaced /BOARD.md redirect parked on a continuation line
ok - guard1 flags a redirect behind a |& pipe (|& cat > BOARD.md)
ok - guard1 flags a |& redirect to a variable target with an adjacent fd dup (2>&1|& cat > "$f")
ok - guard1 stays GREEN on the real 2>&2 --format digest invocation (false-positive control)
ok - guard1 stays GREEN on an fd-dup call beside a comment naming the old redirect
ok - guard1 stays GREEN on a legit call with a trailing '# ... -> ...' comment
ok - guard1 stays GREEN on a >&- fd close (a descriptor, not a file)
ok - guard1 stays GREEN on a pipeline with no redirect (| grep -c)
ok - guard1 stays GREEN on a 2>&1 fd dup piped into a reader (2>&1 | head)
ok - guard1 stays GREEN on a no-space fd dup against a pipe (2>&1| head — pins | in the boundary class)
ok - guard1 stays GREEN on a || error branch whose redirect belongs to another command
ok - guard1 stays GREEN on a no-space || error branch ("..."||echo failed > "$log")
ok - no script under scripts/ (except board-refresh.sh) writes render-board.sh's stdout to a file
ok - the write scan is not vacuous (it swept the scripts tree)
ok - the allowlisted writer scripts/board-refresh.sh exists
ok - complementarity A: guard1 flags comp-nospace-var.sh (a REAL write)
ok - complementarity A: REDIRECT_RE is BLIND to comp-nospace-var.sh — so guard1 may not be deleted
ok - complementarity A: guard1 flags amp-redirect.sh (a REAL write)
ok - complementarity A: REDIRECT_RE is BLIND to amp-redirect.sh — so guard1 may not be deleted
ok - complementarity B: REDIRECT_RE flags comp-brace-group.sh — so the script scan may not be deleted
ok - complementarity B: guard1 is BOUNDED — it does NOT see comp-brace-group.sh (write crosses a statement)
ok - complementarity B: REDIRECT_RE flags comp-capture-write.sh — so the script scan may not be deleted
ok - complementarity B: guard1 is BOUNDED — it does NOT see comp-capture-write.sh (write crosses a statement)
ok - complementarity B: REDIRECT_RE flags comp-wrapper-fn.sh — so the script scan may not be deleted
ok - complementarity B: guard1 is BOUNDED — it does NOT see comp-wrapper-fn.sh (write crosses a statement)
PASS
```

If the `2>&2` false-positive control is RED, the fd-dup erasure is missing or misordered — the `&` of `2>&2` is cutting the token and leaving a dangling `>`. If the `&>` / `>& FILE` rows are GREEN, the erasure is keying on the OPERATOR instead of the TARGET (a `[0-9-]*` that matches zero characters erases a real file write) or the `&>` normalization is missing. If the `>&2board.md` row is GREEN, the erasure lost its RIGHT boundary and has degraded into a prefix match. If the comment-backslash row is GREEN, someone has "tidied" the pipeline back into the intuitive join-first order and a live write is being laundered into a dead comment. If any `| cat > …` or `|& cat > …` row is GREEN, the tokenizer is cutting the token at a `|` (or at the `&` inside a `|&`) — a pipeline IS the renderer's stdout, and those shapes really write `BOARD.md`. If a `||` control is RED, stage 4's `||` → `;` rewrite is gone and the guard is now reddening on ordinary error handling. **If a `complementarity B: REDIRECT_RE flags …` row is RED, `REDIRECT_RE` has been weakened and the statement-boundary class is now caught by NOTHING — fix the regex, do not delete the row.** Fix the pipeline; never relax an assertion.

- [ ] **Step 4: Prove the guard against the LIVE tree, not just fixtures**

The battery proves the function works on fixtures. This proves it is wired to reality: inject the evasion into the real `scripts/docket-status.sh`, confirm the suite goes RED, then revert.

The real line is `scripts/docket-status.sh:172`, and it ends with `--format digest 2>&2)"` — **ONE** closing paren (the command substitution's) before the quote. A pattern expecting `2>&2))"` matches nothing and the "mutation" is a silent no-op that proves the guard works by never testing it. Confirm the mutation landed before trusting the RED.

Use **two** live mutations, in this order — the second is the one that matters most:

1. **The comment-backslash laundering shape.** It indicts the pipeline ORDER, the part a future edit is most likely to "tidy" back into the intuitive join-first form.
2. **The pipeline shape with a VARIABLE target** — `render-board.sh … | cat > "$rel"`. This one is the sharpest proof available, because `REDIRECT_RE` is *structurally blind* to it (no literal `BOARD.md` near the `>`), so **only Guard 1's own assertion can go red**. If you only ever mutate with a literal `BOARD.md` target, both guards fire and you have not shown that Guard 1 is wired to anything.

```bash
cd /Users/homer/dev/docket/.worktrees/redirect-regex-board-write-guard
# BINDING: a trap, not a trailing `git checkout`. This mutates a PRODUCTION file in a TEST-ONLY
# change; if the run is interrupted or times out mid-probe a trailing revert never executes and the
# tree is left dirty. The trap restores scripts/ on ANY exit path.
trap 'git checkout -- scripts/docket-status.sh; echo "[trap] scripts/ restored"' EXIT INT TERM

probe(){   # $1 = the line to inject above the real invocation
  python3 - "$1" <<'PY'
import sys
p='scripts/docket-status.sh'; s=open(p).read()
a='  if ! out="$("$SCRIPTS_DIR"/render-board.sh --changes-dir "$cd_dir" --format digest 2>&2)"; then'
assert a in s, 'ANCHOR NOT FOUND — the mutation would be a silent no-op that "proves" the guard by never testing it'
s=s.replace(a, '  '+sys.argv[1]+'\n'+a, 1)
open(p,'w').write(s)
PY
  grep -n 'cat >' scripts/docket-status.sh || { echo "MUTATION DID NOT LAND"; return 1; }
  bash tests/test_render_board.sh 2>&1 \
    | grep -E "^(ok|NOT OK) - (no script under scripts|scripts/docket-status.sh never)|^(PASS|FAIL)$"
  git checkout -- scripts/docket-status.sh
}

# 1. pipeline, LITERAL /BOARD.md target — both guards should fire.
probe '"$SCRIPTS_DIR"/render-board.sh --changes-dir "$cd_dir" | cat > "$cd_dir/BOARD.md"'

# 2. pipeline, VARIABLE target — REDIRECT_RE is BLIND to this, so ONLY Guard 1 can fire.
probe 'rel="$cd_dir/BOARD.md"; "$SCRIPTS_DIR"/render-board.sh --changes-dir "$cd_dir" | cat > "$rel"'

git status --porcelain scripts/                 # expected: EMPTY
bash tests/test_render_board.sh 2>&1 | tail -1  # expected: PASS
```

Expected — probe 1 reds **both** guards; probe 2 reds **only Guard 1**, and that second result is the load-bearing one: it shows Guard 1 is wired to reality *on its own*. (Probe 2 is also direction A of the complementarity, observed on the live tree rather than a fixture.)

```
--- probe 1 (literal target)
NOT OK - no script under scripts/ (except board-refresh.sh) writes render-board.sh's stdout to a file
NOT OK - scripts/docket-status.sh never redirects render-board.sh stdout into BOARD.md
FAIL
--- probe 2 (variable target — REDIRECT_RE cannot see it)
NOT OK - no script under scripts/ (except board-refresh.sh) writes render-board.sh's stdout to a file
ok - scripts/docket-status.sh never redirects render-board.sh stdout into BOARD.md
FAIL
```

Then `git status --porcelain scripts/` must print **nothing** and the suite must return to `PASS` — the production tree is not modified by this change.

- [ ] **Step 5: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/redirect-regex-board-write-guard
git add tests/test_render_board.sh
git commit -m "test(0070): repo-wide render-board.sh write sentinel + mutation battery"
```

---

### Task 2: Guard 2 — re-derive `REDIRECT_RE`'s comment; KEEP both of its scans

**Files:**
- Modify: `tests/test_render_board.sh` — the `# --- negative sentinel:` comment block, and *only* the comment. `REDIRECT_RE`, `HISTORICAL_REDIRECT`, the positive control, the `skills/*/SKILL.md` scan and the `scripts/docket-status.sh` scan all stay **byte-identical**. Two assertions are ADDED.

**THE SPEC SAID TO DELETE THE `scripts/docket-status.sh` SCAN. DO NOT.** The spec's Guard-2 design rested on Guard 1 *subsuming* it. Task 1's mutation testing **disproved that premise**: Guard 1 is token-scoped and is structurally blind to a write that crosses a statement boundary (`{ …; } > f`, capture-then-write, a wrapper function), and `REDIRECT_RE`'s flattened whole-file scan is the only thing in the suite that catches those in a *script*. Deleting it would reopen exactly the hole the change exists to close. The COMPLEMENTARITY block asserts this; if you delete the scan, three of its rows lose their point and the next author has no way to know why the regex is shaped as it is.

**Interfaces:**
- Consumes: Task 1's COMPLEMENTARITY block, which asserts — against this very `$REDIRECT_RE` — that the statement-boundary shapes are RED here and GREEN under `render_board_write_free`. That passing block is the evidence for the paragraph this task writes.
- Produces: nothing new; `REDIRECT_RE` and `HISTORICAL_REDIRECT` keep their exact names and values.

- [ ] **Step 1: Re-derive the comment; add the blockquote control and the anti-vacuity check**

Replace the comment block above `REDIRECT_RE` (from `# --- negative sentinel: no skill body may redirect …` down to the `REDIRECT_RE=` line) with the block below, and add the two new assertions where shown. The regex, the positive control, and **both** scans are unchanged.

```bash
# --- Guard 2: REDIRECT_RE — the WHOLE-FILE, TARGET-KEYED sentinel (re-derived by change 0070) --
# It scans TWO domains and it is load-bearing in BOTH — do not narrow it to one:
#   1. skills/*/SKILL.md PROSE — no skill body may show the pre-0059 anti-pattern
#      `render-board.sh ... > .../BOARD.md`.
#   2. scripts/docket-status.sh SHELL — kept from change 0069, and NOT retired by 0070. Guard 1
#      (the write sentinel above) is TOKEN-SCOPED: it reads one invocation and the pipeline it
#      feeds, so it is STRUCTURALLY BLIND to a write that crosses a STATEMENT boundary —
#      `{ render-board.sh ...; } > f`, `out=$(render-board.sh ...); printf ... > f`, or a wrapper
#      function. Each really writes the board. THIS regex catches them, because it flattens the
#      file with `tr` and spans 200 characters across the boundary Guard 1 stops at. The
#      COMPLEMENTARITY block below PROVES both directions by mutation. Neither guard subsumes the
#      other; deleting either reopens a hole. ONE GUARD, ONE HOLE.
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
#     (`
> ` -> ` > `) that lands a bare "BOARD.md" prose word inside the window. Blockquotes
#     genuinely appear in docket-status and docket-implement-next: a LIVE false-positive class,
#     asserted below rather than merely asserted in a comment.
#
# WHY IT STAYS NARROW: the shapes that force it to be narrow (spaces around `>`, a literal target)
# are prose hazards, and the shapes a bash script has (`>"$f"`, `>>`, `>|`, `&>`, a VARIABLE
# target) are ones it can never see AT ANY WIDTH. That is Guard 1's job, and Guard 1 does it for
# EVERY script rather than the one that happened to be named. DO NOT widen this regex to chase
# shell forms, and DO NOT delete it as redundant — it is the only net under the statement-boundary
# class.
#
# THE SAME REGEX serves the positive control and both scans, so weakening it (e.g. back to
# `[^>]*`) trips the positive control loudly rather than silently hollowing out a scan.
```

Then, immediately after the existing `HISTORICAL_REDIRECT` positive-control assertion, ADD the blockquote false-positive control:

```bash
# Negative control (change 0070): a flattened markdown blockquote must NOT trip the regex. This is
# the false-positive class that keeps it narrow — assert it, don't just describe it. The
# `/BOARD\.md` slash requirement is what saves this string: the prose word is a bare "BOARD.md".
BLOCKQUOTE_PROSE='Run render-board.sh to regenerate the board. > Never hand-edit BOARD.md — it is generated.'
assert "guard regex does NOT flag a flattened markdown blockquote (false-positive control)" \
  '! printf "%s" "$BLOCKQUOTE_PROSE" | tr "\n" " " | grep -Eq "$REDIRECT_RE"'
```

And immediately after the `skills/*/SKILL.md` scan's assertion, ADD its anti-vacuity check (Guard 1's scan has one; this one did not):

```bash
# Anti-vacuity: the skills glob must be non-empty, or the scan above passes for the wrong reason.
assert "the skills/*/SKILL.md scan is not vacuous" \
  '[ "$(ls "$REPO"/skills/*/SKILL.md | wc -l)" -ge 5 ]'
```

- [ ] **Step 2: Run the test to verify it passes**

Run: `cd /Users/homer/dev/docket/.worktrees/redirect-regex-board-write-guard && bash tests/test_render_board.sh 2>&1 | grep -E "guard regex|skills/|vacuous|docket-status.sh never|^(PASS|FAIL)"`

Expected:
```
ok - guard regex flags the historical bracket-placeholder redirect (positive control)
ok - guard regex does NOT flag a flattened markdown blockquote (false-positive control)
ok - no skills/*/SKILL.md redirects render-board.sh stdout directly into BOARD.md
ok - the skills/*/SKILL.md scan is not vacuous
ok - scripts/docket-status.sh never redirects render-board.sh stdout into BOARD.md
PASS
```

The `scripts/docket-status.sh never redirects …` row MUST still be present. If it is gone, the scan was deleted and the statement-boundary class is now guarded by nothing.

- [ ] **Step 3: Verify the regex and both scans are byte-identical to `origin/main`**

Run:
```bash
cd /Users/homer/dev/docket/.worktrees/redirect-regex-board-write-guard
diff <(git show origin/main:tests/test_render_board.sh | grep '^REDIRECT_RE=') \
     <(grep '^REDIRECT_RE=' tests/test_render_board.sh) && echo "REDIRECT_RE unchanged"
grep -c "scripts/docket-status.sh never redirects" tests/test_render_board.sh   # expected: 1
```
Expected: `REDIRECT_RE unchanged`, and the script scan still present. A diff on the regex means the constraint was violated — restore the original bytes.

- [ ] **Step 4: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/redirect-regex-board-write-guard
git add tests/test_render_board.sh
git commit -m "test(0070): re-derive REDIRECT_RE's comment; keep both of its scans

Mutation testing disproved the spec's premise that the write sentinel
subsumes this regex's scan of scripts/docket-status.sh. The sentinel is
token-scoped and cannot see a write that crosses a statement boundary;
this flattened whole-file scan can, and is the only thing that does. The
scan stays, the regex is unchanged byte-for-byte, and the complementarity
is locked by tests. Adds the blockquote false-positive control and an
anti-vacuity check the skills scan lacked."
```

---

### Task 3: Guard 3 — continuation-joining for the `--format digest` flag check

**Files:**
- Modify: `tests/test_docket_status.sh:12-33` — the inline-board wiring sentinel: add `join_continuations` and pipe the script through it before the comment filter and tokenizer.

**Interfaces:**
- Consumes: the `join_continuations` contract defined in Task 1 (identical twin, defined locally — this file is a standalone script and shares no library with `tests/test_render_board.sh`, matching how every test file in this repo defines its own `assert`).
- Produces: nothing consumed downstream.

- [ ] **Step 1: Write the failing test — a continuation-line mutation of the flag check**

The existing tokenizer reads one physical line at a time, so a legitimate invocation whose `--format digest` flag sits on a continuation line is a **false positive** (loud, not silent) — and a rogue one hiding behind a continuation is a false negative. Both die with the join.

Append this block immediately after the existing `assert "every render-board.sh invocation in docket-status is the read-only --format digest"` assertion in `tests/test_docket_status.sh` (this is the test; Step 3 adds the function it calls and rewires the scan):

```bash
# --- change 0070: the flag check tokenizes LOGICAL lines, not physical ones -------------------
# The tokenizer below reads one PHYSICAL line at a time, so an invocation split across a
# backslash-continuation is torn in half: the first-line token carries the call WITHOUT its
# --format digest flag (false positive — loud), and any redirect parked on the continuation is
# invisible (false negative — silent). Mutation-test both directions on fixtures, using the SAME
# function the real scan uses, so the fix cannot be true here and broken there.
# TWIN: join_continuations is defined identically in tests/test_render_board.sh (Guard 1). Keep
# the two in step — a broken tokenizer sitting beside a fixed one is how the next author
# cargo-cults the broken one (ledger #64).
digest_tokens(){
  join_continuations "$1" \
    | grep -v '^[[:space:]]*#' \
    | grep -oE '[^;&|]*/render-board\.sh[^;&|]*' || true
}

# A legitimate call whose flag sits on the continuation line: exactly ONE logical invocation, and
# it IS the digest projection. The join is what lets the tokenizer see that.
ct="$tmp/continuation-call.sh"
printf '%s\n' '#!/usr/bin/env bash' \
  'out="$("$SCRIPTS_DIR"/render-board.sh --changes-dir "$cd_dir" \' \
  '  --format digest 2>&2)"' > "$ct"
ct_ungated=0
while IFS= read -r inv; do
  [ -n "$inv" ] || continue
  case "$inv" in
    *"--format digest"*) : ;;
    *) ct_ungated=1 ;;
  esac
done < <(digest_tokens "$ct")
assert "flag check sees a --format digest flag parked on a continuation line (no false positive)" \
  '[ "$ct_ungated" -eq 0 ]'

# The same shape WITHOUT the flag must still be caught — the join must not launder a rogue call.
rt="$tmp/continuation-rogue.sh"
printf '%s\n' '#!/usr/bin/env bash' \
  'out="$("$SCRIPTS_DIR"/render-board.sh --changes-dir "$cd_dir" \' \
  '  2>&2)"' > "$rt"
rt_ungated=0
while IFS= read -r inv; do
  [ -n "$inv" ] || continue
  case "$inv" in
    *"--format digest"*) : ;;
    *) rt_ungated=1 ;;
  esac
done < <(digest_tokens "$rt")
assert "flag check still catches an ungated call split across a continuation line" \
  '[ "$rt_ungated" -eq 1 ]'
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd /Users/homer/dev/docket/.worktrees/redirect-regex-board-write-guard && bash tests/test_docket_status.sh 2>&1 | grep -E "continuation|^(PASS|FAIL)"`

Expected: FAIL — `bash: join_continuations: command not found` from `digest_tokens`, so both new assertions report `NOT OK`.

- [ ] **Step 3: Add `join_continuations` and rewire the real scan through it**

In `tests/test_docket_status.sh`, add the function directly above the existing sentinel block (before the `# --- inline-board wiring sentinel` comment):

```bash
# Folds backslash-continuations into logical lines, so the tokenizer below sees INVOCATIONS rather
# than physical lines. Identical twin in tests/test_render_board.sh (Guard 1) — keep in step.
# awk, not sed: BSD sed does not portably treat `\n` in an s/// LHS as a newline.
join_continuations(){
  awk '{ while (sub(/\\$/, "")) { if ((getline nxt) > 0) { $0 = $0 nxt } else { break } } print }' "$1"
}
```

Then change the real scan's input from the line-oriented pipeline to the joined one. Replace:

```bash
done < <(grep -v '^[[:space:]]*#' "$SCRIPT" | grep -oE '[^;&|]*/render-board\.sh[^;&|]*' || true)
```

with:

```bash
done < <(digest_tokens "$SCRIPT")
```

and move the `digest_tokens` definition from Step 1's block up beside `join_continuations`, so the real scan and the mutation fixtures run the SAME tokenizer (a battery that tests a copy is decoration). Extend the sentinel's existing comment with one line:

```bash
# Change 0070: tokenizes LOGICAL lines (continuations joined first) — a flag or a redirect parked
# on a continuation line is otherwise torn away from the call it belongs to.
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd /Users/homer/dev/docket/.worktrees/redirect-regex-board-write-guard && bash tests/test_docket_status.sh 2>&1 | grep -E "continuation|read-only --format digest|routes the inline|^(PASS|FAIL)"`

Expected:
```
ok - docket-status routes the inline board render through board-refresh.sh
ok - every render-board.sh invocation in docket-status is the read-only --format digest
ok - flag check sees a --format digest flag parked on a continuation line (no false positive)
ok - flag check still catches an ungated call split across a continuation line
PASS
```

- [ ] **Step 5: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/redirect-regex-board-write-guard
git add tests/test_docket_status.sh
git commit -m "test(0070): join continuations before tokenizing the digest flag check

The flag sentinel read one physical line at a time, so an invocation
split across a backslash-continuation was torn in half — the flag (false
positive, loud) or a redirect (false negative, silent) fell off the token.
Joins logical lines first, through the same digest_tokens() the mutation
fixtures use, so the fix cannot be true in the battery and broken in the
scan."
```

---

### Task 4: Whole-suite verification against the base

**Files:**
- Modify: none (verification only).

**Interfaces:**
- Consumes: Tasks 1–3, all committed.
- Produces: the evidence the PR rests on.

- [ ] **Step 1: Confirm no production file was touched**

The change is test-only — this is a Global Constraint, and it is cheap to prove.

Run:
```bash
cd /Users/homer/dev/docket/.worktrees/redirect-regex-board-write-guard
git diff --name-only origin/main...HEAD
```
Expected — exactly these, and nothing under `scripts/`:
```
docs/superpowers/plans/2026-07-13-redirect-regex-board-write-guard-plan.md
tests/test_docket_status.sh
tests/test_render_board.sh
```

- [ ] **Step 2: Run the full suite in ONE foreground call**

Per ledger #34/#66: a RED suite is a hypothesis, not a verdict — and per the docket loop's own rule, the suite runs in the FOREGROUND (a backgrounded ~10-minute suite stalls the loop).

Run (single Bash call, `timeout 600000`):
```bash
cd /Users/homer/dev/docket/.worktrees/redirect-regex-board-write-guard
for t in tests/*.sh; do
  printf '=== %s: ' "$t"
  if out="$(bash "$t" 2>&1)"; then printf 'PASS\n'; else printf 'FAIL\n'; printf '%s\n' "$out" | tail -20; fi
done
```
Expected: every test PASS.

- [ ] **Step 3: If anything is RED, differential it against the unmodified base BEFORE calling it a regression**

An ambient `DOCKET_SCRIPTS_DIR` in the dev shell, or an environment fact (`origin/HEAD`, umask, timeouts), has twice produced a RED suite that was not a regression (ledger #34/#66).

```bash
cd /Users/homer/dev/docket        # the primary tree, on main
for t in tests/<the-failing-test>.sh; do env -u DOCKET_SCRIPTS_DIR bash "$t" >/dev/null 2>&1 \
  && echo "base PASS: $t" || echo "base FAIL: $t"; done
```
If the same test fails on unmodified `origin/main`, it is environment-bound, not a regression — record the differential in the results file. If it passes on base and fails on the branch, it IS a regression: fix it before the PR.

- [ ] **Step 4: Commit (only if Step 3 required a fix)**

```bash
cd /Users/homer/dev/docket/.worktrees/redirect-regex-board-write-guard
git add -A tests/
git commit -m "test(0070): fix <the regression Step 3 surfaced>"
```

---

---

## Self-Review

**1. Spec coverage.**

| Spec requirement | Task |
|---|---|
| Guard 1: repo-wide write sentinel, `scripts/*.sh` minus `board-refresh.sh`, glob-derived | Task 1 (widened to a `find` sweep so `scripts/lib/*.sh` is covered) |
| Guard 1: strip comments → join continuations → tokenize per invocation | Task 1, Step 1. **The spec's order (join, then strip) is INVERTED here, deliberately:** bash comments are physical-line scoped, so a comment ending in `\` does not continue — joining first folds a LIVE, redirecting invocation into a dead comment and the comment drop deletes both, laundering the write. Comment stripping covers TRAILING comments too; plus the `&>` normalization, the `|&`/`||` normalization, and the fd-dup erasure the `&`-splitting tokenizer makes mandatory |
| Guard 1: no file-directed redirect; fd dups allowed | Task 1, Step 1, stages 5+7. "fd dups allowed" is keyed on the TARGET being a WHOLE, real descriptor (`[0-9]+` or `-`, **terminated by a right boundary**), never on the `>&` operator |
| Guard 2: `REDIRECT_RE` kept unwidened | Task 2 (byte-identity asserted in Step 3) |
| Guard 2: re-scoped to `skills/*/SKILL.md` ONLY, 0069's script scan retired as subsumed | **NOT DONE — the premise is false. See Deviations (1).** The script scan is KEPT |
| Guard 3: flag check stays, inherits continuation-joining | Task 3 |
| Mutation battery: every evasion RED, `2>&2` GREEN | Task 1, Step 1 (21 RED rows, 9 GREEN controls, 13 complementarity rows) + Step 2 (guard-level mutations) + Step 4 (live-tree probes) |
| Test-only; no production changes | Global Constraints; asserted in Task 4, Step 1 |

**2. Placeholder scan.** No TBDs; every step carries the literal bytes to paste and the exact command with its expected output. Task 1's two code fences are byte-identical to what ships.

**3. Type consistency.** `join_continuations <file>` and `render_board_write_free <file>` are named and used identically in Tasks 1 and 3; `digest_tokens <file>` is defined once in Task 3 and used by both its fixtures and its real scan. The fd-dup control string is copied verbatim from `scripts/docket-status.sh:172`.

**Deviations from the spec, and why** — to disclose in the results file:

1. **THE SPEC'S SUBSUMPTION PREMISE IS FALSE; `REDIRECT_RE` KEEPS ITS SCAN OF `scripts/docket-status.sh`.** The spec said Guard 2's regex would scan *only* skills prose, because 0069's script scan would be "retired as subsumed" by the new write sentinel. Mutation testing disproved it: the write sentinel is **token-scoped** and is structurally blind to a write that crosses a **statement boundary** — `{ render-board.sh …; } > f`, `out=$(render-board.sh …); printf … > f`, and a wrapper function all really write `BOARD.md` (executed against a stub renderer; the bytes arrive) and all are GREEN under it. The flattened `REDIRECT_RE` scan catches all three. The two guards are **complementary, not nested** — neither subsumes the other, and deleting either reopens a hole. Widening the sentinel's token to cover the gap would just rebuild `REDIRECT_RE` badly, against this repo's own ledger rule (*one guard, one hole — add an independent scan rather than widening the first; deleting a sentinel is how the guarded hole reopens*). The complementarity is now **locked by tests** in both directions, against the real regex and the real function, so the claim cannot rot back into a comment.
2. **`find "$REPO/scripts" -name '*.sh'` instead of a flat `scripts/*.sh` glob.** The spec says "iterate `scripts/*.sh`"; the repo has `scripts/lib/*.sh`, which that glob does not reach. Ledger #64 is explicit that a gated operation's call-site list must be derived, not hand-shaped.
3. **Normalizing `&>`/`&>>`, then the `|`-compounds, then erasing fd dups, ALL BEFORE tokenizing** — not in the spec's three-step pipeline, but forced by it. The tokenizer splits on `;` and `&`. That means (a) the `&` in `2>&2` cuts the token mid-redirect, leaving a dangling `>` that would fire on the codebase's own correct invocation (the spec's designated GREEN control); (b) the `&` in `&>f` ends the token *before* its `>`, so a real file write vanishes; and (c) the `&` in `|&` does the same to a pipeline — while `||`, which merely *looks* like a pipe, must CUT the token, since the OR-branch's redirect belongs to a different command.
4. **The fd-dup erasure keys on the TARGET being a WHOLE descriptor, not on the operator and not on a PREFIX.** `>&2`, `2>&1`, and `>&-` are descriptor dups; `>& file`, `>&"$f"`, and `>&2board.md` are file WRITES with the same operator. The erasure therefore requires a digit run or `-` after the `&` **and a right boundary after that**: `[0-9]*>&([0-9]+|-)($|[[:space:];&|)}"])`. The `|` in that boundary class is pinned by the no-space `2>&1| head` row (the spaced twin cannot detect its removal).
5. **Trailing comments are stripped, not just whole-line ones.** `grep -v '^[[:space:]]*#'` alone leaves `render-board.sh … 2>&2  # digest -> stdout only` carrying the comment's arrow, whose `>` reddens the guard on a legitimate call. Disclosed trade-off: a `#`, `;`, `&` or `||` inside a quoted argument on an invocation line would truncate the token early. No such invocation exists (the only args are `--changes-dir`, `--repo`, `--format`).
6. **Guard 1 rejects `2>/dev/null`** (a stderr-to-file redirect). Strictly implied by "no file-directed redirect", but worth stating.
7. **Known, accepted gaps — all disclosed in the guard's comment, none fixed:** `| tee f`; a redirect opened on a prior line (`exec 3>"$f"` + `>&3`, or `exec > "$f"` + a bare invocation); an operator conjured by `eval`; and a metacharacter inside a quoted argument. Plus two **fail-safe false positives** (they can only add an alarm, never hide a write): a heredoc body quoting the anti-pattern as prose, and the join reaching across a comment bash would treat as ending the command. The deferred filesystem-effect test is the answer to the gaps.
