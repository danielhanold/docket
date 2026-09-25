<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0454 — Whole-repository status must not fail on an unrelated change's invalid branch name](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-25-0454-whole-repository-status-must-not-fail-on-an-unrelated-change.md)**
<!-- docket:backlink:end -->
# Whole-Repository Status Survives an Unrelated Invalid Branch Name — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the three whole-repository reads that share one live branch probe (`docket status`, `maintenance.preflight`, automatic `context.implementation`) survive a change whose recorded `branch:` is not a valid git ref name, and report it as a per-change `branch-malformed` error finding instead of failing the whole read.

**Architecture:** Complete gitcli's local ref-name grammar to git's own `check-ref-format` rules and export one predicate (`gitcli.ValidBranchName`). The whole-corpus probe collector `stackBranches` skips names that fail it (such a name cannot exist on the remote, so skipping states absence accurately); the existing `BaseBranchAbsent`/`stack-base-unresolved` domain path then handles selection with no new logic. Status gains a per-displayed-change `branch-malformed` finding whose remedy is branched on whether the record carries a parseable `pr:`. `recordedBranch` delegates its shape check to the same predicate so the printed PR-case remedy (`change repair-identity --adopt-pr-head`) actually applies to every name status flags.

**Tech Stack:** Go; packages `internal/gitcli`, `internal/app`, `internal/domain` (read-only). Test style: stdlib `testing`, table tests, the existing `fakeReader` / `poisonBranchReader` scriptable StatusReader fakes in `internal/app`.

**Spec:** `docs/superpowers/specs/2026-09-24-whole-repository-status-must-not-fail-on-an-unrelated-change-design.md` (synchronized metadata tree; the change record is `docs/changes/active/0454-whole-repository-status-must-not-fail-on-an-unrelated-change.md`).

## Global Constraints

- `stackBranchesFor` (named operations) is **unchanged** — 0449's bounding stays exactly as is.
- `gitStatusReader.BranchFacts` (`internal/app/status_git.go`) is **unchanged**: a well-formed name whose probe fails for a real reason (network, auth) must keep failing the whole read as `external-failed`. An observation failure is never reported as proven absence (learning: probe-error-is-not-clean-absence).
- The new finding is **not** a snapshot-validation finding — nothing is added to `repository.BuildSnapshot`'s report.
- Do **not** merge `domain.malformedBranchRef` or `transaction.validRefShape` into the gitcli predicate. Only `recordedBranch` delegates.
- Every finding-code string literal in `internal/app` must be minted in `internal/app/finding_codes.go` (`TestNoInlineFindingCodeLiterals` enforces this by AST shape) and registered in the hand-sorted `AllFindingCodes` (`TestFindingCodeRegistryIntegrity` asserts ordering/dedup).
- No new ADR; no new readiness token, status, or manifest field.
- Every remedy string must be valid in the exact repo state that produced it (learning: printed-remedy-state-validity); status stays offline and fills in only values it already holds.
- The build gate runs the whole suite via the repo's configured `build.test_command` (read from config; the Go runner `internal/suiterunner` is the sole channel).

## Review Focus

Spec-implied inputs no single task's happy path covers; each line's pinning test is added to the owning task:

1. A recorded branch that is well-formed but unprobeable (network failure) must still fail the whole read `external-failed` — the filter must not widen into swallowing probe errors. → Task 2, Step 6.
2. A malformed branch on a change with `pr:` present but unparseable (e.g. `pr: 'broken'`) must get the hand-edit remedy, never a `repair-identity` command with a fabricated number. → Task 3 remedy tests (row 3).
3. A name only the *completed* grammar rejects (`feat/a:b` — the old grammar passed it to git) must be filtered from the probe and flagged, not just the historically-rejected `feat/a..parent` shape. → Tasks 1–4 all carry a `feat/a:b` row.
4. `recordedBranch` delegation must stay fail-closed, not loosened: `refs/`-prefixed and leading-`-` values must still be refused even though the prefixed-ref predicate alone would pass them. → Task 4, Step 1 rows.
5. An *archived* or filtered-out change with a malformed branch must not produce a finding (the finding covers **displayed active** changes only), while a malformed stack *ancestor* must still be skipped by the probe even when the ancestor itself is filtered out of display. → Task 3, Step 4.

---

### Task 1: Complete gitcli's ref-name grammar and export the branch-name predicate

**Files:**
- Modify: `internal/gitcli/types.go` (function `validateRefName`, and a new exported `ValidBranchName`)
- Test: `internal/gitcli/types_test.go` (extend `TestValidateRefName`; add `TestValidBranchName`)

**Interfaces:**
- Produces: `func ValidBranchName(short string) bool` in package `gitcli` — reports whether `refs/heads/<short>` passes `validateRefName`, i.e. exactly the question the live probe (`FetchBranch`, which validates `refs/heads/`+name) will ask. Tasks 2–4 consume it.
- `validateRefName` keeps its existing rules and error style (`errors.New("gitcli: …")`); the only behavioural change for gitcli operations is that a name git would reject anyway now fails early as `KindInvalidRequest` instead of reaching git and failing as `KindCommandFailed`.

- [ ] **Step 1: Write the failing tests**

In `internal/gitcli/types_test.go`, extend `TestValidateRefName`'s `bad` slice with one row per **added** rule (each row fails exactly one new rule, so deleting any one rule reddens its row — the mutation the spec demands), keeping the existing rows as controls:

```go
	bad := []RefName{"main", "heads/main", "refs/", "refs/heads/", "-refs/heads/x",
		"refs/heads/a b", "refs/heads/a..b", "refs/heads/a.lock", "refs/heads/a@{1}",
		"refs/heads/*", "refs/heads/a\\z", "refs/heads/.hidden", "refs/heads/a\x00b",
		// check-ref-format completion (change 0454): control chars, ~ ^ : ? [, trailing dot.
		"refs/heads/a\x01b", "refs/heads/a\x1fb", "refs/heads/a\x7fb",
		"refs/heads/a~b", "refs/heads/a^b", "refs/heads/a:b",
		"refs/heads/a?b", "refs/heads/a[b", "refs/heads/a."}
```

Add below it:

```go
func TestValidBranchName(t *testing.T) {
	good := []string{"main", "feat/x", "fix/whole-repository-status", "a.b/c-d"}
	for _, b := range good {
		if !ValidBranchName(b) {
			t.Errorf("ValidBranchName(%q) = false, want true", b)
		}
	}
	// Includes the two fixture names every later task reuses: feat/a..parent
	// (old grammar already rejected) and feat/a:b (only the completed grammar
	// rejects it locally; git itself always did).
	bad := []string{"", "feat/a..parent", "feat/a:b", "a b", "a~b", "a^b", "a?b",
		"a[b", "a.", "a\x01b", "@{x", "a\\b", "a*", ".hidden", "a.lock", "a/", "a//b"}
	for _, b := range bad {
		if ValidBranchName(b) {
			t.Errorf("ValidBranchName(%q) = true, want false", b)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/gitcli/ -run 'TestValidateRefName|TestValidBranchName' -v`
Expected: FAIL — `ValidBranchName` undefined (compile error). Fix compile by adding the `ValidBranchName` stub returning `validateRefName(...) == nil` first if you want to see the grammar rows fail individually: `refs/heads/a~b`, `refs/heads/a:b`, `refs/heads/a.` etc. are accepted by the current grammar.

- [ ] **Step 3: Implement**

In `internal/gitcli/types.go`, inside `validateRefName`, after the existing whitespace check, add:

```go
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] == 0x7F {
			return errors.New("gitcli: ref name contains control character")
		}
	}
	if strings.ContainsAny(s, "~^:?[") {
		return errors.New("gitcli: ref name contains ~, ^, :, ? or [")
	}
	if strings.HasSuffix(s, ".") {
		return errors.New("gitcli: ref name ends in dot")
	}
```

Update the function's doc comment to say it implements git's `check-ref-format` rules (list the additions). Then add, near `validateRefName`:

```go
// ValidBranchName reports whether short is a name git's check-ref-format
// accepts as refs/heads/<short> — exactly the question the live branch probe
// asks before fetching (FetchBranch validates the refs/heads/-prefixed name).
// It is the app layer's one sanctioned predicate for deciding that a recorded
// branch: value cannot exist as a ref (change 0454). It deliberately shares
// validateRefName so the caller and the probe can never diverge.
func ValidBranchName(short string) bool {
	return validateRefName(RefName("refs/heads/"+short)) == nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/gitcli/ -v`
Expected: PASS (whole package — the completed grammar must not break other gitcli tests; if an existing test used a now-rejected name, treat that as a real finding to reconcile against the spec, not a test to weaken).

- [ ] **Step 5: Commit**

```bash
git add internal/gitcli/types.go internal/gitcli/types_test.go
git commit -m "feat(gitcli): complete ref-name grammar to check-ref-format and export ValidBranchName (change 0454)"
```

---

### Task 2: The whole-corpus probe skips names that cannot exist

**Files:**
- Modify: `internal/app/status.go` (function `stackBranches`; add `gitcli` import)
- Test: `internal/app/status_branch_malformed_test.go` (new file; Tasks 3 and 5 extend it)

**Interfaces:**
- Consumes: `gitcli.ValidBranchName(string) bool` (Task 1).
- Produces: `stackBranches(snap domain.Snapshot) []string` now excludes any recorded ancestor branch failing the predicate. Signature unchanged; both whole-corpus callers (`Status` step 4 and `implementation_context.go`'s `req.ID <= 0` branch) get the fix from this one edit. `stackBranchesFor` untouched.
- Produces (test helpers this file's later tasks reuse): `malformedStackCorpus(t)` returning a `[]StatusBlob` corpus built with the existing `changeBlob` helper (`status_test.go`): change 0001 `parent-dots` with `branch: 'feat/a..parent'` + `status: in-progress`; 0002 `parent-colon` with `branch: 'feat/a:b'` + `status: in-progress`; 0003 `child-a` with `stacked_on: 1`; 0004 `child-b` with `stacked_on: 2`; 0005 `parent-ok` with `branch: 'feat/ok'` + `status: in-progress`; 0006 `child-ok` with `stacked_on: 5`; 0007 `plain` (no branch, no stack — an ordinary build-ready change). Extra frontmatter goes through `changeBlob`'s `extra` parameter, e.g. `"branch: 'feat/a..parent'\nstatus: in-progress\n"` — note `changeBlob` already writes `status: proposed`, so instead pass status via the extra only if `changeBlob` supports overriding; otherwise write the raw record bytes inline the way `changeBlob` composes them, with the needed `status:`/`branch:`/`stacked_on:` lines. Follow the file's local convention after reading `changeBlob`.

- [ ] **Step 1: Write the failing unit test**

Create `internal/app/status_branch_malformed_test.go`. Build the corpus, parse and snapshot it exactly as `Status` does (`parseCorpus`, then `repository.BuildSnapshot` with `Config: testConfig(t)` effective config — copy the two-call sequence from `Status` steps 2–3), then:

```go
func TestStackBranchesSkipsMalformedNames(t *testing.T) {
	snap := buildSnapshot(t, malformedStackCorpus(t)) // local helper wrapping parseCorpus + repository.BuildSnapshot
	got := stackBranches(snap)
	for _, b := range got {
		if b == "feat/a..parent" || b == "feat/a:b" {
			t.Errorf("stackBranches includes malformed name %q", b)
		}
	}
	want := []string{"feat/ok"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("stackBranches = %v, want %v", got, want)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/app/ -run TestStackBranchesSkipsMalformedNames -v`
Expected: FAIL — got includes `feat/a..parent` and `feat/a:b`.

- [ ] **Step 3: Implement the filter**

In `internal/app/status.go`, `stackBranches`, change the collection condition:

```go
			if b := ancestor.Branch(); b.State == domain.FieldPresent && b.Value != "" && gitcli.ValidBranchName(b.Value) {
				seen[b.Value] = true
			}
```

Add the `gitcli` import (`github.com/danielhanold/docket/internal/gitcli`). Extend `stackBranches`' doc comment: a name failing `gitcli.ValidBranchName` cannot exist on the remote, so leaving it out of the probe is an accurate statement of absence, not a guess — `BranchFacts.HasBranch` returns false for it and `ResolveEffectiveBase` reports `BaseBranchAbsent` (change 0454); `stackBranchesFor` deliberately does **not** filter (named operations keep failing closed, change 0449).

- [ ] **Step 4: Write the failing Status-level test (applied, not external-failed; probe never asked)**

Same file. Use `fakeReader` and the `poisoned` / `poisonBranchReader` helpers (`internal/app/named_branch_facts_test.go`) — poisoning proves the malformed names are never requested (mutation: reverting Step 3 makes this red via the poison error):

```go
func TestStatusSurvivesMalformedStackBranch(t *testing.T) {
	pin := docketPin(t)
	inner := &fakeReader{pin: pin, corpus: malformedStackCorpus(t), facts: domain.NewBranchFacts(map[string]bool{"feat/ok": true})}
	reader := poisoned(inner, "feat/a..parent", "feat/a:b")
	res := Status(context.Background(), reader, StatusOptions{RepoDir: "."})
	if res.Result != ResultApplied {
		t.Fatalf("Status over a malformed stack branch = %s (%s: %s), want applied", res.Result, res.Reason, res.Message)
	}
	rows := map[int]StatusChange{}
	for _, c := range res.Changes {
		rows[c.ID] = c
	}
	for _, id := range []int{3, 4} { // children of the malformed parents
		if rows[id].Readiness != string(domain.ReadyStackBaseUnresolved) {
			t.Errorf("change %04d readiness = %q, want %q", id, rows[id].Readiness, domain.ReadyStackBaseUnresolved)
		}
		for _, r := range res.Ready {
			if r == id {
				t.Errorf("change %04d is in ready, want excluded", id)
			}
		}
	}
	if rows[7].ID != 7 {
		t.Errorf("unrelated change 0007 missing from the rendered backlog")
	}
}
```

Adjust `domain.NewBranchFacts`'s argument to its real signature (see its uses in `status_test.go` — `domain.NewBranchFacts(nil)`); the intent is: `feat/ok` present, nothing else asked.

- [ ] **Step 5: Run it, verify it passes; verify Step 4's mutation**

Run: `go test ./internal/app/ -run TestStatusSurvivesMalformedStackBranch -v`
Expected: PASS. Then temporarily revert the Step 3 filter (`git stash -- internal/app/status.go` or comment the predicate out), rerun, and confirm the test fails with the poison error; restore.

- [ ] **Step 6: Add the regression test — a real probe failure on a well-formed name still fails the read**

```go
func TestStatusStillFailsOnUnprobeableWellFormedBranch(t *testing.T) {
	pin := docketPin(t)
	inner := &fakeReader{pin: pin, corpus: malformedStackCorpus(t), facts: domain.NewBranchFacts(nil)}
	reader := poisoned(inner, "feat/ok") // well-formed, but the probe errors
	res := Status(context.Background(), reader, StatusOptions{RepoDir: "."})
	if res.Result != ResultExternalFailed {
		t.Fatalf("Status with an unprobeable well-formed branch = %s, want external-failed", res.Result)
	}
}
```

Use the exact `Result*` constant `classifyStatusError` maps `ErrStatusExternal` to (see existing external-failure tests in `status_test.go`).

Run: `go test ./internal/app/ -run TestStatusStillFailsOnUnprobeableWellFormedBranch -v` — PASS (this pins existing behaviour; if it fails, the filter widened too far — fix the filter, not the test).

- [ ] **Step 7: Commit**

```bash
git add internal/app/status.go internal/app/status_branch_malformed_test.go
git commit -m "fix(app): whole-corpus branch probe skips names that cannot exist (change 0454)"
```

---

### Task 3: Status reports a per-change `branch-malformed` finding with a state-valid remedy

**Files:**
- Modify: `internal/app/finding_codes.go` (new `FCBranchMalformed` constant + `AllFindingCodes` entry)
- Modify: `internal/app/status.go` (new `branchMalformedCheck`; wire into `Status` step 6 loop)
- Test: `internal/app/status_branch_malformed_test.go` (extend)

**Interfaces:**
- Consumes: `gitcli.ValidBranchName` (Task 1), `parsePRRef(string) (int, bool)` (`internal/app/finalize_context.go`), `changeIdentity(domain.ChangeID) string`, `blobByPath map[string]StatusBlob` (already built in `Status` step 2).
- Produces: `FCBranchMalformed FindingCode = "branch-malformed"` (reuses the token `errBranchMalformed` / `domain.skipBranchMalformed` already spell); `branchMalformedCheck(c domain.Change, blobByPath map[string]StatusBlob) []StatusFinding`, appended to `artifactFindings` so `assembleFindings` sorts it with the artifact group (identity, then field).

- [ ] **Step 1: Write the failing finding tests**

Extend `TestStatusSurvivesMalformedStackBranch` (Task 2) and add remedy-variant coverage. For the corpus, give 0001 a parseable PR (`pr: 'https://github.com/danielhanold/docket/pull/77'`) and 0002 no `pr:`; add 0008 `parent-badpr` with `branch: 'feat/a:c'`, `status: in-progress`, and `pr: 'broken'` (present but unparseable — Review Focus 2):

```go
func TestStatusBranchMalformedFindings(t *testing.T) {
	pin := docketPin(t)
	inner := &fakeReader{pin: pin, corpus: malformedStackCorpus(t), facts: domain.NewBranchFacts(nil)}
	reader := poisoned(inner, "feat/a..parent", "feat/a:b", "feat/a:c")
	res := Status(context.Background(), reader, StatusOptions{RepoDir: "."})
	if res.Result != ResultApplied {
		t.Fatalf("Status = %s, want applied", res.Result)
	}
	byIdentity := map[string][]StatusFinding{}
	for _, f := range res.Findings {
		if f.Code == string(FCBranchMalformed) {
			byIdentity[f.Identity] = append(byIdentity[f.Identity], f)
		}
	}
	// Exactly one finding per malformed displayed change; none for well-formed ones.
	for _, id := range []string{"0001", "0002", "0008"} {
		fs := byIdentity[id]
		if len(fs) != 1 {
			t.Fatalf("change %s: %d branch-malformed findings, want 1", id, len(fs))
		}
		f := fs[0]
		if f.Severity != "error" || f.Entity != string(domain.EntityChange) || f.Field != "branch" {
			t.Errorf("change %s finding shape = %+v", id, f)
		}
		if !strings.Contains(f.Message, "not a valid git branch name") {
			t.Errorf("change %s message = %q", id, f.Message)
		}
	}
	if len(byIdentity) != 3 {
		t.Errorf("branch-malformed identities = %v, want exactly 0001 0002 0008", byIdentity)
	}
	// Remedy variants (printed-remedy-state-validity): parseable pr: names the
	// typed repair with id, version, and PR number filled in; otherwise the
	// hand-edit + repository migrate remedy — including pr: present but unparseable.
	prRemedy := byIdentity["0001"][0].Remedy
	for _, want := range []string{"change repair-identity", "--id 1", "--expect-version blobchange0001", "--adopt-pr-head", "--expect-pr 77"} {
		if !strings.Contains(prRemedy, want) {
			t.Errorf("PR-case remedy %q lacks %q", prRemedy, want)
		}
	}
	for _, id := range []string{"0002", "0008"} {
		r := byIdentity[id][0].Remedy
		if strings.Contains(r, "repair-identity") || !strings.Contains(r, "repository migrate") {
			t.Errorf("change %s remedy = %q, want the hand-edit + repository migrate remedy", id, r)
		}
	}
}
```

(`blobchange0001` is the `Version` the `changeBlob` fixture helper stamps; if the corpus helper writes raw blobs, assert against the version string it used.) Mutation for the spec's remedy test: swapping the `pr:`-parses condition makes the 0001 vs 0002 expectations cross — this test is that mutation's tripwire.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/app/ -run TestStatusBranchMalformedFindings -v`
Expected: FAIL — `FCBranchMalformed` undefined.

- [ ] **Step 3: Implement**

In `internal/app/finding_codes.go`, in the census const block:

```go
	// Whole-repository status branch check (change 0454): a displayed active
	// change records a branch: value that is not a valid git branch name. The
	// token deliberately reuses the spelling finalize's skip reason and
	// recordedBranch's errBranchMalformed already emit.
	FCBranchMalformed FindingCode = "branch-malformed"
```

In `AllFindingCodes`, insert `FCBranchMalformed,` in sorted position — immediately before `FindingCode("branch-still-exists"),`.

In `internal/app/status.go`, next to `artifactChecks`:

```go
// branchMalformedCheck reports one error finding when a displayed active
// change's recorded branch: cannot be a git branch name (gitcli.ValidBranchName,
// the same predicate the whole-corpus probe filters on — change 0454). The
// remedy is branched on the same condition that decides which repair can work
// in this exact state: a parseable pr: names the typed repair-identity
// adopt-pr-head command with the id, record version, and PR number filled in
// (the head branch must be read from the PR itself — status stays offline);
// otherwise no typed operation edits branch:, so the remedy is the hand edit
// plus repository migrate to re-render the board. An absent or empty branch:
// is a distinct, benign state here and produces no finding.
func branchMalformedCheck(c domain.Change, blobByPath map[string]StatusBlob) []StatusFinding {
	b := c.Branch()
	if b.State != domain.FieldPresent || b.Value == "" || gitcli.ValidBranchName(b.Value) {
		return nil
	}
	remedy := "correct branch: on the change record on the docket branch (the real feature branch, or clear it if no branch was ever created), then run: docket repository migrate to re-render the board"
	if pr := c.PR(); pr.State == domain.FieldPresent {
		if n, ok := parsePRRef(pr.Value); ok {
			remedy = fmt.Sprintf("run: docket change repair-identity --id %d --expect-version %s --adopt-pr-head --expect-pr %d --expect-head <the head branch shown on PR #%d>",
				int(c.ID()), blobByPath[c.Path()].Version, n, n)
		}
	}
	return []StatusFinding{{
		Code:     string(FCBranchMalformed),
		Severity: string(domain.SeverityError),
		Entity:   string(domain.EntityChange),
		Identity: changeIdentity(c.ID()),
		Field:    "branch",
		Message: fmt.Sprintf("change %s records branch: %q, which is not a valid git branch name",
			changeIdentity(c.ID()), b.Value),
		Remedy: remedy,
	}}
}
```

Wire it into `Status` step 6, inside the existing `for _, c := range displayed` loop, after the `artifactChecks` append:

```go
		artifactFindings = append(artifactFindings, branchMalformedCheck(c, blobByPath)...)
```

Update the step-6 comment to name both checks. Human output (`status_human.go`) and the preflight envelope render findings generically — verify no format-specific change is needed by reading how `status_human.go` prints `Findings` (it must print Code/Message/Remedy for this finding as for any other; do not add a bespoke branch).

- [ ] **Step 4: Add the display-scope test (Review Focus 5)**

Same file: run `Status` with `StatusOptions{Types: []string{"feature"}}` (or whatever projection excludes the fixture's `fix`-typed malformed parents — pick a filter that keeps 0007 and drops 0001/0002/0008, adjusting fixture types as needed) and assert: result applied, zero `branch-malformed` findings (parents not displayed), while the probe still never asks for the malformed names (poisoned reader stays quiet — the skip is corpus-wide even when the record is filtered from display).

Run: `go test ./internal/app/ -run 'TestStatusBranchMalformed' -v` — Expected: FAIL until implemented, then PASS.

- [ ] **Step 5: Run the package's guard tests**

Run: `go test ./internal/app/ -run 'TestNoInlineFindingCodeLiterals|TestFindingCodeRegistryIntegrity|TestStatus' -v`
Expected: PASS — the constant is registered, sorted, and minted only in `finding_codes.go`.

- [ ] **Step 6: Commit**

```bash
git add internal/app/finding_codes.go internal/app/status.go internal/app/status_branch_malformed_test.go
git commit -m "feat(app): status reports branch-malformed per displayed change with a state-valid remedy (change 0454)"
```

---

### Task 4: `recordedBranch` delegates its shape check, so the PR-case remedy works for every flagged name

**Files:**
- Modify: `internal/app/branch_identity.go` (function `recordedBranch`)
- Test: `internal/app/change_repair_test.go` (extend)

**Interfaces:**
- Consumes: `gitcli.ValidBranchName` (Task 1).
- Produces: `recordedBranch(c domain.Change) (string, error)` — signature and error values (`errBranchMissing`, `errBranchMalformed`) unchanged; the malformed set **widens** to everything the gitcli predicate rejects while keeping its own stricter `refs/` and leading-`-` refusals (fail-closed both ways; Review Focus 4). Effect: `repairProveWorkspaceClear` skips the workspace inspection for exactly the names status flags, so `change repair-identity --adopt-pr-head` applies on a `feat/a:b` record instead of failing inside git.

- [ ] **Step 1: Write the failing test**

`recordedBranch` is package-private with existing direct coverage? Check for a `branch_identity_test.go`; if none, add the table into `change_repair_test.go` (it lives in package `app`):

```go
func TestRecordedBranchDelegatesToGitcliPredicate(t *testing.T) {
	cases := []struct {
		branch string
		wantErr error
	}{
		{"feat/x", nil},
		{"", errBranchMissing},
		{"refs/heads/x", errBranchMalformed}, // kept: stricter than the prefixed-ref predicate alone
		{"-x", errBranchMalformed},           // kept: option smuggling
		{"feat/a..parent", errBranchMalformed},
		{"feat/a:b", errBranchMalformed}, // NEW: only the delegated predicate rejects this
		{"a~b", errBranchMalformed},
		{"a b", errBranchMalformed},
		{"a.", errBranchMalformed},
	}
	for _, c := range cases {
		got, err := recordedBranch(changeWithBranch(t, c.branch)) // build via the file's existing change-fixture helper
		if !errors.Is(err, c.wantErr) {
			t.Errorf("recordedBranch(branch=%q) err = %v, want %v", c.branch, err, c.wantErr)
		}
		if c.wantErr == nil && got != c.branch {
			t.Errorf("recordedBranch(branch=%q) = %q", c.branch, got)
		}
	}
}
```

Build the `domain.Change` fixture the way this test file already builds changes for repair tests (read the file first and reuse its helper; if it only builds via corpus parsing, parse a one-record corpus per row).

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/app/ -run TestRecordedBranchDelegatesToGitcliPredicate -v`
Expected: FAIL on the `feat/a:b` row (currently returns nil error).

- [ ] **Step 3: Implement**

In `internal/app/branch_identity.go`:

```go
	if strings.HasPrefix(b.Value, "refs/") || strings.HasPrefix(b.Value, "-") ||
		!gitcli.ValidBranchName(b.Value) {
		return "", errBranchMalformed
	}
```

Import `gitcli`; drop the now-subsumed hand-listed checks (whitespace, `@{`, `..`, NUL — all rejected by the predicate); keep and comment the two survivors: `refs/` (a short name must not smuggle a full ref — prefixing would still make it a *valid* ref, so the predicate alone cannot catch it) and leading `-` (option smuggling; also invisible to the prefixed-ref question). Update the doc comment: the shape check delegates to `gitcli.ValidBranchName` so it agrees with the probe and with status's `branch-malformed` finding (change 0454) — fail-closed: a name git would reject is refused as `branch-malformed` here rather than failing inside git (duplicated-gate-copies-the-whole-predicate: delegation, not a second enumeration).

- [ ] **Step 4: Run it to verify it passes**

Run: `go test ./internal/app/ -run 'TestRecordedBranch|TestRepair' -v`
Expected: PASS, including every existing repair test (the widened malformed set must not break the adopt-pr-head apply path's fixtures — they use well-formed branches).

- [ ] **Step 5: Write the failing end-to-end remedy test (spec Testing 8)**

In `internal/app/change_repair_test.go`, find the existing test that drives `repair-identity --adopt-pr-head` to `RepairApplied` (the apply-path test around `TestRepairWritesOnlyTheApprovedField` — read it and reuse its fake GitHub + engine harness verbatim). Add a two-row variant where the record's `branch:` is malformed and the adopted head is present on the remote:

```go
func TestRepairAdoptPRHeadAppliesOnMalformedRecordedBranch(t *testing.T) {
	for _, recorded := range []string{"feat/a..parent", "feat/a:b"} {
		t.Run(recorded, func(t *testing.T) {
			// Same harness as the existing adopt-pr-head apply test, with the
			// change record's branch: set to `recorded`, the fake PR reporting
			// head "feat/real", and "feat/real" present on the fake remote.
			// Assert the operation result is the applied disposition and the
			// record's branch: becomes "feat/real".
		})
	}
}
```

Fill the body from the neighbouring apply test's real harness (fakes, request construction with `AdoptPRHead: true`, `ExpectPRNumber`, `ExpectHead: "feat/real"`, version threading) — the two rows and the two assertions above are the contract; the plumbing is whatever that file already does. The `feat/a:b` row is the discriminating one: without Step 3 it reaches `repairProveWorkspaceClear`'s workspace inspection path (recordedBranch called it well-formed) and diverges — run the test before Step 3's commit if you want to watch it, or temporarily revert `branch_identity.go` to confirm the row reddens.

- [ ] **Step 6: Run and verify; confirm the mutation**

Run: `go test ./internal/app/ -run TestRepairAdoptPRHeadAppliesOnMalformedRecordedBranch -v`
Expected: PASS both rows. Temporarily revert the Step 3 delegation and confirm the `feat/a:b` row fails; restore.

- [ ] **Step 7: Commit**

```bash
git add internal/app/branch_identity.go internal/app/change_repair_test.go
git commit -m "fix(app): recordedBranch delegates branch shape to the gitcli predicate (change 0454)"
```

---

### Task 5: Automatic selection and preflight survive the malformed branch (tests only)

**Files:**
- Test: `internal/app/implementation_context_test.go` (extend)
- Test: `internal/app/maintenance_preflight_test.go` (extend)

**Interfaces:**
- Consumes: Task 2's `stackBranches` filter (both behaviours come free from it — these tests pin the two other whole-repository entry points the spec names), `malformedStackCorpus` / fixture idiom from Task 2 (the corpus helper lives in `status_branch_malformed_test.go`, same package, so it is directly callable), `poisoned(...)`.

- [ ] **Step 1: Write the automatic-selection test (spec Testing 4)**

In `internal/app/implementation_context_test.go`, following the file's existing harness for `ImplementationContext` (read how it builds `deps`, its reader, and a no-id request):

```go
func TestImplementationContextAutoSelectionSurvivesMalformedStackBranch(t *testing.T) {
	// Corpus: malformedStackCorpus(t) with 0007 build-ready (proposed, no deps).
	// Reader: poisoned(inner, "feat/a..parent", "feat/a:b") over a fakeReader
	// whose facts carry feat/ok. Request: no explicit id (req.ID == 0).
	// Assert: the result is the applied/context disposition, the selected
	// change is 0007 (top healthy build-ready), and the result is NOT the
	// external-failed classification.
}
```

Fill the body from the file's existing automatic-selection test (same deps construction, same result-field assertions); the three assertions above are the contract. Mutation: reverting Task 2's filter makes this test fail with the poison error.

- [ ] **Step 2: Write the preflight test (spec Testing 5)**

In `internal/app/maintenance_preflight_test.go`, following its existing harness (preflight's post-sweep read is the `Status(ctx, reader, StatusOptions{…})` call inside `maintenance_preflight.go`):

```go
func TestPreflightForwardsBranchMalformedFinding(t *testing.T) {
	// Same corpus + poisoned reader. Run the preflight operation the way the
	// file's other tests do. Assert: the preflight returns its normal verdict
	// (not an external failure), and its forwarded status findings include
	// exactly one with Code == string(FCBranchMalformed) and Identity "0001".
}
```

Fill the body from the neighbouring preflight tests' real harness.

- [ ] **Step 3: Run both to verify they pass**

Run: `go test ./internal/app/ -run 'TestImplementationContextAutoSelectionSurvivesMalformedStackBranch|TestPreflightForwardsBranchMalformedFinding' -v`
Expected: PASS (the production code shipped in Tasks 2–3; these are pinning tests — if either fails, the wiring assumption is wrong: debug the production path, never weaken the test).

- [ ] **Step 4: Commit**

```bash
git add internal/app/implementation_context_test.go internal/app/maintenance_preflight_test.go
git commit -m "test(app): automatic selection and preflight survive a malformed stack branch (change 0454)"
```

---

### Task 6: 0449's integration test goes through the real `Status`

**Files:**
- Modify: `internal/app/named_isolation_integration_test.go` (function `assertUnrelatedBytesIntact` and its doc comment)

**Interfaces:**
- Consumes: the file's existing integration fixtures (`unrelatedBrokenPath`, `unrelatedBrokenBytes`, `originFile`, the git-backed repo harness) and the real `Status` entry point with the production reader the file's flow already constructs (find how the test invokes other real operations against `repo` and use the same construction for `Status` — if the flow drives operations through a CLI/app facade, call `Status` the same way).

- [ ] **Step 1: Rework the assertion**

Replace `assertUnrelatedBytesIntact`'s current in-memory `parseCorpus` re-parse (the spec calls it "routing around" the read) with the real read: after asserting the origin bytes are intact (keep that part verbatim), run the real `Status` over the repo and assert (a) the result is applied — the whole-repository read survives the broken record — and (b) the findings include an error-severity finding with `Path == unrelatedBrokenPath` (the parse finding for the broken record). Update the doc comment to say the proof now goes through the production `Status` read (change 0454) instead of a detached `parseCorpus`.

Note: the 0449 fixture's broken record is a parse-level defect; if the fixture repo also (or instead) needs the `feat/a..parent` branch-name shape the spec names, check what `unrelatedBrokenBytes` seeds — the spec's requirement is "go through the real `Status` over the `feat/a..parent` fixture and assert the finding". If the seeded record parses but records `branch: feat/a..parent`, assert the `FCBranchMalformed` finding for it instead of the parse finding. Read the fixture first; assert the finding class that fixture actually produces, and if it produces neither, extend the fixture with a second unrelated record carrying `branch: 'feat/a..parent'` and assert both: `Status` applied + its `branch-malformed` finding present.

- [ ] **Step 2: Run the integration test**

Run: `go test ./internal/app/ -run TestIntegrationNamedImplementationFlowIsolation -v` (build tags: check the file's header for a required tag and add `-tags` accordingly; `tests/README.md` documents the suite's tag partitions.)
Expected: PASS. Before Task 2's filter this exact call would have failed `external-failed` — that ordering is the point of the rework; you can confirm by stashing the Task 2 edit once, rerunning, and restoring.

- [ ] **Step 3: Commit**

```bash
git add internal/app/named_isolation_integration_test.go
git commit -m "test(app): 0449 isolation proof goes through the real Status read (change 0454)"
```

---

### Final gate

The build gate (owned by the executing build skill) runs the whole suite via the configured `build.test_command` — never only the tests this plan enumerates — and reads the budget report even on green (`BUDGET WATCH:` / `SERIAL CONFIRMED OVER BUDGET:` lines are findings).

## Self-Review

- **Spec coverage:** Design §1 → Task 1; §2 → Task 2; §3 → Task 2 Step 4 (readiness assertions) — no new logic, pinned by test; §4 (finding + remedy + `recordedBranch` delegation) → Tasks 3–4; §5 (no ADR) → no task, correctly. Testing 1→T1, 2→T2 S1, 3→T2 S4/S5, 4→T5 S1, 5→T5 S2, 6→T6, 7→T3 S1, 8→T4 S5, 9→T2 S6. No gaps found.
- **Placeholder scan:** Tasks 4–6 direct the implementer to reuse an existing named harness in the same file for plumbing while stating the full assertion contract inline — deliberate existing-codebase-pattern reuse, with every new behavioural assertion spelled out. No TBDs.
- **Type consistency:** `ValidBranchName(string) bool` used identically in Tasks 2, 3, 4; `FCBranchMalformed` minted once in Task 3 and consumed in Tasks 3, 5, 6; `branchMalformedCheck(c domain.Change, blobByPath map[string]StatusBlob) []StatusFinding` defined and wired in Task 3 only.
- **Review Focus:** all five lines have owning tests (T2 S6, T3 S1 row 0008, T1/T2/T3/T4 `feat/a:b` rows, T4 S1, T3 S4).
