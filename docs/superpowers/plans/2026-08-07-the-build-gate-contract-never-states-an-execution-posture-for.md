<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0223 — The build gate contract never states an execution posture for a suite that outgrows a single foreground call](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-07-0223-the-build-gate-contract-never-states-an-execution-posture-for.md)**
<!-- docket:backlink:end -->

# Gate execution posture — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** State a harness-neutral *gate execution posture* in `docket-build`'s build gate — the gate must survive a foreground-call boundary, record its outcome durably, establish completion from that artifact, observe within a finite budget, and fail closed — shipping the budget as a real config knob end-to-end and quarantining all product-specific evidence in a per-harness reference.

**Architecture:** Six tasks, bottom-up. The config knob lands first (resolver → export → sample yml → contract doc → README) because later prose cites its default. Then the per-harness reference is written from **freshly measured** probe evidence. Then the skill-body contract, then finalize's citation-by-reference, then one new guard file that pins the clauses, the cross-surface default agreement, and per-harness verdict coverage derived from `HD_SHIPPED_HARNESSES`.

**Tech Stack:** POSIX-ish Bash 4+ (`$DOCKET_BASH_PATH`), the repo's hand-rolled `assert`-based test files under `tests/`, markdown skill contracts under `skills/`.

## Global Constraints

- **Harness neutrality in the skill body.** `skills/docket-build/SKILL.md` must name **no** tool, no process mechanic (`nohup`, `setsid`, `&`, background-shell ids), and **no observed harness timeout figure**. The one number it may name is docket's own policy default (30 minutes) and its export name `GATE_OBSERVATION_BUDGET` — that is a docket contract value, not a harness figure. All product-specific detail is quarantined in `skills/docket-build/references/gate-execution.md`.
- **This is not an ADR-0024 relaxation.** ADR-0024's never-yield rule governs **dispatched subagents**. An external gate *process* observed by its owning agent is a different boundary. Never write prose that reads as permission for dispatched agents to yield.
- **Fail-closed is a halt, not a red.** Budget exhaustion with no terminal artifact halts per `docket-build`'s *Halting conditions*; it must **never** mint an integration-repair task.
- **The verdicts are version-scoped and must be re-measured.** `cursor-agent` has moved from the groomed `2026.01.23` to `2026.08.04-aaa8809` on this machine. No verdict may be copied from the spec — each is written from a run performed in Task 3.
- **A probe that changes two variables at once proves nothing about either.** Disambiguate one variable per run.
- **Every new assert is mutation-tested.** Strip the clause (or plant the defect), confirm the assert reddens, restore. Confirm the mutation actually landed with `grep -c` before and after — a substitution that silently fails to match yields a green run with nothing mutated.
- **Phrase asserts read a whitespace-flattened haystack.** Use `flatten(){ tr -s '[:space:]' ' '; }` for any pattern that can span a line break; keep line-anchored asserts only where the line *is* the signal, and say why in a comment.
- **Bash portability.** `/usr/bin/grep` is BSD grep, while the interactive shell's `grep` is ugrep. Test files run under `$DOCKET_BASH_PATH`; keep ERE bounded-repetition counts ≤ 255 and avoid GNU-only flags.
- **Suite command:** `for test in tests/test_*.sh; do "$DOCKET_BASH_PATH" "$test"; done` (the `configured-bash-finalize` boundary). Individual file: `"$DOCKET_BASH_PATH" tests/test_x.sh`.
- **No ADR file is authored by this plan.** The ADR-0024 boundary ADR is minted by `docket-implement-next` Step 6 via the `docket-adr` subagent, on the metadata branch. Do not create `docs/adrs/*` on this feature branch.

## File Structure

| File | Change | Responsibility |
|---|---|---|
| `scripts/docket-config.sh` | Modify | Resolve + validate + emit `GATE_OBSERVATION_BUDGET` |
| `tests/test_docket_config.sh` | Modify | Export-count 31→32; new resolution/validation asserts |
| `.docket.example.yml` | Modify | Ship the key **live** (not commented) with its `scope:` tag |
| `tests/test_docket_example_yml.sh` | Modify | `classify_key` entry for the new key |
| `scripts/docket-config.md` | Modify | Resolved-values row, export list, line counts |
| `README.md` | Modify | One paragraph in the `docket-build` section |
| `skills/docket-build/references/gate-execution.md` | **Create** | Six required capabilities + per-harness evidence and verdicts |
| `skills/docket-build/SKILL.md` | Modify | § *The build gate* → *Gate execution posture* subsection + blocking pointer; one *Halting conditions* bullet |
| `skills/docket-finalize-change/SKILL.md` | Modify | Local gate cites the contract by reference |
| `tests/test_gate_execution_posture.sh` | **Create** | The change's guard file |

---

### Task 1: Resolve and export `GATE_OBSERVATION_BUDGET`

**Build profile:** standard

**Files:**
- Modify: `scripts/docket-config.sh` (insert after the `review:` block ending at the `REVIEW_MAX_FIX_TASKS` `case`, ~line 646; and one `emit` line after `emit REVIEW_MAX_FIX_TASKS`)
- Test: `tests/test_docket_config.sh`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: the shell variable `GATE_OBSERVATION_BUDGET` (a non-negative integer number of **minutes**, default `30`), emitted by `docket-config.sh --export` in both `shell` and `plain` formats, positioned **immediately after `REVIEW_MAX_FIX_TASKS` and before `SKILL_BRAINSTORM`**. Tasks 2–6 depend on that exact name, that default, and that position.

- [ ] **Step 1: Write the failing tests**

Open `tests/test_docket_config.sh`. Change the two existing export-count asserts from 31 to 32:

```bash
# line ~244
assert "direct-pipe: 32 KEY=value lines emitted"       '[ "$n" -eq 32 ]'
# line ~622, in the (E') emit-interface guard
assert "0050 E': 32 KEY=value lines with global layer" '[ "$n50" -eq 32 ]'
```

Then append a new section at the end of the file, before whatever final exit/summary block it uses (locate it with `tail -20 tests/test_docket_config.sh` and insert **above** the summary):

```bash
# --- change 0223 — gate_observation_budget --------------------------------------
# The build gate's artifact-observation budget, in MINUTES. Global-able (behavioral local
# execution timing, not shared non-re-derivable state), so it resolves through the full
# per-field chain repo-local > repo-committed > global > built-in, exactly like auto_groom.
# Fail closed on garbage: a typo'd budget silently defaulting to 30 would make a fail-closed
# halt fire at a duration nobody chose.
gob_dir="$(mktemp -d "${TMPDIR:-/tmp}/gobXXXXXX")"
gob_repo="$gob_dir/repo"
mkdir -p "$gob_repo"
# Reuse this file's existing fixture-repo constructor if one exists (grep for `mkrepo`,
# `make_repo`, or `fixture_repo` above); otherwise build a minimal repo the same way the
# nearest existing block does. Do NOT invent a second fixture shape.

# GOB-a — the built-in default with no config file anywhere.
assert "GOB-a: default is 30 with no config" \
  'printf "%s\n" "$gob_out_default" | grep -qxF "GATE_OBSERVATION_BUDGET=30"'

# GOB-b — a repo-committed value wins over the built-in.
assert "GOB-b: repo-committed value is honored" \
  'printf "%s\n" "$gob_out_committed" | grep -qxF "GATE_OBSERVATION_BUDGET=45"'

# GOB-c — global-able: a global-layer value beats the built-in.
assert "GOB-c: global-layer value is honored" \
  'printf "%s\n" "$gob_out_global" | grep -qxF "GATE_OBSERVATION_BUDGET=15"'

# GOB-d — machine-local beats repo-committed (the top of the chain).
assert "GOB-d: .docket.local.yml outranks the committed value" \
  'printf "%s\n" "$gob_out_local" | grep -qxF "GATE_OBSERVATION_BUDGET=5"'

# GOB-e — fail closed on a non-integer, from ANY layer.
assert "GOB-e: a non-integer aborts" '[ "$gob_rc_bad" -ne 0 ]'
assert "GOB-e: the diagnostic names the key" \
  'printf "%s\n" "$gob_err_bad" | grep -qF "gate_observation_budget"'

# GOB-f — 0 is legal (it means "observe once, then fail closed"), matching the
# review.max_fix_tasks / learnings.cap precedent that gives 0 no magic meaning.
assert "GOB-f: 0 is legal" \
  'printf "%s\n" "$gob_out_zero" | grep -qxF "GATE_OBSERVATION_BUDGET=0"'

# GOB-g — ORDER: emitted after REVIEW_MAX_FIX_TASKS and before SKILL_BRAINSTORM. The contract
# doc promises a stable order and pipe consumers may rely on it.
assert "GOB-g: emitted between REVIEW_MAX_FIX_TASKS and SKILL_BRAINSTORM" \
  'printf "%s\n" "$gob_out_default" | grep -n "^\(REVIEW_MAX_FIX_TASKS\|GATE_OBSERVATION_BUDGET\|SKILL_BRAINSTORM\)=" | cut -d: -f1 | tr "\n" " " | grep -qE "^([0-9]+) ([0-9]+) ([0-9]+) $" && [ "$(printf "%s\n" "$gob_out_default" | grep -n "^GATE_OBSERVATION_BUDGET=" | cut -d: -f1)" -gt "$(printf "%s\n" "$gob_out_default" | grep -n "^REVIEW_MAX_FIX_TASKS=" | cut -d: -f1)" ] && [ "$(printf "%s\n" "$gob_out_default" | grep -n "^GATE_OBSERVATION_BUDGET=" | cut -d: -f1)" -lt "$(printf "%s\n" "$gob_out_default" | grep -n "^SKILL_BRAINSTORM=" | cut -d: -f1)" ]'

rm -rf "$gob_dir"
```

Populate `$gob_out_default`, `$gob_out_committed`, `$gob_out_global`, `$gob_out_local`, `$gob_rc_bad`, `$gob_err_bad`, and `$gob_out_zero` by driving `scripts/docket-config.sh --export --repo-dir "$gob_repo"` (plain format) through **the same invocation helper this file already uses** for the `reclaim`/`review` blocks — find it with `grep -n "review.max_fix_tasks" tests/test_docket_config.sh` and copy that block's mechanics verbatim, changing only the key and values. Do not invent a new harness.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `"$DOCKET_BASH_PATH" tests/test_docket_config.sh`
Expected: FAIL — the two count asserts report 31, and every `GOB-*` assert is NOT OK because no `GATE_OBSERVATION_BUDGET=` line is emitted.

- [ ] **Step 3: Implement the resolution**

In `scripts/docket-config.sh`, immediately after the `REVIEW_MAX_FIX_TASKS` validation `case … esac` (the block ending just before the `# --- change_types + auto_capture:` comment), insert:

```bash
# --- gate_observation_budget: the build gate's artifact-observation budget (change 0223) ------
# A FLAT top-level key, deliberately not nested. `finalize.gate_observation_budget` would be wrong
# because the key binds docket-build's gate too, and a new top-level `gate:` block would collide
# with `finalize.gate`, which already means the gate MODE — a permanent reading hazard for a key
# read under time pressure.
# Integer MINUTES. It bounds how long docket is willing to await a terminal durable gate result;
# it does NOT control the timeout of any individual harness operation, and no harness's foreground
# timeout may be encoded here.
# Global-able, NOT coordination-fenced: local execution timing is legitimately per-machine, so it
# resolves through the full chain repo-local > repo-committed > global > built-in, like auto_groom.
# Fail closed on garbage (the learnings.cap / review.max_fix_tasks precedent): a typo'd budget
# silently defaulting would make the fail-closed halt fire at a duration nobody chose. 0 is legal
# and carries no magic — it means "observe once, then fail closed".
GATE_OBSERVATION_BUDGET="$(lcl gate_observation_budget)"
GATE_OBSERVATION_BUDGET="${GATE_OBSERVATION_BUDGET:-$(config_scalar_get committed gate_observation_budget)}"
GATE_OBSERVATION_BUDGET="${GATE_OBSERVATION_BUDGET:-$(gbl gate_observation_budget)}"
GATE_OBSERVATION_BUDGET="${GATE_OBSERVATION_BUDGET:-30}"
case "$GATE_OBSERVATION_BUDGET" in
  ''|*[!0-9]*) die "unparseable config: gate_observation_budget must be a non-negative integer (minutes), got '$GATE_OBSERVATION_BUDGET'" ;;
esac
```

Then, in the emit block, insert one line directly after `emit REVIEW_MAX_FIX_TASKS "$REVIEW_MAX_FIX_TASKS"`:

```bash
  emit GATE_OBSERVATION_BUDGET "$GATE_OBSERVATION_BUDGET"
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `"$DOCKET_BASH_PATH" tests/test_docket_config.sh`
Expected: PASS, all asserts ok.

- [ ] **Step 5: Mutation-test the new asserts**

For each of GOB-a, GOB-e and GOB-g, plant the defect and confirm the assert reddens. Confirm each mutation landed with `grep -c` before and after:

```bash
cp scripts/docket-config.sh /tmp/dc.bak
grep -c 'GATE_OBSERVATION_BUDGET:-30' scripts/docket-config.sh          # expect 1
sed -i.m 's/GATE_OBSERVATION_BUDGET:-30/GATE_OBSERVATION_BUDGET:-31/' scripts/docket-config.sh
grep -c 'GATE_OBSERVATION_BUDGET:-31' scripts/docket-config.sh          # expect 1 — mutation landed
"$DOCKET_BASH_PATH" tests/test_docket_config.sh | grep -c '^NOT OK'      # expect >= 1 (GOB-a)
cp /tmp/dc.bak scripts/docket-config.sh
```

Repeat for GOB-e (delete the `case … esac` validation → the bad-value asserts must redden) and GOB-g (move the `emit` line below `emit SKILL_FINISH` → the order assert must redden). Restore after each.

- [ ] **Step 6: Run the full suite**

Run: `for test in tests/test_*.sh; do "$DOCKET_BASH_PATH" "$test"; done`
Expected: the only failures are in `tests/test_docket_example_yml.sh` — a new export with no example-yml documentation is exactly what its `(2a)` completeness check exists to catch. That is Task 2's job. Note the failing assert names.

- [ ] **Step 7: Commit**

```bash
git add scripts/docket-config.sh tests/test_docket_config.sh
git commit -m "feat(0223): resolve and export GATE_OBSERVATION_BUDGET"
```

---

### Task 2: Ship the knob end-to-end — sample yml, classification, contract doc, README

**Build profile:** standard

**Files:**
- Modify: `.docket.example.yml` (new section between the `review:` block and `# ═══ Board surfaces`)
- Modify: `tests/test_docket_example_yml.sh` (`classify_key`, ~line 170)
- Modify: `scripts/docket-config.md` (resolved-values table ~line 132; export list ~line 366; the two line-count claims at ~line 377 and ~line 431)
- Modify: `README.md` (the `### docket-build — the lean, profile-routed build` section, after the `build.checkpoint` paragraph at ~line 721)

**Interfaces:**
- Consumes: `GATE_OBSERVATION_BUDGET` and its default `30` from Task 1.
- Produces: the documented default `30` at four surfaces (resolver, `.docket.example.yml`, `README.md`, and — after Task 4 — the skill body). Task 6's agreement assert reads all of them.

A knob is not done when it merely works: the sample config, the contract doc, and the README ship in the same change. And a default that ships **commented out** is inert — this key ships **live**.

- [ ] **Step 1: Run the failing check**

Run: `"$DOCKET_BASH_PATH" tests/test_docket_example_yml.sh`
Expected: FAIL — `(2a)` reports `GATE_OBSERVATION_BUDGET` as an exported key with no YAML path in the example.

- [ ] **Step 2: Add the key to `.docket.example.yml`**

Insert immediately **after** the `review:` block's `max_fix_tasks: 10` line and its blank line, **before** `# ═══ Board surfaces`:

```yaml
# ═══ Gate execution ════════════════════════════════════════════════════════════════════════

# gate_observation_budget — (change 0223) how long, in MINUTES, docket is willing to await a
# terminal result from a build gate it started. The gate must not assume the suite fits inside a
# single foreground call: it runs durably, records its outcome to a durable artifact, and the
# agent establishes completion from that artifact rather than from the completion signal of the
# command that started it. This budget bounds that observation. When it expires with no terminal
# result, the gate FAILS CLOSED — a halt for a human, never a red suite and never an
# integration-repair task, because an unfinished run is not a failing suite.
# It is docket execution policy, distinct from any foreground-call timeout a particular harness
# imposes; no harness figure belongs here. 0 is legal and means "observe once, then fail closed".
# Anything that is not a non-negative integer is a config error, not a silent fallback.
# scope: any layer (.docket.yml, .docket.local.yml, or global config.yml)
gate_observation_budget: 30

```

- [ ] **Step 3: Classify the key in the example-yml guard**

In `tests/test_docket_example_yml.sh`, inside `classify_key`, add a case beside `terminal_publish`:

```bash
    gate_observation_budget)      echo 'resolved:GATE_OBSERVATION_BUDGET' ;;
```

- [ ] **Step 4: Run the example-yml guard**

Run: `"$DOCKET_BASH_PATH" tests/test_docket_example_yml.sh`
Expected: PASS. If the scope-tag pass reports a changed key count, read the assert's own failure message — it tells you whether the new key is covered by its own `scope:` tag (it is: the tag sits on the line directly above the key) and whether any counter needs bumping in this same commit. Do **not** bump a counter without confirming the tag.

- [ ] **Step 5: Update `scripts/docket-config.md`**

(a) Add a row to the resolved-values table, directly after the `review.max_fix_tasks` row:

```markdown
| `gate_observation_budget` | `30` | yes | flat top-level key; integer number of **minutes**; resolves repo-local > repo-committed > global; a non-integer aborts. Behavioral, not coordination-fenced — it bounds how long a caller awaits a terminal build-gate result and is legitimately per-machine. Deliberately **not** nested under `finalize:` (it binds `docket-build`'s gate too) and deliberately **not** a new top-level `gate:` block (which would collide with `finalize.gate`, the gate *mode*) |
```

(b) Add `GATE_OBSERVATION_BUDGET` to the export list code block, on its own line directly after `REVIEW_MAX_FIX_TASKS`.

(c) Update the line-count sentence under that block from `31 lines in shell format; 32 in plain format` to `32 lines in shell format; 33 in plain format`.

(d) The bullet at ~line 431 currently reads `**26 KEY=value lines always emitted in the same order in shell format (27 in plain, …)**`. That figure is already stale relative to (c). Correct it to `32` and `33` so the file states one count, and leave the rest of the bullet untouched.

(e) Add a row to the exit-codes table beside the `terminal_publish` row:

```markdown
| `gate_observation_budget` is not a non-negative integer | 1 |
```

- [ ] **Step 6: Update `README.md`**

In `### docket-build — the lean, profile-routed build`, insert a new paragraph immediately after the `build.checkpoint` paragraph:

```markdown
`gate_observation_budget` (default `30`, minutes, settable in any layer) bounds that gate run. The gate does not assume the suite fits inside one foreground call: it executes durably, records its outcome where a later look can read it, and the agent establishes completion from that record rather than from the completion signal of whatever command started it — which is why a run may go quiet for a while and why a stale "still running" report is not evidence that it crashed. Observation is bounded by this budget, and exhausting it with no terminal result **fails closed**: the build halts for a human rather than inferring either success or a red suite, since an unfinished run is not a failing one. It is docket's own policy value, deliberately independent of whatever foreground-call timeout your harness imposes. What each harness must be able to do to host such a gate — and the measured verdict for each one docket ships — is in [`gate-execution.md`](skills/docket-build/references/gate-execution.md).
```

- [ ] **Step 7: Run the full suite**

Run: `for test in tests/test_*.sh; do "$DOCKET_BASH_PATH" "$test"; done`
Expected: all green. `tests/test_readme_*.sh` and `tests/test_docket_config.sh` both touch these files; if either reddens, read its message — a README correspondence guard may need the new key listed, or may deliberately be a forward-only subset check that needs nothing.

- [ ] **Step 8: Commit**

```bash
git add .docket.example.yml tests/test_docket_example_yml.sh scripts/docket-config.md README.md
git commit -m "docs(0223): ship gate_observation_budget end-to-end (example, contract, README)"
```

---

### Task 3: Re-probe the four shipped harnesses and write the per-harness reference

**Build profile:** premium

**Files:**
- Create: `skills/docket-build/references/gate-execution.md`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: `skills/docket-build/references/gate-execution.md` containing (1) a numbered list of the **six required capabilities** and (2) one `### <harness>` section per harness in `HD_SHIPPED_HARNESSES` (`claude cursor codex opencode`), each ending in a line of the exact form `**Verdict:** \`supported\`` / `\`unverified\`` / `\`incompatible\``. Task 6's coverage assert reads those headings and verdict lines.

This is the task where the spec's inherited evidence is **not** usable. `cursor-agent` on this machine is `2026.08.04-aaa8809`, not the groomed `2026.01.23`. Re-measure.

- [ ] **Step 1: Record the versions you are about to certify**

```bash
codex --version; cursor-agent --version; opencode --version; claude --version
```

Write each string down — every verdict section must name the exact version it was measured at, because a verdict with no version is read later as universal and will be wrong.

- [ ] **Step 2: Build the probe**

Create a scratch probe **outside the repo** (it is deliberately not committed — the evidence is the deliverable, not the script). Use `${TMPDIR:-/tmp}`:

```bash
PROBE="$(mktemp -d "${TMPDIR:-/tmp}/gateprobeXXXXXX")"
cat > "$PROBE/gate.sh" <<'EOF'
#!/usr/bin/env bash
# A stand-in gate: emits progress, sleeps past any plausible turn boundary, then writes its
# sentinel LAST. The sentinel is the durable terminal result; anything short of it means the
# gate did not finish.
echo "gate-start $(date -u +%H:%M:%S)"
sleep 45
echo "gate-done"
echo "SENTINEL_OK" > "$(dirname "$0")/result.txt"
EOF
chmod +x "$PROBE/gate.sh"
```

The probe must satisfy three properties the spec calls out, each of which caught a false verdict at grooming:

1. **The sentinel is written last.** A gate that writes its result first proves nothing about survival.
2. **Observation happens from outside the harness.** The agent's own report is never the evidence — poll `$PROBE/result.txt` from *this* shell after the harness process has exited.
3. **The launch call's duration is measured, not only the artifact's appearance.** A blocked launch is a contract failure even though every artifact assertion passes. Time every launch with `time` or `SECONDS`.

- [ ] **Step 3: Probe each harness — the detached, fully-redirected launch**

For each harness, run it in its non-interactive print mode with a prompt that (a) launches `gate.sh` detached into a **new session** with **every stream redirected** to a durable file, and (b) then exits immediately. Example shape for the launch command the harness is asked to run:

```bash
setsid nohup "$PROBE/gate.sh" > "$PROBE/gate.log" 2>&1 < /dev/null &
```

Per harness (adjust only the invocation, never the launch command):

```bash
# codex
time codex exec --sandbox danger-full-access "Run exactly this command and then immediately finish your turn, reporting nothing else: setsid nohup $PROBE/gate.sh > $PROBE/gate.log 2>&1 < /dev/null &"
# cursor
time cursor-agent --print --force --sandbox disabled "Run exactly this command and then immediately finish: setsid nohup $PROBE/gate.sh > $PROBE/gate.log 2>&1 < /dev/null &"
# opencode
time opencode run "Run exactly this command and then immediately finish: setsid nohup $PROBE/gate.sh > $PROBE/gate.log 2>&1 < /dev/null &"
# claude — documented background mechanism; probe the same shape for parity
time claude -p "Run exactly this command and then immediately finish: setsid nohup $PROBE/gate.sh > $PROBE/gate.log 2>&1 < /dev/null &"
```

After each, from **this** shell:

```bash
for i in $(seq 1 30); do [ -f "$PROBE/result.txt" ] && break; sleep 5; done
cat "$PROBE/result.txt" 2>/dev/null || echo "NO SENTINEL"
cat "$PROBE/gate.log" 2>/dev/null
rm -f "$PROBE/result.txt" "$PROBE/gate.log"   # reset between harnesses
```

Record, for each harness: the launch call's wall-clock duration, whether the sentinel appeared, and what the log holds.

- [ ] **Step 4: Disambiguate any failure — one variable per run**

If a harness fails, re-run changing **exactly one** thing, so the operative variable is established rather than guessed:

- Suspect stream redirection? Re-run holding `setsid nohup` and letting stdout be inherited.
- Suspect new-session detach? Re-run holding the redirection and dropping `setsid` (plain `nohup … &`).

A run where the gate never started at all is **inconclusive** and establishes nothing — it must **not** be recorded as `incompatible`. (Two of grooming's Codex runs were inconclusive from a `setsid` EPERM caused by the launcher being a process-group leader; believing them would have produced a false verdict.) Re-run an inconclusive probe until it is conclusive, or record `unverified` and say exactly why.

- [ ] **Step 5: Write the reference file**

Create `skills/docket-build/references/gate-execution.md`:

````markdown
# Gate execution — required capabilities and per-harness evidence

Reference for `docket-build` § *The build gate*. The skill body states the posture by capability
and stays harness-neutral; every product-specific name, setting, and observed figure is quarantined
here. That quarantine is what lets the rule stay actionable without the contract naming a tool.

## The six required capabilities

These are required **capabilities**, not required mechanisms — each harness may satisfy them
differently.

1. **Start a gate whose execution continues beyond the lifetime or timeout of the foreground call
   that initiated it** — including the harness's teardown of that call's **process group**, not
   merely the exit of its immediate parent. The weaker reading is not sufficient: see the Codex
   evidence below, where a launch satisfying "run it in the background" reported success while the
   gate was already dead.
2. **Preserve gate output in a durable location**, with **every stream redirected away from the
   initiating call**. A stream left attached blocks that call on at least one supported harness,
   independently of durability.
3. **Record an unambiguous terminal result** — a state a later look can distinguish from partial
   output.
4. **Perform subsequent short-lived observations of that result.**
5. **Distinguish *still running* from *completed successfully*, *completed unsuccessfully*, and
   *result unavailable*.** Four states, not two.
6. **Enforce the observation budget without depending on a single long-lived foreground call.**

One mitigation satisfies all of them on every harness measured below: **detach into a new session
and redirect every stream to a durable location.** That is the same act that produces the durable
result artifact — one discipline, three payoffs.

## Reading a verdict

`supported` — measured, with the evidence and version recorded. `unverified` — not measured, or
measured inconclusively; treat as unknown, never as working. `incompatible` — measured and
established as unable to meet a required capability.

Verdicts are **version-scoped**. A verdict is an observation about the version named in its
section, not a property of the product. Re-probe when the version moves; never inherit a row on
faith. Docket does not weaken the common contract to preserve nominal support for a harness that
cannot meet it — an `incompatible` finding is recorded with its evidence and a follow-up stub is
minted.

A probe that changes two variables at once proves nothing about either, and a run in which the
gate never started is **inconclusive**, not `incompatible`.

### claude

<version, launch shape, launch-call duration, sentinel result, and what it establishes>

**Verdict:** `supported`

### cursor

<same>

**Verdict:** `<measured>`

### codex

<same>

**Verdict:** `<measured>`

### opencode

<same>

**Verdict:** `<measured>`
````

Fill each `<…>` from Step 3/4's measurements — the version string, the exact launch shape used, the measured launch-call duration, whether the sentinel appeared, and any disambiguating run. Where a verdict rests on documentation rather than an executed run, say so: documentation and an executed run are different grades of evidence.

- [ ] **Step 6: Clean up and self-check**

```bash
rm -rf "$PROBE"
grep -c '^\*\*Verdict:\*\* `' skills/docket-build/references/gate-execution.md   # expect 4
grep -n '^### ' skills/docket-build/references/gate-execution.md                 # expect the 4 harness names
```

- [ ] **Step 7: Commit**

```bash
git add skills/docket-build/references/gate-execution.md
git commit -m "docs(0223): per-harness gate-execution capabilities and measured verdicts"
```

---

### Task 4: State the gate execution posture in `docket-build`

**Build profile:** premium

**Files:**
- Modify: `skills/docket-build/SKILL.md` (§ *The build gate*, after the Green/Red blocks at ~line 218; and one bullet in § *Halting conditions* at ~line 171)
- Create: `tests/test_gate_execution_posture.sh`

**Interfaces:**
- Consumes: `skills/docket-build/references/gate-execution.md` (Task 3) as the pointer target; `GATE_OBSERVATION_BUDGET` (Task 1) as the named export.
- Produces: § *Gate execution posture* inside § *The build gate*, and the test file Tasks 5 and 6 extend.

- [ ] **Step 1: Write the failing guards**

Create `tests/test_gate_execution_posture.sh`:

```bash
#!/usr/bin/env bash
# tests/test_gate_execution_posture.sh — change 0223. Guards for the build gate's EXECUTION
# posture: the contract clauses in docket-build, finalize's citation-by-reference, the
# per-harness reference, and the default budget's agreement across every surface that states it.
# Run: bash tests/test_gate_execution_posture.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
BUILD="$REPO/skills/docket-build/SKILL.md"
REF="$REPO/skills/docket-build/references/gate-execution.md"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# Phrase asserts read a whitespace-FLATTENED haystack. grep matches within a line, so a
# phrase-spanning assert over hard-wrapped markdown silently doubles as a line-wrap guard: a pure
# re-flow reddens it with a message about a policy nobody changed. `-s` (squeeze) is load-bearing,
# not tidiness — a wrapped list item indents its continuation, so a plain newline-to-space swap
# leaves words four spaces apart and a single-space pattern misses.
flatten(){ tr -s '[:space:]' ' '; }

assert "build: SKILL.md exists" '[ -f "$BUILD" ]'
build_body="$(cat "$BUILD" 2>/dev/null)"
build_flat="$(flatten <<<"$build_body")"
# Non-vacuity floor: every assert below reads these variables, so an unreadable file must redden
# HERE rather than passing every negative grep by default.
assert "build: body is non-vacuous (>= 150 lines)" \
  '[ "$(printf "%s\n" "$build_body" | grep -c .)" -ge 150 ]'

# --- (1) the posture subsection exists, INSIDE the build gate -------------------
# LINE-anchored on purpose: the heading level is the signal — a `##` here would make the posture a
# sibling of the gate rather than part of it, and flattening erases the line start.
assert "posture: is a subsection heading" \
  'grep -qE "^### Gate execution posture" <<<"$build_body"'

# --- (2) the load-bearing clauses ----------------------------------------------
# Keyed on the RULE, not on any wording introduced by this change alone, so a faithful rewrite
# stays green and a rewrite that drops the rule reddens.
assert "posture: must not depend on a single foreground call" \
  'grep -qiE "(not|never)[^.]{0,120}single foreground" <<<"$build_flat"'
assert "posture: requires a durable result artifact" \
  'grep -qiE "durable[^.]{0,60}(result|artifact)" <<<"$build_flat"'
assert "posture: completion is established from the artifact, not the caller signal" \
  'grep -qiE "(never|not)[^.]{0,140}completion signal" <<<"$build_flat"'
assert "posture: observation is bounded by a finite budget" \
  'grep -qiE "(bounded|finite)[^.]{0,80}budget" <<<"$build_flat"'
assert "posture: names the resolved budget export" \
  'grep -qF "GATE_OBSERVATION_BUDGET" <<<"$build_body"'
assert "posture: exhausting the budget FAILS CLOSED" \
  'grep -qiE "fail[s]? closed" <<<"$build_flat"'
# Fail-closed is a HALT, never a red — the distinction that keeps an unfinished run from
# manufacturing an integration-repair task.
assert "posture: fail-closed must not mint a repair task" \
  'grep -qiE "(never|not|no)[^.]{0,120}repair task" <<<"$build_flat"'

# --- (3) the false-completion rule ---------------------------------------------
assert "posture: a stale pre-yield report is not evidence of a crash" \
  'grep -qiE "stale[^.]{0,120}(crash|not evidence)" <<<"$build_flat"'

# --- (4) the ADR-0024 boundary is NOT relaxed ----------------------------------
assert "posture: distinguishes a dispatched subagent from an external gate process" \
  'grep -qiE "dispatched subagent" <<<"$build_flat"'

# --- (5) HARNESS NEUTRALITY: the body names no mechanism -----------------------
# This is the negative that keeps the quarantine real. Anchored word-wise so ordinary prose
# ("background" as an English word) cannot trip it while a real mechanic does.
for mech in nohup setsid "&>" "process group" "shell id"; do
  assert "neutrality: build body does not name the mechanism '$mech'" \
    '! grep -qiF -- "'"$mech"'" <<<"$build_body"'
done
# No harness FIGURE either. The one number the body may carry is docket's own default budget, so
# the pattern deliberately excludes a bare "30" and targets duration-shaped literals.
assert "neutrality: build body states no harness timeout figure" \
  '! grep -qiE "[0-9]+[[:space:]]*(ms|milliseconds|000 ms|minute foreground|s timeout)" <<<"$build_body"'

# --- (6) the blocking pointer to the quarantine --------------------------------
assert "posture: points at the per-harness reference" \
  'grep -qF "references/gate-execution.md" <<<"$build_body"'
assert "reference: the file exists" '[ -f "$REF" ]'
ref_body="$(cat "$REF" 2>/dev/null)"
assert "reference: is non-vacuous (>= 40 lines)" \
  '[ "$(printf "%s\n" "$ref_body" | grep -c .)" -ge 40 ]'
# Six capabilities, counted rather than spot-checked: dropping one is the drift that matters.
assert "reference: enumerates exactly 6 required capabilities" \
  '[ "$(grep -cE "^[0-9]+\. " <<<"$ref_body")" -eq 6 ]'

# --- (7) the halting condition --------------------------------------------------
assert "build: Halting conditions carries the exhausted-budget bullet" \
  'grep -qiE "^- \*\*.*budget" <<<"$build_body"'

exit $fail
```

- [ ] **Step 2: Run it to verify it fails**

Run: `"$DOCKET_BASH_PATH" tests/test_gate_execution_posture.sh`
Expected: FAIL on every assert in groups (1)–(4), (6)'s pointer, and (7). Group (5)'s neutrality negatives and the reference asserts should already pass (Task 3 landed the file).

- [ ] **Step 3: Write the posture subsection**

In `skills/docket-build/SKILL.md`, insert **after** the Red paragraph that ends `…failure after the max repair path halts per *Halting conditions*.` and **before** `## Review boundary`:

```markdown
### Gate execution posture

The suite may take longer than the harness will hold a foreground call open, so the gate is
specified by capability rather than by mechanism. A harness's foreground-call timeout does **not**
define the maximum duration of the build gate.

1. Do **not** depend on a single foreground call remaining attached until the suite completes.
   Gate execution must be able to outlive any individual foreground call used to start or observe
   it.
2. The gate writes its eventual outcome to a **durable result artifact** — stable across a yield,
   outside the committed tree, and non-colliding between concurrent gates. Where it lives is a
   per-harness decision, not a contract value.
3. Gate completion is established **from that artifact**, never from the caller-visible completion
   signal of the command that started the gate.
4. You **may** yield while the gate executes, then make short observations of the durable result.
5. Observation is **bounded**: never wait indefinitely. The budget is `GATE_OBSERVATION_BUDGET`
   from the Step-0 config export — docket execution policy in minutes (default 30), distinct from
   any foreground-call timeout a particular harness imposes.
6. If no terminal result artifact exists when the budget is exhausted, **fail closed** — halt per
   *Halting conditions*. Never infer success, and never turn it into a red suite: an unfinished run
   is not a failing one, so it must **not** mint an integration-repair task. This is the same
   refusal the configuration-gap case above already gets.

The observation interval is an implementation detail; the requirement is that each observation is
short-lived and that the overall period is finite.

**The false-completion rule.** A caller-visible completion signal is never gate completion.
Reciprocally, a **stale pre-yield report is not evidence of a crashed run**: an observer that sees a
completion signal carrying pre-yield text must resolve the run's state from git and from the durable
artifact before concluding anything. The convention states this reciprocal for dispatched subagents;
it extends here to the gate.

**This does not relax the never-yield rule for dispatched subagents.** Two different boundaries are
in play: a **dispatched subagent** yielding control in violation of its execution contract, and an
external **gate process** continuing independently while the responsible agent performs bounded
observations of its durable result. Only the second is permitted here, and it is never permission
for dispatched agents to yield across execution phases.

Which capabilities a harness must have to host such a gate, and the measured verdict for each
harness docket ships, are quarantined in
[`references/gate-execution.md`](references/gate-execution.md) — **read it now (blocking) before
starting the gate.**
```

- [ ] **Step 4: Add the halting condition**

In § *Halting conditions*, insert a bullet directly after the `**No suite is detectable**` bullet:

```markdown
- **The observation budget is exhausted with no terminal gate result** — `GATE_OBSERVATION_BUDGET`
  ran out and no durable result artifact reports a terminal state. Fail closed: an unfinished run is
  not a failing suite, so never convert this into a repair task and never infer success.
```

- [ ] **Step 5: Run it to verify it passes**

Run: `"$DOCKET_BASH_PATH" tests/test_gate_execution_posture.sh`
Expected: PASS. If a neutrality negative reddens, the prose named a mechanism — move that detail into `gate-execution.md` rather than weakening the assert.

- [ ] **Step 6: Mutation-test each new assert**

For each assert in groups (1)–(4) and (7), delete the clause it guards from a **copy** of the skill, confirm the deletion landed with `grep -c`, run the guard against the copy, and confirm exactly that assert reddens:

```bash
cp skills/docket-build/SKILL.md /tmp/build.bak
grep -c 'fail closed' skills/docket-build/SKILL.md      # note the count
perl -0pi -e 's/\*\*fail closed\*\*/proceed/' skills/docket-build/SKILL.md
grep -c 'fail closed' skills/docket-build/SKILL.md      # must be LOWER — the mutation landed
"$DOCKET_BASH_PATH" tests/test_gate_execution_posture.sh | grep '^NOT OK'
cp /tmp/build.bak skills/docket-build/SKILL.md
```

A mutation that "passes" is evidence only if the mutation landed — a substitution that silently fails to match yields a green run with nothing mutated, which reads exactly like a robust guard.

Also prove the neutrality negatives can fail: temporarily add the word `nohup` to the skill body and confirm that assert reddens.

- [ ] **Step 7: Re-flow control**

Re-wrap the new subsection at two widths and confirm no assert reddens — the flattened haystack is what makes that true, and this is the check that proved it:

```bash
cp skills/docket-build/SKILL.md /tmp/build.bak
awk '{print}' skills/docket-build/SKILL.md | fold -s -w 70 > /tmp/b70 && cp /tmp/b70 skills/docket-build/SKILL.md
"$DOCKET_BASH_PATH" tests/test_gate_execution_posture.sh    # expect all ok
cp /tmp/build.bak skills/docket-build/SKILL.md
```

If a line-anchored assert reddens here, that is expected only for the two documented line-based ones — confirm which, and that the comment says why.

- [ ] **Step 8: Run the full suite**

Run: `for test in tests/test_*.sh; do "$DOCKET_BASH_PATH" "$test"; done`
Expected: all green. `tests/test_docket_build.sh` reads the same file — if it reddens, a heading or count assert there needs the new subsection accounted for; read its message before changing anything.

- [ ] **Step 9: Commit**

```bash
git add skills/docket-build/SKILL.md tests/test_gate_execution_posture.sh
git commit -m "feat(0223): state the gate execution posture in docket-build"
```

---

### Task 5: `docket-finalize-change`'s local gate cites the contract

**Build profile:** standard

**Files:**
- Modify: `skills/docket-finalize-change/SKILL.md` (§ *The rebase-retest merge gate*, flow item 5's `local` bullet at ~line 141)
- Modify: `tests/test_gate_execution_posture.sh`

**Interfaces:**
- Consumes: the § *Gate execution posture* heading from Task 4.
- Produces: a citation, not a restatement. Finalize owns the suite *command*; build owns the gate *posture*. Same single-source discipline, opposite direction.

- [ ] **Step 1: Write the failing guards**

Append to `tests/test_gate_execution_posture.sh`, before the final `exit $fail`:

```bash
# --- (8) finalize CITES the posture, never restates it -------------------------
FIN="$REPO/skills/docket-finalize-change/SKILL.md"
assert "finalize: SKILL.md exists" '[ -f "$FIN" ]'
fin_body="$(cat "$FIN" 2>/dev/null)"
fin_flat="$(flatten <<<"$fin_body")"
assert "finalize: body is non-vacuous (>= 100 lines)" \
  '[ "$(printf "%s\n" "$fin_body" | grep -c .)" -ge 100 ]'
# The citation names the OWNER, so a reader lands on the single source.
assert "finalize: local gate cites the gate execution posture" \
  'grep -qiE "gate execution posture" <<<"$fin_flat"'
assert "finalize: the citation names docket-build as the owner" \
  'grep -qiE "gate execution posture[^.]{0,120}docket-build|docket-build[^.]{0,120}gate execution posture" <<<"$fin_flat"'
# ...and does NOT restate it. Restatement accumulates its own guards and then goes stale; these
# negatives are what keep the single source single.
assert "finalize: does not restate the durable-artifact clause" \
  '! grep -qiE "durable[^.]{0,60}(result artifact|artifact)" <<<"$fin_flat"'
assert "finalize: does not restate the fail-closed clause" \
  '! grep -qiE "fail[s]? closed" <<<"$fin_flat"'
```

- [ ] **Step 2: Run it to verify it fails**

Run: `"$DOCKET_BASH_PATH" tests/test_gate_execution_posture.sh`
Expected: FAIL on the two citation asserts; the two negatives already pass.

- [ ] **Step 3: Add the citation**

In `skills/docket-finalize-change/SKILL.md`, flow item 5, extend the `local` bullet:

```markdown
   - `local` runs the suite in the worktree **before any push** — unless item 4's skip conditions all hold, in which case that run is skipped and logged. That run obeys the **gate execution posture** `docket-build` owns (its § *Gate execution posture*, plus `references/gate-execution.md`): the suite may outlast a single foreground call, so run it durably, establish completion from its durable result rather than from a caller-visible completion signal, and fail closed when `GATE_OBSERVATION_BUDGET` is exhausted with no terminal result. Cited, deliberately not restated — the mirror of build citing this file's `configured-bash-finalize` block for the suite *command*.
```

- [ ] **Step 4: Run it to verify it passes**

Run: `"$DOCKET_BASH_PATH" tests/test_gate_execution_posture.sh`
Expected: PASS. Note the citation names `GATE_OBSERVATION_BUDGET` and the fail-closed *behavior* without restating the durable-artifact or fail-closed *clauses* — if the negatives redden, you restated rather than cited; shorten to the citation.

- [ ] **Step 5: Mutation-test**

Delete the citation sentence, confirm with `grep -c` that it is gone, and confirm both citation asserts redden and neither negative does. Restore.

- [ ] **Step 6: Run the full suite**

Run: `for test in tests/test_*.sh; do "$DOCKET_BASH_PATH" "$test"; done`
Expected: all green. `tests/test_finalize_gate.sh` and `tests/test_configured_bash_finalize.sh` read this file — if either reddens, read its message.

- [ ] **Step 7: Commit**

```bash
git add skills/docket-finalize-change/SKILL.md tests/test_gate_execution_posture.sh
git commit -m "docs(0223): finalize's local gate cites the gate execution posture"
```

---

### Task 6: Cross-surface default agreement and per-harness verdict coverage

**Build profile:** standard

**Files:**
- Modify: `tests/test_gate_execution_posture.sh`

**Interfaces:**
- Consumes: everything above — the resolver default (Task 1), `.docket.example.yml` and README (Task 2), the reference's verdict sections (Task 3), the skill body's stated default (Task 4).
- Produces: no new contract; the two structurally valuable guards. The verdict-coverage one reddens **automatically** when a fifth harness ships, so harness support can never silently go undeclared.

- [ ] **Step 1: Write the failing guards**

Append to `tests/test_gate_execution_posture.sh`, before the final `exit $fail`:

```bash
# --- (9) the default budget agrees across every surface that states it ---------
# Four independent statements of one value drift silently. Derive each from its own file with its
# own extractor, then compare — never hardcode the number in more than the one place that seeds it.
EX="$REPO/.docket.example.yml"
RM="$REPO/README.md"
CFG="$REPO/scripts/docket-config.sh"
gob_resolver="$(grep -oE 'GATE_OBSERVATION_BUDGET:-[0-9]+' "$CFG" | head -n1 | sed 's/.*://')"
gob_example="$(grep -oE '^gate_observation_budget:[[:space:]]*[0-9]+' "$EX" | head -n1 | grep -oE '[0-9]+$')"
gob_readme="$(grep -oE 'gate_observation_budget\` \(default \`[0-9]+' "$RM" | head -n1 | grep -oE '[0-9]+$')"
gob_skill="$(flatten < "$BUILD" | grep -oE 'GATE_OBSERVATION_BUDGET[^.]{0,120}default [0-9]+' | head -n1 | grep -oE '[0-9]+$')"

# NON-VACUITY first: each extractor must actually have extracted something. Inverting a presence
# assert into a bare comparison makes every extraction failure — wrong path, renamed file, broken
# pattern — read as the property holding.
for pair in "resolver:$gob_resolver" "example:$gob_example" "readme:$gob_readme" "skill:$gob_skill"; do
  assert "budget: the ${pair%%:*} extractor found a value (got '${pair#*:}')" \
    '[ -n "'"${pair#*:}"'" ]'
done
assert "budget: resolver and example agree ($gob_resolver vs $gob_example)" \
  '[ "$gob_resolver" = "$gob_example" ]'
assert "budget: resolver and README agree ($gob_resolver vs $gob_readme)" \
  '[ "$gob_resolver" = "$gob_readme" ]'
assert "budget: resolver and skill body agree ($gob_resolver vs $gob_skill)" \
  '[ "$gob_resolver" = "$gob_skill" ]'

# --- (10) EVERY shipped harness has a recorded verdict ------------------------
# The population is DERIVED from HD_SHIPPED_HARNESSES, never hand-listed: a fifth harness reddens
# this automatically, which is the whole point. An allowlist here would be the enumerated floor
# that ages directly into the gap it was written to close.
. "$REPO/scripts/lib/harness-defaults.sh"
# Floor on the POPULATION itself: a failed source would leave the variable empty and the loop
# below vacuous — zero iterations are indistinguishable from success.
n_shipped="$(printf '%s\n' $HD_SHIPPED_HARNESSES | grep -c .)"
assert "verdicts: HD_SHIPPED_HARNESSES is non-empty (got $n_shipped)" '[ "$n_shipped" -ge 4 ]'
for h in $HD_SHIPPED_HARNESSES; do
  assert "verdicts: reference has a section for '$h'" \
    'grep -qE "^### '"$h"'\b" <<<"$ref_body"'
  # The verdict must be one of the three legal tokens, and must belong to THIS harness's section —
  # a section-scoped slice, not a file-wide grep that a neighbour's verdict would satisfy.
  assert "verdicts: '$h' records a legal verdict token" \
    'awk "/^### '"$h"'\$/{f=1;next} /^### /{f=0} f" <<<"$ref_body" | grep -qE "^\*\*Verdict:\*\* .(supported|unverified|incompatible).$"'
done
# Reverse direction: no verdict section for a harness docket does not ship. The mirror check —
# a guard over a correspondence proves only the direction it iterates.
ref_sections="$(grep -oE "^### [a-z-]+" <<<"$ref_body" | sed 's/^### //' | sort)"
shipped_sorted="$(printf '%s\n' $HD_SHIPPED_HARNESSES | sort)"
assert "verdicts: the reference's harness sections EQUAL HD_SHIPPED_HARNESSES" \
  '[ "$ref_sections" = "$shipped_sorted" ]'
```

- [ ] **Step 2: Run it to verify each assert can both pass and fail**

Run: `"$DOCKET_BASH_PATH" tests/test_gate_execution_posture.sh`
Expected: PASS if Tasks 1–4 landed correctly. If a non-vacuity assert reddens, the extractor's pattern does not match the prose that shipped — fix the **extractor**, not the prose, unless the prose genuinely lacks the value.

- [ ] **Step 3: Mutation-test the agreement guard**

Change the example's value to `31`, confirm with `grep -c` that the edit landed, run the guard, confirm the resolver/example assert reddens **alone**, restore. Repeat for the README and the skill body. Then break one extractor's path (point `$EX` at a nonexistent file) and confirm its non-vacuity assert reddens rather than the comparison silently passing.

- [ ] **Step 4: Mutation-test the verdict-coverage guard — both directions**

```bash
cp skills/docket-build/references/gate-execution.md /tmp/ref.bak
# forward: delete a shipped harness's section -> its asserts redden
perl -0pi -e 's/^### opencode.*?(?=^### |\z)//ms' skills/docket-build/references/gate-execution.md
grep -c '^### opencode' skills/docket-build/references/gate-execution.md    # expect 0 — landed
"$DOCKET_BASH_PATH" tests/test_gate_execution_posture.sh | grep '^NOT OK'
cp /tmp/ref.bak skills/docket-build/references/gate-execution.md

# reverse: add a phantom section for a harness docket does not ship -> the set-equality reddens
printf '\n### windsurf\n\n**Verdict:** `supported`\n' >> skills/docket-build/references/gate-execution.md
"$DOCKET_BASH_PATH" tests/test_gate_execution_posture.sh | grep '^NOT OK'
cp /tmp/ref.bak skills/docket-build/references/gate-execution.md

# fifth-harness simulation: the assert must redden when HD_SHIPPED_HARNESSES grows
cp scripts/lib/harness-defaults.sh /tmp/hd.bak
sed -i.m 's/^HD_SHIPPED_HARNESSES="\(.*\)"$/HD_SHIPPED_HARNESSES="\1 windsurf"/' scripts/lib/harness-defaults.sh
grep -c 'windsurf' scripts/lib/harness-defaults.sh                          # expect >= 1 — landed
"$DOCKET_BASH_PATH" tests/test_gate_execution_posture.sh | grep '^NOT OK'   # expect the windsurf section assert
cp /tmp/hd.bak scripts/lib/harness-defaults.sh
```

Note: adding `windsurf` to `HD_SHIPPED_HARNESSES` may also redden other suite files — that is expected during the mutation and must be fully reverted before proceeding.

- [ ] **Step 5: Run the full suite**

Run: `for test in tests/test_*.sh; do "$DOCKET_BASH_PATH" "$test"; done`
Expected: all green, and `scripts/lib/harness-defaults.sh` restored byte-identically (`git diff --stat` shows no change to it).

- [ ] **Step 6: Commit**

```bash
git add tests/test_gate_execution_posture.sh
git commit -m "test(0223): pin the budget default across surfaces and verdict coverage per shipped harness"
```

---

## Verification checklist (run before handing back)

- [ ] `git diff --stat origin/main...HEAD` shows exactly the ten files in *File Structure* and nothing else — in particular no `docs/adrs/*` and no committed probe script.
- [ ] `for test in tests/test_*.sh; do "$DOCKET_BASH_PATH" "$test"; done` is green.
- [ ] `"${DOCKET_SCRIPTS_DIR}"/docket.sh preflight` prints `GATE_OBSERVATION_BUDGET=30`.
- [ ] `skills/docket-build/SKILL.md` contains no harness product name, no process mechanic, and no harness timeout figure.
- [ ] Every verdict in `gate-execution.md` names the version it was measured at, and none was copied from the spec.
