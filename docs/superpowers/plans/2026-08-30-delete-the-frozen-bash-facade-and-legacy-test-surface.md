<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0370 — Delete the frozen Bash facade and legacy test surface](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-31-0370-delete-the-frozen-bash-facade-and-legacy-test-surface.md)**
<!-- docket:backlink:end -->
# Delete the Frozen Bash Facade and Legacy Test Surface — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: this plan is executed task-by-task by
> docket-build profile workers under the docket-build-task contract, with a single
> full-suite gate at the end of the run. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the frozen Bash control plane (`scripts/docket.sh`, its helper/runtime tree,
`scripts/run-tests.sh`, compatibility launchers, and the `DOCKET_SCRIPTS_DIR` /
`DOCKET_BASH_PATH` / `runtime.bash` seams), migrate every surviving test invariant to
mutation-sensitive Go coverage or the two retained POSIX product suites, and contract
`docket development test` to the final topology — with shape-derived, mutation-tested
absence guards and truthful ADR consequences.

**Architecture:** Seven ordered build gates from the spec: (1) shape-derived inventory and
9-class classification, fail-closed; (2) assertion-level test classification and
replacement coverage proven red-on-mutation BEFORE deletion; (3) runner contraction to
Go targets plus two declared POSIX product categories; (4) generator correction and
deterministic regeneration; (5) physical deletion plus repeated discovery; (6) final
mutation-tested absence guards; (7) ADR consequences and whole-suite verification.
Coverage replacement always precedes deletion; unknowns always block.

**Tech Stack:** Go 1.x (`go test`, `internal/suiterunner`, new `internal/repoguard`
package), POSIX shell for the two retained product suites, the docket CLI
(`docket artifact`, `docket development test`), the docket-adr workflow.

**Spec:** `docs/superpowers/specs/2026-08-29-delete-the-frozen-bash-facade-and-legacy-test-surface-design.md`
(synchronized copy readable at `.docket/docs/superpowers/specs/…` from the primary tree).
The spec's "Implementation gates" (1–7) and "Acceptance criteria" (1–23) are the backbone
of this plan; every task cites the gates and criteria it discharges.

## Global Constraints

Copied from the spec; every task's requirements implicitly include these.

- **Coverage before deletion.** No legacy test file or assertion is deleted before its
  surviving invariants have replacement coverage proven red on a targeted mutation
  (spec "Assertion-level test classification"; learnings `compensating-assert-must-exist-when-cited`,
  `assert-detects-removal-not-replacement`).
- **Fail closed on unknowns.** An unclassified candidate, an errored probe, or a
  maintained executable consumer of the frozen surface BLOCKS the task that found it —
  return `BLOCKED` with the evidence; never proceed past it. "Failed probes, incomplete
  traversal, inaccessible inputs, parse ambiguity, and classification errors are not
  absence evidence" (spec; learning `probe-error-is-not-clean-absence`). A grep that
  matches nothing must be distinguished from a grep that failed
  (learning `agent-shell-noop-reads-as-success`): check exit codes — `git grep` exits 1
  on no-match, ≥2 on error.
- **No spelling-pinned, count-pinned, or line-number gates.** Counts (~49 scripts,
  ~188 tests, ~204 references) are review context only. Derive sites by syntactic /
  behavioral shape with both-side-bounded patterns
  (learning `byte-pattern-guard-matches-a-spelling`; AGENTS.md "Key a guard on syntactic
  shape"). Never hand-list sites — derive from a whole-repo grep, then sort prose vs
  executable (AGENTS.md).
- **Immutable history stays untouched.** Never edit, delete, or "fix" matches inside:
  `docs/adrs/` (Accepted bodies; status changes go through the docket-adr workflow),
  `docs/superpowers/specs/`, `docs/superpowers/plans/` (prior plans), archived
  changes/results/learnings (metadata tree), and the frozen v0.9.2 corpus under
  `internal/repository/testdata/` (learning `frozen-fixture-corpus-trips-repo-wide-scans`).
  Exclusions in guards are categorical (by location/ownership), never a per-file allowlist.
- **Executable surface is defined by who executes the bytes, not the extension.**
  Maintained `.md` files that instruct an agent to run commands (`scripts/*.md`,
  `skills/*/SKILL.md`, skill references) are executable surface for discovery and
  guards; `docs/` history is not (learning `agent-executed-markdown-is-code`).
- **Mutation protocol (use verbatim everywhere a mutation is required):**
  ```bash
  cp "$f" "$f.bak"            # NEVER rely on `git checkout --` to restore (learning
  # …apply the mutation to "$f"…       mutation-restore-needs-a-backup-copy)
  go test ./<pkg>/ -run '<TestName>' -count=1   # expect FAIL — -count=1 defeats the
  mv -f "$f.bak" "$f"                            # result cache (learning
  go test ./<pkg>/ -run '<TestName>' -count=1   # cached-runner-serves-a-mutated-tree)
  ```
  A mutation that leaves the assert green is a defect until proven otherwise
  (AGENTS.md "Guards and tests"). Record every mutation and its observed FAIL line in
  the build-evidence record.
- **Per-task verification is focused, not whole-suite.** The single whole-suite gate
  runs at the end of the build (docket-build contract). Tasks run
  `go test ./<touched pkgs>/ -count=1` and `bash tests/<touched file>` only.
  **Known transitional red window:** between Task 6 (runner contraction) and Task 8
  (deletion), `docket development test` is expected RED because undeclared legacy files
  fail closed. This is deliberate and ends at Task 8; do not "fix" it early.
- **Classification ledgers are auditable build evidence.** Tasks 1 and 2 produce
  ledgers; record them in the build-evidence record (the channel docket-build already
  maintains for the reviewer), and summarize disposition classes in commit messages.
  Acceptance 7 requires every substantive removed assertion to have an auditable
  individual classification.
- **Commit trailer:** every commit ends with
  `Claude-Session: https://claude.ai/code/session_01WZg51gUmtyVT4g1SSEmVJj`.

---

### Task 1: Shape-derived inventory and 9-class classification (Gate 1; acceptance 1, 2)

**Recommended profile:** premium (fail-closed judgment over the whole tree; everything
downstream keys off this ledger).

**Files:**
- No source changes. Output is the classification ledger in the build-evidence record
  plus a scratch worklist (do not commit scratch files).

**Interfaces:**
- Produces: the **candidate ledger** — one row per candidate site:
  `path[:construct] | shape that matched | class 1–9 | justification`. Classes (spec):
  1 active maintained executable consumer (BLOCKS), 2 active maintained test dependency,
  3 canonical generator, 4 generated product, 5 active maintained prose,
  6 immutable historical record, 7 frozen release artifact, 8 false positive with a
  structural explanation, 9 unknown/unclassified (BLOCKS).
- Later tasks consume the ledger: Task 2 takes class 2; Task 7 takes classes 3/4/5;
  Task 8 takes the deletion set.

- [ ] **Step 1: Verify the base (acceptance 1).** From the feature worktree:
  ```bash
  git log --oneline -20   # confirm base b853e8c0 (contains merged 0369, 0371, 0372)
  ls scripts/docket.sh scripts/lib/ scripts/run-tests.sh   # frozen surface still present
  ```
  If any of the three is already absent, the base moved (learning `moving-base`):
  return `BLOCKED` with the observation — do not improvise.

- [ ] **Step 2: Run the shape probes.** Each probe must be checked for error-vs-no-match.
  Seed shapes (useful, NOT proof of completeness — extend by what you find):
  ```bash
  # Direct execution / path composition of the facade and runner
  git grep -nE '(^|[^[:alnum:]_.-])(docket|run-tests)\.sh([^[:alnum:]_-]|$)' -- . \
    ':!docs/' ':!internal/repository/testdata/'
  # Retired env/config concepts, token-bounded
  git grep -nE '(^|[^[:alnum:]_])(DOCKET_SCRIPTS_DIR|DOCKET_BASH_PATH)([^[:alnum:]_]|$)' -- . \
    ':!docs/' ':!internal/repository/testdata/'
  git grep -nE '(^|[^[:alnum:]_./-])runtime\.bash([^[:alnum:]_-]|$)' -- . \
    ':!docs/' ':!internal/repository/testdata/'
  # Sourcing / runtime imports of the helper tree
  git grep -nE '(^|[;&|[:space:]])(source|\.)[[:space:]]+[^[:space:]]*scripts/lib/' -- . \
    ':!docs/' ':!internal/repository/testdata/'
  # Every scripts/ entry is itself a candidate (executable, contract .md, runners, lib)
  git ls-files scripts/
  # Legacy test corpus + budget registry rows
  git ls-files tests/ | grep -v '^tests/fixtures/' ; cat tests/runtime-budgets.tsv | wc -l
  ```
  Then sweep the *excluded-from-seed* surfaces for candidates the seeds miss:
  variable-composed invocations (`git grep -nE '\$[{A-Za-z_].*\.sh'` over `scripts/`,
  `skills/`, `.github/`), wrapper functions and command arrays, `.github/workflows/`,
  `install.sh`, `skills/*/SKILL.md` + references, `scripts/runners/*`, `.docket.example.yml`
  and any config schema keys (`git grep -n 'scripts' internal/config/`), and generated
  agent wrappers plus their generators. Record every probe command, its exit code, and
  its match count in the evidence.

- [ ] **Step 3: Classify every candidate into classes 1–9.** Judgment calls the ledger
  must land explicitly (verify against the tree, do not assume — learning
  `verify-the-claim`):
  - `internal/cli/development_test_cmd.go` reads `DOCKET_BASH_PATH` → class 1-adjacent
    *runner seam*, disposed by Task 6 (runner contraction), not a cutover blocker: it is
    the surface this change itself retires.
  - `scripts/check-test-source-hygiene.sh` is consumed by the surviving Go runner
    (`Config.HygienePath`) → mixed responsibility, disposed by Task 6.
  - `CLAUDE.md` (AGENTS.md) names `scripts/run-tests.sh` as "the frozen parity oracle" →
    class 5 active maintained prose, corrected in Task 7.
  - `tests/README.md`, `tests/runtime-budgets.tsv` → classes 5/2, Tasks 6–8.
  - Anything under `docs/` in the feature tree and the metadata archive → class 6.
    `internal/repository/testdata/` v0.9.2 corpus → class 7.
- [ ] **Step 4: Enforce the gate.** If any candidate lands in class 1 (a maintained
  executable consumer the merged 0369/0371/0372 cutover should have migrated) or
  class 9, STOP: return `BLOCKED` naming the site. Exception per spec "Failure and
  recovery": a *small* missed caller consistent with the merged cutover may be
  reconciled — note it in the ledger and add its migration to the affected task —
  but a material redesign halts the run.
- [ ] **Step 5: Record the ledger in build evidence and commit nothing.** Report
  `COMPLETE` with the ledger attached.

---

### Task 2: Assertion-level classification of the legacy test corpus (Gate 2, first half; acceptance 7)

**Recommended profile:** premium (the disposition of ~188 files hangs on this; wrong
classes silently lose product coverage).

**Files:**
- No source changes. Output: the **assertion ledger** in build evidence.

**Interfaces:**
- Consumes: Task 1's candidate ledger (class 2 rows).
- Produces: for every `tests/test_*.sh` file (and any shell/bats test found elsewhere by
  Task 1), a per-assertion-block disposition:
  `file | block/case name or anchor | class | replacement home`. Classes (spec):
  **A** surviving product invariant → Go coverage (Task 3 or 4, named per row);
  **B** installer invariant → retained `install.sh` POSIX suite (Task 5);
  **C** release-downloader invariant → retained downloader POSIX suite (Task 5);
  **D** deleted-implementation mechanism → deleted with its subject (Task 8);
  **E** mixed/uncertain → decompose into A–D sub-rows; a residually-uncertain row BLOCKS
  its deletion (it stays until understood).

- [ ] **Step 1: Enumerate the corpus.** `git ls-files 'tests/test_*.sh'` plus any test
  files Task 1 found outside `tests/`. Files already in the surviving topology
  (`test_go_*.sh` wrappers that run `go test`, the installer suite, the downloader
  suites) still get rows — their class is "already A/B/C in place; needs category
  declaration only (Task 6)".
- [ ] **Step 2: Classify by what each block GUARDS, not what it asserts**
  (learning `test-premise-deleted-not-regated`). Rules of thumb the ledger must apply:
  - A block whose subject is facade routing, `scripts/lib` sourcing, Bash-runner
    internals (`run-tests.sh` scheduling/budget mechanics now owned by
    `internal/suiterunner`'s own tests), retired env propagation, or a deleted
    `scripts/*.sh` command's internals → class D.
  - A block guarding a repo-wide property of maintained source (comment anchor style,
    grep/BSD portability of *surviving* shell, budget-registry correspondence,
    generated-block hygiene, docs/link coverage) → class A: the property outlives the
    Bash implementation; its scan moves to Go (Task 3).
  - A block guarding user-visible behavior now owned by the Go CLI (lifecycle state,
    config meaning, generated-content contracts, atomicity/safety/recovery) → class A
    (Task 4) — but FIRST check whether equivalent Go coverage already exists from
    0369/0371/0372 (`git grep -ln '<behavior anchor>' internal/`); if a
    mutation-verified Go test already covers it, record "already covered by <TestName>"
    with the mutation you ran to prove that, and the row needs no new test.
  - Blocks in `test_bash_runtime_install.sh` / `test_bash_runtime_routing.sh` /
    `test_docket_facade.sh` / `test_devtest_*` differential-vs-oracle tests: mostly D,
    but decompose — a differential test's *runner-guarantee* half may be A for Task 6.
- [ ] **Step 3: Grep the suite for prose the deletion will remove** (learning
  `restatement-accumulates-its-own-guards`): for each maintained file scheduled for
  deletion or rewrite, `git grep -ln '<its basename>' tests/ skills/ scripts/ CLAUDE.md`
  and add dependent-assert rows so Task 8 does not strand a grep-dependent test.
- [ ] **Step 4: Record the ledger; report COMPLETE.** Any class-E row that cannot be
  decomposed is reported in the evidence as an open blocker on Task 8 for that row only.

---

### Task 3: Replacement coverage batch A — repo-guard scans move to Go (Gate 2; acceptance 8)

**Recommended profile:** premium (guard design; vacuous guards are the named risk).

**Files:**
- Create: `internal/repoguard/repoguard.go` — shared walker + categorical exclusions.
- Create: `internal/repoguard/guards_test.go` (split into more files by domain if large;
  follow the ledger).
- Test: the same `_test.go` files; executed by the existing `go test ./...` wrappers.

**Interfaces:**
- Consumes: Task 2's class-A rows whose subject is maintained-source shape.
- Produces (for Tasks 8 and 9):
  ```go
  package repoguard
  // MaintainedFiles walks the repo root and returns every maintained file path,
  // applying the categorical exclusions (docs/, internal/repository/testdata/,
  // .git, tests/fixtures/, .worktrees). Fail-closed: an unreadable dir is an error.
  func MaintainedFiles(root string) ([]string, error)
  // ExecutableSurface filters MaintainedFiles to files whose bytes an agent or shell
  // executes: *.sh, *.bash, executable-bit files, and maintained .md command surfaces
  // (scripts/*.md, skills/**/*.md) — learning agent-executed-markdown-is-code.
  func ExecutableSurface(root string) ([]string, error)
  ```

- [ ] **Step 1: Write `MaintainedFiles`/`ExecutableSurface` with their own tests first**
  (temp-dir fixture trees proving inclusion, categorical exclusion, and the fail-closed
  error on an unreadable root). Run `go test ./internal/repoguard/ -count=1` — expect
  FAIL before the implementation, PASS after.
- [ ] **Step 2: Port each ledger row.** For every class-A repo-guard row, write the Go
  test that enforces the same property over `MaintainedFiles`/`ExecutableSurface`.
  Requirements per guard:
  - Bound patterns on both sides; where an equivalent spelling survives unguarded,
    state the limitation in the guard's doc comment, not a buried code comment
    (learning `byte-pattern-guard-matches-a-spelling`).
  - For correspondence guards (e.g., budgets registry ↔ test files), write BOTH
    directions and anchor the reverse loop on consuming code, not an allowlist
    (learning `correspondence-guard-runs-one-way`).
  - Write the assert that DETECTS the violation, then plant the violation and watch it
    redden (learning `assert-detects-removal-not-replacement`).
- [ ] **Step 3: Mutation-prove every guard** with the Global-Constraints protocol
  (plant a violating file/line just outside the excluded corpus; expect FAIL; restore).
  Also run one negative control: a violation planted *inside*
  `internal/repository/testdata/` must stay GREEN.
- [ ] **Step 4: Update the ledger rows** in build evidence with `replaced by <TestName>,
  mutation observed FAIL: <line>`. Do NOT delete any legacy test yet.
- [ ] **Step 5: Commit** `test(0370): port repo-guard invariants to internal/repoguard (batch A)`.

---

### Task 4: Replacement coverage batch B — product-behavior invariants move to Go (Gate 2; acceptance 8)

**Recommended profile:** standard.

**Files:**
- Modify/Create: `_test.go` files in the packages that OWN each behavior
  (`internal/cli/`, `internal/app/`, `internal/config/`, `internal/repository/`, … —
  the ledger names the home per row; a behavior's test lives beside its owner, not in
  `internal/repoguard`).

**Interfaces:**
- Consumes: Task 2's class-A rows whose subject is runtime behavior of the Go CLI or
  generated products; the "already covered by <TestName>" rows (verify, don't re-port).

- [ ] **Step 1: For each row, write the failing/target Go test** in the owning package,
  pinning mechanism not just outcome (learning `assert-pins-outcome-not-mechanism`).
  Where the legacy assertion tested a Bash script's *copy* of behavior the Go CLI now
  owns, the Go test targets the Go implementation — never resurrect the script to test
  against.
- [ ] **Step 2: Mutation-prove each new test** (protocol above; mutate the guarded
  production code, expect FAIL, restore).
- [ ] **Step 3: For "already covered" rows, run the claimed test's mutation once**
  to verify the citation (learning `compensating-assert-must-exist-when-cited`) and
  record the FAIL line.
- [ ] **Step 4: Run** `go test ./... -count=1` over touched packages; expect PASS.
- [ ] **Step 5: Commit** `test(0370): port surviving product invariants to Go (batch B)`.

---

### Task 5: Retained POSIX product suites — installer and release downloader (Gate 2; acceptance 9)

**Recommended profile:** standard.

**Files:**
- Modify: the installer suite (today `tests/test_install_bootstrap.sh` and any sibling
  the ledger assigns to class B) and the downloader suites (today
  `tests/test_release_downloader.sh`, `…_converge.sh`, `…_refusals.sh` and any sibling
  in class C) — exact membership comes from Task 2's ledger, not this list.
- Modify: shared helpers under `tests/lib/` ONLY where they serve these two products
  without reconstructing the deleted runtime (spec boundary).

**Interfaces:**
- Produces: every surviving shell test file carries a category declaration header
  (consumed by Task 6's discovery):
  ```
  # docket-suite: posix-install      (or posix-downloader, or go)
  ```
  placed in the first 10 lines. Task 6 defines the parser; the header line format is
  fixed here so both tasks agree: `^# docket-suite: (go|posix-install|posix-downloader)$`.

- [ ] **Step 1: Move class-B/C assertions.** For each ledger row whose invariant
  currently lives in a file being deleted, rewrite it inside the owning product suite.
  Helpers imported from `tests/lib/` that source `scripts/lib/` or the runtime must be
  inlined/rewritten — the retained suites may not depend on the deletion surface.
- [ ] **Step 2: Add the `# docket-suite:` header** to every surviving shell test file:
  the two POSIX product suites get their product category; every `test_go_*.sh`
  wrapper gets `go`.
- [ ] **Step 3: Mutation-prove one representative invariant per product** (mutate
  `install.sh` / the downloader; expect the suite file to FAIL when run directly via
  `bash tests/<file>`; restore) and record it.
- [ ] **Step 4: Run each touched suite file directly**; expect PASS.
- [ ] **Step 5: Commit** `test(0370): consolidate retained POSIX product suites and declare suite categories`.

---

### Task 6: Contract `docket development test` to the final topology (Gate 3; acceptance 5 (runner seam), 11–14)

**Recommended profile:** premium (the runner is becoming the SOLE channel — every
guarantee the Bash oracle duplicated must be re-proven on the survivor; learning
`sole-channel`).

**Files:**
- Modify: `internal/suiterunner/discover.go` — category-declared discovery.
- Modify: `internal/suiterunner/run.go` — drop any Bash-oracle-only compatibility
  branch; hygiene preflight disposition (below).
- Modify: `internal/cli/development_test_cmd.go` — remove the `DOCKET_BASH_PATH` seam
  (`cfg.Bash` resolves `bash` on PATH only) and the `scripts/check-test-source-hygiene.sh`
  path wiring per the disposition below.
- Modify: `internal/suiterunner/discover_test.go`, `run_test.go`, and siblings.
- Modify: `tests/README.md` (runner section describes the final topology).

**Interfaces:**
- Consumes: Task 5's `# docket-suite:` header contract (regex above).
- Produces:
  ```go
  type Category string
  const (
      CategoryGo         Category = "go"
      CategoryInstall    Category = "posix-install"
      CategoryDownloader Category = "posix-downloader"
  )
  // Target gains: Category Category
  // Discover now returns ([]DiscoveredTarget, error) where each target carries its
  // declared category; a tests/test_*.sh file with a missing, malformed, or unknown
  // docket-suite declaration is a discovery ERROR (fail closed), never skipped.
  ```

- [ ] **Step 1: Write failing discovery tests** in `discover_test.go`:
  - a fixture dir whose files declare `go` / `posix-install` / `posix-downloader`
    discovers all, sorted, with correct categories;
  - a file with NO declaration → error naming the file (no generic/legacy category —
    acceptance 12, spec "no dormant compatibility execution branch");
  - a file declaring an unknown category (`# docket-suite: bash`) → error;
  - an empty dir → error (unchanged fail-closed rule);
  - a declaration below line 10 or with trailing text → error (bounded parse, no
    lenient fallback).
  Run `go test ./internal/suiterunner/ -run TestDiscover -count=1`; expect FAIL.
- [ ] **Step 2: Implement the category parser and discovery contraction.** Read the
  first 10 lines of each candidate; match `^# docket-suite: (go|posix-install|posix-downloader)$`.
  Remove nothing else from discovery's ordering/validation semantics
  (`ResolveTargets` whole-input-set validation stays).
- [ ] **Step 3: Remove the retired seams from the CLI wiring.**
  In `development_test_cmd.go`: delete the `os.Getenv("DOCKET_BASH_PATH")` read
  (`Bash: ""` → Run resolves bash on PATH; keep Run's existing unusable-bash exit-2).
  The `DOCKET_RUNTESTS_*` seams are runner-owned configuration, not facade plumbing —
  they stay. Hygiene preflight: `scripts/check-test-source-hygiene.sh` currently guards
  the *legacy* corpus's source hygiene. Disposition: port its still-meaningful checks
  (over the now-small surviving shell surface) into `internal/repoguard` as a Go test
  (extend Task 3's package), drop `Config.HygienePath` and the exit-5 preflight from the
  runner, and record in the evidence that exit code 5 is retired from the exit contract
  — update `run.go`'s contract comment and its tests accordingly. If the worker finds
  the hygiene checks are wholly mechanism-only (they only police deleted-corpus idioms),
  delete instead of port — justify from the Task 2 ledger either way.
- [ ] **Step 4: Re-prove the preserved guarantees** (acceptance 13, 14) — for each,
  point at the existing suiterunner test or write the missing one, and run a targeted
  mutation where the guarantee's test predates this task's edits (source-copy fidelity,
  per-target isolation, missing/duplicate-target fail-closure, no-valid-result exit 3,
  interruption 130/143, deterministic aggregation, budget screen-then-confirm,
  ADR-0074 tri-state, fail-closed discovery). The contraction may not convert any
  uncertainty into success.
- [ ] **Step 5: Run** `go test ./internal/suiterunner/ ./internal/cli/ -count=1`; expect
  PASS. NOTE: `docket development test` itself is now RED against the still-present
  legacy corpus (undeclared files fail closed). That is the planned transitional window
  — record it in evidence; Task 8 closes it.
- [ ] **Step 6: Update `tests/README.md`** (discovery rule, categories, retired exit 5
  if applicable, removal of the run-tests.sh section) and commit everything:
  `feat(0370): contract docket development test to Go plus two declared POSIX product categories`.

---

### Task 7: Generators and active integration material (Gate 4; acceptance 15, 16; part of 5)

**Recommended profile:** standard.

**Files:**
- Modify: every class-3 canonical generator from Task 1's ledger that emits the retired
  route (candidates the ledger will confirm: `scripts/runners/*` generator sources if
  still canonical after 0371, agent-wrapper/sync generators, `.docket.example.yml`
  templating, `docket development install` asset lists in `internal/cli/install.go`,
  any `internal/` embed/manifest that ships `scripts/docket.sh` or `scripts/lib/`).
- Modify: class-4 generated products — ONLY by regeneration, never by hand.
- Modify: class-5 active maintained prose — `CLAUDE.md` (the build-gate paragraph's
  "scripts/run-tests.sh stays present as the frozen parity oracle" sentence and any
  other retired-route instruction), `skills/*/SKILL.md` and references that actively
  instruct the retired route, `scripts/*.md` contracts of surviving scripts.

**Interfaces:**
- Consumes: Task 1 ledger classes 3/4/5.

- [ ] **Step 1: Correct each canonical generator first** (spec: generators before
  products). Generated dispatch must stay machine-neutral (ADR-0036): no
  checkout-specific or host-specific path may replace the facade reference
  (acceptance 16).
- [ ] **Step 2: Regenerate every affected product through its normal deterministic
  path, then regenerate AGAIN and require a clean `git status`** — repeat-generation
  cleanliness is the determinism proof (acceptance 15). A generator that is not
  reproducible blocks committing its product.
- [ ] **Step 3: Correct active prose.** Before editing, grep the test suite for each
  sentence being rewritten (learning `restatement-accumulates-its-own-guards`) and
  repoint any dependent assert at the surviving owner in the same commit.
- [ ] **Step 4: Verify** — `go test ./... -count=1` over touched packages; run any
  prose-sentinel test files touched, directly.
- [ ] **Step 5: Commit** `feat(0370): correct canonical generators and active material off the retired route`.

---

### Task 8: Delete the facade, runtime, legacy runner, and mechanism-only tests (Gate 5; acceptance 3, 4, 5, 6, 10)

**Recommended profile:** premium (the destructive step; discovery must be repeated
afterward and every deletion must trace to a ledger row).

**Files:**
- Delete (membership derived from Task 1/2 ledgers — the named seeds are anchors, not
  the boundary): `scripts/docket.sh`, `scripts/lib/` runtime tree, `scripts/run-tests.sh`,
  the facade-subcommand scripts and their `.md` contracts whose class is
  deleted-implementation, compatibility launchers, `scripts/check-test-source-hygiene.sh`
  (per Task 6's disposition), every class-D test file, their `tests/runtime-budgets.tsv`
  rows, and `tests/lib/` / `tests/fixtures/` members that served only deleted tests.
- Modify: mixed-responsibility files — split so surviving behavior stays with no
  dormant facade route (acceptance 6).

- [ ] **Step 1: Pre-delete audit.** For every file about to be deleted, confirm its
  ledger disposition: class D, or class A/B/C with a recorded
  `replaced by <TestName>, mutation FAIL observed` row. Any file or assertion block
  without such a row DOES NOT get deleted (acceptance 10: unresolved mixed/uncertain
  assertions stay). `git rm` only ledgered paths — never `git rm -r` a directory that
  still contains an unledgered file.
- [ ] **Step 2: Delete, and prune the budget registry** of exactly the deleted rows
  (the registry↔files correspondence guard from Task 3 verifies both directions).
- [ ] **Step 3: Repeat Task 1's discovery probes** over the post-deletion tree (spec:
  "then repeat discovery"). Every remaining match must classify as immutable history,
  frozen artifact, or structurally-explained false positive. A new class-1/9 finding
  BLOCKS.
- [ ] **Step 4: Verify the suite goes green end-to-end for the first time on the final
  topology:** `go run ./cmd/docket development test` — expect exit 0; the transitional
  red window is now closed. Record the run's summary lines (budget clauses included)
  in evidence.
- [ ] **Step 5: Commit** `refactor(0370): delete the frozen Bash facade, runtime tree, legacy runner, and mechanism-only tests`.

---

### Task 9: Final absence guards with mutation evidence (Gate 6; acceptance 17, 18, 19)

**Recommended profile:** max (the guard's whole value is its mutation-proven
sensitivity; a vacuous seal here is the change's worst failure mode).

**Files:**
- Create: `internal/repoguard/absence_test.go` (+ any helper in `repoguard.go`).

**Interfaces:**
- Consumes: `repoguard.MaintainedFiles` / `repoguard.ExecutableSurface` (Task 3).

- [ ] **Step 1: Write the guard.** One Go test (plus focused subtests) that walks the
  ACTIVE maintained surface (categorical exclusions per Global Constraints — immutable
  history and the frozen v0.9.2 corpus stay permitted) and fails on shape classes, each
  bounded on both sides:
  - direct invocation/reference of a path segment ending in the retired basenames
    (`docket.sh`, `run-tests.sh` under `scripts/`, `runtime.bash`);
  - sourcing shapes reaching `scripts/lib/` or a runtime-named file;
  - token-bounded occurrences of `DOCKET_SCRIPTS_DIR` / `DOCKET_BASH_PATH`;
  - variable-composed invocation shapes (a command word built by interpolation whose
    literal tail is a retired basename, e.g. `"$X/docket.sh"`, `${Y}run-tests.sh`) —
    and state in the guard's doc comment the composition forms it structurally cannot
    see (learning `byte-pattern-guard-matches-a-spelling`: assert the limitation in the
    header);
  - generator output: run each surviving canonical generator into a temp dir and scan
    its OUTPUT with the same shape classes (guards must "inspect generators and
    products");
  - unknown/unclassified: any scan error, unreadable file, or unparseable candidate is
    a test FAILURE with a diagnostic naming the path (fail closed; acceptance 17), and
    the guard separately asserts its scanned population is non-empty — an empty walk is
    an error, not a pass (population floor; learning
    `marker-scoped-guard-needs-a-population-floor`).
- [ ] **Step 2: Run the seven mutations** (spec's mutation set; protocol from Global
  Constraints, `-count=1`, backup-copy restore). Each must produce an observed FAIL,
  recorded in evidence:
  1. plant a direct facade invocation (`scripts/docket.sh preflight`) in a maintained `.sh`;
  2. plant a variable-composed invocation (`f=docket.sh; "$dir/scripts/$f"`);
  3. plant a runtime sourcing line (`. scripts/lib/docket-runtime.sh`);
  4. plant a retired-env dependence (`: "${DOCKET_SCRIPTS_DIR:?}"`);
  5. mutate a surviving generator to emit a forbidden command, regenerate to temp, scan;
  6. plant an unclassifiable candidate (e.g. an unreadable/permission-denied file in the
     scanned surface, or a malformed suite declaration where the guard parses one);
  7. plant a forbidden candidate in active *prose* executable surface
     (a fenced `scripts/docket.sh` command in a maintained `skills/**/SKILL.md`).
- [ ] **Step 3: Run the negative controls** (acceptance 19): assert GREEN with the
  untouched tree; plant nothing, confirm matches inside `docs/adrs/`, archived specs/
  plans, and `internal/repository/testdata/` are ignored by pointing the guard at a
  fixture tree containing exactly such files; and assert the frozen artifacts are
  byte-identical before/after the whole task (`git status` clean over
  `internal/repository/testdata/ docs/`).
- [ ] **Step 4:** `go test ./internal/repoguard/ -count=1` — PASS. Commit
  `test(0370): shape-derived, mutation-proven absence guards for the retired Bash control plane`.

---

### Task 10: ADR consequences, final regeneration, and whole-suite proof (Gate 7; acceptance 20, 21, 22)

**Recommended profile:** standard.

**Files:**
- Modify: `docs/adrs/` ONLY through the docket-adr workflow (status records + index
  regeneration) — never hand-edit Accepted bodies.

- [ ] **Step 1: Audit facade-era ADR promises.** Read ADRs 0014, 0029, 0030, 0033 (and
  any others Task 1's ledger flagged) plus what 0372's ADR audit already recorded.
  Identify only the status consequences that could not truthfully land before physical
  deletion (spec "ADR treatment"). ADRs 0036, 0074, 0099 remain Accepted and must
  remain TRUE — verify each against the final tree (0036: regenerate-and-diff shows
  machine-neutral dispatch; 0074: suiterunner tri-state tests green; 0099: no
  second metadata topology introduced).
- [ ] **Step 2: Dispatch the docket-adr workflow** (per AGENTS.md "Docket agents —
  dispatch, don't run inline": dispatch the registered `docket-adr` agent) with the
  concrete list: which ADR ids move to which terminal status
  (superseded-by-0370 / consequence note), and index regeneration + validation.
  Accepted history is never silently rewritten (acceptance 20).
- [ ] **Step 3: Final determinism pass.** Re-run every surviving canonical generator
  twice; require `git status` clean after the second run.
- [ ] **Step 4: Whole-suite proof.** `go run ./cmd/docket development test` from the
  feature worktree — expect exit 0. Inspect the budget clause lines: a
  `SERIAL CONFIRMED OVER BUDGET:` line is an authoritative breach to resolve before
  COMPLETE (acceptance 22); `BUDGET WATCH:`/`PARALLEL-SENSITIVE:` lines are screening
  findings to record.
- [ ] **Step 5: Final absence statement.** Re-run the Task 8 Step 3 discovery sweep one
  last time; record in evidence that every remaining match is history/frozen/structural
  false positive. Commit any workflow-produced artifacts:
  `docs(0370): facade-era ADR status consequences and final verification`.

---

## Self-Review (performed at plan-writing time)

- **Spec coverage:** Gates 1–7 map to Tasks 1, 2–5, 6, 7, 8, 9, 10. Acceptance 1–2
  (Task 1), 3–6 (Task 8), 7 (Task 2), 8 (Tasks 3–4), 9 (Task 5), 10 (Task 8), 11–14
  (Task 6), 15–16 (Task 7), 17–19 (Task 9), 20–21 (Task 10), 22 (Tasks 8/10),
  23 is a global prohibition (no task introduces a shim, release, or new shell control
  plane — nothing in this plan does).
- **No enumerated gate:** every file list in Tasks 5–8 is explicitly subordinated to
  the Task 1/2 ledgers; seeds are anchors, not boundaries.
- **Type consistency:** `repoguard.MaintainedFiles`/`ExecutableSurface` (Task 3) are
  what Tasks 6 (hygiene port), 9 consume; the `# docket-suite:` header regex is stated
  identically in Tasks 5 and 6; `Category` constants match the header vocabulary.
- **Known risk accepted:** the Task 6→8 transitional red window is documented in
  Global Constraints, Task 6 Step 5, and Task 8 Step 4.
