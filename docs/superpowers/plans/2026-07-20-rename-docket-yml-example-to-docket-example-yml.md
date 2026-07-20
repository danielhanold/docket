# Rename `.docket.yml.example` → `.docket.example.yml` Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rename docket's canonical config reference from `.docket.yml.example` to `.docket.example.yml` (and its guard `tests/test_docket_yml_example.sh` to `tests/test_docket_example_yml.sh`) so the filename ends in `.yml` and every YAML-aware editor and GitHub syntax-highlights it, then sweep every live reference to the new names.

**Architecture:** A `git mv` of two files plus a mechanical reference sweep across 14 live files. The file's *contents*, structure, and the three invariants ADR-0048 established are unchanged — this is a rename and nothing more. The sweep is **atomic**: five separate test files grep the example's path as a literal, so the moment the file is renamed they all redden simultaneously. There is no green intermediate state, and the plan does not pretend otherwise — the rename and the full sweep land in **one commit** (Task 2), bracketed by a baseline capture (Task 1) and a grep-clean + mutation proof (Task 3).

**Tech Stack:** POSIX shell, `git mv`, `sed`, the repo's hand-rolled `assert` test harness (`tests/test_*.sh`).

## Global Constraints

- **Rename + reference sweep only.** No change to the example file's contents, key set, ordering, structure, or scope tags; no change to ADR-0048's three invariants. If a task seems to require editing what the file *says*, stop — that is out of scope.
- **Two independent rename patterns, not one.** They do not overlap textually, so order between them is irrelevant:
  - **A.** `.docket.yml.example` → `.docket.example.yml` (the dotted path)
  - **B.** `test_docket_yml_example` → `test_docket_example_yml` (the underscored test name)
- **A third form exists and a plain literal sed CANNOT reach it.** `tests/test_docket_yml_example.sh:544` embeds the filename as an *escaped ERE* — `\.docket\.yml\.example` — inside a `sed -nE` program. Because backslashes sit between the segments, pattern A does not match this line. It is verified below that pass A leaves this line byte-identical. It gets its own explicit replacement.
- **Historical artifacts are NEVER rewritten.** These keep the old name and are excluded from every sweep and from the final grep-clean assertion:
  - `docs/adrs/0048-docket-yml-example-invariants.md` (Accepted ⇒ body immutable)
  - `docs/adrs/README.md` — **generated** by `render-adr-index.sh` from ADR-0048's immutable title. Never hand-edit it; the old name in it is correct output.
  - `docs/changes/archive/2026-07-20-0101-docket-yml-example.md`
  - `docs/changes/archive/2026-07-20-0107-guard-the-readme-config-snippet-against-docket-yml-example-d.md`
  - `docs/results/2026-07-19-docket-yml-example-results.md`
  - `docs/results/2026-07-20-readme-snippet-drift-guard-results.md`
  - `docs/superpowers/plans/2026-07-19-docket-yml-example.md`
  - `docs/superpowers/plans/2026-07-20-readme-snippet-drift-guard-plan.md`
  - `docs/superpowers/specs/2026-07-19-docket-yml-example-design.md`
  - `docs/superpowers/specs/2026-07-20-readme-snippet-drift-guard-design.md`
- **ADR-0048's `## Update` note is NOT part of this branch.** ADR-0048 is docket metadata living on the `docket` branch; feature branches never modify docket metadata. The dated `## Update` note recording the rename is written by the `docket-adr` dispatch in the implementer's step 6, and reaches `main` via terminal-publish because this change already carries `adrs: [48]`. **Do not create or edit any file under `docs/adrs/` in this worktree.**
- **Do not touch the README section heading** `### \`.docket.yml\` — per-repo settings`. It names `.docket.yml` (the real config file), not the example, and `tests/…/snippet_section()` anchors on it verbatim. Renaming it would silently empty the entire `(8)` guard.
- **Portable shell.** This repo is developed on macOS (BSD sed) and must stay GNU-compatible: **never** use bare `sed -i`. Write through a temp file and `mv` (this also avoids truncating a file on a failed render).
- **Run the whole suite at the build gate**, never only the tests this plan enumerates (AGENTS.md:45).
- **A guard is code: mutation-test it** — strip the thing it guards and watch it redden (AGENTS.md:38). Task 3 does this for the renamed guard.

---

### Task 1: Capture the pre-change suite baseline

This repo has no `tests/run_all.sh`; the suite is an inline loop over `tests/test_*.sh`. Some tests may already be red on `origin/main` for environmental reasons, so "green" is not the bar — **"no NEW failures vs. the baseline"** is. Capture the baseline now, while the tree is still pristine at `origin/main`.

**Files:**
- Create: none (baseline is recorded in the task output, not committed)
- Modify: none

**Interfaces:**
- Consumes: nothing.
- Produces: `BASELINE` — the set of test files failing on unmodified `origin/main`. Tasks 2 and 3 compare against it.

- [ ] **Step 1: Confirm the worktree is pristine at `origin/main`**

```bash
cd /Users/homer/dev/docket/.worktrees/rename-docket-yml-example-to-docket-example-yml
git status --porcelain
git rev-parse HEAD origin/main
```

Expected: `git status --porcelain` prints **nothing**, and both SHAs are identical. If the tree is dirty, stop — the baseline would be meaningless.

- [ ] **Step 2: Run the whole suite and record which files fail**

```bash
for t in tests/test_*.sh; do bash "$t" >/dev/null 2>&1 || echo "FAILED $t"; done | tee /tmp/docket-0109-baseline.txt
echo "--- baseline failure count: $(wc -l < /tmp/docket-0109-baseline.txt)"
```

Expected: a list (possibly empty) of failing test files. **Write the exact list into the task report** — it is the comparison set for Tasks 2 and 3. Do not proceed until this has actually been run; an assumed baseline is not a baseline.

- [ ] **Step 3: Record the two live reference inventories**

```bash
grep -rln --exclude-dir=.git -F '.docket.yml.example' . \
  | grep -vE '^\./docs/(superpowers|results)/|^\./docs/changes/archive/|^\./docs/adrs/' | sort
echo "---"
grep -rn --exclude-dir=.git -F 'test_docket_yml_example' . \
  | grep -vE '^\./docs/(superpowers|results)/|^\./docs/changes/archive/|^\./docs/adrs/'
```

Expected — pattern A, exactly these 14 files:

```
./.docket.yml
./.docket.yml.example
./README.md
./scripts/docket-config.md
./scripts/docket-config.sh
./scripts/ensure-global-config.md
./scripts/ensure-global-config.sh
./scripts/github-mirror.md
./tests/test_docket_yml_example.sh
./tests/test_ensure_global_config.sh
./tests/test_finalize_gate.sh
./tests/test_install.sh
./tests/test_learnings_ledger.sh
./tests/test_sync_agents.sh
```

Expected — pattern B, exactly these 3 lines:

```
./.docket.yml.example:6:# honest (tests/test_docket_yml_example.sh).
./.docket.yml.example:41:# that introduces it. tests/test_docket_yml_example.sh enforces it: a new key with no entry here
./tests/test_docket_yml_example.sh:2:# tests/test_docket_yml_example.sh — run: bash tests/test_docket_yml_example.sh
```

If either inventory differs from the above, the base has moved since this plan was written — **report the difference and stop** rather than sweeping a set the plan did not account for.

- [ ] **Step 4: No commit**

This task produces no file changes. Nothing to commit.

---

### Task 2: Rename both files and sweep every live reference (one atomic commit)

**Why one commit:** `tests/test_docket_yml_example.sh`, `test_ensure_global_config.sh`, `test_finalize_gate.sh`, `test_install.sh`, `test_learnings_ledger.sh`, and `test_sync_agents.sh` all grep the example's path as a **literal string**. Renaming the file reddens all of them at once. Splitting this task would ship a knowingly-red intermediate commit, so it stays atomic.

**Files:**
- Rename: `.docket.yml.example` → `.docket.example.yml`
- Rename: `tests/test_docket_yml_example.sh` → `tests/test_docket_example_yml.sh`
- Modify: `.docket.yml` (lines 4, 23)
- Modify: `README.md` (lines 126, 136, 218 — link text *and* link target on each)
- Modify: `scripts/docket-config.md` (104, 117), `scripts/docket-config.sh` (195)
- Modify: `scripts/ensure-global-config.md` (6, 28, 30), `scripts/ensure-global-config.sh` (5, 41, 52)
- Modify: `scripts/github-mirror.md` (116)
- Modify: `tests/test_ensure_global_config.sh` (4, 24), `tests/test_finalize_gate.sh` (15, 84), `tests/test_install.sh` (60, 65, 66), `tests/test_learnings_ledger.sh` (113, 117, 121), `tests/test_sync_agents.sh` (850, 851, 852)
- Test: `tests/test_docket_example_yml.sh` (the renamed guard — it tests itself)

**Interfaces:**
- Consumes: `BASELINE` from Task 1.
- Produces: `.docket.example.yml` at the repo root and `tests/test_docket_example_yml.sh` as its guard. No shell function, variable, or assert name changes — only string contents. `EX` remains the guard's variable for the example path; `assert <name> <expr>` remains the harness.

- [ ] **Step 1: Rename both files with `git mv`**

Use `git mv` (not `mv`) so git records the rename and the diff stays reviewable.

```bash
cd /Users/homer/dev/docket/.worktrees/rename-docket-yml-example-to-docket-example-yml
git mv .docket.yml.example .docket.example.yml
git mv tests/test_docket_yml_example.sh tests/test_docket_example_yml.sh
git status --porcelain
```

Expected: two `R` (rename) entries, nothing else.

- [ ] **Step 2: Run the guard to watch it redden (proves the sweep is load-bearing)**

```bash
bash tests/test_docket_example_yml.sh 2>&1 | grep -c 'NOT OK'
```

Expected: a **non-zero** count — the guard is now pointing at a path that no longer exists. This is the failing state the rest of this task fixes. If this prints `0`, the guard is not actually reading the renamed file and something is wrong; stop and investigate.

- [ ] **Step 3: Sweep patterns A and B across the 14 live files**

Explicit file list — **never** a repo-wide `find`, which would rewrite the historical artifacts the Global Constraints exclude. Written via temp file + `mv` for BSD/GNU portability.

```bash
cd /Users/homer/dev/docket/.worktrees/rename-docket-yml-example-to-docket-example-yml

FILES="
.docket.yml
.docket.example.yml
README.md
scripts/docket-config.md
scripts/docket-config.sh
scripts/ensure-global-config.md
scripts/ensure-global-config.sh
scripts/github-mirror.md
tests/test_docket_example_yml.sh
tests/test_ensure_global_config.sh
tests/test_finalize_gate.sh
tests/test_install.sh
tests/test_learnings_ledger.sh
tests/test_sync_agents.sh
"

for f in $FILES; do
  [ -f "$f" ] || { echo "MISSING $f"; continue; }
  tmp="$(mktemp)"
  sed -e 's/\.docket\.yml\.example/.docket.example.yml/g' \
      -e 's/test_docket_yml_example/test_docket_example_yml/g' \
      "$f" > "$tmp" && mv "$tmp" "$f"
done
echo "sweep done"
```

Expected: `sweep done`, no `MISSING` lines.

- [ ] **Step 4: Replace the escaped-ERE form that Step 3 provably cannot reach**

`tests/test_docket_example_yml.sh` embeds the filename as `\.docket\.yml\.example` inside a `sed -nE` program. The backslashes break the contiguous literal, so Step 3's pattern A does not match it — verified: running pass A over that line returns it byte-identical.

```bash
cd /Users/homer/dev/docket/.worktrees/rename-docket-yml-example-to-docket-example-yml
f=tests/test_docket_example_yml.sh
tmp="$(mktemp)"
sed 's/\\\.docket\\\.yml\\\.example/\\.docket\\.example\\.yml/g' "$f" > "$tmp" && mv "$tmp" "$f"
grep -n 'sn_ptr=' "$f"
```

Expected exactly (note `\.docket\.example\.yml` inside the ERE):

```
sn_ptr="$(snippet_section | sed -nE 's/.*\[[^]]*\]\(([^)]*\.docket\.example\.yml)\).*/\1/p' | head -n1)"
```

- [ ] **Step 5: Run the renamed guard — it must be fully green**

```bash
bash tests/test_docket_example_yml.sh; echo "EXIT=$?"
```

Expected: `EXIT=0` and **no** `NOT OK` lines. In particular these asserts, which span four different files, must all pass — they are why this task is atomic:

- `.docket.example.yml exists at repo root` (the renamed file)
- `scaffold: points at .docket.example.yml` (reads the scaffold generated by `scripts/ensure-global-config.sh`)
- `README step-2 names .docket.example.yml` (reads `README.md`)
- `repo .docket.yml points at the example` (reads `.docket.yml`)
- `(8) the section links to the canonical reference` / `(8) canonical-reference link target exists (.docket.example.yml)` (needs both the README link target and Step 4's escaped ERE)

- [ ] **Step 6: Run the five sibling tests that grep the example path**

```bash
for t in test_ensure_global_config test_finalize_gate test_install test_learnings_ledger test_sync_agents; do
  printf '%s: ' "$t"; bash "tests/$t.sh" >/dev/null 2>&1 && echo ok || echo FAILED
done
```

Expected: `ok` for every test that was **not** in Task 1's `BASELINE`. A test that was already failing in `BASELINE` may still fail — that is pre-existing and not this change's regression. Any test that was passing at baseline and now says `FAILED` is a real regression: fix it before committing.

- [ ] **Step 7: Run the whole suite and compare to baseline**

```bash
for t in tests/test_*.sh; do bash "$t" >/dev/null 2>&1 || echo "FAILED $t"; done > /tmp/docket-0109-after.txt
diff /tmp/docket-0109-baseline.txt /tmp/docket-0109-after.txt && echo "NO NEW FAILURES"
```

Expected: `NO NEW FAILURES`. The one legitimate `diff` line is the guard's own filename changing (`tests/test_docket_yml_example.sh` → `tests/test_docket_example_yml.sh`) **if and only if** it was in the baseline failure set; any other difference is a regression.

- [ ] **Step 8: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/rename-docket-yml-example-to-docket-example-yml
git add -A
git status --porcelain   # expect: 2 renames + 12 modifications, nothing under docs/adrs/
git commit -m "refactor(0109): rename .docket.yml.example to .docket.example.yml

The .example suffix landed after .yml, so no editor or GitHub recognized the
file as YAML — the one file whose entire job is to be read rendered as plain
text. .docket.example.yml keeps the 'this is an example' signal and ends in
.yml, so every YAML-aware tool highlights it.

Renames its guard to tests/test_docket_example_yml.sh to match, and sweeps
every live reference. Contents, key set, and ADR-0048's three invariants are
unchanged. Historical records (change 0101/0107 artifacts, ADR-0048's body,
the generated ADR index) keep the old name deliberately."
```

Before committing, confirm `git status --porcelain` shows **nothing** under `docs/adrs/`, `docs/changes/`, `docs/results/`, or `docs/superpowers/specs/` — those are either metadata or historical records this branch must not touch.

---

### Task 3: Prove grep-cleanliness and that the renamed guard still guards

A rename sweep can pass its own tests while leaving a stale reference in prose no assert reads, and a guard whose greps were rewritten can silently stop guarding. This task closes both.

**Files:**
- Create: none
- Modify: none (verification only; if a check fails, fix in place and re-run)
- Test: `tests/test_docket_example_yml.sh`

**Interfaces:**
- Consumes: the committed state from Task 2.
- Produces: verification evidence for the results file / PR body. No code artifacts.

- [ ] **Step 1: Assert grep-cleanliness over the working tree, excluding historical artifacts**

```bash
cd /Users/homer/dev/docket/.worktrees/rename-docket-yml-example-to-docket-example-yml
grep -rn --exclude-dir=.git -F '.docket.yml.example' . \
  | grep -vE '^\./docs/(superpowers|results)/|^\./docs/changes/archive/|^\./docs/adrs/'
echo "A exit=$?"
grep -rn --exclude-dir=.git -F 'test_docket_yml_example' . \
  | grep -vE '^\./docs/(superpowers|results)/|^\./docs/changes/archive/|^\./docs/adrs/'
echo "B exit=$?"
grep -rn --exclude-dir=.git -F '\.docket\.yml\.example' . \
  | grep -vE '^\./docs/(superpowers|results)/|^\./docs/changes/archive/|^\./docs/adrs/'
echo "C exit=$?"
```

Expected: **no output lines** from any of the three greps (each `exit=1`, grep's "no match"). Any line printed is a missed reference — fix it and re-run.

- [ ] **Step 2: Confirm the excluded historical artifacts were genuinely left alone**

```bash
git diff --name-only origin/main...HEAD | grep -E '^docs/(adrs|results|superpowers/specs)/|^docs/changes/' || echo "NONE TOUCHED"
```

Expected: `NONE TOUCHED`. If any such path appears, a historical record or metadata file was rewritten — revert that file.

- [ ] **Step 3: Mutation-test the renamed guard — existence assert**

Prove the guard still fails when the thing it guards is gone (AGENTS.md:38).

```bash
mv .docket.example.yml /tmp/docket-0109-hold.yml
bash tests/test_docket_example_yml.sh 2>&1 | grep -c 'NOT OK'
mv /tmp/docket-0109-hold.yml .docket.example.yml
```

Expected: a **non-zero** count while the file is moved away. Then restore and confirm green again:

```bash
bash tests/test_docket_example_yml.sh; echo "EXIT=$?"
```

Expected: `EXIT=0`.

- [ ] **Step 4: Mutation-test the `(8)` pointer assert — the escaped-ERE line from Task 2 Step 4**

This is the line a plain sed could not reach, so it is the one most likely to have been left stale. Break the README's link target and confirm the guard notices.

```bash
cp README.md /tmp/docket-0109-readme.bak
tmp="$(mktemp)"
sed 's|(\.docket\.example\.yml)|(docs/.docket.example.yml)|' README.md > "$tmp" && mv "$tmp" README.md
bash tests/test_docket_example_yml.sh 2>&1 | grep '(8) canonical-reference'
cp /tmp/docket-0109-readme.bak README.md
```

Expected: while broken, the assert prints `NOT OK - (8) canonical-reference link target exists (docs/.docket.example.yml)`. If it prints `ok`, the `sn_ptr` extraction is returning empty and the guard has gone vacuous — the escaped ERE was not updated correctly; go back and fix Task 2 Step 4.

- [ ] **Step 5: Confirm the tree is clean after mutation testing**

```bash
git status --porcelain
```

Expected: **no output**. The mutations in Steps 3–4 were all restored. If anything is dirty, restore it — mutation scaffolding must never be committed.

- [ ] **Step 6: Final whole-suite run**

```bash
for t in tests/test_*.sh; do bash "$t" >/dev/null 2>&1 || echo "FAILED $t"; done > /tmp/docket-0109-final.txt
diff /tmp/docket-0109-baseline.txt /tmp/docket-0109-final.txt && echo "NO NEW FAILURES"
```

Expected: `NO NEW FAILURES` (modulo the guard's own filename, per Task 2 Step 7).

- [ ] **Step 7: No commit**

This task changes no files. If Step 1 or 2 required a fix, commit that fix with a `fix(0109):` message and re-run Steps 1–6.

---

## Self-Review

**1. Requirements coverage** (source: change 0109's `## What changes` + reconcile log):

| Requirement | Task |
|---|---|
| `git mv` the example file | Task 2 Step 1 |
| Rename the test file too | Task 2 Step 1 |
| Update `.docket.yml`, `README.md` | Task 2 Step 3 |
| Update `scripts/docket-config.{sh,md}`, `ensure-global-config.{sh,md}`, `github-mirror.md` | Task 2 Step 3 |
| Update the six test files | Task 2 Steps 3–4 |
| Reconcile pt. 3 — example's two pointers to its own guard filename | Task 2 Step 3 (pattern B) |
| Final `grep` comes back empty | Task 3 Step 1 (all three patterns) |
| Reconcile pt. 4 — expanded exclusion set incl. generated ADR index | Global Constraints + Task 3 Steps 1–2 |
| Reconcile pt. 5 — ADR `## Update` is metadata, not this branch | Global Constraints + Task 2 Step 8 check |
| Contents / invariants unchanged | Global Constraints; Task 3 Step 2 |

No requirement is unassigned.

**2. Placeholder scan:** No TBD/TODO, no "add error handling", no "similar to Task N". Every step carries its exact command and expected output. The one deliberately open value is Task 1's `BASELINE`, which *must* be measured rather than assumed — it is stated as an output to record, not a blank to fill.

**3. Consistency:** `.docket.example.yml` and `tests/test_docket_example_yml.sh` are spelled identically in every task. The guard's internals keep their existing names (`EX`, `assert`, `snippet_section`, `readme_snippet`, `flatten_yaml`, `sn_ptr`) — this change alters string *contents* only, never an identifier, so there is no cross-task signature drift to reconcile.

**4. Risk note carried into the build:** the only step whose failure mode is *silent* is Task 2 Step 4 (the escaped ERE). If missed, `sn_ptr` resolves empty and `(8) the section links to the canonical reference` fails loudly — but a careless "fix" that deletes the assert instead would hide it. Task 3 Step 4 mutation-tests exactly this path.
