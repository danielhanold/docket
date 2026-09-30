<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0472 — Rename change version to revision (--version → --revision)](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0472-rename-change-version-to-revision-version-revision.md)**
<!-- docket:backlink:end -->
# Rename change version to revision Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. In this repo the plan is executed by `docket-build` (one profile worker per task, sequential, one full-suite gate at the end).

**Goal:** Rename the record-revision pin from "version" to "revision" on every wire surface (ADR-0129 family (b), rows 39-44, 40a, 41a, 41b, 43a, 44a) as one hard cut with no aliases, and seal the retired spellings.

**Architecture:** Behavior-neutral Go identifier renames land first (gopls, type-aware), so every later task is a small, reviewable wire change. Then the app layer's JSON keys and refusal codes move, then the CLI flags, then the skills (plus regenerated embedded copies) and maintained docs. The retired-vocabulary seal comes last, once no old spelling survives. It gets fifteen new rows, a new bound-flag kind (text blocks bound to a change/finalize/workspace operation reference derived from the live catalog, and Go flag calls in `internal/cli`), and a schema-registry walk for the bare `version` key. A claim-digest stability test is written first, against the untouched tree, so the committed `Docket-Request-Digest` values are pinned before anything moves.

**Tech Stack:** Go (cobra CLI, `encoding/json`, `go/scanner`), `gopls rename` (`/Users/homer/go/bin/gopls`), `go generate ./internal/assets/` (`cmd/genassets`), the Go suite runner (`go run ./cmd/docket development test`).

**Spec:** `docs/superpowers/specs/2026-09-29-rename-change-version-to-revision-version-revision-design.md` (on the `docket` metadata branch; synchronized copy at `/Users/homer/dev/docket/.docket/docs/superpowers/specs/2026-09-29-rename-change-version-to-revision-version-revision-design.md`). Decision record: ADR-0129 family (b) rows 39-45, 40a, 41a, 41b, 43a, 44a (amended in place at grooming).

## Global Constraints

- Hard cut, no aliases, no hidden flag, no dual-spelling decode, no deprecation window (ADR-0129 Decision 2). An old flag must be unknown to cobra (exit 2, `unknown flag: --version`); an old request key must fail strict decoding (`json: unknown field "version"`).
- Row 40: `--version` → `--revision` on exactly these 14 ops: `change.{attach-plan, attach-results, claim, halt, mark-implemented, reclaim, refresh-claim, resume-halted}`, `finalize.{block, clear-block, merge, rebase, retarget-children}`, `workspace.prepare`.
- Row 40a: `change repair-identity --expect-version` → `--expect-revision`.
- Row 41: request `version` → `revision` on `change.{block, defer, groom, kill, reconcile, revive, unblock}`, `learning.update` and every flag-built request (including `workspace.inspect`'s shared `WorkspaceIDRequest`); `status` `changes[].version`; `context.implementation` `change.version` / `spec.version`; `context.finalize` `candidates[].version` → `revision`.
- Row 41a: `status --records` `records[].version` → `revision`. Row 41b: `context.finalize` `candidates[].pr.version` → `pr.revision`.
- Row 42: `spec_version` → `spec_revision`. Row 43: `target.version` → `target.revision`. Row 43a: `change.version` (adr.record; `successor.change.version`) → `change.revision`. Row 44: `pr_version` → `pr_revision`.
- Row 44a codes: `version-mismatch`→`revision-mismatch`, `version-drift`→`revision-drift`, `spec-version-mismatch`→`spec-revision-mismatch`, `reclaim-version-missing`→`reclaim-revision-missing`, `empty-version`→`empty-revision`, `empty-change-version`→`empty-change-revision`, `empty-target-version`→`empty-target-revision`, `empty-spec_version`→`empty-spec_revision`, `invalid-spec_version`→`invalid-spec_revision`, `empty-child_pr_version`→`empty-child_pr_revision`.
- MUST NOT change (Decision 4): `protocol_version`, `schema_version`, `capability_version`, `format_version`, `go_version`, `product_version`; the `version` operation, `docket version`, its result key `version`, and `capabilities` `binary.version` (`BinaryIdentity.Version`, `VersionResult.Version`); the claim digest payload's JSON key `version` (`claimDigestPayload`, `internal/app/change_claim.go`); `resolver_budget_version` (row 45); `cmd/releasepkg --version`, the release downloader's `--version` (`internal/release/downloader/install.sh`, `scripts/release-smoke.sh`, `tests/test_release_downloader*.sh`, `.github/workflows/release-candidate.yml`); foreign CLIs' `--version` (`codex`, `cursor-agent`, `opencode`); every commit-id `*_revision` key (`committed_revision`, `metadata_revision`, `*_branch_revision`, the run tracker's `revision` / `bound_revision`) and every result struct's existing `Revision` field; `buildinfo.Version`, `release.Version`, the `*SchemaVersion` family, `VersionDir`, `InitDefaultVersionFlag`.
- One meaning for "revision": the exact id of a pinned state. Record revision = git blob id; PR revision = snapshot hash; the existing commit-id keys stay as they are.
- Go identifiers follow their row's term: record/PR-revision fields, types, functions, parameters and locals named `*Version*`/`version` become `*Revision*`/`revision`. Where a renamed local meets an existing commit-meaning `revision` in the same scope, the commit one becomes `committedRevision` (gopls refuses the rename on a conflict; that refusal is the signal). No file renames.
- Placeholder in flag help: the backticked word becomes `revision`, so catalog signatures read `--revision <revision>` / `--expect-revision <revision>`.
- Find sites with a whole-repo grep, never a hand list. The site lists in this plan were traced at base `27278a31d` and are starting points; the grep is authoritative.
- Never edit point-in-time records: `docs/changes/**`, `docs/results/**`, `docs/superpowers/**` (other than this plan), `docs/adrs/**` (ADR-0129 is delivered through the change's `adrs: [129]`; edit it only to record a newly found name, and only with the human's explicit authorization). Frozen corpora (`**/testdata/**`, `tests/fixtures/**`, `internal/install/legacydata/**`) are data, not source.
- After editing anything under `skills/`, run `go generate ./internal/assets/` and commit `internal/assets/embedded/**` in the same commit (`TestEmbeddedMatchesAuthored`). Never hand-edit `internal/assets/embedded/**`.
- Stage explicit paths only (`git add <path> …`), never `git add -A` / `git add .`.
- Go test commands always pass `-count=1`. Build tags: 139 files carry `//go:build integration`; `internal/app/finalize_e2e_test.go` carries `//go:build e2e`. Every task that touches Go ends with `go vet ./... && go vet -tags integration ./... && go vet -tags e2e ./...` clean, so no tagged file is left referencing an old name.
- Mutation tests restore from a backup copy (`cp f f.bak; mutate; test; mv -f f.bak f`), never `git checkout -- f`. Confirm each mutation landed (`grep -c`) before reading the test result.
- Shell: the agent's interactive shell is zsh and its `grep` is ugrep. Run multi-line shell steps under `bash`, keep regexes portable, and capture producer output into a variable before `grep` (AGENTS.md pipefail rule).
- The build gate runs the whole suite through `build.test_command` (`go run ./cmd/docket development test`), not only the tests named here. Read its budget report even when green.

## Review Focus

1. **A flag registration renamed but a read left stale.** `c.Flags().GetString("version")` on an unregistered flag returns `""` with an ignored error, so the op sees an empty revision and refuses `empty-revision` for a caller who passed `--revision`. Expected: the value passed as `--revision` reaches the request on every op. Pinned by Task 4's `TestRevisionFlagReachesRequest` (paired empty-value control per op, plus its mutation step) and by Task 7's Go bound-flag seal.
2. **An in-flight caller (an old loaded skill) still passing `--version` / `--expect-version`.** Expected: a loud `unknown flag` failure, exit 2, never a silently ignored flag. Pinned by Task 4's `TestRevisionFlagHardCut`, which walks the live cobra tree in both directions.
3. **An old request file carrying `version`, `spec_version`, `target.version`, `change.version` or `pr_version`.** Expected: invalid-input naming the unknown field (and, for a top-level key, the accepted-key list naming its replacement), never silently dropped. Pinned by Task 3's `TestRetiredRevisionRequestKeysRefused`.
4. **A lost-response claim retry straddling the upgrade.** Expected: the new binary computes the same idempotency digest as the old one, so the retry replays instead of re-allocating. Pinned by Task 1's `TestClaimDigestStableAcrossRevisionRename` (fixed pre-rename digest values) and its tag mutation.
5. **Kept namesakes caught by the new seal.** `docket version --json`'s `version` key, `capabilities` `binary.version`, the release tools' and foreign CLIs' `--version`, `resolver_budget_version`, the commit `*_revision` keys, the claim digest's `version` tag. Expected: all stay legal. Pinned by Task 7's negative controls (text, Go and schema-walk), its exact `(scope, path)` kept set, and the kept-set mutation.

---

### Task 1: Pin the claim idempotency digest before anything moves

Profile hint: economy.

**Files:**
- Test: `internal/app/claim_receipt_key_test.go` (add `TestClaimDigestStableAcrossRevisionRename`)

**Interfaces:**
- Consumes: `canonicalDigest(operation string, payload any) (transaction.RequestDigest, error)` (`internal/app/planning.go`), `claimDigestPayload{ID int; Version string; RunContextHash string}`, `OperationChangeClaim = "change.claim"`.
- Produces: the pinned digest test Task 2 carries through its field rename (gopls rewrites the `Version:` field name in this test; the JSON tag and the expected digests must not change).

- [ ] **Step 1: Write the test**

Append to `internal/app/claim_receipt_key_test.go`:

```go
// TestClaimDigestStableAcrossRevisionRename pins ADR-0129 Decision 4 (change
// 0472): the claim idempotency digest hashes a `version` key, and its digests
// are committed as Docket-Request-Digest trailers on the metadata branch. The Go
// field may be renamed, but a fixed (id, record revision, run-context hash) must
// still hash to the value computed before the rename, or a lost-response claim
// retry straddling the upgrade re-allocates instead of replaying.
func TestClaimDigestStableAcrossRevisionRename(t *testing.T) {
	const rev = "0123456789abcdef0123456789abcdef01234567"
	for _, c := range []struct{ name, hash, want string }{
		{"run-context", "sha256:run-context-hash", "sha256:da9fe54c15a964beee2ee8b4e241b5c8954bbf7b245b2c5b7b4ca3e3d110d143"},
		{"ungated", "", "sha256:c95b30b1a04b0de646ff621e7f24982df264a43c70ec7389dce8ae502ad4181a"},
	} {
		got, err := canonicalDigest(OperationChangeClaim, claimDigestPayload{ID: 472, Version: rev, RunContextHash: c.hash})
		if err != nil {
			t.Fatalf("%s: canonicalDigest: %v", c.name, err)
		}
		if string(got) != c.want {
			t.Errorf("%s: claim digest = %s, want the pre-rename %s (the payload's JSON keys must not move)", c.name, got, c.want)
		}
	}
	b, err := json.Marshal(claimDigestPayload{ID: 472, Version: rev})
	if err != nil || !strings.Contains(string(b), `"version":"`+rev+`"`) {
		t.Fatalf("claimDigestPayload marshals %s (err %v), want the committed \"version\" key", b, err)
	}
}
```

- [ ] **Step 2: Prove the expected values against the untouched tree**

The digests were computed at plan time from `sha256("change.claim" + "\x00" + json)`, not from the code. They are unverified until the test passes on the unmodified base.

Run: `cd /Users/homer/dev/docket/.worktrees/rename-change-version-to-revision-version-revision && go test -count=1 ./internal/app/ -run 'TestClaimDigestStableAcrossRevisionRename|TestClaimReceiptKeepsCommittedRunContextHashKey' -v`
Expected: PASS for both. If the digest assert fails on the untouched tree, the plan's constants are wrong. Replace them with the values the failure prints (the untouched tree is the oracle), then re-run to PASS.

- [ ] **Step 3: Mutation test (the assert detects a moved key)**

```bash
f=internal/app/change_claim.go
cp "$f" "$f.bak"
perl -0pi -e 's/(Version string `json:)"version"(`\n\t\/\/ RunContextHash)/$1"revision"$2/' "$f"
grep -c 'json:"revision"' "$f"   # must print 1: the mutation landed
go test -count=1 ./internal/app/ -run TestClaimDigestStableAcrossRevisionRename
mv -f "$f.bak" "$f"
go test -count=1 ./internal/app/ -run TestClaimDigestStableAcrossRevisionRename
```
Expected: the mutated run FAILs with `claim digest = … want the pre-rename …` for both cases; the restored run PASSes.

- [ ] **Step 4: Commit**

```bash
git add internal/app/claim_receipt_key_test.go
git commit -m "test(app): pin the claim idempotency digest across the revision rename (change 0472)"
```

---

### Task 2: Behavior-neutral Go identifier renames (record and PR revision)

Profile hint: standard.

No wire change except one: `RepairIdentityRequest.ExpectVersion` is untagged, so its schema key moves from `ExpectVersion` to `ExpectRevision` with the field. The other untagged structs renamed here are never persisted, so no stored key moves: `githubcli.PullRequest`, `MergedFacts`, `domain.PRFacts` and `transaction.EntityExpectation` (the transaction `manifest` stores no expectations). This was verified at plan time. `computeRevision` hashes snapshot values, not field names, so PR revision values are unchanged. Every JSON tag, string literal, flag name and code value stays as it is. Finding and reason constants are NOT renamed here; Task 3 renames them together with their values.

**Files (declarations; gopls rewrites every reference, tests included):**
- Modify: `internal/repository/transaction/types.go` (`VersionKind`, `VersionBlob`, `VersionAbsent`, `ExpectedVersion`, `EntityExpectation.Version`)
- Modify: `internal/githubcli/pr.go` (`PullRequest.Version`, `computeVersion`), `internal/githubcli/ensure.go` (`EnsurePullRequest` request field `ExpectedVersion`), `internal/githubcli/merge.go` (`MergedFacts.Version`), `internal/githubcli/retarget.go` (`RetargetPullRequest` param `expectedVersion`)
- Modify: `internal/domain/finalize.go` (`PRFacts.Version`)
- Modify: `internal/app/*.go` struct fields listed in Step 2, plus `sweepObservedVersion`, `blobVersions`, `hasSpecVersion`, and record/PR-revision locals
- Modify: `internal/cli/change.go` (`changeIDVersionSubcommand`, local `expectVersion`, locals `version`), `internal/cli/finalize.go`, `internal/cli/workspace.go` (locals `version`)
- Every `*_test.go` gopls touches.

**Interfaces:**
- Consumes: Task 1's test (its `Version:` field reference is renamed by gopls; its JSON-key assert and digests must stay green).
- Produces, for every later task: `transaction.ExpectedRevision{Kind transaction.RevisionKind; ObjectID gitcli.ObjectID}`, `transaction.RevisionBlob`, `transaction.RevisionAbsent`, `transaction.EntityExpectation.Revision`; `githubcli.PullRequest.Revision`, `githubcli.MergedFacts.Revision`, the ensure request's `ExpectedRevision`, `computeRevision`; `domain.PRFacts.Revision`; app request fields `.Revision` (all row-40/41 requests), `ChangeGroomRequest.SpecRevision`, `VerifiedMerge.PRRevision`, `AuthorizedChild.PRRevision`, `RepairIdentityRequest.ExpectRevision`, `ADRProducingChange.Revision`, `ADRTarget.Revision`; result fields `StatusChange.Revision`, `StatusRecord.Revision`, `ContextEntity.Revision`, `FinalizeCandidateReport.Revision`, `FinalizePRReport.Revision`; internal `StatusBlob.Revision`, `StatusArtifact.Revision`, `closeoutContext.revision`, `mergeContext.revision`, `rebaseContext.revision`, `workspaceContext.revision`; `sweepObservedRevision`; cli `changeIDRevisionSubcommand`.

- [ ] **Step 1: Write the rename helper**

```bash
REN=$(mktemp "${TMPDIR:-/tmp}/ren.XXXXXX")
cat > "$REN" <<'EOF'
#!/usr/bin/env bash
# ren.sh field <file> <Struct> <Field> <New>   rename one struct field, located inside its struct
# ren.sh decl  <file> <ERE>    <Old>   <New>   rename the identifier on the ONE line matching ERE
set -euo pipefail
mode=$1 f=$2 key=$3 old=$4 new=$5
if [ "$mode" = field ]; then
  line=$(awk -v s="$key" -v fl="$old" '$0 ~ "^type "s" struct" {in_s=1; next} in_s && /^}/ {exit} in_s && ($1==fl || $1==fl",") {print NR; exit}' "$f")
else
  hits=$(grep -n -E -e "$key" "$f" || true)
  [ -n "$hits" ] || { echo "ren: no line matches $key in $f" >&2; exit 1; }
  [ "$(printf '%s\n' "$hits" | wc -l | tr -d ' ')" = 1 ] || { echo "ren: $key matches more than one line in $f" >&2; exit 1; }
  line=${hits%%:*}
fi
[ -n "${line:-}" ] || { echo "ren: $old not found for $key in $f" >&2; exit 1; }
col=$(awk -v n="$line" -v s="$old" 'NR==n{print index($0,s)}' "$f")
[ "${col:-0}" -gt 0 ] || { echo "ren: $old not on $f:$line" >&2; exit 1; }
GOFLAGS=-tags=integration gopls rename -w "$f:$line:$col" "$new"
EOF
chmod +x "$REN"
```

`GOFLAGS=-tags=integration` makes gopls see the 139 integration-tagged files. The one `e2e`-tagged file is fixed in Step 4 by the compiler. gopls refuses a rename that would collide or shadow; treat a refusal as a stop, apply the `committedRevision` rule (Global Constraints), then retry.

- [ ] **Step 2: Rename the declarations**

Run from the worktree root, in this order (types before the fields that use them):

```bash
cd /Users/homer/dev/docket/.worktrees/rename-change-version-to-revision-version-revision
T=internal/repository/transaction/types.go
"$REN" decl  $T '^type VersionKind string'        VersionKind     RevisionKind
"$REN" decl  $T '^[[:space:]]+VersionBlob[[:space:]]'   VersionBlob     RevisionBlob
"$REN" decl  $T '^[[:space:]]+VersionAbsent[[:space:]]' VersionAbsent   RevisionAbsent
"$REN" decl  $T '^type ExpectedVersion struct'    ExpectedVersion ExpectedRevision
"$REN" field $T EntityExpectation Version Revision

"$REN" field internal/githubcli/pr.go    PullRequest Version Revision
"$REN" decl  internal/githubcli/pr.go    '^func computeVersion\(' computeVersion computeRevision
"$REN" decl  internal/githubcli/ensure.go '^[[:space:]]+ExpectedVersion string' ExpectedVersion ExpectedRevision
"$REN" field internal/githubcli/merge.go MergedFacts Version Revision
"$REN" decl  internal/githubcli/retarget.go '^func \(c \*Client\) RetargetPullRequest' expectedVersion expectedRevision
"$REN" decl  internal/domain/finalize.go '^[[:space:]]+Number, Version' Version Revision

A=internal/app
while read -r file st fld new; do "$REN" field "$A/$file" "$st" "$fld" "$new"; done <<'EOF'
adr_ops.go ADRProducingChange Version Revision
adr_ops.go ADRTarget Version Revision
change_attach.go ChangeAttachRequest Version Revision
change_claim.go ChangeClaimRequest Version Revision
change_claim.go claimDigestPayload Version Revision
change_groom.go ChangeGroomRequest Version Revision
change_groom.go ChangeGroomRequest SpecVersion SpecRevision
change_halt.go HaltRequest Version Revision
change_halt.go ResumeRequest Version Revision
change_implemented.go MarkImplementedRequest Version Revision
change_kill.go ChangeKillRequest Version Revision
change_lifecycle.go ChangeBlockRequest Version Revision
change_lifecycle.go ChangeDeferRequest Version Revision
change_lifecycle.go ChangeUnblockRequest Version Revision
change_lifecycle.go ChangeReviveRequest Version Revision
change_reclaim.go ChangeReclaimRequest Version Revision
change_reconcile.go ChangeReconcileRequest Version Revision
change_repair.go RepairIdentityRequest ExpectVersion ExpectRevision
finalize_block.go BlockRequest Version Revision
finalize_block.go ClearBlockRequest Version Revision
finalize_closeout.go closeoutContext version revision
finalize_context.go FinalizePRReport Version Revision
finalize_context.go FinalizeCandidateReport Version Revision
finalize_merge.go FinalizeMergeRequest Version Revision
finalize_merge.go VerifiedMerge PRVersion PRRevision
finalize_merge.go mergeContext version revision
finalize_rebase.go FinalizeRebaseRequest Version Revision
finalize_rebase.go rebaseContext version revision
finalize_retarget.go AuthorizedChild PRVersion PRRevision
finalize_retarget.go RetargetChildrenRequest Version Revision
implementation_context.go ContextEntity Version Revision
learning_ops.go LearningUpdateRequest Version Revision
status.go StatusBlob Version Revision
status.go StatusArtifact Version Revision
status_result.go StatusChange Version Revision
status_result.go StatusRecord Version Revision
workspace_ops.go WorkspaceIDRequest Version Revision
workspace_ops.go workspaceContext version revision
EOF
"$REN" decl $A/maintenance.go '^func sweepObservedVersion\(' sweepObservedVersion sweepObservedRevision
"$REN" decl internal/cli/change.go '^func changeIDVersionSubcommand\(' changeIDVersionSubcommand changeIDRevisionSubcommand
```

Do NOT rename `BinaryIdentity.Version` (`capabilities.go`) or `VersionResult.Version` (`version.go`): they are the kept software version.

- [ ] **Step 3: Rename the remaining locals and parameters, derived by grep**

```bash
out=$(git grep -n -w -E 'version|versions|blobVersions|hasSpecVersion|expectVersion|expectedVersion' -- 'internal/app/*.go' 'internal/cli/*.go' 'internal/githubcli/*.go' 'internal/domain/*.go' 'internal/repository/transaction/*.go' ':!*_test.go' || true)
grep -v -E '^[^:]+:[0-9]+:[[:space:]]*//' <<<"$out"
```

For each hit that is an identifier (not inside a string literal or comment) holding a record or PR revision, rename it with `"$REN" decl <file> '<ERE unique to its declaring line>' <old> <new>`: `version`→`revision`, `versions`→`revisions`, `blobVersions`→`blobRevisions`, `hasSpecVersion`→`hasSpecRevision`, `expectVersion`→`expectRevision`, the `version` parameter of the maintenance `reclaim` func field and its implementations → `revision`. Leave software-version locals alone (for example `cmd/releasepkg`, `internal/release`). Test-local helpers (`ver`, `verOf`, `miVersion`) may keep their names.

- [ ] **Step 4: gofmt, compile every tag set, fix the stragglers**

```bash
gofmt -w $(git diff --name-only -- '*.go')
go vet ./... && go vet -tags integration ./... && go vet -tags e2e ./...
```

gopls leaves misaligned composite literals (`Revision:  b.Version,`), and gofmt fixes them. Any vet failure left is a reference gopls could not see (the `e2e` file `internal/app/finalize_e2e_test.go`, or a file outside its load). Fix each by renaming the reference to the new identifier, then re-run all three vets until clean.

- [ ] **Step 5: Prove nothing observable moved**

Run: `go test -count=1 ./internal/app/ ./internal/cli/ ./internal/githubcli/ ./internal/domain/ ./internal/repository/transaction/`
Expected: PASS. JSON tags, flag names and codes are untouched, and Task 1's digest test still passes. One exception is allowed: a test that pins the untagged schema key `ExpectVersion` for `change.repair-identity`. Update that assert to `ExpectRevision` (a row-40a rename).

Run: `git diff -U0 -- '*.go' | grep -E '^[-+].*json:"' || echo "no JSON tag changed"`
Expected: `no JSON tag changed`. (Tags move in Task 3.)

- [ ] **Step 6: Commit**

```bash
git add $(git diff --name-only)
git commit -m "refactor: rename record and PR revision identifiers from version to revision (change 0472, ADR-0129 family (b))"
```
(`git diff --name-only` lists only modified tracked files; confirm with `git status --porcelain` that nothing untracked was meant to be included.)

---

### Task 3: App wire rename, JSON keys and refusal codes (rows 41-44a)

Profile hint: standard.

**Files:**
- Modify: JSON tags on every struct renamed in Task 2 (`internal/app/adr_ops.go`, `change_attach.go`, `change_claim.go` (`ChangeClaimRequest` only; NOT `claimDigestPayload`), `change_groom.go`, `change_halt.go`, `change_implemented.go`, `change_kill.go`, `change_lifecycle.go`, `change_reclaim.go`, `change_reconcile.go`, `finalize_block.go`, `finalize_context.go`, `finalize_merge.go`, `finalize_rebase.go`, `finalize_retarget.go`, `implementation_context.go`, `learning_ops.go`, `status_result.go`, `workspace_ops.go`)
- Modify: `internal/app/finding_codes.go` (constants, values, the hand-sorted `AllFindingCodes`, the comment's field list)
- Modify: reason constants: `internal/app/change_groom.go` (`reasonSpecVersionMismatch`), `change_implemented.go` (`ReasonImplementedVersionMismatch`), `workspace_ops.go` (`ReasonWorkspaceVersionMismatch`), `finalize_rebase.go` (`ReasonRebaseVersionDrift`), `finalize_retarget.go` (`ReasonRetargetVersionDrift`), `maintenance.go` (`ReasonSweepReclaimVersionMissing`)
- Modify: record/PR-revision refusal and validation messages (list in Step 3)
- Test: `internal/cli/revision_rename_test.go` (new: `TestRetiredRevisionRequestKeysRefused`)
- Test: `internal/app/schema_revision_test.go` (new: `TestSchemaRevisionKeys`, `TestSchemaVocabularyRevisionCodes`)
- Test: every existing test the grep in Step 5 finds (known: `internal/app/schema_test.go`, `internal/cli/requestkeys_test.go`, `internal/cli/change_test.go` key list, `internal/app/finalize_e2e_test.go` `cand["version"]`, `internal/app/finding_codes_test.go`, the `*_test.go` files asserting old codes, listed in Step 5)

**Interfaces:**
- Consumes: Task 2's Go names.
- Produces: the wire keys `revision`, `spec_revision`, `pr_revision`, `target.revision`, `change.revision`, `pr.revision`, and the row-44a codes. Task 5's skills and Task 7's seal rely on these spellings. Constants: `FCEmptyRevision`, `FCEmptyChangeRevision`, `FCEmptyTargetRevision`, `FCEmptySpecRevision`, `FCInvalidSpecRevision`, `FCEmptyChildPRRevision`, `reasonSpecRevisionMismatch`, `ReasonImplementedRevisionMismatch`, `ReasonWorkspaceRevisionMismatch`, `ReasonRebaseRevisionDrift`, `ReasonRetargetRevisionDrift`, `ReasonSweepReclaimRevisionMissing`.

- [ ] **Step 1: Write the failing tests**

Create `internal/app/schema_revision_test.go`:

```go
package app

import (
	"strings"
	"testing"
)

// schemaKeyPaths flattens every request/result key of the live schema into
// "<op> REQ|RES <dotted.path>" strings.
func schemaKeyPaths(t *testing.T) map[string]bool {
	t.Helper()
	doc, err := Schema(nil)
	if err != nil {
		t.Fatalf("Schema: %v", err)
	}
	out := map[string]bool{}
	var walk func(scope, prefix string, fs []FieldDescriptor)
	walk = func(scope, prefix string, fs []FieldDescriptor) {
		for _, f := range fs {
			out[scope+" "+prefix+f.Key] = true
			walk(scope, prefix+f.Key+".", f.Fields)
		}
	}
	for _, op := range doc.Operations {
		if op.Request != nil {
			walk(op.ID+" REQ", "", op.Request.Fields)
		}
		walk(op.ID+" RES", "", op.Result.Fields)
	}
	return out
}

// TestSchemaRevisionKeys is the golden schema check for ADR-0129 rows 41-44
// (change 0472): each record/PR revision key is spelled revision, and the old
// spelling is gone at the same path. The whole-schema negative is Task 7's seal.
func TestSchemaRevisionKeys(t *testing.T) {
	keys := schemaKeyPaths(t)
	for _, p := range []string{
		"adr.record REQ change.revision",
		"adr.reverse REQ target.revision", "adr.reverse REQ successor.change.revision",
		"adr.supersede REQ target.revision", "adr.supersede REQ successor.change.revision",
		"change.attach-plan REQ revision", "change.attach-results REQ revision",
		"change.block REQ revision", "change.claim REQ revision", "change.defer REQ revision",
		"change.groom REQ revision", "change.groom REQ spec_revision",
		"change.halt REQ revision", "change.kill REQ revision", "change.mark-implemented REQ revision",
		"change.reclaim REQ revision", "change.reconcile REQ revision", "change.refresh-claim REQ revision",
		"change.repair-identity REQ ExpectRevision",
		"change.resume-halted REQ revision", "change.revive REQ revision", "change.unblock REQ revision",
		"context.finalize RES candidates.revision", "context.finalize RES candidates.pr.revision",
		"context.implementation RES context.change.revision", "context.implementation RES context.spec.revision",
		"finalize.block REQ revision", "finalize.clear-block REQ revision",
		"finalize.merge REQ revision", "finalize.merge RES merge.pr_revision",
		"finalize.rebase REQ revision",
		"finalize.retarget-children REQ revision", "finalize.retarget-children REQ children.pr_revision",
		"learning.update REQ revision",
		"status RES changes.revision", "status RES records.revision",
		"workspace.inspect REQ revision", "workspace.prepare REQ revision",
	} {
		if !keys[p] {
			t.Errorf("schema lacks %s", p)
		}
	}
	// The software/format versions of spec Decision 4 are the only keys that may
	// still end in "version" (case-insensitive, so ExpectVersion is caught too).
	kept := map[string]bool{
		"capabilities RES capability_version": true,
		"capabilities RES binary.version":     true,
		"diagnostic.runtime RES go_version":   true,
		"version RES version":                 true,
	}
	for p := range keys {
		if strings.HasSuffix(strings.ToLower(p), "version") && !kept[p] {
			t.Errorf("schema still carries the retired record-revision key %s", p)
		}
	}
}

// TestSchemaVocabularyRevisionCodes: the finding_codes vocabulary lists the
// row-44a codes in their revision spelling and none of the retired ones.
func TestSchemaVocabularyRevisionCodes(t *testing.T) {
	doc, err := Schema(nil)
	if err != nil {
		t.Fatalf("Schema: %v", err)
	}
	members := map[string]bool{}
	for _, m := range doc.Vocabularies["finding_codes"].Members {
		members[m] = true
	}
	for _, c := range []string{"empty-revision", "empty-change-revision", "empty-target-revision",
		"empty-spec_revision", "invalid-spec_revision", "empty-child_pr_revision"} {
		if !members[c] {
			t.Errorf("finding_codes lacks %s", c)
		}
	}
	for _, c := range []string{"empty-version", "empty-change-version", "empty-target-version",
		"empty-spec_version", "invalid-spec_version", "empty-child_pr_version"} {
		if members[c] {
			t.Errorf("finding_codes still lists the retired %s", c)
		}
	}
}
```

Create `internal/cli/revision_rename_test.go`:

```go
package cli

import (
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// TestRetiredRevisionRequestKeysRefused pins ADR-0129 Decision 2 for family (b)
// (change 0472): a request file still carrying a retired record-revision key is
// refused by strict decoding as an unknown field, never silently dropped. A
// top-level key's refusal also lists the accepted keys, which name its
// replacement. Each body carries ONLY the retired key, so the refusal can name
// no other field.
func TestRetiredRevisionRequestKeysRefused(t *testing.T) {
	const rev = "0123456789abcdef0123456789abcdef01234567"
	dir := testsupport.TempDir(t)
	for _, c := range []struct {
		name, body, key, replacement string
		args                         []string
	}{
		{"change.block", `{"version":"x"}`, "version", "revision", []string{"change", "block", "--request", "-"}},
		{"change.defer", `{"version":"x"}`, "version", "revision", []string{"change", "defer", "--request", "-"}},
		{"change.kill", `{"version":"x"}`, "version", "revision", []string{"change", "kill", "--request", "-"}},
		{"change.revive", `{"version":"x"}`, "version", "revision", []string{"change", "revive", "--request", "-"}},
		{"change.unblock", `{"version":"x"}`, "version", "revision", []string{"change", "unblock", "--request", "-"}},
		{"change.reconcile", `{"version":"x"}`, "version", "revision", []string{"change", "reconcile", "--input", "-"}},
		{"learning.update", `{"version":"x"}`, "version", "revision", []string{"learning", "update", "--request", "-"}},
		{"change.groom version", `{"version":"x"}`, "version", "revision", []string{"change", "groom", "--request", "-"}},
		{"change.groom spec_version", `{"spec_version":"x"}`, "spec_version", "spec_revision", []string{"change", "groom", "--request", "-"}},
		{"adr.supersede target.version", `{"target":{"version":"x"}}`, "version", "", []string{"adr", "supersede", "--request", "-"}},
		{"adr.reverse target.version", `{"target":{"version":"x"}}`, "version", "", []string{"adr", "reverse", "--request", "-"}},
		{"adr.record change.version", `{"change":{"version":"x"}}`, "version", "", []string{"adr", "record", "--request", "-"}},
		{"finalize.retarget-children pr_version", `{"children":[{"pr_version":"x"}]}`, "pr_version", "",
			[]string{"finalize", "retarget-children", "--id", "1", "--version", rev, "--input", "-"}},
	} {
		args := append(append([]string{}, c.args...), "--repo-dir", dir, "--json")
		out, errS, code := runCLIStdin(t, c.body, args...)
		if code != 2 || errS != "" || !strings.Contains(out, `"result":"invalid-input"`) {
			t.Errorf("%s: want invalid-input exit 2, got code=%d err=%q out=%q", c.name, code, errS, out)
			continue
		}
		if !strings.Contains(out, `unknown field \"`+c.key+`\"`) {
			t.Errorf("%s: refusal does not name the retired key %q: %s", c.name, c.key, out)
		}
		if c.replacement != "" && !strings.Contains(out, c.replacement) {
			t.Errorf("%s: refusal's accepted keys do not name %q: %s", c.name, c.replacement, out)
		}
	}
}
```

(The retarget case passes the still-current `--version` flag; Task 4 flips it to `--revision`.)

- [ ] **Step 2: Run the new tests to verify they fail**

Run: `go test -count=1 ./internal/app/ -run 'TestSchemaRevisionKeys|TestSchemaVocabularyRevisionCodes' && go test -count=1 ./internal/cli/ -run TestRetiredRevisionRequestKeysRefused -v`
Expected: FAIL. The schema lacks the `revision` keys and codes, and the old keys decode cleanly, so exit is not 2 with `unknown field`.

- [ ] **Step 3: Implement the wire rename**

Constant identifiers move with `gopls rename`, so every reference (tests included) follows. Recreate the rename helper first (it is the same helper Task 2 used, and this task's shell does not have it):

```bash
REN=$(mktemp "${TMPDIR:-/tmp}/ren.XXXXXX")
cat > "$REN" <<'EOF'
#!/usr/bin/env bash
# ren.sh decl <file> <ERE> <Old> <New>   rename the identifier on the ONE line matching ERE
set -euo pipefail
f=$2 key=$3 old=$4 new=$5
hits=$(grep -n -E -e "$key" "$f" || true)
[ -n "$hits" ] || { echo "ren: no line matches $key in $f" >&2; exit 1; }
[ "$(printf '%s\n' "$hits" | wc -l | tr -d ' ')" = 1 ] || { echo "ren: $key matches more than one line in $f" >&2; exit 1; }
line=${hits%%:*}
col=$(awk -v n="$line" -v s="$old" 'NR==n{print index($0,s)}' "$f")
[ "${col:-0}" -gt 0 ] || { echo "ren: $old not on $f:$line" >&2; exit 1; }
GOFLAGS=-tags=integration gopls rename -w "$f:$line:$col" "$new"
EOF
chmod +x "$REN"
A=internal/app
while read -r file old new; do "$REN" decl "$A/$file" "^[[:space:]]*(const )?$old[[:space:]]" "$old" "$new"; done <<'EOF'
finding_codes.go FCEmptyChildPRVersion FCEmptyChildPRRevision
finding_codes.go FCEmptyVersion FCEmptyRevision
finding_codes.go FCEmptyChangeVersion FCEmptyChangeRevision
finding_codes.go FCEmptyTargetVersion FCEmptyTargetRevision
finding_codes.go FCEmptySpecVersion FCEmptySpecRevision
finding_codes.go FCInvalidSpecVersion FCInvalidSpecRevision
change_groom.go reasonSpecVersionMismatch reasonSpecRevisionMismatch
change_implemented.go ReasonImplementedVersionMismatch ReasonImplementedRevisionMismatch
workspace_ops.go ReasonWorkspaceVersionMismatch ReasonWorkspaceRevisionMismatch
finalize_rebase.go ReasonRebaseVersionDrift ReasonRebaseRevisionDrift
finalize_retarget.go ReasonRetargetVersionDrift ReasonRetargetRevisionDrift
maintenance.go ReasonSweepReclaimVersionMissing ReasonSweepReclaimRevisionMissing
EOF
gofmt -w internal/app/*.go
```

If a pattern matches more than one line (for example the constant is also listed bare inside `AllFindingCodes` on a line of its own), the helper stops. Narrow the ERE to the declaring line (the one carrying `FindingCode =` or `=`) and re-run that row.

JSON tags (keep each tag's options, such as `,omitempty` or `docket:"required"`):
- `json:"version"` / `json:"version,omitempty"` → `json:"revision"` / `json:"revision,omitempty"` on every Task 2 struct EXCEPT `claimDigestPayload` (kept, Decision 4), `BinaryIdentity`, `VersionResult` and `internal/release/package.go` (software).
- `json:"spec_version,omitempty"` → `json:"spec_revision,omitempty"` (`ChangeGroomRequest.SpecRevision`).
- `json:"pr_version,omitempty"` → `json:"pr_revision,omitempty"` (`VerifiedMerge.PRRevision`); `json:"pr_version"` → `json:"pr_revision"` (`AuthorizedChild.PRRevision`).

Find every tag with: `git grep -n -E 'json:"(version|spec_version|pr_version)[",]' -- 'internal/**/*.go' ':!*_test.go'`. After the edit, only `change_claim.go` (`claimDigestPayload`), `capabilities.go`, `version.go` and `internal/release/package.go` may remain.

Codes (row 44a). Rename each constant together with its value:

```go
FCEmptyChildPRRevision FindingCode = "empty-child_pr_revision"
FCEmptyRevision        FindingCode = "empty-revision"
FCEmptyChangeRevision  FindingCode = "empty-change-revision"
FCEmptyTargetRevision  FindingCode = "empty-target-revision"
FCEmptySpecRevision    FindingCode = "empty-spec_revision"
FCInvalidSpecRevision  FindingCode = "invalid-spec_revision"
```
```go
const reasonSpecRevisionMismatch = "spec-revision-mismatch"      // change_groom.go
ReasonImplementedRevisionMismatch = "revision-mismatch"          // change_implemented.go
ReasonWorkspaceRevisionMismatch   = "revision-mismatch"          // workspace_ops.go
ReasonRebaseRevisionDrift         = "revision-drift"             // finalize_rebase.go
ReasonRetargetRevisionDrift       = "revision-drift"             // finalize_retarget.go
ReasonSweepReclaimRevisionMissing = "reclaim-revision-missing"   // maintenance.go
```
The identifiers were renamed by the helper above; now edit the string values by hand. Re-sort `AllFindingCodes` so it stays strictly sorted by value (`TestFindingCodeRegistryIntegrity`); `empty-revision` moves. Update the field list in the `AllFindingCodes` doc comment (`change-version`, `target-version`, `spec_version` → `change-revision`, `target-revision`, `spec_revision`; `invalid-{…spec_version…}` → `spec_revision`).

Messages. Each names the renamed key or concept (traced at base; re-derive with the grep in Step 5):
- `"version must be the exact full blob object id of the submitted record"` → `"revision must be the exact full blob object id of the submitted record"` (`change_groom.go`, `change_lifecycle.go`, `learning_ops.go`)
- `"change.version must be …"` → `"change.revision must be …"` (`adr_ops.go`); `"target.version must be …"` → `"target.revision must be …"` (`adr_ops.go`)
- `"spec_version must be …"` / `"spec_version applies only …"` / `"… since the submitted spec_version; …"` / `"… plus spec_version and title together …"` → `spec_revision` (`change_groom.go`)
- `"the change record moved since the submitted version; …"` → `"… submitted revision; …"` (`change_implemented.go`, `finalize_rebase.go`, `workspace_ops.go`)
- `"change %04d record version moved under the authorization; …"` → `"record revision moved"`; `"child %04d PR #%d version drifted; …"` → `"PR revision drifted"`; `"children[%d].pr_version must be the exact PR version from context finalize"` → `"children[%d].pr_revision must be the exact PR revision from context finalize"` (`finalize_retarget.go`)
- `"the request was superseded by a newer record version or …"` → `"newer record revision"` (`finalize_merge.go`)
- `"reloaded record carried no blob version"` → `"reloaded record carried no record revision"` (`maintenance.go`)
- Leave `"… does not ship in this version …"` (software version) and the run-tracker `schema version` errors alone.

Do NOT touch `change_repair.go`'s `"expect-version must be …"` or the `status.go` remedy here: both name the CLI flag and move with it in Task 4.

- [ ] **Step 4: Run the new tests to verify they pass**

Run: `go test -count=1 ./internal/app/ -run 'TestSchemaRevisionKeys|TestSchemaVocabularyRevisionCodes|TestClaimDigestStableAcrossRevisionRename' && go test -count=1 ./internal/cli/ -run TestRetiredRevisionRequestKeysRefused -v`
Expected: PASS.

- [ ] **Step 5: Update the existing tests the rename reddens, derived by grep**

```bash
out=$(git grep -n -E '"(version|spec_version|pr_version)"|version-mismatch|version-drift|spec-version-mismatch|reclaim-version-missing|empty-version|empty-change-version|empty-target-version|empty-spec_version|invalid-spec_version|empty-child_pr_version|FCEmpty[A-Za-z]*Version|FCInvalidSpecVersion|Reason[A-Za-z]*Version[A-Za-z]*|reasonSpecVersionMismatch|\[\"version\"\]|pr\.version|spec_version' -- '*_test.go' || true)
printf '%s\n' "$out"
```
Flip each record/PR-revision hit to the new spelling. Known sites at base: `internal/app/schema_test.go` (the `want` key lists), `internal/cli/requestkeys_test.go` (`want` → `…, "revision", "sections", "spec_sections"`, re-sorted), `internal/cli/change_test.go` (the reconcile key list), `internal/cli/finalize_test.go` (`"forbidden"` list: `"version"` → `"revision"`, so the assert keeps testing a real key), `internal/app/finalize_e2e_test.go` (`cand["version"]` → `cand["revision"]`), `internal/app/finding_codes_test.go`, `internal/app/adr_ops_test.go`, `change_attach_test.go`, `change_groom_test.go`, `change_groom_integration_test.go`, `change_kill_test.go`, `change_lifecycle_test.go`, `change_reconcile_test.go`, `change_repair_test.go`, `finalize_retarget_test.go`, `learning_ops_test.go`, `change_integration_test.go`, `finalize_rebase_integration_test.go`, `workflow_integration_test.go`, `internal/githubcli/retarget_integration_test.go`, and every test asserting a message renamed in Step 3. Leave alone the `docket version` / `"version"` op tests (`cmd/docket/main_test.go`, `internal/cli/root_test.go`, `jsonmode_test.go`, `presenter_test.go`, `internal/app/version_test.go`, `shadow_test.go`, `capabilities_test.go`), the Task 1 digest test, and every `"--version"` CLI argument (Task 4).

- [ ] **Step 6: Compile every tag set and run the touched packages**

Run: `go vet ./... && go vet -tags integration ./... && go vet -tags e2e ./... && go test -count=1 ./internal/app/ ./internal/cli/ ./internal/githubcli/ && go test -count=1 -tags integration ./internal/app/ ./internal/githubcli/`
Expected: PASS. (The integration run is the slow one. If it exceeds the tool timeout, run it in the background and wait for it; never skip it.)

- [ ] **Step 7: Commit**

```bash
git add $(git diff --name-only) internal/app/schema_revision_test.go internal/cli/revision_rename_test.go
git commit -m "feat(app)!: rename record revision JSON keys and refusal codes to revision (change 0472, ADR-0129 rows 41-44a)"
```

---

### Task 4: CLI flags `--revision` / `--expect-revision` (rows 40, 40a)

Profile hint: standard.

**Files:**
- Modify: `internal/cli/change.go` (halt, repair-identity, resume-halted, reclaim, mark-implemented, attach builder, `changeIDRevisionSubcommand`: 6 flag definitions, their `GetString` reads and `MarkFlagRequired` calls, help text, `Short` strings "at an exact version")
- Modify: `internal/cli/finalize.go` (5 flag definitions: block, clear-block, merge, rebase, retarget-children), `internal/cli/workspace.go` (prepare; `Short` "at an exact version")
- Modify: `internal/app/status.go` (the `change repair-identity` remedy: `--expect-version` → `--expect-revision`), `internal/app/change_repair.go` (`"expect-version must be the change-record version token from the finalize report"` → `"expect-revision must be the change-record revision from the finalize report"`)
- Modify: record-sense Go doc comments in the three cli files (e.g. "(id, version)" → "(id, revision)")
- Test: `internal/cli/revision_rename_test.go` (add `TestRevisionFlagHardCut`, `TestRevisionFlagReachesRequest`; flip the retarget case's flag)
- Test: `internal/cli/capability_production_test.go` (`TestRepresentativeSignatures`: add the row-40/40a golden signatures)
- Test: `internal/cli/change_test.go`, `finalize_test.go`, `workspace_test.go`, `repodir_test.go`, `internal/app/finalize_e2e_test.go`, `internal/app/status_branch_malformed_test.go`, `internal/app/change_repair_test.go` (flag spellings and remedy/message asserts)

**Interfaces:**
- Consumes: Task 2's `changeIDRevisionSubcommand` and request `.Revision` / `.ExpectRevision` fields.
- Produces: catalog signatures (Tasks 5 and 7 rely on them):
  - `change.attach-plan` / `change.attach-results`: `--commit <sha> --id <id> --path <path> --revision <revision> [--repo-dir <dir>]`
  - `change.claim`: `--id <id> --revision <revision> [--repo-dir <dir>] [--run-context <token>]`
  - `change.halt`: `--id <id> --input <file> --revision <revision> [--repo-dir <dir>]`
  - `change.mark-implemented`: `--evidence <file> --head <ref> --id <id> --pr <ref> --revision <revision> [--repo-dir <dir>]`
  - `change.reclaim` / `change.refresh-claim`: `--id <id> --revision <revision> [--repo-dir <dir>]`
  - `change.repair-identity`: `--expect-revision <revision> --id <id> [--adopt-pr <ref>] [--adopt-pr-head] [--expect-branch <name>] [--expect-head <ref>] [--expect-pr <n>] [--repo-dir <dir>]`
  - `change.resume-halted`: `--id <id> --revision <revision> [--acknowledge-quiescent] [--repo-dir <dir>]`
  - `finalize.block`: `--attempt <token> --head <ref> --id <id> --input <file> --pr-number <n> --reason <token> --revision <revision> [--repo-dir <dir>]`
  - `finalize.clear-block`: `--head <ref> --id <id> --pr-number <n> --revision <revision> [--repo-dir <dir>]`
  - `finalize.merge`: `--head <ref> --id <id> --revision <revision> [--admin] [--repo-dir <dir>]`
  - `finalize.rebase`: `--head <ref> --id <id> --revision <revision> [--repo-dir <dir>]`
  - `finalize.retarget-children`: `--id <id> --input <file> --revision <revision> [--repo-dir <dir>]`
  - `workspace.prepare`: `--id <id> --revision <revision> [--repo-dir <dir>]`

- [ ] **Step 1: Write the failing tests**

Append to `internal/cli/revision_rename_test.go` (add `"os"`, `"path/filepath"` and `"github.com/spf13/cobra"` to its imports):

```go
// revisionFlagOps is ADR-0129 row 40's operation set: exactly these commands pin
// the record revision on a flag.
var revisionFlagOps = [][]string{
	{"change", "attach-plan"}, {"change", "attach-results"}, {"change", "claim"}, {"change", "halt"},
	{"change", "mark-implemented"}, {"change", "reclaim"}, {"change", "refresh-claim"}, {"change", "resume-halted"},
	{"finalize", "block"}, {"finalize", "clear-block"}, {"finalize", "merge"}, {"finalize", "rebase"},
	{"finalize", "retarget-children"}, {"workspace", "prepare"},
}

func flagRequired(f *pflag.Flag) bool {
	v := f.Annotations[cobra.BashCompOneRequiredFlag]
	return len(v) == 1 && v[0] == "true"
}

// TestRevisionFlagHardCut pins ADR-0129 Decision 2 for rows 40/40a (change
// 0472). Over the WHOLE change/finalize/workspace subtree (derived from the live
// cobra tree, not listed): no command registers --version or --expect-version,
// and the commands registering a required --revision are exactly row 40's set
// (checked in both directions). Every old spelling is refused as an unknown flag.
func TestRevisionFlagHardCut(t *testing.T) {
	root := captureTree(t)
	want := map[string]bool{}
	for _, p := range revisionFlagOps {
		want[strings.Join(p, " ")] = true
	}
	got := map[string]bool{}
	var walk func(prefix string, c *cobra.Command)
	walk = func(prefix string, c *cobra.Command) {
		key := strings.TrimSpace(prefix + " " + c.Name())
		for _, gone := range []string{"version", "expect-version"} {
			if c.Flags().Lookup(gone) != nil {
				t.Errorf("%s still registers the retired --%s (ADR-0129 rows 40/40a)", key, gone)
			}
		}
		if f := c.Flags().Lookup("revision"); f != nil {
			got[key] = true
			if !flagRequired(f) {
				t.Errorf("%s: --revision is not required", key)
			}
		}
		for _, sub := range c.Commands() {
			walk(key, sub)
		}
	}
	for _, group := range []string{"change", "finalize", "workspace"} {
		g, _, err := root.Find([]string{group})
		if err != nil {
			t.Fatalf("find %s: %v", group, err)
		}
		walk("", g)
	}
	for k := range want {
		if !got[k] {
			t.Errorf("%s lacks a required --revision", k)
		}
	}
	for k := range got {
		if !want[k] {
			t.Errorf("%s registers --revision but is not a row-40 operation", k)
		}
	}
	rep, _, err := root.Find([]string{"change", "repair-identity"})
	if err != nil {
		t.Fatal(err)
	}
	if f := rep.Flags().Lookup("expect-revision"); f == nil || !flagRequired(f) {
		t.Errorf("change repair-identity lacks a required --expect-revision")
	}
	for _, p := range append(append([][]string{}, revisionFlagOps...), []string{"change", "repair-identity"}) {
		flag := "--version"
		if p[1] == "repair-identity" {
			flag = "--expect-version"
		}
		args := append(append([]string{}, p...), flag, "x")
		_, errS, code := runCLI(t, args...)
		if code != 2 || !strings.Contains(errS, "unknown flag: "+flag) {
			t.Errorf("%v %s: want exit 2 naming the unknown flag, got code=%d err=%q", p, flag, code, errS)
		}
	}
}

// TestRevisionFlagReachesRequest (change 0472): the value passed as --revision
// reaches the request. A read left on the retired "version" name returns ""
// silently and the op refuses empty-revision. Each op first gets an explicit
// empty --revision as a control, which must reach its shape validator, so the
// non-empty assert below it cannot pass vacuously. (workspace.prepare discovers the
// repository before validating shape, so it is covered by the hard cut and by
// Task 7's Go seal, not here.)
func TestRevisionFlagReachesRequest(t *testing.T) {
	const rev = "0123456789abcdef0123456789abcdef01234567"
	head := strings.Repeat("a", 40)
	dir := testsupport.TempDir(t)
	ev := filepath.Join(dir, "evidence.json")
	if err := os.WriteFile(ev, []byte(`{"schema":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		stdin string
		args  []string
	}{
		{"", []string{"change", "claim", "--id", "1"}},
		{"", []string{"change", "refresh-claim", "--id", "1"}},
		{"", []string{"change", "reclaim", "--id", "1"}},
		{"", []string{"change", "resume-halted", "--id", "1"}},
		{"", []string{"change", "attach-plan", "--id", "1", "--path", "p.md", "--commit", head}},
		{"", []string{"change", "attach-results", "--id", "1", "--path", "r.md", "--commit", head}},
		{"", []string{"change", "mark-implemented", "--id", "1", "--head", head, "--pr", "https://github.com/o/r/pull/1", "--evidence", ev}},
		{"{}", []string{"change", "halt", "--id", "1", "--input", "-"}},
		{"{}", []string{"finalize", "block", "--id", "1", "--pr-number", "1", "--attempt", "x", "--reason", "repair-needs-signoff", "--head", head, "--input", "-"}},
		{"", []string{"finalize", "clear-block", "--id", "1", "--head", head, "--pr-number", "1"}},
		{"", []string{"finalize", "merge", "--id", "1", "--head", head}},
		{"", []string{"finalize", "rebase", "--id", "1", "--head", head}},
		{`{"children":[]}`, []string{"finalize", "retarget-children", "--id", "1", "--input", "-"}},
	} {
		name := c.args[0] + " " + c.args[1]
		run := func(value string) string {
			args := append(append([]string{}, c.args...), "--revision", value, "--repo-dir", dir, "--json")
			out, _, _ := runCLIStdin(t, c.stdin, args...)
			return out
		}
		if out := run(""); !strings.Contains(out, `"code":"empty-revision"`) {
			t.Fatalf("%s: control: an empty --revision did not reach the shape validator: %s", name, out)
		}
		if out := run(rev); strings.Contains(out, `"code":"empty-revision"`) {
			t.Errorf("%s: --revision %s never reached the request: %s", name, rev, out)
		}
	}
	// repair-identity validates its own request shape before any read.
	repair := func(value string) string {
		out, _, _ := runCLI(t, "change", "repair-identity", "--id", "1", "--expect-revision", value,
			"--adopt-pr-head", "--expect-pr", "1", "--expect-head", "b", "--repo-dir", dir, "--json")
		return out
	}
	if out := repair(""); !strings.Contains(out, "expect-revision must be") {
		t.Fatalf("repair-identity control: an empty --expect-revision did not reach validation: %s", out)
	}
	if out := repair(rev); strings.Contains(out, "expect-revision must be") {
		t.Errorf("repair-identity: --expect-revision %s never reached the request: %s", rev, out)
	}
}
```
(`pflag` is `github.com/spf13/pflag`; add the import. It is already a module dependency through cobra.)

In `TestRetiredRevisionRequestKeysRefused`, change the retarget case's `"--version", rev` to `"--revision", rev`.

In `internal/cli/capability_production_test.go` `TestRepresentativeSignatures`, add these entries to the id→signature map:

```go
		"change.claim":           "--id <id> --revision <revision> [--repo-dir <dir>] [--run-context <token>]",
		"change.repair-identity": "--expect-revision <revision> --id <id> [--adopt-pr <ref>] [--adopt-pr-head] [--expect-branch <name>] [--expect-head <ref>] [--expect-pr <n>] [--repo-dir <dir>]",
		"finalize.block":         "--attempt <token> --head <ref> --id <id> --input <file> --pr-number <n> --reason <token> --revision <revision> [--repo-dir <dir>]",
		"finalize.merge":         "--head <ref> --id <id> --revision <revision> [--admin] [--repo-dir <dir>]",
		"workspace.prepare":      "--id <id> --revision <revision> [--repo-dir <dir>]",
```
and after the map loop add a catalog-wide negative:

```go
	// change 0472: no catalog operation's signature carries the retired
	// record-revision flags (the `version` op itself takes no flags).
	for _, e := range entries {
		if strings.Contains(e.Signature, "--version") || strings.Contains(e.Signature, "--expect-version") {
			t.Errorf("%s signature still carries a retired flag: %s", e.ID, e.Signature)
		}
	}
```
(Use the loop variable and field names the test already uses for the catalog entries. If the test indexes a map, iterate over its source slice.)

- [ ] **Step 2: Run the new tests to verify they fail**

Run: `go test -count=1 ./internal/cli/ -run 'TestRevisionFlagHardCut|TestRevisionFlagReachesRequest|TestRepresentativeSignatures|TestRetiredRevisionRequestKeysRefused'`
Expected: FAIL (`--version` still registered; `--revision` unknown).

- [ ] **Step 3: Rename the flags**

In `internal/cli/change.go`, `finalize.go` and `workspace.go`, for each of the 12 definition sites (all derived by `git grep -n -E '"(version|expect-version)"' -- internal/cli/change.go internal/cli/finalize.go internal/cli/workspace.go`):
- `Flags().String("version", "", "exact record blob object `id` from the authoritative context read (required)")` → `Flags().String("revision", "", "exact record `revision` (the blob object id) from the authoritative context read (required)")`. Keep each site's own wording: "exact parent record …" on retarget-children, "… from the claim receipt (required)" on workspace prepare.
- `GetString("version")` → `GetString("revision")`, `MarkFlagRequired("version")` → `MarkFlagRequired("revision")`.
- repair-identity: `"expect-version"` → `"expect-revision"` in all three calls; help → "exact change-record `revision` from the finalize report (required)".
- `Short` strings: "at an exact version" → "at an exact revision" (claim, refresh-claim, repair-identity, workspace prepare).
- Doc comments in these files that name the record pin ("(id, version)", "pinned entity version", "exact-version transaction") → revision.

In `internal/app/status.go`, the remedy format string: `--expect-version %s` → `--expect-revision %s`. In `internal/app/change_repair.go`, the invalid-request message → `"expect-revision must be the change-record revision from the finalize report"`, and its record-sense comments.

- [ ] **Step 4: Flip the existing tests, derived by grep**

```bash
out=$(git grep -n -E -e '"--version"|"--expect-version"|"expect-version"|--expect-version|Lookup\("version"\)|"version", "(head|input|path|repo-dir|acknowledge-quiescent)' -- '*_test.go' ':!cmd/releasepkg/*' ':!internal/release/*' || true)
printf '%s\n' "$out"
```
Flip every hit on a row-40/40a command to `--revision` / `--expect-revision` / `"revision"` / `"expect-revision"`. Known: `internal/cli/change_test.go` (required-flag lists, `Lookup("version")`, `--version` args, the repair-identity flag list and required test), `internal/cli/finalize_test.go`, `internal/cli/workspace_test.go` (flag list; the missing-flag assert `strings.Contains(errS, "version")` → `"revision"`), `internal/cli/repodir_test.go`, `internal/app/finalize_e2e_test.go` (every `"--version"` argument), `internal/app/status_branch_malformed_test.go` (`"--expect-version blobchange0001"` → `"--expect-revision blobchange0001"`), `internal/app/change_repair_test.go` (message asserts). Leave `cmd/releasepkg/main_test.go` and the release tests alone.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go vet ./... && go vet -tags integration ./... && go vet -tags e2e ./... && go test -count=1 ./internal/cli/ ./internal/app/`
Expected: PASS.

Run the e2e matrix the way `tests/test_go_finalize_e2e.sh` does: `go test -tags e2e -run TestE2E -count=1 -v ./internal/app/`
Expected: PASS, with a `--- PASS: TestE2E…` line for each of the file's `TestE2E*` functions (a vacuous filter prints none).

- [ ] **Step 6: Mutation tests**

```bash
# (a) a stale read: the flag is registered as revision but read under the old name
f=internal/cli/finalize.go
cp "$f" "$f.bak"
awk 'BEGIN{d=0} /GetString\("revision"\)/ && !d {sub(/GetString\("revision"\)/,"GetString(\"version\")"); d=1} {print}' "$f.bak" > "$f"
grep -c 'GetString("version")' "$f"   # must print 1
go test -count=1 ./internal/cli/ -run TestRevisionFlagReachesRequest
mv -f "$f.bak" "$f"
# (b) an alias: re-register the old flag beside the new one on workspace prepare
f=internal/cli/workspace.go
cp "$f" "$f.bak"
perl -0pi -e 's/(\t_ = prepare\.MarkFlagRequired\("revision"\))/\tprepare.Flags().String("version", "", "alias")\n$1/' "$f"
grep -c '"alias"' "$f"   # must print 1
go test -count=1 ./internal/cli/ -run TestRevisionFlagHardCut
mv -f "$f.bak" "$f"
go test -count=1 ./internal/cli/ -run 'TestRevisionFlagHardCut|TestRevisionFlagReachesRequest'
```
Expected: (a) FAILs naming the finalize op whose read was mutated; (b) FAILs with `workspace prepare still registers the retired --version` and the unknown-flag assert for workspace prepare; the restored run PASSes.

- [ ] **Step 7: Commit**

```bash
git add $(git diff --name-only)
git commit -m "feat(cli)!: rename --version to --revision and --expect-version to --expect-revision (change 0472, ADR-0129 rows 40, 40a)"
```

---

### Task 5: Skills and their embedded copies

Profile hint: standard.

**Files:**
- Modify (known at base; the grep in Step 1 is authoritative): `skills/docket-adr/SKILL.md`, `skills/docket-auto-groom/SKILL.md`, `skills/docket-convention/references/terminal-close-out.md`, `skills/docket-finalize-change/SKILL.md`, `skills/docket-finalize-change/references/gate-failure.md`, `skills/docket-groom-next/SKILL.md`, `skills/docket-implement-next/SKILL.md`, `skills/docket-implement-next/references/edge-paths.md`, `skills/docket-new-change/SKILL.md`
- Regenerate: `internal/assets/embedded/**` (`go generate ./internal/assets/`)
- Check: `agents/**`, `cursor-rules/**`, `AGENTS.md` (no record-revision spelling at base; `AGENTS.md`'s "`version` operation" is the kept software sense)

**Interfaces:**
- Consumes: Task 3's keys and codes, Task 4's flags and signatures.
- Produces: skill text with no retired record-revision spelling, which Task 7's seal scans.

- [ ] **Step 1: Derive the sites**

```bash
out=$(git grep -n -i -E -e 'version' -- skills/ agents/ cursor-rules/ AGENTS.md || true)
printf '%s\n' "$out" | grep -v -i -E 'protocol_version|capability_version|schema_version|docket version|`version` operation|version operation' 
```
Classify each hit. **Rename** every record- or PR-revision sense: `--version <…>` → `--revision <revision>`; `<entity-version>` → `<revision>`; `--expect-version V` → `--expect-revision V`; `` `version` `` as a request/record key → `` `revision` ``; `{id, path, version}` → `{id, path, revision}`; `spec_version` → `spec_revision`; `"version": "<entity-version>"` → `"revision": "<revision>"`; codes (`version-mismatch` → `revision-mismatch`, `empty-spec_version` → `empty-spec_revision`, …); prose "entity version" / "exact version" / "record version" / "version drift" / "version mismatch" / "exact-version transaction" → "record revision" / "exact revision" / "revision drift" / "revision mismatch" / "exact-revision transaction". **Keep** software/harness versions ("earlier docket versions", "tool version", "version-scoped" harness verdicts, "the version operation of the binary", "versioned `ResolverReport`", `protocol_version`).

- [ ] **Step 2: Fix the two stale shapes the rename surfaces**

- `skills/docket-implement-next/SKILL.md` (Step 3, the reconcile write): it says "the `change.reconcile` operation with `--id <id> --version <entity-version> --input <request-file>`", but `change.reconcile`'s signature is `--input <file> [--repo-dir <dir>]`. The id and the revision travel in the request file. Rewrite it as "the `change.reconcile` operation with `--input <request-file>` (the request carries `id` and the pinned `revision`)". Keep the rest of the sentence.
- `skills/docket-finalize-change/SKILL.md` (attended retarget): "`{ID, PRNumber, PRVersion}` per child" names Go fields, not the wire keys. Rewrite it as "`{id, pr_number, pr_revision}` per child".

- [ ] **Step 3: Verify every rewritten flag against the live catalog**

```bash
caps=$(go run ./cmd/docket capabilities --json)
lines=$(git grep -h -E -e '--(expect-)?revision' -- skills/ || true)
# For each line carrying the flag, the nearest op id before the (last) flag.
ids=$(printf '%s\n' "$lines" | perl -ne 'my $last; while (/((?:change|finalize|workspace)\.[a-z-]+)(?=.*?--(?:expect-)?revision)/g) { $last = $1 } print "$last\n" if defined $last' | sort -u)
printf '%s\n' "$ids" | while read -r id; do
  [ -n "$id" ] || continue
  sig=$(jq -r --arg id "$id" '.commands[] | select(.id==$id) | .signature' <<<"$caps")
  case "$sig" in *--revision*|*--expect-revision*) echo "ok   $id";; *) echo "MISMATCH $id: $sig";; esac
done
```
Expected: every line `ok`. A `MISMATCH` means the skill puts a flag on an op that takes the revision in its request file; fix the skill text. (A flag wrapped onto the line after its op id is outside this per-line check. Read those sites, which Step 1's grep lists, by eye.)

- [ ] **Step 4: Regenerate the embedded copies and run the skill guards**

Run: `go generate ./internal/assets/ && go test -count=1 ./internal/assets/ ./internal/repoguard/ ./internal/harness/...`
Expected: PASS, including `TestEmbeddedMatchesAuthored` and the skill word/line budgets in `internal/repoguard/budgets_test.go`. A like-for-like word swap keeps counts; if a budget row trips, re-read the edit before touching the budget.

- [ ] **Step 5: Commit**

```bash
git add $(git diff --name-only -- skills/ agents/ cursor-rules/ AGENTS.md) $(git diff --name-only -- internal/assets/embedded/) $(git ls-files --others --exclude-standard -- internal/assets/embedded/)
git commit -m "docs(skills): speak revision, not version, for the record pin (change 0472)"
```

---

### Task 6: Maintained docs and the glossary's single Revision entry

Profile hint: economy.

**Files:**
- Modify: `docs/reference/glossary.md` (merge "Change version (`--version`)" and "Entity version" into one "Revision (`--revision`)" entry; every record-revision example; the alphabetical index)
- Modify: `docs/guide/keeping-the-backlog-honest.md` (the `docket change reclaim --id <n> --version <v>` example and "exact recorded version")
- Check, no edit expected: `docs/reference/harness/validation.md`, `validation-runbook.md`, `docs/reference/harness/fixtures/nested-launch/*` (at base every hit is `codex --version` / `cursor-agent --version`, a foreign CLI's software version, which stays), `docs/reference/cli.md` (`docket version`, kept), `docs/install/**`, `docs/release/**` (software versions)

**Interfaces:**
- Consumes: Tasks 3-4 spellings.
- Produces: the glossary anchor `#revision---revision` (heading `### Revision (\`--revision\`)`).

- [ ] **Step 1: Derive the sites**

```bash
out=$(git grep -n -i -E -e 'version' -- docs/ ':!docs/changes' ':!docs/results' ':!docs/superpowers' ':!docs/adrs' || true)
printf '%s\n' "$out" | grep -i -E -e '--version|`version`|\.version|entity version|change version|record version|exact version|spec_version|pr_version|version drift|expect-version|#change-version|#entity-version'
```
At base, the glossary alone carries about 40 record-revision sites (for example the `workspace prepare`, `adr supersede` `{id,path,version}`, `learning update`, `attach-plan`, `attach-results`, `defer`/`kill`, `claim`/`refresh-claim`/`reclaim`, `halt`/`resume-halted`, `groom` (`version`, `spec_version`), `mark-implemented`, `finalize rebase`/`merge`/`block`/`clear-block`/`retarget-children`, `repair-identity --expect-version`, and CAS sections).

- [ ] **Step 2: Replace the two entries with one**

Replace the whole `### Change version (\`--version\`)` section with:

````markdown
### Revision (`--revision`)

The exact id of a pinned state. Docket uses the word in one sense only:

- **Record revision:** the git blob object id of a record a typed operation can mutate: a change,
  an ADR, a learning finding, or a linked spec. Spelled `--revision` on a flag, `revision` in a
  request or read (`docket status --json` carries one per change), `spec_revision` for a groom's
  linked spec, `target.revision` for the ADR a supersede/reverse flips, and `change.revision` for
  an ADR's producing change.
- **PR revision:** a hash over a pull request's mutable snapshot, spelled `pr.revision` in
  `context finalize` and `pr_revision` in `finalize merge` / `finalize retarget-children`.
- **Commit revision:** the commit ids a mutation reports, spelled `committed_revision`,
  `metadata_revision` and `*_branch_revision`.

**Used for:** compare-and-swap. A mutating operation takes the record revision you read and refuses
if the record moved under you, so two sessions can never silently overwrite each other.

```sh
docket status --json | jq -r '.changes[] | select(.id==412) | .revision'
docket change claim --id 412 --revision <that-revision>
```
````

Delete the `### Entity version` section entirely. In the index, remove `- [Change version (--version)](#change-version---version)` and `- [Entity version](#entity-version)`. Add `- [Revision (--revision)](#revision---revision)` in alphabetical order (after `Review rung`), plus the lookup aliases `- [Change version / entity version](#revision---revision) — see Revision` where "Change version" would sort, matching the file's existing `— see …` alias style.

- [ ] **Step 3: Rewrite the remaining examples**

Every other hit from Step 1 in the record sense: `--version <v>` → `--revision <v>`, `--version <version>` → `--revision <revision>`, `{id,path,version}` → `{id,path,revision}`, `{path, version, …}` → `{path, revision, …}`, `"version": "<v>"` → `"revision": "<v>"`, `spec_version` → `spec_revision`, `--expect-version` → `--expect-revision`, `.version` in jq → `.revision`, "entity version" → "record revision", "version drift" → "revision drift", "fresh version" → "fresh revision". In the CAS section, "matches the entity version you read" → "matches the record revision you read" and "re-read the path and version" → "re-read the path and revision". Fix `keeping-the-backlog-honest.md` the same way.

- [ ] **Step 4: Verify anchors and residue**

```bash
out=$(git grep -n -E -e '#change-version|#entity-version' -- . ':!docs/changes' ':!docs/results' ':!docs/superpowers' ':!docs/adrs' || true); [ -z "$out" ] && echo "no stale anchors" || printf '%s\n' "$out"
out=$(git grep -n -E -e '--version <|--expect-version|spec_version|pr_version|entity version|\.version\b' -- docs/reference/glossary.md docs/guide/ || true); [ -z "$out" ] && echo "glossary clean" || printf '%s\n' "$out"
go test -count=1 ./internal/repoguard/
```
Expected: `no stale anchors`, `glossary clean`, PASS.

- [ ] **Step 5: Commit**

```bash
git add docs/reference/glossary.md docs/guide/keeping-the-backlog-honest.md
git commit -m "docs: define revision once in the glossary and move record-revision examples (change 0472)"
```

---

### Task 7: Seal the retired spellings (ADR-0129 Decision 10)

Profile hint: premium.

**Files:**
- Modify: `internal/repoguard/retired_vocabulary_test.go` (header scope note; two new kinds; 15 rows; bound-flag text and Go scans; schema walk; table floor; non-vacuity, negative controls, population floors)

**Interfaces:**
- Consumes: `cli.Run(args []string, stdin io.Reader, stdout, stderr io.Writer, info buildinfo.Info, facts buildinfo.RuntimeFacts) int` (`internal/cli/root.go`); `app.Schema(effects []string) (app.SchemaResult, error)`, `app.FieldDescriptor{Key string; Fields []FieldDescriptor}`, `app.OperationSchema{ID string; Request *TypeDescriptor; Result TypeDescriptor}`, `app.SchemaResult{EnvelopeShape TypeDescriptor; Operations []OperationSchema}`; existing `tokenRe`, `scanTextLine`, `scanGoLiteral`, `stripHashComment`, `isMarkdownSurface`, `retiredHit`.
- Produces: `kindBoundFlag`, `kindSchemaKey`, `catalogOpRefs() (*regexp.Regexp, int, error)`, `scanBoundFlags(rel string, lines []string) []retiredHit`, `scanGoBoundFlag(rel string, lineNo int, lit string) []retiredHit`, `schemaVersionKeyHits(doc app.SchemaResult) (hits []string, revisionKeys int, seen map[string]bool)`, `schemaKeptVersionKeys`.

- [ ] **Step 1: Add the kinds, rows and floor**

After `kindGoPrefix` in the `retiredKind` const block:

```go
	// kindBoundFlag: the bare flag Old (row 40's --version), retired only where
	// it is BOUND to a change, finalize or workspace command (see scanBoundFlags
	// for markdown/shell and scanGoBoundFlag for Go). Unbound, it names a
	// software version and stays: docket version, the release tools, foreign
	// CLIs.
	kindBoundFlag
	// kindSchemaKey: a request/result key whose name ends in Old, found by
	// walking the schema registry (the wire contract), never by grepping tags
	// (see schemaVersionKeyHits). The exact kept set is schemaKeptVersionKeys.
	kindSchemaKey
```

Append to `retiredVocabulary`:

```go
	// Family (b) — revision (change 0472): rows 40, 40a, 41-44, 44a. Row 39 is the
	// concept itself; row 45 (resolver_budget_version) is kept.
	{Row: "40", Kind: kindBoundFlag, Old: "--version", New: "--revision"},
	{Row: "40a", Kind: kindToken, Old: "expect-version", New: "--expect-revision / expect-revision"},
	{Row: "41-44", Kind: kindSchemaKey, Old: "version", New: "revision"},
	{Row: "42", Kind: kindToken, Old: "spec_version", New: "spec_revision"},
	{Row: "44", Kind: kindToken, Old: "pr_version", New: "pr_revision"},
	{Row: "44a", Kind: kindToken, Old: "version-mismatch", New: "revision-mismatch"},
	{Row: "44a", Kind: kindToken, Old: "version-drift", New: "revision-drift"},
	{Row: "44a", Kind: kindToken, Old: "spec-version-mismatch", New: "spec-revision-mismatch"},
	{Row: "44a", Kind: kindToken, Old: "reclaim-version-missing", New: "reclaim-revision-missing"},
	{Row: "44a", Kind: kindToken, Old: "empty-version", New: "empty-revision"},
	{Row: "44a", Kind: kindToken, Old: "empty-change-version", New: "empty-change-revision"},
	{Row: "44a", Kind: kindToken, Old: "empty-target-version", New: "empty-target-revision"},
	{Row: "44a", Kind: kindToken, Old: "empty-spec_version", New: "empty-spec_revision"},
	{Row: "44a", Kind: kindToken, Old: "invalid-spec_version", New: "invalid-spec_revision"},
	{Row: "44a", Kind: kindToken, Old: "empty-child_pr_version", New: "empty-child_pr_revision"},
```

In `testRetiredTableIntegrity`, raise `const floor = 60` to `const floor = 78` (63 rows at base + 15).

Update the file header: family (b)'s rows are appended; the bound-flag and schema-walk kinds are the first kinds whose match depends on context (the binding) or on the wire contract (the registry); and add the LIMITATION paragraph from `scanBoundFlags`'s doc comment below.

- [ ] **Step 2: Write the bound-flag scans**

Add imports `bytes`, `encoding/json`, `sort`, `sync`, `github.com/danielhanold/docket/internal/app`, `github.com/danielhanold/docket/internal/buildinfo`, `github.com/danielhanold/docket/internal/cli`. Then:

```go
// boundFlagFamilies are the command families whose bare --version pinned the
// record revision (ADR-0129 row 40).
var boundFlagFamilies = map[string]bool{"change": true, "finalize": true, "workspace": true}

var (
	opRefOnce sync.Once
	opRefRe   *regexp.Regexp
	opRefIDs  int
	opRefErr  error
)

// catalogOpRefs compiles the operation-reference matcher from the live
// capability catalog, read in-process through cli.Run, never hand-listed
// (enumerated-floor): every dotted op id (change.claim) and every multi-word
// command form (change claim, gate drive start). Single-word commands (status,
// version, schema) are deliberately not references: they name no family a
// --version binds to, and as English words they would shadow a real binding.
func catalogOpRefs() (*regexp.Regexp, int, error) {
	opRefOnce.Do(func() {
		var out, errb bytes.Buffer
		if code := cli.Run([]string{"capabilities", "--json"}, strings.NewReader(""), &out, &errb, buildinfo.Info{}, buildinfo.RuntimeFacts{}); code != 0 {
			opRefErr = fmt.Errorf("capabilities exited %d: %s", code, errb.String())
			return
		}
		var doc struct {
			Commands []struct {
				ID   string   `json:"id"`
				Argv []string `json:"argv"`
			} `json:"commands"`
		}
		if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
			opRefErr = fmt.Errorf("parse capabilities: %w", err)
			return
		}
		var alts []string
		for _, c := range doc.Commands {
			if strings.Contains(c.ID, ".") {
				alts = append(alts, regexp.QuoteMeta(c.ID))
				opRefIDs++
			}
			if len(c.Argv) >= 3 {
				words := make([]string, 0, len(c.Argv)-1)
				for _, w := range c.Argv[1:] {
					words = append(words, regexp.QuoteMeta(w))
				}
				alts = append(alts, strings.Join(words, `[ \t]+`))
			}
		}
		// Longest first: Go's alternation is leftmost-first, so change.refresh-claim
		// must be tried before any shorter reference it contains.
		sort.Slice(alts, func(i, j int) bool { return len(alts[i]) > len(alts[j]) })
		opRefRe = regexp.MustCompile(`(?:^|[^A-Za-z0-9_.-])(` + strings.Join(alts, "|") + `)(?:[^A-Za-z0-9_-]|$)`)
	})
	return opRefRe, opRefIDs, opRefErr
}

// refFamily returns an operation reference's family: its text up to the first
// '.' or blank.
func refFamily(ref string) string {
	if i := strings.IndexAny(ref, ". \t"); i >= 0 {
		return ref[:i]
	}
	return ref
}

// scanBoundFlags reports each kindBoundFlag row's flag that is BOUND to a
// change, finalize or workspace command. A block is a maximal run of non-blank
// lines, so a flag wrapped onto the line after its operation still binds, and a
// blank line ends the binding. The flag's binding is the NEAREST catalog
// operation reference that precedes it in its block, so `docket version`, the
// release tools' --version and a foreign CLI's (`codex --version`) never bind.
//
// LIMITATION (byte-pattern-guard-matches-a-spelling): a foreign tool's
// --version whose nearest preceding reference in the same block is a change,
// finalize or workspace operation reads as bound. No maintained surface carries
// that shape; the negative controls pin the shapes that exist.
func scanBoundFlags(rel string, lines []string) []retiredHit {
	re, _, err := catalogOpRefs()
	if err != nil {
		panic(fmt.Sprintf("retired-vocabulary seal: catalog operation references unavailable (fail closed): %v", err))
	}
	var hits []retiredHit
	for start := 0; start < len(lines); {
		if strings.TrimSpace(lines[start]) == "" {
			start++
			continue
		}
		end := start
		for end < len(lines) && strings.TrimSpace(lines[end]) != "" {
			end++
		}
		block := strings.Join(lines[start:end], "\n")
		refs := re.FindAllStringSubmatchIndex(block, -1)
		for _, r := range retiredVocabulary {
			if r.Kind != kindBoundFlag {
				continue
			}
			for _, m := range tokenRe(r.Old).FindAllStringIndex(block, -1) {
				pos := m[0] + strings.Index(block[m[0]:m[1]], r.Old)
				family := ""
				for _, ref := range refs {
					if ref[2] >= pos {
						break
					}
					family = refFamily(block[ref[2]:ref[3]])
				}
				if boundFlagFamilies[family] {
					lineNo := start + 1 + strings.Count(block[:pos], "\n")
					hits = append(hits, retiredHit{rel, lineNo, r, strings.TrimSpace(lines[lineNo-1])})
				}
			}
		}
		start = end
	}
	return hits
}

// scanGoBoundFlag: a kindBoundFlag row's flag name (Old without its leading
// "--") passed as the FIRST argument of a method call in non-test internal/cli
// source, which is a flag definition or lookup (Flags().String("version", …),
// GetString("version"), MarkFlagRequired("version")). internal/cli builds every
// change, finalize and workspace command, and none of its commands takes a
// software --version (docket version is a subcommand with no such flag); the
// release tools' --version lives outside it (cmd/releasepkg, the shell
// downloader). A plain function call (capability("version", …)) and a struct
// field (Use: "version") are not method calls and never bind.
func scanGoBoundFlag(rel string, lineNo int, lit string) []retiredHit {
	val, err := strconv.Unquote(lit)
	if err != nil {
		return nil
	}
	var hits []retiredHit
	for _, r := range retiredVocabulary {
		if r.Kind == kindBoundFlag && val == strings.TrimPrefix(r.Old, "--") {
			hits = append(hits, retiredHit{rel, lineNo, r, lit})
		}
	}
	return hits
}
```

Wire them in:
- `scanText`: store the comment-stripped line back into `lines[i]` inside the loop, then `return append(hits, scanBoundFlags(rel, lines)...)`.
- `scanGoSource`: track the three previous tokens. When `tok == token.STRING && prev1 == token.LPAREN && prev2 == token.IDENT && prev3 == token.PERIOD && strings.HasPrefix(rel, "internal/cli/")`, also append `scanGoBoundFlag(rel, fset.Position(pos).Line, lit)`. Shift at the end of every iteration: `prev3, prev2, prev1 = prev2, prev1, tok`.

- [ ] **Step 3: Write the schema walk**

```go
// schemaKeptVersionKeys is the bounded kept set of spec Decision 4 as it appears
// in the schema: software and format versions, the version operation's own key,
// and capabilities' binary.version. Each entry is an exact "<scope> <path>", never
// a bare name, so a record revision cannot hide behind a kept spelling elsewhere.
// The claim digest payload is not a registered shape and is outside the walk.
var schemaKeptVersionKeys = map[string]bool{
	"envelope protocol_version":           true,
	"capabilities RES capability_version": true,
	"capabilities RES binary.version":     true,
	"diagnostic.runtime RES go_version":   true,
	"version RES version":                 true,
}

// schemaVersionKeyHits walks every key of doc (the envelope, then each
// operation's request and result, recursively) and reports each key whose name
// ends, case-insensitively, in a kindSchemaKey row's Old and is not in
// schemaKeptVersionKeys, naming the replacement path. revisionKeys counts the keys
// ending in "revision", the live population floor a truncated walk would miss.
func schemaVersionKeyHits(doc app.SchemaResult) (hits []string, revisionKeys int, seen map[string]bool) {
	seen = map[string]bool{}
	var walk func(scope, prefix string, fs []app.FieldDescriptor)
	walk = func(scope, prefix string, fs []app.FieldDescriptor) {
		for _, f := range fs {
			path := prefix + f.Key
			key := scope + " " + path
			seen[key] = true
			lower := strings.ToLower(f.Key)
			if strings.HasSuffix(lower, "revision") {
				revisionKeys++
			}
			for _, r := range retiredVocabulary {
				if r.Kind != kindSchemaKey || !strings.HasSuffix(lower, r.Old) || schemaKeptVersionKeys[key] {
					continue
				}
				n := len(f.Key) - len(r.Old)
				repl := f.Key[:n] + r.New
				if f.Key[n] == 'V' {
					repl = f.Key[:n] + strings.ToUpper(r.New[:1]) + r.New[1:]
				}
				hits = append(hits, fmt.Sprintf("schema %s: ADR-0129 rows %s: retired %q — use %s", key, r.Row, f.Key, prefix+repl))
			}
			walk(scope, path+".", f.Fields)
		}
	}
	walk("envelope", "", doc.EnvelopeShape.Fields)
	for _, op := range doc.Operations {
		if op.Request != nil {
			walk(op.ID+" REQ", "", op.Request.Fields)
		}
		walk(op.ID+" RES", "", op.Result.Fields)
	}
	sort.Strings(hits)
	return hits, revisionKeys, seen
}

// testRetiredSchemaWalk seals the bare record-revision key over the live wire
// contract.
func testRetiredSchemaWalk(t *testing.T) {
	doc, err := app.Schema(nil)
	if err != nil {
		t.Fatalf("app.Schema: %v (fail closed)", err)
	}
	if len(doc.Operations) < 75 {
		t.Fatalf("population floor: schema walk saw %d operations (expected >= 75)", len(doc.Operations))
	}
	hits, revs, seen := schemaVersionKeyHits(doc)
	if revs < 60 {
		t.Fatalf("population floor: schema walk saw %d *revision keys (expected >= 60)", revs)
	}
	// A kept entry the live schema no longer carries is a stale exemption.
	for k := range schemaKeptVersionKeys {
		if !seen[k] {
			t.Errorf("kept schema key %q is absent from the live schema: delete it from schemaKeptVersionKeys", k)
		}
	}
	if len(hits) != 0 {
		t.Errorf("retired ADR-0129 record-revision keys in the schema (%d):\n%s", len(hits), strings.Join(hits, "\n"))
	}
}
```

Add `t.Run("catalog_vocabulary", testRetiredCatalogVocabulary)` FIRST in `TestRetiredVocabularySeal`, and `t.Run("schema_walk", testRetiredSchemaWalk)` after `generator_output`:

```go
// testRetiredCatalogVocabulary derives the operation references the bound-flag
// scan needs and fails closed if the catalog cannot be read or has collapsed.
func testRetiredCatalogVocabulary(t *testing.T) {
	re, ids, err := catalogOpRefs()
	if err != nil {
		t.Fatalf("catalog operation references: %v (fail closed)", err)
	}
	if ids < 70 {
		t.Fatalf("population floor: %d dotted operation ids (expected >= 70)", ids)
	}
	for _, fam := range []string{"change.claim", "finalize.merge", "workspace.prepare", "docket change claim"} {
		if !re.MatchString(" " + fam + " ") {
			t.Errorf("operation reference matcher does not recognise %q", fam)
		}
	}
}
```

- [ ] **Step 4: Non-vacuity and negative controls**

In `testRetiredNonVacuity`'s `switch r.Kind`, add:

```go
		case kindBoundFlag:
			line := "the `change.claim` operation with `--id <id> " + r.Old + " <v>`"
			hits = append(hits, scanTextContent("skills/x/SKILL.md", line)...)
			hits = append(hits, scanTextContent("tests/test_x.sh", "docket change claim --id 1 "+r.Old+" v")...)
			hits = append(hits, goHits("internal/cli/change.go", "package p\nfunc f(c *C) { c.Flags().String("+strconv.Quote(strings.TrimPrefix(r.Old, "--"))+", \"\", \"x\") }\n")...)
			want = 3
		case kindSchemaKey:
			doc := app.SchemaResult{Operations: []app.OperationSchema{{ID: "change.x", Request: &app.TypeDescriptor{Fields: []app.FieldDescriptor{{Key: r.Old}}}}}}
			sh, _, _ := schemaVersionKeyHits(doc)
			if len(sh) != 1 || !strings.Contains(sh[0], r.New) {
				t.Errorf("row %s: a planted %q key was not detected naming %q: %v", r.Row, r.Old, r.New, sh)
			}
			continue
```

Then, after the existing row-38g block, add the row-40 binding cases (each must hit row "40"):

```go
	for _, c := range []struct{ rel, text string }{
		{"skills/x/SKILL.md", "docket finalize merge --id 1 --version v --head h"},
		{"skills/x/SKILL.md", "workspace.prepare  --id <id> --version <v>   # resolve argv from the capability catalog"},
		// The flag wrapped onto the line after its operation still binds (edge-paths shape).
		{"skills/x/SKILL.md", "resume through the `change.resume-halted` operation with `--id <id>\n--version <v> --acknowledge-quiescent`"},
		{"tests/test_x.sh", "docket change repair-identity --id 1 --version v   # a bare --version on repair-identity is bound too"},
	} {
		if !hasRetiredRow(scanTextContent(c.rel, c.text), "40") {
			t.Errorf("row 40: a bound --version in %s was not detected: %q", c.rel, c.text)
		}
	}
	for _, src := range []string{
		"package p\nfunc f(c *C) { v, _ := c.Flags().GetString(\"version\"); _ = v }\n",
		"package p\nfunc f(c *C) { _ = c.MarkFlagRequired(\"version\") }\n",
	} {
		if !hasRetiredRow(goHits("internal/cli/finalize.go", src), "40") {
			t.Errorf("row 40: a bound Go flag call was not detected: %q", src)
		}
	}
```

In `testRetiredNegativeControls`, extend `cleanText` with:

```go
		// Change 0472 — kept namesakes and unbound --version (spec §3 shape boundaries).
		"docket version --json",
		"run `releasepkg --source <dir> --version v1.2.3 --commit <sha> --source-epoch 1 --out <dir>`",
		"sh install.sh --version v1.2.3 --harness claude",
		"`opencode run --version` prints the version, while `opencode run -- --version` sends it as the message",
		"record `codex --version` alongside any finding",
		// Nearest binding wins: run.start follows change.claim, so codex's --version is unbound.
		"the `change.claim` operation, then `docket run start implement-next`, then record `codex --version`",
		// A blank line ends the block, so the op cannot bind a --version below it.
		"the `change.claim` operation with `--id <id> --revision <v>`\n\nthen record `cursor-agent --version`",
		"the `change.claim` operation with `--id <id> --revision <v>`",
		"docket change repair-identity --id 1 --expect-revision <v> --adopt-pr-head",
		"the rebase receipt keeps resolver_budget_version",
		"every mutation result carries committed_revision, metadata_revision and base_branch_revision",
		"the claim digest payload keeps its version key",
```

and `cleanGo` with:

```go
		{"cmd/releasepkg/main.go", "package p\nvar v = fs.String(\"version\", \"\", \"safe release version\")\n"},
		{"internal/cli/root.go", "package p\nvar c = capability(\"version\", EffectRead)\n"},
		{"internal/cli/root.go", "package p\nvar c = &C{Use: \"version\"}\n"},
		{"internal/cli/install.go", "package p\nvar m = map[string]bool{\"version\": true}\n"},
		{"internal/cli/change.go", "package p\nfunc f(c *C) { c.Flags().String(\"revision\", \"\", \"x\") }\n"},
		{"internal/app/change_claim.go", "package p\ntype P struct {\n\tRevision string `json:\"version\"`\n}\n"},
		{"internal/workspace/rebasereceipt.go", "package p\ntype R struct {\n\tB string `json:\"resolver_budget_version,omitempty\"`\n}\n"},
```

and add schema-walk controls at the end of `testRetiredNegativeControls`:

```go
	// Kept schema keys are exact (scope, path) pairs: clean where kept, retired
	// anywhere else.
	kept := app.SchemaResult{
		EnvelopeShape: app.TypeDescriptor{Fields: []app.FieldDescriptor{{Key: "protocol_version"}}},
		Operations: []app.OperationSchema{
			{ID: "capabilities", Result: app.TypeDescriptor{Fields: []app.FieldDescriptor{{Key: "capability_version"}, {Key: "binary", Fields: []app.FieldDescriptor{{Key: "version"}}}}}},
			{ID: "diagnostic.runtime", Result: app.TypeDescriptor{Fields: []app.FieldDescriptor{{Key: "go_version"}}}},
			{ID: "version", Result: app.TypeDescriptor{Fields: []app.FieldDescriptor{{Key: "version"}}}},
			{ID: "change.claim", Result: app.TypeDescriptor{Fields: []app.FieldDescriptor{{Key: "committed_revision"}}}},
		},
	}
	if hits, _, _ := schemaVersionKeyHits(kept); len(hits) != 0 {
		t.Errorf("kept schema keys matched: %v", hits)
	}
	moved := app.SchemaResult{Operations: []app.OperationSchema{
		{ID: "status", Result: app.TypeDescriptor{Fields: []app.FieldDescriptor{{Key: "binary", Fields: []app.FieldDescriptor{{Key: "version"}}}}}},
	}}
	if hits, _, _ := schemaVersionKeyHits(moved); len(hits) != 1 || !strings.Contains(hits[0], "binary.revision") {
		t.Errorf("a kept spelling at an unkept path was not detected naming binary.revision: %v", hits)
	}
```

- [ ] **Step 5: Run the seal**

Run: `go test -count=1 ./internal/repoguard/ -run TestRetiredVocabularySeal -v`
Expected: PASS on every subtest. A `maintained_surfaces` or `schema_walk` failure lists a real survivor with its replacement. Fix it at the source (the skill, Go literal or tag), never by widening an exemption. The one exception is a KEPT software `--version` (release tooling, a foreign CLI) that the scan reads as bound. At plan time no kept block holds a change/finalize/workspace reference, so that would be a new finding. Do not reword the kept site and do not widen the predicate: return NEEDS_ESCALATION naming the file, the block and its nearest reference.

- [ ] **Step 6: Mutation tests (one per shape class, plus the kept set)**

```bash
seal() { go test -count=1 ./internal/repoguard/ -run TestRetiredVocabularySeal 2>&1 | tail -40; }
# (1) compound token, markdown
f=skills/docket-groom-next/SKILL.md; cp "$f" "$f.bak"
perl -0pi -e 's/spec_revision/spec_version/' "$f"; grep -c 'spec_version' "$f"
seal   # expect FAIL: row 42 ... retired "spec_version" — use spec_revision
mv -f "$f.bak" "$f"
# (2) bound --version, markdown
f=skills/docket-implement-next/SKILL.md; cp "$f" "$f.bak"
perl -0pi -e 's/(`change\.claim` operation with `--id <id> )--revision/$1--version/' "$f"; grep -c -e '--id <id> --version' "$f"
seal   # expect FAIL: row 40 ... retired "--version" — use --revision
mv -f "$f.bak" "$f"
# (3) bound --version, Go
f=internal/cli/workspace.go; cp "$f" "$f.bak"
perl -0pi -e 's/prepare\.Flags\(\)\.String\("revision"/prepare.Flags().String("version"/' "$f"; grep -c 'String("version"' "$f"
seal   # expect FAIL: internal/cli/workspace.go ... row 40 ... use --revision
mv -f "$f.bak" "$f"
# (4) schema-walk key
f=internal/app/status_result.go; cp "$f" "$f.bak"
perl -0pi -e 's/(type StatusRecord struct \{.*?)json:"revision"/$1json:"version"/s' "$f"; grep -c 'json:"version"' "$f"
seal   # expect FAIL: schema status RES records.version ... use records.revision
mv -f "$f.bak" "$f"
# (5) the kept set is load-bearing, not decoration
f=internal/repoguard/retired_vocabulary_test.go; cp "$f" "$f.bak"
perl -0pi -e 's/\t"capabilities RES binary.version": +true,\n//' "$f"; grep -c 'capabilities RES binary.version' "$f"
seal   # expect FAIL: schema capabilities RES binary.version ... use binary.revision (and the kept-set negative control)
mv -f "$f.bak" "$f"
seal   # expect PASS
```
Before trusting any reading, confirm each `grep -c` shows the mutation landed (non-zero for 1-4, zero for 5). If mutation (1) does not redden, check whether the groom-next skill still carries a `spec_revision` to flip. If not, plant `spec_version` in any skill line instead: the guard is under test, not the site.

- [ ] **Step 7: Commit**

```bash
git add internal/repoguard/retired_vocabulary_test.go
git commit -m "test(repoguard): seal ADR-0129 family (b) retired revision spellings (change 0472)"
```

---

### Task 8: Whole-repo residue audit and the full suite gate

Profile hint: standard.

**Files:**
- Modify: only what the audit finds (expected: nothing)

**Interfaces:**
- Consumes: Tasks 1-7.
- Produces: the gate evidence: a clean residue classification and a green whole suite with its budget report read.

- [ ] **Step 1: Run the residue grep**

```bash
out=$(git grep -n -I -E -e '--version|expect-version|ExpectVersion|expectVersion|spec_version|SpecVersion|pr_version|PRVersion|ExpectedVersion|expectedVersion|computeVersion|VersionKind|VersionBlob|VersionAbsent|version-mismatch|version-drift|reclaim-version-missing|empty-version|empty-change-version|empty-target-version|empty-spec_version|invalid-spec_version|empty-child_pr_version|<entity-version>|entity version|change version|record version|exact.version|"version"' -- . ':!docs/changes' ':!docs/results' ':!docs/superpowers' ':!docs/adrs' ':!**/testdata/**' ':!tests/fixtures/**' ':!internal/install/legacydata/**' || true)
printf '%s\n' "$out" | wc -l
printf '%s\n' "$out"
```

Classify every hit into exactly one of these classes, or fix it:
1. **Kept software/format version (Decision 4):** `cmd/releasepkg/**`, `internal/release/**`, `scripts/release-smoke.*`, `tests/test_release_downloader*.sh`, `.github/workflows/release-candidate.yml`, `scripts/runners/opencode.*`, `docs/reference/harness/**` (`codex --version`, `cursor-agent --version`), the `version` operation (`internal/cli/root.go`, `internal/cli/install.go`, `internal/app/version.go`, `schema_registry.go`, their tests), `capabilities` `binary.version` (`internal/app/capabilities.go` and its test), `internal/codexentry/client.go` `clientInfo` `version`.
2. **Kept committed state:** `claimDigestPayload`'s `json:"version"` and Task 1's test.
3. **Deliberate old-spelling tests:** `internal/cli/revision_rename_test.go`, `internal/repoguard/retired_vocabulary_test.go`, `internal/app/schema_revision_test.go`.
4. **History comments:** `internal/repoguard/budgets_test.go` row comments recording what earlier changes added (for example "+spec_version pin"). They are point-in-time notes inside a test file, which the seal does not scan.

Anything else is a missed site: rename it (and re-run `go generate ./internal/assets/` if it was under `skills/`). Record the class counts in the task report.

- [ ] **Step 2: Run the full suite through the build gate command**

Run: `go run ./cmd/docket development test` (the value of `build.test_command` in `.docket.yml`)
Expected: PASS. Then read the budget report. Report any `BUDGET WATCH:`, `PARALLEL-SENSITIVE:` or `SERIAL CONFIRMED OVER BUDGET:` line, with the margin for the `internal/repoguard` and `internal/cli` rows. The seal now imports `internal/cli` and `internal/app` and builds the catalog in-process, so repoguard's compile and run time moves. Report it as a number, not "did not trip".

- [ ] **Step 3: Commit (only if Step 1 fixed anything)**

```bash
git add <each fixed path>
git commit -m "fix: rename the last record-revision spellings the residue audit found (change 0472)"
```

---

## Landing (human procedure; for the results file's `**Human action:**` and the PR body)

1. Merge only when no dispatched implement-next or finalize run is in flight (drain or cancel first); their loaded skills send `--version`, which the new binary refuses.
2. Run the post-merge binary rebuild immediately (AGENTS.md *Rebuild the binary after a merge to main*).
3. Restart open coordinator sessions (their loaded skills name `--version`).
4. Re-run `docket install` in every consumer repo.
5. On other machines, switch binaries only when no run is in flight there. No storage cleanup is needed.

A skipped step fails loudly (unknown flag / unknown field); recovery is re-dispatch with the new binary and skills. The human's agent-memory notes that name `workspace prepare --version` and `finalize clear-block --version` are updated by the human's session after landing, not by this build.
