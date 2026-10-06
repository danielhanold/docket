<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0529 — Repoint a merged PR's change backlink when the change is archived](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0529-repoint-a-merged-pr-s-change-backlink-when-the-change-is-arc.md)**
<!-- docket:backlink:end -->
# Repoint a Merged PR's Change Backlink on Archive — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. In this repository the plan is executed by `docket-build`.

**Goal:** When close-out archives a change, rewrite the docket-owned `docket:backlink` block in the merged PR's description so it links to the archived record; the cleanup leg and the maintenance sweep retry a failed edit; `docket repository repair --pr-backlinks` repairs the PRs that are already broken, after a preview and a human `--yes`.

**Architecture:** One new GitHub adapter primitive, `githubcli.Client.EditPullRequestBody`, edits one PR's body by number. It follows `RetargetPullRequest`'s probe→act→verify shape: an exact-revision gate, the body sent on stdin only, a fresh by-number re-read to verify. One new app file, `internal/app/pr_backlink_repoint.go`, owns the only decision about whether a PR body needs repointing (`planPRBacklinkRepoint`, keyed on whether the block already names the archive path) and the only edit sequence (`repointPRBacklink` / `applyPRBacklinkRepoint`). Close-out, cleanup, the sweep assessment, and the repair all call these; none of them keeps its own copy. Close-out and cleanup reach GitHub through a narrow `FinalizePRBody` seam that falls back to `deps.GitHub` when that value satisfies it, the same way `cleanupGit` falls back to `Planning.Client`. The sweep reads PR bodies once per invocation through the existing batched reader (`SweepPRBatchReader.ProbePRSet`). The repair gets its GitHub seam through `SetupDeps.GitHub`, and the CLI sets that field only for `--pr-backlinks`, so routine `repository` commands cannot read a PR body.

**Tech Stack:** Go (`internal/githubcli`, `internal/app`, `internal/cli`), the protocol-faithful fake `gh` in `internal/githubcli/harness_integration_test.go`, real-git integration fixtures behind the `integration` build tag, POSIX shard runners under `tests/`.

**Spec:** `docs/superpowers/specs/2026-10-05-repoint-a-merged-pr-s-change-backlink-when-the-change-is-arc-design.md` (on the `docket` branch; read it at `/Users/homer/dev/docket/.docket/docs/superpowers/specs/2026-10-05-repoint-a-merged-pr-s-change-backlink-when-the-change-is-arc-design.md`).

## Global Constraints

- **Best-effort, never blocking.** A failed or unknown GitHub edit never changes close-out's disposition or result. It becomes a `pr-backlink-pending` warning finding, and cleanup and the sweep retry it.
- **Idempotent.** A body whose backlink block already names the archive path is a no-op (no edit issued). A body with no backlink block is left untouched and never given one.
- **Only the docket-owned block changes.** Every authored byte outside `<!-- docket:backlink:start … -->`…`<!-- docket:backlink:end -->` survives byte-for-byte, including CRLF line endings and the build-evidence block. Use the existing `document.Parse` / `PatchSet.ReplaceBlock` / `Apply` path (the same one `assemblePRBody` and `backlinkLegRetarget` use).
- **Redaction.** PR body bytes never appear in an argv element, a result field, a finding message, or a diagnostic. They travel to `gh` on stdin only (`--body-file -`). The only body-derived text that may appear in output is the backlink block's own generated interior line, in the repair preview.
- **Three outcomes for every probe.** A read error, a batch failure, a `Found=false` slot, or malformed markers is *unknown*: never a clean no-op and never authority to edit (learning `probe-error-is-not-clean-absence`).
- **Edit only under the exact revision.** An edit is issued only when the caller's expected revision equals the live one. Drift is `contended`, and the PR is not touched.
- **No new blocking gate.** Nothing in this change can stop a close-out, a cleanup, or a sweep. A stuck PR leg is a visible warning only.
- `repository repair` with no flag, `repository check`, and `repository prepare` make **no** GitHub PR-body reads. Only `repository repair --pr-backlinks` constructs a GitHub client.
- `repository.repair`'s catalog effects become `external-write metadata-write`. `finalize.closeout`'s become `external-write metadata-write`, because close-out now edits GitHub.
- The PR-body leg is separate from the integration-branch backlink legs (`runCloseoutBacklinkLeg`, `finalizeCleanupBacklinkRepair`, `sweepAssessBacklinkLeg`). Do not merge them, rename them, or change their behavior.
- Out of scope, unchanged: the backlink wording; adding links to the PR body; PRs of killed changes (filter on `domain.StatusDone`); private-visibility repositories; moving plans, results, or evidence.
- Docs describe current behavior only, with no change or PR numbers (`TestLivingDocsAlignment`). ADR citations are allowed. Never hand-copy under `internal/assets/embedded/`; regenerate it with `go generate ./internal/assets/`.
- Cross-references in maintained source anchor on symbol names or quoted clauses, never line numbers (`TestCommentAnchorStyle`).
- Mint every finding code from a named constant, never a `Code: "..."` literal (`TestNoInlineFindingCodeLiterals`). The new `ReasonPRBacklinkPending` follows the existing `ReasonCloseoutBacklinkPending` pattern.
- Every mutation probe and re-verification uses `go test -count=1` (learning `cached-runner-serves-a-mutated-tree`). Every mutation is restored from a backup copy (`cp f f.bak` … `mv -f f.bak f`), never `git checkout --` (learning `mutation-restore-needs-a-backup-copy`).
- Integration tests run only through their shard prefixes: `go test -tags integration -count=1 -run '^<Prefix>' <pkg>`. Never run `./internal/app/` integration tests without `-run`. The prefixes used here already exist: `TestIntegrationMerge` (githubcli merge shard), `TestIntegrationFinalizeCloseout`, `TestIntegrationFinalizeArchive`, `TestIntegrationFinalizeCleanup`, `TestIntegrationRepoRepair`. Do not add shards.
- **Never mutate a real PR during the build.** Task 1's real `gh` call is read-only. Never run `repository repair --pr-backlinks --yes` against `danielhanold/docket`; that one-time run is the human's after merge.
- Whole-suite gate: `go run ./cmd/docket development test`. Read its budget report even when green.

## Review Focus

1. **A PR body with CRLF line endings** (any body edited in the GitHub web UI). A person expects the CRLF bytes outside the block to survive unchanged and only the block to change. Pinned in Task 2, `TestPlanPRBacklinkRepointPreservesCRLFProse`.
2. **A PR body whose backlink markers are malformed** (dangling start, end before start). A person expects docket to leave it alone and say so: close-out and cleanup report `pr-backlink-pending`, the sweep reports the leg as unknown, and the repair reports it unreadable and skips it. It is never edited, and nothing is inserted. Pinned in Task 2 (`TestPlanPRBacklinkRepointMalformedMarkersIsError`, `TestRepointPRBacklinkMalformedNeverEdits`), Task 5 (`TestAssessPRBacklinkMalformedIsUnknown`), and Task 6 (`TestIntegrationRepoRepairPRBacklinksSkipsUnreadable`).
3. **A human edits the PR description between docket's read and its write.** A person expects docket not to overwrite their edit: the outcome is `contended`, and the next retry reads the new body. Pinned in Task 1 (`contended-revision-drift`, `verify-shows-different-body`) and Task 6 (the contended entry in `TestIntegrationRepoRepairPRBacklinksSkipsUnreadable`).
4. **A stack root closing out with carried descendants.** A person expects each descendant's PR to link to that descendant's own archive path, not the root's. Pinned in Task 3, subtest `repoints-each-carried-pr-backlink`.
5. **A PR the batched read cannot see** (a `Found=false` slot or a failed batch). A person expects the sweep to report that record as unknown rather than certify it as no-work, and the repair to report the PR as unreadable rather than drop it silently. Pinned in Task 5 (`TestGatherSweepSharedFactsMarksUnreadPRBodiesUnknown`) and Task 6.

## File Structure

- `internal/githubcli/bodyedit.go` (create): `BodyEditOutcome`, `EditPullRequestBody`.
- `internal/githubcli/bodyedit_integration_test.go` (create): fake-gh probe→act→verify matrix, prefix `TestIntegrationMergeBodyEdit` (runs in the existing `tests/test_go_integration_githubcli_merge.sh` shard).
- `internal/app/pr_backlink_repoint.go` (create): `ReasonPRBacklinkPending`, `PRBodyEditor`, `FinalizePRBody`, `prBodyEditor`, `planPRBacklinkRepoint`, `repointPRBacklink`, `applyPRBacklinkRepoint`, `prBacklinkFinding`.
- `internal/app/pr_backlink_repoint_test.go` (create, untagged so integration tests can share its fake): `fakePRBody`, `prBodyWithActiveBacklink`, and the unit tests.
- `internal/app/finalize_context.go` (modify): `FinalizeDeps.PRBody`.
- `internal/app/finalize_closeout.go` (modify): `runCloseoutPRBacklinkLeg`, called from `closeoutIntegrationDestination`; file header comment.
- `internal/app/finalize_closeout_integration_test.go` (modify): new `TestIntegrationFinalizeCloseoutPRBacklink*` tests; new subtest in `TestIntegrationFinalizeArchiveRootCarry`.
- `internal/app/finalize_cleanup.go` (modify): `finalizeCleanupPRBacklinkRepair`, called from `finalizeCleanupDone`; file header comment.
- `internal/app/finalize_cleanup_integration_test.go` (modify): `TestIntegrationFinalizeCleanupPRBacklinkRetry`.
- `internal/app/sweep_prfacts.go` (modify): `SweepPRSetResult.Bodies`.
- `internal/app/maintenance.go` (modify): `gatherSweepSharedFacts` reads done candidates' PR bodies once; `MaintenanceSweep` passes `repoDir`.
- `internal/app/maintenance_assess.go` (modify): `sweepSharedFacts` PR-body fields, `sweepLegPRBacklink`, `sweepAssessPRBacklinkLeg`.
- `internal/app/maintenance_assess_test.go`, `internal/app/sweep_prfacts_test.go` (modify).
- `internal/app/repository_facts.go` (modify): `SetupDeps.GitHub`.
- `internal/app/repository_repair.go` (modify): `RepairOptions.PRBacklinks`, `repairPreflight` (extracted), result field `PRBacklinks`.
- `internal/app/repository_repair_prbacklinks.go` (create): `RepairGitHub`, `PRBacklinkRepair`, outcome constants, `runPRBacklinkRepair` and helpers.
- `internal/app/repository_repair_integration_test.go` (modify): `TestIntegrationRepoRepairPRBacklinks*`, `TestIntegrationRepoRepairRoutineCommandsReadNoPRBodies`.
- `internal/cli/repository.go`, `internal/cli/repository_test.go` (modify): `--pr-backlinks`, `repositoryRepairGitHub`, effects.
- `internal/cli/finalize.go`, `internal/cli/finalize_test.go` (modify): `finalize.closeout` effects.
- `docs/concepts/finalize-sequencer.md`, `docs/reference/glossary.md`, `skills/docket-finalize-change/SKILL.md` (modify) and the regenerated `internal/assets/embedded/`.

---

### Task 1: Confirm a merged PR's body is editable, then add `EditPullRequestBody`

**Files:**
- Create: `internal/githubcli/bodyedit.go`
- Test: `internal/githubcli/bodyedit_integration_test.go`

**Interfaces:**
- Consumes: `(*Client).ViewPullRequest` (`internal/githubcli/probe.go`), `(*Client).run`/`runRequest` (`client.go`), `validateRepository`, `newFailure`, `AsFailure`, `StageLaunch`, `StageValidate`, `KindInvalidInput` (`failure.go`). Test helpers `newFakeClient`, `fakeScenario`, `fakeArm`, `countArgv`, `ensPRJSON`, `mustDecodeOne`, `retRepo`, `retViewArm`, `retEditArm`, `ensHead`, `ensHeadOid`, `ensTitle` (`harness_integration_test.go`, `ensure_integration_test.go`, `retarget_integration_test.go`, `fixtures_test.go`).
- Produces:
  ```go
  type BodyEditOutcome string
  const (
      BodyEdited    BodyEditOutcome = "edited"
      BodyAlready   BodyEditOutcome = "already"
      BodyContended BodyEditOutcome = "contended"
      BodyUnknown   BodyEditOutcome = "unknown"
  )
  func (c *Client) EditPullRequestBody(ctx context.Context, repo Repository, number int, expectedRevision, body string) (BodyEditOutcome, PullRequest, error)
  ```

- [ ] **Step 1: Probe GitHub for real (read-only) and record the result**

The spec requires a real `gh` call confirming that a merged PR's body is editable. Use GitHub's own permission field. It is a read and mutates nothing:

```bash
gh --version
gh api graphql \
  -f query='query($o:String!,$n:String!){repository(owner:$o,name:$n){pullRequest(number:250){number state viewerCanUpdate}}}' \
  -f o=danielhanold -f n=docket
```

Expected: a JSON document with `"state":"MERGED"` and `"viewerCanUpdate":true`. If `viewerCanUpdate` is `false`, if the call fails, or if `gh` is not authenticated, STOP and return BLOCKED with the output. The whole change depends on this fact. Copy the exact commands and their full output into this task's commit message body under a `Verification:` heading, and into your COMPLETE report under `Verification evidence`. The results file is assembled from that report. Do NOT run `gh pr edit` or any other write against a real PR.

- [ ] **Step 2: Write the failing fake-gh tests**

Create `internal/githubcli/bodyedit_integration_test.go`:

```go
//go:build integration

package githubcli

import (
	"context"
	"strings"
	"testing"
)

// EditPullRequestBody drives probe→act→verify for one merged PR's body through
// the protocol-faithful fake gh. Each case asserts the outcome AND the witness
// log (whether an edit ran, and what it carried on stdin), so a green result
// can never hide an unexercised or an extra external mutation.

const (
	bodyOld = "<!-- docket:backlink:start (generated — do not hand-edit) -->\n> ↩ old\n<!-- docket:backlink:end -->\n\nAuthored prose.\r\n"
	bodyNew = "<!-- docket:backlink:start (generated — do not hand-edit) -->\n> ↩ new\n<!-- docket:backlink:end -->\n\nAuthored prose.\r\n"
)

func mergedPRJSON(body string) string {
	return ensPRJSON(7, "MERGED", false, ensHead, ensHeadOid, "main", ensTitle, body)
}

func editRecord(t *testing.T, log *witnessLog) (invocationRecord, int) {
	t.Helper()
	var found invocationRecord
	n := 0
	for _, r := range log.records(t) {
		if len(r.Argv) >= 2 && r.Argv[0] == "pr" && r.Argv[1] == "edit" {
			found = r
			n++
		}
	}
	return found, n
}

func TestIntegrationMergeBodyEditProbeActVerify(t *testing.T) {
	oldRev := mustDecodeOne(t, mergedPRJSON(bodyOld)).Revision

	t.Run("edited", func(t *testing.T) {
		c, log := newFakeClient(t, fakeScenario{Sequential: true, Invocations: []fakeArm{
			retViewArm(mergedPRJSON(bodyOld), 0),
			retEditArm(0),
			retViewArm(mergedPRJSON(bodyNew), 0),
		}})
		out, pr, err := c.EditPullRequestBody(context.Background(), retRepo(), 7, oldRev, bodyNew)
		if err != nil || out != BodyEdited {
			t.Fatalf("outcome = %q err=%v, want %q", out, err, BodyEdited)
		}
		if pr.Body != bodyNew {
			t.Fatalf("verified body differs from the request")
		}
		rec, n := editRecord(t, log)
		if n != 1 {
			t.Fatalf("pr edit issued %d times, want 1", n)
		}
		if rec.Stdin != bodyNew {
			t.Fatalf("edit stdin = %q, want the exact requested body", rec.Stdin)
		}
		joined := strings.Join(rec.Argv, "\x00")
		for _, want := range []string{"7", "--repo", "acme/widget", "--body-file", "-"} {
			if !strings.Contains(joined, want) {
				t.Errorf("edit argv %v lacks %q", rec.Argv, want)
			}
		}
		for _, a := range rec.Argv {
			if strings.Contains(a, "Authored prose") {
				t.Fatalf("body bytes leaked into argv: %v", rec.Argv)
			}
		}
		for _, forbidden := range []string{"--base", "--title"} {
			if strings.Contains(joined, forbidden) {
				t.Errorf("a body edit must not carry %s: %v", forbidden, rec.Argv)
			}
		}
	})

	t.Run("already", func(t *testing.T) {
		c, log := newFakeClient(t, fakeScenario{Invocations: []fakeArm{retViewArm(mergedPRJSON(bodyNew), 0)}})
		out, _, err := c.EditPullRequestBody(context.Background(), retRepo(), 7, oldRev, bodyNew)
		if err != nil || out != BodyAlready {
			t.Fatalf("outcome = %q err=%v, want %q", out, err, BodyAlready)
		}
		if _, n := editRecord(t, log); n != 0 {
			t.Fatalf("pr edit issued %d times on an already-correct body, want 0", n)
		}
	})

	t.Run("contended-revision-drift", func(t *testing.T) {
		c, log := newFakeClient(t, fakeScenario{Invocations: []fakeArm{retViewArm(mergedPRJSON(bodyOld), 0)}})
		out, _, err := c.EditPullRequestBody(context.Background(), retRepo(), 7, "sha256:stale", bodyNew)
		if err != nil || out != BodyContended {
			t.Fatalf("outcome = %q err=%v, want %q", out, err, BodyContended)
		}
		if _, n := editRecord(t, log); n != 0 {
			t.Fatalf("pr edit issued %d times on revision drift, want 0", n)
		}
	})

	t.Run("empty-revision-is-contended", func(t *testing.T) {
		c, log := newFakeClient(t, fakeScenario{Invocations: []fakeArm{retViewArm(mergedPRJSON(bodyOld), 0)}})
		out, _, _ := c.EditPullRequestBody(context.Background(), retRepo(), 7, "", bodyNew)
		if out != BodyContended {
			t.Fatalf("outcome = %q, want %q", out, BodyContended)
		}
		if _, n := editRecord(t, log); n != 0 {
			t.Fatalf("an empty revision authorized %d edits, want 0", n)
		}
	})

	t.Run("probe-error-unknown", func(t *testing.T) {
		c, log := newFakeClient(t, fakeScenario{Invocations: []fakeArm{retViewArm("", 1)}})
		out, _, err := c.EditPullRequestBody(context.Background(), retRepo(), 7, oldRev, bodyNew)
		if out != BodyUnknown || err == nil {
			t.Fatalf("outcome = %q err=%v, want %q with a diagnostic", out, err, BodyUnknown)
		}
		if _, n := editRecord(t, log); n != 0 {
			t.Fatalf("pr edit issued %d times after a probe error, want 0", n)
		}
	})

	t.Run("verify-shows-different-body", func(t *testing.T) {
		c, _ := newFakeClient(t, fakeScenario{Sequential: true, Invocations: []fakeArm{
			retViewArm(mergedPRJSON(bodyOld), 0),
			retEditArm(0),
			retViewArm(mergedPRJSON("a human rewrote it\n"), 0),
		}})
		out, _, err := c.EditPullRequestBody(context.Background(), retRepo(), 7, oldRev, bodyNew)
		if err != nil || out != BodyContended {
			t.Fatalf("outcome = %q err=%v, want %q", out, err, BodyContended)
		}
	})

	t.Run("nonzero-edit-exit-resolved-by-verify", func(t *testing.T) {
		c, _ := newFakeClient(t, fakeScenario{Sequential: true, Invocations: []fakeArm{
			retViewArm(mergedPRJSON(bodyOld), 0),
			retEditArm(1),
			retViewArm(mergedPRJSON(bodyNew), 0),
		}})
		out, _, err := c.EditPullRequestBody(context.Background(), retRepo(), 7, oldRev, bodyNew)
		if err != nil || out != BodyEdited {
			t.Fatalf("a landed edit with a lost response must verify as %q, got %q err=%v", BodyEdited, out, err)
		}
	})

	t.Run("verify-error-unknown", func(t *testing.T) {
		c, _ := newFakeClient(t, fakeScenario{Sequential: true, Invocations: []fakeArm{
			retViewArm(mergedPRJSON(bodyOld), 0),
			retEditArm(0),
			retViewArm("", 1),
		}})
		out, _, err := c.EditPullRequestBody(context.Background(), retRepo(), 7, oldRev, bodyNew)
		if out != BodyUnknown || err == nil {
			t.Fatalf("outcome = %q err=%v, want %q with a diagnostic", out, err, BodyUnknown)
		}
	})

	t.Run("invalid-number", func(t *testing.T) {
		c, log := newFakeClient(t, fakeScenario{})
		out, _, err := c.EditPullRequestBody(context.Background(), retRepo(), 0, oldRev, bodyNew)
		if out != BodyUnknown || err == nil {
			t.Fatalf("outcome = %q err=%v, want %q with a diagnostic", out, err, BodyUnknown)
		}
		if len(log.records(t)) != 0 {
			t.Fatalf("an invalid request must spawn no gh process")
		}
	})
}
```

Before relying on the `invalid-number` case, check that `newFakeClient` accepts an empty scenario and that `log.records` returns an empty slice when the log file was never written. If it does not, script one never-matched arm (`fakeArm{ArgvPrefix: []string{"never"}}`) instead. A supplied assert is unverified code (learning `plan-supplied-test-code-is-unverified`).

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test -tags integration -count=1 -run '^TestIntegrationMergeBodyEdit' ./internal/githubcli/`
Expected: FAIL to compile, `c.EditPullRequestBody undefined`.

- [ ] **Step 4: Implement `bodyedit.go`**

Create `internal/githubcli/bodyedit.go`:

```go
package githubcli

// This file owns EditPullRequestBody: the probe→act→verify body edit of one
// exact pull request by number, in any state (a merged PR included). It is the
// primitive close-out, cleanup, and `repository repair --pr-backlinks` use to
// repoint a merged PR's docket-owned backlink block at the archived change
// record. It mirrors retarget.go's fixed shape:
//
//	1. probe the PR by number for its current body and opaque revision;
//	2. a body already equal to the request is `already` — no edit (idempotency
//	   keyed on the promised remote state, never on the revision);
//	3. an edit is authorized only when ExpectedRevision equals the live revision;
//	   an empty or mismatched revision is `contended` and the PR is untouched;
//	4. `gh pr edit <n> --repo <spec> --body-file -` carries the body on stdin
//	   only — never in argv or a diagnostic — and edits nothing else; and
//	5. a fresh by-number probe re-derives the postcondition: an equal body is
//	   `edited`, a different body is `contended` (a race, reported, never rolled
//	   back), and a probe that cannot be established is `unknown`.
//
// An errored probe is never read as clean absence (learning
// probe-error-is-not-clean-absence): a launch/timeout/cancel Failure or a
// non-zero exit is `unknown`, and `unknown` never authorizes the edit.

import (
	"context"
	"strconv"
)

// bodyEditOp labels every Failure raised while editing a pull-request body.
const bodyEditOp = "edit-pull-request-body"

// BodyEditOutcome is the closed set of body-edit outcomes.
type BodyEditOutcome string

const (
	// BodyEdited: the body was replaced and the postcondition verified.
	BodyEdited BodyEditOutcome = "edited"
	// BodyAlready: the PR already carried exactly the requested body; no edit.
	BodyAlready BodyEditOutcome = "already"
	// BodyContended: the live revision diverged from ExpectedRevision, or the
	// verified body differs from the request; the PR was not overwritten.
	BodyContended BodyEditOutcome = "contended"
	// BodyUnknown: an external probe could not establish the truth (or the input
	// was invalid); nothing was authorized. The returned error is the diagnostic.
	BodyUnknown BodyEditOutcome = "unknown"
)

// EditPullRequestBody replaces one exact PR's body, idempotently.
// edited/already/contended are value outcomes returned with a nil error; unknown
// carries a typed *Failure. The PR snapshot is populated on edited/already.
func (c *Client) EditPullRequestBody(ctx context.Context, repo Repository, number int, expectedRevision, body string) (BodyEditOutcome, PullRequest, error) {
	if err := validateRepository(repo); err != nil {
		return BodyUnknown, PullRequest{}, newFailure(bodyEditOp, StageValidate, KindInvalidInput, "repository identity invalid: "+err.Error(), err)
	}
	if number <= 0 {
		return BodyUnknown, PullRequest{}, newFailure(bodyEditOp, StageValidate, KindInvalidInput, "pull request number must be positive", nil)
	}

	pr, err := c.ViewPullRequest(ctx, repo, number)
	if err != nil {
		return BodyUnknown, PullRequest{}, err
	}
	if pr.Body == body {
		return BodyAlready, pr, nil
	}
	if expectedRevision == "" || expectedRevision != pr.Revision {
		return BodyContended, PullRequest{}, nil
	}

	_, mf := c.run(ctx, runRequest{
		op: bodyEditOp,
		args: []string{
			"pr", "edit", strconv.Itoa(number),
			"--repo", repo.Spec(),
			"--body-file", "-",
		},
		stdin:   []byte(body),
		network: true,
		write:   true,
	})
	if mf != nil && mf.Stage == StageLaunch {
		// gh never started; nothing was mutated.
		return BodyUnknown, PullRequest{}, mf
	}
	// A zero exit, a non-zero exit, a timeout, or a cancel may all have reached
	// GitHub: resolve every one by the same fresh verify probe.
	after, err := c.ViewPullRequest(ctx, repo, number)
	if err != nil {
		return BodyUnknown, PullRequest{}, err
	}
	if after.Body == body {
		return BodyEdited, after, nil
	}
	return BodyContended, PullRequest{}, nil
}
```

Check that `StageLaunch` is the constant `retarget.go` uses for "gh never started" (`grep -n StageLaunch internal/githubcli/failure.go`). Use whatever name it actually has.

- [ ] **Step 5: Run the tests to verify they pass, then mutation-test the revision gate**

Run: `go test -tags integration -count=1 -run '^TestIntegrationMergeBodyEdit' ./internal/githubcli/`
Expected: PASS.

Mutation: `cp internal/githubcli/bodyedit.go /tmp/bodyedit.go.bak`, delete the `expectedRevision == "" || expectedRevision != pr.Revision` block, rerun. Expected: `contended-revision-drift` and `empty-revision-is-contended` FAIL. Restore with `mv -f /tmp/bodyedit.go.bak internal/githubcli/bodyedit.go` and rerun to PASS. Also run the unit boundary guard: `go test -count=1 ./internal/githubcli/`.

- [ ] **Step 6: Commit**

```bash
git add internal/githubcli/bodyedit.go internal/githubcli/bodyedit_integration_test.go
git commit -m "feat(githubcli): edit a pull request's body by number under an exact revision" -m "Verification: <paste the Step 1 commands and their full output>"
```

---

### Task 2: The shared PR-backlink repoint decision and edit sequence

**Files:**
- Create: `internal/app/pr_backlink_repoint.go`
- Create: `internal/app/pr_backlink_repoint_test.go`
- Modify: `internal/app/finalize_context.go` (`FinalizeDeps`)

**Interfaces:**
- Consumes: `githubcli.BodyEditOutcome` and constants and `(*githubcli.Client).EditPullRequestBody` (Task 1); `document.Parse`, `Document.Block`, `Block.Interior` (`internal/document/markers.go`), `document.PatchSet.ReplaceBlock`, `Document.Apply`; `backlinkBlockName` (`artifact_backlink.go`); `StatusFinding`, `domain.SeverityWarning`.
- Produces:
  ```go
  const ReasonPRBacklinkPending = "pr-backlink-pending"
  const (
      prBacklinkRepointed = "repointed"
      prBacklinkAlready   = "already"
      prBacklinkNoBlock   = "no-block"
      prBacklinkContended = "contended"
      prBacklinkUnknown   = "unknown"
  )
  type PRBodyEditor interface {
      EditPullRequestBody(ctx context.Context, repo githubcli.Repository, number int, expectedRevision, body string) (githubcli.BodyEditOutcome, githubcli.PullRequest, error)
  }
  type FinalizePRBody interface {
      ViewPullRequest(ctx context.Context, repo githubcli.Repository, number int) (githubcli.PullRequest, error)
      PRBodyEditor
  }
  type prBacklinkResult struct{ outcome, current, detail string }
  func prBodyEditor(deps FinalizeDeps) FinalizePRBody
  func planPRBacklinkRepoint(body []byte, archivePath, archivedInterior string) (updated []byte, current string, present, needs bool, err error)
  func repointPRBacklink(ctx context.Context, ed FinalizePRBody, repo githubcli.Repository, number int, archivePath, archivedInterior string) prBacklinkResult
  func applyPRBacklinkRepoint(ctx context.Context, ed PRBodyEditor, repo githubcli.Repository, number int, body, revision, archivePath, archivedInterior string) prBacklinkResult
  func prBacklinkFinding(id, number int, r prBacklinkResult) *StatusFinding
  // FinalizeDeps gains: PRBody FinalizePRBody
  // test helpers (pr_backlink_repoint_test.go, untagged):
  type fakePRBody struct{ bodies map[int]string; viewErr, editErr error; editOut githubcli.BodyEditOutcome; views, edits int }
  func newFakePRBody(bodies map[int]string) *fakePRBody
  func prBodyWithActiveBacklink(activePath, authored string) string
  ```

The decision is **path-keyed**: a block whose interior already contains the archive path is a no-op, whatever its title wording or link form (both `render.BacklinkContent` forms, the `[…](<blob-url>)` link and the `` `relPath` `` fallback, end in the record path). A body that needs work gets the canonical rendered archive interior. The `archivedInterior` argument is always produced by the caller with the existing renderer (`archivedBacklinkInterior` or `render.BacklinkContent` + `backlinkInterior`), so the rendering is never copied.

- [ ] **Step 1: Write the failing tests**

Create `internal/app/pr_backlink_repoint_test.go`:

```go
package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/githubcli"
)

// fakePRBody is a scriptable FinalizePRBody keyed by PR number. Its revision is
// a digest of the current body, so a body change moves the revision exactly as
// GitHub's does. Shared with the integration tests (untagged file).
type fakePRBody struct {
	bodies  map[int]string
	viewErr error
	editErr error
	editOut githubcli.BodyEditOutcome // "" applies the edit; any other value is returned as-is
	views   int
	edits   int
}

func newFakePRBody(bodies map[int]string) *fakePRBody { return &fakePRBody{bodies: bodies} }

func (f *fakePRBody) rev(n int) string {
	sum := sha256.Sum256([]byte(f.bodies[n]))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func (f *fakePRBody) ViewPullRequest(_ context.Context, _ githubcli.Repository, n int) (githubcli.PullRequest, error) {
	f.views++
	if f.viewErr != nil {
		return githubcli.PullRequest{}, f.viewErr
	}
	b, ok := f.bodies[n]
	if !ok {
		return githubcli.PullRequest{}, errors.New("fake: no such pull request")
	}
	return githubcli.PullRequest{Number: n, State: githubcli.StateMerged, Body: b, Revision: f.rev(n)}, nil
}

func (f *fakePRBody) EditPullRequestBody(_ context.Context, _ githubcli.Repository, n int, rev, body string) (githubcli.BodyEditOutcome, githubcli.PullRequest, error) {
	f.edits++
	if f.editErr != nil {
		return githubcli.BodyUnknown, githubcli.PullRequest{}, f.editErr
	}
	if f.editOut != "" {
		return f.editOut, githubcli.PullRequest{}, nil
	}
	if rev != f.rev(n) {
		return githubcli.BodyContended, githubcli.PullRequest{}, nil
	}
	f.bodies[n] = body
	return githubcli.BodyEdited, githubcli.PullRequest{Number: n, Body: body, Revision: f.rev(n)}, nil
}

// prBodyWithActiveBacklink is a PR body as pr.publish leaves it: the backlink
// block (fallback form, naming the ACTIVE record path) at the top, then
// authored prose and a build-evidence-shaped block that must survive.
func prBodyWithActiveBacklink(activePath, authored string) string {
	return "<!-- docket:backlink:start (generated — do not hand-edit) -->\n" +
		"> ↩ **Change 0005 — A change** — `" + activePath + "`\n" +
		"<!-- docket:backlink:end -->\n\n" +
		authored + "\n\n" +
		"<!-- docket:build-evidence:start (generated — do not hand-edit) -->\nevidence\n<!-- docket:build-evidence:end -->\n"
}

const (
	tActive   = "docs/changes/active/0005-widget.md"
	tArchive  = "docs/changes/archive/2026-08-18-0005-widget.md"
	tInterior = "> ↩ **Change 0005 — A change** — `" + tArchive + "`"
)

// outsideBlock returns body with the backlink block's interior removed, so two
// bodies can be compared on every byte the repoint must not touch.
func outsideBlock(t *testing.T, body string) string {
	t.Helper()
	start := strings.Index(body, "-->")
	end := strings.Index(body, "<!-- docket:backlink:end")
	if start < 0 || end < 0 || end < start {
		t.Fatalf("no backlink block in %q", body)
	}
	return body[:start] + body[end:]
}

func TestPlanPRBacklinkRepointRepointsActiveLink(t *testing.T) {
	body := prBodyWithActiveBacklink(tActive, "Authored PR prose.")
	updated, current, present, needs, err := planPRBacklinkRepoint([]byte(body), tArchive, tInterior)
	if err != nil || !present || !needs {
		t.Fatalf("present=%v needs=%v err=%v, want a repoint", present, needs, err)
	}
	if !strings.Contains(current, tActive) {
		t.Fatalf("current interior = %q, want the active link", current)
	}
	got := string(updated)
	if !strings.Contains(got, tArchive) || strings.Contains(got, tActive) {
		t.Fatalf("updated body not repointed:\n%s", got)
	}
	if outsideBlock(t, got) != outsideBlock(t, body) {
		t.Fatalf("bytes outside the backlink block changed:\nbefore %q\nafter  %q", body, got)
	}
}

func TestPlanPRBacklinkRepointAlreadyArchivedIsNoWork(t *testing.T) {
	// Path-keyed: a block that already names the archive path is a no-op even
	// when its title wording differs from today's render.
	body := "<!-- docket:backlink:start (generated — do not hand-edit) -->\n" +
		"> ↩ **Change 0005 — An OLDER title** — `" + tArchive + "`\n" +
		"<!-- docket:backlink:end -->\n\nprose\n"
	_, _, present, needs, err := planPRBacklinkRepoint([]byte(body), tArchive, tInterior)
	if err != nil || !present || needs {
		t.Fatalf("present=%v needs=%v err=%v, want present no-work", present, needs, err)
	}
}

func TestPlanPRBacklinkRepointNoBlockIsNeverGivenOne(t *testing.T) {
	body := "A hand-written PR description with no docket block.\n"
	updated, _, present, needs, err := planPRBacklinkRepoint([]byte(body), tArchive, tInterior)
	if err != nil || present || needs {
		t.Fatalf("present=%v needs=%v err=%v, want absent no-work", present, needs, err)
	}
	if string(updated) != body {
		t.Fatalf("a block-less body was rewritten")
	}
}

func TestPlanPRBacklinkRepointPreservesCRLFProse(t *testing.T) {
	body := strings.ReplaceAll(prBodyWithActiveBacklink(tActive, "Line one.\nLine two."), "\n", "\r\n")
	updated, _, _, needs, err := planPRBacklinkRepoint([]byte(body), tArchive, tInterior)
	if err != nil || !needs {
		t.Fatalf("needs=%v err=%v, want a repoint of a CRLF body", needs, err)
	}
	if outsideBlock(t, string(updated)) != outsideBlock(t, body) {
		t.Fatalf("CRLF bytes outside the block changed:\nbefore %q\nafter  %q", body, string(updated))
	}
}

func TestPlanPRBacklinkRepointMalformedMarkersIsError(t *testing.T) {
	body := "<!-- docket:backlink:start (generated — do not hand-edit) -->\n> ↩ dangling\n\nprose\n"
	if _, _, _, _, err := planPRBacklinkRepoint([]byte(body), tArchive, tInterior); err == nil {
		t.Fatal("a dangling backlink marker must be an error (unknown), never a clean no-op")
	}
}

func TestRepointPRBacklinkEditsOnceThenIsIdempotent(t *testing.T) {
	ed := newFakePRBody(map[int]string{7: prBodyWithActiveBacklink(tActive, "prose")})
	r := repointPRBacklink(context.Background(), ed, githubcli.Repository{}, 7, tArchive, tInterior)
	if r.outcome != prBacklinkRepointed || ed.edits != 1 {
		t.Fatalf("first = %+v edits=%d, want repointed once", r, ed.edits)
	}
	r = repointPRBacklink(context.Background(), ed, githubcli.Repository{}, 7, tArchive, tInterior)
	if r.outcome != prBacklinkAlready || ed.edits != 1 {
		t.Fatalf("second = %+v edits=%d, want already with no new edit", r, ed.edits)
	}
	if prBacklinkFinding(5, 7, r) != nil {
		t.Fatal("an already-correct PR must produce no finding")
	}
}

func TestRepointPRBacklinkMalformedNeverEdits(t *testing.T) {
	ed := newFakePRBody(map[int]string{7: "<!-- docket:backlink:start (generated — do not hand-edit) -->\nbad\n"})
	r := repointPRBacklink(context.Background(), ed, githubcli.Repository{}, 7, tArchive, tInterior)
	if r.outcome != prBacklinkUnknown || ed.edits != 0 {
		t.Fatalf("malformed = %+v edits=%d, want unknown with no edit", r, ed.edits)
	}
	if f := prBacklinkFinding(5, 7, r); f == nil || f.Code != ReasonPRBacklinkPending {
		t.Fatalf("malformed must surface a %s finding, got %+v", ReasonPRBacklinkPending, f)
	}
}

func TestRepointPRBacklinkFailuresArePendingFindings(t *testing.T) {
	for name, ed := range map[string]*fakePRBody{
		"view-error": {bodies: map[int]string{}, viewErr: errors.New("gh: network down")},
		"edit-error": {bodies: map[int]string{7: prBodyWithActiveBacklink(tActive, "p")}, editErr: errors.New("gh: 502")},
		"contended":  {bodies: map[int]string{7: prBodyWithActiveBacklink(tActive, "p")}, editOut: githubcli.BodyContended},
	} {
		t.Run(name, func(t *testing.T) {
			r := repointPRBacklink(context.Background(), ed, githubcli.Repository{}, 7, tArchive, tInterior)
			f := prBacklinkFinding(5, 7, r)
			if f == nil || f.Code != ReasonPRBacklinkPending || !strings.Contains(f.Message, "#7") {
				t.Fatalf("%s: finding = %+v, want a %s finding naming PR #7", name, f, ReasonPRBacklinkPending)
			}
			if strings.Contains(f.Message, "prose") || strings.Contains(f.Message, "Authored") {
				t.Fatalf("%s: the finding leaked PR body bytes: %q", name, f.Message)
			}
		})
	}
}

func TestRepointPRBacklinkNoBlockNoEditNoFinding(t *testing.T) {
	ed := newFakePRBody(map[int]string{7: "hand-written\n"})
	r := repointPRBacklink(context.Background(), ed, githubcli.Repository{}, 7, tArchive, tInterior)
	if r.outcome != prBacklinkNoBlock || ed.edits != 0 || prBacklinkFinding(5, 7, r) != nil {
		t.Fatalf("no-block = %+v edits=%d, want no edit and no finding", r, ed.edits)
	}
}

func TestPRBodyEditorFallsBackToTheGitHubClient(t *testing.T) {
	if prBodyEditor(FinalizeDeps{GitHub: &githubcli.Client{}}) == nil {
		t.Fatal("a *githubcli.Client in deps.GitHub must serve as the PR-body editor (production wiring)")
	}
	if prBodyEditor(FinalizeDeps{}) != nil {
		t.Fatal("no editor is wired when neither PRBody nor a capable GitHub client is present")
	}
	ed := newFakePRBody(nil)
	if prBodyEditor(FinalizeDeps{PRBody: ed}) != FinalizePRBody(ed) {
		t.Fatal("an explicit PRBody seam wins")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test -count=1 -run 'TestPlanPRBacklinkRepoint|TestRepointPRBacklink|TestPRBodyEditor' ./internal/app/`
Expected: FAIL to compile, `planPRBacklinkRepoint undefined` (and the related names).

- [ ] **Step 3: Add the `FinalizeDeps.PRBody` field**

In `internal/app/finalize_context.go`, inside `type FinalizeDeps struct`, after `PRBatch SweepPRBatchReader`:

```go
	// PRBody is the narrow GitHub seam close-out and cleanup repoint a merged
	// PR's docket:backlink block through (pr_backlink_repoint.go). Production
	// leaves it nil: prBodyEditor falls back to GitHub when that client satisfies
	// FinalizePRBody (the real *githubcli.Client does). A test injects a fake.
	PRBody FinalizePRBody
```

- [ ] **Step 4: Implement `pr_backlink_repoint.go`**

```go
package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/githubcli"
)

// This file is the one place that decides whether a merged pull request's
// docket:backlink block must be repointed at the change's archived record, and
// the one edit sequence that does it. Close-out (runCloseoutPRBacklinkLeg),
// cleanup (finalizeCleanupPRBacklinkRepair), the sweep assessment
// (sweepAssessPRBacklinkLeg), and `repository repair --pr-backlinks` all call
// it; none keeps a copy of the predicate.
//
// The decision is path-keyed: a block whose interior already names the archive
// path is a no-op whatever its title wording, and a body with no block is never
// given one. Only the docket-owned block is replaced, through the loss-preserving
// document patch path, so every authored byte (CRLF endings and the build-
// evidence block included) survives. The leg is best-effort: a failure is a
// pr-backlink-pending warning, never a stop. PR body bytes never reach a finding.

// ReasonPRBacklinkPending: a merged PR's backlink block could not be repointed at
// the archived record (a read, edit, or marker problem, or a lost race); the
// change stays truthfully done and cleanup / the sweep retry it.
const ReasonPRBacklinkPending = "pr-backlink-pending"

// The closed outcomes of one repoint attempt.
const (
	prBacklinkRepointed = "repointed" // the block now names the archive path
	prBacklinkAlready   = "already"   // the block already named it; no edit
	prBacklinkNoBlock   = "no-block"  // a hand-written body; never given a block
	prBacklinkContended = "contended" // the body moved under us; not overwritten
	prBacklinkUnknown   = "unknown"   // unreadable, malformed, or unverified
)

// PRBodyEditor is the one GitHub write the repoint needs.
type PRBodyEditor interface {
	EditPullRequestBody(ctx context.Context, repo githubcli.Repository, number int, expectedRevision, body string) (githubcli.BodyEditOutcome, githubcli.PullRequest, error)
}

// FinalizePRBody is the read+write seam close-out and cleanup use.
type FinalizePRBody interface {
	ViewPullRequest(ctx context.Context, repo githubcli.Repository, number int) (githubcli.PullRequest, error)
	PRBodyEditor
}

// The production client satisfies the seam, so prBodyEditor's fallback is live.
var _ FinalizePRBody = (*githubcli.Client)(nil)

// prBodyEditor returns the injected PRBody seam, else deps.GitHub when it
// satisfies FinalizePRBody (production: the real client), else nil (a test
// fake GitHub that cannot edit bodies — the leg is then not run).
func prBodyEditor(deps FinalizeDeps) FinalizePRBody {
	if deps.PRBody != nil {
		return deps.PRBody
	}
	if ed, ok := deps.GitHub.(FinalizePRBody); ok {
		return ed
	}
	return nil
}

// prBacklinkResult is one attempt's outcome, the block interior it found
// (generated text only), and a redacted diagnostic.
type prBacklinkResult struct {
	outcome string
	current string
	detail  string
}

// planPRBacklinkRepoint is the pure decision over one PR body. present reports
// whether a docket:backlink block exists; needs reports whether its interior
// fails to name archivePath; updated is the body with only that block's interior
// replaced by archivedInterior (the input bytes unchanged when !needs). A parse
// or patch failure — malformed, unbalanced, or nested markers — is an error.
func planPRBacklinkRepoint(body []byte, archivePath, archivedInterior string) (updated []byte, current string, present, needs bool, err error) {
	doc, err := document.Parse(body)
	if err != nil {
		return nil, "", false, false, err
	}
	blk, ok := doc.Block(backlinkBlockName)
	if !ok {
		return body, "", false, false, nil
	}
	current = strings.TrimRight(string(body[blk.Interior.Start:blk.Interior.End]), "\r\n")
	if strings.Contains(current, archivePath) {
		return body, current, true, false, nil
	}
	var ps document.PatchSet
	ps.ReplaceBlock(backlinkBlockName, archivedInterior)
	updated, err = doc.Apply(ps)
	if err != nil {
		return nil, current, true, false, err
	}
	return updated, current, true, true, nil
}

// repointPRBacklink reads one PR by number and repoints its block.
func repointPRBacklink(ctx context.Context, ed FinalizePRBody, repo githubcli.Repository, number int, archivePath, archivedInterior string) prBacklinkResult {
	pr, err := ed.ViewPullRequest(ctx, repo, number)
	if err != nil {
		return prBacklinkResult{outcome: prBacklinkUnknown, detail: "the pull-request body could not be read: " + err.Error()}
	}
	return applyPRBacklinkRepoint(ctx, ed, repo, number, pr.Body, pr.Revision, archivePath, archivedInterior)
}

// applyPRBacklinkRepoint repoints a body already in hand (the repair reads it in
// a batch), editing only under the revision that body was read at.
func applyPRBacklinkRepoint(ctx context.Context, ed PRBodyEditor, repo githubcli.Repository, number int, body, revision, archivePath, archivedInterior string) prBacklinkResult {
	updated, current, present, needs, err := planPRBacklinkRepoint([]byte(body), archivePath, archivedInterior)
	if err != nil {
		return prBacklinkResult{outcome: prBacklinkUnknown, detail: "the pull-request body carries a malformed docket:backlink block"}
	}
	if !present {
		return prBacklinkResult{outcome: prBacklinkNoBlock}
	}
	if !needs {
		return prBacklinkResult{outcome: prBacklinkAlready, current: current}
	}
	out, _, err := ed.EditPullRequestBody(ctx, repo, number, revision, string(updated))
	switch {
	case err != nil:
		return prBacklinkResult{outcome: prBacklinkUnknown, current: current, detail: "the edit could not be verified: " + err.Error()}
	case out == githubcli.BodyEdited || out == githubcli.BodyAlready:
		return prBacklinkResult{outcome: prBacklinkRepointed, current: current}
	case out == githubcli.BodyContended:
		return prBacklinkResult{outcome: prBacklinkContended, current: current, detail: "the body changed since it was read"}
	default:
		return prBacklinkResult{outcome: prBacklinkUnknown, current: current, detail: "the edit outcome is unknown"}
	}
}

// prBacklinkFinding maps an attempt to a retryable warning, or nil when the PR is
// in its promised state (repointed, already, or a block-less body).
func prBacklinkFinding(id, number int, r prBacklinkResult) *StatusFinding {
	switch r.outcome {
	case prBacklinkRepointed, prBacklinkAlready, prBacklinkNoBlock:
		return nil
	}
	msg := fmt.Sprintf("change %04d is done, but PR #%d's change backlink was not repointed to the archived record (%s)", id, number, r.outcome)
	if r.detail != "" {
		msg += ": " + r.detail
	}
	msg += "; cleanup and the maintenance sweep will retry it"
	return &StatusFinding{Code: ReasonPRBacklinkPending, Severity: string(domain.SeverityWarning), Message: msg}
}
```

Verify by grep that `Document.Apply` returns `([]byte, error)` (it does in `assemblePRBody` and `backlinkLegRetarget`), and that `Block.Interior` spans source bytes, so slicing `body` is correct.

- [ ] **Step 5: Run the tests to verify they pass, then mutation-test the path key**

Run: `go test -count=1 -run 'TestPlanPRBacklinkRepoint|TestRepointPRBacklink|TestPRBodyEditor' ./internal/app/`
Expected: PASS.

Mutation: back up `pr_backlink_repoint.go`, change `if strings.Contains(current, archivePath)` to `if false`, rerun. Expected: `TestPlanPRBacklinkRepointAlreadyArchivedIsNoWork` and `TestRepointPRBacklinkEditsOnceThenIsIdempotent` FAIL. Restore from the backup (`mv -f`) and rerun to PASS. Then run `go test -count=1 ./internal/app/`. The package's literal-code and anchor guards must stay green.

- [ ] **Step 6: Commit**

```bash
git add internal/app/pr_backlink_repoint.go internal/app/pr_backlink_repoint_test.go internal/app/finalize_context.go
git commit -m "feat(app): one shared decision and edit sequence for repointing a PR's change backlink"
```

---

### Task 3: Close-out repoints every archived target's PR backlink

**Files:**
- Modify: `internal/app/finalize_closeout.go` (`closeoutIntegrationDestination`, new `runCloseoutPRBacklinkLeg`, file header comment)
- Modify: `internal/cli/finalize.go` (`finalize.closeout` annotation)
- Test: `internal/app/finalize_closeout_integration_test.go`, `internal/cli/finalize_test.go`

**Interfaces:**
- Consumes: `prBodyEditor`, `repointPRBacklink`, `prBacklinkFinding`, `ReasonPRBacklinkPending`, `fakePRBody`, `newFakePRBody` (Task 2); `archivedBacklinkInterior`, `closeoutTarget`, `closeoutContext` (`cc.snap`, `cc.sources`, `cc.eff`, `cc.link`), `parsePRNumber`, `finalizeHasPRRef`; fixtures `setupCloseoutFixture`, `planRepoModes`, `mergeIntoBase`, `baselineMergedFake`, `closeoutDeps`, `groomPath`, `artifactWithBacklink`, `closeoutPR` (`finalize_closeout_test.go`).
- Produces: `func runCloseoutPRBacklinkLeg(ctx context.Context, deps FinalizeDeps, cc *closeoutContext, ghRepo githubcli.Repository, targets []closeoutTarget) []StatusFinding`

- [ ] **Step 1: Write the failing integration tests**

Append to `internal/app/finalize_closeout_integration_test.go`:

```go
// TestIntegrationFinalizeCloseoutPRBacklinkRepointed proves close-out repoints
// the merged PR's backlink block at the archive path, keeps every authored byte,
// and is idempotent on replay.
func TestIntegrationFinalizeCloseoutPRBacklinkRepointed(t *testing.T) {
	requireRealGit(t)
	f := setupCloseoutFixture(t, planRepoModes()[0])
	mergeCommit := f.mergeIntoBase(t)
	recPath := groomPath(f.id, f.slug)
	before := artifactWithBacklink(recPath, "Summary", "Authored PR prose.")
	pr := newFakePRBody(map[int]string{closeoutPR: before})
	deps := f.closeoutDeps(f.baselineMergedFake(f.head, mergeCommit))
	deps.PRBody = pr

	res := FinalizeCloseout(context.Background(), deps, f.repo.invocation, f.id, CloseoutNotes{})
	if res.Result != ResultApplied || res.Disposition != CloseoutDispDoneArchived {
		t.Fatalf("closeout = %q disp %q (%s)", res.Result, res.Disposition, res.Message)
	}
	for _, fd := range res.Findings {
		if fd.Code == ReasonPRBacklinkPending {
			t.Fatalf("the PR-body leg did not land: %+v", fd)
		}
	}
	after := pr.bodies[closeoutPR]
	if !strings.Contains(after, res.ArchivePath) || strings.Contains(after, "`"+recPath+"`") {
		t.Fatalf("PR body not repointed to %q:\n%s", res.ArchivePath, after)
	}
	if !strings.HasSuffix(after, "# Summary\n\nAuthored PR prose.\n") {
		t.Fatalf("authored PR bytes changed:\n%s", after)
	}
	if pr.edits != 1 {
		t.Fatalf("edits = %d, want 1", pr.edits)
	}

	// Replay: the promised state already holds; no further edit.
	again := FinalizeCloseout(context.Background(), deps, f.repo.invocation, f.id, CloseoutNotes{})
	if again.Disposition != CloseoutDispAlready || pr.edits != 1 {
		t.Fatalf("replay = disp %q edits=%d, want already with no new edit", again.Disposition, pr.edits)
	}
}

// TestIntegrationFinalizeCloseoutPRBacklinkFailureIsPendingOnly proves a failed
// GitHub edit never changes close-out's disposition: the change is done and
// archived, with one pr-backlink-pending warning.
func TestIntegrationFinalizeCloseoutPRBacklinkFailureIsPendingOnly(t *testing.T) {
	requireRealGit(t)
	f := setupCloseoutFixture(t, planRepoModes()[0])
	mergeCommit := f.mergeIntoBase(t)
	pr := newFakePRBody(map[int]string{closeoutPR: artifactWithBacklink(groomPath(f.id, f.slug), "Summary", "p")})
	pr.editErr = errors.New("gh: HTTP 502")
	deps := f.closeoutDeps(f.baselineMergedFake(f.head, mergeCommit))
	deps.PRBody = pr

	res := FinalizeCloseout(context.Background(), deps, f.repo.invocation, f.id, CloseoutNotes{})
	if res.Result != ResultApplied || res.Disposition != CloseoutDispDoneArchived {
		t.Fatalf("a failed PR edit changed close-out: %q disp %q", res.Result, res.Disposition)
	}
	n := 0
	for _, fd := range res.Findings {
		if fd.Code == ReasonPRBacklinkPending {
			n++
			if fd.Severity != string(domain.SeverityWarning) {
				t.Errorf("finding severity = %q, want warning", fd.Severity)
			}
		}
	}
	if n != 1 {
		t.Fatalf("want exactly one %s finding, got %d: %+v", ReasonPRBacklinkPending, n, res.Findings)
	}
}

// TestIntegrationFinalizeCloseoutPRBacklinkNoBlockUntouched proves a
// hand-written PR body is never given a block and produces no finding.
func TestIntegrationFinalizeCloseoutPRBacklinkNoBlockUntouched(t *testing.T) {
	requireRealGit(t)
	f := setupCloseoutFixture(t, planRepoModes()[0])
	mergeCommit := f.mergeIntoBase(t)
	pr := newFakePRBody(map[int]string{closeoutPR: "Hand-written, no docket block.\n"})
	deps := f.closeoutDeps(f.baselineMergedFake(f.head, mergeCommit))
	deps.PRBody = pr

	res := FinalizeCloseout(context.Background(), deps, f.repo.invocation, f.id, CloseoutNotes{})
	if res.Disposition != CloseoutDispDoneArchived || pr.edits != 0 {
		t.Fatalf("disp %q edits=%d, want done-archived with no edit", res.Disposition, pr.edits)
	}
	if pr.bodies[closeoutPR] != "Hand-written, no docket block.\n" {
		t.Fatalf("a block-less body was changed")
	}
}
```

Add `"github.com/danielhanold/docket/internal/domain"` to that file's imports if it is not already there (`errors` is). In `TestIntegrationFinalizeArchiveRootCarry`, add a subtest after `all-proven-archives-root-and-descendants`:

```go
	t.Run("repoints-each-carried-pr-backlink", func(t *testing.T) {
		f := seed(t, "stacked-merged")
		childMerge := f.carryOntoRootFeature(t, map[string]string{"gadget.txt": "gadget work\n"})
		mergeCommit := f.mergeIntoBase(t)
		f.fetchAllIntoInvocation(t)
		pr := newFakePRBody(map[int]string{
			closeoutPR: artifactWithBacklink(groomPath(5, "widget"), "Root", "root prose"),
			8:          artifactWithBacklink(groomPath(6, "gadget"), "Child", "child prose"),
		})
		deps := f.closeoutDeps(rootFake(f, mergeCommit, childMerge))
		deps.PRBody = pr
		res := FinalizeCloseout(context.Background(), deps, f.repo.invocation, f.id, CloseoutNotes{})
		assertBothArchived(t, f, res)
		if got := pr.bodies[closeoutPR]; !strings.Contains(got, "docs/changes/archive/2026-08-18-0005-widget.md") {
			t.Errorf("root PR not repointed to its own archive path:\n%s", got)
		}
		if got := pr.bodies[8]; !strings.Contains(got, "docs/changes/archive/2026-08-18-0006-gadget.md") || strings.Contains(got, "0005-widget") {
			t.Errorf("descendant PR not repointed to ITS OWN archive path:\n%s", got)
		}
	})
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test -tags integration -count=1 -run '^TestIntegrationFinalizeCloseoutPRBacklink' ./internal/app/` and `go test -tags integration -count=1 -run '^TestIntegrationFinalizeArchiveRootCarry' ./internal/app/`
Expected: FAIL. The bodies are unchanged and `edits = 0`, because close-out does not call the leg yet.

- [ ] **Step 3: Implement the leg**

In `internal/app/finalize_closeout.go`, in `closeoutIntegrationDestination`, directly after the existing `runCloseoutBacklinkLeg` call and before `return res`:

```go
	// Follow-up: repoint each archived target's merged PR backlink block at its
	// archive path (a GitHub edit, best-effort). A failed edit leaves the change
	// truthfully done and emits a retryable pr-backlink-pending warning.
	res.Findings = append(res.Findings, runCloseoutPRBacklinkLeg(ctx, deps, cc, ghRepo, targets)...)
```

Add the function after `runCloseoutBacklinkLeg`:

```go
// runCloseoutPRBacklinkLeg repoints, for every archived target (the root and
// each carried descendant), its own merged PR's docket:backlink block at its own
// archive path. It is independent of the integration-ref backlink leg and never
// changes the close-out disposition: each target that did not reach its promised
// state contributes one pr-backlink-pending warning. No editor wired (a test
// GitHub fake) runs nothing.
func runCloseoutPRBacklinkLeg(ctx context.Context, deps FinalizeDeps, cc *closeoutContext, ghRepo githubcli.Repository, targets []closeoutTarget) []StatusFinding {
	ed := prBodyEditor(deps)
	if ed == nil {
		return nil
	}
	var out []StatusFinding
	for _, tg := range targets {
		c, found := cc.snap.Change(domain.ChangeID(tg.id))
		if found != domain.LookupFound {
			continue
		}
		number, ok := parsePRNumber(c.PR().Value)
		if !finalizeHasPRRef(c) || !ok {
			continue
		}
		src, ok := cc.sources[tg.activePath]
		if !ok {
			out = append(out, *prBacklinkFinding(tg.id, number, prBacklinkResult{outcome: prBacklinkUnknown, detail: "the record's source bytes were not in hand"}))
			continue
		}
		interior, err := archivedBacklinkInterior(cc.eff, tg.archivePath, src, cc.link)
		if err != nil {
			out = append(out, *prBacklinkFinding(tg.id, number, prBacklinkResult{outcome: prBacklinkUnknown, detail: "the archived backlink could not be rendered"}))
			continue
		}
		if f := prBacklinkFinding(tg.id, number, repointPRBacklink(ctx, ed, ghRepo, number, tg.archivePath, interior)); f != nil {
			out = append(out, *f)
		}
	}
	return out
}
```

Update the file header's "Backlinks across the one topology" paragraph by appending one sentence: "A third, GitHub-side leg repoints each archived target's merged PR description block at its archive path (runCloseoutPRBacklinkLeg); a failed edit leaves the change truthfully `done` with a pr-backlink-pending warning that cleanup and the sweep retry."

In `internal/cli/finalize.go`, change the `finalize.closeout` annotation and its comment:

```go
		// metadata-write: archive relocation + backlink retarget are
		// metadata-branch transactions. external-write: the merged PR's
		// description backlink is repointed at the archived record (a GitHub
		// edit). It pushes no feature ref.
		Annotations: capability("finalize.closeout", EffectExternalWrite, EffectMetadataWrite),
```

and in `TestFinalizeCloseoutRegistered` (`internal/cli/finalize_test.go`) append:

```go
	if got := cmd.Annotations[capAnnotationEffects]; got != "external-write metadata-write" {
		t.Errorf("finalize closeout effects = %q, want %q", got, "external-write metadata-write")
	}
```

- [ ] **Step 4: Run the tests to verify they pass, then mutation-test acceptance criterion 5**

Run: `go test -tags integration -count=1 -run '^TestIntegrationFinalizeCloseout' ./internal/app/`, `go test -tags integration -count=1 -run '^TestIntegrationFinalizeArchive' ./internal/app/`, `go test -count=1 ./internal/app/ ./internal/cli/`
Expected: PASS. The existing close-out tests stay green because their `fakeCloseoutGitHub` does not implement `EditPullRequestBody`, so no editor is wired for them.

Mutation (spec acceptance 5): back up `finalize_closeout.go`, delete the `res.Findings = append(res.Findings, runCloseoutPRBacklinkLeg(...)...)` line, rerun `-run '^TestIntegrationFinalizeCloseoutPRBacklink'`. Expected: `TestIntegrationFinalizeCloseoutPRBacklinkRepointed` FAIL. Restore from the backup with `mv -f` and rerun to PASS. Record the mutation and its red output in your report.

- [ ] **Step 5: Commit**

```bash
git add internal/app/finalize_closeout.go internal/app/finalize_closeout_integration_test.go internal/cli/finalize.go internal/cli/finalize_test.go
git commit -m "feat(finalize): close-out repoints each merged PR's change backlink at its archive path"
```

---

### Task 4: Cleanup retries the PR-backlink leg

**Files:**
- Modify: `internal/app/finalize_cleanup.go` (`finalizeCleanupDone`, new `finalizeCleanupPRBacklinkRepair`, header comment)
- Test: `internal/app/finalize_cleanup_integration_test.go`

**Interfaces:**
- Consumes: `prBodyEditor`, `repointPRBacklink`, `prBacklinkFinding`, `ReasonPRBacklinkPending`, `newFakePRBody` (Task 2); `archivedBacklinkInterior`, `cleanupWarning`; fixtures `setupCloseoutFixture`, `planRepoModeDocket`, `archiveClosed`, `mergedCleanupFake`, `cleanupDeps` (`finalize_cleanup_test.go`).
- Produces: `func finalizeCleanupPRBacklinkRepair(ctx context.Context, deps FinalizeDeps, cc *closeoutContext, ghRepo githubcli.Repository, number int) *StatusFinding`

- [ ] **Step 1: Write the failing test**

Append to `internal/app/finalize_cleanup_integration_test.go`:

```go
// TestIntegrationFinalizeCleanupPRBacklinkRetry proves cleanup is the retry for
// a PR backlink close-out left pending: a done change whose PR still names the
// active path is repointed; a failing edit makes the cleanup pending (retryable)
// without blocking the independent legs; a replay over a repointed PR issues no
// edit.
func TestIntegrationFinalizeCleanupPRBacklinkRetry(t *testing.T) {
	requireRealGit(t)

	t.Run("repoints-a-pending-pr", func(t *testing.T) {
		f := setupCloseoutFixture(t, planRepoModeDocket())
		head, mergeCommit := f.archiveClosed(t)
		deps := f.cleanupDeps(f.mergedCleanupFake(head, mergeCommit), f.deps.Client, f.svc)
		recPath := groomPath(f.id, f.slug)
		pr := newFakePRBody(map[int]string{closeoutPR: artifactWithBacklink(recPath, "Summary", "prose")})
		deps.PRBody = pr

		res := FinalizeCleanup(context.Background(), deps, f.repo.invocation, f.id)
		if res.Disposition != CleanupDispCleaned {
			t.Fatalf("cleanup = %q disp %q (%s) findings=%+v", res.Result, res.Disposition, res.Message, res.Findings)
		}
		if got := pr.bodies[closeoutPR]; strings.Contains(got, "`"+recPath+"`") || !strings.Contains(got, "docs/changes/archive/") {
			t.Fatalf("cleanup did not repoint the PR backlink:\n%s", got)
		}
		replay := FinalizeCleanup(context.Background(), deps, f.repo.invocation, f.id)
		if replay.Disposition == CleanupDispPending || pr.edits != 1 {
			t.Fatalf("replay disp %q edits=%d, want clean with no new edit", replay.Disposition, pr.edits)
		}
	})

	t.Run("failing-edit-is-pending-and-retryable", func(t *testing.T) {
		f := setupCloseoutFixture(t, planRepoModeDocket())
		head, mergeCommit := f.archiveClosed(t)
		deps := f.cleanupDeps(f.mergedCleanupFake(head, mergeCommit), f.deps.Client, f.svc)
		pr := newFakePRBody(map[int]string{closeoutPR: artifactWithBacklink(groomPath(f.id, f.slug), "Summary", "prose")})
		pr.editErr = errors.New("gh: HTTP 502")
		deps.PRBody = pr

		res := FinalizeCleanup(context.Background(), deps, f.repo.invocation, f.id)
		if res.Disposition != CleanupDispPending || res.Reason != ReasonPRBacklinkPending {
			t.Fatalf("cleanup = disp %q reason %q, want pending/%s", res.Disposition, res.Reason, ReasonPRBacklinkPending)
		}
		if f.remoteBranchPresent(t) {
			t.Fatalf("a PR-body failure must not block the independent ref legs")
		}
	})
}
```

Before relying on these, check three things in the existing helpers. First, that `archiveClosed` leaves the record `done` at its archive path while `cc.change.Path()` is that archive path. Second, that `remoteBranchPresent` reports the feature ref after a normal cleanup. Third, that `"errors"` and `"strings"` are imported in this file. Adjust the asserts to the helpers' real behavior; do not change the helpers.

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test -tags integration -count=1 -run '^TestIntegrationFinalizeCleanupPRBacklink' ./internal/app/`
Expected: FAIL. The body is unchanged in the first subtest, and the disposition is `cleaned` instead of `pending` in the second.

- [ ] **Step 3: Implement the leg**

In `finalizeCleanupDone` (`internal/app/finalize_cleanup.go`), directly after the Leg 1 block (`finalizeCleanupBacklinkRepair`):

```go
	// Leg 1b: repoint the merged PR's description backlink at the archived record
	// when close-out left it pending. Best-effort and independent: a failure is a
	// retryable pr-backlink-pending finding and never blocks the other legs.
	if f := finalizeCleanupPRBacklinkRepair(ctx, deps, cc, ghRepo, number); f != nil {
		findings = append(findings, *f)
	}
```

Add after `finalizeCleanupBacklinkRepair`:

```go
// finalizeCleanupPRBacklinkRepair re-runs the PR-body repoint idempotently for a
// done, archived change. In the normal flow (close-out already repointed it) it
// reads the PR and issues no edit; when close-out left the leg pending it lands
// the block-only edit. No editor wired runs nothing.
func finalizeCleanupPRBacklinkRepair(ctx context.Context, deps FinalizeDeps, cc *closeoutContext, ghRepo githubcli.Repository, number int) *StatusFinding {
	ed := prBodyEditor(deps)
	if ed == nil {
		return nil
	}
	id := int(cc.change.ID())
	interior, err := archivedBacklinkInterior(cc.eff, cc.change.Path(), cc.body, cc.link)
	if err != nil {
		f := cleanupWarning(ReasonPRBacklinkPending, "the archived backlink could not be rendered; retry cleanup")
		return &f
	}
	return prBacklinkFinding(id, number, repointPRBacklink(ctx, ed, ghRepo, number, cc.change.Path(), interior))
}
```

In the file header, change "repairs the final backlinks first when needed;" to "repairs the final backlinks (and the merged PR's description backlink) first when needed;".

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -tags integration -count=1 -run '^TestIntegrationFinalizeCleanup' ./internal/app/` and `go test -count=1 ./internal/app/`
Expected: PASS. The existing cleanup tests stay green because `fakeCleanupGitHub` cannot edit bodies, so no editor is wired.

Mutation: back up `finalize_cleanup.go`, delete the Leg 1b block, rerun `-run '^TestIntegrationFinalizeCleanupPRBacklink'`. Expected: FAIL. Restore with `mv -f` and rerun to PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/finalize_cleanup.go internal/app/finalize_cleanup_integration_test.go
git commit -m "feat(finalize): cleanup retries a pending PR backlink repoint"
```

---

### Task 5: The maintenance sweep assesses the PR-backlink leg

**Files:**
- Modify: `internal/app/sweep_prfacts.go` (`SweepPRSetResult.Bodies`, `ProbePRSet`)
- Modify: `internal/app/maintenance.go` (`gatherSweepSharedFacts`, `MaintenanceSweep`'s `assessHistorical` closure)
- Modify: `internal/app/maintenance_assess.go` (`sweepSharedFacts`, `sweepLegPRBacklink`, `sweepAssessPRBacklinkLeg`, the call in `sweepAssessHistorical`, comments)
- Test: `internal/app/maintenance_assess_test.go`, `internal/app/sweep_prfacts_test.go`

**Interfaces:**
- Consumes: `planPRBacklinkRepoint` (Task 2); `render.BacklinkContent`, `backlinkInterior`, `parsePRNumber`; `SweepPRBatchReader`, `fakeSweepBatchReader`, `countingSweepGitHub`, `sweepTestRepo` (`sweep_prfacts_test.go`); assessment fixtures `newAssessFixture`, `assessDoneBlob`, `assess`, `assessInterior`, `cleanWS`, `findEntry`, `actionableHas`, `assertEntry`, `prRefFor` (`maintenance_assess_test.go` and its neighbors).
- Produces:
  ```go
  // SweepPRSetResult gains:  Bodies map[int]string // Found slots only; never logged
  // sweepSharedFacts gains:  prBodiesGathered bool; prBodies map[int]string; prBodiesUnknown map[int]bool
  const sweepLegPRBacklink = "pr-backlink"
  func gatherSweepSharedFacts(ctx context.Context, deps FinalizeDeps, repoDir string, inv sweepInventory, historical []sweepWorkItem) sweepSharedFacts
  func sweepAssessPRBacklinkLeg(shared sweepSharedFacts, c domain.Change, link render.LinkContext, a *sweepLegAssessment)
  ```

The leg is assessed only when `prBodiesGathered` is true. Production always sets it whenever there is a done candidate. An orchestration test that builds `sweepSharedFacts{}` by hand keeps its existing behavior. Once bodies are gathered, a PR number missing from `prBodies` is unknown, never a clean no-op.

- [ ] **Step 1: Write the failing tests**

Append to `internal/app/maintenance_assess_test.go`:

```go
// prShared builds shared facts whose PR-body inventory was gathered, carrying
// the given bodies (and nothing unknown).
func prShared(bodies map[int]string) sweepSharedFacts {
	return sweepSharedFacts{
		remoteHeads:      map[gitcli.RefName]gitcli.ObjectID{},
		prBodiesGathered: true,
		prBodies:         bodies,
		prBodiesUnknown:  map[int]bool{},
	}
}

func prNumberOf(t *testing.T, f assessFixture, id int) int {
	t.Helper()
	c, _ := f.inv.snap.Change(domain.ChangeID(id))
	n, ok := parsePRNumber(c.PR().Value)
	if !ok {
		t.Fatalf("record %d carries no parsable PR", id)
	}
	return n
}

// TestAssessStalePRBacklinkIsActionable: a done record whose PR body still names
// the active path has work — it is dispatched to cleanup, which repoints it.
func TestAssessStalePRBacklinkIsActionable(t *testing.T) {
	f := newAssessFixture(t, []StatusBlob{assessDoneBlob(41, "archived", "")}, nil)
	n := prNumberOf(t, f, 41)
	shared := prShared(map[int]string{n: prBodyWithActiveBacklink("docs/changes/active/0041-archived.md", "prose")})
	_, actionable := f.assess(t, cleanWS(), shared, 41)
	if !actionableHas(actionable, 41) {
		t.Fatalf("a stale PR backlink must make the record actionable; actionable=%v", actionable)
	}
}

// TestAssessCorrectPRBacklinkIsNoWork: a PR body already naming the archive path
// adds no work.
func TestAssessCorrectPRBacklinkIsNoWork(t *testing.T) {
	f := newAssessFixture(t, []StatusBlob{assessDoneBlob(41, "archived", "")}, nil)
	n := prNumberOf(t, f, 41)
	body := "<!-- docket:backlink:start (generated — do not hand-edit) -->\n" + assessInterior(t, f, 41) + "\n<!-- docket:backlink:end -->\n\nprose\n"
	entries, actionable := f.assess(t, cleanWS(), prShared(map[int]string{n: body}), 41)
	if len(actionable) != 0 {
		t.Fatalf("an already-correct PR backlink must add no work; actionable=%v", actionable)
	}
	assertEntry(t, findEntry(entries, 41), SweepDispSkipped, ReasonSweepSnapshotNoWork)
}

// TestAssessUnreadPRBodyIsUnknownNeverNoWork: a gathered inventory missing this
// PR (a failed batch or a Found=false slot) is unknown, never certified no-work.
func TestAssessUnreadPRBodyIsUnknownNeverNoWork(t *testing.T) {
	f := newAssessFixture(t, []StatusBlob{assessDoneBlob(41, "archived", "")}, nil)
	n := prNumberOf(t, f, 41)
	shared := prShared(map[int]string{})
	shared.prBodiesUnknown[n] = true
	entries, actionable := f.assess(t, cleanWS(), shared, 41)
	if len(actionable) != 0 {
		t.Fatalf("an unread PR body must not dispatch; actionable=%v", actionable)
	}
	e := findEntry(entries, 41)
	assertEntry(t, e, SweepDispUnknown, ReasonSweepSnapshotUnknown)
	if !strings.Contains(e.Message, sweepLegPRBacklink) {
		t.Fatalf("the unknown leg must be named; msg=%q", e.Message)
	}
}

// TestAssessPRBacklinkMalformedIsUnknown: malformed backlink markers in the PR
// body are unknown, never no-work and never work.
func TestAssessPRBacklinkMalformedIsUnknown(t *testing.T) {
	f := newAssessFixture(t, []StatusBlob{assessDoneBlob(41, "archived", "")}, nil)
	n := prNumberOf(t, f, 41)
	shared := prShared(map[int]string{n: "<!-- docket:backlink:start (generated — do not hand-edit) -->\ndangling\n"})
	entries, actionable := f.assess(t, cleanWS(), shared, 41)
	if len(actionable) != 0 {
		t.Fatalf("malformed markers must not dispatch; actionable=%v", actionable)
	}
	assertEntry(t, findEntry(entries, 41), SweepDispUnknown, ReasonSweepSnapshotUnknown)
}
```

Append to `internal/app/sweep_prfacts_test.go`:

```go
// TestSweepPRSetCarriesBodiesForFoundSlotsOnly: ProbePRSet returns each Found
// slot's body and nothing for an unresolved slot.
func TestSweepPRSetCarriesBodiesForFoundSlotsOnly(t *testing.T) {
	gh := &countingSweepGitHub{repo: sweepTestRepo()}
	r := &sweepPRBatchReader{gh: gh}
	res := r.ProbePRSet(context.Background(), "repo", []int{1, 2})
	for n, f := range res.Facts {
		if _, ok := res.Bodies[n]; !ok {
			t.Errorf("found PR #%d (%+v) carries no body", n, f)
		}
	}
	for n := range res.Bodies {
		if _, ok := res.Facts[n]; !ok {
			t.Errorf("PR #%d has a body but no facts (an unresolved slot must carry neither)", n)
		}
	}
}
```

Read `countingSweepGitHub`'s `ViewPullRequestsBatch` first. If it answers every slot `Found: false` (the excerpt above does), give it a field that scripts some `Found: true` slots with a `PR.Body`, so the test can actually tell the two cases apart. A test where every slot is unresolved passes vacuously (learning `green-suite-untested-branch`).

Append a gatherer test to `internal/app/maintenance_assess_test.go`:

```go
// TestGatherSweepSharedFactsMarksUnreadPRBodiesUnknown: every done candidate's PR
// number lands in exactly one of prBodies or prBodiesUnknown, so no done record
// is silently unassessed; one batched read serves them all.
func TestGatherSweepSharedFactsMarksUnreadPRBodiesUnknown(t *testing.T) {
	f := newAssessFixture(t, []StatusBlob{assessDoneBlob(41, "a", ""), assessDoneBlob(42, "b", "")}, nil)
	n41, n42 := prNumberOf(t, f, 41), prNumberOf(t, f, 42)
	batch := &fakeSweepBatchReader{result: SweepPRSetResult{
		Facts:  map[int]domain.PRFacts{n41: {Number: strconv.Itoa(n41), State: "merged"}},
		Bodies: map[int]string{n41: "body"},
	}}
	deps := FinalizeDeps{Planning: PlanningDeps{Reader: f.reader, Clock: testClock()}, PRBatch: batch}
	items := []sweepWorkItem{{id: 41, kind: sweepKindCleanup}, {id: 42, kind: sweepKindCleanup}}
	shared := gatherSweepSharedFacts(context.Background(), deps, "repo", f.inv, items)
	if !shared.prBodiesGathered || batch.calls != 1 {
		t.Fatalf("gathered=%v calls=%d, want one batched read", shared.prBodiesGathered, batch.calls)
	}
	if shared.prBodies[n41] != "body" || shared.prBodiesUnknown[n41] {
		t.Errorf("PR #%d should be read", n41)
	}
	if _, ok := shared.prBodies[n42]; ok || !shared.prBodiesUnknown[n42] {
		t.Errorf("PR #%d was not returned, so it must be unknown", n42)
	}
}
```

Add `"strconv"` to the imports if needed. Check what `gatherSweepSharedFacts` does with a nil `Planning.Client`. The PR-body read must happen **before** that early return (Step 3), or this test sees nothing.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test -count=1 -run 'TestAssess|TestGatherSweepSharedFacts|TestSweepPRSet' ./internal/app/`
Expected: FAIL to compile (`prBodiesGathered`, `Bodies`, `sweepLegPRBacklink` undefined, and the new `gatherSweepSharedFacts` arity).

- [ ] **Step 3: Implement**

`internal/app/sweep_prfacts.go`: add a `Bodies` field to `SweepPRSetResult`:

```go
	// Bodies carries each Found slot's PR body, for the sweep's PR-backlink leg.
	// It is never logged or reported. A number absent here is unread (a failed
	// batch or an unresolved slot).
	Bodies map[int]string
```

In `ProbePRSet`, create `bodies := make(map[int]string)`, set `bodies[n] = br.PR.Body` next to `facts[n] = sweepBatchResultToFacts(n, br)`, and return `Bodies: bodies` on every return path, including the empty-request and resolution-failure paths.

`internal/app/maintenance_assess.go`: add to `sweepSharedFacts`:

```go
	// prBodiesGathered is set when the invocation read its done candidates' PR
	// bodies (one batched pass). When set, a number in neither prBodies nor
	// prBodiesUnknown is still unknown — the leg never reads absence as no-work.
	prBodiesGathered bool
	prBodies         map[int]string // PR number -> body (never logged)
	prBodiesUnknown  map[int]bool   // PR numbers the batched read could not resolve
```

Add the leg name `sweepLegPRBacklink = "pr-backlink"` to the leg-name const block. In `sweepAssessHistorical`, after `sweepAssessBacklinkLeg(ctx, deps, pin, c, link, &a)`, add `sweepAssessPRBacklinkLeg(shared, c, link, &a)`. Add the function after `sweepBacklinkArtifactPaths`:

```go
// sweepAssessPRBacklinkLeg resolves the PR-backlink leg from the shared, batched
// PR-body inventory: a body whose docket:backlink block does not name the
// archived record path has work (cleanup repoints it). A body already naming it,
// or with no block, is no-effect. An unread body or malformed markers is
// unresolved, never a clean no-op. Not gathered (an orchestration seam) is not
// assessed.
func sweepAssessPRBacklinkLeg(shared sweepSharedFacts, c domain.Change, link render.LinkContext, a *sweepLegAssessment) {
	if !shared.prBodiesGathered {
		return
	}
	n, ok := parsePRNumber(c.PR().Value)
	if !ok {
		return // identity already validated by the caller
	}
	body, read := shared.prBodies[n]
	if !read || shared.prBodiesUnknown[n] {
		a.markUnknown(sweepLegPRBacklink, fmt.Sprintf("PR #%d's body could not be read in the shared batch", n))
		return
	}
	block, err := render.BacklinkContent(c, link)
	if err != nil {
		a.markUnknown(sweepLegPRBacklink, "the record's archived backlink could not be rendered")
		return
	}
	_, _, _, needs, err := planPRBacklinkRepoint([]byte(body), c.Path(), backlinkInterior(block))
	if err != nil {
		a.markUnknown(sweepLegPRBacklink, fmt.Sprintf("PR #%d carries a malformed docket:backlink block", n))
		return
	}
	if needs {
		a.markWork()
	}
}
```

Add `"fmt"` to the imports. Update `ReasonSweepSnapshotNoWork`'s comment to read "…absent local/remote refs, and already-correct final and PR backlinks." Update the `sweepAssessHistorical` doc sentence "It dispatches no metadata, PR, or remote-ref read of its own: the remote/worktree inventories arrive in shared" to say "the remote/worktree inventories and the batched PR-body inventory arrive in shared".

`internal/app/maintenance.go`: change the signature to `gatherSweepSharedFacts(ctx context.Context, deps FinalizeDeps, repoDir string, inv sweepInventory, historical []sweepWorkItem) sweepSharedFacts`. Update its one caller in the `MaintenanceSweep` `assessHistorical` closure to pass `repoDir`. Inside, after the `if !needsShared { return sweepSharedFacts{} }` early return and **before** the git-client check, build the PR-body inventory into a local `shared`:

```go
	var shared sweepSharedFacts
	// One batched PR-body read over every done candidate's PR (≤25 per process)
	// for the PR-backlink leg. A number the read could not resolve is unknown.
	var numbers []int
	for _, it := range historical {
		c, out := inv.snap.Change(domain.ChangeID(it.id))
		if out != domain.LookupFound || c.Status() != domain.StatusDone {
			continue
		}
		if n, ok := parsePRNumber(c.PR().Value); ok {
			numbers = append(numbers, n)
		}
	}
	shared.prBodiesGathered = true
	shared.prBodies = map[int]string{}
	shared.prBodiesUnknown = map[int]bool{}
	if len(numbers) > 0 {
		var bodies map[int]string
		if deps.PRBatch != nil {
			bodies = deps.PRBatch.ProbePRSet(ctx, repoDir, numbers).Bodies
		}
		for _, n := range numbers {
			if b, ok := bodies[n]; ok {
				shared.prBodies[n] = b
			} else {
				shared.prBodiesUnknown[n] = true
			}
		}
	}
```

Then make the existing missing-client return carry these fields (`shared.remoteHeadsErr, shared.worktreesErr = err, err; return shared`) and have the rest of the function fill `shared` as it does today. Update the doc comment: "…one complete remote-heads advertisement, one worktree list, and one batched read of the done candidates' PR bodies."

- [ ] **Step 4: Run the tests to verify they pass, then mutation-test the unknown branch**

Run: `go test -count=1 ./internal/app/`
Expected: PASS. If an existing test counts `ProbePRSet` calls across a whole full-scope sweep and now sees one more, fix the expectation only when the extra call is this designed read, and say so in your report.

Mutation: back up `maintenance_assess.go`, change `if !read || shared.prBodiesUnknown[n]` to `if false`, rerun `-run 'TestAssessUnreadPRBodyIsUnknownNeverNoWork'`. Expected: FAIL (the record certifies as no-work). Restore with `mv -f` and rerun to PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/sweep_prfacts.go internal/app/sweep_prfacts_test.go internal/app/maintenance.go internal/app/maintenance_assess.go internal/app/maintenance_assess_test.go
git commit -m "feat(maintenance): the full sweep assesses each done record's PR backlink from one batched read"
```

---

### Task 6: `docket repository repair --pr-backlinks`

**Files:**
- Modify: `internal/app/repository_facts.go` (`SetupDeps.GitHub`)
- Modify: `internal/app/repository_repair.go` (`RepairOptions.PRBacklinks`, `RepositoryRepairResult.PRBacklinks`, extract `repairPreflight`, dispatch)
- Create: `internal/app/repository_repair_prbacklinks.go`
- Modify: `internal/cli/repository.go`, `internal/cli/repository_test.go`
- Test: `internal/app/repository_repair_integration_test.go`

**Interfaces:**
- Consumes: `PRBodyEditor`, `applyPRBacklinkRepoint`, `planPRBacklinkRepoint`, prBacklink outcome constants (Task 2); `githubcli.BatchPRResult`, `githubcli.BodyEditOutcome` (Task 1); `readCheckCorpus`, `buildCorpusSnapshot`, `checkCorpus.link`, `render.BacklinkContent`, `backlinkInterior`, `parsePRNumber`, `finalizeHasPRRef`, `sweepDedupeSortAsc`, `sweepChunkInts`, `sweepPRBatchCap`, `migrateSourceMoved`, `newRepairResult`, `repairExternalFailure`, `repairInternalFailure`, `repairContended`, `repairConfirmationRequiredState`; fixtures `newHealthyRepo`, `writeRepoFile`, `runGit`, `newGitClient`.
- Produces:
  ```go
  // SetupDeps gains: GitHub RepairGitHub  (nil on every command but `repair --pr-backlinks`)
  type RepairGitHub interface {
      DiscoverRepository(ctx context.Context, dir string) (githubcli.Repository, error)
      ViewPullRequestsBatch(ctx context.Context, repo githubcli.Repository, numbers []int) (map[int]githubcli.BatchPRResult, error)
      PRBodyEditor
  }
  // RepairOptions gains: PRBacklinks bool
  // RepositoryRepairResult gains: PRBacklinks []PRBacklinkRepair `json:"pr_backlinks,omitempty"`
  type PRBacklinkRepair struct {
      ID        int    `json:"id"`
      PR        int    `json:"pr"`
      Current   string `json:"current,omitempty"`
      Corrected string `json:"corrected,omitempty"`
      Outcome   string `json:"outcome"`
      Message   string `json:"message,omitempty"`
  }
  const (
      PRBacklinkRepairPlanned    = "planned"
      PRBacklinkRepairRepointed  = "repointed"
      PRBacklinkRepairContended  = "contended"
      PRBacklinkRepairUnreadable = "unreadable"
      PRBacklinkRepairFailed     = "failed"
  )
  func runPRBacklinkRepair(ctx context.Context, d SetupDeps, o RepairOptions) RepositoryRepairResult
  func repairPreflight(ctx context.Context, d SetupDeps) (setupContext, string, *RepositoryRepairResult)
  // cli: var repositoryRepairGitHub = func() (app.RepairGitHub, error)
  ```

`--pr-backlinks` selects the PR-backlink repair **only**. It does not also run the frontmatter or derived-view repairs; run plain `repository repair` for those. The candidate set is every `done` change in the pinned corpus that carries a PR reference. A candidate is planned when `planPRBacklinkRepoint` says its PR body needs work. A done record lives only at its archive path, so a block that does not name that path names a path that no longer exists, which is the spec's "dead" link.

- [ ] **Step 1: Write the failing integration tests**

Append to `internal/app/repository_repair_integration_test.go`:

```go
// fakeRepairGitHub is a scriptable RepairGitHub: bodies by PR number, a batch
// failure switch, per-number edit outcomes, and call counters.
type fakeRepairGitHub struct {
	bodies     map[int]string
	failBatch  bool
	editOut    map[int]githubcli.BodyEditOutcome
	batchCalls int
	edits      int
	t          *testing.T
	forbidAll  bool // any call fails the test (routine-commands guard)
}

func (f *fakeRepairGitHub) rev(n int) string { return "rev:" + strconv.Itoa(len(f.bodies[n])) + ":" + f.bodies[n][:min(8, len(f.bodies[n]))] }

func (f *fakeRepairGitHub) DiscoverRepository(context.Context, string) (githubcli.Repository, error) {
	if f.forbidAll {
		f.t.Errorf("DiscoverRepository called by a command that must not touch GitHub")
		return githubcli.Repository{}, errors.New("forbidden")
	}
	return githubcli.Repository{Host: "github.com", Owner: "acme", Name: "widget"}, nil
}

func (f *fakeRepairGitHub) ViewPullRequestsBatch(_ context.Context, _ githubcli.Repository, numbers []int) (map[int]githubcli.BatchPRResult, error) {
	f.batchCalls++
	if f.forbidAll {
		f.t.Errorf("ViewPullRequestsBatch called by a command that must not read PR bodies")
		return nil, errors.New("forbidden")
	}
	if f.failBatch {
		return nil, errors.New("gh api graphql failed")
	}
	out := map[int]githubcli.BatchPRResult{}
	for _, n := range numbers {
		b, ok := f.bodies[n]
		if !ok {
			out[n] = githubcli.BatchPRResult{Found: false}
			continue
		}
		out[n] = githubcli.BatchPRResult{Found: true, PR: githubcli.PullRequest{Number: n, State: githubcli.StateMerged, Body: b, Revision: f.rev(n)}}
	}
	return out, nil
}

func (f *fakeRepairGitHub) EditPullRequestBody(_ context.Context, _ githubcli.Repository, n int, rev, body string) (githubcli.BodyEditOutcome, githubcli.PullRequest, error) {
	f.edits++
	if f.forbidAll {
		f.t.Errorf("EditPullRequestBody called by a command that must not touch GitHub")
		return githubcli.BodyUnknown, githubcli.PullRequest{}, errors.New("forbidden")
	}
	if o, ok := f.editOut[n]; ok {
		return o, githubcli.PullRequest{}, nil
	}
	if rev != f.rev(n) {
		return githubcli.BodyContended, githubcli.PullRequest{}, nil
	}
	f.bodies[n] = body
	return githubcli.BodyEdited, githubcli.PullRequest{Number: n, Body: body}, nil
}

const (
	prRepairPath    = "docs/changes/archive/2026-08-29-0363-old-change.md"
	prRepairActive  = "docs/changes/active/0363-old-change.md"
	prRepairOKPath  = "docs/changes/archive/2026-08-30-0364-fixed-change.md"
)

// prArchivedRecord is a minimal valid archived done record carrying a PR URL.
func prArchivedRecord(id int, slug string, pr int) string {
	return "---\nid: " + strconv.Itoa(id) + "\nslug: " + slug + "\nstatus: done\ntitle: Change " + slug +
		"\ntype: feature\npr: 'https://github.com/acme/widget/pull/" + strconv.Itoa(pr) + "'\n---\n\nBody for " + slug + ".\n"
}

func prBodyNaming(path string) string {
	return "<!-- docket:backlink:start (generated — do not hand-edit) -->\n" +
		"> ↩ **Change — x** — `" + path + "`\n<!-- docket:backlink:end -->\n\nAuthored PR prose.\n"
}

// publishPRBacklinkRecords publishes one archived record whose PR still names
// the active path (dead) and one whose PR already names its archive path.
func (r *initRepo) publishPRBacklinkRecords(t *testing.T) {
	t.Helper()
	dotDocket := filepath.Join(r.invocation, ".docket")
	writeRepoFile(t, dotDocket, prRepairPath, prArchivedRecord(363, "old-change", 250))
	writeRepoFile(t, dotDocket, prRepairOKPath, prArchivedRecord(364, "fixed-change", 251))
	runGit(t, dotDocket, "add", "--", prRepairPath, prRepairOKPath)
	runGit(t, dotDocket, "commit", "-q", "-m", "publish archived PR-bearing records")
	runGit(t, dotDocket, "push", "-q", "origin", string(reposetup.MetadataBranchName))
}

func (r *initRepo) runPRRepair(t *testing.T, gh RepairGitHub, o RepairOptions) RepositoryRepairResult {
	t.Helper()
	o.PRBacklinks = true
	return RunRepositoryRepair(context.Background(), SetupDeps{Git: newGitClient(t), RepoDir: r.invocation, GitHub: gh}, o)
}

// TestIntegrationRepoRepairPRBacklinksPreviewApplyThenNothing covers spec
// acceptance 3: the preview lists exactly the dead-link PR, --yes fixes it, and
// a second run reports nothing to do.
func TestIntegrationRepoRepairPRBacklinksPreviewApplyThenNothing(t *testing.T) {
	r := newHealthyRepo(t)
	r.publishPRBacklinkRecords(t)
	gh := &fakeRepairGitHub{t: t, bodies: map[int]string{250: prBodyNaming(prRepairActive), 251: prBodyNaming(prRepairOKPath)}}

	preview := r.runPRRepair(t, gh, RepairOptions{})
	if !preview.ConfirmationRequired() || len(preview.PRBacklinks) != 1 {
		t.Fatalf("preview = %q state %q entries %+v, want one planned PR", preview.Result, preview.RepositoryState, preview.PRBacklinks)
	}
	e := preview.PRBacklinks[0]
	if e.ID != 363 || e.PR != 250 || e.Outcome != PRBacklinkRepairPlanned ||
		!strings.Contains(e.Current, prRepairActive) || !strings.Contains(e.Corrected, prRepairPath) {
		t.Fatalf("preview entry = %+v", e)
	}
	if gh.edits != 0 {
		t.Fatalf("a preview must edit nothing; edits=%d", gh.edits)
	}

	applied := r.runPRRepair(t, gh, RepairOptions{Authorized: true, ExpectedSource: preview.SourceRev()})
	if applied.Result != ResultApplied || len(applied.PRBacklinks) != 1 || applied.PRBacklinks[0].Outcome != PRBacklinkRepairRepointed {
		t.Fatalf("apply = %q %+v", applied.Result, applied.PRBacklinks)
	}
	if !strings.Contains(gh.bodies[250], prRepairPath) || !strings.HasSuffix(gh.bodies[250], "\n\nAuthored PR prose.\n") {
		t.Fatalf("PR #250 not repointed with prose intact:\n%s", gh.bodies[250])
	}
	if gh.bodies[251] != prBodyNaming(prRepairOKPath) {
		t.Fatalf("an already-correct PR was edited")
	}

	again := r.runPRRepair(t, gh, RepairOptions{})
	if again.Result != ResultNoOp || len(again.PRBacklinks) != 0 {
		t.Fatalf("second run = %q %+v, want nothing to do", again.Result, again.PRBacklinks)
	}
}

// TestIntegrationRepoRepairPRBacklinksSkipsUnreadable: an unreadable PR (a
// Found=false slot) and a malformed block are reported and skipped; a contended
// edit is reported; none aborts the batch.
func TestIntegrationRepoRepairPRBacklinksSkipsUnreadable(t *testing.T) {
	r := newHealthyRepo(t)
	r.publishPRBacklinkRecords(t)
	dotDocket := filepath.Join(r.invocation, ".docket")
	third := "docs/changes/archive/2026-08-31-0365-third-change.md"
	writeRepoFile(t, dotDocket, third, prArchivedRecord(365, "third-change", 252))
	runGit(t, dotDocket, "add", "--", third)
	runGit(t, dotDocket, "commit", "-q", "-m", "third")
	runGit(t, dotDocket, "push", "-q", "origin", string(reposetup.MetadataBranchName))

	gh := &fakeRepairGitHub{t: t, bodies: map[int]string{
		250: prBodyNaming(prRepairActive),
		252: "<!-- docket:backlink:start (generated — do not hand-edit) -->\ndangling\n",
	}} // 251 absent: Found=false
	preview := r.runPRRepair(t, gh, RepairOptions{})
	byPR := map[int]PRBacklinkRepair{}
	for _, e := range preview.PRBacklinks {
		byPR[e.PR] = e
	}
	if byPR[250].Outcome != PRBacklinkRepairPlanned || byPR[251].Outcome != PRBacklinkRepairUnreadable || byPR[252].Outcome != PRBacklinkRepairUnreadable {
		t.Fatalf("entries = %+v", preview.PRBacklinks)
	}

	gh.editOut = map[int]githubcli.BodyEditOutcome{250: githubcli.BodyContended}
	applied := r.runPRRepair(t, gh, RepairOptions{Authorized: true, ExpectedSource: preview.SourceRev()})
	for _, e := range applied.PRBacklinks {
		if e.PR == 250 && e.Outcome != PRBacklinkRepairContended {
			t.Fatalf("PR #250 outcome = %q, want contended", e.Outcome)
		}
	}
	if gh.edits != 1 {
		t.Fatalf("only the planned PR may be edited; edits=%d", gh.edits)
	}
}

// TestIntegrationRepoRepairRoutineCommandsReadNoPRBodies pins spec acceptance 4:
// with a GitHub fake that fails on ANY call wired into SetupDeps, routine
// `repository repair` (preview and --yes), `check`, and `prepare` never call it.
func TestIntegrationRepoRepairRoutineCommandsReadNoPRBodies(t *testing.T) {
	r := newHealthyRepo(t)
	r.publishPRBacklinkRecords(t)
	r.publishHealthyDrift(t)
	gh := &fakeRepairGitHub{t: t, forbidAll: true}
	d := SetupDeps{Git: newGitClient(t), RepoDir: r.invocation, GitHub: gh}

	_ = RunRepositoryCheck(context.Background(), d)
	_ = RunRepositoryPrepare(context.Background(), d, PrepareOptions{})
	preview := RunRepositoryRepair(context.Background(), d, RepairOptions{})
	_ = RunRepositoryRepair(context.Background(), d, RepairOptions{Authorized: true, ExpectedSource: preview.SourceRev()})
	if gh.batchCalls != 0 || gh.edits != 0 {
		t.Fatalf("routine commands touched GitHub: batch=%d edits=%d", gh.batchCalls, gh.edits)
	}
}
```

Add imports `errors` and `github.com/danielhanold/docket/internal/githubcli` (go 1.21+ provides the `min` builtin). Check `PrepareOptions`' zero value against `RunRepositoryPrepare`'s signature, and check that a done record with only `pr:` (no `branch:`) builds into the snapshot. If the snapshot builder rejects it, add the fields the builder requires to `prArchivedRecord`, using `repairArchivedRecord` and the validator as references.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test -tags integration -count=1 -run '^TestIntegrationRepoRepairPRBacklinks|^TestIntegrationRepoRepairRoutine' ./internal/app/`
Expected: FAIL to compile (`SetupDeps.GitHub`, `RepairOptions.PRBacklinks`, `PRBacklinkRepair` undefined).

- [ ] **Step 3: Implement the app side**

`internal/app/repository_facts.go`, in `SetupDeps`:

```go
	// GitHub is set ONLY by `repository repair --pr-backlinks` (the one
	// repository command that reads and edits merged PR bodies). Every other
	// repository command leaves it nil and never consults it.
	GitHub RepairGitHub
```

`internal/app/repository_repair.go`: add `PRBacklinks bool` to `RepairOptions` (comment: "selects the PR-backlink repair instead of the metadata repairs; requires SetupDeps.GitHub"). Add `PRBacklinks []PRBacklinkRepair \`json:"pr_backlinks,omitempty"\`` to `RepositoryRepairResult`. Move the gather + `migrateRoute` + phase switch + `metadataTip` check at the top of `RunRepositoryRepair`, unchanged, into:

```go
// repairPreflight runs the shared gather and topology routing every repair mode
// starts with and returns the setup context and the pinned metadata tip, or the
// refusal to return verbatim.
func repairPreflight(ctx context.Context, d SetupDeps) (setupContext, string, *RepositoryRepairResult)
```

`RunRepositoryRepair` then becomes:

```go
func RunRepositoryRepair(ctx context.Context, d SetupDeps, o RepairOptions) RepositoryRepairResult {
	if o.PRBacklinks {
		return runPRBacklinkRepair(ctx, d, o)
	}
	sc, metadataTip, refusal := repairPreflight(ctx, d)
	if refusal != nil {
		return *refusal
	}
	// … the existing corpus read / plan / preview / execute, unchanged …
}
```

Create `internal/app/repository_repair_prbacklinks.go`:

```go
package app

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/reposetup"
)

// repository_repair_prbacklinks.go — `docket repository repair --pr-backlinks`:
// the one-time, human-confirmed repair of merged PRs whose docket:backlink block
// still names a change path that no longer exists (the active path of a change
// that has since been archived). It reads the pinned metadata corpus, reads the
// done changes' PR bodies in batches (≤25 per gh process), previews each PR with
// its current and corrected link, and — only under --yes, pinned to the previewed
// metadata revision — edits each one through the shared applyPRBacklinkRepoint,
// one PR at a time. A PR that cannot be read or edited is reported and skipped;
// it never aborts the batch. A second run finds nothing.

// RepairGitHub is the GitHub seam this repair reads and edits merged PR bodies
// through. *githubcli.Client satisfies it.
type RepairGitHub interface {
	DiscoverRepository(ctx context.Context, dir string) (githubcli.Repository, error)
	ViewPullRequestsBatch(ctx context.Context, repo githubcli.Repository, numbers []int) (map[int]githubcli.BatchPRResult, error)
	PRBodyEditor
}

var _ RepairGitHub = (*githubcli.Client)(nil)

// PRBacklinkRepair is one PR's row in the preview or the applied report. Current
// and Corrected are the backlink block's generated interior line (never authored
// prose).
type PRBacklinkRepair struct {
	ID        int    `json:"id"`
	PR        int    `json:"pr"`
	Current   string `json:"current,omitempty"`
	Corrected string `json:"corrected,omitempty"`
	Outcome   string `json:"outcome"`
	Message   string `json:"message,omitempty"`
}

// The closed per-PR outcomes.
const (
	PRBacklinkRepairPlanned    = "planned"
	PRBacklinkRepairRepointed  = "repointed"
	PRBacklinkRepairContended  = "contended"
	PRBacklinkRepairUnreadable = "unreadable"
	PRBacklinkRepairFailed     = "failed"
)

// prRepairCandidate is one done change with a PR and the archive interior its
// PR's block must carry.
type prRepairCandidate struct {
	id       int
	pr       int
	path     string
	interior string
}

// prRepairPlanned is a planned row plus the bytes and revision the edit needs
// (kept out of the published row: body bytes are never reported).
type prRepairPlanned struct {
	row      PRBacklinkRepair
	body     string
	revision string
	path     string
	interior string
}

func runPRBacklinkRepair(ctx context.Context, d SetupDeps, o RepairOptions) RepositoryRepairResult {
	sc, metadataTip, refusal := repairPreflight(ctx, d)
	if refusal != nil {
		return *refusal
	}
	if d.GitHub == nil {
		return repairInternalFailure(reposetup.StateHealthy, "wiring the GitHub client",
			errors.New("no GitHub client is wired for --pr-backlinks"))
	}
	corpus, err := readCheckCorpus(ctx, d.Git, sc)
	if err != nil {
		return repairExternalFailure(reposetup.StateHealthy, "reading the metadata corpus", err)
	}
	snap, ok := buildCorpusSnapshot(sc.cfg, corpus.records)
	if !ok {
		return repairInternalFailure(reposetup.StateHealthy, "building the metadata snapshot",
			errors.New("the metadata corpus could not be built into a snapshot"))
	}
	cands, err := prRepairCandidates(snap, corpus.link)
	if err != nil {
		return repairInternalFailure(reposetup.StateHealthy, "rendering the archived backlinks", err)
	}
	if len(cands) == 0 {
		return prRepairNoOp(metadataTip, nil)
	}
	repo, err := d.GitHub.DiscoverRepository(ctx, sc.repo.PrimaryWorktree)
	if err != nil {
		return repairExternalFailure(reposetup.StateHealthy, "resolving the GitHub repository", err)
	}
	planned, skipped := planPRRepairs(ctx, d.GitHub, repo, cands)
	if len(planned) == 0 {
		return prRepairNoOp(metadataTip, skipped)
	}
	if !o.Authorized {
		return prRepairConfirmationRequired(metadataTip, planned, skipped)
	}
	if migrateSourceMoved(o.ExpectedSource, metadataTip) {
		return repairContended(metadataTip, o.ExpectedSource)
	}
	rows := append([]PRBacklinkRepair(nil), skipped...)
	repointed, failed := 0, 0
	for _, p := range planned {
		r := applyPRBacklinkRepoint(ctx, d.GitHub, repo, p.row.PR, p.body, p.revision, p.path, p.interior)
		row := p.row
		switch r.outcome {
		case prBacklinkRepointed, prBacklinkAlready:
			row.Outcome = PRBacklinkRepairRepointed
			repointed++
		case prBacklinkContended:
			row.Outcome, row.Message = PRBacklinkRepairContended, r.detail
			failed++
		default:
			row.Outcome, row.Message = PRBacklinkRepairFailed, r.detail
			failed++
		}
		rows = append(rows, row)
	}
	return prRepairApplied(metadataTip, rows, repointed, failed)
}

// prRepairCandidates lists every done change carrying a PR reference, sorted by
// id, with the archive interior its PR's block must carry.
func prRepairCandidates(snap domain.Snapshot, link render.LinkContext) ([]prRepairCandidate, error) {
	var out []prRepairCandidate
	for _, c := range snap.Changes() {
		if c.Status() != domain.StatusDone || !finalizeHasPRRef(c) {
			continue
		}
		n, ok := parsePRNumber(c.PR().Value)
		if !ok {
			continue
		}
		block, err := render.BacklinkContent(c, link)
		if err != nil {
			return nil, fmt.Errorf("change %04d: %w", int(c.ID()), err)
		}
		out = append(out, prRepairCandidate{id: int(c.ID()), pr: n, path: c.Path(), interior: backlinkInterior(block)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].id < out[j].id })
	return out, nil
}

// planPRRepairs reads the candidates' PR bodies in ≤25-number batches and keeps
// the ones whose block does not name the archive path. A failed batch, an
// unresolved slot, or malformed markers is an `unreadable` row, never a silent drop.
func planPRRepairs(ctx context.Context, gh RepairGitHub, repo githubcli.Repository, cands []prRepairCandidate) ([]prRepairPlanned, []PRBacklinkRepair) {
	byPR := map[int][]prRepairCandidate{}
	var numbers []int
	for _, c := range cands {
		byPR[c.pr] = append(byPR[c.pr], c)
		numbers = append(numbers, c.pr)
	}
	read := map[int]githubcli.BatchPRResult{}
	failMsg := map[int]string{}
	for _, chunk := range sweepChunkInts(sweepDedupeSortAsc(numbers), sweepPRBatchCap) {
		res, err := gh.ViewPullRequestsBatch(ctx, repo, chunk)
		for _, n := range chunk {
			switch {
			case err != nil:
				failMsg[n] = "the pull-request batch could not be read: " + err.Error()
			case !res[n].Found:
				failMsg[n] = "the pull request could not be resolved"
			default:
				read[n] = res[n]
			}
		}
	}
	var planned []prRepairPlanned
	var skipped []PRBacklinkRepair
	for _, c := range cands {
		br, ok := read[c.pr]
		if !ok {
			skipped = append(skipped, PRBacklinkRepair{ID: c.id, PR: c.pr, Outcome: PRBacklinkRepairUnreadable, Message: failMsg[c.pr]})
			continue
		}
		_, current, present, needs, err := planPRBacklinkRepoint([]byte(br.PR.Body), c.path, c.interior)
		if err != nil {
			skipped = append(skipped, PRBacklinkRepair{ID: c.id, PR: c.pr, Outcome: PRBacklinkRepairUnreadable,
				Message: "the pull-request body carries a malformed docket:backlink block"})
			continue
		}
		if !present || !needs {
			continue
		}
		planned = append(planned, prRepairPlanned{
			row:      PRBacklinkRepair{ID: c.id, PR: c.pr, Current: current, Corrected: c.interior, Outcome: PRBacklinkRepairPlanned},
			body:     br.PR.Body,
			revision: br.PR.Revision,
			path:     c.path,
			interior: c.interior,
		})
	}
	return planned, skipped
}

func prRepairRows(planned []prRepairPlanned, skipped []PRBacklinkRepair) []PRBacklinkRepair {
	rows := make([]PRBacklinkRepair, 0, len(planned)+len(skipped))
	for _, p := range planned {
		rows = append(rows, p.row)
	}
	return append(rows, skipped...)
}

func writePRRepairRows(b *strings.Builder, rows []PRBacklinkRepair) {
	for _, r := range rows {
		fmt.Fprintf(b, "  PR #%d (change %04d): %s\n", r.PR, r.ID, r.Outcome)
		if r.Current != "" {
			fmt.Fprintf(b, "    current:   %s\n", r.Current)
		}
		if r.Corrected != "" {
			fmt.Fprintf(b, "    corrected: %s\n", r.Corrected)
		}
		if r.Message != "" {
			fmt.Fprintf(b, "    %s\n", r.Message)
		}
	}
}

func prRepairConfirmationRequired(metadataTip string, planned []prRepairPlanned, skipped []PRBacklinkRepair) RepositoryRepairResult {
	rows := prRepairRows(planned, skipped)
	out := newRepairResult(ResultInvalidState, RepositoryRepairResult{
		RepositoryState: repairConfirmationRequiredState,
		SourceRevision:  metadataTip,
		PRBacklinks:     rows,
	})
	var b strings.Builder
	fmt.Fprintf(&b, "docket repository repair --pr-backlinks — preview (metadata %s)\n", metadataTip)
	writePRRepairRows(&b, rows)
	fmt.Fprintf(&b, "confirmation required: re-run with --yes to repoint these %d pull-request backlink(s)", len(planned))
	out.human = b.String()
	return out
}

func prRepairNoOp(metadataTip string, skipped []PRBacklinkRepair) RepositoryRepairResult {
	out := newRepairResult(ResultNoOp, RepositoryRepairResult{
		RepositoryState: string(reposetup.StateHealthy),
		SourceRevision:  metadataTip,
		PRBacklinks:     skipped,
	})
	var b strings.Builder
	fmt.Fprintf(&b, "repository repair --pr-backlinks: no pull-request backlink to repoint at metadata %s\n", metadataTip)
	writePRRepairRows(&b, skipped)
	out.human = strings.TrimRight(b.String(), "\n")
	return out
}

func prRepairApplied(metadataTip string, rows []PRBacklinkRepair, repointed, failed int) RepositoryRepairResult {
	result := ResultApplied
	if repointed == 0 && failed > 0 {
		result = ResultExternalFailed
	}
	out := newRepairResult(result, RepositoryRepairResult{
		RepositoryState: string(reposetup.StateHealthy),
		SourceRevision:  metadataTip,
		PRBacklinks:     rows,
	})
	var b strings.Builder
	fmt.Fprintf(&b, "repository repair --pr-backlinks: %d repointed, %d not repointed\n", repointed, failed)
	writePRRepairRows(&b, rows)
	out.human = strings.TrimRight(b.String(), "\n")
	return out
}
```

Check that `newRepairResult` exists under that name (`grep -n "func newRepairResult" internal/app/repository_repair.go`) and that `repairInternalFailure` takes a `reposetup.State`. Use the real helpers; do not write parallel ones.

- [ ] **Step 4: Implement the CLI side and its tests**

In `internal/cli/repository.go`, add the package seam next to `repositoryRepairRunner`:

```go
// repositoryRepairGitHub constructs the GitHub client `repository repair
// --pr-backlinks` reads and edits PR bodies through. It is called only when
// that flag is set; tests replace it.
var repositoryRepairGitHub = func() (app.RepairGitHub, error) { return githubcli.NewClient() }
```

`githubcli.NewClient()` returns `(*githubcli.Client, error)`. If Go rejects the direct return, assign it to a local first. In `newRepositoryRepairCommand`'s `RunE`, after building `deps`:

```go
			prBacklinks, _ := c.Flags().GetBool("pr-backlinks")
			if prBacklinks {
				gh, err := repositoryRepairGitHub()
				if err != nil {
					return err
				}
				deps.GitHub = gh
			}
```

Then pass `PRBacklinks: prBacklinks` in all three `app.RepairOptions{…}` literals: the `--yes` call, the preview call, and the confirmed re-invocation. Register the flag with `cmd.Flags().Bool("pr-backlinks", false, "repoint merged pull requests whose change backlink names a path that no longer exists (reads and edits PR descriptions on GitHub)")`. Change the annotation and its comment to:

```go
		// metadata-write: one repair descendant published to the metadata branch
		// under an exact lease. external-write: --pr-backlinks edits merged PR
		// descriptions on GitHub. It never touches the local .docket worktree.
		Annotations: capability("repository.repair", EffectExternalWrite, EffectMetadataWrite),
```

Add the `githubcli` import if it is missing. In `internal/cli/repository_test.go`, update `TestRepositoryRepairRegisteredWithCapability`: require the `pr-backlinks` flag, and expect effects `"external-write metadata-write"` (fix its doc comment to match). Add:

```go
// TestRepositoryRepairGitHubOnlyWithPRBacklinks proves the GitHub client is
// built and wired only under --pr-backlinks, and the flag reaches the service.
func TestRepositoryRepairGitHubOnlyWithPRBacklinks(t *testing.T) {
	built := 0
	oldGH := repositoryRepairGitHub
	repositoryRepairGitHub = func() (app.RepairGitHub, error) { built++; return nil, nil }
	var seen []app.RepairOptions
	oldR := repositoryRepairRunner
	repositoryRepairRunner = func(ctx context.Context, d app.SetupDeps, o app.RepairOptions) app.OperationResult {
		seen = append(seen, o)
		return fakeRepairResult{Envelope: app.NewEnvelope("repository.repair", app.ResultNoOp), source: "abc"}
	}
	defer func() { repositoryRepairGitHub = oldGH; repositoryRepairRunner = oldR }()

	runCLI(t, "repository", "repair", "--yes")
	if built != 0 || len(seen) != 1 || seen[0].PRBacklinks {
		t.Fatalf("plain repair built GitHub %d time(s) / opts %+v; want none", built, seen)
	}
	runCLI(t, "repository", "repair", "--pr-backlinks", "--yes")
	if built != 1 || len(seen) != 2 || !seen[1].PRBacklinks || !seen[1].Authorized {
		t.Fatalf("--pr-backlinks: built=%d opts=%+v", built, seen)
	}
}
```

Also extend `TestRepositoryRepairInteractiveConfirmReinvokes`, or add a sibling test, so that with `--pr-backlinks` the confirmed re-invocation still carries `PRBacklinks: true`.

- [ ] **Step 5: Run the tests to verify they pass, then mutation-test acceptance 4**

Run: `go test -tags integration -count=1 -run '^TestIntegrationRepoRepair' ./internal/app/`, `go test -count=1 ./internal/app/ ./internal/cli/`
Expected: PASS.

Mutation: back up `repository_repair.go`, change `if o.PRBacklinks {` to `if true {` (routine repair now runs the PR path), and rerun `-run '^TestIntegrationRepoRepairRoutine'`. Expected: FAIL (the forbidding fake is called). Restore with `mv -f` and rerun to PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/app/repository_facts.go internal/app/repository_repair.go internal/app/repository_repair_prbacklinks.go internal/app/repository_repair_integration_test.go internal/cli/repository.go internal/cli/repository_test.go
git commit -m "feat(repository): repair --pr-backlinks repoints merged PRs' dead change backlinks after a preview"
```

---

### Task 7: Living docs, bundle regeneration, and the whole-suite gate

**Files:**
- Modify: `docs/concepts/finalize-sequencer.md`, `docs/reference/glossary.md`, `skills/docket-finalize-change/SKILL.md`
- Regenerate: `internal/assets/embedded/` (via `go generate ./internal/assets/`)

**Interfaces:**
- Consumes: the behavior from Tasks 3–6, as it actually ships.
- Produces: docs that describe current behavior only.

- [ ] **Step 1: Edit the docs (only where they would otherwise be wrong or silent about a command a reader runs)**

`docs/concepts/finalize-sequencer.md`, in the close-out bullet: after "and retargets the backlinks in the change's spec, plan, and results files to the archived record." add: "It also repoints the backlink at the top of the merged pull request's description to the archived record. A description edit that fails leaves a `pr-backlink-pending` finding (the change stays `done`), and `docket finalize cleanup` and the maintenance sweep retry it."

`docs/reference/glossary.md`, in "Derived view / generated block / backlink", after "Each has exactly one writer and is never hand-edited.": "When a change closes out, every one of these backlinks, the one in the merged PR's description included, is repointed to the archived record." In the `repository repair` entry (the paragraph starting "`repository repair` is the human-authorized repair path"), add after the existing code block:

```markdown
`--pr-backlinks` repairs a different thing: merged pull requests whose description backlink still
names a change path that no longer exists. It reads those PR descriptions on GitHub, previews each
PR with its current and corrected link, and edits them one at a time only with `--yes`. A PR it
cannot read or edit is reported and skipped.

```sh
docket repository repair --pr-backlinks         # preview; add --yes to apply
```
```

`skills/docket-finalize-change/SKILL.md`: in the `done-archived` bullet, change "artifact block + spec backlink + inline board rerendered" to "artifact block + spec backlink + inline board rerendered, and the merged PR description's backlink repointed to the archived record". Append to the `contended` / `blocked` / `unknown` bullet: "A PR description that could not be repointed is a `pr-backlink-pending` warning on an otherwise-final result: report it, never redo the merge; cleanup (step 10) and the maintenance sweep retry it." In the step-10 `finalize.cleanup` paragraph, change "repair a pending backlink first" to "repair a pending backlink (artifact files and the PR description) first".

Do not cite this change, its PR, or its spec in any of these files.

- [ ] **Step 2: Regenerate the embedded bundle and run the docs guards**

```bash
go generate ./internal/assets/
go run ./cmd/genassets -check -repo .
go test -count=1 ./internal/repoguard/ ./internal/assets/
```

Expected: the generator rewrites only `internal/assets/embedded/tree/skills/docket-finalize-change/SKILL.md` (plus any index it maintains), `-check` passes, and the guards (`TestLivingDocsAlignment`, `TestCommentAnchorStyle`) PASS. If `-check` takes a different flag shape, use the one `cmd/genassets` documents.

- [ ] **Step 3: Read-only smoke against the real repository (optional, preview only)**

If `gh` is authenticated, run the preview from source against the real repository. This is a read and never `--yes`:

```bash
go run ./cmd/docket repository repair --pr-backlinks --repo-dir /Users/homer/dev/docket --json > "${TMPDIR:-/tmp}/prbacklinks-preview.json"; echo "exit=$?"
jq '{result, repository_state, planned: [.pr_backlinks[]? | select(.outcome=="planned")] | length, unreadable: [.pr_backlinks[]? | select(.outcome=="unreadable")] | length}' "${TMPDIR:-/tmp}/prbacklinks-preview.json"
```

Expected: `repository_state` `confirmation-required` with a planned count of roughly the number of merged PRs (on the order of 250). Record the counts in your report. A `planned` count of 0 here is a red flag: report it, do not ignore it. **Never** add `--yes`.

- [ ] **Step 4: Run the whole-suite gate**

Run: `go run ./cmd/docket development test`
Expected: PASS. Read the budget report. Any `BUDGET WATCH:`, `PARALLEL-SENSITIVE:`, or `SERIAL CONFIRMED OVER BUDGET:` line for `tests/test_go_integration_githubcli_merge.sh`, `…_app_closeout.sh`, `…_app_archive.sh`, `…_app_cleanup.sh`, or `…_app_reporepair.sh` is a finding to report (and, for a serial-confirmed breach, to fix by sizing the row in `tests/runtime-budgets.tsv` per `tests/README.md`).

- [ ] **Step 5: Commit**

```bash
git add docs/concepts/finalize-sequencer.md docs/reference/glossary.md skills/docket-finalize-change/SKILL.md internal/assets/embedded
git commit -m "docs: describe repointing the merged PR's change backlink and repair --pr-backlinks"
```

---

## Human verification items (for the results file)

These depend on facts that only exist outside the repository (learning `external-truth-needs-a-human-checkpoint`). The build cannot certify them.

1. **A real body round-trip on a merged PR.** Task 1 confirms GitHub reports `viewerCanUpdate: true` for a merged PR. It does not prove that `gh pr edit --body-file -` on a merged PR stores the body byte-identically, so that the verify read equals the request. If GitHub normalized the body, every real edit would verify as `contended`. To certify: after this change merges, run the first real `finalize closeout`. The close-out PR's description should link to `docs/changes/archive/…`, and the result should carry no `pr-backlink-pending` finding.
2. **The one-time repair is the human's.** After merge and the binary rebuild, run `docket repository repair --pr-backlinks` (preview), check the planned list, then re-run with `--yes`. A second preview should report nothing to repoint.
