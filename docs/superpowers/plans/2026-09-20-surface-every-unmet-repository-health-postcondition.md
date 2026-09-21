<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0418 — Surface every unmet repository health postcondition](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-21-0418-surface-every-unmet-repository-health-postcondition.md)**
<!-- docket:backlink:end -->
# Surface Every Unmet Repository Health Postcondition — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `docket repository check` reports every applicable unmet health condition — including the causes behind the generic `postconditions-unmet` fallback and independent failures masked by an earlier specific diagnosis — without changing classification precedence, reason tokens, exit codes, or read-only behavior.

**Architecture:** Factor the classifier's terminal healthy conjunction into one pure helper (`UnmetHealthConditions`) that both `Classify` and `EvaluateHealth` consume, so the diagnostic list can never drift from the healthy decision. Extend the committed-ignore probe to preserve the diagnostic detail it currently discards (missing entries, malformed markers, legacy-only, non-canonical, unreadable) via a pure explanatory helper beside `ValidGitignoreBlock`. `EvaluateHealth` then supplements the existing reason-based findings with per-condition findings, gated by applicability/prerequisite rules and deduplicated by the condition explained.

**Tech Stack:** Go, existing `internal/reposetup` + `internal/app` packages, existing test suites (pure Go tests plus the `//go:build integration` partition).

**Spec:** `docs/superpowers/specs/2026-09-20-surface-every-unmet-repository-health-postcondition-design.md` (on the `docket` metadata branch). Change: 0418.

## Global Constraints

- Bounded change: only `internal/reposetup` (`classify.go`, `health.go`, `gitignore.go`, new `healthconditions.go`, tests) and `internal/app` (`repository_check.go`, its unit/integration tests). No new commands, config, ADRs, probes, or subsystems.
- Classification precedence, reason tokens, state names, exit codes (0/1/2 via `CheckExit`), and read-only behavior are unchanged. Supplemental findings never change the selected state.
- `ValidGitignoreBlock` remains the sole acceptance authority: every input it accepts today stays accepted; only rejected inputs gain explanatory detail. No second file read.
- Unknown evidence is never reported as proven absence, foreign ownership, or an enabled setting. A probe that errored is "unverified" (warning), a proven wrong state is an error.
- A dependent condition suppressed for a missing prerequisite must be represented by that prerequisite's concrete finding; the generic `postconditions-unmet` finding never stands alone when `Classify` actually returns it.
- Findings dedupe by the condition explained — a dirty-worktree finding cannot suppress a missing ignore entry; a pending-review finding cannot suppress a hooks failure; naming `.gitignore` as a pending path does not explain its committed defect.
- Committed guarantees are proven from the integration COMMIT tree (learning `gitignore-guarantee-must-be-committed`); every remedy must be valid in the exact state that produced it (learning `printed-remedy-state-validity`).
- Every guard is mutation-tested: delete the population step, watch its assert redden (AGENTS.md "Guards and tests"; spec Verification item 2).
- Build gate: `go run ./cmd/docket development test` — the full suite, run from the feature worktree. Read the budget report even on green.

## File Structure

- Create: `internal/reposetup/healthconditions.go` — `HealthCondition` identifiers (fixed order) and `UnmetHealthConditions(Facts)`.
- Create: `internal/reposetup/healthconditions_test.go`
- Modify: `internal/reposetup/classify.go` — healthy branch delegates to `UnmetHealthConditions`.
- Modify: `internal/reposetup/gitignore.go` — `IgnoreDetail` / `ExplainGitignoreBlock` / `GitignoreEntries` / `CommittedIgnoreOutcome`.
- Modify: `internal/reposetup/gitignore_test.go`
- Modify: `internal/reposetup/probe.go` — `Facts.CommittedIgnoreDetail` field.
- Modify: `internal/reposetup/health.go` — supplemental condition findings inside `EvaluateHealth`; enriched `metadata-worktree-dirty` message.
- Modify: `internal/reposetup/health_test.go`
- Modify: `internal/app/repository_check.go` — `committedIgnorePresence` returns detail; `augmentCheckFacts` stores it.
- Modify: `internal/app/repository_check_test.go`
- Modify: `internal/app/repocheck_integration_test.go` — committed-file regression + baseline preservation.

---

### Task 1: `UnmetHealthConditions` — one evaluation for healthy and for diagnostics

**Files:**
- Create: `internal/reposetup/healthconditions.go`
- Create: `internal/reposetup/healthconditions_test.go`
- Modify: `internal/reposetup/classify.go` (the step-6 healthy conjunction)

**Interfaces:**
- Consumes: `Facts`, `Presence`, `RootShape`, `WorktreeFact` (existing, `internal/reposetup/probe.go`).
- Produces: `type HealthCondition string`; the 17 `Cond*` constants below; `func UnmetHealthConditions(f Facts) []HealthCondition` returning unmet conditions in the fixed declaration order (empty slice ⇔ healthy conjunction holds). Tasks 4–6 rely on these exact names.

- [ ] **Step 1: Write the failing tests**

`internal/reposetup/healthconditions_test.go`:

```go
package reposetup

import (
	"reflect"
	"testing"
)

// TestUnmetHealthConditionsHealthyIsEmpty pins the equivalence anchor: the
// healthy fixture yields no unmet condition.
func TestUnmetHealthConditionsHealthyIsEmpty(t *testing.T) {
	if got := UnmetHealthConditions(healthyFacts()); len(got) != 0 {
		t.Fatalf("UnmetHealthConditions(healthy) = %v, want empty", got)
	}
}

// TestUnmetHealthConditionsEachConjunct varies every conjunct of the healthy
// conjunction through a non-satisfying state and asserts exactly that
// condition is reported (spec Verification item 1). Unknown and Absent are
// both non-satisfying for Presence-valued conjuncts.
func TestUnmetHealthConditionsEachConjunct(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Facts)
		want   HealthCondition
	}{
		{"metadata absent", func(f *Facts) { f.RemoteMetadata.Presence = PresenceAbsent }, CondMetadataBranchPresent},
		{"metadata unknown", func(f *Facts) { f.RemoteMetadata.Presence = PresenceUnknown }, CondMetadataBranchPresent},
		{"root unknown", func(f *Facts) { f.MetadataRoot = RootUnknown }, CondMetadataRootVerified},
		{"root foreign", func(f *Facts) { f.MetadataRoot = RootForeign }, CondMetadataRootVerified},
		{"local metadata absent", func(f *Facts) { f.LocalMetadata.Presence = PresenceAbsent }, CondLocalMetadataPresent},
		{"local metadata unknown", func(f *Facts) { f.LocalMetadata.Presence = PresenceUnknown }, CondLocalMetadataPresent},
		{"worktree absent", func(f *Facts) { f.DocketWorktree.Presence = PresenceAbsent }, CondWorktreePresent},
		{"worktree unregistered", func(f *Facts) { f.DocketWorktree.Registered = PresenceAbsent }, CondWorktreeRegistered},
		{"worktree foreign", func(f *Facts) { f.DocketWorktree.Foreign = true }, CondWorktreeNotForeign},
		{"worktree dirty", func(f *Facts) { f.DocketWorktree.Clean = PresenceAbsent }, CondWorktreeClean},
		{"worktree unsynchronized", func(f *Facts) { f.DocketWorktree.Synchronized = PresenceAbsent }, CondWorktreeSynchronized},
		{"hooks enabled", func(f *Facts) { f.DocketWorktree.HooksOff = PresenceAbsent }, CondWorktreeHooksOff},
		{"hooks unknown", func(f *Facts) { f.DocketWorktree.HooksOff = PresenceUnknown }, CondWorktreeHooksOff},
		{"committed ignore invalid", func(f *Facts) { f.CommittedIgnoreBlock = PresenceAbsent }, CondCommittedIgnoreValid},
		{"committed ignore unknown", func(f *Facts) { f.CommittedIgnoreBlock = PresenceUnknown }, CondCommittedIgnoreValid},
		{"live surface present", func(f *Facts) { f.LiveSurface = PresencePresent }, CondLiveSurfaceAbsent},
		{"legacy key present", func(f *Facts) { f.LegacyConfigKey = PresencePresent }, CondLegacyConfigKeyAbsent},
		{"primary dirty", func(f *Facts) { f.PrimaryClean = PresenceAbsent }, CondPrimaryClean},
		{"primary off integration", func(f *Facts) { f.PrimaryOnIntegration = PresenceAbsent }, CondPrimaryOnIntegration},
		{"primary behind tip", func(f *Facts) { f.PrimaryAtRemoteTip = PresenceAbsent }, CondPrimaryAtRemoteTip},
		{"surfaces drift", func(f *Facts) { f.SurfacesAgree = PresenceAbsent }, CondSurfacesAgree},
		{"pending review paths", func(f *Facts) { f.PendingReviewPaths = []string{".gitignore"} }, CondNoPendingReviewPaths},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := healthyFacts()
			tc.mutate(&f)
			got := UnmetHealthConditions(f)
			if len(got) != 1 || got[0] != tc.want {
				t.Fatalf("UnmetHealthConditions = %v, want exactly [%s]", got, tc.want)
			}
		})
	}
}

// TestUnmetHealthConditionsSurfacesUnauthorized: an unauthorized surface
// declaration satisfies the surfaces conjunct regardless of SurfacesAgree,
// exactly like the classifier's `(!f.SurfacesAuthorized || ...)` disjunct.
func TestUnmetHealthConditionsSurfacesUnauthorized(t *testing.T) {
	f := healthyFacts()
	f.SurfacesAuthorized = false
	f.SurfacesAgree = PresenceAbsent
	if got := UnmetHealthConditions(f); len(got) != 0 {
		t.Fatalf("unauthorized surfaces must not be unmet, got %v", got)
	}
}

// TestUnmetHealthConditionsMultipleInFixedOrder: simultaneous failures are
// all reported, in the fixed declaration order.
func TestUnmetHealthConditionsMultipleInFixedOrder(t *testing.T) {
	f := healthyFacts()
	f.DocketWorktree.HooksOff = PresenceAbsent
	f.CommittedIgnoreBlock = PresenceAbsent
	f.PrimaryClean = PresenceAbsent
	want := []HealthCondition{CondWorktreeHooksOff, CondCommittedIgnoreValid, CondPrimaryClean}
	if got := UnmetHealthConditions(f); !reflect.DeepEqual(got, want) {
		t.Fatalf("UnmetHealthConditions = %v, want %v", got, want)
	}
}

// TestClassifyHealthyIffNoUnmetConditions is the drift guard tying the
// classifier's healthy verdict to the helper (learning
// duplicated-gate-copies-the-whole-predicate): for the healthy fixture and
// every single-conjunct mutation above, Classify says healthy exactly when
// UnmetHealthConditions is empty.
func TestClassifyHealthyIffNoUnmetConditions(t *testing.T) {
	probe := func(f Facts) {
		t.Helper()
		healthy := Classify(f).State == StateHealthy
		empty := len(UnmetHealthConditions(f)) == 0
		if healthy != empty {
			t.Fatalf("Classify healthy=%v but unmet-empty=%v for %+v", healthy, empty, f)
		}
	}
	probe(healthyFacts())
	f := healthyFacts()
	f.DocketWorktree.HooksOff = PresenceAbsent
	probe(f)
	g := healthyFacts()
	g.CommittedIgnoreBlock = PresenceUnknown
	probe(g)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/reposetup/ -run 'TestUnmetHealthConditions|TestClassifyHealthyIff' -count=1`
Expected: FAIL to compile — `UnmetHealthConditions`, `HealthCondition`, `Cond*` undefined.

- [ ] **Step 3: Implement `healthconditions.go` and refactor `Classify`**

`internal/reposetup/healthconditions.go`:

```go
package reposetup

// healthconditions.go — the healthy conjunction as data. UnmetHealthConditions
// is the ONE evaluation of the classifier's terminal healthy conjunction:
// Classify selects healthy exactly when it returns empty, and EvaluateHealth
// uses the same result to supplement reports, so the diagnostic list can never
// drift from the healthy decision (change 0418; learning
// duplicated-gate-copies-the-whole-predicate). It is pure and adds no
// conditions the classifier does not enforce.

// HealthCondition is a stable identifier for one conjunct of the healthy
// postcondition set.
type HealthCondition string

// The full conjunct roster, in the fixed order the healthy conjunction
// checks them. UnmetHealthConditions reports unmet conditions in this order.
const (
	CondMetadataBranchPresent HealthCondition = "metadata-branch-present"
	CondMetadataRootVerified  HealthCondition = "metadata-root-verified"
	CondLocalMetadataPresent  HealthCondition = "local-metadata-present"
	CondWorktreePresent       HealthCondition = "docket-worktree-present"
	CondWorktreeRegistered    HealthCondition = "docket-worktree-registered"
	CondWorktreeNotForeign    HealthCondition = "docket-worktree-not-foreign"
	CondWorktreeClean         HealthCondition = "docket-worktree-clean"
	CondWorktreeSynchronized  HealthCondition = "docket-worktree-synchronized"
	CondWorktreeHooksOff      HealthCondition = "docket-worktree-hooks-off"
	CondCommittedIgnoreValid  HealthCondition = "committed-ignore-valid"
	CondLiveSurfaceAbsent     HealthCondition = "live-surface-absent"
	CondLegacyConfigKeyAbsent HealthCondition = "legacy-config-key-absent"
	CondPrimaryClean          HealthCondition = "primary-clean"
	CondPrimaryOnIntegration  HealthCondition = "primary-on-integration"
	CondPrimaryAtRemoteTip    HealthCondition = "primary-at-remote-tip"
	CondSurfacesAgree         HealthCondition = "surfaces-agree"
	CondNoPendingReviewPaths  HealthCondition = "no-pending-review-paths"
)

// UnmetHealthConditions returns the healthy-conjunction conjuncts f does not
// satisfy, in fixed order. Empty means the conjunction holds. Unknown and
// Absent are both non-satisfying — a conjunct is met only when PROVEN met —
// but presentation (unknown vs proven-wrong) is EvaluateHealth's job, not
// this evaluation's.
func UnmetHealthConditions(f Facts) []HealthCondition {
	var unmet []HealthCondition
	add := func(ok bool, c HealthCondition) {
		if !ok {
			unmet = append(unmet, c)
		}
	}
	add(f.RemoteMetadata.Presence == PresencePresent, CondMetadataBranchPresent)
	add(f.MetadataRoot == RootParentless, CondMetadataRootVerified)
	add(f.LocalMetadata.Presence == PresencePresent, CondLocalMetadataPresent)
	add(f.DocketWorktree.Presence == PresencePresent, CondWorktreePresent)
	add(f.DocketWorktree.Registered == PresencePresent, CondWorktreeRegistered)
	add(!f.DocketWorktree.Foreign, CondWorktreeNotForeign)
	add(f.DocketWorktree.Clean == PresencePresent, CondWorktreeClean)
	add(f.DocketWorktree.Synchronized == PresencePresent, CondWorktreeSynchronized)
	add(f.DocketWorktree.HooksOff == PresencePresent, CondWorktreeHooksOff)
	add(f.CommittedIgnoreBlock == PresencePresent, CondCommittedIgnoreValid)
	add(f.LiveSurface == PresenceAbsent, CondLiveSurfaceAbsent)
	add(f.LegacyConfigKey == PresenceAbsent, CondLegacyConfigKeyAbsent)
	add(f.PrimaryClean == PresencePresent, CondPrimaryClean)
	add(f.PrimaryOnIntegration == PresencePresent, CondPrimaryOnIntegration)
	add(f.PrimaryAtRemoteTip == PresencePresent, CondPrimaryAtRemoteTip)
	add(!f.SurfacesAuthorized || f.SurfacesAgree == PresencePresent, CondSurfacesAgree)
	add(len(f.PendingReviewPaths) == 0, CondNoPendingReviewPaths)
	return unmet
}
```

In `internal/reposetup/classify.go`, replace the whole step-6 conjunction body (the `if f.RemoteMetadata.Presence == PresencePresent && ... { return Classification{State: StateHealthy} }` block) with:

```go
	// 6. Healthy: re-verify EVERY postcondition. UnmetHealthConditions is the
	// single evaluation of the conjunction (healthconditions.go); healthy is
	// selected exactly when it is empty. This is never a default — a single
	// unmet conjunct falls through to the terminal conflict below.
	if len(UnmetHealthConditions(f)) == 0 {
		return Classification{State: StateHealthy}
	}
```

Do not change any earlier ladder branch, any reason token, or the terminal `postconditions-unmet` fall-through.

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/reposetup/ -count=1`
Expected: PASS — including the whole existing `TestClassify` matrix, which proves the refactor is behavior-preserving.

- [ ] **Step 5: Mutation-test the equivalence guard**

Temporarily delete the `add(f.DocketWorktree.HooksOff == PresencePresent, CondWorktreeHooksOff)` line; run `go test ./internal/reposetup/ -run 'TestClassify|TestUnmetHealth' -count=1` and confirm it FAILS (both `TestUnmetHealthConditionsEachConjunct` and the existing `TestClassify` hooks case must redden — the second proves `Classify` really delegates). Restore the line from your editor buffer, never via `git checkout --` (learning `mutation-restore-needs-a-backup-copy`), and re-run to green.

- [ ] **Step 6: Commit**

```bash
git add internal/reposetup/healthconditions.go internal/reposetup/healthconditions_test.go internal/reposetup/classify.go
git commit -m "feat(reposetup): factor the healthy conjunction into UnmetHealthConditions (change 0418)"
```

---

### Task 2: Preserve ignore detail — `ExplainGitignoreBlock` and `CommittedIgnoreOutcome`

**Files:**
- Modify: `internal/reposetup/gitignore.go`
- Modify: `internal/reposetup/gitignore_test.go`
- Modify: `internal/reposetup/probe.go` (add one `Facts` field)

**Interfaces:**
- Consumes: `canonicalBlockBytes`, `ValidGitignoreBlock`, `gitignoreMarkersMalformed`, `hasLine`, `splitLines`, marker constants (all existing in `gitignore.go`); `Presence` values.
- Produces (Tasks 3–6 rely on these exact names):
  - `type IgnoreDefect int` with constants `IgnoreDefectNone, IgnoreDefectFileAbsent, IgnoreDefectBlockAbsent, IgnoreDefectLegacyOnly, IgnoreDefectMalformedMarkers, IgnoreDefectMissingEntries, IgnoreDefectNonCanonical, IgnoreDefectUnreadable`
  - `type IgnoreDetail struct { Defect IgnoreDefect; Generation string; MissingEntries []string }` (zero value = not probed / valid)
  - `func GitignoreEntries() []string` — canonical entries, canonical order
  - `func ExplainGitignoreBlock(fileBytes []byte) IgnoreDetail`
  - `func CommittedIgnoreOutcome(blob []byte, found bool, readErr error) (Presence, IgnoreDetail)`
  - New `Facts` field: `CommittedIgnoreDetail IgnoreDetail`

- [ ] **Step 1: Write the failing tests**

Append to `internal/reposetup/gitignore_test.go`:

```go
// --- change 0418: explanatory detail for inputs ValidGitignoreBlock rejects ---

func TestGitignoreEntriesDerivedFromCanonicalBlock(t *testing.T) {
	entries := GitignoreEntries()
	// Derived, not hand-listed: entries are exactly the canonical block's
	// interior lines, in order (AGENTS.md: never hand-list gated sites).
	lines := strings.Split(strings.TrimSuffix(string(GitignoreBlock()), "\n"), "\n")
	want := lines[1 : len(lines)-1]
	if !reflect.DeepEqual(entries, want) {
		t.Fatalf("GitignoreEntries() = %v, want interior of canonical block %v", entries, want)
	}
	if len(entries) == 0 || entries[0] != ".docket/" {
		t.Fatalf("canonical order lost: %v", entries)
	}
}

func TestExplainGitignoreBlockValidIsNone(t *testing.T) {
	for name, in := range map[string][]byte{
		"canonical alone":        GitignoreBlock(),
		"canonical with prefix":  append([]byte("node_modules/\n\n"), GitignoreBlock()...),
		"canonical with suffix":  append(GitignoreBlock(), []byte("\nextra/\n")...),
	} {
		if d := ExplainGitignoreBlock(in); d.Defect != IgnoreDefectNone {
			t.Fatalf("%s: Defect = %v, want None (validity predicate stays authoritative)", name, d.Defect)
		}
	}
}

// TestExplainGitignoreBlockAcceptancePinned: every input the acceptance
// predicate accepts must explain as None, so explanatory work cannot
// silently tighten validity (spec Verification item 3).
func TestExplainGitignoreBlockAcceptancePinned(t *testing.T) {
	// canonical block present + a malformed EXTRA block elsewhere: today's
	// predicate accepts it, and that stays governed by ValidGitignoreBlock.
	in := append(GitignoreBlock(), []byte("\n"+GitignoreEnd+"\n")...)
	if !ValidGitignoreBlock(in) {
		t.Fatalf("fixture drifted: predicate no longer accepts canonical+stray-end")
	}
	if d := ExplainGitignoreBlock(in); d.Defect != IgnoreDefectNone {
		t.Fatalf("accepted input explained as %v, want None", d.Defect)
	}
}

func TestExplainGitignoreBlockBlockAbsent(t *testing.T) {
	d := ExplainGitignoreBlock([]byte("node_modules/\n"))
	if d.Defect != IgnoreDefectBlockAbsent {
		t.Fatalf("Defect = %v, want BlockAbsent", d.Defect)
	}
}

func TestExplainGitignoreBlockLegacyOnly(t *testing.T) {
	in := []byte(legacyGitignoreStart + "\n.docket/\n" + legacyGitignoreEnd + "\n")
	d := ExplainGitignoreBlock(in)
	if d.Defect != IgnoreDefectLegacyOnly {
		t.Fatalf("Defect = %v, want LegacyOnly", d.Defect)
	}
}

func TestExplainGitignoreBlockMalformedMarkers(t *testing.T) {
	cases := map[string]struct {
		in         string
		generation string
	}{
		"dangling start":      {GitignoreStart + "\n.docket/\n", "docket"},
		"end before start":    {GitignoreEnd + "\n" + GitignoreStart + "\n", "docket"},
		"dangling legacy end": {legacyGitignoreEnd + "\n", "legacy"},
	}
	for name, tc := range cases {
		d := ExplainGitignoreBlock([]byte(tc.in))
		if d.Defect != IgnoreDefectMalformedMarkers || d.Generation != tc.generation {
			t.Fatalf("%s: got (%v, %q), want (MalformedMarkers, %q)", name, d.Defect, d.Generation, tc.generation)
		}
	}
}

// TestExplainGitignoreBlockMissingEntries covers the reported real-world
// regression (missing .opencode entry) plus multiple missing entries, all
// reported in canonical order. An entry elsewhere in the file does not
// satisfy membership in the managed block.
func TestExplainGitignoreBlockMissingEntries(t *testing.T) {
	strip := func(remove ...string) []byte {
		out := string(GitignoreBlock())
		for _, r := range remove {
			out = strings.Replace(out, r+"\n", "", 1)
		}
		return []byte(out)
	}
	d := ExplainGitignoreBlock(strip(".opencode/agents/docket-*.md"))
	if d.Defect != IgnoreDefectMissingEntries ||
		!reflect.DeepEqual(d.MissingEntries, []string{".opencode/agents/docket-*.md"}) {
		t.Fatalf("single missing: got (%v, %v)", d.Defect, d.MissingEntries)
	}
	d = ExplainGitignoreBlock(strip(".worktrees/", ".opencode/agents/docket-*.md"))
	if !reflect.DeepEqual(d.MissingEntries, []string{".worktrees/", ".opencode/agents/docket-*.md"}) {
		t.Fatalf("multiple missing not in canonical order: %v", d.MissingEntries)
	}
	// Entry outside the block does not count as membership.
	outside := append([]byte(".opencode/agents/docket-*.md\n\n"), strip(".opencode/agents/docket-*.md")...)
	d = ExplainGitignoreBlock(outside)
	if d.Defect != IgnoreDefectMissingEntries || len(d.MissingEntries) != 1 {
		t.Fatalf("outside-block entry satisfied membership: (%v, %v)", d.Defect, d.MissingEntries)
	}
}

// TestExplainGitignoreBlockNonCanonical: all entries present but not the
// exact canonical representation (reordered / extra interior line) explains
// the mismatch rather than inventing a missing entry.
func TestExplainGitignoreBlockNonCanonical(t *testing.T) {
	reordered := strings.Replace(string(GitignoreBlock()),
		".docket/\n.worktrees/\n", ".worktrees/\n.docket/\n", 1)
	d := ExplainGitignoreBlock([]byte(reordered))
	if d.Defect != IgnoreDefectNonCanonical || len(d.MissingEntries) != 0 {
		t.Fatalf("reordered: got (%v, %v), want (NonCanonical, none)", d.Defect, d.MissingEntries)
	}
	extra := strings.Replace(string(GitignoreBlock()),
		".docket/\n", ".docket/\nextra-line/\n", 1)
	if d := ExplainGitignoreBlock([]byte(extra)); d.Defect != IgnoreDefectNonCanonical {
		t.Fatalf("extra interior line: got %v, want NonCanonical", d.Defect)
	}
}

// TestCommittedIgnoreOutcome maps the probe's three raw results to
// (Presence, IgnoreDetail). A read error is Unknown+Unreadable, never a
// clean absence (learning probe-error-is-not-clean-absence).
func TestCommittedIgnoreOutcome(t *testing.T) {
	p, d := CommittedIgnoreOutcome(nil, false, errors.New("boom"))
	if p != PresenceUnknown || d.Defect != IgnoreDefectUnreadable {
		t.Fatalf("read error: got (%v, %v), want (Unknown, Unreadable)", p, d.Defect)
	}
	p, d = CommittedIgnoreOutcome(nil, false, nil)
	if p != PresenceAbsent || d.Defect != IgnoreDefectFileAbsent {
		t.Fatalf("file not found: got (%v, %v), want (Absent, FileAbsent)", p, d.Defect)
	}
	p, d = CommittedIgnoreOutcome(GitignoreBlock(), true, nil)
	if p != PresencePresent || d.Defect != IgnoreDefectNone {
		t.Fatalf("valid: got (%v, %v), want (Present, None)", p, d.Defect)
	}
	p, d = CommittedIgnoreOutcome([]byte("stuff\n"), true, nil)
	if p != PresenceAbsent || d.Defect != IgnoreDefectBlockAbsent {
		t.Fatalf("invalid: got (%v, %v), want (Absent, BlockAbsent)", p, d.Defect)
	}
}
```

Add `"errors"`, `"reflect"`, `"strings"` to that test file's imports if absent.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/reposetup/ -run 'TestExplainGitignore|TestGitignoreEntries|TestCommittedIgnoreOutcome' -count=1`
Expected: FAIL to compile — new symbols undefined.

- [ ] **Step 3: Implement in `gitignore.go`**

Append (reusing existing helpers; no changes to `ValidGitignoreBlock`, `EnsureGitignoreBlock`, or the canonical bytes):

```go
// --- change 0418: explanatory detail for the committed-ignore health probe ---

// IgnoreDefect classifies why a .gitignore's managed block failed the
// acceptance predicate. IgnoreDefectNone is the zero value and doubles as
// "valid or never probed"; consumers must not read it as a defect.
type IgnoreDefect int

const (
	IgnoreDefectNone IgnoreDefect = iota
	IgnoreDefectFileAbsent       // the committed .gitignore file does not exist
	IgnoreDefectBlockAbsent      // file readable, no current-generation managed block
	IgnoreDefectLegacyOnly       // only the legacy 0051 markers are present
	IgnoreDefectMalformedMarkers // dangling / out-of-order / nested markers
	IgnoreDefectMissingEntries   // well-formed block lacking canonical entries
	IgnoreDefectNonCanonical     // all entries present but not the exact canonical bytes
	IgnoreDefectUnreadable       // the committed blob could not be read
)

// IgnoreDetail is the small diagnostic payload the committed-ignore probe
// preserves alongside its unchanged three-valued Presence. Only inputs the
// acceptance predicate rejects carry a non-None defect.
type IgnoreDetail struct {
	Defect         IgnoreDefect
	Generation     string   // for MalformedMarkers: "docket" or "legacy"
	MissingEntries []string // for MissingEntries: canonical order
}

// GitignoreEntries returns the canonical managed entries (the block's
// interior lines) in canonical order, derived from the one canonical
// definition rather than a second hand-kept list.
func GitignoreEntries() []string {
	lines := splitLines(canonicalBlockBytes)
	entries := make([]string, 0, len(lines)-2)
	for _, line := range lines[1 : len(lines)-1] {
		entries = append(entries, string(line))
	}
	return entries
}

// ExplainGitignoreBlock explains why fileBytes fails ValidGitignoreBlock.
// The acceptance predicate remains authoritative: any input it accepts
// explains as None, so this helper can never tighten validity. It is pure
// and performs no general Git-ignore semantics analysis.
func ExplainGitignoreBlock(fileBytes []byte) IgnoreDetail {
	if ValidGitignoreBlock(fileBytes) {
		return IgnoreDetail{}
	}
	if gitignoreMarkersMalformed(fileBytes, GitignoreStart, GitignoreEnd) {
		return IgnoreDetail{Defect: IgnoreDefectMalformedMarkers, Generation: "docket"}
	}
	if gitignoreMarkersMalformed(fileBytes, legacyGitignoreStart, legacyGitignoreEnd) {
		return IgnoreDetail{Defect: IgnoreDefectMalformedMarkers, Generation: "legacy"}
	}
	if !hasLine(fileBytes, GitignoreStart) {
		if hasLine(fileBytes, legacyGitignoreStart) {
			return IgnoreDetail{Defect: IgnoreDefectLegacyOnly}
		}
		return IgnoreDetail{Defect: IgnoreDefectBlockAbsent}
	}
	// A well-formed current-generation block exists but is not canonical:
	// membership is judged against the BLOCK's own lines, so an entry
	// elsewhere in the file does not satisfy it.
	member := map[string]bool{}
	in := false
	for _, line := range splitLines(fileBytes) {
		switch string(line) {
		case GitignoreStart:
			in = true
		case GitignoreEnd:
			in = false
		default:
			if in {
				member[string(line)] = true
			}
		}
	}
	var missing []string
	for _, e := range GitignoreEntries() {
		if !member[e] {
			missing = append(missing, e)
		}
	}
	if len(missing) > 0 {
		return IgnoreDetail{Defect: IgnoreDefectMissingEntries, MissingEntries: missing}
	}
	return IgnoreDetail{Defect: IgnoreDefectNonCanonical}
}

// CommittedIgnoreOutcome maps a committed-blob read result to the presence
// fact and its preserved detail — the pure core of the app-layer probe. A
// read error is Unknown+Unreadable, never a fabricated absence (learning
// probe-error-is-not-clean-absence).
func CommittedIgnoreOutcome(blob []byte, found bool, readErr error) (Presence, IgnoreDetail) {
	if readErr != nil {
		return PresenceUnknown, IgnoreDetail{Defect: IgnoreDefectUnreadable}
	}
	if !found {
		return PresenceAbsent, IgnoreDetail{Defect: IgnoreDefectFileAbsent}
	}
	if ValidGitignoreBlock(blob) {
		return PresencePresent, IgnoreDetail{}
	}
	return PresenceAbsent, ExplainGitignoreBlock(blob)
}
```

In `internal/reposetup/probe.go`, add one field to `Facts`, directly under `CommittedIgnoreBlock`:

```go
	CommittedIgnoreDetail IgnoreDetail // why the committed block failed; zero when valid or never probed
```

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/reposetup/ -count=1`
Expected: PASS (all existing gitignore tests untouched and green).

- [ ] **Step 5: Commit**

```bash
git add internal/reposetup/gitignore.go internal/reposetup/gitignore_test.go internal/reposetup/probe.go
git commit -m "feat(reposetup): preserve committed-ignore diagnostic detail at the read boundary (change 0418)"
```

---

### Task 3: Supplemental condition findings in `EvaluateHealth`

**Files:**
- Modify: `internal/reposetup/health.go`
- Modify: `internal/reposetup/health_test.go`

**Interfaces:**
- Consumes: `UnmetHealthConditions`, the `Cond*` constants (Task 1); `IgnoreDetail` / `IgnoreDefect*` and `Facts.CommittedIgnoreDetail` (Task 2); existing `Finding`, `findingFor`, `categoryOf`, category constants, `EvaluateHealth` (unchanged signature).
- Produces (Task 4–5 assert against these): supplemental `Finding` values with the codes listed in Step 3; enriched `metadata-worktree-dirty` message; unexported `supplementalConditionFindings(c Classification, f Facts) []Finding` and `conditionFinding(cond HealthCondition, f Facts) *Finding`.

Design rules this task implements (from the spec, restated here so the implementer needs no other context):

- Supplemental reporting runs only when `f.RemoteMetadata.Presence == PresencePresent` (matching `RunRepositoryCheck`'s augmentation boundary) — fresh/legacy/metadata-unknown repositories keep their existing reports untouched.
- Dedupe by the condition explained: a reason token already in `c.Reasons` suppresses exactly the conditions it explains (map in Step 3), nothing else.
- Prerequisite grouping: a dependent condition whose prerequisite is unresolved is represented by the prerequisite's own concrete finding, never by a fabricated dependent failure. The committed-tree conditions (`committed-ignore-valid`, `legacy-config-key-absent`) require a resolved pinned integration commit — in Facts terms `f.RemoteIntegration.Presence == PresencePresent && f.RemoteIntegration.Tip != ""`; when unresolved, the existing unknown-authority diagnostics already explain them, so return nil. When the read WAS attempted and failed (Unknown presence with the prerequisite met), report "unverified".
- Unknown is never reported as a proven wrong state: unverified findings are `SeverityWarning`; proven defects are `SeverityError`.
- The generic `postconditions-unmet` finding stays, and stays only when the classifier actually selected that reason; it must never stand alone.

- [ ] **Step 1: Write the failing tests**

Append to `internal/reposetup/health_test.go`:

```go
// --- change 0418: supplemental condition findings ---

func findingCodes(fs []Finding) []string {
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		out = append(out, f.Code)
	}
	return out
}

func hasCode(fs []Finding, code string) bool {
	for _, f := range fs {
		if f.Code == code {
			return true
		}
	}
	return false
}

// TestHealthTerminalFallbackNamesEveryCause: the approved mixed-failure
// shape — hooks enabled AND committed ignore missing an entry — surfaces
// BOTH causes beside the retained generic finding, with unchanged state.
func TestHealthTerminalFallbackNamesEveryCause(t *testing.T) {
	f := healthyFacts()
	f.DocketWorktree.HooksOff = PresenceAbsent
	f.CommittedIgnoreBlock = PresenceAbsent
	f.CommittedIgnoreDetail = IgnoreDetail{
		Defect:         IgnoreDefectMissingEntries,
		MissingEntries: []string{".opencode/agents/docket-*.md"},
	}
	c := Classify(f)
	if c.State != StateConflict || c.Reasons[0] != "postconditions-unmet" {
		t.Fatalf("classification changed: %+v", c)
	}
	got := EvaluateHealth(c, f, nil)
	for _, code := range []string{"postconditions-unmet", "docket-worktree-hooks-enabled", "committed-ignore-invalid"} {
		if !hasCode(got, code) {
			t.Fatalf("missing %q in %v", code, findingCodes(got))
		}
	}
	for _, fn := range got {
		if fn.Code == "committed-ignore-invalid" {
			if fn.Ref != ".gitignore" {
				t.Fatalf("ignore finding ref = %q, want .gitignore", fn.Ref)
			}
			if !strings.Contains(fn.Message, ".opencode/agents/docket-*.md") ||
				!strings.Contains(fn.Message, "committed") {
				t.Fatalf("ignore message lacks entry or committed-tree wording: %q", fn.Message)
			}
			if !strings.Contains(fn.Remedy, ".opencode/agents/docket-*.md") {
				t.Fatalf("ignore remedy does not name the entry: %q", fn.Remedy)
			}
		}
	}
	if CheckExit(c, got) != 1 {
		t.Fatalf("exit changed")
	}
}

// TestHealthGenericFindingNeverAlone: whenever Classify returns the
// postconditions-unmet reason, EvaluateHealth accompanies the generic
// finding with at least one specific one (spec: it must never stand alone).
func TestHealthGenericFindingNeverAlone(t *testing.T) {
	mutations := []func(*Facts){
		func(f *Facts) { f.MetadataRoot = RootUnknown },
		func(f *Facts) { f.LocalMetadata = BranchFact{Presence: PresenceAbsent} },
		func(f *Facts) { f.DocketWorktree.Presence = PresenceAbsent; f.DocketWorktree.Registered = PresenceAbsent; f.DocketWorktree.Clean = PresenceUnknown; f.DocketWorktree.Synchronized = PresenceUnknown; f.DocketWorktree.HooksOff = PresenceUnknown },
		func(f *Facts) { f.DocketWorktree.Registered = PresenceAbsent },
		func(f *Facts) { f.DocketWorktree.Clean = PresenceUnknown },
		func(f *Facts) { f.DocketWorktree.HooksOff = PresenceAbsent },
		func(f *Facts) { f.CommittedIgnoreBlock = PresenceAbsent; f.CommittedIgnoreDetail = IgnoreDetail{Defect: IgnoreDefectBlockAbsent} },
		func(f *Facts) { f.CommittedIgnoreBlock = PresenceUnknown; f.CommittedIgnoreDetail = IgnoreDetail{Defect: IgnoreDefectUnreadable} },
		func(f *Facts) { f.LiveSurface = PresencePresent; f.MetadataRoot = RootUnknown },
		func(f *Facts) { f.LegacyConfigKey = PresencePresent },
		func(f *Facts) { f.PrimaryClean = PresenceAbsent },
		func(f *Facts) { f.PrimaryOnIntegration = PresenceAbsent },
		func(f *Facts) { f.PrimaryAtRemoteTip = PresenceAbsent },
	}
	for i, m := range mutations {
		f := healthyFacts()
		m(&f)
		c := Classify(f)
		if c.State != StateConflict || len(c.Reasons) != 1 || c.Reasons[0] != "postconditions-unmet" {
			t.Fatalf("case %d: expected terminal fallback, got %+v", i, c)
		}
		got := EvaluateHealth(c, f, nil)
		if !hasCode(got, "postconditions-unmet") || len(got) < 2 {
			t.Fatalf("case %d: generic finding stands alone or missing: %v", i, findingCodes(got))
		}
	}
}

// TestHealthDirtyWorktreePlusIgnoreDefect pins approved example 1: a dirty
// metadata worktree (specific conflict diagnosis) does not suppress an
// independent committed-ignore defect. State and reasons are unchanged.
func TestHealthDirtyWorktreePlusIgnoreDefect(t *testing.T) {
	f := healthyFacts()
	f.DocketWorktree.Clean = PresenceAbsent
	f.CommittedIgnoreBlock = PresenceAbsent
	f.CommittedIgnoreDetail = IgnoreDetail{
		Defect:         IgnoreDefectMissingEntries,
		MissingEntries: []string{".opencode/agents/docket-*.md"},
	}
	c := Classify(f)
	if c.State != StateConflict || c.Reasons[0] != "metadata-worktree-dirty" {
		t.Fatalf("classification changed: %+v", c)
	}
	got := EvaluateHealth(c, f, nil)
	if !hasCode(got, "metadata-worktree-dirty") || !hasCode(got, "committed-ignore-invalid") {
		t.Fatalf("dirty finding suppressed the ignore defect: %v", findingCodes(got))
	}
	if hasCode(got, "postconditions-unmet") {
		t.Fatalf("generic finding added to a specific-reason report: %v", findingCodes(got))
	}
	// The dirty finding covers the clean/synchronized alternatives: no
	// duplicate per-condition finding for either.
	if hasCode(got, "docket-worktree-clean-unverified") {
		t.Fatalf("duplicate explanation of the clean condition: %v", findingCodes(got))
	}
	if CheckExit(c, got) != 1 {
		t.Fatalf("exit changed")
	}
}

// TestHealthPendingReviewPlusHooksOff pins approved example 2: pending setup
// edits (needs-review) do not hide enabled metadata-worktree hooks — and
// naming .gitignore as a pending path does not explain its committed defect.
func TestHealthPendingReviewPlusHooksOff(t *testing.T) {
	f := healthyFacts()
	f.PendingReviewPaths = []string{".gitignore"}
	f.CommittedIgnoreBlock = PresenceAbsent
	f.CommittedIgnoreDetail = IgnoreDetail{Defect: IgnoreDefectBlockAbsent}
	f.DocketWorktree.HooksOff = PresenceAbsent
	c := Classify(f)
	if c.State != StateNeedsReview {
		t.Fatalf("classification changed: %+v", c)
	}
	got := EvaluateHealth(c, f, nil)
	for _, code := range []string{"pending-review-paths", "docket-worktree-hooks-enabled", "committed-ignore-invalid"} {
		if !hasCode(got, code) {
			t.Fatalf("missing %q: %v", code, findingCodes(got))
		}
	}
	if CheckExit(c, got) != 1 {
		t.Fatalf("exit changed")
	}
}

// TestHealthPartialPlusIndependentIgnoreDefect: a partial/migration diagnosis
// explains its live-surface condition but not an independent ignore defect.
func TestHealthPartialPlusIndependentIgnoreDefect(t *testing.T) {
	f := healthyFacts()
	f.LiveSurface = PresencePresent // -> metadata-seeded-live-surface (partial)
	f.CommittedIgnoreBlock = PresenceAbsent
	f.CommittedIgnoreDetail = IgnoreDetail{Defect: IgnoreDefectLegacyOnly}
	c := Classify(f)
	if c.State != StatePartial {
		t.Fatalf("classification changed: %+v", c)
	}
	got := EvaluateHealth(c, f, nil)
	if !hasCode(got, "migration-incomplete") || !hasCode(got, "committed-ignore-invalid") {
		t.Fatalf("partial suppressed the ignore defect: %v", findingCodes(got))
	}
	if hasCode(got, "live-surface-present") {
		t.Fatalf("live-surface double-explained beside the partial diagnosis: %v", findingCodes(got))
	}
}

// TestHealthUnknownOwnershipIsNotForeign: unresolved ownership reports as
// unverified (warning), never as foreign, and does not suppress a known
// independent defect.
func TestHealthUnknownOwnershipIsNotForeign(t *testing.T) {
	f := healthyFacts()
	f.MetadataRoot = RootUnknown
	f.DocketWorktree.HooksOff = PresenceAbsent
	got := EvaluateHealth(Classify(f), f, nil)
	if !hasCode(got, "metadata-ownership-unverified") || !hasCode(got, "docket-worktree-hooks-enabled") {
		t.Fatalf("codes: %v", findingCodes(got))
	}
	for _, fn := range got {
		if fn.Code == "metadata-ownership-unverified" {
			if fn.Severity != SeverityWarning || strings.Contains(strings.ToLower(fn.Message), "foreign") {
				t.Fatalf("unknown ownership misreported: %+v", fn)
			}
		}
	}
}

// TestHealthMissingWorktreeNoCascade: a missing .docket worktree is the
// blocking observation; its dependent registration/clean/hooks conditions
// must not cascade into separate "independently checked" findings.
func TestHealthMissingWorktreeNoCascade(t *testing.T) {
	f := healthyFacts()
	f.DocketWorktree = WorktreeFact{
		Presence: PresenceAbsent, Registered: PresenceAbsent,
		Clean: PresenceUnknown, Synchronized: PresenceUnknown, HooksOff: PresenceUnknown,
	}
	got := EvaluateHealth(Classify(f), f, nil)
	if !hasCode(got, "docket-worktree-missing") {
		t.Fatalf("blocking observation absent: %v", findingCodes(got))
	}
	for _, banned := range []string{
		"docket-worktree-unregistered", "docket-worktree-registration-unverified",
		"docket-worktree-clean-unverified", "docket-worktree-hooks-enabled",
		"docket-worktree-hooks-unverified",
	} {
		if hasCode(got, banned) {
			t.Fatalf("dependent cascade %q behind a missing worktree: %v", banned, findingCodes(got))
		}
	}
}

// TestHealthUnreadableIgnoreIsUnverified: a failed committed-blob read is
// "cannot verify" (warning), never a claim the file or an entry is missing.
func TestHealthUnreadableIgnoreIsUnverified(t *testing.T) {
	f := healthyFacts()
	f.CommittedIgnoreBlock = PresenceUnknown
	f.CommittedIgnoreDetail = IgnoreDetail{Defect: IgnoreDefectUnreadable}
	got := EvaluateHealth(Classify(f), f, nil)
	if !hasCode(got, "committed-ignore-unverified") || hasCode(got, "committed-ignore-invalid") {
		t.Fatalf("codes: %v", findingCodes(got))
	}
	for _, fn := range got {
		if fn.Code == "committed-ignore-unverified" {
			if fn.Severity != SeverityWarning || strings.Contains(strings.ToLower(fn.Message), "missing") {
				t.Fatalf("unreadable misreported as absence: %+v", fn)
			}
		}
	}
}

// TestHealthCommittedProbesSuppressedWithoutIntegrationTip: with the pinned
// integration commit unresolved, the committed-tree conditions keep their
// prerequisite's existing diagnostic instead of fabricating child findings.
func TestHealthCommittedProbesSuppressedWithoutIntegrationTip(t *testing.T) {
	f := healthyFacts()
	f.RemoteIntegration = BranchFact{Presence: PresenceUnknown}
	f.CommittedIgnoreBlock = PresenceUnknown
	f.LegacyConfigKey = PresenceUnknown
	c := Classify(f)
	if c.State != StateUnknown {
		t.Fatalf("classification changed: %+v", c)
	}
	got := EvaluateHealth(c, f, nil)
	if !hasCode(got, "remote-integration-unknown") {
		t.Fatalf("prerequisite diagnostic missing: %v", findingCodes(got))
	}
	for _, banned := range []string{"committed-ignore-invalid", "committed-ignore-unverified", "legacy-config-key-present", "legacy-config-key-unverified"} {
		if hasCode(got, banned) {
			t.Fatalf("skipped read reported as %q: %v", banned, findingCodes(got))
		}
	}
	if CheckExit(c, got) != 2 {
		t.Fatalf("unknown exit changed")
	}
}

// TestHealthUnknownAuthorityKeepsIndependentDefects: an unknown-authority
// report includes independently established defects whose prerequisites are
// met, while state and exit stay unknown/2.
func TestHealthUnknownAuthorityKeepsIndependentDefects(t *testing.T) {
	f := healthyFacts()
	f.RemoteDefaultBranch = BranchFact{Presence: PresenceUnknown}
	f.DocketWorktree.HooksOff = PresenceAbsent
	c := Classify(f)
	if c.State != StateUnknown {
		t.Fatalf("classification changed: %+v", c)
	}
	got := EvaluateHealth(c, f, nil)
	if !hasCode(got, "docket-worktree-hooks-enabled") {
		t.Fatalf("independent defect dropped in unknown state: %v", findingCodes(got))
	}
	if CheckExit(c, got) != 2 {
		t.Fatalf("unknown exit changed")
	}
}

// TestHealthFreshAndLegacyUnchanged: no supplemental reporting without a
// proven-present metadata branch — a fresh/legacy repository is not told
// about worktree hooks or ignore entries the check never inspected.
func TestHealthFreshAndLegacyUnchanged(t *testing.T) {
	fresh := Facts{
		RemoteConfigured:    PresencePresent,
		RemoteDefaultBranch: BranchFact{Presence: PresencePresent, Tip: "d"},
		RemoteIntegration:   BranchFact{Presence: PresencePresent, Tip: "i"},
		RemoteMetadata:      BranchFact{Presence: PresenceAbsent},
		LiveSurface:         PresenceAbsent,
	}
	got := EvaluateHealth(Classify(fresh), fresh, nil)
	if len(got) != 1 || got[0].Code != "repository-uninitialized" {
		t.Fatalf("fresh report changed: %v", findingCodes(got))
	}
	legacy := fresh
	legacy.LiveSurface = PresencePresent
	got = EvaluateHealth(Classify(legacy), legacy, nil)
	if len(got) != 1 || got[0].Code != "legacy-repository" {
		t.Fatalf("legacy report changed: %v", findingCodes(got))
	}
}

// TestHealthUnauthorizedSurfacesNoFinding: surfaces produce no finding when
// SurfacesAuthorized is false, whatever SurfacesAgree holds.
func TestHealthUnauthorizedSurfacesNoFinding(t *testing.T) {
	f := healthyFacts()
	f.SurfacesAuthorized = false
	f.SurfacesAgree = PresenceAbsent
	f.DocketWorktree.HooksOff = PresenceAbsent // keep the state non-healthy
	got := EvaluateHealth(Classify(f), f, nil)
	for _, fn := range got {
		if strings.HasPrefix(fn.Code, "surfaces-") {
			t.Fatalf("unauthorized surfaces produced %q", fn.Code)
		}
	}
}

// TestHealthDirtyMessageNamesObservedAlternatives: the metadata-worktree-dirty
// finding identifies which alternative(s) were actually observed.
func TestHealthDirtyMessageNamesObservedAlternatives(t *testing.T) {
	onlySync := healthyFacts()
	onlySync.DocketWorktree.Synchronized = PresenceAbsent
	got := EvaluateHealth(Classify(onlySync), onlySync, nil)
	found := false
	for _, fn := range got {
		if fn.Code == "metadata-worktree-dirty" {
			found = true
			if !strings.Contains(fn.Message, "not synchronized") || strings.Contains(fn.Message, "uncommitted") {
				t.Fatalf("message does not identify the observed alternative: %q", fn.Message)
			}
		}
	}
	if !found {
		t.Fatalf("dirty finding missing: %v", findingCodes(got))
	}
}

// TestHealthSupplementalDeterministicOrder: supplemental findings land in
// the existing category order and are stable across runs.
func TestHealthSupplementalDeterministicOrder(t *testing.T) {
	f := healthyFacts()
	f.DocketWorktree.HooksOff = PresenceAbsent
	f.CommittedIgnoreBlock = PresenceAbsent
	f.CommittedIgnoreDetail = IgnoreDetail{Defect: IgnoreDefectBlockAbsent}
	c := Classify(f)
	first := findingCodes(EvaluateHealth(c, f, nil))
	for i := 0; i < 5; i++ {
		if again := findingCodes(EvaluateHealth(c, f, nil)); !reflect.DeepEqual(first, again) {
			t.Fatalf("order not deterministic: %v vs %v", first, again)
		}
	}
	// Integration-tree bucket (ignore) precedes local-worktree bucket (hooks).
	ignoreIdx, hooksIdx := -1, -1
	for i, code := range first {
		if code == "committed-ignore-invalid" {
			ignoreIdx = i
		}
		if code == "docket-worktree-hooks-enabled" {
			hooksIdx = i
		}
	}
	if ignoreIdx < 0 || hooksIdx < 0 || ignoreIdx > hooksIdx {
		t.Fatalf("category order broken: %v", first)
	}
}

// TestHealthHealthyStillEmptyWithDetailField: the healthy baseline is
// unchanged by the new machinery.
func TestHealthHealthyStillEmptyWithDetailField(t *testing.T) {
	f := healthyFacts()
	if got := EvaluateHealth(Classify(f), f, nil); len(got) != 0 {
		t.Fatalf("healthy no longer empty: %v", findingCodes(got))
	}
}
```

Add `"reflect"` and `"strings"` to the test file imports if absent.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/reposetup/ -run TestHealth -count=1`
Expected: FAIL — the new tests fail on missing supplemental codes (compile passes since Tasks 1–2 landed; the assertions themselves redden).

- [ ] **Step 3: Implement supplemental findings in `health.go`**

(a) Enrich the `metadata-worktree-dirty` case in `findingFor` — replace its `Message` line with:

```go
	case "metadata-worktree-dirty":
		msg := "The .docket metadata worktree has uncommitted or unsynchronized changes."
		switch {
		case f.DocketWorktree.Clean == PresenceAbsent && f.DocketWorktree.Synchronized == PresenceAbsent:
			msg = "The .docket metadata worktree has uncommitted changes and is not synchronized with the remote docket tip."
		case f.DocketWorktree.Clean == PresenceAbsent:
			msg = "The .docket metadata worktree has uncommitted or untracked changes."
		case f.DocketWorktree.Synchronized == PresenceAbsent:
			msg = "The .docket metadata worktree is not synchronized with the remote docket tip."
		}
		return Finding{
			Code:     "metadata-worktree-dirty",
			Severity: SeverityError,
			Ref:      ".docket",
			Message:  msg,
			Remedy:   "Commit or inspect the changes in the .docket metadata worktree before any repository operation; leave them in place.",
		}
```

(The remedy string is unchanged — `TestHealthRemedyDirtyMetadataWorktree` pins it.)

(b) In `EvaluateHealth`, after the existing reason loop and before the bucket flush, insert:

```go
	for _, fnd := range supplementalConditionFindings(c, f) {
		buckets[conditionCategory(fnd.Code)] = append(buckets[conditionCategory(fnd.Code)], fnd)
	}
```

(c) Append the new machinery:

```go
// reasonExplains maps each classifier reason token to the health conditions
// it already explains, so a supplemental finding is suppressed exactly when
// an existing finding covers its condition — dedupe is by the condition
// explained, never by state or category. Deliberately NOT listed:
// pending-review-paths does not explain committed-ignore-valid (naming
// .gitignore as pending does not explain its committed defect).
var reasonExplains = map[string][]HealthCondition{
	"metadata-root-foreign":                {CondMetadataRootVerified},
	"docket-dir-foreign":                   {CondWorktreeNotForeign, CondWorktreeRegistered, CondWorktreeClean, CondWorktreeSynchronized, CondWorktreeHooksOff},
	"metadata-worktree-dirty":              {CondWorktreeClean, CondWorktreeSynchronized},
	"local-metadata-diverged":              {CondWorktreeSynchronized},
	"pending-review-paths":                 {CondNoPendingReviewPaths},
	"metadata-seeded":                      {CondLiveSurfaceAbsent},
	"metadata-seeded-live-surface":         {CondLiveSurfaceAbsent},
	"integration-pruned-attach-incomplete": {CondLocalMetadataPresent, CondWorktreePresent, CondWorktreeRegistered},
	"surfaces-drift":                       {CondSurfacesAgree},
}

// supplementalConditionFindings adds one finding per applicable unmet health
// condition an existing reason-based finding does not already explain. It
// runs only when the remote metadata branch is proven present (the same
// boundary RunRepositoryCheck augments behind), so fresh/legacy repositories
// and gather failures keep their existing reports. It never changes the
// classification — it explains it.
func supplementalConditionFindings(c Classification, f Facts) []Finding {
	if f.RemoteMetadata.Presence != PresencePresent {
		return nil
	}
	unmet := UnmetHealthConditions(f)
	if len(unmet) == 0 {
		return nil
	}
	explained := map[HealthCondition]bool{}
	for _, r := range c.Reasons {
		for _, cond := range reasonExplains[r] {
			explained[cond] = true
		}
	}
	var out []Finding
	for _, cond := range unmet {
		if explained[cond] {
			continue
		}
		if fnd := conditionFinding(cond, f); fnd != nil {
			out = append(out, *fnd)
		}
	}
	return out
}

// integrationResolved reports whether the pinned integration commit was
// resolvable — the prerequisite of every committed-tree probe. When it is
// false those probes never ran, and the unknown-authority diagnostics
// already explain the gap.
func integrationResolved(f Facts) bool {
	return f.RemoteIntegration.Presence == PresencePresent && f.RemoteIntegration.Tip != ""
}

// worktreeInspectable reports whether the .docket worktree's dependent facts
// (registration, cleanliness, synchronization, hooks) were meaningfully
// probed: a missing or foreign path is itself the blocking observation.
func worktreeInspectable(f Facts) bool {
	return f.DocketWorktree.Presence == PresencePresent && !f.DocketWorktree.Foreign
}

// conditionFinding builds the supplemental finding for one unmet condition,
// or nil when a missing prerequisite's own finding already represents it
// (explanatory grouping, never permission to drop an unexplained failure).
// Unknown evidence yields an "unverified" warning; a proven wrong state
// yields an error. Every remedy fits the observed state and preserves local
// work (learning printed-remedy-state-validity).
func conditionFinding(cond HealthCondition, f Facts) *Finding {
	switch cond {
	case CondMetadataBranchPresent:
		return nil // the supplemental gate requires it proven present
	case CondMetadataRootVerified:
		// RootForeign is explained by the metadata-root-foreign reason; here
		// only RootUnknown remains: unresolved, never foreign.
		if f.MetadataRoot != RootUnknown {
			return nil
		}
		return &Finding{
			Code:     "metadata-ownership-unverified",
			Severity: SeverityWarning,
			Message:  "The remote docket branch's ownership proof could not be resolved (unverified, not proven wrong).",
			Remedy:   "Ensure the remote is reachable and its objects fetchable, then re-run `docket repository check`.",
		}
	case CondLocalMetadataPresent:
		if f.LocalMetadata.Presence == PresenceAbsent {
			return &Finding{
				Code:     "local-metadata-missing",
				Severity: SeverityError,
				Message:  "No local docket branch exists for the present remote docket branch.",
				Remedy:   "Run `docket repository migrate` to restore the local metadata attachment; it is idempotent.",
			}
		}
		return &Finding{
			Code:     "local-metadata-unverified",
			Severity: SeverityWarning,
			Message:  "The local docket branch could not be resolved (unverified, not proven absent).",
			Remedy:   "Re-run `docket repository check` once local Git reads succeed.",
		}
	case CondWorktreePresent:
		if f.DocketWorktree.Presence == PresenceAbsent {
			return &Finding{
				Code:     "docket-worktree-missing",
				Severity: SeverityError,
				Ref:      ".docket",
				Message:  "The .docket metadata worktree is missing; its registration, cleanliness, and hooks state cannot be established until it exists.",
				Remedy:   "Run `docket repository migrate` to restore the .docket worktree attachment; it is idempotent.",
			}
		}
		return &Finding{
			Code:     "docket-worktree-unverified",
			Severity: SeverityWarning,
			Ref:      ".docket",
			Message:  "The .docket path could not be inspected (unverified, not proven absent); its dependent state cannot be established.",
			Remedy:   "Restore read access to the .docket path, then re-run `docket repository check`.",
		}
	case CondWorktreeNotForeign:
		return nil // always explained by the docket-dir-foreign conflict reason
	case CondWorktreeRegistered:
		if !worktreeInspectable(f) {
			return nil // the worktree-present/foreign finding is the blocking observation
		}
		if f.DocketWorktree.Registered == PresenceAbsent {
			return &Finding{
				Code:     "docket-worktree-unregistered",
				Severity: SeverityError,
				Ref:      ".docket",
				Message:  "The .docket path exists but is not a registered worktree of this repository.",
				Remedy:   "Inspect the .docket path and resolve its registration manually with a human; leave its contents in place.",
			}
		}
		return &Finding{
			Code:     "docket-worktree-registration-unverified",
			Severity: SeverityWarning,
			Ref:      ".docket",
			Message:  "The .docket worktree registration could not be resolved (unverified, not proven foreign).",
			Remedy:   "Re-run `docket repository check` once `git worktree list` succeeds.",
		}
	case CondWorktreeClean:
		if !worktreeInspectable(f) {
			return nil
		}
		// Absent is explained by the metadata-worktree-dirty reason when the
		// classifier selected it; in states where it did not (it always does
		// for a present worktree), only Unknown remains applicable.
		if f.DocketWorktree.Clean != PresenceUnknown {
			return nil
		}
		return &Finding{
			Code:     "docket-worktree-clean-unverified",
			Severity: SeverityWarning,
			Ref:      ".docket",
			Message:  "The .docket worktree's cleanliness could not be resolved (unverified, not proven dirty).",
			Remedy:   "Re-run `docket repository check` once the .docket status read succeeds.",
		}
	case CondWorktreeSynchronized:
		if !worktreeInspectable(f) {
			return nil
		}
		// Synchronization needs both branch tips; missing prerequisites are
		// represented by the local-metadata finding. Absent is explained by
		// the metadata-worktree-dirty reason.
		return nil
	case CondWorktreeHooksOff:
		if !worktreeInspectable(f) {
			return nil
		}
		if f.DocketWorktree.HooksOff == PresenceAbsent {
			return &Finding{
				Code:     "docket-worktree-hooks-enabled",
				Severity: SeverityError,
				Ref:      ".docket",
				Message:  "Git hooks are not disabled on the .docket metadata worktree (per-worktree core.hooksPath is not set to an existing directory).",
				Remedy:   "Point the .docket worktree's per-worktree core.hooksPath at an existing empty directory, as init leaves it, then re-run `docket repository check`.",
			}
		}
		return &Finding{
			Code:     "docket-worktree-hooks-unverified",
			Severity: SeverityWarning,
			Ref:      ".docket",
			Message:  "The .docket worktree's hooks configuration could not be resolved (unverified, not proven enabled).",
			Remedy:   "Re-run `docket repository check` once the .docket config read succeeds.",
		}
	case CondCommittedIgnoreValid:
		if !integrationResolved(f) {
			return nil // the unknown-authority diagnostic explains the skipped read
		}
		if f.CommittedIgnoreBlock == PresenceUnknown {
			return &Finding{
				Code:     "committed-ignore-unverified",
				Severity: SeverityWarning,
				Ref:      ".gitignore",
				Message:  "The committed .gitignore blob could not be read, so the managed ignore block cannot be verified (unverified, not proven absent).",
				Remedy:   "Restore readable committed evidence (fetch the integration objects), then re-run `docket repository check`.",
			}
		}
		return committedIgnoreFinding(f.CommittedIgnoreDetail)
	case CondLiveSurfaceAbsent:
		if f.LiveSurface == PresencePresent {
			return &Finding{
				Code:     "live-surface-present",
				Severity: SeverityError,
				Message:  "The integration tree still carries a live docket surface alongside the metadata branch.",
				Remedy:   "Run `docket repository migrate` to finish pruning the integration surface; it is idempotent.",
			}
		}
		return &Finding{
			Code:     "live-surface-unverified",
			Severity: SeverityWarning,
			Message:  "The integration tree's live-surface state could not be resolved (unverified).",
			Remedy:   "Ensure the remote is reachable, then re-run `docket repository check`.",
		}
	case CondLegacyConfigKeyAbsent:
		if !integrationResolved(f) {
			return nil
		}
		if f.LegacyConfigKey == PresencePresent {
			return &Finding{
				Code:     "legacy-config-key-present",
				Severity: SeverityError,
				Ref:      ".docket.yml",
				Message:  "The committed .docket.yml still declares the legacy top-level metadata_branch key.",
				Remedy:   "Remove the metadata_branch key from .docket.yml, commit, and push the integration branch, then re-run `docket repository check`.",
			}
		}
		return &Finding{
			Code:     "legacy-config-key-unverified",
			Severity: SeverityWarning,
			Ref:      ".docket.yml",
			Message:  "The committed .docket.yml could not be read, so the legacy metadata_branch key cannot be verified (unverified, not proven present).",
			Remedy:   "Restore readable committed evidence, then re-run `docket repository check`.",
		}
	case CondPrimaryClean:
		if f.PrimaryClean == PresenceAbsent {
			return &Finding{
				Code:     "primary-worktree-dirty",
				Severity: SeverityError,
				Message:  "The primary worktree has uncommitted changes.",
				Remedy:   "Review and commit (or deliberately restore) the changes yourself, preserving local work, then re-run `docket repository check`.",
			}
		}
		return &Finding{
			Code:     "primary-clean-unverified",
			Severity: SeverityWarning,
			Message:  "The primary worktree's cleanliness could not be resolved (unverified, not proven dirty).",
			Remedy:   "Re-run `docket repository check` once the primary status read succeeds.",
		}
	case CondPrimaryOnIntegration:
		if f.PrimaryOnIntegration == PresenceAbsent {
			return &Finding{
				Code:     "primary-not-on-integration",
				Severity: SeverityError,
				Message:  "The primary worktree is not checked out on the integration branch.",
				Remedy:   "Switch the primary worktree to the integration branch without discarding local work, then re-run `docket repository check`.",
			}
		}
		return &Finding{
			Code:     "primary-branch-unverified",
			Severity: SeverityWarning,
			Message:  "The primary worktree's branch could not be resolved (unverified).",
			Remedy:   "Re-run `docket repository check` once `git worktree list` succeeds.",
		}
	case CondPrimaryAtRemoteTip:
		if !integrationResolved(f) {
			return nil
		}
		if f.PrimaryAtRemoteTip == PresenceAbsent {
			return &Finding{
				Code:     "primary-behind-remote-tip",
				Severity: SeverityError,
				Message:  "The primary worktree's HEAD does not equal the pinned remote integration tip.",
				Remedy:   "Fetch and fast-forward the integration branch (e.g. `docket repository sync-integration`), preserving local work, then re-run `docket repository check`.",
			}
		}
		return &Finding{
			Code:     "primary-tip-unverified",
			Severity: SeverityWarning,
			Message:  "The primary worktree's position against the remote integration tip could not be resolved (unverified).",
			Remedy:   "Re-run `docket repository check` once the local HEAD read succeeds.",
		}
	case CondSurfacesAgree:
		// Only reached when SurfacesAuthorized (the conjunct is otherwise
		// satisfied); Absent is explained by the surfaces-drift reason.
		if f.SurfacesAgree != PresenceUnknown {
			return nil
		}
		return &Finding{
			Code:     "surfaces-unverified",
			Severity: SeverityWarning,
			Message:  "The authorized parent-facing surface agreement could not be resolved (unverified, not proven drifted).",
			Remedy:   "Re-run `docket repository check` once the surface probe succeeds.",
		}
	case CondNoPendingReviewPaths:
		// Applicable when pending paths exist but the classifier did not
		// select needs-review (e.g. an unverified root shape): reuse the
		// existing finding so the explanation cannot drift.
		fnd := findingFor("pending-review-paths", f)
		return &fnd
	}
	return nil
}

// committedIgnoreFinding renders the preserved ignore detail into the one
// committed-ignore defect finding. The defect is explicitly located in the
// committed integration tree: an uncommitted local fix does not establish
// the guarantee (learning gitignore-guarantee-must-be-committed).
func committedIgnoreFinding(d IgnoreDetail) *Finding {
	fnd := &Finding{
		Code:     "committed-ignore-invalid",
		Severity: SeverityError,
		Ref:      ".gitignore",
	}
	switch d.Defect {
	case IgnoreDefectFileAbsent:
		fnd.Message = "The committed integration tree has no .gitignore file, so the managed docket ignore block is absent."
		fnd.Remedy = "Restore the managed block (e.g. re-run `docket repository migrate`, or add it by hand from the canonical block), review, commit, and push the corrected .gitignore."
	case IgnoreDefectBlockAbsent:
		fnd.Message = "The committed .gitignore does not contain the managed docket ignore block."
		fnd.Remedy = "Restore the managed block, then review, commit, and push the corrected .gitignore."
	case IgnoreDefectLegacyOnly:
		fnd.Message = "The committed .gitignore carries only the legacy managed-block markers; the current-generation block is absent."
		fnd.Remedy = "Upgrade the managed block to the current markers, then review, commit, and push the corrected .gitignore."
	case IgnoreDefectMalformedMarkers:
		fnd.Message = "The committed .gitignore's managed-block markers are malformed (" + d.Generation + " generation): dangling, out-of-order, or nested start/end."
		fnd.Remedy = "Inspect and correct the reported marker structure by hand first, then review, commit, and push the corrected .gitignore."
	case IgnoreDefectMissingEntries:
		fnd.Message = "The committed .gitignore's managed docket block is missing required entries: " + strings.Join(d.MissingEntries, ", ") + "."
		fnd.Remedy = "Restore the missing entries (" + strings.Join(d.MissingEntries, ", ") + ") to the managed block, then review, commit, and push the corrected .gitignore."
	case IgnoreDefectNonCanonical:
		fnd.Message = "The committed .gitignore's managed docket block contains all required entries but differs from the canonical block representation (reordered, extra, or differently-terminated lines)."
		fnd.Remedy = "Rewrite the managed block to the canonical representation, then review, commit, and push the corrected .gitignore."
	default:
		// Absent presence with no preserved detail (an older caller):
		// still a concrete committed-block failure, without invented detail.
		fnd.Message = "The committed .gitignore's managed docket block failed validation in the committed integration tree."
		fnd.Remedy = "Restore the canonical managed block, then review, commit, and push the corrected .gitignore."
	}
	return fnd
}

// conditionCategory places a supplemental finding code into the existing
// deterministic output buckets (same order categoryOf fixes for reasons).
func conditionCategory(code string) int {
	switch code {
	case "metadata-ownership-unverified", "local-metadata-missing", "local-metadata-unverified":
		return catRemoteTopology
	case "committed-ignore-invalid", "committed-ignore-unverified",
		"legacy-config-key-present", "legacy-config-key-unverified",
		"live-surface-present", "live-surface-unverified", "pending-review-paths":
		return catIntegrationTree
	case "surfaces-unverified":
		return catSurface
	default: // every docket-worktree-* and primary-* code
		return catLocalWorktree
	}
}
```

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/reposetup/ -count=1`
Expected: PASS — including all pre-existing health tests (`TestHealthHealthyIsEmpty`, remedies, category order, exit matrix), which prove healthy output and existing state reports are unchanged.

- [ ] **Step 5: Mutation-test the population, not only the suppression**

Per spec Verification item 2 and learning `backstop-must-compute-not-reenumerate`, run these three mutations one at a time; each must redden the named test, then be restored from the editor buffer (never `git checkout --`) and re-run to green with `-count=1` (learning `cached-runner-serves-a-mutated-tree`):

1. In `supplementalConditionFindings`, delete the `if fnd := conditionFinding(...)` append — `TestHealthTerminalFallbackNamesEveryCause` and `TestHealthGenericFindingNeverAlone` must FAIL.
2. In `conditionFinding` case `CondWorktreeHooksOff`, delete the `PresenceAbsent` branch — `TestHealthPendingReviewPlusHooksOff` must FAIL.
3. In `committedIgnoreFinding` case `IgnoreDefectMissingEntries`, drop `strings.Join(d.MissingEntries, ...)` from the message — `TestHealthTerminalFallbackNamesEveryCause` must FAIL.

- [ ] **Step 6: Commit**

```bash
git add internal/reposetup/health.go internal/reposetup/health_test.go
git commit -m "feat(reposetup): supplement health reports with every applicable unmet condition (change 0418)"
```

---

### Task 4: Wire the preserved ignore detail through the check service

**Files:**
- Modify: `internal/app/repository_check.go`
- Modify: `internal/app/repository_check_test.go`

**Interfaces:**
- Consumes: `reposetup.CommittedIgnoreOutcome`, `reposetup.IgnoreDetail` (Task 2); existing `readCommitBlob`, `augmentCheckFacts`, `RunRepositoryCheck`.
- Produces: `committedIgnorePresence` now returns `(reposetup.Presence, reposetup.IgnoreDetail)`; `augmentCheckFacts` stores the detail in `f.CommittedIgnoreDetail`.

- [ ] **Step 1: Write the failing test**

Append to `internal/app/repository_check_test.go`:

```go
// TestRepositoryCheckSupplementalFindingsSerialize: supplemental condition
// findings ride the existing findings pipeline — same JSON serialization and
// the shared human renderer, no envelope change (change 0418).
func TestRepositoryCheckSupplementalFindingsSerialize(t *testing.T) {
	findings := []reposetup.Finding{
		{Code: "postconditions-unmet", Severity: reposetup.SeverityError,
			Message: "The metadata branch exists but not every health postcondition is satisfied.",
			Remedy:  "Resolve the reported issues, then re-run `docket repository check`."},
		{Code: "committed-ignore-invalid", Severity: reposetup.SeverityError, Ref: ".gitignore",
			Message: "The committed .gitignore's managed docket block is missing required entries: .opencode/agents/docket-*.md.",
			Remedy:  "Restore the missing entries (.opencode/agents/docket-*.md) to the managed block, then review, commit, and push the corrected .gitignore."},
	}
	res := newCheckResult(
		reposetup.Classification{State: reposetup.StateConflict, Reasons: []string{"postconditions-unmet"}},
		reposetup.Facts{}, findings)
	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{`"committed-ignore-invalid"`, `".gitignore"`, `.opencode/agents/docket-\*.md`} {
		if !strings.Contains(strings.ReplaceAll(string(raw), `\`, ``), strings.ReplaceAll(want, `\`, ``)) {
			t.Fatalf("JSON lacks %q: %s", want, raw)
		}
	}
	human := res.HumanText()
	if !strings.Contains(human, "committed-ignore-invalid") || !strings.Contains(human, ".opencode/agents/docket-*.md") {
		t.Fatalf("human text lacks the supplemental finding: %q", human)
	}
	if res.CheckExitCode() != 1 {
		t.Fatalf("exit = %d, want 1", res.CheckExitCode())
	}
}
```

Match the surrounding file's existing imports (`encoding/json`, `strings`, `reposetup` are already used there; add any missing).

- [ ] **Step 2: Run test to verify current state**

Run: `go test ./internal/app/ -run TestRepositoryCheckSupplementalFindingsSerialize -count=1`
Expected: PASS already (the pipeline is generic) — this test is the pinned regression net for the wiring change. If it fails, fix the test fixture, not the pipeline.

- [ ] **Step 3: Wire the detail through**

In `internal/app/repository_check.go`, replace `committedIgnorePresence` with:

```go
// committedIgnorePresence proves the managed .gitignore block from the
// integration COMMIT tree — never the working tree (learning
// gitignore-guarantee-must-be-committed) — and preserves the diagnostic
// detail the health report renders (change 0418). A read error is the safe
// Unknown, carried as an Unreadable detail, never a fabricated absence.
func committedIgnorePresence(ctx context.Context, git *gitcli.Client, repo gitcli.Repository, rev string) (reposetup.Presence, reposetup.IgnoreDetail) {
	blob, found, err := readCommitBlob(ctx, git, repo, rev, gitignoreRel)
	return reposetup.CommittedIgnoreOutcome(blob, found, err)
}
```

And in `augmentCheckFacts`, replace the single assignment line `f.CommittedIgnoreBlock = committedIgnorePresence(...)` with:

```go
		f.CommittedIgnoreBlock, f.CommittedIgnoreDetail = committedIgnorePresence(ctx, git, sc.repo, sc.sourceRevision)
```

- [ ] **Step 4: Run the app unit tests**

Run: `go test ./internal/app/ -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/repository_check.go internal/app/repository_check_test.go
git commit -m "feat(app): carry preserved committed-ignore detail through repository check (change 0418)"
```

---

### Task 5: Integration regression — the committed missing-entry incident, end to end

**Files:**
- Modify: `internal/app/repocheck_integration_test.go`

**Interfaces:**
- Consumes: `newHealthyRepo`, `(*initRepo).runCheck`, `(*initRepo).commitAndPushMain`, `runGit` (existing in that file); the finding codes from Task 3.

- [ ] **Step 1: Write the failing integration tests**

Append to `internal/app/repocheck_integration_test.go` (inside the existing `//go:build integration` file):

```go
// TestIntegrationRepoCheckMissingIgnoreEntryNamed is the committed-file
// regression for the reported real-world incident (change 0418): delete ONLY
// the .opencode entry from the canonical committed block, commit and push it,
// leave the working tree matching, and assert human and JSON output name
// .gitignore, the exact missing entry, and its remedy — with state and exit
// unchanged from what a broken postcondition already produced.
func TestIntegrationRepoCheckMissingIgnoreEntryNamed(t *testing.T) {
	r := newHealthyRepo(t)

	gi := filepath.Join(r.invocation, ".gitignore")
	raw, err := os.ReadFile(gi)
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	const entry = ".opencode/agents/docket-*.md"
	mutated := strings.Replace(string(raw), entry+"\n", "", 1)
	if mutated == string(raw) {
		t.Fatalf("fixture drifted: %s not in managed block", entry)
	}
	if err := os.WriteFile(gi, []byte(mutated), 0o644); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}
	r.commitAndPushMain(t, "drop opencode ignore entry", ".gitignore")

	res := r.runCheck(t)
	if res.RepositoryState != string(reposetup.StateConflict) {
		t.Fatalf("state = %q, want conflict", res.RepositoryState)
	}
	if res.CheckExitCode() != 1 {
		t.Fatalf("exit = %d, want 1", res.CheckExitCode())
	}
	var ignore *reposetup.Finding
	for i := range res.Findings {
		if res.Findings[i].Code == "committed-ignore-invalid" {
			ignore = &res.Findings[i]
		}
	}
	if ignore == nil {
		t.Fatalf("no committed-ignore-invalid finding; codes: %+v", res.Findings)
	}
	if ignore.Ref != ".gitignore" ||
		!strings.Contains(ignore.Message, entry) ||
		!strings.Contains(ignore.Remedy, entry) {
		t.Fatalf("finding does not name path+entry+remedy: %+v", ignore)
	}
	human := res.HumanText()
	if !strings.Contains(human, ".gitignore") || !strings.Contains(human, entry) {
		t.Fatalf("human output does not name the defect: %q", human)
	}
}

// TestIntegrationRepoCheckHealthyBaselineHasNoSupplementalFindings: the
// previously healthy baseline stays byte-for-byte clean — no supplemental
// finding leaks into a healthy report.
func TestIntegrationRepoCheckHealthyBaselineHasNoSupplementalFindings(t *testing.T) {
	r := newHealthyRepo(t)
	res := r.runCheck(t)
	if res.RepositoryState != string(reposetup.StateHealthy) || len(res.Findings) != 0 || res.CheckExitCode() != 0 {
		t.Fatalf("healthy baseline changed: state=%q findings=%+v exit=%d",
			res.RepositoryState, res.Findings, res.CheckExitCode())
	}
}
```

(`newHealthyRepo` already asserts the healthy baseline before returning, so the mutation is measured against proven health.)

- [ ] **Step 2: Run to verify the regression test exercises the new path**

Run: `go test -tags integration ./internal/app/ -run 'TestIntegrationRepoCheckMissingIgnoreEntryNamed|TestIntegrationRepoCheckHealthyBaselineHasNoSupplementalFindings' -count=1 -v`
Expected: PASS (Tasks 1–4 are in place). Then prove the test is not vacuous: temporarily revert the `augmentCheckFacts` wiring line to `f.CommittedIgnoreBlock = ...` discarding the detail (compile will require restoring the old single-value helper form or assigning the detail to `_`), re-run, and confirm `TestIntegrationRepoCheckMissingIgnoreEntryNamed` FAILS on the missing entry name. Restore the wiring and re-run to green.

- [ ] **Step 3: Confirm the existing integration shard still passes**

Run: `go test -tags integration ./internal/app/ -run TestIntegrationRepoCheck -count=1`
Expected: PASS — `TestIntegrationRepoCheckFresh`, `...NeedsReviewAfterInit`, `...HealthyFullPostcondition`, `...IsReadOnly`, `...UnknownAuthority`, and `...DerivedViewDrift` all preserved. `...HealthyFullPostcondition`'s sub-cases now see supplemental findings beside the generic one; if any sub-case asserts an exact findings count, extend its expectation to include the new named finding while keeping the state and exit asserts identical (the state/exit contract is the invariant; the findings list deliberately grew — the point of the change). Do not weaken any state, exit, or read-only assert.

- [ ] **Step 4: Commit**

```bash
git add internal/app/repocheck_integration_test.go
git commit -m "test(app): committed missing-ignore-entry regression names path, entry, and remedy (change 0418)"
```

---

### Task 6: Full-suite gate

**Files:** none (verification only).

- [ ] **Step 1: Run the whole suite from the feature worktree**

Run: `go run ./cmd/docket development test`
Expected: SUITE PASS. This is the resolved `build.test_command` — never a hand-picked subset (AGENTS.md "Guards and tests").

- [ ] **Step 2: Read the budget report**

Even on green, check for `BUDGET WATCH:`, `PARALLEL-SENSITIVE:`, and `SERIAL CONFIRMED OVER BUDGET:` lines. A serial-confirmed breach must be acted on per `tests/README.md` before the task completes; a watch line is recorded as a screening finding in the task report.

- [ ] **Step 3: Commit any straggler fixes**

If the suite surfaced repairs (e.g. a repo-wide guard counting the new codes), fix and commit them with message `fix: <what> (change 0418)`. No commit if the tree is already clean.

---

## Self-Review (performed while writing this plan)

- **Spec coverage:** independent evaluation helper → Task 1; ignore-detail preservation at the read boundary → Tasks 2 and 4; supplemental findings, applicability, dedup, remedies, enriched dirty message → Task 3; both approved mixed-failure examples → Task 3 (`TestHealthDirtyWorktreePlusIgnoreDefect`, `TestHealthPendingReviewPlusHooksOff`); partial+ignore, unknown-ownership+known-defect, dedup, no-cascade, unreadable-blob, unknown-authority-with-defects, fresh/legacy unchanged, unauthorized surfaces → Task 3; committed-file authority regression + healthy baseline + read-only preservation → Task 5; mutation of population steps → Tasks 1/3/5; full gate → Task 6.
- **Exclusions honored:** no new commands/config/ADRs/probe states/registries; `ValidGitignoreBlock` untouched; classifier ladder and reason tokens untouched; `EvaluateHealth` signature unchanged.
- **Type consistency:** `HealthCondition`/`Cond*` (Task 1) consumed by Tasks 3; `IgnoreDetail`/`IgnoreDefect*`/`CommittedIgnoreOutcome`/`GitignoreEntries` (Task 2) consumed by Tasks 3–5; `committedIgnorePresence` two-value signature (Task 4) matches `CommittedIgnoreOutcome`'s return.
