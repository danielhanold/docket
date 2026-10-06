<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0535 — Load a private repository's agent instructions without repository files](../../changes/active/0535-load-a-private-repository-s-agent-instructions-without-repos.md)**
<!-- docket:backlink:end -->

# Load a Private Repository's Agent Instructions Without Repository Files: Implementation Plan

> **For agentic workers:** `docket-build` executes this plan. It routes each task to a tier agent running the `docket-build-task` contract. Each task carries its own focused test cycle and ends in one commit. The whole suite runs once at the end. Steps use checkbox (`- [ ]`) syntax.

**Goal:** A private repository's parent-facing rules (the dispatch block plus promoted lessons) live in `<git-common-dir>/dckt/AGENTS.md`. A new read-only `docket instructions` command prints that file, and only that file, inside a private repository. `docket install` writes one content-free trigger per harness so every session loads it:
- a Claude Code `SessionStart` hook;
- a `dckt:` pointer block in the Codex and OpenCode user-level AGENTS.md;
- an excluded `.cursor/rules/dckt-dispatch.mdc` for Cursor.

No AGENTS.md, CLAUDE.md, or docket-named rule file is written into a private working tree.

**Architecture:**
- `internal/document` learns a second, neutral marker namespace. `<!-- dckt:NAME:start … -->` blocks are addressed by the qualified name `dckt:NAME`. Bare names keep meaning `docket:`.
- `internal/install` gains a fourth target kind, `settings-hook`: one exact matcher group inside `~/.claude/settings.json`'s `hooks.SessionStart` array. It is inserted and removed byte-precisely, and every other byte of the file is preserved. The installer also learns to retire one managed block and write a different one in the same file within one run.
- `internal/harness/trigger.go` holds the trigger constants. The Claude, Codex, and OpenCode adapters plan the triggers.
- `internal/reposeed.PlanPrivate` plans a private repository's surfaces. `app.ResolveRepoPhase` branches on the detected mode, so both `docket install`'s repository phase and `repository init --private` write the private file, the excluded Cursor rule, and nothing else.
- `app.ReadPrivateInstructions` and the `instructions` command resolve the common directory from the filesystem alone (no git process), so they cost almost nothing in every session.

**Tech Stack:**
- Go.
- cobra.
- Real-git tests behind `//go:build integration`, sharded by name prefix. This plan adds the shard `TestIntegrationPrivateInstructions`.
- `go generate ./internal/assets/` for the embedded skill twins.

**Spec:** `docs/superpowers/specs/2026-10-06-load-a-private-repository-s-agent-instructions-without-repos-design.md`. It is already in the feature worktree. Read it first.

## Global Constraints

- **The private file** is `<git-common-dir>/dckt/AGENTS.md`. It holds the managed `docket:dispatch` block, with the same interior the repository-level block carries: `harness.DispatchInterior`, or `harness.CodexDispatchInterior` when `codex` is opted in. Promoted lessons are added to it by hand, outside the block.
- **Nothing in a private worktree.** No AGENTS.md, CLAUDE.md, or `.cursor/rules/docket-dispatch.mdc`. Cursor gets `.cursor/rules/dckt-dispatch.mdc`, listed in the `# dckt:` block of `.git/info/exclude`.
- **`docket instructions`:**
  - In a private repository, it prints the private file verbatim from any worktree: primary, feature, or metadata checkout.
  - Everywhere else (a shared repository, a non-docket repository, outside git) it prints **nothing** and exits 0.
  - `--hook` wraps the content in Claude Code's `SessionStart` output, `{"hookSpecificOutput":{"hookEventName":"SessionStart","additionalContext":…}}`, and also prints nothing outside private repositories.
- **The triggers, verbatim:**
  - Claude: the hook command is exactly `dckt instructions --hook`.
  - Codex and OpenCode: the pointer block's qualified name is `dckt:private-instructions`, its annotation is `managed — do not hand-edit`, and its interior is exactly:
    ```
    ## Private repository instructions

    At the start of a session inside a git repository, run `dckt instructions` once.
    If it prints anything, treat that output as this repository's own AGENTS.md and
    follow it for the rest of the session. If it prints nothing, ignore this section.
    ```
- **No rule text and no "docket" in any trigger.** This is pinned by a test (Task 5).
- **Hook safety:**
  - The entry is identified by its exact command.
  - Install adds the entry only when the command is absent, and every other byte of `settings.json` survives.
  - Uninstall removes only the exact canonical group, and reports, rather than touches, a modified one.
- **Shared repositories are unchanged:** their repository-level blocks stay as they are, and they get no private file.
- **The triggers are installed whenever their harness is installed**, including on machines with no private repository yet.
- **Docs:** command help and skills only. No `docs/` pages.
- **AGENTS.md rules:**
  - Mutation-test every guard, restoring from a **backup copy** (never `git checkout --`), and run with `-count=1`.
  - Template every `mktemp` as `"${TMPDIR:-/tmp}/<name>.XXXXXX"`.
  - Anchor cross-references on symbols.
  - Never pipe into `grep -q` or `head`.
  - Run `go generate ./internal/assets/` after any skill edit.
- **Hermetic home.** Every test that reaches the installer or `installAuthorizedSurfaces` pins `HOME`, `XDG_CONFIG_HOME`, and `XDG_DATA_HOME` to temp directories. This change adds a write to `~/.claude/settings.json`, so a leak now overwrites real settings.

## Review Focus

Each item has a test in its owning task.
1. **`settings.json` content docket did not write** (a user's own `SessionStart` group with a matcher, another event, unrelated keys, tab indentation, or a one-line compact file): install inserts exactly one group, and an install-then-uninstall round trip returns the original bytes (Task 4).
2. **A malformed or non-object `settings.json`**, or `hooks` / `SessionStart` of the wrong JSON type: install refuses with `managed-block-invalid` and the file is untouched. It is never read as "hook absent" (Task 4).
3. **A user-edited docket group** (for example, a `timeout` added): install is a no-op, and uninstall reports a conflict and leaves the group in place (Task 4).
4. **A `~/.codex/AGENTS.md` that still carries a provable leftover `docket:dispatch` block:** one install retires it and adds the pointer, user text survives, and a failed later step rolls the file back byte-for-byte (Task 3).
5. **`docket instructions` run from a subdirectory**, from a feature worktree whose `.git` is a file, and with an unreadable private file: it prints from the right file, and the unreadable case gives a stderr diagnostic and exit 1, never silence (Task 8).

## Learnings applied

- **harness-behavior-is-mode-and-version-scoped:** each spike verdict carries the harness version, the mode (headless), and the flags.
- **external-truth-needs-a-human-checkpoint:** a harness the spike cannot exercise, and delivery in an interactive session, become named human items phrased as states to reproduce.
- **generated-artifact-loaded-at-process-start:** triggers load at session start. The spike uses fresh processes only.
- **config-layer-write-and-read-hazards:** the new `~/.claude/settings.json` write makes any test that leaks `HOME` a data-loss bug. Task 5 audits them.
- **probe-error-is-not-clean-absence:** an unreadable private file, a layout probe error, or an unparseable `settings.json` is reported, never treated as absent.
- **shared-resource-keeps-first-owner-assumptions:** Task 3 has a two-block fixture for `~/.codex/AGENTS.md`. Task 7 has an owner-removal fixture for the co-owned private file.
- **byte-pattern-guard-matches-a-spelling** and **assert-detects-removal-not-replacement:** the no-docket guard is a case-insensitive substring match over every byte a trigger writes, and it is mutation-tested.
- **distributed-body-has-no-local-repo:** Task 9's prose is read in unknown repositories.
- **intermediate-task-state-buildable:** every task leaves the tree green.

## ADRs to record

The parent records this through `docket-adr`; it is not a build task.

1. **A private repository's parent-facing rules live in `<git-common-dir>/dckt/AGENTS.md`.** They reach the agent through content-free user-level triggers: a Claude `SessionStart` hook running `dckt instructions --hook`, `dckt:` pointer blocks in the Codex and OpenCode user-level AGENTS.md, and an excluded per-repository Cursor rule. This narrowly revisits 0351's retirement of user-level parent-facing writes. It is acceptable because the triggers carry no rules, so the drift that retired the user-level copy cannot recur. Relates to ADR-0036 and ADR-0078.

---

### Task 1: Spike: confirm each harness delivers the instructions in a fresh session

**Build tier:** standard

**Files:** none in the tree. This task prescribes one **empty** commit (`git commit --allow-empty`) whose message body carries the spike table.

**Interfaces:**
- Produces: the spike table (harness, version, mode and flags, verdict, evidence excerpt). The parent copies it into the results file. Task 8's `--hook` output shape is the one confirmed here.

**Posture (binding):**
- Never write to the real `~/.claude/`, `~/.codex/`, `~/.config/opencode/`, or `~/.cursor/`, or to any credential store. Use only a `mktemp -d "${TMPDIR:-/tmp}/dckt-spike.XXXXXX"` scratch directory.
- Each harness gets one verdict:
  - **PASS:** the token appears in the reply.
  - **FAIL:** the harness ran and the token is missing from **both** of two tries.
  - **NOT-EXERCISABLE:** the CLI is missing, authentication is refused, or the probe would need the real home changed.
- Any FAIL means return `BLOCKED` with the table and the raw replies, and do not commit (the spec says stop and report).
- NOT-EXERCISABLE is recorded and becomes an Important human verification item. It is not a halt.

- [ ] **Step 1: Build the fixture.**

```bash
T="$(mktemp -d "${TMPDIR:-/tmp}/dckt-spike.XXXXXX")"
TOKEN="PRIVTOKEN-$(od -An -N4 -tx1 /dev/urandom | tr -d ' \n')"
git init -q -b main "$T/repo"
mkdir -p "$T/repo/.git/dckt" "$T/bin"
# Realistic payload size: this worktree's CLAUDE.md (the real dispatch block plus
# lessons), with the token rule as the LAST line, so truncation would drop it.
cp "<feature-worktree>/CLAUDE.md" "$T/repo/.git/dckt/AGENTS.md"
printf '\n## Spike rule\n\nEnd every reply with the line %s.\n' "$TOKEN" >> "$T/repo/.git/dckt/AGENTS.md"
wc -c "$T/repo/.git/dckt/AGENTS.md"   # record the byte count
```

Write the stand-in `"$T/bin/dckt"`, which mimics the future `docket instructions`, and `chmod +x` it:

```bash
#!/usr/bin/env bash
set -euo pipefail
f="$(git rev-parse --git-common-dir 2>/dev/null)/dckt/AGENTS.md" || exit 0
[ -f "$f" ] || exit 0
if [ "${1:-}" = "instructions" ] && [ "${2:-}" = "--hook" ]; then
  python3 -c 'import json,sys; print(json.dumps({"hookSpecificOutput":{"hookEventName":"SessionStart","additionalContext":open(sys.argv[1]).read()}}))' "$f"
elif [ "${1:-}" = "instructions" ]; then
  cat "$f"
fi
```

- [ ] **Step 2: Claude Code.** Record `claude --version`. Write `"$T/claude-settings.json"`:

```json
{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"<T>/bin/dckt instructions --hook"}]}]}}
```

Then run, with a 300-second timeout:

```bash
cd "$T/repo" && claude -p --settings "$T/claude-settings.json" "Say hello in one word."
```

`--settings` adds a settings source for this process only; the real `~/.claude/settings.json` is untouched. If the token is missing:
- Re-run once with a stand-in that prints the **plain** file (no JSON). This separates "the shape is wrong" from "headless runs no `SessionStart` hooks".
- Record both outcomes.

Also run a control without `--settings`. The token must be absent there. If it is present, the probe proves nothing, so investigate.

- [ ] **Step 3: Codex.** Record `codex --version`.
- Write the **exact** pointer interior from Global Constraints, inside its markers, into `"$T/repo/AGENTS.md"`. The project-level file stands in for `~/.codex/AGENTS.md`, because Codex keeps its credentials under `CODEX_HOME`, so a temp home would lose them.
- Run: `cd "$T/repo" && PATH="$T/bin:$PATH" codex exec --sandbox workspace-write "Say hello in one word."`
- Record whether the transcript shows `dckt instructions` running, and whether the token appears.
- Note in the table that user-level AGENTS.md loading itself was not exercised.

- [ ] **Step 4: OpenCode.** Record `opencode --version`.
- Use the same project-level pointer.
- Read `opencode run --help` for the non-interactive permission flag that lets a shell command run. Record the flag you used.
- Run: `cd "$T/repo" && PATH="$T/bin:$PATH" opencode run <permission flag if needed> "Say hello in one word."`

- [ ] **Step 5: Cursor.** Record `cursor-agent --version`.
- Remove `"$T/repo/AGENTS.md"`.
- Write `"$T/repo/.cursor/rules/dckt-dispatch.mdc"` with `alwaysApply: true` frontmatter and a body that is the AGENTS.md payload.
- Add `.cursor/rules/dckt-dispatch.mdc` to `"$T/repo/.git/info/exclude"`, then confirm `git -C "$T/repo" status --porcelain` is empty.
- Run: `cd "$T/repo" && cursor-agent -p --trust --output-format text "Say hello in one word."`
- The question this answers is whether Cursor reads a rule file that git ignores.

- [ ] **Step 6: Decide.**
  - Any FAIL: return `BLOCKED` and do not commit.
  - Otherwise, `rm -rf "$T"` and make the empty commit `test: spike private-instructions delivery per harness`. The body holds the table (one row per harness), the payload byte count, the hook shape that worked (JSON or plain), and each NOT-EXERCISABLE reason as a suggested human item phrased as a state to reproduce.
  - Return `COMPLETE` with the table in NOTES.
  - If only plain text worked for Claude, say so: Task 8 then emits plain text for `--hook`.

---

### Task 2: The neutral `dckt:` marker namespace in `internal/document`

**Build tier:** standard

**Files:**
- Modify: `internal/document/markers.go`
- Modify: `internal/document/document.go` (`Block`'s doc comment only)
- Test: `internal/document/markers_test.go`

**Interfaces:**
- Produces:
  - A block whose markers are `<!-- dckt:NAME:start (…) -->` / `<!-- dckt:NAME:end -->` is located, patched, inserted, and removed under the **qualified name** `"dckt:NAME"`. `docket:` blocks keep their bare names.
  - `document.MarkerSpelling(name string) string` returns `"docket:dispatch"` for `"dispatch"` and `"dckt:private-instructions"` for `"dckt:private-instructions"`.
  - `const document.NeutralMarkerPrefix = "dckt:"`.
  - All existing APIs (`Block`, `PatchSet.ReplaceBlock/InsertBlock/RemoveBlock`) accept a qualified name.

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

Add these four tests beside it:
- **`TestNeutralMarkerInsertRendersDcktPrefix`:** `InsertBlock("dckt:private-instructions", "managed — do not hand-edit", "x", AtDocumentStart)` on `"keep\n"` renders `<!-- dckt:private-instructions:start (managed — do not hand-edit) -->\nx\n<!-- dckt:private-instructions:end -->\n`, and the output contains no `docket`.
- **`TestMalformedNeutralMarkerRefuses`:** `Parse("<!-- dckt:Bad Name:start -->\n")` errors.
- **`TestSameNameInBothNamespacesIsTwoBlocks`:** a `docket:x` pair followed by a `dckt:x` pair parses without error.
- **`TestMarkerSpelling`:** `"dispatch"` gives `"docket:dispatch"`, and `"dckt:private-instructions"` is returned unchanged.

- [ ] **Step 2: Run the tests and confirm they fail.**
  - Run: `go test ./internal/document/ -run 'Neutral|SameNameInBoth|MarkerSpelling' -count=1`
  - Expected: FAIL, because `MarkerSpelling` is undefined and the `dckt:` lines are prose today.

- [ ] **Step 3: Implement** in `markers.go`:

```go
// NeutralMarkerPrefix qualifies a block in the neutral `dckt:` marker namespace,
// used where nothing docket-named may appear (a private repository's user-level
// triggers). A bare block name is in the `docket:` namespace.
const NeutralMarkerPrefix = "dckt:"

markerPrefixRE = regexp.MustCompile(`^<!-- (?:docket|dckt):`)
markerRE = regexp.MustCompile(
	`^<!-- (docket|dckt):([a-z][a-z0-9-]*):(start|end)(?: \(([^)]*)\))? -->$`)

var blockNameRE = regexp.MustCompile(`^(?:dckt:)?[a-z][a-z0-9-]*$`)

// splitBlockName returns a block name's marker namespace and its bare name.
func splitBlockName(name string) (namespace, bare string) {
	if rest, ok := strings.CutPrefix(name, NeutralMarkerPrefix); ok {
		return "dckt", rest
	}
	return "docket", name
}

// MarkerSpelling is how a block name appears inside its marker lines, for
// diagnostics: "docket:dispatch", "dckt:private-instructions".
func MarkerSpelling(name string) string {
	ns, bare := splitBlockName(name)
	return ns + ":" + bare
}

func startMarkerLine(name, annotation string) string {
	ns, bare := splitBlockName(name)
	if annotation == "" {
		return "<!-- " + ns + ":" + bare + ":start -->"
	}
	return "<!-- " + ns + ":" + bare + ":start (" + annotation + ") -->"
}

func endMarkerLine(name string) string {
	ns, bare := splitBlockName(name)
	return "<!-- " + ns + ":" + bare + ":end -->"
}
```

In `scanMarkers`, the submatch indexes shift by one. Build the name as `m[2]` for `docket` and `NeutralMarkerPrefix + m[2]` for `dckt`. Kind is `m[3]` and annotation is `m[4]`. The `Msg` "line opens as a docket marker…" may stay. Pairing, duplicate detection, and `Block(name)` then work on the qualified name unchanged. Add `"strings"` to the imports.

- [ ] **Step 4: Run the package and confirm it passes.** Run: `go test ./internal/document/ -count=1`. Expected: PASS, including the fuzz seeds.

- [ ] **Step 5: Run the dependants.** Run: `go test ./internal/install/ ./internal/render/ ./internal/app/ -count=1`. Expected: PASS. A `<!-- dckt:` line was prose before; nothing in the tree depends on that (`grep -rn '<!-- dckt:' --include='*' .` finds only the spec's fenced example).

- [ ] **Step 6: Commit.** Message: `feat(document): neutral dckt: marker namespace for managed blocks`.

---

### Task 3: The installer writes one block and retires another in the same file

**Build tier:** premium. A wrong step order corrupts a user's instruction file or its rollback.

**Files:**
- Modify: `internal/install/inspect.go` (the remedy helpers)
- Modify: `internal/install/txn.go` (`rejectDuplicateDestinations`, and the step ordering in the function that builds `ordered`)
- Test: `internal/install/txn_test.go`, `internal/install/retire_test.go`

**Interfaces:**
- Consumes: `document.MarkerSpelling` (Task 2).
- Produces: one transaction may carry a managed-block **removal** and a managed-block **write** of a different block name in one file. The removal is applied first.

- [ ] **Step 1: Write the failing test** `TestInstallRetiresLeftoverBlockAndWritesPointerInOneFile` in `retire_test.go`, using the existing test planners, temp roots, and a `RealFS`:
  - **Arrange:** `<home>/.codex/AGENTS.md` = `"# mine\n\n"` plus a `docket:dispatch` block whose interior the prior `State` records (so the retirement is provable), plus `"tail\n"`.
  - **Planner `codex`:** `Plan` returns one target: `Kind: KindManagedBlock`, `Path: <home>/.codex/AGENTS.md`, `BlockName: "dckt:private-instructions"`, `Annotation: "managed — do not hand-edit"`, `Content: []byte("pointer")`. `GlobalDispatchTarget` returns that file's `dispatch` block.
  - **Assert after `Install`:** success; the file has no `docket:dispatch` block and has exactly one `dckt:private-instructions` block with interior `pointer`; `# mine` and `tail` survive; the actions include one `remove` and one `update` for that path.
  - **Second case, `...RollsBackBothSteps`:** inject an FS failure on a step that sorts **after** that path (add a second planned file target at a later path whose staging write fails). Assert the AGENTS.md bytes equal the original exactly.

- [ ] **Step 2: Run it and confirm it fails.**
  - Run: `go test ./internal/install/ -run 'RetiresLeftoverBlockAndWritesPointer' -count=1`
  - Expected: FAIL with `is written by more than one step`.

- [ ] **Step 3: Implement.**
  - In `rejectDuplicateDestinations`, allow `prev`/`planned` on one path when both are `KindManagedBlock` and their `BlockName`s differ, **whether or not one of them is a removal**. A removal and a write of the **same** block on one path stays refused.
  - Make the ordering removal-first within one path. Replace the `sort.SliceStable` key with a sort by cleaned path, then by `remove` descending (removals first).
  - Leave a comment explaining why: a removal re-verifies its pre-image, which an earlier write to the same file would invalidate. A write re-reads the file, so it composes after a removal. Rollback restores newest first, so both pre-images return the original bytes.
  - Verify that `capturePreImage` handles two steps on one path (two backups). If it collides on the backup name, key the backup on `Seq`.
  - In `inspect.go`, change the three block remedies to `"the " + document.MarkerSpelling(block) + " markers …"` (and likewise for `" block …"`), so a `dckt:` block's remedy names its own markers.

- [ ] **Step 4: Run the tests and confirm they pass.**
  - Run: `go test ./internal/install/ -count=1`
  - Expected: PASS, and the existing duplicate-destination tests still refuse a write+write and a same-block write+remove.
  - Add `TestRejectDuplicateDestinationsSameBlockRemoveAndWriteRefuses` if no such test exists.

- [ ] **Step 5: Mutation-test.** Back up `txn.go`, revert only the removal-first ordering, and run the Step 1 test with `-count=1`. Expected: red with a pre-image error. Restore from the backup and confirm green.

- [ ] **Step 6: Commit.** Message: `feat(install): retire one managed block and write another in the same file`.

---

### Task 4: A `settings-hook` target kind for Claude Code's `settings.json`

**Build tier:** premium. A defect here eats the user's settings.

**Files:**
- Create: `internal/install/settings_hook.go`
- Create: `internal/install/settings_hook_test.go`
- Modify:
  - `internal/install/state.go`: `KindSettingsHook`, `TargetRecord.HookCommand`, `validateTarget`.
  - `internal/install/target.go`: `Target.HookCommand`, `validate`, `RecordFor`.
  - `internal/install/inspect.go`: the `InspectTarget` switch and `recordMatchesDisk`.
  - `internal/install/txn.go`: the `applyStep` write and removal branches, and `removalTarget`.
  - `internal/install/uninstall.go`: `proveUninstallRemoval`.

**Interfaces:**
- Produces:
  - `install.KindSettingsHook TargetKind = "settings-hook"`
  - `Target.HookCommand string` and `TargetRecord.HookCommand string` (`json:"hook_command,omitempty"`)
  - A target `{Path: <home>/.claude/settings.json, Kind: KindSettingsHook, HookCommand: <cmd>, Role: <role>}` owns exactly one matcher group in `hooks.SessionStart`: `{"hooks":[{"type":"command","command":<cmd>}]}`, with no matcher, so it fires for every session source.
  - Record identity: `SHA256 = hookGroupDigest(cmd)`, the sha256 of the compact canonical group.

- [ ] **Step 1: Write the failing tests** in `settings_hook_test.go`:
  - **`TestInsertHookGroupRoundTrips`**: table-driven over these originals:
    - `{}\n`
    - `{\n  "model": "opus"\n}\n`
    - `{\n\t"env": {\n\t\t"A": "1"\n\t}\n}\n` (tabs)
    - `{"model":"opus"}` (compact, no newline)
    - `{\n  "hooks": {\n    "Stop": [\n      {\n        "hooks": [\n          {\n            "type": "command",\n            "command": "say done"\n          }\n        ]\n      }\n    ]\n  }\n}\n` (hooks without `SessionStart`)
    - A file whose `hooks.SessionStart` already holds a user group with `"matcher": "startup"`.

    For each one:
    - `ins, err := insertHookGroup(orig, "dckt instructions --hook")` succeeds and is `json.Valid`.
    - `readSessionStartGroups(ins)` holds exactly one exact canonical group and every original group.
    - `out, removed, err := removeHookGroup(ins, "dckt instructions --hook")` gives `removed == true` and `bytes.Equal(out, orig)`. This is the byte-for-byte survival property.
  - **`TestInsertHookGroupCreatesFile`**: `insertHookGroup(nil, cmd)` equals `json.MarshalIndent` (prefix `""`, indent two spaces) of `{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":cmd}]}]}}`, plus `\n`.
  - **`TestSettingsHookInspect`** (temp dir, real files):
    - absent file → `create`
    - file without the command → `update`
    - exact group → `no-op`
    - a group whose handler has the command plus `"timeout": 5` → `no-op` (present, so it is never re-added)
    - `{"hooks": []}`, `[1]`, `not json` → `conflict` with reason `managed-block-invalid`
    - a symlinked `settings.json` → `conflict`
  - **`TestSettingsHookInstallUninstallLifecycle`**: drive `Install`, then `Uninstall`, through `Options` with a stub planner returning the settings-hook target, in a temp home seeded with a user `settings.json`. Assert:
    - the file after uninstall equals the seed bytes;
    - a second install is a no-op (no `Applied`);
    - when the group is edited between install and uninstall to add `"timeout": 5`, uninstall reports a `conflict` for the path and leaves the file byte-identical;
    - an injected later-step failure rolls `settings.json` back to the seed bytes.
  - **`TestValidateTargetSettingsHook`**: a `settings-hook` record without `hook_command` or `sha256` is invalid. A `file` record carrying `hook_command` is invalid.

- [ ] **Step 2: Run the tests and confirm they fail.** Run: `go test ./internal/install/ -run 'HookGroup|SettingsHook' -count=1`. Expected: FAIL to compile.

- [ ] **Step 3: Implement `settings_hook.go`.** Precondition for every edit: `json.Valid(src)` is true and the top-level value is an object. That lets the span scanner assume valid input.

```go
package install

// A settings-hook target owns one matcher group in settings.json's
// hooks.SessionStart. Every edit is a byte splice; nothing else is re-encoded.

const settingsHookEvent = "SessionStart"

type hookHandler struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

type hookGroup struct {
	Hooks []hookHandler `json:"hooks"`
}

func canonicalHookGroup(command string) hookGroup {
	return hookGroup{Hooks: []hookHandler{{Type: "command", Command: command}}}
}

// hookGroupDigest is a settings-hook record's identity.
func hookGroupDigest(command string) string {
	b, _ := json.Marshal(canonicalHookGroup(command))
	return hashBytes(b)
}

var errSettingsInvalid = errors.New("install: settings file is not a JSON object with an object `hooks` and an array `hooks.SessionStart`")

// readSessionStartGroups returns the SessionStart groups. ok=false: not
// editable (invalid JSON, non-object top level, or a wrong-typed hooks /
// SessionStart) — never "absent".
func readSessionStartGroups(src []byte) (groups []json.RawMessage, ok bool) {
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
	ss, has := hooks[settingsHookEvent]
	if !has {
		return nil, true
	}
	if json.Unmarshal(ss, &groups) != nil || groups == nil && string(bytes.TrimSpace(ss)) != "[]" {
		return nil, false
	}
	return groups, true
}

// isExactGroup reports whether raw is, semantically, the canonical group.
func isExactGroup(raw json.RawMessage, command string) bool {
	var have, want any
	if json.Unmarshal(raw, &have) != nil {
		return false
	}
	b, _ := json.Marshal(canonicalHookGroup(command))
	_ = json.Unmarshal(b, &want)
	return reflect.DeepEqual(have, want)
}

// groupHasCommand reports whether any handler in raw runs command.
func groupHasCommand(raw json.RawMessage, command string) bool {
	var g struct {
		Hooks []struct {
			Command string `json:"command"`
		} `json:"hooks"`
	}
	if json.Unmarshal(raw, &g) != nil {
		return false
	}
	for _, h := range g.Hooks {
		if h.Command == command {
			return true
		}
	}
	return false
}
```

Then add the span scanner and the splices:

```go
type jsonSpan struct{ Start, End int }

type jsonMember struct {
	Key   string
	Span  jsonSpan // key's opening quote through the value's end
	Value jsonSpan
}

// skipWS skips ' ', '\t', '\n', '\r' from i.
func skipWS(src []byte, i int) int

// valueEnd is the end of the valid JSON value starting at i.
func valueEnd(src []byte, i int) int {
	switch src[i] {
	case '"':
		for i++; src[i] != '"'; i++ {
			if src[i] == '\\' {
				i++
			}
		}
		return i + 1
	case '{', '[':
		depth := 0
		for ; ; i++ {
			switch src[i] {
			case '"':
				i = valueEnd(src, i) - 1
			case '{', '[':
				depth++
			case '}', ']':
				if depth--; depth == 0 {
					return i + 1
				}
			}
		}
	default:
		for i < len(src) && !bytes.ContainsRune([]byte(",}] \t\r\n"), rune(src[i])) {
			i++
		}
		return i
	}
}

func objectMembers(src []byte, open int) ([]jsonMember, int) {
	var out []jsonMember
	i := skipWS(src, open+1)
	if src[i] == '}' {
		return nil, i
	}
	for {
		keyEnd := valueEnd(src, i)
		var key string
		_ = json.Unmarshal(src[i:keyEnd], &key)
		v := skipWS(src, skipWS(src, keyEnd)+1)
		end := valueEnd(src, v)
		out = append(out, jsonMember{Key: key, Span: jsonSpan{i, end}, Value: jsonSpan{v, end}})
		i = skipWS(src, end)
		if src[i] == '}' {
			return out, i
		}
		i = skipWS(src, i+1)
	}
}

// arrayElements mirrors objectMembers over `[`…`]`: element spans, and the `]` offset.
func arrayElements(src []byte, open int) ([]jsonSpan, int)

// indentUnit is the file's own indentation step, or two spaces.
func indentUnit(src []byte) string {
	lines := bytes.Split(src, []byte("\n"))
	for _, line := range lines[1:] {
		trimmed := bytes.TrimLeft(line, " \t")
		if len(trimmed) > 0 && len(trimmed) < len(line) {
			return string(line[:len(line)-len(trimmed)])
		}
	}
	return "  "
}

// appendInto splices text (an already-indented value or `"key": value`) as
// the container's last entry. depth is the container's nesting depth.
func appendInto(src []byte, open, close int, last *jsonSpan, text, unit string, depth int) []byte {
	pad := strings.Repeat(unit, depth+1)
	var out []byte
	if last != nil {
		out = append(out, src[:last.End]...)
		out = append(out, ",\n"+pad+text...)
		return append(out, src[last.End:]...)
	}
	out = append(out, src[:open+1]...)
	out = append(out, "\n"+pad+text+"\n"+strings.Repeat(unit, depth)...)
	return append(out, src[close:]...)
}

// removeEntry splices entry k out of a container with the given entry spans.
func removeEntry(src []byte, open, close int, spans []jsonSpan, k int) []byte {
	var out []byte
	switch {
	case len(spans) == 1:
		out = append(out, src[:open+1]...)
		return append(out, src[close:]...)
	case k > 0:
		out = append(out, src[:spans[k-1].End]...)
		return append(out, src[spans[k].End:]...)
	default:
		out = append(out, src[:spans[0].Start]...)
		return append(out, src[spans[1].Start:]...)
	}
}

// indentJSON renders v as the value of an entry sitting inside a container of
// nesting depth depth (so the entry line itself is indented depth+1 units).
func indentJSON(v any, unit string, depth int) string {
	b, _ := json.MarshalIndent(v, strings.Repeat(unit, depth+1), unit)
	return string(b)
}

// insertHookGroup returns src with the canonical group appended to
// hooks.SessionStart, creating either container as the last member of its
// parent. A nil/blank src yields the canonical new file.
func insertHookGroup(src []byte, command string) ([]byte, error) {
	group := canonicalHookGroup(command)
	if len(bytes.TrimSpace(src)) == 0 {
		var doc struct {
			Hooks struct {
				SessionStart []hookGroup `json:"SessionStart"`
			} `json:"hooks"`
		}
		doc.Hooks.SessionStart = []hookGroup{group}
		b, _ := json.MarshalIndent(doc, "", "  ")
		return append(b, '\n'), nil
	}
	if _, ok := readSessionStartGroups(src); !ok {
		return nil, errSettingsInvalid
	}
	unit := indentUnit(src)
	top := skipWS(src, 0)
	members, closeTop := objectMembers(src, top)
	hooksIdx := findMember(members, "hooks")
	if hooksIdx < 0 {
		v := map[string][]hookGroup{settingsHookEvent: {group}}
		return appendInto(src, top, closeTop, lastSpan(members), `"hooks": `+indentJSON(v, unit, 0), unit, 0), nil
	}
	hv := members[hooksIdx].Value
	hm, hclose := objectMembers(src, hv.Start)
	ssIdx := findMember(hm, settingsHookEvent)
	if ssIdx < 0 {
		return appendInto(src, hv.Start, hclose, lastSpan(hm), `"`+settingsHookEvent+`": `+indentJSON([]hookGroup{group}, unit, 1), unit, 1), nil
	}
	av := hm[ssIdx].Value
	elems, aclose := arrayElements(src, av.Start)
	var last *jsonSpan
	if len(elems) > 0 {
		last = &elems[len(elems)-1]
	}
	return appendInto(src, av.Start, aclose, last, indentJSON(group, unit, 2), unit, 2), nil
}

// removeHookGroup removes the first exact canonical group, then drops a
// SessionStart array and a hooks object that the removal left empty.
// removed=false when no exact group exists.
func removeHookGroup(src []byte, command string) ([]byte, bool, error) {
	if _, ok := readSessionStartGroups(src); !ok {
		return nil, false, errSettingsInvalid
	}
	top := skipWS(src, 0)
	members, _ := objectMembers(src, top)
	hooksIdx := findMember(members, "hooks")
	if hooksIdx < 0 {
		return src, false, nil
	}
	hv := members[hooksIdx].Value
	hm, _ := objectMembers(src, hv.Start)
	ssIdx := findMember(hm, settingsHookEvent)
	if ssIdx < 0 {
		return src, false, nil
	}
	av := hm[ssIdx].Value
	elems, aclose := arrayElements(src, av.Start)
	k := -1
	for i, e := range elems {
		if isExactGroup(src[e.Start:e.End], command) {
			k = i
			break
		}
	}
	if k < 0 {
		return src, false, nil
	}
	out := removeEntry(src, av.Start, aclose, elems, k)
	if len(elems) > 1 {
		return out, true, nil
	}
	// The array is now empty: drop SessionStart; then hooks if it emptied too.
	members, _ = objectMembers(out, top)
	hv = members[findMember(members, "hooks")].Value
	hm, hclose := objectMembers(out, hv.Start)
	out = removeEntry(out, hv.Start, hclose, memberSpans(hm), findMember(hm, settingsHookEvent))
	if len(hm) > 1 {
		return out, true, nil
	}
	members, closeTop := objectMembers(out, top)
	return removeEntry(out, top, closeTop, memberSpans(members), findMember(members, "hooks")), true, nil
}
```

Add the small helpers `findMember(ms []jsonMember, key string) int` (first match, else `-1`), `lastSpan(ms []jsonMember) *jsonSpan` (the last member's `Span`, or nil), and `memberSpans(ms) []jsonSpan`.

Wire the kind through the installer:
- **`state.go`:** the constant and the field. A settings-hook record needs `SHA256`, `HookCommand`, and an empty `BlockName`/`LinkTarget`. Every other kind requires an empty `HookCommand`.
- **`target.go`:** `validate` requires `HookCommand`. `RecordFor` sets `HookCommand` and `SHA256 = hookGroupDigest(cmd)`.
- **`inspect.go` (`inspectSettingsHook`):**
  - non-regular file: `ReasonOwnershipConflict` with `remedyForPath`;
  - not editable: `ReasonManagedBlockInvalid`, with the remedy "this settings file is not a JSON object docket can edit (hooks must be an object, hooks.SessionStart an array); repair it by hand, then re-run";
  - any group with `groupHasCommand`: `DispositionNoop`;
  - otherwise: `DispositionUpdate` (adding only appends).

  `recordMatchesDisk` matches a regular, editable file holding an `isExactGroup` group.
- **`txn.go`:**
  - The write branch refuses `preSymlink`, then writes `insertHookGroup(<current bytes or nil>, cmd)` through `writeThroughStaging`.
  - The removal branch `removeSettingsHook` mirrors `removeManagedBlock`: `preAbsent` is a no-op, `preSymlink` refuses, and the result is `removeHookGroup` through staging with `want == 0`.
  - `removalTarget` maps the record to `Target{Path, Kind, HookCommand, Role}`.
- **`uninstall.go` (`proveUninstallRemoval`):**
  - not editable: `(false, false, ReasonManagedBlockInvalid, nil)`;
  - no group carries the command: `(true, false, "", nil)`;
  - otherwise `recordMatchesDisk` decides, so a modified group is a conflict.

- [ ] **Step 4: Run the tests and confirm they pass.** Run: `go test ./internal/install/ -count=1`. Expected: PASS.

- [ ] **Step 5: Mutation-test.** Restore from a backup copy each time, and use `-count=1`.
  - Make `removeHookGroup` skip the empty-container cleanup. The round-trip table must go red.
  - Make `inspectSettingsHook` treat a non-editable file as `DispositionUpdate`. The inspect test must go red.

- [ ] **Step 6: Commit.** Message: `feat(install): settings-hook target kind for a Claude Code SessionStart entry`.

---

### Task 5: The harness triggers and the no-rule-text guard

**Build tier:** standard

**Files:**
- Create: `internal/harness/trigger.go`, `internal/harness/trigger_test.go`
- Modify:
  - `internal/harness/claude/claude.go` (`Plan`)
  - `internal/harness/codex/codex.go` (`Plan`)
  - `internal/harness/opencode/opencode.go` (`Plan`)
  - their `_test.go` files and `testdata/golden`
- Modify, if the audit finds a leak: any test that reaches the installer without pinning `HOME`

**Interfaces:**
- Consumes: `install.KindSettingsHook` and `Target.HookCommand` (Task 4); qualified block names (Task 2); same-file retirement (Task 3).
- Produces, in package `harness`:

```go
// User-level triggers: content-free by design — no rule text, no "docket".
const (
	TriggerRole            = "trigger"
	TriggerHookCommand     = "dckt instructions --hook"
	PointerBlockName       = "dckt:private-instructions"
	PointerBlockAnnotation = "managed — do not hand-edit"
	PointerInterior        = "## Private repository instructions\n\n" +
		"At the start of a session inside a git repository, run `dckt instructions` once.\n" +
		"If it prints anything, treat that output as this repository's own AGENTS.md and\n" +
		"follow it for the rest of the session. If it prints nothing, ignore this section.\n"
)

// PointerTarget is the Codex/OpenCode pointer block in the given user-level AGENTS.md.
func PointerTarget(path string) install.Target {
	return install.Target{Path: path, Kind: install.KindManagedBlock, BlockName: PointerBlockName,
		Annotation: PointerBlockAnnotation, Content: []byte(PointerInterior), Role: TriggerRole}
}

// HookTarget is Claude Code's SessionStart hook entry in the given settings.json.
func HookTarget(path string) install.Target {
	return install.Target{Path: path, Kind: install.KindSettingsHook, HookCommand: TriggerHookCommand, Role: TriggerRole}
}
```

- Claude's `Plan` appends `harness.HookTarget(filepath.Join(home, ".claude", "settings.json"))`.
- Codex's `Plan` appends `harness.PointerTarget(<home>/.codex/AGENTS.md)`.
- OpenCode's `Plan` appends `harness.PointerTarget(<ConfigHome>/opencode/AGENTS.md)`.
- Each uses the same path its `GlobalDispatchTarget` names, and keeps `Plan`'s sorted order.

- [ ] **Step 1: Write the failing tests.**
  - **`trigger_test.go`, `TestTriggersCarryNoRuleTextAndNoDocket`:**
    - Render every byte a trigger writes:
      - the pointer through `document.Parse(nil)` plus `InsertBlock(PointerBlockName, PointerBlockAnnotation, PointerInterior, AtDocumentStart)`;
      - the hook through `install`'s new-file rendering. Reach it by running a temp-dir `install.InspectTarget`, then a real `Install`, with a stub planner. Alternatively, re-encode `{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":TriggerHookCommand}]}]}}` with `json.MarshalIndent`, and say in a comment that Task 4's test pins that this equals the inserted bytes.
    - Assert `!strings.Contains(strings.ToLower(text), "docket")`.
    - Assert that, for every non-blank line `l` of `DispatchInterior(<real run tracker from assets.EmbeddedCatalog()>)` and of `CodexRootEntryClause` (trimmed, at least 20 characters), `!strings.Contains(text, l)`.
    - Assert the population is not vacuous: the line count checked is ≥ 10. Pin it with a computed count, never a written one (**marker-scoped-guard-needs-a-population-floor**).
  - **Adapter tests** (claude, codex, opencode):
    - `Plan` contains exactly one target of role `trigger`, at the path above, with the fields above.
    - Cursor's `Plan` contains none.
    - Update each golden through the package's documented golden-refresh mechanism (read the test file's header), and review the diff. It must add only the trigger target.

- [ ] **Step 2: Run the tests and confirm they fail.**
  - Run: `go test ./internal/harness/... -count=1`
  - Expected: FAIL (undefined constants, missing targets).

- [ ] **Step 3: Implement** `trigger.go` and the three `Plan` appends. Then check `internal/harness/cross_harness_test.go` and `inventory_test.go` for invariants over target roles or sibling-name bans, and satisfy them without weakening them.

- [ ] **Step 4: Audit hermeticity.** Grep `--include='*_test.go'` under `internal` and `tests` for `"install"`, `install.Install(`, `install.Uninstall(`, `DevelopmentInstall(`, `installAuthorizedSurfaces(`, and `runInitWith`. Confirm every test that reaches a real install pins `HOME` (with `pinInstallEnv`, `t.Setenv`, or temp `UserRoots`). Fix any that do not, and list them in NOTES.

- [ ] **Step 5: Run the dependants and confirm they pass.**
  - Run: `go test ./internal/harness/... ./internal/install/ ./internal/app/ ./internal/cli/ -count=1`
  - Expected: PASS.
  - A full-install test in `internal/cli` that asserts an exact action list now sees the trigger creates. Update its expectation, and do not weaken the assertion.

- [ ] **Step 6: Mutation-test the guard.**
  - Back up `trigger.go`, change `PointerBlockAnnotation` to `managed by docket — do not hand-edit`, and confirm the guard goes red. Restore.
  - Then append one dispatch-preamble sentence to `PointerInterior`, confirm red, and restore.

- [ ] **Step 7: Commit.** Message: `feat(harness): plan the private-instructions triggers for claude, codex, and opencode`.

---

### Task 6: The private file's location, the private surface plan, and the Cursor exclusion

**Build tier:** standard

**Files:**
- Modify: `internal/layout/layout.go`, `internal/layout/layout_test.go`
- Modify: `internal/reposeed/plan.go`, `internal/reposeed/plan_test.go`
- Modify: `internal/reposetup/exclude.go`, `internal/reposetup/exclude_test.go`

**Interfaces:**
- Produces:
  - `layout.PrivateInstructionsFile = "AGENTS.md"`, `layout.PrivateInstructionsDisplay = ".git/dckt/AGENTS.md"`, and `func layout.PrivateInstructionsPath(commonDir string) string`, which returns `<commonDir>/dckt/AGENTS.md`.
  - `reposeed.PrivateCursorRuleRel = ".cursor/rules/dckt-dispatch.mdc"`.
  - `func reposeed.PlanPrivate(in reposeed.PrivatePlanInput) ([]install.Target, map[string][]string, error)` with:

    ```go
    type PrivatePlanInput struct {
    	WorktreeRoot string   // the PRIMARY worktree; its .git is the common dir
    	CommonDir    string   // must equal WorktreeRoot/.git
    	Harnesses    []string // validated opt-ins
    	RunTracker   []byte
    }
    ```

  - It plans:
    - `<CommonDir>/dckt/AGENTS.md` as `KindManagedBlock`, `BlockName "dispatch"`, the same annotation as the shared block, and `CodexDispatchInterior` when codex is selected (otherwise `DispatchInterior`). It is owned by whichever of claude, codex, and opencode are selected, and present iff at least one of them is.
    - `<WorktreeRoot>/.cursor/rules/dckt-dispatch.mdc` as `KindFile` with `cursor.DispatchRuleContent`, owned by `cursor`.
  - It errors on an unknown token, or when `filepath.Clean(CommonDir) != filepath.Join(filepath.Clean(WorktreeRoot), ".git")`. The error message names the unsupported separate-git-dir layout.
  - The exclude block now lists `.worktrees/` and then `.cursor/rules/dckt-dispatch.mdc`.

- [ ] **Step 1: Write the failing tests.**
  - **layout:** `PrivateInstructionsPath("/r/.git") == "/r/.git/dckt/AGENTS.md"`.
  - **reposeed (`TestPlanPrivate`):**
    - `[claude]` plans one target, the private file, with interior `harness.DispatchInterior(rt)`.
    - `[claude, codex]` plans the private file with `CodexDispatchInterior` and owners `[claude, codex]`.
    - `[cursor]` plans only the mdc file.
    - `[opencode, cursor]` plans both.
    - No target path, for any input, has base name `AGENTS.md` or `CLAUDE.md` outside `CommonDir`, or is `.cursor/rules/docket-dispatch.mdc`. Assert this over the union of every case.
    - A `CommonDir` that is not `<root>/.git` errors.
    - An unknown token errors.
  - **exclude:**
    - Update the `canonicalExclude` constant to `"# dckt:start\n.worktrees/\n.cursor/rules/dckt-dispatch.mdc\n# dckt:end\n"`.
    - Add `TestExcludeBlockRewritesPreCursorBlock`: an old two-line block is reported `changed` and rewritten. `TestExcludeBlockNeutralSpelling` must still pass, since the line contains no "docket".

- [ ] **Step 2: Run the tests and confirm they fail.**
  - Run: `go test ./internal/layout/ ./internal/reposeed/ ./internal/reposetup/ -count=1`
  - Expected: FAIL.

- [ ] **Step 3: Implement.**
  - `PlanPrivate` reuses `Plan`'s token validation, `contained` check, and owner sorting. Factor the shared token switch into a helper instead of copying it.
  - In `canonicalExcludeBytes`, add the line `.cursor/rules/dckt-dispatch.mdc\n` after `.worktrees/\n`.

- [ ] **Step 4: Run the tests and confirm they pass.**
  - Run the Step 2 command, then `go test ./internal/app/ -count=1`.
  - The private integration tests use `ValidExcludeBlock` rather than literal bytes. If any compares literal exclude bytes, update the expectation.
  - Expected: PASS.

- [ ] **Step 5: Commit.** Message: `feat(reposeed): plan a private repository's instructions file and excluded Cursor rule`.

---

### Task 7: The repository phase writes the private surfaces in a private repository

**Build tier:** premium. A mistake writes docket files into a private working tree or retires a user's file.

**Files:**
- Modify: `internal/app/repophase.go` (`ResolveRepoPhase`, `composeRecordBytes`, `computeRemovals`)
- Modify: `internal/app/repository_init.go` (`installAuthorizedSurfaces`)
- Modify: `internal/app/repository_init_private.go` (step 8; export the exclude helper)
- Modify: `internal/cli/install.go` (ensure the exclude block before a mutating install)
- Create:
  - `internal/app/private_instructions_integration_test.go` (`//go:build integration`, blank line 2)
  - `tests/test_go_integration_app_privateinstructions.sh`
- Modify: `tests/runtime-budgets.tsv`

**Interfaces:**
- Consumes: `reposeed.PlanPrivate`, `layout.PrivateInstructionsPath`, the exclude block (Task 6).
- Produces:
  - In a private repository, `ResolveRepoPhase` always plans against the **primary** worktree, publishes its record at `reposeed.RecordPath(commonDir, layout.PrivateName)`, and plans only `PlanPrivate`'s targets.
  - `func EnsurePrivateExclude(ctx context.Context, git *gitcli.Client, repoDir string) (bool, error)` is a no-op (`false, nil`) outside a private repository. Inside one, it is `ensureExcludeFile(<common>/info/exclude)`, which this task renames and exports from the existing helper. A malformed block returns `*reposetup.MalformedExcludeError`.

- [ ] **Step 1: Create the shard.**
  - Copy `tests/test_go_integration_app_reposetup.sh` to `tests/test_go_integration_app_privateinstructions.sh`. Set `SHARD_PREFIX="TestIntegrationPrivateInstructions"`, update the header comment, and keep the `# docket-suite: go` declaration in the first 10 lines.
  - Add the row `tests/test_go_integration_app_privateinstructions.sh<TAB>30<TAB>parallel` to `tests/runtime-budgets.tsv`, in sorted position.

- [ ] **Step 2: Write the failing integration tests** in `private_instructions_integration_test.go`. Reuse the existing private and shared init helpers (`newPrivateInitRepo`, `runInitWith`). Pin `HOME` and `XDG_CONFIG_HOME` to temp dirs in each test.
  - **`TestIntegrationPrivateInstructionsInitWritesPrivateFileOnly`** (acceptance 1):
    - Private init, add `agent_harnesses: [claude, codex, cursor]` to `.git/dckt/config.yml`, then init again.
    - `.git/dckt/AGENTS.md` holds a `docket:dispatch` block containing `### Codex root-coordinator entry`.
    - `.cursor/rules/dckt-dispatch.mdc` exists.
    - `AGENTS.md`, `CLAUDE.md`, and `.cursor/rules/docket-dispatch.mdc` are absent.
    - `git status --porcelain` is empty, and `git check-ignore -q .cursor/rules/dckt-dispatch.mdc` exits 0.
    - `PendingPaths` names no private path.
  - **`…InstallPhaseFromFeatureWorktree`:** `ResolveRepoPhase` called from a `git worktree add` worktree plans against the primary: the private file plus the primary's mdc, with the record at `<common>/dckt/install.json`.
  - **`…RetiresBuggyWorktreeSurfaces`:**
    - Apply a phase built by the shared `reposeed.Plan` (`[claude, codex]`) to the private primary through `applyRepoPhaseSurfaces`, recording at `<common>/dckt/install.json`.
    - Re-run `installAuthorizedSurfaces`.
    - AGENTS.md and the CLAUDE.md link are gone, and the private file exists.
  - **`…OwnerRemovalKeepsSharedFile`:**
    - Change `[claude, codex]` to `[claude]`: the file keeps the plain interior.
    - Change it to `[]`: the block is retired, and user lines outside it survive.
  - **`…SharedRepositoryUnchanged`** (acceptance 6): a shared init with `agent_harnesses: [claude, codex]` writes AGENTS.md and the CLAUDE.md link, and no `.git/dckt` exists.
  - **`…EnsureExcludeNoOpShared`:** `EnsurePrivateExclude` on a shared repository returns `false, nil`, with the exclude file byte-unchanged.

- [ ] **Step 3: Run the shard and confirm it fails.**
  - Run: `bash tests/test_go_integration_app_privateinstructions.sh`
  - Expected: `NOT OK` lines, because worktree AGENTS.md is written and no private file exists.

- [ ] **Step 4: Implement.**
  - **`ResolveRepoPhase`:** after `CommonDirOf`, call `layout.Detect(common)`. An error returns `ReasonFilesystemFailed`. When the mode is private:

    ```go
    repo, derr := git.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: invocation})
    if derr != nil {
    	return nil, "", nil, &RepoResolutionError{Reason: install.ReasonInvalidRepoDir, Err: derr}
    }
    root = repo.PrimaryWorktree
    recordPath = reposeed.RecordPath(repo.CommonDir, layout.PrivateName)
    ```

    Load the config from `root`; the authorization check is unchanged. Plan with `reposeed.PlanPrivate(…{WorktreeRoot: root, CommonDir: repo.CommonDir, Harnesses: effective, RunTracker: runTracker})`. A layout error from it maps to `ReasonInvalidRepoDir`.
  - **Retiring leftovers in private mode.** Pass `planned map[string]bool` (the cleaned planned paths, nil in shared mode) to `composeRecordBytes` and `computeRemovals`. In private mode, an unplanned prior surface is never wanted and never carried, unless it has no in-scope owner, in which case it is carried untouched. Retirement stays proof-gated through `install.PlanGlobalRetirements`.
  - **`installAuthorizedSurfaces`:** return nil pending paths when `layout.Detect(<primary common dir>)` is private.
  - **`runPrivateInit`, step 8:** call `installAuthorizedSurfaces(ctx, d.Git, sc.repo.PrimaryWorktree)`, which authorizes itself. Wrap its error as `fail(mapSurfaceFailure(cls.State, serr))` and fold `changed` into the result. Keep the "no `.gitignore` edit" comment.
  - **`internal/cli/install.go`:** on the install and development-install paths only (never check, uninstall, or a dry run), once the phase is `Authorized`, call `app.EnsurePrivateExclude(ctx, git, repoDir)` before the transaction. Any error refuses as `InstallRefusal{Reason: install.ReasonFilesystemFailed}`. A malformed block's message says to fix the `# dckt:` markers in `.git/info/exclude` by hand.

- [ ] **Step 5: Run the tests and confirm they pass.**
  - Run: `bash tests/test_go_integration_app_privateinstructions.sh && bash tests/test_go_integration_app_reposetup.sh && go test ./internal/app/ ./internal/cli/ -count=1`
  - Expected: all `ok`, exit 0.
  - Run `bash tests/test_go_integration_contract.sh`. Expected: exit 0, with the new shard matched exactly once.

- [ ] **Step 6: Mutation-test.** Restore from a backup copy each time.
  - Force `mode = layout.Shared` in `ResolveRepoPhase`. `...InitWritesPrivateFileOnly` must go red.
  - Pass `planned = nil` in private mode. `...RetiresBuggyWorktreeSurfaces` must go red.

- [ ] **Step 7: Commit.** Message: `feat(app): write a private repository's instructions to .git/dckt, never the working tree`.

---

### Task 8: The `docket instructions` command

**Build tier:** standard

**Files:**
- Create: `internal/app/instructions.go`, `internal/app/instructions_test.go`
- Create: `internal/cli/instructions.go`, `internal/cli/instructions_test.go`
- Modify:
  - `internal/cli/root.go`: register the command; add a raw-output path beside `devTestCode`.
  - `internal/cli/install.go`: `assetIndependent["instructions"] = true`.
  - `internal/app/schema_registry.go`: the binding `{ID: "instructions", Request: nil, Result: InstructionsResult{}}`, in sorted position, with the `// Instructions` derivation comment.
- Test, extended: `internal/app/private_instructions_integration_test.go`

**Interfaces:**
- Consumes: `layout.CommonDirOf`, `layout.Detect`, `layout.PrivateInstructionsPath`.
- Produces:

```go
// ReadPrivateInstructions walks up from dir to the first `.git` entry (no git
// process) and returns the private instructions file. Outside git or in a
// non-private repository: (nil, false, nil). Private without the file:
// (nil, true, nil). Any other probe/read error is returned, never "nothing".
func ReadPrivateInstructions(dir string) (content []byte, private bool, err error)

// SessionStartHookOutput wraps content in the SessionStart hook document + "\n"; nil → nil.
func SessionStartHookOutput(content []byte) []byte

type InstructionsResult struct {
	Envelope
	Private bool   `json:"private"`
	Content string `json:"content"`
}

// Instructions is the --json form: applied, or external-failed on a read error.
func Instructions(dir string) InstructionsResult
```

- [ ] **Step 1: Write the failing unit tests** in `internal/app/instructions_test.go`. They fabricate git layouts on disk, with no git process.
  - **Outside git:** a temp dir with no `.git` anywhere up to `/`. Use `testsupport.TempDir` and assert the walk-up stops at the root without error. Expect `nil, false, nil`.
  - **Shared:** `r/.git/` exists, `r/.git/dckt` does not. Expect `nil, false, nil` from `r` and from `r/sub/dir`.
  - **Private primary:** `r/.git/dckt/AGENTS.md` = `"rules\n"`. From `r/sub`, expect `"rules\n", true, nil`.
  - **Private feature worktree:** `r/.worktrees/f/.git` is a file `gitdir: ../../.git/worktrees/f`, and `r/.git/worktrees/f/commondir` = `../..`. From `r/.worktrees/f`, expect `"rules\n", true`.
  - **Private metadata checkout** outside the clone: `s/checkouts/c/.git` is a file with the absolute gitdir `r/.git/worktrees/c`, and `commondir` = `../..`. Expect `"rules\n", true`.
  - **Private without file:** expect `nil, true, nil`.
  - **Unreadable:** `chmod 000` on the file. Skip when running as root. Expect a non-nil error.
  - **`r/.git/dckt` is a regular file:** expect an error.
  - **`SessionStartHookOutput([]byte("a\"b\n"))`** must unmarshal to `hookSpecificOutput.hookEventName == "SessionStart"` and `additionalContext == "a\"b\n"`. `SessionStartHookOutput(nil)` must be nil.

- [ ] **Step 2: Write the failing CLI tests** in `internal/cli/instructions_test.go`. Use `t.Chdir` into the fabricated layouts from Step 1; copy the small fabrication helper.
  - Private: `runCLI(t, "instructions")` gives stdout exactly `"rules\n"` (no extra newline), stderr empty, code 0.
  - Shared, and outside git: stdout `""`, stderr `""`, code 0. Also run `--hook` and confirm the same empty result.
  - Private `--hook`: stdout is one JSON line carrying the content, code 0.
  - `--json` in private: the document has `"operation":"instructions","result":"applied","private":true,"content":"rules\n"`.
  - `--json --hook`: an invalid-arguments error, exit 2.
  - Unreadable in human mode: stdout `""`, stderr non-empty naming the path, non-zero exit.
  - Asset independence: with `pinInstallEnv(t)` and no installation, `instructions` still exits 0, because the hook must never fail on a machine mid-install.
  - `TestAssetIndependentSetExact` and the capability and schema correspondence tests must pass with the new command.

- [ ] **Step 3: Run the tests and confirm they fail.**
  - Run: `go test ./internal/app/ -run 'Instructions|HookOutput' -count=1 && go test ./internal/cli/ -run Instructions -count=1`
  - Expected: FAIL to compile.

- [ ] **Step 4: Implement.**
  - **`app/instructions.go`:**
    - Walk up from `filepath.Abs(dir)` to the first `os.Lstat(<d>/.git)` hit, stopping at `filepath.Dir(d) == d`.
    - Then call `layout.CommonDirOf` and `layout.Detect` (errors are returned; `Shared` gives `nil, false, nil`), and read `layout.PrivateInstructionsPath` (not-exist gives `nil, true, nil`).
    - `SessionStartHookOutput` is `json.Marshal` plus `'\n'`.
    - `Instructions` builds `NewEnvelope("instructions", …)`: `ResultApplied`, or `ResultExternalFailed` with the failure status the other read operations use.
    - `HumanText()` returns the content.
  - **`cli/instructions.go`:** `newInstructionsCommand(jsonMode func() bool, setResult func(app.OperationResult), setRaw func(out []byte, errText string, code int))`, with `Use: "instructions"`, `cobra.NoArgs`, `capability("instructions", EffectRead)`, and a `--hook` flag.
    - Help: prints this private repository's agent instructions (`.git/dckt/AGENTS.md`) from any of its worktrees, prints nothing elsewhere and exits 0, and `--hook` emits Claude Code `SessionStart` hook output.
    - `RunE` cases:
      - JSON mode with `--hook`: error `"--hook cannot be combined with --json"`.
      - JSON mode: `setResult(app.Instructions(cwd))`.
      - Read error: `setRaw(nil, "docket instructions: "+err.Error(), 1)`.
      - `--hook`: `setRaw(app.SessionStartHookOutput(content), "", 0)`.
      - Otherwise: `setRaw(content, "", 0)`.
  - **`root.go`:** add `var rawOut *rawOutput` (`out []byte; errText string; code int`). Add a `case rawOut != nil:` **before** `case result != nil:`. It writes `out` verbatim to stdout, writes `errText` plus `"\n"` to stderr when it is set, and returns `code`. Register the command.
  - If Task 1 found that only plain text works for Claude, `--hook` emits the content as-is. Say so in the help and in NOTES.

- [ ] **Step 5: Add the end-to-end integration test** `TestIntegrationPrivateInstructionsReadFromEveryWorktree` (acceptance 2):
  - After Task 7's private init with `[claude, codex]`, `ReadPrivateInstructions` returns the private file's bytes from the primary worktree, from a `git worktree add` feature worktree, and from `privateLayoutOf(...).MetadataWorktree`.
  - It returns `private == false` from a shared-init repository and from a plain `git init` repository.

- [ ] **Step 6: Run the tests and confirm they pass.**
  - Run: `go test ./internal/app/ ./internal/cli/ -count=1 && bash tests/test_go_integration_app_privateinstructions.sh`
  - Expected: PASS, exit 0.

- [ ] **Step 7: Mutation-test.** Back up `instructions.go`, make the read error return `nil, true, nil`, and confirm the unreadable test goes red. Restore.

- [ ] **Step 8: Commit.** Message: `feat(cli): docket instructions prints a private repository's agent instructions`.

---

### Task 9: The skills: private promotion destination and private delivery

**Build tier:** standard

**Files:**
- Modify:
  - `skills/docket-convention/references/learnings.md` (*Promotion — the shrink valve*)
  - `skills/docket-convention/references/agent-layer.md` (*Repository dispatch blocks: agent_harnesses*)
  - their generated twins, via `go generate ./internal/assets/`
- Test: the existing skill and asset drift tests, plus one new prose guard in the package that already guards these references. Find it with `grep -rln 'agent-layer.md' --include='*_test.go' internal tests`.

- [ ] **Step 1: Write the failing guard.** It asserts that `learnings.md`'s promotion section binds `.git/dckt/AGENTS.md` to "private repository" within one paragraph (**prose-guard-binds-phrase-to-claim**). It also asserts that `agent-layer.md` names `dckt instructions --hook`, `dckt:private-instructions`, and `.cursor/rules/dckt-dispatch.mdc` inside the agent_harnesses section. Slice the section by its named heading and assert that the terminator heading exists.

- [ ] **Step 2: Run the guard and confirm it fails.** Run the package's tests with `-count=1`. Expected: FAIL.

- [ ] **Step 3: Edit the prose.**
  - **`learnings.md`, after the sentence naming the integration-branch file:** "In a private repository the graduation lands instead in the private instructions file `.git/dckt/AGENTS.md`, outside its managed `docket:dispatch` block, since a private repository carries no agent-instructions file in its tree."
  - **`agent-layer.md`, a new short subsection** under the agent_harnesses section, *In a private repository*, stating:
    - `agent_harnesses` writes the dispatch block into `.git/dckt/AGENTS.md`, plus the git-excluded `.cursor/rules/dckt-dispatch.mdc` for cursor, and nothing else in the working tree;
    - `docket install` adds user-level triggers that carry no rules: a Claude Code `SessionStart` hook running `dckt instructions --hook` in `~/.claude/settings.json`, and a `dckt:private-instructions` pointer block in `~/.codex/AGENTS.md` and `~/.config/opencode/AGENTS.md`;
    - outside a private repository, `docket instructions` prints nothing, so the triggers are inert;
    - uninstall removes the hook entry only while it is unchanged.

    Write it for a reader in an unknown repository (**distributed-body-has-no-local-repo**), and state current behavior only.
  - Run `go generate ./internal/assets/`.

- [ ] **Step 4: Run the tests and confirm they pass.** Run: `go test ./internal/assets/ <guard package> -count=1`. Expected: PASS.

- [ ] **Step 5: Mutation-test.** Back up `learnings.md`, delete the new sentence, regenerate, and confirm the guard goes red. Restore the backup and regenerate.

- [ ] **Step 6: Commit.** Message: `docs(skills): private-repository instructions file and its user-level triggers`.

---

### Final: whole-suite gate

- [ ] **Step 1: Check the twins.** Run `go generate ./internal/assets/`, then `git status --porcelain`. Expected: empty.
- [ ] **Step 2: Run the build gate.** Run whatever `build.test_command` resolves to; read it from config. Expected: green. Act on `SERIAL CONFIRMED OVER BUDGET:`, and record `BUDGET WATCH:` and `PARALLEL-SENSITIVE:` lines, especially for the new shard and `reposetup`.
- [ ] **Step 3: Confirm the residue.**
  - `grep -rn -i 'docket' internal/harness/trigger.go`: the only hits must be Go comments, never a string literal.
  - `grep -rn '"\.cursor/rules/docket-dispatch\.mdc"\|cursorRuleRel' internal/reposeed/plan.go`: the hits must be confined to `Plan`. Read each to confirm.
- [ ] **Step 4: Collect notes for the results file** (written by the parent):
  - Task 1's spike table, verbatim, plus each NOT-EXERCISABLE harness as an **Important human verification item**, phrased as a state to reproduce. Always include one interactive item: "in a private repository after `docket install`, start a fresh interactive Claude Code session and confirm the dispatch rules are in context."
  - Every mutation and its red message.
  - The ADR above.
  - **Residuals:**
    - the triggers load in a session started **after** install; a running session keeps its old context (generated-artifact-loaded-at-process-start);
    - a private repository whose git directory is not `<primary>/.git` (`--separate-git-dir`) refuses the repository phase;
    - the excluded Cursor rule is written in the primary worktree only;
    - a `hook_command` field in the machine state cannot be read by a binary older than this change (a downgrade);
    - Claude Code may cap hook context size: record Task 1's payload byte count against any cap the spike observed;
    - the `dckt` alias being foreign (#534) makes the triggers fail harmlessly, and `docket instructions` still works by hand.
  - **Verified unchanged:** shared repositories plan through `reposeed.Plan` exactly as before (acceptance 6).
