<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0488 — Run task-worker tests directly in the foreground, not through gate drives](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-02-0488-run-task-worker-tests-directly-in-the-foreground-not-through.md)**
<!-- docket:backlink:end -->
# Run Task-Worker Tests Directly in the Foreground Implementation Plan

> **For agentic workers:** this plan is executed by `docket-build`: one tier worker per `### Task N`
> heading, each running the `docket-build-task` contract (one task, one commit). Steps use checkbox
> (`- [ ]`) syntax for readability only; nobody ticks them.

**Goal:** Build-task workers run every test directly in the foreground under
`timeout --kill-after=10s 10m`, call no gate operation, and return only `COMPLETE`,
`NEEDS_ESCALATION`, or `BLOCKED`. The build controller runs every full-suite attempt itself and
audits each worker's reported commands for the wrapper without ever halting on it.

**Architecture:** This is a prose-and-guard change. No Go production code changes. The worker
contract (`skills/docket-build-task/SKILL.md`), the controller (`skills/docket-build/SKILL.md`),
implement-next's run-id sentence, the shared gate caller contract, the run-tracker rule, the
glossary, and the install prerequisites are rewritten. The repoguard tests move with the contract:
a new absence guard replaces change 0416's scoped-start identity guard, sentinels pin the time
limit and the audit, guards whose subject is gone are retired, and the rest are narrowed. The
embedded asset mirror is regenerated with `go generate ./internal/assets`. The managed `AGENTS.md`
block is regenerated through docket's install path.

**Tech Stack:** Markdown skill bodies; Go 1.27 tests in `internal/repoguard` (stdlib `regexp`,
`strings`, `testing`); `go generate ./internal/assets` (cmd/genassets); GNU coreutils `timeout`.

**Spec:** `docs/superpowers/specs/2026-10-02-run-task-worker-tests-directly-in-the-foreground-not-through-design.md`
on the `docket` metadata branch (worktree copy:
`/Users/homer/dev/docket/.docket/docs/superpowers/specs/2026-10-02-run-task-worker-tests-directly-in-the-foreground-not-through-design.md`).
Read it before starting any task; it is the design authority.

## Global Constraints

- **Run every focused test directly, in the foreground, in the feature worktree, wrapped in GNU `timeout`:**
  `timeout --kill-after=10s 10m go test -count=1 ./internal/repoguard -run '<regex>'` (use
  `gtimeout` if `timeout` is missing). This change's tests are never started through a gate drive,
  and no task ever runs the full suite (`go run ./cmd/docket development test`). The full suite is
  the controller's build gate. Always pass `-count=1`: a cached `ok` is not evidence.
- The time limit is exactly `timeout --kill-after=10s 10m <test command>`. Exit `124` or `137` means
  the limit was hit; `125`, `126`, and `127` mean `timeout` or the command could not run. Neither is red.
- Worker outcomes are exactly `COMPLETE | NEEDS_ESCALATION | BLOCKED`. The retired outcome is never
  named in the controller's outcome text.
- **No Go production code changes.** The task-owned drive machinery (`--owner task`, scopes,
  predecessor receipts, `acknowledge`, `takeover`) stays in the Go catalog, unused, until change
  0489. Only `_test.go` files under `internal/repoguard/` change in Go.
- **The run id stays on every build-owned start:** Step 5's suite gate, each post-repair attempt,
  and Step 6's evidence re-mint and re-gates. It leaves only the worker path. `run.cancel` depends
  on it until change 0491.
- **No plan task writes an ADR file.** The new ADR (superseding ADR-0117, `relates_to` ADR-0107 and
  ADR-0024) is recorded by the coordinator through `docket-adr`.
- **Every edit under `skills/` or `cursor-rules/` is followed by `go generate ./internal/assets`**,
  and the regenerated `internal/assets/embedded/` paths are staged in the same commit. `TestEmbeddedMatchesAuthored`
  enforces the match.
- Never hand-edit the managed `docket:dispatch` block in `AGENTS.md`. `CLAUDE.md` is a symlink to
  it. Regenerate it through the install path exactly as Task 1 Step 5 shows.
- Never restore deleted prose to keep a grep green. Repoint or retire the assert instead
  (learnings: restatement-accumulates-its-own-guards, test-premise-deleted-not-regated).
- Mutation probes restore from a backup copy (`cp f f.bak; <mutate>; <run>; mv -f f.bak f`), never
  `git checkout --`. Confirm that each mutation actually landed before trusting a green result.
- Stage by explicit path only; never `git add -A` / `git add .` / `git commit -a`.

## Review Focus

These are the five spec-implied inputs most likely to bite a user that no behavioral test can
exercise, because the behavior lives in agent prose. Each one is pinned by a sentinel in the task
named.

1. **A macOS host without GNU coreutils (`timeout` absent, maybe only `gtimeout`):** the worker
   uses `gtimeout`, or returns `BLOCKED` naming GNU coreutils, and never runs a test unwrapped. The
   install docs tell the user what to install. Pinned by `worker-gtimeout-fallback` and
   `worker-never-unlimited` (Task 2) and `TestInstallPrerequisiteDocContracts` (Task 1).
2. **A focused test that outlives the harness's default shell-call timeout** (for example a
   multi-minute `go test -race` package run under a 2-minute default): the worker raises the
   harness timeout to at least 10 minutes and treats a cut-off call as the limit being hit, never
   starting a second copy. Pinned by `worker-harness-wait` (Task 2).
3. **A stale-install worker that still returns the retired `WAITING`:** the controller treats any
   token outside the three outcomes as malformed and halts. It never accepts or continues. Pinned
   by `controller-unknown-outcome-halts` (Task 2).
4. **A repair worker that reruns the full suite out of habit** (double-charging attempts, or hitting
   `worktree-busy`): both contracts say the repair worker reruns only the failing tests, and the
   controller starts the next counted attempt. Pinned by `worker-never-full-suite` and
   `repair-never-full-suite` (Task 2).
5. **An in-place mutation probe left unrestored** (the 0486 trap, now with no drive to halt): the
   worker contract requires restoring the code and rerunning. Pinned by `worker-mutation-restore`
   (Task 2).

## File map

| File | Task | Responsibility after the change |
|---|---|---|
| `cursor-rules/run-tracker.md` | 1 | `--run-id` flag list loses `gate drive prepare-scope` |
| `AGENTS.md` (managed block; `CLAUDE.md` symlinks to it) | 1 | regenerated from the run-tracker source through the install path |
| `docs/reference/glossary.md` | 1 | run id, run fence, gate drive, and docket-build entries match the new contract |
| `docs/install/install.md` | 1 | *What you need first* names GNU coreutils `timeout` |
| `internal/repoguard/prose_contracts_test.go` | 1, 2 | Task 1: `TestInstallPrerequisiteDocContracts`; Task 2: `TestTaskTestTimeLimitContract`, 0459 rows retired |
| `skills/docket-build-task/SKILL.md` | 2 | worker contract: direct tests under `timeout`, three outcomes, no gate operation |
| `skills/docket-build/SKILL.md` | 2 | controller: plain dispatch payload, time-limit audit, controller-run post-repair attempts |
| `skills/docket-implement-next/SKILL.md` | 2 | run-context/run-id sentence names only build-owned starts |
| `skills/docket-build/references/gate-caller-loop.md` | 2 | callers are the full-suite gates; scope ops and takeover removed |
| `internal/repoguard/gatedrive_scope_identity_test.go` → `gatedrive_task_absence_test.go` | 2 | `TestNoTaskOwnedDriveInstructions` (absence guard + non-vacuity companion) |
| `internal/repoguard/gatedrive_run_id_thread_test.go` | 2 | keeps prong B + `TestRunTrackerCopiesRunIDIntoDispatchPrompt` + `TestCodexRequestFileCarriesRunID` |
| `internal/repoguard/gatedriver_test.go` | 2 | detector D retired |
| `internal/repoguard/gatedrive_json_capture_test.go` | 2 | contract prongs kept; failure-mapping clause repointed; floors re-counted |
| `internal/repoguard/gatecapture_reserved_param_test.go` | 2 | floors re-counted |
| `internal/repoguard/inline_role_stop_test.go` | 2 | anchor `Return exactly one of three outcomes` |
| `skills/docket-build/references/gate-execution.md` | 3 | 0359 section becomes run-boundary outstanding human verification |
| `internal/repoguard/testexec_boundary_test.go` | 3 | header rationale narrowed to the full-suite channel |
| `skills/docket-implement-next/references/fix-loop.md` | 3 | confirmed unchanged (no edit) |
| `internal/repoguard/budgets_test.go` | 4 | ceilings lowered to the new sizes; comments rewritten |
| `internal/assets/embedded/**` | 1, 2, 3 | regenerated mirror, staged with each task |

Site list provenance: `git grep` for the retired vocabulary (`--owner task`, `prepare-scope`,
`predecessor`, `acknowledge`, `takeover`, `child-cap` / child capability, scope bundle,
`scope-id`, `WAITING` as a worker outcome, the repair worker's run-id exception) over the whole
repo, excluding point-in-time records (`docs/superpowers/plans`, `docs/superpowers/specs`,
`docs/results`, `docs/changes`, `docs/adrs`). The executable Go hits (`internal/app`,
`internal/cli`, `internal/gatedrive`, and their tests) are the catalog machinery that change 0489
deletes, and they stay. The repoguard tests are handled in Tasks 2–4. The prose hits are exactly the
rows above, plus two left alone deliberately: `docs/reference/glossary.md`'s *Worktree changed*
entry, which describes the still-shipped `takeover` refusal mechanism rather than a workflow, and
`docs/concepts/build-tiers-and-gate.md`, which already says workers run their own focused tests.

---

### Task 1: Run-tracker rule, regenerated AGENTS.md block, glossary, and install prerequisite

**Build tier:** standard

**Files:**
- Modify: `cursor-rules/run-tracker.md` (step 1's `--run-id` flag list)
- Modify (regenerated, never hand-edited): `AGENTS.md` (managed `docket:dispatch` block)
- Modify: `docs/reference/glossary.md` (four entries)
- Modify: `docs/install/install.md` (*What you need first*)
- Modify: `internal/repoguard/prose_contracts_test.go` (add `prerequisiteDocContracts` + `TestInstallPrerequisiteDocContracts`)
- Regenerate: `internal/assets/embedded/manifest.json`, `internal/assets/embedded/tree/cursor-rules/run-tracker.md`

**Interfaces:**
- Consumes: `docSectionContract` and `scanDocSection` (existing, `prose_contracts_test.go`);
  `guardRoot`, `readMaintained` (existing, `guards_test.go`).
- Produces: `cursor-rules/run-tracker.md` free of every retired token. Task 2's absence guard scans
  `cursor-rules/` and fails if this task did not land.

- [ ] **Step 1: Write the failing prerequisite contract test**

In `internal/repoguard/prose_contracts_test.go`, insert immediately above the line
`// scanDocSection is the whole detector, exposed so non_vacuity exercises the`:

```go
// change 0488 — build workers run every focused test under GNU `timeout`, so the
// install prerequisites name GNU coreutils and the macOS `gtimeout` spelling.
var prerequisiteDocContracts = []docSectionContract{
	{change: "change_0488_coreutils_prerequisite", file: "docs/install/install.md",
		section: "## What you need first", terminator: "## Install docket on your machine",
		present: []string{
			"**GNU coreutils `timeout`.** Build workers run each focused test under `timeout --kill-after=10s 10m`.",
			"on macOS, run `brew install coreutils` (it may install as `gtimeout`, which workers also accept)",
		}},
}

func TestInstallPrerequisiteDocContracts(t *testing.T) {
	root := guardRoot(t)
	for _, c := range prerequisiteDocContracts {
		for _, v := range scanDocSection(readMaintained(t, root, c.file), c) {
			t.Errorf("[%s] %s", c.change, v)
		}
	}
}

```

- [ ] **Step 2: Run it and confirm it fails for the intended reason**

Run: `timeout --kill-after=10s 10m go test -count=1 ./internal/repoguard -run 'TestInstallPrerequisiteDocContracts$'`
Expected: exit 1 with two `missing required clause` lines naming `docs/install/install.md §"## What you need first"`.

- [ ] **Step 3: Add the prerequisite bullet**

In `docs/install/install.md`, directly after the bullet that ends
`implementer opens pull requests with \`gh\`.`, insert:

```markdown
- **GNU coreutils `timeout`.** Build workers run each focused test under
  `timeout --kill-after=10s 10m`. It is standard on Linux; on macOS, run `brew install coreutils`
  (it may install as `gtimeout`, which workers also accept).
```

Rerun Step 2's command and expect exit 0.

- [ ] **Step 4: Edit the run-tracker rule and regenerate the mirror**

In `cursor-rules/run-tracker.md`, replace the line

```
   flag (`agent.enter`, `gate drive start`, `gate drive prepare-scope`). Add `--resume <id>` to
```

with

```
   flag (`agent.enter`, `gate drive start`). Add `--resume <id>` to
```

Then run `go generate ./internal/assets` (expect a `genassets: wrote internal/assets/embedded (…)` line).

Run: `timeout --kill-after=10s 10m go test -count=1 ./internal/repoguard -run 'TestCommittedCodexDispatchMatchesGenerator$'`
Expected: exit 1, `AGENTS.md dispatch block is stale; regenerate it from the reposeed dispatch interior`.
This is the RED for the AGENTS.md half: the committed block no longer matches the generator.

- [ ] **Step 5: Regenerate the AGENTS.md managed block through the install path**

Verified at plan time against a scratch clone and a linked worktree: install refuses to overwrite
a block it has no ownership record for (`ownership-conflict`). The sanctioned remedy it prints is
to delete the block and rerun. With a throwaway `HOME`, nothing outside the worktree's `AGENTS.md`
is touched. Run this from the feature worktree root:

```bash
WT="$(git rev-parse --show-toplevel)"
S="$(mktemp -d "${TMPDIR:-/tmp}/regen-0488.XXXXXX")"
go build -o "$S/docket" ./cmd/docket
python3 - "$WT/AGENTS.md" <<'EOF'
import sys
p = sys.argv[1]
s = open(p).read()
start, end = '<!-- docket:dispatch:start', '<!-- docket:dispatch:end -->'
assert s.count(start) == 1 and s.count(end) == 1, 'dispatch markers missing or duplicated; refusing to edit'
a, b = s.index(start), s.index(end)
assert a < b, 'dispatch markers out of order; refusing to edit'
b += len(end) + (1 if s[b + len(end):b + len(end) + 1] == '\n' else 0)
open(p, 'w').write(s[:a] + s[b:])
EOF
H="$S/home"; mkdir -p "$H"
HOME="$H" XDG_CONFIG_HOME="$H/.config" XDG_DATA_HOME="$H/.local/share" \
  XDG_STATE_HOME="$H/.local/state" XDG_CACHE_HOME="$H/.cache" \
  "$S/docket" install --repo-dir "$WT" --harness opencode
git diff --stat -- AGENTS.md
```

Expected: `install: applied`, and `AGENTS.md | 2 +-`. The only change is the single flag-list
line, identical to Step 4's. `git status --short` must show nothing beyond this task's paths. If
anything else changed, stop and return `BLOCKED` with the extra paths.

Run: `timeout --kill-after=10s 10m go test -count=1 ./internal/repoguard -run 'TestCommittedCodexDispatchMatchesGenerator$|TestRunTrackerCopiesRunIDIntoDispatchPrompt$|TestDispatchBlockBudget$'`
Expected: exit 0.

- [ ] **Step 6: Update the glossary (documentation only)**

In `docs/reference/glossary.md`, make these four replacements.

(a) *Start / run key / run id / run context*. Replace

```
**Starting** a run (`run.start`) mints three values before a dispatch: the **run key** (ties a finish
to this launch), the **run id** (the id of this run, threaded into cancel and drive flags), and
the **run context** (a token). The run context and the run id are both copied into the
implement-next dispatch prompt; a scope prepared with `--run-id` hands that run id to every scoped
start under it, so build-task workers never receive it (except the repair worker, for its
build-owned post-fix re-run). It prints
```

with

```
**Starting** a run (`run.start`) mints three values before a dispatch: the **run key** (ties a finish
to this launch), the **run id** (the id of this run, threaded into cancel and build-owned drive
flags), and the **run context** (a token). The run context and the run id are both copied into the
implement-next dispatch prompt; build-task workers never receive either, because they run their
focused tests directly and call no gate operation. It prints
```

(b) *Run fence*. Replace

```
attach to it. A fenced run is never restored, and a scope prepared with `--run-id` lets the fence also revoke a later
takeover.
```

with

```
attach to it. A fenced run is never restored.
```

(c) *Gate drive / slice / owner generation / handoff / takeover*. Keep the heading unchanged so the
anchor `#gate-drive--slice--owner-generation--handoff--takeover` and its two index links stay
valid. Replace everything from `A **gate drive** runs a gate in resumable` through the closing
` ``` ` of that entry's `sh` block with:

````markdown
A **gate drive** runs a gate in resumable **slices** so no agent has to block for the whole suite.
Each drive has an **owner generation**; ownership moves by **handoff** (a single-use token the next
owner **claims**).

**Used for:** the build and finalize suite gates. Build-task workers never start a drive: they run
their focused tests directly under a fixed `timeout`. The component making these calls is the **gate
driver**. A forked controller drives the suite with inline, blocking `advance` calls — it must never
background the suite and yield. The catalog still carries the recovery-scope operations
(`prepare-scope`, `takeover`, `acknowledge`) that once served build-task workers; no workflow uses
them, and change 0489 removes them.

```sh
docket gate drive start   --repo-dir . --owner build --run-root <dir> --run-id <run-id> -- <suite argv>
docket gate drive advance --drive-id <id> --owner-gen <gen>
docket gate drive handoff --drive-id <id> --owner-gen <gen>
docket gate drive claim   --drive-id <id> --handoff-id <token>
```
````

(d) *docket-build*. Replace

```
directly. Worker outcomes are `COMPLETE`, `WAITING`, `NEEDS_ESCALATION`, or `BLOCKED`, and a
```

with

```
directly. Worker outcomes are `COMPLETE`, `NEEDS_ESCALATION`, or `BLOCKED`, and a
```

Verify: `git grep -n -E -e 'prepare-scope|scoped start|post-fix re-run|WAITING., .NEEDS' -- docs/reference/glossary.md`
Expect only the new catalog sentence in (c), which names `prepare-scope` as unused.

- [ ] **Step 7: Mutation-check the new prerequisite contract**

```bash
f=docs/install/install.md; cp "$f" "$f.bak"
perl -0pi -e 's/which workers also\s+accept/which workers accept/' "$f"
c=$(tr -s '[:space:]' ' ' < "$f"); grep -c -F -- 'which workers accept' <<<"$c"   # must print 1 (mutation landed)
timeout --kill-after=10s 10m go test -count=1 ./internal/repoguard -run 'TestInstallPrerequisiteDocContracts$'   # must exit 1
mv -f "$f.bak" "$f"
timeout --kill-after=10s 10m go test -count=1 ./internal/repoguard -run 'TestInstallPrerequisiteDocContracts$'   # must exit 0
```

- [ ] **Step 8: Run the task's focused set**

Run: `timeout --kill-after=10s 10m go test -count=1 ./internal/repoguard -run 'TestInstallPrerequisiteDocContracts$|TestUninstallCollectionDocContracts$|TestCommittedCodexDispatchMatchesGenerator$|TestRunTrackerCopiesRunIDIntoDispatchPrompt$|TestDispatchBlockBudget$|TestProseContracts$'`
and `timeout --kill-after=10s 10m go test -count=1 ./internal/assets`.
Expected: both exit 0.

- [ ] **Step 9: Commit**

```bash
git add cursor-rules/run-tracker.md AGENTS.md docs/reference/glossary.md docs/install/install.md \
  internal/repoguard/prose_contracts_test.go \
  internal/assets/embedded/manifest.json internal/assets/embedded/tree/cursor-rules/run-tracker.md
git commit -m "docs: drop worker drive scopes from the run-tracker rule, glossary, and install prerequisites (change 0488)"
```

(`CLAUDE.md` is a symlink to `AGENTS.md`, so it carries no separate diff.)

---

### Task 2: Worker and controller contract cutover, with the absence guard and time-limit sentinels

**Build tier:** premium. This is the change's core. Five prose files and seven guard files are
bound together: every retired guard is replaced in this same commit by the absence guard written
in Step 1.

**Files:**
- Rename + rewrite: `internal/repoguard/gatedrive_scope_identity_test.go` → `internal/repoguard/gatedrive_task_absence_test.go`
- Modify: `internal/repoguard/gatedrive_run_id_thread_test.go`
- Modify: `internal/repoguard/gatedriver_test.go`
- Modify: `internal/repoguard/prose_contracts_test.go`
- Modify: `internal/repoguard/gatedrive_json_capture_test.go`
- Modify: `internal/repoguard/gatecapture_reserved_param_test.go`
- Modify: `internal/repoguard/inline_role_stop_test.go`
- Modify: `skills/docket-build-task/SKILL.md`
- Modify: `skills/docket-build/SKILL.md`
- Modify: `skills/docket-implement-next/SKILL.md`
- Modify: `skills/docket-build/references/gate-caller-loop.md`
- Regenerate: `internal/assets/embedded/manifest.json` and the four mirrored `internal/assets/embedded/tree/skills/...` files

**Interfaces:**
- Consumes (existing): `paragraphs` and `sharedContractRel` (`gatedrive_json_capture_test.go`); `isWorkflowMD`
  (`gatedriver_test.go`); `guardRoot`, `maintainedPop`, `readMaintained`, `hasExt`, `underDir`
  (`guards_test.go`); `collapseWS` (`finalize_rebuild_test.go`); `cursor-rules/run-tracker.md`
  already clean (Task 1).
- Produces: `TestNoTaskOwnedDriveInstructions`; `TestTaskTestTimeLimitContract`; package-level
  `buildSkillRel`, `buildTaskSkillRel`, `runTrackerRuleRel`, `workerOutcomeList`, `startOpRe`,
  `taskDriveTokenRe`, `waitFwd`, `waitRev`, `isTaskDriveCorpus`, `taskDriveViolations` (new file);
  `ownerBuildRe`, `runIDFlagRe`, `isBuildOwnerStartSite`, `carriesRunID` (kept in
  `gatedrive_run_id_thread_test.go`). The symbols `ownerTaskRe`, `isScopedTaskStartSite`,
  `missingScopedStartFlags`, `bundleSentence`, `requiredScopedStartFlags`, `requiredStartRowFlags`,
  `requiredBundleElems`, `requiredBlockElems`, `bundleBindRe`, `childOnlyRe`, `parentStayRe`,
  `prepareScopeOpRe`, `repairRerunRe`, `isPrepareScopeSite`, `isRepairRerunSite`,
  `scanWaitingHandoff`, `namesHandoff`, and `forbidsBare` cease to exist. Nothing else in the
  package references them. Confirm with
  `git grep -n -E 'ownerTaskRe|isScopedTaskStartSite|isPrepareScopeSite|isRepairRerunSite|scanWaitingHandoff|namesHandoff|forbidsBare|bundleBindRe' -- internal/`
  returning nothing after Step 3.

- [ ] **Step 1: Write the absence guard (converted from the scoped-start identity guard)**

```bash
git mv internal/repoguard/gatedrive_scope_identity_test.go internal/repoguard/gatedrive_task_absence_test.go
```

Replace the entire content of `internal/repoguard/gatedrive_task_absence_test.go` with:

```go
package repoguard

// Change 0488 replaces change 0416's scoped-start identity guard (its subject,
// a build-task worker's task-owned drive, is gone) with that guard's inverse.
// Build-task workers run every test directly in the foreground under
// `timeout --kill-after=10s 10m` and call no gate operation; the gate driver
// serves only the full-suite gates. Three parts:
//   (1) no paragraph of the maintained workflow-markdown corpus (isWorkflowMD:
//       skills/ and agents/ plus their embedded mirrors) or the dispatch-rule
//       surface (cursor-rules/ plus its embedded mirror) instructs a
//       task-owned drive or its recovery-scope protocol: `--owner task`, the
//       prepare-scope / acknowledge / takeover operations (dotted catalog id
//       or spaced CLI spelling), the predecessor-receipt flags, or the scope
//       credentials --scope-id / --child-cap;
//   (2) no paragraph pairs WAITING with a worker outcome (COMPLETE,
//       NEEDS_ESCALATION, BLOCKED): gatedriver_test.go's retired detector D
//       shape (waitFwd/waitRev), inverted from "must name a handoff" to "must
//       not occur";
//   (3) a non-vacuity companion through the SAME extractor (the same corpus
//       walk and paragraphs split): the build controller's build-owned
//       gate.drive.start site and docket-build-task's outcome list must still
//       be found, so a dead extractor or a moved corpus reddens instead of
//       passing.
// Sites are discovered by scanning the whole corpus, never a per-file list;
// the forbidden token set is the asserted property.
// Residual risk, recorded not hidden (byte-pattern-guard-matches-a-spelling):
// a retired operation named without its gate.drive / gate drive prefix (a
// bare `takeover`) is not matched — the bare words are ordinary English, and
// the Go catalog keeps these operations until change 0489 deletes them.

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

const (
	buildSkillRel     = "skills/docket-build/SKILL.md"
	buildTaskSkillRel = "skills/docket-build-task/SKILL.md"
	runTrackerRuleRel = "cursor-rules/run-tracker.md"
	// workerOutcomeList is docket-build-task's return-schema outcome line.
	workerOutcomeList = "OUTCOME: COMPLETE | NEEDS_ESCALATION | BLOCKED"
)

var (
	// startOpRe also serves gatedrive_run_id_thread_test.go's build-owned sites.
	startOpRe = regexp.MustCompile(`gate\.drive\.start`)

	// taskDriveTokenRe: one alternative per forbidden class, each right-bounded
	// so a longer token (--owner taskforce, --scope-ids) never matches.
	taskDriveTokenRe = regexp.MustCompile(`--owner[ =]task(?:[^a-z-]|$)` +
		`|gate[. ]drive[. ](?:prepare-scope|acknowledge|takeover)(?:[^a-z-]|$)` +
		`|--predecessor-(?:drive-id|owner-gen)(?:[^a-z-]|$)` +
		`|--(?:scope-id|child-cap)(?:[^a-z-]|$)`)

	// waitFwd / waitRev: WAITING within 45 characters of a worker outcome, in
	// either order (retired detector D's shape; waitRev also matches a
	// paragraph-final WAITING).
	waitFwd = regexp.MustCompile(`(^|[^-A-Za-z])WAITING([^-A-Za-z]).{0,45}(COMPLETE|BLOCKED|NEEDS_ESCALATION)`)
	waitRev = regexp.MustCompile(`(COMPLETE|BLOCKED|NEEDS_ESCALATION)([^-A-Za-z]).{0,45}([^-A-Za-z])WAITING([^-A-Za-z]|$)`)
)

// isTaskDriveCorpus: the workflow-markdown corpus plus the dispatch-rule
// surface the run-tracker block renders from (cursor-rules/ and its mirror).
func isTaskDriveCorpus(rel string) bool {
	return isWorkflowMD(rel) ||
		(hasExt(rel, ".md") && underDir(rel, "cursor-rules", "internal/assets/embedded/tree/cursor-rules"))
}

// taskDriveViolations is the whole detector over one file's paragraphs,
// exposed so non_vacuity exercises it directly.
func taskDriveViolations(rel string, paras []string) []string {
	var v []string
	for _, p := range paras {
		if m := taskDriveTokenRe.FindString(p); m != "" {
			v = append(v, fmt.Sprintf("%s: instructs a task-owned drive or its recovery scope (%q): %.160s", rel, strings.TrimSpace(m), p))
		}
		flat := strings.ReplaceAll(p, "`", "")
		if waitFwd.MatchString(flat) || waitRev.MatchString(flat) {
			v = append(v, fmt.Sprintf("%s: pairs WAITING with a worker outcome (workers return COMPLETE, NEEDS_ESCALATION, or BLOCKED): %.160s", rel, p))
		}
	}
	return v
}

func TestNoTaskOwnedDriveInstructions(t *testing.T) {
	root := guardRoot(t)
	var violations []string
	scanned := map[string]bool{}
	buildStartSeen, outcomeListSeen := false, false
	for _, rel := range maintainedPop(t, root) {
		if !isTaskDriveCorpus(rel) {
			continue
		}
		scanned[rel] = true
		paras := paragraphs(readMaintained(t, root, rel))
		violations = append(violations, taskDriveViolations(rel, paras)...)
		for _, p := range paras {
			if rel == buildSkillRel && startOpRe.MatchString(p) && ownerBuildRe.MatchString(p) {
				buildStartSeen = true
			}
			if rel == buildTaskSkillRel && strings.Contains(p, workerOutcomeList) {
				outcomeListSeen = true
			}
		}
	}

	// Population and non-vacuity FIRST (a vacuous scan passes every negative).
	if len(scanned) < 20 {
		t.Fatalf("population floor: only %d corpus files scanned (expected >= 20)", len(scanned))
	}
	for _, rel := range []string{
		buildSkillRel, buildTaskSkillRel, runTrackerRuleRel,
		"internal/assets/embedded/tree/" + buildTaskSkillRel,
		"internal/assets/embedded/tree/" + runTrackerRuleRel,
	} {
		if !scanned[rel] {
			t.Errorf("coverage floor: %s was not scanned (corpus predicate or walk drifted)", rel)
		}
	}
	if !buildStartSeen {
		t.Errorf("non-vacuity: the extractor found no build-owned gate.drive.start paragraph in %s", buildSkillRel)
	}
	if !outcomeListSeen {
		t.Errorf("non-vacuity: the extractor found no %q paragraph in %s", workerOutcomeList, buildTaskSkillRel)
	}
	if len(violations) != 0 {
		t.Errorf("task-owned drive instructions (%d) — workers run tests directly under timeout and call no gate operation:\n%s",
			len(violations), strings.Join(violations, "\n"))
	}

	t.Run("non_vacuity", func(t *testing.T) {
		flagged := []string{
			"run the `gate.drive.start` operation with `--owner task --json -- <cmd>`",
			"start it with `--owner=task`",
			"run the `gate.drive.prepare-scope` operation with `--change-id <id>`",
			"run `docket gate drive prepare-scope --change-id 1`",
			"perform the `gate.drive.acknowledge` operation",
			"run `docket gate drive acknowledge --drive-id <id>`",
			"run the `gate.drive.takeover` operation",
			"pass `--predecessor-drive-id <id>`",
			"pass `--predecessor-owner-gen <gen>`",
			"pass `--scope-id <id>` through",
			"hand over `--child-cap <token>`",
			"Return `COMPLETE`, `WAITING`, or `BLOCKED`.",
			"OUTCOME: COMPLETE | NEEDS_ESCALATION | BLOCKED | WAITING",
			"`WAITING` — then `NEEDS_ESCALATION` follows",
		}
		for _, s := range flagged {
			if got := taskDriveViolations("skills/x/SKILL.md", paragraphs(s)); len(got) == 0 {
				t.Errorf("missed a forbidden instruction: %q", s)
			}
		}
		clean := []string{
			"the `gate.drive.start` operation with `--owner build --run-id <run-id> --json`",
			"`WAITING` is the only nonterminal disposition and the only one that advances again",
			workerOutcomeList,
			"the `--owner taskforce` flag, `--scope-ids`, and `--child-capacity`",
			"run the `maintenance.sweep` operation with `--scope full --json`",
			"a run-waiting verdict is never COMPLETE",
			"the catalog's takeover and acknowledge operations",
		}
		for _, s := range clean {
			if got := taskDriveViolations("skills/x/SKILL.md", paragraphs(s)); len(got) != 0 {
				t.Errorf("wrongly flagged a permitted paragraph %q: %v", s, got)
			}
		}
		// Whitespace collapse: a wrapped token is still one matchable paragraph.
		for _, wrapped := range []string{
			"the `gate.drive.prepare-scope`\noperation",
			"run `docket gate drive\nacknowledge`",
			"Return COMPLETE or\nWAITING",
		} {
			if got := taskDriveViolations("skills/x/SKILL.md", paragraphs(wrapped)); len(got) == 0 {
				t.Errorf("whitespace collapse failed: %q", wrapped)
			}
		}
		// Paragraph scoping: WAITING and an outcome in different paragraphs are no pair.
		if got := taskDriveViolations("skills/x/SKILL.md", paragraphs("`WAITING` ends a slice.\n\nReturn COMPLETE.")); len(got) != 0 {
			t.Errorf("a WAITING/outcome pair across paragraphs was wrongly flagged: %v", got)
		}
		if !isTaskDriveCorpus(runTrackerRuleRel) ||
			!isTaskDriveCorpus("internal/assets/embedded/tree/"+runTrackerRuleRel) ||
			isTaskDriveCorpus("docs/reference/glossary.md") {
			t.Errorf("corpus predicate misclassified the dispatch-rule surface or docs/")
		}
	})
}
```

- [ ] **Step 2: Retire detector D from `gatedriver_test.go`**

Detector D's shape now lives, inverted, in the guard above. Both files declare `waitFwd` and
`waitRev`, so this step is also needed for the package to compile.

1. In the header comment, replace

   ```
   // directly, never re-parses raw observation state, never recreates a poll loop,
   // and every task-level WAITING return names an explicit ownership handoff.
   //
   // Four detectors,
   ```

   with

   ```
   // directly, never re-parses raw observation state, and never recreates a poll
   // loop. Change 0488 retired detector D (a task-level WAITING return must name an
   // explicit ownership handoff): workers no longer return WAITING, and its shape
   // lives on, inverted, in TestNoTaskOwnedDriveInstructions.
   //
   // Three detectors,
   ```

   and replace `// Go/handoff detectors carry the rest of the teeth.` with
   `// Go detector carries the rest of the teeth.`
2. Delete the four `var` lines `waitFwd`, `waitRev`, `namesHandoff`, `forbidsBare` and the blank
   line before them.
3. Delete the whole `scanWaitingHandoff` function, including its `// scanWaitingHandoff (D): …`
   comment.
4. In `TestGateDriverBoundary`, replace

   ```go
   	// The driver surface being protected actually exists, and detector D has live
   	// input (a real WAITING contract).
   	if !slices_containsPrefix(pop, "internal/gatedrive/") {
   		t.Fatalf("the native gate driver package internal/gatedrive is absent")
   	}
   	buildTask := readMaintained(t, root, "skills/docket-build-task/SKILL.md")
   	if !regexp.MustCompile(`(^|[^-A-Za-z])WAITING([^-A-Za-z])`).MatchString(buildTask) {
   		t.Fatalf("detector D has no live input: skills/docket-build-task/SKILL.md declares no WAITING outcome")
   	}
   ```

   with

   ```go
   	// The driver surface being protected actually exists.
   	if !slices_containsPrefix(pop, "internal/gatedrive/") {
   		t.Fatalf("the native gate driver package internal/gatedrive is absent")
   	}
   ```

   delete the line `violations = append(violations, scanWaitingHandoff(rel, c)...)`, and delete
   the `// (D) WAITING contract missing the handoff clauses vs the real, complete one.` block at
   the end of `non_vacuity` (its `bad` case and the `buildTask` case).
5. Run `gofmt -w internal/repoguard/gatedriver_test.go`.

- [ ] **Step 3: Narrow `gatedrive_run_id_thread_test.go` to prong B**

Replace everything from the top of the file down to, but not including, the line
`// runTrackerRunIDCopyRe binds the copy instruction to the run id AND to its` with the block
below. `TestRunTrackerCopiesRunIDIntoDispatchPrompt` and `TestCodexRequestFileCarriesRunID` stay
byte-identical.

```go
package repoguard

// Change 0467 threaded run.start's run id into the build chain; change 0488
// retired its worker half (prepare-scope, task-owned starts, and the repair
// worker's re-run): build-task workers now run tests directly and call no gate
// operation, an absence TestNoTaskOwnedDriveInstructions guards. One prong
// remains over maintained workflow markdown (isWorkflowMD, so the embedded
// mirrors are scanned too): every build-owned gate.drive.start paragraph
// (--owner build) carries --run-id. run.cancel finds a run's suites by run id,
// so a build-owned start without it would survive a cancel (until change 0491
// retires the run id). Floors: docket-build carries both the final-gate start
// and the post-repair-attempt start the controller now makes itself, and
// docket-implement-next carries its own build-owned starts.
// TestRunTrackerCopiesRunIDIntoDispatchPrompt (below) binds the managed run-tracker
// source to copying the run into the dispatch prompt.
// Site discovery is keyed on syntactic shape, never a per-file allowlist; the
// shared caller contract (sharedContractRel) is the operation reference, not a
// caller, and is excluded.
// Residual risk, recorded not hidden: a build-owned start without the
// --owner build token in the same paragraph is not a site; at run time the
// driver still fences such a no-run-record start against a run-owned worktree
// (stale-run-id).

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/harness"
)

const implementNextSkillRel = "skills/docket-implement-next/SKILL.md"

var (
	ownerBuildRe = regexp.MustCompile(`--owner build(?:[^a-z-]|$)`)
	runIDFlagRe  = regexp.MustCompile(`--run-id(?:[^a-z-]|$)`)
)

// isBuildOwnerStartSite: a collapsed paragraph that references gate.drive.start
// AND carries the --owner build token.
func isBuildOwnerStartSite(p string) bool {
	return startOpRe.MatchString(p) && ownerBuildRe.MatchString(p)
}

// carriesRunID: the paragraph carries the --run-id flag token.
func carriesRunID(p string) bool { return runIDFlagRe.MatchString(p) }

func TestGateDriveRunIDThreaded(t *testing.T) {
	root := guardRoot(t)
	var violations []string
	buildSites := map[string]int{}
	for _, rel := range maintainedPop(t, root) {
		if !isWorkflowMD(rel) || strings.HasSuffix(rel, sharedContractRel) {
			continue
		}
		for _, p := range paragraphs(readMaintained(t, root, rel)) {
			if !isBuildOwnerStartSite(p) {
				continue
			}
			buildSites[rel]++
			if !carriesRunID(p) {
				violations = append(violations, fmt.Sprintf(
					"%s: build-owned gate.drive.start instruction lacks --run-id: %.160s", rel, p))
			}
		}
	}

	// Population floors FIRST (a vacuous scan passes every negative).
	mirror := func(rel string) []string { return []string{rel, "internal/assets/embedded/tree/" + rel} }
	for _, rel := range mirror(implementNextSkillRel) {
		if buildSites[rel] == 0 {
			t.Errorf("coverage floor: %s contributes no build-owned gate.drive.start site (scan or corpus drifted)", rel)
		}
	}
	for _, rel := range mirror(buildSkillRel) {
		// The final suite gate AND the post-repair attempt the controller starts itself.
		if buildSites[rel] < 2 {
			t.Errorf("coverage floor: %s must carry both the final-gate and the post-repair-attempt build-owned start sites, found %d", rel, buildSites[rel])
		}
	}
	if len(violations) != 0 {
		t.Errorf("run-id threading violations (%d):\n%s", len(violations), strings.Join(violations, "\n"))
	}

	t.Run("non_vacuity", func(t *testing.T) {
		build := "the `gate.drive.start` operation with `--owner build --run-id <run-id> --json`"
		if !isBuildOwnerStartSite(build) || !carriesRunID(build) {
			t.Fatalf("a complete build-owned start was misclassified")
		}
		if carriesRunID(strings.Replace(build, "--run-id <run-id> ", "", 1)) {
			t.Errorf("stripping --run-id from a build-owned start was not detected")
		}
		if isBuildOwnerStartSite("the `gate.drive.start` operation with `--owner builder --json`") {
			t.Errorf("--owner build token boundary failed: 'builder' matched")
		}
		if isBuildOwnerStartSite("the `gate.drive.start` operation with `--owner task --json`") {
			t.Errorf("a task-owned start was classified as build-owned")
		}
		if carriesRunID("pass `--run-id-x <x>`") {
			t.Errorf("--run-id token boundary failed: '--run-id-x' matched")
		}
		wrapped := "start the next attempt with the `gate.drive.start`\noperation with `--owner build\n--run-id <run-id>`"
		if got := paragraphs(wrapped); len(got) != 1 || !isBuildOwnerStartSite(got[0]) || !carriesRunID(got[0]) {
			t.Errorf("whitespace collapse failed: a wrapped build-owned start did not match as one paragraph")
		}
	})
}

```

- [ ] **Step 4: Add the time-limit sentinels and retire the 0459 rows in `prose_contracts_test.go`**

1. Add `"regexp"` to the import block (between `"path/filepath"` and `"strings"`).
2. Delete the two `change_0459_scope_transferred` rows from `proseContracts`, together with their
   six-line `// change 0459 — a handed-off worker's scope authority ends at the parent's …`
   comment. Their subject (the WAITING-handoff continuation and its scope) is gone. The phrase-check
   total drops from 169 to 162, still above the table's floor of 40.
3. Append at the end of the file:

```go
// change 0488 — build-task workers run every test directly under a fixed
// 10-minute GNU timeout, read the result from the exit status, and report each
// command exactly as run; the build controller's time-limit audit reports a
// missing wrapper and is never a halting condition. Each clause is matched
// against a backtick-and-emphasis-stripped, whitespace-collapsed haystack
// (phrase-grep-over-wrapped-prose) and bound to its claim with one bounded
// gap (prose-guard-binds-phrase-to-claim).
var timeLimitContracts = []struct {
	name string
	file string
	re   *regexp.Regexp
}{
	{"worker-runs-under-timeout", buildTaskSkillRel, regexp.MustCompile(`timeout --kill-after=10s 10m <test command>`)},
	{"worker-limit-hit-is-not-red", buildTaskSkillRel, regexp.MustCompile(`\| 124 or 137 \| the 10-minute limit was hit[^|]{0,60}\| not red`)},
	{"worker-never-unlimited", buildTaskSkillRel, regexp.MustCompile(`Never run a test without the limit`)},
	{"worker-verification-as-run", buildTaskSkillRel, regexp.MustCompile(`VERIFICATION lists every test command exactly as it ran[^.]{0,60}timeout wrapper`)},
	{"controller-audits-wrapper", buildSkillRel, regexp.MustCompile(`Time-limit audit.{0,160}timeout --kill-after=10s 10m`)},
	{"controller-audit-never-halts", buildSkillRel, regexp.MustCompile(`The audit is never a halting condition and never makes a return malformed`)},
	// Review-focus pins (plan): the inputs the spec implies but no other test exercises.
	{"worker-gtimeout-fallback", buildTaskSkillRel, regexp.MustCompile(`Use gtimeout when timeout is not on PATH\. If neither exists, return BLOCKED naming the missing prerequisite \(GNU coreutils\)`)},
	{"worker-harness-wait", buildTaskSkillRel, regexp.MustCompile(`Raise the harness's own shell-call timeout to at least 10 minutes`)},
	{"worker-never-full-suite", buildTaskSkillRel, regexp.MustCompile(`Never run the configured full-suite command\. The full suite is the controller's gate`)},
	{"worker-mutation-restore", buildTaskSkillRel, regexp.MustCompile(`verify it turns red, then restore and run it again`)},
	{"controller-unknown-outcome-halts", buildSkillRel, regexp.MustCompile(`Valid outcomes are COMPLETE, NEEDS_ESCALATION, and BLOCKED; any other token, or a missing or malformed outcome, halts the build`)},
	{"repair-never-full-suite", buildSkillRel, regexp.MustCompile(`re-runs the failing tests directly as its focused check, and commits; it never runs the full suite`)},
}

// timeLimitHaystack strips inline-code and emphasis markers and collapses
// whitespace, so a re-flow or a re-emphasis never reddens a clause.
func timeLimitHaystack(content string) string {
	return collapseWS(strings.NewReplacer("`", "", "*", "").Replace(content))
}

func TestTaskTestTimeLimitContract(t *testing.T) {
	root := guardRoot(t)
	for _, c := range timeLimitContracts {
		if !c.re.MatchString(timeLimitHaystack(readMaintained(t, root, c.file))) {
			t.Errorf("%s lost its %s clause (pattern %v)", c.file, c.name, c.re)
		}
	}

	t.Run("non_vacuity", func(t *testing.T) {
		good := map[string]string{
			"worker-runs-under-timeout":        "```text\ntimeout --kill-after=10s 10m <test command>\n```",
			"worker-limit-hit-is-not-red":      "| `124` or `137` | the 10-minute limit was hit (TERM, or KILL after the\n10-second grace) | not red, and not by itself a reason to escalate |",
			"worker-never-unlimited":           "If neither exists, return `BLOCKED`. Never run a\ntest without the limit.",
			"worker-verification-as-run":       "`VERIFICATION` lists every test command exactly as it ran,\nincluding its `timeout` wrapper, with its exit status.",
			"controller-audits-wrapper":        "**Time-limit audit — visibility only.** For every return, check each test command in\n`VERIFICATION` for the `timeout --kill-after=10s 10m` wrapper (or `gtimeout`).",
			"controller-audit-never-halts":     "- The audit is never a halting\n  condition and never makes a return malformed.",
			"worker-gtimeout-fallback":         "- **Fallback.** Use `gtimeout` when `timeout` is not on `PATH`. If neither exists, return\n  `BLOCKED` naming the missing prerequisite (GNU coreutils).",
			"worker-harness-wait":              "Raise the harness's own shell-call timeout to\n  at least 10 minutes where the harness allows it",
			"worker-never-full-suite":          "- **Never run the configured full-suite command.** The full suite is the controller's gate.",
			"worker-mutation-restore":          "run the guard and verify it turns red, then restore and\n  run it again.",
			"controller-unknown-outcome-halts": "Valid outcomes are `COMPLETE`, `NEEDS_ESCALATION`, and `BLOCKED`; any other token, or a **missing or\nmalformed outcome, halts** the build.",
			"repair-never-full-suite":          "fixes it, re-runs the failing tests\n   directly as its focused check, and commits; it never runs the full suite, and",
		}
		bad := map[string]string{
			"worker-runs-under-timeout":        "run <test command> directly",
			"worker-limit-hit-is-not-red":      "| `124` or `137` | the 10-minute limit was hit | red |",
			"worker-never-unlimited":           "Run a test without the limit when timeout is missing.",
			"worker-verification-as-run":       "`VERIFICATION` lists the focused command. The timeout wrapper is optional.",
			"controller-audits-wrapper":        "**Time-limit audit — visibility only.** For every return, read `VERIFICATION`.",
			"controller-audit-never-halts":     "- The audit is a halting condition when the wrapper is missing.",
			"worker-gtimeout-fallback":         "- **Fallback.** Use `gtimeout` when `timeout` is not on `PATH`. If neither exists, run the test anyway.",
			"worker-harness-wait":              "Keep the harness's default shell-call timeout.",
			"worker-never-full-suite":          "- Run the configured full-suite command when unsure.",
			"worker-mutation-restore":          "run the guard and verify it turns red.",
			"controller-unknown-outcome-halts": "Valid outcomes are `COMPLETE`, `WAITING`, `NEEDS_ESCALATION`, and `BLOCKED`; a missing outcome halts the build.",
			"repair-never-full-suite":          "fixes it, re-runs the full suite, and commits",
		}
		for _, c := range timeLimitContracts {
			if !c.re.MatchString(timeLimitHaystack(good[c.name])) {
				t.Errorf("%s: the intended (wrapped) wording did not match", c.name)
			}
			if c.re.MatchString(timeLimitHaystack(bad[c.name])) {
				t.Errorf("%s: a wording that drops the claim still matched", c.name)
			}
		}
	})
}
```

Run `gofmt -w internal/repoguard/prose_contracts_test.go`.

- [ ] **Step 5: Run the new guards and confirm they fail for the intended reasons (RED)**

Run: `timeout --kill-after=10s 10m go test -count=1 ./internal/repoguard -run 'TestNoTaskOwnedDriveInstructions|TestTaskTestTimeLimitContract|TestGateDriveRunIDThreaded|TestGateDriverBoundary'`
Expected (verified at plan time against this exact tree): exit 1.
- `TestNoTaskOwnedDriveInstructions` reports about 26 violations, in `skills/docket-build-task/SKILL.md`, `skills/docket-build/SKILL.md`,
  `skills/docket-build/references/gate-caller-loop.md`, `skills/docket-implement-next/SKILL.md`,
  and their embedded mirrors. It also reports the companion line
  `non-vacuity: the extractor found no "OUTCOME: COMPLETE | NEEDS_ESCALATION | BLOCKED" paragraph`.
  Its `non_vacuity` subtest PASSES.
- `TestTaskTestTimeLimitContract` reports 12 `lost its … clause` lines; its `non_vacuity` PASSES.
- `TestGateDriveRunIDThreaded` and `TestGateDriverBoundary` PASS.

A compile error at this point means a step above was skipped. Run the symbol grep from
*Interfaces*.

- [ ] **Step 6: Rewrite the worker contract — `skills/docket-build-task/SKILL.md`**

(a) Intro. Replace

```
the branch, the worktree, the selected build tier, the routing reason, and this task's
start-ready drive scope bundle — its scope id, child capability, and every identity value the
scope pinned. You are a fresh worker: nothing carries over from earlier tasks
except the code and commits already on the branch.
```

with

```
the branch, the worktree, the selected build tier, and the routing reason. You are a fresh worker:
nothing carries over from earlier tasks except the code and commits already on the branch.
```

(b) Cycle step 4. Replace

```
4. Re-run the focused test set. Focused, not the whole suite: the controller runs the full suite
   once after every task.
```

with

```
4. Re-run the focused test set. Focused, not the whole suite: the controller runs the full suite
   once, after every task has committed.
```

(c) Replace everything from the line beginning `**Every test execution this task runs — baseline, RED, GREEN, focused re-run, ad-hoc`
up to, but not including, the line `Two obligations the cycle does not relax:`, with:

````markdown
**Run every test directly, in the foreground, under a fixed time limit.** Every test this task
runs — baseline, RED, GREEN, focused re-run, ad-hoc check, mutation probe — runs directly in the
feature worktree as:

```text
timeout --kill-after=10s 10m <test command>
```

- **Block until it exits.** Never background a test, and never end your turn while one runs. If
  the harness hands back a still-running session, keep waiting on that same session, and never
  start a second copy.
- **Make the harness wait at least as long.** Raise the harness's own shell-call timeout to at
  least 10 minutes where the harness allows it, so a shorter default never becomes the real limit.
  If the harness still cuts the call off, treat that as hitting the limit: `timeout` runs the test
  in its own process group, so that run still ends at the 10-minute deadline. Start no other test
  until it is gone; stop it yourself if you can see it.
- **Fallback.** Use `gtimeout` when `timeout` is not on `PATH`. If neither exists, return
  `BLOCKED` naming the missing prerequisite (GNU coreutils). Never run a test without the limit.
- **Never run the configured full-suite command.** The full suite is the controller's gate.
  "Focused" means the narrowest command that exercises this task.

Read the result from the **exit status, never from output text**:

| Exit status | Meaning | Your action |
|---|---|---|
| `0` | green | continue |
| `124` or `137` | the 10-minute limit was hit (TERM, or KILL after the 10-second grace) | not red, and not by itself a reason to escalate: narrow the command, or return `BLOCKED` naming it |
| `125`, `126`, `127` | `timeout` or the command could not run | not red: fix the invocation, or return `BLOCKED` |
| any other non-zero | red | the existing repair discretion |

You call no gate operation — no `gate.drive` operation and no raw `gate.launch`/`observe`/`stop`
operation. The gate driver serves the controller's full-suite gate, never a task's focused tests.

**An integration-repair task follows this same contract.** It fixes the cross-task failure,
re-runs the failing tests directly as its focused check, commits, and returns `COMPLETE`. It never
runs the full suite: the controller starts the next counted full-suite attempt itself.

````

(d) Mutation obligation. Replace

```
- A guard requires **mutation evidence**: remove or defeat the thing being guarded and verify the
  guard turns red. A guard you never watched fail is decoration.
```

with

```
- A guard requires **mutation evidence**: remove or defeat the thing being guarded, run the guard
  and verify it turns red, then restore and run it again. A guard you never watched fail is
  decoration.
```

(e) The commit. Replace

```
**exactly one successful task commit** exists for this task. Never commit on `WAITING`,
`NEEDS_ESCALATION`, or `BLOCKED`: leave the worktree as it stands so the next worker or the human can
read it.
```

with

```
**exactly one successful task commit** exists for this task. Never commit on `NEEDS_ESCALATION` or
`BLOCKED`: leave the worktree as it stands so the next worker or the human can read it.
```

(f) Outcomes. Replace `Return exactly one of four outcomes.` with `Return exactly one of three outcomes.`,
and delete the whole `- **\`WAITING\`** — a slice-bounded focused gate run is still live …` bullet
(five lines, ending `\`WAITING\` is neither repair nor escalation, and never accompanies a commit.`).

(g) Your return. Replace

````
Keep it short. The controller keeps only this; there are no brief files, task reports, or review
records.

```text
OUTCOME: COMPLETE | WAITING | NEEDS_ESCALATION | BLOCKED
TIER: <economy|standard|premium|max> — <one-line routing reason as given to you>
VERIFICATION: <the focused command you ran> -> <result>
TDD: <RED/GREEN evidence, or the three-part exception: why unsuitable / what replaced it / residual risk>
HANDOFF: <drive-id + single-use handoff token — REQUIRED on WAITING, omit otherwise>
COMMIT:
````

with

````
Keep it short. The controller keeps only this; there are no brief files, task reports, or review
records. `VERIFICATION` lists every test command exactly as it ran, including its `timeout` wrapper,
with its exit status — the controller audits that line.

```text
OUTCOME: COMPLETE | NEEDS_ESCALATION | BLOCKED
TIER: <economy|standard|premium|max> — <one-line routing reason as given to you>
VERIFICATION: <each test command exactly as it ran, timeout wrapper included> -> <exit status>
TDD: <RED/GREEN evidence, or the three-part exception: why unsuitable / what replaced it / residual risk>
COMMIT:
````

(the rest of the `COMMIT:` line and the `NOTES:` line stay unchanged).

- [ ] **Step 7: Rewrite the controller — `skills/docket-build/SKILL.md`**

(a) Dispatch block. Inside the `<!-- docket:feature-dispatch:start … -->` / `<!-- docket:feature-dispatch:end -->`
markers, replace everything from `**Before each worker dispatch, prepare its recovery scope:**`
up to, but not including, `Never dispatch a task reviewer, and` with:

```
Dispatch the selected tier agent **by name** — one of `docket-build-economy`,
`docket-build-standard`, `docket-build-premium`, or `docket-build-max` — foreground, one task at a
time; later tasks build on earlier task commits and share the worktree, so workers are strictly
sequential. Its dispatch payload contains:
Feature worktree: <absolute canonical feature-worktree root>
It also gives the worker the plan task text, applicable repository instructions, selected
tier and routing reason, and the return schema. No run context, run id, or capability goes into a
worker prompt: the worker runs its tests directly and calls no gate operation. 
```

The `Feature worktree: <absolute canonical feature-worktree root>` line must stay a raw line of its
own, byte-exact, between the markers (`TestFeatureDispatchPayloadsCarryCanonicalWorktree`). The
text continues on the same line with `Never dispatch a task reviewer, and`. Leave the rest of the
block unchanged.

(b) Reading a worker's return. Replace

```
Valid outcomes are `COMPLETE`, `WAITING`, `NEEDS_ESCALATION`, and `BLOCKED`; a **missing or malformed
outcome halts** the build.
```

with

```
Valid outcomes are `COMPLETE`, `NEEDS_ESCALATION`, and `BLOCKED`; any other token, or a **missing or
malformed outcome, halts** the build.
```

(c) Replace the whole `## Task-level WAITING and the continuation` section (from that heading up
to, but not including, `## Escalation`, which removes the *Exceptional branch* takeover paragraph
too) with this block. The block deliberately has no blank line inside it, so it stays one paragraph:

```markdown
**Time-limit audit — visibility only.** For every return, check each test command in
`VERIFICATION` for the `timeout --kill-after=10s 10m` wrapper (or `gtimeout`).
- A command without it, or a `VERIFICATION` line too unclear to tell, becomes one informational
  line in the results file's `## Verification performed` section, through the worker-surfaced
  findings path (*Build-findings checkpoint*): "task N: `<command>` ran without the 10-minute
  `timeout` wrapper — informational, no action required; the full-suite gate certified the branch."
- The audit is never a halting condition and never makes a return malformed. It never
  re-dispatches, escalates, or re-runs a test, and never casts doubt on the task's commit; it adds
  no operation, state, transaction, or commit of its own and rides on the results checkpoint the
  coordinator already writes.
- It catches an honest omission. It cannot catch a worker misreporting what it ran.

```

(d) Halting conditions. Replace `- **A worker return is malformed or unverifiable** — a missing or unparsable outcome, a \`COMPLETE\``
with `- **A worker return is malformed or unverifiable** — a missing, unparsable, or unknown outcome, a \`COMPLETE\``.

(e) The build gate opener. Replace

```
policy. A worker's passed *task* gate does **not** substitute for this final full-suite gate: task
gates cover only what each worker ran, and this is the one run that certifies the branch.
```

with

```
policy. A worker's focused tests do **not** substitute for this final full-suite gate: they cover
only what each worker ran, and this is the one run that certifies the branch.
```

(f) Replace `binds every full-suite run this role performs, including the repair worker's post-fix re-run below.`
with `binds every full-suite run this role performs, including every post-repair attempt below.`

(g) Red path. Replace everything from `**Red** → the build **never invokes review**.` up to, but not
including, `### Gate run posture` with:

```markdown
**Red** → the build **never invokes review**. `build_max_attempts` (from the implementation context,
default 4) caps the full-suite runs this phase may spend, counting the initial run;
`build_max_attempts: 1` means a red initial run halts with no repair cycle. Otherwise:

1. Each red full-suite result becomes exactly one synthetic integration-repair task, run through
   the same worker contract on the ladder `premium -> max -> halt` — one tier above the default
   deliberately: repair is cross-task diagnosis, never routine work. The repair worker diagnoses
   the failure, adds regression coverage where appropriate, fixes it, re-runs the failing tests
   directly as its focused check, and commits; it never runs the full suite, and its dispatch
   payload carries no run id.
2. When the repair worker returns `COMPLETE`, you start the next counted attempt yourself with the
   same build-owned start — the `gate.drive.start` operation with `--owner build --run-id <run-id>
   --json` (`--run-id` only when your prompt carried a run id) — and drive it to a final result
   exactly as *Gate run posture* describes, so the facade charges it with no bypass.
3. **Green at any point ends the phase immediately; review is never invoked while red.** A red
   result becomes the next repair task while attempts remain. A refused start
   (`suite-attempts-exhausted`) or a red final permitted run halts per *Halting conditions* with
   the exhaustion reason naming `build.max_attempts` and used/limit.

`build_gate: off` runs no suite and spends no attempt, and an infrastructure, result-unavailable,
configuration-gap, or observation-budget halt is unchanged and is **not** a red result to repair.

```

Leave *Gate run posture*, *Keying on the disposition*, *Abandoning a live drive*, and
*Build-findings checkpoint* byte-identical. `change_0410_build_results` pins two of their
single-line phrases.

- [ ] **Step 8: Narrow implement-next's run-context / run-id sentence — `skills/docket-implement-next/SKILL.md`**

Replace this exact text, which sits on one physical line inside the Step 7 paragraph:

```
A gated parent's prompt may also carry a **run context** token; pass it, always as `--run-context`, into every `gate.drive.prepare-scope` / `gate.drive.start` this run performs — each invoked with `--json` per the shared capture requirement — and into the Step-2 claim. It may also carry the **run id**: pass it as `--run-id <run-id>` into every `gate.drive.prepare-scope` and every build-owned `gate.drive.start` (`--owner build` — Step 6's evidence re-mint, the build role's final suite gate, and its repair worker's post-fix re-run) this run performs, omitting the flag only when the prompt carried none (a `run-untracked` or ungated run). Scoped task-owned starts inherit the run id from their scope, so a build-task worker is handed it only for that repair re-run.
```

with

```
A gated parent's prompt may also carry a **run context** token; pass it, always as `--run-context`, into every build-owned `gate.drive.start` this run performs — each invoked with `--json` per the shared capture requirement — and into the Step-2 claim. It may also carry the **run id**: pass it as `--run-id <run-id>` into every build-owned `gate.drive.start` (`--owner build` — Step 6's evidence re-mint and re-gates, the build role's final suite gate, and each post-repair attempt the build role starts) this run performs, omitting the flag only when the prompt carried none (a `run-untracked` or ungated run). Build-task workers receive neither value: they run their tests directly and call no gate operation.
```

- [ ] **Step 9: Rewrite the shared caller contract — `skills/docket-build/references/gate-caller-loop.md`**

Make these replacements in order:

1. Replace the two lines

   ```
   A caller makes **short, slice-bounded,
   synchronous** calls to the native gate **driver**, which composes the raw supervisor,
   ```

   with

   ```
   Its callers are the build controller's full-suite gate (every counted attempt), implement-next's
   evidence re-mint and re-gates, and finalize's local gate. A build-task worker is never a caller: it
   runs its focused tests directly under a fixed time limit. A caller makes **short, slice-bounded,
   synchronous** calls to the native gate **driver**, which composes the raw supervisor,
   ```

   (the three new sentences are prepended to the paragraph; everything after
   `which composes the raw supervisor,` stays unchanged).
2. Replace

   ```
   The high-level surface is the `gate.drive` operation group: `gate.drive.start`,
   `gate.drive.advance`, `gate.drive.handoff`, `gate.drive.claim`, `gate.drive.prepare-scope`,
   `gate.drive.takeover`, and `gate.drive.acknowledge` (resolve each argv from the capability catalog). Each op is
   ```

   with

   ```
   The high-level surface is the `gate.drive` operation group: `gate.drive.start`,
   `gate.drive.advance`, `gate.drive.handoff`, and `gate.drive.claim` (resolve each argv from the
   capability catalog). Each op is
   ```
3. Replace the whole `| \`start\` | Fingerprint the execution context, …` operation row (one long
   line ending `… pending predecessor is refused. |`) with:

   ```
   | `start` | Fingerprint the execution context, launch the first raw run through the supervisor, advance one slice, and return the drive id, owner generation, and disposition. A caller passes `--repo-dir <worktree> --owner build --run-root <dir> --json`, plus `--run-context <token>` and `--run-id <id>` when its prompt carried them; `--owner build` resolves the build-owned suite command from config, so the caller passes no suite argv. |
   ```
4. Delete the three operation rows beginning `| \`prepare-scope\` |`, `| \`takeover\` |`, and
   `| \`acknowledge\` |`.
5. In the JSON-capture table, replace the five rows from `| \`start\` | the drive identifier …`
   through `| \`acknowledge\` | the confirmation document …` with:

   ```
   | `start` | the drive identifier **and** the ownership generation |
   | `handoff` | the **single-use** handoff token |
   | `claim` | the **fresh** owner generation |
   ```
6. Replace

   ```
   disposition only, deliberately omitting generations, tokens, and capabilities. Each token keeps
   its existing meaning — nothing here widens handoff, claim, or takeover authorization, and the
   parent capability from `prepare-scope` stays with the parent as before.
   ```

   with

   ```
   disposition only, deliberately omitting generations and tokens. Each token keeps its existing
   meaning — nothing here widens handoff or claim authorization.
   ```
7. Replace

   ```
   **existing** blocked/halt posture — a build-task worker returns `BLOCKED` with the
   missing-response reason — and never invents credentials
   ```

   with

   ```
   **existing** halt posture — the build controller and implement-next halt with the missing-response
   reason — and never invents credentials
   ```
8. In *Worktree admission*, replace `existing blocked/halt posture (a build-task worker returns \`BLOCKED\`); resolving`
   with `caller's own halt posture (the build controller halts per its *Halting conditions*); resolving`.
9. Delete the whole `## Parent takeover — the event-authorized exception` section (heading, its
   paragraph, and the blank line after it).
10. In the last section, replace

    ```
    them directly and never recreates a shell observe/sleep poll loop — every build task worker, the
    build controller's final gate, implement-next's evidence re-mint and re-gates, and finalize's local
    gate drive the gate through the `gate.drive` operations above instead.
    ```

    with

    ```
    them directly and never recreates a shell observe/sleep poll loop — the build controller's
    full-suite gate, implement-next's evidence re-mint and re-gates, and finalize's local gate drive
    the gate through the `gate.drive` operations above instead.
    ```

Keep `## The disposition vocabulary`, `## Handoff`, the shell-safe capture names paragraph, and
*Worktree admission* otherwise unchanged. `test_gate_caller_loop` and the JSON-capture and
reserved-parameter contract prongs pin them.

- [ ] **Step 10: Repoint and narrow the remaining guards**

1. `internal/repoguard/gatedrive_json_capture_test.go`:
   - Header: replace `//       caller-contract-failure rule (phrase bound to claim, bounded gaps);` with
     ```go
     //       caller-contract-failure rule (phrase bound to claim, bounded gaps) —
     //       change 0488 repointed the failure mapping from the retired worker
     //       BLOCKED to the caller's own halt, and retired the parent-capability
     //       clause with the prepare-scope operation it described;
     ```
   - In the `var` block, replace the `reqBlocked` and `reqParentCap` lines with one line
     ``reqHalt = regexp.MustCompile(`(?i)halt with the[^.]{0,20}missing-response reason`)``. In the
     prong-1 map, replace the `"maps-to-blocked": reqBlocked,` and `"parent-cap-stays": reqParentCap,`
     entries with `"maps-to-halt": reqHalt,`.
   - Floors: replace the comment `// three caller skills each contribute, and the corpus (source + embedded\n\t// mirrors) stays above a global floor.` with
     ```go
     	// two caller skills each contribute, and the corpus (source + embedded
     	// mirrors) stays above a global floor — docket-build's final-gate and
     	// post-repair-attempt starts plus implement-next's two build-owned starts,
     	// doubled by the mirrors. docket-build-task is no longer a caller (change
     	// 0488: workers run tests directly and call no gate operation).
     ```
     Keep `if len(sites) < 8`: the new corpus has exactly 8 sites, measured at plan time. Remove
     `"skills/docket-build-task/SKILL.md",` from the per-file coverage list.
   - In `non_vacuity`, change the `bad` example to
     ``"run the `gate.drive.start` operation with `--owner build --run-root <dir>` and read the drive id"``.
2. `internal/repoguard/gatecapture_reserved_param_test.go`: replace the floor block with
   ```go
   	// Population floors FIRST (a vacuous scan passes every negative): each
   	// caller skill carries the shell-safe names at a gate.drive paragraph,
   	// and the corpus (source + embedded mirrors) stays above a global floor.
   	// Change 0488 lowered the floor 6 -> 4: docket-build-task is no longer a
   	// gate caller (workers run tests directly and call no gate operation).
   	if sites < 4 {
   		t.Fatalf("population floor: only %d gate.drive paragraphs carry the shell-safe capture names (expected >= 4: two caller skills plus embedded mirrors)", sites)
   	}
   ```
   and remove `"skills/docket-build-task/SKILL.md",` from its per-file list.
3. `internal/repoguard/inline_role_stop_test.go`: change both occurrences of
   `"Return exactly one of four outcomes"` to `"Return exactly one of three outcomes"`. The
   `Wrapper preload is not self-invocation` disambiguation is still within six lines of the anchor,
   so leave it as is.
4. Run `gofmt -w internal/repoguard/*.go` and check that `gofmt -l internal/repoguard` prints nothing.

- [ ] **Step 11: Regenerate the embedded mirror**

Run: `go generate ./internal/assets`
Expected: the `genassets: wrote internal/assets/embedded (…)` line. `git status --short internal/assets`
lists `manifest.json` and the four mirrored skill files.

- [ ] **Step 12: Run the focused set (GREEN)**

Run:
`timeout --kill-after=10s 10m go test -count=1 ./internal/repoguard ./internal/assets ./internal/harness/... ./internal/install/...`
Expected: exit 0. This set runs all of `repoguard`, which holds every guard this task touched plus
`TestSkillSizeBudgets` (files only shrank), `TestFeatureDispatchPayloadsCarryCanonicalWorktree`,
`TestCommittedCodexDispatchMatchesGenerator`, and `TestEmbeddedMatchesAuthored`. It is
still a focused package set, not the configured full suite. At plan time the same edits produced
exactly this result in a scratch clone.

If the absence guard still reports a `pairs WAITING with a worker outcome` line, the paragraph it
names puts a drive disposition (`WAITING`) within 45 characters of a worker outcome. Separate them
into different paragraphs or reword; never suppress the guard.

- [ ] **Step 13: Mutation-test every new and repointed guard**

Restore each mutation from its backup. Before trusting a green, confirm the mutation landed by
counting the target in a whitespace-flattened copy (`c=$(tr -s '[:space:]' ' ' < "$f")`).

```bash
# (a) absence guard: re-insert one token of each class into the worker skill → red each time.
f=skills/docket-build-task/SKILL.md
for tok in 'start it with `--owner task`.' 'run the `gate.drive.prepare-scope` operation.' \
           'run `docket gate drive acknowledge`.' 'run the `gate.drive.takeover` operation.' \
           'pass `--predecessor-drive-id <id>`.' 'pass `--child-cap <token>`.' \
           'pass `--scope-id <id>`.' 'Return `COMPLETE` or `WAITING`.'; do
  cp "$f" "$f.bak"; printf '\n%s\n' "$tok" >> "$f"
  timeout --kill-after=10s 10m go test -count=1 ./internal/repoguard -run 'TestNoTaskOwnedDriveInstructions$'; echo "rc=$? for: $tok (want 1)"
  mv -f "$f.bak" "$f"
done
# (b) break the extractor → the non-vacuity companion reddens.
t=internal/repoguard/gatedrive_task_absence_test.go; cp "$t" "$t.bak"
perl -pi -e 's/paras := paragraphs\(readMaintained\(t, root, rel\)\)/paras := paragraphs("")/' "$t"
grep -c -F -- 'paras := paragraphs("")' "$t"   # must print 1
timeout --kill-after=10s 10m go test -count=1 ./internal/repoguard -run 'TestNoTaskOwnedDriveInstructions$'   # want rc=1 with both "non-vacuity:" lines
mv -f "$t.bak" "$t"
# (c) each time-limit sentinel: delete its clause from the real file → red.
f=skills/docket-build/SKILL.md; cp "$f" "$f.bak"
perl -0pi -e 's/The audit is never a halting\s+condition/The audit is a halting condition/' "$f"
c=$(tr -s '[:space:]' ' ' < "$f"); grep -c -F -- 'The audit is a halting condition' <<<"$c"   # must print 1
timeout --kill-after=10s 10m go test -count=1 ./internal/repoguard -run 'TestTaskTestTimeLimitContract$'   # want rc=1 naming controller-audit-never-halts
mv -f "$f.bak" "$f"
f=skills/docket-build-task/SKILL.md; cp "$f" "$f.bak"
perl -0pi -e 's/\| `124` or `137` \|/| `124` |/' "$f"
timeout --kill-after=10s 10m go test -count=1 ./internal/repoguard -run 'TestTaskTestTimeLimitContract$'   # want rc=1 naming worker-limit-hit-is-not-red
mv -f "$f.bak" "$f"
# (d1) run-id prong B: strip BOTH --run-id tokens from the red-path start (the flag and the
#      parenthetical — either alone still satisfies the token match) → red.
f=skills/docket-build/SKILL.md; cp "$f" "$f.bak"
perl -0pi -e 's/--owner build --run-id <run-id>\n   --json` \(`--run-id` only when your prompt carried a run id\)/--owner build\n   --json`/' "$f"
c=$(tr -s '[:space:]' ' ' < "$f"); grep -c -F -- 'with `--owner build --json` — and drive it' <<<"$c"   # must print 1
timeout --kill-after=10s 10m go test -count=1 ./internal/repoguard -run 'TestGateDriveRunIDThreaded$'   # want rc=1 "lacks --run-id"
mv -f "$f.bak" "$f"
# (d2) post-repair-attempt floor: make the red-path start no longer build-owned → floor reddens.
cp "$f" "$f.bak"
perl -0pi -e 's/the `gate.drive.start` operation with `--owner build --run-id <run-id>\n   --json`/the start with `--run-id <run-id>\n   --json`/' "$f"
c=$(tr -s '[:space:]' ' ' < "$f"); grep -c -F -- 'the start with `--run-id <run-id> --json`' <<<"$c"   # must print 1
timeout --kill-after=10s 10m go test -count=1 ./internal/repoguard -run 'TestGateDriveRunIDThreaded$|TestNoTaskOwnedDriveInstructions$'   # want rc=1 "must carry both … found 1"
mv -f "$f.bak" "$f"
# (e) repointed JSON-capture clause: remove the halt mapping → red.
f=skills/docket-build/references/gate-caller-loop.md; cp "$f" "$f.bak"
perl -0pi -e 's/halt with the missing-response\s+reason/stop/' "$f"
timeout --kill-after=10s 10m go test -count=1 ./internal/repoguard -run 'TestGateDriveJSONCapture$'   # want rc=1 "maps-to-halt"
mv -f "$f.bak" "$f"
# Final: everything restored and green.
git diff --stat; timeout --kill-after=10s 10m go test -count=1 ./internal/repoguard
```

Record each mutation's exit status in `VERIFICATION`.

- [ ] **Step 14: Commit**

```bash
git add internal/repoguard/gatedrive_task_absence_test.go internal/repoguard/gatedrive_scope_identity_test.go \
  internal/repoguard/gatedrive_run_id_thread_test.go internal/repoguard/gatedriver_test.go \
  internal/repoguard/prose_contracts_test.go internal/repoguard/gatedrive_json_capture_test.go \
  internal/repoguard/gatecapture_reserved_param_test.go internal/repoguard/inline_role_stop_test.go \
  skills/docket-build-task/SKILL.md skills/docket-build/SKILL.md skills/docket-implement-next/SKILL.md \
  skills/docket-build/references/gate-caller-loop.md \
  internal/assets/embedded/manifest.json \
  internal/assets/embedded/tree/skills/docket-build-task/SKILL.md \
  internal/assets/embedded/tree/skills/docket-build/SKILL.md \
  internal/assets/embedded/tree/skills/docket-implement-next/SKILL.md \
  internal/assets/embedded/tree/skills/docket-build/references/gate-caller-loop.md
git commit -m "fix: build-task workers run tests directly under timeout; controller runs every suite attempt (change 0488)"
```

(`git add` on the old `gatedrive_scope_identity_test.go` path records the rename's deletion side.)

---

### Task 3: Gate-execution acceptance section, test-execution boundary rationale, fix-loop confirmation

**Build tier:** economy

**Files:**
- Modify: `skills/docket-build/references/gate-execution.md` (the 0359 acceptance section)
- Modify: `internal/repoguard/testexec_boundary_test.go` (comments only)
- Verify unchanged: `skills/docket-implement-next/references/fix-loop.md`
- Regenerate: `internal/assets/embedded/manifest.json`, `internal/assets/embedded/tree/skills/docket-build/references/gate-execution.md`

**Interfaces:**
- Consumes: Task 2's `TestNoTaskOwnedDriveInstructions` (scans this reference too).
- Produces: nothing new. Documentation and comments only.

**TDD exception (state it in the return):** RED/GREEN is unsuitable because this task changes
documentation and Go comments only, with no executable behavior. Replacement verification: the
focused repoguard/assets run below, plus the greps. Residual risk: the retitled acceptance section
is prose that only human review reads for meaning.

- [ ] **Step 1: Rewrite the 0359 acceptance section**

In `skills/docket-build/references/gate-execution.md`, replace everything from the heading
`## Change 0359 continuation/takeover acceptance — PENDING HUMAN RE-PROBE (before merge)` up to,
but not including, `## Evidence`, with:

```markdown
## Run-boundary continuation acceptance — outstanding human verification

The four-harness acceptance probes for the run-boundary continuation (change 0359's implement-next
controller returning to its top parent) were carved out of the autonomous build: they require
driving four separately-installed harnesses and **must never be fabricated**. They are recorded
here as outstanding human verification. The measured verdict sections above are point-in-time
records and are untouched by this section. Change 0488 retired the worker-level scenarios
(worker-to-controller handoff and controller takeover): build-task workers no longer drive a gate.

### Pending rows

| Harness | Version | Paths to re-probe | Verdict |
| --- | --- | --- | --- |
| Claude Code | `2.1.251` | interactive AND forked/dispatched implement-next path | `unverified — run-boundary re-probe pending (human)` |
| Cursor | `3.17.21` | registered named-agent + continuation dispatch | `unverified — run-boundary re-probe pending (human)` |
| Codex | `0.150.1` | named dispatch, same-agent resume when available, fresh continuation fallback | `unverified — run-boundary re-probe pending (human)` |
| OpenCode | `1.18.23` | named dispatch + continuation dispatch | `unverified — run-boundary re-probe pending (human)` |

### The probe scenarios

Each harness must be observed against all of them:

1. a fast build-owned suite gate that returns before 30 seconds;
2. an implement-next controller that returns, followed by top-parent continuation of the same
   process and same run key;
3. no duplicate process, no new task, and no retry consumption while the drive is active;
4. terminal pass and terminal failure consumed by the correct resumed role;
5. explicit resume of an already-in-progress change remaining attributable.

### Standing rules

- A verdict is version-scoped: re-probe when the version moves; never inherit a row on faith.
- An interactive-only observation cannot stand in for a dispatched path.
- A harness that cannot supply the direct-child return event or an explicit continuation is
  unsupported on that path, and the gap is reported, never bridged with a timer.

This section is outstanding human verification of the run-boundary continuation: a human runs
these probes and records the evidence here. Never write probe evidence that was not observed.

```

Check: `git grep -n -E -e 'before merge|worker-to-controller handoff, |controller takeover of the same' -- skills/docket-build/references/gate-execution.md`
returns only the retirement sentence.

- [ ] **Step 2: Narrow the test-execution boundary rationale (comments only)**

In `internal/repoguard/testexec_boundary_test.go`:

1. Replace

   ```go
   // Enforces change 0359's migration invariant: after Tasks 8/9/10 every test-intent
   // command on a workflow fixture routes through the native gate DRIVER (`docket gate
   // drive …`); a DIRECT test execution injected into a workflow fixture is a defect.
   ```

   with

   ```go
   // Enforces the full-suite channel boundary (change 0359, narrowed by change 0488):
   // every FULL-SUITE command on a workflow fixture — docket's own suite channel or a
   // resolved test_command — routes through the native gate DRIVER (`docket gate
   // drive …`); a DIRECT full-suite execution injected into a workflow fixture is a
   // defect. Focused task tests are outside this boundary: since change 0488 a
   // build-task worker runs them directly under `timeout --kill-after=10s 10m`
   // (pinned by TestTaskTestTimeLimitContract), and only the full suite is the gate's.
   ```
2. Replace

   ```go
   // SAME line — the sanctioned task-owner recipe carries the suite argv AFTER `-- ` on a
   // `gate drive start` line, so a same-line excuse covers it, while a direct-suite
   ```

   with

   ```go
   // SAME line — a driver recipe that carries the suite argv AFTER `-- ` on a
   // `gate drive start` line is excused by that same line, while a direct-suite
   ```
3. Replace

   ```go
   // a `gate drive` line — the task-owner recipe carries the suite argv after `-- ` on the
   // driver line, so a same-line excuse covers it, while a direct-suite spelling on any
   ```

   with

   ```go
   // a `gate drive` line — a driver recipe carrying the suite argv after `-- ` on the
   // driver line is excused by that line, while a direct-suite spelling on any
   ```

Leave the detector code and the `red_injection` fixtures untouched. Their `--owner task`
fixture strings exercise the scanner's same-line excuse, and that spelling stays a valid catalog
flag until change 0489. Run `gofmt -l internal/repoguard` and expect no output.

- [ ] **Step 3: Confirm fix-loop.md needs no edit**

Run: `git grep -n -i -E -e 'scope|run id|run-id|WAITING|gate\.drive|takeover|acknowledge' -- skills/docket-implement-next/references/fix-loop.md`
Expected: only the two unrelated `scope` words (*scoped to the gate*). Fix workers already run the
`docket-build-task` contract with no scope (*focused test → implement → verify → self-review → one
commit*), and the loop's full-suite gate is the coordinator's. So this file gets no edit; say so in
`NOTES`.

- [ ] **Step 4: Regenerate and run the focused set**

Run `go generate ./internal/assets`, then
`timeout --kill-after=10s 10m go test -count=1 ./internal/repoguard ./internal/assets`
Expected: exit 0.

- [ ] **Step 5: Commit**

```bash
git add skills/docket-build/references/gate-execution.md internal/repoguard/testexec_boundary_test.go \
  internal/assets/embedded/manifest.json \
  internal/assets/embedded/tree/skills/docket-build/references/gate-execution.md
git commit -m "docs: retire the worker-level 0359 probes and narrow the test-exec boundary rationale (change 0488)"
```

---

### Task 4: Lower the skill size ceilings to the new file sizes

**Build tier:** economy

**Files:**
- Modify: `internal/repoguard/budgets_test.go` (`skillBudgets` rows and the change-history comment)

**Interfaces:**
- Consumes: the final sizes Tasks 2 and 3 left behind.
- Produces: ceilings pinned at the exact new counts (a direction-only ratchet).

- [ ] **Step 1: Measure**

```bash
for f in skills/docket-build/SKILL.md skills/docket-build-task/SKILL.md \
         skills/docket-build/references/gate-caller-loop.md skills/docket-build/references/gate-execution.md \
         skills/docket-implement-next/SKILL.md; do printf '%s ' "$f"; wc -l -w < "$f"; done
```

Expected, measured at plan time with the exact text from Tasks 2–3 (pin what you measure; these
are a cross-check): docket-build/SKILL.md 404 lines / 4018 words; docket-build-task/SKILL.md
168 / 1637; gate-caller-loop.md 136 / 1491; gate-execution.md 163 / 1469;
docket-implement-next/SKILL.md 214 / 8162. If any measured value is **above** its current
ceiling, stop and return `BLOCKED` naming the file. This change only shrinks these files, and a
ceiling is never raised here.

- [ ] **Step 2: Write the failing ratchet check first (RED by mutation)**

Set each of the five rows to its measured count **minus one** (lines and words), run
`timeout --kill-after=10s 10m go test -count=1 ./internal/repoguard -run 'TestSkillSizeBudgets$'`,
and confirm exit 1 with ten `over its … budget` lines (two per file). This proves the rows bind
these files.

- [ ] **Step 3: Pin the exact measured counts and rewrite the comments**

Set each row to the exact measured numbers. Prepend a change note to each row's trailing comment,
keeping the older history after it. With the plan-time numbers:

```go
	{"docket-build/SKILL.md", 404, 4018}, // 0488: workers run tests directly; the dispatch payload carries no scope bundle, the task-level WAITING/takeover section is gone, the controller runs every post-repair attempt, +time-limit audit (439/4479 -> 404/4018); 0467 review fix: …(existing history unchanged)
	{"docket-build/references/gate-caller-loop.md", 136, 1491}, // 0488: prepare-scope/takeover/acknowledge rows and the parent-takeover section removed; callers are the full-suite gates (175/1872 -> 136/1491); 0467: …(existing history unchanged)
	{"docket-build/references/gate-execution.md", 163, 1469}, // 0488: 0359 acceptance section narrowed to the run-boundary continuation (170/1520 -> 163/1469)
	{"docket-build-task/SKILL.md", 168, 1637}, // 0488: scoped drive protocol replaced by direct foreground tests under timeout; three outcomes (211/2235 -> 168/1637); 0467 review fix: …(existing history unchanged)
	{"docket-implement-next/SKILL.md", 214, 8162}, // 0488: run context and run id name only build-owned starts (word ceiling 8175 -> 8162); 0467 review fix: …(existing history unchanged)
```

(Each `0467…: …(existing history unchanged)` tail stands for that row's entire current comment
text, kept verbatim after the new `0488:` note; `gate-execution.md`'s row has no comment today. Lowering a ceiling while the plan-time numbers differ from what you
measured means pinning what you measured.) Also append this paragraph to the `skillBudgets` doc comment, directly above
`var skillBudgets = []skillBudget{`:

```go
//
// Change 0488 lowered docket-build/SKILL.md, docket-build-task/SKILL.md,
// gate-caller-loop.md, gate-execution.md, and docket-implement-next/SKILL.md to
// their new sizes: build-task workers run focused tests directly under
// `timeout --kill-after=10s 10m` and call no gate operation, so the 0405/0416/
// 0459/0467 worker-scope contract the notes above re-baselined for is gone.
// Pinned at the exact new counts — the ratchet reddens on any regrowth.
```

Run `gofmt -w internal/repoguard/budgets_test.go`.

- [ ] **Step 4: Run the focused check (GREEN)**

Run: `timeout --kill-after=10s 10m go test -count=1 ./internal/repoguard -run 'TestSkillSizeBudgets$|TestDispatchBlockBudget$'`
Expected: exit 0.

- [ ] **Step 5: Commit**

```bash
git add internal/repoguard/budgets_test.go
git commit -m "test: lower skill size ceilings to the post-0488 sizes"
```

---

## Build gate (controller-owned — not a task)

After Task 4 commits, the `docket-build` controller runs the whole suite once, through the
configured `build.test_command` (`go run ./cmd/docket development test`), as its build-owned gate
drive. It reads the budget report even on green (AGENTS.md *Guards and tests*). No task worker runs
it.

## Coordinator notes (not plan tasks)

- **ADR** (via `docket-adr`, never a plan task): *Build-task workers run focused tests directly
  under a fixed time limit; the gate driver serves only full-suite gates.* `supersedes: [117]`,
  `relates_to: [107, 24]`, with the rejected alternatives, chosen enforcement, and accepted
  losses exactly as the spec's *Prose and decision record* lists them.
- **Results file, *Human actions and testing* (Important):** after merge and the binary reinstall,
  check that the next real implement-next run shows workers making no `gate.drive.*` calls and
  wrapping each test in `timeout --kill-after=10s 10m`. Separately, the post-merge
  `development.install` reconciles this repo's `AGENTS.md` block, which this branch already
  regenerated, so expect no diff there.
- **Results file, accepted residual:** nothing mechanically proves a worker used `timeout`. The
  controller's audit reports honest omissions only. The open case is a non-Go repo on a harness
  with no call limit, with a forgotten wrapper and a hung test.
- **Results file, budget margins:** report each lowered row's margin as a number (it is zero by
  construction).
