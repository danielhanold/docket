<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0327 — Stacked-merged close-out can stamp `done` after a stale-worktree rebase clobbers the child — prove reachability in git, not metadata](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-08-0327-stack-closeout-must-prove-integration-reachability.md)**
<!-- docket:backlink:end -->
# Stack Closeout Git Preservation Proof — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Under docket, `docket-build` executes this plan task-by-task.

**Goal:** Prove in Git — never from metadata, PR destinations, or a green suite — that every stacked descendant's merged work is still present before a carrying branch is rewritten, published, or merged, and before a stack root (plus descendants) is archived `done`.

**Architecture:** One narrow typed Git adapter operation (`gitcli.ProvePreserved`) answers "is `source`'s work preserved in `target`?" with a closed proven/unproven/error vocabulary (ancestry arm, then an exact changed-entry comparison against the single merge base). One pure-domain derivation (`domain.DeriveCarriedSet`) selects the descendants a live carrying branch promises to carry. One shared app helper (`proveCarriedOnHead`) joins the two with authoritative GitHub facts and structured findings. Enforcement is added at four boundaries: `FinalizeRebase` (pre-rewrite fresh path + the `composeLocalGate` chokepoint that all post-rewrite paths funnel through), `FinalizePublish` (before the remote rewrite), `FinalizeMerge` (before the external merge), and `FinalizeCloseout` (stacked marking and root-carry archive).

**Tech Stack:** Go; git plumbing via `internal/gitcli` (`merge-base --all`, `diff-tree -r -z --no-renames --full-index`, `ls-tree` via the existing `TreeEntryIDs`); fake GitHub seams from the existing `internal/app` integration fixtures; suite runner `go run ./cmd/docket development test`.

**Spec:** `docs/superpowers/specs/2026-09-07-stack-closeout-must-prove-integration-reachability-design.md` (on the `docket` metadata branch; local synchronized copy at `.docket/docs/superpowers/specs/…` in the primary checkout). The change file is `docs/changes/active/0327-stack-closeout-must-prove-integration-reachability.md` on the metadata branch.

## Global Constraints

- Preserve the existing rebase → merge-commit → squash merge-method preference; no merge-method configuration changes, no fallback mutation after a denied merge.
- `internal/domain` stays free of I/O; the adapter owns Git mechanics; the app owns lifecycle decisions.
- The comparison reads Git objects only: no merge drivers, filters, patch-id normalization, filename exclusions, index/worktree/branch mutation. Rename detection disabled (`--no-renames`); NUL-safe structured records; full object IDs (`--full-index`).
- Missing objects, shallow/incomplete history, unsupported plumbing, and uncertain external observations are observation **errors**, never proof and never a clean "unproven". Fetching a required exact object is permitted only through the typed adapter against the already-established remote.
- No external push, merge, terminal metadata write, or cleanup follows an unproven gate. A post-rewrite failure retains the owned rebase attempt and original refs for the existing abort/repair flow — never resets or discards the user's resolution.
- Proofs are scoped to immutable target commit IDs — no caching by branch name or lifecycle status; a proof is invalid for any changed target ID.
- `finalize.gate: off` keeps its meaning (no rebase, no local suite); it cannot disable the proofs at publication, merge, or closeout.
- Keep every closed result vocabulary closed: new reason tokens are added as named constants; obey `prohibition-needs-a-return-value` — every new refusal names its Result/Disposition/Reason mapping.
- Cross-references in maintained source anchor on symbol names or verbatim-quoted clauses, never line numbers.
- All mutation probes and manual re-verification defeat Go's test cache with `-count=1` (learning `cached-runner-serves-a-mutated-tree`).
- Every new suite file needs a `# docket-suite: go` declaration in its first 10 lines and a row in `tests/runtime-budgets.tsv`; new Go integration tests must be routed by a shard prefix registered with `tests/test_go_integration_contract.sh`'s completeness contract. Do not raise an existing budget to accommodate new tests.
- Final gate: run the whole suite via the current resolved `build.test_command` (today `go run ./cmd/docket development test`), never only the tests this plan enumerates.

**Learnings folded in (read them if a task's step cites them):** `moving-base` (re-verify the snapshot the spec measured against — origin/main was `0d1a7e1b…` — and reconcile any drift at build time, superseding merged edits over yours at conflict), `groomed-root-cause-is-a-hypothesis` (the stale-worktree route is already closed in Go; the gap being fixed is the missing Git proof — do not rebuild the shipped head/lease guards), `verify-the-claim` (this plan's code sketches are unverified until run; prove each assert CAN fail), `assert-pins-outcome-not-mechanism` (pin the reason token, the specific missing effect — receipt absent, zero push calls, ref tip unchanged — never a bare non-success), `green-suite-untested-branch` (the fabricated `bbbb…` fixture is the proof that a green suite hid this; every new fixture must carry a real discriminating input — a real object that exists but is *dropped*, distinct from an object that is *missing*).

---

### Task 1: gitcli plumbing — merge bases, shallow probe, commit resolution

**Files:**
- Create: `internal/gitcli/preservecommit.go`
- Test: `internal/gitcli/preservecommit_integration_test.go`
- Create: `tests/test_go_integration_gitcli_preserve.sh`
- Modify: `tests/runtime-budgets.tsv` (add one row)
- Read first: `internal/gitcli/history.go` (the `HasSharedAncestry` / `TreeEntryIDs` idioms), `internal/gitcli/push.go` (`IsAncestor`), `internal/gitcli/exec.go` (`c.run`), `internal/gitcli/failure.go`, `tests/test_go_integration_gitcli_history.sh` + `tests/lib/go-integration-shard.sh` + `tests/test_go_integration_contract.sh` (shard registration mechanics).

**Interfaces:**
- Produces (used by Tasks 2–3): unexported helpers on `*Client` —
  `func (c *Client) ensurePreservationInputs(ctx context.Context, repo Repository, remote RemoteName, source, target ObjectID) *Failure` (validates both IDs, refuses shallow history, resolves both as commit objects, fetching a missing exact object once from `remote`);
  `func (c *Client) mergeBasesAll(ctx context.Context, repo Repository, a, b ObjectID) ([]ObjectID, *Failure)` (`git merge-base --all a b`; exit 1 with empty output = no base = empty slice, nil failure; any other nonzero = failure).
- Test-shard registration: new shard `tests/test_go_integration_gitcli_preserve.sh` with `SHARD_PKG="./internal/gitcli"`, `SHARD_PREFIX="TestIntegrationPreserveCommit"`, mirroring `tests/test_go_integration_gitcli_history.sh` byte-for-byte in structure. All integration tests in Tasks 1–3 are named `TestIntegrationPreserveCommit*` so the one shard routes them. Note `internal/gitcli/preserve_integration_test.go` already exists and is UNRELATED (checkout-preservation proof matrix); do not touch it, and confirm its tests' prefix does not collide with `TestIntegrationPreserveCommit` before choosing the prefix — if it does, use `TestIntegrationPreservationProof` consistently everywhere this plan says `TestIntegrationPreserveCommit`.

- [ ] **Step 1: Register the shard.** Copy `tests/test_go_integration_gitcli_history.sh` to `tests/test_go_integration_gitcli_preserve.sh`; edit the header comment, set `SHARD_PREFIX="TestIntegrationPreserveCommit"`. Add to `tests/runtime-budgets.tsv` (tab-separated, keep file ordering conventions): `tests/test_go_integration_gitcli_preserve.sh	15	parallel`. Read `tests/test_go_integration_contract.sh` and satisfy whatever registration it enforces for a new shard (it is the completeness contract; run it: `bash tests/test_go_integration_contract.sh` — it must pass once at least one `TestIntegrationPreserveCommit*` test exists; if it reds before any test exists, do this step together with Step 2).

- [ ] **Step 2: Write the failing tests** in `internal/gitcli/preservecommit_integration_test.go` (`//go:build integration`, package `gitcli`). Use the repo-fixture helpers the sibling `history_integration_test.go` uses (read it first; reuse its setup helpers rather than inventing new ones). Cover:

```go
func TestIntegrationPreserveCommitMergeBasesAll(t *testing.T) {
	// fixture: linear a->b: bases(a,b) == [a]
	// fixture: criss-cross (two branches each merging the other once) -> len(bases) == 2
	// fixture: two orphan roots (independent initial commits) -> len(bases) == 0, nil error
}
func TestIntegrationPreserveCommitEnsureInputs(t *testing.T) {
	// invalid id (short hex) -> *Failure KindInvalidRequest
	// missing object, file:// remote without it -> *Failure (fetch attempted, still absent)
	// missing object PRESENT on the remote -> fetch succeeds, nil failure
	// blob id (hash a blob, pass it) -> *Failure (not a commit)
	// shallow clone (git clone --depth 1) -> *Failure naming shallow history
}
```

Write each sub-case as `t.Run` blocks with real git fixtures built by shelling through the existing test helpers (`git commit-tree`/porcelain in a temp repo, as `history_integration_test.go` does). Assert failure kinds via `gitcli.AsFailure`.

- [ ] **Step 3: Run to verify failure.** `go test -tags=integration ./internal/gitcli -run '^TestIntegrationPreserveCommit' -count=1 -v` — expected: compile error (helpers undefined).

- [ ] **Step 4: Implement** `internal/gitcli/preservecommit.go`:

```go
package gitcli

// Operation names follow the file-local convention (see history.go).
const (
	preserveOp      Operation = "preserve-proof"
	mergeBasesOp    Operation = "merge-bases"
)

// ensurePreservationInputs validates both IDs, refuses shallow history, and
// resolves both as commit objects — fetching a missing exact object at most
// once from the established remote. Inability to obtain an object, a non-commit
// object, or a shallow boundary is an observation failure, never an answer.
func (c *Client) ensurePreservationInputs(ctx context.Context, repo Repository, remote RemoteName, source, target ObjectID) *Failure {
	for _, id := range []ObjectID{source, target} {
		if err := validateObjectID(id); err != nil {
			return newFailure(preserveOp, KindInvalidRequest, "invalid commit id", err)
		}
	}
	res, f := c.run(ctx, runRequest{op: preserveOp, dir: repo.PrimaryWorktree,
		args: []string{"rev-parse", "--is-shallow-repository"}})
	if f != nil { return f }
	if strings.TrimSpace(string(res.stdout)) != "false" {
		return newFailure(preserveOp, KindInvalidRepository,
			"shallow history cannot answer a preservation query", nil)
	}
	for _, id := range []ObjectID{source, target} {
		if f := c.resolveCommitFetching(ctx, repo, remote, id); f != nil { return f }
	}
	return nil
}

// resolveCommitFetching: `cat-file -e <id>^{commit}`; on failure fetch the
// exact id from remote once (ensureRemoteConfigured first) and recheck.
// (implement with two c.run calls; classify the fetch through
// classifyFetchFailure-style handling or a plain KindUnexpectedObject on
// post-fetch absence, message naming the id)

// mergeBasesAll returns ALL best common ancestors of a and b.
// exit 0: one id per line; exit 1 + empty stdout: no common base (empty slice,
// nil failure); anything else: *Failure.
func (c *Client) mergeBasesAll(ctx context.Context, repo Repository, a, b ObjectID) ([]ObjectID, *Failure) {
	res, f := c.run(ctx, runRequest{op: mergeBasesOp, dir: repo.PrimaryWorktree,
		args: []string{"merge-base", "--all", string(a), string(b)}})
	if f != nil { return nil, f }
	if res.exitCode == 1 && len(bytes.TrimSpace(res.stdout)) == 0 { return nil, nil }
	if res.exitCode != 0 {
		return nil, newFailure(mergeBasesOp, KindCommandFailed,
			"merge-base --all failed: "+stderrExcerpt(res.stderr), nil).withExitCode(res.exitCode)
	}
	var out []ObjectID
	for _, line := range stdoutLines(res.stdout) {
		id := ObjectID(line)
		if err := validateObjectID(id); err != nil {
			return nil, newFailure(mergeBasesOp, KindInvalidOutput, "malformed merge-base output", err)
		}
		out = append(out, id)
	}
	return out, nil
}
```

Match the exact `newFailure`/`stderrExcerpt`/`stdoutLines` idioms already in `history.go`/`refs.go` — read them; do not invent parallel helpers.

- [ ] **Step 5: Run to verify pass.** Same command as Step 3, expected PASS. Also run `bash tests/test_go_integration_gitcli_preserve.sh` and `bash tests/test_go_integration_contract.sh` — both green.

- [ ] **Step 6: Commit.** `git add internal/gitcli/preservecommit.go internal/gitcli/preservecommit_integration_test.go tests/test_go_integration_gitcli_preserve.sh tests/runtime-budgets.tsv && git commit -m "feat(0327): gitcli preservation plumbing — merge bases, shallow refusal, exact-object resolution"`

---

### Task 2: gitcli plumbing — the source delta (diff-tree raw records)

**Files:**
- Modify: `internal/gitcli/preservecommit.go`
- Test: `internal/gitcli/preservecommit_integration_test.go`, plus a pure-parse unit test in `internal/gitcli/preservecommit_test.go` (no build tag)

**Interfaces:**
- Produces (used by Task 3):

```go
// deltaEntry is one changed tracked entry from base to source, rename
// detection disabled: a rename is a delete plus an add.
type deltaEntry struct {
	Path    RepoPath
	Status  byte     // 'A','M','D','T' (diff-tree raw status, first byte)
	NewMode FileMode // final mode at source ("000000" when Status=='D')
	NewOID  ObjectID // final oid at source (all-zero when Status=='D')
}
func (c *Client) commitDelta(ctx context.Context, repo Repository, base, source ObjectID) ([]deltaEntry, *Failure)
func parseDiffTreeRawZ(out []byte) ([]deltaEntry, error)
```

- [ ] **Step 1: Write the failing parse unit test** (`preservecommit_test.go`): feed literal `diff-tree -r -z --no-renames --full-index` byte streams — record shape `:<oldmode> <newmode> <oldoid> <newoid> <status>\x00<path>\x00` — covering: add, modify, delete, mode-only change (`T`/`M` with same oid different mode), a path containing a newline and a non-UTF8 byte (NUL-delimited must handle both), and malformed records (missing NUL, short header, bad oid) each returning an error with no entries. Also an empty stream → `nil, nil`.

- [ ] **Step 2: Run to verify fail.** `go test ./internal/gitcli -run 'ParseDiffTreeRawZ' -count=1` — FAIL (undefined).

- [ ] **Step 3: Implement** `commitDelta` (`args: []string{"diff-tree", "-r", "-z", "--no-renames", "--full-index", string(base), string(source)}` through `c.run`, then `parseDiffTreeRawZ`) and the parser. Parser discipline mirrors `parseLsTreeEntriesZ` (read it): every record NUL-terminated, header splits into exactly 5 space-separated fields after the leading `:`, both oids validate via `validateObjectID` (an all-zero oid is legal for D's new side — permit exactly the all-zero spelling), status is one nonempty field whose first byte ∈ `ACDMRTUXB` but with `--no-renames` require it ∈ `ADMT` and treat anything else as `KindInvalidOutput`. Any violation → error, zero entries.

- [ ] **Step 4: Add one integration case** to `TestIntegrationPreserveCommitDelta` in the integration file: real repo, base→source adding a file, modifying one, deleting one, chmod +x one, and adding a symlink; assert the exact five entries (path, status, mode, oid — compute expected oids with `git rev-parse <source>:<path>`). Run: `go test -tags=integration ./internal/gitcli -run '^TestIntegrationPreserveCommitDelta$' -count=1 -v` → PASS; unit test → PASS.

- [ ] **Step 5: Commit.** `git add -u internal/gitcli && git add internal/gitcli/preservecommit_test.go && git commit -m "feat(0327): gitcli commit delta — NUL-safe no-rename raw records base->source"`

---

### Task 3: gitcli `ProvePreserved` — the preservation primitive

**Files:**
- Modify: `internal/gitcli/preservecommit.go`
- Test: `internal/gitcli/preservecommit_integration_test.go`

**Interfaces:**
- Produces (consumed by Tasks 5–10):

```go
type PreservationOutcome string
const (
	PreservationProven   PreservationOutcome = "proven"
	PreservationUnproven PreservationOutcome = "unproven"
)
type PreservationKind string
const (
	PreservationByAncestry PreservationKind = "ancestry"
	PreservationByContent  PreservationKind = "exact-content"
)
// Closed unproven-detail tokens (stable machine strings).
const (
	PreserveNoCommonBase  = "no-common-base"
	PreserveMultipleBases = "multiple-bases"
	PreserveEmptyDelta    = "empty-delta"
	PreserveEntryDiffers  = "entry-differs"
)
type PreservationCheck struct {
	Outcome PreservationOutcome
	Kind    PreservationKind // set only when proven
	Detail  string           // closed token when unproven; "" when proven
	Paths   []RepoPath       // bounded diagnostic (<= 8 differing paths)
}
// ProvePreserved reports whether source's work (source = a child's authoritative
// merge-result commit) is preserved in target. Ancestry proves; otherwise the
// full base->source tracked-entry delta must match target exactly (oid+mode; a
// source deletion requires absence). Uncertainty is unproven; inability to
// observe is an error — never a verdict.
func (c *Client) ProvePreserved(ctx context.Context, repo Repository, remote RemoteName, source, target ObjectID) (PreservationCheck, error)
```

- [ ] **Step 1: Write the failing integration matrix** — table-driven inside `TestIntegrationPreserveCommitProve`, each row a `t.Run` building a real repo (helper: init repo, commit files via porcelain, capture OIDs). Required rows (spec Tests §1, §8–10):
  1. `ancestral`: source is ancestor of target → `proven/ancestry`.
  2. `rebase-rewrite`: child adds `catalog.yaml` on a branch; rebase the branch onto an advanced main (different commit IDs, same blobs); target = rebased tip → `proven/exact-content`.
  3. `squash-rewrite`: squash child's two commits into one on target's line → `proven/exact-content`.
  4. `unrelated-destination-advance`: like 2, plus target has extra commits touching OTHER files after the rewrite → still `proven/exact-content` (extra target changes outside the delta are allowed).
  5. `content-dropped`: target line omits `catalog.yaml` entirely → `unproven/entry-differs`, `Paths` contains `catalog.yaml`. Assert the source object EXISTS in the repo first (`git cat-file -e`), pinning drop vs missing-object (spec Tests §2).
  6. `partial-loss`: two changed files, one preserved one dropped → `unproven/entry-differs`.
  7. `overlapping-edit`: target carries `catalog.yaml` with DIFFERENT bytes (a later edit) → `unproven/entry-differs` (spec §10: ambiguity refuses, matched on oid, not filename presence).
  8. `deletion-preserved` / `deletion-lost`: source deletes a file; target absent → proven; target still has it → unproven.
  9. `rename-as-delete-add`: source renames a→b; target preserving both sides → proven; target keeping old name only → unproven.
  10. `mode-change`: chmod +x preserved vs lost (compare exact mode).
  11. `symlink`, `binary-blob`, `gitlink` (use `git update-index --add --cacheinfo 160000,<sha>,sub` to fabricate a gitlink), `file-to-directory transition`, `unusual path` (space + UTF-8) — one preserved-proven and one lost-unproven variant each; table rows may share fixture builders.
  12. `no-common-base`: orphan target root → `unproven/no-common-base`.
  13. `multiple-bases`: criss-cross ancestry → `unproven/multiple-bases` (never pick an arbitrary base).
  14. `empty-delta-non-ancestral`: base == source (source IS the sole merge base but not ancestor of target? impossible — instead: source's delta from base is empty because source is the base's equal tree via an empty commit) → `unproven/empty-delta`.
  15. `missing-source-object`: 40-hex id absent everywhere → error (from Task 1's resolution), not an outcome.
  16. `merge-driver-cannot-help`: configure a custom merge driver in the repo's `.gitattributes`/config that would "resolve" anything, re-run row 7 → still `unproven` (the comparison never invokes merge machinery; this pins spec §9's "custom merge driver cannot manufacture proof").

Assert on `Outcome`, `Kind`, `Detail`, and `Paths` membership — never just "not proven" (`assert-pins-outcome-not-mechanism`).

- [ ] **Step 2: Run to verify fail.** `go test -tags=integration ./internal/gitcli -run '^TestIntegrationPreserveCommitProve' -count=1` — FAIL (undefined `ProvePreserved`).

- [ ] **Step 3: Implement** `ProvePreserved`:

```go
func (c *Client) ProvePreserved(ctx context.Context, repo Repository, remote RemoteName, source, target ObjectID) (PreservationCheck, error) {
	if f := c.ensurePreservationInputs(ctx, repo, remote, source, target); f != nil {
		return PreservationCheck{}, f
	}
	ok, err := c.IsAncestor(ctx, repo, source, target)
	if err != nil { return PreservationCheck{}, err }
	if ok {
		return PreservationCheck{Outcome: PreservationProven, Kind: PreservationByAncestry}, nil
	}
	bases, f := c.mergeBasesAll(ctx, repo, source, target)
	if f != nil { return PreservationCheck{}, f }
	if len(bases) == 0 {
		return PreservationCheck{Outcome: PreservationUnproven, Detail: PreserveNoCommonBase}, nil
	}
	if len(bases) > 1 {
		return PreservationCheck{Outcome: PreservationUnproven, Detail: PreserveMultipleBases}, nil
	}
	delta, f := c.commitDelta(ctx, repo, bases[0], source)
	if f != nil { return PreservationCheck{}, f }
	if len(delta) == 0 {
		// Conservative: never manufacture proof from an empty population.
		return PreservationCheck{Outcome: PreservationUnproven, Detail: PreserveEmptyDelta}, nil
	}
	paths := make([]RepoPath, 0, len(delta))
	for _, d := range delta { paths = append(paths, d.Path) }
	entries, err := c.TreeEntryIDs(ctx, repo, target, paths)
	if err != nil { return PreservationCheck{}, err }
	at := make(map[RepoPath]TreeEntry, len(entries))
	for _, e := range entries { at[e.Path] = e }
	var differing []RepoPath
	for _, d := range delta {
		e, present := at[d.Path]
		match := false
		if d.Status == 'D' {
			match = !present // a source deletion requires absence in target
		} else {
			match = present && e.OID == d.NewOID && e.Mode == d.NewMode
		}
		if !match && len(differing) < 8 { differing = append(differing, d.Path) }
		if !match && len(differing) >= 8 { break }
	}
	if len(differing) > 0 {
		return PreservationCheck{Outcome: PreservationUnproven, Detail: PreserveEntryDiffers, Paths: differing}, nil
	}
	return PreservationCheck{Outcome: PreservationProven, Kind: PreservationByContent}, nil
}
```

Adjust field names (`TreeEntry`'s actual OID/Mode field spellings — read `internal/gitcli/types.go` and `parseLsTreeEntriesZ`) to the real types; the `break` must not stop before recording that a mismatch exists (restructure: track `mismatched bool` separately if capping paths at 8). Note `TreeEntryIDs` is non-recursive per explicit path: verify by reading it that an exact full path argument yields the leaf entry (its doc comment says an absent path yields no entry); if a directory-valued path (file→dir transition) yields a `tree` entry, that entry's mode `040000` never equals a blob's `NewMode`, so the comparison stays conservative — add a fixture row proving it.

- [ ] **Step 4: Run to verify pass.** Step 2's command → PASS. Then the two hostile probes: temporarily delete the `len(bases) > 1` branch → rows 13 reddens; restore. Temporarily flip `match = !present` to `true` on `'D'` → deletion-lost row reddens; restore. (`guards-are-code`; use a scratch copy of the edit, restore via your uncommitted-edit backup, not `git checkout` — learning `mutation-restore-needs-a-backup-copy`.)

- [ ] **Step 5: Commit.** `git add -u internal/gitcli && git commit -m "feat(0327): gitcli ProvePreserved — ancestry or exact changed-entry preservation proof"`

---

### Task 4: domain `DeriveCarriedSet` — the pre-merge carried population

**Files:**
- Modify: `internal/domain/stackcloseout.go`
- Test: `internal/domain/stackcloseout_test.go` (extend; read its existing fixture idioms first)

**Interfaces:**
- Consumes: existing `Snapshot`, `StackChildren`, `PRFacts`, `CarriedDescendant`, the `carry*` refusal tokens, `prStateMerged`.
- Produces (used by Task 5):

```go
// DeriveCarriedSet selects, for a LIVE carrying change (a branch about to be
// rewritten, published, or merged), every descendant whose stacked-merged code
// that branch currently PROMISES to carry: children connected by an unbroken
// chain of stacked-merged changes whose verified PR destinations match their
// recorded parent branches. It stops descending into an open intermediate
// child — grandchildren merged into a still-open child are not yet promised by
// this branch. A purported carried link (a stacked-merged child) with unknown
// or mismatched facts is surfaced with a refusal token, never omitted.
// A merged PR destination proves a RELATIONSHIP only; Git preservation of the
// content is the caller's separate obligation.
func DeriveCarriedSet(s Snapshot, parent ChangeID, facts map[ChangeID]PRFacts) ([]CarriedDescendant, *PolicyFailure)
```

- [ ] **Step 1: Write the failing unit tests** (pure domain, table fixtures like the existing `DeriveRootCloseoutSet` tests):
  - transitive chain parent←A(stacked-merged, merged→parent.branch)←B(stacked-merged, merged→A.branch): both returned, both `Proof: ""` (spec Tests §6).
  - open intermediate: parent←A(in-progress)←B(stacked-merged into A): result EMPTY — B is not promised by parent yet (spec §6).
  - stacked-merged child with no facts entry → `Proof: "pr-unknown"`; with facts whose `BaseRef` ≠ parent's recorded branch → `"destination-mismatch"`; parent branch field absent → `"destination-mismatch"` (never silently omitted).
  - a broken link is NOT descended: parent←A(stacked-merged, pr-unknown)←B(stacked-merged into A): A surfaced with token; B absent (the whole set already blocks; assert A's token specifically).
  - cycle in the child graph → the revisited node reports `"cycle"`, derivation terminates.
  - absent/ambiguous parent id → `*PolicyFailure` with `FailInvalidInput` (mirror `DeriveRootCloseoutSet`'s root checks).
  - killed or proposed child: contributes nothing and is not descended.

- [ ] **Step 2: Run to verify fail.** `go test ./internal/domain -run DeriveCarriedSet -count=1` — FAIL.

- [ ] **Step 3: Implement** in `stackcloseout.go`:

```go
func DeriveCarriedSet(s Snapshot, parent ChangeID, facts map[ChangeID]PRFacts) ([]CarriedDescendant, *PolicyFailure) {
	pc, out := s.Change(parent)
	switch out {
	case LookupAbsent:
		return nil, &PolicyFailure{Kind: FailInvalidInput, Change: parent, Reason: "parent-not-found"}
	case LookupAmbiguous:
		return nil, &PolicyFailure{Kind: FailInvalidInput, Change: parent, Reason: "parent-ambiguous"}
	}
	set := []CarriedDescendant{}
	visited := map[ChangeID]bool{parent: true}
	var walk func(p Change)
	walk = func(p Change) {
		for _, cid := range StackChildren(s, p.ID()) {
			if visited[cid] {
				set = append(set, CarriedDescendant{ID: cid, Proof: carryCycle})
				continue
			}
			visited[cid] = true
			c, out := s.Change(cid)
			if out != LookupFound {
				set = append(set, CarriedDescendant{ID: cid, Proof: carryChainBroken})
				continue
			}
			if c.Status() != StatusStackedMerged {
				continue // an open/terminal child claims no carry; do not descend
			}
			token := ""
			f, ok := facts[cid]
			branch := p.Branch()
			switch {
			case !ok || f.State != prStateMerged:
				token = carryPRUnknown
			case branch.State != FieldPresent || branch.Value == "" || f.BaseRef != branch.Value:
				token = carryDestinationMismatch
			}
			set = append(set, CarriedDescendant{ID: cid, Proof: token})
			if token == "" {
				walk(c)
			}
		}
	}
	walk(pc)
	return set, nil
}
```

Reuse `RootCloseoutProven` unchanged for the all-proven check. While in the file, do NOT yet reword `DeriveRootCloseoutSet`/`proveCarry` comments — Task 11 owns the comment truth-up so it lands with the code that makes it true.

- [ ] **Step 4: Run to verify pass**, then mutation-probe the population (`backstop-must-compute-not-reenumerate`): temporarily change `walk(c)` to a no-recursion body → the transitive test reddens naming B missing; restore. `go test ./internal/domain -run DeriveCarriedSet -count=1` green.

- [ ] **Step 5: Commit.** `git add -u internal/domain && git commit -m "feat(0327): domain DeriveCarriedSet — pre-merge carried population with surfaced refusals"`

---

### Task 5: app shared proof helper `proveCarriedOnHead`

**Files:**
- Create: `internal/app/finalize_preservation.go`
- Test: `internal/app/finalize_preservation_integration_test.go` (`//go:build integration`; name tests with an existing app shard's prefix — read the `SHARD_PREFIX` of `tests/test_go_integration_app_rebase.sh` and siblings, and pick the shard whose package run already compiles this package; if prefixes are operation-scoped (e.g. `^TestIntegrationFinalizeRebase`), name these `TestIntegrationFinalizeRebaseCarryHelper*` under that shard — confirm against `tests/test_go_integration_contract.sh`).
- Read first: `internal/app/finalize_closeout.go` (`probeDescendantFacts`, `validFullObjectID`, `StatusFinding` usage), `internal/app/finalize_context.go` / the closeout integration fixtures (fake GitHub seams: `fakeCloseoutGitHub`), `internal/app/finalize_rebase.go` (`FinalizeDeps` shape).

**Interfaces:**
- Consumes: `gitcli.ProvePreserved` (Task 3), `domain.DeriveCarriedSet` (Task 4), existing `probeDescendantFacts`, `validFullObjectID`, `originRemote`.
- Produces (used by Tasks 6–10):

```go
// The closed per-descendant preservation finding categories.
const (
	CarryFindingRelationship = "carry-relationship-unproven" // DeriveCarriedSet token
	CarryFindingMissingMerge = "carry-merge-id-missing"      // no usable merge-result id (missing evidence)
	CarryFindingUnpreserved  = "carry-content-unpreserved"   // proof RAN and observed unproven
)
// ReasonCarryUnproven is the shared refusal reason rebase/publish/merge/stacked
// closeout report when a carried descendant's preservation is not proven.
const ReasonCarryUnproven = "carried-descendant-unproven"

type carryProof struct {
	Proven   bool
	Findings []StatusFinding // one per unproven descendant; empty when proven
}

// proveCarriedOnHead proves every descendant the parent's branch promises to
// carry is preserved at exactly the immutable target commit. An empty carried
// set is vacuously proven with ZERO external probes. A returned error is an
// observation/external failure (map to unknown; retain). Findings distinguish
// missing evidence (CarryFindingMissingMerge) from an observed mismatch
// (CarryFindingUnpreserved); each names descendant id, canonical PR, source
// merge id, target id, and category. Results are valid ONLY for this target id.
func proveCarriedOnHead(ctx context.Context, deps FinalizeDeps, repoDir string, gitRepo gitcli.Repository, snap domain.Snapshot, parent domain.Change, target gitcli.ObjectID) (carryProof, error)
```

- [ ] **Step 1: Write the failing tests** (real git repos via the closeout fixture helpers, fake GitHub for facts — `green-suite-untested-branch`: the fake must return real-shaped `MergedFacts` with real 40-hex OIDs that EXIST in the fixture repo for positive rows):
  - no stack descendants → `Proven: true`, and the fake GitHub records ZERO probe calls (side-effect witness on the fake, plus the companion assert that the witness fires on a row that DOES probe).
  - one stacked-merged child whose real merge commit is an ancestor of `target` → proven, no findings.
  - child preserved by content only (rebased target) → proven.
  - child whose facts are missing → `Proven: false`, one finding, `Code == ReasonCarryUnproven`, message contains `carry-relationship-unproven` and the child id.
  - child merged but `MergeCommit` empty/invalid → finding category `carry-merge-id-missing` (missing evidence ≠ mismatch).
  - child with a real, existing merge commit whose content target dropped → finding category `carry-content-unpreserved`; assert the message names both the source merge id and the target id.
  - a `ProvePreserved` observation error (point the fixture at a merge id absent from repo+remote) → helper returns `err != nil`, `Findings` empty (never a verdict).

- [ ] **Step 2: Run to verify fail.** `go test -tags=integration ./internal/app -run '<chosen prefix>CarryHelper' -count=1` — FAIL.

- [ ] **Step 3: Implement** `finalize_preservation.go`:

```go
func proveCarriedOnHead(ctx context.Context, deps FinalizeDeps, repoDir string, gitRepo gitcli.Repository, snap domain.Snapshot, parent domain.Change, target gitcli.ObjectID) (carryProof, error) {
	if len(domain.StackDescendantsParentFirst(snap, parent.ID())) == 0 {
		return carryProof{Proven: true, Findings: []StatusFinding{}}, nil
	}
	ghRepo, err := deps.GitHub.DiscoverRepository(ctx, repoDir)
	if err != nil { return carryProof{}, err }
	facts, err := probeDescendantFacts(ctx, deps, ghRepo, snap, parent.ID())
	if err != nil { return carryProof{}, err }
	set, polFail := domain.DeriveCarriedSet(snap, parent.ID(), facts)
	if polFail != nil {
		return carryProof{Findings: []StatusFinding{{Code: ReasonCarryUnproven,
			Severity: string(domain.SeverityError),
			Message: "carried-set derivation refused: " + polFail.Reason}}}, nil
	}
	findings := []StatusFinding{}
	for _, d := range set {
		if d.Proof != "" {
			findings = append(findings, carryFinding(d.ID, "", string(target), CarryFindingRelationship, d.Proof))
			continue
		}
		mergeID := facts[d.ID].MergeCommit
		if !validFullObjectID(mergeID) {
			findings = append(findings, carryFinding(d.ID, mergeID, string(target), CarryFindingMissingMerge,
				"no usable merge-result commit id"))
			continue
		}
		check, perr := deps.Planning.Client.ProvePreserved(ctx, gitRepo, originRemote,
			gitcli.ObjectID(mergeID), target)
		if perr != nil { return carryProof{}, perr }
		if check.Outcome != gitcli.PreservationProven {
			findings = append(findings, carryFinding(d.ID, mergeID, string(target), CarryFindingUnpreserved,
				check.Detail+pathsSuffix(check.Paths)))
		}
	}
	return carryProof{Proven: len(findings) == 0, Findings: findings}, nil
}
```

Add the small `carryFinding(id domain.ChangeID, sourceMerge, target, category, detail string) StatusFinding` constructor (Code `ReasonCarryUnproven`, Severity error, Message formatted `"change %04d (PR %s): %s: %s [source %s -> target %s]"` pulling the PR from `facts` — pass what the message needs) and `pathsSuffix`. Check `deps.Planning.Client`'s concrete type: if it is an interface seam rather than `*gitcli.Client`, add `ProvePreserved` to that interface and to any test doubles the compiler then flags (spec: no caller assembles ad hoc shell proofs — the seam is the typed method).

- [ ] **Step 4: Run to verify pass** (Step 2 command). Also `go build ./...` and `go test ./internal/app -count=1 -run 'TestUnit' ` equivalent short run to catch interface breakage; fix doubles.

- [ ] **Step 5: Commit.** `git add internal/app/finalize_preservation.go internal/app/finalize_preservation_integration_test.go && git add -u && git commit -m "feat(0327): shared app carry-preservation helper with structured findings"`

---

### Task 6: enforce at `FinalizeRebase` — pre-rewrite and the post-rewrite chokepoint

**Files:**
- Modify: `internal/app/finalize_rebase.go`
- Test: `internal/app/finalize_rebase_integration_test.go` (extend, existing shard prefix)

**Interfaces:**
- Consumes: `proveCarriedOnHead`, `ReasonCarryUnproven` (Task 5).
- Produces: two new enforcement points; no new exported symbols. New reason constant lives in Task 5's file (shared).

- [ ] **Step 1: Write the failing tests** (extend the rebase integration file; reuse its fixtures — read how it seeds a stacked child; if it has none, seed like `TestIntegrationFinalizeCloseoutRootCarry` does, but with a REAL child merge commit on the parent branch):
  - **pre-rewrite refusal** (spec Tests §3): local/remote/PR heads all agree on `req.Head`, but the parent branch at that head has LOST the child's merged content (build: merge child into parent, then hard-reset parent past the merge and force the fixture remote to match, keeping the child merge commit alive via a tag or dangling ref so the object exists). `FinalizeRebase` → `Result` blocked, `Reason == ReasonCarryUnproven`, and pin the missing effects: NO rebase receipt exists afterward (`deps.Workspace.ReadRebaseReceipt` → absent), workspace head unchanged, remote untouched. This exercises the NEW proof, not the existing mismatch guard — assert the heads agreed by construction.
  - **pre-rewrite pass-through**: same stack with content intact → proceeds past the proof (reaches the normal rebase outcome).
  - **post-rewrite refusal** (spec Tests §4, rebase leg): a completed rewrite whose result dropped the child (drive the fixture through `BeginRebase` to completion onto a base constructed so the child's file vanishes — simplest: pre-create the owned receipt + completed state the recovery path adopts, with the rewritten head missing the child content) → blocked with `ReasonCarryUnproven`, AND the owned receipt still present + orig refs retained (assert receipt readable and `OrigHead` unchanged — the abort/repair flow stays available).
  - **green suite does not substitute**: the post-rewrite fixture carries green gate evidence; still refuses (the proof runs before/independently of gate evidence — assert reason is the carry reason, not a gate reason).

- [ ] **Step 2: Run to verify fail.** `go test -tags=integration ./internal/app -run '^TestIntegrationFinalizeRebase' -count=1` — new cases FAIL.

- [ ] **Step 3: Implement.**
  (a) Fresh-path pre-rewrite gate in `FinalizeRebase`, immediately after the `string(remoteHead) != req.Head` check and BEFORE `newRebaseAttempt`/`WriteRebaseReceipt` (a refusal leaves workspace, branch, receipt, and remote untouched):

```go
	proof, perr := proveCarriedOnHead(ctx, deps, repoDir, rc.repo, rc.snap, rc.change, gitcli.ObjectID(req.Head))
	if perr != nil {
		return rebaseRefusal(op, ResultExternalFailed, RebaseDispBlocked, ReasonCarryUnproven,
			"carried-descendant preservation could not be established: "+perr.Error(), id)
	}
	if !proof.Proven {
		r := rebaseRefusal(op, ResultBlocked, RebaseDispBlocked, ReasonCarryUnproven,
			"a carried descendant's merged work is not preserved at the agreed pre-rewrite head; refusing to rebase", id)
		r.Findings = append(r.Findings, proof.Findings...)
		return r
	}
```

(Confirm `rebaseContext` carries `snap`; if the field is named differently, use it — read `loadRebaseContext`. An observation failure maps to `ResultExternalFailed` per the failure contract: unknown/external, retained.)
  (b) Post-rewrite gate at the TOP of `composeLocalGate` (the single chokepoint the normal, `FinalizeRebaseContinue`, and receipt-recovery paths all funnel through — verify by reading its three callers, and note in a comment anchored on the quoted clause "composeLocalGate decides skip-or-run after a completed rebase" that the proof re-discovers the carried set fresh and never trusts a receipt boolean). Same code shape against the completed `head` parameter; on `!proof.Proven` return a blocked refusal that does NOT clear the receipt or owned refs (do not touch the existing clear/abort mechanics — return before them; read the function to place the early return before any receipt-pair mutation).

- [ ] **Step 4: Run to verify pass** (Step 2 command). Mutation-probe both boundaries (each with `-count=1`): delete gate (a) → the pre-rewrite test reddens on "receipt exists"; delete gate (b) → the post-rewrite test reddens; restore both. Record the two probe outcomes in the commit message body.

- [ ] **Step 5: Commit.** `git add -u internal/app && git commit -m "feat(0327): FinalizeRebase proves carried descendants before and after the rewrite"`

---

### Task 7: enforce at `FinalizePublish`

**Files:**
- Modify: `internal/app/finalize_publish.go`
- Test: extend the publish coverage where it lives (find it: `grep -rn "FinalizePublish" internal/app/*_test.go`; add integration cases under the same shard prefix as the file they extend).

**Interfaces:** Consumes `proveCarriedOnHead`, `ReasonCarryUnproven`. New reason mapping: blocked refusal with `PublishDispBlocked`; observation error → `ResultExternalFailed` + `PublishDispUnknown`.

- [ ] **Step 1: Write the failing tests** (spec Tests §4, publish leg):
  - a valid receipt/attempt whose branch lost the carried child: `FinalizePublish` refuses with `ReasonCarryUnproven` and — pin the mechanism — the workspace seam records ZERO `PublishRewrite` calls (instrument the fake/seam the existing publish tests use; add the invocation-witness plus its companion "witness fires on the happy path" assert), and the remote feature ref tip is unchanged.
  - covers recovered/direct invocation: `FinalizePublish` is the single entry (verify by grepping callers); assert the gate sits before the rewrite for a crash-replay-shaped fixture too (receipt present, remote already at old head).
  - ordinary change (no descendants): publishes exactly as before, zero GitHub descendant probes.

- [ ] **Step 2: Run to verify fail** (`-count=1`, the file's shard prefix).

- [ ] **Step 3: Implement**: in `FinalizePublish`, after the receipt/attempt authorization block (the `receipt.ChangeID != … || receipt.Attempt != req.Attempt` refusal) and BEFORE `deps.Workspace.PublishRewrite`, insert the same proof shape as Task 6(a) against `gitcli.ObjectID(req.Head)`, using `wc.snap`/`wc.change`/`wc.repo` (read `loadWorkspaceContext` for exact field names). Unproven → `publishRefusal(ResultBlocked, PublishDispBlocked, ReasonCarryUnproven, "a carried descendant's merged work is not preserved at the publication head; refusing to push", id)` with findings appended; error → `newPublishResult(ResultExternalFailed, …PublishDispUnknown… Reason: ReasonCarryUnproven…)`. Content proof cannot authorize a different head or broaden the lease — the proof adds a conjunct and changes nothing about the existing lease plumbing (do not touch `PublishRewrite`'s inputs).

- [ ] **Step 4: Run to verify pass.** Mutation-probe: delete the gate → the zero-push-calls test reddens; restore; re-run `-count=1`.

- [ ] **Step 5: Commit.** `git add -u internal/app && git commit -m "feat(0327): FinalizePublish proves carried descendants before any remote rewrite"`

---

### Task 8: enforce at `FinalizeMerge`

**Files:**
- Modify: `internal/app/finalize_merge.go`
- Test: `internal/app/finalize_merge_integration_test.go` (extend, its shard prefix)

**Interfaces:** Consumes `proveCarriedOnHead`, `ReasonCarryUnproven`.

- [ ] **Step 1: Write the failing tests** (spec Tests §5):
  - all conjuncts hold, but the freshly verified PR head lost a carried child: refusal `ReasonCarryUnproven`, and the fake GitHub records ZERO `MergePullRequest` calls (witness + companion assert).
  - gate-off path (`finalize.gate: off` in fixture config) enforces the same proof — the proof runs at merge regardless of gate mode.
  - direct-merge entry (however the fixture reaches `FinalizeMerge` without a prior rebase — an explicit-id request) enforces it too.
  - concurrent head move still fails the EXISTING exact-head/lease conjunct first (fixture: PR head ≠ req.Head) — assert the existing conjunct token, proving the new gate complements, not replaces (spec: "the existing explicit lease / matching-head authorization must reject that movement").
  - already-merged short circuit (line "`outcome == githubcli.MergeAlreadyMerged`") is unchanged: no new proof there (closeout owns post-merge proof) — assert an already-merged fixture still returns `MergeDispAlreadyMerged`.
  - ordinary change: merges as before.

- [ ] **Step 2: Run to verify fail** (`-count=1`).

- [ ] **Step 3: Implement**: in `FinalizeMerge`, after `conj.FirstFailure()` returns empty and BEFORE `deps.GitHub.MergePullRequest`, the Task 6(a) proof shape against `gitcli.ObjectID(req.Head)` with `mc.snap`/`mc.change`/`mc.repo`. Unproven → `mergeRefusal(ResultBlocked, MergeDispBlocked, ReasonCarryUnproven, "a carried descendant's merged work is not preserved at the verified PR head; refusing to merge", id)` + findings; error → `ResultExternalFailed`/`MergeDispUnknown`/`ReasonCarryUnproven`.

- [ ] **Step 4: Run to verify pass.** Mutation-probe: delete the gate → zero-merge-calls test reddens; restore; `-count=1`.

- [ ] **Step 5: Commit.** `git add -u internal/app && git commit -m "feat(0327): FinalizeMerge proves carried descendants before the external merge"`

---

### Task 9: enforce at stacked closeout (child → live parent)

**Files:**
- Modify: `internal/app/finalize_closeout.go` (`closeoutStacked` and its caller in `FinalizeCloseout`)
- Test: `internal/app/finalize_closeout_integration_test.go` (extend)

**Interfaces:**
- Consumes: `gitcli.ProvePreserved`, `validFullObjectID`, existing `FetchBranch`.
- Produces: new reason constant in `finalize_closeout.go`'s reason block:

```go
	// ReasonCloseoutStackedUnpreserved: the child's merge result is not preserved
	// at the parent's freshly pinned remote head; a historical PR destination is
	// not evidence the parent currently carries the merge.
	ReasonCloseoutStackedUnpreserved = "stacked-merge-not-preserved"
```

- [ ] **Step 1: Write the failing tests** (spec "Stacked and root closeout" ¶1):
  - fresh marking: child PR merged into parent branch, but parent's remote branch was since rewritten WITHOUT the child's content → `FinalizeCloseout(child)` refuses (`ResultBlocked`, `CloseoutDispBlocked`, `ReasonCloseoutStackedUnpreserved`), metadata ref tip unchanged, child record still `implemented`.
  - fresh marking, content preserved after a parent rebase (child merge id rewritten away but blobs preserved) → `CloseoutDispStackedMerged` applied (exact-content arm accepts a legitimate rewrite).
  - replay path: record ALREADY `stacked-merged`, parent branch since dropped the content → the replay REFUSES with `ReasonCloseoutStackedUnpreserved` instead of returning `CloseoutDispAlready` (spec: "This also applies to the corresponding replay path"); with content intact, replay still returns `CloseoutDispAlready` (idempotency preserved when the promise still holds).
  - facts with unusable merge id → refusal whose message distinguishes missing evidence (reuse the `CarryFindingMissingMerge` category string in the message).
  - fetch/probe error on the parent branch → `ResultExternalFailed`/`CloseoutDispUnknown` (never blocked, never applied).

- [ ] **Step 2: Run to verify fail.** `go test -tags=integration ./internal/app -run '^TestIntegrationFinalizeCloseout' -count=1` — new cases FAIL.

- [ ] **Step 3: Implement**: add to `finalize_closeout.go`:

```go
// requireStackedPreservation pins the parent's remote branch head and proves the
// child's merge-result commit preserved there. It gates BOTH the fresh
// stacked-merged marking and the already-stacked-merged replay: a historical PR
// destination is a relationship, not evidence the parent still carries the work.
func requireStackedPreservation(ctx context.Context, deps FinalizeDeps, cc *closeoutContext, parentBranch string, facts githubcli.MergedFacts) *CloseoutResult
```

Body: `rev, err := deps.Planning.Client.FetchBranch(ctx, cc.repo, originRemote, gitcli.RefName(branchRefPrefix+parentBranch))` — error → unknown result (`ReasonCloseoutDestinationProbe`); `!validFullObjectID(facts.MergeCommit)` is already guarded upstream by `reprobeMerged` (verify by reading it; if so, note it in a comment rather than re-checking — `duplicated-gate-copies-the-whole-predicate` warns against a partial re-copy); `check, perr := deps.Planning.Client.ProvePreserved(ctx, cc.repo, originRemote, gitcli.ObjectID(facts.MergeCommit), rev.Commit)` — `perr` → unknown; `check.Outcome != gitcli.PreservationProven` → blocked refusal `ReasonCloseoutStackedUnpreserved` with the detail and paths in the message. Call it in `closeoutStacked` BEFORE the `cc.change.Status() == domain.StatusStackedMerged` replay short-circuit AND before the transaction (one call at the function top covers both). Return `*refusal` when non-nil.

- [ ] **Step 4: Run to verify pass.** Mutation-probe: delete the `requireStackedPreservation` call → the replay-refuses test and the fresh-refusal test both redden; restore; `-count=1`.

- [ ] **Step 5: Commit.** `git add -u internal/app && git commit -m "feat(0327): stacked closeout proves the child's merge survives at the parent's pinned head"`

---

### Task 10: enforce at root closeout + replace the fabricated fixture

**Files:**
- Modify: `internal/app/finalize_closeout.go` (`closeoutIntegrationDestination`)
- Modify: `internal/app/finalize_closeout_integration_test.go` (replace the fabricated-child positive fixture; add negatives)

**Interfaces:** Consumes `gitcli.ProvePreserved`, `IsAncestor`, `probeDescendantFacts` output (`descFacts`), existing `set`/`RootCloseoutProven`. Produces no new exported symbols; reuses `ReasonCloseoutChildUnproven` for a proven-content failure and `ReasonCloseoutProbeUnknown`/`ReasonCloseoutDestinationProbe` for observation failures, plus a missing-evidence variant message via `CarryFindingMissingMerge`.

- [ ] **Step 1: Replace the fabricated positive fixture** (spec Tests §1, §7; learning `green-suite-untested-branch`): in `TestIntegrationFinalizeCloseoutRootCarry`, rebuild `seed` so the descendant (id 6) has a REAL merge: create a commit on the root's feature branch adding `gadget.txt` (via `f.repo` helpers — read `setupCloseoutFixture` and `writerAdvance`/`mergeIntoBase` to find the branch-commit helper; add one if the fixture only writes the metadata branch: commit to the feature branch in the origin as `mergeIntoBase` does for main), record its real OID as PR 8's `MergeCommit` with `BaseRef` = the root's recorded branch, then `f.mergeIntoBase(t)` so the root's merge carries `gadget.txt` into main. `all-proven-archives-root-and-descendants` then proves via ancestry-or-content and still archives both records — assertions unchanged.

- [ ] **Step 2: Add the new sub-tests** to `TestIntegrationFinalizeCloseoutRootCarry`:
  - `absent-merge-object-refuses`: keep a WELL-FORMED but nonexistent child merge id (`strings.Repeat("b", 40)`) → NOT applied/no-op; disposition `CloseoutDispUnknown` (fetch of the exact object fails at the local-file origin → observation failure), metadata ref tip unchanged, both records still active (the old fixture's exact bytes, now a negative — it "must no longer archive anything").
  - `descendant-dropped-refuses` (spec Tests §7): real child merge commit EXISTS (tagged so it survives), but the root branch was rewritten to drop `gadget.txt` before `mergeIntoBase` → blocked, `ReasonCloseoutChildUnproven`, and pin the missing effects: origin metadata tip byte-identical, no archive files, board unchanged, child's feature/recovery refs untouched. Assert `git cat-file -e` on the child merge id first (loss ≠ missing object).
  - `squash-rewrite-archives` (spec Tests §1, §8): child squashed into the root branch (child merge id NOT an ancestor of the root merge result, blobs identical) → root-archived via the exact-content fallback against the ROOT's merge-result commit.
  - `integration-advanced-after-root-merge` (spec Tests §8, and step 4 of the spec's root-closeout list): after `mergeIntoBase`, advance main with an unrelated commit that EDITS `gadget.txt` → closeout still archives, because the fallback targets the root merge result, not the integration tip. This is the discriminating fixture for target choice — it reddens if the implementation compares against the tip.
  - `maintenance-sweep-same-refusal`: whatever op the sweep uses reaches the same `FinalizeCloseout` — grep `FinalizeCloseout(` callers; if the sweep calls it directly, a comment + one assertion through the sweep entry (or, if that is disproportionate, verify the single-callee property by grep and record it in the test file header comment as a quoted-clause anchor).

- [ ] **Step 3: Run to verify fail.** `go test -tags=integration ./internal/app -run '^TestIntegrationFinalizeCloseoutRootCarry' -count=1` — `descendant-dropped-refuses`, `squash-rewrite-archives`, `integration-advanced-after-root-merge` FAIL (no Git proof exists yet); the replaced positive still passes (ancestry facts are now real but unchecked).

- [ ] **Step 4: Implement** in `closeoutIntegrationDestination`, after the `RootCloseoutProven(set)` block and before `archiveDateFromMerge`:

```go
	// Relationship proven; now prove CONTENT in Git for every carried descendant.
	// Direct ancestry in the pinned integration history suffices; otherwise the
	// child's merge result must be exactly preserved at the ROOT's verified merge
	// result — proving what the root delivered, immune to later integration
	// commits touching the same files. Content similarity never rescues a failed
	// relationship proof (that returned above).
	for _, d := range set {
		mergeID := descFacts[d.ID].MergeCommit
		if !validFullObjectID(mergeID) {
			return closeoutRefusal(ResultBlocked, CloseoutDispBlocked, ReasonCloseoutChildUnproven,
				fmt.Sprintf("change %04d: %s: no usable merge-result commit id; the root stays recoverable", int(d.ID), CarryFindingMissingMerge), id)
		}
		reach, err := deps.Planning.Client.IsAncestor(ctx, cc.repo, gitcli.ObjectID(mergeID), rev.Commit)
		if err != nil {
			return newCloseoutResult(ResultExternalFailed, CloseoutResult{ID: id,
				Disposition: CloseoutDispUnknown, Reason: ReasonCloseoutDestinationProbe, Message: err.Error()})
		}
		if reach { continue }
		check, perr := deps.Planning.Client.ProvePreserved(ctx, cc.repo, originRemote,
			gitcli.ObjectID(mergeID), gitcli.ObjectID(facts.MergeCommit))
		if perr != nil {
			return newCloseoutResult(ResultExternalFailed, CloseoutResult{ID: id,
				Disposition: CloseoutDispUnknown, Reason: ReasonCloseoutDestinationProbe, Message: perr.Error()})
		}
		if check.Outcome != gitcli.PreservationProven {
			return closeoutRefusal(ResultBlocked, CloseoutDispBlocked, ReasonCloseoutChildUnproven,
				fmt.Sprintf("change %04d: %s: %s [source %s -> target %s]; the root stays recoverable",
					int(d.ID), CarryFindingUnpreserved, check.Detail, mergeID, facts.MergeCommit), id)
		}
	}
```

(`rev` is the integration fetch already in scope; `facts.MergeCommit` is the root's verified merge result. Only after every proof does the existing atomic transaction run — contention already re-plans through the engine; the proof sits before `runCloseoutArchiveTransaction`, so a contended transaction retry re-enters `FinalizeCloseout` and redoes the proofs on the fresh snapshot, never narrowing the target set — verify that retry shape by reading `mapOutcome`/engine contention handling and note it in the comment.)

- [ ] **Step 5: Run to verify pass** (Step 3 command). Then the headline mutation (spec's own necessity probe, inverted): revert the loop (delete it) → `descendant-dropped-refuses` and `integration-advanced-after-root-merge`… the second should still PASS (it asserts success) — so the deletion must redden `descendant-dropped-refuses` and `absent-merge-object-refuses`; confirm exactly those redden, restore, re-run `-count=1`.

- [ ] **Step 6: Commit.** `git add -u internal/app && git commit -m "feat(0327): root closeout proves every carried descendant in Git; fabricated fixture replaced with a real stack"`

---

### Task 11: transitive/open-intermediate boundary tests + comment and doc truth-up

**Files:**
- Modify: `internal/app/finalize_closeout_integration_test.go` or the rebase/merge test files (one transitive end-to-end case)
- Modify: `internal/domain/stackcloseout.go`, `internal/app/finalize_closeout.go` (comments)
- Modify: whatever maintained docs describe stacked closeout — find them: `grep -rln "stacked-merged\|stacked_on" README.md docs/reference/ docs/*.md internal/` (executable-vs-prose sort per AGENTS.md; point-in-time records — archived changes, specs, ADRs, results — are NOT edited)

- [ ] **Step 1: Write the failing transitive test** (spec Tests §6) at one pre-merge boundary (merge is cheapest): root←A(stacked-merged, real merge into root's branch)←B(stacked-merged, real merge into A's branch), all content present → `FinalizeMerge` proceeds; drop B's content from the root head → refuses naming B in a finding (every descendant verified, transitively). Add the open-intermediate case: root←C(open, in-progress)←D(stacked-merged into C); D's content absent from root's head → `FinalizeMerge` still proceeds (D is not promised by the root pre-merge). Run to verify the drop-B case fails before implementation only if Task 8 landed without transitive coverage — if it already passes, mutation-verify instead: stub `DeriveCarriedSet`'s recursion (as in Task 4 Step 4) and confirm the drop-B integration test reddens (`-count=1`), then restore.

- [ ] **Step 2: Comment truth-up** (now that the code makes it true): in `stackcloseout.go`, reword `CarriedDescendant`'s and `DeriveRootCloseoutSet`'s doc comments — a chain of merged PR destinations establishes the carry RELATIONSHIP; Git preservation of the content is proven separately by the caller ("comments must stop calling destination checks alone proof of shipped code"). In `finalize_closeout.go`'s file header, extend shape 3's paragraph: after "proves, from the authoritative graph and live PR facts, the chain of merged destinations", add that each descendant is then proven in Git (ancestry in the pinned integration history, or exact-content at the root's merge result) before any archive write.

- [ ] **Step 3: Maintained docs.** From the grep in **Files**, update every maintained (non-archive, non-ADR, non-spec) description of stacked closeout/finalize so a merged destination is described as a relationship check and verified Git carry as the promotion condition. If any updated doc is a source for generated/installed copies (check `internal/install`/`internal/assets` for embedded copies: `grep -rln "stacked" internal/assets internal/install` and read how they regenerate), regenerate through the supported workflow rather than hand-editing the copy.

- [ ] **Step 4: Run** `go test ./internal/domain ./internal/app -count=1` (unit) and the touched integration shards; green.

- [ ] **Step 5: Commit.** `git add -u && git commit -m "docs(0327): destination checks are relationship proofs; Git preservation is the promotion condition"`

---

### Task 12: consolidated mutation matrix + full-suite gate

**Files:** none created (probes are applied and reverted; results recorded in the commit message and later the results file). Working notes may go in the scratchpad, never committed.

- [ ] **Step 1: Enforcement-boundary mutation matrix** (spec: "Mutation-test each new enforcement boundary and the descendant-population derivation… Pin the missing push/merge/archive effect and reason, not merely a nonzero exit. Defeat Go's test cache."). For each row: apply the mutation to a COPY-backed working tree edit (keep `git stash push` or a file backup of your uncommitted state; never `git checkout --` over uncommitted work — `mutation-restore-needs-a-backup-copy`), run the named test with `-tags=integration -count=1`, confirm the named assert reddens on the named effect, restore, confirm green.

| # | Mutation | Must redden (test / pinned effect) |
|---|---|---|
| 1 | Delete `FinalizeRebase`'s pre-rewrite `proveCarriedOnHead` call | Task 6 pre-rewrite test: receipt now EXISTS / reason wrong |
| 2 | Delete the proof at the top of `composeLocalGate` | Task 6 post-rewrite test: success returned despite dropped child |
| 3 | Delete `FinalizePublish`'s proof | Task 7: `PublishRewrite` witness fires (nonzero push attempts) |
| 4 | Delete `FinalizeMerge`'s proof | Task 8: `MergePullRequest` witness fires |
| 5 | Delete `requireStackedPreservation` call | Task 9: fresh + replay tests both redden |
| 6 | Delete the root-closeout per-descendant loop | Task 10: `descendant-dropped-refuses` archives / `absent-merge-object-refuses` archives |
| 7 | In `DeriveCarriedSet`, stop recursing (`walk(c)` removed) | Task 4 transitive unit test AND Task 11's drop-B integration test |
| 8 | In `ProvePreserved`, treat `len(bases) > 1` as single-base (use `bases[0]`) | Task 3 `multiple-bases` row |
| 9 | In `ProvePreserved`, return proven on empty delta | Task 3 `empty-delta-non-ancestral` row |

Each row's redden must be the NAMED test failing on the NAMED effect — if a row survives green, that is a defect to fix (a vacuous assert or a missed wiring), not a row to skip (`residual-is-for-undetectable-not-unprobed`).

- [ ] **Step 2: Full-suite gate.** Resolve the current `build.test_command` from config (read `.docket.yml` / the config layers — never a second copy) and run it from the feature worktree; today that is `go run ./cmd/docket development test`. All green; treat any `BUDGET WATCH:`/`PARALLEL-SENSITIVE:` line as a screening finding to note and any `SERIAL CONFIRMED OVER BUDGET:` as a breach to act on (confirm serially per `tests/README.md` before concluding; the new gitcli shard's 15s budget was set in Task 1 — if screening flags it, measure solo and right-size the budget row in a follow-up commit, never by inflating an existing row).

- [ ] **Step 3: Commit** anything the gate forced (formatting, budget row adjustment): `git add -u && git commit -m "test(0327): mutation matrix verified; suite green at the build gate"` (skip the commit if the tree is clean; record the matrix outcomes in the build's evidence/results notes either way).

---

## Self-Review (performed while writing; executor re-checks)

- **Spec coverage:** primitive algorithm steps 1–6 → Tasks 1–3; app collection (pre-merge + root) → Tasks 4–5, 10; rebase/continue/recovery → Task 6; publish/merge → Tasks 7–8; stacked marking + replay → Task 9; root closeout ordering, root-merge-result target, atomicity/contention → Task 10; fixture replacement → Task 10 Steps 1–2; transitive/open-intermediate → Tasks 4, 11; failure/recovery contract → the Result/Disposition/Reason mappings named in each enforcement task; docs → Task 11; mutation + suite gate → Task 12. Spec Tests §1–§11 map to: §1→T3/T10, §2→T3 rows 5,15 + T10, §3→T6, §4→T6/T7, §5→T8, §6→T4/T11, §7→T10, §8→T3/T10, §9→T3, §10→T3 row 7, §11→T10 (transaction behavior asserted unchanged; replay/contention rides existing engine tests plus T10's comment-verified retry shape).
- **Known unknowns the builder must resolve on contact (not placeholders — verification steps are named inline):** exact context-struct field names (`rc.snap`, `wc.snap`, `mc.snap`), the `Planning.Client` seam's interface-vs-concrete shape (Task 5 Step 3), app integration shard prefixes (Task 5 Files), and whether `TestIntegrationPreserveCommit` collides with the existing checkout-preservation tests (Task 1). Each names its fallback.
- **Type consistency:** `proveCarriedOnHead(ctx, deps, repoDir, gitRepo, snap, parent, target)` is spelled identically in Tasks 5–8; `ProvePreserved(ctx, repo, remote, source, target) (PreservationCheck, error)` identically in Tasks 3, 5, 9, 10; `DeriveCarriedSet(s, parent, facts)` in Tasks 4, 5, 12.
