<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0254 — BSD tool-default sweep: templated mktemp and non-interactive mv](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-08-0254-bsd-tool-default-sweep-templated-mktemp-and-non-interactive.md)**
<!-- docket:backlink:end -->

# BSD tool-default sweep: templated mktemp and non-interactive mv — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stop two BSD tool defaults from silently defeating guards in this repo — bare `mv` self-answering an overwrite prompt and exiting 0, and bare `mktemp` ignoring `TMPDIR` — by sweeping every call site to the non-interactive/templated form and adding a repo-wide shape-keyed guard that keeps them swept.

**Architecture:** Four tasks, each test-first. Tasks 1 and 2 each add one half of a new guard file `tests/test_bsd_tool_defaults.sh` (a negative shape grep plus a positive population floor), watch it redden against the unswept tree, then perform the mechanical sweep that greens it. Task 3 adds the one behavioral pin where a fixture already depends on `TMPDIR`. Task 4 lands the prose rule in `AGENTS.md`, registers the new file's runtime budget, and re-runs the `cp`/`rm` audit.

**Tech Stack:** POSIX `sh`-compatible bash scripts, the repo's own `tests/test_*.sh` harness (auto-discovered by `scripts/run-tests.sh`), POSIX ERE greps run under a **pinned `/usr/bin/grep`**.

## Global Constraints

Copied verbatim from the spec (2026-08-07) and `AGENTS.md`. Every task's requirements implicitly include this section.

- **mktemp house form:** `mktemp -d "${TMPDIR:-/tmp}/<script-name>.XXXXXX"` for dirs and `mktemp "${TMPDIR:-/tmp}/<script-name>.XXXXXX"` for files. `<script-name>` is the owning script's basename **sans `.sh`**. Exactly **6** `X`s. `${TMPDIR:-/tmp}` uses `:-` (covers unset *and* empty), never `-`.
- **mv house form:** uniform `mv -f`. **`git mv` is carved out** — `scripts/archive-change.sh` spells it `$GIT -C "$WT" mv …` and stays exactly as it is.
- **Six beside-destination templated `mktemp` sites are correct and MUST NOT be touched:** `scripts/ensure-docket-env.sh` (3 sites), `scripts/board-refresh.sh`, `scripts/docket-status.sh`, `scripts/ensure-global-config.sh`. They are templated *beside their destination* so the following `mv` is a same-filesystem atomic rename — a documented guarantee. The guard predicate is therefore **template-required, never TMPDIR-required**.
- **Guard scope (both directions):** `scripts/*.sh`, `scripts/lib/*.sh`, `scripts/runners/*.sh`, and repo-root `install.sh`, `sync-agents.sh`, `migrate-to-docket.sh`. **`tests/` is excluded.**
- **Patterns stay POSIX ERE.** No PCRE, no `\b` / `\<` (BSD ERE returns zero for them, silently), no bounded repetition near 255, and never two bounded gaps in one ERE.
- **The guard invokes `/usr/bin/grep` by absolute path**, never PATH `grep` — PATH `grep` here is ugrep 7.5.0 and disagrees on the combined pattern spelling.
- **Guards are code: mutation-test every assert** (strip what it guards, watch it redden, restore). Restore from a **backup copy**, never `git checkout -- <file>` (that restores to HEAD and destroys the uncommitted work under test).
- **No `file:line` anchors in test comments** (ADR-0054 / the comment-anchor guard).
- **A guard failure message states the rule**, never "bump the count".
- Commit after every task. Do not merge, do not open a PR.

---

## File Structure

| File | Disposition | Responsibility |
|---|---|---|
| `tests/test_bsd_tool_defaults.sh` | **Create** (Task 1, extended Task 2) | The repo-wide shape-keyed guard: no bare `mv "`, every `$(mktemp` line templated, with population floors both halves. |
| 16 scripts (Task 1 list) | Modify | `mv` → `mv -f` at each atomic-replace/rename site. |
| 21 scripts (Task 2 list) | Modify | Every untemplated `mktemp` gets the house template. |
| `tests/test_backfill_change_types.sh` | Modify (Task 3) | Behavioral pin: the stage remnant lands **under** the fixture's redirected `TMPDIR`. |
| `AGENTS.md` | Modify (Task 4) | Two new `## Shell` bullets. |
| `tests/runtime-budgets.tsv`, `tests/test_runtime_budgets.sh` | Modify (Task 4) | Budget row for the new test file + `EXPECTED_TOTAL` re-seed. |

**Derive every site list by grep at build time.** The line numbers below are point-in-time (re-derived 2026-08-08) and are navigation aids, not the source of truth.

---

### Task 1: The `mv` half of the guard, then the `mv -f` sweep

**Files:**
- Create: `tests/test_bsd_tool_defaults.sh`
- Modify (16 sites, one edit each): `scripts/archive-change.sh:71`, `scripts/board-refresh.sh:128`, `scripts/docket-status.sh:1047`, `scripts/ensure-claude-settings.sh:68`, `scripts/ensure-docket-env.sh:92`, `scripts/ensure-docket-env.sh:119`, `scripts/ensure-global-config.sh:169`, `scripts/mark-publish-deferred.sh:116`, `scripts/mark-publish-deferred.sh:192`, `scripts/mint-stub.sh:148`, `scripts/mint-stub.sh:201`, `scripts/reclaim-claims.sh:49`, `scripts/render-artifact-backlink.sh:117`, `scripts/render-change-links.sh:181`, `sync-agents.sh:128`, `migrate-to-docket.sh:345`
- Do **not** modify: `scripts/archive-change.sh:95` (`$GIT -C "$WT" mv …`), `scripts/backfill-change-types.sh:169` (already `mv -f`)

**Interfaces:**
- Consumes: nothing (first task).
- Produces: `tests/test_bsd_tool_defaults.sh` containing the shell functions `scope_files()` (prints one absolute in-scope path per line) and `hits_mv()` (prints `path:lineno:text` for every bare-`mv` shape), plus the constants `GREP=/usr/bin/grep`, `MIN_FILES=40`, `MIN_MV_F=16`. Task 2 extends this same file and reuses `scope_files` and `GREP` unchanged.

- [ ] **Step 1: Write the failing guard (the `mv` half only)**

Create `tests/test_bsd_tool_defaults.sh` with exactly this content:

```bash
#!/usr/bin/env bash
# tests/test_bsd_tool_defaults.sh — a BSD tool's DEFAULT behavior may not silently defeat a guard
# (change 0254). Two shapes, one class.
#
# WHY (mv): BSD `mv` with an unwritable destination and a tty on stdin PROMPTS, self-answers `n`
# at EOF, prints "not overwritten", and EXITS 0. Every `|| die` guard on such a call is therefore
# unreachable and the write is silently discarded. `-f` is what makes an install non-interactive;
# it is load-bearing, not style.
#
# WHY (mktemp): a bare `mktemp`, with or without `-d`, ignores TMPDIR on macOS and lands the temp
# file outside any redirect. A fixture that redirects TMPDIR to contain a script's scratch dir is
# then a no-op, and undeletable debris accumulates outside the fixture forever.
#
# SHAPE-KEYED, NOT FILE-KEYED: both halves are repo-wide policy asserted over a call shape. There
# is no allowlist of exempt files (ADR-0050) — exclusions are by walk scope only.
#
# PINNED GREP: the scan runs /usr/bin/grep by absolute path, never PATH grep. Probed during design:
# the single combined ERE `(^|[^-[:alnum:]])mv "` matches NOTHING under this machine's PATH grep
# (ugrep) while matching correctly under /usr/bin/grep — the combined spelling would make this
# guard vacuous exactly where the suite usually runs. Two split patterns agree under both engines,
# and pinning the binary removes the engine variable entirely. Engine agreement is never assumed.
#
# POPULATION FLOORS: a negative grep passes both when the tree is clean and when the scan has
# silently collapsed. Each half therefore also asserts a floor on the POSITIVE population it
# expects to find. The floors are literals both engines handle; they cannot detect a vacuous
# negative predicate, which is what the pinned binary and the mutation tests cover instead.
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# The scan binary, pinned. On Linux this is GNU grep; on macOS, BSD grep. Both agree on the split
# patterns below. PATH grep is deliberately not trusted.
GREP=/usr/bin/grep
assert "the pinned scan binary exists" '[ -x "$GREP" ]'

# In-scope population floor: if the walk collapses, every negative assert below passes vacuously.
MIN_FILES=40
# Post-sweep floor on atomic-replace/rename call sites written the non-interactive way.
MIN_MV_F=16

# The guarded surface: shipped shell, entry scripts included. tests/ is excluded — test-side
# hygiene is owned elsewhere, and this file's own patterns would match themselves.
scope_files(){
  local f
  for f in "$ROOT"/scripts/*.sh "$ROOT"/scripts/lib/*.sh "$ROOT"/scripts/runners/*.sh \
           "$ROOT"/install.sh "$ROOT"/sync-agents.sh "$ROOT"/migrate-to-docket.sh; do
    [ -f "$f" ] && printf '%s\n' "$f"
  done
}

n_files="$(scope_files | wc -l | tr -d ' ')"
assert "the scan reaches at least $MIN_FILES in-scope files (it reached $n_files)" \
  '[ "$n_files" -ge "$MIN_FILES" ]'

# Every `mv` invocation whose first argument is a double-quoted word. Two patterns, never one
# combined alternation: column-0 `mv`, and `mv` preceded by any character that is not part of a
# longer word or a trailing option cluster. They are disjoint, so no line is reported twice.
hits_mv(){
  local f
  while IFS= read -r f; do
    "$GREP" -nE '^mv "' "$f" | sed "s|^|$f:|"
    "$GREP" -nE '[^-[:alnum:]]mv "' "$f" | sed "s|^|$f:|"
  done < <(scope_files)
}

# An allowance keyed on the invocation shape actually present in the tree: archive-change.sh spells
# its rename `$GIT -C "$WT" mv …`, so a literal `git mv` allowance would never fire and the guard
# would redden on the carve-out. `git mv` is a different tool with different prompting semantics,
# and `-f` there means force-overwrite a tracked target — a semantics change, not a hardening.
offenders_mv(){ hits_mv | "$GREP" -vE 'mv -f ' | "$GREP" -vE '(git|\$GIT)[^|]* mv '; }

bad_mv="$(offenders_mv)"
assert "every mv that replaces a file passes -f, so it cannot prompt on a tty" \
  '[ -z "$bad_mv" ] || { echo "$bad_mv" | sed "s|^$ROOT/|  |" >&2; echo "  RULE: bare mv prompts on an unwritable destination with a tty, self-answers n, and exits 0 — so the || die never fires and the write is lost. Write these as: mv -f SRC DEST. git mv is exempt (different tool)." >&2; false; }'

n_mv_f="$("$GREP" -rlE 'mv -f ' $(scope_files) 2>/dev/null | wc -l | tr -d " ")"
n_mv_f_sites="$("$GREP" -hcE 'mv -f ' $(scope_files) 2>/dev/null | awk '{s+=$1} END{print s+0}')"
assert "at least $MIN_MV_F non-interactive mv sites exist, so the check above is not vacuous (found $n_mv_f_sites)" \
  '[ "$n_mv_f_sites" -ge "$MIN_MV_F" ] || { echo "  RULE: this floor exists because a negative grep also passes when the scan finds nothing. A drop means either the scan broke or an install path stopped replacing files — check which before touching this number." >&2; false; }'

exit "$fail"
```

- [ ] **Step 2: Run it to verify it fails**

Run: `bash tests/test_bsd_tool_defaults.sh`

Expected: FAIL. The `mv -f` assert prints `NOT OK` and lists **16** offending sites (the ones in the Files list above). The population-floor assert also fails, because only `scripts/backfill-change-types.sh` carries `mv -f ` today (1 site, under the floor of 16). `archive-change.sh`'s `$GIT … mv` line must **not** appear in the offender list — if it does, the allowance pattern is wrong; fix it before proceeding.

- [ ] **Step 3: Sweep all 16 sites to `mv -f`**

Re-derive the list rather than trusting the plan's line numbers:

```bash
/usr/bin/grep -nE '(^|[^-[:alnum:]])mv "' scripts/*.sh scripts/lib/*.sh scripts/runners/*.sh \
  install.sh sync-agents.sh migrate-to-docket.sh
```

Each edit inserts exactly ` -f` after `mv`, changing nothing else on the line. The full set, before → after:

```sh
# scripts/archive-change.sh
sed -E "/^---$/,/^---$/ s|^($k:)[[:space:]]*.*|\1 $v|" "$f" > "$t" && mv -f "$t" "$f"
# scripts/board-refresh.sh
mv -f "$tmp_board" "$CHANGES_DIR/BOARD.md" || { printf 'board-refresh: failed to replace BOARD.md\n' >&2; exit 1; }
# scripts/docket-status.sh
  mv -f "$tmp" "$ldir/README.md"
# scripts/ensure-claude-settings.sh
  mv -f "$tmp" "$SETTINGS"
# scripts/ensure-docket-env.sh   (two sites)
mv -f "$tmp" "$prof" || die "cannot atomically replace $prof"
      if ! mv -f "$t" "$settings"; then
# scripts/ensure-global-config.sh
mv -f "$_tmp" "$DEST" || die "cannot atomically replace $DEST"
# scripts/mark-publish-deferred.sh   (two sites)
  mv -f "$tmp.2" "$CHANGE_FILE" || die "write failed"
mv -f "$tmp.3" "$CHANGE_FILE" || die "write failed"
# scripts/mint-stub.sh   (two sites)
  mv -f "$t" "$f"
  mv -f "$tmp2" "$tmp"
# scripts/reclaim-claims.sh
  sed -E "/^---$/,/^---$/ s|^($k:)[[:space:]]*.*|\1 $v|" "$f" > "$t" && mv -f "$t" "$f"
# scripts/render-artifact-backlink.sh
mv -f "$out" "$ARTIFACT_FILE"
# scripts/render-change-links.sh
mv -f "$out" "$CHANGE_FILE"
# sync-agents.sh
  mv -f "$LEGACY_GLOBAL_CFG" "$LEGACY_GLOBAL_CFG.migrated"
# migrate-to-docket.sh
  mv -f "$tmp" "$GITIGNORE"
```

Leave `scripts/archive-change.sh`'s `$GIT -C "$WT" mv …` untouched.

- [ ] **Step 4: Run the guard to verify it passes**

Run: `bash tests/test_bsd_tool_defaults.sh`
Expected: PASS — every assert `ok`, exit 0.

- [ ] **Step 5: Mutation-test both directions**

The guard is code; prove each half fails when it should. Restore from a **backup copy**, never `git checkout`.

```bash
cp scripts/mint-stub.sh /tmp/mint-stub.bak
sed -i '' 's|  mv -f "$t" "$f"|  mv "$t" "$f"|' scripts/mint-stub.sh
bash tests/test_bsd_tool_defaults.sh; echo "exit=$?"   # expect: NOT OK on the -f assert, exit 1
cp /tmp/mint-stub.bak scripts/mint-stub.sh
bash tests/test_bsd_tool_defaults.sh; echo "exit=$?"   # expect: all ok, exit 0
```

Then prove the floor is independent of the negative predicate — mutate `MIN_MV_F` to `99` in a copy of the test, confirm the floor assert alone reddens, and restore:

```bash
cp tests/test_bsd_tool_defaults.sh /tmp/guard.bak
sed -i '' 's|^MIN_MV_F=16|MIN_MV_F=99|' tests/test_bsd_tool_defaults.sh
bash tests/test_bsd_tool_defaults.sh   # expect: only the floor assert NOT OK
cp /tmp/guard.bak tests/test_bsd_tool_defaults.sh
```

Finally confirm the carve-out is really exercised: `bash -c 'source /dev/stdin <<< "$(sed -n "/^hits_mv/,/^}/p" tests/test_bsd_tool_defaults.sh)"'` is **not** needed — simply confirm `/usr/bin/grep -nE '[^-[:alnum:]]mv "' scripts/archive-change.sh` prints the `$GIT` line, and that the guard still passes. If the allowance were removed, that line would be an offender.

- [ ] **Step 6: Commit**

```bash
git add tests/test_bsd_tool_defaults.sh scripts/archive-change.sh scripts/board-refresh.sh \
  scripts/docket-status.sh scripts/ensure-claude-settings.sh scripts/ensure-docket-env.sh \
  scripts/ensure-global-config.sh scripts/mark-publish-deferred.sh scripts/mint-stub.sh \
  scripts/reclaim-claims.sh scripts/render-artifact-backlink.sh scripts/render-change-links.sh \
  sync-agents.sh migrate-to-docket.sh
git commit -m "fix(0254): make every atomic-replace mv non-interactive, guarded repo-wide"
```

---

### Task 2: The `mktemp` half of the guard, then the template sweep

**Files:**
- Modify: `tests/test_bsd_tool_defaults.sh` (append the mktemp half before the final `exit "$fail"`)
- Modify (23 call sites across 15 files): `scripts/archive-change.sh:70`, `scripts/backfill-change-types.sh:113`, `scripts/docket-config.sh:260`, `scripts/docket-config.sh:274`, `scripts/ensure-claude-settings.sh:59`, `scripts/mark-publish-deferred.sh:94`, `scripts/mint-stub.sh:139`, `scripts/mint-stub.sh:192` (**two calls on one line**), `scripts/profile-asserts.sh:73`, `scripts/profile-one-test.sh:73`, `scripts/reclaim-claims.sh:48`, `scripts/render-artifact-backlink.sh:90`, `scripts/render-artifact-backlink.sh:101`, `scripts/render-change-links.sh:157`, `scripts/render-change-links.sh:164`, `scripts/run-tests.sh:185`, `scripts/runners/codex.sh:89`, `scripts/terminal-publish.sh:112`, `scripts/terminal-publish.sh:270`, `sync-agents.sh:1364`, `sync-agents.sh:1382`, `migrate-to-docket.sh:344`
- Do **not** modify: the six beside-destination templated sites named in Global Constraints, nor `migrate-to-docket.sh:213` (already the house form).

**Interfaces:**
- Consumes: `scope_files()`, `GREP`, and the `assert` helper from Task 1's file — reuse them, do not write a second scan implementation.
- Produces: the constant `MIN_MKTEMP_TEMPLATED=29` and the function `hits_mktemp()`. Nothing later depends on them.

- [ ] **Step 1: Write the failing assert**

Insert immediately **before** the `exit "$fail"` line of `tests/test_bsd_tool_defaults.sh`:

```bash
# Post-sweep floor on templated mktemp calls: 23 swept here plus 6 pre-existing beside-destination
# sites. Same reason as the mv floor — a negative grep is also green when it scans nothing.
MIN_MKTEMP_TEMPLATED=29

# Every line invoking mktemp through command substitution. One predicate for BOTH the -d and the
# file form: no option parsing, so a future flag cannot slip a site past the check.
hits_mktemp(){
  local f
  while IFS= read -r f; do
    "$GREP" -nF '$(mktemp' "$f" | sed "s|^|$f:|"
  done < <(scope_files)
}

# TEMPLATE-required, deliberately NOT TMPDIR-required. Six in-scope sites are templated BESIDE
# their destination so the following mv is a same-filesystem atomic rename — a documented
# guarantee. A TMPDIR-required predicate would redden on those correct sites and push the next
# author into breaking that atomicity to get back to green.
offenders_mktemp(){ hits_mktemp | "$GREP" -vF 'XXXXXX'; }

bad_mktemp="$(offenders_mktemp)"
assert "every mktemp call passes a template, so TMPDIR is honored" \
  '[ -z "$bad_mktemp" ] || { echo "$bad_mktemp" | sed "s|^$ROOT/|  |" >&2; echo "  RULE: bare mktemp ignores TMPDIR on macOS, so a redirect meant to contain the scratch dir is a no-op and the debris lands outside it. Write: mktemp [-d] \"\${TMPDIR:-/tmp}/<script-name>.XXXXXX\" — or, when the temp file must sit beside its destination for an atomic rename, template it there instead." >&2; false; }'

n_tmpl="$(hits_mktemp | "$GREP" -cF 'XXXXXX')"
assert "at least $MIN_MKTEMP_TEMPLATED templated mktemp sites exist, so the check above is not vacuous (found $n_tmpl)" \
  '[ "$n_tmpl" -ge "$MIN_MKTEMP_TEMPLATED" ] || { echo "  RULE: this floor exists because a negative grep also passes when the scan finds nothing. A drop means either the scan broke or scratch files stopped being created — check which before touching this number." >&2; false; }'
```

- [ ] **Step 2: Run it to verify it fails**

Run: `bash tests/test_bsd_tool_defaults.sh`
Expected: FAIL — the template assert lists **22 lines** (23 call sites; `scripts/mint-stub.sh`'s two calls share one line), and the floor assert fails at 6 found versus 29 required. The six beside-destination sites must **not** appear in the offender list.

- [ ] **Step 3: Sweep every untemplated `mktemp`**

Re-derive first: `/usr/bin/grep -rnF '$(mktemp' scripts/*.sh scripts/lib/*.sh scripts/runners/*.sh install.sh sync-agents.sh migrate-to-docket.sh | /usr/bin/grep -vF XXXXXX`

Each edit replaces the bare call with the house form, `<script-name>` being that file's own basename sans `.sh`. Nothing else on the line changes:

```sh
# scripts/archive-change.sh
  local f="$1" k="$2" v="$3" t; t="$(mktemp "${TMPDIR:-/tmp}/archive-change.XXXXXX")"
# scripts/backfill-change-types.sh
stage="$(mktemp -d "${TMPDIR:-/tmp}/backfill-change-types.XXXXXX")"; trap 'rm -rf "$stage"' EXIT
# scripts/docket-config.sh   (two sites)
FETCH_ERR="$(mktemp "${TMPDIR:-/tmp}/docket-config.XXXXXX")" || die "could not create git-fetch diagnostic file"
CFG="$(mktemp "${TMPDIR:-/tmp}/docket-config.XXXXXX")"
# scripts/ensure-claude-settings.sh
tmp="$(mktemp "${TMPDIR:-/tmp}/ensure-claude-settings.XXXXXX")"
# scripts/mark-publish-deferred.sh
tmp="$(mktemp "${TMPDIR:-/tmp}/mark-publish-deferred.XXXXXX")" || die "mktemp failed"
# scripts/mint-stub.sh   (two sites; the second line holds TWO calls)
  local f="$1" k="$2" t; t="$(mktemp "${TMPDIR:-/tmp}/mint-stub.XXXXXX")" || return 1
  tmp="$(mktemp "${TMPDIR:-/tmp}/mint-stub.XXXXXX")"; tmp2="$(mktemp "${TMPDIR:-/tmp}/mint-stub.XXXXXX")"
# scripts/profile-asserts.sh
tmp="$(mktemp -d "${TMPDIR:-/tmp}/profile-asserts.XXXXXX")"
# scripts/profile-one-test.sh
tmp="$(mktemp -d "${TMPDIR:-/tmp}/profile-one-test.XXXXXX")"
# scripts/reclaim-claims.sh   (keep the trailing comment exactly as it is)
  local f="$1" k="$2" v="$3" t; t="$(mktemp "${TMPDIR:-/tmp}/reclaim-claims.XXXXXX")"     # clearing a field ⇒ VALUE="" (leaves "key: ").
# scripts/render-artifact-backlink.sh   (two sites)
block_file="$(mktemp "${TMPDIR:-/tmp}/render-artifact-backlink.XXXXXX")"; trap 'rm -f "$block_file"' EXIT
out="$(mktemp "${TMPDIR:-/tmp}/render-artifact-backlink.XXXXXX")"
# scripts/render-change-links.sh   (two sites)
block_file="$(mktemp "${TMPDIR:-/tmp}/render-change-links.XXXXXX")"; trap 'rm -f "$block_file"' EXIT
out="$(mktemp "${TMPDIR:-/tmp}/render-change-links.XXXXXX")"
# scripts/run-tests.sh
WORK="$(mktemp -d "${TMPDIR:-/tmp}/run-tests.XXXXXX")"; trap 'rm -rf "$WORK"' EXIT
# scripts/runners/codex.sh
last_msg="$(mktemp "${TMPDIR:-/tmp}/codex.XXXXXX")"
# scripts/terminal-publish.sh   (two sites; the second keeps its nested /pub suffix)
tmpd="$(mktemp -d "${TMPDIR:-/tmp}/terminal-publish.XXXXXX")"
pub="$(mktemp -d "${TMPDIR:-/tmp}/terminal-publish.XXXXXX")/pub"
# sync-agents.sh   (two sites)
  tmp="$(mktemp -d "${TMPDIR:-/tmp}/sync-agents.XXXXXX")"
  rule_tmp="$(mktemp "${TMPDIR:-/tmp}/sync-agents.XXXXXX")"
# migrate-to-docket.sh
  tmp="$(mktemp "${TMPDIR:-/tmp}/migrate-to-docket.XXXXXX")"; grep -F -x -v -- "$bare" "$GITIGNORE" | grep -F -x -v -- "$bare/" > "$tmp" || true
```

Note on `scripts/terminal-publish.sh`'s second site: the original is `pub="$(mktemp -d)/pub"` — the `/pub` suffix is **outside** the substitution and must stay outside it.

- [ ] **Step 4: Run the guard and the two most affected suites**

```bash
bash tests/test_bsd_tool_defaults.sh                 # expect: all ok, exit 0
bash tests/test_backfill_change_types.sh             # expect: unchanged pass
bash tests/test_mint_stub.sh                         # expect: unchanged pass
```

Expected: PASS on all three. `run-tests.sh` and `sync-agents.sh` were edited too — Task 4's full-suite run covers them.

- [ ] **Step 5: Mutation-test the new half**

```bash
cp scripts/run-tests.sh /tmp/run-tests.bak
sed -i '' 's|mktemp -d "${TMPDIR:-/tmp}/run-tests.XXXXXX"|mktemp -d|' scripts/run-tests.sh
bash tests/test_bsd_tool_defaults.sh; echo "exit=$?"   # expect: NOT OK on the template assert
cp /tmp/run-tests.bak scripts/run-tests.sh
bash tests/test_bsd_tool_defaults.sh; echo "exit=$?"   # expect: all ok, exit 0
```

Then confirm the predicate does **not** redden on a correct beside-destination site: `/usr/bin/grep -nF '$(mktemp' scripts/board-refresh.sh` prints a templated line with no `TMPDIR`, and the guard passes. That is the intended behavior, not a gap.

- [ ] **Step 6: Commit**

```bash
git add tests/test_bsd_tool_defaults.sh scripts/ sync-agents.sh migrate-to-docket.sh
git commit -m "fix(0254): template every mktemp so TMPDIR is honored, guarded repo-wide"
```

---

### Task 3: Behavioral pin — TMPDIR honored where a fixture already depends on it

**Files:**
- Modify: `tests/test_backfill_change_types.sh` (inside the `if chflags uchg …` rollback block, after the existing `rollback: …` asserts)

**Interfaces:**
- Consumes: Task 2's templated `stage="$(mktemp -d "${TMPDIR:-/tmp}/backfill-change-types.XXXXXX")"` in `scripts/backfill-change-types.sh`.
- Produces: nothing consumed downstream.

**Why this one site only:** the shape guard is the regression surface for all 23 sites. This fixture is the sole place that *already* redirects `TMPDIR` and documents containment as the reason, so without an assert here the fix is invisible to the suite. The `uchg`-blocked rollback copies survive the script's own `rm -rf` trap, so with the fix a remnant is left **inside** the fixture, where the existing `chflags -R nouchg "$tmp"` cleanup reaches it; without the fix it lands outside and leaks permanently.

- [ ] **Step 1: Write the failing assert**

In `tests/test_backfill_change_types.sh`, immediately after the existing assert `"rollback: no rollback-failure warning was emitted"` and still **inside** the `if chflags uchg …` branch, add:

```bash
  # TMPDIR-honoring, pinned behaviorally. The script's scratch dir holds the rollback copies, which
  # inherit the immutable flag, so its own `rm -rf` trap cannot remove it and a remnant survives.
  # A bare `mktemp -d` ignores TMPDIR on macOS, so that remnant would land OUTSIDE this fixture and
  # leak undeletably; templated, it lands under the redirect where the cleanup trap above reaches
  # it. Asserting the remnant is here is what makes the template load-bearing rather than cosmetic.
  assert "rollback: the script's scratch dir honored the redirected TMPDIR" \
    '[ -n "$(find "$drb/tmpdir" -maxdepth 1 -name "backfill-change-types.*" -print -quit)" ]'
```

- [ ] **Step 2: Run it against the pre-fix script to verify it fails**

```bash
cp scripts/backfill-change-types.sh /tmp/backfill.bak
sed -i '' 's|mktemp -d "${TMPDIR:-/tmp}/backfill-change-types.XXXXXX"|mktemp -d|' scripts/backfill-change-types.sh
bash tests/test_backfill_change_types.sh | grep 'redirected TMPDIR'
```

Expected: `NOT OK - rollback: the script's scratch dir honored the redirected TMPDIR`. If it prints `ok`, the assert is not actually keyed on the fix — stop and fix it before restoring.

If instead the line is absent entirely, this host could not set `chflags uchg` and the whole block was skipped; record that in the results file and move on — the assert is honest only where the branch runs.

- [ ] **Step 3: Restore the fix**

```bash
cp /tmp/backfill.bak scripts/backfill-change-types.sh
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `bash tests/test_backfill_change_types.sh`
Expected: PASS, including the new assert, with no leaked directory outside the fixture. Confirm cleanliness explicitly: `ls "${TMPDIR:-/tmp}" | grep -c '^backfill-change-types\.'` counts only remnants this run legitimately left under the *real* TMPDIR (expect no new ones attributable to the fixture).

- [ ] **Step 5: Commit**

```bash
git add tests/test_backfill_change_types.sh
git commit -m "test(0254): pin that backfill's scratch dir honors a redirected TMPDIR"
```

---

### Task 4: `AGENTS.md` rules, runtime budget registration, and the cp/rm re-audit

**Files:**
- Modify: `AGENTS.md` (the `## Shell` section)
- Modify: `tests/runtime-budgets.tsv` (one new row)
- Modify: `tests/test_runtime_budgets.sh` (`EXPECTED_TOTAL`)
- Possibly modify: `scripts/archive-change.md`, `scripts/board-refresh.md`, `scripts/run-tests.md`, `scripts/terminal-publish.md`, `scripts/cleanup-feature-branch.md` — **only if** a sentence there states tmp-dir or install behavior the sweep changed (expected: few or none)

**Interfaces:**
- Consumes: `tests/test_bsd_tool_defaults.sh` from Tasks 1–2 (it is the file needing a budget row).
- Produces: nothing consumed downstream.

- [ ] **Step 1: Add the two `AGENTS.md` rules**

Append to the `## Shell` bullet list in `AGENTS.md`, after the existing awk-indent bullet:

```markdown
- Non-interactive flags on tools that can prompt are load-bearing, not style: BSD `mv` on an
  unwritable destination with a tty prompts, self-answers `n` at EOF, and **exits 0**, so `|| die`
  guards are unreachable and the write is silently lost — always `mv -f` on install/replace paths
  (`git mv` excepted; there `-f` means force-overwrite a tracked target).
- Bare `mktemp` — with or without `-d` — ignores `TMPDIR` on macOS, so a redirect meant to contain
  a script's scratch dir is a no-op. Always pass a template: `"${TMPDIR:-/tmp}/<name>.XXXXXX"`,
  unless the temp file must sit **beside its destination** for a same-filesystem atomic rename, in
  which case template it there instead.
```

This is a direct rule addition through the PR gate, exactly like the existing `## Shell` bullets — **not** a learnings promotion, so no `promotion_state` field changes anywhere.

- [ ] **Step 2: Measure the new test file's runtime and register its budget**

`tests/test_runtime_budgets.sh` asserts that every `tests/test_*.sh` has a budget row and that the table's ceilings sum to `EXPECTED_TOTAL`. A new test file bringing its own row is one of the two cases that message names as legitimate.

```bash
scripts/run-tests.sh -j 1 --timings /tmp/0254-timings.tsv tests/test_bsd_tool_defaults.sh
```

**`--timings` takes a PATH argument.** Omitting it makes the runner consume the following test
file as the timings output path and **truncate that file to zero bytes** — write the destination
explicitly, as above.

Add a row to `tests/runtime-budgets.tsv`, tab-separated, in the file's existing sort position, using the measured value rounded up the way neighboring rows are (the comparable shape-scan guard `tests/test_grep_portability.sh` is budgeted at 10s):

```
tests/test_bsd_tool_defaults.sh	10	parallel
```

Then raise `EXPECTED_TOTAL` in `tests/test_runtime_budgets.sh` by exactly the ceiling you added — `1355` → `1365` if you used 10. Use the measured number, not this one, if they differ.

- [ ] **Step 3: Verify the budget guard passes**

```bash
bash tests/test_runtime_budgets.sh
```
Expected: PASS, including `every tests/test_*.sh has a budget row` and the total assert.

- [ ] **Step 4: Re-run the cp/rm audit and check the sibling docs**

Two different BSD shapes: `cp` prompts only under `-i`; `rm` prompts **without** `-f` on a write-protected target with a tty.

```bash
/usr/bin/grep -rnE '(^|[^-[:alnum:]])cp -[a-zA-Z]*i' scripts/*.sh scripts/lib/*.sh \
  scripts/runners/*.sh install.sh sync-agents.sh migrate-to-docket.sh
/usr/bin/grep -rnE '(^|[^-[:alnum:]])rm ' scripts/*.sh scripts/lib/*.sh scripts/runners/*.sh \
  install.sh sync-agents.sh migrate-to-docket.sh | /usr/bin/grep -vE 'rm -[a-zA-Z]*f'
```

Expected: zero `cp -i` sites; the `rm` grep returns only known chaff — two `git rm` invocations (one inside a constructed remedy string in `sync-agents.sh`, one in `migrate-to-docket.sh`) plus comment mentions. **Anything beyond that chaff is a finding** — surface it rather than silently fixing it. No code change either way; record the result in the results file.

Then check whether any script's sibling `.md` states behavior the sweep changed:

```bash
/usr/bin/grep -n 'mktemp\|TMPDIR' scripts/*.md
```

Update only a sentence that is now false. A doc that merely mentions a temp file without asserting where it lands needs no edit.

- [ ] **Step 5: Run the full suite**

Run: `scripts/run-tests.sh`
Expected: fully green. This is the gate for the whole branch — 21 shipped scripts were edited, and their own suites are what prove the sweep was mechanical.

- [ ] **Step 6: Commit**

```bash
git add AGENTS.md tests/runtime-budgets.tsv tests/test_runtime_budgets.sh scripts/
git commit -m "docs(0254): promote the mv -f and templated-mktemp rules; budget the new guard"
```

---

## Self-Review

**Spec coverage.** §1 templated mktemp → Task 2. §2 `mv -f` sweep with the `git mv` carve-out → Task 1. §3 the shape-keyed guard, its pinned grep, split patterns, `$GIT … mv` allowance, template-required predicate, population floors, rule-stating failure text, no `file:line` anchors → Tasks 1–2. §4 the behavioral TMPDIR pin → Task 3. §5 the two `AGENTS.md` bullets → Task 4 Step 1. §6 the cp/rm re-audit → Task 4 Step 4. §7 sibling docs → Task 4 Step 4. Assumptions A1–A10 are all encoded as Global Constraints or task steps; A9's 0118 coupling needs no task (compose at rebase). No gaps.

**Placeholder scan.** Every code step carries the literal text to write; every site list is enumerated and paired with a re-derivation command; every expected test output names the count or the message. No TBDs.

**Type consistency.** `scope_files()`, `GREP`, `assert`, `hits_mv()`/`offenders_mv()`, `hits_mktemp()`/`offenders_mktemp()`, `MIN_FILES`, `MIN_MV_F`, `MIN_MKTEMP_TEMPLATED` are spelled identically in Tasks 1 and 2. Task 2 explicitly reuses Task 1's scan helpers rather than writing a second implementation, so neutering the scan neuters it everywhere — a control cannot stay green while the loop goes blind.

**One caution for the implementer.** The `n_mv_f_sites` count in Task 1 passes `$(scope_files)` unquoted into `grep`. That is deliberate word-splitting over a path list with no spaces in it; if any in-scope path ever gains a space, convert those two lines to a `while IFS= read -r` loop like `hits_mv`. Do not "fix" it by quoting — that would pass a single glued argument and the count would silently collapse to zero.
