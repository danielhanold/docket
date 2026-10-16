<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0548 — Name test fixture folders by what they hold](../../changes/archive/2026-10-16-0548-rename-the-v0-9-13-frozen-fixture-tree-to-the-1-0-release-se.md)**
<!-- docket:backlink:end -->

# Name Test Fixture Folders by What They Hold — Implementation Plan

> **For agentic workers:** this plan is executed by `docket-build`, task by task, in order. Each
> task is owned by one tier worker under the `docket-build-task` contract and ends in one commit.
> Steps use checkbox (`- [ ]`) syntax for tracking. Later tasks build on earlier task commits.

**Goal:** Replace the release-named folders under `testdata/repositories/` (`v0.9.2` … `v0.9.13`)
with folders named for what they hold (`configs/`, `documents/`, `status-corpus/`), stop keeping
byte copies of shipped files under `testdata/`, and add a guard so no version- or date-named
folder comes back.

**Architecture:** The samples move with `git mv` and keep their exact bytes. The two tests that
compared byte copies against shipped files instead read the shipped file directly
(`agents/harness-defaults.yml`, `.docket.yml`) or write their input inline into a temp directory
(the complete deferred-capability config). Everything that only held copies is deleted. A new
`internal/repoguard` test rejects any folder directly under `testdata/repositories/` whose name
matches `^v?[0-9]`.

**Tech Stack:** Go (`go test`), git. No new dependencies.

**Spec:** `docs/superpowers/specs/2026-10-15-rename-the-v0-9-13-frozen-fixture-tree-to-the-1-0-release-se-design.md`
(on the `docket` metadata branch; the coordinator hands it to every worker). Read it alongside
this plan: §1–§7 are implemented by Tasks 1–5 below; §8 is **not** a build task (see the end).

All paths below are relative to the feature worktree root
(`/Users/homer/dev/docket/.worktrees/rename-the-v0-9-13-frozen-fixture-tree-to-the-1-0-release-se`).
Run every `go test` with `-count=1` so a cached result never stands in for a real run.

## Global Constraints

- Every moved sample keeps its exact bytes: git must show each moved sample file as an `R100` rename.
- After this change `testdata/repositories/` holds exactly `configs/`, `documents/`, `status-corpus/`, and the existing `.gitignore`.
- Each of the three folders has exactly one `PROVENANCE.md`, at its root, covering everything inside it.
- No copy of a shipped file, and no fixture that must be kept up to date, lives under `testdata/`.
- Tests never write inside `testdata/`.
- Folder-name guard pattern: `^v?[0-9]`.
- Out of scope, never touched: any sample's contents; renaming individual fixtures (`fenced-machine-keys` keeps its name); `internal/install/` (including `internal/install/legacydata/` and `internal/install/testdata/legacy/`); `testdata/bash-upgrade/`; `internal/bashupgrade/`; package-local `testdata/` directories; frozen build records (old plans, specs, results, archived change records) and ADRs.
- Comments describe what the code does now, without history (no "was re-cut", no change-number timelines).
- Cross-references in maintained source anchor on symbol names or quoted clauses, never line numbers.
- Mutation probes: back up with `cp "$f" "$f.bak"` and restore with `mv -f "$f.bak" "$f"` — never `git checkout --` — and confirm with `git diff` that the mutation landed before reading the result. Record every probe's observed outcome for the results file; never commit a probe.
- Commit messages: Conventional Commits with the `(change 0548)` suffix, matching this repo's history (for example `test(config): … (change 0548)`).

## Review Focus

1. **A developer's machine config reaching `TestFixtureDocketSelf`.** A `.docket.local.yml` at the checkout root, or a real `~/.config/docket/config.yml`, must not change the test's verdict. Pinned by Task 2, Step 6 (local-layer probe) and by `loadFixtureFrom`'s existing `HOME`/`XDG_CONFIG_HOME` pin.
2. **A new numbered folder landing on `main` before this merges** (for example #554 cutting `v0.9.14`). The build must re-list the tree rather than trust the spec's table (Task 2, Step 1 and Task 3, Step 1), and the guard (Task 4) must fail on it if one slips through.
3. **`PROVENANCE.md` being read as data by a folder walker.** `frozenCorpus` (documents) must skip it; the status-corpus loader already does; the document fuzz seeders deliberately keep it as harmless seed bytes. Pinned by Task 3, Steps 6 and 10.
4. **A moved sample silently losing or changing bytes** — in particular the three tracked `.docket.local.yml` samples that the root ignore rules would hide without `testdata/repositories/.gitignore`'s negation, and the `sparse-defaults/repo/.gitkeep` placeholder. Pinned by Task 3, Step 11 (`R100` check and tracked-file count).
5. **The inline deferred-active YAML failing for the wrong reason.** A missing request must fail naming the missing path and `deferredActiveYAML`; a tab-indented or truncated YAML would instead fail as invalid config. Pinned by Task 2, Step 7 (deletion probe on a top-level line) and the constraint that the raw string is space-indented.

---

### Task 1: Agent-defaults parity reads the shipped file

**Files:**
- Modify: `internal/config/defaults_test.go` (the `sidecarPath`/`liveSidecarPath` const block, `sidecarDoc`, `TestBuiltinAgentsParityWithFrozenSidecar`, and the `TestBuiltinAgentsShape` doc comment)
- Modify: `internal/config/defaults.go` (file header comment only)
- Modify: `internal/repoguard/absence_test.go` (the "Config YAML (.yml)" bullet of the header comment only)

**Interfaces:**
- Consumes: `builtinAgents()` (unchanged).
- Produces: const `shippedDefaultsPath = "../../agents/harness-defaults.yml"`; type `shippedDefaultsDoc`; test `TestBuiltinAgentsParityWithShippedDefaults`. After this task nothing in `defaults_test.go` calls `assertFrozenCopyMatchesLive` (Task 2 deletes the helper).

- [ ] **Step 1: Replace the const block and the doc type**

In `internal/config/defaults_test.go`, replace the comment + `const ( sidecarPath … liveSidecarPath … )` block and the `sidecarDoc` type with:

```go
// shippedDefaultsPath is the agent defaults file docket ships, read where it
// lives. The parity test below compares it with the Go built-in table, so an
// edit to it fails here only when it changes what the table must hold.
const shippedDefaultsPath = "../../agents/harness-defaults.yml"

// shippedDefaultsDoc is the shipped-defaults file's shape: harness → agent
// short name → model/effort pair, plus the optional prompt-cache TTL.
type shippedDefaultsDoc struct {
	Agents map[string]map[string]struct {
		Model    string `yaml:"model"`
		Effort   string `yaml:"effort"`
		CacheTTL string `yaml:"cache_ttl"`
	} `yaml:"agents"`
}
```

- [ ] **Step 2: Rewrite the parity test**

Replace the doc comment and the whole body of `TestBuiltinAgentsParityWithFrozenSidecar` with:

```go
// TestBuiltinAgentsParityWithShippedDefaults compares agents/harness-defaults.yml
// against builtinAgents() as WHOLE structures, both directions in one DeepEqual:
// every (harness, agent) pair present in both, models equal, efforts equal, and
// cache TTLs equal — where the file says `auto` the Go table must hold "".
// Vendor-ID VALIDITY is deliberately not asserted: that oracle lives outside
// the repo.
func TestBuiltinAgentsParityWithShippedDefaults(t *testing.T) {
	data, err := os.ReadFile(filepath.FromSlash(shippedDefaultsPath))
	if err != nil {
		t.Fatalf("reading %s: %v", shippedDefaultsPath, err)
	}
	var doc shippedDefaultsDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parsing %s: %v", shippedDefaultsPath, err)
	}
	if len(doc.Agents) == 0 {
		t.Fatalf("%s declares no agents", shippedDefaultsPath)
	}

	want := make(map[string]map[string]pair, len(doc.Agents))
	for harness, agents := range doc.Agents {
		row := make(map[string]pair, len(agents))
		for name, entry := range agents {
			effort := entry.Effort
			if effort == "auto" {
				effort = ""
			}
			row[name] = pair{Model: entry.Model, Effort: effort, CacheTTL: entry.CacheTTL}
		}
		want[harness] = row
	}

	got := make(map[string]map[string]pair, len(builtinAgents()))
	for harness, agents := range builtinAgents() {
		row := make(map[string]pair, len(agents))
		for name, entry := range agents {
			row[name] = pair{Model: entry.Model.Value, Effort: entry.Effort.Value, CacheTTL: entry.CacheTTL.Value}
		}
		got[harness] = row
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("the built-in agent table differs from agents/harness-defaults.yml: "+
			"agents/harness-defaults.yml and the built-in table in internal/config/defaults.go must change together.\n"+
			" got: %v\nwant: %v", got, want)
	}
}
```

In the `TestBuiltinAgentsShape` doc comment, change "independently of the\n// sidecar:" to "independently of the\n// shipped defaults file:". Imports (`os`, `path/filepath`, `reflect`, `strings`, `testing`, `yaml`) all stay in use.

- [ ] **Step 3: Update the `defaults.go` header**

In `internal/config/defaults.go`, replace the last three lines of the file header comment
("it is a table rather than a cell. Its parity oracle is … PROVENANCE.md records.") so the
paragraph ends:

```go
// the 17x4 agent table, which the registry deliberately does not carry because
// it is a table rather than a cell. Its parity oracle is the file docket ships,
// agents/harness-defaults.yml: TestBuiltinAgentsParityWithShippedDefaults reads
// it directly and compares the two, so they must change together.
```

- [ ] **Step 4: Update the `absence_test.go` header bullet**

In `internal/repoguard/absence_test.go`, in the "Config YAML (.yml)" bullet, replace the sentence
starting "It is byte-pinned twice —" through "regenerates the embedded copy." with:

```go
//     reference, so it stays GREEN on its content. It is checked by
//     TestBuiltinAgentsParityWithShippedDefaults, which reads it directly and
//     compares it with the built-in table, and pinned by the embedded-tree copy
//     internal/assets/generate.go writes — so any edit to it must keep the
//     built-in table in step and regenerates the embedded copy.
```

(Keep the preceding words of the bullet — "…it carries no retired" — intact so the sentence reads on.)

- [ ] **Step 5: Run the package tests**

Run: `go test -count=1 ./internal/config/ ./internal/repoguard/`
Expected: PASS. (`go vet` runs as part of `go test`; an unused symbol would fail here.)

- [ ] **Step 6: Mutation probe — an edit to the shipped file fails parity**

```bash
f=agents/harness-defaults.yml
cp "$f" "$f.bak"
# entries are flow maps like `adr: { model: claude-opus-5, effort: low }`; the first `adr:` row is
# claude's. Change only its model token (never the comma), so the file still parses.
perl -pi -e 'if (!$d && s/^(\s+adr:\s+\{ model: )[^,]+/${1}probe-model-x/) { $d = 1 }' "$f"
git diff --stat -- "$f"        # MUST show 1 file changed; if not, the mutation did not land — fix it before reading anything
go test -count=1 ./internal/config/ -run TestBuiltinAgentsParityWithShippedDefaults
mv -f "$f.bak" "$f"
git diff --quiet -- "$f" && echo restored
```

Expected: FAIL whose message contains `must change together`; then `restored`. Record the outcome.

- [ ] **Step 7: Commit**

```bash
git add internal/config/defaults_test.go internal/config/defaults.go internal/repoguard/absence_test.go
git commit -m "test(config): check agent defaults parity against the shipped file (change 0548)"
```

---

### Task 2: Tests that need current input build it themselves

**Files:**
- Modify: `internal/config/fixtures_test.go` (add `writeFixture`, `docketSelfGlobalYAML`, `deferredActiveYAML`; rewrite `TestFixtureDocketSelf`, `TestFixtureDeferredActive`, `repoSettableDeferredPaths`'s dynamic-row message, and the `fixtureAgentHarness`/`loadFixtureFrom`/`mustResolveFixtureFrom` comments; delete `assertFrozenCopyMatchesLive` and the now-unused `bytes` import)

**Interfaces:**
- Consumes: `loadFixtureFrom(t, root, name)`, `mustResolveFixtureFrom(t, root, name)`, `blockerPaths(snap)`, `assertSameStrings`, `repoSettableDeferredPaths(t)`, `PreflightMutation`, `GuardMutation` (all existing, signatures unchanged).
- Produces: `func writeFixture(t *testing.T, name, repoYAML, globalYAML string) string` — returns a root such that `<root>/<name>/repo/.docket.yml` exists (and `<root>/<name>/xdg/docket/config.yml` when `globalYAML != ""`). Consts `docketSelfGlobalYAML`, `deferredActiveYAML`. After this task nothing reads `testdata/repositories/v0.9.12` or `v0.9.13`, and `assertFrozenCopyMatchesLive` no longer exists.

- [ ] **Step 1: Confirm which deferred-active copy is newest**

```bash
ls testdata/repositories
ls -d testdata/repositories/*/deferred-active
```

Expected: the newest numbered folder is `v0.9.13`, and `v0.9.13/deferred-active` is the newest
`deferred-active`. If `main` has gained a newer folder (for example `v0.9.14/deferred-active`),
use that folder's `repo/.docket.yml` as the source for Step 3 instead, and note it for the results
file.

- [ ] **Step 2: Add the `writeFixture` helper**

In `internal/config/fixtures_test.go`, directly after `mustResolveFixtureFrom`, add:

```go
// writeFixture builds a fixture directory from strings in a fresh temp dir:
// <root>/<name>/repo/.docket.yml, plus <root>/<name>/xdg/docket/config.yml when
// globalYAML is non-empty. It returns root, for loadFixtureFrom and
// mustResolveFixtureFrom. A test whose input must keep up with docket builds it
// here rather than reading it from testdata/.
func writeFixture(t *testing.T, name, repoYAML, globalYAML string) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		p := filepath.Join(root, name, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("fixture %s: creating %s: %v", name, filepath.Dir(p), err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatalf("fixture %s: writing %s: %v", name, p, err)
		}
	}
	write("repo/.docket.yml", repoYAML)
	if globalYAML != "" {
		write("xdg/docket/config.yml", globalYAML)
	}
	return root
}
```

Rewrite the two comments above `loadFixtureFrom` and `mustResolveFixtureFrom`:

```go
// loadFixtureFrom is loadFixture over an explicit fixture root instead of
// fixtureRoot — in practice a directory writeFixture built, for the tests whose
// input must keep up with docket (TestFixtureDocketSelf and
// TestFixtureDeferredActive). Every other fixture test uses loadFixture.
```

```go
// mustResolveFixtureFrom is mustResolveFixture over an explicit fixture root —
// see loadFixtureFrom.
```

- [ ] **Step 3: Add the inline deferred-active configuration**

Directly above `TestFixtureDeferredActive`, add the constant. Its body is the content of
`testdata/repositories/v0.9.13/deferred-active/repo/.docket.yml` (or the newer copy from Step 1)
without the file's four leading `#` comment lines and the blank line after them — their meaning
moves into the Go comment. Indent with **spaces only** — YAML rejects tabs. After writing it,
this must print `0` (no tab inside the literal):

```bash
awk '/^const deferredActiveYAML/,/^`$/' internal/config/fixtures_test.go | grep -c "$(printf '\t')"
```

```go
// deferredActiveYAML requests every repository-settable deferred capability at
// once: the complete blocker set in one document. TestFixtureDeferredActive
// derives the expected set from the schema registry rather than from this text,
// so a deferred row added to the registry without a request here fails, naming
// the path to add. The agents block uses fixtureAgentHarness/fixtureAgentName.
const deferredActiveYAML = `finalize:
  gate: ci
  skip_results_only_delta: true

build:
  checkpoint: true

terminal_publish: true
auto_groom: true

auto_capture:
  enabled: true

dummy_mode:
  enabled: true

board_surfaces: [inline, github]

skills:
  brainstorm: docket-brainstorm
  plan: docket-plan
  build: docket-build
  review: docket-review
  finish: docket-finish

agents:
  claude:
    adr:
      model: repo-pinned-model
      effort: high
      cache_ttl: 1h
      runner: codex
`
```

Rewrite the comment on the `fixtureAgentHarness`/`fixtureAgentName` const block:

```go
// fixtureAgentHarness and fixtureAgentName are the concrete names
// deferredActiveYAML uses for the dynamic `agents.<h>.<a>` subtree. The
// derivation in repoSettableDeferredPaths substitutes them for the registry's
// "*" segments, so the inline configuration and the expectation cannot drift
// apart.
```

- [ ] **Step 4: Rewrite `TestFixtureDeferredActive`**

Replace the doc comment and the opening of the test (the `const deferredActiveRoot …` line, the
`mustResolveFixtureFrom` call, and the `assertSameStrings(t, "deferred-active blockers", …)` line)
so the test reads, up to the unchanged remedy/provenance and capability-entry loops:

```go
// TestFixtureDeferredActive: every repo-settable deferred capability requested
// at once, from deferredActiveYAML. The expected blocker set is DERIVED from the
// registry rather than transcribed from the configuration, so adding a deferred
// row without requesting it in deferredActiveYAML fails here, naming the path.
func TestFixtureDeferredActive(t *testing.T) {
	root := writeFixture(t, "deferred-active", deferredActiveYAML, "")
	snap := mustResolveFixtureFrom(t, root, "deferred-active")

	decision := PreflightMutation(snap)
	if decision.Allowed {
		t.Fatalf("preflight allowed a configuration requesting every deferred capability")
	}

	got := blockerPaths(snap)
	want := repoSettableDeferredPaths(t)
	requested := make(map[string]bool, len(got))
	for _, path := range got {
		requested[path] = true
	}
	settable := make(map[string]bool, len(want))
	for _, path := range want {
		settable[path] = true
		if !requested[path] {
			t.Errorf("%s is a repository-settable deferred capability that deferredActiveYAML does not request: "+
				"add it to deferredActiveYAML in internal/config/fixtures_test.go", path)
		}
	}
	for _, path := range got {
		if !settable[path] {
			t.Errorf("deferredActiveYAML requests %s, which blocks but is not a repository-settable deferred registry row", path)
		}
	}
	assertSameStrings(t, "deferred-active blockers", got, want)

	// … the existing "Each blocker names the file and offers a way out" loop and
	// the "Every blocker has a matching capability entry" loop stay unchanged …
}
```

In `repoSettableDeferredPaths`, replace the dynamic-row `t.Fatalf` message with:

```go
			t.Fatalf("registry row %q is a blocking dynamic row with no concrete name in this test: "+
				"add a request for it to deferredActiveYAML in internal/config/fixtures_test.go and extend this substitution", spec.path)
```

- [ ] **Step 5: Rewrite `TestFixtureDocketSelf`**

Replace the doc comment, the `const docketSelfRoot …` block with its history comment, the
`assertFrozenCopyMatchesLive(...)` call, and the `mustResolveFixtureFrom(t, docketSelfRoot, …)`
call; keep every expectation, with each failure message extended by the re-derive remedy. The
whole test becomes:

```go
// docketSelfGlobalYAML is the machine-global layer of the docket-self fixture:
// one deferred capability requested from outside the repository, so the
// blocker envelope spans layers.
const docketSelfGlobalYAML = `auto_capture:
  enabled: true
`

// docketSelfRederive is the remedy every TestFixtureDocketSelf failure carries.
const docketSelfRederive = "docket's own .docket.yml changed what this test checks: " +
	"re-derive these expectations by running the resolver against it and reading its actual output."

// TestFixtureDocketSelf: this repository's own committed .docket.yml, read where
// it lives, plus a global layer requesting auto-capture — the blocker envelope
// docket runs under. Only that one file is copied into the fixture, never the
// checkout root, so a developer's .docket.local.yml cannot reach this test.
func TestFixtureDocketSelf(t *testing.T) {
	self, err := os.ReadFile(filepath.FromSlash("../../.docket.yml"))
	if err != nil {
		t.Fatalf("reading docket's own .docket.yml: %v", err)
	}
	root := writeFixture(t, "docket-self", string(self), docketSelfGlobalYAML)
	snap := mustResolveFixtureFrom(t, root, "docket-self")

	// docket's own gate is the Go-native whole-suite runner, entered from source
	// so the gate tests the exact checkout under review.
	if got, want := snap.Effective.Finalize.TestCommand.Value, "go run ./cmd/docket development test"; got != want {
		t.Errorf("finalize.test_command = %q, want %q.\n%s", got, want, docketSelfRederive)
	}

	// The committed file requests no deferred capability; the machine-global
	// auto-capture request is the sole blocker.
	decision := PreflightMutation(snap)
	if decision.Allowed {
		t.Fatalf("preflight allowed a configuration still requesting global auto-capture.\n%s", docketSelfRederive)
	}
	assertSameStrings(t, "docket-self blockers ("+docketSelfRederive+")", blockerPaths(snap), []string{
		"auto_capture.enabled",
	})

	// The lone blocker comes from the machine-global layer; asserting the layer
	// keeps the test honest about which file the request lives in.
	wantLayer := map[string]LayerKind{
		"auto_capture.enabled": LayerGlobal,
	}
	for _, b := range decision.Blockers {
		if b.Provenance == nil {
			t.Errorf("blocker %s carries no provenance.\n%s", b.Path, docketSelfRederive)
			continue
		}
		if got := b.Provenance.Layer; got != wantLayer[b.Path] {
			t.Errorf("blocker %s came from layer %q, want %q.\n%s", b.Path, got, wantLayer[b.Path], docketSelfRederive)
		}
		if b.Severity != SeverityError {
			t.Errorf("blocker %s severity %q, want error", b.Path, b.Severity)
		}
	}

	// A blocked configuration is still a VALID one: mustResolve already
	// required a nil error, and the guard is what refuses the mutation.
	if err := GuardMutation(snap, func() error { return nil }); !errors.Is(err, ErrUnsupportedConfig) {
		t.Errorf("GuardMutation = %v, want ErrUnsupportedConfig", err)
	}
}
```

Then delete the whole `assertFrozenCopyMatchesLive` function and its doc comment, and remove
`"bytes"` from the import block (that function was its only user — confirm with
`grep -n 'bytes\.' internal/config/fixtures_test.go`, which must print nothing).

- [ ] **Step 6: Run the package and the local-layer probe**

Run: `go test -count=1 ./internal/config/`
Expected: PASS.

Local-layer probe (Review Focus 1):

```bash
test ! -e .docket.local.yml || { echo "a .docket.local.yml already exists — stop and report"; exit 1; }
printf 'auto_groom: true\n' > .docket.local.yml
go test -count=1 ./internal/config/ -run TestFixtureDocketSelf
rm -f .docket.local.yml
```

Expected: PASS (the root `.docket.local.yml` never reaches the fixture). Record the outcome.

- [ ] **Step 7: Mutation probes**

Probe A — `.docket.yml` edit fails `TestFixtureDocketSelf` with the re-derive message:

```bash
f=.docket.yml
cp "$f" "$f.bak"
# the first `  test_command:` line in .docket.yml is the one inside the `finalize:` block
perl -pi -e 'if (!$d && s/^(  test_command: ).*/${1}probe-command/) { $d = 1 }' "$f"
git diff -- "$f"          # MUST show the finalize test_command line changed before you read the result
go test -count=1 ./internal/config/ -run TestFixtureDocketSelf
mv -f "$f.bak" "$f"; git diff --quiet -- "$f" && echo restored
```

If `git diff` shows a line outside the `finalize:` block changed, the probe hit the wrong key:
restore and target the `finalize:` block's `test_command:` line before reading any result.

Expected: FAIL containing `re-derive these expectations`; then `restored`.

Probe B — deleting a request from the inline YAML names the missing path and the constant:

```bash
f=internal/config/fixtures_test.go
cp "$f" "$f.bak"
perl -0pi -e 's/\nauto_groom: true\n/\n/' "$f"
git diff --stat -- "$f"
go test -count=1 ./internal/config/ -run TestFixtureDeferredActive
mv -f "$f.bak" "$f"
grep -c '^auto_groom: true$' "$f"   # must print 1: the request is back
```

Expected: FAIL containing `auto_groom is a repository-settable deferred capability that deferredActiveYAML does not request` and `add it to deferredActiveYAML`; then `1`. Record both outcomes.
(After the restore, `git diff` still shows your uncommitted Task 2 edits — that is correct; the
`.bak` held them, which is why the restore is a copy-back and never `git checkout --`.)

- [ ] **Step 8: Commit**

```bash
git add internal/config/fixtures_test.go
git commit -m "test(config): build docket-self and deferred-active input inside the tests (change 0548)"
```

---

### Task 3: Move the samples into folders named for what they hold

**Files:**
- Move (`git mv`, bytes unchanged): `testdata/repositories/v0.9.2/{sparse-defaults,example-activated,four-layer-collision,mode-docket,mode-main-custom-paths,fenced-machine-keys,deferred-pairs,deferred-active,invalid}/` → `testdata/repositories/configs/<same name>/`; `testdata/repositories/v0.9.2/documents/` → `testdata/repositories/documents/`; `testdata/repositories/v0.9.3/status-corpus/` → `testdata/repositories/status-corpus/`
- Create: `testdata/repositories/configs/PROVENANCE.md`, `testdata/repositories/documents/PROVENANCE.md`
- Modify: `testdata/repositories/status-corpus/PROVENANCE.md` (title path; delete one sentence)
- Delete: everything left under `testdata/repositories/v0.9.2/` and `v0.9.3/` (`PROVENANCE.md`, `agents-harness-defaults.yml`, `docket-self/`), and `testdata/repositories/v0.9.4/` … `v0.9.13/`
- Modify: `internal/config/fixtures_test.go` (`fixtureRoot`, file header comment, the `TestFixtureModeMainCustomPaths` "the frozen fixture still carries" comment)
- Modify: `cmd/docket/config_cli_test.go` (`copyFixtureRepo` root and comment)
- Modify: `internal/document/fixtures_test.go` (`frozenDir`, its comment, `frozenCorpus` skips `PROVENANCE.md`)
- Modify: `internal/app/status_corpus_test.go` (`corpusRelDir` and its comment)
- Modify: `internal/repoguard/repoguard.go` (the `testdata` exclusion comment)
- Modify: `internal/repoguard/absence_test.go`, `internal/repoguard/repoguard_test.go`, `internal/repoguard/retired_vocabulary_test.go` (synthetic paths / negative control)

**Interfaces:**
- Consumes: Tasks 1–2 (nothing reads `v0.9.12`/`v0.9.13` copies any more).
- Produces: the final layout `configs/`, `documents/`, `status-corpus/`, `.gitignore`; `fixtureRoot = "../../testdata/repositories/configs"`; `frozenDir = "../../testdata/repositories/documents"`; `corpusRelDir = "../../testdata/repositories/status-corpus"`.

- [ ] **Step 1: Re-list the tree and confirm no reader is left on the old folders**

```bash
ls -A testdata/repositories
git grep -n -e 'repositories/v0\.' -e '"v0\.9\.' -- '*.go' ':!internal/install' ':!internal/bashupgrade'
```

Expected listing: `.gitignore v0.9.10 v0.9.11 v0.9.12 v0.9.13 v0.9.2 v0.9.3 v0.9.4 v0.9.5 v0.9.6 v0.9.7 v0.9.8 v0.9.9`.
If a newer numbered folder exists, apply the spec §1 rule to it: a folder holding only copies of
shipped files is deleted with the others in Step 3 (and anything that reads it is re-pointed the
way Tasks 1–2 did); note it for the results file.
The grep must show only the readers this task changes: `fixtureRoot` and the `frozenDir`/`corpusRelDir`
constants, `copyFixtureRepo`'s `"v0.9.2"` join, and the synthetic repoguard paths and comments.
A `"v0.9.x"` literal that does not build a path under `testdata/repositories/` (for example a
release-tag string) is not a reader — leave it. Any other hit that does build such a path is a
reader the spec did not foresee: re-point it to the new folder in this task and note it for the
results file.

- [ ] **Step 2: Move the samples**

```bash
cd testdata/repositories
mkdir -p configs
for f in sparse-defaults example-activated four-layer-collision mode-docket mode-main-custom-paths \
         fenced-machine-keys deferred-pairs deferred-active invalid; do
  git mv "v0.9.2/$f" "configs/$f" || exit 1
done
git mv v0.9.2/documents documents
git mv v0.9.3/status-corpus status-corpus
cd -
```

- [ ] **Step 3: Delete the copies and the unread folders**

```bash
git rm -r -q testdata/repositories/v0.9.2 testdata/repositories/v0.9.3 \
  testdata/repositories/v0.9.4 testdata/repositories/v0.9.5 testdata/repositories/v0.9.6 \
  testdata/repositories/v0.9.7 testdata/repositories/v0.9.8 testdata/repositories/v0.9.9 \
  testdata/repositories/v0.9.10 testdata/repositories/v0.9.11 testdata/repositories/v0.9.12 \
  testdata/repositories/v0.9.13
ls -A testdata/repositories
```

Expected: `.gitignore configs documents status-corpus`. (Before Step 3, `v0.9.2/` holds only
`PROVENANCE.md`, `agents-harness-defaults.yml`, `docket-self/`, and `v0.9.3/` holds only
`PROVENANCE.md`, `agents-harness-defaults.yml`. Read `v0.9.2/PROVENANCE.md` from `HEAD` with
`git show HEAD:testdata/repositories/v0.9.2/PROVENANCE.md` while writing Step 4.)

- [ ] **Step 4: Write `configs/PROVENANCE.md`**

Create `testdata/repositories/configs/PROVENANCE.md` with exactly:

```markdown
# Provenance — `testdata/repositories/configs/`

- **Source repo:** `danielhanold/docket`
- **Commit:** `096c48de`
- **Date:** 2026-08-13
- **Redaction:** none

This one file covers everything in `configs/`, per `testdata/README.md`. A sample added later
adds one entry below.

These configurations were hand-written at the commit above to look like configurations written
for docket at the time. None was captured from a real repository. Each holds a `repo/` directory
(the repository handed to `FSOptions.RepoDir`) and, where the sample needs a global layer, an
`xdg/docket/config.yml`. Configuration files only.

## Contents

- `sparse-defaults/` — a repository with no configuration files at all (the
  `repo/.gitkeep` placeholder exists only so git tracks the empty directory).
- `example-activated/` — every supported setting declared explicitly at its
  shipped default.
- `four-layer-collision/` — the same leaves declared in the global, repository,
  and repository-local layers, with scalar, list, nested, and agent collisions.
- `mode-main-custom-paths/` — main-mode repository with an explicit integration
  branch and all three docket directories relocated.
- `mode-docket/` — docket-mode repository leaving `integration_branch: auto`.
- `fenced-machine-keys/` — every repo-fenced setting declared from a machine
  layer, split between the global and repository-local files.
- `deferred-pairs/` — every deferred setting that has an inactive spelling,
  declared inactive.
- `deferred-active/` — the deferred capabilities a repository could request at that commit, all
  switched on; the command-line tests use it as a configuration docket refuses to write under.
  The complete, current list is written inline in `TestFixtureDeferredActive`.
- `invalid/<reason>/` — nine single-fault documents, one per rejection reason:
  `malformed`, `duplicate-key`, `alias-merge`, `multi-doc`, `wrong-type`,
  `bad-enum`, `scalar-auto-capture`, `unknown-key`, `model-typo`.
```

- [ ] **Step 5: Write `documents/PROVENANCE.md` and edit `status-corpus/PROVENANCE.md`**

Create `testdata/repositories/documents/PROVENANCE.md` with exactly:

```markdown
# Provenance — `testdata/repositories/documents/`

- **Source repo:** `danielhanold/docket`
- **Redaction:** none — every record is byte-exact.

This one file covers everything in `documents/`, per `testdata/README.md`. A sample added later
adds one entry below.

## Contents

Five real records for the document layer's compatibility corpus:

- three from tag `v0.9.2`: archived change 0298 (`archived-change-0298.md`), ADR 0092
  (`adr-0092.md`), and the 0298 plan with its backlink block (`plan-with-backlink.md`);
- two from the `docket` metadata branch at commit `6d34592c`, which is where active changes and
  learnings live: active change 0158 (`active-change-0158.md`) and learning finding
  `frontmatter-anchored-read` (`learning-frontmatter-anchored-read.md`).
```

In `testdata/repositories/status-corpus/PROVENANCE.md`:
- change the title line to `# Provenance — \`testdata/repositories/status-corpus/\``;
- delete the sentence "It coexists with — and never relabels or absorbs — the agent-defaults
  sidecar that shares this versioned tree; that sidecar keeps its own provenance in
  `../PROVENANCE.md`." (it spans three lines; re-wrap the paragraph so it ends after "not the whole
  tree."). Change nothing else in the file.

- [ ] **Step 6: Re-point the readers**

`internal/config/fixtures_test.go` — replace the file header comment and the `fixtureRoot` lines:

```go
// This file is the end-to-end tier: it drives the real filesystem adapter over
// repository fixtures and asserts what Resolve makes of a whole repository,
// rather than what one stage makes of one node. Every other test file in this
// package builds its sources in memory; these read bytes from disk, so a change
// that only looks right against a hand-written literal is caught here.
//
// Most fixtures are the configuration samples under
// testdata/repositories/configs/. They are samples of data docket must keep
// reading: a failing test here means docket broke a configuration that already
// exists, so a sample is never edited to make a test pass (testdata/README.md).
// A test whose input must keep up with docket builds it with writeFixture.
// Tests never write inside testdata/.

// fixtureRoot is the configuration-sample folder, relative to this package's
// directory.
const fixtureRoot = "../../testdata/repositories/configs"
```

In `TestFixtureModeMainCustomPaths`, change "the frozen fixture still\n\t\t// carries" to
"the sample still\n\t\t// carries" (wording only).

`cmd/docket/config_cli_test.go` — replace the `copyFixtureRepo` comment and `src` line:

```go
// copyFixtureRepo copies a configuration sample's repo/ tree
// (testdata/repositories/configs/<fixture>/repo) into a fresh temp directory,
// so the command under test points at a writable copy. Tests never write
// inside testdata/.
func copyFixtureRepo(t *testing.T, fixture string) string {
	t.Helper()
	src := filepath.Join("..", "..", "testdata", "repositories", "configs", fixture, "repo")
```

`internal/document/fixtures_test.go` — replace the `frozenDir` comment and value, and skip the
provenance file in `frozenCorpus`:

```go
// frozenDir holds byte-exact real records captured at pinned commits;
// testdata/repositories/documents/PROVENANCE.md records the pins. They are
// samples of records docket must keep reading, so they deliberately do not
// track their live originals and are never edited to make a test pass (see
// testdata/README.md).
const frozenDir = "../../testdata/repositories/documents"

func frozenCorpus(t *testing.T) map[string][]byte {
	t.Helper()
	entries, err := os.ReadDir(frozenDir)
	if err != nil {
		t.Fatalf("frozen corpus missing: %v", err)
	}
	out := map[string][]byte{}
	for _, e := range entries {
		if e.Name() == "PROVENANCE.md" {
			continue // documentation, not a record
		}
		b, err := os.ReadFile(filepath.Join(frozenDir, e.Name()))
```

(the rest of `frozenCorpus`, including the "want the 5 captured records" check, is unchanged).
Leave `internal/document/fuzz_test.go` alone: its seeders read every file in `frozenDir`, and the
provenance file is harmless extra seed bytes — the fuzz properties hold for any input.

`internal/app/status_corpus_test.go` — replace the `corpusRelDir` comment and value:

```go
// corpusRelDir is the status semantic corpus captured from tag v0.9.3,
// repo-relative from this package directory (internal/app). Its provenance —
// tag v0.9.3, peeled commit dd742abd5e9fcdf8ffe78eb6f36a293410873bbf — is
// recorded in testdata/repositories/status-corpus/PROVENANCE.md.
const corpusRelDir = "../../testdata/repositories/status-corpus"
```

- [ ] **Step 7: Update the repoguard comment and synthetic paths**

`internal/repoguard/repoguard.go` — in the `testdata` bullet, replace
"the top-level testdata/ release-versioned\n//     repository corpora (testdata/repositories/v0.9.2..v0.9.6, PROVENANCE.md),"
with:

```go
//     frozen v0.9.x recorded corpus), the top-level testdata/ repository
//     samples (testdata/repositories/<kind>/, folders named for what they
//     hold, each with a PROVENANCE.md),
```

(keep the surrounding bullet text — "This single rule covers internal/repository/testdata (the"
before it and "internal/install/testdata/legacy (adopted pre-migration input), and every" after it —
so the sentence still reads).

`internal/repoguard/absence_test.go` — change the synthetic map key
`"testdata/repositories/v0.9.6/x.sh"` to `"testdata/repositories/configs/x.sh"` (re-align the
map's value column with `gofmt -w internal/repoguard/absence_test.go` if needed).

`internal/repoguard/repoguard_test.go` — change both occurrences of
`"testdata/repositories/v0.9.2/x.md"` to `"testdata/repositories/configs/x.md"` (the `writeFile`
call and the expected-excluded list entry).

`internal/repoguard/retired_vocabulary_test.go` — change the negative control
`"the frozen fixture testdata/repositories/v0.9.2/fenced-machine-keys/ keeps its name"` to
`"the frozen fixture testdata/repositories/configs/fenced-machine-keys/ keeps its name"`.

- [ ] **Step 8: gofmt**

Run: `gofmt -l internal cmd`
Expected: no output (run `gofmt -w` on any file listed, then re-run).

- [ ] **Step 9: Run the affected packages**

```bash
go test -count=1 ./internal/config/ ./internal/document/ ./internal/repoguard/
go test -count=1 ./internal/app/ -run 'TestStatusCorpus'
go test -count=1 ./cmd/docket/ -run 'TestConfig'
```

Expected: all PASS. If `./cmd/docket/ -run TestConfig` matches no test that calls
`copyFixtureRepo`, find its two callers with `grep -n 'copyFixtureRepo(t' cmd/docket/config_cli_test.go`
and run them by name.

- [ ] **Step 10: Confirm the provenance file is not read as a record**

```bash
ls testdata/repositories/documents
res=$(go test -count=1 ./internal/document/ -run 'Corpus' -v 2>&1); echo "exit=$?"
grep -c -- '--- PASS' <<<"$res"
grep -- '--- FAIL' <<<"$res" || echo "no failures"
```

Expected: the listing shows the five `.md` records plus `PROVENANCE.md`; `exit=0`; a non-zero PASS
count; `no failures`. Then observe what the skip protects: with a `cp` backup of
`internal/document/fixtures_test.go`, delete the three-line `PROVENANCE.md` skip, re-run the same
command, record whether anything fails (the provenance file has no frontmatter, so a pass is
possible — the skip is there so the corpus holds only records, not to satisfy a specific assert),
and restore with `mv -f`.

- [ ] **Step 11: Commit, then verify bytes and tracked files**

```bash
git add -A testdata/repositories internal/config/fixtures_test.go cmd/docket/config_cli_test.go \
  internal/document/fixtures_test.go internal/app/status_corpus_test.go internal/repoguard/repoguard.go \
  internal/repoguard/absence_test.go internal/repoguard/repoguard_test.go internal/repoguard/retired_vocabulary_test.go
git commit -m "test: name repository sample folders for what they hold (change 0548)"
git status --porcelain     # must print nothing

out=$(git diff -M100% --name-status HEAD~1 HEAD -- testdata/repositories)
printf '%s\n' "$out" | awk '$1 !~ /^R100/ && $1 != "D"'
printf '%s\n' "$out" | awk '$1 == "D" && $2 !~ /^testdata\/repositories\/v0\.9\.[0-9]+\//'
git ls-files testdata/repositories | grep -c -- '\.docket\.local\.yml$'
git ls-files testdata/repositories/configs/sparse-defaults
```

Expected:
- the first `awk` prints exactly three lines, all `A`: `testdata/repositories/configs/PROVENANCE.md`,
  `testdata/repositories/documents/PROVENANCE.md`, `testdata/repositories/status-corpus/PROVENANCE.md`
  — every other moved file is `R100`;
- the second `awk` prints nothing (every deletion is under a `v0.9.*` folder);
- the `.docket.local.yml` count is `3`;
- `configs/sparse-defaults/repo/.gitkeep` is listed.

Record this output for the results file. If a moved sample shows as anything but `R100`, its bytes
changed: restore it from `HEAD~1` with `git show HEAD~1:<old path> > <new path>`, amend, and re-check.

---

### Task 4: Folder-name guard

**Files:**
- Create: `internal/repoguard/sample_folder_names_test.go`

**Interfaces:**
- Consumes: `guardRoot(t)` (`internal/repoguard/guards_test.go`), Task 3's final layout.
- Produces: `func sampleFolderNameAllowed(name string) bool`; tests `TestSampleFolderNameAllowed` and `TestSampleFoldersNamedForWhatTheyHold`.

- [ ] **Step 1: Write the unit test for the name check**

Create `internal/repoguard/sample_folder_names_test.go`:

```go
package repoguard

// Guard: every folder directly under testdata/repositories/ is named for what
// it holds (configs, documents, status-corpus), never for a release or a date.
// Where a sample came from belongs in its folder's PROVENANCE.md, and tagging a
// release creates nothing here (testdata/README.md).
//
// Shape: a name that starts with a digit, optionally after a "v", looks like a
// version or a date — `^v?[0-9]`.
//
// Mutation-tested: an empty testdata/repositories/v0.9.14/ reddens
// TestSampleFoldersNamedForWhatTheyHold.

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestSampleFolderNameAllowed(t *testing.T) {
	for _, name := range []string{"configs", "documents", "status-corpus", "visibility-modes"} {
		if !sampleFolderNameAllowed(name) {
			t.Errorf("sampleFolderNameAllowed(%q) = false, want true", name)
		}
	}
	for _, name := range []string{"v0.9.14", "v1.0.0-rc.1", "1.0.0", "2026-10-11"} {
		if sampleFolderNameAllowed(name) {
			t.Errorf("sampleFolderNameAllowed(%q) = true, want false", name)
		}
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test -count=1 ./internal/repoguard/ -run TestSampleFolderNameAllowed`
Expected: FAIL to compile — `undefined: sampleFolderNameAllowed`.

- [ ] **Step 3: Implement the check and the live guard**

Append to the same file:

```go
// versionOrDateName matches a folder name shaped like a version or a date.
var versionOrDateName = regexp.MustCompile(`^v?[0-9]`)

// sampleFolderNameAllowed reports whether name is acceptable for a folder
// directly under testdata/repositories/.
func sampleFolderNameAllowed(name string) bool {
	return !versionOrDateName.MatchString(name)
}

func TestSampleFoldersNamedForWhatTheyHold(t *testing.T) {
	dir := filepath.Join(guardRoot(t), "testdata", "repositories")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	folders := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		folders++
		if !sampleFolderNameAllowed(e.Name()) {
			t.Errorf("testdata/repositories/%s/ is named like a version or a date. "+
				"Name sample folders for what they hold, and record the release or date in the folder's PROVENANCE.md. "+
				"See testdata/README.md.", e.Name())
		}
	}
	if folders == 0 {
		t.Fatalf("no folders under %s: the guard has nothing to check", dir)
	}
}
```

- [ ] **Step 4: Run both tests**

Run: `go test -count=1 ./internal/repoguard/ -run 'TestSampleFolder'`
Expected: PASS (both tests).

- [ ] **Step 5: Mutation probe — a numbered folder reddens the guard**

```bash
mkdir testdata/repositories/v0.9.14
go test -count=1 ./internal/repoguard/ -run TestSampleFoldersNamedForWhatTheyHold
rmdir testdata/repositories/v0.9.14
git status --porcelain testdata/repositories   # must be empty
```

Expected: FAIL naming `testdata/repositories/v0.9.14/` and `PROVENANCE.md`; then a clean status.
Second probe: replace the body of `sampleFolderNameAllowed` with `return true` (cp backup first),
run `go test -count=1 ./internal/repoguard/ -run TestSampleFolderNameAllowed`, expect FAIL on the
four rejected names, restore with `mv -f`. Record both outcomes.

- [ ] **Step 6: Run the whole repoguard package**

Run: `go test -count=1 ./internal/repoguard/`
Expected: PASS (the new file must satisfy the package's own hygiene guards, e.g. no bare temp dirs,
no line-number anchors).

- [ ] **Step 7: Commit**

```bash
git add internal/repoguard/sample_folder_names_test.go
git commit -m "test(repoguard): reject version- or date-named sample folders (change 0548)"
```

---

### Task 5: Rewrite `testdata/README.md` and sweep for the old rule

**Files:**
- Modify: `testdata/README.md`
- Modify: any maintained file the Step 3 sweep finds still stating the old rule (expected: none)

**Interfaces:**
- Consumes: Tasks 1–4 (names `TestSampleFoldersNamedForWhatTheyHold`, `configs/`, `documents/`, `status-corpus/`).
- Produces: documentation only.

- [ ] **Step 1: Rewrite the repository-fixtures section**

In `testdata/README.md`, replace everything from the line
`` ## Root `testdata/repositories/v0.9.2/<fixture-name>/` `` up to (not including)
`` ## Root `testdata/bash-upgrade/<tag>/` `` with exactly:

```markdown
## Root `testdata/repositories/<kind>/`

Repository samples shared across packages, in folders named for what they hold: `configs/`
(configuration files), `documents/` (records), and `status-corpus/` (a repository snapshot for
`docket status`). Where a sample came from is recorded in its folder's `PROVENANCE.md`, never in a
folder name. Tagging a release creates nothing here, and `TestSampleFoldersNamedForWhatTheyHold`
(`internal/repoguard`) fails on a folder named like a version or a date.

These are samples of data docket must keep reading. A failing test here means docket broke
something that already exists in real repositories, so don't edit a sample to make the test pass.
Remove a sample only when docket deliberately drops support for it, and record why in its
provenance file. Adding new samples is always fine.

- **What doesn't live here:** input that must keep up with docket. A test that checks a file
  docket ships reads it where it lives. A test that needs a complete, current list builds its
  input itself.
- **Provenance:** one `PROVENANCE.md` at each folder's root covers everything in that folder:
  source repo, commit or tag, date, and any redaction. A test that walks a folder skips it.
- **Unread samples are deleted;** git history keeps them.
- **Copy before mutation:** a test that needs to mutate a sample copies it into its own temp
  directory first and mutates the copy. Tests never write inside `testdata/`.
- **Expected outputs live with the test:** expected transformed output belongs beside the owning
  test (its package `testdata/`), never inside `testdata/repositories/`.

```

- [ ] **Step 2: Fix the `bash-upgrade` bullet and drop the closing paragraph**

Replace the bullet

```markdown
- **Immutable**, exactly like the versioned trees above: a different state is a new case, never
  an edit. Tests restore into their own temp directories and never write here.
```

with

```markdown
- **Never edited to make a test pass**, like the samples above: a different state is a new case.
  Tests restore into their own temp directories and never write here.
```

Delete the final paragraph ("Change 0304 establishes the convention only; the first frozen
fixtures arrive with the changes that need them (0305 configuration, 0306 documents).") and the
blank line before it, so the file ends after the `**Temporary:**` bullet with a single newline.
The multi-owner provenance exception is gone with the Step 1 replacement; confirm with
`grep -n -i 'multiple independent owners' testdata/README.md` (no output).

- [ ] **Step 3: Final grep (spec §5)**

```bash
out=$(git grep -n -i -e 'repositories/v0\.' -e 'sidecarPath' -e 'frozen sidecar' -e 'FrozenSidecar' \
  -e 'versioned tree' -e 'versioned fixture tree' -e 'assertFrozenCopyMatchesLive' -- . \
  ':!docs/adrs' ':!docs/superpowers' ':!docs/changes' ':!docs/results' \
  ':!internal/install' ':!testdata/bash-upgrade' ':!internal/bashupgrade'); rc=$?
if [ "$rc" -eq 1 ]; then echo clean; else printf '%s\n' "$out"; echo "rc=$rc"; fi
```

Expected: `clean`. Any hit outside the exclusions is a maintained file still naming the old layout:
fix it in this task. (rc 2 is a grep error, not a clean result.)

Old-rule wording sweep over the files this change touched:

```bash
git grep -n -i -e 'immutable input' -e 'frozen copy' -e 're-cut' -e 'new versioned' -e 'cut a new' -- \
  internal/config internal/document internal/app/status_corpus_test.go cmd/docket/config_cli_test.go \
  internal/repoguard testdata/README.md testdata/repositories
```

Expected: no hit that states the old "never edit, cut a new folder" rule. Hits inside
`testdata/repositories/status-corpus/PROVENANCE.md` are sample provenance and stay unedited.

- [ ] **Step 4: Verify the layout**

```bash
ls -A testdata/repositories
```

Expected: exactly `.gitignore configs documents status-corpus`.

- [ ] **Step 5: Commit**

```bash
git add testdata/README.md
git commit -m "docs(testdata): describe sample folders and the never-edit-to-pass rule (change 0548)"
```

(If Step 3 required fixes in other files, add them to the same commit.)

---

## Build gate

After Task 5, the build role runs the whole suite through `build.test_command` as resolved from
config (`go run ./cmd/docket development test`, entered from source), never only these packages,
and reads the budget report. The results file records: the five mutation/probe outcomes from the
spec's Testing section (Task 1 Step 6, Task 2 Steps 6–7, Task 4 Step 5), the `ls` listing, the
`R100` check output (Task 3 Step 11), and the clean §5 grep (Task 5 Step 3).

## Not a build task — spec §8 (learnings)

Updating the learnings finding `config-edit-trips-its-own-frozen-drift-guard` (new `hook`,
rewritten `## Apply`, `changes` gains 548, war story kept) is a metadata-branch operation done by
the coordinator through `learning.update`, not on the feature branch. No worker edits anything
under the metadata worktree, and no task above touches the learnings ledger.
