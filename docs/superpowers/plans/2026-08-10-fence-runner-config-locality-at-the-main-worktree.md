<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0270 — Fence runner-config locality at the main worktree (regression test + contract correction)](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0270-fence-runner-config-locality-at-the-main-worktree.md)**
<!-- docket:backlink:end -->

# Fence runner-config locality at the main worktree — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fence the already-correct invariant that `runner-dispatch.sh` resolves `runners.<name>:`
config at the **main worktree** while anchoring the run at `--worktree`, with one regression test,
and correct the two contract documents whose wording invited the opposite reading.

**Architecture:** Two independent deliverables, no production code changes. Task 1 adds one section
to `tests/test_runner_dispatch.sh` that crosses the two axes the existing suite tests separately —
a **real linked git worktree**, a grant written only to the main worktree, a dispatch issued from
inside the worktree with `--worktree` set — plus the two mandatory mutation probes that prove the
new asserts are load-bearing. Task 2 rewrites the two prose passages (`scripts/runner-dispatch.md`
step 3, `scripts/runners/opencode.md`'s env row and Prerequisites bullet) so they state which tree
the config is read from and why.

**Tech Stack:** Bash 4+ test scripts, the repo's own `assert` harness, `git worktree`, the existing
fake-codex argv-recording fixture (`make_fixture`).

## Global Constraints

- **No production code changes.** `scripts/runner-dispatch.sh` and `scripts/runners/*.sh` are
  read-only in this change. If a mutation probe fails to redden the expected assert, the **test** is
  wrong — fix the test, never the script.
- **Diff scope is tight.** Changes 0208 and 0277 are queued against the same two files. Touch only
  what this plan names; no opportunistic edits to the `--worktree` gates, the existing `mkdir`
  fixtures in the change-0206 block, or the `runners:` config reader.
- **Cross-references anchor on symbol names, never line numbers** (AGENTS.md). The prose must say
  `docket_main_worktree()`, not `runner-dispatch.sh:116`.
- **Mutation restore is a `cp` backup**, never `git checkout -- <file>`: the latter restores to HEAD
  and would silently discard the uncommitted work under test.
- **`grep` for a pattern leading with `--` must declare it** (`grep -qxF -- "…"`), and never pipe a
  producer into an early-exiting consumer under `pipefail` — use the here-string form the file
  already uses.
- Test file budget: `tests/test_runner_dispatch.sh` has a **10s** row in
  `tests/runtime-budgets.tsv`; measured baseline before this change is **5.79s** serial.
- Suite command: `scripts/run-tests.sh` (the resolved `finalize.test_command`).

---

## File Structure

| File | Responsibility in this change |
|---|---|
| `tests/test_runner_dispatch.sh` | Modify. Gains one new section (`0270`) between the change-0206 `--worktree` block and the `facade: runners.<name> config resolution across layers` block. Nothing else in the file moves. |
| `scripts/runner-dispatch.md` | Modify. Step 3 of `## Behavior` — bind the config tree to the main worktree, state the `--worktree` independence and its gitignore reason. |
| `scripts/runners/opencode.md` | Modify. Two wording touches: the `DOCKET_RUNNER_CFG_PERMISSIONS` env-table row and the `runners.opencode.permissions` Prerequisites bullet. |
| `scripts/runner-dispatch.sh` | **Read-only.** Mutated temporarily during Task 1's probes and restored from a `cp` backup. It must be byte-identical to `origin/main` at commit time. |

---

### Task 1: The regression test — anchor × config locality, with mutation probes

**Files:**
- Modify: `tests/test_runner_dispatch.sh` — insert a new section immediately after the
  change-0206 block's trailing `rm -rf "$SBX"` and immediately before the line
  `# ---- facade: runners.<name> config resolution across layers ------------------------`
- Read-only, mutated and restored: `scripts/runner-dispatch.sh`

**Interfaces:**
- Consumes: the file's existing fixture helpers — `make_fixture` (sets `SBX` = a real git repo with
  one commit, `BIN` = fake-bin dir holding the argv-recording fake `codex`, `LOG` = the argv log,
  `MSG`), the `assert <description> <shell-expression>` helper, and the module-level `FACADE`
  (`scripts/runner-dispatch.sh`).
- Produces: nothing consumed by a later task. Task 2 is independent.

**Why this file and not `tests/test_runner_opencode.sh`:** the provenance is an opencode
`permissions` grant, but the config loop lives in the **facade** and is runner-agnostic — it knows
no runner's key names. `test_runner_opencode.sh` exercises the adapter in isolation and never runs
the facade, so it cannot test this at all. The section comment carries the opencode provenance so a
reader searching for the permissions story lands here.

**Why a real linked worktree and not `mkdir -p`:** with a plain subdirectory,
`docket_main_worktree "$ANCHOR"` trivially returns `$SBX` because the subdirectory *is* part of the
main worktree — the resolution under test never happens and every assert is vacuous. A real linked
worktree is the only fixture that exercises `git worktree list --porcelain`'s "main worktree is
listed first, from every worktree in the set" behavior in `scripts/lib/docket-root.sh`, which is
what actually makes the invariant true.

- [ ] **Step 1: Write the failing test**

Open `tests/test_runner_dispatch.sh`. Find the change-0206 block's end — the two consecutive lines:

```bash
rm -rf "$OUTSIDE"
rm -rf "$SBX"
```

immediately followed by the blank line and then
`# ---- facade: runners.<name> config resolution across layers ------------------------`.

Insert the following section between that `rm -rf "$SBX"` and the
`# ---- facade: runners.<name> config resolution across layers` comment, separated by one blank
line on each side:

```bash
# ---- 0270: config locality — a MAIN-worktree grant survives a --worktree dispatch ----
# Provenance (opencode): filed as "a machine-local `runners.opencode.permissions: auto-approve`
# grant is invisible to a build-* delegation". It never was. The facade resolves runners.<name>.*
# at docket_main_worktree() and anchors the RUN at --worktree; the two trees are deliberately
# DECOUPLED, and the decoupling is load-bearing because .docket.local.yml is gitignored — a
# feature worktree carries no copy of it, so an anchor-relative read would drop every
# machine-local grant on exactly the build-* dispatches that require --worktree.
# Tested here, at the FACADE, because the config loop is runner-agnostic (it knows no runner's key
# names); tests/test_runner_opencode.sh drives the adapter in isolation and never runs the facade.
#
# The fixture MUST be a real linked worktree. With a bare `mkdir -p` subdirectory,
# docket_main_worktree "$ANCHOR" trivially returns $SBX because the subdirectory IS part of the
# main worktree, and the resolution under test never happens — every assert below goes vacuous.
make_fixture
git -C "$SBX" worktree add -q -b featslug "$SBX/.worktrees/featslug" >/dev/null 2>&1
WT="$SBX/.worktrees/featslug"
assert "0270: fixture is a REAL linked worktree, not a subdirectory" '[ -f "$WT/.git" ]'

# Mirror production: the machine-local layer is gitignored, and the grant is written to the main
# worktree ONLY — after the worktree exists, so it can never be copied into it. The .gitignore
# line is documentation inside the fixture: it shows WHY a feature worktree lacks the file.
printf '.docket.local.yml\n' > "$SBX/.gitignore"
printf 'runners:\n  codex:\n    sandbox: danger-full-access\n' > "$SBX/.docket.local.yml"
assert "0270: the grant exists ONLY in the main worktree" '[ ! -e "$WT/.docket.local.yml" ]'

# cwd INSIDE the linked worktree is the production shape (a build worker dispatches from its own
# tree) and is the condition under which a cwd-derived config root would read the wrong tree.
: > "$LOG"
( cd "$WT" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent status \
    --worktree "$WT" >/dev/null 2>&1 )
argv="$(cat "$LOG")"
assert "0270: main-worktree grant reaches the child across a --worktree dispatch" \
  'grep -qxF -- "danger-full-access" <<<"$argv"'
# Anti-vacuity pair. Without these, a regression that anchored the config loop at $ANCHOR *and*
# let the anchor fall back to the main worktree would leave the assert above green.
assert "0270: the anchor handed to the adapter IS the linked worktree" \
  'grep -qxF -- "$WT" <<<"$argv"'
assert "0270: the anchor is NOT the main worktree" '! grep -qxF -- "$SBX" <<<"$argv"'
rm -rf "$SBX"
```

- [ ] **Step 2: Run the file and verify the new asserts PASS on unmodified code**

Run: `bash tests/test_runner_dispatch.sh 2>&1 | grep -E "^(ok|NOT OK) - 0270"`

Expected: exactly five lines, all `ok -`:

```
ok - 0270: fixture is a REAL linked worktree, not a subdirectory
ok - 0270: the grant exists ONLY in the main worktree
ok - 0270: main-worktree grant reaches the child across a --worktree dispatch
ok - 0270: the anchor handed to the adapter IS the linked worktree
ok - 0270: the anchor is NOT the main worktree
```

This is the inverted order of a normal TDD cycle and it is deliberate: the behavior already exists,
so the test must pass first and the **mutation probes in Steps 3–4 are the red half of the cycle**.
A green reading here proves only that the asserts *can* pass; Steps 3 and 4 are what prove they are
load-bearing rather than decoration.

Also confirm the whole file is still green:

Run: `bash tests/test_runner_dispatch.sh 2>&1 | grep -c "^NOT OK"`
Expected: `0`

- [ ] **Step 3: Mutation probe 1 — anchor the config loop at `$ANCHOR`**

This probe proves assert (a) — "the grant reaches the child" — is load-bearing. Back up with `cp`,
never `git checkout --`: the file is unmodified here, but the idiom is unconditional so it stays
correct when a later step has uncommitted edits.

```bash
cp scripts/runner-dispatch.sh /tmp/rd.bak
```

Edit `scripts/runner-dispatch.sh`. Find the config-layer loop, whose head reads:

```bash
for f in "$REPO_ROOT/.docket.local.yml" "$REPO_ROOT/.docket.yml" "$GLOBAL_CFG"; do
```

Change both `$REPO_ROOT` prefixes on that one line to `$ANCHOR`:

```bash
for f in "$ANCHOR/.docket.local.yml" "$ANCHOR/.docket.yml" "$GLOBAL_CFG"; do
```

Run: `bash tests/test_runner_dispatch.sh 2>&1 | grep -E "^(ok|NOT OK) - 0270"`

Expected: the grant assert goes RED, the two anchor asserts stay green:

```
ok - 0270: fixture is a REAL linked worktree, not a subdirectory
ok - 0270: the grant exists ONLY in the main worktree
NOT OK - 0270: main-worktree grant reaches the child across a --worktree dispatch
ok - 0270: the anchor handed to the adapter IS the linked worktree
ok - 0270: the anchor is NOT the main worktree
```

**If the grant assert stays green,** something other than the config loop is supplying
`danger-full-access` — most likely the codex adapter's own default. That would make the assert
vacuous. Do not proceed: change the fixture's carrier to a value the adapter cannot default to
(any other legal `--sandbox` value, e.g. `workspace-write`, chosen so it differs from every default
in `scripts/runners/codex.sh`), re-run Step 2, and repeat this probe until it reddens.

Record the observed output verbatim — it goes in the results file.

Restore before the next probe:

```bash
cp /tmp/rd.bak scripts/runner-dispatch.sh
git diff --stat scripts/runner-dispatch.sh   # expected: no output
```

- [ ] **Step 4: Mutation probe 2 — export the main worktree as the run anchor**

This probe proves the anti-vacuity pair (asserts b and c) is load-bearing.

```bash
cp scripts/runner-dispatch.sh /tmp/rd.bak
```

Edit `scripts/runner-dispatch.sh`. Find:

```bash
export DOCKET_REPO_ROOT="$ANCHOR"
```

Change it to:

```bash
export DOCKET_REPO_ROOT="$REPO_ROOT"
```

Run: `bash tests/test_runner_dispatch.sh 2>&1 | grep -E "^(ok|NOT OK) - 0270"`

Expected: the grant assert stays green (the config loop is untouched) and **both** anchor asserts go
red:

```
ok - 0270: fixture is a REAL linked worktree, not a subdirectory
ok - 0270: the grant exists ONLY in the main worktree
ok - 0270: main-worktree grant reaches the child across a --worktree dispatch
NOT OK - 0270: the anchor handed to the adapter IS the linked worktree
NOT OK - 0270: the anchor is NOT the main worktree
```

Record the observed output verbatim for the results file.

Restore:

```bash
cp /tmp/rd.bak scripts/runner-dispatch.sh
git diff --stat scripts/runner-dispatch.sh   # expected: no output
rm -f /tmp/rd.bak
```

- [ ] **Step 5: Confirm the probes reddened only this section**

Each probe's blast radius should be confined to the new `0270` asserts plus, for probe 2 only,
whatever pre-existing anchor asserts legitimately depend on the mutated export. Re-read the two
recorded outputs from Steps 3 and 4 and note in the results file which non-`0270` asserts (if any)
also reddened, with a one-line reason each. Do **not** re-run the probes to collect this — the
Step 3/4 runs already printed the whole file's results; scroll them.

- [ ] **Step 6: Verify the script is byte-identical to its base**

Run: `git diff --exit-code scripts/runner-dispatch.sh && echo SCRIPT_CLEAN`
Expected: `SCRIPT_CLEAN` with no diff. This change ships **no** production code edit; a surviving
mutation is the single worst outcome available here.

- [ ] **Step 7: Measure the file's runtime against its budget**

A real `git worktree add` is slower than a `mkdir`, and this file's `tests/runtime-budgets.tsv` row
is **10s** against a measured pre-change baseline of **5.79s**.

Run: `time bash tests/test_runner_dispatch.sh > /dev/null 2>&1`

Expected: total wall clock comfortably under 10s. Record the number for the results file. If it is
at or over the budget, do **not** raise the budget row silently — report it, since a budget row is a
claim about serial cost that reviewers rely on.

- [ ] **Step 8: Commit**

```bash
git add tests/test_runner_dispatch.sh
git commit -m "test(0270): fence config locality — a main-worktree grant survives a --worktree dispatch"
```

---

### Task 2: Correct the two contracts that invited the misreading

**Files:**
- Modify: `scripts/runner-dispatch.md` — step 3 of the `## Behavior` list
- Modify: `scripts/runners/opencode.md` — the `DOCKET_RUNNER_CFG_PERMISSIONS` env-table row and the
  `runners.opencode.permissions` Prerequisites bullet

**Interfaces:**
- Consumes: nothing from Task 1. This task is independently reviewable and independently revertable.
- Produces: nothing consumed downstream.

Both edits are statements of **fact about which tree config is read from**. Do not restate the
facade's precedence rules inside `opencode.md` — `runner-dispatch.md` owns those.

- [ ] **Step 1: Rewrite `scripts/runner-dispatch.md` step 3's opening**

The current text opens the third item of the `## Behavior` list:

```markdown
3. **Resolve `runners.<name>:`** — per **key**, first layer that has the key wins, across
   `<repo>/.docket.local.yml` > `<repo>/.docket.yml` >
   `${XDG_CONFIG_HOME:-$HOME/.config}/docket/config.yml`. Each `key: value` scalar is exported
```

`<repo>` is never bound anywhere in this document, and step 2 immediately above has just defined the
anchor as possibly a feature worktree — so `<repo>` reads naturally as "the anchor". Replace those
first three lines with:

```markdown
3. **Resolve `runners.<name>:`** — per **key**, first layer that has the key wins, across the
   **main worktree's** `.docket.local.yml` > the **main worktree's** `.docket.yml` >
   `${XDG_CONFIG_HOME:-$HOME/.config}/docket/config.yml`. The config tree is the main worktree —
   the `docket_main_worktree()` result the facade binds before any argument-dependent anchoring —
   and it is **independent of `--worktree`**: the machine-local layer is gitignored, so a feature
   worktree carries no copy of it, and an anchor-relative read would silently drop every
   machine-local runner grant on exactly the `build-*` dispatches that *require* `--worktree`.
   Each `key: value` scalar is exported
```

Leave the rest of item 3 — from `as \`DOCKET_RUNNER_CFG_<KEY>\` (uppercased; …` onward, including
the value-parsing paragraph — byte-identical.

Constraint check before moving on: the replacement names `docket_main_worktree()` by **symbol**, and
contains no `file:line` reference. `tests/test_comment_anchor_style.sh` rejects the
filename-plus-line-number form.

- [ ] **Step 2: State the config tree in `scripts/runners/opencode.md`'s env table**

The env table's two relevant rows currently read:

```markdown
| `DOCKET_REPO_ROOT` | absolute run anchor — the main worktree unless the caller named a feature worktree; becomes `opencode run --dir` | required |
| `DOCKET_RUNNER_CFG_PERMISSIONS` | `runners.opencode.permissions` — `ask` \| `auto-approve` | `ask` |
```

The adjacency is what produced the misreading — the row above correctly qualifies
`DOCKET_REPO_ROOT`, and the row below introduces a config value with no statement of where it comes
from — so the correction belongs adjacent too. Leave the `DOCKET_REPO_ROOT` row untouched and
replace only the second row with:

```markdown
| `DOCKET_RUNNER_CFG_PERMISSIONS` | `runners.opencode.permissions` — `ask` \| `auto-approve`. Resolved by the facade from the **main worktree's** config layers, independently of the anchor above, so a `--worktree` delegation still sees a machine-local grant | `ask` |
```

- [ ] **Step 3: Name the tree in the Prerequisites bullet**

The last bullet of `## Prerequisites (documented, not automated)` currently reads:

```markdown
- `runners.opencode.permissions: auto-approve` in a config layer — without it every delegated run
  refuses by design.
```

This is the line a human follows when setting the grant up, and getting it wrong here is the exact
shape of the reported "defect". Replace it with:

```markdown
- `runners.opencode.permissions: auto-approve` in a config layer read at the **main worktree** —
  its `.docket.local.yml` or `.docket.yml`, or the global
  `${XDG_CONFIG_HOME:-$HOME/.config}/docket/config.yml`; never inside a feature worktree, which
  carries no copy of the gitignored machine-local layer. Without the grant every delegated run
  refuses by design.
```

- [ ] **Step 4: Verify no prose sentinel reddened**

Two suites grep these documents. `tests/test_runner_dispatch_build_gate.sh` asserts on
`runner-dispatch.md`'s *delegation execution posture* section — a different section from step 3,
and one of its asserts is a **count ceiling** (`six (required )?capabilities` at most once), so a
stray phrase would break it. `tests/test_runner_opencode.sh` asserts the contract doc exists.

Run: `bash tests/test_runner_dispatch_build_gate.sh 2>&1 | grep -c "^NOT OK"`
Expected: `0`

Run: `bash tests/test_runner_opencode.sh 2>&1 | grep -c "^NOT OK"`
Expected: `0`

Run: `bash tests/test_comment_anchor_style.sh 2>&1 | grep -c "^NOT OK"`
Expected: `0`

- [ ] **Step 5: Commit**

```bash
git add scripts/runner-dispatch.md scripts/runners/opencode.md
git commit -m "docs(0270): bind the runners config tree to the main worktree in both contracts"
```

---

### Task 3: Whole-suite gate

**Files:** none modified. This task only runs and reports.

**Interfaces:**
- Consumes: the committed output of Tasks 1 and 2.
- Produces: the green build evidence the review step reads.

- [ ] **Step 1: Run the whole suite**

Never only the files this plan named — the suite command is whatever `finalize.test_command`
resolves to, and it is `scripts/run-tests.sh`.

Run: `bash scripts/run-tests.sh`

Expected: every file passes.

- [ ] **Step 2: Read the budget line**

A trailing `OVER BUDGET:` line does **not** fail the run, so nothing else will catch it. If
`tests/test_runner_dispatch.sh` appears there, report it as a finding along with the Task 1 Step 7
serial measurement — a parallel-contention breach and a genuine cost increase are different
findings and the serial number is what distinguishes them.

- [ ] **Step 3: Confirm the production script is still untouched**

Run: `git diff origin/main --stat -- scripts/runner-dispatch.sh scripts/runners/ | grep -v '\.md' ; echo "EXIT_MARKER"`
Expected: only `EXIT_MARKER` — no `.sh` file appears in the branch's diff against its base.
