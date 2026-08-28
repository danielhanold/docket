<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0361 — Release-candidate source-gate green on a macOS runner](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-28-0361-release-candidate-source-gate-macos-runner.md)**
<!-- docket:backlink:end -->
# Release-candidate source-gate on macos-15 — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the `release-candidate` workflow's `source-gate` job a real, green signal by moving it to the platform the suite is authored for (`macos-15`), provisioning GNU Bash 4.3+ explicitly, widening the PR triggers to the suite's inputs, and replacing the stale budget classifier with the runner's full report vocabulary — all pinned by mutation-tested guards.

**Architecture:** Two files change. `.github/workflows/release-candidate.yml` gets four edits confined to the `on:` block and the `source-gate` job (runner label, one new `run:` step, a rewritten suite-step classifier); `package`/`smoke`/`summary` are untouched. `tests/test_release_package.sh` — the file that already owns the workflow's grep-shaped contract — gains four new sections (I–L) built on a shared shape-keyed job/step extractor, written test-first so every guard is seen red against the pre-change workflow, then mutation-tested against targeted reverts after each commit.

**Tech Stack:** GitHub Actions YAML, Bash, awk/grep guards in the repo's hermetic shell-test suite (`scripts/run-tests.sh`).

**Spec:** `docs/superpowers/specs/2026-08-27-release-candidate-source-gate-macos-runner-design.md`

## Global Constraints

- Only `.github/workflows/release-candidate.yml` and `tests/test_release_package.sh` change. The `package`, `smoke`, and `summary` jobs keep their runners and logic verbatim.
- The runner label is pinned `macos-15` — never `macos-latest`, never `macos-14` (spec "Considered alternatives").
- The suite Bash floor is GNU Bash **4.3** (`wait -n`); the setup step must fail loudly rather than fall back to Apple Bash 3.2.
- No new `uses:` steps (the new step is pure `run:`), so Section D's SHA-pin guard population is unchanged; no `write` spelling may enter any `permissions:` context (Section B) and no publishing verb may appear (Section C).
- The workflow does not pass `--strict-budget`; `tests/runtime-budgets.tsv` is not retuned.
- Shell rules from AGENTS.md bind every guard: never `producer | early-exiting-consumer` under `pipefail` (capture to a variable, then match on a herestring); awk indent classes are `[^[:space:]]`, never `[^ ]`; a grep pattern leading with `--` must be declared (`grep -qF -- "<pat>"`).
- Guards key on syntactic **shape** (indent-scoped job/step blocks), not enumerated spellings, and every guard is mutation-tested: the named mutation must redden it, or the guard is a defect.
- Mutation tests run **after** the task's commit, so restore is `git checkout -- <file>` back to the committed state (never restore an uncommitted edit that way). Confirm each mutation landed with `/usr/bin/grep -cF` counts before and after — PATH `grep` is ugrep and must not be used for landing checks.
- One bounded gap per ERE at most; where ordering matters, compare line numbers inside an extracted block instead of stacking gaps.
- Cross-reference comments anchor on symbol names or verbatim-quoted clauses, never line numbers.
- The final gate runs the **whole** resolved suite: `finalize.test_command` in `.docket.yml` resolves to `scripts/run-tests.sh`.

## Verification that no in-repo test can give

The guards below are byte-pattern scans of workflow text (the file's own "THESE ARE GREP-SHAPED GUARDS" header states this contract). Whether GitHub actually schedules `macos-15`, whether Homebrew serves `bash`, and whether the live suite goes green on the runner is external truth: the PR that carries this change itself triggers `release-candidate` (the workflow file is in `pull_request.paths`), and the spec's Acceptance section is checked against that live run at review time, not by the suite.

---

### Task 1: Move `source-gate` to `macos-15`, guarded by job-runner shape asserts

**Files:**
- Modify: `tests/test_release_package.sh` (append SECTION I after SECTION H, before the final `exit "$fail"`)
- Modify: `.github/workflows/release-candidate.yml` (the `source-gate` job's `runs-on:` line)

**Interfaces:**
- Produces: shell function `job_block <job-name>` — prints the 2-space-indented job block for `<job-name>` from `$WF` (opener line through the last line more-indented than 2 spaces; ends at the next 2-space-indented non-blank line, which in this file is the next phase banner comment). Tasks 3 and 4 consume it verbatim; it is defined once, in SECTION I.

- [ ] **Step 1: Write the failing guards**

Append to `tests/test_release_package.sh`, immediately above the trailing `exit "$fail"`:

```bash
# ================================================================================================
# SECTION I — job runner assignments (change 0361). SPELLING LIMIT: extracts each job's block by
# the shape of its 2-space-indent "  <job>:" opener and scans the runs-on TEXT inside it. It
# proves which label this file requests, never what GitHub actually schedules — the live-run
# acceptance is external truth. The macos-15 assert and the no-other-label assert both read the
# SAME extracted runs_on line, so a dead extractor reddens the positive assert (its non-vacuity
# companion) instead of leaving a vacuous negative.
# ================================================================================================
# A job block: the "  <job>:" opener (exactly 2-space indent) through the lines indented deeper
# than 2 spaces; the block ends at the next line whose first 2 columns are spaces and third is
# not (the next job or phase banner).
job_block(){
  awk -v job="$1" '
    $0 ~ "^  " job ":[[:space:]]*$" {p=1; print; next}
    p && /^  [^[:space:]]/ {p=0}
    p {print}
  ' "$WF"
}

sg_block="$(job_block source-gate)"
if [ -n "$sg_block" ]; then
  ok "source-gate job block extracted (population floor for the runner asserts)"
else
  nok "source-gate job block not found — the runner asserts below would be vacuous"
fi
sg_runs_on="$(grep -E 'runs-on:' <<<"$sg_block" || true)"
if grep -Eq '^[[:space:]]*runs-on:[[:space:]]*macos-15[[:space:]]*$' <<<"$sg_runs_on"; then
  ok "source-gate runs on macos-15 (the suite's authored platform)"
else
  nok "source-gate does not run on macos-15; its runs-on is: ${sg_runs_on:-<none>}"
fi
if [ -n "$sg_runs_on" ] && ! grep -q 'ubuntu' <<<"$sg_runs_on"; then
  ok "source-gate's runs-on names no ubuntu label"
else
  nok "source-gate's runs-on still names an ubuntu label (or is missing): ${sg_runs_on:-<none>}"
fi

# The other three jobs RETAIN their runners — moving them is out of scope for change 0361.
for job in package summary; do
  jb="$(job_block "$job")"
  if [ -n "$jb" ] && grep -Eq '^[[:space:]]*runs-on:[[:space:]]*ubuntu-24\.04[[:space:]]*$' <<<"$jb"; then
    ok "$job job retains runs-on ubuntu-24.04"
  else
    nok "$job job block missing or no longer runs on ubuntu-24.04"
  fi
done
smoke_block="$(job_block smoke)"
if [ -n "$smoke_block" ] && grep -qF -- 'runs-on: ${{ matrix.runner }}' <<<"$smoke_block"; then
  ok "smoke job retains its matrix runner indirection"
else
  nok "smoke job block missing or no longer runs on \${{ matrix.runner }}"
fi
```

- [ ] **Step 2: Run the test to verify the new guards fail against the unchanged workflow**

Run: `bash tests/test_release_package.sh; echo "exit=$?"`
Expected: `NOT OK - source-gate does not run on macos-15 …` and `NOT OK - source-gate's runs-on still names an ubuntu label …`; all four retention asserts `ok`; `exit=1`. (This red run is the mutation evidence for the "flip source-gate back to Ubuntu" mutation: the guard has been seen red against exactly that state.)

- [ ] **Step 3: Edit the workflow**

In `.github/workflows/release-candidate.yml`, inside the `source-gate:` job, change:

```yaml
    runs-on: ubuntu-24.04
```

to:

```yaml
    runs-on: macos-15
```

Only the occurrence inside the `source-gate` job (the first job under `jobs:`); the `package` and `summary` jobs and the smoke matrix keep theirs. Also update the job's phase-banner/header comment block at the top of the file if you touch nothing else there — no comment change is required by this task.

- [ ] **Step 4: Run the test to verify it passes**

Run: `bash tests/test_release_package.sh; echo "exit=$?"`
Expected: every SECTION I assert `ok`; `exit=0`.

- [ ] **Step 5: Commit**

```bash
git add tests/test_release_package.sh .github/workflows/release-candidate.yml
git commit -m "fix(0361): move source-gate to macos-15 with shape-keyed runner guards"
```

- [ ] **Step 6: Mutation-test the retention guard**

The red run in Step 2 already proved the macos-15 assert reddens on an Ubuntu source-gate. Now prove the retention direction:

```bash
# Mutate: retarget the package job's runner (the retention guard's named mutation).
/usr/bin/grep -cF 'runs-on: ubuntu-24.04' .github/workflows/release-candidate.yml   # expect 2 (package, summary)
perl -0pi -e 's/(package:\n(?:.*\n)*?    runs-on: )ubuntu-24\.04/${1}macos-15/' .github/workflows/release-candidate.yml
/usr/bin/grep -cF 'runs-on: ubuntu-24.04' .github/workflows/release-candidate.yml   # expect 1 — the mutation LANDED
bash tests/test_release_package.sh; echo "exit=$?"                                   # expect NOT OK on package retention, exit=1
git checkout -- .github/workflows/release-candidate.yml
bash tests/test_release_package.sh; echo "exit=$?"                                   # expect exit=0 — restored
```

If the mutated run stays green, the guard is a defect: stop and fix the guard, then repeat.

---

### Task 2: Trigger the workflow on `tests/**` and `.docket.yml` changes

**Files:**
- Modify: `tests/test_release_package.sh` (append SECTION J after SECTION I)
- Modify: `.github/workflows/release-candidate.yml` (the `on.pull_request.paths` list)

**Interfaces:**
- Consumes: nothing from Task 1 (SECTION J uses its own paths-block extractor; the `paths:` list is at deep indent inside `on:`, not a job).

- [ ] **Step 1: Write the failing guards**

Append to `tests/test_release_package.sh`, above `exit "$fail"`:

```bash
# ================================================================================================
# SECTION J — pull_request path triggers (change 0361). SPELLING LIMIT: extracts the single
# paths: list in this file by shape (the "paths:" opener, then contiguous "- " item lines) and
# scans for the quoted item spellings. It proves the file lists the paths, not that GitHub's
# filter semantics match them. The scripts/** assert is the live companion: it reads a
# pre-existing entry through the SAME extractor, so a dead extractor reddens it.
# ================================================================================================
paths_block="$(awk '
  /^[[:space:]]+paths:[[:space:]]*$/ {p=1; next}
  p { if ($0 ~ /^[[:space:]]*-[[:space:]]/) print; else p=0 }
' "$WF")"
if [ -n "$paths_block" ]; then
  ok "pull_request paths: list extracted"
else
  nok "no pull_request paths: list found — the trigger asserts below would be vacuous"
fi
# Patterns lead with '-' — declare them with -- so grep does not parse them as options.
if grep -qF -- "- 'scripts/**'" <<<"$paths_block"; then
  ok "paths list still triggers on scripts/** (live companion through the same extractor)"
else
  nok "paths list lost its scripts/** entry (or the extractor went dead)"
fi
if grep -qF -- "- 'tests/**'" <<<"$paths_block"; then
  ok "a tests/** change triggers the release-candidate workflow"
else
  nok "the paths list does not include tests/** — a suite change would bypass the source gate"
fi
if grep -qF -- "- '.docket.yml'" <<<"$paths_block"; then
  ok "a .docket.yml change triggers the release-candidate workflow"
else
  nok "the paths list does not include .docket.yml — a finalize.test_command change would bypass the source gate"
fi
```

- [ ] **Step 2: Run the test to verify the two new-entry guards fail**

Run: `bash tests/test_release_package.sh; echo "exit=$?"`
Expected: `NOT OK` for the `tests/**` and `.docket.yml` asserts; the extractor-floor and `scripts/**` companion `ok`; `exit=1`. (Red-run mutation evidence for "remove either trigger".)

- [ ] **Step 3: Edit the workflow**

In `.github/workflows/release-candidate.yml`, extend `on.pull_request.paths`:

```yaml
on:
  pull_request:
    paths:
      - 'cmd/**'
      - 'internal/**'
      - 'go.mod'
      - 'go.sum'
      - 'skills/**'
      - 'agents/**'
      - 'install.sh'
      - 'scripts/**'
      - 'tests/**'
      - '.docket.yml'
      - '.github/workflows/release-candidate.yml'
```

(The two new entries are `- 'tests/**'` and `- '.docket.yml'`; every existing entry stays.)

- [ ] **Step 4: Run the test to verify it passes**

Run: `bash tests/test_release_package.sh; echo "exit=$?"`
Expected: all SECTION J asserts `ok`; `exit=0`.

- [ ] **Step 5: Commit**

```bash
git add tests/test_release_package.sh .github/workflows/release-candidate.yml
git commit -m "fix(0361): trigger release-candidate on tests/** and .docket.yml"
```

- [ ] **Step 6: Mutation-test each trigger independently**

```bash
/usr/bin/grep -cF -- "- 'tests/**'" .github/workflows/release-candidate.yml      # expect 1
perl -0pi -e "s/^\s*- 'tests\/\*\*'\n//m" .github/workflows/release-candidate.yml
/usr/bin/grep -cF -- "- 'tests/**'" .github/workflows/release-candidate.yml      # expect 0 — landed
bash tests/test_release_package.sh; echo "exit=$?"                                # expect NOT OK tests/**, exit=1
git checkout -- .github/workflows/release-candidate.yml

/usr/bin/grep -cF -- "- '.docket.yml'" .github/workflows/release-candidate.yml   # expect 1
perl -0pi -e "s/^\s*- '\.docket\.yml'\n//m" .github/workflows/release-candidate.yml
/usr/bin/grep -cF -- "- '.docket.yml'" .github/workflows/release-candidate.yml   # expect 0 — landed
bash tests/test_release_package.sh; echo "exit=$?"                                # expect NOT OK .docket.yml, exit=1
git checkout -- .github/workflows/release-candidate.yml
bash tests/test_release_package.sh; echo "exit=$?"                                # expect exit=0 — restored
```

Any green mutated run is a guard defect: fix the guard, repeat.

---

### Task 3: Provision GNU Bash 4.3+ and export `DOCKET_BASH_PATH`

**Files:**
- Modify: `tests/test_release_package.sh` (append SECTION K after SECTION J)
- Modify: `.github/workflows/release-candidate.yml` (one new step in `source-gate`, between `Set up Go` and `Prove tree cleanliness and resolve build identity`)

**Interfaces:**
- Consumes: `job_block` from Task 1 (SECTION I), exactly as defined there.
- Produces: the workflow step named `Provision suite Bash (GNU 4.3+)`, which exports `DOCKET_BASH_PATH=<absolute path>` via `GITHUB_ENV`. The suite step (Task 4's subject) relies on `scripts/run-tests.sh`'s existing re-exec: when launched by Bash 3.2 it re-executes itself through `DOCKET_BASH_PATH` (the `DOCKET_RUNTESTS_REEXEC` branch near the top of `scripts/run-tests.sh`) — no change to the suite step's `bash $test_cmd` invocation is needed for that.

- [ ] **Step 1: Write the failing guards**

Append to `tests/test_release_package.sh`, above `exit "$fail"`:

```bash
# ================================================================================================
# SECTION K — the suite-Bash provisioning step (change 0361). SPELLING LIMIT: extracts the
# source-gate step named "Provision suite Bash" by step shape ("- name:" opener to the next
# "- name:") and asserts its TEXT: the brew resolution spelling, the BASH_VERSINFO version
# check, and the GITHUB_ENV export — plus the ORDER of check and export by line number inside
# the extracted step (a single-gap-free way to pin "export only after verifying"). It cannot
# prove the step runs, that brew serves Bash >= 4.3, or that the exported path is executable —
# the step's own runtime refusal and the live workflow run own those.
# ================================================================================================
bash_step="$(job_block source-gate | awk '
  /- name: Provision suite Bash/ {p=1; print; next}
  p && /^[[:space:]]*- name:/ {p=0}
  p {print}
')"
if [ -n "$bash_step" ]; then
  ok "source-gate has a Provision suite Bash step"
else
  nok "source-gate has no Provision suite Bash step — the suite would run on Apple Bash 3.2"
fi
if grep -qF 'brew install bash' <<<"$bash_step"; then
  ok "the suite-Bash step installs the Homebrew bash formula when absent"
else
  nok "the suite-Bash step never installs the bash formula"
fi
if grep -qF 'prefix="$(brew --prefix bash)"' <<<"$bash_step" \
   && grep -qF 'suite_bash="$prefix/bin/bash"' <<<"$bash_step"; then
  ok "the suite-Bash step resolves an absolute path via brew --prefix bash"
else
  nok "the suite-Bash step does not resolve the bash path through brew --prefix"
fi
# Order: the BASH_VERSINFO floor check must precede the GITHUB_ENV export — line numbers within
# the extracted step, not a stacked-gap regex.
ver_ln="$(awk '/BASH_VERSINFO/{print NR; exit}' <<<"$bash_step")"
exp_ln="$(awk '/DOCKET_BASH_PATH=.*GITHUB_ENV/{print NR; exit}' <<<"$bash_step")"
if [ -n "$ver_ln" ]; then
  ok "the suite-Bash step version-checks via BASH_VERSINFO"
else
  nok "the suite-Bash step has no BASH_VERSINFO version check"
fi
if [ -n "$exp_ln" ]; then
  ok "the suite-Bash step exports DOCKET_BASH_PATH through GITHUB_ENV"
else
  nok "the suite-Bash step never exports DOCKET_BASH_PATH through GITHUB_ENV"
fi
if [ -n "$ver_ln" ] && [ -n "$exp_ln" ] && [ "$ver_ln" -lt "$exp_ln" ]; then
  ok "the version check precedes the DOCKET_BASH_PATH export"
else
  nok "DOCKET_BASH_PATH is exported without a preceding version check"
fi
if grep -qF 'need GNU Bash >= 4.3' <<<"$bash_step"; then
  ok "the suite-Bash step names the 4.3 floor in its refusal"
else
  nok "the suite-Bash step does not state the GNU Bash 4.3 floor"
fi
```

- [ ] **Step 2: Run the test to verify the new guards fail**

Run: `bash tests/test_release_package.sh; echo "exit=$?"`
Expected: every SECTION K assert `NOT OK` (the step does not exist yet); `exit=1`.

- [ ] **Step 3: Add the workflow step**

In `.github/workflows/release-candidate.yml`, inside `source-gate`, insert after the `Set up Go` step and before `Prove tree cleanliness and resolve build identity`:

```yaml
      - name: Provision suite Bash (GNU 4.3+)
        run: |
          set -euo pipefail
          # The suite is authored against GNU Bash >= 4.3 (wait -n); the macOS image ships Apple
          # Bash 3.2 at /bin/bash. Resolve the Homebrew formula's absolute path, refuse anything
          # older than the floor — never silently fall back — and hand the path to later steps as
          # DOCKET_BASH_PATH, which scripts/run-tests.sh re-execs itself through.
          if ! brew list --versions bash >/dev/null 2>&1; then
            brew install bash
          fi
          prefix="$(brew --prefix bash)"
          suite_bash="$prefix/bin/bash"
          if [ ! -x "$suite_bash" ]; then
            echo "::error::no executable bash at $suite_bash (brew --prefix bash resolved $prefix)" >&2
            exit 1
          fi
          ver="$("$suite_bash" -c 'printf %s "${BASH_VERSINFO[0]}.${BASH_VERSINFO[1]}"')"
          major="${ver%%.*}"
          minor="${ver#*.}"
          if [ "$major" -lt 4 ] || { [ "$major" -eq 4 ] && [ "$minor" -lt 3 ]; }; then
            echo "::error::suite bash at $suite_bash is $ver; need GNU Bash >= 4.3" >&2
            exit 1
          fi
          echo "suite bash: $suite_bash (GNU Bash $ver)" >&2
          echo "DOCKET_BASH_PATH=$suite_bash" >>"$GITHUB_ENV"
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `bash tests/test_release_package.sh; echo "exit=$?"`
Expected: all SECTION K asserts `ok`; `exit=0`.

- [ ] **Step 5: Commit**

```bash
git add tests/test_release_package.sh .github/workflows/release-candidate.yml
git commit -m "fix(0361): provision GNU Bash 4.3+ on the source-gate runner via DOCKET_BASH_PATH"
```

- [ ] **Step 6: Mutation-test the version-check/export coupling**

The Step 2 red run covered "step absent". Now the two named surviving mutations:

```bash
# Mutation A: strip the version check but keep the export ("export without verifying").
/usr/bin/grep -cF 'BASH_VERSINFO' .github/workflows/release-candidate.yml   # expect 1
perl -0pi -e 's/^\s*ver="\$\("\$suite_bash".*?fi\n//sm' .github/workflows/release-candidate.yml
/usr/bin/grep -cF 'BASH_VERSINFO' .github/workflows/release-candidate.yml   # expect 0 — landed
bash tests/test_release_package.sh; echo "exit=$?"                           # expect NOT OK (BASH_VERSINFO + ordering + floor asserts), exit=1
git checkout -- .github/workflows/release-candidate.yml

# Mutation B: drop the GITHUB_ENV export.
/usr/bin/grep -cF 'DOCKET_BASH_PATH=$suite_bash' .github/workflows/release-candidate.yml  # expect 1
perl -0pi -e 's/^\s*echo "DOCKET_BASH_PATH=\$suite_bash" >>"\$GITHUB_ENV"\n//m' .github/workflows/release-candidate.yml
/usr/bin/grep -cF 'DOCKET_BASH_PATH=$suite_bash' .github/workflows/release-candidate.yml  # expect 0 — landed
bash tests/test_release_package.sh; echo "exit=$?"                           # expect NOT OK (export + ordering asserts), exit=1
git checkout -- .github/workflows/release-candidate.yml
bash tests/test_release_package.sh; echo "exit=$?"                           # expect exit=0 — restored
```

If a `perl` edit does not land (count unchanged), the mutation never happened — a green run proves nothing; fix the substitution and repeat. If a landed mutation leaves the suite green, the guard is a defect: fix the guard.

---

### Task 4: Replace the stale budget classifier with the full report contract

**Files:**
- Modify: `tests/test_release_package.sh` (append SECTION L after SECTION K)
- Modify: `.github/workflows/release-candidate.yml` (the `Run the resolved test suite` step's classifier tail)

**Interfaces:**
- Consumes: `job_block` from Task 1.
- Produces: nothing later tasks rely on.

**Background for the implementer:** `scripts/run-tests.sh` emits, at line start: `BUDGET WATCH:` and `PARALLEL-SENSITIVE:` (screening — contended parallel wall-clock observations, machine-dependent, non-fatal by design), and `OVER BUDGET:` and `SERIAL CONFIRMED OVER BUDGET:` (authoritative — uncontended measurements/serial confirmations). Per-file report lines also carry an *indented* `  OVER BUDGET (ceiling …)` suffix, which is why every classifier grep below is `^`-anchored. The suite exits 0 on an advisory breach (no `--strict-budget`), so the workflow must fail an otherwise-green run itself when an authoritative line appears. The current step greps only `^OVER BUDGET:` and never fails on it — both halves are stale.

- [ ] **Step 1: Write the failing guards**

Append to `tests/test_release_package.sh`, above `exit "$fail"`:

```bash
# ================================================================================================
# SECTION L — the suite step's budget-report classifier (change 0361). SPELLING LIMIT: extracts
# the "Run the resolved test suite" step and asserts the classifier TEXT: the two combined
# grep patterns (screening and authoritative — asserted as fixed strings, so this pins the exact
# pattern spelling the step ships), the summary append for screening, and — by line order inside
# the step — that the authoritative capture precedes a bare status=1 escalation. It cannot prove
# the branch logic runs or that screening leaves the exit status untouched; the mutation tests
# and the live run own that.
# ================================================================================================
suite_step="$(job_block source-gate | awk '
  /- name: Run the resolved test suite/ {p=1; print; next}
  p && /^[[:space:]]*- name:/ {p=0}
  p {print}
')"
if [ -n "$suite_step" ]; then
  ok "source-gate has the Run the resolved test suite step"
else
  nok "source-gate lost its Run the resolved test suite step"
fi
if grep -qF 'finalize.test_command' <<<"$suite_step" && grep -qF '.docket.yml' <<<"$suite_step"; then
  ok "the suite step still resolves the command from .docket.yml finalize.test_command (live companion)"
else
  nok "the suite step no longer resolves finalize.test_command from .docket.yml"
fi
if grep -qF -- "'^(BUDGET WATCH|PARALLEL-SENSITIVE):'" <<<"$suite_step"; then
  ok "the classifier recognizes the screening vocabulary (BUDGET WATCH, PARALLEL-SENSITIVE)"
else
  nok "the classifier does not recognize the screening vocabulary"
fi
if grep -qF -- "'^(OVER BUDGET|SERIAL CONFIRMED OVER BUDGET):'" <<<"$suite_step"; then
  ok "the classifier recognizes the authoritative vocabulary (OVER BUDGET, SERIAL CONFIRMED OVER BUDGET)"
else
  nok "the classifier does not recognize the authoritative vocabulary"
fi
# The authoritative capture must be followed by a status=1 escalation (fail an otherwise-green
# gate); screening must be followed by a step-summary append. Line order inside the step, not a
# stacked-gap regex.
scr_ln="$(awk '/\^\(BUDGET WATCH\|PARALLEL-SENSITIVE\):/{print NR; exit}' <<<"$suite_step")"
scr_sum_ln="$(awk -v start="${scr_ln:-0}" 'NR>start && /GITHUB_STEP_SUMMARY/{print NR; exit}' <<<"$suite_step")"
aut_ln="$(awk '/\^\(OVER BUDGET\|SERIAL CONFIRMED OVER BUDGET\):/{print NR; exit}' <<<"$suite_step")"
esc_ln="$(awk -v start="${aut_ln:-0}" 'NR>start && /^[[:space:]]*status=1[[:space:]]*$/{print NR; exit}' <<<"$suite_step")"
if [ -n "$scr_ln" ] && [ -n "$scr_sum_ln" ]; then
  ok "screening findings are appended to the job summary"
else
  nok "no job-summary append follows the screening capture"
fi
if [ -n "$aut_ln" ] && [ -n "$esc_ln" ]; then
  ok "an authoritative budget finding escalates to status=1 (fails an otherwise-green gate)"
else
  nok "no status=1 escalation follows the authoritative capture"
fi
```

- [ ] **Step 2: Run the test to verify the new guards fail**

Run: `bash tests/test_release_package.sh; echo "exit=$?"`
Expected: the step-presence and `finalize.test_command` companion asserts `ok`; the four classifier asserts `NOT OK` (the current step knows only `^OVER BUDGET:` and never escalates); `exit=1`. (Red-run mutation evidence for "revert the budget classifier".)

- [ ] **Step 3: Rewrite the classifier tail of the suite step**

In `.github/workflows/release-candidate.yml`, in the `Run the resolved test suite` step, keep everything through `set -e` unchanged and replace the block from the `# Surface any OVER BUDGET: block …` comment through `exit "$status"` with:

```bash
          # Budget-report classification (change 0361) — run-tests.sh's complete vocabulary, all
          # patterns line-anchored so a per-file "  OVER BUDGET (ceiling …)" suffix never matches.
          # Screening findings are contended parallel wall-clock observations: machine-dependent
          # by design, so they are summarized and never change the exit status.
          screening="$(grep -nE '^(BUDGET WATCH|PARALLEL-SENSITIVE):' suite.log || true)"
          if [ -n "$screening" ]; then
            {
              echo "### Budget screening findings (non-fatal)"
              echo ""
              echo '```'
              printf '%s\n' "$screening"
              echo '```'
            } >>"$GITHUB_STEP_SUMMARY"
          fi
          # Authoritative findings — a direct uncontended breach or a serial confirmation. The
          # suite itself exits 0 on an advisory breach (no --strict-budget), so an otherwise-green
          # gate must fail here or nothing fails at all.
          authoritative="$(grep -nE '^(OVER BUDGET|SERIAL CONFIRMED OVER BUDGET):' suite.log || true)"
          if [ -n "$authoritative" ]; then
            {
              echo "### Authoritative budget findings"
              echo ""
              echo '```'
              printf '%s\n' "$authoritative"
              echo '```'
            } >>"$GITHUB_STEP_SUMMARY"
            if [ "$status" -eq 0 ]; then
              echo "::error::authoritative budget finding on an otherwise-green suite" >&2
              status=1
            fi
          fi
          exit "$status"
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `bash tests/test_release_package.sh; echo "exit=$?"`
Expected: all SECTION L asserts `ok`; `exit=0`.

- [ ] **Step 5: Commit**

```bash
git add tests/test_release_package.sh .github/workflows/release-candidate.yml
git commit -m "fix(0361): classify the full budget-report vocabulary in the source-gate suite step"
```

- [ ] **Step 6: Mutation-test the classifier guards**

```bash
# Mutation A: revert the authoritative pattern to the stale single spelling.
/usr/bin/grep -cF 'SERIAL CONFIRMED OVER BUDGET' .github/workflows/release-candidate.yml  # expect 1
perl -0pi -e "s/\Q'^(OVER BUDGET|SERIAL CONFIRMED OVER BUDGET):'\E/'^OVER BUDGET:'/" .github/workflows/release-candidate.yml
/usr/bin/grep -cF 'SERIAL CONFIRMED OVER BUDGET' .github/workflows/release-candidate.yml  # expect 0 — landed
bash tests/test_release_package.sh; echo "exit=$?"   # expect NOT OK on the authoritative-vocabulary assert, exit=1
git checkout -- .github/workflows/release-candidate.yml

# Mutation B: drop the escalation (authoritative finding no longer fails a green gate).
/usr/bin/grep -cE '^ +status=1$' .github/workflows/release-candidate.yml   # expect 1
perl -0pi -e 's/^\s*status=1\n//m' .github/workflows/release-candidate.yml
/usr/bin/grep -cE '^ +status=1$' .github/workflows/release-candidate.yml   # expect 0 — landed
bash tests/test_release_package.sh; echo "exit=$?"   # expect NOT OK on the status=1 escalation assert, exit=1
git checkout -- .github/workflows/release-candidate.yml

# Mutation C: drop the screening pattern.
/usr/bin/grep -cF 'PARALLEL-SENSITIVE' .github/workflows/release-candidate.yml  # expect 1
perl -0pi -e "s/\Q'^(BUDGET WATCH|PARALLEL-SENSITIVE):'\E/'^BUDGET WATCH:'/" .github/workflows/release-candidate.yml
/usr/bin/grep -cF 'PARALLEL-SENSITIVE' .github/workflows/release-candidate.yml  # expect 0 — landed
bash tests/test_release_package.sh; echo "exit=$?"   # expect NOT OK on the screening-vocabulary assert, exit=1
git checkout -- .github/workflows/release-candidate.yml
bash tests/test_release_package.sh; echo "exit=$?"   # expect exit=0 — restored
```

Same rule as before: an unlanded mutation proves nothing; a landed-but-green mutation is a guard defect to fix before proceeding.

---

### Task 5: Record the mutation evidence and run the whole resolved suite

**Files:**
- Modify: `tests/test_release_package.sh` (the `MUTATION EVIDENCE` header comment near the top of the file)

**Interfaces:**
- Consumes: the committed state of Tasks 1–4.

- [ ] **Step 1: Extend the file's mutation-evidence header**

In `tests/test_release_package.sh`, the header comment contains:

```
# MUTATION EVIDENCE (Task 11): unpinning one `uses:` to @v4 reddens the SHA-pin guard; adding a
# `git tag` step reddens the publishing-verb ban. Both were exercised at authoring and restored.
```

Append directly below it (keep the existing lines):

```
# MUTATION EVIDENCE (change 0361, Sections I–L): flipping source-gate's runs-on back to
# ubuntu-24.04 (or package's to macos-15), deleting the tests/** or .docket.yml trigger,
# stripping the BASH_VERSINFO check or the DOCKET_BASH_PATH export from the suite-Bash step, and
# reverting either budget-classifier pattern or its status=1 escalation each redden the owning
# guard. All exercised at authoring and restored.
```

- [ ] **Step 2: Run the changed test file once more**

Run: `bash tests/test_release_package.sh; echo "exit=$?"`
Expected: `exit=0`, no `NOT OK` lines.

- [ ] **Step 3: Run the whole resolved suite**

The suite command is what `finalize.test_command` in `.docket.yml` resolves to — `scripts/run-tests.sh`. Run the whole suite, never only this file:

Run: `scripts/run-tests.sh`
Expected: final `SUITE files=… failed=0 …` line. Treat any `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` line as a screening finding to report in the results notes; a `SERIAL CONFIRMED OVER BUDGET:` line is an authoritative breach to act on before proceeding.

- [ ] **Step 4: Commit**

```bash
git add tests/test_release_package.sh
git commit -m "fix(0361): record Sections I-L mutation evidence in the guard header"
```

---

## Self-Review

Checked against the spec with fresh eyes:

- **Runner move** ("The change") → Task 1; `package`/`summary`/`smoke` retention asserted, `macos-15` pinned, no `macos-latest`/`macos-14`.
- **Trigger coverage** → Task 2 (`tests/**`, `.docket.yml`, existing filters untouched, `scripts/**` companion).
- **Bash on the runner** (install-if-absent, `brew --prefix` absolute path, refuse < 4.3, `GITHUB_ENV` export; suite step's `bash $test_cmd` untouched because `run-tests.sh` re-execs through `DOCKET_BASH_PATH`) → Task 3.
- **Budgets** (screening summarized non-fatal; authoritative fails an otherwise-green gate; no `--strict-budget`, no budget retuning) → Task 4.
- **Regression guards + mutation testing** → Sections I–L, each with its named mutation exercised post-commit; evidence recorded in Task 5.
- **Acceptance items that live outside the repo** (live green run, log showing GNU Bash 4.3+, four smoke legs green) → "Verification that no in-repo test can give" section; the PR itself triggers the workflow.
- Types/names consistent across tasks: `job_block` (Task 1) is the only shared symbol and Tasks 3–4 use it byte-identically; step names `Provision suite Bash (GNU 4.3+)` and `Run the resolved test suite` match between guard extractors and workflow YAML.
