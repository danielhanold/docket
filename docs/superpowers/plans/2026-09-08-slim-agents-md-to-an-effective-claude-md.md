<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0283 — Slim AGENTS.md to an effective, lean always-in-context file](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0283-slim-agents-md-to-an-effective-claude-md.md)**
<!-- docket:backlink:end -->
# Slim AGENTS.md Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rewrite the unmanaged authored prefix of root `AGENTS.md` (lines 1–107 at base 19faaf60) to a leaner text that preserves every operational obligation, removes obsolete pointers (automated harvest, `mint-stub.sh`, `tests/test_comment_anchor_style.sh`), and leaves the managed `docket:dispatch` block (lines 108–144) byte-for-byte untouched.

**Architecture:** One documentation edit to one file. The new prefix is fully drafted in Task 1 below — it reconciles the spec's editorial draft with the current, richer post-0346/0392 rebuild section and with the exact prose anchors that `internal/repoguard` tests pin. Verification is guard-test driven: targeted repoguard tests before and after the edit, one deliberate mutation probe, a whole-repo dependent sweep, and byte-comparison of the managed block.

**Tech Stack:** Markdown, Go test suite (`go test ./internal/repoguard/`), git.

**Spec:** `docs/superpowers/specs/2026-09-07-slim-agents-md-to-an-effective-claude-md-design.md` (on the `docket` metadata branch; synchronized copy at `.docket/docs/superpowers/specs/…` from the primary tree). The spec's "Proposed authored prefix" is an editorial baseline measured against an older base (9cce7f98); this plan's Task 1 text supersedes it where the current file carries obligations and test anchors the spec's draft predates — chiefly the rebuild section.

## Global Constraints

- **Managed block is untouchable:** everything from `<!-- docket:dispatch:start (managed by docket — do not hand-edit) -->` through `<!-- docket:dispatch:end -->` (inclusive) must be byte-identical to base `19faaf60a9a40c122ad8945088d140890dcc708d`. `TestDispatchBlockBudget` requires both markers present; `TestFinalizeRebuildAgentsRule` slices the rebuild section using `<!-- docket:dispatch:start` as its terminator, so the rebuild heading must remain the last unmanaged section, immediately before the block.
- **`CLAUDE.md` stays a symlink to `AGENTS.md`** — do not replace, retarget, or convert it.
- **Only `AGENTS.md` changes in this branch's diff** (plus this plan file, committed by the plan writer). No test edits are expected; if the whole-repo sweep (Task 1, Step 2) surfaces a maintained dependent, fix by relocation/repointing per the decision rule in that step, never by restoring deleted prose.
- **Test anchors that must survive verbatim** (whitespace-collapsed matching, so line wrapping is free — see `TestFinalizeRebuildAgentsRule` in `internal/repoguard/finalize_rebuild_test.go`): heading `## Rebuild the binary after a merge to main`; `repository.sync-integration` named **before** `development.install`; `only on disposition `advanced` or `already-current``; `merge-base --is-ancestor`; `same full, clean commit id`; `binary rebuild incomplete`; `never stash, reset, or switch branches`; `extends the `.docket.yml` schema`.
- **Capability-surface guard** (`TestCapabilitySurface`): AGENTS.md is scanned; never introduce an executable `docket <family> <token>` spelling. Name operations semantically (`repository.sync-integration`, `development.install`) as the current file does. The one permitted hard-coded bootstrap is `docket capabilities --json` — this rewrite needs none.
- **Retired-control-plane guard** (`internal/repoguard/absence_test.go`): AGENTS.md markdown is scanned fenced-only. The new text contains no fenced code blocks; keep it that way (inline code spans only).
- **Shell-portability guard** (`shellPortabilityCorpus` includes AGENTS.md): no ERE interval bound > 255 and no `\b`/`\<`/`\>` word boundaries in the file's own demonstrated patterns. The Task 1 text's only patterns are `[^[:space:]]`, `[^ ]`, and literal grep flags — all safe.
- **Word target is a direction, not a gate** (learnings: `size-target-is-direction`): the spec's 499-word draft was measured against a base whose rebuild section was far shorter. Preserving the current rebuild obligations, expect roughly 600–700 unmanaged words (from 1,192). Meaning outranks the count; if preserving an obligation defeats a cut, keep the obligation and record the smaller reduction.
- Suite runs use `-count=1` (learnings: `cached-runner-serves-a-mutated-tree`).

---

### Task 1: Rewrite the unmanaged AGENTS.md prefix and verify against the guard suite

**Files:**
- Modify: `AGENTS.md` (unmanaged prefix only — everything above the `<!-- docket:dispatch:start … -->` marker)
- Test: existing guards in `internal/repoguard/` (`finalize_rebuild_test.go`, `budgets_test.go`, `capability_surface_test.go`, `shellshape_test.go`, `absence_test.go`, `anchors_test.go`) — no test file edits expected

**Interfaces:**
- Consumes: base `AGENTS.md` at commit `19faaf60a9a40c122ad8945088d140890dcc708d`.
- Produces: the final `AGENTS.md`; nothing downstream consumes new symbols.

- [ ] **Step 1: Capture pre-edit evidence (worktree root: `.worktrees/slim-agents-md-to-an-effective-claude-md`)**

Run each and keep the outputs for the commit message and Step 9's comparison:

```bash
cd /Users/homer/dev/docket/.worktrees/slim-agents-md-to-an-effective-claude-md
git rev-parse HEAD                          # expect 19faaf60a9a40c122ad8945088d140890dcc708d
ls -la CLAUDE.md                            # expect symlink -> AGENTS.md
wc -l -w AGENTS.md                          # expect 144 lines / 1603 words
awk 'NR<108' AGENTS.md | wc -l -w           # unmanaged prefix: expect 107 / 1192
MARKER_LINES=$(grep -n "docket:dispatch" AGENTS.md)
printf '%s\n' "$MARKER_LINES"               # expect exactly two hits: 108 start, 144 end, start first
sed -n '108,144p' AGENTS.md > "${TMPDIR:-/tmp}/dispatch-block-before.txt"
shasum -a 256 "${TMPDIR:-/tmp}/dispatch-block-before.txt"
```

(A fixed name under `${TMPDIR:-/tmp}` is fine here: the file is a read-only comparison snapshot, never renamed into place.) Confirm exactly two marker lines, start before end. If HEAD, the symlink, or the marker layout differs, stop and report — the base moved.

- [ ] **Step 2: Sweep for maintained dependents of the prose being removed (learnings: `restatement-accumulates-its-own-guards`)**

The phrases below are deleted or reworded by Step 4. For each, grep the repo and classify hits: maintained executable/test surface (`internal/`, `cmd/`, `tests/`, `skills/`, `agents/`, `scripts/`, `cursor-rules/`) vs. point-in-time records (`docs/adrs/`, `docs/changes/archive/`, `docs/results/`, `docs/superpowers/`, `testdata/`, `internal/assets/embedded/`), which are never edited:

```bash
cd /Users/homer/dev/docket/.worktrees/slim-agents-md-to-an-effective-claude-md
for p in "graduation destination" "tiering criterion" "harvest proposes" \
         "mint-stub.sh" "test_comment_anchor_style" "early-exiting-consumer" \
         "Today's grep/awk reader" "docs-shaped reading" "go run ./cmd/docket development test" \
         "no out-of-band"; do
  echo "== $p"
  grep -rIFn -e "$p" --exclude-dir=.git --exclude-dir=.worktrees --exclude-dir=testdata . | grep -vF "AGENTS.md" | grep -vF "CLAUDE.md" || true
done
```

Expected from the planning-time survey: all hits land in point-in-time records, the embedded mirror, `skills/docket-convention` (which owns the tiering criterion independently — it quotes its own copy, not AGENTS.md's), or code comments that reference the *ported* guards (`internal/app/change_create.go` mirrors `mint-stub.sh` slugify history; `internal/repoguard/anchors_test.go` says it ports `tests/test_comment_anchor_style.sh` — both are historical provenance comments, not greps of AGENTS.md, and stay untouched). Decision rule if a real maintained dependent greps AGENTS.md's copy of a deleted phrase: repoint the assert at the artifact that owns the content or the surviving canonical wording — never re-add deleted text to keep a grep green, and record the repoint in the commit message. If that happens, this becomes the one narrowly justified test-anchor edit the change file allows.

- [ ] **Step 3: Run the targeted guards on the unedited tree (green baseline)**

```bash
cd /Users/homer/dev/docket/.worktrees/slim-agents-md-to-an-effective-claude-md
go test ./internal/repoguard/ -count=1 -run 'TestFinalizeRebuildAgentsRule|TestFinalizeRebuildContract|TestDispatchBlockBudget|TestCapabilitySurface|TestCommentAnchorStyle' -v
```

Expected: PASS on all. A pre-existing red here is a blocking environmental finding — stop and report rather than editing on a red base.

- [ ] **Step 4: Replace the unmanaged prefix**

Replace lines 1–107 of `AGENTS.md` (everything above the line `<!-- docket:dispatch:start (managed by docket — do not hand-edit) -->`) with exactly the following text. Do not touch anything from that marker line to end of file. The final unmanaged line stays a blank line so the marker line remains separated exactly as it is at base.

````markdown
# AGENTS.md — always-in-context rules for this repo

These rules must fire unprompted — they guide ad hoc agent actions before any repository test
could catch a mistake. Promotion into this file is a human decision; detailed history lives in
the learnings ledger on the `docket` branch, and the docket-convention skill's *Learnings
ledger* section owns the promotion mechanics.

## Shell

- Under `set -o pipefail`, never pipe a producer into an early-exiting consumer such as
  `grep -q`, `head`, or `head -n1`: the producer takes SIGPIPE and the 141 becomes an
  intermittent failure. Capture into a variable first, then `grep <<<"$var"`.
- Declare a grep pattern that leads with `--`: `grep -E -e "<pat>"` or `grep -qF -- "<pat>"`.
  A bare leading `--` parses as an option (exit 2), and inside a negated assert (`! grep …`)
  that error inverts into a permanently green, vacuous guard.
- awk indent classes are `[^[:space:]]`, never `[^ ]` — a literal-space class silently drops
  tab-indented input.
- Always `mv -f` on install/replace paths: BSD `mv` on an unwritable destination with a tty
  prompts, self-answers `n` at EOF, and exits 0, so `|| die` guards never fire and the write is
  silently lost. `git mv` is excepted — there `-f` means force-overwrite a tracked target.
- Always template `mktemp`, with or without `-d`: `"${TMPDIR:-/tmp}/<name>.XXXXXX"` — bare
  macOS `mktemp` ignores `TMPDIR`. When the temp file must sit beside its destination for a
  same-filesystem atomic rename, template it there instead.

## Frontmatter and generated blocks

- Anchor a frontmatter-field edit to the first `---…---` block, never a bare column-0 line
  match: change/ADR bodies discuss `status:`/`updated:` in prose.
- Quote any YAML scalar carrying a colon-space, a trailing colon, a ` #`, a leading indicator
  character, or a boolean keyword (`on/off/yes/no/true/false`) — whoever writes it, model or
  script. A script writing free-text prose into frontmatter quotes unconditionally at the
  write boundary rather than predicating on shape (ADR-0071). Scalars only: a flow collection
  (`depends_on: [3]`, `adrs: [71]`) is not a scalar, and quoting one is a defect — it changes
  the parsed type from a sequence to a string.
- Before rewriting a marker-delimited managed block, validate marker order and balance —
  refuse on dangling/out-of-order/nested markers and leave the file untouched. Presence alone
  is not enough; an unbounded range consumes to EOF and eats the user's content.

## Guards and tests

- A guard is code: mutation-test it — strip the thing it guards, watch it redden — or it is
  decoration. A mutation that leaves an assert green is a defect until proven otherwise.
- Key a guard on syntactic shape, never an enumerated list of spellings. The spelling you miss
  is the target file's own house idiom.
- Never hand-list the sites of a literal or an operation you are gating — derive them from a
  whole-repo grep, then sort them into prose vs executable; only the executable ones can
  violate a gate.
- Run the whole suite at the build gate, never only the tests the spec enumerated. The BUILD
  gate's command is whatever `build.test_command` resolves to and finalize's is whatever
  `finalize.test_command` resolves to — read each from config, never from a second copy —
  entered from source so the gate tests the exact checkout under review. The Go runner
  (`internal/suiterunner`) is the sole channel; there is no separate Bash oracle.
  `tests/README.md` covers how to run the suite and where a new test belongs.
- Read the budget report even on a green run: a `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` line
  is a screening finding, and a `SERIAL CONFIRMED OVER BUDGET:` line is an authoritative
  breach to act on. Neither fails the run by default (a parallel wall-clock number is
  machine-dependent, so a real breach is confirmed serially; see `tests/README.md`), so
  nothing else will catch them for you.

## Comments and cross-references

- A cross-reference in maintained source anchors on a symbol name or a verbatim-quoted
  clause — never a line number, which nothing can check and which rots fastest in the files
  that move most (ADR-0054). `TestCommentAnchorStyle` (`internal/repoguard/anchors_test.go`)
  rejects the filename-plus-line-number form only; the bare colon-number and prose "line N"
  forms rest on this rule.
- This binds maintained source only. Point-in-time records — results files, archived changes,
  specs, and Accepted ADRs — keep whatever pointer was true when written; rewriting them
  falsifies history.

## Rebuild the binary after a merge to main

- Whenever a PR is successfully merged into `main`, rebuild the installed `docket` binary from
  a source tree proven to contain the merge, never blindly. First run the
  `repository.sync-integration` operation (argv resolved from the capability catalog) with
  `--repo-dir /Users/homer/dev/docket --json`; proceed only on disposition `advanced` or
  `already-current` — a `skipped`, `refused`, or `failed` sync leaves the rebuild incomplete.
  Confirm the checkout is clean on `main` with its full HEAD equal to the sync target, and
  prove each merge landed in it: `git merge-base --is-ancestor <landed-commit> HEAD`, where
  `<landed-commit>` is the commit the merge produced on `main` — the rebased or squashed tip,
  never the PR's feature-branch head. A negative answer and a failed probe are different
  outcomes; neither permits the install. Then resolve the `development.install` operation from
  the capability catalog, run it with `--source /Users/homer/dev/docket`, and confirm the
  installed binary's `version` operation reports the same full, clean commit id as that HEAD —
  a fresh timestamp, a short prefix, or an older running process proves nothing. On any failed
  condition, report `binary rebuild incomplete` naming it, keep the merged change done, and
  never stash, reset, or switch branches to force the rebuild — fix the reported source state,
  re-sync, and repeat.
- A merged change that extends the `.docket.yml` schema no longer blocks this: since change
  0392 the install path tolerates unknown configuration keys (surfaced as warnings), so the
  tracked `development.install` reinstall works directly with the pre-schema binary.

````

- [ ] **Step 5: Verify the managed block and symlink are untouched**

```bash
cd /Users/homer/dev/docket/.worktrees/slim-agents-md-to-an-effective-claude-md
NEW_START=$(grep -n "docket:dispatch:start" AGENTS.md | cut -d: -f1)
NEW_END=$(grep -n "docket:dispatch:end" AGENTS.md | cut -d: -f1)
sed -n "${NEW_START},${NEW_END}p" AGENTS.md > "${TMPDIR:-/tmp}/dispatch-block-after.txt"
cmp "${TMPDIR:-/tmp}/dispatch-block-before.txt" "${TMPDIR:-/tmp}/dispatch-block-after.txt" && echo BLOCK-IDENTICAL
git diff --stat                             # expect: AGENTS.md only
ls -la CLAUDE.md                            # still a symlink -> AGENTS.md
tail -1 AGENTS.md | grep -F "docket:dispatch:end" && echo TERMINAL-MARKER-OK
```

Expected: `BLOCK-IDENTICAL`, a one-file diff, the symlink unchanged, `TERMINAL-MARKER-OK`. Any mismatch: fix the prefix edit; never adjust the block to match.

- [ ] **Step 6: Run the targeted guards on the edited tree**

```bash
cd /Users/homer/dev/docket/.worktrees/slim-agents-md-to-an-effective-claude-md
go test ./internal/repoguard/ -count=1 -run 'TestFinalizeRebuildAgentsRule|TestFinalizeRebuildContract|TestDispatchBlockBudget|TestCapabilitySurface|TestCommentAnchorStyle' -v
```

Expected: PASS. If `TestFinalizeRebuildAgentsRule` reds, a pinned rebuild phrase drifted — restore the exact wording from the Global Constraints anchor list; do not edit the test.

- [ ] **Step 7: Mutation-probe one pinned anchor (learnings: `assert-detects-removal-not-replacement`, `phrase-grep-over-wrapped-prose`)**

Prove the guard still bites the new text. Temporarily change `binary rebuild incomplete` to `binary rebuild unfinished` in AGENTS.md, then:

```bash
cd /Users/homer/dev/docket/.worktrees/slim-agents-md-to-an-effective-claude-md
tr -s '[:space:]' ' ' < AGENTS.md | grep -c "binary rebuild incomplete"   # expect 0: mutation landed
go test ./internal/repoguard/ -count=1 -run 'TestFinalizeRebuildAgentsRule'
```

Expected: the grep count drops from 1 to 0 (whitespace-flattened, so wrapping cannot fake it) and the test FAILS naming the `incomplete report named` binding. Then revert the mutation by re-editing the word back (never `git checkout -- AGENTS.md`, which would destroy the whole uncommitted rewrite — learnings: `mutation-restore-needs-a-backup-copy`), re-run the same grep expecting 1 and the same test expecting PASS.

- [ ] **Step 8: Run the full repoguard package**

```bash
cd /Users/homer/dev/docket/.worktrees/slim-agents-md-to-an-effective-claude-md
go test ./internal/repoguard/ -count=1
```

Expected: PASS (this also exercises the shell-portability, retired-control-plane, source-hygiene, and budget guards over the edited file).

- [ ] **Step 9: Obligation audit and measurement**

Diff-read the old prefix against the new (`git diff AGENTS.md`) with the spec's rule-by-rule disposition table open, confirming each retained obligation survives: all five shell rules with their operand examples and both exceptions (`git mv`, beside-destination mktemp); first-block anchoring; the full scalar-quoting trigger list, unconditional script quoting, ADR-0071, scalar/collection distinction; marker order-and-balance validation with refuse-untouched behavior; mutation-testing, shape-keying, whole-repo-grep site derivation with prose/executable sort; whole-suite rule with both config keys read independently, source-checkout entry, sole Go runner, `tests/README.md` pointer; all three budget tokens with screening/authoritative distinction and default-green caveat; symbol/verbatim-clause anchor rule with ADR-0054, the ported guard's partial coverage, and the historical-record exception; the complete rebuild sequence (sync gate, disposition gate, clean-main-HEAD-equals-target, ancestry proof with landed-vs-feature-head and answer-vs-probe distinctions, catalog-resolved install, identity equality, incomplete report, done-retention, no destructive recovery) and the schema-tolerance bullet. Confirm the three obsolete pointers are gone: automated-harvest framing, `mint-stub.sh`, `tests/test_comment_anchor_style.sh`. Then measure:

```bash
cd /Users/homer/dev/docket/.worktrees/slim-agents-md-to-an-effective-claude-md
wc -l -w AGENTS.md
D_START=$(grep -n "docket:dispatch:start" AGENTS.md | cut -d: -f1)
awk -v n="$D_START" 'NR<n' AGENTS.md | wc -l -w
```

Record both numbers against the baselines (144/1603 full, 107/1192 unmanaged). Any missing obligation: restore it and re-run Steps 5–8. A smaller-than-hoped reduction is acceptable; a lost obligation is not.

- [ ] **Step 10: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/slim-agents-md-to-an-effective-claude-md
git add AGENTS.md
git commit -m "docs(0283): slim the unmanaged AGENTS.md prefix

Preserves every operational obligation and all repoguard prose anchors;
removes obsolete pointers (automated harvest, mint-stub.sh,
tests/test_comment_anchor_style.sh). Managed dispatch block byte-identical
to base. Unmanaged prefix: 1192 -> <measured> words."
```

Replace `<measured>` with Step 9's actual unmanaged word count. Stage only `AGENTS.md` — never `git add -A` (the shared-worktree discipline).

---

## Verification gate (owned by docket-build, after the task)

The build gate runs the whole configured suite (`build.test_command`, today `go run ./cmd/docket development test`) from the feature checkout. Read the budget report lines per the retained rule; a `BUDGET WATCH:`/`PARALLEL-SENSITIVE:` line is a screening finding to record, not a failure. Confirm the final diff for the branch touches only `AGENTS.md` and this plan file.

## Self-review notes (spec coverage)

- Spec "Rule-by-rule disposition" — every row is implemented by Task 1 Step 4's text; the Rebuild row is implemented against the *current* base section (post-0346 sync/ancestry/identity contract + 0392 bullet), which postdates the spec's draft and is pinned by `TestFinalizeRebuildAgentsRule`; the spec itself instructs preserving the newer base over its own snapshot.
- Spec verification items 1–6 map to Steps 1 (baseline capture), 2 (whole-repo search + historical/maintained sort), 9 (obligation review + measurement), 3/6/8 (guard runs with names discovered from current source), the docket-build gate (whole suite), and 5 (managed-block byte identity + symlink).
- No placeholder content: the complete replacement text, all commands, and expected outputs are inline.
