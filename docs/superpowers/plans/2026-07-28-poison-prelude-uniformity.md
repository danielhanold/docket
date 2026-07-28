<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0126 — Apply the poison-value prelude uniformly to every resolver eval in the config suite](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0126-apply-the-poison-value-prelude-uniformly-to-every-resolver-e.md)**
<!-- docket:backlink:end -->

# Poison-Prelude Uniformity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make every resolver-output `eval` in `tests/test_docket_config.sh` clear the exported variables its following asserts read, and add a correspondence guard that keeps it true.

**Architecture:** Three moves in strict order. First, *demonstrate* the hazard on the unmodified file at the natural `AUTO_GROOM` coincidence (evidence, no edit). Second, add the correspondence guard as a new section at the end of the file — it goes in **red**, reporting the real violations. Third, add the per-fixture poison preludes until the guard is green, then mutation-test the guard in both directions.

**Tech Stack:** Bash (`set -uo pipefail`), POSIX awk, the repo's existing `assert()` harness. No new files, no new dependencies.

## Global Constraints

Copied verbatim from the spec and from the reconcile pass; every task's requirements implicitly include this section.

- **Scope is exactly one file:** `tests/test_docket_config.sh`. No sibling suite is touched.
- **Idiom for newly-protected sites: poison assignment** — `VAR=__poison__`. Never `unset` + `${VAR-unset}` rewrites: `tests/test_docket_config.sh:4` sets `set -uo pipefail` and the `assert()` helper at `:8` is **not** subshell-wrapped, so an `unset` prelude without an assert rewrite aborts the whole harness on the first unbound read instead of recording one NOT OK.
- **Existing `unset`-idiom blocks stay byte-untouched** (spec assumption 2).
- **Clearing is per-fixture:** clear exactly the variables the following asserts read — never a blanket line.
- **Exemptions are derived, never enumerated.** A site is exempt exactly when the asserts between it and the next site read no exported variable.
- **Never hardcode a hand-count.** Two independent reviews of this file disagreed (64 vs 65). Every count in the guard is derived at runtime and cross-checked against a structurally different extractor, over a floor of `>= 60`.
- **Portability:** the repo's promoted `shell-portability` learning binds. Verify greps with `/usr/bin/grep`, not the PATH `grep` (which is ugrep and accepts patterns BSD grep rejects). Use `split("", arr)` to clear awk arrays, not `delete arr`. No `grep -P`, no GNU-only flags, no literal `\t` inside `grep -E` — use the repo's `"$(printf '^x\ty\t')"` idiom.
- **The guard's corpus is the WHOLE file** minus a single marker-delimited self-block. Never truncate at an end-of-file marker — the file's tail is exactly where new fixtures land.

## Verified Baseline (measured on this branch, 2026-07-28)

These numbers were produced by running the extractor against the real file at `origin/main`. They are the plan's predictions; each task re-derives rather than trusting them, but a large deviation means something moved and the implementer should stop and re-check rather than adjust the expected value.

| Quantity | Value | How it was derived |
|---|---|---|
| Discovered eval sites | **64** | the tokenizer in Task 2 |
| Raw `eval "$` line count | **66** | `/usr/bin/grep -c 'eval "\$' tests/test_docket_config.sh` |
| Difference accounted for | **2** | `:8` (the `assert()` helper's `eval "$2"` — a positional, not a cmdsub var) and `:131` (a comment) |
| Resolver export keys (shell format) | **28** | `bash scripts/docket-config.sh --export \| sed 's/=.*//'` — matches the existing E′ count assert |
| Sites exempt by derivation | **3** | no exported variable asserted in the segment |
| Sites already compliant | **21** | 12 poison-idiom + 9 existing `unset`-idiom blocks |
| Sites needing a prelude | **40** | the Task 3 work list |

Note `REPO_ROOT` is deliberately absent from the 28: the shell format omits it (`scripts/docket-config.sh:661–663`), which is exactly why the key set is derived from a live `--export` run rather than from grepping the resolver's 29 `emit` calls.

## File Structure

One file is modified. No files are created.

- **Modify:** `tests/test_docket_config.sh`
  - **Body (lines 1–~1811):** 40 one-line poison preludes inserted before existing eval sites. No fixture is restructured; no assert is rewritten.
  - **New section at end of file (before the final `if [ "$fail" = 0 ]` epilogue):** section **(T)**, the correspondence guard — a self-contained block bounded by `# docket:prelude-guard:self:start` / `# docket:prelude-guard:self:end` markers around its pattern literals.

Placement at the end keeps the guard travelling with the file it guards, and leaves change 0125's future rung-pair guard an adjacent section rather than a merge conflict.

---

### Task 1: Demonstrate the hazard on the unmodified file

The completion bar the stub set: *prove* the vacuous pass rather than assert it. This task changes no committed bytes — its deliverable is recorded evidence, which Task 5 writes into the results file. Do it first, because it is only reproducible **before** the preludes land.

The demonstration cannot run at the site the stub named. At `:501` the previously-eval'd fixture leaves `BOARD_SURFACES=none`, so an aborting resolver there makes the assert go **red**, not vacuously green. A natural coincidence exists instead at the O→P boundary, verified present on this branch:

- `:509` evals block O, which leaves `AUTO_GROOM=false`; `:511` asserts it.
- `:518` evals block P; `:520` asserts `[ "$AUTO_GROOM" = false ]`.
- Nothing writes `AUTO_GROOM` between `:509` and `:520`.

**Files:**
- Temporarily modify (reverted before commit): `tests/test_docket_config.sh`
- No test file — this task *is* a test of the file's current state.

**Interfaces:**
- Consumes: nothing.
- Produces: three recorded observations (baseline / vacuous-pass / prelude-fixes-it) plus their ok / NOT-OK counts, consumed by Task 5's results file.

- [ ] **Step 1: Record the clean baseline**

```bash
cd /Users/homer/dev/docket/.worktrees/apply-the-poison-value-prelude-uniformly-to-every-resolver-e
bash tests/test_docket_config.sh > /tmp/poison-baseline.txt 2>&1; echo "rc=$?"
printf 'ok=%s notok=%s\n' \
  "$(/usr/bin/grep -c '^ok - ' /tmp/poison-baseline.txt)" \
  "$(/usr/bin/grep -c '^NOT OK - ' /tmp/poison-baseline.txt)"
/usr/bin/grep -c '^PASS$' /tmp/poison-baseline.txt
```

Expected: `rc=0`, `notok=0`, and one `PASS` line. Record the `ok` count — it is the denominator for the next two steps. If the baseline is already red, **stop and report**: per the `environment` learning, a red suite is a hypothesis, not a verdict, and the demonstration is meaningless on a broken base.

- [ ] **Step 2: Make block P's resolver abort, and watch `:520` pass vacuously**

Every `die` in `scripts/docket-config.sh` precedes the first `emit`, so an aborting run emits nothing on stdout and `eval ""` is a no-op. Break block P's repo so its resolver dies — an unresolvable `origin/<integration_branch>` is a hard config error. Insert the sabotage immediately **after** the `mkdir -p "$tmp/p.xdg/docket/config.yml"` line (currently `:516`):

```bash
git -C "$tmp/p" remote set-url origin /nonexistent/definitely-not-a-repo.git
git -C "$tmp/p" update-ref -d refs/remotes/origin/main 2>/dev/null || true
```

Then run and read **`:520`'s assert alone**:

```bash
bash tests/test_docket_config.sh 2>&1 | /usr/bin/grep -F '0050 P: built-ins fallback (auto_groom)'
```

Expected: `ok - 0050 P: built-ins fallback (auto_groom)`.

That line is the whole point: block P's resolver produced **nothing**, so the assert passed by reading **block O's** stale `AUTO_GROOM=false`. Record the full ok / NOT-OK counts too, and record explicitly that the sibling asserts `0050 P: malformed global warned` (`:519`) and `0050 P: malformed global not fatal (exit 0)` (`:521`) legitimately go NOT OK under this sabotage. **The counts must not be read as "only `:520` changed"** — the demonstration is about `:520` alone.

- [ ] **Step 3: Add the prelude and watch the same sabotage redden it**

Keeping the Step-2 sabotage in place, insert immediately before the eval at `:518`:

```bash
AUTO_GROOM=__poison__
```

Run the same single-line check:

```bash
bash tests/test_docket_config.sh 2>&1 | /usr/bin/grep -F '0050 P: built-ins fallback (auto_groom)'
```

Expected: `NOT OK - 0050 P: built-ins fallback (auto_groom)`.

The prelude converted a silent vacuous pass into a visible failure. That is the property the whole change buys, demonstrated on the real file rather than argued.

- [ ] **Step 4: Revert both mutations and confirm the baseline returns**

```bash
git -C /Users/homer/dev/docket/.worktrees/apply-the-poison-value-prelude-uniformly-to-every-resolver-e checkout -- tests/test_docket_config.sh
git status --porcelain tests/test_docket_config.sh
bash tests/test_docket_config.sh 2>&1 | tail -1
```

Expected: `git status` prints nothing (file is clean), and the suite prints `PASS`. Per the `agent-shell-noop-reads-as-success` learning, assert on the **effect** — a clean `git status` — not on the fact that `checkout` exited 0.

- [ ] **Step 5: Commit**

Nothing to commit — this task deliberately produces no file changes. Verify that:

```bash
git -C /Users/homer/dev/docket/.worktrees/apply-the-poison-value-prelude-uniformly-to-every-resolver-e status --porcelain
```

Expected: empty (or showing only this plan file). Carry the three recorded observations forward to Task 5.

---

### Task 2: The correspondence guard (goes in RED)

Add section (T). It discovers eval sites by **shape**, extracts the exported variables each site's asserts read, and requires each to be cleared in that site's clearing window. It is expected to report ~40 violations on arrival — that is TDD, not a defect. Task 3 makes it green.

**Files:**
- Modify: `tests/test_docket_config.sh` — append section (T) immediately before the final `if [ "$fail" = 0 ]; then echo PASS; else echo FAIL; fi` epilogue.

**Interfaces:**
- Consumes: the existing `assert()` helper (`:8`), `$REPO`, `$SCRIPT`, `$tmp`.
- Produces: a shell function `prelude_report()` writing `SITE <line> <status> [missing vars]` records plus a `TOTALS sites=<n> exempt=<n> ok=<n> viol=<n>` line — consumed by Task 4's mutation tests.

**Three design decisions this task locks in, each with its rejected alternative:**

1. **The clearing window is "since the previous eval site", not "the preceding line".** The hazard is a stale value left by the *previous fixture's* eval, so any clearing between that eval and this one kills it. The narrow adjacency rule was measured and is wrong: it reports the 9 existing `unset`-idiom sites (the learnings block at `:809`–`:854`, the reclaim block at `:937`–`:969`) as violations, because their `unset` sits several lines upstream ahead of `mkrepo` and a heredoc. Adjacency would therefore force edits to blocks that spec assumption 2 says stay byte-untouched. The wide window models the real invariant and reconciles the guard with the spec: measured, it moves those 9 sites from violation to compliant with zero edits.
2. **Correspondence, never presence.** A presence-only guard is green on precisely the regression this change exists to stop — append a fixture asserting `BOARD_SURFACES`, prepend `FINALIZE_TEST_COMMAND=__poison__`, hazard live, suite green. That would mechanically bless a wrong convention.
3. **The key set is derived live from the resolver**, while the asserted names are extracted from the test file — so the two sides of the correspondence are genuinely independent. Not from the E′ assert (which pins a *count* and names nothing), and not from grepping `emit` (29 calls; the shell format omits `REPO_ROOT`, so an `emit`-derived set carries a key no `eval "$out"` site can ever define).

- [ ] **Step 1: Write the failing test**

Append this section to `tests/test_docket_config.sh`, immediately before the final epilogue line. Note the self-block markers wrapping the pattern literals — the guard's own patterns contain the strings it scans for, so without them it discovers itself.

```bash
# --- (T) prelude correspondence guard (change 0126) ---------------------------
# Every `eval "$V"` of resolver output must clear the exported variables the
# asserts between it and the NEXT eval site read. The window is "anything since
# the previous eval site", not "the preceding line": the hazard is a stale value
# left by the PREVIOUS fixture's eval, so a clearing anywhere in between kills
# it. That is also what lets the pre-existing `unset`-idiom blocks satisfy this
# guard byte-untouched (change 0126, spec assumption 2).
#
# Corpus is the WHOLE file minus this section's own marker-delimited self-block.
# Deliberately NOT truncated at an end-of-file marker: the file's tail is where
# new fixtures land, so truncation would make them permanently invisible.

prelude_report(){
  local file="$1" keys="$2"
  awk -v keys="$keys" '
    { L[NR] = $0 }
    END {
      n = NR
      split(keys, ka, " "); for (i in ka) KEY[ka[i]] = 1

      # --- locate this guards own self-block (FIRST occurrence of each marker;
      # the literal necessarily appears twice - the marker and the pattern that
      # searches for it).
      sstart = 0; send = 0
      for (i = 1; i <= n; i++) {
        if (sstart == 0 && index(L[i], SELFSTART) > 0) sstart = i
        else if (sstart != 0 && send == 0 && index(L[i], SELFEND) > 0) send = i
      }

      # --- discover sites: eval "$V" where V came from a command substitution
      ns = 0
      split("", cmdsub)
      for (i = 1; i <= n; i++) {
        if (sstart != 0 && i >= sstart && i <= send) continue
        line = L[i]; s = line; sub(/^[ \t]+/, "", s)
        if (s ~ /^#/) continue
        tmp = line
        while (match(tmp, /[A-Za-z_][A-Za-z0-9_]*="?\$\(/)) {
          seg = substr(tmp, RSTART, RLENGTH); sub(/=.*$/, "", seg); cmdsub[seg] = 1
          tmp = substr(tmp, RSTART + RLENGTH)
        }
        tmp = line
        while (match(tmp, /eval[ \t]+"\$[A-Za-z_][A-Za-z0-9_]*"/)) {
          seg = substr(tmp, RSTART, RLENGTH); v = seg
          sub(/^eval[ \t]+"\$/, "", v); sub(/"$/, "", v)
          if (v in cmdsub) { ns++; SL[ns] = i }
          tmp = substr(tmp, RSTART + RLENGTH)
        }
      }

      exempt = 0; okc = 0; viol = 0
      for (k = 1; k <= ns; k++) {
        lo = SL[k]; hi = (k < ns ? SL[k+1] - 1 : n)

        # asserted exported variables in this segment (matches ${VAR-unset} too,
        # otherwise every existing unset-idiom assert reads as an empty
        # intersection and is wrongly exempted)
        split("", need)
        for (i = lo; i <= hi; i++) {
          if (sstart != 0 && i >= sstart && i <= send) continue
          s = L[i]; t = s; sub(/^[ \t]+/, "", t); if (t ~ /^#/) continue
          tmp = s
          while (match(tmp, /\$\{?[A-Za-z_][A-Za-z0-9_]*/)) {
            w = substr(tmp, RSTART, RLENGTH); sub(/^\$\{?/, "", w)
            if (w in KEY) need[w] = 1
            tmp = substr(tmp, RSTART + RLENGTH)
          }
        }

        # clearing window: since the previous site, through this eval line
        split("", cleared)
        wlo = (k > 1 ? SL[k-1] + 1 : 1)
        for (i = wlo; i <= lo; i++) {
          if (sstart != 0 && i >= sstart && i <= send) continue
          c = L[i]; q = c; sub(/^[ \t]+/, "", q); if (q ~ /^#/) continue
          tmp = c
          while (match(tmp, /[A-Za-z_][A-Za-z0-9_]*=(__poison__|"")/)) {
            w = substr(tmp, RSTART, RLENGTH); sub(/=.*$/, "", w); cleared[w] = 1
            tmp = substr(tmp, RSTART + RLENGTH)
          }
          if (c ~ /(^|[;&| \t])unset[ \t]/) {
            tmp = c; sub(/^.*unset[ \t]+/, "", tmp)
            nf = split(tmp, uu, /[ \t;]+/)
            for (j = 1; j <= nf; j++) if (uu[j] ~ /^[A-Z_][A-Z0-9_]*$/) cleared[uu[j]] = 1
          }
        }

        cnt = 0; miss = ""
        for (w in need) { cnt++; if (!(w in cleared)) miss = miss " " w }
        if (cnt == 0)        { exempt++; print "SITE " SL[k] " exempt" }
        else if (miss == "") { okc++;    print "SITE " SL[k] " ok" }
        else                 { viol++;   print "SITE " SL[k] " viol" miss }
      }
      print "TOTALS sites=" ns " exempt=" exempt " ok=" okc " viol=" viol
    }
  ' SELFSTART="$T_SELF_START" SELFEND="$T_SELF_END" "$file"
}

# docket:prelude-guard:self:start
# The two marker literals below are the guard's own scan patterns. Everything
# between the start and end marker is subtracted from the corpus.
T_SELF_START='docket:prelude-guard:self:start'
T_SELF_END='docket:prelude-guard:self:end'
T_EVAL_LITERAL='eval "$'
# docket:prelude-guard:self:end

t_keys="$(bash "$SCRIPT" --export 2>/dev/null | sed 's/=.*//' | sort | tr '\n' ' ')"
t_out="$(prelude_report "${BASH_SOURCE[0]}" "$t_keys")"
t_sites="$(printf '%s\n' "$t_out" | sed -n 's/^TOTALS sites=\([0-9]*\) .*/\1/p')"
t_viol="$(printf '%s\n' "$t_out" | sed -n 's/^TOTALS .* viol=\([0-9]*\)$/\1/p')"

# Population floor, from a STRUCTURALLY DIFFERENT extractor: a plain grep of the
# raw literal, minus the two known non-sites (the assert() helper at :8, whose
# eval takes a positional rather than a cmdsub var, and the explanatory comment).
# Two counts from one parser would be tautological.
t_raw="$(/usr/bin/grep -cF "$T_EVAL_LITERAL" "${BASH_SOURCE[0]}")"
t_helper="$(/usr/bin/grep -cE '^assert\(\)\{' "${BASH_SOURCE[0]}")"
t_selfrefs="$(awk -v s="$T_SELF_START" -v e="$T_SELF_END" '
  index($0,s)>0 && !st {st=NR} index($0,e)>0 && st && !en {en=NR}
  END{ if (st && en) print en-st+1; else print 0 }' "${BASH_SOURCE[0]}")"

assert "0126 T: guard reached a real population (>= 60 sites)" '[ "$t_sites" -ge 60 ]'
assert "0126 T: site count agrees with the independent grep extractor" \
  '[ "$t_sites" -eq "$(( t_raw - t_helper - 1 ))" ]'
assert "0126 T: the self-block is bounded and non-empty" '[ "$t_selfrefs" -ge 3 ]'
assert "0126 T: every eval site clears the exported vars its asserts read" \
  '[ "$t_viol" -eq 0 ]'
```

- [ ] **Step 2: Run it to verify it fails for the right reason**

```bash
cd /Users/homer/dev/docket/.worktrees/apply-the-poison-value-prelude-uniformly-to-every-resolver-e
bash tests/test_docket_config.sh 2>&1 | /usr/bin/grep -E '^(ok|NOT OK) - 0126 T:'
```

Expected: the three structural asserts (`>= 60 sites`, `site count agrees`, `self-block is bounded`) are **ok**, and only `every eval site clears...` is **NOT OK**.

That split is the point. If the *count* asserts fail, the extractor is mis-bound and must be fixed before the violations mean anything — do not proceed to Task 3 on a guard whose corpus is wrong. Confirm the violation count directly:

```bash
bash tests/test_docket_config.sh 2>&1 | /usr/bin/grep '^TOTALS' || true
```

Expected: `sites=64 exempt=3 ok=21 viol=40`. A deviation of a site or two is tolerable (the file moves); a large one means re-derive before continuing.

- [ ] **Step 3: No implementation yet**

This task deliberately ships the guard red. Task 3 is the implementation that turns it green. Do not weaken any assert to make this step pass — that is the `guards-are-code` failure the whole change exists to prevent.

- [ ] **Step 4: Verify the guard cannot see past its own corpus bound**

Before committing, prove the self-block subtraction did not silently eat the file's tail. Append a throwaway compliant site **below** the guard and confirm the site count rises:

```bash
cp tests/test_docket_config.sh /tmp/t-probe.sh
printf '\nBOARD_SURFACES=__poison__\nprobe_out="$(bash "$SCRIPT" --export 2>/dev/null)"; eval "$probe_out"\n' >> /tmp/t-probe.sh
awk 'NR==FNR{next}1' /dev/null /tmp/t-probe.sh > /dev/null   # syntax sanity
bash /tmp/t-probe.sh 2>&1 | /usr/bin/grep '^TOTALS'
```

Expected: `sites=65` — one more than the committed file. If it still reads 64, the corpus is truncated and the guard has a permanent blind spot exactly where new fixtures land. Delete `/tmp/t-probe.sh` afterward.

- [ ] **Step 5: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/apply-the-poison-value-prelude-uniformly-to-every-resolver-e
git add tests/test_docket_config.sh docs/superpowers/plans/2026-07-28-poison-prelude-uniformity.md
git commit -m "test(0126): add the prelude correspondence guard (red: 40 violations)"
```

---

### Task 3: Apply the per-fixture poison preludes

Turn the guard green by clearing, at each violating site, exactly the variables that site's asserts read.

**Files:**
- Modify: `tests/test_docket_config.sh` — one inserted line per violating site (~40).

**Interfaces:**
- Consumes: `prelude_report()`'s `SITE <line> viol <vars...>` records from Task 2.
- Produces: a green guard; no new symbols.

- [ ] **Step 1: Generate the work list from the guard itself**

Do not hand-enumerate the sites — the guard already knows them, and a hand list is the enumerated floor this repo has been bitten by repeatedly.

```bash
cd /Users/homer/dev/docket/.worktrees/apply-the-poison-value-prelude-uniformly-to-every-resolver-e
bash tests/test_docket_config.sh 2>&1 | /usr/bin/grep '^SITE .* viol' > /tmp/worklist.txt
wc -l < /tmp/worklist.txt
cat /tmp/worklist.txt
```

Expected: ~40 lines, each `SITE <line> viol VAR [VAR...]`.

- [ ] **Step 2: Insert the preludes, highest line number first**

Work **bottom-up** so earlier insertions do not shift the line numbers of later ones. For each record `SITE <n> viol V1 V2`, insert immediately before line `<n>`, at the same indentation as line `<n>`:

```bash
V1=__poison__; V2=__poison__
```

Written as one line per site, listing exactly that site's variables and no others. Copying section S's `FINALIZE_TEST_COMMAND=__poison__` onto a fixture whose assert reads `BOARD_SURFACES` is decoration and the guard will still report it.

Per the `agent-shell-noop-reads-as-success` learning, run any scripted sweep under an explicit `bash -c` and assert on the effect (`git diff --stat`), never on the sweep's exit code. A zero-iteration loop prints success just as happily as a working one.

Do **not** touch the existing `unset`-idiom blocks. They are already compliant under the wide window; the guard proves it, and spec assumption 2 requires it.

- [ ] **Step 3: Run the guard to verify it passes**

```bash
bash tests/test_docket_config.sh 2>&1 | /usr/bin/grep -E '^(ok|NOT OK) - 0126 T:'
bash tests/test_docket_config.sh 2>&1 | /usr/bin/grep '^TOTALS'
```

Expected: all four `0126 T:` asserts **ok**, and `TOTALS sites=64 exempt=3 ok=61 viol=0`.

- [ ] **Step 4: Run the whole suite and confirm no fixture regressed**

The preludes are inserted into live fixtures; a prelude clearing a variable a *preceding* assert still needed would redden that assert.

```bash
bash tests/test_docket_config.sh > /tmp/poison-after.txt 2>&1; echo "rc=$?"
printf 'ok=%s notok=%s\n' \
  "$(/usr/bin/grep -c '^ok - ' /tmp/poison-after.txt)" \
  "$(/usr/bin/grep -c '^NOT OK - ' /tmp/poison-after.txt)"
tail -1 /tmp/poison-after.txt
```

Expected: `rc=0`, `notok=0`, `PASS`, and an `ok` count equal to the Task 1 baseline **plus 4** (the four new `0126 T:` asserts). An `ok` count that did not rise by exactly 4 means a fixture changed behavior — investigate before proceeding.

- [ ] **Step 5: Commit**

```bash
git add tests/test_docket_config.sh
git commit -m "test(0126): clear the asserted exported vars at every eval site"
```

---

### Task 4: Mutation-test the guard in both directions

A guard that has never been observed to fail is an untested claim. The critical direction is the second one — it is the property presence-only guards miss, and it is the entire justification for building a correspondence guard rather than a cheap presence check.

**Files:**
- Temporarily modify (each reverted): `tests/test_docket_config.sh`
- No new files.

**Interfaces:**
- Consumes: the green guard from Task 3.
- Produces: a four-cell mutation matrix recorded for Task 5's results file.

- [ ] **Step 1: Mutation A — delete a prelude**

Remove any one prelude line added in Task 3, then:

```bash
bash tests/test_docket_config.sh 2>&1 | /usr/bin/grep -E 'NOT OK - 0126 T:|^TOTALS'
```

Expected: `viol=1`, and `NOT OK - 0126 T: every eval site clears the exported vars its asserts read`. Revert.

- [ ] **Step 2: Mutation B — clear the WRONG variable (the load-bearing cell)**

Take a prelude such as `BOARD_SURFACES=__poison__` and change it to `FINALIZE_TEST_COMMAND=__poison__` — a prelude that is *present* but clears a variable the following asserts never read.

```bash
bash tests/test_docket_config.sh 2>&1 | /usr/bin/grep -E 'NOT OK - 0126 T:|^TOTALS'
```

Expected: `viol=1` naming `BOARD_SURFACES`, and the correspondence assert **NOT OK**.

A presence-only guard would be **green** here. If this cell does not redden, the guard is presence-only in effect and must be fixed or, per the spec's fallback, removed entirely with the reason recorded — never shipped as a presence guard wearing a correspondence guard's name. Revert.

- [ ] **Step 3: Mutation C — a new unprotected fixture appended at the file's tail**

This is the regression the change exists to stop: the next author appends a fixture and forgets the prelude.

```bash
cp tests/test_docket_config.sh /tmp/t-tail.sh
printf '\ntail_out="$(bash "$SCRIPT" --export 2>/dev/null)"; eval "$tail_out"\nassert "probe" %s\n' "'[ \"\$BOARD_SURFACES\" = x ]'" >> /tmp/t-tail.sh
bash /tmp/t-tail.sh 2>&1 | /usr/bin/grep -E 'NOT OK - 0126 T:|^TOTALS'
```

Expected: `sites=65`, `viol=1` naming `BOARD_SURFACES`. This is simultaneously the coverage proof and the proof the corpus is not truncated. Delete `/tmp/t-tail.sh`.

- [ ] **Step 4: Mutation D — break the corpus bound**

Confirm the count asserts actually catch a mis-bound extractor. Move the `# docket:prelude-guard:self:end` marker up so it swallows part of the guard, then:

```bash
bash tests/test_docket_config.sh 2>&1 | /usr/bin/grep -E 'NOT OK - 0126 T:'
```

Expected: at least one of the structural asserts (`>= 60 sites` or `site count agrees`) goes **NOT OK** rather than the suite quietly reporting a smaller population as success. Revert and re-confirm green.

- [ ] **Step 5: Commit**

No committed change — mutations are all reverted. Verify and record the matrix:

```bash
git status --porcelain tests/test_docket_config.sh   # expect empty
bash tests/test_docket_config.sh 2>&1 | tail -1      # expect PASS
```

---

### Task 5: Results file and close-out

**Files:**
- Create: `docs/results/2026-07-28-apply-the-poison-value-prelude-uniformly-to-every-resolver-e-results.md`

**Interfaces:**
- Consumes: Task 1's three observations, Task 3's counts, Task 4's four-cell matrix.
- Produces: the merge-gate record.

- [ ] **Step 1: Write the results file**

Record, from the `results-template.md` shape:

- The Task 1 demonstration: the exact `:520` assert text under baseline, under sabotage (**ok** — the vacuous pass), and under sabotage + prelude (**NOT OK**), with the ok / NOT-OK counts and the explicit note that `:519` and `:521` legitimately reddened under the sabotage, so the counts must not be read as "only `:520` changed".
- The derived population: 64 sites, cross-checked at `66 - 2` by the independent grep extractor, against the `>= 60` floor. State plainly that no count was hand-written.
- The 3 / 21 / 40 split, and that the 9 existing `unset`-idiom sites are compliant **byte-untouched** because the clearing window is "since the previous eval site" rather than "the preceding line" — a documented departure from the spec's narrower phrasing, made because the narrow rule would have forced edits the spec's assumption 2 forbids.
- Task 4's four-cell matrix, calling out Mutation B as the cell that distinguishes this guard from a presence-only one.
- Whether the guard shipped at all. If correspondence had proved infeasible, the spec's fallback is **no guard**, reason recorded — never a presence-only one.

- [ ] **Step 2: Stamp the artifact back-link**

```bash
"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh render-artifact-backlink \
  --artifact-file .worktrees/apply-the-poison-value-prelude-uniformly-to-every-resolver-e/docs/results/2026-07-28-apply-the-poison-value-prelude-uniformly-to-every-resolver-e-results.md \
  --change-file .docket/docs/changes/active/0126-apply-the-poison-value-prelude-uniformly-to-every-resolver-e.md
```

- [ ] **Step 3: Run the full repo suite**

One foreground run, no backgrounding.

- [ ] **Step 4: Commit**

```bash
git add docs/results/
git commit -m "docs(0126): results — hazard demonstrated, guard mutation-tested both ways"
```

---

## Self-Review

**Spec coverage.** Design §1 per-fixture rule → Task 3 Step 2. §2 poison idiom → Global Constraints + Task 3. §3 mutation demonstration → Task 1 (all three observations, at the O→P coincidence the spec names, with the `:519`/`:521` caveat carried into Task 5). §4 enforcement guard → Task 2: shape-keyed discovery ✓, correspondence with `${VAR-unset}` matching ✓, live-derived key set ✓, population floor from a structurally different extractor ✓, self-reference via marker-delimited self-block with no end-of-file truncation ✓, both-direction mutation testing → Task 4. Assumption 9 (no ADR) → no ADR task. Fallback (ship no guard if correspondence is infeasible) → Task 4 Step 2 and Task 5 Step 1.

**One deliberate departure from the spec, recorded rather than silent:** the clearing window is "since the previous eval site", not "same line or preceding non-blank line". Measured on the real file, the narrow rule flags the 9 existing `unset`-idiom sites as violations, which would force edits that spec assumption 2 explicitly forbids. The wide window also models the actual hazard more faithfully. Task 2's decision list and Task 5's results file both carry this.

**Placeholder scan.** No TBDs. Every code step carries runnable code. Task 3 Step 2 intentionally describes a per-site edit rather than listing 40 literal diffs — the site list is *generated by the guard* in Step 1, which is the enumerated-floor-avoiding form, not a placeholder.

**Type consistency.** `prelude_report()` is defined in Task 2 and consumed by name in Tasks 3 and 4. The record formats (`SITE <line> <status>`, `TOTALS sites= exempt= ok= viol=`) are identical everywhere they appear. Variables `T_SELF_START` / `T_SELF_END` / `T_EVAL_LITERAL` are defined once inside the self-block and referenced consistently.

**Plan-supplied test code status.** Per the `plan-supplied-test-code-is-unverified` learning, the extractor in Task 2 is **not** a draft: it was prototyped against the real file on this branch before this plan was written, and its predicted output (`sites=64 exempt=3 ok=21 viol=40`) is a measured value, not an estimate. The implementer should still run it as code — but it starts from a verified position rather than an authored one.
