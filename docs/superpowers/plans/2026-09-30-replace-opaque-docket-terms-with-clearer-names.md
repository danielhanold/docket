<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0469 — Replace opaque docket terms with clearer names](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0469-replace-opaque-docket-terms-with-clearer-names.md)**
<!-- docket:backlink:end -->
# Readability renames (ADR-0129 family (e)) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. In this repo the plan is executed by `docket-build` (one tier worker per task, sequential, one full-suite gate at the end).

**Goal:** Deliver ADR-0129 family (e), rows 67-86, as a hard cut with no aliases. Seven wire tokens get plainer names and are sealed (rows 67-73). Eleven opaque concepts get plainer prose names across Go, skills, agent wrappers, generated dispatch material and docs (rows 74-84). Two names that match no code are retired to the glossary's "Obsolete terms" section (rows 85-86).

**Architecture:** A behavior-preserving rename, split by row group so a reviewer can accept or reject each task on its own. Each wire task (1-4) renames its token on **every** maintained surface that carries it: Go, tests, goldens, skills, agent wrappers, cursor rules, `.docket.example.yml` and docs. Its derivation grep must come back empty when the task commits. The concept tasks (5-7) do the same for their prose rows. Task 8 appends the seven wire rows to the existing `internal/repoguard` retired-vocabulary table. It adds one new match kind for row 68 and mutation-tests every row. Task 9 runs the closing whole-repo grep and the shard, kept-spelling and generated-material checks. Any task that edits `skills/`, `agents/`, `cursor-rules/` or `.docket.example.yml` regenerates the embedded bundle in the same commit. A task that changes an agent description or the run-tracker payload also regenerates the harness goldens and the AGENTS.md dispatch block from their generators.

**Tech Stack:** Go (`internal/domain`, `internal/render`, `internal/app`, `internal/gatedrive`, `internal/cli`, `internal/process`, `internal/reposetup`, `internal/workspace`, `internal/harness`, `internal/repoguard`), Markdown skills, agent wrappers, cursor rules and docs, `go generate ./internal/assets` (`cmd/genassets`), build tags `integration` and `e2e`, and the Go suite runner (`go run ./cmd/docket development test`).

**Spec:** `docs/superpowers/specs/2026-09-30-replace-opaque-docket-terms-with-clearer-names-design.md` on the `docket` metadata branch (synchronized copy at `/Users/homer/dev/docket/.docket/docs/superpowers/specs/2026-09-30-replace-opaque-docket-terms-with-clearer-names-design.md`). Authority for every name: ADR-0129, family (e), rows 67-86 (`/Users/homer/dev/docket/.docket/docs/adrs/0129-collision-free-docket-vocabulary.md`). The build does **not** edit ADR-0129.

**Worktree:** `/Users/homer/dev/docket/.worktrees/replace-opaque-docket-terms-with-clearer-names` (branch `refactor/replace-opaque-docket-terms-with-clearer-names`, base `366827eb540d22b7672f95b5ab1656ae54242491`). Every command below runs from this directory.

## Global Constraints

- **Behavior is unchanged.** Every rename keeps behavior identical. The same conditions produce the renamed token, and the same statuses, refusals and exit codes follow. Do not fix the semantics you notice along the way. Note it under `NOTES` in the task report instead.
- **Hard cut, no aliases** (ADR-0129 Decision 2). Old spellings are not accepted, not mapped, and not mentioned in refusal or help text. `docket change repair-identity` stops existing.
- **Wire-token map** (rows 67-73; sealed in Task 8):

  | Row | Old | New | Go identifiers (ADR-0129: "Go identifiers follow their row's term") |
  |---|---|---|---|
  | 67 | readiness `needs-brainstorm` (board cell, `status` readiness value, the composed refusal reason `not-ready-needs-brainstorm`) | `needs-grooming` (`not-ready-needs-grooming`) | `ReadyNeedsBrainstorm` → `ReadyNeedsGrooming` |
  | 68 | gate-drive halt cause `identity-mismatch` | `worktree-changed` | none (string literals only) |
  | 69 | `unresolved-execution` (halt cause and ownership error kind) | `launch-unconfirmed` | `ErrUnresolvedExecution` → `ErrLaunchUnconfirmed` |
  | 70 | op `change.repair-identity` / CLI `docket change repair-identity` | `change.relink` / `docket change relink` | `OperationChangeRepairIdentity` → `OperationChangeRelink`, `RepairIdentity` → `Relink`, `RepairIdentityRequest` → `RelinkRequest`, `RepairIdentityResult` → `RelinkResult`, `newRepairIdentitySubcommand` → `newRelinkSubcommand` |
  | 71 | result tokens `repaired-branch` / `repaired-pr` | `relinked-branch` / `relinked-pr` | `RepairRepairedBranch` → `RepairRelinkedBranch`, `RepairRepairedPR` → `RepairRelinkedPR` (the sibling `Repair*` reason constants keep their names) |
  | 72 | finalize merge condition `pr-identity-mismatch` | `pr-link-mismatch` | `PRIdentityMatch` → `PRLinkMatch` |
  | 73 | `evidence.recertify` reason `identity-drift` | `certified-input-changed` | `ReasonRecertifyIdentityDrift` → `ReasonRecertifyCertifiedInputChanged` |

- **Concept map** (rows 74-84; prose and Go identifiers, never a wire token):

  | Row | Old | New |
  |---|---|---|
  | 74 | unmet conjuncts / conjunct / conjunction; Go `MergeConjuncts` | unmet conditions / condition; `MergeConditions` |
  | 75 | admission slot / worktree admission slot | worktree slot |
  | 76 | liveness transition | moved to background ("liveness probe" is unchanged) |
  | 77 | native supervisor | gate supervisor |
  | 78 | gate execution | gate run |
  | 79 | presence-encoded section / marker | marker section |
  | 80 | Step-0 preamble | startup check |
  | 81 | closed vocabulary | allowed values |
  | 82 | compare-and-swap (prose) | conflict-checked write (adjective form "conflict-checked <noun>" is fine; the `contended` token and any `*-cas` wire token are unchanged) |
  | 83 | pay per relevance | read on demand |
  | 84 | identity repair; finalize's identity checkpoint | relink; finalize's link check |

- **Retirements** (rows 85-86): the Bash-era bootstrap verdict names `BOOTSTRAP=` / `PROCEED` / `STOP_MIGRATE` / `CREATE_ORPHAN`, and `docket status --digest-only` / "digest-only read". Prose describes `repository.prepare`'s dispositions and plain `docket status` instead; the names move to the glossary's "Obsolete terms" section.
- **Explicitly kept (ADR-0129 "Kept in family (e)" and "Explicitly not renamed"; never rename):**
  - The worktree slot state `unresolved`, the stage `mark-worktree-execution-unresolved`, and the distinct existing halt cause `launch-unresolved`.
  - The `--adopt-pr-head`, `--adopt-pr`, `--expect-*` flags and the other relink result tokens `stale-evidence`, `workspace-conflict`, `candidate-branch-absent`, `pr-unknown`, `invalid-request`, and their Go constants (`RepairStaleEvidence`, …).
  - "identity" in its which-record sense: `scope-identity-mismatch` / `ErrScopeIdentityMismatch`, `results-identity-broken`, `identity-reused`, `identity-mutated`, the `status` JSON `identity` key, the process-lock `identity` stage, `fingerprint-mismatch` / `ErrFingerprintMismatch`, `scopeIdentityMatch`, "supervised identity drifted" test messages, "identity-mismatched repository definition" (`docs/install/codex.md`), the finalize step-12 binary "identity check", and "identity mismatch branch/worktree/…" test-case names that describe a scope mismatch.
  - The `skills.brainstorm` key, the `docket-brainstorm` / `docket-brainstorm-consultant` names, and "brainstorm" as the name of the design conversation. Only the readiness token changes.
  - Config keys, agent names, frontmatter fields (Decision 9), the protocol-v1 `disposition` key, `.vocabularies` in `docket schema`, `run-record-cas` and every other `*-cas` token, `contended`.
  - File names: `skills/docket-build/references/gate-execution.md`, `gate-execution-evidence.md`, `internal/app/change_repair*.go`, and the fixture file `internal/render/testdata/board/corpus/active/0003-needs-brainstorm-change.md`. They are not in any row; only their prose changes.
  - "Step 0" / "Step-0" as a step number (for example "the Step-0 `repository.prepare` operation", implement-next's "step-0 implementation preflight"). Change it only where the sentence names the preamble itself.
  - The integration-repair "repair" sense (`repair-needs-signoff`, "repair sign-off", `docket-integration-repair`). Row 84 renames only the *identity* repair.
- **Frozen, never edited:** `docs/changes/**`, `docs/results/**`, `docs/superpowers/**` (except this plan), `docs/adrs/**` (ADR-0129 included), `testdata/repositories/**`, `internal/repository/testdata/**`, `internal/install/legacydata/**`, `internal/install/testdata/**`, `tests/fixtures/**`, and the history comments in `internal/repoguard/budgets_test.go`. Expected-output goldens are not frozen: `internal/render/testdata/board/board.golden` (+ its `PROVENANCE.md` line) is updated by hand in Task 1, and `internal/harness/*/testdata/golden/**` is regenerated with `-update` only where a task says so.
- **Regenerated, never hand-edited:**
  - `internal/assets/embedded/**`: `go generate ./internal/assets`, then `go run ./cmd/genassets -repo . -check` must print nothing and exit 0. Run it in the same commit as any edit under `skills/`, `agents/`, `cursor-rules/` or `.docket.example.yml`.
  - The AGENTS.md `docket:dispatch` block (`CLAUDE.md` is a symlink to it): regenerate from `harness.DispatchInterior(harness.RunTracker(embedded catalog))` with the throwaway helper in Task 5. This repo's `agent_harnesses` are `claude` and `opencode` (no `codex`), and the committed block at base is byte-identical to `DispatchInterior` of `cursor-rules/run-tracker.md` (verified at base). Never hand-edit the block.
  - Harness goldens: `go test -count=1 ./internal/harness/claude ./internal/harness/codex ./internal/harness/cursor ./internal/harness/opencode -update`, only in the tasks that say so, and only after the embedded bundle is regenerated. Then read `git diff --stat -- internal/harness` and `git diff -- internal/harness`: every changed line must be a rename from that task's map. Anything else is a stop-and-investigate.
- **Site derivation:** derive every site by whole-repo grep (AGENTS.md: "Never hand-list the sites of a literal or an operation you are gating"). The site lists below were traced at base `366827eb5` and are starting points. The derivation command in each task and the closing grep in Task 9 are authoritative. The maintained pathspec used throughout (run under `bash -c` or in a bash script, since it is an array):

  ```bash
  MAINT=(-- . ':!docs/changes' ':!docs/results' ':!docs/superpowers' ':!docs/adrs' \
    ':(exclude,glob)**/testdata/**' ':!tests/fixtures' ':!internal/install/legacydata' \
    ':!internal/assets/embedded')
  ```

- **Budgets:** `internal/repoguard/budgets_test.go` ceilings are **never raised**. Most swaps keep the word count (`wc -w` counts `needs-grooming`, `worktree-changed`, `launch-unconfirmed` as one word each). Four swaps grow it: "compare-and-swap" → "conflict-checked write" (+1), "presence-encoded" → "marker section" (+1), "liveness transition" → "moved to background" (+1), and "Identity repair checkpoint" → "Link check" (−1). Where a file would cross its ceiling, compensate by trimming a redundant word in the same sentence, or use the adjective form ("the conflict-checked final push"). Measured at base (lines/words vs ceiling): `docket-groom-next/SKILL.md` **78/78, 2082/2082 (at both ceilings)**; `docket-build/references/gate-caller-loop.md` 151/175, **1871/1872**; `docket-auto-groom/SKILL.md` 66/70, **1625/1627**; `docket-implement-next/SKILL.md` **214/214**, 8167/8175; `docket-implement-next/references/edge-paths.md` **118/118**, 1550/1554; `docket-finalize-change/SKILL.md` 238/239, 5643/5647; `docket-new-change/SKILL.md` 59/61, 1674/1675; `docket-build/SKILL.md` **439/439**, 4477/4479; `docket-build-task/SKILL.md` **211/211**, 2231/2235; `docket-convention/SKILL.md` 390/400, 7856/7969; `docket-status/SKILL.md` 129/140, 2974/3065; `docket-adr/SKILL.md` 92/110, 1362/1600; `docket-build/references/gate-execution.md` 164/170, 1486/1520; `docket-build/references/gate-execution-evidence.md` 102/110, 998/1050; `docket-finalize-change/references/gate-failure.md` 145/147, 1894/1901. Never add a line to a file at its line ceiling. The AGENTS.md dispatch block has its own word budget (`TestDispatchBlockBudget`, 1154); "conjuncts" → "conditions" keeps it.
- **Build tags:** integration tests carry `//go:build integration` and `internal/app/finalize_e2e_test.go` carries `//go:build e2e`. A plain `go test` or `go vet` never compiles them. Every task that renames a Go identifier runs all three: `go vet ./...`, `go vet -tags integration ./...` and `go vet -tags e2e ./internal/app/`.
- **Integration shards:** each `tests/test_go_integration_*.sh` selects tests by `SHARD_PREFIX`. A renamed `TestIntegration*` keeps its shard's prefix (for example `TestIntegrationRecordOpsRepairIdentity…` → `TestIntegrationRecordOpsRelink…`; checked in Task 9). For an edited integration-tagged test, run exactly its enclosing functions: `go test -count=1 -tags integration -run '^(TestA|TestB)$' ./internal/<pkg>/`.
- **Focused verification only.** Per-task verification uses focused `go test` packages. The build gate runs the whole suite once at the end (`build.test_command`, `go run ./cmd/docket development test`); do not run it per task.
- **Pre-existing red at base (out of scope, do not fix, do not "repair"):** four `internal/repoguard` tests fail at `366827eb5` because base commit `366827eb5` ("regenerate docket-managed block in AGENTS.md", after `a96558229` dropped `codex` from `agent_harnesses`) removed the Codex root-entry clause from AGENTS.md: `TestCommittedCodexDispatchMatchesGenerator`, `TestCommittedCodexDispatchRoutesEveryScope`, `TestCommittedCodexDispatchObservesYieldedEntrySession`, `TestCodexRequestFileCarriesRunID`. Every repoguard run in this plan therefore passes `-skip 'TestCommittedCodexDispatch|TestCodexRequestFileCarriesRunID'`. Task 6 still updates the phrase those tests pin, so they stay consistent with the generator. Report the pre-existing red in the task `NOTES`. The final gate will show it. The fix is a human decision (restore the Codex clause or scope those tests to repos that opt into Codex) and does not belong to this change.
- Go test commands always pass `-count=1` (a cached `ok (cached)` is not evidence). Run `gofmt -l` on every Go file you touch and expect no output.
- Stage explicit paths only (`git add -- <path> …`), never `git add -A` / `git add .`. Use `git add -u -- <dir>` only for directories this task regenerated (`internal/assets/embedded`, `internal/harness`).
- Mutation tests restore from a backup copy (`cp f f.bak; mutate; test; mv -f f.bak f`), never `git checkout -- f`. Confirm each mutation landed with a count before you read the test result. Count through a whitespace-flattened copy when the phrase can wrap (learning *phrase-grep-over-wrapped-prose*).
- Shell: the agent's interactive shell is zsh and its `grep` is ugrep. Run multi-line shell steps under `bash -c`, use `git grep` or `command grep` for verification, and capture producer output into a variable before grepping it (AGENTS.md pipefail rule). Write a grep pattern that starts with `-` as `-e <pat>`.
- Glossary anchors: `docs/reference/glossary.md` has an "Alphabetical index" of `[Heading](#anchor)` links plus in-body links. When a task renames a heading, it updates every link to the old anchor (`git grep -n '#<old-anchor>'`) and moves the index line into alphabetical order. GitHub anchor rule: lowercase, drop punctuation except `-` and `_`, spaces become `-`.
- Test code in this plan is unverified until run. Prove each new assert can pass, and mutation-test its key where the task says so.
- The build gate runs the whole suite, not only the tests named here. Read its budget report even when it is green.

## Review Focus

1. **A hyphenated compound that contains a retired token.** `scope-identity-mismatch` (kept) and `pr-identity-mismatch` (row 72) both contain row 68's `identity-mismatch`. `not-ready-needs-brainstorm` contains row 67's token. Expected: the seal treats `scope-identity-mismatch` as clean, attributes `pr-identity-mismatch` only to row 72, and attributes `not-ready-needs-brainstorm` to row 67. Task 8 pins all three with controls and a kind-swap mutation.
2. **A kept which-record "identity" gets renamed.** Renaming `scope-identity-mismatch`, `fingerprint-mismatch`, `results-identity-broken`, the `identity` JSON key or the process-lock stage would break wire contracts and stored state. Expected: each kept spelling's maintained-source count is identical to base (Task 9 Step 4 diffs the counts).
3. **A rename missed in an integration- or e2e-tagged file, or a renamed integration test that leaves its shard.** Expected: all three vet passes run in every identifier task, and Task 9 Step 5 checks every `TestIntegration*` name against the shard prefixes.
4. **A consumer still names a retired spelling.** For example, finalize still tells a human to run `change.repair-identity` (now an unknown operation), or a skill tells an agent to select `needs-brainstorm` changes (none exist). Expected: none survives in skills, agents, cursor rules, AGENTS.md, generator output or non-test Go string literals (Task 8 seal). Docs and comments are covered by Task 9's closing grep.
5. **Generated material drifts from its generator.** A hand-edited embedded bundle, AGENTS.md block or harness golden would pass locally and then drift on the next `docket install`. Expected: `genassets -check` is clean, AGENTS.md equals the regenerated `DispatchInterior`, and the golden diffs contain only mapped renames (Tasks 1, 5, 6, 9).

---

### Task 1: Readiness `needs-brainstorm` → `needs-grooming` (row 67)

**Build tier:** standard — one wire token across Go (domain, render, app), goldens, six skills, one agent wrapper, one cursor rule, `.docket.example.yml` and docs, plus regenerating the embedded bundle and harness goldens.

**Files (traced at base; re-derive):**
- Modify: `internal/domain/readiness.go`, `internal/render/board.go`, `internal/app/status.go` (`readinessReason` prose), `internal/app/change_groom.go` (comments)
- Test: `internal/domain/readiness_test.go`, `internal/domain/actions_test.go` (`not-ready-needs-brainstorm`), `internal/render/board_test.go`, `internal/render/testdata/board/board.golden`, `internal/render/testdata/board/PROVENANCE.md`, `internal/app/change_groom_test.go`, `internal/app/change_groom_integration_test.go`, `internal/app/implementation_context_test.go`, `internal/app/change_integration_test.go`
- Modify (prose/consumers): `skills/docket-auto-groom/SKILL.md`, `skills/docket-convention/SKILL.md`, `skills/docket-groom-next/SKILL.md`, `skills/docket-implement-next/SKILL.md`, `skills/docket-new-change/SKILL.md`, `skills/docket-status/SKILL.md`, `agents/docket-auto-groom.md`, `cursor-rules/dispatch/docket-auto-groom.md`, `.docket.example.yml`, `docs/concepts/change-lifecycle.md`, `docs/guide/capturing-work.md`, `docs/guide/designing-before-building.md`, `docs/reference/config-keys.md`, `docs/reference/skills-and-agents.md`, `docs/reference/glossary.md`
- Regenerate: `internal/assets/embedded/**`, `internal/harness/{claude,codex,cursor,opencode}/testdata/golden/docket-auto-groom.*`

**Interfaces:**
- Produces: `domain.ReadyNeedsGrooming ReadinessKind = "needs-grooming"`; `status --json` `.changes[].readiness == "needs-grooming"`; board Proposed cell `needs-grooming`; refusal reason `not-ready-needs-grooming`.

- [ ] **Step 1: Derive the sites.**

```bash
bash -c 'MAINT=(-- . ":!docs/changes" ":!docs/results" ":!docs/superpowers" ":!docs/adrs" ":(exclude,glob)**/testdata/**" ":!tests/fixtures" ":!internal/install/legacydata" ":!internal/assets/embedded"); git grep -n -i -E -e "needs.brainstorm|NeedsBrainstorm" "${MAINT[@]}"; git grep -n -i -e "needs-brainstorm" -- internal/render/testdata/board internal/harness'
```

Expected: the files listed above (~45 files, ~120 lines), plus the board golden/PROVENANCE and four harness goldens. Classify each hit: the readiness token, its Go identifier, or prose naming the readiness state all move. A prose "Needs brainstorm" test-fixture title (`board_test.go`) becomes "Needs grooming". The local variable `needsBrainstorm` becomes `needsGrooming`.

- [ ] **Step 2: Write the failing tests.** Change every test expectation first:
  - `internal/domain/readiness_test.go`: expected kind `needs-grooming`, identifier `ReadyNeedsGrooming`.
  - `internal/domain/actions_test.go`: `wantReason: "not-ready-needs-grooming"` (the reason is composed as `"not-ready-"+string(readiness.Kind)` in `internal/domain/actions.go`, so no code change there).
  - `internal/render/board_test.go`: every `needs-brainstorm` cell expectation → `needs-grooming`; the case label `"proposed needs-brainstorm"` → `"proposed needs-grooming"`; `needsBrainstorm` → `needsGrooming`; fixture titles "Needs brainstorm" → "Needs grooming".
  - `internal/render/testdata/board/board.golden`: the one Proposed row's last cell `needs-brainstorm |` → `needs-grooming |` (leave the fixture link path `active/0003-needs-brainstorm-change.md` alone; it is a fixture file name). `PROVENANCE.md`: "`needs-brainstorm` (no spec, not trivial)" → "`needs-grooming` (no spec, not trivial)".
  - `internal/app/*_test.go` sites: identifier `domain.ReadyNeedsGrooming` and literal `needs-grooming`.

- [ ] **Step 3: Run them to verify they fail.**

Run: `go test -count=1 ./internal/domain/ ./internal/render/`
Expected: FAIL (compile error `undefined: domain.ReadyNeedsGrooming` / `ReadyNeedsGrooming`, or golden mismatch).

- [ ] **Step 4: Rename the code.**
  - `internal/domain/readiness.go`: `ReadyNeedsBrainstorm ReadinessKind = "needs-brainstorm"` → `ReadyNeedsGrooming ReadinessKind = "needs-grooming"`; comment "missing design reports needs-brainstorm" → "needs-grooming"; the use site `kind := ReadyNeedsGrooming`.
  - `internal/render/board.go`: the comment table row `needs-brainstorm → "needs-brainstorm"` → `needs-grooming → "needs-grooming"`; `case domain.ReadyNeedsGrooming: return "needs-grooming", nil`.
  - `internal/app/status.go`: `case domain.ReadyNeedsGrooming: return "needs grooming before it can be built"`.
  - `internal/app/change_groom.go`: comments "a needs-brainstorm change" → "a needs-grooming change".

- [ ] **Step 5: Rename every consumer.** Replace the token in every skill, agent wrapper, cursor rule, `.docket.example.yml` comment and doc from Step 1. Keep the agent description in `agents/docket-auto-groom.md` byte-identical to the `description:` in `skills/docket-auto-groom/SKILL.md` (both say "the auto-groomable needs-grooming queue"). In `docs/reference/glossary.md`, rename the heading `### Readiness: build-ready / needs-brainstorm / not-proposed` → `### Readiness: build-ready / needs-grooming / not-proposed`, its body ("**Needs-grooming** is a proposed change …", "only needs-grooming changes can be groomed"), and the index link `#readiness-build-ready--needs-brainstorm--not-proposed` → `#readiness-build-ready--needs-grooming--not-proposed`. In `docs/concepts/change-lifecycle.md`, keep the ASCII diagram's column alignment: `needs-grooming` is two characters shorter than `needs-brainstorm`, so pad with two `─` characters so the arrows still line up.

- [ ] **Step 6: Regenerate the embedded bundle and the harness goldens.**

```bash
go generate ./internal/assets && go run ./cmd/genassets -repo . -check
go test -count=1 ./internal/harness/claude ./internal/harness/codex ./internal/harness/cursor ./internal/harness/opencode -update
git diff -- internal/harness | command grep -E '^[+-][^+-]'
```

Expected: `-check` prints nothing. The golden diff shows only `docket-auto-groom.*` lines where `needs-brainstorm` became `needs-grooming`.

- [ ] **Step 7: Run the focused tests and confirm the derivation is empty.**

```bash
go vet ./... && go vet -tags integration ./... && go vet -tags e2e ./internal/app/
go test -count=1 ./internal/domain/ ./internal/render/ ./internal/assets/ ./internal/harness/...
go test -count=1 ./internal/app/ -run 'Groom|Status|Readiness|ImplementationContext'
go test -count=1 ./internal/repoguard/ -skip 'TestCommittedCodexDispatch|TestCodexRequestFileCarriesRunID'
```

Then run the enclosing `TestIntegration*` functions of every edited site in `internal/app/change_integration_test.go` and `change_groom_integration_test.go` (see Global Constraints). Re-run Step 1's grep. Expected: PASS everywhere; the grep prints nothing.

- [ ] **Step 8: Commit.**

```bash
git add -- internal/domain internal/render internal/app skills agents cursor-rules .docket.example.yml docs/concepts docs/guide docs/reference
git add -u -- internal/assets/embedded internal/harness
git commit -m "refactor: rename readiness needs-brainstorm to needs-grooming (change 0469, ADR-0129 row 67)"
```

---

### Task 2: Gate-drive halt causes `worktree-changed` and `launch-unconfirmed` (rows 68-69)

**Build tier:** standard — string tokens and one exported error-kind identifier across `internal/gatedrive`, `internal/app` and `internal/cli`, three skill references, and a glossary entry rewrite that needs sense judgment (the which-record "identity" stays).

**Files (traced at base; re-derive):**
- Modify: `internal/gatedrive/driver.go` (the two `"identity-mismatch"` fingerprint halts in the slice loop and the recheck helper; the `"unresolved-execution"` halts and returns; comments), `internal/gatedrive/takeover.go` (the `"identity-mismatch"` halt after `scopeIdentityMatch`), `internal/gatedrive/ownership.go` (`ErrUnresolvedExecution OwnershipErrorKind = "unresolved-execution"` and comments), `internal/gatedrive/admission.go`, `internal/gatedrive/incumbent.go`, `internal/gatedrive/drive.go` (comment "identity drift"), `internal/app/gate_drive.go` (`case gatedrive.ErrUnresolvedExecution`, `ownershipNextAction`), `internal/app/gate.go` (comments), `internal/app/finalize_rebase.go` and `internal/app/runtracker_continuation.go` (comments "identity drift" meaning this halt)
- Test: `internal/gatedrive/{admission,drive,driver,driver_runfence,driver_concurrency,driver_faults,history,incumbent,takeover,slot_run_fence}_test.go`, `internal/gatedrive/sequence_race_integration_test.go`, `internal/app/gate_drive_test.go`, `internal/app/gate_integration_test.go`, `internal/app/runtracker_verdict_integration_test.go`
- Modify (prose): `skills/docket-build-task/SKILL.md`, `skills/docket-build/SKILL.md` ("identity drift" in the HALTED list), `skills/docket-build/references/gate-caller-loop.md`, `skills/docket-finalize-change/references/gate-failure.md`, `docs/reference/glossary.md`

**Interfaces:**
- Produces: `gatedrive.ErrLaunchUnconfirmed OwnershipErrorKind = "launch-unconfirmed"`; HALTED causes `"worktree-changed"` and `"launch-unconfirmed"`.

**Trace note (record it in the task report's `NOTES`):** the takeover path (`internal/gatedrive/takeover.go`, the halt after `!scopeIdentityMatch(...)`) emits the **same** `identity-mismatch` token when the resolved drive's recorded repo/branch/worktree/change/task/phase disagrees with the scope. That is a which-record mismatch, not a fingerprint change, and ADR-0129 row 68 does not describe it. This task renames it to `worktree-changed` like every other site, which keeps behavior identical (consumers see one token for both conditions, as today). Splitting the two conditions into different tokens would need an ADR-0129 `## Update` under its *Deviations* rule, so it is a follow-up for a human, not this change.

- [ ] **Step 1: Derive the sites.**

```bash
bash -c 'MAINT=(-- . ":!docs/changes" ":!docs/results" ":!docs/superpowers" ":!docs/adrs" ":(exclude,glob)**/testdata/**" ":!tests/fixtures" ":!internal/install/legacydata" ":!internal/assets/embedded"); git grep -n -E -e "(^|[^A-Za-z0-9_-])identity-mismatch([^A-Za-z0-9_-]|$)|unresolved-execution|UnresolvedExecution" "${MAINT[@]}"; git grep -n -i -E -e "identity drift|identity mismatch" "${MAINT[@]}"'
```

The first grep's hits all move. The bounded pattern already excludes `scope-identity-mismatch`, `pr-identity-mismatch` (Task 4) and "identity-mismatched". Classify the second grep's spaced prose. Where it names **this halt** (`internal/gatedrive/drive.go` "HALTED: … identity drift", `skills/docket-build/SKILL.md` and `gate-caller-loop.md` HALTED lists, `internal/app/finalize_rebase.go` and `runtracker_continuation.go` cause lists, `internal/gatedrive/driver_test.go` case `name: "identity mismatch"` and message "identity drift must never be reported as red"), it becomes "a changed worktree" / "worktree changed". Where it names a scope or record identity (`takeover_test.go` "identity mismatch branch/worktree/…", `driver.go` "an identity mismatch" in the adopter comments, `install/collection_journal_test.go`, "supervised identity drifted"), keep it.

- [ ] **Step 2: Write the failing tests.** Change every expected cause string `"identity-mismatch"` → `"worktree-changed"` (`driver_test.go` wantCause, `takeover_test.go` five `want:` values, `drive_test.go` `Cause:`, `runtracker_verdict_integration_test.go` `takeoverCause` and the `res.Reason` assert and message), and every `"unresolved-execution"` → `"launch-unconfirmed"` and `ErrUnresolvedExecution` → `ErrLaunchUnconfirmed` in the tests. In `admission_test.go`, the history seeds (`seedMixedHistory`'s cause list, its `halted-<cause>` directory names, and the cause list near the top of the file) use the cause as an incidental label. Rename `identity-mismatch` → `worktree-changed` there too, and keep `launch-unresolved` (a different, kept cause).

- [ ] **Step 3: Run them to verify they fail.**

Run: `go test -count=1 ./internal/gatedrive/`
Expected: FAIL (`undefined: ErrLaunchUnconfirmed`, or cause mismatches).

- [ ] **Step 4: Rename the code.** In `ownership.go`: `ErrLaunchUnconfirmed OwnershipErrorKind = "launch-unconfirmed"` with its comment reworded ("nothing proved whether a launch happened: a lost launch response, or a crash between reserving the worktree and attaching the process"). Replace every `ErrUnresolvedExecution` reference in non-test Go. Replace the halt literals `"identity-mismatch"` → `"worktree-changed"` (`driver.go` ×2, `takeover.go` ×1) and `"unresolved-execution"` → `"launch-unconfirmed"` (`driver.go`). Update the comments that quote the tokens (`worktree-busy / unresolved-execution` → `worktree-busy / launch-unconfirmed`, and so on). Keep `mark-worktree-execution-unresolved` and the slot state `unresolved`.

- [ ] **Step 5: Rename the prose.** Skills: `skills/docket-build-task/SKILL.md` ("reason `worktree-busy` (or `launch-unconfirmed`)"), `skills/docket-build/references/gate-caller-loop.md` (the `unresolved-execution` bullet → `launch-unconfirmed`, reworded "nothing proved whether a launch happened (a lost launch response, …)"), `skills/docket-finalize-change/references/gate-failure.md` (two sites), and the HALTED lists that say "identity drift" → "a changed worktree". Glossary: rewrite `### Identity mismatch / identity drift` as `### Worktree changed / certified input changed`:

  ```markdown
  ### Worktree changed / certified input changed

  The state a gate checked no longer matches the state now in front of it. A gate drive halts
  `worktree-changed` when the worktree fingerprint (HEAD, index, status, live file bytes) moved
  since the drive started, or when a takeover finds a drive whose recorded branch, worktree or
  change is not the scope's. `evidence.recertify` refuses `certified-input-changed` when the PR
  head or the build command moved after the gate passed. In finalize, a pull request whose pushed
  head no longer equals the branch finalize just rebased and retested is refused `pr-head-mismatch`.

  **Used for:** refusing to certify or merge something that was not verified. It is a halt, never a
  red suite. Realign the pushed head (or undo the stray edit), then name the change id to run
  finalize again.
  ```

  `pr-head-mismatch` is the real finalize token (`ReasonRebasePRHeadMismatch`, `ReasonPublishPRHeadMismatch` in `internal/app`). Update the index link `#identity-mismatch--identity-drift` → `#worktree-changed--certified-input-changed` and re-sort it under W. In the `### Drive disposition` entry, "identity drift" → "a changed worktree". The glossary's `unresolved-execution` mentions (four) → `launch-unconfirmed`, where they describe the slot's admission refusal. Regenerate the bundle (`go generate ./internal/assets && go run ./cmd/genassets -repo . -check`).

- [ ] **Step 6: Run the focused tests and confirm the derivation.**

```bash
go vet ./... && go vet -tags integration ./... && go vet -tags e2e ./internal/app/
go test -count=1 ./internal/gatedrive/ ./internal/cli/ ./internal/assets/
go test -count=1 ./internal/app/ -run 'GateDrive|Gate'
go test -count=1 -tags integration -run '^TestIntegrationRunVerdict' ./internal/app/
go test -count=1 -tags integration -run 'Sequence' ./internal/gatedrive/
go test -count=1 ./internal/repoguard/ -skip 'TestCommittedCodexDispatch|TestCodexRequestFileCarriesRunID'
```

Also run the enclosing functions of the edited sites in `internal/app/gate_integration_test.go`. Re-run Step 1's first grep: it must print nothing.

- [ ] **Step 7: Commit.**

```bash
git add -- internal/gatedrive internal/app internal/cli skills docs/reference/glossary.md
git add -u -- internal/assets/embedded
git commit -m "refactor: rename gate-drive halt causes to worktree-changed and launch-unconfirmed (change 0469, ADR-0129 rows 68-69)"
```

---

### Task 3: `change.relink` and finalize's link check (rows 70, 71, 84)

**Build tier:** standard — an operation id, CLI verb, schema registry entry, capability catalog and asset-independence allowlist, exported Go types, two result tokens, and the finalize skill's human-gated recovery section. A miss fails loudly (catalog and CLI tests), but the finalize prose needs care to leave the integration-repair "repair" sense untouched.

**Files (traced at base; re-derive):**
- Modify: `internal/app/change_repair.go` (op const, request/result types, `RepairIdentity`, result-token consts, error prefixes `"repair-identity: …"` → `"relink: …"`, commit subject `"change %04d identity repaired (%s)"` → `"change %04d relinked (%s)"`, comments "identity repair" / "identity checkpoint"), `internal/app/schema_registry.go` (`{ID: "change.relink", Request: RelinkRequest{}, Result: RelinkResult{}}, // Relink`), `internal/app/status.go` (the remedy string `run: docket change relink --id …` and its comment), `internal/cli/change.go` (`newRelinkSubcommand`, `Use: "relink"`, `Short`, `capability("change.relink", EffectMetadataWrite)`, the doc comment), `internal/cli/install.go` (asset-independence key `"change relink": true`), `internal/domain/finalize.go` (comment "identity repair is meaningless …" → "a relink is meaningless …")
- Test: `internal/app/change_repair_test.go`, `internal/app/change_repair_integration_test.go`, `internal/app/change_integration_test.go`, `internal/app/named_branch_facts_test.go`, `internal/app/schema_revision_test.go`, `internal/app/status_branch_malformed_test.go`, `internal/app/status_human_test.go`, `internal/cli/change_test.go`, `internal/cli/capability_production_test.go`, `internal/cli/revision_rename_test.go`, `internal/repoguard/finalize_rebuild_test.go` (`rebuildTerminator = "## Link check"`)
- Modify (prose): `skills/docket-finalize-change/SKILL.md` (`## Identity repair checkpoint` → `## Link check`; the two `change.repair-identity` commands → `change.relink`; "After a successful repair" → "After a successful relink"; "the repair op itself proves" → "the relink op itself proves"; "from the repair op" → "from the relink op"; "never repair autonomously" → "never relink autonomously"), `docs/reference/glossary.md` (`### Identity repair (\`change repair-identity\`)` → `### Relink (\`change relink\`)`, body "After a relink, …", the example command, index link `#identity-repair-change-repair-identity` → `#relink-change-relink`, re-sorted under R), `docs/comparison/ai-native-sdlc-playbook.md` ("blocked and identity repair" → "blocked and relink")
- Do **not** touch `internal/repoguard/retired_vocabulary_test.go` here (Task 8 rewrites its two `repair-identity` control lines).

**Interfaces:**
- Produces: `app.OperationChangeRelink = "change.relink"`; `app.Relink(ctx, deps FinalizeDeps, repoDir string, req RelinkRequest) RelinkResult`; `app.RelinkRequest`, `app.RelinkResult`; `app.RepairRelinkedBranch = "relinked-branch"`, `app.RepairRelinkedPR = "relinked-pr"`; CLI `docket change relink` with the unchanged flags `--id --expect-revision --adopt-pr-head --expect-pr --expect-head --adopt-pr --expect-branch`.

- [ ] **Step 1: Derive the sites.**

```bash
bash -c 'MAINT=(-- . ":!docs/changes" ":!docs/results" ":!docs/superpowers" ":!docs/adrs" ":(exclude,glob)**/testdata/**" ":!tests/fixtures" ":!internal/install/legacydata" ":!internal/assets/embedded"); git grep -n -i -E -e "repair-identity|RepairIdentity|repaired-branch|repaired-pr|RepairRepaired|identity repair|identity-repair|identity checkpoint|identity repaired" "${MAINT[@]}"'
```

Expected: the files above, plus `internal/repoguard/retired_vocabulary_test.go` (left for Task 8).

- [ ] **Step 2: Write the failing tests.** In `internal/cli/change_test.go`, rename `TestChangeRepairIdentityRegistered` → `TestChangeRelinkRegistered`, `TestChangeRepairIdentityFlagsRequired` → `TestChangeRelinkFlagsRequired`, `TestChangeRepairIdentityReachesOperation` → `TestChangeRelinkReachesOperation`, and point them at `[]string{"change", "relink"}`, `assetIndependent["change relink"]` and `"operation":"change.relink"`. Add the hard-cut test next to them:

```go
// TestChangeRelinkRetiresRepairIdentity is the ADR-0129 row 70 hard cut: the
// catalog lists change.relink and no longer lists change.repair-identity, and the
// old verb is not a registered subcommand.
func TestChangeRelinkRetiresRepairIdentity(t *testing.T) {
	out, errS, code := runCLI(t, "capabilities", "--json")
	if code != 0 {
		t.Fatalf("capabilities exited %d: %s", code, errS)
	}
	if !strings.Contains(out, `"id":"change.relink"`) {
		t.Errorf("capability catalog lacks change.relink")
	}
	if strings.Contains(out, "repair-identity") {
		t.Errorf("capability catalog still names the retired repair-identity")
	}
	root := captureTree(t)
	if cmd, _, err := root.Find([]string{"change", "repair-identity"}); err == nil && cmd != nil && cmd.Name() == "repair-identity" {
		t.Errorf("change repair-identity is still a registered subcommand")
	}
}
```

`captureTree` (used by the existing registration test) and `runCLI` (`internal/cli/root_test.go`) are the file's own helpers, and the catalog JSON is compact (`"id":"change.relink"`, no spaces). In `internal/cli/capability_production_test.go`, rename the map key `"change.repair-identity"` → `"change.relink"` and its comment. In `internal/cli/revision_rename_test.go`, point every `repair-identity` path at `relink` and its messages. In the app tests, rename identifiers and expected tokens (`relinked-branch`, `relinked-pr`, `"operation":"change.relink"`, the status remedy text). Rename `TestIntegrationRecordOpsRepairIdentityUnrelatedInvalidRecord{Progress,Refusals}` → `TestIntegrationRecordOpsRelinkUnrelatedInvalidRecord{Progress,Refusals}` (same shard prefix). Set `rebuildTerminator = "## Link check"` in `finalize_rebuild_test.go`.

- [ ] **Step 3: Run them to verify they fail.**

Run: `go test -count=1 ./internal/cli/ -run 'Relink|Capability|Revision'`
Expected: FAIL (`change relink` not registered, or compile errors).

- [ ] **Step 4: Rename the code** per the Interfaces block and the Files list. The receipt struct `changeRepairReceipt` keeps its name and fields; its `Op` value becomes `OperationChangeRelink`, so new relink commits carry `change.relink` in their receipt and transaction trailer (old metadata commits keep their old receipts; Decision 3 leaves committed history alone). Reword the file header comment ("This file is `change relink`: the revision-pinned relink that finalize's link check hands a human's decision to."). Keep the internal helpers (`repairResolveMode`, `repairViewPR`, …) and the file names.

- [ ] **Step 5: Rename the prose** in `skills/docket-finalize-change/SKILL.md`, the glossary entry and the comparison doc, as listed in Files. Keep "repair sign-off", `repair-needs-signoff` and every integration-repair mention. Regenerate the bundle (`go generate ./internal/assets && go run ./cmd/genassets -repo . -check`).

- [ ] **Step 6: Run the focused tests and confirm the derivation.**

```bash
go vet ./... && go vet -tags integration ./... && go vet -tags e2e ./internal/app/
go test -count=1 ./internal/cli/ ./internal/domain/ ./internal/assets/
go test -count=1 ./internal/app/ -run 'Relink|Repair|Schema|Status'
go test -count=1 -tags integration -run '^TestIntegrationRecordOpsRelink' ./internal/app/
go test -count=1 ./internal/repoguard/ -skip 'TestCommittedCodexDispatch|TestCodexRequestFileCarriesRunID'
```

Also run the enclosing functions of the edited sites in `internal/app/change_integration_test.go` and `change_repair_integration_test.go`. Re-run Step 1's grep: only `internal/repoguard/retired_vocabulary_test.go` may remain.

- [ ] **Step 7: Commit.**

```bash
git add -- internal/app internal/cli internal/domain internal/repoguard/finalize_rebuild_test.go skills/docket-finalize-change docs/reference/glossary.md docs/comparison/ai-native-sdlc-playbook.md
git add -u -- internal/assets/embedded
git commit -m "refactor: rename change repair-identity to change relink (change 0469, ADR-0129 rows 70, 71, 84)"
```

---

### Task 4: `pr-link-mismatch` and `certified-input-changed` (rows 72-73)

**Build tier:** standard — two wire tokens with their Go identifiers, finalize merge condition tables in two packages, and one skill sentence.

**Files (traced at base; re-derive):**
- Modify: `internal/domain/finalize.go` (field `PRIdentityMatch` → `PRLinkMatch`; `FirstFailure` returns `"pr-link-mismatch"`), `internal/app/finalize_merge.go` (the assembly `PRLinkMatch: in.prNumber == in.canonicalPRNumber`, the two `case "pr-identity-mismatch"` → `"pr-link-mismatch"`), `internal/app/evidence_recertify.go` (`ReasonRecertifyCertifiedInputChanged = "certified-input-changed"`, three uses, the comment)
- Test: `internal/domain/finalize_test.go` (the row `{"pr-identity", … "pr-identity-mismatch"}` → `{"pr-link", func(m *MergeConjuncts) { m.PRLinkMatch = false }, "pr-link-mismatch"}`), `internal/app/finalize_merge_integration_test.go` (`{"pr-identity", …, "pr-identity-mismatch"}` → `{"pr-link", …, "pr-link-mismatch"}`), `internal/app/finalize_context_test.go`, `internal/app/evidence_recertify*_test.go` (expected reason; the message "head-disagreement/identity-drift refusal" → "head-disagreement/certified-input-changed refusal")
- Modify (prose): `skills/docket-finalize-change/SKILL.md` (the merge-condition list "implemented, PR identity, heads agree, …" → "implemented, PR link, heads agree, …"). "a wrong PR identity" sentences that mean *which PR* stay.

**Interfaces:**
- Consumes: `domain.MergeConjuncts` (renamed in Task 5; use the current name here).
- Produces: `domain.MergeConjuncts.PRLinkMatch bool`; merge refusal reason `pr-link-mismatch`; `app.ReasonRecertifyCertifiedInputChanged = "certified-input-changed"`.

- [ ] **Step 1: Derive the sites.**

```bash
bash -c 'MAINT=(-- . ":!docs/changes" ":!docs/results" ":!docs/superpowers" ":!docs/adrs" ":(exclude,glob)**/testdata/**" ":!tests/fixtures" ":!internal/install/legacydata" ":!internal/assets/embedded"); git grep -n -E -e "pr-identity-mismatch|PRIdentityMatch|identity-drift|IdentityDrift" "${MAINT[@]}"; git grep -n -i -e "PR identity" -- skills internal/app/finalize_merge.go internal/domain/finalize.go'
```

`TestRunVerifyPRIdentityForms` / `TestIntegrationChangeRuntimeRunVerifyPRIdentityForms` test run.verify's pr-form matching (which PR), not this merge condition. Keep their names.

- [ ] **Step 2: Write the failing tests** (the test edits in Files).

- [ ] **Step 3: Run them to verify they fail.**

Run: `go test -count=1 ./internal/domain/ -run 'Merge'`
Expected: FAIL (`unknown field PRLinkMatch` or reason mismatch).

- [ ] **Step 4: Rename the code** (Files list). Check whether any schema vocabulary lists recertify reasons or merge reasons: `git grep -n -e "ReasonRecertify" -e "pr-identity" -- internal/app/schema*.go internal/app/vocab*.go`. If one does, it derives from the constants and needs no edit. If it lists spellings, rename them there too.

- [ ] **Step 5: Rename the skill sentence** and regenerate the bundle (`go generate ./internal/assets && go run ./cmd/genassets -repo . -check`).

- [ ] **Step 6: Run the focused tests and confirm the derivation.**

```bash
go vet ./... && go vet -tags integration ./... && go vet -tags e2e ./internal/app/
go test -count=1 ./internal/domain/ ./internal/assets/
go test -count=1 ./internal/app/ -run 'Finalize|Merge|Recertify'
go test -count=1 -tags integration -run '^TestIntegrationFinalizeMerge' ./internal/app/
go test -count=1 -tags integration -run '^TestIntegrationEvidence' ./internal/app/
go test -count=1 ./internal/repoguard/ -skip 'TestCommittedCodexDispatch|TestCodexRequestFileCarriesRunID'
```

Re-run Step 1's first grep: it must print nothing.

- [ ] **Step 7: Commit.**

```bash
git add -- internal/domain internal/app skills/docket-finalize-change
git add -u -- internal/assets/embedded
git commit -m "refactor: rename pr-identity-mismatch and identity-drift (change 0469, ADR-0129 rows 72-73)"
```

---

### Task 5: "conjunct" → "condition" everywhere, including `MergeConditions` and the dispatch block (row 74)

**Build tier:** standard — a wide, mechanical identifier and comment rename (≈300 lines in ≈60 maintained files) that fails loudly on a missed identifier, plus regenerating the AGENTS.md dispatch block and cursor golden from the run-tracker payload.

**Files (traced at base; re-derive):**
- Go identifiers: `MergeConjuncts` → `MergeConditions` (type and its `AllHold` / `FirstFailure` methods keep their names), `mergeConjuncts` → `mergeConditions`, `mergeConjunctInputs` → `mergeConditionInputs`, `mergeConjunctMessage` → `mergeConditionMessage`, `mergeConjunctOutcome` → `mergeConditionOutcome`, `RunVerifyConjunct` → `RunVerifyCondition` (its `json:"unmet"` tag is unchanged), `identityConjunction` → `identityConditions` (`internal/process/lock.go`, `launch.go`). Test names: `Conjuncts` → `Conditions`, `Conjunct` → `Condition`, `Conjunction` → `Conditions` (for example `TestMergeConjunctsFirstFailure` → `TestMergeConditionsFirstFailure`, `TestIdentityConjunctionRejectsOwnGroup` → `TestIdentityConditionsRejectOwnGroup`, `TestIntegrationEvidenceRunVerifyMissingResultsIsUnmetConjunct` → `…IsUnmetCondition`). Every renamed `TestIntegration*` keeps its shard prefix.
- Go comments and test messages: `internal/app/{agent_guardian,change_implemented,change_reclaim,change_repair,finalize_block,finalize_merge,finalize_publish,finalize_rebase,pr_publish,repository_migrate,run_verify,runtracker_cancel,runtracker_verdict}.go` and their tests, `internal/domain/{actions,finalize,lease}.go` and tests, `internal/cli/finalize.go`, `internal/process/{ids,launch,lock,observe,paths,records,stop}.go` and tests, `internal/reposetup/{classify,health,healthconditions}.go` and tests, `internal/workspace/rebasereceipt.go` and `inspect_integration_test.go`.
- Generator source and generated material: `cursor-rules/run-tracker.md` ("the id and unmet conjuncts it names" → "the id and unmet conditions it names"), then the regenerated AGENTS.md dispatch block, `internal/assets/embedded/**` and any harness golden that embeds the run tracker.
- Prose: `skills/docket-finalize-change/SKILL.md` ("an open-children conjunct" → "an open-children condition", "every merge conjunct" → "every merge condition", "that conjunct's closed token" → "that condition's token", "a child conjunct" → "a child condition"), `skills/docket-finalize-change/references/gate-failure.md` (3), `skills/docket-implement-next/SKILL.md` (5), `skills/docket-implement-next/references/edge-paths.md` (1), `docs/reference/glossary.md` (`run-retry-once` row), `docs/comparison/ai-native-sdlc-playbook.md`.
- Never edit `internal/install/legacydata/**` or `internal/install/testdata/**` (frozen legacy dispatch blocks that still say "conjuncts"; the installer recognizes old blocks by them).

**Interfaces:**
- Consumes: `domain.MergeConjuncts.PRLinkMatch` from Task 4.
- Produces: `domain.MergeConditions` with fields `Implemented, PRLinkMatch, HeadsAgree, OpenNonDraft, BaseIsEffectiveBase, GateSatisfied, ApprovalSatisfied, NoOpenChildren, NotSuperseded bool` and methods `AllHold() bool`, `FirstFailure() string`; `app.RunVerifyCondition{Reason, Observed}`.

- [ ] **Step 1: Derive the sites.**

```bash
bash -c 'MAINT=(-- . ":!docs/changes" ":!docs/results" ":!docs/superpowers" ":!docs/adrs" ":(exclude,glob)**/testdata/**" ":!tests/fixtures" ":!internal/install/legacydata" ":!internal/assets/embedded"); git grep -c -i -e "conjunct" "${MAINT[@]}"; git grep -h -o -i -E -e "[A-Za-z_]*conjunct[A-Za-z_]*" "${MAINT[@]}" | sort | uniq -c'
```

Expected at base: about 64 files, and the word forms `conjunct`, `conjuncts`, `conjunction`, `Conjunct`, `CONJUNCT` plus the identifiers above. No Go string literal or JSON tag contains "conjunct" (checked at base), so this row changes no wire token.

- [ ] **Step 2: Write the failing test.** Rename `domain.MergeConjuncts` → `domain.MergeConditions` in `internal/domain/finalize_test.go` (and `TestMergeConjuncts*` → `TestMergeConditions*`).

- [ ] **Step 3: Run it to verify it fails.**

Run: `go test -count=1 ./internal/domain/ -run 'MergeConditions'`
Expected: FAIL (`undefined: MergeConditions`).

- [ ] **Step 4: Rename the identifiers** in non-test and test Go (Files list). Prefer `gofmt -r` for the type (`gofmt -r 'MergeConjuncts -> MergeConditions' -w <files>`), then rename the rest by hand. Doc comments: "conjunct" → "condition", "conjuncts" → "conditions", "conjunction" → "conditions" (or "all of the conditions" where it reads better), "each conjunct" → "each condition". Keep sentence sense: "the closed set of preconditions a merge requires" stays (that is not row 81's "closed vocabulary").

- [ ] **Step 5: Rename the prose and the generator source.** Edit the skills, `docs/reference/glossary.md`, the comparison doc, and `cursor-rules/run-tracker.md` (Files list). Then regenerate the embedded bundle:

```bash
go generate ./internal/assets && go run ./cmd/genassets -repo . -check
```

- [ ] **Step 6: Regenerate the AGENTS.md dispatch block from its generator.** Create a throwaway helper (it is never committed):

```go
//go:build regenagents

// File: internal/repoguard/zz_regen_agents_tmp_test.go (THROWAWAY — delete after use)
package repoguard

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/assets"
	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/harness"
)

// TestRegenerateAgentsDispatchBlock rewrites AGENTS.md's docket:dispatch block
// from the generator, the way the repo phase of `docket install` does for a repo
// whose agent_harnesses exclude codex (this repo: claude, opencode).
func TestRegenerateAgentsDispatchBlock(t *testing.T) {
	p := filepath.Join(guardRoot(t), "AGENTS.md")
	src, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := document.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	cat, err := assets.EmbeddedCatalog()
	if err != nil {
		t.Fatal(err)
	}
	rt, err := harness.RunTracker(cat)
	if err != nil {
		t.Fatal(err)
	}
	var patch document.PatchSet
	patch.ReplaceBlock("dispatch", harness.DispatchInterior(rt))
	out, err := doc.Apply(patch)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, out, 0o644); err != nil {
		t.Fatal(err)
	}
}
```

```bash
go test -count=1 -tags regenagents -run '^TestRegenerateAgentsDispatchBlock$' ./internal/repoguard/
rm -f internal/repoguard/zz_regen_agents_tmp_test.go
git status --porcelain -- internal/repoguard   # must not list zz_regen_agents_tmp_test.go
git diff -- AGENTS.md
```

Expected: `git diff -- AGENTS.md` shows exactly one changed line pair, `unmet conjuncts` → `unmet conditions`, inside the dispatch block. Any other diff means the helper did not match the committed generator shape. Stop and investigate; do not hand-edit.

- [ ] **Step 7: Regenerate the harness goldens** (the cursor dispatch rule and any golden that embeds the run tracker):

```bash
go test -count=1 ./internal/harness/claude ./internal/harness/codex ./internal/harness/cursor ./internal/harness/opencode -update
git diff -- internal/harness | command grep -E '^[+-][^+-]'
```

Expected: an empty diff, or only `conjuncts` → `conditions` lines.

- [ ] **Step 8: Run the focused tests and confirm the derivation.**

```bash
go vet ./... && go vet -tags integration ./... && go vet -tags e2e ./internal/app/
go test -count=1 ./internal/domain/ ./internal/process/ ./internal/reposetup/ ./internal/workspace/ ./internal/cli/ ./internal/harness/... ./internal/reposeed/ ./internal/install/ ./internal/assets/
go test -count=1 ./internal/app/ -run 'Finalize|Merge|RunVerify|Implemented|Reclaim|Publish|Guardian|Cancel|Verdict'
go test -count=1 -tags integration -run '^(TestIntegrationFinalizeMerge|TestIntegrationEvidence|TestIntegrationChangeRuntime)' ./internal/app/
go test -count=1 ./internal/repoguard/ -skip 'TestCommittedCodexDispatch|TestCodexRequestFileCarriesRunID'
```

`TestDispatchBlockBudget` must stay green at 1154 words. Re-run Step 1's first grep: it must print nothing.

- [ ] **Step 9: Commit.**

```bash
git add -- internal/domain internal/app internal/cli internal/process internal/reposetup internal/workspace cursor-rules/run-tracker.md AGENTS.md skills docs/reference/glossary.md docs/comparison/ai-native-sdlc-playbook.md
git add -u -- internal/assets/embedded internal/harness
git commit -m "refactor: rename conjunct to condition and MergeConjuncts to MergeConditions (change 0469, ADR-0129 row 74)"
```

---

### Task 6: Concept renames — worktree slot, moved to background, gate supervisor, gate run, marker section, allowed values, conflict-checked write, read on demand (rows 75-79, 81-83)

**Build tier:** standard — judgment wording across Go comments, one generator constant (`CodexRootEntryClause`), skills, docs and the glossary, where every hit must be sorted into the renamed concept or a kept sense.

**Files (traced at base; re-derive):**
- Row 75 "admission slot": `internal/app/gate_drive.go` (the next-action string "an execution occupies this worktree's admission slot …" → "… this worktree's slot …"; it is prose, not a token), `internal/app/runtracker_cancel.go`, `internal/gatedrive/ownership.go`, gatedrive tests' comments, `docs/concepts/run-tracker.md`, glossary (`### Admission slot` → `### Worktree slot`; four in-body links `#admission-slot` → `#worktree-slot`).
- Row 76 "liveness transition": `internal/harness/dispatch.go` (`CodexRootEntryClause`: "A shell-tool yield carrying a live task/session identity is a liveness transition, not completion." → "A shell-tool yield carrying a live task/session identity means the command moved to background, not that it completed."), `internal/repoguard/root_entry_dispatch_test.go` (the pinned phrase → "shell-tool yield carrying a live task/session identity means the command moved to background, not that it completed"), `skills/docket-status/SKILL.md` ("that is a liveness transition, not completion" → "that is a move to background, not completion"), `internal/repoguard/prose_contracts_test.go` (row `change_0389_sweep_scope`: "a liveness transition, not completion" → "a move to background, not completion"), glossary (`### Liveness probe / liveness transition` → `### Liveness probe / moved to background`, body "A command **moved to background** is a harness moving a still-running command into the background.").
- Row 77 "native supervisor": `internal/app/runtracker_continuation.go`, `internal/app/runtracker_continue.go`, `internal/cli/run.go` (comments), gatedrive integration tests' comments, `tests/test_go_integration_gatedrive_process.sh` (comment), `internal/app/workflow_e2e_test.go`, glossary.
- Row 78 "gate execution": `.docket.example.yml`, `internal/app/gate_drive.go`, `internal/gatedrive/{admission,ownership}.go`, `internal/gatedrive/driver_test.go`, `internal/repoguard/gatelaunch_admission_test.go` (comment), `skills/docket-build/SKILL.md` (heading `### Gate execution posture` → `### Gate run posture`, and "exactly as *Gate execution posture* describes"), `skills/docket-build/references/gate-execution.md` (title "# Gate execution — …" → "# Gate run — …", "§ *Gate execution posture*" → "§ *Gate run posture*"), `skills/docket-build/references/gate-execution-evidence.md` (title and "§ *Gate execution posture* clause 4"), `skills/docket-build/references/gate-caller-loop.md`, `skills/docket-finalize-change/SKILL.md` ("`docket-build`'s *Gate execution posture*" → "*Gate run posture*"), `docs/concepts/run-tracker.md`, glossary (`### Native supervisor / gate execution` folds into `### Gate run / run dir`: its body moves there, reworded with "gate supervisor"; delete the old heading and its index line; the `### Gate run / run dir` body "launched under docket's native supervisor" → "launched under the gate supervisor").
- Row 79 "presence-encoded": `internal/app/{change_groom,change_kill,finalize_rebase,finalize_reserve}.go` comments, `internal/app/change_groom_test.go` message ("left the marker section behind"), `internal/app/{finalize_closeout,workflow}_integration_test.go` messages ("(marker-section state)"), `skills/docket-convention/SKILL.md` ("**Presence-encoded state**" → "**Marker-section state**"), `docs/concepts/two-branches.md` ("a presence-encoded marker" → "a marker section"), `docs/comparison/ai-native-sdlc-playbook.md`, glossary (`### Presence-encoded section` → `### Marker section`).
- Row 81 "closed vocabulary": `internal/app/{maintenance,maintenance_assess}.go`, `internal/cli/{capability,change,schema}.go`, `internal/cli/capability_production_test.go` (message "outside the allowed values"), `internal/domain/{finalize,lease}.go`, `internal/githubcli/{merge,mergemethod}.go` and `mergemethod_test.go`, `internal/render/board.go`, `skills/docket-convention/SKILL.md` (two: "an effect outside the allowed values (`read` | …)" and "Its **disposition** takes the allowed values `applied` | `no-op` | `refused` | `error`"), `skills/docket-convention/references/stacked-changes.md`, `skills/docket-status/SKILL.md`, `docs/reference/cli.md` ("emit request/result payload schemas and allowed values"), `docs/reference/outcomes.md` (the quote of the convention's disposition sentence must match the new convention text verbatim; the pointer to "Step-0 preamble" is Task 7's), glossary (`### Closed vocabulary (operation dispositions)` → `### Allowed values (operation dispositions)`, body "A fixed, schema-published set of allowed tokens" stays).
- Row 82 "compare-and-swap": `internal/app/{runtracker_fence,runtracker_run_record,runtracker_store}.go`, `internal/app/app_concurrency_race_integration_test.go`, `internal/gatedrive/{admission,ownership,scope,store,suitebudget}.go` (comments), `skills/docket-adr/SKILL.md`, `skills/docket-build/references/gate-caller-loop.md` (2), `skills/docket-groom-next/SKILL.md` (at its word ceiling: "is the compare-and-swap that protects the write" → "is the conflict check that protects the write" keeps the count), `skills/docket-implement-next/SKILL.md`, `docs/concepts/{change-lifecycle,two-branches}.md`, `docs/guide/building-without-supervision.md`, `docs/comparison/ai-native-sdlc-playbook.md`, glossary (`### Compare-and-swap (CAS) / push-retry` → `### Conflict-checked write / push-retry`; body "A **conflict-checked write** is a write that succeeds only if …"; "the final-push CAS already protects it" → "the conflict-checked final push already protects it"). The abbreviation "CAS" in prose goes with it. The wire token `run-record-cas` and Go identifiers containing `CAS`/`Cas` stay.
- Row 83 "pay per relevance": `skills/docket-convention/SKILL.md` ("**Read contract — read on demand.**"), `docs/guide/remembering-why.md` ("- **Read on demand.** …"), glossary (`### Learnings index / pay per relevance` → `### Learnings index / read on demand`; "**Read on demand** is the read rule").

**Interfaces:**
- Produces: `harness.CodexRootEntryClause` with the sentence "A shell-tool yield carrying a live task/session identity means the command moved to background, not that it completed." (every other sentence unchanged).

- [ ] **Step 1: Derive the sites.**

```bash
bash -c 'MAINT=(-- . ":!docs/changes" ":!docs/results" ":!docs/superpowers" ":!docs/adrs" ":(exclude,glob)**/testdata/**" ":!tests/fixtures" ":!internal/install/legacydata" ":!internal/assets/embedded"); git grep -n -i -E -e "admission slot|liveness transition|native supervisor|gate execution|presence-encoded|closed vocabular|compare-and-swap|\bCAS\b|pay.per.relevance" "${MAINT[@]}"'
```

Sort each hit. The `\bCAS\b` hits are prose only where they abbreviate compare-and-swap in a sentence; leave identifiers and the `*-cas` tokens.

- [ ] **Step 2: Write the failing tests.** Update the two pinned phrases first: `internal/repoguard/prose_contracts_test.go` row `change_0389_sweep_scope` → `"a move to background, not completion"`, and `internal/repoguard/root_entry_dispatch_test.go` → `"shell-tool yield carrying a live task/session identity means the command moved to background, not that it completed"`. Add a dispatch-constant assert to `internal/harness/dispatch_test.go`:

```go
// TestCodexRootEntryClauseSaysMovedToBackground pins ADR-0129 row 76 in the
// generator: a yielded live task is a command moved to background, and the
// retired "liveness transition" wording is gone.
func TestCodexRootEntryClauseSaysMovedToBackground(t *testing.T) {
	if !strings.Contains(CodexRootEntryClause, "means the command moved to background, not that it completed") {
		t.Errorf("CodexRootEntryClause lost the moved-to-background clause")
	}
	if strings.Contains(CodexRootEntryClause, "liveness transition") {
		t.Errorf("CodexRootEntryClause still says liveness transition")
	}
}
```

- [ ] **Step 3: Run them to verify they fail.**

Run: `go test -count=1 ./internal/harness/ -run 'CodexRootEntryClause' && go test -count=1 ./internal/repoguard/ -run 'TestProseContracts'`
Expected: FAIL (the clause and the skill still say "liveness transition"). The prose-contract test name may differ; find it with `git grep -n "func Test" internal/repoguard/prose_contracts_test.go`.

- [ ] **Step 4: Apply the renames** from the Files list: Go comments and the next-action string, `CodexRootEntryClause`, skills (headings and every pointer to a renamed heading in the same commit), `.docket.example.yml`, docs, glossary (headings, bodies, merged entry, every `#anchor` link, index re-sorted). Keep word counts within the ceilings (Global Constraints: `gate-caller-loop.md` has 1 word of slack and two compare-and-swap sites; use the adjective form or trim a word).

- [ ] **Step 5: Regenerate** the embedded bundle and the harness goldens:

```bash
go generate ./internal/assets && go run ./cmd/genassets -repo . -check
go test -count=1 ./internal/harness/claude ./internal/harness/codex ./internal/harness/cursor ./internal/harness/opencode -update
git diff -- internal/harness | command grep -E '^[+-][^+-]'
```

Expected: golden changes, if any, are only this task's renames (for example a Codex golden carrying `CodexRootEntryClause`). AGENTS.md needs no regeneration: it carries no Codex clause in this repo, and the run-tracker payload does not change here.

- [ ] **Step 6: Run the focused tests and confirm the derivation.**

```bash
go vet ./... && go vet -tags integration ./... && go vet -tags e2e ./internal/app/
go test -count=1 ./internal/harness/... ./internal/reposeed/ ./internal/install/ ./internal/gatedrive/ ./internal/cli/ ./internal/domain/ ./internal/githubcli/ ./internal/render/ ./internal/assets/
go test -count=1 ./internal/app/ -run 'Gate|Groom|Kill|Maintenance|Cancel|Continu'
go test -count=1 ./internal/repoguard/ -skip 'TestCommittedCodexDispatch|TestCodexRequestFileCarriesRunID'
```

Also run the enclosing functions of any edited integration-tagged message sites. Re-run Step 1's grep: only kept senses may remain (none expected besides identifiers and `*-cas` tokens).

- [ ] **Step 7: Commit.**

```bash
git add -- internal skills .docket.example.yml tests/test_go_integration_gatedrive_process.sh docs/concepts docs/guide docs/reference docs/comparison
git add -u -- internal/assets/embedded internal/harness
git commit -m "docs: rename opaque gate, slot, section and write concepts (change 0469, ADR-0129 rows 75-79, 81-83)"
```

---

### Task 7: Startup check, and retiring the bootstrap verdicts and `--digest-only` (rows 80, 85, 86)

**Build tier:** standard — a convention heading plus every skill pointer to it in one commit, a rewrite of the Bootstrap guard paragraph that must match `repository.prepare`'s real dispositions, and glossary Obsolete-terms moves.

**Files (traced at base; re-derive):**
- Row 80: `skills/docket-convention/SKILL.md` (heading `### Step-0 preamble (every operating skill)` → `### Startup check (every operating skill)`; "(the *Step-0 preamble*)" → "(the *startup check*)"; "the Step-0 disposition the *Step-0 preamble* acts on" is rewritten in the row 85 paragraph below), and every pointer: `skills/docket-adr/SKILL.md`, `skills/docket-auto-groom/SKILL.md`, `skills/docket-groom-next/SKILL.md`, `skills/docket-new-change/SKILL.md`, `skills/docket-status/SKILL.md` ("run its *startup check*"), `skills/docket-implement-next/SKILL.md` ("Run the convention's **startup check**"), `skills/docket-finalize-change/SKILL.md` ("follow its **Startup check (every operating skill)**"); `internal/repoguard/capability_surface_test.go` (`capabilitySurfaceRemedy` and the floor message: "docket-convention's startup check"); `docs/reference/outcomes.md` ("owned by the `docket-convention` skill's startup check", and its verbatim quote of the convention's disposition sentence, which Task 6 changed to "allowed values"); glossary (`### Step-0 preamble` → `### Startup check`; index link `#step-0-preamble` → `#startup-check`, re-sorted under S).
- Row 85: `skills/docket-convention/SKILL.md` Bootstrap guard section (the 2×2 table and the paragraph after it), `docs/reference/harness/validation-runbook.md` (the two `BOOTSTRAP=PROCEED` passages), glossary (`### Bootstrap guard` rewritten; `### Bootstrap verdict: …` moved to "Obsolete terms"; the in-body link and index line for it removed or repointed).
- Row 86: `docs/guide/capturing-work.md` (the `--digest-only` block), glossary (`### Digest / digest-only read` split: a live `### Backlog digest` entry stays in place, and a `### Digest-only read (\`docket status --digest-only\`)` entry moves to "Obsolete terms").

- [ ] **Step 1: Derive the sites.**

```bash
bash -c 'MAINT=(-- . ":!docs/changes" ":!docs/results" ":!docs/superpowers" ":!docs/adrs" ":(exclude,glob)**/testdata/**" ":!tests/fixtures" ":!internal/install/legacydata" ":!internal/assets/embedded"); git grep -n -i -E -e "step-0 preamble|preamble\*|BOOTSTRAP=|STOP_MIGRATE|CREATE_ORPHAN|digest-only|digest only" "${MAINT[@]}"'
```

"digest only" in `docs/guide/capturing-work.md` ("These narrow the **digest only**") is plain English about which output a filter narrows. Reword it anyway ("These narrow only the **digest**") so the closing grep stays empty.

- [ ] **Step 2: Write the failing test.** Change `internal/repoguard/capability_surface_test.go`'s two messages to "startup check". They are messages, so they do not fail on their own. Run the capability-surface test afterwards to prove it still finds the convention's `docket capabilities --json` bootstrap under the renamed heading: `go test -count=1 ./internal/repoguard/ -run 'CapabilitySurface'` (find the exact test name with `git grep -n "func Test" internal/repoguard/capability_surface_test.go`).

- [ ] **Step 3: Rename the heading and every pointer in one edit pass** (row 80). Re-run the derivation: no `Step-0 preamble` may remain.

- [ ] **Step 4: Rewrite the Bootstrap guard** (row 85) in `skills/docket-convention/SKILL.md`. The code is authoritative. `internal/app/repository_prepare.go` refuses a fresh repo (`repository-fresh`, remedy `docket repository init`), a legacy one (`repository-legacy`, remedy `docket repository migrate`) and a half-migrated one (`migration-incomplete`, remedy `docket repository migrate`). It never creates the orphan branch implicitly. Replace the table and the paragraph after it with:

  ```markdown
  | | `LIVE` | `¬LIVE` |
  |---|---|---|
  | **`¬DOCKET`** | existing single-branch repo → `refused` (`repository-legacy`), remedy `docket repository migrate`; never auto-create or move data | fresh repo → `refused` (`repository-fresh`), remedy `docket repository init` |
  | **`DOCKET`** | **half-migrated** (interrupted run) → `refused` (`migration-incomplete`), remedy `docket repository migrate` to finish its prune | migrated → `applied` / `no-op` |

  `repository.prepare` computes this 2×2 **fail-closed** and reports it as the disposition the *startup check* acts on: `applied`/`no-op` when the repository is migrated, otherwise `refused` (or `error`) with a finding whose remedy names the human-typed `docket repository migrate` or `docket repository init`. Probe mechanics, the read-only export default, and the guarded bootstrap write path live in `repository.prepare`. The migration itself lives in the human-typed `docket repository migrate`; creating the metadata branch lives in the human-typed `docket repository init`.
  ```

  Before committing, confirm the three finding codes and their remedies with `git grep -n -e "FCRepositoryFresh" -e "FCRepositoryLegacy" -e "FCMigrationIncomplete" -- internal/app/repository_prepare.go internal/app/finding_codes.go`. Keep the paragraph above the table (the `DOCKET` / `LIVE` probe definitions) unchanged. `docket-convention/SKILL.md` has 10 lines and 113 words of slack. In `docs/reference/harness/validation-runbook.md`, `scripts/docket.sh` no longer exists (the Bash runtime is gone), so replace the step-2 fixture bootstrap with the Go path: run `docket repository init --repo-dir .`, then `docket repository prepare --repo-dir . --json`, and expect disposition `applied`. Replace the "Pass when" clause "`docket.sh preflight` runs to a `BOOTSTRAP=PROCEED` block" with "`docket repository prepare --repo-dir . --json` reports disposition `applied` or `no-op`". Check each flag against `docket repository init --help` and `docket repository prepare --help` before writing it. Change nothing else in the runbook.

- [ ] **Step 5: Fix `--digest-only`** (row 86) in `docs/guide/capturing-work.md`. `docket status` is read-only (`docket status --help`: "Report backlog status, readiness, selection, and repository health (read-only)"):

  ```bash
  # 1. the exact inventory. `docket status` is a read-only read: it writes nothing.
  docket status --type untyped
  ```

- [ ] **Step 6: Glossary.**
  - `### Step-0 preamble` → `### Startup check` (body unchanged except its first words: "The fixed startup check every operating skill runs before anything else …").
  - `### Bootstrap guard`: rewrite the body around the dispositions ("… and reports the result as `repository.prepare`'s disposition: `applied`/`no-op` when the repository is migrated, otherwise `refused` with a remedy naming the human-typed `docket repository init` or `docket repository migrate`."). Drop the link to the verdict entry.
  - Move `### Bootstrap verdict: \`PROCEED\` / \`STOP_MIGRATE\` / \`CREATE_ORPHAN\`` into "## Obsolete terms" as `### Bootstrap verdicts (\`BOOTSTRAP=\`)`: "The Bash-era bootstrap guard printed a `BOOTSTRAP=` line whose value was `PROCEED`, `STOP_MIGRATE` or `CREATE_ORPHAN`. The Go binary has no such line; see [Bootstrap guard](#bootstrap-guard)."
  - `### Digest / digest-only read` → keep a live `### Backlog digest` entry in place ("The **backlog digest** is the structured `status` payload: … `docket status` is always a write-free read.") and move `### Digest-only read (\`docket status --digest-only\`)` to "Obsolete terms": "A Bash-era `docket-status --digest-only` flag (ADR-0047) that produced the digest without writing. The Go `docket status` never writes, so the flag does not exist."
  - Reword the "## Obsolete terms" intro so it covers retired names as well as retired features: "Retired features and names. A retired feature's config key is still recognised, so a stale file gets a warning or a refusal instead of being silently accepted; nothing in current docket uses any of these." No old→new mapping table (Decision 10).
  - Fix every affected `#anchor` link (`#bootstrap-verdict-proceed--stop_migrate--create_orphan`, `#digest--digest-only-read`, `#step-0-preamble`) and re-sort the index.

- [ ] **Step 7: Regenerate and verify.**

```bash
go generate ./internal/assets && go run ./cmd/genassets -repo . -check
go test -count=1 ./internal/assets/
go test -count=1 ./internal/repoguard/ -skip 'TestCommittedCodexDispatch|TestCodexRequestFileCarriesRunID'
```

Re-run Step 1's grep: it must print nothing outside the glossary's "## Obsolete terms" section (`command grep -n -E -e "BOOTSTRAP=|STOP_MIGRATE|CREATE_ORPHAN|digest-only" docs/reference/glossary.md` must report only lines after the `## Obsolete terms` heading).

- [ ] **Step 8: Commit.**

```bash
git add -- skills internal/repoguard/capability_surface_test.go docs/reference docs/guide/capturing-work.md
git add -u -- internal/assets/embedded
git commit -m "docs: rename Step-0 preamble to startup check; retire bootstrap verdicts and --digest-only (change 0469, ADR-0129 rows 80, 85, 86)"
```

---

### Task 8: Seal the family (e) wire tokens (rows 67-73)

**Build tier:** standard — test-code changes to an existing repo guard: one new match kind, eight appended rows, controls, and mutation proof.

**Files:**
- Modify: `internal/repoguard/retired_vocabulary_test.go`

**Interfaces:**
- Consumes: the renamed spellings from Tasks 1-4 (the maintained surface must already be clean).
- Produces: `kindWord retiredKind`; `wordRe(old string) *regexp.Regexp`; `textMatcher(r retiredToken) *regexp.Regexp`; eight `retiredVocabulary` rows for 67-73.

- [ ] **Step 1: Add the kind, the matcher and the header paragraph.** In the `retiredKind` const block, append after `kindSchemaKey`:

```go
	// kindWord: like kindToken, but the LEADING boundary also excludes '-', so a
	// hyphenated compound that merely ends in Old is a different word. Family (e)
	// row 68 needs it: identity-mismatch is retired, while the kept
	// scope-identity-mismatch and row 72's own pr-identity-mismatch are other
	// words (ADR-0129 "Kept in family (e)").
	kindWord
```

After `tokenRe`, add:

```go
var wordReCache = map[string]*regexp.Regexp{}

// wordRe is the bounded matcher for a kindWord row (see kindWord).
func wordRe(old string) *regexp.Regexp {
	if re, ok := wordReCache[old]; ok {
		return re
	}
	re := regexp.MustCompile(`(^|[^A-Za-z0-9_-])` + regexp.QuoteMeta(old) + `([^A-Za-z0-9_-]|$)`)
	wordReCache[old] = re
	return re
}

// textMatcher returns the line matcher for a row scanned as text (kindToken,
// kindWord), or nil for a kind scanned another way.
func textMatcher(r retiredToken) *regexp.Regexp {
	switch r.Kind {
	case kindToken:
		return tokenRe(r.Old)
	case kindWord:
		return wordRe(r.Old)
	}
	return nil
}
```

Replace `scanTextLine`'s loop body with:

```go
	for _, r := range retiredVocabulary {
		re := textMatcher(r)
		if re == nil {
			continue
		}
		if re.MatchString(line) {
			hits = append(hits, retiredHit{rel, lineNo, r, strings.TrimSpace(line)})
		}
	}
```

In `scanGoLiteral`'s switch, add `case kindWord: hit = wordRe(r.Old).MatchString(val)`. In the header comment, after the family (d) paragraph, add:

```go
// Family (e), readability renames (change 0469), appends rows 67-73, the wire
// rows: the readiness needs-brainstorm, the gate-drive halt causes
// identity-mismatch and unresolved-execution, the change.repair-identity
// operation and its repaired-branch / repaired-pr results, finalize's
// pr-identity-mismatch and recertify's identity-drift. Rows 74-84 are prose and
// rows 85-86 retire names with no Go sites, which the seal does not scan (the
// results file records the closing grep). Row 68 is the first kindWord row:
// scope-identity-mismatch (kept) and pr-identity-mismatch (row 72) end in its
// spelling. LIMITATION: a composite such as halted-identity-mismatch is not a
// kindWord hit; the cause literal it would be composed from is sealed.
```

- [ ] **Step 2: Append the rows** at the end of `retiredVocabulary`:

```go
	// Family (e) — readability renames (change 0469): rows 67-73, the wire rows.
	// Rows 74-86 are prose or retire names with no Go sites.
	{Row: "67", Kind: kindToken, Old: "needs-brainstorm", New: "needs-grooming"},
	{Row: "68", Kind: kindWord, Old: "identity-mismatch", New: "worktree-changed"},
	{Row: "69", Kind: kindToken, Old: "unresolved-execution", New: "launch-unconfirmed"},
	{Row: "70", Kind: kindToken, Old: "repair-identity", New: "change.relink / docket change relink"},
	{Row: "71", Kind: kindToken, Old: "repaired-branch", New: "relinked-branch"},
	{Row: "71", Kind: kindToken, Old: "repaired-pr", New: "relinked-pr"},
	{Row: "72", Kind: kindToken, Old: "pr-identity-mismatch", New: "pr-link-mismatch"},
	{Row: "73", Kind: kindToken, Old: "identity-drift", New: "certified-input-changed"},
```

Raise the `testRetiredTableIntegrity` floor from `89` to `97` (the new exact length; confirm by counting `{Row:` lines inside the slice).

- [ ] **Step 3: Controls.**
  - In `testRetiredNonVacuity`, change `case kindToken:` to `case kindToken, kindWord:` (the planted "run `<old>` now" line and Go literal hit both kinds).
  - Rewrite the two lines that still name the retired verb. The row-40 positive `{"tests/test_x.sh", "docket change repair-identity --id 1 --version v   # a bare --version on repair-identity is bound too"}` becomes `{"tests/test_x.sh", "docket change relink --id 1 --version v   # a bare --version on relink is bound too"}`. The negative control `"docket change repair-identity --id 1 --expect-revision <v> --adopt-pr-head"` becomes `"docket change relink --id 1 --expect-revision <v> --adopt-pr-head"`.
  - After the row-40 blocks in `testRetiredNonVacuity`, add the family (e) attribution checks:

```go
	// Family (e) (change 0469): row 68 is a kindWord row. The bare halt cause hits
	// it in Go and markdown; the two hyphenated compounds ending in its spelling
	// are other words — scope-identity-mismatch hits nothing (a negative control
	// below), and pr-identity-mismatch hits row 72, never row 68. Row 67's
	// composed refusal reason not-ready-needs-brainstorm hits row 67.
	if !hasRetiredRow(goHits("internal/gatedrive/driver.go", "package p\nfunc f() { halt(&res, \"identity-mismatch\") }\n"), "68") {
		t.Errorf("row 68: a bare identity-mismatch halt literal was not detected")
	}
	if !hasRetiredRow(scanTextContent("skills/x/SKILL.md", "the drive halts `identity-mismatch`"), "68") {
		t.Errorf("row 68: a bare identity-mismatch in markdown was not detected")
	}
	if hits := scanTextContent("skills/x/SKILL.md", "finalize refuses `pr-identity-mismatch`"); hasRetiredRow(hits, "68") || !hasRetiredRow(hits, "72") {
		t.Errorf("pr-identity-mismatch must hit row 72 and never row 68: %v", hits)
	}
	if !hasRetiredRow(goHits("internal/domain/actions.go", "package p\nvar r = \"not-ready-needs-brainstorm\"\n"), "67") {
		t.Errorf("row 67: the composed not-ready-needs-brainstorm reason was not detected")
	}
```

  - Append to `cleanText` in `testRetiredNegativeControls`:

```go
		// Change 0469 — family (e): the new spellings and the kept namesakes
		// (ADR-0129 "Kept in family (e)").
		"readiness `needs-grooming`; the refusal reason `not-ready-needs-grooming`",
		"refused `scope-identity-mismatch`: the scope pins another change",
		"the drive halts `worktree-changed` or `launch-unconfirmed`; a `launch-unresolved` halt is different",
		"the slot state `unresolved` and the stage `mark-worktree-execution-unresolved` stay",
		"`relinked-branch` / `relinked-pr` / `stale-evidence` / `workspace-conflict` / `candidate-branch-absent` / `pr-unknown` / `invalid-request`",
		"finalize refuses `pr-link-mismatch`; recertify refuses `certified-input-changed`",
		"`results-identity-broken`, `identity-reused`, `identity-mutated`, `fingerprint-mismatch`",
		"an identity-mismatched repository definition is refused",
```

  - Append to `cleanGo`:

```go
		{"internal/gatedrive/ownership.go", "package p\nconst k = \"scope-identity-mismatch\"\n"},
		{"internal/gatedrive/ownership.go", "package p\nconst k = \"launch-unconfirmed\"\n"},
		{"internal/gatedrive/driver.go", "package p\nvar c = \"worktree-changed\"\n"},
		{"internal/app/change_repair.go", "package p\nconst o = \"change.relink\"\n"},
		{"internal/domain/finalize.go", "package p\nconst c = \"pr-link-mismatch\"\n"},
```

- [ ] **Step 4: Run the seal.**

Run: `go test -count=1 -run 'TestRetiredVocabularySeal' ./internal/repoguard/`
Expected: PASS. If `maintained_surfaces` or `generator_output` reports a family (e) hit, an earlier task missed a site. Fix that site (rename it; never widen the seal), regenerate as needed, and re-run.

- [ ] **Step 5: Mutation proof** (restore from a backup; confirm each mutation landed).
  - **M1, the kind matters:** set row 68's `Kind` to `kindToken`. Run the seal: `negative_controls` must FAIL on `scope-identity-mismatch`, and the `pr-identity-mismatch` attribution check must FAIL. Restore.
  - **M2, the matcher matters:** make `textMatcher` return `nil` for `kindWord` (delete its `case`). Run the seal: `non_vacuity` must FAIL for row 68. Restore.
  - **M3, every row seals its spelling** in maintained source:

```bash
bash -c '
f=skills/docket-status/SKILL.md
for pair in "67:needs-brainstorm" "68:identity-mismatch" "69:unresolved-execution" "70:repair-identity" "71:repaired-branch" "71:repaired-pr" "72:pr-identity-mismatch" "73:identity-drift"; do
  row=${pair%%:*}; old=${pair#*:}
  cp "$f" "$f.bak"
  printf "\nrun \`%s\` now\n" "$old" >> "$f"
  n=$(command grep -c -F -- "$old" "$f"); [ "$n" -ge 1 ] || { echo "MUTATION DID NOT LAND: $old"; mv -f "$f.bak" "$f"; exit 1; }
  out=$(go test -count=1 -run "TestRetiredVocabularySeal/maintained_surfaces" ./internal/repoguard/ 2>&1); rc=$?
  mv -f "$f.bak" "$f"
  if [ $rc -eq 0 ] || ! command grep -q -F -- "ADR-0129 row $row: retired \"$old\"" <<<"$out"; then echo "SEAL DID NOT REDDEN for row $row $old"; exit 1; fi
  echo "row $row $old: red as expected"
done
git diff --quiet -- "$f" && echo "restored clean"'
```

Expected: eight "red as expected" lines, then "restored clean". Record M1-M3 in the task report.

- [ ] **Step 6: Run the package and commit.**

```bash
gofmt -l internal/repoguard/
go vet ./internal/repoguard/
go test -count=1 ./internal/repoguard/ -skip 'TestCommittedCodexDispatch|TestCodexRequestFileCarriesRunID'
git add -- internal/repoguard/retired_vocabulary_test.go
git commit -m "test(repoguard): seal ADR-0129 family (e) wire tokens, rows 67-73 (change 0469)"
```

---

### Task 9: Closing verification — whole-repo grep, kept spellings, shards, generated material

**Build tier:** standard — verification with possible small fixes that need sense judgment; it commits only if a fix is needed.

**Files:**
- Modify: only files where the closing grep finds a straggler.

- [ ] **Step 1: Closing grep over maintained source** (spec *Testing*: "comes back empty, except inside the retired-vocabulary table, the glossary's 'Obsolete terms' section, ADR-0129, and frozen records"):

```bash
bash -c '
MAINT=(-- . ":!docs/changes" ":!docs/results" ":!docs/superpowers" ":!docs/adrs" ":(exclude,glob)**/testdata/**" ":!tests/fixtures" ":!internal/install/legacydata" ":!internal/assets/embedded" ":!internal/repoguard/retired_vocabulary_test.go")
out=$(git grep -n -i -E -e "needs.brainstorm|NeedsBrainstorm|(^|[^a-z0-9_-])identity-mismatch([^a-z0-9_-]|$)|unresolved-execution|UnresolvedExecution|repair-identity|RepairIdentity|repaired-branch|repaired-pr|RepairRepaired|pr-identity-mismatch|PRIdentityMatch|identity-drift|IdentityDrift|conjunct|admission slot|liveness transition|native supervisor|gate execution|presence-encoded|step-0 preamble|closed vocabular|compare-and-swap|pay.per.relevance|identity repair|identity checkpoint|BOOTSTRAP=|STOP_MIGRATE|CREATE_ORPHAN|digest-only" "${MAINT[@]}")
printf "%s\n" "$out"'
```

Expected: hits only inside `docs/reference/glossary.md` after its `## Obsolete terms` heading (the bootstrap verdict and digest-only entries). Any other hit is a straggler: rename it per the maps, or, if it is a documented kept sense (Global Constraints), record why in `NOTES`.

- [ ] **Step 2: Retired table and generated material.**

```bash
go run ./cmd/genassets -repo . -check
go test -count=1 -run 'TestRetiredVocabularySeal|TestDispatchBlockBudget|TestSkillSizeBudgets' ./internal/repoguard/
git diff 366827eb540d22b7672f95b5ab1656ae54242491 -- AGENTS.md | command grep -E '^[+-][^+-]'
git diff --stat 366827eb540d22b7672f95b5ab1656ae54242491 -- internal/harness/claude/testdata internal/harness/codex/testdata internal/harness/cursor/testdata internal/harness/opencode/testdata
```

Expected: `-check` clean; tests PASS; the AGENTS.md diff since base is exactly the `unmet conjuncts` → `unmet conditions` line pair; every golden change is a mapped rename (read the diff).

- [ ] **Step 3: Frozen paths untouched.**

```bash
git diff --stat 366827eb540d22b7672f95b5ab1656ae54242491 -- docs/changes docs/results docs/adrs testdata internal/repository/testdata internal/install/legacydata internal/install/testdata tests/fixtures
git diff --stat 366827eb540d22b7672f95b5ab1656ae54242491 -- docs/superpowers
```

Expected: the first prints nothing; the second lists only this plan.

- [ ] **Step 4: Kept spellings unchanged.** Compare maintained-source counts at base and HEAD:

```bash
bash -c '
MAINT=(":!docs/changes" ":!docs/results" ":!docs/superpowers" ":!docs/adrs" ":(exclude,glob)**/testdata/**" ":!internal/install/legacydata" ":!internal/assets/embedded" ":!internal/repoguard/retired_vocabulary_test.go")
for t in scope-identity-mismatch ErrScopeIdentityMismatch fingerprint-mismatch results-identity-broken identity-reused identity-mutated launch-unresolved mark-worktree-execution-unresolved stale-evidence workspace-conflict candidate-branch-absent pr-unknown adopt-pr-head expect-revision run-record-cas contended skills.brainstorm docket-brainstorm; do
  a=$(git grep -c -F -e "$t" 366827eb540d22b7672f95b5ab1656ae54242491 -- . "${MAINT[@]}" | awk -F: "{s+=\$NF} END {print s+0}")
  b=$(git grep -c -F -e "$t" HEAD -- . "${MAINT[@]}" | awk -F: "{s+=\$NF} END {print s+0}")
  [ "$a" = "$b" ] && echo "same  $t $a" || echo "DIFF  $t $a -> $b"
done'
```

Expected: every line `same`, except where a line holding a kept spelling was also rewritten for a mapped rename in the same sentence. Explain each `DIFF` in `NOTES`; a kept spelling may never be renamed.

- [ ] **Step 5: Integration shards.** Every `func TestIntegration*` and `func TestRaceIntegration*` still starts with a declared shard prefix:

```bash
bash -c '
prefixes=$(command grep -h -o -E "SHARD_PREFIX=\"[A-Za-z]+\"" tests/test_go_integration_*.sh | sed -E "s/SHARD_PREFIX=\"(.*)\"/\1/" | sort -u)
names=$(git grep -h -o -E "^func Test(Race)?Integration[A-Za-z0-9_]+" -- "*_test.go" | sed "s/^func //" | sort -u)
missing=0
while read -r n; do ok=0; while read -r p; do case "$n" in "$p"*) ok=1;; esac; done <<<"$prefixes"; [ $ok -eq 1 ] || { echo "UNSHARDED: $n"; missing=1; }; done <<<"$names"
[ $missing -eq 0 ] && echo "all integration tests shard-prefixed"'
```

Expected: exactly one line, `UNSHARDED: TestIntegrationBranchAuto`. That is an untagged unit test in `internal/config/resolve_test.go` whose name merely starts with "TestIntegration", and it is reported identically at base (measured when this plan was written). Any other reported name is a test this change renamed out of its shard: restore its shard prefix.

- [ ] **Step 6: Commit any straggler fixes** (skip if Steps 1-5 found nothing):

```bash
git add -- <each fixed path>
git commit -m "refactor: sweep remaining family (e) spellings (change 0469)"
```

---

## Handoff notes for the results file (not build tasks)

- **Human action:** consumer repos must re-run `docket install` to pick up the renamed dispatch material. Land with **no gate drive in flight** in any docket repo on the machine (rows 68-69 rename halt causes that stored drive records carry), and run the post-merge binary rebuild immediately (AGENTS.md *Rebuild the binary after a merge to main*).
- The committed `BOARD.md` on `docket` keeps `needs-brainstorm` cells until its next re-render; 0469's own close-out commit re-renders it.
- Follow-up candidates for a human (not built here): the takeover scope-mismatch halt shares the `worktree-changed` token with the fingerprint halt (Task 2 trace note; splitting it needs an ADR-0129 `## Update`). The four Codex dispatch repoguard tests are red at base (Global Constraints).
