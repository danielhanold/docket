<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0246 — Make the docket-example-yml guard suite bite](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0246-make-the-docket-example-yml-guard-suite-bite.md)**
<!-- docket:backlink:end -->

# Make the docket-example-yml guard suite bite — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `tests/test_docket_example_yml.sh` actually execute all 393 of its asserts on every invocation path, and make four of its guards stop passing vacuously — the truncation that hides 290 asserts, the one-way mirror loop, the round-trip slice that stops two harness blocks early, and the `elsewhere:` word-grep that a sentence of English prose satisfies.

**Architecture:** Three sequential parts over two existing test files, in a binding order. Part 1 lands a Bash>=4 fail-fast prologue (without it, parts 2 and 3 edit a region that never executes under a system-first `PATH`, so their green runs prove nothing), converts three non-portable `\b` patterns to explicit character-class boundaries, and adds a source-portability guard class that keeps the escaped form from coming back. Part 2 repairs the mirror and round-trip guards in the now-reachable region. Part 3 replaces the `elsewhere:` bare-word grep with a shape-tightened, evidence-derived match plus one named exemption.

**Tech Stack:** Bash 4+ shell test scripts (`assert`-style, no framework), POSIX ERE through PATH `grep`, `sed`/`awk` text extraction, `scripts/lib/harness-defaults.sh` as the sidecar reader, `scripts/run-tests.sh` as the suite runner.

## Global Constraints

- **Bash floor for the edited test file is 4** (not 4.3). `scripts/run-tests.sh` has its own 4.3 floor for `wait -n`; this file needs only 4. Do not restate 4.3 in the new prologue.
- **No `\b`, `\<`, or `\>` in any ERE this plan writes.** BSD grep's and git-grep's ERE do not support them and return zero silently. Use explicit `[^[:alnum:]_]` character classes.
- **No new `\\b` / `\\<` / `\\>` (escaped double-quoted) source form anywhere** — Task 3 adds a guard that reddens on it, and that guard scans itself.
- **Never hand-write a literal that the sidecar can supply.** Terminators, harness lists, model IDs, and row counts are derived from `agents/harness-defaults.yml` via `scripts/lib/harness-defaults.sh` (`HD_SHIPPED_HARNESSES`, `hd_agents`, `hd_field`), never typed.
- **Never restate a count in prose.** Replace "all thirty-nine rows" with derived phrasing; do not substitute a new number.
- **Mutation-prove every new guard.** A new assert is not done until you have watched it go red on a deliberate mutation and then restored the tree. Restore by `cp` from a backup copy taken before the mutation — **never `git checkout -- <file>`**, which restores to HEAD and destroys the uncommitted work under test (learnings: `mutation-restore-needs-a-backup-copy`).
- **`tests/test_docket_example_yml.sh` currently has 393 asserts, all green** under `/opt/homebrew/bin/bash`. Every task must leave the file green; assert counts only grow.
- Run the file directly during development with an explicit Bash 4+ binary: `/opt/homebrew/bin/bash tests/test_docket_example_yml.sh`. The full suite gate at the end is `scripts/run-tests.sh`.
- Repo root for all paths: the feature worktree `.worktrees/make-the-docket-example-yml-guard-suite-bite`.

---

## File Structure

| File | Responsibility | Tasks |
|---|---|---|
| `tests/test_docket_example_yml.sh` | The guard suite under repair. Gains a version prologue (T1), portable EREs (T2), a reverse mirror loop + boundary-class terminator (T4), a re-derived round-trip slice (T5), and a shape-tightened `elsewhere:` match (T6). | 1, 2, 4, 5, 6 |
| `tests/test_grep_portability.sh` | Static source-portability scanner. Gains a second banned class for the escaped `\\b`/`\\<`/`\\>` form. | 3 |

No files are created. No production code changes — this change is entirely test-surface repair.

---

### Task 1: Bash>=4 fail-fast prologue

**Why:** With `PATH=/usr/bin:/bin`, `bash` is `/bin/bash` 3.2.57, whose `$(...)` parser cannot see heredocs. At line 684, `scope_guard_awk="$(cat <<'SCOPE_GUARD_AWK' ...)"` has a backtick inside its heredoc body (line 688), and 3.2 scans that body as shell. Everything from 684 to EOF dies: 103 of 393 asserts run, then a cryptic `unexpected EOF while looking for matching \``. Bash 3.2 parses incrementally, so a prologue at the top of the file runs before the line-684 construct is ever parsed.

**Files:**
- Modify: `tests/test_docket_example_yml.sh:6` (immediately after `set -uo pipefail`)

**Interfaces:**
- Consumes: nothing.
- Produces: the guarantee every later task relies on — that a run either executes all asserts or exits 2 with a named reason. No shell functions or variables are exported.

- [ ] **Step 1: Reproduce the failure and record the baseline**

```bash
cd .worktrees/make-the-docket-example-yml-guard-suite-bite
PATH=/usr/bin:/bin bash tests/test_docket_example_yml.sh 2>&1 | tail -3
PATH=/usr/bin:/bin bash tests/test_docket_example_yml.sh 2>/dev/null | grep -c '^ok'
```

Expected: the tail shows `line 1710: unexpected EOF while looking for matching \`` and `line 1717: syntax error: unexpected end of file`; the count is `103`.

- [ ] **Step 2: Record the healthy baseline**

```bash
/opt/homebrew/bin/bash tests/test_docket_example_yml.sh 2>&1 | grep -c '^ok'
/opt/homebrew/bin/bash tests/test_docket_example_yml.sh 2>&1 | grep -c '^NOT OK'
```

Expected: `393` and `0`. Write both numbers down — every later task re-checks them.

- [ ] **Step 3: Insert the prologue**

Insert immediately after the existing `set -uo pipefail` on line 6, before `REPO=`:

```bash
# --- BASH VERSION GATE (change 0246) -------------------------------------------------------------
# THIS MUST STAY AT THE TOP OF THE FILE, above every function and heredoc.
#
# WHY: bash 3.2's $(...) parser cannot see heredocs — it scans the heredoc BODY as shell. The
# scope_guard_awk assignment below (`scope_guard_awk="$(cat <<'SCOPE_GUARD_AWK'`) has a backtick
# inside a comment in its body, so under 3.2 the whole file from that point to EOF fails to parse.
# Observed directly: `PATH=/usr/bin:/bin bash tests/test_docket_example_yml.sh` ran 103 of this
# file's asserts, printed zero failures, then died with "unexpected EOF while looking for matching
# `" and exit 2. The 290 asserts that never ran include the ENTIRE mirror and round-trip family.
#
# scripts/run-tests.sh never hits this — it re-execs itself under a Bash 4.3+ runtime and runs every
# test file with $TEST_BASH. The exposed path is DIRECT invocation, which is this file's own
# documented run line (see the header above), on any machine whose PATH resolves bash to 3.2 first
# (stock macOS). Bash parses incrementally, so this gate executes before the line-684 construct is
# ever parsed — which is exactly why it must not be moved down or wrapped in a function.
#
# The floor here is 4, not run-tests.sh's 4.3: that file needs `wait -n`, this one does not.
if [ "${BASH_VERSINFO[0]:-0}" -lt 4 ]; then
  printf '%s\n' "test_docket_example_yml.sh requires bash >= 4 (running ${BASH_VERSION:-unknown} from ${BASH:-unknown}). Bash 3.2 cannot parse this file's heredoc-in-\$() constructs and silently skips most of its asserts. Re-run with a bash 4+ binary, or use scripts/run-tests.sh." >&2
  exit 2
fi
```

- [ ] **Step 4: Verify the gate fires on bash 3.2 and is silent on bash 5**

```bash
PATH=/usr/bin:/bin bash tests/test_docket_example_yml.sh; echo "rc=$?"
```

Expected: exactly one line naming `requires bash >= 4`, no `ok -` lines at all, `rc=2`.

```bash
/opt/homebrew/bin/bash tests/test_docket_example_yml.sh 2>&1 | grep -c '^ok'
/opt/homebrew/bin/bash tests/test_docket_example_yml.sh 2>&1 | grep -c '^NOT OK'
```

Expected: `393` and `0` — unchanged from Step 2.

- [ ] **Step 5: Confirm the runner path is unaffected**

```bash
/opt/homebrew/bin/bash scripts/run-tests.sh 2>&1 | tail -5
```

Expected: the suite completes and reports green. (This is a slow run; it is worth doing once here because Task 1 is the task that changes how the file starts.)

- [ ] **Step 6: Commit**

```bash
git add tests/test_docket_example_yml.sh
git commit -m "test(0246): fail fast under bash 3.2 instead of skipping 290 asserts

Bash 3.2's \$() parser cannot see heredocs, so the scope_guard_awk
assignment killed everything from line 684 to EOF: 103 of 393 asserts
ran, then a cryptic unexpected-EOF error. run-tests.sh re-execs under
Bash 4.3+ and never saw it; direct invocation (this file's own
documented run line) did. Gate at the top, floor 4, exit 2."
```

---

### Task 2: Convert the three `\b` patterns to portable EREs

**Why:** `\b` is a GNU extension. BSD grep's ERE and git-grep's ERE do not support it and **return zero matches silently** — a guard written with it can go blind rather than red on another machine. Three sites use the escaped double-quoted form `\\b`; they are the only such sites in the tracked tree outside `docs/`, which is what makes Task 3's guard land on a clean tree.

Note on the reframed root cause: current macOS `/usr/bin/grep` *does* accept ERE `\b` (probed live), and none of these three sites was observed failing at runtime. They are converted for **source portability**, not to fix an observed break. The ~26 single-quoted `\b` sites elsewhere in `tests/` are deliberate, comment-blessed PATH-grep idiom and are explicitly **out of scope** — do not touch them.

**Files:**
- Modify: `tests/test_docket_example_yml.sh:376` (resolved-arm correspondence grep)
- Modify: `tests/test_docket_example_yml.sh:409` (`elsewhere:` consumer-mention grep)
- Modify: `tests/test_docket_example_yml.sh:585` (orphan-key grep)

**Interfaces:**
- Consumes: Task 1's version gate (this region is above line 684 and did run under 3.2, but every later verification depends on the whole file running).
- Produces: a tree with zero `\\b`/`\\<`/`\\>` escaped-form occurrences outside `docs/` — the precondition Task 3's guard asserts. Line 409's grep is replaced wholesale by Task 6; converting it here anyway is required so Task 3 can land on a clean tree between the two.

All three replacements were prototyped against the real tree and verified to leave the file at 393 ok / 0 NOT OK.

- [ ] **Step 1: Confirm the three sites are exactly where the plan says**

```bash
grep -n '\\\\b' tests/test_docket_example_yml.sh
```

Expected exactly three lines — 376, 409, 585 (line numbers shift by the ~20 lines Task 1 added; match on content, not number):

```
grep -qE "^$exp_name=.*\\b$leaf_k\\b" "$CFGSCRIPT" \
grep -qE "\\b$leaf_k\\b" "$REPO/$consumer" \
grep -qlE "\\b$k\\b" $consumers >/dev/null 2>&1 || orphan_keys="$orphan_keys $k"
```

- [ ] **Step 2: Replace site 1 — the resolved-arm correspondence grep**

Find:

```bash
          grep -qE "^$exp_name=.*\\b$leaf_k\\b" "$CFGSCRIPT" \
```

Replace with:

```bash
          # Boundary by explicit class, never \b: BSD grep's and git-grep's ERE do not support \b
          # and return zero SILENTLY, so a \b guard goes blind rather than red off-GNU (change
          # 0246). The leading class needs no (^|...) alternative — `^$exp_name=.*` already
          # guarantees at least one character precedes the key.
          grep -qE "^$exp_name=.*[^[:alnum:]_]$leaf_k([^[:alnum:]_]|$)" "$CFGSCRIPT" \
```

- [ ] **Step 3: Replace site 2 — the `elsewhere:` consumer-mention grep**

Find:

```bash
        grep -qE "\\b$leaf_k\\b" "$REPO/$consumer" \
```

Replace with:

```bash
        grep -qE "(^|[^[:alnum:]_])$leaf_k([^[:alnum:]_]|$)" "$REPO/$consumer" \
```

- [ ] **Step 4: Replace site 3 — the orphan-key grep**

Find:

```bash
  grep -qlE "\\b$k\\b" $consumers >/dev/null 2>&1 || orphan_keys="$orphan_keys $k"
```

Replace with:

```bash
  grep -qlE "(^|[^[:alnum:]_])$k([^[:alnum:]_]|$)" $consumers >/dev/null 2>&1 || orphan_keys="$orphan_keys $k"
```

- [ ] **Step 5: Verify no escaped-form sites remain and the suite is unchanged**

```bash
grep -c '\\\\b' tests/test_docket_example_yml.sh || echo "0 (grep exit 1 = none)"
/opt/homebrew/bin/bash tests/test_docket_example_yml.sh 2>&1 | grep -c '^ok'
/opt/homebrew/bin/bash tests/test_docket_example_yml.sh 2>&1 | grep '^NOT OK'
```

Expected: no `\\b` occurrences; `393`; no `NOT OK` output.

- [ ] **Step 6: Mutation-prove the converted boundaries still discriminate**

The point of a boundary is that a *substring* match must not satisfy it. Prove site 1 still binds the export to its own key by breaking that binding — the same mutation the original guard was written for.

```bash
cp tests/test_docket_example_yml.sh /tmp/0246-t2-backup.sh
# Point a resolved: arm at an export whose assignment does NOT name this leaf key.
sed -i '' "s/    learnings.enabled)            echo 'resolved:LEARNINGS_ENABLED' ;;/    learnings.enabled)            echo 'resolved:BOARD_SURFACES' ;;/" tests/test_docket_example_yml.sh
/opt/homebrew/bin/bash tests/test_docket_example_yml.sh 2>&1 | grep '^NOT OK'
```

Expected: at least one `NOT OK` naming `export is tied back to its key`. If the arm text above does not match verbatim, pick any other `resolved:` arm and repoint it the same way — the requirement is that the correspondence assert reddens.

```bash
cp /tmp/0246-t2-backup.sh tests/test_docket_example_yml.sh && rm /tmp/0246-t2-backup.sh
/opt/homebrew/bin/bash tests/test_docket_example_yml.sh 2>&1 | grep -c '^ok'
```

Expected: `393` restored.

- [ ] **Step 7: Commit**

```bash
git add tests/test_docket_example_yml.sh
git commit -m "test(0246): replace \\b with explicit boundary classes in three greps

BSD grep's and git-grep's ERE do not support \\b and return zero
matches silently, so these guards would go blind rather than red on a
non-GNU grep. Converted to [^[:alnum:]_] classes; correspondence
discrimination mutation-proven unchanged."
```

---

### Task 3: Ban the escaped `\\b` / `\\<` / `\\>` source form

**Why:** Task 2 removed the three occurrences; nothing stops the fourth. `tests/test_grep_portability.sh` already owns exactly this kind of rule for over-255 repetition bounds — a static source scan over every tracked file minus `docs/`, with a positive, boundary, and negative control, and a self-membership assert that forces its own literals to be assembled at runtime.

**Scope is the escaped double-quoted form only** (two source backslashes). The single-quoted `\b` form is deliberately **not** banned: ~26 tracked sites use it as blessed PATH-grep idiom, and banning it would redden healthy code on the guard's first run.

**Files:**
- Modify: `tests/test_grep_portability.sh` — header comment, a new pattern constant, a second scan function, the main loop, and new controls.

**Interfaces:**
- Consumes: a tree with zero escaped-form occurrences outside `docs/` (Task 2's deliverable). If Task 2 is not committed, this guard is red on arrival.
- Produces: `scan_word_boundary(file)` → prints `lineno:match` lines; a `wb_violations` accumulator; four new asserts. Nothing is exported to other files.

- [ ] **Step 1: Write the failing controls first**

The existing file has no word-boundary class at all, so the honest TDD move is to add the *controls* (which must fire) before the scan they exercise. Append this block immediately after the existing negative control (after the `neg=` assert, before `exit "$fail"`) — it will fail because `scan_word_boundary` does not exist yet:

```bash
# --- word-boundary class controls (change 0246) --------------------------------------------------
wb_over="$tmp/wb-bad.txt"
wb_clean="$tmp/wb-ok.txt"
# ASSEMBLED AT RUNTIME, exactly like the MAX_BOUND fixtures above and for the same reason: this
# guard is in its own scanned population (see the self-membership assert), so writing the banned
# form literally here would make the guard fail its own scan.
BS='\'
printf 'grep -qE "%s%sb$k%s%sb" "$f"\n' "$BS" "$BS" "$BS" "$BS" > "$wb_over"
printf 'grep -qE "%s%s<$k" "$f"\n'      "$BS" "$BS"             >> "$wb_over"
# Legal neighbours that must NOT be flagged: the blessed single-quoted form, a literal backslash-b
# in a printf, and a doubled backslash not followed by b/</>.
printf "grep -qE '%sb\$k%sb' \"\$f\"\n" "$BS" "$BS" > "$wb_clean"
printf 'printf "%s%sn"\n'               "$BS" "$BS" >> "$wb_clean"

wb_pos="$(scan_word_boundary "$wb_over")"
[ -n "$wb_pos" ] \
  && ok "word-boundary positive control: the escaped \\\\b / \\\\< form is reported" \
  || nok "word-boundary positive control FAILED: the escaped form is not reported — the class is vacuous"

wb_neg="$(scan_word_boundary "$wb_clean")"
[ -z "$wb_neg" ] \
  && ok "word-boundary negative control: the single-quoted \\b idiom and a plain backslash are not flagged" \
  || nok "word-boundary negative control FAILED: a legal single-backslash form was flagged — ~26 blessed sites would redden"
```

- [ ] **Step 2: Run to verify it fails**

Run: `/opt/homebrew/bin/bash tests/test_grep_portability.sh`
Expected: FAIL with `scan_word_boundary: command not found` (and both new controls `NOT OK`).

- [ ] **Step 3: Add the pattern, the scan function, and the header rationale**

Add to the header comment block, after the existing `TRACKED-FILES-ONLY:` paragraph:

```bash
# TWO CLASSES, ONE WALK (change 0246): this file scans for two independent portability defects over
# the same population. (1) an ERE repetition bound above 255. (2) the ESCAPED double-quoted word-
# boundary form — two source backslashes before b, < or > — which is how a \b reaches grep from
# inside a double-quoted shell string. BSD grep's and git-grep's ERE do not support \b/\</\>; they
# return zero matches SILENTLY, so such a guard goes blind rather than red, which is the same
# fail-open shape as the interval class. The SINGLE-backslash form inside single quotes is NOT
# banned: ~26 tracked sites use it as deliberate, comment-blessed PATH-grep idiom (see
# tests/test_docket_build.sh, which blesses them explicitly), and banning it would redden healthy
# code. Change 0246 converted the only three escaped-form sites (all in
# tests/test_docket_example_yml.sh), so the tree is provably clean as this class lands.
```

Add after the `INTERVAL=` / `scan_file` definitions:

```bash
# The escaped double-quoted word-boundary form. Written with an assembled backslash literal for the
# same self-membership reason as the fixtures below: this pattern is itself scanned. Two source
# backslashes, then b, < or >.
WORD_BOUNDARY="$(printf '%s%s[b<>]' '\\' '\\')"

# ONE scan implementation for this class, used by the main loop AND both controls — same discipline
# as scan_file: routing every caller through one function means neutering the scan anywhere neuters
# it everywhere, so a control cannot stay green while the loop goes blind.
scan_word_boundary(){ grep -InoE "$WORD_BOUNDARY" "$1" 2>/dev/null; }
```

- [ ] **Step 4: Run the controls to verify they now pass**

Run: `/opt/homebrew/bin/bash tests/test_grep_portability.sh 2>&1 | grep -i 'word-boundary'`
Expected: both control lines start with `ok   -`.

- [ ] **Step 5: Wire the class into the main loop**

In the `--- the check ---` block, add a `wb_violations` accumulator alongside `violations`. Change:

```bash
violations=""
scanned=0
skipped=""
```

to:

```bash
violations=""
wb_violations=""
scanned=0
skipped=""
```

and inside the `for f in "${FILES[@]}"` loop, after the existing `bad=` / `violations+=` block, add:

```bash
    wb_hits="$(scan_word_boundary "$ROOT/$f")"
    if [ -n "$wb_hits" ]; then
      while IFS= read -r l; do
        wb_violations+="$f:$l"$'\n'
      done <<<"$wb_hits"
    fi
```

Then add the reporting assert immediately after the existing `no ERE repetition bound above` assert:

```bash
if [ -z "$wb_violations" ]; then
  ok "no escaped \\\\b / \\\\< / \\\\> word-boundary form in maintained source"
else
  nok "escaped word-boundary form found — BSD grep and git-grep ERE return zero for these SILENTLY; use an explicit [^[:alnum:]_] class instead:"
  printf '%s' "$wb_violations" | sed 's/^/       /'
fi
```

- [ ] **Step 6: Run to verify the tree is clean and the guard is in its own population**

```bash
/opt/homebrew/bin/bash tests/test_grep_portability.sh
echo "rc=$?"
```

Expected: `rc=0`, and among the `ok` lines both `guard is itself in the scanned population` and `no escaped \\b / \\< / \\> word-boundary form in maintained source`.

- [ ] **Step 7: Mutation-prove the loop (not just the controls) fires**

```bash
cp tests/test_docket_example_yml.sh /tmp/0246-t3-backup.sh
# Reintroduce the banned form at a real tracked site.
python3 - <<'PY'
p = "tests/test_docket_example_yml.sh"
s = open(p).read()
old = '"(^|[^[:alnum:]_])$k([^[:alnum:]_]|$)"'
assert s.count(old) == 1, s.count(old)
open(p, "w").write(s.replace(old, '"' + chr(92)*2 + 'b$k' + chr(92)*2 + 'b"'))
PY
/opt/homebrew/bin/bash tests/test_grep_portability.sh 2>&1 | grep -A2 'NOT OK'
```

Expected: `NOT OK - escaped word-boundary form found` naming `tests/test_docket_example_yml.sh` with a line number.

```bash
cp /tmp/0246-t3-backup.sh tests/test_docket_example_yml.sh && rm /tmp/0246-t3-backup.sh
/opt/homebrew/bin/bash tests/test_grep_portability.sh; echo "rc=$?"
```

Expected: `rc=0`.

- [ ] **Step 8: Commit**

```bash
git add tests/test_grep_portability.sh
git commit -m "test(0246): ban the escaped \\\\b/\\\\</\\\\> word-boundary source form

Second class on the same walk as the >255 interval scan. BSD grep and
git-grep ERE return zero for these silently, so a guard written with
one goes blind rather than red. Scopes to the escaped double-quoted
form only — the single-quoted \\b idiom is blessed repo practice at ~26
sites. Pattern assembled at runtime for self-membership; positive,
negative, and whole-loop mutation controls."
```

---

### Task 4: Reverse mirror loop and a non-prefix-matchable slice terminator

**Why:** Two independent defects in the `(4) MIRROR EQUALITY` block.

*One-way loop.* The loop iterates the **sidecar** (`hd_agents "$HD" "$h"`) and asserts each row exists in the example. That proves `sidecar ⊆ example`. A row present in the example with **no sidecar counterpart** — a stale agent removed from the sidecar and left in the docs, or a typo'd agent name — is structurally invisible. On a file whose entire deliverable is documentation accuracy, the orphan direction is the one that ships a documented agent that does not exist (learnings: `correspondence-guard-runs-one-way`). This correspondence is a **mirror, not a subset** — the example claims to reproduce the shipped defaults value for value — so the reverse loop is mandatory, not the `#107` subset exception.

*Prefix-weak terminator.* `ex_slice` terminates on `build-max:.*$(ere_escape "$bm_model")`. Claude's build-max model is `claude-opus-5`; cursor's is `claude-opus-5-high`. Today claude's own row (example line 388) matches first so the range closes correctly — but delete that row and the range runs on to cursor's line 432, terminating on a *different harness's* row. The existing terminator guard (`grep -q "build-max:" <<<"$last"`) still passes, because the over-wide slice's last line is still a `build-max:` line. A silently over-wide slice with green asserts is the exact fail-open this task closes.

**Files:**
- Modify: `tests/test_docket_example_yml.sh` — the `ex_slice` call site inside the `for h in $HD_SHIPPED_HARNESSES` loop, and the loop body.

**Interfaces:**
- Consumes: `HD_SHIPPED_HARNESSES`, `hd_agents "$HD" "$h"`, `hd_field "$HD" "$h" build-max model` from `scripts/lib/harness-defaults.sh` (already sourced at the top of this block); the existing `ex_slice`, `ex_slice_field`, and `ere_escape` helpers.
- Produces: `ex_agent_keys(slice)` → newline-separated agent key names extracted from an uncommented example slice. Used only within this task's block.

**Verified against the real tree while planning:** the four example harness headers sit at lines 372 (claude), 393 (codex), 416 (cursor), 439 (opencode); `HD_SHIPPED_HARNESSES` is `claude cursor codex opencode`; each block's build-max model is followed by `,` in the example, so the boundary class `[,[:space:]}]` matches all four today.

- [ ] **Step 1: Write the failing reverse-direction assert**

Add `ex_agent_keys` next to `ex_slice_field`:

```bash
# Agent keys carried by an uncommented example slice. The `\{` requirement is what excludes the
# slice's own first line (the bare `<harness>:` header, which has no flow map) without needing to
# special-case it.
ex_agent_keys(){ # $1=slice
  sed -nE 's/^[[:space:]]+([A-Za-z0-9_-]+):[[:space:]]*\{.*/\1/p' <<<"$1"
}
```

Inside the `for h in $HD_SHIPPED_HARNESSES` loop, after the existing `while IFS= read -r a; do ... done < <(hd_agents "$HD" "$h")` loop and before the `every shipped $h entry was checked` floor assert, add:

```bash
  # REVERSE DIRECTION (change 0246). The loop above iterates the SIDECAR, so it proves only
  # sidecar ⊆ example: an agent row sitting in the example with no sidecar counterpart — a stale
  # entry left behind by a removal, or a typo'd agent name — is structurally invisible to it, and
  # to every neighbouring assert too (they are all keyed on sidecar rows). This correspondence is a
  # MIRROR, not a proper subset: the example claims to reproduce the shipped defaults value for
  # value, so the reverse loop is mandatory here. Set membership plus arity; values need no second
  # comparison, because the forward loop already compares both fields row by row.
  hd_keys="$(hd_agents "$HD" "$h")"
  ex_keys="$(ex_agent_keys "$slice")"
  ex_orphans=""
  while IFS= read -r ek; do
    [ -n "$ek" ] || continue
    grep -qxF "$ek" <<<"$hd_keys" || ex_orphans="$ex_orphans $ek"
  done <<<"$ex_keys"
  assert "$h mirror (reverse): every example $h row exists in the shipped sidecar (${ex_orphans:-none orphaned})" \
    '[ -z "$ex_orphans" ]'
  n_ex="$(grep -c . <<<"$ex_keys")"
  n_hd="$(grep -c . <<<"$hd_keys")"
  assert "$h mirror (reverse): example row count equals sidecar row count (example $n_ex, sidecar $n_hd)" \
    '[ "$n_ex" = "$n_hd" ] && [ "$n_ex" -gt 0 ]'
```

- [ ] **Step 2: Run to verify the new asserts pass on the clean tree, then prove they can fail**

```bash
/opt/homebrew/bin/bash tests/test_docket_example_yml.sh 2>&1 | grep 'mirror (reverse)'
```

Expected: eight `ok -` lines (two per harness), all reporting `none orphaned` and equal counts.

```bash
cp .docket.example.yml /tmp/0246-t4-ex-backup.yml
# Phantom row in the example with no sidecar counterpart.
sed -i '' 's|^#     build-max:             { model: claude-opus-5,             effort: high }|&\n#     ghost-agent:           { model: claude-opus-5,             effort: high }|' .docket.example.yml
/opt/homebrew/bin/bash tests/test_docket_example_yml.sh 2>&1 | grep '^NOT OK'
```

Expected: `NOT OK - claude mirror (reverse): every example claude row exists in the shipped sidecar ( ghost-agent)` **and** the arity assert red. This is the mutation the forward loop cannot see — confirm the forward `claude/<agent>: model mirrors` asserts are all still green in the same run.

```bash
cp /tmp/0246-t4-ex-backup.yml .docket.example.yml && rm /tmp/0246-t4-ex-backup.yml
```

- [ ] **Step 3: Verify the forward direction still reddens too**

```bash
cp .docket.example.yml /tmp/0246-t4-ex-backup.yml
# Delete a real example row that the sidecar still has.
sed -i '' '/^#     review-deep:           { model: claude-opus-5-high,        effort: high }/d' .docket.example.yml
/opt/homebrew/bin/bash tests/test_docket_example_yml.sh 2>&1 | grep '^NOT OK' | head -4
cp /tmp/0246-t4-ex-backup.yml .docket.example.yml && rm /tmp/0246-t4-ex-backup.yml
```

Expected: the forward `model mirrors the shipped sidecar` assert reddens (and the reverse arity assert too). If the exact row text above does not match, delete any other claude row verbatim from the file.

- [ ] **Step 4: Add the boundary class to the slice terminator**

Find the terminator construction inside the same loop:

```bash
  slice="$(ex_slice "$h" "build-max:.*$(ere_escape "$bm_model")")"
```

Replace with:

```bash
  # BOUNDARY CLASS on the model (change 0246): without it the terminator is prefix-weak. claude's
  # build-max model is `claude-opus-5` and cursor's is `claude-opus-5-high`, so with claude's own
  # build-max row deleted this range would run past the codex block and close on CURSOR's row — an
  # over-wide slice whose last line is still a `build-max:` line, so the terminator guard below
  # stayed green while the asserts read another harness's values. The example writes every flow map
  # as `{ model: X, effort: Y }`, so a real terminator's model is always followed by a comma;
  # whitespace and `}` are admitted too so a reformat does not falsely redden.
  slice="$(ex_slice "$h" "build-max:.*$(ere_escape "$bm_model")[,[:space:]}]")"
```

- [ ] **Step 5: Run to verify the tree is still green**

```bash
/opt/homebrew/bin/bash tests/test_docket_example_yml.sh 2>&1 | grep -c '^ok'
/opt/homebrew/bin/bash tests/test_docket_example_yml.sh 2>&1 | grep '^NOT OK'
```

Expected: `401` (393 + 8 reverse asserts), no `NOT OK`.

- [ ] **Step 6: Mutation-prove the terminator guard itself now reddens**

This is the assert that was previously green on a wrong slice.

```bash
cp .docket.example.yml /tmp/0246-t4-ex-backup.yml
sed -i '' '/^#     build-max:             { model: claude-opus-5,             effort: high }$/d' .docket.example.yml
/opt/homebrew/bin/bash tests/test_docket_example_yml.sh 2>&1 | grep '^NOT OK' | head -4
```

Expected: `NOT OK - claude mirror: the claude slice was isolated and terminates at its build-max anchor`. Before this step's change, that assert passed on the over-wide slice — confirm the difference by temporarily reverting only the boundary class if the reviewer asks for the before/after reading.

```bash
cp /tmp/0246-t4-ex-backup.yml .docket.example.yml && rm /tmp/0246-t4-ex-backup.yml
/opt/homebrew/bin/bash tests/test_docket_example_yml.sh 2>&1 | grep -c '^ok'
```

Expected: `401`.

- [ ] **Step 7: Commit**

```bash
git add tests/test_docket_example_yml.sh
git commit -m "test(0246): mirror the example back to the sidecar, and bound the terminator

The mirror loop iterated the sidecar only, so an example row with no
sidecar counterpart was invisible to it and to every neighbouring
assert. Added key-set + arity in the reverse direction, mutation-proven
both ways. Separately, the slice terminator was prefix-weak
(claude-opus-5 matches cursor's claude-opus-5-high), so a deleted
build-max row closed the range on another harness's block with the
terminator guard still green; bounded with [,[:space:]}]."
```

---

### Task 5: Re-derive the round-trip slice terminator and cover the rows it was missing

**Why:** The resolver round-trip slices the example with a **hand-written** end anchor — the cursor `finalize-change` literal, at example line 425. The example's blocks run claude (372), codex (393), cursor (416), opencode (439), so that anchor sits above cursor's own build rows (426-432) **and above the entire opencode block** (439-455). Those rows never reach the real resolver: the round-trip proves nothing about them. This is exactly what went stale when change 0192 appended opencode — a hand-written literal in a file whose whole point is that hand-written literals go stale.

**Files:**
- Modify: `tests/test_docket_example_yml.sh` — the `agents_block=` slice, its terminator guard, the stale `thirty-nine rows` comment, and the round-trip's per-harness asserts.

**Interfaces:**
- Consumes: `HD_SHIPPED_HARNESSES`, `hd_field` (already in scope from Task 4's block), `ere_escape` (defined above the mirror loop, so it is in scope here), and `fm` (defined in the mirror block).
- Produces: `last_ex_harness` (the last shipped harness header **in example file order**) and a slice covering all four blocks. Later tasks do not depend on either.

**Verified against the real tree while planning:** deriving the terminator from the last-in-file-order shipped harness yields `opencode` / `openrouter/moonshotai/kimi-k3`; the resulting slice is 85 lines (vs the current ~55), contains all four harness headers, and resolves through the real `sync-agents.sh` at exit 0, generating `.cursor/agents/docket-build-max.md` with `model: claude-opus-5-high`.

- [ ] **Step 1: Show the coverage gap before fixing it**

```bash
grep -n '^# agents:$' .docket.example.yml
awk '/^# agents:$/,/finalize-change:.*cursor-grok-4\.5-high-fast/' .docket.example.yml | wc -l
awk '/^# agents:$/,/finalize-change:.*cursor-grok-4\.5-high-fast/' .docket.example.yml | grep -c 'opencode:'
```

Expected: the slice is ~55 lines and contains **zero** `opencode:` headers — the gap, measured.

- [ ] **Step 2: Derive the terminator from the sidecar and the file's own ordering**

Replace this block:

```bash
agents_block="$(sed -n '/^# agents:$/,/finalize-change:.*cursor-grok-4\.5-high-fast/p' "$EX")"
```

with:

```bash
# TERMINATOR DERIVED, NOT WRITTEN (change 0246). This anchor used to be the hand-written cursor
# finalize-change literal, which sits ABOVE cursor's own build rows and above the entire opencode
# block — so cursor's build/review rows and all sixteen opencode rows never reached the real
# resolver, while every assert below stayed green on the short slice. That is precisely what went
# stale when 0192 appended opencode, so the replacement must not be another literal.
#
# Derivation: find which shipped harness block comes LAST in the example (file order, which is NOT
# HD_SHIPPED_HARNESSES order — the sidecar lists claude cursor codex opencode, the example writes
# claude codex cursor opencode), then anchor on that block's build-max row with the model read from
# the sidecar. build-max is the ladder's top rung and closes every block.
last_ex_harness=""; _last_ex_ln=0
for _h in $HD_SHIPPED_HARNESSES; do
  _ln="$(grep -nE "^#[[:space:]]*$_h:[[:space:]]*$" "$EX" | head -n1 | cut -d: -f1)"
  if [ -n "$_ln" ] && [ "$_ln" -gt "$_last_ex_ln" ]; then _last_ex_ln="$_ln"; last_ex_harness="$_h"; fi
done
assert "round-trip: a last shipped harness block was located in the example (got ${last_ex_harness:-none})" \
  '[ -n "$last_ex_harness" ]'
rt_bm="$(hd_field "$HD" "$last_ex_harness" build-max model)"
assert "round-trip: the sidecar supplies a build-max model to anchor the slice on (got ${rt_bm:-none})" \
  '[ -n "$rt_bm" ]'
# Same boundary class as the mirror slice, for the same prefix-weakness reason, and the same
# address-delimiter escaping: opencode's OpenRouter IDs carry `/`, which would close sed's address
# and kill the expression with "invalid command code" — surfacing as an empty slice that reads like
# a missing block rather than a quoting bug.
rt_term="build-max:.*$(ere_escape "$rt_bm")[,[:space:]}]"
agents_block="$(sed -n "/^# agents:\$/,/${rt_term//\//\\/}/p" "$EX")"
```

- [ ] **Step 3: Replace the terminator guard with one that pins the widened slice**

Replace:

```bash
assert "round-trip: the agents slice terminates at its cursor finalize-change anchor (not EOF)" \
  '[ -n "$agents_block" ] && printf "%s\n" "$agents_block" | tail -n1 | grep -q "finalize-change:.*cursor-grok-4\.5-high-fast"'
```

with:

```bash
rt_last="${agents_block##*$'\n'}"
assert "round-trip: the agents slice terminates at the last block's build-max anchor (not EOF)" \
  '[ -n "$agents_block" ] && grep -q "build-max:" <<<"$rt_last" && grep -qF "$rt_bm" <<<"$rt_last"'
# GUARD THE ORDERING ASSUMPTION rather than trusting it. The derivation above assumes the last
# shipped harness header in the example is also the last CONTENT in the agents: block. A re-ordered
# example, or a fifth harness appended after this anchor, would silently shrink coverage back to
# exactly the bug this change fixes — with every assert below still green on the short slice. So
# assert the slice reaches every shipped harness. Derived population, never a literal list.
rt_missing=""
for _h in $HD_SHIPPED_HARNESSES; do
  grep -qE "^#[[:space:]]*$_h:[[:space:]]*$" <<<"$agents_block" || rt_missing="$rt_missing $_h"
done
assert "round-trip: the slice reaches every shipped harness block (${rt_missing:-none missing})" \
  '[ -z "$rt_missing" ]'
```

- [ ] **Step 4: Run to verify the slice widened and the resolver still accepts it**

```bash
/opt/homebrew/bin/bash tests/test_docket_example_yml.sh 2>&1 | grep 'round-trip'
```

Expected: every `round-trip:` line is `ok -`, including the three new ones (`a last shipped harness block was located`, `the sidecar supplies a build-max model`, `the slice reaches every shipped harness block (none missing)`), and `round-trip: sync-agents resolves the uncommented example (exit 0)` still passes on the now-85-line slice.

- [ ] **Step 5: Mutation-prove the ordering guard**

```bash
cp .docket.example.yml /tmp/0246-t5-ex-backup.yml
# Move the opencode header out of the slice's reach by renaming it — the derivation then picks
# cursor, and the slice stops before opencode.
sed -i '' 's/^#   opencode:$/#   opencodeX:/' .docket.example.yml
/opt/homebrew/bin/bash tests/test_docket_example_yml.sh 2>&1 | grep '^NOT OK' | head -4
cp /tmp/0246-t5-ex-backup.yml .docket.example.yml && rm /tmp/0246-t5-ex-backup.yml
```

Expected: `NOT OK - round-trip: the slice reaches every shipped harness block ( opencode)`. Restore, then re-run and confirm green.

- [ ] **Step 6: Cover a cursor build row, previously unexercised**

Immediately after the existing `round-trip: cursor status model came from the example block` assert, add:

```bash
# A cursor BUILD row (change 0246). Before the terminator was re-derived, the slice ended above
# cursor's build rows entirely, so no build profile on this harness was ever resolved. Same
# "both sides move together" caveat as the codex and opencode legs — this catches a VALUE drift
# between example and sidecar, not a missing example row; the sentinel block below owns provenance.
assert "round-trip: a cursor build-max wrapper was generated" '[ -f "$SB/.cursor/agents/docket-build-max.md" ]'
assert "round-trip: cursor build-max model came from the example block" \
  '[ -n "$(hd_field "$HD" cursor build-max model)" ] &&
   [ "$(fm "$SB/.cursor/agents/docket-build-max.md" model)" = "$(hd_field "$HD" cursor build-max model)" ]'
```

- [ ] **Step 7: Fix the stale row-count comment**

Find, in the comment above `stage2=`:

```bash
# Since change 0169 all three harness blocks sit at the SAME single-comment level, so one strip
# uncomments agents:, its three harness blocks, and all thirty-nine rows. (Before 0169 codex and
# cursor sat a level deeper and needed a second, block-scoped strip; that stage is gone with the
# asymmetry it existed for.)
```

Replace with:

```bash
# Since change 0169 every harness block sits at the SAME single-comment level, so one strip
# uncomments agents:, all of its shipped harness blocks, and every row of every one of them.
# (Before 0169 codex and cursor sat a level deeper and needed a second, block-scoped strip; that
# stage is gone with the asymmetry it existed for.) No count is written here on purpose: the
# previous wording said "all three harness blocks" and "all thirty-nine rows", both of which went
# stale the moment 0192 shipped a fourth block — and a restated number is the thing this suite
# exists to catch elsewhere. The population is asserted from HD_SHIPPED_HARNESSES above.
```

- [ ] **Step 8: Run to verify**

```bash
/opt/homebrew/bin/bash tests/test_docket_example_yml.sh 2>&1 | grep -c '^ok'
/opt/homebrew/bin/bash tests/test_docket_example_yml.sh 2>&1 | grep '^NOT OK'
grep -n 'thirty-nine\|all three harness blocks' tests/test_docket_example_yml.sh
```

Expected: `406` (401 + 5 new asserts), no `NOT OK`, and the last grep finds nothing.

- [ ] **Step 9: Commit**

```bash
git add tests/test_docket_example_yml.sh
git commit -m "test(0246): round-trip the whole agents block, not the first half

The slice's end anchor was a hand-written cursor finalize-change
literal that sits above cursor's own build rows and above the entire
opencode block, so those rows never reached the real resolver — the
staleness 0192 introduced and nothing caught. Terminator now derived
from the last-in-file shipped harness's sidecar build-max model, with a
boundary class and an assert that the slice reaches every shipped
harness. Added a cursor build row leg; de-numbered the stale comment."
```

---

### Task 6: Shape-tighten the `elsewhere:` consumer grep

**Why:** The `elsewhere:` arm asserts that a key's **named consumer actually mentions it** — that is what keeps the entry anchored on consuming code rather than decaying into a bare allowlist. But a bare word-boundary grep is satisfied by the key appearing in a comment, or in a sentence of English prose. The historical false positive is change 0102's: a key appearing only in prose inside an embedded heredoc prompt. A guard that a sentence can satisfy is not anchoring anything.

**The shape set is DERIVED from the six entries' actual mentions, not guessed.** Evidence gathered from the real tree while planning, non-comment lines only:

| Entry (leaf) | Consumer | The mention that carries it | Shape |
|---|---|---|---|
| `agents` | `sync-agents.sh:118` | `grep -qE '^agents[[:space:]]*:' "$GLOBAL_CFG"` | key followed by `:`, optionally through a literal `[[:space:]]*` |
| `agent_harnesses` | `sync-agents.sh:146` | `grep -qE '^agent_harnesses[[:space:]]*:' "$f"` | same |
| `network` | `scripts/runners/codex.sh:74` | `die "runners.codex.network must be 'true' or 'false' …"` | dot-qualified (`codex.network`) |
| `permissions` | `scripts/runners/opencode.sh:46,50` | `die "runners.opencode.permissions is 'ask' …"` | dot-qualified |
| `sandbox` | `scripts/runners/codex.sh:83` | `cmd=( "$CODEX_BIN" exec -C "$DOCKET_REPO_ROOT" --sandbox "$SANDBOX" … )` | flag argument (`--sandbox`) |
| `github_project` | `scripts/docket-config.sh:400` | `for _fkey in … results_dir github_project terminal_publish …` | **none** — a bare space-delimited token in a fence list |

Three shapes, each traced to a named line: **`:`-adjacency**, **dot-qualification**, **flag argument**. No unevidenced shape is added — widening the set until prose passes is exactly the failure mode this task exists to prevent.

`github_project` cannot meet any shape, and that is correct rather than a gap: `.docket.example.yml` itself says the key is NOT WIRED, `docket-config.sh`'s only match is its coordination-key **fence** list (which warns-and-ignores the key, it does not read it), and the classifier's own comment already calls that anchor "documentation-only, unlike every other elsewhere: entry". It is routed through an explicit one-key exemption mirroring the resolved arm's existing `correspondence_exempt` — **never** by widening the shape set.

**Verified against the real tree while planning:** with the three shapes above, the five non-exempt entries pass, `github_project` fails (→ exemption), and a fixture with the key appearing only in English prose inside a heredoc correctly does not match.

**Files:**
- Modify: `tests/test_docket_example_yml.sh` — the `elsewhere:*` case arm (the grep Task 2 converted), plus a new exemption constant and three new asserts.

**Interfaces:**
- Consumes: `$leaf_k`, `$k`, `$consumer`, `$REPO`, `$allowlisted` — all already in scope in that arm.
- Produces: `elsewhere_shape_exempt` (a space-delimited key list) and `code_shaped_mention(key, file)` → exit 0 iff a non-comment line carries a code-shaped mention.

- [ ] **Step 1: Write the failing fixture asserts first**

Add immediately after the `manifest: every elsewhere:HEADER entry is a real bare block opener` assert:

```bash
# --- elsewhere: shape guard, its exemption, and its false-positive fixture (change 0246) ---------
# The exemption list is asserted to hold EXACTLY the one key it is allowed to hold. An exemption
# list that can grow silently is a bare allowlist wearing a different name — the drift this whole
# manifest exists to prevent. Widening this list is a deliberate act that must redden here first.
assert "elsewhere: the shape exemption holds exactly github_project (got '$elsewhere_shape_exempt')" \
  '[ "$elsewhere_shape_exempt" = "github_project" ]'
# Positive control: the shape predicate must FIRE on a real consumer mention, or every green
# above it is consistent with a predicate that can never match anything.
assert "elsewhere: shape control — a real flag-argument mention is code-shaped" \
  'code_shaped_mention sandbox "$REPO/scripts/runners/codex.sh"'
# NEGATIVE control reproducing the historical false positive (change 0102): a key appearing ONLY as
# an English word in prose, inside an embedded heredoc prompt, on non-comment lines. A bare
# word-boundary grep passes this; the shape predicate must not. Comment-region exclusion alone
# would also fail here, which is why the shapes exist.
_shape_fx="$tmp/shape-prose.sh"
{
  printf 'run_it(){\n'
  printf '  cat <<PROMPT\n'
  printf 'Please choose an appropriate timeout before you continue running the job.\n'
  printf 'PROMPT\n'
  printf '}\n'
} > "$_shape_fx"
assert "elsewhere: shape control — a key appearing only in heredoc prose is NOT code-shaped" \
  '! code_shaped_mention timeout "$_shape_fx"'
```

- [ ] **Step 2: Run to verify it fails**

Run: `/opt/homebrew/bin/bash tests/test_docket_example_yml.sh 2>&1 | grep -E 'shape|unbound'`
Expected: FAIL — `elsewhere_shape_exempt: unbound variable` under `set -u`, and/or `code_shaped_mention: command not found`.

- [ ] **Step 3: Add the exemption constant and the predicate**

Add next to the existing `correspondence_exempt="…"` declaration (around line 172), so the two exemption mechanisms sit together:

```bash
# The one elsewhere: key whose consumer mention cannot be code-shaped, and why (change 0246).
# .docket.example.yml says github_project is NOT WIRED TODAY — no script reads it. Its only match in
# docket-config.sh is the coordination-key FENCE list (`for _fkey in … github_project …`), which
# warns-and-ignores the key in machine-scoped layers rather than reading it, so the mention is a
# bare space-delimited token with no code shape at all. The classifier's own comment on that arm
# already calls the anchor "documentation-only, unlike every other elsewhere: entry". Exempting the
# one honest outlier is right; widening the shape set until a fence-list token counts as code would
# re-admit the English-prose match the shapes exist to reject. Asserted to hold exactly this key.
elsewhere_shape_exempt="github_project"
```

Add the predicate next to `is_header_key` (it belongs with the other classifier helpers):

```bash
# code_shaped_mention <leaf-key> <file> -> exit 0 iff a NON-COMMENT line of the file mentions the
# key in a code-shaped context. Backs the elsewhere: arm, replacing a bare word-boundary grep that
# a sentence of English prose satisfied (change 0102's `timeout`-in-a-heredoc false positive).
#
# Two conditions: the line is not a comment (first non-space character is not `#`), and the key
# occurs in one of three shapes, each DERIVED from a real mention in the six entries' named
# consumers rather than guessed:
#
#   1. `:`-adjacency   — `agents[[:space:]]*:` (sync-agents.sh, a quoted YAML-key regex) and
#                        `runners.opencode.permissions: auto-approve` (opencode.sh). The optional
#                        literal `[[:space:]]*` is matched because the real mentions are grep
#                        patterns that spell the gap that way.
#   2. dot-qualified   — `codex.network`, `opencode.permissions`: the key as the leaf of a config
#                        path, which is how the runner adapters name it in their die messages.
#   3. flag argument   — `--sandbox` (codex.sh:83). runners.codex.sandbox's QUALIFIED form appears
#                        nowhere in its consumer; the flag is the only real mention, which is why
#                        this shape is required and not decorative.
#
# Deliberately NOT added: assignment/`$var` shapes, or anything else no current entry needs. Every
# shape widens what counts as an anchor, and the failure mode being closed is over-permissiveness.
# All six current targets are shell scripts, so shell shapes suffice; if a PROSE consumer (a
# SKILL.md) is ever reclassified to elsewhere:, this shape set must be revisited rather than
# stretched.
code_shaped_mention(){ # $1=leaf key  $2=file
  local k="$1" f="$2"
  [ -f "$f" ] || return 1
  grep -vE '^[[:space:]]*#' "$f" \
    | grep -qE "($k(\[\[:space:\]\]\*)?:)|([A-Za-z0-9_]\.$k)|(--?$k)"
}
```

- [ ] **Step 4: Run to verify the three fixture asserts pass**

Run: `/opt/homebrew/bin/bash tests/test_docket_example_yml.sh 2>&1 | grep 'elsewhere: '`
Expected: all three new lines are `ok -`.

- [ ] **Step 5: Swap the arm's grep for the predicate**

In the `elsewhere:*` case arm, replace:

```bash
      if [ "$allowlisted" -eq 1 ]; then
        grep -qE "(^|[^[:alnum:]_])$leaf_k([^[:alnum:]_]|$)" "$REPO/$consumer" \
          || manifest_bad_consumer="$manifest_bad_consumer $k(not in $consumer)"
      fi
```

with:

```bash
      # SHAPE-TIGHTENED (change 0246). A bare word-boundary grep here was satisfied by the key
      # appearing in a comment or in a sentence of English prose — 0102's heredoc-prompt false
      # positive — which anchors nothing. See code_shaped_mention above for the three derived
      # shapes, and elsewhere_shape_exempt for the single documented outlier.
      if [ "$allowlisted" -eq 1 ]; then
        case " $elsewhere_shape_exempt " in
          *" $leaf_k "*)
            # Exempt from the SHAPE requirement, never from the mention requirement: the key must
            # still actually appear in its named consumer, or the entry has no anchor at all.
            grep -qE "(^|[^[:alnum:]_])$leaf_k([^[:alnum:]_]|$)" "$REPO/$consumer" \
              || manifest_bad_consumer="$manifest_bad_consumer $k(not in $consumer)"
            ;;
          *)
            code_shaped_mention "$leaf_k" "$REPO/$consumer" \
              || manifest_bad_consumer="$manifest_bad_consumer $k(no code-shaped mention in $consumer)"
            ;;
        esac
      fi
```

- [ ] **Step 6: Run to verify the five non-exempt entries stay green**

```bash
/opt/homebrew/bin/bash tests/test_docket_example_yml.sh 2>&1 | grep "elsewhere: entry's named consumer"
```

Expected: `ok - manifest: every elsewhere: entry's named consumer mentions the key (none bad)`.

```bash
/opt/homebrew/bin/bash tests/test_docket_example_yml.sh 2>&1 | grep -c '^ok'
/opt/homebrew/bin/bash tests/test_docket_example_yml.sh 2>&1 | grep '^NOT OK'
```

Expected: `409` (406 + 3 new asserts), no `NOT OK`.

- [ ] **Step 7: Mutation-prove the arm — both that it fires and that the exemption is not a hole**

```bash
cp tests/test_docket_example_yml.sh /tmp/0246-t6-backup.sh
# (a) Repoint a real entry at a consumer that mentions the key only in prose/comments.
sed -i '' "s|    runners.codex.sandbox) echo 'elsewhere:scripts/runners/codex.sh' ;;|    runners.codex.sandbox) echo 'elsewhere:scripts/runners/opencode.sh' ;;|" tests/test_docket_example_yml.sh
/opt/homebrew/bin/bash tests/test_docket_example_yml.sh 2>&1 | grep '^NOT OK'
cp /tmp/0246-t6-backup.sh tests/test_docket_example_yml.sh
```

Expected: `NOT OK - manifest: every elsewhere: entry's named consumer mentions the key ( runners.codex.sandbox(no code-shaped mention in scripts/runners/opencode.sh))`. (`sandbox` appears in `opencode.sh` only at line 40, a comment.)

```bash
# (b) The exemption must not be silently extendable.
sed -i '' 's/^elsewhere_shape_exempt="github_project"$/elsewhere_shape_exempt="github_project agents"/' tests/test_docket_example_yml.sh
/opt/homebrew/bin/bash tests/test_docket_example_yml.sh 2>&1 | grep '^NOT OK'
cp /tmp/0246-t6-backup.sh tests/test_docket_example_yml.sh && rm /tmp/0246-t6-backup.sh
```

Expected: `NOT OK - elsewhere: the shape exemption holds exactly github_project (got 'github_project agents')`.

```bash
/opt/homebrew/bin/bash tests/test_docket_example_yml.sh 2>&1 | grep -c '^ok'
```

Expected: `409` restored.

- [ ] **Step 8: Confirm no banned form was reintroduced**

```bash
/opt/homebrew/bin/bash tests/test_grep_portability.sh; echo "rc=$?"
```

Expected: `rc=0` — Task 3's guard is clean, i.e. Task 6's new EREs use no `\\b` form.

- [ ] **Step 9: Commit**

```bash
git add tests/test_docket_example_yml.sh
git commit -m "test(0246): make the elsewhere: anchor prove a code-shaped read

A bare word-boundary grep was satisfied by the key appearing in a
comment or in English prose — 0102's heredoc-prompt false positive —
so the entry anchored nothing. Replaced with three shapes derived from
the six entries' real mentions (:-adjacency, dot-qualified, flag
argument), on non-comment lines only. github_project's only mention is
a bare token in docket-config.sh's coordination-key fence list, so it
takes a one-key exemption asserted to hold exactly that key, rather
than widening the shapes until prose passes."
```

---

### Task 7: Full-suite gate

**Files:** none modified.

**Interfaces:**
- Consumes: every preceding task's commits.
- Produces: the build-evidence record.

- [ ] **Step 1: Run the whole suite**

Run: `/opt/homebrew/bin/bash scripts/run-tests.sh`
Expected: green, exit 0.

- [ ] **Step 2: Re-confirm the two headline numbers**

```bash
/opt/homebrew/bin/bash tests/test_docket_example_yml.sh 2>&1 | grep -c '^ok'
PATH=/usr/bin:/bin bash tests/test_docket_example_yml.sh; echo "rc=$?"
```

Expected: `409` on the first; on the second, exactly one line naming `requires bash >= 4` and `rc=2` — never a partial run.

- [ ] **Step 3: Confirm the tree carries no banned pattern form**

```bash
/opt/homebrew/bin/bash tests/test_grep_portability.sh; echo "rc=$?"
```

Expected: `rc=0`.

---

## Notes for the reviewer

- **Assert-count arithmetic.** 393 baseline → +8 (T4 reverse: two per harness × four) → +5 (T5: two derivation asserts, one ordering assert, two cursor build asserts) → +3 (T6: exemption arity, positive control, prose negative control) = **409**. A different final number means a task added or dropped an assert; reconcile before merging.
- **Task 2 and Task 6 both touch the same grep**, by design: Task 2 converts it so Task 3's guard can land on a clean tree, and Task 6 replaces it. Task 6's replacement must not reintroduce a `\\b` form; Task 6 Step 8 is the check.
- **Line numbers in this plan are from `origin/main` @ 483c5dad** and shift as tasks land (Task 1 alone adds ~20 lines). Match on quoted content, never on a line number.
- **`sed -i ''`** in the mutation steps is BSD/macOS syntax. On GNU sed, use `sed -i` with no argument.
