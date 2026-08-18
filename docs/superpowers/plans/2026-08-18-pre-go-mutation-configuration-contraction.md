<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0326 — Pre-Go mutation configuration contraction](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0326-pre-go-mutation-configuration-contraction.md)**
<!-- docket:backlink:end -->

# Pre-Go mutation configuration contraction — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Contract the migration repository's committed config so Go v1's capability fence permits mutation, and prove the four-layer resolution with fixtures — without touching `internal/config`.

**Architecture:** Turn the three deferred switches in committed `.docket.yml` explicitly `false`, then add a config fixture test that reproduces the migration host's four-layer state and proves the classifier's behavior: pre-change blocks, post-change (global agent pins kept, repo-layer pins removed, auto_capture off) allows mutation, and one-at-a-time negatives still fail closed. The Go classifier is already layer-aware (`internal/config/capability.go` `dispAgentsLeaf` + `isRepositoryLayer`): global agent pins are supported, repository/repository-local agent pins are deferred. This change writes fixtures, not machinery.

**Tech Stack:** committed `.docket.yml` (YAML), Go `testing` over the existing `internal/app.DiagnosticConfig` API.

**Spec:** `docs/superpowers/specs/2026-08-18-pre-go-mutation-configuration-contraction-design.md` (on the `docket` metadata branch)

## Global Constraints

- **Do NOT modify the `internal/config` classifier/schema/resolver LOGIC** or reclassify any capability, and do NOT add a migration override or ignore `.docket.local.yml` (spec "Explicit exclusions"). Note the scope boundary: contracting `.docket.yml` legitimately trips the drift guard `TestFixtureDocketSelf` (`internal/config/fixtures_test.go`), whose OWN remedy message mandates cutting a new versioned fixture tree + re-deriving its expectations. That fixture re-baseline is **sanctioned maintenance the spec's green-suite requirement forces**, not a logic change — it is authorized here (human decision, 2026-08-18). It does not weaken the fence: the guard still fires on future drift.
- **Do NOT weaken or delete any capability-fence test.** The drift-guard re-baseline (Task 1b) re-derives `TestFixtureDocketSelf`'s expectations against the new frozen state; it must keep the guard live (still byte-compares, still asserts the remaining blocker set + layers). Touch no other `internal/config` test.
- **The tracked diff changes ONLY the three owned committed switches and their explanatory comments** — preserve every other `.docket.yml` key and comment; do not normalize or reorder the file (spec "Preservation and failure rules").
- **Set the three booleans explicitly to `false`** (visible decision; robust to a future built-in default move) — never delete the keys.
- **Global model/effort pins are supported and untouched.** Only repository and repository-local agent pins block.
- The machine-local `.docket.local.yml` edit and the real `diagnostic config --for-mutation` run are **operator/verification steps recorded in results**, not code in this PR.
- **Suite gate:** the whole resolved suite (`scripts/run-tests.sh`) stays green.

---

## Task 1: Contract `.docket.yml` + prove it with a four-layer fixture test

**Files:**
- Modify: `.docket.yml` (repo root) — the three switches only.
- Modify: `internal/app/config_test.go` (add the migration-host fixture test).
- Reference (do not modify): `internal/config/capability.go` (`dispAgentsLeaf`, `isRepositoryLayer`), `internal/app/config.go` (`DiagnosticConfig`, `Result`, `MutationAllowed`), the existing `TestPreflightUnsupported` / `TestPreflightAllowedResult` for the `config.Source{Layer,Name,Data}` + `DiagnosticConfig(sources, mainCtx(), true)` pattern and the `CodeDeferredCapRequested` blocker-path extraction.

**Interfaces:**
- Consumes: `internal/app.DiagnosticConfig(sources []config.Source, ctx, forMutation bool) Result`; `Result.MutationAllowed`, `Result.Diagnostics` (each `.Code`, `.Path`), `config.CodeDeferredCapRequested`; `config.LayerGlobal`, `config.LayerRepository`, `config.LayerRepositoryLocal`.
- Produces: a green migration-host contraction test + the contracted `.docket.yml`.

- [ ] **Step 1: Write the failing test** — add `TestMigrationHostContraction` to `internal/app/config_test.go`. Build the four-layer migration-host state with `config.Source` structs (mirror the real layers; do NOT read the machine's real files):
  - `LayerGlobal` (`config.yml`): `agents:\n  claude:\n    implement-next: { model: m, effort: low }\n` (a supported global pin) — MUST remain non-blocking.
  - `LayerRepository` (`.docket.yml`): the pre-change switches `terminal_publish: true`, `finalize:\n  skip_results_only_delta: true`, `build:\n  checkpoint: true`.
  - `LayerRepositoryLocal` (`.docket.local.yml`): `auto_capture:\n  enabled: true` **and** a repo-local `agents:\n  claude:\n    build-standard: { model: m, effort: medium }` pin.

  Assertions (three sub-cases; use `DiagnosticConfig(src, mainCtx(), true)`):
  1. **pre-change** → `MutationAllowed == false`; the deferred-blocker paths (diagnostics with `Code == config.CodeDeferredCapRequested`) include `build.checkpoint`, `finalize.skip_results_only_delta`, `terminal_publish`, `auto_capture.enabled`, **and** the repo-local `agents.claude.build-standard.*` pin path — and do **NOT** include the global `agents.claude.implement-next.*` pin (assert the global pin is absent from blockers).
  2. **post-change** → committed switches all `false`, the `LayerRepositoryLocal` source dropped to nothing (no auto_capture, no agents), the global agents pin retained → `MutationAllowed == true`, zero `CodeDeferredCapRequested` diagnostics.
  3. **one-at-a-time negatives** → from the post-change state, re-activate exactly one blocker per sub-case (`build.checkpoint: true`; `finalize.skip_results_only_delta: true`; `terminal_publish: true`; repo-local `auto_capture.enabled: true`; a repo-local `agents.*` pin) → each asserts `MutationAllowed == false`. Table-drive so each is independent and non-vacuous.

- [ ] **Step 2: Run it, verify it fails** — `go test ./internal/app/ -run TestMigrationHostContraction -v`. Before the `.docket.yml` edit this test is self-contained (it builds synthetic sources), so it should pass on the classifier already IF your fixtures are correct — the RED you must first confirm is a deliberately-wrong assertion (e.g. assert the global pin IS a blocker), see it fail, then correct it. This proves the layer-awareness claim is real, not assumed. (If sub-case 1 or 2 fails for a real reason — e.g. the classifier flags the global pin — STOP and report: that contradicts the spec's premise and would need `internal/config`, which is out of scope.)

- [ ] **Step 3: Contract `.docket.yml`** — set exactly these three, preserving surrounding comments and order:
  - `terminal_publish: true` → `terminal_publish: false`
  - under `finalize:`, `skip_results_only_delta: true` → `skip_results_only_delta: false`
  - under `build:`, `checkpoint: true` → `checkpoint: false`
  Update only the adjacent explanatory comment if one asserts the value; touch nothing else. Confirm `git diff .docket.yml` shows only these three value flips (+ any comment on those lines).

- [ ] **Step 4: Run tests green** — `go test ./internal/app/... ./internal/config/...` all green; `go vet ./...` and `go build ./...` clean. Confirm `TestPreflightUnsupported` and the other existing fence tests still pass unchanged.

- [ ] **Step 5: Commit** — `chore(0326): contract .docket.yml deferred switches; prove four-layer mutation envelope`.

---

## Task 1b: Re-baseline the `docket-self` drift guard for the contracted config

Contracting `.docket.yml` (Task 1) reds `internal/config`'s `TestFixtureDocketSelf` — a drift guard that byte-compares the live `.docket.yml` against a frozen copy at `testdata/repositories/v0.9.2/docket-self/repo/.docket.yml`. The guard's own failure remedy prescribes the fix; follow it exactly.

**Files:**
- Create: a new versioned fixture tree `testdata/repositories/<new-version>/docket-self/…` (mirror the existing `v0.9.2/docket-self/` layout: `repo/.docket.yml`, `repo/` peers if any, and `xdg/docket/config.yml`) with `repo/.docket.yml` = the **contracted** file, plus a `PROVENANCE.md` for the new tree following the existing `v0.9.2`/`v0.9.3` `PROVENANCE.md` shape (record the current source commit + date).
- Modify: `internal/config/fixtures_test.go` — re-point `TestFixtureDocketSelf` (and only that test) to the new versioned `docket-self` tree, and re-derive its expectations.
- Reference (do NOT modify): the classifier/schema/resolver source; `testdata/README.md` (the frozen-fixture immutability + new-versioned-tree protocol).

**Interfaces:**
- Consumes: `assertFrozenCopyMatchesLive`, `mustResolveFixture`, `blockerPaths`, `PreflightMutation` (all existing in `fixtures_test.go`).

- [ ] **Step 1: Determine the new version + cut the tree** — read `testdata/README.md` for the versioning convention and the two existing `PROVENANCE.md` files for the shape. Cut a new versioned tree holding the `docket-self` fixture: copy `v0.9.2/docket-self/` verbatim, then overwrite `repo/.docket.yml` with the CONTRACTED file (byte-identical to the live `.docket.yml` after Task 1). Change NOTHING else in the copied tree — in particular leave `xdg/docket/config.yml` exactly as frozen (its `auto_capture.enabled` is not this change's to touch). Add `PROVENANCE.md`.
- [ ] **Step 2: Re-point + re-derive** — update `TestFixtureDocketSelf` to read the frozen `.docket.yml` from the NEW tree (leave every other fixture test on `v0.9.2`). Re-derive its expectations by **running the resolver and observing the actual output**, never by guessing: the committed-layer blockers (`build.checkpoint`, `finalize.skip_results_only_delta`, `terminal_publish`) drop out; whatever remains (e.g. a still-active `auto_capture.enabled` from the fixture's global layer, if present) stays. Update the `blockerPaths` assertion, the per-blocker `wantLayer` map, `MutationAllowed`/`decision.Allowed`, and `finalize.test_command`/`metadata_branch` assertions to match the resolver's actual result for the new fixture. If the re-derived result is `MutationAllowed == true` (no blockers remain), assert that; if a global blocker remains, assert the reduced set — either is correct, whatever the resolver actually produces.
- [ ] **Step 3: Run** — `go test ./internal/config/... -run TestFixtureDocketSelf -v` green, then the whole `internal/config` package green, then `go build ./...` / `go vet ./...` clean. Confirm the guard is still LIVE: mutate the new frozen `.docket.yml` by one byte, watch `TestFixtureDocketSelf` redden, revert.
- [ ] **Step 4: Commit** — `test(0326): re-baseline docket-self drift fixture to the contracted config`.

---

## Task 2: Full-suite gate

- [ ] **Step 1:** From the feature worktree run the whole suite: `scripts/run-tests.sh`.
- [ ] **Step 2:** It must be green. Note: `test_docket_example_yml.sh` / `test_docket_config.sh` may assert on `.docket.yml` contents — if one reds because it pinned an old switch value, that is in-scope to fix (update the assertion to the contracted value; do not weaken it). Any unrelated red (e.g. the known `test_gate_run_stop`/recover-family flakes) → isolate-verify and re-run, do not "fix" on this branch.
- [ ] **Step 3:** `go build ./...` and `go vet ./...` clean.
- [ ] **Step 4: Commit** any gate fixes — `chore(0326): green the suite after config contraction`.

---

## Self-Review

**Spec coverage:** AC1 (committed switches off, no normalization) → Task 1 Step 3. AC2 (migration host drops repo-local routing + auto_capture, global overrides remain) → the operator/results step + the post-change fixture. AC3 (four-layer diagnostic allows mutation; one-at-a-time negatives fail closed) → Task 1 fixture sub-cases 2 & 3, plus the real `diagnostic config --for-mutation` run recorded in results. AC4/AC5 (bridge workflow; scope boundaries) → process + Global Constraints.

**Placeholder scan:** fixture assertions are concrete against the real `DiagnosticConfig` API; the `.docket.yml` edit is three named value flips.

**Type consistency:** `config.Source{Layer,Name,Data}`, `config.LayerGlobal/LayerRepository/LayerRepositoryLocal`, `config.CodeDeferredCapRequested`, `app.DiagnosticConfig`, `Result.MutationAllowed`/`.Diagnostics[].Code`/`.Path` — all from the existing test's usage.

## Notes for the executor

- The layer-awareness of agent pins is the load-bearing premise (global supported, repo/repo-local deferred): `internal/config/capability.go` `dispAgentsLeaf` case gated by `isRepositoryLayer`. Your fixture must exercise BOTH so a future regression that starts flagging global pins reddens here.
- Do not touch `.docket.local.yml` or `~/.config/docket/config.yml` in code — those are the operator's machine-local verification step.
