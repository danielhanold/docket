<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0310 — Read-only status and health vertical slice](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-16-0310-read-only-status-and-health-vertical-slice.md)**
<!-- docket:backlink:end -->

# Read-Only Status and Health Vertical Slice — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add one `docket status` command (change 0310) that opens an existing repository from pinned authoritative Git objects and reports backlog state, readiness, selection, stack context, artifact integrity, and deterministic health as protocol-v1 JSON and human text — without mutating anything.

**Architecture:** One application operation (`internal/app`) orchestrates the landed packages — `gitcli` discovery/pinned object sources, `config` resolution, `document` parsing, `repository` snapshot building, `domain` readiness/stack/selection — into one `StatusResult` DTO that both presenters consume. The CLI (`internal/cli`) registers `docket status` as an asset-independent Cobra command that only parses flags and renders. A Git-backed reader implements narrow injected seams so application tests run on fakes and Git integration tests run on real temp repos.

**Tech Stack:** Go (module `github.com/danielhanold/docket`), Cobra, the repo's bash test runner (`scripts/run-tests.sh`), real-git temp fixtures, a frozen `v0.9.3` semantic corpus.

**Spec:** `docs/superpowers/specs/2026-08-15-read-only-status-and-health-vertical-slice-design.md` (synchronized metadata-tree copy; the change file is `docs/changes/active/0310-read-only-status-and-health-vertical-slice.md` on the `docket` branch).

## Global Constraints

- **Compose, never duplicate.** The slice is a client of `internal/gitcli`, `config`, `document`, `repository`, `domain`, `app`, `cli`. No new parsers, validators, graph walks, priority ordering, or Git porcelain. `SelectQueue`, `EvaluateReadiness`, `ResolveEffectiveBase`, `BuildSnapshot` are called, not reimplemented.
- **Read-only.** The only permitted local mutation is targeted `FetchBranch` updates to the object database and remote-tracking refs. No checkout, index, `HEAD`, branch, or document writes.
- **Health ≠ failure.** Repository defects stay `findings` under `result: applied`, exit 0. The operation fails only for: `invalid-input` (bad args/path/topology-blocking config), `external-failed` (missing refs, remote/git failures), `interrupted` (cancellation), `internal-error` (contract violation). Failures emit exactly one protocol document and never a partial `StatusResult`.
- **Filters project, health doesn't shrink.** `--type`/`--priority` narrow the displayed changes and the ready queue derived from them; the corpus load and every health finding always cover the complete repository.
- **Determinism.** Changes/records in repository identity order (numeric change ID ascending; records by entity kind then identity), ready IDs in selector order, findings in landed-report order followed by status-read checks. Collections marshal as `[]`, never `null`. No host-absolute paths in protocol context.
- **Protocol v1.** New fields are additive; embed `app.Envelope`; register the new result struct in `TestEnvelopeNotShadowed` (`internal/app/shadow_test.go`).
- **TDD, and mutation probes run with `go test -count=1`** — Go's cache serves stale passes against a mutated tree otherwise (learnings: `cached-runner-serves-a-mutated-tree`).
- **Build gate = the whole suite** via `scripts/run-tests.sh`, never just the tasks' own tests.
- **v0.9.3 fixture tree has two owners.** `testdata/repositories/v0.9.3/` already holds change 0324's agent-defaults sidecar (`agents-harness-defaults.yml` + `PROVENANCE.md` naming commit `a4d72613`). The new frozen semantic corpus is derived from the `v0.9.3` tag's peeled commit `dd742abd5e9fcdf8ffe78eb6f36a293410873bbf` and coexists with — never relabels or absorbs — the sidecar (learnings: `shared-resource-keeps-first-owner-assumptions`).

## File Structure

- `internal/app/status_result.go` — `StatusResult` DTO and its sections (context, summary, changes, ready, records, findings); JSON contract.
- `internal/app/status_result_test.go` — marshalling/ordering/empty-array tests; envelope registration.
- `internal/app/status.go` — `StatusOptions`, reader seams (`StatusReader` interface set), `Status(ctx, deps, opts)` orchestration, finding normalization, failure mapping.
- `internal/app/status_test.go` — fake-reader application tests.
- `internal/app/status_git.go` — `NewGitStatusReader(*gitcli.Client)`: discovery, pinning, corpus reads, branch facts, artifact checks.
- `internal/app/status_git_test.go` — real-git integration tests (temp repos + bare remotes, read-only before/after).
- `internal/app/status_human.go` + `internal/app/status_human_test.go` — `HumanText()` and golden reports.
- `internal/cli/root.go`, `internal/cli/install.go`, `internal/cli/root_test.go` — command registration, `assetIndependent["status"]`, CLI/protocol tests.
- `testdata/repositories/v0.9.3/status-corpus/` — frozen semantic corpus + its own `PROVENANCE.md`; `testdata/repositories/v0.9.3/PROVENANCE.md` amended to scope its "whole tree" claim.
- `internal/app/status_corpus_test.go` — corpus-driven semantic assertions.
- `tests/runtime-budgets.tsv`, possibly a `tests/test_go_race_*.sh` shard — budget accounting (Task 7).

---

### Task 1: StatusResult DTO and JSON protocol contract

**Files:**
- Create: `internal/app/status_result.go`
- Test: `internal/app/status_result_test.go`
- Modify: `internal/app/shadow_test.go` (add the status case to the result table in `TestEnvelopeNotShadowed`)

**Interfaces:**
- Consumes: `app.Envelope`, `app.NewEnvelope`, `app.Result`, `config.Diagnostic`, `domain.Finding` (only for doc reference — the DTO carries its own normalized finding type).
- Produces (later tasks rely on these exact names):

```go
const OperationStatus = "status"

// StatusContext: mode, branch names, exact authoritative revisions. No absolute paths.
type StatusContext struct {
	MetadataMode          string `json:"metadata_mode"`           // "main" | "docket"
	DefaultBranch         string `json:"default_branch"`          // e.g. "main"
	DefaultBranchRevision string `json:"default_branch_revision"` // full object id
	IntegrationBranch     string `json:"integration_branch"`
	IntegrationRevision   string `json:"integration_revision"`
	MetadataBranch        string `json:"metadata_branch,omitempty"`          // docket mode only
	MetadataRevision      string `json:"metadata_revision,omitempty"`
}

type StatusSummary struct {
	TotalChanges     int `json:"total_changes"`     // complete corpus: active + archived
	ActiveChanges    int `json:"active_changes"`    // complete active set
	DisplayedChanges int `json:"displayed_changes"` // after --type/--priority projection
	ReadyChanges     int `json:"ready_changes"`     // len of Ready
	ADRs             int `json:"adrs"`
	Learnings        int `json:"learnings"`
	ErrorFindings    int `json:"error_findings"`
	WarningFindings  int `json:"warning_findings"`
}

type StatusChange struct {
	ID           int      `json:"id"`
	Slug         string   `json:"slug"`
	Title        string   `json:"title"`
	Status       string   `json:"status"`   // stored spelling
	Priority     string   `json:"priority"` // stored spelling
	Type         string   `json:"type"`
	Location     string   `json:"location"` // "active" | "archive"
	Path         string   `json:"path"`     // repo-relative record path
	Version      string   `json:"version"`  // blob object id
	Readiness    string   `json:"readiness"`        // domain Readiness kind's named string
	ReadinessWhy string   `json:"readiness_reason"` // explanatory, not parseable
	UnmetDeps    []int    `json:"unmet_dependencies"`
	StackParent  int      `json:"stack_parent,omitempty"`
	EffectiveBase string  `json:"effective_base,omitempty"` // resolved base branch name
	Ready        bool     `json:"ready"` // member of the ordered ready queue
}

type StatusRecord struct {
	Kind     string `json:"kind"`     // "change" | "adr" | "learning"
	Identity string `json:"identity"` // "0310", adr id, or learning slug
	Location string `json:"location"` // named domain location string
	Path     string `json:"path"`
	Version  string `json:"version"` // blob object id
}

type StatusFinding struct {
	Code     string   `json:"code"`
	Severity string   `json:"severity"` // "error" | "warning" | "notice"
	Entity   string   `json:"entity_kind,omitempty"`
	Identity string   `json:"entity_identity,omitempty"`
	Field    string   `json:"field,omitempty"`
	Path     string   `json:"path,omitempty"`
	Related  []string `json:"related,omitempty"`
	Message  string   `json:"message"`          // explanatory prose
	Remedy   string   `json:"remedy,omitempty"` // must be valid for the exact reported state
}

type StatusResult struct {
	Envelope
	Context  StatusContext   `json:"context"`
	Summary  StatusSummary   `json:"summary"`
	Changes  []StatusChange  `json:"changes"`
	Ready    []int           `json:"ready"`
	Records  []StatusRecord  `json:"records"`
	Findings []StatusFinding `json:"findings"`
	Reason   string          `json:"reason,omitempty"`  // failure results only
	Message  string          `json:"message,omitempty"`
}
```

Also produce the constructor later tasks call to guarantee non-nil collections:

```go
// NewStatusResult stamps the envelope and normalizes nil collections to empty
// slices so the four arrays marshal as [] on every path, including failures.
func NewStatusResult(result Result, r StatusResult) StatusResult {
	r.Envelope = NewEnvelope(OperationStatus, result)
	if r.Changes == nil { r.Changes = []StatusChange{} }
	if r.Ready == nil { r.Ready = []int{} }
	if r.Records == nil { r.Records = []StatusRecord{} }
	if r.Findings == nil { r.Findings = []StatusFinding{} }
	return r
}
```

`HumanText()` is implemented in Task 4; do **not** declare or dispatch `StatusResult` through `OperationResult` in this task (nothing returns it yet, so the interface obligation does not bind until Task 4 supplies the method).

- [ ] **Step 1: Write the failing tests** in `internal/app/status_result_test.go`:

```go
func TestStatusResultEmptyCollectionsMarshalAsArrays(t *testing.T) {
	r := NewStatusResult(ResultApplied, StatusResult{})
	buf, err := json.Marshal(r)
	if err != nil { t.Fatal(err) }
	s := string(buf)
	for _, want := range []string{`"changes":[]`, `"ready":[]`, `"records":[]`, `"findings":[]`} {
		if !strings.Contains(s, want) { t.Errorf("marshalled document missing %s: %s", want, s) }
	}
	if strings.Contains(s, "null") { t.Errorf("null leaked into protocol document: %s", s) }
}

func TestStatusResultEnvelope(t *testing.T) {
	r := NewStatusResult(ResultApplied, StatusResult{})
	env := r.Env()
	if env.ProtocolVersion != ProtocolVersion || env.Operation != "status" || env.Result != ResultApplied {
		t.Errorf("unexpected envelope: %+v", env)
	}
}

func TestStatusResultFailureShapeCarriesNoPartialReport(t *testing.T) {
	r := NewStatusResult(ResultExternalFailed, StatusResult{Reason: "unreachable-ref", Message: "boom"})
	if len(r.Changes)+len(r.Ready)+len(r.Records) != 0 {
		t.Errorf("failure result carried report sections: %+v", r)
	}
}
```

Also extend `TestEnvelopeNotShadowed`'s table in `internal/app/shadow_test.go` with one case: `{"status", NewStatusResult(ResultApplied, StatusResult{})}` (match the table's existing entry shape exactly — read the surrounding entries first).

- [ ] **Step 2: Run tests to verify they fail.** Run: `go test -count=1 ./internal/app/ -run 'TestStatusResult|TestEnvelopeNotShadowed'`. Expected: compile FAIL (`StatusResult` undefined).
- [ ] **Step 3: Implement** `internal/app/status_result.go` with exactly the types and constructor above (plus package doc comments in the house style — read `internal/app/config.go` first and match its commenting register).
- [ ] **Step 4: Run tests to verify they pass.** Same command. Expected: PASS.
- [ ] **Step 5: Commit.**

```bash
git add internal/app/status_result.go internal/app/status_result_test.go internal/app/shadow_test.go
git commit -m "feat(status): StatusResult protocol-v1 DTO with empty-array and envelope contract"
```

---

### Task 2: Status operation — seams, orchestration, finding normalization, failure mapping

**Files:**
- Create: `internal/app/status.go`
- Test: `internal/app/status_test.go`

**Interfaces:**
- Consumes: Task 1's types; `config.Resolve`, `config.Source`, `config.ResolveContext`, `config.Snapshot`, sentinels `config.ErrInvalidConfig` / `config.ErrMissingResolutionContext`; `document.Parse` (read `internal/document` for the exact parse entry point before coding); `repository.BuildSnapshot`, `repository.BuildInput`, `repository.InputDocument`, `repository.KindChange/KindADR/KindLearning`, `repository.LocationActive/LocationArchive/LocationLedger`; `domain.EvaluateReadiness`, `domain.SelectQueue`, `domain.SelectionFilter`, `domain.ResolveEffectiveBase`, `domain.StackParent`, `domain.NewBranchFacts`, `domain.ValidationReport`.
- Produces:

```go
// StatusOptions are the operation's arguments, already CLI-validated.
type StatusOptions struct {
	RepoDir    string   // invocation directory; "" = cwd
	Types      []string // repeatable --type values (validated against configured change_types)
	Priorities []string // repeatable --priority values (closed domain priority spellings)
}

// StatusReader is the seam between orchestration and Git. One call per
// concern; the Git-backed implementation arrives in Task 3.
type StatusReader interface {
	// PinContext discovers the repo, resolves origin's default branch, reads
	// .docket.yml from the pinned default-branch source, resolves configuration
	// (with global + repository-local filesystem layers), fetches and pins the
	// branches the resolved metadata mode requires, and returns everything
	// pinned once. Errors: ErrStatusInvalidInput / ErrStatusExternal wrapped.
	PinContext(ctx context.Context, repoDir string) (StatusPin, error)
	// ReadCorpus lists and reads every configured record (active + archived
	// changes, ADRs, learnings when enabled) from the pinned metadata source.
	ReadCorpus(ctx context.Context, pin StatusPin) ([]StatusBlob, error)
	// BranchFacts fetches/resolves only the distinct feature branches named by
	// live stack relationships and reports which exist on the remote.
	BranchFacts(ctx context.Context, pin StatusPin, branches []string) (domain.BranchFacts, error)
	// ArtifactExists reports whether a repo-relative path exists on the named
	// pinned source ("metadata" for specs, "integration" for plans/results).
	ArtifactExists(ctx context.Context, pin StatusPin, source string, path string) (bool, error)
}

type StatusPin struct {
	Mode                string // "main" | "docket"
	DefaultBranch       string
	DefaultRevision     string
	IntegrationBranch   string
	IntegrationRevision string
	MetadataBranch      string // "" in main mode
	MetadataRevision    string
	Config              config.Snapshot
	ConfigDiags         []config.Diagnostic
}

type StatusBlob struct {
	Kind     repository.RecordKind
	Location repository.RecordLocation
	Path     string // repo-relative
	Version  string // blob object id
	Data     []byte
}

// Sentinel classification errors the reader wraps its failures in.
var (
	ErrStatusInvalidInput = errors.New("status: invalid input")
	ErrStatusExternal     = errors.New("status: external failure")
)

// Status runs the whole read and returns the one protocol document.
func Status(ctx context.Context, reader StatusReader, opts StatusOptions) StatusResult
```

**Orchestration order inside `Status` (spec §Authoritative read algorithm):**
1. `reader.PinContext` — map errors: `errors.Is(err, ErrStatusInvalidInput)` → `ResultInvalidInput` reason `"invalid-input"`; `ErrStatusExternal` → `ResultExternalFailed` reason `"external-failed"`; `ctx.Err() != nil` → `ResultInterrupted` reason `"interrupted"`; anything else → `ResultInternalError` reason `"internal-error"`. Every failure returns `NewStatusResult(result, StatusResult{Context: <whatever pinned>, Reason: ..., Message: err.Error()})` — no partial changes/records.
2. `reader.ReadCorpus`, then parse each blob with `internal/document`; a blob whose parse or decode fails becomes a `StatusFinding` (severity `"error"`, code from the typed error) while the rest continue — one bad record never aborts the read.
3. `repository.BuildSnapshot(BuildInput{Config: pin.Config.Effective, Documents: parsed})`; keep the `ValidationReport` findings in landed order.
4. Collect the distinct feature branches referenced by live stack relationships (walk snapshot changes; for each with a declared stack parent, the parent's `branch:` — read `domain.ResolveEffectiveBase`'s doc comment first and feed exactly the branch names it consults); call `reader.BranchFacts` once.
5. Per active change: `EvaluateReadiness`, unmet dependency IDs, `StackParent` + `ResolveEffectiveBase`, record path/version, filter projection; `SelectQueue(snapshot, facts, domain.SelectionFilter{Types: opts.Types, Priorities: parsedPriorities})` for `Ready`.
6. Artifact checks: for each active change, non-empty `spec:` checked via `ArtifactExists(pin, "metadata", path)`, non-empty `plan:`/`results:` via `ArtifactExists(pin, "integration", path)`. Missing target → `StatusFinding` code `"artifact-missing"` severity `"error"` with `Field` naming which link; an **empty** link produces no finding (distinct states, per spec). An `ArtifactExists` error → operation failure per the same mapping as step 1.
7. Assemble `StatusResult`: findings = config diagnostics (normalized) → parse/decode findings → validation-report findings → artifact/status-read findings, in that fixed order; counts computed from the assembled arrays so summary and body can never disagree.

- [ ] **Step 1: Write the failing tests** in `internal/app/status_test.go` around a scriptable fake:

```go
type fakeReader struct {
	pin        StatusPin
	pinErr     error
	corpus     []StatusBlob
	corpusErr  error
	facts      domain.BranchFacts
	factsErr   error
	artifacts  map[string]bool // "source|path" -> exists
	artifactErr error
	branchAsks [][]string      // records BranchFacts calls
	artifactAsks []string      // records ArtifactExists calls
}
```

Cover, at minimum, one test each (real record bytes in fixtures — reuse the frontmatter shapes in `internal/repository`'s own tests as templates; give every list PLURALITY, ≥2 changes/ADRs, per learnings `green-suite-untested-branch`):
- **Both modes:** a docket-mode pin (metadata branch set) reads specs against the metadata source and plans against the integration source; a main-mode pin reads both against the integration/default source. Assert via `artifactAsks` that the source names actually diverge in docket mode — this is the artifact-source-distinction probe target.
- **Filters cannot suppress health:** corpus with one malformed record plus changes of two types; filter to one type; assert `Summary.DisplayedChanges` shrank but the malformed record's finding is still present and `Summary.ErrorFindings` unchanged versus the unfiltered run. Make the crossed case real: the filtered-out change is the unhealthy one.
- **Partial damage:** one unparseable blob → finding, remaining changes still evaluated and present.
- **Failure mapping:** table test over `pinErr` = wrapped `ErrStatusInvalidInput` / `ErrStatusExternal` / plain error → `ResultInvalidInput` / `ResultExternalFailed` / `ResultInternalError`, each with empty report arrays; a canceled context (cancel before call, fake returns `ctx.Err()`) → `ResultInterrupted`.
- **Pinned-revision consistency:** the result's `Context` echoes the pin verbatim; the fake's corpus/artifact calls all receive the same `StatusPin` value.
- **Stable ordering:** shuffled corpus input still yields changes by numeric ID ascending, records by kind-then-identity, ready in selector order; assert two runs marshal byte-identically.
- **Empty states are explicit:** empty ready queue and zero matching filtered changes still produce `applied` with empty arrays.

- [ ] **Step 2: Run to verify failure.** `go test -count=1 ./internal/app/ -run TestStatus` — compile FAIL (`Status` undefined).
- [ ] **Step 3: Implement `internal/app/status.go`** per the orchestration order above. Keep every policy question in landed packages: no priority ordering, readiness re-derivation, or graph walking here — translation only.
- [ ] **Step 4: Run to verify pass.** `go test -count=1 ./internal/app/`. Expected: PASS (whole package — the Task 1 tests must stay green).
- [ ] **Step 5: Commit.**

```bash
git add internal/app/status.go internal/app/status_test.go
git commit -m "feat(status): status operation orchestration over injected reader seams"
```

---

### Task 3: Git-backed StatusReader

**Files:**
- Create: `internal/app/status_git.go`
- Test: `internal/app/status_git_test.go`

**Interfaces:**
- Consumes: Task 2's `StatusReader`/`StatusPin`/`StatusBlob` and sentinels; `gitcli.NewClient`, `Client.Discover`, `Client.RemoteDefaultBranch`, `Client.FetchBranch`, `Client.OpenObjectSource`, `ObjectSource.Revision/ListTree/ReadBlobs`; `config.LoadFilesystemSources` mechanics (but see below — the repository layer comes from Git, so assemble `[]config.Source` by hand); `config.Resolve`.
- Produces:

```go
// NewGitStatusReader returns the production StatusReader over one gitcli
// client. It caches nothing across calls except the sources pinned in the
// StatusPin the caller threads back in.
func NewGitStatusReader(client *gitcli.Client) StatusReader
```

**Implementation notes (spec §1–3, verified against the landed APIs):**
- `PinContext`: `Discover({InvocationPath: repoDir})` → `RemoteDefaultBranch(repo, "origin")` → `FetchBranch(repo, "origin", defaultBranch)` → `OpenObjectSource` pinned at that revision → `ReadBlobs` for `.docket.yml`. Build the layer stack: `config.Source{Layer: LayerRepository, Name: ".docket.yml", Data: blobBytes}` (absent blob = absent layer), plus the **filesystem** global layer and the repository-local layer read relative to the discovered primary worktree (mirror `LoadFilesystemSources`'s candidate paths; do not call it wholesale — its repo `.docket.yml` read would come from the working tree, which the spec forbids). `config.Resolve(sources, ResolveContext{DefaultBranch: shortName(defaultBranch)})`. Resolution failure that blocks topology → wrap in `ErrStatusInvalidInput`; discovery outside a repo or a bad `--repo-dir` → `ErrStatusInvalidInput`; missing remote/ref, network, git executable failures → `ErrStatusExternal` (a `*gitcli.Failure` — read `internal/gitcli/failure.go` for its classification fields and map from those, not from message text). Then fetch + pin the integration branch and, in docket mode, the metadata branch.
- `ReadCorpus`: `ListTree` with the configured prefixes (`changes_dir` active + archive subdirs, `adrs_dir`, learnings dir when `Learnings.Enabled` — read how the Bash convention lays these out from `config.Effective`'s doc comments and the corpus fixtures in `internal/repository/testdata/corpus/`), then one `ReadBlobs` batch; classify each path into `KindChange`/`KindADR`/`KindLearning` and `LocationActive`/`LocationArchive`/`LocationLedger` by prefix.
- `BranchFacts`: for each distinct branch, `FetchBranch` and record present/absent — a fetch whose failure classifies as "no such remote branch" is `false`, not an error (compare against `classifyFetchFailure`'s taxonomy in `internal/gitcli/refs.go`); other failures propagate as `ErrStatusExternal`. Return `domain.NewBranchFacts(map)`.
- `ArtifactExists`: `ReadBlobs` (or `ListTree` on the parent) against the named pinned source; absent path → `(false, nil)`.

- [ ] **Step 1: Write the failing integration tests.** Model the harness on `internal/gitcli`'s own real-git tests (read `internal/gitcli/harness_test.go` first and reuse its temp-repo/bare-remote helpers if exported; otherwise build a local `t.TempDir()` fixture: `git init --bare origin.git`, clone, commit a `.docket.yml` + `docs/changes/active/...` corpus, push). Cover:
  - discovery from a nested subdirectory of the worktree;
  - main mode end-to-end pin + corpus read (assert record versions are blob IDs from the pinned revision);
  - docket mode: a `docket` branch with different record content than `main`; assert corpus content came from the metadata revision and `StatusPin` carries distinct integration/metadata revisions;
  - missing metadata branch on the remote → `ErrStatusExternal`;
  - branch facts: one pushed feature branch, one absent → `HasBranch` true/false, no error;
  - **read-only before/after:** capture `git status --porcelain`, `git rev-parse HEAD`, the symbolic ref, and a checksum of every worktree file before `PinContext`+`ReadCorpus`+`BranchFacts`; assert all identical after. Assert remote-tracking refs *did* move when the remote advanced — the permitted mutation, positively witnessed;
  - **concurrent remote movement:** pin, then push a new commit to the remote, then `ReadCorpus` with the old pin; assert content is still the pinned revision's.
- [ ] **Step 2: Run to verify failure.** `go test -count=1 ./internal/app/ -run TestGitStatusReader` — compile FAIL.
- [ ] **Step 3: Implement `internal/app/status_git.go`.**
- [ ] **Step 4: Run to verify pass.** `go test -count=1 ./internal/app/` and `go vet ./...`. Expected: PASS.
- [ ] **Step 5: Commit.**

```bash
git add internal/app/status_git.go internal/app/status_git_test.go
git commit -m "feat(status): git-backed reader with pinned sources and read-only guarantees"
```

---

### Task 4: Human presenter

**Files:**
- Create: `internal/app/status_human.go`
- Test: `internal/app/status_human_test.go`

**Interfaces:**
- Consumes: Task 1's `StatusResult`.
- Produces: `func (r StatusResult) HumanText() string` — which makes `StatusResult` satisfy `app.OperationResult` (Task 5 relies on that).

**Report shape (spec §Human report, in order):** (1) mode + short (12-char) revisions; (2) complete and displayed counts; (3) ordered ready queue — an empty queue prints `ready queue: (empty)`, never a missing section; (4) one row per displayed change: id, title, readiness, unmet deps, effective base; (5) health totals then ordered error and warning findings — a healthy repo prints `health: ok (0 errors, 0 warnings)`. Repository identity, when shown, is a safe display path (branch names/revisions carry the stable identity), matching the spec's host-path rule. Match `ConfigInspectionResult.HumanText`'s style: `strings.Builder`, `TrimRight(…, "\n")`.

- [ ] **Step 1: Write failing golden tests** — four fixtures built as in-memory `StatusResult` values (healthy, unhealthy, filtered/empty-projection, empty-ready), each asserting the full expected multi-line string with `if got != want { t.Errorf("%q\n!=\n%q", got, want) }`. Derive the `want` strings while writing the renderer, then freeze them; determinism test: render twice, assert identical.
- [ ] **Step 2: Run to verify failure.** `go test -count=1 ./internal/app/ -run TestStatusHumanText` — compile FAIL (`HumanText` undefined).
- [ ] **Step 3: Implement `internal/app/status_human.go`.**
- [ ] **Step 4: Run to verify pass**, then compile-assert the interface: add `var _ OperationResult = StatusResult{}` at the top of `status_human.go`.
- [ ] **Step 5: Commit.**

```bash
git add internal/app/status_human.go internal/app/status_human_test.go
git commit -m "feat(status): deterministic human status report"
```

---

### Task 5: CLI registration and protocol tests

**Files:**
- Modify: `internal/cli/root.go` (add `statusCmd` beside `versionCmd`, register in `root.AddCommand`)
- Modify: `internal/cli/install.go` (add `"status": true` to `assetIndependent` — `TestAssetIndependentSetExact` enforces both directions, so forgetting either side reddens)
- Test: `internal/cli/root_test.go` (extend the existing command-driving tests)

**Interfaces:**
- Consumes: `app.Status`, `app.NewGitStatusReader`, `app.StatusOptions`, `gitcli.NewClient`; the existing `result app.OperationResult` assignment pattern and `Presenter`.
- Produces: the user-facing command:

```text
docket status [--repo-dir PATH] [--type TYPE] [--priority PRIORITY] [--json]
```

Command body — a thin adapter, mirroring `configCmd` exactly (no policy branches):

```go
statusCmd := &cobra.Command{
	Use:   "status",
	Short: "Report backlog status, readiness, selection, and repository health (read-only)",
	Args:  cobra.NoArgs,
	RunE: func(c *cobra.Command, _ []string) error {
		repoDir, _ := c.Flags().GetString("repo-dir")
		types, _ := c.Flags().GetStringArray("type")
		priorities, _ := c.Flags().GetStringArray("priority")
		client, err := gitcli.NewClient()
		if err != nil {
			return err
		}
		result = app.Status(c.Context(), app.NewGitStatusReader(client),
			app.StatusOptions{RepoDir: repoDir, Types: types, Priorities: priorities})
		return nil
	},
}
statusCmd.Flags().String("repo-dir", "", "repository directory to read (default: current directory)")
statusCmd.Flags().StringArray("type", nil, "filter the displayed projection to a configured change type (repeatable)")
statusCmd.Flags().StringArray("priority", nil, "filter the displayed projection to a priority: critical, high, medium, or low (repeatable)")
```

Flag-value validation (closed values only, spec §Command surface): priorities are validated in `app.Status` against the domain's closed priority spellings, and types against the resolved `change_types` — both produce `ResultInvalidInput`, because only the resolved configuration knows the closed type set. The CLI validates nothing but flag syntax.

- [ ] **Step 1: Write failing tests** in `internal/cli/root_test.go` using the file's existing `run(...)`-driving helpers (read the neighboring tests first and copy their harness idiom):
  - `docket status --priority bogus --json --repo-dir <temp git repo>` → exit 2, one JSON document with `"result":"invalid-input"`;
  - `docket status` outside any git repository (`--repo-dir` at a bare temp dir) → `invalid-input`, and in human mode stdout carries the document while stderr stays empty per the presenter contract (assert against how other failing operations present — follow the existing tests' assertions);
  - `docket status --json` in a minimal real fixture repo (temp origin + clone with a `.docket.yml` and one active change) → exit 0, exactly one JSON document terminated by one newline, `"operation":"status"`, `"protocol_version":1`, arrays present;
  - `TestAssetIndependentSetExact` needs no edit — run it and watch it pass only once the map entry exists.
- [ ] **Step 2: Run to verify failure.** `go test -count=1 ./internal/cli/` — new tests FAIL (`unknown command "status"`), and `TestAssetIndependentSetExact` fails after registration until the map entry lands — observe both orders explicitly.
- [ ] **Step 3: Implement** the command and the `assetIndependent` entry.
- [ ] **Step 4: Run to verify pass.** `go test -count=1 ./internal/cli/ ./internal/app/ ./cmd/docket/`. Expected: PASS (cmd/docket's exit-site and help-surface tests must absorb the new command without edits; if one pins the command roster, extend it deliberately).
- [ ] **Step 5: Commit.**

```bash
git add internal/cli/root.go internal/cli/install.go internal/cli/root_test.go
git commit -m "feat(status): register docket status as an asset-independent command"
```

---

### Task 6: Frozen v0.9.3 semantic corpus

**Files:**
- Create: `testdata/repositories/v0.9.3/status-corpus/` (frozen records) + `testdata/repositories/v0.9.3/status-corpus/PROVENANCE.md`
- Modify: `testdata/repositories/v0.9.3/PROVENANCE.md` (the incumbent sidecar's doc)
- Test: `internal/app/status_corpus_test.go`

**Interfaces:**
- Consumes: `app.Status` with a fake or local-git reader over the frozen bytes.
- Produces: the corpus directory layout later regression work reads; no Go API.

**Capture procedure (record it verbatim in the new PROVENANCE.md):**
1. In a scratch clone of `danielhanold/docket`, verify the tag peels to the spec's commit: `git rev-parse 'v0.9.3^{commit}'` → must print `dd742abd5e9fcdf8ffe78eb6f36a293410873bbf`; abort the capture on any other value.
2. Export the selected source paths from that commit **of the metadata branch content docket's own repo carries** — a representative slice, not the whole tree: `docs/changes/active/` (all), a bounded sample of `docs/changes/archive/` (~10 records including at least one stacked pair and one with all three artifact links), `docs/adrs/` index + ~5 ADRs, `docs/changes/learnings/README.md` + 2 findings, and `.docket.yml`. Use `git -C <scratch> show dd742abd:<path> > status-corpus/<path>` so bytes are exact and no working tree is involved.
3. Compute the expected semantic outcomes **by hand from the frozen records** (counts, ready order, dependency satisfaction, effective stack bases, health classifications, artifact locations) and write them into the test as literals with a comment deriving each number.

**Provenance rules:** the new `status-corpus/PROVENANCE.md` names tag `v0.9.3`, peeled commit `dd742abd5e8...` (full id), the date, the capture procedure above, the selected paths, and the expected-outcome summary. The **incumbent** `testdata/repositories/v0.9.3/PROVENANCE.md` currently claims "This one file covers the whole `v0.9.3/` tree" and "the tree is sparse: it carries only the single fixture below" — both false the moment the corpus lands. Amend those two sentences to scope that file to the agent-defaults sidecar and point at `status-corpus/PROVENANCE.md` for the corpus; change nothing else in it, and do not touch `agents-harness-defaults.yml`.

The corpus compares **semantics, not Bash presentation bytes**: no golden Bash output lands here.

- [ ] **Step 1: Capture the corpus** per the procedure; commit nothing yet.
- [ ] **Step 2: Write the failing corpus test** `internal/app/status_corpus_test.go`: build `StatusBlob`s by walking `status-corpus/` (kind/location from path prefix, version from `git hash-object` equivalent — `fmt.Sprintf` of a sha1 over the blob via `git hash-object --stdin` is overkill; use the file bytes' actual git blob id computed with `gitcli` if exported, else a constant "frozen" version string, and assert versions only for the real-git tests), feed a fake reader pin (docket mode, both revisions distinct literals), run `app.Status`, and assert the hand-derived expected outcomes: total/active counts, the exact ready ID order, each stacked change's effective base, each expected health finding code, and each artifact-location verdict.
- [ ] **Step 3: Run to verify failure**, then reconcile: `go test -count=1 ./internal/app/ -run TestStatusCorpus`. A first failure whose diff shows the *derived* numbers were wrong is a fixture-arithmetic bug — recompute by hand; a failure showing the operation is wrong is a real finding. Never adjust an expectation to whatever the code printed without re-deriving it from the records (the corpus is the oracle, not the code).
- [ ] **Step 4: Run the repo-wide guards over the new corpus.** `bash scripts/run-tests.sh tests/test_comment_anchor_style.sh tests/test_assert_hygiene.sh` plus any scan the full run reddens on. Frozen records are DATA: if a repo-wide pattern guard matches inside `testdata/repositories/v0.9.3/status-corpus/`, add a **bounded** exclusion naming that directory (never a wildcard), and mutation-test it — plant the violating pattern in a scratch file just outside the excluded path, watch the guard redden, remove the plant (learnings: `frozen-fixture-corpus-trips-repo-wide-scans`).
- [ ] **Step 5: Run to verify pass**, then commit.

```bash
go test -count=1 ./internal/app/
git add testdata/repositories/v0.9.3/ internal/app/status_corpus_test.go
git commit -m "test(status): frozen v0.9.3 semantic corpus with dual provenance"
```

---

### Task 7: Whole-suite gate, budgets, and mutation probes

**Files:**
- Modify: `tests/runtime-budgets.tsv` (only if measurement demands a re-budget or shard)
- Possibly create: `tests/test_go_race_status.sh` (only if the race shard hits its ceiling)
- Modify: `tests/test_runtime_budgets.sh` `EXPECTED_TOTAL` (only alongside a row change)

**Interfaces:** none — this task certifies the branch.

- [ ] **Step 1: Mutation probes** (every run with `-count=1`; restore each mutation from a **copy you took first**, never `git checkout --`, which would also destroy uncommitted work — learnings: `mutation-restore-needs-a-backup-copy`). For each, record the reddening test name for the results file:
  1. **Read-only guard:** in `status_git.go`, add a stray write (e.g., touch a file in the worktree inside `ReadCorpus`) → the Task 3 before/after test must fail.
  2. **Artifact-source distinction:** make `ArtifactExists` ignore its `source` argument and always read the integration source → the Task 2 docket-mode source-divergence test must fail.
  3. **Full-corpus health:** apply the type/priority filter before health assembly (filter the corpus, not the projection) → the Task 2 filters-cannot-suppress-health test must fail.
  If any probe stays green, the guard is decoration: fix the test before proceeding (CLAUDE.md, Guards and tests).
- [ ] **Step 2: Measure the Go rows.** This change grows `go test ./...` (Task 1–6 tests, real-git fixtures) inside two budgeted files whose margins are known-tight: `tests/test_go_toolchain.sh` (45s) and `tests/test_go_race.sh` (60s — the table's **hard ceiling**, already sharded twice this week; learnings: `budget-headroom-is-spent-before-it-is-breached`). Measure each standalone serial:

```bash
bash scripts/run-tests.sh -j 1 --timings /tmp/t1 tests/test_go_toolchain.sh
bash scripts/run-tests.sh -j 1 --timings /tmp/t2 tests/test_go_race.sh tests/test_go_race_transaction.sh
```

- [ ] **Step 3: Act on the numbers.** If `test_go_toolchain.sh` exceeds its row, re-budget by the table's own rule (next multiple of 5 above the worst standalone serial reading, plus 5s margin) and bump `EXPECTED_TOTAL` by the delta, with an in-file comment defending the measurement in the style of the existing `test_go_toolchain.sh 20 -> 45` entry. If `test_go_race.sh` is at or over 60s, **shard — the hard ceiling has no re-budget**: create `tests/test_go_race_status.sh` carrying `go test -race -count=1 ./internal/app/`, exclude that package from `test_go_race.sh`'s `go list`-derived set exactly the way `test_go_race_transaction.sh` does, extend the **partition guard** so the (now three) files' package sets still union to exactly `go list ./...`, add the new row, and re-seed `EXPECTED_TOTAL`. Copy the shard file's structure from `tests/test_go_race_transaction.sh` verbatim, including the assert-helper block and header rationale. If both files are comfortably under, change nothing — but record the measured margins as numbers for the results file.
- [ ] **Step 4: Run the whole suite.** `bash scripts/run-tests.sh` (background it to a log if it nears the 600s foreground ceiling; key on the exit code). Expected: exit 0, no `NOT OK`, and read any `OVER BUDGET:` block as a finding to act on in Step 3's terms, not noise.
- [ ] **Step 5: Commit** whatever Steps 2–4 changed (or nothing, if nothing moved):

```bash
git add tests/runtime-budgets.tsv tests/test_runtime_budgets.sh tests/test_go_race.sh tests/test_go_race_status.sh 2>/dev/null || true
git commit -m "test(status): budget accounting for the grown Go suite" || echo "no budget changes"
```

---

## Self-Review

- **Spec coverage:** delivered boundary items 1–7 → Tasks 3 (discover/pin/read), 2 (orchestrate/evaluate/artifact checks), 1 (result contract), 4 (human), 5 (CLI + `--json`); §Command surface → Task 5; §Result contract → Task 1; §Health and failure semantics → Task 2's mapping table; §Application tests → Task 2; §Git integration tests → Task 3; §Frozen semantic corpus → Task 6; §Presenter and protocol tests → Tasks 4–5; mutation probes and the whole-suite gate → Task 7. Asset-independent registration (spec §Architecture, last line) → Task 5.
- **Exclusions honored:** no board writes, no maintenance option, no plan parsing, no backlink writing, no transaction/workspace/PR behavior; 0261's board surface untouched.
- **Type consistency:** `StatusPin`/`StatusBlob`/`StatusReader` names match across Tasks 2–3, 6; `NewStatusResult`/`OperationStatus` across 1–2, 4–5; `HumanText` lands in Task 4 before Task 5's `OperationResult` assignment needs it.
- **Known judgment points left to the executor, deliberately:** the exact `document` parse entry point, `gitcli.Failure` classification fields, and the corpus path sample are read-then-apply steps against landed code, named in their tasks rather than transcribed here, so the plan cannot drift from the tree it composes.
