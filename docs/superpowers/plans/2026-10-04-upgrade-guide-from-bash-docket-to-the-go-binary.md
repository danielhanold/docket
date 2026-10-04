<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0511 — Upgrade guide from Bash docket to the Go binary, proven on saved v0.9.2 and v0.9.3 installs](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0511-upgrade-guide-from-bash-docket-to-the-go-binary-proven-on-sa.md)**
<!-- docket:backlink:end -->
# Upgrade Guide from Bash docket to the Go Binary Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use docket-build to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship `docs/release/upgrading-from-bash.md`, a user guide for moving a Bash docket v0.9.2 or v0.9.3 install and its `docket`-branch repositories to the Go binary. Prove every step of it with an integration test that runs the guide's own marked blocks against saved Bash state built from the real v0.9.2 and v0.9.3 tags.

**Architecture:** Each tag's Bash state is built once, by hand, in a throwaway sandbox (temp `HOME`, every `XDG_*` inside it, a local bare `origin`). It is then committed under `testdata/bash-upgrade/<tag>/` as a git bundle of `origin`, a placeholder-rewritten `home.tar` of the harness folders, a replayable clone-config file, a record inventory and a `PROVENANCE.md`. A new test-only package, `internal/bashupgrade`, does four things: builds `./cmd/docket`, restores each case into a per-test sandbox, extracts the guide's `<!-- upgrade-step: <name> -->` blocks, and drives them through a name-keyed registry of run/observe/mirror actions. It then asserts the end state. A new shard runner, `tests/test_go_integration_bashupgrade.sh`, puts the test in the suite.

**Tech Stack:** Go (stdlib `archive/tar`, `encoding/json`, `os/exec`), `go test -tags integration`, the v0.9.2/v0.9.3 Bash tags (run only as subprocesses), git bundles, POSIX sh shard runner.

**Spec:** `docs/superpowers/specs/2026-10-04-upgrade-guide-from-bash-docket-to-the-go-binary-proven-on-sa-design.md` (on the `docket` branch; synchronized copy at `/Users/homer/dev/docket/.docket/docs/superpowers/specs/2026-10-04-upgrade-guide-from-bash-docket-to-the-go-binary-proven-on-sa-design.md`). Read it in full before Task 1. Its tables of traced facts (findings, config behavior, leftovers) are hypotheses that the test confirms or corrects.

## Global Constraints

- **No product code change.** Nothing under `cmd/`, or under `internal/` outside the new `internal/bashupgrade/` package, may change. Spec section 4 governs defects: if following the guide on a saved case fails because the binary misbehaves, and the guide cannot honestly explain it, **stop and return BLOCKED** with the defect and its evidence (command, full stdout/stderr, relevant `git` object ids). Do not patch the binary. Do not write a guide step that asks the user to hand-edit the `docket` branch or to bypass a refusal; that is a defect, not a step. The most likely defect is `verifyMetadataOwnership` (`internal/app/metadata_ownership.go`) refusing a Bash-made `docket` branch (`metadata-root-foreign`, `metadata-root-unresolved`, `metadata-ownership-unverified`).
- **Sandbox discipline for the Bash tags.** Never touch the real `HOME`, `~/.claude`, `~/.config`, `~/dev/docket`'s working tree or refs, GitHub, or the network. Run the tag scripts only as subprocesses under `env -i` with the sandbox environment. **Never `source` a tag script into the agent's shell.** Bash docket scripts are not pure, and sourcing one has committed and pushed before. Read `~/dev/docket` only through `git clone`/`git show` of the tag.
- **No generator script is committed.** The tags are the generator. Scratch helper scripts live only in the session scratchpad. Every command they ran is copied verbatim into `PROVENANCE.md`.
- Saved form per tag: `testdata/bash-upgrade/<tag>/` holding `origin.bundle`, `home.tar`, `clone-config.txt`, `records.txt` and `PROVENANCE.md`. Keep each tag directory under 1 MB (`du -sk` it and record the number in the results).
- One placeholder token for the sandbox home, `@@SANDBOX_HOME@@`, rewritten in both symlink targets and regular-file contents inside `home.tar`. The test substitutes it on restore.
- The test name prefix is **`TestIntegrationBashUpgrade`**. The spec says "`TestBashUpgrade` name prefix", but `tests/test_go_integration_contract.sh` rule (2) rejects any integration-tagged test without a `TestIntegration` prefix. `TestIntegrationBashUpgrade` keeps the spec's stem and satisfies the contract. Record this in the results file so 0366's gate names the right prefix.
- The integration test file opens with `//go:build integration` on line 1 and a blank line 2.
- Temp dirs in `internal/bashupgrade` tests come from `testsupport.TempDir(t)`, never `t.TempDir()` or `os.MkdirTemp`. The exception is a `// tempdir-exempt: <reason>` line where a lifetime truly needs it (`TestRealProcessPackagesUseFixtureTempDir`, `TestMkdirTempViolations`). Spawned git gets `testsupport.GitEnv(t)` appended last.
- Guide voice: written for users, in plain words; present behavior only; no change numbers; no history narration. Claude Code only. Say that Cursor and OpenCode sections arrive in later pre-releases and that Codex is not supported. No rollback instructions. Include exactly one factual line that the Bash tags `v0.9.2` and `v0.9.3` remain available. Run `humanizer` over the prose before committing it.
- **Truth rule.** Every fact the guide states must be one the test observed, or one that 0366's release verification proves (the real download and its checksum step). Remove a claim neither covers, or mark it explicitly as not covered.
- `README.md` and `docs/install/install.md` link to the guide and spell no refused setting. `TestLivingDocsAlignment` checks this.
- Cross-references in comments anchor on symbol names, never line numbers (ADR-0054).
- Every mutation probe and re-verification uses `go test -count=1`. Restore a mutated file from a `cp` backup, never with `git checkout --`. Confirm a mutation landed (`git diff` / `cmp`) before reading its result.
- Stage explicit paths only, never `git add -A`. Run `gofmt -l internal/bashupgrade` before each Go commit; the output must be empty.
- The build gate is the whole suite through `build.test_command` (`go run ./cmd/docket development test`). Read the budget report lines even when the run is green.

## Review Focus

1. **A Bash user's clone already has a `.docket/` metadata worktree** on the `docket` branch, plus whatever local git config the tag set. `repository prepare` should adopt that and not refuse it as `docket-dir-foreign`. Task 2's restore recreates the worktree exactly as the tag did (recorded in `clone-config.txt`), and the probe asserts no `docket-dir-*` refusal.
2. **On a failed `docket install`, the release downloader leaves no `docket` on `PATH`.** It runs the staged binary's `install` and dies with "the existing binary was left untouched". The v0.9.3 remedy must therefore say to re-run the *download command*, not `docket install`. Task 5's mirror asserts `$HOME/.local/bin/docket` is absent after the first v0.9.3 run, and the guide-truth assert checks that the remedy text names the downloader re-run.
3. **The Bash install left more than agent files.** It left skill symlinks under `~/.claude/skills/` pointing at an intact checkout, a dispatch block in `~/.claude/CLAUDE.md`, a `DOCKET_SCRIPTS_DIR`/`DOCKET_BASH_PATH` env block in `~/.claude/settings.json`, and a shell-profile export. The installer's handling of each must be observed, not assumed. Task 5 asserts that every path an ownership conflict names is covered by the guide's remedy text. Task 6 asserts that every Bash leftover the guide lists still exists after the upgrade (so it is a real leftover) and is not named by `docket install check`.
4. **A placeholder that survives restore** (an unsubstituted `@@SANDBOX_HOME@@` in a symlink target or file) turns every later assert into a test of a broken home. Task 2 asserts that zero occurrences remain anywhere under the restored home, in link targets and in bytes.
5. **The saved inventory drifting from the bundle.** If `records.txt` disagrees with what the bundle actually holds, "every record survives" is vacuous for the missing ones. Task 2 asserts `records.txt` equals the pre-upgrade listing of the restored repository in both directions, and Task 5 asserts every entry survives post-upgrade.

---

### Task 1: Build the saved v0.9.2 Bash state in a sandbox

**Files:**
- Create: `testdata/bash-upgrade/v0.9.2/origin.bundle`
- Create: `testdata/bash-upgrade/v0.9.2/home.tar`
- Create: `testdata/bash-upgrade/v0.9.2/clone-config.txt`
- Create: `testdata/bash-upgrade/v0.9.2/records.txt`
- Create: `testdata/bash-upgrade/v0.9.2/PROVENANCE.md`
- Create: `testdata/bash-upgrade/PROVENANCE.md` (tree-root pointer: one line per tag naming its own `PROVENANCE.md`)
- Modify: `testdata/README.md` (add a third tier, *Root `testdata/bash-upgrade/<tag>/`*: saved Bash-install upgrade cases, made once from the tags in a sandbox, immutable, temporary until stable v1.0.0, with a `PROVENANCE.md` per tag)

**Interfaces:**
- Produces, for Tasks 2–6 (file formats are the contract):
  - `origin.bundle`: `git bundle create … --all` of the sandbox bare origin. It carries `refs/heads/main` (integration) and `refs/heads/docket`, with exact history.
  - `home.tar`: an uncompressed tar, rooted at the sandbox `HOME` (member names are relative: `.claude/agents/docket-build.md`, `dev/docket/skills/docket-build/SKILL.md`, …). Symlinks are kept as symlink members. Every absolute sandbox-home prefix in a symlink target or regular-file body is replaced by `@@SANDBOX_HOME@@`.
  - `clone-config.txt`: one shell-free line per action the restore must replay on the clone, in order. The grammar is exactly `config <key> <value>` (a `git config --local` setting) or `worktree <path> <branch>` (a `git worktree add <path> <branch>` relative to the clone root). Blank lines and `#` comments are allowed.
  - `records.txt`: sorted repo-relative paths, prefixed `docket:` or `main:` for the branch they live on. It lists every change record, ADR, spec and learning that existed when the case was saved. `BOARD.md` and generated indexes are excluded.

This task is manual sandbox work. Its "test" is a set of verification commands whose output is copied into `PROVENANCE.md`.

- [ ] **Step 1: Create the sandbox and a fail-loud `gh` stub**

Work under the session scratchpad. `$SCRATCH` below is the scratchpad directory the harness gave you.

```bash
SB="$(mktemp -d "$SCRATCH/bash-upgrade-v092.XXXXXX")"
mkdir -p "$SB/home" "$SB/stub-bin" "$SB/home/.config" "$SB/home/.local/share" "$SB/home/.cache" "$SB/home/.local/state"
printf '#!/bin/sh\necho "gh stub: network is forbidden in the sandbox: $*" >&2\necho "$*" >> "%s/gh-calls.log"\nexit 97\n' "$SB" > "$SB/stub-bin/gh"
chmod +x "$SB/stub-bin/gh"
printf '[user]\n\tname = Bash Upgrade Fixture\n\temail = fixture@docket.invalid\n[init]\n\tdefaultBranch = main\n' > "$SB/gitconfig"
```

Write one scratch wrapper, `$SB/sbx`, that runs any command inside the sandbox. Every later command in this task goes through it:

```bash
cat > "$SB/sbx" <<EOF
#!/bin/sh
exec env -i HOME="$SB/home" USER=fixture LOGNAME=fixture SHELL=/bin/zsh TERM=dumb LANG=en_US.UTF-8 \\
  XDG_CONFIG_HOME="$SB/home/.config" XDG_DATA_HOME="$SB/home/.local/share" \\
  XDG_CACHE_HOME="$SB/home/.cache" XDG_STATE_HOME="$SB/home/.local/state" \\
  XDG_BIN_HOME="$SB/home/.local/bin" TMPDIR="$SB/tmp" \\
  GIT_CONFIG_GLOBAL="$SB/gitconfig" GIT_CONFIG_NOSYSTEM=1 \\
  PATH="$SB/stub-bin:/opt/homebrew/bin:/usr/bin:/bin:/usr/sbin:/sbin" "\$@"
EOF
chmod +x "$SB/sbx"; mkdir -p "$SB/tmp"
"$SB/sbx" sh -c 'echo "HOME=$HOME"; command -v docket || echo "no docket on PATH"; env | grep -c -e "^DOCKET_" || true'
```

Expected: `HOME` is inside `$SB`, `no docket on PATH`, and a `DOCKET_` count of `0`. If `/opt/homebrew/bin` holds a `docket`, drop it from the wrapper's PATH and give Bash 4+ through its absolute path instead (`/opt/homebrew/bin/bash`). Record the choice.

- [ ] **Step 2: Check the tag out into the sandbox home, outside the repository**

```bash
"$SB/sbx" git clone --quiet --branch v0.9.2 --depth 1 file:///Users/homer/dev/docket "$SB/home/dev/docket"
"$SB/sbx" git -C "$SB/home/dev/docket" remote remove origin
git -C /Users/homer/dev/docket rev-parse 'v0.9.2^{commit}'     # expect ca8251e8bc0a7944ece39dd750b1b159c978f2b3
shasum -a 256 "$SB/home/dev/docket/install.sh"
```

Record the commit and the `install.sh` SHA-256. `~/dev/docket` is the tag's own default checkout location (`link-skills.sh` header), so the skill symlinks will look like a real user's.

- [ ] **Step 3: Pre-create the harness folders the tag recognizes, then run its installer**

Read `$SB/home/dev/docket/link-skills.sh` (`HARNESS_SKILL_DIRS`) and `$SB/home/dev/docket/sync-agents.sh` (its per-harness target roots) to list the harness parents. At v0.9.2 they include at least `.claude`, `.cursor`, `.codex`, `.agents` and the OpenCode config root. Create the parent directory of each one that belongs to Claude Code, Cursor, OpenCode or Codex (Codex is saved, never asserted). Do not create the others. Then:

```bash
"$SB/sbx" sh -c 'cd "$HOME/dev/docket" && /opt/homebrew/bin/bash ./install.sh' 2>&1 | tee "$SB/install.log"
```

Expected: `docket: install complete`. If it stops because a harness is absent, create that harness's parent and re-run (the installer is idempotent). Record every attempt. Confirm that `$SB/gh-calls.log` does not exist. Then check that the real home is untouched: `ls -la ~/.claude/agents | head` should show only files older than this task's start.

- [ ] **Step 4: Bootstrap a consuming repository through the tag's own empty-orphan `docket`-branch bootstrap**

The v0.9.2 case covers the fresh-repository bootstrap (`docket-config.sh --export --bootstrap`, the `CREATE_ORPHAN` cell, which writes an empty-tree root). The v0.9.3 case in Task 3 covers the `migrate-to-docket.sh` seeded-root bootstrap. Between them they exercise both receiptless legacy proofs (`proofLegacyEmpty`, `proofLegacyEquivalent`).

```bash
"$SB/sbx" git init --quiet --bare -b main "$SB/origin.git"
"$SB/sbx" git clone --quiet "$SB/origin.git" "$SB/home/dev/sample"
"$SB/sbx" sh -c 'cd "$HOME/dev/sample" && printf "# sample\n" > README.md && git add README.md && git commit -qm "initial" && git push -q origin main && git remote set-head origin main'
```

Read `$SB/home/dev/docket/scripts/docket-config.md` (*Bootstrap* 2×2 and `--bootstrap` write path) and `skills/docket-convention/SKILL.md` (its bootstrap guard) for the exact `.docket.yml` a fresh docket-mode repository starts with. If none is needed, commit none. Then run the bootstrap the way a skill's Step 0 does:

```bash
"$SB/sbx" sh -c 'cd "$HOME/dev/sample" && export DOCKET_SCRIPTS_DIR="$HOME/dev/docket/scripts" DOCKET_BASH_PATH=/opt/homebrew/bin/bash && "$DOCKET_BASH_PATH" "$DOCKET_SCRIPTS_DIR/docket-config.sh" --export --bootstrap'
```

Expected: `BOOTSTRAP=PROCEED` and `origin/docket` existing with an empty-tree root (`git -C … rev-parse 'origin/docket^{tree}'` equals `4b825dc642cb6eb9a060e54bf8d69288fbee4904`). Commit the `.gitignore` block it left modified, as its `COMMIT THIS` notice tells the user, and push. Create the `.docket/` metadata worktree the way the tag's skills do (read the convention's *Metadata worktree* section and use its exact command). Record that command.

- [ ] **Step 5: Populate the repository through the tag's own flows**

Minimum content: one proposed change with a spec; one trivial proposed change; one deferred change; two done changes in the archive (terminal-published to `main` the way the tag's finalize does); one ADR; the board as the tag renders it; and learnings, which v0.9.2 has (`render-learnings-index.sh`). Use the tag's scripts wherever one exists: `mint-stub.sh` for stubs, `render-board.sh`/`board-refresh.sh` for the board, `archive-change.sh` + `terminal-publish.sh` for done changes, `render-adr-index.sh`, `render-learnings-index.sh`, `render-change-links.sh`/`render-artifact-backlink.sh`. Read each script's `.md` contract first, and run each one only through `"$SB/sbx" /opt/homebrew/bin/bash <script> …` with `DOCKET_SCRIPTS_DIR`/`DOCKET_BASH_PATH` exported inside the `sh -c`. Where a flow is an interactive agent step (writing a spec body, a groom verdict, a defer), hand-author the file with the exact frontmatter fields and section headings the tag's skill prescribes. Commit it in the `.docket` worktree with the subject style that skill uses, and push. Record each such gap in the provenance note ("hand-authored per `skills/<x>/SKILL.md` §…").

After each push, confirm `$SB/gh-calls.log` is still absent. If a script needs `gh` and fails, record the gap and take the hand-authored route.

- [ ] **Step 6: Leave the `.docket.yml` the bootstrap produced (the common path)**

Make no config edits beyond what Steps 4–5 required. Run the tag's resolver read-only and keep the output for the provenance note:

```bash
"$SB/sbx" sh -c 'cd "$HOME/dev/sample" && DOCKET_BASH_PATH=/opt/homebrew/bin/bash /opt/homebrew/bin/bash "$HOME/dev/docket/scripts/docket-config.sh" --export' > "$SB/config-export.txt"
```

- [ ] **Step 7: Export the saved form**

```bash
OUT=/Users/homer/dev/docket/.worktrees/upgrade-guide-from-bash-docket-to-the-go-binary-proven-on-sa/testdata/bash-upgrade/v0.9.2
mkdir -p "$OUT"
"$SB/sbx" git -C "$SB/origin.git" bundle create "$OUT/origin.bundle" --all
git bundle verify "$OUT/origin.bundle"     # expect refs/heads/main and refs/heads/docket listed
```

`clone-config.txt`: compare `git -C "$SB/home/dev/sample" config --local --list` with a fresh `git clone` of `origin.git`. Write one `config <key> <value>` line per key the tag added, excluding `remote.*`, `branch.*` and `core.*` defaults that every clone has. Add one `worktree .docket docket` line, or whatever path/branch Step 4 recorded.

`records.txt`: list the files and keep only the record kinds:

```bash
{ git -C "$SB/origin.git" ls-tree -r --name-only docket | sed 's/^/docket:/'; git -C "$SB/origin.git" ls-tree -r --name-only main | sed 's/^/main:/'; } > "$SB/all-paths.txt"
```

Keep the change records (`docs/changes/active/*.md`, `docs/changes/archive/*.md`), ADRs (`docs/adrs/[0-9]*.md`), specs (`docs/superpowers/specs/*.md`) and learnings (`docs/changes/learnings/*.md` except `README.md`), with paths resolved through the case's config. Drop `BOARD.md` and index `README.md`s. Sort with `LC_ALL=C sort`.

`home.tar`: write a scratch Python 3 helper (never committed) that walks `$SB/home` with `os.walk(followlinks=False)`. It includes only the harness folders (`.claude`, `.cursor`, `.codex`, `.agents`, the OpenCode root, `.config/docket`) plus `dev/docket/skills` (the tree the skill links point at) and the shell profile files the tag's `ensure-docket-env.sh` wrote. It does not include `dev/sample`, `.cache`, or the rest of `dev/docket`. The helper writes a `tarfile` in `USTAR_FORMAT`, with mtimes zeroed and uid/gid 0. In every symlink target and every regular-file body it replaces the absolute `$SB/home` prefix with `@@SANDBOX_HOME@@`, and any `realpath` spelling of it too (`/private/var/...` vs `/var/...` on macOS). After writing, assert that the original prefix appears nowhere: `tar -xOf "$OUT/home.tar" | grep -c -e "$SB" || true` must print `0`. Then check `tar -tvf "$OUT/home.tar" | grep -c -e '^l'` (symlink count > 0).

- [ ] **Step 8: Write `PROVENANCE.md` and check the size**

`testdata/bash-upgrade/v0.9.2/PROVENANCE.md` must state:
- tag, commit, `install.sh` SHA-256, date, host OS and Bash version;
- the sandbox `sbx` environment (all variables) and every command run, verbatim, in order, including failed attempts;
- the bootstrap kind (empty-orphan `CREATE_ORPHAN`) and the `.docket` worktree command;
- one row per record in `records.txt` saying what it exercises (proposed+spec, trivial, deferred, done×2, ADR, learning);
- the `config-export.txt` output;
- which harness folders were written, and that Codex files are saved but never asserted;
- every gap (hand-authored steps, scripts that needed `gh`), and the empty categories (learning *frozen-corpus-covers-what-it-contains*);
- that the case is temporary and is retired when stable v1.0.0 ships.

```bash
du -sk testdata/bash-upgrade/v0.9.2     # expect < 1024
```

- [ ] **Step 9: Commit**

```bash
git -C /Users/homer/dev/docket/.worktrees/upgrade-guide-from-bash-docket-to-the-go-binary-proven-on-sa add testdata/README.md testdata/bash-upgrade/PROVENANCE.md testdata/bash-upgrade/v0.9.2
git -C /Users/homer/dev/docket/.worktrees/upgrade-guide-from-bash-docket-to-the-go-binary-proven-on-sa commit -m "test(bash-upgrade): saved v0.9.2 Bash install and docket-branch repository"
```

---

### Task 2: Restore harness and the metadata-ownership probe (BLOCKED gate)

This is the spec's highest-risk unknown. Its job is to answer, with evidence, whether the Go binary accepts the Bash-made `docket` branch, before any guide work starts.

**Files:**
- Create: `internal/bashupgrade/doc.go`
- Create: `internal/bashupgrade/restore_test.go` (untagged: restore and sandbox helpers shared by the default and integration builds)
- Create: `internal/bashupgrade/restore_unit_test.go` (default tag: unit tests for the placeholder rewrite and `clone-config.txt` parsing)
- Create: `internal/bashupgrade/bashupgrade_integration_test.go`
- Create: `tests/test_go_integration_bashupgrade.sh`
- Modify: `tests/runtime-budgets.tsv` (one row)

**Interfaces:**
- Consumes: Task 1's file formats.
- Produces (package `bashupgrade`, test files):
  - `type upgradeCase struct { Tag, Root, Home, Origin, Clone, BinDir string; Env []string; Docket string }`. `Root` is the per-test sandbox root. `Home` is the restored home. `Clone` is the user's repository. `BinDir` is `$Home/.local/bin`. `Docket` is the absolute path of the freshly built binary (not yet in `BinDir`). `Env` is the complete environment for every subprocess.
  - `func restoreCase(t *testing.T, tag string) *upgradeCase`
  - `func buildDocket(t *testing.T) string` (builds `./cmd/docket` once per test binary through `sync.Once`, into a `// tempdir-exempt:` process-lifetime dir)
  - `func (c *upgradeCase) run(t *testing.T, dir string, name string, args ...string) cmdResult`, where `type cmdResult struct { Stdout, Stderr string; Code int }`
  - `func findingCodes(t *testing.T, stdout string) []findingRef`, where `type findingRef struct { Code, Severity string }`. It decodes the JSON document and walks it recursively, collecting every object that has string fields `code` and `severity`. This is robust to the wrapper shape.
  - `func repoRoot(t *testing.T) string` (module root via `runtime.Caller`, as in `internal/release/package_integration_test.go`)
  - `func readRecords(t *testing.T, tag string) []string` and `func listRecords(t *testing.T, c *upgradeCase) []string` (same grammar as `records.txt`, computed from `origin` with `git ls-tree`)

- [ ] **Step 1: Write the failing unit tests for the pure helpers**

`internal/bashupgrade/restore_unit_test.go`:

```go
package bashupgrade

import (
	"reflect"
	"testing"
)

func TestSubstituteHome(t *testing.T) {
	got := substituteHome([]byte("x=@@SANDBOX_HOME@@/dev/docket/scripts\ny=@@SANDBOX_HOME@@"), "/tmp/h")
	if string(got) != "x=/tmp/h/dev/docket/scripts\ny=/tmp/h" {
		t.Fatalf("substituteHome = %q", got)
	}
	if string(substituteHome([]byte("no token"), "/tmp/h")) != "no token" {
		t.Fatal("substituteHome changed bytes without a token")
	}
}

func TestParseCloneConfig(t *testing.T) {
	in := "# replay\nconfig core.hooksPath /dev/null\n\nworktree .docket docket\n"
	got, err := parseCloneConfig(in)
	if err != nil {
		t.Fatal(err)
	}
	want := []cloneAction{{Kind: "config", A: "core.hooksPath", B: "/dev/null"}, {Kind: "worktree", A: ".docket", B: "docket"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseCloneConfig = %#v", got)
	}
	for _, bad := range []string{"config onlykey\n", "worktree .docket\n", "exec rm -rf /\n", "config a b c\n"} {
		if _, err := parseCloneConfig(bad); err == nil {
			t.Errorf("parseCloneConfig(%q) accepted a malformed line", bad)
		}
	}
}

func TestFindingCodesWalksAnyShape(t *testing.T) {
	doc := `{"result":"applied","checks":[{"findings":[{"code":"board-stale","severity":"warning"}]}],"findings":[{"code":"test-config-missing","severity":"error"}]}` + "\n"
	got := findingCodes(t, doc)
	want := []findingRef{{"board-stale", "warning"}, {"test-config-missing", "error"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("findingCodes = %#v", got)
	}
}
```

- [ ] **Step 2: Run them and confirm they fail to compile**

Run: `go test -count=1 -run 'TestSubstituteHome|TestParseCloneConfig|TestFindingCodesWalksAnyShape' ./internal/bashupgrade/`
Expected: FAIL (`undefined: substituteHome`, and so on; also no non-test Go files until `doc.go` exists).

- [ ] **Step 3: Write `doc.go` and the helpers in `restore_test.go`**

`internal/bashupgrade/doc.go`:

```go
// Package bashupgrade holds the temporary proof that docs/release/upgrading-from-bash.md
// works: integration tests restore saved Bash docket v0.9.2/v0.9.3 installs from
// testdata/bash-upgrade/ and run the guide's own marked steps against the binary
// built from this checkout. The package has no non-test code beyond this file and is
// deleted, with testdata/bash-upgrade/, when stable v1.0.0 ships.
package bashupgrade
```

`internal/bashupgrade/restore_test.go` (untagged) implements the following.

`substituteHome(b []byte, home string) []byte` replaces every `@@SANDBOX_HOME@@` with `home`.

`parseCloneConfig(s string) ([]cloneAction, error)` follows the exact grammar from Task 1. Every non-blank, non-`#` line must split by `strings.Fields` into exactly 3 fields, with the first being `config` or `worktree`. Anything else is an error naming the line.

`findingCodes` uses `json.Unmarshal` into `any` and walks it depth-first in document order (map keys sorted for determinism), collecting `findingRef` from any `map[string]any` whose `code` and `severity` are strings.

`restoreCase(t, tag)`:
1. `root := testsupport.TempDir(t)`. Set `home := root+"/home"`. Read `filepath.Join(repoRoot(t), "testdata/bash-upgrade", tag, "home.tar")` with `archive/tar`. For each member: reject absolute names or names containing `..`. Recreate directories (`0o755` or header mode). Write regular files with `substituteHome` applied and the header mode. Create symlinks with `os.Symlink(string(substituteHome([]byte(h.Linkname), home)), dst)`.
2. Walk the restored home. `t.Fatalf` if any regular file's bytes or any symlink target still contains `@@SANDBOX_HOME@@` (Review Focus 4).
3. `git init --bare -b main <root>/origin.git`, then `git -C origin.git fetch <bundle> 'refs/heads/*:refs/heads/*'`. Clone it into `<home>/dev/sample` (the same path the Bash sandbox used, so restored absolute paths agree). Run `git -C clone remote set-head origin main`.
4. Replay `clone-config.txt`: `config` becomes `git -C clone config --local A B`, and `worktree` becomes `git -C clone worktree add A B` (the branch is created tracking `origin/B` when it is not local).
5. `Env`: start from an empty slice, not `os.Environ()`. Add `HOME`, `USER=fixture`, `SHELL=/bin/zsh`, `TERM=dumb`, the five `XDG_*` variables (inside `home`) plus `XDG_BIN_HOME=<home>/.local/bin`, `TMPDIR=<root>/tmp` (created), and `PATH=<home>/.local/bin:/usr/bin:/bin:/usr/sbin:/sbin`, then append `testsupport.GitEnv(t)...` last. No `DOCKET_*` variable is inherited.
6. `Docket: buildDocket(t)`.

`buildDocket` runs `go build -o <dir>/docket ./cmd/docket` from `repoRoot`, once, under `sync.Once`. It stores the error and `t.Fatal`s on it in every caller.

`run` executes `exec.Command(name, args...)` with `Dir: dir` and `Env: c.Env`, captures stdout/stderr, and maps `*exec.ExitError` to `Code`. Any other error is `t.Fatalf`.

- [ ] **Step 4: Run the unit tests and confirm they pass**

Run: `go test -count=1 -run 'TestSubstituteHome|TestParseCloneConfig|TestFindingCodesWalksAnyShape' ./internal/bashupgrade/`
Expected: PASS.

- [ ] **Step 5: Write the ownership probe as an integration test**

`internal/bashupgrade/bashupgrade_integration_test.go`:

```go
//go:build integration

package bashupgrade

import (
	"reflect"
	"strings"
	"testing"
)

// ownershipRefusals are the repository-setup finding codes that mean the binary
// refused (or could not prove) docket ownership of the Bash-made metadata branch.
// Any of them on a saved case is a binary defect per the spec's "When the binary is
// wrong" rule, never a guide step.
var ownershipRefusals = []string{
	"metadata-root-foreign", "metadata-root-unresolved", "metadata-ownership-unverified",
	"metadata-presence-unknown", "docket-dir-foreign",
}

func TestIntegrationBashUpgradeOwnership(t *testing.T) {
	for _, tag := range savedTags(t) {
		t.Run(tag, func(t *testing.T) {
			c := restoreCase(t, tag)
			if got, want := listRecords(t, c), readRecords(t, tag); !reflect.DeepEqual(got, want) {
				t.Fatalf("records.txt disagrees with the saved bundle\n got %v\nwant %v", got, want)
			}
			for _, args := range [][]string{{"repository", "prepare", "--json"}, {"repository", "check", "--json"}} {
				r := c.run(t, c.Clone, c.Docket, args...)
				for _, f := range findingCodes(t, r.Stdout) {
					for _, bad := range ownershipRefusals {
						if f.Code == bad {
							t.Fatalf("BLOCKED: %s refused the Bash-made docket branch (%s)\nstdout:\n%s\nstderr:\n%s",
								strings.Join(args, " "), f.Code, r.Stdout, r.Stderr)
						}
					}
				}
				if r.Stdout == "" {
					t.Fatalf("%s produced no JSON (code %d); stderr:\n%s", strings.Join(args, " "), r.Code, r.Stderr)
				}
			}
		})
	}
}
```

`savedTags(t)` lists the subdirectories of `testdata/bash-upgrade/` that hold an `origin.bundle`, sorted. It `t.Fatal`s when the list is empty (population floor). Put it in `restore_test.go`. Do not hand-list the tags: Task 3 adds v0.9.3 with no test edit. `listRecords` applies Task 1 Step 7's filter to `git -C origin.git ls-tree -r --name-only <branch>`. If the filter needs the case's config dirs, read `.docket.yml` from `main` with `git show` and use the default dirs when it is absent.

Verify the JSON flag and the shape of the `prepare`/`check` output against the real binary before trusting the assert (learning *plan-supplied-test-code-is-unverified*): `go run ./cmd/docket repository check --help`.

- [ ] **Step 6: Add the shard runner and its budget row**

`tests/test_go_integration_bashupgrade.sh`: copy `tests/test_go_integration_app_archive.sh` byte for byte, then change only the header comment (describe this shard: the Bash v0.9.2/v0.9.3 upgrade proof, temporary until stable v1.0.0, prefix `^TestIntegrationBashUpgrade`), `SHARD_PKG="./internal/bashupgrade"` and `SHARD_PREFIX="TestIntegrationBashUpgrade"`. Keep `# docket-suite: go` on line 2 and the canonical `assert` line byte-identical. Add `tests/test_go_integration_bashupgrade.sh<TAB>60<TAB>parallel` to `tests/runtime-budgets.tsv` next to the other `test_go_integration_*` rows. The ceiling is provisional and is re-measured in Task 7.

- [ ] **Step 7: Run the probe and decide**

Run: `go test -tags integration -count=1 -run '^TestIntegrationBashUpgradeOwnership' ./internal/bashupgrade/ -v`

- **PASS**: continue. Record the full `prepare`/`check` finding lists for v0.9.2 in your task report. Task 5 uses them.
- **FAIL with `BLOCKED:`**: first confirm the case is faithful. Re-check that `origin/docket`'s root tree is the empty tree (`git -C <origin> rev-list --max-parents=0 docket` then `git cat-file -p <root>`), that `main` and `docket` share no ancestry, and that the restore replayed `clone-config.txt`. If the case is faithful, stop the build and **return BLOCKED** with the probe output, the root commit id and tree, the `origin` ref list, and the matching `verifyMetadataOwnership` branch. Do not commit a workaround, do not change product code, and do not continue to Task 3.
- **FAIL for any other reason** (bundle restore, JSON decode): fix the test, not the binary.

- [ ] **Step 8: Run the shard runner and the contract**

Run: `bash tests/test_go_integration_bashupgrade.sh && bash tests/test_go_integration_contract.sh && go test -count=1 ./internal/repoguard/ -run 'TestRuntimeBudgetsCorrespondence|TestRealProcessPackagesUseFixtureTempDir|TestMkdirTempViolations|TestNoExecutableBacktickInSuiteSource'`
Expected: all `ok`.

- [ ] **Step 9: Commit**

```bash
git add internal/bashupgrade/doc.go internal/bashupgrade/restore_test.go internal/bashupgrade/restore_unit_test.go internal/bashupgrade/bashupgrade_integration_test.go tests/test_go_integration_bashupgrade.sh tests/runtime-budgets.tsv
git commit -m "test(bash-upgrade): restore saved Bash cases and probe metadata ownership"
```

---

### Task 3: Build the saved v0.9.3 Bash state (migrated repo, deliberate config) and re-probe

**Files:**
- Create: `testdata/bash-upgrade/v0.9.3/origin.bundle`, `home.tar`, `clone-config.txt`, `records.txt`, `PROVENANCE.md`
- Modify: `testdata/bash-upgrade/PROVENANCE.md` (add the v0.9.3 pointer line)

**Interfaces:**
- Consumes: Task 1's procedure and formats, and Task 2's `TestIntegrationBashUpgradeOwnership`, which picks up the new tag through `savedTags`.
- Produces: the v0.9.3 case. Its `.docket.yml` on `main` sets every then-valid key that Go now refuses or warns about, and `PROVENANCE.md` lists exactly which.

- [ ] **Step 1: Repeat Task 1 Steps 1–3 for `v0.9.3`**

Use a fresh sandbox (`bash-upgrade-v093.XXXXXX`) and `git clone --branch v0.9.3`. Expect commit `dd742abd5e9fcdf8ffe78eb6f36a293410873bbf`. Re-read the v0.9.3 `link-skills.sh`/`sync-agents.sh` harness lists, since they may differ from v0.9.2.

- [ ] **Step 2: Bootstrap through `migrate-to-docket.sh` (the seeded-root path)**

Create the sample repository in the tag's single-branch layout first: the planning surface on `main`, populated through the same tag flows Task 1 Step 5 lists, in single-branch mode. Read `migrate-to-docket.sh`'s header for its preconditions. Then:

```bash
"$SB/sbx" sh -c 'cd "$HOME/dev/sample" && /opt/homebrew/bin/bash "$HOME/dev/docket/migrate-to-docket.sh" --yes' 2>&1 | tee "$SB/migrate.log"
```

After migration, add the rest of the minimum content on the `docket` branch through the tag's docket-mode flows, so both branches carry history. Each category from Task 1 Step 5 must exist at the end. The seeded root, rather than an empty tree, is the point of this case. Record the root commit and its tree in the provenance note.

- [ ] **Step 3: Commit the deliberate config last, and prove the tag accepted it**

Read v0.9.3's own schema (`scripts/docket-config.sh`, `.docket.example.yml`, `scripts/docket-config.md` and `skills/docket-convention`) and intersect it with the spec's configuration table: `metadata_branch`, `runtime.bash`, `terminal_publish: true`, `auto_groom`, `build.checkpoint`, `finalize.skip_results_only_delta: true`, `auto_capture.enabled: true`, `dummy_mode.enabled: true`, `finalize.gate: ci` (or `both`), `board_surfaces` containing `github`, `skills.<role>`, `agents.*.*.runner`, `agents.*.*.model`/`effort` in the repository layer, `learnings.cap`, `github_project`, `delegation_observation_budget` and the companion keys. Set each key the tag accepts, switched **on** where Go blocks only when on. Commit and push it as the **final** commit on `main`, after all population, so no Bash flow reaches `gh` because of it. Then run `docket-config.sh --export` (read-only) through `sbx` and save the output. It must exit 0, which proves the config was valid at v0.9.3. `PROVENANCE.md` lists every key set, and every table key the tag did not accept along with the tag's own error.

- [ ] **Step 4: Export, write `PROVENANCE.md`, check the size**

Follow Task 1 Steps 7–8 exactly, with paths under `testdata/bash-upgrade/v0.9.3`.

- [ ] **Step 5: Re-run the ownership probe on both tags**

Run: `go test -tags integration -count=1 -run '^TestIntegrationBashUpgradeOwnership' ./internal/bashupgrade/ -v`
Expected: both `v0.9.2` and `v0.9.3` subtests appear and PASS. Config-driven *findings* on v0.9.3 are fine here, because the probe only rejects ownership refusals. A `BLOCKED:` failure on v0.9.3 is handled exactly as Task 2 Step 7 says. Check fidelity first (the seeded root must equal the `{changes, adrs, specs}` projection of the pre-migration `main` commit, per `verifyLegacyEquivalence`). Then **return BLOCKED** with the evidence.

- [ ] **Step 6: Commit**

```bash
git add testdata/bash-upgrade/PROVENANCE.md testdata/bash-upgrade/v0.9.3
git commit -m "test(bash-upgrade): saved v0.9.3 Bash install and migrated repository with legacy config"
```

---

### Task 4: Guide step extraction and its guards

**Files:**
- Create: `internal/bashupgrade/steps_test.go` (untagged: parser and guard helpers)
- Create: `internal/bashupgrade/steps_unit_test.go` (default tag)

**Interfaces:**
- Produces:
  - `type guideStep struct { Name, Body string; Line int }`
  - `type guideDoc struct { Steps []guideStep; Unmarked []guideFence }`, where `type guideFence struct { Body string; Line int }`
  - `func parseGuide(src string) (guideDoc, error)`
  - `func docketCommandLines(body string) []string`: the lines whose first shell word (after trimming leading whitespace and an optional `$ ` prompt) is exactly `docket`
  - `func substitutePlaceholders(body string, repl map[string]string) (string, error)`: replaces each `<key>` from `repl` and errors on any remaining `<[a-z-]+>` token
  - `const stepMarkerPrefix = "<!-- upgrade-step: "`

Grammar: a marker line is exactly `<!-- upgrade-step: <name> -->`, with `name` matching `^[a-z][a-z0-9-]*$`. The next non-blank line must open a fence (three or more backticks, optional info string). The fence closes at the first line consisting of the same backtick run. Errors: a marker not followed by a fence; a malformed marker (a line starting with `<!-- upgrade-step:` that does not match); a duplicate name; an unterminated fence. Every fence that is not directly marked goes to `Unmarked`. Fences nested inside fences do not exist in this grammar; the closing rule handles them.

- [ ] **Step 1: Write the failing tests**

`internal/bashupgrade/steps_unit_test.go`:

````go
package bashupgrade

import (
	"reflect"
	"strings"
	"testing"
)

const sampleGuide = "# Guide\n\n<!-- upgrade-step: repo-prepare -->\n\n```sh\ncd <repo>\ndocket repository prepare\n```\n\nProse.\n\n```sh\ncurl -fsSLO https://example.invalid/docket/install.sh\n```\n\n<!-- upgrade-step: repo-check -->\n```sh\n$ docket repository check\n```\n"

func TestParseGuideOrderAndFences(t *testing.T) {
	g, err := parseGuide(sampleGuide)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, s := range g.Steps {
		names = append(names, s.Name)
	}
	if !reflect.DeepEqual(names, []string{"repo-prepare", "repo-check"}) {
		t.Fatalf("steps = %v", names)
	}
	if g.Steps[0].Body != "cd <repo>\ndocket repository prepare\n" {
		t.Fatalf("body = %q", g.Steps[0].Body)
	}
	if len(g.Unmarked) != 1 || !strings.Contains(g.Unmarked[0].Body, "curl") {
		t.Fatalf("unmarked = %#v", g.Unmarked)
	}
}

func TestParseGuideRejects(t *testing.T) {
	for name, src := range map[string]string{
		"marker without fence": "<!-- upgrade-step: a -->\nprose\n",
		"malformed marker":     "<!-- upgrade-step: Bad_Name -->\n```sh\nx\n```\n",
		"duplicate":            "<!-- upgrade-step: a -->\n```\nx\n```\n<!-- upgrade-step: a -->\n```\ny\n```\n",
		"unterminated":         "<!-- upgrade-step: a -->\n```\nx\n",
	} {
		if _, err := parseGuide(src); err == nil {
			t.Errorf("%s: parseGuide accepted it", name)
		}
	}
}

func TestDocketCommandLines(t *testing.T) {
	got := docketCommandLines("cd <repo>\n  docket version\n$ docket install check\n# docket in a comment\ncurl https://x/docket/install.sh\ndocketx run\n")
	want := []string{"docket version", "docket install check"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
}

func TestSubstitutePlaceholders(t *testing.T) {
	out, err := substitutePlaceholders("cd <repo>\n", map[string]string{"repo": "/r"})
	if err != nil || out != "cd /r\n" {
		t.Fatalf("got %q, %v", out, err)
	}
	if _, err := substitutePlaceholders("cd <other>\n", map[string]string{"repo": "/r"}); err == nil {
		t.Fatal("an unknown placeholder was accepted")
	}
}
````

- [ ] **Step 2: Run them and confirm they fail**

Run: `go test -count=1 -run 'TestParseGuide|TestDocketCommandLines|TestSubstitutePlaceholders' ./internal/bashupgrade/`
Expected: FAIL (`undefined: parseGuide`, and so on).

- [ ] **Step 3: Implement `steps_test.go`**

Implement the grammar above with a line scanner (`strings.Split(src, "\n")`). Use `regexp.MustCompile` inside the functions, not at package level. `docketCommandLines` skips lines whose first non-space character is `#`, strips a leading `$ `, and takes `strings.Fields(line)[0] == "docket"`.

- [ ] **Step 4: Run them and confirm they pass**

Run: `go test -count=1 -run 'TestParseGuide|TestDocketCommandLines|TestSubstitutePlaceholders' ./internal/bashupgrade/`
Expected: PASS.

- [ ] **Step 5: Mutation-check the parser's guards**

Back up `steps_test.go` with `cp`. Remove the duplicate-name check and confirm `TestParseGuideRejects/duplicate` goes red. Restore the backup with `mv -f`. Do the same for the marker-without-fence check. Record both readings in the task report.

- [ ] **Step 6: Commit**

```bash
git add internal/bashupgrade/steps_test.go internal/bashupgrade/steps_unit_test.go
git commit -m "test(bash-upgrade): parse marked upgrade steps out of the guide"
```

---

### Task 5: Write the guide and drive it end to end on both saved cases

**Files:**
- Create: `docs/release/upgrading-from-bash.md`
- Create: `internal/bashupgrade/registry_test.go` (untagged: the step registry and its actions)
- Modify: `internal/bashupgrade/bashupgrade_integration_test.go` (add `TestIntegrationBashUpgradeGuide`, `TestIntegrationBashUpgradeGuideShape`)

**Interfaces:**
- Consumes: `restoreCase`, `upgradeCase.run`, `findingCodes`, `readRecords`, `listRecords`, `parseGuide`, `docketCommandLines` and `substitutePlaceholders`.
- Produces:
  - `type stepAction func(t *testing.T, c *upgradeCase, st *runState, body string)`, where `type runState struct { InstallAttempts []cmdResult; Conflicts []string; Observed map[string][]findingRef }`
  - `var stepRegistry = map[string]stepAction{…}`
  - `func guidePath(t *testing.T) string`: returns `$DOCKET_BASH_UPGRADE_GUIDE` when set (the mutation-check seam, used only by hand in Task 6), else `<repoRoot>/docs/release/upgrading-from-bash.md`
  - `func runGuide(t *testing.T, c *upgradeCase) *runState` and `func readGuide(t *testing.T) string` (both in `registry_test.go`)

The guide's outline is fixed by spec section 3 (sections 1–9 in order). Its marked steps start as the list below. Correct it from observation under the truth rule. Adding a step is allowed only when the binary needs it *and* it is not a `docket`-branch hand edit or a refusal bypass; that case is BLOCKED, per Global Constraints.

| Marker name | Guide section | Action kind | What the action does |
|---|---|---|---|
| `install-binary` | 3 | mirror | Copies `c.Docket` to `$BinDir/.docket-stage`, runs `<stage> install --harness claude` (the downloader's exact hand-off), then `mv -f`s it to `$BinDir/docket` only on exit 0, mirroring `internal/release/downloader/install.sh`. Records the attempt. On failure it collects the conflict paths named in the output into `st.Conflicts`. The guide block holds the real downloader commands: fetch `install.sh` and `checksums.txt`, verify with `shasum -a 256 -c`, run `sh install.sh --harness claude`. The mirror asserts the block contains `--harness claude` and a checksum-verify line. |
| `confirm-install` | 3 | run | Runs the block (`docket version`, `docket install check`). Expects exit 0. |
| `takeover-remedy` | 4 | mirror | Applies only when `st.Conflicts` is non-empty. Asserts every conflict path is covered by the remedy the block names (the `~/.claude/agents/docket-*` files, plus anything else observed and written into the guide). Deletes exactly the named paths, then re-runs the `install-binary` mirror. The block says to re-run the same download command (Review Focus 2). |
| `repo-prepare` | 5 | run | `cd <repo>` then `docket repository prepare`. Expects exit 0. |
| `repo-check` | 5 | observe | Runs `docket repository check --json`. Records the codes in `st.Observed["repo-check"]`. Asserts each observed code appears in the guide's finding-to-fix table. |
| `config-cleanup` | 5/6 | mirror | Removes from `.docket.yml` every key the settings table marks as blocking or warning that is present in this case (v0.9.3: the deliberate keys; v0.9.2: whatever the bootstrap left, e.g. `metadata_branch`). Asserts each removed key has a row in the settings table. |
| `configure-tests` | 5 | run | `docket repository configure-tests`. |
| `commit-config` | 5 | run | `git add .docket.yml && git commit -m "…" && git push` on the default branch (block as written in the guide). Skipped with a recorded note when there is nothing to commit. |
| `repair-preview` | 5 | run | `docket repository repair`. |
| `repair-apply` | 5 | run | `docket repository repair --yes`. |
| `leftovers` | 7 | mirror | Deletes the per-repository agent wrappers and the other leftovers the guide lists. First asserts each listed path exists (it is a real leftover) and is not named by `docket install check`. |

`<repo>` is the only placeholder (`substitutePlaceholders` with `{"repo": c.Clone}`). `~` expands through `HOME` because run blocks execute with `/bin/sh -e -c` in `c.Env` and `Dir: c.Clone`.

- [ ] **Step 1: Draft the guide**

Write `docs/release/upgrading-from-bash.md` with spec section 3's nine sections, in order, in plain user prose:

1. **Who this is for.** A Bash docket v0.9.2 or v0.9.3 install with repositories on the `docket` branch. Single-branch repositories are not covered. The guide covers Claude Code only: Cursor and OpenCode sections arrive in later pre-releases, and Codex is not supported.
2. **Before you start.** Finish or defer every in-flight change.
3. **Install the Go binary.** The release download with checksum verification (marked `install-binary`), then `confirm-install`. One sentence pointing contributors at `docs/install/install.md` for the checkout route, which needs Go and is not an upgrade step.
4. **Take over the old Claude Code install** (`takeover-remedy`).
5. **Upgrade each repository** (`repo-prepare`, `repo-check` with a finding-to-fix table, `config-cleanup`, `configure-tests`, `commit-config`, `repair-preview`, `repair-apply`).
6. **Settings that changed** (table: setting, what Go does, the fix).
7. **Leftovers you can delete** (`leftovers`).
8. **Restart Claude Code** (a new process; clearing a conversation is not enough).
9. **If something goes wrong.** Where to report, plus exactly one line saying the Bash tags `v0.9.2` and `v0.9.3` remain available, with no rollback steps.

Seed the finding table and the settings table from the spec's tables. Task 5 Step 5 reconciles them against observation. Every fenced block that contains a `docket` command carries a marker.

- [ ] **Step 2: Write the guide-shape test (default-runnable assertions, integration-tagged file)**

Add to `bashupgrade_integration_test.go`:

```go
func TestIntegrationBashUpgradeGuideShape(t *testing.T) {
	src := readGuide(t)
	g, err := parseGuide(src)
	if err != nil {
		t.Fatal(err)
	}
	marked := map[string]bool{}
	for _, s := range g.Steps {
		marked[s.Name] = true
		if _, ok := stepRegistry[s.Name]; !ok {
			t.Errorf("guide step %q (line %d) is neither executed nor mirrored by the test", s.Name, s.Line)
		}
	}
	for name := range stepRegistry {
		if !marked[name] {
			t.Errorf("test registry step %q has no marked block in the guide", name)
		}
	}
	for _, f := range g.Unmarked {
		if cmds := docketCommandLines(f.Body); len(cmds) > 0 {
			t.Errorf("unmarked fenced block at line %d runs docket commands %q; mark it as an upgrade step", f.Line, cmds)
		}
	}
	if len(g.Steps) < len(stepRegistry) || len(stepRegistry) == 0 {
		t.Fatalf("population floor: %d marked steps, %d registry entries", len(g.Steps), len(stepRegistry))
	}
}
```

`readGuide(t)` reads `guidePath(t)`. Both directions of the marker↔registry correspondence are asserted (learning *correspondence-guard-runs-one-way*).

- [ ] **Step 3: Write the end-to-end test**

```go
func TestIntegrationBashUpgradeGuide(t *testing.T) {
	for _, tag := range savedTags(t) {
		t.Run(tag, func(t *testing.T) {
			c := restoreCase(t, tag)
			before := listRecords(t, c)
			st := runGuide(t, c)

			wantConflict := tag == "v0.9.3"
			if gotConflict := len(st.Conflicts) > 0; gotConflict != wantConflict {
				t.Fatalf("first install conflict = %v, want %v (attempts: %+v)", gotConflict, wantConflict, st.InstallAttempts)
			}
			if wantConflict && len(st.InstallAttempts) < 2 {
				t.Fatalf("v0.9.3: the remedy did not lead to a second install run")
			}
			assertCleanEndState(t, c)
			assertRecordsSurvive(t, c, before)
			assertWritable(t, c)
		})
	}
}
```

`runGuide` parses the guide and runs `stepRegistry[name]` for each step in guide order, passing the placeholder-substituted body. `run` actions fail the test on a non-zero exit and print the block, stdout and stderr.

`assertCleanEndState`:
- `docket install check` exits 0, and its JSON findings for the `claude` harness hold no `error`/`warning`.
- `docket repository check --json` exits 0 with no `error`/`warning` findings.
- `docket status --json` has zero `severity == "error"` findings.

Confirm the `--json` support of each command from `--help` first.

`assertRecordsSurvive(t, c, before)`: after `git -C c.Clone fetch origin`, every entry of `before` (which Task 2 proved equal to `records.txt`) still exists on its branch at `origin`. A change record may have moved from `active/` to `archive/` only if it was already archived, which it was not, so require exact path presence. Content changes made by `repository repair`'s frontmatter fixes are allowed.

`assertWritable`: write a `change.create` request JSON (`request_id`, `title`, `why`, `what_changes`, `out_of_scope`; check the field names against `app.ChangeCreateRequest`) into `c.Root`. Run `docket change create --request <file>` from `c.Clone`, expecting exit 0. Then assert the new title appears in `BOARD.md` on `origin/docket` (`git show origin/docket:<board path>`).

- [ ] **Step 4: Implement `registry_test.go`**

Implement every action in the table above. Rules that keep the actions honest:
- **Run actions** execute only the guide's block text (`/bin/sh -e -c <body>`). They never add commands the guide does not show.
- **Mirror actions** do exactly what the guide's prose and block instruct, and assert the guide text contains the instruction they mirror, bound to its object: the path pattern, key name, or command. For example, `takeover-remedy` asserts the block mentions `~/.claude/agents/` and every observed conflict path matches a pattern the block names.
- Parse conflict paths from the install output. Prefer `--json` if `docket install` supports it (`reason: ownership-conflict` entries); otherwise parse the human output by the path shape the binary prints, and assert the parse found at least one path whenever exit ≠ 0.
- `config-cleanup` edits `.docket.yml` through a small line-based remover keyed on top-level and nested key paths. Assert the file still parses afterwards with `docket repository check` (no `config-invalid`-class finding).

- [ ] **Step 5: Run, observe, and reconcile the guide with what the binary does**

Run: `go test -tags integration -count=1 -run '^TestIntegrationBashUpgradeGuide' ./internal/bashupgrade/ -v 2>&1 | tee "$SCRATCH/guide-run.log"`

Iterate until green, and change only the guide and the registry. On each red result, classify it:
- **The guide is wrong or incomplete** (a finding not in the table, a conflict path not named, a step out of order): fix the guide text and table, and the mirror, from the observed output. This is the truth rule at work.
- **The binary misbehaves and the guide cannot honestly explain it**: stop and **return BLOCKED** with the evidence (spec section 4).

Finally, delete from the guide every claim the run did not observe and 0366 does not prove, or mark it explicitly as not covered.

- [ ] **Step 6: Run the shape test and the docs guards**

Run: `go test -tags integration -count=1 -run '^TestIntegrationBashUpgradeGuideShape' ./internal/bashupgrade/ && go test -count=1 ./internal/repoguard/ -run 'TestLivingDocsAlignment|TestCommentAnchorStyle'`
Expected: PASS. `docs/release/` is outside `livingDocRoots`, so refused setting names in the guide are allowed.

- [ ] **Step 7: Humanize and commit**

Run the `humanizer` skill over the guide's prose. Keep marker lines, fenced blocks and table cells byte-identical, then re-run Step 5's command, which must still pass.

```bash
git add docs/release/upgrading-from-bash.md internal/bashupgrade/registry_test.go internal/bashupgrade/bashupgrade_integration_test.go
git commit -m "docs(release): upgrade guide from Bash docket, proven on saved v0.9.2 and v0.9.3 installs"
```

---

### Task 6: Mutation checks, links from README and the install doc

**Files:**
- Modify: `README.md` (*Install and the five steps* section: one sentence linking `docs/release/upgrading-from-bash.md` for Bash docket users)
- Modify: `docs/install/install.md` (*Adopting docket in a repository* section: one sentence linking the guide for repositories already on the `docket` branch from a Bash install)

**Interfaces:**
- Consumes: `guidePath`'s `DOCKET_BASH_UPGRADE_GUIDE` seam, `stepRegistry`, and the `TestIntegrationBashUpgrade*` tests.
- Produces: mutation evidence for the results file (the build's evidence record), in the task report.

- [ ] **Step 1: Mutation 1, dropping a marked step**

Two readings are required. Use a guide copy for both, and never edit the committed guide.

```bash
G=docs/release/upgrading-from-bash.md
cp "$G" "$SCRATCH/guide-drop.md"
# delete the configure-tests marker line and its fenced block from the copy (by hand or with a scratch script)
cmp -s "$G" "$SCRATCH/guide-drop.md" && echo "MUTATION DID NOT LAND"
DOCKET_BASH_UPGRADE_GUIDE="$SCRATCH/guide-drop.md" go test -tags integration -count=1 -run '^TestIntegrationBashUpgradeGuide' ./internal/bashupgrade/
```

Expected: FAIL. `GuideShape` names `configure-tests` as a registry step with no marked block.

Second reading: also remove `configure-tests` from `stepRegistry`. Back up `registry_test.go` with `cp` first, and restore it with `mv -f` afterwards. Re-run. Expected: `TestIntegrationBashUpgradeGuide` fails in `assertCleanEndState` with `test-config-missing`, which proves the step is load-bearing and not only listed. Record both outputs.

- [ ] **Step 2: Mutation 2, dropping the v0.9.3 remedy**

Copy the guide, remove the `takeover-remedy` marker and block from the copy, back up `registry_test.go`, delete the `takeover-remedy` registry entry, and run:

```bash
DOCKET_BASH_UPGRADE_GUIDE="$SCRATCH/guide-noremedy.md" go test -tags integration -count=1 -run '^TestIntegrationBashUpgradeGuide/v0.9.3' ./internal/bashupgrade/ -v
```

Expected: FAIL on the v0.9.3 case. The install never succeeds, and `confirm-install` or the end state goes red. Restore with `mv -f`, confirm `git status --short internal/bashupgrade` is clean, and record the output.

- [ ] **Step 3: Add the two links**

In `README.md`, in *Install and the five steps*, add: `Upgrading from the Bash version of docket? Follow [Upgrading from Bash docket](docs/release/upgrading-from-bash.md).`

In `docs/install/install.md`, in *Adopting docket in a repository*, add: `If the repository already has a docket branch from the Bash version of docket, follow [Upgrading from Bash docket](../release/upgrading-from-bash.md) instead.`

Neither sentence names a setting or a change number.

- [ ] **Step 4: Run the docs guards**

Run: `go test -count=1 ./internal/repoguard/ -run 'TestLivingDocsAlignment|TestInstallPrerequisiteDocContracts'`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add README.md docs/install/install.md
git commit -m "docs: link the Bash docket upgrade guide from README and the install doc"
```

---

### Task 7: Budget measurement and the whole-suite gate

**Files:**
- Modify: `tests/runtime-budgets.tsv` (only if the measured ceiling needs to change)

- [ ] **Step 1: Measure the shard serially**

```bash
/usr/bin/time -p bash tests/test_go_integration_bashupgrade.sh
```

Run it twice. Set the ceiling to about twice the slower `real` value, rounded up to a multiple of 5. Record both measurements and the chosen ceiling in the task report as numbers (learning *budget-headroom-is-spent-before-it-is-breached*). If the shard exceeds 120s serially, return NEEDS_ESCALATION with the timings rather than pinning `serial` or splitting the tags into separate shards on your own judgment.

- [ ] **Step 2: Run the whole suite through the configured build test command**

Read `build.test_command` from the feature worktree's `.docket.yml` at run time (at plan time it resolves to `go run ./cmd/docket development test`; re-read it, never a second copy) and run it from the feature worktree. Expected: exit 0. Read every `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` / `SERIAL CONFIRMED OVER BUDGET:` line and put them in the task report.

- [ ] **Step 3: Commit any budget change**

```bash
git add tests/runtime-budgets.tsv
git commit -m "test(bash-upgrade): set the measured shard budget"
```

Skip this step when the row did not change.

---

## Results-file notes for the finalizer

The build's results record must carry:
- the test prefix `TestIntegrationBashUpgrade`, used in place of the spec's `TestBashUpgrade` (contract rule (2)), so 0366's whole-suite gate names it;
- the per-tag `du -sk` sizes;
- the v0.9.2 and v0.9.3 bootstrap kinds (empty-orphan vs `migrate-to-docket.sh` seed) and the ownership proof each exercised;
- both mutation readings from Task 6, with output excerpts;
- the observed finding sets per tag, and any guide claim marked *not covered*;
- a follow-up note that a deferred retirement stub (delete `testdata/bash-upgrade/`, `internal/bashupgrade/`, the shard and its budget row when stable v1.0.0 ships) is needed if it does not already exist.
