<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0344 — Finalize PR prober cannot parse the full-URL pr: form](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-24-0344-finalize-pr-prober-cannot-parse-the-full-url-pr-form.md)**
<!-- docket:backlink:end -->
# Finalize PR Prober Full-URL `pr:` Parsing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: this plan is executed by docket's build role (`docket-build`), task-by-task under the docket-build-task contract. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Teach finalize's only two `pr:`-number parsers to accept the board-required full-URL form (`https://github.com/owner/repo/pull/N`) as well as the `owner/repo#N` shorthand, by routing both through one shared extractor.

**Architecture:** A single new unexported `parsePRRef(ref string) (int, bool)` in `internal/app/finalize_context.go` handles both forms — checking `/pull/` before `#` so a URL fragment is never misread as the number — and both existing functions (`parsePRNumber`, `prNumberToken`) become thin delegates, so they can never again diverge on which forms they accept. Because the widening lands in `parsePRNumber` itself, all five of its call sites (probe `finalize_context.go`, cleanup `finalize_cleanup.go`, closeout `finalize_closeout.go` ×2, merge `finalize_merge.go`) inherit the URL form with no further edits.

**Tech Stack:** Go (`internal/app`, package `app`), standard `strings`/`strconv` only — no new dependencies. Tests are table-driven package-internal tests beside `internal/app/finalize_context_test.go`.

**Spec:** `docs/superpowers/specs/2026-08-24-finalize-pr-prober-cannot-parse-the-full-url-pr-form-design.md` (synchronized on the `docket` metadata branch; change 0344)

## Global Constraints

- The shared extractor requires a **positive** integer in both forms; `(0, false)` otherwise. This deliberately tightens `prNumberToken` (which historically ran bare `strconv.Atoi` and would accept `0`/negative) — spec Assumption 3 audits this as safe: its sole caller folds the token into unknown-facts records where a non-positive "number" is meaningless.
- URL parse rule (spec "What changes" item 3): after `/pull/`, the number segment runs to the next `/`, `?`, `#`, or end-of-string, and must be a non-empty all-digit positive integer. Accepts canonical `.../pull/N`, trailing slash, `?query`, `#fragment`, and sub-page (`.../pull/N/files`); rejects `.../pull/abc` and a missing number.
- `/pull/` is checked **before** `#` (spec Assumption 2) — a `#`-first reader would misread `.../pull/235#discussion` as fragment-number.
- Do **not** reuse change 0341's `link_context.go` helpers (spec Assumption 4): they live on 0341's unmerged branch and parse the opposite direction.
- Go's test cache can serve a stale pass against a mutated tree: every verification and mutation-probe run in this plan uses `-count=1` (learnings: `cached-runner-serves-a-mutated-tree`).
- Full-suite gate command: `scripts/run-tests.sh` (the repo's `finalize.test_command`); run from the feature worktree root.
- All work happens in the feature worktree `/Users/homer/dev/docket/.worktrees/finalize-pr-prober-cannot-parse-the-full-url-pr-form` on branch `feat/finalize-pr-prober-cannot-parse-the-full-url-pr-form`.

---

### Task 1: Shared `parsePRRef` extractor with both parsers delegating

**Files:**
- Modify: `internal/app/finalize_context.go` (functions `prNumberToken` and `parsePRNumber`; add `parsePRRef` beside `parsePRNumber`)
- Test: `internal/app/finalize_context_test.go` (append two table-driven tests)

**Interfaces:**
- Consumes: existing unexported `parsePRNumber(ref string) (int, bool)` and `prNumberToken(ref string) string` in package `app`.
- Produces: unexported `parsePRRef(ref string) (int, bool)` — the single source of truth both delegate to. Task 2's selector-level test relies on `parsePRNumber` (and therefore `githubFinalizeProber.ProbePR`) accepting the URL form.

- [ ] **Step 1: Write the failing tests**

Append to `internal/app/finalize_context_test.go` (no new imports needed — `testing` and `fmt` are already imported):

```go
// TestParsePRNumberForms: parsePRNumber (via the shared parsePRRef) accepts both
// the canonical owner/repo#N shorthand and the board-required full-URL
// .../pull/N form — including benign URL suffixes, since the number after
// /pull/ is unambiguous in every one — and rejects non-positive or non-numeric
// references. The /pull/ check runs before the # fallback, so a URL fragment is
// never misread as the number.
func TestParsePRNumberForms(t *testing.T) {
	cases := []struct {
		name string
		ref  string
		want int
		ok   bool
	}{
		{"shorthand", "acme/widgets#42", 42, true},
		{"url canonical", "https://github.com/acme/widgets/pull/235", 235, true},
		{"url trailing slash", "https://github.com/acme/widgets/pull/235/", 235, true},
		{"url query", "https://github.com/acme/widgets/pull/235?w=1", 235, true},
		{"url fragment", "https://github.com/acme/widgets/pull/235#discussion_r1", 235, true},
		{"url sub-page", "https://github.com/acme/widgets/pull/235/files", 235, true},
		{"url non-numeric", "https://github.com/acme/widgets/pull/abc", 0, false},
		{"url missing number", "https://github.com/acme/widgets/pull/", 0, false},
		{"url zero", "https://github.com/acme/widgets/pull/0", 0, false},
		{"url signed", "https://github.com/acme/widgets/pull/+42", 0, false},
		{"shorthand zero", "acme/widgets#0", 0, false},
		{"shorthand negative", "acme/widgets#-1", 0, false},
		{"garbage", "not a pr ref", 0, false},
		{"empty", "", 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n, ok := parsePRNumber(tc.ref)
			if n != tc.want || ok != tc.ok {
				t.Errorf("parsePRNumber(%q) = (%d, %v), want (%d, %v)", tc.ref, n, ok, tc.want, tc.ok)
			}
		})
	}
}

// TestPRNumberTokenForms: prNumberToken carries the canonical number for both
// reference forms and returns "" otherwise — it never fabricates a number. The
// rejection of a non-positive shorthand is a deliberate tightening: the twin
// parsers now share parsePRRef, and prNumberToken's sole caller folds the token
// into unknown-facts records, where a zero or negative "number" is meaningless.
func TestPRNumberTokenForms(t *testing.T) {
	cases := []struct {
		ref  string
		want string
	}{
		{"acme/widgets#42", "42"},
		{"https://github.com/acme/widgets/pull/235", "235"},
		{"https://github.com/acme/widgets/pull/235/files", "235"},
		{"acme/widgets#0", ""},
		{"acme/widgets#-7", ""},
		{"https://github.com/acme/widgets/pull/abc", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := prNumberToken(tc.ref); got != tc.want {
			t.Errorf("prNumberToken(%q) = %q, want %q", tc.ref, got, tc.want)
		}
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test -count=1 -run 'TestParsePRNumberForms|TestPRNumberTokenForms' ./internal/app/ -v`
Expected: FAIL — every `url *` case of `TestParsePRNumberForms` fails (current parser sees no `#` in a canonical URL, and misreads the `url fragment` case), and `TestPRNumberTokenForms` fails on the URL rows plus `acme/widgets#0` / `acme/widgets#-7` (current `prNumberToken` accepts non-positive). Shorthand-positive rows pass. If instead it fails to compile, fix the test code first — the functions under test already exist.

- [ ] **Step 3: Implement `parsePRRef` and delegate both parsers**

In `internal/app/finalize_context.go`, replace the whole `prNumberToken` function (currently the `strings.LastIndex(ref, "#")` implementation under the comment beginning `// prNumberToken extracts the trailing "#<n>" number`) with:

```go
// prNumberToken extracts the canonical PR number from a PR reference in either
// accepted form (see parsePRRef), returning "" when the reference has no
// parseable positive number. It never fabricates a number.
func prNumberToken(ref string) string {
	n, ok := parsePRRef(ref)
	if !ok {
		return ""
	}
	return strconv.Itoa(n)
}
```

Replace the whole `parsePRNumber` function (currently the `strings.LastIndex(ref, "#")` implementation under the comment beginning `// parsePRNumber extracts the positive PR number`) with:

```go
// parsePRNumber extracts the positive PR number from a canonical reference in
// either accepted form (see parsePRRef). It returns false for a reference with
// no parseable positive number.
func parsePRNumber(ref string) (int, bool) { return parsePRRef(ref) }

// parsePRRef is the single source of truth for reading a PR number out of a
// pr: reference. It accepts both canonical forms:
//
//   - the full GitHub URL — ".../pull/N", tolerating a trailing slash, a
//     "?query", a "#fragment", or a deeper sub-page (".../pull/N/files"),
//     because the number immediately after "/pull/" is unambiguous in every
//     one of those shapes;
//   - the "owner/repo#N" shorthand — the integer after the last "#".
//
// The "/pull/" check runs before the "#" fallback so a URL fragment is never
// mistaken for the number. Both forms require a positive integer; anything
// else — a non-numeric segment, a missing number, zero or negative — returns
// (0, false). Both parsePRNumber and prNumberToken delegate here so the two
// can never diverge on which forms they accept.
func parsePRRef(ref string) (int, bool) {
	if i := strings.Index(ref, "/pull/"); i >= 0 {
		seg := ref[i+len("/pull/"):]
		if j := strings.IndexAny(seg, "/?#"); j >= 0 {
			seg = seg[:j]
		}
		if seg == "" {
			return 0, false
		}
		for _, r := range seg {
			if r < '0' || r > '9' {
				return 0, false
			}
		}
		n, err := strconv.Atoi(seg)
		if err != nil || n <= 0 {
			return 0, false
		}
		return n, true
	}
	i := strings.LastIndex(ref, "#")
	if i < 0 || i+1 >= len(ref) {
		return 0, false
	}
	n, err := strconv.Atoi(ref[i+1:])
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}
```

Note the all-digit loop in the URL branch: it implements the spec's "all-digit positive integer" rule and rejects `+42`, which bare `strconv.Atoi` would accept. The shorthand branch keeps `strconv.Atoi` exactly as `parsePRNumber` has always used it — no behavior change for shorthand-form callers beyond `prNumberToken`'s audited tightening.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -count=1 -run 'TestParsePRNumberForms|TestPRNumberTokenForms' ./internal/app/ -v`
Expected: PASS, all subtests.

- [ ] **Step 5: Run the package tests to verify no shorthand regression**

Run: `go test -count=1 ./internal/app/`
Expected: PASS — the existing finalize fixtures all use the shorthand form (`prRefFor` renders `acme/widgets#N`), so this run proves the shorthand path is untouched.

- [ ] **Step 6: Commit**

```bash
git add internal/app/finalize_context.go internal/app/finalize_context_test.go
git commit -m "fix(0344): shared parsePRRef accepts full-URL and shorthand pr: forms"
```

---

### Task 2: Selector-level proof that a URL-form `pr:` is no longer `pr-unknown`

**Files:**
- Test: `internal/app/finalize_context_test.go` (append one helper + one test)

**Interfaces:**
- Consumes: `parsePRRef` routing from Task 1 (via `githubFinalizeProber.ProbePR`, which calls `parsePRNumber` first and refused URL refs before Task 1); the existing test doubles in package `app`: `finalizeBlob(id, slug, status, priority, prRef, extra string) StatusBlob`, `finalizeDeps(fake *fakeReader, prober FinalizePRProber, engine *recordingEngine) FinalizeDeps`, `docketPin(t)`, and — from `internal/app/finalize_closeout_test.go`, same package — `fakeCloseoutGitHub` with its `closeoutProbe{outcome, facts}` entry type (its unused seam methods panic, which is exactly the loudness we want).
- Produces: `prURLFor(number int) string` test helper (the URL-form sibling of `prRefFor`), available to any later test.

- [ ] **Step 1: Write the test**

The pre-0344 failure lives in the **production prober**: `githubFinalizeProber.ProbePR` refused a URL ref with "carries no parseable number" before ever contacting GitHub, and `probeFinalizeFacts` folded that error into unknown facts, so the selector reported `pr-unknown`. The existing selector tests bypass that code (their `fakeFinalizeProber` never parses), so this test wires the **real** prober (`NewGitHubFinalizeProber`) over a scripted `FinalizeGitHub` fake — reusing `fakeCloseoutGitHub`, whose scripted path answers exactly the calls a merged-outcome probe makes (`DiscoverRepository`, `ProbeMerged`).

Append to `internal/app/finalize_context_test.go`, and add `"github.com/danielhanold/docket/internal/githubcli"` to that file's import block:

```go
// prURLFor builds the full-URL pr: reference the board requires — the form the
// pre-0344 prober could not parse.
func prURLFor(number int) string {
	return fmt.Sprintf("https://github.com/acme/widgets/pull/%d", number)
}

// TestContextFinalizeURLFormPRRef: a change whose pr: is the board-required
// full-URL form flows through the PRODUCTION prober (which parses the ref
// before contacting GitHub) and surfaces as a probed merged-recovery candidate
// — never pr-unknown. Before 0344 the prober refused the ref with "carries no
// parseable number" and the selector read pr-unknown, making the change
// un-finalizable through the binary.
func TestContextFinalizeURLFormPRRef(t *testing.T) {
	pin := docketPin(t)
	corpus := []StatusBlob{finalizeBlob(90, "urlform", "implemented", "high", prURLFor(235), "")}
	gh := &fakeCloseoutGitHub{
		repo: githubcli.Repository{Host: "github.com", Owner: "acme", Name: "widgets"},
		merged: map[int]closeoutProbe{
			235: {outcome: githubcli.MergeAlreadyMerged, facts: githubcli.MergedFacts{
				Version: "v235", HeadOID: "h235", BaseRef: "main",
				MergedAtUTC: "2026-08-24T00:00:00Z", MergeCommit: "m235",
			}},
		},
	}
	fake := &fakeReader{pin: pin, corpus: corpus}

	got := ContextFinalize(context.Background(), finalizeDeps(fake, NewGitHubFinalizeProber(gh), &recordingEngine{}), "", FinalizeContextRequest{})
	if got.Result != ResultApplied || len(got.Candidates) != 1 {
		t.Fatalf("result=%q reason=%q candidates=%d", got.Result, got.Reason, len(got.Candidates))
	}
	c := got.Candidates[0]
	if c.SkipReason == "pr-unknown" {
		t.Fatalf("URL-form pr: still reads as pr-unknown — the prober refused the reference")
	}
	if c.PR.Verdict != "probed" || c.PR.State != "merged" || c.PR.Number != "235" {
		t.Errorf("PR report = verdict %q state %q number %q, want probed/merged/235", c.PR.Verdict, c.PR.State, c.PR.Number)
	}
	if c.Band != "merged-recovery" {
		t.Errorf("band = %q, want merged-recovery", c.Band)
	}
}
```

- [ ] **Step 2: Run the test to verify it passes**

Run: `go test -count=1 -run TestContextFinalizeURLFormPRRef ./internal/app/ -v`
Expected: PASS. If it fails on a fixture detail (field name, candidate shape), fix the test against the neighboring `TestContextFinalizeSelection` — the assertion targets (`SkipReason`, `PR.Verdict`, `PR.State`, `Band`) are the ones that test already exercises.

- [ ] **Step 3: Mutation-test the guard**

This test was written after the fix, so prove it can redden (repo rule: a guard is code — strip what it guards and watch it fail). In `internal/app/finalize_context.go`, temporarily delete the entire `if i := strings.Index(ref, "/pull/"); i >= 0 { ... }` block from `parsePRRef` (restoring the shorthand-only behavior), then:

Run: `go test -count=1 -run 'TestContextFinalizeURLFormPRRef|TestParsePRNumberForms' ./internal/app/`
Expected: FAIL — `TestContextFinalizeURLFormPRRef` reddens (the selector reads `pr-unknown` again) and `TestParsePRNumberForms` reddens on every `url *` row. A green run here means the mutation did not land or the cache served a stale pass — investigate before proceeding; never accept it.

Restore the mutation with `git checkout -- internal/app/finalize_context.go` (safe here because Task 1's implementation is already committed and Step 3 touched no other hunk of that file), then re-run the same command and confirm PASS.

- [ ] **Step 4: Confirm the caller set inherits the widened parse**

The spec requires the widening to reach cleanup, closeout, and merge — derive the population by grep, never by recall:

Run: `grep -rn 'parsePRNumber\|prNumberToken' internal --include='*.go' | grep -v _test.go`
Expected: exactly the definitions in `finalize_context.go` plus call sites in `finalize_context.go` (`ProbePR`, `probeFinalizeFacts`), `finalize_cleanup.go`, `finalize_closeout.go` (×2), and `finalize_merge.go` — all routed through the two delegates and therefore through `parsePRRef`, with no other parser of a `pr:` number anywhere in the tree. If a new call site or a second parser appears, stop and reassess: the single-source-of-truth claim in the spec no longer holds and the plan must cover it. (Unit coverage in Task 1 pins `parsePRNumber`'s behavior for all of these; the selector test covers the probe path end-to-end, which is where the bug was observable. The cleanup/closeout/merge paths consume the identical delegate and gain no independently observable parse behavior to fixture.)

- [ ] **Step 5: Commit**

```bash
git add internal/app/finalize_context_test.go
git commit -m "test(0344): selector-level proof a URL-form pr: is not pr-unknown"
```

---

### Task 3: Full-suite gate

**Files:**
- None modified — verification only.

**Interfaces:**
- Consumes: the committed work of Tasks 1–2.
- Produces: a green whole-suite run, the build gate's evidence.

- [ ] **Step 1: Run the whole suite**

From the feature worktree root, run: `scripts/run-tests.sh`
Expected: PASS with no `SERIAL CONFIRMED OVER BUDGET:` line. A `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` line is a screening finding to note in the build evidence, per repo rules. Run the whole suite, never only the tests this plan enumerated — the suite command is the repo's resolved `finalize.test_command`.

- [ ] **Step 2: Verify a clean tree**

Run: `git status --porcelain`
Expected: empty output — everything this plan produced is committed on `feat/finalize-pr-prober-cannot-parse-the-full-url-pr-form`. (The plan document itself was committed by the plan writer before the build began.)

---

### Task 4: Stamp the full-URL `pr:` on write, and verify PR identity by parsed number

> **Scope note (human-directed addendum, 2026-08-24):** Tasks 1–3 widened the *reader* so a
> full-URL `pr:` no longer strands finalize at `pr-unknown`. But the *writer* still records the
> `owner/repo#N` shorthand into the manifest `pr:` field — the exact form that renders as bare text
> in the change block and mangles to `[#docket#N](…/pull/docket#N)` on the board. This task closes
> the loop: record the canonical full URL, and migrate the two remaining string-equality identity
> checks onto the `parsePRRef` extractor Task 1 introduced, so the recorded value and the identity
> comparison stay consistent regardless of form.

**The two coupled facts (established by trace, not assumption):**
1. The manifest `pr:` field is written from the `pr-publish` result. That result already carries
   **both** `Reference` (the `spec#number` shorthand, `pr_publish.go` / `finalize_publish.go`) and
   `URL` (the canonical `https://…/pull/N`, sourced from the verified PR snapshot). Only `URL` is
   the board-safe form.
2. The **same** recorded string is re-used as the PR *identity token*: `change_implemented.go`
   (`reference := fmt.Sprintf("%s#%d", ghRepo.Spec(), pr.Number); reference != req.PR`) and
   `run_verify.go` (`reference != recordedPR`) recompute the shorthand live and string-compare it to
   the recorded `pr:`. If the recorded value becomes a URL while these still compare shorthand, both
   checks break (implemented-verify and run-verify would reject the very PR they wrote).

**Files (confirm the exact set by grep — do not trust this list blind; per repo rule, derive gated
sites from a whole-repo grep of `fmt.Sprintf("%s#%d"` and the recorded-`pr:` readers):**
- `internal/app/pr_publish.go` and/or the CLI wiring that plumbs the pr-publish result into the
  mark-implemented request — record `URL`, not `Reference`, as the manifest `pr:` value.
- `internal/app/change_implemented.go` — the `reference != req.PR` identity conjunct.
- `internal/app/run_verify.go` — the `reference != recordedPR` identity conjunct.
- `internal/app/finalize_context.go` — `parsePRRef` (already exists from Task 1; reused, not
  modified, unless a shared comparison helper belongs beside it).
- Their `_test.go` neighbours, plus any e2e/fixture that hard-codes a recorded `pr:` in shorthand
  form (`change_implemented_test.go`, `run_verify_test.go`, finalize e2e fixtures — grep
  `Spec() + "#"` and `#42`/`#8` recorded-PR literals and update them to exercise **both** forms).

**Interfaces:**
- Consumes: `parsePRRef` (Task 1); the `URL` field already on the pr-publish result struct.
- Produces: a manifest `pr:` that is the canonical PR URL; identity conjuncts that accept either
  recorded form by comparing the **parsed PR number** within the already-resolved repository.

**Design decision to honor (record as an ADR — see Step 6):** the migrated identity checks compare
by **parsed PR number**, not by full-string equality. This is sound because the repository is already
pinned upstream of the comparison: `DiscoverRepository` + `FindOpenPullRequestsByHead(ghRepo, branch)`
establish that the live PR belongs to `ghRepo` on the feature branch, so the PR *number* is a complete
discriminator at that point — the `owner/repo` prefix in the old shorthand comparison was redundant
with an already-verified fact. Parse the recorded `pr:` via `parsePRRef`; if it does not parse to a
positive number, that is an identity failure (unchanged severity). Do **not** widen this into a
cross-repo number match in code that has *not* already pinned the repo — if any such site exists, it
keeps a repo-qualified comparison. Verify by grep that the only two recorded-`pr:` identity
comparisons are the two named above.

- [ ] **Step 1: Write the failing tests**

Add/extend package-internal tests proving: (a) a mark-implemented transition records the **URL** form
(not shorthand) as `pr:`; (b) `change_implemented`'s identity conjunct **passes** when the recorded
`pr:` is the URL form and the live PR number matches, and **fails** when the number differs; (c) the
same for `run_verify`'s identity conjunct. Table-drive both recorded forms (URL and legacy shorthand)
so a future regression on either is caught. Use `-count=1` on every run (Global Constraints).

- [ ] **Step 2: Run the tests to verify they fail**

`go test -count=1 ./internal/app/` — expect the new assertions red (writer still records shorthand;
comparisons still string-equal). A test that passes before implementation is not testing the change.

- [ ] **Step 3: Implement**

Record `URL` as the manifest `pr:` at the write boundary; route both identity conjuncts through
`parsePRRef`-based number comparison per the design decision above. Keep `Reference` (shorthand) only
where it is genuinely a human-facing *display* string (protocol result output), not an identity or a
recorded value — confirm each surviving `Reference` use is display-only by reading its consumer.

- [ ] **Step 4: Run the tests to verify they pass**

`go test -count=1 ./internal/app/` — green, including the legacy-shorthand rows (a manifest written
before this change must still verify).

- [ ] **Step 5: Mutation-test the identity guards**

For each migrated conjunct, strip the comparison (or hard-code it to match) and confirm the
number-mismatch row reddens — the guard must be load-bearing, not decoration (repo rule; Global
Constraints).

- [ ] **Step 6: Record the ADR and commit**

The number-based identity decision is non-obvious and must be captured in the immutable ledger:
dispatch the `docket-adr` agent (do not hand-write the ADR) to record *"manifest `pr:` is the
canonical PR URL; PR identity is verified by parsed number within the already-resolved repository,
via `parsePRRef`"*, scoped to the `docket` branch, adding the ADR id to change 0344's `adrs:` so
finalize publishes it at merge (per the superseded-ADR / main-publish conventions). Then commit the
code with a `fix(0344):` subject that names the writer + identity migration; reference the ADR id in
the body.

### Task 5: Full-suite gate (re-run after Task 4)

**Files:**
- None modified — verification only.

**Interfaces:**
- Consumes: the committed work of Task 4 (atop Tasks 1–2).
- Produces: a green whole-suite run; refreshed build evidence.

- [ ] **Step 1: Run the whole suite**

From the feature worktree root, run: `scripts/run-tests.sh`
Expected: PASS with no `SERIAL CONFIRMED OVER BUDGET:` line. Treat a `BUDGET WATCH:` /
`PARALLEL-SENSITIVE:` line as a screening finding to note, per repo rules. Run the whole suite, never
only the enumerated tests.

- [ ] **Step 2: Verify a clean tree**

Run: `git status --porcelain`
Expected: empty output — everything is committed on
`feat/finalize-pr-prober-cannot-parse-the-full-url-pr-form`.
