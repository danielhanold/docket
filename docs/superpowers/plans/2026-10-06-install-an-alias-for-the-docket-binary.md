<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0534 — Install an alias for the docket binary](../../changes/active/0534-install-an-alias-for-the-docket-binary.md)**
<!-- docket:backlink:end -->

# Install an alias for the docket binary — Implementation Plan

> **For agentic workers:** this plan is executed by `docket-build`, which routes each task to the build tier named on it (economy / standard / premium) under the `docket-build-task` contract and runs one full-suite gate at the end. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Every install also places `dckt`, a symlink to the installed `docket`, in the same bin directory. Neither installer ever overwrites a `dckt` it does not own, and `docket install check` reports a missing or foreign alias as a warning, never as a failure.

**Architecture:** The downloader adds a three-way alias step after its `mv -f` and records the alias. The dev installer adds the alias as a journaled `KindSymlink` target, pre-inspected with `InspectTarget` so a foreign alias is reported instead of refusing the install. `install.Check` finds every installed binary (dev `roleBinary` records plus the downloader record's `path=`) and classifies the `dckt` beside each one. `AliasFinding`s ride a new `Outcome` field that the app layer renders as warnings, with result class and exit code unchanged.

**Tech Stack:** Go (`internal/install`, `internal/app`), POSIX `/bin/sh` (downloader), bash test scripts under `tests/`, `scripts/release-smoke.sh` (CI-only).

**Spec:** `docs/superpowers/specs/2026-10-06-install-an-alias-for-the-docket-binary-design.md` (committed in the feature worktree, and on the `docket` metadata branch).

## Global Constraints

- The alias name is exactly `dckt`, a symlink to the installed `docket` in the **same** bin directory.
- Both install paths create it: the release downloader and `docket development install` (the path behind the repository-root `install.sh`).
- Three-way rule, both installers: **absent** → create; **already a symlink resolving to `<bin>/docket`** → leave it (converge); **anything else** → leave it untouched, warn with the path and remedy, and finish the install successfully. The binary install is never failed by the alias. There is no force path.
- Downloader: the link target is **relative** (`docket`), so moving the bin directory keeps it valid. Development installer: the `Target.validate` contract requires an absolute `LinkTarget`, so its link is absolute. Each installer must accept the other's spelling as "ours" because both compare canonical identity, not spelling (learning `canonicalise-every-symlink-hop`).
- The downloader's ownership record gains the alias path (`alias=<bin>/dckt`), written only when the alias is in place and ours.
- `docket install check` reports a missing or foreign `dckt` as a **finding with its remedy, not a refusal**: a warning in the result document, with the result class and exit code unchanged. This is load-bearing: the downloader ends with `exec "$dest" install check`, so a non-zero check would fail every release install that meets a foreign `dckt` (learning `exit-code-encodes-a-non-failure`).
- `docket uninstall` leaves the alias in place, exactly as it leaves the binary.
- Downloader runtime constraint is unchanged except for admitting `ln -s`. `[ -L ]` and `[ -ef ]` are `/bin/sh` `test` builtins (POSIX.1-2024 `test -ef`; supported by dash, busybox ash, and macOS `/bin/sh`). No `readlink`, no `realpath`, no new interpreter. The spelling ban `bash|python|perl|shasum|jq|eval` must stay green.
- Leave the downloader line `mv -f "$stage" "$dest" || die "cannot move the staged binary into $dest"` byte-identical. `scripts/release-smoke.sh` Block H doctors exactly that line and asserts that exactly one line changed.
- Documentation is sparse: installer output, the downloader's usage text, the root `install.sh` header, and `scripts/release-smoke.md`. No `docs/` pages.
- Out of scope: renaming the binary or the capability catalog's `docket` spelling; shell aliases, completions, and package formulas; any user-level surface that invokes `dckt` (the private-instructions change, #532).
- Shell rules (AGENTS.md): `mv -f` on install paths; templated `mktemp`; never pipe a producer into `grep -q`/`head` under pipefail; lead a `--`-starting grep pattern with `-e`/`--`.
- Comments in maintained source anchor on symbol names or quoted clauses, never line numbers (`TestCommentAnchorStyle`).
- Full suite (the build gate): `go run ./cmd/docket development test`. Read the budget report even on green: the three `tests/test_release_downloader*.sh` rows carry a 10 s parallel budget in `tests/runtime-budgets.tsv`.

## Review Focus

1. **Cross-installer spelling:** the dev installer's absolute link meets the downloader, and the downloader's relative link meets the dev installer. Each must converge silently. Pinned in Task 5 G4 and Task 2.
2. **Bin dir behind a symlinked parent (`/tmp` → `/private/tmp`):** still ours. Pinned in Task 1 with a fixture symlinked directory.
3. **Dangling `dckt`, or a directory named `dckt`:** foreign, never removed, install succeeds with a warning. Pinned in Task 1 and Task 5 G5.
4. **`install check` with a foreign or missing alias:** exit 0 and same result class, or the downloader's closing `exec … install check` fails the install. Pinned in Task 3 and Task 4.
5. **Probe error on the alias path is not "missing":** check fails `filesystem-failed` (learning `probe-error-is-not-clean-absence`). Pinned in Task 1.

---

### Task 1: Alias classification primitive and state rule

**Build tier:** standard

**Files:**
- Create: `internal/install/alias.go`
- Create: `internal/install/alias_test.go` (`package install`, internal, so it can reach unexported helpers)
- Modify: `internal/install/state.go` (the `target.Role == roleBinary && target.Harness != ""` check inside state validation)
- Test: `internal/install/state_test.go`

**Interfaces:**
- Consumes: `canonicalPath(p string) (string, error)` and `linkDestination(path string) (string, error)` from `internal/install/inspect.go` (both canonicalise every symlink hop).
- Produces (later tasks rely on these exact names):
  ```go
  const AliasName = "dckt"
  const roleBinaryAlias = "binary-alias"
  const (
      AliasMissing = "missing"
      AliasForeign = "foreign"
  )
  type AliasFinding struct {
      Kind   string // AliasMissing | AliasForeign
      Path   string // <bin>/dckt
      Binary string // the docket binary the alias should resolve to
      Remedy string
  }
  func AliasPathFor(binary string) string                          // filepath.Join(filepath.Dir(binary), AliasName)
  func InspectBinaryAlias(binary string) (*AliasFinding, error)    // nil finding == healthy alias
  ```

- [ ] **Step 1: Write the failing tests**

`internal/install/alias_test.go`:

```go
package install

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// aliasBin makes a canonical bin dir holding a docket binary and returns both.
func aliasBin(t *testing.T) (string, string) {
	t.Helper()
	dir, err := filepath.EvalSymlinks(testsupport.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(bin, "docket")
	if err := os.WriteFile(binary, []byte("binary\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin, binary
}

func TestInspectBinaryAlias(t *testing.T) {
	t.Run("absent is missing", func(t *testing.T) {
		_, binary := aliasBin(t)
		f, err := InspectBinaryAlias(binary)
		if err != nil || f == nil || f.Kind != AliasMissing || f.Path != AliasPathFor(binary) || f.Remedy == "" {
			t.Fatalf("finding = %+v, err %v; want missing with a remedy", f, err)
		}
	})
	t.Run("relative link (downloader spelling) is healthy", func(t *testing.T) {
		bin, binary := aliasBin(t)
		if err := os.Symlink("docket", filepath.Join(bin, AliasName)); err != nil {
			t.Fatal(err)
		}
		if f, err := InspectBinaryAlias(binary); err != nil || f != nil {
			t.Fatalf("finding = %+v, err %v; want healthy", f, err)
		}
	})
	t.Run("absolute link (development spelling) is healthy", func(t *testing.T) {
		bin, binary := aliasBin(t)
		if err := os.Symlink(binary, filepath.Join(bin, AliasName)); err != nil {
			t.Fatal(err)
		}
		if f, err := InspectBinaryAlias(binary); err != nil || f != nil {
			t.Fatalf("finding = %+v, err %v; want healthy", f, err)
		}
	})
	t.Run("alias reached through a symlinked directory is healthy", func(t *testing.T) {
		bin, binary := aliasBin(t)
		via := filepath.Join(filepath.Dir(bin), "via")
		if err := os.Symlink(bin, via); err != nil {
			t.Fatal(err)
		}
		// The link spells the binary through the symlinked directory; the
		// identity check must canonicalise every hop and still call it ours.
		if err := os.Symlink(filepath.Join(via, "docket"), filepath.Join(bin, AliasName)); err != nil {
			t.Fatal(err)
		}
		if f, err := InspectBinaryAlias(filepath.Join(via, "docket")); err != nil || f != nil {
			t.Fatalf("finding = %+v, err %v; want healthy", f, err)
		}
	})
	t.Run("link elsewhere is foreign", func(t *testing.T) {
		bin, binary := aliasBin(t)
		other := filepath.Join(bin, "other")
		if err := os.WriteFile(other, []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(other, filepath.Join(bin, AliasName)); err != nil {
			t.Fatal(err)
		}
		f, err := InspectBinaryAlias(binary)
		if err != nil || f == nil || f.Kind != AliasForeign {
			t.Fatalf("finding = %+v, err %v; want foreign", f, err)
		}
	})
	t.Run("dangling link is foreign", func(t *testing.T) {
		bin, binary := aliasBin(t)
		if err := os.Symlink(filepath.Join(bin, "gone"), filepath.Join(bin, AliasName)); err != nil {
			t.Fatal(err)
		}
		if f, err := InspectBinaryAlias(binary); err != nil || f == nil || f.Kind != AliasForeign {
			t.Fatalf("finding = %+v, err %v; want foreign", f, err)
		}
	})
	t.Run("regular file is foreign", func(t *testing.T) {
		bin, binary := aliasBin(t)
		if err := os.WriteFile(filepath.Join(bin, AliasName), []byte("mine\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		if f, err := InspectBinaryAlias(binary); err != nil || f == nil || f.Kind != AliasForeign {
			t.Fatalf("finding = %+v, err %v; want foreign", f, err)
		}
	})
	t.Run("directory is foreign", func(t *testing.T) {
		bin, binary := aliasBin(t)
		if err := os.Mkdir(filepath.Join(bin, AliasName), 0o755); err != nil {
			t.Fatal(err)
		}
		if f, err := InspectBinaryAlias(binary); err != nil || f == nil || f.Kind != AliasForeign {
			t.Fatalf("finding = %+v, err %v; want foreign", f, err)
		}
	})
	t.Run("probe error is an error, not missing", func(t *testing.T) {
		if runtime.GOOS == "windows" || os.Geteuid() == 0 {
			t.Skip("permission bits do not deny root")
		}
		bin, binary := aliasBin(t)
		if err := os.Chmod(bin, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(bin, 0o755) })
		f, err := InspectBinaryAlias(binary)
		if err == nil {
			t.Fatalf("finding = %+v with nil error; an unreadable bin dir must not read as a clean absence", f)
		}
	})
}
```

Add to `internal/install/state_test.go` (match the file's existing validation-test style; the case below shows the essential assertion):

```go
func TestStateRejectsHarnessAttributedAlias(t *testing.T) {
	s := sampleState()
	unattributed := TargetRecord{Path: "/home/u/.local/bin/dckt", Kind: KindSymlink,
		LinkTarget: "/home/u/.local/bin/docket", Role: roleBinaryAlias}
	s.Targets = append(s.Targets, unattributed)
	if err := ValidateState(s); err != nil {
		t.Fatalf("an unattributed alias record was rejected: %v", err)
	}
	s = sampleState()
	attributed := unattributed
	attributed.Harness = s.Harnesses[0]
	s.Targets = append(s.Targets, attributed)
	if err := ValidateState(s); err == nil {
		t.Fatal("a harness-attributed alias record validated")
	}
}
```

(`sampleState` and `ValidateState` are the existing helpers in `state_test.go` and `state.go`. If `ValidateState` also enforces canonical target order, insert the record in sorted position rather than appending.)

- [ ] **Step 2: Run the tests and confirm they fail**

Run: `go test -count=1 ./internal/install/ -run 'TestInspectBinaryAlias|TestStateRejectsHarnessAttributedAlias'`
Expected: FAIL (compile errors: `InspectBinaryAlias`, `AliasName`, `roleBinaryAlias` undefined).

- [ ] **Step 3: Implement `internal/install/alias.go` (classification part)**

```go
package install

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Every install also places a second, shorter name for the binary beside it:
// AliasName, a symlink to the installed docket. It follows the binary — same
// directory, same ownership posture, left in place by uninstall — and a dckt
// docket did not create is never overwritten. A missing or foreign alias is a
// finding, never a failure: the binary is installed either way.
const (
	AliasName = "dckt"
	// roleBinaryAlias marks the development install's alias in the installed
	// state. Like roleBinary it belongs to the installation, never a harness.
	roleBinaryAlias = "binary-alias"
)

// The alias finding kinds.
const (
	AliasMissing = "missing"
	AliasForeign = "foreign"
)

// AliasFinding is one alias that is not what an install would leave: absent,
// or occupied by something that does not resolve to the binary.
type AliasFinding struct {
	Kind   string
	Path   string
	Binary string
	Remedy string
}

const (
	remedyAliasMissing = "re-run the installer that placed docket here (the release install.sh, or docket development install) to create it"
	remedyAliasForeign = "docket did not create what is at this path and will not replace it; move or delete it, then re-run the installer to get the dckt alias"
)

// AliasPathFor is where the alias for binary lives: beside it.
func AliasPathFor(binary string) string {
	return filepath.Join(filepath.Dir(binary), AliasName)
}

// InspectBinaryAlias classifies the alias beside binary. A nil finding means a
// symlink that resolves — every hop canonicalised — to binary. Absence is
// AliasMissing; anything else at the path (a link elsewhere, a dangling link,
// a file, a directory) is AliasForeign. It only reads. An error means the
// probe itself failed, which is never reported as a clean absence.
func InspectBinaryAlias(binary string) (*AliasFinding, error) {
	path := AliasPathFor(binary)
	info, err := os.Lstat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return &AliasFinding{Kind: AliasMissing, Path: path, Binary: binary, Remedy: remedyAliasMissing}, nil
	case err != nil:
		return nil, fmt.Errorf("install: inspecting %s: %w", path, err)
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		have, err := linkDestination(path)
		if err != nil {
			return nil, err
		}
		want, err := canonicalPath(binary)
		if err != nil {
			return nil, err
		}
		if have == want {
			if _, err := os.Stat(path); err == nil {
				return nil, nil
			}
		}
	}
	return &AliasFinding{Kind: AliasForeign, Path: path, Binary: binary, Remedy: remedyAliasForeign}, nil
}
```

Note: `canonicalPath` resolves a missing final component as far as it exists, so a dangling `dckt -> docket` with `docket` gone would compare equal. The extra `os.Stat(path)` makes healthy mean "resolves to an existing binary". Pin that guard with this sub-test:

```go
	t.Run("link to a vanished binary is foreign", func(t *testing.T) {
		bin, binary := aliasBin(t)
		if err := os.Symlink("docket", filepath.Join(bin, AliasName)); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(binary); err != nil {
			t.Fatal(err)
		}
		if f, err := InspectBinaryAlias(binary); err != nil || f == nil || f.Kind != AliasForeign {
			t.Fatalf("finding = %+v, err %v; want foreign (the alias resolves to nothing)", f, err)
		}
	})
```

Mutation for this guard: delete the `os.Stat(path)` condition so a canonical match returns `nil, nil` directly. This sub-test must go red.

- [ ] **Step 4: Extend the state rule in `internal/install/state.go`**

Replace the binary-only attribution check with:

```go
		if (target.Role == roleBinary || target.Role == roleBinaryAlias) && target.Harness != "" {
			return fmt.Errorf("installation target %q (role %s) is attributed to harness %q", target.Path, target.Role, target.Harness)
		}
```

Before changing the error text, grep for tests that pin the old message: `grep -rn "is attributed to harness" internal/`. If one pins `binary target %q`, keep the old wording for `roleBinary` and add a separate branch for the alias rather than rewording a pinned message.

- [ ] **Step 5: Run the tests and confirm they pass**

Run: `go test -count=1 ./internal/install/ -run 'TestInspectBinaryAlias|TestStateRejectsHarnessAttributedAlias|TestState'`
Expected: PASS.

- [ ] **Step 6: Mutation-check the canonical comparison**

Temporarily compare a raw `os.Readlink(path)` string to `binary` instead of `have == want`. The relative-link and symlinked-directory sub-tests must go red. Restore from a backup copy (`cp` first; never `git checkout --`, learning `mutation-restore-needs-a-backup-copy`), and use `-count=1` (learning `cached-runner-serves-a-mutated-tree`).

- [ ] **Step 7: Commit**

```bash
git add internal/install/alias.go internal/install/alias_test.go internal/install/state.go internal/install/state_test.go
git commit -m "feat(install): classify the dckt alias beside an installed binary"
```

---

### Task 2: Development install places the alias inside its transaction

**Build tier:** premium (it writes into the user's bin directory, and the foreign-alias preservation and rollback guarantees are what keep it from destroying a user's own `dckt`)

**Files:**
- Modify: `internal/install/alias.go` (add `planBinaryAlias`)
- Modify: `internal/install/service.go` (`Outcome` gains `AliasFindings`)
- Modify: `internal/install/devmode.go` (`developmentInstallCandidate`, after the binary target is appended)
- Test: `internal/install/devmode_test.go`, `internal/install/uninstall_test.go`

**Interfaces:**
- Consumes: `AliasName`, `roleBinaryAlias`, `AliasFinding`, `AliasForeign`, `AliasPathFor`, `remedyAliasForeign` (Task 1); `InspectTarget(t Target, prior *State, legacy LegacyReproducer) (Inspection, error)`; `LoadState(path string) (*State, error)`.
- Produces:
  ```go
  // in Outcome (service.go):
  AliasFindings []AliasFinding
  // in alias.go:
  func planBinaryAlias(binary string, prior *State) (*Target, *AliasFinding, error)
  ```

- [ ] **Step 1: Write the failing tests** (in `internal/install/devmode_test.go`, `package install_test`; reuse the existing helpers `newWorld`, `newSource`, `devCandidate`, `snapshot`, `assertUnchanged`, `findAction`, `readFile`, `loadState`, and `failingFS` from `adoption_test.go`)

```go
// The development install places dckt beside the binary it installs, as a
// recorded, journaled target attributed to no harness.
func TestDevInstallPlacesTheAlias(t *testing.T) {
	w := newWorld(t)
	mkdirAll(t, w.path(".toy"))
	src := newSource(t)
	bin := filepath.Join(w.home, "bin")

	out := install.DevelopmentInstall(w.devCandidate(t, src, bin))
	if out.Err != nil {
		t.Fatalf("DevelopmentInstall: %v (reason %q)", out.Err, out.Reason)
	}
	alias := filepath.Join(bin, install.AliasName)
	binary := filepath.Join(bin, "docket")
	dest, err := os.Readlink(alias)
	if err != nil {
		t.Fatalf("alias not created: %v", err)
	}
	if dest != binary {
		t.Errorf("alias -> %s, want %s", dest, binary)
	}
	if _, ok := findAction(out, install.OpCreate, alias); !ok {
		t.Errorf("no create action reported for %s: %v", alias, out.Actions)
	}
	if len(out.AliasFindings) != 0 {
		t.Errorf("a fresh install reported alias findings: %+v", out.AliasFindings)
	}
	var rec *install.TargetRecord
	for _, r := range loadState(t, w.roots).Targets {
		if r.Path == alias {
			r := r
			rec = &r
		}
	}
	if rec == nil || rec.Kind != install.KindSymlink || rec.Harness != "" {
		t.Fatalf("alias record = %+v, want an unattributed symlink record", rec)
	}

	// A second run converges: nothing applied, nothing changed on disk.
	before := snapshot(t, w.home)
	again := install.DevelopmentInstall(w.devCandidate(t, src, bin))
	if again.Err != nil || again.Applied || len(again.AliasFindings) != 0 {
		t.Fatalf("second run = %+v", again)
	}
	assertUnchanged(t, before, snapshot(t, w.home), "second development install")
}

// A dckt the release downloader left (a RELATIVE link to docket) is already
// ours: the development install leaves it, reporting no finding.
func TestDevInstallAcceptsTheDownloadersRelativeAlias(t *testing.T) {
	w := newWorld(t)
	mkdirAll(t, w.path(".toy"))
	bin := filepath.Join(w.home, "bin")
	mkdirAll(t, bin)
	alias := filepath.Join(bin, install.AliasName)
	if err := os.Symlink("docket", alias); err != nil {
		t.Fatal(err)
	}
	out := install.DevelopmentInstall(w.devCandidate(t, newSource(t), bin))
	if out.Err != nil || len(out.AliasFindings) != 0 {
		t.Fatalf("outcome = %+v", out)
	}
	if dest, _ := os.Readlink(alias); dest != "docket" {
		t.Errorf("the downloader's relative alias was rewritten to %q", dest)
	}
}

// A dckt docket does not own is never touched; the install still succeeds and
// reports the foreign alias as a finding, and the alias is not recorded.
func TestDevInstallPreservesAForeignAlias(t *testing.T) {
	for _, tc := range []struct {
		name  string
		plant func(t *testing.T, alias string)
	}{
		{"file", func(t *testing.T, alias string) { writeFile(t, alias, "#!/bin/sh\necho mine\n") }},
		{"link elsewhere", func(t *testing.T, alias string) {
			if err := os.Symlink("/usr/bin/true", alias); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t)
			mkdirAll(t, w.path(".toy"))
			bin := filepath.Join(w.home, "bin")
			mkdirAll(t, bin)
			alias := filepath.Join(bin, install.AliasName)
			tc.plant(t, alias)
			aliasBefore := snapshot(t, bin)[install.AliasName] // snapshot keys on the path relative to its root

			out := install.DevelopmentInstall(w.devCandidate(t, newSource(t), bin))
			if out.Err != nil || !out.Applied {
				t.Fatalf("a foreign alias failed the install: %+v", out)
			}
			if len(out.AliasFindings) != 1 || out.AliasFindings[0].Kind != install.AliasForeign ||
				out.AliasFindings[0].Path != alias || out.AliasFindings[0].Remedy == "" {
				t.Fatalf("alias findings = %+v, want one foreign finding with a remedy", out.AliasFindings)
			}
			if got := snapshot(t, bin)[install.AliasName]; got != aliasBefore {
				t.Errorf("the foreign alias was modified")
			}
			if _, err := os.Stat(filepath.Join(bin, "docket")); err != nil {
				t.Errorf("the binary was not installed: %v", err)
			}
			for _, rec := range loadState(t, w.roots).Targets {
				if rec.Path == alias {
					t.Errorf("a foreign alias was recorded as docket's: %+v", rec)
				}
			}
		})
	}
}

// A failed transaction rolls back an alias it created, and leaves a
// pre-existing one exactly as it was.
func TestDevInstallRollbackAndTheAlias(t *testing.T) {
	failCommit := func(w *world) install.FSOps {
		return &failingFS{inner: install.RealFS{}, fail: func(op, path string) error {
			if op == "Rename" && path == w.roots.StatePath() {
				return errors.New("injected: state commit fails")
			}
			return nil
		}}
	}
	t.Run("created alias is removed", func(t *testing.T) {
		w := newWorld(t)
		mkdirAll(t, w.path(".toy"))
		bin := filepath.Join(w.home, "bin")
		o := w.devCandidate(t, newSource(t), bin)
		o.FS = failCommit(w)
		if out := install.DevelopmentInstall(o); out.Err == nil {
			t.Fatalf("the injected commit failure did not fail the install")
		}
		if _, err := os.Lstat(filepath.Join(bin, install.AliasName)); !os.IsNotExist(err) {
			t.Fatalf("rollback left the alias it created: %v", err)
		}
	})
	t.Run("pre-existing alias survives", func(t *testing.T) {
		w := newWorld(t)
		mkdirAll(t, w.path(".toy"))
		bin := filepath.Join(w.home, "bin")
		mkdirAll(t, bin)
		alias := filepath.Join(bin, install.AliasName)
		if err := os.Symlink("docket", alias); err != nil {
			t.Fatal(err)
		}
		o := w.devCandidate(t, newSource(t), bin)
		o.FS = failCommit(w)
		_ = install.DevelopmentInstall(o)
		if dest, err := os.Readlink(alias); err != nil || dest != "docket" {
			t.Fatalf("rollback disturbed the pre-existing alias: %q, %v", dest, err)
		}
	})
}
```

Before trusting the rollback test, confirm in `internal/install/txn.go` that `CommitDocs` renames into `StatePath()`, and re-key the injection if not. Assert the injected error text appears in `out.Err` (learning `assert-pins-outcome-not-mechanism`).

In `internal/install/uninstall_test.go`, extend `newUninstallFixture` to plant and record the alias beside the binary:

```go
	alias := filepath.Join(roots.BinDir, AliasName)
	if err := os.Symlink(binary, alias); err != nil {
		t.Fatal(err)
	}
	records = append(records, TargetRecord{Path: alias, Kind: KindSymlink, LinkTarget: binary, Role: roleBinaryAlias})
```

In `TestUninstallAllAndScopedHarnesses`, next to the existing `unattributed binary` assertion, add:

```go
		if dest, err := os.Readlink(filepath.Join(f.roots.BinDir, AliasName)); err != nil || dest != filepath.Join(f.roots.BinDir, "docket") {
			t.Fatalf("uninstall disturbed the alias: %q, %v", dest, err)
		}
```

Also assert that the alias record is still present in the post-uninstall state, the same way the test checks the binary record (or add that check for both if the test checks neither).

- [ ] **Step 2: Run the tests and confirm they fail**

Run: `go test -count=1 ./internal/install/ -run 'TestDevInstallPlacesTheAlias|TestDevInstallAcceptsTheDownloadersRelativeAlias|TestDevInstallPreservesAForeignAlias|TestDevInstallRollbackAndTheAlias|TestUninstallAllAndScopedHarnesses'`
Expected: FAIL (`AliasFindings` undefined, no alias created). The uninstall test may already pass, which is expected: uninstall only retires harness-attributed targets. Its job is to pin that behavior.

- [ ] **Step 3: Implement**

`internal/install/service.go`, in `Outcome` (after `Collection`):

```go
	// AliasFindings are dckt aliases an operation found missing or foreign.
	// They are findings, never failures: they leave Reason and Err alone, and
	// the app layer renders them as warnings.
	AliasFindings []AliasFinding
```

`internal/install/alias.go`:

```go
// planBinaryAlias is the development install's alias step: the symlink target
// it would own beside binary, or — when what is there is not provably docket's
// — no target and a foreign finding. It inspects with the same ownership proofs
// every other target gets, so "already owned" means exactly what it means for a
// harness link: a link already resolving to binary, or one the prior state
// recorded and that still matches its record. A conflict is reported rather
// than returned as a refusal: the binary install must never be failed by the
// alias.
func planBinaryAlias(binary string, prior *State) (*Target, *AliasFinding, error) {
	t := Target{Path: AliasPathFor(binary), Kind: KindSymlink, LinkTarget: binary, Role: roleBinaryAlias}
	inspection, err := InspectTarget(t, prior, nil)
	if err != nil {
		return nil, nil, err
	}
	if inspection.Disposition == DispositionConflict {
		return nil, &AliasFinding{Kind: AliasForeign, Path: t.Path, Binary: binary, Remedy: remedyAliasForeign}, nil
	}
	return &t, nil, nil
}
```

`internal/install/devmode.go`, in `developmentInstallCandidate`, immediately after the `targets = append(targets, Target{... Role: roleBinary})` block:

```go
	// The alias rides the same transaction as the binary it names, so a
	// rollback removes an alias this run created and restores nothing it did
	// not. A foreign dckt is left out of the plan and reported, never refused.
	prior, err := LoadState(o.Roots.StatePath())
	if err != nil {
		return fail(out, ReasonStateInvalid, err)
	}
	aliasTarget, aliasFinding, err := planBinaryAlias(installedBinary, prior)
	if err != nil {
		return fail(out, ReasonFilesystemFailed, err)
	}
	if aliasTarget != nil {
		targets = append(targets, *aliasTarget)
	}
```

After `out = applyPlan(...)` and before `collectPostCommitLocked`:

```go
	if aliasFinding != nil {
		out.AliasFindings = append(out.AliasFindings, *aliasFinding)
	}
```

A second `LoadState` under the same held lock is equivalent to `applyPlan`'s own read; reuse an in-scope value if one exists. The alias gets no `owner` entry, so `desiredState` records it unattributed and `scopedTo` keeps it out of every prune scan.

- [ ] **Step 4: Run the tests and confirm they pass**

Run: `go test -count=1 ./internal/install/`
Expected: PASS. Tests that enumerate exact created paths or bin-dir contents may now see `<bin>/dckt`: add it to their expectations, never loosen them. `TestDevInstallIsIdempotent` and `TestCheckVerifiesTheDevelopmentBinary` should stay green unchanged; diagnose if not.

- [ ] **Step 5: Mutation-check**

1. Delete the `targets = append(targets, *aliasTarget)` line (back up the file first). `TestDevInstallPlacesTheAlias` must go red.
2. Change `planBinaryAlias` to return the target even on conflict. `TestDevInstallPreservesAForeignAlias` must go red: either the install refuses with `ownership-conflict` or the file changes.

Restore from the backup copy each time and run with `-count=1`.

- [ ] **Step 6: Commit**

```bash
git add internal/install/alias.go internal/install/service.go internal/install/devmode.go internal/install/devmode_test.go internal/install/uninstall_test.go
git commit -m "feat(install): development install places the dckt alias beside the binary"
```

---

### Task 3: `install check` locates every installed binary and reports its alias

**Build tier:** standard

**Files:**
- Modify: `internal/install/roots.go` (`UserRoots.StateHome`, `ReleaseBinaryRecordPath`)
- Modify: `internal/install/alias.go` (`readReleaseBinaryPath`, `checkBinaryAliases`)
- Modify: `internal/install/service.go` (`Check`)
- Test: `internal/install/alias_test.go`, `internal/install/roots_test.go`, `internal/install/devmode_test.go`, `internal/install/service_test.go`

**Interfaces:**
- Consumes: `InspectBinaryAlias`, `AliasFinding` (Task 1); `Outcome.AliasFindings` (Task 2); `roleBinary`.
- Produces:
  ```go
  // UserRoots
  StateHome string // XDG_STATE_HOME or ~/.local/state
  func (r UserRoots) ReleaseBinaryRecordPath() string // "" when StateHome == ""
  // alias.go
  func readReleaseBinaryPath(recordPath string) (string, error) // "" when absent/no usable path=
  func checkBinaryAliases(state *State, roots UserRoots) ([]AliasFinding, error)
  ```

**Why the downloader record:** release state (`install.json`) records no binary, since the downloader places it. The downloader's `release-binary.record` is the only durable statement of where a release binary lives. Dev installs record theirs as a `roleBinary` target. Check reads both.

- [ ] **Step 1: Write the failing tests**

`internal/install/roots_test.go`, in the style of the existing accessor assertions:

```go
func TestResolveRootsStateHome(t *testing.T) {
	home := cleanTempDir(t)
	roots, err := ResolveRoots(func() (string, error) { return home, nil }, func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	if got, want := roots.ReleaseBinaryRecordPath(), filepath.Join(home, ".local", "state", "docket", "release-binary.record"); got != want {
		t.Errorf("default ReleaseBinaryRecordPath = %q, want %q", got, want)
	}
	state := filepath.Join(home, "xdg-state")
	roots, err = ResolveRoots(func() (string, error) { return home, nil }, func(k string) string {
		if k == "XDG_STATE_HOME" {
			return state
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := roots.ReleaseBinaryRecordPath(), filepath.Join(state, "docket", "release-binary.record"); got != want {
		t.Errorf("XDG ReleaseBinaryRecordPath = %q, want %q", got, want)
	}
	if got := (UserRoots{}).ReleaseBinaryRecordPath(); got != "" {
		t.Errorf("zero roots ReleaseBinaryRecordPath = %q, want empty", got)
	}
}
```

(`roots_test.go` may be `package install` or `package install_test`; qualify the names to match the file. `cleanTempDir` is its existing helper.)

`internal/install/alias_test.go`:

```go
func TestReadReleaseBinaryPath(t *testing.T) {
	dir := testsupport.TempDir(t)
	rec := filepath.Join(dir, "release-binary.record")
	if got, err := readReleaseBinaryPath(rec); err != nil || got != "" {
		t.Fatalf("absent record = %q, %v; want empty, nil", got, err)
	}
	write := func(body string) {
		if err := os.WriteFile(rec, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("path=/abs/bin/docket\nversion=v1.0.0\nsha256=abc\nalias=/abs/bin/dckt\n")
	if got, err := readReleaseBinaryPath(rec); err != nil || got != "/abs/bin/docket" {
		t.Fatalf("record path = %q, %v", got, err)
	}
	write("version=v1.0.0\n")
	if got, err := readReleaseBinaryPath(rec); err != nil || got != "" {
		t.Fatalf("record without path= = %q, %v; want empty", got, err)
	}
	write("path=relative/docket\n")
	if got, err := readReleaseBinaryPath(rec); err != nil || got != "" {
		t.Fatalf("relative path= = %q, %v; want empty", got, err)
	}
}

// The Go reader and the POSIX writer spell the record location and its path=
// key independently; this ties the two so a respelling on either side reddens.
func TestReleaseRecordSpellingMatchesDownloader(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "release", "downloader", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`record="${XDG_STATE_HOME:-$HOME/.local/state}/docket/release-binary.record"`,
		`printf 'path=%s\n' "$dest"`,
	} {
		if !strings.Contains(string(src), want) {
			t.Errorf("downloader no longer contains %q; ReleaseBinaryRecordPath/readReleaseBinaryPath must move with it", want)
		}
	}
	if got := (UserRoots{StateHome: "/s"}).ReleaseBinaryRecordPath(); got != "/s/docket/release-binary.record" {
		t.Errorf("ReleaseBinaryRecordPath = %q", got)
	}
}
```

(Add `"strings"` to the imports.)

`internal/install/devmode_test.go`, extending `TestCheckVerifiesTheDevelopmentBinary` with two sub-tests that reuse its `setup`:

```go
	t.Run("alias deleted is a finding, not drift", func(t *testing.T) {
		w, o, binary := setup(t)
		alias := filepath.Join(filepath.Dir(binary), install.AliasName)
		if err := os.Remove(alias); err != nil {
			t.Fatal(err)
		}
		before := snapshot(t, w.home)
		out := install.Check(o)
		if out.Reason != "" || out.Err != nil {
			t.Fatalf("a missing alias failed the check: reason %q err %v", out.Reason, out.Err)
		}
		if len(out.AliasFindings) != 1 || out.AliasFindings[0].Kind != install.AliasMissing || out.AliasFindings[0].Path != alias {
			t.Fatalf("alias findings = %+v", out.AliasFindings)
		}
		assertUnchanged(t, before, snapshot(t, w.home), "check over a missing alias")
	})

	t.Run("alias foreign is a finding, not drift", func(t *testing.T) {
		w, o, binary := setup(t)
		alias := filepath.Join(filepath.Dir(binary), install.AliasName)
		if err := os.Remove(alias); err != nil {
			t.Fatal(err)
		}
		writeFile(t, alias, "mine\n")
		before := snapshot(t, w.home)
		out := install.Check(o)
		if out.Reason != "" || out.Err != nil {
			t.Fatalf("a foreign alias failed the check: reason %q err %v", out.Reason, out.Err)
		}
		if len(out.AliasFindings) != 1 || out.AliasFindings[0].Kind != install.AliasForeign {
			t.Fatalf("alias findings = %+v", out.AliasFindings)
		}
		assertUnchanged(t, before, snapshot(t, w.home), "check over a foreign alias")
	})
```

Also add one assertion to the existing `healthy` sub-test: `len(out.AliasFindings) == 0`.

`internal/install/service_test.go`, release mode: after a successful release `install.Install` in a `newWorld` fixture, plant a downloader record and a binary under `<home>/.local/bin`:

```go
func TestCheckReportsTheReleaseAlias(t *testing.T) {
	// Arrange a healthy release installation the way this file's existing
	// Check tests do, then add what the downloader would have left.
	w := newWorld(t, ".claude")
	o := w.options(nil) // use this file's existing release-install options helper
	if out := install.Install(o); out.Err != nil {
		t.Fatalf("Install: %v (reason %q)", out.Err, out.Reason)
	}
	bin := filepath.Join(w.home, ".local", "bin")
	mkdirAll(t, bin)
	binary := filepath.Join(bin, "docket")
	writeFile(t, binary, "release binary\n")
	recDir := filepath.Join(w.home, ".local", "state", "docket")
	mkdirAll(t, recDir)
	writeFile(t, filepath.Join(recDir, "release-binary.record"), "path="+binary+"\nversion=v1.0.0\nsha256=x\n")

	check := o
	check.FS = panicFS{}
	out := install.Check(check)
	if out.Reason != "" || out.Err != nil {
		t.Fatalf("check: reason %q err %v", out.Reason, out.Err)
	}
	if len(out.AliasFindings) != 1 || out.AliasFindings[0].Kind != install.AliasMissing {
		t.Fatalf("alias findings = %+v, want one missing", out.AliasFindings)
	}

	if err := os.Symlink("docket", filepath.Join(bin, install.AliasName)); err != nil {
		t.Fatal(err)
	}
	if out := install.Check(check); len(out.AliasFindings) != 0 {
		t.Fatalf("a healthy release alias reported findings: %+v", out.AliasFindings)
	}
}
```

If the file's release-install fixture needs a harness dir or catalog set differently, copy the arrangement of the nearest existing healthy-`Check` test in `service_test.go` verbatim. Do not invent new fixture helpers.

- [ ] **Step 2: Run the tests and confirm they fail**

Run: `go test -count=1 ./internal/install/ -run 'TestResolveRootsStateHome|TestReadReleaseBinaryPath|TestReleaseRecordSpellingMatchesDownloader|TestCheckVerifiesTheDevelopmentBinary|TestCheckReportsTheReleaseAlias'`
Expected: FAIL (undefined `StateHome`, `ReleaseBinaryRecordPath`, `readReleaseBinaryPath`; no findings from `Check`).

- [ ] **Step 3: Implement**

`internal/install/roots.go`. Add the field and set it in `ResolveRoots`. Do **not** add it to the `requireDirIfPresent` loop: install never creates or requires it, and it only locates the downloader's record.

```go
	StateHome  string // XDG_STATE_HOME or ~/.local/state (the release downloader's record)
```

```go
		StateHome:  xdgOr(getenv, "XDG_STATE_HOME", filepath.Join(home, ".local", "state")),
```

```go
// ReleaseBinaryRecordPath is the release downloader's ownership record: where
// it says the release binary it installed lives. It is the downloader's file,
// spelled independently in internal/release/downloader/install.sh and tied to
// this spelling by TestReleaseRecordSpellingMatchesDownloader. Empty roots
// (StateHome unset) name no record.
func (r UserRoots) ReleaseBinaryRecordPath() string {
	if r.StateHome == "" {
		return ""
	}
	return filepath.Join(r.StateHome, "docket", "release-binary.record")
}
```

`internal/install/alias.go`. Add `"bufio"` and `"strings"` to the imports.

```go
// readReleaseBinaryPath returns the absolute binary path the release
// downloader's record names, or "" when there is no record or it names no
// usable absolute path — the downloader itself grants no ownership to such a
// record. A record that exists but cannot be read is an error, not an absence.
func readReleaseBinaryPath(recordPath string) (string, error) {
	if recordPath == "" {
		return "", nil
	}
	f, err := os.Open(recordPath)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "", nil
	case err != nil:
		return "", fmt.Errorf("install: reading %s: %w", recordPath, err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if v, ok := strings.CutPrefix(sc.Text(), "path="); ok {
			if filepath.IsAbs(v) {
				return filepath.Clean(v), nil
			}
			return "", nil
		}
	}
	if err := sc.Err(); err != nil {
		return "", fmt.Errorf("install: reading %s: %w", recordPath, err)
	}
	return "", nil
}

// checkBinaryAliases classifies the alias beside every binary an installer
// owns: the development installation's recorded binary, and the binary the
// release downloader's record names. It only reads.
func checkBinaryAliases(state *State, roots UserRoots) ([]AliasFinding, error) {
	var binaries []string
	seen := map[string]bool{}
	add := func(p string) {
		if p != "" && !seen[filepath.Clean(p)] {
			seen[filepath.Clean(p)] = true
			binaries = append(binaries, filepath.Clean(p))
		}
	}
	if state != nil {
		for _, rec := range state.Targets {
			if rec.Role == roleBinary {
				add(rec.Path)
			}
		}
	}
	release, err := readReleaseBinaryPath(roots.ReleaseBinaryRecordPath())
	if err != nil {
		return nil, err
	}
	add(release)

	var findings []AliasFinding
	for _, binary := range binaries {
		f, err := InspectBinaryAlias(binary)
		if err != nil {
			return nil, err
		}
		if f != nil {
			findings = append(findings, *f)
		}
	}
	return findings, nil
}
```

`internal/install/service.go`, in `Check`, right after the `checkBinaryRecords` block (and before the `prunes` scan). Findings are attached regardless of whether drift follows:

```go
	// The dckt alias beside each installed binary is a finding, never drift:
	// the binary works without it, and a check that failed on it would fail
	// the release downloader's closing `install check` for a user who merely
	// owns a dckt of their own.
	aliasFindings, err := checkBinaryAliases(state, o.Roots)
	if err != nil {
		return fail(out, ReasonFilesystemFailed, err)
	}
	out.AliasFindings = aliasFindings
```

Because `fail(out, …)` copies `out`, findings set before a later drift return are preserved in that failed outcome. Confirm with the drift test path that this holds.

- [ ] **Step 4: Run the tests and confirm they pass**

Run: `go test -count=1 ./internal/install/`
Expected: PASS.

Then run `go test -count=1 ./internal/bashupgrade/`: its simulated release install now has a downloader record but no alias, so check exits 0 with a warning. If a test there pins exact check output, expect the `binary-alias-missing` warning (or plant the alias); never suppress it.

- [ ] **Step 5: Mutation-check**

1. Remove the `add(release)` line. `TestCheckReportsTheReleaseAlias` must go red.
2. Change one character of the record spelling in `ReleaseBinaryRecordPath` (for example `release-binary.records`). `TestResolveRootsStateHome` and `TestReleaseRecordSpellingMatchesDownloader` must go red.
3. In the downloader, temporarily change `release-binary.record` to `release-binary.recordX` on the `record=` line. `TestReleaseRecordSpellingMatchesDownloader` must go red.

Restore from backups and use `-count=1`.

- [ ] **Step 6: Commit**

```bash
git add internal/install/roots.go internal/install/roots_test.go internal/install/alias.go internal/install/alias_test.go internal/install/service.go internal/install/service_test.go internal/install/devmode_test.go
git commit -m "feat(install): install check reports a missing or foreign dckt as a finding"
```

---

### Task 4: Render alias findings as warnings in the result document

**Build tier:** standard

**Files:**
- Modify: `internal/app/finding_codes.go` (two census constants, and `AllFindingCodes` in sorted position)
- Modify: `internal/app/install.go` (`NewInstallResult`)
- Test: `internal/app/install_test.go`

**Interfaces:**
- Consumes: `install.Outcome.AliasFindings`, `install.AliasFinding`, `install.AliasMissing`, `install.AliasForeign`.
- Produces: finding codes `binary-alias-missing` (`FCBinaryAliasMissing`) and `binary-alias-foreign` (`FCBinaryAliasForeign`). Each finding becomes an `InstallWarning{Diagnostic: config.Diagnostic{Code, Severity: config.SeverityWarning, Path, Message, Remedy}}`.

- [ ] **Step 1: Write the failing tests** (`internal/app/install_test.go`)

```go
// A dckt finding is a warning: it never changes the result class, so a check
// that only finds an alias problem still exits clean.
func TestInstallResultRendersAliasFindingsAsWarnings(t *testing.T) {
	out := install.Outcome{
		Mode: install.ModeRelease,
		AliasFindings: []install.AliasFinding{
			{Kind: install.AliasMissing, Path: "/b/dckt", Binary: "/b/docket", Remedy: "re-run"},
			{Kind: install.AliasForeign, Path: "/c/dckt", Binary: "/c/docket", Remedy: "move it"},
		},
	}
	r := NewInstallResult(OperationInstallCheck, out)
	if r.Result != ResultNoOp {
		t.Fatalf("result = %q, want %q: an alias finding must not change the class", r.Result, ResultNoOp)
	}
	if len(r.Warnings) != 2 {
		t.Fatalf("warnings = %+v", r.Warnings)
	}
	want := []struct{ code, path, remedy string }{
		{string(FCBinaryAliasMissing), "/b/dckt", "re-run"},
		{string(FCBinaryAliasForeign), "/c/dckt", "move it"},
	}
	for i, w := range want {
		got := r.Warnings[i]
		if got.Code != w.code || got.Path != w.path || got.Remedy != w.remedy ||
			got.Severity != config.SeverityWarning || got.Message == "" {
			t.Errorf("warning[%d] = %+v, want code %s path %s remedy %s", i, got, w.code, w.path, w.remedy)
		}
	}
	human := r.HumanText()
	if !strings.Contains(human, "warning: /c/dckt") || !strings.Contains(human, "move it") {
		t.Errorf("human text does not show the foreign alias warning with its remedy:\n%s", human)
	}
}
```

Also confirm that the existing `TestInstallResultNoWarningsOmitsField` still passes unchanged (no findings means no `warnings` key).

- [ ] **Step 2: Run the test and confirm it fails**

Run: `go test -count=1 ./internal/app/ -run 'TestInstallResultRendersAliasFindingsAsWarnings|TestInstallResultNoWarningsOmitsField'`
Expected: FAIL (`FCBinaryAliasMissing` undefined).

- [ ] **Step 3: Implement**

`internal/app/finding_codes.go`, in the "Install-maintenance findings" group:

```go
	// binary-alias-missing / binary-alias-foreign are the warning codes an
	// install check or development install surfaces for the dckt alias beside
	// an installed binary. They are warnings only: the primary result is
	// unaffected.
	FCBinaryAliasForeign FindingCode = "binary-alias-foreign"
	FCBinaryAliasMissing FindingCode = "binary-alias-missing"
```

In `AllFindingCodes`, insert both in sorted order: after `FCAuthoredInputTooLarge`, before `FCBranchMalformed` (`binary-alias-foreign`, then `binary-alias-missing`). `TestFindingCodeRegistryIntegrity` checks the ordering.

`internal/app/install.go`, at the end of `NewInstallResult` (before `return r`):

```go
	r.Warnings = append(r.Warnings, aliasWarnings(out.AliasFindings)...)
```

and below it:

```go
// aliasWarnings renders the dckt alias findings as warnings. The alias is a
// convenience beside the binary, so its absence or a foreign occupant is
// reported with its remedy and never reclassifies the operation.
func aliasWarnings(findings []install.AliasFinding) []InstallWarning {
	var out []InstallWarning
	for _, f := range findings {
		code, msg := FCBinaryAliasMissing, "the dckt alias for "+f.Binary+" is missing"
		if f.Kind == install.AliasForeign {
			code, msg = FCBinaryAliasForeign, "dckt here is not docket's alias for "+f.Binary+"; left untouched"
		}
		out = append(out, InstallWarning{Diagnostic: config.Diagnostic{
			Code: string(code), Severity: config.SeverityWarning, Path: f.Path, Message: msg, Remedy: f.Remedy,
		}})
	}
	return out
}
```

- [ ] **Step 4: Run the tests and confirm they pass**

Run: `go test -count=1 ./internal/app/ -run 'Install|FindingCode'`
Expected: PASS, including `TestFindingCodeRegistryIntegrity` and `TestNoInlineFindingCodeLiterals`. If a schema or golden test of the result document (see `internal/app/schema_registry.go`) enumerates warning codes, regenerate or update it as its own failure message instructs.

- [ ] **Step 5: Mutation-check**

Comment out the `r.Warnings = append(r.Warnings, aliasWarnings(...)...)` line. The new test must go red. Restore it.

- [ ] **Step 6: Commit**

```bash
git add internal/app/finding_codes.go internal/app/install.go internal/app/install_test.go
git commit -m "feat(app): surface dckt alias findings as install warnings"
```

---

### Task 5: The release downloader places and records the alias

**Build tier:** standard

**Files:**
- Modify: `internal/release/downloader/install.sh`
- Modify: `tests/test_release_downloader.sh` (`DL_REAL_TOOLS`, new Section G)
- Modify: `tests/test_release_downloader_refusals.sh`, `tests/test_release_downloader_converge.sh` (twin helper block: `DL_REAL_TOOLS` only)

**Interfaces:**
- Consumes: nothing from the Go tasks. The record spelling is tied to Task 3 by `TestReleaseRecordSpellingMatchesDownloader`, so do not change the `record=` line or the `printf 'path=%s\n' "$dest"` line.
- Produces: `$bin_dir/dckt` (relative symlink `docket`) and a record line `alias=$bin_dir/dckt` when the alias is ours.

- [ ] **Step 1: Write the failing tests**

In `tests/test_release_downloader.sh`:

1. Add `ln` to `DL_REAL_TOOLS` (keep the list space-separated): `DL_REAL_TOOLS='curl tar gzip uname mktemp mkdir mv cp chmod rm grep sed cat dirname ln'`. Update the comment above it to say `ln` is the alias step's one new tool.
2. Make the same `DL_REAL_TOOLS` edit in `tests/test_release_downloader_refusals.sh` and `tests/test_release_downloader_converge.sh`. The header's TWIN NOTE requires the copied helper block to stay in sync.
3. Insert a new section before `# --- (F) mutation pass` (keep F last) and update the file header's STRUCTURE paragraph to mention Section G:

```bash
# --- (G) the dckt alias: created, converged, and never taken from its owner ----------------------
# Uses the host's readlink in the OUTER shell only (test scaffolding); the downloader itself
# decides with `[ -L ]` / `[ -ef ]` builtins and creates with `ln -s`.
if have sha256sum; then PROV_G=sha256sum; elif have openssl; then PROV_G=openssl; else PROV_G=''; fi
if [ -n "$PROV_G" ]; then
  ALIAS_ERR="$WORK/alias.err"

  # (G1) fresh install creates a RELATIVE dckt -> docket and records it.
  dl_build_sandbox "$PROV_G" || nok "G1: sandbox build failed"
  dl_case; VER=v0.1.0; dl_mk_release "$RELEASES" "$VER"
  ALIAS="$RUN_BIN/dckt"
  dl_run --version "$VER" --harness claude 2>"$ALIAS_ERR"; rc=$?
  if [ "$rc" = 0 ]; then ok "alias: fresh install exits 0"; else nok "alias: fresh install exit $rc"; fi
  if [ -L "$ALIAS" ] && [ "$(readlink "$ALIAS")" = docket ]; then ok "alias: dckt is a relative symlink to docket"; else nok "alias: dckt missing or not -> docket ($(readlink "$ALIAS" 2>/dev/null))"; fi
  if [ "$ALIAS" -ef "$DEST" ]; then ok "alias: dckt resolves to the installed binary"; else nok "alias: dckt does not resolve to $DEST"; fi
  if grep -qxF "alias=$ALIAS" "$RECORD"; then ok "alias: ownership record names the alias"; else nok "alias: record lacks alias=: $(tr '\n' '|' < "$RECORD" 2>/dev/null)"; fi
  if grep -qF warning "$ALIAS_ERR"; then nok "alias: fresh install warned: $(cat "$ALIAS_ERR")"; else ok "alias: fresh install printed no warning"; fi
  if [ -s "$TRIP_LOG" ]; then nok "alias: a banned tool was invoked"; else ok "alias: tripwire log empty"; fi

  # (G2) rerun converges: the link and the record are unchanged, no warning.
  link_before=$(readlink "$ALIAS"); rec_before=$(cat "$RECORD")
  : > "$FAKE_DOCKET_LOG"
  dl_run --version "$VER" --harness claude 2>"$ALIAS_ERR"; rc=$?
  if [ "$rc" = 0 ] && [ "$(readlink "$ALIAS")" = "$link_before" ] && [ "$(cat "$RECORD")" = "$rec_before" ]; then
    ok "alias: rerun converges (exit 0, link and record unchanged)"
  else
    nok "alias: rerun did not converge (exit $rc)"
  fi
  if grep -qF warning "$ALIAS_ERR"; then nok "alias: converging rerun warned"; else ok "alias: converging rerun printed no warning"; fi

  # (G3) a foreign regular file named dckt: untouched, install succeeds with a warning, not recorded.
  dl_build_sandbox "$PROV_G" || nok "G3: sandbox build failed"
  dl_case; dl_mk_release "$RELEASES" "$VER"
  ALIAS="$RUN_BIN/dckt"
  printf '#!/bin/sh\necho mine\n' > "$ALIAS"; foreign_sha=$(dl_sha "$ALIAS")
  dl_run --version "$VER" --harness claude 2>"$ALIAS_ERR"; rc=$?
  if [ "$rc" = 0 ]; then ok "alias-foreign-file: install exits 0"; else nok "alias-foreign-file: exit $rc"; fi
  if [ ! -L "$ALIAS" ] && [ "$(dl_sha "$ALIAS")" = "$foreign_sha" ]; then ok "alias-foreign-file: foreign dckt byte-identical"; else nok "alias-foreign-file: foreign dckt was modified"; fi
  if grep -qF "warning" "$ALIAS_ERR" && grep -qF -- "$ALIAS" "$ALIAS_ERR"; then ok "alias-foreign-file: warning names the path"; else nok "alias-foreign-file: no warning naming $ALIAS (got: $(tr '\n' ' ' < "$ALIAS_ERR"))"; fi
  if grep -q '^alias=' "$RECORD"; then nok "alias-foreign-file: a foreign dckt was recorded as owned"; else ok "alias-foreign-file: record carries no alias="; fi
  if [ -f "$DEST" ]; then ok "alias-foreign-file: the binary was still installed"; else nok "alias-foreign-file: binary missing"; fi

  # (G4) an ABSOLUTE link to the binary (the development installer's spelling) converges silently.
  dl_build_sandbox "$PROV_G" || nok "G4: sandbox build failed"
  dl_case; dl_mk_release "$RELEASES" "$VER"
  ALIAS="$RUN_BIN/dckt"
  ln -s "$DEST" "$ALIAS"
  dl_run --version "$VER" --harness claude 2>"$ALIAS_ERR"; rc=$?
  if [ "$rc" = 0 ] && [ "$(readlink "$ALIAS")" = "$DEST" ]; then ok "alias-absolute: absolute link to docket left as is"; else nok "alias-absolute: exit $rc, link now $(readlink "$ALIAS" 2>/dev/null)"; fi
  if grep -qF warning "$ALIAS_ERR"; then nok "alias-absolute: an owned absolute alias was warned about"; else ok "alias-absolute: no warning"; fi
  if grep -qxF "alias=$ALIAS" "$RECORD"; then ok "alias-absolute: record names the alias"; else nok "alias-absolute: record lacks alias="; fi

  # (G5) a symlink elsewhere, and a dangling one: both foreign, both untouched.
  for _kind in elsewhere dangling; do
    dl_build_sandbox "$PROV_G" || nok "G5: sandbox build failed"
    dl_case; dl_mk_release "$RELEASES" "$VER"
    ALIAS="$RUN_BIN/dckt"
    case $_kind in elsewhere) _tgt="$SANDBOX/bin/cat" ;; dangling) _tgt="$RUN_BIN/gone" ;; esac
    ln -s "$_tgt" "$ALIAS"
    dl_run --version "$VER" --harness claude 2>"$ALIAS_ERR"; rc=$?
    if [ "$rc" = 0 ] && [ "$(readlink "$ALIAS")" = "$_tgt" ]; then ok "alias-$_kind: install exits 0 and the link is untouched"; else nok "alias-$_kind: exit $rc, link now $(readlink "$ALIAS" 2>/dev/null)"; fi
    if grep -qF warning "$ALIAS_ERR"; then ok "alias-$_kind: warned"; else nok "alias-$_kind: no warning"; fi
  done
else
  ok "no hash provider on host — alias cases skipped"
fi
```

- [ ] **Step 2: Run the test and confirm it fails**

Run: `bash tests/test_release_downloader.sh`
Expected: `NOT OK` lines for G1 (no `dckt`), G3/G5 (no warning), and G4 (no `alias=`).

- [ ] **Step 3: Implement in `internal/release/downloader/install.sh`**

1. Contract comment: after the "It keeps its own ownership record and REFUSES to replace a binary it does not own" bullet, add:

```sh
#   - It also places dckt, a RELATIVE symlink to docket, beside the binary (ln -s is its one tool
#     beyond the list above). An absent dckt is created; a dckt already resolving to the binary is
#     left as is; anything else is left UNTOUCHED with a warning, and the install still succeeds —
#     the binary install is never failed by the alias. The record gains alias= only when the
#     alias is in place and ours.
```

2. Next to `die()`:

```sh
warn() { printf '%s\n' "install.sh: warning: $1" >&2; }
```

3. Usage text: after the "will only ever replace a binary it installed." line, add:

```sh
"Also places dckt, a symlink to docket, in the same directory, unless a dckt" \
"it did not create is already there (then it warns and leaves it alone)." \
```

4. Between step (3) (the `mv -f "$stage" "$dest"` line, **left byte-identical**) and step (4) (record publication), insert:

```sh
# (3b) The dckt alias. Identity is decided by -ef (device + inode, every symlink hop followed), never
# by a link's spelling: the development installer writes an absolute link, this script a relative
# one, and both are ours. A dangling or foreign link fails -ef and is left alone.
alias_path="$bin_dir/dckt"
alias_owned=no
if [ -L "$alias_path" ] && [ "$alias_path" -ef "$dest" ]; then
	alias_owned=yes
elif [ ! -e "$alias_path" ] && [ ! -L "$alias_path" ]; then
	if ln -s docket "$alias_path"; then
		alias_owned=yes
	else
		warn "could not create the dckt alias at $alias_path; docket is installed, re-run to retry"
	fi
else
	warn "$alias_path exists and is not docket's alias; left untouched. Move or delete it, then re-run to get dckt."
fi
```

5. In the record block (4), insert the `alias=` line **between** the `version=` and `sha256=` printfs, so the brace group still ends on the unconditional `sha256=` printf whose status the `|| { rm …; die …; }` guard reads (learning `brace-group-guard-covers-last-command`):

```sh
{
	printf 'path=%s\n' "$dest"
	printf 'version=%s\n' "$version"
	if [ "$alias_owned" = yes ]; then printf 'alias=%s\n' "$alias_path"; fi
	printf 'sha256=%s\n' "$bin_sha"
} > "$record_tmp" || { rm -f "$record_tmp"; die "cannot write the ownership record"; }
```

The existing `path=`/`version=`/`sha256=` asserts use `grep -qxF` per line, so the added line breaks none of them. Leave the `printf 'path=%s\n' "$dest"` line byte-identical, because Task 3's `TestReleaseRecordSpellingMatchesDownloader` pins it.

- [ ] **Step 4: Run all three downloader tests and confirm they pass**

Run:
```bash
bash tests/test_release_downloader.sh
bash tests/test_release_downloader_refusals.sh
bash tests/test_release_downloader_converge.sh
```
Expected: no `NOT OK` lines, exit 0. The spelling-ban assert `(4)` stays green, because `ln` is not banned.

Also confirm the smoke's doctoring anchor survived: `grep -c -F -- 'mv -f "$stage" "$dest" || die "cannot move the staged binary into $dest"' internal/release/downloader/install.sh` must print `1`.

Then run the Go tie from Task 3: `go test -count=1 ./internal/install/ -run TestReleaseRecordSpellingMatchesDownloader` (PASS).

- [ ] **Step 5: Mutation-check (spec acceptance 6)**

Back up `install.sh`, replace the whole `(3b)` block with `alias_owned=no`, and rerun: G1 must go red. Restore. Separately, drop `ln` from the main file's `DL_REAL_TOOLS`: G1 must go red (proving the sandbox constrains the step). Restore.

- [ ] **Step 6: Commit**

```bash
git add internal/release/downloader/install.sh tests/test_release_downloader.sh tests/test_release_downloader_refusals.sh tests/test_release_downloader_converge.sh
git commit -m "feat(release): downloader places and records the dckt alias"
```

---

### Task 6: Invocation-name independence, smoke assertions, and header text

**Build tier:** economy

**Files:**
- Create: `internal/bashupgrade/alias_invocation_test.go`
- Modify: `scripts/release-smoke.sh` (Blocks D and E), `scripts/release-smoke.md` (Behavior table rows D and E)
- Modify: `install.sh` (repository root, header comment only)

**Interfaces:**
- Consumes: `buildDocket(t *testing.T) string` (in `internal/bashupgrade/restore_test.go`, `sync.Once`-cached real build of `./cmd/docket`); `install.AliasName`.

**Verified at plan time:** no production code reads `os.Args[0]` (the only hit is test support), and the cobra root is the fixed `Use: "docket"`. The `os.Executable()` sites only re-exec or read the running binary. Re-check with `grep -rn 'os.Args\[0\]' --include='*.go' internal cmd`.

- [ ] **Step 1: Write the failing test** (`internal/bashupgrade/alias_invocation_test.go`; match the package clause of `restore_test.go`)

```go
package bashupgrade

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/install"
	"github.com/danielhanold/docket/internal/testsupport"
)

// The alias is only useful if the binary behaves the same whatever name it was
// invoked by. This uses the real built binary (shared with the upgrade tests via
// buildDocket's sync.Once, so it costs no extra build) behind a dckt symlink.
func TestAliasInvocationMatchesDocket(t *testing.T) {
	bin := buildDocket(t)
	dir := testsupport.TempDir(t)
	alias := filepath.Join(dir, install.AliasName)
	if err := os.Symlink(bin, alias); err != nil {
		t.Fatal(err)
	}
	run := func(path string) string {
		t.Helper()
		out, err := exec.Command(path, "version", "--json").Output()
		if err != nil {
			t.Fatalf("%s version --json: %v", path, err)
		}
		return string(out)
	}
	if got, want := run(alias), run(bin); got != want {
		t.Fatalf("dckt version --json = %q, docket version --json = %q", got, want)
	}
}
```

If `internal/bashupgrade` has a test-name or tempdir guard (for example the `tempdir-exempt` comment convention seen in `restore_test.go`), follow it. `testsupport.TempDir` is the sanctioned helper.

- [ ] **Step 2: Run it**

Run: `go test -count=1 ./internal/bashupgrade/ -run TestAliasInvocationMatchesDocket`
Expected: PASS straight away. This test pins existing behavior (acceptance 4) rather than driving new code. To prove it can fail, temporarily point `run(alias)` at `exec.Command(path, "version")` (non-JSON output) and confirm red, then restore.

- [ ] **Step 3: Smoke assertions in `scripts/release-smoke.sh`**

At the end of Block D (after the `grep -qxF "version=$VERSION" "$S_RECORD"` check):

```bash
S_ALIAS="$S_BIN/dckt"
[ -L "$S_ALIAS" ] && [ "$S_ALIAS" -ef "$S_DEST" ] \
	|| die "install: no dckt alias resolving to the installed binary at $S_ALIAS"
grep -qxF "alias=$S_ALIAS" "$S_RECORD" \
	|| die "install: ownership record does not name the dckt alias: $(tr '\n' '|' < "$S_RECORD")"
alias_ver=$(env HOME="$S_HOME" XDG_STATE_HOME="$S_STATE" XDG_BIN_HOME="$S_BIN" \
	XDG_DATA_HOME="$S_DATA" XDG_CONFIG_HOME="$S_CONFIG" TMPDIR="$S_TMP" "$S_ALIAS" version --json) \
	|| die "install: '$S_ALIAS version --json' exited non-zero"
[ "$alias_ver" = "$(run_docket version --json)" ] \
	|| die "install: dckt version --json differs from docket version --json"
```

In Block E, after the result-class `case`:

```bash
case $check_json in
	*'"binary-alias-'*) die "check: install check reports a dckt alias warning after a clean install" ;;
esac
```

Then run `bash -n scripts/release-smoke.sh`. The smoke itself runs in the release-candidate CI workflow. If a local bundle is practical (`go run ./cmd/releasepkg --help`), run the smoke and record `SMOKE PASS` in the results file; otherwise record it as CI-only evidence.

- [ ] **Step 4: Contract doc `scripts/release-smoke.md`**

Behavior table rows:
- D: append "; a `dckt` symlink resolving to the binary lands beside it, the record names it (`alias=`), and `dckt version --json` equals `docket version --json`."
- E: append "; it carries no `binary-alias-*` warning."

- [ ] **Step 5: Repository-root `install.sh` header**

In the header paragraph "The heavy lifting (building the binary, linking harnesses, …)", add a sentence:

```sh
# An install leaves the docket binary plus dckt, a symlink to it, in the bin directory; a dckt
# docket did not create is never replaced (the install warns and leaves it alone).
```

Run `bash tests/test_install_bootstrap.sh` to confirm the bootstrapper tests do not pin the header.

- [ ] **Step 6: Commit**

```bash
git add internal/bashupgrade/alias_invocation_test.go scripts/release-smoke.sh scripts/release-smoke.md install.sh
git commit -m "test(release): dckt answers like docket; smoke asserts the alias"
```

---

### Task 7: Whole-suite gate

**Build tier:** standard

- [ ] **Step 1: Regenerate embedded assets if anything under the asset roots changed**

This change edits no skill or agent asset. Confirm with `git diff --name-only <pre-dispatch HEAD>..HEAD` that nothing under the asset roots moved. If something did, run `go generate ./internal/assets/` and commit.

- [ ] **Step 2: Run the full suite**

Run: `go run ./cmd/docket development test`
Expected: green. Read the budget report even on green. Any `BUDGET WATCH:` or `SERIAL CONFIRMED OVER BUDGET:` line on `tests/test_release_downloader.sh` (Section G adds six downloader runs) is a finding to act on, by trimming the cases or adjusting the budget row with a measured reason. It is not something to ignore.

- [ ] **Step 3: Acceptance trace (write it into the results file)**

| Spec acceptance | Evidence |
|---|---|
| 1. Fresh release install leaves `dckt` → `docket`; rerun converges | Task 5 G1, G2 |
| 2. Fresh development install does the same | Task 2 `TestDevInstallPlacesTheAlias` |
| 3. Foreign `dckt` untouched by both, installs succeed with a warning, `install check` reports it | Task 5 G3/G5; Task 2 `TestDevInstallPreservesAForeignAlias`; Task 3 check sub-tests + `TestCheckReportsTheReleaseAlias`; Task 4 warning rendering |
| 4. `dckt version --json` == `docket version --json` | Task 6 `TestAliasInvocationMatchesDocket`; smoke Block D (CI) |
| 5. `uninstall` leaves binary and alias in place | Task 2 `TestUninstallAllAndScopedHarnesses` extension |
| 6. Spelling-ban and PATH-sandbox tests pass with `ln -s`; removing the alias step reddens the new test | Task 5 Step 4 and Step 5 mutation |
