<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0516 — Remove stale auto_groom comments and fix TestSkillHandoffSites' 'cannot be invoked' match](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-04-0516-remove-stale-auto-groom-comments-and-fix-testskillhandoffsit.md)**
<!-- docket:backlink:end -->
# Stale auto_groom comments and the handoff guard's negation match Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use docket-build to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rewrite three Go comments that still claim an unset `auto_groomable` inherits a repository `auto_groom` setting, and make `TestSkillHandoffSites` read "cannot be invoked" / "can't be invoked" as a negation instead of an invocation.

**Architecture:** Part 1 is comment-only (no behavior change). Part 2 replaces the single regex `negatedInvokeRe` in `internal/repoguard/skill_handoff_sites_test.go` with a shape-keyed pattern (a word ending in `not`, `never` as a word, or a word ending in the `n't` contraction), extends the existing `non_vacuity` subtest with the new cases, updates the file's header comment to state the residual, and mutation-tests the guard in both directions.

**Tech Stack:** Go (stdlib `regexp`, RE2 semantics; `\w` and `\b` are ASCII-only, the typographic apostrophe `’` is matched as a literal UTF-8 rune inside the class), `go test`.

**Spec:** `docs/superpowers/specs/2026-10-04-remove-stale-auto-groom-comments-and-fix-testskillhandoffsit-design.md` (on the `docket` branch; synchronized copy at `/Users/homer/dev/docket/.docket/docs/superpowers/specs/2026-10-04-remove-stale-auto-groom-comments-and-fix-testskillhandoffsit-design.md`)

## Global Constraints

- No behavior change to auto-groom selection. The only consumer (`internal/app/change_groom.go`) treats a stub as auto-groomable only when `AutoGroomable().State == domain.FieldPresent && .Value`; unset and `false` behave identically.
- Leave the generic `OptionalBool` type comment ("absent or valueless (inherit a default) stays distinguishable from an explicit false") unchanged — it describes the type and is true.
- Leave the `auto_groom` row in `internal/config/schema.go`, all skill prose under `skills/`, and the frozen fixture `internal/render/testdata/records/PROVENANCE.md` unchanged.
- New negation pattern, exactly: `(?i)(?:\b\w*not|\bnever|\w+n['’]t)\W+(?:\w+\W+){0,2}invok(?:e|ed|es|ing)\b`. `never` keeps its leading `\b` on purpose so "whenever invoked" is not a negation. Do not widen the pattern to "unable to invoke" / "fails to invoke" (YAGNI); state that residual in the header comment instead.
- If any live wrapper-backed role-skill line under `skills/` changes class under the new pattern (the whole-tree scan reddens, or the `invocations < 3` floor trips), STOP and report BLOCKED — never edit skill prose to fit the guard.
- Every mutation probe runs with `-count=1` (Go's test cache can serve a stale PASS) and restores from a backup copy (`cp f f.bak; mutate; test; mv -f f.bak f`), never `git checkout --`, because the file under mutation carries uncommitted work.
- Cross-references in comments anchor on symbol names or quoted clauses, never line numbers.

## Review Focus

1. A line using a typographic apostrophe (`can’t be invoked`) — expected: classified as a mention, same as `can't`. Pinned in Task 2 Step 1.
2. A line with an invocation AND a "cannot be invoked" clause and no marker — expected: still an invocation (2 verbs vs 1 negation) and still a violation. Pinned in Task 2 Step 1.
3. "whenever" directly before the verb ("runs whenever invoked") — expected: not a negation; still an invocation. Pinned in Task 2 Step 1 (this is the case that reddens if the `\b` before `never` is dropped).
4. A word merely containing `not` mid-word ("annotate", "notable") — expected: no negation, because `\w*not` must be followed by `\W+`. Pinned in Task 2 Step 1.
5. Live skills tree — expected: no wrapper-backed role-skill line flips class; the existing whole-tree scan and its floors stay green. Pinned by Task 2 Step 4 (full `TestSkillHandoffSites` run).

---

### Task 1: Rewrite the three stale auto_groom comments

**Files:**
- Modify: `internal/domain/entities.go` (the `AutoGroomable` field comment on the change spec struct, and the doc comment on `func (c Change) AutoGroomable()`)
- Modify: `internal/app/change_create.go` (the `AutoGroomable *bool` field comment on `ChangeCreateRequest`)

**Interfaces:**
- Consumes: nothing.
- Produces: nothing (comment-only; no signature changes).

- [ ] **Step 1: Re-derive the site list by whole-repo grep (do not trust the hand list)**

Run (from the feature worktree root):

```bash
grep -rn -i -E "inherit.*auto_groom|auto_groom knob|repo's auto_groom|repository's auto_groom|auto_groom (setting|applies)" . \
  --include='*.go' --include='*.md' --include='*.sh' --include='*.yml' \
  | grep -v -E "^\./docs/superpowers/|/results/|testdata/|^\./\.worktrees/"
```

Expected: exactly three hits — `internal/app/change_create.go` (`inherit the repo's auto_groom`), `internal/domain/entities.go` (`unset ⇒ inherit auto_groom`), `internal/domain/entities.go` (`the repository's auto_groom knob applies`). If more maintained sites appear, fix them too in this task with the same wording rule (unset or `false` means not auto-groomable; only `true` opts in).

- [ ] **Step 2: Rewrite the field comment in `internal/domain/entities.go`**

Replace:

```go
	AutoGroomable  OptionalBool   // per-change auto-groom override; unset ⇒ inherit auto_groom
```

with:

```go
	AutoGroomable  OptionalBool   // per-change auto-groom opt-in; only an explicit true opts in
```

(Keep the column alignment `gofmt` produces; run `gofmt -l internal/domain` afterwards and expect no output.)

- [ ] **Step 3: Rewrite the `AutoGroomable()` doc comment in `internal/domain/entities.go`**

Replace:

```go
// AutoGroomable returns the optional per-change auto-groom override. Unset
// (absent or valueless) means the repository's auto_groom knob applies.
func (c Change) AutoGroomable() OptionalBool { return c.spec.AutoGroomable }
```

with:

```go
// AutoGroomable returns the optional per-change auto-groom opt-in. Only an
// explicit true makes a stub auto-groomable; unset (absent or valueless) and
// false both mean it is not. The tri-state is kept so the record writer
// round-trips what the human wrote.
func (c Change) AutoGroomable() OptionalBool { return c.spec.AutoGroomable }
```

- [ ] **Step 4: Rewrite the request-field comment in `internal/app/change_create.go`**

Replace:

```go
	// AutoGroomable is the optional per-change auto-groom override: nil leaves
	// the record unset (inherit the repo's auto_groom); true/false are explicit.
	AutoGroomable *bool `json:"auto_groomable"`
```

with:

```go
	// AutoGroomable is the optional per-change auto-groom opt-in: nil leaves
	// the record unset; true/false are written as given. Only true makes the
	// stub auto-groomable — unset and false both mean it is not.
	AutoGroomable *bool `json:"auto_groomable"`
```

- [ ] **Step 5: Verify the stale claim is gone and the packages still build**

Run the Step 1 grep again. Expected: no output.

Run:

```bash
gofmt -l internal/domain internal/app
go vet ./internal/domain/ ./internal/app/
go test -count=1 ./internal/domain/
```

Expected: `gofmt -l` prints nothing; `go vet` exits 0; `go test` prints `ok`.

- [ ] **Step 6: Commit**

```bash
git add internal/domain/entities.go internal/app/change_create.go
git commit -m "docs(domain): auto_groomable comments say only true opts in"
```

---

### Task 2: Read negation by shape in TestSkillHandoffSites

**Files:**
- Modify: `internal/repoguard/skill_handoff_sites_test.go` (the `negatedInvokeRe` var and its comment, the header comment under "# Classes, each keyed on shape", and the `non_vacuity` subtest)

**Interfaces:**
- Consumes: existing `classifyHandoffSite(text string) handoffClass`, `handoffInvocation`, `handoffMention`, `invokeRe`, `negatedInvokeRe`, `skillMarker` — all in the same file; names unchanged.
- Produces: nothing new for other tasks.

- [ ] **Step 1: Add the failing cases to the `non_vacuity` subtest**

Inside `t.Run("non_vacuity", func(t *testing.T) { ... })`, after the existing `if classifyHandoffSite(prohibited) != handoffMention { ... }` block, add:

```go
		// Negation is read by shape: a word ending in "not" (not, cannot), the
		// n't contraction (straight or typographic apostrophe), or "never" as a
		// whole word.
		for _, neg := range []string{
			"When `docket-review` cannot be invoked, review the branch inline.",
			"When `docket-review` can't be invoked, review the branch inline.",
			"When `docket-review` can’t be invoked, review the branch inline.",
		} {
			if classifyHandoffSite(neg) != handoffMention {
				t.Errorf("a negated invocation was classified as an invocation: %q", neg)
			}
		}
		// A line that invokes AND mentions a negated invocation still invokes
		// (2 verbs vs 1 negation), so an unmarked one must still be caught.
		const mixed = "Invoke `docket-build`; when `docket-build` cannot be invoked, run the plan inline."
		if classifyHandoffSite(mixed) != handoffInvocation || strings.Contains(mixed, skillMarker) {
			t.Errorf("an unmarked invocation beside a negated clause was not classified as a violating invocation")
		}
		// Shape boundaries: "whenever" is not "never", and a word that merely
		// contains "not" mid-word is not a negation.
		for _, inv := range []string{
			"`docket-build` is invoked whenever the plan is ready.",
			"`docket-build` runs whenever invoked by the controller.",
			"Annotate the plan, then `docket-build` is invoked to execute it.",
		} {
			if classifyHandoffSite(inv) != handoffInvocation {
				t.Errorf("an invocation was misread as a negation: %q", inv)
			}
		}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test -count=1 ./internal/repoguard/ -run 'TestSkillHandoffSites/non_vacuity' -v`
Expected: FAIL with three `a negated invocation was classified as an invocation:` errors (the cannot / can't / can’t lines). The mixed and boundary cases already pass under the old pattern.

- [ ] **Step 3: Replace `negatedInvokeRe` and its comment**

Replace:

```go
	// negatedInvokeRe is a prohibition ("do NOT invoke", "never invoked"),
	// which pre-specifies nothing and needs no marker.
	negatedInvokeRe = regexp.MustCompile(`(?i)\b(?:not|never)\W+(?:\w+\W+){0,2}invok(?:e|ed|es|ing)\b`)
```

with:

```go
	// negatedInvokeRe is a prohibition or inability ("do NOT invoke", "never
	// invoked", "cannot be invoked", "can't be invoked"), which pre-specifies
	// nothing and needs no marker. It keys on the negation's shape within two
	// words of the verb: a word ending in "not" (not, cannot), "never" as a
	// whole word (so "whenever" is not one), or a word ending in the n't
	// contraction with a straight or typographic apostrophe.
	negatedInvokeRe = regexp.MustCompile(`(?i)(?:\b\w*not|\bnever|\w+n['’]t)\W+(?:\w+\W+){0,2}invok(?:e|ed|es|ing)\b`)
```

- [ ] **Step 4: Update the header comment and run the whole guard**

In the file header, replace the first class bullet:

```go
//   - an INVOCATION line is one whose invocation verbs ("invoke", "invoked", ...)
//     outnumber its prohibitions ("do NOT invoke", "never invoked"); it must carry
//     the marker;
```

with:

```go
//   - an INVOCATION line is one whose invocation verbs ("invoke", "invoked", ...)
//     outnumber its negations ("do NOT invoke", "never invoked", "cannot be
//     invoked", "can't be invoked"); it must carry the marker. A negation is
//     read by shape (a word ending in "not", "never", or an n't contraction),
//     not by meaning: a negation outside those shapes ("unable to invoke",
//     "fails to invoke") is still read as an invocation;
```

Run: `go test -count=1 ./internal/repoguard/ -run TestSkillHandoffSites -v`
Expected: PASS for both `TestSkillHandoffSites` and `TestSkillHandoffSites/non_vacuity`. The whole-tree scan staying green with the floors met is the live-tree check: if it reddens on a skill line or trips `invocations < 3`, STOP and report BLOCKED naming the line (see Global Constraints).

- [ ] **Step 5: Mutation A — revert the pattern; the new negation cases must redden**

Each mutation below rewrites the whole `negatedInvokeRe` line (anchored on its tab-indented `negatedInvokeRe = regexp.MustCompile(` prefix), prints the line to prove the mutation landed, runs with `-count=1`, and restores from a backup copy.

```bash
f=internal/repoguard/skill_handoff_sites_test.go
cp "$f" "$f.bak"
perl -pi -e 's/^(\tnegatedInvokeRe = regexp\.MustCompile\().*$/$1`(?i)\\b(?:not|never)\\W+(?:\\w+\\W+){0,2}invok(?:e|ed|es|ing)\\b`)/' "$f"
grep -n 'negatedInvokeRe = ' "$f"   # expect the OLD pattern: (?i)\b(?:not|never)\W+...
go test -count=1 ./internal/repoguard/ -run TestSkillHandoffSites 2>&1 | tail -15
mv -f "$f.bak" "$f"
```

Expected: the grep shows the old pattern; the test FAILS with three `a negated invocation was classified as an invocation:` errors (cannot, can't, can’t). If the grep does not show the mutated line, the mutation did not land: make the edit by hand and re-run. A green run from an unlanded mutation is not evidence.

- [ ] **Step 6: Mutation B — count every invoke verb as negated; the invocation cases must redden**

B1 (floor). Mutate only the pattern:

```bash
f=internal/repoguard/skill_handoff_sites_test.go
cp "$f" "$f.bak"
perl -pi -e 's/^(\tnegatedInvokeRe = regexp\.MustCompile\().*$/$1`(?i)\\binvok(?:e|ed|es|ing)\\b`)/' "$f"
grep -n 'negatedInvokeRe = ' "$f"   # expect (?i)\binvok(?:e|ed|es|ing)\b
go test -count=1 ./internal/repoguard/ -run TestSkillHandoffSites 2>&1 | tail -15
mv -f "$f.bak" "$f"
```

Expected: FAIL with `population floor: only 0 marker-checked invocation lines (expected >= 3)`. That `t.Fatalf` stops the parent test before `non_vacuity` runs, so B2 below is needed to see the subtest cases redden.

B2 (subtest). Same mutation, with the floor check turned off in the scratch state:

```bash
f=internal/repoguard/skill_handoff_sites_test.go
cp "$f" "$f.bak"
perl -pi -e 's/^(\tnegatedInvokeRe = regexp\.MustCompile\().*$/$1`(?i)\\binvok(?:e|ed|es|ing)\\b`)/; s/^(\tif )(invocations < 3 \{)$/$1false && $2/' "$f"
grep -n 'negatedInvokeRe = \|invocations < 3' "$f"   # expect the mutated pattern AND "if false && invocations < 3 {"
go test -count=1 ./internal/repoguard/ -run TestSkillHandoffSites 2>&1 | tail -20
mv -f "$f.bak" "$f"
```

Expected: `non_vacuity` FAILS with `an unmarked invocation was not classified as a violating invocation`, `a marked invocation was not classified ...`, `an unmarked invocation beside a negated clause was not classified as a violating invocation`, and three `an invocation was misread as a negation:` errors.

- [ ] **Step 7: Mutation C — strip the marker from one live invocation; the whole-tree scan must redden**

```bash
s=skills/docket-implement-next/SKILL.md
cp "$s" "$s.bak"
perl -pi -e 's/is invoked \*\*DIRECTED to:\*\* review the whole branch/is invoked to review the whole branch/' "$s"
grep -c -F 'is invoked to review the whole branch' "$s"   # expect 1: proves the mutation landed
go test -count=1 ./internal/repoguard/ -run TestSkillHandoffSites 2>&1 | tail -15
mv -f "$s.bak" "$s"
git status --porcelain skills/                              # expect no output: skill restored
```

Expected: the grep prints `1`; the test FAILS with `autonomy-precedence site violations (1)` naming `skills/docket-implement-next/SKILL.md` and `missing "DIRECTED to:"`. After the restore, `git status --porcelain skills/` prints nothing.

- [ ] **Step 8: Mutation D — drop the `\b` before `never`; the "whenever invoked" boundary case must redden**

```bash
f=internal/repoguard/skill_handoff_sites_test.go
cp "$f" "$f.bak"
perl -pi -e 's/\|\\bnever\|/|never|/' "$f"
grep -n 'negatedInvokeRe = ' "$f"   # expect ...not|never|\w+n... (no \b before never)
go test -count=1 ./internal/repoguard/ -run TestSkillHandoffSites 2>&1 | tail -15
mv -f "$f.bak" "$f"
```

Expected: FAIL with `an invocation was misread as a negation:` naming the line "`docket-build` runs whenever invoked by the controller.".

- [ ] **Step 9: Confirm restore and green, then commit**

```bash
go test -count=1 ./internal/repoguard/ -run TestSkillHandoffSites
git status --porcelain
```

Expected: `ok`; `git status --porcelain` lists only `M internal/repoguard/skill_handoff_sites_test.go` (no `.bak` files, no `skills/` changes).

Report each mutation (A–D), the exact edit, and the observed red failure text in the task's return report so it lands in the results file.

```bash
git add internal/repoguard/skill_handoff_sites_test.go
git commit -m "test(repoguard): read handoff negation by shape (cannot, n't, never)"
```

---

## Build gate

After both tasks, the whole suite runs once at the build gate via the resolved `build.test_command` (`go run ./cmd/docket development test`), not only the targeted test. Read the budget report even on a green run.
