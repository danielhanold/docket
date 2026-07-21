# `finalize.require_pr_approval` Layer Resolution Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire `finalize.require_pr_approval` through `docket-config.sh`'s layer resolution as a genuinely global-able, fail-closed boolean, make the finalize skill read the exported value as its sole channel, and add a manifest guard binding every key documented in `.docket.example.yml` to either a real export or a named non-resolver consumer.

**Architecture:** Four surfaces move together. (1) `scripts/docket-config.sh` gains a four-rung resolution chain (`lcl` → repo `yaml_get` → `gbl` → built-in `false`) plus a `true|false` validation that `die`s, and emits `FINALIZE_REQUIRE_PR_APPROVAL`. (2) `scripts/docket-config.md` — the resolver's authoritative contract — gains the table row, the export-list entry, and the corrected line counts. (3) `skills/docket-finalize-change/SKILL.md` reads the exported value instead of parsing `.docket.yml` by eye. (4) `tests/test_docket_example_yml.sh` gains a classification manifest that replaces the hand-written `(2b)` allowlist entry for this key and turns "documented but unresolved" into a red test.

**Tech Stack:** POSIX-ish `bash` (the repo's scripts run under `set -uo pipefail`, no `set -e`), `sed`/`awk`/`grep` ERE, hand-rolled assert harnesses in `tests/*.sh` (no framework), markdown skill bodies read by the model.

## Global Constraints

- **Shell portability.** Scripts target bash but avoid GNU-only flags; `sed -E` for ERE. Match the existing style in `scripts/docket-config.sh` exactly — the new chain is a visual sibling of the two lines above it.
- **No `set -e` in tests.** `tests/*.sh` run with `set -uo pipefail` only, and use `assert "<name>" '<shell expr>'`. A failing assert sets `fail=1`; the file ends `exit $fail`.
- **Fail closed on a non-boolean.** The diagnostic must name the key. House precedent is `auto_capture` (`scripts/docket-config.sh:210-213`): `die "unparseable config: <key> must be 'true' or 'false', got '<value>'"`.
- **`require_pr_approval` is deliberately NOT coordination-fenced.** It must never be added to the `for _fkey in …` loop at `scripts/docket-config.sh:169`. Global-able is the entire point of the change.
- **Export order is a contract.** `FINALIZE_REQUIRE_PR_APPROVAL` goes immediately after `FINALIZE_TEST_COMMAND` in both the `emit` block and the contract doc's list. Export line counts go **24 → 25** (`shell`) and **25 → 26** (`plain`).
- **Exact filename:** `.docket.example.yml` (the spec text alternates with `.docket.yml.example`; only `.docket.example.yml` exists).
- **Never weaken an existing assert to make a change pass.** If an existing assert reddens, that is the guard working — update it deliberately and say why in the commit message.
- **Commit messages** end with the trailer:
  ```
  Claude-Session: https://claude.ai/code/session_01EoXH1GHEjjXk7HDVD4ze4w
  ```

**Not a task in this plan:** the ADR from spec §7 is recorded on the metadata branch by the `docket-adr` subagent during the implementer's review step, never as a feature-branch file. Do not create `docs/adrs/*.md` here.

---

## File Structure

| File | Change | Responsibility |
|---|---|---|
| `scripts/docket-config.sh` | Modify (~`:93` comment, after `:194`, after `:411`) | Resolve + validate + emit the key |
| `scripts/docket-config.md` | Modify (table ~`:104`, list ~`:275`, counts `:292`, `:343`) | The resolver's authoritative contract |
| `tests/test_docket_config.sh` | Modify (`:143`, `:402`, new section) | Resolution, fail-closed, not-fenced, export-count coverage |
| `skills/docket-finalize-change/SKILL.md` | Modify (`:108`, `:120`, + behavioral mentions) | Read the export as sole channel |
| `.docket.example.yml` | Modify (`:93-101`) | Collapse the bespoke scope note to the standard tag |
| `tests/test_docket_example_yml.sh` | Modify (`:88-120`, `:122-147`, new manifest) | The documented-key → resolved-key drift guard |

---

### Task 1: Resolve, validate, and emit the key

**Files:**
- Modify: `scripts/docket-config.sh:93` (comment), `scripts/docket-config.sh:194` (add chain after), `scripts/docket-config.sh:411` (add emit after)
- Modify: `scripts/docket-config.md:104` (table row), `:275` (export list), `:292` + `:343` (counts)
- Test: `tests/test_docket_config.sh` (new section + `:143`, `:402` count bumps)
- Test: `tests/test_docket_example_yml.sh:88-112` (`map_for` entry — see Step 9, this is **required in this task**)

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: the exported variable name `FINALIZE_REQUIRE_PR_APPROVAL` (values: exactly the strings `true` or `false`), consumed by Task 3's skill body and Task 4's manifest. The resolver helpers `lcl <leafkey>` and `gbl <leafkey>` (defined at `scripts/docket-config.sh:153` and `:140`) read the machine-local and global layers respectively; `yaml_get "$CFG" <leafkey>` reads the repo-committed layer.

- [ ] **Step 1: Write the failing resolution tests**

Append to `tests/test_docket_config.sh`, immediately before its final `exit $fail` line (find it with `tail -3 tests/test_docket_config.sh`):

```bash
# ============================================================================
# Change 0102 — finalize.require_pr_approval layer resolution
# ============================================================================
# The key was documented (README, .docket.example.yml, the finalize SKILL) but resolved NOWHERE:
# a value in .docket.local.yml or the global config was neither honored nor warned-and-ignored.
# It is deliberately NOT coordination-fenced — global-able is the point (see (R4) below).

# --- (R1) built-in default when unset in every layer -------------------------
mkrepo "$tmp/r1"
out="$(rung "$tmp/r1.xdg" "$tmp/r1" --export)"; eval "$out"
assert "0102 R1: unset everywhere -> built-in false" \
  '[ "$FINALIZE_REQUIRE_PR_APPROVAL" = false ]'

# --- (R2) each layer honored, and the precedence between them ----------------
# Global only.
mkrepo "$tmp/r2"
mkdir -p "$tmp/r2.xdg/docket"
printf 'finalize:\n  require_pr_approval: true\n' > "$tmp/r2.xdg/docket/config.yml"
out="$(rung "$tmp/r2.xdg" "$tmp/r2" --export)"; eval "$out"
assert "0102 R2: global finalize.require_pr_approval honored" \
  '[ "$FINALIZE_REQUIRE_PR_APPROVAL" = true ]'

# Repo-committed beats global.
mkrepo "$tmp/r3"
printf 'metadata_branch: main\nfinalize:\n  require_pr_approval: false\n' > "$tmp/r3/.docket.yml"
git -C "$tmp/r3" add .docket.yml; git -C "$tmp/r3" commit --quiet -m cfg
git -C "$tmp/r3" push --quiet origin main
mkdir -p "$tmp/r3.xdg/docket"
printf 'finalize:\n  require_pr_approval: true\n' > "$tmp/r3.xdg/docket/config.yml"
out="$(rung "$tmp/r3.xdg" "$tmp/r3" --export)"; eval "$out"
assert "0102 R3: repo-committed false beats global true" \
  '[ "$FINALIZE_REQUIRE_PR_APPROVAL" = false ]'

# Repo-local beats repo-committed (and global).
mkrepo "$tmp/r4"
printf 'metadata_branch: main\nfinalize:\n  require_pr_approval: false\n' > "$tmp/r4/.docket.yml"
git -C "$tmp/r4" add .docket.yml; git -C "$tmp/r4" commit --quiet -m cfg
git -C "$tmp/r4" push --quiet origin main
printf 'finalize:\n  require_pr_approval: true\n' > "$tmp/r4/.docket.local.yml"
out="$(rung "$tmp/r4.xdg" "$tmp/r4" --export)"; eval "$out"
assert "0102 R4: repo-local true beats repo-committed false" \
  '[ "$FINALIZE_REQUIRE_PR_APPROVAL" = true ]'

# --- (R5) NOT coordination-fenced: machine layers are HONORED and UNWARNED ---
# The direct inverse of the fenced-key assertions at (0051 L3). This is the assert that would
# have caught the original bug, and the one that reddens if someone "helpfully" adds the key to
# the fence loop at scripts/docket-config.sh:169.
errout="$(XDG_CONFIG_HOME="$tmp/r4.xdg" bash "$SCRIPT" --repo-dir "$tmp/r4" --export 2>&1 >/dev/null)"
assert "0102 R5: no per-repo-only warning for require_pr_approval" \
  '! grep -q "require_pr_approval" <<<"$errout"'
assert "0102 R5: the key is absent from the coordination-key fence loop" \
  '! sed -n "/^for _fkey in /p" "$SCRIPT" | grep -q "require_pr_approval"'

# --- (R6) fail closed on a non-boolean --------------------------------------
mkrepo "$tmp/r6"
printf 'metadata_branch: main\nfinalize:\n  require_pr_approval: yes\n' > "$tmp/r6/.docket.yml"
git -C "$tmp/r6" add .docket.yml; git -C "$tmp/r6" commit --quiet -m cfg
git -C "$tmp/r6" push --quiet origin main
rc6="$(rung_rc "$tmp/r6.xdg" "$tmp/r6" --export)"
err6="$(XDG_CONFIG_HOME="$tmp/r6.xdg" bash "$SCRIPT" --repo-dir "$tmp/r6" --export 2>&1 >/dev/null)"
assert "0102 R6: non-boolean aborts (non-zero exit)"        '[ "$rc6" != "0" ]'
assert "0102 R6: diagnostic names the key"                  'grep -q "require_pr_approval" <<<"$err6"'
assert "0102 R6: diagnostic shows the offending value"      'grep -q "yes" <<<"$err6"'
assert "0102 R6: no KEY=value block on the abort path" \
  '[ -z "$(XDG_CONFIG_HOME="$tmp/r6.xdg" bash "$SCRIPT" --repo-dir "$tmp/r6" --export 2>/dev/null)" ]'

# --- (R7) export presence and POSITION --------------------------------------
# Position matters: scripts/docket-config.md documents the order as a contract, and pipe
# consumers may rely on it. Anchor on the neighbour rather than a bare "is present".
out7="$(rung "$tmp/r1.xdg" "$tmp/r1" --export)"
assert "0102 R7: FINALIZE_REQUIRE_PR_APPROVAL is emitted" \
  'grep -q "^FINALIZE_REQUIRE_PR_APPROVAL=" <<<"$out7"'
assert "0102 R7: emitted directly after FINALIZE_TEST_COMMAND" \
  '[ "$(grep -n "^FINALIZE_REQUIRE_PR_APPROVAL=" <<<"$out7" | cut -d: -f1)" \
     = "$(( $(grep -n "^FINALIZE_TEST_COMMAND=" <<<"$out7" | cut -d: -f1) + 1 ))" ]'
assert "0102 R7: present in plain format too" \
  'rung "$tmp/r1.xdg" "$tmp/r1" --export --format plain | grep -q "^FINALIZE_REQUIRE_PR_APPROVAL="'

# --- (R8) the contract doc documents it -------------------------------------
assert "0102 R8: docket-config.md has a require_pr_approval table row" \
  'grep -q "require_pr_approval" "$REPO/scripts/docket-config.md"'
assert "0102 R8: docket-config.md lists the export name" \
  'grep -q "^FINALIZE_REQUIRE_PR_APPROVAL$" "$REPO/scripts/docket-config.md"'
```

Note: `$REPO` is used by the `(R8)` asserts. Confirm it is already defined near the top of `tests/test_docket_config.sh`; if the variable there is named differently (e.g. `$ROOT`), use that name instead — check with `grep -n 'REPO=\|ROOT=\|SCRIPT=' tests/test_docket_config.sh | head -5`.

- [ ] **Step 2: Run the new tests to verify they fail**

Run: `bash tests/test_docket_config.sh 2>&1 | grep -E '^(NOT OK|ok) - 0102'`

Expected: the `0102 R*` asserts appear as `NOT OK` — in particular `R1` fails because `FINALIZE_REQUIRE_PR_APPROVAL` is unset (an `eval` of the export block never defines it, so `[ "$FINALIZE_REQUIRE_PR_APPROVAL" = false ]` errors under `set -u`).

If `R1` instead errors the whole file out with `unbound variable`, that confirms the same thing — proceed.

- [ ] **Step 3: Add the resolution chain**

In `scripts/docket-config.sh`, immediately after the `FINALIZE_TEST_COMMAND` `auto`-sentinel line (`[ "$FINALIZE_TEST_COMMAND" = auto ] && FINALIZE_TEST_COMMAND=""`, currently `:201`), insert:

```sh
# change 0102: require_pr_approval — the human-sign-off half of the merge gate (ADR-0011).
# Global-able, deliberately NOT coordination-fenced: `finalize.gate` — already global-able and
# gating the very same merge — is the governing precedent, and splitting the two halves of one
# merge gate across opposite scope classes would be the harder thing to explain. Per-machine
# divergence here is a policy the maintainer chose per machine, never a split backlog.
# Fails CLOSED on a non-boolean (the auto_capture / terminal_publish precedent): defaulting a
# typo to `false` would DISARM a gate the user believes is armed — the exact failure this change
# exists to eliminate.
FINALIZE_REQUIRE_PR_APPROVAL="$(lcl require_pr_approval)"
FINALIZE_REQUIRE_PR_APPROVAL="${FINALIZE_REQUIRE_PR_APPROVAL:-$(yaml_get "$CFG" require_pr_approval)}"
FINALIZE_REQUIRE_PR_APPROVAL="${FINALIZE_REQUIRE_PR_APPROVAL:-$(gbl require_pr_approval)}"
FINALIZE_REQUIRE_PR_APPROVAL="${FINALIZE_REQUIRE_PR_APPROVAL:-false}"
case "$FINALIZE_REQUIRE_PR_APPROVAL" in
  true|false) ;;
  *) die "unparseable config: finalize.require_pr_approval must be 'true' or 'false', got '$FINALIZE_REQUIRE_PR_APPROVAL'" ;;
esac
```

- [ ] **Step 4: Update the `yaml_get` leaf-read comment**

In `scripts/docket-config.sh`, the comment block above `yaml_get()` currently reads (at `:93`):

```
# unintended lines. Nested finalize.gate / finalize.test_command are read by their unique
```

Change that line to:

```
# unintended lines. Nested finalize.gate / finalize.test_command / finalize.require_pr_approval
# are read by their unique
```

...merging cleanly with the following line (`# leaf-key name. NOTE: a value may not contain a literal '#' …`). Read the surrounding four lines first and re-flow so the sentence stays grammatical — the goal is that the comment names all three leaf-read finalize keys.

- [ ] **Step 5: Add the emit line**

In `scripts/docket-config.sh`, immediately after `emit FINALIZE_TEST_COMMAND "$FINALIZE_TEST_COMMAND"` (currently `:411`), insert:

```sh
  emit FINALIZE_REQUIRE_PR_APPROVAL "$FINALIZE_REQUIRE_PR_APPROVAL"
```

Mind the two-space indentation — the `emit` calls sit inside a function body.

- [ ] **Step 6: Bump the two export-count asserts**

In `tests/test_docket_config.sh`:

At `:143`, change:
```bash
assert "direct-pipe: 24 KEY=value lines emitted"       '[ "$n" -eq 24 ]'
```
to:
```bash
assert "direct-pipe: 25 KEY=value lines emitted"       '[ "$n" -eq 25 ]'
```

At `:400-402`, change the comment and assert:
```bash
# --- (E') emit-interface guard: still exactly 25 lines with a global file present ---
n50="$(rung "$tmp/k.xdg" "$tmp/k" --export | grep -c '=')"
assert "0050 E': still 25 KEY=value lines with global layer" '[ "$n50" -eq 25 ]'
```

- [ ] **Step 7: Update the contract doc — table row**

In `scripts/docket-config.md`, insert this row immediately after the `test_command` (finalize) row:

```
| `require_pr_approval` (finalize) | `false` | yes | read from `finalize.require_pr_approval` leaf key; resolves repo-local > repo-committed > global; `true`/`false`, anything else aborts. Deliberately **not** coordination-fenced — `finalize.gate` is the precedent: both halves of one merge gate share a scope class (change 0102) |
```

- [ ] **Step 8: Update the contract doc — export list and counts**

In `scripts/docket-config.md`:

1. In the fenced export list (~`:264-288`), add `FINALIZE_REQUIRE_PR_APPROVAL` on its own line directly after `FINALIZE_TEST_COMMAND`.
2. At `:292`, change `24 lines in \`shell\` format; 25 in \`plain\` format,` to `25 lines in \`shell\` format; 26 in \`plain\` format,`.
3. At `:343`, change `- **24 \`KEY=value\` lines always emitted in the same order in \`shell\` format (25 in \`plain\`,` to `- **25 \`KEY=value\` lines always emitted in the same order in \`shell\` format (26 in \`plain\`,`.

Verify no other literal counts remain: `grep -n '24 \|25 ' scripts/docket-config.md | grep -i 'line\|format'`

- [ ] **Step 9: Add the `map_for` entry (required — the export guard reddens without it)**

Adding the export makes `tests/test_docket_example_yml.sh`'s `(2a)` loop iterate `FINALIZE_REQUIRE_PR_APPROVAL`, and `map_for` returns empty for it, so `completeness: export key FINALIZE_REQUIRE_PR_APPROVAL is mapped` goes **NOT OK**. That is the correspondence guard working as designed — satisfy it here rather than deferring.

In `tests/test_docket_example_yml.sh`, inside `map_for()`, add after the `FINALIZE_TEST_COMMAND)` line:

```bash
    FINALIZE_REQUIRE_PR_APPROVAL) echo '^[[:space:]]+require_pr_approval:[[:space:]]*false[[:space:]]*$' ;;
```

- [ ] **Step 10: Run the full affected suites to verify green**

Run: `bash tests/test_docket_config.sh 2>&1 | tail -20; echo "rc=$?"`
Expected: no `NOT OK` lines; every `0102 R*` assert reads `ok -`.

Run: `bash tests/test_docket_example_yml.sh 2>&1 | grep -E 'NOT OK|FINALIZE_REQUIRE'`
Expected: the two `FINALIZE_REQUIRE_PR_APPROVAL` completeness asserts read `ok -`, and no `NOT OK` lines appear.

- [ ] **Step 11: Mutation-test the fail-closed path**

Prove `R6` is non-vacuous — temporarily relax the validation and confirm the test reddens:

```bash
cp scripts/docket-config.sh /tmp/dc.bak
# Neuter the new case block's die arm:
perl -0pi -e "s/\*\) die \"unparseable config: finalize\.require_pr_approval[^\n]*\n/*) : ;;\n/" scripts/docket-config.sh
bash tests/test_docket_config.sh 2>&1 | grep -E 'NOT OK - 0102 R6'
```
Expected: at least one `NOT OK - 0102 R6` line.

```bash
cp /tmp/dc.bak scripts/docket-config.sh && rm /tmp/dc.bak
bash tests/test_docket_config.sh 2>&1 | grep -c 'NOT OK'
```
Expected: `0`.

- [ ] **Step 12: Commit**

```bash
git add scripts/docket-config.sh scripts/docket-config.md tests/test_docket_config.sh tests/test_docket_example_yml.sh
git commit -m "feat(0102): resolve finalize.require_pr_approval through the config layers

Adds the four-rung chain (repo-local > repo-committed > global > built-in
false), fails closed on a non-boolean, and emits
FINALIZE_REQUIRE_PR_APPROVAL after FINALIZE_TEST_COMMAND. Deliberately NOT
coordination-fenced: finalize.gate is the precedent for the other half of
the same merge gate. Export counts 24->25 (shell), 25->26 (plain).

Claude-Session: https://claude.ai/code/session_01EoXH1GHEjjXk7HDVD4ze4w"
```

---

### Task 2: Collapse the example's bespoke scope annotation

**Files:**
- Modify: `.docket.example.yml:93-101`
- Verify: `README.md:180`, `README.md:662`
- Test: `tests/test_docket_example_yml.sh` (existing asserts must stay green)

**Interfaces:**
- Consumes: `FINALIZE_REQUIRE_PR_APPROVAL` from Task 1 — the key is now resolver-read, which is what makes the old annotation false.
- Produces: the standard `# scope: any layer (.docket.yml, .docket.local.yml, or global config.yml)` tag on this key, which Task 4's manifest and the existing scope-tag `awk` both read.

- [ ] **Step 1: Write the failing assert**

In `tests/test_docket_example_yml.sh`, immediately after the existing assert block ending at `:147` (`assert "completeness: runners block header present" …`), add:

```bash
# change 0102: require_pr_approval is now RESOLVER-read and global-able, so it carries the
# standard any-layer tag like its two finalize siblings. The pre-0102 example carried a bespoke
# three-line note asserting the opposite (repo-committed only, silently ignored elsewhere) —
# that text described a state that no longer exists, and this pair keeps it from coming back.
assert "0102: require_pr_approval carries the any-layer scope tag" \
  'awk "/^  # require_pr_approval/,/^  require_pr_approval:/" "$EX" | grep -qF "scope: any layer"'
assert "0102: the stale repo-committed-only note is gone" \
  '! grep -qF "read by the finalize SKILL BODY, not by the config" "$EX"'
```

- [ ] **Step 2: Run to verify it fails**

Run: `bash tests/test_docket_example_yml.sh 2>&1 | grep '0102'`
Expected: both new asserts read `NOT OK -`.

- [ ] **Step 3: Rewrite the example's comment block**

In `.docket.example.yml`, replace lines 93-101 (the `require_pr_approval` block) with:

```yaml
  # require_pr_approval — governs the AUTO-DETECT finalize path only. false (default): docket
  # merges an eligible implemented change without requiring a GitHub approval. true: the no-arg
  # path REFUSES to merge a PR whose reviewDecision != APPROVED, surfacing it instead.
  # An explicit `docket-finalize-change <id>` (or an id allowlist) always OVERRIDES this —
  # naming the id IS the authorization.
  # scope: any layer (.docket.yml, .docket.local.yml, or global config.yml)
  require_pr_approval: false
```

The behavioral comment is unchanged; the trailing "Read by the finalize skill body, not by any script." sentence and the three-line bespoke `scope:` note are deleted, and the standard one-line tag replaces them.

- [ ] **Step 4: Run to verify the new asserts pass and nothing regressed**

Run: `bash tests/test_docket_example_yml.sh 2>&1 | grep -E 'NOT OK'`
Expected: no output.

Pay particular attention to two asserts that could regress here:
- `fidelity: example resolves byte-identically to no config at all` — the value stays `false`, which is the new built-in default, so this must stay green. If it reddens, the resolver default from Task 1 does not match the example's shipped value.
- `scope tag: every ACTIVE top-level key is individually tagged` — the awk window for `finalize:` extends through its nested body, so the replacement tag must sit inside that window.

- [ ] **Step 5: Verify the README makes no now-false claim**

Run: `grep -n 'require_pr_approval' README.md`

Expected: exactly two hits (`:180`, `:662`). Read both. Neither may claim the key is repo-only or read by the skill body rather than the resolver:
- `:180` describes the id allowlist overriding the approval gate — a behavioral statement, still true.
- `:662` describes a human approval satisfying both branch protection and `require_pr_approval: true` — still true.

If both are as described, **make no README edit** and note that in the commit message. If either does make a scope/skill-read claim, correct it to say the key resolves through the config layers like `finalize.gate`.

- [ ] **Step 6: Commit**

```bash
git add .docket.example.yml tests/test_docket_example_yml.sh
git commit -m "docs(0102): collapse require_pr_approval to the standard any-layer scope tag

The bespoke three-line note (repo-committed only; machine-scoped values
silently ignored) described the pre-0102 world. README's two mentions make
no scope claim and are left untouched.

Claude-Session: https://claude.ai/code/session_01EoXH1GHEjjXk7HDVD4ze4w"
```

---

### Task 3: Make the finalize skill read the export as its sole channel

**Files:**
- Modify: `skills/docket-finalize-change/SKILL.md:108` (framing sentence), `:120` (the policy paragraph), and each behavioral mention
- Test: `tests/test_docket_example_yml.sh:175-178` (the producer assert), plus a new sole-channel assert

**Interfaces:**
- Consumes: `FINALIZE_REQUIRE_PR_APPROVAL` from Task 1, read from the Step-0 `preflight` block exactly as the skill already reads `FINALIZE_GATE`, `LEARNINGS_ENABLED`, and `TERMINAL_PUBLISH`.
- Produces: no new symbol. The skill's `finalize:` yaml documentation block **stays** — it documents the key's meaning and existing asserts anchor on it.

Per the `sole-channel` learning: once the export is the only channel, the survivor must carry what the fallback gave free. Task 1's `(R7)` export-presence asserts are that proof; there is deliberately **no fallback** to a direct `.docket.yml` read, because a fallback would see only `.docket.yml` and would therefore honor a machine-scoped value on one path while ignoring it on the other — this exact bug, made intermittent.

- [ ] **Step 1: Write the failing asserts**

In `tests/test_docket_example_yml.sh`, replace the existing producer assert at `:175-178`:

```bash
# require_pr_approval is model-read, so nothing but this assert couples the example to the skill
# that consumes it. Anchor on the PRODUCER (the skill body) so the pair cannot silently diverge.
assert "require_pr_approval is still read by the finalize skill body" \
  'grep -q "require_pr_approval" "$REPO/skills/docket-finalize-change/SKILL.md"'
```

with:

```bash
# change 0102: require_pr_approval is now RESOLVER-read. The skill still NAMES the policy (that is
# what the (2c) consumer grep anchors on), but it must obtain the VALUE from the Step-0 export
# block — never by parsing .docket.yml itself. The second assert is the sole-channel proof: a
# reintroduced direct read would make a machine-scoped value honored on one path and ignored on
# the other, which is precisely the bug 0102 closed, returning as an intermittent one.
assert "require_pr_approval is still named by the finalize skill body" \
  'grep -q "require_pr_approval" "$REPO/skills/docket-finalize-change/SKILL.md"'
assert "0102: the finalize skill reads the EXPORTED value" \
  'grep -q "FINALIZE_REQUIRE_PR_APPROVAL" "$REPO/skills/docket-finalize-change/SKILL.md"'
assert "0102: the finalize skill does not parse .docket.yml for the key" \
  '! grep -nE "require_pr_approval" "$REPO/skills/docket-finalize-change/SKILL.md" | grep -q "Configured by .\?\.docket\.yml"'
```

- [ ] **Step 2: Run to verify the new assert fails**

Run: `bash tests/test_docket_example_yml.sh 2>&1 | grep '0102'`
Expected: `NOT OK - 0102: the finalize skill reads the EXPORTED value` (the skill body has no such string yet).

- [ ] **Step 3: Correct the framing sentence**

In `skills/docket-finalize-change/SKILL.md:108`, change:

```
Guards step 1's merge — the **only** place docket itself merges. Configured by `.docket.yml`:
```

to:

```
Guards step 1's merge — the **only** place docket itself merges. Configured by the resolved config — every value below is read from the Step-0 `preflight` export block (`FINALIZE_GATE`, `FINALIZE_TEST_COMMAND`, `FINALIZE_REQUIRE_PR_APPROVAL`), never by parsing `.docket.yml`; the block below documents what each key MEANS and where a user sets it:
```

Leave the fenced `finalize:` yaml block that follows byte-unchanged — it is user-facing documentation of the keys, and existing asserts anchor on it.

- [ ] **Step 4: Make the policy paragraph name the exported value**

In `skills/docket-finalize-change/SKILL.md:120`, change the opening of the paragraph from:

```
`require_pr_approval` validates *human sign-off* (`gate` validates *correctness*); it governs only the auto-detect path
```

to:

```
`require_pr_approval` validates *human sign-off* (`gate` validates *correctness*); the skill reads its resolved value as **`FINALIZE_REQUIRE_PR_APPROVAL`** from the Step-0 export block — the sole channel, resolving repo-local > repo-committed > global > the built-in `false` (change 0102) — and it governs only the auto-detect path
```

Leave the rest of that paragraph (the `true` ⇒ semantics, the human-reviewer requirement, the ADR-0011/ADR-0043 cross-references) unchanged.

- [ ] **Step 5: Point the behavioral mentions at the exported value**

Four other places name the key. In each, the prose keeps saying `require_pr_approval` where it names the **policy**, and names `FINALIZE_REQUIRE_PR_APPROVAL` where it names the **value the skill reads**. Make exactly these edits:

At `:50` (the Selection matrix row), change:
```
| `require_pr_approval: true` AND unapproved (`reviewDecision != APPROVED`) | **Surface, do not merge** — the policy gate |
```
to:
```
| `FINALIZE_REQUIRE_PR_APPROVAL` is `true` AND unapproved (`reviewDecision != APPROVED`) | **Surface, do not merge** — the policy gate |
```

At `:54` (the eligibility definition), change the opening:
```
"Eligible" = git-mergeable AND (`require_pr_approval: false` OR approved).
```
to:
```
"Eligible" = git-mergeable AND (`FINALIZE_REQUIRE_PR_APPROVAL` is `false` OR approved).
```
Leave the remainder of that sentence and paragraph unchanged (it correctly names the *policy* thereafter).

At `:40` and `:42` (the explicit-id and id-allowlist override paragraphs) and `:86`/`:88` (the disposition rules and the final report's skip reasons): **leave these unchanged** — every one of them names the policy, not the value read. Re-read each to confirm before moving on; if any of them describes *reading* the value, update that clause only.

- [ ] **Step 6: Verify no `.docket.yml` parse remains for this key**

Run: `grep -n -B2 -A2 'require_pr_approval' skills/docket-finalize-change/SKILL.md`

Read every hit. Expected: no sentence anywhere states or implies that the skill reads, parses, or looks up this key in `.docket.yml`. The only file-shaped mention left is the documentation yaml block, whose new framing sentence explicitly says it documents meaning rather than being the read path.

- [ ] **Step 7: Run the suites**

Run: `bash tests/test_docket_example_yml.sh 2>&1 | grep -E 'NOT OK'`
Expected: no output.

Run: `bash tests/test_readme_finalize_docs.sh 2>&1 | grep -E 'NOT OK'; bash tests/test_finalize_gate.sh 2>&1 | grep -E 'NOT OK'; bash tests/test_finalize_disposition.sh 2>&1 | grep -E 'NOT OK'`
Expected: no output from any of the three. These anchor on finalize's doc surface and are the most likely to catch an over-eager rewrite.

- [ ] **Step 8: Commit**

```bash
git add skills/docket-finalize-change/SKILL.md tests/test_docket_example_yml.sh
git commit -m "refactor(0102): finalize reads FINALIZE_REQUIRE_PR_APPROVAL as its sole channel

The skill no longer parses .docket.yml for the key; it reads the resolved
export like FINALIZE_GATE. No fallback by design — a fallback sees only
.docket.yml and would honor a machine-scoped value on one path while
ignoring it on the other, re-creating this bug as an intermittent one.
The documentation yaml block stays; its framing sentence is corrected.

Claude-Session: https://claude.ai/code/session_01EoXH1GHEjjXk7HDVD4ze4w"
```

---

### Task 4: The documented-key → resolved-key manifest guard

**Files:**
- Modify: `tests/test_docket_example_yml.sh:122-167` (replace the `(2b)` hand-list and generalize `(2c)`)

**Interfaces:**
- Consumes: the export surface produced by Task 1 (`$exp_none`, already computed at `:59` in this file) and the `map_for` entry added in Task 1 Step 9.
- Produces: a `classify_key()` function mapping an example YAML key name to either `resolved:<EXPORT_NAME>` or `elsewhere:<consumer-path>`; used only within this test file.

**Why this shape.** Per the `correspondence-guard-runs-one-way` learning (harvested from change 0101, the change that produced this file): `(2a)` iterates the resolver's export surface and proves `exports ⊆ example`. The manifest is the **reverse** loop — `example ⊆ classified` — and it must be mutation-proven in both directions. Per the same learning's war story, the `(2b)` allowlist "answers *is this one expected?*, never *does this one exist?*", and is the enumerated floor that made the original gap: so an `elsewhere:` entry does **not** merely allow a key, it **names the consumer and the test greps that file**. That is what keeps the guard anchored on consuming code rather than on a hand-maintained list.

**Do not delete `(2c)`.** It answers a different question (is the key known to *any* consumer) with a different anchor. The manifest is stricter and per-key; `(2c)` remains the backstop for a key nobody classified at all.

- [ ] **Step 1: Write the failing manifest test**

In `tests/test_docket_example_yml.sh`, replace the entire `(2b)` block — from the comment line `# (2b) NON-EXPORTED schema keys.` (`:122`) through `assert "completeness: runners block header present" 'grep -Eq "^runners:" "$EX"'` (`:147`) — with:

```bash
# (2b) THE CLASSIFICATION MANIFEST (change 0102).
# Every key documented in the example is classified in exactly one of two ways:
#
#   resolved:<EXPORT_NAME>   the resolver reads it; the test asserts that export is ACTUALLY
#                            emitted, so a manifest entry cannot claim an export that does not
#                            exist (nor survive one being removed).
#   elsewhere:<consumer>     deliberately not resolver-read, with its REAL consumer named; the
#                            test greps that named file for the key. Naming the consumer is what
#                            keeps this from decaying into an allowlist — per the
#                            correspondence-guard-runs-one-way learning, an allowlist answers
#                            "is this expected?" and never "does this exist?", which is the
#                            enumerated floor that let require_pr_approval ship documented-but-
#                            unwired in the first place.
#
# An UNCLASSIFIED key fails, naming itself as documented-but-unclassified. That is the direction
# that catches this bug class: a key added to the example with no resolution and no named reader.
#
# The mapping is explicit rather than derived because key -> export name is not 1:1
# (gate -> FINALIZE_GATE, enabled -> LEARNINGS_ENABLED, auto -> RECLAIM_AUTO,
# brainstorm -> SKILL_BRAINSTORM); any derivation would need this same table, hidden inside a
# transform instead of stated plainly.
classify_key(){ # classify_key <example-key-name> -> "resolved:EXPORT" | "elsewhere:path" | ""
  case "$1" in
    metadata_branch)      echo 'resolved:METADATA_BRANCH' ;;
    integration_branch)   echo 'resolved:INTEGRATION_BRANCH' ;;
    changes_dir)          echo 'resolved:CHANGES_DIR' ;;
    adrs_dir)             echo 'resolved:ADRS_DIR' ;;
    results_dir)          echo 'resolved:RESULTS_DIR' ;;
    gate)                 echo 'resolved:FINALIZE_GATE' ;;
    test_command)         echo 'resolved:FINALIZE_TEST_COMMAND' ;;
    require_pr_approval)  echo 'resolved:FINALIZE_REQUIRE_PR_APPROVAL' ;;
    enabled)              echo 'resolved:LEARNINGS_ENABLED' ;;
    cap)                  echo 'resolved:LEARNINGS_CAP' ;;
    board_surfaces)       echo 'resolved:BOARD_SURFACES' ;;
    auto_groom)           echo 'resolved:AUTO_GROOM' ;;
    auto_capture)         echo 'resolved:AUTO_CAPTURE' ;;
    terminal_publish)     echo 'resolved:TERMINAL_PUBLISH' ;;
    lease_ttl)            echo 'resolved:RECLAIM_LEASE_TTL' ;;
    auto)                 echo 'resolved:RECLAIM_AUTO' ;;
    brainstorm)           echo 'resolved:SKILL_BRAINSTORM' ;;
    plan)                 echo 'resolved:SKILL_PLAN' ;;
    build)                echo 'resolved:SKILL_BUILD' ;;
    review)               echo 'resolved:SKILL_REVIEW' ;;
    finish)               echo 'resolved:SKILL_FINISH' ;;
    # Block headers carry no value of their own; their children are classified above.
    finalize|learnings|reclaim|skills|runners|codex) echo 'elsewhere:HEADER' ;;
    # Genuinely non-resolver-read keys, each with its real consumer named.
    github_project)       echo 'elsewhere:scripts/docket-config.sh' ;;
    agents)               echo 'elsewhere:scripts/sync-agents.sh' ;;
    agent_harnesses)      echo 'elsewhere:scripts/sync-agents.sh' ;;
    sandbox)              echo 'elsewhere:scripts/runners/codex.sh' ;;
    network)              echo 'elsewhere:scripts/runners/codex.sh' ;;
    *) echo '' ;;
  esac
}

# Collect every key the example documents: active keys at any nesting depth, PLUS the two
# presence-sensitive keys that ship commented (agents / agent_harnesses), which are documented
# schema all the same.
manifest_unclassified=""
manifest_bad_export=""
manifest_bad_consumer=""
example_keys="$(
  { sed -nE 's/^[[:space:]]*([A-Za-z_][A-Za-z0-9_]*):.*/\1/p' "$EX"
    sed -nE 's/^#[[:space:]]*(agents|agent_harnesses):.*/\1/p' "$EX"
  } | sort -u
)"
for k in $example_keys; do
  cls="$(classify_key "$k")"
  case "$cls" in
    '')
      manifest_unclassified="$manifest_unclassified $k"
      ;;
    resolved:*)
      exp_name="${cls#resolved:}"
      # The export must ACTUALLY be emitted — a manifest entry cannot claim a phantom export.
      printf '%s\n' "$exp_none" | grep -q "^$exp_name=" \
        || manifest_bad_export="$manifest_bad_export $k($exp_name)"
      ;;
    elsewhere:HEADER)
      : # a mapping opener; its children carry the real classification
      ;;
    elsewhere:*)
      consumer="${cls#elsewhere:}"
      # The NAMED consumer must actually mention the key — this is what keeps the entry anchored
      # on consuming code instead of decaying into a bare allowlist.
      grep -qE "\\b$k\\b" "$REPO/$consumer" \
        || manifest_bad_consumer="$manifest_bad_consumer $k(not in $consumer)"
      ;;
  esac
done

assert "manifest: every documented key is classified (${manifest_unclassified:-none unclassified})" \
  '[ -z "$manifest_unclassified" ]'
assert "manifest: every resolved: entry names a REAL export (${manifest_bad_export:-none bad})" \
  '[ -z "$manifest_bad_export" ]'
assert "manifest: every elsewhere: entry's named consumer mentions the key (${manifest_bad_consumer:-none bad})" \
  '[ -z "$manifest_bad_consumer" ]'
# NON-VACUITY: the loop above must actually iterate. An extraction that silently yields nothing
# would make all three asserts pass while proving nothing.
mf_count="$(printf '%s\n' "$example_keys" | grep -c .)"
assert "manifest: key extraction is non-empty (got $mf_count)" '[ "$mf_count" -ge 20 ]'

# The value-anchored asserts for the non-exported keys are retained from the pre-0102 (2b): the
# fidelity check in (1) is structurally blind to keys the resolver never emits, so without these
# a typo'd value that merely has the right value as a PREFIX ("auto" matching "automanaged",
# "true" matching "truthy") would pass silently. sandbox/network carry a trailing inline comment
# in the example, so their anchors allow one optionally.
assert "completeness: github_project present (auto sentinel)" \
  'grep -Eq "^github_project:[[:space:]]*auto[[:space:]]*$" "$EX"'
assert "completeness: agent_harnesses present (commented)" \
  'grep -Eq "^#[[:space:]]*agent_harnesses:[[:space:]]*\[[[:space:]]*claude[[:space:]]*\][[:space:]]*$" "$EX"'
assert "completeness: agents present (commented)" \
  'grep -Eq "^#[[:space:]]*agents:[[:space:]]*$" "$EX"'
assert "completeness: runners.codex.sandbox present" \
  'grep -Eq "^[[:space:]]+sandbox:[[:space:]]*workspace-write[[:space:]]*(#.*)?$" "$EX"'
assert "completeness: runners.codex.network present" \
  'grep -Eq "^[[:space:]]+network:[[:space:]]*true[[:space:]]*(#.*)?$" "$EX"'
assert "completeness: runners block header present" 'grep -Eq "^runners:" "$EX"'
```

Note: `require_pr_approval`'s old standalone value assert is intentionally dropped from this retained list — as of Task 1 it is an **exported** key, so `(2a)`'s `map_for` entry now anchors its value, and keeping both would be the duplicate classification this manifest exists to prevent.

- [ ] **Step 2: Run to verify the suite is green with the manifest in place**

Run: `bash tests/test_docket_example_yml.sh 2>&1 | grep -E 'NOT OK|manifest'`
Expected: the four `manifest:` asserts read `ok -`, and no `NOT OK` lines.

If `manifest: every documented key is classified` fails, read the named keys: the extraction picks up nested keys the pre-0102 test never enumerated. Add each to `classify_key` with its true classification — do **not** widen the extraction to hide it.

- [ ] **Step 3: Mutation-test direction A — an unclassified key fails**

This is the direction that catches the bug the change exists to close.

```bash
cp .docket.example.yml /tmp/ex.bak
printf '\n# phantom_key — documented but read by nothing\n# scope: any layer\nphantom_key: false\n' >> .docket.example.yml
bash tests/test_docket_example_yml.sh 2>&1 | grep -E 'NOT OK - manifest: every documented key is classified'
```
Expected: one `NOT OK` line, naming `phantom_key`.

```bash
cp /tmp/ex.bak .docket.example.yml && rm /tmp/ex.bak
bash tests/test_docket_example_yml.sh 2>&1 | grep -c 'NOT OK'
```
Expected: `0`.

- [ ] **Step 4: Mutation-test direction B — a `resolved:` entry claiming a phantom export fails**

```bash
cp tests/test_docket_example_yml.sh /tmp/tex.bak
sed -i.tmp "s/resolved:FINALIZE_REQUIRE_PR_APPROVAL/resolved:FINALIZE_NO_SUCH_EXPORT/" tests/test_docket_example_yml.sh
bash tests/test_docket_example_yml.sh 2>&1 | grep -E 'NOT OK - manifest: every resolved: entry names a REAL export'
```
Expected: one `NOT OK` line, naming `require_pr_approval(FINALIZE_NO_SUCH_EXPORT)`.

```bash
cp /tmp/tex.bak tests/test_docket_example_yml.sh && rm -f /tmp/tex.bak tests/test_docket_example_yml.sh.tmp
bash tests/test_docket_example_yml.sh 2>&1 | grep -c 'NOT OK'
```
Expected: `0`.

- [ ] **Step 5: Mutation-test direction C — an `elsewhere:` entry whose consumer does not read the key fails**

```bash
cp tests/test_docket_example_yml.sh /tmp/tex.bak
sed -i.tmp "s#agent_harnesses)      echo 'elsewhere:scripts/sync-agents.sh'#agent_harnesses)      echo 'elsewhere:scripts/runners/codex.sh'#" tests/test_docket_example_yml.sh
bash tests/test_docket_example_yml.sh 2>&1 | grep -E "NOT OK - manifest: every elsewhere: entry"
```
Expected: one `NOT OK` line naming `agent_harnesses`.

```bash
cp /tmp/tex.bak tests/test_docket_example_yml.sh && rm -f /tmp/tex.bak tests/test_docket_example_yml.sh.tmp
bash tests/test_docket_example_yml.sh 2>&1 | grep -c 'NOT OK'
```
Expected: `0`.

- [ ] **Step 6: Prove the manifest catches the ORIGINAL bug**

The completion bar for this task: with the manifest in place, revert Task 1's export and confirm the guard reddens — i.e. it would have caught `require_pr_approval` shipping documented-but-unresolved.

```bash
cp scripts/docket-config.sh /tmp/dc.bak
sed -i.tmp '/emit FINALIZE_REQUIRE_PR_APPROVAL/d' scripts/docket-config.sh
bash tests/test_docket_example_yml.sh 2>&1 | grep -E 'NOT OK - manifest: every resolved: entry names a REAL export'
```
Expected: one `NOT OK` line — the manifest claims an export the resolver no longer emits.

```bash
cp /tmp/dc.bak scripts/docket-config.sh && rm -f /tmp/dc.bak scripts/docket-config.sh.tmp
bash tests/test_docket_example_yml.sh 2>&1 | grep -c 'NOT OK'
```
Expected: `0`.

- [ ] **Step 7: Commit**

```bash
git add tests/test_docket_example_yml.sh
git commit -m "test(0102): manifest guard binding documented keys to real consumers

Replaces the (2b) hand-written allowlist with a per-key classification:
resolved:<EXPORT> asserts the export is actually emitted, elsewhere:<file>
asserts the named consumer mentions the key. An unclassified key fails.
Mutation-verified in three directions, including that it reddens when
FINALIZE_REQUIRE_PR_APPROVAL's emit is removed — i.e. it would have caught
the original documented-but-unwired bug.

Claude-Session: https://claude.ai/code/session_01EoXH1GHEjjXk7HDVD4ze4w"
```

---

### Task 5: Full-suite verification

**Files:** none modified — this task is the whole-change gate.

- [ ] **Step 1: Run the entire suite**

Run, in ONE foreground call, from the worktree root:

```bash
for t in tests/test_*.sh; do echo "=== $t ==="; bash "$t" 2>&1 | grep -E '^NOT OK' || echo "(all ok)"; done
```

Expected: `(all ok)` under every test file.

This suite takes several minutes. Run it in the **foreground** and wait — never background it.

- [ ] **Step 2: Verify the resolver's real-world output**

Run: `./scripts/docket-config.sh --export | grep -n FINALIZE`

Expected, in this order:
```
FINALIZE_GATE=local
FINALIZE_TEST_COMMAND=
FINALIZE_REQUIRE_PR_APPROVAL=false
```

Run: `./scripts/docket-config.sh --export | wc -l`
Expected: `25`.

- [ ] **Step 3: Confirm no stray count references remain**

Run: `grep -rn '24 KEY=value\|24 lines' scripts/ tests/ README.md`
Expected: no output.

- [ ] **Step 4: Commit any fixes**

If steps 1-3 surfaced nothing, there is nothing to commit — proceed. Otherwise fix and commit with a `fix(0102):` message and the session trailer.

---

## Self-Review

**Spec coverage:**
- §1 Resolver (chain, leaf-read comment, not-fenced, no `auto` sentinel, emit) → Task 1 Steps 3-5, asserted by `(R1)`-`(R7)`.
- §2 Contract doc (table row, export list, counts) → Task 1 Steps 7-8, asserted by `(R8)`.
- §3 Skill (framing sentence, behavioral mentions, no `.docket.yml` parse) → Task 3.
- §4 Surfacing (example collapse, README check) → Task 2.
- §5 Drift guard (manifest, both classification forms, unclassified fails) → Task 4.
- §6 Tests (resolution 4 rungs, fail-closed, not-fenced, export presence, guard, skill) → `(R1)`-`(R8)` + Task 2 Step 1 + Task 3 Step 1 + Task 4.
- §7 ADR → **out of band**, recorded by the `docket-adr` subagent on the metadata branch; flagged at the top of this plan so no task creates an ADR file.

**Type/name consistency:** the export name `FINALIZE_REQUIRE_PR_APPROVAL` is identical in Task 1 (definition + emit + `map_for`), Task 3 (skill body), and Task 4 (`classify_key`). The leaf key read by `lcl`/`yaml_get`/`gbl` is `require_pr_approval` throughout — never the dotted `finalize.require_pr_approval`, which is how the *doc* names it but not how the flat reader keys it.

**Sequencing note:** Task 1 Step 9 is not optional. Adding the export without the `map_for` entry leaves `tests/test_docket_example_yml.sh` red at the end of Task 1, violating the rule that every task ends green and independently testable.
