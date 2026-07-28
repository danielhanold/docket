<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0130 — Make the finalize marker reachability guard portable to BSD grep](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0130-make-the-finalize-marker-reachability-guard-portable-to-bsd.md)**
<!-- docket:backlink:end -->

# BSD grep interval portability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the whole test suite runnable under BSD grep by removing the one ERE repetition bound above 255, and add a repo-wide static guard so the class cannot come back.

**Architecture:** Two independent halves. (1) A one-line repair to `tests/test_finalize_disposition.sh:186`, replacing `.{0,600}` with an unbounded within-line `.*` — `grep` is line-based, so the same-line scope and the anchor ordering (the properties the assertion actually rests on) survive; the numeric bound never was the constraint. (2) A new `tests/test_grep_portability.sh`, a static guard that walks every tracked path except `docs/` and fails on any brace-interval literal carrying a bound above 255. The guard extracts candidate intervals with a portable ERE and does the numeric comparison in shell, because "greater than 255" is not expressible as a readable regex.

**Tech Stack:** bash 4+ (`set -uo pipefail`), `git ls-files`, POSIX `grep -E`. No new dependencies. The suite has no runner — each `tests/*.sh` file is invoked directly.

## Global Constraints

Copied verbatim from the spec and from `AGENTS.md`. Every task's requirements implicitly include this section.

- **Verification PATH.** Every verification of this change runs with `PATH=/usr/bin:$PATH` — a **prepend, never a replace**. This machine's PATH `grep` is `ugrep 7.5.0`, which accepts the bound the fix exists to eliminate, so a green ambient-PATH run proves nothing. The prepend (not a replace) is required because roughly a dozen test files call `jq`/`gh`, which resolve from Homebrew paths.
- **Both PATHs must be green.** `tests/test_finalize_disposition.sh` and `tests/test_grep_portability.sh` must pass under the prepended PATH *and* under the ambient one, and the whole suite must be green.
- **The threshold is 255.** A bound of exactly 255 is legal; 256 and above is a violation. BSD grep's message is `grep: maximum repetition exceeds 255`.
- **The guard carries no >255 literal of its own.** Every such literal it needs is assembled at runtime (arithmetic or `printf`), never written literally. The guard asserts explicitly that its own file is in the scanned population and is clean — it does **not** self-exclude.
- **`docs/` is the one exclusion**, and it is a decision, not an accident: archived change files, historical plans, published terminal records and this change's own spec legitimately quote `{0,600}` verbatim and are immutable point-in-time records the convention forbids rewriting. It must be documented in the guard's header comment.
- **No allowlist.** Exclusions are by walk scope, never by exception entry (ADR-0050, `enumerated-floor`). The walk is computed from `git ls-files`, with **no extension filter** — an extension list re-introduces the same blind spot on a different axis.
- **Repo root from `BASH_SOURCE`**, never cwd-relative: `ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"`. This also excludes `.worktrees/` and untracked scratch for free.
- **Shell rules (`AGENTS.md`).** Never `producer | early-exiting-consumer` (`grep -q`, `head`, `head -n1`) under `set -o pipefail` — capture into a variable first, then use a here-string. No GNU-only constructs: no `grep -P`, no `\d`, no `-z`, no `\b`/`\<`.
- **A guard is code** — mutation-test it, or it is decoration. Mutation-test its **population**, not only its suppression (ADR-0050).
- **Out of scope:** any change to finalize behavior or the `## Finalize blocked` contract; rewriting other disposition assertions; rewriting the four existing `docs/` occurrences; pinning or reporting the resolved toolchain across all 63 test files (captured separately as change 0150).

---

## File Structure

| File | Disposition | Responsibility |
|---|---|---|
| `tests/test_finalize_disposition.sh` | Modify (line 186 only) | Change-0087-scoped assertions about the finalize disposition contract. One assertion is repaired; nothing else in the file is touched. |
| `tests/test_grep_portability.sh` | Create | One repo-wide invariant: no maintained source file carries a brace-interval bound above 255. Self-contained — walk, floor, check, self-membership, positive/negative controls, informational toolchain line. |
| `skills/docket-finalize-change/SKILL.md` | **Not modified** — mutated and restored during Task 1 verification only | The artifact the repaired assertion reads. Its line 168 carries both anchors. |
| `docs/superpowers/plans/2026-07-28-bsd-grep-interval-portability.md` | Create (this file) | The plan, committed with the code on the feature branch. |

Two files, two tasks, plus a cross-cutting verification task the spec makes mandatory (§Verification) and that neither authoring task can perform on its own.

---

### Task 1: Repair the finalize marker reachability assertion

**Files:**
- Modify: `tests/test_finalize_disposition.sh` (the single assertion at line 186)
- Mutate-and-restore during verification only: `skills/docket-finalize-change/SKILL.md`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: nothing later tasks rely on. This task is independent of Task 2 — Task 2's guard will *also* cover this file, but neither task's code calls the other's.

**Background you need.** The assertion proves that the `## Finalize blocked` marker write is *reachable* from the abort-and-report procedure, not merely *defined* somewhere in the file. Both anchors live on one line — `skills/docket-finalize-change/SKILL.md` line 168, 481 characters, beginning `**Where the reason surfaces.**` and containing `**and appends the \`## Finalize blocked\` marker to the change file**`. The gap between the two anchors is 301 characters, genuinely above 255, so shrinking the bound is not available. `grep` is line-based: `.*` cannot cross a newline, so it still confines the match to a single line and still enforces the ordering of the two anchors. The trailing `.{0,4}` (backtick/emphasis slack) is far below 255 and stays.

- [ ] **Step 1: Confirm the bug reproduces under BSD grep**

Run:

```bash
cd /Users/homer/dev/docket/.worktrees/make-the-finalize-marker-reachability-guard-portable-to-bsd
PATH=/usr/bin:$PATH bash tests/test_finalize_disposition.sh 2>&1 | grep -i 'repetition\|NOT OK' | head -5
```

Expected: a line containing `maximum repetition exceeds 255`. This is the RED state — the assertion errors out before it ever inspects the skill.

Also confirm the ambient PATH hides it (this is *why* the bug survived):

```bash
bash tests/test_finalize_disposition.sh >/dev/null 2>&1; echo "ambient exit=$?"
```

Expected: `ambient exit=0`.

- [ ] **Step 2: Apply the one-line repair**

In `tests/test_finalize_disposition.sh`, find this exact line:

```sh
  'grep -Eqi "where the reason surfaces.{0,600}appends the .{0,4}## Finalize blocked" "$FIN"'
```

Replace it with:

```sh
  'grep -Eqi "where the reason surfaces.*appends the .{0,4}## Finalize blocked" "$FIN"'
```

Change nothing else in the file. Do not touch the neighbouring assertions (`re-mark.{0,60}replaces.{0,120}`, `prerequisite.{0,200}` and the rest) — every one of those bounds is already below 255.

- [ ] **Step 3: Run the repaired file under BSD grep**

Run:

```bash
PATH=/usr/bin:$PATH bash tests/test_finalize_disposition.sh; echo "exit=$?"
```

Expected: `exit=0`, no `maximum repetition` line, and a line reading
`ok   - SKILL wires the marker write into the abort-and-report surfacing step`.

- [ ] **Step 4: Run it under the ambient PATH too**

Run:

```bash
bash tests/test_finalize_disposition.sh; echo "exit=$?"
```

Expected: `exit=0`. Both toolchains must agree; a fix that only works under one is not a portability fix.

- [ ] **Step 5: Mutation-proof the repaired assertion**

A repaired assert that no longer bites is worse than the broken one, because it is green. Delete the clause the assertion exists to find, and confirm **only** that assertion reddens.

```bash
cd /Users/homer/dev/docket/.worktrees/make-the-finalize-marker-reachability-guard-portable-to-bsd
cp skills/docket-finalize-change/SKILL.md /tmp/fin-skill.bak
# Remove the marker-write clause from line 168, leaving the rest of the sentence intact.
perl -0pi -e 's/ \*\*and appends the `## Finalize blocked` marker to the change file\*\*//' \
  skills/docket-finalize-change/SKILL.md
# Confirm the mutation actually landed:
grep -c 'and appends the `## Finalize blocked` marker to the change file' \
  skills/docket-finalize-change/SKILL.md
```

Expected from that last command: `0`. **If it prints anything else, the mutation did not land and the next step's result is meaningless** — stop and fix the mutation before continuing.

Then:

```bash
PATH=/usr/bin:$PATH bash tests/test_finalize_disposition.sh 2>&1 | grep 'NOT OK'
```

Expected: exactly one `NOT OK` line, and it is
`NOT OK - SKILL wires the marker write into the abort-and-report surfacing step`.

Record the count. If zero lines appear the assertion is decoration; if more than one appears, note which others and why (an unrelated assertion keying on the same clause is a finding for the review step, not necessarily a defect).

- [ ] **Step 6: Restore the mutated skill and prove the tree is clean**

```bash
cp /tmp/fin-skill.bak skills/docket-finalize-change/SKILL.md && rm /tmp/fin-skill.bak
git diff --stat -- skills/docket-finalize-change/SKILL.md
```

Expected: **empty output** from `git diff --stat` — the skill file is byte-identical to `origin/main`. Do not proceed to the commit until this is empty; a mutation left in the tree would ship a contract change this task has no business making.

- [ ] **Step 7: Commit**

```bash
git add tests/test_finalize_disposition.sh
git commit -m "fix(0130): unbounded within-line .* in the finalize marker reachability assert

BSD grep rejects an ERE repetition bound above 255, so .{0,600} errored out
before the assertion ever inspected the skill. grep is line-based, so .* keeps
both the single-line scope and the anchor ordering the assert actually rests
on; the numeric bound was never the load-bearing constraint. Mutation-proofed
by deleting the marker-write clause from the finalize skill and confirming
only this assertion reddens."
```

---

### Task 2: Add the repo-wide interval portability guard

**Files:**
- Create: `tests/test_grep_portability.sh`
- Reference precedent (read, do not modify): `tests/test_comment_anchor_style.sh`

**Interfaces:**
- Consumes: nothing from Task 1. (Task 1's repair is what makes the guard green on the current tree, but the guard is authored against the invariant, not against that repair.)
- Produces: `tests/test_grep_portability.sh`, exit 0 on a clean tree, exit 1 with per-violation output otherwise. Task 3 runs it.

**Design decisions already settled — do not re-litigate:**

- **The scan pattern is `\{[0-9]+(,[0-9]*)?\}`.** It matches `{600}`, `{0,600}` and `{0,}`. In ERE, `\{` is a literal brace on both GNU and BSD. It also matches the BRE form `\{0,600\}` — the backslash simply precedes the matched text and does not interfere. The pattern deliberately contains no bound of its own above 255, so the guard stays clean under its own scan.
- **The numeric comparison happens in shell, not in the regex.** "Any number above 255" is not readably expressible as an ERE, and an attempt to write it (`\{([3-9][0-9]{2}|...)`) would itself embed brace intervals into the guard. Extract candidates, then compare.
- **False positives are impossible at this threshold** (spec A3): the pattern also matches BRE `\{m,n\}` and non-regex braces, which is harmless because the repo's entire interval inventory tops out at 600 and no non-regex brace construct carries a number that large.
- **Binary safety comes from `grep -I`**, never from a filename pattern.
- **Tracked-only is a known, accepted edge:** a source file added in the same in-progress commit is not yet in the population until `git add`. The self-membership assert makes the gap visible rather than silent — and it stays red until the new file is staged, which is a red for the *right* reason at authoring time and something you must not "fix" by weakening the assert.

- [ ] **Step 1: Write the guard, failing-first, against a fixture-only skeleton**

Create `tests/test_grep_portability.sh` with exactly this content:

```bash
#!/usr/bin/env bash
# tests/test_grep_portability.sh — no maintained source file may carry an ERE repetition bound
# above 255 (change 0130).
#
# WHY: BSD grep rejects a repetition bound above 255 with "maximum repetition exceeds 255". A test
# written with {0,600} therefore errors out before it examines anything, and on a machine whose
# PATH grep is GNU grep or ugrep it passes anyway — the suite runs green while the bug is real.
# This guard is a STATIC SOURCE scan, deliberately not a runtime probe of the local grep's
# behavior: on Linux /usr/bin/grep is GNU grep and accepts the bound, so a behavioral assertion
# would be a platform-dependent false failure. The property wanted is source portability, which is
# true or false independent of the machine running the suite.
#
# SCOPE: every tracked path (git ls-files, anchored on the repo root resolved from BASH_SOURCE),
# minus the docs/ prefix. NO extension filter — an extension list is the same re-enumeration on a
# different axis (.mdc, .py, an extensionless hook) and buys nothing, because no false positive is
# possible at a >255 threshold. Binary safety comes from grep -I.
#
# docs/ IS THE ONE EXCLUSION, AND IT IS A DECISION: archived change files, historical plans,
# published terminal records and design specs legitimately quote defective patterns verbatim, and
# they are immutable point-in-time records the convention forbids rewriting (AGENTS.md, "Comments
# and cross-references"). Four such occurrences exist today, and terminal_publish: true will add
# this change's own file and spec — which quote {0,600} verbatim — at close-out. The guard must not
# demand a repair it cannot legally have. Every OTHER tracked surface is in scope automatically,
# including any new top-level directory added later.
# NO ALLOWLIST: exclusions are by walk scope, never by exception entry (ADR-0050).
#
# SELF-MEMBERSHIP: this file is NOT self-excluded. It is asserted to be in the scanned population
# and clean, which is why every >255 literal it needs is assembled at runtime rather than written.
#
# TRACKED-FILES-ONLY: a brand-new file is invisible here until it is staged. Accepted — the guard
# runs at the build gate over committed work — and the self-membership assert makes the gap loud.
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SELF_REL="tests/$(basename "${BASH_SOURCE[0]}")"
fail=0
ok(){   printf 'ok   - %s\n' "$1"; }
nok(){  printf 'NOT OK - %s\n' "$1"; fail=1; }

# The maximum repetition bound BSD grep accepts. A bound EQUAL to this is legal; above it is not.
MAX_BOUND=255

# A brace interval literal: {m}, {m,} or {m,n}. \{ is a literal brace in ERE on GNU and BSD alike.
# This also matches the BRE form \{m,n\} — the backslash merely precedes the matched text.
# NOTE: no \b / \< anywhere — git grep's and BSD grep's ERE do not support them and return zero
# silently. This pattern carries no bound of its own above MAX_BOUND, so the guard stays clean
# under its own scan.
INTERVAL='\{[0-9]+(,[0-9]*)?\}'

# ONE scan implementation, used by the main loop AND both controls below — never a second,
# independently-written grep call. Routing everything through this function means neutering the
# scan path anywhere neuters it everywhere, so a control cannot stay green while the loop goes
# blind. -I skips binaries; -o emits one interval per line; -n prefixes the source line number.
scan_file(){ grep -InoE "$INTERVAL" "$1" 2>/dev/null; }

# Report every bound above MAX_BOUND in "lineno:interval" input. Reads scan_file output on stdin.
# Pure text + arithmetic; no regex expresses "greater than 255" readably, and attempting one would
# embed brace intervals into this guard.
offenders(){
  local line lineno interval nums n
  while IFS= read -r line; do
    [ -n "$line" ] || continue
    lineno="${line%%:*}"
    interval="${line#*:}"
    nums="${interval#\{}"; nums="${nums%\}}"     # {0,600} -> 0,600
    while [ -n "$nums" ]; do
      n="${nums%%,*}"
      if [ -n "$n" ] && [ "$n" -gt "$MAX_BOUND" ] 2>/dev/null; then
        printf '%s:%s\n' "$lineno" "$interval"
        break
      fi
      [ "$nums" = "${nums#*,}" ] && break
      nums="${nums#*,}"
    done
  done
}

# --- informational, NON-GATING: which grep did this run actually exercise? -----------------------
# A portability guard that silently tests a different tool than the one it targets is the trap this
# change exists to close. This line does not gate anything — the scan is static — but it lets a
# reader see the toolchain. Captured into a variable first, then read with a here-string: a
# producer feeding an early-exiting consumer under pipefail can take SIGPIPE and become an
# intermittent 141 (AGENTS.md, Shell).
grep_path="$(command -v grep 2>/dev/null || true)"
grep_ver_all="$(grep --version 2>/dev/null || true)"
grep_ver="$(sed -n '1p' <<<"$grep_ver_all")"
printf '#    - resolved grep: %s (%s)\n' "${grep_path:-unknown}" "${grep_ver:-version unavailable}"

# --- collect the in-scope population -------------------------------------------------------------
# Computed from tracked files, never a hand-enumerated directory list: a hand list leaves any new
# top-level source directory silently unguarded, which is precisely what ADR-0050 rules out.
mapfile -t FILES < <(
  cd "$ROOT" || exit 1
  git ls-files | grep -v '^docs/'
)

# --- population floor: the walk must actually reach files ----------------------------------------
# A guard iterating an empty list is green and proves nothing.
n_files=${#FILES[@]}
[ "$n_files" -ge 100 ] \
  && ok "walk population is non-trivial ($n_files files)" \
  || nok "walk population collapsed to $n_files files (expected >= 100) — ls-files or the filter broke"

files_joined=""
[ "$n_files" -gt 0 ] && files_joined="$(printf '%s\n' "${FILES[@]}")"

# Named probes across distinct surfaces, including two NON-.sh files: the walk has no extension
# filter, and these are what prove it.
for probe in tests/test_finalize_disposition.sh scripts/board-checks.sh AGENTS.md \
             .docket.example.yml skills/docket-adr/SKILL.md agents/docket-adr.md \
             cursor-rules/dispatch/docket-adr.md migrate-to-docket.sh; do
  grep -qxF "$probe" <<<"$files_joined" \
    && ok "walk includes $probe" \
    || nok "walk MISSES $probe — the in-scope surface is not fully covered"
done

# The docs/ exclusion must actually exclude.
grep -qE '^docs/' <<<"$files_joined" \
  && nok "walk leaked a docs/ path — the exclusion is not applied" \
  || ok "walk excludes docs/"

# --- self-membership: this guard is scanned like everything else ---------------------------------
# Stays RED until this file is git-added. That is the tracked-only edge made loud, not a bug.
grep -qxF "$SELF_REL" <<<"$files_joined" \
  && ok "guard is itself in the scanned population ($SELF_REL)" \
  || nok "guard is NOT in the scanned population — git add $SELF_REL (tracked-files-only walk)"

# --- the check -----------------------------------------------------------------------------------
violations=""
scanned=0
if [ "$n_files" -gt 0 ]; then
  for f in "${FILES[@]}"; do
    [ -f "$ROOT/$f" ] || continue
    scanned=$(( scanned + 1 ))
    hits="$(scan_file "$ROOT/$f")"
    [ -n "$hits" ] || continue
    bad="$(offenders <<<"$hits")"
    [ -n "$bad" ] && violations+="$(sed "s|^|$f:|" <<<"$bad")"$'\n'
  done
fi

[ "$scanned" -ge 100 ] \
  && ok "scanned $scanned files" \
  || nok "scanned only $scanned files — the scan loop is not reaching the population"

if [ -z "$violations" ]; then
  ok "no ERE repetition bound above $MAX_BOUND in maintained source"
else
  nok "ERE repetition bound above $MAX_BOUND found — BSD grep rejects these; rewrite the pattern:"
  printf '%s' "$violations" | sed 's/^/       /'
fi

# --- controls: prove the predicate FIRES and where its boundary sits ------------------------------
# Without these, every assert above is consistent with a pattern that can never match anything.
# Routed through the SAME scan_file + offenders the loop uses, so a control can only stay green if
# the exact path the loop runs is still capable of firing.
#
# The over-threshold fixtures are ASSEMBLED AT RUNTIME. Writing {0,600} literally here would make
# this guard fail its own scan — the self-membership assert above is what makes that a real
# constraint rather than a stylistic one.
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

over="$(( MAX_BOUND + 345 ))"       # 600 — the real-world bound, never written literally
edge="$(( MAX_BOUND + 1 ))"         # 256 — one past the boundary
printf 'x.%s%s,%s%sy\n' '{' 0 "$over" '}' > "$tmp/over.txt"
printf 'x.%s%s,%s%sy\n' '{' 0 "$edge" '}' > "$tmp/edge.txt"

pos="$(offenders <<<"$(scan_file "$tmp/over.txt")")"
[ -n "$pos" ] \
  && ok "positive control: a bound of $over is reported" \
  || nok "positive control FAILED: a bound of $over is not reported — the guard is vacuous"

edge_hit="$(offenders <<<"$(scan_file "$tmp/edge.txt")")"
[ -n "$edge_hit" ] \
  && ok "boundary control: a bound of $edge (one past $MAX_BOUND) is reported" \
  || nok "boundary control FAILED: $edge slipped through — the threshold is off by at least one"

# Negative control. This 255 bound is written LITERALLY on purpose: it is legal under this guard's
# own rule, so it doubles as a demonstration that the boundary is inclusive.
printf 'x.{0,255}y\n' > "$tmp/clean.txt"
printf 'a{b}c and ${VAR} and awk "{print}"\n' >> "$tmp/clean.txt"
neg="$(offenders <<<"$(scan_file "$tmp/clean.txt")")"
[ -n "$neg" ] \
  && nok "negative control FAILED: a legal bound of $MAX_BOUND or a non-regex brace was flagged" \
  || ok "negative control: a bound of exactly $MAX_BOUND and non-regex braces are not flagged"

exit "$fail"
```

- [ ] **Step 2: Run it before staging — the self-membership assert must be RED**

```bash
cd /Users/homer/dev/docket/.worktrees/make-the-finalize-marker-reachability-guard-portable-to-bsd
chmod +x tests/test_grep_portability.sh
PATH=/usr/bin:$PATH bash tests/test_grep_portability.sh; echo "exit=$?"
```

Expected: `exit=1`, with exactly one `NOT OK` line:
`NOT OK - guard is NOT in the scanned population — git add tests/test_grep_portability.sh (tracked-files-only walk)`.

This is the designed red — the tracked-only edge made loud. **Do not weaken the assert to make it green.** Every other line must already be `ok`, including both controls and the main check (Task 1 removed the only real violation; if the main check is red here, Task 1 was not applied or a new violation exists — investigate before continuing).

- [ ] **Step 3: Stage the file and re-run — now fully green**

```bash
git add tests/test_grep_portability.sh
PATH=/usr/bin:$PATH bash tests/test_grep_portability.sh; echo "exit=$?"
```

Expected: `exit=0`, zero `NOT OK` lines, and an informational line beginning `#    - resolved grep: /usr/bin/grep`.

- [ ] **Step 4: Run under the ambient PATH too**

```bash
bash tests/test_grep_portability.sh; echo "exit=$?"
```

Expected: `exit=0`, and the informational line now names the ambient `grep` (ugrep on this machine). The verdict must be identical under both toolchains — the scan is static, so a divergence here means a GNU-only construct leaked into the guard.

- [ ] **Step 5: Prove the guard would have caught the original bug**

Re-introduce the exact defect Task 1 removed, confirm the guard reports it, then restore.

```bash
cp tests/test_finalize_disposition.sh /tmp/fin-disp.bak
big="$(( 255 + 345 ))"
perl -pi -e "s/where the reason surfaces\.\*/where the reason surfaces.\{0,${big}\}/" \
  tests/test_finalize_disposition.sh
grep -c 'where the reason surfaces\.{0,600}' tests/test_finalize_disposition.sh
```

Expected from that last command: `1`. If it prints `0` the mutation did not land — stop and fix it, because the next command's result would be meaningless.

```bash
PATH=/usr/bin:$PATH bash tests/test_grep_portability.sh 2>&1 | grep -A3 'NOT OK'
```

Expected: a `NOT OK` line naming the bound, followed by an indented line containing
`tests/test_finalize_disposition.sh:186:{0,600}`.

Restore and confirm clean:

```bash
cp /tmp/fin-disp.bak tests/test_finalize_disposition.sh && rm /tmp/fin-disp.bak
git diff --cached --stat -- tests/test_finalize_disposition.sh
PATH=/usr/bin:$PATH bash tests/test_grep_portability.sh >/dev/null 2>&1; echo "restored exit=$?"
```

Expected: `restored exit=0`. Do not proceed until it is 0.

- [ ] **Step 6: Commit**

```bash
git add tests/test_grep_portability.sh
git commit -m "test(0130): guard against ERE repetition bounds above 255

BSD grep rejects a bound above 255, and a machine whose PATH grep is GNU grep
or ugrep runs the suite green while the bug is real. Static source scan over
every tracked path except docs/ — computed from git ls-files with no extension
filter (ADR-0050: derive, never re-enumerate). docs/ is excluded because
archived records, historical plans and published specs legitimately quote the
defective pattern and are immutable. The guard is in its own scanned
population and carries no over-threshold literal: its fixtures are assembled
at runtime. Positive, boundary (256) and negative (255) controls all route
through the same scan the main loop uses."
```

---

### Task 3: Cross-cutting verification — population mutation and the whole suite

**Files:**
- Mutate-and-restore only: one tracked non-`.sh` file, one tracked `docs/` file
- No file is modified by this task's deliverable

**Interfaces:**
- Consumes: `tests/test_grep_portability.sh` from Task 2, `tests/test_finalize_disposition.sh` from Task 1.
- Produces: evidence. This task ships no code; its deliverable is a recorded mutation matrix and a green whole-suite run under both toolchains.

**Why this is its own task.** ADR-0050's corollary: mutate a guard's **population**, not only its suppression. Task 2's controls prove the *predicate* fires against temp fixtures; they prove nothing about whether the **walk** actually reaches the files it claims to. Those are different failure modes, and the second is the one an extension filter or a broken pathspec produces. Separately, `AGENTS.md` requires running the whole suite at the build gate, never only the tests the spec enumerated — and this change's whole premise is that a green run under the wrong toolchain is not evidence.

- [ ] **Step 1: Population mutation — a tracked NON-`.sh` file must redden the guard**

Picking a `.sh` file here would leave the extension-axis blind spot unexposed, which is exactly the hole the no-extension-filter design exists to close. Use `AGENTS.md` — tracked, root-level, `.md`, outside `docs/`.

```bash
cd /Users/homer/dev/docket/.worktrees/make-the-finalize-marker-reachability-guard-portable-to-bsd
cp AGENTS.md /tmp/agents.bak
big="$(( 255 + 345 ))"
printf '\n<!-- population mutation probe: x.{0,%s}y -->\n' "$big" >> AGENTS.md
grep -c 'population mutation probe' AGENTS.md
```

Expected: `1` — the mutation landed.

```bash
PATH=/usr/bin:$PATH bash tests/test_grep_portability.sh 2>&1 | grep -A3 'NOT OK'
```

Expected: a `NOT OK` line, with an indented line containing `AGENTS.md:` and `{0,600}`. **A green run here means the walk never reaches root-level `.md` files** — a real defect, not a fixture problem.

Restore:

```bash
cp /tmp/agents.bak AGENTS.md && rm /tmp/agents.bak
git diff --stat -- AGENTS.md
```

Expected: empty output.

- [ ] **Step 2: Exclusion mutation — the same literal under `docs/` must stay GREEN**

This proves the `docs/` exclusion is the intended one and not an accident of the walk.

```bash
cp docs/adrs/README.md /tmp/adr-readme.bak
big="$(( 255 + 345 ))"
printf '\n<!-- exclusion probe: x.{0,%s}y -->\n' "$big" >> docs/adrs/README.md
grep -c 'exclusion probe' docs/adrs/README.md
```

Expected: `1`.

```bash
PATH=/usr/bin:$PATH bash tests/test_grep_portability.sh; echo "exit=$?"
```

Expected: `exit=0`, zero `NOT OK` lines. A red run here means the exclusion is not applied and the guard would demand repairs to immutable records.

Restore:

```bash
cp /tmp/adr-readme.bak docs/adrs/README.md && rm /tmp/adr-readme.bak
git diff --stat -- docs/adrs/README.md
```

Expected: empty output.

- [ ] **Step 3: Record the mutation matrix**

Write down, for the review step, the six cells and whether each matched prediction:

| # | Mutation | Predicted | Observed |
|---|---|---|---|
| 1 | Delete the marker-write clause from the finalize skill | only the reachability assert reddens | |
| 2 | Re-introduce `{0,600}` into `tests/test_finalize_disposition.sh` | guard reports it | |
| 3 | Guard file unstaged | only the self-membership assert reddens | |
| 4 | `{0,600}` appended to `AGENTS.md` (tracked, non-`.sh`) | guard reddens | |
| 5 | `{0,600}` appended to `docs/adrs/README.md` | guard stays green | |
| 6 | `{0,255}` fixture (negative control) | not flagged | |

Any cell that does not match prediction is a defect to fix before the PR, not a note to file.

- [ ] **Step 4: Whole suite under BSD grep**

There is no suite runner — invoke every test file.

```bash
cd /Users/homer/dev/docket/.worktrees/make-the-finalize-marker-reachability-guard-portable-to-bsd
red=0
for t in tests/test_*.sh; do
  if PATH=/usr/bin:$PATH bash "$t" >/tmp/out.$$ 2>&1; then :; else
    red=$(( red + 1 )); printf '=== RED: %s ===\n' "$t"; tail -20 /tmp/out.$$
  fi
done
rm -f /tmp/out.$$
echo "red files under BSD grep: $red"
```

Expected: `red files under BSD grep: 0`.

If a file is red for a reason unrelated to this change, verify it against the unmodified base before treating it as a regression — a red suite in a modified tree is a hypothesis, not a verdict:

```bash
git stash && PATH=/usr/bin:$PATH bash <the-red-test> ; git stash pop
```

- [ ] **Step 5: Whole suite under the ambient PATH**

```bash
red=0
for t in tests/test_*.sh; do
  if bash "$t" >/tmp/out.$$ 2>&1; then :; else
    red=$(( red + 1 )); printf '=== RED: %s ===\n' "$t"; tail -20 /tmp/out.$$
  fi
done
rm -f /tmp/out.$$
echo "red files under ambient PATH: $red"
```

Expected: `red files under ambient PATH: 0`. This is the run that would have been green all along — it is here to prove the fix did not trade one toolchain for the other, not as evidence for the fix.

- [ ] **Step 6: Confirm the working tree carries only the intended changes**

```bash
git status --porcelain
git diff origin/main --stat
```

Expected from `git diff origin/main --stat`: exactly three paths — `tests/test_finalize_disposition.sh` (1 insertion, 1 deletion), `tests/test_grep_portability.sh` (new), and `docs/superpowers/plans/2026-07-28-bsd-grep-interval-portability.md` (new). Expected from `git status --porcelain`: nothing beyond untracked scratch. **No mutation may survive into the branch.**

---

## Self-Review

**Spec coverage.**

| Spec requirement | Task |
|---|---|
| §What changes 1 — replace `.{0,600}` with `.*` | Task 1, Step 2 |
| §What changes 2 — new `tests/test_grep_portability.sh`, computed walk over `git ls-files`, `docs/` excluded, no extension filter, population-collapse sentinel | Task 2, Step 1 |
| §What changes 3 — no >255 literal in the guard, fixtures assembled at runtime, self-membership assert | Task 2, Step 1; the designed red at Step 2 |
| §What changes 4 — non-vacuity by fixture (600 flagged, 255 not), informational resolved-`grep` line | Task 2, Step 1 controls; Steps 3–4 |
| §What changes 5 — mutation-proof the repaired assertion | Task 1, Step 5 |
| §What changes 5 — mutate the guard's POPULATION: non-`.sh` tracked file reddens, `docs/` stays green | Task 3, Steps 1–2 |
| §Verification — `PATH=/usr/bin:$PATH` prepend, both files green under both PATHs, whole suite green | Task 1 Steps 3–4; Task 2 Steps 3–4; Task 3 Steps 4–5 |
| A8 — no ADR unless a non-obvious trade-off surfaces while building | Deferred to the review step, as the spec directs |

Two spec-adjacent items are deliberately *not* tasks: the four existing `docs/` occurrences (out of scope, and the exclusion is what protects them) and suite-wide toolchain pinning (out of scope per A4; captured as change 0150).

**Placeholder scan.** No `TBD`, no "add appropriate error handling", no "similar to Task N". Every code step carries the literal content. The one intentionally under-specified step is Task 3 Step 4's stash-and-recheck, which applies only if an unrelated test is red — a conditional diagnostic, not a deliverable.

**Type consistency.** `scan_file` and `offenders` are defined once in Task 2 Step 1 and referenced by those exact names in the controls and in Task 3's expectations. `MAX_BOUND`, `INTERVAL`, `SELF_REL`, `files_joined`, `violations`, `scanned` are each defined before first use. The file path `tests/test_grep_portability.sh` is spelled identically in the guard's `SELF_REL` construction, the probe list, the commit commands and Task 3's expectations.

**One risk carried forward, flagged for review.** Task 2 Step 1's `offenders` parses `scan_file` output by splitting on the first `:`. `grep -no` emits `lineno:match`, and the match is a brace interval that can contain no colon, so the split is unambiguous. If a future edit changes `scan_file`'s flags, that assumption breaks silently — worth a reviewer's eye.
