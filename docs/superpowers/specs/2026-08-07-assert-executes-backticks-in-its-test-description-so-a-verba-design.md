<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0221 — assert() executes backticks in its test description, so a verbatim-quoted anchor can run shell](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0221-assert-executes-backticks-in-its-test-description-so-a-verba.md)**
<!-- docket:backlink:end -->

# 0221 — backtick execution in the test suite's `assert` idiom: corrected diagnosis, hardening, and an enforced quoting rule

**Change:** 0221 · **Date:** 2026-08-07 · **Type:** fix

## Problem

During 0212's build, running `tests/test_inline_role_stop_scoping.sh` executed a backticked
`git checkout .` that appeared inside a verbatim-quoted guard anchor, reverting the worker's own
uncommitted edits — silently, with the test still printing `ok`. The stub (and the candidate
learning `test-helper-interpolates-its-own-description`) attribute this to the `assert` helper
interpolating its description into a double-quoted string (`echo "ok - $1"`), and propose hardening
the helper with `printf '%s'`.

## Corrected diagnosis (empirically verified)

A probe run during grooming (bash, four cases) shows **parameter expansion does not re-trigger
command substitution**: a backtick held in a variable's *value* is inert through `echo "ok - $1"`,
`printf "… $1 …"`, and expansion inside an `eval`'d single-quoted condition. The helper's
description print is provably not the executing vector, and swapping `echo "ok - $1"` for
`printf 'ok - %s\n' "$1"` is, by itself, a no-op against the hazard.

The vectors that DO execute a backtick all fire at **source evaluation** — bash parses a command,
then performs command substitution during expansion as that line executes, so a backtick in test
source runs when the shell reaches its line, before (or without) `assert` ever being called:

1. A bare backtick inside a **double-quoted literal in test source** — a description written
   `assert "… `git checkout .` …" …`, or data assigned `SITES="… `cmd` …"` (multi-line
   double-quoted assignments are exactly where 0212's anchors live). The shell substitutes as it
   evaluates that line of the test file, before `assert` even runs.
2. A bare backtick reaching `eval "$2"` **as a literal character in the condition string** — `eval`
   is a second parse; anything literal in the string is code. This has two spellings with different
   source-level looks but the same eval fate: an *unescaped* backtick typed inside a single-quoted
   condition (source quoting protects the first evaluation; eval strips that protection — probe:
   `assert demo 'printf "%s\n" "`printf EXECUTED`"'` prints `EXECUTED` then `ok`), and an *escaped*
   backtick inside a double-quoted condition (`assert "d" "grep \`cmd\`"` — the escape is consumed
   at source evaluation, so `$2` carries a bare backtick into eval). (The suite's house idiom —
   `'grep -qF "\`span\`" "$f"'`, backslash-escaped backtick inside a single-quoted condition — is
   safe and stays: eval sees the backslash and treats the backtick as literal.)
3. A bare backtick in the body of an **unquoted-delimiter heredoc** (`<<EOF`), which the shell
   also substitutes.

So the fix must land at call sites and data blocks, and be *enforced* there; helper hardening is
kept, but as drift control and ledger alignment, not as the safety mechanism.

## Design

### D1 — normalize the `assert` helper everywhere (mechanical)

Derive the site list by whole-repo grep under `tests/`, keyed on **shape, not one spelling**: the
census pattern must catch `assert(){`, `assert () {`, `function assert`, and multiline declarations
(house rule — the spelling you miss is the next file's idiom), plus the wrapper variants
(`ok`/`nok`/`no`, `fails` counter). Counts are point-in-time only — 74 files at grooming, 85
definitions across 105 test files at review (2026-08-11), the drift itself proving the point — so
the build MUST re-run the census at reconciliation and sort what it finds, never reuse these
numbers. Rewrite each definition to print its description without interpolation:

    assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

The `ok - ` / `NOT OK - ` prefixes are a **runner contract**, not taste: `scripts/run-tests.sh`
greps `^NOT OK` at line start for its failure accounting, so the canonical form must keep the marker
at column 1. Semantic variants keep their semantics with the same print discipline: the three
subshell files keep `( eval "$2" )`; the `fails`-counter file keeps its counter but normalizes its
divergent `FAIL - ` marker to `NOT OK - ` so its failures are runner-visible; the `ok`/`nok` file's
wrappers print via `printf '%s\n'` with the same markers. `eval "$2"` is retained everywhere — the condition is code by contract; rewriting
call sites to a non-eval `"$@"` form is out of scope (stub's explicit boundary, and it would break
compound conditions across hundreds of call sites).

### D2 — enforcement: a synchronous pre-execution hygiene gate

The scanner cannot live only in an ordinary test file: `scripts/run-tests.sh` launches test files
in parallel with no lint pass, and its isolation explicitly does not protect the repository from
test writes (per its own header). A peer test file's backticks execute at its source evaluation —
before, or concurrently with, any hygiene test in the same run. Detection after execution is not
prevention. So:

- **The scanner is a standalone checker**, `scripts/check-test-source-hygiene.sh`, taking test-file
  paths and exiting non-zero on any violation, naming file:line.
- **`scripts/run-tests.sh` calls it as a preflight gate** over every target file, synchronously,
  before the first launch. A hit aborts the run before any test file is executed (a distinct exit
  code and a loud report line, consistent with the runner's existing exit-code contract).
- **`tests/test_assert_hygiene.sh`** (+ its `tests/runtime-budgets.tsv` row) remains, as the
  *regression test of the checker*: it runs the checker against committed green and red fixtures
  (see the fixture list below) and asserts each verdict — so a weakened checker reddens the suite.

**Documented limitation (stated in both the checker header and `tests/README.md`):** the preflight
protects suite runs only. `bash tests/test_x.sh` run directly bypasses it; that path is accepted as
residual rather than paid for with a per-file preamble in 100+ files.

The checker guards two properties across every `tests/test_*.sh`:

- **(a) Definition allowlist.** Every assert-family definition in the suite must byte-match one of
  the canonical hardened forms from D1. Discovery is shape-tolerant (same census shape as D1 —
  `assert(){`, `assert () {`, `function assert`, multiline), so a drifted *spelling* cannot dodge
  the allowlist by dodging the census.
- **(b) Quoting rule for backticks.** Implemented as a small awk character-state scanner per file:
  states normal / single-quote / double-quote / comment, backslash handling, plus heredoc
  awareness — on `<<'X'` skip the body to the delimiter (quoted heredocs are inert); on `<<X` and
  `<<-X` keep checking the body (unquoted delimiters substitute). Violations, each naming file:line:
  - a bare backtick in **normal state** (also outlaws intentional `` `cmd` `` substitution
    suite-wide; Assumption 9);
  - **any** backtick in a **double-quoted region**, bare or escaped — bare executes at source
    evaluation; escaped survives into `$2` and executes at eval, so the previously "accepted
    residual" is rejected outright rather than waved through as unidiomatic;
  - a bare backtick in an **unquoted-delimiter heredoc body**;
  - an **unescaped backtick inside a single-quoted condition argument** of an assert-family call —
    the eval-specific rule. Single-quoted regions are safe as ordinary data (descriptions, fixture
    prose) but not as eval'd conditions, so the scanner is call-aware for exactly this case: on a
    line whose command word is an assert-family helper, the condition argument's single-quoted
    content must carry backticks only backslash-escaped (the house idiom).

  Because the checker scans the suite including the hygiene test's fixtures-of-green, its own
  patterns live in single-quoted awk programs with escaped backticks.

**Calibration gate:** the checker runs against the current suite during the build. Every hit is
either a real hazard (fix the test file) or a false positive; the gate may only land at zero false
positives. If a false-positive class can't be cleanly lexed (shell quoting is genuinely hairy), the
rule *shrinks* to what is soundly checkable and the shrink is documented in the results artifact —
with one floor: a shrink may never reopen a **demonstrated execution path** (the four violation
classes above, each of which has a probe or incident behind it). If one of those can't be soundly
lexed, that is a design escalation, not a shrink. The guard must never be appeased with exception
lists scattered in test files.

**Mutation fixtures (required, committed, exercised by `test_assert_hygiene.sh`):** zero false
positives establishes nothing about false negatives, so the checker's verdicts are proven both
ways. Red fixtures (checker must flag): bare backtick in a double-quoted description; in a
double-quoted `SITES="…"` block; in an `<<EOF` body and an `<<-EOF` body; unescaped backtick in a
single-quoted assert condition; escaped backtick in a double-quoted condition; a drifted helper in
each alternate declaration spelling (`assert () {`, `function assert`, multiline). Green fixtures
(checker must pass): escaped backtick in a single-quoted condition (house idiom); backticks in
comments; in `<<'EOF'` bodies; a backslash-newline continuation and a comment-boundary case
adjacent to a backtick. Plus a **side-effect sentinel**: a red fixture whose backtick would write a
marker file — the regression test asserts the checker flags it AND the marker does not exist,
proving detection happened without execution. Fixtures live outside the runner's `tests/test_*.sh`
glob (e.g. `tests/fixtures/hygiene/`) so no red fixture is ever launched as a test — they are data
handed to the checker, never sourced.

### D3 — write the rule down

A short paragraph in `tests/README.md`: verbatim clauses and anchors are data; carry them in
single-quoted literals or quoted-delimiter heredocs, escape backticks inside conditions, never put
any backtick in double quotes. Point at the preflight gate as the enforcement, and state the
standalone-run limitation (D2).

### D4 — correct the candidate learning's mechanism claim

`docs/changes/learnings/test-helper-interpolates-its-own-description.md` still carries the
disproven diagnosis (helper interpolation as the executing vector) as its hook and Apply guidance.
Known-wrong guidance must not stay discoverable behind a done change: this change rewrites the
learning's mechanism claim to the corrected diagnosis (source-evaluation substitution at call sites
/ eval re-parse / unquoted heredocs), keeping the war story, the blast-radius framing, and the
`printf '%s'` hygiene advice (correct, just not the fix). Markdown-only, on the `docket` branch,
alongside this change's other metadata edits. (Grooming excluded this only because auto-groom may
not touch learnings; human review lifts that constraint.)

### Non-goals

- No shared sourced `tests/lib/assert.sh`: files are hermetic and standalone-runnable by contract;
  the one existing lib (`sync_agents_common.sh`) is topical, not structural. Per-file edit wins.
- No test framework; no changes to what any guard asserts.
- No per-file standalone-run preamble (limitation documented instead; see D2).
- No edit to 0212's in-file comment (see Assumption 7 for the recommended human follow-up). The
  learning file is now IN scope — see D4.

## Assumptions

(11 entries; 9 was added in the critic-gated revision round; 10–11 and the revisions to 1 and 6 in
the 2026-08-11 human review round.)

1. **Design follows the verified mechanism, not the stub's/learning's stated one.** Chosen: correct
   the diagnosis (source-evaluation substitution at call sites / eval re-parse / unquoted heredocs)
   and aim the enforcement there. Rejected: building only the stub's literal ask (printf the
   description), because the probe shows it alone changes nothing — the 0212 incident would recur
   verbatim under it. *(Revised 2026-08-11:)* the learning file's correction is now in scope (D4) —
   leaving disproven guidance discoverable was the weaker call, and the auto-groom constraint that
   forced it does not bind a human-reviewed spec.
2. **Helper is still normalized to `printf '%s'` despite being safety-neutral.** Chosen for ledger
   alignment, echo-flag/escape hygiene, and to give guard (a) a canonical byte-exact anchor.
   Rejected: leaving 74 slightly-divergent definitions (guard would need fuzzy matching), and
   rejected: guarding against the interpolation shape as if it were the bug (cargo cult).
3. **Per-file mechanical edit, no shared library.** Rejected the sourced-helper option the stub
   floated: it trades a one-time mechanical diff for a permanent hermeticity break in 70+ files.
4. **Enforcement is a new guard file, not an extension of an existing shard.** Suite-wide test
   hygiene is a new surface (README rule 3); no topical shard owns "all test files". Requires the
   budget row. Rejected: bolting it onto `test_run_tests.sh` (runner ≠ hygiene).
5. **Scanner scope: sound-but-shrinkable, zero false positives required.** Rejected a full shell
   lexer (tar pit) and rejected a blunt "no backticks anywhere" ban (would outlaw the safe house
   idiom and heredoc fixture prose the repo depends on).
6. **`eval "$2"` contract kept — but eval-reachable backticks are now scanned, not accepted.**
   Per the stub's own boundary, call sites stay on `eval "$2"`. *(Revised 2026-08-11:)* the earlier
   "documented residual" stance was too weak for a data-loss class: a probe showed an unescaped
   backtick in a single-quoted condition executes at eval and prints `ok`, and the escaped
   double-quoted form does the same. Both are now violation classes in the checker (D2's call-aware
   rule and the total double-quoted ban). What remains unmodeled is arbitrary code-building
   (conditions assembled from variables) — that residual is documented in the checker header.
7. **0212's "no backticks in SITES anchors" comment stays as-is.** Once the guard lands, a follow-up
   *may* relax it by moving SITES to a quoted-delimiter heredoc so backticked anchors become legal
   data — recommended as human follow-up prose, not minted here (auto-groom mints nothing).
8. **Dependency state:** none. `discovered_from: 212` is merged context only; nothing here blocks on
   open work.
9. **Normal-state bare backticks are banned suite-wide, beyond the strict 0212 hazard.** Chosen: the
   scanner also flags bare backticks in unquoted code position, i.e. legacy `` `cmd` ``
   substitution. The suite has zero live uses today (verified — all current backticks are comments,
   escaped-in-condition, or single-quoted), so the ban costs nothing, closes the same
   silent-execution class, and matches the `$()` house style. Rejected: carving out an allowance for
   intentional backtick substitution, which would force the scanner to distinguish intent it cannot
   see.
10. **Prevention is a runner preflight, not a peer test.** Chosen: `check-test-source-hygiene.sh`
    invoked synchronously by `run-tests.sh` before the first launch, with `test_assert_hygiene.sh`
    demoted to the checker's regression test. Rejected: the original ordinary-test wiring — the
    runner launches files in parallel and lints nothing, so a hazardous file's backticks execute
    before or alongside the hygiene test, and the original acceptance claim ("caught rather than
    executed") was unsatisfiable. Rejected: a per-file self-check preamble to also cover direct
    `bash tests/test_x.sh` runs — 100+-file churn for a path the suite contract doesn't use;
    documented as a limitation instead.
11. **Helper discovery is shape-tolerant and freshly derived.** The census keys on syntactic shape
    (house rule) — `assert(){` alone would miss `assert () {` / `function assert` / multiline
    forms; today's tree is uniform (85 × `assert(){`, verified 2026-08-11) but the guard exists
    precisely for tomorrow's drifted copy. Counts in this spec are point-in-time; the build re-runs
    discovery at reconciliation (D1). Mutation fixtures cover the alternate spellings (D2).

## Acceptance

- All assert-family definitions under `tests/` (shape-tolerant census, freshly derived) match a
  canonical hardened form (checker rule (a) green).
- `scripts/run-tests.sh` runs `scripts/check-test-source-hygiene.sh` over every target before the
  first launch; on a violation it aborts with its distinct exit code having executed zero test
  files. Proven by the side-effect sentinel: seeding a hazard file into a run leaves the sentinel's
  marker unwritten.
- The checker is green on the whole current suite with zero exceptions, and red on every committed
  red fixture: bare backtick in a double-quoted description / `SITES="…"` block / `<<EOF` and
  `<<-EOF` bodies; unescaped backtick in a single-quoted condition; escaped backtick in a
  double-quoted condition; each alternate helper-declaration spelling. Green fixtures (house-idiom
  escaped condition, comments, `<<'EOF'`, continuation/comment-boundary cases) all pass.
- `tests/test_assert_hygiene.sh` reddens if the checker weakens (fixture verdicts asserted both
  directions), and carries its `runtime-budgets.tsv` row.
- The learning `test-helper-interpolates-its-own-description` no longer states helper interpolation
  as the executing mechanism (D4).
- `tests/README.md` states the quoting rule, the enforcement point, and the standalone-run
  limitation.
- `scripts/run-tests.sh` fully green, including the new budget row.
