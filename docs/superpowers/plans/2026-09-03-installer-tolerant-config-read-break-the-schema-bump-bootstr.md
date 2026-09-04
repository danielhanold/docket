<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0392 — Installer-tolerant config read: break the schema-bump bootstrap deadlock](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0392-installer-tolerant-config-read-break-the-schema-bump-bootstr.md)**
<!-- docket:backlink:end -->
# Installer-Tolerant Config Read Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make docket's three install operations (`install`, `install check`, `development install`) tolerate unknown configuration keys — degrading the `unknown-key` diagnostic from error to warning on the install path only — so a schema-extending merge can no longer deadlock the installer, while every other command keeps the strict typo policy.

**Architecture:** One boolean, `config.ResolveContext.TolerateUnknownKeys`, reclassified in a single function inside `internal/config/resolve.go` (never in the decoder). Exactly one call site sets it: the CLI's shared `installOptions`, on both of its config reads (the global-only read and the repository-phase read inside `app.ResolveRepoPhase`, which gains the `ResolveContext` as a parameter). The install reads stop discarding diagnostics: warnings flow `config.Resolve → installOptions / ResolveRepoPhase → install.Options.ConfigWarnings → app.InstallResult.Warnings` (JSON `warnings`, one human line each). An ADR records the decision and AGENTS.md's schema-bump caveat shrinks to one sentence.

**Tech Stack:** Go (stdlib + existing internal packages only; no new dependencies). Suite gate: `go run ./cmd/docket development test`.

**Spec:** `docs/superpowers/specs/2026-09-03-installer-tolerant-config-read-break-the-schema-bump-bootstr-design.md` (on the `docket` metadata branch; synchronized copy at `.docket/docs/superpowers/specs/…` in the primary tree).

## Global Constraints

- Tolerance degrades **only** `CodeUnknownKey` diagnostics whose severity is `SeverityError`, at any depth. `invalid-yaml`, `duplicate-key`, `invalid-type`, `invalid-value` stay fatal everywhere; the coordination fence (`fenced-setting-ignored`, ADR-0019) is untouched.
- With `TolerateUnknownKeys` false (the zero value), the resolver must be byte-for-byte today's behaviour.
- The flag is set at exactly one site: `installOptions` in `internal/cli/install.go`. No other caller — `diagnostic config`, `status`, `repository.prepare`, `change.*`, `finalize.*` — may set it.
- The two pre-existing warn-and-ignore surfaces (unknown `skills.<role>`, unknown `board.sorting.<section>`) already emit `unknown-key` at `SeverityWarning` with their own messages; the reclassifier must not touch them (it keys on `SeverityError`).
- The tolerated diagnostic's `Message` is kept; its `Remedy` becomes one shared package constant naming both causes (newer docket, or typo).
- Every warning-severity diagnostic from the install-path reads is surfaced — no filtering to tolerated keys only.
- `development install` relay protocol is preserved: the parent (old binary) prints nothing; the candidate prints the sole document. No parent-side stderr line.
- No new config key: `.docket.yml.example` and the config reference need no edit.
- Go tests: always run with `-count=1` (the module cache serves stale greens otherwise).
- Repo-root `AGENTS.md` is the real file; `CLAUDE.md` is a symlink to it — edit `AGENTS.md` only.

---

### Task 1: `config.ResolveContext.TolerateUnknownKeys` and the reclassifier

**Files:**
- Modify: `internal/config/config.go` (add field to `ResolveContext`; add `Warnings` helper near the `Diagnostic` type)
- Modify: `internal/config/resolve.go` (remedy constant + reclassifier, applied inside `resolve` after the decode loop, before the first `hasInvalid`)
- Test: `internal/config/resolve_test.go`

**Interfaces:**
- Consumes: existing `resolve(sources, rctx)`, `Resolve(sources, rctx)`, `Diagnostic`, `CodeUnknownKey`, `CodeFencedIgnored`, severities, and the test helpers `srcG`/`srcR`, `mainCtx`, `mustResolve`, `diagsWithCode`, `effectiveLeaf`.
- Produces (later tasks rely on these exact names):
  - `config.ResolveContext{DefaultBranch string, TolerateUnknownKeys bool}`
  - `config.ToleratedUnknownKeyRemedy` (exported string constant)
  - `config.Warnings(diags []Diagnostic) []Diagnostic` (filter to `SeverityWarning`)

- [ ] **Step 1: Write the failing tests**

Append to `internal/config/resolve_test.go` (spec Testing items 1–5; item 4 is the mutation control — deleting the reclassifier call must redden it and only it among the tolerance tests):

```go
// TestTolerateUnknownKeysDegradesToWarning holds the install path's tolerance
// rule (change 0392): with the flag on, an unknown key — top-level or nested —
// is a WARNING carrying the shared remedy, the snapshot stays valid, and the
// unknown subtree resolves to defaults.
func TestTolerateUnknownKeysDegradesToWarning(t *testing.T) {
	rctx := ResolveContext{DefaultBranch: "main", TolerateUnknownKeys: true}
	for name, yml := range map[string]string{
		"top-level": "some_future_block:\n  enabled: true\n",
		"nested":    "finalize:\n  some_new_field: 7\n",
	} {
		t.Run(name, func(t *testing.T) {
			res := mustResolve(t, []Source{srcR(yml)}, rctx)
			warns := diagsWithCode(res, CodeUnknownKey)
			if len(warns) != 1 {
				t.Fatalf("unknown-key diagnostics = %v, want exactly 1", diagSummary(res))
			}
			d := warns[0]
			if d.Severity != SeverityWarning {
				t.Errorf("severity = %s, want warning", d.Severity)
			}
			if d.Remedy != ToleratedUnknownKeyRemedy {
				t.Errorf("remedy = %q, want the shared ToleratedUnknownKeyRemedy", d.Remedy)
			}
			if d.Message == "" {
				t.Errorf("message was dropped; the reclassifier must keep it")
			}
			// The unknown subtree contributed no leaves: a known sibling leaf
			// still resolves to its built-in default, non-explicit.
			if _, _, explicit := effectiveLeaf(t, res.effective, "finalize.gate"); name == "nested" && explicit {
				t.Errorf("finalize.gate became explicit; the unknown subtree must contribute nothing")
			}
		})
	}
}

// TestTolerateUnknownKeysLeavesOtherClassesFatal proves the option changes
// nothing for the other invalid classes.
func TestTolerateUnknownKeysLeavesOtherClassesFatal(t *testing.T) {
	rctx := ResolveContext{DefaultBranch: "main", TolerateUnknownKeys: true}
	for name, yml := range map[string]string{
		"invalid-yaml":  "board: [unclosed\n",
		"duplicate-key": "learnings:\n  cap: 1\n  cap: 2\n",
		"invalid-type":  "metadata_branch: [not, a, string]\n",
		"invalid-value": "learnings:\n  cap: -1\n",
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := Resolve([]Source{srcR(yml)}, rctx)
			if !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("err = %v, want ErrInvalidConfig — tolerance must not reach %s", err, name)
			}
		})
	}
}

// TestUnknownKeyStrictWithoutTolerance is the mutation control for the option:
// the reclassifier must be the ONLY thing that flips the verdict, so the zero
// value keeps today's hard failure.
func TestUnknownKeyStrictWithoutTolerance(t *testing.T) {
	_, diags, err := Resolve([]Source{srcR("some_future_block: true\n")}, mainCtx)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("err = %v, want ErrInvalidConfig", err)
	}
	for _, d := range diags {
		if d.Code == CodeUnknownKey && d.Severity != SeverityError {
			t.Errorf("unknown-key severity = %s without tolerance, want error", d.Severity)
		}
		if d.Code == CodeUnknownKey && d.Remedy == ToleratedUnknownKeyRemedy {
			t.Errorf("the tolerated remedy leaked into the strict path")
		}
	}
}

// TestTolerateUnknownKeysLeavesFenceIntact: a fenced KNOWN key keeps its
// existing warn-and-ignore posture with the option on. Reuse the exact
// layer/fixture of TestBoardGithubTokenMachineFence, changing only the rctx.
func TestTolerateUnknownKeysLeavesFenceIntact(t *testing.T) {
	rctx := ResolveContext{DefaultBranch: "main", TolerateUnknownKeys: true}
	res := mustResolve(t, []Source{srcR("board:\n  github_token: tok\n")}, rctx)
	fenced := diagsWithCode(res, CodeFencedIgnored)
	if len(fenced) != 1 || fenced[0].Severity != SeverityWarning {
		t.Fatalf("fenced diagnostics = %v, want one fenced-setting-ignored warning", diagSummary(res))
	}
	if fenced[0].Remedy == ToleratedUnknownKeyRemedy {
		t.Errorf("the fence's own remedy was overwritten")
	}
}

// TestWarningsFilter pins the helper the install path reads its surfaced
// diagnostics through.
func TestWarningsFilter(t *testing.T) {
	in := []Diagnostic{
		{Code: CodeUnknownKey, Severity: SeverityWarning},
		{Code: CodeInvalidValue, Severity: SeverityError},
		{Code: "x", Severity: SeverityInfo},
	}
	out := Warnings(in)
	if len(out) != 1 || out[0].Code != CodeUnknownKey {
		t.Fatalf("Warnings = %v, want only the warning-severity diagnostic", out)
	}
}
```

Note for the fence fixture: before finalizing, read `TestBoardGithubTokenMachineFence` in this same file and mirror its layer choice exactly (if it plants `board.github_token` in a different layer than `srcR`, copy that). The assert — one `CodeFencedIgnored` warning, unchanged by tolerance — stays the same.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/config/ -run 'TolerateUnknownKeys|UnknownKeyStrict|WarningsFilter' -count=1 -v`
Expected: compile FAIL — `TolerateUnknownKeys`, `ToleratedUnknownKeyRemedy`, and `Warnings` undefined.

- [ ] **Step 3: Implement**

In `internal/config/config.go`, extend `ResolveContext` (keep the existing comment on `DefaultBranch`):

```go
// ResolveContext supplies facts resolution cannot derive from the layers.
type ResolveContext struct {
	DefaultBranch string // consumed only when integration_branch resolves to "auto"
	// TolerateUnknownKeys reclassifies every unknown-key ERROR diagnostic as a
	// WARNING so an unrecognized setting cannot invalidate the snapshot. It
	// exists for the install path alone: an installer must never be blocked by
	// a configuration written for a newer docket than the one running.
	// Operating commands never set it.
	TolerateUnknownKeys bool
}
```

In `internal/config/config.go`, near the `Diagnostic` type:

```go
// Warnings filters diagnostics to the warning-severity ones. The install path
// surfaces exactly this subset in its result document (change 0392).
func Warnings(diags []Diagnostic) []Diagnostic {
	var out []Diagnostic
	for _, d := range diags {
		if d.Severity == SeverityWarning {
			out = append(out, d)
		}
	}
	return out
}
```

In `internal/config/resolve.go`, the constant and the reclassifier (one site, applied to collected diagnostics — never inside the decoder, which has no `ResolveContext`):

```go
// ToleratedUnknownKeyRemedy is the remedy every tolerated unknown-key warning
// carries. One constant so tests and presenters share the exact sentence; it
// names both causes because the resolver cannot tell them apart.
const ToleratedUnknownKeyRemedy = "the key may belong to a newer docket than the one running (rebuild or upgrade docket), or it may be a typo (fix or remove it)"

// tolerateUnknownKeys degrades every unknown-key ERROR to a warning carrying
// the shared remedy (change 0392). It keys on severity so the two deliberate
// warn-and-ignore surfaces — already warnings, with their own messages — pass
// through untouched, and it never touches any other code, so the invalid
// classes and the coordination fence keep their posture.
func tolerateUnknownKeys(diags []Diagnostic) {
	for i := range diags {
		if diags[i].Code == CodeUnknownKey && diags[i].Severity == SeverityError {
			diags[i].Severity = SeverityWarning
			diags[i].Remedy = ToleratedUnknownKeyRemedy
		}
	}
}
```

Apply it inside `resolve` in `internal/config/resolve.go`, immediately after the layer parse/decode loop and **before** the first `res.hasInvalid()` (the second `hasInvalid` after `checkAutoCaptureTypes` then needs no second application — the diagnostics are already flipped, and `checkAutoCaptureTypes` emits `invalid-value`, which tolerance must not reach):

```go
	if rctx.TolerateUnknownKeys {
		tolerateUnknownKeys(res.diags)
	}
	if res.hasInvalid() {
		return res.done(), ErrInvalidConfig
	}
```

- [ ] **Step 4: Run the new tests, then the package**

Run: `go test ./internal/config/ -run 'TolerateUnknownKeys|UnknownKeyStrict|WarningsFilter' -count=1 -v`
Expected: PASS
Run: `go test ./internal/config/ -count=1`
Expected: PASS (zero-value behaviour unchanged — any other failure here means the reclassifier leaked)

- [ ] **Step 5: Mutation-test the control**

Temporarily comment out the `tolerateUnknownKeys(res.diags)` call (keep a copy of the exact lines; restore by re-editing, never `git checkout --`), re-run the Step 4 first command. Expected: `TestTolerateUnknownKeysDegradesToWarning` and `TestTolerateUnknownKeysLeavesFenceIntact` FAIL; `TestUnknownKeyStrictWithoutTolerance` still PASSES. Restore the call, re-run, all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/config/config.go internal/config/resolve.go internal/config/resolve_test.go
git commit -m "feat(config): TolerateUnknownKeys reclassifies unknown-key errors to warnings (0392)"
```

---

### Task 2: `app.ResolveRepoPhase` takes the `ResolveContext` and returns warnings

**Files:**
- Modify: `internal/app/repophase.go`
- Modify: every existing caller of `ResolveRepoPhase` in `internal/app/repophase_test.go` (mechanical signature update) and the one production caller `internal/cli/install.go` `resolveRepoPhase` (temporarily passing a strict context; Task 3 flips it)
- Test: `internal/app/repophase_test.go`

**Interfaces:**
- Consumes: Task 1's `config.ResolveContext.TolerateUnknownKeys` and `config.Warnings`.
- Produces: `func ResolveRepoPhase(ctx context.Context, git *gitcli.Client, repoDir string, harnessScope []string, runGate []byte, legacy install.LegacyReproducer, rctx config.ResolveContext) (*install.RepoPhase, string, []config.Diagnostic, error)` — third return is the warning-severity diagnostics from the repository resolve; nil on the machine-only and error paths.

- [ ] **Step 1: Write the failing test**

Append to `internal/app/repophase_test.go` (spec Testing item 6; uses the file's existing `initGitRepo`, `newGitClient` helpers):

```go
// TestResolveRepoPhaseToleratesUnknownKeys (change 0392): with a tolerant
// context, a .docket.yml carrying an unknown key plus an explicit
// agent_harnesses still yields an authorized phase, and the unknown-key
// warning comes back for the install result to surface. The strict control —
// today's ReasonInvalidConfig refusal — pins that the CLI's context, not this
// assembler, owns the decision.
func TestResolveRepoPhaseToleratesUnknownKeys(t *testing.T) {
	root, _ := initGitRepo(t, "agent_harnesses: [claude]\nsome_future_block: true\n")
	git := newGitClient(t)

	tolerant := config.ResolveContext{DefaultBranch: "main", TolerateUnknownKeys: true}
	phase, gotRoot, warnings, err := ResolveRepoPhase(context.Background(), git, root, nil, []byte("gate\n"), nil, tolerant)
	if err != nil {
		t.Fatalf("tolerant ResolveRepoPhase: %v", err)
	}
	if phase == nil || !phase.Authorized || gotRoot != root {
		t.Fatalf("phase = %+v root = %q, want an authorized phase at %q", phase, gotRoot, root)
	}
	found := false
	for _, w := range warnings {
		if w.Code == config.CodeUnknownKey && w.Severity == config.SeverityWarning {
			found = true
		}
	}
	if !found {
		t.Fatalf("warnings = %v, want the tolerated unknown-key warning", warnings)
	}

	strict := config.ResolveContext{DefaultBranch: "main"}
	_, _, _, err = ResolveRepoPhase(context.Background(), git, root, nil, []byte("gate\n"), nil, strict)
	var re *RepoResolutionError
	if !errors.As(err, &re) || re.Reason != ReasonInvalidConfig {
		t.Fatalf("strict err = %v, want RepoResolutionError with %q", err, ReasonInvalidConfig)
	}
}
```

Add `"github.com/danielhanold/docket/internal/config"` to the test file's imports.

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/app/ -run TestResolveRepoPhaseToleratesUnknownKeys -count=1 -v`
Expected: compile FAIL — wrong argument count / return arity.

- [ ] **Step 3: Implement the signature change**

In `internal/app/repophase.go`:

1. Delete the `repoConfigBranch` constant and its comment (the context now arrives from the caller; nothing else reads it — verify with `grep -rn repoConfigBranch internal/`).
2. Change the signature and thread the context and warnings:

```go
// ... existing doc comment, plus:
// rctx is the resolution context the caller owns — the CLI's install path
// passes a tolerant one (change 0392); this assembler carries no
// install-specific knowledge of why. The third return is the warning-severity
// diagnostics from the repository resolve, for the install result to surface;
// it is nil on the machine-only and error paths.
func ResolveRepoPhase(ctx context.Context, git *gitcli.Client, repoDir string, harnessScope []string, runGate []byte, legacy install.LegacyReproducer, rctx config.ResolveContext) (*install.RepoPhase, string, []config.Diagnostic, error) {
```

Inside, replace the resolve call:

```go
	snap, diags, err := config.Resolve(sources, rctx)
	if err != nil {
		return nil, "", nil, &RepoResolutionError{Reason: ReasonInvalidConfig, Err: err}
	}
	warnings := config.Warnings(diags)
```

Then update **every** `return` in the function to the new arity: the machine-only return becomes `return nil, "", nil, nil`; every error return gains a `nil` warnings slot; the unauthorized-phase return and the final authorized return carry `warnings` (`return &install.RepoPhase{…}, root, warnings, nil`).

3. Update the existing test callers in `repophase_test.go`: every current `ResolveRepoPhase(ctx, git, dir, scope, gate, legacy)` call gains a trailing `config.ResolveContext{DefaultBranch: "main"}` and a discarded third return (`phase, gotRoot, _, err := …`). Do not change what those tests assert.
4. Update `internal/cli/install.go` `resolveRepoPhase` minimally so the tree compiles — add the parameter pass-through with today's strict values (Task 3 owns the tolerance flip):

```go
	phase, _, warnings, err := app.ResolveRepoPhase(ctx, git, repoDir, harnesses, runGate, legacy, config.ResolveContext{DefaultBranch: installDefaultBranch})
```

and have `resolveRepoPhase` return `(*install.RepoPhase, []config.Diagnostic, *InstallRefusal)`, its caller discarding the warnings for now.

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/app/ -run TestResolveRepoPhase -count=1 -v && go build ./... && go test ./internal/cli/ -count=1`
Expected: all PASS, whole tree compiles.

- [ ] **Step 5: Commit**

```bash
git add internal/app/repophase.go internal/app/repophase_test.go internal/cli/install.go
git commit -m "feat(app): ResolveRepoPhase takes the caller's ResolveContext and returns warnings (0392)"
```

---

### Task 3: Install path sets the flag once and surfaces the warnings

**Files:**
- Modify: `internal/cli/install.go` (`installOptions` — the one site that sets `TolerateUnknownKeys`)
- Modify: `internal/install/service.go` (`Options.ConfigWarnings`)
- Modify: `internal/app/install.go` (`InstallResult.Warnings`, `withConfigWarnings`, `HumanText` warning lines, wiring in the three `Run*` functions)
- Test: `internal/app/install_test.go`, `internal/cli/root_test.go`

**Interfaces:**
- Consumes: Task 1's `config.Warnings`/`ToleratedUnknownKeyRemedy`; Task 2's `ResolveRepoPhase` signature.
- Produces:
  - `install.Options.ConfigWarnings []config.Diagnostic`
  - `app.InstallResult.Warnings []config.Diagnostic` (JSON tag `warnings,omitempty`)
  - `installResolveContext() config.ResolveContext` (unexported, `internal/cli/install.go`)
  - `withConfigWarnings(r InstallResult, warnings []config.Diagnostic) InstallResult` (unexported, `internal/app/install.go`)

- [ ] **Step 1: Write the failing unit tests (result document)**

Append to `internal/app/install_test.go`:

```go
// TestInstallResultCarriesConfigWarnings (change 0392): warning-severity
// diagnostics from the install-path config reads reach the JSON document and
// the human text, one line each, provenance included when present and omitted
// when absent.
func TestInstallResultCarriesConfigWarnings(t *testing.T) {
	warns := []config.Diagnostic{
		{
			Code: config.CodeUnknownKey, Severity: config.SeverityWarning,
			Path: "some_future_block", Message: "is not a docket configuration setting",
			Remedy:     config.ToleratedUnknownKeyRemedy,
			Provenance: &config.Provenance{Layer: config.LayerRepository, Source: ".docket.yml", Line: 3},
		},
		{
			Code: config.CodeUnknownKey, Severity: config.SeverityWarning,
			Path: "another", Message: "is ignored",
		},
	}
	o := install.Options{ConfigWarnings: warns}
	r := app.RunInstallCheck(o) // any Run* path works; check is the cheapest

	if len(r.Warnings) != 2 {
		t.Fatalf("Warnings = %v, want both diagnostics", r.Warnings)
	}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"warnings"`) {
		t.Fatalf("JSON lacks warnings: %s", data)
	}

	text := r.HumanText()
	if !strings.Contains(text, "warning: .docket.yml:3 some_future_block — is not a docket configuration setting ("+config.ToleratedUnknownKeyRemedy+")") {
		t.Errorf("human text lacks the provenance-bearing warning line:\n%s", text)
	}
	if !strings.Contains(text, "warning: another — is ignored") {
		t.Errorf("human text lacks the provenance-free warning line:\n%s", text)
	}
}

// TestInstallResultNoWarningsOmitsField: an empty warning set marshals to no
// warnings key and prints no warning line.
func TestInstallResultNoWarningsOmitsField(t *testing.T) {
	r := app.RunInstallCheck(install.Options{})
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"warnings"`) {
		t.Fatalf("empty warnings must be omitted: %s", data)
	}
	if strings.Contains(r.HumanText(), "warning:") {
		t.Fatalf("human text has a phantom warning line:\n%s", r.HumanText())
	}
}
```

Adjust the imports and call shapes to the file's local conventions (it is package `app` or `app_test` — mirror the neighbouring tests; if the file is package `app`, drop the `app.` qualifiers). If `RunInstallCheck` on empty options panics before reaching the result (it calls `install.Check`), mirror how `TestInstallResultRepoReporting` in the same file builds its fixture options and reuse that instead — the assertion targets `Warnings`/`HumanText`, not the check outcome.

- [ ] **Step 2: Write the failing CLI wiring tests**

The flag and the warnings plumbing are caller wiring a defaulted path hides (learnings: `defaulted-param-hides-caller-wiring`) — so pin them end-to-end through the real commands. Append to `internal/cli/root_test.go`, next to `TestInstallCheckWithoutInstallation`:

```go
// TestInstallCheckToleratesUnknownGlobalKey (change 0392): the wiring assert
// for the GLOBAL install-path read. A global config written for a newer schema
// must not abort install check — deleting TolerateUnknownKeys from
// installOptions turns this into invalid-input/invalid-config and reddens it.
func TestInstallCheckToleratesUnknownGlobalKey(t *testing.T) {
	home := pinInstallEnv(t)
	cfgDir := filepath.Join(home, ".config", "docket")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yml"), []byte("some_future_block: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, errS, code := runCLI(t, "install", "check", "--json")
	if errS != "" {
		t.Fatalf("stderr = %q", errS)
	}
	if strings.Contains(out, `"reason":"invalid-config"`) {
		t.Fatalf("install check refused a newer-schema global config: %s", out)
	}
	// The unwritten machine still answers installation-required — the read
	// completed and the operation ran.
	if code != 1 || !strings.Contains(out, `"reason":"installation-required"`) {
		t.Fatalf("out=%q code=%d, want the operation to complete to installation-required", out, code)
	}
	if !strings.Contains(out, `"warnings"`) || !strings.Contains(out, `"some_future_block"`) {
		t.Fatalf("the tolerated key's warning is missing from the document: %s", out)
	}
}

// TestInstallToleratesUnknownRepositoryKey: the wiring assert for the
// REPOSITORY install-path read — install over a .docket.yml carrying an
// unknown key completes (applied) and surfaces the warning, exercising the
// same installOptions path development install's parent runs before its
// build/hand-off step (spec Testing items 7 and 8).
func TestInstallToleratesUnknownRepositoryKey(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	pinInstallEnv(t)
	repo := testsupport.TempDir(t)
	for _, args := range [][]string{{"init", "-b", "main"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if outB, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, outB)
		}
	}
	if err := os.WriteFile(filepath.Join(repo, ".docket.yml"), []byte("agent_harnesses: [claude]\nsome_future_block: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, errS, code := runCLI(t, "install", "--repo-dir", repo, "--json")
	if errS != "" {
		t.Fatalf("stderr = %q", errS)
	}
	if strings.Contains(out, `"reason":"invalid-config"`) || code == 2 {
		t.Fatalf("install refused a newer-schema repository config (code %d): %s", code, out)
	}
	if !strings.Contains(out, `"result":"applied"`) {
		t.Fatalf("install did not complete: %s", out)
	}
	if !strings.Contains(out, `"warnings"`) || !strings.Contains(out, `"some_future_block"`) {
		t.Fatalf("the repository read's warning is missing: %s", out)
	}
}
```

Mirror the file's existing helpers where they exist (if `root_test.go` or a sibling already has a git-repo helper, use it instead of raw `exec.Command`). If the full `install` run in the second test fails for a reason unrelated to config (inspect the JSON), keep the two config asserts (`not invalid-config`, `warnings` present) and replace the `applied` assert with the actual completed result — the test's job is that the read passed and the warning surfaced, not to certify the whole transaction.

- [ ] **Step 3: Run both to verify they fail**

Run: `go test ./internal/app/ -run TestInstallResult -count=1 && go test ./internal/cli/ -run 'TestInstallCheckTolerates|TestInstallTolerates' -count=1 -v`
Expected: compile FAIL (`ConfigWarnings`, `Warnings` undefined), then after types exist, the CLI tests FAIL with `invalid-config`.

- [ ] **Step 4: Implement**

`internal/install/service.go`, in `Options`:

```go
	// ConfigWarnings are the warning-severity diagnostics the install-path
	// config reads produced (change 0392). The service ignores them; the app
	// layer surfaces them in the result document so a tolerated unknown key —
	// or any other warning — is never silently dropped.
	ConfigWarnings []config.Diagnostic
```

`internal/app/install.go`:

1. Field on `InstallResult`, after `RepoHarnesses`:

```go
	// Warnings are the warning-severity configuration diagnostics from the
	// install-path reads (change 0392): tolerated unknown keys, fenced
	// settings, and the rest — everything is surfaced, because filtering would
	// only hide information.
	Warnings []config.Diagnostic `json:"warnings,omitempty"`
```

2. The seam, next to `withRepoReporting`:

```go
// withConfigWarnings stamps the install-path config warnings onto the result
// (change 0392), so the document says what the reads degraded rather than
// discarding it.
func withConfigWarnings(r InstallResult, warnings []config.Diagnostic) InstallResult {
	r.Warnings = warnings
	return r
}
```

3. Wire the three operations:

```go
func RunInstall(o install.Options) InstallResult {
	return withConfigWarnings(withRepoReporting(NewInstallResult(OperationInstall, install.Install(o)), o.RepoPhase), o.ConfigWarnings)
}

func RunInstallCheck(o install.Options) InstallResult {
	return withConfigWarnings(NewInstallResult(OperationInstallCheck, install.Check(o)), o.ConfigWarnings)
}

func RunDevelopmentInstall(o install.DevOptions) InstallResult {
	return withConfigWarnings(withRepoReporting(NewInstallResult(OperationDevelopmentInstall, install.DevelopmentInstall(o)), o.RepoPhase), o.ConfigWarnings)
}
```

(`DevOptions` embeds/carries `Options`; spell the field access the way `o.RepoPhase` already resolves in that function.)

4. `HumanText`, after the `RepoHarnesses` block and before the `Reason` block:

```go
	for _, w := range r.Warnings {
		b.WriteString("warning: ")
		if w.Provenance != nil {
			fmt.Fprintf(&b, "%s:%d ", w.Provenance.Source, w.Provenance.Line)
		}
		fmt.Fprintf(&b, "%s — %s", w.Path, w.Message)
		if w.Remedy != "" {
			fmt.Fprintf(&b, " (%s)", w.Remedy)
		}
		b.WriteString("\n")
	}
```

`internal/cli/install.go` — the one site:

```go
// installResolveContext is the resolution context every install-path config
// read uses. TolerateUnknownKeys is set HERE and nowhere else: an install
// operation is never blocked by configuration it does not understand — the
// config may be written for a newer docket than the binary running the
// installer (the schema-bump bootstrap deadlock, change 0392) — while every
// operating command keeps the strict typo policy.
func installResolveContext() config.ResolveContext {
	return config.ResolveContext{DefaultBranch: installDefaultBranch, TolerateUnknownKeys: true}
}
```

In `installOptions`: use it for the global read, keep the warnings, and thread it to the repo read —

```go
	snapshot, diags, err := config.Resolve(sources, installResolveContext())
	if err != nil {
		return install.Options{}, &InstallRefusal{Reason: app.ReasonInvalidConfig, Err: err}
	}
	configWarnings := config.Warnings(diags)
```

set `ConfigWarnings: configWarnings` in the `install.Options` literal; and in the repo branch:

```go
	phase, repoWarnings, refusal := resolveRepoPhase(ctx, opts, harnesses, repoDir)
	if refusal != nil {
		return install.Options{}, refusal
	}
	opts.RepoPhase = phase
	opts.HarnessOptIns = repoOptIns(phase)
	opts.ConfigWarnings = append(opts.ConfigWarnings, repoWarnings...)
```

with `resolveRepoPhase` passing `installResolveContext()` into `app.ResolveRepoPhase` (replacing Task 2's temporary strict context) and returning the warnings it gets back.

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/app/ ./internal/cli/ ./internal/install/ -count=1`
Expected: PASS, including both new CLI wiring tests.

- [ ] **Step 6: Mutation-test the wiring**

Temporarily change `installResolveContext()` to omit `TolerateUnknownKeys: true`. Run: `go test ./internal/cli/ -run 'TestInstallCheckTolerates|TestInstallTolerates' -count=1`. Expected: both FAIL with `invalid-config`. Restore, re-run, PASS. (This is the assert that deleting the one-site flag reddens something — the resolved non-default value arrives end-to-end.)

- [ ] **Step 7: Commit**

```bash
git add internal/cli/install.go internal/install/service.go internal/app/install.go internal/app/install_test.go internal/cli/root_test.go
git commit -m "feat(install): tolerate unknown config keys on the install path and surface the warnings (0392)"
```

---

### Task 4: Strictness regressions — operating commands keep the hard failure

**Files:**
- Test: `internal/app/config_test.go` (or the file holding `DiagnosticConfig` tests — locate with `grep -rn "DiagnosticConfig" internal/app/*_test.go`)

**Interfaces:**
- Consumes: `app.DiagnosticConfig(sources []config.Source, rctx config.ResolveContext, forMutation bool)`; Task 1's semantics.
- Produces: nothing new — regression pins only.

- [ ] **Step 1: Write the regression test**

Spec Testing item 9: the same unknown-key fixture must still hard-fail every operating read. `diagnostic config` is the named human-visible strict verdict; the resolver-level strict default is already pinned by Task 1's `TestUnknownKeyStrictWithoutTolerance`, and `status` shares the same untouched `config.Resolve` call sites (`internal/app/operational_context.go`, `internal/app/config.go` — none sets the flag; confirm with `grep -rn "TolerateUnknownKeys" internal/ | grep -v _test`, which must list only `internal/config/` and `internal/cli/install.go`).

```go
// TestDiagnosticConfigStaysStrictOnUnknownKeys (change 0392): the install
// path's tolerance must not leak into the operating commands' strict verdict —
// diagnostic config over a newer-schema layer still reports invalid
// configuration, so a human can always see the strict reading on demand.
func TestDiagnosticConfigStaysStrictOnUnknownKeys(t *testing.T) {
	sources := []config.Source{{Layer: config.LayerRepository, Name: ".docket.yml", Data: []byte("some_future_block: true\n")}}
	r := DiagnosticConfig(sources, config.ResolveContext{DefaultBranch: "main"}, false)
	if r.Result != ResultInvalidInput && r.Reason == "" {
		t.Fatalf("diagnostic config tolerated an unknown key: %+v", r)
	}
	found := false
	for _, d := range r.Diagnostics {
		if d.Code == config.CodeUnknownKey && d.Severity == config.SeverityError {
			found = true
		}
	}
	if !found {
		t.Fatalf("diagnostics = %v, want an unknown-key ERROR", r.Diagnostics)
	}
}
```

Before finalizing, read how the neighbouring `DiagnosticConfig` tests assert the failure envelope (`Result`/`Reason` values for `ErrInvalidConfig`) and match their exact expected values instead of the loose disjunction above — the assert must pin the real invalid-configuration outcome, not merely "not applied".

- [ ] **Step 2: Run it**

Run: `go test ./internal/app/ -run TestDiagnosticConfigStaysStrict -count=1 -v`
Expected: PASS immediately (it is a regression pin; if it fails, tolerance leaked — stop and fix).

Also run the one-site grep and record its output in the commit message body if it surprised you:
`grep -rn "TolerateUnknownKeys" internal/ | grep -v _test` → only `internal/config/config.go`, `internal/config/resolve.go`, `internal/cli/install.go`.

- [ ] **Step 3: Commit**

```bash
git add internal/app/config_test.go
git commit -m "test(app): pin diagnostic config's strict unknown-key verdict (0392)"
```

---

### Task 5: ADR and the AGENTS.md caveat replacement

**Files:**
- Create: one ADR in the metadata tree's `docs/adrs/` via the `adr.record` operation (never by hand)
- Modify: `AGENTS.md` (repo root of the feature worktree; `CLAUDE.md` is a symlink to it — do not edit the symlink as a file)

**Interfaces:**
- Consumes: the shipped behaviour of Tasks 1–3.
- Produces: the ADR id (add it to this change's `adrs:` list via the change-file mechanics the build workflow owns), and the rewritten rebuild rule.

- [ ] **Step 1: Record the ADR**

Use the docket ADR workflow (the `docket-adr` agent / `adr record` capability operation — resolve the exact argv from `docket capabilities`; never write the file by hand). Decision content, per the spec:

- **Title:** Install-path configuration reads tolerate unknown keys; the strict typo policy binds operating commands only.
- **Context:** the strict `unknown-key` error deadlocked `development install` after every schema-extending merge (change 0374's `build:` block was the trigger; recovery was an out-of-band `go build` and a manual binary swap).
- **Decision:** `config.ResolveContext.TolerateUnknownKeys`, set at exactly one site (`installOptions`), degrades `unknown-key` errors to warnings carrying a shared remedy; all other diagnostic classes and the coordination fence stay fatal on every path; install-path warnings are surfaced in the result document.
- **Consequences:** a schema-extending change no longer needs an out-of-band rebuild; the strict policy is now a property of *operating* the repository, not of parsing; an old binary still refuses to operate on an unknown field, and `docket diagnostic config` still shows the strict verdict.
- **Relations:** relates to ADR-0019 (the fence is unchanged) and ADR-0102 (the `build:` block whose merge exposed the deadlock).

- [ ] **Step 2: Rewrite the AGENTS.md caveat**

In `AGENTS.md`, section "Rebuild the binary after a merge to main": replace the entire second bullet (the one beginning "When the merged change **extends the `.docket.yml` schema**" through "…load-bearing for any schema-extending change.") with this single bullet:

```markdown
- A merged change that **extends the `.docket.yml` schema** no longer blocks this: since change
  0392 the install path tolerates unknown configuration keys (surfaced as warnings), so the tracked
  `development.install` reinstall works directly with the pre-schema binary — no out-of-band
  `go build` recovery is needed.
```

Keep the first bullet untouched. Before committing, check for prose guards over this section: run the repo's test suite grep first — `grep -rn "schema" tests/*.sh | grep -i "agents\|claude"` — and if any test pins the old wording, follow that guard's own remedy rather than editing around it.

- [ ] **Step 3: Verify and commit**

Run: `readlink CLAUDE.md` → `AGENTS.md` (unchanged), and `go build ./...` still green.

```bash
git add AGENTS.md
git commit -m "docs(0392): AGENTS.md rebuild rule — schema-bump deadlock caveat replaced by the tolerance note"
```

(The ADR lands on the metadata branch through the ADR workflow's own commit; do not stage metadata-tree paths into this feature branch.)

---

## Final verification (build gate)

After the last task, the build workflow runs the whole suite via the config-resolved gate command (`go run ./cmd/docket development test` in this repo today) — never only the tests this plan enumerated. Any `SERIAL CONFIRMED OVER BUDGET:` line is authoritative; `BUDGET WATCH:`/`PARALLEL-SENSITIVE:` lines are screening findings (see `tests/README.md`).

Manual, unautomatable check (spec's simulation note): a "schema-older binary" is simulated everywhere above by an unknown key against the current binary; the two are equivalent by construction, since the binary has no other way to know a key is newer than itself. No live schema-bump rehearsal is required at build time.
