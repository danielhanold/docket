<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0317 — Release packaging and four-harness acceptance](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-27-0317-release-packaging-and-four-harness-acceptance.md)**
<!-- docket:backlink:end -->
# Release Packaging and Four-Harness Acceptance — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Produce one deterministic, checksummed release-candidate bundle (four Darwin/Linux amd64/arm64 archives + sorted `checksums.txt` + rendered POSIX downloader), prove its install/upgrade/refusal behavior hermetically, wire a non-publishing `release-candidate` GitHub Actions workflow with native tuple smokes, and hand the four-harness live acceptance to the human merge gate as a documented checklist.

**Architecture:** A small repository-owned Go packaging command (`cmd/releasepkg`) backed by `internal/release` builds `./cmd/docket` four times with `CGO_ENABLED=0 -trimpath` and injected `internal/buildinfo` identity, writes byte-deterministic single-member tar.gz archives keyed on a caller-supplied source epoch, emits a sorted SHA-256 manifest, and renders the `/bin/sh` downloader with the bundle's default version. The downloader verifies checksums before extraction and extraction before execution, keeps its own release-binary ownership record, runs the landed `docket install` before an atomic `mv -f` binary replacement, and finishes with read-only `docket install check`. CI packages once and smokes the same bytes natively per tuple; nothing publishes.

**Tech Stack:** Go 1.26 stdlib only (archive/tar, compress/gzip, crypto/sha256, os/exec with explicit argv), Cobra-free plain-flag dev command, POSIX `/bin/sh` + curl + tar + (`sha256sum` | OpenSSL), GitHub Actions with commit-pinned actions, bash suite runner `scripts/run-tests.sh`.

**Spec:** `docs/superpowers/specs/2026-08-20-release-packaging-and-four-harness-acceptance-design.md` — the authority; this plan argues from it. Read it before any task.

## Global Constraints

Copied from the spec; every task's requirements implicitly include these.

- Module path `github.com/danielhanold/docket`; production binary is `./cmd/docket` only. The packager is development/release tooling — a second `cmd/` like `cmd/genassets`, never a shipped product executable and never a public Go API (`internal/release`). It must not invoke a shell, import workflow packages (`internal/app`, `internal/cli`, `internal/githubcli`…), or teach the production binary about GitHub Releases. `os/exec` with explicit argument arrays only.
- The packager **accepts, never infers**: source root, safe version, full commit, source epoch. Git cleanliness/identity validation stays at the workflow boundary (Phase 1), not in the package.
- Safe version grammar (also filename-safe): `^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z][0-9A-Za-z.-]*)?$`. Reject everything else before any build.
- Exact artifact set, no more, no fewer: `docket_<version>_{darwin,linux}_{amd64,arm64}.tar.gz`, `checksums.txt`, `install.sh`. Each archive: exactly one regular executable member named `docket`, mode `0755`, numeric uid/gid 0, empty uname/gname, ModTime = source epoch UTC, USTAR format, gzip header ModTime = source epoch, no gzip Name/OS leakage. Same inputs + toolchain ⇒ identical bytes.
- `checksums.txt`: one lowercase-hex SHA-256 line per distributable file except itself, `<hash>  <filename>` (two spaces, `sha256sum -c` compatible), sorted bytewise by filename.
- Downloader is `/bin/sh`; runtime deps exactly `/bin/sh`, `curl`, `tar`, and one of `sha256sum`/OpenSSL. Never bash, python, perl, `shasum`, `jq`, `eval`, or a downloaded helper. Unverified bytes are never executed or moved into the bin dir. No `--force` path. It never deletes a previous embedded version tree (0323's job).
- The workflow is **non-publishing**: read-only permissions, no tag, no GitHub Release, no release-notes/doc edits, no Docket metadata writes. Publication is 0318.
- Do not touch behavior owned by 0305–0316; do not modify the repository-root `install.sh` (0322's contributor bootstrap — it stays byte-for-byte); no uninstall/GC (0323); no new `docket` subcommand for packaging or releases.
- Every new safety guard gets a mutation test — remove its premise, watch the assert redden — and every Go mutation probe/re-verification runs `go test -count=1` (learnings `guards-are-code`, `cached-runner-serves-a-mutated-tree`). Grep-shaped bans state their spelling limitation in the guard header (learning `byte-pattern-guard-matches-a-spelling`).
- Promised file modes are chmod'd explicitly and regression-tested under `umask 077` (learning `promised-file-mode-needs-explicit-chmod`).
- Scratch dirs use templates: `mktemp -d "${TMPDIR:-/tmp}/<name>.XXXXXX"` (AGENTS.md Shell); `mv -f` on every install/replace path; no `producer | early-exiting-consumer` under pipefail.
- Every new `tests/test_*.sh` file needs a `tests/runtime-budgets.tsv` row and the table total (`EXPECTED_TOTAL` in `tests/test_runtime_budgets.sh`) moves with it. The build gate runs the whole resolved suite (`scripts/run-tests.sh` via `finalize.test_command`); every `OVER BUDGET:` line is a finding to act on. Go tests wire in through `tests/test_go_toolchain.sh`'s `go test ./...`.
- The fresh-session four-harness acceptance and the four-tuple native execution are **external truth**: no in-repo test may claim them; they route to named human items in the change's results record (learnings `external-truth-needs-a-human-checkpoint`, `generated-artifact-loaded-at-process-start`, `harness-behavior-is-mode-and-version-scoped`). Runner labels for the smoke matrix are revalidated against current GitHub-hosted inventory during implementation, in the workflow-authoring task.
- Repository-mode note (learning `metadata-branch-invisible-to-suite`): hermetic suites see only fixtures and this checkout; results-record content is a build-time artifact on the metadata branch, so the plan's final task authors the in-branch checklist the results record will carry, not the results file itself.

## File Structure

```
internal/release/version.go(+_test)        safe version grammar, tuple table, input validation
internal/release/archive.go(+_test)        deterministic single-member tar.gz writer + reopening verifier
internal/release/checksums.go(+_test)      sorted manifest writer + bidirectional validator
internal/release/downloader/install.sh     POSIX downloader source (rendered into bundles)
internal/release/render.go(+_test)         downloader render (default-version stamp), //go:embed
internal/release/package.go(+_test)        orchestration: 4× go build, identity check, verify, emit
cmd/releasepkg/main.go(+main_test.go)      flag parsing + exit codes for the dev command
scripts/release-smoke.sh(+.md)             per-tuple native smoke driver (CI + host-tuple test)
.github/workflows/release-candidate.yml    4-phase non-publishing candidate workflow
docs/release/four-harness-acceptance.md    manual merge-gate checklist template (final task)
tests/test_release_package.sh              suite wiring: packager integration + workflow guards
tests/test_release_downloader.sh           downloader happy paths (sha256sum + openssl, umask 077)
tests/test_release_downloader_refusals.sh  checksum/archive/tuple/tool/ownership refusals
tests/test_release_downloader_converge.sh  interruption injection + rerun convergence + upgrade
```

Go tests in `internal/release` and `cmd/releasepkg` run inside the existing `go test ./...` (Check 3 of `tests/test_go_toolchain.sh`) — no new wiring needed for them; the three new shell files self-register via the glob and need budget rows.

---

### Task 1: `internal/release` version grammar, tuple table, and input validation

**Files:**
- Create: `internal/release/version.go`, `internal/release/version_test.go`

**Interfaces:**
- Produces (later tasks depend on these exact names):

```go
package release

// Tuple is one approved GOOS/GOARCH target.
type Tuple struct{ OS, Arch string }

// Tuples is the fixed approved set, in emission order.
func Tuples() []Tuple // {darwin,amd64},{darwin,arm64},{linux,amd64},{linux,arm64}

// ArchiveName returns docket_<version>_<os>_<arch>.tar.gz.
func ArchiveName(version string, t Tuple) string

// ValidateVersion returns a descriptive error unless version matches the
// safe grammar ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z][0-9A-Za-z.-]*)?$.
func ValidateVersion(version string) error

// Inputs are the caller-proved facts the packager accepts, never infers.
type Inputs struct {
    SourceRoot  string // absolute path to the checkout to build
    Version     string // safe version, validated
    Commit      string // full 40-hex source commit
    SourceEpoch int64  // unix seconds; derives BuildDate and all timestamps
    OutDir      string // destination directory for the bundle
}

// Validate checks every field before any work: version grammar, 40-hex
// lowercase commit, positive epoch, absolute existing SourceRoot, absolute OutDir.
func (in Inputs) Validate() error

// BuildDate renders SourceEpoch as UTC RFC3339 (2026-08-20T00:00:00Z).
func (in Inputs) BuildDate() string
```

- [ ] Write `version_test.go` first: accepted versions (`v1.0.0`, `v0.2.10-rc.1`, `v1.0.0-candidate.base`), rejected (`1.0.0`, `v1.0`, `v01.0.0`, `v1.0.0-`, `v1.0.0-a/b`, `v1.0.0-a b`, empty, leading/trailing space); all four archive names for `v1.2.3`; `Inputs.Validate` rejecting short/uppercase/non-hex commit, zero/negative epoch, relative or missing SourceRoot, relative OutDir — every element validated before any is acted on (learning `validate-the-whole-input-set-first`); `BuildDate` for epoch 0 = `1970-01-01T00:00:00Z`.
- [ ] Run `go test -count=1 ./internal/release/` — FAIL (package absent), then implement and re-run to PASS. Regexp compiled once at package scope with `regexp.MustCompile`.
- [ ] Mutation probe: drop the `[0-9A-Za-z]` prerelease-lead restriction — the `v1.0.0-` and separator tests must redden (`-count=1`).
- [ ] Commit: `feat(0317): release input validation — safe version grammar and approved tuple table`

### Task 2: Deterministic archive writer and reopening verifier

**Files:**
- Create: `internal/release/archive.go`, `internal/release/archive_test.go`

**Interfaces:**

```go
// WriteArchive writes path as a gzip'd USTAR tar holding exactly one regular
// member named "docket" with the given bytes, mode 0755, uid/gid 0, empty
// uname/gname, ModTime=epoch UTC; gzip header ModTime=epoch, Name="", OS=0xFF.
func WriteArchive(path string, binary []byte, epoch int64) error

// VerifyArchive reopens path, checks it holds exactly one regular member
// named "docket" with mode 0755 and nothing else (no links, no dirs, no
// second member, no traversal-shaped name), and returns the member's size
// and the archive file's lowercase-hex SHA-256.
func VerifyArchive(path string) (size int64, sha256hex string, err error)
```

- [ ] Tests first, on synthetic bytes (no `go build` here — determinism is provable cheaply): write twice with identical inputs into two paths, assert **byte-identical files**; write with a different epoch, assert bytes differ (pins that epoch actually enters the stream — removal mutation for determinism plumbing); `VerifyArchive` round-trips size + hash.
- [ ] Refusal tests: hand-craft hostile archives with `archive/tar` directly — a second member, a symlink member, a member named `../docket` or `bin/docket`, a 0644 mode member, an empty archive — `VerifyArchive` refuses each with a distinct error mentioning the member. Also: `WriteArchive` output must not contain host leakage — reopen and assert `Uid==0 && Gid==0 && Uname=="" && Gname==""` and `hdr.Format` is USTAR.
- [ ] Implement: `tar.Header{Typeflag: tar.TypeReg, Name: "docket", Mode: 0o755, Size: int64(len(binary)), ModTime: time.Unix(epoch, 0).UTC(), Format: tar.FormatUSTAR}`; `gzip.NewWriterLevel(f, gzip.BestCompression)` with `zw.ModTime = time.Unix(epoch, 0).UTC()`, `zw.OS = 0xFF`. Write via temp file in the destination dir + `os.Rename` (atomic, same filesystem — learning `atomic-generated-write`). `go test -count=1 ./internal/release/` PASS.
- [ ] Mutation probes (each must redden, `-count=1`): skip the second-member check; accept mode 0644; accept a name containing `/`. Restore.
- [ ] Commit: `feat(0317): deterministic single-member release archives with reopening verifier`

### Task 3: Sorted checksum manifest, writer and bidirectional validator

**Files:**
- Create: `internal/release/checksums.go`, `internal/release/checksums_test.go`

**Interfaces:**

```go
// WriteChecksums hashes each named file in dir and writes dir/checksums.txt:
// "<64 lowercase hex>  <filename>\n" per file, sorted bytewise by filename.
// names must be exactly the distributable set; a missing file fails.
func WriteChecksums(dir string, names []string) error

// ValidateChecksums proves the correspondence in BOTH directions: every
// manifest line matches a present file's hash and syntax; every expected
// name has exactly one line; no extra, duplicate, or unsafe (path
// separator, leading '-') filename appears; checksums.txt lists itself never.
func ValidateChecksums(dir string, expected []string) error
```

- [ ] Tests first: golden ordering (feed names unsorted, assert output sorted bytewise); two-space `sha256sum -c` format; validator reddens on — a flipped hex digit, a missing line, a duplicated line, an extra line for an unexpected file, a `../evil` filename, an uppercase hash, a one-space separator; and the **reverse direction** — an expected file with no line (learning `correspondence-guard-runs-one-way`).
- [ ] Implement with `crypto/sha256` + `sort.Strings`; write via temp + rename. `go test -count=1` PASS.
- [ ] Mutation probes: drop the duplicate-line check; drop the reverse (expected-but-absent) loop — each must redden.
- [ ] Commit: `feat(0317): sorted SHA-256 manifest with bidirectional validation`

### Task 4: POSIX downloader source and its static-ban guard

**Files:**
- Create: `internal/release/downloader/install.sh`
- Create: `tests/test_release_downloader.sh` (this task adds only the static section; behavior sections come in Tasks 7–9)
- Modify: `tests/runtime-budgets.tsv` (row for the new test file), `tests/test_runtime_budgets.sh` (`EXPECTED_TOTAL`)

**Interfaces (the script's contract; Tasks 5–9 and the smoke script depend on every spelling):**

- Shebang `#!/bin/sh`, `set -u`. Flags: `--version <v…>` (default: the stamped `DOCKET_DEFAULT_VERSION`), `--bin-dir <absolute dir>` (default `${XDG_BIN_HOME:-$HOME/.local/bin}`), repeatable `--harness <claude|codex|cursor|opencode>` (forwarded verbatim as separate argv words to `docket install --harness <h> …`). Unknown flag / non-absolute bin dir / bad version (same grammar as Task 1, enforced with a `case`/POSIX pattern, not ERE) ⇒ usage error exit 2 before any probe.
- `DOCKET_DEFAULT_VERSION="@DOCKET_DEFAULT_VERSION@"` placeholder line near the top — Task 5 renders it.
- Download base: `base_url="${DOCKET_RELEASE_BASE_URL:-https://github.com/danielhanold/docket/releases/download}/<version>"`. The env override is the **one documented test/mirror seam** (comment says exactly that); `file://` URLs through real curl keep tests network-free.
- Platform map: `uname -s` Darwin|Linux, `uname -m` x86_64|amd64→amd64, arm64|aarch64→arm64; anything else fails **before any network request**.
- Prereq probe before any destination change: `curl`, `tar`, and SHA-256 provider — prefer `command -v sha256sum`, else `command -v openssl`; neither ⇒ actionable failure naming both. `sha_file() { … }` is the only hashing seam (sha256sum path: `sha256sum "$1" | { read -r h _; printf '%s\n' "$h"; }`; openssl path: `openssl dgst -sha256 -r "$1"` first field). The script contains no invocation of bash, python, perl, `shasum`, `jq`, or `eval`, and never executes downloaded shell.
- Scratch: `work=$(mktemp -d "${TMPDIR:-/tmp}/docket-release.XXXXXX")` with a `trap 'rm -rf "$work"' EXIT` that leaves stderr diagnostics intact (learning `transient-resource-lifecycle`).
- Integrity sequence: download `checksums.txt` and the one tuple archive into `$work`; require **exactly one** syntactically valid manifest line for the archive name (`grep -c`, count must equal 1 — zero, two, or malformed all refuse); compare `sha_file` output against it **before extraction**; `tar -tzf` listing must be exactly the single line `docket` (refuse links/dirs/extra/traversal by refusing any listing that is not that one line); extract with `tar -xzf … -C "$work"`, require a regular file, `chmod 755` explicitly (umask, learning `promised-file-mode-needs-explicit-chmod`), record `bin_sha=$(sha_file "$work/docket")`.
- Ownership record `record="${XDG_STATE_HOME:-$HOME/.local/state}/docket/release-binary.record"` — three `key=value` lines: `path=`, `version=`, `sha256=`. Decision table before touching `dest="$bin_dir/docket"`:
  - no `docket` at dest ⇒ fresh install allowed;
  - dest exists, record exists, record's `path` equals dest and `sha_file dest` equals record's `sha256` ⇒ owned, replace allowed;
  - dest exists and `sha_file dest` equals `$bin_sha` ⇒ interrupted-completion, converge allowed;
  - anything else (absent record + foreign binary, drifted owned bytes, record naming a different path, malformed record) ⇒ **refuse**, preserve every existing byte, no `--force`, never execute the unknown binary.
- Install sequence (spec order, verbatim): (1) stage to `stage=$(mktemp "$bin_dir/.docket-stage.XXXXXX")`, `cp` verified binary, `chmod 755` — beside dest so the rename is one filesystem; (2) `"$stage" install` + one `--harness <h>` pair per selection; on failure remove stage, prior binary untouched; (3) `mv -f "$stage" "$dest"`; (4) publish record atomically: write `$record.tmp.$$` beside it (`mkdir -p` its dir first), `mv -f` into place; (5) `exec "$dest" install check` — its exit status is the script's.
- Exit codes: 0 success; 2 usage; 1 everything else, each with a one-line actionable stderr diagnostic.

- [ ] Write the script complete per the contract above. Keep every guard a real branch (`|| { echo …>&2; exit 1; }`), no function hides an exit path behind a pipeline's last command (learning `brace-group-guard-covers-last-command`).
- [ ] `sh -n internal/release/downloader/install.sh` clean; run `--help`/bad-flag by hand for exit 2.
- [ ] Start `tests/test_release_downloader.sh` with the static section: assert the file exists, has the `#!/bin/sh` shebang, carries the `@DOCKET_DEFAULT_VERSION@` placeholder, and a banned-spelling grep finds no word-boundary invocation of `bash`, `python`, `perl`, `shasum`, `jq`, or `eval` — guard header states this matches spellings, not the property, and that the PATH-sandbox tests in Tasks 7–8 are the semantic check (learning `byte-pattern-guard-matches-a-spelling`). Add the budget row + bump `EXPECTED_TOTAL`.
- [ ] Mutation: add a commented-out `eval` line — the ban must redden (proves it scans, not just file presence); restore.
- [ ] Commit: `feat(0317): POSIX release downloader — checksum-verified install/upgrade with ownership record`

### Task 5: Downloader render and packager orchestration

**Files:**
- Create: `internal/release/render.go`, `internal/release/render_test.go`, `internal/release/package.go`, `internal/release/package_test.go`

**Interfaces:**

```go
//go:embed downloader/install.sh
var downloaderSource string

// RenderDownloader returns the downloader with the placeholder line
// DOCKET_DEFAULT_VERSION="@DOCKET_DEFAULT_VERSION@" stamped to version.
// Exactly one placeholder must exist; zero or two is an error.
func RenderDownloader(version string) (string, error)

// Package builds the full candidate bundle into in.OutDir:
// four archives, checksums.txt, install.sh. Deterministic for equal
// inputs+toolchain. goBin is the go tool to run ("go" in production;
// injectable for tests). It refuses a non-empty OutDir collision set.
func Package(in Inputs, goBin string) error
```

- [ ] `render_test.go` first: stamped output contains `DOCKET_DEFAULT_VERSION="v1.2.3"` and no `@DOCKET_DEFAULT_VERSION@`; a doctored source with the placeholder duplicated or removed errors (feed via a small unexported seam `renderDownloaderFrom(src, version)` that `RenderDownloader` wraps).
- [ ] `package_test.go` — one real integration run (the four cross-builds are warm from `tests/test_go_toolchain.sh`'s existing four-tuple check, so this is minutes cold, seconds warm): `Package` with `SourceRoot` = repo root (locate via `go env GOMOD`? no — use `runtime.Caller` + `../..`, cleaned), version `v0.0.1-planintegration`, commit = 40 hex fixture `strings.Repeat("ab", 20)`, epoch `1700000000`, temp OutDir. Assert: exactly the 6 expected names, nothing else; `VerifyArchive` passes on all four; `ValidateChecksums` passes; rendered `install.sh` stamped and `0755`; **host-tuple identity**: extract the host GOOS/GOARCH archive's member, run `docket version --json`, decode, assert `version`, `commit`, and `build_date == in.BuildDate()` exactly (mismatch fails packaging, per spec). Note in a comment: non-host tuples get identical ldflags and are identity-checked by the CI native smokes — a host process cannot execute them (external truth).
- [ ] Implement `Package`: `Validate()` first; build each tuple via `exec.Command(goBin, "build", "-trimpath", "-ldflags", flags, "-o", tmpBin, "./cmd/docket")` with `Dir=SourceRoot`, `Env` = `os.Environ()` + `CGO_ENABLED=0`, `GOOS`, `GOARCH`, `GOFLAGS=` (cleared, so ambient flags cannot enter the bytes); ldflags exactly the three `-X github.com/danielhanold/docket/internal/buildinfo.{Version,Commit,BuildDate}=` values; `WriteArchive` each; `VerifyArchive` each (reopen-and-check is mandatory, not optional); render + write `install.sh` with explicit `os.Chmod(0o755)`; `WriteChecksums(dir, sortedNames)` where names = 4 archives + `install.sh`; `ValidateChecksums` before returning. Host-tuple identity check inside `Package` (run the built host binary `version --json` before archiving) so a wrong stamp fails packaging everywhere, not only under test.
- [ ] `go test -count=1 ./internal/release/` PASS. Determinism spot-check by hand: run the integration test's `Package` twice in a scratch `main` (or re-run test with `-count=2` and a fixed OutDir comparison inside the test — implement the test to package twice into two dirs and byte-compare `checksums.txt`, which pins all archive bytes transitively).
- [ ] Mutation probes (`-count=1`): drop `CGO_ENABLED=0` from the env — identity/determinism assertions or archive comparison must redden (if green on this machine, redden via the ldflags mutation instead and record it); drop the `ValidateChecksums` call and corrupt one file in a doctored unit test of the refusal path.
- [ ] Commit: `feat(0317): candidate packager — four-tuple deterministic bundle with host identity check`

### Task 6: `cmd/releasepkg` dev command

**Files:**
- Create: `cmd/releasepkg/main.go`, `cmd/releasepkg/main_test.go`

**Interfaces:**
- Flags (stdlib `flag`, no Cobra — this is dev tooling): `--source`, `--version`, `--commit`, `--source-epoch`, `--out`; all required; `--source-epoch` decimal unix seconds. Exit 2 on usage error with all missing flags named at once (learning `validate-the-whole-input-set-first`); exit 1 with the packager's error on failure; exit 0 printing `packaged <version> -> <out>` on success.

- [ ] `main_test.go` first: build the command with `go build -o` into `t.TempDir()` via `TestMain` (the `cmd/docket` test files are the pattern — see `cmd/docket/main_test.go`); run with no flags → exit 2 naming every missing flag; bad epoch → exit 2; a full happy run against the repo source (same fixture identity as Task 5) → exit 0 and the six files present. Keep the happy run to **one** invocation; the deep assertions live in Task 5.
- [ ] Implement `main.go` as a thin adapter: parse, fill `release.Inputs`, call `release.Package(in, "go")`.
- [ ] `go test -count=1 ./cmd/releasepkg/` PASS. Commit: `feat(0317): releasepkg command — flag adapter over the release packager`

### Task 7: Downloader happy-path tests (both hash providers, hermetic)

**Files:**
- Modify: `tests/test_release_downloader.sh` (behavior sections)
- Test fixtures are built inline — no `go build`: the archive member is a fake `docket` that is itself a `#!/bin/sh` script logging `"$@"` to `$FAKE_DOCKET_LOG` and succeeding on `install`, `install check`, and answering `version --json` with a canned JSON line. `tar -czf` + real `sha256sum`/`openssl` produce the "release dir"; `DOCKET_RELEASE_BASE_URL="file://$release_dir_parent"` feeds real curl with zero network.

**Interfaces:**
- Consumes: the downloader contract from Task 4 verbatim (flag spellings, record path/format, exit codes, `DOCKET_RELEASE_BASE_URL` seam).
- Produces: `dl_mk_release <dir> <version>` and `dl_run <args…>` helpers reused by Tasks 8–9 (define them in this file's top section; the sibling files re-source nothing — each file is hermetic per `tests/README.md`, so copy the ~30-line helper block into each and note the twin in a comment).

- [ ] **PATH sandbox** (the semantic ban): build `$sandbox/bin` holding symlinks to real `sh`-needed tools only — `curl`, `tar`, `sha256sum` (or `openssl`), `uname`, `mktemp`, `mkdir`, `mv`, `cp`, `chmod`, `rm`, `grep`, `sed`, `cat`, `printf`, plus a **tripwire** fake for each banned tool (`bash`, `python`, `python3`, `perl`, `shasum`, `jq`) that writes to `$TRIP_LOG` and exits 9. Run every downloader invocation as `env -i PATH="$sandbox/bin" HOME=… XDG_STATE_HOME=… XDG_BIN_HOME=… TMPDIR=… DOCKET_RELEASE_BASE_URL=… /bin/sh internal/release/downloader/install.sh …`; after each assert `$TRIP_LOG` is empty.
- [ ] Fresh-install happy path, `sha256sum` present: run with `--harness claude --harness opencode`; assert exit 0; `$XDG_BIN_HOME/docket` present, mode `755` **under `umask 077`** (set inside the test); fake log shows, in order, `install --harness claude --harness opencode` then `install check` (argument-capture, spec's "distinct arguments"); ownership record present with correct `path=`, `version=`, `sha256=` (recompute and compare); no stage file left behind.
- [ ] Same run with `sha256sum` removed from the sandbox and `openssl` linked in — identical outcome (learning `green-suite-untested-branch`: both provider branches executed, not mocked away).
- [ ] Default-version path: render a copy with Task 5's placeholder replaced by the fixture version (one `sed` at the test's write boundary), run with **no** `--version`, assert it fetched that version.
- [ ] Idempotent rerun: run again same version — allowed (dest equals requested bytes), exit 0, record unchanged.
- [ ] Mutation pass (each restores after): remove the tripwire-empty assert's subject by linking real `bash` — TRIP assert must be shown able to redden (invoke the tripwire directly once); flip one byte in the release archive — the happy-path run must fail, proving verification precedes install (this is Task 8's refusal, but assert here that **no** `install` line reached the fake log).
- [ ] Update the budget row measurement; `scripts/run-tests.sh --verbose tests/test_release_downloader.sh` green. Commit: `test(0317): downloader happy paths — both hash providers, sandboxed PATH, umask 077`

### Task 8: Downloader refusal and ownership tests

**Files:**
- Create: `tests/test_release_downloader_refusals.sh`
- Modify: `tests/runtime-budgets.tsv` (+row), `tests/test_runtime_budgets.sh` (`EXPECTED_TOTAL`)

Every case asserts three things: non-zero exit, **no byte changed** at the destination/record (hash before/after), and no `install` line in the fake log (unverified bytes never executed). Cases, each its own fixture (learning `assert-pins-outcome-not-mechanism` — also pin the diagnostic keyword per case):

- [ ] Corrupt archive (flip a byte) — "checksum" diagnostic. Missing manifest entry — refuse. Duplicate manifest entry (two lines for the archive) — refuse (count must equal exactly 1). Malformed manifest line (63-hex, one-space) — refuse.
- [ ] Hostile archives: extra member, symlink member, `../docket` member, member named `evil` — tar-listing refusal before extraction reaches the bin dir.
- [ ] Unsupported tuple: fake `uname` reporting `SunOS`/`mips` — fails **before any curl invocation** (tripwire-style fake curl that logs; assert log empty).
- [ ] Missing prerequisites: sandbox without `curl`; without `tar`; with **neither** `sha256sum` nor `openssl` — each an actionable named failure before any destination change.
- [ ] Download failure: base URL pointing at an empty dir — curl fails, clean refusal, empty scratch cleaned up.
- [ ] Non-absolute `--bin-dir`, unknown `--harness` value, bad `--version` — exit 2 usage before probes.
- [ ] Ownership: (a) foreign `docket` at dest, no record — refusal preserving it byte-for-byte; (b) record present but dest bytes drifted — refusal; (c) record naming a **different** bin dir than `--bin-dir` — refusal (changing recorded bin dir is out of scope); (d) malformed record (truncated, extra key) — refusal preserving record and dest; (e) interrupted-completion — dest already equals the newly verified requested binary, record absent → run **succeeds** and publishes the record (convergence, the one allowed "anything else" exception).
- [ ] Asset-install failure: fake docket exits 3 on `install` — prior binary untouched, stage removed, non-zero exit.
- [ ] Mutation pass: comment out the drifted-bytes comparison in the script — case (b) must redden; comment out the single-line tar-listing check — the extra-member case must redden. Restore both.
- [ ] Budget row + total; suite file green. Commit: `test(0317): downloader refusals — checksum, archive, tuple, tool, and ownership guards preserve every byte`

### Task 9: Interruption injection, convergence, and upgrade

**Files:**
- Create: `tests/test_release_downloader_converge.sh`
- Modify: `tests/runtime-budgets.tsv` (+row), `tests/test_runtime_budgets.sh` (`EXPECTED_TOTAL`)

Injection seam: the fake `docket` inside the staged archive reads `FAIL_ON` from the environment (`install` | `check`) to fail at chosen points; the two filesystem-boundary interruptions (before rename, before record publication) are injected by running a **doctored copy** of the downloader with the single `mv -f "$stage" "$dest"` or record-publish line replaced by `exit 97` — the copy is written by the test at its write boundary and the doctoring `sed` asserts it changed exactly one line (learning `assert-detects-removal-not-replacement`: prove the mutation landed).

- [ ] Fail before asset install (`FAIL_ON=install`): prior state fully restored (old binary + old record byte-identical, stage gone).
- [ ] Fail after asset install, before rename (doctored copy, exit 97 at the `mv`): old binary + old record intact — then **rerun the real script**, assert it converges to the requested version (owned-record branch allows replacement, staged install idempotent per spec), record updated.
- [ ] Fail after rename, before record publication (doctored copy): dest = new bytes, record = old — rerun converges via the interrupted-completion branch; record repaired.
- [ ] Fail at final check (`FAIL_ON=check`): binary and record already published (steps 3–4 precede 5); script exits non-zero relaying the check status; rerun exits 0.
- [ ] Upgrade: publish release A (`v0.0.1-a`), install; publish release B (`v0.0.2-b`) in a second release dir; run with `--version v0.0.2-b`; assert record now names B's version + hash, dest is B's bytes, and a sentinel file planted under the fake harness dir before the upgrade is byte-identical after (spec: unrelated harness bytes unchanged).
- [ ] Mutation: in the doctored-copy helper, assert the `sed` diff is non-empty (a silent no-op doctoring would make every interruption test vacuously test the happy path).
- [ ] Budget row + total; file green. Commit: `test(0317): downloader interruption points converge on rerun; upgrade replaces only owned bytes`

### Task 10: Native tuple smoke script

**Files:**
- Create: `scripts/release-smoke.sh`, `scripts/release-smoke.md` (contract note, matching the `scripts/*.md` house pattern)

**Interfaces:**
- Usage: `release-smoke.sh --bundle <dir> --version <v> [--base-bundle <dir> --base-version <v>]`. Bash is fine here (it runs on runners and dev machines, not inside the downloader's constraint); `set -uo pipefail`, no `producer | early-exit` pipelines.
- Behavior (spec "Native tuple smoke", one assertion block each, all against isolated `HOME`/`XDG_*`/bin roots the script creates under a templated mktemp dir): map the host tuple; verify the tuple archive against `checksums.txt` and extract; run the binary directly: `version --json` must report the exact expected version, and `diagnostic runtime --json` the native supported tuple; then drive the **downloader from the bundle** (`DOCKET_RELEASE_BASE_URL=file://…`) to install with `--harness claude --harness codex --harness cursor --harness opencode`; `docket install check --json` clean; same-version rerun idempotent (record + dest hashes unchanged); when `--base-bundle` given: install base first, plant a foreign sentinel under a harness dir, upgrade to head, assert head identity + sentinel unchanged; finally re-run the Task 9 rename-interruption convergence once (doctored-copy technique inlined) to prove the interruption point converges on a real tuple.
- Exit non-zero on the first failed block with a named diagnostic; print `SMOKE PASS <os>/<arch> <version>` on success (the workflow summary greps this exact line).

- [ ] Write the script; verify locally against a real bundle: `go run ./cmd/releasepkg --source "$PWD" --version v0.0.1-smokelocal --commit $(git rev-parse HEAD) --source-epoch $(git log -1 --format=%ct) --out /tmp-scratch…` then run the smoke on the host tuple. This local run is build evidence for the host tuple only — the other three tuples are CI/external truth; say so in `release-smoke.md`.
- [ ] Add a static section to `tests/test_release_package.sh` — created in Task 11 — so no suite change lands here beyond the script itself.
- [ ] Commit: `feat(0317): native tuple smoke driver — verify, install, check, upgrade, converge on the packaged bytes`

### Task 11: `release-candidate` workflow and its guards

**Files:**
- Create: `.github/workflows/release-candidate.yml`
- Create: `tests/test_release_package.sh` (workflow + packager-surface guards; suite wiring)
- Modify: `tests/runtime-budgets.tsv` (+row), `tests/test_runtime_budgets.sh` (`EXPECTED_TOTAL`)

Workflow shape (spec "Candidate workflow", four phases):

- [ ] Triggers: `pull_request` with `paths:` covering `cmd/**`, `internal/**`, `go.mod`, `go.sum`, `skills/**`, `agents/**`, `install.sh`, `scripts/**`, `.github/workflows/release-candidate.yml`; `workflow_dispatch` with `ref` and `version` inputs (version validated in-job against the Task 1 grammar before packaging). Top-level `permissions: contents: read` and nothing else; no publication token anywhere.
- [ ] Job `source-gate` (ubuntu): `actions/checkout` with `fetch-depth: 0`, prove tree cleanliness + resolve full commit + source epoch (`git log -1 --format=%ct`) as outputs; set up Go 1.26; regenerate/check embedded assets (`go run ./cmd/genassets` + `git diff --exit-code` — mirror what `tests/test_asset_bundle_drift.sh` checks); run `scripts/run-tests.sh` (the resolved whole-suite command — read from `.docket.yml`, never a second copy) and surface any `OVER BUDGET:` block into the job summary as a finding.
- [ ] Job `package` (needs source-gate): run `go run ./cmd/releasepkg` once for the **head** candidate (version: dispatch input, or for PRs `v0.0.0-candidate.head.<short-sha>`); for PRs also a **base** candidate `v0.0.0-candidate.base.<short-sha>` from the same source (distinct safe prerelease — the upgrade smoke needs a real prior Go binary); `actions/upload-artifact` the complete candidate dir(s), nothing partial.
- [ ] Job `smoke` matrix (needs package): four native runners — **revalidate labels against current GitHub-hosted inventory now, at authoring time** (as of plan-writing: `macos-15` arm64, `macos-15-intel` amd64, `ubuntu-24.04` amd64, `ubuntu-24.04-arm` arm64; the implementer confirms each label exists and is the claimed architecture before pinning, and records the check in the PR description — emulation or cross-compile does not count); download the artifacts; run `scripts/release-smoke.sh --bundle … --version … [--base-bundle … --base-version …]`.
- [ ] Job `summary` (needs smoke, `if: always()` + explicit per-job result check requiring every tuple): write one machine-readable JSON artifact — source commit, versions, `checksums.txt` content, per-tuple verdicts, and the constant field `"live_host_acceptance": "outstanding — see docs/release/four-harness-acceptance.md"`. An artifact, not a release and not a Docket record.
- [ ] Every `uses:` is pinned to a full commit SHA with a trailing `# vN` comment.
- [ ] `tests/test_release_package.sh` — hermetic guards over the two authored surfaces (grep-shaped; each header states the spelling limitation): workflow file exists; `permissions:` block grants `contents: read` only (assert no `write` token appears in a permissions context); no publishing verbs (`gh release`, `softprops/action-gh-release`, `git tag`, `git push`) appear; every `uses:` matches `@[0-9a-f]{40}`; both `workflow_dispatch` inputs present; matrix names all four tuples; `release-smoke.sh` exists, is executable, and greps for its `SMOKE PASS` line + the `--base-bundle` flag; `internal/release/downloader/install.sh` is referenced by `internal/release/render.go`'s embed. Mutation-test: unpin one action to `@v4` — the SHA guard reddens; add a `git tag` step — the publish ban reddens. Restore.
- [ ] Budget row + total. `scripts/run-tests.sh --verbose tests/test_release_package.sh` green. Commit: `feat(0317): non-publishing release-candidate workflow — gate, package once, four native smokes, evidence summary`

### Task 12: Whole-suite gate

- [ ] Run `go vet ./...` and `go test -count=1 ./...` — green.
- [ ] Run the resolved suite command `scripts/run-tests.sh` (from `finalize.test_command`) in full. Every failure is yours to fix; every `OVER BUDGET:` line is a finding to disposition explicitly (split the file or argue the row in the diff — never bump-and-forget). Verify the three new shell files' budget rows against measured wall clock with headroom (learning `budget-headroom-is-spent-before-it-is-breached`).
- [ ] Commit any gate fixes individually with their reasons.

### Task 13: Four-harness acceptance checklist (the merge-gate document)

**Files:**
- Create: `docs/release/four-harness-acceptance.md`

The fresh-session live acceptance is external truth no in-repo test can promote (spec "Evidence and gate"; learnings `external-truth-needs-a-human-checkpoint`, `generated-artifact-loaded-at-process-start`). This task authors the in-branch checklist the change's results record will reference and carry as named human-verify items — it does **not** run the acceptance and does not write the results file (a metadata-branch artifact).

- [ ] Author the checklist with, verbatim from the spec: the fixture recipe (disposable Git-backed repo, one known build-ready change, local authoritative remote, recorded initial refs + repository-byte digest, setup command spelled out using `git` + the candidate binary); the CLI baseline (protocol v1, operation `status`, result `applied`, expected ready ID); then **one section per harness** (Claude, Codex, Cursor, OpenCode), each a state-to-reproduce procedure, not a conclusion to confirm: (1) install candidate binary + that harness's native assets via the bundle downloader; (2) terminate any process holding the old registry, start a genuinely fresh native session (Cursor: the IDE, never a CLI proxy); (3) directly invoke the installed `docket-status` named agent through that harness's own dispatch surface — never another harness, never a runner shim; (4) have the child run PATH-resolved `docket version --json` and read-only `docket status --repo-dir <fixture> --json`, no maintenance sweep; (5) record the evidence row — harness name + exact vendor version + mode, host OS/arch, candidate commit + archive SHA-256, fresh-process proof, named-child-ran proof (not parent-inline), observed protocol fields + ready ID, unchanged before/after fixture refs and bytes, pass/fail + sanitized transcript location. State the gate: **all four rows required**; a missing, stale-session, cross-harness, or ambiguous observation is not a pass; a failure blocks acceptance and is diagnosed against the recorded harness version/mode without widening the change.
- [ ] Include a short "what merely running `docket version` in the parent does NOT prove" paragraph (spec's non-pass list) and the instruction that the results record's human-verify section copies these items with per-row checkboxes.
- [ ] Commit: `docs(0317): four-harness fresh-session acceptance checklist — the human merge-gate procedure`

---

## Self-Review (performed at plan-writing)

- **Spec coverage:** packaging tool + accepted-not-inferred inputs (T1, T5, T6); artifact contract incl. determinism, reopening validation, sorted manifest (T2, T3, T5); downloader interface/platform/integrity/ownership/sequence (T4, T7–T9); mirror seam (T4/T7 `DOCKET_RELEASE_BASE_URL`); candidate workflow four phases, base-candidate upgrade path, runner-label revalidation, evidence summary (T11); native tuple smoke content (T10); hermetic tests + mutation coverage of every safety guard (T2–T4, T7–T9, T11); fresh-session four-harness acceptance as recorded human gate (T13). Exclusions honored: no tag/release/publish step exists in any task; repo-root `install.sh` untouched; no new `docket` subcommand.
- **Host-tuple caveat:** the spec's per-binary `version --json` identity check is executed for the host tuple at package time and for every tuple in the native smokes — a host process cannot execute foreign-tuple binaries; T5 and T10 state this and route it to CI + the smoke line, not a vacuous assert.
- **Type/name consistency:** `release.Inputs`/`Tuples`/`ArchiveName`/`ValidateVersion`/`WriteArchive`/`VerifyArchive`/`WriteChecksums`/`ValidateChecksums`/`RenderDownloader`/`Package` used consistently across T1–T6; downloader flag spellings, record path `${XDG_STATE_HOME:-$HOME/.local/state}/docket/release-binary.record`, and `DOCKET_RELEASE_BASE_URL` identical across T4, T7–T10.
