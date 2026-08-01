<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0175 — sync-agents.sh costs ~5.5s per invocation and dominates the test suite](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0175-sync-agents-per-invocation-cost.md)**
<!-- docket:backlink:end -->

# sync-agents.sh Per-Invocation Cost Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce a real `sync-agents.sh` generation pass from thousands of repeated parser subprocesses to a small, bounded set while preserving all config-resolution behavior.

**Architecture:** Keep the existing precedence loop and YAML subset intact. Prime a file/harness body cache synchronously before command-substituted reads, then scan cached bodies and flow-map fields with Bash builtins; validate arguments before any generation side effect.

**Tech Stack:** GNU Bash 4+, existing shell test harness, `time`, and PATH-injected counting shims for the retained performance oracle.

## Global Constraints

- Preserve the in-repo shell YAML reader boundary from ADR-0062; add no YAML dependency.
- Preserve change 0173's `[^,}[:space:]]+` consumed-value class, `field_of_raw`, and ADR-0065's two-leg bare-scalar validation.
- Preserve layer order, harness-over-default precedence, and every `RES_*_FROM_*` provenance flag.
- Prime cache entries on a synchronous caller path; a cache filled inside command-substituted `harness_agent_line` is lost with its subshell.
- Existing assertions in `tests/test_sync_agents.sh`, `tests/test_sync_agents_codex.sh`, and `tests/test_sync_agents_cursor.sh` must remain unchanged; add new assertions without weakening old ones.
- Preserve the existing tab-indented layer assertions and use `[[:space:]]` indentation classes.
- Measure performance on the same machine before and after; unchanged or slower wall clock is a red result even if correctness is green.
- Run the whole repository suite at the build gate.

---

### Task 1: Make the command-line contract side-effect-free

**Files:**
- Modify: `sync-agents.sh`
- Test: `tests/test_sync_agents.sh`

**Interfaces:**
- Consumes: existing executable guard `[ "${BASH_SOURCE[0]}" = "${0}" ]` and `CHECK` flag.
- Produces: `usage()`, `--help` exit 0 without writes, `--check` as the only operational flag, and unknown-argument exit 2.

- [ ] **Step 1: Add failing argument-contract tests**

Add a new block near the generator's basic existence checks. Use `make_sandbox`, set a separate harness root, and assert exact side effects:

```bash
make_sandbox
HROOT175A="$(mktemp -d)"; mkdir -p "$HROOT175A/.claude"
help_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT175A" bash "$SYNC" --help 2>&1)"; help_rc=$?
assert "0175 args: --help succeeds" '[ "$help_rc" = "0" ]'
assert "0175 args: --help prints usage" '/usr/bin/grep -qF "Usage: bash sync-agents.sh [--check]" <<<"$help_out"'
assert "0175 args: --help writes no wrapper" '[ ! -e "$SBX/.claude/agents/docket-status.md" ]'
bad_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT175A" bash "$SYNC" --bogus 2>&1)"; bad_rc=$?
assert "0175 args: unknown flag fails with rc=2" '[ "$bad_rc" = "2" ]'
assert "0175 args: unknown flag names the argument" '/usr/bin/grep -qF "unknown argument: --bogus" <<<"$bad_out"'
assert "0175 args: unknown flag writes no wrapper" '[ ! -e "$SBX/.claude/agents/docket-status.md" ]'
rm -rf "$SBX" "$HROOT175A"
```

- [ ] **Step 2: Run the focused test to prove RED for the intended reason**

Run: `bash tests/test_sync_agents.sh`

Expected: the new `--help prints usage`, `--help writes no wrapper`, and unknown-argument assertions fail against the current fall-through behavior. Record the pre-change assert total so later runs can prove the file did not terminate early.

- [ ] **Step 3: Implement guarded argument parsing before all generation work**

Replace the loose `CHECK` assignment with an executable-only parser so sourcing the file for helper tests never consumes the caller's positional parameters:

```bash
usage() {
  printf '%s\n' 'Usage: bash sync-agents.sh [--check]'
}

CHECK=0
if [ "${BASH_SOURCE[0]}" = "${0}" ]; then
  case "$#:${1:-}" in
    0:) ;;
    1:--check) CHECK=1 ;;
    1:--help) usage; exit 0 ;;
    *)
      printf 'sync-agents: unknown argument: %s\n' "${1:-<empty>}" >&2
      usage >&2
      exit 2
      ;;
  esac
fi
```

- [ ] **Step 4: Re-run focused tests**

Run: `bash tests/test_sync_agents.sh`

Expected: PASS, no `NOT OK`, and the assert total is the RED-run total (not lower).

- [ ] **Step 5: Commit the independently useful CLI contract**

```bash
git add sync-agents.sh tests/test_sync_agents.sh
git commit -m "fix(sync-agents): validate command arguments"
```

### Task 2: Replace repeated layer parsing with an effective cache

**Files:**
- Modify: `sync-agents.sh`
- Test: `tests/test_sync_agents.sh`

**Interfaces:**
- Consumes: `section_body KEY` unchanged, existing `harness_agent_line FILE HARNESS AGENT UNDER_AGENTS`, and `resolve_agent_layers` provenance globals.
- Produces: file-scope associative `_LAYER_BODY_CACHE`, synchronous `prime_layer_body FILE HARNESS UNDER_AGENTS`, fork-free cached `harness_agent_line`, and fork-free `field_of`/`field_of_raw`.

- [ ] **Step 1: Add direct reader-equivalence assertions before changing helpers**

In a subshell that sources `sync-agents.sh`, compare the fixed baseline outputs for both reader tiers. Cover inline and block-shaped lines, punctuation, exact field boundaries, missing fields, and repeated fields:

```bash
reader_out="$({
  . "$SYNC"
  printf '%s\t%s\n' inline "$(field_of 'x: {model: a.b_c-d, effort: high}' model)"
  printf '%s\t%s\n' block "$(field_of '  model: slash/vendor:id' model)"
  printf '%s\t%s\n' prefix "$(field_of 'x: {model_alias: wrong, model: right}' model)"
  printf '%s\t%s\n' repeated "$(field_of 'x: {model: first, model: last}' model)"
  printf '%s\t%s\n' missing "$(field_of 'x: {effort: high}' model)"
  printf '%s\t%s\n' raw "$(field_of_raw 'x: {model: two words   , effort: high}' model)"
} )"
assert "0175 readers: consumed/raw edge cases preserve fixed semantics" \
  '[ "$reader_out" = "$(printf "inline\\ta.b_c-d\\nblock\\tslash/vendor:id\\nprefix\\tright\\nrepeated\\tlast\\nmissing\\t\\nraw\\ttwo words")" ]'
```

Run: `bash tests/test_sync_agents.sh`

Expected: PASS before implementation. This is a characterization test; RED comes from the performance oracle in Task 3, not changed behavior.

- [ ] **Step 2: Add the file-scope cache and synchronous priming function**

Declare the cache beside the config helpers. Key on file, harness, and `under_agents`; cache absent files as empty. Use `${array[$key]+_}` rather than a newer Bash minor-version-only presence test. Prime with the unchanged `section_body` implementation:

```bash
declare -A _LAYER_BODY_CACHE=()

prime_layer_body() {  # $1=file $2=harness $3=under_agents(0|1)
  local file="$1" harness="$2" under_agents="$3" key sub body
  key="${file}"$'\x1f'"${harness}"$'\x1f'"${under_agents}"
  [ -z "${_LAYER_BODY_CACHE[$key]+_}" ] || return 0
  if [ ! -f "$file" ]; then
    _LAYER_BODY_CACHE[$key]=""
    return 0
  fi
  if [ "$under_agents" = "1" ]; then
    sub="$(section_body agents < "$file")"
  else
    sub="$(<"$file")"
  fi
  body="$(section_body "$harness" <<<"$sub" || true)"
  _LAYER_BODY_CACHE[$key]="$body"
}
```

Record the command-substitution reason in the source comment: callers must invoke `prime_layer_body` synchronously before assigning `line="$(harness_agent_line ...)"`.

- [ ] **Step 3: Make `harness_agent_line` a zero-fork cache reader**

Read the primed entry, strip comments with parameter expansion, and return the first matching line with a Bash loop. Preserve the current first-match and ERE behavior:

```bash
harness_agent_line() {
  local key body line stripped
  key="${1}"$'\x1f'"${2}"$'\x1f'"${4}"
  body="${_LAYER_BODY_CACHE[$key]-}"
  while IFS= read -r line || [ -n "$line" ]; do
    stripped="${line%%#*}"
    if [[ $stripped =~ ^[[:space:]]*${3}[[:space:]]*: ]]; then
      printf '%s' "$stripped"
      return 0
    fi
  done <<<"$body"
}
```

Call `prime_layer_body "$f" "$harness" 1` and `prime_layer_body "$f" default 1` synchronously in `resolve_agent_layers` before its two command substitutions. In `validate_user_agent_values`, call `prime_layer_body "$f" "$h" 1` before scanning that harness's agents. Do not change the precedence loop or provenance assignments.

- [ ] **Step 4: Port both field readers to Bash ERE matching**

Use a variable for the ERE so Bash parses the character classes consistently, and trim raw trailing whitespace with parameter expansion:

```bash
field_of() {
  local re=".*[{,[:space:]]${2}[[:space:]]*:[[:space:]]*([^,}[:space:]]+).*"
  [[ $1 =~ $re ]] && printf '%s' "${BASH_REMATCH[1]}"
}

field_of_raw() {
  local re=".*[{,[:space:]]${2}[[:space:]]*:[[:space:]]*([^,}]*).*" out
  [[ $1 =~ $re ]] || return 0
  out="${BASH_REMATCH[1]}"
  while [[ $out == *[[:space:]] ]]; do out="${out%?}"; done
  printf '%s' "$out"
}
```

- [ ] **Step 5: Run all three unchanged correctness suites**

Run:

```bash
bash tests/test_sync_agents.sh
bash tests/test_sync_agents_codex.sh
bash tests/test_sync_agents_cursor.sh
```

Expected: all PASS, no `NOT OK`, and `test_sync_agents.sh`'s assert total is not lower than Task 1. Diagnose any behavior delta; do not edit an old assertion to accept it.

- [ ] **Step 6: Commit the parser optimization**

```bash
git add sync-agents.sh tests/test_sync_agents.sh
git commit -m "perf(sync-agents): cache parsed agent layers"
```

### Task 3: Retain a performance oracle and record acceptance evidence

**Files:**
- Modify: `tests/test_sync_agents.sh`
- Verify: `sync-agents.sh`, `tests/test_sync_agents_codex.sh`, `tests/test_sync_agents_cursor.sh`

**Interfaces:**
- Consumes: optimized real generation path and existing `make_sandbox` fixture.
- Produces: a standing dominant-parser-command ceiling that fails if priming becomes inert, plus before/after wall-clock evidence for the results artifact.

- [ ] **Step 1: Measure the unmodified-base baseline and the optimized branch with identical commands**

Use a clean temporary harness root for a real no-argument generation pass. Run three samples and record the median; time each test file once on the same machine:

```bash
/usr/bin/time -p env DOCKET_HARNESS_ROOT="$tmp_root" bash sync-agents.sh
/usr/bin/time -p bash tests/test_sync_agents.sh
/usr/bin/time -p bash tests/test_sync_agents_codex.sh
/usr/bin/time -p bash tests/test_sync_agents_cursor.sh
```

Run the baseline commands from a temporary worktree at `origin/main`, never by reverting the feature worktree. Expected: the optimized generation median and each test time improve; unchanged or worse is RED and must be diagnosed before continuing.

- [ ] **Step 2: Add a dominant parser-command counting guard**

In `tests/test_sync_agents.sh`, create PATH shims for `sed`, `head`, `awk`, and `grep`. Each shim appends its own name to `$DOCKET_175_FORK_LOG` and `exec`s the absolute real tool captured before PATH changes. Run one real generation fixture, count log lines with `wc -l`, assert a population greater than zero, and assert `count < 400`. The 400-call ceiling leaves ordinary-edit headroom while remaining far below the measured baseline total of 2,427 calls across those four commands; lower it only if post-change evidence shows a tighter ceiling is stable.

The generated shim body must have this exact behavior (substitute the captured tool path and tool name when writing each file):

```bash
#!/usr/bin/env bash
printf '%s\n' TOOL >> "${DOCKET_175_FORK_LOG:?}"
exec REAL_TOOL "$@"
```

Add a population floor asserting the optimized count is greater than zero, then the ceiling assert. This prevents an empty log or broken shim setup from making the guard vacuously green.

- [ ] **Step 3: Mutation-test the standing guard**

In a disposable copy of `sync-agents.sh`, bypass the synchronous cache priming or replace cached `harness_agent_line` with the old reparsing body, then run the new counting block. Verify the ceiling assertion goes red. Diff the mutated file to prove the intended mutation landed; discard only the disposable copy.

- [ ] **Step 4: Run the full repository suite**

There is no suite runner. Run every test under the configured Bash, keep each file's output, and aggregate failures without an early-exiting producer pipeline:

```bash
suite_rc=0
for test_file in tests/test_*.sh; do
  GIT_EDITOR=true "$DOCKET_BASH_PATH" "$test_file" >"/tmp/0175-$(basename "$test_file").out" 2>&1 || suite_rc=1
done
[ "$suite_rc" = "0" ]
```

Expected: all tests PASS. Record total wall clock and the three focused timings for close-out.

- [ ] **Step 5: Commit the retained performance guard**

```bash
git add tests/test_sync_agents.sh
git commit -m "test(sync-agents): guard parser subprocess budget"
```

## Plan Self-Review

- Spec coverage: CLI validation, eager parse-once cache, both flow-map readers, precedence preservation, unchanged tab coverage, three focused suites, before/after timing, standing subprocess ceiling, mutation evidence, and whole-suite verification are all assigned.
- Placeholder scan: no TBD/TODO or unspecified implementation step remains; the 400-call ceiling is exact and carries a population floor plus mutation proof.
- Interface consistency: every cached key is `file + unit-separator + harness + unit-separator + under_agents`; `prime_layer_body` is synchronous and `harness_agent_line` only reads it; both call sites prime before command substitution.
