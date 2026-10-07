<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0538 — Choose agent harnesses during init and with configure-harnesses](../../changes/active/0538-choose-agent-harnesses-during-repository-init.md)**
<!-- docket:backlink:end -->

# Choose Agent Harnesses During Init and With configure-harnesses: Implementation Plan

> **For agentic workers:** `docket-build` executes this plan. It routes each task to a tier agent running the `docket-build-task` contract. Each task carries its own focused test cycle and ends in one commit. The whole suite runs once at the end. Steps use checkbox (`- [ ]`) syntax.

**Goal:**
- `docket repository init` asks which coding agents get docket's dispatch instructions (`--harnesses`, or a checkbox picker on a terminal) and writes `agent_harnesses` to the repository config.
- A new `docket repository configure-harnesses` changes the choice later.
- `docket repository check` warns `harnesses-unset` while no repository-level layer declares the key.

**Architecture:**
- `internal/reposetup` gets the pure pieces. `ParseHarnessSelection` validates against the config's own token set. `RenderAgentHarnessesEdit` is a byte-preserving line splice of the top-level key. It reuses `topLevelMapping`, `maxNodeLine`, `lineOffsets`, `lineEndByte`, `mappingKeys`, and `findChild`, plus the post-splice re-parse discipline of `verifyOwnerPairs`.
- `internal/app/repository_harnesses.go` holds the shared step in two halves, both called by init and by configure-harnesses:
  - `resolveHarnessChoice` (flag, chooser, or caller policy) runs **before any write**, so a cancelled picker leaves nothing behind.
  - `applyHarnessChoice` writes through `writeTargetConfig`/`repoConfigTarget`, then calls `installAuthorizedSurfaces`. That function re-reads config from disk, so a key written in this run authorizes this run's surfaces. It also warns on a `.docket.local.yml` override.
- Init's `if facts.SurfacesAuthorized` gate is replaced by `applyHarnessChoice`. Private init calls it at its step 8 with the **resolved private layout**, never the gather-time one.
- `internal/cli/harness_picker.go` is the only importer of `github.com/charmbracelet/huh`. The CLI passes a chooser only when stdin and stdout are terminals, `--json` is off, and `--harnesses` was not given.
- `reposetup.HarnessesUnsetFinding` is appended by `RunRepositoryCheck` beside `TestConfigFinding`. It never changes the classified state.

**Tech Stack:** Go, cobra/pflag, `go.yaml.in/yaml/v3` nodes, and `github.com/charmbracelet/huh v1.0.0` (new). Real-git tests sit behind `//go:build integration`, sharded by prefix:
- `TestIntegrationRepoSetup` (`tests/test_go_integration_app_reposetup.sh`)
- `TestIntegrationRepoCheck` (`tests/test_go_integration_app_repocheck.sh`)
- `TestIntegrationBashUpgrade` (`tests/test_go_integration_bashupgrade.sh`)

**Spec:** `docs/superpowers/specs/2026-10-07-choose-agent-harnesses-during-repository-init-design.md` (in the feature worktree). Read *Design* and *Acceptance criteria* first.

## Global Constraints

- **Tokens:** `claude`, `codex`, `cursor`, `opencode`, validated against `config.AgentHarnessTokens()` (Task 1), never a second list. `none` stands alone and means `[]`.
  - An unknown token, a duplicate, an empty value or token, or `none` with a name is refused **before any repository read or write**, for both commands.
- **Written line:** `agent_harnesses: [claude, cursor]` / `agent_harnesses: []`. One flow line, in canonical order, with every other byte preserved. A value already present writes nothing.
- **Target:** shared -> `.docket.yml` (unstaged, a pending path). Private -> `.git/dckt/config.yml` (nothing pending).
- **Authorization unchanged:** only an explicit value at the repository or repository-local layer authorizes writes. A global value only pre-checks the picker.
- **Policies:**
  - init: a repository value with no flag -> kept, chooser never called. No flag and no chooser -> succeed and warn.
  - configure-harnesses: a repository value -> pre-checked. No flag and no chooser -> `invalid-input`.
- **Pre-check:** the resolved value when any layer declares it (repository, else global), else what `Detect` finds. Options are in `harness.Order`. An empty confirmation writes `[]`. Cancel -> `interrupted`, nothing written.
- **Init's guard is unchanged.** configure-harnesses admits `healthy`, `needs-review`, and any state that reclassifies as one of those once `PrimaryClean`/`PrimaryAtRemoteTip` are set aside. Remedies: fresh -> init, legacy -> migrate, else -> check.
- **`harnesses-unset`:** a warning, raised only when the remote metadata branch is present. It never changes the state. Like any finding on a healthy repository it makes `repository check` exit 1 (as `test-config-missing` does).
- **Capability:** `repository.configure-harnesses`, `local-write`, `RepositoryOpResult`, in `schema_registry.go`, asset-independent.
- **Only `internal/cli` imports huh.** `skills/` and `agents/` markdown name the operation id, never `docket repository configure-harnesses` (`TestCapabilitySurface`); `docs/` may spell it. Docs describe current behavior, with no change numbers.
- **AGENTS.md rules:**
  - Mutation-check from a **backup copy** (never `git checkout --`), with `-count=1`.
  - Template every `mktemp`.
  - Anchor on symbols.
  - Never pipe into `grep -q`/`head`.
  - Run `go generate ./internal/assets/` after editing `skills/**` or `.docket.example.yml`.
- **Hermetic home:** tests reaching `installAuthorizedSurfaces` or `detectHarnesses` pin `HOME` (`pinInitUserRoots`, `runInitWith`, the helpers below).

## Review Focus

Each item has a test in its owning task.
1. **Real `.docket.yml` shapes:** block-form items (this repository's own file), a trailing key comment, CRLF, no final newline, a nested key, and a block scalar. Each is replaced or appended byte-exactly, or refused (Task 2).
2. **Fresh `init --private`:** writes `.git/dckt/config.yml` and never creates a `.docket.yml`. The trap is the gather-time shared layout (Task 4).
3. **Shell-typed flags:** `"claude, cursor"`, a repeated flag, `""`, and `none,claude` (Tasks 1, 4, 6).
4. **Re-run before commit:** init never re-asks, and configure-harnesses twice leaves one key line (Tasks 4, 5).
5. **A `.docket.local.yml` override:** warns, names the applied value, and the surfaces follow it (Task 5).

## Decisions made in planning

- **Resolve before any write, apply at the spec's point** (after `.gitignore`), so a cancelled init leaves nothing. `sweepSetupDebris` (owned crash debris only) still runs first.
- **Fresh-private init** forces `repoDeclared=false` and the display `.git/dckt/config.yml`: that private config is all the repository will read.
- **configure-harnesses' pending paths** come from `git status` filtered by `docketManagedWorktreePaths`, so a removed surface counts. `installAuthorizedSurfaces`' target list also names unchanged files. Init keeps its reporting, deduplicated.
- **`CheckExit` is unchanged.** Healthy fixtures declare the key instead.
- **Bash-upgrade guide:** records `--harnesses none`. Choosing `claude` would collide with the Bash-era `CLAUDE.md` block, which the guide removes later. The prose says to choose agents after that removal.
- **huh:** `github.com/charmbracelet/huh v1.0.0`, the path the spec names (not `charm.land/huh/v2`).

## Learnings applied

Each lesson points to the task that applies it:
- predicate-must-ask-the-post-pass-state (Task 4)
- guard-keyed-on-presence-not-provenance (Task 5)
- validate-the-whole-input-set-first and validator-must-match-the-reader-it-feeds (Tasks 1, 3)
- defaulted-param-hides-caller-wiring (Task 6)
- printed-remedy-state-validity (Task 5)
- config-knob-ship-end-to-end (Task 8)
- config-layer-write-and-read-hazards: `HOME` is pinned in every test
- plan-supplied-test-code-is-unverified and intermediate-task-state-buildable: each task has a mutation step and leaves the tree green

## ADRs to record

The parent records this through `docket-adr`; it is not a build task.

1. **docket adopts `github.com/charmbracelet/huh` as its interactive terminal UI dependency.** It was chosen over a hand-rolled `golang.org/x/term` picker despite about 1.3–1.8 MB (around 12%) more binary and 42 modules in the graph. Only `internal/cli` imports it, and later prompts reuse it.

## Residuals (for the results file)

- The full-screen picker is tested only through huh's accessible mode. Human item: run `docket repository configure-harnesses` in a terminal; check the pre-checks, Space/Enter, and Ctrl-C.
- Record the binary size before and after, and the `go list -m all | wc -l` module count.
- Detection is a hint: an unresolvable home pre-checks nothing.
- A block-form key's replacement drops comment lines inside its own item range.

---

### Task 1: Parse and validate a harness selection

**Build tier:** economy

**Files:**
- Modify: `internal/config/schema.go`, `internal/config/schema_test.go`
- Create: `internal/reposetup/harnesses.go`, `internal/reposetup/harnesses_test.go`

**Interfaces (produces):**
- `config.AgentHarnessTokens() []string`: a fresh copy of `agentHarnessTokens`.
- `reposetup.HarnessesNone = "none"`
- `reposetup.ConfigureHarnessesCommand = "docket repository configure-harnesses"`
- `reposetup.ErrInvalidHarnesses` (every refusal wraps it)
- `reposetup.ParseHarnessSelection(tokens []string) ([]string, error)`: trimmed tokens; a **non-nil** result in canonical order; `["none"]` -> `[]string{}`.
- `reposetup.FormatHarnessList(h) string` -> `"[claude, cursor]"`
- `reposetup.AgentHarnessesLine(h) string` -> `"agent_harnesses: [claude, cursor]"`

- [ ] **Step 1: Write the failing tests.**
  - `TestParseHarnessSelection`, table-driven. Accepted:
    - `[claude]` -> `[claude]`
    - `[cursor claude]` -> `[claude cursor]`
    - `[" claude" "cursor "]` -> `[claude cursor]`
    - all four reversed -> canonical
    - `[none]` and `[" none "]` -> non-nil empty

    Refused: each case gives `errors.Is(err, ErrInvalidHarnesses)` and a message containing the listed words.
    - `nil`, `[]`, `[""]`, `[claude ""]` -> "empty"
    - `[claude claude]` -> "claude", "more than once"
    - `[none claude]` -> "none"
    - `[bogus claude nope]` -> "bogus", "nope", "claude, codex, cursor, opencode"
    - `[Claude]` -> "Claude"
  - `TestAgentHarnessesLine`: `[claude cursor]` -> `agent_harnesses: [claude, cursor]`; `[]` -> `agent_harnesses: []`.
  - `TestAgentHarnessTokensIsACopy` (config): joins to `claude,codex,cursor,opencode`, and mutating the result does not change the next call.

- [ ] **Step 2: Run and confirm they fail.** `go test ./internal/reposetup/ ./internal/config/ -run 'ParseHarnessSelection|AgentHarnessesLine|AgentHarnessTokens' -count=1`. Expected: build failure.

- [ ] **Step 3: Implement.**
  - `schema.go`: `func AgentHarnessTokens() []string { return append([]string(nil), agentHarnessTokens...) }`. Its doc comment says this is the accept set every writer of the key validates against.
  - `harnesses.go` (with a file comment): the constants and sentinel above; `FormatHarnessList` (`"[" + strings.Join(h, ", ") + "]"`); and `AgentHarnessesLine` (`"agent_harnesses: " + FormatHarnessList(h)`). `ParseHarnessSelection`:
    1. Takes `allowed := config.AgentHarnessTokens()`.
    2. Classifies **every** trimmed token before returning: `""` is empty; `none` is none; a token not in `allowed` (use `containsString`, which `testconfigedit.go` already has) is unknown; a token already seen is a dup; anything else is added to the seen set. `len(tokens) == 0` counts as empty.
    3. Then refuses, in this order, each wrapping `ErrInvalidHarnesses`:
       - empty: "--harnesses has an empty value; pass a comma list such as `claude,cursor`, or `none`"
       - unknown, all named: "unknown harness bogus, nope; choose from claude, codex, cursor, opencode, or `none`"
       - dup: "claude is listed more than once"
       - `none` with a name: "`none` cannot be combined with a harness name"
    4. Returns `out := []string{}` filled by walking `allowed` and keeping the seen ones (canonical order, never nil).

- [ ] **Step 4: Run and confirm they pass.** Mutation-check: delete the `none && len(seen) > 0` arm (keep a backup copy). The `[none claude]` row goes red. Restore and confirm green.

- [ ] **Step 5: Commit.**

`git add` exactly the files under **Files**, then `git commit -m "feat(reposetup): validate an agent_harnesses selection against the config's token set"`.

---

### Task 2: The byte-preserving `agent_harnesses` writer

**Build tier:** standard

**Files:** Create `internal/reposetup/harnessesedit.go` and `internal/reposetup/harnessesedit_test.go`.

**Interfaces:**
- Consumes: Task 1, plus `topLevelMapping`, `maxNodeLine` (configedit.go) and `lineOffsets`, `lineEndByte`, `mappingKeys`, `findChild`, `containsString` (testconfigedit.go).
- Produces: `reposetup.RenderAgentHarnessesEdit(existing []byte, harnesses []string) (edited []byte, changed bool, err error)`.
  - `existing == nil` means no file.
  - `harnesses` must be non-nil and canonical.
  - `changed == false` returns `existing` untouched.
  - An error returns `(nil, false, err)`.

- [ ] **Step 1: Write the failing tests.**

  `TestRenderAgentHarnessesEditRewrites`, one sub-test per row, asserting `changed && err == nil && string(got) == want`. An empty `in` is passed as `nil`.

| in | harnesses | want |
|---|---|---|
| `integration_branch: main\n` | claude | `integration_branch: main\nagent_harnesses: [claude]\n` |
| (nil) | none | `agent_harnesses: []\n` |
| `a: 1` | claude | `a: 1\nagent_harnesses: [claude]\n` |
| `# top\nagent_harnesses: [codex] # mine\nb: 2 # keep\n` | claude,cursor | `# top\nagent_harnesses: [claude, cursor]\nb: 2 # keep\n` |
| `x: 1\n# Harnesses here.\nagent_harnesses:\n  - claude\n  - opencode\nfinalize:\n  gate: local\n` | claude | `x: 1\n# Harnesses here.\nagent_harnesses: [claude]\nfinalize:\n  gate: local\n` |
| `a: 1\r\nagent_harnesses: [codex]\r\nb: 2\r\n` | claude | `a: 1\r\nagent_harnesses: [claude]\r\nb: 2\r\n` |
| `a: 1\r\n` | none | `a: 1\r\nagent_harnesses: []\r\n` |
| `a: 1\nagent_harnesses: [codex]` | none | `a: 1\nagent_harnesses: []` |
| `agent_harnesses:\nb: 2\n` | cursor | `agent_harnesses: [cursor]\nb: 2\n` |
| `build:\n  agent_harnesses: x\n` | claude | `build:\n  agent_harnesses: x\nagent_harnesses: [claude]\n` |
| `# only a comment\n` | claude | `# only a comment\nagent_harnesses: [claude]\n` |

  `TestRenderAgentHarnessesEditUnchangedWritesNothing`: these give `changed == false`, `err == nil`, and the bytes back unchanged.
  - `agent_harnesses: [claude, cursor]\n` with `[claude cursor]`
  - `a: 1\nagent_harnesses:\n  - claude\n  - cursor\n` with `[claude cursor]`
  - `agent_harnesses: []\n` with `[]`

  `TestRenderAgentHarnessesEditRefuses`: each case gives `err != nil`, `!changed`, and `got == nil`.
  - with `[claude]`:
    - `a: 1\n---\nb: 2\n`
    - `- a\n- b\n`
    - `a: [\n`
    - `agent_harnesses: []\nagent_harnesses: [claude]\n`
    - `{a: 1, agent_harnesses: []}\n`
    - `agent_harnesses: |\n  claude\nb: 2\n` (its error also contains "by hand")
  - selections `nil`, `[bogus]`, and `[cursor claude]` (non-canonical), on `a: 1\n`

- [ ] **Step 2: Run and confirm they fail.** `go test ./internal/reposetup/ -run RenderAgentHarnessesEdit -count=1`.

- [ ] **Step 3: Implement** `harnessesedit.go`. File comment: the key is located with a YAML AST parse and replaced or appended by a line splice on raw bytes, never re-serialized (like `RemoveMetadataBranchKey`). The result is re-parsed and refused unless the only change is the key (like `verifyOwnerPairs`).

```go
package reposetup

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"slices"

	"go.yaml.in/yaml/v3"
)

const agentHarnessesKey = "agent_harnesses"

func RenderAgentHarnessesEdit(existing []byte, harnesses []string) (edited []byte, changed bool, err error) {
	if harnesses == nil {
		return nil, false, errors.New("reposetup: no agent_harnesses selection to write")
	}
	if len(harnesses) > 0 {
		canon, perr := ParseHarnessSelection(harnesses)
		if perr != nil {
			return nil, false, perr
		}
		if !slices.Equal(canon, harnesses) {
			return nil, false, fmt.Errorf("reposetup: agent_harnesses selection %v is not in canonical order", harnesses)
		}
	}
	root, err := topLevelMapping(existing)
	if err != nil {
		return nil, false, err
	}
	byHand := "; refusing to edit — set agent_harnesses by hand"
	if root != nil && root.Style&yaml.FlowStyle != 0 {
		return nil, false, errors.New("reposetup: the config is a single-line {…} mapping" + byHand)
	}
	eol := "\n"
	if bytes.Contains(existing, []byte("\r\n")) {
		eol = "\r\n"
	}
	line := AgentHarnessesLine(harnesses)
	keyIdx := -1
	if root != nil {
		for i := 0; i+1 < len(root.Content); i += 2 {
			if k := root.Content[i]; k.Kind == yaml.ScalarNode && k.Value == agentHarnessesKey {
				if keyIdx >= 0 {
					return nil, false, fmt.Errorf("reposetup: %q declared more than once at the top level%s", agentHarnessesKey, byHand)
				}
				keyIdx = i
			}
		}
	}
	var out []byte
	if keyIdx < 0 {
		out = append([]byte(nil), existing...)
		if len(out) > 0 && out[len(out)-1] != '\n' {
			out = append(out, eol...)
		}
		out = append(out, line+eol...)
	} else {
		val := root.Content[keyIdx+1]
		if harnessSequenceEquals(val, harnesses) {
			return existing, false, nil
		}
		start, end := root.Content[keyIdx].Line, maxNodeLine(val)
		if end < start || val.Kind == yaml.ScalarNode && val.Tag == "!!null" && val.Value == "" {
			end = start // an empty value lives on the key's own line
		}
		if keyIdx > 0 && maxNodeLine(root.Content[keyIdx-1]) >= start ||
			keyIdx+2 < len(root.Content) && root.Content[keyIdx+2].Line <= end {
			return nil, false, fmt.Errorf("reposetup: %q shares a line with another setting%s", agentHarnessesKey, byHand)
		}
		starts := lineOffsets(existing)
		from, to := starts[start-1], lineEndByte(existing, starts, end)
		text := line
		if !(to == len(existing) && (to == 0 || existing[to-1] != '\n')) {
			text += eol
		}
		out = append(append(append(make([]byte, 0, len(existing)+len(text)), existing[:from]...), text...), existing[to:]...)
	}
	if verr := verifyHarnessesEdit(root, out, harnesses); verr != nil {
		return nil, false, verr
	}
	return out, true, nil
}

func harnessSequenceEquals(n *yaml.Node, want []string) bool {
	if n == nil || n.Kind != yaml.SequenceNode || len(n.Content) != len(want) {
		return false
	}
	for i, item := range n.Content {
		if item.Kind != yaml.ScalarNode || item.Tag != "!!str" || item.Value != want[i] {
			return false
		}
	}
	return true
}

// verifyHarnessesEdit: same keys (+ appended agent_harnesses), every other
// value decodes identically, agent_harnesses == want. Catches block scalars.
func verifyHarnessesEdit(origRoot *yaml.Node, edited []byte, want []string) error {
	refuse := func(format string, args ...any) error {
		return fmt.Errorf("reposetup: the agent_harnesses edit cannot be made safely ("+format+"); refusing to edit — set agent_harnesses by hand", args...)
	}
	after, err := topLevelMapping(edited)
	if err != nil || after == nil {
		return refuse("the edited file does not parse as a mapping: %v", err)
	}
	wantKeys := mappingKeys(origRoot)
	if !containsString(wantKeys, agentHarnessesKey) {
		wantKeys = append(append([]string(nil), wantKeys...), agentHarnessesKey)
	}
	if got := mappingKeys(after); !slices.Equal(got, wantKeys) {
		return refuse("the top-level settings would be %v, want %v", got, wantKeys)
	}
	for i := 0; i+1 < len(after.Content); i += 2 {
		k, v := after.Content[i].Value, after.Content[i+1]
		if k == agentHarnessesKey {
			if !harnessSequenceEquals(v, want) {
				return refuse("agent_harnesses would not be %s", FormatHarnessList(want))
			}
			continue
		}
		_, orig, _ := findChild(origRoot, k) // reached only for original keys, so origRoot != nil
		var a, b any
		if orig == nil || orig.Decode(&a) != nil || v.Decode(&b) != nil || !reflect.DeepEqual(a, b) {
			return refuse("setting %q would change", k)
		}
	}
	return nil
}
```

  Add a doc comment on `RenderAgentHarnessesEdit` listing the refusals: multi-document, non-mapping or `{…}` root, undecodable, duplicate key, shared line, and an unverifiable edit.

- [ ] **Step 4: Run and confirm they pass** (plus `go test ./internal/reposetup/ -count=1`). Mutation-check each from a backup copy:
  1. Return `nil` instead of calling `verifyHarnessesEdit`. The block-scalar refusal goes red.
  2. Delete the `harnessSequenceEquals` early return. The unchanged test goes red.

- [ ] **Step 5: Commit.**

`git add` exactly the files under **Files**, then `git commit -m "feat(reposetup): write agent_harnesses as one flow line, preserving every other byte"`.

---

### Task 3: The shared harness step in the app layer

**Build tier:** standard

**Files:**
- Create: `internal/app/repository_harnesses.go` and `internal/app/repository_harnesses_test.go` (**no build tag**, so integration tests reuse its helper)
- Modify: `internal/app/repository_init.go` (`RepositoryOpResult` fields; `mapSurfaceFailure` -> `mapSurfaceFailureFor`)

**Interfaces:**
- Consumes: Tasks 1–2, plus the existing `writeTargetConfig`, `repoConfigTarget`, `installAuthorizedSurfaces`, `resolveSetupConfig`, `isRepositoryLayer`, `Planners`, and `repositoryInternalFailure`.
- Produces, exported:
  - `HarnessChoiceRequest{Options, Preselected []string; ConfigPath string}`
  - `HarnessChooser func(ctx context.Context, req HarnessChoiceRequest) ([]string, error)`
  - `ErrHarnessChoiceCancelled`
  - `HarnessesOptions{Set bool; Tokens []string; Chooser HarnessChooser}`
  - `RepositoryOpResult.AgentHarnesses *[]string` (`json:"agent_harnesses,omitempty"`)
  - `RepositoryOpResult.Warnings []string` (`json:"warnings,omitempty"`)
- Produces, package-internal (exact signatures in Step 3):
  - `parseHarnessesFlag`, `harnessPolicyInit`/`harnessPolicyConfigure`, `harnessChoiceInput`, `harnessChoice`, `resolveHarnessChoice`, `errHarnessesFlagRequired`
  - `harnessRepoDeclared`, `harnessPreselection`, `detectHarnesses`
  - `harnessApplied`, `applyHarnessChoice`, `*harnessConfigError`, `harnessApplyFailure`, `harnessChoiceCancelled`
  - `appendPending`, `setHarnessResult`, `mapSurfaceFailureFor(operation, state, err)`
- Test helper: `recordingChooser(got *HarnessChoiceRequest, calls *int, ret []string, err error) HarnessChooser`.

- [ ] **Step 1: Write the failing tests** in `repository_harnesses_test.go`. Define `recordingChooser` (it records the request, counts calls, and returns `ret, err`) and `harnessCfg(layer config.LayerKind, v ...string) config.Effective`. The latter sets `AgentHarnesses = config.Value[[]string]{Value: v, Explicit: true, Provenance: config.Provenance{Layer: layer}}` when `layer != ""`. A `noDetect` func calls `t.Fatal`.
  - `TestHarnessTokensMatchHarnessOrder`: `reflect.DeepEqual(config.AgentHarnessTokens(), harness.Order)`.
  - `TestResolveHarnessChoice` sub-tests (input -> expected):
    1. flag `[claude]`, with a chooser and `repoDeclared` -> decided `[claude]`, 0 chooser calls.
    2. init, `repoDeclared`, cfg repository `[claude]` -> undecided, no warning, 0 calls.
    3. configure, `repoDeclared`, cfg repository `[claude cursor]`, display `.docket.yml`, chooser returns `[cursor]` -> `[cursor]`. The request is `Preselected=[claude cursor]`, `Options=harness.Order`, `ConfigPath=.docket.yml`.
    4. init, cfg global `[codex]`, with `noDetect` -> `Preselected=[codex]`.
    5. init, cfg unset, detect `[cursor]`, chooser `[]` -> `Preselected=[cursor]`; decided with a non-nil empty selection.
    6. chooser `[opencode claude]` -> `[claude opencode]`; chooser `[bogus]` -> error.
    7. chooser returns `ErrHarnessChoiceCancelled` -> `errors.Is` holds.
    8. init, no input, display `.git/dckt/config.yml` -> undecided. The warning names `` `agent_harnesses: [claude]` ``, the display, and `ConfigureHarnessesCommand`.
    9. configure, no input -> `errHarnessesFlagRequired`.
  - `TestParseHarnessesFlag`:
    - unset -> `nil, nil`
    - `Set` + `[none]` -> non-nil empty
    - `Set` + `[bogus]` -> refusal `ResultInvalidInput` whose text contains `bogus`
  - `TestDetectHarnessesReadsTheHome`:
    - Set `HOME`/`XDG_CONFIG_HOME` to `testsupport.TempDir`; the empty home -> empty.
    - Create `$HOME/.cursor` and `$XDG_CONFIG_HOME/opencode` -> `[cursor opencode]`.
  - `TestRepositoryOpResultHarnessFields`:
    - `AgentHarnesses: &[]string{}` with `Warnings: ["w"]` marshals `"agent_harnesses":[]` and `"warnings":["w"]`.
    - The zero value marshals neither.
  - `TestAppendPendingDedupes`: `appendPending([.gitignore], "", .docket.yml, .gitignore, .docket.yml)` -> `[.gitignore .docket.yml]`.

- [ ] **Step 2: Run and confirm they fail.** `go test ./internal/app/ -run 'HarnessTokensMatch|ResolveHarnessChoice|ParseHarnessesFlag|DetectHarnesses|RepositoryOpResultHarnessFields|AppendPending' -count=1`.

- [ ] **Step 3: Implement.**
  - In `RepositoryOpResult` (after `Findings`), add `AgentHarnesses *[]string \`json:"agent_harnesses,omitempty"\`` (the selection written or confirmed this run; `[]` is a recorded "no agents") and `Warnings []string \`json:"warnings,omitempty"\``.
  - Move `mapSurfaceFailure`'s body into `mapSurfaceFailureFor(operation string, state reposetup.State, err error)`, using `operation` wherever it now says `OperationRepositoryInit`. Keep `mapSurfaceFailure(state, err)` as a one-line delegate.

  `repository_harnesses.go`. The file comment explains the two halves and that `installAuthorizedSurfaces` re-reads config from disk, so a key written in this run authorizes this run (learning predicate-must-ask-the-post-pass-state). Imports: context, errors, fmt, os, slices, config, gitcli, harness, install, layout, reposetup. Give every name a doc comment.

  Small helpers, as specified:
  - Types and sentinels as in Interfaces. `ErrHarnessChoiceCancelled` reads "the harness choice was cancelled"; `errHarnessesFlagRequired` reads "no --harnesses value and no terminal to show the checklist on".
  - `parseHarnessesFlag`: unset -> `nil, nil`; otherwise `ParseHarnessSelection`, with an error -> an `invalid-input` result "<op>: invalid-input: <err>".
  - `harnessRepoDeclared`: `Explicit && isRepositoryLayer(Provenance.Layer)`, the `SurfacesAuthorized` predicate.
  - `harnessPreselection(cfg, detect)`: a copy of `cfg.AgentHarnesses.Value` when `Explicit`; else `detect()`, or `[]string{}` when `detect` is nil.
  - `detectHarnesses()`: `install.ResolveRoots(os.UserHomeDir, os.Getenv)`, with an error -> `[]string{}`. Then the `p.Name` of each `Planners(roots, config.AgentsTable{})` entry whose `p.Detect(roots)` reports present.
  - `harnessConfigError{display, err}`: `Error()` is "writing agent_harnesses to <display>: <err>", plus `Unwrap`.
  - `harnessApplyFailure`: a `*harnessConfigError` -> `repositoryInternalFailure(op, state, "writing agent_harnesses to "+display, inner)`; anything else -> `mapSurfaceFailureFor`.
  - `harnessChoiceCancelled`: `ResultInterrupted`, "<op>: interrupted: the harness choice was cancelled; nothing was written".
  - `appendPending`: skips "" and duplicates.
  - `setHarnessResult`: when decided, `AgentHarnesses = &copy`. Sets `Warnings`, and appends "\nwarning: "+w to `human` for each.

  The two halves:

```go
type harnessPolicy int

const (
	harnessPolicyInit      harnessPolicy = iota // keep a repository value; warn when it cannot ask
	harnessPolicyConfigure                      // always ask; refuse when it cannot
)

type harnessChoiceInput struct {
	flag          []string // parsed --harnesses; nil when not passed
	chooser       HarnessChooser
	policy        harnessPolicy
	cfg           config.Effective
	repoDeclared  bool
	configDisplay string
	detect        func() []string
}

type harnessChoice struct {
	decided   bool
	selection []string // non-nil when decided
	warning   string
}

func resolveHarnessChoice(ctx context.Context, in harnessChoiceInput) (harnessChoice, error) {
	if in.flag != nil {
		return harnessChoice{decided: true, selection: in.flag}, nil
	}
	if in.repoDeclared && in.policy == harnessPolicyInit {
		return harnessChoice{}, nil
	}
	if in.chooser != nil {
		got, err := in.chooser(ctx, HarnessChoiceRequest{
			Options: append([]string(nil), harness.Order...), Preselected: harnessPreselection(in.cfg, in.detect), ConfigPath: in.configDisplay,
		})
		if err != nil {
			return harnessChoice{}, err
		}
		sel := []string{}
		if len(got) > 0 {
			if sel, err = reposetup.ParseHarnessSelection(got); err != nil {
				return harnessChoice{}, fmt.Errorf("the harness chooser returned an invalid selection: %w", err)
			}
		}
		return harnessChoice{decided: true, selection: sel}, nil
	}
	if in.policy == harnessPolicyInit {
		return harnessChoice{warning: fmt.Sprintf("agent_harnesses is not set, so docket wrote no agent dispatch instructions; add a line such as `%s` to %s, or run `%s`",
			reposetup.AgentHarnessesLine([]string{"claude"}), in.configDisplay, reposetup.ConfigureHarnessesCommand)}, nil
	}
	return harnessChoice{}, errHarnessesFlagRequired
}

type harnessApplied struct {
	pendingConfig  string
	wroteConfig    bool
	surfacePending []string
	wroteSurfaces  bool
	warnings       []string
}

// applyHarnessChoice writes a decided selection, ALWAYS refreshes the surfaces
// (a kept value still installs them; dropped harnesses lose theirs), and warns
// when .docket.local.yml overrides the write — keyed on provenance.
func applyHarnessChoice(ctx context.Context, git *gitcli.Client, sc setupContext, choice harnessChoice) (harnessApplied, error) {
	var out harnessApplied
	_, display, _ := repoConfigTarget(sc)
	if choice.decided {
		pending, wrote, err := writeTargetConfig(sc, func(existing []byte) ([]byte, error) {
			edited, changed, rerr := reposetup.RenderAgentHarnessesEdit(existing, choice.selection)
			if rerr != nil || !changed {
				return nil, rerr
			}
			return edited, nil
		})
		if err != nil {
			return out, &harnessConfigError{display: display, err: err}
		}
		out.pendingConfig, out.wroteConfig = pending, wrote
	}
	var err error
	if out.surfacePending, out.wroteSurfaces, err = installAuthorizedSurfaces(ctx, git, sc.repo.PrimaryWorktree); err != nil {
		return out, err
	}
	if choice.warning != "" {
		out.warnings = append(out.warnings, choice.warning)
	}
	if choice.decided && sc.layout.Mode != layout.Private {
		eff, rerr := resolveSetupConfig(sc.repo.PrimaryWorktree, sc.defaultBranch)
		if rerr != nil {
			return out, &harnessConfigError{display: display, err: rerr}
		}
		if eff.AgentHarnesses.Provenance.Layer == config.LayerRepositoryLocal {
			out.warnings = append(out.warnings, fmt.Sprintf("`.docket.local.yml` also sets agent_harnesses and overrides %s in this clone; the applied value is `%s`",
				display, reposetup.FormatHarnessList(eff.AgentHarnesses.Value)))
		}
	}
	return out, nil
}
```

- [ ] **Step 4: Run and confirm they pass** (plus `go build ./... && go vet ./internal/app/`). Mutation-check each from a backup copy:
  1. Drop `&& in.policy == harnessPolicyInit`. Sub-test 3 goes red.
  2. Make `harnessPreselection` return `detect()` unconditionally. Sub-test 4 goes red.

- [ ] **Step 5: Commit.**

`git add` exactly the files under **Files**, then `git commit -m "feat(app): add the shared choose-harnesses step for init and configure-harnesses"`.

---

### Task 4: `repository init` chooses harnesses (shared and private)

**Build tier:** premium. Init's effect order and the private-layout trap are easy to get subtly wrong.

**Files:**
- Modify: `internal/app/repository_init.go`, `internal/app/repository_init_private.go`
- Test: `internal/app/reposetup_harnesses_integration_test.go` (new, `//go:build integration`)

**Interfaces:**
- Consumes: Task 3.
- Produces:
  - `InitOptions.Harnesses HarnessesOptions`
  - `runPrivateInit(..., debris, choice harnessChoice)`, with the new trailing parameter
  - test helpers `harnessFlag(tokens ...string) HarnessesOptions` (`Set: true`) and `(r *initRepo) runInitInHome(t, o InitOptions, homeDirs ...string) RepositoryOpResult`. The latter calls `pinInitUserRoots`, creates each `$HOME/<dir>`, then `RunRepositoryInit` with `newGitClient(t)`.

- [ ] **Step 1: Write the failing integration tests** (prefix `TestIntegrationRepoSetup`). Existing helpers: `newInitRepo`, `defaultSetupYML`, `runInitWith`, `runInitWithGlobal`, `newPrivateInitRepo`, `privateLayoutOf`, `mustReadFile`, `contains`, `runGit`, `remoteBranchExists`.

  - **`…InitHarnessesFlagShared`** (AC2): `newInitRepo(t, defaultSetupYML, nil)`; `runInitWith(t, InitOptions{Harnesses: harnessFlag("claude")})` -> applied. Assert:
    - `.docket.yml` contains `"\nagent_harnesses: [claude]\n"`, and `CLAUDE.md` contains `docket:dispatch`, both from this run.
    - `PendingPaths` include `.docket.yml`, `CLAUDE.md`, and `.gitignore`.
    - `*res.AgentHarnesses == [claude]`.
    - `git diff --cached --name-only` is empty.

  Also add:
  - **`…PrivateInitHarnessesFlag`**:
    - `r, data := newPrivateInitRepo(t, nil)`; init with `Private: true, Harnesses: harnessFlag("claude")` -> applied.
    - With `lay := privateLayoutOf(t, r.invocation, data)`, `lay.ConfigPath` contains `agent_harnesses: [claude]\n`, and `filepath.Join(filepath.Dir(lay.ConfigPath), "AGENTS.md")` contains `docket:dispatch`.
    - `.docket.yml` does **not** exist in `r.invocation`; `PendingPaths` is empty; `git status --porcelain` is empty.
  - **`…InitInteractivePrecheckGlobal`**:
    - `runInitWithGlobal(t, "agent_harnesses: [codex]\n", InitOptions{Harnesses: HarnessesOptions{Chooser: recordingChooser(&req, &calls, []string{"cursor"}, nil)}})` on `newInitRepo(t, defaultSetupYML, nil)`.
    - Expect `calls == 1`, `req.Preselected == [codex]`, `req.ConfigPath == ".docket.yml"`, `.docket.yml` holding `agent_harnesses: [cursor]`, and `.cursor/rules/docket-dispatch.mdc` existing.
  - **`…InitInteractivePrecheckDetected`**:
    - `runInitInHome(t, InitOptions{Harnesses: HarnessesOptions{Chooser: recordingChooser(&req, &calls, []string{}, nil)}}, ".cursor")`.
    - Expect `req.Preselected == [cursor]`, `.docket.yml` holding `agent_harnesses: []`, an empty non-nil `*res.AgentHarnesses`, and no `CLAUDE.md`.
  - **`…InitInteractiveCancelWritesNothing`**: the chooser returns `ErrHarnessChoiceCancelled` -> `ResultInterrupted`, no remote `docket` branch, no `.docket` directory, no valid managed `.gitignore` block, and `.docket.yml` byte-identical.
  - **`…InitNoInputWarns`** (AC5): `runInitWith(t, InitOptions{})` -> applied. The text contains `` `agent_harnesses: [claude]` ``, `.docket.yml`, and `reposetup.ConfigureHarnessesCommand`; `len(res.Warnings) == 1`; `.docket.yml` has no `agent_harnesses`; there is no `CLAUDE.md`.
  - **`…InitKeepsDeclaredValue`** (AC6): `newInitRepo(t, defaultSetupYML+"agent_harnesses: [codex]\n", nil)` with a `t.Error` chooser -> applied. `agent_harnesses: [codex]` appears exactly once, `AGENTS.md` has `docket:dispatch`, `AgentHarnesses == nil`, and there are no warnings.
  - **`…InitRerunDoesNotReask`**: init `--harnesses claude` without committing, then re-run with a `t.Error` chooser -> not called, one `agent_harnesses` line.
  - **`…InitRefusesBadHarnessesBeforeAnyWrite`**:
    - For each of `harnessFlag("bogus","claude")`, `("claude","claude")`, `("none","claude")`, and `("")`: `ResultInvalidInput`, no remote `docket` branch, `.docket.yml` byte-identical.
    - The first case's text names `bogus`.

- [ ] **Step 2: Run and confirm they fail.** `go test -tags integration -count=1 -run 'TestIntegrationRepoSetup(Init|PrivateInit)' ./internal/app/`.

- [ ] **Step 3: Implement.**
  1. `InitOptions` gains `Harnesses HarnessesOptions` ("--harnesses and the CLI's interactive chooser").
  2. `RunRepositoryInit`, right after the `--private`/`--shared` check: `flagSel, refusal := parseHarnessesFlag(OperationRepositoryInit, o.Harnesses)`; return `*refusal` if non-nil.
  3. Right after `decideInitMode`'s refusal check and **before** `if mode == layout.Private`:

```go
	// Resolved before any metadata, config, or working-tree write, so a
	// cancelled picker leaves nothing behind. A fresh repository set up private
	// reads only the private config this run creates.
	repoDeclared := harnessRepoDeclared(sc.cfg)
	_, configDisplay, _ := repoConfigTarget(sc)
	if mode == layout.Private && sc.layout.Mode != layout.Private {
		repoDeclared, configDisplay = false, layout.PrivateConfigDisplay
	}
	choice, cerr := resolveHarnessChoice(ctx, harnessChoiceInput{flag: flagSel, chooser: o.Harnesses.Chooser, policy: harnessPolicyInit,
		cfg: sc.cfg, repoDeclared: repoDeclared, configDisplay: configDisplay, detect: detectHarnesses})
	if errors.Is(cerr, ErrHarnessChoiceCancelled) {
		return harnessChoiceCancelled(OperationRepositoryInit, cls.State)
	}
	if cerr != nil {
		return repositoryInternalFailure(OperationRepositoryInit, cls.State, "choosing agent harnesses", cerr)
	}
```

     Pass `choice` to `runPrivateInit`.
  4. Replace the `wroteSurfaces := false / if facts.SurfacesAuthorized {…}` block with:
     - `applied, herr := applyHarnessChoice(ctx, d.Git, sc, choice)`
     - on error -> `harnessApplyFailure(OperationRepositoryInit, cls.State, herr)`
     - `pending = appendPending(pending, applied.pendingConfig); pending = appendPending(pending, applied.surfacePending...)`

     Route `docketYMLPending` and `debris.pending()` through `appendPending` too. The applied condition gains `applied.wroteConfig || applied.wroteSurfaces` (drop `wroteSurfaces`). Call `setHarnessResult(&out, choice, applied)` after the discovery note. Update the "Effect 5" comment: surfaces are authorized from the post-write config.
  5. Doc comment of `RunRepositoryInit`:
     - "It never prompts and never reads stdin." becomes "It never reads stdin itself; its only interaction is the harness chooser the CLI supplies on a terminal, asked before any write."
     - "(only when authorized) the parent-facing dispatch surfaces" becomes "the chosen agent_harnesses and the dispatch surfaces it authorizes".
  6. `runPrivateInit` step 8 replaces `installAuthorizedSurfaces` with:

```go
	// The layout resolved in step 3: a fresh repository's gather-time layout is
	// still shared and would point the write at .docket.yml.
	psc := sc
	psc.layout = lay
	applied, herr := applyHarnessChoice(ctx, d.Git, psc, choice)
	if herr != nil {
		return fail(harnessApplyFailure(OperationRepositoryInit, cls.State, herr))
	}
	changed = changed || applied.wroteConfig || applied.wroteSurfaces
```

     Call `setHarnessResult(&out, choice, applied)` after `out.human` is complete.

- [ ] **Step 4: Run and confirm they pass,** then run the regressions: `go test -tags integration -count=1 -run TestIntegrationRepoSetup ./internal/app/` and `go test -count=1 ./internal/app/`. Mutation-check each from a backup copy:
  1. Pass `sc` instead of `psc`. The private test goes red on `.docket.yml`.
  2. Move the resolve block below `publishOrAdoptMetadataRoot`. The cancel test goes red.

- [ ] **Step 5: Commit.**

`git add` exactly the files under **Files**, then `git commit -m "feat(repository): choose agent harnesses during init and write them in the same run"`.

---

### Task 5: `repository configure-harnesses` (app operation)

**Build tier:** standard

**Files:**
- Create: `internal/app/repository_configure_harnesses.go`, `internal/app/repository_configure_harnesses_test.go`
- Modify: `internal/app/schema_registry.go`, `internal/app/reposetup_harnesses_integration_test.go` (append)

**Interfaces:**
- Consumes: Tasks 3–4, plus `GatherSetupFacts`, `augmentCheckFacts`, `repositoryGatherFailure`, `repositoryExternalFailure`, `docketManagedWorktreePaths`, and `healthyConfigureFacts` (in `repository_configure_tests_test.go`).
- Produces:
  - `OperationRepositoryConfigureHarnesses = "repository.configure-harnesses"`
  - `ConfigureHarnessesOptions{Harnesses HarnessesOptions}`
  - `RunRepositoryConfigureHarnesses(ctx, d SetupDeps, o) OperationResult` (a `RepositoryOpResult`)
  - `configureHarnessesGuard(facts) (reposetup.Classification, *RepositoryOpResult)`
  - `workingTreePendingPaths(ctx, git *gitcli.Client, sc setupContext) ([]string, error)`
  - test helpers `(r *initRepo) runConfigureHarnesses(t, o) RepositoryOpResult` (calls `pinInitUserRoots`, `newGitClient`, and type-asserts) and `newHarnessRepo(t, tokens ...string) *initRepo`. The latter uses `newInitRepo(t, "integration_branch: main\n", nil)`, runs init with `harnessFlag(tokens...)` (which must be applied), and runs `commitAndPushMain` over `res.PendingPaths`.

- [ ] **Step 1: Write the failing tests.**

  `TestConfigureHarnessesGuard` (unit, from `healthyConfigureFacts()`):
  - admitted:
    - unchanged
    - `PendingReviewPaths = [.docket.yml]`
    - `PrimaryClean = Absent`
    - `PrimaryAtRemoteTip = Absent`
  - refused `ResultInvalidState`, with the text naming the listed command:
    - `RemoteMetadata` Absent -> `docket repository init`
    - `RemoteMetadata` Absent with `LiveSurface` Present -> `docket repository migrate`
    - `DocketWorktree.Foreign = true` -> `docket repository check`
    - `DocketWorktree.HooksOff = Absent` -> `docket repository check`

  Integration tests (prefix `TestIntegrationRepoSetup`):

  - **`…ConfigureHarnessesDropsAHarness`** (AC7):
    - `r := newHarnessRepo(t, "claude", "cursor")`; check that `.cursor/rules/docket-dispatch.mdc` exists (fixture sanity).
    - `--harnesses claude` -> applied, `needs-review`. The `.mdc` is gone, `CLAUDE.md` still has `docket:dispatch`, and `PendingPaths` include `.docket.yml` and `.cursor/rules/docket-dispatch.mdc`.
    - Then `--harnesses none` -> applied. `CLAUDE.md`, if present, has no `docket:dispatch`. `.docket.yml` has exactly one `agent_harnesses:` line, and it is `agent_harnesses: []`.

  Also add:
  - **`…ConfigureHarnessesNoPreconditions`** (AC8). Each case runs `--harnesses codex` and expects a result that is not `invalid-state`:
    - dirty: `newHarnessRepo(t, "claude")` plus a modified `README.md`
    - behind: `newHarnessRepo(t, "claude")`; then `runGit(t, r.writer, "pull", "-q", "--ff-only", "origin", "main")` (the writer clone is stale after `commitAndPushMain`); then `r.advanceIntegration(t, "later.txt", "x\n")`; no fetch in the invocation clone
    - needs-review: `newInitRepo` with init and no flag, nothing committed
  - **`…ConfigureHarnessesRefusesFreshAndLegacy`**: an uninitialized `newInitRepo` is refused naming `docket repository init`. One with integration file `docs/changes/active/0001-x.md` (`---\nid: 1\n---\n`) is refused naming `docket repository migrate`. `.docket.yml` is unchanged in both.
  - **`…ConfigureHarnessesNoFlagNoTerminalRefuses`** (AC9): `ConfigureHarnessesOptions{}` gives `ResultInvalidInput`, the text names `--harnesses`, and `.docket.yml` is unchanged.
  - **`…ConfigureHarnessesRefusesBadTokens`** (AC10): `claude,claude`, `none,claude`, and `bogus` each give `ResultInvalidInput` with `.docket.yml` unchanged.
  - **`…ConfigureHarnessesPrechecksRepoValue`**: on `newHarnessRepo(t,"claude","cursor")`, a recording chooser returning `[claude]` sees `Preselected == [claude cursor]`, and `.docket.yml` ends up `[claude]`.
  - **`…ConfigureHarnessesTwiceInNeedsReview`**: on `newHarnessRepo(t, "cursor")`, run `--harnesses claude` (applied, `needs-review`), then again: admitted, `no-op`, and one `agent_harnesses` line.
  - **`…ConfigureHarnessesLocalOverrideWarns`** (AC11, both halves):
    - On `newHarnessRepo(t,"claude")`, write `.docket.local.yml` = `agent_harnesses: [cursor]\n`, then run `--harnesses codex`.
    - Expect applied and one warning containing `.docket.local.yml` and `` `[cursor]` ``; `.docket.yml` has `[codex]`.
    - Expect the **applied** surfaces to follow cursor: `.cursor/rules/docket-dispatch.mdc` exists and `AGENTS.md` has no `docket:dispatch`.
  - **`…PrivateConfigureHarnesses`**: on `initPrivateHealthy(t)`, `--harnesses claude` gives applied, no pending paths, state `healthy`, the private config line, the private `AGENTS.md` block, and text naming `.git/dckt/config.yml`.

- [ ] **Step 2: Run and confirm they fail.** `go test -count=1 -run ConfigureHarnessesGuard ./internal/app/` and `go test -tags integration -count=1 -run 'TestIntegrationRepoSetup(Private)?ConfigureHarnesses' ./internal/app/`.

- [ ] **Step 3: Implement** `repository_configure_harnesses.go`. Define the constant and options type as in Interfaces. The doc comment says the command changes `agent_harnesses` through init's shared step, needs no clean checkout or tip, and admits `needs-review`.

  `RunRepositoryConfigureHarnesses`, in this order:
  1. `!o.Harnesses.Set && o.Harnesses.Chooser == nil`: before any read, return `invalid-input` with "<op>: invalid-input: there is no terminal to show the checklist on; pass --harnesses <list> (for example --harnesses claude,cursor), or --harnesses none to record that no agents are used".
  2. `parseHarnessesFlag(op, o.Harnesses)`; on a refusal, return it.
  3. `GatherSetupFacts(ctx, d, true)` (on error, `repositoryGatherFailure`). When the remote metadata branch is present, run `augmentCheckFacts`, as configure-tests does. Then apply `configureHarnessesGuard`.
  4. `resolveHarnessChoice` with `harnessPolicyConfigure`, `repoDeclared: harnessRepoDeclared(sc.cfg)`, `configDisplay` from `repoConfigTarget(sc)`, and `detect: detectHarnesses`. `ErrHarnessChoiceCancelled` -> `harnessChoiceCancelled`; any other error -> `repositoryInternalFailure(op, state, "choosing agent harnesses", err)`.
  5. `applyHarnessChoice`; on error, `harnessApplyFailure`. Then `workingTreePendingPaths`; on error, `repositoryExternalFailure(op, state, "listing the pending review paths", err)`.
  6. Result is `applied` when `wroteConfig || wroteSurfaces`, else `no-op`. State is `cls.State`, raised to `needs-review` when pending paths exist and the state was `healthy`. Fill `PendingPaths` and `SourceRevision`.
  7. Human text, with `list` = `` `[claude]` ``:
     - no-op: "<op>: no-op (<state>): agent_harnesses is already <list> and its instructions are current; nothing to write"
     - private (`display == layout.PrivateConfigDisplay`): "agent harnesses set to <list> (<state>); wrote .git/dckt/config.yml"
     - otherwise: "agent harnesses set to <list> (<state>); review and commit the pending paths: <comma list>"

     Then `setHarnessResult(&out, choice, applied)`.

```go
// configureHarnessesGuard: healthy, needs-review, or either once init's
// clean-checkout and at-tip preconditions are set aside. Pure.
func configureHarnessesGuard(facts reposetup.Facts) (reposetup.Classification, *RepositoryOpResult) {
	cls := reposetup.Classify(facts)
	refuse := func(remedy string) (reposetup.Classification, *RepositoryOpResult) {
		out := newRepositoryOpResult(OperationRepositoryConfigureHarnesses, ResultInvalidState, RepositoryOpResult{RepositoryState: string(cls.State)})
		out.human = fmt.Sprintf("%s: %s (%s): %s", OperationRepositoryConfigureHarnesses, ResultInvalidState, cls.State, remedy)
		return cls, &out
	}
	switch cls.State {
	case reposetup.StateHealthy, reposetup.StateNeedsReview:
		return cls, nil
	case reposetup.StateFresh:
		return refuse("repository is not initialized; run `docket repository init` (it takes --harnesses too)")
	case reposetup.StateLegacy:
		return refuse("repository has a legacy single-branch layout; run `docket repository migrate`, then re-run `" + reposetup.ConfigureHarnessesCommand + "`")
	}
	relaxed := facts
	relaxed.PrimaryClean, relaxed.PrimaryAtRemoteTip = reposetup.PresencePresent, reposetup.PresencePresent
	if s := reposetup.Classify(relaxed).State; s == reposetup.StateHealthy || s == reposetup.StateNeedsReview {
		return cls, nil
	}
	return refuse("repository is not in a healthy state; run `docket repository check` and resolve the reported findings first")
}
```

  `workingTreePendingPaths` returns `nil` for a private layout. Otherwise it reads `git.ChangedPaths(ctx, sc.repo.PrimaryWorktree)`, keeps each path in `docketManagedWorktreePaths` (through `appendPending`), and sorts the result.

  If `ChangedPaths` omits a deletion, fix the listing (never the assert) and say why in the commit. In `schema_registry.go`, insert `{ID: "repository.configure-harnesses", Request: nil, Result: RepositoryOpResult{}}, // RunRepositoryConfigureHarnesses` in id order.

- [ ] **Step 4: Run and confirm they pass** (Step 2 commands, then `go test -count=1 ./internal/app/`). A catalog-completeness test may stay red until Task 6, but only for the missing CLI command. Never loosen it. Mutation-check each from a backup copy:
  1. Delete the `relaxed` block. The dirty and behind admits go red.
  2. Change `config.LayerRepositoryLocal` to `config.LayerRepository` in `applyHarnessChoice`. The override test goes red.

- [ ] **Step 5: Commit.**

`git add` exactly the files under **Files**, then `git commit -m "feat(repository): add configure-harnesses to change a repository's agent harnesses later"`.

---

### Task 6: CLI: `--harnesses`, `configure-harnesses`, and the huh picker

**Build tier:** standard

**Files:**
- Modify: `go.mod`, `go.sum`, `internal/cli/repository.go`, `internal/cli/install.go`, `internal/cli/repository_test.go`
- Create: `internal/cli/harness_picker.go`, `internal/cli/harness_picker_test.go`

**Interfaces:**
- Consumes: `app.HarnessesOptions`, `app.HarnessChooser`, `app.HarnessChoiceRequest`, `app.ErrHarnessChoiceCancelled`, `app.InitOptions.Harnesses`, `app.ConfigureHarnessesOptions`, `app.RunRepositoryConfigureHarnesses`.
- Produces:
  - `repositoryConfigureHarnessesRunner`
  - seams: `repositoryHarnessesInteractive func() bool` (stdin **and** stdout are character devices), `repositoryHarnessChooser app.HarnessChooser = huhHarnessChooser`, and `runHarnessForm func(ctx context.Context, f *huh.Form) error`
  - `newHarnessForm(req, picked *[]string) *huh.Form`
  - `harnessesOptions(c *cobra.Command) app.HarnessesOptions`

- [ ] **Step 1: Add the dependency.** Run `go get github.com/charmbracelet/huh@v1.0.0` and then `go mod tidy` once Step 4 imports it. The `go` directive stays `go 1.26.0`.

- [ ] **Step 2: Write the failing tests.**

  `harness_picker_test.go`. huh's accessible mode reads one number per line: `n` toggles option n and `0` confirms. Feed input through `iotest.OneByteReader` so each prompt reads only its own line.

```go
func harnessRequest(pre ...string) app.HarnessChoiceRequest {
	return app.HarnessChoiceRequest{Options: []string{"claude", "codex", "cursor", "opencode"}, Preselected: pre, ConfigPath: ".docket.yml"}
}

func TestHarnessFormPrechecksAndToggles(t *testing.T) {
	var picked []string
	f := newHarnessForm(harnessRequest("cursor"), &picked)
	in := iotest.OneByteReader(strings.NewReader("1\n0\n"))
	if err := f.WithAccessible(true).WithInput(in).WithOutput(io.Discard).Run(); err != nil {
		t.Fatal(err)
	}
	slices.Sort(picked)
	if !slices.Equal(picked, []string{"claude", "cursor"}) {
		t.Fatalf("picked = %v, want [claude cursor]", picked)
	}
}
```

  Also add:
  - **`TestHarnessFormConfirmWithNothingChecked`**: no pre-checks, input `"0\n"` -> `picked` empty.
  - **`TestHuhHarnessChooserMapsAbortToCancel`**: set `runHarnessForm` to return `huh.ErrUserAborted` -> `errors.Is(err, app.ErrHarnessChoiceCancelled)`.
  - **`TestHuhHarnessChooserReturnsNonNilEmpty`**: `runHarnessForm` returns nil, the request has no pre-checks -> a non-nil empty result.

  If accessible mode hangs on huh v1.0.0, keep the seam tests and replace the form tests with one asserting that `newHarnessForm(harnessRequest("cursor"), &picked)` seeds `picked == [cursor]`. Note the reason in the commit.

  `repository_test.go`:
  - add `"configure-harnesses"` to `TestRepositoryCommandsRegistered`.
  - add **`TestRepositoryHarnessesFlagAndChooserFlow`**:
    - Stub `repositoryConfigureHarnessesRunner` (capture `o`, return `fakeSyncResult{…ResultNoOp}`) and `repositoryHarnessesInteractive` (a test-controlled bool); restore both.
    - Assert:
      - `--harnesses claude,cursor --harnesses codex` -> Set, tokens `claude|cursor|codex`, no chooser
      - `--harnesses ""` -> Set, non-nil tokens
      - non-interactive with no flag -> unset, no chooser
      - interactive -> a chooser
      - interactive with `--json` -> no chooser
      - interactive with `--harnesses none` -> no chooser
  - add **`TestRepositoryInitHarnessesFlagFlows`**: stub `repositoryInitRunner`; `init --harnesses claude --harnesses cursor` -> `o.Harnesses.Set` and tokens `[claude cursor]`.
  - add **`TestRepositoryHarnessesHelp`**: `--help` on `init` and on `configure-harnesses` mentions `--harnesses` and `none`.
  - add an assert that `assetIndependent["repository configure-harnesses"]` is true.

- [ ] **Step 3: Run and confirm they fail.** `go test -count=1 -run 'HarnessForm|HuhHarnessChooser|RepositoryHarnesses|RepositoryInitHarnesses|RepositoryCommandsRegistered' ./internal/cli/`.

- [ ] **Step 4: Implement.** `harness_picker.go`, whose file comment says it is docket's only importer of huh:

```go
var runHarnessForm = func(ctx context.Context, f *huh.Form) error { return f.RunWithContext(ctx) }

// newHarnessForm: every harness in req.Options order, req.Preselected
// pre-checked (huh selects the options already in *picked).
func newHarnessForm(req app.HarnessChoiceRequest, picked *[]string) *huh.Form {
	*picked = append([]string{}, req.Preselected...)
	field := huh.NewMultiSelect[string]().
		Title("Which coding agents should get docket's instructions in this repository?").
		Description("Space toggles, Enter confirms. Written to " + req.ConfigPath + "; confirm with nothing checked to record that no agents are used.").
		Value(picked).
		Options(huh.NewOptions(req.Options...)...)
	return huh.NewForm(huh.NewGroup(field))
}

func huhHarnessChooser(ctx context.Context, req app.HarnessChoiceRequest) ([]string, error) {
	var picked []string
	err := runHarnessForm(ctx, newHarnessForm(req, &picked))
	if errors.Is(err, huh.ErrUserAborted) {
		return nil, app.ErrHarnessChoiceCancelled
	}
	if err != nil {
		return nil, err
	}
	if picked == nil {
		picked = []string{}
	}
	return picked, nil
}
```

  `repository.go`:
  - Add the runner var `repositoryConfigureHarnessesRunner` (calls `app.RunRepositoryConfigureHarnesses`) beside `repositoryConfigureTestsRunner`.
  - Beside `repositoryConfirmInteractive`, add the seams:
    - `repositoryHarnessesInteractive`: stdin **and** stdout are character devices (an `isCharDevice` helper).
    - `repositoryHarnessChooser app.HarnessChooser = huhHarnessChooser`.
    - The flag reader:

```go
// harnessesOptions reads --harnesses by Changed (an explicit empty value must
// reach the app, which refuses it) and supplies the picker only when no flag
// was given, --json is off, and the session is interactive.
func harnessesOptions(c *cobra.Command) app.HarnessesOptions {
	var o app.HarnessesOptions
	if c.Flags().Changed("harnesses") {
		o.Set = true
		if o.Tokens, _ = c.Flags().GetStringSlice("harnesses"); o.Tokens == nil {
			o.Tokens = []string{}
		}
	}
	if jsonMode, _ := c.Flags().GetBool("json"); !o.Set && !jsonMode && repositoryHarnessesInteractive() {
		o.Chooser = repositoryHarnessChooser
	}
	return o
}
```

  - In the init closure add `o.Harnesses = harnessesOptions(c)`. Register on init and on the new command:
    - `harnessesUsage` = "comma `list` of coding agents to write docket's instructions for (claude, codex, cursor, opencode), or none; without it, a terminal shows a checklist"
    - `Flags().StringSlice("harnesses", nil, harnessesUsage)`
  - New subcommand: `repositorySubcommand("configure-harnesses", "Choose which coding agents get docket's instructions in this repository, and refresh them", …, EffectLocalWrite)`, calling `repositoryConfigureHarnessesRunner(c.Context(), deps, app.ConfigureHarnessesOptions{Harnesses: harnessesOptions(c)})`. Add it to `AddCommand`.
  - Update the file-header comment (the CLI also owns the harness picker) and `repositorySubcommand`'s effects sentence.
  - If `--json` is not visible through `c.Flags()`, read it as `newRepositoryMigrateCommand` does. Never drop the subtest.
  - `install.go`: add `"repository configure-harnesses": true,` after `"repository configure-tests"`.

- [ ] **Step 5: Run and confirm they pass** (Step 3 command, `go test -count=1 ./internal/cli/ ./internal/app/ ./internal/repoguard/`, `go build ./...`). Task 5's completeness tests are green now.
  - Import boundary: `out="$(go list -deps ./internal/app/)"; grep -c charmbracelet <<<"$out"` prints `0`.
  - Mutation-check from a backup copy: drop `!jsonMode &&`. The `--json` subtest goes red.

- [ ] **Step 6: Commit.**

`git add` exactly the files under **Files**, then `git commit -m "feat(cli): add --harnesses, configure-harnesses, and a checkbox picker built on huh"`.

---

### Task 7: `repository check` warns `harnesses-unset`; fixtures and the Bash-upgrade guide follow

**Build tier:** standard

**Files:**
- Modify: `internal/reposetup/health.go`, `internal/reposetup/health_test.go`, `internal/app/repository_check.go`
- Fixtures: `internal/app/repocheck_integration_test.go` (`healthySetupYML`), `internal/app/repository_private_integration_test.go` (`initPrivateHealthy`), plus any found in Step 4
- Guide: `docs/release/upgrading-from-bash.md`, `internal/bashupgrade/registry_test.go`
- Create: `internal/app/repocheck_harnesses_integration_test.go` (`//go:build integration`, prefix `TestIntegrationRepoCheck`)

**Interfaces:**
- Consumes: Task 1 constants, `Facts.SurfacesAuthorized`, and the Task 4–5 test helpers.
- Produces: `reposetup.HarnessesUnsetCode = "harnesses-unset"` and `reposetup.HarnessesUnsetFinding(f Facts) *Finding`.

- [ ] **Step 1: Write the failing tests.**
  - **`TestHarnessesUnsetFinding`** (reposetup):
    - `Facts{}` -> code `HarnessesUnsetCode`, `SeverityWarning`; the message names `agent_harnesses`; the remedy contains `ConfigureHarnessesCommand` and `--harnesses none`.
    - `Facts{SurfacesAuthorized: true}` -> nil.
  - **`TestIntegrationRepoCheckHarnessesUnsetShared`**:
    1. `newInitRepo(t, "integration_branch: main\n", nil)`, `runInit`, commit and push `.gitignore` and `.docket.yml`.
    2. Check -> state `healthy`, has `harnesses-unset`, exit 1.
    3. `runConfigureHarnesses(--harnesses none)`, commit `.docket.yml` -> no finding, exit 0.
    4. `--harnesses claude`, commit its `PendingPaths` -> no finding, exit 0.
  - **`TestIntegrationRepoCheckHarnessesUnsetPrivate`**:
    1. `newPrivateInitRepo`, then `runInitWith(Private: true)` without harnesses, then configure-tests `--command true`.
    2. `checkIn` -> `healthy` with `harnesses-unset`.
    3. `--harnesses none` -> no finding, exit 0.

- [ ] **Step 2: Run and confirm they fail.** `go test -count=1 -run HarnessesUnsetFinding ./internal/reposetup/` and `go test -tags integration -count=1 -run TestIntegrationRepoCheckHarnessesUnset ./internal/app/`.

- [ ] **Step 3: Implement.** In `health.go` beside `TestConfigFinding`:

```go
// HarnessesUnsetCode: no repository-level layer declares agent_harnesses, so
// docket writes no dispatch instructions here. `[]` is a recorded decision and
// silences it.
const HarnessesUnsetCode = "harnesses-unset"

// HarnessesUnsetFinding explains; it never changes the classified state.
func HarnessesUnsetFinding(f Facts) *Finding {
	if f.SurfacesAuthorized {
		return nil
	}
	return &Finding{
		Code:     HarnessesUnsetCode,
		Severity: SeverityWarning,
		Message:  "docket writes no agent dispatch instructions in this repository because agent_harnesses is not set in its repository config.",
		Remedy:   "Run `" + ConfigureHarnessesCommand + "` to choose the coding agents, or `" + ConfigureHarnessesCommand + " --harnesses " + HarnessesNone + "` to record that none are used.",
	}
}
```

  In `RunRepositoryCheck`:
  - Inside the remote-metadata-present block, set `harnessesUnset = reposetup.HarnessesUnsetFinding(facts)`. Comment it as a setup gap reported like the test policy, never a state change.
  - Append it after `testConfig` when non-nil.

- [ ] **Step 4: Fix the fixtures. Declare the key; never weaken an assert.**
  1. Set `healthySetupYML = "integration_branch: main\nagent_harnesses: []\n"` and note in its comment that the healthy baseline records "no agents".
  2. `initPrivateHealthy`: init with `Harnesses: harnessFlag("none")`.
  3. Find the rest:

     ```sh
     files="$(git grep -l -e 'runCheck(t)' -e 'checkIn(t' -e 'RunRepositoryCheck(' -- 'internal/app/*_test.go')"
     ```

     Then run:

     ```sh
     for s in repocheck reposetup reposynccheck repoinplaceff repoownership repocontention reporepair reposetup_race workflow runverdict; do
       bash tests/test_go_integration_app_$s.sh
     done
     ```

     Fix each newly red healthy baseline with `agent_harnesses: []` in its YAML or `harnessFlag("none")` at init.
  4. **Bash-upgrade guide.** Neither saved case (`testdata/bash-upgrade/v0.9.*`) declares the key. In `docs/release/upgrading-from-bash.md` section 5:
     - Add a table row after `test-config-missing`: `` | `harnesses-unset` | No coding agents are chosen for this repository, so docket writes no instructions for them. | `docket repository configure-harnesses`, below. | ``
     - Between the `configure-tests` and `commit-config` steps, add prose and a step:
       - The prose: on a terminal the command shows a checklist. The command below records "none yet", because Bash docket's `CLAUDE.md` block would get in the way. After removing that block (the last step), run `docket repository configure-harnesses --harnesses claude` (or your agents) and commit.
       - The step: `<!-- upgrade-step: configure-harnesses -->` followed by a `sh` block of exactly `docket repository configure-harnesses --harnesses none`.
     - Change the `commit-config` message to `"Configure docket's test command and agents"`, and make its lead-in say it commits both.
     - Add `"configure-harnesses": runPlain,` to `stepRegistry` after `"configure-tests"`.
     - `bash tests/test_go_integration_bashupgrade.sh` must pass. Never drop the step.

- [ ] **Step 5: Run and confirm** (the unit packages, the Step 2 integration run, and every shard from Step 4). Mutation-check each from a backup copy:
  1. Make the finding always nil. The shared test and unit test go red.
  2. Drop the `SurfacesAuthorized` early return. The `[]` leg goes red.

- [ ] **Step 6: Commit,** naming every touched fixture file explicitly:

`git add` exactly the files under **Files**, then `git commit -m "feat(repository): warn harnesses-unset in repository check until agents are chosen"`.

---

### Task 8: Documentation for the new behavior

**Build tier:** economy

**Files:** `.docket.example.yml`, `skills/docket-convention/references/agent-layer.md`, `docs/reference/glossary.md`, `docs/reference/config-keys.md`, `docs/guide/where-the-metadata-lives.md`, and `internal/assets/embedded/**` (regenerated). Write for a reader new to docket: lead with what they get, no change numbers.

- [ ] **Step 1: Edit.**
  - **`.docket.example.yml`:** after "…the installer ignores a value in the global config.", add: "`docket repository init` asks for it (a checklist on a terminal, or `--harnesses claude,cursor` / `--harnesses none`), `docket repository configure-harnesses` changes it later and refreshes the instructions in the same run, and `docket repository check` warns `harnesses-unset` while it is not set. `agent_harnesses: []` records that no agents are used."
  - **`agent-layer.md`:** after the three-state list, add one paragraph using operation ids only: "`repository.init` asks for the value on a terminal or takes `--harnesses <list>|none`; the `repository.configure-harnesses` operation changes it later and refreshes the dispatch blocks in the same run, with no clean checkout needed; `repository.check` reports `harnesses-unset` while no repository-level value exists."
  - **`glossary.md`:**
    - In the setup block, add `docket repository configure-harnesses    # choose which coding agents get docket's instructions` and mention `--harnesses` in the init line's comment.
    - In the `agent_harnesses` entry, replace "Re-run the install after changing it." with "`docket repository init` asks for it; `docket repository configure-harnesses` changes it and refreshes the surfaces; `docket repository check` warns `harnesses-unset` while it is unset."
  - **`config-keys.md`:** the same three facts in the `agent_harnesses` paragraph.
  - **`where-the-metadata-lives.md`:** the init bullet adds that init asks which coding agents get docket's instructions (`--harnesses` answers up front). Add a bullet: **`docket repository configure-harnesses`** changes that choice any time, without a clean checkout, and leaves the `.docket.yml` edit unstaged.

- [ ] **Step 2: Regenerate and verify.** Run `go generate ./internal/assets/` and then `go test -count=1 ./internal/assets/ ./internal/repoguard/ ./internal/config/`. A red `TestCapabilitySurface` means a `docket repository …` spelling landed in `skills/`. Use the operation id; never raise an exemption count.

- [ ] **Step 3: Commit.**

`git add` exactly the files under **Files**, then `git commit -m "docs: describe choosing agent harnesses with init and configure-harnesses"`.
