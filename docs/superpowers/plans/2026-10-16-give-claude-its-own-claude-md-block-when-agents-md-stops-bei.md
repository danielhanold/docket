<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0555 — Keep Claude's dispatch block in AGENTS.md while CLAUDE.md links to it](../../changes/active/0555-give-claude-its-own-claude-md-block-when-agents-md-stops-bei.md)**
<!-- docket:backlink:end -->

# Keep Claude's dispatch block in AGENTS.md while CLAUDE.md links to it — Implementation Plan

> **For agentic workers:** this plan is executed by `docket-build` (task-by-task, tier-routed, one full-suite gate at the end). Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** When `CLAUDE.md` is (or becomes) a relative link to `AGENTS.md`, Claude co-owns the `AGENTS.md` dispatch block, the block's text and the share decision follow the full opt-in set, and install never retires a block in the same run that writes it.

**Architecture:** `reposeed.Plan` (pure) gains two caller-computed inputs — the full opt-in set and the `AGENTS.md` pre-state — and a new share rule. `internal/app` computes the `AGENTS.md` state beside `classifyClaudeMD`, passes both inputs from every shared-mode `PlanInput` builder, and `computeRemovals` skips any prior surface whose path this run plans. No change to `internal/install` behavior.

**Tech Stack:** Go (std lib only), docket's integration-tagged real-git test shards.

**Spec:** `docs/superpowers/specs/2026-10-16-give-claude-its-own-claude-md-block-when-agents-md-stops-bei-design.md` (on the `docket` metadata branch; read it alongside this plan — its "What each case does after the change" table is the acceptance matrix).

## Global Constraints

- Shared repositories only. Private mode (`PlanPrivate`, the `planned` parameter's private meaning in `composeRecordBytes`/`computeRemovals`) keeps its behavior byte-for-byte.
- `Plan` never stats a path; every filesystem fact is computed by the caller.
- `classifyClaudeMD` does not change. A dangling relative link to `AGENTS.md` still reads as `ClaudeMDLinkToAgents`.
- A `CLAUDE.md` that links anywhere other than `AGENTS.md` stays skipped (`ClaudeMDForeignLink`). A regular `CLAUDE.md` keeps its own block and is never replaced by a link. `ClaudeMDOther` is still planned as a block.
- An `AGENTS.md` probe error other than not-exist is `AgentsMDOther`, never "missing" (probe-error-is-not-clean-absence).
- No `internal/install` behavior change; `inspect.go` gets a comment fix only.
- `composeRecordBytes` needs no change.
- Comments anchor on symbols or quoted clauses, never line numbers (ADR-0054 / `TestCommentAnchorStyle`).
- Docs describe only current behavior; no change/PR citations in docs.
- Run every mutation probe with `-count=1`, and restore from a backup copy of your edited file, never `git checkout --` (that restores HEAD and destroys uncommitted work).
- Integration tests run only through their shard prefix: `go test -tags integration -count=1 -run '^<Prefix>' ./internal/app/` — never an unfiltered `./internal/app/` integration run.

## Review Focus

1. **A repository already carrying the pre-change record** (`AGENTS.md` owned by `[opencode]` or `[codex]` only, link owned by `[claude]`) dropped to `[claude]` — the block must survive and the run must not refuse. Pinned by Task 3's `...DropKeepsAgentsBlock` (both variants).
2. **A user's `AGENTS.md` that already holds unrelated text** in a claude-only repository with a hand-made link — every user byte must survive and the block must be inserted (top/after frontmatter), not appended. Pinned by Task 2's `...HandMadeLinkAddsBlock`.
3. **A dangling `CLAUDE.md` link** (`AGENTS.md` deleted, e.g. by #537's empty-file deletion) — install must recreate `AGENTS.md` and the link must resolve. Pinned by Task 2's `...RepairsMissingAgentsMD`.
4. **`AGENTS.md` that is itself a symlink or a directory** under a claude-only link — nothing planned for Claude, run still succeeds. Pinned by Task 1's planner row and Task 2's `TestClassifyAgentsMD`.
5. **A scoped run (`--harness opencode`) in a codex-opted repository** — the Codex clause must survive byte-identically. Pinned by Task 1's scoped-interior row and Task 2's `...ScopedRunKeepsCodexClause`.

---

### Task 1: Planner share rule in `reposeed.Plan`

**Files:**
- Modify: `internal/reposeed/plan.go` (`ClaudeMDState` doc + `ClaudeMDLinkToAgents` comment, new `AgentsMDState`, `PlanInput`, `Plan`)
- Test: `internal/reposeed/plan_test.go`

**Interfaces:**
- Produces:
  ```go
  // AgentsMDState is the AGENTS.md pre-state the caller computes with os.Lstat.
  type AgentsMDState int
  const (
      AgentsMDMissing     AgentsMDState = iota // zero value: no AGENTS.md
      AgentsMDRegularFile                      // a regular file
      AgentsMDOther                            // a symlink (incl. circular), a directory, or an unreadable path
  )
  type PlanInput struct {
      WorktreeRoot  string
      Harnesses     []string      // the set this run plans for (a --harness scope narrows it)
      OptIns        []string      // the repository's full agent_harnesses; nil means Harnesses is the full set
      RunTracker    []byte
      ClaudeMDState ClaudeMDState
      AgentsMDState AgentsMDState
  }
  ```
  `Plan(in PlanInput) ([]install.Target, map[string][]string, error)` — signature unchanged.

- [ ] **Step 1: Write the failing table test `TestPlanClaudeSharesAgentsMD` in `plan_test.go`**

  Rows (`plain := []byte(harness.DispatchInterior(runTracker))`, `codex := []byte(harness.CodexDispatchInterior(runTracker))`). Columns: harnesses, optIns, ClaudeMDState, AgentsMDState → want AGENTS.md owners (nil = no AGENTS.md target), want AGENTS.md interior, want CLAUDE.md kind (`""` = no CLAUDE.md target).

  | # | name | harnesses | optIns | claude | agents | AGENTS owners | interior | CLAUDE.md |
  |---|---|---|---|---|---|---|---|---|
  | 1 | link, claude alone | `[claude]` | nil | `LinkToAgents` | `RegularFile` | `[claude]` | plain | `KindSymlink` |
  | 1b | link, claude alone, AGENTS.md missing | `[claude]` | nil | `LinkToAgents` | `Missing` | `[claude]` | plain | `KindSymlink` |
  | 2 | link, claude and codex | `[claude codex]` | nil | `LinkToAgents` | `RegularFile` | `[claude codex]` | codex | `KindSymlink` |
  | 3 | absent, own AGENTS.md, claude alone | `[claude]` | nil | `Absent` | `RegularFile` | `[claude]` | plain | `KindSymlink` |
  | 4 | absent, no AGENTS.md, claude alone | `[claude]` | nil | `Absent` | `Missing` | nil | – | `KindManagedBlock` |
  | 5 | regular CLAUDE.md, own AGENTS.md, claude alone | `[claude]` | nil | `RegularFile` | `RegularFile` | nil | – | `KindManagedBlock` |
  | 6a | scoped opencode keeps codex interior | `[opencode]` | `[codex opencode]` | `Absent` | `RegularFile` | `[opencode]` | codex | `""` |
  | 6b | scoped claude, link, codex opted in | `[claude]` | `[claude codex]` | `LinkToAgents` | `RegularFile` | `[claude]` | codex | `KindSymlink` |
  | 7 | scoped claude, absent, codex opted in | `[claude]` | `[claude codex]` | `Absent` | `Missing` | `[claude]` | codex | `KindSymlink` |
  | 9a | link to non-regular AGENTS.md, claude alone | `[claude]` | nil | `LinkToAgents` | `Other` | nil | – | `""` |

  Per row assert: AGENTS.md target present iff owners non-nil, with `Kind == install.KindManagedBlock`, `BlockName == "dispatch"`, `bytes.Equal(Content, interior)`, `reflect.DeepEqual(owners[agentsMD()], wantOwners)`; CLAUDE.md target kind matches; a `KindSymlink` CLAUDE.md has `LinkTarget == agentsMD()` and `owners[claudeMD()] == []string{"claude"}`; a `KindManagedBlock` CLAUDE.md carries `plain`; `""` means `byPath(targets)[claudeMD()]` absent and `owners[claudeMD()]` absent. Row 9a also asserts no target at all is planned (`len(targets) == 0`).

- [ ] **Step 2: Update the existing tests to the new rule**
  - `TestPlanClaudeSymlinkWithoutShareIsSkipped`: delete the `"link to AGENTS.md, claude alone"` row; keep the two foreign-link rows; rewrite its comment to say only a foreign link is skipped (a link to `AGENTS.md` now shares).
  - `TestPlanClaudeCodexAbsentSymlinks`: AGENTS.md owners want `[]string{"claude", "codex"}`; fix the "claude shares via the link, it does not co-own the block" comment and the error text.
  - `TestPlanClaudeAloneAbsentStateStillBlock`: comment says "no AGENTS.md exists and no codex/opencode opted in" instead of "no shared AGENTS.md surface exists".
  - `TestSurfacePathsCoverPlan`: add an inner loop over `[]AgentsMDState{AgentsMDMissing, AgentsMDRegularFile, AgentsMDOther}` so every combination is planned; the expected set is unchanged.

- [ ] **Step 3: Run to verify failure**

  Run: `go test -count=1 ./internal/reposeed/`
  Expected: FAIL — compile error (`AgentsMDState`/`OptIns` undefined) first; once stubbed, rows 1, 1b, 2 (owners), 3, 6a, 6b, 7 red.

- [ ] **Step 4: Implement the rule in `Plan`**

  Decide in this order (`selected` = `Harnesses`; `full` = `selectHarnesses(OptIns)` when `OptIns != nil`, else `selected`; an unknown OptIns token is an error as for Harnesses):

  ```go
  codexInterior := interior; if full[codex] { codexInterior = CodexDispatchInterior }
  claudeShares := false
  if selected[claude] {
      switch in.ClaudeMDState {
      case ClaudeMDLinkToAgents:
          claudeShares = in.AgentsMDState == AgentsMDMissing || in.AgentsMDState == AgentsMDRegularFile
      case ClaudeMDAbsent:
          claudeShares = full[codex] || full[opencode] || in.AgentsMDState == AgentsMDRegularFile
      }
  }
  // AGENTS.md block planned iff selected[codex] || selected[opencode] || claudeShares;
  // owners = selected codex/opencode, plus claude when claudeShares; content = codexInterior.
  // CLAUDE.md: claudeShares → KindSymlink to AGENTS.md; LinkToAgents/ForeignLink (not sharing) → skip;
  // otherwise → managed block with the plain interior.
  ```

  Replace the "Claude never co-owns this block — when it shares, it does so through a CLAUDE.md link" comment and the comment above `share :=` with the new rule: Claude co-owns the block when it shares; it shares when `CLAUDE.md` already links to `AGENTS.md` (and `AGENTS.md` is missing or a regular file), or when there is no `CLAUDE.md` and `AGENTS.md` exists as a regular file or codex/opencode is opted in anywhere; the interior and the no-`CLAUDE.md` share follow the full opt-in set so a scoped run never strips the Codex clause. Update the `ClaudeMDLinkToAgents` doc ("a share re-plans the same link") to say the link shares `AGENTS.md`, so the block goes into the file the link points to; update the `ClaudeMDState` type doc ("share the codex/opencode AGENTS.md surface") to match. Document `OptIns` and `AgentsMDState` on `PlanInput` the way `PrivatePlanInput.OptIns` is documented.

- [ ] **Step 5: Run to verify pass**

  Run: `go test -count=1 ./internal/reposeed/`
  Expected: PASS

- [ ] **Step 6: Mutation probes (back up `plan.go` first; restore from the copy after each)**
  - Restore the skip for a link when codex/opencode are not selected (`claudeShares` for `LinkToAgents` additionally requires `selected[codex]||selected[opencode]`) → rows 1, 1b red.
  - Pick the interior from `selected` instead of `full` → rows 6a, 6b red.
  - Decide the `ClaudeMDAbsent` share from `selected` instead of `full` → row 7 red.
  - Drop the `AgentsMDState == AgentsMDRegularFile` term in the absent share → row 3 red.
  - Share for a link whatever `AgentsMDState` is → row 9a red.
  - Share for `ClaudeMDRegularFile` too → row 5 red.

- [ ] **Step 7: Commit**

  ```bash
  git add internal/reposeed/plan.go internal/reposeed/plan_test.go
  git commit -m "fix(reposeed): let Claude co-own the AGENTS.md block when CLAUDE.md links to it"
  ```

---

### Task 2: Wire the new inputs through `internal/app`

**Files:**
- Modify: `internal/app/repophase.go` (`ResolveRepoPhase`, new `classifyAgentsMD`)
- Modify: `internal/app/repository_set_visibility_commit.go` (the shared-mode `reposeed.Plan` call for `st.primary`)
- Leave: `internal/reposetup/initplan.go` `SeedInput` — no production code consumes it (only `initplan_test.go` reads `WorktreeRoot`/`Harnesses`); the zero values (`OptIns` nil = full set, `AgentsMDMissing`) are correct. Add nothing.
- Create: `internal/app/repophase_test.go` (default, untagged corpus — touches only the filesystem, never git)
- Test: `internal/app/repophase_integration_test.go` (prefix `TestIntegrationContextProbe`, shard `tests/test_go_integration_app_contextprobe.sh`)

**Interfaces:**
- Consumes: `reposeed.AgentsMDState`, `reposeed.PlanInput.OptIns`/`.AgentsMDState` (Task 1).
- Produces: `func classifyAgentsMD(root string) reposeed.AgentsMDState` in `repophase.go` — `os.Lstat(filepath.Join(root, "AGENTS.md"))`: not-exist → `AgentsMDMissing`; any other error → `AgentsMDOther`; regular → `AgentsMDRegularFile`; anything else (symlink, directory) → `AgentsMDOther`.
- Produces (test helpers in `repophase_integration_test.go`, used by Task 3): 
  - `seedSharedSurfaces(t *testing.T, root, gitDir string, harnesses []string, claudeMD reposeed.ClaudeMDState, agentsMD reposeed.AgentsMDState, agentsOwners []string)` — runs `reposeed.Plan` with the given inputs and `buildRunTracker()`, overrides `owners[filepath.Join(root,"AGENTS.md")]` with `agentsOwners` when non-nil (to reproduce the pre-change record shape), encodes `reposeed.DesiredRecord`, and applies it with `applyRepoPhaseSurfaces` to `reposeed.RecordPath(gitDir, layout.SharedName)`.
  - `installShared(t *testing.T, root string, scope []string) (*install.RepoPhase, bool)` — `ResolveRepoPhase(ctx, newGitClient(t), root, scope, rt, nil, config.ResolveContext{DefaultBranch: "main"})`, fails on error or unauthorized, then `applyRepoPhaseSurfaces(phase, roots)` with `install.ResolveRoots(os.UserHomeDir, os.Getenv)`, failing on its error; returns the phase and whether the apply wrote anything.
  - Every test using them calls `pinInstructionsUserRoots(t)` first (defined in `private_instructions_integration_test.go`, same package and tag).

- [ ] **Step 1: Write the failing unit test `TestClassifyAgentsMD` in `repophase_test.go`**

  In `t.TempDir()`: no file → `AgentsMDMissing`; `writeRepoFile(..., "AGENTS.md", "x\n")` → `AgentsMDRegularFile`; `AGENTS.md` a directory → `AgentsMDOther`; `AGENTS.md` a symlink to a regular sibling file → `AgentsMDOther`; `AGENTS.md` a circular symlink (`os.Symlink("AGENTS.md", AGENTS.md)`) → `AgentsMDOther`.

- [ ] **Step 2: Write the failing integration tests in `repophase_integration_test.go`**

  Each: `pinInstructionsUserRoots(t)`, `root, gitDir := initGitRepo(t, "agent_harnesses: [...]\n")`, fixture, `installShared`, assertions. Use `document.Parse` + `doc.Block("dispatch")` to locate the block.

  - `TestIntegrationContextProbeClaudeShareHandMadeLinkAddsBlock` — `[claude]`, `AGENTS.md` = `"# Project rules\n\nUse tabs.\n"`, `os.Symlink("AGENTS.md", CLAUDE.md)`, no record. After install: `AGENTS.md` has a dispatch block; the file with the block's byte range removed, trimmed of leading blank lines, equals the user text (do not assume the block is at the end); `os.Lstat(CLAUDE.md)` is still a symlink reading `"AGENTS.md"`; the record lists `CLAUDE.md` (symlink, `[claude]`) and `AGENTS.md` (`[claude]`).
  - `TestIntegrationContextProbeClaudeShareRepairsLostBlock` — `[claude]`, the link, `AGENTS.md` with user text and no block, a record holding only the link (`seedSharedSurfaces` is not usable here; write the record via `reposeed.EncodeRecord` with one `SurfaceRecord{Path:"CLAUDE.md", Kind: install.KindSymlink, LinkTarget:"AGENTS.md", Harnesses: []string{"claude"}}`, as `...RetiresDroppedClaudeLink` does). After install: block present and the user text survives byte-for-byte outside it.
  - `TestIntegrationContextProbeClaudeShareRepairsMissingAgentsMD` — same record and link, no `AGENTS.md` (dangling). After install: `AGENTS.md` exists with the block; `os.Stat(CLAUDE.md)` succeeds and `os.SameFile` with `AGENTS.md`.
  - `TestIntegrationContextProbeClaudeShareNoClaudeMDLinksOwnAgentsMD` — `[claude]`, user `AGENTS.md`, no `CLAUDE.md`. After install: `CLAUDE.md` is a symlink resolving to `AGENTS.md`; `AGENTS.md` has the block plus the user text.
  - `TestIntegrationContextProbeClaudeShareNonRegularAgentsMDUnchanged` — `[claude]`, the link, `AGENTS.md` a directory. `ResolveRepoPhase` succeeds with `len(phase.Targets) == 0`; nothing on disk changes.
  - `TestIntegrationContextProbeClaudeShareScopedRunKeepsCodexClause` — `[claude, codex, opencode]`, first `installShared(t, root, nil)`, read `AGENTS.md` interior; then `installShared(t, root, []string{"opencode"})`; the interior is byte-identical and contains `"### Codex root-coordinator entry"`.

- [ ] **Step 3: Run to verify failure**

  Run: `go test -count=1 -run '^TestClassifyAgentsMD$' ./internal/app/` and `go test -tags integration -count=1 -run '^TestIntegrationContextProbeClaudeShare' ./internal/app/`
  Expected: FAIL (classifyAgentsMD undefined; once stubbed, hand-made link / repair / no-CLAUDE.md / scoped cases red).

- [ ] **Step 4: Implement**
  - Add `classifyAgentsMD` beside `classifyClaudeMD`, doc comment naming the three states and the probe-error rule.
  - `ResolveRepoPhase` shared branch: `OptIns: optIns, AgentsMDState: classifyAgentsMD(root)`.
  - `repository_set_visibility_commit.go`: `OptIns: ah.Value, AgentsMDState: classifyAgentsMD(st.primary)`.

- [ ] **Step 5: Run to verify pass**

  Same two commands. Expected: PASS. Also run the whole contextprobe shard: `bash tests/test_go_integration_app_contextprobe.sh` — PASS. Note its runtime against the 15 s row in `tests/runtime-budgets.tsv`; if it is at or near the ceiling, move the new `ClaudeShare` tests to a new prefix/shard rather than raising the number.

- [ ] **Step 6: Mutation probe** — make `classifyAgentsMD` return `AgentsMDMissing` for a non-not-exist error and for a symlink: `TestClassifyAgentsMD` reddens. Restore from backup.

- [ ] **Step 7: Commit**

  ```bash
  git add internal/app/repophase.go internal/app/repository_set_visibility_commit.go internal/app/repophase_test.go internal/app/repophase_integration_test.go
  git commit -m "fix(install): plan Claude's AGENTS.md share from the AGENTS.md state and full opt-ins"
  ```

---

### Task 3: Never retire a surface this run also writes (`computeRemovals`)

**Files:**
- Modify: `internal/app/repophase.go` (`ResolveRepoPhase`, `computeRemovals` signature and doc)
- Test: `internal/app/repophase_integration_test.go`

**Interfaces:**
- Consumes: `seedSharedSurfaces`, `installShared` (Task 2).
- Produces: `func computeRemovals(prior *reposeed.Record, root string, optIns []string, scope map[string]bool, planned map[string]bool, writes map[string]bool, priorState *install.State, legacy install.LegacyReproducer) ([]install.TargetRecord, error)` — `writes` is the set of cleaned absolute paths this run plans, in BOTH modes; a prior surface whose `abs` is in `writes` is never retired. `planned` keeps its private-only, nil-means-shared meaning unchanged.

- [ ] **Step 1: Write the failing tests**
  - `TestIntegrationContextProbeClaudeShareDropKeepsAgentsBlock`, subtests:
    - `"opencode, only the block"`: `initGitRepo(t, "agent_harnesses: [claude, opencode]\n")`; `seedSharedSurfaces(t, root, gitDir, []string{"claude","opencode"}, reposeed.ClaudeMDAbsent, reposeed.AgentsMDMissing, []string{"opencode"})`; assert the seeded record lists `AGENTS.md` as `[opencode]` (fixture guard — the case is vacuous otherwise); rewrite `.docket.yml` to `agent_harnesses: [claude]\n`; `phase, _ := installShared(t, root, nil)`. Assert: `phase.Removals` holds no `AGENTS.md` entry; `AGENTS.md` still has the dispatch block; `CLAUDE.md` still a link resolving to `AGENTS.md`; the record on disk lists `AGENTS.md` owned `[claude]`. Run `installShared` a second time and assert `wrote == false` and the block is still there (no flip-flop).
    - `"opencode, user text around it"`: as above, then before the drop append `"\n## My notes\n\nkeep me\n"` to `AGENTS.md`; after: block present and `"keep me"` present.
    - `"codex"`: harnesses `[claude, codex]`, owners override `[codex]`; on disk the Codex interior (assert it contains `"### Codex root-coordinator entry"` as a fixture guard); drop to `[claude]`; `installShared` succeeds (no ownership-conflict / duplicate-destination error), the block remains, and its interior no longer contains the Codex clause.
  - `TestIntegrationContextProbeClaudeShareDropEverythingRetires` — `[claude]`; make the link by hand (`os.Symlink("AGENTS.md", CLAUDE.md)`, no `AGENTS.md`), then `seedSharedSurfaces(t, root, gitDir, []string{"claude"}, reposeed.ClaudeMDLinkToAgents, reposeed.AgentsMDMissing, nil)` so the seed creates `AGENTS.md` holding only the block, owned `[claude]`. Then `.docket.yml` → `agent_harnesses: []\n`; install. Assert `CLAUDE.md` and `AGENTS.md` are both gone (`os.Lstat` not-exist).
  - Keep `TestIntegrationContextProbeResolveRepoPhaseRetiresDroppedClaudeLink` (the `[claude, codex] → [codex]` row) green unchanged.

- [ ] **Step 2: Run to verify failure**

  Run: `go test -tags integration -count=1 -run '^TestIntegrationContextProbeClaudeShareDrop' ./internal/app/`
  Expected: FAIL — opencode subtests: block missing after the run; codex subtest: run refused ("written by more than one step").

- [ ] **Step 3: Implement**
  - In `ResolveRepoPhase`, build `writes` from `targets` (cleaned paths) for both modes; set `planned = writes` only in private mode (keep the existing private block's behavior).
  - Pass `writes` to both `computeRemovals` calls (the legacy linked-worktree call too — its paths sit under `invokedRoot`, so nothing there matches; behavior unchanged).
  - In `computeRemovals`, skip a prior surface whose `abs` is in `writes` before the `stillWanted` logic.
  - Rewrite the `computeRemovals` doc comment: add the rule "a prior surface whose path this run plans is never retired; the planned write replaces its record, and ownership is still proven at inspection."

- [ ] **Step 4: Run to verify pass**

  Run the Step 2 command, then `bash tests/test_go_integration_app_contextprobe.sh` and `bash tests/test_go_integration_app_privateinstructions.sh` (private mode must be unchanged). Expected: PASS.

- [ ] **Step 5: Mutation probe** — delete the `writes[abs]` skip in `computeRemovals`: the opencode subtests redden (block missing) and the codex subtest reddens (refused). Restore from backup.

- [ ] **Step 6: Commit**

  ```bash
  git add internal/app/repophase.go internal/app/repophase_integration_test.go
  git commit -m "fix(install): never retire a repository surface the same run writes"
  ```

---

### Task 4: Comments and docs describe the new rule

**Files:**
- Modify: `internal/install/inspect.go` — `inspectManagedBlock`'s "Adding a block only appends" comment: the block is inserted at the top of the file or right after frontmatter (`renderManagedBlock`); every existing byte survives. Comment only.
- Modify: `docs/install/claude-code.md` — the "The dispatch block in `CLAUDE.md`, on opt-in only." bullet.
- Modify: `docs/reference/glossary.md` — the `agent_harnesses` entry's parenthetical "with a `CLAUDE.md` → `AGENTS.md` symlink when Codex or opencode is also opted in".
- Modify: `skills/docket-convention/references/agent-layer.md` — the "a non-empty list" bullet's link sentence.
- Regenerate: `go generate ./internal/assets/` (embedded skill copy).

**Interfaces:** none.

- [ ] **Step 1: Edit the three docs** to state the rule as it now stands, in plain words: Claude shares `AGENTS.md` through a `CLAUDE.md` link when `CLAUDE.md` already links there, or when there is no `CLAUDE.md` and `AGENTS.md` already exists or Codex or opencode is opted in; then the one block in `AGENTS.md` serves every harness. Otherwise `CLAUDE.md` gets its own block, and a regular `CLAUDE.md` keeps its content. No change or PR numbers.

- [ ] **Step 2: Fix the `inspect.go` comment.**

- [ ] **Step 3: Regenerate and verify**

  Run: `go generate ./internal/assets/ && go test -count=1 ./internal/assets/ ./internal/install/ ./internal/repoguard/`
  Expected: PASS; `git status` shows the regenerated embedded copy of `agent-layer.md` changed.

- [ ] **Step 4: Commit**

  ```bash
  git add internal/install/inspect.go docs/install/claude-code.md docs/reference/glossary.md skills/docket-convention/references/agent-layer.md internal/assets/
  git commit -m "docs: describe when Claude shares AGENTS.md through the CLAUDE.md link"
  ```

---

### Final gate (docket-build)

Run the whole suite via `build.test_command` from config. Expect fallout only where a test encoded the old rule; known candidates are set-visibility and repository-setup integration tests whose fixtures commit a user `AGENTS.md` with `agent_harnesses: [claude]` (e.g. `newSwitchCommitRepo` in `repository_set_visibility_commit_integration_test.go`, the `"# Agents\n"` fixture in `TestIntegrationRepoVisibilitySharedResumesCommitWithoutJournaledEdits`). Update one only when its new outcome is a row of the spec's "What each case does after the change" table (for example, a claude-only repository with its own `AGENTS.md` now gets the `CLAUDE.md` link and the block in `AGENTS.md`); any other red is a defect to fix. Read the budget report for `BUDGET WATCH:` / `SERIAL CONFIRMED OVER BUDGET:` lines on the contextprobe shard.
