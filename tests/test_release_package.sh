#!/usr/bin/env bash
# tests/test_release_package.sh — hermetic guards over the two authored release-candidate surfaces
# (change 0317, Task 11): the non-publishing .github/workflows/release-candidate.yml and the native
# smoke driver scripts/release-smoke.sh, plus the embed tie between the downloader and render.go.
#
# THESE ARE GREP-SHAPED GUARDS. Every assertion below is a BYTE-PATTERN scan of source text, not a
# semantic proof. It proves a spelling is present or absent in the file; it does NOT execute the
# workflow, run a smoke, or observe GitHub Actions. Each section header restates the specific
# spelling limitation it carries. The semantic proofs live elsewhere: the packager and downloader
# behavior is proven by the Go tests and the tests/test_release_downloader*.sh sandbox suites, and
# the four native tuple executions plus the four-harness live acceptance are external truth no
# in-repo test can promote (docs/release/four-harness-acceptance.md).
#
# MUTATION EVIDENCE (Task 11): unpinning one `uses:` to @v4 reddens the SHA-pin guard; adding a
# `git tag` step reddens the publishing-verb ban. Both were exercised at authoring and restored.
# MUTATION EVIDENCE (change 0361, Sections I–L): flipping source-gate's runs-on back to
# ubuntu-24.04 (or package's to macos-15), deleting the tests/** or .docket.yml trigger,
# stripping the BASH_VERSINFO check or the DOCKET_BASH_PATH export from the suite-Bash step, and
# reverting either budget-classifier pattern or its status=1 escalation each redden the owning
# guard. All exercised at authoring and restored.
#
# The ok/nok helpers are the tree's canonical byte-for-byte spelling (see tests/test_release_downloader.sh);
# scripts/run-tests.sh accounts results on the `ok - ` / `NOT OK - ` markers they print.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
WF="$REPO/.github/workflows/release-candidate.yml"
SMOKE="$REPO/scripts/release-smoke.sh"
RENDER="$REPO/internal/release/render.go"
DOWNLOADER="$REPO/internal/release/downloader/install.sh"
fail=0
ok(){ printf 'ok - %s\n' "$1"; }
nok(){ printf 'NOT OK - %s\n' "$1"; fail=1; }

# ================================================================================================
# SECTION A — the workflow file exists (nothing below can scan a missing file)
# ================================================================================================
if [ -f "$WF" ]; then
  ok "release-candidate workflow exists at .github/workflows/release-candidate.yml"
else
  nok "release-candidate workflow missing at .github/workflows/release-candidate.yml"
  exit "$fail"
fi

# ================================================================================================
# SECTION B — read-only permissions. SPELLING LIMIT: this scans permissions-block TEXT for the
# `contents: read` grant and for the `write` spelling; it matches those spellings, not the effective
# GitHub token. The `contents: read` and top-level-presence asserts read the TOP-LEVEL block only;
# the write-grant scan covers the top-level block AND every indented (job-level) `permissions:`
# mapping, since a job-level `permissions: contents: write` escalates a token just as a top-level one
# would. It cannot prove GitHub resolves an effective read-only token — only what this file requests.
# ================================================================================================
# The block is `permissions:` plus the indented lines under it, up to the next column-0 line.
perm_block="$(awk '/^permissions:/{p=1;print;next} p&&/^[^[:space:]]/{p=0} p{print}' "$WF")"
if [ -n "$perm_block" ]; then
  ok "a top-level permissions: block is present"
else
  nok "no top-level permissions: block found — a candidate workflow must pin read-only permissions"
fi
if grep -Eq 'contents:[[:space:]]*read' <<<"$perm_block"; then
  ok "permissions block grants contents: read"
else
  nok "permissions block does not grant contents: read"
fi
# The write-grant scan must cover EVERY permissions: mapping, not just the top-level one — a
# job-level `permissions: contents: write` would escalate a token and evade a top-level-only scan.
# Keyed on SHAPE, not an indent list: a line whose first non-space token is `permissions:` opens a
# block at whatever column it sits (0 for top-level, indented for job-level); its body is the
# more-indented lines beneath it, up to the next line at or below the opener's indent.
all_perm_blocks="$(awk '
  function indent(s){ match(s, /^[[:space:]]*/); return RLENGTH }
  { if (p && $0 !~ /^[[:space:]]*$/ && indent($0) <= base) p=0 }
  /^[[:space:]]*permissions:/ { p=1; base=indent($0); print; next }
  p { print }
' "$WF")"
# Any `write` spelling inside a permissions context is a write grant (contents: write, write-all,
# packages: write, …). No permissions mapping — top-level or job-level — may carry one.
if grep -qw 'write' <<<"$all_perm_blocks"; then
  nok "a write grant appears in a permissions context (every permissions block must be read-only):"
  grep -nw 'write' <<<"$all_perm_blocks" | sed 's/^/    /'
else
  ok "no write grant appears in any permissions context (top-level or job-level)"
fi

# ================================================================================================
# SECTION C — no publishing verbs. SPELLING LIMIT: this bans the literal spellings `gh release`,
# `softprops/action-gh-release`, `git tag`, and `git push` anywhere in the workflow text. A publish
# reached by some other spelling (an aliased tool, a composite action) would slip past; the
# non-publishing guarantee is ultimately the read-only token (Section B) plus review. Because this
# is a pure spelling scan it fires on a comment too — which is what makes the `git tag` mutation
# reddenable.
# ================================================================================================
pub_hits="$(grep -nE 'gh[[:space:]]+release|softprops/action-gh-release|git[[:space:]]+tag|git[[:space:]]+push' "$WF" || true)"
if [ -n "$pub_hits" ]; then
  nok "a publishing verb spelling appears in the workflow (this workflow must not publish):"
  printf '%s\n' "$pub_hits" | sed 's/^/    /'
else
  ok "no publishing verb (gh release, softprops/action-gh-release, git tag, git push) appears"
fi

# ================================================================================================
# SECTION D — every action is SHA-pinned. SPELLING LIMIT: this requires every `uses:` line to carry
# an `@` followed by exactly 40 lowercase hex — the shape of a full commit SHA. It does not verify
# the SHA resolves to the tagged release the trailing `# vN` comment claims.
# ================================================================================================
uses_lines="$(grep -nE '^[[:space:]]*(-[[:space:]]+)?uses:' "$WF" || true)"
if [ -n "$uses_lines" ]; then
  ok "the workflow has at least one uses: step to pin"
else
  nok "the workflow has no uses: step — expected the checkout/setup-go/artifact actions"
fi
unpinned="$(grep -Ev '@[0-9a-f]{40}' <<<"$uses_lines" || true)"
if [ -n "$unpinned" ]; then
  nok "a uses: line is not pinned to a full 40-hex commit SHA:"
  printf '%s\n' "$unpinned" | sed 's/^/    /'
else
  ok "every uses: line is pinned to a full 40-hex commit SHA"
fi

# ================================================================================================
# SECTION E — both workflow_dispatch inputs. SPELLING LIMIT: scans the inputs block for the `ref:`
# and `version:` keys; it does not validate their descriptions or defaults.
# ================================================================================================
inputs_block="$(awk '/^[[:space:]]+inputs:/{p=1;next} p&&/^[[:space:]]{0,4}[a-z_]+:/{if($0 !~ /^[[:space:]]{6,}/){p=0}} p{print}' "$WF")"
# Fall back to the whole workflow-dispatch region if the indent heuristic came up empty.
if [ -z "$inputs_block" ]; then
  inputs_block="$(awk '/workflow_dispatch:/{p=1} p{print}' "$WF")"
fi
if grep -Eq '^[[:space:]]+ref:' <<<"$inputs_block"; then
  ok "workflow_dispatch declares the ref input"
else
  nok "workflow_dispatch is missing the ref input"
fi
if grep -Eq '^[[:space:]]+version:' <<<"$inputs_block"; then
  ok "workflow_dispatch declares the version input"
else
  nok "workflow_dispatch is missing the version input"
fi

# ================================================================================================
# SECTION F — the smoke matrix names all four native tuples. SPELLING LIMIT: this enumerates the
# four approved runner-label spellings (the fixed target set). It cannot tell that a label still
# maps to the architecture the comment claims — that is the authoring-time runner-image
# revalidation recorded in the PR description.
# ================================================================================================
for label in 'macos-15' 'macos-15-intel' 'ubuntu-24\.04' 'ubuntu-24\.04-arm'; do
  if grep -Eq "runner:[[:space:]]*${label}[[:space:]]*$" "$WF"; then
    ok "the smoke matrix names runner ${label}"
  else
    nok "the smoke matrix does not name runner ${label}"
  fi
done

# The workflow's smoke phase must actually invoke the native smoke driver.
if grep -qF -- 'scripts/release-smoke.sh' "$WF"; then
  ok "the workflow invokes scripts/release-smoke.sh"
else
  nok "the workflow never invokes scripts/release-smoke.sh"
fi

# ================================================================================================
# SECTION G — the smoke driver's own contract. SPELLING LIMIT: scans scripts/release-smoke.sh for
# the SMOKE PASS marker the summary greps and the --base-bundle flag the workflow forwards for PRs.
# It does not run the smoke.
# ================================================================================================
if [ -x "$SMOKE" ]; then
  ok "scripts/release-smoke.sh exists and is executable"
else
  nok "scripts/release-smoke.sh is missing or not executable"
fi
if grep -qF -- 'SMOKE PASS' "$SMOKE"; then
  ok "scripts/release-smoke.sh emits the SMOKE PASS marker the summary greps"
else
  nok "scripts/release-smoke.sh does not emit a SMOKE PASS marker"
fi
# Pattern leads with '--' — declare it with -- so grep does not parse it as an option.
if grep -qF -- '--base-bundle' "$SMOKE"; then
  ok "scripts/release-smoke.sh honors the --base-bundle upgrade flag"
else
  nok "scripts/release-smoke.sh does not honor the --base-bundle flag"
fi

# ================================================================================================
# SECTION H — the downloader is embedded. SPELLING LIMIT: scans render.go for the go:embed
# directive naming downloader/install.sh and confirms that file exists. It does not compile the
# package (the Go build is the semantic proof, via go test ./...).
# ================================================================================================
if [ -f "$RENDER" ] && grep -Eq '^//go:embed[[:space:]]+downloader/install\.sh' "$RENDER"; then
  ok "internal/release/render.go embeds downloader/install.sh"
else
  nok "internal/release/render.go does not embed downloader/install.sh"
fi
if [ -f "$DOWNLOADER" ]; then
  ok "the embedded downloader source internal/release/downloader/install.sh exists"
else
  nok "the embedded downloader source internal/release/downloader/install.sh is missing"
fi

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

exit "$fail"
