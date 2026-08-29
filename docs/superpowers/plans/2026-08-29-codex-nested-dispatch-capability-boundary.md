<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0365 — Make nested Docket dispatch reliable for every Codex agent invocation](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0365-codex-nested-dispatch-capability-boundary.md)**
<!-- docket:backlink:end -->
# Codex Nested Dispatch Capability Boundary — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: this plan is executed by the `docket-build` role
> (profile-routed, one task per worker, single full-suite gate at the end). Steps use checkbox
> (`- [ ]`) syntax for tracking.

**Goal:** Every generated Codex agent uses direct named-agent dispatch from its active top-level
tool surface and never treats a nested orchestration inventory as evidence that dispatch is
unavailable — with the shared convention rule strengthened by shape, both user entry paths
documented, and the whole thing guarded by mutation-tested tests.

**Architecture:** One Codex-specific developer-instruction paragraph is emitted by the Codex
renderer (`internal/harness/codex/codex.go`, `renderAgent`) into every generated `docket-*.toml`
wrapper — shared agent bodies in `agents/docket-*.md` stay harness-neutral, and non-Codex harness
output stays byte-unchanged. The convention's canonical dispatch-capability section
(`skills/docket-convention/SKILL.md`) gains the active/top-level-surface rule stated by shape (no
vendor tool name), mirrored in lockstep into the embedded asset tree via `go generate
./internal/assets`. Docs teach the two supported invocation paths and the fresh-process
requirement; the validation runbook gains a live two-path certification phase.

**Tech Stack:** Go (renderer + golden tests, `-update` regen, `-count=1` cache defeat), bash guard
suite (`tests/*.sh`, run by `scripts/run-tests.sh`), embedded-asset generator (`cmd/genassets` via
`go generate ./internal/assets`).

**Spec:** `docs/superpowers/specs/2026-08-29-codex-nested-dispatch-capability-boundary-design.md`
(on the `docket` metadata branch; synchronized copy at
`.docket/docs/superpowers/specs/2026-08-29-codex-nested-dispatch-capability-boundary-design.md`
from the primary tree).

## Global Constraints

- Change id 0365, branch `fix/codex-nested-dispatch-capability-boundary`, type fix, priority
  critical.
- The new paragraph is emitted in the **Codex renderer only**. `agents/docket-*.md` bodies are not
  edited. Claude, Cursor, and OpenCode goldens
  (`internal/harness/{claude,cursor,opencode}/testdata/golden/`, 17 `docket-*.md` each) must stay
  **byte-unchanged** for the whole branch.
- The convention rule is strengthened **by shape** — no vendor tool name (no "Codex", no
  "ALL_TOOLS", no "JavaScript") appears in the convention text. A tool name is a diagnostic, never
  a decision input (existing rule; keep it true of the new sentences too).
- Existing tier postures (A/B/C) and the finalize carve-out are **preserved verbatim** — this
  change adds to the resolution rule; it does not touch the tier table or carve-out paragraph.
- The embedded mirror `internal/assets/embedded/tree/skills/docket-convention/SKILL.md` is
  regenerated with `go generate ./internal/assets` — **never hand-edited**.
  `TestEmbeddedMatchesAuthored` (in `internal/assets/embedded_test.go`) enforces lockstep.
- Every Go run whose purpose is to observe an outcome uses `-count=1` (Go's test cache serves
  stale verdicts otherwise — learnings: cached-runner-serves-a-mutated-tree).
- Every mutation probe runs **after the task's commit**, restores with
  `git checkout -- <file>`, and proves the mutation landed with a `grep -c` before/after taken
  through a whitespace-flattened copy (learnings: mutation-restore-needs-a-backup-copy,
  phrase-grep-over-wrapped-prose).
- New shell asserts use **one bounded gap per ERE pattern**, never two stacked gaps (stacked gaps
  hang instead of reddening on exactly the mutated input — learnings:
  stacked-gap-regex-hangs-instead-of-failing). Phrase asserts over hard-wrapped markdown match a
  whitespace-collapsed haystack.
- No backtick inside double quotes in test source (`scripts/check-test-source-hygiene.sh`; use the
  `BT` variable pattern already in `tests/test_codex_runbook.sh`).
- Out of scope — do not build: runner/subprocess fallbacks; model/effort pin, topology, return
  protocol, or tier changes; background dispatch or notification waits; implicit inline `auto`
  recovery; skill wrappers for agent-only workers; change 0359's run-gate work. **No new ADR**
  (ADR-0036/0059/0060/0094 are cited and retained, not superseded).
- Suite gate: `scripts/run-tests.sh` plus `go test -race -count=1 ./...` per
  `finalize.test_command` (read it there). Any `SERIAL CONFIRMED OVER BUDGET:` line is a failure
  to address; `BUDGET WATCH:` is screening only.
- The diagnostic prototype is already absent from `main` — start from the clean base, failing
  tests first.

---

### Task 1: Codex renderer emits the nested-dispatch boundary paragraph

**Files:**
- Modify: `internal/harness/codex/codex.go` (`renderAgent`; new package const)
- Test: `internal/harness/codex/codex_test.go` (new `TestCodexNestedDispatchBoundary`)
- Regenerate: `internal/harness/codex/testdata/golden/docket-*.toml` (all 17, via `-update`)

**Interfaces:**
- Consumes: `harness.ParseInventory`, `harness.RecursionGuard`, the existing `fixtureInput`/
  `planFixture` helpers in `codex_test.go`.
- Produces: package const `codexDispatchBoundary` (unexported string), emitted as the **second**
  paragraph of every `developer_instructions` value (after the shared recursion guard, before the
  skills preamble/body). Later tasks rely on the emitted clauses being greppable in the goldens.

- [ ] **Step 1: Write the failing test**

Append to `internal/harness/codex/codex_test.go`:

```go
// TestCodexNestedDispatchBoundary — change 0365: every rendered Codex agent
// must carry the nested-dispatch boundary, because composition is a property
// of the invoked skill and may change through configuration; an allowlist of
// today's dispatching wrappers would silently miss a future binding. The
// expected clauses are LITERAL BEHAVIORAL EXPECTATIONS derived independently
// from the spec — deliberately NOT the renderer's own constant, so deleting
// the renderer paragraph reddens this test instead of both sides drifting
// together.
func TestCodexNestedDispatchBoundary(t *testing.T) {
	in := fixtureInput(t)
	sources, err := harness.ParseInventory(in.Assets)
	if err != nil {
		t.Fatalf("ParseInventory: %v", err)
	}
	if len(sources) == 0 {
		t.Fatal("inventory is empty — universality would be vacuous")
	}

	// The spec's five composition families must be present in the inventory,
	// so "every source carries the paragraph" cannot be satisfied by a
	// shrunken agent set. This is a sampling floor, not the coverage boundary
	// — the loop below runs over ALL sources.
	byName := map[string]bool{}
	for _, s := range sources {
		byName[s.Name] = true
	}
	families := []string{
		"docket-implement-next",     // implement-next composition
		"docket-plan-writer",        // the pinned Step 4 dispatch that failed live
		"docket-build-standard",     // profile-routed build
		"docket-review-standard",    // rung-routed review
		"docket-auto-groom-critic",  // auto-groom critic
		"docket-rebase-resolver",    // finalize resolver
		"docket-integration-repair", // finalize repair
	}
	for _, want := range families {
		if !byName[want] {
			t.Errorf("composition-family agent %s missing from inventory", want)
		}
	}

	byPath := map[string][]byte{}
	for _, tg := range planFixture(t) {
		if tg.Kind == install.KindFile {
			byPath[tg.Path] = tg.Content
		}
	}

	// The three semantic clauses, as literal behavioral text.
	clauses := []string{
		"direct named-agent dispatch",           // (1) how to dispatch
		"active top-level tool surface",         // (1) from where
		"omit top-level collaboration controls", // (2) what nested inventories lack
		"cannot establish dispatch unavailability", // (3) what absence proves: nothing
	}
	for _, s := range sources {
		p := filepath.Join(fakeHome, ".codex", "agents", s.Name+".toml")
		content, ok := byPath[p]
		if !ok {
			t.Fatalf("no rendered agent file at %s", p)
		}
		for _, c := range clauses {
			if !strings.Contains(string(content), c) {
				t.Errorf("agent %s: rendered wrapper missing nested-dispatch clause %q", s.Name, c)
			}
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/harness/codex/ -run TestCodexNestedDispatchBoundary -count=1 -v`
Expected: FAIL — every agent missing all four clauses.

- [ ] **Step 3: Implement the renderer paragraph**

In `internal/harness/codex/codex.go`, add below the `skillsPreambleFormat` const (inside the same
`const (...)` block or as its own const — its own block reads better given the comment length):

```go
// codexDispatchBoundary is the Codex-specific nested-dispatch boundary
// (change 0365). It is emitted into EVERY generated agent, unconditionally:
// composition is a property of the invoked skill and may change through
// configuration, so an allowlist of today's dispatching wrappers would
// silently miss a future custom binding, while a leaf agent receiving a
// conditional instruction incurs no behavioral change. It is Codex-specific
// tool placement (ADR-0060), so it lives in this renderer — the shared agent
// bodies stay harness-neutral. It closes the false-negative shape a live run
// hit: a parent inspected a nested JavaScript tool inventory, found no
// dispatch entry there, and halted without ever attempting the registered
// dispatch (see the convention's "Dispatch-capability resolution" section for
// the harness-neutral rule this instantiates).
const codexDispatchBoundary = "When your active charter requires another agent, dispatch it with Codex's direct named-agent dispatch from your active top-level tool surface. Nested orchestration inventories — tool lists read from inside another tool — omit top-level collaboration controls, so absence from such a nested inventory cannot establish dispatch unavailability; only a failed direct dispatch attempt or an explicit policy denial does."
```

Then in `renderAgent`, change the recursion-guard prepend line:

```go
	dev = harness.RecursionGuard(s.Name) + "\n\n" + dev
```

to:

```go
	// The dispatch boundary is the SECOND paragraph: the recursion guard keeps
	// its cross-harness first-paragraph position, and the boundary sits ahead
	// of the skills preamble and body so it reads as harness contract, not
	// role prose.
	dev = harness.RecursionGuard(s.Name) + "\n\n" + codexDispatchBoundary + "\n\n" + dev
```

(The paragraph contains no `\` and no `"""`, so `escapeMultiline` passes it through unchanged.)

- [ ] **Step 4: Run the focused test to verify it passes**

Run: `go test ./internal/harness/codex/ -run TestCodexNestedDispatchBoundary -count=1 -v`
Expected: PASS.

- [ ] **Step 5: Regenerate the Codex goldens (and only those)**

Run: `go test ./internal/harness/codex/ -run TestCodexGoldenAgents -update -count=1`
Then: `go test ./internal/harness/codex/ -count=1`
Expected: PASS; `git status --porcelain` shows modified files ONLY under
`internal/harness/codex/` (17 golden `.toml` files + `codex.go` + `codex_test.go`).

- [ ] **Step 6: Verify non-Codex output is untouched**

Run: `go test ./internal/harness/... -count=1` and
`git diff --no-renames --name-only -- internal/harness/claude internal/harness/cursor internal/harness/opencode`
Expected: all harness tests PASS; the diff command prints nothing. (The per-harness golden tests
are the standing byte-unchanged check — they compare rendered output to frozen goldens on every
suite run; this step proves this branch triggers none of them.)

- [ ] **Step 7: Commit**

```bash
git add internal/harness/codex/codex.go internal/harness/codex/codex_test.go internal/harness/codex/testdata/golden/
git commit -m "fix(0365): Codex renderer emits nested-dispatch boundary in every wrapper"
```

- [ ] **Step 8: Mutation-test the guard (post-commit)**

In `codex.go`, revert only the prepend line to its pre-change form (delete
`codexDispatchBoundary + "\n\n" + ` from the `dev = ...` line; leave the const so it compiles).
Run: `go test ./internal/harness/codex/ -run 'TestCodexNestedDispatchBoundary|TestCodexGoldenAgents' -count=1`
Expected: **both** FAIL (the focused test on missing clauses; the golden test on byte mismatch).
Restore: `git checkout -- internal/harness/codex/codex.go`, re-run the same command, expect PASS.
If the focused test stays green under the mutation, that is a defect in the test — stop and fix
it before proceeding (learnings: assert-detects-removal-not-replacement).

---

### Task 2: Convention names the authoritative surface by shape; embedded mirror in lockstep

**Files:**
- Modify: `skills/docket-convention/SKILL.md` ("### Dispatch-capability resolution (change 0137)"
  section, first paragraph only)
- Regenerate: `internal/assets/embedded/tree/skills/docket-convention/SKILL.md` and
  `internal/assets/embedded/manifest.json` (via `go generate ./internal/assets` — never by hand)
- Test: `tests/test_dispatch_capability.sh` (new asserts)
- Possibly modify: `tests/test_skill_size_budgets.sh` (budget ratchet, only if the word/line
  budget reddens)

**Interfaces:**
- Consumes: the existing `assert()` helper and `CONV` variable in
  `tests/test_dispatch_capability.sh`.
- Produces: three new convention sentences (exact text in Step 3) that Task 4's runbook prose and
  the Task 1 renderer paragraph both instantiate; a flattened haystack variable `conv_flat` other
  asserts in this file may reuse.

- [ ] **Step 1: Write the failing asserts**

In `tests/test_dispatch_capability.sh`, directly after the existing assert
`"convention: a tool name is a diagnostic, never a decision input"`, add:

```bash
# --- the authoritative surface, by shape (change 0365) -------------------------------------------
# The live failure this closes: a parent read a NESTED tool inventory (a tool list exposed from
# inside another tool), found no dispatch entry, and halted without attempting the registered
# dispatch. The rule is stated by SHAPE — no vendor tool name — and each phrase is bound to its
# claim with ONE bounded gap over a whitespace-collapsed haystack (learnings:
# prose-guard-binds-phrase-to-claim, phrase-grep-over-wrapped-prose,
# stacked-gap-regex-hangs-instead-of-failing).
conv_flat="$(tr '\n' ' ' < "$CONV" | tr -s '[:space:]' ' ')"
assert "convention 0365: resolution reads the agent's own active, top-level tool surface" \
  'grep -qE "resolving from the agent.s [*][*]own active, top-level tool surface" <<<"$conv_flat"'
assert "convention 0365: a nested namespace/inventory is explicitly non-authoritative" \
  'grep -qE "nested namespace or inventory[^.]{0,120}non-authoritative" <<<"$conv_flat"'
assert "convention 0365: nested absence establishes nothing" \
  'grep -qE "cannot be invoked from within that tool[^.]{0,80}absence there establishes nothing" <<<"$conv_flat"'
assert "convention 0365: nested inventory / absent spelling / tool-search miss never satisfy the rule" \
  'grep -qE "[Nn]either does inspecting a nested inventory[^.]{0,80}irrelevant tool-search result" <<<"$conv_flat"'
```

- [ ] **Step 2: Run to verify the new asserts fail**

Run: `bash tests/test_dispatch_capability.sh`
Expected: exactly the four new `convention 0365:` asserts print `NOT OK`; everything else `ok`.

- [ ] **Step 3: Edit the convention section**

In `skills/docket-convention/SKILL.md`, first paragraph of "### Dispatch-capability resolution
(change 0137)". Two edits:

Edit A — extend clause (1). Replace:

```
since a partially-loaded tool set makes absence observable without anything having been resolved; and (2)
```

with:

```
since a partially-loaded tool set makes absence observable without anything having been resolved, and resolving from the agent's **own active, top-level tool surface** — a **nested namespace or inventory** exposed from inside another tool is **non-authoritative**, because it may omit controls that cannot be invoked from within that tool, so absence there establishes nothing; and (2)
```

Edit B — extend the never-sufficient list. Replace:

```
such a name is an observed internal, not an interface.
```

with:

```
such a name is an observed internal, not an interface. Neither does inspecting a nested inventory, an absent spelling, or an irrelevant tool-search result — if direct dispatch is present on the active surface, call it; only that call's failure (or an explicit policy denial) is evidence.
```

Do not touch the tier table, the carve-out paragraph, or the missing-skill paragraph.

- [ ] **Step 4: Run the guard, then regenerate the embedded mirror**

Run: `bash tests/test_dispatch_capability.sh` — expected: all `ok` (new and old).
Run: `go generate ./internal/assets` then `go test ./internal/assets/ -count=1`
Expected: PASS (`TestEmbeddedMatchesAuthored` proves the mirror is in lockstep).
`git status --porcelain` must show `internal/assets/embedded/tree/skills/docket-convention/SKILL.md`
and `internal/assets/embedded/manifest.json` regenerated — hand-editing the mirror is forbidden.

- [ ] **Step 5: Sibling guards and budgets**

Run: `bash tests/test_docket_build.sh && bash tests/test_docket_review.sh && bash tests/test_skill_size_budgets.sh`
Expected: all green. If `test_skill_size_budgets.sh` reddens on
`skills/docket-convention/SKILL.md` (current budget 400 lines / 7250 words), raise the budget
following that file's own documented ratchet pattern — a dated comment naming change 0365, the
old/new figures, and what the added sentences carry (the nested-inventory surface rule), with the
line budget at the next multiple of 5 above the actual count and the word budget at the next
round step the file's history uses. Do not raise either figure beyond what the actual new counts
require.

- [ ] **Step 6: Commit**

```bash
git add skills/docket-convention/SKILL.md internal/assets/embedded/tree/skills/docket-convention/SKILL.md internal/assets/embedded/manifest.json tests/test_dispatch_capability.sh
git add tests/test_skill_size_budgets.sh   # only if Step 5 ratcheted it
git commit -m "fix(0365): convention resolves dispatch from the active top-level surface, by shape"
```

- [ ] **Step 7: Mutation-test BOTH sides (post-commit)**

Probe A — the new side must be load-bearing. Delete Edit A's inserted clause (the text from
`, and resolving from the agent's` through `absence there establishes nothing`) from
`skills/docket-convention/SKILL.md`. Prove the mutation landed:
`tr -s '[:space:]' ' ' < skills/docket-convention/SKILL.md | grep -c "own active, top-level tool surface"`
must go 1 → 0. Run `bash tests/test_dispatch_capability.sh`: the first three `convention 0365:`
asserts must print `NOT OK`. Restore: `git checkout -- skills/docket-convention/SKILL.md`; re-run;
all `ok`.

Probe B — the old side must remain sufficient/load-bearing. Delete the sentence
`Only a **failed attempt** or an explicit **policy denial** establishes unavailability.` Prove it
landed (`grep -c "policy denial.{0,5} establishes"` via the flattened copy, 1 → 0 — adjust the
count command to whatever uniquely matches). Run the guard: the existing assert
`"convention: only a failed attempt or a policy denial establishes unavailability"` must print
`NOT OK` — a nested inventory must not have quietly become sufficient evidence by the old
sentence going unguarded. Restore with `git checkout --`, re-run, all `ok`.
If either probe stays green, the guard is decoration — fix the assert before proceeding.

---

### Task 3: Setup doc and README teach the two invocation paths and the fresh-process rule

**Files:**
- Modify: `docs/codex/setup.md`
- Modify: `README.md` (one sentence in the per-repo agents paragraph that points at
  `docs/codex/setup.md`)

**Interfaces:**
- Consumes: nothing from earlier tasks (prose only).
- Produces: a `## Two invocation paths — one contract` section in `docs/codex/setup.md` whose
  heading Task 4's guard asserts; the strengthened *Restart after (re)generating* sentence Task
  4's guard also asserts.

- [ ] **Step 1: Add the invocation-paths section to `docs/codex/setup.md`**

Insert a new section after `## Verifying it works` and before `## Restart after (re)generating`:

```markdown
## Two invocation paths — one contract

Docket supports exactly two ways to start its work under Codex, and both are first-class — neither
is a workaround, and neither requires flipping a workflow's `skills:` binding to `auto`:

1. **Prose, routed by the dispatch block.** A plain request ("refresh the docket board") is routed
   by the repo's managed `AGENTS.md` dispatch block to the registered same-name `docket-*` agent.
2. **Direct invocation.** `@docket-status` (or any `@docket-…` agent) starts that same registered
   wrapper explicitly.

Either way, the wrapper you land in may need to dispatch further docket agents — planning, build,
review, grooming's critic, finalize's resolver and repair. Every generated Codex wrapper carries
the rule for that: **nested dispatch uses Codex's direct named-agent dispatch from the active
top-level tool surface.** A tool inventory read from *inside* another tool (a nested orchestration
namespace) intentionally omits Codex's top-level collaboration controls, so an agent must never
conclude from such an inventory that dispatch is unavailable — only a failed direct attempt or an
explicit policy denial establishes that. The harness-neutral statement of this rule lives in the
docket-convention skill's *Dispatch-capability resolution* section.
```

- [ ] **Step 2: Strengthen the restart section**

In the `## Restart after (re)generating` section of `docs/codex/setup.md`, make it state
explicitly (keeping whatever is already there that says Codex registers agents at process start):

```markdown
Codex registers agent definitions **once, at process start**. After any install or sync that
changed a wrapper or the dispatch block, start a **fresh Codex application/CLI process** before
relying on the new definitions. Opening another conversation inside an already-running process is
**not sufficient** — that process is still holding the definitions it loaded at start.
```

- [ ] **Step 3: README — only where it distinguishes the paths**

Run `grep -n "docs/codex/setup.md" README.md` to find the per-repo agents paragraph (the sentence
beginning `For **Codex** — its ` + backtick + `.codex/agents/*.toml` + backtick + ` wrappers plus the committed ` +
backtick + `AGENTS.md` + backtick + ` dispatch block`). Extend that sentence's pointer clause so it reads
(keeping the existing links intact):

```markdown
… see [docs/codex/setup.md](docs/codex/setup.md) — both entry paths, a prose request routed by the
committed `AGENTS.md` dispatch block and a direct `@docket-…` invocation, are supported and reach
the identical registered wrapper; …
```

Check for other Codex invocation-path claims with `grep -n "@docket" README.md` — edit only lines
that distinguish or imply one of the two paths for Codex; do not duplicate the runbook or setup
content anywhere else.

- [ ] **Step 4: Run the doc-adjacent guards**

Run: `bash tests/test_codex_runbook.sh` (it asserts `docs/codex/setup.md` is cited and cited
paths exist) and `bash scripts/run-tests.sh tests/test_readme_agents.sh` if such a README guard
exists (`ls tests/ | grep -i readme` — run whatever matches). Expected: green.

- [ ] **Step 5: Commit**

```bash
git add docs/codex/setup.md README.md
git commit -m "docs(0365): Codex two invocation paths, top-level-surface rule, fresh-process requirement"
```

---

### Task 4: Runbook Phase 7 — live two-path nested-dispatch certification (guard first)

**Files:**
- Test: `tests/test_codex_runbook.sh` (extend: 7 phases; new content asserts)
- Modify: `docs/codex/validation-runbook.md` (new `## Phase 7 — …` + pass criterion; update any
  "six phases" framing in its intro and `## Pass criteria`)

**Interfaces:**
- Consumes: Task 3's `docs/codex/setup.md` section heading (cited from the new phase).
- Produces: the Phase 7 heading `## Phase 7 — Nested dispatch from both entry paths` and its
  `**Pass when:**` stamp, which the guard counts.

- [ ] **Step 1: Extend the guard (failing first)**

In `tests/test_codex_runbook.sh`:

a. Change the phase-count derivation `grep -cE '^## Phase [1-6] — '` to `'^## Phase [1-7] — '`
and the expected count `"6"` to `"7"` (message text too).

b. Change the ordinal loop `for n in 1 2 3 4 5 6; do` to `for n in 1 2 3 4 5 6 7; do` and its
`ok` message to `"each phase 1..7 appears exactly once"`.

c. Append a new assertion block (before the final exit/summary lines; match the file's `ok`/`no`
helper style; note the whitespace-collapsed haystack and one bounded gap per pattern):

```bash
# --- Assertion 8: Phase 7 certifies nested dispatch on BOTH entry paths (change 0365) ----------
# Hard-wrapped prose: match a whitespace-collapsed haystack; ONE bounded gap per pattern
# (learnings: phrase-grep-over-wrapped-prose, stacked-gap-regex-hangs-instead-of-failing).
RB_FLAT="$(tr '\n' ' ' < "$RUNBOOK" | tr -s '[:space:]' ' ')"
if grep -qE "prose request[^.]{0,120}dispatch block" <<<"$RB_FLAT"; then ok "phase 7: prose entry path routed by the dispatch block"; else no "phase 7: prose entry path routed by the dispatch block"; fi
if grep -qF -- "@docket-" "$RUNBOOK"; then ok "phase 7: direct @docket- entry path present"; else no "phase 7: direct @docket- entry path present"; fi
if grep -qE "fresh Codex[^.]{0,80}process" <<<"$RB_FLAT"; then ok "phase 7: fresh-process requirement stated"; else no "phase 7: fresh-process requirement stated"; fi
if grep -qE "counts only when[^.]{0,120}return is consumed" <<<"$RB_FLAT"; then ok "phase 7: dispatch evidence rule (child started, return consumed)"; else no "phase 7: dispatch evidence rule (child started, return consumed)"; fi
if grep -qE "unavailable[^.]{0,140}direct rejection or an explicit policy denial" <<<"$RB_FLAT"; then ok "phase 7: unavailable verdict needs rejection/denial in transcript"; else no "phase 7: unavailable verdict needs rejection/denial in transcript"; fi
if grep -qE "[Rr]ecord the Codex version" <<<"$RB_FLAT"; then ok "phase 7: Codex version capture"; else no "phase 7: Codex version capture"; fi
# The two-path claim also lives in setup.md — bind it there too, not only in the runbook.
SETUP="$REPO/docs/codex/setup.md"
SETUP_FLAT="$(tr '\n' ' ' < "$SETUP" | tr -s '[:space:]' ' ')"
if grep -qE "Two invocation paths" <<<"$SETUP_FLAT"; then ok "setup.md: two-invocation-paths section exists"; else no "setup.md: two-invocation-paths section exists"; fi
if grep -qE "not sufficient[^.]{0,120}definitions it loaded at start" <<<"$SETUP_FLAT"; then ok "setup.md: running-process conversations insufficient"; else no "setup.md: running-process conversations insufficient"; fi
if grep -qE "active top-level tool surface" <<<"$SETUP_FLAT"; then ok "setup.md: nested dispatch uses the top-level surface"; else no "setup.md: nested dispatch uses the top-level surface"; fi
```

- [ ] **Step 2: Run to verify it fails**

Run: `bash tests/test_codex_runbook.sh`
Expected: FAIL — phase count 6≠7, Phase-7 ordinal missing, and (if Task 3 landed) the setup.md
asserts already green; the runbook-content asserts `NOT OK`.

- [ ] **Step 3: Write Phase 7**

Append to `docs/codex/validation-runbook.md` after Phase 6 and before `## Pass criteria`
(also update the runbook's intro and `## Pass criteria` section wherever they say six phases /
enumerate phases, adding Phase 7 to the enumeration):

```markdown
## Phase 7 — Nested dispatch from both entry paths

Change 0365's live certification. Phases 1–6 proved wrappers generate, load, and dispatch once;
this phase proves a **registered parent agent can perform the nested dispatches its charter
requires**, on both supported entry paths, without falsely concluding dispatch is unavailable
from a nested tool inventory. Run it in the **disposable fixture repo** from Phase 1 — never the
real docket backlog. Install the build under test first (Phase 1), then start a **fresh Codex
process** (see `docs/codex/setup.md`, *Restart after (re)generating* — a conversation opened in
an already-running process holds stale definitions and certifies nothing).

- [ ] 1. **Record the Codex version** (`codex --version`) in the results doc before any probe —
  this certification is scoped to the exact version it ran on.

- [ ] 2. **Entry path A — prose.** In the fixture repo, issue a plain prose request the managed
  `AGENTS.md` dispatch block routes ("show me the docket status board"). Expected: the request is
  delegated to the registered `docket-status` agent, and that agent's own composition dispatches
  run as child agents.

- [ ] 3. **Entry path B — direct invocation.** Start `@docket-implement-next` against a fixture
  change staged as build-ready in the fixture repo. Expected: the run reaches Step 4 and
  dispatches the registered `docket-plan-writer` (the exact dispatch the live 0361 run falsely
  declared unavailable), then continues into build/review composition.

- [ ] 4. **Sample every composition family** across the two runs (and additional fixture
  invocations as needed): implement-next composition (`docket-status`, `docket-plan-writer`,
  `docket-adr`), profile-routed build (a `docket-build-*` profile agent), rung-routed review
  (a `docket-review-*` agent), the auto-groom critic (`docket-auto-groom-critic`), and finalize's
  resolver and repair (`docket-rebase-resolver`, `docket-integration-repair` — stage a fixture
  conflict/red-suite if needed, or record them as attempted-dispatch evidence only). For each,
  capture an **observable child-return sentinel**: a dispatch attempt **counts only when the
  child actually starts and its expected return is consumed** by the parent — model narration
  that a dispatch "happened" is not evidence.

- [ ] 5. **Negative-evidence discipline.** If any run reports dispatch unavailable, that verdict
  is valid **only** if the transcript contains the **direct rejection or an explicit policy
  denial** of an actually-attempted dispatch. A verdict derived from inspecting a nested tool
  inventory is the defect this change fixed — record it as a regression, not as an environment
  finding.

**Pass when:** both entry paths completed their nested dispatches with sentinel evidence for
every composition family sampled (or a recorded direct rejection/policy denial for any that
genuinely cannot run), and the Codex version is recorded in the results doc.
```

Adjust wording as needed so every cited repo path exists (Assertion 3 of the guard checks) and no
new phase text names a fabricated path; fixture-only paths must be listed in the guard's
`FIXTURE_PATHS` variable if any new one is introduced (prefer not to introduce any).

- [ ] **Step 4: Run to verify it passes**

Run: `bash tests/test_codex_runbook.sh`
Expected: all `ok`, including the pre-existing assertions (agent-set completeness, cited-path
existence, facade spelling — Phase 7's agent names help Assertion 2, never hurt it).

- [ ] **Step 5: Commit**

```bash
git add tests/test_codex_runbook.sh docs/codex/validation-runbook.md
git commit -m "docs(0365): runbook Phase 7 — live two-path nested-dispatch certification, guard first"
```

- [ ] **Step 6: Mutation probe (post-commit)**

Delete the entire `## Phase 7 — …` section from the runbook; run the guard; expect `NOT OK` on
phase count, ordinal 7, and every Assertion-8 runbook-content line. Restore with
`git checkout -- docs/codex/validation-runbook.md`; re-run; all `ok`.

---

### Task 5: Whole-site derivation evidence + full-suite gate

**Files:**
- No source files — verification, derivation evidence, and the suite gate. (Fix anything red; any
  fix stays inside this change's files above.)

**Interfaces:**
- Consumes: everything committed by Tasks 1–4.
- Produces: the build-evidence record docket-build carries into review/results (derivation
  classification + human-verify items).

- [ ] **Step 1: Derive nested-dispatch consumers from a whole-repo search**

Run (from the worktree root):

```bash
grep -rn --include='*.md' -E "dispatch(es|ing|ed)?[^.]{0,80}docket-|docket-(plan-writer|build|review|status|adr|auto-groom-critic|brainstorm-consultant|rebase-resolver|integration-repair)[^.]{0,60}dispatch" skills/ agents/ | grep -vE "^(skills|agents)/[^:]*:[0-9]+:\s*<!--" 
```

Sort every hit into: (a) **executable/current skill+agent sources** (`skills/*/SKILL.md`,
`agents/docket-*.md` — these are what the Codex emitter renders or what generated wrappers load)
vs (b) **prose/records** (references, docs). Confirm: every category-(a) consumer reaches Codex
through a wrapper the emitter generates — i.e., every `agents/docket-*.md` file has a
corresponding golden in `internal/harness/codex/testdata/golden/` (compare
`ls agents/docket-*.md | wc -l` against `ls internal/harness/codex/testdata/golden/docket-*.toml | wc -l`
— both 17, and name-for-name via `diff <(cd agents && ls docket-*.md | sed 's/\.md$//') <(cd internal/harness/codex/testdata/golden && ls docket-*.toml | sed 's/\.toml$//')`).
Together with Task 1's universal per-source test, this demonstrates the emitter reaches every
generated wrapper with **no hand-list of consumers**. Record the classification (counts + the
five composition families found: implement-next composition, profile-routed build, rung-routed
review, auto-groom critic, finalize resolver + repair) in the build-evidence record.

- [ ] **Step 2: Non-Codex byte-check over the whole branch**

Run: `git diff --no-renames --name-only 0b3980713dffdf41cbd2cce61b5c77ffa54e1470..HEAD -- internal/harness/claude internal/harness/cursor internal/harness/opencode agents/`
Expected: empty output — non-Codex goldens AND shared agent bodies untouched for the entire
branch (`--no-renames` so a moved file cannot hide its source — learnings:
diff-derived-allowlist-needs-no-renames).

- [ ] **Step 3: Full suite gate**

Run the resolved `finalize.test_command` (read it from config; currently `scripts/run-tests.sh`)
and `go test -race -count=1 ./...`.
Expected: SUITE summary green (trust the `SUITE …` summary line, not a piped exit code). Any
`SERIAL CONFIRMED OVER BUDGET:` line must be addressed before completion; `BUDGET WATCH:` /
`PARALLEL-SENSITIVE:` lines are screening findings — note them in evidence.

- [ ] **Step 4: Record the human-verify items**

In the build-evidence record (for the results file), state explicitly:

- Codex wrappers are **start-time-loaded artifacts**: this branch validates the *generator*
  (hermetic Go tests + goldens); the *loader* — that a freshly started Codex process actually
  carries the paragraph into behavior — requires runbook Phase 7, executed by a human against a
  fresh process, and is **not claimed as runtime-validated** by this build (learnings:
  generated-artifact-loaded-at-process-start).
- The live two-path certification (runbook Phase 7, with Codex version capture) is a named human
  verification item, not a suite item.

- [ ] **Step 5: Final commit (only if Steps 1–3 required fixes)**

```bash
git add -u  # only files this plan already owns; never add -A
git commit -m "test(0365): close full-suite gate findings"
```

---

## Self-Review (performed at plan time)

- **Spec coverage:** renderer paragraph + universal test + golden regen (Task 1 = spec §1,
  Testing/renderer regression); convention by shape + mirror + both-sided mutation (Task 2 = spec
  §2, Testing/convention guard); two paths + fresh process docs (Task 3 = spec §3, §5,
  Documentation); live two-path certification procedure + evidence rules (Task 4 = spec
  Testing/live Codex validation); derived whole-site coverage + full gate + honest human-verify
  boundary (Task 5 = spec Testing/whole-site + full gate). Spec §4 (genuine failures stay loud)
  is preserved by construction — no tier text, carve-out text, or posture is edited, and Task 2
  Probe B proves the failed-attempt clause is still guarded.
- **No new ADR:** none of the retained decisions is altered; no task writes `docs/adrs/`.
- **Type consistency:** `codexDispatchBoundary` (Task 1 Step 3) is the constant Task 1's
  mutation probe strips; the four literal clauses in `TestCodexNestedDispatchBoundary` appear
  verbatim inside the constant's text; the convention phrases asserted in Task 2 Step 1 appear
  verbatim in Task 2 Step 3's inserted text; the Phase 7 phrases asserted in Task 4 Step 1c
  appear verbatim in Task 4 Step 3's markdown; the setup.md phrases asserted in Task 4 Step 1c
  appear verbatim in Task 3 Steps 1–2.
