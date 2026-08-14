<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0311 — Installer, embedded assets, and four first-class harnesses](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-14-0311-installer-embedded-assets-and-four-harnesses.md)**
<!-- docket:backlink:end -->

# Installer, Embedded Assets, and Four First-Class Harnesses — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** One Go binary carries a deterministic embedded asset bundle and installs (or source-links) Docket's skills, native agent definitions, and dispatch material into Claude, Codex, Cursor, and OpenCode with journaled, ownership-safe, rollback-capable filesystem transactions, plus a read-only `install check`.

**Architecture:** A repo generator freezes the authored asset roots into a canonical manifest + embedded byte tree inside `internal/assets`. `internal/install` owns roots, ownership state (`state/install.json`), drift inspection, the journaled transaction engine, version-tree extraction, and the three operation services. `internal/harness` defines a pure planning `Adapter` interface with four child packages (`claude`, `codex`, `cursor`, `opencode`) that render native artifacts but never touch the filesystem. `internal/app`/`internal/cli` compose the operations onto the change-0304 protocol-v1 envelope.

**Tech Stack:** Go 1.26 (`go.mod` module `github.com/danielhanold/docket`), Cobra CLI, `go.yaml.in/yaml/v3`, `go:embed`, existing `internal/config` (change 0305) and `internal/document` (change 0306).

**Spec:** `.docket/docs/superpowers/specs/2026-08-13-installer-embedded-assets-and-four-harnesses-design.md` (metadata worktree, `docket` branch)

## Global Constraints

- Module path `github.com/danielhanold/docket`; Go `1.26.0`; YAML import is `go.yaml.in/yaml/v3`, never `gopkg.in/yaml.v3`.
- No new third-party dependencies without a recorded reason; prefer stdlib (`crypto/sha256`, `embed`, `encoding/json`, `io/fs`, `path`, `os`).
- Only `cmd/docket/main.go` may call `os.Exit`; commands set a package-scoped `result app.OperationResult` and the single `Presenter` writes it (see `internal/cli/root.go`).
- Result taxonomy is closed (change 0304): `applied`, `no-op`, `invalid-input`, `invalid-state`, `unsupported-config`, `external-failed`, `internal-error` are the ones this change uses. `internal/app/shadow_test.go`'s `TestEnvelopeNotShadowed` must keep passing for every new result struct.
- Stable machine reasons for this change (spec, "Operations and protocol results"): `no-harness-detected`, `ownership-conflict`, `managed-block-invalid`, `installation-required`, `installation-drift`, `transaction-recovery-required`, `asset-manifest-invalid`, `asset-protocol-mismatch`, `source-assets-drifted`.
- Tests never mutate the developer's real home: every test that touches roots injects fake `HOME`/XDG via `t.TempDir()` (pattern: `internal/config/fixtures_test.go`).
- Go test cache lies to mutation probes: every verify/mutation re-run uses `go test -count=1` (learnings: cached-runner-serves-a-mutated-tree).
- Every symlink comparison canonicalises every hop (`filepath.EvalSymlinks`) before identity checks — macOS `/tmp -> /private/tmp` makes this observable in tests (learnings: canonicalise-every-symlink-hop).
- Golden fixtures freezing generated harness output need a drift tie: adapters render from the embedded catalog, and the generator drift test ties embedded bytes to authored files, so the chain authored → embedded → golden is closed both ways (learnings: frozen-copy-needs-a-drift-assert, correspondence-guard-runs-one-way).
- Vendor agent-file schemas (Claude/Codex/Cursor/OpenCode native fields) are outside-truth. The authoritative in-repo reference is `sync-agents.sh`'s emitters (`emit_codex_toml`, `emit_cursor_md`, `emit_opencode_md`, generic `emit` for Claude) — mirror their field mappings exactly, freeze goldens, and route live-vendor confirmation to the results file as human verification for change 0317 (learnings: external-truth-needs-a-human-checkpoint).
- Release bundles contain no symlinks; generation fails on any non-regular file under an allowed root.
- `agent_harnesses` from legacy config is inert (change 0305); harness selection is flags or detection only.
- No package under this change imports Git, GitHub, board, planning, or workflow packages; `internal/assets` does not import `internal/harness`; harness adapters import asset/config value types but no filesystem implementation.
- Commit after every task; commit messages `feat(0311): …` / `test(0311): …`.

---

### Task 1: `internal/assets` — manifest model, canonical JSON, validation

**Files:**
- Create: `internal/assets/manifest.go`
- Create: `internal/assets/manifest_test.go`

**Interfaces:**
- Consumes: stdlib only.
- Produces (later tasks rely on these exact names):

```go
package assets

const ManifestFormatVersion = 1
const AssetProtocol = 1

type Manifest struct {
    FormatVersion int     `json:"format_version"`
    AssetProtocol int     `json:"asset_protocol"`
    AssetSetID    string  `json:"asset_set_id"`
    Entries       []Entry `json:"entries"`
}

type Entry struct {
    Path   string `json:"path"`   // slash-separated, relative, no ".."/"."/empty segments
    Role   Role   `json:"role"`
    Mode   uint32 `json:"mode"`   // portable policy mode: 0o644 files only in v1
    Size   int64  `json:"size"`
    SHA256 string `json:"sha256"` // lowercase hex
}

type Role string
const (
    RoleSkill           Role = "skill"            // skills/<skill>/**
    RoleAgentSource     Role = "agent-source"     // agents/docket-*.md
    RoleHarnessDefaults Role = "harness-defaults" // agents/harness-defaults.yml
    RoleDispatch        Role = "dispatch"         // cursor-rules/**
    RoleConfigSchema    Role = "config-schema"    // .docket.example.yml
)

// EncodeCanonical renders the manifest as canonical JSON: two-space indent,
// keys in struct order, entries sorted by Path, trailing newline.
func EncodeCanonical(m Manifest) ([]byte, error)
// ComputeAssetSetID digests the canonical encoding with AssetSetID forced to "".
func ComputeAssetSetID(m Manifest) (string, error) // "sha256:<hex>"
// ValidateManifest checks structure only (no payload access):
// format/protocol known, entries sorted strictly ascending by Path (no dups),
// every Path safe (no leading '/', no "..", ".", empty segment, no backslash),
// Role known, Mode == 0o644, Size >= 0, SHA256 64 lowercase hex chars,
// AssetSetID matches ComputeAssetSetID.
func ValidateManifest(m Manifest) error
```

- Validation failures return a sentinel-wrapped error: `var ErrManifestInvalid = errors.New("asset manifest invalid")`, wrapped with `%w` and a per-entry detail message.

- [ ] **Step 1: Write failing tests** in `manifest_test.go`: `TestEncodeCanonicalDeterministic` (encode twice, byte-equal; entries pre-shuffled input rejected by ValidateManifest, sorted input stable), `TestComputeAssetSetIDExcludesSelf` (two manifests differing only in AssetSetID digest equal; changing one entry byte changes the digest), `TestValidateManifestRejects` — table-driven over: unsorted entries, duplicate path, `../escape`, absolute path, empty segment, `a\\b` backslash, unknown role, mode `0o755`, short/uppercase sha, wrong AssetSetID, unknown FormatVersion, unknown AssetProtocol. `TestValidateManifestAcceptsMinimal` for a two-entry valid manifest.
- [ ] **Step 2: Run** `go test -count=1 ./internal/assets/` — expect FAIL (undefined symbols).
- [ ] **Step 3: Implement** `manifest.go` per the interface block above. Path safety helper `func safeRelPath(p string) bool` shared with later tasks (exported as `SafeRelPath` — Task 2 and Task 12 reuse it).
- [ ] **Step 4: Run** `go test -count=1 ./internal/assets/` — expect PASS. Mutation-probe once: comment out the sort check, re-run with `-count=1`, watch `TestValidateManifestRejects` redden; restore.
- [ ] **Step 5: Commit** `feat(0311): asset manifest model, canonical JSON, validation`

---

### Task 2: Bundle generator, embedded data, and the drift guard

**Files:**
- Create: `internal/assets/generate.go` (generator library: walk, build manifest, write tree)
- Create: `internal/assets/generate_test.go`
- Create: `cmd/genassets/main.go` (thin CLI over the library; `-check` mode; run via `go:generate`)
- Create: `internal/assets/embedded.go` (`//go:embed` of the generated tree + accessors)
- Create: `internal/assets/embedded/` (generated output: `manifest.json` + `tree/...` — committed)
- Create: `internal/assets/embedded_test.go`

**Interfaces:**
- Consumes: Task 1 (`Manifest`, `Entry`, `Role`, `EncodeCanonical`, `ComputeAssetSetID`, `ValidateManifest`, `SafeRelPath`).
- Produces:

```go
// generate.go
type AllowedRoot struct {
    Root string // repo-relative: "skills", "agents", "cursor-rules", ".docket.example.yml"
    Role Role
}
func DefaultAllowedRoots() []AllowedRoot
// Generate walks repoDir's allowed roots and returns the manifest plus a
// path->bytes payload map. Fails (wrapped ErrGenerate) on: symlink or other
// non-regular file, path escaping its root, collision after path
// normalization, unreadable file.
func Generate(repoDir string, roots []AllowedRoot) (Manifest, map[string][]byte, error)
// WriteTree writes manifest.json + tree/<path> under outDir (fresh dir).
func WriteTree(outDir string, m Manifest, payload map[string][]byte) error

// embedded.go
//go:embed embedded/manifest.json embedded/tree
var embeddedFS embed.FS
// EmbeddedManifest parses, validates, and verifies every payload hash/size.
// Any failure is ErrManifestInvalid (reason for the app layer: asset-manifest-invalid).
func EmbeddedManifest() (Manifest, error)
// Open returns the embedded payload for a manifest path.
func Open(path string) ([]byte, error)
// Catalog is the read view harness adapters consume (Task 7).
type Catalog struct { Manifest Manifest; open func(string) ([]byte, error) }
func EmbeddedCatalog() (Catalog, error)
func (c Catalog) EntriesByRole(r Role) []Entry
func (c Catalog) Bytes(path string) ([]byte, error)
```

- `DefaultAllowedRoots()` returns exactly: `{"skills", RoleSkill}`, `{"agents", RoleAgentSource}` (with `agents/harness-defaults.yml` re-roled to `RoleHarnessDefaults` by exact basename match inside the walk), `{"cursor-rules", RoleDispatch}`, `{".docket.example.yml", RoleConfigSchema}`. A single-file root is allowed.
- `cmd/genassets/main.go`: `genassets [-check]` run from the repo root. Default mode regenerates `internal/assets/embedded/` in a temp dir and moves it into place; `-check` regenerates into a temp dir, byte-compares against the committed tree, exits 1 with a path-listing diff on mismatch. Add `//go:generate go run ../../cmd/genassets` in `internal/assets/generate.go`.

- [ ] **Step 1: Write failing tests** in `generate_test.go` against a constructed temp repo fixture (create files with `os.WriteFile` in `t.TempDir()`): `TestGenerateDeterministic` (two Generate calls byte-equal manifests and payloads), `TestGenerateRejectsSymlink` (plant `os.Symlink` under a root), `TestGenerateRejectsEscape` (root containing a file created outside then referenced — construct with a symlinked directory), `TestGenerateRejectsCollision` (two roots mapping the same normalized path), `TestGenerateRolesAssigned` (harness-defaults.yml gets `RoleHarnessDefaults`, skill files `RoleSkill`).
- [ ] **Step 2: Run** `go test -count=1 ./internal/assets/` — FAIL.
- [ ] **Step 3: Implement** `generate.go` + `cmd/genassets/main.go`; run `go run ./cmd/genassets` from the worktree root to produce and commit `internal/assets/embedded/`.
- [ ] **Step 4: Write the drift + correspondence tests** in `embedded_test.go`, running only when the authored roots are present (skip with `t.Skip` if `../../skills` does not exist — they always exist in this repo; the guard is for future extraction):
  - `TestEmbeddedMatchesAuthored`: `Generate` from the repo root (`../..` relative to the package dir, resolved via `runtime.Caller`-free `filepath.Abs`), byte-compare manifest and every payload against `EmbeddedManifest()`/`Open` — **both directions**: every generated path exists embedded with equal bytes, and every embedded path exists in the generated set.
  - `TestEmbeddedValidates`: `EmbeddedManifest()` returns no error; every entry's Size/SHA256 match `Open` output; AssetSetID recomputes.
  - `TestEmbeddedCorruptionDetected`: construct a Manifest copy with one flipped hash and prove the verifying loop (extract the verification into `func VerifyPayloads(m Manifest, open func(string) ([]byte, error)) error` so it is testable) rejects it.
- [ ] **Step 5: Run** `go test -count=1 ./internal/assets/` — PASS. Mutation-probe the drift guard: append a byte to a file under `skills/` in the worktree, re-run `-count=1`, watch `TestEmbeddedMatchesAuthored` redden, revert the byte (restore from a backup copy you made first — `git checkout` would eat any staged work; learnings: mutation-restore-needs-a-backup-copy).
- [ ] **Step 6: Commit** `feat(0311): asset bundle generator, embedded tree, drift guard`

---

### Task 3: `internal/install` — user roots and installed-state model

**Files:**
- Create: `internal/install/roots.go`
- Create: `internal/install/roots_test.go`
- Create: `internal/install/state.go`
- Create: `internal/install/state_test.go`

**Interfaces:**
- Consumes: `assets.Manifest` field types only.
- Produces:

```go
package install

type UserRoots struct {
    Home       string // validated absolute home
    DataRoot   string // <XDG_DATA_HOME|~/.local/share>/docket
    ConfigHome string // XDG_CONFIG_HOME or ~/.config (for opencode)
    BinDir     string // XDG_BIN_HOME or ~/.local/bin (development mode)
}
// ResolveRoots reads env through the injected getenv func; production passes
// os.Getenv. Requires a non-empty absolute home (os.UserHomeDir seam injected
// as homeFn). XDG values are honored only when set AND absolute. Validates
// that each pre-existing root path is a directory (a root that does not exist
// yet is fine).
func ResolveRoots(homeFn func() (string, error), getenv func(string) string) (UserRoots, error)

func (r UserRoots) VersionsDir() string     // <DataRoot>/versions
func (r UserRoots) VersionDir(assetSetID string) string // .../versions/<sanitized-id>/assets
func (r UserRoots) TransactionsDir() string // <DataRoot>/transactions
func (r UserRoots) StatePath() string       // <DataRoot>/state/install.json

const StateFormatVersion = 1
type Mode string
const ( ModeRelease Mode = "release"; ModeDevelopment Mode = "development" )

type TargetKind string
const ( KindFile TargetKind = "file"; KindSymlink TargetKind = "symlink"; KindManagedBlock TargetKind = "managed-block" )

type TargetRecord struct {
    Path       string     `json:"path"`         // absolute
    Kind       TargetKind `json:"kind"`
    LinkTarget string     `json:"link_target,omitempty"` // canonical, symlinks only
    SHA256     string     `json:"sha256,omitempty"`      // file: whole file; managed-block: block interior
    BlockName  string     `json:"block_name,omitempty"`  // managed-block only, e.g. "dispatch"
    Role       string     `json:"role"`
}

type State struct {
    FormatVersion int            `json:"format_version"`
    ProductVersion string        `json:"product_version"`
    AssetProtocol int            `json:"asset_protocol"`
    AssetSetID    string         `json:"asset_set_id"`
    Mode          Mode           `json:"mode"`
    SourceRoot    string         `json:"source_root,omitempty"`    // development: canonical checkout
    SourceDigest  string         `json:"source_digest,omitempty"`  // development: asset-set id of the source
    Harnesses     []string       `json:"harnesses"`                // sorted
    AgentDigest   string         `json:"agent_digest"`             // sha256 of canonical JSON of resolved agent settings
    Targets       []TargetRecord `json:"targets"`                  // sorted by Path
}

func LoadState(path string) (*State, error)   // nil, nil when absent
func WriteStateAtomic(path string, s *State) error // temp file beside dest + rename; creates state/ dir 0o700
```

- [ ] **Step 1: Write failing tests**: `TestResolveRootsXDG` (set XDG_DATA_HOME absolute → used; relative → ignored; unset → `~/.local/share/docket`), `TestResolveRootsNoHome` (homeFn error → error), `TestResolveRootsExistingNonDirRoot` (plant a file where DataRoot should be → error), `TestStateRoundTrip` (WriteStateAtomic then LoadState deep-equal), `TestLoadStateAbsent` (nil, nil), `TestLoadStateMalformed` (garbage JSON → error), `TestWriteStateAtomicNoTorn` (write over an existing state; on injected rename failure the old file is intact — inject via a `renameFn` package seam defaulting to `os.Rename`).
- [ ] **Step 2: Run** `go test -count=1 ./internal/install/` — FAIL.
- [ ] **Step 3: Implement** `roots.go`, `state.go`.
- [ ] **Step 4: Run** — PASS.
- [ ] **Step 5: Commit** `feat(0311): install roots and installed-state model`

---

### Task 4: `internal/install` — target plans, inspection, and ownership proofs

**Files:**
- Create: `internal/install/target.go`
- Create: `internal/install/inspect.go`
- Create: `internal/install/inspect_test.go`

**Interfaces:**
- Consumes: Task 3 types; `internal/document` (`document.Parse`, `Document.Block`, `Block.Interior`, error kinds `KindMalformedMarker`, `KindMarkerImbalance`).
- Produces:

```go
// Target is the declarative unit a harness adapter emits and the installer applies.
type Target struct {
    Path       string     // absolute destination
    Kind       TargetKind
    Content    []byte     // KindFile: full bytes; KindManagedBlock: desired interior
    LinkTarget string     // KindSymlink: desired destination (canonicalised by planner)
    BlockName  string     // KindManagedBlock: marker name ("dispatch")
    Annotation string     // KindManagedBlock: start-marker annotation
    Role       string
}

type Disposition string
const (
    DispositionCreate    Disposition = "create"     // target absent
    DispositionNoop      Disposition = "no-op"      // present, already desired
    DispositionUpdate    Disposition = "update"     // present, owned, differs
    DispositionConflict  Disposition = "conflict"   // present, not provably ours
)

type Inspection struct {
    Target     Target
    Disposition Disposition
    Reason      string // conflict detail: "ownership-conflict" | "managed-block-invalid"
}

// LegacyReproducer reproduces a legacy user-level artifact's complete bytes
// from the frozen legacy renderer; Task 4 ships only the seam with a nil
// default (no legacy takeover in the initial matrix — recorded in the state
// as technique "prior-manifest" or "managed-block" only).
type LegacyReproducer func(t Target) ([]byte, bool)

// InspectTarget classifies one target against disk + prior state.
// prior may be nil (fresh install). Symlink identity checks canonicalise
// every hop of both sides with filepath.EvalSymlinks.
func InspectTarget(t Target, prior *State, legacy LegacyReproducer) (Inspection, error)

// PruneCandidates returns prior-state targets absent from the new plan, each
// classified: removable (identity still equals the prior record) or drifted
// (preserved; blocks the upgrade).
type Prune struct { Record TargetRecord; Removable bool }
func PruneCandidates(prior *State, plan []Target) ([]Prune, error)
```

- Ownership rules implemented exactly as the spec's three proofs: (1) byte/link identity with the prior `TargetRecord`; (2) for `KindManagedBlock`, `document`-parsed file whose named block exists with valid markers and interior hash equal to the prior record's `SHA256` (a file whose parse fails with `KindMalformedMarker`/`KindMarkerImbalance` → conflict `managed-block-invalid`, file untouched); (3) legacy reproduction through the injected seam. A managed-block target in a file that does not exist → `DispositionCreate` (the apply creates the file with only the block). A managed-block file that exists but has no block and no frontmatter constraint → `DispositionUpdate` appending the block (surrounding bytes preserved exactly); the block content comparison covers desired interior vs current interior.

- [ ] **Step 1: Write failing tests**, table-driven in a temp dir per case: absent file → create; identical file → no-op; owned-but-different (prior record hash matches disk, plan differs) → update; unknown existing file (no prior record) → conflict `ownership-conflict`; drifted owned file (prior record hash ≠ disk) → conflict; symlink pointing at the desired target through a `/tmp`-style extra hop → no-op (canonicalisation, use a symlinked parent dir in the fixture); symlink at a different canonical target → conflict; managed block present + hash match + plan differs → update; malformed marker (start marker, no end) → conflict `managed-block-invalid`; block absent in existing file → update that would append; `TestPruneCandidates` (removed-from-plan owned target removable; drifted one not).
- [ ] **Step 2: Run** `go test -count=1 ./internal/install/` — FAIL.
- [ ] **Step 3: Implement** `target.go`, `inspect.go`.
- [ ] **Step 4: Run** — PASS. Mutation-probe: disable the canonicalisation (compare raw `os.Readlink` strings), watch the extra-hop case redden; restore.
- [ ] **Step 5: Commit** `feat(0311): target inspection and ownership proofs`

---

### Task 5: `internal/install` — journaled transaction engine with rollback and recovery

**Files:**
- Create: `internal/install/txn.go`
- Create: `internal/install/txn_test.go`
- Create: `internal/install/fsops.go` (the failure-injection seam)

**Interfaces:**
- Consumes: Tasks 3–4.
- Produces:

```go
// FSOps is the mutation seam. Production is RealFS{}; tests wrap it to
// inject a failure at step N.
type FSOps interface {
    WriteFile(path string, data []byte, mode os.FileMode) error
    Rename(old, new string) error
    Symlink(target, path string) error
    Remove(path string) error
    MkdirAll(path string, mode os.FileMode) error
}
type RealFS struct{}

type Txn struct { /* unexported: dir, journal, fs */ }

// BeginTxn creates <TransactionsDir>/<txn-id>/ (0o700) containing:
//   plan.json     — ordered apply steps
//   backup/<n>    — pre-image bytes (or a link-target/absent marker) per touched path
// Journal entry per step: {Seq, Path, Kind, Action, PreImage} where PreImage
// records absent | file bytes ref | link target | managed-file bytes ref.
func BeginTxn(fs FSOps, roots UserRoots, inspections []Inspection) (*Txn, error)

// Apply executes create/update steps in plan order. Every file replacement
// writes a same-directory temp file then renames. Managed-block updates
// rewrite the whole file via document.Apply (ReplaceBlock/InsertBlock) with
// the same temp+rename. On any step error it calls Rollback and returns the
// step error wrapped in ErrApplyFailed.
func (t *Txn) Apply() error
// Rollback restores every already-applied step from the journal, newest first.
func (t *Txn) Rollback() error
// Commit publishes state (WriteStateAtomic) and then removes the journal dir.
func (t *Txn) Commit(statePath string, s *State) error

// DetectRecovery reports an unpublished journal left by an interrupted run.
func DetectRecovery(roots UserRoots) (txnID string, found bool, err error)
// Recover rolls the named journal back deterministically and removes it.
func Recover(fs FSOps, roots UserRoots, txnID string) error
```

- [ ] **Step 1: Write failing tests**: `TestTxnApplyCreatesAndUpdates` (mixed plan: new file, updated file, new symlink, managed-block append into an existing file whose surrounding bytes must survive byte-for-byte — assert with a full-file compare), `TestTxnApplyFailureRollsBack` (wrap RealFS with `failAt{n}`; for each step index n in the plan, run to failure, then assert every path's bytes/link equal the pre-image — table over n), `TestTxnNoTornFile` (failure injected between temp-write and rename leaves old complete bytes), `TestRecoveryAfterInterrupt` (BeginTxn + partially Apply with a fail, do NOT rollback — simulate process death by constructing a second Txn world; `DetectRecovery` finds it; `Recover` restores pre-images and removes the journal), `TestCommitRemovesJournal`, `TestDetectRecoveryClean` (no journal → not found).
- [ ] **Step 2: Run** — FAIL.
- [ ] **Step 3: Implement** `fsops.go`, `txn.go`.
- [ ] **Step 4: Run** `go test -count=1 ./internal/install/` — PASS.
- [ ] **Step 5: Commit** `feat(0311): journaled install transaction with rollback and recovery`

---

### Task 6: `internal/install` — immutable version-tree extraction

**Files:**
- Create: `internal/install/version.go`
- Create: `internal/install/version_test.go`

**Interfaces:**
- Consumes: Tasks 1–3 (`assets.Manifest`, `Catalog`-shaped `open` func, `UserRoots`).
- Produces:

```go
// EnsureVersionTree extracts the validated bundle to
// <DataRoot>/versions/<sanitized asset-set-id>/assets, staging under
// <DataRoot>/versions/.staging-<id>-<rand>, verifying every hash after the
// copy, chmod-ing files 0o444 and dirs 0o555, then renaming into place.
// An existing complete, byte-identical tree is reused (verified, not trusted
// by name). An existing directory that fails verification returns
// ErrVersionTreeInvalid — never adopted, never silently replaced.
func EnsureVersionTree(roots UserRoots, m assets.Manifest, open func(string) ([]byte, error)) (dir string, reused bool, err error)
```

- [ ] **Step 1: Write failing tests**: `TestEnsureVersionTreeExtractsAndVerifies` (fresh root → files present, read-only mode, second call `reused=true` with zero writes — assert by making the staged parent unwritable? simpler: count via an injected open that fails on second call and prove it is not called), `TestEnsureVersionTreeRejectsPartial` (pre-create the version dir with one file missing → `ErrVersionTreeInvalid`), `TestEnsureVersionTreeRejectsMutated` (flip a byte in an existing tree → error), `TestStagingNeverVisible` (inject a copy failure mid-extract; the final versions/<id> path does not exist).
- [ ] **Step 2: Run** — FAIL.
- [ ] **Step 3: Implement** `version.go`.
- [ ] **Step 4: Run** `go test -count=1 ./internal/install/` — PASS.
- [ ] **Step 5: Commit** `feat(0311): immutable version-tree extraction with staging and reuse`

---

### Task 7: `internal/harness` — adapter interface, detection, shared rendering inputs

**Files:**
- Create: `internal/harness/harness.go`
- Create: `internal/harness/inventory.go`
- Create: `internal/harness/inventory_test.go`

**Interfaces:**
- Consumes: `assets.Catalog` (Task 2), `install.Target`/`install.TargetKind` (Task 4), `config.AgentsTable`/`config.AgentSetting` (existing).
- Produces:

```go
package harness

type InstallMode string
const ( ModeRelease InstallMode = "release"; ModeDevelopment InstallMode = "development" )

type PlanInput struct {
    Assets    assets.Catalog
    Mode      InstallMode
    AssetsDir string // release: immutable version tree assets dir; development: canonical source root
    Roots     install.UserRoots
    Agents    config.AgentsTable // resolved built-in ⊕ global
}

type Detection struct { Present bool; Root string }

type Adapter interface {
    Name() string
    Detect(install.UserRoots) Detection
    Plan(PlanInput) ([]install.Target, error)
}

// Order is the fixed planning order.
var Order = []string{"claude", "codex", "cursor", "opencode"}

// AgentSource is one parsed agents/docket-*.md: the shared input every renderer maps.
type AgentSource struct {
    ShortName   string   // "build-standard"
    Name        string   // "docket-build-standard"
    Description string
    Skills      []string // frontmatter skills: flow list; empty for the consultant
    Body        string   // markdown body after frontmatter
}
// ParseInventory decodes every RoleAgentSource entry via internal/document
// (Parse + DecodeFrontmatter into a struct {Name, Description string; Skills []string}).
// Sorted by ShortName. An entry that fails to parse is an error, not a skip.
func ParseInventory(c assets.Catalog) ([]AgentSource, error)
// SkillDirs returns the sorted set of top-level skill directory names from
// RoleSkill entries ("docket-build", ...).
func SkillDirs(c assets.Catalog) []string
// ResolvedAgent returns the model/effort for harness+agent from the table,
// normalizing docket's no-pin sentinels: model "inherit" → "", effort "auto"
// (already "" in config.AgentSetting) stays "".
func ResolvedAgent(t config.AgentsTable, harnessName, shortName string) (model, effort string)
```

- Detection roots (read-only stat, never shelling out): claude → `~/.claude` exists as dir; codex → `~/.codex`; cursor → `~/.cursor`; opencode → `<ConfigHome>/opencode`.

- [ ] **Step 1: Write failing tests**: `TestParseInventoryFromEmbedded` (use the real `assets.EmbeddedCatalog()`; assert 16 sources, sorted, `build-standard` has `Skills == []string{"docket-build-task"}`, `brainstorm-consultant` has empty Skills), `TestSkillDirsFromEmbedded` (12 known dirs, sorted), `TestResolvedAgentSentinels` (inherit→"", populated table passthrough), `TestOrderFixed` (the four names, exact order), `TestDetect` per adapter deferred to adapter tasks.
- [ ] **Step 2: Run** `go test -count=1 ./internal/harness/` — FAIL.
- [ ] **Step 3: Implement.** The inventory count asserts derive from the catalog (`len(EntriesByRole(RoleAgentSource))`), not a hardcoded 16 — assert consistency (every source parses, names unique, prefix `docket-`), then separately pin the floor `>= 16` so an emptied inventory cannot pass vacuously.
- [ ] **Step 4: Run** — PASS.
- [ ] **Step 5: Commit** `feat(0311): harness adapter interface and shared inventory parsing`

---

### Task 8: Claude adapter (`internal/harness/claude`)

**Files:**
- Create: `internal/harness/claude/claude.go`
- Create: `internal/harness/claude/claude_test.go`
- Create: `internal/harness/claude/testdata/golden/` (one file per rendered agent under a fixed input table + the dispatch block interior)

**Interfaces:**
- Consumes: Task 7 (`harness.PlanInput`, `harness.AgentSource`, `harness.ParseInventory`, `harness.SkillDirs`, `harness.ResolvedAgent`), Task 4 (`install.Target`).
- Produces: `func New() harness.Adapter` with `Name() == "claude"`.

**Plan output (native matrix row):**
- Skills: `~/.claude/skills/<skill>` — `KindSymlink`, `LinkTarget` = `<AssetsDir>/skills/<skill>`.
- Agents: `~/.claude/agents/docket-<short>.md` — `KindFile`. Rendering mirrors `sync-agents.sh`'s generic `emit` for Claude: YAML frontmatter `name:`, `description:`, `model:` (omit when empty/"inherit"), effort key exactly as the current generated wrappers carry it (read one generated wrapper under `~/.claude/agents/` is not allowed in tests — instead mirror `emit()` in `sync-agents.sh`; its frontmatter emits `model:` and the skills preload line in the body), `skills: [..]` preserved, body verbatim.
- Dispatch: managed block `docket:dispatch` in `~/.claude/CLAUDE.md` — `KindManagedBlock`, `BlockName "dispatch"`, `Annotation "managed by docket — do not hand-edit"`, interior = `## Docket agents — dispatch, don't run inline` preamble + one bullet per agent (`- **<name>** — <description> Delegate to the \`<name>\` agent.`) + the run-gate section from the asset `cursor-rules/run-gate.md` payload. Generated from `ParseInventory` — never a second name list.

- [ ] **Step 1: Write failing golden tests**: build a fixed `PlanInput` (embedded catalog, `AssetsDir "/data/versions/sha256-x/assets"`, fake roots home `/home/u`, agents table with three cases: built-in-style pin, model-only, unpinned) and assert: `TestClaudePlanDeterministic` (two calls deep-equal, sorted by Path), `TestClaudeGoldenAgents` (each rendered agent file byte-equals `testdata/golden/<name>.md` — generate goldens by running the renderer once and eyeballing against `sync-agents.sh`'s `emit` mapping before freezing), `TestClaudeDispatchGolden` (block interior golden; contains every agent name; contains the run-gate heading; contains no `runner` or cross-harness token — assert `!strings.Contains(interior, "codex")` etc. for the other three harness names), `TestClaudeSkillLinks` (every skill dir gets a symlink target under AssetsDir), `TestClaudeDetect` (fake home with/without `.claude`), `TestClaudeEscaping` (a synthetic AgentSource with `description: a: b # c` renders parseable YAML — reparse with `document.Parse`+`DecodeFrontmatter`).
- [ ] **Step 2: Run** `go test -count=1 ./internal/harness/claude/` — FAIL.
- [ ] **Step 3: Implement** the adapter; YAML scalar quoting at the write boundary is unconditional for description/free-text (repo rule, ADR-0071); model/effort are bare opaque scalars.
- [ ] **Step 4: Freeze goldens; run** — PASS. `TestInventoryAdditionPropagates`: append a synthetic 17th AgentSource via a wrapped catalog and assert the plan grows by one agent file and one dispatch bullet without touching adapter code.
- [ ] **Step 5: Commit** `feat(0311): claude harness adapter with golden fixtures`

---

### Task 9: Codex adapter (`internal/harness/codex`)

**Files:**
- Create: `internal/harness/codex/codex.go`, `codex_test.go`, `testdata/golden/`

**Interfaces:** Consumes/produces as Task 8; `Name() == "codex"`.

**Plan output:**
- Skills: `$HOME/.agents/skills/<skill>` symlinks (NOT `~/.codex/skills` — assert this in a dedicated test).
- Agents: `~/.codex/agents/docket-<short>.toml` — `KindFile`, mirroring `emit_codex_toml`: `name = "…"`, `description = "…"` (TOML basic-string escaping: backslash, double quote), `model = "…"` omitted when empty/inherit, `model_reasoning_effort = "…"` omitted when empty/auto, `developer_instructions = """\n<skills preamble + body>\n"""` with `\\` and `"""` escaping.
- Dispatch: managed block `docket:dispatch` in `~/.codex/AGENTS.md`, same interior generator as Claude's (shared helper in `internal/harness` — extract `func DispatchInterior(sources []AgentSource, runGate []byte) string` in Task 8 and reuse it here).

- [ ] **Step 1: Failing tests**: `TestCodexGoldenAgents` (pinned/model-only/unpinned triple), `TestCodexTOMLEscaping` (description with `"` and `\`, body containing `"""`), `TestCodexSkillsRootIsAgents` (`/home/u/.agents/skills/docket-build`, and no target under `.codex/skills`), `TestCodexDispatchBlockPath` (`/home/u/.codex/AGENTS.md`), `TestCodexDetect`.
- [ ] **Step 2: Run** — FAIL. **Step 3: Implement.** **Step 4: Run** `-count=1` — PASS.
- [ ] **Step 5: Commit** `feat(0311): codex harness adapter`

---

### Task 10: Cursor adapter (`internal/harness/cursor`)

**Files:**
- Create: `internal/harness/cursor/cursor.go`, `cursor_test.go`, `testdata/golden/`

**Interfaces:** As Task 8; `Name() == "cursor"`.

**Plan output:**
- Skills: `~/.cursor/skills/<skill>` symlinks.
- Agents: `~/.cursor/agents/docket-<short>.md` — mirror `emit_cursor_md`: frontmatter `name:`, `description:`, `model: <model>[effort=<effort>]` when both resolve, `model: <model>` when model only, no model line when unpinned (an effort with no model is dropped — that docket policy carries over; record the drop as a plan-computation non-fatal note, not a WARN stream), no `readonly:`/`is_background:`; body = skills preamble + body verbatim.
- Dispatch: dedicated file `~/.cursor/rules/docket-dispatch.mdc` — `KindFile`, content assembled from the asset payloads `cursor-rules/dispatch.head.md` + every `cursor-rules/dispatch/<agent>.md` fragment in sorted order + `cursor-rules/run-gate.md`, exactly the current authored material (opaque passthrough — no re-authoring).

- [ ] **Step 1: Failing tests**: `TestCursorGoldenAgents` (both-pinned → `[effort=…]` suffix; model-only; unpinned → no model line), `TestCursorEffortWithoutModelDropped`, `TestCursorDispatchFileAssembled` (contains head, one fragment per agent source — count equality both directions against `ParseInventory`, run-gate tail), `TestCursorDetect`.
- [ ] **Step 2: Run** — FAIL. **Step 3: Implement.** **Step 4: Run** `-count=1` — PASS.
- [ ] **Step 5: Commit** `feat(0311): cursor harness adapter`

---

### Task 11: OpenCode adapter (`internal/harness/opencode`)

**Files:**
- Create: `internal/harness/opencode/opencode.go`, `opencode_test.go`, `testdata/golden/`

**Interfaces:** As Task 8; `Name() == "opencode"`.

**Plan output:**
- Skills: `<ConfigHome>/opencode/skills/<skill>` symlinks (XDG-honoring root comes from `UserRoots.ConfigHome`).
- Agents: `<ConfigHome>/opencode/agents/docket-<short>.md` — mirror `emit_opencode_md`: frontmatter `description:`, `mode: subagent`, `model:` (omit when empty/inherit), `reasoningEffort:` only when a model is present (effort-without-model dropped), NO `name:` (filename is the identifier); body = skills preamble + body.
- Dispatch: managed block `docket:dispatch` in `<ConfigHome>/opencode/AGENTS.md`, shared `DispatchInterior`.

- [ ] **Step 1: Failing tests**: `TestOpencodeGoldenAgents` (pinned → `model:` + `reasoningEffort:`; model-only → no reasoningEffort line; unpinned → neither; always `mode: subagent`, never `name:`), `TestOpencodeEffortDrop`, `TestOpencodeXDGRoot` (ConfigHome override honored in every path), `TestOpencodeDetect`, `TestOpencodeDispatchBlockPath`.
- [ ] **Step 2: Run** — FAIL. **Step 3: Implement.** **Step 4: Run** `-count=1` — PASS. Cross-adapter guard in `internal/harness/inventory_test.go`: `TestNoCrossHarnessDelegation` — for each of the four adapters' full plans under one shared PlanInput, assert no rendered agent-definition payload names another harness's binary/dispatch (`opencode run`, `codex exec`, `cursor-agent`, `claude -p` as substrings) and no plan target path sits under another harness's root.
- [ ] **Step 5: Commit** `feat(0311): opencode harness adapter and cross-harness guard`

---

### Task 12: Operation services — `Install`, `Check`, `DevelopmentInstall` in `internal/install`

**Files:**
- Create: `internal/install/service.go`
- Create: `internal/install/service_test.go`
- Create: `internal/install/devmode.go`
- Create: `internal/install/devmode_test.go`

**Interfaces:**
- Consumes: everything above plus `config.Snapshot`, `config.PreflightMutation`, `assets.Generate` (dev-mode drift check), `buildinfo.Info`.
- Produces:

```go
type Options struct {
    Roots     UserRoots
    Adapters  []harness.Adapter // fixed harness.Order
    Harnesses []string          // explicit --harness selection; nil = detect
    Catalog   assets.Catalog
    Config    *config.Snapshot  // built-in ⊕ global only
    Info      buildinfo.Info
    FS        FSOps
}
type Action struct { Op string `json:"op"`; Path string `json:"path"`; Detail string `json:"detail,omitempty"` }
type Outcome struct {
    Applied   bool
    Mode      Mode
    Harnesses []string
    AssetProtocol int
    AssetSetID    string
    StatePath     string
    Actions   []Action
    Reason    string // stable reason on failure ("" on success)
    Err       error  // classified by the app layer
}

func Install(o Options) Outcome            // release mode
func Check(o Options) Outcome              // strictly read-only
type DevOptions struct { Options; SourceRoot, BinDir string; GoRunner func(dir string, argv []string) error }
func DevelopmentInstall(o DevOptions) Outcome
```

**Behavior (Install):** (1) `config.PreflightMutation` — blocked → reason `deferred-capability-requested`, classified `unsupported-config`; (2) validate catalog (`asset-manifest-invalid`); (3) recovery detection — a pending journal in `Install` is rolled back deterministically (`Recover`) before planning, and reported as an action; in `Check` it is reason `transaction-recovery-required` without mutation; (4) harness selection — explicit names validated against `harness.Order` (unknown → invalid input), else detection; empty detection → reason `no-harness-detected`; (5) `EnsureVersionTree`; (6) plan all selected adapters in fixed order; (7) inspect every target — any conflict → reason `ownership-conflict` (or `managed-block-invalid`), no mutation at all; (8) prune candidates — drifted stale target blocks with `installation-drift`; (9) all no-op and state matches → `Applied=false` no-op success; (10) else BeginTxn/Apply/Commit with the new `State` (targets sorted, `AgentDigest` = sha256 of canonical JSON of the resolved agents table for the selected harnesses).

**Behavior (Check):** validates embedded manifest, loads state (absent → `installation-required`), verifies binary/asset compatibility (state.AssetProtocol vs `assets.AssetProtocol` → `asset-protocol-mismatch`), re-plans under current global config and inspects every target (differences → `installation-drift` with per-target actions), verifies version-tree integrity, dev-mode source digest, and journal cleanliness. Never writes — enforce by passing a `checkFS` FSOps whose every mutation method panics (test proves the panic guard is wired via a mutation probe).

**Behavior (DevelopmentInstall):** validate source root exists/dir/contains allowed roots; run `assets.Generate` on the source and byte-compare against `assets.EmbeddedCatalog()`? No — dev-mode drift is source-internal: run the generator's check (regenerate from source, compare to the source's committed `internal/assets/embedded/` tree) → mismatch reason `source-assets-drifted`; compute the source digest = `ComputeAssetSetID` of the freshly generated manifest; asset-protocol equality between the running binary and the source's generated manifest → `asset-protocol-mismatch`; build `./cmd/docket` via `GoRunner(sourceRoot, []string{"go", "build", "-o", staged, "./cmd/docket"})` (argument vector, no shell) — failure → `external-failed`; plan with `Mode: development`, `AssetsDir` = canonicalised source root (links point into the checkout), binary staged into BinDir as an ordinary owned `KindFile` target inside the same transaction.

- [ ] **Step 1: Write failing service tests** with fake homes (`t.TempDir()`), the real embedded catalog, and a stub config snapshot: `TestInstallFreshApplies` (all four fake roots pre-created → detection selects all; assert files/links/blocks exist, state published, second run → `Applied=false` no-op), `TestInstallNoHarnessDetected`, `TestInstallExplicitHarnessCreatesRoot`, `TestInstallConflictPreservesEverything` (plant an unknown file at an agent path → outcome `ownership-conflict`, byte-compare the entire fake home before/after — zero mutation), `TestInstallGlobalPinChange` (state exists; new agents table → normal update, only agent files change), `TestInstallUpgradePrunesOwned` + `TestInstallUpgradeDriftBlocks`, `TestInstallRecoversPendingJournal`, `TestCheckReadOnly` (panic-guard FS; run Check over: healthy install → no-op; missing → `installation-required`; drifted file → `installation-drift`; pending journal → `transaction-recovery-required`; wrong protocol in state → `asset-protocol-mismatch`), `TestSharedAncestorOwnership` (codex `$HOME/.agents` and a sibling harness root under one home: neither plan claims the other's directory — assert prune candidates never name another harness's paths), `TestRepoLayerNeverLoaded` (Options carry only a built-in⊕global snapshot by construction; assert `service.go` has no import of `config.LoadFilesystemSources` with repo paths — structurally: the service takes a Snapshot, never paths; the test asserts a `.docket.yml` planted in CWD changes nothing).
- [ ] **Step 2: Failing dev-mode tests**: `TestDevInstallLinksToSource` (links canonicalise into the checkout, wrappers rendered from source templates), `TestDevInstallDriftRefuses` (edit a source skill byte without regenerating → `source-assets-drifted`, zero mutation), `TestDevInstallBuildFailureNoPublish` (GoRunner returns error → `external-failed`, no binary, no links, no state), `TestDevInstallBuildsViaArgv` (capture the argv; assert no shell string), `TestDevInstallMissingSource` (invalid input), `TestDevInstallRecordsSourceDigest`.
- [ ] **Step 3: Run** — FAIL. **Step 4: Implement** `service.go`, `devmode.go`. **Step 5: Run** `go test -count=1 ./internal/install/...` — PASS.
- [ ] **Step 6: Commit** `feat(0311): install, check, and development-install services`

---

### Task 13: App results + CLI commands + asset-dependence guard

**Files:**
- Create: `internal/app/install.go`
- Create: `internal/app/install_test.go`
- Modify: `internal/cli/root.go` (add `install`, `install check`, `development install`; add the asset-dependence default guard)
- Modify: `internal/cli/root_test.go`
- Modify: `cmd/docket/main_test.go` (end-to-end: JSON golden for `docket install check` on an empty fake home; help text)

**Interfaces:**
- Consumes: Task 12 (`install.Install/Check/DevelopmentInstall`, `Outcome`), existing `app.Envelope`, `app.CLIError`, `cli.Presenter`.
- Produces:

```go
// internal/app/install.go
type InstallResult struct {
    app.Envelope
    Mode          string            `json:"mode"`
    Harnesses     []string          `json:"harnesses"`
    AssetProtocol int               `json:"asset_protocol"`
    AssetSetID    string            `json:"asset_set_id"`
    StatePath     string            `json:"state_path"`
    AppliedWork   bool              `json:"applied_work"`
    Actions       []install.Action  `json:"actions"`
    Reason        string            `json:"reason,omitempty"`
    Message       string            `json:"message,omitempty"`
}
func RunInstall(o install.Options) InstallResult              // operation "install"
func RunInstallCheck(o install.Options) InstallResult          // operation "install.check"
func RunDevelopmentInstall(o install.DevOptions) InstallResult // operation "development.install"
func (r InstallResult) HumanText() string
```

- Result classification from `Outcome`: success+applied → `ResultApplied`; success+no work → `ResultNoOp`; reasons `no-harness-detected`, `ownership-conflict`, `managed-block-invalid`, `installation-drift`, `asset-protocol-mismatch`, `source-assets-drifted`, `transaction-recovery-required` (in check) → `ResultInvalidState`; `installation-required` (check) → `ResultInvalidState`; deferred capability → `ResultUnsupportedConfig`; bad args/unsafe path → `ResultInvalidInput`; fs/go-tool failure → `ResultExternalFailed`.
- CLI: `docket install [--harness <name>]...`, `docket install check`, `docket development install --source <dir> [--bin-dir <dir>] [--harness <name>]...`. Global config loading: `config.LoadFilesystemSources` with an empty/ignored repo layer — construct the snapshot from the **global source only** plus built-ins (`config.Resolve` with just the global Source; `RepoDir` never consulted). `--repo-dir` is NOT a flag on these commands.
- **Asset-dependence guard:** add to `internal/cli` a root-level gate: a command is asset-dependent unless registered in the explicit asset-independent set — `version`, `diagnostic runtime`, `diagnostic config`, `install`, `install check`, `development install`, help/completion. Implement as a `PersistentPreRunE` on the root that looks up the command path in `assetIndependent map[string]bool`; asset-dependent commands (none exist yet — the guard is exercised by a hidden test-only command registered in tests) load state and refuse with `installation-required` / `asset-protocol-mismatch`. The guard function is exported for tests: `func RequireCompatibleInstallation(roots install.UserRoots) error`.

- [ ] **Step 1: Write failing app tests**: `TestInstallResultClassification` (table: Outcome → Result), `TestInstallResultEnvelopeNotShadowed` (extend the existing reflective shadow test's type list), `TestHumanTextSummaries`.
- [ ] **Step 2: Write failing CLI/e2e tests**: `TestInstallCheckJSONGolden` in `cmd/docket/main_test.go` (fake `HOME` via env; `docket install check --json` on a machine with no installation → one JSON document, `"result":"invalid-state"`, `"reason":"installation-required"`), `TestInstallCommandsRegistered` (help lists them; `--json` + help conflict path preserved), `TestAssetIndependentSetExact` (walk the Cobra tree; every command is either in the independent set or gated — the correspondence runs both directions), `TestAssetDependentRefusal` (register a scratch gated command in-test; with no installation it returns `installation-required` before its RunE body executes).
- [ ] **Step 3: Run** — FAIL. **Step 4: Implement.** **Step 5: Run** `go test -count=1 ./...` — PASS, plus `gofmt -l` clean and `go vet ./...` clean (the shell gate `tests/test_go_toolchain.sh` will enforce; run it once here: `bash tests/test_go_toolchain.sh`).
- [ ] **Step 6: Commit** `feat(0311): install operations wired to protocol CLI with asset-dependence guard`

---

### Task 14: Repo integration — drift gate in the shell suite, budgets, generated-tree hygiene

**Files:**
- Create: `tests/test_asset_bundle_drift.sh`
- Modify: `tests/runtime-budgets.tsv` (one row for the new file)
- Modify: `.gitattributes` (mark `internal/assets/embedded/**` as `linguist-generated=true -diff` if the repo marks generated trees; follow the existing pattern in the file — if none exists for generated trees, add `internal/assets/embedded/** linguist-generated=true`)

**Interfaces:**
- Consumes: `cmd/genassets -check` (Task 2).
- Produces: a suite-visible drift gate independent of the Go test cache.

- [ ] **Step 1: Write the failing shell test** `tests/test_asset_bundle_drift.sh` following the house pattern (canonical `assert()` helper byte-identical to the suite's; source nothing from docket scripts): assert 1 — `go run ./cmd/genassets -check` exits 0 from the repo root (uses the same `GOMODCACHE`/`GOCACHE` pinning pattern as `tests/test_go_toolchain.sh`); assert 2 — mutation probe in a **copied** temp repo (`cp -R` the allowed roots + `internal/assets` + `cmd/genassets` + `go.mod`/`go.sum` into `$TMPDIR`, append a byte to a skill file, run `-check`, expect exit 1). Keep runtime under budget by scoping the copy to the needed paths only.
- [ ] **Step 2: Add the budget row** to `tests/runtime-budgets.tsv` (measure locally with `time bash tests/test_asset_bundle_drift.sh`, set budget with the suite's usual headroom convention — read two neighboring rows for the multiplier idiom).
- [ ] **Step 3: Run** `bash tests/test_asset_bundle_drift.sh` — PASS; `bash tests/test_runtime_budgets.sh` — PASS (row present).
- [ ] **Step 4: Commit** `test(0311): asset-bundle drift gate in the shell suite`

---

### Task 15: Self-review sweep against the spec's test matrix

**Files:**
- Modify: any package where a matrix row below lacks a named test.

The spec's "Validation and testing" section is the acceptance checklist. Walk each bullet and point at a test by name; add the missing ones. Expected residuals already planned above are marked ✓:

- [ ] Generated asset tests: determinism ✓ (T2), two-way correspondence ✓ (T2), unsafe/dup/non-regular/changed-payload/digest-corruption ✓ (T1/T2), source-or-generated-entry removal reddens ✓ (T2 mutation probe + T14 assert 2), runtime validation of bytes/modes/sizes/hashes/set ✓ (T2 `TestEmbeddedValidates`).
- [ ] Harness goldens: four adapters deterministic ✓, native goldens ✓, pin/override/auto/escaping ✓, dispatch passthrough + no cross-harness ✓ (T11 `TestNoCrossHarnessDelegation` + per-adapter dispatch asserts), codex `$HOME/.agents/skills` ✓, inventory-addition propagation ✓ (T8; add the same wrapped-catalog probe for the other three adapters if absent).
- [ ] Filesystem/ownership: fake homes ✓, unrelated bytes survive ✓ (T5/T12), the nine distinct ownership cases ✓ (T4/T12; verify each is a named test, add any missing — especially alternate symlink spelling and legacy takeover: the legacy seam ships nil, so add `TestLegacySeamReproducible`/`NonReproducible` against a stub reproducer in T4's file), per-step failure injection ✓ (T5 table), restart recovery ✓ (T5), no partial version tree ✓ (T6), no torn file ✓ (T5), shared-ancestor ✓ (T12), repo-local absence ✓ (T12).
- [ ] Mode/compat: release links to version tree ✓ (T12), dev rejections ✓ (T12), argv build + no-publish-on-fail ✓ (T12), mismatch blocks default command while the independent set works ✓ (T13), global config affects output/repo layers never loaded/unsupported capability blocks ✓ (T12/T13), `install check` write-free including with journal present ✓ (T12 panic-guard FS).
- [ ] Run the whole Go suite once: `go test -count=1 ./...` green; `gofmt -l` empty; `go vet ./...` clean.
- [ ] **Commit** `test(0311): close spec test-matrix gaps`

---

## Human verification (route to the results file, change 0317 carries the live half)

- Live vendor confirmation that Claude/Codex/Cursor/OpenCode each load the installed user-level agents and skills in a fresh session (a process-start artifact cannot be certified by fixtures — learnings: generated-artifact-loaded-at-process-start).
- The native field schemas frozen in the goldens (`model_reasoning_effort`, `[effort=…]`, `reasoningEffort`, `mode: subagent`) mirror `sync-agents.sh`'s production emitters, which were live-verified at changes 0135/0168/0192 — re-verification against current vendor releases is the 0317 acceptance item.
- Release-archive installation and download/checksum validation: change 0317.
