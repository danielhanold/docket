<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0372 — Retire deferred Go v1 workflow surfaces and seal the consumer cutover](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0372-retire-deferred-go-v1-workflow-surfaces-and-seal-the-consume.md)**
<!-- docket:backlink:end -->
# Retire Deferred Go v1 Workflow Surfaces and Seal the Consumer Cutover — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: this plan is executed by the `docket-build`
> role (per-task `docket-build-task` workers, one commit per task, TDD, single full-suite
> gate at the end). Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove every maintained executable activation path for the three deferred Go-v1
feature families — auto-capture/`mint-stub`, automated learnings
(harvest/index/capacity/promotion), and `terminal-publish`/`mark-publish-deferred` — then
install a repo-wide, shape-derived, mutation-tested seal that keeps them out, and formally
disposition the conflicting facade-era ADRs.

**Architecture:** Pure retirement + one guard. Canonical skill/reference/README sources are
edited to stop invoking or promising the deferred features (each replaced leg carries a
stable capability-specific "deferred from Go v1" diagnostic and its supported explicit
alternative); the byte-identical generated twins under `internal/assets/embedded/tree/` are
regenerated deterministically inside each editing task; existing tests that positively
assert the retired routing are inverted, deleted, or repointed per what each block guards;
a new seal test derives its prohibited set by syntactic shape (the facade op-structure
narrowed to the four retired op-tokens plus enabled-deferred-key→Bash wiring) over a
structurally-derived maintained corpus, with scratch-tree mutation evidence for all seven
required classes. No Bash file is deleted (change 0370 owns that), no config key removed,
no replacement feature built.

**Tech Stack:** Bash test suite (`tests/test_*.sh`, `set -uo pipefail`, house `assert`
idiom), Go (`go generate ./internal/assets` → `cmd/genassets`), the `docket` Go CLI, the
`docket-adr` agent for ADR transactions.

**Spec:** `docs/superpowers/specs/2026-08-30-retire-deferred-go-v1-workflow-surfaces-and-seal-the-consumer-cutover-design.md`
(synchronized metadata copy under `.docket/`). The change file's `## Reconcile log`
(2026-08-30 entry) narrows the spec and is equally binding.

## Global Constraints

- Changes 0369 and 0371 are **merged**. Do NOT redo their work: the standalone ADR-index
  render retirement and the `runner:` dispatch facade retirement are already done.
- Retired op-token set (exact, closed): `mint-stub`, `render-learnings-index`,
  `terminal-publish`, `mark-publish-deferred`.
- Still-supported facade ops that MUST stay legal until change 0370: `preflight`, `env`,
  `board-refresh`, `docket-status` — plus 0369's frozen carve-outs `archive-change`
  (killed-outcome leg), `render-change-links` (sweep leg), and the repair-only
  `render-adr-index` mention in `docket-adr`'s Index/validate section. The seal must not
  reject any of these.
- PRESERVE: the Go learnings read/validate path (`internal/repository/decode.go`,
  `internal/repository/validate.go`), the supported atomic ADR-index render
  (`internal/render/adrindex.go`), every existing learning record/index/marker byte, all
  config keys and schema, the whole frozen `scripts/` tree, `docs/` history, and Accepted
  ADR bodies.
- Never edit anything under `scripts/`, `docs/` (except this plan's own directory — which
  is written by the planner, not by build tasks), or `internal/repository/testdata/`.
- Canonical sources change before generated outputs; regeneration is
  `go generate ./internal/assets` run **twice** with a clean diff between runs, inside
  every task that touches a file under `skills/`, `agents/`, `cursor-rules/`, or
  `.docket.example.yml`.
- Full-suite command (docket-build's final gate, not per-task):
  `go run ./cmd/docket development test`. Go probes always use `-count=1`
  (cached-runner-serves-a-mutated-tree).
- Mutation-test procedure, every time: `cp "$f" "$f.bak"` → mutate → prove the mutation
  landed with `/usr/bin/grep -cF` before/after **through a whitespace-flattened copy**
  (`tr -s '[:space:]' ' '`) → run the assert → `mv -f "$f.bak" "$f"`. Never
  `git checkout --` as a restore (mutation-restore-needs-a-backup-copy). One bounded gap
  per ERE, never two (stacked-gap-regex-hangs-instead-of-failing). No backtick inside any
  double-quoted string in test source (`scripts/check-test-source-hygiene.sh` enforces;
  use the `BT='\`'` single-quoted idiom).
- Before deleting any prose, grep `tests/` for the phrases being removed and disposition
  every dependent assert (restatement-accumulates-its-own-guards); invert a block whose
  mechanism survives, delete a block whose premise is gone, repoint one whose content moved
  (test-premise-deleted-not-regated). Frozen-script tests (`test_mint_stub.sh`,
  `test_mark_publish_deferred.sh`, `test_terminal_publish.sh`,
  `test_render_learnings_index.sh` and every other test that only executes `scripts/*.sh`)
  are the frozen parity corpus — leave them alone unless a block asserts a **maintained**
  caller.
- Stable diagnostic vocabulary (used verbatim wherever a task says "deferred diagnostic"):
  - capture: `automatic change capture is deferred from Go v1 — capture work deliberately with \`docket change create\``
  - harvest: `automated learnings harvest is deferred from Go v1 — record or update findings by editing \`learnings/\` files directly`
  - index: `automated learnings-index rendering is deferred from Go v1 — existing \`learnings/README.md\` bytes are preserved, not refreshed`
  - capacity/promotion: `automated learnings capacity and promotion are deferred from Go v1 — ledger curation is human-directed`
  - publication: `terminal publication is deferred from Go v1 — \`docket finalize closeout\` is the complete automated closeout boundary`
  - marker: `publication-deferral marking is deferred from Go v1 — existing \`publish-deferred\` markers remain as historical evidence`
- **HALT RULE (spec):** if retiring any listed leg turns out to require building a new
  production subsystem, or the Task 7 ADR audit finds a still-current Accepted decision
  that *requires* a retired capability to remain active, the run halts for re-grooming —
  report the conflict, do not widen scope.

## File Map

- Modify: `skills/docket-convention/SKILL.md` (§ *Auto-capture (shared definition)* →
  deferral statement; learnings-ledger prose per Task 2)
- Delete: `skills/docket-convention/references/auto-capture.md`
- Modify: `skills/docket-convention/references/learnings.md`
- Modify: `skills/docket-convention/references/terminal-close-out.md`
- Modify: `skills/docket-status/SKILL.md` (harvest leg + dangling pointer, learnings pass,
  sweep-posture publish legs, health-check remedies)
- Modify: `skills/docket-implement-next/SKILL.md`,
  `skills/docket-implement-next/references/fix-loop.md`
- Modify: `skills/docket-adr/SKILL.md` (both `terminal-publish --adr` sites)
- Modify: `README.md`, `.docket.example.yml` (honest deferred boundary; keys stay)
- Regenerate: `internal/assets/embedded/manifest.json`,
  `internal/assets/embedded/tree/**` (twins of every edited/deleted canonical file)
- Modify (invert/retire blocks): `tests/test_skill_facade_wiring.sh`,
  `tests/test_learnings_ledger.sh`, `tests/test_closeout.sh`, `tests/test_board_checks.sh`,
  `tests/test_docket_status.sh`, `tests/test_typed_changes_docs.sh`,
  `tests/test_skill_size_budgets.sh` (row removal for the deleted reference), plus any
  further dependents each task's grep sweep surfaces
- Create: `tests/test_deferred_surface_seal.sh`; add its row to
  `tests/runtime-budgets.tsv` (+ re-seed `EXPECTED_TOTAL` in
  `tests/test_runtime_budgets.sh`)
- ADR transactions (via the `docket-adr` agent, metadata branch): disposition of ADRs
  0014, 0029, 0030, 0033 as the audit concludes; 0036, 0074, 0099 untouched

---

### Task 1: Retire auto-capture / `mint-stub` from canonical sources

**Files:**
- Modify: `skills/docket-convention/SKILL.md` (§ `### Auto-capture (shared definition)`,
  the config block comment at the `auto_capture:` key, and any other `mint`/`AUTO_CAPTURE`
  activation prose the Step-1 sweep finds)
- Delete: `skills/docket-convention/references/auto-capture.md`
- Modify: `skills/docket-implement-next/SKILL.md` (the reconcile-step capture sentence,
  the review-step "minted per *Auto-capture*" sentence, the fix-loop pointer sentence, the
  final-report "auto-captured" enumeration item)
- Modify: `skills/docket-implement-next/references/fix-loop.md` (§ `## Auto-capture is
  narrower here`, the `minted` disposition row, the three-mints-precedent sentence)
- Modify: `skills/docket-status/SKILL.md` (only the `AUTO_CAPTURE_ENABLED` auto-capture-leg
  sentence inside the `harvest <id> <path>` bullet — the harvest leg itself is Task 2's)
- Modify: `tests/test_skill_facade_wiring.sh` (the `--- change 0127:` block at the end),
  `tests/test_typed_changes_docs.sh` (whatever blocks assert active minting),
  `tests/test_skill_size_budgets.sh` (remove the deleted reference's budget row),
  `tests/test_learnings_ledger.sh` (only asserts naming AUTO_CAPTURE, if any), plus every
  other dependent the sweep finds
- Test: the inverted/added asserts live in `tests/test_skill_facade_wiring.sh`

**Interfaces:**
- Consumes: nothing (first task).
- Produces: the capture deferral diagnostic (Global Constraints wording) present verbatim
  in `skills/docket-convention/SKILL.md`; `skills/docket-convention/references/auto-capture.md`
  gone. Tasks 5–6 assert both.

- [ ] **Step 1: Derive the dependent-surface inventory (never hand-list).** In the feature
  worktree run, capturing output to a variable first (pipefail rule):

```bash
cd "$REPO"  # feature worktree root
SWEEP="$(grep -rn -E 'mint-stub|auto.capture|AUTO_CAPTURE|policy-suppressed|--minted|three-mints|mint site' \
  skills/ agents/ cursor-rules/ README.md .docket.example.yml tests/ | grep -v 'internal/assets')"
printf '%s\n' "$SWEEP"
```

  Sort every hit into: (a) canonical prose to retire (this task), (b) README/example.yml
  user docs (Task 4 — leave), (c) tests that positively assert the retired behavior
  (invert/delete here), (d) tests that only exercise frozen `scripts/*.sh`
  (`test_mint_stub.sh` — leave), (e) prose that survives as schema documentation
  ("`auto_capture` is a map, scalar is a hard error" — keep; it documents parseable legacy
  config). `docket-new-change`'s "mints new `proposed` ids" prose is manual id allocation,
  not auto-capture — leave it.

- [ ] **Step 2: Write the failing guards.** In `tests/test_skill_facade_wiring.sh`, replace
  the entire `# --- change 0127: capture prose reads the new exports ---` block (from that
  comment line through the last `0127:` assert) with:

```bash
# --- change 0372: auto-capture is retired from maintained instructions -------------------
# What this block GUARDS: no maintained skill instructs minting a stub or activating
# auto-capture. The schema keys stay documented as parseable-and-inactive (spec: preserved
# inactive configuration). Absence asserts are scoped per-file; the presence floor on the
# deferral diagnostic keeps them non-vacuous (assert-detects-removal-not-replacement).
for f in docket-implement-next docket-status docket-convention docket-new-change docket-groom-next docket-auto-groom; do
  ac372="$(cat "$REPO/skills/$f/SKILL.md")"
  assert "0372: $f carries no mint-stub instruction" \
    '! grep -Eq "docket\.sh[[:space:]]+mint-stub|mint-stub\.sh" <<<"$ac372"'
  assert "0372: $f never keys behavior on AUTO_CAPTURE_ENABLED being true" \
    '! grep -Fq "AUTO_CAPTURE_ENABLED\` is \`true" <<<"$ac372"'
done
assert "0372: the auto-capture reference file is gone" \
  '[ ! -e "$REPO/skills/docket-convention/references/auto-capture.md" ]'
assert "0372: no maintained pointer to references/auto-capture.md survives in skills/" \
  '! grep -rq "references/auto-capture.md" "$REPO/skills/"'
conv372="$(cat "$REPO/skills/docket-convention/SKILL.md")"
assert "0372: convention states the capture deferral diagnostic (floor)" \
  'grep -Fq "automatic change capture is deferred from Go v1" <<<"$conv372"'
assert "0372: convention names the supported alternative (floor)" \
  'grep -Fq "docket change create" <<<"$conv372"'
```

- [ ] **Step 3: Run to verify the new guards fail.**
  `bash tests/test_skill_facade_wiring.sh` — expect `NOT OK` on the mint-stub /
  AUTO_CAPTURE / reference-file / diagnostic-floor asserts, everything pre-existing still
  ok. (If a new assert is *already* green, stop and check it against
  assert-detects-removal-not-replacement — it may be matching nothing.)

- [ ] **Step 4: Retire the prose.**
  - `skills/docket-convention/SKILL.md`: rewrite `### Auto-capture (shared definition)`
    into a short `### Discovered work (auto-capture deferred)` section stating, verbatim
    including the diagnostic: the `auto_capture` map (`enabled`, `types`) remains a
    parseable configuration key and activates nothing; `automatic change capture is
    deferred from Go v1 — capture work deliberately with ` `` `docket change create` ``;
    work discovered mid-run is **reported in the run's final report, never silently minted
    or discarded**; an explicit request or an enabled key must be answered with this
    diagnostic **before any mutation**, never by invoking the frozen Bash scripts, and
    reinstalling will not make a missing verb appear. Update the config-block comment at
    `auto_capture:` (keep the "map since 0127, scalar is a hard error" schema fact; drop
    any promise that enabling activates capture — say "parseable; capture itself is
    deferred from Go v1"). Delete the mint-site paragraph and the blocking-read pointer to
    `references/auto-capture.md`.
  - `git rm skills/docket-convention/references/auto-capture.md`.
  - `skills/docket-implement-next/SKILL.md`: replace the reconcile-step sentence ("When
    `AUTO_CAPTURE_ENABLED` is `true` … does NOT consume a mint slot.") with: "Adjacent
    follow-up work this pass surfaces is **noted for the final report** — automatic
    change capture is deferred from Go v1, so nothing is minted; a human captures reported
    work deliberately with `docket change create`." Apply the same replacement to the
    review-step minting sentence and the fix-loop cross-reference sentence ("A finding
    that is genuinely distinct beyond-the-branch work still takes the auto-capture path
    above" → "…is reported as follow-up work in the final report"). In the final-report
    enumeration, replace "any stubs **auto-captured** (plus every dedup skip and any cap
    overflow)" with "any follow-up work **reported for deliberate capture**".
  - `skills/docket-implement-next/references/fix-loop.md`: rewrite
    `## Auto-capture is narrower here` into `## Beyond-the-branch findings are reported`
    (a finding about this branch's own diff is fixed or recorded; a genuinely distinct
    finding is reported as follow-up work — never minted; capture is deferred from Go v1).
    Change the `minted` disposition row to `reported` ("genuinely distinct,
    beyond-the-branch work reported for deliberate capture") and fix the
    three-mints-precedent sentence to no longer cite a mint precedent.
  - `skills/docket-status/SKILL.md`: in the `harvest <id> <path>` bullet, delete only the
    sentence beginning "When `AUTO_CAPTURE_ENABLED` is `true`, the same step's
    auto-capture leg …" (Task 2 rewrites the rest of that bullet).
  - Disposition every remaining Step-1 hit in category (c): in
    `tests/test_typed_changes_docs.sh` keep asserts that pin the `change_types` taxonomy
    and schema docs; delete/invert asserts that require active mint instructions. Remove
    the `skills/docket-convention/references/auto-capture.md` row and its narrative
    comments' *row* (comments may stay — they are history) from
    `tests/test_skill_size_budgets.sh` so the budget loop no longer reads a deleted file.

- [ ] **Step 5: Regenerate the embedded twins, twice.**

```bash
go generate ./internal/assets && git status --porcelain internal/assets | cat
go generate ./internal/assets && git diff --stat internal/assets | cat   # second run: must print nothing
bash tests/test_asset_bundle_drift.sh
```

  The deleted reference's twin
  `internal/assets/embedded/tree/skills/docket-convention/references/auto-capture.md` must
  be gone from the regenerated tree (`git status` shows its deletion); the second
  `go generate` must produce zero diff (determinism).

- [ ] **Step 6: Run the focused tests green.**
  `bash tests/test_skill_facade_wiring.sh && bash tests/test_typed_changes_docs.sh && bash tests/test_skill_size_budgets.sh && bash tests/test_learnings_ledger.sh && bash tests/test_asset_bundle_drift.sh`
  — all pass. Then mutation-probe the central new guard: `cp` a backup of
  `skills/docket-convention/SKILL.md`, re-add a line
  `` `docket.sh mint-stub --changes-dir x` ``, confirm landing via
  `/usr/bin/grep -cF 'docket.sh mint-stub'`, re-run `test_skill_facade_wiring.sh` →
  the convention assert reddens; `mv -f` the backup back.

- [ ] **Step 7: Commit** — canonical edits + deletion + regenerated twins + test edits in
  one commit: `git add -A skills/ internal/assets/embedded/ tests/` (then verify with
  `git status` that nothing outside those paths is staged);
  `git commit -m "refactor(0372): retire auto-capture/mint-stub from maintained instructions"`.

### Task 2: Retire automated learnings harvest / index / capacity / promotion

**Files:**
- Modify: `skills/docket-convention/references/learnings.md`
- Modify: `skills/docket-convention/SKILL.md` (*Learnings ledger* prose the sweep flags —
  keep format/read/promotion-**state** vocabulary, retire harvest/render/cap automation)
- Modify: `skills/docket-status/SKILL.md` (the `harvest <id> <path>` bullet — including
  its dangling pointer to a `docket-finalize-change` *Harvest learnings* step that no
  longer exists — and the full-pass learnings-index/advisories paragraph)
- Modify: `tests/test_learnings_ledger.sh` (invert/delete the harvest-leg and
  index-render sentinels; keep the read-path and ledger-format asserts), plus dependents
  the sweep finds
- Test: `tests/test_learnings_ledger.sh`

**Interfaces:**
- Consumes: Task 1's edited `skills/docket-status/SKILL.md` (the auto-capture sentence is
  already gone from the harvest bullet).
- Produces: harvest/index/capacity diagnostics (Global Constraints wording) present in
  `skills/docket-convention/references/learnings.md`; `skills/docket-status/SKILL.md`
  contains neither `Harvest learnings` nor `render-learnings-index`. Tasks 5–6 assert
  the absence repo-wide.

- [ ] **Step 1: Derive the dependent inventory.**

```bash
SWEEP="$(grep -rn -E 'harvest|render-learnings-index|learnings over-cap|promotion-pending|learnings\.cap|promotion_state' \
  skills/ agents/ cursor-rules/ README.md .docket.example.yml tests/ | grep -v 'internal/assets')"
printf '%s\n' "$SWEEP"
```

  PRESERVE unconditionally: every **read** site (`docket-implement-next` reading
  `learnings/README.md` and gating on `learnings.enabled`; review dispatch reading
  hooks), the ledger **format** definition (finding-file frontmatter, `promotion_state`
  vocabulary, the human-gated promotion rule — spec preserves "human-gated learning
  promotion" as a decision), and the Go read/validate path (`internal/repository/decode.go`,
  `validate.go` — do not touch). RETIRE: the harvest procedure, the index re-render
  instruction, the over-cap and promotion-pending advisories, any claim that a workflow
  refreshes the index.

- [ ] **Step 2: Write the failing guards.** In `tests/test_learnings_ledger.sh`, delete
  the asserts that positively require the harvest leg (`grep -qF "Harvest learnings"
  …docket-status…`, `grep -qF "learnings disabled" …docket-status…`,
  `grep -qF "render-learnings-index" …docket-status…`, `grep -qF "over-cap" … &&
  grep -qF "promotion-pending" …`) and add, adjacent to the surviving read-path asserts:

```bash
# --- change 0372: learnings automation is retired; the read path and ledger format stay --
st372="$(cat "$REPO/skills/docket-status/SKILL.md")"
assert "0372: docket-status carries no harvest leg" '! grep -Fq "Harvest learnings" <<<"$st372"'
assert "0372: docket-status never invokes the learnings-index renderer" \
  '! grep -Eq "render-learnings-index" <<<"$st372"'
assert "0372: docket-status computes no capacity/promotion advisories" \
  '! grep -Fq "over-cap" <<<"$st372" && ! grep -Fq "promotion-pending" <<<"$st372"'
ll372="$(cat "$REPO/skills/docket-convention/references/learnings.md")"
assert "0372: learnings ref states the harvest deferral diagnostic (floor)" \
  'grep -Fq "automated learnings harvest is deferred from Go v1" <<<"$ll372"'
assert "0372: learnings ref states the index deferral diagnostic (floor)" \
  'grep -Fq "automated learnings-index rendering is deferred from Go v1" <<<"$ll372"'
assert "0372: learnings ref states the capacity/promotion deferral diagnostic (floor)" \
  'grep -Fq "automated learnings capacity and promotion are deferred from Go v1" <<<"$ll372"'
# read path survives (non-vacuity companions through the same files the absence asserts read)
assert "0372: implement-next still reads the learnings index" \
  '[ "$(grep -cF "learnings/README.md" "$REPO/skills/docket-implement-next/SKILL.md")" -ge 2 ]'
assert "0372: implement-next still gates reads on learnings.enabled" \
  '[ "$(grep -cF "learnings.enabled" "$REPO/skills/docket-implement-next/SKILL.md")" -ge 1 ]'
```

  Keep every surviving assert about the convention's ledger-format section untouched.

- [ ] **Step 3: Run to verify failure.** `bash tests/test_learnings_ledger.sh` — the six
  new 0372 asserts red (three absence, three floors), read-path asserts green.

- [ ] **Step 4: Retire the prose.**
  - `skills/docket-status/SKILL.md`: rewrite the `harvest <id> <path>` bullet to: "For
    each, note the id in the pass report — automated learnings harvest is deferred from
    Go v1 — record or update findings by editing `learnings/` files directly. The absence
    of a harvest is never a sweep failure, and the pass never fabricates an empty harvest
    result." (This removes both the leg and the dangling *Harvest learnings* pointer.)
    Replace the full-pass learnings paragraph (the one invoking
    `render-learnings-index.sh` and emitting `over-cap` / `promotion-pending`) with:
    "**Learnings (deferred).** automated learnings-index rendering is deferred from Go v1
    — existing `learnings/README.md` bytes are preserved, not refreshed; automated
    learnings capacity and promotion are deferred from Go v1 — ledger curation is
    human-directed. The pass reads nothing and writes nothing under `learnings/`; an
    enabled `learnings.enabled` key gates *reads* elsewhere and activates no automation
    here."
  - `skills/docket-convention/references/learnings.md`: keep the ledger contract (file
    shape, topics, `promotion_state` vocabulary, human-gated promotion, the AGENTS.md
    graduation criterion) and the read path. Replace the harvest-procedure, cap-enforcement,
    and index-render prose with the three deferral diagnostics above plus: existing
    records, the index, `learnings.cap`, and promotion-state data remain parseable
    evidence; explicit record creation/updates are ordinary file edits committed on
    `metadata_branch`; no workflow refreshes `README.md` automatically, and the frozen
    Bash renderer is not a supported fallback.
  - `skills/docket-convention/SKILL.md`: apply the same retirement to any *Learnings
    ledger* sentence the sweep flagged as promising harvest/render/cap automation; keep
    format + promotion-gating prose.
  - Disposition remaining test hits from Step 1 (e.g. `tests/test_docket_metadata_branch.sh`
    or others greping harvest vocabulary from skill prose — repoint at the frozen script
    contract or delete per what the block guards).

- [ ] **Step 5: Regenerate twins, twice** — same commands as Task 1 Step 5; second run
  diff-clean; `bash tests/test_asset_bundle_drift.sh` green.

- [ ] **Step 6: Focused tests + mutation probe.**
  `bash tests/test_learnings_ledger.sh && bash tests/test_docket_metadata_branch.sh && bash tests/test_asset_bundle_drift.sh`
  green. Mutation: back up `skills/docket-status/SKILL.md`, re-add a line containing
  `docket.sh render-learnings-index`, prove landing (`/usr/bin/grep -cF`), re-run
  `test_learnings_ledger.sh` → renderer-absence assert reddens; restore with `mv -f`.

- [ ] **Step 7: Commit** —
  `git commit -m "refactor(0372): retire automated learnings harvest/index/capacity/promotion legs"`.

### Task 3: Retire `terminal-publish` / `mark-publish-deferred` from canonical sources

**Files:**
- Modify: `skills/docket-convention/references/terminal-close-out.md` (publish + mark
  steps on every driver path; the killed-leg `docket.sh archive-change` and the sweep-leg
  `docket.sh render-change-links` STAY — they are 0369 frozen carve-outs)
- Modify: `skills/docket-status/SKILL.md` (the *Sweep posture* paragraph's publish legs,
  manual `docket.sh terminal-publish` remedies, `mark-publish-deferred.sh` mark step; the
  `publish-deferred` / `adr-unpublished` health-check *descriptions* stay — they read
  existing markers — but their printed remedies must stop instructing a retired op)
- Modify: `skills/docket-adr/SKILL.md` (both `docket.sh terminal-publish --adr` call
  sites and the re-publish prose)
- Modify: `skills/docket-implement-next/SKILL.md` (the docket-adr dispatch sentence's
  "publishes it onto the integration branch on acceptance" claim)
- Modify: `tests/test_closeout.sh` (the maintained-caller wiring blocks), and dependents
  in `tests/test_board_checks.sh`, `tests/test_docket_status.sh`,
  `tests/test_board_refresh_on_transition.sh` etc. per the sweep
- Test: `tests/test_closeout.sh`

**Interfaces:**
- Consumes: Tasks 1–2 (docket-status SKILL already partially rewritten).
- Produces: publication + marker diagnostics (Global Constraints wording) present in
  `skills/docket-convention/references/terminal-close-out.md`; zero
  `docket.sh terminal-publish` / `mark-publish-deferred` occurrences anywhere under
  `skills/` outside frozen carve-outs. Tasks 5–6 assert repo-wide.

- [ ] **Step 1: Derive the dependent inventory.**

```bash
SWEEP="$(grep -rn -E 'terminal-publish|mark-publish-deferred|publish-deferred|terminal_publish|skipped-publish|adr-unpublished' \
  skills/ agents/ cursor-rules/ README.md .docket.example.yml tests/ | grep -v 'internal/assets')"
printf '%s\n' "$SWEEP"
```

  Classify: retired instructions (this task) vs. schema/key documentation (`terminal_publish:`
  in the convention config block and `.docket.example.yml` — keep, reworded per Task 4 if
  it promises activation) vs. marker-*reading* health-check prose (keep the check, fix the
  remedy) vs. frozen-script tests (leave). **ADR-publication halt check:** before editing
  `skills/docket-adr/SKILL.md`, confirm no still-current Accepted ADR *requires* active
  ADR publication to the integration branch (search `docs/adrs/` for Accepted decisions
  about ADR publication; `grep -rln "publish" docs/adrs/ | …` then read the Accepted
  hits). The `adr-unpublished` visibility check reading existing state is fine; if an
  Accepted decision mandates the *publish action itself* as current architecture, apply
  the HALT RULE.

- [ ] **Step 2: Write the failing guards.** In `tests/test_closeout.sh`, rework the
  maintained-caller wiring blocks (the `--- call-site wiring sentinels`, `--- change 0064`
  and finalize-wiring sections): delete every assert that *requires* a
  `docket.sh terminal-publish` / `mark-publish-deferred` site in `skills/` prose (their
  premise — active publication wiring — is deleted), keep the asserts pinning the Go
  closeout/cleanup verbs and the frozen `archive-change` kill legs, and add:

```bash
# --- change 0372: terminal publication + deferral marking retired from maintained prose --
tp372_hits="$(grep -rn -E 'docket\.sh[[:space:]]+(terminal-publish|mark-publish-deferred)([^[:alnum:]_-]|$)|(terminal-publish|mark-publish-deferred)\.sh[[:space:]]+--' "$REPO/skills/" || true)"
assert "0372: no maintained skill invokes terminal-publish or mark-publish-deferred (hits:[$tp372_hits])" \
  '[ -z "$tp372_hits" ]'
tco372="$(cat "$REPO/skills/docket-convention/references/terminal-close-out.md")"
assert "0372: close-out states the publication deferral diagnostic (floor)" \
  'grep -Fq "terminal publication is deferred from Go v1" <<<"$tco372"'
assert "0372: close-out states the marker deferral diagnostic (floor)" \
  'grep -Fq "publication-deferral marking is deferred from Go v1" <<<"$tco372"'
assert "0372: close-out still drives the done path through Go closeout (floor)" \
  'grep -Fq "docket finalize closeout --id" <<<"$tco372"'
assert "0372: the frozen killed-outcome archive leg survives (0369 carve-out untouched)" \
  'grep -Fq "docket.sh archive-change" <<<"$tco372"'
```

  Also update the `find_ungated_terminal_publish_call_sites` consumer: its `scripts/*.sh`
  half still guards the frozen tree (keep); narrow its skills/ half away or drop the
  helper's skills scan with a comment (the maintained side is now governed by the absence
  assert above).

- [ ] **Step 3: Run to verify failure.** `bash tests/test_closeout.sh` — the 0372 absence
  assert red (live sites exist in terminal-close-out.md, docket-status, docket-adr), both
  diagnostic floors red; Go-verb and archive-change floors already green.

- [ ] **Step 4: Retire the prose.**
  - `terminal-close-out.md`: delete the publish step (`docket.sh terminal-publish --id …`)
    and the mark step (`docket.sh mark-publish-deferred --mode add …`) from every driver
    path (done, proposed-kill, reconcile-kill), replace with one shared paragraph carrying
    both diagnostics verbatim plus: supported Go metadata closeout is the complete
    automated closeout boundary; a request that specifically requires published terminal
    artifacts stops **before** claiming that outcome even when the metadata transaction
    succeeded; existing markers and published records remain untouched historical
    evidence; the frozen Bash publisher is not a supported fallback and an enabled
    `terminal_publish:` key activates nothing. Keep step ordering, the killed-leg
    `docket.sh archive-change`, the sweep's `docket.sh render-change-links`, and
    `docket finalize closeout|cleanup` untouched.
  - `skills/docket-status/SKILL.md` *Sweep posture*: rewrite so the sweep's close-out has
    no publish leg at all — remove `terminal-publish` failure/remedy/`skipped-publish`
    publish-chaining prose and the `mark-publish-deferred.sh` mark instruction; a failed
    `render-change-links` follow-up is now only "re-render manually" (that leg is 0370's
    frozen carve-out and stays). Where the `publish-deferred` / `adr-unpublished` health
    checks' surfacing prose points at a remedy, replace the remedy command with the
    publication diagnostic (the marker is *read*; acting on it is deferred).
  - `skills/docket-adr/SKILL.md`: replace both `terminal-publish --adr` call sites and
    the re-publish bullet with: the ADR and its index live on `metadata_branch`;
    `terminal publication is deferred from Go v1 — ` integration-branch publication of ADR
    bytes is not performed, and a status flip to an already-published ADR leaves the
    previously published copy as history (the `adr-unpublished` health check keeps the
    drift visible).
  - `skills/docket-implement-next/SKILL.md`: in the docket-adr dispatch sentence, drop
    "publishes it onto the integration branch on acceptance if the repo has opted in".
  - Disposition the remaining Step-1 test hits: in `tests/test_board_checks.sh` /
    `tests/test_docket_status.sh`, blocks executing `scripts/board-checks*` or
    `scripts/docket-status.sh` against fixtures are frozen-parity — leave; blocks greping
    `skills/docket-status/SKILL.md` for publish-leg prose are inverted or deleted per
    what they guard (e.g. keep the log-and-continue posture asserts, which survive the
    rewrite — reword their anchors if the anchor sentence moved).

- [ ] **Step 5: Regenerate twins, twice** — as Task 1 Step 5; drift test green.

- [ ] **Step 6: Focused tests + mutation probe.**
  `bash tests/test_closeout.sh && bash tests/test_board_checks.sh && bash tests/test_docket_status.sh && bash tests/test_asset_bundle_drift.sh` green
  (`test_docket_status.sh` is the 60s row — run it once, serial). Mutation: back up
  `skills/docket-adr/SKILL.md`, re-add `docket.sh terminal-publish --adr 7`, prove
  landing, re-run `test_closeout.sh` → 0372 absence assert reddens naming the path;
  restore with `mv -f`.

- [ ] **Step 7: Commit** —
  `git commit -m "refactor(0372): retire terminal-publish and mark-publish-deferred from maintained prose"`.

### Task 4: Honest deferred boundary in user documentation (`README.md`, `.docket.example.yml`)

**Files:**
- Modify: `README.md` (§ *Capturing discovered work (`auto_capture`)…* ~lines 287–370;
  the migration terminal-publish permission paragraph ~line 1078; the two `auto_capture:`
  config-fence entries ~523/560; any other hit from the Task 1–3 sweeps)
- Modify: `.docket.example.yml` (the `auto_capture` block comment ~303–323;
  `terminal_publish` and learnings key comments if they promise activation)
- Test: `tests/test_docket_example_yml.sh`, `tests/test_typed_changes_docs.sh` (existing
  suites over these files), plus a hand re-run of `tests/test_skill_facade_wiring.sh`

**Interfaces:**
- Consumes: the three diagnostics' wording (Global Constraints).
- Produces: README/example.yml describe each key as parseable-and-inactive with the
  supported alternative; no active-documentation promise of a deferred capability. Task 5
  scans both files (they are in the seal corpus) — wording here must not place a deferred
  key and an invocation shape on one line.

- [ ] **Step 1: Sweep and classify.** Re-run the three family sweeps from Tasks 1–3 over
  `README.md .docket.example.yml` only, and list every line that (a) instructs running a
  retired op, (b) promises that an enabled key activates the feature, or (c) merely
  documents key shape/history (keep). The README single-branch paragraph (~683,
  "no terminal-publish copy") is descriptive of an absence — keep.

- [ ] **Step 2: Write the failing guard.** Append to `tests/test_docket_example_yml.sh`:

```bash
# --- change 0372: deferred keys stay parseable; docs stop promising activation ----------
readme372="$(cat "$ROOT/README.md")"
example372="$(cat "$ROOT/.docket.example.yml")"
assert "0372: README carries no retired-op invocation" \
  '! grep -Eq "docket\.sh[[:space:]]+(mint-stub|render-learnings-index|terminal-publish|mark-publish-deferred)([^[:alnum:]_-]|$)" <<<"$readme372"'
assert "0372: README documents auto_capture as deferred from Go v1" \
  'grep -Fq "deferred from Go v1" <<<"$readme372"'
assert "0372: example yml keeps the auto_capture key (schema preserved)" \
  'grep -Eq "^auto_capture:" <<<"$example372"'
assert "0372: example yml marks capture as deferred from Go v1" \
  'grep -Fq "deferred from Go v1" <<<"$example372"'
```

  (Use this file's actual root variable name — read its header; if it uses `$REPO`,
  substitute.)

- [ ] **Step 3: Run to verify failure.** `bash tests/test_docket_example_yml.sh` — the
  two "deferred from Go v1" floors red (the key-presence and no-invocation asserts may
  already be green; that is fine — they are companions, not the detection edge).

- [ ] **Step 4: Rewrite the docs.** README auto_capture section: keep the taxonomy and
  key-shape documentation and the 0127 hard-error history; replace activation promises
  ("`auto_capture.enabled: true` closes that gap…") with the boundary: the key remains
  parseable and stored values are never rewritten, but `automatic change capture is
  deferred from Go v1 — capture work deliberately with ` `` `docket change create` ``;
  an enabled value causes a capability-specific unsupported diagnostic before any leg
  mutates state, never a Bash fallback. Update the config-fence comments (`# a MAP since
  change 0127 …` lines) to append `; capture itself is deferred from Go v1`. Rework the
  migration permission paragraph (~1078): the granted push allow-rule is historical
  compatibility; `terminal_publish: true` no longer exercises it (`terminal publication
  is deferred from Go v1 — `docket finalize closeout` is the complete automated closeout
  boundary`). Apply the same one-line boundary comments in `.docket.example.yml` at
  `auto_capture:` (and at `terminal_publish:` / learnings automation keys if their
  comments promise activation) — comments only; never change a key or value
  (config-shape-change-strands-outer-layers).

- [ ] **Step 5: Run focused tests green.**
  `bash tests/test_docket_example_yml.sh && bash tests/test_typed_changes_docs.sh && bash tests/test_docket_config.sh` — README/example fences that other tests pin verbatim may
  redden; disposition each red per its block's purpose (config-fence value equality must
  keep passing — comments beside values are usually outside the pinned fence; verify).

- [ ] **Step 6: Commit** —
  `git commit -m "docs(0372): document the deferred capability boundary in README and example config"`.

### Task 5: The seal — `tests/test_deferred_surface_seal.sh` (live-tree scan)

**Files:**
- Create: `tests/test_deferred_surface_seal.sh`
- Modify: `tests/runtime-budgets.tsv` (new row), `tests/test_runtime_budgets.sh`
  (re-seed `EXPECTED_TOTAL`; state the old→new sum and why in the tsv header comment)
- Test: the new file itself

**Interfaces:**
- Consumes: the clean post-Task-1–4 tree; the diagnostic floor phrases.
- Produces: `seal_filter()` (stdin: repo-relative paths → stdout: maintained-corpus
  paths) and `seal_scan <rootdir> <filelist-file>` (stdout: zero or more
  `SEAL VIOLATION <canonical|generated> <family> <path>:<line>:<matched line>` records;
  exit 0 always — callers assert on output). Task 6 reuses both against scratch trees, so
  they MUST read the root from `$1`, never a hard-coded `$REPO`.

- [ ] **Step 1: Write the test (it should pass immediately on the live tree — its red
  proof arrives via Task 6's scratch mutations, plus the one live mutation probe in
  Step 3).** Create `tests/test_deferred_surface_seal.sh`:

```bash
#!/usr/bin/env bash
# tests/test_deferred_surface_seal.sh — change 0372: the final consumer-cutover seal.
#
# CLAIM: no MAINTAINED file re-activates a deferred Go-v1 feature family. Prohibited set =
# the facade op-structure NARROWED to the retired op-tokens (mint-stub,
# render-learnings-index, terminal-publish, mark-publish-deferred) + a direct retired
# script invocation + enabled-deferred-key -> Bash wiring. Still-supported facade ops
# (preflight, env, board-refresh, docket-status) and 0369's frozen carve-outs
# (archive-change, render-change-links, render-adr-index) are deliberately OUTSIDE the
# prohibited set until change 0370 (spec: "narrowed through the explicit retirement
# classification").
#
# CORPUS is structural, never a caller-file list (enumerated-floor): every git-tracked
# file MINUS four structural exclusions —
#   scripts/                       the frozen Bash tree awaiting 0370
#   docs/                          point-in-time history (changes/specs/plans/results/ADRs)
#   tests/                         the frozen parity/deletion corpus + this suite itself
#   internal/repository/testdata/  recorded fixture corpora (DATA, not source)
# Each exclusion is bounded (a named directory prefix, no wildcards) and held honest by
# Task-6 negative controls + the live-tree tolerance floors below
# (frozen-fixture-corpus-trips-repo-wide-scans).
#
# RESIDUAL LIMITS (named, per byte-pattern-guard-matches-a-spelling): (1) a bare noun
# mention of a retired script ("mark-publish-deferred.sh" with no arguments and no
# invocation prefix) is permitted — prose may describe history (ADR-0030 precedent);
# (2) a paraphrase that re-teaches a retired op with no op token or script name at all is
# invisible to any byte guard — whole-branch review owns it; (3) key->Bash wiring is
# detected only on ONE physical line (multi-line wiring is review-owned; one bounded gap
# per ERE, stacked gaps hang — stacked-gap-regex-hangs-instead-of-failing).
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

RETIRED='mint-stub|render-learnings-index|terminal-publish|mark-publish-deferred'
DEFERRED_KEYS='terminal_publish|auto_capture|AUTO_CAPTURE|learnings\.enabled|LEARNINGS_ENABLED'
INVOKE_SHAPE='docket\.sh|DOCKET_SCRIPTS_DIR|scripts/[a-z][a-z-]*\.sh'

# stdin: repo-relative paths; stdout: the maintained corpus (structural exclusions applied)
seal_filter(){
  grep -Ev '^(scripts/|docs/|tests/|internal/repository/testdata/)'
}

# $1=root dir, $2=file holding relative paths (one per line). Emits SEAL VIOLATION records.
seal_scan(){
  local root="$1" list="$2" p hits line fam kind
  while IFS= read -r p; do
    [ -f "$root/$p" ] || continue
    kind='canonical'
    case "$p" in internal/assets/embedded/tree/*) kind='generated' ;; esac
    # Shape A: retired facade op (both sides of the op token bounded)
    hits="$(grep -nE -e "docket\.sh[[:space:]]+($RETIRED)([^[:alnum:]_-]|\$)" "$root/$p")" || true
    # Shape B: direct retired-script invocation (an argument, or the invocation prefix)
    hits="$hits$(grep -nE -e "($RETIRED)\.sh[[:space:]]+--" "$root/$p")" || true
    hits="$hits$(grep -nE -e "DOCKET_SCRIPTS_DIR[^}]*\}\"?/($RETIRED)\.sh" "$root/$p")" || true
    while IFS= read -r line; do
      [ -z "$line" ] && continue
      fam="$(grep -oE -e "$RETIRED" <<<"$line" | sed -n 1p)"
      printf 'SEAL VIOLATION %s %s %s:%s\n' "$kind" "${fam:-facade}" "$p" "$line"
    done <<<"$hits"
    # Shape C: enabled-deferred-key -> Bash wiring (same-line co-occurrence)
    hits="$(grep -nE -e "$DEFERRED_KEYS" "$root/$p" | grep -E -e "$INVOKE_SHAPE")" || true
    while IFS= read -r line; do
      [ -z "$line" ] && continue
      printf 'SEAL VIOLATION %s deferred-key-wiring %s:%s\n' "$kind" "$p" "$line"
    done <<<"$hits"
  done <"$list"
}

# ---- live-tree run ----------------------------------------------------------------------
LIST="${TMPDIR:-/tmp}/seal-corpus.XXXXXX"; LIST="$(mktemp "$LIST")"
git -C "$REPO" ls-files | seal_filter >"$LIST"

# Population floors: the corpus is real and holds the surfaces this seal exists to watch.
corpus_n="$(wc -l <"$LIST" | tr -d ' ')"
assert "corpus floor: the maintained corpus is populated (found $corpus_n)" '[ "$corpus_n" -ge 150 ]'
for must in skills/docket-convention/SKILL.md \
            skills/docket-convention/references/terminal-close-out.md \
            internal/assets/embedded/tree/skills/docket-convention/SKILL.md \
            README.md AGENTS.md .docket.example.yml; do
  assert "corpus floor: $must is scanned" 'grep -qxF "$must" "$LIST"'
done
# Exclusion boundedness: the excluded trees are PRESENT and carry the very content the
# scan must tolerate — so the exclusions are load-bearing, not decorative.
assert "frozen tree present: scripts/mint-stub.sh still ships (0370's to delete)" \
  '[ -f "$REPO/scripts/mint-stub.sh" ] && [ -f "$REPO/scripts/terminal-publish.sh" ]'
assert "history preserved: an archived ADR still names docket.sh mint-stub" \
  'grep -qF "docket.sh mint-stub" "$REPO/docs/adrs/0045-auto-capture-is-best-effort.md"'

violations="$(seal_scan "$REPO" "$LIST")"
assert "SEAL: zero prohibited surfaces in the maintained corpus
$violations" '[ -z "$violations" ]'

# Diagnostic floors: the deferral boundary the absence relies on is actually stated
# (assert-detects-removal-not-replacement: absence + companion through the same surface).
assert "floor: capture deferral stated in the convention" \
  'grep -qF "automatic change capture is deferred from Go v1" "$REPO/skills/docket-convention/SKILL.md"'
assert "floor: harvest deferral stated in the learnings reference" \
  'grep -qF "automated learnings harvest is deferred from Go v1" "$REPO/skills/docket-convention/references/learnings.md"'
assert "floor: publication deferral stated in the close-out reference" \
  'grep -qF "terminal publication is deferred from Go v1" "$REPO/skills/docket-convention/references/terminal-close-out.md"'
# Supported-surface controls: the narrowing really permits what it claims to permit.
assert "control: still-supported board pass survives in maintained prose" \
  'grep -rqF -- "docket.sh docket-status --board-only" "$REPO/skills/"'
assert "control: supported Go closeout boundary survives" \
  'grep -qF "docket finalize closeout --id" "$REPO/skills/docket-convention/references/terminal-close-out.md"'
assert "control: supported Go ADR transactions survive" \
  'grep -qF "docket adr record" "$REPO/skills/docket-adr/SKILL.md"'
assert "control: Go learnings read/validate path untouched" \
  '[ -f "$REPO/internal/repository/decode.go" ] && [ -f "$REPO/internal/repository/validate.go" ] && [ -f "$REPO/internal/render/adrindex.go" ]'

rm -f "$LIST"
exit $fail
```

  Calibrate before finalizing: run
  `git ls-files | grep -Ev '^(scripts/|docs/|tests/|internal/repository/testdata/)' | wc -l`
  and set the corpus floor to roughly half the measured count (comment the measured
  number beside it). Then run Shape C against the live corpus by hand; if
  `.docket.example.yml` or README wording trips it on a legitimate schema comment, fix
  the *wording* (reflow so key and invocation shape never share a line) — add a
  file-scoped exclusion ONLY if a legitimate same-line pairing is irreducible, and then
  bound it to the exact path with its reason in the header plus a Task-6 negative control
  proving the exclusion cannot hide a Shape A hit.

- [ ] **Step 2: Run it.** `bash tests/test_deferred_surface_seal.sh` — every assert `ok`,
  exit 0. If the SEAL assert reports violations, those are real leftovers from Tasks 1–4:
  fix them at the *source* (canonical file, then regenerate twins twice), never by
  widening an exclusion.

- [ ] **Step 3: One live mutation probe (landing-proved, backup-restored).**

```bash
f="skills/docket-convention/SKILL.md"; cp "$f" "$f.bak"
printf '%s\n' 'Run `docket.sh terminal-publish --id 9` now.' >>"$f"
/usr/bin/grep -cF 'docket.sh terminal-publish' "$f"   # must be >= 1 (landed)
bash tests/test_deferred_surface_seal.sh              # SEAL assert must go NOT OK,
                                                      # naming canonical + terminal-publish + the path
mv -f "$f.bak" "$f"
bash tests/test_deferred_surface_seal.sh              # green again
```

- [ ] **Step 4: Budget row.** Measure serially:
  `/usr/bin/time -p bash tests/test_deferred_surface_seal.sh`. Apply the table's rule
  (next multiple of 5 above the worst standalone serial reading, plus 5s margin, min
  10s). Add `tests/test_deferred_surface_seal.sh<TAB><ceiling><TAB>parallel` to
  `tests/runtime-budgets.tsv`, and re-seed `EXPECTED_TOTAL` in
  `tests/test_runtime_budgets.sh` (old sum + new ceiling − any ceiling removed if a test
  file was deleted in Tasks 1–3; state old→new and why in the tsv header). Run
  `bash tests/test_runtime_budgets.sh` green.

- [ ] **Step 5: Commit** —
  `git commit -m "test(0372): repo-wide shape-derived seal over the retired deferred-surface families"`.

### Task 6: Mutation evidence — seven classes + negative controls

**Files:**
- Modify: `tests/test_deferred_surface_seal.sh` (append the scratch-tree harness before
  `exit $fail`)
- Test: the same file

**Interfaces:**
- Consumes: `seal_filter` / `seal_scan` exactly as Task 5 defined them (root-relative,
  `$1`-rooted).
- Produces: the spec's seven mutation classes, each red-for-the-right-reason, plus green
  negative controls — all inside scratch trees so a failed assert can never leave the
  repo mutated (fixtures restore by construction: `rm -rf` under trap).

- [ ] **Step 1: Append the harness** (before the final `exit $fail`):

```bash
# ---- mutation evidence (spec §Mutation evidence): scratch trees, never the live repo ----
MUT="$(mktemp -d "${TMPDIR:-/tmp}/seal-mut.XXXXXX")"
trap 'rm -rf "$MUT"' EXIT
plant(){ # $1=relpath $2=content -> builds the file inside $MUT
  mkdir -p "$MUT/$(dirname "$1")" && printf '%s\n' "$2" >"$MUT/$1"
}
scan_scratch(){ # scans everything planted so far, THROUGH the same filter the live run uses
  local l="$MUT/.list"
  (cd "$MUT" && find . -type f ! -name .list | sed 's#^\./##') | seal_filter >"$l"
  seal_scan "$MUT" "$l"
}
expect_hit(){ # $1=name $2=required substrings (space-separated), asserts each appears
  local out; out="$(scan_scratch)"; local ok=1 s
  for s in $2; do grep -qF -- "$s" <<<"$out" || ok=0; done
  [ -n "$out" ] || ok=0
  assert "mutation $1 detected for the intended reason ($out)" '[ "$ok" = 1 ]'
  rm -f "$MUT/$LAST_PLANT"
}
P(){ LAST_PLANT="$1"; plant "$1" "$2"; }

# M1 — direct facade invocation of a retired op in a maintained workflow
P 'skills/x/SKILL.md' 'Run `"${DOCKET_SCRIPTS_DIR:?x}"/docket.sh terminal-publish --id 7 --enabled true`.'
expect_hit M1 'canonical terminal-publish skills/x/SKILL.md'
# M2 — an auto-capture / mint-stub instruction
P 'skills/x/SKILL.md' 'Mint it: `docket.sh mint-stub --changes-dir d --type fix`.'
expect_hit M2 'canonical mint-stub skills/x/SKILL.md'
# M3 — an executable learnings-index renderer call (direct script shape)
P 'skills/x/references/r.md' '"${DOCKET_SCRIPTS_DIR:?x}"/render-learnings-index.sh --learnings-dir d'
expect_hit M3 'canonical render-learnings-index skills/x/references/r.md'
# M4 — an automated harvest/capacity/promotion leg (re-teaching the renderer op)
P 'skills/x/SKILL.md' 'After harvesting, re-render via `docket.sh render-learnings-index --learnings-dir d`.'
expect_hit M4 'canonical render-learnings-index skills/x/SKILL.md'
# M5 — a terminal-publication marker call
P 'skills/x/references/t.md' 'mark-publish-deferred.sh --mode add --reason blocked'
expect_hit M5 'canonical mark-publish-deferred skills/x/references/t.md'
# M6 — a prohibited caller restored through generated/embedded output
P 'internal/assets/embedded/tree/skills/x/SKILL.md' 'Run `docket.sh terminal-publish --id 7`.'
expect_hit M6 'generated terminal-publish internal/assets/embedded/tree/skills/x/SKILL.md'
# M7 — configuration wiring from an enabled deferred key to Bash
P 'skills/x/SKILL.md' 'When `terminal_publish: true`, run `"${DOCKET_SCRIPTS_DIR:?x}"/docket.sh docket-status --board-only` afterwards.'
expect_hit M7 'canonical deferred-key-wiring skills/x/SKILL.md'

# ---- negative controls: every permitted category stays green ---------------------------
neg(){ # $1=name $2=relpath $3=content — plant, scan, expect silence, unplant
  plant "$2" "$3"; local out; out="$(scan_scratch)"
  assert "negative control $1 stays green ($out)" '[ -z "$out" ]'
  rm -f "$MUT/$2"
}
neg history          'docs/adrs/0999-x.md'      'History: `docket.sh terminal-publish --id 7` was the old path.'
neg frozen-scripts   'scripts/x.md'             'Contract: `docket.sh mint-stub --changes-dir d`.'
neg frozen-tests     'tests/test_x.sh'          'grep -qF "docket.sh terminal-publish" "$f"'
neg fixture-corpus   'internal/repository/testdata/c.md' 'docket.sh render-learnings-index --learnings-dir d'
neg supported-op     'skills/x/SKILL.md'        'Board pass: `docket.sh docket-status --board-only`.'
neg frozen-carveout  'skills/x/refs/t.md'       'Killed leg: `docket.sh archive-change --outcome killed`.'
neg deferred-doc     'skills/x/SKILL.md'        'terminal publication is deferred from Go v1 — `docket finalize closeout` is the boundary.'
neg schema-key       'skills/x/SKILL.md'        'terminal_publish:            # parseable; publication itself is deferred from Go v1'
neg noun-mention     'skills/x/SKILL.md'        'The frozen mark-publish-deferred.sh remains on disk until 0370.'
neg go-adr-path      'skills/x/SKILL.md'        'Record it with `docket adr record` (atomic index render included).'
```

- [ ] **Step 2: Run it.** `bash tests/test_deferred_surface_seal.sh` — all mutation
  asserts `ok` (each names its intended family/kind), all negative controls `ok`, live
  SEAL section unchanged, exit 0. Verify one class the honest way: temporarily break
  `seal_scan` (comment out the Shape C block), re-run — M7 must go `NOT OK` while M1–M6
  stay ok (proving the classes exercise distinct shapes); restore the block.

- [ ] **Step 3: Re-measure the budget.** Re-run
  `/usr/bin/time -p bash tests/test_deferred_surface_seal.sh`; if the reading now crosses
  the Task 5 ceiling's rule boundary, raise the row per the table's own rounding rule and
  re-seed `EXPECTED_TOTAL` again (say old→new in the header comment).

- [ ] **Step 4: Commit** —
  `git commit -m "test(0372): seven-class mutation evidence and negative controls for the seal"`.

### Task 7: Individual ADR audit and dispositions (0014, 0029, 0030, 0033)

**Files:**
- Read: `docs/adrs/0014-consuming-repo-script-resolution.md`,
  `docs/adrs/0029-docket-facade-routing-and-config-presentation.md`,
  `docs/adrs/0030-facade-wiring-guard-discriminates-on-invocation-prefix.md`,
  `docs/adrs/0033-cursor-auto-run-trust-at-facade.md` (metadata worktree `.docket/docs/adrs/`),
  plus `0036`, `0074`, `0099` (verify-preserve only)
- Write: nothing by hand — every disposition goes through a **`docket-adr` agent
  dispatch** (one per decision, never bulk), which owns numbering, status flips, the
  atomic index render, and the metadata-branch commit

**Interfaces:**
- Consumes: the accepted Go-v1 decisions (ADR-0099 one-metadata-topology, ADR-0074 gate
  verdict, the 0371 native-dispatch ADR(s), this change's retirement classification).
- Produces: each of 0014/0029/0030/0033 either formally dispositioned
  (Superseded/Reversed via a transaction that records the reasoning and names change
  0372) or explicitly recorded in the results notes as still-current-and-compatible with
  the audit argument; 0036/0074/0099 byte-untouched.

- [ ] **Step 1: Audit each ADR individually.** For each of the four, read the full body
  and answer in writing (goes into the PR body / results notes): (a) what does it decide,
  normatively, today? (b) does that decision conflict with the accepted Go-v1 state after
  this change (native dispatch merged, facade ops narrowed to four still-supported
  survivors, retired families sealed)? (c) if it conflicts, is the right transaction
  supersede (a successor decision replaces it) or reverse (the decision no longer holds
  and nothing replaces it)? Starting hypotheses to *check, not assume*:
  0014 (DOCKET_SCRIPTS_DIR script resolution) and 0033 (cursor auto-run trust at the
  facade) likely conflict with 0371's native dispatch; 0029 (facade routing/config
  presentation) likely conflicts with the narrowed facade; 0030 (invocation-prefix guard
  discrimination) is the very guard family Task 3 inverted — but its *noun-mention
  permission* is still load-bearing in this plan's own seal residuals, so a supersede
  that carries that clause forward may fit better than a reverse. **No bulk
  disposition:** an ADR that survives the audit is left Accepted, with the argument
  recorded.
- [ ] **Step 2: HALT check.** If any audited ADR — or any other Accepted ADR the audit
  reading surfaces — *requires* an active retired capability (e.g. mandates terminal
  publication as current architecture), stop the run and report per the Global
  Constraints HALT RULE. Do not disposition your way around a still-current requirement.
- [ ] **Step 3: Execute the dispositions.** One `docket-adr` dispatch per conflicting
  ADR ("supersede ADR-00NN because …" / "reverse ADR-00NN because …", citing change 0372
  and the Go-v1 decisions). The agent returns each new/updated ADR number; record any
  NEW numbers in the change's `adrs:` relation via `docket change reconcile` (the
  transaction re-renders the Artifacts block; never hand-edit the change file).
- [ ] **Step 4: Verify.** In the metadata worktree: the four audited ADRs show their new
  status lines (or recorded still-current verdicts), `0036`/`0074`/`0099` show **no
  diff**, and the ADR index was re-rendered by the transactions (never by a standalone
  render — that path is retired). `git -C .docket log --oneline -5` shows the agent's
  commits.
- [ ] **Step 5: Commit (feature branch).** This task normally leaves the feature branch
  untouched (ADRs live on the metadata branch). If Step 1's written audit produced no
  feature-tree edit, record the audit summary for the results file in the task report and
  make no empty commit.

### Task 8: Full-suite gate and margin reporting (docket-build's closing gate)

**Files:**
- None (verification only; fixes route back to the owning task's files)

- [ ] **Step 1:** Run the authoritative suite from the feature worktree:
  `go run ./cmd/docket development test`. Drive it via the gate driver per the
  docket-build contract (inline blocking `docket gate drive advance` slices; never
  background-and-yield).
- [ ] **Step 2:** Treat any `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` line as a screening
  finding — serially confirm the named file before acting; a
  `SERIAL CONFIRMED OVER BUDGET:` line is authoritative (re-budget or shard per
  `tests/runtime-budgets.tsv`'s own rules). Report the seal test's remaining margin **as
  a number** in the results notes (budget-headroom-is-spent-before-it-is-breached).
- [ ] **Step 3:** Any red is dispositioned per systematic debugging: a test asserting a
  retired premise → invert/delete per the Global Constraints rule (name it in the
  commit); a genuine defect → fix at the owning task's surface. Re-run to green.

---

## Self-Review (performed at plan time)

- **Spec coverage:** Retirement mapping families → Tasks 1–3; configuration
  compatibility + failure behavior → diagnostics wired into Tasks 1–4 prose and floors;
  maintained-consumer inventory → each task's Step-1 derivation sweep (shape-derived,
  never hand-listed); generation-twice → Tasks 1–3 Step 5; final seal + shape derivation
  + diagnostics → Task 5; seven mutation classes + negative controls → Task 6; ADR
  handling + halt → Task 7 (+ Task 3 Step 1's publication-ADR check); testing/rollout →
  Task 8; acceptance criteria 1–19 each map to a task step (17/18 are held by the Global
  Constraints "never edit scripts/, docs/" rule and the seal's exclusion floors).
- **Known judgment calls made explicit for the builder:** `skills/docket-adr/SKILL.md`'s
  two `terminal-publish --adr` sites are inside family 3 even though the reconcile log's
  canonical list did not name them — the inventory is shape-derived and the seal would
  otherwise contradict the tree (Task 3); `references/auto-capture.md` is deleted rather
  than rewritten because the convention SKILL carries the boundary statement (Task 1);
  tests/ is excluded from the seal corpus wholesale because a test cannot re-activate a
  caller without the maintained prose it greps tripping the scan first (Task 5 header
  documents this as structural, Task 6 `frozen-tests` control proves it).
- **Type consistency:** `seal_filter`/`seal_scan`/`plant`/`expect_hit`/`neg` signatures
  match between Tasks 5 and 6; diagnostic phrases are defined once in Global Constraints
  and consumed verbatim by Tasks 1–6.
