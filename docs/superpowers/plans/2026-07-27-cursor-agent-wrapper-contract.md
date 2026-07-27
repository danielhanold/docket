<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0135 — Generated Cursor wrappers violate Cursor's subagent contract, disabling skills and model effort](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0135-cursor-agent-wrapper-contract.md)**
<!-- docket:backlink:end -->

# Cursor Agent Wrapper Contract Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make docket's generated Cursor subagent wrappers conform to Cursor's own documented
subagent contract, so the model pin, the reasoning-effort pin, and the docket skills docket
advertises are genuinely honored instead of silently ignored.

**Architecture:** `sync-agents.sh` gains a named `emit_cursor_md()` emitter alongside the existing
`emit_codex_toml()`, and `emit_for_harness()` gains explicit `cursor)`/`claude)` branches with the
`*)` catch-all documented as an unverified gap. The Cursor wrapper emits only Cursor's documented
frontmatter fields, encodes reasoning effort inside the model value (`<model>[effort=<e>]`), and
replaces the inert `skills:` preload with a body preamble — exactly mirroring how the Codex emitter
already solves the same problem via `developer_instructions`. A new `scripts/runners/cursor.sh`
adapter mirrors `codex.sh` for delegating a whole agent run to `cursor-agent`.

**Tech Stack:** Bash 3.2-compatible shell (`sync-agents.sh`, `scripts/runners/*.sh`), awk/sed text
processing, hand-rolled `assert`-based test scripts under `tests/`.

## Global Constraints

Every task's requirements implicitly include this section.

- **Shell portability.** Target macOS's system Bash 3.2 and BSD userland. No `declare -A`, no
  `${var^^}`, no GNU-only `sed -i` (use `sed -i.bak` + `rm -f`), no `grep -P`.
- **Never `producer | early-exiting-consumer`** (`grep -q`, `head`, `head -n1`) under
  `set -o pipefail` — capture into a variable first, then `grep <<<"$var"`. (AGENTS.md)
- **`grep` for a pattern leading with `--`** must declare it: `grep -qF -- "<pat>"`. (AGENTS.md)
- **awk indent classes are `[^[:space:]]`**, never `[^ ]`. (AGENTS.md)
- **A guard is code: mutation-test it** — strip the thing it guards, watch it redden, restore.
  Every new assert in this plan carries an explicit mutation step. (AGENTS.md)
- **Key a guard on syntactic shape**, never an enumerated list of spellings. (AGENTS.md)
- **Cross-references anchor on a symbol name or a verbatim-quoted clause, never a line number.**
  `tests/test_comment_anchor_style.sh` enforces the filename-plus-line-number form. (AGENTS.md,
  ADR-0054)
- **ADR-0015 passthrough:** model IDs and effort tokens are **never validated or rewritten**. Docket
  keeps **no allowlist of Cursor model IDs and no allowlist of effort tokens**.
- **`log()` writes to stderr** (`sync-agents: <msg>` on fd 2). Emitters write the wrapper document to
  **stdout only** — a WARN must never leak into a generated file.
- **Emitter argument order is fixed by the existing call site:**
  `emit_<harness>() { # $1=src md  $2=model_override  $3=effort_override }`, invoked from
  `emit_for_harness()` as `emit_cursor_md "$1" "$3" "$4"`.
- **An empty override means "keep the built-in"** — identical in meaning to `emit()`'s args.
- **The ADR is NOT written on this branch.** ADRs live on the `docket` metadata branch and are
  authored by the `docket-adr` agent at review time. No task here creates `docs/adrs/*`.
- **The suite is the gate; there is no GitHub Actions CI.** Run the whole suite at the build gate,
  never only the tests a task enumerated.

---

## File Structure

| File | Status | Responsibility |
|---|---|---|
| `sync-agents.sh` | Modify | `emit_cursor_md()`; named `emit_for_harness()` branches; `REGISTERED_RUNNERS`; capability-worded auto-block fallback in `assemble_dispatch_rule()` |
| `tests/test_sync_agents_cursor.sh` | Create | Tier 1 hermetic Cursor contract tests (mirrors `test_sync_agents_codex.sh`) |
| `tests/test_sync_agents.sh` | Modify | Retire the four defect-encoding Cursor assertions; keep the Claude/Codex regression guards |
| `cursor-rules/dispatch.head.md` | Modify | Capability-worded dispatch instruction |
| `cursor-rules/dispatch/docket-*.md` (9) | Modify | Capability-worded instruction; call snippet demoted to a labelled illustration |
| `tests/test_cursor_dispatch_rule.sh` | Create | Guards the capability wording across head + all nine fragments, derived by glob |
| `scripts/runners/cursor.sh` | Create | The `cursor` runner adapter — preflight, prompt assembly, flag mapping, foreground exec, relay |
| `scripts/runners/cursor.md` | Create | Its contract (mechanically required by `test_script_contracts_coverage.sh`) |
| `tests/test_runner_cursor.sh` | Create | Adapter tests against the `CURSOR_BIN` mock seam |
| `skills/docket-convention/references/agent-layer.md` | Modify | Per-harness wrapper-shape table; correct uniform-shape prose |
| `README.md` | Modify | Register `cursor` in the Runner delegation section |
| `docs/cursor/validation.md` | Create | Tier 2 `cursor-agent` probe + Tier 3 human IDE checklist |
| `tests/test_cursor_contract_docs.sh` | Create | Guards the agent-layer table + the validation runbook's load-bearing evidence rule |

**Task order rationale.** Task 1 makes the Cursor wrapper shape change, which necessarily reddens
four existing assertions in `tests/test_sync_agents.sh` — so retiring those assertions is folded
**into** Task 1 rather than deferred, or Task 1 would end on a red suite. Tasks 2–5 are independent
of one another and each ends green on its own.

---

### Task 1: The Cursor emitter and the emitter split

**Files:**
- Modify: `sync-agents.sh` — `emit_for_harness()`, and a new `emit_cursor_md()` placed immediately
  after `emit_codex_toml()`
- Create: `tests/test_sync_agents_cursor.sh`
- Modify: `tests/test_sync_agents.sh` — the four assertions whose premise this task deletes

**Interfaces:**
- Consumes: `short_name()`, `agent_description()`, `log()` — all already defined in `sync-agents.sh`
  above the emitter block.
- Produces: `emit_cursor_md <src-md> <model-override> <effort-override>` → a Cursor wrapper document
  on stdout. Later tasks do not call it; Task 4's adapter reads the **built-in** `agents/*.md`
  sources, not generated Cursor wrappers.

**Target output.** For `agents/docket-status.md` (built-in `model: claude-haiku-4-5-20251001`,
`effort: medium`, `skills: [docket-status, docket-convention]`) with no overrides:

```markdown
---
name: docket-status
description: Use when you want to see or refresh the docket backlog — …
model: claude-haiku-4-5-20251001[effort=medium]
---

Before acting, load these docket skills from your Cursor skills directory: docket-status, docket-convention.

<the source wrapper's body, verbatim>
```

`readonly` and `is_background` are **not emitted** — their Cursor defaults are already right for
every docket agent (the agents commit and push, so not readonly; every docket dispatch is
foreground, so not background), and emitting them would assert a policy docket does not have.

**Model/effort encoding table** (this is the whole contract; copy it into the emitter as a comment):

| resolved model | resolved effort | emitted |
|---|---|---|
| `claude-opus-4-8` | `xhigh` | `model: claude-opus-4-8[effort=xhigh]` |
| `claude-opus-4-8` | unset or `auto` | `model: claude-opus-4-8` |
| unset or `inherit` | unset or `auto` | *(no `model:` line)* |
| unset or `inherit` | `xhigh` | *(no `model:` line)* **+ a generation-time WARN on stderr** |

- [ ] **Step 1: Write the failing contract tests**

Create `tests/test_sync_agents_cursor.sh`. It mirrors `tests/test_sync_agents_codex.sh`'s structure
(same `assert` helper, same `mk_*_repo` sandbox shape, same `rm -rf "$SBX"` discipline).

```bash
#!/usr/bin/env bash
# tests/test_sync_agents_cursor.sh — Cursor harness wrapper generation against Cursor's OWN
# documented subagent contract (change 0135). Cursor documents exactly five frontmatter fields
# (name, description, model, readonly, is_background) and encodes reasoning effort INSIDE the
# model value; it has no `effort:` field and no `skills:` preload. Before 0135 docket emitted a
# Claude-shaped wrapper here, so the model pin, the effort pin, and the skills were all silently
# ignored while docket reported them as honored.
# run: bash tests/test_sync_agents_cursor.sh
set -uo pipefail
unset XDG_CONFIG_HOME
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SYNC="$REPO/sync-agents.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# Single-line frontmatter scalar. Anchored to the FIRST --- block would be stricter, but these
# generated wrappers are emitter output with a known shape and no body prose using these keys.
fm(){ sed -n "s/^$2:[[:space:]]*//p" "$1" | head -n1 | sed 's/[[:space:]]*$//'; }
# Frontmatter = lines between the first two --- fences. Key-absence asserts MUST scope to it:
# a bare `grep -q '^effort:'` over the whole file would also match wrapper body prose.
front(){ awk '/^---[[:space:]]*$/{d++; next} d==1{print} d>=2{exit}' "$1"; }
has_fm_key(){ local f; f="$(front "$1")"; grep -qE "^$2[[:space:]]*:" <<<"$f"; }

mk_cursor_repo(){  # $1 = optional .docket.yml body (default: bare [claude, cursor] opt-in)
  SBX="$(mktemp -d)"
  git -C "$SBX" init --quiet
  git -C "$SBX" config user.email t@t.test
  git -C "$SBX" config user.name Test
  printf '%s' "${1:-$(printf 'agent_harnesses: [claude, cursor]\n')}" > "$SBX/.docket.yml"
  ( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
  C="$SBX/.cursor/agents/docket-status.md"
}

# --- the two fields Cursor does not have are GONE ------------------------------------------------
mk_cursor_repo
assert "cursor: wrapper generated"            '[ -f "$C" ]'
assert "cursor: full built-in set (9 files)"  '[ "$(find "$SBX/.cursor/agents" -name "docket-*.md" | wc -l | tr -d " ")" = "9" ]'
assert "cursor: emits NO standalone effort: key" '! has_fm_key "$C" effort'
assert "cursor: emits NO skills: preload key"    '! has_fm_key "$C" skills'
assert "cursor: emits name"                      '[ "$(fm "$C" name)" = "docket-status" ]'
assert "cursor: emits description from source"   '[ "$(fm "$C" description)" = "$(sed -n "/^description:/{s/^description:[[:space:]]*//;p;q;}" "$REPO/agents/docket-status.md")" ]'
assert "cursor: does NOT emit readonly"          '! has_fm_key "$C" readonly'
assert "cursor: does NOT emit is_background"     '! has_fm_key "$C" is_background'

# --- effort rides INSIDE the model value ---------------------------------------------------------
assert "cursor: model carries bracket-encoded effort" \
  '[ "$(fm "$C" model)" = "claude-haiku-4-5-20251001[effort=medium]" ]'

# --- the body preamble replaces the inert skills: preload ----------------------------------------
assert "cursor: body preamble names the agent's own skill" 'grep -qF "docket-status" "$C"'
assert "cursor: body preamble names docket-convention"     'grep -qF "docket-convention" "$C"'
assert "cursor: preamble tells the child to LOAD them"     'grep -qiF "load these docket skills" "$C"'
assert "cursor: wrapper body survives verbatim"            'grep -qi "refresh docket state" "$C"'

# --- the claude and codex sides are untouched (emitter-split regression guard) --------------------
assert "cursor split: claude wrapper still byte-identical to its source" \
  'diff -q "$REPO/agents/docket-status.md" "$SBX/.claude/agents/docket-status.md" >/dev/null'
assert "cursor split: claude wrapper still HAS effort:" 'has_fm_key "$SBX/.claude/agents/docket-status.md" effort'
assert "cursor split: claude wrapper still HAS skills:" 'has_fm_key "$SBX/.claude/agents/docket-status.md" skills'
rm -rf "$SBX"

# --- bare model when no effort resolves (effort: auto drops the effort) --------------------------
mk_cursor_repo "$(printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: gpt-5.5-medium-fast, effort: auto }\n')"
assert "cursor: effort auto => BARE model, no bracket" '[ "$(fm "$C" model)" = "gpt-5.5-medium-fast" ]'
assert "cursor: effort auto => still no effort: key"   '! has_fm_key "$C" effort'
rm -rf "$SBX"

# --- an arbitrary non-Claude id and an arbitrary effort token pass through VERBATIM --------------
# ADR-0015 + ADR-0059: docket holds no allowlist of Cursor model IDs or effort tokens. A committed
# table of a vendor's internals goes stale silently, and a stale entry produces a FALSE NEGATIVE
# that reads as a successful degrade. `zzz-not-a-real-effort` is the discriminating input: any
# validation layer would reject or rewrite it, and this assert would redden.
mk_cursor_repo "$(printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: gpt-5.5-medium-fast, effort: zzz-not-a-real-effort }\n')"
assert "cursor: unknown model+effort pass through verbatim (no allowlist)" \
  '[ "$(fm "$C" model)" = "gpt-5.5-medium-fast[effort=zzz-not-a-real-effort]" ]'
rm -rf "$SBX"

# --- inherit + no effort => NO model line at all --------------------------------------------------
mk_cursor_repo "$(printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: inherit, effort: auto }\n')"
assert "cursor: model inherit => no model: line" '! has_fm_key "$C" model'
rm -rf "$SBX"

# --- inherit + an effort => no model line, and a LOUD warn ----------------------------------------
# Effort has nowhere to attach without a model, so the pin is dropped — and a dropped pin must
# never be silent, since silently-dropped pins are the defect this whole change exists to fix.
SBX="$(mktemp -d)"
git -C "$SBX" init --quiet; git -C "$SBX" config user.email t@t.test; git -C "$SBX" config user.name Test
printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: inherit, effort: xhigh }\n' > "$SBX/.docket.yml"
warn_out="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" 2>&1 >/dev/null )"
C="$SBX/.cursor/agents/docket-status.md"
assert "cursor: inherit+effort => no model: line"      '! has_fm_key "$C" model'
assert "cursor: inherit+effort => WARN emitted"        'grep -qi "effort" <<<"$warn_out" && grep -qi "dropped" <<<"$warn_out"'
assert "cursor: WARN names the cursor harness + agent"  'grep -qF "cursor/docket-status" <<<"$warn_out"'
# The WARN goes to STDERR and must never leak into the generated document.
assert "cursor: WARN never leaks into the wrapper file" '! grep -qi "dropped" "$C"'
rm -rf "$SBX"

echo "---"; [ "$fail" = "0" ] && echo "ALL PASS" || echo "FAILURES"; exit $fail
```

- [ ] **Step 2: Run it to verify it fails**

Run: `bash tests/test_sync_agents_cursor.sh`

Expected: FAIL. The `no standalone effort: key`, `no skills: preload key`, and every bracket-encoded
`model:` assert redden, because Cursor currently goes through the generic Claude-shaped `emit()`.
Confirm the failures are those specific asserts — a whole-file error (e.g. a helper typo) is a test
bug, not the defect.

- [ ] **Step 3: Add `emit_cursor_md()` to `sync-agents.sh`**

Insert immediately after `emit_codex_toml()` ends (the function whose last line is the
`printf 'developer_instructions = """\n%s\n"""\n'` call), so the two named emitters sit together.

```sh
# Transform a built-in markdown wrapper into a Cursor custom-agent document on stdout (change 0135).
# Cursor documents exactly five frontmatter fields — name, description, model, readonly,
# is_background — and encodes reasoning effort INSIDE the model value (`<id>[effort=<e>]`). It has
# no standalone `effort:` field and no `skills:` preload, so the generic Claude-shaped emitter
# silently dropped all three. Field mapping (ADR-0015 verbatim passthrough for model/effort):
#   frontmatter name:        -> name
#   frontmatter description: -> description
#   effective model + effort -> model: <model>[effort=<effort>]   (see the table below)
#   skills: preload + body   -> a body preamble + the body verbatim
# readonly/is_background are deliberately NOT emitted: their Cursor defaults already match every
# docket agent (agents commit and push; every docket dispatch is foreground), and emitting them
# would assert a policy docket does not have.
#
#   model            effort           emitted
#   claude-opus-4-8  xhigh            model: claude-opus-4-8[effort=xhigh]
#   claude-opus-4-8  unset|auto       model: claude-opus-4-8
#   unset|inherit    unset|auto       (no model: line)
#   unset|inherit    xhigh            (no model: line) + a generation-time WARN
#
# Docket keeps NO allowlist of Cursor model IDs and NO allowlist of effort tokens: Cursor's own
# compatible-model fallback handles anything it does not recognize, and a committed table of a
# vendor's internals goes stale silently (ADR-0015; ADR-0059's rejection of vendor-internal tables).
emit_cursor_md(){  # $1=src md  $2=model_override  $3=effort_override
  local src="$1" mo="$2" eo="$3"
  local name desc bi_model bi_effort model effort skills_csv body
  name="$(sed -n '/^name:/{s/^name:[[:space:]]*//;p;q;}' "$src")"
  [ -n "$name" ] || name="docket-$(short_name "$src")"
  desc="$(agent_description "$src")"
  bi_model="$(sed -n '/^model:/{s/^model:[[:space:]]*//;p;q;}' "$src")"
  bi_effort="$(sed -n '/^effort:/{s/^effort:[[:space:]]*//;p;q;}' "$src")"
  model="${mo:-$bi_model}"
  effort="${eo:-$bi_effort}"
  # Normalize the two "no pin" sentinels to empty, so the emit logic below has one shape to test.
  [ "$model" = "inherit" ] && model=""
  [ "$effort" = "auto" ] && effort=""
  skills_csv="$(sed -n '/^skills:/{s/^skills:[[:space:]]*//;p;q;}' "$src" | sed -e 's/^\[//' -e 's/\][[:space:]]*$//' -e 's/[[:space:]]*$//')"
  # body = everything after the frontmatter closing --- , leading blank lines trimmed.
  body="$(awk '/^---[[:space:]]*$/ && d<2 {d++; next} d>=2 {print}' "$src" | awk 'NF{p=1} p{print}')"
  printf -- '---\n'
  printf 'name: %s\n' "$name"
  printf 'description: %s\n' "$desc"
  if [ -n "$model" ]; then
    if [ -n "$effort" ]; then
      printf 'model: %s[effort=%s]\n' "$model" "$effort"
    else
      printf 'model: %s\n' "$model"
    fi
  elif [ -n "$effort" ]; then
    log "WARN cursor/$name: effort '$effort' dropped — Cursor encodes effort inside the model value, and the resolved model is 'inherit'. Set an explicit model to pin effort on Cursor."
  fi
  printf -- '---\n\n'
  if [ -n "$skills_csv" ]; then
    printf 'Before acting, load these docket skills from your Cursor skills directory: %s.\n\n' "$skills_csv"
  fi
  printf '%s\n' "$body"
}
```

- [ ] **Step 4: Give `emit_for_harness()` named branches**

Replace the existing two-arm `case` with four named arms. The `*)` arm keeps working exactly as
today — the point is to make it a **stated** gap rather than silent inheritance, which is how Cursor
got here in the first place.

```sh
emit_for_harness(){  # $1=src md  $2=harness  $3=model  $4=effort
  case "$2" in
    codex)  emit_codex_toml "$1" "$3" "$4";;
    cursor) emit_cursor_md  "$1" "$3" "$4";;
    claude) emit            "$1" "$3" "$4";;
    # The generic Claude-shaped wrapper. A harness reaching this branch has NO verified contract
    # mapping — its wrapper is a best guess, not a supported shape. Adding a harness token here
    # without a named emitter is how the Cursor defect (change 0135) shipped: the token inherited
    # Claude's frontmatter, and docket reported pins the harness never read. Give a new harness its
    # own emitter, or accept that its wrapper is unverified.
    *)      emit            "$1" "$3" "$4";;
  esac
}
```

- [ ] **Step 5: Run the new test file to verify it passes**

Run: `bash tests/test_sync_agents_cursor.sh`
Expected: `ALL PASS`.

- [ ] **Step 6: Mutation-test the three load-bearing asserts**

A guard is code. For each mutation, apply it, run `bash tests/test_sync_agents_cursor.sh`, confirm
it **reddens**, then revert.

1. In `emit_cursor_md()`, add `printf 'effort: %s\n' "$effort"` before the closing `---`.
   → `cursor: emits NO standalone effort: key` must redden.
2. Change the bracket `printf` to `printf 'model: %s\n' "$model"` (drop the effort suffix).
   → `cursor: model carries bracket-encoded effort` must redden.
3. Change `cursor)` in `emit_for_harness()` back to routing through `emit`.
   → the effort-key, skills-key, and bracket asserts must all redden together.

If any mutation leaves the suite green, that assert is decoration — fix the assert before
continuing.

- [ ] **Step 7: Retire the four defect-encoding assertions in `tests/test_sync_agents.sh`**

Run: `bash tests/test_sync_agents.sh` — it is now RED. Four assertions asserted the **defect**.
Ask what each block **guards**, not what it asserts (learnings: `test-premise-deleted-not-regated`):
a block guarding a still-live mechanism is narrowed; a block whose subject is gone is deleted.

**(a) `0046 (b): cursor effort inherited from default`** — its subject (a standalone `effort:` key
in a Cursor wrapper) no longer exists, but the **mechanism** it guards — field-level merge, where
`effort` falls through to `default:` while `model` comes from the `cursor:` block — is still live and
still worth a guard. Narrow it to read the surviving carrier of that value:

```bash
assert "0046 (b): cursor effort inherited from default (now inside the model value)" \
  '[ "$(fm "$SBX/.cursor/agents/docket-status.md" model)" = "gpt-5.5-medium-fast[effort=high]" ]'
```

**(b) `0046 (a): cursor model from cursor block` and `0046 (c): non-Claude id verbatim in .cursor`**
— both still meaningful; update the expected value to the bracket-encoded form
`gpt-5.5-medium-fast[effort=high]` (the sandbox's `default:` block sets `effort: high`).

**(c) `0046 (d): default-only => both harness files byte-identical`** — its premise is exactly the
defect. Byte-identity across harnesses is now **forbidden**, so invert it into the positive contract
and keep the surviving half of the original claim (that `default:` alone reaches both harnesses):

```bash
# 0135 inverted this: a Cursor wrapper is NO LONGER Claude-shaped, so default-only config must
# produce DIFFERENT files. The surviving property is that the default: block reaches both.
assert "0135 (d): default-only => harness files DIFFER (cursor has its own shape)" \
  '! diff -q "$SBX/.claude/agents/docket-status.md" "$SBX/.cursor/agents/docket-status.md" >/dev/null'
assert "0046 (d): default-only applies model to claude" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "sonnet" ]'
assert "0135 (d): default-only applies model+effort to cursor" '[ "$(fm "$SBX/.cursor/agents/docket-status.md" model)" = "sonnet[effort=high]" ]'
```

**(d) `global cursor block wins for cursor`** — update the expected value to
`gpt-5.5-medium-fast[effort=medium]` (`docket-status`'s built-in effort is `medium`; the global
config sets only a model, so effort falls through to the built-in).

**(e) `0046 fanout: cursor carries its override model`** — same update; check the fixture's resolved
effort and write the bracket form it produces.

Two nearby assertions need a **look, not an edit**:
- `0046: harness files differ when overridden` and `0046 fanout: harness files differ when cursor
  overrides` still pass, but are now trivially true (every Cursor file differs). Leave them —
  they are cheap — and note in the results file that their discriminating power moved to the
  inverted `(d)` assert above.
- `0045 check: advisory-flags .cursor/agents drift` uses
  `sed -i.bak 's/^model: sonnet/model: haiku/'`. `^model: sonnet` still matches the prefix of
  `model: sonnet[effort=high]`, so the drift test still drifts the file. **Verify this by reading
  the sed result**, do not assume.

- [ ] **Step 8: Run the two sync-agents suites plus the codex regression guard**

Run:
```bash
bash tests/test_sync_agents_cursor.sh
bash tests/test_sync_agents.sh
bash tests/test_sync_agents_codex.sh
```
Expected: `ALL PASS` from all three. The Codex file is the regression guard on the emitter split —
if it moved, `emit_for_harness()` was edited wrong.

- [ ] **Step 9: Commit**

```bash
git add sync-agents.sh tests/test_sync_agents_cursor.sh tests/test_sync_agents.sh
git commit -m "fix(0135): emit Cursor wrappers against Cursor's own subagent contract

Cursor documents five frontmatter fields and encodes reasoning effort inside
the model value; it has no effort: field and no skills: preload. Routing cursor
through the generic Claude-shaped emitter silently dropped the model pin, the
effort pin, and the skills, while docket reported all three as honored.

Adds emit_cursor_md() and names the emit_for_harness() branches, documenting
the *) catch-all as an unverified gap rather than a supported mapping.
Retires the four test assertions that encoded the defect."
```

---

### Task 2: Reword the Cursor dispatch rule to capability language

**Files:**
- Modify: `cursor-rules/dispatch.head.md`
- Modify: all nine `cursor-rules/dispatch/docket-*.md` fragments
- Modify: `sync-agents.sh` — the no-fragment auto-block inside `assemble_dispatch_rule()`
- Create: `tests/test_cursor_dispatch_rule.sh`

**Interfaces:**
- Consumes: nothing from Task 1.
- Produces: nothing later tasks depend on.

**Why.** ADR-0059 §2 permits a tool name as an observed internal or a diagnostic, and forbids one as
a **decision input**. These fragments tell a parent chat how to act. Change 0137's narrower prose fix
correctly left them alone (they are Cursor-scoped), but standardizing them on capability language
removes the last place a reader could infer that docket depends on the literal name — the exact
priming that produced #0136's false negative (learnings:
`capability-absence-needs-a-failed-attempt`).

- [ ] **Step 1: Write the failing guard**

Create `tests/test_cursor_dispatch_rule.sh`. The fragment population is derived **by glob**, never
hand-listed (AGENTS.md), with a population floor so a vanished glob cannot pass vacuously
(learnings: `marker-scoped-guard-needs-a-population-floor`).

```bash
#!/usr/bin/env bash
# tests/test_cursor_dispatch_rule.sh — the Cursor dispatch rule instructs by CAPABILITY, never by
# a literal tool name (change 0135; ADR-0059 §2). A concrete call snippet may still appear, but
# only as a clearly-labelled ILLUSTRATION — naming a tool in an instruction primes the next agent
# to probe for that literal and conclude absence when the mechanism ships under a different name
# (learnings: capability-absence-needs-a-failed-attempt).
# run: bash tests/test_cursor_dispatch_rule.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

HEAD="$REPO/cursor-rules/dispatch.head.md"
assert "head: exists" '[ -f "$HEAD" ]'
assert "head: instructs by capability (subagent-launch mechanism)" \
  'grep -qiE "subagent-launch mechanism|this mode.s subagent" "$HEAD"'
assert "head: still forbids running inline" 'grep -qiE "not? run the skill inline|never run.*inline" "$HEAD"'
assert "head: still requires foreground" 'grep -qi "foreground" "$HEAD"'

# Population derived by glob, with a floor. Nine built-in agents ship fragments today; the floor
# is >= 9 so adding a tenth agent does not redden, while a vanished/renamed directory does.
frags=""; n=0
for f in "$REPO"/cursor-rules/dispatch/docket-*.md; do
  [ -e "$f" ] || continue
  frags="$frags $f"; n=$((n+1))
done
assert "fragments: population floor reached (>= 9 found)" '[ "$n" -ge 9 ]'

for f in $frags; do
  b="$(basename "$f")"
  # A fragment may SHOW a call snippet, but its INSTRUCTION line must not name a tool. The
  # instruction lines are the prose ones; the illustration is indented as a code block.
  instr="$(grep -v '^    ' "$f")"
  assert "$b: instruction prose names no dispatch tool literal" \
    '! grep -qE "\b(Task|Agent)\b" <<<"$instr"'
  assert "$b: instruction says dispatch to the subagent" \
    'grep -qiE "dispatch (to|the)" <<<"$instr"'
  # If a call snippet is present at all, it must be LABELLED as an illustration, so a reader
  # cannot mistake the name for the contract.
  if grep -qE '^    [A-Za-z]+\(' "$f"; then
    assert "$b: call snippet is labelled an illustration" 'grep -qi "illustration" "$f"'
  fi
done

echo "---"; [ "$fail" = "0" ] && echo "ALL PASS" || echo "FAILURES"; exit $fail
```

- [ ] **Step 2: Run it to verify it fails**

Run: `bash tests/test_cursor_dispatch_rule.sh`
Expected: FAIL — every fragment's instruction prose currently says `Launch a **Task** with
subagent_type: …`, and the head's `description:` frontmatter says "force a Task dispatch".

- [ ] **Step 3: Reword `cursor-rules/dispatch.head.md`**

Replace the `description:` line and the numbered dispatch pattern. Keep everything else.

```markdown
---
description: Docket agents must be dispatched, never run inline. Cursor runs a directly-invoked skill at the current model, which defeats docket's model/effort pins — so force a dispatch to the matching docket subagent.
alwaysApply: true
---

# Docket agents — dispatch only

Docket ships model/effort-pinned subagent wrappers in `.cursor/agents/docket-*.md`. When you are
asked to run one of the docket agents listed below, Cursor would otherwise run the skill **inline at
the currently-selected model**, which defeats the pin. Always dispatch to the matching subagent
instead.

## Required dispatch pattern

For every docket agent named below:

1. Do **NOT** run the skill inline in this chat.
2. Dispatch to the subagent `docket-<name>` using this mode's subagent-launch mechanism,
   **foreground** — block until it returns; never background it and never poll.
3. Pass the user's request through in the prompt, including any change / ADR id or argument they
   gave.
4. Relay the subagent's result back; do not re-do its work in the parent chat.

If the dispatch mechanism appears unavailable, resolve before concluding — including any deferred or
lazily-loaded tool surface this mode exposes — and, if resolution is inconclusive, attempt one
trivial dispatch. Only a failed attempt or an explicit policy denial establishes unavailability; the
absence of a tool with a particular **name** never does.
```

- [ ] **Step 4: Reword all nine fragments**

For each `cursor-rules/dispatch/docket-*.md`, replace the instruction line and demote the snippet.
Every fragment follows the same shape. Worked example — `docket-implement-next.md`:

```markdown
## docket-implement-next — dispatch only

Trigger when asked to implement the next build-ready change, drain the backlog, or build a specific
change id end-to-end (e.g. "implement the next change", "build change 48", "drain the docket backlog").

Dispatch to the subagent `docket-implement-next`, foreground, using this mode's subagent-launch
mechanism. The prompt must include the explicit change id if the user named one (otherwise let the
agent select), and that it runs autonomously to an open PR and stops at the human merge gate.

Do NOT run the build inline, merge the PR, or re-brainstorm the design (the agent reconciles but never
re-brainstorms).

One concrete call, as an illustration of the shape — not the contract:

    Task(subagent_type: "docket-implement-next", run_in_background: false,
         prompt: "Implement change 48 end-to-end to an open PR; stop at the merge gate.")
```

Apply the identical transformation to the other eight (`docket-adr`, `docket-auto-groom`,
`docket-auto-groom-critic`, `docket-brainstorm-consultant`, `docket-finalize-change`,
`docket-integration-repair`, `docket-rebase-resolver`, `docket-status`): replace
`Launch a **Task** with \`subagent_type: "docket-<name>"\`…` (and any `Dispatch prompt must include…`
opener) with `Dispatch to the subagent \`docket-<name>\`, foreground, using this mode's
subagent-launch mechanism.`, keep each fragment's own trigger/prompt/do-not prose verbatim, and
insert the `One concrete call, as an illustration of the shape — not the contract:` line directly
above the existing indented snippet. **Do not otherwise reword a fragment** — each one carries
per-agent variance that a template would flatten (learnings:
`consolidation-flattens-caller-variance`); diff the nine against each other before assuming any two
are the same.

- [ ] **Step 5: Reword the no-fragment auto-block in `sync-agents.sh`**

Inside `assemble_dispatch_rule()`, the `else` arm emits a minimal auto-block for a built-in with no
fragment. It names `Task` in an instruction, so it violates the same rule:

```sh
      printf 'When this applies, do NOT run the skill inline. Dispatch to the subagent `docket-%s` using this mode'"'"'s subagent-launch mechanism, foreground, and relay its result.\n' "$name"
```

- [ ] **Step 6: Run the guard to verify it passes**

Run: `bash tests/test_cursor_dispatch_rule.sh`
Expected: `ALL PASS`.

- [ ] **Step 7: Mutation-test the guard**

1. Revert one fragment's instruction line to `Launch a **Task** with subagent_type: …`.
   → `<that fragment>: instruction prose names no dispatch tool literal` must redden.
2. Delete the `illustration` label from one fragment (leaving its snippet).
   → `<that fragment>: call snippet is labelled an illustration` must redden.
3. `mv cursor-rules/dispatch cursor-rules/dispatch.bak` and rerun.
   → `fragments: population floor reached (>= 9 found)` must redden. Restore.

- [ ] **Step 8: Run the affected suites**

Run:
```bash
bash tests/test_cursor_dispatch_rule.sh
bash tests/test_sync_agents.sh
bash tests/test_dispatch_capability.sh
```
Expected: `ALL PASS`. `test_sync_agents.sh` asserts dispatch-rule content
(`0048 rule: …`) — if any of those assertions pinned a phrase you just reworded, update **that
assertion's expected phrase**, not the rule back to naming a tool.

- [ ] **Step 9: Commit**

```bash
git add cursor-rules sync-agents.sh tests/test_cursor_dispatch_rule.sh tests/test_sync_agents.sh
git commit -m "fix(0135): word the Cursor dispatch rule by capability, not tool name

ADR-0059 permits a tool name as a diagnostic and forbids one as a decision
input. The head and all nine fragments now instruct 'dispatch using this
mode's subagent-launch mechanism'; the concrete call stays as a labelled
illustration."
```

---

### Task 3: The `cursor` runner adapter

**Files:**
- Create: `scripts/runners/cursor.sh`
- Create: `scripts/runners/cursor.md`
- Modify: `sync-agents.sh` — `REGISTERED_RUNNERS="codex cursor"`
- Create: `tests/test_runner_cursor.sh`

**Interfaces:**
- Consumes: `runner-dispatch.sh`'s adapter contract — the adapter is invoked as
  `bash scripts/runners/cursor.sh --agent <name> [--model <m>] [--effort <e>] [-- <args…>]` with
  `DOCKET_REPO_ROOT` exported, plus `DOCKET_RUNNER_CFG_<KEY>` for each `runners.cursor` key.
- Produces: the `cursor` token in `REGISTERED_RUNNERS`, which `sync-agents.sh`'s
  `is_registered_runner()` gates `runner: cursor` on. Registration is **also** the adapter file's
  existence, per `runner-dispatch.sh` — `tests/test_sync_agents.sh` already asserts parity in both
  directions, so adding the file without the token (or vice versa) reddens.

**Failure posture — the load-bearing constraint.** `cursor-agent` is known from prior hands-on
testing to be **unreliable and to lag the Cursor IDE in features**. A failure, timeout, or
missing-feature error is a **loud abort-and-report** — never a silent fall-back to running the agent
inline in the parent. This adapter must not become a new silent-degradation path, which would
reproduce this change's own root cause in a new location.

- [ ] **Step 1: Write the failing adapter tests**

Create `tests/test_runner_cursor.sh`, using a `CURSOR_BIN` mock seam exactly as the codex adapter
uses `CODEX_BIN`.

```bash
#!/usr/bin/env bash
# tests/test_runner_cursor.sh — the cursor runner adapter (change 0135). Mirrors runners/codex.sh:
# preflight, prompt assembly from the built-in wrapper source, verbatim model passthrough with
# effort ridden inside the model value, foreground exec, final-message relay on stdout.
# Failure posture is LOUD abort-and-report — never a silent inline fall-back, which would
# reproduce change 0135's own root cause in a new location.
# run: bash tests/test_runner_cursor.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ADAPTER="$REPO/scripts/runners/cursor.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

assert "adapter exists" '[ -f "$ADAPTER" ]'
assert "contract doc exists (test_script_contracts_coverage parity)" '[ -f "$REPO/scripts/runners/cursor.md" ]'
assert "registered in sync-agents REGISTERED_RUNNERS" 'grep -qE "^REGISTERED_RUNNERS=\"[^\"]*\bcursor\b" "$REPO/sync-agents.sh"'

# A mock cursor-agent that records its argv + stdin prompt and prints a final message.
MOCK_DIR="$(mktemp -d)"
cat > "$MOCK_DIR/cursor-agent" <<'MOCK'
#!/usr/bin/env bash
printf '%s\n' "$@" > "$MOCK_ARGV"
printf 'MOCK-FINAL-MESSAGE\n'
exit "${MOCK_RC:-0}"
MOCK
chmod +x "$MOCK_DIR/cursor-agent"

run_adapter(){  # $@ = adapter args ; sets OUT / RC / ARGV
  MOCK_ARGV="$MOCK_DIR/argv.txt"
  OUT="$( MOCK_ARGV="$MOCK_ARGV" MOCK_RC="${MOCK_RC:-0}" CURSOR_BIN="$MOCK_DIR/cursor-agent" \
          DOCKET_REPO_ROOT="$REPO" bash "$ADAPTER" "$@" 2>/dev/null )"
  RC=$?
  ARGV="$(cat "$MOCK_ARGV" 2>/dev/null)"
}

# --- happy path: foreground exec, final message relayed on stdout --------------------------------
run_adapter --agent status --model gpt-5.5-medium-fast --effort high
assert "happy: exits 0"                       '[ "$RC" = "0" ]'
assert "happy: relays the child final message" 'grep -qF "MOCK-FINAL-MESSAGE" <<<"$OUT"'
assert "happy: passes -p (non-interactive print mode)" 'grep -qxF -- "-p" <<<"$ARGV"'
assert "happy: effort rides INSIDE the model value" 'grep -qF -- "gpt-5.5-medium-fast[effort=high]" <<<"$ARGV"'
assert "happy: prompt carries the skills to load"  'grep -qF "docket-convention" <<<"$ARGV"'
assert "happy: prompt carries the wrapper body"    'grep -qi "refresh docket state" <<<"$ARGV"'

# --- no effort => BARE model, no bracket ---------------------------------------------------------
run_adapter --agent status --model gpt-5.5-medium-fast
assert "no effort: model passed bare" 'grep -qxF -- "gpt-5.5-medium-fast" <<<"$ARGV"'
assert "no effort: no bracket encoding" '! grep -qF -- "[effort=" <<<"$ARGV"'

# --- no model + an effort => effort DROPPED with a warn (mirrors the emitter's edge case) --------
ERR="$( MOCK_ARGV="$MOCK_DIR/argv.txt" CURSOR_BIN="$MOCK_DIR/cursor-agent" DOCKET_REPO_ROOT="$REPO" \
        bash "$ADAPTER" --agent status --effort high 2>&1 >/dev/null )"
assert "no model: effort dropped with a WARN" 'grep -qi "effort" <<<"$ERR" && grep -qi "dropped" <<<"$ERR"'
assert "no model: no --model flag passed" '! grep -qxF -- "--model" "$MOCK_DIR/argv.txt"'

# --- preflight: binary missing => loud abort, NEVER a degrade ------------------------------------
OUT="$( CURSOR_BIN="$MOCK_DIR/definitely-not-here" DOCKET_REPO_ROOT="$REPO" \
        bash "$ADAPTER" --agent status 2>&1 )"; RC=$?
assert "preflight: missing binary exits nonzero" '[ "$RC" != "0" ]'
assert "preflight: diagnostic names cursor-agent" 'grep -qi "cursor-agent" <<<"$OUT"'
assert "preflight: never suggests running inline instead" '! grep -qi "inline" <<<"$OUT"'

# --- child nonzero propagates (abort-and-report, no retry) ---------------------------------------
MOCK_RC=7 run_adapter --agent status
assert "child nonzero: adapter propagates it" '[ "$RC" = "7" ]'
unset MOCK_RC

# --- missing DOCKET_REPO_ROOT => precondition abort ----------------------------------------------
OUT="$( CURSOR_BIN="$MOCK_DIR/cursor-agent" bash "$ADAPTER" --agent status 2>&1 )"; RC=$?
assert "precondition: unset DOCKET_REPO_ROOT aborts" '[ "$RC" != "0" ]'
assert "precondition: names runner-dispatch as the entry point" 'grep -qi "runner-dispatch" <<<"$OUT"'

# --- unknown agent => precondition abort ----------------------------------------------------------
OUT="$( CURSOR_BIN="$MOCK_DIR/cursor-agent" DOCKET_REPO_ROOT="$REPO" bash "$ADAPTER" --agent nope 2>&1 )"; RC=$?
assert "precondition: unknown agent aborts" '[ "$RC" != "0" ]'
assert "precondition: names the expected source path" 'grep -qF "docket-nope.md" <<<"$OUT"'

rm -rf "$MOCK_DIR"
echo "---"; [ "$fail" = "0" ] && echo "ALL PASS" || echo "FAILURES"; exit $fail
```

- [ ] **Step 2: Run it to verify it fails**

Run: `bash tests/test_runner_cursor.sh`
Expected: FAIL at `adapter exists` — `scripts/runners/cursor.sh` does not exist yet.

- [ ] **Step 3: Write `scripts/runners/cursor.sh`**

Structurally parallel to `codex.sh`. Read `scripts/runners/codex.sh` first and keep the same
section order, the same `die()` shape, and the same pipefail-safe `head -n1 <<<"$var"` idiom.

```sh
#!/usr/bin/env bash
# scripts/runners/cursor.sh — the cursor runner adapter (change 0135). Owns everything
# child-specific for delegating a whole agent run to Cursor's CLI via `cursor-agent -p`:
# preflight (binary), prompt assembly from the built-in wrapper source, flag mapping (model
# verbatim per ADR-0015; effort ridden INSIDE the model value, matching Cursor's own
# `<id>[effort=<e>]` encoding — Cursor has no separate effort flag), foreground execution,
# final-message relay on stdout. Invoked by runner-dispatch.sh — not directly by skills.
# Contract: scripts/runners/cursor.md. Mock seam: CURSOR_BIN.
#
# RECORDED RISK: cursor-agent is known to be unreliable and to lag the Cursor IDE in features, so
# this adapter rests on a shakier foundation than runners/codex.sh. Its failure posture is pinned
# accordingly — any failure, timeout, or missing-feature error is a LOUD abort-and-report, never a
# silent fall-back to running the agent inline in the parent. A silent degrade here would
# reproduce change 0135's own root cause in a new location.
set -uo pipefail
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AGENTS_SRC="$SELF_DIR/../../agents"
CURSOR_BIN="${CURSOR_BIN:-cursor-agent}"

die(){ printf 'runners/cursor: %s\n' "$*" >&2; exit 1; }
warn(){ printf 'runners/cursor: %s\n' "$*" >&2; }

AGENT=""; MODEL=""; EFFORT=""
while [ $# -gt 0 ]; do
  case "$1" in
    --agent)  AGENT="${2:-}"; shift 2 ;;
    --model)  MODEL="${2:-}"; shift 2 ;;
    --effort) EFFORT="${2:-}"; shift 2 ;;
    --) shift; break ;;
    *) die "unknown argument: $1" ;;
  esac
done
[ -n "$AGENT" ] || die "--agent is required"
[ -n "${DOCKET_REPO_ROOT:-}" ] || die "DOCKET_REPO_ROOT is not set (invoke via docket.sh runner-dispatch)"

SRC="$AGENTS_SRC/docket-$AGENT.md"
[ -f "$SRC" ] || die "no built-in agent source for '$AGENT' (expected $SRC)"

# --- preflight: binary (abort-and-report; never degrade to a native run) --------
command -v "$CURSOR_BIN" >/dev/null 2>&1 || die "cursor-agent CLI not on PATH — install Cursor's CLI or unset runner: cursor"

# --- prompt assembly: skills to load + the wrapper body + passthrough args -------
skills_line="$(sed -n 's/^skills:[[:space:]]*\[\(.*\)\].*/\1/p' "$SRC")"
skills_line="$(head -n1 <<<"$skills_line" | tr ',' ' ')"
skills_line="$(printf '%s' "$skills_line" | tr -s '[:space:]' ' ' | sed 's/^ *//; s/ *$//')"
body="$(awk '/^---[[:space:]]*$/{d++; next} d>=2{print}' "$SRC")"
prompt=""
if [ -n "$skills_line" ]; then
  prompt="First, load these skills by name, in this order:"
  for s in $skills_line; do prompt="$prompt
- invoke skill \`$s\`"; done
  prompt="$prompt

Then execute the following instructions exactly:

"
fi
prompt="$prompt$body"
if [ $# -gt 0 ]; then
  prompt="$prompt

Additional caller arguments / task context:
$*"
fi

# --- flag mapping -----------------------------------------------------------------
# ADR-0015: the model ID is passed VERBATIM and never validated. Cursor has no --effort flag —
# reasoning effort is a model parameter encoded inside the model value, the same `<id>[effort=<e>]`
# shape the wrapper emitter uses. With no model resolved the effort has nowhere to attach, so it is
# dropped — LOUDLY, because a silently-dropped pin is this change's own root cause.
if [ -n "$MODEL" ] && [ -n "$EFFORT" ] && [ "$EFFORT" != "auto" ]; then
  MODEL="$MODEL[effort=$EFFORT]"
elif [ -z "$MODEL" ] && [ -n "$EFFORT" ] && [ "$EFFORT" != "auto" ]; then
  warn "WARN effort '$EFFORT' dropped — Cursor encodes effort inside the model value, and no model is resolved. Set an explicit model to pin effort on Cursor."
fi

cmd=( "$CURSOR_BIN" -p --output-format text )
[ -n "$MODEL" ] && cmd+=( --model "$MODEL" )
cmd+=( "$prompt" )

# --- foreground execution + final-message relay ------------------------------------
"${cmd[@]}"
rc=$?
if [ "$rc" != "0" ]; then
  printf 'runners/cursor: cursor-agent exited %s\n' "$rc" >&2
  exit "$rc"
fi
exit 0
```

- [ ] **Step 4: Register the runner**

In `sync-agents.sh`, change the registry line:

```sh
REGISTERED_RUNNERS="codex cursor"
```

- [ ] **Step 5: Write `scripts/runners/cursor.md`**

Mechanically required by `tests/test_script_contracts_coverage.sh` (parts 3 and 4). Follow
`scripts/runners/codex.md`'s section order exactly: Purpose / Usage / Behavior / Exit codes /
Invariants / Prerequisites. Its load-bearing content:

- **Usage:** `bash scripts/runners/cursor.sh --agent <name> [--model <m>] [--effort <e>] [--] [<args…>]`
- **`--model`** — passed to `cursor-agent --model` **verbatim** (ADR-0015 opaque passthrough).
- **`--effort`** — Cursor has **no effort flag**; the value rides inside the model value as
  `<model>[effort=<effort>]`. With no model resolved it is **dropped with a WARN**.
- **Behavior:** preflight (binary resolvable) → prompt assembly from `agents/docket-<agent>.md`
  (skills list, then body verbatim, then passthrough args) → flag mapping → one foreground
  `cursor-agent -p --output-format text` invocation → the child's final message on stdout.
- **Exit codes:** `0` success; `1` precondition abort (bad args, missing agent source, missing
  binary, missing `DOCKET_REPO_ROOT`); any other — the child's own nonzero exit, propagated.
- **Invariants:** model IDs are never validated or rewritten (ADR-0015); exactly one `cursor-agent`
  invocation per run, always foreground, never backgrounded; **never degrades to running the agent
  natively** — a `cursor-agent` failure, timeout, or missing-feature error is a loud
  abort-and-report.
- **Recorded risk (state it in the contract, not only the script):** `cursor-agent` is unreliable
  and lags the Cursor IDE in features; this adapter therefore rests on a shakier foundation than
  `runners/codex.md`, which is exactly why its failure posture admits no fall-back.
- **Prerequisites:** Cursor CLI installed (`cursor-agent` on PATH) and authenticated; docket skills
  linked into `~/.cursor/skills` (`link-skills.sh`, automatic on install).

- [ ] **Step 6: Run the adapter tests to verify they pass**

Run: `bash tests/test_runner_cursor.sh`
Expected: `ALL PASS`.

- [ ] **Step 7: Mutation-test the two posture asserts**

1. Change the missing-binary `die` to a `warn` + `exit 0`.
   → `preflight: missing binary exits nonzero` must redden. Revert.
2. Change the flag mapping to pass `--effort "$EFFORT"` as a separate flag instead of bracketing.
   → `happy: effort rides INSIDE the model value` must redden. Revert.

- [ ] **Step 8: Run the runner + coverage + parity suites**

Run:
```bash
bash tests/test_runner_cursor.sh
bash tests/test_runner_dispatch.sh
bash tests/test_script_contracts_coverage.sh
bash tests/test_sync_agents.sh
```
Expected: `ALL PASS`. `test_sync_agents.sh`'s registry↔adapter parity block asserts both directions
— it reddens if the token and the file disagree.

- [ ] **Step 9: Commit**

```bash
git add scripts/runners/cursor.sh scripts/runners/cursor.md sync-agents.sh tests/test_runner_cursor.sh
git commit -m "feat(0135): add the cursor runner adapter

Mirrors runners/codex.sh for cursor-agent -p. Model passes through verbatim
(ADR-0015); effort rides inside the model value since Cursor has no effort
flag. cursor-agent is unreliable and lags the IDE, so every failure is a loud
abort-and-report — never a silent fall-back to an inline run."
```

---

### Task 4: Documentation — per-harness wrapper shapes

**Files:**
- Modify: `skills/docket-convention/references/agent-layer.md`
- Modify: `README.md`
- Create: `tests/test_cursor_contract_docs.sh`

**Interfaces:**
- Consumes: the emitted shapes from Tasks 1 and 3.
- Produces: nothing later tasks depend on.

**Why.** `agent-layer.md` currently implies **one uniform wrapper shape** — its `agents:` example
shows `{ model: gpt-5.1, effort: high }` under a `cursor:` key with no hint that the generated Cursor
file carries neither key, and the "Always-full-set generation" section still says the Cursor rule
"forces a Task dispatch". Both read as settled contract and are wrong.

- [ ] **Step 1: Write the failing docs guard**

Create `tests/test_cursor_contract_docs.sh`.

```bash
#!/usr/bin/env bash
# tests/test_cursor_contract_docs.sh — the agent-layer reference states the per-harness wrapper
# shapes rather than implying one uniform shape (change 0135), and the Cursor validation runbook
# carries its load-bearing evidence rule.
# run: bash tests/test_cursor_contract_docs.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

AL="$REPO/skills/docket-convention/references/agent-layer.md"
assert "agent-layer: has a per-harness wrapper-shape table" \
  'grep -qiE "^\| *harness *\|" "$AL"'
# One row per harness with a named emitter. Derived from the emitters that actually exist, so a
# new named emitter without a doc row reddens (correspondence-guard-runs-one-way: anchor on the
# consuming code, never an allowlist).
for h in claude cursor codex; do
  assert "agent-layer: table has a $h row" 'grep -qE "^\| *'"$h"' *\|" "$AL"'
done
assert "agent-layer: cursor row shows bracket-encoded effort" 'grep -qF -- "[effort=" "$AL"'
assert "agent-layer: cursor row says skills ride in the body" \
  'grep -qE "^\| *cursor *\|.*body preamble" "$AL"'
assert "agent-layer: no longer claims the Cursor rule forces a Task dispatch" \
  '! grep -qF "forces a Task dispatch" "$AL"'

# Every emitter named in sync-agents.sh must have a documented row (the reverse direction).
while IFS= read -r h; do
  [ -n "$h" ] || continue
  assert "agent-layer: emitter '$h' has a documented wrapper shape" 'grep -qE "^\| *'"$h"' *\|" "$AL"'
done < <(sed -n -E 's/^[[:space:]]*(claude|cursor|codex)\)[[:space:]]*emit.*/\1/p' "$REPO/sync-agents.sh")

RB="$REPO/docs/cursor/validation.md"
assert "runbook: exists" '[ -f "$RB" ]'
assert "runbook: states the Tier 2 evidence rule (a negative CLI result proves nothing)" \
  'grep -qiE "never evidence that the wrapper contract is wrong" "$RB"'
assert "runbook: Tier 3 is the certifying tier, required before merge" \
  'grep -qiE "required before merge|certifying tier" "$RB"'
assert "runbook: names all six IDE phases" \
  '[ "$(grep -cE "^### Phase [1-6]" "$RB")" = "6" ]'

assert "README: registers cursor as a shipped runner" \
  'grep -qE "runners/cursor\.md|runner: cursor" "$REPO/README.md"'

echo "---"; [ "$fail" = "0" ] && echo "ALL PASS" || echo "FAILURES"; exit $fail
```

- [ ] **Step 2: Run it to verify it fails**

Run: `bash tests/test_cursor_contract_docs.sh`
Expected: FAIL — no table, no runbook, no README entry.

- [ ] **Step 3: Add the per-harness wrapper-shape table to `agent-layer.md`**

Insert it at the end of the *Harness-portable model IDs* section, immediately after the paragraph
ending "…lets docket drive non-Claude harnesses."

```markdown
**Per-harness wrapper shapes.** The generated wrapper is **not one uniform document** — each harness
gets its target harness's own documented shape, written by its own named emitter in
`sync-agents.sh`. A harness with no named emitter falls to the generic `*)` branch, which emits
**Claude's** shape: a best guess, not a supported mapping (change 0135; the Cursor defect shipped
exactly that way).

| harness | file | model | effort | skills |
|---|---|---|---|---|
| claude | `.md` | `model:` | `effort:` | `skills:` frontmatter |
| cursor | `.md` | `model: <id>[effort=<e>]` | *(inside the model value)* | body preamble |
| codex | `.toml` | `model =` | `model_reasoning_effort =` | `developer_instructions` preamble |

Cursor documents five frontmatter fields (`name`, `description`, `model`, `readonly`,
`is_background`) and has neither a standalone `effort:` key nor a `skills:` preload; docket emits
only `name`/`description`/`model` and leaves `readonly`/`is_background` at their Cursor defaults,
which already match every docket agent. When the resolved model is `inherit`, a resolved effort has
nowhere to attach and is dropped with a generation-time WARN.
```

- [ ] **Step 4: Correct the uniform-shape prose in `agent-layer.md`**

In the *Always-full-set generation + the Cursor dispatch rule* section, replace "that forces a Task
dispatch to the matching `subagent_type`" with "that forces a dispatch to the matching docket
subagent". In the `agents:` YAML example, append a comment to the `cursor:` block:

```yaml
  cursor:                               # per-harness override — only what differs
    implement-next: { model: gpt-5.1, effort: high }
    # NOTE: this is the CONFIG shape, identical across harnesses. The GENERATED Cursor wrapper
    # carries no `effort:` key — effort rides inside the model value. See the per-harness
    # wrapper-shape table above.
```

- [ ] **Step 5: Register `cursor` in README's Runner delegation section**

That section currently says "One pair ships today". Two ship now. Update the count, add a
`runner: cursor` line to the example config, and add the adapter's recorded risk in one sentence:

```markdown
`cursor` delegates to `cursor-agent -p`. Note that `cursor-agent` is unreliable and lags the Cursor
IDE in features, so the adapter's posture is a loud abort-and-report on any failure — it never falls
back to running the agent inline. Full adapter contract: `scripts/runners/cursor.md`.
```

- [ ] **Step 6: Write `docs/cursor/validation.md`**

The Tier 2 + Tier 3 runbook, sitting beside the existing `docs/cursor/permissions.md`. Modelled on
change #0078's Codex CLI validation runbook — this repo's house pattern.

Required content:

- **A `## Tier 2 — cursor-agent probe (best-effort, non-gating)` section** carrying the copy-pasteable
  probe (`cursor-agent -p --output-format json` dispatching a docket agent and reporting the
  effective model, whether the docket skills loaded, and whether a nested dispatch succeeds), the
  `cursor-agent --version` to record alongside the findings, and — verbatim, as a block quote — the
  **evidence rule**:

  > **A negative or absent result from `cursor-agent` is never evidence that the wrapper contract is
  > wrong.** It is recorded as a CLI limitation observation and nothing more. Only a *positive*
  > result carries weight, and it proves only that the contract works on the CLI surface.

  Follow it with why: treating an unreliable probe's silence as capability absence is the exact
  false-negative shape ADR-0059 exists to prevent — an absence observed in the wrong surface,
  promoted to a verdict. State plainly that a future implementer must not re-promote this spike to
  a gate.

- **A `## Tier 3 — Cursor IDE validation checklist (human-executed, required before merge)`
  section** with six `### Phase N — <name>` subsections, each with a definitive observable outcome:

  1. **Generated artifacts** — run `./sync-agents.sh`; assert `.cursor/agents/docket-*.md` contain no
     `effort:`/`skills:` keys, carry bracket-encoded models, and carry the skills preamble.
  2. **Agent visible** — the docket agents are listed and selectable in the Cursor IDE.
  3. **Dispatch honored** — asking for a docket agent dispatches to the subagent rather than running
     the skill inline in the parent chat.
  4. **Pin honored** — the child reports the pinned model *and* the pinned reasoning effort.
  5. **Skills loaded** — the child confirms `docket-convention` and its own docket skill loaded, and
     can state a convention rule it could only know from having loaded them.
  6. **SDD reachable at depth 2** — a real dispatch from inside the child succeeds, confirming live
     that Cursor's documented nesting limit of two is enough for docket's flat SDD topology.

  State the pass condition explicitly: **passes when phases 1–3 and 5 are green and phases 4 and 6
  have definitive observed answers.** Every gap found becomes a follow-up stub.

- **A closing `## The merge-gate obligation` section**: Tier 3 necessarily runs *after* the PR opens,
  so the PR body must state that Cursor IDE validation is pending, naming this checklist, so the
  human merge gate is not cleared on a green hermetic suite alone.

- [ ] **Step 7: Run the docs guard to verify it passes**

Run: `bash tests/test_cursor_contract_docs.sh`
Expected: `ALL PASS`.

- [ ] **Step 8: Mutation-test the docs guard**

1. Delete the `cursor` row from the wrapper-shape table.
   → `agent-layer: table has a cursor row` **and** `agent-layer: emitter 'cursor' has a documented
   wrapper shape` must both redden (the second proves the reverse direction is live). Revert.
2. Delete the evidence-rule block quote from `docs/cursor/validation.md`.
   → `runbook: states the Tier 2 evidence rule` must redden. Revert.
3. Delete one `### Phase N` heading.
   → `runbook: names all six IDE phases` must redden. Revert.

- [ ] **Step 9: Run the full suite**

Run every file in `tests/`, not only the ones this task touched (AGENTS.md: *run the whole suite at
the build gate*). Several prose guards span these files —
`tests/test_skill_size_budgets.sh` (the convention reference has a size budget),
`tests/test_comment_anchor_style.sh`, `tests/test_readme_finalize_docs.sh`, and
`tests/test_dispatch_capability.sh` all read documents this task edited.

```bash
for t in tests/test_*.sh; do
  printf '\n===== %s\n' "$t"
  bash "$t" || printf 'SUITE FAILED: %s\n' "$t"
done
```
Expected: every file ends `ALL PASS` (or its own success line).

- [ ] **Step 10: Commit**

```bash
git add skills/docket-convention/references/agent-layer.md README.md docs/cursor/validation.md tests/test_cursor_contract_docs.sh
git commit -m "docs(0135): per-harness wrapper shapes + the Cursor validation runbook

agent-layer.md stops implying one uniform wrapper shape and carries a
per-harness table. docs/cursor/validation.md adds the non-gating cursor-agent
probe (with its evidence rule: a negative CLI result is never evidence the
contract is wrong) and the six-phase human IDE checklist that is the
certifying tier."
```

---

## Not built by this plan

Two spec items are deliberately **not** feature-branch tasks:

- **The new ADR.** ADRs live on the `docket` metadata branch and are authored by the `docket-adr`
  agent, which assigns the number and updates the index. The decision to record: *a generated
  wrapper conforms to its target harness's own documented contract; the generic emitter is Claude's
  shape, not a default other harnesses may silently inherit, and a harness without a dedicated
  emitter is a known gap rather than a supported mapping.* `relates_to: [8, 15, 17, 59]`,
  `supersedes: []`, `reverses: []` — it **refines** ADR-0008 and ADR-0015 without superseding
  either, and cites ADR-0059 as governing the dispatch/tiering question it does not reopen.

- **Tier 3 execution.** The IDE checklist is human-executed after the PR opens. The plan ships the
  runbook; a human runs it. The PR body must say so.

## Self-Review

**Spec coverage.** Design §1 `emit_cursor_md()` → Task 1. §2 named `emit_for_harness()` branches →
Task 1 Step 4. §3 dispatch rule rewording → Task 2. §4 `runners/cursor.sh` → Task 3. §5 docs →
Task 4; §5 ADR → *Not built by this plan* (metadata-branch artifact). Verification Tier 1 → Task 1's
`tests/test_sync_agents_cursor.sh` plus Task 1 Step 7's retirement of the defect-encoding
assertions. Tier 2 → Task 4's runbook §Tier 2. Tier 3 → Task 4's runbook §Tier 3. The merge-gate
obligation → the runbook's closing section, and the PR body.

**Type consistency.** `emit_cursor_md` is spelled identically in Task 1 Steps 3, 4, 6 and Task 4's
reverse-direction guard. Its argument order (`src`, `model_override`, `effort_override`) matches
`emit_codex_toml`'s and the `emit_for_harness "$1" "$3" "$4"` call site. `CURSOR_BIN` is the mock
seam in both Task 3's adapter and its tests. `REGISTERED_RUNNERS="codex cursor"` is written once
(Task 3 Step 4) and asserted in Task 3's Step 1 test.

**Carve-out point.** If the PR proves unreviewable at full scope, **Task 3** (the runner adapter) is
the pre-agreed clean carve-out — it is a new capability rather than a repair of the wrapper-contract
defect, and Tasks 1, 2, 4 form a coherent standalone fix without it.
