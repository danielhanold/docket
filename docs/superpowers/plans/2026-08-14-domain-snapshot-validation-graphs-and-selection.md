<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0307 — Domain snapshot, validation, graphs, and selection](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0307-domain-snapshot-validation-graphs-and-selection.md)**
<!-- docket:backlink:end -->

# Domain Snapshot, Validation, Graphs, and Selection — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the pure `internal/domain` policy package and the read-model portion of `internal/repository`: typed immutable entities, complete deterministic validation, lifecycle actions, readiness/selection, dependency and stack graphs, claims/reclaim, ADR evolution rules, and learning consumption — with zero filesystem, Git, or subprocess access.

**Architecture:** Two packages with one-way dependency flow: `internal/config` + `internal/document` → `internal/repository` (anti-corruption decoder + validation) → `internal/domain` (pure policy, imports neither config nor document). All aggregates are immutable (defensive copies both directions); policy outcomes are typed values, never Go errors; Go errors are reserved for violated API preconditions.

**Tech Stack:** Go 1.26 (module `github.com/danielhanold/docket`), stdlib only. Tests are table-driven `go test` packages; the repo suite runs them via `tests/test_go_toolchain.sh` (`go test ./...`).

**Spec:** `.docket/docs/superpowers/specs/2026-08-13-domain-snapshot-validation-graphs-and-selection-design.md` (metadata branch; the spec travels with this plan — executors read both).

## Global Constraints

- `internal/domain` imports **neither** `internal/config` nor `internal/document` — nor `os`, `os/exec`, `time` (for `Now`), Git, GitHub, CLI, render, process, or harness packages. `time` may be imported for the `time.Time` type only; no production call to `time.Now()` in either package.
- `internal/repository` may import `internal/config`, `internal/document`, `internal/domain`; never `os`, `os/exec`, `internal/app`, `internal/cli`, `internal/harness`, `internal/install`, `internal/assets`.
- Policy functions return new values; nothing mutates its input. Constructors copy caller-owned slices/maps; accessors return fresh collections.
- Expected policy outcomes are values (typed results), not Go errors.
- Every mutation probe and manual re-verification runs `go test -count=1` (Go's test cache serves stale passes otherwise — learning `cached-runner-serves-a-mutated-tree`).
- Malformed/missing `created:` sorts **after** every valid date in its priority band; unknown stored priority ranks as `medium` (learning `unset-sort-key-check-your-own-template`).
- Missing or malformed `claimed_at` is **never** evidence of expiry; the reclaim TTL boundary is strict `>` (equality is fresh).
- Validation keys on semantic shape, never enumerated bad spellings.
- Commit after every task; all commits on `feat/domain-snapshot-validation-graphs-and-selection`.

## File Structure

```
internal/domain/
  types.go          ChangeID, ADRID, Status, Priority + rank table, ADRStatus, PromotionState, type-token validation
  types_test.go
  entities.go       Change, ADR, Learning, Artifact, DerivedView, RepositoryPolicy + constructors/accessors
  entities_test.go
  findings.go       Severity, EntityRef, Finding, ValidationReport (sort + HasErrors)
  findings_test.go
  snapshot.go       Snapshot + constructors + ambiguity-aware lookups
  snapshot_test.go
  graph.go          dependency graph, cycle detection, dependency satisfaction summary
  graph_test.go
  readiness.go      Readiness, NeedsDesign, build-ready predicate + precedence
  readiness_test.go
  selection.go      deterministic selection queue
  selection_test.go
  stack.go          stack graph walks + effective-base resolution (6 rules)
  stack_test.go
  actions.go        named lifecycle actions + typed policy failures
  actions_test.go
  lease.go          lease evaluation, claim, reclaim
  lease_test.go
  adr.go            ADR graph validation, back-pointers, next-ID, supersede/reverse actions
  adr_test.go
  learning.go       learning-consumption queries
  learning_test.go
internal/repository/
  input.go          RecordKind, RecordLocation, InputDocument, BuildInput, BuildResult
  decode.go         wire structs + per-record decoding (absent/empty/malformed distinction)
  build.go          BuildSnapshot
  validate.go       single-snapshot validation passes
  evolution.go      ValidateEvolution (frozen-ADR byte rules)
  decode_test.go, build_test.go, validate_test.go, evolution_test.go
  boundary_test.go  dependency-direction + purity pin (covers BOTH packages)
  testdata/corpus/  frozen v0.9.2 corpus records
```

---

### Task 1: Closed value types (`internal/domain/types.go`)

**Files:**
- Create: `internal/domain/types.go`
- Test: `internal/domain/types_test.go`

**Interfaces:**
- Consumes: nothing (leaf package files).
- Produces (all later tasks build on these exact names):

```go
type ChangeID int
type ADRID int

type Status string
const (
    StatusProposed      Status = "proposed"
    StatusInProgress    Status = "in-progress"
    StatusBlocked       Status = "blocked"
    StatusDeferred      Status = "deferred"
    StatusImplemented   Status = "implemented"
    StatusStackedMerged Status = "stacked-merged"
    StatusDone          Status = "done"
    StatusKilled        Status = "killed"
)
func ParseStatus(s string) (Status, bool)      // membership check
func (s Status) Terminal() bool                // done|killed

type Priority string
const (
    PriorityCritical Priority = "critical"
    PriorityHigh     Priority = "high"
    PriorityMedium   Priority = "medium"
    PriorityLow      Priority = "low"
)
func ParsePriority(s string) (Priority, bool)
// priorityRank is the ONE package-owned rank table (critical=0 … low=3).
// Unknown priority ranks as PriorityMedium for read-only computation.
func priorityRank(p Priority) int

// ADRStatus is a tagged value: Accepted | Deprecated | SupersededBy(id) | ReversedBy(id).
type ADRStatusKind string
const (
    ADRAccepted     ADRStatusKind = "accepted"
    ADRDeprecated   ADRStatusKind = "deprecated"
    ADRSupersededBy ADRStatusKind = "superseded-by"
    ADRReversedBy   ADRStatusKind = "reversed-by"
)
type ADRStatus struct { Kind ADRStatusKind; Ref ADRID } // Ref valid only for the two *By kinds
// ParseADRStatus parses "Accepted", "Deprecated", "Superseded by ADR-NNNN",
// "Reversed by ADR-NNNN" (case of the verb phrase as written by v0.9.2:
// exact prefix match, ID > 0). Anything else → ok=false.
func ParseADRStatus(s string) (ADRStatus, bool)
func (a ADRStatus) String() string // renders the exact v0.9.2 spelling

type PromotionState string
const (
    PromotionRetained  PromotionState = "retained"
    PromotionCandidate PromotionState = "candidate"
    PromotionPromoted  PromotionState = "promoted"
)
// ParsePromotionState: "" (legacy-missing) resolves to retained, ok=true;
// unknown non-empty → ok=false.
func ParsePromotionState(s string) (PromotionState, bool)

// ValidTypeToken reports whether a stored change type matches [a-z][a-z0-9-]*.
func ValidTypeToken(s string) bool

// BranchForSlug returns the deterministic feature branch name "feat/<slug>".
func BranchForSlug(slug string) string
```

- [ ] **Step 1: Write failing tests** — `internal/domain/types_test.go`, table-driven:
  - `TestParseStatus`: all 8 members round-trip; `"Proposed"`, `""`, `"open"` → false.
  - `TestStatusTerminal`: done/killed true; other six false.
  - `TestParsePriority` + `TestPriorityRank`: 4 members ranked 0–3 in declaration order; `Priority("urgent")` ranks equal to medium.
  - `TestParseADRStatus`: `"Accepted"`, `"Deprecated"`, `"Superseded by ADR-0071"`→`{SupersededBy,71}`, `"Reversed by ADR-0042"`→`{ReversedBy,42}`; rejects `"superseded by ADR-1"` (case), `"Superseded by ADR-"`, `"Superseded by 71"`, `"Accepted "` (trailing space), `"Superseded by ADR-0"` (non-positive). `String()` round-trips all four kinds (`ADR-%04d` padding).
  - `TestParsePromotionState`: three members; `""`→retained ok; `"graduated"`→false.
  - `TestValidTypeToken`: accepts `feat`, `fix2`, `a-b`; rejects `""`, `Feat`, `2fix`, `-a`, `a_b`.
  - `TestBranchForSlug`: `BranchForSlug("x-y") == "feat/x-y"`.
- [ ] **Step 2: Run** `go test ./internal/domain/ -count=1` — expect FAIL (package does not compile / functions undefined).
- [ ] **Step 3: Implement `types.go`** exactly per the Produces block. `ParseADRStatus` uses `strings.CutPrefix` on the two verb phrases then parses the numeric tail of `ADR-<digits>` with `strconv.Atoi`, requiring ≥1 digit and value > 0.
- [ ] **Step 4: Run** `go test ./internal/domain/ -count=1` — expect PASS.
- [ ] **Step 5: Commit** `feat(0307): domain closed value types — status, priority, ADR status, promotion state`

---

### Task 2: Entities and immutability (`internal/domain/entities.go`)

**Files:**
- Create: `internal/domain/entities.go`
- Test: `internal/domain/entities_test.go`

**Interfaces:**
- Consumes: Task 1 types.
- Produces (constructor-arg structs are the mutable *input* shape; the entity itself is opaque):

```go
// OptionalString / OptionalTime / OptionalInt preserve the absent/empty/
// malformed/present distinction the decoder needs. Raw always carries the
// stored text when State != FieldAbsent.
type FieldState int
const (
    FieldAbsent FieldState = iota // key not present
    FieldEmpty                    // key present, no value
    FieldMalformed                // present but unparseable for its type
    FieldPresent                  // present and parsed
)
type OptionalString struct { State FieldState; Value string }
type OptionalInt    struct { State FieldState; Value int; Raw string }
type OptionalTime   struct { State FieldState; Value time.Time; Raw string }

type ChangeSpec struct {
    ID            ChangeID
    Slug          string
    Title         string
    Status        Status
    RawStatus     string      // as stored, for diagnostics
    Priority      Priority
    RawPriority   string
    Type          string
    Created       OptionalTime // date-only; Raw keeps stored text
    Updated       OptionalTime
    DependsOn     []ChangeID
    StackedOn     OptionalInt  // single integer scalar or absent
    Related       []ChangeID
    DiscoveredFrom []ChangeID
    ADRs          []ADRID
    Spec          OptionalString
    Plan          OptionalString
    Results       OptionalString
    Trivial       bool
    Branch        OptionalString
    ClaimedAt     OptionalTime // second-precision UTC; Raw kept
    PR            OptionalString
    Issue         OptionalString
    BlockedBy     OptionalString
    Reconciled    bool
    Location      RecordLocation // active | archive
    Path          string
    ArchiveDate   OptionalTime   // parsed from archive filename prefix
    HasRunHalted        bool     // "## Run halted" body section present
    HasAutoGroomBlocked bool     // "## Auto-groom blocked" present
    HasFinalizeBlocked  bool
    HasPublishDeferred  bool
}
func NewChange(s ChangeSpec) Change   // deep-copies every slice
type Change struct { /* unexported copy of ChangeSpec */ }
// Accessors mirror every ChangeSpec field: ID(), Slug(), Status(),
// DependsOn() []ChangeID (fresh slice), StackedOn() (OptionalInt), … etc.

type ADRSpec struct {
    ID        ADRID
    Slug      string
    Title     string
    Status    ADRStatus
    RawStatus string
    Date      OptionalTime
    Supersedes []ADRID
    Reverses   []ADRID
    RelatesTo  []ADRID
    Change     OptionalInt // producing change
    Path       string
    ContentID  string // opaque content identity supplied by the decoder
}
func NewADR(s ADRSpec) ADR

type LearningSpec struct {
    Slug      string
    Hook      string
    Topics    []string
    Changes   []ChangeID
    Created   OptionalTime
    Updated   OptionalTime
    Promotion PromotionState
    PromotedTo OptionalString
    Content   string
    Path      string
}
func NewLearning(s LearningSpec) Learning

type ArtifactKind string // spec|plan|results|other
type Artifact struct { /* Path, Kind, ContentID, HasBacklinkMarker — via NewArtifact(ArtifactSpec) */ }
type ArtifactSpec struct { Path string; Kind ArtifactKind; ContentID string; HasBacklinkMarker bool }

type DerivedViewKind string // board|adr-index|learnings-index|other
type DerivedView struct { /* Path, Kind — via NewDerivedView(DerivedViewSpec) */ }
type DerivedViewSpec struct { Path string; Kind DerivedViewKind }

type RepositoryPolicy struct {
    IntegrationBranch string
    ChangeTypes       []string
    ReclaimTTLHours   int
    LearningsEnabled  bool
}
func NewRepositoryPolicy(p RepositoryPolicy) RepositoryPolicy // returns a deep copy

type RecordLocation string
const (
    LocationActive   RecordLocation = "active"
    LocationArchive  RecordLocation = "archive"
    LocationLedger   RecordLocation = "ledger"
    LocationArtifact RecordLocation = "artifact"
    LocationDerived  RecordLocation = "derived"
)
```

(`RecordLocation` lives in **domain** so `ChangeSpec.Location` types cleanly; `internal/repository` re-exports the vocabulary via its input aliases in Task 11.)

- [ ] **Step 1: Write failing tests**:
  - `TestNewChangeDefensiveCopies`: build a `ChangeSpec` with populated `DependsOn`/`Related`/`ADRs`; construct; mutate the input slices; assert accessors still return original values; then mutate an accessor's returned slice and re-read — unchanged.
  - `TestChangeAccessorsRoundTrip`: every field set → accessor returns it (one table row per field family, including all four `FieldState`s on `Spec` and `ClaimedAt`).
  - Same copy tests for `NewADR` (`Supersedes`), `NewLearning` (`Topics`), `NewRepositoryPolicy` (`ChangeTypes`).
- [ ] **Step 2: Run** `go test ./internal/domain/ -count=1` — FAIL.
- [ ] **Step 3: Implement.** Constructors copy slices with `slices.Clone`; accessors clone before returning. No maps in entity state.
- [ ] **Step 4: Run** — PASS.
- [ ] **Step 5: Commit** `feat(0307): immutable domain entities with absent/empty/malformed field states`

---

### Task 3: Findings and ValidationReport (`internal/domain/findings.go`)

**Files:**
- Create: `internal/domain/findings.go`
- Test: `internal/domain/findings_test.go`

**Interfaces:**
- Produces:

```go
type Severity string
const (
    SeverityError   Severity = "error"
    SeverityWarning Severity = "warning"
)

type EntityKind string
const (
    EntityChange   EntityKind = "change"
    EntityADR      EntityKind = "adr"
    EntityLearning EntityKind = "learning"
    EntityArtifact EntityKind = "artifact"
    EntityDerived  EntityKind = "derived-view"
    EntityRepo     EntityKind = "repository"
)
type EntityRef struct {
    Kind EntityKind
    ID   int    // 0 when identity is non-numeric
    Slug string
    Path string
}

type Finding struct {
    Code     string
    Severity Severity
    Entity   EntityRef
    Field    string
    Related  []EntityRef
    Detail   map[string]string
}

type ValidationReport struct { /* unexported findings slice */ }
func NewValidationReport(findings []Finding) ValidationReport // copies, sorts deterministically
func (r ValidationReport) Findings() []Finding                // fresh deep copy
func (r ValidationReport) HasErrors() bool
// sort key: Code, Entity.Kind, Entity.ID, Entity.Slug, Entity.Path, Field
```

- [ ] **Step 1: Failing tests**: `TestReportSortDeterministic` (shuffled input, two constructions, identical output; order follows the documented key), `TestHasErrors` (warnings-only false; one error true; empty false), `TestReportImmutable` (mutate input slice + returned slice/`Detail` map; report unchanged).
- [ ] **Step 2: Run** — FAIL.  
- [ ] **Step 3: Implement** with `slices.SortStableFunc` and deep copies (`maps.Clone` for `Detail`, `slices.Clone` for `Related`).
- [ ] **Step 4: Run** `go test ./internal/domain/ -count=1` — PASS.
- [ ] **Step 5: Commit** `feat(0307): typed validation findings and deterministic report`

---

### Task 4: Snapshot and ambiguity-aware lookups (`internal/domain/snapshot.go`)

**Files:**
- Create: `internal/domain/snapshot.go`
- Test: `internal/domain/snapshot_test.go`

**Interfaces:**
- Produces:

```go
type SnapshotSpec struct {
    Policy       RepositoryPolicy
    Changes      []Change
    ADRs         []ADR
    Learnings    []Learning
    Artifacts    []Artifact
    DerivedViews []DerivedView
}
func NewSnapshot(s SnapshotSpec) Snapshot

type LookupOutcome int
const (
    LookupFound LookupOutcome = iota
    LookupAbsent
    LookupAmbiguous // duplicate IDs — no winner is picked
)
func (s Snapshot) Change(id ChangeID) (Change, LookupOutcome)
func (s Snapshot) ADR(id ADRID) (ADR, LookupOutcome)
func (s Snapshot) Learning(slug string) (Learning, LookupOutcome)
func (s Snapshot) Changes() []Change       // authored input order, fresh slice
func (s Snapshot) ADRs() []ADR
func (s Snapshot) Learnings() []Learning
func (s Snapshot) Artifacts() []Artifact
func (s Snapshot) DerivedViews() []DerivedView
func (s Snapshot) Policy() RepositoryPolicy
```

- [ ] **Step 1: Failing tests**: found/absent/ambiguous for all three lookups (two changes with the same ID → `LookupAmbiguous`, both retained in `Changes()`); collection accessors preserve input order and are fresh copies; `TestSnapshotImmutable` mutates the spec's slices post-construction.
- [ ] **Step 2: Run** — FAIL.
- [ ] **Step 3: Implement.** Index maps built at construction; an ID seen twice maps to a sentinel ambiguous marker.
- [ ] **Step 4: Run** `-count=1` — PASS.
- [ ] **Step 5: Commit** `feat(0307): immutable snapshot with ambiguity-aware lookups`

---

### Task 5: Dependency graph, cycles, satisfaction (`internal/domain/graph.go`)

**Files:**
- Create: `internal/domain/graph.go`
- Test: `internal/domain/graph_test.go`

**Interfaces:**
- Produces:

```go
type DependencyReason string
const (
    DepNotBuilt  DependencyReason = "not-built"
    DepNeedsMerge DependencyReason = "needs-merge" // outranks not-built for the summary
)
type UnmetDependency struct { ID ChangeID; Reason DependencyReason; Missing bool }
type DependencyEvaluation struct {
    Unmet          []UnmetDependency // authored depends_on order
    Summary        DependencyReason  // zero value when Satisfied
    Representative ChangeID          // first unmet dep in authored order matching Summary
    Satisfied      bool
}
// EvaluateDependencies: a dependency is satisfied ONLY when the referenced
// change is done. implemented → needs-merge; every other non-done status,
// and a missing or ambiguous reference, → not-built (Missing marks absent refs).
func EvaluateDependencies(s Snapshot, c Change) DependencyEvaluation

type Cycle struct { Members []ChangeID } // rotation-normalized: starts at smallest ID
// DependencyCycles finds every cycle (self-cycles included) over depends_on,
// deterministically ordered by first member then length.
func DependencyCycles(s Snapshot) []Cycle
// StackCycles: same over the stacked_on edge.
func StackCycles(s Snapshot) []Cycle
```

- [ ] **Step 1: Failing tests** (tables build snapshots in memory via Tasks 2/4 constructors):
  - satisfaction: dep done → satisfied; dep implemented → `needs-merge`; dep proposed/in-progress/blocked/deferred/stacked-merged/killed → `not-built`; missing ID → `not-built` + `Missing`; mixed `[implemented, missing]` → summary `needs-merge`, representative = the implemented one even when the missing one is authored first (**needs-merge outranks not-built**); representative *within* the same reason = first in authored order.
  - cycles: self-cycle `A→A`; two-node `A→B→A`; three-node with tail; disjoint cycles reported separately, deterministically; `StackCycles` on `stacked_on` chains.
- [ ] **Step 2: Run** — FAIL.
- [ ] **Step 3: Implement.** Iterative DFS with explicit stack (spec: no recursion that can loop forever on bad input); cycle normalization rotates members to start at the smallest ID.
- [ ] **Step 4: Run** `-count=1` — PASS.
- [ ] **Step 5: Commit** `feat(0307): dependency graphs, cycle detection, done-only satisfaction`

---

### Task 6: Readiness (`internal/domain/readiness.go`)

**Files:**
- Create: `internal/domain/readiness.go`
- Test: `internal/domain/readiness_test.go`

**Interfaces:**
- Consumes: `EvaluateDependencies`, `ResolveEffectiveBase` (Task 7 — see note below).
- Produces:

```go
type ReadinessKind string
const (
    ReadyBuildReady      ReadinessKind = "build-ready"
    ReadyNeedsBrainstorm ReadinessKind = "needs-brainstorm"
    ReadyAutoGroomBlocked ReadinessKind = "auto-groom-blocked"
    ReadyWaitingDependency ReadinessKind = "waiting-dependency"
    ReadyStackBaseUnresolved ReadinessKind = "stack-base-unresolved"
    ReadyInvalid         ReadinessKind = "invalid"
    ReadyNotProposed     ReadinessKind = "not-proposed"
)
type Readiness struct {
    Kind       ReadinessKind
    Dependency DependencyEvaluation // populated for waiting-dependency
    StackBase  EffectiveBase        // populated when stack resolution was consulted
}
// EvaluateReadiness precedence: not-proposed → invalid (ambiguous identity)
// → waiting-dependency → needs-brainstorm / auto-groom-blocked (HasAutoGroomBlocked
// distinguishes) → stack-base-unresolved (consulted ONLY when otherwise
// build-ready) → build-ready.
// facts.RemoteBranches feeds stack resolution.
func EvaluateReadiness(s Snapshot, c Change, facts BranchFacts) Readiness

// NeedsDesign ignores dependency satisfaction: proposed, no spec, not trivial.
func NeedsDesign(c Change) bool
```

**Ordering note:** Tasks 6 and 7 are mutually arranged so the compile works: implement Task 7 (`stack.go`, `EffectiveBase`, `BranchFacts`) **before** this task if executing strictly sequentially — the task order below already does that. This task is listed first only because readiness is conceptually upstream; **execute Task 7, then Task 6.**

- [ ] **Step 1** (after Task 7 exists): failing tests:
  - each `ReadinessKind` reachable: non-proposed → `not-proposed`; unmet dep + no spec → `waiting-dependency` (dependency outranks design); proposed+no spec+not trivial → `needs-brainstorm`; same with `HasAutoGroomBlocked` → `auto-groom-blocked`; spec'd, deps met, stacked on unresolvable parent → `stack-base-unresolved`; `trivial: true` with no spec → build-ready; spec empty-string (`FieldEmpty`) counts as **no spec**.
  - `NeedsDesign` true even with unmet deps (design-ahead).
- [ ] **Step 2: Run** — FAIL.  
- [ ] **Step 3: Implement.**  
- [ ] **Step 4: Run** `-count=1` — PASS.
- [ ] **Step 5: Commit** `feat(0307): typed readiness with retained precedence and NeedsDesign`

---

### Task 7: Stack graph and effective bases (`internal/domain/stack.go`) — execute before Task 6

**Files:**
- Create: `internal/domain/stack.go`
- Test: `internal/domain/stack_test.go`

**Interfaces:**
- Produces:

```go
type BranchFacts struct { RemoteBranches map[string]bool } // copied on use
type EffectiveBaseKind string
const (
    BaseResolved     EffectiveBaseKind = "resolved"
    BaseParentKilled EffectiveBaseKind = "parent-killed"
    BaseMissingParent EffectiveBaseKind = "missing-parent"
    BaseCycle        EffectiveBaseKind = "cycle"
    BaseMalformedEdge EffectiveBaseKind = "malformed-edge"
    BaseBranchAbsent EffectiveBaseKind = "branch-absent" // live parent, no remote branch
)
type EffectiveBase struct {
    Kind   EffectiveBaseKind
    Branch string   // resolved branch name (Kind==BaseResolved)
    Cause  ChangeID // exact ancestor for parent-killed/missing/branch-absent
}
func ResolveEffectiveBase(s Snapshot, c Change, facts BranchFacts) EffectiveBase

// Stack walks (iterative, cycle-safe):
func StackParent(s Snapshot, c Change) (Change, LookupOutcome)
func StackAncestors(s Snapshot, c Change) []ChangeID  // nearest first; stops at cycle
func StackChildren(s Snapshot, id ChangeID) []ChangeID // ascending ID
func StackDescendantsParentFirst(s Snapshot, id ChangeID) []ChangeID
```

Resolution rules in precedence order (spec §Stack graph):
1. unstacked → integration branch;
2. killed parent → `parent-killed`, naming the exact ancestor reached, **even when a branch with its recorded name still exists**;
3. `done` parent → integration branch, terminally (ADR-0092) — never recurses further;
4. other parent whose recorded branch is in `RemoteBranches` → that branch;
5. branchless `stacked-merged` parent → recurse into ITS effective base;
6. missing parent / cycle / malformed edge (`StackedOn` state `FieldMalformed`) / live parent with no branch → the distinct causes above.

- [ ] **Step 1: Failing tests** — one table row per rule plus:
  - **ADR-0092 discriminating test:** `done` parent above a **killed grandparent** resolves to the integration branch and never reaches the killed ancestor.
  - killed parent whose recorded branch is still in `RemoteBranches` → still `parent-killed`.
  - rule-5 recursion: `stacked-merged` branchless parent whose own parent has a live branch → that branch; chain of two `stacked-merged` parents → keeps recursing; recursion hitting a killed ancestor → `parent-killed` with the exact ancestor's ID.
  - cycle in `stacked_on` → `BaseCycle`; walks terminate.
  - `StackDescendantsParentFirst` ordering: parents before children, ascending ID among siblings.
- [ ] **Step 2: Run** — FAIL. **Step 3: Implement** (iterative loop with visited set). **Step 4: Run** `-count=1` — PASS.
- [ ] **Step 5: Commit** `feat(0307): stack graph walks and effective-base resolution incl ADR-0092 terminal done arm`

---

### Task 8: Lifecycle actions (`internal/domain/actions.go`)

**Files:**
- Create: `internal/domain/actions.go`
- Test: `internal/domain/actions_test.go`

**Interfaces:**
- Produces:

```go
type PolicyFailureKind string
const (
    FailInvalidState PolicyFailureKind = "invalid-state" // illegal source status
    FailBlocked      PolicyFailureKind = "blocked"       // unmet precondition
    FailInvalidInput PolicyFailureKind = "invalid-input" // malformed supplied fact
)
type PolicyFailure struct {
    Kind   PolicyFailureKind
    Change ChangeID
    State  Status
    Reason string            // stable token, not prose (e.g. "missing-pr", "empty-block-reason")
    Detail map[string]string
}
func (f PolicyFailure) Error() string // implements error for plumbing; callers switch on Kind

type FieldChange struct { Field string; From, To string }
type ActionResult struct {
    Change  Change        // the next semantic Change
    Changed []FieldChange // typed description of what changed
    // OwnedRemovals names body sections the persisting layer must remove
    // (e.g. "## Run halted" on claim). Never empty strings.
    OwnedRemovals []string
}

// Every action takes explicit facts; none reads a clock or a repo.
func Claim(c Change, now time.Time) (ActionResult, *PolicyFailure)             // proposed → in-progress; branch=feat/<slug>; claimed_at=now; reconciled=false
func RefreshClaim(c Change, now time.Time) (ActionResult, *PolicyFailure)      // in-progress only; re-stamps claimed_at
func Block(c Change, reason string) (ActionResult, *PolicyFailure)             // in-progress → blocked; reason must be non-empty
func Unblock(c Change) (ActionResult, *PolicyFailure)                          // blocked → in-progress
func Defer(c Change) (ActionResult, *PolicyFailure)                            // proposed|in-progress → deferred
func Revive(c Change) (ActionResult, *PolicyFailure)                           // deferred → proposed
func Kill(c Change) (ActionResult, *PolicyFailure)                             // proposed|in-progress → killed; clears claimed_at
type ImplementedFacts struct { PR string; Plan string; Now time.Time }
func MarkImplemented(c Change, f ImplementedFacts) (ActionResult, *PolicyFailure) // in-progress → implemented; requires non-empty PR and Plan, reconciled==true
type MergeFacts struct { VerifiedDestination string } // branch the merge verifiably landed on
func MarkStackedMerged(c Change, parentBranch string, f MergeFacts) (ActionResult, *PolicyFailure) // implemented → stacked-merged; destination must equal parentBranch
type DoneFacts struct { ReachableFromIntegration bool }
func MarkDone(c Change, f DoneFacts) (ActionResult, *PolicyFailure)            // implemented|stacked-merged → done; requires ReachableFromIntegration
func Reclaim(c Change, now time.Time, ttlHours int, facts BranchFacts) (ActionResult, *PolicyFailure) // via Task 9 predicate

// KillStackParent: distinct graph action. Returns the kill result for the
// parent plus a Block result per non-terminal descendant with the retained
// reason "stack parent killed — re-scope, re-parent, or kill"; an
// already-blocked descendant is a semantic no-op (present with Changed==nil).
type StackKillResult struct {
    Parent      ActionResult
    Descendants []DescendantOutcome
}
type DescendantOutcome struct { ID ChangeID; Result ActionResult; NoOp bool }
func KillStackParent(s Snapshot, id ChangeID) (StackKillResult, *PolicyFailure)
```

- [ ] **Step 1: Failing tests** — full source-state × action matrix:
  - every action from every one of the 8 statuses: legal rows produce the expected next status + `Changed` set; illegal rows → `FailInvalidState`.
  - `Claim`: sets `branch: feat/<slug>` (uses `BranchForSlug`), `claimed_at` = injected now, `reconciled: false`; `OwnedRemovals` contains `"## Run halted"` **only** when `HasRunHalted`.
  - `Block("")` → `FailInvalidInput` (`empty-block-reason`).
  - `MarkImplemented` without PR / without Plan / with `reconciled: false` → `FailBlocked` with stable reasons.
  - `MarkStackedMerged` with destination ≠ parent branch → `FailBlocked`.
  - `MarkDone` with `ReachableFromIntegration: false` → `FailBlocked`; from `stacked-merged` → legal.
  - `Kill` clears `claimed_at`/`branch` in `Changed`.
  - `KillStackParent`: parent with proposed+blocked+done children → proposed child blocked, already-blocked child NoOp, done child not touched; failure when the parent lookup is ambiguous → `FailInvalidInput`.
  - Input `Change` values are never mutated (compare a deep snapshot of the input before/after each action).
- [ ] **Step 2: Run** — FAIL. **Step 3: Implement** (each action validates then constructs a new `Change` via the entity constructor). **Step 4: Run** `-count=1` — PASS.
- [ ] **Step 5: Commit** `feat(0307): named lifecycle actions with typed policy failures`

---

### Task 9: Lease evaluation and reclaim predicate (`internal/domain/lease.go`)

**Files:**
- Create: `internal/domain/lease.go`
- Modify: `internal/domain/actions.go` (wire `Reclaim` to the predicate)
- Test: `internal/domain/lease_test.go`

**Interfaces:**
- Produces:

```go
type LeaseState string
const (
    LeaseFresh        LeaseState = "fresh"
    LeaseExpired      LeaseState = "expired"
    LeaseMissing      LeaseState = "missing"
    LeaseMalformed    LeaseState = "malformed"
    LeaseNotInProgress LeaseState = "not-in-progress"
)
func EvaluateLease(c Change, now time.Time, ttlHours int) LeaseState

type ReclaimVerdict struct {
    Eligible bool
    Lease    LeaseState
    BlockingBranch string // which branch name blocked it, "" if none
}
// Eligible iff: in-progress AND lease strictly expired (now - claimed_at > TTL)
// AND neither the recorded branch nor conventional feat/<slug> exists in facts.
func EvaluateReclaim(c Change, now time.Time, ttlHours int, facts BranchFacts) ReclaimVerdict
```

- [ ] **Step 1: Failing tests**:
  - `EvaluateLease`: fresh under TTL; **exactly at TTL boundary → fresh** (strict `>`); one second past → expired; `ClaimedAt` `FieldAbsent` → missing; `FieldMalformed` → malformed; proposed change → not-in-progress.
  - `EvaluateReclaim`: all three conjuncts; recorded branch present → ineligible with `BlockingBranch` = recorded name **regardless of lease age**; recorded branch absent but conventional `feat/<slug>` present → ineligible; missing/malformed stamp → ineligible (never positive evidence of expiry).
  - `Reclaim` action: eligible → `proposed`, clears `branch` + `claimed_at`, sets `reconciled: false`; ineligible → `FailBlocked` with the verdict's reason.
- [ ] **Step 2: Run** — FAIL. **Step 3: Implement.** **Step 4: Run** `-count=1` — PASS.
- [ ] **Step 5: Commit** `feat(0307): lease states and three-conjunct reclaim with strict TTL boundary`

---

### Task 10: ADR policy and learning queries (`internal/domain/adr.go`, `internal/domain/learning.go`)

**Files:**
- Create: `internal/domain/adr.go`, `internal/domain/learning.go`
- Test: `internal/domain/adr_test.go`, `internal/domain/learning_test.go`

**Interfaces:**
- Produces:

```go
// adr.go
// ValidateADRGraph: supersedes edge X→Y requires Y.Status exactly
// SupersededBy(X); reverses edge requires ReversedBy(X); a *By status Ref
// must exist and carry the matching reciprocal edge; dangling relates_to /
// supersedes / reverses / change references are errors; numbering gaps are
// warnings (code "adr-id-gap").
func ValidateADRGraph(s Snapshot) []Finding
func NextADRID(s Snapshot) ADRID // max(existing)+1; gaps are never allocation targets; empty set → 1

type ADRActionResult struct {
    NewStatus ADRStatus // the flipped status for the target
    Target    ADRID
}
// Supersede/Reverse: target must exist, be Accepted; returns the status flip
// for the target (the new ADR itself is authored by change 0312).
func Supersede(s Snapshot, target ADRID, successor ADRID) (ADRActionResult, *PolicyFailure)
func Reverse(s Snapshot, target ADRID, successor ADRID) (ADRActionResult, *PolicyFailure)

// learning.go
type LearningCatalog struct {
    Disabled bool
    Findings []Learning // active = retained+candidate; promoted excluded
}
// LearningCandidates: policy.LearningsEnabled==false → {Disabled: true, nil}.
func LearningCandidates(s Snapshot) LearningCatalog
// FilterLearnings filters the active catalog by explicit topic/slug inputs
// (OR within a dimension, AND across when both supplied). Relevance judgment
// stays with the calling skill.
func FilterLearnings(s Snapshot, topics []string, slugs []string) LearningCatalog
```

- [ ] **Step 1: Failing tests**:
  - graph: X supersedes Y with Y `Superseded by ADR-X` → clean; wrong verb (Y says `Reversed by ADR-X`) → error; Y still Accepted → error; Y's status names Z but Z has no edge → error; dangling `relates_to` → error; dangling `change:` back-link → error; IDs 1,2,4 → gap **warning**; self-supersede → error.
  - `NextADRID`: {1,2,4} → 5; empty → 1.
  - `Supersede`/`Reverse`: Accepted target → correct flipped `ADRStatus`; missing target → `FailInvalidInput`; already-superseded target → `FailInvalidState`.
  - learnings: disabled policy → `Disabled: true` even with findings present; enabled → promoted excluded from `Findings`; filters by topic, by slug, by both.
- [ ] **Step 2: Run** — FAIL. **Step 3: Implement.** **Step 4: Run** `-count=1` — PASS.
- [ ] **Step 5: Commit** `feat(0307): ADR graph validation, supersede/reverse policy, learning consumption queries`

---

### Task 11: Repository input types and record decoding (`internal/repository/input.go`, `decode.go`)

**Files:**
- Create: `internal/repository/input.go`, `internal/repository/decode.go`
- Test: `internal/repository/decode_test.go`

**Interfaces:**
- Consumes: `document.Document` (`Parse`, `DecodeFrontmatter`, `Field`, `FieldShape`), `config.Effective`, all domain constructors.
- Produces:

```go
package repository

type RecordKind string
const (
    KindChange   RecordKind = "change"
    KindADR      RecordKind = "adr"
    KindLearning RecordKind = "learning"
    KindArtifact RecordKind = "artifact"
    KindDerived  RecordKind = "derived-view"
)
type RecordLocation = domain.RecordLocation // alias; constants re-used

type InputDocument struct {
    Kind     RecordKind
    Location RecordLocation
    Path     string
    Document document.Document
}
type BuildInput struct {
    Config    config.Effective
    Documents []InputDocument
}

// decode.go (private): per-kind wire structs with yaml tags, e.g.
type changeWire struct {
    ID        *int      `yaml:"id"`
    Slug      *string   `yaml:"slug"`
    Title     *string   `yaml:"title"`
    Status    *string   `yaml:"status"`
    Priority  *string   `yaml:"priority"`
    Type      *string   `yaml:"type"`
    Created   *string   `yaml:"created"`
    Updated   *string   `yaml:"updated"`
    DependsOn []int     `yaml:"depends_on"`
    StackedOn *int      `yaml:"stacked_on"`
    Related   []int     `yaml:"related"`
    DiscoveredFrom []int `yaml:"discovered_from"`
    ADRs      []int     `yaml:"adrs"`
    Spec      *string   `yaml:"spec"`
    Plan      *string   `yaml:"plan"`
    Results   *string   `yaml:"results"`
    Trivial   *bool     `yaml:"trivial"`
    Branch    *string   `yaml:"branch"`
    ClaimedAt *string   `yaml:"claimed_at"`
    PR        *string   `yaml:"pr"`
    Issue     *string   `yaml:"issue"`
    BlockedBy *string   `yaml:"blocked_by"`
    Reconciled *bool    `yaml:"reconciled"`
}
// decodeChange(doc InputDocument) (domain.Change, []domain.Finding)
// decodeADR(doc InputDocument)    (domain.ADR, []domain.Finding)
// decodeLearning(doc InputDocument) (domain.Learning, []domain.Finding)
// decodeArtifact / decodeDerived: path+kind+content identity, no prose interpretation.
```

Decoding rules (each is a test row):
- pointer-nil = `FieldAbsent`; present-but-empty (`FieldShape` from `Document.Field`) = `FieldEmpty`; present-unparseable (bad date/timestamp, non-integer `stacked_on`) = `FieldMalformed` with `Raw` preserved and a finding; never launder to a zero value.
- Optional-key/body-prose hazard: `Document.Field` locates **frontmatter** fields only, so key-shaped body prose (`status:` discussed in a body paragraph) never decodes (fixture `body-keylike-lines.md` pattern).
- Unknown fields stay in the document sidecar; typed decode does not fail (`unknown-fields.md` pattern).
- Lists preserve authored order.
- `type:` validated with `domain.ValidTypeToken`; a token absent from `Config.ChangeTypes` is **not** an error (readability of shared history).
- ADR status parsed with `domain.ParseADRStatus`; unparseable → finding + record retained as invalid.
- Learning promotion `""` → retained.
- Change body scanned for the four presence markers as whole heading lines: `## Run halted`, `## Auto-groom blocked`, `## Finalize blocked`, `## Publish deferred` (bare-heading whole-line match; a dated heading variant does not count).
- ADR/artifact `ContentID` = SHA-256 hex of `Document.Source()` (opaque identity; also used by evolution masking in Task 13).
- Archive filename `YYYY-MM-DD-` prefix parsed into `ArchiveDate` for `LocationArchive`.

- [ ] **Step 1: Failing tests** — in-memory literals through `document.Parse` for every rule above, plus one full happy-path change/ADR/learning each asserting every decoded field.
- [ ] **Step 2: Run** `go test ./internal/repository/ -count=1` — FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** — PASS.
- [ ] **Step 5: Commit** `feat(0307): repository input boundary and record decoders preserving absent/empty/malformed`

---

### Task 12: BuildSnapshot and single-snapshot validation (`internal/repository/build.go`, `validate.go`)

**Files:**
- Create: `internal/repository/build.go`, `internal/repository/validate.go`, `internal/repository/testdata/corpus/` (frozen v0.9.2 records)
- Test: `internal/repository/build_test.go`, `internal/repository/validate_test.go`

**Interfaces:**
- Produces:

```go
type BuildResult struct {
    Snapshot domain.Snapshot
    Report   domain.ValidationReport
}
// BuildSnapshot decodes every document, translates config.Effective's
// supported leaves into domain.RepositoryPolicy (IntegrationBranch,
// ChangeTypes, Reclaim.LeaseTTL, Learnings.Enabled), runs the complete
// validation pass, and returns snapshot + report. Go error ONLY for
// violated call shape (nil document, unknown RecordKind token); repository
// defects are findings.
func BuildSnapshot(input BuildInput) (BuildResult, error)
```

Validation pass (each bullet = table rows in `validate_test.go`; codes are stable lowercase-hyphen tokens):
- identity: usable numeric IDs (`change-id-invalid`), slug shape, status domain (`change-status-unknown`), priority domain (warning `change-priority-unknown`), date domains, path grammar, filename↔frontmatter identity (`change-filename-mismatch`: `0042-slug.md` must match `id`/`slug`), unique IDs (`change-id-duplicate`), slugs, paths.
- placement: terminal status in `active/` → error; non-terminal in `archive/` → error; archive filename date missing/malformed → error; terminal archived records carry no claim stamp.
- state coherence (v0.9.2 writer guarantees): `in-progress` carries branch + valid claim stamp; `blocked` carries `blocked_by`; `implemented` carries branch, claim stamp, plan, PR, `reconciled: true`; `stacked-merged` additionally a usable stack parent. Fields legitimately absent from older valid records stay tolerated where no lifecycle state requires them (a `proposed` change with no `claimed_at` key at all is clean).
- references: `depends_on`/`related`/`discovered_from`/`stacked_on` name existing changes; `adrs:` name existing ADRs; ADR `change:` back-links exist; learning `changes:` exist; supplied artifact references (change `spec:`/`plan:`/`results:` paths that were supplied as artifact inputs) resolve — a referenced path NOT among the supplied documents is **not** a finding (the composer decides what to supply); a supplied artifact nothing references is accounted, not flagged.
- graphs: `DependencyCycles` + `StackCycles` members each produce an error finding naming every member; missing stack parents.
- ADR pass: `ValidateADRGraph` findings merged in.
- learnings: malformed identities, topics, dates, promotion states, promotion destinations.
- accounting: every supplied path lands in exactly one of — a decoded entity, an invalid-record finding (`record-undecodable`), or artifact/derived-view accounting. Nothing silently dropped; duplicate IDs retain both records and make lookups `LookupAmbiguous`.

- [ ] **Step 1: Freeze the corpus.** Copy 6 representative real records from the metadata branch into `internal/repository/testdata/corpus/`: one archived `done` change, one archived `killed` change, one active `proposed` change with dependencies and spec, one `deferred` change, one Accepted ADR, one superseded ADR pair member, one learning finding. Source them with `git -C .docket show docket:<path>` at build time of this task (they become frozen bytes; never regenerated).
- [ ] **Step 2: Failing tests** — `build_test.go`: happy-path corpus builds a clean snapshot (zero error findings; warnings allowed and asserted by code); policy translation asserts each `RepositoryPolicy` field against a literal `config.Effective`; call-shape violation (unknown `RecordKind`) returns a Go error. `validate_test.go`: one in-memory table row per bullet above, asserting `Code`, `Severity`, and `Entity`.
- [ ] **Step 3: Run** — FAIL. **Step 4: Implement.** **Step 5: Run** `go test ./internal/repository/ -count=1` — PASS.
- [ ] **Step 6: Commit** `feat(0307): snapshot builder with complete deterministic validation over frozen v0.9.2 corpus`

---

### Task 13: Before/after evolution validation (`internal/repository/evolution.go`)

**Files:**
- Create: `internal/repository/evolution.go`
- Test: `internal/repository/evolution_test.go`

**Interfaces:**
- Consumes: `document.Document` spans (`Field("status").Value` span masks the status value), Task 11 decode results.
- Produces:

```go
// EvolutionInput pairs each record identity with its exact source bytes and
// parsed document, for both snapshots.
type EvolutionInput struct {
    Before, After BuildResult
    // Sources: path → exact bytes for every ADR document in each snapshot.
    BeforeSources, AfterSources map[string][]byte
}
// ValidateEvolution checks before→after rules without writing either side:
//  - an ADR present in before is byte-frozen except (a) a legal status flip
//    where ONLY the status value span differs, or (b) while still Accepted,
//    one or more appended "## Update" sections at EOF with every
//    pre-existing byte identical ("## Update" or "## Update — …" headings).
//  - a superseded/reversed/deprecated ADR cannot receive a later body update.
//  - identity reuse or mutation of an existing record's immutable identity
//    (change/ADR id or slug changed in place at the same path, or a new
//    record reusing a before-snapshot id at a different path) is an error.
// Findings use codes "adr-frozen-content-modified", "adr-update-after-terminal",
// "adr-status-flip-illegal", "identity-mutated", "identity-reused".
func ValidateEvolution(in EvolutionInput) []domain.Finding
```

Mechanics: for a status-only comparison, splice the before bytes as `prefix + mask + suffix` using the before document's `Field("status")` value span and the after's, comparing prefix and suffix byte-ranges directly — **no normalization, no re-encode**. For update-append: after must begin with the entire before bytes verbatim (prefix check), and the appended tail must start with a `## Update` heading line.

- [ ] **Step 1: Failing tests** (byte-level, per spec §Testing):
  - status-only flip Accepted→`Superseded by ADR-0099` with reciprocal edge in after → clean;
  - append `## Update — 2026-08-14` at EOF while Accepted → clean; both legacy `## Update` and current `## Update — …` heading shapes pass;
  - editing `## Decision` → `adr-frozen-content-modified`;
  - editing an earlier update, a comment, an unknown frontmatter field, or any other existing byte → same code;
  - append to a superseded ADR → `adr-update-after-terminal`;
  - status flip with body edit in the same diff → frozen-content error;
  - illegal flip (Accepted→Accepted-with-different-slug, or Superseded→Accepted) → `adr-status-flip-illegal` / `identity-mutated`;
  - id reuse at a new path → `identity-reused`;
  - **mutation-probe guards** (write as regular tests asserting internal helpers): removing the status mask (compare raw bytes) must redden the status-flip-clean row; removing the prefix check must redden the append-tamper row. Structure the implementation so each is one predicate function (`statusMaskedEqual`, `frozenPrefixIntact`) the tests exercise directly with tampered inputs.
- [ ] **Step 2: Run** — FAIL. **Step 3: Implement.** **Step 4: Run** `-count=1` — PASS.
- [ ] **Step 5: Commit** `feat(0307): before/after ADR evolution validation over exact bytes`

---

### Task 14: Boundary pin, immutability sweep, whole-suite gate

**Files:**
- Create: `internal/repository/boundary_test.go`
- Test: full suite.

**Interfaces:** none new.

- [ ] **Step 1: Write the dependency-direction test** in `boundary_test.go` using `go/parser` (`parser.ImportsOnly`) over the two package directories (test files excluded):
  - `internal/domain` production files import **no** `internal/...` package and none of: `os`, `os/exec`, `net`, `net/http`, `io/fs`, `path/filepath`, `internal/app`, `internal/cli`.
  - `internal/repository` production files import from `internal/...` only `internal/domain`, `internal/config`, `internal/document`; and never `os`, `os/exec`, `net`, `net/http`.
  - Neither package's production files contain a `time.Now` call site (scan token stream for the selector; `time` import for types remains legal).
- [ ] **Step 2: Run** `go test ./internal/... -count=1` — PASS (or fix violations).
- [ ] **Step 3: Mutation-probe the load-bearing guards**, re-running each probe with `-count=1` and reverting after each (keep a copy of the edited file first — never `git checkout --` over uncommitted work):
  1. invert the strict-`>` TTL boundary to `>=` → the at-boundary lease test reddens;
  2. make `implemented` satisfy dependencies → the done-only satisfaction test reddens;
  3. remove the ADR-0092 `done` terminal arm (fall through to recorded-branch lookup) → the killed-grandparent test reddens;
  4. skip the status mask in `statusMaskedEqual` → status-flip-clean row reddens;
  5. drop the `needs-merge` outranking → summary-precedence test reddens;
  6. drop a defensive copy in `NewChange` → immutability test reddens.
  Record each probe's observed red in the task commit message body.
- [ ] **Step 4: Run the whole repo suite** (the build gate): `scripts/run-tests.sh` from the worktree root. Expect green; investigate any `OVER BUDGET:` line (finding, not noise).
- [ ] **Step 5: Commit** `test(0307): dependency-direction pin, time.Now ban, mutation-probe receipts`

---

## Self-Review

- **Spec coverage:** closed types (T1); entities + immutability (T2); findings/report/HasErrors gate (T3); snapshot + inconclusive lookups (T4); dependency graph/cycles/satisfaction + needs-merge precedence (T5); readiness + NeedsDesign precedence (T6); selection + malformed-date/priority fallbacks (below — folded into T6? **No: selection is its own file**; covered in Task 6.5 below); stack graph + 6 base rules + ADR-0092 test (T7); named actions + stack-parent kill + typed failures (T8); lease/claim/reclaim strict boundary (T9); ADR graph/next-ID/supersede/reverse + learning queries (T10); decoder distinctions + markers + wire structs (T11); BuildSnapshot + complete single-snapshot validation + corpus + accounting (T12); evolution byte rules (T13); boundary pin + mutation probes + suite gate (T14).
- **Gap found and fixed:** selection had no task — added as Task 6.5 below (numbering preserved to avoid renumbering later tasks).
- **Type consistency check:** `BranchFacts` defined in T7, consumed in T6/T8/T9 — consistent. `RecordLocation` defined in domain (T2), aliased in repository (T11) — consistent. `PolicyFailure` (T8) reused by T9/T10 — consistent. `domain.Finding` produced by repository passes (T11–T13) — consistent.

### Task 6.5: Selection (`internal/domain/selection.go`) — execute after Task 6

**Files:**
- Create: `internal/domain/selection.go`
- Test: `internal/domain/selection_test.go`

**Interfaces:**
- Consumes: `EvaluateReadiness` (T6), `priorityRank` (T1).
- Produces:

```go
type SelectionFilter struct {
    Types      []string   // empty = all
    Priorities []Priority // empty = all
}
// SelectQueue returns unambiguous build-ready changes ordered by:
// priority rank → well-formed created ascending → numeric ID ascending.
// Malformed/absent created sorts AFTER every valid date in its band.
// Unknown stored priority ranks as medium (finding is validation's job).
func SelectQueue(s Snapshot, facts BranchFacts, filter SelectionFilter) []Change
```

- [ ] **Step 1: Failing tests**: all four priorities interleave correctly; two changes same priority, earlier `created` first; malformed `created` (`FieldMalformed`) and absent both sort after every dated change in the same band, tie-broken by ID; unknown priority slots with medium; ID ascending as final tie-break; non-build-ready and ambiguous-ID changes excluded; type and priority filters; returned slice is fresh.
- [ ] **Step 2: Run** — FAIL. **Step 3: Implement** with `slices.SortStableFunc`. **Step 4: Run** `-count=1` — PASS.
- [ ] **Step 5: Commit** `feat(0307): deterministic selection queue with malformed-date and unknown-priority fallbacks`

**Execution order:** 1 → 2 → 3 → 4 → 5 → 7 → 6 → 6.5 → 8 → 9 → 10 → 11 → 12 → 13 → 14.
