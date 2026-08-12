<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0221 — assert() executes backticks in its test description, so a verbatim-quoted anchor can run shell](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-12-0221-assert-executes-backticks-in-its-test-description-so-a-verba.md)**
<!-- docket:backlink:end -->

# Test-source backtick hygiene: canonical asserts + an enforced pre-execution gate — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stop test-source backticks from executing during a suite run, by normalizing every assert-family definition to a canonical non-interpolating form and gating the runner on a standalone source-hygiene checker that aborts before any test file is launched.

**Architecture:** One new standalone checker (`scripts/check-test-source-hygiene.sh`) implements two rules — a byte-exact allowlist of assert-family *definitions*, and a whole-file shell-quoting state machine that finds backticks in positions where the shell (or a later `eval`) would execute them. `scripts/run-tests.sh` calls it synchronously over its targets before the first job launches, so a violation aborts the run with **zero** test files executed. `tests/test_assert_hygiene.sh` is the checker's own regression test, driving it against committed red/green fixtures under `tests/fixtures/hygiene/` — deliberately outside the runner's `tests/test_*.sh` glob so a red fixture is never launched as a test.

**Tech Stack:** bash 3.2-compatible shell, awk (BSD/macOS and GNU), the repo's existing `scripts/run-tests.sh` harness and `tests/runtime-budgets.tsv` budget table.

## Global Constraints

- **The executing vector is source evaluation, not the helper's print.** Parameter expansion does not re-trigger command substitution: a backtick held in a *variable's value* is inert through `echo "ok - $1"`. Normalizing the helper is ledger alignment and drift control; it is **not** the safety mechanism. Never write a comment or test name claiming otherwise.
- **Derive every site list from a fresh whole-repo grep, never from a number in this plan or the spec.** Counts here are point-in-time (origin/main @ `ddd5ffc7`, 2026-08-11) and have already drifted twice.
- **Key every guard on syntactic shape, never an enumerated list of spellings** (AGENTS.md). The spelling you miss is the next file's house idiom.
- **A guard is code: mutation-test it.** Strip the thing it guards and watch it redden, or it is decoration. Confirm a mutation actually **changed bytes** — check exact removed/added line counts, never a token count.
- **`grep` on PATH is ugrep and diverges from `/usr/bin/grep`.** Re-check every probe grep under `/usr/bin/grep` before trusting a green.
- **Never `producer | early-exiting-consumer` under `set -o pipefail`** — capture into a variable, then `grep <<<"$var"`. `tests/test_pipe_shapes.sh` enforces this repo-wide and will redden on a violation in new code.
- **`ok - ` / `NOT OK - ` at column 1 is a runner contract**, not taste: `scripts/run-tests.sh` greps `^NOT OK` for failure accounting.
- **`eval "$2"` is retained at every call site.** Rewriting call sites to a non-eval form is out of scope.
- **New top-level `scripts/<name>.sh` requires a co-located `scripts/<name>.md`** or `tests/test_script_contracts_coverage.sh` reddens.
- **Run the whole suite at the build gate** via `scripts/run-tests.sh`, never only the files a task names. A trailing `OVER BUDGET:` line is a finding to act on.
- **Do NOT run `scripts/run-tests.sh --timings <test path>`** against a real test file — it truncates the named file to zero bytes (#0290, unfixed). Measure runtime another way (`time bash tests/<file>.sh`).
- **Known flake:** `tests/test_gate_run_stop.sh` at "the stop is held where the completed marker would be written" is flake #0293, reproduces on clean origin/main. Re-run and proceed; do not absorb it.

## File Structure

| File | Responsibility |
|---|---|
| `scripts/check-test-source-hygiene.sh` | **Create.** The standalone checker. Rule (a) definition allowlist + rule (b) quoting scanner. Exits 0 clean, 1 on violations (naming `file:line:class`), 2 on usage error. |
| `scripts/check-test-source-hygiene.md` | **Create.** Its contract (Purpose / Usage / Behavior / Exit codes / Invariants). Required by `test_script_contracts_coverage.sh`. |
| `tests/fixtures/hygiene/red/*.sh`, `tests/fixtures/hygiene/green/*.sh` | **Create.** Mutation fixtures. Data handed to the checker — never sourced, never launched. Outside the `tests/test_*.sh` glob by construction. |
| `tests/test_assert_hygiene.sh` | **Create.** The checker's regression test: asserts each fixture's verdict in both directions, plus the side-effect sentinel. |
| `tests/runtime-budgets.tsv` | **Modify.** One new row for `test_assert_hygiene.sh`, measured. |
| `scripts/run-tests.sh` | **Modify.** Synchronous preflight call after target validation, before the budget table and the first launch. New exit code 5. |
| `scripts/run-tests.md` | **Modify.** Exit-code table row for 5; a paragraph on the preflight and its standalone-run limitation. |
| ~88 `tests/**/*.sh` files | **Modify.** D1 normalization of assert-family definitions, plus whatever the calibration pass turns up. |
| `tests/README.md` | **Modify.** The quoting rule, the enforcement point, the standalone-run limitation. |
| `docs/changes/learnings/test-helper-interpolates-its-own-description.md` | **Modify — on the `docket` metadata branch ONLY.** See Task 8's boundary note. |

---

### Task 1: The quoting scanner (rule b) and its fixtures

The heart of the change. A whole-file character-state machine that finds backticks the shell would execute.

**Files:**
- Create: `scripts/check-test-source-hygiene.sh`
- Create: `scripts/check-test-source-hygiene.md`
- Create: `tests/fixtures/hygiene/red/dq_description.sh`, `red/dq_sites_block.sh`, `red/heredoc_plain.sh`, `red/heredoc_dash.sh`, `red/sq_condition_unescaped.sh`, `red/dq_condition_escaped.sh`, `red/normal_substitution.sh`, `red/sentinel.sh`
- Create: `tests/fixtures/hygiene/green/house_idiom.sh`, `green/comments.sh`, `green/quoted_heredoc.sh`, `green/continuation.sh`, `green/multiline_sq.sh`
- Test: `tests/test_assert_hygiene.sh` (created in Task 3; this task verifies by direct invocation)

**Interfaces:**
- Consumes: nothing.
- Produces: `scripts/check-test-source-hygiene.sh <path>...` — scans each path, prints one `<path>:<line>: <CLASS>: <message>` line per violation to stdout, exits **0** clean / **1** violations found / **2** usage (no paths, unreadable path). Violation class tokens, which Task 3's asserts and Task 5's wiring both key on: `NORMAL-BACKTICK`, `DQ-BACKTICK`, `HEREDOC-BACKTICK`, `EVAL-BACKTICK`, `DEFN-DRIFT` (added in Task 2).

**The state machine — design notes the implementer must honor:**

State is carried **across lines**, not reset per line. This is load-bearing, not an optimization: change 0212's actual incident was a backtick inside a multi-line double-quoted `SITES="…"` assignment, and a per-line scanner cannot see it. Line-local scanning is therefore not an available shrink.

States: `NORMAL`, `SQ`, `DQ`, plus heredoc-body mode.

- In `NORMAL`: a backslash escapes the next character (including a newline — continuation). `#` starts a comment **only when it begins a word** (start of line, or preceded by whitespace, `;`, `|`, `&`, `(`) — `foo#bar` is not a comment; the rest of the physical line is skipped. `'` → `SQ`. `"` → `DQ`. A backtick → **`NORMAL-BACKTICK`**.
- In `SQ`: nothing escapes — not even a backslash. Only `'` returns to `NORMAL`. Backticks here are inert *as data*; they are only checked by the call-aware eval rule below.
- In `DQ`: `\` escapes the next character. `"` returns to `NORMAL`. A **bare** backtick → **`DQ-BACKTICK`** (executes at source evaluation). A **backslash-escaped** backtick → also **`DQ-BACKTICK`** (the escape is consumed at source evaluation, so a bare backtick reaches `$2` and executes at `eval`). Both spellings are the same violation class deliberately — the spec rejects the previously "accepted residual".
- **Heredocs.** While in `NORMAL`, detect `<<` or `<<-` followed by optional whitespace and a delimiter word. A delimiter that is quoted in any way — `<<'X'`, `<<"X"`, `<<\X` — makes the body **inert**: skip lines until the terminator. An unquoted delimiter — `<<X`, `<<-X` — makes the body **live**: scan its lines for bare backticks (→ **`HEREDOC-BACKTICK`**) but do *not* apply quote-state transitions inside it, since a heredoc body is not shell-quoted text. Multiple heredocs may be queued on one line; process them in order after the line ends. For `<<-`, the terminator may be preceded by tabs. `<<` immediately followed by `<` is `<<<` (a here-string) — **not** a heredoc; do not consume it as one.
- **The call-aware eval rule.** On encountering, in `NORMAL` state at a command position, a word that is an assert-family helper name (`assert`, `ok`, `no`, `nok` — derived, see below), begin argument tracking: count top-level argument boundaries (runs of unquoted whitespace). Within **argument 2** — the condition, the string that reaches `eval "$2"` — a backtick inside an `SQ` region that is **not** immediately preceded by a backslash is **`EVAL-BACKTICK`**. The house idiom `'grep -qF "\`span\`" "$f"'` carries the backslash, survives `eval` as a literal, and must stay green. Argument 1 (the description) is printed via `printf '%s'` and is inert — flagging it would be a false positive.
  - "Command position" means the word begins a command: start of line, or after `;`, `|`, `&&`, `||`, `(`, `{`, `then`, `do`, `else`. This keeps `# assert …` prose and `printf 'assert %s'` from arming the rule.

- [ ] **Step 1: Write the red and green fixtures first**

They are the executable specification of the rules; write them before the scanner so the scanner is developed against them.

`tests/fixtures/hygiene/red/dq_description.sh` — bare backtick in a double-quoted description:

```bash
#!/usr/bin/env bash
# RED FIXTURE: bare backtick inside a double-quoted string executes at source evaluation.
assert "the skill says `git checkout .` in its guard anchor" 'true'
```

`tests/fixtures/hygiene/red/dq_sites_block.sh` — the 0212 vector, a multi-line double-quoted assignment. This is the fixture that fails if the scanner is line-local:

```bash
#!/usr/bin/env bash
# RED FIXTURE: change 0212's actual incident shape — a MULTI-LINE double-quoted assignment.
# A per-line scanner reports nothing here; that is the regression this fixture pins.
SITES="
skills/docket-implement-next/SKILL.md
  anchor: run `git checkout .` to discard the pending claim
scripts/run-tests.sh
"
printf '%s\n' "$SITES"
```

`tests/fixtures/hygiene/red/heredoc_plain.sh` and `red/heredoc_dash.sh` — unquoted-delimiter bodies:

```bash
#!/usr/bin/env bash
# RED FIXTURE: an UNQUOTED heredoc delimiter substitutes in the body.
cat <<EOF
the anchor is `printf HAZARD` here
EOF
```

```bash
#!/usr/bin/env bash
# RED FIXTURE: <<- is the same hazard; only the leading-tab stripping differs.
	cat <<-EOF
	the anchor is `printf HAZARD` here
	EOF
```

`tests/fixtures/hygiene/red/sq_condition_unescaped.sh` — the eval re-parse vector:

```bash
#!/usr/bin/env bash
# RED FIXTURE: source quoting protects the FIRST evaluation; eval strips that protection.
assert "demo" 'printf "%s\n" "`printf EXECUTED`"'
```

`tests/fixtures/hygiene/red/dq_condition_escaped.sh` — the escaped-in-double-quotes spelling:

```bash
#!/usr/bin/env bash
# RED FIXTURE: the backslash is consumed at source evaluation, so $2 carries a BARE backtick
# into eval and it executes there.
assert "demo" "grep \`printf pattern\` file"
```

`tests/fixtures/hygiene/red/normal_substitution.sh` — the suite-wide ban on legacy substitution (Assumption 9):

```bash
#!/usr/bin/env bash
# RED FIXTURE: legacy backtick substitution in unquoted code position. Banned suite-wide; the
# repo has zero live uses and `$(…)` is the house style.
now=`date -u +%s`
printf '%s\n' "$now"
```

`tests/fixtures/hygiene/red/sentinel.sh` — the side-effect sentinel. `HYGIENE_SENTINEL_DIR` is supplied by the regression test; the fixture is never executed, so the marker must never appear:

```bash
#!/usr/bin/env bash
# RED FIXTURE + SIDE-EFFECT SENTINEL: if anything ever EXECUTES this file instead of scanning it,
# the marker file appears and tests/test_assert_hygiene.sh reddens. Detection must not require
# execution.
assert "sentinel" "touch \`printf %s "$HYGIENE_SENTINEL_DIR/EXECUTED"\`"
```

Green fixtures — `green/house_idiom.sh`:

```bash
#!/usr/bin/env bash
# GREEN FIXTURE: the suite's house idiom. eval sees the backslash and treats the backtick as a
# literal character, so this is safe and must stay legal.
assert "the block names the token" 'grep -qF "\`docket:backlink\`" "$f"'
```

`green/comments.sh`:

```bash
#!/usr/bin/env bash
# GREEN FIXTURE: backticks in comments are inert. The repo's prose is full of them — `$SKILL_PLAN`,
# `## Run halted`, `assert(){` — and flagging them would make the guard unusable.
# A comment mid-line is also fine: see below.
printf 'x\n'   # trailing prose about `git checkout .`
```

`green/quoted_heredoc.sh`:

```bash
#!/usr/bin/env bash
# GREEN FIXTURE: a QUOTED heredoc delimiter makes the body inert. All three spellings.
cat <<'EOF'
prose containing `printf INERT` and $notexpanded
EOF
cat <<"EOF"
more prose containing `printf INERT`
EOF
cat <<\EOF
still inert: `printf INERT`
EOF
```

`green/continuation.sh`:

```bash
#!/usr/bin/env bash
# GREEN FIXTURE: a backslash-newline continuation inside a double-quoted string, and a `#` that is
# NOT a comment because it does not begin a word. Neither may desynchronize the state machine.
msg="first line \
second line"
url='https://example.invalid/path#fragment'
printf '%s %s\n' "$msg" "$url"
```

`green/multiline_sq.sh`:

```bash
#!/usr/bin/env bash
# GREEN FIXTURE: a multi-line SINGLE-quoted region carrying backticks as data (an awk program,
# the shape scripts/ uses everywhere). Inert, and not an assert condition.
prog='
/^`/ { print "fenced" }
{ next }
'
printf '%s\n' "$prog"
```

- [ ] **Step 2: Verify the fixtures actually encode the hazard**

Before trusting any scanner verdict, prove the red fixtures are genuinely dangerous and the green ones genuinely safe. This is probing the probe.

```bash
cd /Users/homer/dev/docket/.worktrees/assert-executes-backticks-in-its-test-description-so-a-verba
T="$(mktemp -d "${TMPDIR:-/tmp}/hygprobe.XXXXXX")"
# The eval vector: does the single-quoted condition really execute at eval?
bash -c 'assert(){ if eval "$2"; then printf "ok - %s\n" "$1"; fi; }; assert "demo" '"'"'printf "%s\n" "`printf EXECUTED`"'"'"''
# Expected: prints EXECUTED, then "ok - demo" — the incident shape, green test, executed code.
rm -rf "$T"
```

Expected: `EXECUTED` appears in the output. If it does not, stop — the diagnosis this whole change rests on is wrong and that is a design escalation, not a scanner bug.

- [ ] **Step 3: Write the checker with rule (b) only**

`scripts/check-test-source-hygiene.sh`. Header must state: the two rules; that the preflight protects **suite runs only** and `bash tests/test_x.sh` bypasses it; and the residual — conditions assembled from variables at runtime are not modeled, because the scanner reads source, not values.

Structure: argument parsing, then one awk invocation per file (simpler to reason about than a multi-file program with `FNR`/heredoc state bleed, and the suite is ~110 files). Collect violations, print them, exit 1 if any.

The awk program must live in a **single-quoted** literal with backslash-escaped backticks, since the checker scans a tree that includes itself.

- [ ] **Step 4: Run the checker against every fixture and verify both directions**

```bash
cd /Users/homer/dev/docket/.worktrees/assert-executes-backticks-in-its-test-description-so-a-verba
for f in tests/fixtures/hygiene/red/*.sh; do
  out="$(bash scripts/check-test-source-hygiene.sh "$f" 2>&1)"; rc=$?
  printf '%s rc=%s %s\n' "$f" "$rc" "$(printf '%s' "$out" | tr '\n' ' ')"
done
for f in tests/fixtures/hygiene/green/*.sh; do
  out="$(bash scripts/check-test-source-hygiene.sh "$f" 2>&1)"; rc=$?
  printf '%s rc=%s %s\n' "$f" "$rc" "$(printf '%s' "$out" | tr '\n' ' ')"
done
```

Expected: every `red/` file `rc=1` with the class token its name implies; every `green/` file `rc=0` with no output. Any deviation is fixed in the scanner, never by editing a fixture to agree with the scanner.

- [ ] **Step 5: Write the contract file**

`scripts/check-test-source-hygiene.md`, matching the Purpose / Usage / Behavior / Exit codes / Invariants shape of its siblings (read `scripts/run-tests.md` and `scripts/board-refresh.md` for house form). It must state the standalone-run limitation and the variable-built-condition residual.

- [ ] **Step 6: Commit**

```bash
git add scripts/check-test-source-hygiene.sh scripts/check-test-source-hygiene.md tests/fixtures/hygiene
git commit -m "feat(0221): source-hygiene checker — backtick quoting scanner + mutation fixtures"
```

---

### Task 2: Rule (a) — the definition allowlist

**Files:**
- Modify: `scripts/check-test-source-hygiene.sh`
- Modify: `scripts/check-test-source-hygiene.md`
- Create: `tests/fixtures/hygiene/red/defn_spaced.sh`, `red/defn_function_kw.sh`, `red/defn_multiline.sh`, `red/defn_echo.sh`

**Interfaces:**
- Consumes: Task 1's checker and its exit contract.
- Produces: the `DEFN-DRIFT` violation class; and the checker's **own** discovery of `tests/**/*.sh` for rule (a), independent of the paths it is handed.

**Boundary that the spec did not model (reconcile addendum A1):** rule (a) must discover definitions across the whole `tests/` tree on its own, **not** only in the paths the caller passes. `tests/lib/gate_run_common.sh`, `tests/lib/runner_dispatch_detach_common.sh` and `tests/lib/sync_agents_common.sh` each define `assert` and are not `tests/test_*.sh`, so the target list `run-tests.sh` hands the preflight excludes them. Trusting the caller's list would leave three definitions permanently unguarded.

- [ ] **Step 1: Re-derive the census — do not reuse any number**

```bash
cd /Users/homer/dev/docket/.worktrees/assert-executes-backticks-in-its-test-description-so-a-verba
grep -rnE '^[[:space:]]*(function[[:space:]]+)?[A-Za-z_][A-Za-z0-9_]*[[:space:]]*(\(\))?[[:space:]]*\{' tests --include='*.sh' \
  | grep -E 'ok - |ok   - |NOT OK|FAIL - ' \
  | sed 's/^[^:]*:[0-9]*://' | sed 's/^[[:space:]]*//' | sort | uniq -c | sort -rn
```

Point-in-time expectation (2026-08-11, will drift): 84 canonical `assert(){ if eval "$2"; …}`, 3 subshell `( eval "$2" )`, 1 `fails`-counter with a `FAIL - ` marker, and 22 `ok`/`no`/`nok` wrappers across six whitespace spellings. **Sort what you actually find**; the counts above are orientation, not a target.

- [ ] **Step 2: Write the drifted-declaration red fixtures**

Today's tree is uniform in declaration shape — no `assert () {`, no `function assert`, no multiline. The guard exists for tomorrow's copy, so the alternate spellings live only here:

```bash
#!/usr/bin/env bash
# RED FIXTURE: spaced declaration. Shape-tolerant discovery must catch it; the allowlist must
# then reject it, because its body still interpolates through echo.
assert () { if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }
```

```bash
#!/usr/bin/env bash
# RED FIXTURE: `function` keyword declaration.
function assert { if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }
```

```bash
#!/usr/bin/env bash
# RED FIXTURE: multiline declaration.
assert() {
  if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi
}
```

```bash
#!/usr/bin/env bash
# RED FIXTURE: canonical one-line shape but the pre-0221 echo body — the drift the allowlist
# exists to catch after normalization lands.
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }
```

- [ ] **Step 3: Implement rule (a)**

Discovery is shape-tolerant (the four declaration shapes above, keyed on syntax not spelling); the verdict is a **byte-exact** match of the whole definition against the canonical allowlist. Normalize only insignificant leading whitespace before comparing — a byte-exact anchor is the entire reason D1 normalizes 88 definitions, so do not soften the comparison into a fuzzy match.

The allowlist is the four canonical forms Task 4 writes, plus the wrapper forms. Keep it in one clearly-marked block at the top of the checker so a reader can see the whole legal set at once.

- [ ] **Step 4: Mutation-probe rule (a), and confirm the mutation changed bytes**

Deletion and inversion are different probes; run both.

```bash
cd /Users/homer/dev/docket/.worktrees/assert-executes-backticks-in-its-test-description-so-a-verba
cp scripts/check-test-source-hygiene.sh "${TMPDIR:-/tmp}/hyg.orig"
# Deletion probe: remove the allowlist comparison; the drifted fixtures must go GREEN (guard gone).
# Inversion probe: negate it; the canonical suite must go RED.
# After each edit, PROVE the bytes changed:
diff <(cat "${TMPDIR:-/tmp}/hyg.orig") scripts/check-test-source-hygiene.sh | grep -c '^[<>]'
# Expected: a small non-zero number matching the lines you intended to touch. A 0 means the
# mutation silently no-opped and any verdict below is vacuous.
cp -f "${TMPDIR:-/tmp}/hyg.orig" scripts/check-test-source-hygiene.sh
```

- [ ] **Step 5: Re-check every probe grep under /usr/bin/grep**

PATH `grep` is ugrep and accepts constructs BSD grep rejects. Any grep inside the checker or inside a probe used above must be re-run with `/usr/bin/grep` and produce the same verdict.

- [ ] **Step 6: Commit**

```bash
git add scripts/check-test-source-hygiene.sh scripts/check-test-source-hygiene.md tests/fixtures/hygiene
git commit -m "feat(0221): source-hygiene checker rule (a) — byte-exact assert definition allowlist"
```

---

### Task 3: `tests/test_assert_hygiene.sh` and its budget row

**Files:**
- Create: `tests/test_assert_hygiene.sh`
- Modify: `tests/runtime-budgets.tsv`

**Interfaces:**
- Consumes: the checker's `file:line: CLASS:` output shape and its 0/1/2 exit contract.
- Produces: the suite-visible regression gate. Nothing later depends on it.

- [ ] **Step 1: Write the test**

Model it on `tests/test_pipe_shapes.sh` for house form. It must assert **both directions** per fixture — a red fixture flagged *with its expected class token*, a green fixture clean — because zero false positives establishes nothing about false negatives.

Two disciplines specific to this file:

1. **Assert the class, not just the exit code.** An assert on "it failed" is satisfied by every unrelated way of failing; pin the mechanism. `red/heredoc_plain.sh` must report `HEREDOC-BACKTICK`, not merely `rc=1`.
2. **The side-effect sentinel.** Point `HYGIENE_SENTINEL_DIR` at a scratch dir, run the checker over `red/sentinel.sh`, and assert **both** that the checker flagged it **and** that the marker file does not exist — proving detection happened without execution.

Use the canonical helper form this change establishes:

```bash
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }
```

Derive the fixture list from a glob of `tests/fixtures/hygiene/red` and `…/green`, and **assert the discovered count is non-zero** — a glob that matches nothing would make every downstream assert vacuous and the file would print a confident green. Also assert every red fixture was actually visited.

- [ ] **Step 2: Run it and watch it pass**

```bash
cd /Users/homer/dev/docket/.worktrees/assert-executes-backticks-in-its-test-description-so-a-verba
bash tests/test_assert_hygiene.sh; echo "rc=$?"
```

Expected: only `ok - ` lines, `rc=0`.

- [ ] **Step 3: Mutation-probe the test itself**

Weaken the checker (comment out one violation class), re-run `tests/test_assert_hygiene.sh`, and confirm it **reddens** naming that class. Restore. A guard that does not redden when the thing it guards is removed is decoration.

- [ ] **Step 4: Measure the runtime and add the budget row**

Do **not** use `--timings` against a real test file (#0290 truncates it). Measure directly:

```bash
cd /Users/homer/dev/docket/.worktrees/assert-executes-backticks-in-its-test-description-so-a-verba
time bash tests/test_assert_hygiene.sh >/dev/null
```

Add the row to `tests/runtime-budgets.tsv` in the file's existing sorted position, tab-separated, rounded up to the next multiple of 5 plus a 5s margin, minimum 10 — the rule stated in the table's own header:

```
tests/test_assert_hygiene.sh	10	parallel
```

Use the **measured** number, not the 10 shown here, if the measurement warrants more.

- [ ] **Step 5: Commit**

```bash
git add tests/test_assert_hygiene.sh tests/runtime-budgets.tsv
git commit -m "test(0221): assert-hygiene regression test + measured budget row"
```

---

### Task 4: D1 — normalize every assert-family definition

**Files:**
- Modify: every file the Task 2 Step 1 census names (point-in-time: ~88 definitions + 22 wrappers across ~100 files under `tests/`, including `tests/lib/*.sh`)

**Interfaces:**
- Consumes: the canonical forms rule (a) allowlists.
- Produces: a tree where rule (a) is green.

The canonical forms — these are the exact bytes rule (a) allowlists:

```bash
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }
```

```bash
assert(){ if ( eval "$2" ); then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }
```

```bash
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fails=$((fails+1)); fi; }
```

Wrappers:

```bash
ok(){ printf 'ok - %s\n' "$1"; }
no(){ printf 'NOT OK - %s\n' "$1"; fail=1; }
nok(){ printf 'NOT OK - %s\n' "$1"; fail=1; }
```

- [ ] **Step 1: Rewrite the definitions**

Semantics are preserved per variant, print discipline is unified:
- the three subshell files (`test_loop_continuation.sh`, `test_inline_role_stop_scoping.sh`, `test_finalize_disposition.sh` at the time of writing — **re-derive**) keep `( eval "$2" )`;
- `test_docket_review.sh`'s `fails` counter keeps its counter but its divergent `FAIL - ` marker becomes `NOT OK - `, so its failures become runner-visible for the first time;
- the `ok`/`no`/`nok` wrappers collapse their whitespace variants (`ok   - `, `printf` vs `echo`) onto the canonical bytes.

**The `ok   - ` → `ok - ` change is not cosmetic where a test greps its own output.** Before rewriting, check for self-referential greps:

```bash
grep -rnE "'ok   - |\"ok   - |FAIL - " tests scripts --include='*.sh' --include='*.md'
```

Any hit is a call site that must move with the definition.

- [ ] **Step 2: Prove the sweep landed and nothing else moved**

```bash
cd /Users/homer/dev/docket/.worktrees/assert-executes-backticks-in-its-test-description-so-a-verba
git diff --stat | tail -1
# Every changed line must be a definition line or a marker call site. Confirm the counts:
git diff -U0 | grep -c '^+' ; git diff -U0 | grep -c '^-'
```

Removed and added line counts should be equal and should match the census count plus any call sites. A mismatch means the sweep touched something it should not have.

- [ ] **Step 3: Run rule (a) over the tree**

```bash
bash scripts/check-test-source-hygiene.sh $(find tests -maxdepth 1 -name 'test_*.sh' | sort)
```

Expected: no `DEFN-DRIFT` lines.

- [ ] **Step 4: Run the whole suite**

```bash
bash scripts/run-tests.sh 2>&1 | tail -30
```

Expected: green. `test_gate_run_stop.sh` may show flake #0293 — re-run it alone to confirm, then proceed.

- [ ] **Step 5: Commit**

```bash
git add -A tests
git commit -m "refactor(0221): normalize every assert-family definition to the canonical printf form"
```

---

### Task 5: Calibration — the checker green on the real suite

**Files:**
- Modify: whatever test files the scan flags (unknown until run; the reconcile pass measured 41 bare-in-double-quote, 76 escaped-in-double-quote and 108 normal-state candidates from a deliberately crude line-local prototype — an **upper bound**, since a correct cross-line, heredoc-aware scanner reclassifies most of them)

**Interfaces:**
- Consumes: the completed checker.
- Produces: a tree where the checker exits 0 with **zero** exceptions, which Task 6 then makes mandatory.

**The gate:** every hit is either a real hazard (fix the test file) or a false positive (fix the scanner). The rule may **shrink** to what is soundly checkable, documented in the results artifact — with one floor: **a shrink may never reopen a demonstrated execution path** (the four violation classes, each with a probe or an incident behind it). If one of those cannot be soundly lexed, that is a design escalation, not a shrink. **Never** appease the guard with exception lists scattered in test files.

- [ ] **Step 1: Scan the whole suite and classify every hit**

```bash
cd /Users/homer/dev/docket/.worktrees/assert-executes-backticks-in-its-test-description-so-a-verba
bash scripts/check-test-source-hygiene.sh $(find tests -name '*.sh' | sort) > "${TMPDIR:-/tmp}/hits.txt" 2>&1
awk -F': ' '{print $2}' "${TMPDIR:-/tmp}/hits.txt" | sort | uniq -c
```

Read **every** hit. Do not batch-fix by class without reading the sites.

- [ ] **Step 2: Fix the real hazards**

The remedies, in order of preference:
- data carrying verbatim anchors → move into a **quoted-delimiter heredoc** (`<<'EOF'`), which is inert;
- a double-quoted string that needs `$` expansion **and** a literal backtick → split: build the backtick part from a single-quoted literal and concatenate;
- an assert condition → escape the backtick (`\``), the house idiom;
- legacy `` `cmd` `` substitution → rewrite to `$(cmd)`.

Never delete an anchor to silence the guard — an anchor is what makes drift mechanically visible.

- [ ] **Step 3: Fix the false positives in the scanner, then re-verify the fixtures**

Any scanner change re-runs Task 1 Step 4 in full. A narrowing that makes a red fixture pass is a regression, not a fix.

- [ ] **Step 4: Confirm zero, and confirm the scan actually visited the files**

```bash
bash scripts/check-test-source-hygiene.sh $(find tests -name '*.sh' | sort); echo "rc=$?"
# rc must be 0. Now prove the run was not vacuous — a glob that matched nothing also exits 0:
find tests -name '*.sh' | wc -l
```

- [ ] **Step 5: Full suite, then commit**

```bash
bash scripts/run-tests.sh 2>&1 | tail -30
git add -A tests scripts
git commit -m "fix(0221): calibrate the hygiene checker to zero exceptions on the live suite"
```

---

### Task 6: Wire the preflight into `scripts/run-tests.sh`

**Files:**
- Modify: `scripts/run-tests.sh` (insert after the missing/collide validation block ending `{ [ "$missing" = 0 ] && [ "$collide" = 0 ]; } || exit 2`, before the budget table is read)
- Modify: `scripts/run-tests.md`

**Interfaces:**
- Consumes: the checker's exit contract.
- Produces: exit code **5** — "test-source hygiene violation; zero test files executed".

**Why 5:** the runner already spends 0, 1, 2, 3 and 4 (4 is the `--strict-budget` breach). Verify this is still true before writing it.

- [ ] **Step 1: Insert the gate**

Synchronous, before the first launch. It must be impossible for a target file's source to be evaluated first — that is the entire point, since detection after execution is not prevention.

```bash
# ---- source-hygiene preflight (change 0221) ----------------------------------------------------
# Synchronous, BEFORE the first launch: a hazardous backtick in a test file executes when the shell
# reaches its line, so a peer test file's source evaluation is exactly what a post-hoc lint cannot
# undo. A violation aborts the run with ZERO test files executed.
HYGIENE="$REPO/scripts/check-test-source-hygiene.sh"
if [ -x "$HYGIENE" ] || [ -f "$HYGIENE" ]; then
  if ! hyg_out="$(bash "$HYGIENE" "${TARGETS[@]}" 2>&1)"; then
    printf 'run-tests: test-source hygiene violation — aborting with zero test files executed\n' >&2
    printf '%s\n' "$hyg_out" >&2
    exit 5
  fi
fi
```

Note the capture-then-print shape: no `producer | early-exiting-consumer`, per the repo-wide pipe-shape guard.

- [ ] **Step 2: Prove the abort executes zero test files — the acceptance criterion**

This is the sentinel test of the wiring, and it is the claim the whole design rests on.

```bash
cd /Users/homer/dev/docket/.worktrees/assert-executes-backticks-in-its-test-description-so-a-verba
S="$(mktemp -d "${TMPDIR:-/tmp}/hygseed.XXXXXX")"
cat > tests/test_zz_hygiene_seed.sh <<SEEDEOF
#!/usr/bin/env bash
# TEMPORARY seed file — deleted at the end of this step.
marker="\$(printf '%s' "$S/RAN")"
assert "seeded" "printf 'x' > \\\`printf %s "\$marker"\\\`"
SEEDEOF
bash scripts/run-tests.sh 2>&1 | tail -5; echo "rc=${PIPESTATUS[0]}"
ls "$S"
rm -f tests/test_zz_hygiene_seed.sh; rm -rf "$S"
```

Expected: the runner exits **5**, prints the violation, and `ls "$S"` is **empty** — the seed's backtick never ran. If the marker exists, the gate is downstream of execution and the wiring is wrong.

- [ ] **Step 3: Prove the gate is not vacuous when clean**

Remove the seed (done above) and confirm a normal run still reaches the tests and exits on its own merits — a preflight that silently always passes is the fault-injection-wrapper failure mode this repo has hit repeatedly.

```bash
bash scripts/run-tests.sh tests/test_assert_hygiene.sh 2>&1 | tail -5
```

- [ ] **Step 4: Document it**

`scripts/run-tests.md`: add the exit-code table row for 5, and a short section on the preflight — what it checks, that it runs before the first launch, and the **standalone-run limitation** (`bash tests/test_x.sh` bypasses it; accepted as residual rather than paid for with a preamble in 100+ files).

- [ ] **Step 5: Full suite, then commit**

```bash
bash scripts/run-tests.sh 2>&1 | tail -30
git add scripts/run-tests.sh scripts/run-tests.md
git commit -m "feat(0221): gate run-tests on the source-hygiene preflight (exit 5, zero files executed)"
```

---

### Task 7: D3 — write the rule into `tests/README.md`

**Files:**
- Modify: `tests/README.md`

**Interfaces:**
- Consumes: the landed gate.
- Produces: the prose contract a future test author reads.

- [ ] **Step 1: Add the section**

After "Where new tests go", add a short section covering exactly three things:

1. **The rule.** Verbatim clauses and guard anchors are *data*: carry them in single-quoted literals or quoted-delimiter heredocs (`<<'EOF'`); escape backticks inside assert conditions (`'grep -qF "\`span\`" "$f"'`); never put any backtick inside double quotes, bare or escaped.
2. **The enforcement.** `scripts/run-tests.sh` runs `scripts/check-test-source-hygiene.sh` over every target before the first launch; a violation aborts with exit 5 having executed zero test files.
3. **The limitation.** The preflight protects suite runs only — `bash tests/test_x.sh` run directly bypasses it.

Explain *why* in one clause — a backtick in test source executes when the shell reaches that line, before `assert` is ever called — so the rule is not read as style.

- [ ] **Step 2: Commit**

```bash
git add tests/README.md
git commit -m "docs(0221): write the test-source quoting rule and its enforcement into tests/README.md"
```

---

### Task 8: D4 — correct the learning's mechanism claim

**⚠ BOUNDARY — this task does NOT land on the feature branch.** `docs/changes/learnings/` exists only on the `docket` metadata branch and is never published to `main`. Edit it in the metadata worktree at `/Users/homer/dev/docket/.docket/`, commit and push on `docket`, staging **by explicit path** (the tree is shared with concurrent agents; `git add -A` there commits their staged work under your message).

**Files:**
- Modify: `/Users/homer/dev/docket/.docket/docs/changes/learnings/test-helper-interpolates-its-own-description.md`

- [ ] **Step 1: Rewrite the mechanism claim**

The file still carries the disproven diagnosis — helper interpolation as the executing vector — as its hook and its Apply guidance. Known-wrong guidance must not stay discoverable behind a done change. Replace the mechanism with the corrected one (source-evaluation substitution at call sites, the `eval` re-parse, unquoted-delimiter heredocs). **Keep** the war story, the blast-radius framing, and the `printf '%s'` hygiene advice — that advice is correct, it just is not the fix.

- [ ] **Step 2: Re-render the index and push**

```bash
cd /Users/homer/dev/docket
"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh render-learnings-index --changes-dir .docket/docs/changes
cd /Users/homer/dev/docket/.docket
git add docs/changes/learnings/test-helper-interpolates-its-own-description.md docs/changes/learnings/README.md
git commit -m "docs(0221): correct the mechanism claim in test-helper-interpolates-its-own-description"
git push origin docket
```

---

## Self-Review

**Spec coverage.** D1 → Task 4. D2 checker → Tasks 1–2; D2 runner wiring → Task 6; D2 regression test + budget row → Task 3; D2 calibration gate → Task 5; D2 mutation fixtures → Tasks 1–2. D3 → Task 7. D4 → Task 8. Reconcile addenda: A1 → Task 2's boundary note; A2 → Task 1 Step 5; A3 → Task 6; A4 → Task 1's state-machine notes and `red/dq_sites_block.sh`; A5 → Task 2 Step 1. Every acceptance bullet maps to a step that produces evidence.

**Placeholder scan.** No TBDs. Every fixture is written out in full. The one deliberately unenumerated set is Task 5's fix list, which cannot be known before the scan runs — the task supplies the classification procedure, the remedy ordering, and the shrink floor instead of a fabricated file list.

**Type consistency.** The five violation class tokens (`NORMAL-BACKTICK`, `DQ-BACKTICK`, `HEREDOC-BACKTICK`, `EVAL-BACKTICK`, `DEFN-DRIFT`) are introduced in Task 1's Interfaces and used unchanged in Tasks 2, 3, 5 and 6. The canonical assert forms in Task 4 are the same bytes rule (a) allowlists in Task 2. The exit contract (0/1/2 from the checker; 5 from the runner) is consistent across Tasks 1, 3 and 6.
