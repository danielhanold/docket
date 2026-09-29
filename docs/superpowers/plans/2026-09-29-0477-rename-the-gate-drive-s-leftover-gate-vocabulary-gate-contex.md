<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0477 — Finish the run-tracker rename (--gate-context, DOCKET_AGENT_GUARDIAN_GATE_KEY, dispatch_context)](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0477-rename-the-gate-drive-s-leftover-gate-vocabulary-gate-contex.md)**
<!-- docket:backlink:end -->
# Finish the run-tracker rename Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. In this repo the plan is executed by `docket-build` (one profile worker per task, sequential, one full-suite gate at the end).

**Goal:** Rename the three run-tracker spellings change 0471 kept (ADR-0129 rows 38e-38g) as one hard cut with no aliases, and replace the retired-vocabulary seal's row-12 kept-namesake special case with plain rows.

**Architecture:** Four renames with no persisted-state impact: the gate drive's `--gate-context` flag becomes `--run-context` (the flag `change claim` already takes), the guardian env var `DOCKET_AGENT_GUARDIAN_GATE_KEY` becomes `DOCKET_AGENT_GUARDIAN_RUN_KEY`, the `run.start` result JSON key `dispatch_context` becomes `run_context`, and "outer gate" / "gate context" prose in Go moves to run-tracker words. Consumers (skills, embedded copies, the generated Codex dispatch clause and the committed `AGENTS.md`) move in the same PR. The seal task comes last: once no old spelling survives, row 12 turns into a plain row covering 38e, its `Kept` machinery is deleted, and rows 38f and 38g are added and mutation-tested.

**Tech Stack:** Go (cobra CLI, `go/scanner`), `go generate ./internal/assets/` (`cmd/genassets`), the Go suite runner (`go run ./cmd/docket development test`).

**Spec:** `docs/superpowers/specs/2026-09-29-rename-the-gate-drive-s-leftover-gate-vocabulary-gate-contex-design.md` (on the `docket` metadata branch; synchronized copy at `/Users/homer/dev/docket/.docket/docs/superpowers/specs/2026-09-29-rename-the-gate-drive-s-leftover-gate-vocabulary-gate-contex-design.md`). Decision record: ADR-0129 rows 38e-38h and its 2026-09-29 Update.

## Global Constraints

- Hard cut, no aliases, no hidden flag, no deprecation window, no custom refusal for the old flag (ADR-0129 Decision 2). The old flag must simply be unknown to cobra (exit 2).
- Row 38e: `gate drive start` and `gate drive prepare-scope` flag `--gate-context` → `--run-context`.
- Row 38f: `DOCKET_AGENT_GUARDIAN_GATE_KEY` → `DOCKET_AGENT_GUARDIAN_RUN_KEY`.
- Row 38g: `run.start` result JSON key `dispatch_context` → `run_context`.
- Row 38h (`rungate store` → `run-tracker store`) was already done by 0471 and is sealed by row 38's `rungate` token: record only, no new row.
- Must NOT change: the committed claim-receipt key `gate_context_hash` (`internal/app/change_claim.go`, `claim_proof.go`) and the claim idempotency digest; the v2 gate-drive/scope store key `run_context_hash`; the gate drive's own name, the `docket gate …` CLI noun, and every checkpoint sense of "gate" (ADR-0129 Decision 5).
- Never edit point-in-time records: `docs/changes/**`, `docs/results/**`, `docs/superpowers/**` (other than this plan), `docs/adrs/**`. ADR-0129 is delivered by the change's `adrs: [129]`, not by this build.
- After editing anything under `skills/`, run `go generate ./internal/assets/` and commit `internal/assets/embedded/**` in the same commit (`TestEmbeddedMatchesAuthored`).
- `AGENTS.md` (`CLAUDE.md` is a symlink to it; never edit `CLAUDE.md`) and `internal/harness/dispatch.go` `CodexRootEntryClause` must stay byte-identical in the changed sentence; commit both together (`TestCommittedCodexDispatchMatchesGenerator`).
- Find sites with a whole-repo grep, never a hand list. The site lists in this plan were traced at base `32fd8adea` and are starting points; the grep is authoritative.
- Stage explicit paths only (`git add <path> …`), never `git add -A` / `git add .`.
- Go test commands always pass `-count=1` (the test cache can serve a stale green against a mutated tree).
- Mutation tests restore from a backup copy (`cp f f.bak; mutate; test; mv -f f.bak f`), never `git checkout -- f` (that restores HEAD and destroys the uncommitted edit under test). Confirm each mutation landed (`grep -c`) before reading the test result.
- `internal/app/*_integration_test.go` files carry `//go:build integration`: run them with `go test -tags integration …`.
- The build gate runs the whole suite through `build.test_command` (`go run ./cmd/docket development test`), not just the tests named here; read its budget report.

## Review Focus

1. **Registration renamed but a read left stale.** `c.Flags().GetString("gate-context")` on an unregistered flag returns `""` with an ignored error, so the run context silently disappears and the scope pins nothing (a scope that pinned no context accepts any start). Expected: the value passed as `--run-context` reaches both `prepare-scope` and `start`. Pinned by the CLI wiring test in Task 1 (`TestGateDriveRunContextWiredThroughScope`), including its mutation step.
2. **An in-flight caller still passing `--gate-context` to the gate drive.** Expected: a loud unknown-flag failure (exit 2), never a silently ignored flag. Pinned in Task 1 (`TestRunTrackerVocabularyHardCut` additions).
3. **The committed claim receipt key `gate_context_hash`.** Expected: still legal and unchanged; the seal's row 38c token boundary must keep excluding it. Pinned by the negative controls kept and extended in Task 5.
4. **The old flag reappearing in generator output** (the Codex dispatch clause that consumer repos receive). Expected: the seal's `generator_output` and Go-literal scans both refuse it now that the kept-namesake exception is gone. Pinned by Task 5's non-vacuity lines and the `dispatch.go` mutation step.
5. **A pinned skill instruction losing the run-context pass-through while being reworded.** Expected: every scoped task-start instruction still carries the full identity bundle (now with `--run-context`). Pinned by `TestGateDriveScopedStartIdentity`'s flipped required-flag lists and its per-flag removal mutation loop in Task 2.

---

### Task 1: Rename the gate drive's `--gate-context` flag to `--run-context` (row 38e, CLI side)

Profile hint: standard.

**Files:**
- Modify: `internal/cli/gate.go` (the `start` command's `GetString("gate-context")` read and `start.Flags().String("gate-context", …)` registration; the `prepareScope` command's read and registration)
- Modify: `internal/app/runtracker_run_id_refusal.go` (`RunIDNextAction`, the `ReasonUnknownRunID` message)
- Test: `internal/cli/run_tracker_rename_test.go` (`TestRunTrackerVocabularyHardCut`)
- Test: `internal/cli/gate_test.go` (`TestGateDriveStartUnknownRunIDIsNamed`, `TestGateDrivePrepareScopeUnknownRunIDIsNamed`, new `TestGateDriveRunContextWiredThroughScope`)
- Test: `internal/app/gate_drive_test.go` (`TestMapDriveFailureRunErrors`, `TestPrepareScopeRefusesUnknownRunID`)
- Test: `internal/app/runtracker_run_id_refusal_test.go` (`TestRunIDNextAction`)
- Test: `internal/cli/capability_production_test.go` (`TestRepresentativeSignatures` golden signatures for `gate.drive.start` and `gate.drive.prepare-scope`)

**Interfaces:**
- Consumes: nothing from other tasks.
- Produces: the CLI flag name `run-context` on `docket gate drive start` and `docket gate drive prepare-scope` (catalog signatures `[--run-context <token>]`). Task 2's skill text and Task 5's seal rely on this name. Go field names are unchanged (`GateDriveStartRequest.RunContext`, `gatedrive.ScopeRequest.RunContext`).

- [ ] **Step 1: Flip the tests that pin the kept name, and add the wiring test**

In `internal/cli/run_tracker_rename_test.go`, extend the `(path, want, gone)` table and delete the "must keep" loop. The table becomes:

```go
	for _, f := range []struct {
		path       []string
		want, gone string
	}{
		{[]string{"run", "cancel"}, "run-id", "epoch"},
		{[]string{"change", "claim"}, "run-context", "gate-context"},
		{[]string{"agent", "enter"}, "run-key", "run-gate-key"},
		{[]string{"agent", "enter"}, "run-id", "run-epoch"},
		{[]string{"gate", "drive", "start"}, "run-id", "run-epoch"},
		{[]string{"gate", "drive", "prepare-scope"}, "run-id", "run-epoch"},
		// change 0477, ADR-0129 row 38e: the gate drive takes the same
		// --run-context as change claim; --gate-context is gone everywhere.
		{[]string{"gate", "drive", "start"}, "run-context", "gate-context"},
		{[]string{"gate", "drive", "prepare-scope"}, "run-context", "gate-context"},
	} {
```

Delete this block entirely:

```go
	// The gate drive's own --gate-context is not an ADR-0129 row and stays.
	for _, p := range [][]string{{"gate", "drive", "start"}, {"gate", "drive", "prepare-scope"}} {
		if find(p...).Flags().Lookup("gate-context") == nil {
			t.Errorf("%v must keep its --gate-context flag", p)
		}
	}
```

and add, after the existing `run cancel --epoch` exit-2 check at the end of the function:

```go
	// change 0477: an in-flight caller still passing the gate drive's old flag
	// fails loudly as an unknown flag (exit 2), never silently ignored.
	if _, _, code := runCLI(t, "--json", "gate", "drive", "prepare-scope",
		"--change-id", "1", "--task-id", "t", "--phase", "build", "--branch", "b",
		"--worktree", "/tmp", "--gate-context", "x"); code != 2 {
		t.Errorf("docket gate drive prepare-scope --gate-context exited %d, want 2 (unknown flag)", code)
	}
```

Update the function's doc comment to say it also pins change 0477's row 38e.

In `internal/cli/gate_test.go`:
- `TestGateDriveStartUnknownRunIDIsNamed`: change the message assert to
  ```go
  	if msg, _ := doc["message"].(string); !strings.Contains(msg, "--run-context") || strings.Contains(msg, "--gate-context") {
  		t.Fatalf("refusal must carry the next action naming --run-context, got %q", msg)
  	}
  ```
- `TestGateDrivePrepareScopeUnknownRunIDIsNamed`: change the human assert to
  ```go
  	if !strings.Contains(human, "unknown-run-id") || !strings.Contains(human, "--run-context") ||
  		strings.Contains(human, "--gate-context") || strings.Contains(human, bogus) {
  		t.Fatalf("human output must name reason + remedy and never the value, got %q", human)
  	}
  ```
- Add this new test after `TestGateDriveScopeBoundStartRoundTrips` (it reuses `gateDriveConfiguredRepo`, `testsupport.TempDir`, `runCLI`, `decodeOneJSON`, `driveDoc`, all already in the package):

```go
// TestGateDriveRunContextWiredThroughScope (change 0477, ADR-0129 row 38e): the
// value passed as --run-context reaches BOTH prepare-scope (which pins its hash)
// and start (which must present the same token). A read left on the retired
// "gate-context" name returns "" silently: a stale prepare-scope read pins no
// context and the mismatched start below would bind; a stale start read presents
// "" and the matched start below would be refused.
func TestGateDriveRunContextWiredThroughScope(t *testing.T) {
	wt := gateDriveConfiguredRepo(t, "metadata_branch: main\n")
	root := testsupport.TempDir(t)
	prepare := func(task string) (scopeID, childCap string) {
		t.Helper()
		out, errS, code := runCLI(t, "--json", "gate", "drive", "prepare-scope",
			"--repo-dir", wt, "--change-id", "477", "--task-id", task,
			"--phase", "build", "--branch", "fix/x", "--worktree", wt,
			"--run-context", "ctx-A")
		if code != 0 || errS != "" {
			t.Fatalf("prepare-scope %s: out=%q err=%q code=%d", task, out, errS, code)
		}
		g := decodeOneJSON(t, out)
		scopeID, _ = g["scope_id"].(string)
		childCap, _ = g["child_capability"].(string)
		if scopeID == "" || childCap == "" {
			t.Fatalf("prepare-scope %s missing a grant field: %v", task, g)
		}
		return scopeID, childCap
	}
	start := func(task, scopeID, childCap, runContext string) map[string]any {
		t.Helper()
		out, errS, _ := runCLI(t, "--json", "gate", "drive", "start",
			"--repo-dir", wt, "--run-root", root, "--owner", "task",
			"--scope-id", scopeID, "--child-cap", childCap, "--run-context", runContext,
			"--change-id", "477", "--task-id", task, "--phase", "build",
			"--branch", "fix/x", "--", "/bin/echo", "hi")
		if errS != "" {
			t.Fatalf("start %s wrote stderr: out=%q err=%q", task, out, errS)
		}
		return decodeOneJSON(t, out)
	}

	sA, cA := prepare("task-matched")
	doc := start("task-matched", sA, cA, "ctx-A")
	if doc["result"] != "applied" {
		t.Fatalf("a start presenting the scope's own --run-context must bind: %v", doc)
	}
	if id, _ := driveDoc(t, doc)["drive_id"].(string); id == "" {
		t.Fatalf("matched start bound no drive id: %v", doc)
	}

	sB, cB := prepare("task-mismatched")
	doc = start("task-mismatched", sB, cB, "ctx-B")
	if doc["result"] == "applied" {
		t.Fatalf("a start presenting a different --run-context than the scope pinned must be refused: %v", doc)
	}
	if _, ok := doc["drive"]; ok {
		t.Fatalf("a refused start must carry no drive document: %v", doc)
	}
}
```

In `internal/app/gate_drive_test.go`, in both `TestMapDriveFailureRunErrors` and `TestPrepareScopeRefusesUnknownRunID`, replace `"--gate-context"` with `"--run-context"` in the `strings.Contains(got.Message, …)` assert and add `|| strings.Contains(got.Message, "--gate-context")` to the same failure condition.

In `internal/app/runtracker_run_id_refusal_test.go` `TestRunIDNextAction`, change the wanted list to `[]string{"--run-id", "--run-context", "run-started <key> <run-id> <run-context>"}` and add:

```go
	if strings.Contains(unknown, "--gate-context") {
		t.Errorf("unknown-run-id message names the retired --gate-context: %q", unknown)
	}
```

In `internal/cli/capability_production_test.go` `TestRepresentativeSignatures`, the signatures are sorted flag lists, so `--run-context` sorts after `--repo-dir` and before `--run-id`:

```go
		"gate.drive.start": "--owner <role> --run-root <dir> [--branch <name>] [--change-id <id>] [--child-cap <token>] [--cwd <dir>] [--env-hash <hash>] [--idempotent-suite-gate] [--phase <name>] [--predecessor-drive-id <id>] [--predecessor-owner-gen <gen>] [--ref <ref>] [--repo-dir <dir>] [--run-context <token>] [--run-id <id>] [--scope-id <id>] [--task-id <id>] -- <argv...>",
```
```go
		"gate.drive.prepare-scope": "--branch <name> --change-id <id> --phase <name> --task-id <id> --worktree <dir> [--repo-dir <dir>] [--run-context <token>] [--run-id <id>]",
```

(If the generator orders them differently, trust the generator's actual order in the failure diff. It must be the same list with `[--gate-context <token>]` replaced by `[--run-context <token>]`.)

- [ ] **Step 2: Run the flipped tests to verify they fail**

Run:
```bash
cd <feature-worktree>
go test -count=1 ./internal/cli/ -run 'TestRunTrackerVocabularyHardCut|TestGateDriveStartUnknownRunIDIsNamed|TestGateDrivePrepareScopeUnknownRunIDIsNamed|TestGateDriveRunContextWiredThroughScope|TestRepresentativeSignatures'
go test -count=1 ./internal/app/ -run 'TestMapDriveFailureRunErrors|TestPrepareScopeRefusesUnknownRunID|TestRunIDNextAction'
```
Expected: FAIL. `gate drive start/prepare-scope lacks --run-context`, `still registers the retired --gate-context`, `prepare-scope --gate-context exited 0` (or another non-2 code), unknown flag `--run-context` in the wiring test, signature mismatches, and messages lacking `--run-context`.

- [ ] **Step 3: Rename the flag registration, both reads, the help text and the refusal hint**

In `internal/cli/gate.go`, in the `start` command:
```go
			runContext, _ := c.Flags().GetString("run-context")
```
```go
	start.Flags().String("run-context", "", "run-context `token` from run start, linking this drive to its started run (optional; omitted for an untracked run)")
```
In the `prepareScope` command:
```go
			runContext, _ := c.Flags().GetString("run-context")
```
```go
	prepareScope.Flags().String("run-context", "", "run-context `token` from run start, linking this scope's nested drives to its started run (optional; omitted for an untracked run)")
```
Keep the backticked `` `token` ``, because it yields the `<token>` placeholder in the catalog signature.

In `internal/app/runtracker_run_id_refusal.go` `RunIDNextAction`, change the `ReasonUnknownRunID` message's parenthetical from `(the <run-context> goes to the gate drive's --gate-context)` to `(the <run-context> goes to --run-context)`, leaving the rest of the string unchanged:

```go
	case ReasonUnknownRunID:
		return "the --run-id value names no run in this repository; pass the <run-id> field of the start's " +
			"`run-started <key> <run-id> <run-context>` line (the <run-context> goes to --run-context) — " +
			"never drop --run-id and retry"
```

Then confirm that no read or registration was missed:
```bash
command git grep -n -e '"gate-context"' -e 'gate-context' -- 'internal/cli/*.go' 'internal/app/*.go' ':!*_test.go'
```
Expected: no output.

- [ ] **Step 4: Run the tests to verify they pass**

Run the two commands from Step 2. Expected: PASS.
Then run `go build ./... && go vet ./internal/cli/ ./internal/app/` (expected: clean) and the seal, which must still be green because its row-12 `Kept` hooks tolerate the fewer remaining sites:
```bash
go test -count=1 ./internal/repoguard/ -run 'TestRetiredVocabularySeal'
```
Expected: PASS.

- [ ] **Step 5: Mutation-test the wiring test (Review Focus 1)**

```bash
f=internal/cli/gate.go; cp "$f" "$f.bak"
# stale prepare-scope read: change ONLY the prepareScope command's GetString("run-context") back to GetString("gate-context")
# (edit that one line by hand, then confirm exactly one stale read landed:)
grep -c 'GetString("gate-context")' "$f"   # expect 1
go test -count=1 ./internal/cli/ -run TestGateDriveRunContextWiredThroughScope   # expect FAIL (mismatched start bound)
mv -f "$f.bak" "$f"
cp "$f" "$f.bak"
# stale start read: change ONLY the start command's GetString("run-context") back to GetString("gate-context")
grep -c 'GetString("gate-context")' "$f"   # expect 1
go test -count=1 ./internal/cli/ -run TestGateDriveRunContextWiredThroughScope   # expect FAIL (matched start refused)
mv -f "$f.bak" "$f"
go test -count=1 ./internal/cli/ -run TestGateDriveRunContextWiredThroughScope   # expect PASS again
```
If either mutation stays green, the test is vacuous: fix the test before continuing.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/gate.go internal/app/runtracker_run_id_refusal.go \
  internal/cli/run_tracker_rename_test.go internal/cli/gate_test.go \
  internal/app/gate_drive_test.go internal/app/runtracker_run_id_refusal_test.go \
  internal/cli/capability_production_test.go
git commit -m "refactor(gate-drive)!: rename --gate-context to --run-context (change 0477, ADR-0129 row 38e)"
```

---

### Task 2: Move the skills, embedded copies, dispatch clause and AGENTS.md to `--run-context`

Profile hint: standard.

**Files:**
- Modify: `skills/docket-build-task/SKILL.md` (scoped task-start instruction)
- Modify: `skills/docket-build/SKILL.md` (feature-dispatch prepare-scope instruction)
- Modify: `skills/docket-build/references/gate-caller-loop.md` (`start` and `prepare-scope` operation rows)
- Modify: `skills/docket-implement-next/SKILL.md` ("Verify the run" paragraph)
- Regenerate: `internal/assets/embedded/**` (`go generate ./internal/assets/`)
- Modify: `internal/harness/dispatch.go` (`CodexRootEntryClause`)
- Modify: `AGENTS.md` (the same sentence inside the managed `docket:dispatch` block)
- Test: `internal/repoguard/gatedrive_scope_identity_test.go` (`requiredScopedStartFlags`, `requiredStartRowFlags`, header comments, `non_vacuity` string)
- Test: `internal/repoguard/gatedrive_run_id_thread_test.go` (`TestGateDriveRunIDThreaded` `non_vacuity` `prep` string; `TestCodexRequestFileCarriesRunID` `good`/`old` strings)

**Interfaces:**
- Consumes: Task 1's CLI flag name `run-context` on `gate drive start` / `gate drive prepare-scope`.
- Produces: skill text and generated dispatch text with no `--gate-context` left. Task 5's seal scans these surfaces and must find none.

- [ ] **Step 1: Flip the pin tests to the new flag**

In `internal/repoguard/gatedrive_scope_identity_test.go`:
- In `requiredScopedStartFlags` and `requiredStartRowFlags`, replace `"--gate-context"` with `"--run-context"`.
- In the file header comment (prong A) and the `requiredScopedStartFlags` doc comment, change "the --gate-context pass-through" / "the gate-context pass-through" to "the --run-context pass-through" / "the run-context pass-through".
- In the `non_vacuity` subtest, change `--gate-context <g>` in `full` to `--run-context <g>`.

In `internal/repoguard/gatedrive_run_id_thread_test.go`:
- In `TestGateDriveRunIDThreaded`'s `non_vacuity`, change `prep` to use `--run-context <g>` in place of `--gate-context <g>`.
- In `TestCodexRequestFileCarriesRunID`'s `non_vacuity`, change the labeled-for clause in both `good` and `old` to the new wording:
  ```go
  		good := "Write a request file containing the request unchanged; for implement-next include the unchanged run-context token, labeled for `--run-context` on `change.claim` and the gate drive, and the unchanged run id, labeled for `--run-id`."
  ```
  ```go
  		old := "Write a request file containing the request unchanged; for implement-next include the unchanged run-context token, labeled for `--run-context` on `change.claim` and the gate drive. Preserve ids. Pass the run id to `--run-id`."
  ```

- [ ] **Step 2: Run the pin tests to verify they fail**

```bash
go test -count=1 ./internal/repoguard/ -run 'TestGateDriveScopedStartIdentity|TestGateDriveRunIDThreaded|TestCodexRequestFileCarriesRunID'
```
Expected: FAIL in `TestGateDriveScopedStartIdentity`. The scoped task-start site in `skills/docket-build-task/SKILL.md` (and its embedded mirror) is missing `--run-context`, and the `gate-caller-loop.md` start row does not document `--run-context`. (The two `non_vacuity` edits in the other file may already pass. That is fine, because their job is to keep the fixtures in the new wording.)

- [ ] **Step 3: Rewrite the four skill sites**

Find every site first. The grep is authoritative:
```bash
command git grep -n -e '--gate-context' -e 'gate-context' -- skills agents cursor-rules .docket.example.yml
```
At base it lists exactly these five lines (all in the four files above):

1. `skills/docket-build-task/SKILL.md`: in the start instruction, replace `--child-cap <token> --gate-context <token> --run-root` with `--child-cap <token> --run-context <token> --run-root`, and replace `omitting gate-drive `` `--gate-context` `` only when no run` with ``omitting `--run-context` only when no run``. Keep the hard wrap near the surrounding line width, and change nothing else in the paragraph.
2. `skills/docket-build/SKILL.md`: in the prepare-scope instruction inside the `docket:feature-dispatch` block, replace `<worktree> --gate-context <run-context> --run-id <run-id> --json` with `<worktree> --run-context <run-context> --run-id <run-id> --json`.
3. `skills/docket-build/references/gate-caller-loop.md`, `start` row: replace ``plus gate-drive `--gate-context <token>` if dispatched with one`` with ``plus `--run-context <token>` if dispatched with one``.
4. Same file, `prepare-scope` row: replace `[--gate-context <token>]` with `[--run-context <token>]`.
5. `skills/docket-implement-next/SKILL.md`, "Verify the run" paragraph: replace
   ``pass it into every `gate.drive.prepare-scope` / `gate.drive.start --gate-context` this run performs — each invoked with `--json` per the shared capture requirement — and into the Step-2 claim's --run-context.``
   with
   ``pass it, always as `--run-context`, into every `gate.drive.prepare-scope` / `gate.drive.start` this run performs — each invoked with `--json` per the shared capture requirement — and into the Step-2 claim.``

Then regenerate the embedded bundle:
```bash
go generate ./internal/assets/
command git grep -n -e 'gate-context' -- skills internal/assets/embedded   # expect no output
```

- [ ] **Step 4: Rewrite the Codex dispatch sentence in the generator and in AGENTS.md, identically**

In `internal/harness/dispatch.go` `CodexRootEntryClause`, replace
``labeled for `change.claim --run-context` and gate-drive `--gate-context`, and the unchanged run id``
with
``labeled for `--run-context` on `change.claim` and the gate drive, and the unchanged run id``.

Make the byte-identical replacement in `AGENTS.md` (the same sentence inside the `docket:dispatch` managed block; it is the only occurrence of `--gate-context` in the file). Before editing, confirm the block's markers are present, balanced and in order:
```bash
grep -n 'docket:dispatch' AGENTS.md   # expect exactly one start marker followed by one end marker
```
Edit only that sentence. Do not touch `CLAUDE.md` (a symlink to `AGENTS.md`).

- [ ] **Step 5: Run the pin, drift and generator tests to verify they pass**

```bash
go test -count=1 ./internal/repoguard/ -run 'TestGateDriveScopedStartIdentity|TestGateDriveRunIDThreaded|TestCodexRequestFileCarriesRunID|TestCommittedCodexDispatch|TestRunTrackerCopiesRunIDIntoDispatchPrompt|TestRetiredVocabularySeal'
go test -count=1 ./internal/repoguard/ -run 'FeatureWorktreeDispatch|Dispatch'
go test -count=1 ./internal/assets/
go test -count=1 ./internal/harness/...
```
Expected: PASS. If a harness golden test fails, run it once with `-update` (for example `go test -count=1 ./internal/harness/codex/ -update`), read the golden diff, and keep it only if the sole change is the reworded sentence or an asset digest. Then re-run without `-update`.

- [ ] **Step 6: Commit**

```bash
git add skills/docket-build-task/SKILL.md skills/docket-build/SKILL.md \
  skills/docket-build/references/gate-caller-loop.md skills/docket-implement-next/SKILL.md \
  internal/assets/embedded internal/harness/dispatch.go AGENTS.md \
  internal/repoguard/gatedrive_scope_identity_test.go internal/repoguard/gatedrive_run_id_thread_test.go
# plus any golden file Step 5 legitimately updated, by explicit path
git commit -m "docs(skills): pass the run context as --run-context to the gate drive (change 0477, ADR-0129 row 38e)"
```

---

### Task 3: Rename the guardian env var and the `run.start` JSON key (rows 38f, 38g)

Profile hint: economy.

**Files:**
- Create: `internal/app/runtracker_start_result_json_test.go` (no build tag, so it runs in the fast unit set)
- Modify: `internal/app/runtracker_start.go` (`RunStartResult.RunContext` struct tag and its doc comment)
- Modify: `internal/app/agent_guardian.go` (`guardianRunKeyEnv` value)
- Test: `internal/app/runtracker_start_resume_integration_test.go` (`TestIntegrationRunStartNoRunRecordResumeMintsBoundRun` JSON assert)

**Interfaces:**
- Consumes: nothing from other tasks.
- Produces: the `run.start` JSON key `run_context` (the schema descriptor in `internal/app/schema_registry.go` reflects the tag, so it needs no edit) and the env var `DOCKET_AGENT_GUARDIAN_RUN_KEY`. Task 5's rows 38f and 38g seal the old spellings.

- [ ] **Step 1: Write the failing test**

Create `internal/app/runtracker_start_result_json_test.go`:

```go
package app

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestRunStartResultRunContextKey (change 0477, ADR-0129 row 38g): the run.start
// result names the run-context token under the key run_context, the same word
// the run-started text line and every flag use. The retired dispatch_context key
// is gone, with no alias.
func TestRunStartResultRunContextKey(t *testing.T) {
	buf, err := json.Marshal(RunStartResult{Started: true, Key: "k", RunContext: "ctx-token"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(buf), `"run_context":"ctx-token"`) {
		t.Errorf("run.start JSON lacks \"run_context\": %s", buf)
	}
	if strings.Contains(string(buf), "dispatch_context") {
		t.Errorf("run.start JSON still carries the retired dispatch_context key: %s", buf)
	}
}
```

In `internal/app/runtracker_start_resume_integration_test.go`, change the wanted key in `TestIntegrationRunStartNoRunRecordResumeMintsBoundRun` from `` `"dispatch_context":"` + scopeGrantChild + `"` `` to `` `"run_context":"` + scopeGrantChild + `"` ``.

- [ ] **Step 2: Run the tests to verify they fail**

```bash
go test -count=1 ./internal/app/ -run TestRunStartResultRunContextKey
go test -count=1 -tags integration ./internal/app/ -run TestIntegrationRunStartNoRunRecordResumeMintsBoundRun
```
Expected: FAIL. The JSON lacks `"run_context"` and carries `dispatch_context`.

- [ ] **Step 3: Change the tag and the env value**

In `internal/app/runtracker_start.go`:
```go
	RunContext string `json:"run_context,omitempty"`
```
Also rewrite the field's doc comment so it no longer says "outer scope" without context, for example: "RunContext is the run-context token (the run tracker's recovery scope's ChildCapability) the parent copies into the implement-next dispatch prompt; …". Keep the rest of the comment's claims.

In `internal/app/agent_guardian.go`:
```go
	guardianRunKeyEnv  = "DOCKET_AGENT_GUARDIAN_RUN_KEY"
```

Confirm nothing else names the old spellings in maintained code:
```bash
command git grep -n -e 'dispatch_context' -e 'DOCKET_AGENT_GUARDIAN_GATE_KEY' -- . ':!docs'
```
Expected: no output. (The Task 5 seal table adds them as retired rows later.)

- [ ] **Step 4: Run the tests to verify they pass**

```bash
go test -count=1 ./internal/app/ -run 'TestRunStartResultRunContextKey|Schema'
go test -count=1 -tags integration ./internal/app/ -run 'TestIntegrationRunStartNoRunRecordResumeMintsBoundRun|TestIntegrationGateLifecycleGuardian'
go build ./...
```
Expected: PASS. The guardian integration tests exercise the spawn→read path end to end through the renamed variable.

- [ ] **Step 5: Commit**

```bash
git add internal/app/runtracker_start_result_json_test.go internal/app/runtracker_start.go \
  internal/app/agent_guardian.go internal/app/runtracker_start_resume_integration_test.go
git commit -m "refactor(run-tracker)!: rename dispatch_context key and guardian GATE_KEY env (change 0477, ADR-0129 rows 38f, 38g)"
```

---

### Task 4: Move "outer gate" / "gate context" prose in Go to run-tracker words

Profile hint: economy. This is comment, test-name and failure-message text only, with no behavior change. It has no RED step: the deliverable is the empty residual grep in Step 3 plus unchanged green tests.

**Files (starting points traced at base; the grep in Step 1 is authoritative):**
- Modify: `internal/app/gate_drive.go` (`GateDriveStartRequest` scope-binding comment; `reserveBuildSuiteAttempt` doc comment)
- Modify: `internal/app/runtracker_start.go` (step "(5) Prepare the OUTER recovery scope" comment)
- Modify: `internal/app/runtracker_verdict_integration_test.go` (run-continue section comment)
- Modify: `internal/gatedrive/drive.go` (`RunContextHash` field comment)
- Modify: `internal/gatedrive/driver.go` (`StartRequest.RunContext` comment; the "Stamp the recovery-scope linkage" comment; `scopedIdentityMatch` doc comment)
- Modify: `internal/gatedrive/run_waiting.go` (`ContinuationHandle` doc comment)
- Modify: `internal/gatedrive/scope.go` (`scopeRecord` doc comment; `ScopeRequest` doc comment)
- Modify: `internal/gatedrive/takeover.go` (`Takeover` doc comment; the outer-scope resolution comment; the close-revalidation comment)
- Modify: `internal/gatedrive/driver_test.go`, `internal/gatedrive/scope_test.go`, `internal/gatedrive/takeover_test.go` (comments, table-case names, failure messages)
- Modify: `internal/app/runtracker_continuation.go` (header comment) only if the grep shows a run-tracker sense there

**Interfaces:**
- Consumes: nothing. Produces: nothing other tasks rely on. No identifier, JSON tag or string literal a test asserts changes meaning.

- [ ] **Step 1: Derive the site list**

```bash
command git grep -n -i -E -e 'outer[ -]gate' -e 'gate[ -]context' -- '*.go' | command grep -v -e 'gate_context_hash' -e 'retired_vocabulary_test.go'
```
Sort every hit into one of two groups:
- **Run-tracker sense** (rewrite it): the "outer gate" is the run tracker (`run.start` / `run.verdict` / `run.continue`, its outer recovery scope, `run-retry-once`), and "gate context" / "gate-context" is the run-context token or its hash.
- **Checkpoint sense** (leave it): the text means an outer *suite* gate (a checkpoint that runs tests). ADR-0129 Decision 5 keeps it.

At base every hit is run-tracker sense. The `retired_vocabulary_test.go` hits are Task 5's.

- [ ] **Step 2: Rewrite each run-tracker-sense site**

Use these words: "the run tracker" for the component, "the dispatched run" / "its started run" for the run, "run context" / "run-context token" / "run-context hash" for the token and its hash. Keep "outer recovery scope" / "outer scope", which is a scope term, not a gate term. Replacements at base:

| Site | Old wording | New wording |
|---|---|---|
| `internal/app/gate_drive.go` scope-binding comment | `linking a nested drive to the outer gate` | `linking a nested drive to the dispatched run` |
| `internal/app/gate_drive.go` `reserveBuildSuiteAttempt` doc | `reset by an outer-gate run-retry-once re-dispatch of the same change: an outer retry inherits` | `reset by a run-tracker run-retry-once re-dispatch of the same change: such a retry inherits` |
| `internal/app/runtracker_start.go` step (5) | `drive to this outer gate` | `drive to this run` |
| `internal/app/runtracker_verdict_integration_test.go` | `the outer gate emits` | `the run tracker emits` |
| `internal/gatedrive/drive.go` `RunContextHash` | `links every nested drive to the outer gate` | `links every nested drive to the dispatched run` |
| `internal/gatedrive/driver.go` `StartRequest.RunContext` | `drive to the outer gate; it is stored only` | `drive to the dispatched run; it is stored only` |
| `internal/gatedrive/driver.go` stamp comment | `links a nested drive to the outer gate.` | `links a nested drive to the dispatched run.` |
| `internal/gatedrive/driver.go` `scopedIdentityMatch` | `plus the gate-context token when the scope pinned one` / `A scope that pinned no gate context accepts any` | `plus the run-context token when the scope pinned one` / `A scope that pinned no run context accepts any` |
| `internal/gatedrive/run_waiting.go` `ContinuationHandle` | `(the outer gate synthesizes a normal handoff` | `(the run tracker synthesizes a normal handoff` |
| `internal/gatedrive/scope.go` `scopeRecord` | `of the outer gate-context token` | `of the run-context token` |
| `internal/gatedrive/scope.go` `ScopeRequest` | `linking nested drives to the outer gate` | `linking nested drives to the dispatched run` |
| `internal/gatedrive/takeover.go` `Takeover` | `the unique gate-context match` | `the unique run-context match` |
| `internal/gatedrive/takeover.go` resolver comment | `resolves to the UNIQUE gate-context match` | `resolves to the UNIQUE run-context match` |
| `internal/gatedrive/takeover.go` close comment | `resolves nested drives by gate context` | `resolves nested drives by run context` |
| `internal/gatedrive/driver_test.go` | `(pinning a gate context)`; cases `"wrong gate context"`, `"missing gate context"` | `(pinning a run context)`; `"wrong run context"`, `"missing run context"` |
| `internal/gatedrive/scope_test.go` | map label `"gate context hash"`; `persists no gate-context hash` | `"run context hash"`; `persists no run-context hash` |
| `internal/gatedrive/takeover_test.go` | `resolve via gate context`; `the gate-context discriminators`; `must stamp the gate-context hash`; `the outer-gate candidate resolver`; `change + gate-context hash`; `wrong gate context → excluded` | `resolve via run context`; `the run-context discriminators`; `must stamp the run-context hash`; `the outer-scope candidate resolver`; `change + run-context hash`; `wrong run context → excluded` |

Reflow a comment only when a line would otherwise exceed the file's usual width. Run `gofmt -l internal/app internal/gatedrive` and expect no output.

- [ ] **Step 3: Verify residuals and that nothing broke**

```bash
command git grep -n -i -E -e 'outer[ -]gate' -e 'gate[ -]context' -- '*.go' | command grep -v -e 'gate_context_hash' -e 'retired_vocabulary_test.go'
```
Expected: no output, or only lines you classified as checkpoint sense in Step 1 (list any such line in the commit body with its reason).

```bash
go vet ./internal/app/ ./internal/gatedrive/
go test -count=1 ./internal/gatedrive/
go test -count=1 ./internal/app/ -run 'GateDrive|RunTracker|Scope'
go test -count=1 -tags integration ./internal/app/ -run 'TestIntegrationRun'
```
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add <each modified .go path, explicitly>
git commit -m "docs(go): run-tracker words for outer-gate and gate-context prose (change 0477)"
```

---

### Task 5: Retired-vocabulary seal — row 12 plain (covers 38e), delete the kept-namesake machinery, add rows 38f and 38g

Profile hint: premium. The task deletes about 100 lines of guard machinery and must prove by mutation that the replacement still guards.

**Files:**
- Modify/Test: `internal/repoguard/retired_vocabulary_test.go`

**Interfaces:**
- Consumes: Tasks 1-4 leave no `--gate-context`, `gate-context` flag literal, `DOCKET_AGENT_GUARDIAN_GATE_KEY` or `dispatch_context` json tag in any maintained surface. This task's `maintained_surfaces` and `generator_output` subtests are the proof.
- Produces: `retiredToken` without a `Kept` field; rows `{Row: "12, 38e", …}` ×2, `{Row: "38f", …}`, `{Row: "38g", …}`.

- [ ] **Step 1: Write the failing non-vacuity asserts and the new negative controls**

In `testRetiredNonVacuity`, delete the two row-12 blocks that follow the per-row loop (the `mixed := …` check and the `for _, line := range []string{ "pass it into every …", … }` / `unbound := …` checks). Replace them with:

```go
	// Rows 12 and 38e (change 0477): the gate drive's flag is now --run-context,
	// so every --gate-context is retired, including the spellings change 0471 kept
	// as the gate drive's namesake. Each line below was a CLEAN negative control
	// before 0477; each must now hit the plain row.
	for _, line := range []string{
		"docket gate drive prepare-scope --gate-context <ctx>",
		"docket gate drive start --gate-context <ctx> --run-id <id>",
		"include the run-context token, labeled for `change.claim --run-context` and gate-drive `--gate-context`, and the run id",
		"plus gate-drive `--gate-context <token>` if dispatched with one",
		"| `prepare-scope` | `gate.drive.prepare-scope --change-id <id> [--gate-context <token>] [--run-id <id>]`: mint",
		"pass it into `gate.drive.start` and into the Step-2 claim's `--gate-context`",
	} {
		for _, rel := range []string{"skills/x/SKILL.md", "tests/test_x.sh"} {
			if !hasRetiredRow(scanTextContent(rel, line), "12, 38e") {
				t.Errorf("rows 12/38e: a --gate-context in %s was not detected: %q", rel, line)
			}
		}
	}
	for _, c := range []struct{ rel, src string }{
		{"internal/cli/gate.go", "package p\nvar f = \"gate-context\"\n"},
		{"internal/app/x.go", "package p\nvar m = \"(the <run-context> goes to the gate drive's --gate-context)\"\n"},
	} {
		if !hasRetiredRow(goHits(c.rel, c.src), "12, 38e") {
			t.Errorf("rows 12/38e: the former kept Go spelling in %s was not detected: %q", c.rel, c.src)
		}
	}
```

In `testRetiredNegativeControls`:
- From `cleanText`, delete the two `docket gate drive … --gate-context` lines, the "Row 12's scope" comment with its line, and the "other gate-drive bindings" comment with its four lines. Keep `"the claim receipt keeps gate_context_hash"`. Add:
  ```go
  		// Change 0477's new spellings, and the committed receipt key row 38c must
  		// never reach (Decision 3).
  		"docket gate drive start --run-context <ctx> --run-id <id>",
  		"pass it, always as `--run-context`, into every `gate.drive.prepare-scope` / `gate.drive.start`",
  		"DOCKET_AGENT_GUARDIAN_RUN_KEY is set on the guardian",
  		"the run.start result carries run_context and the gate drive stores run_context_hash",
  		"the claim receipt's gate_context_hash is committed state",
  ```
- Delete the whole `cleanMarkdown` block (both entries carry `--gate-context`, which is now retired).
- In `cleanGo`, delete the `internal/cli/gate.go` `"gate-context"` entry and the `internal/app/x.go` gate-drive literal entry. Add:
  ```go
  		{"internal/cli/gate.go", "package p\nvar f = \"run-context\"\n"},
  		{"internal/app/agent_guardian.go", "package p\nconst e = \"DOCKET_AGENT_GUARDIAN_RUN_KEY\"\n"},
  		{"internal/app/runtracker_start.go", "package p\ntype R struct {\n\tC string `json:\"run_context,omitempty\"`\n}\n"},
  ```
  Keep the existing `internal/app/change_claim.go` `gate_context_hash` json-tag control, which is the Review Focus 3 pin.

- [ ] **Step 2: Run the seal to verify it fails**

```bash
go test -count=1 ./internal/repoguard/ -run 'TestRetiredVocabularySeal'
```
Expected: FAIL in `non_vacuity`, with `rows 12/38e: a --gate-context … was not detected` for the lines the old `gateDriveBound` still binds (and every line, since the current row label is `"12"`), plus the two Go spellings.

- [ ] **Step 3: Replace row 12's special case with plain rows, and add rows 38f and 38g**

In `internal/repoguard/retired_vocabulary_test.go`:

1. Delete the `Kept` field (and its comment) from `retiredToken`.
2. Delete the whole `var ( gateDriveHeadRe …; gateDriveQualifierRe …; gateDriveOpSpanRe … )` block and the functions `gateDriveBound`, `gateDriveFlagFile`, `gateDriveCommand`, `argvShaped`, `afterShellSeparator`, `enclosingCodeSpan`, `tokenSpans`, `isIdentByte`, `isParagraphBreak` and `scanKeptRows`. Before deleting each one, confirm that no other file uses it: `command git grep -n -w <name> -- internal/repoguard` must list only this file.
3. Simplify the scanners:
   ```go
   // scanTextLine checks one markdown/shell/generated line against every
   // kindToken row.
   func scanTextLine(rel string, lineNo int, line string) []retiredHit {
   	var hits []retiredHit
   	for _, r := range retiredVocabulary {
   		if r.Kind != kindToken {
   			continue
   		}
   		if tokenRe(r.Old).MatchString(line) {
   			hits = append(hits, retiredHit{rel, lineNo, r, strings.TrimSpace(line)})
   		}
   	}
   	return hits
   }
   ```
   ```go
   // scanText scans content line by line. Shell and config lines (md false)
   // have their `#` comment stripped first.
   func scanText(rel, content string, md bool) []retiredHit {
   	lines := strings.Split(content, "\n")
   	var hits []retiredHit
   	for i, line := range lines {
   		if !md {
   			line = stripHashComment(line)
   		}
   		hits = append(hits, scanTextLine(rel, i+1, line)...)
   	}
   	return hits
   }
   ```
   In `scanGoLiteral`:
   ```go
   		case kindToken:
   			hit = tokenRe(r.Old).MatchString(val)
   		case kindGoFlag:
   			hit = val == r.Old
   ```
4. Rows. Replace the two row-12 entries in place:
   ```go
   	{Row: "12, 38e", Kind: kindToken, Old: "--gate-context", New: "--run-context"},
   	{Row: "12, 38e", Kind: kindGoFlag, Old: "gate-context", New: "run-context"},
   ```
   and append after row 38d:
   ```go
   	// Change 0477 — rows 38e-38h: 38e shares row 12's entries above (the gate
   	// drive's flag now takes the same --run-context as change claim, so no kept
   	// namesake remains); 38h (the `rungate store` error-text prefix) is already
   	// sealed by row 38's rungate token and needs no row of its own.
   	{Row: "38f", Kind: kindToken, Old: "DOCKET_AGENT_GUARDIAN_GATE_KEY", New: "DOCKET_AGENT_GUARDIAN_RUN_KEY"},
   	{Row: "38g", Kind: kindJSONKey, Old: "dispatch_context", New: "run_context"},
   ```
   Also update the `// Family (a) — run tracker (change 0471): …` comment to say 0477 finished rows 38e-38h.
5. Header comments: in the file header, add one paragraph after the SCOPE paragraph:
   ```go
   // Every row retires its spelling everywhere in the scanned surface. Change 0471
   // carried a kept-namesake exception for row 12 (the gate drive's own
   // --gate-context); change 0477 renamed that flag to --run-context (ADR-0129 row
   // 38e) and deleted the exception, since a mechanism with no users cannot be
   // mutation-tested (Decision 10). A later family that needs one brings it back
   // with its own mutation tests.
   ```
   In the `kindToken` doc comment keep the `gate_context never matches the kept gate_context_hash` clause (still true, pinned by the negative controls). Update the `scanTextContent` comment if it still mentions blocks or paragraphs.

- [ ] **Step 4: Run the seal and the package to verify they pass**

```bash
go vet ./internal/repoguard/
go test -count=1 ./internal/repoguard/ -run 'TestRetiredVocabularySeal' -v 2>&1 | command grep -E -e '^(=== RUN|--- (PASS|FAIL)|PASS|FAIL|ok)'
go test -count=1 ./internal/repoguard/
```
Expected: PASS for all five subtests (`table_integrity` floor 60 is met: 62 rows) and for the whole package. If `maintained_surfaces` or `generator_output` reports a hit, an earlier task missed a site. Fix that site, never the seal.

- [ ] **Step 5: Mutation-test every new or changed row (backup-copy idiom)**

For each mutation, put the old spelling back at one executable site, confirm it landed, run the seal, expect a FAIL whose message names the row and its replacement, then restore:

```bash
seal() { go test -count=1 ./internal/repoguard/ -run 'TestRetiredVocabularySeal/(maintained_surfaces|generator_output)' 2>&1; }

# (a) row 12/38e kindToken — a skill argv line
f=skills/docket-build/SKILL.md; cp "$f" "$f.bak"
perl -0pi -e 's/--run-context <run-context>/--gate-context <run-context>/' "$f"
grep -c -- '--gate-context <run-context>' "$f"          # expect 1
seal | grep -F -e 'row 12, 38e' -e 'use --run-context'  # expect a hit on skills/docket-build/SKILL.md
mv -f "$f.bak" "$f"

# (b) row 12/38e kindGoFlag — the flag literal in internal/cli/gate.go
f=internal/cli/gate.go; cp "$f" "$f.bak"
perl -0pi -e 's/start\.Flags\(\)\.String\("run-context"/start.Flags().String("gate-context"/' "$f"
grep -c 'String("gate-context"' "$f"                    # expect 1
seal | grep -F -e 'row 12, 38e' -e 'use run-context'    # expect a hit on internal/cli/gate.go
mv -f "$f.bak" "$f"

# (c) row 38f kindToken — the env-var constant
f=internal/app/agent_guardian.go; cp "$f" "$f.bak"
perl -0pi -e 's/DOCKET_AGENT_GUARDIAN_RUN_KEY/DOCKET_AGENT_GUARDIAN_GATE_KEY/' "$f"
grep -c 'DOCKET_AGENT_GUARDIAN_GATE_KEY' "$f"          # expect 1
seal | grep -F -e 'row 38f' -e 'use DOCKET_AGENT_GUARDIAN_RUN_KEY'
mv -f "$f.bak" "$f"

# (d) row 38g kindJSONKey — the struct tag
f=internal/app/runtracker_start.go; cp "$f" "$f.bak"
perl -0pi -e 's/json:"run_context,omitempty"/json:"dispatch_context,omitempty"/' "$f"
grep -c 'json:"dispatch_context' "$f"                   # expect 1
seal | grep -F -e 'row 38g' -e 'use run_context'
mv -f "$f.bak" "$f"

# (e) generator output — the Codex dispatch clause consumers receive (Review Focus 4)
f=internal/harness/dispatch.go; cp "$f" "$f.bak"
perl -0pi -e 's/labeled for `--run-context` on `change\.claim` and the gate drive/labeled for `change.claim --run-context` and gate-drive `--gate-context`/' "$f"
grep -c 'gate-drive `--gate-context`' "$f"              # expect 1
seal | grep -F -e '[generator-output]' -e 'row 12, 38e' # expect both a generator-output hit and a Go-literal hit
mv -f "$f.bak" "$f"

seal   # expect PASS (ok) after all restores
git status --porcelain   # expect only this task's intended edit(s); no *.bak left behind
```
Each mutation must turn the seal red with a message naming the replacement. A mutation that stays green is a defect: fix the row or the scanner and re-run all five.

- [ ] **Step 6: Residual audit**

```bash
command git grep -n -e 'gate-context' -e 'GUARDIAN_GATE_KEY' -e 'dispatch_context' -- . \
  ':!docs/changes' ':!docs/results' ':!docs/superpowers' ':!docs/adrs' | command grep -v -e 'gate_context_hash' -e 'gate-context-invalid' -e 'gate-context-conflict'
```
Expected: only `_test.go` lines that assert the old spelling is refused or retired (`internal/cli/run_tracker_rename_test.go`, `internal/repoguard/retired_vocabulary_test.go`, and the `!Contains` asserts from Tasks 1 and 3). There must be nothing in `skills/`, `internal/assets/embedded/`, `AGENTS.md`, or non-test Go.

- [ ] **Step 7: Commit**

```bash
git add internal/repoguard/retired_vocabulary_test.go
git commit -m "test(repoguard): row 12 plain for 38e, drop kept-namesake machinery, seal rows 38f/38g (change 0477)"
```

---

## Build-gate and handoff notes (for docket-build / implement-next, not a task)

- The final gate is the whole suite through `build.test_command` (`go run ./cmd/docket development test`). Read its budget report (`BUDGET WATCH:` / `SERIAL CONFIRMED OVER BUDGET:` lines).
- An unrelated `gofmt` failure in `internal/githubcli/comment_integration_test.go` is tracked by change 0478 and is out of scope. If it reds the gate, report it as pre-existing (re-check on the unmodified base) and do not fix it here.
- Results `**Human action:**` (landing procedure from spec §5): (1) merge with no dispatched run in flight, because a run whose loaded skill text passes `--gate-context` fails against the new binary; (2) run the post-merge binary rebuild immediately; (3) restart coordinator sessions so they load the regenerated `AGENTS.md` dispatch block; (4) re-run `docket install` in consumer repos. No storage reset is needed, since nothing persisted changes name.
