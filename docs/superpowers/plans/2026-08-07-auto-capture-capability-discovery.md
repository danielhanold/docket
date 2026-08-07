<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0226 — Reframe auto-capture as capability discovery with strict admission gates](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-07-0226-reframe-auto-capture-as-capability-discovery-with-strict-adm.md)**
<!-- docket:backlink:end -->

# Auto-capture as capability discovery — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reframe `skills/docket-convention/references/auto-capture.md` (and the convention's inline summary) from a suppression-first rule into a capability-discovery pipeline with six explicit admission gates, without weakening a single existing suppression rule.

**Architecture:** Two documentation files change; two test files change with them. `references/auto-capture.md` is rewritten in place — the reframed intent, discovery categories, admission gates, and routing table are ADDED, and the existing *Materiality bar* and *The deterministic mint* sections are carried forward with their guarded clauses byte-intact. `docket-convention/SKILL.md`'s `### Auto-capture (shared definition)` gets an intent sentence and a widened drill-down pointer under progressive disclosure — no duplication of the reference's detail. `tests/test_docket_review.sh` owns the prose sentinels for this reference; its change-0218 guard block is re-anchored (never deleted) and extended with the two coverage cases. `tests/test_skill_size_budgets.sh` carries the budget raises, each with the dated justification block that file's convention requires.

**Tech Stack:** Markdown skill files; POSIX-ish bash test scripts using an `assert(){ eval }` harness; `grep -E`, `awk` section extractors, `wc`.

## Global Constraints

Copied from the spec and the repo's `AGENTS.md`; every task's requirements implicitly include these.

- **Do not modify** `scripts/mint-stub.sh` or `scripts/mint-stub.md`. Stub-minting mechanics, deterministic naming, numbering, and change-creation behavior are unchanged.
- **Do not change** `AUTO_CAPTURE_TYPES` filtering, its ordering before the cap, or the per-invocation cap of 3.
- **Every existing suppression rule stays in effect.** Nothing about review-finding suppression is relaxed.
- **Change 0218's guard clauses must survive**, re-anchored to new wording where prose moves — never deleted or weakened. The clauses are: `current run will fix in-branch … fails the bar`, `harvest … exempt`, `no open branch … no fix loop`, the *Materiality bar* section extractor's non-vacuity anchor, and the reference's line floor.
- **`mint-stub.sh` hard-rejects any `--body-file` that does not start with `## Why`.** The five capture fields are therefore labelled lines *under* one leading `## Why`, never five top-level `##` sections.
- **Site C (`docket-finalize-change` / `docket-status` harvest) keeps its own admission bar** — the *would a human file this as its own change / PR* test, not the six capability-discovery gates.
- **Budget raises are in-diff edits** to `tests/test_skill_size_budgets.sh`, each with a dated justification block that NAMES the `references/` file the prose was considered for and STATES why it cannot live there. Rounding rule: lines up to the next multiple of 5, words up to the next multiple of 50 — and if that lands within 25 words of the actual, take the multiple AFTER it.
- **Shell rules (`AGENTS.md`):** never `producer | grep -q` — capture into a variable and use `grep <<<"$var"`. A pattern leading with `--` must use `grep -qF --` or `grep -E -e`. awk indent classes are `[^[:space:]]`, never `[^ ]`.
- **Comment rules (`AGENTS.md`):** a cross-reference in a test comment anchors on a symbol name or a verbatim-quoted clause — never a filename-plus-line-number.
- **Guard rule (`AGENTS.md`, learnings `assert-detects-removal-not-replacement`):** a guard is code — mutation-test it. Write the assert that DETECTS the state being removed, and confirm the mutation actually landed (`grep -c` before and after) before believing a green run.
- **Phrase-grep rule (learnings `phrase-grep-over-wrapped-prose`):** phrase asserts over hard-wrapped markdown read a whitespace-collapsed haystack. `tests/test_docket_review.sh` already defines `flatten(){ tr -s '[:space:]' ' '; }`; reuse it. Leave `awk` section extractors on newline-bearing input, and leave line-count floors alone.

**Full-suite command** (the build gate; the repo has no runner script — every `tests/test_*.sh` is standalone and exits non-zero on failure):

```bash
cd /Users/homer/dev/docket/.worktrees/reframe-auto-capture-as-capability-discovery-with-strict-adm
red=""; for t in tests/test_*.sh; do bash "$t" >/tmp/suite-$(basename "$t").log 2>&1 || red="$red $t"; done
echo "RED:[$red]"
```

The suite is large (79 files) and runs near the foreground tool ceiling — run it backgrounded to a stable log and block on the exit code, not on prose.

---

## File Structure

| File | Responsibility after this change |
|---|---|
| `skills/docket-convention/references/auto-capture.md` | The full definition: discovery intent, categories, six admission gates, never-mint list, materiality bar (with 0218's scoping), site-dependent routing, the five capture fields, per-discovery classification, and the deterministic mint. |
| `skills/docket-convention/SKILL.md` (`### Auto-capture (shared definition)`) | Intent sentence + config/classification/policy mechanics + mint-site enumeration + the blocking drill-down pointer. No categories, gates, fields, or routing table. |
| `tests/test_docket_review.sh` | Prose sentinels over `references/auto-capture.md`: 0218's surviving clauses (re-anchored, flattened) plus the qualifying-discovery and current-branch-finding coverage cases. |
| `tests/test_skill_size_budgets.sh` | The two budget rows and their dated justification blocks. |

---

## Task 1: Reframe the reference, re-anchor its guards, raise its budget

**Files:**
- Modify: `skills/docket-convention/references/auto-capture.md` (full in-place rewrite)
- Modify: `tests/test_docket_review.sh` (the `--- change 0218: auto-capture no longer absorbs this branch's own findings ---` block)
- Modify: `tests/test_skill_size_budgets.sh` (the `skills/docket-convention/references/auto-capture.md` BUDGETS row + a new justification block)

**Interfaces:**
- Consumes: `flatten(){ tr -s '[:space:]' ' '; }`, already defined in `tests/test_docket_review.sh` above the 0218 block; `assert(){ if eval "$2"; ...; }` from the same file; `$AC` / `$ac_body` / `$ac_bar`, defined by the 0218 block itself.
- Produces: the rewritten reference's section headings, which Task 2's convention summary must NOT duplicate: `## What to look for`, `## Admission gates`, `## Materiality bar`, `## Routing`, `## What a captured discovery says`, `## Per discovery`, `## The deterministic mint`.

- [ ] **Step 1: Write the failing guards — re-anchor 0218's block and add the two coverage cases**

Replace the whole `--- change 0218: auto-capture no longer absorbs this branch's own findings ---` block in `tests/test_docket_review.sh` (it currently begins with `AC="$REPO/skills/docket-convention/references/auto-capture.md"` and ends with the `the exemption says why — no branch, no fix loop at harvest` assert) with the block below. Everything before and after it is untouched.

Three deliberate changes to the surviving 0218 asserts, each recorded in the comments below: the line floor rises `20 -> 60` (the file roughly doubles); the three phrase asserts read a **flattened** `$ac_bar` per `phrase-grep-over-wrapped-prose`; the `awk` extractor keeps its newline-bearing input, since flattening its input would collapse the file into one line and make the range match everything.

```bash
# --- change 0218: auto-capture no longer absorbs this branch's own findings ---
# Extended by change 0226, which reframed this reference as a capability-discovery pipeline. The
# 0218 clauses below are RE-ANCHORED to the new wording, never dropped: the reframe adds the
# positive half (what to look for, and the gates it must clear) and must not soften the negative.
AC="$REPO/skills/docket-convention/references/auto-capture.md"
ac_body="$(cat "$AC" 2>/dev/null)"
# Floor raised 20 -> 60 by 0226: the file roughly doubled, and a floor that a half-deleted file
# still clears is not a non-vacuity anchor.
assert "auto-capture: reference is non-vacuous (>= 60 lines)" \
  '[ "$(printf "%s\n" "$ac_body" | grep -c .)" -ge 60 ]'
ac_flat="$(flatten <<<"$ac_body")"
# Scoped to the Materiality bar SECTION, not the whole file: a whole-file grep would match the
# clause wherever it landed, including a passing mention in the mint paragraph, which is not where
# the bar is applied. The section extractor gets its own non-vacuity anchor for the same reason.
# The extractor keeps NEWLINE-BEARING input on purpose — an awk range over a flattened file has one
# line to range over, so the slice would become the whole file.
ac_bar="$(awk '/^## Materiality bar/{f=1;next} /^## /{f=0} f' "$AC")"
assert "auto-capture: the Materiality bar section was located (non-vacuity anchor)" \
  '[ -n "$ac_bar" ]'
# The three clause asserts below read a FLATTENED slice (learnings: phrase-grep-over-wrapped-prose):
# each pattern can span a line break, so against raw prose they double as line-wrap guards and a
# pure re-flow reddens asserts about policy that never changed.
ac_bar_flat="$(flatten <<<"$ac_bar")"
assert "auto-capture: the Materiality bar slice survives flattening (non-vacuity anchor)" \
  '[ -n "$ac_bar_flat" ]'
# Proximity-shaped, not a bare "in-branch" presence check. The clause this guards is "work THE
# CURRENT RUN WILL FIX in-branch FAILS THE BAR"; the sentence after it independently says the
# finding "is fixed in-branch", so a presence grep stayed GREEN when the rule-bearing clause was
# deleted (observed while mutation-testing this assert). Key it on in-branch fixability sitting next
# to failing the bar — the `[^.]` class keeps the pair inside one sentence.
#
# The scoping ("the current run will fix") is INSIDE the keyed shape, not a separate assert: this
# reference is shared by mint sites with no branch and no fix loop (the finalize/status harvest),
# and an unscoped bar tells those sites to drop precisely the follow-up nothing else will pick up.
# Dropping the scope back to the unscoped "work fixable by a small in-branch edit" wording must
# redden here, so the run-will-fix qualifier is load-bearing in the pattern.
assert "auto-capture: work the current run will fix in-branch fails the bar" \
  'grep -qiE "current run will fix in-branch[^.]{0,40}fails the bar" <<<"$ac_bar_flat"'
# The other half of the scoping: the harvest sites must be told the clause does not reach them,
# or a harvest-time reader applies the fix-loop caller's rule with no fix loop to apply it with.
assert "auto-capture: the harvest sites are exempt from the in-branch clause" \
  'grep -qiE "harvest[^.]{0,20}exempt" <<<"$ac_bar_flat"'
assert "auto-capture: the exemption says why — no branch, no fix loop at harvest" \
  'grep -qiE "no open branch[^.]{0,20}no fix loop" <<<"$ac_bar_flat"'

# --- change 0226: the reference is a DISCOVERY pipeline, gated -----------------------------
# Case one of the two the spec requires: a discovery that QUALIFIES as a new change. The file must
# actively instruct a search, not merely permit one, and must state the gates that search clears.
ac_look="$(awk '/^## What to look for/{f=1;next} /^## /{f=0} f' "$AC")"
assert "auto-capture: the 'What to look for' section was located (non-vacuity anchor)" \
  '[ -n "$ac_look" ]'
ac_look_flat="$(flatten <<<"$ac_look")"
# Shape, not spelling: an imperative to LOOK FOR work, near the property that makes it mintable.
assert "auto-capture: the reader is told to look for independently valuable work" \
  'grep -qiE "look for[^.]{0,200}(worth its own change|independently valuable)" <<<"$ac_look_flat"'
# The six discovery categories the spec enumerates. Keyed one per assert so deleting any single
# category reddens by name — a single "the categories are present" assert would not.
for cat in "reusable capabilit" "product or workflow feature" "policy or lifecycle" \
           "tooling opportunit" "architectural gap" "outlives the active change"; do
  assert "auto-capture: discovery category present: '$cat'" \
    'grep -qiF -- "'"$cat"'" <<<"$ac_look_flat"'
done
ac_gates="$(awk '/^## Admission gates/{f=1;next} /^## /{f=0} f' "$AC")"
assert "auto-capture: the 'Admission gates' section was located (non-vacuity anchor)" \
  '[ -n "$ac_gates" ]'
# All SIX gates, as an ordered list — a prose paragraph that merely mentions them would let the
# count drift silently. Six numbered items is the shape the spec pins.
n_gates="$(grep -cE '^[0-9]+\. ' <<<"$ac_gates")"
assert "auto-capture: exactly six numbered admission gates (found $n_gates)" \
  '[ "$n_gates" -eq 6 ]'
ac_gates_flat="$(flatten <<<"$ac_gates")"
for gate in "outside the scope" "independently valuable" "more than a defect" \
            "boundary" "separate change" "without expanding"; do
  assert "auto-capture: admission gate names '$gate'" \
    'grep -qiF -- "'"$gate"'" <<<"$ac_gates_flat"'
done

# Case two: a current-branch finding that must NOT become a change. The never-mint list is the
# suppression half; it lives with the gates so a reader who reaches the gates cannot miss it.
assert "auto-capture: a review finding about the active diff is never minted" \
  'grep -qiE "never mint[^.]{0,400}review finding about the active diff" <<<"$ac_gates_flat"'
assert "auto-capture: work implement-next fixes in the current branch is never minted" \
  'grep -qiE "never mint[^.]{0,400}fix in the current branch" <<<"$ac_gates_flat"'
assert "auto-capture: cleanup with no independent value is never minted" \
  'grep -qiE "never mint[^.]{0,400}no independent value" <<<"$ac_gates_flat"'
assert "auto-capture: a vague idea with no boundary is never minted" \
  'grep -qiE "never mint[^.]{0,400}(vague idea|no clear outcome)" <<<"$ac_gates_flat"'

# --- change 0226: routing is site-dependent, and site C keeps its own bar -------------------
ac_route="$(awk '/^## Routing/{f=1;next} /^## /{f=0} f' "$AC")"
assert "auto-capture: the 'Routing' section was located (non-vacuity anchor)" \
  '[ -n "$ac_route" ]'
for route in "fix-in-branch" "record-as-learning" "report-only" "capture-as-new-change"; do
  assert "auto-capture: routing names the '$route' route" \
    'grep -qF -- "'"$route"'" <<<"$ac_route"'
done
# Row-shaped, deliberately NOT flattened: flattening the table would let one row's cells satisfy a
# pattern keyed on another row (the bridging failure the fix-loop guards above record).
assert "auto-capture: routing has a row for the implement-next reconcile site" \
  'grep -qE "^\| A [^|]*\|" <<<"$ac_route"'
assert "auto-capture: routing has a row for the implement-next review site" \
  'grep -qE "^\| B [^|]*\|" <<<"$ac_route"'
assert "auto-capture: routing has a row for the finalize/status harvest site" \
  'grep -qE "^\| C [^|]*\|" <<<"$ac_route"'
# The load-bearing asymmetry: fix-in-branch is UNAVAILABLE at site C. A guard that only checked the
# row exists would pass with the whole exemption flattened away.
c_row="$(grep -E "^\| C [^|]*\|" <<<"$ac_route")"
assert "auto-capture: site C marks fix-in-branch unavailable" \
  'grep -qiE "unavailable|\*\*no\*\*" <<<"$c_row"'
ac_route_flat="$(flatten <<<"$ac_route")"
assert "auto-capture: site C keeps its own admission bar, not the six gates" \
  'grep -qiE "site C[^.]{0,120}own admission bar" <<<"$ac_route_flat"'

# --- change 0226: the five capture fields live UNDER a leading `## Why` ---------------------
# mint-stub.sh hard-rejects a body that does not START with `## Why`, so the fields must be labelled
# lines under that one heading. The negative assert is the load-bearing one (learnings:
# assert-detects-removal-not-replacement): promoting any field to a top-level `##` is the exact
# defect that would ship a body mint-stub rejects, and a presence-only guard would stay green.
ac_fields="$(awk '/^## What a captured discovery says/{f=1;next} /^## /{f=0} f' "$AC")"
assert "auto-capture: the capture-fields section was located (non-vacuity anchor)" \
  '[ -n "$ac_fields" ]'
assert "auto-capture: the capture body starts with a leading '## Why'" \
  'grep -qE "^## Why$" <<<"$ac_fields"'
for fld in Trigger Opportunity "Independent value" Boundary "Reason for deferral"; do
  assert "auto-capture: capture field present: '$fld'" \
    'grep -qF -- "**'"$fld"'**" <<<"$ac_fields"'
  assert "auto-capture: capture field '$fld' is NOT a top-level heading (mint-stub body contract)" \
    '! grep -qiE "^## '"$fld"'" <<<"$ac_fields"'
done
assert "auto-capture: the mint-stub '## Why' body contract is stated where the fields are" \
  'grep -qiF -- "mint-stub" <<<"$(flatten <<<"$ac_fields")"'
```

- [ ] **Step 2: Run the guards to verify they fail**

Run: `bash tests/test_docket_review.sh 2>&1 | grep -c '^FAIL'`
Expected: a non-zero count — every `change 0226` assert fails (the sections do not exist yet), and the `>= 60` line floor fails against the 51-line file. The surviving 0218 asserts still pass.

- [ ] **Step 3: Rewrite the reference**

Replace the entire contents of `skills/docket-convention/references/auto-capture.md` with:

````markdown
# auto-capture — the full shared definition

Auto-capture exists to **discover independently valuable capability** — work worth its own change
that surfaces while you are doing something else — and to file it before it is forgotten. The gates
below are what keep that from becoming stub churn: the active change's own work never becomes a
stub. The mechanics behind the convention's *Auto-capture (shared definition)* summary — read
before minting or suppressing a discovered stub. Loaded on demand from `docket-convention/SKILL.md`;
sibling files are not auto-loaded with the skill.

## What to look for

Actively look for work that is worth its own change — this pass is the only one that will see it:

- **reusable capabilities** — a mechanism this repo would use again if it existed;
- **new product or workflow features** — behavior a user or an operating skill would ask for;
- **missing policy or lifecycle behavior** — a state, transition, or gate that nothing owns;
- **tooling opportunities** — a deterministic script that would replace repeated model judgment;
- **architectural gaps** — a boundary asserted in prose and owned by no code;
- **improvements whose value outlives the active change** — worth doing even with this change
  reverted.

Finding it is the point of the pass; admitting it is gated.

## Admission gates

Capture only when the discovery clears **all six**. It must:

1. fall **outside the scope** of the active change;
2. have **independently valuable** outcomes — they stand up with the active change reverted;
3. be **more than a defect** or review finding in the current implementation;
4. have a clear, defensible **boundary** — you can say where the work stops;
5. be concrete enough to describe as a **separate change** — a title, a why, an outcome;
6. be work that cannot reasonably be completed on the active branch **without expanding** that
   branch's intended scope.

**Never mint** for: a review finding about the active diff; a bug or regression the active change
introduced; work `docket-implement-next` is expected to fix in the current branch; minor cleanup or
refactoring with no independent value; documentation needed to complete the active change; a vague
idea with no clear outcome or boundary.

## Materiality bar

Mint only for *actionable follow-up work that would be its own change / PR*
("would a human file this as a `docket-new-change`?"). A build lesson → the **learnings** harvest;
drift inside the current change → the **reconcile log**; a bare observation → the run report.

**Work the current run will fix in-branch fails the bar** (change 0218). A review finding about the
diff currently on the branch is **never mintable** — it is fixed in-branch or recorded in the PR
body, per `docket-implement-next`'s fix loop. A stub costs a title, an id, a groom, a spec, a plan,
a branch, a PR, and a close-out; a dead line of code costs one deletion, and routing the second
through the machinery built for the first is what made the backlog self-generating. Minting from
review survives only for genuinely distinct, beyond-the-branch work that clears the bar on its own.

**That clause binds only where a branch and a fix loop exist** — `docket-implement-next`'s reconcile
and review mint sites. **The `docket-finalize-change` / `docket-status` harvest is exempt**: it runs
with no open branch and no fix loop, so no run there fixes anything in-branch. Cheap-to-fix work
found at harvest is exactly what nothing else picks up — judge it on the *own change / PR* test
above.

## Routing

Four routes for a discovery: **fix-in-branch**, **record-as-learning**, **report-only**, and
**capture-as-new-change**. Fix-in-branch exists only where the site has an open branch AND a live
fix loop, so the available space differs per site:

| Site | Branch + fix loop | Routing |
|---|---|---|
| A — `docket-implement-next` reconcile | yes | all four; a discovery here is usually drift → the **reconcile log** |
| B — `docket-implement-next` review | yes | the **fix loop is the default** consumer (`REVIEW_MIN_FIX_SEVERITY` gates entry; blockers regardless); capture is the narrow exception |
| C — `docket-finalize-change` / `docket-status` harvest | **no** | fix-in-branch **unavailable**; the other three are the whole space |

**Site C keeps its own admission bar** — the *would a human file this as its own change / PR* test
of the *Materiality bar* above, not the six gates. With no branch and no fix loop, applying the
stricter capability-discovery gates there would suppress the cheap-to-fix follow-up that nothing
else picks up.

## What a captured discovery says

`mint-stub.sh` rejects any `--body-file` whose contents do not **start with `## Why`** (validated
before any write; exit 1). The five required fields are therefore labelled lines *under* that one
leading heading — never five top-level sections:

```markdown
## Why

**Trigger** — what surfaced this, and while doing what.
**Opportunity** — the capability that does not exist today.
**Independent value** — what it is worth with the active change reverted.
**Boundary** — where the work stops, and what it deliberately leaves alone.
**Reason for deferral** — why it cannot ride the active branch without expanding its scope.
```

## Per discovery

**Per discovery** (after the gates and the materiality bar): assign exactly one type from
`CHANGE_TYPES` — the model classifies, the script never infers (ADR-0012). `AUTO_CAPTURE_ENABLED:
false` ⇒ report, mint nothing. Enabled but the type is outside `AUTO_CAPTURE_TYPES` (the literal
`all`, or a subset) ⇒ mint nothing, report it as **policy-suppressed**. Enabled and admitted ⇒
`mint-stub --type`. Every outcome keeps ADR-0045's best-effort posture. **Type filtering runs before
the cap is consumed** — a suppressed candidate must never spend a mint slot; dedup stays after
admission.

## The deterministic mint

**The mint itself is deterministic** (ADR-0012 — the model judges *what*, the script does the mint):
`"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh mint-stub --changes-dir .docket/<changes_dir>
--title <title> --type <type> --body-file <file> --discovered-from <this change's id> --minted <n so far>` (in
`docket`-mode; in `main`-mode, `--changes-dir <changes_dir>` — the metadata worktree IS the primary
tree) — one stub per call, `--body-file` **must start with `## Why`**, contract in
`scripts/mint-stub.md`. **`<n so far>` is the running count across the whole run on a single
change, never reset per mint site** — a skill with two mint sites (`docket-implement-next`'s
reconcile and review) carries the total forward. (`docket-status`'s sweep scopes it per swept
change — see its SKILL.md.) It owns dedup, id allocation, the template write, and the CAS push;
**exit 3** = duplicate skipped, **exit 4** = cap (3) reached, **exit 1** = a real error (push
failure, malformed body, retry exhaustion). Every skip, overflow, and exit-1 failure is **surfaced
in the run report, never silently dropped** — but none is fatal: **auto-capture is best-effort and
must never abort the change being built**, because capture is a courtesy while the change is the
job. Minting is a metadata-worktree write only — it never touches the running change's own
claim/branch/PR state.
````

- [ ] **Step 4: Measure the file and raise its budget row**

Run: `wc -l -w skills/docket-convention/references/auto-capture.md`

Apply the rounding rule from `tests/test_skill_size_budgets.sh`'s BUDGETS comment to the measured actual: lines up to the next multiple of 5, words up to the next multiple of 50 — if either lands within its near-zero threshold (0 lines of margin, or ≤ 25 words of margin), take the multiple after it. Edit the row in the `BUDGETS` heredoc:

```
skills/docket-convention/references/auto-capture.md         55  600
```

to the computed values (the spec's target is ≈ `100 1150`; use the measured numbers, not this estimate).

Append this justification block to the comment section above `BUDGETS=` — immediately after the existing block that begins `# That same row was raised again, 50/550 -> 55/600, by a 0218 review fix:` — filling in the measured actuals and the computed row:

```
# skills/docket-convention/references/auto-capture.md's budget was raised 55/600 -> <L>/<W> by
# change 0226, which reframed the file from a suppression rule into a capability-discovery pipeline:
# it ADDS the positive half the file never had — six discovery categories to search for, six
# admission gates the discovery must clear, the five capture fields a minted body carries, and a
# per-site routing table — on top of every suppression rule, which is carried forward unweakened.
# The prose has no other home. This file IS the definition every mint site reads before minting, and
# each added part is a rule applied at that same moment: the categories are what the reader searches
# with, the gates are what admits, the fields are what the body must contain to survive
# mint-stub.sh's `## Why` contract, and the routing table is what tells a site whether fix-in-branch
# even exists for it. The considered home was skills/docket-implement-next/references/fix-loop.md
# (already the considered-and-rejected home for the 0218 raises above): it is read ONLY by the
# implementer's Step 6, so the finalize/status harvest — a mint site that never reads the
# implementer's references — would get the reframe's gates and none of its site-C carve-out. The
# convention's SKILL.md summary was considered and rejected under progressive disclosure: it is
# loaded on every skill invocation, and the detail here is read only when a discovery is in hand.
# Set per the rounding rule above from the measured actual: <lines> lines -> <L>; <words> words ->
# <W>.
```

- [ ] **Step 5: Run the guards and the budget test to verify they pass**

Run: `bash tests/test_docket_review.sh; echo "exit=$?"`
Expected: `exit=0`, no `FAIL - ` lines.

Run: `bash tests/test_skill_size_budgets.sh; echo "exit=$?"`
Expected: `exit=0`, no `NOT OK - ` lines.

- [ ] **Step 6: Mutation-test the two new coverage cases**

Prove each new guard family bites. For each mutation: apply it, confirm with `grep -c` that the edit actually landed (a silently-unmatched substitution yields a green run with nothing mutated — see `assert-detects-removal-not-replacement`), run `bash tests/test_docket_review.sh`, confirm it reddens, then `git checkout -- skills/docket-convention/references/auto-capture.md`.

```bash
AC=skills/docket-convention/references/auto-capture.md

# M1 — qualifying-discovery case: delete a discovery category.
grep -c 'tooling opportunit' "$AC"                      # expect >= 1
perl -0pi -e 's/^- \*\*tooling opportunities\*\*.*\n//m' "$AC"
grep -c 'tooling opportunit' "$AC"                      # expect 0 — the mutation LANDED
bash tests/test_docket_review.sh | grep -c '^FAIL'      # expect >= 1
git checkout -- "$AC"

# M2 — qualifying-discovery case: drop one admission gate (six -> five).
grep -cE '^[0-9]+\. ' "$AC"                             # note the count
perl -0pi -e 's/^6\. be work that cannot.*?scope\.\n//ms' "$AC"
grep -cE '^[0-9]+\. ' "$AC"                             # expect one fewer — mutation LANDED
bash tests/test_docket_review.sh | grep -c '^FAIL'      # expect >= 1
git checkout -- "$AC"

# M3 — current-branch-finding case: delete the never-mint list.
grep -c 'Never mint' "$AC"                              # expect 1
perl -0pi -e 's/\*\*Never mint\*\* for:.*?boundary\.\n//ms' "$AC"
grep -c 'Never mint' "$AC"                              # expect 0 — mutation LANDED
bash tests/test_docket_review.sh | grep -c '^FAIL'      # expect >= 1
git checkout -- "$AC"

# M4 — site-C exemption: flatten site C to look like A and B.
grep -c 'unavailable' "$AC"                             # expect >= 1
perl -0pi -e 's/fix-in-branch \*\*unavailable\*\*/all four routes/' "$AC"
grep -c 'unavailable' "$AC"                             # expect one fewer — mutation LANDED
bash tests/test_docket_review.sh | grep -c '^FAIL'      # expect >= 1
git checkout -- "$AC"

# M5 — mint-stub body contract: promote a capture field to a top-level heading.
perl -0pi -e 's/^\*\*Trigger\*\* —/## Trigger —/m' "$AC"
grep -cE '^## Trigger' "$AC"                            # expect 1 — mutation LANDED
bash tests/test_docket_review.sh | grep -c '^FAIL'      # expect >= 1
git checkout -- "$AC"
```

If any mutation leaves the suite green, the corresponding assert is decoration — fix the assert (narrow its scope, or key it on the shape the mutation destroys) before continuing.

- [ ] **Step 7: Re-flow control for the flattened asserts**

The flattened phrase asserts must survive a pure re-wrap (learnings: `phrase-grep-over-wrapped-prose`). Re-flow the *Materiality bar* section at two widths and confirm the suite stays green:

```bash
AC=skills/docket-convention/references/auto-capture.md
cp "$AC" /tmp/ac-orig.md
for w in 78 110; do
  awk -v w="$w" '/^## Materiality bar/{f=1} /^## Routing/{f=0} {print}' "$AC" >/dev/null
  perl -0pi -e "s/(## Materiality bar\n)(.*?)(\n## Routing)/\$1.join(\"\\n\", split(\/(?<=.{0,$w})\\s+\/, \$2)).\$3/se" "$AC"
  bash tests/test_docket_review.sh | grep -c '^FAIL'    # expect 0
  cp /tmp/ac-orig.md "$AC"
done
```

If the re-flow reddens an assert, that assert is reading unflattened input — route it through `$ac_bar_flat` / `$ac_look_flat` / `$ac_gates_flat` and re-run. Restore the original file when done: `cp /tmp/ac-orig.md "$AC"`.

- [ ] **Step 8: Run the whole suite**

Run the full-suite command from *Global Constraints*.
Expected: `RED:[]`.

`tests/test_convention_extraction.sh` is the one most likely to react to a reference rewrite (it samples convention sections for the reference-never-restate rule) — if it reddens, read its assert and repoint it at the artifact that now owns the content rather than restoring deleted text (learnings: `restatement-accumulates-its-own-guards`).

- [ ] **Step 9: Commit**

```bash
git add skills/docket-convention/references/auto-capture.md tests/test_docket_review.sh tests/test_skill_size_budgets.sh
git commit -m "docs(0226): reframe auto-capture as a gated capability-discovery pipeline

Adds the positive half the reference never had — six discovery categories,
six admission gates, the five capture fields under mint-stub's leading
'## Why', and a per-site routing table where fix-in-branch exists only with
an open branch and a live fix loop. Every suppression rule carries forward
unweakened; change 0218's guard clauses are re-anchored, not dropped, and
its phrase asserts now read a flattened haystack so a re-flow cannot redden
policy that did not change. Budget row raised in-diff with justification."
```

---

## Task 2: Reframe the convention's inline summary under progressive disclosure

**Files:**
- Modify: `skills/docket-convention/SKILL.md` — the `### Auto-capture (shared definition)` section only
- Modify: `tests/test_skill_size_budgets.sh` — the `skills/docket-convention/SKILL.md` row, **only if** the rewrite does not fit
- Modify: `tests/test_docket_review.sh` — add the progressive-disclosure guard

**Interfaces:**
- Consumes: Task 1's section headings in `references/auto-capture.md` (`## What to look for`, `## Admission gates`, `## Routing`, `## What a captured discovery says`) — the summary must not duplicate their content.
- Produces: nothing later tasks consume; this is the last task.

- [ ] **Step 1: Write the failing guard — the summary carries intent + pointer, not the detail**

Append this block to `tests/test_docket_review.sh`, immediately after the `change 0226: the five capture fields live UNDER a leading '## Why'` block from Task 1.

```bash
# --- change 0226: the convention SUMMARY stays a summary (progressive disclosure) -----------
# The summary is what a mint site reads inline before deciding whether to drill down. It must carry
# the intent and the pointer; the categories, gates, fields, and routing table live ONLY in the
# reference. The negative asserts are the load-bearing half: a well-meaning future edit that copies
# the gates up here is exactly the restatement class this project keeps paying for, and a
# presence-only guard would never see it.
CONV="$REPO/skills/docket-convention/SKILL.md"
ac_sum="$(awk '/^### Auto-capture \(shared definition\)/{f=1;next} /^### /{f=0} f' "$CONV")"
assert "convention: the Auto-capture summary section was located (non-vacuity anchor)" \
  '[ -n "$ac_sum" ]'
ac_sum_flat="$(flatten <<<"$ac_sum")"
assert "convention: the summary leads with capability-discovery intent" \
  'grep -qiE "(capability|independently valuable)[^.]{0,160}(discover|discovery)" <<<"$ac_sum_flat"'
assert "convention: the summary names the strict admission gating without enumerating it" \
  'grep -qiE "admission|gate" <<<"$ac_sum_flat"'
assert "convention: the summary keeps its blocking drill-down pointer" \
  'grep -qF -- "references/auto-capture.md" <<<"$ac_sum_flat"'
assert "convention: the drill-down pointer is BLOCKING" \
  'grep -qiE "blocking" <<<"$ac_sum_flat"'
# Progressive disclosure, asserted as absence. Each of these is a thing the reference owns.
assert "convention: the summary does NOT enumerate the discovery categories" \
  '! grep -qiE "reusable capabilit|tooling opportunit" <<<"$ac_sum_flat"'
assert "convention: the summary does NOT enumerate the six admission gates" \
  '! grep -qE "^[0-9]+\. " <<<"$ac_sum"'
assert "convention: the summary does NOT carry the routing table" \
  '! grep -qE "^\| " <<<"$ac_sum"'
assert "convention: the summary does NOT carry the five capture fields" \
  '! grep -qiE "reason for deferral" <<<"$ac_sum_flat"'
# Mechanics that MUST stay inline — the summary is not a bare pointer either.
for tok in AUTO_CAPTURE_ENABLED AUTO_CAPTURE_TYPES policy-suppressed docket-auto-groom; do
  assert "convention: the summary keeps the '$tok' mechanic inline" \
    'grep -qF -- "'"$tok"'" <<<"$ac_sum_flat"'
done
```

- [ ] **Step 2: Run the guard to verify it fails**

Run: `bash tests/test_docket_review.sh 2>&1 | grep '^FAIL'`
Expected: the two intent asserts fail (`leads with capability-discovery intent`, `names the strict admission gating`) — the current summary opens on config mechanics. The absence asserts and the mechanics asserts already pass.

- [ ] **Step 3: Rewrite the summary in place**

In `skills/docket-convention/SKILL.md`, replace the three paragraphs under `### Auto-capture (shared definition)` (from ``​`auto_capture` (a map: `enabled` default `false`…`` through `…the cross-site `--minted` count carry-forward.`) with:

```markdown
Auto-capture is **capability discovery under strict admission gates**: an autonomous skill actively
looks for independently valuable work it discovers mid-run — and files it only if that work clears
every gate. `auto_capture` (a map: `enabled` default `false`, `types` default `all`; global-able —
resolved as `AUTO_CAPTURE_ENABLED` / `AUTO_CAPTURE_TYPES`) governs what happens then. Disabled, the
model reports it in prose; enabled, it **classifies** the work and — only if that type is admitted —
mints an ordinary `proposed` needs-brainstorm stub (`mint-stub --type`, one per call) with
`discovered_from:` and `type:` set. Capture fidelity, **not** autonomy: every stub still waits at
the human's groom gate. A type outside policy is reported as **policy-suppressed**, never minted —
and type filtering runs **before the cap** is consumed.

**Mint sites** are the autonomous *single-change* skills: `docket-implement-next` (reconcile and
review) and the `docket-finalize-change` / `docket-status` harvest. **`docket-auto-groom` is never a
mint site** — a minted stub is itself autonomous-eligible, so minting would break its
provable-termination invariant. **Interactive skills need no auto-capture path** — a human is
present to decide what gets filed.

Discovered follow-up work mid-run → **read [`references/auto-capture.md`](references/auto-capture.md)
now (blocking)** before minting or suppressing — it owns what to look for, the admission gates and
the suppression list, the materiality bar, the per-site routing, what a captured discovery must say,
and the deterministic mint with its exit codes and cross-site `--minted` carry-forward.
```

- [ ] **Step 4: Measure and decide whether the SKILL.md row needs a raise**

Run: `wc -l -w skills/docket-convention/SKILL.md`

The row is `skills/docket-convention/SKILL.md 345 5850`; the pre-change actual was 339 lines / 5804 words.

- If both measurements are within budget, **do not touch the row** — this resolves the change's open question by measurement.
- If either exceeds, raise only the exceeded dimension per the rounding rule (lines → next multiple of 5, words → next multiple of 50, taking the multiple after if the margin is 0 lines or ≤ 25 words), and append this justification block to the comment section above `BUDGETS=`, after the block added in Task 1:

```
# skills/docket-convention/SKILL.md's <LINE|WORD> budget was raised <old> -> <new> by change 0226,
# which reframed the Auto-capture shared definition's opening from config mechanics to the intent
# the mechanics serve — auto-capture is capability discovery under strict admission gates — and
# widened the blocking drill-down pointer to name what the reference now owns. The rewrite was taken
# in place and compression paid for most of it; the residual is the intent sentence, which cannot
# move to skills/docket-convention/references/auto-capture.md (the considered home): a reader who
# has not yet decided a discovery is in hand never opens the reference, and the intent sentence is
# precisely what makes them look. Progressive disclosure is preserved — the categories, the six
# gates, the five capture fields, and the routing table live only in the reference, asserted as
# ABSENCE from this section by tests/test_docket_review.sh. Set per the rounding rule above from the
# measured actual: <actual> -> <new>.
```

- [ ] **Step 5: Run the guard and the budget test to verify they pass**

Run: `bash tests/test_docket_review.sh; echo "exit=$?"`
Expected: `exit=0`.

Run: `bash tests/test_skill_size_budgets.sh; echo "exit=$?"`
Expected: `exit=0`.

- [ ] **Step 6: Mutation-test the progressive-disclosure guard**

```bash
CONV=skills/docket-convention/SKILL.md
cp "$CONV" /tmp/conv-orig.md

# M1 — the intent assert bites: revert the summary's opening to config-first.
perl -0pi -e 's/Auto-capture is \*\*capability discovery under strict admission gates\*\*.*?governs what happens then\./`auto_capture` governs what an autonomous skill does with genuine follow-up work it discovers mid-run./s' "$CONV"
grep -c 'capability discovery under strict admission gates' "$CONV"   # expect 0 — mutation LANDED
bash tests/test_docket_review.sh | grep -c '^FAIL'                    # expect >= 1
cp /tmp/conv-orig.md "$CONV"

# M2 — the absence asserts bite: copy the reference's gates up into the summary.
perl -0pi -e 's/(\*\*Mint sites\*\* are the autonomous)/1. falls outside the scope of the active change;\n\n$1/' "$CONV"
grep -cE '^1\. falls outside the scope' "$CONV"                       # expect 1 — mutation LANDED
bash tests/test_docket_review.sh | grep -c '^FAIL'                    # expect >= 1
cp /tmp/conv-orig.md "$CONV"
```

If either mutation leaves the suite green, fix the assert before continuing.

- [ ] **Step 7: Run the whole suite**

Run the full-suite command from *Global Constraints*.
Expected: `RED:[]`.

`tests/test_convention_extraction.sh` samples convention sections for the reference-never-restate rule and is the likeliest reactor; `tests/test_typed_changes_docs.sh` guards `auto_capture:` config-example shapes and must stay green (this task edits prose, not examples). Repoint rather than restore if either reddens.

- [ ] **Step 8: Commit**

```bash
git add skills/docket-convention/SKILL.md tests/test_docket_review.sh tests/test_skill_size_budgets.sh
git commit -m "docs(0226): reframe the convention's auto-capture summary to intent + pointer

The inline summary now leads with what auto-capture is for — capability
discovery under strict admission gates — keeps the config, classification,
policy-suppression, and mint-site mechanics, and widens the blocking
drill-down pointer to name what the reference owns. Progressive disclosure
is enforced as absence: the categories, the six gates, the five capture
fields, and the routing table are asserted NOT to appear here."
```

---

## Self-Review

**1. Spec coverage.** Every acceptance criterion maps to a step:

| Spec acceptance criterion | Where |
|---|---|
| leads with the intent to discover independently valuable capabilities | T1 S3 (opening paragraph), guarded T1 S1 |
| instructs agents to search for features, workflow improvements, policy gaps, tooling, architecture | T1 S3 `## What to look for`, guarded by the six per-category asserts |
| all existing suppression rules remain in effect | T1 S3 `## Admission gates` never-mint list + the unchanged `## Materiality bar`, guarded by the surviving 0218 asserts and the four never-mint asserts |
| findings about the active change are fixed in-branch, never captured | T1 S3 `## Materiality bar` (0218 clauses carried verbatim), guarded T1 S1 |
| five capture fields under a leading `## Why`, satisfying mint-stub's body contract | T1 S3 `## What a captured discovery says`, guarded by the presence + NOT-a-heading assert pairs |
| routing states four routes, fix-in-branch conditional, site C exempt with its own bar | T1 S3 `## Routing`, guarded by the route, row, `c_row`, and own-admission-bar asserts |
| convention summary reframed to intent + mechanics + pointer, no duplication | T2 S3, guarded T2 S1 (four absence asserts) |
| deterministic change-creation behavior unchanged | Global Constraints; no script is touched by any step |
| auto-capture.md budget row raised with a dated justification block | T1 S4 |
| SKILL.md row raised only if needed, likewise justified | T2 S4 (measure-then-decide) |
| 0218 guard block still passes, re-anchored not removed, line floor raised | T1 S1 (block rewritten in place, floor `20 -> 60`) |
| tests cover a qualifying discovery AND a current-branch finding that must not become one | T1 S1 (the two labelled `change 0226` case blocks), mutation-proved T1 S6 M1–M3 |

**2. Placeholder scan.** The only intentional fill-ins are the measured numbers in T1 S4 and T2 S4 (`<L>`, `<W>`, `<actual>`), which cannot be known before the file is written — each is accompanied by the exact command that produces it and the exact rounding rule that converts it. No step says "add error handling", "similar to Task N", or "write tests for the above".

**3. Type consistency.** Shell variable names are consistent across both tasks: `$AC`/`$ac_body`/`$ac_flat`/`$ac_bar`/`$ac_bar_flat`/`$ac_look`/`$ac_look_flat`/`$ac_gates`/`$ac_gates_flat`/`$ac_route`/`$ac_route_flat`/`$c_row`/`$ac_fields` in Task 1, and `$CONV`/`$ac_sum`/`$ac_sum_flat` in Task 2 — no collisions with the file's existing `$fix_body`/`$fix_flat`/`$tasks_section`/`$dispatch_para`. `flatten` is used, never redefined. Section headings referenced by Task 2's absence asserts are exactly the headings Task 1 writes.
