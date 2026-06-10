# Convention Extraction Skill Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extract the 146-line convention block (duplicated byte-identically across all five docket skills) into a sixth, pure-reference skill `docket-convention`, replace each embedded copy with a blocking Step-0 load instruction, and retire the sync machinery.

**Architecture:** The convention text moves verbatim (one sentence edit) from `skills/docket-new-change/SKILL.md` — today's canonical copy — into `skills/docket-convention/SKILL.md`. Each operating skill's marker-delimited block becomes a ~4-line stub that forces the reference load (undefined-terms forcing function: slimmed skills keep using convention vocabulary without redefining it). `sync-convention.sh` and its test are deleted; a new test asserts the extraction holds in both directions (sentinels present in the reference, absent from operating skills).

**Tech Stack:** Markdown skill files, bash test scripts (repo's existing `assert`-style harness, run via `bash tests/<name>.sh`).

**Spec:** `docs/superpowers/specs/2026-06-10-convention-extraction-skill-design.md` on the `docket` branch (read it from the `.docket/` metadata worktree; it is NOT on this feature branch).

**Working directory:** all paths below are relative to the feature worktree root (`.worktrees/convention-extraction-skill/`).

---

### Task 1: Write the failing extraction test

The whole change is verified by one test file. Write it first; it fails until Tasks 2–4 land, then gates Task 5's final run. Style matches the existing tests (`set -uo pipefail`, `assert(){ ... eval ... }`).

**Files:**
- Create: `tests/test_convention_extraction.sh`

- [ ] **Step 1: Write the test**

```bash
#!/usr/bin/env bash
# tests/test_convention_extraction.sh — run: bash tests/test_convention_extraction.sh
#
# Guards change 0005's extraction invariant in BOTH directions:
#   - the docket-convention reference skill exists and carries the full contract
#   - no operating skill contains a copy of convention content (sentinel scan)
#   - every operating skill carries the blocking Step-0 load line
#   - the retired sync machinery stays retired
#   - link-skills.sh's glob picks up the sixth skill
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

REF="$REPO/skills/docket-convention/SKILL.md"
OPERATING=(docket-new-change docket-implement-next docket-status docket-finalize-change docket-adr)

# (a) reference skill exists and carries the convention's section headers
assert "docket-convention/SKILL.md exists" '[ -f "$REF" ]'
for h in "### Configuration" "### Directory layout" "### Change manifest" "### ADR file" "### Lifecycle" "### Build-readiness" "### Bootstrap guard" "### Branch model"; do
  assert "reference has header: $h" '[ -f "$REF" ] && grep -qF "$h" "$REF"'
done

# (b) anti-copy sentinels — one per convention section (spec §5); each must be IN the
# reference and ABSENT from every operating skill. The old sync markers count as copies.
SENTINELS=(
  "never gitignored"
  "proposed ──claim──▶"
  "satisfied when it reaches"
  "immutable once Accepted"
  "live planning surface"
  "half-migrated"
  "only flow of metadata onto the code line"
  "<!-- docket:convention:begin -->"
  "<!-- docket:convention:end -->"
)
for s in "${SENTINELS[@]:0:7}"; do
  assert "reference contains sentinel: $s" '[ -f "$REF" ] && grep -qF "$s" "$REF"'
done
for sk in "${OPERATING[@]}"; do
  f="$REPO/skills/$sk/SKILL.md"
  for s in "${SENTINELS[@]}"; do
    assert "$sk has no convention copy: $s" '! grep -qF "$s" "$f"'
  done
  # (c) the blocking Step-0 load line
  assert "$sk has the Step-0 load heading" 'grep -qF "## Convention (load first — blocking)" "$f"'
  assert "$sk names docket-convention" 'grep -qF "docket-convention" "$f"'
done

# (d) retired machinery stays retired
assert "sync-convention.sh retired" '[ ! -e "$REPO/sync-convention.sh" ]'
assert "test_sync_convention.sh retired" '[ ! -e "$REPO/tests/test_sync_convention.sh" ]'
assert "no other test calls sync-convention" \
  '! grep -rl "sync-convention" "$REPO/tests" --include="*.sh" | grep -v test_convention_extraction >/dev/null'

# (e) link-skills.sh globs the sixth skill (uses the script's DOCKET_HARNESS_ROOT test seam)
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/.claude/skills"
DOCKET_HARNESS_ROOT="$tmp" bash "$REPO/link-skills.sh" >/dev/null
assert "link-skills.sh links docket-convention" '[ -L "$tmp/.claude/skills/docket-convention" ]'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
```

- [ ] **Step 2: Run it to verify it fails**

Run: `bash tests/test_convention_extraction.sh`
Expected: `NOT OK` lines for the missing reference skill, the sentinel/marker copies still present in all five skills, the missing Step-0 headings, and the still-present `sync-convention.sh` + `test_sync_convention.sh`; the `link-skills.sh` assert also fails (no `docket-convention` dir to link yet). Exit code 1, final line `FAIL`.

- [ ] **Step 3: Commit**

```bash
git add tests/test_convention_extraction.sh
git commit -m "test(0005): extraction invariant test for docket-convention (red)"
```

---

### Task 2: Create `skills/docket-convention/SKILL.md`

Extract the convention block from today's canonical copy mechanically — do NOT retype it. One sentence inside it changes (the sync-script sentence), and that is the ONLY content edit.

**Files:**
- Create: `skills/docket-convention/SKILL.md`

- [ ] **Step 1: Extract the block under a new skill header**

```bash
mkdir -p skills/docket-convention
{
  cat <<'EOF'
---
name: docket-convention
description: Use when any docket skill runs — docket-new-change, docket-implement-next, docket-status, docket-finalize-change, and docket-adr load this first (their blocking Step 0) — or when you need to understand how docket tracks work. The shared contract — .docket.yml configuration, directory layout, the change manifest and lifecycle, ADR format, build-readiness and selection, the bootstrap guard, and the branch model. Pure reference — defines the convention; performs no reads, writes, or git operations.
---

# docket-convention — the shared contract (pure reference)

This skill defines the docket convention and does nothing else: no procedure, no reads or writes, no git. The five operating skills load it as their blocking Step 0 and use its vocabulary without restating it.

EOF
  awk '/^<!-- docket:convention:begin -->$/{g=1;next} /^<!-- docket:convention:end -->$/{g=0;next} g' \
    skills/docket-new-change/SKILL.md
} > skills/docket-convention/SKILL.md
```

Note: the `description` is the spec §3 wording with its one colon replaced by an em-dash ("tracks work. The shared contract —") so the value stays a plain unquoted YAML scalar like every other skill's frontmatter (a `: ` inside a plain scalar is invalid YAML).

- [ ] **Step 2: Apply the single content edit (Edit tool, exact strings)**

In `skills/docket-convention/SKILL.md`, replace:

```
This block is the shared contract every docket skill embeds. It is kept byte-identical across the five skills by `sync-convention.sh` (canonical source: `docket-new-change/SKILL.md`); never hand-edit it in a non-canonical skill.
```

with:

```
This skill is the single source of the convention; the operating skills (docket-new-change, docket-implement-next, docket-status, docket-finalize-change, docket-adr) load it at startup as their blocking Step 0 and never restate it.
```

- [ ] **Step 3: Verify the extraction is byte-faithful**

Run:
```bash
diff <(awk '/^<!-- docket:convention:begin -->$/{g=1;next} /^<!-- docket:convention:end -->$/{g=0;next} g' skills/docket-new-change/SKILL.md | tail -n +3) \
     <(awk '/^# docket-convention/{found=1} found' skills/docket-convention/SKILL.md | tail -n +5 | head -n +144) | head -20
```
Expected: the only diff hunk is the replaced sentence from Step 2 (the paragraph starting "docket tracks planned work"). If other hunks appear, the extraction mangled content — redo Step 1.

(Simpler equivalent check if the offsets prove brittle: `grep -c '^###' skills/docket-convention/SKILL.md` → `8`, and every sentinel grep from the test's section (b) passes for `$REF`.)

- [ ] **Step 4: Run the test — reference-side asserts now pass**

Run: `bash tests/test_convention_extraction.sh`
Expected: all `reference has header:` and `reference contains sentinel:` asserts `ok`; the five skills' `has no convention copy` asserts still `NOT OK` (blocks not yet removed); `link-skills.sh links docket-convention` now `ok`. Exit 1 overall.

- [ ] **Step 5: Commit**

```bash
git add skills/docket-convention/SKILL.md
git commit -m "feat(0005): docket-convention — the shared contract as a pure-reference skill"
```

---

### Task 3: Replace the embedded block in all five operating skills with the Step-0 stub

**Files:**
- Modify: `skills/docket-new-change/SKILL.md` (block spans the `<!-- docket:convention:begin/end -->` markers)
- Modify: `skills/docket-implement-next/SKILL.md` (same markers)
- Modify: `skills/docket-status/SKILL.md` (same markers)
- Modify: `skills/docket-finalize-change/SKILL.md` (same markers)
- Modify: `skills/docket-adr/SKILL.md` (same markers)

- [ ] **Step 1: Swap block → stub in all five files**

```bash
STUB='## Convention (load first — blocking)

Before anything else in this skill, invoke the `docket-convention` skill via the Skill tool — unless it was already invoked earlier in this session and its content is in context. Everything below uses its vocabulary (build-ready, metadata working tree, terminal-publish, the `DOCKET`/`LIVE` bootstrap probes, …) without redefinition; no step below is executable without the convention loaded.'

for sk in docket-new-change docket-implement-next docket-status docket-finalize-change docket-adr; do
  f="skills/$sk/SKILL.md"
  awk -v stub="$STUB" '
    $0=="<!-- docket:convention:begin -->"{print stub; skip=1; next}
    $0=="<!-- docket:convention:end -->"{skip=0; next}
    !skip{print}
  ' "$f" > "$f.tmp" && mv "$f.tmp" "$f"
done
```

- [ ] **Step 2: Sweep for restatements outside the old markers**

The brainstorm-day collision scan found none, but verify against the now-slimmed files — every sentinel grep must come back empty:

```bash
for s in "never gitignored" "proposed ──claim──▶" "satisfied when it reaches" "immutable once Accepted" "live planning surface" "half-migrated" "only flow of metadata onto the code line"; do
  grep -rF "$s" skills/docket-new-change skills/docket-implement-next skills/docket-status skills/docket-finalize-change skills/docket-adr && echo "RESTATEMENT: $s"
done; echo "sweep done"
```
Expected: only `sweep done`. Any hit means convention content lives outside the markers in that skill — remove or repoint that prose to "per the convention" (do not paraphrase it), then re-run.

- [ ] **Step 3: Sanity-read one slimmed skill**

Run: `sed -n '1,40p' skills/docket-status/SKILL.md`
Expected: frontmatter + Overview + When-to-use intact, then the 3-line `## Convention (load first — blocking)` stub where the 146-line block used to be, then the skill's own sections (`## Shared dependency-resolution pass`, …) untouched.

- [ ] **Step 4: Run the test — operating-skill asserts now pass**

Run: `bash tests/test_convention_extraction.sh`
Expected: every `has no convention copy` / `Step-0 load heading` / `names docket-convention` assert `ok`. Still `NOT OK`: the two `retired` asserts and `no other test calls sync-convention` (Task 4). Exit 1.

- [ ] **Step 5: Commit**

```bash
git add skills/docket-new-change/SKILL.md skills/docket-implement-next/SKILL.md skills/docket-status/SKILL.md skills/docket-finalize-change/SKILL.md skills/docket-adr/SKILL.md
git commit -m "feat(0005): operating skills load docket-convention (blocking Step 0) instead of embedding"
```

---

### Task 4: Retire the sync machinery

**Files:**
- Delete: `sync-convention.sh`
- Delete: `tests/test_sync_convention.sh`
- Modify: `tests/test_board_refresh_on_transition.sh:17-18`
- Modify: `tests/test_results_artifact.sh:13-14`
- Modify: `tests/test_docket_metadata_branch.sh:12-13`

- [ ] **Step 1: Delete the script and its test**

```bash
git rm sync-convention.sh tests/test_sync_convention.sh
```

- [ ] **Step 2: Drop the sync-check assert from the three other tests**

In each of `tests/test_board_refresh_on_transition.sh`, `tests/test_results_artifact.sh`, `tests/test_docket_metadata_branch.sh`, remove exactly these two lines (Edit tool, identical in all three):

```
assert "convention blocks in sync (sync-convention.sh --check)" \
  'bash sync-convention.sh --check >/dev/null 2>&1'
```

Leave surrounding lines untouched (no double blank lines left behind — if removal leaves two consecutive blank lines, collapse to one).

- [ ] **Step 3: Run the full extraction test — all green**

Run: `bash tests/test_convention_extraction.sh`
Expected: every assert `ok`, final line `PASS`, exit 0.

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "chore(0005): retire sync-convention.sh + drift checks — extraction makes drift impossible"
```

---

### Task 5: README — five skills become six

**Files:**
- Modify: `README.md:3` (intro sentence: "provides five skills …")
- Modify: `README.md:13` ("five skills, no CLI")
- Modify: `README.md:15` ("The five skills cover the full loop…")
- Modify: `README.md:198` (`## The five skills` heading + table)

- [ ] **Step 1: Update the three prose mentions (Edit tool, exact strings)**

Replace `provides five skills to create changes, work the next change to a PR, finalize a merged change, report the board, and record architecture decisions (ADRs)` with `provides six skills to create changes, work the next change to a PR, finalize a merged change, report the board, record architecture decisions (ADRs), and define the shared convention they all load`.

Replace `plain markdown files in your repo, five skills, no CLI` with `plain markdown files in your repo, six skills, no CLI`.

Replace `The five skills cover the full loop: create, implement, finalize, report, decide.` with `The six skills cover the full loop: create, implement, finalize, report, decide — plus the shared contract they all load as a pure-reference skill.`

- [ ] **Step 2: Update the skills table**

Replace the heading `## The five skills` with `## The six skills`, and add this row at the bottom of the table (after the `docket-adr` row):

```
| `docket-convention` | Shared contract, pure reference — single source of the docket convention (configuration, layout, manifest, lifecycle, build-readiness, bootstrap guard, branch model); every operating skill loads it as its blocking Step 0. |
```

- [ ] **Step 3: Verify no stale "five skills" or sync-script mentions remain in living docs**

Run: `grep -rn "five skills\|sync-convention" README.md skills/`
Expected: no output. (Historical records under `docs/` are immutable and deliberately excluded.)

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "docs(0005): README — six skills, convention single-sourced in docket-convention"
```

---

### Task 6: Full test suite

- [ ] **Step 1: Run every test**

```bash
for t in tests/*.sh; do echo "== $t"; bash "$t" || echo "SUITE-FAIL: $t"; done
```
Expected: every file ends `PASS` (or all-`ok` output for tests without a PASS line); no `SUITE-FAIL` lines. Note `tests/test_sync_convention.sh` no longer exists, so it cannot run.

- [ ] **Step 2: Verify the worktree is clean and the branch is coherent**

Run: `git status --short && git log --oneline origin/main..HEAD`
Expected: clean tree; commits from Tasks 1–5 (red test, new skill, slimmed skills, retirement, README), in that order.

---

## Out of plan (orchestrator work, not subagent tasks)

These belong to the `docket-implement-next` orchestrator AFTER the plan executes — they touch the metadata worktree, which feature-branch subagents must never do:

- Mint the ADR ("reference-loading over embedding for the docket convention", distilling spec §2; `change: 5`, `relates_to: [2]` — ADR-0002 set the reference-don't-restate precedent for terminal-publish) via the `docket-adr` skill; append its id to the change's `adrs:`.
- The manual behavioral acceptance check from spec §8 (fresh-session `docket-status` invokes `docket-convention` first) — a merge-gate item for the human, recorded in the results file / PR description.
- `plan:` / `results:` field writes, `status: implemented`, `pr:` — all on the `docket` branch.
