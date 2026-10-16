<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0554 — Refresh the shipped Claude and Cursor model defaults; drop the interactive-skill model advice](../../changes/active/0554-update-cursor-review-tier-defaults-to-grok-4-7.md)**
<!-- docket:backlink:end -->

# Refresh the shipped Claude and Cursor model defaults; drop the interactive-skill model advice — Implementation Plan

> **For agentic workers:** this plan is executed by `docket-build`, task by task, each task under the `docket-build-task` contract. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship new built-in Claude and Cursor model defaults (17 rows each) in every copy of the table, and remove the hard-coded session-model recommendation from the two interactive skills in favour of short, model-agnostic guidance in the docs.

**Architecture:** Pure data and prose edit. The shipped table lives in `agents/harness-defaults.yml`, is compiled into `builtinAgentDefaults` in `internal/config/defaults.go`, and is mirrored as a commented example in `.docket.example.yml`; existing guards (`TestBuiltinAgentsParityWithShippedDefaults`, `TestExampleSchemaCorrespondence/commented_agents_table`, `TestEmbeddedMatchesAuthored`) prove the copies agree. No behavior, schema, or emitter changes.

**Tech Stack:** Go (tests, `go generate ./internal/assets/`), YAML, Markdown.

**Spec:** `docs/superpowers/specs/2026-10-16-update-cursor-review-tier-defaults-to-grok-4-7-design.md` (change #554).

## Global Constraints

- Every Cursor row keeps `effort: auto`; no Cursor ID gets an `[effort=…]` bracket.
- Model IDs are copied verbatim from the spec tables below, including the review tiers' `grok-4.7-*` IDs that have **no** `cursor-` prefix. Model IDs are opaque (ADR-0015): no allowlist, no validation added.
- Codex and opencode **values** do not change. Only one opencode **comment** sentence is removed.
- `implement-next` on Claude keeps `cache_ttl: 1h` (the `builtinAgentCacheTTL` table and the comment above that YAML row stay exactly as they are).
- `internal/assets/embedded/tree/` is regenerated with `go generate ./internal/assets/` (run from the worktree root), never hand-edited.
- Never touch frozen historical data: `internal/install/legacydata/`, `internal/install/testdata/legacy/`, any `testdata/repositories/` tree, `docs/release/`, archived specs/plans/results/changes, the corpus copies under `internal/repository/testdata/`, and `testdata/bash-upgrade/`. Never create a versioned fixture folder (#548 removed the need).
- Tests driven by their own fixed input tables (harness renderer goldens, `TestAgentDigestCoversCacheTTL`, `internal/harness/inventory_test.go`, decode/capability tests that pin `claude-opus-5` as an *input*, `TestAgentsShippedCacheTTLLayering`'s `claude-sonnet-5` global *override*) do **not** change. Only asserts on a *shipped built-in* value move.
- The illustrative override sample in `skills/docket-convention/references/agent-layer.md` (`implement-next: { model: claude-opus-5, effort: medium }` under `default:`) is not a shipped default and stays.
- ADR-0008's update note is **not** a plan task (the coordinator dispatches `docket-adr` for it). Do not edit any ADR.
- Commit messages follow the repo style `<type>(<scope>): <summary> (change 0554)`.
- Docs prose describes only current behavior, names no model ID or harness-specific command in the new session-model guidance, and cites no change/PR numbers.

### Target values — Claude (all 17 rows)

| agent | model | effort |
|---|---|---|
| adr | `claude-opus-5-5` | low |
| auto-groom | `claude-opus-5-5` | medium |
| auto-groom-critic | `claude-opus-5-5` | high |
| brainstorm-consultant | `claude-opus-5-5` | medium |
| build-economy | `claude-opus-5-5` | low |
| build-standard | `claude-opus-5-5` | medium |
| build-premium | `claude-opus-5-5` | high |
| build-max | `claude-opus-5-5` | xhigh |
| finalize-change | `claude-opus-5-5` | low |
| implement-next | `claude-opus-5-5` | low (+ `cache_ttl: 1h`) |
| integration-repair | `claude-opus-5-5` | medium |
| plan-writer | `claude-opus-5-5` | high |
| rebase-resolver | `claude-opus-5-5` | medium |
| review-lean | `claude-opus-5-5` | low |
| review-standard | `claude-opus-5-5` | medium |
| review-deep | `claude-opus-5-5` | high |
| status | `claude-haiku-5-5` | medium |

### Target values — Cursor (all 17 rows, every effort `auto`)

| agent | model |
|---|---|
| adr | `cursor-grok-4.6-high` |
| auto-groom | `cursor-grok-4.6-medium` |
| auto-groom-critic | `cursor-grok-4.6-high` |
| brainstorm-consultant | `cursor-grok-4.6-high` |
| build-economy | `cursor-grok-4.6-low` |
| build-standard | `cursor-grok-4.6-medium` |
| build-premium | `cursor-grok-4.6-high` |
| build-max | `claude-opus-5-5-high` |
| finalize-change | `cursor-grok-4.6-low` |
| implement-next | `cursor-grok-4.6-low` |
| integration-repair | `cursor-grok-4.6-high` |
| plan-writer | `claude-opus-5-5-medium` |
| rebase-resolver | `cursor-grok-4.6-high` |
| review-lean | `grok-4.7-low` |
| review-standard | `grok-4.7-medium` |
| review-deep | `grok-4.7-high` |
| status | `claude-haiku-5-5-thinking-medium` |

## Review Focus

- **Mutation probe retarget must still bite.** After Sonnet leaves the example, the probe in `internal/config/example_correspondence_test.go` must match a value that still ships and must still redden `check(mutated)`; a probe whose `strings.Replace` matches nothing hits `t.Fatalf("mutation probe did not change the example")`. Task 1 retargets it and proves both the match and the bite.
- **A copy that disagrees silently.** YAML, Go table, commented example, and embedded tree must all carry the same 34 changed rows; the parity, example-correspondence, and embedded-match tests are the proof, and each task runs them.
- **Stale comment claims.** Comments in `agents/harness-defaults.yml` that the new values make false (review-deep = build-max rule in two places, the lean-tier-departs-to-Sonnet sentence, the Sonnet-not-Haiku build-economy comment, the Cursor tiering header). No test checks prose; Task 1 and Task 2 each end with a grep that the removed claims are gone.
- **Tests that grep the removed advisory prose.** Deleting the `## Recommended model/effort (advisory)` sections must not leave a test grepping the deleted copy. A pre-plan grep of `*_test.go` and `*.sh` found none; Task 3 re-runs that grep before deleting and repoints (never restores) any hit.
- **Leftover old IDs in maintained source.** Task 4's whole-repo grep (frozen data excluded) must find no `cursor-grok-4.5`, `claude-opus-5-high`, `claude-haiku-4-5-20251001`, no shipped-default `claude-sonnet-5`/`claude-opus-5`, and no `Recommended model/effort`.

---

### Task 1: Claude block — new values in every copy, comment rewrites, probe retarget

**Files:**
- Modify: `agents/harness-defaults.yml` (the `claude:` block, its build-economy and review-tier comments, and the opencode block comment)
- Modify: `internal/config/defaults.go` (`builtinAgentDefaults["claude"]`)
- Modify: `.docket.example.yml` (the commented `#   claude:` block)
- Modify: `internal/config/example_correspondence_test.go` (mutation probe in `TestExampleSchemaCorrespondence/commented_agents_table`)
- Modify: `internal/app/config_test.go` (`TestConfigDiagnosticsAgentCacheTTLSurface`)
- Modify: `internal/config/resolve_test.go` (`TestAgentsCacheTTLResolution`, `TestAgentsRepoLayerExcludedFromEffective`)
- Regenerate: `internal/assets/embedded/tree/` via `go generate ./internal/assets/`

**Interfaces:**
- Consumes: nothing.
- Produces: the Claude block in all copies equals the Claude target table; Task 2 edits only the Cursor block of the same three files.

- [ ] **Step 1: Move the shipped-value asserts to the new Claude values (red)**

In `internal/app/config_test.go`, `TestConfigDiagnosticsAgentCacheTTLSurface`, replace the five expected strings:
- `"agents.claude.implement-next = claude-opus-5 / medium / ttl 1h  [built-in]\n"` → `"agents.claude.implement-next = claude-opus-5-5 / low / ttl 1h  [built-in]\n"`
- `"agents.claude.adr = claude-opus-5 / low  [built-in]\n"` → `"agents.claude.adr = claude-opus-5-5 / low  [built-in]\n"`
- override case: `... = claude-opus-5 / medium / ttl 5m ...` → `... = claude-opus-5-5 / low / ttl 5m ...`
- auto case: `... = claude-opus-5 / medium / ttl (no pin) ...` → `... = claude-opus-5-5 / low / ttl (no pin) ...`
- new-pin case: `"agents.claude.adr = claude-opus-5 / low / ttl 1h ..."` → `"agents.claude.adr = claude-opus-5-5 / low / ttl 1h ..."`

In `internal/config/resolve_test.go`:
- `TestAgentsCacheTTLResolution`: `pw.Model.Value != "claude-opus-5"` → `!= "claude-opus-5-5"` (plan-writer effort stays `"high"`).
- `TestAgentsRepoLayerExcludedFromEffective`: `got.Value != "claude-opus-5"` → `!= "claude-opus-5-5"`.

In `internal/config/example_correspondence_test.go`, retarget the model-value mutation probe from the Sonnet literal to the status row, which still ships after this change:

```go
mutated := strings.Replace(content, "{ model: claude-haiku-5-5,", "{ model: claude-haiku-5-4,", 1)
```

(The trailing comma keeps it from matching Cursor's `claude-haiku-5-5-thinking-medium`.) Leave the `cache_ttl` probe and the unparseable-row probe as they are.

- [ ] **Step 2: Run to verify red**

Run: `go test ./internal/config/ ./internal/app/ -run 'TestConfigDiagnosticsAgentCacheTTLSurface|TestAgentsCacheTTLResolution|TestAgentsRepoLayerExcludedFromEffective|TestExampleSchemaCorrespondence' -count=1`
Expected: FAIL. The diagnostics and resolve asserts report the old `claude-opus-5` built-in, and `commented_agents_table` stops with `mutation probe did not change the example`.

- [ ] **Step 3: Write the Claude values into all three copies**

- `internal/config/defaults.go`: set every `builtinAgentDefaults["claude"]` row to the Claude target table (`{"claude-opus-5-5", "<effort>"}`; status `{"claude-haiku-5-5", "medium"}`). Keep the existing column alignment. `builtinAgentCacheTTL` is unchanged.
- `agents/harness-defaults.yml`: same 17 values in the `claude:` block; `implement-next` stays `{ model: claude-opus-5-5, effort: low, cache_ttl: 1h }`. Keep the column alignment.
- `.docket.example.yml`: same 17 values in the commented `#   claude:` block, `implement-next` keeps `, cache_ttl: 1h`. Re-pad so `effort:` starts in one column across the Claude block (the longest model is now `claude-haiku-5-5,`), as the block aligns today.

- [ ] **Step 4: Rewrite the comments the new values make untrue**

In `agents/harness-defaults.yml`:

- Replace the build-economy comment above `build-economy:` with exactly:

```yaml
    # build-economy runs Opus at low effort rather than a smaller model deliberately: the worker
    # contract is long and strict, and its failure modes (malformed return, stray commit,
    # unverifiable COMPLETE) HALT the build instead of escalating, so contract-fumble risk lands on
    # the human. Haiku (claude-haiku-5-5) is the documented cost-aggressive user-layer override, not
    # a shipped default.
```

- Replace the review-tier comment above `review-lean:` with exactly (the review-deep = build-max rule and the lean-tier-departs-to-Sonnet sentences are removed, no replacement rule):

```yaml
    # The review tiers price REVIEW work directly rather than shifting the build ladder by one: a
    # read-and-return reviewer spends its budget reading a whole branch, not assembling one.
```

- In the opencode block comment, delete only the sentence `review-deep equals build-max so the cap tier never reviews below the strength the riskiest build work was built with.` and reflow so the paragraph reads: `As in every other block the PAIR is the role, not the model: \`high\` on Flash is not "stronger than" \`medium\` on Kimi. Efforts cap at \`high\` — reasoning-effort vocabularies are model-specific and xhigh-class tokens are not assumed portable here.` Keep `#` comment lines under 100 columns. No opencode value changes.

- [ ] **Step 5: Regenerate the embedded tree**

Run: `go generate ./internal/assets/`
Expected: exit 0; `git status --short internal/assets/embedded/tree/` lists `agents/harness-defaults.yml` and `.docket.example.yml` (plus the manifest if the generator updates one).

- [ ] **Step 6: Run to verify green, and prove the retargeted probe bites**

Run: `go test ./internal/config/ ./internal/app/ ./internal/assets/ -run 'TestBuiltinAgentsParityWithShippedDefaults|TestConfigDiagnosticsAgentCacheTTLSurface|TestAgentsCacheTTLResolution|TestAgentsRepoLayerExcludedFromEffective|TestExampleSchemaCorrespondence|TestEmbeddedMatchesAuthored' -count=1`
Expected: PASS.

Probe check: `grep -c '{ model: claude-haiku-5-5,' .docket.example.yml` prints `1`. The test passing then proves both halves: the replace matched (else `Fatalf`) and the changed model value reddened `check(mutated)` (else `Errorf`).

- [ ] **Step 7: Confirm the removed comment claims are gone**

Run: `grep -n -E 'equals (the )?build-max|deliberately departs|ships Sonnet|claude-sonnet-5|claude-haiku-4-5-20251001' agents/harness-defaults.yml .docket.example.yml internal/config/defaults.go`
Expected: no output (exit 1).

- [ ] **Step 8: Commit**

```bash
git add agents/harness-defaults.yml internal/config/defaults.go .docket.example.yml \
  internal/config/example_correspondence_test.go internal/config/resolve_test.go \
  internal/app/config_test.go internal/assets/embedded/tree
git commit -m "chore(config): ship Opus 5.5 and Haiku 5.5 as the Claude agent defaults (change 0554)"
```

---

### Task 2: Cursor block — new values in every copy, header comment, validation doc

**Files:**
- Modify: `internal/config/defaults.go` (`builtinAgentDefaults["cursor"]`)
- Modify: `agents/harness-defaults.yml` (the `cursor:` block and its header comment)
- Modify: `.docket.example.yml` (the commented `#   cursor:` block)
- Modify: `docs/reference/harness/validation.md` (the Claude-model-ID exception paragraph in the Cursor phase)
- Regenerate: `internal/assets/embedded/tree/` via `go generate ./internal/assets/`

**Interfaces:**
- Consumes: Task 1's committed Claude block (untouched here).
- Produces: the Cursor block in all copies equals the Cursor target table.

- [ ] **Step 1: Write the Cursor values into the Go table first (red)**

In `internal/config/defaults.go`, set every `builtinAgentDefaults["cursor"]` row to the Cursor target table, effort `"auto"` on every row, exactly as the existing rows write it. Keep the alignment. Leave the `// Each cursor ID is a complete built-in…` comment as is (still true).

- [ ] **Step 2: Run to verify red**

Run: `go test ./internal/config/ -run 'TestBuiltinAgentsParityWithShippedDefaults|TestExampleSchemaCorrespondence' -count=1`
Expected: FAIL — the parity test reports the built-in table differs from `agents/harness-defaults.yml`, and `commented_agents_table` reports the example differs from `builtinAgents()`.

- [ ] **Step 3: Write the Cursor values into the YAML and the example**

- `agents/harness-defaults.yml` `cursor:` block: the same 17 IDs, `effort: auto` on each, alignment kept.
- `.docket.example.yml` commented `#   cursor:` block: the same 17 IDs, `effort: auto` on each; re-pad so `effort:` starts in one column across the Cursor block (the longest model is now `claude-haiku-5-5-thinking-medium,`).

- [ ] **Step 4: Rewrite the Cursor header comment**

Replace the comment above `  cursor:` in `agents/harness-defaults.yml` with exactly:

```yaml
  # Cursor's full wrapper set. Each ID is a COMPLETE Cursor built-in whose variant is already
  # encoded, so effort is `auto`: the Cursor emitter writes the ID verbatim rather than appending
  # a second, conflicting `[effort=…]` suffix. Most rows run Grok 4.6 at the level its name
  # carries, the three review tiers run Grok 4.7, and status, plan-writer, and build-max run
  # Claude models selected through Cursor under Cursor's own IDs.
```

- [ ] **Step 5: Update the validation doc's exception list**

In `docs/reference/harness/validation.md`, replace the sentences beginning `The two deliberate exceptions are \`docket-build-max\` and` through `…not a leaked Claude Code pin.` with:

```markdown
The three deliberate exceptions are `docket-build-max` (`claude-opus-5-5-high`),
`docket-plan-writer` (`claude-opus-5-5-medium`), and `docket-status`
(`claude-haiku-5-5-thinking-medium`): those are Cursor's own names for the models, selected through
Cursor, not leaked Claude Code pins.
```

Keep the following sentence (`A **missing** \`model:\` line is also a defect.`) and the rest of the section unchanged. `docket-review-deep` is no longer named as an exception.

- [ ] **Step 6: Regenerate the embedded tree**

Run: `go generate ./internal/assets/`
Expected: exit 0.

- [ ] **Step 7: Run to verify green**

Run: `go test ./internal/config/ ./internal/assets/ -run 'TestBuiltinAgentsParityWithShippedDefaults|TestExampleSchemaCorrespondence|TestEmbeddedMatchesAuthored' -count=1`
Expected: PASS.

Then: `grep -n -E 'cursor-grok-4\.5|claude-opus-5-high|cheapest for the mechanical status' agents/harness-defaults.yml .docket.example.yml internal/config/defaults.go docs/reference/harness/validation.md`
Expected: no output (exit 1).

- [ ] **Step 8: Commit**

```bash
git add internal/config/defaults.go agents/harness-defaults.yml .docket.example.yml \
  docs/reference/harness/validation.md internal/assets/embedded/tree
git commit -m "chore(config): ship Grok 4.6/4.7 and Claude-through-Cursor as the Cursor agent defaults (change 0554)"
```

---

### Task 3: Remove the interactive skills' model recommendation (skills, convention, glossary)

**Files:**
- Modify: `skills/docket-new-change/SKILL.md` (delete the `## Recommended model/effort (advisory)` section)
- Modify: `skills/docket-groom-next/SKILL.md` (delete the `## Recommended model/effort (advisory)` section)
- Modify: `skills/docket-convention/SKILL.md` (*Agent layer* paragraph)
- Modify: `docs/reference/glossary.md` (`docket-groom-next` entry)
- Regenerate: `internal/assets/embedded/tree/` via `go generate ./internal/assets/`

**Interfaces:**
- Consumes: nothing from Tasks 1–2.
- Produces: no maintained skill or glossary text names a recommended session model; Task 4 writes the replacement guidance in `docs/install/models-and-effort.md`.

- [ ] **Step 1: Check no test greps the prose being removed**

Run: `grep -rn -I -E 'Recommended model|Recommended: .claude|advisory only|surface an \*\*advisory\*\*|model it recommends' --include='*_test.go' --include='*.sh' --include='*.go' . | grep -v -E 'legacy|internal/repository/testdata|internal/assets/embedded'`
Expected: no output. If a hit appears, repoint that assert at the text that owns the content after this change (never re-add the deleted prose) and include it in this task's commit.

- [ ] **Step 2: Delete both skill sections**

Delete the `## Recommended model/effort (advisory)` heading, its paragraph, and the blank line that follows, from `skills/docket-new-change/SKILL.md` and `skills/docket-groom-next/SKILL.md`. Put nothing in their place; the surrounding sections keep a single blank line between them.

- [ ] **Step 3: Edit the convention's *Agent layer* paragraph**

In `skills/docket-convention/SKILL.md`, replace exactly:

`the two **interactive** skills (\`docket-new-change\`, \`docket-groom-next\`) stay inline and only surface an **advisory** recommended model/effort at startup.`

with:

`the two **interactive** skills (\`docket-new-change\`, \`docket-groom-next\`) stay inline and run at the session model.`

Nothing else in that paragraph changes.

- [ ] **Step 4: Edit the glossary**

In `docs/reference/glossary.md`, `### docket-groom-next` entry, replace `It runs inline at the session model, and the model\nit recommends is advisory.` with `It runs inline at the session model.` (reflow the paragraph). The `### docket-new-change` entry carries no such clause today; leave it unchanged.

- [ ] **Step 5: Regenerate and verify**

Run: `go generate ./internal/assets/ && go test ./internal/assets/ -count=1`
Expected: PASS.

Run: `grep -rn -E 'Recommended model/effort|recommended model/effort|model it recommends|claude-sonnet-5' skills/ docs/reference/glossary.md internal/assets/embedded/tree/skills/`
Expected: no output (exit 1).

- [ ] **Step 6: Commit**

```bash
git add skills/docket-new-change/SKILL.md skills/docket-groom-next/SKILL.md \
  skills/docket-convention/SKILL.md docs/reference/glossary.md internal/assets/embedded/tree
git commit -m "docs(skills): drop the interactive skills' recommended session model (change 0554)"
```

---

### Task 4: Model-agnostic guidance in models-and-effort, and the whole-repo sweep

**Files:**
- Modify: `docs/install/models-and-effort.md` (the opening two paragraphs and the autonomous/interactive paragraph)

**Interfaces:**
- Consumes: Tasks 1–3 committed.
- Produces: the final state the full-suite gate runs against.

- [ ] **Step 1: Rewrite the intro so it names no per-skill tiers**

In `docs/install/models-and-effort.md`, keep the first paragraph (`**Why pin a model per task.**` … `matched to the **session** instead of to the task.`). Replace the second paragraph (`docket's unit of work is the skill, …` through `…and you never pick one.`) with a paragraph that keeps the point — the tier is a property of the skill, not the session; different docket agents run in the same session minutes apart at their own model and effort, and you never pick one — **without** claiming any relationship between specific skills' tiers (no "spends more effort", no "lowest effort", no "cheap tier"/"top tier" assigned to a named skill). The built-in values themselves are what `.docket.example.yml` shows; the paragraph may point there.

- [ ] **Step 2: Replace the advisory clause with session-model guidance**

The paragraph beginning `Each **autonomous** docket skill runs as a model/effort-pinned **agent**` becomes three pieces, in this order, followed by the existing `**1. Edit your global config.**` step:

1. The paragraph, ending its parenthetical at `…; the two interactive skills, \`docket-new-change\` and \`docket-groom-next\`, stay inline).` (drop `and only surface an advisory recommendation`, and drop the trailing `To change the model or effort one of them runs at:`).
2. A new paragraph with exactly this content:

```markdown
The two interactive skills, `docket-new-change` and `docket-groom-next`, run at whatever model and
effort your session uses, so you choose them. Pick by preference and by how complex you expect the
change to be: a quick stub, a trivial verdict, or a small revise needs little, while a net-new
design, a cross-cutting change, or a stub whose premise needs tracing through the code is worth
your strongest model at higher effort.
```

3. The lead-in line `To change the model or effort an autonomous skill runs at:`.

The new text names no model ID and no harness-specific command (`/model`, `/effort`).

- [ ] **Step 3: Whole-repo sweep over maintained source**

Run (capture first, then filter — no early-exit pipe):

```bash
hits=$(grep -rn -I -E 'cursor-grok-4\.5|claude-opus-5-high|claude-haiku-4-5-20251001|claude-sonnet-5|claude-opus-5[^-0-9]|claude-opus-5$|Recommended model/effort|advisory recommend' . \
  --exclude-dir=.git --exclude-dir=legacydata --exclude-dir=legacy \
  --exclude-dir=repositories --exclude-dir=bash-upgrade --exclude-dir=release \
  --exclude-dir=archive --exclude-dir=results --exclude-dir=plans --exclude-dir=specs || true)
grep -v -E 'internal/repository/testdata/|docs/adrs/' <<<"$hits" || true
```

Expected: the only remaining lines are inputs or illustrative samples the Global Constraints say stay — `internal/harness/**` tests and goldens, `internal/app/install_test.go` (`TestAgentDigestCoversCacheTTL`), `internal/app/finalize_e2e_test.go`, `internal/config/{capability,decode}_test.go`, the `claude-sonnet-5` global override in `TestAgentsShippedCacheTTLLayering`, `internal/install/legacy_test.go` (legacy data), the `agent-layer.md` override sample (source and embedded copy). Any other hit — in particular any `docs/install/`, `docs/reference/`, `skills/`, `agents/`, `.docket.example.yml`, or `internal/config/defaults.go` line — is fixed in this task.

- [ ] **Step 4: Run the full suite**

Run: `go run ./cmd/docket development test`
Expected: all tests pass. Read the budget report even when green (`BUDGET WATCH:` / `SERIAL CONFIRMED OVER BUDGET:` lines are findings to report).

- [ ] **Step 5: Commit**

```bash
git add docs/install/models-and-effort.md
git commit -m "docs(install): leave the interactive skills' session model to the human (change 0554)"
```

(Include any sweep fixes from Step 3 in this commit, staging their explicit paths.)

---

## Not in this plan

- ADR-0008's dated `## Update` note — dispatched separately to `docket-adr` by the coordinator.
- The human verification item (wrappers resolve to the new IDs on Claude Code and in Cursor's agent picker) — a post-install check no in-repo test can make.
