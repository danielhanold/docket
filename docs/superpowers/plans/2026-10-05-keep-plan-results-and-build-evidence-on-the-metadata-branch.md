<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0530 — Keep plan, results, and build evidence on the metadata branch, and ship the spec with the PR](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0530-keep-plan-results-and-build-evidence-on-the-metadata-branch.md)**
<!-- docket:backlink:end -->
# Keep Plan, Results, and Build Evidence on the Metadata Branch — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. In this repository the plan is executed by `docket-build`.

**Goal:** Plan files, results files, and build evidence live on the `docket` metadata branch (plan and results at their existing paths, evidence in a `## Build evidence` section of the change record); the feature branch's first commit is a copy of the spec, so a PR carries exactly the spec copy plus code; same-branch links become relative; the post-merge backlink commit to `main`, the `Docket-Plan-Path` trailer, `artifact.backlink`, and `finalize.skip_results_only_delta` are retired.

**Architecture:** Five moves, sequenced so every intermediate commit builds and passes. (1) A new app-layer helper set (`internal/app/evidence_section.go`) reads and writes the existing evidence codec's block inside an operation-owned `## Build evidence` record section; writers move onto it first (`change.mark-implemented`, then `finalize.publish` / `evidence.recertify` dual-writing), then every reader switches (merge gate, clear-block, rebase no-op skip, `run.verify`), then the PR-body writes are deleted. (2) A new local-write operation `workspace.commit-spec` commits the spec copy as the feature branch's first commit. (3) `change.attach-plan` and then `change.attach-results` become metadata transactions that carry the artifact Markdown (`--markdown <file|->`), write the file on `docket`, stamp its backlink, and set the field in one commit; every reader of those files moves to the metadata tip. (4) The renderer emits relative links for same-branch rows and file backlinks (PR-body backlinks stay absolute), with a legacy rule that keeps pre-cutover plan/results rows absolute; the existing `repository check`/`repair` artifact-links drift machinery performs the one-time conversion. (5) The integration-branch backlink legs and `artifact.backlink` are deleted, a guard pins "no operation pushes the integration branch outside a PR merge", the deferred config key becomes an obsolete tombstone, and every skill, agent, embedded twin, and living doc is corrected.

**Tech Stack:** Go (`internal/app`, `internal/render`, `internal/repository`, `internal/domain`, `internal/config`, `internal/cli`, `internal/repoguard`), real-git fixtures behind the `integration` build tag (shard prefixes only), the `e2e` tag for `internal/app/finalize_e2e_test.go`, the protocol-faithful fake `gh`, Markdown skills mirrored into `internal/assets/embedded/tree/` by `go generate ./internal/assets/`.

**Spec:** `docs/superpowers/specs/2026-10-05-keep-plan-results-and-build-evidence-on-the-metadata-branch-design.md` on the `docket` branch. Read it at `/Users/homer/dev/docket/.docket/docs/superpowers/specs/2026-10-05-keep-plan-results-and-build-evidence-on-the-metadata-branch-design.md`. The change record is `/Users/homer/dev/docket/.docket/docs/changes/active/0530-keep-plan-results-and-build-evidence-on-the-metadata-branch.md`.

## Global Constraints

- **Cutover (spec "Cutover").** This run itself is built with the pre-change binary and the pre-change flow: this plan and the results file ride this feature branch as today. No build task writes to the real `docket` branch, pushes anywhere, or runs the installed `docket` binary to exercise new behavior (the installed binary is the OLD one). Exercise new behavior only through `go test` or `go run ./cmd/docket`.
- **One location for every repository** (spec Decision 1). Nothing in this change branches on a visibility mode. Private visibility is never mentioned in any `docs/` page. The one seam #532 needs is `specCopyChangeLine` (Task 6): a single function that renders the copy's change line.
- **Paths do not change** (Decision 2): plan `docs/superpowers/plans/<date>-<slug>.md`, results `<results_dir>/<date>-<slug>-results.md`, spec copy at the metadata spec's own path `docs/superpowers/specs/<date>-<slug>-design.md`.
- **Evidence block bytes are unchanged** (Decision 3): the record section's body is exactly `evidence.Render(rec)` (converted to the record's line ending). Do not change `internal/evidence` (its boundary guard `TestEvidenceImportsOnlyDocument` forbids importing `internal/render`; the section helpers live in `internal/app`).
- **No compatibility read of PR-body evidence** (Decision 7). After Task 5 no code reads or writes a `build-evidence` block in a PR body.
- **No new blocking gate.** The only new refusals are typed input/state refusals of the operations this change rewrites, each named in its task (`spec-path-occupied`, `artifact-path-mismatch`, `artifact-path-occupied`, `workspace-not-ready`, `spec-missing`). Nothing new can stop a close-out, cleanup, or sweep.
- **`pr.publish` writes no metadata.** The spec lists `pr.publish` among the evidence writers; this plan narrows that deliberately: `change.mark-implemented` already verifies the same evidence bytes at the same head in the same run and runs in a metadata transaction, every evidence reader runs after it, and a second record write at `pr.publish` would move the record revision under the run between publish and mark-implemented. `pr.publish` keeps verifying `--evidence` against the head (it still never opens a PR on an uncertified head) but writes the evidence nowhere. Record this narrowing in the results file.
- **Same-branch links are relative; cross-branch links are absolute** (Decision 6). PR row, `Spec (merged)` row, PR-body backlink, PR-body plan/results links: absolute. Everything else on the metadata branch: relative to the file that contains the link.
- **Docs describe current behavior only**, with no change or PR numbers (`TestLivingDocsAlignment`). ADR citations are allowed. Never hand-copy under `internal/assets/embedded/`; regenerate with `go generate ./internal/assets/` in the same task that edits a mirrored file (`TestEmbeddedMatchesAuthored`). Harness goldens under `internal/harness/*/testdata/golden/` are regenerated with their tests' `-update` flag (read the flag's definition in that package's golden test before running it).
- **Skill size budgets** (`internal/repoguard/budgets_test.go`, `TestSkillSizeBudgets`): every edited skill file sits at its ceiling today. Prose tasks aim to be net-neutral or shrinking (deleting the trailer, delta, and staleness prose frees room). If a file must grow, re-baseline exactly that row with a comment giving the reason, never a blanket bump.
- **Finding codes** are minted from named constants, never `Code: "..."` literals (`TestNoInlineFindingCodeLiterals`). When a task removes the last minting site of a reason constant, delete the constant and any registry row that lists it.
- **Cross-references** in maintained source anchor on symbol names or quoted clauses, never line numbers (`TestCommentAnchorStyle`).
- **Every mutation probe and re-verification uses `go test -count=1`** (learning `cached-runner-serves-a-mutated-tree`). Every mutation is restored from a backup copy (`cp f f.bak` … `mv -f f.bak f`), never `git checkout --` (learning `mutation-restore-needs-a-backup-copy`). After restoring, re-run the test green.
- **Integration tests run only through shard prefixes:** `go test -tags integration -count=1 -run '^<Prefix>' ./internal/app/`. Never run `./internal/app/` integration tests without `-run`. Prefixes used here all exist: `TestIntegrationWorkflowLifecycle`, `TestIntegrationWorkflowRepo`, `TestIntegrationChangeRuntime`, `TestIntegrationRecordOps`, `TestIntegrationEvidence`, `TestIntegrationFinalizeOps`, `TestIntegrationFinalizeState`, `TestIntegrationFinalizeMerge`, `TestIntegrationFinalizeRebaseGate`, `TestIntegrationFinalizeRebaseRecovery`, `TestIntegrationFinalizeRebaseForwardRefresh`, `TestIntegrationFinalizeCloseout`, `TestIntegrationFinalizeArchive`, `TestIntegrationFinalizeCleanup`, `TestIntegrationSweep`, `TestIntegrationRepoCheck`, `TestIntegrationRepoRepair`, `TestIntegrationNamed`, `TestIntegrationRunFence`, `TestIntegrationRunCompletion`. Do not add shards. The e2e file runs as `go test -tags e2e -count=1 -run '^TestE2E' ./internal/app/` (also driven by `tests/test_go_finalize_e2e.sh`).
- **Whole-repo greps, never memory** (AGENTS.md "Never hand-list the sites"): every task that removes or renames a symbol, flag, reason, or phrase ends with a `git grep -n -F -- '<token>'` sweep over the whole tree, sorting hits into executable / test / maintained prose / point-in-time records (results, archived changes, specs, Accepted ADRs, `docs/superpowers/plans/`) / frozen `testdata/`. Only the first three are edited. Frozen `testdata/` corpora are never edited.
- **Shell (AGENTS.md):** capture into a variable before `grep`; `grep -F -- '<pat>'` for patterns starting with `--`; `mv -f`; templated `mktemp`.
- **Whole-suite gate:** `go run ./cmd/docket development test` (docket-build runs it once at the end). Read its budget report even when green.
- **ADR:** the spec's expected ADR ("build artifacts live on the metadata branch; the spec ships with the PR", relating to ADR-0001 and ADR-0066) is recorded by `docket-implement-next` Step 6 through `docket-adr`, not by any build task. No source comment cites the new ADR's number.

## Review Focus

1. **A spec whose first `# ` line sits inside a fenced code block, or a spec with no H1 at all.** A person expects the change line right after the real title, or at the very top when there is none, and never inside a code fence. Pinned in Task 6, `TestSpecCopyBytesSkipsFencedH1` and `TestSpecCopyBytesNoTitle`.
2. **CRLF change records and specs** (any file edited on Windows or through the GitHub web UI). A person expects the evidence section and the spec copy to keep the file's line endings, never a mixed file. Pinned in Task 1, `TestUpsertRecordEvidenceKeepsCRLF`, and Task 6, `TestSpecCopyBytesKeepsCRLF`.
3. **An artifact body resubmitted exactly as read back from the metadata worktree** (it still carries its `docket:backlink` block). A person expects the operation to replace that block, not duplicate it, and a body whose backlink markers are malformed to be refused untouched. Pinned in Task 7, `TestAttachPlanStripsSubmittedBacklink` and `TestAttachPlanRefusesMalformedBacklink`.
4. **A `## Build evidence` heading written inside a fenced code block of authored record prose.** A person expects it to be content, not the evidence section. Pinned in Task 1, `TestReadRecordEvidenceIgnoresFencedHeading`.
5. **A legacy archived record** (status `done`, no `## Build evidence` section) whose plan and results live on `main`. A person expects `repository repair` to convert its spec and ADR rows to relative links, keep its Plan/Results rows pointing at `main`, and a second repair to change nothing. Pinned in Task 10, `TestArtifactBlockLegacyDoneKeepsIntegrationRows`, and `TestIntegrationRepoRepairConvertsAbsoluteSameBranchLinksOnce`.

---

## File Structure

| File | Responsibility | Task |
|---|---|---|
| `internal/repository/decode.go`, `internal/domain/entities.go` | `HasBuildEvidence` presence predicate (shape-keyed like `HasRunHalted`) | 1 |
| `internal/app/evidence_section.go` (new) | read/verify/upsert the `## Build evidence` section; `recordEvidenceFacts`; `recordBuildEvidence` transaction | 1, 3 |
| `internal/app/change_implemented.go` | writes the section in the implemented transaction; results read from the metadata tip | 2, 8 |
| `internal/app/finalize_publish.go`, `internal/app/evidence_recertify.go` | write the section; stop editing the PR body | 3, 5 |
| `internal/app/finalize_merge.go`, `finalize_block.go`, `finalize_rebase.go`, `run_verify.go`, `workspace_ops.go` | read evidence from the record | 4 |
| `internal/app/pr_publish.go` | no evidence block; PR-body plan/results links block | 5, 10 |
| `internal/app/workspace_spec.go` (new), `internal/cli/workspace.go` | `workspace.commit-spec` | 6 |
| `internal/app/change_attach.go`, `internal/cli/change.go` | metadata-transaction attach ops | 7, 8 |
| `internal/app/status.go`, `run_verify.go` | plan/results existence on the metadata tip, integration fallback | 7, 8 |
| `internal/app/artifact_backlink.go`, `internal/cli/artifact.go` (deleted) | retired op | 9 |
| `internal/render/artifacts.go`, `internal/render/relpath.go` (new) | relative rows and backlinks, legacy rule, `Spec (merged)`, PR plan/results block | 10 |
| `internal/app/finalize_closeout.go`, `finalize_cleanup.go`, `maintenance_assess.go`, `finalize_backlink_loader.go` (deleted), `change_kill.go` | retire integration legs; kill retargets plan/results | 10, 11 |
| `internal/repoguard/integration_push_test.go` (new) | no integration-branch push outside a PR merge | 11 |
| `internal/config/schema.go`, `decode.go`, `.docket.yml` | obsolete tombstone for `finalize.skip_results_only_delta` | 12 |
| skills, agents, cursor-rules, embedded twins, goldens | prose | 14, 15 |
| `README.md`, `docs/guide/*`, `docs/concepts/*`, `docs/reference/*`, `docs/release/upgrading-from-bash.md` | living docs | 16 |

---

### Task 1: The `## Build evidence` record section and the `HasBuildEvidence` predicate

**Build tier:** standard

**Files:**
- Modify: `internal/repository/decode.go` (the `presenceMarkers` const block and the line that sets `spec.HasRunHalted`)
- Modify: `internal/domain/entities.go` (the change spec struct field list next to `HasRunHalted`, and a `HasBuildEvidence` method next to `HasRunHalted()`)
- Create: `internal/app/evidence_section.go`
- Test: `internal/app/evidence_section_test.go` (new, untagged)
- Test: the existing decode test file that covers `HasRunHalted` (find it with `git grep -n 'HasRunHalted' -- 'internal/repository/*_test.go' 'internal/domain/*_test.go'`)

**Interfaces:**
- Produces:
  - `const buildEvidenceHeading = "## Build evidence"` (package `app`)
  - `func ReadRecordEvidence(record []byte) (evidence.Record, error)` — `evidence.ErrMissing` when the section is absent
  - `func VerifyRecordEvidence(record []byte, head string) evidence.Verdict`
  - `func UpsertRecordEvidence(record []byte, rec evidence.Record) ([]byte, error)` — accepts green and skipped
  - `func recordEvidenceFacts(record []byte) (head, command string, green bool)`
  - `func (c domain.Change) HasBuildEvidence() bool`

- [ ] **Step 1: Write the failing tests**

`internal/app/evidence_section_test.go`:

```go
package app

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/evidence"
)

const esHead = "0123456789abcdef0123456789abcdef01234567"

func esRecord() string {
	return "---\nid: 7\nslug: x\n---\n\n## Artifacts\n\n<!-- docket:artifacts:start (generated — do not hand-edit) -->\n<!-- docket:artifacts:end -->\n\n## Why\n\nBecause.\n"
}

func esGreen(t *testing.T) evidence.Record {
	t.Helper()
	r, err := evidence.NewRecord("go test ./...", esHead, time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestUpsertRecordEvidenceAppendsSection(t *testing.T) {
	rec := esGreen(t)
	out, err := UpsertRecordEvidence([]byte(esRecord()), rec)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	want := esRecord() + "\n## Build evidence\n\n" + evidence.Render(rec) + "\n"
	if string(out) != want {
		t.Fatalf("upsert bytes:\n%q\nwant\n%q", out, want)
	}
	got, err := ReadRecordEvidence(out)
	if err != nil || got != rec {
		t.Fatalf("read back = %+v, %v; want %+v", got, err, rec)
	}
}

func TestUpsertRecordEvidenceReplacesInPlace(t *testing.T) {
	first := esGreen(t)
	src := esRecord() + "\n## Build evidence\n\n" + evidence.Render(first) + "\n\n## Reconcile log\n\n### 2026-10-05\n\nok\n"
	second, _ := evidence.NewSkippedRecord(esHead, time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC))
	out, err := UpsertRecordEvidence([]byte(src), second)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if !strings.Contains(string(out), "## Build evidence\n\n"+evidence.Render(second)+"\n\n## Reconcile log") {
		t.Fatalf("section not replaced in place:\n%s", out)
	}
	if strings.Count(string(out), "## Build evidence") != 1 {
		t.Fatalf("duplicate section:\n%s", out)
	}
	if !strings.HasSuffix(string(out), "## Reconcile log\n\n### 2026-10-05\n\nok\n") {
		t.Fatalf("bytes after the section changed:\n%s", out)
	}
}

func TestUpsertRecordEvidenceKeepsCRLF(t *testing.T) {
	crlf := strings.ReplaceAll(esRecord(), "\n", "\r\n")
	out, err := UpsertRecordEvidence([]byte(crlf), esGreen(t))
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if strings.Contains(strings.ReplaceAll(string(out), "\r\n", ""), "\n") {
		t.Fatalf("mixed line endings in a CRLF record:\n%q", out)
	}
}

func TestReadRecordEvidenceMissingAndFenced(t *testing.T) {
	if _, err := ReadRecordEvidence([]byte(esRecord())); !errors.Is(err, evidence.ErrMissing) {
		t.Fatalf("absent section err = %v, want ErrMissing", err)
	}
}

func TestReadRecordEvidenceIgnoresFencedHeading(t *testing.T) {
	src := esRecord() + "\n```\n## Build evidence\n\n" + evidence.Render(esGreen(t)) + "\n```\n"
	if _, err := ReadRecordEvidence([]byte(src)); !errors.Is(err, evidence.ErrMissing) {
		t.Fatalf("fenced heading read as the section: err = %v", err)
	}
	if v := VerifyRecordEvidence([]byte(src), esHead); v != evidence.VerdictMissing {
		t.Fatalf("verify fenced = %q, want missing", v)
	}
}

func TestVerifyRecordEvidenceVerdicts(t *testing.T) {
	out, _ := UpsertRecordEvidence([]byte(esRecord()), esGreen(t))
	if v := VerifyRecordEvidence(out, esHead); v != evidence.VerdictVerified {
		t.Fatalf("exact head = %q", v)
	}
	if v := VerifyRecordEvidence(out, strings.Repeat("f", 40)); v != evidence.VerdictStale {
		t.Fatalf("other head = %q", v)
	}
	dup := string(out) + "\n## Build evidence\n\nx\n"
	if v := VerifyRecordEvidence([]byte(dup), esHead); v != evidence.VerdictMalformed {
		t.Fatalf("duplicated section = %q, want malformed", v)
	}
}

func TestRecordEvidenceFacts(t *testing.T) {
	out, _ := UpsertRecordEvidence([]byte(esRecord()), esGreen(t))
	h, cmd, green := recordEvidenceFacts(out)
	if h != esHead || cmd != "go test ./..." || !green {
		t.Fatalf("facts = %q %q %v", h, cmd, green)
	}
	if h, cmd, green := recordEvidenceFacts([]byte(esRecord())); h != "" || cmd != "" || green {
		t.Fatalf("absent facts = %q %q %v", h, cmd, green)
	}
}
```

In the decode test file, add a case beside the `HasRunHalted` case: a record body carrying a bare `## Build evidence` line decodes with `HasBuildEvidence() == true`; one carrying it only inside a fenced block, or as `## Build evidence — note`, decodes `false`.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test -count=1 -run 'RecordEvidence|BuildEvidence' ./internal/app/ ./internal/repository/ ./internal/domain/`
Expected: FAIL — `undefined: UpsertRecordEvidence` (and `HasBuildEvidence` undefined).

- [ ] **Step 3: Implement**

`internal/repository/decode.go`: add `markerBuildEvidence = "## Build evidence"` to the `presenceMarkers` const block and set `spec.HasBuildEvidence = hasHeading(text, markerBuildEvidence)` next to `spec.HasRunHalted`. `internal/domain/entities.go`: add the field `HasBuildEvidence bool // "## Build evidence" body section present` next to `HasRunHalted`, and:

```go
// HasBuildEvidence reports whether the body carries the operation-owned
// "## Build evidence" section — the durable build-evidence record that
// change.mark-implemented first writes. Its presence also marks a change built
// by the metadata-branch artifact flow (render's legacy-link rule keys on it).
func (c Change) HasBuildEvidence() bool { return c.spec.HasBuildEvidence }
```

`internal/app/evidence_section.go`:

```go
package app

import (
	"fmt"
	"strings"

	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/evidence"
	"github.com/danielhanold/docket/internal/render"
)

// This file owns the change record's operation-owned "## Build evidence"
// section: the durable home of a change's build evidence. The section body is
// exactly the evidence codec's build-evidence block (evidence.Render), so the
// bytes a reader extracts are the bytes the PR description used to carry. The
// section is never authored: change.groom's owned-heading allowlist
// (render.ChangeOwnedHeadings) does not contain it.

// buildEvidenceHeading is the exact H2 line of the section.
const buildEvidenceHeading = "## Build evidence"

// recordEvidenceSection returns the section body, whether it is present, and an
// error for a duplicated heading. Headings inside fenced code are content
// (namedSectionBody scans fence-aware).
func recordEvidenceSection(record []byte) (string, bool, error) {
	return namedSectionBody(record, buildEvidenceHeading)
}

// ReadRecordEvidence extracts the record's build evidence. An absent section is
// evidence.ErrMissing; a duplicated section or a malformed block is an error.
func ReadRecordEvidence(record []byte) (evidence.Record, error) {
	body, present, err := recordEvidenceSection(record)
	if err != nil {
		return evidence.Record{}, fmt.Errorf("record evidence: %w", err)
	}
	if !present {
		return evidence.Record{}, evidence.ErrMissing
	}
	return evidence.Extract([]byte(body))
}

// VerifyRecordEvidence is evidence.Verify scoped to the record's section: missing
// when absent, malformed when the section is duplicated or its block unreadable.
func VerifyRecordEvidence(record []byte, head string) evidence.Verdict {
	body, present, err := recordEvidenceSection(record)
	if err != nil {
		return evidence.VerdictMalformed
	}
	if !present {
		return evidence.VerdictMissing
	}
	return evidence.Verify([]byte(body), head)
}

// UpsertRecordEvidence makes rec (green or skipped) the whole body of the
// record's "## Build evidence" section, appending the section at EOF when it is
// absent and preserving every other byte. The record is re-normalized through
// the codec constructors, rendered in the record's own line ending, and the
// candidate must read back as exactly rec before any bytes are returned.
func UpsertRecordEvidence(record []byte, rec evidence.Record) ([]byte, error) {
	var norm evidence.Record
	var err error
	switch rec.Result {
	case evidence.ResultGreen:
		norm, err = evidence.NewRecord(rec.Command, rec.Head, rec.RanAt)
	case evidence.ResultSkipped:
		norm, err = evidence.NewSkippedRecord(rec.Head, rec.RanAt)
	default:
		return nil, fmt.Errorf("record evidence: result %q is neither green nor skipped", rec.Result)
	}
	if err != nil {
		return nil, err
	}
	doc, err := document.Parse(record)
	if err != nil {
		return nil, fmt.Errorf("record evidence: malformed record: %w", err)
	}
	block := evidence.Render(norm)
	if le := doc.LineEnding(); le != "" && le != "\n" {
		block = strings.ReplaceAll(block, "\n", le)
	}
	out, err := render.ApplySectionEdits(record, []string{buildEvidenceHeading},
		[]render.SectionEdit{{Heading: buildEvidenceHeading, Intent: render.SectionReplace, Markdown: block}})
	if err != nil {
		return nil, fmt.Errorf("record evidence: %w", err)
	}
	got, err := ReadRecordEvidence(out)
	if err != nil {
		return nil, fmt.Errorf("record evidence: candidate failed reparse: %w", err)
	}
	if got != norm {
		return nil, fmt.Errorf("record evidence: candidate reparsed to a different record")
	}
	return out, nil
}

// recordEvidenceFacts is the record-section reader the finalize gates share: the
// recorded head and command, and whether the record is green. Any read failure
// — absent, duplicated, malformed — reads as no green evidence.
func recordEvidenceFacts(record []byte) (head, command string, green bool) {
	rec, err := ReadRecordEvidence(record)
	if err != nil {
		return "", "", false
	}
	return rec.Head, rec.Command, rec.Result == evidence.ResultGreen
}
```

If `document.Document.LineEnding()` returns `""` for a single-line file, the `le != ""` guard keeps LF. If `TestUpsertRecordEvidenceAppendsSection`'s expected bytes differ only in the blank-line layout `render.ApplySectionEdits` produces (read `appendSection`), adjust the expectation to that layout — never the helper — and keep the assert byte-exact.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -count=1 -run 'RecordEvidence|BuildEvidence' ./internal/app/ ./internal/repository/ ./internal/domain/`
Expected: PASS.

- [ ] **Step 5: Mutation-test the fence guard**

`cp internal/app/evidence_section.go internal/app/evidence_section.go.bak`; replace the body of `recordEvidenceSection` with a naive `strings.Index(string(record), buildEvidenceHeading)` slice; run `go test -count=1 -run 'IgnoresFencedHeading' ./internal/app/` — expect FAIL; `mv -f internal/app/evidence_section.go.bak internal/app/evidence_section.go`; re-run — PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/repository/decode.go internal/domain/entities.go internal/app/evidence_section.go internal/app/evidence_section_test.go <the decode test file you extended>
git commit -m "feat(evidence): the change record's ## Build evidence section"
```

---

### Task 2: `change.mark-implemented` records the evidence it verifies

**Build tier:** standard

**Files:**
- Modify: `internal/app/change_implemented.go` (`ChangeMarkImplemented`, `changeImplementedOp`, `(o changeImplementedOp) Plan`)
- Test: `internal/app/change_implemented_test.go` (`TestMarkImplementedApplies`)
- Test: `internal/app/change_implemented_integration_test.go` (`TestIntegrationRecordOpsMarkImplementedAcceptsSkippedEvidence`)

**Interfaces:**
- Consumes: `UpsertRecordEvidence`, `ReadRecordEvidence` (Task 1).
- Produces: after an applied `change.mark-implemented`, the committed record carries `## Build evidence` holding exactly the `--evidence` record. Later tasks rely on this.

- [ ] **Step 1: Write the failing tests**

In `TestMarkImplementedApplies`, after the applied assertion, read the committed record bytes from the metadata remote (the test already reads the record for its status/pr asserts — reuse that read) and add:

```go
gotEv, err := ReadRecordEvidence(recordBytes)
if err != nil {
	t.Fatalf("implemented record carries no readable build evidence: %v", err)
}
wantEv, _ := evidence.Extract(evidenceBytes) // the bytes the test passed as EvidenceRecord
if gotEv != wantEv {
	t.Fatalf("recorded evidence = %+v, want %+v", gotEv, wantEv)
}
```

In `TestIntegrationRecordOpsMarkImplementedAcceptsSkippedEvidence`, assert the same for the skipped record (`gotEv.Result == evidence.ResultSkipped`, `gotEv.Reason == evidence.ReasonBuildGateOff`).

- [ ] **Step 2: Run to verify failure**

Run: `go test -count=1 -run 'TestMarkImplementedApplies' ./internal/app/` and `go test -tags integration -count=1 -run '^TestIntegrationRecordOpsMarkImplementedAcceptsSkippedEvidence' ./internal/app/`
Expected: FAIL — `evidence: no build-evidence block`.

- [ ] **Step 3: Implement**

In `ChangeMarkImplemented`, right after the condition-3 `evidence.Verify` check, extract the record once (`rec, err := evidence.Extract(req.EvidenceRecord)`; on error refuse `ReasonImplementedEvidenceUnverified`, the same fail-closed shape `FinalizePublish` uses). Add `evidence evidence.Record` to `changeImplementedOp` and set it when building `txOp`. In `Plan`, after `finalBytes` is computed and before `files` is built:

```go
// The implemented transition records the evidence it just verified as the
// record's durable "## Build evidence" section, in this same commit.
finalBytes, err = UpsertRecordEvidence(finalBytes, o.evidence)
if err != nil {
	return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("mark-implemented: recording build evidence: %w", err)
}
```

Update the `changeImplementedOp` doc comment to name the section among the owned writes.

- [ ] **Step 4: Run to verify pass**

Same commands as Step 2, then the shard: `go test -tags integration -count=1 -run '^TestIntegrationRecordOps' ./internal/app/` and `go test -tags integration -count=1 -run '^TestIntegrationChangeRuntime' ./internal/app/`.
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/change_implemented.go internal/app/change_implemented_test.go internal/app/change_implemented_integration_test.go
git commit -m "feat(mark-implemented): record the verified build evidence in the change record"
```

---

### Task 3: `finalize.publish` and `evidence.recertify` also write the record section

**Build tier:** premium — the finalize publication order and its CAS against a concurrently moving metadata branch are consequential.

**Files:**
- Modify: `internal/app/evidence_section.go` (add the transaction helper)
- Modify: `internal/app/finalize_publish.go` (`FinalizePublish`)
- Modify: `internal/app/evidence_recertify.go` (`publishRecertifiedEvidence`, and the `build.gate: off` branch of `EvidenceRecertify`)
- Modify: `internal/cli/finalize.go` (`newFinalizePublishSubcommand` capability + its "opens no metadata transaction" comment), `internal/cli/evidence.go` (`evidence.recertify` capability)
- Test: `internal/app/evidence_section_test.go`, `internal/app/finalize_publish_integration_test.go`, `internal/app/evidence_recertify_integration_test.go`, `internal/app/finalize_state_integration_test.go`

**Interfaces:**
- Consumes: `UpsertRecordEvidence` (Task 1).
- Produces:

```go
// recordEvidenceOutcome is the folded result of one record-evidence write.
type recordEvidenceOutcome struct {
	Result   Result // ResultApplied, ResultNoOp, ResultContended, or a failure class
	Reason   string
	Message  string
	Revision string // the applied metadata commit, when applied
}

// recordBuildEvidence writes rec into change id's "## Build evidence" section in
// one exact-revision metadata transaction keyed on the record blob it reads.
func recordBuildEvidence(ctx context.Context, deps PlanningDeps, repoDir, opKey string, id int, rec evidence.Record) recordEvidenceOutcome
```

- [ ] **Step 1: Write the failing tests**

1. `TestIntegrationFinalizeOpsFinalizePublishAcceptsSkippedEvidence` and `TestIntegrationFinalizeStatePublishOrder`: after the publish, read the record at the metadata remote tip and assert `ReadRecordEvidence` equals the published record (the PR-body assertions stay in this task; they leave in Task 5).
2. `TestIntegrationEvidenceEvidenceRecertifyHappyPath`: assert the record section equals the recertified record. `TestIntegrationEvidenceEvidenceRecertifyGateOffRecordsSkipped`: assert the record section holds the skipped record (until now gate-off wrote nothing durable).
3. New `TestIntegrationFinalizeStatePublishRecordEvidenceContended` (prefix `TestIntegrationFinalizeState`): advance the change record on the metadata remote between the publish's corpus read and its transaction (use the same injection seam the existing `...PublishCrashReplay` test uses to interpose; if none fits, advance the record before the call and stub the pin so the publish reads the stale revision) and assert the result is `contended` with the record untouched.
4. A unit test `TestRecordBuildEvidenceIsIdempotent` beside the Task 1 tests if the package already has a fake-engine harness for transactions (look for the fake `Engine` used in `change_attach_test.go`); otherwise cover idempotency in the integration test by calling publish twice and asserting the second record write is a no-op (no new metadata commit).

- [ ] **Step 2: Run to verify failure**

Run: `go test -tags integration -count=1 -run '^TestIntegrationFinalizeOps' ./internal/app/`, `... -run '^TestIntegrationFinalizeState' ...`, `... -run '^TestIntegrationEvidence' ...`
Expected: the new record assertions FAIL.

- [ ] **Step 3: Implement `recordBuildEvidence`**

Append to `internal/app/evidence_section.go`:

```go
// recordEvidenceOp is the SemanticOperation recordBuildEvidence drives. It edits
// only the change record's "## Build evidence" section; no board-visible field
// changes, so no board render is included. An unchanged section is the engine's
// clean no-op path.
type recordEvidenceOp struct {
	opKey    string
	changeID int
	rec      evidence.Record
}

func (o recordEvidenceOp) Key() transaction.OperationKey { return transaction.OperationKey(o.opKey) }

func (o recordEvidenceOp) Plan(ctx context.Context, st transaction.AttemptState) (transaction.MutationPlan, transaction.OperationResult, error) {
	c, out := st.State.Snapshot.Change(domain.ChangeID(o.changeID))
	if out != domain.LookupFound {
		return refuseLifecycle(FindingCode(ReasonImplementedUnknownChange), fmt.Sprintf("change %04d is not a single record in the current corpus", o.changeID))
	}
	src, ok := st.State.Sources[c.Path()]
	if !ok {
		return refuseLifecycle(FCPathMismatch, fmt.Sprintf("no record source loaded at %q for change %04d", c.Path(), o.changeID))
	}
	updated, err := UpsertRecordEvidence(src, o.rec)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, err
	}
	receipt, err := json.Marshal(struct {
		Head string `json:"head"`
		ID   int    `json:"id"`
		Op   string `json:"op"`
	}{Head: o.rec.Head, ID: o.changeID, Op: o.opKey})
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, err
	}
	var files []transaction.FileMutation
	if !bytes.Equal(updated, src) {
		files = append(files, transaction.FileMutation{Path: gitcli.RepoPath(c.Path()), Kind: transaction.MutationReplace, Bytes: updated})
	}
	return transaction.MutationPlan{
		Files:         files,
		CommitSubject: fmt.Sprintf("change %04d build evidence recorded", o.changeID),
		Receipt:       receipt,
	}, transaction.OperationResult{}, nil
}
```

`recordBuildEvidence` pins (`deps.Reader.PinContext`), reads the corpus once and resolves the change, its path, and its blob revision the same way `resolveImplementedChange` does (reuse that function — it already returns `(change, recPath, revision, refusal)`; map its refusal's first finding code into `Reason`), discovers the repository, and runs `deps.Engine.Execute` with `TargetRef` `branchRefPrefix + reposetup.MetadataBranchName`, `Expected` = the record path at that blob revision, `Loader: newPlanningLoader(eff)`, `Scope: changeScope(id, recPath, false)`, `Operation: recordEvidenceOp{...}`. Fold the outcome with `mapOutcome(res, execErr, ResultInvalidState)`; set `Revision` from `res.AppliedCommit` when applied. Receipt fields are alphabetical (the engine's receipt validator requires the sorted compact form, as `changeAttachReceipt` notes).

- [ ] **Step 4: Call it from the two writers**

`FinalizePublish`: after `publishResultFromEnsure` would return a success disposition (published or noop), call `recordBuildEvidence(ctx, deps.Planning, repoDir, OperationFinalizePublish, id, rec)`. A `contended` record write returns `PublishDispContended` with `Reason: ReasonPublishRecordContended` (new constant `"record-evidence-contended"`); any other non-applied/no-op result returns `PublishDispUnknown` with `Reason: ReasonPublishRecordFailed` (new constant `"record-evidence-failed"`) and the message. When the PR edit was a noop but the record write applied, the disposition is `published`.

`EvidenceRecertify`: in `publishRecertifiedEvidence`, after the PR ensure succeeds, call `recordBuildEvidence(..., OperationEvidenceRecertify, id, rec)` and map failures to new reasons `ReasonRecertifyRecordContended` / `ReasonRecertifyRecordFailed` on the existing contended/unknown result shapes. In the `build.gate: off` branch, write the minted skipped record with `recordBuildEvidence` (this is now its durable home) and report applied/no-op from it.

Capabilities: `finalize.publish` → `EffectExternalWrite, EffectMetadataWrite`; `evidence.recertify` → `EffectExternalWrite, EffectMetadataWrite` (it still edits the PR until Task 5). Replace the "opens no metadata transaction" comment and recertify's "no Docket metadata mutation" file-header sentence with the truth. Update any capability-catalog golden or effects table test the catalog change reddens (`go test -count=1 ./internal/cli/ ./internal/repoguard/` names them).

- [ ] **Step 5: Run to verify pass**

Run the three shards from Step 2 plus `go test -count=1 ./internal/cli/ ./internal/app/`.
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/app/evidence_section.go internal/app/evidence_section_test.go internal/app/finalize_publish.go internal/app/evidence_recertify.go internal/cli/finalize.go internal/cli/evidence.go <updated tests and goldens>
git commit -m "feat(finalize): publish and recertify record build evidence in the change record"
```

---

### Task 4: Every evidence reader reads the record section

**Build tier:** premium — the finalize merge gate decides whether code merges.

**Files:**
- Modify: `internal/app/workspace_ops.go` (`workspaceContext`, `loadWorkspaceContext`: carry the record bytes from the same corpus read that yields `revision`)
- Modify: `internal/app/finalize_merge.go` (`mergeContext` gains `record []byte` from `loadMergeContext`'s corpus read; the `prBodyEvidence(pr)` call in `FinalizeMerge`)
- Modify: `internal/app/finalize_block.go` (`FinalizeClearBlock`'s `prBodyEvidence(prs[0])` call)
- Modify: `internal/app/finalize_rebase.go` (`rebaseContext` gains `record []byte` from `loadRebaseContext`'s workspace context; `composeLocalGate`'s `prBodyEvidence(pr)` call; delete `prBodyEvidence`)
- Modify: `internal/app/run_verify.go` (evidence condition; the `ReasonRunEvidenceUnverified` doc comment)
- Test: `internal/app/finalize_rebase_test.go` (`TestPRBodyEvidenceReportsCommandAndGreenFlag` → `TestRecordEvidenceFactsReportsCommandAndGreenFlag`), `finalize_merge_integration_test.go`, `finalize_state_integration_test.go` (`TestIntegrationFinalizeStateClearBlockReprobes`), `finalize_rebase_integration_test.go`, `run_verify_integration_test.go`, `change_integration_test.go` (`TestIntegrationChangeRuntimeRunVerify*`), `run_verify_test.go`

**Interfaces:**
- Consumes: `recordEvidenceFacts`, `VerifyRecordEvidence` (Task 1); the section written by Tasks 2–3.
- Produces: `workspaceContext.record`, `mergeContext.record`, `rebaseContext.record` (`[]byte`, the change record's bytes from the one pinned corpus read — decide and act on the same copy).

- [ ] **Step 1: Convert the fixtures and write the parity tests**

Every test that seeds evidence in a fake or real PR body for these four readers now seeds it in the change record's `## Build evidence` section instead (build the record with `UpsertRecordEvidence` before writing it to the metadata branch fixture) and leaves the PR body without a block. Keep each test's original expectation (pass/fail, reason) — that is the parity claim (spec acceptance 3). Specifically:

- `TestIntegrationFinalizeMergeConditionAssembly`: the `gate-no-evidence` case seeds no section; `gate-stale-evidence` seeds a section at another head; the happy case seeds green at the head. Add a case `gate-evidence-only-in-pr-body`: green block in the PR body, no record section → `gate-unsatisfied` (no compatibility read, Decision 7).
- `TestIntegrationFinalizeMergeConditionsRechecked`: the "sets `Body = ""`" step becomes "removes the record section" (rewrite the record on the metadata remote without the section).
- `TestIntegrationFinalizeStateClearBlockReprobes`: the `ReasonClearEvidenceUnverified` row seeds a stale record section.
- Rebase gate tests that rely on the no-op skip (`TestIntegrationFinalizeRebaseGateOutcomes` and the others the evidence report names) seed record evidence; the gate-decision unit tests (`TestGateDecision*`) are unchanged.
- `TestIntegrationChangeRuntimeRunVerify*` and `TestIntegrationEvidenceRunVerifyAcceptsSkippedEvidenceAtExactHead`: seed the record section. Add `TestIntegrationEvidenceRunVerifyEvidenceReadsRecordNotPR`: green block in the PR body only → `run-incomplete` with `evidence-unverified`.
- Rename/rewrite `TestPRBodyEvidenceReportsCommandAndGreenFlag` to drive `recordEvidenceFacts` over a record built with `UpsertRecordEvidence` (green → command and `true`; skipped → empty command and `false`).

- [ ] **Step 2: Run to verify failure**

Run each shard: `TestIntegrationFinalizeMerge`, `TestIntegrationFinalizeState`, `TestIntegrationFinalizeRebaseGate`, `TestIntegrationFinalizeRebaseRecovery`, `TestIntegrationFinalizeRebaseForwardRefresh`, `TestIntegrationEvidence`, `TestIntegrationChangeRuntime`; and `go test -count=1 -run 'RecordEvidenceFacts|RunVerify' ./internal/app/`.
Expected: FAIL — readers still consult the PR body.

- [ ] **Step 3: Implement**

- `loadWorkspaceContext`: where it scans `blobs` for `b.Path == c.Path()` to set `revision`, also keep `record = b.Data`; add `record []byte` to `workspaceContext` and set it.
- `loadRebaseContext`: copy `wc.record` into `rebaseContext.record`.
- `composeLocalGate`: `evidenceHead, evidenceCommand, evidenceGreen := recordEvidenceFacts(rc.record)`. Update the surrounding comments ("The PR-body evidence skip" → "The record-evidence skip"; spec §4 wording stays).
- `loadMergeContext`: capture the record bytes from its single corpus read into `mergeContext.record`; `FinalizeMerge`: `evHead, _, evGreen := recordEvidenceFacts(mc.record)`.
- `FinalizeClearBlock`: read the record bytes from the same corpus read that gives it the record revision it pins, and call `recordEvidenceFacts(record)`; change its refusal message to "the change record's build evidence does not verify green for the exact current head; the marker stays".
- `RunVerify`: take the record bytes from `blobs` (the entry whose `Path == c.Path()`); replace the PR-body `evidence.Verify` with `if v := VerifyRecordEvidence(record, head); v != evidence.VerdictVerified && v != evidence.VerdictSkipped { add(ReasonRunEvidenceUnverified, string(v)) }`, evaluated once, independent of the PR count (delete the `"no unique pull-request body to read evidence from"` add). Update `ReasonRunEvidenceUnverified`'s comment to "the durable build evidence (the change record's ## Build evidence section)".
- Delete `prBodyEvidence`. `git grep -n 'prBodyEvidence\|pr\.Body' -- internal/app` afterwards: no evidence read of a PR body may remain (the `pr_backlink_repoint.go` body edits are backlink, not evidence — leave them).

- [ ] **Step 4: Run to verify pass**

Re-run every command from Step 2. Expected: PASS.

- [ ] **Step 5: Mutation-test the reads (spec acceptance 3)**

For each of `FinalizeMerge`, `composeLocalGate`, `FinalizeClearBlock`, `RunVerify`, one at a time: back up the file, replace the record read with a constant "green at the requested head" (`evHead, evGreen = req.Head, true` / the equivalent), run that reader's shard with `-count=1` — expect at least one FAIL naming the missing or stale case — restore with `mv -f`, re-run green. Record the four readings for the results file.

- [ ] **Step 6: Commit**

```bash
git add internal/app/workspace_ops.go internal/app/finalize_merge.go internal/app/finalize_block.go internal/app/finalize_rebase.go internal/app/run_verify.go <updated tests>
git commit -m "feat(finalize): merge gate, clear-block, rebase skip, and run verify read the record's build evidence"
```

---

### Task 5: Nothing writes build evidence into a PR body

**Build tier:** standard

**Files:**
- Modify: `internal/app/pr_publish.go` (`assemblePRBody` drops its `rec` parameter and the `evidence.Upsert` call; `PRPublish` keeps the `--evidence` verify/extract as a head gate)
- Modify: `internal/app/finalize_publish.go` (delete the `evidence.Upsert` + `EnsurePullRequest` block; the record write from Task 3 becomes the publication; delete `finalizePublishEnsurer` if unused, `ReasonPublishBodyAssembly`, `ReasonPublishEnsurerUnavailable`; rewrite `publishResultFromEnsure` into a mapper over the rewrite outcome and the record outcome)
- Modify: `internal/app/evidence_recertify.go` (delete the PR-body upsert/ensure in `publishRecertifiedEvidence`; the record write is the publication)
- Modify: `internal/cli/pr.go`, `internal/cli/finalize.go`, `internal/cli/evidence.go` (comments: `evidence.record`'s "becomes the durable record only later, at `pr publish`" → "at `change mark-implemented`"; `evidence.recertify` capability → `EffectMetadataWrite` only)
- Test: `internal/app/pr_publish_test.go`, `pr_publish_integration_test.go`, `change_integration_test.go` (`TestIntegrationChangeRuntimePRPublishBodyAssembly`), `finalize_publish_helpers_test.go`, `finalize_publish_integration_test.go`, `finalize_state_integration_test.go`, `evidence_recertify_integration_test.go`, `runtracker_fence_integration_test.go`

**Interfaces:**
- Consumes: `recordBuildEvidence` (Task 3).
- Produces: `func assemblePRBody(authored []byte, backlink string) ([]byte, error)` (Task 10 adds a third parameter).
- `FinalizePublish` dispositions: `published` when the rewrite published or the record write applied; `noop` when the rewrite was a noop and the record write a no-op; `contended`/`unknown` as Task 3 mapped them. The PR is still probed (exactly one open PR whose head equals the request head) before the record write, with the existing reasons.

- [ ] **Step 1: Rewrite the assertions**

- `TestIntegrationChangeRuntimePRPublishBodyAssembly`: the assembled body equals the authored body with the backlink block inserted, and `evidence.Extract(body)` returns `evidence.ErrMissing`. A pre-existing stale build-evidence block inside the AUTHORED body is authored bytes and survives untouched (pin that).
- `TestIntegrationFinalizeOpsPRPublishAcceptsSkippedEvidenceAtExactHead`: a skipped record now publishes the PR (no `body-assembly-failed`); assert applied and no block in the body.
- `TestIntegrationFinalizeStatePublishOrder` and the `finalize_publish_helpers_test.go` fakes: no `EnsurePullRequest` call is made (`gh.ensLast` stays zero); the record section is the publication.
- Recertify tests (`...HappyPath`, `...EditContended`, `...EditFailureThenRetry`): the GitHub edit cases become record-write cases (contended = the record moved; failure-then-retry = the second call converges with a no-op or applied record write). Delete cases that only exercised the PR edit, recording each deletion's reason in the commit body.
- `TestIntegrationRunFencePRPublishJournalsPublicationIdentity`: the body digest is computed from the new assembled body; update the expected digest input, not the mechanism.

- [ ] **Step 2: Run to verify failure**

Run: shards `TestIntegrationChangeRuntime`, `TestIntegrationFinalizeOps`, `TestIntegrationFinalizeState`, `TestIntegrationEvidence`, `TestIntegrationRunFence`; `go test -count=1 ./internal/app/ ./internal/cli/`.
Expected: FAIL on the rewritten assertions.

- [ ] **Step 3: Implement**

Delete the writes listed under **Files**. `assemblePRBody` keeps inserting/replacing only the backlink block. In `FinalizePublish`, after the PR head check, call `recordBuildEvidence` and return `publishResultFromRecord(id, req.Head, rout, repo, pr, recOutcome)` — a new mapper replacing `publishResultFromEnsure` with the dispositions in **Interfaces**. In `publishRecertifiedEvidence`, after the second probe's head/PR/command checks, call `recordBuildEvidence` and map it.

- [ ] **Step 4: Sweep**

`git grep -n -F -- 'evidence.Upsert' -- internal/` → only `internal/evidence` and its tests remain. `git grep -n 'EnsurePullRequest' -- internal/app` → only `pr_publish.go` (PR creation) remains. Remove now-unused helpers the compiler or `go vet` flags.

- [ ] **Step 5: Run to verify pass**

Same as Step 2, plus `go test -tags e2e -count=1 -run '^TestE2E' ./internal/app/` (update its PR-body evidence expectations the same way).
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add -- internal/app internal/cli
git commit -m "feat(pr): build evidence no longer rides the PR description"
```

(Stage the exact files you changed by path; `git add -- internal/app internal/cli` is acceptable only if `git status --porcelain` shows nothing else under those directories.)

---

### Task 6: `workspace.commit-spec` — the spec copy is the feature branch's first commit

**Build tier:** premium — a new operation that commits on the feature branch.

**Files:**
- Create: `internal/app/workspace_spec.go`
- Modify: `internal/app/workspace_ops.go` (`workspaceContext` gains `pin StatusPin`; `loadWorkspaceContext` sets it)
- Modify: `internal/app/schema_registry.go` (row `{ID: "workspace.commit-spec", Request: nil, Result: WorkspaceOpResult{}}`)
- Modify: `internal/cli/workspace.go` (subcommand `commit-spec`, flag `--id`, `capability("workspace.commit-spec", EffectLocalWrite)`), `internal/cli/install.go` (`"workspace commit-spec": true` in `assetIndependent`)
- Test: `internal/app/workspace_spec_test.go` (new, untagged: pure `specCopyBytes` tests)
- Test: `internal/app/workflow_integration_test.go` (new `TestIntegrationWorkflowLifecycleCommitSpec*` tests)
- Test: `internal/cli/workspace_test.go` (or the file that asserts the workspace subcommands are registered)

**Interfaces:**
- Produces:

```go
const OperationWorkspaceCommitSpec = "workspace.commit-spec"

// Dispositions (WorkspaceOpResult.Disposition):
const (
	SpecCopyCommitted = "committed" // first copy committed (applied)
	SpecCopyRefreshed = "refreshed" // a revised spec replaced the copy (applied)
	SpecCopyCurrent   = "current"   // the feature head already carries the exact copy (no-op)
	SpecCopyNoSpec    = "no-spec"   // trivial change: no spec, no copy (no-op)
)

// Refusal reasons:
const (
	ReasonSpecCopyNotReady       = "workspace-not-ready" // workspace not in the ready (clean, owned) state
	ReasonSpecCopyMissing        = "spec-missing"        // spec: names no file on the metadata tip
	ReasonSpecCopyPathOccupied   = "spec-path-occupied"  // integration branch holds different bytes at the path
	ReasonSpecCopyMalformed      = "spec-malformed"      // the metadata spec's managed markers do not parse
)

func WorkspaceCommitSpec(ctx context.Context, deps PlanningDeps, wdeps WorkspaceDeps, repoDir string, req WorkspaceIDRequest) WorkspaceOpResult
func specCopyBytes(spec []byte, c domain.Change) ([]byte, error)
func specCopyChangeLine(c domain.Change) string
```

`WorkspaceOpResult` fields used: `ID`, `Path` (the spec path), `Head` (the feature head after the call), `Disposition`, `Reason`, `Message`.

- [ ] **Step 1: Write the failing pure tests**

`internal/app/workspace_spec_test.go`:

```go
package app

import (
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/repository"
)

func specFixture() string {
	return "<!-- docket:backlink:start (generated — do not hand-edit) -->\n" +
		"> ↩ **[Change 0042 — Widget](https://example.invalid/blob/docket/docs/changes/active/0042-widget.md)**\n" +
		"<!-- docket:backlink:end -->\n\n" +
		"# Widget: design\n\nBody paragraph.\n\n## Decisions\n\n1. One.\n"
}

// specChange builds change 42 "Widget" through the same corpus path the
// operations use (snapshotOf / mustChange live in named_branch_facts_test.go).
func specChange(t *testing.T) domain.Change {
	t.Helper()
	rec := strings.Replace(lifecycleChange(42, "widget", "in-progress"), "title: 'A change'", "title: 'Widget'", 1)
	snap := snapshotOf(t, []StatusBlob{{
		Kind:     repository.KindChange,
		Location: repository.LocationActive,
		Path:     groomPath(42, "widget"),
		Revision: "blobspec0042",
		Data:     []byte(rec),
	}})
	return mustChange(t, snap, 42)
}

func TestSpecCopyBytesStripsBacklinkAndAddsChangeLine(t *testing.T) {
	got, err := specCopyBytes([]byte(specFixture()), specChange(t))
	if err != nil {
		t.Fatal(err)
	}
	want := "# Widget: design\n\nChange 0042 — Widget\n\nBody paragraph.\n\n## Decisions\n\n1. One.\n"
	if string(got) != want {
		t.Fatalf("copy:\n%q\nwant\n%q", got, want)
	}
}

func TestSpecCopyBytesSkipsFencedH1(t *testing.T) {
	src := "```\n# not a title\n```\n\n# Real title\n\ntext\n"
	got, _ := specCopyBytes([]byte(src), specChange(t))
	if !strings.Contains(string(got), "# Real title\n\nChange 0042 — Widget\n\ntext\n") {
		t.Fatalf("change line not after the real title:\n%s", got)
	}
	if strings.Contains(string(got), "# not a title\n\nChange") {
		t.Fatalf("change line placed inside the fence:\n%s", got)
	}
}

func TestSpecCopyBytesNoTitle(t *testing.T) {
	got, _ := specCopyBytes([]byte("Just prose.\n"), specChange(t))
	if string(got) != "Change 0042 — Widget\n\nJust prose.\n" {
		t.Fatalf("no-title copy = %q", got)
	}
}

func TestSpecCopyBytesKeepsCRLF(t *testing.T) {
	src := strings.ReplaceAll(specFixture(), "\n", "\r\n")
	got, _ := specCopyBytes([]byte(src), specChange(t))
	if strings.Contains(strings.ReplaceAll(string(got), "\r\n", ""), "\n") {
		t.Fatalf("mixed line endings:\n%q", got)
	}
	if strings.Contains(string(got), "docket:backlink") {
		t.Fatalf("backlink survived:\n%q", got)
	}
}

func TestSpecCopyBytesRefusesMalformedMarkers(t *testing.T) {
	src := "<!-- docket:backlink:start (generated — do not hand-edit) -->\n# T\n"
	if _, err := specCopyBytes([]byte(src), specChange(t)); err == nil {
		t.Fatal("a dangling backlink marker must refuse, never be copied")
	}
}
```

`lifecycleChange`, `groomPath`, `snapshotOf`, and `mustChange` are existing untagged helpers in this package; if `snapshotOf` needs a different pin than the one it builds, keep it and adjust only the record bytes.

- [ ] **Step 2: Run to verify failure**

Run: `go test -count=1 -run 'SpecCopy' ./internal/app/`
Expected: FAIL — `undefined: specCopyBytes`.

- [ ] **Step 3: Implement the copy renderer**

In `internal/app/workspace_spec.go`:

```go
// specCopyChangeLine renders the one plain line the feature-branch spec copy
// carries after its title. It has no URL, so it never needs updating. It is the
// single seam a visibility mode would change (a private repository omits it).
func specCopyChangeLine(c domain.Change) string {
	return fmt.Sprintf("Change %04d — %s", int(c.ID()), c.Title())
}

// specCopyBytes renders the copy of a change's metadata spec that rides the PR:
// the spec without its docket:backlink block, with specCopyChangeLine inserted
// after the first top-level "# " title outside fenced code (or at the top when
// the spec has no title). Line endings follow the source. Malformed managed
// markers refuse — the copy is never built from a spec the parser rejects.
func specCopyBytes(spec []byte, c domain.Change) ([]byte, error) {
	doc, err := document.Parse(spec)
	if err != nil {
		return nil, fmt.Errorf("spec copy: %w", err)
	}
	le := doc.LineEnding()
	if le == "" {
		le = "\n"
	}
	body := spec
	if _, ok := doc.Block(backlinkBlockName); ok {
		var ps document.PatchSet
		ps.RemoveBlock(backlinkBlockName)
		if body, err = doc.Apply(ps); err != nil {
			return nil, fmt.Errorf("spec copy: removing backlink: %w", err)
		}
	}
	text := strings.TrimLeft(string(body), "\r\n")
	line := specCopyChangeLine(c)
	if end, ok := firstTitleLineEnd(text); ok {
		return []byte(text[:end] + le + line + le + text[end:]), nil
	}
	return []byte(line + le + le + text), nil
}
```

`firstTitleLineEnd(text string) (int, bool)` returns the offset just past the line terminator of the first line that starts with `"# "` outside fenced code, scanning with the same fence rules as `scanTopHeadings` (reuse its fence-run detection; do not write a second fence parser — extract a shared helper if needed). Run Step 2's tests; adjust only `firstTitleLineEnd`'s implementation until all five pass.

- [ ] **Step 4: Write the failing integration tests**

In `internal/app/workflow_integration_test.go`, prefix `TestIntegrationWorkflowLifecycle`:

- `TestIntegrationWorkflowLifecycleCommitSpecFirstCommit`: docket-mode repo (`newDocketModeRepo`) with an in-progress record (`lifecycleChange(31, "copyme", "in-progress")` with `spec:` set to `docs/superpowers/specs/2026-08-17-copyme-design.md`) and that spec (carrying a backlink block) on `docket`; `WorkspacePrepare`; `WorkspaceCommitSpec` → `Result == ResultApplied`, `Disposition == "committed"`; `git rev-list <base>..HEAD` in the workspace is exactly one commit whose subject is `Add design spec: A change` and whose only path is the spec path; file bytes equal `specCopyBytes(metadataSpec, change)`; the worktree is clean. A second call → `ResultNoOp`, `"current"`, head unchanged.
- `TestIntegrationWorkflowLifecycleCommitSpecRefreshesRevisedSpec`: after the first commit, advance the spec on `docket` (`repo.writerAdvance(t, "docket", ...)`), call again → `"refreshed"`, a second commit `Update design spec: A change`, file equals the new copy.
- `TestIntegrationWorkflowLifecycleCommitSpecTrivialNoSpec`: record with empty `spec:` → `ResultNoOp`, `"no-spec"`, no commit.
- `TestIntegrationWorkflowLifecycleCommitSpecRefusesDirtyWorkspace`: an untracked file in the workspace → `ResultInvalidState`, reason `workspace-not-ready`, no commit, the untracked file untouched.
- `TestIntegrationWorkflowLifecycleCommitSpecRefusesOccupiedIntegrationPath`: `main` already has different bytes at the spec path → `ResultInvalidState`, reason `spec-path-occupied`, no commit.
- `TestIntegrationWorkflowLifecycleCommitSpecMissingSpecFile`: `spec:` names a path absent on `docket` → reason `spec-missing`.

Run: `go test -tags integration -count=1 -run '^TestIntegrationWorkflowLifecycleCommitSpec' ./internal/app/` — expect FAIL (undefined).

- [ ] **Step 5: Implement the operation**

`WorkspaceCommitSpec`:
1. `loadWorkspaceContext(ctx, deps, repoDir, req.ID, OperationWorkspaceCommitSpec)`; refuse a non-in-progress change with `ReasonWorkspaceNotInProgress` (as prepare does).
2. `specPath := strings.TrimSpace(wc.change.Spec().Value)`; empty → `ResultNoOp` with `SpecCopyNoSpec`.
3. `resolveWorkspaceTarget`, then `wdeps.Service.Inspect`; `insp.Kind != workspace.StateReady` → `ResultInvalidState`, `ReasonSpecCopyNotReady`, message naming `insp.Kind` (never the dirty path contents).
4. Metadata spec: `deps.Reader.ReadArtifact(ctx, wc.pin, sourceMetadata, specPath)`; not found → `ReasonSpecCopyMissing`; read error → `ResultExternalFailed` via `classifyStatusError`.
5. `copyBytes, err := specCopyBytes(art.Data, wc.change)`; error → `ResultInvalidState`, reason `ReasonSpecCopyMalformed` (`"spec-malformed"`).
6. Integration probe: `deps.Reader.ReadArtifact(ctx, wc.pin, sourceIntegration, specPath)`; found with bytes different from `copyBytes` → `ReasonSpecCopyPathOccupied`.
7. Feature head probe: open `insp.HeadCommit` with `deps.Client.OpenObjectSource` and read `specPath` (`trackedRegularBlobBytes`). Equal bytes → `ResultNoOp`, `SpecCopyCurrent`. Absent → subject `"Add design spec: " + title`, disposition `committed`; present but different → `"Update design spec: " + title`, `refreshed`.
8. Fence: `done, ferr := admitWorkflowMutation(repoDir, OperationWorkspaceCommitSpec, nil)`; on error return `workspaceFenceRefusal(req.ID, ferr)`.
9. Write `filepath.Join(insp.Path, filepath.FromSlash(specPath))` (`os.MkdirAll` the parent, `os.WriteFile` 0o644), make an empty hooks dir with `os.MkdirTemp("", "docket-commit-spec-hooks-*")` (removed with `defer os.RemoveAll`), and `deps.Client.CommitPaths(ctx, wc.repo, gitcli.CommitRequest{Dir: insp.Path, Paths: []gitcli.RepoPath{gitcli.RepoPath(specPath)}, Subject: subject, HooksPath: hooks, When: deps.Clock.Now()})`. On a commit error, restore the prior bytes (or remove the file when it was absent) so the workspace is left clean, and return `ResultExternalFailed`.
10. `done(mutationJournalOutcome(result))`; return `ResultApplied` with `Head` = the new commit, `Path` = `specPath`.

Wire the CLI subcommand after `prepare` in `internal/cli/workspace.go` (`Use: "commit-spec"`, Short: "Commit the change's spec copy onto its feature branch (the branch's first commit)", flags `--id` required and `--repo-dir`), the `assetIndependent` row, and the schema-registry row. Update the CLI registration test and any capability-catalog golden.

- [ ] **Step 6: Run to verify pass**

Run: `go test -count=1 -run 'SpecCopy' ./internal/app/`; `go test -tags integration -count=1 -run '^TestIntegrationWorkflowLifecycle' ./internal/app/`; `go test -count=1 ./internal/cli/ ./internal/repoguard/`.
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/app/workspace_spec.go internal/app/workspace_spec_test.go internal/app/workspace_ops.go internal/app/schema_registry.go internal/app/workflow_integration_test.go internal/cli/workspace.go internal/cli/install.go <cli test / golden files>
git commit -m "feat(workspace): commit-spec ships the spec copy as the feature branch's first commit"
```

---

### Task 7: `change.attach-plan` writes the plan on the metadata branch

**Build tier:** premium — the attach contract every run depends on.

**Files:**
- Modify: `internal/app/change_attach.go`
- Modify: `internal/cli/change.go` (`changeAttachSubcommand` gains a kind-specific flag set: `attach-plan` takes `--id --revision --path --markdown`, no `--commit`; `attach-results` keeps `--commit` until Task 8)
- Modify: `internal/app/run_verify.go` (plan existence on the metadata tip)
- Modify: `internal/app/status.go` (`artifactChecks`: plan on metadata, integration fallback)
- Test: `internal/app/change_attach_test.go`, `internal/app/workflow_integration_test.go` (`TestIntegrationWorkflowRepoChangeAttachPlanGitVerification*`, `...AttachPlanBoardLinkAtomicity`, `...EffectiveBaseConsumedFromDomain`), `internal/app/workflow_e2e_test.go` (`runClaimToImplemented` step 5–6), `internal/app/claim_workflow_git_test.go` (`commitPlanFile` — delete when unused), `internal/app/named_isolation_integration_test.go`, `internal/app/finalize_e2e_test.go`, `internal/app/change_attach_git_integration_test.go`, `internal/app/status_test.go` (`TestStatusSourceDistinction`), `internal/app/run_verify_test.go`, `internal/cli/change_test.go`

**Interfaces:**
- Produces:

```go
// ChangeAttachRequest (Commit removed in Task 8; Markdown added here).
type ChangeAttachRequest struct {
	ID       int    `json:"id" docket:"required"`
	Revision string `json:"revision" docket:"required"`
	Path     string `json:"path" docket:"required"`
	Commit   string `json:"commit"` // results only, until Task 8
	// Markdown is the authored artifact body, read from --markdown at the CLI
	// boundary (a non-JSON file input, ADR-0138), never a JSON key.
	Markdown []byte `json:"-"`
}

func ChangeAttachPlan(ctx context.Context, deps PlanningDeps, repoDir string, req ChangeAttachRequest) ChangeAttachResult
// ChangeAttachResults keeps its WorkspaceDeps signature until Task 8.

const (
	ReasonAttachEmptyMarkdown   = "empty-markdown"
	ReasonAttachMarkdownTooLarge = "authored-input-too-large"
	ReasonAttachPathMismatch    = "artifact-path-mismatch" // the field already names a different path
	ReasonAttachPathOccupied    = "artifact-path-occupied" // a file at the path belongs to another change
)

// metadataArtifactBytes assembles the stored artifact: any submitted backlink
// block removed, the freshly rendered backlink prepended.
func metadataArtifactBytes(markdown []byte, backlink string) ([]byte, error)
```

- [ ] **Step 1: Write the failing unit tests**

In `internal/app/change_attach_test.go` (use its existing fake-engine/fake-reader kit; rename tests that no longer apply):

- `TestAttachPlanWritesFileFieldAndBacklink`: plan Markdown `"# Plan\n\n## Task 1\n\nDo it.\n"` → applied; the mutation plan declares two files: the record (with `plan:` set) and `docs/superpowers/plans/2026-08-17-widget-plan.md` whose bytes equal `assembleSpecFile(render.BacklinkContent(change, link), markdown)` (the same prepend shape the spec file uses).
- `TestAttachPlanStripsSubmittedBacklink`: Markdown that already starts with a backlink block (any interior) → stored bytes contain exactly one `docket:backlink:start` and its interior is the freshly rendered one.
- `TestAttachPlanRefusesMalformedBacklink`: a dangling `docket:backlink:start` → `ReasonAttachUnbalancedBacklink`, nothing declared.
- `TestAttachPlanReplacesContentOnReplan`: field already set to the path, file present with older content → one `MutationReplace` of the plan file; the record is not declared when its bytes are unchanged (same day).
- `TestAttachPlanRefusesDifferentPathWhenLinked`: field set to path A, request path B → `ReasonAttachPathMismatch`.
- `TestAttachPlanRefusesOccupiedPath`: field unset, a file exists at the path whose backlink targets another change → `ReasonAttachPathOccupied`.
- `TestAttachPlanIdenticalReattachIsNoOp`: same Markdown twice → second is `ResultNoOp` (keep the 0458 property).
- `TestAttachPlanPlaceholderStillRefuses`: a section whose body is only `TBD` → `ReasonAttachPlaceholderToken`.
- `TestAttachPlanRejectsBadShape`: empty Markdown → `ReasonAttachEmptyMarkdown`; over `maxAuthoredMarkdownBytes` → `ReasonAttachMarkdownTooLarge`; absolute / escaping / outside-root paths keep their reasons.

- [ ] **Step 2: Run to verify failure**

Run: `go test -count=1 -run 'AttachPlan' ./internal/app/`
Expected: FAIL.

- [ ] **Step 3: Implement the plan path**

In `changeAttach`, branch at the top: `if kind == attachKindPlan { return changeAttachMetadata(ctx, deps, repoDir, req, kind) }` (Task 8 routes results here too and deletes the old body). `changeAttachMetadata`:

1. Shape: `validateLifecycleShape("id", req.ID, req.Path, req.Revision)`; empty Markdown → `ReasonAttachEmptyMarkdown`; `len(req.Markdown) > maxAuthoredMarkdownBytes` → `ReasonAttachMarkdownTooLarge`.
2. Pin, `resolveBoardSurface`, `Discover`, `resolveAttachChange`, `verifyAttachPath` (unchanged).
3. If the kind's field is set and differs from `req.Path` → `ReasonAttachPathMismatch`.
4. `backlink, err := render.BacklinkContent(ac.change, ac.link)`; `artifact, err := metadataArtifactBytes(req.Markdown, backlink)` — `document.Parse(markdown)` (error → `ReasonAttachUnbalancedBacklink`), `PatchSet.RemoveBlock(backlinkBlockName)` when present, then `assembleSpecFile(backlink, string(stripped))` (move `assembleSpecFile` to a neutral name `prependBacklink` if you prefer, updating `change_groom.go` in the same commit).
5. Plan content: `planPlaceholderSlot(artifact)` → `ReasonAttachPlaceholderToken` (unchanged message).
6. Digest: `canonicalDigest(opKey, attachDigestPayload{Blob: sha256hex(artifact), ID: req.ID, Path: req.Path})` — keep the payload struct, rename the field's meaning in its comment to "the sha256 of the stored artifact bytes"; `RequestID: fmt.Sprintf("attach-%s-%d-%s", kind, req.ID, sha256hex(artifact)[:16])`.
7. Transaction: as today, plus `changeAttachOp` gains `artifactBytes []byte`. In `Plan`, before the record patch: `existing, _, exists, err := treeBlob(ctx, st.Tree, o.artifact)`; when `exists` and the field is not already `o.artifact`, require `backlinkTargets(existing, c, o.link)` to be true, else `refuseLifecycle(FindingCode(ReasonAttachPathOccupied), ...)`; declare the artifact file (`MutationCreate` when absent, `MutationReplace` when present and different, nothing when equal).

Delete for the plan kind: the `IsAncestor` descent check, `verifySingleArtifactDelta`, `verifyPathTrailer`, `planPathTrailerKey`, `resultsPathTrailerKey` (never used), `ReasonAttachCommitNotDescendant`, `ReasonAttachMultiArtifactDelta`, `ReasonAttachMissingTrailer`. Keep `readAttachBlob`, `verifyBacklink`, and `ReasonAttachCommitNotHead` / `ReasonAttachUntrackedFile` / `ReasonAttachSymlinkedPlan` / `ReasonAttachMissingBacklink` / `ReasonAttachBacklinkMismatch` / `ReasonAttachArtifactUnreadable` for the results kind until Task 8. Update the file-header comment to describe both profiles truthfully.

`ChangeAttachPlan` drops its `WorkspaceDeps` parameter. CLI: `attach-plan` reads `--markdown` with `readRecordSource(c.InOrStdin(), src)` and builds deps with `newPlanningDeps(repoDir)`; flag help: "plan Markdown `file`, or - for stdin; the operation writes it on the metadata branch with its backlink (required)".

- [ ] **Step 4: Move the plan readers to the metadata tip**

- `RunVerify`: replace the head-source `trackedRegularBlob(ctx, src, planPath)` with `art, err := deps.Reader.ReadArtifact(ctx, pin, sourceMetadata, planPath)` (error → `ReasonRunArtifactRead` operational refusal; `!art.Found` → `ReasonRunPlanMissing`). Keep the head source open only for results until Task 8. Update `ReasonRunPlanMissing`'s comment: "the linked plan no longer resolves to a file on the metadata branch".
- `artifactChecks`: the plan link checks `sourceMetadata` first and, only when absent there, `sourceIntegration` (a record closed before plans moved); `artifact-missing` only when both are absent. Update `TestStatusSourceDistinction` to expect the metadata ask for the plan, then the integration ask only when the fake reports it absent on metadata.

- [ ] **Step 5: Convert the workflow tests**

- `runClaimToImplemented` steps (5)–(6): no plan file in the workspace, no `ArtifactBacklink`, no commit; call `ChangeAttachPlan(ctx, node.deps, node.dir, ChangeAttachRequest{ID: id, Revision: ver(), Path: planPath, Markdown: []byte("# Implementation Plan\n\nConcrete steps here.\n")})`. Assert the plan file exists at the metadata remote tip with its backlink, and the workspace head is unchanged by planning.
- `TestIntegrationWorkflowRepoChangeAttachPlanGitVerification` (orphan/symlink/rename/commit-not-head/commit-not-descendant/multi-artifact-delta/missing-trailer cases): replace with the metadata-transaction cases (occupied path, path mismatch, malformed backlink) against real git, and delete the feature-commit cases; keep the HappyPath sibling, converted.
- `TestIntegrationWorkflowRepoEffectiveBaseConsumedFromDomain`: keep the stacked-base assertions; replace the plan half with a metadata attach (the ancestry property it pinned no longer exists — say so in a comment).
- `named_isolation_integration_test.go`, `finalize_e2e_test.go`, `change_attach_git_integration_test.go`: convert every plan attach the same way. Delete `commitPlanFile` once `git grep -n commitPlanFile` shows no caller.

- [ ] **Step 6: Run to verify pass**

Run: `go test -count=1 ./internal/app/ ./internal/cli/`; shards `TestIntegrationWorkflowRepo`, `TestIntegrationWorkflowLifecycle`, `TestIntegrationRecordOps`, `TestIntegrationNamed`, `TestIntegrationChangeRuntime`, `TestIntegrationEvidence`; `go test -tags e2e -count=1 -run '^TestE2E' ./internal/app/`.
Expected: PASS.

- [ ] **Step 7: Sweep and commit**

`git grep -n -F -- 'Docket-Plan-Path'` and `git grep -n 'planPathTrailerKey\|verifyPathTrailer\|verifySingleArtifactDelta'`: no executable or test hit remains (prose hits are Tasks 14–16).

```bash
git add <exact changed files>
git commit -m "feat(attach-plan): the plan is written on the metadata branch with its backlink"
```

---

### Task 8: `change.attach-results` writes results on the metadata branch; every results reader follows

**Build tier:** premium

**Files:**
- Modify: `internal/app/change_attach.go` (route results through `changeAttachMetadata`; delete the feature-head driver and the reasons/helpers only it used: `readAttachBlob`, `verifyBacklink`, `ReasonAttachCommitNotHead`, `ReasonAttachUntrackedFile`, `ReasonAttachSymlinkedPlan`, `ReasonAttachMissingBacklink`, `ReasonAttachBacklinkMismatch`, `ReasonAttachArtifactUnreadable`, `FCEmptyCommit` use; drop `Commit` from `ChangeAttachRequest`; drop `WorkspaceDeps` from `ChangeAttachResults`)
- Modify: `internal/cli/change.go` (`attach-results` takes `--markdown`, no `--commit`; collapse `changeAttachSubcommand` back to one flag set)
- Modify: `internal/app/change_implemented.go` (`verifyImplementedResults` reads at the metadata tip)
- Modify: `internal/app/run_verify.go` (results at the metadata tip; no head object source remains)
- Modify: `internal/app/status.go` (`artifactChecks` results: metadata, integration fallback)
- Test: `internal/app/change_attach_test.go`, `workflow_integration_test.go` (`...ChangeAttachResultsCheckpointContent`), `workflow_e2e_test.go` (steps 7–7b), `change_implemented_test.go` (`miAdvanceHead`, `miResults*` fixtures), `change_integration_test.go` (`TestIntegrationChangeRuntimeMarkImplementedConditions` condition-5 rows), `run_verify_test.go` (`rvResults*`), `run_verify_integration_test.go`, `named_isolation_integration_test.go`, `finalize_e2e_test.go`, `status_test.go`, `internal/cli/change_test.go`

**Interfaces:**
- Consumes: `changeAttachMetadata`, `metadataArtifactBytes` (Task 7).
- Produces: `func ChangeAttachResults(ctx context.Context, deps PlanningDeps, repoDir string, req ChangeAttachRequest) ChangeAttachResult`; `ChangeAttachRequest{ID, Revision, Path, Markdown}`.

- [ ] **Step 1: Write the failing tests**

- `change_attach_test.go`: `TestAttachResultsWritesFileAndField`, `TestAttachResultsCheckpointContentRefuses` (raw template scaffolding → `ReasonAttachResultsContent`), `TestAttachResultsLaterCheckpointReplacesContent`, `TestAttachResultsFeatureHeadNeverConsulted` (the request carries no commit; the fake workspace service records no Inspect call).
- `runClaimToImplemented`: the implementation commit carries only `widget.go`; results attach with `Markdown` before mark-implemented; mark-implemented and run.verify still reach `run-complete`. Add: a SECOND `ChangeAttachResults` after `EvidenceRecord` (new Outcome text) leaves the workspace head equal to `head` and `EvidenceVerify(evidenceBytes, head)` still verifies (spec acceptance 2, first half).
- `TestIntegrationChangeRuntimeMarkImplementedConditions`: the condition-5 rows seed results on the metadata branch (`results-missing` rows unchanged; `results-content-invalid`: a metadata results file failing the final contract; `results-identity-broken`: `results:` set but no file on the metadata tip, and a metadata file whose backlink targets another change).
- `run_verify_*`: results fixtures on the metadata branch; a results file present only at the feature head and absent on metadata → `results-identity-broken`.
- `TestStatusSourceDistinction`: results follow the plan rule.

- [ ] **Step 2: Run to verify failure**

Run: `go test -count=1 -run 'AttachResults|MarkImplemented|RunVerify|StatusSource' ./internal/app/`; shards `TestIntegrationChangeRuntime`, `TestIntegrationEvidence`, `TestIntegrationWorkflowLifecycle`, `TestIntegrationWorkflowRepo`.
Expected: FAIL.

- [ ] **Step 3: Implement**

- `changeAttach` routes both kinds to `changeAttachMetadata`; for results, the content check is `ValidateResultsContent(artifact, ResultsPhaseCheckpoint)` (unchanged reason and message). Delete the old driver body and everything listed under **Files**.
- `verifyImplementedResults(ctx, deps PlanningDeps, pin StatusPin, resultsPath string, ch domain.Change, link render.LinkContext, id int)`: `art, err := deps.Reader.ReadArtifact(ctx, pin, sourceMetadata, resultsPath)`; error → `results-identity-broken` with the error; `!art.Found` → `results-identity-broken` ("the attached results path %q is not a file on the metadata branch"); then the unchanged `backlinkTargets` and `ValidateResultsContent(ResultsPhaseFinal)` checks. The call site passes `pin`. Update condition 5's comment and `ReasonImplementedResultsIdentity`'s comment ("at the supplied head" → "on the metadata branch").
- `RunVerify`: results via `deps.Reader.ReadArtifact(ctx, pin, sourceMetadata, resultsPath)` with the same unmet mapping; remove the head `OpenObjectSource` block entirely; update `ReasonRunResultsIdentity`'s comment.
- `artifactChecks`: results metadata-first with integration fallback.
- CLI: one `changeAttachSubcommand` flag set for both verbs (`--id --revision --path --markdown`); Short texts: "Write a plan on the metadata branch and link it to an in-progress change" / "Write a results record on the metadata branch and link it to an in-progress change".

- [ ] **Step 4: Run to verify pass**

Re-run Step 2's commands plus `TestIntegrationRecordOps`, `TestIntegrationNamed`, `TestIntegrationRunCompletion`, `go test -tags e2e -count=1 -run '^TestE2E' ./internal/app/`, `go test -count=1 ./internal/cli/`.
Expected: PASS.

- [ ] **Step 5: Sweep and commit**

`git grep -n 'readAttachBlob\|ReasonAttachCommitNotHead\|Commit:.*ChangeAttachRequest\|--commit' -- internal/` → no attach-related executable or test hit remains.

```bash
git add <exact changed files>
git commit -m "feat(attach-results): results checkpoints are metadata commits; readers follow"
```

---

### Task 9: Retire `artifact.backlink`

**Build tier:** standard

**Files:**
- Delete: `internal/app/artifact_backlink.go`, `internal/app/artifact_backlink_test.go`, `internal/cli/artifact.go`, `internal/cli/artifact_test.go` (or the `TestArtifactCommandsRegistered` part if the file holds more). `artifact_backlink_test.go` defines `changeByPath`, `backlinkGolden`, and `backlinkDeps`; `workflow_integration_test.go` also calls some of them — move every still-called helper into a surviving `_test.go` file (for example `change_attach_git_helpers_test.go`) before deleting.
- Modify: `internal/cli/root.go` (or wherever `newArtifactCommand` is added), `internal/cli/install.go` (remove `"artifact"` and `"artifact backlink"` from `assetIndependent`), `internal/app/schema_registry.go` (remove the `artifact.backlink` row), `internal/repoguard/budgets_test.go` (the comment that cites the op)
- Keep: `backlinkBlockName` and `backlinkBlockAnnotation` — move them to `internal/app/change_attach.go` (they are shared by attach, groom, kill, close-out).

**Interfaces:** removes `ArtifactBacklink`, `ArtifactBacklinkRequest`, `ArtifactBacklinkResult`, `OperationArtifactBacklink`.

- [ ] **Step 1: Prove there is no caller**

Run: `git grep -n 'ArtifactBacklink\b\|ArtifactBacklink(\|artifact\.backlink\|artifact backlink' -- ':!docs/superpowers' ':!docs/results' ':!docs/changes' ':!testdata'`
Expected: hits only in the files this task deletes or edits, plus maintained prose (skills/agents/docs — Tasks 14–16). Any remaining Go caller is a defect from Tasks 7–8: convert it first.

- [ ] **Step 2: Delete and rewire**

Delete the files, the CLI registration, the `assetIndependent` rows, and the schema-registry row; move the two constants.

- [ ] **Step 3: Run the guards that track the command surface**

Run: `go build ./... && go test -count=1 ./internal/cli/ ./internal/app/ ./internal/repoguard/`
Expected: PASS — including the `assetIndependent` ↔ Cobra-tree correspondence test and the schema-registry coverage test. Fix any capability golden they name.

- [ ] **Step 4: Commit**

```bash
git add -A -- internal/app/artifact_backlink.go internal/app/artifact_backlink_test.go internal/cli/artifact.go internal/cli/artifact_test.go
git add <the other exact edited files>
git commit -m "refactor: retire artifact.backlink; the attach operations stamp their own backlinks"
```

---

### Task 10: Relative links, the legacy rule, `Spec (merged)`, and the PR-body plan/results links

**Build tier:** premium — every record's rendering changes and many goldens move.

**Files:**
- Create: `internal/render/relpath.go` (`RelativeLink`)
- Modify: `internal/render/artifacts.go` (`ArtifactBlockContent`, `pathRow`, `adrCell`; delete `lifecycleBranch`; add `ArtifactBacklinkContent`, `PRArtifactLinksContent`; `BacklinkContent` stays absolute for PR bodies)
- Modify: every file-backlink caller to `ArtifactBacklinkContent`: `internal/app/change_groom.go` (spec create, spec revise, `restampSpecBacklink`), `internal/app/change_kill.go`, `internal/app/finalize_closeout.go` (`retargetArtifactBacklinks`), `internal/app/change_attach.go` (`backlinkTargets` gains an `artifactPath` parameter; the attach writer), `internal/app/change_implemented.go` (`verifyImplementedResults` passes the results path)
- Keep absolute (`BacklinkContent`): `pr_publish.go`, `archivedBacklinkInterior` (PR leg), `repository_repair_prbacklinks.go`, `sweepAssessPRBacklinkLeg`
- Modify: `internal/app/change_kill.go` (`changeKillOp.Plan` retargets plan and results backlinks too, via the same per-artifact loop `retargetArtifactBacklinks` uses — reuse it, do not copy it)
- Modify: `internal/app/pr_publish.go` (`assemblePRBody(authored []byte, backlink, artifacts string)` upserts a `docket:artifacts` block after the backlink)
- Test: `internal/render/relpath_test.go` (new), `internal/render/artifacts_test.go`, `internal/render/testdata/artifacts/*` (read `PROVENANCE.md` first), `internal/app/*` tests that pin rendered rows or backlinks (find them with `git grep -n 'blob/docket\|blob/main\|blob/feat' -- 'internal/**/*_test.go'`), `internal/app/repository_repair_integration_test.go`, `internal/app/repocheck_corpus_link_integration_test.go`, `internal/app/finalize_closeout_integration_test.go`, `change_kill` tests

**Interfaces:**
- Produces:

```go
// RelativeLink returns the slash-separated relative path from the directory of
// fromFile to toFile, both canonical repo-relative paths.
func RelativeLink(fromFile, toFile string) string

// ArtifactBacklinkContent renders the docket:backlink block for a file on the
// metadata branch: "> ↩ **[Change NNNN — Title](<relative path to the record>)**".
func ArtifactBacklinkContent(c domain.Change, artifactPath string) string

// PRArtifactLinksContent renders the interior of the PR description's
// docket:artifacts block: absolute metadata-branch links to the plan and
// results. "" when RepoWebURL is empty or neither field is set.
func PRArtifactLinksContent(c domain.Change, link LinkContext) string
```

Row rules (`ArtifactBlockContent`), in order Spec, Spec (merged), Plan, Results, ADRs:
- **Spec:** `| Spec | [<base>](<RelativeLink(c.Path(), spec)>) |`.
- **Spec (merged):** only when `c.Status() == domain.StatusDone && c.HasBuildEvidence() && spec != ""`: `| Spec (merged) | [<base>](<BlobURLOnBranch(spec, IntegrationBranch)>) |`, or `` | Spec (merged) | `<spec>` | `` when `RepoWebURL` is empty.
- **Plan / Results:** legacy (`c.Status() == domain.StatusDone && !c.HasBuildEvidence()`) → today's done rendering (absolute on `IntegrationBranch`, backtick path without a web URL); otherwise relative.
- **ADRs:** `[ADR-NNNN](<RelativeLink(c.Path(), adr.Path())>)`.
Relative rows render the same with or without `RepoWebURL`.

- [ ] **Step 1: Write the failing render tests**

`internal/render/relpath_test.go`:

```go
package render

import (
	"path"
	"testing"
)

func TestRelativeLink(t *testing.T) {
	cases := []struct{ from, to, want string }{
		{"docs/changes/active/0001-a.md", "docs/superpowers/specs/s.md", "../../superpowers/specs/s.md"},
		{"docs/changes/archive/2026-10-06-0001-a.md", "docs/superpowers/specs/s.md", "../../superpowers/specs/s.md"},
		{"docs/superpowers/plans/p.md", "docs/changes/active/0001-a.md", "../../changes/active/0001-a.md"},
		{"docs/results/r.md", "docs/changes/active/0001-a.md", "../changes/active/0001-a.md"},
		{"docs/changes/active/0001-a.md", "docs/changes/active/0002-b.md", "0002-b.md"},
		{"README.md", "docs/x.md", "docs/x.md"},
	}
	for _, c := range cases {
		got := RelativeLink(c.from, c.to)
		if got != c.want {
			t.Errorf("RelativeLink(%q, %q) = %q, want %q", c.from, c.to, got, c.want)
		}
		if resolved := path.Join(path.Dir(c.from), got); resolved != c.to {
			t.Errorf("link %q from %q resolves to %q, not %q", got, c.from, resolved, c.to)
		}
	}
}
```

In `internal/render/artifacts_test.go` add (build changes with the file's existing change-fixture helper; set the body to carry `## Build evidence` via the decode path so `HasBuildEvidence` is true where needed):
- `TestArtifactBlockRelativeRowsSurviveArchive`: an in-progress change at `docs/changes/active/0007-x.md` and the same change at `docs/changes/archive/2026-10-06-0007-x.md` render byte-identical blocks, every link resolves (via `path.Join(path.Dir(recPath), link)`) to the field's path, and no row contains `/blob/` (spec acceptance 6).
- `TestArtifactBlockLegacyDoneKeepsIntegrationRows`: done, no evidence section → Plan/Results rows absolute on `main`; Spec and ADR rows relative; no `Spec (merged)` row.
- `TestArtifactBlockDoneWithEvidenceAddsSpecMerged`: done with the section → `Spec (merged)` row absolute on `main`; Plan/Results relative.
- `TestArtifactBacklinkContentRelative`: plan at `docs/superpowers/plans/p.md` → interior links `../../changes/active/0007-x.md`; after archive, `../../changes/archive/...`.
- `TestPRArtifactLinksContent`: both fields set → two lines with `/blob/docket/` URLs; no web URL → `""`; neither field → `""`.
Replace the lifecycle tests (`TestArtifactBlockLifecyclePinsPlanResults`, `TestArtifactBlockStackedMergedUsesFeatureBranch`, `TestArtifactBlockMissingBranchFallsBackToMetadata`, `TestArtifactBlockRelativeModeIgnoresLifecycle`) with the legacy-rule tests above; their premise (plan/results on the feature branch) no longer exists.

- [ ] **Step 2: Run to verify failure**

Run: `go test -count=1 ./internal/render/`
Expected: FAIL.

- [ ] **Step 3: Implement the renderer**

`RelativeLink`: split both paths' directory parts on `/`, drop the common prefix, emit one `..` per remaining `from` directory, then the rest of `to`. `ArtifactBlockContent` per the row rules; `pathRow` splits into `relativeRow(label, recPath, target)` and `absoluteRow(label, target, branch, link)`. `ArtifactBacklinkContent` uses the same marker constants as `BacklinkContent`. `PRArtifactLinksContent` lines: `- Plan: [<base>](<BlobURL(plan)>)` and `- Results: [<base>](<BlobURL(results)>)`. Update the package doc comments that say plan/results "never live on the metadata branch".

Goldens: follow `internal/render/testdata/artifacts/PROVENANCE.md`'s own remedy for an intentional output change (learning `config-edit-trips-its-own-frozen-drift-guard`: obey the guard). If it describes them as frozen parity copies of a retired Bash renderer, replace the affected goldens with the new Go output and record in `PROVENANCE.md` that the Go renderer is now their source and why.

- [ ] **Step 4: Switch the file-backlink callers and the kill retarget**

Change each caller listed under **Files** to `render.ArtifactBacklinkContent(change, <the file's own path>)`; `backlinkTargets(artifactBytes []byte, ch domain.Change, artifactPath string) (bool, error)`. `retargetArtifactBacklinks` renders per artifact path. `changeKillOp.Plan`: replace its spec-only block with a call to `retargetArtifactBacklinks` over the killed change's candidate. `assemblePRBody` gains the `artifacts` block (insert after the backlink block when absent, `ReplaceBlock` when present; omitted when the interior is `""`); `PRPublish` passes `render.PRArtifactLinksContent(change, link)`.

- [ ] **Step 5: Update the app-level pins and add the conversion test**

Run `go test -count=1 ./internal/app/` and every shard that renders records (`TestIntegrationChangeAuthoring`, `TestIntegrationChangeRuntime`, `TestIntegrationRecordOps`, `TestIntegrationWorkflowRepo`, `TestIntegrationWorkflowLifecycle`, `TestIntegrationFinalizeCloseout`, `TestIntegrationFinalizeArchive`, `TestIntegrationRepoCheck`, `TestIntegrationRepoRepair`, `TestIntegrationNamed`); update each failing expectation of absolute same-branch URLs to the relative form — never weaken an assert to a substring that would also match the old form. `TestIntegrationRepoCheckCorpusPinsDoneChangeToIntegrationBranch` stays true for a legacy done record; add a done-with-evidence record to it asserting relative Plan/Results and the `Spec (merged)` row.

New `TestIntegrationRepoRepairConvertsAbsoluteSameBranchLinksOnce` (prefix `TestIntegrationRepoRepair`): seed `docket` with (a) an active record whose `## Artifacts` block carries the old absolute spec/ADR URLs, and (b) a legacy done archived record with absolute Plan/Results rows on `main`. `RunRepositoryCheck` reports `artifact-links-stale` for (a) and for (b)'s spec/ADR rows; `RunRepositoryRepair` with `Authorized: true` lands exactly one metadata commit; afterwards (a) is relative, (b)'s Plan/Results rows still point at `main`; a second repair is a clean no-op (spec acceptance 6).

- [ ] **Step 6: Run to verify pass**

Re-run Steps 2 and 5 plus `go test -tags e2e -count=1 -run '^TestE2E' ./internal/app/`. Expected: PASS.

- [ ] **Step 7: Mutation-test the legacy rule**

Back up `internal/render/artifacts.go`, make the legacy predicate always false, run `go test -count=1 -run 'Legacy' ./internal/render/` → FAIL; restore with `mv -f`, re-run green.

- [ ] **Step 8: Commit**

```bash
git add <exact changed files>
git commit -m "feat(render): same-branch artifact links and backlinks are relative"
```

---

### Task 11: Retire the integration-branch backlink legs; pin "no integration push outside a PR merge"

**Build tier:** standard

**Files:**
- Modify: `internal/app/finalize_closeout.go` (delete `runCloseoutBacklinkLeg`, `closeoutBacklinkOp`, `closeoutBacklinkTargets`, `closeoutBacklinkTarget`, `closeoutBacklinkReceipt`, `OperationFinalizeCloseoutBacklink`, `ReasonCloseoutBacklinkPending`, the call in `FinalizeCloseout`; rewrite the file-header topology comment and `retargetArtifactBacklinks`'s "left to the follow-up leg" note)
- Modify: `internal/app/finalize_cleanup.go` (delete `finalizeCleanupBacklinkRepair`, `cleanupBacklinkOp`, `ReasonCleanupBacklinkPending`, its "Leg 1" call in `finalizeCleanupDone`; keep the PR-body "Leg 1b")
- Modify: `internal/app/maintenance_assess.go` (delete `sweepAssessBacklinkLeg`, `sweepBacklinkArtifactPaths`, `sweepLegBacklink` and its call in `sweepAssessHistorical`)
- Delete: `internal/app/finalize_backlink_loader.go` and `finalize_backlink_loader_test.go` once nothing uses `newBacklinkArtifactLoader`/`backlinkLegDetail` (if the PR leg uses `backlinkLegDetail`, move that function into `pr_backlink_repoint.go` instead of deleting it)
- Delete helpers whose last caller you removed: `backlinkLegRetarget`, `backlinkLegHasWork` (check `git grep` first)
- Modify: `internal/cli/finalize.go` (the cleanup annotation comment that calls it a metadata-branch leg), `internal/cli/maintenance.go` (comment)
- Modify: `internal/repoguard/retired_vocabulary_test.go` (the `terminal-backlink-pending` → `final-backlink-pending` row and the allowlist sentence: `final-backlink-pending` is now itself retired; follow the file's own convention for a retired code with no successor)
- Create: `internal/repoguard/integration_push_test.go`
- Test: `finalize_closeout_integration_test.go`, `finalize_cleanup_integration_test.go`, `finalize_state_integration_test.go`, `finalize_git_test.go` (`matrixCloseout`), `finalize_closeout_test.go` (`setupCloseoutFixture`), `maintenance_assess_test.go`, `maintenance_test.go`, `finalize_cleanup_test.go` (`TestFinalLifecycleCodeSpellings`)

**Interfaces:** consumes `retargetArtifactBacklinks` (relative, Task 10). Produces nothing new.

- [ ] **Step 1: Write the guard (it must fail first)**

`internal/repoguard/integration_push_test.go`:

```go
package repoguard

// TestNoIntegrationPushOutsidePRMerge pins that no operation writes the
// integration branch outside a PR merge. Two syntactic shapes are checked over
// every non-test Go file under internal/app:
//
//  1. Every transaction.Request composite literal's TargetRef expression must
//     mention MetadataBranchName — the engine may only commit to the metadata
//     branch.
//  2. Every direct PushLease / PushCreateLease call must sit in a function on
//     pushAllowlist. Each entry names why it may push; only migrateExecute
//     touches the integration branch, for the one-time human-run legacy
//     migration prune. A new direct pusher reddens this test and must argue its
//     way onto the list.
//
// Mutation-tested: re-adding a TargetRef of branchRefPrefix + integrationBranch
// reddens shape 1; adding a PushLease call to a new function reddens shape 2.
```

Implement with `go/parser` + `go/ast` over `filepath.Glob(<repo>/internal/app/*.go)` minus `_test.go` (resolve the repo root the way sibling repoguard tests do). For shape 1, match `*ast.CompositeLit` whose `Type` is a selector `transaction.Request`, find the `TargetRef` key, print its value with `go/printer`, and require `strings.Contains(src, "MetadataBranchName")`. For shape 2, walk each `*ast.FuncDecl`, find `*ast.CallExpr` whose `Fun` is a selector named `PushLease` or `PushCreateLease`, and require the FuncDecl name to be in:

```go
var pushAllowlist = map[string]string{
	"publishOrAdoptMetadataRoot": "repository init seeds the metadata branch",
	"executeRepositoryRepair":    "repository repair publishes one metadata commit",
	"reconcileResumeSeed":        "repository migrate resumes the metadata seed",
	"publishSeed":                "repository migrate publishes the metadata seed",
	"migrateExecute":             "repository migrate's one-time, human-run legacy prune of the integration branch",
}
```

Also assert the population floor: shape 1 found at least 15 literals and shape 2 found exactly the allowlisted functions (an allowlist entry with no call is stale and fails). Run: `go test -count=1 -run TestNoIntegrationPushOutsidePRMerge ./internal/repoguard/` → FAIL today on `finalize_closeout.go` and `finalize_cleanup.go`.

- [ ] **Step 2: Delete the legs**

Delete everything listed under **Files**. In `FinalizeCloseout`, the archive transaction's result plus the PR-body leg's findings are the whole outcome.

- [ ] **Step 3: Convert the tests**

- `setupCloseoutFixture`: plan and results live on `docket` (with relative backlinks to the active record), not on `main`.
- `TestIntegrationFinalizeCloseoutBacklinkLegDocketMode` → `TestIntegrationFinalizeCloseoutArchiveRetargetsAllArtifactBacklinks`: the single archive commit on `docket` retargets spec, plan, and results backlinks to the archived path, and `origin/main`'s tip after close-out equals the merge commit — no integration commit (spec acceptance 4, behavioral half).
- Delete `...BacklinkLegIgnoresUnrelatedCorpusErrors`, `...BacklinkPendingFindingNamesTheCause`, `TestIntegrationFinalizeCleanupBacklinkRepairIgnoresUnrelatedCorpusErrors`, the backlink-leg sweep assessment tests (`TestAssessStaleBacklinkLegIsActionable`, `TestAssessMissingManifestWithStaleBacklinkIsActionable`, and the malformed-markers case that targets the integration leg); keep and adjust the PR-backlink assessment tests. Record each deletion and its reason in the commit body.
- `TestIntegrationFinalizeArchiveNeverEditsAuthoredBytes`, `TestIntegrationFinalizeStateBytePreservation`, `matrixCloseout`: read plan/results on `docket`; assert no integration commit on replay.
- `TestFinalLifecycleCodeSpellings`: drop the two retired constants.

- [ ] **Step 4: Run to verify pass**

Run: `go test -count=1 ./internal/app/ ./internal/repoguard/ ./internal/cli/`; shards `TestIntegrationFinalizeCloseout`, `TestIntegrationFinalizeArchive`, `TestIntegrationFinalizeCleanup`, `TestIntegrationFinalizeState`, `TestIntegrationSweep`; `go test -tags e2e -count=1 -run '^TestE2E' ./internal/app/`.
Expected: PASS.

- [ ] **Step 5: Mutation-test the guard**

Back up `internal/app/finalize_block.go`, change one `TargetRef` to `gitcli.RefName(branchRefPrefix + "main")`, run the guard → FAIL; restore with `mv -f`. Back up `internal/app/repository_repair.go`, wrap its `PushLease` call in a new helper function → FAIL; restore. Re-run green.

- [ ] **Step 6: Sweep and commit**

`git grep -n 'final-backlink-pending\|closeout-backlink\|final backlinks retargeted' -- ':!docs/superpowers' ':!docs/results' ':!docs/changes' ':!testdata'` → only maintained prose hits remain (Tasks 14–16).

```bash
git add <exact changed and deleted files>
git commit -m "refactor(finalize): no post-merge backlink commit to the integration branch"
```

---

### Task 12: Retire `finalize.skip_results_only_delta`

**Build tier:** standard

**Files:**
- Modify: `internal/config/schema.go` (the `finalize.skip_results_only_delta` row: `disp: dispObsolete`, mirroring the `metadata_branch` row's shape)
- Modify: `internal/config/decode.go` (`decodeLeaf` per-path switch: add a case mirroring `case "metadata_branch":` — an `obsolete-setting` warning whose remedy says "remove finalize.skip_results_only_delta from <layer>")
- Modify: `.docket.yml` (delete the comment block beginning "Arms the gate's docs-only post-gate skip" and the `skip_results_only_delta: false` line)
- Test: `internal/config/capability_test.go`, `decode_test.go`, `fixtures_test.go`, `preflight_test.go`, `resolve_test.go`, `schema_test.go`, `setting_paths_test.go`; `internal/app/config_test.go`, `status_corpus_test.go`; `internal/app/finalize_e2e_test.go` (`TestE2EUnsupportedConfigRefused`: switch to another `dispDeferred` key from `schema.go`, e.g. the next deferred bool row)
- Modify: `docs/release/upgrading-from-bash.md` (its `skip_results_only_delta` row now says it is obsolete and ignored with a warning; remedy "Remove it.")

- [ ] **Step 1: Write the failing decode test**

In `decode_test.go` beside the `metadata_branch` obsolete case: a layer setting `finalize: {skip_results_only_delta: true}` decodes with one `obsolete-setting` diagnostic, classification Obsolete, no blocker, and `PreflightMutation` allows writes. Run `go test -count=1 -run 'Obsolete|SkipResults' ./internal/config/` → FAIL.

- [ ] **Step 2: Implement** the row change and the decode case. Run the whole `./internal/config/` package; update each test that pinned the deferred classification (`deferred-setting` notice for `false`, `deferred-capability-requested` blocker for `true`) to the obsolete classification. Frozen corpora under `testdata/` stay untouched; a test that loads one and counted deferred notices now counts an obsolete warning instead — update the count, never the corpus.

- [ ] **Step 3: Run**

`go test -count=1 ./internal/config/ ./internal/app/ ./internal/repoguard/` and `go test -tags e2e -count=1 -run '^TestE2EUnsupportedConfigRefused' ./internal/app/`. `TestLivingDocsAlignment` must still pass (the docs row no longer describes the key as unsupported-blocking). Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/config/schema.go internal/config/decode.go .docket.yml docs/release/upgrading-from-bash.md <updated tests>
git commit -m "refactor(config): finalize.skip_results_only_delta is an obsolete tombstone"
```

---

### Task 13: End-to-end acceptance pins

**Build tier:** standard

**Files:**
- Modify: `internal/app/workflow_e2e_test.go` (`runClaimToImplemented`)
- Modify: `internal/app/finalize_e2e_test.go` (`TestE2EOrdinaryFinalize`)

**Interfaces:** consumes Tasks 2–11.

- [ ] **Step 1: Extend `runClaimToImplemented`**

The record fixture `buildReadyChange(id, slug)` gains `spec:` pointing at a spec seeded on `docket` (with a backlink block). Insert, right after `WorkspacePrepare`, `WorkspaceCommitSpec` → `committed`. After `PRPublish`, assert (spec acceptance 1):

```go
diff := runGit(t, wp, "diff", "--name-only", "--no-renames", prep.BaseCommit+".."+head)
if got := strings.Fields(diff); !reflect.DeepEqual(got, []string{specPath, "widget.go"}) {
	t.Fatalf("PR diff = %v, want exactly the spec copy and the code", got)
}
first := runGit(t, wp, "rev-list", "--reverse", prep.BaseCommit+".."+head)
if strings.Fields(first)[0] != specCommit { // specCommit from the WorkspaceCommitSpec result's Head
	t.Fatalf("the spec copy is not the feature branch's first commit")
}
```

After mark-implemented, assert the plan file, the results file, and the record's `## Build evidence` section all exist at the metadata remote tip (`blobRevisionAt` / a blob read on `m.branch`). Keep the Task 8 post-gate results checkpoint and add: the record evidence still verifies against `head` (`VerifyRecordEvidence`). Extend `assertEngineOnlyMetadataCommits`'s operation list only if a new metadata operation key now appears in this flow (it should not: `workspace.commit-spec` is a feature-branch commit).

- [ ] **Step 2: Extend `TestE2EOrdinaryFinalize`**

Seed a post-gate results checkpoint (a second `ChangeAttachResults` after evidence) before finalize; assert finalize's rebase reports the no-op skip (`Gate.Compose == gateComposeSkipped`) — spec acceptance 2, second half — and that `origin/main` after close-out holds the merge commit plus nothing else (no backlink commit).

- [ ] **Step 3: Run**

`go test -tags integration -count=1 -run '^TestIntegrationWorkflowLifecycle' ./internal/app/`; `... -run '^TestIntegrationNamed' ...`; `go test -tags e2e -count=1 -run '^TestE2E' ./internal/app/`. Expected: PASS. If an assert passes on the first run, mutation-check it once (e.g., temporarily commit a plan file in the workspace before publishing → the diff assert must FAIL), then restore.

- [ ] **Step 4: Commit**

```bash
git add internal/app/workflow_e2e_test.go internal/app/finalize_e2e_test.go
git commit -m "test(workflow): pin the spec-plus-code PR and the post-gate results checkpoint"
```

---

### Task 14: `docket-implement-next` prose

**Build tier:** standard

**Files:**
- Modify: `skills/docket-implement-next/SKILL.md`, `skills/docket-implement-next/references/edge-paths.md`, `skills/docket-implement-next/references/fix-pass.md`, `skills/docket-implement-next/results-template.md`
- Regenerate: `internal/assets/embedded/tree/skills/docket-implement-next/**` (`go generate ./internal/assets/`)
- Test: `internal/repoguard/prose_contracts_test.go` (only where a pinned phrase must change — keep every pinned clause that is still true verbatim), `internal/repoguard/budgets_test.go` (only if a ceiling must move)

Describe current behavior only. Rewrite (sites from the prose map; re-derive with `git grep -n -i 'feature branch\|Docket-Plan-Path\|artifact.backlink\|build-evidence\|PR body\|post-gate' skills/docket-implement-next`):

- [ ] **Step 1: Step 0 and Step 4**
  - Step 0: bookkeeping on `docket` includes the plan, results, and build evidence; "only the spec copy and the code land on the feature branch".
  - Step 4: after `workspace.prepare`, run the `workspace.commit-spec` operation with `--id <id>` (it commits the spec copy as the branch's first commit; a trivial change reports `no-spec`; a `spec-path-occupied` or `workspace-not-ready` refusal halts). Record HEAD after it as the pre-dispatch HEAD.
  - The `docket:feature-dispatch` block for `docket-plan-writer` keeps the exact `Feature worktree: <absolute canonical feature-worktree root>` line; its payload adds the exact record revision; the child authors the plan in a scratch file outside both worktrees and runs `change.attach-plan` with `--id <id> --revision <revision> --path docs/superpowers/plans/<date>-<slug>.md --markdown <file>`, which writes the plan with its backlink on `docket` and sets `plan:` in one commit; it returns `PLAN_PATH=<repo-relative-path>`.
  - **Verification (parent):** re-sync `.docket/` (`repository.prepare`), then verify from git on the metadata branch that `plan:` equals the returned path and the file exists at the metadata tip with a backlink to this change, and that the feature worktree is clean with HEAD equal to the pre-dispatch HEAD. Delete every trailer, single-file-delta, and "plan file merges with the code" clause.
  - **Continue:** delete "at close-out `finalize.closeout` re-points the merged copy"; the archive transaction retargets every metadata backlink in its own commit.
  - Build workers and reviewers read the plan from the metadata worktree path (`.docket/<plan path>`, after a re-sync).
- [ ] **Step 2: Step 6 and Step 6.5**
  - Results are authored from `results-template.md` in a scratch file; the current content is read from `.docket/<results path>` after a re-sync; each checkpoint is one `change.attach-results` call with `--id --revision --path --markdown`, a metadata commit that never moves the feature head. Replace the six per-checkpoint mechanics with: verify ownership/quiescence; read and preserve prior content; write the update; attach. Delete "commit only the results path", "publish through the feature-branch transport", and the "local commit vs remote publication" paragraph.
  - **Evidence sequencing:** results prose is never gate evidence, and a results checkpoint never moves HEAD, so it never stales evidence; obtain gate evidence for the final code head; any code commit (a fix) still invalidates evidence. Delete the "post-gate results edit invalidates stale evidence" sentence. Keep "never commit or move HEAD beneath a live gate, a running worker, or a transferred drive" verbatim (pinned) — it still governs code commits.
  - Keep every pinned clause of `TestResultsReviewPlacementDocContracts` verbatim (the review disposition table stays in the PR body).
- [ ] **Step 3: Step 7, postconditions, branch discipline**
  - PR-body assembly: the authored body carries the `#<issue>` reference and the backlink line; `pr.publish` adds the plan/results links block; the PR body carries no build-evidence block. `pr.publish` still requires `--evidence` and verifies it against the head.
  - Mark implemented: it also records the verified evidence as the record's `## Build evidence` section in the same transaction; the results artifact is read from the metadata branch.
  - Postcondition rows: row 4 → "the spec copy committed as the first commit on `<type>/<slug>` (non-trivial changes), then `plan:` and the plan file with its backlink landed on the `docket` branch, the feature HEAD unchanged by planning"; row 7 → "… `results:` set and the final results file with its backlink on the `docket` branch, and the record's `## Build evidence` section naming the PR head". Delete the "a later commit moving branch HEAD (Step 6.5's results file)" staleness sentence.
  - Field-write rule and feature-branch invariants: the feature branch adds only the spec copy and the code; plan and results files are metadata writes through their attach transactions.
- [ ] **Step 4: `edge-paths.md`, `fix-pass.md`, `results-template.md`**
  - Plan seam: `plan:` set → planning is complete (the attach wrote file and field together); unset → re-dispatch the plan-writer. Results seam: reuse the `results:` path when set; never mint a second file. Resume after a mid-flight spec revision: re-run `workspace.commit-spec` before building continues (it refreshes the copy).
  - Delete the "Build-evidence block" paragraph of `## PR-body assembly (Step 7)` and its expected-staleness rule; keep the issue-reference and backlink paragraphs; add one sentence on the plan/results links block.
  - `fix-pass.md`: keep "the **PR body remains the disposition table's durable home**"; replace "the block `docket-finalize-change` reads" with a statement that build evidence lives in the change record; keep "The results file never carries the machine build-evidence block".
  - `results-template.md` header comment: authored by the coordinator, written on the `docket` branch through `change.attach-results` at each checkpoint; the backlink block is owned by that operation.
- [ ] **Step 5: Regenerate and run the guards**

```bash
go generate ./internal/assets/
go test -count=1 ./internal/assets/ ./internal/repoguard/
```

Expected: PASS (`TestEmbeddedMatchesAuthored`, `TestProseContracts`, `TestResultsReviewPlacementDocContracts`, `TestFollowUpBacklogMatchDocContracts`, `TestFeatureDispatchPayloadsCarryCanonicalWorktree`, `TestSkillSizeBudgets`, `TestLivingDocsAlignment`, `TestCapabilitySurface`). A size breach is fixed by tightening prose first; re-baseline a row only with a reason.

- [ ] **Step 6: Commit**

```bash
git add skills/docket-implement-next internal/assets/embedded <any guard test you had to update>
git commit -m "docs(implement-next): plan, results, and evidence on the metadata branch; the spec copy rides the PR"
```

---

### Task 15: Agents, cursor rules, and the other skills

**Build tier:** standard

**Files:**
- Modify: `agents/docket-plan-writer.md`, `cursor-rules/dispatch/docket-plan-writer.md`
- Modify: `skills/docket-convention/SKILL.md`, `skills/docket-convention/references/close-out.md`, `skills/docket-convention/references/stacked-changes.md`
- Modify: `skills/docket-build/SKILL.md`, `skills/docket-finalize-change/SKILL.md`, `skills/docket-status/SKILL.md`, `skills/docket-review/SKILL.md` (only if it calls the evidence record's home the PR body)
- Regenerate: `internal/assets/embedded/tree/**` and the harness goldens for `docket-plan-writer` (`internal/harness/{claude,cursor,opencode}/testdata/golden/docket-plan-writer.md`, `internal/harness/codex/testdata/golden/docket-plan-writer.toml`)

- [ ] **Step 1: `docket-plan-writer`** (agent + cursor dispatch copy). Keep `worktree-scope: feature` (it reads the feature tree and verifies it stays clean). Contract: owns exactly one durable artifact, the plan file on the metadata branch, written through exactly one docket operation, `change.attach-plan` (argv from the capability catalog); writes nothing in either worktree; the payload carries the record revision. Sequence: (1) confirm the feature worktree is clean at the pre-dispatch HEAD; (2) read change, spec, feature code, learnings; (3) invoke `superpowers:writing-plans` DIRECTED to write the plan to a scratch file made with `mktemp -d "${TMPDIR:-/tmp}/docket-plan.XXXXXX"` and stop there; (4) missing-skill fallback; (5) run `change.attach-plan` with `--id --revision --path docs/superpowers/plans/<date>-<slug>.md --markdown <scratch file>`; a refusal is a blocking diagnostic; (6) re-confirm the feature worktree is clean at the pre-dispatch HEAD; (7) `PLAN_PATH=<repo-relative-path>`. The description frontmatter changes to match ("writes the plan artifact with its backlink on the metadata branch through change.attach-plan").
- [ ] **Step 2: `docket-convention`**
  - Agent-layer exceptions sentence: the plan-writer performs exactly one docket metadata operation (`change.attach-plan`); the others perform none. Composition: the plan-writer's proof is git state on the `docket` branch (`plan:` set, the plan file with its backlink at the tip).
  - Role table: plan → "a plan file on the `docket` branch, recorded in `plan:`".
  - Directory layout: `<results_dir>/` comment → "on the `docket` branch" (keep the pinned clause "required close-out artifacts (one per implemented change, trivial included)" verbatim); add the plans directory if the layout lists specs.
  - Results artifact shape: the results file and `results:` field are written together by `change.attach-results` on the `docket` branch.
  - Manifest comments for `plan:` and `results:`.
  - Frozen-artifact rule → "Closed-out plans and results are frozen build records." The old sentence `Merged plans and results are frozen build records.` is pinned by `test_results_artifact` in `prose_contracts_test.go`; its claim is now false (plans no longer merge), so change that pin to the new sentence in this task. The only later writes re-stamp the backlink, done by the archive transaction.
  - `## Artifacts` bullet: relative links for same-branch artifacts; the attach operations and `change.groom` write file backlinks; `pr.publish` writes the PR body's (absolute), repointed at archive.
  - Derived views paragraph: drop `artifact.backlink`.
  - Branch model: "the feature branch adds the spec copy and the code"; "the integration branch gets code and specs through PRs alone".
  - `close-out.md`: the archive commit re-renders `## Artifacts` (relative rows, plus `Spec (merged)`) and re-stamps every metadata backlink — spec, plan, results; nothing touches the integration branch. `stacked-changes.md`: "carries only the spec copy and the code".
  - Missing-skill rule's "in the PR body" for plan/build/review stays (it is about warnings, not evidence).
- [ ] **Step 3: `docket-build`, `docket-finalize-change`, `docket-status`**
  - Build: Inputs → the plan at `.docket/<plan path>`; the controller's build checkpoint and the time-limit audit line go to the results file through `change.attach-results`; "Step 7 writes it into the PR body" → "`change.mark-implemented` records it in the change record"; Output's "(and the PR description where evidence belongs)" → "(and the change record's build evidence)".
  - Finalize: §3/§4 — the no-op skip reads green build evidence for the exact head **from the change record's `## Build evidence` section**, recorded against the resolved `finalize.test_command`; keep "There is no strict-ancestor or results-only skip." (still true and pinned); §7 — `finalize.publish` records the rebased head's evidence in the change record; §9 — delete the integration-ref leg and `final-backlink-pending`; close-out is one metadata commit plus the best-effort PR-description backlink repoint; §10 — cleanup repairs only the PR description's backlink.
  - Status: `artifact-missing` (pinned) is reported when a linked plan or results file is on neither the metadata branch nor (for records closed before the move) the integration branch.
- [ ] **Step 4: Regenerate and run**

```bash
go generate ./internal/assets/
go test -count=1 ./internal/harness/... -update   # only the golden tests; read the flag first
go test -count=1 ./internal/assets/ ./internal/harness/... ./internal/repoguard/
```

Expected: PASS. Commit:

```bash
git add agents/docket-plan-writer.md cursor-rules/dispatch/docket-plan-writer.md skills internal/assets/embedded internal/harness internal/repoguard/prose_contracts_test.go
git commit -m "docs(skills): the plan-writer attaches on the metadata branch; evidence lives in the record"
```

---

### Task 16: Living docs and the final sweep

**Build tier:** standard

**Files (derive the final list with the Step 1 grep; these are known):** `README.md`; `docs/guide/where-the-metadata-lives.md`, `landing-changes.md`, `proving-the-build.md`, `building-without-supervision.md`, `reviewing-before-the-human.md`, `keeping-the-backlog-honest.md`; `docs/concepts/two-branches.md`, `build-tiers-and-gate.md`, `finalize-sequencer.md`, `skills-agents-dispatch.md`, `run-tracker.md`; `docs/reference/glossary.md`, `skills-and-agents.md`, `outcomes.md`, `fields.md` (only if it describes a location).

- [ ] **Step 1: Derive the sites**

```bash
hits=$(git grep -n -i -E 'feature branch|plan and results|plans, and results|build evidence|build-evidence|pull request body|PR body|PR description|artifact\.backlink|artifact backlink|final-backlink-pending|Docket-Plan-Path|retarget' -- README.md docs/guide docs/concepts docs/reference docs/install)
printf '%s\n' "$hits"
```

Sort each hit: true today (leave) or describes removed behavior (rewrite).

- [ ] **Step 2: Rewrite** to current behavior only, no change numbers, no mention of private visibility:
  - Where artifacts live: change record, ADRs, learnings, board, spec, plan, results, and build evidence on the metadata branch; the integration branch receives the spec copy and the code through the pull request; nothing is pushed to it at close-out. Update the `where-the-metadata-lives.md` table to the spec's "After" table.
  - Build evidence: lives in the change record's `## Build evidence` section; `finalize publish` and `evidence recertify` refresh it there; a results checkpoint never moves the feature head, so it never forces a retest.
  - Close-out: one metadata commit archives the record and retargets every artifact backlink; the merged PR's description backlink is repointed best-effort.
  - Links: same-branch links in records and artifacts are relative; `docket repository repair` converts older absolute ones.
  - Glossary entries: Archived record, Derived view/backlink, Frozen build record, Plan, Results, Build evidence, Plan writer (attaches through `change attach-plan`), PR publish (no evidence block; adds plan/results links), Re-certify, Run verify (drop the stale "verify-run" alias line if it is the only claim left there), Finalize gate, Finalize publish; remove the `artifact.backlink` entries.
- [ ] **Step 3: Run the docs guards**

`go test -count=1 -run 'LivingDocs|Alignment|ProseContracts|CapabilitySurface' ./internal/repoguard/` → PASS.

- [ ] **Step 4: Final whole-repo sweep**

For each retired token — `Docket-Plan-Path`, `Docket-Results-Path`, `artifact.backlink`, `artifact backlink`, `final-backlink-pending`, `closeout-backlink`, `skip_results_only_delta`, `prBodyEvidence`, `lifecycleBranch`, `commitPlanFile` — run `git grep -n -F -- '<token>'` and confirm every remaining hit is a point-in-time record (`docs/superpowers/`, `docs/results/`, `docs/changes/`, `docs/adrs/`), a frozen `testdata/` corpus, a retired-vocabulary guard row, or the obsolete-key tombstone. List the residual classification in the commit body.

- [ ] **Step 5: Commit**

```bash
git add README.md docs/guide docs/concepts docs/reference
git commit -m "docs: build artifacts live on the metadata branch; the spec ships with the PR"
```

---

## Spec coverage (self-review)

| Spec item | Task |
|---|---|
| §1 plan on the metadata branch; attach-plan transaction; refusals removed; receipt/verification on metadata | 7, 14, 15 |
| §2 results on the metadata branch; each checkpoint a metadata commit; readers at the metadata tip | 8, 13, 14 |
| §3 `## Build evidence` section; writers (`finalize.publish`, `evidence.recertify`, `mark-implemented`); readers; PR body carries none; catalog effects | 1–5 (`pr.publish` narrowing: Global Constraints) |
| §4 spec copy as first commit; change line seam; refresh after re-groom; trivial; occupied path; `Spec (merged)` row | 6, 10, 13 |
| §5 relative links; backlinks retargeted in the archive commit; PR rows absolute; PR plan/results links; one-time conversion via check/repair; status fallback | 7, 8, 10 |
| §6 retire integration legs, trailer, `Docket-Results-Path`, `skip_results_only_delta`, `artifact.backlink` | 7, 9, 11, 12 |
| §7 skills, agents, embedded twins, prose pins | 14, 15 |
| §8 living docs | 16 |
| Acceptance 1 (PR diff = spec copy + code; artifacts on metadata) | 13 |
| Acceptance 2 (post-gate results checkpoint keeps head; no finalize retest) | 8, 13 |
| Acceptance 3 (record-read parity; removing the read reddens) | 4 |
| Acceptance 4 (no integration push outside a PR merge, pinned) | 11 |
| Acceptance 5 (spec copy first commit, content, refresh) | 6, 13 |
| Acceptance 6 (relative links survive archive; repair converts once; second run no-op) | 10 |
| Acceptance 7 (whole suite green; docs guards) | 16 + docket-build's final gate |
| ADR expected | implement-next Step 6 (`docket-adr`), not a build task |

## Human verification items (for the results file)

- **Important — install ordering (spec Cutover).** Every change that is `in-progress` or `implemented` when this merges must finish (merge and close out) on the previous binary before the new binary is installed. A change caught mid-flight recovers with `docket evidence recertify` (writes the record section) and re-attaches its plan/results through the new attach operations.
- **Important — one-time link conversion.** After installing, run `docket repository check`: expect `artifact-links-stale` warnings for records with absolute same-branch links. Run `docket repository repair`, review the preview, apply with `--yes`, and re-run `repository check` to confirm the warnings are gone and archived records closed before this change still link their plan/results on `main`.
- **Optional — first run on the new flow.** On the next implement-next run, confirm the PR's "Files changed" tab shows only the spec copy and code, and that the change record on `docket` shows the plan, results, and `## Build evidence` section.
