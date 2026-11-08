<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0514 — Retire the saved Bash upgrade test cases when stable v1.0.0 ships](../../changes/active/0514-retire-the-saved-bash-upgrade-test-cases-when-stable-v1-0-0.md)**
<!-- docket:backlink:end -->

# Retire the saved Bash upgrade test cases Implementation Plan

> **For agentic workers:** This plan is executed by `docket-build` task by task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Delete the saved Bash `v0.9.2`/`v0.9.3` upgrade cases and everything that exists only for them, keep the `dckt` alias test by moving it to `cmd/docket`, and leave `docs/release/upgrading-from-bash.md` describing only the upgrade.

**Architecture:** Pure removal plus one test move and one doc edit. `TestAliasInvocationMatchesDocket` moves into package `main` under `cmd/docket`, where `TestMain` in `main_test.go` already builds the binary once into `binPath`. Then `internal/bashupgrade/`, `testdata/bash-upgrade/`, the integration shard, its budget row and its `testdata/README.md` section are deleted together, so the shard contract and the budget correspondence guard never see one without the other. Last, the guide loses its 20 step markers and every sentence about the retired test.

**Tech Stack:** Go tests (`cmd/docket`, `internal/repoguard`), Bash suite shards under `tests/`, Markdown.

**Spec:** `docs/superpowers/specs/2026-11-08-retire-the-saved-bash-upgrade-test-cases-when-stable-v1-0-0-design.md` (on the metadata branch; read it beside this plan).

## Global Constraints

- No upgrade step, command, code block, or the guide's section structure changes. Only the sentences named in Task 3 and the marker comments go.
- No "last verified against v1.0.0" line and no other note pointing at the retired test (spec anti-goal).
- Point-in-time records stay as written: `docs/release/v1.0.0*/` (`decisions.md`, `candidate/run.txt`), `docs/results/`, `docs/superpowers/specs/`, `docs/superpowers/plans/`, `docs/changes/`, and `internal/export/testdata/corpus/`.
- No change to the installer's v0.9.2 takeover (ADR-0096) or to anything under `internal/install/`.
- The links to the guide from `README.md` and `docs/install/install.md` stay.
- New or edited comments anchor on symbol names, never line numbers (`TestCommentAnchorStyle`), and must not name `bashupgrade`, `bash-upgrade` or `BashUpgrade` (the Task 4 sweep would flag them).
- Temp dirs in the moved test come from `testsupport.TempDir(t)`, as today.
- Stage explicit paths only (`git add <path>` / `git rm -r <path>`), never `git add -A`. Run `gofmt -l cmd/docket` before the Go commit; output must be empty.
- Mutation probes: back up with `cp "$f" "$f.bak"`, mutate, prove the mutation landed with `git diff -- "$f"` showing the changed line, run with `-count=1` (learning `cached-runner-serves-a-mutated-tree`: `cmd/docket` tests build the binary in `TestMain`, so a bare `go test` can answer `(cached)`), confirm red, restore with `mv -f "$f.bak" "$f"`, confirm green. Never restore with `git checkout --` (learning `mutation-restore-needs-a-backup-copy`).
- The whole suite runs at the build gate: `build.test_command`, which resolves to `go run ./cmd/docket development test`.

## Review Focus

1. **The alias test silently disappears with the package.** A reviewer must see `TestAliasInvocationMatchesDocket` listed by `go test -list` in `./cmd/docket` and absent from any deleted path. Pinned in Task 1 Steps 2 and 4.
2. **A guard that greps a deleted copy goes red.** Removing prose that a test greps (learning `restatement-accumulates-its-own-guards`): the grooming grep found no test grepping `testdata/README.md`'s bash-upgrade section or the guide's markers outside `internal/bashupgrade`, but Task 2 Step 5 and Task 4 re-derive it rather than trust that.
3. **Shard and budget row removed out of step.** `repoguard.TestRuntimeBudgetsCorrespondence` (no orphaned row, every test file has a row) and `tests/test_go_integration_contract.sh` (integration tests vs. shard membership) must both pass after Task 2. Pinned in Task 2 Step 4.
4. **A test-coverage sentence the spec's table missed survives in the guide.** One was found while planning: the §6 `finalize.gate` row says "The saved installs cover `ci` only." Task 3 deletes it; Task 3 Step 3's grep confirms no other `test`/`saved install` sentence remains.
5. **Marker removal leaves doubled blank lines or glues prose to a fence.** Every marker today sits between a blank line and an opening fence, so deleting the marker line leaves exactly one blank line. Task 3 Step 3 checks for `\n\n\n` runs.

---

### Task 1: Move the `dckt` alias check into `cmd/docket`

**Files:**
- Create: `cmd/docket/alias_invocation_test.go`
- Delete: `internal/bashupgrade/alias_invocation_test.go`

**Interfaces:**
- Consumes: `binPath string` (package-level var set by `TestMain` in `cmd/docket/main_test.go`); `install.AliasName` (`"dckt"`, `internal/install/alias.go`); `testsupport.TempDir(t)`.
- Produces: `func TestAliasInvocationMatchesDocket(t *testing.T)` in package `main`.

- [ ] **Step 1: Write the test in its new home**

`cmd/docket/alias_invocation_test.go`, package `main`, untagged. Same body as the current `internal/bashupgrade/alias_invocation_test.go` with exactly one substitution: `bin := buildDocket(t)` becomes `bin := binPath`. Keep the symlink at `filepath.Join(testsupport.TempDir(t), install.AliasName)`, the `run` helper calling `exec.Command(path, "version", "--json").Output()`, and the failure message `"dckt version --json = %q, docket version --json = %q"`. Replace the comment with one that names no retired package, e.g.:

```go
// The alias is only useful if the binary behaves the same whatever name it was
// invoked by. This puts the binary TestMain builds once (binPath) behind a dckt
// symlink, so it costs no extra build.
```

Then `git rm internal/bashupgrade/alias_invocation_test.go` (its package keeps compiling: an unused test helper such as `buildDocket` is not a compile error).

- [ ] **Step 2: Run it and confirm it is listed and passes**

Run: `go test -count=1 -run '^TestAliasInvocationMatchesDocket$' -v ./cmd/docket/`
Expected: `--- PASS: TestAliasInvocationMatchesDocket` and `ok`.
Run: `go test -list '^TestAliasInvocationMatchesDocket$' ./internal/bashupgrade/`
Expected: no `TestAliasInvocationMatchesDocket` line.

- [ ] **Step 3: Mutation check**

With `f=cmd/docket/alias_invocation_test.go`: `cp "$f" "$f.bak"`, change `if got, want := run(alias), run(bin); got != want {` to `... got == want {`, prove the mutation landed with `diff "$f.bak" "$f"` showing that line (the file is still untracked, so `git diff` would show nothing), then run Step 2's first command.
Expected: FAIL with `dckt version --json = ...`. Restore with `mv -f`, re-run, expected PASS.

- [ ] **Step 4: gofmt, vet, commit**

Run: `gofmt -l cmd/docket && go vet ./cmd/docket/ ./internal/bashupgrade/`
Expected: no output from `gofmt`, `go vet` exits 0.

```bash
git add cmd/docket/alias_invocation_test.go
git commit -m "test: move the dckt alias check into cmd/docket"
```
(The `git rm` from Step 1 is already staged and lands in the same commit.)

---

### Task 2: Delete the saved Bash cases, their package, shard, budget row and README section

**Files:**
- Delete: `internal/bashupgrade/` (whole package: `doc.go` and every remaining `*_test.go`)
- Delete: `testdata/bash-upgrade/` (whole tree: `PROVENANCE.md`, `v0.9.2/`, `v0.9.3/`)
- Delete: `tests/test_go_integration_bashupgrade.sh`
- Modify: `tests/runtime-budgets.tsv` (remove the row `tests/test_go_integration_bashupgrade.sh	45	parallel`)
- Modify: `testdata/README.md`

**Interfaces:**
- Consumes: Task 1 (the alias test no longer lives in `internal/bashupgrade`).
- Produces: nothing later tasks call.

- [ ] **Step 1: Delete the trees and the shard**

```bash
git rm -r -q internal/bashupgrade testdata/bash-upgrade tests/test_go_integration_bashupgrade.sh
```

- [ ] **Step 2: Remove the budget row**

Delete only the `tests/test_go_integration_bashupgrade.sh` row from `tests/runtime-budgets.tsv`; leave every other row byte-identical.

- [ ] **Step 3: Edit `testdata/README.md`**

Delete the whole `## Root \`testdata/bash-upgrade/<tag>/\`` section, from that heading through the end of the file (its last line is the "**Temporary:** ..." bullet), plus the blank line before the heading, so the file ends with the `testdata/repositories/` section's last bullet and a single trailing newline. Change the intro line `Three fixture tiers, one rule each:` to `Two fixture tiers, one rule each:`.

- [ ] **Step 4: Run the guards that tie shards, budgets and fixtures together**

Run: `go vet ./... && go test -count=1 ./internal/repoguard/ && bash tests/test_go_integration_contract.sh`
Expected: `go vet` exits 0; `ok  .../internal/repoguard`; the contract shard prints no `NOT OK` line and exits 0. In particular `TestRuntimeBudgetsCorrespondence` and `TestSampleFoldersNamedForWhatTheyHold` pass.

- [ ] **Step 5: Confirm nothing in maintained source still reads what was deleted**

Run: `git grep -n -I -e 'testdata/bash-upgrade' -e 'internal/bashupgrade' -e 'test_go_integration_bashupgrade' -- ':!docs/release/v1*' ':!docs/results' ':!docs/superpowers' ':!docs/changes' ':!internal/export/testdata'`
Expected: no output. Any hit in maintained source is fixed by relocating or deleting the reference, never by restoring deleted text.

- [ ] **Step 6: Commit**

```bash
git add tests/runtime-budgets.tsv testdata/README.md
git commit -m "test: retire the saved Bash upgrade cases and their shard"
```
(The `git rm -r` from Step 1 is already staged.)

---

### Task 3: Make the upgrade guide describe only the upgrade

**Files:**
- Modify: `docs/release/upgrading-from-bash.md`

**Interfaces:**
- Consumes: nothing (the parser that read the markers was deleted in Task 2).
- Produces: nothing.

- [ ] **Step 1: Delete all 20 step markers**

Every line matching exactly `^<!-- upgrade-step: [a-z0-9-]+ -->$`. Before the edit `grep -c -E -e '^<!-- upgrade-step: [a-z0-9-]+ -->$' docs/release/upgrading-from-bash.md` prints `20`; after, `0`.

- [ ] **Step 2: Edit the passages about the test**

Rewrap touched paragraphs to the file's ~100-column width; change nothing else.

| Where | Today | After |
|---|---|---|
| Intro | Paragraph starting "An automated test runs every marked step below against saved copies…" through "…the guide says so." | Delete the paragraph (and one of the two blank lines around it). |
| §2, first bullet | "…finish or kill every change that is in progress. The test covers repositories with no change in progress." | "…finish or kill every change that is in progress. The guide covers repositories with no change in progress." |
| §3, after the install block | "The download and checksum lines are checked by the release's own verification, not by this guide's test." | Delete the sentence. Keep "The `shasum` line must print `install.sh: OK`. If it prints anything else, stop and do not run `install.sh`." |
| §6, `finalize.gate` row | "Error when set to `ci` or `both`. The saved installs cover `ci` only." | "Error when set to `ci` or `both`." (the Setting and Fix cells are unchanged) |
| §7, after the leftovers block | "The test covers zsh, where those lines are in `~/.zshenv`. Other shells keep them in a different startup file, which the test does not cover." | "These lines are in `~/.zshenv` for zsh. Other shells keep them in a different startup file." |
| §7, repository agent files paragraph | "…If a repository has none, the command does nothing. The saved installs the test runs carry no repository Cursor agent files, so the test does not prove the `.cursor/agents` line." | Delete the last sentence; the paragraph ends at "…the command does nothing." |
| §7, `.claude/settings.local.json` paragraph | "This guide does not cover that file: the test leaves it in place and does not show whether deleting it is safe." | "This guide does not cover that file and does not say whether deleting it is safe." |

- [ ] **Step 3: Verify the guide**

Run: `grep -n -i -w -e test -e tests docs/release/upgrading-from-bash.md; grep -n -i -e 'saved install' -e 'saved cop' -e 'upgrade-step' docs/release/upgrading-from-bash.md`
Expected: only hits about docket's own test command (`test-config-missing`, "No test command is set up", `configure-tests`, "test settings", "the test command"). Any other sentence about a test is deleted if it only describes test coverage, or rewritten as the guide's own scope if the limit still matters to a reader.
Run: `awk 'BEGIN{b=0} /^$/{b++; if(b>=2){print "double blank at " NR}; next} {b=0}' docs/release/upgrading-from-bash.md`
Expected: no output.
Run: `git diff --stat -- docs/release/upgrading-from-bash.md` and read `git diff -- docs/release/upgrading-from-bash.md`.
Expected: no added or removed line inside a code fence, and no table row changed except the `finalize.gate` row's middle cell.

- [ ] **Step 4: Commit**

```bash
git add docs/release/upgrading-from-bash.md
git commit -m "docs: describe only the upgrade in the Bash upgrade guide"
```

---

### Task 4: Whole-repo sweep and full suite

**Files:**
- None expected. Any maintained-source hit from Step 1 is fixed in the file it lands in.

**Interfaces:**
- Consumes: Tasks 1–3.
- Produces: the build-gate result.

- [ ] **Step 1: Re-run the spec's whole-repo grep and sort every hit**

Run: `git grep -n -I -e bashupgrade -e bash-upgrade -e BashUpgrade -e upgrade-step`
Expected: every hit is under a point-in-time path from Global Constraints (`docs/release/v1*/`, `docs/results/`, `docs/superpowers/specs/`, `docs/superpowers/plans/`, `docs/changes/`, `internal/export/testdata/corpus/`). A hit anywhere else is maintained source: fix it (delete or relocate the reference) and commit it with `test:` or `docs:` as fits, staging its explicit path.

- [ ] **Step 2: Run the full suite**

Run: `go run ./cmd/docket development test` (the resolved `build.test_command`).
Expected: `SUITE ... failed=0`; no line names `test_go_integration_bashupgrade`. Read the budget report: no `OVER BUDGET` line, and no orphaned-row or missing-row complaint. A red file is first re-run on the unmodified base before it is treated as caused by this change (learning `environment`).
