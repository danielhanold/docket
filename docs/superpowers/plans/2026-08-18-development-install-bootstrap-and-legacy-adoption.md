<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0322 — Bootstrap Go development installation and adopt legacy user-level artifacts](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0322-go-installer-adopt-legacy-bash-installed-user-level-artifact.md)**
<!-- docket:backlink:end -->

# Development-install bootstrap and legacy user-level adoption — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make change 0311's Go development installer usable from a bare checkout and adopt a machine's existing legacy Bash user-level artifacts without hand-deletion.

**Architecture:** Two independent seams on top of an already-built engine. (1) Convert repo-root `install.sh` from the legacy 4-primitive Bash installer into a small POSIX bootstrapper that delegates to `docket development install` (installed binary) or `go run ./cmd/docket development install` (no-binary fallback). (2) Implement the frozen, deterministic legacy byte reproducer that fills change 0311's third ownership proof (`LegacyReproducer`, currently wired `nil`), and thread it through the three inspection call sites so an exact legacy install is adopted rather than reported as an ownership conflict.

**Tech Stack:** Go 1.x (`internal/install`, `internal/cli`, `internal/app`, `internal/harness`), POSIX `sh` for `install.sh`, the repo's Bash test suite (`scripts/run-tests.sh`) for shell tests, Go `testing` + testdata goldens for the reproducer.

**Spec:** `docs/superpowers/specs/2026-08-18-development-install-bootstrap-and-legacy-adoption-design.md` (read from the `docket` metadata branch)

## Global Constraints

- **Do NOT touch the config capability fence.** `internal/config/preflight.go` (`PreflightMutation`/`GuardMutation`) and the `dispDeferred`/`dispAgentsLeaf` schema rows in `internal/config/schema.go` are out of scope (spec "Explicit exclusions"). The dev-install path already calls the fence at `internal/install/devmode.go:83`.
- **No `--force`, no filename-only adoption, no hand-delete instruction, no repository-local scan.** Adoption is byte-exact or it is an `ownership-conflict` that changes nothing (spec "Legacy adoption contract").
- **The reproducer is FROZEN.** It reproduces the bytes the *final v0.9.2 Bash installer* emitted, from a closed inventory and the legacy global inputs only. It must not call the live harness renderers (`internal/harness/*`) — a future renderer change must never silently change what counts as "a legacy install" (spec "Legacy adoption contract"; learning `shared-resource-keeps-first-owner-assumptions`).
- **All filesystem mutation stays inside change 0311's journaled install transaction** (`internal/install/txn.go`). This change adds no new journal or partial-success state (spec "Failure and recovery").
- **The closed inventory is exactly three shapes** (spec lines 85-88): native user-level docket agent definitions, Cursor's docket dispatch `.mdc` rule, and the docket-managed dispatch blocks in supported harness instruction files. Source-linked skill directories are 0311's link-identity proof and are NOT in this inventory.
- **Atomic writes only** — never redirect a renderer into the file it generates (learning `atomic-generated-write`); the transaction engine already does temp+rename.
- **Suite gate:** the whole resolved suite (`scripts/run-tests.sh`) must pass at the end. New Bash tests live under `tests/`; new Go tests are inline `_test.go` beside the code with fixtures under `internal/install/testdata/`.

---

## Part A — `install.sh` POSIX bootstrapper

Independent of Part B; may be built first. `install.sh` today (`install.sh:29-46`) runs `ensure-global-config.sh`, `link-skills.sh`, `sync-agents.sh`, `ensure-docket-env.sh` and never builds a binary. Replace that body with a POSIX bootstrapper.

### Task A1: Bootstrapper source-resolution + tri-state binary discovery

**Files:**
- Modify: `install.sh` (repo root)
- Test: `tests/test_install_bootstrap.sh` (new)

**Interfaces:**
- Produces: an `install.sh` that, run from any CWD, resolves its own directory as `<checkout>` and chooses one of three paths: (a) a compatible installed `docket` → delegate; (b) clean absence of `docket` → `go run`; (c) an errored/ambiguous probe → fail non-zero without mutating anything.

- [ ] **Step 1: Write the failing test** — a Bash test that sources/execs a harness around the discovery function. Create `tests/test_install_bootstrap.sh` following the conventions in `tests/README.md` (read it first for the assert helpers and file placement). First case: run `install.sh` from a different CWD with a stub `docket` on `PATH` that prints a compatibility banner for `development install`, and assert it selects the delegate path (assert via a `DOCKET_BOOTSTRAP_DRY_RUN=1` env that makes the script print the chosen command to stdout instead of executing it).

```sh
# tests/test_install_bootstrap.sh (excerpt)
test_delegates_to_installed_binary() {
  tmp="$(mktemp -d "${TMPDIR:-/tmp}/itest.XXXXXX")"
  # stub docket that advertises the development-install operation
  cat > "$tmp/docket" <<'EOF'
#!/bin/sh
case "$*" in
  "development install --help"|"development --help") echo "install  bootstrap the development binary"; exit 0;;
  *) echo "docket stub: $*"; exit 0;;
esac
EOF
  chmod +x "$tmp/docket"
  out="$( cd / && PATH="$tmp:$PATH" DOCKET_BOOTSTRAP_DRY_RUN=1 sh "$REPO_ROOT/install.sh" )"
  assert_contains "$out" "docket development install --source $REPO_ROOT"
}
```

- [ ] **Step 2: Run it, verify it fails** — `scripts/run-tests.sh tests/test_install_bootstrap.sh`; expect FAIL (script still runs legacy primitives / has no dry-run).
- [ ] **Step 3: Implement source-resolution + discovery** — replace `install.sh`'s body. Resolve the checkout dir independent of CWD (canonicalise `$0`'s dir; reuse the `pwd -P` hop idiom already in `sync-agents.sh:296-324`). Implement `docket_is_compatible()` — probes `command -v docket` and that `docket development install --help` (or `docket development --help`) exits 0 and names the install operation; a probe that errors ambiguously (found but incompatible/erroring) is the third state → fail with an actionable message, do NOT fall through to `go run`. Honor `DOCKET_BOOTSTRAP_DRY_RUN=1` by printing the resolved command and exiting 0.

- [ ] **Step 4: Run it, verify it passes.**
- [ ] **Step 5: Commit** — `feat(0322): install.sh resolves checkout + tri-state docket discovery`.

### Task A2: Delegation, argument passing, and exit propagation

**Files:**
- Modify: `install.sh`
- Test: `tests/test_install_bootstrap.sh`

**Interfaces:**
- Consumes: A1's `docket_is_compatible`, source dir.
- Produces: `install.sh` execs the delegated operation with `--source <checkout>` plus any explicitly-supported passthrough args (`--bin-dir`, repeatable `--harness`), passed as distinct argv words (never string-concatenated), and returns the delegated exit status unchanged.

- [ ] **Step 1: Write failing tests** — (a) absent/incompatible `docket` on `PATH` with `go` present selects `go run ./cmd/docket development install --source <checkout>` (dry-run assert); (b) a `<checkout>` path containing a space is passed as one argv word (stub `docket` echoes `$#` and the raw args; assert arg count and exact source); (c) missing `go` on the no-binary path exits non-zero with an actionable "Go toolchain required" message; (d) the delegated command's non-zero exit is propagated (stub exits 7 → `install.sh` exits 7); (e) supported passthrough flags forwarded in order.

```sh
test_space_in_path_is_one_argument() {
  tmp="$(mktemp -d "${TMPDIR:-/tmp}/it sp.XXXXXX")"   # space in path
  cp -R "$REPO_ROOT" "$tmp/checkout"
  cat > "$tmp/docket" <<'EOF'
#!/bin/sh
case "$1 $2" in "development install") echo "ARGC=$#"; shift 2; printf 'SRC=%s\n' "$2";; esac
EOF
  chmod +x "$tmp/docket"; # ...advertise compatibility as in A1...
  out="$( PATH="$tmp:$PATH" sh "$tmp/checkout/install.sh" )"
  assert_contains "$out" "SRC=$tmp/checkout"
}
```

- [ ] **Step 2-4: Red → implement `exec`-based delegation (`"$@"`-safe, no `eval`, no command-string construction) → green.**
- [ ] **Step 5: Commit** — `feat(0322): install.sh delegates with argv-safe args and exit propagation`.

### Task A3: Remove the legacy primitive sequence; prove none runs

**Files:**
- Modify: `install.sh`
- Test: `tests/test_install_bootstrap.sh`

**Interfaces:**
- Produces: `install.sh` runs NONE of `ensure-global-config.sh` / `link-skills.sh` / `sync-agents.sh` / `ensure-docket-env.sh` before or after delegation.

- [ ] **Step 1: Write the failing test** — put shim copies of the four legacy scripts on the resolved path that each `touch` a sentinel file when run; run `install.sh` in dry-run and assert no sentinel exists. (Key the guard on the *property* — "a legacy primitive executed" via sentinels — not on grepping `install.sh` for names, per learning `byte-pattern-guard-matches-a-spelling`.)
- [ ] **Step 2: Run it** — likely still FAILS until every legacy line is gone.
- [ ] **Step 3: Delete the four-primitive body** from `install.sh`, leaving only the bootstrapper. Preserve the file header comment's intent but rewrite it to describe the bootstrapper.
- [ ] **Step 4: Run the whole bootstrap test file — green.**
- [ ] **Step 5: Commit** — `feat(0322): install.sh runs no legacy install primitive`.

---

## Part B — Frozen legacy reproducer + adoption wiring

The seam: `type LegacyReproducer func(t Target) ([]byte, bool)` (`internal/install/inspect.go:50`). Proof three (`provenByLegacy`, `inspect.go:269`) already compares on-disk bytes to `legacy(t)` for KindFile and KindSymlink. Production passes `nil` at `service.go:320` and `service.go:431`. **Managed blocks are NOT yet wired** — `inspectManagedBlock` takes no `legacy` param, so a legacy dispatch block is currently a foreign-block conflict. This part supplies the reproducer and threads it through all three kinds.

### Task B1: Capture the frozen legacy golden corpus

**Files:**
- Create: `internal/install/testdata/legacy/README.md` (inventory notes)
- Create: `internal/install/testdata/legacy/<harness>/<shape>/...` (captured bytes)
- Create: `internal/install/legacy_capture_test.go` (a guarded generator, skipped by default)

**Interfaces:**
- Produces: byte-exact snapshots of what the v0.9.2 Bash installer writes at user level, across the applicable harness set (`claude`, and the AGENTS.md-dispatch harnesses `codex`/`opencode`, and `cursor`) and the global-pin shapes (agents fully pinned model+effort; partially pinned; unpinned). This corpus is the source of truth every later task's goldens derive from (learning `frozen-corpus-covers-what-it-contains`: inventory what the fixture actually contains, and record the empty categories).

- [ ] **Step 1:** Read `sync-agents.sh` end-to-end and `link-skills.sh` to enumerate exactly which user-level files the final Bash installer writes and how each is parameterised by the global `agents:` pins and `agent_harnesses`. Write the enumeration into `testdata/legacy/README.md` — one row per (kind, harness, pin-shape), marking which shapes exist and which are empty.
- [ ] **Step 2:** Capture real bytes: run `sync-agents.sh` under a sandboxed `HOME` and a controlled `~/.config/docket/config.yml` for each pin shape (fully/partial/unpinned), copying the emitted `~/.<harness>/agents/docket-*.md`, the Cursor `docket-dispatch.mdc`, and the managed dispatch block interior from each instruction file into the matching `testdata/legacy/...` path. Do the same for the two agents whose wrappers wrap no skill vs. the skill-bearing ones, so both wrapper variants are represented.
- [ ] **Step 3:** Commit the corpus — `test(0322): freeze legacy user-level golden corpus`. (No red/green cycle: this task's deliverable is the fixtures + inventory that later tasks assert against.)

### Task B2: Reproducer for native user-level agent definitions (KindFile)

**Files:**
- Create: `internal/install/legacy.go`
- Create: `internal/install/legacy_test.go`
- Reference: `internal/install/target.go` (Target), `internal/install/inspect.go:50` (LegacyReproducer)

**Interfaces:**
- Produces:
  - `type LegacyInputs struct { Harnesses []string; AgentPins map[string]AgentPin /* keyed by agent short-name */; ... }` — the closed set of legacy global inputs (resolved model/effort per agent, harness set) required to recreate the bytes, and NOTHING else.
  - `func NewLegacyReproducer(in LegacyInputs) LegacyReproducer` — returns a `func(Target)([]byte,bool)`. For a KindFile agent-definition Target whose path/kind/harness are in the closed inventory, it returns the frozen bytes and `true`; for any target outside the inventory it returns `(nil, false)`.
  - Frozen rendering is a self-contained Go port of the v0.9.2 wrapper format (frontmatter + body). It reads embedded frozen template text (`//go:embed` of a snapshot placed under `internal/install/legacydata/`), never `internal/harness/*`.

- [ ] **Step 1: Write the failing test** — table test over the B1 corpus: for each captured native agent file, build the matching `LegacyInputs`, call `NewLegacyReproducer(in)` and then the returned func with the corresponding `Target{Path:…, Kind:KindFile, Role:roleAgent…}`, assert `ok == true` and `bytes.Equal(got, goldenBytes)`.

```go
func TestLegacyReproducer_NativeAgents(t *testing.T) {
	for _, tc := range loadLegacyCorpus(t, "claude") { // reads testdata/legacy/claude/<shape>/agents/*
		rep := NewLegacyReproducer(tc.Inputs)
		got, ok := rep(Target{Path: tc.Path, Kind: KindFile, Role: roleAgent})
		if !ok { t.Fatalf("%s: reproducer reported no legacy spelling", tc.Path) }
		if !bytes.Equal(got, tc.Golden) {
			t.Fatalf("%s: bytes differ:\n%s", tc.Path, firstDiff(got, tc.Golden))
		}
	}
}
```

- [ ] **Step 2: Run it — FAIL** (`legacy.go` absent).
- [ ] **Step 3: Implement** `LegacyInputs`, `NewLegacyReproducer`, the embedded frozen templates, and the per-target rendering for KindFile agent defs, matching the corpus byte-for-byte. Route target→inventory identity by absolute path shape (`~/.<harness>/agents/docket-<name>.md`) and role, canonicalising path hops (learning `canonicalise-every-symlink-hop`).
- [ ] **Step 4: Run it — PASS** for every pin shape.
- [ ] **Step 5: Commit** — `feat(0322): frozen legacy reproducer for native agent definitions`.

### Task B3: Reproducer for Cursor's docket-dispatch `.mdc` rule

**Files:**
- Modify: `internal/install/legacy.go`, `internal/install/legacy_test.go`
- Reference: `internal/harness/cursor/testdata/golden/docket-dispatch.mdc` (shape reference only), B1 cursor corpus

**Interfaces:**
- Consumes: B2's `LegacyInputs`, `NewLegacyReproducer`.
- Produces: the reproducer additionally returns frozen bytes+`true` for the Cursor `docket-dispatch.mdc` target (KindFile or KindSymlink per how the legacy installer wrote it — confirm from B1 capture) when `cursor` is in `Harnesses`.

- [ ] **Step 1: Write the failing test** — assert `rep(cursorDispatchTarget)` equals the captured `.mdc` bytes; and that with `cursor` absent from `Harnesses` it returns `(nil,false)`.
- [ ] **Step 2-4: Red → implement the frozen `.mdc` rendering → green.**
- [ ] **Step 5: Commit** — `feat(0322): frozen legacy reproducer for Cursor dispatch rule`.

### Task B4: Reproducer for managed dispatch blocks; thread `legacy` into `inspectManagedBlock`

**Files:**
- Modify: `internal/install/legacy.go`, `internal/install/legacy_test.go`
- Modify: `internal/install/inspect.go` (thread `legacy` into `inspectManagedBlock` + add a legacy-interior proof)
- Modify: `internal/install/inspect_test.go`

**Interfaces:**
- Produces:
  - For a `KindManagedBlock` Target (`BlockName == "dispatch"`), `legacy(t)` returns the frozen **block interior** bytes + `true`.
  - `inspectManagedBlock(t, info, rec, hasRec, legacy)` — new trailing param; and its caller in `InspectTarget` (`inspect.go` KindManagedBlock case) passes `legacy` through. When `hasRec` is false and the on-disk block interior equals `legacy(t)`'s output, classify `DispositionUpdate` (adopt) instead of `conflict(ReasonOwnershipConflict, remedyForeignBlock)`. Malformed/unbalanced markers still short-circuit to `ReasonManagedBlockInvalid` BEFORE any legacy check (spec: markers valid, ordered, balanced is a precondition of adoption).

- [ ] **Step 1: Write the failing tests** — in `inspect_test.go`, supply a stub `LegacyReproducer` that returns a known interior; assert: (a) a file containing a valid docket:dispatch block whose interior equals the reproducer output, with NO prior record, classifies `DispositionUpdate`; (b) the same block with one byte changed classifies `DispositionConflict`/`ReasonOwnershipConflict`; (c) a malformed-marker file classifies `ReasonManagedBlockInvalid` even when the reproducer would match; (d) `legacy == nil` preserves today's foreign-block conflict behavior (regression guard).
- [ ] **Step 2: Run — FAIL** (`inspectManagedBlock` has no legacy param).
- [ ] **Step 3: Implement** the param threading, the interior legacy proof (reuse `normalizeInterior` for the comparison to match how proof-two compares), and the managed-block rendering in `legacy.go`.
- [ ] **Step 4: Run — PASS**, and run the whole `internal/install` package tests to confirm no regression in existing `inspect_test.go`.
- [ ] **Step 5: Commit** — `feat(0322): adopt legacy docket:dispatch blocks via frozen interior proof`.

### Task B5: Production wiring — build inputs, pass non-nil, update the not-adopted note

**Files:**
- Modify: `internal/install/service.go` (`:320`, `:431` call sites, and the KindManagedBlock caller from B4)
- Modify: `internal/install/devmode.go` and/or `internal/app/install.go` (construct `LegacyInputs` from resolved config/global layer; pass a reproducer into the plan/inspect path)
- Modify: `internal/app/install.go:150-157` (`legacyNotAdoptedNote`)
- Test: `internal/install/service_test.go`, `internal/app/install_test.go`

**Interfaces:**
- Consumes: `NewLegacyReproducer`, `LegacyInputs`.
- Produces: the real install path constructs `LegacyInputs` from the same resolved global inputs the legacy installer used (harness set + resolved agent model/effort pins) and passes `NewLegacyReproducer(in)` where `nil` is passed today; `legacyNotAdoptedNote` is updated/removed so an *adoptable* exact-legacy tree no longer prints "move them aside" (that note remains only for genuinely unknown/drifted targets).

- [ ] **Step 1: Write the failing test** — a `service`/`app`-level test: seed a temp HOME with an exact-legacy user-level tree (bytes from the B1 corpus) and no prior `state/install.json`; run the plan/inspect step; assert every legacy target classifies `DispositionUpdate` (adopt), NOT `DispositionConflict`, and that a subsequent apply records Go ownership in `state/install.json`. Add a negative: a tree with one unknown foreign file still yields a conflict for that file only.
- [ ] **Step 2: Run — FAIL** (nil reproducer today → conflicts).
- [ ] **Step 3: Implement** input construction + non-nil wiring; update `legacyNotAdoptedNote`.
- [ ] **Step 4: Run — PASS.**
- [ ] **Step 5: Commit** — `feat(0322): wire the frozen legacy reproducer into the install path`.

### Task B6: Filesystem/adoption state matrix

**Files:**
- Create/Modify: `internal/install/adoption_test.go`
- Reference: `internal/install/txn.go`, `internal/install/state.go`

**Interfaces:**
- Consumes: the wired install path (B5).
- Produces: end-to-end tests over homes in each starting state — clean, exactly-legacy, partially-legacy (some targets legacy, some absent), mixed-unknown (legacy + foreign files), drifted (legacy byte-mutated), and interrupted (an unpublished journal present) — asserting atomic replacement, full rollback on an injected apply failure, unrelated-byte preservation, and the final `state/install.json` ownership records.

- [ ] **Step 1: Write the failing tests** (one sub-test per starting state; use the transaction engine's existing test seams for fault injection — read `txn_test.go` for the pattern).
- [ ] **Step 2-4: Red → implement any gaps surfaced (most behavior should already hold via B5 + 0311's engine) → green.**
- [ ] **Step 5: Commit** — `test(0322): adoption state matrix over clean/legacy/mixed/drifted/interrupted homes`.

### Task B7: Mutation refusal guards

**Files:**
- Modify: `internal/install/legacy_test.go`

**Interfaces:**
- Produces: a mutation table proving adoption refuses when ANY dimension is perturbed — flip one byte, break a marker, change a path to an out-of-inventory spelling, change a legacy input (a pin), or change the target kind — each must make `rep(...)` return `false` or make inspection classify `Conflict`/`ManagedBlockInvalid`. Bound the guard on the property, not a spelling (learning `byte-pattern-guard-matches-a-spelling`); mutation-test the guard itself (AGENTS.md "Guards and tests").

- [ ] **Step 1: Write the table** — parametrised over the five mutation dimensions against the B1 corpus.
- [ ] **Step 2: Run — expect PASS** if B2-B5 are correct; any RED here is a real reproducer over-match to fix.
- [ ] **Step 3: Commit** — `test(0322): mutation matrix — adoption refuses on any perturbed dimension`.

---

## Part C — Full-suite gate

### Task C1: Run the whole resolved suite and green it

- [ ] **Step 1:** From the feature worktree run the whole suite: `scripts/run-tests.sh` (the resolved `finalize.test_command`). It runs every file in parallel with per-job isolation and per-file wall-clock budgets.
- [ ] **Step 2:** Fix any failure or `OVER BUDGET:` line the run surfaces (AGENTS.md "Run the whole suite at the build gate"). An OVER BUDGET line does not fail the run but is a finding to act on — shard or trim as needed, or capture a follow-up if out of scope.
- [ ] **Step 3:** Confirm `go build ./...` and `go vet ./...` are clean.
- [ ] **Step 4: Commit** any gate fixes — `test(0322): green the full suite`.

---

## Self-Review

**Spec coverage:**
- Checkout entry-point contract (spec "Checkout entry-point contract", AC1) → Part A (A1-A3).
- Legacy adoption contract, closed inventory, byte-exactness, markers, no `--force` (spec "Legacy adoption contract", AC2) → B2-B5, B7.
- Clean + adopted installs publish binary/assets/harness material/`state/install.json` atomically and idempotently (AC3) → B5, B6 (the binary/asset/harness pieces themselves already exist per reconcile; this change adds adoption + the bootstrap entry).
- Frozen goldens across harness + pin shapes; mutation refuses (spec "Testing strategy") → B1, B7.
- Filesystem state matrix incl. interrupted/rollback (spec "Testing strategy", "Failure and recovery") → B6.
- Migration-host transition rule (spec) → largely process (executed via this v0.9.2 Bash bridge run); the repository-testable slice is B5/B6 (machine-install roots only); finalize/recovery and the live restart/reload confirmation are explicitly deferred (0316) / recorded as a human verification item.
- Explicit exclusions (fence, config edits, metadata ops, release packaging) → enforced by Global Constraints; no task touches them.

**Placeholder scan:** Shell steps carry real code; Go steps carry real signatures and test bodies. The one deliberately capture-first task is B1 (its "content" is real captured bytes from the actual generator, not a TODO) — every later Go task asserts against that concrete corpus.

**Type consistency:** `LegacyReproducer func(Target)([]byte,bool)` (existing), `LegacyInputs`/`NewLegacyReproducer` (B2, reused B3-B5), `inspectManagedBlock(…, legacy)` new param (B4) and its lone caller updated in the same task, `Target`/`KindFile`/`KindSymlink`/`KindManagedBlock`/`DispositionUpdate`/`DispositionConflict`/`ReasonOwnershipConflict`/`ReasonManagedBlockInvalid` all from `internal/install/target.go`/`inspect.go`.

## Notes for the executor

- Read `tests/README.md` before writing any Bash test (assert helpers, where a new test belongs, how the suite runner budgets files).
- The exact legacy byte formats are authoritative in `sync-agents.sh` / `link-skills.sh`; the B1 corpus freezes them. When B2-B4 goldens disagree with a live `internal/harness/*` golden, the *legacy* capture wins — the reproducer proves legacy ownership, not current-render equality.
- Keep `legacy.go`'s embedded frozen templates under a dedicated `internal/install/legacydata/` dir so a later renderer change cannot reach them.
