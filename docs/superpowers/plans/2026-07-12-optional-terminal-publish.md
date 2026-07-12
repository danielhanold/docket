# Optional terminal-publish (`terminal_publish` knob) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a per-repo `terminal_publish` knob (default `true`) that, when `false`, makes docket's terminal-publish step a no-op — keeping archived change files, specs, and ADRs on the `docket` branch instead of committing them directly onto the integration branch.

**Architecture:** One guard, one config leaf, and the call-site wiring. `docket-config.sh` reads the `terminal_publish` leaf from the repo-committed `.docket.yml` only (coordination-key fenced, ADR-0019) and emits `TERMINAL_PUBLISH`. `terminal-publish.sh` — the single executor of **both** publish shapes (`--id` for close-out, `--adr` for `docket-adr`) — gains an `--enabled <true|false>` flag whose guard sits **before the mode dispatch**, so one guard covers both shapes. Every call site passes `--enabled "$TERMINAL_PUBLISH"`. Skill bodies otherwise unchanged.

**Tech Stack:** Bash (`set -uo pipefail`), the docket helper-script family under `scripts/`, hermetic bash test suites under `tests/` (temp repos + bare origins; no network, no `gh`).

## Global Constraints

- **Default `true`.** Omitting the knob, and omitting the `--enabled` flag, must behave **byte-identically to today**. Back-compat is non-negotiable — existing repos see no change.
- **Coordination-key fenced (per-repo-only).** `terminal_publish` is honored ONLY in the repo-committed `.docket.yml`. Set in the global `~/.config/docket/config.yml` or in `<repo>/.docket.local.yml` it is **warned-and-ignored** — never honored, never fatal. Rationale: the headless `docket-status` merge sweep can run where those machine-scoped files do not exist, so the policy must be committed to hold for every agent.
- **All-or-nothing.** `false` suppresses the whole publish: change file, spec, Accepted ADRs, AND the integration-branch ADR-index refresh. No per-artifact granularity.
- **Both publish shapes.** The guard covers `--id` AND `--adr`. Gating only close-out would leave `docket-adr` still committing ADRs to `main` — defeating the knob.
- **Fail-closed on a bad value.** An unparseable `terminal_publish` (repo config) or `--enabled` (flag) value **aborts** (exit 1). Never silently coerce to `true` — a typo must not silently re-enable writes to the integration branch.
- **A suppressed publish is SUCCESS (exit 0),** not a failure: it must not trip the close-out's skip-publish guard on steps 4–5 (cleanup + board still run).
- **Inert in `main`-mode.** terminal-publish is already a no-op there; the knob has no surface to act on and emits no warning.
- **Never edit ADR-0019.** An `Accepted` ADR is immutable except its `status:` line. The new fence classification is recorded in a NEW ADR authored at build time via `docket-adr`.

---

## File Structure

| File | Responsibility | Task |
|---|---|---|
| `scripts/terminal-publish.sh` | `--enabled` flag + the guard (before mode dispatch) | 1 |
| `scripts/terminal-publish.md` | contract: Usage / Behavior / Exit codes / Invariants | 1 |
| `tests/test_closeout.sh` | publish-guard tests (both shapes) + call-site wiring checks | 1, 3 |
| `scripts/docket-config.sh` | `terminal_publish` leaf, fence entry, `TERMINAL_PUBLISH` emit | 2 |
| `scripts/docket-config.md` | classification table row, fence bullet, key list, **18→19** | 2 |
| `tests/test_docket_config.sh` | resolution + fence tests; **18→19** count assertions | 2 |
| `skills/docket-convention/references/terminal-close-out.md` | step 3 gated; stale preamble corrected | 3 |
| `skills/docket-adr/SKILL.md` | both `--adr` call sites pass `--enabled` | 3 |
| `skills/docket-convention/SKILL.md` | `.docket.yml` schema block + fence sentence | 4 |
| `README.md` | document the knob + the coordination-key list | 4 |
| `.docket.yml` | commented sample entry (this repo keeps the default) | 4 |

---

### Task 1: `terminal-publish.sh` — the `--enabled` gate (both shapes)

**Files:**
- Modify: `scripts/terminal-publish.sh` (header ~10-14, arg loop ~27-42, validation ~44-55, guard ~57-61)
- Modify: `scripts/terminal-publish.md` (Usage, Behavior, Exit codes, Invariants)
- Test: `tests/test_closeout.sh`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: the flag `--enabled <true|false>` (default `true`) on `scripts/terminal-publish.sh`. Task 3's call sites pass `--enabled "$TERMINAL_PUBLISH"`; Task 2 produces that variable.

- [ ] **Step 1: Write the failing tests**

Append to `tests/test_closeout.sh`, immediately after the existing `--- terminal-publish.sh --adr: main-mode no-op ---` block:

```bash
# --- change 0064: --enabled false suppresses the publish (change shape) ---
read -r W _ < <(new_repo)
before="$(git -C "$W" rev-parse origin/main)"
( cd "$W" && "$PUBLISH" --id 7 --outcome done --integration-branch main --metadata-branch docket \
    --changes-dir docs/changes --adrs-dir docs/adrs --enabled false ) >/dev/null 2>&1
rc=$?
git -C "$W" fetch origin main >/dev/null 2>&1
after="$(git -C "$W" rev-parse origin/main)"
assert "0064 publish --enabled false: exits 0 (suppressed publish is success)" '[ "$rc" -eq 0 ]'
assert "0064 publish --enabled false: integration branch untouched" '[ "$before" = "$after" ]'
assert "0064 publish --enabled false: no pub worktree provisioned" '! git -C "$W" worktree list | grep -q "pub-7"'

# --- change 0064: --enabled false suppresses the ADR shape too (the docket-adr path) ---
read -r W _ < <(new_repo)
before="$(git -C "$W" rev-parse origin/main)"
( cd "$W" && "$PUBLISH" --adr 3 --integration-branch main --metadata-branch docket \
    --changes-dir docs/changes --adrs-dir docs/adrs --enabled false ) >/dev/null 2>&1
rc=$?
git -C "$W" fetch origin main >/dev/null 2>&1
after="$(git -C "$W" rev-parse origin/main)"
assert "0064 publish --adr --enabled false: exits 0" '[ "$rc" -eq 0 ]'
assert "0064 publish --adr --enabled false: no ADR file on integration branch" \
  '! git -C "$W" ls-tree -r --name-only origin/main | grep -q "docs/adrs/0003-accepted.md"'
assert "0064 publish --adr --enabled false: no ADR index on integration branch" \
  '! git -C "$W" ls-tree -r --name-only origin/main | grep -q "docs/adrs/README.md"'
assert "0064 publish --adr --enabled false: integration branch untouched" '[ "$before" = "$after" ]'

# --- change 0064: back-compat — omitting --enabled still publishes (default true) ---
read -r W _ < <(new_repo)
( cd "$W" && "$PUBLISH" --adr 3 --integration-branch main --metadata-branch docket \
    --changes-dir docs/changes --adrs-dir docs/adrs ) >/dev/null 2>&1
rc=$?
git -C "$W" fetch origin main >/dev/null 2>&1
assert "0064 publish: omitting --enabled defaults to true (publishes)" '[ "$rc" -eq 0 ]'
assert "0064 publish: default-true actually landed the ADR" \
  'git -C "$W" ls-tree -r --name-only origin/main | grep -q "docs/adrs/0003-accepted.md"'

# --- change 0064: an explicit --enabled true publishes exactly as today ---
read -r W _ < <(new_repo)
( cd "$W" && "$PUBLISH" --adr 3 --integration-branch main --metadata-branch docket \
    --changes-dir docs/changes --adrs-dir docs/adrs --enabled true ) >/dev/null 2>&1
rc=$?
git -C "$W" fetch origin main >/dev/null 2>&1
assert "0064 publish --enabled true: exits 0" '[ "$rc" -eq 0 ]'
assert "0064 publish --enabled true: ADR landed on integration branch" \
  'git -C "$W" ls-tree -r --name-only origin/main | grep -q "docs/adrs/0003-accepted.md"'

# --- change 0064: fail-closed — an unparseable --enabled value aborts ---
read -r W _ < <(new_repo)
( cd "$W" && "$PUBLISH" --id 7 --outcome done --integration-branch main --metadata-branch docket \
    --changes-dir docs/changes --adrs-dir docs/adrs --enabled maybe ) >/dev/null 2>&1
rc=$?
assert "0064 publish: unparseable --enabled exits non-zero (never coerced to true)" '[ "$rc" -ne 0 ]'

# --- change 0064: argument validation still fires when publishing is disabled ---
# A disabled publish must not mask a broken call site.
read -r W _ < <(new_repo)
( cd "$W" && "$PUBLISH" --id 7 --integration-branch main --metadata-branch docket \
    --changes-dir docs/changes --adrs-dir docs/adrs --enabled false ) >/dev/null 2>&1
rc=$?
assert "0064 publish: missing --outcome still aborts even with --enabled false" '[ "$rc" -ne 0 ]'
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `bash tests/test_closeout.sh 2>&1 | grep -E "^NOT OK - 0064"`
Expected: FAIL — the `--enabled` tests report `NOT OK`. The script currently `die`s with `unknown argument: --enabled`, so the `--enabled false` runs exit non-zero (failing the "exits 0" assertions) and the "unparseable value" test may pass for the wrong reason (unknown-arg, not validation).

- [ ] **Step 3: Add the flag, its validation, and the guard**

In `scripts/terminal-publish.sh`, update the usage header (the `# Usage (exactly one of --id / --adr):` block) to show the new flag on both shapes:

```bash
# Usage (exactly one of --id / --adr):
#   terminal-publish.sh --id N --outcome done|killed --integration-branch B --metadata-branch M
#                       --changes-dir REL --adrs-dir REL [--message MSG] [--remote R]
#                       [--enabled true|false]
#   terminal-publish.sh --adr NN --integration-branch B --metadata-branch M
#                       --changes-dir REL --adrs-dir REL [--message MSG] [--remote R]
#                       [--enabled true|false]
#
# --enabled false (change 0064: the per-repo `terminal_publish` knob) makes this script a no-op:
# the record stays on the metadata branch and nothing is committed onto the integration branch.
# Default true — omitting the flag behaves exactly as before the knob existed. The guard sits
# BEFORE the --id/--adr mode dispatch, so one guard covers BOTH publish shapes.
```

Add `ENABLED` to the defaults line:

```bash
ID="" ADR="" OUTCOME="" INT_BRANCH="" META_BRANCH="" CHANGES_DIR="" ADRS_DIR="" MESSAGE="" REMOTE="origin"
ENABLED="true"   # change 0064: default true == today's behavior
```

Add the flag to the arg loop, beside `--remote`:

```bash
    --remote) REMOTE="$2"; shift ;;
    --enabled) ENABLED="$2"; shift ;;
```

Add validation with the other argument checks, AFTER the `--changes-dir`/`--adrs-dir` check (so a malformed call still fails loudly whether or not publishing is enabled):

```bash
[ -n "$CHANGES_DIR" ] && [ -n "$ADRS_DIR" ]   || die "missing --changes-dir/--adrs-dir"
# change 0064: fail closed on an unparseable value — never silently coerce to true, which would
# publish onto the integration branch against the repo's stated intent.
case "$ENABLED" in true|false) ;; *) die "invalid --enabled: '$ENABLED' (expected true|false)" ;; esac
```

Add the guard immediately after the existing mode guard, BEFORE the `--- fetch the authoritative metadata remote tip ---` block (i.e. before the `--id`/`--adr` dispatch):

```bash
# Mode guard: main-mode has no docket branch to copy from.
if [ "$META_BRANCH" = "$INT_BRANCH" ]; then
  log "main-mode (metadata-branch == integration-branch); no-op"
  exit 0
fi

# Knob guard (change 0064): terminal_publish: false. A second no-op guard beside the mode guard,
# placed BEFORE the --id/--adr dispatch so it covers BOTH publish shapes. A suppressed publish is
# SUCCESS (exit 0) — callers trust the exit code, and close-out steps 4-5 (cleanup, board) still run.
if [ "$ENABLED" = false ]; then
  log "terminal_publish: false — skipping publish onto $INT_BRANCH; the record stays on $META_BRANCH"
  exit 0
fi
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `bash tests/test_closeout.sh 2>&1 | grep -E "0064|NOT OK"`
Expected: every `0064 …` assertion prints `ok - …`; no `NOT OK` lines anywhere in the file.

- [ ] **Step 5: Update the contract `scripts/terminal-publish.md`**

In **Usage**, add `[--enabled true|false]` to BOTH code blocks (change publish and ADR-only publish), then add below them:

```markdown
`--enabled` defaults to `true`. `--enabled false` (change 0064 — the per-repo `terminal_publish`
knob, resolved by `docket-config.sh`) makes the script a no-op. An unparseable value is rejected
before any git work, like `--id`/`--adr`.
```

In **Behavior**, add a subsection immediately after `### Mode guard`:

```markdown
### Knob guard (change 0064)

When `--enabled false`, the script logs a single skip line and exits 0 **before the `--id`/`--adr`
mode dispatch** — so the guard covers **both** publish shapes: the close-out change publish and
`docket-adr`'s standalone/status-changed ADR publish. Nothing is fetched, no worktree is
provisioned, and no commit reaches the integration branch; the archived change file, its spec, its
ADRs, and the integration-branch ADR index all stay on the metadata branch.

A suppressed publish is **success, not failure**: it exits 0, so a caller trusting the exit code
proceeds normally and the close-out's skip-publish guard does not fire — cleanup and the board
refresh still run.

Argument validation runs **before** this guard, so a malformed call fails loudly whether or not
publishing is enabled — a disabled publish never masks a broken call site.

The guard is inert in `main`-mode: the mode guard already exits 0 first.
```

In **Exit codes**, extend the `0` row:

```markdown
- `0` — the full copy-set landed on `origin/<integration_branch>` and the worktree was torn down
  cleanly; **or** the publish was suppressed (`--enabled false`, change 0064) or is a `main`-mode
  no-op. Callers should **trust the exit code**: non-zero means abort-and-report.
```

and add to the non-zero list: `invalid --enabled value`.

In **Invariants**, add:

```markdown
**The knob guard covers both publish shapes** — it precedes the `--id`/`--adr` dispatch, so
`terminal_publish: false` suppresses the close-out change publish AND `docket-adr`'s ADR publish.
Gating only one shape would let ADRs keep landing on the integration branch, defeating the knob.

**`--enabled false` is inert, not destructive** — it suppresses a *future* copy; it never removes
records already published to the integration branch by prior runs.
```

- [ ] **Step 6: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/optional-terminal-publish
git add scripts/terminal-publish.sh scripts/terminal-publish.md tests/test_closeout.sh
git commit -m "feat(0064): terminal-publish.sh --enabled gate covering both publish shapes"
```

---

### Task 2: `docket-config.sh` — the `terminal_publish` leaf, the fence, the emit

**Files:**
- Modify: `scripts/docket-config.sh` (fence loop ~143, resolution block ~164-169, emit block ~259-278)
- Modify: `scripts/docket-config.md` (classification table ~68-82, fence bullet ~107, key list ~198-217, exit-code table ~229-235)
- Test: `tests/test_docket_config.sh` (count assertions at lines 104 and 360; new sections)

**Interfaces:**
- Consumes: `--enabled` from Task 1 (documentation cross-reference only; no code dependency).
- Produces: `TERMINAL_PUBLISH=true|false` in `docket-config.sh --export`, emitted after `AUTO_GROOM` and before `SKILL_BRAINSTORM` (`BOOTSTRAP` stays last). Task 3's call sites read `$TERMINAL_PUBLISH`.

- [ ] **Step 1: Write the failing tests**

First fix the two existing count assertions — the export gains one line (learning #56: a literal count of an enumerated set drifts across prose and tests).

In `tests/test_docket_config.sh` line ~104, change:

```bash
assert "direct-pipe: 18 KEY=value lines emitted"       '[ "$n" -eq 18 ]'
```
to:
```bash
assert "direct-pipe: 19 KEY=value lines emitted"       '[ "$n" -eq 19 ]'
```

At line ~358-360, change:

```bash
# --- (E') emit-interface guard: still exactly 19 lines with a global file present ---
n50="$(rung "$tmp/k.xdg" "$tmp/k" --export | grep -c '=')"
assert "0050 E': still 19 KEY=value lines with global layer" '[ "$n50" -eq 19 ]'
```

Then add the default assertion to section (A), beside `absent cfg: AUTO_GROOM default false`:

```bash
assert "absent cfg: TERMINAL_PUBLISH default true"     '[ "$TERMINAL_PUBLISH" = true ]'
```

Then append a new section at the end of the file, before the final exit/summary lines:

```bash
# --- (0064) terminal_publish: repo-committed value honored; fenced in machine layers ---
mkrepo "$tmp/tp"
printf 'metadata_branch: docket\nterminal_publish: false\n' > "$tmp/tp/.docket.yml"
git -C "$tmp/tp" add .docket.yml; git -C "$tmp/tp" commit --quiet -m cfg
git -C "$tmp/tp" push --quiet origin main
out="$(run "$tmp/tp" --export)"; eval "$out"
assert "0064: repo terminal_publish false is honored" '[ "$TERMINAL_PUBLISH" = false ]'

# explicit true round-trips
mkrepo "$tmp/tp2"
printf 'metadata_branch: docket\nterminal_publish: true\n' > "$tmp/tp2/.docket.yml"
git -C "$tmp/tp2" add .docket.yml; git -C "$tmp/tp2" commit --quiet -m cfg
git -C "$tmp/tp2" push --quiet origin main
out="$(run "$tmp/tp2" --export)"; eval "$out"
assert "0064: repo terminal_publish true is honored" '[ "$TERMINAL_PUBLISH" = true ]'

# fence: a GLOBAL terminal_publish is warned-and-ignored, never honored, never fatal
mkrepo "$tmp/tp3"
mkdir -p "$tmp/tp3.xdg/docket"
printf 'terminal_publish: false\n' > "$tmp/tp3.xdg/docket/config.yml"
tperr="$(rung "$tmp/tp3.xdg" "$tmp/tp3" --export 2>&1 >/dev/null)"
out="$(rung "$tmp/tp3.xdg" "$tmp/tp3" --export 2>/dev/null)"; eval "$out"
assert "0064 fence: global terminal_publish warns"        'printf "%s" "$tperr" | grep -q "terminal_publish"'
assert "0064 fence: warning says per-repo-only"           'printf "%s" "$tperr" | grep -qi "per-repo-only"'
assert "0064 fence: global value NOT honored (stays true)" '[ "$TERMINAL_PUBLISH" = true ]'
assert "0064 fence: global terminal_publish is not fatal"  '[ "$(rung_rc "$tmp/tp3.xdg" "$tmp/tp3" --export)" -eq 0 ]'

# fence: a MACHINE-LOCAL .docket.local.yml terminal_publish is warned-and-ignored too
mkrepo "$tmp/tp4"
printf 'terminal_publish: false\n' > "$tmp/tp4/.docket.local.yml"
lerr="$(run "$tmp/tp4" --export 2>&1 >/dev/null)"
out="$(run "$tmp/tp4" --export 2>/dev/null)"; eval "$out"
assert "0064 fence: .docket.local.yml terminal_publish warns" 'printf "%s" "$lerr" | grep -q "terminal_publish"'
assert "0064 fence: local names .docket.local.yml"            'printf "%s" "$lerr" | grep -q ".docket.local.yml"'
assert "0064 fence: local value NOT honored (stays true)"     '[ "$TERMINAL_PUBLISH" = true ]'

# fail-closed: an unparseable repo value aborts (never silently coerced to true)
mkrepo "$tmp/tp5"
printf 'metadata_branch: docket\nterminal_publish: flase\n' > "$tmp/tp5/.docket.yml"
git -C "$tmp/tp5" add .docket.yml; git -C "$tmp/tp5" commit --quiet -m cfg
git -C "$tmp/tp5" push --quiet origin main
assert "0064: unparseable terminal_publish exits non-zero" \
  '! run "$tmp/tp5" --export >/dev/null 2>&1'
assert "0064: unparseable terminal_publish emits nothing"  \
  '[ -z "$(run "$tmp/tp5" --export 2>/dev/null)" ]'
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `bash tests/test_docket_config.sh 2>&1 | grep -E "^NOT OK"`
Expected: FAIL — the `19 KEY=value lines` assertions fail (only 18 emitted), `TERMINAL_PUBLISH` is unbound so the default/honored assertions fail, and the fence warnings are absent.

- [ ] **Step 3: Implement in `scripts/docket-config.sh`**

Add `terminal_publish` to the fence loop (line ~143) — it joins the existing coordination keys:

```bash
for _fkey in metadata_branch integration_branch changes_dir adrs_dir results_dir github_project terminal_publish; do
```

Add the resolution beside the other scalar knobs, after the `AUTO_GROOM` line (~169). Note it reads the repo-committed layer ONLY — no `lcl`/`gbl` fallbacks, because the key is fenced:

```bash
AUTO_GROOM="$(lcl auto_groom)"; AUTO_GROOM="${AUTO_GROOM:-$(yaml_get "$CFG" auto_groom)}"; AUTO_GROOM="${AUTO_GROOM:-$(gbl auto_groom)}"; AUTO_GROOM="${AUTO_GROOM:-false}"
# change 0064: coordination-key fenced — repo-committed .docket.yml ONLY (no lcl/gbl rungs; a
# machine-scoped value is warned-and-ignored by the Stage 2c fence above). Fail closed on garbage:
# silently defaulting a typo to `true` would publish onto the integration branch against intent.
TERMINAL_PUBLISH="$(yaml_get "$CFG" terminal_publish)"; TERMINAL_PUBLISH="${TERMINAL_PUBLISH:-true}"
case "$TERMINAL_PUBLISH" in
  true|false) ;;
  *) die "unparseable .docket.yml: terminal_publish must be 'true' or 'false', got '$TERMINAL_PUBLISH'" ;;
esac
```

Add the emit after `AUTO_GROOM` (keeping `BOOTSTRAP` last):

```bash
  emit AUTO_GROOM "$AUTO_GROOM"
  emit TERMINAL_PUBLISH "$TERMINAL_PUBLISH"
  emit SKILL_BRAINSTORM "$SKILL_BRAINSTORM"
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `bash tests/test_docket_config.sh 2>&1 | grep -E "0064|NOT OK"`
Expected: every `0064 …` assertion prints `ok - …`; no `NOT OK` lines.

- [ ] **Step 5: Update the contract `scripts/docket-config.md`**

Add the classification-table row after `auto_groom` (~line 78):

```markdown
| `terminal_publish` | `true` | no (fenced) | `true`/`false`; `false` makes `terminal-publish.sh` a no-op for BOTH shapes — archived change files, specs, and ADRs stay on the metadata branch. Anything else aborts |
```

Extend the coordination-key fence bullet (~line 107):

```markdown
- **Coordination-key fence:** `metadata_branch`, `integration_branch`, `changes_dir`,
  `adrs_dir`, `results_dir`, `github_project`, `terminal_publish` set in the global layer OR in
  `.docket.local.yml` → each warned "per-repo-only" (naming which file) and ignored.
```

Add `TERMINAL_PUBLISH` to the emitted key list, after `AUTO_GROOM`, and change the count line:

```markdown
19 lines total. The last line is always `BOOTSTRAP=…`.
```

Add to the exit-code table:

```markdown
| `terminal_publish` is neither `true` nor `false` | 1 |
```

- [ ] **Step 6: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/optional-terminal-publish
git add scripts/docket-config.sh scripts/docket-config.md tests/test_docket_config.sh
git commit -m "feat(0064): docket-config.sh resolves + fences terminal_publish, emits TERMINAL_PUBLISH"
```

---

### Task 3: Call-site wiring — close-out reference + `docket-adr`

**Files:**
- Modify: `skills/docket-convention/references/terminal-close-out.md` (preamble ~5-9, step 3 ~51-62)
- Modify: `skills/docket-adr/SKILL.md` (the two `--adr` invocations, ~57 and ~65; the `main`-mode line ~70)
- Test: `tests/test_closeout.sh` (structural wiring assertions)

**Interfaces:**
- Consumes: `--enabled` (Task 1) and `$TERMINAL_PUBLISH` (Task 2).
- Produces: the guarantee that **every** documented `terminal-publish.sh` invocation passes `--enabled "$TERMINAL_PUBLISH"` — enforced by a structural test so a future call site cannot silently reintroduce an ungated publish.

- [ ] **Step 1: Write the failing structural tests**

Append to `tests/test_closeout.sh`, in the structural/wiring section at the end (where `$TCO` — the terminal-close-out reference — is already asserted against):

```bash
# --- change 0064: every documented terminal-publish call site passes --enabled ---
ADRSKILL="$REPO/skills/docket-adr/SKILL.md"
assert "0064 wiring: close-out step 3 passes --enabled" \
  'grep -q -- "--enabled" "$TCO"'
assert "0064 wiring: close-out step 3 passes the TERMINAL_PUBLISH value" \
  'grep -q -- "--enabled \"\$TERMINAL_PUBLISH\"" "$TCO"'
assert "0064 wiring: docket-adr passes --enabled on BOTH --adr call sites" \
  '[ "$(grep -c -- "--enabled \"\$TERMINAL_PUBLISH\"" "$ADRSKILL")" -eq 2 ]'
# The invariant: no documented invocation may omit the gate. Every line that invokes
# terminal-publish.sh in the skill/reference prose must carry --enabled.
assert "0064 wiring: no ungated terminal-publish invocation in skills/" \
  '[ -z "$(grep -rn "terminal-publish\.sh --\(id\|adr\)" "$REPO/skills/" | grep -v -- "--enabled")" ]'

# The close-out contract: a SUPPRESSED publish is success, so it must not trip the skip-publish
# guard — cleanup (step 4) and the board refresh (step 5) still run. This is the spec's
# "close-out integration" requirement; the sequence is skill-driven prose, so it is asserted here
# as a contract sentinel rather than an executable close-out fixture.
assert "0064 wiring: close-out states a suppressed publish does not skip steps 4-5" \
  'grep -qi "does NOT trip the\|not trip the skip-publish" "$TCO"'
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `bash tests/test_closeout.sh 2>&1 | grep -E "^NOT OK - 0064 wiring"`
Expected: FAIL — all four wiring assertions report `NOT OK`; no call site passes `--enabled` yet.

- [ ] **Step 3: Gate step 3 in `terminal-close-out.md`**

Replace the step-3 invocation block and its prose:

````markdown
3. **Publish the terminal record.** Reached only after the step-2 commit is on `origin/docket`.
   **Gated by `TERMINAL_PUBLISH`** (change 0064) — pass the resolved value straight through:

   ```
   "${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/terminal-publish.sh --id <id> --outcome <done|killed> \
     --integration-branch <integration_branch> --metadata-branch docket \
     --changes-dir <changes_dir> --adrs-dir <adrs_dir> --message "<msg>" \
     --enabled "$TERMINAL_PUBLISH"
   ```

   Copies the archived change file + its `spec:` (if set) + the **`Accepted`** ADRs in `adrs:`
   from `origin/docket` onto the integration branch in one dedicated commit — the only flow of
   metadata onto the code line. Trust the exit code; its reuse-existing-file idempotency makes two
   drivers racing on the same change a safe no-op.

   When the repo sets `terminal_publish: false`, the script is a **no-op that exits 0** — the
   record stays on `docket`, and a suppressed publish is *success*: it does NOT trip the
   skip-publish guard, so steps 4–5 still run. Callers pass the flag and keep trusting the exit
   code; no caller branches on the knob itself.
````

Also correct the now-stale preamble (0054/0055 shipped on 2026-07-11; all four drivers route through this file):

```markdown
> Single source for the close-out sequence a terminal transition (`done` or `killed`) runs:
> archive → re-render `## Artifacts` → terminal-publish → cleanup → board. All four drivers route
> through this file: `docket-finalize-change`'s per-change close-out and `docket-status`'s merge
> sweep (the two `done` drivers), plus the kill callers — `docket-implement-next`'s reconcile-kill
> and `docket-new-change`'s proposed-kill (rewired onto this file by changes 0054/0055). The
> sequence is one; only the failure posture differs per caller (table below). This file owns
> ordering and posture; each script's mechanics live in its co-located contract
> (`scripts/<name>.md`).
```

In the **main-mode degradation** section, append one sentence:

```markdown
The `terminal_publish` knob (change 0064) is likewise inert in `main`-mode — the mode guard already
makes the publish a no-op, so there is no surface for the knob to act on.
```

- [ ] **Step 4: Gate both `docket-adr` call sites**

In `skills/docket-adr/SKILL.md`, replace BOTH invocation blocks (the standalone-ADR publish and the status-change re-publish) with the gated form — the same text in both places:

```
"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/terminal-publish.sh --adr <NN> --integration-branch <integration_branch> --metadata-branch <metadata_branch> --changes-dir <changes_dir> --adrs-dir <adrs_dir> --enabled "$TERMINAL_PUBLISH"
```

Then, immediately before the existing `main`-mode paragraph that closes the section, add:

```markdown
All three cases are **gated by `TERMINAL_PUBLISH`** (change 0064): the same `--enabled` flag the close-out passes. In a repo that sets `terminal_publish: false`, the ADR publish is a no-op that exits 0 — the decision ledger lives on `docket` only, and the integration branch carries no ADR files and no ADR index. Trust the exit code either way; do not branch on the knob.
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `bash tests/test_closeout.sh 2>&1 | grep -E "0064|NOT OK"`
Expected: all `0064 wiring` assertions print `ok - …`; no `NOT OK` lines.

- [ ] **Step 6: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/optional-terminal-publish
git add skills/docket-convention/references/terminal-close-out.md skills/docket-adr/SKILL.md tests/test_closeout.sh
git commit -m "feat(0064): gate every terminal-publish call site on TERMINAL_PUBLISH"
```

---

### Task 4: Surface the knob end-to-end (convention, README, sample config)

> Learning #49 (2026-07-09, PR #58): *a new config knob is not done when it merely works — ship it
> end-to-end in the same change: add it (commented, with every option) to the sample `.docket.yml`,
> document it in README, and update any prose that stated the now-relaxed requirement as absolute.*
> That change shipped its logic but not its surfacing, and the human caught it at the merge gate.
> This task is that lesson applied.

**Files:**
- Modify: `skills/docket-convention/SKILL.md` (`.docket.yml` schema block ~20-40; the fence sentence in **Config layers** ~45; the Branch model terminal-publish sentence)
- Modify: `README.md` (the config sample ~285-292; **Coordination keys are per-repo-only** ~298)
- Modify: `.docket.yml` (this repo's own committed config — a commented sample entry)
- Test: `tests/test_docket_config.sh` / `tests/test_sync_agents.sh` already assert the fence prose; no new test needed beyond a doc-sentinel below.

**Interfaces:**
- Consumes: `TERMINAL_PUBLISH` (Task 2), the gated call sites (Task 3).
- Produces: user-facing documentation. No code depends on this task.

- [ ] **Step 1: Add the knob to the convention's `.docket.yml` schema block**

In `skills/docket-convention/SKILL.md`, add the key to the documented schema, after `board_surfaces`:

```yaml
board_surfaces: [inline]     # which derived board view(s) to render: inline (BOARD.md) and/or github; [] = none
terminal_publish: true       # true (default) = copy terminal records (change file, spec, Accepted ADRs)
                             # onto the integration branch at close-out. false = keep them on the
                             # metadata branch only — for repos where every write to the integration
                             # branch must go through a PR. Per-repo-only (coordination-key fenced).
```

- [ ] **Step 2: Add it to the fence sentence in *Config layers***

Extend the coordination-key list so the fenced set stays accurate in the convention prose:

```markdown
**Coordination-key fence:** a key whose effect writes shared, non-re-derivable state (`metadata_branch`, `integration_branch`, `changes_dir`/`adrs_dir`/`results_dir`, `github_project`, `terminal_publish`, and `board_surfaces`' `github` token) is per-repo-only — set in either machine-scoped file it is loudly warned-and-ignored, never honored, never fatal (the classification rule is ADR-0019).
```

- [ ] **Step 3: Note the opt-out in the Branch model's terminal close-out sentence**

In the **Branch model** section, append to the paragraph describing terminal close-out / terminal-publish:

```markdown
A repo may set **`terminal_publish: false`** (per-repo-only; change 0064) to suppress that copy entirely — the archived change file, its spec, and its `Accepted` ADRs then stay on `metadata_branch`, and the integration branch receives only code, plans, and results through the normal PR merge. The knob gates both publish shapes (change close-out and `docket-adr`'s ADR publish); it is inert in `main`-mode.
```

- [ ] **Step 4: Document it in `README.md`**

Add `terminal_publish` to the coordination-key list in **Coordination keys are per-repo-only**:

```markdown
These keys — `metadata_branch`, `integration_branch`, `changes_dir`, `adrs_dir`, `results_dir`, `github_project`, `terminal_publish`, and the `github` token of `board_surfaces` — are therefore **per-repo-only**: they are ignored with a loud warning when set globally **or** in a repo's `.docket.local.yml`. Set them in the repo's committed `.docket.yml` only.
```

Then add a short subsection under the docket-mode / metadata documentation (immediately before or after **docket-mode: where metadata lives**):

```markdown
### Keeping metadata off the integration branch (`terminal_publish`)

By default, when a change reaches a terminal state docket copies its record — the archived change
file, its spec, and its `Accepted` ADRs — from the `docket` branch onto the integration branch in a
direct commit. In a repo where **every** write to the integration branch is expected to go through a
pull request, that direct commit fights the workflow.

Set `terminal_publish: false` in the repo's committed `.docket.yml` to suppress it:

```yaml
terminal_publish: false   # keep change files, specs, and ADRs on the docket branch
```

Then the integration branch accumulates **only** code, plans, and results — all through PRs — while
change files, specs, and ADRs live on `docket`, fully browsable there. The knob gates both publish
shapes: the change close-out *and* `docket-adr`'s ADR publish. It is **per-repo-only** (a
machine-scoped value is warned-and-ignored), because the headless `docket-status` merge sweep must
see the same policy as every other agent. It is inert in `main`-mode, and it never retroactively
removes records already published.
```

- [ ] **Step 5: Add the commented sample entry to this repo's `.docket.yml`**

This repo keeps the default (`true`), so the entry stays commented — but it must be discoverable. Add after the `board_surfaces` block:

```yaml
# Terminal-publish opt-out (change 0064). Default true: on a terminal transition, the archived
# change file + its spec + its Accepted ADRs are copied from `docket` onto the integration branch
# in a direct commit (and docket-adr publishes ADRs the same way). Set false in a repo where every
# write to the integration branch must go through a PR — records then stay on `docket` only, and
# the integration branch gets code/plans/results via PRs alone. Per-repo-only (coordination-key
# fenced): a value in the global config or .docket.local.yml is warned-and-ignored. Inert in
# main-mode. This repo publishes its terminal records, so the default stands.
# terminal_publish: true
```

- [ ] **Step 6: Add a doc-sentinel so the surfacing cannot silently rot**

Append to `tests/test_docket_config.sh` (end of file, before the summary):

```bash
# --- (0064) surfacing: the knob is documented end-to-end (learning #49) ---
CONV_SKILL="$REPO/skills/docket-convention/SKILL.md"
assert "0064 doc: convention schema block documents terminal_publish" \
  'grep -q "terminal_publish" "$CONV_SKILL"'
assert "0064 doc: convention fence list includes terminal_publish" \
  'grep -q "terminal_publish" <<<"$(grep -A2 "Coordination-key fence" "$CONV_SKILL")"'
assert "0064 doc: README documents terminal_publish" \
  'grep -q "terminal_publish" "$REPO/README.md"'
assert "0064 doc: sample .docket.yml carries the commented knob" \
  'grep -q "terminal_publish" "$REPO/.docket.yml"'
assert "0064 doc: config contract classifies terminal_publish as fenced" \
  'grep -q "terminal_publish" "$REPO/scripts/docket-config.md"'
```

- [ ] **Step 7: Run the tests to verify they pass**

Run: `bash tests/test_docket_config.sh 2>&1 | grep -E "0064 doc|NOT OK"`
Expected: every `0064 doc` assertion prints `ok - …`; no `NOT OK` lines.

- [ ] **Step 8: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/optional-terminal-publish
git add skills/docket-convention/SKILL.md README.md .docket.yml tests/test_docket_config.sh
git commit -m "docs(0064): surface terminal_publish end-to-end (convention, README, sample config)"
```

---

## Final verification

- [ ] **Run the WHOLE suite, not just the touched tests.**

> Learning #54/#42: a goal-scoped change can pass its own enumerated sentinels while reddening a
> test *outside* the anticipated set — the spec's list is a floor. Run everything.

```bash
cd /Users/homer/dev/docket/.worktrees/optional-terminal-publish
for t in tests/test_*.sh; do
  echo "=== $t ==="
  bash "$t" 2>&1 | grep -E "^NOT OK" && echo "^^ FAILURES IN $t"
done; echo "SUITE SWEEP COMPLETE"
```

Expected: no `NOT OK` lines from any test file.

Pay particular attention to suites that assert on the config export shape or the fence prose, which this change moves:
- `tests/test_docket_config.sh` — the 18→19 count.
- `tests/test_sync_agents.sh` — asserts the convention states the coordination-key fence (lines ~823, ~833).
- `tests/test_script_contracts_coverage.sh` — every script has a co-located contract.
- `tests/test_closeout.sh` — publish behavior + call-site wiring.
- `tests/test_docket_metadata_branch.sh` — asserts terminal-publish is skipped in main-mode.
- `tests/test_convention_extraction.sh` — convention prose structure.

- [ ] **Confirm back-compat by inspection:** a repo with no `terminal_publish` key resolves
  `TERMINAL_PUBLISH=true`, every call site passes `--enabled true`, and `terminal-publish.sh`
  behaves exactly as before the knob existed.

## Notes for the implementer

- **Never edit `docs/adrs/0019-*.md`.** An `Accepted` ADR is immutable except its `status:` line.
  The new fence classification + conditional-publish rule is recorded in a NEW ADR, authored after
  the build via the `docket-adr` subagent (docket-implement-next step 6).
- **`skills/` is source here.** This repo *is* docket, so the skill markdown files are the product,
  not just docs — edits to them are real changes and are tested by the suites above.
- **Do not touch the change file or `BOARD.md`.** They live on the `docket` branch (metadata), never
  on this feature branch.
