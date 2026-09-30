<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0473 — Rename build profile and review rung to tiers, and dispatch tiers to dispatch fallbacks](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0473-rename-build-profile-and-review-rung-to-tiers-and-dispatch-t.md)**
<!-- docket:backlink:end -->
# Tiers rename (build tier, review tier, dispatch fallbacks) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. In this repo the plan is executed by `docket-build` (one tier worker per task, sequential, one full-suite gate at the end).

**Goal:** Apply ADR-0129 family (c) (rows 46, 46a, 47, 47a, 48–52): "build profile" becomes "build tier", "review rung" becomes "review tier", and dispatch tiers A / B / C plus "the carve-out" become the dispatch fallbacks `inline` / `abstain` / `auto-or-halt` / `no-fallback`. Three skill-to-skill labels are hard-cut, and nothing else changes in meaning.

**Architecture:** This is a prose-only rename across maintained source, with no wire tokens and no Go identifiers. Tasks are split by file cluster so each file is edited by exactly one task: (1) the build family, including the `**Build tier:**` / `TIER:` / `Tier:` labels; (2) the review family; (3) the dispatch fallbacks in the convention and the workflow skills; (4) config comments; (5) docs, the concept-page rename and the glossary, followed by the closing whole-repo grep. Any task that edits a file under `skills/`, `agents/`, `cursor-rules/` or `.docket.example.yml` regenerates the embedded tree and the four harness goldens in the same commit.

**Tech Stack:** Markdown and YAML prose, Go tests (`internal/harness`, `internal/repoguard`, `internal/assets`), `go generate ./internal/assets` (`cmd/genassets`), golden refresh via each harness package's `-update` flag, and the Go suite runner (`go run ./cmd/docket development test`).

**Spec:** `docs/superpowers/specs/2026-09-30-rename-build-profile-and-review-rung-to-tiers-and-dispatch-t-design.md` on the `docket` metadata branch (synchronized copy at `/Users/homer/dev/docket/.docket/docs/superpowers/specs/2026-09-30-rename-build-profile-and-review-rung-to-tiers-and-dispatch-t-design.md`). Decision record: ADR-0129 family (c), amended in place at grooming.

**Worktree:** `/Users/homer/dev/docket/.worktrees/rename-build-profile-and-review-rung-to-tiers-and-dispatch-t` (branch `refactor/rename-build-profile-and-review-rung-to-tiers-and-dispatch-t`, base `c38ca3bed9d95682de26ca71421a948865b2d5e4`). Every command below runs from this directory.

## Global Constraints

- Meaning is unchanged everywhere: this is vocabulary, not policy. Exact sentence wording is the implementer's, within the budgets below.
- Rename map (spec §1): build profile(s) → build tier(s); profile agent / worker → tier agent / worker; review rung(s) / reviewer rung → review tier(s); profile-routed / rung-routed → tier-routed; task-to-profile → task-to-tier; "character→profile rubric" → "character→tier rubric"; cap rung → cap tier; "one rung below max" → "one tier below max".
- Skill labels (row 46a) are hard-cut, with producer and consumer changed in the same commit: `**Build profile:** <tier>` → `**Build tier:** <tier>`; `PROFILE: <tier> — <reason>` → `TIER: <tier> — <reason>`; `Profile: <tier> (…)` / `Rung: <tier> (…)` → `Tier: <tier> (…)`. Add no alias, dual spelling, or refusal text naming the old spelling (ADR-0129 Decision 2).
- Dispatch fallbacks (rows 48–52): Tier A → the `inline` fallback; Tier B → the `abstain` fallback; Tier C / Tier-C evidence / authorized-or-halt → the `auto-or-halt` fallback; the carve-out → `no-fallback`; "## Dispatch unavailability — the carve-out" → "## Dispatch unavailability — no fallback"; "the posture is tiered" → "the fallback differs by kind"; "degrades in tiers" → "falls back by kind"; "tiered unavailability" → "per-kind fallbacks".
- Collisions (row 47a, Decision 4): "severity-tiered findings" → "severity-ranked findings"; "The tiers a reviewer assigns" → "The severity levels a reviewer assigns"; "tiered by severity" → "ranked by severity"; agent-layer.md "no tier layer" → "no model-alias layer".
- The convention keeps ADR-0086's layout: a three-row fallback table, then the `no-fallback` paragraph after it. Never a fourth row.
- **Kept:** the tier names (economy / standard / premium / max, lean / standard / deep), agent names, config keys, frontmatter fields, the "ladder" metaphor, generic "tier" senses (model/effort cost tiers, validation Tier 1/2/3, the docs' "three tiers", "high-tier consultant", "tiered autonomy/response" in the playbook comparison, the learnings "tiering criterion"), "profile" in its unrelated senses (verification profile, remote-call profile, budget profile, "profilers", "shell profile"), and the shared words `inline` and `abstain`.
- **Budgets:** the skill line/word ceilings in `internal/repoguard/budgets_test.go`. **No ceiling is raised**, and `internal/repoguard/budgets_test.go` must stay byte-identical to base. Swaps are made in place, word-for-word or shorter, with no reflow that adds lines. A file that would exceed its ceiling gets tighter wording instead. Measured at base (lines/words vs ceiling): `docket-implement-next/SKILL.md` 214/214, 8175/8175; `docket-review/SKILL.md` 110/110, 913/913; `docket-auto-groom/SKILL.md` 66/70, 1627/1627; `docket-build/SKILL.md` 439/439, 4478/4479; `docket-build-task/SKILL.md` 211/211, 2231/2235; `docket-convention/references/agent-layer.md` 202/205, 2349/2350; `docket-finalize-change/SKILL.md` 238/239, 5645/5647; `docket-implement-next/references/fix-loop.md` 190/190, 1957/1958; `docket-implement-next/references/edge-paths.md` 118/118, 1550/1554; `docket-convention/SKILL.md` 390/400, 7860/7969; `docket-finalize-change/references/gate-failure.md` 145/147, 1892/1901; `docket-status/SKILL.md` 129/140, 2975/3065; `docket-build/references/task-routing.md` 48/50, 485/500. Lines are counted like `wc -l` and words like `wc -w`.
- **Regenerated, never hand-edited:** `internal/assets/embedded/**` (`go generate ./internal/assets`) and `internal/harness/{claude,codex,cursor,opencode}/testdata/golden/**` (each package's tests run with `-update`).
- **Frozen, never edited:** `docs/changes/**`, `docs/results/**`, `docs/superpowers/**` (except this plan), `docs/adrs/**` (ADR-0015, ADR-0086 and ADR-0129 included), `internal/install/legacydata/`, `internal/install/testdata/legacy/`, `testdata/repositories/`, `internal/repository/testdata/`, `docs/reference/harness/fixtures/`, and the history comments in `internal/repoguard/budgets_test.go`. ADR link *targets* keep their frozen filenames (for example `adrs/0063-docket-owns-the-build-role-profile-routed-workers.md`); only the prose around them changes.
- No new guard or retired-vocabulary seal (Decision 8). Verification is the full suite plus the closing grep in Task 5.
- Leave managed-block markers alone. `skills/docket-build/SKILL.md` has a `<!-- docket:feature-dispatch:start … -->` / `<!-- docket:feature-dispatch:end -->` block: swap words inside it, but never touch or move the marker lines.
- Stage explicit paths only (`git add -- <path> …`), never `git add -A` / `git add .`.
- Go test commands always pass `-count=1` (a cached `ok (cached)` is not evidence).
- Mutation tests restore from a backup copy (`cp f f.bak; mutate; test; mv -f f.bak f`), never `git checkout -- f`. Confirm with a count that each mutation landed before you read the test result.
- Shell: the agent's interactive shell is zsh and its `grep` is ugrep. Run multi-line shell steps under `bash -c`, use `git grep` or `command grep` for verification, and capture producer output into a variable before grepping it (AGENTS.md pipefail rule).
- The build gate runs the whole suite through `build.test_command` (`go run ./cmd/docket development test`), not only the tests named here. Read its budget report even when it is green.
- Site lists below were traced at base `c38ca3bed` and are starting points. The closing grep (Task 5) is authoritative.

## Review Focus

1. **A plan written with the new label is not read by the new docket-build.** A plan task carrying `**Build tier:** premium` should route to premium. If any consumer sentence in `skills/docket-build/SKILL.md` (`## Routing`, *Halting conditions*) or `references/task-routing.md` still names `Build profile:`, the override is silently ignored and a risky task drops to a cheaper tier. Task 1 Step 6 pins this with a positive/negative grep over both consumer files, and Task 5's closing grep checks it repo-wide.
2. **A skill file crosses its budget ceiling** because a replacement is longer than the phrase it replaces (for example "Tier B" → "the `abstain` fallback" in `docket-auto-groom/SKILL.md`, which sits at 1627/1627 words). Expected: tighter wording, never a raised ceiling. Each task that edits skills runs `TestSkillSizeBudgets` and compares its measurements against the Global Constraints table. Task 5 asserts that `budgets_test.go` is unchanged from base.
3. **Glossary and concept-page links break** after the heading and file renames: the index's *Escalation (NEEDS_ESCALATION)* link, `#review-tier`, `#dispatch-fallbacks`, and the four inbound links to `build-tiers-and-gate.md`. Expected: every link resolves. Task 5 Step 5 greps for the old anchors and path (none may remain) and checks that each new anchor has a matching heading.
4. **An unrelated "tier" or "profile" sense gets renamed**, such as "shell profile" in `validation-runbook.md`, "effort tiers" in docket-build, validation "Tier 2/3", or an ADR filename in a link target. Expected: those stay. Task 5's closing grep lists the exact permitted "profile" residue, and the task's diff review checks that no generic sense changed.
5. **Regenerated goldens or the embedded tree drift beyond the intended wording**, for example a stale embedded tree or goldens refreshed before `go generate`. Expected: every golden diff line is a rename-map swap. Each regenerating task inspects `git diff -U0` over the goldens before it commits.

---

### Task 1: Build tier and the build labels (`**Build tier:**`, `TIER:`, `Tier:`)

**Files:**
- Modify: `skills/docket-build/SKILL.md`, `skills/docket-build/references/task-routing.md`, `skills/docket-build-task/SKILL.md`
- Modify: `agents/docket-build-economy.md`, `agents/docket-build-standard.md`, `agents/docket-build-premium.md`, `agents/docket-build-max.md`
- Modify: `cursor-rules/dispatch.head.md`, `cursor-rules/dispatch/docket-build-economy.md`, `cursor-rules/dispatch/docket-build-standard.md`, `cursor-rules/dispatch/docket-build-premium.md`, `cursor-rules/dispatch/docket-build-max.md`
- Test: `internal/harness/inventory_test.go` (pin `"STANDARD profile"` → `"STANDARD tier"`)
- Regenerate: `internal/assets/embedded/**`, `internal/harness/{claude,codex,cursor,opencode}/testdata/golden/**`

**Interfaces:**
- Consumes: nothing from other tasks.
- Produces: the new label spellings that later tasks and docs quote exactly: `**Build tier:** <economy|standard|premium|max>` (plan override, read by docket-build `## Routing` and *Halting conditions*), `TIER: <economy|standard|premium|max> — <reason>` (the docket-build-task return line), `Tier: <tier> (<reason>)` (the cursor dispatch prompt label). Also the `## Tiers` heading in `skills/docket-build/SKILL.md` (was `## Profiles`).

- [ ] **Step 1: Update the pin first (failing test)**

In `internal/harness/inventory_test.go`, change the assertion inside `TestParseInventoryFromEmbedded`:

```go
	if !strings.Contains(bs.Body, "STANDARD tier") {
		t.Errorf("build-standard body does not read like the authored body: %.80q", bs.Body)
	}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test -count=1 -run TestParseInventoryFromEmbedded ./internal/harness/`
Expected: FAIL with `build-standard body does not read like the authored body`. The embedded body still says "STANDARD profile".

- [ ] **Step 3: Rename in the docket-build skill and its routing reference**

`skills/docket-build/SKILL.md`: each swap is one word for one word, and no line is added:

| Old (quoted) | New |
|---|---|
| description: "a named economy/standard/premium/max profile agent" | "… tier agent" |
| `# docket-build — profile-routed plan execution` | `# docket-build — tier-routed plan execution` |
| "route each task to a profile," | "route each task to a tier," |
| `## Profiles` | `## Tiers` |
| "A higher rung means greater reasoning investment" | "A higher tier means …" |
| "every\nprofile carries identical testing" | "every\ntier carries …" |
| "an\nunmapped harness/profile pair runs unpinned" | "… harness/tier pair …" (keep "effort tiers" in the next sentence: generic sense) |
| `**Build profile:** economy` (the fenced example) | `**Build tier:** economy` |
| "the shared character→profile rubric" | "the shared character→tier rubric" |
| "naming both the profile and its reason" | "naming both the tier and its reason" |
| "dispatch the selected profile agent **by name**" (inside the feature-dispatch block; markers untouched) | "dispatch the selected tier agent **by name**" |
| "selected\nprofile and routing reason, the completion schema" (same block) | "selected\ntier and routing reason, …" |
| "If profile dispatch is genuinely unavailable" | "If tier dispatch is genuinely unavailable" |
| "— this role is **Tier C,\nauthorized-or-halt**: only an explicitly configured" | "— this role's fallback is\n**`auto-or-halt`**: only an explicitly configured" (6 words → 5, same two lines) |
| "A profile agent **not registered on this machine** is the same authorized-or-halt condition" | "A tier agent **not registered on this machine** is the same `auto-or-halt` condition" |
| "the\nassigned profile." (NEEDS_ESCALATION rule) | "the\nassigned tier." |
| "(task, profile, SHA, command, or harness" | "(task, tier, SHA, …" |
| "- **Profile routing is un-dispatchable**" | "- **Tier routing is un-dispatchable**" |
| "- **A profile agent is not registered on this machine**" | "- **A tier agent is not registered on this machine**" |
| "- **An explicit plan `Build profile:` value is invalid**" | "- **An explicit plan `Build tier:` value is invalid**" |
| "That ladder starts one rung above the default" | "… one tier above the default" |
| "identity and status, profile and reason, escalation" | "… tier and reason …" |
| "task-to-profile selection and reason" | "task-to-tier selection and reason" |

`skills/docket-build/references/task-routing.md`:

| Old | New |
|---|---|
| `# task-routing — the character→profile rubric` | `# task-routing — the character→tier rubric` |
| "behind docket's profile-routed work" | "behind docket's tier-routed work" |
| "routes each plan task to a profile agent" | "routes each plan task to a tier agent" |
| "(a plan's `**Build profile:**` override" | "(a plan's `**Build tier:**` override" |
| "(docket-build's `**Build profile:**`;" | "(docket-build's `**Build tier:**`;" |

`skills/docket-build-task/SKILL.md`:

| Old | New |
|---|---|
| description: "Preloaded into the docket-build profile agents" | "Preloaded into the docket-build tier agents" |
| "the selected build profile, the routing reason" | "the selected build tier, the routing reason" |
| "riskier than the assigned\n  profile, with a **concrete reason**" | "… assigned\n  tier, with a **concrete reason**" |
| `PROFILE: <economy\|standard\|premium\|max> — <one-line routing reason as given to you>` | `TIER: <economy\|standard\|premium\|max> — <one-line routing reason as given to you>` |

- [ ] **Step 4: Rename in the build agents and cursor dispatch rules**

`agents/docket-build-*.md` (edit the `description:` frontmatter value in place and keep its quoting style unchanged, then edit the body):

| File | Old | New |
|---|---|---|
| economy | "Economy build-profile worker for docket-build" … "the cheapest of docket-build's four profiles." | "Economy build-tier worker for docket-build" … "the cheapest of docket-build's four tiers." |
| economy | "You were routed to the ECONOMY profile" | "You were routed to the ECONOMY tier" |
| standard | "Standard build-profile worker" … "docket-build's default profile and its uncertainty sink." | "Standard build-tier worker" … "docket-build's default tier and its uncertainty sink." |
| standard | "You were routed to the STANDARD profile" | "You were routed to the STANDARD tier" |
| premium | "Premium build-profile worker" … "the tier for named risk, one rung below max." | "Premium build-tier worker" … "the tier for named risk, one tier below max." |
| premium | "You were routed to the PREMIUM profile" | "You were routed to the PREMIUM tier" |
| max | "Max build-profile worker" … "the strongest and rarest of docket-build's four profiles." | "Max build-tier worker" … "… four tiers." |
| max | "You were routed to the MAX profile"; "There is no profile above you." | "You were routed to the MAX tier"; "There is no tier above you." |

`cursor-rules/dispatch/docket-build-{economy,standard,premium,max}.md` (three sites each):

| Old | New |
|---|---|
| "routed a plan task to the ECONOMY\nprofile." (STANDARD / PREMIUM / MAX likewise) | "… ECONOMY\ntier." |
| "profile and its routing reason, and the completion schema" | "tier and its routing reason, …" |
| `prompt: "Task 3 of <plan path>. Profile: economy (…). <task text>")` (each file's own tier and reason) | `prompt: "Task 3 of <plan path>. Tier: economy (…). <task text>")` |

`cursor-rules/dispatch.head.md`: "including all four build-profile workers," → "including all four build-tier workers,".

- [ ] **Step 5: Regenerate the embedded tree and the goldens**

```bash
bash -c '
set -euo pipefail
go generate ./internal/assets
for p in claude codex cursor opencode; do go test -count=1 ./internal/harness/$p/ -update; done
for p in claude codex cursor opencode; do go test -count=1 ./internal/harness/$p/; done
'
```

Expected: every package `ok`. Then inspect the golden and embedded diff. Every changed line must be a rename-map swap from Steps 3–4 and nothing else:

```bash
bash -c 'd=$(git diff -U0 -- internal/harness/claude/testdata/golden internal/harness/codex/testdata/golden internal/harness/cursor/testdata/golden internal/harness/opencode/testdata/golden internal/assets/embedded); printf "%s\n" "$d" | command grep -E "^[-+][^-+]" | cut -c1-200'
```

- [ ] **Step 6: Run the focused tests, the budget check, and the label check**

Run: `go test -count=1 ./internal/harness/... ./internal/assets/ && go test -count=1 -run 'TestSkillSizeBudgets|TestDispatchBlockBudget|TestInlineRoleStopScoping|TestFeatureDispatchPayloadsCarryCanonicalWorktree' ./internal/repoguard/`
Expected: PASS. `TestParseInventoryFromEmbedded` is now green.

Budget and label check (the label check is Review Focus 1):

```bash
bash -c '
for f in skills/docket-build/SKILL.md skills/docket-build-task/SKILL.md skills/docket-build/references/task-routing.md; do printf "%-50s %s %s\n" "$f" "$(wc -l <"$f" | tr -d " ")" "$(wc -w <"$f" | tr -d " ")"; done
echo "-- old label (must be empty):"; git grep -n -F -e "Build profile:" -e "PROFILE:" -e "Profile:" -- skills cursor-rules agents || true
echo "-- new label (docket-build SKILL.md >=2, task-routing.md 2, build-task 1):"
git grep -c -F -e "Build tier:" -- skills/docket-build/SKILL.md skills/docket-build/references/task-routing.md
git grep -c -F -e "TIER: <economy" -- skills/docket-build-task/SKILL.md
'
```

Expected: `docket-build/SKILL.md` ≤ 439 lines / ≤ 4479 words, `docket-build-task/SKILL.md` ≤ 211 / ≤ 2235, `task-routing.md` ≤ 50 / ≤ 500. No old-label hits. `docket-build/SKILL.md:2` or more (routing example plus the halting condition), `task-routing.md:2`, `docket-build-task/SKILL.md:1`.

- [ ] **Step 7: Mutation-check the updated pin**

The inventory test reads the **embedded** catalog, so mutate the embedded copy:

```bash
bash -c '
f=internal/assets/embedded/tree/agents/docket-build-standard.md
cp "$f" "$f.bak"
sed -i "" "s/STANDARD tier/STANDARD level/" "$f"
echo "landed: $(command grep -c "STANDARD level" "$f")"
go test -count=1 -run TestParseInventoryFromEmbedded ./internal/harness/ ; echo "exit=$?"
mv -f "$f.bak" "$f"
go test -count=1 -run TestParseInventoryFromEmbedded ./internal/harness/
'
```

Expected: `landed: 1`, then FAIL (`does not read like the authored body`) with a non-zero exit, then PASS after the restore. If `landed: 0`, the mutation did not apply: fix the sed before you trust any result.

- [ ] **Step 8: Residue check for this task's files**

```bash
bash -c 'git grep -n -I -w -i -E "rungs?|profiles?|tier [abc]|authorized-or-halt" -- skills/docket-build skills/docket-build-task agents/docket-build-*.md cursor-rules/dispatch.head.md cursor-rules/dispatch/docket-build-*.md || echo "(none)"'
```

Expected: `(none)`, or only a match in the task-routing rubric's generic tier bullets or "effort tiers" (a kept generic sense). Any "profile" or "rung" hit is residue: fix it and re-run Steps 5–6.

- [ ] **Step 9: Commit**

```bash
git add -- internal/harness/inventory_test.go skills/docket-build/SKILL.md skills/docket-build/references/task-routing.md skills/docket-build-task/SKILL.md agents/docket-build-economy.md agents/docket-build-standard.md agents/docket-build-premium.md agents/docket-build-max.md cursor-rules/dispatch.head.md cursor-rules/dispatch/docket-build-economy.md cursor-rules/dispatch/docket-build-standard.md cursor-rules/dispatch/docket-build-premium.md cursor-rules/dispatch/docket-build-max.md internal/assets/embedded internal/harness/claude/testdata/golden internal/harness/codex/testdata/golden internal/harness/cursor/testdata/golden internal/harness/opencode/testdata/golden
git commit -m "refactor(0473): build profile -> build tier; hard-cut Build tier:/TIER:/Tier: labels"
```

---

### Task 2: Review tier and severity-ranked findings

**Files:**
- Modify: `skills/docket-review/SKILL.md`
- Modify: `agents/docket-review-lean.md`, `agents/docket-review-standard.md`, `agents/docket-review-deep.md`
- Modify: `cursor-rules/dispatch/docket-review-lean.md`, `cursor-rules/dispatch/docket-review-standard.md`, `cursor-rules/dispatch/docket-review-deep.md`
- Test: `internal/repoguard/inline_role_stop_test.go` (anchor), `internal/repoguard/skill_handoff_sites_test.go` (synthetic negative-control string), `internal/harness/codex/codex_test.go` (two comments)
- Regenerate: `internal/assets/embedded/**`, the four harness goldens

**Interfaces:**
- Consumes: Task 1's `Tier: <tier> (…)` dispatch-prompt label spelling (the review cursor rules use the same label).
- Produces: the stop anchor `One shot at the dispatched tier` in `skills/docket-review/SKILL.md`, which `stopSites` in `inline_role_stop_test.go` pins.

- [ ] **Step 1: Update the anchor first (failing test)**

In `internal/repoguard/inline_role_stop_test.go`, in `stopSites`:

```go
	{"skills/docket-review/SKILL.md", "One shot at the dispatched tier", "second-person prohibitions"},
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test -count=1 -run TestInlineRoleStopScoping ./internal/repoguard/`
Expected: FAIL. The anchor is not found in `skills/docket-review/SKILL.md` (the existence floor reddens).

- [ ] **Step 3: Rename in the review skill, agents and cursor rules**

`skills/docket-review/SKILL.md` (at 110/110 lines and 913/913 words, so every swap is one word for one word):

| Old | New |
|---|---|
| description: "returns severity-tiered findings" | "returns severity-ranked findings" |
| "once, at the rung you were dispatched to." | "once, at the tier you were dispatched to." |
| "- One shot at the dispatched rung." | "- One shot at the dispatched tier." |
| "re-litigating which profile a task routed to" | "re-litigating which tier a task routed to" |

`agents/docket-review-*.md`: in each `description:`, "returns severity-tiered findings" → "returns severity-ranked findings". Bodies:

| File | Old | New |
|---|---|---|
| deep | "routed to the DEEP rung because the build reached one of its two strongest profiles" … "There is no rung above you" | "routed to the DEEP tier because the build reached one of its two strongest tiers" … "There is no tier above you" |
| lean | "routed to the LEAN rung because the build it reviews stayed on its cheapest profile throughout" | "… LEAN tier … cheapest tier throughout" |
| standard | "routed to the STANDARD rung because the build routed or escalated a task to its standard profile." … "to a stronger rung: one rung, one pass." | "… STANDARD tier … its standard tier." … "to a stronger tier: one tier, one pass." |

`cursor-rules/dispatch/docket-review-{lean,standard,deep}.md`:

| Old | New |
|---|---|
| "the DEEP reviewer rung." (LEAN / STANDARD likewise) | "the DEEP review tier." |
| `Rung: deep (build reached one of its two strongest profiles, or the diff crossed the size threshold)` | `Tier: deep (build reached one of its two strongest tiers, or the diff crossed the size threshold)` |
| `Rung: lean (build stayed on its cheapest profile)` | `Tier: lean (build stayed on its cheapest tier)` |
| `Rung: standard (build routed or escalated a task to its standard profile)` | `Tier: standard (build routed or escalated a task to its standard tier)` |

Go test strings:
- `internal/repoguard/skill_handoff_sites_test.go`: `skillFramedRe.MatchString("Dispatch the selected rung wrapper by name, foreground")` → `"Dispatch the selected tier wrapper by name, foreground"` (the negative control must still not match).
- `internal/harness/codex/codex_test.go`: comments `// profile-routed build` → `// tier-routed build`, `// rung-routed review` → `// tier-routed review`.

- [ ] **Step 4: Regenerate the embedded tree and the goldens**

```bash
bash -c '
set -euo pipefail
go generate ./internal/assets
for p in claude codex cursor opencode; do go test -count=1 ./internal/harness/$p/ -update; done
for p in claude codex cursor opencode; do go test -count=1 ./internal/harness/$p/; done
d=$(git diff -U0 -- internal/harness/claude/testdata/golden internal/harness/codex/testdata/golden internal/harness/cursor/testdata/golden internal/harness/opencode/testdata/golden internal/assets/embedded)
printf "%s\n" "$d" | command grep -E "^[-+][^-+]" | cut -c1-200
'
```

Expected: every package `ok`, and every diff line is a Step 3 swap.

- [ ] **Step 5: Run the focused tests and the budget check**

Run: `gofmt -l internal/harness/codex internal/repoguard && go vet ./internal/harness/... ./internal/repoguard/ && go test -count=1 ./internal/harness/... ./internal/assets/ && go test -count=1 -run 'TestInlineRoleStopScoping|TestSkillSizeBudgets|TestDispatchBlockBudget|TestSkillHandoffSites' ./internal/repoguard/`
Expected: `gofmt -l` prints nothing, and every test passes. `TestInlineRoleStopScoping` is now green. `skills/docket-review/SKILL.md` remains ≤ 110 lines / ≤ 913 words (`wc -l`, `wc -w`).

- [ ] **Step 6: Mutation-check the updated anchor**

```bash
bash -c '
f=skills/docket-review/SKILL.md
cp "$f" "$f.bak"
sed -i "" "s/One shot at the dispatched tier/One shot at the assigned tier/" "$f"
echo "landed: $(command grep -c "One shot at the assigned tier" "$f")"
go test -count=1 -run TestInlineRoleStopScoping ./internal/repoguard/ ; echo "exit=$?"
mv -f "$f.bak" "$f"
go test -count=1 -run TestInlineRoleStopScoping ./internal/repoguard/
'
```

Expected: `landed: 1`, then FAIL with a non-zero exit, then PASS after the restore.

- [ ] **Step 7: Residue check for this task's files**

```bash
bash -c 'git grep -n -I -w -E "rungs?|Rungs?|Rung:|profiles?|severity-tiered" -- skills/docket-review agents/docket-review-*.md cursor-rules/dispatch/docket-review-*.md internal/repoguard/inline_role_stop_test.go internal/repoguard/skill_handoff_sites_test.go internal/harness/codex/codex_test.go || echo "(none)"'
```

Expected: `(none)`.

- [ ] **Step 8: Commit**

```bash
git add -- skills/docket-review/SKILL.md agents/docket-review-lean.md agents/docket-review-standard.md agents/docket-review-deep.md cursor-rules/dispatch/docket-review-lean.md cursor-rules/dispatch/docket-review-standard.md cursor-rules/dispatch/docket-review-deep.md internal/repoguard/inline_role_stop_test.go internal/repoguard/skill_handoff_sites_test.go internal/harness/codex/codex_test.go internal/assets/embedded internal/harness/claude/testdata/golden internal/harness/codex/testdata/golden internal/harness/cursor/testdata/golden internal/harness/opencode/testdata/golden
git commit -m "refactor(0473): review rung -> review tier; severity-ranked findings"
```

---

### Task 3: Dispatch fallbacks in the convention and the workflow skills

**Build profile:** premium

This task rewrites the normative dispatch contract that every workflow reads, in files sitting at their word ceilings. That is a named risk: a paraphrase that drops a clause changes policy.

**Files:**
- Modify: `skills/docket-convention/SKILL.md`, `skills/docket-convention/references/agent-layer.md`
- Modify: `skills/docket-implement-next/SKILL.md`, `skills/docket-implement-next/references/fix-loop.md`, `skills/docket-implement-next/references/edge-paths.md`
- Modify: `skills/docket-auto-groom/SKILL.md`, `skills/docket-status/SKILL.md`
- Modify: `skills/docket-finalize-change/SKILL.md`, `skills/docket-finalize-change/references/gate-failure.md`
- Regenerate: `internal/assets/embedded/**`, the four harness goldens (run it even if the goldens do not change, to prove it)

**Interfaces:**
- Consumes: Task 1's "build tier" / "tier agent" vocabulary and Task 2's "review tier".
- Produces: the convention section vocabulary that Task 5's glossary mirrors: the column `Fallback`, rows `` **`inline` — deterministic** ``, `` **`abstain` — adversarial** ``, `` **`auto-or-halt` — discipline** ``, the paragraph lead `**Outside the table — `no-fallback` (change 0260).**`, and the finalize heading `## Dispatch unavailability — no fallback`.

TDD exception (docket-build-task three-part form): **why unsuitable:** prose-only vocabulary with no committed guard allowed (spec Decision 8), so no test can go red first. **What replaced it:** a measured budget check, a before/after word-count delta per file, and a residue grep. **Residual risk:** meaning drift in a paraphrase, which the whole-branch review reads for.

- [ ] **Step 1: Baseline measurements**

```bash
bash -c 'for f in skills/docket-convention/SKILL.md skills/docket-convention/references/agent-layer.md skills/docket-implement-next/SKILL.md skills/docket-implement-next/references/fix-loop.md skills/docket-implement-next/references/edge-paths.md skills/docket-auto-groom/SKILL.md skills/docket-status/SKILL.md skills/docket-finalize-change/SKILL.md skills/docket-finalize-change/references/gate-failure.md; do printf "%-60s %s %s\n" "$f" "$(wc -l <"$f" | tr -d " ")" "$(wc -w <"$f" | tr -d " ")"; done'
```

Expected: matches the Global Constraints table. Record it in your notes.

- [ ] **Step 2: The convention's *Dispatch-capability resolution* section**

In `skills/docket-convention/SKILL.md` (headroom: 390/400 lines, 7860/7969 words):

1. Lead-in: "so the posture is tiered:" → "so the fallback differs by kind:".
2. Table header `| Tier | Dispatch | Posture |` → `| Fallback | Dispatch | Posture |`.
3. Row labels: `| **A — deterministic** |` → ``| **`inline` — deterministic** |``; `| **B — adversarial** |` → ``| **`abstain` — adversarial** |``; `| **C — discipline** |` → ``| **`auto-or-halt` — discipline** |``. Leave the Dispatch and Posture cells unchanged, except that the C row's posture opens with ``**`auto-or-halt`.**`` instead of `**Authorized-or-halt.**`.
4. The paragraph after the table becomes (keep every clause, rename only):

   > **Outside the table — `no-fallback` (change 0260).** `docket-finalize-change`'s two finalize-gate dispatches — `docket-rebase-resolver` and `docket-integration-repair` — sit **outside** this table rather than in a row of it, because their contract is an **in-context report** gating the merge, not git state on `metadata_branch`. Neither fallback can be borrowed for them: `inline`'s first-class-equivalent path presupposes a git-state transition to reproduce, and `auto-or-halt` presupposes a `skills:` role whose resolved value could carry a human's `auto` authorization — these dispatches have neither. When dispatch is genuinely unavailable for either — established per the resolution rule above, never from a tool name — the posture is finalize's own pre-existing **abort-and-report**: the gate stops, the PR stays open, the change stays `implemented`, and the reason is recorded through the three channels `docket-finalize-change`'s failure reference owns. **Inline substitution is forbidden** for both, and that is the point of `no-fallback` rather than a table row: reconciling the conflict, or authoring the repair, inside the very agent that would then merge that work is the same self-approval shape `abstain` exists to avoid for the critic.

5. "Tier C neither replaces nor softens the *Skill layer*'s **missing-skill rule** … while a skill that was invoked and then cannot **dispatch** is Tier C." → "`auto-or-halt` neither replaces nor softens … cannot **dispatch** is `auto-or-halt`."

Elsewhere in the same file:
- The autonomous-skill wrapper paragraph: "`docket-build-task` (shared by its four profile agents), and `docket-review` (shared by its three rung wrappers)" → "(shared by its four tier agents)" / "(shared by its three tier wrappers)". Also "the `docket-build-*` profile workers, and the `docket-review-*` rung wrappers" → "the `docket-build-*` tier workers, and the `docket-review-*` tier wrappers".
- *Composition*: "unavailable dispatch is Tier C, per *Dispatch-capability resolution*" → "unavailable dispatch falls back `auto-or-halt`, per *Dispatch-capability resolution*".
- *Skill layer*: "*Dispatch-capability resolution*'s Tier-C evidence" → "*Dispatch-capability resolution*'s `auto-or-halt` evidence".

`skills/docket-convention/references/agent-layer.md`: "passed\nthrough verbatim** — no tier layer." → "… — no model-alias layer." (three words for three; the file sits at 2349/2350).

- [ ] **Step 3: docket-implement-next (SKILL.md at 214/214 lines and 8175/8175 words, so no swap may grow)**

`skills/docket-implement-next/SKILL.md` (each paragraph is one physical line, so only the word count can move):

| Old | New | Δ words |
|---|---|---|
| "**Dispatch posture (Tier C):** plan-writer dispatch unavailable" | "**Dispatch fallback (`auto-or-halt`):** plan-writer dispatch unavailable" | −1 |
| "— is **authorized-or-halt**: an explicitly resolved `SKILL_PLAN=auto`" | "— is **`auto-or-halt`**: an explicitly resolved `SKILL_PLAN=auto`" | 0 |
| "dispatch one named build-profile worker per plan task" | "dispatch one named build-tier worker per plan task" | 0 |
| "the driver adds no Docket profile dispatch on top of it" | "… no Docket tier dispatch …" | 0 |
| "— a build-profile worker, or a custom skill's own required dispatch —" | "— a build-tier worker, …" | 0 |
| "the build role is **Tier C, authorized-or-halt**: an explicitly configured `auto`" | "the build role is **`auto-or-halt`**: an explicitly configured `auto`" | −2 |
| "**select the reviewer rung** deterministically" | "**select the review tier** deterministically" | 0 |
| "take the **highest profile any task routed or escalated to**" | "take the **highest tier any task routed or escalated to**" | 0 |
| "the rung defaults to `docket-review-standard`" | "the tier defaults to …" | 0 |
| "bumps the rung one step, capped at deep" | "bumps the tier one step, capped at deep" | 0 |
| "Log the chosen rung and its reason as one line." | "Log the chosen tier …" | 0 |
| "dispatch **no** Docket reviewer rung in addition" | "dispatch **no** Docket review tier in addition" | 0 |
| "(the selected rung for the `docket-review` binding" | "(the selected tier for …" | 0 |
| "makes the review role **Tier C** on the same authorized-or-halt terms as step 5; the `docket-adr` dispatch is **Tier A**, running inline instead with its git-state contract unchanged." | "makes the review role **`auto-or-halt`** on the same terms as step 5; the `docket-adr` dispatch falls back **`inline`**, with its git-state contract unchanged." | −4 |
| "never reaches the `max` profile; every fix runs" | "never reaches the `max` tier; …" | 0 |

`skills/docket-implement-next/references/fix-loop.md` (190/190 lines and 1957/1958 words, no reflow):

| Old | New |
|---|---|
| "**Character picks the profile. Severity picks only the failure posture.**" | "**Character picks the tier. Severity picks only the failure posture.**" |
| "- **Character → profile.**" | "- **Character → tier.**" |
| "**No fix task dispatches the `max` profile, at any severity.**" | "**No fix task dispatches the `max` tier, at any severity.**" |
| "place severity touches the profile:" | "place severity touches the tier:" |
| "fix (no retry — the next rung is `max`)" | "fix (no retry — the next tier is `max`)" |
| "dispatched by profile name," | "dispatched by tier name," |
| "**If profile dispatch is unavailable**" | "**If tier dispatch is unavailable**" |
| "an unregistered profile wrapper is" | "an unregistered tier wrapper is" |
| "the fix dispatch is **Tier C**, on the same\nauthorized-or-halt terms Step 5's build role carries:" | "the fix dispatch is **`auto-or-halt`**, on the same\nterms Step 5's build role carries:" |
| "at `docket-build`'s own profiles," | "at `docket-build`'s own tiers," |
| "those sharing a profile into one task per\n  profile —" | "those sharing a tier into one task per\n  tier —" |

`skills/docket-implement-next/references/edge-paths.md`: "the rung that reviewed," → "the tier that reviewed,".

- [ ] **Step 4: auto-groom, status, finalize**

`skills/docket-auto-groom/SKILL.md` (1627/1627 words, so the net change must be ≤ 0):

| Old | New | Δ |
|---|---|---|
| "the `docket-auto-groom-critic` dispatch is **Tier B**: the groom **abstains**" | "the `docket-auto-groom-critic` dispatch's fallback is **`abstain`**: the groom **abstains**" | −1 |
| "skip it straight to Tier B." | "skip it straight to `abstain`." | −1 |
| "*Dispatch-capability resolution*: **Tier B**, so the groom **abstains**" | "*Dispatch-capability resolution*: **`abstain`**, so the groom **abstains**" | −1 |

`skills/docket-status/SKILL.md`: "— the convention's Tier A path —" → "— the convention's `inline` fallback —".

`skills/docket-finalize-change/SKILL.md` (5645/5647 words):

| Old | New | Δ |
|---|---|---|
| "(the carve-out below)" | "(the `no-fallback` posture below)" | +1 |
| `## Dispatch unavailability — the carve-out` | `## Dispatch unavailability — no fallback` | 0 |
| "sit outside the convention's dispatch-tier table by an explicit carve-out." | "sit outside the convention's dispatch-fallback table as `no-fallback`." | −2 |
| "takes the carve-out posture:" | "takes the `no-fallback` posture:" | 0 |
| "is the self-approval shape the carve-out forbids." | "is the self-approval shape `no-fallback` forbids." | −1 |

`skills/docket-finalize-change/references/gate-failure.md`: "— the carve-out below," → "— the `no-fallback` posture," (the "below" had no referent in this file).

- [ ] **Step 5: Regenerate and run the focused tests**

```bash
bash -c '
set -euo pipefail
go generate ./internal/assets
for p in claude codex cursor opencode; do go test -count=1 ./internal/harness/$p/ -update; done
go test -count=1 ./internal/assets/ ./internal/harness/...
go test -count=1 ./internal/repoguard/
git diff --stat -- internal/harness/claude/testdata/golden internal/harness/codex/testdata/golden internal/harness/cursor/testdata/golden internal/harness/opencode/testdata/golden
'
```

Expected: every test passes, including `TestSkillSizeBudgets` and the whole `internal/repoguard` package, which covers the feature-dispatch, handoff and prose-contract guards. A golden diff, if any, contains only Step 2–4 swaps.

- [ ] **Step 6: Measure again, then check residue**

Re-run the Step 1 command. Every file must be ≤ its ceiling in the Global Constraints table, and `docket-implement-next/SKILL.md` and `docket-auto-groom/SKILL.md` must be ≤ their Step 1 word counts. Then:

```bash
bash -c 'git grep -n -I -w -E "rungs?|profiles?|Tier [ABC]|Tier-C|[Aa]uthorized-or-halt|carve-out|dispatch-tier|posture is tiered|tier layer" -- skills/docket-convention skills/docket-implement-next skills/docket-auto-groom skills/docket-status skills/docket-finalize-change || echo "(none)"'
```

Expected: `(none)`.

- [ ] **Step 7: Commit**

```bash
git add -- skills/docket-convention/SKILL.md skills/docket-convention/references/agent-layer.md skills/docket-implement-next/SKILL.md skills/docket-implement-next/references/fix-loop.md skills/docket-implement-next/references/edge-paths.md skills/docket-auto-groom/SKILL.md skills/docket-status/SKILL.md skills/docket-finalize-change/SKILL.md skills/docket-finalize-change/references/gate-failure.md internal/assets/embedded internal/harness/claude/testdata/golden internal/harness/codex/testdata/golden internal/harness/cursor/testdata/golden internal/harness/opencode/testdata/golden
git commit -m "refactor(0473): dispatch tiers and the carve-out -> dispatch fallbacks inline/abstain/auto-or-halt/no-fallback"
```

---

### Task 4: Config comments (`agents/harness-defaults.yml`, `.docket.example.yml`)

**Files:**
- Modify: `agents/harness-defaults.yml` (comments only), `.docket.example.yml` (comments only)
- Regenerate: `internal/assets/embedded/**` (both files are embedded), the four harness goldens

**Interfaces:**
- Consumes: the build tier / review tier vocabulary from Tasks 1–2.
- Produces: nothing that other tasks read.

TDD exception: **why unsuitable:** comment-only YAML edits with no behavior. **What replaced it:** proof that the YAML parses identically (config and harness tests) plus a residue grep. **Residual risk:** none beyond wording.

- [ ] **Step 1: Edit the comments (keys and values untouched)**

`agents/harness-defaults.yml`:

| Old | New |
|---|---|
| "# The review rungs price REVIEW work directly" | "# The review tiers price REVIEW work directly" |
| "so the cap rung never reviews below the" (two sites) | "so the cap tier never reviews below the" |
| "# strength … The lean rung deliberately departs" | "… The lean tier deliberately departs" |
| "max build profile — expressed in Cursor's variant suffixes" | "max build tier — …" |
| "# The profile names describe the capability/cost role of a complete pair" | "# The tier names describe …" |
| "Luna/xhigh for the economy rung, Terra/medium for the standard\n  # rung, Sol/low for the premium rung, and Sol/medium for the rare max rung" | "… economy tier, … standard\n  # tier, … premium tier, … rare max tier" |
| "the economy and standard build rungs, the lean review rung)" | "the economy and standard build tiers, the lean review tier)" |

`.docket.example.yml`:

| Old | New |
|---|---|
| "the lean, profile-routed build" | "the lean, tier-routed build" |
| "Routing a fix to a model profile is by the fix's CHARACTER" | "Routing a fix to a build tier is by the fix's CHARACTER" |
| "never reaches the `max` profile;" | "never reaches the `max` tier;" |
| "minors sharing a routed profile are batched" | "minors sharing a routed tier are batched" |
| "# docket-review's rung wrappers and all preload" | "# docket-review's tier wrappers and all preload" |

- [ ] **Step 2: Regenerate and verify**

```bash
bash -c '
set -euo pipefail
go generate ./internal/assets
for p in claude codex cursor opencode; do go test -count=1 ./internal/harness/$p/ -update; done
go test -count=1 ./internal/assets/ ./internal/harness/... ./internal/config/
d=$(git diff -U0 -- agents/harness-defaults.yml .docket.example.yml)
printf "%s\n" "$d" | command grep -E "^[-+][^-+]" | command grep -v -E "^[-+][[:space:]]*#" || echo "only comment lines changed"
git diff --stat -- internal/harness/claude/testdata/golden internal/harness/codex/testdata/golden internal/harness/cursor/testdata/golden internal/harness/opencode/testdata/golden
'
```

Expected: every test passes (including `internal/config`'s example-correspondence scan), the output reads `only comment lines changed`, and the goldens show no diff (comments are not rendered).

- [ ] **Step 3: Residue check**

```bash
bash -c 'git grep -n -I -w -E "rungs?|profiles?" -- agents/harness-defaults.yml .docket.example.yml || echo "(none)"'
```

Expected: `(none)`.

- [ ] **Step 4: Commit**

```bash
git add -- agents/harness-defaults.yml .docket.example.yml internal/assets/embedded internal/harness/claude/testdata/golden internal/harness/codex/testdata/golden internal/harness/cursor/testdata/golden internal/harness/opencode/testdata/golden
git commit -m "refactor(0473): tier vocabulary in harness-defaults and example-config comments"
```

---

### Task 5: Docs, the concept-page rename, the glossary, and the closing grep

**Files:**
- Rename: `docs/concepts/build-profiles-and-gate.md` → `docs/concepts/build-tiers-and-gate.md` (`git mv`)
- Modify: `docs/reference/glossary.md`, `docs/README.md`, `docs/concepts/README.md`, `docs/concepts/skills-agents-dispatch.md`
- Modify: `docs/guide/building-without-supervision.md`, `docs/guide/reviewing-before-the-human.md`, `docs/guide/proving-the-build.md`
- Modify: `docs/install/delegating-across-harnesses.md`, `docs/reference/skills-and-agents.md`, `docs/reference/harness/validation.md`, `docs/comparison/ai-native-sdlc-playbook.md`, `scripts/runners/opencode.md`
- No regeneration: `docs/` and `scripts/runners/` are not embedded.

**Interfaces:**
- Consumes: the Task 3 convention names (`inline`, `abstain`, `auto-or-halt`, `no-fallback`), Task 1's `**Build tier:**` label, and Task 2's review-tier wording.
- Produces: the glossary anchors `#build-tier--escalation`, `#review-tier`, `#dispatch-fallbacks`, and the concept path `docs/concepts/build-tiers-and-gate.md`.

TDD exception: **why unsuitable:** docs prose, and a new guard is out of scope (Decision 8). **What replaced it:** an anchor/link check, the closing whole-repo grep, and the full suite at the build gate. **Residual risk:** wording quality, which review reads.

- [ ] **Step 1: Rename the concept page and fix its inbound links**

```bash
git mv docs/concepts/build-profiles-and-gate.md docs/concepts/build-tiers-and-gate.md
```

In `docs/concepts/build-tiers-and-gate.md`: `# Build profiles and the suite gate` → `# Build tiers and the suite gate`. "each is routed to a **build profile**: one of four worker\ntiers (economy, standard, premium, max) a plan task is routed to by risk." → "each is routed to a **build tier**: one of four workers\n(economy, standard, premium, max) chosen by risk." "routed to exactly one profile by its risk" → "routed to exactly one tier by its risk". Under *Decided in*: "as profile-routed workers" → "as tier-routed workers" and "the fix loop's profile envelope" → "the fix loop's tier envelope". **Keep both ADR link targets byte-identical** (`0063-docket-owns-the-build-role-profile-routed-workers.md`, `0070-fix-loop-profile-envelope-blocker-floor-and-max-ceiling.md`).

Inbound links, each `[Build profiles and the suite gate](…build-profiles-and-gate.md)` → `[Build tiers and the suite gate](…build-tiers-and-gate.md)`, keeping each file's relative prefix: `docs/README.md` (`concepts/`), `docs/concepts/README.md` (`./`), `docs/guide/building-without-supervision.md` (`../concepts/`), `docs/guide/reviewing-before-the-human.md` (`../concepts/`).

- [ ] **Step 2: Glossary**

In `docs/reference/glossary.md`:

1. Heading `### Build profile / escalation` → `### Build tier / escalation`. Body: "A build profile is one of four worker tiers (economy, standard, premium, max) a plan task is\nrouted to by risk." → "A build tier is one of four workers (economy, standard, premium, max) a plan task is\nrouted to by risk." "Each profile is its own agent" → "Each tier is its own agent". Keep "escalates one\ntier" and "beyond its tier" as they are.
2. The *Plan* entry's **Used for:** "routing each task to a build profile." → "… to a build tier."
3. *Finding severity*: "The tiers a reviewer assigns. They decide which findings the fix loop routes and at what profile." → "The severity levels a reviewer assigns. They decide which findings the fix loop routes and at what tier."
4. *Fix loop*: "routed\nto build profiles as fix tasks" → "routed\nto build tiers as fix tasks".
5. Heading `### Review rung` → `### Review tier`. "The rung is chosen\ndeterministically one step above the build" → "The tier is chosen\ndeterministically one step above the build".
6. Replace the whole `### Dispatch tiers (A / B / C) and the carve-out` entry with:

   ```markdown
   ### Dispatch fallbacks

   What a workflow does when dispatch is genuinely unavailable; the fallback differs by kind.
   **`inline`** (status, ADR): run the same work inline as a first-class equivalent. **`abstain`**
   (the critic): abstain. **`auto-or-halt`** (plan writer, build, review, fix workers): proceed inline
   only if the role is explicitly `auto`, otherwise halt (in Go v1 an explicit `skills.*` value blocks
   mutation, so in practice it halts). **`no-fallback`** (the finalize rebase resolver and integration
   repair): abort-and-report, never inline.
   ```

7. *docket-adr*: "It is Tier A, so without dispatch it runs inline with the same git-state contract." → "Its dispatch fallback is `inline`: without dispatch it runs inline with the same git-state contract."
8. *docket-build*: "one of\nthe four build-profile agents" → "one of\nthe four build-tier agents". "See *Build profile / escalation* and *Build gate*." → "See *Build tier / escalation* and *Build gate*."
9. *docket-build-task*: "the four `docket-build-*` profile agents" → "the four `docket-build-*` tier agents".
10. *docket-review*: "returns findings tiered by severity" → "returns findings ranked by severity". "one of the three review\nrung agents" → "one of the three review\ntier agents". "See *Review rung* and *Fix loop*." → "See *Review tier* and *Fix loop*."
11. *docket-status*: "It is Tier A, so without dispatch it runs\ninline." → "Its dispatch fallback is `inline`: without dispatch it runs\ninline."
12. Alphabetical index. The entries keep their current positions (each sorts the same):
    - `- [Build profile / escalation](#build-profile--escalation)` → `- [Build tier / escalation](#build-tier--escalation)`
    - `- [Dispatch tiers (A / B / C) and the carve-out](#dispatch-tiers-a--b--c-and-the-carve-out)` → `- [Dispatch fallbacks](#dispatch-fallbacks)`
    - `- [Escalation (NEEDS_ESCALATION)](#build-profile--escalation) — see Build profile / escalation` → `- [Escalation (NEEDS_ESCALATION)](#build-tier--escalation) — see Build tier / escalation`
    - `- [Review rung](#review-rung)` → `- [Review tier](#review-tier)`

Do not add an old→new mapping anywhere in the glossary (ADR-0129 Decision 10).

- [ ] **Step 3: Guides, install, reference, comparison, runner contract**

`docs/guide/building-without-supervision.md`: heading `## Build profiles and the one escalation` → `## Build tiers and the one escalation`. "one of four\n**build profiles** (one of four worker tiers — economy, standard, premium, max — a plan task is\nrouted to by risk)" → "one of four\n**build tiers** (economy, standard, premium, max — the worker a plan task is\nrouted to by risk)". "Each profile is a separately launched **agent**" → "Each tier is a separately launched **agent**". "with a build-profile\nline on that task" → "with a build-tier\nline on that task". "only ever one rung up:" → "only ever one tier up:". "The\ndeeper mechanism behind profile routing" → "… behind tier routing". Update the concept link text and path (Step 1).

`docs/guide/reviewing-before-the-human.md`: `## The reviewer and its rungs` → `## The reviewer and its tiers`. "three pinned rung wrappers:" → "three pinned tier wrappers:". "one shot at the rung it was dispatched at" → "one shot at the tier it was dispatched at". `## Choosing the rung` → `## Choosing the review tier`. "The rung is chosen **deterministically as one above the build**" → "The review tier is chosen **deterministically as one above the build**". "the highest **build profile** (one of four worker tiers — economy, standard, premium,\nmax — a plan task is routed to by risk)" → "the highest **build tier** (one of four workers — economy, standard, premium,\nmax — a plan task is routed to by risk)". "bumps the rung one step" → "bumps the tier one step". "Findings come back **severity-tiered**" → "Findings come back **severity-ranked**". "the profile ladder and the gate verdict" → "the tier ladder and the gate verdict". Update the concept link (Step 1). Keep "picks the model tier", "never reaches the `max` tier" and "one routed tier" as they are.

`docs/guide/proving-the-build.md`: "(a **build profile** being\n  one of four worker tiers — economy, standard, premium, max — a plan task is routed to by risk)" → "(a **build tier** being\n  one of four workers — economy, standard, premium, max — a plan task is routed to by risk)". "recording each task's profile," → "recording each task's tier,".

`docs/install/delegating-across-harnesses.md`: "A **build profile** worker — one of\nfour worker tiers (economy, standard, premium, max) a plan task is routed to by risk —" → "A **build tier** worker — one of\nfour workers (economy, standard, premium, max) a plan task is routed to by risk —". "the\n  four `build-*` profile workers" → "the\n  four `build-*` tier workers". Keep "DeepSeek-tier models".

`docs/concepts/skills-agents-dispatch.md`: "the workflow degrades in tiers instead\nof crashing" → "the workflow falls back by kind instead\nof crashing". "unavailability degrades in tiers." → "unavailability falls back by kind." In the ADR-0059 bullet: "with\n  tiered unavailability." → "with\n  per-kind fallbacks."

`docs/reference/skills-and-agents.md`: "route each plan task to a profile worker" → "… to a tier worker". "cheapest build profile:", "strongest build profile:", "build profile for consequential but correctable named risk.", "default build profile and uncertainty sink" → the same text with "build tier". "the deep rung of the whole-branch reviewer." (and lean / standard) → "the deep tier of the whole-branch reviewer."

`docs/reference/harness/validation.md` (keep "Tier 1/2/3", "one tier up" and "the next tier"): `### Phase 7 — Profile-routed build under Cursor (…)` → `### Phase 7 — Tier-routed build under Cursor (…)`. "the four build profiles among them" → "the four build tiers among them". "can run a profile-routed build" → "can run a tier-routed build". "**Explicit routing, all four profiles.** A task carrying `**Build profile:** economy`" → "**Explicit routing, all four tiers.** A task carrying `**Build tier:** economy`". "A task with no `**Build profile:**` line" → "A task with no `**Build tier:**` line". "names the profile it chose and why" → "names the tier it chose and why". "that runs is that profile's agent." → "that runs is that tier's agent."

`docs/comparison/ai-native-sdlc-playbook.md` (keep "tiered autonomy" and "tiered response", the playbook's own senses): "profile-routed build" → "tier-routed build". "one rung above default." → "one tier above default." "| Review rung chosen by rule |" → "| Review tier chosen by rule |". "from the highest build profile;" → "from the highest build tier;". "build profiles and escalation;" → "build tiers and escalation;". "Reviewer contract and rungs;" → "Reviewer contract and tiers;".

`scripts/runners/opencode.md`: "docket's four\nbuild profile workers can be delegated to cheap models while the review rungs stay native" → "docket's four\nbuild tier workers … while the review tiers stay native". "which covers the four build profiles," → "which covers the four build tiers,". "and the three review rungs —" → "and the three review tiers —".

Do **not** edit `docs/reference/harness/validation-runbook.md`. Its only matches are "shell profile", an unrelated sense. The spec's site list named this file, but tracing shows nothing there to rename.

- [ ] **Step 4: Frozen-path check and budget-file check**

```bash
bash -c '
B=c38ca3bed9d95682de26ca71421a948865b2d5e4
echo "-- frozen paths touched (must be empty):"
git diff --name-only $B..HEAD -- docs/changes docs/results docs/adrs internal/install/legacydata internal/install/testdata/legacy testdata/repositories internal/repository/testdata docs/reference/harness/fixtures
git diff --name-only -- docs/changes docs/results docs/adrs internal/install/legacydata internal/install/testdata/legacy testdata/repositories internal/repository/testdata docs/reference/harness/fixtures
echo "-- docs/superpowers touched (only this plan):"
git diff --name-only $B -- docs/superpowers
echo "-- budgets_test.go diff (must be empty):"
git diff $B -- internal/repoguard/budgets_test.go
'
```

Expected: the first two lists are empty. `docs/superpowers` shows only `docs/superpowers/plans/2026-09-30-0473-rename-build-profile-and-review-rung-to-tiers-and-dispatch-t.md`. The budgets diff is empty.

- [ ] **Step 5: Anchor and link check (Review Focus 3)**

```bash
bash -c '
echo "-- old anchors / path in maintained source (must be empty):"
git grep -n -I -E "#build-profile--escalation|#review-rung|#dispatch-tiers-a--b--c-and-the-carve-out|build-profiles-and-gate\.md" -- . ":!docs/changes" ":!docs/results" ":!docs/superpowers" ":!docs/adrs" || echo "(none)"
echo "-- new headings present (expect 3 lines):"
git grep -n -E "^### (Build tier / escalation|Review tier|Dispatch fallbacks)$" -- docs/reference/glossary.md
echo "-- new anchors referenced (expect #build-tier--escalation x2, #review-tier x1, #dispatch-fallbacks x1):"
git grep -n -o -E "#(build-tier--escalation|review-tier|dispatch-fallbacks)\)" -- docs/reference/glossary.md
echo "-- concept page links resolve:"
test -f docs/concepts/build-tiers-and-gate.md && echo "file ok"
git grep -c "build-tiers-and-gate.md" -- docs/README.md docs/concepts/README.md docs/guide/building-without-supervision.md docs/guide/reviewing-before-the-human.md
'
```

Expected: `(none)`; three heading lines; the anchor counts as stated; `file ok`; each of the four files `:1`.

- [ ] **Step 6: The closing whole-repo grep (spec *Testing*; record the command and its output verbatim)**

Run from the repo root. The embedded tree and goldens are inside the scope on purpose, as proof that regeneration ran:

```bash
bash -c '
EX=(":!docs/changes" ":!docs/results" ":!docs/superpowers" ":!docs/adrs" ":!internal/install/legacydata" ":!internal/install/testdata/legacy" ":!testdata/repositories" ":!internal/repository/testdata" ":!docs/reference/harness/fixtures")
echo "== (1) rung words (expect none)"
git grep -n -I -w -E "rungs?|Rungs?|RUNGS?" -- . "${EX[@]}" || echo "(none)"
echo "== (2) retired phrases (expect none)"
git grep -n -I -F -e "authorized-or-halt" -e "Authorized-or-halt" -e "severity-tiered" -e "tiered by severity" -e "tiers a reviewer" -e "no tier layer" -e "Tier A" -e "Tier B" -e "Tier C" -e "Tier-C" -e "dispatch-tier" -e "dispatch tiers" -e "Dispatch tiers" -e "degrades in tiers" -e "tiered unavailability" -e "posture is tiered" -e "**Build profile:**" -e "Build profile:" -e "PROFILE:" -e "Profile:" -e "Rung:" -- . "${EX[@]}" || echo "(none)"
echo "== (3) profile (expect only the permitted residue)"
git grep -n -I -i "profile" -- . "${EX[@]}" || echo "(none)"
echo "== (4) carve (expect only the permitted residue)"
git grep -n -I -i "carve" -- . "${EX[@]}" || echo "(none)"
'
```

Expected:
- (1) `(none)`. `rungate` and `runGit` are not word matches.
- (2) `(none)`.
- (3) exactly these, and nothing else:
  - `internal/app/change_attach.go` (verification profile)
  - `internal/app/maintenance_traffic_integration_test.go` (remote-call profile)
  - `internal/suiterunner/budgetstate.go` (budget profile)
  - `tests/runtime-budgets.tsv` ("profilers")
  - `docs/reference/harness/validation-runbook.md` (two "shell profile" lines)
  - `docs/concepts/build-tiers-and-gate.md` (the two frozen ADR link targets `…-profile-routed-workers.md` and `…-profile-envelope-…md`)
- (4) exactly these:
  - `internal/repoguard/capability_surface_test.go` ("carves out")
  - `skills/docket-build/references/gate-execution.md` and its embedded twin `internal/assets/embedded/tree/skills/docket-build/references/gate-execution.md` ("carved out", change 0359's probes)
  - the `internal/repoguard/budgets_test.go` history comment ("abort-set carve-out")

Any other hit is residue. Fix it in the file that owns it. If that file is under `skills/`, `agents/`, `cursor-rules/` or `.docket.example.yml`, re-run the regeneration (`go generate ./internal/assets` and the four `-update` runs) and include those paths in this task's commit. Then re-run this step. In your return's `NOTES`, copy the command block and its full output for the results file.

- [ ] **Step 7: Focused tests**

Run: `go test -count=1 ./internal/repoguard/ ./internal/assets/ ./internal/harness/... ./internal/config/`
Expected: PASS. The whole suite runs at docket-build's gate after this task.

- [ ] **Step 8: Commit**

```bash
git add -- docs/concepts/build-profiles-and-gate.md docs/concepts/build-tiers-and-gate.md docs/reference/glossary.md docs/README.md docs/concepts/README.md docs/concepts/skills-agents-dispatch.md docs/guide/building-without-supervision.md docs/guide/reviewing-before-the-human.md docs/guide/proving-the-build.md docs/install/delegating-across-harnesses.md docs/reference/skills-and-agents.md docs/reference/harness/validation.md docs/comparison/ai-native-sdlc-playbook.md scripts/runners/opencode.md
git commit -m "docs(0473): build tiers, review tiers and dispatch fallbacks in docs and glossary; rename concept page"
```

(If Step 6 required residue fixes in earlier-task files, add those paths, plus `internal/assets/embedded` and the four golden directories when regenerated, to this `git add`.)

---

## Notes for the results file (carried by the final task's NOTES)

- Closing grep command and full output (Task 5 Step 6). Mutation evidence for the two updated pins (Task 1 Step 7, Task 2 Step 6).
- Spec deviation to record: `docs/reference/harness/validation-runbook.md` needed no edit (its only matches are "shell profile"), and the closing grep's permitted "profile" residue adds that file plus the two frozen ADR link targets in `docs/concepts/build-tiers-and-gate.md`. The permitted "carve" residue adds the embedded twin of `gate-execution.md`.
- Generated agent wrappers and cursor rules are loaded at session start, so this run cannot runtime-validate the new wording. Generator and golden tests are green. **Human action (Optional):** re-run `docket install` in consumer repos and restart agent sessions so the generated agent descriptions and cursor rules show the new wording. Nothing breaks without it.
- Landing: merge only after a drain (no change in any docket repo mid-build or holding a committed-but-unbuilt plan). A pre-upgrade plan's `**Build profile:**` line is not read by the new docket-build. This plan itself uses that label: it is built by the pre-merge installed skills.
