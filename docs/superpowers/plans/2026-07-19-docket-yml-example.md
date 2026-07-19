# `.docket.yml.example` — Canonical Config Reference Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace docket's three drifting config-documentation surfaces with one committed `.docket.yml.example` at the repo root — every key active at its shipped default, fully documented, scope-tagged, and test-enforced against the resolver.

**Architecture:** The example is pure documentation — no docket tooling ever reads it (the resolver reads `.docket.yml` from `origin/HEAD`). Tests are what keep it honest: a *fidelity* test copies the example into a fixture repo as `.docket.yml` and asserts the resolver's export is byte-identical to the no-config case, and a *completeness* test asserts every schema key appears. `config.yml.example` is deleted; `install.sh` scaffolds a minimal pointer-only global config instead; this repo's own `.docket.yml` slims to its set values and dogfoods the copy-out workflow.

**Tech Stack:** Bash 3.2-compatible shell scripts, POSIX/BSD-safe `sed`/`awk`/`grep -E`, git fixture repos with bare origins, docket's own `assert`-style test harness.

## Global Constraints

- **Shell portability:** BSD/macOS `sed` and `awk` — no GNU-only flags. `set -uo pipefail` in tests (matching `tests/test_docket_config.sh`), `set -euo pipefail` in scripts.
- **The example is never read by tooling.** It is documentation. Only tests may load it.
- **Every active value in the example MUST equal the resolver's shipped default.** This is the fidelity invariant; Task 2's test enforces it.
- **`auto` sentinel case:** accept the literal lowercase `auto` only, consistent with `integration_branch`.
- **Sentinels must not leak to consumers:** the resolved export surface for `test_command: auto` must be byte-identical to the unset case.
- **Presence-sensitive keys (`agents:`, `agent_harnesses:`) ship COMMENTED.** Uncommenting either opts a repo into per-repo wrapper generation even at default values. Exact marker wording:
  `# PRESENCE-SENSITIVE: uncommenting this key changes behavior even at these default values.`
- **Scope tags** — exactly two forms, one line per key:
  - `# scope: repo-only (coordination-fenced, ADR-0019)`
  - `# scope: any layer (.docket.yml, .docket.local.yml, or global config.yml)`
- **`scripts/docket-config.md`'s table stays the authoritative scope source.** The example mirrors it; it never becomes the source.
- **ADR-0039's mirror rule survives relocated:** the example's commented `agents.claude` block mirrors `agents/docket-*.md` wrapper frontmatter. The wrappers lead; the mirror never does.
- **Every test assertion must be non-vacuous** — deleting the clause it guards must flip it to `NOT OK`.

---

## Reconcile findings folded into this plan

Two discoveries from the 2026-07-19 reconcile pass change specific task shapes. Read these before Task 1 — they are the difference between a correct build and a plausible-looking one.

**(A) The two `auto` sentinels are NOT symmetric.**
- `finalize.test_command` **is** exported (`FINALIZE_TEST_COMMAND`, `scripts/docket-config.sh:194`). Shipping `test_command: auto` without a code change would resolve `FINALIZE_TEST_COMMAND=auto` and **leak the sentinel to finalize**, which would try to run `auto` as a shell command. This needs a real resolver change (Task 1) and the fidelity export-diff proves it.
- `github_project` is **not read by any script at all**. `scripts/docket-config.sh:169` only coordination-*fences* it; `PROJECT_FLAG` in `scripts/docket-status.sh:36` comes solely from a `--project` CLI flag that no skill passes; the `project-minted` write-back at `scripts/github-mirror.sh:294` is a comment describing a caller that does not exist. So `github_project` is today a **documented-but-unwired key**. Its `auto` sentinel is therefore **documentation-only** — wiring the read would be a behavior change, which the spec puts out of scope. Ship it active as `auto`, document the sentinel in the contracts, and assert it is inert.

**(B) Export keys alone under-cover the schema.** Four keys have no export key and need a **second, explicit list** in the completeness test: `github_project`, `agents:`, `agent_harnesses:`, and **`finalize.require_pr_approval`**. That last one is a *model-read* key — read only by `skills/docket-finalize-change/SKILL.md` (lines 40, 42, 50, 54, 86), implemented in no script, and **never named in the spec's own key inventory**. Without the explicit list the "canonical" reference ships missing a real key on day one — exactly the drift this change exists to end.

## Learnings that bear on this change

Pulled from `docs/changes/learnings/` at plan time; each is a known failure mode this specific change can hit.

- **`config-layer-write-and-read-hazards`** — a change touching a shared user-level location upgrades every non-hermetic test that reaches it from *read-leak* to **data-loss**. Task 5 rewrites the `~/.config/docket/config.yml` scaffold. Every test that can transitively reach `ensure-global-config.sh` **must** pin `XDG_CONFIG_HOME` and `DOCKET_HARNESS_ROOT` hermetically. `tests/test_install.sh` previously inherited `XDG_CONFIG_HOME` and nearly rewrote a developer's real global config — re-audit it in Task 5.
- **`consolidation-flattens-caller-variance`** — this change collapses prose that two files each restated. **Diff the restatements against each other before templating.** `config.yml.example`'s prose is *global-layer-specific* ("this is a legitimate place to set them", the harness-enablement flow); `.docket.yml`'s is *repo-specific* ("this repo runs the defaults, so the block stays commented out"). Merging them by picking one voice silently rewrites the other's meaning. The scope tags are the mechanism that lets one file carry both — use them; do not flatten.
- **`config-knob-ship-end-to-end`** — a config surface is not done when it merely works. The README, the script contracts, and the now-relaxed prose ship in the **same** change (Tasks 5–6).
- **`opt-in-signal-not-file-presence`** — the presence-sensitivity of `agents:`/`agent_harnesses:` is exactly this finding's shape. Task 4's assert that neither ships active is a **regression guard for a real past break** (change 0048 littered wrappers into tracking-only repos), not a style check.
- **`specified-but-unreachable`** — audit the sentinel set for **producer** coverage. The must-update rule is prose in a header; the thing that *enforces* it is the completeness test. Anchor an assert on the enforcement, not only on the header's wording.
- **`adr-update-delivery`** — the new ADR supersedes ADR-0039, so ADR-0039's `status:` line changes. Both ids must appear in change 0101's `adrs:` so terminal-publish copies both atomically at merge. (Handled by `docket-adr` at review time, not by a plan task — noted so it is not lost.)

---

## File Structure

**Created:**
- `.docket.yml.example` — repo root. The canonical all-comprehensive config reference. Pure documentation.
- `tests/test_docket_yml_example.sh` — the enforcement: fidelity, completeness, mirror equality, presence-sensitivity, resolver round-trip, scaffold shape, README wiring.

**Deleted:**
- `config.yml.example` — absorbed into `.docket.yml.example`.
- `tests/test_config_example.sh` — replaced by `tests/test_docket_yml_example.sh`.

**Modified:**
- `scripts/docket-config.sh` — the `finalize.test_command: auto` sentinel (~line 194).
- `scripts/docket-config.md` — document both sentinels in the resolved-values table.
- `scripts/github-mirror.md` — document `github_project: auto` as ≡ unminted at the write-back site.
- `scripts/ensure-global-config.sh` — scaffold a minimal pointer-only global config instead of copying `config.yml.example`.
- `scripts/ensure-global-config.md` — match the new behavior.
- `install.sh` — the header comment naming `config.yml.example` (line 6).
- `.docket.yml` — slim to actually-set values + header pointer to the example.
- `README.md` — retarget step 2 (line ~132) and the `ensure-global-config.sh` bullet (line 126) from `config.yml.example` to `.docket.yml.example`.

---

### Task 1: The `auto` sentinel

**Files:**
- Modify: `scripts/docket-config.sh:194`
- Modify: `scripts/docket-config.md:104` (the `test_command` table row), and the note at `:114`
- Modify: `scripts/github-mirror.md` (the write-back section around the `project-minted` contract)
- Test: `tests/test_docket_config.sh` (append a new fixture section)

**Interfaces:**
- Consumes: nothing (first task).
- Produces: the guarantee that `finalize.test_command: auto` resolves to `FINALIZE_TEST_COMMAND=` (empty). Task 2's fidelity test depends on this — without it, shipping `test_command: auto` in the example breaks the byte-identical export assertion.

- [ ] **Step 1: Write the failing test**

Append to `tests/test_docket_config.sh`, immediately before the final `exit $fail`:

```bash
# --- (S) finalize.test_command: auto  ==  unset (change 0101 sentinel) -------
# `auto` is the example file's way of shipping the default EXPLICITLY. It must resolve
# byte-identically to an absent key, or the sentinel leaks into finalize as a command to run.
mkrepo "$tmp/s"
cat > "$tmp/s/.docket.yml" <<'EOF'
metadata_branch: main
integration_branch: main
finalize:
  gate: local
  test_command: auto
EOF
git -C "$tmp/s" add .docket.yml; git -C "$tmp/s" commit --quiet -m cfg
git -C "$tmp/s" push --quiet origin main
out="$(run "$tmp/s" --export)"; eval "$out"
assert "test_command auto: FINALIZE_TEST_COMMAND empty" '[ -z "$FINALIZE_TEST_COMMAND" ]'
assert "test_command auto: FINALIZE_GATE still local"   '[ "$FINALIZE_GATE" = local ]'

# An explicit non-sentinel value is still honored verbatim (the sentinel is not a blanket clear).
mkrepo "$tmp/s2"
cat > "$tmp/s2/.docket.yml" <<'EOF'
metadata_branch: main
integration_branch: main
finalize:
  test_command: make test
EOF
git -C "$tmp/s2" add .docket.yml; git -C "$tmp/s2" commit --quiet -m cfg
git -C "$tmp/s2" push --quiet origin main
out="$(run "$tmp/s2" --export)"; eval "$out"
assert "explicit test_command honored verbatim" '[ "$FINALIZE_TEST_COMMAND" = "make test" ]'

# Case-sensitivity: only the literal lowercase `auto` is the sentinel (integration_branch precedent).
mkrepo "$tmp/s3"
cat > "$tmp/s3/.docket.yml" <<'EOF'
metadata_branch: main
integration_branch: main
finalize:
  test_command: AUTO
EOF
git -C "$tmp/s3" add .docket.yml; git -C "$tmp/s3" commit --quiet -m cfg
git -C "$tmp/s3" push --quiet origin main
out="$(run "$tmp/s3" --export)"; eval "$out"
assert "test_command AUTO is NOT the sentinel (case-sensitive)" '[ "$FINALIZE_TEST_COMMAND" = "AUTO" ]'
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_docket_config.sh 2>&1 | grep -E 'test_command auto|NOT OK'`

Expected: `NOT OK - test_command auto: FINALIZE_TEST_COMMAND empty` (the resolver currently passes `auto` through verbatim). The other two asserts already pass — that is correct and expected; they are the regression guards proving Step 3 does not over-reach.

- [ ] **Step 3: Write the minimal implementation**

In `scripts/docket-config.sh`, replace line 194:

```bash
FINALIZE_TEST_COMMAND="$(lcl test_command)"; FINALIZE_TEST_COMMAND="${FINALIZE_TEST_COMMAND:-$(yaml_get "$CFG" test_command)}"; FINALIZE_TEST_COMMAND="${FINALIZE_TEST_COMMAND:-$(gbl test_command)}"
```

with:

```bash
FINALIZE_TEST_COMMAND="$(lcl test_command)"; FINALIZE_TEST_COMMAND="${FINALIZE_TEST_COMMAND:-$(yaml_get "$CFG" test_command)}"; FINALIZE_TEST_COMMAND="${FINALIZE_TEST_COMMAND:-$(gbl test_command)}"
# change 0101: `auto` ≡ unset — the sentinel that lets .docket.yml.example ship this default as an
# ACTIVE value instead of a commented "normally unset" note. Applied AFTER layer resolution, so a
# lower layer's `auto` cannot resurrect a higher layer's real command (both resolve to auto-detect).
# Literal lowercase only, matching the integration_branch precedent. Consumers must never see the
# sentinel: finalize would try to RUN `auto` as a shell command.
[ "$FINALIZE_TEST_COMMAND" = auto ] && FINALIZE_TEST_COMMAND=""
```

Note: `[ ... ] && ...` as the last statement of a `set -e` script would abort on a false test — but `docket-config.sh` runs under `set -uo pipefail` without `-e`, and this line is mid-script. Verify with Step 4; if the script is later hardened to `set -e`, use the `if` form instead.

- [ ] **Step 4: Run the test to verify it passes**

Run: `bash tests/test_docket_config.sh 2>&1 | tail -20; echo "EXIT=$?"`

Expected: all three new asserts `ok -`, and the whole file exits `0`.

Then prove non-vacuity — delete the new sentinel line, re-run, confirm the first assert flips to `NOT OK`, and restore it.

- [ ] **Step 5: Document both sentinels in the contracts**

In `scripts/docket-config.md`, replace the `test_command` row (line 104):

```markdown
| `test_command` (finalize) | `` (empty) | yes | read from `finalize.test_command` leaf key; resolves repo-local > repo-committed > global. **`auto` ≡ unset** (change 0101): the literal lowercase `auto` resolves to the empty string so finalize auto-detects the suite, letting `.docket.yml.example` ship this default as an active value. Applied after layer resolution; any other value (including `AUTO`) is honored verbatim |
```

And extend the note at line 114:

```markdown
`github_project` and `agents:`/`agent_harnesses` are per-repo-only / not read by this script (see
Stage 2b/2b'/2c below and `sync-agents.sh`'s own contract, respectively) — every other key above
not marked "Global-able" is per-repo-only. **`github_project: auto` ≡ unset** (change 0101): the
sentinel marks the board as unminted, so the first `github` sync mints and writes back over it.
This script only *fences* `github_project`; it never resolves or emits it, so the sentinel is inert
here by construction — see `scripts/github-mirror.md` for the consuming contract.
```

In `scripts/github-mirror.md`, at the section documenting the `project-minted` write-back, append:

```markdown
**`github_project: auto` (change 0101).** The literal lowercase `auto` is the explicit spelling of
"unminted" — identical in effect to an absent key. The write-back path treats it as a value to
**overwrite**, never as a minted project reference. This lets `.docket.yml.example` ship the key
active at its default instead of as a commented-out note.
```

- [ ] **Step 6: Commit**

```bash
git add scripts/docket-config.sh scripts/docket-config.md scripts/github-mirror.md tests/test_docket_config.sh
git commit -m "feat(0101): auto sentinel for finalize.test_command; document both sentinels

finalize.test_command: auto now resolves to empty (auto-detect), so the
example file can ship the default as an active value without leaking the
sentinel into finalize as a command to run. github_project: auto is
documented at its consuming contract — the resolver only fences that key."
```

---

### Task 2: The example file + the fidelity test

**Files:**
- Create: `.docket.yml.example`
- Create: `tests/test_docket_yml_example.sh`

**Interfaces:**
- Consumes: Task 1's `auto` sentinel (without it the fidelity assert cannot pass with `test_command: auto` active).
- Produces: `.docket.yml.example` at the repo root, and `tests/test_docket_yml_example.sh` exposing two shell helpers that Tasks 3–6 extend — `EX` (path to the example) and `assert <name> <expr>` (the house harness, identical to `tests/test_config_example.sh`'s).

- [ ] **Step 1: Write the failing test**

Create `tests/test_docket_yml_example.sh`:

```bash
#!/usr/bin/env bash
# tests/test_docket_yml_example.sh — run: bash tests/test_docket_yml_example.sh
# Guards .docket.yml.example, docket's canonical all-comprehensive config reference (change 0101).
# The example is PURE DOCUMENTATION — no docket tooling reads it — so these tests are the only
# thing keeping it honest. Replaces tests/test_config_example.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
EX="$REPO/.docket.yml.example"
CFGSCRIPT="$REPO/scripts/docket-config.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
# Hermetic: never read OR WRITE the dev machine's real global config. See the
# config-layer-write-and-read-hazards learning — this suite reaches ensure-global-config.sh.
export XDG_CONFIG_HOME="$tmp/xdg-void"

# fixture builder: a clone with a bare origin, one commit on main (origin/HEAD -> main).
# Mirrors tests/test_docket_config.sh's mkrepo.
mkrepo(){
  local dir="$1" bare="$1.origin.git"
  git init --quiet --bare "$bare"
  git clone --quiet "$bare" "$dir" 2>/dev/null
  git -C "$dir" config user.email t@t.test
  git -C "$dir" config user.name  Test
  git -C "$dir" checkout --quiet -b main
  : > "$dir/README.md"
  git -C "$dir" add README.md
  git -C "$dir" commit --quiet -m init
  git -C "$dir" push --quiet -u origin main
  git -C "$dir" remote set-head origin -a >/dev/null 2>&1
}

assert ".docket.yml.example exists at repo root" '[ -f "$EX" ]'

# --- (1) FIDELITY: example == shipped defaults -------------------------------
# Copy the example in as .docket.yml on a fixture's default branch; the resolver's export must be
# BYTE-IDENTICAL to the same fixture with no config file at all. This proves (a) every active value
# equals the shipped default, (b) both `auto` sentinels resolve to the unset behavior, and (c) no
# active key in the example collides with the resolver's FLAT leaf-key reader.
mkrepo "$tmp/none"
mkrepo "$tmp/full"
cp "$EX" "$tmp/full/.docket.yml"
git -C "$tmp/full" add .docket.yml
git -C "$tmp/full" commit --quiet -m cfg
git -C "$tmp/full" push --quiet origin main

# --repo-dir differs between the two fixtures, and plain format emits absolute REPO_ROOT /
# METADATA_WORKTREE paths — normalize those two lines out before diffing.
norm(){ grep -vE '^(REPO_ROOT|METADATA_WORKTREE)=' ; }
exp_none="$(bash "$CFGSCRIPT" --repo-dir "$tmp/none" --export --format plain 2>/dev/null | norm)"
exp_full="$(bash "$CFGSCRIPT" --repo-dir "$tmp/full" --export --format plain 2>/dev/null | norm)"

assert "fidelity: export is non-empty (guard against both sides failing silently)" \
  '[ -n "$exp_none" ] && [ "$(printf "%s\n" "$exp_none" | wc -l)" -ge 15 ]'
assert "fidelity: example resolves byte-identically to no config at all" \
  '[ "$exp_none" = "$exp_full" ]'
if [ "$exp_none" != "$exp_full" ]; then
  echo "--- diff (no-config vs example-as-.docket.yml) ---"
  diff <(printf '%s\n' "$exp_none") <(printf '%s\n' "$exp_full") || true
  echo "---"
fi

exit $fail
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_docket_yml_example.sh`

Expected: `NOT OK - .docket.yml.example exists at repo root`, then the fidelity asserts also fail (the `cp` errors, leaving no `.docket.yml`). This is the correct starting state.

- [ ] **Step 3: Write the example file**

Create `.docket.yml.example` at the repo root. **Every key below ships ACTIVE at the exact value shown, except the two marked PRESENCE-SENSITIVE which ship commented.** Each key gets its scope tag line plus its documentation prose.

**Sourcing the prose — do not write it from scratch.** Absorb it, preserving change/ADR back-references:
- from `.docket.yml` (current repo copy): the `metadata_branch` docket-mode paragraph, the `finalize` block prose, the `learnings` block prose, the `reclaim` block prose, the `board_surfaces` prose, the `terminal_publish` prose, the `agent_harnesses` + `agents` prose, and the `skills` prose.
- from `config.yml.example`: the harness-enablement flow ("add it here AND uncomment that harness's block"), the "unedited file behaves exactly as if it were absent" guarantee, the `agents.claude` mirror block **verbatim** (all nine lines), and the commented `codex:`/`cursor:` blocks **verbatim**.

**Do not flatten the two voices** (see the `consolidation-flattens-caller-variance` learning). `config.yml.example` said "this is a legitimate place to set them" (global-layer-specific); `.docket.yml` said "this repo runs the defaults, so the block stays commented out" (repo-specific). Neither sentence survives as-is in a layer-neutral file — the **scope tag** carries that information instead. Replace both with the tag plus layer-neutral prose.

Header block (exact content required):

```yaml
# .docket.yml.example — docket's canonical, all-comprehensive configuration reference.
#
# THIS FILE IS DOCUMENTATION. No docket tooling ever reads it: the resolver reads `.docket.yml`
# from your repo's default branch (origin/HEAD). This file exists so that every key, its shipped
# default, its meaning, and the layers it may be set in are visible in ONE place. Tests keep it
# honest (tests/test_docket_yml_example.sh).
#
# ── The four layers ────────────────────────────────────────────────────────────────────────
# Configuration resolves PER KEY, with precedence:
#   1. repo-local    <repo>/.docket.local.yml                     (this machine, this repo; gitignored)
#   2. repo-committed <repo>/.docket.yml                          (every clone — committed on origin/HEAD)
#   3. global        ${XDG_CONFIG_HOME:-~/.config}/docket/config.yml  (this machine, every repo)
#   4. built-in      docket's defaults — the values in this file
# Map-valued keys (`skills:`, `agents:`) merge FIELD BY FIELD with the same precedence.
#
# ── How to use this file ───────────────────────────────────────────────────────────────────
# Copy the keys you want to CHANGE into the layer you want them in. Do not copy the whole file
# unless you mean to — though an unedited full copy behaves identically to no file at all, since
# every active value here IS the shipped default. The two PRESENCE-SENSITIVE keys are the
# exception: they ship commented because uncommenting them changes behavior at default values.
#
# ── Scope tags ─────────────────────────────────────────────────────────────────────────────
# Every key carries one:
#   # scope: repo-only (coordination-fenced, ADR-0019)
#       Writes shared, non-re-derivable state. Settable ONLY in the repo's committed .docket.yml;
#       a value in .docket.local.yml or the global config is loudly warned-and-ignored.
#   # scope: any layer (.docket.yml, .docket.local.yml, or global config.yml)
#       Behavioral only. Per-machine divergence is benign.
# The authoritative classification lives in scripts/docket-config.md; this file mirrors it.
#
# ── THE STANDING RULE ──────────────────────────────────────────────────────────────────────
# EVERY new config flag lands in THIS FILE — its value AND its documentation — in the SAME PR
# that introduces it. tests/test_docket_yml_example.sh enforces it: a new key with no entry here
# fails the suite. This rule is why the file is worth having; without it the file drifts and
# becomes the fourth stale surface it was created to replace.
```

The complete key inventory — nothing may be omitted:

| Key | Ships as | Scope |
|---|---|---|
| `metadata_branch` | `docket` | repo-only |
| `integration_branch` | `auto` | repo-only |
| `changes_dir` | `docs/changes` | repo-only |
| `adrs_dir` | `docs/adrs` | repo-only |
| `results_dir` | `docs/results` | repo-only |
| `finalize.gate` | `local` | any layer |
| `finalize.test_command` | `auto` | any layer |
| `finalize.require_pr_approval` | `false` | any layer |
| `learnings.enabled` | `true` | any layer |
| `learnings.cap` | `300` | any layer |
| `reclaim.lease_ttl` | `72` | any layer |
| `reclaim.auto` | `false` | any layer |
| `board_surfaces` | `[inline]` | any layer, minus the `github` token (repo-only) |
| `terminal_publish` | `false` | repo-only |
| `auto_groom` | `false` | any layer |
| `auto_capture` | `false` | any layer |
| `github_project` | `auto` | repo-only |
| `agent_harnesses` | `[claude]` — **COMMENTED** | any layer |
| `agents` | the nine-line `claude:` mirror + commented `codex:`/`cursor:` — **COMMENTED** | any layer |
| `skills.brainstorm` | `superpowers:brainstorming` | any layer |
| `skills.plan` | `superpowers:writing-plans` | any layer |
| `skills.build` | `superpowers:subagent-driven-development` | any layer |
| `skills.review` | `superpowers:requesting-code-review` | any layer |
| `skills.finish` | `superpowers:finishing-a-development-branch` | any layer |

Two keys need documentation the old surfaces never carried:

`finalize.require_pr_approval` — the key the spec's inventory missed. It is read by the **skill body**, `skills/docket-finalize-change/SKILL.md`, not by any script. Prose to write:

```yaml
  # require_pr_approval — governs the AUTO-DETECT finalize path only. false (default): docket
  # merges an eligible implemented change without requiring a GitHub approval. true: the no-arg
  # path REFUSES to merge a PR whose reviewDecision != APPROVED, surfacing it instead.
  # An explicit `docket-finalize-change <id>` (or an id allowlist) always OVERRIDES this —
  # naming the id IS the authorization. Read by the finalize skill body, not by any script.
  # scope: any layer (.docket.yml, .docket.local.yml, or global config.yml)
  require_pr_approval: false
```

`github_project` — ships as the `auto` sentinel. Prose to write:

```yaml
# github_project — the auto-managed GitHub Projects v2 board backing the `github` board surface.
# `auto` (default) means UNMINTED: the first `github` sync creates the board (private, under the
# repo owner) and writes the resolved {owner, number} back over this value. Consulted only when
# board_surfaces includes `github`. Set it explicitly to point at an existing board:
#   github_project: {owner: my-org, number: 7}
# scope: repo-only (coordination-fenced, ADR-0019)
github_project: auto
```

The presence-sensitive pair, with the exact marker:

```yaml
# agent_harnesses — which harnesses the PER-REPO agent pass generates wrapper files for.
# Value default is [claude]. But PRESENCE in a repo file is itself the opt-in to per-repo wrapper
# generation: a repo that adopts docket for change-tracking only, with neither this key nor
# `agents:` set anywhere, generates no wrappers and keeps `sync-agents.sh --check` a no-op.
# To enable another harness: add it here AND uncomment that harness's block under `agents:`,
# then re-run install.sh. Model IDs pass through verbatim; unknown tokens are warned + dropped.
# PRESENCE-SENSITIVE: uncommenting this key changes behavior even at these default values.
# scope: any layer (.docket.yml, .docket.local.yml, or global config.yml)
# agent_harnesses: [claude]
```

...and the `agents:` block likewise commented, carrying the nine-line `claude:` mirror copied **verbatim** from `config.yml.example:50-58`, the ADR-0039 mirror note (rewritten to name this file rather than `config.yml.example`), the same `PRESENCE-SENSITIVE` marker line, and the commented `codex:`/`cursor:` example blocks copied verbatim from `config.yml.example:60-80` — including the "IDs here are UNVALIDATED examples" warning.

- [ ] **Step 4: Run the test to verify it passes**

Run: `bash tests/test_docket_yml_example.sh`

Expected: `ok - .docket.yml.example exists at repo root`, `ok - fidelity: export is non-empty ...`, `ok - fidelity: example resolves byte-identically to no config at all`.

If the fidelity assert fails, the test prints the diff. Read it literally — each differing line names a key whose active value is **not** the shipped default, or a key whose name collides with the resolver's flat leaf-key reader (`yaml_get` matches `^[[:space:]]*<key>[[:space:]]*:`, so an indented nested key can be picked up as a top-level one). Fix the example, not the test.

- [ ] **Step 5: Prove the fidelity assert is non-vacuous**

Temporarily change one active value in the example to a non-default (e.g. `auto_groom: true`), re-run, and confirm the assert flips to `NOT OK` with a diff naming `AUTO_GROOM`. Restore the file.

This step matters: a fidelity test that passes because both sides failed identically would be worthless. The non-empty guard covers that case, but confirm the diff path works too.

- [ ] **Step 6: Commit**

```bash
git add .docket.yml.example tests/test_docket_yml_example.sh
git commit -m "feat(0101): add .docket.yml.example + the fidelity test

Every key active at its shipped default, per-key docs, and a scope tag.
The fidelity test copies it into a fixture as .docket.yml and asserts the
resolver export is byte-identical to the no-config case."
```

---

### Task 3: The completeness test

**Files:**
- Modify: `tests/test_docket_yml_example.sh` (append before `exit $fail`)

**Interfaces:**
- Consumes: `EX`, `assert`, `mkrepo`, `tmp` from Task 2.
- Produces: the enforcement half of the standing rule — a new config key with no entry in the example (or no entry in the test's mapping) fails the suite.

- [ ] **Step 1: Write the failing test**

Append to `tests/test_docket_yml_example.sh`, before `exit $fail`:

```bash
# --- (2) COMPLETENESS: every schema key appears in the example ---------------
# Two sources, because export keys alone UNDER-COVER the schema (change 0101 reconcile).
#
# (2a) Exported keys: every KEY= the resolver emits maps to a YAML path in the example.
# The mapping lives here on purpose — a new export key with no entry fails this test, forcing
# the example AND this mapping to be updated in the same PR. That is the must-update rule's
# enforcement; the header prose is only its statement.
#
# Format: "EXPORT_KEY:yaml_regex". A leading '#' in the regex matches the commented form.
# Export keys that are DERIVED (not settable config) are listed in the skip set below.
exported_skip="DOCKET_MODE DEFAULT_BRANCH METADATA_WORKTREE REPO_ROOT BOOTSTRAP"
map_for(){ # map_for <EXPORT_KEY> -> ERE matching the example's line, or empty if unmapped
  case "$1" in
    METADATA_BRANCH)       echo '^metadata_branch:[[:space:]]*docket' ;;
    INTEGRATION_BRANCH)    echo '^integration_branch:[[:space:]]*auto' ;;
    CHANGES_DIR)           echo '^changes_dir:[[:space:]]*docs/changes' ;;
    ADRS_DIR)              echo '^adrs_dir:[[:space:]]*docs/adrs' ;;
    RESULTS_DIR)           echo '^results_dir:[[:space:]]*docs/results' ;;
    FINALIZE_GATE)         echo '^[[:space:]]+gate:[[:space:]]*local' ;;
    FINALIZE_TEST_COMMAND) echo '^[[:space:]]+test_command:[[:space:]]*auto' ;;
    LEARNINGS_ENABLED)     echo '^[[:space:]]+enabled:[[:space:]]*true' ;;
    LEARNINGS_CAP)         echo '^[[:space:]]+cap:[[:space:]]*300' ;;
    BOARD_SURFACES)        echo '^board_surfaces:[[:space:]]*\[[[:space:]]*inline[[:space:]]*\]' ;;
    AUTO_GROOM)            echo '^auto_groom:[[:space:]]*false' ;;
    AUTO_CAPTURE)          echo '^auto_capture:[[:space:]]*false' ;;
    TERMINAL_PUBLISH)      echo '^terminal_publish:[[:space:]]*false' ;;
    RECLAIM_LEASE_TTL)     echo '^[[:space:]]+lease_ttl:[[:space:]]*72' ;;
    RECLAIM_AUTO)          echo '^[[:space:]]+auto:[[:space:]]*false' ;;
    SKILL_BRAINSTORM)      echo '^[[:space:]]+brainstorm:[[:space:]]*superpowers:brainstorming' ;;
    SKILL_PLAN)            echo '^[[:space:]]+plan:[[:space:]]*superpowers:writing-plans' ;;
    SKILL_BUILD)           echo '^[[:space:]]+build:[[:space:]]*superpowers:subagent-driven-development' ;;
    SKILL_REVIEW)          echo '^[[:space:]]+review:[[:space:]]*superpowers:requesting-code-review' ;;
    SKILL_FINISH)          echo '^[[:space:]]+finish:[[:space:]]*superpowers:finishing-a-development-branch' ;;
    *) echo '' ;;
  esac
}

# Drive the loop off the resolver's ACTUAL export surface, never a hand-copied list.
for k in $(printf '%s\n' "$exp_none" | sed -n 's/^\([A-Z_][A-Z_0-9]*\)=.*/\1/p'); do
  case " $exported_skip " in *" $k "*) continue ;; esac
  re="$(map_for "$k")"
  assert "completeness: export key $k is mapped" '[ -n "$re" ]'
  [ -n "$re" ] && assert "completeness: $k present in example" 'grep -Eq "$re" "$EX"'
done

# (2b) NON-EXPORTED schema keys. These have NO export key, so (2a) is structurally blind to
# them; without this explicit list the "canonical" reference silently ships incomplete.
#   github_project                — fenced by the resolver, never emitted; consumed by github-mirror.sh
#   agents / agent_harnesses      — consumed by sync-agents.sh; ship COMMENTED (presence-sensitive)
#   finalize.require_pr_approval  — MODEL-READ: skills/docket-finalize-change/SKILL.md only
assert "completeness: github_project present (auto sentinel)" \
  'grep -Eq "^github_project:[[:space:]]*auto" "$EX"'
assert "completeness: agent_harnesses present (commented)" \
  'grep -Eq "^#[[:space:]]*agent_harnesses:[[:space:]]*\[[[:space:]]*claude[[:space:]]*\]" "$EX"'
assert "completeness: agents present (commented)" \
  'grep -Eq "^#[[:space:]]*agents:" "$EX"'
assert "completeness: finalize.require_pr_approval present" \
  'grep -Eq "^[[:space:]]+require_pr_approval:[[:space:]]*false" "$EX"'

# require_pr_approval is model-read, so nothing but this assert couples the example to the skill
# that consumes it. Anchor on the PRODUCER (the skill body) so the pair cannot silently diverge.
assert "require_pr_approval is still read by the finalize skill body" \
  'grep -q "require_pr_approval" "$REPO/skills/docket-finalize-change/SKILL.md"'

# The standing rule is STATED in the header (and enforced by the loop above).
assert "example header states the must-update rule" \
  'grep -Eqi "every new config flag lands in" "$EX"'
assert "example documents the four layers" \
  'grep -qF ".docket.local.yml" "$EX" && grep -qF "config.yml" "$EX"'

# Scope tags: both forms present, and every ACTIVE top-level key is tagged.
assert "scope tag: repo-only form present"  'grep -qF "scope: repo-only (coordination-fenced, ADR-0019)" "$EX"'
assert "scope tag: any-layer form present"  'grep -qF "scope: any layer" "$EX"'
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_docket_yml_example.sh 2>&1 | grep 'NOT OK'`

Expected: any key you omitted or mis-valued in Task 2 now names itself. If Task 2's example was complete, this section passes immediately — in that case **prove non-vacuity** by deleting the `require_pr_approval` line from the example, re-running (assert flips to `NOT OK`), and restoring it.

- [ ] **Step 3: Fix the example until green**

Add whatever the failures name. The mapping regexes are the contract for *how* each key must be spelled in the example; adjust indentation in the example to match, not the regex to match a typo.

- [ ] **Step 4: Run the full test to verify it passes**

Run: `bash tests/test_docket_yml_example.sh; echo "EXIT=$?"`

Expected: every line `ok -`, `EXIT=0`.

- [ ] **Step 5: Commit**

```bash
git add tests/test_docket_yml_example.sh .docket.yml.example
git commit -m "test(0101): completeness — exported keys + the four non-exported schema keys

Drives the exported half off the resolver's actual export surface, so a new
export key with no mapping fails the suite. The non-exported half is an
explicit list: github_project, agents, agent_harnesses, and the model-read
finalize.require_pr_approval."
```

---

### Task 4: Presence-sensitivity, the ADR-0039 mirror, and the resolver round-trip

**Files:**
- Modify: `tests/test_docket_yml_example.sh` (append before `exit $fail`)

**Interfaces:**
- Consumes: `EX`, `assert`, `REPO`, `tmp` from Task 2.
- Produces: the relocated ADR-0039 coupling (example's commented `agents.claude` block == `agents/docket-*.md` frontmatter) and the guarantee that the commented blocks are valid YAML that resolves once uncommented. Task 5 deletes `tests/test_config_example.sh`, which is where these assertions live today — they must be green here **before** that deletion.

- [ ] **Step 1: Write the failing test**

Append to `tests/test_docket_yml_example.sh`, before `exit $fail`:

```bash
# --- (3) PRESENCE-SENSITIVE keys ship COMMENTED ------------------------------
# Regression guard for a real break (change 0048): gating per-repo generation on file PRESENCE
# littered wrappers into change-tracking-only repos and flipped their --check from no-op to
# failing. An ACTIVE agents:/agent_harnesses: header in this example would re-arm that hazard
# for anyone who copies the file wholesale. See the opt-in-signal-not-file-presence learning.
assert "no ACTIVE agents: header"          '! grep -Eq "^agents:[[:space:]]*$" "$EX"'
assert "no ACTIVE agent_harnesses: header" '! grep -Eq "^agent_harnesses:" "$EX"'
assert "no ACTIVE codex: header"           '! grep -Eq "^[[:space:]]*codex:[[:space:]]*$" "$EX"'
assert "no ACTIVE cursor: header"          '! grep -Eq "^[[:space:]]*cursor:[[:space:]]*$" "$EX"'
assert "PRESENCE-SENSITIVE marker present (agents + agent_harnesses)" \
  '[ "$(grep -cF "PRESENCE-SENSITIVE: uncommenting this key changes behavior" "$EX")" -ge 2 ]'
# ...but the commented examples ARE present, so a user can find and enable them.
assert "commented codex example present"  'grep -Eq "^#[[:space:]]*codex:" "$EX"'
assert "commented cursor example present" 'grep -Eq "^#[[:space:]]*cursor:" "$EX"'

# --- (4) MIRROR EQUALITY: relocated ADR-0039 ---------------------------------
# The commented agents.claude block mirrors agents/docket-*.md wrapper frontmatter VALUE FOR
# VALUE. The wrappers LEAD; this file mirrors. Same field regex as sync-agents.sh's field_of(),
# so the test cannot accept a shape the real resolver would reject.
fm(){ sed -n "s/^$2:[[:space:]]*//p" "$1" | head -n1 | sed 's/[[:space:]]*$//'; }
# The example's agent lines are COMMENTED, so strip a leading '# ' before matching.
ex_field(){ # $1=agent  $2=field(model|effort)
  local line
  line="$(sed -E 's/^[[:space:]]*#[[:space:]]?//' "$EX" | grep -E "^    $1:[[:space:]]" | head -n1)"
  printf '%s' "$line" | sed -nE "s/.*[{,[:space:]]$2[[:space:]]*:[[:space:]]*([A-Za-z0-9._-]+).*/\1/p" | head -n1
}
for a in status adr brainstorm-consultant auto-groom auto-groom-critic \
         implement-next rebase-resolver integration-repair finalize-change; do
  w="$REPO/agents/docket-$a.md"
  assert "$a: wrapper exists" '[ -f "$w" ]'
  assert "$a: model mirrors wrapper" '[ -n "$(ex_field "$a" model)" ] && [ "$(ex_field "$a" model)" = "$(fm "$w" model)" ]'
  assert "$a: effort mirrors wrapper" '[ -n "$(ex_field "$a" effort)" ] && [ "$(ex_field "$a" effort)" = "$(fm "$w" effort)" ]'
done

# --- (5) RESOLVER ROUND-TRIP (retained from tests/test_config_example.sh) ----
# Uncomment the agents: block + the cursor block and enable cursor — the example IDs must resolve
# through the REAL resolver (sync-agents.sh) into a cursor wrapper. Proves the commented blocks
# are valid YAML, not decorative prose.
SB="$(mktemp -d)"; _sbs="$SB"
mkdir -p "$SB/.claude/agents" "$SB/.cursor/agents" "$SB/.config/docket"
# Uncomment: the agents: block, its claude children, and the cursor block; then enable cursor.
sed -E 's/^#[[:space:]]?(agents:)/\1/; s/^#[[:space:]]?(  )/\1/; s/^#[[:space:]]?(agent_harnesses:.*)/\1/' "$EX" \
  | sed -E 's/^[[:space:]]*#[[:space:]]?(  cursor:)/\1/; s/^[[:space:]]*#[[:space:]]?(    [a-z-]+:[[:space:]]*\{)/\1/' \
  | sed -E 's/^agent_harnesses:.*/agent_harnesses: [claude, cursor]/' \
  > "$SB/.config/docket/config.yml"
err="$(cd "$SB" && HOME="$SB" XDG_CONFIG_HOME="$SB/.config" DOCKET_HARNESS_ROOT="$SB" \
       bash "$REPO/sync-agents.sh" 2>&1 >/dev/null)"; rc=$?
assert "round-trip: sync-agents resolves the uncommented example (exit 0)" '[ "$rc" = "0" ]'
assert "round-trip: no unknown-harness-token warning" \
  '! printf "%s" "$err" | grep -qiE "unknown agent_harnesses token"'
assert "round-trip: a claude wrapper was generated" '[ -f "$SB/.claude/agents/docket-status.md" ]'
assert "round-trip: claude status model mirrors the built-in" \
  '[ "$(fm "$SB/.claude/agents/docket-status.md" model)" = "$(fm "$REPO/agents/docket-status.md" model)" ]'
assert "round-trip: a cursor wrapper was generated" '[ -f "$SB/.cursor/agents/docket-status.md" ]'
assert "round-trip: cursor status model came from the example block" \
  '[ "$(fm "$SB/.cursor/agents/docket-status.md" model)" = "grok-4.5-fast-medium" ]'
rm -rf "$_sbs"
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_docket_yml_example.sh 2>&1 | grep -E 'NOT OK|round-trip'`

Expected: the round-trip asserts fail first — the `sed` uncomment pipeline is the fragile part and almost certainly needs adjusting to the exact comment indentation Task 2 wrote.

- [ ] **Step 3: Fix the uncomment pipeline against the real file**

Debug it directly rather than guessing:

```bash
sed -E 's/^#[[:space:]]?(agents:)/\1/; s/^#[[:space:]]?(  )/\1/' .docket.yml.example | sed -n '/^agents:/,/^[^[:space:]#]/p'
```

Expected: a well-formed `agents:` block with `  claude:` and nine 4-space-indented agent lines. Adjust the `sed` expressions until it is. If the example's comment style makes a robust one-liner impossible, change the **example's** comment indentation to a uniform `# ` prefix — a file meant to be uncommented by hand should be uncommentable by machine too.

- [ ] **Step 4: Run the test to verify it passes**

Run: `bash tests/test_docket_yml_example.sh; echo "EXIT=$?"`

Expected: every assert `ok -`, `EXIT=0`.

Then prove non-vacuity on the mirror coupling: temporarily change one model value in the example's commented `claude:` block, re-run, confirm that agent's `model mirrors wrapper` assert flips to `NOT OK`, and restore.

- [ ] **Step 5: Commit**

```bash
git add tests/test_docket_yml_example.sh .docket.yml.example
git commit -m "test(0101): presence-sensitivity, ADR-0039 mirror equality, resolver round-trip

Migrates the assertions that live in tests/test_config_example.sh today, so
that file can be deleted next task. The presence-sensitivity asserts are a
regression guard for change 0048's tracking-only-repo break."
```

---

### Task 5: Delete `config.yml.example`; scaffold a minimal global config

**Files:**
- Delete: `config.yml.example`
- Delete: `tests/test_config_example.sh`
- Modify: `scripts/ensure-global-config.sh`
- Modify: `scripts/ensure-global-config.md`
- Modify: `install.sh:6`
- Modify: `tests/test_docket_yml_example.sh` (append the scaffold-shape assert)
- Audit: `tests/test_install.sh`, `tests/test_ensure_global_config.sh`

**Interfaces:**
- Consumes: Task 4's migrated assertions (they must be green here before the old test file is deleted).
- Produces: `ensure-global-config.sh` writing a pointer-only global config with **zero active keys**.

- [ ] **Step 1: Write the failing test**

Append to `tests/test_docket_yml_example.sh`, before `exit $fail`:

```bash
# --- (6) SCAFFOLD SHAPE: install writes a POINTER, never pinned values -------
# Why this guard exists: the old scaffold COPIED config.yml.example, so a user installed once and
# then carried a frozen snapshot of that day's defaults forever — every later default change was
# silently pinned by their stale copy. The scaffold must therefore write NO active keys at all.
SC="$(mktemp -d)"; _scs="$SC"
out="$(HOME="$SC" DOCKET_HARNESS_ROOT="$SC" XDG_CONFIG_HOME="$SC/.config" \
       bash "$REPO/scripts/ensure-global-config.sh" 2>&1)"; scrc=$?
GC="$SC/.config/docket/config.yml"
assert "scaffold: exits 0"            '[ "$scrc" = "0" ]'
assert "scaffold: wrote the file"     '[ -f "$GC" ]'
# "No active keys" = every non-blank line is a comment.
assert "scaffold: contains NO active keys (comment/blank lines only)" \
  '[ -z "$(grep -vE "^[[:space:]]*(#.*)?$" "$GC" 2>/dev/null)" ]'
assert "scaffold: points at .docket.yml.example" 'grep -qF ".docket.yml.example" "$GC"'
assert "scaffold: names the layer precedence"    'grep -qiE "repo-local|precedence" "$GC"'
# Idempotent + non-destructive: a second run leaves an existing file byte-untouched.
printf '# user edited\nauto_capture: true\n' > "$GC"
before="$(cat "$GC")"
HOME="$SC" DOCKET_HARNESS_ROOT="$SC" XDG_CONFIG_HOME="$SC/.config" \
  bash "$REPO/scripts/ensure-global-config.sh" >/dev/null 2>&1
assert "scaffold: existing user config left byte-untouched" '[ "$(cat "$GC")" = "$before" ]'
rm -rf "$_scs"

# The deleted surfaces stay deleted.
assert "config.yml.example is gone"          '[ ! -f "$REPO/config.yml.example" ]'
assert "tests/test_config_example.sh is gone" '[ ! -f "$REPO/tests/test_config_example.sh" ]'
assert "no stale config.yml.example reference in install.sh" \
  '! grep -qF "config.yml.example" "$REPO/install.sh"'
assert "no stale config.yml.example reference in ensure-global-config.sh" \
  '! grep -qF "config.yml.example" "$REPO/scripts/ensure-global-config.sh"'
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_docket_yml_example.sh 2>&1 | grep 'NOT OK'`

Expected: `scaffold: contains NO active keys`, `config.yml.example is gone`, `tests/test_config_example.sh is gone`, and both stale-reference asserts all fail.

- [ ] **Step 3: Rewrite the scaffold**

Replace the body of `scripts/ensure-global-config.sh` after the `HARNESS_ROOT`/`DEST` resolution. Remove the `SRC` variable and its existence check entirely; write a heredoc instead of copying:

```bash
#!/usr/bin/env bash
# ensure-global-config.sh — scaffold the global docket config on first run.
#
# Writes a MINIMAL, pointer-only ${XDG_CONFIG_HOME:-$HOME/.config}/docket/config.yml — a header
# comment naming .docket.yml.example as the reference, and ZERO active keys — but ONLY if that
# file does not already exist. Never overwrites, never merges, never edits an existing file.
# Idempotent: safe to re-run any number of times. Run by install.sh BEFORE sync-agents.sh.
#
# Why pointer-only (change 0101): the previous version COPIED config.yml.example, so a user who
# installed once carried a frozen snapshot of that day's defaults forever — every later default
# change was silently pinned by their stale copy. A file with no active keys cannot pin anything.
#
# Test seam: DOCKET_HARNESS_ROOT overrides $HOME for the config root (matching sync-agents.sh),
# and it is only consulted when XDG_CONFIG_HOME is unset (a set XDG_CONFIG_HOME wins).
set -euo pipefail

HARNESS_ROOT="${DOCKET_HARNESS_ROOT:-$HOME}"
DEST_DIR="${XDG_CONFIG_HOME:-$HARNESS_ROOT/.config}/docket"
DEST="$DEST_DIR/config.yml"

if [ -e "$DEST" ]; then
  echo "docket: $DEST already exists — left untouched"
  exit 0
fi

mkdir -p "$DEST_DIR"
cat > "$DEST" <<'EOF'
# ~/.config/docket/config.yml — docket's GLOBAL (per-machine, every-repo) configuration.
#
# This file is intentionally EMPTY: every key is unset, so docket runs its shipped defaults.
# Add only the keys you want to change on this machine.
#
# Configuration resolves PER KEY, precedence highest to lowest:
#   1. repo-local     <repo>/.docket.local.yml   (this machine, this repo; gitignored)
#   2. repo-committed <repo>/.docket.yml         (every clone)
#   3. global         this file                  (this machine, every repo)
#   4. built-in       docket's defaults
#
# FOR EVERY KEY, ITS DEFAULT, AND WHICH LAYERS MAY SET IT, SEE:
#   .docket.yml.example  in the docket repo — the canonical, all-comprehensive reference.
#
# Keys tagged "scope: repo-only (coordination-fenced, ADR-0019)" there are NOT settable here:
# a value for one of those in this file is loudly warned-and-ignored. Everything else is fair game.
#
# Common things to set on this machine:
#   agent_harnesses: [claude, cursor]   # enable another harness (then also set agents: below)
#   agents:                             # per-skill model/effort overrides
#   auto_capture: true                  # mint discovered follow-up work as stubs
#   reclaim: {auto: true}               # let expired claims self-heal
EOF
echo "docket: wrote $DEST (empty pointer config — see .docket.yml.example for every key)"
exit 0
```

- [ ] **Step 4: Update the two contracts and delete the old surfaces**

In `install.sh`, line 6, replace:

```
#   2. ensure-global-config.sh — scaffold ~/.config/docket/config.yml from config.yml.example on
```

with:

```
#   2. ensure-global-config.sh — scaffold a minimal pointer-only ~/.config/docket/config.yml on
```

Update `scripts/ensure-global-config.md` to match the new behavior: it no longer has a source file, it writes a fixed heredoc with zero active keys, and its Invariants gain "the scaffolded file contains no active keys, so it can never pin a shipped default." Remove every `config.yml.example` mention.

Then delete both old surfaces:

```bash
git rm config.yml.example tests/test_config_example.sh
```

- [ ] **Step 5: Audit every test that reaches the scaffold**

This is the `config-layer-write-and-read-hazards` guard — the scaffold writes to a shared per-user location, so any test reaching it without a pinned `XDG_CONFIG_HOME`/`DOCKET_HARNESS_ROOT` can rewrite a developer's real global config.

Run: `grep -rn 'ensure-global-config\|XDG_CONFIG_HOME\|DOCKET_HARNESS_ROOT' tests/test_install.sh tests/test_ensure_global_config.sh`

Confirm every invocation pins both env vars to a temp dir. `tests/test_ensure_global_config.sh` additionally asserts the *old* copy-from-source behavior — update its assertions to the pointer-only shape (no source file, no active keys), and confirm it does not assert `config.yml.example` exists.

- [ ] **Step 6: Run the affected tests to verify they pass**

Run: `for t in test_docket_yml_example test_ensure_global_config test_install; do echo "== $t"; bash "tests/$t.sh" 2>&1 | grep -E 'NOT OK' || echo "all ok"; done`

Expected: `all ok` for all three.

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "refactor(0101): delete config.yml.example; scaffold a pointer-only global config

The old scaffold copied config.yml.example, freezing that day's defaults into
every user's global config forever. It now writes a header + pointer with zero
active keys, so it can never pin a shipped default. Deletes config.yml.example
and tests/test_config_example.sh, whose assertions moved to
tests/test_docket_yml_example.sh in the previous task."
```

---

### Task 6: Slim this repo's `.docket.yml`; retarget the README

**Files:**
- Modify: `.docket.yml`
- Modify: `README.md:126` and the `### 2. Set up your global config` section (~line 132)
- Modify: `tests/test_docket_yml_example.sh` (append the README wiring assert)

**Interfaces:**
- Consumes: `.docket.yml.example` from Task 2 (the file the README and `.docket.yml` now point at).
- Produces: the final state — no surviving reference to `config.yml.example` anywhere in the repo.

- [ ] **Step 1: Write the failing test**

Append to `tests/test_docket_yml_example.sh`, before `exit $fail`:

```bash
# --- (7) README + dogfooding -------------------------------------------------
README="$REPO/README.md"
assert "README has the step-2 global-config heading" 'grep -qF "### 2. Set up your global config" "$README"'
assert "README step-2 names .docket.yml.example"     'grep -qF ".docket.yml.example" "$README"'
assert "README no longer names config.yml.example"   '! grep -qF "config.yml.example" "$README"'

# Dogfooding: this repo's own .docket.yml carries ONLY the values it actually sets, plus a
# pointer to the example. It is the copy-out workflow's worked demonstration, so it must not
# regress into a second all-keys surface — that drift is exactly what change 0101 ended.
DY="$REPO/.docket.yml"
assert "repo .docket.yml points at the example" 'grep -qF ".docket.yml.example" "$DY"'
assert "repo .docket.yml is slim (<= 40 lines)"  '[ "$(wc -l < "$DY")" -le 40 ]'
assert "repo .docket.yml keeps its set values" \
  'grep -Eq "^metadata_branch:[[:space:]]*docket" "$DY" && grep -Eq "^terminal_publish:[[:space:]]*true" "$DY"'
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_docket_yml_example.sh 2>&1 | grep 'NOT OK'`

Expected: `README step-2 names .docket.yml.example`, `README no longer names config.yml.example`, `repo .docket.yml points at the example`, and `repo .docket.yml is slim` all fail (`.docket.yml` is currently 130 lines).

- [ ] **Step 3: Slim `.docket.yml`**

Replace the whole file with only the values this repo actually sets, one-line comments, and a header pointer:

```yaml
# .docket.yml — committed on the repo's default branch; read by every docket skill at startup.
#
# This file carries ONLY the values docket's own repo overrides. For EVERY key, its default, its
# documentation, and which layers may set it, see .docket.yml.example — the canonical reference.
# Copy the keys you want from there; anything not set here runs docket's shipped default.
#
# This repo runs in DOCKET-MODE (migrated 2026-06-04 via migrate-to-docket.sh — the dogfood of
# change 0002): planning metadata is the live surface on the `docket` branch, while `main` keeps
# code, build artifacts, and published terminal records.

metadata_branch: docket        # planning commits land on the `docket` branch
integration_branch: main       # code lands on main; feature branches cut from origin/main
changes_dir: docs/changes
adrs_dir: docs/adrs
results_dir: docs/results

finalize:
  gate: local                  # rebase onto main + re-run the suite locally before merging

board_surfaces: [inline]       # BOARD.md only; the GitHub mirror stays off

# This repo mirrors its terminal records (archived change files, specs, Accepted ADRs) onto main,
# so it opts in to the direct-commit publish explicitly. See .docket.yml.example for the caveats.
terminal_publish: true
```

Everything removed here — the `learnings`, `reclaim`, `agents`, `agent_harnesses`, and `skills` commented blocks — now lives in `.docket.yml.example`. Verify nothing removed was actually *set*: every one of those blocks was commented out in the original file, so this is a documentation move, not a config change. The fidelity test does not cover this file, so confirm by hand:

```bash
git diff .docket.yml | grep '^-' | grep -vE '^-[[:space:]]*#' | grep -vE '^--- '
```

Expected: only the reformatted lines for keys that remain set (`metadata_branch`, `integration_branch`, the three dirs, `finalize`/`gate`, `board_surfaces`, `terminal_publish`) — **no line for a key that disappears entirely**. If any other active key shows up, put it back.

- [ ] **Step 4: Retarget the README**

In `README.md` line 126, replace the `ensure-global-config.sh` bullet:

```markdown
- **`ensure-global-config.sh`** drops a minimal starter `~/.config/docket/config.yml` into place the first time you install — non-destructively (an existing config is left untouched). It contains no active keys: it is a header plus a pointer to [`.docket.yml.example`](.docket.yml.example), docket's canonical reference for every key and its default (see step 2). It runs before `sync-agents.sh` so the generator reads the just-written config.
```

And rewrite the `### 2. Set up your global config` section body:

```markdown
### 2. Set up your global config

`install.sh` writes a minimal `~/.config/docket/config.yml` the first time it runs (and leaves an existing one untouched). It ships with **no active keys** — docket's defaults already apply, so most users never edit it.

The canonical reference for every key is [`.docket.yml.example`](.docket.yml.example) in this repo: every config key, active at its shipped default, with full documentation and a scope tag saying which layers may set it. Copy the keys you want to change into the layer you want them in.

- **To see docket's built-in per-skill model and effort:** the example's commented `agents.claude` block mirrors the shipped defaults for all nine subagents, so you can read and tune them in one place instead of opening nine wrapper files.
- **Claude-only users can skip this entirely** — the defaults already apply.
- **To enable another harness (Cursor, Codex):** uncomment `agent_harnesses` and add the harness, **and** uncomment that harness's block under `agents:`, then re-run `install.sh` so `sync-agents.sh` regenerates the wrappers. Both keys are **presence-sensitive** — uncommenting either opts the repo into per-repo wrapper generation even at default values.

See [Configuration](#configuration--docketyml-global-config-and-machine-local-overrides) for the layer model.
```

- [ ] **Step 5: Run the full suite to verify everything passes**

Run the whole repo suite in ONE foreground call — several tests read `.docket.yml` and the README, so a targeted run can miss a break:

```bash
for t in tests/test_*.sh; do
  out="$(bash "$t" 2>&1)"; rc=$?
  if [ $rc -ne 0 ] || printf '%s' "$out" | grep -q 'NOT OK'; then
    echo "=== FAIL: $t (exit $rc)"; printf '%s\n' "$out" | grep -E 'NOT OK|Error|error:' | head -20
  fi
done; echo "SUITE SWEEP DONE"
```

Expected: `SUITE SWEEP DONE` with no `=== FAIL:` lines.

Pay particular attention to `tests/test_readme_finalize_docs.sh`, `tests/test_script_contracts_coverage.sh`, and `tests/test_docket_root.sh` — all three read repo-root files this task changed.

- [ ] **Step 6: Commit**

```bash
git add .docket.yml README.md tests/test_docket_yml_example.sh
git commit -m "docs(0101): slim this repo's .docket.yml; retarget the README

.docket.yml now carries only the values this repo actually overrides plus a
pointer to .docket.yml.example — dogfooding the copy-out workflow. README
step 2 and the ensure-global-config.sh bullet retarget from the deleted
config.yml.example to the new canonical reference."
```

---

## Post-plan notes for the implementer

**The ADR is authored at review time, not by a task.** Change 0101 produces a new ADR that **supersedes ADR-0039**: the mirror rule survives relocated (the example's commented `agents.claude` block mirrors wrapper frontmatter; wrappers lead), joined by two new invariants — **example = resolver defaults** (test-enforced) and the **must-update rule**. It is authored by dispatching the `docket-adr` subagent during the implementer's review step. Both `39` and the new id must end up in change 0101's `adrs:` so terminal-publish copies ADR-0039's changed `status:` line and the new ADR onto `main` atomically at merge (see the `adr-update-delivery` learning).

**Suite runtime.** The full suite takes roughly ten minutes. Run it in ONE foreground call — never backgrounded.

---

## Self-Review

**1. Spec coverage.** Every spec section maps to a task:

| Spec item | Task |
|---|---|
| Decision 1 — one canonical reference, Helm-values style | 2 |
| Decision 2 — `config.yml.example` deleted | 5 |
| Decision 3 — per-key scope tags | 2 (written), 3 (asserted) |
| Decision 4 — presence-sensitive keys commented + marker | 2 (written), 4 (asserted) |
| Decision 5 — `auto` sentinel, both keys | 1 (`test_command` code + both contracts), 2/3 (shipped + asserted) |
| Decision 6 — `install.sh` scaffolds a minimal global config | 5 |
| Decision 7 — this repo's `.docket.yml` slims | 6 |
| Decision 8 — the standing rule, stated + enforced | 2 (header), 3 (enforcement loop) |
| §The file — header block, four layers, copy-out, scope legend | 2 |
| §Body — full key inventory | 2 (table), 3 (completeness) |
| §Resolver change — `auto` sentinel | 1 |
| §Consolidation edits — delete, install.sh, `.docket.yml`, README, ADR | 5, 6, post-plan note |
| §Tests 1–7 | 2 (t1), 3 (t2), 4 (t3/t4/t5), 5 (t6), 6 (t7) |
| Reconcile (A) — asymmetric sentinels | 1 (explicitly split: code vs doc-only) |
| Reconcile (B) — non-exported key list | 3 (§2b) |

No gaps.

**2. Placeholder scan.** Every code step carries real content. The one place the plan describes rather than dictates is the per-key *prose* in `.docket.yml.example` (Task 2, Step 3) — deliberate, because that prose is **absorbed verbatim from two named files at named line ranges**, and transcribing ~250 lines of existing documentation into the plan would invite drift between plan and source. The plan pins what is actually load-bearing: the exact header text, the exact key/value/scope inventory as a table, the exact marker wording, the exact new prose for the two keys with no prior documentation, and the exact sourcing instructions.

**3. Type consistency.** Shell helper names are consistent across tasks: `assert`, `mkrepo`, `norm`, `map_for`, `fm`, `ex_field`, and the variables `EX`, `REPO`, `CFGSCRIPT`, `tmp`, `exp_none`, `exp_full`. `fm()` is defined in Task 4 and used only in Task 4. `exp_none` is defined in Task 2 and reused by Task 3's completeness loop — the one cross-task dependency, and it is declared in Task 3's Interfaces block. Task 2's `ex_field` deliberately differs from the deleted test's `cfg_field` because the example's agent lines are commented and need a `# ` strip first.
