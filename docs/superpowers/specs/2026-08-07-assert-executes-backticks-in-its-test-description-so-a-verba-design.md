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

The vectors that DO execute a backtick are all **parse-time**:

1. A bare backtick inside a **double-quoted literal in test source** — a description written
   `assert "… `git checkout .` …" …`, or data assigned `SITES="… `cmd` …"` (multi-line
   double-quoted assignments are exactly where 0212's anchors live). The shell substitutes while
   parsing the test file, before `assert` even runs.
2. A bare backtick reaching `eval "$2"` **as a literal character in the condition string** — e.g. a
   condition built with double quotes at the call site so data is spliced into code, or an
   *unescaped* backtick typed inside a single-quoted condition. `eval` is a second parse; anything
   literal in the string is code. (The suite's house idiom — `'grep -qF "\`span\`" "$f"'`,
   backslash-escaped backtick inside a single-quoted condition — is safe and stays.)
3. A bare backtick in the body of an **unquoted-delimiter heredoc** (`<<EOF`), which the shell
   also substitutes.

So the fix must land at call sites and data blocks, and be *enforced* there; helper hardening is
kept, but as drift control and ledger alignment, not as the safety mechanism.

## Design

### D1 — normalize the `assert` helper everywhere (mechanical)

Derive the site list by whole-repo grep of `assert(){` under `tests/` (74 files at grooming time:
69 standard, 3 subshell `( eval "$2" )`, 1 `ok`/`nok` wrapper, 1 `fails` counter — never hand-list;
the count moves with the suite). Rewrite each definition to print its description without
interpolation:

    assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

The `ok - ` / `NOT OK - ` prefixes are a **runner contract**, not taste: `scripts/run-tests.sh`
greps `^NOT OK` at line start for its failure accounting, so the canonical form must keep the marker
at column 1. Semantic variants keep their semantics with the same print discipline: the three
subshell files keep `( eval "$2" )`; the `fails`-counter file keeps its counter but normalizes its
divergent `FAIL - ` marker to `NOT OK - ` so its failures are runner-visible; the `ok`/`nok` file's
wrappers print via `printf '%s\n'` with the same markers. `eval "$2"` is retained everywhere — the condition is code by contract; rewriting
call sites to a non-eval `"$@"` form is out of scope (stub's explicit boundary, and it would break
compound conditions across hundreds of call sites).

### D2 — enforcement: a backtick-hygiene guard test

New file `tests/test_assert_hygiene.sh` (+ its `tests/runtime-budgets.tsv` row), guarding two
properties across every `tests/test_*.sh` (self-including):

- **(a) Definition allowlist.** Every `assert(){` line in the suite must byte-match one of the
  canonical hardened forms from D1. A copy-pasted drifted helper reddens here.
- **(b) Quoting rule for backticks.** Every backtick in executable context must be either inside a
  single-quoted region or backslash-escaped. Implemented as a small awk character-state scanner per
  file: states normal / single-quote / double-quote / comment, backslash handling, plus heredoc
  awareness — on `<<'X'` skip the body to the delimiter (quoted heredocs are inert); on `<<X`
  keep checking the body (unquoted heredocs substitute). A bare backtick in normal state, in a
  double-quoted region, or in an unquoted heredoc body is a failure naming file:line. Note the
  normal-state clause is deliberately broader than the 0212 hazard: it also outlaws intentional
  `` `cmd` `` command substitution suite-wide (Assumption 9). Because the guard scans itself, its
  own scanner code must carry every backtick single-quoted or escaped — the implementation keeps
  its patterns in single-quoted awk programs.

**Calibration gate:** the scanner is run against the current suite during the build. Every hit is
either a real hazard (fix the test file) or a false positive; the guard may only land at zero false
positives. If a false-positive class can't be cleanly lexed (shell quoting is genuinely hairy), the
rule *shrinks* to what is soundly checkable and the shrink is documented in the results artifact —
the guard must never be appeased with exception lists scattered in test files.

Known residual (documented in the guard's header, accepted): an *escaped* backtick inside a
double-quoted condition string (`assert "d" "grep \`cmd\`"`) passes rule (b) but executes at eval —
the escape survives to `$2` as a literal backtick. The house style (single-quoted conditions) makes
this shape unidiomatic; the scanner does not model per-argument eval destiny.

### D3 — write the rule down

A short paragraph in `tests/README.md`: verbatim clauses and anchors are data; carry them in
single-quoted literals or quoted-delimiter heredocs, escape backticks inside conditions, never put a
bare backtick in double quotes. Point at the guard as the enforcement.

### Non-goals

- No shared sourced `tests/lib/assert.sh`: files are hermetic and standalone-runnable by contract;
  the one existing lib (`sync_agents_common.sh`) is topical, not structural. Per-file edit wins.
- No test framework; no changes to what any guard asserts.
- No edit to the learning file or 0212's in-file comment (auto-groom must not touch either; see
  Assumptions for the recommended human follow-ups).

## Assumptions

(9 entries; 9 was added in the critic-gated revision round.)

1. **Design follows the verified mechanism, not the stub's/learning's stated one.** Chosen: correct
   the diagnosis (parse-time substitution at call sites / eval re-parse / unquoted heredocs) and aim
   the enforcement there. Rejected: building only the stub's literal ask (printf the description),
   because the probe shows it alone changes nothing — the 0212 incident would recur verbatim under
   it. The learning file stays untouched (not this groom's to edit); a human should later revise its
   mechanism claim.
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
6. **`eval "$2"` contract kept.** Per the stub's own boundary; the residual eval-splice shape is
   documented, not mechanically prevented.
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

## Acceptance

- All `assert(){` definitions under `tests/` match a canonical hardened form (guard (a) green).
- The hygiene scanner is green on the whole suite with zero exceptions, and seeding a bare backtick
  into a double-quoted description, a `SITES="…"` block, or an unquoted heredoc makes it red.
- A reproduction of the 0212 shape (backticked verbatim anchor in a double-quoted context) is caught
  by the guard rather than executed by a run.
- `scripts/run-tests.sh` fully green, including the new budget row.
