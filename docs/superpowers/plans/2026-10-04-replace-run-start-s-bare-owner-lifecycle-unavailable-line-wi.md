<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0501 — Replace run.start's bare owner-lifecycle-unavailable line with a plain stop note](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-04-0501-replace-run-start-s-bare-owner-lifecycle-unavailable-line-wi.md)**
<!-- docket:backlink:end -->
# Replace run.start's bare owner-lifecycle-unavailable line with a plain stop note — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: execute this plan with `docket-build` (task-by-task, one commit per task, a single full-suite gate at the end). Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A started `run.start` prints `note: closing this session may not stop this run. To stop it: docket run cancel --key <key> --reason <why>` as its second line instead of the bare `owner-lifecycle-unavailable` token, and the run-tracker rule tells coordinators not to relay that note.

**Architecture:** One behavior change in `RunStartResult.HumanText` (`internal/app/runtracker_start.go`): the second line, still gated on `OwnerLifecycle != ""`, becomes a stop note rendered by a new unexported helper `runStartStopNote(key)`. The JSON field and the `ReasonOwnerLifecycleUnavailable` constant stay as they are. Then a prose change in `cursor-rules/run-tracker.md`, its embedded mirror (`go generate ./internal/assets`), and this repo's managed `docket:dispatch` block in `AGENTS.md`, which is regenerated through docket's install path. `CLAUDE.md` is a symlink to `AGENTS.md`.

**Tech Stack:** Go (stdlib `testing`), docket's `genassets` generator, `docket install --repo-dir`.

**Spec:** `docs/superpowers/specs/2026-10-04-replace-run-start-s-bare-owner-lifecycle-unavailable-line-wi-design.md` (on the `docket` metadata branch; read it at `/Users/homer/dev/docket/.docket/docs/superpowers/specs/2026-10-04-replace-run-start-s-bare-owner-lifecycle-unavailable-line-wi-design.md`).

## Global Constraints

- The printed second line is exactly: `note: closing this session may not stop this run. To stop it: docket run cancel --key <key> --reason <why>`. `<key>` is the run's own key (`r.Key`), filled in. `<why>` stays a literal placeholder.
- The started report stays exactly two lines. The first line `run-started <key> <run-context>` is unchanged.
- JSON is unchanged. `owner_lifecycle` keeps the value `owner-lifecycle-unavailable`. `ReasonOwnerLifecycleUnavailable` keeps its name and value. The printed note stays gated on that field being set.
- `docket run cancel --key <key> --reason <why>` is the CLI's real spelling (verified: `docket run cancel --help` lists `--key` and `--reason`, both required). Do not invent another spelling.
- Never hand-edit the `AGENTS.md` / `CLAUDE.md` `docket:dispatch` managed block. Regenerate it through the install path in Task 2.
- `docs/reference/glossary.md` names the caveat as a concept and stays as is. Point-in-time records (`docs/results/**`, `docs/superpowers/plans/**` other than this plan, `docs/changes/**`) keep their old wording.
- Out of scope: route awareness in `run.start`; `agent.enter` reporting its guardian; changing or removing the JSON field or the constant; rewording the rule's "there is no automatic Stop button" heading.
- Every `go test` run in this plan uses `-count=1`, so the result cache cannot serve a stale green (learning `cached-runner-serves-a-mutated-tree`).
- Never run `./internal/app` integration tests without `-run`. The unfiltered corpus exceeds go test's default timeout (`tests/README.md`).
- Stage explicit paths only, never `git add -A`.

## Review Focus

1. **The note names a different run's key, or a fixed `<key>`.** A human copies the command and cancels the wrong run, or nothing. Pinned by `TestRunStartStopNoteNamesTheRunsOwnKey` (Task 1), with a mutation that hard-codes `<key>`.
2. **The bare token leaks back into the text report** (for example, the note is appended *after* the token, or a future edit prints both). Pinned by `TestRunStartTextOmitsBareTokenJSONKeepsIt` (Task 1), with the mutation that reverts to the bare token.
3. **The JSON field silently changes or disappears** while the text looks right, which breaks machine readers. Pinned by the same test's JSON half (Task 1).
4. **A note on a result that has no caveat** (an unset `OwnerLifecycle`, or a `run-untracked` report). This would split text from JSON. Pinned by `TestRunStartStopNoteGatedOnOwnerLifecycle` (Task 1).
5. **The report grows past two lines, or the first line changes shape.** Callers parse the first line as three fields. Pinned by the newline count in `TestRunStartStopNoteNamesTheRunsOwnKey` and the existing `TestRunStartResultCarriesNoRunID` first-line assert (Task 1).

---

### Task 1: Print the stop note instead of the bare token

**Files:**
- Modify: `internal/app/runtracker_start.go`: the `ReasonOwnerLifecycleUnavailable` doc comment; the `RunStartResult.OwnerLifecycle` field comment; `HumanText` and its doc comment; add `runStartStopNote`; the step (7) comment at the end of `RunStart` (the comment that starts `// (7) Report the started run with its key`).
- Test (unit, untagged): `internal/app/runtracker_start_result_json_test.go`
- Test (integration, `//go:build integration`): `internal/app/runtracker_start_integration_test.go` (`TestIntegrationRunStartPreparesOuterScope`, `TestIntegrationRunStartFreshStartSurfacesKeyAndContext`), `internal/app/change_integration_test.go` (`TestIntegrationChangeRuntimeRunStartMintsLoadableKey`), `internal/app/runtracker_run_record_integration_test.go` (`TestIntegrationRunRecordNoAdapterReportsLifecycleUnavailable`)
- Check, no edit expected: `internal/app/runtracker_start_resume_integration_test.go` (`TestIntegrationRunStartStartedResultFields`). It compares the **field** `got.OwnerLifecycle != ReasonOwnerLifecycleUnavailable`, which stays correct, and its comment ("the owner-lifecycle caveat") names the field's concept, which also stays accurate. Leave it unchanged and run it in Step 7.

**Interfaces:**
- Consumes: `startedRunResult(key, runContext string) RunStartResult`, `runUntracked(reason string) RunStartResult`, `ReasonRunMintFailed`, `ReasonOwnerLifecycleUnavailable`, `ResultApplied` (all existing in `internal/app/runtracker_start.go`).
- Produces: `func runStartStopNote(key string) string` (unexported, `internal/app/runtracker_start.go`). Test helper `func runStartedWant(key, runContext string) string` in `internal/app/runtracker_start_result_json_test.go`. That file is untagged, so the helper is visible to the integration-tagged files in package `app` too.

- [ ] **Step 1: Write the failing unit tests**

Append to `internal/app/runtracker_start_result_json_test.go` (imports `encoding/json`, `strings`, `testing` are already present):

```go
// runStartedWant is the exact text report of a started run: the two-token
// `run-started <key> <run-context>` line, then the stop note naming the run's own
// key (change 0501). It spells the note out literally rather than calling
// runStartStopNote, so a wrong note fails every assert that uses it.
func runStartedWant(key, runContext string) string {
	return "run-started " + key + " " + runContext + "\n" +
		"note: closing this session may not stop this run. To stop it: docket run cancel --key " +
		key + " --reason <why>"
}

// TestRunStartStopNoteNamesTheRunsOwnKey (change 0501): a started report's second
// line is the stop note, and its cancel command carries this run's own key, so a
// human can run it as printed. The report stays exactly two lines.
func TestRunStartStopNoteNamesTheRunsOwnKey(t *testing.T) {
	res := startedRunResult("k0501-own", "ctx-token")
	got := res.HumanText()
	if want := runStartedWant("k0501-own", "ctx-token"); got != want {
		t.Errorf("HumanText = %q, want %q", got, want)
	}
	if !strings.Contains(got, "docket run cancel --key k0501-own --reason <why>") {
		t.Errorf("stop note does not name the run's own key: %q", got)
	}
	if n := strings.Count(got, "\n"); n != 1 {
		t.Errorf("started report has %d newlines, want exactly 1 (two lines): %q", n, got)
	}
}

// TestRunStartTextOmitsBareTokenJSONKeepsIt (change 0501): the bare
// owner-lifecycle-unavailable token no longer appears in the text report, while
// the JSON owner_lifecycle field still carries it for machine readers. The JSON
// half is also the absence assert's non-vacuity companion: the same result still
// holds the token.
func TestRunStartTextOmitsBareTokenJSONKeepsIt(t *testing.T) {
	if ReasonOwnerLifecycleUnavailable != "owner-lifecycle-unavailable" {
		t.Fatalf("ReasonOwnerLifecycleUnavailable = %q, must stay %q", ReasonOwnerLifecycleUnavailable, "owner-lifecycle-unavailable")
	}
	res := startedRunResult("k0501", "ctx-token")
	if strings.Contains(res.HumanText(), ReasonOwnerLifecycleUnavailable) {
		t.Errorf("text report still prints the bare %q token: %q", ReasonOwnerLifecycleUnavailable, res.HumanText())
	}
	buf, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(buf), `"owner_lifecycle":"owner-lifecycle-unavailable"`) {
		t.Errorf("run.start JSON lost owner_lifecycle=owner-lifecycle-unavailable: %s", buf)
	}
}

// TestRunStartStopNoteGatedOnOwnerLifecycle (change 0501): the note is printed only
// when OwnerLifecycle is set, so text and JSON cannot drift apart. A run-untracked
// report never carries it.
func TestRunStartStopNoteGatedOnOwnerLifecycle(t *testing.T) {
	res := startedRunResult("k", "ctx")
	res.OwnerLifecycle = ""
	if got := res.HumanText(); got != "run-started k ctx" {
		t.Errorf("HumanText without OwnerLifecycle = %q, want %q", got, "run-started k ctx")
	}
	if got := runUntracked(ReasonRunMintFailed).HumanText(); strings.Contains(got, "note:") {
		t.Errorf("run-untracked report carries the stop note: %q", got)
	}
}
```

- [ ] **Step 2: Run the unit tests and confirm they fail for the right reason**

Run (from the feature worktree root):
```bash
go test -count=1 ./internal/app -run '^(TestRunStartStopNoteNamesTheRunsOwnKey|TestRunStartTextOmitsBareTokenJSONKeepsIt|TestRunStartStopNoteGatedOnOwnerLifecycle|TestRunStartResultCarriesNoRunID)$' -v
```
Expected: `TestRunStartStopNoteNamesTheRunsOwnKey` and `TestRunStartTextOmitsBareTokenJSONKeepsIt` FAIL. The text is still `run-started k0501-own ctx-token\nowner-lifecycle-unavailable`. `TestRunStartStopNoteGatedOnOwnerLifecycle` and `TestRunStartResultCarriesNoRunID` PASS, because the gating and the first line already hold. This proves Step 1's asserts can redden on the current defect.

- [ ] **Step 3: Implement the note in `internal/app/runtracker_start.go`**

3a. Replace the `ReasonOwnerLifecycleUnavailable` doc comment (the block that starts `// ReasonOwnerLifecycleUnavailable is the honest limitation a started run reports`) with:

```go
	// ReasonOwnerLifecycleUnavailable is the honest limitation a started run carries
	// in its JSON owner_lifecycle field (change 0375 Task 13): the default dispatch
	// route has NO owner-death or Stop lifecycle event that would cancel the run
	// automatically, so a Stop is the explicit `run.cancel` operation. Only the Codex
	// `agent.enter` route carries a signal-connected cancellation and a death
	// guardian; every other route relies on the human running `run.cancel`. It is a
	// standing caveat, never a refusal — a started run still starts. The text report
	// never prints this token: HumanText prints the stop note (runStartStopNote)
	// instead (change 0501).
	ReasonOwnerLifecycleUnavailable = "owner-lifecycle-unavailable"
```

3b. Replace the `OwnerLifecycle` field comment in `RunStartResult` with:

```go
	// OwnerLifecycle is the honest owner-lifecycle limitation of the dispatched
	// route (change 0375 Task 13). On a started run it carries
	// `owner-lifecycle-unavailable`: the default dispatch route has no automatic
	// Stop/owner-death cancellation, so a Stop is the explicit `run.cancel`
	// operation. Empty when no run is started. HumanText prints it as the stop note
	// (runStartStopNote), never as the bare token, and only when this field is set,
	// so the text and JSON forms cannot drift apart (change 0501).
	OwnerLifecycle string `json:"owner_lifecycle,omitempty"`
```

3c. Replace `HumanText`'s doc comment and body, and add `runStartStopNote` directly after it:

```go
// HumanText renders the report. A started run prints `run-started <key>
// <run-context>`. That first line is always two tokens, because the callers of
// startedRunResult refuse an empty run context as scope-failed before minting, and
// the key comes from a successful mint. When OwnerLifecycle is set, a second line
// follows: the stop note (runStartStopNote) naming this run's own key. A
// run-untracked report prints `run-untracked <reason-token>`; a usage error (a
// non-applied result) names its reason instead of a report line. The parent
// capability never appears here — only the child run context, which is meant for
// the child.
func (r RunStartResult) HumanText() string {
	if r.Result == ResultApplied {
		if r.Started {
			line := "run-started " + r.Key + " " + r.RunContext
			if r.OwnerLifecycle != "" {
				// The standing caveat, printed as a note a human can act on.
				line += "\n" + runStartStopNote(r.Key)
			}
			return line
		}
		return "run-untracked " + r.Reason
	}
	if r.Reason != "" {
		return fmt.Sprintf("%s: %s (%s)", r.Operation, r.Result, r.Reason)
	}
	return fmt.Sprintf("%s: %s", r.Operation, r.Result)
}

// runStartStopNote renders the second line of a started report (change 0501): the
// owner-lifecycle caveat as a plain note, with this run's own key filled into a
// ready-to-run cancel command. "May not" rather than "won't" keeps it true on every
// route: on the Codex route agent.enter, which runs after run.start, sets up a
// death guardian that run.start cannot see. <why> stays a literal placeholder for
// the human's reason.
func runStartStopNote(key string) string {
	return "note: closing this session may not stop this run. To stop it: docket run cancel --key " +
		key + " --reason <why>"
}
```

3d. Replace the step (7) comment at the end of `RunStart` (the four comment lines before `return startedRunResult(key, grant.ChildCapability)`) with:

```go
	// (7) Report the started run with its key, its run context, and the honest
	// owner-lifecycle caveat (change 0375 Task 13), which HumanText prints as a stop
	// note naming this run's `docket run cancel --key` command (change 0501). The key
	// and run context are both non-empty here (the empty-capability refusal at step
	// 5), so the first line is always two tokens.
```

- [ ] **Step 4: Run the unit tests and confirm they pass**

```bash
gofmt -l internal/app/runtracker_start.go internal/app/runtracker_start_result_json_test.go
go test -count=1 ./internal/app -run '^(TestRunStartStopNoteNamesTheRunsOwnKey|TestRunStartTextOmitsBareTokenJSONKeepsIt|TestRunStartStopNoteGatedOnOwnerLifecycle|TestRunStartResultCarriesNoRunID|TestRunStartResultRunContextKey)$' -v
```
Expected: `gofmt -l` prints nothing, and all five tests PASS.

- [ ] **Step 5: Mutation-test both new asserts** (AGENTS.md "Guards and tests"; learnings `assert-detects-removal-not-replacement`, `mutation-restore-needs-a-backup-copy`)

Back up the file with a copy, not with `git checkout`, because the edit is uncommitted. Apply each mutation, prove it landed with a count, run the tests, then restore from the backup:

```bash
WT="$(git rev-parse --show-toplevel)"
F="$WT/internal/app/runtracker_start.go"
S="$(mktemp -d "${TMPDIR:-/tmp}/mut-0501.XXXXXX")"
cp "$F" "$S/runtracker_start.go.bak"

# Mutation A: revert to the bare token.
perl -0pi -e 's/line \+= "\\n" \+ runStartStopNote\(r\.Key\)/line += "\\n" + r.OwnerLifecycle/' "$F"
grep -cF 'line += "\n" + r.OwnerLifecycle' "$F"   # must print 1 (fixed-string: the agent's grep may be ugrep/ERE)
go test -count=1 ./internal/app -run '^(TestRunStartStopNoteNamesTheRunsOwnKey|TestRunStartTextOmitsBareTokenJSONKeepsIt)$'
cp "$S/runtracker_start.go.bak" "$F"

# Mutation B: the note ignores the run's own key.
perl -0pi -e 's/runStartStopNote\(r\.Key\)/runStartStopNote("<key>")/' "$F"
grep -cF 'runStartStopNote("<key>")' "$F"   # must print 1
go test -count=1 ./internal/app -run '^TestRunStartStopNoteNamesTheRunsOwnKey$'
cp "$S/runtracker_start.go.bak" "$F"

cmp "$F" "$S/runtracker_start.go.bak" && rm -rf "$S"
```
Expected: each `grep -c` prints `1`. Mutation A makes both tests FAIL. Mutation B makes `TestRunStartStopNoteNamesTheRunsOwnKey` FAIL. After the restores, `cmp` succeeds. If a `grep -c` prints `0`, the mutation did not land and the run proves nothing: fix the substitution and repeat. If a landed mutation leaves a test green, the assert is defective: fix it before continuing.

- [ ] **Step 6: Update the integration assertions and their comments**

In `internal/app/runtracker_start_integration_test.go`, `TestIntegrationRunStartPreparesOuterScope`. Replace the comment that starts `// Started line: run-started <key> <run-context>, followed by the honest` and its three lines with:

```go
	// Started report: run-started <key> <run-context>, then the stop note naming
	// this run's own key (changes 0375 Task 13, 0501). The run is minted beside the
	// run-tracker record under the same key, so the printed cancel command is followable.
```
and replace the assertion

```go
	if got, want := res.HumanText(), "run-started "+res.Key+" "+scopeGrantChild+"\n"+ReasonOwnerLifecycleUnavailable; got != want {
```
with

```go
	if got, want := res.HumanText(), runStartedWant(res.Key, scopeGrantChild); got != want {
```

In the same file, `TestIntegrationRunStartFreshStartSurfacesKeyAndContext`. Replace the comment

```go
	// Human report line: run-started <key> <run-context>, then the owner-lifecycle
	// caveat.
```
with

```go
	// Human report: run-started <key> <run-context>, then the stop note naming this
	// run's own key (change 0501).
```
and make the same assertion replacement as above (`runStartedWant(res.Key, scopeGrantChild)`).

In `internal/app/change_integration_test.go`, `TestIntegrationChangeRuntimeRunStartMintsLoadableKey`, make the same assertion replacement:

```go
	if got, want := res.HumanText(), runStartedWant(res.Key, scopeGrantChild); got != want {
```

In `internal/app/runtracker_run_record_integration_test.go`, `TestIntegrationRunRecordNoAdapterReportsLifecycleUnavailable`. Keep the `res.OwnerLifecycle != ReasonOwnerLifecycleUnavailable` field check. Replace

```go
	if !strings.Contains(res.HumanText(), ReasonOwnerLifecycleUnavailable) {
		t.Fatalf("human text omits the owner-lifecycle caveat: %q", res.HumanText())
	}
```
with

```go
	if !strings.Contains(res.HumanText(), "docket run cancel --key "+res.Key+" --reason <why>") {
		t.Fatalf("human text omits the stop note naming this run's key: %q", res.HumanText())
	}
```
and append one sentence to that test's doc comment, after "the run tracker still starts.":

```go
// The text report prints it as the stop note naming this run's key (change 0501).
```

Then confirm no executable site still pins the bare token in text:
```bash
git grep -n -F -e '"\n"+ReasonOwnerLifecycleUnavailable' -e 'HumanText(), ReasonOwnerLifecycleUnavailable' -- internal
```
Expected: no output. The positive control `git grep -c -e ReasonOwnerLifecycleUnavailable -- internal/app` still prints counts for `runtracker_start.go` and the test files, which proves the grep can match.

- [ ] **Step 7: Run the focused integration tests**

```bash
go vet -tags integration ./internal/app
go test -tags integration -count=1 ./internal/app -run '^(TestIntegrationRunStartPreparesOuterScope|TestIntegrationRunStartFreshStartSurfacesKeyAndContext|TestIntegrationChangeRuntimeRunStartMintsLoadableKey|TestIntegrationRunRecordNoAdapterReportsLifecycleUnavailable|TestIntegrationRunStartStartedResultFields|TestRunStart)' -v
```
Expected: `vet` is clean and every listed test PASSES. The `^TestRunStart` prefix also runs the unit tests from Step 1 under the integration tag.

- [ ] **Step 8: Commit**

```bash
git add internal/app/runtracker_start.go internal/app/runtracker_start_result_json_test.go \
  internal/app/runtracker_start_integration_test.go internal/app/change_integration_test.go \
  internal/app/runtracker_run_record_integration_test.go
git status --short   # only the five paths above, staged
git commit -m "fix(0501): run.start prints a plain stop note naming its own key instead of the bare owner-lifecycle-unavailable token"
```

---

### Task 2: Tell coordinators about the note and not to relay it, then regenerate the mirror and `AGENTS.md`

**Files:**
- Modify: `cursor-rules/run-tracker.md` (section "Stopping a dispatched run — there is no automatic Stop button", first paragraph)
- Regenerate (never hand-edit): `internal/assets/embedded/tree/cursor-rules/run-tracker.md`, `internal/assets/embedded/manifest.json` (via `go generate ./internal/assets`)
- Regenerate (never hand-edit): the `AGENTS.md` `docket:dispatch` managed block (via `docket install --repo-dir`). `CLAUDE.md` is a symlink to `AGENTS.md` and needs nothing.
- Modify: `internal/repoguard/budgets_test.go` (`dispatchBudget` constant and its comment)

**Interfaces:**
- Consumes: nothing from Task 1 at the code level. The rule describes Task 1's note in words only.
- Produces: no code symbols.

- [ ] **Step 1: Edit the rule source**

Run from the feature worktree root. This refuses unless the old text occurs exactly once:

```bash
python3 - cursor-rules/run-tracker.md <<'EOF'
import sys
p = sys.argv[1]
s = open(p).read()
old = ("A run you dispatched has **no automatic Stop**: closing a tab, interrupting the coordinator, or\n"
       "killing a process does not tell the run tracker the run is over, and `run.start` says so (it\n"
       "reports the honest owner-lifecycle caveat). To stop a dispatched run deliberately, invoke the\n"
       "explicit `run.cancel` operation (argv resolved from the capability catalog) with the key\n"
       "`run.start` gave you, plus a human reason — `--key <key> --reason <why>`.\n")
new = ("A run you dispatched has **no automatic Stop**: closing a tab, interrupting the coordinator, or\n"
       "killing a process does not tell the run tracker the run is over, and `run.start` prints a one-line\n"
       "stop note saying so. That note is for you, not a finding: do not repeat it in your dispatch report,\n"
       "and bring up `run.cancel` only when the human asks how to stop a run or a run actually needs\n"
       "stopping. To stop a dispatched run deliberately, invoke the explicit `run.cancel` operation (argv\n"
       "resolved from the capability catalog) with the key `run.start` gave you, plus a human reason —\n"
       "`--key <key> --reason <why>`.\n")
assert s.count(old) == 1, "expected exactly one occurrence of the old Stop paragraph; refusing to edit"
open(p, "w").write(s.replace(old, new))
EOF
git diff --stat -- cursor-rules/run-tracker.md
grep -c -e 'honest owner-lifecycle caveat' cursor-rules/run-tracker.md   # must print 0
grep -c -e 'do not repeat it in your dispatch report' cursor-rules/run-tracker.md   # must print 1
```
Expected: one file changed, and the two counts are `0` and `1`. Leave the heading "Stopping a dispatched run — there is no automatic Stop button" unchanged (out of scope).

- [ ] **Step 2: Regenerate the embedded mirror and see the `AGENTS.md` drift (RED)**

```bash
go generate ./internal/assets
go run ./cmd/genassets -repo . -check
go test -count=1 ./internal/repoguard -run '^TestCommittedCodexDispatchMatchesGenerator$'
```
Expected: `go generate` rewrites `internal/assets/embedded/tree/cursor-rules/run-tracker.md` and `internal/assets/embedded/manifest.json` (size and sha256 of that entry), and `-check` passes. The drift test FAILS with `AGENTS.md dispatch block is stale; regenerate it from the reposeed dispatch interior`. This RED proves the guard sees the change. `.docket.yml` enables `claude` and `opencode`, not `codex`, so the block carries no Codex clause.

- [ ] **Step 3: Regenerate the `AGENTS.md` block through the install path**

This is the path change 0491 used, verified at change 0488. Install refuses to overwrite a block it has no ownership record for, and its sanctioned remedy is to delete the block and rerun. With a throwaway `HOME`, nothing outside the worktree is touched. Run from the feature worktree root:

```bash
WT="$(git rev-parse --show-toplevel)"
S="$(mktemp -d "${TMPDIR:-/tmp}/regen-0501.XXXXXX")"
git -C "$WT" status --porcelain --untracked-files=all > "$S/before.txt"
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
git -C "$WT" status --porcelain --untracked-files=all > "$S/after.txt"
diff "$S/before.txt" "$S/after.txt"
git -C "$WT" diff -- AGENTS.md
```
Expected: `install: applied`. `git diff -- AGENTS.md` changes only the Stop paragraph inside the dispatch block, matching Step 1 word for word. The block keeps its position, and its markers and surroundings are unchanged.

Inspect the `before`/`after` diff:
- New untracked paths the install created inside the worktree (for example `.opencode/`, which did not exist before this step) are byproducts of the throwaway run. Remove exactly those paths (`rm -rf "$WT/.opencode"` if it is listed and was absent from `before.txt`). Never stage them.
- If any **tracked** path other than `AGENTS.md` changed, or `AGENTS.md` changed outside the dispatch block, stop and return `BLOCKED` naming the paths. Do not hand-patch.

Then run `rm -rf "$S"`.

- [ ] **Step 4: Re-baseline the dispatch block word budget**

The block sits exactly at its budget today (897 words; `dispatchBudget = 897`), and Step 1 adds words. Measure the new count the way `dispatchBlockWords` counts it (`strings.Fields` over the lines between the markers, which matches `wc -w` for this text):

```bash
awk '/docket:dispatch:start/{f=1;next}/docket:dispatch:end/{f=0;next}f' AGENTS.md | wc -w
go test -count=1 ./internal/repoguard -run '^TestDispatchBlockBudget$'
```
Expected: the count is above 897, and the test FAILS with `the AGENTS.md dispatch block is N words, over its 897-word budget`. N must be strictly below `dispatchOld` (1156); if it is not, return `BLOCKED` with the number.

In `internal/repoguard/budgets_test.go`, set `dispatchBudget` to exactly N, and prepend this to its trailing comment (immediately after `// `):

```
0501: run.start's stop note replaces the "honest owner-lifecycle caveat" wording, and coordinators are told not to relay it (was 897); 
```
so the comment reads `// 0501: run.start's stop note replaces ... (was 897); 0491: the run id is retired ...`. Do not touch `dispatchOld`.

- [ ] **Step 5: Run the guards**

```bash
go build ./... && go vet ./...
go run ./cmd/genassets -repo . -check
go test -count=1 ./internal/repoguard/ ./internal/assets/ ./internal/harness/...
go test -count=1 ./internal/install/ ./internal/reposeed/
```
Expected: all PASS, including `TestCommittedCodexDispatchMatchesGenerator`, `TestDispatchBlockBudget`, and the embedded-mirror checks. The harness goldens use synthetic run-tracker payloads, so they need no update. If a harness golden does fail on this text, regenerate it with that package's `-update` flag, commit it with this task, and name it in the report.

- [ ] **Step 6: Commit**

```bash
git add cursor-rules/run-tracker.md internal/assets/embedded AGENTS.md internal/repoguard/budgets_test.go
git status --short   # only this task's paths staged; no untracked install byproducts left
git commit -m "docs(0501): the run-tracker rule names run.start's stop note and tells coordinators not to relay it"
```

---

## Build gate

`docket-build` runs the whole suite once at the end, using whatever `build.test_command` resolves to in `.docket.yml` (today `go run ./cmd/docket development test`). Read the budget report even on a green run (`BUDGET WATCH:` / `PARALLEL-SENSITIVE:` / `SERIAL CONFIRMED OVER BUDGET:`).

Note for the results file: the installed `docket` binary keeps printing the old bare token, and coordinator sessions keep loading the old rule text, until the post-merge rebuild and a session restart (learning `generated-artifact-loaded-at-process-start`). The unit tests in Task 1 are the oracle for the printed note; do not run a real `run start` to check it (that would mint a live run record).
