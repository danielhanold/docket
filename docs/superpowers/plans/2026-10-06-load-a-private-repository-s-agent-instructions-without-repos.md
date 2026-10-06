<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0535 — Load a private repository's agent instructions without repository files](../../changes/active/0535-load-a-private-repository-s-agent-instructions-without-repos.md)**
<!-- docket:backlink:end -->

# Load a Private Repository's Agent Instructions Without Repository Files: Implementation Plan

> **For agentic workers:** `docket-build` executes this plan. It routes each task to a tier agent running the `docket-build-task` contract. Each task carries its own focused test cycle and ends in one commit. The whole suite runs once at the end. Steps use checkbox (`- [ ]`) syntax.

**Goal:** A private repository's parent-facing rules (dispatch block plus promoted lessons) live in `<git-common-dir>/dckt/AGENTS.md`. A read-only `docket instructions` prints that file, or one section, only inside a private repository, optionally as Claude Code or Cursor session-start hook output. `docket install` writes one content-free trigger per harness, once per machine: two `SessionStart` entries in `~/.claude/settings.json`; a `sessionStart` entry in `~/.cursor/hooks.json`; the plugin `~/.config/opencode/plugins/dckt-instructions.js`; a `dckt:` pointer block in `~/.codex/AGENTS.md` (best effort). Nothing goes in a private working tree or `.git/info/exclude`.

**This revision** replaces the plan whose Task 1 spike halted the first build, following the re-groomed spec.

**Architecture:**
- `internal/document` gains a neutral marker namespace: `<!-- dckt:NAME:start … -->` blocks are addressed as `dckt:NAME`; bare names stay `docket:`.
- `internal/install`: a transaction may retire one managed block and write another in the same file, removal first (Codex's leftover `docket:dispatch` and the pointer share `~/.codex/AGENTS.md`). A new `hook-entries` kind: **one** target per hooks JSON file owns an ordered list of commands in one dialect (`claude`, `cursor`), because the state refuses two records on one path (`ValidateState`) and a transaction cannot apply two removals to one file (`applySteps` re-verifies each later removal's pre-image digest). Edits are byte splices.
- `internal/harness/trigger.go` holds every trigger; all four adapters plan theirs.
- `reposeed.PlanPrivate` plans the private file; `app.ResolveRepoPhase` branches on the detected mode, so install's repository phase and `repository init --private` write it and nothing else.
- `app.ReadPrivateInstructions` resolves the common dir from the filesystem alone (no git process), so every session pays almost nothing.

**Tech Stack:** Go, cobra; real-git tests behind `//go:build integration`, sharded by name prefix (new shard `TestIntegrationPrivateInstructions`); `go generate ./internal/assets/` for skill twins.

**Spec:** `docs/superpowers/specs/2026-10-06-load-a-private-repository-s-agent-instructions-without-repos-design.md`, already in the feature worktree. Read it first, especially *Delivery spike (2026-10-06)* and *Design*.

## Global Constraints

- **The private file** is `<git-common-dir>/dckt/AGENTS.md`. It holds the managed `docket:dispatch` block, with the same interior the repository-level block carries: `harness.DispatchInterior`, or `harness.CodexDispatchInterior` when `codex` is opted in. Its owners are every opted-in harness (claude, codex, cursor, opencode); it is present iff at least one is opted in. Promoted lessons are added to it by hand, outside the block.
- **Nothing in a private worktree.** No AGENTS.md, CLAUDE.md, `.cursor/rules/docket-dispatch.mdc`, or any other rule file, and no `.git/info/exclude` edit.
- **`docket instructions`** (details in Task 7): prints the private file from any worktree of a private repository and **nothing**, exit 0, anywhere else; `--section dispatch|lessons`; `--hook claude|cursor` wraps the content, Cursor locating the repository from `CURSOR_PROJECT_DIR` or stdin `workspace_roots`, never the working directory. **Hook modes never fail a session:** a runtime error means no output and exit 0. An invalid flag value is an argument error, exit 2, in every mode.
- **The triggers, verbatim,** are the constants in Task 4 (`internal/harness/trigger.go`): the spec's commands, entry shapes, plugin bytes, and pointer wording. Nothing else may spell them.
- **No rule text and no "docket" in any trigger** (Task 4). **The dispatch block fits one Claude hook:** under 10,000 characters for every `agent_harnesses` selection (Task 7).
- **JSON hook safety:** an entry is identified by its exact command; install adds only absent ones and every other byte survives; uninstall removes only exact entries and reports a modified one; an unparseable or wrong-typed file is a conflict, never "absent" (Task 3). **The OpenCode plugin** is a wholly owned `KindFile`: created when absent, no-op when identical, conflict when modified, removed only on an exact match.
- **Shared repositories are unchanged:** their repository-level blocks stay as they are, and they get no private file.
- **The triggers are installed whenever their harness is installed**, including on machines with no private repository yet.
- **Docs:** command help and skills only. No `docs/` pages.
- **AGENTS.md rules:**
  - Mutation-test every guard, restoring from a **backup copy** (never `git checkout --`), and run with `-count=1`.
  - Template every `mktemp` as `"${TMPDIR:-/tmp}/<name>.XXXXXX"`.
  - Anchor cross-references on symbols, never line numbers.
  - Never pipe into `grep -q` or `head`; capture into a variable first.
  - Run `go generate ./internal/assets/` after any skill edit.
- **Hermetic home.** Every test that reaches the installer or `installAuthorizedSurfaces` pins `HOME`, `XDG_CONFIG_HOME`, and `XDG_DATA_HOME` to temp directories. This change adds writes to `~/.claude/settings.json`, `~/.cursor/hooks.json`, and `~/.config/opencode/plugins/`, so a leak now overwrites real user configuration.

## Review Focus

Each item has a test in its owning task.
1. **Hooks-file content docket did not write** (a user group with a matcher, another event, unrelated keys, tabs, a compact one-line file, other Cursor entries): only missing entries are inserted, and install-then-uninstall returns the original bytes (Task 3).
2. **A malformed, non-object, wrong-typed, or symlinked hooks file:** a conflict with the file untouched, never "entries absent" (Task 3).
3. **A user-edited docket entry** (a `timeout` added): install is a no-op for it; uninstall reports a conflict and leaves the file byte-identical (Task 3).
4. **A `~/.codex/AGENTS.md` with a provable leftover `docket:dispatch` block:** one install retires it and adds the pointer, user text survives, and a later failure rolls back byte-for-byte (Task 2).
5. **`docket instructions` from a subdirectory, a `.git`-file worktree, an unreadable file, and `--hook cursor` with garbage stdin:** the right file; stderr plus exit 1 in plain mode and silence plus exit 0 in hook mode; nothing for garbage (Task 7).

## Learnings applied

- **harness-behavior-is-mode-and-version-scoped** / **external-truth-needs-a-human-checkpoint** / **generated-artifact-loaded-at-process-start:** Task 9 records version, mode, and flags per verdict, uses fresh processes only, and names human items for what it cannot run.
- **config-layer-write-and-read-hazards:** new user-level writes make a `HOME` leak a data-loss bug (Task 4 audit).
- **probe-error-is-not-clean-absence:** an unreadable private file, a layout probe error, or an unparseable hooks file is reported, never "absent".
- **shared-resource-keeps-first-owner-assumptions:** two-block `~/.codex/AGENTS.md` (Task 2), the four-owner private file (Task 6), two entries in one `settings.json` target (Task 3).
- **byte-pattern-guard-matches-a-spelling**, **assert-detects-removal-not-replacement**, **specified-but-unreachable**, **compensating-assert-must-exist-when-cited:** Task 4's guard renders through the producers and is mutation-tested; Task 7 parses every trigger command with the real CLI; `TestNoGlobalParentSurface` is narrowed only beside its replacement assert.
- **plan-supplied-test-code-is-unverified**, **distributed-body-has-no-local-repo**, **intermediate-task-state-buildable.**

## ADRs to record

The parent records this through `docket-adr`; it is not a build task.

1. **A private repository's parent-facing rules live in `<git-common-dir>/dckt/AGENTS.md`.** They reach the agent through content-free user-level triggers: two Claude Code `SessionStart` hooks in `~/.claude/settings.json`, split by meaning (dispatch block, lessons) so each fits Claude Code's per-hook 10,000-character cap; a Cursor `sessionStart` hook in `~/.cursor/hooks.json`; an OpenCode system-prompt plugin; and a Codex pointer block (best effort). This narrowly revisits 0351's retirement of user-level parent-facing writes. It is acceptable because the triggers carry no rules, so the drift that retired the user-level copy cannot recur. Relates to ADR-0036 and ADR-0078.

## Residuals (for the results file)

- The triggers load in a session started **after** install; a running session keeps its old context.
- A private repository whose git directory is not `<primary>/.git` (`--separate-git-dir`) refuses the repository phase.
- A symlinked `settings.json` or `hooks.json` (dotfiles managers) is a conflict that stops `docket install`; the transaction publishes by rename and never writes through a link.
- Uninstall leaves a hooks file docket created as `{}` / `{"version": 1}`, and removes a pre-existing empty `hooks` object or event array with docket's last entry.
- An older binary cannot read the new `hook-entries` state fields (a downgrade reads the state as invalid).
- OpenCode's hook is `experimental.`; Cursor documents `sessionStart` as fire-and-forget; only the Cursor CLI is exercised; lessons past 10,000 characters reach Claude as its path-plus-preview.
- A foreign `dckt` (#534) makes the triggers fail harmlessly; `docket instructions` still works by hand.

---

### Task 1: The neutral `dckt:` marker namespace in `internal/document`

**Build tier:** standard

**Files:** Modify `internal/document/markers.go` (and `Block`'s doc comment in `document.go` if it names only `docket:`). Test: `internal/document/markers_test.go`.

**Interfaces:**
- Produces: a block marked `<!-- dckt:NAME:start (…) -->` / `<!-- dckt:NAME:end -->` is located, patched, inserted, and removed under the **qualified name** `"dckt:NAME"` through the existing APIs (`Block`, `PatchSet.ReplaceBlock/InsertBlock/RemoveBlock`); `docket:` blocks keep bare names. `const document.NeutralMarkerPrefix = "dckt:"`. `document.MarkerSpelling(name) string` gives `"docket:dispatch"` for `"dispatch"` and returns `"dckt:private-instructions"` unchanged.

- [ ] **Step 1: Write the failing tests** in `markers_test.go`:

```go
func TestNeutralMarkerBlockRoundTrip(t *testing.T) {
	src := []byte("user line\n<!-- dckt:private-instructions:start (managed — do not hand-edit) -->\nbody\n<!-- dckt:private-instructions:end -->\n<!-- docket:dispatch:start -->\nd\n<!-- docket:dispatch:end -->\n")
	doc, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	b, ok := doc.Block("dckt:private-instructions")
	if !ok || string(src[b.Interior.Start:b.Interior.End]) != "body\n" || b.Annotation != "managed — do not hand-edit" {
		t.Fatalf("neutral block not located: %+v ok=%v", b, ok)
	}
	if _, ok := doc.Block("private-instructions"); ok {
		t.Fatal("a bare name must not find a dckt: block")
	}
	if _, ok := doc.Block("dispatch"); !ok {
		t.Fatal("the docket: block must keep its bare name")
	}
	var p PatchSet
	p.RemoveBlock("dckt:private-instructions")
	out, err := doc.Apply(p)
	if err != nil || string(out) != "user line\n<!-- docket:dispatch:start -->\nd\n<!-- docket:dispatch:end -->\n" {
		t.Fatalf("remove: %q %v", out, err)
	}
}
```

  Beside it: `TestNeutralMarkerInsertRendersDcktPrefix` (`InsertBlock("dckt:private-instructions", "managed — do not hand-edit", "x", AtDocumentStart)` on `"keep\n"` renders the `dckt:` start line, `x`, and the end line ahead of `keep\n`, with no `docket` anywhere); `TestMalformedNeutralMarkerRefuses` (`<!-- dckt:Bad Name:start -->` fails `Parse`); `TestSameNameInBothNamespacesIsTwoBlocks` (a `docket:x` pair and a `dckt:x` pair parse; both `Block("x")` and `Block("dckt:x")` are found); `TestMarkerSpelling`.

- [ ] **Step 2: Run and confirm they fail.** `go test ./internal/document/ -run 'Neutral|SameNameInBoth|MarkerSpelling' -count=1`. Expected: FAIL (`MarkerSpelling` undefined; `dckt:` lines are prose today).

- [ ] **Step 3: Implement** in `markers.go`:

```go
// NeutralMarkerPrefix qualifies a block in the neutral `dckt:` marker namespace,
// used where nothing docket-named may appear. A bare name is in `docket:`.
const NeutralMarkerPrefix = "dckt:"

markerPrefixRE = regexp.MustCompile(`^<!-- (?:docket|dckt):`)
markerRE = regexp.MustCompile(
	`^<!-- (docket|dckt):([a-z][a-z0-9-]*):(start|end)(?: \(([^)]*)\))? -->$`)

var blockNameRE = regexp.MustCompile(`^(?:dckt:)?[a-z][a-z0-9-]*$`)

func splitBlockName(name string) (namespace, bare string) {
	if rest, ok := strings.CutPrefix(name, NeutralMarkerPrefix); ok {
		return "dckt", rest
	}
	return "docket", name
}

// MarkerSpelling is how a block name appears inside its marker lines.
func MarkerSpelling(name string) string {
	ns, bare := splitBlockName(name)
	return ns + ":" + bare
}
```

  `startMarkerLine` and `endMarkerLine` render `"<!-- " + MarkerSpelling(name) + ":start …"` / `":end -->"`. In `scanMarkers` the submatch indexes shift by one: the name is `m[2]` for `docket` and `NeutralMarkerPrefix + m[2]` for `dckt`, the kind `m[3]`, the annotation `m[4]`; error `Name` fields use the qualified name. Pairing, duplicates, and `Block(name)` then work unchanged. Update the regex comments to describe both namespaces.

- [ ] **Step 4: Run the package.** `go test ./internal/document/ -count=1`. Expected: PASS, fuzz seeds included.

- [ ] **Step 5: Run the dependants.** First confirm nothing relies on a column-0 `<!-- dckt:` line being prose: `out="$(grep -rn -e '^<!-- dckt:' --include='*.md' --include='*.go' . || true)"` (only fenced examples may match). Then `go test ./internal/install/ ./internal/render/ ./internal/app/ -count=1`. Expected: PASS.

- [ ] **Step 6: Commit.** Message: `feat(document): neutral dckt: marker namespace for managed blocks`.

---

### Task 2: The installer retires one block and writes another in the same file

**Build tier:** premium. A wrong step order corrupts a user's instruction file or its rollback.

**Files:** Modify `internal/install/inspect.go` (`remedyBlockMarkers`, `remedyForeignBlock`, `remedyDriftedBlock`) and `internal/install/txn.go` (`rejectDuplicateDestinations`; the `sort.SliceStable` in `planSteps`). Test: `internal/install/retire_test.go`, `internal/install/txn_test.go`.

**Interfaces:**
- Consumes: `document.MarkerSpelling` (Task 1).
- Produces: one transaction may carry a managed-block **removal** and writes of other block names in one file, removal first. Task 4 relies on it for `~/.codex/AGENTS.md`.

Background: `applySteps` re-verifies the captured pre-image digest of every removal still ahead before each removal (and again in `applyStep`), so a removal after any other step on its path always fails as stale. A write is not re-verified; it re-reads the file (`renderManagedBlock`) and composes. `capturePreImage` already names each backup by `Seq`, so two steps on one path get two backups, and `Rollback` restores newest first, so both pre-images restore the original bytes.

- [ ] **Step 1: Write the failing tests.**
  - **`TestInstallRetiresLeftoverBlockAndWritesPointerInOneFile`** (`retire_test.go`, existing test planners, temp roots, `RealFS`): `<home>/.codex/AGENTS.md` = `"# mine\n\n"` + a `docket:dispatch` block whose interior the prior `State` records + `"tail\n"`. Planner `codex`'s `Plan` returns one `KindManagedBlock` target on that path (`BlockName: "dckt:private-instructions"`, `Annotation: "managed — do not hand-edit"`, `Content: []byte("pointer")`, `Role: "trigger"`); its `GlobalDispatchTarget` names that file's `dispatch` block. After `Install`: success; no `docket:dispatch` block; one `dckt:private-instructions` block with interior `pointer`; `# mine` and `tail` survive; one `remove` and one `update` action for the path; one state record for the path.
  - **`…RollsBackBothSteps`:** the same, plus a planned `KindFile` target at a later-sorting path (`<home>/.codex/zz.txt`) whose staging write fails through the existing FS-failure injection: the AGENTS.md bytes equal the original.
  - In `txn_test.go`, if absent: a removal and a write of the **same** block on one path refuse; two removals of different blocks on one path refuse.

- [ ] **Step 2: Run and confirm the first fails.** `go test ./internal/install/ -run 'RetiresLeftoverBlockAndWritesPointer|RejectDuplicateDestinations' -count=1`. Expected: FAIL with `is written by more than one step`.

- [ ] **Step 3: Implement.**
  - `rejectDuplicateDestinations`: on a repeated path, allow the pair only when both are `KindManagedBlock` with different `BlockName`s and at most one step on that path is a removal (track removals per path). Comment why: a second removal on a path would fail its own pre-image re-verification; a same-block pair's outcome would depend on order.
  - Ordering: cleaned path ascending, then removals before writes, stable. Comment why (the background above).
  - `inspect.go`: the three block remedies say `"the " + document.MarkerSpelling(block) + " markers …"` / `" block …"`; the `docket:dispatch` text stays byte-identical.

- [ ] **Step 4: Run.** `go test ./internal/install/ -count=1`. Expected: PASS; a whole-file write+write still refuses.

- [ ] **Step 5: Mutation-test.** Back up `txn.go`, sort by path alone, rerun the Step 1 test with `-count=1`: red with a stale pre-image error. Restore and confirm green.

- [ ] **Step 6: Commit.** Message: `feat(install): retire one managed block and write another in the same file`.

---

### Task 3: A `hook-entries` target kind for Claude Code and Cursor hooks files

**Build tier:** premium. A defect here eats the user's settings.

**Files:**
- Create: `internal/install/hook_entries.go`, `internal/install/hook_entries_test.go`
- Modify: `internal/install/state.go` (`KindHookEntries`, `TargetRecord` fields, `validateTarget`); `target.go` (`Target` fields, `validate`, `RecordFor`); `inspect.go` (`InspectTarget` switch, `recordMatchesDisk`, two remedies); `txn.go` (`applyStep` write and removal branches, `removalTarget`); `uninstall.go` (`proveUninstallRemoval`).

**Interfaces:**
- Produces:
  - `install.KindHookEntries TargetKind = "hook-entries"`; `install.HookDialectClaude = "claude"`, `install.HookDialectCursor = "cursor"`.
  - `Target.HookDialect string`, `Target.HookCommands []string`; `TargetRecord.HookDialect` (`json:"hook_dialect,omitempty"`), `TargetRecord.HookCommands` (`json:"hook_commands,omitempty"`).
  - `func install.NewHookFileBytes(dialect string, commands []string) []byte`: what install writes when the file is absent. Task 4's guard renders through it.
  - A target `{Path, Kind: KindHookEntries, HookDialect, HookCommands, Role}` owns one canonical entry per command under `hooks.<event>`:
    - `claude`: event `SessionStart`, entry `{"hooks":[{"type":"command","command":C}]}` (no matcher, so every session source fires it); new file `{"hooks":{"SessionStart":[…]}}`.
    - `cursor`: event `sessionStart`, entry `{"command":C}`; new file `{"version": 1, "hooks": {"sessionStart": […]}}`.
  - Record identity: `SHA256 = hookEntriesDigest(dialect, commands)`, the sha256 (`hashBytes`) of `json.Marshal` of `{"dialect":…,"commands":[…]}`.

- [ ] **Step 1: Write the failing tests** in `hook_entries_test.go`:
  - **`TestInsertHookEntriesRoundTrips`**, table-driven over dialect × original, with commands `{"a --x", "a --y"}` (claude) and `{"a --z"}` (cursor). Originals: `{}\n`; `{\n  "model": "opus"\n}\n`; `{\n\t"env": {\n\t\t"A": "1"\n\t}\n}\n` (tabs); `{"model":"opus"}` (compact, no newline); a file whose `hooks` has another event (claude `Stop`, cursor `afterFileEdit`); a file whose event array already holds a user entry (claude: a group with `"matcher": "startup"`; cursor: `{"command": "./mine.sh"}`); a file with two `hooks` keys, the first `{}` and the last holding another event. For each:
    - `ins, err := insertHookEntries(orig, dialect, cmds)` succeeds and `json.Valid(ins)`;
    - `readHookEntries(ins, dialect)` holds one exact canonical entry per command plus every original entry;
    - `removeHookEntries(ins, dialect, cmds)` returns bytes equal to `orig` (the byte-for-byte survival property).
    - Separately, cursor `{\n  "version": 1,\n  "hooks": {}\n}\n` round-trips to `{\n  "version": 1\n}\n` (see *Residuals*).
  - **`TestInsertHookEntriesAddsOnlyMissing`:** a claude file holding the exact `a --x` group gets only `a --y`; so does one whose `a --x` handler has `"timeout": 5` added.
  - **`TestNewHookFileBytes`:** the claude form equals `json.MarshalIndent` (prefix `""`, two-space indent) of the new-file document plus `\n`; the cursor form starts `{\n  "version": 1,\n  "hooks": {` and decodes to `{"version":1,"hooks":{"sessionStart":[{"command":"a --z"}]}}`. `insertHookEntries(nil, …)` and `insertHookEntries([]byte(" \n"), …)` both equal `NewHookFileBytes`.
  - **`TestHookEntriesInspect`** (temp dir, both dialects): absent → `create`; none of the commands → `update`; one of claude's two → `update`; all present, exact or with an added `timeout` → `no-op`; `{"hooks": []}`, a wrong-typed event value (claude `{"hooks": {"SessionStart": {}}}`, cursor `{"hooks": {"sessionStart": "x"}}`), `[1]`, `null`, and `not json` → `conflict`, reason `managed-block-invalid`, file untouched; a symlinked file → `conflict`, reason `ownership-conflict`, with the not-a-regular-file remedy.
  - **`TestHookEntriesInstallUninstallLifecycle`**, both dialects, driving `Install` then `Uninstall` through `Options` with a stub planner, in a temp home seeded with a user hooks file:
    - after uninstall the file equals the seed bytes;
    - a second install is a no-op (`Applied == false`);
    - with one docket entry edited (add `"timeout": 5`) between the two, uninstall reports a `conflict` for the path and leaves the file byte-identical;
    - an injected later-step failure rolls the file back to the seed bytes;
    - with no seed, uninstall leaves `{}\n` (claude) / `{\n  "version": 1\n}\n` (cursor).
  - **`TestValidateTargetHookEntries`:** a `hook-entries` record without `hook_dialect`, with an unknown dialect, with no commands, with an empty or duplicate command, or without `sha256` is invalid; a `file` record carrying `hook_commands` is invalid; a `Target` with the same defects fails `validate` with `ErrInvalidTarget`.

- [ ] **Step 2: Run the tests and confirm they fail.** Run: `go test ./internal/install/ -run 'HookEntries|HookFile' -count=1`. Expected: FAIL to compile.

- [ ] **Step 3: Implement `hook_entries.go`.** Package comment: a hook-entries target owns a fixed list of commands inside one JSON hooks file; one target per file because the state allows one record per path and a transaction one removal per path; every edit is a byte splice and nothing else is re-encoded. The code below is a starting point (plan-supplied code is unverified); the round-trip table is the oracle.

  The dialect layer (write these from the description):
  - `hookEvent(dialect) (string, bool)`: `SessionStart` / `sessionStart`; unknown → false.
  - `canonicalHookEntry(dialect, command) any`: structs `claudeHookGroup{Hooks []claudeHookHandler}` with `claudeHookHandler{Type, Command}` (JSON `type`, `command`), and `cursorHookEntry{Command}`.
  - `entryHasCommand(raw, dialect, command) bool`: claude — any handler in `.hooks` whose `command` equals it; cursor — `.command` equals it; a value that does not decode is false.
  - `isExactEntry(raw, dialect, command) bool`: `reflect.DeepEqual` of `raw` decoded into `any` and the canonical entry round-tripped through JSON into `any`.
  - `NewHookFileBytes`: `json.MarshalIndent(doc, "", "  ")` plus `\n`, where `doc` is a struct `{Hooks map[string][]any}` (claude) or `{Version int; Hooks map[string][]any}` with `Version: 1` (cursor), so `version` precedes `hooks`.

```go
var errHookFileInvalid = errors.New("install: hooks file is not a JSON object whose hooks value is an object and whose event value is an array")

// readHookEntries returns the event entries. ok=false: not editable — never
// "absent". A whitespace-only file is editable and empty.
func readHookEntries(src []byte, dialect string) (entries []json.RawMessage, ok bool) {
	event, known := hookEvent(dialect)
	if !known {
		return nil, false
	}
	if len(bytes.TrimSpace(src)) == 0 {
		return nil, true
	}
	var top map[string]json.RawMessage
	if json.Unmarshal(src, &top) != nil || top == nil {
		return nil, false
	}
	raw, has := top["hooks"]
	if !has {
		return nil, true
	}
	var hooks map[string]json.RawMessage
	if json.Unmarshal(raw, &hooks) != nil || hooks == nil {
		return nil, false
	}
	ev, has := hooks[event]
	if !has {
		return nil, true
	}
	if !bytes.HasPrefix(bytes.TrimSpace(ev), []byte("[")) || json.Unmarshal(ev, &entries) != nil {
		return nil, false
	}
	return entries, true
}
```

  The splicer. Every splice runs only after `readHookEntries` returned ok, so the input is valid JSON with an object top level; that is what lets the scanner skip error handling. Write:
  - A byte-span scanner: `valueEnd(src, i)` (string with `\\` escapes, nested `{`/`[` with strings skipped, scalars up to `,}] \t\r\n`), `objectMembers(src, open) ([]jsonMember, close int)` where a member records its key, its whole span (key quote through value end), and its value span, and `arrayElements(src, open) ([]jsonSpan, close int)`. `findMember` returns the **last** member with a key, matching encoding/json's last-duplicate-wins rule that `readHookEntries` used.
  - `insertHookEntries(src, dialect, commands) ([]byte, error)`: not editable is `errHookFileInvalid`; collect the commands no entry runs (`entryHasCommand`); none missing returns `src` unchanged; a blank `src` returns `NewHookFileBytes(dialect, missing)`; otherwise append each missing entry in order. One append: find `hooks` in the top object, then the event in it, then splice the entry in as the **last** element of the event array; when the event or `hooks` is absent, splice `"<event>": [entry]` or `"hooks": {"<event>": [entry]}` in as the last member of its parent instead. Splicing after the last entry writes `",\n" + pad + text`; into an empty container writes `"\n" + pad + text + "\n" + closingPad` right after the opening bracket. `pad` is the file's indent unit (the leading whitespace of the first indented line after line 1, else two spaces) repeated depth+1 times, and `text` is `json.MarshalIndent(v, pad, unit)`.
  - `removeHookEntries(src, dialect, commands) ([]byte, error)`: not editable is `errHookFileInvalid`; for each command, **last command first** (the reverse of insertion, so a round trip splices back exact bytes), repeatedly remove the first `isExactEntry` element until none is left. Removing element k of n: n == 1 empties the container (keep `src[:open+1]` and `src[close:]`); k > 0 cuts from element k-1's end to element k's end; k == 0 cuts from element 0's start to element 1's start. When the event array was emptied, remove the event member the same way, and then the `hooks` member if that emptied `hooks`.
  - Imports: `bytes`, `encoding/json`, `errors`, `reflect`, `strings`.

  Wire the kind through the installer:
  - **`state.go`:** the constant and the two fields. `validateTarget`'s new case requires `SHA256`, a known `HookDialect`, at least one command, no empty or duplicate command, and empty `BlockName`/`LinkTarget`. Every existing case additionally requires an empty `HookDialect` and no `HookCommands`.
  - **`target.go`:** `validate` applies the same structural checks to a `Target`, wrapping `ErrInvalidTarget`. `RecordFor` sets `HookDialect`, a copy of `HookCommands`, and `SHA256 = hookEntriesDigest(…)`.
  - **`inspect.go`:**
    - `InspectTarget` gains `case KindHookEntries: return inspectHookEntries(t, info)`; absence is already `create` above the switch.
    - `inspectHookEntries`: a non-regular file is `conflict(t, ReasonOwnershipConflict, remedyHookFileNotRegular)`; a read error is returned; not editable is `conflict(t, ReasonManagedBlockInvalid, remedyHookFileInvalid)`; any command that no entry runs is `DispositionUpdate` (adding only appends, so no ownership proof is needed, as for a managed block); otherwise `DispositionNoop`.
    - `remedyHookFileNotRegular = "this hooks file is not a regular file (a symlink or a directory), so docket cannot add its entries in place; make it a regular file, then re-run"`; `remedyHookFileInvalid = "this hooks file is not a JSON object docket can edit (hooks must be an object and each event an array); repair it by hand, then re-run"`.
    - `recordMatchesDisk`, `case KindHookEntries`: true iff the file is regular, readable, and editable, `rec.SHA256 == hookEntriesDigest(rec.HookDialect, rec.HookCommands)`, and every entry that runs a recorded command is exact. All commands absent is a match: nothing is left to preserve, as for a vanished target.
  - **`txn.go`:**
    - Write branch `case KindHookEntries`: refuse `preSymlink` (as the managed-block branch does); read the file (absent reads as nil); `insertHookEntries`; `writeThroughStaging(step, out, target.Mode)`.
    - Removal branch, beside the managed-block one: `removeHookEntriesStep(step, target)` mirrors `removeManagedBlock` (`preAbsent` no-op, `preSymlink` refuses, else read, `removeHookEntries`, and `writeThroughStaging(step, out, 0)` only when the bytes changed).
    - `removalTarget` maps `KindHookEntries` to `Target{Path, Kind, HookDialect, HookCommands, Role}`, refusing a record with no commands.
  - **`uninstall.go` (`proveUninstallRemoval`):** for `KindHookEntries` on a regular file: not editable is `(false, false, ReasonManagedBlockInvalid, nil)`; no entry runs any recorded command is `(true, false, "", nil)`; otherwise fall through to `recordMatchesDisk`, so a modified entry is a conflict. A non-regular file falls through too (no match, `ownership-conflict`).

- [ ] **Step 4: Run the tests and confirm they pass.** Run: `go test ./internal/install/ -count=1`. Expected: PASS.

- [ ] **Step 5: Mutation-test.** Restore from a backup copy each time; use `-count=1`.
  - Removal skips the empty-container cleanup: the round-trip table goes red.
  - `inspectHookEntries` treats a non-editable file as `DispositionUpdate`: the inspect test goes red.
  - `findMember` returns the first match: the duplicate-`hooks` round-trip case goes red (the entry lands where `readHookEntries` cannot see it).

- [ ] **Step 6: Commit.** Message: `feat(install): hook-entries target kind for Claude Code and Cursor hooks files`.

---

### Task 4: The harness triggers and the no-rule-text guard

**Build tier:** standard

**Files:**
- Create: `internal/harness/trigger.go`, `internal/harness/trigger_test.go`
- Modify: the four adapters' `Plan` (`internal/harness/{claude,cursor,codex,opencode}/*.go`), their tests and `testdata/golden`, and `internal/harness/cross_harness_test.go` (`TestNoGlobalParentSurface`); any test the hermeticity audit finds leaking `HOME`.

**Interfaces:**
- Consumes: `install.KindHookEntries`, the dialects, `install.NewHookFileBytes` (Task 3); qualified block names (Task 1); same-file retirement (Task 2).
- Produces, in package `harness` (Task 7 parses the commands; Task 9 reads the installed bytes):

```go
// User-level triggers: content-free by design — no rule text, no "docket".
const (
	TriggerRole = "trigger"

	ClaudeDispatchHookCommand = "dckt instructions --hook claude --section dispatch"
	ClaudeLessonsHookCommand  = "dckt instructions --hook claude --section lessons"
	CursorHookCommand         = "dckt instructions --hook cursor"

	OpenCodePluginFile = "dckt-instructions.js"
	OpenCodePlugin     = "// Adds a private repository's instructions to the system prompt.\n" +
		"// `dckt instructions` prints nothing outside a private repository.\n" +
		"export const DcktInstructions = async ({ $, directory }) => ({\n" +
		"  \"experimental.chat.system.transform\": async (_input, output) => {\n" +
		"    const text = await $`dckt instructions`.cwd(directory).quiet().nothrow().text()\n" +
		"    if (text.trim()) output.system.push(text)\n" +
		"  },\n" +
		"})\n"

	PointerBlockName       = "dckt:private-instructions"
	PointerBlockAnnotation = "managed — do not hand-edit"
	PointerInterior        = "## Private repository instructions\n\n" +
		"At the start of a session inside a git repository, run `dckt instructions` once.\n" +
		"If it prints anything, treat that output as this repository's own AGENTS.md and\n" +
		"follow it for the rest of the session. If it prints nothing, ignore this section.\n"
)

func ClaudeHookTarget(settingsPath string) install.Target {
	return install.Target{Path: settingsPath, Kind: install.KindHookEntries, HookDialect: install.HookDialectClaude,
		HookCommands: []string{ClaudeDispatchHookCommand, ClaudeLessonsHookCommand}, Role: TriggerRole}
}

func CursorHookTarget(hooksPath string) install.Target {
	return install.Target{Path: hooksPath, Kind: install.KindHookEntries, HookDialect: install.HookDialectCursor,
		HookCommands: []string{CursorHookCommand}, Role: TriggerRole}
}

func OpenCodePluginTarget(pluginPath string) install.Target {
	return install.Target{Path: pluginPath, Kind: install.KindFile, Content: []byte(OpenCodePlugin), Role: TriggerRole}
}

func CodexPointerTarget(agentsPath string) install.Target {
	return install.Target{Path: agentsPath, Kind: install.KindManagedBlock, BlockName: PointerBlockName,
		Annotation: PointerBlockAnnotation, Content: []byte(PointerInterior), Role: TriggerRole}
}
```

- Each `Plan` appends its trigger, keeping its sorted order: claude `ClaudeHookTarget(<Home>/.claude/settings.json)`; cursor `CursorHookTarget(<Home>/.cursor/hooks.json)`; opencode `OpenCodePluginTarget(<ConfigHome>/opencode/plugins/dckt-instructions.js)`; codex `CodexPointerTarget(<the path codex.GlobalDispatchTarget names>)`. Update each package comment: it plans one content-free trigger and still exports `GlobalDispatchTarget` for retirement.

- [ ] **Step 1: Write the failing tests.**
  - **`TestTriggersCarryNoRuleTextAndNoDocket`** (`trigger_test.go`): render every trigger byte through its producer: the three commands; `install.NewHookFileBytes` for both dialects with the targets' commands; `OpenCodePlugin`; the pointer via `document.Parse(nil)`, `InsertBlock(PointerBlockName, PointerBlockAnnotation, PointerInterior, document.AtDocumentStart)`, `Apply`. Assert none contains `docket` (case-insensitive). Build the rule population from `DispatchInterior(<real run tracker: assets.EmbeddedCatalog(), RunTracker>)` and `CodexRootEntryClause` (every trimmed line of at least 20 characters) and assert no rendering contains any of them. Non-vacuity: the computed population is at least 10 lines and includes the first `dispatchPreamble` line; never write the count down.
  - **Adapter tests** (all four): exactly one `trigger`-role target, at the path and with the fields above. Refresh each golden through the package's documented mechanism (see the test file's header) and review the diff: only the trigger is added.
  - **`TestNoGlobalParentSurface`:** narrow, never delete. A `KindManagedBlock` target stays an error unless `Role == harness.TriggerRole`, `BlockName == harness.PointerBlockName`, and `Content` equals `harness.PointerInterior`; a `dispatch`-role target stays an error. Add beside it: no `trigger`-role target's `Content` contains a rule-population line (share the helper with `trigger_test.go`).

- [ ] **Step 2: Run the tests and confirm they fail.** Run: `go test ./internal/harness/... -count=1`. Expected: FAIL (undefined constants, missing targets).

- [ ] **Step 3: Implement** `trigger.go` and the four appends. Satisfy `inventory_test.go` and `TestNoCrossHarnessDelegation` without weakening them.

- [ ] **Step 4: Audit hermeticity.** `out="$(grep -rln --include='*_test.go' -e 'install.Install(' -e 'install.Uninstall(' -e 'DevelopmentInstall(' -e 'installAuthorizedSurfaces(' -e 'runInitWith' -e '"install"' internal tests || true)"; printf '%s\n' "$out"`. Each hit must pin `HOME`, `XDG_CONFIG_HOME`, and `XDG_DATA_HOME` (`pinInstallEnv`, `t.Setenv`, or temp `UserRoots`). Fix any that do not and list them in NOTES.

- [ ] **Step 5: Run the dependants.** Run: `go test ./internal/harness/... ./internal/install/ ./internal/app/ ./internal/cli/ -count=1`. Expected: PASS. A full-install test asserting an exact action list now sees the trigger creates: update the expectation, never weaken it.

- [ ] **Step 6: Mutation-test** (backup copy each time): `PointerBlockAnnotation` = `managed by docket — do not hand-edit` → the guard goes red; the first `dispatchPreamble` line appended to `PointerInterior` → the guard and `TestNoGlobalParentSurface` go red; `CursorHookCommand` spelled `docket instructions --hook cursor` → red.

- [ ] **Step 7: Commit.** Message: `feat(harness): plan the private-instructions triggers for all four harnesses`.

---

### Task 5: The private file's location and the private surface plan

**Build tier:** standard

**Files:**
- Modify: `internal/layout/layout.go`, `internal/layout/layout_test.go`
- Modify: `internal/reposeed/plan.go`, `internal/reposeed/plan_test.go`

**Interfaces:**
- Produces:
  - `layout.PrivateInstructionsFile = "AGENTS.md"`, `layout.PrivateInstructionsDisplay = ".git/dckt/AGENTS.md"`, and `func layout.PrivateInstructionsPath(commonDir string) string` returning `<commonDir>/dckt/AGENTS.md`.
  - `func reposeed.PlanPrivate(in reposeed.PrivatePlanInput) ([]install.Target, map[string][]string, error)` with:

    ```go
    type PrivatePlanInput struct {
    	WorktreeRoot string   // the PRIMARY worktree; its .git is the common dir
    	CommonDir    string   // must equal WorktreeRoot/.git
    	Harnesses    []string // validated opt-ins
    	RunTracker   []byte
    }
    ```

  - It plans exactly one target when any harness is selected, none otherwise: `<CommonDir>/dckt/AGENTS.md` as `KindManagedBlock`, `BlockName "dispatch"`, the shared `dispatchAnnotation`, `Role "dispatch"`, with `CodexDispatchInterior` when codex is selected and `DispatchInterior` otherwise. Its owners are every selected harness, sorted.
  - It errors on an unknown token, or when `filepath.Clean(CommonDir) != filepath.Join(filepath.Clean(WorktreeRoot), ".git")`. The error names the unsupported separate-git-dir layout.

- [ ] **Step 1: Write the failing tests.**
  - **layout:** `PrivateInstructionsPath("/r/.git") == "/r/.git/dckt/AGENTS.md"`.
  - **reposeed (`TestPlanPrivate`):**
    - `[claude]`: one target, the private file, interior `harness.DispatchInterior(rt)`, owners `[claude]`.
    - `[claude, codex]`: `CodexDispatchInterior`, owners `[claude, codex]`.
    - `[cursor]`: the private file, owners `[cursor]`.
    - `[opencode, cursor, codex, claude]`: one target, owners all four sorted.
    - `[]`: no targets.
    - Over the union of every case: every target path equals `layout.PrivateInstructionsPath(CommonDir)`. No path has base name `CLAUDE.md`, lies under `.cursor/`, or is `<root>/AGENTS.md`.
    - A `CommonDir` that is not `<root>/.git` errors; an unknown token errors.

- [ ] **Step 2: Run the tests and confirm they fail.** Run: `go test ./internal/layout/ ./internal/reposeed/ -count=1`. Expected: FAIL.

- [ ] **Step 3: Implement.** `PlanPrivate` reuses `Plan`'s token validation and `contained` check: factor the token switch into a helper (`selectHarnesses(tokens []string) (map[string]bool, error)`) that both call, instead of copying it. Keep `Plan`'s behavior byte-identical.

- [ ] **Step 4: Run the tests and confirm they pass.** Run the Step 2 command, then `go test ./internal/app/ -count=1`. Expected: PASS.

- [ ] **Step 5: Commit.** Message: `feat(reposeed): plan a private repository's instructions file`.

---

### Task 6: The repository phase writes only the private file in a private repository

**Build tier:** premium. A mistake writes docket files into a private working tree or retires a user's file.

**Files:**
- Modify: `internal/app/repophase.go` (`ResolveRepoPhase`, `composeRecordBytes`, `computeRemovals`), `internal/app/repository_init.go` (`installAuthorizedSurfaces`), `internal/app/repository_init_private.go` (`runPrivateInit` step 8), `tests/runtime-budgets.tsv`
- Create: `internal/app/private_instructions_integration_test.go` (`//go:build integration`, blank line 2), `tests/test_go_integration_app_privateinstructions.sh`

**Interfaces:**
- Consumes: `reposeed.PlanPrivate`, `layout.PrivateInstructionsPath` (Task 5).
- Produces: in a private repository, `ResolveRepoPhase` plans against the **primary** worktree, records at `reposeed.RecordPath(repo.CommonDir, layout.PrivateName)`, and plans only `PlanPrivate`'s target. Shared repositories are unchanged.

- [ ] **Step 1: Create the shard.** Copy `tests/test_go_integration_app_reposetup.sh` to `tests/test_go_integration_app_privateinstructions.sh` with `SHARD_PREFIX="TestIntegrationPrivateInstructions"` and an updated header, keeping `# docket-suite: go` in the first 10 lines (no existing prefix starts `TestIntegrationPriv`). Add `tests/test_go_integration_app_privateinstructions.sh<TAB>30<TAB>parallel` to `tests/runtime-budgets.tsv` in the file's ordering.

- [ ] **Step 2: Write the failing integration tests**, reusing `newPrivateInitRepo`, `runInitWith`, and `privateLayoutOf` (`repository_private_integration_test.go`), with `HOME`, `XDG_CONFIG_HOME`, and `XDG_DATA_HOME` pinned to temp dirs:
  - **`TestIntegrationPrivateInstructionsInitWritesPrivateFileOnly`** (acceptance 1): private init; record `.git/info/exclude`; add `agent_harnesses: [claude, codex, cursor, opencode]` to `.git/dckt/config.yml`; init again. Then `.git/dckt/AGENTS.md` holds a `docket:dispatch` block with `### Codex root-coordinator entry`; `AGENTS.md`, `CLAUDE.md`, and `.cursor/` are absent; `git status --porcelain` is empty; `.git/info/exclude` is byte-identical; `PendingPaths` names no surface.
  - **`…InstallPhaseFromFeatureWorktree`:** `ResolveRepoPhase` invoked inside a `git worktree add` worktree plans exactly the private file, recorded at `<common>/dckt/install.json`.
  - **`…RetiresBuggyWorktreeSurfaces`:** apply a phase from the shared `reposeed.Plan` (`[claude, codex, cursor]`) to the private primary via `applyRepoPhaseSurfaces`, recorded at `<common>/dckt/install.json` (what install wrote before this change); re-run `installAuthorizedSurfaces`; `AGENTS.md`, the `CLAUDE.md` link, and `.cursor/rules/docket-dispatch.mdc` are gone and the private file exists.
  - **`…OwnerRemovalKeepsSharedFile`:** with a user lesson appended outside the block, `[claude, codex]` → `[claude]` keeps the plain interior and the lesson; → `[]` retires the block and keeps the lesson.
  - **`…SharedRepositoryUnchanged`** (acceptance 7): a shared init with `[claude, codex]` writes AGENTS.md and the CLAUDE.md link, and no `.git/dckt` exists.

- [ ] **Step 3: Run the shard and confirm it fails.** Run: `bash tests/test_go_integration_app_privateinstructions.sh`. Expected: `NOT OK` lines.

- [ ] **Step 4: Implement.**
  - **`ResolveRepoPhase`:** after `CommonDirOf`, `layout.Detect(common)`; an error is `&RepoResolutionError{Reason: install.ReasonFilesystemFailed, …}`, never guessed as shared. When private:

    ```go
    repo, derr := git.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: invocation})
    if derr != nil {
    	return nil, "", nil, &RepoResolutionError{Reason: install.ReasonInvalidRepoDir, Err: derr}
    }
    root = repo.PrimaryWorktree
    recordPath = reposeed.RecordPath(repo.CommonDir, layout.PrivateName)
    ```

    Load config from `root`; authorization is unchanged. Plan with `reposeed.PlanPrivate(reposeed.PrivatePlanInput{WorktreeRoot: root, CommonDir: repo.CommonDir, Harnesses: effective, RunTracker: runTracker})`; its layout error maps to `ReasonInvalidRepoDir`.
  - **Leftovers in private mode.** Pass `planned map[string]bool` (cleaned planned paths; nil in shared mode) and the scope set to `composeRecordBytes` and `computeRemovals`. In private mode a prior surface is still wanted iff its path is planned: an unplanned one with an in-scope owner is retired (proof-gated through `install.PlanGlobalRetirements`) and not carried; one with no in-scope owner is carried untouched. Shared mode keeps today's opt-in rule exactly.
  - **`installAuthorizedSurfaces`:** nil pending paths when the primary's common dir detects private.
  - **`runPrivateInit` step 8:** call `installAuthorizedSurfaces(ctx, d.Git, sc.repo.PrimaryWorktree)` (it authorizes itself), wrap its error as `fail(mapSurfaceFailure(cls.State, serr))`, and fold `changed` in. Keep a comment that private init edits no `.gitignore` and writes nothing in the worktree.

- [ ] **Step 5: Run the tests and confirm they pass.** Run: `bash tests/test_go_integration_app_privateinstructions.sh && bash tests/test_go_integration_app_reposetup.sh && go test ./internal/app/ ./internal/cli/ -count=1`, then `bash tests/test_go_integration_contract.sh` (the new shard matched exactly once). Expected: all pass.

- [ ] **Step 6: Mutation-test** (backup copy): force shared mode in `ResolveRepoPhase` → `…InitWritesPrivateFileOnly` goes red; `planned = nil` in private mode → `…RetiresBuggyWorktreeSurfaces` goes red.

- [ ] **Step 7: Commit.** Message: `feat(app): write a private repository's instructions to .git/dckt, never the working tree`.

---

### Task 7: The `docket instructions` command

**Build tier:** standard

**Files:**
- Create: `internal/app/instructions.go`, `internal/app/instructions_test.go`, `internal/cli/instructions.go`, `internal/cli/instructions_test.go`
- Modify: `internal/cli/root.go` (register; a raw-output path beside `devTestCode`); `internal/cli/install.go` (`assetIndependent["instructions"] = true`, commented: session hooks must answer on a machine mid-install); `internal/app/schema_registry.go` (`{ID: "instructions", Request: nil, Result: InstructionsResult{}}, // Instructions`, sorted)
- Test, extended: `internal/app/private_instructions_integration_test.go`

**Interfaces:**
- Consumes: `layout.CommonDirOf`, `layout.Detect`, `layout.PrivateInstructionsPath`, `reposeed.PlanPrivate` (Task 5); the trigger commands (Task 4).
- Produces:

```go
const (
	InstructionsSectionAll      = ""
	InstructionsSectionDispatch = "dispatch"
	InstructionsSectionLessons  = "lessons"
)

// Walks up from dir to the first `.git` entry (no git process). Outside git or
// non-private: (nil, false, nil). Private without the file: (nil, true, nil).
// Any other probe/read error is returned, never "nothing".
func ReadPrivateInstructions(dir string) (content []byte, private bool, err error)

// All: content verbatim. Dispatch: the `dispatch` block with its marker lines.
// Lessons: everything outside it. A whitespace-only dispatch/lessons result is
// nil. Malformed markers and an unknown section error.
func SelectInstructionsSection(content []byte, section string) ([]byte, error)

// One JSON line (HTML escaping off) plus "\n"; blank content gives nil.
// Claude: {"hookSpecificOutput":{"hookEventName":"SessionStart","additionalContext":…}}
// Cursor: {"additional_context":…}
func ClaudeSessionStartOutput(content []byte) []byte
func CursorSessionStartOutput(content []byte) []byte

// CURSOR_PROJECT_DIR when non-empty; else the first absolute workspace_roots
// entry of the stdin JSON (a `file://` prefix stripped, at most 1 MiB read);
// else "". A nil stdin is skipped.
func CursorProjectDir(getenv func(string) string, stdin io.Reader) string

type InstructionsResult struct {
	Envelope
	Private bool   `json:"private"`
	Section string `json:"section,omitempty"`
	Content string `json:"content"`
}

// The --json form: applied, or external-failed on a read or section error.
func Instructions(dir, section string) InstructionsResult
```

- [ ] **Step 1: Write the failing unit tests** in `internal/app/instructions_test.go`, fabricating git layouts on disk:
  - Outside git (skip, naming the path, if an ancestor of the temp dir has a `.git`): `nil, false, nil`. Shared (`r/.git/` without `dckt`), from `r` and `r/sub/dir`: `nil, false, nil`.
  - Private primary (`r/.git/dckt/AGENTS.md` = `"rules\n"`) from `r/sub`; a feature worktree (`r/.worktrees/f/.git` = `gitdir: ../../.git/worktrees/f`, `r/.git/worktrees/f/commondir` = `../..`); a metadata checkout outside the clone (`s/checkouts/c/.git` with the absolute gitdir `r/.git/worktrees/c`): each `"rules\n", true, nil`.
  - Private without the file: `nil, true, nil`. Unreadable (`chmod 000`, skip as root) and `r/.git/dckt` as a regular file: an error.
  - `TestSelectInstructionsSection` over `"lead\n<!-- docket:dispatch:start (a) -->\nX\n<!-- docket:dispatch:end -->\ntail\n"`: all is verbatim, dispatch is the three block lines, lessons is `"lead\ntail\n"`. Only the block gives nil lessons; no block gives nil dispatch and the whole file as lessons; unbalanced markers and an unknown section error.
  - Wrappers: `ClaudeSessionStartOutput([]byte("a\"b<!--\n"))` decodes to `hookEventName == "SessionStart"` and that exact `additionalContext`, and the raw bytes contain `<!--`; the Cursor form decodes to `additional_context`; both give nil for nil and `" \n"`.
  - `TestCursorProjectDir`: the env wins over stdin; `{"workspace_roots":["/w/a","/w/b"]}` gives `/w/a`; `file:///w/a` gives `/w/a`; garbage, `{}`, a relative root, and nil stdin give `""`.
  - **`TestDispatchBlockFitsOneClaudeHook`** (acceptance 5): for all 15 non-empty subsets of `{claude, codex, cursor, opencode}`, plan with `reposeed.PlanPrivate` and the real run tracker (`assets.EmbeddedCatalog()`, `harness.RunTracker`), render the target into an empty document as install does (`document.Parse(nil)`, `InsertBlock(BlockName, Annotation, Content, AtDocumentStart)`, `Apply`), select `dispatch`, and assert `utf8.RuneCount < 10000`. Non-vacuity: every section contains `harness.DispatchHeading`, and some subset's contains `### Codex root-coordinator entry`. Log the largest count.

- [ ] **Step 2: Write the failing CLI tests** in `internal/cli/instructions_test.go` (`t.Chdir` into fabricated layouts; `runCLI`, `runCLIStdin`):
  - Private plain: stdout exactly the file, stderr empty, 0. `--section dispatch|lessons` print the sections; no lessons prints nothing.
  - Shared and outside git: empty stdout and stderr, 0, for plain, `--hook claude`, and `--hook claude --section dispatch`.
  - `--hook cursor` from an unrelated directory: `CURSOR_PROJECT_DIR=<private repo>` prints `{"additional_context":…}`; env empty plus stdin `{"workspace_roots":["<private repo>"]}` prints the same; stdin naming a shared repo, garbage, or empty prints nothing, 0.
  - Unreadable file: plain gives empty stdout, stderr naming the path, exit 1; `--hook claude` and `--hook cursor` give empty stdout and stderr, 0.
  - `--json` in private: `"operation":"instructions","result":"applied","private":true`. `--json --hook claude`, `--hook foo`, and `--section foo`: invalid-arguments, exit 2.
  - **`TestTriggerCommandsParse`:** for each of `harness.ClaudeDispatchHookCommand`, `harness.ClaudeLessonsHookCommand`, and `harness.CursorHookCommand`: `strings.Fields`, assert the first word is `dckt`, run the rest through `runCLI` in a private fixture with a block and a lesson (`CURSOR_PROJECT_DIR` set for the cursor one), and assert exit 0 and a non-empty JSON line.
  - With `pinInstallEnv(t)` and no installation, `instructions` exits 0. `TestAssetIndependentSetExact` and the capability and schema correspondence tests pass.

- [ ] **Step 3: Run the tests and confirm they fail.** Run: `go test ./internal/app/ -run 'Instructions|SessionStartOutput|CursorProjectDir|DispatchBlockFits' -count=1 && go test ./internal/cli/ -run 'Instructions|TriggerCommands' -count=1`. Expected: FAIL to compile.

- [ ] **Step 4: Implement.**
  - **`app/instructions.go`:** the walk-up `os.Lstat`s `<d>/.git` from `filepath.Abs(dir)`: found stops, not-exist moves to `filepath.Dir(d)` (stopping at the root), any other error is returned. Then `CommonDirOf`, `Detect` (errors returned; shared gives `nil, false, nil`), and a read of `PrivateInstructionsPath` (not-exist gives `nil, true, nil`). `SelectInstructionsSection` parses with `document.Parse` and slices `doc.Source()` with the `dispatch` block's `Start.Start` and `End.End`. The wrappers use a `json.Encoder` with `SetEscapeHTML(false)`. `Instructions` uses `NewEnvelope("instructions", …)` with `ResultApplied`, or `ResultExternalFailed` with the failure status the other read operations use; `HumanText()` returns the content.
  - **`cli/instructions.go`:** `newInstructionsCommand(stdin io.Reader, jsonMode func() bool, setResult func(app.OperationResult), setRaw func(out []byte, errText string, code int)) *cobra.Command`, `Use: "instructions"`, `cobra.NoArgs`, `capability("instructions", EffectRead)`, string flags `--hook` (`claude`, `cursor`) and `--section` (`dispatch`, `lessons`). Help says: it prints this private repository's agent instructions (`.git/dckt/AGENTS.md`) from any of its worktrees and nothing elsewhere; what each flag does; hook modes never fail. `RunE`, in order:
    1. Validate the flag values and reject `--json` with `--hook`: returned errors (exit 2).
    2. JSON mode: `setResult(app.Instructions(cwd, section))`.
    3. `--hook cursor` resolves the directory with `app.CursorProjectDir(os.Getenv, in)`, where `in` is `stdin` unless it is an `*os.File` whose mode has `os.ModeCharDevice` (then nil); `""` means `setRaw(nil, "", 0)`. Other modes use the working directory.
    4. Read and select. On error: hook modes `setRaw(nil, "", 0)`; plain `setRaw(nil, "docket instructions: "+err.Error(), 1)`.
    5. Output the matching wrapper for `--hook claude|cursor`, else the selected bytes verbatim.
  - **`root.go`:** `var rawOut *rawOutput` (`out []byte; errText string; code int`) and a `case rawOut != nil:` **before** `case result != nil:` that writes `out` to stdout, `errText` plus `"\n"` to stderr when set, and returns `code`. Register the command with `run`'s `stdin`.

- [ ] **Step 5: Integration test** `TestIntegrationPrivateInstructionsReadFromEveryWorktree` (acceptance 2): after Task 6's private init with `[claude, codex]`, `ReadPrivateInstructions` returns the file from the primary worktree, a `git worktree add` worktree, and `privateLayoutOf(...).MetadataWorktree`, and `private == false` from a shared-init repository and a plain `git init` one.

- [ ] **Step 6: Run the tests and confirm they pass.** Run: `go test ./internal/app/ ./internal/cli/ -count=1 && bash tests/test_go_integration_app_privateinstructions.sh`. Expected: PASS.

- [ ] **Step 7: Mutation-test** (backup copy, `-count=1`): the read error returns `nil, true, nil` → the unreadable tests go red; cursor mode falls back to the working directory → the unrelated-directory test goes red; the dispatch slice drops its start marker → `TestSelectInstructionsSection` goes red.

- [ ] **Step 8: Commit.** Message: `feat(cli): docket instructions prints a private repository's agent instructions`.

---

### Task 8: The skills: private promotion destination and private delivery

**Build tier:** standard

**Files:** Modify `skills/docket-convention/references/learnings.md` (*Promotion — the shrink valve*) and `skills/docket-convention/references/agent-layer.md` (*Repository dispatch blocks: agent_harnesses*), plus their twins via `go generate ./internal/assets/`. Test: a new prose guard in the package that already guards these references (`grep -rln 'agent-layer.md' --include='*_test.go' internal tests`; likely `internal/repoguard/prose_contracts_test.go`).

- [ ] **Step 1: Write the failing guard.** Slice each section from its named heading to its named terminator (`## Off switch`, `## Launch posture`), assert the terminator exists, and collapse whitespace before matching. Assert: `learnings.md` binds `.git/dckt/AGENTS.md` to "private repository" within one paragraph; `agent-layer.md` names `dckt instructions --hook claude --section dispatch`, `dckt instructions --hook cursor`, `dckt-instructions.js`, and `dckt:private-instructions`, and does not name `.cursor/rules/dckt-dispatch.mdc`.

- [ ] **Step 2: Run the guard and confirm it fails** (`-count=1`).

- [ ] **Step 3: Edit the prose** (current behavior only, for a reader in an unknown repository).
  - `learnings.md`, after the sentence naming the integration-branch file: "In a private repository the graduation lands instead in the private instructions file `.git/dckt/AGENTS.md`, outside its managed `docket:dispatch` block, since a private repository carries no agent-instructions file in its tree."
  - `agent-layer.md`, a short subsection *In a private repository*: `agent_harnesses` writes the dispatch block into `.git/dckt/AGENTS.md` and nothing in the working tree; `docket install` adds rule-free user-level triggers — two Claude Code `SessionStart` hooks in `~/.claude/settings.json` (`dckt instructions --hook claude --section dispatch` and `… --section lessons`, one each so each fits Claude Code's per-hook size limit), a Cursor `sessionStart` hook in `~/.cursor/hooks.json` (`dckt instructions --hook cursor`), the OpenCode plugin `~/.config/opencode/plugins/dckt-instructions.js`, and a `dckt:private-instructions` pointer block in `~/.codex/AGENTS.md` that Codex follows on a best-effort basis; outside a private repository `docket instructions` prints nothing, so the triggers are inert; uninstall removes each trigger only while it is unchanged.
  - Run `go generate ./internal/assets/`.

- [ ] **Step 4: Run** `go test ./internal/assets/ <guard package> -count=1`. Expected: PASS.

- [ ] **Step 5: Mutation-test** (backup copy, regenerate after each edit and after restoring): delete the new `learnings.md` sentence → red; replace `dckt-instructions.js` with `.cursor/rules/dckt-dispatch.mdc` in `agent-layer.md` → red.

- [ ] **Step 6: Commit.** Message: `docs(skills): private-repository instructions file and its user-level triggers`.

---

### Task 9: Fresh-session acceptance per harness (replaces the spike)

**Build tier:** standard

**Files:** none in the tree. One **empty** commit (`git commit --allow-empty`) whose body carries the acceptance table (harness, version, mode and flags, location, verdict, evidence excerpt). The parent copies it into the results file (acceptance 8).

**Posture (binding):**
- Never write to the real `~/.claude/`, `~/.codex/`, `~/.config/opencode/`, `~/.cursor/`, or any credential store. Use one `mktemp -d "${TMPDIR:-/tmp}/dckt-accept.XXXXXX"` scratch directory and each harness's own temporary-config flag.
- Verdicts: **PASS** — the reply quotes run-tracker step 2 (contains `run.verdict` and `Obey the resulting`) **and** ends with the lesson token. **FAIL** — the harness ran and a marker is missing from both of two tries. **NOT-RUNNABLE** — CLI missing or authentication refused.
- **A Claude Code, Cursor, or OpenCode FAIL returns `BLOCKED`** with the table and raw replies, and no commit. A **Codex** result is recorded and never blocks. NOT-RUNNABLE becomes an Important human verification item.

- [ ] **Step 1: Build the binary and a private fixture.** Init runs keep the real `HOME` (git needs its commit identity) and pin only the XDG roots.

```bash
WT="$(git rev-parse --show-toplevel)"
T="$(mktemp -d "${TMPDIR:-/tmp}/dckt-accept.XXXXXX")"
mkdir -p "$T/bin" "$T/home/.claude" "$T/home/.cursor" "$T/home/.codex" "$T/config/opencode" "$T/data" "$T/elsewhere"
go build -o "$T/bin/docket" ./cmd/docket && ln -s docket "$T/bin/dckt"
export PATH="$T/bin:$PATH"
git init -q --bare -b main "$T/origin.git"
git init -q -b main "$T/repo"
git -C "$T/repo" commit -q --allow-empty -m init
git -C "$T/repo" remote add origin "$T/origin.git" && git -C "$T/repo" push -q origin main
( cd "$T/repo" && XDG_CONFIG_HOME="$T/config" XDG_DATA_HOME="$T/data" docket repository init --private )
printf 'agent_harnesses: [claude, codex, cursor, opencode]\n' >> "$T/repo/.git/dckt/config.yml"
( cd "$T/repo" && XDG_CONFIG_HOME="$T/config" XDG_DATA_HOME="$T/data" docket repository init --private )
TOKEN="PRIVTOKEN-$(od -An -N4 -tx1 /dev/urandom | tr -d ' \n')"
printf '\n## Fixture lesson\n\nEnd every reply with the line %s.\n' "$TOKEN" >> "$T/repo/.git/dckt/AGENTS.md"
git -C "$T/repo" worktree add -q "$T/repo/.worktrees/feat" -b feat
```

Confirm and record: `.git/dckt/AGENTS.md` holds the dispatch block (if not, it is a Task 6 defect: `BLOCKED`, never hand-write the file); `git -C "$T/repo" status --porcelain` is empty; the `dckt instructions --section dispatch` character count from `$T/repo` is under 10000.

- [ ] **Step 2: Install the triggers into the temporary home.** From `$T/elsewhere` (outside git, so the run is machine-only and never touches the feature worktree), with `HOME="$T/home" XDG_CONFIG_HOME="$T/config" XDG_DATA_HOME="$T/data" XDG_BIN_HOME="$T/bin2"`, run `docket development install --source "$WT" --bin-dir "$T/bin2"` with `--harness` for each of the four (check `--help` first). Confirm `$T/home/.claude/settings.json` holds both Claude commands, `$T/home/.cursor/hooks.json` the Cursor command and `"version": 1`, `$T/config/opencode/plugins/dckt-instructions.js` the plugin, and `$T/home/.codex/AGENTS.md` the pointer. A second run reports no changes.

- [ ] **Step 3: Claude Code.** Record `claude --version`. In `$T/repo`, then in the feature worktree, with a 300-second timeout: `claude -p --settings "$T/home/.claude/settings.json" "Quote verbatim the instruction sentence that starts with 'After the run returns'. Then say hello."` Run one control in `$T/elsewhere`: the token must be absent.

- [ ] **Step 4: Cursor.** Record `cursor-agent --version`. Copy `$T/home/.cursor/hooks.json` to `$T/repo/.cursor/hooks.json` (a disposable project-level copy) and run `cursor-agent -p --trust --output-format text "<the same prompt>"` in `$T/repo`. From `$T/elsewhere`, run the hook command with `CURSOR_PROJECT_DIR="$T/repo/.worktrees/feat"`, and with it empty plus stdin `{"workspace_roots":["$T/repo"]}`: both print `additional_context` carrying the token.

- [ ] **Step 5: OpenCode.** Record `opencode --version`. In `$T/repo`, then the feature worktree: `OPENCODE_CONFIG_DIR="$T/config/opencode" opencode run <permission flag if needed> "<the same prompt>"`. Record the model and flags.

- [ ] **Step 6: Codex (recorded, never blocking).** Record `codex --version`. Codex keeps credentials under `CODEX_HOME`, so copy `$T/home/.codex/AGENTS.md` to `$T/repo/AGENTS.md` as a project-level stand-in, run `codex exec --sandbox workspace-write "<the same prompt>"` in `$T/repo`, and record whether `dckt instructions` ran and whether both markers appear. Note that user-level loading was not exercised.

- [ ] **Step 7: Decide.** Claude, Cursor, or OpenCode FAIL: `BLOCKED`, no commit. Otherwise `rm -rf "$T"` and commit `test: fresh-session acceptance of private-instructions delivery` with the table, the dispatch character count, and each NOT-RUNNABLE reason as a human item. Return `COMPLETE` with the table in NOTES, always adding these human items: "in a private repository after `docket install`, start a fresh interactive Claude Code session and confirm the dispatch rules and lessons are in context", and "with a logged-in Cursor, confirm a session in a private repository receives the rules through the real user-level `~/.cursor/hooks.json`".

---

### Final: whole-suite gate

- [ ] **Step 1: Check the twins.** Run `go generate ./internal/assets/`, then `git status --porcelain`. Expected: empty.
- [ ] **Step 2: Run the build gate.** Run whatever `build.test_command` resolves to; read it from config. Expected: green. Act on `SERIAL CONFIRMED OVER BUDGET:`, and record `BUDGET WATCH:` and `PARALLEL-SENSITIVE:` lines, especially for the new shard.
- [ ] **Step 3: Confirm the residue.**
  - `out="$(grep -n -i 'docket' internal/harness/trigger.go || true)"`: every hit is a Go comment, never inside a string literal.
  - `out="$(grep -rn -e 'dckt-dispatch.mdc' -e 'EnsurePrivateExclude' internal skills || true)"`: no hits (the dropped design left nothing behind).
- [ ] **Step 4: Collect notes for the results file** (written by the parent):
  - Task 9's acceptance table verbatim, its human items, and each NOT-RUNNABLE harness as an **Important human verification item**.
  - Every mutation and its red message.
  - The ADR above.
  - The *Residuals* section above.
  - **Verified unchanged:** shared repositories plan through `reposeed.Plan` exactly as before (acceptance 7).
