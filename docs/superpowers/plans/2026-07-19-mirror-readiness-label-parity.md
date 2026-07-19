# GitHub mirror readiness parity — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend `readiness_label` in `scripts/github-mirror.sh` so an `implemented` change carrying the `## Finalize blocked` marker maps to a `docket:readiness/finalize-blocked` label, restoring parity with the inline board and digest.

**Architecture:** The GitHub mirror is a deterministic, one-way, best-effort derived view over the change files (ADR-0012). `readiness_label(f, status)` today early-returns for any non-`proposed` status, so the `finalize-blocked` readiness state (change 0087) never becomes a `docket:readiness/` label. This change makes the mirror *consume* the readiness ownership already declared by `render-board.sh`'s `digest_readiness` (readiness is meaningful for `proposed` via `readiness()`, and for `implemented` via `finalize_blocked()`; every other status has none) — it adds no second policy. Both `readiness()` and `finalize_blocked()` are already sourced from `lib/docket-frontmatter.sh`; the new label self-provisions through the existing idempotent `gh label create --force` path.

**Tech Stack:** Bash (`set -uo pipefail`, no `set -e`), the docket test-harness idiom (`--dry-run` + mock `gh`, `assert` helper), `lib/docket-frontmatter.sh` frontmatter/section helpers.

## Global Constraints

- **The mirror invents no readiness policy** — it mirrors `render-board.sh`'s per-status ownership one-for-one: `proposed` (its four `readiness()` tokens) and `implemented` (`finalize_blocked()`); every other status has none. Any future readiness value is added to the owner (`render-board.sh`) first, and the mirror follows the same status→owner shape.
- **Label form only** — readiness is a `docket:readiness/` label like every other derived label; no bespoke surface, no issue-body callout, no Projects field.
- **Additive `--add-label` is unchanged** — the mirror does not prune a stale readiness label when a change leaves the state; that is a pre-existing, mirror-wide concern (affects status/priority/proposed-readiness labels identically) and is explicitly out of scope.
- **No new external calls, no manifest, no label registration** — the new label rides the existing `label create ... --force` self-provision path.
- **Tests use the existing `--dry-run` + mock-`gh` seam only** — no live `gh`. Dry-run traces argv to **stderr** as `+ gh …` (the test merges with `2>&1`); `issue edit`/`issue create` traces span multiple physical lines because `--body` is multi-line, so scope assertions to the single-line `label create <lbl> --force` trace and the contiguous trailing `--add-label <lbl>` token — never to `issue edit <n> .*<lbl>`.

---

### Task 1: Map `implemented` + `## Finalize blocked` → `docket:readiness/finalize-blocked`

**Files:**
- Modify: `scripts/github-mirror.sh:121-134` (the `readiness_label` function)
- Test: `tests/test_github_mirror.sh` (add one fixture + readiness-parity assertions in section A)

**Interfaces:**
- Consumes: `readiness "$f"` and `finalize_blocked "$f"` from `lib/docket-frontmatter.sh` (already sourced at `github-mirror.sh:80`). `finalize_blocked FILE` exits 0 iff the body contains a whole line exactly `## Finalize blocked` (`has_section` = `grep -qxF`); meaningful only for `implemented`. `field "$f" <key>` reads a frontmatter scalar.
- Produces: `readiness_label FILE STATUS` echoes at most one `docket:*` label (or nothing) on stdout; unchanged signature. `labels_for` (`github-mirror.sh:172`) already calls it and emits any non-empty line — no caller change.

- [ ] **Step 1: Add the positive fixture — an `implemented` change carrying the marker**

In `tests/test_github_mirror.sh`, immediately after the `0012-waiter.md` fixture heredoc (ends at line 92, before the `# --- mock gh:` comment at line 94), add:

```bash
cat > "$tmp/active/0014-finalize-blocked.md" <<'EOF'
---
id: 14
slug: finalize-blocked
title: Implemented change flagged finalize-blocked
status: implemented
priority: medium
depends_on: []
adrs: []
issue: 202
---

## Finalize blocked

### 2026-07-19 — gate failure
Rebased suite went red; a human must intervene.
EOF
```

(The existing `0013-target.md` — `implemented`, issue `200`, **no** marker — is the negative regression case; it stays as-is.)

- [ ] **Step 2: Add the readiness-parity assertions**

In `tests/test_github_mirror.sh`, in section A (after the existing label assertions, i.e. after the `every mirror label is docket:-namespaced` assert that ends at line 135), add:

```bash
# readiness parity (change 0097): the mirror carries readiness for the two statuses the
# board/digest owner (render-board.sh) declares — proposed (four tokens) and implemented
# (finalize-blocked) — and for no other status.
assert "implemented change WITH ## Finalize blocked self-provisions the finalize-blocked label" \
  'echo "$out" | grep -qF "label create docket:readiness/finalize-blocked"'
assert "implemented change WITH ## Finalize blocked attaches the finalize-blocked label" \
  'echo "$out" | grep -qF -- "--add-label docket:readiness/finalize-blocked"'
assert "exactly one change carries the finalize-blocked readiness label (only the marked implemented one)" \
  '[ "$(echo "$out" | grep -cF "label create docket:readiness/finalize-blocked")" -eq 1 ]'
assert "only proposed(needs-brainstorm) + implemented(finalize-blocked) carry ANY docket:readiness/ label — implemented-without-marker and every non-owning status carry none" \
  '[ "$(echo "$out" | grep -cF "label create docket:readiness/")" -eq 2 ]'
```

Rationale for the last two (records the owner rule as a regression guard):
- The fixture set has exactly two changes that should carry a `docket:readiness/` label: `0009-existing` (`proposed`, no spec → `needs-brainstorm`) and the new `0014` (`implemented` + marker → `finalize-blocked`). `0012-waiter` carries `docket:waiting/…`, not `docket:readiness/…`.
- Count `== 2` over `label create docket:readiness/` therefore proves that `0013-target` (`implemented`, **no** marker), `0011` (`in-progress`), `0006` (`done`), and `0005` (`killed`) each emit **no** readiness label — the spec's "any other status ⇒ no readiness label" and "implemented without marker ⇒ no label" in one invariant. `blocked`/`deferred` share the same no-matching-arm path, so no dedicated fixture is needed.

- [ ] **Step 3: Run the test to verify the new assertions FAIL**

Run: `bash tests/test_github_mirror.sh`
Expected: the four new asserts print `NOT OK - …` (readiness label absent because `readiness_label` still early-returns for `implemented`; the count over `label create docket:readiness/` is `1`, not `2`), overall exit non-zero. Every pre-existing assert still prints `ok - …`.

- [ ] **Step 4: Extend `readiness_label` to consume the board's readiness ownership**

In `scripts/github-mirror.sh`, replace the whole `readiness_label` function (lines 121-134):

```bash
# --- readiness label (maps the shared readiness token to the docket: label) ---
readiness_label(){
  local f="$1" status="$2" id tok
  [ "$status" = "proposed" ] || return 0
  id="$(field "$f" id)"; tok="$(readiness "$f")"
  case "$tok" in
    waiting)
      local r="${DEP_REASON[$id]:-not yet built}"
      printf 'docket:waiting/%s' "${r// /-}" ;;     # "needs your merge" -> needs-your-merge
    auto-groom-blocked) printf 'docket:readiness/auto-groom-blocked' ;;
    needs-brainstorm)   printf 'docket:readiness/needs-brainstorm' ;;
    build-ready)        printf 'docket:readiness/build-ready' ;;
  esac
}
```

with:

```bash
# --- readiness label (maps the shared readiness token to the docket: label) ---
# Readiness mirrors the board/digest OWNER one-for-one (render-board.sh's digest_readiness,
# change 0087): `proposed` carries the four readiness() tokens; `implemented` carries
# finalize_blocked(); every other status has none. Any future readiness value is added to the
# owner (render-board.sh) FIRST, and this follows the same status->owner shape — so the three
# projections (board, digest, mirror) cannot drift at the next value. (change 0097)
readiness_label(){
  local f="$1" status="$2" id tok
  case "$status" in
    proposed)
      id="$(field "$f" id)"; tok="$(readiness "$f")"
      case "$tok" in
        waiting)
          local r="${DEP_REASON[$id]:-not yet built}"
          printf 'docket:waiting/%s' "${r// /-}" ;; # "needs your merge" -> needs-your-merge
        auto-groom-blocked) printf 'docket:readiness/auto-groom-blocked' ;;
        needs-brainstorm)   printf 'docket:readiness/needs-brainstorm' ;;
        build-ready)        printf 'docket:readiness/build-ready' ;;
      esac ;;
    implemented)
      if finalize_blocked "$f"; then printf 'docket:readiness/finalize-blocked'; fi ;;
  esac
}
```

Notes:
- The `implemented` arm uses `if finalize_blocked "$f"; then … fi` (not `finalize_blocked "$f" && printf …`) so the case arm always returns 0 — matching `render-board.sh`'s `digest_readiness` exactly and keeping `ready="$(readiness_label …)"` clean regardless of the marker's presence.
- The `proposed` arm is byte-for-byte the old body, now nested under `case "$status"` — the four existing `proposed` tokens are unchanged (regression guarded by the pre-existing asserts at lines 130-133).

- [ ] **Step 5: Run the test to verify ALL assertions PASS**

Run: `bash tests/test_github_mirror.sh`
Expected: every line prints `ok - …` (the four new readiness asserts and all pre-existing asserts), overall exit `0`.

- [ ] **Step 6: Run the frontmatter-lib and board tests as a no-regression check on the shared readiness helpers**

Run: `bash tests/test_docket_frontmatter.sh && bash tests/test_render_board.sh`
Expected: both exit `0`, all `ok - …` — confirms this change did not disturb `readiness()`/`finalize_blocked()` or the board's own readiness rendering (this change touches neither).

- [ ] **Step 7: Commit**

```bash
git add scripts/github-mirror.sh tests/test_github_mirror.sh docs/superpowers/plans/2026-07-19-mirror-readiness-label-parity.md
git commit -m "feat(0097): mirror finalize-blocked readiness label for implemented changes"
```
