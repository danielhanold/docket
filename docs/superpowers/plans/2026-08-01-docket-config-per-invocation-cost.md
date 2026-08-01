<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0176 — docket-config.sh costs ~0.87s per invocation and dominates test_docket_config.sh](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-01-0176-docket-config-sh-costs-0-87s-per-invocation-and-dominates-te.md)**
<!-- docket:backlink:end -->

# docket-config.sh per-invocation cost Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `docket-build` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make real `docket-config.sh` resolver runs at least twice as fast by replacing repeated file-backed YAML scans with one immutable, in-memory snapshot per config layer.

**Architecture:** Load committed, machine-local, and global configuration files into Bash indexed arrays after their existing readability policy has run. Replace `yaml_get` and `yaml_block_body` plus their temporary block files with fork-free scalar, block-leaf, and block-key readers over those snapshots; preserve the separate `runtime.bash` file reader and all resolution/validation code. Add behavioral mutation coverage and a PATH-shim command-count guard so the suite proves both compatibility and the performance mechanism.

**Tech Stack:** GNU Bash 4+, repository shell-test harness, Git fixture repositories.

## Global Constraints

- Preserve the documented YAML subset, layer precedence, export order/quoting, warnings, diagnostics, bootstrap probes, and exit codes byte-for-byte for stable input.
- Keep `runtime.bash` on `scripts/lib/docket-runtime.sh`'s existing file-backed reader; do not fold it into the snapshot parser.
- Use `[[:space:]]`, not literal-space indentation classes, and retain first-match behavior for flat keys and repeated blocks.
- Do not add an external YAML parser, cache across resolver invocations, or remove Git freshness work.
- The command-count test must derive external command execution from the runtime PATH/shim behavior; it may not allowlist parser programs.
- Each task must leave one focused, reviewable commit. Do not tick plan checkboxes.

---

### Task 1: Snapshot-backed scalar and nested-block resolution

**Build profile:** standard

**Files:**

- Modify: `scripts/docket-config.sh`
- Modify: `tests/test_docket_config.sh`

**Interfaces:**

- Consumes: normalized `CFG`, `GCFG`, and `LCFG` source paths after their existing missing/unreadable policy.
- Produces: `config_layer_load <slot> <file>`, `config_scalar_get <slot> <key>`, `config_block_get <slot> <block> <leaf>`, and `config_block_keys <slot> <block>` private Bash helpers used by existing resolution assignments.

- [ ] **Step 1: Add failing compatibility fixtures for the in-memory reader boundaries.**

  Add cases near the current nested-map tests that create committed/local/global layers and prove all of the following through resolver output: a tab-indented `skills:` leaf resolves; a bare same-named top-level leaf does not shadow `skills:`; the first duplicate scalar/leaf wins; a repeated block returns the first matching leaf; and a block ends at the next non-indented content line. Include a control assertion before each mutation-sensitive assertion so an absent fixture cannot pass vacuously.

  ```bash
  cat > "$tmp/snapshot/.docket.yml" <<'EOF'
  metadata_branch: main
  skills:
  \tbuild: auto
  build: wrong-top-level-value
  skills:
    build: wrong-later-block
  EOF
  ct_commit "$tmp/snapshot"
  snapshot_out="$(rung "$tmp/snapshot.xdg" "$tmp/snapshot" --export)"
  assert "snapshot reader keeps the first block leaf" \
    'grep -qxF "SKILL_BUILD=auto" <<<"$snapshot_out"'
  ```

- [ ] **Step 2: Run the targeted tests to establish behavioral equivalence before the refactor.**

  Run: `bash tests/test_docket_config.sh`

  Expected: PASS. These cases pin existing resolver behavior, so they must be green both before and after the implementation swap; their mutation tests in Step 4 prove the assertions are not decorative.

- [ ] **Step 3: Implement immutable layer snapshots and route general readers through them.**

  Replace the `yaml_get`/`yaml_block_body` pipeline helpers with Bash built-ins over three eagerly-loaded indexed arrays. Keep the arrays keyed by fixed slot names rather than user text. Implement scalar normalization once (first `#`, trailing whitespace, one surrounding quote pair), use literal string comparison for keys, and return the first matching line. Implement block traversal by entering only on a column-zero empty `<block>:` header, leaving at the next non-indented non-comment line, and scanning child lines with the same scalar helper. Replace temporary `SKILLS_BLK`, `LEARN_BLK`, `RECLAIM_BLK`, `BUILD_BLK`, and `AUTO_CAPTURE` block bodies with direct slot/block/leaf reads and a direct block-key iterator for unknown `skills` warnings.

  ```bash
  config_scalar_get(){ # config_scalar_get <slot> <key>
    local line body key="$2"
    for line in "${CONFIG_LINES_$1[@]}"; do
      body="${line%%#*}"
      [[ "$body" =~ ^[[:space:]]*"$key"[[:space:]]*:[[:space:]]*(.*)$ ]] || continue
      config_normalize_scalar "${BASH_REMATCH[1]}"
      return 0
    done
    return 0
  }
  ```

  Use Bash 4-compatible indirect/nameref-free storage if needed; never evaluate configuration text. Leave `runtime_get` and `runtime_count` file-backed.

- [ ] **Step 4: Run the focused resolver suite and mutation-test each new boundary.**

  Run: `bash tests/test_docket_config.sh`

  Expected: PASS. Then temporarily (without committing) bypass loading one layer, return the last rather than first scalar/leaf, and remove the block-exit condition one at a time; each corresponding new assertion must fail. Restore each mutation before continuing.

- [ ] **Step 5: Commit the snapshot reader implementation and behavioral tests.**

  ```bash
  git add scripts/docket-config.sh tests/test_docket_config.sh
  git commit -m "perf: snapshot docket config layers"
  ```

### Task 2: Spawn-count performance guard and resolver contract documentation

**Build profile:** standard

**Files:**

- Modify: `tests/test_docket_config.sh`
- Modify: `scripts/docket-config.md`

**Interfaces:**

- Consumes: Task 1's snapshot-backed resolver and the test file's reusable local-origin fixture helpers.
- Produces: a hermetic PATH-shim count of external command executions for one representative resolver run, with a ceiling of 120, and documented one-snapshot semantics.

- [ ] **Step 1: Add a failing command-count test that observes execution rather than source spellings.**

  Create a temporary shim directory containing executable wrappers for command names found from the real resolver run. Each wrapper appends its own invoked command name to a log and delegates to the saved absolute executable. Derive the wrapper set from the command words actually resolved through PATH during fixture setup/run; do not hard-code `sed`, `awk`, `head`, or parser names. Run one representative `rung ... --export` invocation with the shim directory prepended to `PATH`, assert the fixture still emits a known resolved export, and count log lines.

  ```bash
  command_count="$(wc -l < "$spawn_log" | tr -d '[:space:]')"
  assert "0176 spawn guard measured a non-empty command population" \
    '[ "$command_count" -gt 0 ]'
  assert "0176 snapshot resolver stays under the spawned-command ceiling" \
    '[ "$command_count" -le 120 ]'
  ```

  Make the test fail before the final optimization is complete by retaining the 120 ceiling; do not weaken the ceiling to match an intermediate count.

- [ ] **Step 2: Document the stable snapshot contract.**

  In `scripts/docket-config.md`, update the resolution/read behavior to state that each committed, local, and global config layer is loaded once per resolver invocation after existing path/readability checks; a single run observes an immutable per-layer snapshot. Document that this changes neither supported syntax nor the separate `runtime.bash` reader, and does not remove the authoritative Git probes.

- [ ] **Step 3: Run the focused test suite and mutation-test the guard.**

  Run: `bash tests/test_docket_config.sh`

  Expected: PASS. Then temporarily restore one external scan in a repeated general-reader path (or bypass snapshot use) and prove the command-count assertion turns red; restore the optimized implementation. Also temporarily loosen the block boundary and prove Task 1's boundary assertion turns red.

- [ ] **Step 4: Commit the performance guard and documentation.**

  ```bash
  git add tests/test_docket_config.sh scripts/docket-config.md
  git commit -m "test: guard docket config resolver spawn cost"
  ```

### Task 3: Measure acceptance and run the whole build gate

**Build profile:** economy

**Files:**

- Modify: no source files unless the measured acceptance test identifies a defect; amend the focused fix into a new task commit if needed.

**Interfaces:**

- Consumes: Task 1's reader and Task 2's command-count guard.
- Produces: recorded before/after median measurements for results close-out and a green full repository suite.

- [ ] **Step 1: Measure 20 warmed resolver runs using one hermetic local-origin fixture.**

  Use the fixture helpers from `tests/test_docket_config.sh` or an equivalent temporary fixture to run `bash scripts/docket-config.sh --repo-dir <fixture> --export` 20 times after one warm-up. Record the median duration and compare it to the pre-change baseline captured from `origin/main`; acceptance requires at least a 2× reduction.

  ```bash
  for run in $(seq 1 20); do
    /usr/bin/time -p bash "$SCRIPT" --repo-dir "$fixture" --export >/dev/null 2>>"$timings"
  done
  sort -n "$timings" | sed -n '10p'
  ```

- [ ] **Step 2: Run the whole repository suite.**

  Run: `bash scripts/test-all.sh`

  Expected: PASS. If the project’s published suite entry point differs, use finalize’s auto-detected equivalent and record the exact command/result.

- [ ] **Step 3: Commit only any necessary acceptance-fix changes.**

  If measurement and the full suite pass without source/test changes, make no empty commit. If a defect is fixed, commit its focused regression test and fix together:

  ```bash
  git add scripts/docket-config.sh tests/test_docket_config.sh scripts/docket-config.md
  git commit -m "fix: preserve docket config snapshot compatibility"
  ```

## Self-Review

- Spec coverage: Task 1 covers one-load snapshots, literal/first-match scalar and block behavior, all resolution consumers, and preserves the runtime reader. Task 2 supplies the required runtime-derived spawn ceiling and documents snapshot consistency. Task 3 supplies the 20-run median, 2× gate, and full-suite evidence.
- Placeholder scan: no deferred implementation or generic test steps remain; each task names files, interfaces, commands, and observable expected outcomes.
- Type consistency: all tasks use the same private snapshot helper vocabulary and preserve existing public resolver output.
