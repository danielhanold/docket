<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0346 — Finalize's post-merge binary rebuild runs against an unpulled source tree](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0346-finalize-rebuilds-binary-from-unpulled-source-tree.md)**
<!-- docket:backlink:end -->
# Verified Post-Merge Binary Rebuild Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. (In a docket run, `docket-build` is the executor and owns task routing.)

**Goal:** Make finalize's repository-required post-merge binary rebuild run only after a successful end-of-run integration sync, against a source tree proven to contain every verified merge, with the installed binary's exact commit identity verified — and any failure reported separately as "binary rebuild incomplete" without reversing merges.

**Architecture:** This is a workflow + repository-policy correction with Go regression guards — no new production CLI surface. The maintained finalize skill (`skills/docket-finalize-change/SKILL.md`) gains a generic step 12 (post-merge rebuild after step 11's sync); the concrete Docket rebuild rule in `AGENTS.md` (CLAUDE.md is its symlink) is updated so its condition and proof travel together. Mutation-tested Go guards land in `internal/repoguard`; embedded skill assets are regenerated through the normal generation path.

**Tech Stack:** Go (stdlib `testing`, `regexp`, `strings`), markdown skill prose, `go generate ./internal/assets` (cmd/genassets), the source-entered Go suite runner (`go run ./cmd/docket development test`).

**Spec:** `docs/superpowers/specs/2026-09-07-finalize-rebuilds-binary-from-unpulled-source-tree-design.md` (synchronized copy on the `docket` branch; the plan argues from it — read it alongside this plan).

## Global Constraints

- Work in the feature worktree `/Users/homer/dev/docket/.worktrees/finalize-rebuilds-binary-from-unpulled-source-tree` on branch `fix/finalize-rebuilds-binary-from-unpulled-source-tree`. All paths below are relative to that worktree root.
- No new production CLI operation, flag, configuration key, installer mode, or alternate source checkout. No standalone Bash tests (guards are Go, in `internal/repoguard`). No edits to frozen records, archived changes, specs, or Accepted ADRs.
- The finalize skill stays **generic**: it must not hard-code `/Users/homer/dev/docket` or impose binary installation on consuming repositories. The concrete Docket path lives only in `AGENTS.md`.
- Facts verified against source at HEAD `0d1a7e1b`: the sync operation is `repository.sync-integration` with closed dispositions `advanced` / `already-current` (success) and structured fields `disposition`, `target_oid`, `after_oid`, `primary_path`, `integration_branch` (`internal/app/repository_sync.go`, consts `SyncDispAdvanced`/`SyncDispAlreadyCurrent`). The install operation is `development.install`; it stamps identity via ldflags from `git describe --tags --always --dirty` + `git rev-parse HEAD`, appending `-dirty` to the commit on a dirty tree and defaulting to `unknown` when unstamped (`internal/install/devmode.go` `buildIdentity`). The installed binary's `version` operation reports `{"commit": "<full 40-hex>", ...}` in `--json` mode.
- Every mutation probe and manual re-verification runs Go tests with `-count=1` (learnings: `cached-runner-serves-a-mutated-tree`). Restore mutated files from an explicit `cp` backup, never `git checkout --` against uncommitted work (learnings: `mutation-restore-needs-a-backup-copy`).
- Guards bind phrases to their claims with bounded gaps over whitespace-collapsed text, slice sections with named, asserted terminators, and carry population floors (learnings: `prose-guard-binds-phrase-to-claim`, `phrase-grep-over-wrapped-prose`, `section-slice-needs-a-named-terminator`, `marker-scoped-guard-needs-a-population-floor`, `assert-detects-removal-not-replacement`). Go RE2 cannot backtrack catastrophically, but keep every gap bounded anyway.
- The skill byte-budget ratchet (`internal/repoguard/budgets_test.go`) will redden on the SKILL.md growth; re-baseline its row at the exact measured counts with a `0346:` annotation, following the file's established 0388/0399 pattern.
- Final gate: the full configured suite via the source-entered Go runner, `go run ./cmd/docket development test`, run from the worktree root. A `SERIAL CONFIRMED OVER BUDGET:` line is an authoritative breach to act on; `BUDGET WATCH:`/`PARALLEL-SENSITIVE:` lines are screening findings.

---

### Task 1: Go regression guards (written first, red)

**Files:**
- Create: `internal/repoguard/finalize_rebuild_test.go`
- Test: same file (`go test ./internal/repoguard/ -run TestFinalizeRebuildContract`)

**Interfaces:**
- Consumes: `guardRoot(t)` — the existing `internal/repoguard` test helper returning the repo root (see `prose_contracts_test.go` `TestProseContracts` for usage).
- Produces: `TestFinalizeRebuildContract` (skill-side asserts) and `TestFinalizeRebuildAgentsRule` (AGENTS.md asserts), which Tasks 2–3 turn green and Task 5 mutation-probes. Anchor strings in this file are the exact headings/phrases Tasks 2–3 must author — they are load-bearing; if you reword prose in a later task, repoint the guard in the same commit.

This is TDD for prose: the guards are written against the contract Tasks 2–3 will author, run red now (the sections do not exist yet), and go green as the prose lands.

- [ ] **Step 1: Write the failing guard file**

Create `internal/repoguard/finalize_rebuild_test.go`:

```go
package repoguard

// Change 0346 — finalize's repository-required post-merge binary rebuild.
//
// These guards pin the maintained instruction contract (skill prose +
// AGENTS.md policy), not universal obedience by every agent harness:
//   1. end-of-run ordering — the rebuild step follows step 11's integration
//      sync, structurally (heading order) and in prose;
//   2. the success-disposition gate — only sync disposition `advanced` or
//      `already-current` authorizes a rebuild, and the failure vocabulary
//      does not;
//   3. the source ancestry proof — every retained merge commit is proven an
//      ancestor via `git merge-base --is-ancestor`, with a negative answer
//      and a failed probe distinguished;
//   4. the post-install exact identity check — the installed binary's full,
//      clean commit identity must equal the verified source commit S (no
//      prefix/timestamp/label proof);
//   5. the separate incomplete-report — failures keep merged changes done
//      and are reported as "binary rebuild incomplete", never by reversing
//      a merge.
//
// Guard shape: each claim is bound to its subject with a single bounded gap
// over whitespace-collapsed section slices (learnings:
// prose-guard-binds-phrase-to-claim, phrase-grep-over-wrapped-prose). Every
// slice has a named start anchor and a named terminator, both asserted to
// exist (learnings: section-slice-needs-a-named-terminator,
// marker-scoped-guard-needs-a-population-floor). Ordering is asserted by
// index comparison, not adjacency. Embedded copies are NOT read here —
// internal/assets' drift guard owns authored-vs-embedded equality.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// collapseWS joins all whitespace runs (including newlines) into single
// spaces so a claim split across hard-wrapped lines stays matchable.
func collapseWS(s string) string { return strings.Join(strings.Fields(s), " ") }

// readGuarded reads one maintained file by path, fail-closed: a moved or
// renamed file is a test failure, never a silent skip.
func readGuarded(t *testing.T, root, rel string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("guarded file unreadable (moved/renamed files must repoint this guard): %v", err)
	}
	return string(raw)
}

// sliceSection cuts [start, terminator) out of content, asserting both
// anchors exist and appear in order — an absent anchor fails loudly instead
// of silently matching nothing.
func sliceSection(t *testing.T, rel, content, start, terminator string) string {
	t.Helper()
	i := strings.Index(content, start)
	if i < 0 {
		t.Fatalf("%s: section start anchor missing: %q", rel, start)
	}
	rest := content[i:]
	j := strings.Index(rest, terminator)
	if j < 0 {
		t.Fatalf("%s: section terminator missing after start: %q", rel, terminator)
	}
	return rest[:j]
}

// binding is one claim bound to its subject: a named regexp that must match
// the collapsed section slice.
type binding struct {
	name    string
	pattern string
}

func assertBindings(t *testing.T, rel string, collapsed string, bindings []binding) {
	t.Helper()
	// Population floor: an empty table would pass vacuously.
	if len(bindings) < 3 {
		t.Fatalf("%s: binding table collapsed to %d rows", rel, len(bindings))
	}
	for _, b := range bindings {
		re := regexp.MustCompile(b.pattern)
		if !re.MatchString(collapsed) {
			t.Errorf("%s: contract binding %q not found (pattern %q) — the claim was removed or severed from its subject", rel, b.name, b.pattern)
		}
	}
}

const (
	finalizeSkillRel   = "skills/docket-finalize-change/SKILL.md"
	agentsRel          = "AGENTS.md"
	syncStepAnchor     = "### 11. Integration sync"
	rebuildStepAnchor  = "### 12. Repository-required post-merge rebuild"
	rebuildTerminator  = "## Identity repair checkpoint"
	agentsRebuildStart = "## Rebuild the binary after a merge to main"
	agentsTerminator   = "<!-- docket:dispatch:start"
)

// TestFinalizeRebuildContract pins the maintained finalize skill's step-12
// rebuild contract (change 0346).
func TestFinalizeRebuildContract(t *testing.T) {
	root := guardRoot(t)
	content := readGuarded(t, root, finalizeSkillRel)

	// (1) End-of-run ordering, structural: step 11's sync heading precedes
	// the rebuild heading. Both anchors are asserted to exist first, so a
	// renamed heading fails loudly rather than passing an index race.
	si := strings.Index(content, syncStepAnchor)
	ri := strings.Index(content, rebuildStepAnchor)
	if si < 0 {
		t.Fatalf("%s: integration-sync step heading missing: %q", finalizeSkillRel, syncStepAnchor)
	}
	if ri < 0 {
		t.Fatalf("%s: rebuild step heading missing: %q", finalizeSkillRel, rebuildStepAnchor)
	}
	if si > ri {
		t.Errorf("%s: rebuild step precedes the integration sync step — the rebuild must be the post-sync suffix", finalizeSkillRel)
	}

	sec := collapseWS(sliceSection(t, finalizeSkillRel, content, rebuildStepAnchor, rebuildTerminator))

	assertBindings(t, finalizeSkillRel, sec, []binding{
		// (1) ordering, in prose: bound to step 11 and to the never-per-change rule.
		{"rebuild runs after step 11's sync", "runs \\*\\*after\\*\\* step 11's integration sync"},
		{"never per-change inside merge/closeout", "never per-change inside steps 8"},
		// (2) success-disposition gate, positive and negative.
		{"gate on advanced/already-current", "authorized only when[^.]{0,160}`advanced` or `already-current`"},
		{"failure vocabulary does not authorize", "`skipped`, `refused`, or `failed` sync[^.]{0,160}leaves the rebuild not performed"},
		{"clean exit is not authorization", "clean process exit[^.]{0,80}not authorization"},
		// (3) ancestry proof, bound to its universal quantifier and its two-outcome rule.
		{"every retained merge commit proven", "every retained merge commit[^.]{0,200}merge-base --is-ancestor"},
		{"negative answer vs failed probe distinct", "negative answer and a failed probe[^.]{0,120}neither permits"},
		// merge identity comes from the merge verification, never a feature head.
		{"merge ids from step 8 verification", "merge-commit ids from step 8"},
		{"feature head is never a substitute", "original head is never a substitute"},
		// (4) post-install exact identity, equality not prefix.
		{"identity equals S exactly", "full, clean commit identity to equal `S` exactly"},
		{"prefix/timestamp/label prove nothing", "short-prefix comparison[^.]{0,120}proves nothing"},
		{"unknown/dirty/mismatch is failure", "unknown, dirty, mismatching, or unreadable identity is verification failure"},
		// (5) separate incomplete-report, bound to done-retention and no-reversal.
		{"failures keep merged changes done", "keeps every verified merged change `done`[^.]{0,160}binary rebuild incomplete"},
		{"never reverses a merge", "never reverses a merge"},
		{"no destructive source recovery", "Never stash, switch branches, reset"},
		// generic skill: no hard-coded Docket source path.
	})

	// The distributed skill body must not hard-code this repo's source path
	// (learnings: distributed-body-has-no-local-repo).
	if strings.Contains(sec, "/Users/homer/dev/docket") {
		t.Errorf("%s: the shipped skill hard-codes a repo-local source path", finalizeSkillRel)
	}
}

// TestFinalizeRebuildAgentsRule pins the concrete Docket policy in AGENTS.md:
// sync-gated, ancestry-proven, identity-verified rebuild, reported as
// incomplete on failure. CLAUDE.md is a symlink of AGENTS.md; reading
// AGENTS.md covers both.
func TestFinalizeRebuildAgentsRule(t *testing.T) {
	root := guardRoot(t)
	content := readGuarded(t, root, agentsRel)
	raw := sliceSection(t, agentsRel, content, agentsRebuildStart, agentsTerminator)
	sec := collapseWS(raw)

	// Ordering inside the rule: the sync operation is named before the
	// install operation.
	si := strings.Index(sec, "repository.sync-integration")
	ii := strings.Index(sec, "development.install")
	if si < 0 || ii < 0 {
		t.Fatalf("%s: rebuild rule must name both repository.sync-integration (%d) and development.install (%d)", agentsRel, si, ii)
	}
	if si > ii {
		t.Errorf("%s: the rebuild rule names install before sync — sync must gate the install", agentsRel)
	}

	assertBindings(t, agentsRel, sec, []binding{
		{"gate on advanced/already-current", "only on disposition `advanced` or `already-current`"},
		{"ancestry proof named", "merge-base --is-ancestor"},
		{"installed identity equality", "same full, clean commit id"},
		{"incomplete report named", "binary rebuild incomplete"},
		{"no destructive source recovery", "never stash, reset, or switch branches"},
		// The pre-existing schema-tolerance bullet survives the rewrite.
		{"schema-extension bullet retained", "extends the `\\.docket\\.yml` schema"},
	})
}
```

- [ ] **Step 2: Run the guards to verify they fail red**

Run: `cd /Users/homer/dev/docket/.worktrees/finalize-rebuilds-binary-from-unpulled-source-tree && go test -count=1 ./internal/repoguard/ -run 'TestFinalizeRebuild' -v`

Expected: FAIL — `TestFinalizeRebuildContract` fails at "rebuild step heading missing" (step 12 does not exist yet) and `TestFinalizeRebuildAgentsRule` fails on the new bindings ("only on disposition …" etc.). If either passes, the guard is vacuous — stop and fix it before proceeding.

- [ ] **Step 3: Commit the red guards**

```bash
git add internal/repoguard/finalize_rebuild_test.go
git commit -m "test(0346): red regression guards for the verified post-merge rebuild contract"
```

(Committing a deliberately red test is safe here: the suite gate runs at the end of the branch, and Tasks 2–3 turn these green before any gate.)

---

### Task 2: Finalize skill step 12 + budget re-baseline

**Files:**
- Modify: `skills/docket-finalize-change/SKILL.md` (insert a new `### 12.` section between `### 11. Integration sync — best-effort, end-of-run` and `## Identity repair checkpoint`)
- Modify: `internal/repoguard/budgets_test.go` (re-baseline the `docket-finalize-change/SKILL.md` row)

**Interfaces:**
- Consumes: Task 1's anchor strings — the heading must be exactly `### 12. Repository-required post-merge rebuild — after sync, verified` (its prefix is the guard's `rebuildStepAnchor`), and the bound phrases below must appear verbatim.
- Produces: the generic skill contract Task 4 embeds and Task 5 mutation-probes.

- [ ] **Step 1: Insert the step-12 section**

Insert the following, verbatim, immediately after the end of the `### 11. Integration sync — best-effort, end-of-run` section (i.e. just before the `## Identity repair checkpoint` heading), separated by blank lines:

```markdown
### 12. Repository-required post-merge rebuild — after sync, verified

Some repositories' agent-instruction files require rebuilding or reinstalling a tool after a merge
to the integration branch. When such a requirement applies, it runs **after** step 11's integration
sync — never before it, and never per-change inside steps 8–9 — and a rebuild is authorized only
when that sync's document reported disposition `advanced` or `already-current` with valid
`primary_path` / `integration_branch` / target facts. A `skipped`, `refused`, or `failed` sync,
missing facts, malformed output, or an unobservable outcome leaves the rebuild not performed — a
clean process exit on a deliberate skip is not authorization.

Retain the actual merge-commit ids from step 8's authoritative merge verification (including
already-merged recovery); a feature branch's original head is never a substitute — rebase and
squash change commit identity. A stacked-only merge into a live parent's branch does not trigger an
integration-merge rebuild rule.

Before installing, verify the exact source directory the repository policy names for the install:

- Its canonical identity must be the synced primary checkout, on the expected integration branch,
  with no unfinished Git operation and a clean index and worktree including non-ignored untracked
  files. Read and retain its full HEAD commit id `S`; require `S` to equal the sync document's
  target/after commit ids. Unknown or changed state does not authorize installation.
- Prove every retained merge commit is contained in that source: run
  `git merge-base --is-ancestor <merge-commit> <S>` in the source repository, treating a
  negative answer and a failed probe as two distinct outcomes — neither permits the rebuild. This
  proves the source install reads, not merely a remote-tracking ref or a feature worktree.

Then run the repository's named install operation (argv resolved from the capability catalog) with
its stated source argument; a single successful rebuild may satisfy every verified integration
merge in the batch, and a failed install is never silently retried through another build method.
After a successful install, re-check that the source branch, cleanliness, and HEAD still equal the
observations for `S`, then read the installed executable's own identity — the version operation of
the binary at the destination the install actually updated. Require its full, clean commit
identity to equal `S` exactly. A timestamp, a version label, a short-prefix comparison, or the
identity of an older running process proves nothing; unknown, dirty, mismatching, or unreadable
identity is verification failure. The success report names the verified installed commit; observed
source movement is reported honestly as an unverified rebuild — an installation may already have
changed, so never claim it was untouched and never attempt a rollback.

**Failure posture — report separately, reverse nothing.** Any unmet condition keeps every verified
merged change `done` and is reported separately as **binary rebuild incomplete**, naming the failed
condition and the source path. A failed rebuild never reverses a merge, revives a terminal change,
writes a `## Finalize blocked` marker, rewrites frozen records or closeout notes, or changes an
earlier halt verdict. Never stash, switch branches, reset, discard files, or build from another
checkout to force the rebuild. State the specific obstacle and the recovery sequence: resolve the
reported source state, rerun integration sync, then repeat the proof, install, and identity check —
the bare install command alone is never a sufficient remedy for stale source. A repository whose
instructions carry no such requirement skips this step entirely.
```

- [ ] **Step 2: Run the skill-side guard**

Run: `go test -count=1 ./internal/repoguard/ -run TestFinalizeRebuildContract -v`

Expected: PASS. (`TestFinalizeRebuildAgentsRule` still FAILs — that is Task 3.) If a binding fails, fix the prose or — only for a deliberate rewording — repoint the guard in the same commit.

- [ ] **Step 3: Re-baseline the budget row**

Run: `wc -l -w skills/docket-finalize-change/SKILL.md` and note the exact counts (budget semantics: lines = newline count, words = `wc -w` tokens).

In `internal/repoguard/budgets_test.go`, update the row

```go
	{"docket-finalize-change/SKILL.md", 190, 4344}, // 0388: +sync-integration prose (see note above)
```

to the exact measured values, e.g. (numbers illustrative — pin what `wc` reports):

```go
	{"docket-finalize-change/SKILL.md", 233, 4810}, // 0346: +verified post-merge rebuild contract prose; 0388: +sync-integration prose (see note above)
```

Also append to the file's header comment block, after the 0388 paragraph:

```go
// Change 0346 re-baselined docket-finalize-change/SKILL.md upward once to hold
// the verified post-merge rebuild contract (step 12): the sync-disposition
// gate, the source ancestry proof, the post-install identity check, and the
// separate binary-rebuild-incomplete report. Authored contract documentation,
// not slack — the ceiling is pinned at the exact new counts and the ratchet
// stays in force.
```

Pin exactly — do not leave slack headroom; the ratchet's value is that any further regrowth reddens.

- [ ] **Step 4: Run the budget and repoguard package tests**

Run: `go test -count=1 ./internal/repoguard/ -run 'TestSkillBudgets|TestFinalizeRebuildContract' -v`

Expected: PASS for both. If `TestSkillBudgets` still reddens, the pinned numbers do not match `wc` — re-measure and correct.

- [ ] **Step 5: Commit**

```bash
git add skills/docket-finalize-change/SKILL.md internal/repoguard/budgets_test.go
git commit -m "feat(0346): finalize step 12 — verified post-merge rebuild after integration sync"
```

---

### Task 3: AGENTS.md — condition and proof updated together

**Files:**
- Modify: `AGENTS.md` (the `## Rebuild the binary after a merge to main` section only; `CLAUDE.md` is a symlink to it — touch nothing else, and stay clear of the `<!-- docket:dispatch:start -->` managed block below it)

**Interfaces:**
- Consumes: Task 1's `TestFinalizeRebuildAgentsRule` bound phrases (verbatim below).
- Produces: the concrete Docket policy the always-loaded instructions carry.

- [ ] **Step 1: Rewrite the section's first bullet; keep the second verbatim**

Replace the first bullet of `## Rebuild the binary after a merge to main` (the two lines beginning "Whenever a PR is successfully merged…") with:

```markdown
- Whenever a PR is successfully merged into `main`, rebuild the `docket` binary so the installed
  tool matches source — from a source tree **proven** to contain the merge, never blindly. First
  run the `repository.sync-integration` operation (argv resolved from the capability catalog) with
  `--repo-dir /Users/homer/dev/docket --json`; proceed only on disposition `advanced` or
  `already-current` — a `skipped`, `refused`, or `failed` sync leaves the rebuild incomplete.
  Confirm the checkout is clean on `main` with its full HEAD equal to the sync target, and prove
  each merge landed in it: `git merge-base --is-ancestor <merge-commit> HEAD` (a negative answer
  and a failed probe are different outcomes; neither permits the install). Then resolve the
  `development.install` operation from the capability catalog, run it with
  `--source /Users/homer/dev/docket`, and confirm the installed binary's `version` operation
  reports the same full, clean commit id as that HEAD — a fresh timestamp, a short prefix, or an
  older running process proves nothing. On any failed condition, report `binary rebuild
  incomplete` naming it, keep the merged change done, and never stash, reset, or switch branches
  to force the rebuild — fix the reported source state, re-sync, and repeat the
  proof/install/identity sequence.
```

Keep the existing second bullet (the `.docket.yml` schema-extension tolerance, "A merged change that **extends the `.docket.yml` schema** …") byte-for-byte unchanged.

- [ ] **Step 2: Run the AGENTS.md guard**

Run: `go test -count=1 ./internal/repoguard/ -run TestFinalizeRebuildAgentsRule -v`

Expected: PASS.

- [ ] **Step 3: Run the whole repoguard package**

Run: `go test -count=1 ./internal/repoguard/`

Expected: PASS — this also proves the AGENTS.md edit did not trip the always-loaded-surface absence/shape guards that scan `AGENTS.md`/`CLAUDE.md` as agent-executed markdown.

- [ ] **Step 4: Commit**

```bash
git add AGENTS.md
git commit -m "docs(0346): AGENTS.md rebuild rule — sync-gated, ancestry-proven, identity-verified"
```

---

### Task 4: Regenerate embedded skill assets

**Files:**
- Modify (generated): `internal/assets/embedded/tree/skills/docket-finalize-change/SKILL.md` and `internal/assets/embedded/` manifest artifacts, via the generator only — never hand-edit the embedded tree.

**Interfaces:**
- Consumes: Task 2's authored SKILL.md.
- Produces: an embedded tree in sync with the authored skill, so `internal/assets`' authored-vs-embedded drift guard passes.

- [ ] **Step 1: Regenerate through the normal generation path**

Run: `go generate ./internal/assets`

(This runs `go run ../../cmd/genassets -repo ../..` per `internal/assets/generate.go`.)

- [ ] **Step 2: Verify the drift guard and inspect the diff**

Run: `go test -count=1 ./internal/assets/ && git status --porcelain`

Expected: PASS, and the only unstaged changes are under `internal/assets/embedded/` (the finalize SKILL.md copy plus manifest). Any change outside that directory means the generator touched something unexpected — stop and investigate.

- [ ] **Step 3: Commit the regenerated assets**

```bash
git add internal/assets/embedded/
git commit -m "chore(0346): regenerate embedded skill assets for the finalize rebuild contract"
```

---

### Task 5: Mutation-test every guard

**Files:**
- Modify (temporarily, restored): `skills/docket-finalize-change/SKILL.md`, `AGENTS.md`
- No durable file changes; the deliverable is the verified evidence recorded in the build notes/results.

Each probe: back up with `cp`, apply the mutation, **prove the mutation landed** (grep for the mutated text) before running, run the guard with `-count=1`, require the named assert to redden, then restore from the backup and confirm the guard is green again. A mutation that leaves the guard green is a defect in the guard — fix the guard (Task 1's file), re-run the mutation, and only then continue.

- [ ] **Step 1: Prepare backups**

```bash
cd /Users/homer/dev/docket/.worktrees/finalize-rebuilds-binary-from-unpulled-source-tree
BK="${TMPDIR:-/tmp}/docket-0346-mutation.XXXXXX"; BK="$(mktemp -d "$BK")"
cp skills/docket-finalize-change/SKILL.md "$BK/SKILL.md"
cp AGENTS.md "$BK/AGENTS.md"
echo "$BK"
```

- [ ] **Step 2: M1 — strip the success-disposition gate**

In `skills/docket-finalize-change/SKILL.md` step 12, replace the exact text `` authorized only when that sync's document reported disposition `advanced` or `already-current` `` with `authorized regardless of the sync's reported disposition`. Verify the mutation landed: `grep -c "authorized regardless" skills/docket-finalize-change/SKILL.md` prints `1`. Run `go test -count=1 ./internal/repoguard/ -run TestFinalizeRebuildContract`. Expected: FAIL naming binding `"gate on advanced/already-current"`. Restore: `cp "$BK/SKILL.md" skills/docket-finalize-change/SKILL.md` and confirm the test passes again with `-count=1`.

- [ ] **Step 3: M2 — remove the ancestry proof**

Delete the entire second bullet of step 12's verification list (the one containing `git merge-base --is-ancestor`). Verify landed: `grep -c "is-ancestor" skills/docket-finalize-change/SKILL.md` prints `0`. Run the same test. Expected: FAIL naming `"every retained merge commit proven"` (and `"negative answer vs failed probe distinct"`). Restore from `$BK` and re-verify green.

- [ ] **Step 4: M3 — weaken the post-install identity check to a prefix**

Replace `` full, clean commit identity to equal `S` exactly `` with `` commit identity to start with `S`'s short prefix ``. Verify landed via grep for `short prefix`. Run the test. Expected: FAIL naming `"identity equals S exactly"`. Restore and re-verify green. (This is the exact vacuity class of learnings: `identity-match-relaxed-to-prefix-is-vacuous`.)

- [ ] **Step 5: M4 — invert the end-of-run ordering**

Cut the entire `### 12. Repository-required post-merge rebuild — after sync, verified` section and paste it immediately **above** the `### 11. Integration sync` heading. Verify landed by heading order: `grep -n "^### 1[12]\." skills/docket-finalize-change/SKILL.md` shows 12 before 11. Run the test. Expected: FAIL at "rebuild step precedes the integration sync step". Restore and re-verify green.

- [ ] **Step 6: M5 — drop the separate incomplete-report**

Replace `` keeps every verified merged change `done` and is reported separately as **binary rebuild incomplete** `` with `is retried until it succeeds`. Verify landed via grep. Run the test. Expected: FAIL naming `"failures keep merged changes done"`. Restore and re-verify green.

- [ ] **Step 7: M6 — AGENTS.md: name install before sync**

In `AGENTS.md`, within the rebuild section's first bullet, swap the operation names: change `repository.sync-integration` to `development.install` at its first occurrence and the later `development.install` to `repository.sync-integration`. Verify landed via `grep -n "development.install" AGENTS.md` (first hit now precedes the sync name). Run `go test -count=1 ./internal/repoguard/ -run TestFinalizeRebuildAgentsRule`. Expected: FAIL at "names install before sync". Restore `cp "$BK/AGENTS.md" AGENTS.md` and re-verify green.

- [ ] **Step 8: Confirm the tree is byte-identical to the committed state**

Run: `git status --porcelain` (expect empty) and `git diff --stat` (expect empty). Remove the backup dir: `rm -rf "$BK"`. Record the six probe outcomes (mutation, expected red assert, observed red assert, restored-green) for the results file.

---

### Task 6: Full suite gate

**Files:** none modified.

- [ ] **Step 1: Run the whole configured suite from source**

Run: `cd /Users/homer/dev/docket/.worktrees/finalize-rebuilds-binary-from-unpulled-source-tree && go run ./cmd/docket development test`

Expected: the suite's PASS summary. Act on any `SERIAL CONFIRMED OVER BUDGET:` line (authoritative breach — investigate before proceeding); note `BUDGET WATCH:`/`PARALLEL-SENSITIVE:` lines as screening findings in the results. Do not raise budgets to conceal growth.

- [ ] **Step 2: Verify branch state**

Run: `git log --oneline origin/main..HEAD` — expect exactly the plan's commits (plan, red guards, skill step 12 + budgets, AGENTS.md, embedded assets) and a clean `git status`.

---

## Self-Review (performed at authoring)

- **Spec coverage:** end-of-run ordering + disposition gate (Task 2 §1, guard 1–2); merge-id retention incl. already-merged recovery and the stacked-only exclusion (Task 2 step-12 ¶2); source cleanliness/identity + S-equality (Task 2 bullet 1); ancestry proof with two-outcome distinction (Task 2 bullet 2, M2); catalog-resolved install, single rebuild per batch, no alternate build method (Task 2 ¶4); post-install recheck + exact full identity, no prefix/timestamp/label/old-process proof (Task 2 ¶4, M3); separate `binary rebuild incomplete` report, done-retention, no reversal/rollback, recovery sequence, no destructive source recovery (Task 2 failure posture, M5); AGENTS.md concrete rule with condition and proof together (Task 3, M6); mutation-tested Go guards in `internal/repoguard` for the five named claims (Tasks 1, 5); embedded-asset regeneration via the normal path (Task 4); full configured suite via the source-entered Go runner (Task 6). Case review from the spec's validation list maps onto the prose: dirty/detached/other-branch/diverged source → bullet 1's clean-checkout requirement; sync target excluding a retained merge → S-equality + ancestry; failed Git probe → two-outcome rule; failed install / identity mismatch → verification-failure sentence; consuming repo without the requirement → the final skip sentence; batch → "single successful rebuild may satisfy every verified integration merge".
- **Placeholder scan:** none — every step carries its exact text, command, and expected outcome.
- **Type consistency:** the guard's anchor constants match Task 2/3's authored headings and phrases verbatim; the two test names used in Tasks 2, 3, and 5 match Task 1's definitions.
