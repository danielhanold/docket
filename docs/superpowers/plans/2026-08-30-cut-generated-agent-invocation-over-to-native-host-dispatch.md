<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0371 — Cut generated agent invocation over to native host dispatch](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0371-cut-generated-agent-invocation-over-to-native-host-dispatch.md)**
<!-- docket:backlink:end -->
# Cut Generated Agent Invocation Over to Native Host Dispatch — Implementation Plan

> **For agentic workers:** This plan is executed under docket's build role (`docket-build`):
> one build-profile worker per task, each under the `docket-build-task` contract. Steps use
> checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `internal/harness/dispatch.go` the single canonical native-host-dispatch policy, prove all four adapters and the reposeed `docket:dispatch` block derive from it with no `runner-dispatch` residue, and strip the two maintained runner-dispatch references (plus embedded mirrors) — with mutation-tested guards throughout.

**Architecture:** The Go seam already exists: `dispatchPreamble` + `DispatchInterior(runGate)` in `internal/harness/dispatch.go` feed the four `internal/harness/{claude,codex,cursor,opencode}` adapters and `internal/reposeed.Plan`. This change (a) amends the preamble to state the never-fall-back rule explicitly, (b) adds shape-keyed, mutation-tested Go guards over every rendered dispatch surface, (c) strips the runner-era sections from `skills/docket-convention/references/agent-layer.md` and `skills/docket-build/references/delegation-execution.md` and regenerates their embedded mirrors deterministically, and (d) dispositions conflicting runner-era ADRs.

**Tech Stack:** Go (module `github.com/danielhanold/docket`), `cmd/genassets` for the embedded bundle, the repo's Bash test suite run via `go run ./cmd/docket development test`.

**Spec:** `docs/superpowers/specs/2026-08-30-cut-generated-agent-invocation-over-to-native-host-dispatch-design.md` (synchronized on the `docket` branch). Change file: `docs/changes/active/0371-cut-generated-agent-invocation-over-to-native-host-dispatch.md`.

## Global Constraints

- **DO NOT TOUCH (0370-owned frozen facade):** `scripts/runner-dispatch.sh`, `scripts/runner-dispatch.md`, `scripts/runners/*`, `sync-agents.sh`, `scripts/verify-run.md`, `scripts/docket.md`, `scripts/docket-config.md`, `tests/test_runner_dispatch*.sh`, `tests/lib/runner_dispatch_detach_common.sh`, `tests/test_runner_cursor.sh`, `tests/test_runner_opencode.sh`, `tests/test_sync_agents_*.sh`, `README.md`'s *Runner delegation* subsection. Also untouchable: archived changes, `docs/results/`, accepted ADR bodies, `docs/superpowers/plans|specs` history (point-in-time records), and `docs/codex/validation-runbook.md` / `docs/opencode/setup.md` (facade-era runbooks that go with 0370's deletion).
- **0369's guard must keep passing untouched:** `tests/test_go_consumer_migration_guard.sh`.
- **No new Go delegation/runner-dispatch verb, no new harness** (spec non-goals; acceptance 5).
- **Before removing or rewording ANY phrase from a maintained file, grep the tests tree for that phrase first** (`/usr/bin/grep -rF "<phrase>" tests/`) — asserts grep the copy, not the source (learnings: `restatement-accumulates-its-own-guards`). A dependent found is fixed by relocation/repoint, never by re-adding the deleted text.
- **Every guard is mutation-tested:** strip the guarded thing, watch the guard redden, restore, and confirm each mutation landed with a `grep -c` count before/after through `/usr/bin/grep` (PATH grep is ugrep — its regex dialect differs; use `/usr/bin/grep -cF` for landing checks). Re-run Go tests with `-count=1` after every mutation/restore (learnings: `cached-runner-serves-a-mutated-tree`).
- **Marker-managed blocks:** validate marker order and balance before any rewrite; refuse and leave the file untouched on malformed markers.
- **Stage exact paths only** — never `git add -A` (concurrent-loop discipline).
- **Skill size budgets are ceilings** (`tests/test_skill_size_budgets.sh`): `references/agent-layer.md` 205 lines / 2350 words; `references/delegation-execution.md` 85 lines / 850 words. Every edit in this plan shrinks or holds these files — never grow them past their rows.
- **Embedded bundle:** any edit under `skills/` must regenerate `internal/assets/embedded/` via `go run ./cmd/genassets` in the same commit, and a second regeneration must produce **no diff** (acceptance 10); `go run ./cmd/genassets -check` must pass.
- Full-suite gate command (read from `finalize.test_command`, currently): `go run ./cmd/docket development test`.

---

### Task 1: Canonical policy — the preamble states the never-fall-back rule (dispatch.go + dispatch_test.go)

**Files:**
- Modify: `internal/harness/dispatch.go` (const `dispatchPreamble` and its doc comment)
- Test: `internal/harness/dispatch_test.go`
- Modify: `AGENTS.md` (the repo's own `docket:dispatch` managed block — regenerated from the Go emitter, see Step 6; `CLAUDE.md` is a symlink to it and needs no edit)

**Interfaces:**
- Consumes: `harness.DispatchInterior(runGate []byte) string`, `harness.DispatchHeading` (exists), `assets.EmbeddedCatalog()`, `harness.RunGate(c assets.Catalog) ([]byte, error)`.
- Produces: the amended `dispatchPreamble` wording that Tasks 2–4 and 6 assert against. The one new sentence, verbatim (Tasks 2 and 6 grep it collapsed-whitespace): `Never reroute a registered workflow through a shell runner, another harness, a generic agent, or an inline reconstruction of its contract — a missing registration is a visible capability failure, not a fallback trigger.`

**Context for the implementer:** The current preamble (see `dispatchPreamble` in `internal/harness/dispatch.go`) already states: exact registered same-name `docket-*` agent, native named-agent dispatch, request passed through unchanged, registry authoritative, no invented agent. What it does NOT state is the spec's never-fall-back clause (spec § Canonical dispatch policy; change file `## What changes` bullet 1). Gate bracketing is carried by the run-gate payload `DispatchInterior` appends (`TestDispatchInteriorCarriesGate` already pins that) — do not restate the gate protocol in the preamble.

Two coupled copies exist and are handled differently:
- `sync-agents.sh` carries a textual twin of the preamble. It is FROZEN (0370-owned) — do not edit it. No test asserts Go/shell textual parity (verified at planning: the only coupling is `tests/test_dispatch_block_budget.sh`, which measures the *shell*-emitted block in a hermetic fixture and is unaffected). But `dispatch.go`'s doc comment currently claims "the two are now textually identical here" — that claim becomes false and MUST be updated in the same commit (a stale comment in maintained source is a defect).
- The repo's own `AGENTS.md` `docket:dispatch` block embeds the preamble verbatim. It is a generated derivative — regenerate it from the Go emitter (Step 6), never hand-write the new sentence into it.

- [ ] **Step 1: Write the failing test.** Append to `internal/harness/dispatch_test.go` (package `harness`, so `dispatchPreamble` is directly visible; follow the file's existing style):

```go
// TestDispatchPreambleStatesNativeOnlyPolicy pins the canonical policy's
// load-bearing clauses (change 0371, spec § Canonical dispatch policy). Asserts
// collapse whitespace so a re-wrap never reddens them, and each phrase is a
// verbatim slice of the claim, not a common noun (learnings:
// assert-detects-removal-not-replacement).
func TestDispatchPreambleStatesNativeOnlyPolicy(t *testing.T) {
	collapse := func(s string) string { return strings.Join(strings.Fields(s), " ") }
	got := collapse(dispatchPreamble)
	for _, phrase := range []string{
		// exact registered same-name identity
		"registered same-name `docket-*` agent",
		// native facility, request forwarded unchanged
		"harness's native named-agent dispatch, and pass the request through unchanged",
		// registry is authoritative; no invented registration
		"native agent registry is authoritative",
		"do not invent one",
		// the never-fall-back rule (NEW in 0371)
		"Never reroute a registered workflow through a shell runner, another harness, a generic agent, or an inline reconstruction of its contract",
		"a missing registration is a visible capability failure, not a fallback trigger",
	} {
		if !strings.Contains(got, phrase) {
			t.Errorf("dispatchPreamble lost the policy clause %q", phrase)
		}
	}
	// The policy must never name or recommend the retired runner path, and must
	// stay machine-neutral (no harness names — the same interior lands on every
	// host surface).
	for _, banned := range []string{"runner-dispatch", "docket.sh", "scripts/runners", "claude", "codex", "cursor", "opencode"} {
		if strings.Contains(strings.ToLower(got), banned) {
			t.Errorf("dispatchPreamble contains banned token %q", banned)
		}
	}
}
```

Note `"claude"` etc. are lowercase-folded checks against the *preamble only* (which today names no harness) — NOT against the interior, whose run-gate payload legitimately names `docket-implement-next` (contains no harness name either, but keep the ban scoped to the preamble regardless).

- [ ] **Step 2: Run it, confirm it fails on exactly the two NEW phrases.** `go test ./internal/harness/ -run TestDispatchPreambleStatesNativeOnlyPolicy -count=1` → FAIL, two `lost the policy clause` lines (the never-fall-back sentence and the capability-failure clause). If it fails on any *other* phrase, the phrase was transcribed wrong — fix the test, not the preamble.

- [ ] **Step 3: Amend `dispatchPreamble`.** Append the new sentence to the constant (continuing the existing `+`-concatenated string style, wrapped to match), so the preamble ends:

```go
const dispatchPreamble = "When a requested Docket workflow has a registered same-name `docket-*` agent, dispatch that agent\n" +
	"instead of running the workflow inline: the agent carries that workflow's dispatch contract, its\n" +
	"skill preload, and whatever model and reasoning effort your config layers pin for it. Your\n" +
	"harness's native agent registry is authoritative for agent names, descriptions, and availability —\n" +
	"this block does not restate it. If no same-name agent is registered, do not invent one; follow the\n" +
	"workflow's own inline or unavailable-capability contract. Dispatch through the harness's native\n" +
	"named-agent dispatch, and pass the request through unchanged, including any change or ADR id.\n" +
	"Never reroute a registered workflow through a shell runner, another harness, a generic agent, or\n" +
	"an inline reconstruction of its contract — a missing registration is a visible capability\n" +
	"failure, not a fallback trigger."
```

In the same edit, fix the constant's doc comment: the paragraph beginning "With the list gone the two are now textually identical here" must now say the Go emitter is canonical and `sync-agents.sh`'s frozen mirror intentionally lags one sentence behind (change 0371; the mirror is deleted by change 0370). Keep the machine-neutral rationale paragraph — it is still true and Task 2's guards depend on the property it documents.

- [ ] **Step 4: Run the package tests.** `go test ./internal/harness/... -count=1` → PASS. If `TestDispatchInterior` or an adapter golden pins the old preamble bytes, update that golden to the new constant — the goldens are derivatives of this constant, and this is the one commit where they legitimately move.

- [ ] **Step 5: Mutation-test the guard.** (a) Delete the new sentence from the constant → `TestDispatchPreambleStatesNativeOnlyPolicy` reds on the never-fall-back phrase. (b) Weaken `same-name` to a generic "an appropriate agent" spelling → reds on the identity phrase. (c) Insert the token `runner-dispatch` into the preamble → reds on the banned-token loop. Confirm each mutation landed via `/usr/bin/grep -cF` on `dispatch.go` before/after; restore the real constant after each probe and re-run with `-count=1`.

- [ ] **Step 6: Regenerate the repo's own AGENTS.md dispatch block from the Go emitter.** Never hand-type the new sentence into `AGENTS.md`. Create a throwaway printer inside the module (same module ⇒ internal imports resolve), splice between the markers after validating them, then delete the printer:

```bash
cd <worktree-root>
mkdir -p tmp_printdispatch
cat > tmp_printdispatch/main.go <<'EOF'
package main

import (
	"fmt"
	"os"

	"github.com/danielhanold/docket/internal/assets"
	"github.com/danielhanold/docket/internal/harness"
)

func main() {
	c, err := assets.EmbeddedCatalog()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	rg, err := harness.RunGate(c)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Print(harness.DispatchInterior(rg))
}
EOF
go run ./tmp_printdispatch > tmp_printdispatch/interior.md

# Marker order/balance gate (AGENTS.md rule): exactly one start, one end, start first.
starts=$(/usr/bin/grep -cF "docket:dispatch:start" AGENTS.md)
ends=$(/usr/bin/grep -cF "docket:dispatch:end" AGENTS.md)
s_line=$(/usr/bin/grep -nF "docket:dispatch:start" AGENTS.md | cut -d: -f1)
e_line=$(/usr/bin/grep -nF "docket:dispatch:end" AGENTS.md | cut -d: -f1)
[ "$starts" = "1" ] && [ "$ends" = "1" ] && [ "$s_line" -lt "$e_line" ] || { echo "REFUSE: malformed dispatch markers"; exit 1; }

awk -v interior=tmp_printdispatch/interior.md '
  /docket:dispatch:start/ { print; while ((getline line < interior) > 0) print line; skip=1; next }
  /docket:dispatch:end/   { skip=0 }
  !skip { print }
' AGENTS.md > AGENTS.md.new && mv -f AGENTS.md.new AGENTS.md
rm -rf tmp_printdispatch
```

Then verify: `/usr/bin/grep -cF "Never reroute a registered workflow" AGENTS.md` → 1, and `git diff AGENTS.md` shows changes only between the markers.

- [ ] **Step 7: Suite-coupling check.** The Go emitter and the repo AGENTS.md block both changed prose. Run `/usr/bin/grep -rF "named-agent dispatch, and pass the request through unchanged" tests/` (the phrase survives — nothing to repoint) and run the shell tests that read this surface: `bash tests/test_dispatch_block_budget.sh` (fixture uses frozen `sync-agents.sh`; must stay green and untouched).

- [ ] **Step 8: Commit.**

```bash
git add internal/harness/dispatch.go internal/harness/dispatch_test.go AGENTS.md
git commit -m "feat(0371): dispatchPreamble states the never-fall-back native-dispatch rule"
```

---

### Task 2: Cross-surface guards — every rendered dispatch surface is native-only, byte-stable, and runner-free

**Files:**
- Create: `internal/harness/native_dispatch_test.go` (package `harness_test` — same external package as `cross_harness_test.go`, so it can reuse `crossPlanInput`, `collapseWS`, `harness.Order`)
- Test (modify): `internal/reposeed/plan_test.go`

**Interfaces:**
- Consumes: `crossPlanInput(t)` and `collapseWS` from `internal/harness/cross_harness_test.go`; adapters `claude.New()`, `codex.New()`, `cursor.New()`, `opencode.New()` each satisfying `harness.Adapter` with `Plan(in harness.PlanInput) ([]install.Target, error)`; `reposeed.Plan(reposeed.PlanInput) ([]install.Target, map[string][]string, error)` with `PlanInput{WorktreeRoot, Harnesses, RunGate}`; `harness.DispatchHeading`.
- Produces: `TestNativeDispatchSurfaceRunnerFree`, `TestAdapterRenderByteStable` (harness), `TestPlanRunnerFreeAndByteStable` (reposeed) — the guards Task 6's mutation matrix drives.

- [ ] **Step 1: Write the failing-capable harness-side test.** Create `internal/harness/native_dispatch_test.go`:

```go
// Change 0371: the maintained native-dispatch surface — everything the four
// adapters render — must carry the canonical native policy and no runner-era
// routing. Keyed on the targets each adapter actually renders, never a hand
// list of files (AGENTS.md: derive sites, don't enumerate them).
package harness_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/harness"
	"github.com/danielhanold/docket/internal/harness/claude"
	"github.com/danielhanold/docket/internal/harness/codex"
	"github.com/danielhanold/docket/internal/harness/cursor"
	"github.com/danielhanold/docket/internal/harness/opencode"
)

func crossAdapters(t *testing.T) map[string]harness.Adapter {
	t.Helper()
	a := map[string]harness.Adapter{
		"claude":   claude.New(),
		"codex":    codex.New(),
		"cursor":   cursor.New(),
		"opencode": opencode.New(),
	}
	if len(a) != len(harness.Order) {
		t.Fatalf("guard covers %d adapters for %d harnesses in Order", len(a), len(harness.Order))
	}
	return a
}

// runnerEraTokens are the retired Bash delegation spellings. Zero occurrences
// in ANY rendered target of ANY adapter. The list is a ban on spellings, not
// the property itself (learnings: byte-pattern-guard-matches-a-spelling) —
// the property "no shell/cross-harness fallback" is carried by the preamble
// phrase assert below plus TestNoCrossHarnessDelegation's shape guard.
var runnerEraTokens = []string{"runner-dispatch", "docket.sh", "scripts/runners"}

func TestNativeDispatchSurfaceRunnerFree(t *testing.T) {
	in := crossPlanInput(t)
	adapters := crossAdapters(t)

	// The two clauses every parent-facing dispatch surface must carry, spelled
	// here independently of the emitter (an assert built FROM dispatchPreamble
	// would move in lockstep with a mutated emitter and stay green).
	const identityClause = "registered same-name `docket-*` agent"
	const neverFallBack = "Never reroute a registered workflow through a shell runner, another harness, a generic agent"

	dispatchSurfaces := 0
	for _, name := range harness.Order {
		targets, err := adapters[name].Plan(in)
		if err != nil {
			t.Fatalf("%s Plan: %v", name, err)
		}
		if len(targets) == 0 {
			t.Fatalf("%s rendered no targets — the ban below would be vacuous", name)
		}
		for _, tg := range targets {
			body := collapseWS(string(tg.Content))
			lower := strings.ToLower(body)
			for _, tok := range runnerEraTokens {
				if strings.Contains(lower, tok) {
					t.Errorf("%s target %q carries runner-era token %q", name, tg.Path, tok)
				}
			}
			if strings.Contains(body, harness.DispatchHeading) {
				dispatchSurfaces++
				if !strings.Contains(body, identityClause) {
					t.Errorf("%s dispatch surface %q lost the exact-identity clause", name, tg.Path)
				}
				if !strings.Contains(body, neverFallBack) {
					t.Errorf("%s dispatch surface %q lost the never-fall-back clause", name, tg.Path)
				}
			}
		}
	}
	// Population floor (learnings: marker-scoped-guard-needs-a-population-floor):
	// at least one adapter renders a parent-facing dispatch surface, or every
	// per-surface assert above quietly iterated zero times.
	if dispatchSurfaces == 0 {
		t.Fatalf("no adapter rendered a dispatch surface carrying %q", harness.DispatchHeading)
	}
}

// TestAdapterRenderByteStable: rendering is deterministic — a second Plan over
// the same input yields byte-identical content for every target (acceptance 10).
func TestAdapterRenderByteStable(t *testing.T) {
	in := crossPlanInput(t)
	adapters := crossAdapters(t)
	for _, name := range harness.Order {
		first, err := adapters[name].Plan(in)
		if err != nil {
			t.Fatalf("%s first Plan: %v", name, err)
		}
		second, err := adapters[name].Plan(in)
		if err != nil {
			t.Fatalf("%s second Plan: %v", name, err)
		}
		if len(first) != len(second) {
			t.Fatalf("%s target count moved between renders: %d then %d", name, len(first), len(second))
		}
		for i := range first {
			if first[i].Path != second[i].Path || !bytes.Equal(first[i].Content, second[i].Content) {
				t.Errorf("%s target %q is not byte-stable across renders", name, first[i].Path)
			}
		}
	}
}
```

- [ ] **Step 2: Run it.** `go test ./internal/harness/ -run 'TestNativeDispatchSurfaceRunnerFree|TestAdapterRenderByteStable' -count=1`. Expected: PASS if Task 1 landed (the surface is already runner-free — these guards exist to *hold* the state, and Task 6 proves they can redden). If `identityClause`/`neverFallBack` fail, the preamble text and the test disagree — reconcile against Task 1's exact constant.

- [ ] **Step 3: Add the reposeed-side guard.** Append to `internal/reposeed/plan_test.go` (helpers `mustPlan`, `runGate`, `worktreeRoot` already exist there):

```go
// Change 0371: the parent-facing docket:dispatch block is native-only and
// deterministic. Iterates every target Plan renders — never a path allowlist.
func TestPlanRunnerFreeAndByteStable(t *testing.T) {
	in := PlanInput{
		WorktreeRoot: worktreeRoot,
		Harnesses:    []string{"claude", "codex", "cursor", "opencode"},
		RunGate:      runGate,
	}
	first, _ := mustPlan(t, in)
	if len(first) == 0 {
		t.Fatal("Plan rendered no targets — the ban below would be vacuous")
	}
	for _, tg := range first {
		lower := strings.ToLower(strings.Join(strings.Fields(string(tg.Content)), " "))
		for _, tok := range []string{"runner-dispatch", "docket.sh", "scripts/runners"} {
			if strings.Contains(lower, tok) {
				t.Errorf("dispatch target %q carries runner-era token %q", tg.Path, tok)
			}
		}
	}
	second, _ := mustPlan(t, in)
	if len(first) != len(second) {
		t.Fatalf("target count moved between renders: %d then %d", len(first), len(second))
	}
	for i := range first {
		if first[i].Path != second[i].Path || !bytes.Equal(first[i].Content, second[i].Content) {
			t.Errorf("target %q is not byte-stable across renders", first[i].Path)
		}
	}
}
```

Add `"strings"` to the file's imports (`bytes` is already imported).

- [ ] **Step 4: Run the two packages.** `go test ./internal/harness/... ./internal/reposeed/... -count=1` → PASS.

- [ ] **Step 5: Mutation-test.** (a) Insert the literal `runner-dispatch` into `dispatchPreamble` → both new runner-free tests red (confirms the ban reaches every surface through the one seam). (b) In `cursor.go`'s `dispatchRuleContent` (the `dispatchRuleFrontmatter + harness.DispatchInterior(runGate)` expression), replace the interior with a fixed string lacking the heading → `TestNativeDispatchSurfaceRunnerFree`'s population floor or per-surface clause asserts red for cursor. (c) Make one adapter append a timestamp-ish suffix to a target's content (e.g. concatenate `strconv.Itoa(len(targets))` twice differently — any second-render divergence) → the byte-stable test reds. Verify each mutation landed with `/usr/bin/grep -cF` before/after; restore, re-run `-count=1`.

- [ ] **Step 6: Commit.**

```bash
git add internal/harness/native_dispatch_test.go internal/reposeed/plan_test.go
git commit -m "test(0371): native-only, runner-free, byte-stable guards over every rendered dispatch surface"
```

---

### Task 3: Strip the `runner:` cross-harness delegation section from agent-layer.md (+ embedded mirror)

**Files:**
- Modify: `skills/docket-convention/references/agent-layer.md`
- Modify (regenerated, never hand-edited): `internal/assets/embedded/tree/skills/docket-convention/references/agent-layer.md`, `internal/assets/embedded/manifest.json` (whatever `genassets` rewrites)

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: an agent-layer reference with no cross-harness-delegation instruction; the embedded mirror in lockstep.

**Context:** The strip target is three consecutive bold-led paragraphs in the *user config layering* area of the file (all three exist only for the retired runner path):
1. `**\`runner:\` — cross-harness delegation (change 0079).**…` (names `docket.sh runner-dispatch`, `--launch`/`--observe`, `runners.<name>:`, `scripts/runners/<name>.md`, README's *Runner delegation* subsection),
2. `**Model and effort on a delegated agent (0168, 0205).**…`,
3. `**The shim's own pin is a third value (change 0269).**…` (names `docket.sh runner-dispatch` again).

KEEP: the shipped-layer paragraph's clause "…and forbids `runner:`, since delegation is user policy, never a shipped default" — it documents the still-live validator in the frozen facade and is itself anti-runner; it recommends nothing. Planning-time verification found NO test asserting runner/shim prose inside agent-layer.md (`tests/test_sync_agents_drift_docs.sh`, `test_sync_agents_defaults.sh`, `test_cursor_contract_docs.sh`, `test_skill_fork_dispatch.sh` all grep other sections), but re-verify at build time (Step 1) — the planning snapshot is a hypothesis, not an oracle.

- [ ] **Step 1: Derive the coupling set before editing.** For each of the three paragraphs, extract 3–4 distinctive verbatim slices (e.g. `cross-harness delegation (change 0079)`, `launch-then-observe`, `runners.<name>`, `shim_model`, `never forwarded to`) and run `/usr/bin/grep -rF "<slice>" tests/` for each. Expected: zero hits outside `tests/test_skill_size_budgets.sh` ledger *comments* (comments are not asserts). Any real assert hit → repoint it at the surviving owner of that claim per the relocation discipline; do NOT keep the paragraph to keep a grep green. Record the grep outputs in the task's commit message body.

- [ ] **Step 2: Edit the file.** Delete the three paragraphs. In their place insert one short paragraph so a reader looking for the old `runner:` key gets a live answer instead of silence:

```markdown
**Cross-harness delegation is retired (change 0371).** An agent entry carries no `runner:` key on the
maintained surface: a parent invokes a registered `docket-*` agent through its own harness's native
named-agent dispatch — the generated `docket:dispatch` block is the contract — and a workflow with no
registration on the current host fails visibly rather than falling back to a shell runner, another
harness, or a generic agent.
```

- [ ] **Step 3: Verify the residue and the budget.** `/usr/bin/grep -n "runner-dispatch\|docket\.sh\|launch-then-observe\|shim" skills/docket-convention/references/agent-layer.md` → the only surviving `runner` spellings are the kept "forbids `runner:`" clause and the new retirement paragraph's `runner:`/"shell runner" mentions; no `runner-dispatch`, no `docket.sh`. `wc -l -w skills/docket-convention/references/agent-layer.md` → at or under 205 lines / 2350 words (the edit shrinks the file; do not lower the budget row — ratchet moves are their own change).

- [ ] **Step 4: Regenerate the embedded bundle, twice.**

```bash
go run ./cmd/genassets
git status --porcelain internal/assets/embedded/   # shows the mirror updated
go run ./cmd/genassets                              # second run:
git diff --stat internal/assets/embedded/           # MUST print nothing new (deterministic)
go run ./cmd/genassets -check                       # MUST pass
go test ./internal/assets/ -count=1                 # drift gate green
```

- [ ] **Step 5: Run the coupled shell tests.** `bash tests/test_sync_agents_drift_docs.sh && bash tests/test_sync_agents_defaults.sh && bash tests/test_cursor_contract_docs.sh && bash tests/test_skill_fork_dispatch.sh && bash tests/test_skill_size_budgets.sh` → all green.

- [ ] **Step 6: Commit.**

```bash
git add skills/docket-convention/references/agent-layer.md internal/assets/embedded/
git commit -m "refactor(0371): retire the runner: cross-harness delegation section from agent-layer.md"
```

---

### Task 4: Strip runner-dispatch references from delegation-execution.md under the frozen-guard constraint (+ embedded mirror)

**Files:**
- Modify: `skills/docket-build/references/delegation-execution.md`
- Modify (regenerated): `internal/assets/embedded/tree/skills/docket-build/references/delegation-execution.md` + manifest

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: a delegation-execution reference that recommends no runner-dispatch invocation, while every assertion in frozen `tests/test_runner_dispatch_build_gate.sh` stays green.

**Context — the binding constraint:** `tests/test_runner_dispatch_build_gate.sh` (0370-owned, UNTOUCHABLE) asserts against this file (`DEL=`): the file **exists**; for every harness in `HD_SHIPPED_HARNESSES` a table row `^|<space>*<harness>[space|]` exists; each row contains `unverified` and not `supported`. It does NOT grep the file for `runner-dispatch` spellings. Step 1 re-reads the whole test to confirm no further `"$DEL"` asserts before editing. The three references to strip (dispatch payload + spec): the header's `scripts/runner-dispatch.md` framing, the mechanism section's `tests/test_runner_dispatch_detach.sh` pointer, and the probe recipe's `docket.sh runner-dispatch --launch` command.

- [ ] **Step 1: Read the frozen guard end-to-end.** `/usr/bin/grep -n '"\$DEL"' tests/test_runner_dispatch_build_gate.sh` and read every matching assert plus 5 lines of context. Confirm the constraint set above is complete; if an additional `$DEL` assert greps prose this task rewrites, adjust the rewrite to keep that exact phrase (relocation is not available — the test is frozen).

- [ ] **Step 2: Rewrite the file.** Keep the title, the two-verdict-scoping paragraphs, the mechanism findings, and the per-harness verdict table (rows byte-untouched). Make exactly these changes:
  - Header sentence `Reference for \`scripts/runner-dispatch.md\` § *Delegation execution posture*.` → replace with: `Point-in-time evidence record for the retired Bash delegation facade's adapter launch shape. The maintained dispatch surface is host-native (change 0371) and never invokes the facade; this page is kept as the measurement record behind the frozen facade until change 0370 deletes it.`
  - Mechanism section: the sentence ``tests/test_runner_dispatch_detach.sh` pins it with a fake adapter, and the assert is mutation-tested by removing `set -m`.`` → replace with: `The frozen facade's own detach test pins it with a fake adapter, and that assert is mutation-tested by removing \`set -m\`.` (drops the maintained cross-reference to a 0370-owned test file without falsifying the record).
  - Probe recipe step 1: delete the verbatim command `` `docket.sh runner-dispatch --launch --runner <h> --agent status` `` and reword the step to: `Launch a delegated agent through the frozen facade's launch verb (see the facade's own contract doc) with a task deliberately longer than the parent harness's foreground ceiling.` The recipe stays (it is the honest record of what a re-probe would take) but the maintained surface no longer spells the invocation.
  - The sentence citing `scripts/runners/<name>.md` in the table's footnote: reword `(scripts/runners/<name>.md and its sibling script)` → `(each runner's own adapter contract in the frozen facade)`.
  - Add nothing else; the file must not grow past 85 lines / 850 words (it shrinks).

- [ ] **Step 3: Verify.** `/usr/bin/grep -c "runner-dispatch" skills/docket-build/references/delegation-execution.md` → 0. `bash tests/test_runner_dispatch_build_gate.sh` → green (rows intact). `bash tests/test_skill_size_budgets.sh` → green. `/usr/bin/grep -rF "delegation-execution" tests/ | /usr/bin/grep -v test_runner_dispatch_build_gate` → only the size-budget ledger comment.

- [ ] **Step 4: Regenerate the embedded bundle, twice, exactly as Task 3 Step 4** (`go run ./cmd/genassets` ×2 with a no-diff second run, `-check`, `go test ./internal/assets/ -count=1`).

- [ ] **Step 5: Confirm the whole-repo maintained surface is clean (acceptance 15).** Run the derivation grep and sort it:

```bash
/usr/bin/grep -rln "runner-dispatch" --include="*.md" --include="*.go" . | /usr/bin/grep -v "^\./\.git"
```

Every remaining hit must fall into: (a) the 0370-owned frozen facade (`scripts/`, `README.md` runner subsection, `sync-agents.sh`, `tests/`), (b) point-in-time records (`docs/changes/archive/`, `docs/results/`, `docs/superpowers/`, ADR bodies, `docs/codex/`, `docs/opencode/`), or (c) nothing else. A hit outside those buckets is unfinished work for THIS task. Paste the sorted classification into the commit message body.

- [ ] **Step 6: Commit.**

```bash
git add skills/docket-build/references/delegation-execution.md internal/assets/embedded/
git commit -m "refactor(0371): delegation-execution.md stops recommending the retired runner-dispatch facade"
```

---

### Task 5: ADR audit and disposition (build-time decision task)

**Files:**
- Read: `docs/adrs/README.md`, `docs/adrs/0036-*.md`, `docs/adrs/0074-*.md`, and the runner-era set: `0037`, `0038`, `0067` (if present under its slug), `0068`, `0079`, `0080`, `0081`, `0083`, `0084`, `0087`, `0088`.
- Possibly create/modify: ADR files — but ONLY through the `docket-adr` agent (CLAUDE.md: dispatch the registered same-name agent for ADR transactions; never run the ADR workflow inline).

**Interfaces:**
- Consumes: the merged state of Tasks 1–4 (the maintained surface is native-only).
- Produces: either (a) a recorded no-op with reasoning in the results-facing commit message, or (b) formal supersede/reverse transactions via `docket-adr`, plus at most one new ADR.

**Decision rule (spec § ADR handling; acceptance 14):**
1. ADR-0036 (machine-neutral committed AGENTS.md dispatch block) and ADR-0074 (gate tri-state) are **preserved** — never rewritten. Verify nothing in Tasks 1–4 contradicted them (Task 1 kept the interior machine-neutral; nothing touched gate semantics).
2. A runner-era ADR **normatively conflicts** only if it *directs the maintained dispatch surface* to route through the runner path. An ADR that records the frozen facade's internal mechanics (detach posture 0080, per-platform process groups 0081, liveness 0087, halt codes 0088, brief channel, anchors 0068) governs code that is still physically present and 0370-owned — those are NOT dispositioned here; deleting the facade (0370) is when they fall. The live candidates are **ADR-0037** (`runner:` field is the delegation opt-in) and **ADR-0038** (shim wrapper is the single dispatch chokepoint): after Task 3 the maintained surface documents no `runner:` key and no shim. Read both bodies; if their decisions bind the *generated agent layer* (they do, on their face), disposition them as Superseded by this change's decision, via dispatch to `docket-adr` with the change id 371. If reading shows a prior change already dispositioned them (check the status column in `docs/adrs/README.md` first), record the no-op.
3. **New ADR only if needed:** if the accepted set after step 2 nowhere states "native host dispatch is authoritative for registered docket agents; missing registration fails visibly with no shell/cross-harness/generic/inline fallback", record ONE new ADR saying exactly that (again via `docket-adr`, change id 371). If ADR-0036 + the superseding notes already carry the complete claim, add none (spec: avoid a redundant ADR). This is a judgment call the worker makes from the actual ADR texts — record the reasoning either way.

- [ ] **Step 1:** Read `docs/adrs/README.md`'s index rows for the ADRs above; note current statuses.
- [ ] **Step 2:** Read ADR-0036, ADR-0074, ADR-0037, ADR-0038 in full; apply the decision rule.
- [ ] **Step 3:** If any disposition or new ADR is required, dispatch the registered `docket-adr` agent (native named-agent dispatch, request passed through unchanged: name the ADR ids, the action — supersede/reverse/record — and change id 371). The agent owns the ledger transaction and index regeneration.
- [ ] **Step 4:** Verify: `bash tests/test_adr_ledger.sh` if it exists (`ls tests/ | /usr/bin/grep -i adr` to find the actual ADR suite file) → green; `git log --oneline -3` shows the ADR commit(s) if any.
- [ ] **Step 5:** If the outcome was a no-op, commit nothing here but write the reasoning into the Task 6 evidence; if ADR files changed outside the `docket-adr` agent's own commit, stage exactly those paths and commit with `docs(0371): adr dispositions for the native-dispatch cutover`.

---

### Task 6: Mutation matrix, fresh-install coverage confirmation, and the full-suite gate

**Files:**
- Read/verify: `internal/install/service_test.go` (isolated `t.TempDir` HOME/XDG install coverage), `tests/test_go_consumer_migration_guard.sh`
- Possibly modify: `internal/install/service_test.go` (only if Step 2 finds the runner-free assert missing over installed output)

**Interfaces:**
- Consumes: every guard from Tasks 1–2; the stripped files from Tasks 3–4.
- Produces: the recorded mutation evidence (for the results file the parent workflow writes) and a green full suite.

- [ ] **Step 1: Run the spec's required mutation matrix (spec § Testing; acceptance 13).** For each, confirm the mutation landed (`/usr/bin/grep -cF` before/after), observe the named guard redden, restore, and re-run green with `-count=1`:

| # | Mutation | Must redden |
|---|---|---|
| 1 | Insert `runner-dispatch` guidance into `dispatchPreamble` | `TestDispatchPreambleStatesNativeOnlyPolicy` + `TestNativeDispatchSurfaceRunnerFree` + `TestPlanRunnerFreeAndByteStable` |
| 2 | Drop native dispatch from ONE host: make `cursor.go`'s rule content skip `harness.DispatchInterior` (fixed string, no heading) | `TestNativeDispatchSurfaceRunnerFree` (cursor clause asserts) — and confirm the other three hosts alone did NOT satisfy the population floor for cursor's surface |
| 3 | Delete the gate payload append in `DispatchInterior` (drop the `runGate` write) | `TestDispatchInteriorCarriesGate` (existing) |
| 4 | Weaken exact identity: change the preamble's `same-name` clause to a generic-agent spelling | `TestDispatchPreambleStatesNativeOnlyPolicy` (identity phrase) + the surface clause assert in `TestNativeDispatchSurfaceRunnerFree` |
| 5 | Make missing registration fall back: rewrite the preamble's "do not invent one; follow the workflow's own inline or unavailable-capability contract" to "run the workflow inline yourself" | `TestDispatchPreambleStatesNativeOnlyPolicy` ("do not invent one" phrase) |

Record each cell's before-count, red test name, and after-restore green run — this table is the results-file evidence.

- [ ] **Step 2: Fresh isolated four-host install coverage (acceptance 11).** Read `internal/install/service_test.go` and confirm: (a) installs run against `t.TempDir()`-rooted HOME/XDG (no developer-machine state), (b) all four harnesses are exercised. Then check whether any install-level test asserts the *installed* files are runner-free. If not, add one focused assert to the existing four-host install test (walk the installed tree, `strings.Contains(lower(content), "runner-dispatch")` → error) — an install-boundary twin of Task 2's render-boundary ban; skip if Task 2's coverage is judged sufficient AND the installed bytes are the rendered bytes by construction (they pass through `install.Target.Content` unmodified — verify that claim in `internal/install/txn.go` before waiving, and record the verification).

- [ ] **Step 3: 0369's guard, untouched and green.** `git diff main -- tests/test_go_consumer_migration_guard.sh` → empty; `bash tests/test_go_consumer_migration_guard.sh` → green.

- [ ] **Step 4: Embedded determinism, final check.** `go run ./cmd/genassets -check` → pass; `git status --porcelain internal/assets/embedded/` → clean.

- [ ] **Step 5: Full-suite gate.** `go run ./cmd/docket development test` → suite green. Read any `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` lines as screening findings; a `SERIAL CONFIRMED OVER BUDGET:` line is a breach to act on (serial-confirm per `tests/README.md` before treating a parallel number as real).

- [ ] **Step 6: Commit** (only if Step 2 added the install assert):

```bash
git add internal/install/service_test.go
git commit -m "test(0371): installed dispatch surface is runner-free (isolated four-host install)"
```

---

## Acceptance criteria → task map (self-review)

| Criterion | Where |
|---|---|
| 1 canonical policy owns shared semantics | Task 1 (preamble is the one seam; adapters/reposeed consume `DispatchInterior`) |
| 2 deterministic per-host render | Task 2 (`TestAdapterRenderByteStable`, `TestPlanRunnerFreeAndByteStable`) |
| 3 exact same-name agent, request preserved | Task 1 phrases + Task 2 surface clause asserts |
| 4 no maintained dispatch block recommends runner-dispatch | Task 2 token bans; Tasks 3–4 strips |
| 5 no new Go delegation op / harness | Global constraint (nothing in this plan adds one) |
| 6 missing registration fails visibly, no fallback | Task 1 never-fall-back sentence + mutation 5 |
| 7–9 gate caller-side, retry-once, fail-closed outcomes | carried by the run-gate payload `DispatchInterior` appends — pinned by existing `TestDispatchInteriorCarriesGate` + mutation 3; gate semantics deliberately untouched (spec § Gate preservation) |
| 10 marker-safe, deterministic, clean repeat render | Task 1 Step 6 marker gate; Tasks 3–4 double `genassets`; Task 2 byte-stable tests |
| 11 fresh isolated four-host coverage | Task 6 Step 2 |
| 12 identity/forwarding/missing-agent/completion/gate/retry coverage | Tasks 1–2 tests + existing `cross_harness_test.go` suite + run-gate payload asserts |
| 13 mutations fail for the intended reason | Task 6 Step 1 matrix (+ per-task mutation steps) |
| 14 ADR dispositions without rewriting history | Task 5 |
| 15 maintained surface has no runner-dispatch call | Task 4 Step 5 whole-repo derivation grep |
| 16 PR absorbs no 0369/0370/0372/release work | Global constraints (frozen-facade DO-NOT-TOUCH list) |
| 17 full suite passes | Task 6 Step 5 |

Learnings applied: `restatement-accumulates-its-own-guards` (Tasks 3–4 pre-edit suite greps), `byte-pattern-guard-matches-a-spelling` + `assert-detects-removal-not-replacement` (Task 2 guard design: independent spellings, population floors, non-vacuity), `cached-runner-serves-a-mutated-tree` (`-count=1` everywhere), `config-edit-trips-its-own-frozen-drift-guard` / drift gates (regenerate embedded in the same commit as every `skills/` edit), `phrase-grep-over-wrapped-prose` (collapsed-whitespace phrase asserts), `agent-executed-markdown-is-code` (the stripped `.md` command blocks were executable surface — which is why they, not just prose, had to go).
