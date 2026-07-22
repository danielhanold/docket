# Line-number comment anchors — conversion, guard, ADR — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Convert every line-number cross-reference in the repo's maintained source to a symbol-name or verbatim-quoted-clause anchor, and add a partial suite guard that mechanically prevents the one anchor form that can be matched false-positive-free from coming back.

**Architecture:** Three conversion tasks (scripts/root, then tests/), then one task adding `tests/test_comment_anchor_style.sh` plus the authoring rule in `AGENTS.md`, then one task minting the ADR and repairing ADR-0044's own stale anchor via the append-only `## Update` path. Every commit leaves the suite green: the guard lands *after* the conversions it enforces, and earns its red evidence by being run against the pre-conversion base commit in a throwaway worktree.

**Tech Stack:** bash 3.2-compatible shell, `git grep`, the repo's hand-rolled `assert`-style test harness (`tests/test_*.sh`, run via `for t in tests/test_*.sh; do bash "$t"; done`). No CI — the suite is the de-facto gate.

## Global Constraints

- **Comment-only in `scripts/`.** No behavioral change to any script. `scripts/docket-config.sh` in particular changes only comment text (spec *Out of scope*).
- **`tests/test_docket_config.sh` stays byte-identical outside its anchor comments** — the 0106 fixtures are mutation-verified (spec *Out of scope*).
- **Surfaces in scope:** `scripts/`, `tests/`, `skills/`, `agents/`, `cursor-rules/`, root `*.md` / `*.yml`.
- **Surfaces excluded, structurally:** `docs/adrs/`, `docs/results/`, `docs/changes/archive/`, `docs/superpowers/specs/`, `docs/changes/active/`. Do not edit them and do not let the guard walk them.
- **No allowlist** anywhere in the guard. Exclusions are by walk scope, never by exception entry.
- **The anchor idiom:** a cross-reference anchors on a **symbol name** or a **verbatim-quoted clause**, never a line number.
- **Shell floor: bash 4.0.** `mapfile` is fine — `render-board.sh`, `github-mirror.sh`, `render-adr-index.sh` and `tests/test_render_board.sh` already use it. Avoid constructs newer than 4.0 (the repo's own note in `board-checks.sh` is "bash-4.0-safe; no unset arr[-1]").
- Existing repo rules in `AGENTS.md` apply, notably: never `producer | grep -q` under `pipefail`; a guard is code and must be mutation-tested.

### Verified environment facts (checked against the running tree — do not re-derive, do not trust contrary memory)

These were measured during planning and are load-bearing for Task 3:

1. **`git grep -E` does NOT support `\b` or `\<`.** Both return **zero** matches silently. The guard must use a bracket class instead.
2. **`git grep` output is `path:lineno:content`, and `path` ends in `.sh:`/`.md:` — which is exactly the shape the guard hunts.** Filtering `git grep` output with the anchor pattern therefore matches the *prefix* on essentially every line. **The guard must scan each file with per-file `grep -n` (output `lineno:content`, no path), never with `git grep`'s path-prefixed output.** This trap cost a full debugging cycle during planning.
3. The harness's interactive shell shadows `grep`; inside test files use plain `grep` (tests run under `bash`), but any ad-hoc verification you run yourself must use `command grep` or `git grep` ([[agent-shell-noop-reads-as-success]]).

**Pre-verified against the running tree during planning** (do not re-derive; if any disagrees, the base moved — STOP and report):

| Value | Measured |
|---|---|
| Explicit-file anchors on the in-scope surface | **26** |
| Walk population (`git ls-files`, docs/ excluded) | **144** files |
| Probes present in the walk | `scripts/board-checks.sh`, `tests/test_board_checks.sh`, `AGENTS.md`, `.docket.example.yml` — all 4 |
| Positive control (a real anchor in a temp file) | pattern **fires** |
| Negative control (bash array slice, JSON timestamp, plain prose) | **not** flagged |
| Bare-form anchors to convert (unguarded) | 13 real, 8 false positives (~38% FP — matches the spec's measurement) |
| Prose-form anchors to convert (unguarded) | 2 real, 3 false positives (60% FP — matches the spec) |

---

## File Structure

**Modified (conversions):**
- `.docket.example.yml` — 2 explicit anchors
- `scripts/board-checks.sh` — 5 explicit + 3 bare
- `scripts/docket-config.sh` — 1 explicit + 1 bare line
- `scripts/docket-config.md` — 1 explicit + 1 bare line
- `scripts/github-mirror.md` — 2 explicit
- `scripts/lib/docket-frontmatter.sh` — 1 bare line
- `scripts/docket-status.md` — 1 prose
- `tests/test_board_checks.sh` — 5 explicit + 4 bare
- `tests/test_docket_config.sh` — 3 explicit
- `tests/test_docket_example_yml.sh` — 7 explicit + 3 bare
- `tests/test_finalize_disposition.sh` — 1 prose

**Created:**
- `tests/test_comment_anchor_style.sh` — the guard
- `docs/adrs/0054-comment-cross-references-anchor-on-symbols-not-line-numbers.md` — the ADR (created by the docket-adr subagent in Task 5, not by hand)

**Modified (docs):**
- `AGENTS.md` — the authoring rule
- `docs/adrs/0044-autonomy-precedence-call-site-pre-specification.md` — append-only `## Update`

---

### Task 1: Convert the `scripts/` and root-level anchors

**Files:**
- Modify: `.docket.example.yml`, `scripts/board-checks.sh`, `scripts/docket-config.sh`, `scripts/docket-config.md`, `scripts/github-mirror.md`, `scripts/lib/docket-frontmatter.sh`, `scripts/docket-status.md`
- Test: the existing suite (no new test in this task)

**Interfaces:**
- Consumes: nothing.
- Produces: a `scripts/` + root surface with zero explicit-file anchors. Task 3's guard asserts exactly this.

**Every replacement below is comment/prose text only. Do not change any executable line.**

- [ ] **Step 1: Record the pre-conversion baseline** (this is the evidence Task 3's red-run is checked against)

```bash
cd "$(git rev-parse --show-toplevel)"
git rev-parse HEAD > /tmp/anchor-base-sha.txt
bash -c 'git grep -nE "[A-Za-z0-9_-]+\.(sh|md|yml|yaml|mdc):[0-9]+" -- scripts tests skills agents cursor-rules ":(glob)*.md" ":(glob)*.yml" | wc -l'
```

Expected: `26`. If it is not 26, STOP and report — the base moved and the plan's site list needs re-derivation.

- [ ] **Step 2: `.docket.example.yml` — 2 anchors**

Replace (around the `integration_branch` block):

```
# origin, or startup fails closed rather than guessing (the bootstrap guard's ls-tree probe reads
# it, docket-config.sh:369-381); in main-mode the resolver does not validate the ref.
```

with:

```
# origin, or startup fails closed rather than guessing (docket-config.sh's Stage 3 bootstrap guard
# reads it via `g ls-tree "origin/$INTEGRATION_BRANCH"`, which dies on a non-zero exit rather than
# reading it as ¬LIVE); in main-mode the resolver does not validate the ref.
```

Replace (in the `github_project` note):

```
#   (docket-config.sh:170), which warns-and-ignores it in the machine-scoped layers. Enabling
```

with:

```
#   (docket-config.sh's `for _fkey in metadata_branch integration_branch …` fence loop), which
#   warns-and-ignores it in the machine-scoped layers. Enabling
```

- [ ] **Step 3: `scripts/board-checks.sh` — 5 explicit + 3 bare**

Replace:

```
# message` (docket-status.sh:627), so an interior TAB in ANY embedded value shifts every later
```

with:

```
# message` (docket-status.sh's `health_checks`), so an interior TAB in ANY embedded value
# shifts every later
```

Replace the `renders_row` header's two numbered clauses:

```
#   1. render-board.sh:76  `id="$(int_field "$f" id)"; [ -n "$id" ] || continue`
#      — a file with no usable integer id never enters SECTION at all.
#   2. render-board.sh:265-269 calls print_section for exactly the DOCKET_STATUSES_ACTIVE members,
#      and :78 buckets on the RAW `status:` read — so a status outside that set lands in a SECTION
```

with:

```
#   1. render-board.sh's id gate, `id="$(int_field "$f" id)"; [ -n "$id" ] || continue`
#      — a file with no usable integer id never enters SECTION at all.
#   2. render-board.sh calls print_section for exactly the DOCKET_STATUSES_ACTIVE members, and
#      buckets on the RAW `status:` read (`SECTION["$st"]+="$id"…`) — so a status outside that set
#      lands in a SECTION
```

Replace:

```
#      move failed). :86 still counts the file in `total`, so the count line and the tables disagree.
```

with:

```
#      move failed). `total=${#AFILES[@]}` still counts the file, so the count line and the tables
#      disagree.
```

Replace:

```
      # EXPLAINED: a non-integer id is a genuine drop CAUSE (render-board.sh:76 skips the row), so
```

with:

```
      # EXPLAINED: a non-integer id is a genuine drop CAUSE (render-board.sh's `[ -n "$id" ] ||
      # continue` skips the row), so
```

Replace:

```
  # archive table renders from its own pass, :297+, and is not subject to this invariant).
```

with:

```
  # archive table renders from its own pass, under the `# --- archive ---` section, and is not
  # subject to this invariant).
```

Replace:

```
  # slugify's own alphabet (mint-stub.sh:88-91). Empty fails — slug has no documented default.
```

with:

```
  # slugify's own alphabet (mint-stub.sh's `slugify()`). Empty fails — slug has no documented
  # default.
```

- [ ] **Step 4: `scripts/docket-config.sh` — 1 explicit + 1 bare (comment only)**

Replace:

```
  # absent from the SHELL format: ensure-claude-settings.sh:24 sets its own REPO_ROOT and eval's
  # the shell export at :33, reading it at :38/:74 — emitting it there would silently capture that
```

with:

```
  # absent from the SHELL format: ensure-claude-settings.sh sets its own REPO_ROOT (from
  # `git rev-parse --show-toplevel`) and eval's the shell export, reading it back later —
  # emitting it there would silently capture that
```

- [ ] **Step 5: `scripts/docket-config.md` — 1 explicit + 1 bare**

Replace:

```
the `shell` format**: `scripts/ensure-claude-settings.sh:24` sets its own `REPO_ROOT` variable
and `eval`s the shell export at `:33`, reading it back at `:38` and `:74` — emitting a
```

with:

```
the `shell` format**: `scripts/ensure-claude-settings.sh` sets its own `REPO_ROOT` variable from
`git rev-parse --show-toplevel` and `eval`s the shell export, reading it back later — emitting a
```

- [ ] **Step 6: `scripts/github-mirror.md` — 2 explicit (one of them STALE)**

The `docket-status.sh:272` anchor is **stale** — line 272 is a `printf`/brace region. Repoint it to what the prose *means*: the arg-parse and the invocation.

Replace:

```
`docket-status.sh` populates those only from its own CLI flags (`docket-status.sh:272`), which no
skill passes. The key's only live effect anywhere is the coordination-key fence in
`docket-config.sh:170`, which warns-and-ignores it in the two machine-scoped layers. When the
```

with:

```
`docket-status.sh` populates those only from its own CLI flags — the `--project) PROJECT_FLAG="$2"`
arg-parse arm, forwarded through `${PROJECT_FLAG:+--project "$PROJECT_FLAG"}` — which no skill
passes. The key's only live effect anywhere is the coordination-key fence in `docket-config.sh`'s
`for _fkey in metadata_branch integration_branch …` loop, which warns-and-ignores it in the two
machine-scoped layers. When the
```

- [ ] **Step 7: `scripts/lib/docket-frontmatter.sh` — 1 bare line (both numbers STALE)**

`:52` is now 54 and `:71` is now 73 — both drifted under change 0111. Convert rather than re-number.

Replace:

```
# can drift from what bash actually assigns. This lib IS sourceable (board-checks.sh sources it at
# :52, well before emit() at :71), so tests/test_board_checks.sh reads the real runtime array.
```

with:

```
# can drift from what bash actually assigns. This lib IS sourceable (board-checks.sh's
# `source "$(dirname "${BASH_SOURCE[0]}")/lib/docket-frontmatter.sh"` runs well before its
# `emit()` definition), so tests/test_board_checks.sh reads the real runtime array.
```

- [ ] **Step 8: `scripts/docket-status.md` — 1 prose anchor (note the EN-DASH)**

Replace:

```
| `-h`, `--help` | Print the usage synopsis (script header lines 2–19) and exit 0. |
```

with:

```
| `-h`, `--help` | Print the usage synopsis (the script's leading `# scripts/docket-status.sh —` header comment block) and exit 0. |
```

- [ ] **Step 9: Verify this task's surface is clean and nothing executable moved**

```bash
cd "$(git rev-parse --show-toplevel)"
bash -c 'git grep -nE "[A-Za-z0-9_-]+\.(sh|md|yml|yaml|mdc):[0-9]+" -- scripts ":(glob)*.md" ":(glob)*.yml"'
```

Expected: **no output** (all `scripts/` + root explicit anchors converted).

```bash
git diff --stat
git diff -U0 -- scripts/docket-config.sh scripts/board-checks.sh scripts/lib/docket-frontmatter.sh | grep -E '^\+' | grep -vE '^\+\+\+' | grep -vE '^\+\s*#' && echo "NON-COMMENT CHANGE PRESENT — STOP" || echo "comment-only: OK"
```

Expected: `comment-only: OK`. Any non-comment added line in a `.sh` file is a Global-Constraint violation — STOP and report.

- [ ] **Step 10: Run the full suite**

```bash
cd "$(git rev-parse --show-toplevel)"
fail=0; for t in tests/test_*.sh; do bash "$t" >/tmp/$(basename "$t").out 2>&1 || { echo "FAIL: $t"; fail=1; }; done; echo "suite fail=$fail"
```

Expected: `suite fail=0`.

- [ ] **Step 11: Commit**

```bash
git add .docket.example.yml scripts/
git commit -m "docs(0114): anchor scripts/ + root cross-references on symbols, not line numbers

Converts 11 explicit-file and 5 bare line-number anchors. Three were stale:
github-mirror.md's docket-status.sh:272 (now a printf region), board-checks.sh's
:297+ archive pointer, and docket-frontmatter.sh's :52/:71 (both drifted +2
under change 0111). Comment/prose text only — no executable line changes."
```

---

### Task 2: Convert the `tests/` anchors

**Files:**
- Modify: `tests/test_board_checks.sh`, `tests/test_docket_config.sh`, `tests/test_docket_example_yml.sh`, `tests/test_finalize_disposition.sh`
- Test: the existing suite

**Interfaces:**
- Consumes: Task 1's converted `scripts/` surface (some of these comments reference lines Task 1 rewrote — anchor on the NEW symbol text).
- Produces: a `tests/` surface with zero explicit-file anchors. Task 3's guard asserts this.

**Note:** `tests/test_docket_example_yml.sh` carries anchors inside **live `assert` description strings**, not only comments. Those descriptions are the assert's human-readable name; converting them changes test output text but not behavior. Do **not** change the assert's shell condition.

- [ ] **Step 1: `tests/test_board_checks.sh` — 5 explicit + 4 bare**

Replace:

```
# line; it silently missed every `cond || emit ...` call (board-checks.sh:197 and :206 — the
```

with:

```
# line; it silently missed every `cond || emit ...` call (the `broken-spec` and field-loop sites in
# board-checks.sh — the
```

Replace:

```
# board-checks.sh:11-13 — the set spans three comment lines, so the extraction joins them before
```

with:

```
# board-checks.sh's `check-id ∈ {…}` header enumeration — the set spans three comment lines, so
# the extraction joins them before
```

Replace:

```
# `sort -u`'d, which comm requires. Matches tests/test_docket_facade.sh:148's exact-set idiom for
```

with:

```
# `sort -u`'d, which comm requires. Matches the exact-set idiom in tests/test_docket_facade.sh's
# "docket.sh op set == docket.md documented op set" assert, for
```

Replace:

```
# tests/test_render_board.sh:1883-1885 uses for DOCKET_STATUSES, and it deletes a whole class of
```

with:

```
# tests/test_render_board.sh uses for DOCKET_STATUSES (its `source "$LIB"` of
# lib/docket-frontmatter.sh), and it deletes a whole class of
```

Replace:

```
# mutating board-checks.sh:181's `emit field-domain` to `emit "$dyn"` holds the set at 12 and
```

with:

```
# mutating board-checks.sh's slug-alphabet `emit field-domain` call to `emit "$dyn"` holds the set
# at 12 and
```

Replace (bare, in the S0 header):

```
# vocabulary is declared in the lib that board-checks.sh already sources at :52. That lets this
```

with:

```
# vocabulary is declared in the lib that board-checks.sh already sources near the top of the file.
# That lets this
```

Replace (bare):

```
# `total` (:86) and calls print_section only for the five ACTIVE statuses (:265-269), so the row is
```

with:

```
# `total` (`total=${#AFILES[@]}`) and calls print_section only for the five ACTIVE statuses, so the
# row is
```

Replace (bare — `:94` is stale, the prose is now at 96):

```
# precedes it on the line, and does NOT match the English "emit a table row" prose comment on :94
```

with:

```
# precedes it on the line, and does NOT match the English "emit a table row" prose comment in the
# `renders_row` header
```

Replace (bare — same stale `:94`):

```
# Comments are stripped first so the header's prose (`emit a table row`, :94) is out of scope; it
```

with:

```
# Comments are stripped first so the header's prose (`emit a table row`, in the `renders_row`
# header) is out of scope; it
```

- [ ] **Step 2: `tests/test_docket_config.sh` — 3 explicit (anchor comments ONLY)**

Global Constraint: this file stays byte-identical outside these three comment lines.

Replace:

```
# The collapse at scripts/docket-config.sh:202 runs AFTER the :195 resolution chain. That
```

with:

```
# The collapse (`[ "$FINALIZE_TEST_COMMAND" = auto ] && FINALIZE_TEST_COMMAND=""`) runs AFTER the
# three-rung `lcl` → committed → `gbl` resolution chain for finalize.test_command. That
```

Replace:

```
# the fence loop at scripts/docket-config.sh:170.
```

with:

```
# the coordination-key fence loop, `for _fkey in metadata_branch integration_branch …`.
```

Replace:

```
# scripts/docket-config.sh:215-218 before Stage 3 ever runs) but, before this pair, unproven.
```

with:

```
# scripts/docket-config.sh's `finalize.require_pr_approval must be 'true' or 'false'` validation,
# before Stage 3 ever runs) but, before this pair, unproven.
```

- [ ] **Step 3: `tests/test_docket_example_yml.sh` — 7 explicit + 3 bare**

Replace:

```
#   _filtered; docket-config.sh:242-266). No `BOARD_SURFACES=` assignment line ever contains the
```

with:

```
#   _filtered; docket-config.sh's board_surfaces layer-resolution block). No `BOARD_SURFACES=`
#   assignment line ever contains the
```

Replace:

```
# has ever satisfied — it was vacuous on day one and stayed green under both a revert of SKILL.md:108
```

with:

```
# has ever satisfied — it was vacuous on day one and stayed green under both a revert of the
# finalize SKILL's sole-channel sentence
```

Replace:

```
# (finding 2) Anchored on the PROVENANCE clause at SKILL.md:120 — the sentence that actually tells
```

with:

```
# (finding 2) Anchored on the PROVENANCE clause in the finalize SKILL — the `require_pr_approval`
# sentence naming FINALIZE_REQUIRE_PR_APPROVAL as "the sole channel" — the sentence that actually
# tells
```

Replace:

```
# anywhere" check. FINALIZE_REQUIRE_PR_APPROVAL also appears at SKILL.md:108's config-block framing
```

with:

```
# anywhere" check. FINALIZE_REQUIRE_PR_APPROVAL also appears in the SKILL's "Every value below is
# read from the Step-0 `preflight` export block" framing
```

Replace the **assert description** (leave its condition untouched):

```
assert "0102: the finalize skill's provenance clause (SKILL.md:120) ties FINALIZE_REQUIRE_PR_APPROVAL to the Step-0 export block as the sole channel" \
```

with:

```
assert "0102: the finalize skill's provenance clause (the 'sole channel' sentence) ties FINALIZE_REQUIRE_PR_APPROVAL to the Step-0 export block" \
```

Replace:

```
# (finding 1a) Positive framing: SKILL.md:108 states the sole-channel rule as "never by parsing
```

with:

```
# (finding 1a) Positive framing: the SKILL's export-block sentence states the sole-channel rule as
# "never by parsing
```

Replace the **assert description** (leave its condition untouched):

```
assert "0102: the finalize skill states its sole channel positively (never by parsing .docket.yml, SKILL.md:108)" \
```

with:

```
assert "0102: the finalize skill states its sole channel positively (never by parsing .docket.yml)" \
```

Replace (bare):

```
# sentence, so an existence-anywhere grep stays green even if this :120 clause is deleted outright;
```

with:

```
# sentence, so an existence-anywhere grep stays green even if this provenance clause is deleted
# outright;
```

Replace (bare):

```
# .docket.yml", tied to the exported keys it names. Reverting :108 back to its pre-0102 framing
```

with:

```
# .docket.yml", tied to the exported keys it names. Reverting the export-block sentence back to its
# pre-0102 framing
```

Replace (bare, two lines):

```
# example's prose comments (`# exceptions:` :22, `# scope: any layer` in many places, `# line:`
# :179), which would silently accept anything.
```

with:

```
# example's prose comments (`# exceptions:`, `# scope: any layer` in many places, `# line:`),
# which would silently accept anything.
```

- [ ] **Step 4: `tests/test_finalize_disposition.sh` — 1 prose anchor (STALE)**

"line 54" cites the delegation, but line 54 asserts *naming the ids IS the authorization*; the delegation assert is elsewhere. Repoint to meaning.

Replace:

```
# CONFLICTING DEPRIORITIZES, it is not excluded — line 54's delegation to the rebase-retest gate
```

with:

```
# CONFLICTING DEPRIORITIZES, it is not excluded — the "keeps conflict resolution delegated to the
# rebase-resolver" assert's delegation to the rebase-retest gate
```

- [ ] **Step 5: Verify the whole in-scope surface is now clean**

```bash
cd "$(git rev-parse --show-toplevel)"
bash -c 'git grep -nE "[A-Za-z0-9_-]+\.(sh|md|yml|yaml|mdc):[0-9]+" -- scripts tests skills agents cursor-rules ":(glob)*.md" ":(glob)*.yml"'
```

Expected: **no output** — zero explicit anchors across every in-scope surface. This is the state Task 3's guard will enforce.

- [ ] **Step 6: Run the full suite**

```bash
fail=0; for t in tests/test_*.sh; do bash "$t" >/tmp/$(basename "$t").out 2>&1 || { echo "FAIL: $t"; fail=1; }; done; echo "suite fail=$fail"
```

Expected: `suite fail=0`. `test_docket_example_yml.sh` and `test_board_checks.sh` must still pass — if an assert description edit broke a self-referential check, fix the check, not by reverting to a line number.

- [ ] **Step 7: Commit**

```bash
git add tests/
git commit -m "docs(0114): anchor tests/ cross-references on symbols, not line numbers

Converts 15 explicit-file and 7 bare/prose anchors across four test files.
Two more were stale: test_board_checks.sh's :94 prose pointer (now 96) and
test_finalize_disposition.sh's 'line 54' delegation cite (line 54 asserts
authorization, not delegation). Assert descriptions changed where they carried
an anchor; no assert condition was touched."
```

---

### Task 3: Add the guard and the authoring rule

**Files:**
- Create: `tests/test_comment_anchor_style.sh`
- Modify: `AGENTS.md`
- Test: the guard is itself the test; it is mutation-verified in both directions below.

**Interfaces:**
- Consumes: the zero-anchor state produced by Tasks 1–2.
- Produces: `tests/test_comment_anchor_style.sh`, enforcing the explicit-file form across the in-scope surfaces.

**Design constraints, all load-bearing (see *Verified environment facts*):**
- **Never use `\b` or `\<`** — unsupported, silently zero.
- **Never filter `git grep`'s path-prefixed output with the anchor pattern** — the path prefix self-matches. Scan per-file with `grep -n`.
- **Exclude the guard from its own walk structurally**, by `BASH_SOURCE` basename — never an allowlist entry.
- **Assert the population**, not only the absence of violations: a guard walking zero files is green and worthless ([[backstop-must-compute-not-reenumerate]], [[marker-scoped-guard-needs-a-population-floor]]).

- [ ] **Step 1: Write the guard**

Create `tests/test_comment_anchor_style.sh`:

```bash
#!/usr/bin/env bash
# tests/test_comment_anchor_style.sh — cross-references in MAINTAINED SOURCE anchor on a symbol
# name or a verbatim-quoted clause, never on a line number (change 0114, ADR-0054).
#
# PARTIAL BY DESIGN. This guard enforces exactly ONE anchor form: the explicit-file form, a
# filename with a source extension followed by a colon and a line number. That is the only
# predicate measurable without false positives (26/26 true anchors at conversion time). The two
# other forms were converted by hand and are deliberately NOT guarded, because neither can be
# matched cleanly:
#   - the bare colon-number form measured ~38% false positives (bash array slices such as
#     "${PATH_STACK[@]:0:...}" and JSON fixtures such as "p10":{...} are indistinguishable
#     without parsing), and tightening it introduces a false NEGATIVE on real anchors;
#   - the prose "line N" form measured 60% false positives (test fixtures legitimately discuss
#     "line 2" of a constructed input) and would additionally have to match an en-dash range.
# Those rest on the AGENTS.md authoring rule plus review — where this repo already puts claims it
# cannot mechanically check (ADR-0031 bounds source-syntax scanning; ADR-0050 shapes this guard).
#
# SCOPE: maintained source only. docs/adrs/ is excluded because an Accepted ADR is immutable
# except its status: line, so a guard cannot demand a repair the convention forbids;
# docs/results/, docs/changes/archive/ and docs/superpowers/specs/ are immutable point-in-time
# records; docs/changes/active/ lives on the docket metadata branch and is absent from the
# integration-branch checkout this suite runs in, so there is no such path to walk.
# NO ALLOWLIST: exclusions are by walk scope, never by exception entry (ADR-0050, enumerated-floor).
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SELF="$(basename "${BASH_SOURCE[0]}")"
fail=0
ok(){   printf 'ok   - %s\n' "$1"; }
nok(){  printf 'NOT OK - %s\n' "$1"; fail=1; }

# The explicit-file anchor: <name>.<source-ext> immediately followed by :<digits>.
# NOTE: no \b / \< anywhere — git grep's ERE does not support them and returns zero silently.
ANCHOR='[A-Za-z0-9_-]+\.(sh|md|yml|yaml|mdc):[0-9]+'

# --- collect the in-scope population ------------------------------------------------------------
# git ls-files, NOT git grep: git grep prefixes every hit with "path:lineno:", and that path ends
# in ".sh:"/".md:" — the exact shape ANCHOR matches. Filtering git grep output would therefore
# match the tool's own prefix on every line. Each file is scanned separately with grep -n, whose
# "lineno:content" prefix cannot collide with ANCHOR (no extension precedes the colon).
mapfile -t FILES < <(
  cd "$ROOT" || exit 1
  git ls-files -- scripts tests skills agents cursor-rules ':(glob)*.md' ':(glob)*.yml' \
    | grep -vE '^docs/'
)

# --- population floor: the walk must actually reach files ---------------------------------------
# A guard iterating an empty list is green and proves nothing. Assert a non-trivial count AND the
# presence of specific known in-scope files, so a broken pathspec or a bad ls-files invocation
# reddens instead of passing vacuously.
n_files=${#FILES[@]}
[ "$n_files" -ge 40 ] \
  && ok "walk population is non-trivial ($n_files files)" \
  || nok "walk population collapsed to $n_files files (expected >= 40) — pathspec or ls-files broke"

for probe in scripts/board-checks.sh tests/test_board_checks.sh AGENTS.md .docket.example.yml; do
  printf '%s\n' "${FILES[@]}" | grep -qxF "$probe" \
    && ok "walk includes $probe" \
    || nok "walk MISSES $probe — the in-scope surface is not fully covered"
done

# --- the check ----------------------------------------------------------------------------------
violations=""
scanned=0
for f in "${FILES[@]}"; do
  [ "$(basename "$f")" = "$SELF" ] && continue   # structural self-exclusion; never an allowlist
  [ -f "$ROOT/$f" ] || continue
  scanned=$(( scanned + 1 ))
  hits="$(grep -nE "$ANCHOR" "$ROOT/$f" 2>/dev/null)"
  [ -n "$hits" ] && violations+="$(printf '%s\n' "$hits" | sed "s|^|$f:|")"$'\n'
done

[ "$scanned" -ge 40 ] \
  && ok "scanned $scanned files (guard self-excluded)" \
  || nok "scanned only $scanned files — the scan loop is not reaching the population"

if [ -z "$violations" ]; then
  ok "no line-number cross-reference anchors in maintained source"
else
  nok "line-number cross-reference anchors found — anchor on a symbol name or a quoted clause instead:"
  printf '%s' "$violations" | sed 's/^/       /'
fi

# --- positive control: prove the predicate FIRES ------------------------------------------------
# Without this, every assert above is consistent with a pattern that can never match anything.
# Mutate a throwaway copy so the drift is really present, and assert it is reported.
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
printf '# see render-board.sh:76 for the id gate\n' > "$tmp/probe.sh"
grep -qE "$ANCHOR" "$tmp/probe.sh" \
  && ok "positive control: the anchor pattern reports a real explicit-file anchor" \
  || nok "positive control FAILED: the anchor pattern matches nothing — the guard is vacuous"

# Negative control: the forms this guard deliberately does NOT claim to catch, plus the shapes
# that must never be flagged. Pins the FP-free property the partial scope rests on.
printf '%s\n' \
  'PATH_STACK=("${PATH_STACK[@]:0:${#PATH_STACK[@]}-1}")' \
  '{"data":{"p10":{"number":101,"mergedAt":"2026-07-05T18:22:31Z"}}}' \
  '# the archive table renders from its own pass' \
  > "$tmp/clean.sh"
grep -qE "$ANCHOR" "$tmp/clean.sh" \
  && nok "negative control FAILED: the pattern flags a bash array slice or JSON timestamp" \
  || ok "negative control: array slices and JSON timestamps are not flagged"

exit "$fail"
```

- [ ] **Step 2: Run the guard — expect GREEN on the converted tree**

```bash
cd "$(git rev-parse --show-toplevel)"
bash tests/test_comment_anchor_style.sh; echo "exit=$?"
```

Expected: every line `ok   - …`, `exit=0`. In particular the population lines must report a real count, not 0.

- [ ] **Step 3: Earn the red evidence — run the guard against the PRE-conversion base**

The guard must be shown to catch the 26 real anchors it was written for, not merely to pass on a tree that has none.

```bash
cd "$(git rev-parse --show-toplevel)"
BASE="$(cat /tmp/anchor-base-sha.txt)"
git worktree add /tmp/anchor-base "$BASE"
cp tests/test_comment_anchor_style.sh /tmp/anchor-base/tests/
cd /tmp/anchor-base && bash tests/test_comment_anchor_style.sh; echo "exit=$?"
```

Expected: `NOT OK - line-number cross-reference anchors found`, `exit=1`, and the listed violations number **26**. Count them:

```bash
cd /tmp/anchor-base && bash tests/test_comment_anchor_style.sh 2>&1 | grep -cE '^       [^ ]+\.(sh|md|yml):[0-9]+:'
```

Expected: `26`. Then clean up:

```bash
cd "$(git rev-parse --show-toplevel)" && git worktree remove --force /tmp/anchor-base
```

If the count is not 26, the guard's population or pattern disagrees with the measured baseline — STOP and reconcile before continuing.

- [ ] **Step 4: Mutation-test the guard in both directions**

Each mutation is applied, the guard run, then reverted.

```bash
cd "$(git rev-parse --show-toplevel)"

# M1 — reintroduce a real anchor into a live in-scope file: must REDDEN.
printf '\n# see render-board.sh:76 for the id gate\n' >> scripts/board-checks.sh
bash tests/test_comment_anchor_style.sh >/dev/null 2>&1; echo "M1 (expect 1): $?"
git checkout scripts/board-checks.sh

# M2 — break the population (empty the walk): must REDDEN, not pass vacuously.
sed -i.bak 's|git ls-files -- scripts tests skills agents cursor-rules|git ls-files -- no-such-dir|' tests/test_comment_anchor_style.sh
bash tests/test_comment_anchor_style.sh >/dev/null 2>&1; echo "M2 (expect 1): $?"
mv tests/test_comment_anchor_style.sh.bak tests/test_comment_anchor_style.sh

# M3 — neuter the pattern: the positive control must REDDEN.
sed -i.bak "s|^ANCHOR=.*|ANCHOR='ZZZ_NEVER_MATCHES_ZZZ'|" tests/test_comment_anchor_style.sh
bash tests/test_comment_anchor_style.sh >/dev/null 2>&1; echo "M3 (expect 1): $?"
mv tests/test_comment_anchor_style.sh.bak tests/test_comment_anchor_style.sh

# M4 — remove the self-exclusion: the guard's own pattern literals must then flag it.
sed -i.bak 's|\[ "$(basename "$f")" = "$SELF" \] && continue.*|:|' tests/test_comment_anchor_style.sh
bash tests/test_comment_anchor_style.sh >/dev/null 2>&1; echo "M4 (expect 1): $?"
mv tests/test_comment_anchor_style.sh.bak tests/test_comment_anchor_style.sh

# Confirm clean state restored
bash tests/test_comment_anchor_style.sh >/dev/null 2>&1; echo "restored (expect 0): $?"
```

Expected: `M1 (expect 1): 1`, `M2 (expect 1): 1`, `M3 (expect 1): 1`, `M4 (expect 1): 1`, `restored (expect 0): 0`.

**M4 is the trap check** — it confirms the guard genuinely needs its structural self-exclusion (its own header carries pattern literals such as the `render-board.sh:76` example inside the positive control). If M4 returns 0, the self-exclusion is dead code and the header no longer contains what you think it does.

- [ ] **Step 5: Add the authoring rule to `AGENTS.md`**

Append to the `## Guards and tests` section — or add a new `## Comments and cross-references` section after it. **Write no literal counter-example**: `AGENTS.md` is inside the walked surface, so an illustrative bad anchor would redden the guard.

```markdown
## Comments and cross-references

- A cross-reference in maintained source anchors on a **symbol name** or a **verbatim-quoted
  clause** — never on a line number. A quoted clause is greppable, so drift is mechanically
  visible; a line number is checkable by nothing, and rots fastest in exactly the files that move
  most. `tests/test_comment_anchor_style.sh` enforces the filename-plus-line-number form; the bare
  colon-number and prose "line N" forms are unenforceable without false positives and rest on this
  rule (ADR-0054).
- This binds maintained source only. Point-in-time records — results files, archived changes,
  specs, and Accepted ADRs — keep whatever pointer was true when written; rewriting them falsifies
  history.
```

- [ ] **Step 6: Confirm `AGENTS.md` did not redden the guard**

```bash
bash tests/test_comment_anchor_style.sh; echo "exit=$?"
```

Expected: `exit=0`. If the new prose flagged itself, rewrite it without the literal form — do not add an allowlist entry.

- [ ] **Step 7: Run the full suite**

```bash
fail=0; for t in tests/test_*.sh; do bash "$t" >/tmp/$(basename "$t").out 2>&1 || { echo "FAIL: $t"; fail=1; }; done; echo "suite fail=$fail"
```

Expected: `suite fail=0`.

- [ ] **Step 8: Commit**

```bash
git add tests/test_comment_anchor_style.sh AGENTS.md
git commit -m "test(0114): guard the explicit-file anchor form; document the rule

Adds tests/test_comment_anchor_style.sh, enforcing that no maintained-source
cross-reference anchors on <file>.<ext>:<N>. Partial by design: the bare and
prose forms measured ~38% and 60% false positives and rest on the AGENTS.md
rule plus review instead.

Verified against the pre-conversion base commit: the guard reports all 26 real
anchors. Mutation-tested four ways — reintroduced anchor, emptied population,
neutered pattern, and removed self-exclusion each redden it."
```

---

### Task 4: Mint ADR-0054 and repair ADR-0044's stale anchor

**Files:**
- Create: `docs/adrs/0054-*.md` — **via the `docket-adr` subagent**, which assigns the number, updates the index, and commits on `origin/docket`. Do not hand-author the file.
- Modify: `docs/adrs/0044-autonomy-precedence-call-site-pre-specification.md` — append-only `## Update`.

**Interfaces:**
- Consumes: the guard and rule from Task 3.
- Produces: ADR-0054, whose number is appended to change 0114's `adrs:` field by the implementer skill.

- [ ] **Step 1: Repair ADR-0044's stale anchor the one way the convention permits**

ADR-0044 cites `skills/docket-finalize-change/SKILL.md:124` as "the human-present close-out"; that line is now the `gate == off` rule. An `Accepted` ADR is immutable except its `status:` line, so the body is left **byte-untouched** and a dated note is appended.

Append to the end of `docs/adrs/0044-autonomy-precedence-call-site-pre-specification.md`:

```markdown
## Update — 2026-07-22 (change 0114)

The corollary's cross-reference into `skills/docket-finalize-change/SKILL.md` was anchored on a
line number, which has since drifted to an unrelated rule. Restating it with a stable anchor, and
leaving the Decision and Corollaries above byte-untouched: the human-present exception is the
close-out step whose SKILL text conditions merge on an attended run — anchor on that conditional
sentence, not on a line number.

This ADR's body keeps its original line-number citation, which is precisely why `docs/adrs/` sits
outside the walk of `tests/test_comment_anchor_style.sh`: an Accepted ADR cannot be edited to
satisfy a guard, so guarding it would demand a repair the convention forbids (ADR-0054).
```

- [ ] **Step 2: Verify ADR-0044's body above the new section is unchanged**

```bash
cd "$(git rev-parse --show-toplevel)"
git diff -U0 -- docs/adrs/0044-autonomy-precedence-call-site-pre-specification.md | grep -E '^-' | grep -vE '^---' && echo "BODY WAS MODIFIED — STOP" || echo "append-only: OK"
```

Expected: `append-only: OK`. Any removed line violates ADR immutability — revert and re-do as a pure append.

- [ ] **Step 3: Confirm the ADR edit did not redden the guard**

```bash
bash tests/test_comment_anchor_style.sh; echo "exit=$?"
```

Expected: `exit=0` — `docs/adrs/` is outside the walk, so this passes; the run confirms the exclusion is real rather than assumed.

- [ ] **Step 4: Commit the ADR-0044 update**

```bash
git add docs/adrs/0044-autonomy-precedence-call-site-pre-specification.md
git commit -m "docs(0114): ADR-0044 — restate the corollary's cross-reference with a stable anchor

Append-only Update note; Decision and Corollaries untouched. The body's original
line-number citation stays, which is why docs/adrs/ is outside the new guard's walk."
```

- [ ] **Step 5: Run the full suite one final time**

```bash
fail=0; for t in tests/test_*.sh; do bash "$t" >/tmp/$(basename "$t").out 2>&1 || { echo "FAIL: $t"; fail=1; }; done; echo "suite fail=$fail"
```

Expected: `suite fail=0`.

**Note for the implementing skill:** ADR-0054 itself is minted by dispatching the `docket-adr` subagent at review time (step 6 of docket-implement-next), recording: cross-references in maintained source anchor on symbols or quoted clauses; the guard enforces the explicit-file form only; the bare and prose forms are deliberately unguarded with their measured false-positive rates as the reason. It should `relates_to` ADR-0031 (the bound of source-syntax scanning) and ADR-0050 (compute, don't re-enumerate), and `change: 114`. **The ADR must not itself contain an explicit-file anchor** — `docs/adrs/` is unwalked, so nothing would catch it.

---

## Self-Review

**1. Spec coverage.**
- *Adopt the anchor idiom* → Task 3 Step 5 (`AGENTS.md`) + Task 4 (ADR).
- *Convert the in-scope refs* → Tasks 1–2. The spec said 26 refs across 9 files as of 2026-07-20; the reconcile re-measured **26 explicit-file lines across 8 files**, plus 13 bare and 2 prose lines, and every one has an exact replacement above.
- *Add `tests/test_comment_anchor_style.sh`, Pattern-1 only, no allowlist, mutation-tested both ways, population asserted* → Task 3 Steps 1–4.
- *Document the rule in `AGENTS.md`* → Task 3 Step 5, written without a literal counter-example (spec build-trap 2).
- *Mint a new ADR; repair ADR-0044 by append* → Task 4.
- *Build traps 1–3* → self-exclusion by `BASH_SOURCE` (Task 3 Step 1, mutation-checked as M4), `AGENTS.md` self-match (Step 6), ADR hygiene (Task 4 note).
- *Excluded surfaces* → encoded in the walk's `grep -vE '^docs/'` and documented in the guard header.

**2. Placeholder scan.** No TBDs. Every conversion gives exact old and new text; every command has an expected result; the guard is given in full.

**3. Type consistency.** `ANCHOR`, `FILES`, `SELF`, `violations`, `scanned`, `n_files` are used consistently between the guard body and the mutation commands that target them (M2 targets the `git ls-files` line, M3 the `ANCHOR=` assignment, M4 the self-exclusion line — all three exist verbatim in Step 1's source).

**One deliberate deviation from strict TDD, recorded so a reviewer does not read it as an oversight:** the guard lands *after* the conversions rather than red-first, because a red-first commit would leave the suite broken across two commits. The red evidence is not skipped — Task 3 Step 3 runs the finished guard against the pre-conversion base commit in a throwaway worktree and requires it to report exactly the 26 real anchors. That is stronger evidence than a red-first run, since it also pins the count.
