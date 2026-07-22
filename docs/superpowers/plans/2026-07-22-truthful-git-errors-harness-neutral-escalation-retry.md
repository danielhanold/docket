# Truthful Git Errors and Harness-Neutral Escalation Retry Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Preserve the real `git fetch origin` failure in the deterministic resolver and give every Docket operating skill one shared, harness-neutral exact-command sandbox-retry contract.

**Architecture:** `scripts/docket-config.sh` remains the only deterministic resolver and retains the fetch return-code gate; its Stage-1 failure path captures Git stderr, emits a Docket-owned neutral wrapper, replays the diagnostic, and emits no config values. `docket-convention` owns the agent-level recovery rule beside Step 0, so operating skills inherit it instead of gaining per-harness or per-skill branches. Hermetic fake-Git tests pin the resolver behavior, while the existing facade-wiring test pins the one shared contract section structurally.

**Tech Stack:** Bash, Git command seam (`GIT`), POSIX-style shell utilities, Markdown contracts, repository shell-test harness.

## Global Constraints

- Keep `docket.sh preflight` as the only Step-0 facade invocation; retry the exact outer command once through the host harness, never an extracted helper.
- Never add `sudo`, shell-owned elevation, automatic approval, sandbox disabling, retry loops, or harness-specific command syntax to Docket scripts.
- Preserve the existing fail-closed behavior after the one eligible sandbox/permission retry is unavailable, denied, or fails.
- Capture all fetch stderr, including multiline diagnostics; emit the neutral `docket-config: git fetch origin failed` wrapper even when Git produced no stderr; emit no `KEY=value` stdout on failure.
- Keep ordinary network, authentication, conflict, non-fast-forward, and repository failures on their existing caller-defined posture; do not infer sandbox denial from every Git failure.
- Derive guard scope from the shared convention section and mutate every new assertion’s protected behavior to prove it reddens.
- Run the full `tests/test_*.sh` suite before opening the PR, not only the focused tests.

---

### Task 1: Preserve the resolver’s real fetch failure

**Files:**

- Modify: `scripts/docket-config.sh:1-130`
- Modify: `tests/test_docket_config.sh:231-251`

**Interfaces:**

- Consumes: `GIT` as the existing executable seam; `g()` forwards `-C "$REPO_DIR"` and the supplied Git subcommand.
- Produces: On Stage-1 fetch failure, stderr starts with `docket-config: git fetch origin failed`, followed by Git’s captured stderr; stdout stays empty and exit status remains nonzero.

- [ ] **Step 1: Add a discriminating fake-Git regression fixture.**

  In the fail-closed section of `tests/test_docket_config.sh`, create an executable fake Git program under `$tmp`. It must return success for `rev-parse`, fail only when its argv contains `fetch`, and write two unique diagnostic lines to stderr. Invoke the resolver with `GIT="$fake_git"`, capture stdout and stderr separately, and assert all five independent properties:

  ```bash
  assert "fetch failure: nonzero exit" '[ "$fetch_rc" -ne 0 ]'
  assert "fetch failure: emits no KEY=value stdout" '[ -z "$fetch_stdout" ]'
  assert "fetch failure: neutral wrapper is present" \
    'grep -qxF "docket-config: git fetch origin failed" <<<"$fetch_stderr"'
  assert "fetch failure: first fake diagnostic is preserved" \
    'grep -qxF "fake-git: permission boundary denied" <<<"$fetch_stderr"'
  assert "fetch failure: second fake diagnostic is preserved" \
    'grep -qxF "fake-git: cannot write origin lock" <<<"$fetch_stderr"'
  assert "fetch failure: old network diagnosis is absent" \
    '! grep -qF "cannot reach origin" <<<"$fetch_stderr" && ! grep -qF "check the remote/network" <<<"$fetch_stderr"'
  ```

  The fake’s command discriminator must scan argv rather than assume an argument index, because `g()` inserts `-C <repo>` before the Git subcommand.

- [ ] **Step 2: Run the focused test against the baseline and verify the new wrapper assertion is red.**

  Run: `bash tests/test_docket_config.sh`

  Expected: nonzero; the fetch-failure wrapper/preserved-diagnostic assertions report `NOT OK`, proving the baseline still suppresses stderr and makes the false network claim.

- [ ] **Step 3: Implement captured fetch diagnostics without weakening the return-code gate.**

  In `scripts/docket-config.sh`, initialize the diagnostic path before Stage 1 and extend the one EXIT cleanup trap so it removes both the config temp file and this fetch-stderr temp file. Replace the Stage-1 one-liner with a failure branch shaped as follows:

  ```bash
  FETCH_ERR="$(mktemp)" || die "could not create git-fetch diagnostic file"
  if ! g fetch --quiet origin 2>"$FETCH_ERR"; then
    printf 'docket-config: git fetch origin failed\n' >&2
    cat "$FETCH_ERR" >&2
    exit 1
  fi
  rm -f "$FETCH_ERR"
  FETCH_ERR=""
  ```

  Keep `g remote set-head origin -a` and all subsequent Stage-1/Stage-2 behavior unchanged. Do not redirect the captured Git stderr to `/dev/null`, reclassify it, or read cached `origin/HEAD` after a failed fetch.

- [ ] **Step 4: Run the focused regression test and confirm the new behavior passes.**

  Run: `bash tests/test_docket_config.sh`

  Expected: exit 0; the fake’s two stderr lines and the neutral wrapper are visible, while the existing destroyed-remote/stale-cache cases still prove failure is keyed on fetch return status.

- [ ] **Step 5: Mutation-test both halves of the diagnostic contract.**

  In an isolated temporary copy of the feature worktree, make these two one-at-a-time mutations and run `bash tests/test_docket_config.sh` after each:

  ```bash
  # M1: restore the old suppression and network-specific failure line.
  g fetch --quiet origin 2>/dev/null || die "cannot reach origin (git fetch failed) — check the remote/network"

  # M2: preserve captured stderr but delete only the neutral wrapper printf.
  ```

  Expected: both mutations exit nonzero. M1 must redden the preserved-diagnostic and old-text-absence assertions; M2 must redden the wrapper assertion. Restore the unmodified feature worktree after each mutation.

- [ ] **Step 6: Commit the resolver and focused-test deliverable.**

  ```bash
  git add scripts/docket-config.sh tests/test_docket_config.sh
  git commit -m "fix: preserve git fetch diagnostics"
  ```

### Task 2: Establish and guard the shared harness-native recovery rule

**Files:**

- Modify: `skills/docket-convention/SKILL.md:Step-0 preamble`
- Modify: `scripts/docket-config.md:Stage 1, Exit codes, Invariants`
- Modify: `tests/test_skill_facade_wiring.sh:Layer 2 convention anchors`

**Interfaces:**

- Consumes: a required Docket facade or direct Git command that failed with evidence of a host sandbox/permission denial.
- Produces: one exact-command retry through the host harness’s native approval mechanism before the caller applies its already-defined normal failure posture.

- [ ] **Step 1: Add a failing structural contract guard in the facade-wiring test.**

  Extract a single named recovery subsection from `docket-convention/SKILL.md` with start/end heading anchors, then assert the extracted section is nonempty and carries all five structural requirements:

  ```bash
  assert "shared recovery section exists exactly once" '[ "$recovery_heading_count" = "1" ]'
  assert "recovery rule requires sandbox or permission evidence" 'grep -qiE "sandbox|permission" <<<"$recovery"'
  assert "recovery rule retries the exact command" 'grep -qi "exact command" <<<"$recovery"'
  assert "recovery rule uses the harness-native approval boundary" 'grep -qiE "harness.*(native|approval)|native.*approval" <<<"$recovery"'
  assert "recovery rule limits the retry to one attempt" 'grep -qiE "once|one.*attempt" <<<"$recovery"'
  assert "recovery rule falls back to the caller posture" 'grep -qi "existing failure posture" <<<"$recovery"'
  ```

  Scope every assertion to the extracted shared convention section, not a hand-maintained list of operating skills. Keep the test’s existing facade spelling and inventory guards intact.

- [ ] **Step 2: Run the focused wiring test and verify the missing shared recovery section is red.**

  Run: `bash tests/test_skill_facade_wiring.sh`

  Expected: nonzero; the new recovery-section assertions report `NOT OK` before the convention is changed.

- [ ] **Step 3: Write the canonical convention rule.**

  Add one clearly headed section immediately after the Step-0 preamble’s command/verdict rules. State that, when a required `docket.sh` facade command or direct Git command fails with sandbox/permission-denial evidence, the agent retries the **exact command once** via the host harness’s native approval/escalation mechanism. State explicitly that the retry does not change arguments, use `sudo`, or broaden the session sandbox; if the capability is unavailable, denied, or the retry fails, the caller follows its existing failure posture with the preserved diagnostic. Clarify that an ordinary Git failure does not qualify and that Step 0 retries the outer `docket.sh preflight`, never a private inner fetch.

- [ ] **Step 4: Update the resolver contract to make only proven claims.**

  In `scripts/docket-config.md`, replace every Stage-1/exit-table/invariant statement that labels all fetch failures as an unreachable origin or tells users to check the network. Document instead: the resolver reports `git fetch origin failed`, preserves Git stderr verbatim after its neutral wrapper, emits no config output, and keys fail-closed behavior on the fetch return code before cached references are read. Retain the separate, specific `set-head` failure description.

- [ ] **Step 5: Run focused tests and mutation-test the shared guard.**

  Run:

  ```bash
  bash tests/test_skill_facade_wiring.sh
  bash tests/test_docket_config.sh
  ```

  Expected: both exit 0. Then, in an isolated temporary copy, delete the recovery subsection and rerun `bash tests/test_skill_facade_wiring.sh`; it must exit nonzero because the heading-count/nonempty/structural assertions fail. Restore the unmodified feature worktree afterward.

- [ ] **Step 6: Commit the workflow contract, resolver documentation, and guard.**

  ```bash
  git add skills/docket-convention/SKILL.md scripts/docket-config.md tests/test_skill_facade_wiring.sh
  git commit -m "docs: define sandbox retry boundary"
  ```

### Task 3: Verify the integrated branch and record the manual receipt

**Files:**

- Create: `docs/results/2026-07-22-truthful-git-errors-harness-neutral-escalation-retry-results.md`
- Test: every `tests/test_*.sh` file, derived by the shell glob rather than a hand-maintained list.

**Interfaces:**

- Consumes: the completed resolver and convention changes.
- Produces: a full-suite receipt plus the one manual, host-mediated merge-gate check required by the specification.

- [ ] **Step 1: Run the complete repository suite.**

  Run from the feature worktree:

  ```bash
  suite_status=0
  for test_file in tests/test_*.sh; do
    bash "$test_file" || suite_status=1
  done
  exit "$suite_status"
  ```

  Expected: exit 0. Capture any failing test name before changing code; do not weaken a test to make the suite pass.

- [ ] **Step 2: Perform the manual, permission-scoped acceptance check.**

  Under Codex `workspace-write` with network enabled, run the exact canonical preflight normally. If it fails with the preserved `.git` lock/permission evidence, request the host’s scoped approval for the identical `docket.sh preflight` command once and rerun it unchanged.

  Expected: the ordinary attempt reports the neutral wrapper plus the real denial; the one approved retry reaches `BOOTSTRAP=PROCEED`. If the first attempt is already permitted by the current harness state, record that no denial was reproducible and do not manufacture a failure.

- [ ] **Step 3: Write the results receipt.**

  Create the results file from `skills/docket-implement-next/results-template.md`. Record the focused tests, both mutation outcomes, full-suite status, the exact manual acceptance outcome (including whether approval was needed), no new ADR, and no auto-captured follow-up. Do not copy credentials, environment dumps, or full remote diagnostics into the receipt.

- [ ] **Step 4: Commit the results receipt.**

  ```bash
  git add docs/results/2026-07-22-truthful-git-errors-harness-neutral-escalation-retry-results.md
  git commit -m "docs: record change 128 verification"
  ```

## Self-Review

- **Spec coverage:** Task 1 implements and proves the deterministic stderr-preservation contract; Task 2 supplies the single inherited harness-neutral retry rule and updated resolver contract; Task 3 runs the required full suite and records the interactive acceptance receipt. The plan explicitly excludes shell elevation, broad approvals, and retrying ordinary Git failures.
- **Placeholder scan:** Complete; every test, mutation, implementation shape, and command is spelled out.
- **Interface consistency:** Task 1 exports only stderr/exit behavior at Stage 1; Task 2 consumes the caller-level failure and never changes script-level escalation; Task 3 validates those two contracts without adding a new runtime interface.
