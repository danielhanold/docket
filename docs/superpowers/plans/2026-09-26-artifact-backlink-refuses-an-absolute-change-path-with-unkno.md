<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0460 — artifact.backlink refuses an absolute --change path with unknown-change](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-26-0460-artifact-backlink-refuses-an-absolute-change-path-with-unkno.md)**
<!-- docket:backlink:end -->
# artifact.backlink `--change` Path Validation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `artifact.backlink` refuse a malformed `--change` path with a typed, form-naming message instead of the misleading `unknown-change`, and fix the caller prose that produced the absolute path.

**Architecture:** Add a purely lexical validation of `req.ChangePath` inside `ArtifactBacklink` (`internal/app/artifact_backlink.go`), mirroring the four checks `verifyAttachPath` (`internal/app/change_attach.go`) already enforces — copied inline as a small private helper, so the attach operations' observable reasons and messages are untouched. The check runs after the artifact read/parse and before the corpus read, so every refusal still predates any write and a malformed artifact is still reported first. Reuse the existing `ReasonBacklinkAbsolutePath` / `ReasonBacklinkPathEscape` constants — no new reason codes. Separately, state the repo-relative path form in the two `skills/docket-implement-next/SKILL.md` sentences that pass `artifact.backlink` flags with the form unstated, and regenerate the embedded asset tree that byte-copies that skill.

**Tech Stack:** Go (stdlib `path`, `path/filepath`), Go tests in `internal/app`; `go generate ./internal/assets/` for the embedded asset bundle.

**Spec:** `docs/superpowers/specs/2026-09-25-artifact-backlink-refuses-an-absolute-change-path-with-unkno-design.md` (on the `docket` metadata branch; synchronized copy at `.docket/docs/superpowers/specs/…` from the primary checkout)

## Global Constraints

- Accepting absolute paths on any flag is out of scope; the fix is a clear refusal, never a second accepted path form.
- Reuse `ReasonBacklinkAbsolutePath` (`"absolute-path"`) and `ReasonBacklinkPathEscape` (`"path-escape"`); mint no new reason codes.
- Every refusal must happen before any write: a refused call leaves the artifact byte-identical.
- The attach operations' (`change.attach-plan` / `change.attach-results`) observable reasons and messages must not change — `verifyAttachPath` is a reference, not an edit target.
- Each new message names the flag (`--change`) and the expected form (canonical repository-relative), e.g. `docs/changes/active/<id>-<slug>.md`.
- A well-formed path that matches no record still returns `unknown-change`, with its existing message.
- Path handling in other operations, and the gate-drive `scope-closed` issue from the 0458 run, are out of scope.
- Point-in-time records (archived changes, results files, specs, old plans) keep their existing wording — only maintained caller prose is edited (repo rule: rewriting point-in-time records falsifies history).

## Review Focus

Checked the spec's input space against the tasks below; each line's test is pinned into Task 1 (test table) as noted:

1. An **absolute path that names the real change file** (the exact 0458 shape, `/Users/…/docs/changes/active/<id>-<slug>.md` where the repo-relative tail would match a record) must refuse `absolute-path`, never resolve — Task 1 table case `absolute`.
2. A **whitespace-only** `--change` value (`"  "`) must refuse `path-escape` like empty, not panic or reach the corpus read — Task 1 table case `whitespace-only`.
3. An **interior `..` that does not escape** (`docs/changes/active/../active/0315-claim.md`) is lexically local (`filepath.IsLocal` accepts it) but non-canonical — it must be caught by the `path.Clean` check as `path-escape`, not fall through to `unknown-change` — Task 1 table case `interior-dotdot`.
4. A **trailing slash** (`docs/changes/active/0315-claim.md/`) is a non-canonical spelling of a real record's path and must refuse `path-escape`, not resolve and not report `unknown-change` — Task 1 table case `trailing-slash`.
5. **Refusal-before-write on the new branch**: a `--change` refusal must leave an artifact that already carries a stale backlink block byte-identical (the validation sits after the parse; a reorder during review could move it after the write) — Task 1's test seeds the artifact and asserts byte-identity on every table case.

---

### Task 1: Lexical `--change` validation in `ArtifactBacklink`

**Files:**
- Modify: `internal/app/artifact_backlink.go` (new helper + one call site between the document parse and `resolveBacklinkChange`)
- Test: `internal/app/artifact_backlink_test.go` (new `TestArtifactBacklinkChangePathValidation`)

**Interfaces:**
- Consumes: existing test fixtures in `internal/app/artifact_backlink_test.go` — `docketPin(t)`, `backlinkCorpus()`, `backlinkDeps(&fakeReader{pin: pin, corpus: corpus})`, `testsupport.TempDir(t)`, and the constants `ReasonBacklinkAbsolutePath`, `ReasonBacklinkPathEscape`, `ResultInvalidInput`.
- Produces: `validateBacklinkChangePath(changePath string) (reason, message string)` — unexported, `("", "")` on a well-formed path; used only inside this file. No later task consumes it.

- [ ] **Step 1: Write the failing test**

Append to `internal/app/artifact_backlink_test.go`:

```go
// TestArtifactBacklinkChangePathValidation: --change is validated as a
// canonical repository-relative path before the corpus read — the same rule
// --artifact and the attach operations enforce. A malformed spelling is a
// typed refusal naming the flag and the expected form, never unknown-change,
// and the artifact is left byte-identical.
func TestArtifactBacklinkChangePathValidation(t *testing.T) {
	pin := docketPin(t)
	corpus := backlinkCorpus()

	cases := []struct {
		name       string
		changePath string
		reason     string
	}{
		// The 0458 shape: an absolute spelling of a path whose repo-relative
		// tail names a real record must refuse, never resolve.
		{"absolute", "/work/repo/" + backlinkChangePath, ReasonBacklinkAbsolutePath},
		{"dotdot-escape", "../" + backlinkChangePath, ReasonBacklinkPathEscape},
		{"non-canonical-dot", "./" + backlinkChangePath, ReasonBacklinkPathEscape},
		{"interior-dotdot", "docs/changes/active/../active/0315-claim.md", ReasonBacklinkPathEscape},
		{"trailing-slash", backlinkChangePath + "/", ReasonBacklinkPathEscape},
		{"empty", "", ReasonBacklinkPathEscape},
		{"whitespace-only", "  ", ReasonBacklinkPathEscape},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := testsupport.TempDir(t)
			artifact := filepath.Join(root, "plan.md")
			original := []byte("# Plan\n\nAuthored body.\n")
			if err := os.WriteFile(artifact, original, 0o644); err != nil {
				t.Fatalf("seed artifact: %v", err)
			}

			got := ArtifactBacklink(context.Background(), backlinkDeps(&fakeReader{pin: pin, corpus: corpus}), root,
				ArtifactBacklinkRequest{ArtifactPath: "plan.md", ChangePath: tc.changePath})

			if got.Result != ResultInvalidInput {
				t.Fatalf("result=%q, want %q (reason=%q message=%q)", got.Result, ResultInvalidInput, got.Reason, got.Message)
			}
			if got.Reason != tc.reason {
				t.Fatalf("reason=%q, want %q (message=%q)", got.Reason, tc.reason, got.Message)
			}
			// The message must name the flag and the expected form — the 0458
			// failure was precisely a message that named neither.
			if !strings.Contains(got.Message, "--change") {
				t.Fatalf("message does not name the --change flag: %q", got.Message)
			}
			if !strings.Contains(got.Message, "repository-relative") {
				t.Fatalf("message does not name the expected form: %q", got.Message)
			}
			// Refusal predates any write.
			out, err := os.ReadFile(artifact)
			if err != nil {
				t.Fatalf("read back: %v", err)
			}
			if string(out) != string(original) {
				t.Fatalf("file mutated on refusal:\n got %q\nwant %q", out, original)
			}
		})
	}
}
```

No new imports are needed: `context`, `os`, `filepath`, `strings`, `testing`, and `testsupport` are already imported by this file.

- [ ] **Step 2: Run the new test to verify it fails**

Run: `go test ./internal/app/ -run TestArtifactBacklinkChangePathValidation -count=1 -v`
Expected: FAIL. Every case currently falls through to `resolveBacklinkChange` and reports `reason="unknown-change"` (want `absolute-path` / `path-escape`).

Also confirm the pre-change baseline is green so Step 4's diff is attributable:
Run: `go test ./internal/app/ -run 'TestArtifactBacklink' -count=1`
Expected: FAIL only in `TestArtifactBacklinkChangePathValidation`; every other `TestArtifactBacklink*` test passes.

- [ ] **Step 3: Implement the validation**

In `internal/app/artifact_backlink.go`:

(a) Add `"path"` to the imports (alongside the existing `"path/filepath"`).

(b) Add the helper next to `containedArtifactPath` (bottom of the file):

```go
// validateBacklinkChangePath proves the --change value is a canonical
// repository-relative path — the one rule every path flag crossing the CLI
// follows (--artifact above, verifyAttachPath in change_attach.go). The check
// is purely lexical: the change record lives in the pinned git corpus, not on
// the feature worktree's filesystem, so there is no containment root to
// resolve against and no symlink to canonicalise. It mirrors verifyAttachPath
// deliberately (learning duplicated-gate-copies-the-whole-predicate: all four
// checks, not just the absolute-path threshold) without factoring it out, so
// the attach operations' observable reasons and messages stay untouched. It
// returns a stable refusal reason and a message naming the flag and the
// expected form, or ("", "") for a well-formed path.
func validateBacklinkChangePath(changePath string) (string, string) {
	const form = "pass the canonical repository-relative change path (e.g. docs/changes/active/<id>-<slug>.md)"
	if strings.TrimSpace(changePath) == "" {
		return ReasonBacklinkPathEscape,
			fmt.Sprintf("--change path is empty; %s", form)
	}
	if filepath.IsAbs(changePath) {
		return ReasonBacklinkAbsolutePath,
			fmt.Sprintf("--change path %q is absolute; %s", changePath, form)
	}
	if !filepath.IsLocal(filepath.FromSlash(changePath)) {
		return ReasonBacklinkPathEscape,
			fmt.Sprintf("--change path %q escapes the repository root; %s", changePath, form)
	}
	// Clean is a no-op for a canonical path; an input that changes under Clean
	// is non-canonical (a `./`, `//`, interior `..`, or trailing-slash
	// spelling) and is refused as an escape, matching verifyAttachPath.
	if clean := path.Clean(changePath); clean != changePath {
		return ReasonBacklinkPathEscape,
			fmt.Sprintf("--change path %q is not in canonical repository-relative form; %s", changePath, form)
	}
	return "", ""
}
```

(c) Call it in `ArtifactBacklink`, between the numbered step-3 document parse and the step-4 corpus read — i.e. immediately after the `document.Parse` refusal block and before the `resolveBacklinkChange(...)` line — keeping the spec's ordering (artifact containment and read, parse, `--change` validation, corpus read; every refusal before any write):

```go
	// 4. Validate --change lexically before the corpus read: a malformed
	//    spelling is refused by form, so unknown-change is reached only by a
	//    well-formed path that names no record.
	if reason, msg := validateBacklinkChangePath(req.ChangePath); reason != "" {
		return backlinkRefusal(ResultInvalidInput, reason, msg)
	}
```

Renumber the existing step comments 4–7 in `ArtifactBacklink` to 5–8 so the numbered narration stays consecutive.

(d) Update the two reason-constant doc comments so they cover both flags — change

```go
	// ReasonBacklinkAbsolutePath: the artifact path is absolute; paths crossing
	// the CLI are canonical repository-relative (Global Constraints).
```

to

```go
	// ReasonBacklinkAbsolutePath: the artifact or change path is absolute;
	// paths crossing the CLI are canonical repository-relative (Global
	// Constraints).
```

and

```go
	// ReasonBacklinkPathEscape: the artifact path escapes the worktree with a
	// `..` traversal.
```

to

```go
	// ReasonBacklinkPathEscape: the artifact path escapes the worktree with a
	// `..` traversal, or the change path is empty, escaping, or a
	// non-canonical spelling.
```

Also update `ReasonBacklinkUnknownChange`'s comment from "the --change path names no record in the corpus" to "the well-formed --change path names no record in the corpus" — the constant's contract narrowed.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/app/ -run 'TestArtifactBacklink' -count=1 -v`
Expected: PASS — all of `TestArtifactBacklinkChangePathValidation`, plus the pre-existing tests that pin the neighbors this change must not move:
- `TestArtifactBacklinkPathContainment/absolute` — the spec's required regression that absolute `--artifact` still refuses `absolute-path` (already exists; it must stay green).
- `TestArtifactBacklinkUnknownChange` — a well-formed absent path still refuses `unknown-change` with its existing message.
- `TestArtifactBacklinkRendersBlock` / `TestArtifactBacklinkIdempotent` — the repo-relative happy path still renders.

Then `gofmt -l internal/app/` — expected: no output.

- [ ] **Step 5: Commit**

```bash
git add internal/app/artifact_backlink.go internal/app/artifact_backlink_test.go
git commit -m "fix(app): artifact.backlink refuses a malformed --change path by form (change 0460)"
```

- [ ] **Step 6: Mutation-test the guard (after the commit, so restore is safe)**

The restore below is `git checkout -- <file>`, which restores to HEAD — that is only safe because Step 5 committed the finished implementation first (learning mutation-restore-needs-a-backup-copy).

1. In `validateBacklinkChangePath`, delete (or comment out) the entire `filepath.IsAbs` branch.
2. Run: `go test ./internal/app/ -run TestArtifactBacklinkChangePathValidation -count=1` (`-count=1` defeats the result cache — a cached PASS against the mutated tree is the known trap).
   Expected: FAIL — the `absolute` case now reports `path-escape` (the absolute path fails `filepath.IsLocal`), not `absolute-path`. If it stays green, the guard is decoration: stop and fix the test.
3. Restore: `git checkout -- internal/app/artifact_backlink.go`
4. Re-run: `go test ./internal/app/ -run TestArtifactBacklinkChangePathValidation -count=1` — expected: PASS.

---

### Task 2: State the repo-relative form in the caller prose, and regenerate the embedded assets

The absolute path in the 0458 run came from `skills/docket-implement-next/SKILL.md` leaving the path form unstated. The skill tree is byte-copied into the embedded asset bundle, and `TestEmbeddedMatchesAuthored` (`internal/assets/embedded_test.go`) is a two-directional drift guard — so the prose edit and the `go generate` regeneration must land in the same commit.

**Files:**
- Modify: `skills/docket-implement-next/SKILL.md` (two sentences: the Step 4 dispatch summary and per-checkpoint mechanics item 3)
- Regenerate: `internal/assets/embedded/tree/**` and `internal/assets/manifest.json` via `go generate ./internal/assets/` (never hand-edited)
- Test: existing `internal/assets` drift guard (no new test)

**Interfaces:**
- Consumes: nothing from Task 1 (independent; either order works, but keep plan order).
- Produces: nothing consumed later.

- [ ] **Step 1: Derive the full edit list from a whole-repo grep (never a hand-made list)**

Run from the worktree root:

```bash
hits=$(grep -rn -e "artifact.backlink" -e "artifact backlink" --include='*.md' . | grep -v -e '^\./docs/changes/archive/' -e '^\./docs/results/' -e '^\./docs/superpowers/' -e '^\./docs/adrs/' -e '^\./internal/assets/embedded/' -e '^\./internal/harness/' -e '^\./internal/render/testdata/' -e '^\./\.docket/' -e '^\./\.worktrees/')
printf '%s\n' "$hits"
```

(Capture into a variable first — never pipe the producer into an early-exiting consumer under pipefail.)

Sort the hits: point-in-time records and generated/embedded copies are excluded above by construction; what remains is maintained prose. Expected maintained hits: `skills/docket-implement-next/SKILL.md` (lines quoted in Step 2, plus flagless mentions), `skills/docket-implement-next/results-template.md` (marker-ownership note, passes no flags — leave), `skills/docket-convention/SKILL.md` (operation-roster and block-ownership mentions, pass no flags — leave), `agents/docket-plan-writer.md` (already says "repo-relative" — leave), `docs/comparison/ai-native-sdlc-playbook.md` (historical script name, passes no flags — leave). Only sites that **pass `artifact.backlink` flags with the path form unstated** are edited. If the grep surfaces a flag-passing site not listed here, fix it the same way as Step 2.

- [ ] **Step 2: Make the two edits in `skills/docket-implement-next/SKILL.md`**

Edit 1 — per-checkpoint mechanics item 3 (the sentence the spec names; currently line 123). Replace:

```
3. Write the update, then stamp and validate the back-link home with the `artifact.backlink` operation (`--artifact <results path> --change <change path>`).
```

with:

```
3. Write the update, then stamp and validate the back-link home with the `artifact.backlink` operation (`--artifact <results repo-relative path> --change <change repo-relative path>`).
```

Edit 2 — the Step 4 dispatch summary (currently line 84) has the same unstated form. In the sentence beginning `The child invokes the resolved plan skill`, replace the fragment:

```
stamps the backlink with the `artifact.backlink` operation (`--artifact <plan path> --change <change path>`)
```

with:

```
stamps the backlink with the `artifact.backlink` operation (`--artifact <plan repo-relative path> --change <change repo-relative path>`)
```

This matches the wording already used in `agents/docket-plan-writer.md` ("`--artifact <plan repo-relative path> --change <change repo-relative path>`").

- [ ] **Step 3: Regenerate the embedded asset bundle**

Run: `go generate ./internal/assets/`
Expected: `internal/assets/embedded/tree/skills/docket-implement-next/SKILL.md` and `internal/assets/manifest.json` now reflect the edit (`git status` shows them modified; nothing else).

- [ ] **Step 4: Run the drift guard and prose-adjacent suites**

Run: `go test ./internal/assets/ -count=1`
Expected: PASS (`TestEmbeddedMatchesAuthored` green in both directions).

Run: `go test ./internal/harness/... -count=1`
Expected: PASS — the harness golden wrappers are thin (they do not embed the skill body), so nothing reddens; this run proves that assumption.

- [ ] **Step 5: Commit**

```bash
git add skills/docket-implement-next/SKILL.md internal/assets/embedded/tree internal/assets/manifest.json
git commit -m "docs(skills): name the repo-relative form for artifact.backlink flags (change 0460)"
```

---

### Task 3: Whole-package verification

**Files:** none (verification only).

**Interfaces:**
- Consumes: Tasks 1–2 committed.
- Produces: a clean tree for the build gate.

- [ ] **Step 1: Run the touched packages together, cache-defeated**

Run: `go test ./internal/app/ ./internal/assets/ ./internal/harness/... -count=1`
Expected: PASS.

- [ ] **Step 2: Confirm the tree is clean**

Run: `git status --porcelain`
Expected: empty. (The full repository suite is the build gate's job — the gate runs whatever `build.test_command` resolves to, from source; do not substitute a partial package run for it.)

---

## Self-review notes

- Spec coverage: §1 validation table → Task 1 (all five rows: empty/whitespace, absolute, `..` escape, non-canonical, well-formed-absent unchanged); §1 message rule → Task 1 Step 1 message asserts + Step 3 wording; §1 ordering rule → Task 1 Step 3(c); §1 helper choice (inline copy, attach ops untouched) → Task 1 Step 3(b); §2 prose fix + grep-derived caller list → Task 2; Testing section rows map to Task 1's table, the existing `TestArtifactBacklinkUnknownChange` / happy-path / `--artifact`-absolute tests (pinned green in Task 1 Step 4), and the mutation check → Task 1 Step 6.
- The `--artifact` absolute regression the spec asks for already exists (`TestArtifactBacklinkPathContainment/absolute`); Task 1 Step 4 names it as a must-stay-green rather than duplicating it.
- Learnings consulted: duplicated-gate-copies-the-whole-predicate (copy all four checks, cited in the helper comment), fix-reintroduces-its-own-defect-class (the new helper is itself a path validator — its own table is the audit), mutation-restore-needs-a-backup-copy (Task 1 Step 6 commits first), cached-runner-serves-a-mutated-tree (`-count=1` throughout), consolidation-flattens-caller-variance (Task 2 edits name each site's own artifact kind rather than templating one sentence).
