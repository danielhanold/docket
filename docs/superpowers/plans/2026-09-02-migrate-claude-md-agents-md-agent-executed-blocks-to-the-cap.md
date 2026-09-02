<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0395 — Migrate CLAUDE.md/AGENTS.md agent-executed blocks to the capability-first idiom](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0395-migrate-claude-md-agents-md-agent-executed-blocks-to-the-cap.md)**
<!-- docket:backlink:end -->
# Migrate CLAUDE.md/AGENTS.md agent-executed blocks to the capability-first idiom — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove every hard-coded `docket run gate-before` / `docket run gate-verdict` / `docket development install` argv from the repo-root instruction files (AGENTS.md, and CLAUDE.md via its symlink), replacing them with the catalog-resolved idiom, and extend the repoguard capability-surface guard so a re-introduced hard-coded argv in either file reddens the suite.

**Architecture:** The "Run gate" block is NOT hand-authored prose — it lives inside the machine-managed `<!-- docket:dispatch:start … -->` block that `docket development install` reconciles from `harness.DispatchInterior(runGate)` (see `internal/harness/dispatch.go` and `internal/reposeed/plan.go`, which splices `harness.DispatchInterior(in.RunGate)` between the markers). The bundled run-gate asset is ALREADY migrated (it byte-matches the authored `cursor-rules/run-gate.md`); the committed block is merely stale. So Task 1 regenerates the managed block through the same emitter, never a prose hand-edit — and reconciles the dispatch-block word budget that grows as a result. Only the "Rebuild the binary after a merge to main" block is hand-authored prose outside the markers; Task 2 migrates it directly and extends `TestCapabilitySurface`'s corpus to cover the two repo-root files, with a mutation battery proving the extended guard is alive. Task 3 verifies embedded/derived assets are untouched and the affected packages are green.

**Tech Stack:** Go (`internal/repoguard`, `internal/harness`), markdown instruction surfaces, `go test`.

**Spec:** `docs/superpowers/specs/2026-09-02-migrate-claude-md-agents-md-agent-executed-blocks-to-the-cap-design.md` (on the `docket` metadata branch; synchronized copy at `.docket/docs/superpowers/specs/…` in the primary tree). Change file: `docs/changes/active/0395-migrate-claude-md-agents-md-agent-executed-blocks-to-the-cap.md` (same branch).

## Global Constraints

- **CLAUDE.md is a symlink to AGENTS.md** (`CLAUDE.md -> AGENTS.md`). Edit ONLY `AGENTS.md`; never replace the symlink with a regular file. Every task that touches the file ends by verifying `test -L CLAUDE.md`.
- Work from the repository root of this worktree; all commands below assume it as cwd (workers with per-call cwd resets must `cd` to the worktree root or use absolute paths).
- Each block's operational contract must survive verbatim in meaning: gate sequencing, "keep the printed key in your own notes", the `gate-*` report-line-not-exit-code discipline, "only `gate-retry-once` authorizes another dispatch", and the schema-bump install-deadlock caveat (spec acceptance criterion 2).
- Do not rewrite anything under `docs/` history (changes, specs, plans, results, Accepted ADRs) other than adding this plan file — the repoguard walk excludes `docs/` anyway.
- Out of scope: new catalog leaves, catalog protocol changes, migrating any other surface, JSON-schema discovery (change 0360).
- Go's test cache serves stale greens against a mutated tree: every mutation probe and every re-verification run uses `-count=1` (learnings: cached-runner-serves-a-mutated-tree).
- Mutation probes use the backup-copy idiom — `cp <file> <backup-outside-repo>`, mutate, prove the mutation landed via `git diff` BEFORE reading the result, run, `mv -f` the backup back — never `git checkout -- <file>` (learnings: mutation-restore-needs-a-backup-copy; keep backups OUTSIDE the repo tree so they never enter the repoguard population).
- The full suite at the build gate is `go run ./cmd/docket development test` (docket-build's own gate runs it; Task 3 only runs the affected packages).

---

### Task 1: Regenerate the managed dispatch block and reconcile the word budget

The committed `docket:dispatch` block in AGENTS.md is stale in two ways: it predates change 0371's never-fall-back sentence in `dispatchPreamble`, and it predates change 0394's catalog-idiom run gate. Regenerating through `harness.DispatchInterior` fixes both at once and makes the committed bytes exactly what the next `docket development install` would write. The regenerated interior is ~419 words, which breaches `dispatchBudget = 400` in `internal/repoguard/budgets_test.go` — that guard's own comment sanctions reconciling to the new actual rounded up to a multiple of 50, i.e. 450 (still strictly below the pre-0334 `dispatchOld = 1156` ceiling).

**Files:**
- Modify: `AGENTS.md` (the managed block between `<!-- docket:dispatch:start … -->` and `<!-- docket:dispatch:end -->` only)
- Modify: `internal/repoguard/budgets_test.go` (`dispatchBudget` constant and its comment)
- Create-then-delete: `scratch-regen-0395/main.go` (throwaway regenerator; MUST NOT be committed)

**Interfaces:**
- Consumes: `harness.DispatchInterior(runGate []byte) string` (`internal/harness/dispatch.go`), the authored `cursor-rules/run-gate.md` payload.
- Produces: a committed AGENTS.md whose dispatch-block interior byte-equals `DispatchInterior(cursor-rules/run-gate.md)` — Task 2's guard extension goes green on the run-gate spellings only because of this task.

- [ ] **Step 1: Confirm the premise — the bundled run-gate asset is already migrated and byte-identical to the authored one**

Run:
```bash
cmp internal/assets/embedded/tree/cursor-rules/run-gate.md cursor-rules/run-gate.md && echo IDENTICAL
```
Expected: `IDENTICAL`. If they differ, STOP — the reconcile premise is wrong; surface it rather than proceeding.

- [ ] **Step 2: Write the throwaway regenerator**

Create `scratch-regen-0395/main.go` (a new directory at the repo root; it is inside the module so it can import `internal/`):

```go
// Package main is a THROWAWAY regenerator for change 0395: it re-renders the
// managed docket:dispatch block in the repo-root AGENTS.md through the same
// emitter the install path uses (harness.DispatchInterior over the run-gate
// payload), so the committed block byte-matches what an install would write.
// Delete this directory after running it; it must never be committed.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/danielhanold/docket/internal/harness"
)

func die(err error) {
	fmt.Fprintln(os.Stderr, "regen:", err)
	os.Exit(1)
}

func main() {
	rg, err := os.ReadFile("cursor-rules/run-gate.md")
	if err != nil {
		die(err)
	}
	interior := harness.DispatchInterior(rg)

	const start = "<!-- docket:dispatch:start (managed by docket — do not hand-edit) -->"
	const end = "<!-- docket:dispatch:end -->"

	raw, err := os.ReadFile("AGENTS.md")
	if err != nil {
		die(err)
	}
	content := string(raw)
	// Marker ORDER AND BALANCE before any range rewrite (AGENTS.md rule:
	// "validate marker order and balance — refuse … and leave the file
	// untouched"): exactly one of each, start before end.
	if strings.Count(content, start) != 1 || strings.Count(content, end) != 1 {
		die(fmt.Errorf("marker balance: need exactly one start and one end marker"))
	}
	si, ei := strings.Index(content, start), strings.Index(content, end)
	if si > ei {
		die(fmt.Errorf("marker order: start marker sits after end marker"))
	}

	updated := content[:si] + start + "\n" + interior + end + content[ei+len(end):]

	// Temp file BESIDE the destination for a same-filesystem atomic rename
	// (AGENTS.md mktemp/atomic-write rules).
	tmp, err := os.CreateTemp(".", "AGENTS.md.regen.*")
	if err != nil {
		die(err)
	}
	if _, err := tmp.WriteString(updated); err != nil {
		die(err)
	}
	if err := tmp.Close(); err != nil {
		die(err)
	}
	// CreateTemp makes 0600; the mode is chmod'd explicitly, never assumed
	// (learnings: promised-file-mode-needs-explicit-chmod).
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		die(err)
	}
	if err := os.Rename(tmp.Name(), "AGENTS.md"); err != nil {
		die(err)
	}
	fmt.Println("regenerated the AGENTS.md docket:dispatch block")
}
```

- [ ] **Step 3: Run it and prove the regeneration landed**

Run:
```bash
go run ./scratch-regen-0395 && git diff --stat AGENTS.md
```
Expected: `regenerated the AGENTS.md docket:dispatch block`, and a non-empty diff touching ONLY `AGENTS.md`. Then inspect the diff:
```bash
git diff AGENTS.md
```
Expected content changes, all inside the marker block: the preamble gains the "Never reroute a registered workflow through a shell runner, another harness, a generic agent, or an inline reconstruction of its contract …" sentence (change 0371), and the Run gate steps now read "run the `run.gate-before` operation with the `implement-next` argument" / "run the `run.gate-verdict` operation with `<key>`" instead of literal `docket run gate-before` / `docket run gate-verdict` argv, with the preamble line "resolve each operation below from the capability catalog and run it with the flags shown". Nothing outside the markers may change. Verify the operational contract survived: steps 1–5 are all present, "keep the printed key in your own notes", "Obey the facade's `gate-*` report line exactly", and "Only `gate-retry-once` authorizes another dispatch" all still appear.

- [ ] **Step 4: Verify the symlink and byte-parity with the emitter**

Run:
```bash
test -L CLAUDE.md && echo SYMLINK-OK
cmp <(awk '/docket:dispatch:start/{f=1;next} /docket:dispatch:end/{f=0} f' AGENTS.md) <(awk '/docket:dispatch:start/{f=1;next} /docket:dispatch:end/{f=0} f' CLAUDE.md) && echo SAME-THROUGH-SYMLINK
```
Expected: `SYMLINK-OK` and `SAME-THROUGH-SYMLINK`.

- [ ] **Step 5: Run the dispatch-block budget guard — expect it to FAIL (the failing test for the budget reconcile)**

Run:
```bash
go test ./internal/repoguard/ -run 'TestDispatchBlockBudget' -count=1
```
Expected: FAIL with "the AGENTS.md dispatch block is N words, over its 400-word budget" where N is approximately 419. Record the exact N — the next step's comment pins it. (If this PASSES, stop and investigate: either the regeneration did not land or the interior shrank unexpectedly.)

- [ ] **Step 6: Reconcile `dispatchBudget` to the new actual**

In `internal/repoguard/budgets_test.go`, change the constant (substituting the actual N from Step 5 for 419 if it differs):

```go
	dispatchBudget = 450  // NEW actual (352 at 0334, 369 at 0371, 419 now — 0371's never-fall-back sentence and 0394's catalog-idiom run gate landed in the committed block via change 0395's regeneration) rounded up to a multiple of 50.
```

Leave `dispatchOld = 1156` untouched — 450 stays strictly below it, so the direction guard keeps holding.

- [ ] **Step 7: Run the budget guard again — expect PASS**

Run:
```bash
go test ./internal/repoguard/ -run 'TestDispatchBlockBudget' -count=1
```
Expected: PASS.

- [ ] **Step 8: Delete the throwaway regenerator**

Run:
```bash
rm -rf scratch-regen-0395 && git status --porcelain
```
Expected: only `AGENTS.md` and `internal/repoguard/budgets_test.go` modified; no untracked scratch directory remains.

- [ ] **Step 9: Commit**

```bash
git add AGENTS.md internal/repoguard/budgets_test.go
git commit -m "refactor: regenerate the AGENTS.md dispatch block to the catalog idiom

Re-render the managed docket:dispatch block through harness.DispatchInterior
so the committed block matches the already-migrated run-gate asset (0394)
and the 0371 never-fall-back preamble, and reconcile dispatchBudget to the
new 419-word actual, rounded to 450 (still strictly below the pre-0334
ceiling)."
```

---

### Task 2: Migrate the "Rebuild the binary" block and extend the capability-surface guard to the repo-root files

TDD order inside one task (so every commit stays green): extend the guard first and watch it redden on the real un-migrated sites, then migrate the prose to green, then mutation-test the extended guard.

**Files:**
- Modify: `internal/repoguard/capability_surface_test.go` (`capabilitySurfaceCorpus`, its doc comment, the guard's header comment, and a population-presence assert in `TestCapabilitySurface`)
- Modify: `AGENTS.md` (the hand-authored "Rebuild the binary after a merge to main" section ONLY — it sits outside the managed markers)

**Interfaces:**
- Consumes: Task 1's regenerated dispatch block (without it, the guard extension also flags `docket run gate-before` / `docket run gate-verdict` sites inside the managed block).
- Produces: a `capabilitySurfaceCorpus` that includes the repo-root `AGENTS.md` and `CLAUDE.md` rel paths — the standing regression guard the spec's acceptance criterion 3 requires.

- [ ] **Step 1: Extend the corpus to the repo-root instruction files**

In `internal/repoguard/capability_surface_test.go`, edit `capabilitySurfaceCorpus`'s switch to add a repo-root case:

```go
	for _, rel := range maintainedPop(t, root) {
		switch {
		case rel == "AGENTS.md" || rel == "CLAUDE.md":
			// Change 0395: the repo-root instruction files are always-loaded
			// agent-executed surface — both spellings are scanned even though
			// CLAUDE.md is today a symlink to AGENTS.md, so the guard keeps
			// covering both if the alias is ever replaced by a divergent
			// regular file.
			out = append(out, rel)
		case underDir(rel, "cursor-rules"):
			out = append(out, rel)
		case (underDir(rel, "skills") || underDir(rel, "agents")) && hasExt(rel, ".md"):
			out = append(out, rel)
		}
	}
```

Update the function's doc comment ("every *.md under skills/ and agents/, every file under cursor-rules/, **and the repo-root instruction files AGENTS.md / CLAUDE.md**") and the guard's file-header comment: where it says the guard enforces the invariant "on the MAINTAINED workflow surfaces (skills/, agents/, cursor-rules/)", extend the parenthetical to "(skills/, agents/, cursor-rules/, and the repo-root AGENTS.md / CLAUDE.md — change 0395)".

- [ ] **Step 2: Add the population-presence assert**

In `TestCapabilitySurface`, immediately after the existing corpus population floor (`if len(corpus) < 40 …`), add (learnings: marker-scoped-guard-needs-a-population-floor — "at least 40 files" does not prove the two files this change adds are among them):

```go
	// Change 0395 scope extension: the repo-root instruction files must be IN
	// the population, or the coverage this change added is silently gone (a
	// removed corpus case, or a broken CLAUDE.md alias, must redden here).
	inCorpus := map[string]bool{}
	for _, rel := range corpus {
		inCorpus[rel] = true
	}
	for _, want := range []string{"AGENTS.md", "CLAUDE.md"} {
		if !inCorpus[want] {
			t.Errorf("capability-surface corpus is missing the repo-root instruction file %s — the change-0395 scope extension regressed", want)
		}
	}
```

- [ ] **Step 3: Run the guard — expect FAIL on the real un-migrated sites (the failing test)**

Run:
```bash
go test ./internal/repoguard/ -run 'TestCapabilitySurface' -count=1
```
Expected: FAIL, leading with the migration remedy and listing exactly four violations — the spelling `docket development install` twice in `AGENTS.md` (the "Rebuild the binary" bullets) and the same two line numbers again under `CLAUDE.md` (the symlink reads the same bytes). No `docket run gate-*` violations may appear (Task 1 already removed them — if any do, Task 1 regressed; stop and investigate). No exemption-pin count errors may appear (the two files carry none of the exempt human-remedy spellings).

- [ ] **Step 4: Migrate the "Rebuild the binary after a merge to main" section**

In `AGENTS.md`, replace the two bullets of that section (currently reading "rebuild the `docket` binary so the installed tool matches source: `docket development install --source /Users/homer/dev/docket`." and "…which deadlocks `docket development install` itself, since the installer reads config at startup. … then run the tracked `install --source` reinstall.") with the catalog-first house form (matching the migrated cursor-mirror's "resolve … from the capability catalog and run it" idiom):

```markdown
## Rebuild the binary after a merge to main

- Whenever a PR is successfully merged into `main`, rebuild the `docket` binary so the installed
  tool matches source: resolve the `development.install` operation from the capability catalog and
  run it with `--source /Users/homer/dev/docket`.
- When the merged change **extends the `.docket.yml` schema** (a new field or block), the still-installed
  pre-schema binary rejects *all* config reads with `invalid configuration` — which deadlocks the
  `development.install` operation itself, since the installer reads config at startup. Break the
  deadlock by rebuilding out-of-band with `go build`/`go run` (not the installed `docket`) and
  swapping the binary in, then run the tracked `development.install` reinstall with `--source`. Until
  parsing is forward-compatible, this rebuild rule is load-bearing for any schema-extending change.
```

Both operational contracts survive: the rebuild-on-merge trigger, the schema-bump deadlock caveat, the out-of-band recovery, and the tracked reinstall afterward.

- [ ] **Step 5: Run the guard — expect PASS**

Run:
```bash
go test ./internal/repoguard/ -run 'TestCapabilitySurface' -count=1
test -L CLAUDE.md && echo SYMLINK-OK
```
Expected: PASS, and `SYMLINK-OK`.

- [ ] **Step 6: Mutation probe A — a re-introduced gate/install argv reddens the guard**

Backup-copy idiom, backup OUTSIDE the repo (never `git checkout --`; the file carries this task's uncommitted edit):

```bash
cp AGENTS.md "${TMPDIR:-/tmp}/AGENTS.md.0395.bak"
printf '\n- mutation probe: run `docket run gate-before implement-next` then `docket development install --source .`\n' >> AGENTS.md
git diff --stat AGENTS.md   # prove the mutation LANDED before reading any result
go test ./internal/repoguard/ -run 'TestCapabilitySurface' -count=1
```
Expected: `git diff --stat` shows AGENTS.md changed; the test FAILS listing `docket run gate-before` and `docket development install` violations attributed to BOTH `AGENTS.md` and `CLAUDE.md`. A green run here is a defect in the guard extension — stop and fix before restoring.

Restore and re-verify:
```bash
mv -f "${TMPDIR:-/tmp}/AGENTS.md.0395.bak" AGENTS.md
go test ./internal/repoguard/ -run 'TestCapabilitySurface' -count=1
test -L CLAUDE.md && echo SYMLINK-OK
```
Expected: PASS and `SYMLINK-OK` (the `mv -f` over the symlink's target name replaces the regular file `AGENTS.md`; confirm the alias survived).

- [ ] **Step 7: Mutation probe B — removing the corpus case reddens the population-presence assert**

This proves the scope extension itself is load-bearing even while the migrated files are green:

```bash
cp internal/repoguard/capability_surface_test.go "${TMPDIR:-/tmp}/cst.0395.bak"
```
Then comment out the `case rel == "AGENTS.md" || rel == "CLAUDE.md":` case (and its `out = append(out, rel)` line) in `capabilitySurfaceCorpus`, and:
```bash
git diff --stat internal/repoguard/capability_surface_test.go   # prove it landed
go test ./internal/repoguard/ -run 'TestCapabilitySurface' -count=1
```
Expected: FAIL with "capability-surface corpus is missing the repo-root instruction file AGENTS.md" (and CLAUDE.md). Restore:
```bash
mv -f "${TMPDIR:-/tmp}/cst.0395.bak" internal/repoguard/capability_surface_test.go
go test ./internal/repoguard/ -run 'TestCapabilitySurface' -count=1
```
Expected: PASS. (The `mv -f` must actually replace the file — if it errors on a mistyped backup path, the mutation is still in the tree; verify with `git diff --stat`.)

- [ ] **Step 8: Commit**

```bash
git add AGENTS.md internal/repoguard/capability_surface_test.go
git commit -m "refactor: migrate the rebuild block to development.install and guard the repo-root files

Rewrite the hand-authored 'Rebuild the binary after a merge to main'
bullets to the catalog-resolved development.install idiom, and extend
TestCapabilitySurface's corpus to AGENTS.md/CLAUDE.md with a
population-presence assert; mutation-tested in both directions."
```

---

### Task 3: Derived-asset consistency and affected-package verification

The scope extension touches no authored asset tree (`internal/assets/embedded/tree/` holds only `agents/`, `cursor-rules/`, `skills/` — AGENTS.md is not embedded), so regeneration must be a no-op; this task proves that instead of assuming it (spec acceptance criterion 5).

**Files:**
- None expected to change. If `go generate` produces a diff, STOP and investigate before committing anything — it means an authored tree changed unexpectedly.

**Interfaces:**
- Consumes: Tasks 1–2 committed.
- Produces: evidence for the results file that derived assets are consistent and the affected packages are green.

- [ ] **Step 1: Regenerate embedded assets — expect no diff**

Run:
```bash
go generate ./internal/assets && git status --porcelain
```
Expected: generation succeeds and `git status --porcelain` prints nothing (clean tree). A diff here is a blocking finding, not something to commit silently.

- [ ] **Step 2: Run the affected packages**

Run:
```bash
go build ./... && go test ./internal/repoguard/ ./internal/harness/... ./internal/reposeed/ ./internal/install/ -count=1
```
Expected: all PASS. (The FULL suite — `go run ./cmd/docket development test` — is the build gate's job and runs after the last task; do not skip it there.)

- [ ] **Step 3: Final state check**

Run:
```bash
test -L CLAUDE.md && echo SYMLINK-OK
git status --porcelain
git log --oneline 9793ec2208bce5afc42973aca88af9383ae4b7b4..HEAD
```
Expected: `SYMLINK-OK`, a clean tree, and exactly the plan commit plus Task 1's and Task 2's commits on the branch. Nothing to commit in this task.

---

## Self-Review Notes

- Spec coverage: AC1 (no hard-coded argv; cursor-mirror idiom) — Tasks 1+2; AC2 (contracts preserved) — Task 1 Step 3's contract checklist and Task 2 Step 4's fixed replacement text; AC3 (guard covers both files, reddens on reintroduction, green when migrated) — Task 2 Steps 1–6; AC4 (honest exemption pins, zero laundering) — Task 2 Step 3 asserts no pin-count drift, and no new exemptions are added at all; AC5 (derived assets regenerated/consistent) — Task 3.
- The spec's open questions are already resolved by the reconcile pass: the corpus draws from `maintainedPop`/`MaintainedFiles` (whole-tree walk, symlink-resolving), so repo-root coverage is a corpus case, not a structural change; and the only agent-executed argv in these files beyond the two named blocks is `go run ./cmd/docket development test`, which is left-bounded by `/` and does not match the guard shape — nothing extra to migrate or exempt.
- Deliberately NOT added: a standing byte-parity guard tying the committed AGENTS.md interior to `DispatchInterior` output. That would make every future emitter change also touch AGENTS.md in the same commit — a policy decision beyond this change's scope (the capability guard already prevents the argv regression the spec targets).
