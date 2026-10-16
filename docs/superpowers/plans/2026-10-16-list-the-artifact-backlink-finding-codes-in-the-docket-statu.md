<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0539 — Describe repository-check drift findings by family in the docket-status skill](../../changes/archive/2026-10-16-0539-list-the-artifact-backlink-finding-codes-in-the-docket-statu.md)**
<!-- docket:backlink:end -->

# Describe repository-check drift findings by family in the docket-status skill — Implementation Plan

> **For agentic workers:** this plan is executed by `docket-build` (one `docket-build-task` worker per task, one whole-suite gate at the end). Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the docket-status skill's incomplete, partly wrong derived-view drift sentence with one that names the finding family and defers to each finding's own remedy, and make the glossary's `repository check` example cover backlinks too.

**Architecture:** Two prose edits (one shipped skill body, one reference doc), each pinned by a row in the existing `alignmentContracts` table (`internal/repoguard/prose_contracts_test.go`) that asserts the retired wording is gone and the new claim is present. The skill edit grows the file, so its exact-count word ceiling in `internal/repoguard/budgets_test.go` is re-baselined in the same diff, and the embedded asset tree is regenerated.

**Tech Stack:** Markdown; Go tests (`internal/repoguard`, `internal/assets`); `go generate`.

**Spec:** none — trivial change. The change record's `## What changes` is the authority: `docs/changes/active/0539-list-the-artifact-backlink-finding-codes-in-the-docket-statu.md` on the `docket` branch.

## Global Constraints

- Name the family, never the codes: no finding code list is added anywhere in the skills (the `schema` operation owns codes and shapes; each finding owns its remedy).
- Do not change any finding, its detection, its remedy text, or repair behavior. No Go code outside `internal/repoguard/*_test.go` and the generated `internal/assets/embedded/` tree changes.
- Leave the bash-upgrade guide's findings table alone (deliberate Bash-repository subset).
- Docs describe only current behavior: no change or PR numbers in the skill or glossary prose (they belong only in test comments).
- Spell it "backlink" (one word, no hyphen) — the spelling the glossary heading and `skills/docket-status/SKILL.md` line 46 already use.
- The skill body ships into other repositories: the new sentence must be true in any repository, not only this one.
- Commit subjects follow the repo style: `docs(<area>): <summary> (change 0539)`.
- After any edit under `skills/`, run `go generate ./internal/assets/` and commit the regenerated `internal/assets/embedded/` tree in the same commit.

## Review Focus

- A future drift code (say a new `-malformed` or `-shared` form) is added: the new sentence must still be true with no edit — it states a family and a remedy split, not a list. The Task 1 guard asserts the code-list wording stays gone.
- A reader runs `repository.repair` on a `-malformed` or `artifact-backlink-shared` finding expecting it to clear: the skill and glossary must no longer say repair fixes everything. Task 1 asserts "repaired through `repository.repair`" is absent; Task 2 changes the repair comment to say it re-renders only the stale ones.
- The *Sweep posture* line (line 116: a stale `## Artifacts` block surfaces later as an `artifact-links-stale` finding that `repository.repair` re-renders) stays accurate — `artifact-links-stale` is repairable in `internal/reposetup/derived.go`. Leave it unchanged; Task 1 asserts it is still present so the edit cannot drop it by accident.
- A rewrap of the Findings paragraph must not redden the guard: `TestAlignmentContracts` already whitespace-collapses both the file and the phrases, so no extra work — but do not hand-roll a line-anchored grep instead.
- The word-budget ceiling is an exact count: re-baseline it to the measured `wc -w` value, never a rounded-up number with slack.

---

### Task 1: docket-status *Findings* sentence names the drift family and defers to each finding's remedy

**Files:**
- Modify: `skills/docket-status/SKILL.md:124` (the *Findings* paragraph — one long line)
- Modify: `internal/repoguard/prose_contracts_test.go` (append a row to `alignmentContracts`, just before its closing `}` at ~line 1224)
- Modify: `internal/repoguard/budgets_test.go:215` (the `docket-status/SKILL.md` row of `skillBudgets`)
- Regenerate: `internal/assets/embedded/tree/skills/docket-status/SKILL.md` (and the manifest) via `go generate ./internal/assets/`

**Interfaces:**
- Consumes: the existing `alignmentContracts` row shape `{sentinel, file, present []string, absent []string}` and the `skillBudgets` row shape `{rel, maxLines, maxWords}`.
- Produces: sentinel name `change_0539_drift_family`, which Task 2 reuses for its glossary row.

- [ ] **Step 1: Write the failing guard row**

Append to `alignmentContracts` in `internal/repoguard/prose_contracts_test.go`:

```go
	// change 0539 — the docket-status Findings paragraph names the derived-view
	// drift family and defers to each finding's own remedy; it no longer lists
	// codes (the list drifted) or claims repair clears every drift finding
	// (-malformed and artifact-backlink-shared need a hand edit first). The
	// Sweep posture's artifact-links-stale line stays: that code is repairable.
	{sentinel: "change_0539_drift_family", file: "skills/docket-status/SKILL.md",
		present: []string{
			"is reported by the `repository.check` operation. Each finding carries its own remedy",
			"Relay the finding's remedy rather than telling the human to just run repair",
			"surfaces later as an `artifact-links-stale` finding that the authorized `repository.repair` operation re-renders",
		},
		absent: []string{
			"and their `-malformed` forms",
			"and repaired through `repository.repair`",
		}},
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/repoguard -run TestAlignmentContracts -count=1`
Expected: FAIL, with `[change_0539_drift_family]` messages for the two missing `present` phrases and the two forbidden `absent` phrases (the third `present` phrase already holds).

- [ ] **Step 3: Replace the drift sentence in `skills/docket-status/SKILL.md`**

In the *Findings* paragraph (line 124), replace exactly this sentence:

> Derived-view drift (`board-stale`, `adr-index-stale`, `artifact-links-stale`, and their `-malformed` forms) is reported by the `repository.check` operation and repaired through `repository.repair`.

with this text (keep it on the same single line; do not touch the rest of the paragraph or the *Sweep posture* line):

> Derived-view drift — the board, the ADR index, a change's `## Artifacts` block, or the backlink at the top of a spec, plan, or results file being out of date or broken — is reported by the `repository.check` operation. Each finding carries its own remedy: most clear with `repository.repair`; a broken marker, or a file linked from more than one change, needs a hand edit first. Relay the finding's remedy rather than telling the human to just run repair.

- [ ] **Step 4: Run the guard to verify it passes**

Run: `go test ./internal/repoguard -run TestAlignmentContracts -count=1`
Expected: PASS.

- [ ] **Step 5: Mutation-check the guard**

Restore the old sentence in a scratch copy only: `cp skills/docket-status/SKILL.md /tmp/0539-skill.bak`, put the old sentence back in the working file, run `go test ./internal/repoguard -run TestAlignmentContracts -count=1` and confirm FAIL naming `change_0539_drift_family`; then `cp /tmp/0539-skill.bak skills/docket-status/SKILL.md` (never `git checkout --`, which would discard the uncommitted edit) and re-run to confirm PASS.

- [ ] **Step 6: Re-baseline the word budget at the exact new count**

Run: `wc -lw skills/docket-status/SKILL.md` (was `126 2923`; lines should stay 126, words grow by roughly 50).
Run: `go test ./internal/repoguard -run TestSkillSizeBudgets -count=1` — Expected: FAIL, `docket-status/SKILL.md is <N> words, over its 2923-word budget`.
Edit the `docket-status/SKILL.md` row in `skillBudgets` to `{"docket-status/SKILL.md", 126, <N>}` with `<N>` the measured count, and prepend to that row's trailing comment: `0539: derived-view drift is named by family and each finding's own remedy is relayed instead of a code list (126/2923 -> 126/<N>); `.
Re-run: `go test ./internal/repoguard -run TestSkillSizeBudgets -count=1` — Expected: PASS.

- [ ] **Step 7: Regenerate embedded assets and check the drift guard**

Run: `go generate ./internal/assets/`
Run: `go test ./internal/assets -count=1`
Expected: PASS (`TestEmbeddedMatchesAuthored` confirms the embedded copy matches `skills/`). `git status` shows `internal/assets/embedded/tree/skills/docket-status/SKILL.md` (and the manifest if it hashes content) modified.

- [ ] **Step 8: Commit**

```bash
git add skills/docket-status/SKILL.md internal/repoguard/prose_contracts_test.go internal/repoguard/budgets_test.go internal/assets/embedded
git commit -m "docs(docket-status): describe derived-view drift by family and relay each finding's remedy (change 0539)"
```

---

### Task 2: glossary `repository check` / `repair` example covers backlinks and the remedy split

**Files:**
- Modify: `docs/reference/glossary.md:300-301` (the `sh` example under *Derived view / generated block / backlink*)
- Modify: `internal/repoguard/prose_contracts_test.go` (append one more row after Task 1's)

**Interfaces:**
- Consumes: sentinel `change_0539_drift_family` from Task 1.
- Produces: nothing later tasks use.

- [ ] **Step 1: Write the failing guard row**

Append after Task 1's row:

```go
	{sentinel: "change_0539_drift_family", file: "docs/reference/glossary.md",
		present: []string{
			"# reports links blocks and backlinks that are stale or broken",
			"# re-renders the stale ones in one metadata commit",
		},
		absent: []string{"stale or absolute"}},
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/repoguard -run TestAlignmentContracts -count=1`
Expected: FAIL with `[change_0539_drift_family]` messages for `docs/reference/glossary.md` (two missing, one forbidden).

- [ ] **Step 3: Edit the example block**

Replace the two lines:

```sh
docket repository check    # reports records whose links are stale or absolute
docket repository repair   # re-renders them in one metadata commit
```

with (keep the comment column aligned):

```sh
docket repository check    # reports links blocks and backlinks that are stale or broken
docket repository repair   # re-renders the stale ones in one metadata commit
```

The repair comment changes because a broken (malformed or shared) backlink needs a hand edit before repair can re-render it; "re-renders them" would now claim otherwise.

- [ ] **Step 4: Run the guard to verify it passes**

Run: `go test ./internal/repoguard -run TestAlignmentContracts -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add docs/reference/glossary.md internal/repoguard/prose_contracts_test.go
git commit -m "docs(glossary): repository check covers backlinks; repair re-renders only stale views (change 0539)"
```

---

### Final verification (whole suite)

- [ ] Run: `go run ./cmd/docket development test`
  Expected: exit 0. (`docs/` is not embedded, so Task 2 needs no regeneration; `go test ./internal/assets -count=1` was already green after Task 1.)
- [ ] Confirm the diff touches only: `skills/docket-status/SKILL.md`, `docs/reference/glossary.md`, `internal/repoguard/prose_contracts_test.go`, `internal/repoguard/budgets_test.go`, and `internal/assets/embedded/**`.
