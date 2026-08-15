<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0308 — Git adapter and authoritative object source](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-15-0308-git-adapter-and-authoritative-object-source.md)**
<!-- docket:backlink:end -->

# Git Adapter and Authoritative Object Source Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `internal/gitcli` — typed repository discovery, controlled Git execution, remote inspection, targeted fetch, and an immutable revision-pinned object source with NUL-safe batch reads — leaving the invocation checkout untouched.

**Architecture:** A layered adapter: a `Client` owns the resolved git executable, sanitized environment, and timeout policy; `Discover` canonicalizes repository identity from any linked worktree; ref/fetch operations end the mutable boundary at one exact commit; an `ObjectSource` pinned to that commit serves tree listings and batch blob reads. Git execution is private — no exported command runner.

**Tech Stack:** Go 1.26 stdlib only (`os/exec`, `context`, `bytes`, `bufio`, `path/filepath`). Real temporary Git repositories in tests; a Go helper-process (never a shell script) for fake-git failure cases.

**Spec:** `docs/superpowers/specs/2026-08-13-git-adapter-and-authoritative-object-source-design.md` (on the `docket` metadata branch; reconciled 2026-08-15). The spec travels with this plan — executors read both.

## Global Constraints

- Module `github.com/danielhanold/docket`, Go 1.26; **no new module dependencies** — stdlib only.
- `gofmt -l` clean, `go vet ./...` clean, `go test ./...` green — all enforced by `tests/test_go_toolchain.sh` through `scripts/run-tests.sh` (the resolved `finalize.test_command`). Do NOT add a second shell producer for this package.
- Never construct a shell command string or invoke a shell to run Git; argument arrays only.
- No exported `Run(args ...)` or generic command surface; Git execution stays package-private.
- No operation changes the Go process working directory or process-global environment.
- Diagnostics never contain environment values, remote URLs, credentials, blob bytes, or unbounded tool output; stderr excerpts are bounded and explicitly truncated.
- Every mutation probe and manual re-verification uses `go test -count=1` (Go's test cache serves stale passes otherwise — learnings: `cached-runner-serves-a-mutated-tree`).
- All Git path output is read NUL-delimited (`-z`); never display-form / `core.quotePath` output (learnings: `git-path-output-is-quoted`).
- Path identity comparisons canonicalize **every symlink hop** (`filepath.EvalSymlinks` after `filepath.Abs`) — macOS `/tmp` → `/private/tmp` makes this observable in every test (learnings: `canonicalise-every-symlink-hop`).
- Tests run on Darwin and Linux with temporary repositories and local file remotes; no live network.

## File Structure

```
internal/gitcli/
  types.go          # ObjectID, RemoteName, RefName, RepoPath, Revision, TreeEntry, BlobResult, Blob, FileMode, ObjectType + validation
  failure.go        # Operation, FailureKind, Failure (error), constructors
  client.go         # Client, options, executable resolution, timeout policy
  exec.go           # private runner: env sanitization, ctx/timeouts, output capture, bounded stderr
  discover.go       # Discover → Repository identity (canonical primary worktree + common dir)
  refs.go           # RemoteDefaultBranch, FetchBranch, ResolveRef
  source.go         # OpenObjectSource, objectSource, ListTree (NUL-safe ls-tree parse)
  readblobs.go      # ReadBlobs (ls-tree path resolve + cat-file --batch parser)
  types_test.go
  failure_test.go
  exec_test.go      # helper-process env/timeout/cancel/secret tests
  harness_test.go   # real-repo fixture builders (main + docket topologies)
  discover_test.go
  refs_test.go
  source_test.go
  readblobs_test.go
  preserve_test.go  # checkout-preservation + revision-consistency proof matrix
```

`tests/runtime-budgets.tsv` row `tests/test_go_toolchain.sh` (currently 20s) will absorb the new package's real-repo tests — Task 9 re-measures and re-budgets it deliberately (learnings: `budget-headroom-is-spent-before-it-is-breached`).

---

### Task 1: Value types, path/ref validation, failure model

**Files:**
- Create: `internal/gitcli/types.go`
- Create: `internal/gitcli/failure.go`
- Test: `internal/gitcli/types_test.go`, `internal/gitcli/failure_test.go`

**Interfaces:**
- Consumes: nothing (leaf task).
- Produces (exact, used by every later task):

```go
type ObjectID string    // normalized full hex, exact-compare; SHA-1 or SHA-256 length
type RemoteName string
type RefName string     // fully qualified: refs/heads/..., refs/remotes/...
type RepoPath string    // repo-relative byte path; opaque bytes in a Go string

type Revision struct {
    Commit ObjectID
    Remote RemoteName
    Ref    RefName
}

type FileMode string    // git octal mode text: "100644", "100755", "120000", "160000", "040000"
type ObjectType string  // "blob", "tree", "commit"

type TreeEntry struct {
    Path     RepoPath
    Mode     FileMode
    Type     ObjectType
    ObjectID ObjectID
}

type BlobResult struct {
    Path  RepoPath
    Found bool
    Blob  Blob
}

type Blob struct {
    Mode     FileMode
    ObjectID ObjectID
    Bytes    []byte
}

func validateRepoPath(p RepoPath, allowEmptyRootPrefix bool) error
func validateRemoteName(r RemoteName) error
func validateRefName(r RefName) error
func validateObjectID(id ObjectID) error

type Operation string   // "discover", "remote-default-branch", "fetch-branch", "resolve-ref", "open-source", "list-tree", "read-blobs"

type FailureKind string

const (
    KindInvalidRequest        FailureKind = "invalid-request"
    KindExecutableUnavailable FailureKind = "executable-unavailable"
    KindInvalidRepository     FailureKind = "invalid-repository"
    KindRemoteUnavailable     FailureKind = "remote-unavailable"
    KindRefUnavailable        FailureKind = "ref-unavailable"
    KindCommandFailed         FailureKind = "command-failed"
    KindUnexpectedObject      FailureKind = "unexpected-object"
    KindInvalidOutput         FailureKind = "invalid-output"
    KindCancelled             FailureKind = "cancelled"
    KindTimedOut              FailureKind = "timed-out"
)

type Failure struct {
    Operation Operation
    Kind      FailureKind
    ExitCode  int    // 0 when no process exit is involved
    Detail    string // bounded safe prose; never env/URL/credential/bytes
    Err       error  // wrapped cause, may be nil
}

func (f *Failure) Error() string
func (f *Failure) Unwrap() error
func newFailure(op Operation, kind FailureKind, detail string, err error) *Failure
func AsFailure(err error) (*Failure, bool)   // errors.As convenience
```

Validation rules (spec, "Typed vocabulary" + "Immutable object source"):

- `validateRepoPath`: reject NUL bytes, a leading `/`, a trailing `/`, empty components (`a//b`), `.` components, `..` components anywhere, and the empty string — except when `allowEmptyRootPrefix` is true, where `""` is legal (root tree listing only).
- `validateRemoteName`: non-empty; reject NUL, whitespace, a leading `-` (option smuggling), and `/`.
- `validateRefName`: must begin `refs/` with at least two components; reject NUL, whitespace, a leading `-`, empty components, `.`/`..` components, `@{`, `\`, trailing `.lock` component, and `*`.
- `validateObjectID`: non-empty, all lowercase hex, length 40 or 64 (SHA-1 and SHA-256 representable; never truncated).

- [ ] **Step 1: Write failing tests for validation and Failure**

`types_test.go` — table-driven; representative rows (implement all listed rejects):

```go
func TestValidateRepoPath(t *testing.T) {
    good := []RepoPath{"a", "a/b.txt", "docs/changes/active/0001-x.md", "spa ce/né.md"}
    for _, p := range good {
        if err := validateRepoPath(p, false); err != nil {
            t.Errorf("validateRepoPath(%q) = %v, want nil", p, err)
        }
    }
    bad := []RepoPath{"", "/abs", "a/", "a//b", ".", "./a", "a/./b", "..", "a/../b", "a\x00b"}
    for _, p := range bad {
        if err := validateRepoPath(p, false); err == nil {
            t.Errorf("validateRepoPath(%q) = nil, want error", p)
        }
    }
    if err := validateRepoPath("", true); err != nil {
        t.Errorf("empty root prefix must be legal with allowEmptyRootPrefix: %v", err)
    }
}

func TestValidateRefName(t *testing.T) {
    if err := validateRefName("refs/heads/main"); err != nil { t.Fatal(err) }
    bad := []RefName{"main", "heads/main", "refs/", "refs/heads/", "-refs/heads/x",
        "refs/heads/a b", "refs/heads/a..b", "refs/heads/a.lock", "refs/heads/a@{1}", "refs/heads/*"}
    for _, r := range bad {
        if err := validateRefName(r); err == nil { t.Errorf("%q accepted", r) }
    }
}

func TestValidateObjectID(t *testing.T) {
    sha1 := ObjectID(strings.Repeat("ab", 20))
    sha256 := ObjectID(strings.Repeat("cd", 32))
    for _, id := range []ObjectID{sha1, sha256} {
        if err := validateObjectID(id); err != nil { t.Fatal(err) }
    }
    for _, id := range []ObjectID{"", "abc", ObjectID(strings.Repeat("AB", 20)), ObjectID(strings.Repeat("zz", 20))} {
        if err := validateObjectID(id); err == nil { t.Errorf("%q accepted", id) }
    }
}
```

`failure_test.go`:

```go
func TestFailureErrorAndUnwrap(t *testing.T) {
    cause := errors.New("boom")
    f := newFailure("fetch-branch", KindCommandFailed, "git exited 128", cause)
    if !errors.Is(f, cause) { t.Fatal("Unwrap chain broken") }
    var got *Failure
    if !errors.As(f, &got) || got.Kind != KindCommandFailed { t.Fatal("errors.As failed") }
    for _, s := range []string{"fetch-branch", "command-failed", "git exited 128"} {
        if !strings.Contains(f.Error(), s) { t.Errorf("Error() missing %q", s) }
    }
}
```

- [ ] **Step 2: Run to verify failure** — `go test -count=1 ./internal/gitcli/` → FAIL (undefined symbols).
- [ ] **Step 3: Implement `types.go` and `failure.go`** exactly to the Produces block; validation via byte scans and `strings.Split(p, "/")` component checks, no regexp needed.
- [ ] **Step 4: Run to verify pass** — `go test -count=1 ./internal/gitcli/` → PASS; `gofmt -l internal/gitcli` empty; `go vet ./internal/gitcli/` clean.
- [ ] **Step 5: Commit** — `git add internal/gitcli && git commit -m "feat(gitcli): typed values, path/ref validation, failure model"`

---

### Task 2: Client and controlled execution core

**Files:**
- Create: `internal/gitcli/client.go`, `internal/gitcli/exec.go`
- Test: `internal/gitcli/exec_test.go`

**Interfaces:**
- Consumes: Task 1 types.
- Produces:

```go
type Client struct { /* unexported fields */ }

type Option func(*clientConfig)

func WithExecutable(path string) Option            // tests: explicit fake/real git
func WithLocalTimeout(d time.Duration) Option      // must be > 0, else invalid-request at construction
func WithNetworkTimeout(d time.Duration) Option    // must be > 0
func WithBaseEnvironment(env []string) Option      // tests: fully pinned environment

func NewClient(opts ...Option) (*Client, error)
// production: resolves "git" via exec.LookPath once; absolute path recorded;
// missing/unresolvable → Failure{Kind: executable-unavailable}.

// package-private execution seam every operation uses:
type runRequest struct {
    op        Operation
    dir       string        // working directory ("" = process cwd is NOT inherited; always set by callers after discovery)
    args      []string      // git argument vector, no leading "git"
    stdin     []byte        // nil = no stdin
    network   bool          // selects network vs local default timeout
}
type runResult struct {
    stdout   []byte
    stderr   []byte // raw; excerpted only at diagnostic time
    exitCode int
}
func (c *Client) run(ctx context.Context, req runRequest) (runResult, *Failure)

func sanitizeEnvironment(base []string) []string
func stderrExcerpt(stderr []byte) string // ≤ 1024 bytes, appends " [truncated]" when cut
```

`sanitizeEnvironment` removes, **by semantic class** (prefix match on `NAME=`):

- repository/worktree redirection: `GIT_DIR`, `GIT_COMMON_DIR`, `GIT_WORK_TREE`, `GIT_INDEX_FILE`, `GIT_OBJECT_DIRECTORY`, `GIT_ALTERNATE_OBJECT_DIRECTORIES`, `GIT_NAMESPACE`
- config injection: every `GIT_CONFIG*` name (`GIT_CONFIG`, `GIT_CONFIG_GLOBAL`, `GIT_CONFIG_SYSTEM`, `GIT_CONFIG_NOSYSTEM`, `GIT_CONFIG_COUNT`, `GIT_CONFIG_KEY_*`, `GIT_CONFIG_VALUE_*`, `GIT_CONFIG_PARAMETERS`)
- tracing: every `GIT_TRACE*` name (`GIT_TRACE`, `GIT_TRACE2*`, `GIT_TRACE_PACKET`, …)
- also: `GIT_CEILING_DIRECTORIES`, `GIT_DISCOVERY_ACROSS_FILESYSTEM`, `GIT_ALTERNATE*`

then appends: `LC_ALL=C`, `LANG=C`, `GIT_TERMINAL_PROMPT=0`, `GIT_OPTIONAL_LOCKS=0`. Everything else (HOME, XDG, `SSH_AUTH_SOCK`, `GIT_SSH_COMMAND`, proxies, cert roots) survives.

`run` behavior:

- `exec.CommandContext` with the absolute executable; never a shell.
- Effective deadline: caller ctx capped by the class default (30s local, 5m network, or the option overrides); a shorter caller deadline wins. Zero/negative overrides are rejected in `NewClient`.
- On ctx cancellation/timeout: process killed, pipes drained and closed, `cmd.Wait` completed before return; kind `cancelled` vs `timed-out` chosen by `ctx.Err()` (`context.Canceled` vs `context.DeadlineExceeded`).
- Start failure → `executable-unavailable`. Non-zero exit is returned as data (`runResult.exitCode`); classification is the caller's job.
- stdout and stderr captured separately; stdout is binary-safe (`bytes.Buffer`).

- [ ] **Step 1: Write failing helper-process tests**

`exec_test.go` uses the standard `TestMain`/helper-process pattern (a re-exec of the test binary, never a shell script):

```go
func TestMain(m *testing.M) {
    if os.Getenv("GITCLI_HELPER_MODE") != "" {
        helperMain() // acts as fake git per GITCLI_HELPER_MODE; os.Exit inside
    }
    os.Exit(m.Run())
}

// helperMain modes:
//  "dump":   write os.Args[1:] and os.Environ() as NUL-joined lines to stdout, exit 0
//  "stderr": write GITCLI_HELPER_STDERR_BYTES 'x' bytes to stderr, exit 3
//  "block":  ignore args, sleep 30s (killed by timeout/cancel)
//  "exit":   exit with code GITCLI_HELPER_EXIT

func helperClient(t *testing.T, mode string, extraEnv ...string) *Client {
    t.Helper()
    exe, err := os.Executable()
    if err != nil { t.Fatal(err) }
    env := append(os.Environ(), "GITCLI_HELPER_MODE="+mode)
    env = append(env, extraEnv...)
    c, err := NewClient(WithExecutable(exe), WithBaseEnvironment(env),
        WithLocalTimeout(2*time.Second), WithNetworkTimeout(2*time.Second))
    if err != nil { t.Fatal(err) }
    return c
}
```

Required test functions:

```go
// Environment scrub: plant one variable from EACH removed class plus a benign
// sentinel; assert every planted redirection/config/trace var is absent from
// the child's observed environment and the sentinel survives.
func TestSanitizeRemovesRedirectionClassesKeepsAuthSentinel(t *testing.T) {
    planted := []string{
        "GIT_DIR=/evil", "GIT_COMMON_DIR=/evil", "GIT_WORK_TREE=/evil",
        "GIT_INDEX_FILE=/evil", "GIT_OBJECT_DIRECTORY=/evil",
        "GIT_ALTERNATE_OBJECT_DIRECTORIES=/evil", "GIT_NAMESPACE=evil",
        "GIT_CONFIG_GLOBAL=/evil", "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=a.b",
        "GIT_TRACE=1", "GIT_TRACE2_EVENT=/evil", "GIT_CEILING_DIRECTORIES=/evil",
    }
    sentinel := "GIT_SSH_COMMAND=ssh -o BatchMode=yes"
    c := helperClient(t, "dump", append(planted, sentinel)...)
    res, f := c.run(context.Background(), runRequest{op: "discover", dir: t.TempDir(), args: []string{"status"}})
    if f != nil { t.Fatal(f) }
    envDump := string(res.stdout)
    for _, p := range planted {
        name := p[:strings.Index(p, "=")]
        if regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(name) + `=`).MatchString(envDump) {
            t.Errorf("%s leaked into child environment", name)
        }
    }
    if !strings.Contains(envDump, sentinel) { t.Error("benign auth sentinel was scrubbed") }
    for _, added := range []string{"GIT_TERMINAL_PROMPT=0", "LC_ALL=C", "GIT_OPTIONAL_LOCKS=0"} {
        if !strings.Contains(envDump, added) { t.Errorf("missing added control %s", added) }
    }
}

// Timeout: "block" mode must return timed-out within the 2s test timeout, with
// the process actually reaped (no goroutine leak; assert elapsed < 10s).
func TestRunTimeoutKillsProcess(t *testing.T)

// Cancellation: cancel after 100ms → Kind cancelled.
func TestRunCancelledKind(t *testing.T)

// Bounded stderr: "stderr" mode with 64 KiB of stderr → stderrExcerpt ≤ 1024
// bytes + explicit " [truncated]" marker; full stderr still available raw in runResult.
func TestStderrExcerptBounded(t *testing.T)

// Secret non-disclosure: plant SECRET_TOKEN=hunter2 in env and 64KiB stderr
// containing "hunter2"; assert Failure.Error()/Detail from a failing run never
// contains "hunter2" beyond the bounded excerpt policy — construct the caller-side
// failure with newFailure(..., stderrExcerpt(...)) and assert the excerpt is the
// only stderr-derived content and the environment never enters Detail.
func TestNoEnvironmentInDiagnostics(t *testing.T)

// Construction: WithLocalTimeout(0) and negative values → error (invalid-request);
// missing executable path → executable-unavailable.
func TestClientConstructionRejections(t *testing.T)
```

- [ ] **Step 2: Run to verify failure** — `go test -count=1 ./internal/gitcli/ -run 'TestSanitize|TestRun|TestStderr|TestNoEnv|TestClientConstruction'` → FAIL.
- [ ] **Step 3: Implement `client.go` + `exec.go`** per the Produces block.
- [ ] **Step 4: Run to verify pass**; `go vet ./...` clean.
- [ ] **Step 5: Commit** — `feat(gitcli): client construction and controlled git execution`

---

### Task 3: Real-repository test harness (both topologies)

**Files:**
- Create: `internal/gitcli/harness_test.go`

**Interfaces:**
- Consumes: real `git` on PATH (skip with `t.Skip` only if truly absent — CI and dev machines have it).
- Produces (used by Tasks 4–8; `_test.go` only, never product code):

```go
// mainModeRepos: bare origin + writer clone + invocation clone, content on main.
type testRepos struct {
    Origin     string // bare repo path (file remote)
    Writer     string // writer clone: pushes advance origin
    Invocation string // clone under test; remote "origin" -> Origin
}

func newMainModeRepos(t *testing.T) *testRepos
// origin holds branch main with:
//   README.md, .docket.yml, docs/changes/active/0001-a.md,
//   "spa ce/né tab\tfile.md" (space, non-ASCII, tab), "line\nbreak.md" (embedded newline),
//   a symlink "link.md" -> "README.md", an empty file "empty.txt",
//   an executable "tool.sh" (mode 100755)

func newDocketModeRepos(t *testing.T) *testRepos
// origin holds: main with .docket.yml + code; orphan branch "docket" with
// docs/changes/active/... planning files; the invocation clone additionally has
// linked worktrees: ".docket" parked on docket, ".worktrees/feat-x" on a feature branch.

func (r *testRepos) writerCommit(t *testing.T, branch string, files map[string]string) ObjectID
// commit files on branch in the writer clone, push to origin, return the new commit id
// (from the writer clone's rev-parse; independent of the adapter under test)

func gitOut(t *testing.T, dir string, args ...string) string
// independent plumbing oracle: exec real git directly, trimmed stdout, t.Fatal on error
```

Builder rules:

- All repos under `t.TempDir()`; every returned path is `filepath.EvalSymlinks`-canonicalized where compared, but builders return the raw `t.TempDir()` spelling so tests exercise the symlinked `/tmp` case on macOS.
- Explicit test identity: `git -C <dir> -c user.name=t -c user.email=t@t commit …` or config set at init; also `git config commit.gpgsign false` in fixtures.
- One fixture repo sets `core.quotePath=true` explicitly so a developer's global `false` cannot disarm the hostile-path proof (learnings: `git-path-output-is-quoted`).
- The gitlink fixture: a nested real repo committed as a submodule entry (`git submodule add` in the writer with `-c protocol.file.allow=always`, or `git update-index --add --cacheinfo 160000,<sha>,subdir` for a synthetic gitlink — the cacheinfo form is simpler and needs no submodule machinery; use it).
- File remotes use the bare path directly (`git remote add origin /path/to/origin.git`); no network.

- [ ] **Step 1: Write the builders plus one self-test** `TestHarnessBuildersProduceExpectedTopology` asserting: origin is bare; invocation `git status` clean; docket-mode invocation has 3 registered worktrees (`git worktree list --porcelain` shows primary + .docket + feature); the hostile paths exist on main (verify via `gitOut(t, writer, "ls-tree", "-r", "-z", "--name-only", "main")` containing the raw bytes).
- [ ] **Step 2: Run** — `go test -count=1 ./internal/gitcli/ -run TestHarnessBuilders` → PASS.
- [ ] **Step 3: Commit** — `test(gitcli): real-repository fixture builders for both metadata topologies`

---

### Task 4: Repository discovery

**Files:**
- Create: `internal/gitcli/discover.go`
- Test: `internal/gitcli/discover_test.go`

**Interfaces:**
- Consumes: `Client.run`, Task 1 types, Task 3 harness.
- Produces:

```go
type DiscoverOptions struct{ InvocationPath string }

type Repository struct {
    PrimaryWorktree string // absolute, symlink-canonical
    CommonDir       string // absolute, symlink-canonical
}

func (c *Client) Discover(ctx context.Context, opts DiscoverOptions) (Repository, error)
```

Algorithm (ADR-0034: never treat a linked worktree's toplevel as the root):

1. Reject empty/nonexistent `InvocationPath` (`invalid-request` / `invalid-repository` for missing path); `filepath.Abs` + `filepath.EvalSymlinks` it.
2. `git -C <path> rev-parse --is-bare-repository --git-common-dir` (one process, two lines). Non-zero exit (exit 128 "not a git repository") → `invalid-repository`. `true` bare → `invalid-repository`.
3. Canonicalize the common dir (may be relative to the invocation path). Internally inconsistent output (wrong line count, empty lines) → `invalid-output`.
4. `git -C <commonDirParentQuery> worktree list --porcelain -z` run with `-C` at the invocation path; parse NUL-delimited stanzas; the **first** entry is the main worktree (git documents main-first ordering). Missing/empty first `worktree <path>` line → `invalid-output`.
5. Canonicalize the main worktree path; verify the invocation path's common dir equals the main worktree's `.git` common dir (consistency check → `invalid-repository` on mismatch, e.g. an unregistered/broken worktree).
6. Return both canonical paths. No writes of any kind.

- [ ] **Step 1: Write failing tests**

```go
// Identity matrix: for BOTH topologies, Discover from (a) primary root,
// (b) nested dir under primary, (c) .docket linked worktree (docket mode),
// (d) feature linked worktree (docket mode), (e) a path reached through an
// extra symlink to the primary — all must return the SAME canonical
// PrimaryWorktree and CommonDir (compare with require-equality; the oracle is
// filepath.EvalSymlinks of the builder's primary path + gitOut rev-parse).
func TestDiscoverCanonicalIdentityAcrossWorktrees(t *testing.T)

// Rejections: missing path, a plain non-repo temp dir, a bare repo
// (the origin itself), each → typed Failure with the exact kinds
// invalid-repository (and invalid-request for ""), asserted via AsFailure.
func TestDiscoverRejections(t *testing.T)

// Inconsistent fake output: helper-process client ("dump" replaced by a
// "script" mode emitting a bogus rev-parse answer) → invalid-output.
func TestDiscoverInconsistentIdentityOutput(t *testing.T)

// Read-only: capture the invocation repo's full `git status --porcelain -z`
// and HEAD before/after Discover; byte-identical.
func TestDiscoverIsReadOnly(t *testing.T)
```

(For `TestDiscoverInconsistentIdentityOutput` add helper mode `"script"`: emits `GITCLI_HELPER_STDOUT` verbatim and exits 0.)

- [ ] **Step 2: Run to verify failure**, **Step 3: Implement**, **Step 4: Run to verify pass** (`go test -count=1 ./internal/gitcli/ -run TestDiscover`).
- [ ] **Step 5: Commit** — `feat(gitcli): repository discovery with canonical primary-worktree identity`

---

### Task 5: Remote default branch, targeted fetch, ref resolution

**Files:**
- Create: `internal/gitcli/refs.go`
- Test: `internal/gitcli/refs_test.go`

**Interfaces:**
- Consumes: `Client.run`, `Repository`, Task 1 types.
- Produces:

```go
func (c *Client) RemoteDefaultBranch(ctx context.Context, repo Repository, remote RemoteName) (RefName, error)
// `git -C <primary> ls-remote --symref <remote> HEAD`; parse the
// "ref: refs/heads/<b>\tHEAD" line; returns refs/heads/<b>.
// No remote set-head, no cached refs/remotes/<remote>/HEAD mutation, no guessing.
// Detached/absent symref answer → ref-unavailable; unknown remote → remote-unavailable.

func (c *Client) FetchBranch(ctx context.Context, repo Repository, remote RemoteName, branch RefName) (Revision, error)
// branch MUST be fully qualified refs/heads/<b> (else invalid-request).
// `git -C <primary> fetch --no-tags --recurse-submodules=no <remote>
//    +refs/heads/<b>:refs/remotes/<remote>/<b>`
// then resolve refs/remotes/<remote>/<b> via ResolveRef and return
// Revision{Commit, Remote: remote, Ref: refs/heads/<b>}.
// Never reads FETCH_HEAD. Fetch failure → remote-unavailable when git reports
// the remote/repo missing by exit status classification below, else command-failed;
// post-fetch unresolvable tracking ref → ref-unavailable (never a silently
// accepted stale cached ref).

func (c *Client) ResolveRef(ctx context.Context, repo Repository, ref RefName) (ObjectID, error)
// `git -C <primary> rev-parse --verify --end-of-options <ref>^{commit}`
// (network=false). Exit 128 with empty stdout → ref-unavailable; success →
// validated ObjectID. Malformed stdout → invalid-output.
```

Classification (spec "Failure model" — never match stderr phrases):

- unknown remote name: probe by `git -C <primary> remote get-url <remote>` (exit 2/128 → `remote-unavailable`) **before** fetch/ls-remote; after a failed fetch with a known-configured remote, the kind is `command-failed` (covers auth + network).
- absent source branch on the remote: after a fetch exit 128, run one `ls-remote <remote> refs/heads/<b>` probe; empty successful output → `ref-unavailable`; probe itself failing → `command-failed`.

- [ ] **Step 1: Write failing tests**

```go
// RemoteDefaultBranch returns refs/heads/main from the harness origin (which
// has HEAD -> main); after `git -C origin symbolic-ref HEAD refs/heads/other`
// (created via writer push), it returns refs/heads/other — proving it asks
// the remote, not a cached local guess.
func TestRemoteDefaultBranchAsksRemote(t *testing.T)

// FetchBranch: writer advances main; FetchBranch(origin, refs/heads/main)
// returns the NEW commit (equal to writerCommit's returned id); the tracking
// ref refs/remotes/origin/main now equals it (gitOut oracle); no tags were
// fetched (gitOut "tag" list empty) even though the writer pushed a tag.
func TestFetchBranchTargetedUpdatesTrackingRef(t *testing.T)

// FetchBranch fetches ONLY the named branch: writer pushes branch "unrelated";
// after FetchBranch(main), refs/remotes/origin/unrelated does not exist.
func TestFetchBranchDoesNotFetchUnrelatedBranches(t *testing.T)

// ResolveRef found/not-found: refs/heads/main resolves; refs/heads/nope →
// Failure kind ref-unavailable with AsFailure.
func TestResolveRefFoundAndNotFound(t *testing.T)

// Typed failures: unconfigured remote name → remote-unavailable;
// fetch of an absent remote branch → ref-unavailable;
// a remote URL pointing at a nonexistent path → command-failed.
func TestRefsFailureKinds(t *testing.T)

// Ref/remote validation: unqualified "main", option-shaped "-o", pathspec-magic
// ":(top)x" as branch → invalid-request before any process starts (assert via
// helper-process client whose "dump" mode would make any spawn visible: stdout empty).
func TestRefsValidationBlocksSmuggling(t *testing.T)
```

- [ ] **Step 2: Run to verify failure**, **Step 3: Implement `refs.go`**, **Step 4: Run to verify pass** (`go test -count=1 ./internal/gitcli/ -run 'TestRemote|TestFetch|TestResolveRef|TestRefs'`).
- [ ] **Step 5: Commit** — `feat(gitcli): remote default branch, targeted fetch, ref resolution`

---

### Task 6: ObjectSource open + NUL-safe ListTree

**Files:**
- Create: `internal/gitcli/source.go`
- Test: `internal/gitcli/source_test.go`

**Interfaces:**
- Consumes: `Client.run`, `Repository`, `Revision`, Task 1 types.
- Produces:

```go
type ObjectSource interface {
    Revision() Revision
    ListTree(ctx context.Context, prefixes []RepoPath) ([]TreeEntry, error)
    ReadBlobs(ctx context.Context, paths []RepoPath) ([]BlobResult, error)
}

func (c *Client) OpenObjectSource(ctx context.Context, repo Repository, rev Revision) (ObjectSource, error)
// verifies rev.Commit exists locally AND is a commit:
// `git -C <primary> cat-file -t <commit>` → "commit" (else unexpected-object;
// missing object → ref-unavailable). The returned source stores the commit id
// by value; nothing can move it afterwards.
```

`ListTree(ctx, prefixes)`:

- `prefixes` empty slice, or containing `""`: the root listing (`validateRepoPath(p, true)`); all other entries validated strictly. Duplicate prefixes are legal input (results dedupe).
- One process: `git -C <primary> ls-tree -r -z --full-tree <commit> -- <prefix>...` (root listing passes no pathspec). `--full-tree` + `-z` gives raw-byte, NUL-delimited records `"<mode> <type> <oid>\t<path>\x00"`.
- Parse by scanning to each NUL; split the header at the first `\t`; header fields split by single spaces; validate mode ∈ {100644,100755,120000,160000}, type ∈ {blob,commit} for leaves (`-r` never emits tree entries except gitlinks as commit); `validateObjectID` each id. Any malformed record, trailing partial record, or non-UTF8-safe mishandling → `invalid-output` and **no partial result**.
- De-duplicate identical paths selected through overlapping prefixes; sort by raw path bytes (`bytes.Compare`).
- Symlinks are returned as blobs with mode 120000, never followed; gitlinks as `Type: "commit"`, mode 160000. Absent prefix contributes zero entries (not an error).

- [ ] **Step 1: Write failing tests**

```go
// Oracle equality: ListTree(nil) over the main-mode fixture equals an
// independently-parsed `gitOut ls-tree -r -z` listing — same paths (raw bytes,
// including "spa ce/né tab\tfile.md" and "line\nbreak.md"), same modes, types, ids.
func TestListTreeMatchesPlumbingOracle(t *testing.T)

// Prefix semantics: ListTree([]{"docs"}) returns only docs/** leaves;
// overlapping []{"docs", "docs/changes"} returns them once (dedup);
// absent prefix []{"nope"} returns empty, nil error; result order is raw-byte sorted.
func TestListTreePrefixesOverlapAndAbsent(t *testing.T)

// core.quotePath=true fixture: hostile paths come back as raw bytes, not
// C-quoted spellings ("\303\251"-free; assert bytes equal the original names).
func TestListTreeQuotePathTrueFixture(t *testing.T)

// Pinning: source opened at commit A still lists A's tree after the writer
// advances origin/main and a second FetchBranch lands B locally.
// (full A/B proof matrix lives in Task 8; this is the ListTree-local case)
func TestListTreePinnedAfterFetch(t *testing.T)

// Open rejections: a blob id → unexpected-object; an unknown id → ref-unavailable.
func TestOpenObjectSourceRejections(t *testing.T)

// Malformed plumbing (helper "script" mode): truncated record (no NUL),
// missing tab, bad mode "999999", short oid — each → invalid-output, empty result.
func TestListTreeMalformedOutput(t *testing.T)
```

- [ ] **Step 2: Run to verify failure**, **Step 3: Implement `source.go`**, **Step 4: Run to verify pass** (`go test -count=1 ./internal/gitcli/ -run 'TestListTree|TestOpenObjectSource'`).
- [ ] **Step 5: Commit** — `feat(gitcli): revision-pinned object source and NUL-safe tree listing`

---

### Task 7: Batch blob reads

**Files:**
- Create: `internal/gitcli/readblobs.go`
- Test: `internal/gitcli/readblobs_test.go`

**Interfaces:**
- Consumes: the `objectSource` struct from Task 6 (adds its `ReadBlobs` method), `Client.run` extended with stdin support (already in Task 2's `runRequest.stdin`).
- Produces: `ReadBlobs(ctx, paths []RepoPath) ([]BlobResult, error)` on `ObjectSource`.

Algorithm:

1. Validate every path strictly (`allowEmptyRootPrefix=false`) and reject duplicate request paths (`invalid-request`) — the whole input set validated before any work (learnings: `validate-the-whole-input-set-first`). Empty input → empty result, **no process started**.
2. Resolve paths to typed entries in ONE process: `git -C <primary> ls-tree -z <commit> -- <path>...` (non-recursive; an exact path argument lists exactly that entry). Parse as in Task 6. Paths absent from the output → `Found: false`. A path whose entry is `tree` (a directory requested as a blob) or `commit` (gitlink) → `unexpected-object` failing the entire call.
3. Feed the found entries' object ids, in request order, to ONE `git -C <primary> cat-file --batch --buffer` process via stdin (one id per line, then close stdin — never `<rev>:<path>` tokens).
4. Parse each response frame: header `"<oid> <type> <size>\n"`, then exactly `size` payload bytes, then a mandatory `"\n"`. Verify: response order matches request order, oid matches the requested id, type is `blob`, size consumed exactly. Any `missing`/mismatched/truncated/reordered/oversized frame → `invalid-output`, no partial result.
5. Each `BlobResult.Blob.Bytes` is a fresh copy owned by the result (no shared backing array with the parse buffer).

- [ ] **Step 1: Write failing tests**

```go
// Oracle: ReadBlobs over README.md, empty.txt, tool.sh, the symlink, and both
// hostile paths — bytes equal `gitOut cat-file blob <oid>` (raw), modes equal the
// ls-tree oracle (100755 for tool.sh, 120000 for the link), ids validated,
// results in REQUEST order (deliberately not tree order).
func TestReadBlobsMatchesOracleInRequestOrder(t *testing.T)

// Missing path → Found:false at its slot, other slots intact, nil error.
// Duplicate request path → invalid-request. Empty input → empty result and,
// via helper client, zero processes spawned.
func TestReadBlobsMissingDuplicateEmpty(t *testing.T)

// Symlink bytes are the stored target string ("README.md"), never the
// target file's contents; a gitlink path → unexpected-object; a directory
// path → unexpected-object.
func TestReadBlobsSymlinkGitlinkDirectory(t *testing.T)

// One batch process, not one per blob: helper "script" client counts spawns
// for a 5-path read — exactly 2 git invocations (ls-tree resolve + cat-file batch).
// Implement by pointing the helper "count" mode at a spawn-log file
// (GITCLI_HELPER_SPAWNLOG) it appends argv[0..] to; assert 2 lines.
func TestReadBlobsUsesOneBatchProcess(t *testing.T)

// Frame injection via helper "script" mode: truncated payload, wrong oid,
// wrong type ("tree"), wrong size (short and long), extra trailing frame,
// reordered frames — each → invalid-output and no partial result.
func TestReadBlobsMalformedBatchFrames(t *testing.T)

// Result ownership: mutate one result's Bytes; a second ReadBlobs of the same
// path returns the original bytes (no shared buffer).
func TestReadBlobsResultOwnership(t *testing.T)
```

- [ ] **Step 2: Run to verify failure**, **Step 3: Implement `readblobs.go`** (+ the helper "count"/"script" extensions in `exec_test.go`), **Step 4: Run to verify pass** (`go test -count=1 ./internal/gitcli/ -run TestReadBlobs`).
- [ ] **Step 5: Commit** — `feat(gitcli): batched exact-byte blob reads with strict frame verification`

---

### Task 8: Proof matrix — checkout preservation and revision consistency

**Files:**
- Create: `internal/gitcli/preserve_test.go`

**Interfaces:**
- Consumes: everything above; no new product code expected (any failure here is a product bug fixed in the task it belongs to).

- [ ] **Step 1: Write the checkout-preservation proof**

```go
// checkoutSnapshot records every property the spec names.
type checkoutSnapshot struct {
    headSymbolic string // gitOut symbolic-ref -q HEAD (or "" when detached)
    headCommit   string // gitOut rev-parse HEAD
    indexTree    string // gitOut write-tree (via a temp index copy? NO — write-tree writes objects)
                        // use: gitOut diff --cached --raw -z + gitOut ls-files -s -z (pure reads)
    statusRaw    string // gitOut status --porcelain=v2 -z --untracked-files=all
    dirtyBytes   string // os.ReadFile of the deliberately dirtied tracked file
    untracked    string // os.ReadFile of the untracked file
}

// For EACH topology and EACH invocation worktree (primary, .docket, feature):
//  1. dirty one tracked file + create one untracked file in that worktree,
//  2. snapshot,
//  3. writer advances origin/main (and, docket mode, origin/docket),
//  4. adapter: Discover → FetchBranch → OpenObjectSource → ListTree(nil) →
//     ReadBlobs(3 paths),
//  5. snapshot again → require byte-identical field-by-field,
//  6. require the tracking ref DID move (gitOut rev-parse refs/remotes/origin/main
//     equals the writer's new commit) — proving the fetch was real, not skipped.
func TestCheckoutPreservationAcrossWorktreesAndTopologies(t *testing.T)
```

(Note on `indexTree`: use only read commands — `ls-files -s -z` and `diff --cached --raw -z` — never `write-tree`, which writes objects and would contaminate the proof.)

- [ ] **Step 2: Write the revision-consistency proof**

```go
// Fetch A, open source A. Writer advances to B (changing README.md and adding
// a file). Prove source A still returns A's bytes/tree. Fetch again → revision
// B; open source B → returns B. Writer then advances an UNRELATED branch;
// prove source A and B revisions and a re-read of one blob are unchanged.
func TestRevisionConsistencyABAndUnrelatedBranch(t *testing.T)

// Docket-mode two-source composition primitive: fetch refs/heads/main and
// refs/heads/docket, open two sources, read .docket.yml from the main source
// and a planning file from the docket source; assert each source only sees its
// own branch's tree (the planning file is NOT in the main source: Found false).
func TestDocketModeTwoSourceReads(t *testing.T)
```

- [ ] **Step 3: Run** — `go test -count=1 ./internal/gitcli/ -run 'TestCheckout|TestRevision|TestDocketMode'` → PASS (fix any product bug in its owning file).
- [ ] **Step 4: Commit** — `test(gitcli): checkout-preservation and revision-consistency proof matrix`

---

### Task 9: Suite gate, mutation discipline, and budget re-measure

**Files:**
- Modify: `tests/runtime-budgets.tsv` (the `tests/test_go_toolchain.sh` row, only if re-measurement requires it)

- [ ] **Step 1: Run the four required mutation probes** (spec, "Suite integration and mutation discipline"). Each: apply the mutation, run the named test with `go test -count=1`, observe RED, revert from the saved patch (`git stash` / `git checkout -p` is forbidden as a restore of uncommitted work — save each mutation as a `git diff > /tmp` patch and `git apply -R` it; learnings: `mutation-restore-needs-a-backup-copy`), confirm GREEN again:

  1. In the fetch/source seam, replace the pinned `rev.Commit` with the moving tracking ref name (`refs/remotes/origin/main`) in one `ListTree` call path → `TestRevisionConsistencyABAndUnrelatedBranch` and `TestListTreePinnedAfterFetch` must go RED.
  2. Remove `GIT_DIR` (one repository-redirection scrub) from `sanitizeEnvironment` → `TestSanitizeRemovesRedirectionClassesKeepsAuthSentinel` must go RED.
  3. Replace the NUL-safe `ls-tree -z` parse with line-splitting on `\n` (display form) → `TestListTreeMatchesPlumbingOracle` / `TestListTreeQuotePathTrueFixture` must go RED on the hostile-path fixture.
  4. Insert a `git checkout -- .` (or `read-tree`) into the fetch path → `TestCheckoutPreservationAcrossWorktreesAndTopologies` must go RED on the dirty-file property.

  Record each probe's RED evidence (test name + failure line) in the build notes for the results file.

- [ ] **Step 2: Full package + repo Go gate** — `gofmt -l $(git ls-files '*.go' | xargs -n1 dirname | sort -u)` empty; `go vet ./...`; `go test -count=1 ./...` → all green.
- [ ] **Step 3: Re-measure the Go suite budget row** — `scripts/run-tests.sh -j 1 --timings /tmp/timings.tsv tests/test_go_toolchain.sh`; read the serial number. If the worst standalone serial reading exceeds the current 20s row's comfort, apply the table's own rule (next multiple of 5 plus a 5s margin on the worst standalone serial reading) and update `tests/runtime-budgets.tsv` with a comment noting the 0308 measurement. Record the measured number either way.
- [ ] **Step 4: Run the whole resolved suite** — `scripts/run-tests.sh` (the resolved `finalize.test_command`) from the feature worktree root; all files green; act on any trailing `OVER BUDGET:` line (it does not fail the run — nothing else will catch it).
- [ ] **Step 5: Commit** — `test(gitcli): mutation-probe evidence and Go suite budget re-measure` (only if files changed; otherwise fold the budget edit into this commit).

---

## Self-Review

- **Spec coverage:** discovery + canonicalization (T4), controlled execution/env policy/timeouts (T2), remote default/fetch/resolve + FETCH_HEAD ban (T5), pinned source + NUL-safe ListTree (T6), batch ReadBlobs + frame verification + ownership (T7), checkout preservation + A/B consistency + two-topology matrix (T8), failure kinds (T1, exercised T2/T4–T7), secret non-disclosure (T2), one-batch-process proof (T7), mutation discipline + suite/budget gate (T9). Shallow-repo tolerance needs no code (targeted fetch + object-presence checks already behave); not separately tested per spec's out-of-scope note.
- **Placeholder scan:** no TBDs; every test named with concrete assertions or an explicit oracle; helper modes enumerated.
- **Type consistency:** `Revision`/`ObjectSource`/`Failure` signatures identical across Tasks 1–8; `runRequest.stdin` introduced in T2 and consumed in T7.
