<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0416 — Scoped build-task gate starts omit prepared scope identity](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-10-0416-scoped-build-task-gate-starts-omit-prepared-scope-identity.md)**
<!-- docket:backlink:end -->
# Scoped Build-Task Gate Starts Omit Prepared Scope Identity — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix the maintained caller contract so a scoped build-task worker's `gate.drive.start` carries the complete identity bundle the prepared recovery scope pinned, and add a mutation-tested whole-repository guard so no maintained scoped task-start instruction can omit it again.

**Architecture:** Pure caller-contract fix — three maintained markdown surfaces (`skills/docket-build-task/SKILL.md`, `skills/docket-build/SKILL.md`, `skills/docket-build/references/gate-caller-loop.md`) plus their generated embedded mirrors, guarded by a new syntactic-shape scan in `internal/repoguard` modeled on `TestGateDriveJSONCapture` (change 0376). The gate driver, `scopeIdentityMatch`, capability/handoff/takeover semantics, and the invalid-request error vocabulary are **not touched** — the driver already enforces the behavior; only the instructions that feed it are mechanically incomplete.

**Tech Stack:** Go tests (`internal/repoguard`), markdown skill contracts, `go generate ./internal/assets/` (cmd/genassets).

**Spec:** none — trivial change. Authoritative statement: the change file `docs/changes/active/0416-scoped-build-task-gate-starts-omit-prepared-scope-identity.md` on the `docket` metadata branch (id 416, acceptance criteria 1–4 plus the whole-suite gate).

## Global Constraints

- **Do not change** `internal/gatedrive/` (driver, `scopeIdentityMatch`, takeover), `internal/cli/gate.go`, capability/handoff/takeover/credential-redaction semantics, or the invalid-request error vocabulary. The CLI already accepts every flag this fix instructs callers to pass.
- Out of scope: identity inference/hydration in the driver; concurrent-scope / duplicate-drive questions (change 405); resuming or implementing change 323.
- The parent capability remains parent-only: it must never enter any worker prompt, and the guarded dispatch payload keeps saying so.
- Preserve verbatim the raw line `Feature worktree: <absolute canonical feature-worktree root>` inside the feature-dispatch block — `internal/repoguard/feature_worktree_dispatch_test.go` requires it exactly.
- Every edited invocation paragraph must keep `--json` — `TestGateDriveJSONCapture` (internal/repoguard/gatedrive_json_capture_test.go) scans these same paragraphs.
- `internal/repoguard/budgets_test.go` pins `docket-build/SKILL.md` at exactly 406 lines / 4054 words and `docket-build-task/SKILL.md` at 160 / 1605 — both currently AT their ceiling, so the edits require a documented one-time upward re-baseline pinned at the exact new counts (house precedent: the 0376/0410 header notes in that file).
- Guard discipline (repo AGENTS.md): key on syntactic shape, never an enumerated list of spellings, for **site discovery** (the required-flag list is the asserted property, not the discovery key); derive sites from a whole-repo scan; mutation-test every protected field with `-count=1` (Go caches otherwise); restore mutations from a backup **copy** (`cp`), never `git checkout --` over uncommitted work.
- Stage by explicit path only — never `git add -A` (shared worktree discipline).
- Whole-suite gate (run by the build controller after the task, not inside it): `go run ./cmd/docket development test`.
- Known populations (verified 2026-09-09 on this branch): exactly 2 `--owner task` instruction sites exist — `skills/docket-build-task/SKILL.md` and its byte-identical mirror `internal/assets/embedded/tree/skills/docket-build-task/SKILL.md`; exactly 1 feature-dispatch marker block exists in `skills/docket-build/SKILL.md` (plus its mirror). The "installed" surface is written verbatim from the embed at install time, so source + regen covers all three surfaces.

---

### Task 1: Guard-first fix — scoped-start identity guard, the three contract edits, embedded regen, budget re-baseline

**Build profile:** premium

Reason: one task carries a new repo-wide guard whose vacuity risk is the named hazard (a green decoration-guard would defeat the change's purpose), mutation probes across mirrored generated surfaces, and exact-count budget re-baselines. Everything is fully specified below; the risk is consequential but correctable.

This is deliberately one task: the new guard **is** the failing regression test that reproduces the bug (RED against the current corpus), and the doc fixes + asset regeneration are what turn it GREEN — neither half is committable alone (a committed RED test breaks the gate; committed doc fixes without the guard ship the fix without its regression test).

**Files:**
- Create: `internal/repoguard/gatedrive_scope_identity_test.go`
- Modify: `skills/docket-build-task/SKILL.md` (the intro hand-off list and the `gate.drive.start` invocation paragraph)
- Modify: `skills/docket-build/SKILL.md` (inside the `<!-- docket:feature-dispatch:start ... -->` block)
- Modify: `skills/docket-build/references/gate-caller-loop.md` (the `| `start` |` operation-table row)
- Modify: `internal/repoguard/budgets_test.go` (two `skillBudgets` rows + one header note)
- Regenerate: `internal/assets/embedded/tree/...` and `internal/assets/embedded/manifest.json` via `go generate ./internal/assets/`

**Interfaces:**
- Consumes: shared `internal/repoguard` package helpers — `guardRoot`, `maintainedPop`, `readMaintained` (guards_test.go), `isWorkflowMD` (gatedriver_test.go), `paragraphs` and the `sharedContractRel` const (gatedrive_json_capture_test.go). Same package: reuse them, do **not** redefine any of them.
- Produces: `TestGateDriveScopedStartIdentity` plus package-level identifiers `requiredScopedStartFlags`, `requiredStartRowFlags`, `requiredBundleElems`, `startOpRe`, `ownerTaskRe`, `bundleBindRe`, `childOnlyRe`, `parentStayRe`, `isScopedTaskStartSite`, `missingScopedStartFlags`, `buildSkillRel`, `buildTaskSkillRel`. Before writing, confirm none collide: `grep -rn "ownerTaskRe\|buildSkillRel\|buildTaskSkillRel\|startOpRe\|childOnlyRe\|parentStayRe\|bundleBindRe\|requiredScopedStartFlags\|requiredStartRowFlags\|requiredBundleElems\|isScopedTaskStartSite\|missingScopedStartFlags" internal/repoguard/` must return nothing; if a name is taken, rename yours (keep the `scopedStart`/`scopeIdentity` stem) consistently through this file.

- [ ] **Step 1: Write the failing guard test**

Create `internal/repoguard/gatedrive_scope_identity_test.go` with exactly this content (adjust only for name collisions found by the grep above):

```go
package repoguard

// Change 0416: gate.drive.prepare-scope pins repository, worktree, change,
// task, phase, and branch identity, and the driver's scopeIdentityMatch
// rejects a scope-bound start whose identity does not match — so a maintained
// instruction telling a worker to start a task-owned drive with only
// --scope-id/--child-cap/--run-root manufactures a guaranteed pre-test halt
// (invalid-request, no drive). Three prongs:
//   (A) corpus scan: every scoped task-start instruction paragraph (syntactic
//       shape: a gate.drive.start reference plus --owner task in one collapsed
//       paragraph) carries the complete flag bundle, including the
//       --gate-context pass-through;
//   (B) the build controller's feature-dispatch payload block hands the worker
//       the complete start-ready scope bundle (and only the child capability);
//   (C) the shared caller contract's start row documents the identity flags.
// Site discovery is keyed on syntactic shape, never a per-file allowlist; the
// required-flag list is the asserted PROPERTY, not the discovery key.
// Residual risk, recorded not hidden: an author who instructs a task-owned
// start without the --owner task token in the same paragraph dodges prong A;
// the driver itself still fails such a start closed at run time
// (scope-identity-mismatch), and prongs B/C carry the contract prose.

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

const (
	buildSkillRel     = "skills/docket-build/SKILL.md"
	buildTaskSkillRel = "skills/docket-build-task/SKILL.md"
)

// requiredScopedStartFlags is the complete bundle a scoped task-owned
// gate.drive.start instruction must carry: the identity flags the prepared
// scope pinned, the scope-binding pair, the gate-context pass-through, and
// the existing transport flags.
var requiredScopedStartFlags = []string{
	"--repo-dir", "--change-id", "--task-id", "--phase", "--branch",
	"--scope-id", "--child-cap", "--gate-context", "--run-root", "--json",
}

// requiredStartRowFlags is what the shared contract's start row must document
// for a scope-bound start (transport flags are documented elsewhere in it).
var requiredStartRowFlags = []string{
	"--repo-dir", "--change-id", "--task-id", "--phase", "--branch",
	"--scope-id", "--child-cap", "--gate-context",
}

// requiredBundleElems is what the controller's dispatch payload must name for
// the worker (matched case-insensitively against the collapsed block).
var requiredBundleElems = []string{
	"feature worktree", "change id", "task id", "phase", "branch",
	"scope id", "child capability", "dispatch context",
}

var (
	startOpRe   = regexp.MustCompile(`gate\.drive\.start`)
	ownerTaskRe = regexp.MustCompile(`--owner task(?:[^a-z-]|$)`)
	// One bounded gap each (two stacked gaps backtrack catastrophically on
	// non-matching input); flag/element presence uses strings.Contains.
	bundleBindRe = regexp.MustCompile(`(?i)complete start-ready scope bundle`)
	childOnlyRe  = regexp.MustCompile(`(?i)child capability only`)
	parentStayRe = regexp.MustCompile(`(?i)parent capability[^.]{0,120}never enters any prompt`)
)

// isScopedTaskStartSite: a collapsed paragraph is a SITE when it references
// gate.drive.start AND carries the --owner task token.
func isScopedTaskStartSite(p string) bool {
	return startOpRe.MatchString(p) && ownerTaskRe.MatchString(p)
}

// missingScopedStartFlags returns the required flags the paragraph omits.
func missingScopedStartFlags(p string) []string {
	var missing []string
	for _, f := range requiredScopedStartFlags {
		if !strings.Contains(p, f) {
			missing = append(missing, f)
		}
	}
	return missing
}

func TestGateDriveScopedStartIdentity(t *testing.T) {
	root := guardRoot(t)

	// Prong A — corpus scan over maintained workflow markdown (isWorkflowMD
	// already includes the internal/assets/embedded/tree mirrors). The shared
	// caller contract is the requirement's definition, not a caller: prong C
	// covers it directly, mirroring TestGateDriveJSONCapture's exclusion.
	var violations []string
	perFile := map[string]int{}
	siteCount := 0
	for _, rel := range maintainedPop(t, root) {
		if !isWorkflowMD(rel) || strings.HasSuffix(rel, "docket-build/references/gate-caller-loop.md") {
			continue
		}
		for _, p := range paragraphs(readMaintained(t, root, rel)) {
			if !isScopedTaskStartSite(p) {
				continue
			}
			siteCount++
			perFile[rel]++
			if missing := missingScopedStartFlags(p); len(missing) != 0 {
				violations = append(violations, fmt.Sprintf(
					"%s: scoped task-start instruction missing %s: %.160s",
					rel, strings.Join(missing, " "), p))
			}
		}
	}

	// Population floors FIRST (a vacuous scan passes every negative): the
	// worker skill and its embedded mirror each contribute a site.
	if siteCount < 2 {
		t.Fatalf("population floor: only %d scoped task-start sites discovered (expected >= 2)", siteCount)
	}
	for _, rel := range []string{
		buildTaskSkillRel,
		"internal/assets/embedded/tree/" + buildTaskSkillRel,
	} {
		if perFile[rel] == 0 {
			t.Errorf("coverage floor: %s contributes no scoped task-start site (scan or corpus drifted)", rel)
		}
	}
	if len(violations) != 0 {
		t.Errorf("scoped task-start identity violations (%d):\n%s", len(violations), strings.Join(violations, "\n"))
	}

	// Prong B — the controller's dispatch payload hands the worker the
	// complete start-ready bundle. The embedded mirror is byte-identical by
	// the assets DiffTree check, so the source copy is the one asserted here.
	// Marker discipline: refuse dangling/duplicate/out-of-order markers.
	lines := strings.Split(readMaintained(t, root, buildSkillRel), "\n")
	startIdx, endIdx := -1, -1
	for i, line := range lines {
		if strings.HasPrefix(line, "<!-- docket:feature-dispatch:start") {
			if startIdx != -1 {
				t.Fatalf("%s: second feature-dispatch start marker at line %d — refusing to guess the block", buildSkillRel, i+1)
			}
			startIdx = i
		}
		if strings.HasPrefix(line, "<!-- docket:feature-dispatch:end") {
			if endIdx != -1 {
				t.Fatalf("%s: second feature-dispatch end marker at line %d", buildSkillRel, i+1)
			}
			endIdx = i
		}
	}
	if startIdx == -1 || endIdx == -1 || endIdx < startIdx {
		t.Fatalf("%s: feature-dispatch markers missing or out of order", buildSkillRel)
	}
	block := strings.ToLower(strings.Join(strings.Fields(strings.Join(lines[startIdx:endIdx+1], " ")), " "))
	if !bundleBindRe.MatchString(block) {
		t.Errorf("%s: dispatch payload lost its 'complete start-ready scope bundle' claim", buildSkillRel)
	}
	for _, elem := range requiredBundleElems {
		if !strings.Contains(block, elem) {
			t.Errorf("%s: dispatch payload bundle lost element %q", buildSkillRel, elem)
		}
	}
	if !childOnlyRe.MatchString(block) {
		t.Errorf("%s: dispatch payload lost the child-capability-only restriction", buildSkillRel)
	}
	if !parentStayRe.MatchString(block) {
		t.Errorf("%s: dispatch payload lost the parent-capability containment clause", buildSkillRel)
	}

	// Prong C — the shared contract's start row documents the identity flags
	// for a scope-bound start.
	var startRows []string
	for _, line := range strings.Split(readMaintained(t, root, sharedContractRel), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "| `start` |") {
			startRows = append(startRows, line)
		}
	}
	if len(startRows) != 1 {
		t.Fatalf("%s: expected exactly one '| `start` |' operation row, found %d", sharedContractRel, len(startRows))
	}
	for _, f := range requiredStartRowFlags {
		if !strings.Contains(startRows[0], f) {
			t.Errorf("%s: start operation row does not document %s", sharedContractRel, f)
		}
	}

	t.Run("non_vacuity", func(t *testing.T) {
		full := "run the `gate.drive.start` operation with `--owner task --repo-dir <w> --change-id <id> --task-id <t> --phase build --branch <b> --scope-id <s> --child-cap <c> --gate-context <g> --run-root <r> --json -- <cmd>`"
		if !isScopedTaskStartSite(full) {
			t.Fatalf("a complete scoped task-start invocation was not classified as a site")
		}
		if missing := missingScopedStartFlags(full); len(missing) != 0 {
			t.Errorf("a complete invocation reported missing flags: %v", missing)
		}
		// MUTATION per protected field: removing each required flag must be
		// detected as exactly that missing flag.
		for _, f := range requiredScopedStartFlags {
			mutated := strings.Replace(full, f, "", 1)
			miss := missingScopedStartFlags(mutated)
			if len(miss) != 1 || miss[0] != f {
				t.Errorf("removing %s was not detected as exactly that missing flag: %v", f, miss)
			}
		}
		buildOwned := "the `gate.drive.start` operation with `--owner build --json` — capture the drive id and owner generation"
		if isScopedTaskStartSite(buildOwned) {
			t.Errorf("a build-owned start was wrongly classified as a scoped task-start site")
		}
		mention := "a scoped `gate.drive.start` binds the drive into the prepared recovery scope"
		if isScopedTaskStartSite(mention) {
			t.Errorf("a prose mention without --owner task was wrongly classified as a site")
		}
		if isScopedTaskStartSite("the `gate.drive.start` operation for the --owner taskforce") {
			t.Errorf("--owner task token boundary failed: 'taskforce' matched")
		}
		wrapped := "the `gate.drive.start` operation\nwith `--owner task\n--scope-id <id>`"
		if got := paragraphs(wrapped); len(got) != 1 || !isScopedTaskStartSite(got[0]) {
			t.Errorf("whitespace collapse failed: a wrapped scoped start was not one matchable paragraph")
		}
		if parentStayRe.MatchString("the parent capability is held. it never enters any prompt") {
			t.Errorf("bounded gap failed: parent-capability clause bind must not span sentences")
		}
	})
}
```

- [ ] **Step 2: Run the guard and verify it FAILS for the intended reason (the regression reproduction)**

Run: `go test ./internal/repoguard/ -run TestGateDriveScopedStartIdentity -count=1 -v 2>&1 | tail -30`

Expected: FAIL with, at minimum, all of:
- Prong A: 2 violations — `skills/docket-build-task/SKILL.md` and `internal/assets/embedded/tree/skills/docket-build-task/SKILL.md`, each `missing --repo-dir --change-id --task-id --phase --branch --gate-context` (the sites already carry `--scope-id --child-cap --run-root --json`).
- Prong B: the `complete start-ready scope bundle` claim missing, and at least elements `"change id"` and `"task id"` missing (`phase`/`branch`/`dispatch context` are pre-satisfied by the block's `prepare-scope` invocation text — that is expected, not a defect).
- Prong C: the start row missing `--repo-dir --change-id --task-id --phase --branch`.

If it fails for any *other* reason (compile error, helper collision, floor miscount), fix the test — do not proceed on an unread failure. This RED run is the failing regression test required for a fix change: it reproduces the exact caller-contract omission that halted change 0323.

- [ ] **Step 3: Fix the worker contract — `skills/docket-build-task/SKILL.md`**

Two edits. First the intro hand-off list (so "comes in your dispatch prompt" below stays true end-to-end). Replace:

```markdown
You own **exactly one task** from the implementation plan, handed to you in your prompt along with
the branch, the worktree, the selected build profile, the routing reason, and the drive scope id
and child capability for this task. You are a fresh worker: nothing carries over from earlier tasks
```

with:

```markdown
You own **exactly one task** from the implementation plan, handed to you in your prompt along with
the branch, the worktree, the selected build profile, the routing reason, and this task's
start-ready drive scope bundle — its scope id, child capability, and every identity value the
scope pinned. You are a fresh worker: nothing carries over from earlier tasks
```

Second, the invocation instruction. Replace:

```markdown
**Every test execution this task runs — baseline, RED, GREEN, focused re-run, ad-hoc
verification — starts through the native gate driver.** Use the task-intent owner: the
`gate.drive.start` operation with `--owner task --scope-id <id> --child-cap <token> --run-root
<task-scratch-dir> --json -- <the test command>`. The scope id and child capability come in your dispatch
prompt; the run root is a scratch dir you pick and read from.
```

with:

```markdown
**Every test execution this task runs — baseline, RED, GREEN, focused re-run, ad-hoc
verification — starts through the native gate driver.** Start it from the canonical feature
worktree, and use the task-intent owner: the `gate.drive.start` operation with `--owner task
--repo-dir <feature-worktree> --change-id <id> --task-id <task-N> --phase build --branch <branch>
--scope-id <id> --child-cap <token> --gate-context <token> --run-root
<task-scratch-dir> --json -- <the test command>`. Every identity value comes in your dispatch
prompt — pass the bundle through unchanged, omitting `--gate-context` only when no dispatch
context was handed to you; the prepared scope pinned exactly this identity, and the driver
rejects a start that omits or alters any of it. The run root is a scratch dir you pick and read from.
```

Leave the following sentence ("Capture the drive id and owner generation from that `--json` response…") and everything after it untouched.

- [ ] **Step 4: Fix the controller dispatch payload — `skills/docket-build/SKILL.md`**

Inside the `<!-- docket:feature-dispatch:start ... -->` block, replace:

```markdown
It also gives the worker the plan task text, branch, applicable repository instructions, selected
profile and routing reason, **scope id and child capability only**, and completion schema; the parent
capability stays in your notes and never enters any prompt, log, or report. Never dispatch a task reviewer, and
```

with:

```markdown
It also gives the worker the plan task text, applicable repository instructions, selected
profile and routing reason, the completion schema, and one **complete start-ready scope bundle**:
the change id, task id, phase (`build`), branch, scope id, child capability, and the dispatch
context when your prompt carried one — each value exactly as `prepare-scope` pinned it, for the
worker to pass through to `gate.drive.start` unchanged. Of the two capabilities the worker
receives the child capability only; the parent
capability stays in your notes and never enters any prompt, log, or report. Never dispatch a task reviewer, and
```

Do not touch the markers, the `prepare-scope` invocation sentences above it, or the raw `Feature worktree: <absolute canonical feature-worktree root>` line.

- [ ] **Step 5: Fix the documented start flags — `skills/docket-build/references/gate-caller-loop.md`**

Replace the `start` operation-table row:

```markdown
| `start` | Fingerprint the execution context, launch the first raw run through the supervisor, advance one slice, and return the drive id, owner generation, and disposition. Optional `--scope-id <id> --child-cap <token> --gate-context <token>` bind the new drive into a recovery scope. |
```

with:

```markdown
| `start` | Fingerprint the execution context, launch the first raw run through the supervisor, advance one slice, and return the drive id, owner generation, and disposition. A scope-bound start passes the complete identity the scope pinned — `--repo-dir <worktree> --change-id <id> --task-id <id> --phase <name> --branch <name> --scope-id <id> --child-cap <token>`, plus `--gate-context <token>` when the dispatch carried one — and the driver rejects a start whose identity does not match the prepared scope. |
```

Leave the `prepare-scope` row untouched — it is already accurate.

- [ ] **Step 6: Regenerate the embedded skill assets**

Run: `go generate ./internal/assets/`
Then: `git -C . status --porcelain` — expect modifications only under `skills/docket-build-task/`, `skills/docket-build/`, `internal/assets/embedded/tree/skills/docket-build-task/`, `internal/assets/embedded/tree/skills/docket-build/`, `internal/assets/embedded/manifest.json`, plus your new test file and (later) `budgets_test.go`. Any other modified path is not yours — leave it and report it.

- [ ] **Step 7: Re-baseline the two size-budget rows — `internal/repoguard/budgets_test.go`**

Measure exact new sizes: `wc -l -w skills/docket-build/SKILL.md skills/docket-build-task/SKILL.md skills/docket-build/references/gate-caller-loop.md`
(`wc -l` equals the guard's `wcLines`; `wc -w` equals `wcWords`.) The gate-caller-loop file stays under its existing 175/1750 row — verify, and leave that row alone.

Update the two `skillBudgets` rows to the exact measured numbers (never rounded headroom — the ratchet stays exact):

```go
	{"docket-build/SKILL.md", <exact lines>, <exact words>}, // 0416: +complete start-ready scope bundle in the worker dispatch payload (see note above)
```
```go
	{"docket-build-task/SKILL.md", <exact lines>, <exact words>}, // 0416: +scoped start identity bundle (see note above)
```

And append this note to the header comment block above `var skillBudgets` (house style, after the existing 0349 note):

```go
// Change 0416 re-baselined docket-build/SKILL.md and docket-build-task/SKILL.md
// upward once to hold the complete start-ready scope-bundle contract: the
// controller's dispatch payload hands the worker every identity value
// prepare-scope pinned, and the worker passes the bundle through to its scoped
// task-owned gate.drive.start unchanged — closing the identity omission that
// made the driver reject every scoped build-task start before its first test.
// Authored contract documentation, not slack — the ceilings are pinned at the
// exact new counts, so the ratchet still reddens on any further regrowth.
```

- [ ] **Step 8: Verify GREEN across the affected guard surfaces**

Run: `go test ./internal/repoguard/ ./internal/assets/ -count=1`

Expected: PASS — in particular `TestGateDriveScopedStartIdentity` (the new guard, now green), `TestGateDriveJSONCapture` (edited paragraphs still carry `--json`), `TestFeatureWorktreeDispatch`-family checks in `feature_worktree_dispatch_test.go` (exact payload line preserved), `TestSkillSizeBudgets` (re-baselined rows), and `TestEmbeddedMatchesAuthored` (source and embedded byte-aligned). Read any failure; do not weaken a guard to pass.

- [ ] **Step 9: Mutation-test the guard against the real files (every protected surface)**

The in-test `non_vacuity` loop already proves per-flag detection on the synthetic site, and Step 2's RED proved detection on the real corpus. Now prove the shipped guard reddens against real-file mutations. All probes use `-count=1`; restore from a backup **copy** every time (the working tree is uncommitted — `git checkout --` would destroy your own edits).

Prong A — for each required flag, remove its token from the source worker skill and confirm red for the intended reason:

```bash
cd <feature-worktree>
F=skills/docket-build-task/SKILL.md
for flag in --repo-dir --change-id --task-id --phase --branch --scope-id --child-cap --gate-context --run-root --json; do
  cp "$F" "$F.bak"
  python3 -c "
import sys
p, flag = '$F', '$flag'
s = open(p).read()
i = s.index(flag)
open(p, 'w').write(s[:i] + s[i+len(flag):])
"
  out=$(go test ./internal/repoguard/ -run TestGateDriveScopedStartIdentity -count=1 2>&1); status=$?
  mv -f "$F.bak" "$F"
  if [ $status -eq 0 ]; then echo "MUTATION NOT DETECTED: $flag"; else echo "ok red: $flag"; fi
  grep -F -e "missing $flag" <<<"$out" | head -1
done
```

Every line must print `ok red:` and a violation naming exactly the removed flag. (Note `python3 -c` with `.index` fails loudly if the token is absent; the first occurrence in this file is the invocation paragraph's. `--json` removal here trips only the focused test since nothing else runs; `--phase`/`--branch` removal leaves a dangling `build`/`<branch>` word — harmless, the file is restored.)

Population-floor probe: `cp "$F" "$F.bak"`, edit the invocation's `--owner task` to `--owner build` in the source copy only, run the focused test, expect red on the **coverage floor** for `skills/docket-build-task/SKILL.md` (siteCount stays ≥ 2 via the embedded mirror — the per-file floor is what catches a single drifted copy), then `mv -f "$F.bak" "$F"`.

Prong B probes on `skills/docket-build/SKILL.md` (same `cp`/`mv -f` backup pattern, focused test between):
1. Delete the words `task id, ` from the bundle sentence → expect red: `dispatch payload bundle lost element "task id"`.
2. Change `start-ready` to `startready` → expect red: lost `complete start-ready scope bundle` claim.
3. Delete the sentence fragment `receives the child capability only; ` → expect red: lost the child-capability-only restriction.
Restore after each.

Prong C probe on `skills/docket-build/references/gate-caller-loop.md`: delete `--branch <name> ` from the start row → expect red: `start operation row does not document --branch`. Restore.

After all probes: `git status --porcelain` must show no `.bak` files and the same modified-path set as Step 6, and `go test ./internal/repoguard/ ./internal/assets/ -count=1` must pass again (proves every restore landed).

- [ ] **Step 10: Confirm acceptance criterion 2 via the existing driver/CLI regressions (no code changes)**

Run: `go test ./internal/gatedrive/ ./internal/cli/ -count=1`

Expected: PASS. This is the existing enforcement the fix relies on, unchanged: `internal/gatedrive` covers `scopeIdentityMatch` (empty/mismatched identity on a scope-bound start → scope-identity-mismatch ownership failure, mapped to invalid-input/invalid-request with no drive), and `internal/cli/gate_test.go` (`TestStartBindsScope` and its siblings) covers the complete-bundle start producing an applied drive with parent takeover attribution preserved. If any of these tests fail, stop and diagnose — do not modify driver, CLI, or their tests to get past it.

- [ ] **Step 11: Self-review the diff, then commit (one commit, explicit paths)**

Review `git diff` against the boundaries: no changes outside the files listed in this task; markers balanced; the parent-capability sentence intact; no `.bak` leftovers.

```bash
cd <feature-worktree>
git add internal/repoguard/gatedrive_scope_identity_test.go \
        internal/repoguard/budgets_test.go \
        skills/docket-build-task/SKILL.md \
        skills/docket-build/SKILL.md \
        skills/docket-build/references/gate-caller-loop.md \
        internal/assets/embedded/tree/skills/docket-build-task/SKILL.md \
        internal/assets/embedded/tree/skills/docket-build/SKILL.md \
        internal/assets/embedded/manifest.json
git commit -m "fix(0416): scoped task-owned gate starts carry the prepared scope's full identity bundle

The docket-build-task worker contract started task-owned drives with only
--scope-id/--child-cap/--run-root, omitting the --repo-dir/--change-id/
--task-id/--phase/--branch identity prepare-scope pinned — so the driver's
scopeIdentityMatch correctly rejected every scoped worker start before its
first test. Require the controller to hand the worker one complete
start-ready scope bundle (dispatch context included when present; parent
capability still parent-only) and the worker to pass it through unchanged;
document the identity flags in the shared caller contract's start row;
guard every maintained scoped task-start site with a mutation-tested
syntactic-shape scan (TestGateDriveScopedStartIdentity); regenerate the
embedded skill assets; re-baseline the two size-budget rows at exact counts."
```

Check `git status` afterward: the manifest path is staged only if `go generate` actually changed it; if `git add` reports a listed path unmodified, that is fine — commit what changed, and every changed path must be in the list above.

---

## Verification after the plan (controller-owned, not a task)

- The build controller runs the full-suite gate: `go run ./cmd/docket development test` — the configured `build.test_command`. All green, including the new guard, budgets, embedded alignment, and the untouched gatedrive/CLI suites. Read the budget report lines even on green (`BUDGET WATCH:` / `SERIAL CONFIRMED OVER BUDGET:`).

## Self-review notes (plan against the change's acceptance criteria)

1. **AC1** — Task 1 Steps 3–5 make the worker start from the canonical feature worktree with the complete bundle (`--repo-dir --change-id --task-id --phase --branch --scope-id --child-cap`, `--gate-context` when present, plus `--owner task --run-root --json`), make the controller's dispatch payload carry the full start-ready bundle while keeping the parent capability parent-only, and update the shared contract's start row. The CLI resolves `Worktree` from `--repo-dir` (see the change-0359 comment in `internal/cli/gate.go`), which is why "start it from the canonical feature worktree" and `--repo-dir <feature-worktree>` together satisfy the worktree/repository identity conjuncts.
2. **AC2** — Step 2's RED reproduces the rejected former shape at the contract level; Step 10 confirms the driver-level behavior via existing tests only; Global Constraints forbid touching the driver, `scopeIdentityMatch`, capability/handoff/takeover/redaction semantics, and the error vocabulary.
3. **AC3** — the new guard derives sites from the whole maintained workflow-markdown corpus (`isWorkflowMD`, embedded mirror included), keys discovery on syntactic shape, enforces population and per-file coverage floors, and is mutation-tested per protected field both in-test (Step 1's `non_vacuity` loop) and against the real files (Step 9), gate-context pass-through included.
4. **AC4** — Step 6 regenerates via `go generate ./internal/assets/`; `TestEmbeddedMatchesAuthored` enforces byte alignment in Step 8 and at the gate.
5. **AC5** — the whole-suite gate is the controller's, named above.
