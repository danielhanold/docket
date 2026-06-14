# GitHub board mirror — Implementation Plan

> **For agentic workers:** implement task-by-task with TDD; `- [ ]` steps track progress. The mirror's external writes are exercised through the deterministic script in `--dry-run` against a mocked `gh`; no live GitHub.

**Goal:** Implement change 0011 — a selectable `board_surfaces` model and a one-way GitHub Issues + Projects mirror rendered by `docket-status`'s Board pass, implemented as a deterministic script (`scripts/github-mirror.sh`).

**Architecture:**
- **New code:** `scripts/github-mirror.sh` — the deterministic sync engine. Parses change-file frontmatter, resolves dependency readiness, computes per-change desired GitHub state (issue open/closed + reason, `docket:`-namespaced labels, body with artifact hrefs, Projects v2 item Status), and constructs `gh`/`gh api graphql` commands. Mock seam `GH="${GH:-gh}"`; `--dry-run` prints the argv it would exec (deterministic, testable); best-effort (no network/auth/`project` scope ⇒ degrade, never fail the caller).
- **New test:** `tests/test_github_mirror.sh` — asserts command construction via `--dry-run` + a mock `gh`, covering create-vs-update, the seven status→state/reason mappings, the `docket:` label set, body hrefs, and Projects-skipped-when-no-scope.
- **Skill prose (markdown):** `docket-convention` (config keys, `issue:` field, status→issue mapping, `docket:` label namespace, one-way rule, generalized board-refresh), `docket-status` (Board pass → render-each-enabled-surface, invoke the script), `docket-implement-next` (best-effort PR→issue reference at step 7).
- **Config:** `.docket.yml` documents `board_surfaces` (default `[inline]`) and `github_project`.

**Tech Stack:** Bash (script + tests, run with `bash`); markdown skills. Tests are sentinel-grep + behavioral dry-run assertions, run directly.

**Spec:** `.docket/docs/superpowers/specs/2026-06-14-github-issues-board-mirror-design.md` (on `docket`; read-only input — never edited from this worktree).

**Hard constraints (from existing tests):**
- `docket-status` and `docket-implement-next` are *operating* skills — my edits MUST NOT introduce any anti-copy sentinel from `tests/test_convention_extraction.sh` (e.g. `PM-altitude proposal`, `satisfied when it reaches`, `must never trail the change files`, …). Sentinels belong only in the `docket-convention` reference.
- `tests/test_board_refresh_on_transition.sh` pins the convention heading **`Board refresh on status writes`** and a `>=3` count of `run the Board pass (best-effort` clauses in `docket-implement-next` — PRESERVE both: keep the heading verbatim when generalizing, don't reduce the clause count.
- Every operating skill keeps `## Convention (load first — blocking)` and the string `docket-convention`.

---

### Task 1: The mirror script (TDD — test first)

**Files:** add `tests/test_github_mirror.sh`, add `scripts/github-mirror.sh`

- [ ] **Step 1 (red):** Write `tests/test_github_mirror.sh` asserting, against a temp change-file fixture and a mock `gh` via `GH=`/`--dry-run`:
  - new change (`issue:` empty) → emits `gh issue create` with title + body; prints a machine-readable `issue-minted <id> <number>` line.
  - existing change (`issue:` set) → emits `gh issue edit <n>` (not create).
  - status mapping: `done` → `gh issue close … --reason completed`; `killed` → `… --reason not planned`; active states → issue stays open.
  - labels: emits `docket:status/<s>`, `docket:priority/<p>`, and the readiness/waiting label; only `docket:`-prefixed labels.
  - body contains hrefs to the change file, and to `spec`/adrs when set.
  - no `Closes #` anywhere in script output (sync owns close).
  - Projects: with no project configured/auto-create disabled-by-mock, the run still emits the issue commands and skips Projects cleanly (exit 0).
  Run it; expect failure (script absent).
- [ ] **Step 2 (green):** Write `scripts/github-mirror.sh`:
  - `set -uo pipefail`; usage `github-mirror.sh [--dry-run] --changes-dir DIR [--project OWNER/NUM]`.
  - Two-pass: index id→status, then per change resolve `depends_on` readiness (mirror `docket-status`'s dependency pass: dep `done` ⇒ satisfied; `implemented` ⇒ `needs your merge`; else `not yet built`) and needs-brainstorm/build-ready.
  - frontmatter parse via a small `field()` grep helper.
  - `run_gh()` wrapper: in `--dry-run` print `+ $GH "$@"`; else exec `$GH "$@"`, capture failures best-effort (log to stderr, continue).
  - issue upsert, label reconcile (only `docket:` namespace), body builder (banner + digest + Why distillation + artifact hrefs), close with reason.
  - Projects v2 section: gated on `--project`/config; `gh api graphql` command construction; on any failure print a skip notice and continue (degrade to issues-only).
  - emit `issue-minted <id> <number>` on create so the caller persists `issue:`.
- [ ] **Step 3:** Run `tests/test_github_mirror.sh` → green. Run the full suite → still green.
- [ ] **Step 4:** Commit (`feat(0011): deterministic github-mirror script + test`).

### Task 2: Convention contract (`docket-convention`)

**File:** modify `skills/docket-convention/SKILL.md`

- [ ] Add `board_surfaces:` and `github_project:` to the `.docket.yml` block + a paragraph (list semantics, default `[inline]`, `[]` = no board, unknown-token warn-and-ignore, non-GitHub remote drops `github`).
- [ ] Add `issue:` to the Change manifest frontmatter block (shape of `pr:`, minted on first github sync).
- [ ] Generalize the **`Board refresh on status writes`** paragraph (keep the heading verbatim) to "refreshes each *enabled* surface."
- [ ] Add a `### GitHub board mirror` section: status→issue-state + close-reason mapping (all 7), `docket:` label namespace, the strictly one-way rule, the sync-owns-close / reference-not-`Closes` rule.
- [ ] Commit.

### Task 3: `docket-status` Board pass

**File:** modify `skills/docket-status/SKILL.md`

- [ ] Reframe `## Board` as render-each-enabled-surface: `inline` = today's `BOARD.md` regen (unchanged), `github` = invoke `scripts/github-mirror.sh` best-effort. Gate `inline` on membership; `[]` ⇒ no board.
- [ ] Generalize the board/source drift health check to "per enabled surface"; add an `issue:`-set-but-unreachable note. (No sentinel strings.)
- [ ] Commit.

### Task 4: `docket-implement-next` PR→issue reference

**File:** modify `skills/docket-implement-next/SKILL.md`

- [ ] In Step 7, after PR open: best-effort add a plain `#<issue>` reference to the PR body when `issue:` is set (NOT `Closes`); skip silently otherwise. Do not add a `run the Board pass (best-effort` clause (keep the count at 3). (No sentinel strings.)
- [ ] Commit.

### Task 5: Config documentation

**File:** modify `.docket.yml`

- [ ] Add `board_surfaces: [inline]` (with the explanatory comment) and a commented `github_project:` placeholder. Behavior identical to absent (default).
- [ ] Commit.

### Task 6: Full suite + close-out

- [ ] Run every `tests/*.sh` → all green. Record results if warranted (this build has manual-at-merge checks → write a results file).
