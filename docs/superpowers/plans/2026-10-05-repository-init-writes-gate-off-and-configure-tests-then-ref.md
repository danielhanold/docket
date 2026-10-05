<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0526 — repository configure-tests takes the test command as input](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-05-0526-repository-init-writes-gate-off-and-configure-tests-then-ref.md)**
<!-- docket:backlink:end -->
# configure-tests takes the test command as input Implementation Plan

> **For agentic workers:** This plan is executed by `docket-build` (one tier worker per task under the `docket-build-task` contract, one full-suite gate at the end). Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `docket repository configure-tests --command "<cmd>"` sets both the build and finalize gates to `local` with the given command. Without `--command`, configure-tests (and init) say what discovery actually found and point at `--command`. Migrate's ambiguity refusal stops pointing at configure-tests.

**Architecture:** The byte-preserving `.docket.yml` planner in `internal/reposetup/testconfigedit.go` gets a second entry point, `RenderExplicitTestCommandEdit`. It shares the existing owner-block splice machinery (`planOwnerBlock`) through an extracted `renderOwnerPairs`, and its gate pair is NOT preserve-explicit, so the explicit command overwrites. A validator, `ExplicitTestCommand`, refuses empty, whitespace-only, `auto`, and line-breaking or control input. The renderer's `plainSafe` also learns to ask the YAML resolver whether a bare scalar reads back as a string. `internal/app`'s `RunRepositoryConfigureTests` takes a `ConfigureTestsOptions{Command *string}` that the CLI fills from a new `--command` flag. Its human text is built per discovery outcome by pure functions.

**Tech Stack:** Go, `go.yaml.in/yaml/v3`, cobra, `go test` (unit plus `-tags integration`).

**Spec:** `docs/superpowers/specs/2026-10-05-repository-init-writes-gate-off-and-configure-tests-then-ref-design.md` (on the `docket` metadata branch). Change record: `docs/changes/active/0526-repository-init-writes-gate-off-and-configure-tests-then-ref.md`.

## Global Constraints

- With `--command`, discovery is skipped entirely (no detector probes). The plan writes `gate: local` and `test_command: <cmd>` in BOTH `build:` and `finalize:`, and overwrites any existing gate value or different command.
- Posture is unchanged: healthy topology only (`configureTestsGuard`), a pending UNSTAGED `.docket.yml` edit in the primary worktree, never staged, never committed, state reported `needs-review` with the pending path.
- Idempotent: a file already carrying `gate: local` plus exactly `<cmd>` under both owners is `no-op`, names the command, and writes nothing.
- Input refused as `invalid-input` with nothing written: an empty value, a whitespace-only value, and `auto`. The command is trimmed of surrounding whitespace and written through the existing renderer, which owns the YAML quoting.
- No per-gate flags. No `--command` on `init` or `migrate`. init's `none → gate: "off"` policy stays. No new detector families. Gate semantics, evidence, and the `test-config-missing` trigger stay unchanged. The healthy-only guard stays unchanged.
- The capability catalog entry `repository.configure-tests` keeps its id and its `local-write` effect. Do not change the command's `Short` text.
- Do NOT edit any `skills/**` or `internal/assets/embedded/**` file. `internal/repoguard/capability_surface_test.go` pins `"docket repository configure-tests": 3` occurrences on the workflow surfaces, and the spec requires no skill edit.
- Docs describe only current behavior, with no change or PR citations (`docs/` pages, not historical plans or results).
- Cross-references in maintained source anchor on symbol names, never line numbers (AGENTS.md, ADR-0054).
- Every `go test` run in a mutation or re-verification step uses `-count=1`. A cached result is not evidence.
- Full suite at the gate: `go run ./cmd/docket development test` (the configured `build.test_command`), run from the worktree root. Read the budget report: `tests/test_go_integration_app_reposetup.sh` has a 15s parallel ceiling, and Task 4 adds two integration tests to that shard. Report any `BUDGET WATCH:` / `SERIAL CONFIRMED OVER BUDGET:` line for it in the results. Do not silently raise the ceiling.

## Review Focus

1. **A printed remedy that is invalid in the state that printed it** (learning printed-remedy-state-validity). init's notes print after init has left pending edits, and configure-tests refuses anything but `healthy`. So init's `--command` remedy must say to commit the pending paths first. Pinned in Task 3 (`TestInitTestDiscoveryNoteNoneNamesCommandAfterCommit`).
2. **The `none` no-op claiming "the gates are off" when they are not.** Discovery's `none` plan is preserve-explicit on `gate`, so a file whose gate is explicitly `local` with no command also reaches the `none` no-op. The message must report the resolved gates, not assume `off`. Pinned in Task 3 (`TestConfigureTestsDiscoveryTextNoneReportsResolvedGates`).
3. **A command that renders to YAML that does not read back as the same string** (learning validator-must-match-the-reader-it-feeds). Today `plainSafe` emits `123`, `1.5`, `2026-10-05`, or `0x10` bare, and they resolve to `!!int`/`!!float`/`!!timestamp`. The config schema's string leaf refuses those, so configure-tests would write a `.docket.yml` that bricks config resolution. A newline inside a double-quoted scalar silently folds to a space. Pinned in Task 1 (`TestExplicitCommandRoundTripsThroughConfigResolve` plus the control-character refusals).
4. **An explicit empty `--command ""` mistaken for "flag absent"**, which would silently run discovery instead of refusing. The CLI must key on `Flags().Changed("command")`, and the app on a non-nil pointer. Pinned in Task 3 (`TestRepositoryConfigureTestsCommandFlagFlows`, the `--command ""` case) and Task 4 (integration refusal).
5. **Validation running after a repository read.** Bad input must be refused before any gather or write. Pinned in Task 3 (`TestRunRepositoryConfigureTestsRefusesInvalidCommandBeforeGather`, which passes a nil Git client that would fail if reached).

---

### Task 1: Explicit-command planner, validator, and string-safe rendering (`internal/reposetup`)

**Files:**
- Modify: `internal/reposetup/testconfigedit.go`: add `ConfigureTestsCommandRemedy`, `ErrInvalidTestCommand`, `ExplicitTestCommand`, `RenderExplicitTestCommandEdit`, `explicitPairs`, `renderOwnerPairs`, `resolvesToStr`. Refactor `RenderTestConfigEdit` to call `renderOwnerPairs`. Extend `plainSafe`.
- Test: `internal/reposetup/testconfigedit_test.go` (append)

**Interfaces:**
- Consumes: the existing `planOwnerBlock`, `applySplices`, `appendBlocks`, `topLevelMapping`, `lineOffsets`, `kvPair`, `desiredPairs`, `isConfiguredCommand` (all in package `reposetup`), and `document.IllegalTextRune(r rune) bool` from `internal/document` (already imported elsewhere in `reposetup`).
- Produces:
  - `const ConfigureTestsCommandRemedy = "docket repository configure-tests --command \"<cmd>\""`
  - `var ErrInvalidTestCommand = errors.New("invalid test command")`
  - `func ExplicitTestCommand(raw string) (string, error)` returns the trimmed command, or an error wrapping `ErrInvalidTestCommand`.
  - `func RenderExplicitTestCommandEdit(existing []byte, cmd string) (edited []byte, changed bool, err error)`

- [ ] **Step 1: Write the failing tests**

Append to `internal/reposetup/testconfigedit_test.go`. Add `"errors"` and `"github.com/danielhanold/docket/internal/config"` to its imports (`bytes`, `strings`, `testing` are already there):

```go
// explicitWant is the exact owner-block text the explicit path writes for cmd.
func explicitBlock(owner, cmd string) string {
	return owner + ":\n  gate: local\n  test_command: " + cmd + "\n"
}

func TestExplicitEditNoFileRendersBothLocalGates(t *testing.T) { // (a)
	out, changed, err := RenderExplicitTestCommandEdit(nil, "sh ./test.sh")
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v, want a fresh file", changed, err)
	}
	want := explicitBlock("build", "sh ./test.sh") + explicitBlock("finalize", "sh ./test.sh")
	if string(out) != want {
		t.Fatalf("byte mismatch\n got: %q\nwant: %q", out, want)
	}
}

func TestExplicitEditOverwritesOffGates(t *testing.T) { // (b)
	existing := "# top comment\nintegration_branch: main  # inline\n" +
		"build:\n  gate: \"off\"\nfinalize:\n  gate: \"off\"\n"
	out, changed, err := RenderExplicitTestCommandEdit([]byte(existing), "sh ./test.sh")
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v, want an edit", changed, err)
	}
	want := "# top comment\nintegration_branch: main  # inline\n" +
		explicitBlock("build", "sh ./test.sh") + explicitBlock("finalize", "sh ./test.sh")
	if string(out) != want {
		t.Fatalf("byte mismatch\n got: %q\nwant: %q", out, want)
	}
}

func TestExplicitEditReplacesDifferentCommand(t *testing.T) { // (c)
	existing := explicitBlock("build", "make check") + explicitBlock("finalize", "make check")
	out, changed, err := RenderExplicitTestCommandEdit([]byte(existing), "sh ./test.sh")
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v, want an edit", changed, err)
	}
	want := explicitBlock("build", "sh ./test.sh") + explicitBlock("finalize", "sh ./test.sh")
	if string(out) != want {
		t.Fatalf("byte mismatch\n got: %q\nwant: %q", out, want)
	}
}

func TestExplicitEditCompletesHalfConfiguredPair(t *testing.T) { // (d)
	existing := explicitBlock("build", "sh ./test.sh") + "finalize:\n  gate: local\n"
	out, changed, err := RenderExplicitTestCommandEdit([]byte(existing), "sh ./test.sh")
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v, want an edit", changed, err)
	}
	want := explicitBlock("build", "sh ./test.sh") + explicitBlock("finalize", "sh ./test.sh")
	if string(out) != want {
		t.Fatalf("byte mismatch\n got: %q\nwant: %q", out, want)
	}
}

func TestExplicitEditOverwritesDivergentExplicitGateOnOneOwner(t *testing.T) { // (e)
	existing := explicitBlock("build", "sh ./test.sh") +
		"finalize:\n  gate: \"off\"\n  test_command: sh ./test.sh\n"
	out, changed, err := RenderExplicitTestCommandEdit([]byte(existing), "sh ./test.sh")
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v, want an edit (explicit command overwrites the gate)", changed, err)
	}
	want := explicitBlock("build", "sh ./test.sh") + explicitBlock("finalize", "sh ./test.sh")
	if string(out) != want {
		t.Fatalf("byte mismatch\n got: %q\nwant: %q", out, want)
	}
}

func TestExplicitEditAlreadyEqualIsNoChange(t *testing.T) { // (f)
	existing := []byte("integration_branch: main\n" +
		explicitBlock("build", "sh ./test.sh") + explicitBlock("finalize", "sh ./test.sh"))
	out, changed, err := RenderExplicitTestCommandEdit(existing, "sh ./test.sh")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if changed || !bytes.Equal(out, existing) {
		t.Fatalf("an already-equal file must be untouched: changed=%v\n got: %q", changed, out)
	}
}

func TestExplicitEditMalformedYAMLErrors(t *testing.T) {
	existing := []byte("build: [unclosed\n")
	out, changed, err := RenderExplicitTestCommandEdit(existing, "sh ./test.sh")
	if err == nil || changed || out != nil {
		t.Fatalf("malformed YAML must error with no edit: out=%q changed=%v err=%v", out, changed, err)
	}
}

func TestExplicitTestCommandRefusesInvalidInput(t *testing.T) {
	for _, raw := range []string{"", "   ", "\t\n", "auto", "  auto  ",
		"make\ntest", "make\ttest", "make\rtest", "a b", "a b", "a\u0085b", "\xff"} {
		got, err := ExplicitTestCommand(raw)
		if err == nil {
			t.Errorf("ExplicitTestCommand(%q) = %q, want a refusal", raw, got)
			continue
		}
		if !errors.Is(err, ErrInvalidTestCommand) {
			t.Errorf("ExplicitTestCommand(%q) error %v must wrap ErrInvalidTestCommand", raw, err)
		}
	}
}

func TestExplicitTestCommandTrims(t *testing.T) {
	got, err := ExplicitTestCommand("  sh ./test.sh  ")
	if err != nil || got != "sh ./test.sh" {
		t.Fatalf("ExplicitTestCommand = %q, %v; want the trimmed command", got, err)
	}
}

// TestExplicitCommandRoundTripsThroughConfigResolve is the reader-correspondence
// guard: every accepted command, rendered into .docket.yml, must resolve through
// the REAL config resolver (which types test_command as a !!str leaf) back to
// exactly that command with both gates local. Digit-led tokens such as 123 or
// 2026-10-05 are the cases a plain-charset check renders bare and the YAML
// resolver retypes.
func TestExplicitCommandRoundTripsThroughConfigResolve(t *testing.T) {
	cmds := []string{
		"sh ./test.sh", "123", "1.5", "2026-10-05", "0x10", "1e3",
		"off", "true", "null", "yes", "auto-test",
		`pytest -k "a and b"`, `echo 'x' \ y`, "make test # tail", "npm test -- --watch=false",
		"a: b", "{x}", "[x]", "- dash", "ünï test", "@at", "`tick`", "&anchor", "*alias",
		"!tag", "%pct", "|pipe", ">gt", `bash -c 'set -e; for t in tests/test_*.sh; do bash "$t"; done'`,
	}
	bases := map[string][]byte{
		"no file":  nil,
		"off file": []byte("integration_branch: main\nbuild:\n  gate: \"off\"\nfinalize:\n  gate: \"off\"\n"),
	}
	for _, cmd := range cmds {
		valid, err := ExplicitTestCommand(cmd)
		if err != nil {
			t.Fatalf("ExplicitTestCommand(%q) refused a legal command: %v", cmd, err)
		}
		for name, base := range bases {
			out, _, err := RenderExplicitTestCommandEdit(base, valid)
			if err != nil {
				t.Fatalf("%s / %q: render error %v", name, cmd, err)
			}
			snap, _, err := config.Resolve([]config.Source{{
				Layer: config.LayerRepository, Name: ".docket.yml", Data: out,
			}}, config.ResolveContext{DefaultBranch: "main"})
			if err != nil {
				t.Fatalf("%s / %q: rendered file does not resolve: %v\n%s", name, cmd, err, out)
			}
			eff := snap.Effective
			if eff.Build.TestCommand.Value != cmd || eff.Finalize.TestCommand.Value != cmd {
				t.Errorf("%s / %q: resolved build=%q finalize=%q\n%s", name, cmd,
					eff.Build.TestCommand.Value, eff.Finalize.TestCommand.Value, out)
			}
			if eff.Build.Gate.Value != "local" || eff.Finalize.Gate.Value != "local" {
				t.Errorf("%s / %q: resolved gates build=%q finalize=%q, want local", name, cmd,
					eff.Build.Gate.Value, eff.Finalize.Gate.Value)
			}
		}
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test -count=1 ./internal/reposetup/ -run 'TestExplicit'`
Expected: FAIL to compile (`undefined: RenderExplicitTestCommandEdit`, `ExplicitTestCommand`, `ErrInvalidTestCommand`).

- [ ] **Step 3: Implement**

In `internal/reposetup/testconfigedit.go`, add `"errors"`, `"unicode/utf8"`, and `"github.com/danielhanold/docket/internal/document"` to the imports.

Replace the body of `RenderTestConfigEdit` after its `switch` (from `pairs := desiredPairs(out)` to the final `return`) with a call into a new shared function, and add that function:

```go
	return renderOwnerPairs(existing, desiredPairs(out))
}

// renderOwnerPairs is the shared splice core of every test-policy render: it
// ensures each pair under BOTH the build and finalize owner blocks, replacing
// a divergent leaf in place, inserting a missing one, or appending a wholly
// missing block, and preserves every other byte. changed == false returns the
// existing bytes untouched. A malformed or structurally unsafe file is an
// error with (nil, false, err).
func renderOwnerPairs(existing []byte, pairs []kvPair) (edited []byte, changed bool, err error) {
	root, err := topLevelMapping(existing)
	if err != nil {
		return nil, false, err
	}
	starts := lineOffsets(existing)

	var splices []byteSplice
	var appends []string
	// build and finalize are edited independently: each owns its own gate and
	// test_command, even though one command is written identically to both.
	for _, owner := range []string{"build", "finalize"} {
		sp, appText, ch, err := planOwnerBlock(existing, starts, root, owner, pairs)
		if err != nil {
			return nil, false, err
		}
		if ch {
			changed = true
		}
		splices = append(splices, sp...)
		if appText != "" {
			appends = append(appends, appText)
		}
	}

	if !changed {
		return existing, false, nil
	}

	edited = applySplices(existing, splices)
	edited = appendBlocks(edited, appends)
	return edited, true, nil
}
```

Add the explicit-command entry points (place them after `desiredPairs`):

```go
// ConfigureTestsCommandRemedy is the configure-tests invocation that sets both
// gates to `local` with an operator-supplied suite command. Every note that
// sends an operator to an explicit command names it through this constant.
const ConfigureTestsCommandRemedy = `docket repository configure-tests --command "<cmd>"`

// ErrInvalidTestCommand classifies a refused --command value.
var ErrInvalidTestCommand = errors.New("invalid test command")

// ExplicitTestCommand validates an operator-supplied suite command and returns
// it trimmed of surrounding whitespace. It refuses an empty or whitespace-only
// value, the legacy `auto` sentinel (it never survives resolution as a command;
// isConfiguredCommand), invalid UTF-8, and any rune document.IllegalTextRune
// refuses (control characters, tab, and the U+2028/U+2029 line separators),
// because a double-quoted YAML scalar folds a line break to a space and would
// write a command other than the one given.
func ExplicitTestCommand(raw string) (string, error) {
	if !utf8.ValidString(raw) {
		return "", fmt.Errorf("%w: --command is not valid UTF-8", ErrInvalidTestCommand)
	}
	cmd := strings.TrimSpace(raw)
	if cmd == "" {
		return "", fmt.Errorf(`%w: --command is empty; pass the suite command, e.g. --command "make test"`, ErrInvalidTestCommand)
	}
	if !isConfiguredCommand(cmd) {
		return "", fmt.Errorf("%w: --command %q is the legacy unconfigured sentinel, not a suite command", ErrInvalidTestCommand, cmd)
	}
	for _, r := range cmd {
		if document.IllegalTextRune(r) {
			return "", fmt.Errorf("%w: --command contains a control or line-break character (%U); pass a single-line command", ErrInvalidTestCommand, r)
		}
	}
	return cmd, nil
}

// RenderExplicitTestCommandEdit produces the pending `.docket.yml` bytes that
// set BOTH build and finalize to `gate: local` + `test_command: <cmd>`. Unlike
// the discovery render, the explicit command is the human's choice, so it wins:
// the gate pair is NOT preserve-explicit, an existing `off` (or any other)
// gate becomes `local`, and a different command is replaced. changed == false
// means the file already carries exactly these settings. cmd must already have
// passed ExplicitTestCommand; an invalid cmd is refused here too.
func RenderExplicitTestCommandEdit(existing []byte, cmd string) (edited []byte, changed bool, err error) {
	if _, verr := ExplicitTestCommand(cmd); verr != nil {
		return nil, false, verr
	}
	return renderOwnerPairs(existing, explicitPairs(cmd))
}

// explicitPairs is the explicit-command policy: gate local and the command,
// neither preserve-explicit.
func explicitPairs(cmd string) []kvPair {
	return []kvPair{{"gate", "local", false}, {"test_command", cmd, false}}
}
```

Extend `plainSafe` so its final `return true` asks the YAML resolver, and add the helper:

```go
	// The charset admits digit-led tokens that YAML resolves to a non-string
	// (123 → !!int, 1.5 → !!float, 2026-10-05 → !!timestamp), and the config
	// schema's string leaves accept only !!str. Ask the resolver itself rather
	// than enumerating number shapes.
	return resolvesToStr(v)
}

// resolvesToStr reports whether v, emitted as a bare plain scalar, reads back
// as the !!str v.
func resolvesToStr(v string) bool {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(v), &doc); err != nil || len(doc.Content) != 1 {
		return false
	}
	n := doc.Content[0]
	return n.Kind == yaml.ScalarNode && n.Tag == "!!str" && n.Value == v
}
```

(The `for` loop in `plainSafe` currently ends the function with `return true`. Replace only that final line.)

- [ ] **Step 4: Run the package tests**

Run: `go test -count=1 ./internal/reposetup/`
Expected: PASS. That covers every new test plus the existing `TestConfigEdit*` and `TestPolicyEdit*` tests. The refactor of `RenderTestConfigEdit` is behavior-neutral.

- [ ] **Step 5: Mutation-check the two guards**

1. In `explicitPairs`, change the gate pair to `{"gate", "local", true}`. Confirm the edit landed with `grep -c '{"gate", "local", true}' internal/reposetup/testconfigedit.go` (expect `2`: the detected pair and the mutated one). Run `go test -count=1 ./internal/reposetup/ -run 'TestExplicitEdit'`. Expected: `TestExplicitEditOverwritesOffGates` (b) and `TestExplicitEditOverwritesDivergentExplicitGateOnOneOwner` (e) FAIL. Restore the line from a backup copy you saved before editing (not `git checkout`, which would discard the uncommitted work), then re-run and confirm PASS.
2. In `plainSafe`, replace `return resolvesToStr(v)` with `return true`. Run `go test -count=1 ./internal/reposetup/ -run TestExplicitCommandRoundTripsThroughConfigResolve`. Expected: FAIL on `123` (and the other digit-led tokens). Restore and re-run, PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/reposetup/testconfigedit.go internal/reposetup/testconfigedit_test.go
git commit -m "feat(reposetup): explicit test-command planner for configure-tests --command"
```

---

### Task 2: Remedies that resolve: the gap note and migrate's ambiguity refusal (`internal/reposetup`)

**Files:**
- Modify: `internal/reposetup/health.go` (`ConfigureTestsGapNote` text and doc comment)
- Modify: `internal/reposetup/testdiscovery.go` (add `DescribeCandidates`; rewrite `AmbiguousTestDiscoveryError.Error` and its doc comment)
- Modify: `internal/reposetup/health_test.go` (extend the two `TestConfigureTestsGapNote*` tests)
- Modify: `internal/reposetup/migrateplan_test.go` (`TestPlanMigrationAmbiguousRefusesWithNoPlan`)
- Modify: `internal/app/repomigration_integration_test.go` (`TestIntegrationRepoMigrationAmbiguousTestDiscoveryBlocksBeforeAnyWrite`)

**Interfaces:**
- Consumes: `ConfigureTestsCommandRemedy` (Task 1).
- Produces: `func DescribeCandidates(cands []DetectedSuite) string`, which renders ``go (`go test ./...`), rust (`cargo test`)`` (family, space, backticked command in parentheses, joined by `", "`). Task 3 uses it.

- [ ] **Step 1: Write the failing tests**

In `internal/reposetup/health_test.go`, append these asserts to the END of BOTH `TestConfigureTestsGapNoteFinalizeLocalEmptyWhileBuildConfigured` and `TestConfigureTestsGapNoteBuildLocalEmptyWhileFinalizeConfigured`:

```go
	if !strings.Contains(note, ConfigureTestsCommandRemedy) {
		t.Errorf("note %q must name the command remedy %q", note, ConfigureTestsCommandRemedy)
	}
	if strings.Contains(note, "by hand") {
		t.Errorf("note %q must not prescribe a hand edit now that --command exists", note)
	}
```

In `internal/reposetup/migrateplan_test.go`, `TestPlanMigrationAmbiguousRefusesWithNoPlan`, replace the block

```go
	if !strings.Contains(amb.Error(), "docket repository configure-tests") {
		t.Errorf("remedy %q must name `docket repository configure-tests`", amb.Error())
	}
```

with

```go
	// configure-tests refuses a legacy repository, so naming it here loops; the
	// remedy is the explicit finalize command migrate itself preserves.
	if strings.Contains(amb.Error(), "configure-tests") {
		t.Errorf("remedy %q must not name configure-tests, which refuses a legacy repository", amb.Error())
	}
	for _, want := range []string{"finalize.test_command", "docket repository migrate"} {
		if !strings.Contains(amb.Error(), want) {
			t.Errorf("remedy %q must name %q", amb.Error(), want)
		}
	}
	for _, c := range amb.Candidates {
		if !strings.Contains(amb.Error(), c.Command) {
			t.Errorf("remedy %q must name candidate command %q", amb.Error(), c.Command)
		}
	}
```

Append to `internal/reposetup/testdiscovery_test.go`:

```go
func TestDescribeCandidatesNamesFamilyAndCommand(t *testing.T) {
	got := DescribeCandidates([]DetectedSuite{
		{Family: "go", Command: "go test ./..."},
		{Family: "rust", Command: "cargo test"},
	})
	want := "go (`go test ./...`), rust (`cargo test`)"
	if got != want {
		t.Fatalf("DescribeCandidates = %q, want %q", got, want)
	}
}
```

In `internal/app/repomigration_integration_test.go`, `TestIntegrationRepoMigrationAmbiguousTestDiscoveryBlocksBeforeAnyWrite`, replace

```go
	if !strings.Contains(res.HumanText(), "docket repository configure-tests") {
		t.Errorf("ambiguous refusal %q must name the remedy", res.HumanText())
	}
```

with

```go
	if !strings.Contains(res.HumanText(), "docket repository migrate") || !strings.Contains(res.HumanText(), "finalize.test_command") {
		t.Errorf("ambiguous refusal %q must name the finalize.test_command + re-run migrate remedy", res.HumanText())
	}
	if strings.Contains(res.HumanText(), "configure-tests") {
		t.Errorf("ambiguous refusal %q must not name configure-tests, which refuses a legacy repository", res.HumanText())
	}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test -count=1 ./internal/reposetup/ -run 'TestConfigureTestsGapNote|TestPlanMigrationAmbiguous|TestDescribeCandidates'`
Expected: FAIL (`undefined: DescribeCandidates`; after that compiles, the gap-note and migrate remedy asserts fail).

- [ ] **Step 3: Implement**

In `internal/reposetup/testdiscovery.go`, add after `isConfiguredCommand`:

```go
// DescribeCandidates renders discovery candidates for an operator as
// "family (`command`)" joined by ", " in registry order, so a human choosing
// between them sees the exact command each would configure.
func DescribeCandidates(cands []DetectedSuite) string {
	parts := make([]string, 0, len(cands))
	for _, c := range cands {
		parts = append(parts, fmt.Sprintf("%s (`%s`)", c.Family, c.Command))
	}
	return strings.Join(parts, ", ")
}
```

Replace `AmbiguousTestDiscoveryError.Error` (and update its doc comment and the type's doc comment so they say the remedy is the explicit `finalize.test_command` plus a re-run of migrate):

```go
// Error names the ambiguous candidates with their commands and the remedy valid
// on a legacy repository: commit an explicit finalize.test_command, which
// migrateTestOutcome preserves and carries into build.test_command, then re-run
// migrate. It never names configure-tests, which refuses a legacy repository.
func (e *AmbiguousTestDiscoveryError) Error() string {
	return fmt.Sprintf("test discovery is ambiguous: multiple suite families match (%s); set `finalize.test_command` in .docket.yml to the suite to use, commit and push it, then re-run `docket repository migrate` (migrate carries an explicit finalize.test_command into build.test_command)",
		DescribeCandidates(e.Candidates))
}
```

In `internal/reposetup/health.go`, replace the `return fmt.Sprintf(...)` at the end of `ConfigureTestsGapNote` with:

```go
	return fmt.Sprintf("the %s gate is `local` with no command (%s unset; the pair reads as configured because the other gate already has a command); run `%s` to set both gates to `local` with one suite command, commit the pending .docket.yml, then re-run `docket repository check`.",
		strings.Join(owners, " and "), strings.Join(keys, " and "), ConfigureTestsCommandRemedy)
```

Update the `ConfigureTestsGapNote` doc comment. Its sentence ending "so it names the specific gate(s) and the by-hand completion" becomes "so it names the specific gate(s) and the explicit `configure-tests --command` completion, which sets both gates to one command".

- [ ] **Step 4: Run to verify pass**

Run: `go test -count=1 ./internal/reposetup/`
Expected: PASS.
Run: `go test -count=1 -tags integration -run '^TestIntegrationRepoMigrationAmbiguousTestDiscoveryBlocksBeforeAnyWrite$' ./internal/app/`
Expected: PASS.

- [ ] **Step 5: Mutation-check**

Temporarily restore the old `Error()` tail (`..., then run \`docket repository configure-tests\``). Run the migrate unit test with `-count=1`: it must FAIL on the `configure-tests` absence assert. Restore, re-run, PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/reposetup/health.go internal/reposetup/testdiscovery.go internal/reposetup/health_test.go internal/reposetup/migrateplan_test.go internal/reposetup/testdiscovery_test.go internal/app/repomigration_integration_test.go
git commit -m "fix(reposetup): gap note and migrate ambiguity name remedies that resolve"
```

---

### Task 3: `configure-tests --command` end to end in `internal/app` and `internal/cli`, plus truthful messages

App and CLI change together: the runner signature changes, so neither package builds alone.

**Files:**
- Modify: `internal/app/repository_configure_tests.go` (options type, explicit path, message functions, doc comment)
- Modify: `internal/app/repository_init.go` (extract `writePendingDocketYML`; add `ensureExplicitTestCommand`; rewrite `testDiscoveryNote`)
- Modify: `internal/cli/repository.go` (runner signature, `--command` flag)
- Modify: `internal/app/reposetup_configure_tests_integration_test.go` (only the `runConfigureTests` helper, so the integration build still compiles)
- Test: `internal/app/repository_configure_tests_test.go` (append), `internal/app/repository_init_test.go` (append), `internal/cli/repository_test.go` (append)

**Interfaces:**
- Consumes: `reposetup.ExplicitTestCommand`, `reposetup.RenderExplicitTestCommandEdit`, `reposetup.ConfigureTestsCommandRemedy` (Task 1); `reposetup.DescribeCandidates` and the new `reposetup.ConfigureTestsGapNote` text (Task 2).
- Produces:
  - `type ConfigureTestsOptions struct { Command *string }`
  - `func RunRepositoryConfigureTests(ctx context.Context, d SetupDeps, o ConfigureTestsOptions) OperationResult`
  - `func configureTestsDiscoveryText(state reposetup.State, wrote bool, pendingPath string, outcome reposetup.DiscoveryOutcome, cfg config.Effective) string`
  - `func configureTestsExplicitText(state reposetup.State, wrote bool, pendingPath, cmd string) string`
  - `func ensureExplicitTestCommand(primaryWorktree, cmd string) (pendingPath string, wrote bool, err error)`
  - `func (r *initRepo) runConfigureTestsWith(t *testing.T, o ConfigureTestsOptions) RepositoryOpResult` (integration helper Task 4 uses)
  - CLI runner var `repositoryConfigureTestsRunner func(ctx context.Context, d app.SetupDeps, o app.ConfigureTestsOptions) app.OperationResult`

- [ ] **Step 1: Write the failing app tests**

Append to `internal/app/repository_configure_tests_test.go`. Add `"context"` and `"github.com/danielhanold/docket/internal/config"` to its imports:

```go
func testPolicyCfg(buildGate, buildCmd, finalizeGate, finalizeCmd string) config.Effective {
	var eff config.Effective
	eff.Build.Gate = config.Value[string]{Value: buildGate}
	eff.Build.TestCommand = config.Value[string]{Value: buildCmd}
	eff.Finalize.Gate = config.Value[string]{Value: finalizeGate}
	eff.Finalize.TestCommand = config.Value[string]{Value: finalizeCmd}
	return eff
}

const alreadyConfiguredText = "already configured"

func TestConfigureTestsDiscoveryTextNoneNamesCommandRemedy(t *testing.T) {
	got := configureTestsDiscoveryText(reposetup.StateHealthy, false, "",
		reposetup.DiscoveryOutcome{Kind: reposetup.DiscoveryNone}, testPolicyCfg("off", "", "off", ""))
	if !strings.Contains(got, "no supported test suite was found") || !strings.Contains(got, reposetup.ConfigureTestsCommandRemedy) {
		t.Errorf("none text %q must say no suite was found and name %q", got, reposetup.ConfigureTestsCommandRemedy)
	}
	if strings.Contains(got, alreadyConfiguredText) {
		t.Errorf("none text %q must not claim the policy is already configured", got)
	}
}

// The none plan is preserve-explicit on gate, so an explicit `local` gate with
// no command also reaches the none no-op: the text must report the RESOLVED
// gates, never assume off.
func TestConfigureTestsDiscoveryTextNoneReportsResolvedGates(t *testing.T) {
	got := configureTestsDiscoveryText(reposetup.StateHealthy, false, "",
		reposetup.DiscoveryOutcome{Kind: reposetup.DiscoveryNone}, testPolicyCfg("local", "", "off", ""))
	if !strings.Contains(got, "build gate `local`") || !strings.Contains(got, "finalize gate `off`") {
		t.Errorf("none text %q must report the resolved gates (build local, finalize off)", got)
	}
}

func TestConfigureTestsDiscoveryTextAmbiguousListsEveryCandidateCommand(t *testing.T) {
	outcome := reposetup.DiscoveryOutcome{Kind: reposetup.DiscoveryAmbiguous, Candidates: []reposetup.DetectedSuite{
		{Family: "go", Command: "go test ./..."}, {Family: "rust", Command: "cargo test"},
	}}
	got := configureTestsDiscoveryText(reposetup.StateHealthy, false, "", outcome, testPolicyCfg("off", "", "off", ""))
	for _, want := range []string{"go test ./...", "cargo test", reposetup.ConfigureTestsCommandRemedy} {
		if !strings.Contains(got, want) {
			t.Errorf("ambiguous text %q must name %q", got, want)
		}
	}
	if strings.Contains(got, alreadyConfiguredText) {
		t.Errorf("ambiguous text %q must not claim the policy is already configured", got)
	}
}

func TestConfigureTestsDiscoveryTextConfiguredNamesBothCommands(t *testing.T) {
	got := configureTestsDiscoveryText(reposetup.StateHealthy, false, "",
		reposetup.DiscoveryOutcome{Kind: reposetup.DiscoveryConfigured}, testPolicyCfg("local", "make a", "local", "make b"))
	for _, want := range []string{alreadyConfiguredText, "`make a`", "`make b`"} {
		if !strings.Contains(got, want) {
			t.Errorf("configured text %q must contain %q", got, want)
		}
	}
	if strings.Contains(got, reposetup.ConfigureTestsCommandRemedy) {
		t.Errorf("a fully configured pair has no gap; text %q must not name the command remedy", got)
	}
}

func TestConfigureTestsDiscoveryTextConfiguredWithGapNamesCommandRemedy(t *testing.T) {
	got := configureTestsDiscoveryText(reposetup.StateHealthy, false, "",
		reposetup.DiscoveryOutcome{Kind: reposetup.DiscoveryConfigured}, testPolicyCfg("local", "make a", "local", ""))
	for _, want := range []string{"finalize.test_command", reposetup.ConfigureTestsCommandRemedy} {
		if !strings.Contains(got, want) {
			t.Errorf("configured-with-gap text %q must contain %q", got, want)
		}
	}
}

func TestConfigureTestsDiscoveryTextDetectedNoChangeNamesCommand(t *testing.T) {
	outcome := reposetup.DiscoveryOutcome{Kind: reposetup.DiscoveryDetected, Command: "go test ./...",
		Candidates: []reposetup.DetectedSuite{{Family: "go", Command: "go test ./..."}}}
	got := configureTestsDiscoveryText(reposetup.StateHealthy, false, "", outcome, testPolicyCfg("off", "", "off", ""))
	if !strings.Contains(got, alreadyConfiguredText) || !strings.Contains(got, "`go test ./...`") {
		t.Errorf("detected-no-change text %q must say already configured and name the command", got)
	}
}

func TestConfigureTestsDiscoveryTextWroteNoneStillNamesRemedy(t *testing.T) {
	got := configureTestsDiscoveryText(reposetup.StateNeedsReview, true, ".docket.yml",
		reposetup.DiscoveryOutcome{Kind: reposetup.DiscoveryNone}, testPolicyCfg("", "", "", ""))
	for _, want := range []string{"review and commit the pending path: .docket.yml", reposetup.ConfigureTestsCommandRemedy} {
		if !strings.Contains(got, want) {
			t.Errorf("wrote-none text %q must contain %q", got, want)
		}
	}
}

func TestConfigureTestsExplicitText(t *testing.T) {
	wrote := configureTestsExplicitText(reposetup.StateNeedsReview, true, ".docket.yml", "sh ./test.sh")
	for _, want := range []string{"`sh ./test.sh`", "review and commit the pending path: .docket.yml"} {
		if !strings.Contains(wrote, want) {
			t.Errorf("explicit wrote text %q must contain %q", wrote, want)
		}
	}
	noop := configureTestsExplicitText(reposetup.StateHealthy, false, "", "sh ./test.sh")
	for _, want := range []string{"no-op", "`sh ./test.sh`", "nothing to write"} {
		if !strings.Contains(noop, want) {
			t.Errorf("explicit no-op text %q must contain %q", noop, want)
		}
	}
}

// Bad input is refused before ANY repository read: the nil Git client would
// fail the gather if validation ran after it.
func TestRunRepositoryConfigureTestsRefusesInvalidCommandBeforeGather(t *testing.T) {
	for _, raw := range []string{"", "   ", "auto"} {
		v := raw
		res := RunRepositoryConfigureTests(context.Background(), SetupDeps{}, ConfigureTestsOptions{Command: &v})
		got, ok := res.(RepositoryOpResult)
		if !ok {
			t.Fatalf("result is %T, want RepositoryOpResult", res)
		}
		if got.Result != ResultInvalidInput {
			t.Errorf("--command %q = %q (%s), want invalid-input", raw, got.Result, got.HumanText())
		}
		if len(got.PendingPaths) != 0 {
			t.Errorf("--command %q wrote pending paths %v", raw, got.PendingPaths)
		}
	}
}
```

Append to `internal/app/repository_init_test.go`:

```go
func TestInitTestDiscoveryNoteNoneNamesCommandAfterCommit(t *testing.T) {
	note := testDiscoveryNote(reposetup.DiscoveryOutcome{Kind: reposetup.DiscoveryNone})
	for _, want := range []string{"no supported test suite was found", "after committing the pending paths", reposetup.ConfigureTestsCommandRemedy} {
		if !strings.Contains(note, want) {
			t.Errorf("init none note %q must contain %q", note, want)
		}
	}
}

func TestInitTestDiscoveryNoteAmbiguousNamesCandidatesAndCommand(t *testing.T) {
	note := testDiscoveryNote(reposetup.DiscoveryOutcome{Kind: reposetup.DiscoveryAmbiguous, Candidates: []reposetup.DetectedSuite{
		{Family: "go", Command: "go test ./..."}, {Family: "rust", Command: "cargo test"},
	}})
	for _, want := range []string{"go test ./...", "cargo test", "after committing the pending paths", reposetup.ConfigureTestsCommandRemedy} {
		if !strings.Contains(note, want) {
			t.Errorf("init ambiguous note %q must contain %q", note, want)
		}
	}
}

func TestInitTestDiscoveryNoteDetectedIsEmpty(t *testing.T) {
	if note := testDiscoveryNote(reposetup.DiscoveryOutcome{Kind: reposetup.DiscoveryDetected, Command: "make test"}); note != "" {
		t.Errorf("a detected outcome needs no note, got %q", note)
	}
}
```

- [ ] **Step 2: Write the failing CLI tests**

Append to `internal/cli/repository_test.go`:

```go
// TestRepositoryConfigureTestsCommandFlagFlows proves --command reaches the app
// options as a non-nil pointer whenever the flag is PASSED, including the empty
// value (which the app must refuse, not mistake for "absent"), and stays nil
// when it is not passed.
func TestRepositoryConfigureTestsCommandFlagFlows(t *testing.T) {
	tmp := testsupport.TempDir(t)
	var got app.ConfigureTestsOptions
	old := repositoryConfigureTestsRunner
	repositoryConfigureTestsRunner = func(ctx context.Context, d app.SetupDeps, o app.ConfigureTestsOptions) app.OperationResult {
		got = o
		return fakeSyncResult{Envelope: app.NewEnvelope("repository.configure-tests", app.ResultNoOp)}
	}
	defer func() { repositoryConfigureTestsRunner = old }()

	if _, _, code := runCLI(t, "repository", "configure-tests", "--repo-dir", tmp, "--command", "sh ./test.sh"); code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if got.Command == nil || *got.Command != "sh ./test.sh" {
		t.Fatalf("Command = %v, want pointer to %q", got.Command, "sh ./test.sh")
	}

	got = app.ConfigureTestsOptions{}
	if _, _, code := runCLI(t, "repository", "configure-tests", "--repo-dir", tmp, "--command", ""); code != 0 {
		t.Fatalf("exit = %d, want 0 (the stubbed runner decides)", code)
	}
	if got.Command == nil || *got.Command != "" {
		t.Fatalf("an explicit empty --command must arrive as a non-nil empty pointer, got %v", got.Command)
	}

	got = app.ConfigureTestsOptions{Command: new(string)}
	if _, _, code := runCLI(t, "repository", "configure-tests", "--repo-dir", tmp); code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if got.Command != nil {
		t.Fatalf("no --command must arrive as nil (run discovery), got %q", *got.Command)
	}
}

func TestRepositoryConfigureTestsHelpNamesCommandFlag(t *testing.T) {
	out, _, _ := runCLI(t, "repository", "configure-tests", "--help")
	if !strings.Contains(out, "--command") {
		t.Errorf("configure-tests --help must document --command:\n%s", out)
	}
}
```

(If `runCLI` writes `--help` output to its stderr return value instead, assert on the concatenation of both outputs.)

- [ ] **Step 3: Run to verify failure**

Run: `go test -count=1 ./internal/app/ ./internal/cli/ -run 'ConfigureTests|InitTestDiscoveryNote'`
Expected: FAIL to compile (`undefined: configureTestsDiscoveryText`, `ConfigureTestsOptions`, …).

- [ ] **Step 4: Implement the shared write helper and the init note (`internal/app/repository_init.go`)**

Replace `ensureTestPolicyConfig` with the helper plus two thin callers (keep its doc comment, adjusted):

```go
// writePendingDocketYML reads the primary worktree's .docket.yml (nil when
// absent), asks render for the edited bytes, and writes them UNSTAGED when
// render returns non-nil — the managed-.gitignore posture: generated config is
// human-gated, never staged. It returns the pending review path ("" when nothing
// was written). A read fault, a render error (malformed config, probe fault,
// invalid command), or a write fault is an error with the file untouched.
func writePendingDocketYML(primaryWorktree string, render func(existing []byte) ([]byte, error)) (pendingPath string, wrote bool, err error) {
	abs := filepath.Join(primaryWorktree, docketYMLRel)
	existing, rerr := os.ReadFile(abs)
	if rerr != nil {
		if !os.IsNotExist(rerr) {
			return "", false, rerr
		}
		existing = nil
	}
	edited, perr := render(existing)
	if perr != nil {
		return "", false, perr
	}
	if edited == nil {
		return "", false, nil
	}
	if werr := os.WriteFile(abs, edited, 0o644); werr != nil {
		return "", false, werr
	}
	return docketYMLRel, true, nil
}

// ensureTestPolicyConfig discovers the suite from the primary worktree and
// writes the generated test-policy edit through writePendingDocketYML,
// returning the discovery outcome so the caller can report it.
func ensureTestPolicyConfig(primaryWorktree string, cfg config.Effective) (pendingPath string, wrote bool, outcome reposetup.DiscoveryOutcome, err error) {
	tree := newOSTree(primaryWorktree)
	pendingPath, wrote, err = writePendingDocketYML(primaryWorktree, func(existing []byte) ([]byte, error) {
		edited, oc, perr := reposetup.TestPolicyEdit(cfg, existing, tree)
		outcome = oc
		return edited, perr
	})
	if err != nil {
		return "", false, reposetup.DiscoveryOutcome{}, err
	}
	return pendingPath, wrote, outcome, nil
}

// ensureExplicitTestCommand writes the explicit-command policy (both gates
// local, both commands cmd) through writePendingDocketYML. No discovery runs.
// cmd must already have passed reposetup.ExplicitTestCommand.
func ensureExplicitTestCommand(primaryWorktree, cmd string) (pendingPath string, wrote bool, err error) {
	return writePendingDocketYML(primaryWorktree, func(existing []byte) ([]byte, error) {
		edited, changed, rerr := reposetup.RenderExplicitTestCommandEdit(existing, cmd)
		if rerr != nil || !changed {
			return nil, rerr
		}
		return edited, nil
	})
}
```

Replace `testDiscoveryNote` (it is now init-only; update its doc comment to say so and that every remedy is valid only once init's pending paths are committed):

```go
func testDiscoveryNote(outcome reposetup.DiscoveryOutcome) string {
	switch outcome.Kind {
	case reposetup.DiscoveryAmbiguous:
		return fmt.Sprintf("test discovery was ambiguous (%s); no test policy was written — after committing the pending paths, run `%s` with the one to use",
			reposetup.DescribeCandidates(outcome.Candidates), reposetup.ConfigureTestsCommandRemedy)
	case reposetup.DiscoveryNone:
		return fmt.Sprintf("no supported test suite was found, so no test command was written and a gate .docket.yml does not set is `off`; after committing the pending paths, run `%s` to set both gates to `local` with your suite command",
			reposetup.ConfigureTestsCommandRemedy)
	}
	return ""
}
```

- [ ] **Step 5: Implement the app service (`internal/app/repository_configure_tests.go`)**

Add `"github.com/danielhanold/docket/internal/config"` to the imports. Add the options type, and rewrite `RunRepositoryConfigureTests`. Its doc comment gains: "With `--command` (o.Command non-nil) it validates the command before any read, skips discovery, and plans both gates `local` with that command, overwriting the existing policy." The guard, refusal helpers, and the augmentation block stay as they are.

```go
// ConfigureTestsOptions carries configure-tests' one input. Command is nil when
// --command was not passed (discovery runs) and non-nil whenever it was, even
// when empty: an explicit empty value is refused, never read as absent.
type ConfigureTestsOptions struct {
	Command *string
}

func RunRepositoryConfigureTests(ctx context.Context, d SetupDeps, o ConfigureTestsOptions) OperationResult {
	// Validate the whole input before any repository read or write.
	var explicit string
	if o.Command != nil {
		cmd, verr := reposetup.ExplicitTestCommand(*o.Command)
		if verr != nil {
			out := newRepositoryOpResult(OperationRepositoryConfigureTests, ResultInvalidInput, RepositoryOpResult{})
			out.human = fmt.Sprintf("%s: %s: %s", OperationRepositoryConfigureTests, ResultInvalidInput, verr.Error())
			return out
		}
		explicit = cmd
	}

	facts, sc, err := GatherSetupFacts(ctx, d, true)
	if err != nil {
		return repositoryGatherFailure(OperationRepositoryConfigureTests, err)
	}
	// (keep the existing augmentCheckFacts block and its comment unchanged)
	if facts.RemoteMetadata.Presence == reposetup.PresencePresent {
		augmentCheckFacts(ctx, d.Git, &facts, sc)
	}
	cls, refusal := configureTestsGuard(facts)
	if refusal != nil {
		return *refusal
	}

	var (
		pendingPath string
		wrote       bool
		discovery   reposetup.DiscoveryOutcome
		werr        error
	)
	if o.Command != nil {
		pendingPath, wrote, werr = ensureExplicitTestCommand(sc.repo.PrimaryWorktree, explicit)
	} else {
		pendingPath, wrote, discovery, werr = ensureTestPolicyConfig(sc.repo.PrimaryWorktree, sc.cfg)
	}
	if werr != nil {
		return repositoryInternalFailure(OperationRepositoryConfigureTests, cls.State, "generating the test-policy config", werr)
	}

	result := ResultNoOp
	state := cls.State
	var pending []string
	if wrote {
		result = ResultApplied
		state = reposetup.StateNeedsReview
		pending = []string{pendingPath}
	}
	out := newRepositoryOpResult(OperationRepositoryConfigureTests, result, RepositoryOpResult{
		RepositoryState: string(state),
		PendingPaths:    pending,
		SourceRevision:  sc.sourceRevision,
	})
	if o.Command != nil {
		out.human = configureTestsExplicitText(state, wrote, pendingPath, explicit)
	} else {
		out.human = configureTestsDiscoveryText(state, wrote, pendingPath, discovery, sc.cfg)
	}
	return out
}

// configureTestsExplicitText renders the --command outcome.
func configureTestsExplicitText(state reposetup.State, wrote bool, pendingPath, cmd string) string {
	if wrote {
		return fmt.Sprintf("test policy set: build and finalize gates `local` running `%s` (%s); review and commit the pending path: %s",
			cmd, state, pendingPath)
	}
	return fmt.Sprintf("%s: %s (%s): build and finalize gates are already `local` running `%s`; nothing to write",
		OperationRepositoryConfigureTests, ResultNoOp, state, cmd)
}

// configureTestsDiscoveryText renders the discovery outcome truthfully per
// kind, never one generic "already configured" line: none reports the resolved
// gates and the --command remedy (the none plan is preserve-explicit on gate,
// so the gates are not assumed off); ambiguous names every candidate's command;
// configured names both resolved commands plus any per-gate gap; detected with
// no change names the command already in place.
func configureTestsDiscoveryText(state reposetup.State, wrote bool, pendingPath string, outcome reposetup.DiscoveryOutcome, cfg config.Effective) string {
	noneRemedy := fmt.Sprintf("re-run with `%s` to set both gates to `local` with your suite command", reposetup.ConfigureTestsCommandRemedy)
	if wrote {
		text := fmt.Sprintf("test policy generated (%s); review and commit the pending path: %s", state, pendingPath)
		if outcome.Kind == reposetup.DiscoveryNone {
			text += "\nno supported test suite was found, so no test command was written; after committing, " + noneRemedy
		}
		return text
	}
	var body string
	switch outcome.Kind {
	case reposetup.DiscoveryNone:
		body = fmt.Sprintf("no supported test suite was found, so no test command was written (build gate `%s`, finalize gate `%s`); %s",
			cfg.Build.Gate.Value, cfg.Finalize.Gate.Value, noneRemedy)
	case reposetup.DiscoveryAmbiguous:
		body = fmt.Sprintf("test discovery found more than one suite, so nothing was written: %s; re-run `%s` with the one to use",
			reposetup.DescribeCandidates(outcome.Candidates), reposetup.ConfigureTestsCommandRemedy)
	case reposetup.DiscoveryDetected:
		body = fmt.Sprintf("the test policy is already configured (build and finalize: `%s`); nothing to write", outcome.Command)
	default: // configured
		body = fmt.Sprintf("the test policy is already configured (build: %s; finalize: %s); nothing to write",
			describeCommand(cfg.Build.TestCommand.Value), describeCommand(cfg.Finalize.TestCommand.Value))
		if gap := reposetup.ConfigureTestsGapNote(cfg); gap != "" {
			body += "\n" + gap
		}
	}
	return fmt.Sprintf("%s: %s (%s): %s", OperationRepositoryConfigureTests, ResultNoOp, state, body)
}

// describeCommand renders a resolved test command for an operator: the
// backticked command, or "no command" when unset.
func describeCommand(cmd string) string {
	if cmd == "" {
		return "no command"
	}
	return "`" + cmd + "`"
}
```

- [ ] **Step 6: Wire the CLI (`internal/cli/repository.go`)**

Change the runner var:

```go
	repositoryConfigureTestsRunner = func(ctx context.Context, d app.SetupDeps, o app.ConfigureTestsOptions) app.OperationResult {
		return app.RunRepositoryConfigureTests(ctx, d, o)
	}
```

Change the `configureTestsCmd` construction (keep its `Short` text and `EffectLocalWrite`), then register the flag right after it:

```go
	configureTestsCmd := repositorySubcommand("configure-tests",
		"Generate the pending .docket.yml build/finalize test policy for an already-initialized repository",
		func(c *cobra.Command, deps app.SetupDeps) {
			// Changed, not the value: an explicit empty --command must reach the
			// app (which refuses it), never read as "run discovery".
			var o app.ConfigureTestsOptions
			if c.Flags().Changed("command") {
				v, _ := c.Flags().GetString("command")
				o.Command = &v
			}
			setResult(repositoryConfigureTestsRunner(c.Context(), deps, o))
		},
		// local-write: (re)generates the pending, unstaged .docket.yml edit —
		// never commits, never stages.
		EffectLocalWrite)
	configureTestsCmd.Flags().String("command", "",
		"suite `command` to set as both build.test_command and finalize.test_command with both gates local (skips discovery)")
```

- [ ] **Step 7: Keep the integration build compiling**

In `internal/app/reposetup_configure_tests_integration_test.go`, replace the `runConfigureTests` helper with:

```go
// runConfigureTests runs a discovery configure-tests (no --command).
func (r *initRepo) runConfigureTests(t *testing.T) RepositoryOpResult {
	t.Helper()
	return r.runConfigureTestsWith(t, ConfigureTestsOptions{})
}

// runConfigureTestsWith runs RunRepositoryConfigureTests against the invocation
// clone with a fresh isolated client and type-asserts the concrete result.
func (r *initRepo) runConfigureTestsWith(t *testing.T, o ConfigureTestsOptions) RepositoryOpResult {
	t.Helper()
	client := newGitClient(t)
	res := RunRepositoryConfigureTests(context.Background(), SetupDeps{Git: client, RepoDir: r.invocation}, o)
	got, ok := res.(RepositoryOpResult)
	if !ok {
		t.Fatalf("configure-tests result is %T, want RepositoryOpResult", res)
	}
	return got
}
```

Then confirm no other caller exists: `grep -rn -e 'RunRepositoryConfigureTests(' -e 'repositoryConfigureTestsRunner' --include='*.go' .`. Every hit must pass the options argument.

- [ ] **Step 8: Run to verify pass**

Run: `go build ./... && go vet -tags integration ./internal/app/ && go test -count=1 ./internal/app/ ./internal/cli/ ./internal/reposetup/`
Expected: PASS.
Run: `go test -count=1 -tags integration -run '^TestIntegrationRepoSetup' ./internal/app/`
Expected: PASS. The existing `TestIntegrationRepoSetupConfigureTestsAmbiguousLeavesFileUntouched` still finds `go`, `rust`, and `docket repository configure-tests` in the new ambiguous text.

- [ ] **Step 9: Mutation-check**

1. In the CLI closure, replace `if c.Flags().Changed("command") {` with `if v, _ := c.Flags().GetString("command"); v != "" {` (adjusting the body to use that `v`). Run `go test -count=1 ./internal/cli/ -run TestRepositoryConfigureTestsCommandFlagFlows`: it must FAIL on the empty case. Restore from a backup copy.
2. In `configureTestsDiscoveryText`, make the `DiscoveryNone` case return the old `"the test policy is already configured; nothing to write"` body. Run `go test -count=1 ./internal/app/ -run TestConfigureTestsDiscoveryText`: the none tests must FAIL. Restore.

- [ ] **Step 10: Commit**

```bash
git add internal/app/repository_configure_tests.go internal/app/repository_init.go internal/cli/repository.go internal/app/repository_configure_tests_test.go internal/app/repository_init_test.go internal/cli/repository_test.go internal/app/reposetup_configure_tests_integration_test.go
git commit -m "feat(repository): configure-tests --command and truthful discovery messages"
```

---

### Task 4: Integration proof on the root-`test.sh` fixture shape

**Files:**
- Test: `internal/app/reposetup_configure_tests_integration_test.go` (append two tests; add `"github.com/danielhanold/docket/internal/config"` to its imports)

**Interfaces:**
- Consumes: `(*initRepo).runConfigureTestsWith` and `ConfigureTestsOptions` (Task 3); existing helpers `newInitRepo`, `newHealthyRepo`, `runInit`, `runCheck`, `commitAndPushMain`, `mustReadFile`, `runGit`, `contains`; `healthySetupYML`; `reposetup.TestConfigMissingCode`, `reposetup.ConfigureTestsCommandRemedy`.
- Produces: nothing consumed later.

- [ ] **Step 1: Write the tests**

```go
// resolveDocketYML resolves the invocation clone's .docket.yml through the real
// config resolver, so asserts read what docket will actually run.
func resolveDocketYML(t *testing.T, r *initRepo) config.Effective {
	t.Helper()
	data := mustReadFile(t, filepath.Join(r.invocation, ".docket.yml"))
	snap, _, err := config.Resolve([]config.Source{{
		Layer: config.LayerRepository, Name: ".docket.yml", Data: data,
	}}, config.ResolveContext{DefaultBranch: "main"})
	if err != nil {
		t.Fatalf("resolving .docket.yml: %v\n%s", err, data)
	}
	return snap.Effective
}

// TestIntegrationRepoSetupConfigureTestsCommandTurnsOnRootTestSh is the
// acceptance-fixture shape: a repository whose only test is a root test.sh
// (no registered detector family) is initialized with both gates off, plain
// configure-tests says so truthfully and names --command, and
// `configure-tests --command "sh ./test.sh"` turns both gates on. configure-tests
// never runs the command, so the file's mode is irrelevant here.
func TestIntegrationRepoSetupConfigureTestsCommandTurnsOnRootTestSh(t *testing.T) {
	r := newInitRepo(t, healthySetupYML, map[string]string{"test.sh": "#!/bin/sh\nexit 0\n"})

	initRes := r.runInit(t)
	if initRes.Result != ResultApplied {
		t.Fatalf("init = %q (%s), want applied", initRes.Result, initRes.HumanText())
	}
	if !strings.Contains(initRes.HumanText(), reposetup.ConfigureTestsCommandRemedy) {
		t.Errorf("init's none note %q must name %q", initRes.HumanText(), reposetup.ConfigureTestsCommandRemedy)
	}
	if eff := resolveDocketYML(t, r); eff.Build.Gate.Value != "off" || eff.Finalize.Gate.Value != "off" {
		t.Fatalf("init on a no-suite repo must write both gates off, got build=%q finalize=%q", eff.Build.Gate.Value, eff.Finalize.Gate.Value)
	}
	r.commitAndPushMain(t, "commit init's pending edits", ".gitignore", ".docket.yml")

	plain := r.runConfigureTests(t)
	if plain.Result != ResultNoOp {
		t.Fatalf("plain configure-tests = %q (%s), want no-op", plain.Result, plain.HumanText())
	}
	if strings.Contains(plain.HumanText(), "already configured") || !strings.Contains(plain.HumanText(), reposetup.ConfigureTestsCommandRemedy) {
		t.Errorf("plain configure-tests on none %q must name --command, not claim already configured", plain.HumanText())
	}

	cmd := "sh ./test.sh"
	res := r.runConfigureTestsWith(t, ConfigureTestsOptions{Command: &cmd})
	if res.Result != ResultApplied || res.RepositoryState != string(reposetup.StateNeedsReview) {
		t.Fatalf("configure-tests --command = %q/%q (%s), want applied/needs-review", res.Result, res.RepositoryState, res.HumanText())
	}
	if !contains(res.PendingPaths, ".docket.yml") {
		t.Errorf("PendingPaths = %v, want .docket.yml", res.PendingPaths)
	}
	if staged := runGit(t, r.invocation, "diff", "--cached", "--name-only"); strings.Contains(staged, ".docket.yml") {
		t.Errorf("configure-tests --command staged .docket.yml; staged: %q", staged)
	}
	eff := resolveDocketYML(t, r)
	if eff.Build.Gate.Value != "local" || eff.Finalize.Gate.Value != "local" ||
		eff.Build.TestCommand.Value != cmd || eff.Finalize.TestCommand.Value != cmd {
		t.Fatalf("resolved policy build=%q/%q finalize=%q/%q, want local/%q for both",
			eff.Build.Gate.Value, eff.Build.TestCommand.Value, eff.Finalize.Gate.Value, eff.Finalize.TestCommand.Value, cmd)
	}

	r.commitAndPushMain(t, "commit the explicit test policy", ".docket.yml")
	check := r.runCheck(t)
	if check.RepositoryState != string(reposetup.StateHealthy) || check.CheckExitCode() != 0 {
		t.Fatalf("check after commit = %q exit %d (%s), want healthy/0", check.RepositoryState, check.CheckExitCode(), check.HumanText())
	}
	for _, f := range check.Findings {
		if f.Code == reposetup.TestConfigMissingCode {
			t.Errorf("check still reports %s: %+v", reposetup.TestConfigMissingCode, f)
		}
	}

	before := mustReadFile(t, filepath.Join(r.invocation, ".docket.yml"))
	again := r.runConfigureTestsWith(t, ConfigureTestsOptions{Command: &cmd})
	if again.Result != ResultNoOp {
		t.Errorf("repeat --command = %q (%s), want no-op", again.Result, again.HumanText())
	}
	if after := mustReadFile(t, filepath.Join(r.invocation, ".docket.yml")); string(before) != string(after) {
		t.Errorf("repeat --command rewrote .docket.yml; want byte-identical")
	}
}

// TestIntegrationRepoSetupConfigureTestsCommandRefusesInvalidInput proves an
// empty, whitespace-only, or `auto` --command is invalid-input and leaves the
// healthy repository's .docket.yml byte-identical with no working-tree change.
func TestIntegrationRepoSetupConfigureTestsCommandRefusesInvalidInput(t *testing.T) {
	r := newHealthyRepo(t)
	before := mustReadFile(t, filepath.Join(r.invocation, ".docket.yml"))
	for _, raw := range []string{"", "   ", "auto"} {
		v := raw
		res := r.runConfigureTestsWith(t, ConfigureTestsOptions{Command: &v})
		if res.Result != ResultInvalidInput {
			t.Errorf("--command %q = %q (%s), want invalid-input", raw, res.Result, res.HumanText())
		}
	}
	if after := mustReadFile(t, filepath.Join(r.invocation, ".docket.yml")); string(before) != string(after) {
		t.Errorf("a refused --command changed .docket.yml")
	}
	if dirty := runGit(t, r.invocation, "status", "--porcelain"); strings.TrimSpace(dirty) != "" {
		t.Errorf("a refused --command left working-tree changes: %q", dirty)
	}
}
```

- [ ] **Step 2: Run them**

Run: `go test -count=1 -tags integration -run '^TestIntegrationRepoSetupConfigureTests' ./internal/app/`
Expected: PASS. These tests pin behavior Tasks 1 to 3 already built, so they pass on first run. To prove they bite, do Step 3.

- [ ] **Step 3: Mutation-check**

In `internal/app/repository_configure_tests.go`, temporarily make the explicit branch call the discovery path (`pendingPath, wrote, discovery, werr = ensureTestPolicyConfig(sc.repo.PrimaryWorktree, sc.cfg)`). Re-run Step 2's command with `-count=1`: `TestIntegrationRepoSetupConfigureTestsCommandTurnsOnRootTestSh` must FAIL (no-op instead of applied). Restore from a backup copy and re-run, PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/app/reposetup_configure_tests_integration_test.go
git commit -m "test(repository): configure-tests --command on the root test.sh fixture"
```

---

### Task 5: Docs describe `--command`

**Files:**
- Modify: `docs/reference/glossary.md` (the setup section near the `docket repository configure-tests    # set the build and finalize test commands` example, and the `### Suite command (...) / configure-tests` entry)
- Modify: `docs/guide/proving-the-build.md` (`## Configuring the suite command`)
- Modify: `docs/concepts/build-tiers-and-gate.md` (the bullet ending "the build halts with the remedy `docket repository configure-tests`.")
- Modify: `docs/guide/where-the-metadata-lives.md` (the `docket repository configure-tests` bullet)

**Interfaces:** none (prose only).

- [ ] **Step 1: Edit the glossary**

In the setup section, replace the sentence `A local gate with no test command halts until one is configured, and \`repository configure-tests\` sets the test commands.` with:

```markdown
A local gate with no test command halts until one is configured, and `repository configure-tests` sets the test commands: from suite discovery, or from `--command "<cmd>"` when discovery cannot find the suite.
```

In the same section's `sh` block, add this line right after the existing `docket repository configure-tests    # set the build and finalize test commands` line:

```sh
docket repository configure-tests --command "sh ./test.sh"   # set both gates to local with this command
```

In `### Suite command (...) / configure-tests`, add this paragraph after the **Used for:** paragraph:

```markdown
Plain `docket repository configure-tests` re-runs suite discovery and writes what it finds. `--command "<cmd>"` skips discovery and sets both `build` and `finalize` to `gate: local` with that command, replacing whatever test policy was there. Either way the `.docket.yml` edit is left unstaged for you to review and commit. Different build and finalize commands are a hand edit.
```

and add to that entry's `sh` block:

```sh
docket repository configure-tests --command "sh ./test.sh" --repo-dir .   # set both gates to local with this command
```

- [ ] **Step 2: Edit the guides and concept page**

`docs/guide/proving-the-build.md`: after the sentence ending `rather than trying to guess a command at runtime.`, insert:

```markdown
Plain `configure-tests` re-runs suite discovery. When discovery cannot find your suite, or finds more than one, pass it yourself: `docket repository configure-tests --command "<cmd>"` sets both gates to `local` with that command and leaves the `.docket.yml` edit unstaged for you to commit.
```

`docs/concepts/build-tiers-and-gate.md`: change the end of that bullet from `the build halts with the remedy \`docket repository configure-tests\`.` to:

```markdown
the build halts with the remedy `docket repository configure-tests` (add `--command "<cmd>"` when discovery cannot find the suite).
```

`docs/guide/where-the-metadata-lives.md`: change the bullet body `writes the build and finalize test commands into \`.docket.yml\` for a repository that is already set up, for you to review and commit.` to:

```markdown
writes the build and finalize test commands into `.docket.yml` for a repository that is already set up, from suite discovery or from `--command "<cmd>"`, for you to review and commit.
```

(Keep each file's existing line-wrapping style. The proving-the-build paragraph wraps near 100 columns.)

- [ ] **Step 3: Check nothing else claims configure-tests only discovers**

Run: `grep -rn -e 'configure-tests' docs/guide docs/concepts docs/reference skills .docket.example.yml`
Expected: every hit is either one you just edited or a remedy mention ("names `docket repository configure-tests` as the remedy") that stays true. Do not edit `skills/**` (see Global Constraints).

- [ ] **Step 4: Commit**

```bash
git add docs/reference/glossary.md docs/guide/proving-the-build.md docs/concepts/build-tiers-and-gate.md docs/guide/where-the-metadata-lives.md
git commit -m "docs: describe repository configure-tests --command"
```

---

### Final gate

- [ ] Run the whole suite from the worktree root: `go run ./cmd/docket development test`. Expected exit `0`. Read the budget report for `tests/test_go_integration_app_reposetup.sh`, `tests/test_go_integration_app_reposetup_race.sh`, and `tests/test_go_integration_app_repomigration.sh`, and record any `BUDGET WATCH:` / `SERIAL CONFIRMED OVER BUDGET:` line in the results.
- [ ] Confirm `git status --porcelain` is clean and `git log --oneline 19206a65d..HEAD` shows the plan commit plus the five task commits.
