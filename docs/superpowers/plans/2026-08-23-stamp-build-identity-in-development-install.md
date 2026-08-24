<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0340 — Stamp build identity into the `development install` binary](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-23-0340-stamp-build-identity-in-development-install.md)**
<!-- docket:backlink:end -->
# Stamp Build Identity into `development install` — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `docket development install` inject build-identity ldflags (Version/Commit/BuildDate) into the binary it compiles, so `docket version` reports a truthful identity instead of `development (commit unknown, built unknown)`.

**Architecture:** All work lands in `internal/install/devmode.go`. A new injectable git-runner seam (`GitRunner`, an argv runner beside the existing `GoRunner`) reads the checkout's git state; a private `buildIdentity` helper turns that state into the exact `-ldflags` value the release packager uses, and `buildBinary` appends `-ldflags <value>` to its existing `go build` argv only when identity fully resolved. On any git failure the build proceeds unstamped — identity is a nicety, never an install gate, and stamping is all-three-or-none.

**Tech Stack:** Go (stdlib only: `os/exec`, `strings`, `time`, `fmt`). No new dependencies.

**Spec:** `docs/superpowers/specs/2026-08-23-stamp-build-identity-in-development-install-design.md` (lives on the `docket` metadata branch; synchronized copy at `.docket/docs/superpowers/specs/…` in the primary checkout).

## Global Constraints

- **Do NOT import or depend on `internal/release`** — change 0317's `internal/release/package.go` is NOT merged to main. Duplicate the three-`-X` format string locally in `devmode.go`. The exact format (confirmed against 0317's branch and the existing injection test `TestInjectedBuildIdentity` in `cmd/docket/main_test.go`): `-X github.com/danielhanold/docket/internal/buildinfo.Version=<v> -X github.com/danielhanold/docket/internal/buildinfo.Commit=<c> -X github.com/danielhanold/docket/internal/buildinfo.BuildDate=<d>`.
- **`internal/buildinfo` is untouched** — no new fields, no edits to that package.
- **Runners take an argv, never a shell string** — no part of a user-typed path is ever handed to a shell (same rule the existing `GoRunner` comment states).
- **All three identity values or none** — a partial stamp (e.g. a Version with `unknown` Commit) is forbidden; any probe failure means a bare, unstamped build that still succeeds.
- **The install never fails because of git** — git unavailable, source not a repo, probe error: all degrade to unstamped.
- **BuildDate = UTC now in RFC3339** (`time.Now().UTC().Format(time.RFC3339)`) — the same rendering 0317's `Inputs.BuildDate()` produces ("UTC RFC3339 timestamp").
- **One dirtiness probe, applied consistently** — dirtiness is read once from the `git describe … --dirty` output's `-dirty` suffix and drives both the Version (already carries it) and the Commit (`<full-sha>-dirty`).
- Go's test cache can serve a green PASS against a tree you just mutated: every mutation probe or manual re-verification run uses `go test -count=1`.
- The final build gate runs the WHOLE suite via whatever `finalize.test_command` resolves to (docket-build handles that); per-task runs below are focused `go test` invocations.

## File Structure

- Modify: `internal/install/devmode.go` — `GitRunner` field on `DevOptions`, `DefaultGitRunner`, `buildinfoPkg` const, `buildIdentity` helper, `buildBinary` gains an `ldflags` parameter, `DevelopmentInstall` wires them.
- Modify: `internal/install/devmode_test.go` — the shared `devOptions` helper gains a default no-git runner.
- Create: `internal/install/devmode_identity_test.go` — all new tests (package `install_test`, same as `devmode_test.go`; it reuses that file's `world`, `newSource`, `goRun`, `devOptions` helpers).
- Modify: `internal/cli/root.go` — wire `GitRunner: install.DefaultGitRunner` in the `development install` command.
- Modify: `internal/cli/root_test.go` — one discriminating CLI wiring test.

---

### Task 1: The `GitRunner` seam — field, nil refusal, production runner

**Files:**
- Modify: `internal/install/devmode.go` (the `DevOptions` struct and `DefaultGoRunner`/`DevelopmentInstall` neighborhood)
- Modify: `internal/install/devmode_test.go` (the `devOptions` helper)
- Create: `internal/install/devmode_identity_test.go`

**Interfaces:**
- Consumes: existing `install.DevOptions`, `install.DevelopmentInstall`, `install.ReasonInvalidOptions`, test helpers `world`/`newWorld`, `newSource`, `goRun`, `devOptions`, `mkdirAll`, `devPlanner` from `devmode_test.go`.
- Produces: `DevOptions.GitRunner func(dir string, argv []string) (string, error)` (nil is refused with `ReasonInvalidOptions`, exactly like a nil `GoRunner`); `install.DefaultGitRunner(dir string, argv []string) (string, error)` returning stdout only. Task 2 injects canned runners through this field; Task 3 wires `DefaultGitRunner` in the CLI.

Design note (why nil is refused rather than "nil means unstamped"): the production wiring in `internal/cli/root.go` is otherwise behaviorally invisible — delete it and every test stays green while every user's install silently loses its stamp, the exact population the feature exists for. A nil refusal makes a lost wiring loud at the first real run, and Task 3 adds the test that pins the wiring.

- [ ] **Step 1: Write the failing tests**

Create `internal/install/devmode_identity_test.go`:

```go
package install_test

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/install"
)

// gitRun cans git output per subcommand (argv[1]) and records every probe the
// identity seam issues, so tests assert both the values and the argv shapes.
type gitRun struct {
	dirs  []string
	argvs [][]string
	out   map[string]string
	errs  map[string]error
}

func (g *gitRun) runner() func(string, []string) (string, error) {
	return func(dir string, argv []string) (string, error) {
		g.dirs = append(g.dirs, dir)
		g.argvs = append(g.argvs, append([]string(nil), argv...))
		key := ""
		if len(argv) > 1 {
			key = argv[1]
		}
		if err := g.errs[key]; err != nil {
			return "", err
		}
		return g.out[key], nil
	}
}

func TestDevInstallRequiresGitRunner(t *testing.T) {
	w := newWorld(t)
	mkdirAll(t, w.path(".toy"))
	src := newSource(t)
	o := w.devOptions(t, src, filepath.Join(w.home, "bin"), &goRun{body: "binary\n"})
	o.GitRunner = nil

	out := install.DevelopmentInstall(o)
	if out.Err == nil || out.Reason != install.ReasonInvalidOptions {
		t.Fatalf("err=%v reason=%q, want a %q refusal", out.Err, out.Reason, install.ReasonInvalidOptions)
	}
}

func TestDefaultGitRunner(t *testing.T) {
	repo := t.TempDir()
	init := [][]string{
		{"git", "init", "-q"},
		{"git", "-c", "user.email=t@t", "-c", "user.name=t", "commit", "--allow-empty", "-q", "-m", "x"},
	}
	for _, argv := range init {
		if _, err := install.DefaultGitRunner(repo, argv); err != nil {
			t.Fatalf("%v: %v", argv, err)
		}
	}
	out, err := install.DefaultGitRunner(repo, []string{"git", "rev-parse", "HEAD"})
	if err != nil {
		t.Fatalf("rev-parse: %v", err)
	}
	// Stdout only, machine-readable: a full SHA and git's own trailing newline.
	if !regexp.MustCompile(`^[0-9a-f]{40}\n$`).MatchString(out) {
		t.Fatalf("rev-parse output = %q, want a bare 40-hex SHA line", out)
	}

	if _, err := install.DefaultGitRunner(t.TempDir(), []string{"git", "rev-parse", "HEAD"}); err == nil {
		t.Fatal("rev-parse outside a repository must error, not succeed")
	}
	if _, err := install.DefaultGitRunner(repo, nil); err == nil {
		t.Fatal("an empty argv must error")
	}
	_ = strings.TrimSpace // keep the import stable for later tasks in this file
}
```

Also modify the `devOptions` helper in `devmode_test.go` so every existing fixture carries an explicit no-git runner (routing existing tests through the unstamped path on purpose — Task 2 covers the stamped path with its own fixtures):

```go
func (w *world) devOptions(t *testing.T, src, bin string, g *goRun) install.DevOptions {
	o := w.options(nil)
	o.Planners = []install.Planner{devPlanner(t, w.path(".toy"))}
	return install.DevOptions{
		Options: o, SourceRoot: src, BinDir: bin, GoRunner: g.runner(),
		// Fixtures are not git checkouts; identity degrades to unstamped.
		GitRunner: func(string, []string) (string, error) {
			return "", errors.New("no git in this fixture")
		},
	}
}
```

(`errors` is already imported in `devmode_test.go`.)

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test -count=1 ./internal/install/ -run 'TestDevInstallRequiresGitRunner|TestDefaultGitRunner'`
Expected: compile FAILURE — `DevOptions` has no field `GitRunner`; `install.DefaultGitRunner` undefined.

- [ ] **Step 3: Write the minimal implementation**

In `internal/install/devmode.go`:

Add to `DevOptions` (after the `GoRunner` field, mirroring its comment style):

```go
	// GitRunner runs a git argument vector in a directory and returns its
	// stdout. Like GoRunner it is a vector, never a shell string. It only
	// feeds build identity, which is a nicety: probe failures degrade the
	// build to unstamped, but a missing runner is a wiring bug and refused.
	GitRunner func(dir string, argv []string) (string, error)
```

Add below `DefaultGoRunner`:

```go
// DefaultGitRunner is the production git seam. It returns stdout alone:
// git writes progress and advice to stderr, and captured stderr must never
// leak into ldflags values.
func DefaultGitRunner(dir string, argv []string) (string, error) {
	if len(argv) == 0 {
		return "", fmt.Errorf("%w: empty command", ErrInvalidInput)
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s: %w", argv[0], err)
	}
	return string(out), nil
}
```

In `DevelopmentInstall`, directly after the existing `o.GoRunner == nil` refusal:

```go
	if o.GitRunner == nil {
		return fail(out, ReasonInvalidOptions, fmt.Errorf("%w: no git runner", ErrInvalidInput))
	}
```

- [ ] **Step 4: Run the package tests to verify green**

Run: `go test -count=1 ./internal/install/`
Expected: PASS (the two new tests plus every existing devmode test, now running through the explicit no-git fixture runner).

- [ ] **Step 5: Commit**

```bash
git add internal/install/devmode.go internal/install/devmode_test.go internal/install/devmode_identity_test.go
git commit -m "feat(0340): add injectable GitRunner seam to development install"
```

---

### Task 2: Compute identity and append `-ldflags` in `buildBinary`

**Files:**
- Modify: `internal/install/devmode.go` (`buildBinary` and a new `buildIdentity` helper + `buildinfoPkg` const)
- Modify: `internal/install/devmode_identity_test.go` (add the stamped/unstamped tests)

**Interfaces:**
- Consumes: `DevOptions.GitRunner` and the `gitRun` test double from Task 1; existing `goRun` double (its `outputPath` scans argv for `-o`, so extra leading flags are fine).
- Produces: private `buildIdentity(run func(string, []string) (string, error), source string, now func() time.Time) string` returning the full `-ldflags` value or `""`; `buildBinary(run func(string, []string) error, source, ldflags string) ([]byte, error)` where `ldflags == ""` means a bare build. Task 3 changes nothing about these — it only wires the CLI.

- [ ] **Step 1: Write the failing tests**

Append to `internal/install/devmode_identity_test.go`:

```go
const (
	testSHA     = "84a10275ffe1aa1242e33386da5be2bd52806b2b"
	buildinfoPk = "github.com/danielhanold/docket/internal/buildinfo"
)

func TestDevInstallStampsIdentity(t *testing.T) {
	cases := []struct {
		name, describe string
		wantVersion    string
		wantCommit     string
	}{
		{"tagged clean", "v0.3.0\n", "v0.3.0", testSHA},
		{"past a tag, clean", "v0.3.0-12-g84a1027\n", "v0.3.0-12-g84a1027", testSHA},
		{"no tags, dirty", "84a1027-dirty\n", "84a1027-dirty", testSHA + "-dirty"},
		{"tagged, dirty", "v0.3.0-dirty\n", "v0.3.0-dirty", testSHA + "-dirty"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t)
			mkdirAll(t, w.path(".toy"))
			src := newSource(t)
			g := &goRun{body: "binary\n"}
			git := &gitRun{out: map[string]string{
				"describe":  tc.describe,
				"rev-parse": testSHA + "\n",
			}}
			o := w.devOptions(t, src, filepath.Join(w.home, "bin"), g)
			o.GitRunner = git.runner()

			out := install.DevelopmentInstall(o)
			if out.Err != nil {
				t.Fatalf("DevelopmentInstall: %v (reason %q)", out.Err, out.Reason)
			}

			// Both probes ran in the canonical source root, argv-shaped.
			wantProbes := [][]string{
				{"git", "describe", "--tags", "--always", "--dirty"},
				{"git", "rev-parse", "HEAD"},
			}
			if len(git.argvs) != len(wantProbes) {
				t.Fatalf("git probes = %q, want %q", git.argvs, wantProbes)
			}
			for i, want := range wantProbes {
				if strings.Join(git.argvs[i], " ") != strings.Join(want, " ") {
					t.Errorf("probe %d = %q, want %q", i, git.argvs[i], want)
				}
				if git.dirs[i] != src {
					t.Errorf("probe %d ran in %s, want %s", i, git.dirs[i], src)
				}
			}

			// The build argv gained exactly one flag pair before -o.
			if len(g.argv) != 7 || g.argv[0] != "go" || g.argv[1] != "build" ||
				g.argv[2] != "-ldflags" || g.argv[4] != "-o" || g.argv[6] != "./cmd/docket" {
				t.Fatalf("argv = %q", g.argv)
			}
			wantPrefix := "-X " + buildinfoPk + ".Version=" + tc.wantVersion +
				" -X " + buildinfoPk + ".Commit=" + tc.wantCommit +
				" -X " + buildinfoPk + ".BuildDate="
			if !strings.HasPrefix(g.argv[3], wantPrefix) {
				t.Fatalf("ldflags = %q, want prefix %q", g.argv[3], wantPrefix)
			}
			stamped := strings.TrimPrefix(g.argv[3], wantPrefix)
			if _, err := time.Parse(time.RFC3339, stamped); err != nil || !strings.HasSuffix(stamped, "Z") {
				t.Fatalf("BuildDate = %q, want UTC RFC3339 (parse err %v)", stamped, err)
			}
		})
	}
}

func TestDevInstallUnstampedOnGitFailure(t *testing.T) {
	probeErr := errors.New("git failed")
	cases := []struct {
		name string
		git  *gitRun
	}{
		{"describe errors", &gitRun{errs: map[string]error{"describe": probeErr},
			out: map[string]string{"rev-parse": testSHA + "\n"}}},
		{"rev-parse errors", &gitRun{errs: map[string]error{"rev-parse": probeErr},
			out: map[string]string{"describe": "v0.3.0\n"}}},
		{"empty describe output", &gitRun{out: map[string]string{
			"describe": "\n", "rev-parse": testSHA + "\n"}}},
		{"garbage with embedded space", &gitRun{out: map[string]string{
			"describe": "fatal: not a git repository\n", "rev-parse": testSHA + "\n"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t)
			mkdirAll(t, w.path(".toy"))
			src := newSource(t)
			g := &goRun{body: "binary\n"}
			o := w.devOptions(t, src, filepath.Join(w.home, "bin"), g)
			o.GitRunner = tc.git.runner()

			out := install.DevelopmentInstall(o)
			if out.Err != nil {
				t.Fatalf("a git failure must never fail the install: %v (reason %q)", out.Err, out.Reason)
			}
			// All three or none: the bare five-element build, no -ldflags at all.
			if len(g.argv) != 5 {
				t.Fatalf("argv = %q, want the bare 5-element build", g.argv)
			}
			for _, a := range g.argv {
				if strings.Contains(a, "-ldflags") || strings.Contains(a, "buildinfo") {
					t.Fatalf("argv = %q carries a partial stamp", g.argv)
				}
			}
		})
	}
}
```

Add `"time"` and `"errors"` to this file's imports (`errors` becomes genuinely used here; drop the `_ = strings.TrimSpace` keep-alive line from Task 1 if the compiler flags anything — `strings` is used above either way).

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test -count=1 ./internal/install/ -run 'TestDevInstallStampsIdentity|TestDevInstallUnstampedOnGitFailure'`
Expected: FAIL — `argv = ["go" "build" "-o" … "./cmd/docket"]` (len 5, no `-ldflags`) in every stamped case; the failure cases may already pass, which is fine at this step.

- [ ] **Step 3: Write the implementation**

In `internal/install/devmode.go`:

Add the const (place it near the top consts, with the duplication called out):

```go
	// buildinfoPkg is the package whose exported identity vars the stamp
	// targets. The three-`-X` format deliberately duplicates the release
	// packager's ("reusing the release packager's exact `-X` triple format"
	// per the 0340 spec): change 0317's internal/release is not merged, and
	// its branch must not become a dependency of this path.
	buildinfoPkg = "github.com/danielhanold/docket/internal/buildinfo"
```

Add the helper beside `buildBinary`:

```go
// buildIdentity renders the -ldflags value stamping this build's identity,
// or "" when the checkout's git state cannot be read. Identity is a nicety,
// never a gate: every failure path degrades to an unstamped build, and the
// stamp is all-three-or-none — a Version beside an unknown Commit would be a
// new, misleading shape. Dirtiness is probed once, via describe's --dirty
// suffix, and applied to both Version and Commit.
func buildIdentity(run func(string, []string) (string, error), source string, now func() time.Time) string {
	describe, err := run(source, []string{"git", "describe", "--tags", "--always", "--dirty"})
	if err != nil {
		return ""
	}
	head, err := run(source, []string{"git", "rev-parse", "HEAD"})
	if err != nil {
		return ""
	}
	version := strings.TrimSpace(describe)
	commit := strings.TrimSpace(head)
	if strings.HasSuffix(version, "-dirty") {
		commit += "-dirty"
	}
	for _, v := range []string{version, commit} {
		// A value with whitespace would silently corrupt the space-separated
		// -X list; treat it as a failed probe, not a stampable identity.
		if v == "" || strings.ContainsAny(v, " \t\n") {
			return ""
		}
	}
	return fmt.Sprintf("-X %s.Version=%s -X %s.Commit=%s -X %s.BuildDate=%s",
		buildinfoPkg, version, buildinfoPkg, commit, buildinfoPkg,
		now().UTC().Format(time.RFC3339))
}
```

Change `buildBinary` to take the ldflags value (empty = bare build), keeping its staging behavior byte-identical:

```go
func buildBinary(run func(string, []string) error, source, ldflags string) ([]byte, error) {
	staging, err := os.MkdirTemp("", "docket-build-")
	if err != nil {
		return nil, fmt.Errorf("%w: staging the build: %s", ErrBuildFailed, err)
	}
	defer os.RemoveAll(staging)

	staged := filepath.Join(staging, binaryName)
	argv := []string{"go", "build"}
	if ldflags != "" {
		argv = append(argv, "-ldflags", ldflags)
	}
	argv = append(argv, "-o", staged, "./cmd/docket")
	if err := run(source, argv); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrBuildFailed, err)
	}
	body, err := os.ReadFile(staged)
	if err != nil {
		return nil, fmt.Errorf("%w: the build reported success but produced no binary: %s", ErrBuildFailed, err)
	}
	return body, nil
}
```

Update the single call site in `DevelopmentInstall`:

```go
	binary, err := buildBinary(o.GoRunner, source, buildIdentity(o.GitRunner, source, time.Now))
```

Add `"strings"` and `"time"` to `devmode.go`'s imports.

- [ ] **Step 4: Run the package tests to verify green**

Run: `go test -count=1 ./internal/install/`
Expected: PASS — including the untouched `TestDevInstallBuildsViaArgv`, which still sees the bare 5-element argv because the fixture's default `GitRunner` errors (its no-shell-syntax and staging-path asserts must stay green unmodified; if that test reddened, the fixture default from Task 1 is wrong — fix the fixture, never that test).

- [ ] **Step 5: Sanity mutation probe**

Break the all-or-none rule on purpose: in `buildIdentity`, temporarily return the formatted string even when `head` errors (e.g. set `commit := "unknown"` on that path instead of returning `""`). Run `go test -count=1 ./internal/install/ -run TestDevInstallUnstampedOnGitFailure` — the `rev-parse errors` case must go RED. Revert the mutation (revert by editing back — never `git checkout -- internal/install/devmode.go`, which would discard the whole uncommitted implementation).

- [ ] **Step 6: Commit**

```bash
git add internal/install/devmode.go internal/install/devmode_identity_test.go
git commit -m "feat(0340): stamp git-derived build identity into the development-install binary"
```

---

### Task 3: Wire `DefaultGitRunner` in the CLI

**Files:**
- Modify: `internal/cli/root.go` (the `developmentInstallCmd` `RunE`, at the `install.DevOptions{…}` literal)
- Modify: `internal/cli/root_test.go` (one new test beside `TestDevelopmentInstallRequiresSource`)

**Interfaces:**
- Consumes: `install.DefaultGitRunner` (Task 1); the `pinInstallEnv`/`runCLI` helpers already in `root_test.go`.
- Produces: production wiring; nothing downstream.

- [ ] **Step 1: Write the failing test**

The test discriminates the wiring: `DevelopmentInstall` refuses a nil `GitRunner` (reason `invalid-options`) BEFORE it validates the source root (reason `invalid-source-root`). A run with a bogus `--source` therefore reports `invalid-source-root` only if both runners were wired — delete the `GitRunner:` line and this test reddens with `invalid-options`.

Add to `internal/cli/root_test.go` beside `TestDevelopmentInstallRequiresSource`:

```go
func TestDevelopmentInstallWiresBothRunners(t *testing.T) {
	pinInstallEnv(t)
	bogus := filepath.Join(t.TempDir(), "not-a-checkout")
	out, errS, code := runCLI(t, "development", "install", "--source", bogus, "--json")
	if code != 1 || errS != "" {
		t.Fatalf("out=%q err=%q code=%d", out, errS, code)
	}
	// invalid-source-root proves execution got PAST the runner nil-checks:
	// an unwired GoRunner or GitRunner would have refused as invalid-options.
	if !strings.Contains(out, `"reason":"invalid-source-root"`) {
		t.Fatalf("stdout = %q, want an invalid-source-root refusal (a runner is unwired if this reads invalid-options)", out)
	}
}
```

(`filepath` and `strings` are already imported in `root_test.go`; add `filepath` if not.)

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test -count=1 ./internal/cli/ -run TestDevelopmentInstallWiresBothRunners`
Expected: FAIL — the JSON reads `"reason":"invalid-options"` ("no git runner"), because root.go does not wire `GitRunner` yet.

- [ ] **Step 3: Wire the runner**

In `internal/cli/root.go`, in the `developmentInstallCmd` `RunE`, extend the options literal:

```go
			result = app.RunDevelopmentInstall(install.DevOptions{
				Options:    opts,
				SourceRoot: source,
				BinDir:     binDir,
				GoRunner:   install.DefaultGoRunner,
				GitRunner:  install.DefaultGitRunner,
			})
```

- [ ] **Step 4: Run the tests to verify green**

Run: `go test -count=1 ./internal/cli/ ./internal/install/ ./internal/app/ ./cmd/docket/`
Expected: PASS. `cmd/docket`'s existing `TestInjectedBuildIdentity` is the end-to-end proof that the `-X` triple this change assembles actually lands in `docket version` output — no new binary-level test is required (spec: "The existing `cmd/docket` ldflags-injection test already proves the seam end-to-end").

- [ ] **Step 5: Commit**

```bash
git add internal/cli/root.go internal/cli/root_test.go
git commit -m "feat(0340): wire DefaultGitRunner into the development install command"
```

---

## Self-Review (performed while writing)

- **Spec coverage:** Identity values incl. `--always` no-tag fallback → Task 2 table ("no tags, dirty" case uses a bare short SHA); one dirtiness probe applied to both values → `buildIdentity` reads only describe's suffix, pinned by the dirty table rows; BuildDate UTC/release format → RFC3339 assert; fallback never fails the install, all-three-or-none → `TestDevInstallUnstampedOnGitFailure` + Step 5 mutation probe; runner-seam-beside-GoRunner, argv-never-shell → Task 1; ldflags appended only when resolved → `buildBinary`'s empty-string branch; `internal/buildinfo` and 0317 untouched → Global Constraints, no task touches them; unit tests via injected runner, no new binary-level test → Tasks 1–3 as specified.
- **Type consistency:** `GitRunner func(dir string, argv []string) (string, error)` is identical in the struct field, `DefaultGitRunner`, `buildIdentity`'s parameter, and the `gitRun.runner()` double. `buildBinary`'s new third parameter is threaded at its only call site.
- **Known intentional asymmetry:** `DefaultGitRunner` uses `cmd.Output()` (stdout only) where `DefaultGoRunner` uses `CombinedOutput()` — deliberate and commented: git's stderr chatter must never become an ldflags value; the go build's combined output is diagnostics, not a value.
