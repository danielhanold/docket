# Convention Progressive Disclosure — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extract the `### GitHub board mirror` section out of `docket-convention/SKILL.md` into an on-demand sibling reference, leaving a stub + pointer, so the convention's common-path context footprint shrinks.

**Architecture:** Move the heavy, opt-in, script-delegated mirror mechanics into a flat sibling `skills/docket-convention/github-board-mirror.md`. The core `SKILL.md` keeps a 2-line stub under the same `### GitHub board mirror` heading (so existing cross-references resolve) plus a pointer to the sibling. The one skill that needs the mechanics — `docket-status` (the Board-pass owner) — gets a one-phrase pointer added. A structural bash test guards the new shape.

**Tech Stack:** Markdown skill files; bash structural test (`tests/test_convention_extraction.sh`), run with `bash`.

## Global Constraints

- This is a **text move**, not a contract change — the mirror's behavior is unchanged. The full mirror content must land in the sibling **verbatim** (no loss, no silent paraphrase — ledger #5).
- The `### GitHub board mirror` **heading must survive** in `SKILL.md` — `### Configuration` references it by name twice (`see *GitHub board mirror*`), so deleting the heading breaks those anchors.
- The feature branch carries **only** the plan + code (skill/test files); it never edits docket metadata (the change file, `BOARD.md`, ADRs).
- New test assertions must be **non-vacuous** (deleting the guarded clause flips the test to NOT OK — ledger #2/#13) and must grep **files, never piped producers** (ledger #11/#16).
- No edit to `link-skills.sh` — it symlinks skill **directories**, so the new sibling is auto-included (verified at reconcile — ledger #12).

---

### Task 1: Extract the GitHub board mirror into a progressive-disclosure sibling

**Files:**
- Create: `skills/docket-convention/github-board-mirror.md`
- Modify: `skills/docket-convention/SKILL.md` (replace the `### GitHub board mirror` section, lines ~206–218, with a stub + pointer; keep the heading)
- Modify: `skills/docket-status/SKILL.md` (the `**\`github\` surface**` bullet, ~line 47 — add a sibling pointer)
- Test: `tests/test_convention_extraction.sh` (append a progressive-disclosure guard block)

**Interfaces:**
- Produces: a sibling reference at `skills/docket-convention/github-board-mirror.md` containing the full mirror mechanics; a `SKILL.md` stub that points to it; a `docket-status` pointer to it.
- Consumes: nothing from earlier tasks.

- [ ] **Step 1: Write the failing test** — append this block to `tests/test_convention_extraction.sh`, immediately before the final `if [ "$fail" = 0 ]` summary line:

```bash
# (f) progressive disclosure — the GitHub board mirror moved to a sibling (change 0020)
MIRROR_REF="$REPO/skills/docket-convention/github-board-mirror.md"
STATUS="$REPO/skills/docket-status/SKILL.md"
# the sibling exists and carries the moved mechanics (two distinct mirror-only phrases)
assert "mirror sibling exists" '[ -f "$MIRROR_REF" ]'
assert "mirror sibling carries the status-mapping mechanics" \
  '[ -f "$MIRROR_REF" ] && grep -qF "closed as **not planned**" "$MIRROR_REF"'
assert "mirror sibling carries the label-namespace rule" \
  '[ -f "$MIRROR_REF" ] && grep -qF "never touches a label it did not mint" "$MIRROR_REF"'
# moved, NOT copied — those mechanics are gone from the core SKILL.md
assert "core no longer carries the status-mapping mechanics" \
  '! grep -qF "closed as **not planned**" "$REF"'
assert "core no longer carries the label-namespace rule" \
  '! grep -qF "never touches a label it did not mint" "$REF"'
# the core keeps the stub heading (anchor for Configuration's cross-refs) AND a pointer
assert "core keeps the GitHub board mirror stub heading" 'grep -qF "### GitHub board mirror" "$REF"'
assert "core points at the mirror sibling" 'grep -qF "github-board-mirror.md" "$REF"'
# docket-status (the Board-pass owner) points at the sibling
assert "docket-status points at the mirror sibling" \
  '[ -f "$STATUS" ] && grep -qF "github-board-mirror.md" "$STATUS"'
```

- [ ] **Step 2: Run the test to verify the new assertions fail (red)**

Run: `bash tests/test_convention_extraction.sh`
Expected: overall `FAIL`; the new lines `NOT OK - mirror sibling exists`, `NOT OK - core no longer carries…`, `NOT OK - core points at the mirror sibling`, `NOT OK - docket-status points at the mirror sibling` appear. (The `core keeps the stub heading` line is already `ok` — it is a deletion-guard, intentionally green before and after.)

- [ ] **Step 3: Create the sibling `skills/docket-convention/github-board-mirror.md`** with an orientation note followed by the mirror section **verbatim** (intro paragraph + all five `**bold**` subsections, copied exactly from the current `SKILL.md` section):

```markdown
# GitHub board mirror — mechanics

> On-demand detail for the convention's GitHub board mirror. Read this when `board_surfaces`
> includes `github`. The core contract (one-way · change-files-authoritative · script-owned)
> lives in `SKILL.md`'s *GitHub board mirror* stub; this file is the full mechanics.

The `github` board surface mirrors each change to one GitHub issue (and one Projects v2 item) — **strictly one-way**: change files are the source of truth, the mirror is derived output that is **never read back** (no comments, labels, assignments, or state flow into change files). It rides in the Board pass (`docket-status`) and is **best-effort**, identical to the inline board rule: it needs network + `gh` auth, it self-heals on the next pass, and it **never aborts a build**. The mirror's external-write mechanics are owned by the deterministic `scripts/github-mirror.sh` (not agent-constructed `gh` calls); the Board pass only invokes it.

**`issue:` field.** One issue per change, upserted idempotently on the per-change `issue:` field (shape of `pr:`), minted on first sync and persisted into the change file on `metadata_branch`.

**Status → issue mapping (all seven).** Active states (`proposed`, `in-progress`, `blocked`, `deferred`, `implemented`) keep the issue **open**; terminal states close it with the native reason — `done` → closed as **completed**, `killed` → closed as **not planned**. The sync is the **sole writer** of issue open/closed state and reason: a PR may *reference* its mirror issue (a plain `#N` link, for the linked-PR "awaiting merge" view) but never `Closes #N`, which would make GitHub a second writer that cannot express `killed → not planned`.

**Labels — `docket:` namespace only.** Mirror labels are prefixed `docket:` (`docket:status/<state>`, `docket:priority/<p>`, and the derived `docket:readiness/<needs-brainstorm|auto-groom-blocked|build-ready>` / `docket:waiting/<needs-your-merge|not-yet-built>`). docket creates/updates only labels inside that namespace and never touches a label it did not mint, so existing repo labels are collision-proof.

**Issue body.** A visibility pointer, never a second home for the content: a one-way banner, a one-line frontmatter digest, the `## Why` distilled to a sentence or two, and hrefs to every relevant artifact (the change file on `metadata_branch`, the `spec:`, each ADR in `adrs:`, and `plan:`/`results:` once those resolve on the integration branch).

**Projects v2.** The optional half of `github`. When `github_project` is unset, first sync mints a **private** Projects v2 board under the integration repo's owner (Status single-select seeded from the active statuses) and writes its `{owner, number}` back into `.docket.yml` on the default branch — a one-time config commit that keeps later runs idempotent. Missing `project` token scope or any GraphQL failure ⇒ skip Projects and still mirror Issues + labels.
```

- [ ] **Step 4: Replace the mirror section in `skills/docket-convention/SKILL.md` with the stub** — replace the entire section (heading + intro + all five subsections, the current lines ~206–218) with exactly:

```markdown
### GitHub board mirror (shared definition)

The `github` board surface mirrors each change to one GitHub issue (and one Projects v2 item) — **strictly one-way**: change files are the source of truth, the mirror is derived output that is **never read back**. It rides in the Board pass (`docket-status`) and is **best-effort** (network + `gh` auth; self-heals next pass; never aborts a build); its external-write mechanics are owned by the deterministic `scripts/github-mirror.sh` (not agent-constructed `gh` calls) — the Board pass only invokes it. **Full mechanics — the `issue:` upsert, the `docket:` label namespace, the status→issue mapping across all seven states, the issue body, and Projects v2 — are in [`github-board-mirror.md`](github-board-mirror.md); read it when `board_surfaces` includes `github`.**
```

(Preserves the anchors the rest of the convention relies on: one-way · change-files-authoritative · never-read-back · rides-in-Board-pass · script-owned · the heading · the sibling pointer. The blank line before `### Bootstrap guard` stays.)

- [ ] **Step 5: Add the sibling pointer to `skills/docket-status/SKILL.md`** — in the `**\`github\` surface**` bullet (~line 47), extend the parenthetical so it reads:

```markdown
**`github` surface** — the one-way Issues + Projects v2 mirror (per the convention's *GitHub board mirror* definition; mechanics in `skills/docket-convention/github-board-mirror.md`). Invoke the deterministic `scripts/github-mirror.sh` against the change files, **best-effort**:
```

(One parenthetical insertion only; the rest of the bullet is unchanged.)

- [ ] **Step 6: Run the test to verify green**

Run: `bash tests/test_convention_extraction.sh`
Expected: `PASS` — every existing assertion plus all eight new `(f)` assertions print `ok`.

- [ ] **Step 7: Verify faithfulness (moved, not lost; no stale counts)**

Run:
```bash
# Every original subsection marker is present in the sibling
for p in "**\`issue:\` field.**" "**Status → issue mapping (all seven).**" "**Labels — \`docket:\` namespace only.**" "**Issue body.**" "**Projects v2.**"; do
  grep -qF "$p" skills/docket-convention/github-board-mirror.md && echo "ok: $p" || echo "MISSING: $p"
done
# No section-count or "GitHub board mirror" enumeration drifted elsewhere in the repo
grep -rn "GitHub board mirror" skills/ README.md 2>/dev/null
```
Expected: all five subsection markers `ok`; the only `GitHub board mirror` mentions are the `SKILL.md` stub heading + Configuration's two cross-refs, the sibling, and docket-status's pointer — no stale count to fix.

- [ ] **Step 8: Run the full suite to confirm no regression**

Run: `for t in tests/*.sh; do echo "== $t =="; bash "$t" | tail -1; done`
Expected: every test prints `PASS`.

- [ ] **Step 9: Commit**

```bash
git add skills/docket-convention/SKILL.md skills/docket-convention/github-board-mirror.md skills/docket-status/SKILL.md tests/test_convention_extraction.sh docs/superpowers/plans/2026-06-17-convention-progressive-disclosure.md
git commit -m "feat(0020): extract GitHub board mirror behind progressive disclosure

Move the mirror mechanics from docket-convention/SKILL.md into a flat
sibling github-board-mirror.md (read on demand when board_surfaces
includes github). Core keeps a stub + pointer under the same heading;
docket-status points at the sibling. Guarded by test_convention_extraction.sh."
```

---

## Notes for the reviewer (Step 6 of docket-implement-next)

- **Faithfulness (ledger #5):** confirm by reading — the sibling carries the *complete* mirror semantics (issue upsert, all-seven status mapping, `docket:` label namespace + collision-proofing, issue-body boundary, Projects v2 mint + write-back). The stub is a deliberate summary, not a second copy.
- **Anchor survival:** `### Configuration`'s two `see *GitHub board mirror*` references and the manifest `issue:` note still resolve to the stub heading.
- **Progressive disclosure works:** invoking `docket-convention` presents only `SKILL.md`; a skill needing mirror mechanics must Read the sibling — the stub and docket-status both say so.
