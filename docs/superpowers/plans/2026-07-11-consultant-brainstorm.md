# Consultant-authored brainstorm Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an opt-in, off-by-default consultant-author pattern for the brainstorm role: a new `skills/docket-brainstorm` (single-dispatch consultant flow) + a pinned `agents/docket-brainstorm-consultant.md` wrapper that wraps no skill and injects no convention, activatable per-invocation (verbal) or durably (`skills: brainstorm: docket-brainstorm`), documented prominently in the README, degrading inline-with-warning when the consultant can't be dispatched.

**Architecture:** The parent (session model) runs the real human dialogue inline; once the design settles, `docket-brainstorm` dispatches the pinned consultant ONCE (in-context return, the finalize gate-agent contract) which either authors the spec or returns critique concerns for another human round. The consultant wrapper is a built-in `agents/docket-*.md` file with NO `skills:` line — sync-agents.sh uses built-in wrapper frontmatter verbatim except for model/effort, so "no skill, no convention" needs no sync-agents.sh code change. Binding uses the existing 0049 `skills:` passthrough (no new machinery). Off by default: the built-in brainstorm default stays `superpowers:brainstorming`.

**Tech Stack:** Markdown (skill, agent wrapper, Cursor dispatch fragment, README), bash sentinel tests under `tests/`. No script logic changes; sync-agents.sh auto-discovers the new wrapper via its `agents/docket-*.md` glob.

## Global Constraints

- **Off by default — zero behavior shift for existing repos.** The built-in brainstorm role default stays `superpowers:brainstorming`; `docket-new-change`/`docket-groom-next` are unchanged except a one-line verbal-trigger discoverability note. A repo opts in per-invocation (verbal) or durably (`skills: brainstorm: docket-brainstorm`).
- **The consultant wrapper injects NEITHER a wrapped skill NOR `docket-convention`** — a deliberate, documented deviation from the ADR-0009 critic (which injects the convention). The consultant authors prose and performs ZERO docket operations (no git, no status writes, no board), so the convention-vocabulary risk is nil; a compact brief rides the dispatch prompt instead. Default pin **model: claude-opus-4-8, effort: xhigh**; config key `brainstorm-consultant`.
- **Single dispatch, fully harness-portable.** No `SendMessage`/agent-continuation anywhere; no pre-dialogue analysis call; no relay/ping-pong; no simulated-human answering (ADR-0006 boundary — the dialogue stays with the real human, inline). One fresh consultant dispatch, in-context return.
- **Nothing becomes build-ready without pinned-tier sign-off:** the consultant's author-or-critique gate is what keeps the pinned tier load-bearing even though option generation ran at the session model. `docket-brainstorm` stops at the spec (the 0049 role contract's artifact/stop-point).
- **Degrade rule (ADR-0018 posture):** consultant undispatchable (agents not synced, harness without dispatch) ⇒ `docket-brainstorm` runs the whole flow inline at the session model WITH A PROMINENT WARNING — never a hard abort (availability is a per-machine property, not repo state).
- **`test_sync_agents.sh` invariant to respect:** its existing no-skill loops (`docket-auto-groom-critic`; `docket-rebase-resolver docket-integration-repair`) each assert `skills: docket-convention` — the consultant is NOT in those lists and MUST NOT be added to them. A NEW test block asserts the consultant injects neither a skill nor the convention.
- **Run the FULL suite as the gate** (ONE foreground call, `timeout 600000`) plus `sync-agents.sh --check`. Never background the suite.
- **The build-time ADR** (consultant-author pattern; refines-not-reverses ADR-0008; the no-convention deviation from ADR-0009) is recorded at review time via the `docket-adr` subagent — NOT a plan task.

---

### Task 1: The consultant wrapper + Cursor dispatch fragment + test block

**Files:**
- Create: `agents/docket-brainstorm-consultant.md`
- Create: `cursor-rules/dispatch/docket-brainstorm-consultant.md`
- Modify: `tests/test_sync_agents.sh` (add a consultant test block)

**Interfaces:**
- Consumes: the sync-agents.sh built-in-verbatim mechanism.
- Produces: a built-in wrapper `docket-brainstorm-consultant` with frontmatter `name`, `description`, `model: claude-opus-4-8`, `effort: xhigh`, and **NO `skills:` line**; a body that IS the compact brief (see below); a Cursor dispatch fragment so sync-agents emits no "no dispatch fragment" warning; a test block asserting the wrapper's shape.

- [ ] **Step 1: Read** `agents/docket-auto-groom-critic.md` (the closest sibling — no-skill wrapper) and `cursor-rules/dispatch/docket-auto-groom-critic.md` to match house style. Note the critic HAS `skills: [docket-convention]`; the consultant must NOT.
- [ ] **Step 2: Write the failing test block** in `tests/test_sync_agents.sh` (near the other no-skill wrapper blocks, e.g. after the rebase-resolver/integration-repair loop). Assert on `CONSULT="$AGENTS/docket-brainstorm-consultant.md"`:

```bash
# ---- the brainstorm consultant wrapper (wraps NO skill AND injects NO convention) ----
CONSULT="$AGENTS/docket-brainstorm-consultant.md"
assert "consultant: wrapper exists" '[ -f "$CONSULT" ]'
assert "consultant: name matches file" '[ "$(fm "$CONSULT" name)" = "docket-brainstorm-consultant" ]'
assert "consultant: has a description" '[ -n "$(fm "$CONSULT" description)" ]'
assert "consultant: model is claude-opus-4-8" '[ "$(fm "$CONSULT" model)" = "claude-opus-4-8" ]'
assert "consultant: effort is xhigh" '[ "$(fm "$CONSULT" effort)" = "xhigh" ]'
# Deliberate ADR-0009 deviation: injects NEITHER a wrapped skill NOR docket-convention.
assert "consultant: injects NO docket-convention" '! grep -Eq "^skills:.*docket-convention" "$CONSULT"'
assert "consultant: injects NO wrapped docket skill" '! grep -Eq "^skills:.*docket-(finalize-change|implement-next|auto-groom|status|adr|groom-next|new-change|brainstorm)\b" "$CONSULT"'
assert "consultant: body names the spec deliverable + assumptions requirement" 'grep -qi "spec" "$CONSULT" && grep -qi "assumption" "$CONSULT"'
```

- [ ] **Step 3: Run to verify fail** `bash tests/test_sync_agents.sh` → the consultant asserts are `NOT OK` (wrapper absent).
- [ ] **Step 4: Author `agents/docket-brainstorm-consultant.md`** — frontmatter with `name: docket-brainstorm-consultant`, a `description:` (one line: pinned design consultant that authors a spec or returns critique concerns; wraps no skill, injects no convention), `model: claude-opus-4-8`, `effort: xhigh`, and NO `skills:` line. Body = the **compact brief** (well under a page): you are a senior design consultant; you are handed a settled design + the stub/idea + neighbouring changes + relevant ADRs + LEARNINGS excerpts; return EXACTLY ONE of (a) an authored spec in markdown ready to write to the spec path, following the PM-altitude boundary (design detail belongs in the spec; intent/scope in the change) and INCLUDING AN EXPLICIT ASSUMPTIONS SECTION, or (b) critique concerns naming a hole the human must resolve first. You perform ZERO docket operations — no git, no status writes, no board, no file writes; you return prose in-context. You run without a human to prompt — never ask an interactive question; if you cannot proceed, return that as a critique concern.
- [ ] **Step 5: Author `cursor-rules/dispatch/docket-brainstorm-consultant.md`** — a `## docket-brainstorm-consultant — dispatch only` fragment matching the critic's fragment shape (so sync-agents emits no warning).
- [ ] **Step 6: Run to verify pass** `bash tests/test_sync_agents.sh` → all ok. Also run `bash sync-agents.sh --check` from the repo and confirm no "no dispatch fragment for docket-brainstorm-consultant" warning.
- [ ] **Step 7: Commit**

```bash
git add agents/docket-brainstorm-consultant.md cursor-rules/dispatch/docket-brainstorm-consultant.md tests/test_sync_agents.sh
git commit -m "feat(0056): docket-brainstorm-consultant wrapper (no skill, no convention) + dispatch fragment"
```

---

### Task 2: The `docket-brainstorm` skill (single-dispatch consultant flow + degrade)

**Files:**
- Create: `skills/docket-brainstorm/SKILL.md`
- Test: `tests/test_consultant_brainstorm.sh` (new)

**Interfaces:**
- Consumes: `agents/docket-brainstorm-consultant` (dispatch target), the 0049 role contract (stop at the spec).
- Produces: a skill implementing §3 of the spec: inline dialogue → single consultant dispatch (author or critique) → present + write spec + stop; degrade rule inline+warn.

- [ ] **Step 1: Write the failing test** `tests/test_consultant_brainstorm.sh` (follow the `tests/test_render_board.sh` harness shape). Assert:

```bash
SKILL="$REPO/skills/docket-brainstorm/SKILL.md"
assert "docket-brainstorm skill exists" '[ -f "$SKILL" ]'
assert "dispatches the pinned consultant" 'grep -q "docket-brainstorm-consultant" "$SKILL"'
assert "single dispatch — no SendMessage/continuation" '! grep -qi "SendMessage" "$SKILL" && ! grep -qi "continuation" "$SKILL"'
assert "author-or-critique gate documented" 'grep -qi "critique" "$SKILL" && grep -qi "author" "$SKILL"'
assert "in-context return contract" 'grep -qiE "in-context|in context" "$SKILL"'
assert "stops at the spec (0049 role contract)" 'grep -qi "stop" "$SKILL" && grep -qi "spec" "$SKILL"'
assert "degrade rule: inline + warn when undispatchable" 'grep -qiE "degrade|undispatchable|cannot be dispatched" "$SKILL" && grep -qi "warn" "$SKILL"'
assert "respects ADR-0006 — no simulated human / real dialogue inline" 'grep -qiE "real human|no.{0,4}simulat|inline" "$SKILL"'
assert "Convention load-first block present" 'grep -qF "## Convention (load first — blocking)" "$SKILL" || grep -qi "docket-convention" "$SKILL"'
```
Run → fail (skill absent).
- [ ] **Step 2: Author `skills/docket-brainstorm/SKILL.md`** — frontmatter `name: docket-brainstorm`, a `description:` (docket-owned brainstorm role implementing the single-dispatch consultant-author flow; bindable via `skills: brainstorm:`; invoked by docket-new-change/docket-groom-next). Body per spec §3:
  - A short overview + `## Convention (load first — blocking)` block (invoke docket-convention unless loaded — it is, since the caller's Step 0 loaded it).
  - **Step 1 — dialogue (inline, real):** the parent explores with the human directly, one question at a time, generating approaches/trade-offs at the session model. No relay, no auto-answerer (ADR-0006).
  - **Step 2 — dispatch (author or critique):** once settled, dispatch `docket-brainstorm-consultant` (foreground, in-context return, run at the model/effort its wrapper resolves — NO model literal in the body) with the settled design, the stub/idea, neighbouring changes, relevant ADRs, `LEARNINGS.md` excerpts, and the compact brief. It returns exactly one of an authored spec OR critique concerns; on concerns, take them back to the human, resolve in dialogue, re-dispatch.
  - **Step 3 — present + write:** show the authored spec to the human; change requests loop as further dispatch rounds; on approval, write the spec to the configured spec path and STOP (the 0049 role artifact/stop-point). Do NOT continue to writing-plans.
  - **Degrade rule (ADR-0018):** if the consultant cannot be dispatched, run the whole flow inline at the session model WITH A PROMINENT WARNING — no worse than today; never a hard abort.
  - Use wrapper-resolved-tier phrasing (no `opus`/`xhigh` literal in the body).
- [ ] **Step 3: Run to verify pass** `bash tests/test_consultant_brainstorm.sh` → all ok. Confirm `link-skills.sh`'s glob will pick it up (no registry edit needed; do not run link-skills against real HOME in the test).
- [ ] **Step 4: Commit** `git add skills/docket-brainstorm/SKILL.md tests/test_consultant_brainstorm.sh && git commit -m "feat(0056): docket-brainstorm skill — single-dispatch consultant flow + degrade rule"`.

---

### Task 3: Verbal opt-in discoverability notes in the two interactive skills

**Files:**
- Modify: `skills/docket-new-change/SKILL.md`, `skills/docket-groom-next/SKILL.md`
- Test: `tests/test_consultant_brainstorm.sh` (extend)

**Interfaces:**
- Consumes: the `docket-brainstorm` skill.
- Produces: a one-line note in each skill's brainstorm step that a human asking for a consultant-written spec makes the skill invoke `docket-brainstorm` for that run regardless of the resolved `$SKILL_BRAINSTORM` (human steering of an interactive session always wins).

- [ ] **Step 1: Extend the test** with:

```bash
NC="$REPO/skills/docket-new-change/SKILL.md"; GN="$REPO/skills/docket-groom-next/SKILL.md"
assert "new-change notes the consultant verbal opt-in" 'grep -q "docket-brainstorm" "$NC"'
assert "groom-next notes the consultant verbal opt-in" 'grep -q "docket-brainstorm" "$GN"'
```
Run → fail.
- [ ] **Step 2: Add the one-line note** to `docket-new-change`'s Brainstorm step (step 2) and `docket-groom-next`'s brainstorm step: "If the human asks for a consultant-written spec, invoke `docket-brainstorm` for this run regardless of `$SKILL_BRAINSTORM` — human steering of an interactive session always wins (see the README's consultant-brainstorm section)." Keep behavior-neutral otherwise; do NOT alter the resolved-`$SKILL_BRAINSTORM` default path. Confirm the notes don't break `test_docket_config.sh` (`/docket-config.sh` still present) or other sentinels — run those two skills' relevant tests.
- [ ] **Step 3: Run** `bash tests/test_consultant_brainstorm.sh` → ok; spot-run `bash tests/test_docket_config.sh` and `bash tests/test_composition_wiring.sh` → no new failures.
- [ ] **Step 4: Commit** `git add skills/docket-new-change/SKILL.md skills/docket-groom-next/SKILL.md tests/test_consultant_brainstorm.sh && git commit -m "feat(0056): verbal consultant opt-in note in docket-new-change + docket-groom-next"`.

---

### Task 4: README prominent section + capture-then-groom guidance

**Files:**
- Modify: `README.md`
- Test: `tests/test_consultant_brainstorm.sh` (extend)

**Interfaces:**
- Consumes: the skill + both activation channels.
- Produces: a prominent top-level README section documenting the off-by-default status, both opt-in channels (verbal + `skills: brainstorm: docket-brainstorm`), and the capture-then-groom guidance for whole-brainstorm model control.

- [ ] **Step 1: Extend the test:**

```bash
RM="$REPO/README.md"
assert "README documents the consultant brainstorm" 'grep -qi "consultant" "$RM" && grep -q "docket-brainstorm" "$RM"'
assert "README states off-by-default" 'grep -qiE "off by default|opt-in" "$RM"'
assert "README documents capture-then-groom" 'grep -qiE "capture-then-groom|capture .* groom" "$RM"'
assert "README shows the durable binding" 'grep -qF "brainstorm: docket-brainstorm" "$RM"'
```
Run → fail.
- [ ] **Step 2: Add a prominent top-level `##` section** to `README.md` (e.g. after "Why docket" or within the workflow-roles area, but as its own discoverable heading — NOT a buried footnote): "Consultant-authored brainstorm (opt-in)". Cover: what it is (pinned design consultant authors the spec while the dialogue stays inline with you), off by default, the two opt-in channels (verbal when running docket-new-change/docket-groom-next; durable `skills: brainstorm: docket-brainstorm`), the degrade behavior, and the **capture-then-groom** pattern for running an entire brainstorm at a chosen model (stub via docket-new-change in any session, then run docket-groom-next from a session set to the desired model — no new machinery). Keep it accurate to the skill.
- [ ] **Step 3: Run** `bash tests/test_consultant_brainstorm.sh` → ok.
- [ ] **Step 4: Commit** `git add README.md tests/test_consultant_brainstorm.sh && git commit -m "docs(0056): README — prominent consultant-brainstorm section + capture-then-groom"`.

---

### Task 5: Full-suite verification + sync-agents --check + read-only smoke

**Files:** none (verification only, unless a fix is needed).

**Interfaces:**
- Consumes: the whole change.
- Produces: evidence the suite is green, sync-agents --check passes, and the new wrapper generates cleanly.

- [ ] **Step 1: `sync-agents.sh --check`** from the repo — confirm rc 0 (or advisory-only), no error about the consultant wrapper, and that a generated pass would include `docket-brainstorm-consultant` with opus/xhigh. Also verify a per-repo/global `agents: { brainstorm-consultant: { model, effort } }` override would resolve (the config key auto-discovers via the glob — spot-check by reading how another agent key resolves; no need to mutate real config).
- [ ] **Step 2: Read-only smoke** — `eval "$("${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket-config.sh --export)"` and confirm `SKILL_BRAINSTORM` resolves (default `superpowers:brainstorming`; unchanged). Confirm `skills: brainstorm: docket-brainstorm` would pass through (0049 passthrough — no validation).
- [ ] **Step 3: FULL SUITE, ONE foreground call, timeout 600000:**

```bash
for t in tests/test_*.sh; do out="$(bash "$t" 2>&1)"; n=$(grep -c "^NOT OK" <<<"$out"); [ "$n" -gt 0 ] && echo "FAIL $(basename "$t") ($n)"; done; echo "suite done"
```
Zero `FAIL`. A failure outside the new tests = a broken invariant (e.g. a broad "all wrappers inject convention" assertion, or a skill-enumeration test that must learn about docket-brainstorm) — fix it: if a test legitimately must account for the new skill/wrapper, update it minimally with justification; if the consultant broke an invariant it shouldn't, fix the wrapper/skill.
- [ ] **Step 4: Commit** (only if a fix was needed) `git add -A && git commit -m "test(0056): full-suite green + sync-agents --check clean"`.

---

## Notes for the implementer

- The consultant wrapper's "no `skills:` line" is the whole trick — do NOT add `skills: [docket-convention]` to it (that's the critic's pattern; the consultant deliberately deviates). The compact brief in the BODY replaces the convention.
- No model/effort tier literal in the `docket-brainstorm` SKILL body's dispatch clause (the #0017 guard idiom) — say "at the model/effort its wrapper resolves". The tier literal lives ONLY in the wrapper frontmatter.
- This is largely a docs/skill/wrapper change — no script logic changes. The read-only Step-0 smoke + sync-agents --check + full suite are the acceptance gates.
- The build-time ADR is recorded at review time via the `docket-adr` subagent (relates_to 8, 9, 18; refines-not-reverses ADR-0008; documents the no-convention deviation from ADR-0009) — not a task here.
- Run the full suite in ONE foreground Bash call with `timeout 600000`.
