<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0260 — Tier finalize's in-context dispatches and name the push-denial posture](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-11-0260-tier-finalize-s-in-context-dispatches-and-name-the-push-deni.md)**
<!-- docket:backlink:end -->

# Tier finalize's in-context dispatches and name the push-denial posture — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give `docket-finalize-change`'s two in-context gate dispatches an explicit **carve-out** outside the convention's A/B/C dispatch-capability taxonomy, wire that posture at its single canonical site, add the two missing abort-and-report enumeration members (dispatch-unavailable, and a policy denial of the post-rebase `--force-with-lease` push), and re-wire the guards that pinned the old deferral.

**Architecture:** Documentation + test rewiring only — **no behavior change, no script changes**. Three surfaces move together: the convention gains a carve-out paragraph immediately after its tier table; `skills/docket-finalize-change/references/gate-failure.md` gains one dispatch-unavailability paragraph (the single canonical site marker, blocking-loaded at both dispatch moments) plus two enumeration members and one de-numeralization; the two test files trade the `PENDING_TIER` deferral for ordinary `check_site` coverage plus new sentinels.

**Tech Stack:** Markdown prose (docket skill bodies), Bash 3.2-compatible `grep`/`awk` sentinel tests run by `scripts/run-tests.sh`.

## Global Constraints

Copied verbatim from the spec and from AGENTS.md; every task's requirements implicitly include these.

- **Label literal is the bare hyphenated word `carve-out`** — metacharacter-free, because it is spliced raw into an ERE at `check_site`'s clause-proximity assert. Never `Tier D`, never a multi-word phrase (the coherence loop's `${t##* }` last-word extraction would split on a space).
- **Name the push by its own noun, never by a step number** — "the gate's post-rebase `--force-with-lease` push". `gate-failure.md` already uses "gate-step-5" to mean the **red-suite** step, so a step number would collide.
- **Harness-neutral prose throughout** — no product-specific retry syntax, no dispatch tool name. `tests/test_dispatch_capability.sh`'s negative guard scans `skills/` for the shapes `` `Task` ``, `**Task**`, `Task(`, and `Task` followed by `dispatch|tool|launch`; new prose in `skills/` must match none of them.
- **No changes to the convention's *Composition* paragraph** (spec Assumption 8) — the carve-out cites it, never restates it.
- **No changes to the rebase-resolver / integration-repair contracts**, to when finalize dispatches them, or to the guarded-force-push settings posture in `ensure-claude-settings.sh` (spec *Out of scope*).
- **Derive every site list from a whole-repo grep, never a hand-list** (AGENTS.md). Both population floors in `test_dispatch_capability.sh` are re-derived by *running* the file's own greps, per its MAINTAINER NOTE at the `derived_count` assert.
- **A guard is code: mutation-test it** (AGENTS.md). Every new assert gets a probe that deletes the thing it guards. **Confirm the probe changed bytes** (`grep -c` before and after) before trusting a green — against prose, a probe that silently fails to match is indistinguishable from a guard that held.
- **Deletion and inversion are different probes.** Any assert over a comparison or a two-sided binding gets both.
- **Run the whole suite at the build gate**, via `scripts/run-tests.sh` (the resolved `finalize.test_command`). **Never** run `scripts/run-tests.sh --timings <test path>` against a real test file — it truncates the named file to zero bytes (tracked as #0290, unfixed).
- `tests/test_sync_agents_runners.sh` running ~190s against its 60s ceiling is **pre-existing** (#0280) — not this change's, leave it alone.

## File Structure

| File | Responsibility in this change |
|---|---|
| `skills/docket-convention/SKILL.md` | **Modify.** One new paragraph immediately after the A/B/C tier table: the carve-out definition. The taxonomy's canonical home. |
| `skills/docket-finalize-change/references/gate-failure.md` | **Modify.** One new paragraph (the single canonical site marker for both agents) + two new members in the abort-and-report enumeration + one de-numeralization in the `## Finalize blocked` section. |
| `tests/test_dispatch_capability.sh` | **Modify.** Two new `check_site` rows at `carve-out`; coherence loop skips non-`Tier <letter>` labels behind its own floor; new convention-side carve-out asserts; `PENDING_TIER` empty-pinned; both population floors re-derived. |
| `tests/test_finalize_gate.sh` | **Modify.** New `GF` anchor + sentinels for the two new enumeration members and the de-numeralization. |
| `tests/runtime-budgets.tsv` | **Verify, modify only if measured.** Both touched test files sit at a `10` second ceiling today. |

---

### Task 1: The carve-out paragraph in the convention, and the guards that pin it

**Files:**
- Modify: `skills/docket-convention/SKILL.md` (insert one paragraph immediately after the A/B/C tier table, above the paragraph beginning "Tier C neither replaces nor softens")
- Test: `tests/test_dispatch_capability.sh` (new asserts appended to the "the tiered posture" region, after the existing Tier C asserts)

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: the literal `carve-out` as the posture label, and a paragraph containing **both** agent nouns `docket-rebase-resolver` and `docket-integration-repair`. Task 2 reuses the same label literal in `gate-failure.md`; Task 3's `check_site` rows pass `carve-out` as their expected-posture argument and its coherence-loop replacement asserts against this paragraph.

- [ ] **Step 1: Write the failing tests**

Append to `tests/test_dispatch_capability.sh`, immediately after the existing `assert "convention: Tier C halt adds no new status or field"` block (currently ending at the line `  'grep -qiE "[Nn]o new status, no new field" "$CONV"'`) and before the `# --- the boundary against the pre-existing missing-skill rule ---` banner:

```bash
# --- the carve-out: the two finalize gate dispatches sit OUTSIDE the taxonomy (change 0260) -------
# Their contract is an in-context report gating the merge, not git state on metadata_branch, so
# neither Tier A's "inline is a first-class equivalent" nor Tier C's authorized-or-halt can apply.
# Paragraph-scoped, not file-scoped: the paragraph is identified BY the label literal, so
# membership in it IS the binding "this noun is carve-out-classified" (learnings:
# prose-guard-binds-phrase-to-claim). A file-wide `grep -q docket-rebase-resolver "$CONV"` would be
# satisfied by the *Composition* paragraph, which names both agents for an unrelated reason.
carveout_para="$(awk 'BEGIN{RS="";} /carve-out/ {print; exit}' "$CONV")"
assert "convention: a carve-out paragraph exists (anchor for the asserts below)" \
  '[ -n "$carveout_para" ]'
assert "convention carve-out: names docket-rebase-resolver" \
  'grep -qF -- "docket-rebase-resolver" <<<"$carveout_para"'
assert "convention carve-out: names docket-integration-repair" \
  'grep -qF -- "docket-integration-repair" <<<"$carveout_para"'
assert "convention carve-out: states the posture is finalize's abort-and-report" \
  'grep -qF -- "abort-and-report" <<<"$carveout_para"'
assert "convention carve-out: forbids inline substitution" \
  'grep -qiE "[Ii]nline substitution is forbidden" <<<"$carveout_para"'
assert "convention carve-out: gives the self-approval reason for that prohibition" \
  'grep -qF -- "self-approval" <<<"$carveout_para"'
# The reason these two are OUT of the table, in the paragraph's own words — without it the
# carve-out reads as an unexplained exception and a later editor folds it back into a row.
assert "convention carve-out: says their contract is an in-context report, not git state" \
  'grep -qE -- "in-context report[^.]{0,80}gating the merge" <<<"$carveout_para"'

# The swapped-subjects blind spot, negative direction: neither carved-out noun may appear in an
# A/B/C tier ROW. Without this, moving `docket-integration-repair` into the Tier C row (claiming
# authorized-or-halt for it, so `skills.build: auto` would authorize inline repair by the agent
# that then merges it) keeps every positive assert above green.
tier_rows_all="$(grep -E "^\| \*\*[A-Z] —" "$CONV")"
# Non-vacuity companion through the SAME extractor: an absence assert over a dead extractor reads
# as the property holding (learnings: assert-detects-removal-not-replacement, rule 5).
assert "convention: the tier-row extractor still reaches the table" \
  '[ "$(grep -c . <<<"$tier_rows_all")" -ge 3 ] && grep -qF -- "docket-status" <<<"$tier_rows_all"'
assert "convention: no tier row claims docket-rebase-resolver (carve-out, not a tier member)" \
  '! grep -qF -- "docket-rebase-resolver" <<<"$tier_rows_all"'
assert "convention: no tier row claims docket-integration-repair (carve-out, not a tier member)" \
  '! grep -qF -- "docket-integration-repair" <<<"$tier_rows_all"'
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_dispatch_capability.sh 2>&1 | grep -E "NOT OK|^PASS|^FAIL"`

Expected: FAIL, with `NOT OK - convention: a carve-out paragraph exists` and every dependent carve-out assert also NOT OK (they run against an empty `$carveout_para`). The three `tier_rows_all` asserts pass already — they are guarding a property that holds today and must keep holding.

- [ ] **Step 3: Write the carve-out paragraph**

In `skills/docket-convention/SKILL.md`, insert this paragraph as its own blank-line-delimited block immediately **after** the Tier C table row (the line beginning `| **C — discipline** |`) and **before** the paragraph beginning `Tier C neither replaces nor softens`:

```markdown
**Outside the table — the `carve-out` (change 0260).** `docket-finalize-change`'s two merge-gate dispatches — `docket-rebase-resolver` and `docket-integration-repair` — sit **outside** this taxonomy rather than in a row of it, because their contract is an **in-context report** gating the merge, not git state on `metadata_branch`. Neither tier posture can be borrowed for them: Tier A's first-class-equivalent inline path presupposes a git-state transition to reproduce, and Tier C's authorized-or-halt presupposes a `skills:` role whose resolved value could carry a human's `auto` authorization — these dispatches have neither. When dispatch is genuinely unavailable for either — established per the resolution rule above, never from a tool name — the posture is finalize's own pre-existing **abort-and-report**: the gate stops, the PR stays open, the change stays `implemented`, and the reason is recorded through the three channels `docket-finalize-change`'s failure reference owns. **Inline substitution is forbidden** for both, and that is the point of carving them out rather than tiering them: reconciling the conflict, or authoring the repair, inside the very agent that would then merge that work is the same self-approval shape Tier B rejects for the critic.
```

Verify the insertion did not disturb the `Tier C neither replaces` paragraph — Task 3's coherence work and two pre-existing polarity asserts read it.

- [ ] **Step 4: Run the test to verify it passes**

Run: `bash tests/test_dispatch_capability.sh 2>&1 | grep -E "NOT OK|^PASS|^FAIL"`

Expected: PASS. (If `NOT OK - reverse: PENDING_TIER holds exactly the two knowingly-untiered finalize dispatches` appears, the paragraph accidentally introduced a new backticked-name-then-`subagent` shape — reword; that assert is Task 3's to change, not this one's.)

- [ ] **Step 5: Mutation-probe every new assert, confirming the probe landed**

For each probe: take a `grep -c` **before**, apply it, take a `grep -c` **after**, and treat `before == after` as `MUTATION DID NOT LAND` — not as a guard that held. Work on a backup copy and restore from it, never `git checkout --` (learnings: `mutation-restore-needs-a-backup-copy`).

```bash
cp skills/docket-convention/SKILL.md /tmp/conv.bak

probe() { # $1 = human label, $2 = perl expression
  local before after
  before="$(grep -c 'carve-out' skills/docket-convention/SKILL.md)"
  perl -0pi -e "$2" skills/docket-convention/SKILL.md
  after="$(grep -c 'carve-out' skills/docket-convention/SKILL.md)"
  if [ "$before" = "$after" ]; then echo "MUTATION DID NOT LAND: $1 (count $before)"; fi
  bash tests/test_dispatch_capability.sh 2>&1 | grep -E "NOT OK" | sed "s/^/[$1] /"
  cp /tmp/conv.bak skills/docket-convention/SKILL.md
}

# A: delete the whole carve-out paragraph  -> every carve-out assert must redden
probe "delete-paragraph" 's/\*\*Outside the table — the `carve-out`.*?\n\n//s'
# B: drop ONE noun (inversion of "names both") -> exactly the integration-repair assert reddens
probe "drop-one-noun" 's/ and `docket-integration-repair`//'
# C: MOVE a noun into a tier row instead of deleting it -> the tier-row negative must redden
#    while the paragraph asserts stay green. This is the swapped-subjects case; deletion (probe B)
#    cannot reach it, so both probes are required.
probe "promote-noun-into-Tier-C" 's/^(\| \*\*C — discipline\*\* \| the )/$1`docket-integration-repair` plus /m'
# D: weaken the posture to Tier A's inline path -> the abort-and-report / inline-forbidden asserts
probe "delete-inline-prohibition" 's/\*\*Inline substitution is forbidden\*\*/Inline substitution is available/'
```

Expected: probe A reddens the paragraph-existence assert and all five content asserts; B reddens only `names docket-integration-repair`; C reddens only `no tier row claims docket-integration-repair`; D reddens `forbids inline substitution`. **No probe may print `MUTATION DID NOT LAND`.** Any probe that prints it has a pattern problem (most likely a hard-wrap in the target) — fix the probe and re-run before drawing a conclusion.

Restore and confirm clean: `cp /tmp/conv.bak skills/docket-convention/SKILL.md && git diff --stat` shows the intended paragraph only.

- [ ] **Step 6: Commit**

```bash
git add skills/docket-convention/SKILL.md tests/test_dispatch_capability.sh
git commit -m "docs(0260): carve the two finalize gate dispatches out of the A/B/C taxonomy"
```

---

### Task 2: The site marker and the two new abort-and-report members in gate-failure.md

**Files:**
- Modify: `skills/docket-finalize-change/references/gate-failure.md`
- Test: `tests/test_finalize_gate.sh`

**Interfaces:**
- Consumes: the label literal `carve-out` from Task 1, used verbatim here.
- Produces: a paragraph whose two unique anchor phrases — `conflicted rebase whose` and `red rebased suite whose` — Task 3's two `check_site` rows use as their anchor regexes. That paragraph must also contain the exact strings `Dispatch-capability resolution` and `never from a tool name`, which Task 3's inherited asserts require, and must pair each agent noun with the literal `carve-out` **inside one clause** (no `;` or `.` between them, ≤80 characters apart) — `check_site`'s proximity assert is `${noun}[^;.]{0,80}${tier}|${tier}[^;.]{0,80}${noun}`.

- [ ] **Step 1: Write the failing tests**

In `tests/test_finalize_gate.sh`, add the new anchor beside the existing `FIN`/`CONV`/`STAT` declarations (after the line `STAT="$REPO/skills/docket-status/SKILL.md"`):

```bash
GF="$REPO/skills/docket-finalize-change/references/gate-failure.md"   # change 0260: the abort set's canonical home
```

Then append this section immediately before the final `exit $fail`:

```bash
# --- change 0260: the two new abort-and-report members live in gate-failure.md ------------------
# The enumeration is one long line today, but guard it through a whitespace-collapsed haystack
# anyway: a future re-flow must not redden asserts about policy that did not change (learnings:
# phrase-grep-over-wrapped-prose).
gf_flat="$(tr '\n' ' ' < "$GF" | tr -s '[:space:]' ' ')"
assert "0260: gate-failure.md is reachable (non-vacuity for the flattened asserts below)" \
  '[ "${#gf_flat}" -gt 500 ] && grep -qF -- "abort-and-report points" <<<"$gf_flat"'

# Member 1 — a POLICY denial of the gate's own post-rebase push. Bound to the push's noun, not to
# a step number: gate-failure.md already uses "gate-step-5" for the RED-SUITE step, so a number
# here would name the wrong thing. Bound phrase-to-claim: "force-with-lease" alone is satisfied by
# the pre-existing CONCURRENT-push member two clauses away, which is a different reason entirely.
assert "0260: abort set names a harness/permission DENIAL of the post-rebase force-with-lease push" \
  'grep -qE -- "(denial|denying)[^.]{0,120}--force-with-lease|--force-with-lease[^.]{0,120}(denial|denying)" <<<"$gf_flat"'
assert "0260: the push-denial member is conditioned on Harness-native recovery first" \
  'grep -qE -- "--force-with-lease[^.]{0,200}Harness-native recovery" <<<"$gf_flat"'
# The distinct pre-existing member must SURVIVE alongside it — a rewrite that merges the two
# reasons into one clause is exactly the drift this pair exists to catch.
assert "0260: the concurrent-push lease rejection survives as its own distinct member" \
  'grep -qE -- "lease[^.]{0,60}concurrent push" <<<"$gf_flat"'

# Member 2 — the carve-out's posture pointer must resolve to a LISTED reason, not an implied one.
assert "0260: abort set names dispatch-unavailability for the gate agents" \
  'grep -qE -- "dispatch[^.]{0,80}unavailable|unavailable[^.]{0,80}dispatch" <<<"$gf_flat"'
assert "0260: the dispatch-unavailable member points at the carve-out" \
  'grep -qE -- "(dispatch|unavailable)[^.]{0,120}carve-out" <<<"$gf_flat"'

# De-numeralization: the count sentence must be GONE, not merely re-counted (a re-count rots again
# at the next member). Negative assert plus a non-vacuity companion through the same haystack.
assert "0260: the stale numeral is gone from the abort-reason count sentence" \
  '! grep -qiE -- "(six|seven|eight|nine) distinct abort reasons" <<<"$gf_flat"'
assert "0260: the de-numeralized sentence still makes its claim (non-vacuity)" \
  'grep -qF -- "distinct abort reasons" <<<"$gf_flat"'
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_finalize_gate.sh 2>&1 | grep -E "NOT OK|^ok - 0260"`

Expected: FAIL on the four new-member asserts and on `the stale numeral is gone` (the file says "six distinct abort reasons" today). The non-vacuity asserts and the concurrent-push survival assert pass already.

- [ ] **Step 3: Make the three edits to `gate-failure.md`**

**3a — the site marker.** Insert as a new paragraph immediately after the `On a gate-step-2 conflict, …` paragraph (the one ending `stuck or cannot reach green → abort-and-report.`), inside the `## The two agents (split at rebase-completion)` section:

```markdown
**If the dispatch itself is unavailable — the `carve-out`.** Both gate dispatches sit outside the convention's A/B/C tier table by an explicit carve-out; read its *Dispatch-capability resolution* section for the rule that decides when unavailability is established at all — resolution first, then one trivial attempt, and **never from a tool name**. A conflicted rebase whose `docket-rebase-resolver` cannot be dispatched takes that carve-out posture, abort-and-report, exactly as an ambiguous conflict does; a red rebased suite whose `docket-integration-repair` cannot be dispatched takes the same carve-out posture, exactly as a repair stuck short of green does. Neither is ever substituted inline: reconciling hunks, or authoring a repair, in the same agent that would then merge that work is the self-approval shape the convention's carve-out forbids.
```

Check the proximity requirement by eye before running anything: from the end of `` `docket-rebase-resolver` `` to `carve-out` reads `` cannot be dispatched takes that `` — no `;`, no `.`, well under 80 characters. The same holds for `` `docket-integration-repair` `` → `takes the same carve-out posture`.

**3b — the two enumeration members.** In the `## abort-and-report points (the full set)` section, extend the middle-dot list. Append both members after the existing final member (the one ending `that is a `halted`, not a retry loop).`), keeping the `·` separator style:

```markdown
 · **the dispatch mechanism being unavailable for either gate agent** (the `carve-out` above — never substituted inline) · **a harness or permission classifier denying the gate's own post-rebase `--force-with-lease` push** — named by that noun, never by a step number — which fires only *after* the convention's *Harness-native recovery* retry has been exhausted or is unavailable; like the merge denial, a denial that still stands is a `halted`, not a retry loop.
```

**3c — the de-numeralization.** In the `## The `## Finalize blocked` marker` section, change `would flatten the six distinct abort reasons into one label` to `would flatten the distinct abort reasons into one label`. Change the numeral only — leave the rest of that sentence byte-identical.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `bash tests/test_finalize_gate.sh 2>&1 | grep -E "NOT OK|^PASS|^FAIL"; echo "exit=$?"`

Expected: no `NOT OK` lines. Also run `bash tests/test_dispatch_capability.sh 2>&1 | grep -cE "NOT OK"` — expected `0`; the new gate-failure prose must not have disturbed Task 1's asserts or the negative tool-name scan (which walks `skills/`, and `gate-failure.md` is inside it).

- [ ] **Step 5: Mutation-probe the new sentinels, confirming each probe landed**

```bash
GFP=skills/docket-finalize-change/references/gate-failure.md
cp "$GFP" /tmp/gf.bak
gprobe() { # $1 = label, $2 = perl expression, $3 = a literal whose count must change
  local before after
  before="$(grep -c -- "$3" "$GFP")"
  perl -0pi -e "$2" "$GFP"
  after="$(grep -c -- "$3" "$GFP")"
  [ "$before" = "$after" ] && echo "MUTATION DID NOT LAND: $1 (count $before)"
  bash tests/test_finalize_gate.sh 2>&1 | grep -E "NOT OK" | sed "s/^/[$1] /"
  cp /tmp/gf.bak "$GFP"
}
# E: delete the push-denial member entirely
gprobe "delete-push-denial" 's/ · \*\*a harness or permission classifier denying the gate.s own post-rebase.*?retry loop\.//s' "post-rebase"
# F: keep the words, sever the CLAIM — re-point the denial at the merge (which already has its own
#    member), leaving "force-with-lease" in the file via the concurrent-push member. A bare
#    presence grep survives this; the bound assert must not.
gprobe "sever-push-binding" 's/denying the gate.s own post-rebase `--force-with-lease` push/denying the merge a second time/' "post-rebase"
# G: drop the Harness-native-recovery precondition (turns a bounded remedy into a bare halt)
gprobe "drop-recovery-condition" 's/which fires only \*after\* the convention.s \*Harness-native recovery\* retry has been exhausted or is unavailable; //' "Harness-native"
# H: delete the dispatch-unavailable member
gprobe "delete-dispatch-member" 's/ · \*\*the dispatch mechanism being unavailable for either gate agent\*\*[^·]*//' "dispatch mechanism being unavailable"
# I: re-numeralize (inversion of the de-numeralization, not a deletion)
gprobe "re-numeralize" 's/flatten the distinct abort reasons/flatten the nine distinct abort reasons/' "distinct abort reasons"
```

Expected: E reddens both push-denial asserts; **F reddens `names a harness/permission DENIAL of the post-rebase force-with-lease push` while `the concurrent-push lease rejection survives` stays green** (this is the assert's whole reason for existing — if F leaves everything green the assert is pinning vocabulary, not the claim, and must be re-bound before proceeding); G reddens the conditioned-on-recovery assert; H reddens both dispatch-unavailable asserts; I reddens `the stale numeral is gone` while the non-vacuity companion stays green. No `MUTATION DID NOT LAND`.

- [ ] **Step 6: Commit**

```bash
git add skills/docket-finalize-change/references/gate-failure.md tests/test_finalize_gate.sh
git commit -m "docs(0260): wire the carve-out at its site and name the push denial in the abort set"
```

---

### Task 3: Retire the PENDING_TIER deferral — the two sites become ordinary check_site rows

**Files:**
- Modify: `tests/test_dispatch_capability.sh`
- Verify: `tests/runtime-budgets.tsv`

**Interfaces:**
- Consumes: Task 1's convention paragraph and Task 2's `gate-failure.md` paragraph, with its anchors `conflicted rebase whose` and `red rebased suite whose` and the label literal `carve-out`.
- Produces: nothing later tasks depend on. This is the last task.

- [ ] **Step 1: Add the two check_site rows and turn the deferral off**

Four edits to `tests/test_dispatch_capability.sh`.

**1a — a file anchor.** Beside the existing `IMPL`/`AUTOGROOM`/`FIXLOOP` declarations:

```bash
GATEFAIL="$REPO/skills/docket-finalize-change/references/gate-failure.md"
```

**1b — the rows.** Immediately after the existing `check_site "$FIXLOOP" …` row:

```bash
# Change 0260: the two finalize gate dispatches, formerly parked in $PENDING_TIER. Their posture is
# the `carve-out` — not a tier letter — because their contract is an in-context report gating the
# merge. Anchored on gate-failure.md, which SKILL.md blocking-loads at BOTH dispatch moments, so
# this is the one canonical marker home and nothing is copy-pinned across two files. The anchors are
# each clause's own unique phrase, so moving a clause out of the paragraph reddens "site found"
# rather than silently re-pointing at the neighbouring paragraph that also names both agents.
check_site "$GATEFAIL" "conflicted rebase whose"  "carve-out" "finalize gate rebase-resolver"    "docket-rebase-resolver"
check_site "$GATEFAIL" "red rebased suite whose"  "carve-out" "finalize gate integration-repair" "docket-integration-repair"
```

**1c — the reach floor.** Change the assert from `-eq 6` to `-eq 8`, and update its prose from "all six dispatch sites" to "all eight dispatch sites":

```bash
assert "consumer coverage: all eight dispatch sites were reached (floor)" '[ "$seen" -eq 8 ]'
```

**1d — empty-pin `PENDING_TIER`.** Replace the value and rewrite the comment's tense — the block stays as a live guard, per spec Assumption 6:

```bash
# NOW EMPTY, AND PINNED EMPTY (change 0260 tiered the two finalize dispatches into `carve-out`
# rows above). The variable and its count assert deliberately SURVIVE the shrink: their property is
# not "these two are deferred" but "a knowingly-untiered dispatch site is an in-diff decision,
# never a silent one". Parking a genuinely new site here to quiet the coverage loop below is the
# abuse the count assert exists to make visible.
PENDING_TIER=" "
```

and the count assert:

```bash
assert "reverse: PENDING_TIER is empty — no dispatch site is knowingly untiered" \
  '[ "$(printf "%s" "$PENDING_TIER" | wc -w)" -eq 0 ]'
```

- [ ] **Step 2: Teach the coherence loop to skip a non-tier posture, behind its own floor**

The loop keys on `^\| \*\*<letter> —` rows and extracts the letter with `${t##* }`; a `carve-out` record would look for a table row named `carve-out` and fail. Skip by **shape**, never by name (spec Assumption 9). Replace the `while IFS='|' read -r t n; do` body:

```bash
tier_checked=0
while IFS='|' read -r t n; do
  [ -n "$t" ] || continue
  # SHAPE, not a name list: any posture label that is not `Tier <letter>`-shaped is outside the
  # letter-keyed table by construction, so asking the table for a row named after it is a category
  # error. Skipping by shape means a future non-tier posture needs no edit here, and — unlike an
  # `if [ "$t" = carve-out ]` exclusion — a typo'd label cannot silently exempt a real tier row.
  case "$t" in
    "Tier "[A-Z]) ;;
    *) continue ;;
  esac
  tier_checked=$((tier_checked+1))
  letter="${t##* }"
  assert "convention table: the Tier $letter row names '$n' (agrees with its wired site)" \
    "tier_row_names '$letter' '$n'"
done <<EOF
$site_rows
EOF
# Floor on the SKIP itself. Without it, broadening the case pattern (or mislabelling every row)
# skips the whole loop and the cross-file coherence property vanishes with every assert still
# green — the loop would guard nothing while reporting nothing. Six tier-shaped rows today.
assert "coherence loop: the shape filter still admitted every tier-shaped row (floor)" \
  '[ "$tier_checked" -eq 6 ]'
```

The cross-file property the skipped rows would otherwise have lost is carried by Task 1's `carveout_para` asserts (both nouns named in the convention's carve-out paragraph) plus its `tier_rows_all` negatives (neither noun in a tier row) — the same agreement, expressed for a paragraph instead of a table row.

- [ ] **Step 3: Re-derive both population floors by running the file's own greps**

Do **not** hand-edit these numbers. Run the two derivation greps verbatim from the file and count:

```bash
cd /Users/homer/dev/docket/.worktrees/tier-finalize-s-in-context-dispatches-and-name-the-push-deni
{ grep -rohE --include='*.md' '`[A-Za-z0-9_-]+`[^`]{0,20}subagent' skills/ \
    | grep -oE '`[A-Za-z0-9_-]+`' | tr -d '`'
  grep -rohE --include='*.md' 'resolved (build|review) skill' skills/ \
    | grep -oE 'build|review'
} > /tmp/derived.txt
wc -w < /tmp/derived.txt
sort /tmp/derived.txt | uniq -c | sort -rn
```

The pre-change count is **12** (`docket-status` 2, `docket-rebase-resolver` 2, `docket-auto-groom-critic` 2, `docket-adr` 2, `build` 2, `review` 1, `docket-integration-repair` 1). Set the floor to the **newly measured** count, and only after confirming from the `uniq -c` listing that **every distinct name is now a `check_site` noun** — with `PENDING_TIER` empty there is no other place for one to be covered:

```bash
assert "reverse: derivation reached the whole observed dispatch-shape population (floor: >=N)" \
  '[ "$derived_count" -ge N ]'
```

Update the `>=N` in the assert's label string too — a label that disagrees with its predicate is how the next maintainer gets misled. Leave the MAINTAINER NOTE comment above it intact.

If the count came out **lower** than 12, stop: a name was dropped from the population rather than covered, and the floor must not be lowered to accommodate it (the note above the assert says so explicitly).

- [ ] **Step 4: Run the file and verify it passes**

Run: `bash tests/test_dispatch_capability.sh 2>&1 | grep -E "NOT OK|^PASS|^FAIL"`

Expected: PASS. Confirm by eye in the full output that both `seen …/gate-failure.md carve-out` records printed — `check_site` emits one per row before any skip, so their absence means the anchors never matched.

- [ ] **Step 5: Mutation-probe the rewiring**

```bash
cp tests/test_dispatch_capability.sh /tmp/tdc.bak
cp skills/docket-finalize-change/references/gate-failure.md /tmp/gf2.bak

# J: break ONE site's clause proximity by splitting it with a period. Deletion is not enough here:
#    the interesting failure is a clause that still names both things but no longer binds them.
perl -0pi -e 's/cannot be dispatched takes that carve-out posture/cannot be dispatched. It takes that carve-out posture/' \
  skills/docket-finalize-change/references/gate-failure.md
grep -c 'cannot be dispatched\. It takes' skills/docket-finalize-change/references/gate-failure.md   # must be 1
bash tests/test_dispatch_capability.sh 2>&1 | grep "NOT OK"
# expect exactly: names carve-out next to its own noun (docket-rebase-resolver), same clause
cp /tmp/gf2.bak skills/docket-finalize-change/references/gate-failure.md

# K: remove the convention citation from the site paragraph
perl -0pi -e 's/read its \*Dispatch-capability resolution\* section/read its failure notes/' \
  skills/docket-finalize-change/references/gate-failure.md
grep -c 'read its failure notes' skills/docket-finalize-change/references/gate-failure.md          # must be 1
bash tests/test_dispatch_capability.sh 2>&1 | grep "NOT OK"   # expect both "cites the ... rule" rows
cp /tmp/gf2.bak skills/docket-finalize-change/references/gate-failure.md

# L: the reach floor must catch a site the scanner never found (rename one anchor's phrase)
perl -0pi -e 's/A conflicted rebase whose/A rebase in conflict whose/' \
  skills/docket-finalize-change/references/gate-failure.md
grep -c 'A rebase in conflict whose' skills/docket-finalize-change/references/gate-failure.md      # must be 1
bash tests/test_dispatch_capability.sh 2>&1 | grep "NOT OK"   # expect "site found" AND the eight-site floor
cp /tmp/gf2.bak skills/docket-finalize-change/references/gate-failure.md

# M: the coherence-loop floor must catch an over-broad shape filter (INVERSION probe — the skip
#    swallowing everything is the failure mode, and deleting the filter cannot produce it)
perl -0pi -e 's/    "Tier "\[A-Z\]\) ;;/    "Tier "[A-Z]) continue ;;/' tests/test_dispatch_capability.sh
grep -c '"Tier "\[A-Z\]) continue' tests/test_dispatch_capability.sh                                # must be 1
bash tests/test_dispatch_capability.sh 2>&1 | grep "NOT OK"   # expect the tier_checked floor, alone
cp /tmp/tdc.bak tests/test_dispatch_capability.sh

# N: parking a new untiered site in PENDING_TIER must be visible, not silent
perl -0pi -e 's/^PENDING_TIER=" "$/PENDING_TIER=" docket-newthing "/m' tests/test_dispatch_capability.sh
grep -c 'docket-newthing' tests/test_dispatch_capability.sh                                        # must be 1
bash tests/test_dispatch_capability.sh 2>&1 | grep "NOT OK"   # expect the PENDING_TIER empty pin
cp /tmp/tdc.bak tests/test_dispatch_capability.sh
```

Every probe must print at least one `NOT OK`, and each `grep -c` sanity line must print `1`. A probe whose count prints `0` never applied — re-write the pattern (check for a hard wrap in the target) and re-run; a never-applied mutation is indistinguishable from a guard that failed to catch it.

- [ ] **Step 6: Full suite + budget check**

```bash
scripts/run-tests.sh 2>&1 | tail -40
```

Expected: green. Read the trailing `OVER BUDGET:` line if one appears — it does not fail the run, so nothing else will catch it. `tests/test_sync_agents_runners.sh` over its ceiling is pre-existing (#0280) and is **not** this change's to fix.

Then time the two touched files directly — **never** via `scripts/run-tests.sh --timings <path>`, which truncates the named file to zero bytes (#0290):

```bash
for t in tests/test_dispatch_capability.sh tests/test_finalize_gate.sh; do
  s=$(date +%s); bash "$t" >/dev/null 2>&1; echo "$t $(( $(date +%s) - s ))s (budget 10s)"
done
```

If either exceeds its `10` second row in `tests/runtime-budgets.tsv`, raise **that row only**, to the measured number rounded up with headroom, and say so in the commit message. Both files gain only greps over files already read, so no change is expected.

- [ ] **Step 7: Commit**

```bash
git add tests/test_dispatch_capability.sh tests/runtime-budgets.tsv
git commit -m "test(0260): tier the two finalize dispatches as check_site rows; empty-pin PENDING_TIER"
```

---

## Self-Review

**Spec coverage.**

| Spec section | Task |
|---|---|
| §1 Carve-out, not a fourth tier (both nouns, the why, the posture, inline forbidden, harness-neutral) | Task 1, Step 3 |
| §2 Site-level wiring in `gate-failure.md`, one clause per agent noun, not in `SKILL.md` | Task 2, Step 3a |
| §2 "dispatch mechanism unavailable" as a named enumeration member | Task 2, Step 3b |
| §3 Push-denial member, named by noun, conditioned on *Harness-native recovery* | Task 2, Step 3b |
| §3 De-numeralize "the six distinct abort reasons" | Task 2, Step 3c |
| §4 Two ordinary `check_site` rows with the carve-out posture | Task 3, Step 1b |
| §4 Coherence loop excludes carve-out rows by shape + dedicated both-nouns convention assert | Task 3, Step 2 (exclusion) and Task 1, Step 1 (both-nouns assert) |
| §4 `PENDING_TIER` survives, empty-pinned at count 0 | Task 3, Step 1d |
| §4 `seen` floor 6 → 8; `derived_count` floor re-derived by running the greps | Task 3, Steps 1c and 3 |
| §4 `test_finalize_gate.sh` sentinels for both new members | Task 2, Step 1 |
| Assumptions 1, 2, 6, 7, 9 (design choices) | Encoded in the prose and the shape-based skip; Assumption 9's metacharacter-free literal is a Global Constraint |
| Assumption 8 (no *Composition* edit) | No task touches it; Task 1 Step 3 inserts after the table only |

No gaps.

**Placeholder scan.** No `TBD` / `TODO` / "similar to Task N" / "add appropriate error handling". Every prose insertion is given verbatim; every assert is given as runnable code; the one deliberately unresolved value — the re-derived `>=N` floor — is unresolvable before the edits land by design, and Step 3 gives the exact command that produces it plus the rule for accepting or rejecting the result.

**Type consistency.** The label literal is `carve-out` in all four places it appears (convention prose, gate-failure prose, both `check_site` rows). The shell variable is `GATEFAIL` in `test_dispatch_capability.sh` and `GF` in `test_finalize_gate.sh` — different files, no shared scope, each matching its own file's existing naming (`IMPL`/`AUTOGROOM`/`FIXLOOP` vs `FIN`/`CONV`/`STAT`). The two anchor phrases produced in Task 2 (`conflicted rebase whose`, `red rebased suite whose`) are consumed verbatim in Task 3 and appear in Task 3's probe L.
