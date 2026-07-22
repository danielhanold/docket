# Configured Bash Runtime Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Discover, validate, persist, and consistently use a configured Bash 4+ interpreter for every Docket-owned shell execution path.

**Architecture:** `runtime.bash` is a machine-local configuration value resolved by `scripts/docket-config.sh` and emitted as `DOCKET_BASH_PATH`. A small POSIX bootstrap layer keeps installation and the public facade runnable before Bash 4 is available, while all Bash-specific helpers and automatically detected test scripts run through the validated path. Explicit repository `finalize.test_command` remains untouched except for receiving the environment variable.

**Tech Stack:** POSIX shell bootstrap, Bash 4+, existing shell-test harnesses, `jq`, `awk`, `sed`, Git.

## Global Constraints

- Accept only an absolute, executable Bash path whose major version is at least 4; missing, relative, non-executable, or legacy paths fail closed with an actionable remedy.
- `runtime.bash` is machine-local: global `config.yml` is its normal home and `.docket.local.yml` may override it; committed `.docket.yml` must warn and ignore it.
- Preserve a valid user-selected runtime and never silently replace an invalid explicit value.
- All Docket-owned helper/test execution uses `DOCKET_BASH_PATH`; do not rewrite arbitrary repository commands supplied as `finalize.test_command`.
- Tests that touch global configuration or profiles must pin `HOME`/`XDG_CONFIG_HOME` to a sandbox and mutation-test new guards.

---

### Task 1: Resolve and validate the machine-local runtime setting

**Files:**
- Modify: `scripts/docket-config.sh`
- Modify: `scripts/docket-config.md`
- Modify: `.docket.example.yml`
- Modify: `README.md`
- Modify: `tests/test_docket_config.sh`
- Modify: `tests/test_docket_example_yml.sh`

**Interfaces:**
- Produces: `DOCKET_BASH_PATH=<absolute executable Bash 4+ path>` in both resolver output formats, adjacent to the other resolved runtime values.
- Consumes: block-scoped `runtime:` values from `.docket.local.yml` then global `config.yml`; a repo-committed value is ignored with a diagnostic.

- [ ] **Step 1: Add failing hermetic resolver cases.**

  Extend `tests/test_docket_config.sh` with fixture configurations that prove local overrides global, global resolves when local is absent, and a committed `runtime:` entry is warned-and-ignored. Use distinct executable fake Bash scripts that print `5.2.0` for `--version`, plus negative fixtures for a relative path, a missing file, a non-executable file, and a fake `3.2.57` runtime. Assert an otherwise valid export contains the exact `DOCKET_BASH_PATH=<fixture path>` line; clear any captured value before each abort case.

- [ ] **Step 2: Run the new resolver tests and verify they fail.**

  Run: `bash tests/test_docket_config.sh`

  Expected: the new runtime assertions fail because `DOCKET_BASH_PATH` is not resolved or emitted.

- [ ] **Step 3: Implement nested runtime resolution and validation.**

  In `scripts/docket-config.sh`, reuse `yaml_block_body` to read only the `bash:` child of a `runtime:` block from local/global sources. Add a dedicated runtime resolver that rejects a committed value with the ADR-0019-style warning, validates `[[ $path = /* ]]`, `[[ -x $path ]]`, and reads the major version by invoking `"$path" --version`; reject any nonnumeric or `< 4` major. Emit `DOCKET_BASH_PATH` in the stable export sequence. Do not accept a bare `bash` command or depend on PATH for the resolved value.

- [ ] **Step 4: Document the knob end to end.**

  Add a commented `runtime:` sample to `.docket.example.yml`, document its local-only scope, validation, discovery, and `DOCKET_BASH_PATH` export in `scripts/docket-config.md` and the install/configuration sections of `README.md`. Update `tests/test_docket_example_yml.sh` so the example-to-resolver mapping checks the `runtime.bash` nested shape rather than a bare `bash:` leaf.

- [ ] **Step 5: Run focused tests and mutation-check the fence.**

  Run: `bash tests/test_docket_config.sh && bash tests/test_docket_example_yml.sh`

  Expected: PASS. Temporarily remove the committed-layer ignore branch and confirm the committed-runtime fixture becomes red; restore it before committing.

- [ ] **Step 6: Commit the resolver slice.**

  ```bash
  git add scripts/docket-config.sh scripts/docket-config.md .docket.example.yml README.md tests/test_docket_config.sh tests/test_docket_example_yml.sh
  git commit -m "feat: resolve configured Bash runtime"
  ```

### Task 2: Discover and persist Bash safely during installation

**Files:**
- Modify: `install.sh`
- Modify: `scripts/ensure-global-config.sh`
- Modify: `scripts/ensure-docket-env.sh`
- Modify: `scripts/ensure-global-config.md`
- Modify: `scripts/ensure-docket-env.md`
- Modify: `tests/test_install.sh`
- Modify: `tests/test_ensure_global_config.sh`
- Modify: `tests/test_ensure_docket_env.sh`
- Create: `tests/test_bash_runtime_install.sh`

**Interfaces:**
- Produces: a managed `runtime:` / `bash:` block in the global config and a profile/harness `DOCKET_BASH_PATH` binding.
- Consumes: deterministic candidates: Homebrew prefix result, `/opt/homebrew/bin/bash`, `/usr/local/bin/bash`, then an absolute PATH-resolved Bash.

- [ ] **Step 1: Write discovery and persistence tests.**

  In a new hermetic test, put fake Bash executables and a fake `brew` ahead of PATH. Cover Homebrew discovery, standard-location fallback, PATH fallback, a legacy-only outcome, and no candidate. Verify only a version-4+ candidate is chosen. Add config fixtures proving a valid existing explicit setting is preserved, an invalid existing setting stops installation, and unrelated user config remains byte-preserved. Extend the environment tests to assert each profile flavor and Claude settings JSON receive `DOCKET_BASH_PATH` alongside `DOCKET_SCRIPTS_DIR`.

- [ ] **Step 2: Run installer tests and verify they fail.**

  Run: `bash tests/test_bash_runtime_install.sh && bash tests/test_install.sh && bash tests/test_ensure_docket_env.sh`

  Expected: FAIL because the installer neither discovers nor persists a runtime.

- [ ] **Step 3: Implement a POSIX-safe discovery/bootstrap helper.**

  Keep the installation entry point runnable under macOS `/bin/bash` 3.2 by placing discovery before Bash-4-only operations and using POSIX syntax there. Deduplicate candidates in deterministic order, require absolute executable paths, ask each candidate only for its version, and print the documented `brew install bash` remedy when none qualifies. Implement managed-block rewriting only after validating marker balance/order; preserve user content and an explicit valid runtime. Use an atomic same-directory temporary file plus rename for config/profile writes.

- [ ] **Step 4: Bind the discovered runtime to the environment.**

  Generalize `scripts/ensure-docket-env.sh`'s managed export/settings update so it writes both runtime bindings from validated values. Preserve the existing clone-move replacement and settings JSON behavior. Ensure `install.sh` sequences discovery/config persistence before scripts that need `DOCKET_BASH_PATH`.

- [ ] **Step 5: Run tests and mutation-check persistence.**

  Run: `bash tests/test_bash_runtime_install.sh && bash tests/test_ensure_global_config.sh && bash tests/test_ensure_docket_env.sh && bash tests/test_install.sh`

  Expected: PASS. Temporarily remove the version-floor condition and confirm the Bash-3.2 case fails; temporarily replace the read-modify-write config operation with a blind write and confirm the preserved-user-config case fails. Restore both changes.

- [ ] **Step 6: Commit the installer slice.**

  ```bash
  git add install.sh scripts/ensure-global-config.sh scripts/ensure-docket-env.sh scripts/ensure-global-config.md scripts/ensure-docket-env.md tests/test_bash_runtime_install.sh tests/test_install.sh tests/test_ensure_global_config.sh tests/test_ensure_docket_env.sh
  git commit -m "feat: install configured Bash runtime"
  ```

### Task 3: Route Docket-owned execution through the configured interpreter

**Files:**
- Modify: `scripts/docket.sh`
- Modify: `scripts/docket.md`
- Modify: `scripts/lib/docket-preflight.sh`
- Modify: every executable Docket-owned launcher that invokes a helper with bare `bash`
- Modify: `tests/test_docket_facade.sh`
- Create: `tests/test_bash_runtime_routing.sh`

**Interfaces:**
- Consumes: validated `DOCKET_BASH_PATH` from bootstrap/environment.
- Produces: facade/helper processes started through that exact path; unsupported direct invocation fails before Bash-4-specific work.

- [ ] **Step 1: Add a routing witness.**

  Create a fake configured Bash that records `"$@"` then delegates to a real modern Bash. In `tests/test_bash_runtime_routing.sh`, set PATH so `bash` resolves to a distinct fake and assert facade/preflight/helper launches record the configured path, never the PATH-selected path. Include an unsupported/missing configured path case and assert it emits the same actionable remediation and exits nonzero.

- [ ] **Step 2: Run routing tests and verify they fail.**

  Run: `bash tests/test_bash_runtime_routing.sh && bash tests/test_docket_facade.sh`

  Expected: FAIL because `scripts/docket.sh` currently runs directly under its shebang and helper calls can inherit PATH-selected Bash.

- [ ] **Step 3: Add the explicit interpreter boundary.**

  Make the public facade a POSIX bootstrap that validates/uses `DOCKET_BASH_PATH` and `exec`s its Bash implementation with an internal, non-user-controlled marker to avoid recursion. Update Docket-owned launchers to use the canonical configured interpreter rather than a bare `bash`, preserving argv and exit status. Keep the facade operation inventory closed: no generic runtime or shell execution operation is added.

- [ ] **Step 4: Keep the facade contract and guards in lockstep.**

  Update `scripts/docket.md` only for the bootstrap/runtime contract; retain the documented op set. Extend `tests/test_docket_facade.sh` fixtures with a real-shape configured interpreter and assert the routed implementation still preserves unknown-op rejection, raw config output, and helper exit-code passthrough.

- [ ] **Step 5: Run focused tests and mutation-check the route.**

  Run: `bash tests/test_bash_runtime_routing.sh && bash tests/test_docket_facade.sh && bash tests/test_skill_facade_wiring.sh`

  Expected: PASS. Temporarily change one routed launch back to bare `bash` and confirm the fake-runtime log assertion becomes red; restore the explicit route.

- [ ] **Step 6: Commit the routing slice.**

  ```bash
  git add scripts/docket.sh scripts/docket.md scripts/lib/docket-preflight.sh scripts tests/test_bash_runtime_routing.sh tests/test_docket_facade.sh
  git commit -m "feat: route docket helpers through configured Bash"
  ```

### Task 4: Use the runtime for auto-detected finalize tests without rewriting user commands

**Files:**
- Modify: `skills/docket-finalize-change/SKILL.md`
- Modify: `tests/test_readme_finalize_docs.sh`
- Create: `tests/test_configured_bash_finalize.sh`
- Modify: any existing finalize workflow/doc guard that asserts suite invocation behavior

**Interfaces:**
- Consumes: `DOCKET_BASH_PATH` and `FINALIZE_TEST_COMMAND` from the preflight block.
- Produces: `"$DOCKET_BASH_PATH" "$test"` for auto-detected `tests/test_*.sh`; explicit `FINALIZE_TEST_COMMAND` runs verbatim with `DOCKET_BASH_PATH` available in its environment.

- [ ] **Step 1: Write finalize-boundary tests.**

  Add a harness fixture with two `tests/test_*.sh` files and a fake configured Bash that logs both paths. Assert auto-detection executes each through the configured runtime. Add an explicit command fixture that records its raw argv/environment and proves the command text is neither prefixed nor rewritten while `DOCKET_BASH_PATH` is exported.

- [ ] **Step 2: Run the new tests and verify they fail.**

  Run: `bash tests/test_configured_bash_finalize.sh`

  Expected: FAIL because the current finalize workflow does not prescribe the configured runtime for the auto-detected shell suite.

- [ ] **Step 3: Update the finalize gate contract.**

  In `skills/docket-finalize-change/SKILL.md`, make suite detection explicitly route shell-test files through `"$DOCKET_BASH_PATH" "$test"`. State that the configured runtime is supplied in the environment for an explicit `finalize.test_command`, whose command text is executed unchanged. Preserve the existing gate, rebase, review, and no-merge boundaries.

- [ ] **Step 4: Prove both sides and run the relevant documentation guard.**

  Run: `bash tests/test_configured_bash_finalize.sh && bash tests/test_readme_finalize_docs.sh`

  Expected: PASS. Mutate the auto-detected execution to PATH `bash` and confirm its routing assertion fails; mutate the explicit-command path to prepend an interpreter and confirm the verbatim-argv assertion fails. Restore both mutations.

- [ ] **Step 5: Commit the finalize slice.**

  ```bash
  git add skills/docket-finalize-change/SKILL.md tests/test_configured_bash_finalize.sh tests/test_readme_finalize_docs.sh
  git commit -m "feat: run detected shell tests with configured Bash"
  ```

### Task 5: Validate the integrated runtime contract

**Files:**
- Modify: only files required by failures found in this task

**Interfaces:**
- Verifies: installation discovery, resolver emission, facade/helper routing, documented configuration, and finalize test behavior as one contract.

- [ ] **Step 1: Run the focused runtime battery from a clean feature worktree.**

  Run: `bash tests/test_docket_config.sh && bash tests/test_docket_example_yml.sh && bash tests/test_bash_runtime_install.sh && bash tests/test_ensure_global_config.sh && bash tests/test_ensure_docket_env.sh && bash tests/test_install.sh && bash tests/test_bash_runtime_routing.sh && bash tests/test_docket_facade.sh && bash tests/test_configured_bash_finalize.sh`

  Expected: PASS with no writes outside test-owned temporary `HOME`/`XDG_CONFIG_HOME` roots.

- [ ] **Step 2: Run the entire suite through the discovered configured interpreter.**

  Run: `"$DOCKET_BASH_PATH" tests/run-all.sh`

  Expected: PASS. If this repository uses a different whole-suite runner discovered from current files, run that runner through `"$DOCKET_BASH_PATH"` and record its exact command/output in the results artifact.

- [ ] **Step 3: Review diff scope and commit any integration corrections.**

  Run: `git diff --check origin/main...HEAD && git status --short`

  Expected: no whitespace errors and only planned code, tests, and documentation changes. Commit any required integration correction with a focused message.

## Self-review

- Spec coverage: Task 1 implements machine-local resolution, validation, export, fence, and docs; Task 2 covers discovery, persistence, profile/harness binding, and failure remedy; Task 3 enforces the execution boundary; Task 4 protects auto-detected versus explicit finalize behavior; Task 5 runs the required full-suite gate.
- Placeholder scan: no deferred or unspecified behavior remains; every task names files, commands, expected result, and test shape.
- Interface consistency: `runtime.bash` resolves to `DOCKET_BASH_PATH` in Task 1; Tasks 2–4 consume that exact variable and do not introduce a second runtime name.
