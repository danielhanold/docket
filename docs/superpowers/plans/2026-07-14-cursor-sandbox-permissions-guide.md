# Cursor sandbox & permissions guide — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship docket's Cursor Auto-run guidance — one guide, two copyable JSON fragments, a README pointer, and a mutation-tested guard suite — all derived from the change's live-verification appendix (Cursor 3.11.19, 2026-07-14).

**Architecture:** Three user-facing docs under `docs/cursor/` plus a README pointer, built on the feature branch like code. A single hermetic guard file `tests/test_cursor_permissions_docs.sh` enforces structure (JSON parses, the fragment carries the four observed facade spellings, the guide's fenced JSON is byte-identical to the files, every troubleshooting entry is provenance-stamped, the never-allowlist set matches `scripts/docket.md`, and README links the guide). The guards prove **structure, never truth** — the truth of each classifier claim is validated by the human reading the guide against the spec's verification-log appendix at the merge gate.

**Tech Stack:** Markdown docs; JSON fragments; POSIX/bash hermetic test script (`python3 -m json.tool` or `jq` for JSON parsing; `awk`/`grep`/parameter-expansion for extraction). No network, no new dependencies.

## Global Constraints

- **Target Cursor mode:** publish for **Allowlist (with Sandbox)** + a `permissions.json` `terminalAllowlist` for the facade + a `sandbox.json` read-path fragment. Do **not** claim Auto-review composes with a file allowlist (it does not in 3.11.19). Do **not** publish or recommend `approvalMode`.
- **The four facade spellings** (appendix §G) — publish exactly these in `terminalAllowlist`, all four, no more, no fewer:
  1. `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh` (canonical guarded-expansion)
  2. `"${DOCKET_SCRIPTS_DIR:?}"/docket.sh` (short form — `/docket.sh` **outside** the closing quote)
  3. `"${DOCKET_SCRIPTS_DIR:?}/docket.sh"` (short form — `/docket.sh` **inside** the closing quote)
  4. `/Users/$USER/dev/docket/scripts/docket.sh` (resolved-absolute; `$USER`/`$HOME` placeholders only — **never** a real username or home dir)
- **Provenance:** every troubleshooting entry carries the stamp `Cursor 3.11.19 · 2026-07-14`. Assert nothing the appendix does not record; a mode that did not reproduce is **cut**, not softened. (The appendix cut two candidates — protected `.git` writes, and "network blocked while allowlisted" as stated — so they get **no** troubleshooting entry.)
- **Guards are code:** every assertion in `tests/test_cursor_permissions_docs.sh` MUST be mutation-tested (strip the thing it guards, watch it redden) before it is trusted. Derive sets by grep from the authoritative file at test time — never hand-list or retype what a source of truth already declares. Under `set -o pipefail`, never pipe a producer into an early-exiting consumer (`grep -q`, `head`) — capture into a variable first. Prove each assert can FIRE (the derived token is non-empty) before trusting that it passes.
- **Placeholders only for identity:** paths use `/Users/$USER/dev/docket/...`; never a resolved home directory or real username, in any doc or JSON file.
- **`docs/cursor/` does not exist on `origin/main`** — this change creates it fresh; there is no prior file to reconcile against.

---

### Task 1: The two JSON fragments + parse & spelling guards

**Files:**
- Create: `docs/cursor/permissions.example.json`
- Create: `docs/cursor/sandbox.example.json`
- Test: `tests/test_cursor_permissions_docs.sh` (create; assertions #1 and #2)

**Interfaces:**
- Produces: two JSON files whose exact bytes Task 2 embeds byte-identically into the guide; a guard file Tasks 2–3 extend.
- Consumes: `skills/docket-convention/SKILL.md` (authoritative source of the canonical guarded spelling — derived, never retyped).

- [ ] **Step 1: Write `docs/cursor/permissions.example.json`**

Exact bytes (note JSON-escaped `\"` around each shell parameter expansion; file ends with a single trailing newline):

```json
{
  "terminalAllowlist": [
    "\"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}\"/docket.sh",
    "\"${DOCKET_SCRIPTS_DIR:?}\"/docket.sh",
    "\"${DOCKET_SCRIPTS_DIR:?}/docket.sh\"",
    "/Users/$USER/dev/docket/scripts/docket.sh"
  ]
}
```

- [ ] **Step 2: Write `docs/cursor/sandbox.example.json`**

Exact bytes (read path to the docket clone, which lives outside the workspace; `$USER` placeholder; single trailing newline):

```json
{
  "additionalReadonlyPaths": [
    "/Users/$USER/dev/docket"
  ]
}
```

- [ ] **Step 3: Write the failing guard file `tests/test_cursor_permissions_docs.sh` (assertions #1 + #2)**

Create the file with a hermetic harness that resolves `REPO` from its own location, then assertions #1 (both JSON files parse) and #2 (the fragment carries all four observed facade spellings, canonical derived from the convention). Make it executable.

```bash
#!/usr/bin/env bash
# tests/test_cursor_permissions_docs.sh — guards for the Cursor permissions guide (change 0073).
# Structure only: these prove a claim is stamped / a set is complete / JSON parses. They CANNOT
# prove a classifier claim is TRUE — that is validated by the human reading docs/cursor/permissions.md
# against the spec's verification-log appendix at the merge gate (LEARNINGS, verify-the-claim family).
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
PERMS_JSON="$REPO/docs/cursor/permissions.example.json"
SANDBOX_JSON="$REPO/docs/cursor/sandbox.example.json"
GUIDE="$REPO/docs/cursor/permissions.md"
CONV="$REPO/skills/docket-convention/SKILL.md"
FACADE_DOC="$REPO/scripts/docket.md"
README="$REPO/README.md"
fail=0
ok(){ echo "ok - $1"; }
no(){ echo "NOT OK - $1"; fail=1; }

# JSON parser seam: prefer jq, fall back to python3.
json_ok(){ # $1 = file
  if command -v jq >/dev/null 2>&1; then jq -e . "$1" >/dev/null 2>&1
  else python3 -m json.tool "$1" >/dev/null 2>&1; fi
}

# --- Assertion 1: both example JSON fragments parse -------------------------------------------
if json_ok "$PERMS_JSON"; then ok "permissions.example.json parses"; else no "permissions.example.json parses"; fi
if json_ok "$SANDBOX_JSON"; then ok "sandbox.example.json parses"; else no "sandbox.example.json parses"; fi

# --- Assertion 2: the fragment carries all four observed facade spellings ---------------------
# Canonical guard token is DERIVED from the convention (authoritative), never retyped here.
CANON="$(grep -oE '\$\{DOCKET_SCRIPTS_DIR:\?run docket/install\.sh\}' "$CONV")"
CANON="${CANON%%$'\n'*}"   # first match; no pipe-to-head (pipefail-safe)
if [ -n "$CANON" ]; then ok "canonical guard token derivable from convention"; else no "canonical guard token derivable from convention"; fi
if [ -n "$CANON" ] && grep -qF -- "$CANON" "$PERMS_JSON"; then ok "fragment carries canonical guarded spelling"; else no "fragment carries canonical guarded spelling"; fi
# The three remaining observed forms (short x2, absolute) have no feature-branch source of truth
# but the fragment itself — they are empirical observations from appendix §G. Assert each is present
# (mutation: drop one from the fragment -> reddens). $USER placeholder kept verbatim.
for form in \
  '"${DOCKET_SCRIPTS_DIR:?}"/docket.sh' \
  '"${DOCKET_SCRIPTS_DIR:?}/docket.sh"' \
  '/Users/$USER/dev/docket/scripts/docket.sh'; do
  if grep -qF -- "$form" "$PERMS_JSON"; then ok "fragment carries spelling: $form"; else no "fragment carries spelling: $form"; fi
done

exit $fail
```

- [ ] **Step 4: Run the guard — assertions #1/#2 pass, later assertions absent**

Run: `bash tests/test_cursor_permissions_docs.sh`
Expected: all `ok -` lines for the JSON-parse and spelling checks; exit 0. (Guide/README assertions are added in later tasks.)

- [ ] **Step 5: Mutation-test assertions #1 and #2**

Run each mutation, confirm the named `ok` flips to `NOT OK` (non-zero exit), then revert:
1. Break JSON: delete the final `}` from `permissions.example.json` → "permissions.example.json parses" reddens. Revert.
2. Drop the canonical spelling line from `permissions.example.json` → "fragment carries canonical guarded spelling" reddens. Revert.
3. Drop the `"${DOCKET_SCRIPTS_DIR:?}/docket.sh"` line → the matching `fragment carries spelling:` reddens. Revert.
4. Temporarily rename `CONV` target check by editing the convention grep token to a non-existent string in the test → "canonical guard token derivable from convention" reddens (proves the derivation is live, not vacuous). Revert.

Record the four mutation results (green→red confirmed) before trusting the guards.

- [ ] **Step 6: Commit**

```bash
git add docs/cursor/permissions.example.json docs/cursor/sandbox.example.json tests/test_cursor_permissions_docs.sh
git commit -m "docs(0073): Cursor permissions & sandbox JSON fragments + parse/spelling guards"
```

---

### Task 2: The guide `docs/cursor/permissions.md` + structure guards

**Files:**
- Create: `docs/cursor/permissions.md`
- Modify: `tests/test_cursor_permissions_docs.sh` (add assertions #3, #4, #5)

**Interfaces:**
- Consumes: `docs/cursor/permissions.example.json` and `docs/cursor/sandbox.example.json` (Task 1) — embedded byte-identically; `scripts/docket.md`'s Subcommand inventory + Not-exposed sections (trust-tier derivation).
- Produces: the guide the README points at (Task 3).

- [ ] **Step 1: Write `docs/cursor/permissions.md`**

Author the guide with these sections (grounded strictly in the spec's appendix — assert nothing it does not record). The two fenced ```json blocks MUST be **byte-identical** to the Task 1 files (first fence = permissions, second fence = sandbox), so paste the exact file bytes. Troubleshooting entries are `### ` headings, each closing with a bold `**Observed:** Cursor 3.11.19 · 2026-07-14` stamp line.

Guide content:

````markdown
# Running docket under Cursor Auto-run — sandbox & permissions

docket's runtime is now two command shapes behind one facade (`docket.sh`), which finally makes a
small, stable Cursor permission configuration possible. This guide shows the configuration, explains
why docket must run **outside** Cursor's sandbox, states plainly what one allowlist entry authorizes,
and troubleshoots the failure modes observed in a live session.

> **Provenance.** Every classifier claim below was observed in **Cursor 3.11.19** on **2026-07-14**
> under **Allowlist (with Sandbox)**. Cursor's auto-run classifier is not a documented contract, so
> treat these as empirical claims about that version, and re-verify if your Cursor differs.

## The three gates, and why they are independent

Cursor decides whether an agent command runs, and how, through three independent gates:

1. **Command approval** (`permissions.json` → `terminalAllowlist`) — whether a command auto-runs at
   all, and whether it runs **outside** the sandbox.
2. **Filesystem access** (`sandbox.json` → `additionalReadonlyPaths`) — what a **sandboxed** command
   may read.
3. **Network** — whether a **sandboxed** command may reach the network.

They do not substitute for one another. Granting filesystem or network access to a sandboxed command
does **not** move it outside the sandbox; only a `terminalAllowlist` match does. Config edits to
`~/.cursor/permissions.json` are picked up within a second or two without restarting Cursor (file
watcher).

**Run Modes and the allowlist lock.** When `~/.cursor/permissions.json` defines a non-empty
`terminalAllowlist` (or `mcpAllowlist`), Cursor constrains the selectable Run Modes to **Allowlist**
and **Allowlist (with Sandbox)** only. **Run Everything** is disabled (a banner says so), and
**Auto-review (with Sandbox)** — though still shown — becomes non-selectable. Publishing a facade
allowlist therefore commits you to an Allowlist mode; it does **not** compose with Auto-review the way
the public docs imply. Do **not** try to escape this with an `approvalMode` key — writing
`approvalMode: "unrestricted"` alongside allowlists emptied the Run Mode dropdown entirely (no options
selectable) and had to be removed. The recommended operator mode is **Allowlist (with Sandbox)**.

## Why docket must run outside the sandbox

docket's runtime needs two things a sandbox denies: an **out-of-workspace program**
(`$DOCKET_SCRIPTS_DIR/docket.sh`, in your docket clone outside the repo) and the **network**
(`preflight` fetches and rebases; skills push). A sandboxed docket command fails — typically the
`git fetch` to origin dies and `preflight` exits non-zero — **even when** `sandbox.json` grants a read
path and network access, because the command is still sandboxed. The fix is not more sandbox
permissions; it is a `terminalAllowlist` entry that runs the facade **outside** the sandbox.

## The fragments

Copy these into your Cursor config. Replace `$USER` with your username (or keep it if your shell
expands it), and adjust the absolute path to your actual docket clone.

**`~/.cursor/permissions.json`** — allowlist the facade. Four spellings are listed because agents emit
the facade invocation in all four forms in practice; Cursor prefix-matches the literal command string,
and the short guarded forms are not prefixes of the canonical one, so each must be listed explicitly.

```json
{
  "terminalAllowlist": [
    "\"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}\"/docket.sh",
    "\"${DOCKET_SCRIPTS_DIR:?}\"/docket.sh",
    "\"${DOCKET_SCRIPTS_DIR:?}/docket.sh\"",
    "/Users/$USER/dev/docket/scripts/docket.sh"
  ]
}
```

**`~/.cursor/sandbox.json`** — grant a read path to the docket clone (complementary; it does **not**
move docket out of the sandbox — see above).

```json
{
  "additionalReadonlyPaths": [
    "/Users/$USER/dev/docket"
  ]
}
```

## Trust tiers — docket's shell surface, classified

docket's operations sort into three tiers. The classification is derived from `scripts/docket.md`,
which declares itself the permission inventory — it is not hand-copied here.

- **Daily operations, behind the facade — allowlisted.** Everything an agent runs day to day goes
  through `docket.sh <operation>`. That is the one entry above. The facade is the trust boundary.
- **The human-initiated tier — never allowlisted.** `install.sh`, `migrate-to-docket.sh`,
  `sync-agents.sh`, `ensure-docket-env.sh`, `ensure-claude-settings.sh` — one-time setup and migration
  tools a human runs deliberately. They are never invoked by an agent and never belong in the
  allowlist.
- **Internals the facade must not expose** — `docket-config.sh`, `disable-worktree-hooks.sh`,
  `render-board.sh` (plus `lib/` and the tests). They are reached only indirectly, through the facade
  verbs; naming them directly in an allowlist would route around the boundary.

## What one allowlist entry authorizes

Allowlisting the facade authorizes, **unprompted**, every operation the facade can run — including
destructive and external-writing ones:

- `docket-status`'s guarded sweep — archives merged changes, publishes terminal records onto the
  integration branch, and deletes merged feature branches and worktrees.
- `terminal-publish`'s direct push to the integration branch.
- `github-mirror`'s external writes to GitHub Issues and Projects.
- `cleanup-feature-branch`'s provenance-guarded branch and worktree deletion.

These are shared-history and external writes, and they are the deal you accept for one line of config.
Each is guarded or provenance-checked, which is a mitigation — not a reason to leave the statement out.

## Why the workarounds are not acceptable

It is tempting to allowlist something broader — `eval`, a blanket `bash`, a bootstrap-command prefix,
or a generic command-runner subcommand. Each erases the trust boundary the facade exists to draw and
returns the permission surface to unbounded. The facade deliberately has **no** `run`/`exec`/`shell`/
`eval` operation for exactly this reason; do not reintroduce one at the permission layer.

## Scope — what this fragment does and does not cover

The facade stabilizes docket's own metadata and lifecycle operations. Your repo's **build-time**
commands — feature-branch git, the test suite, `gh` — are that repo's own permission surface. They are
not covered by docket's fragment and not silently granted by it; allowlist them separately according to
your own trust policy. (For example, an agent compound that runs `docket.sh board-refresh` alongside
`git status` needs `git status` allowlisted on its own — the facade entry does not cover it.)

## Troubleshooting

Each entry was reproduced in the live session; the stamp records the Cursor version and date.

### A sandbox grant did not make docket work

You added a read path and network access in `sandbox.json`, but `docket.sh preflight` still fails
(often `git fetch` to origin). Sandbox permissions govern **sandboxed** commands; they do not move a
command outside the sandbox. Only a `terminalAllowlist` match runs the facade unsandboxed. Add the
facade entry to `permissions.json`.

**Observed:** Cursor 3.11.19 · 2026-07-14

### A typo in the guard message breaks the match

An allowlist entry with a typo in the guard text — e.g. `docket/instal.sh` instead of
`docket/install.sh` — does not match the real command; Cursor prefix-matches the literal string, so the
command is demoted to the sandbox and origin fetch fails. Copy the spellings exactly.

**Observed:** Cursor 3.11.19 · 2026-07-14

### A short `:?}` spelling is not matched by the canonical entry

If your allowlist has only the canonical `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh`
form but an agent emits a short guarded form like `"${DOCKET_SCRIPTS_DIR:?}"/docket.sh` or
`"${DOCKET_SCRIPTS_DIR:?}/docket.sh"`, the command still requires approval or is demoted — the short
guard is not a prefix of the canonical one. List all four spellings.

**Observed:** Cursor 3.11.19 · 2026-07-14

### One unmatched command in a compound sandboxes the whole program

A compound command (`"…"/docket.sh env; eval true`) is demoted to the sandbox **as a whole** if any
leaf is unmatched — even a leaf that can never execute (`if false; then eval true; fi; "…"/docket.sh
env`). Keep facade calls as standalone commands, and allowlist any other leaf (e.g. `git status`) on
its own.

**Observed:** Cursor 3.11.19 · 2026-07-14

### Invalid JSON silently disables the whole allowlist

A malformed `permissions.json` (e.g. a truncated trailing `}`) is silently ignored — the allowlist
stops taking effect and every facade call is demoted to the sandbox. Restoring valid JSON restores the
allowlist within a second or two (file watcher; no restart needed). Validate the file after editing.

**Observed:** Cursor 3.11.19 · 2026-07-14

### Allowlisting a helper directly does not work (and should not)

Naming an internal like `docket-config.sh` directly in the allowlist — instead of going through the
facade — leaves it sandboxed (origin fetch fails), because the facade-only allowlist does not cover
per-helper invocations. This is by design: the facade is the boundary. Run everything through
`docket.sh`.

**Observed:** Cursor 3.11.19 · 2026-07-14
````

- [ ] **Step 2: Add assertion #3 (fenced JSON byte-identical to the files)**

Insert into `tests/test_cursor_permissions_docs.sh`, before `exit $fail`. Extraction uses awk (reads to EOF — pipefail-safe) to pull the Nth ```json fenced block; compare byte-for-byte to the file with `diff`.

```bash
# --- Assertion 3: the guide's fenced json blocks are byte-identical to the example files ------
# awk emits the body of the Nth ```json ... ``` fence (no fence lines). Reads to EOF (pipefail-safe).
extract_json_fence(){ # $1 = file, $2 = which (1-based)
  awk -v want="$2" '
    /^```json$/ { infence=1; n++; next }
    /^```$/ { if (infence) { infence=0 }; next }
    infence && n==want { print }
  ' "$1"
}
if diff <(extract_json_fence "$GUIDE" 1) "$PERMS_JSON" >/dev/null; then ok "guide permissions fence == permissions.example.json"; else no "guide permissions fence == permissions.example.json"; fi
if diff <(extract_json_fence "$GUIDE" 2) "$SANDBOX_JSON" >/dev/null; then ok "guide sandbox fence == sandbox.example.json"; else no "guide sandbox fence == sandbox.example.json"; fi
```

- [ ] **Step 3: Add assertion #4 (troubleshooting entry count == stamp count)**

The count is scoped to the `## Troubleshooting` section only (so a stamp appearing in prose elsewhere cannot false-green it). Entries = `### ` headings in that section; stamps = `**Observed:**` lines in that section. Assert equal AND non-zero.

```bash
# --- Assertion 4: every troubleshooting entry carries a provenance stamp ----------------------
# Scope to the ## Troubleshooting section (awk reads to EOF; pipefail-safe).
TROUBLE="$(awk '/^## Troubleshooting$/{f=1;next} /^## /{f=0} f' "$GUIDE")"
ENTRIES="$(grep -cE '^### ' <<<"$TROUBLE")"
STAMPS="$(grep -cE '^\*\*Observed:\*\* Cursor 3\.11\.19' <<<"$TROUBLE")"
if [ "$ENTRIES" -gt 0 ] && [ "$ENTRIES" = "$STAMPS" ]; then ok "troubleshooting entries ($ENTRIES) all stamped ($STAMPS)"; else no "troubleshooting entries ($ENTRIES) all stamped ($STAMPS)"; fi
```

- [ ] **Step 4: Add assertion #5 (never-allowlist set matches `scripts/docket.md`'s Not-exposed set)**

Derive the true not-exposed set from `scripts/docket.md`: take the backtick-wrapped `*.sh` tokens inside the `## Not exposed` section, then **subtract** the exposed op basenames from the Subcommand inventory (this removes the incidental `board-refresh.sh` mention — it names the exposed op that wraps the internal `render-board.sh`). Assert every remaining name appears in the guide.

```bash
# --- Assertion 5: guide's never-allowlist set == scripts/docket.md Not-exposed set ------------
# Exposed op basenames from the Subcommand inventory rows: | `op` | ... |  -> op.sh
EXPOSED="$(grep -oE '^\| `[a-z-]+`' "$FACADE_DOC" | tr -d '|` ' | sed 's/$/.sh/' | sort -u)"
# Backtick *.sh tokens inside the ## Not exposed section (awk reads to EOF; pipefail-safe).
NOTEXP_SECTION="$(awk '/^## Not exposed$/{f=1;next} /^## /{f=0} f' "$FACADE_DOC")"
NOTEXP_RAW="$(grep -oE '`[a-z-]+\.sh`' <<<"$NOTEXP_SECTION" | tr -d '`' | sort -u)"
# Subtract exposed ops (drops board-refresh.sh) -> the true never-allowlist set.
NOTEXP="$(comm -23 <(printf '%s\n' "$NOTEXP_RAW") <(printf '%s\n' "$EXPOSED"))"
if [ -n "$NOTEXP" ]; then ok "not-exposed set derivable from docket.md"; else no "not-exposed set derivable from docket.md"; fi
missing=""
while IFS= read -r s; do
  [ -z "$s" ] && continue
  grep -qF -- "$s" "$GUIDE" || missing="$missing $s"
done <<<"$NOTEXP"
if [ -z "$missing" ]; then ok "guide names every not-exposed script"; else no "guide missing not-exposed scripts:$missing"; fi
```

- [ ] **Step 5: Run the guard — assertions #1–#5 pass**

Run: `bash tests/test_cursor_permissions_docs.sh`
Expected: every `ok -` for assertions 1–5; exit 0. If assertion 3 fails, the fence bytes drifted from the files — fix the guide fence to match the `.json` file exactly. If assertion 5 lists missing scripts, add them to the guide's trust-tier section.

- [ ] **Step 6: Mutation-test assertions #3, #4, #5**

Run each, confirm the named `ok` flips to `NOT OK`, then revert:
1. Change one byte inside the guide's first ```json fence (e.g. delete a comma) → "guide permissions fence == permissions.example.json" reddens. Revert.
2. Delete one `**Observed:**` stamp line from a troubleshooting entry → "troubleshooting entries … all stamped" reddens (count mismatch). Revert.
3. Add a `### ` troubleshooting entry with no stamp → same assertion reddens. Revert.
4. Remove `sync-agents.sh` (and its context) from the guide's trust-tier list → "guide names every not-exposed script" reddens listing `sync-agents.sh`. Revert.
5. Sanity that #5's subtraction works: temporarily confirm `board-refresh.sh` is NOT in `$NOTEXP` (add a debug `echo "$NOTEXP"`); it must be absent (else the guide would be forced to list an exposed op). Remove the debug line.

Record all mutation results before trusting the guards.

- [ ] **Step 7: Commit**

```bash
git add docs/cursor/permissions.md tests/test_cursor_permissions_docs.sh
git commit -m "docs(0073): Cursor permissions guide + fence/stamp/not-exposed guards"
```

---

### Task 3: README pointer + link guard

**Files:**
- Modify: `README.md` (add a pointer to `docs/cursor/permissions.md`)
- Modify: `tests/test_cursor_permissions_docs.sh` (add assertion #6)

**Interfaces:**
- Consumes: `docs/cursor/permissions.md` (Task 2).
- Produces: a discoverable entry point (LEARNINGS 2026-07-09 #49: a knob nobody can find is not shipped).

- [ ] **Step 1: Add the pointer to `README.md`**

Add a short subsection under `## Customization` (near the harness/Cursor discussion) linking the guide. The link target must be exactly `docs/cursor/permissions.md`:

```markdown
### Running under Cursor Auto-run

Cursor users running the skills under Auto-run in Sandbox: see
[docs/cursor/permissions.md](docs/cursor/permissions.md) for the copyable `permissions.json` and
`sandbox.json` fragments, the trust tiers, what one allowlist entry authorizes, and troubleshooting.
```

If `## Customization` has entries in the Table of contents with sub-links, add a matching ToC line; otherwise leave the ToC unchanged (the section-level ToC entry already resolves).

- [ ] **Step 2: Add assertion #6 (README links the guide)**

Append to `tests/test_cursor_permissions_docs.sh`, before `exit $fail`:

```bash
# --- Assertion 6: README links the guide -----------------------------------------------------
if grep -qF -- '](docs/cursor/permissions.md)' "$README"; then ok "README links the guide"; else no "README links the guide"; fi
```

- [ ] **Step 3: Run the full guard file — all six assertions pass**

Run: `bash tests/test_cursor_permissions_docs.sh`
Expected: every `ok -`; exit 0.

- [ ] **Step 4: Mutation-test assertion #6**

Remove the README link line → "README links the guide" reddens. Revert.

- [ ] **Step 5: Commit**

```bash
git add README.md tests/test_cursor_permissions_docs.sh
git commit -m "docs(0073): link the Cursor permissions guide from README + guard"
```

---

### Task 4: Whole-suite regression check

**Files:** none (verification only).

- [ ] **Step 1: Run the entire test suite on the feature branch**

Run every `tests/test_*.sh` (the repo has no top-level runner):

```bash
cd /Users/homer/dev/docket/.worktrees/cursor-sandbox-permissions-guide
rc=0; for t in tests/test_*.sh; do echo "== $t =="; bash "$t" || rc=1; done; echo "SUITE rc=$rc"
```

Expected: no test that passed on `origin/main` newly fails (the enumerated-floor learning: run the WHOLE suite, not just this change's guards — an out-of-goal regression is what the other tests exist to catch). This change only ADDS files under `docs/cursor/` and one test file, and adds one README subsection, so no existing guard should redden.

- [ ] **Step 2: If any test is red, triage against the base**

A red suite in this worktree is a hypothesis, not a verdict (LEARNINGS, environment family). Re-run the identical failing test against unmodified `origin/main` (a throwaway checkout) and byte-compare; if it fails there too, it is environment-bound / pre-existing, not this change's regression — record the differential in the results file. If it passes on base but fails here, it is a real regression — fix it before proceeding.

---

## Self-Review

**Spec coverage** (against `docs/superpowers/specs/2026-07-14-cursor-sandbox-permissions-guide-design.md`):
- `docs/cursor/permissions.md` (the guide) → Task 2. ✓
- `docs/cursor/permissions.example.json` (four spellings) → Task 1. ✓
- `docs/cursor/sandbox.example.json` (read path) → Task 1. ✓
- README pointer → Task 3. ✓
- Guide outline items 1–8 (three gates; why-outside-sandbox; fragments; trust tiers; security consequences; workarounds rejected; scope; troubleshooting) → all present in Task 2's guide draft. ✓
- Verification assertions #1–#6, each mutation-tested → Tasks 1–3. ✓
- ADR (trust at the facade, not per operation) → recorded post-build via `docket-adr` in the implementer's Step 6 (metadata-branch artifact, not a feature-branch task). Noted, not a plan task.
- Build precondition (appendix non-empty) → confirmed at reconcile; not a build task.
- Human-at-merge-gate truth check → stated in the guide's provenance note and the test file's header; flagged for the results file.

**Placeholder scan:** no TBD/TODO; every JSON file and test block is complete literal content; the guide draft is full prose. ✓

**Type/name consistency:** the four spellings, the `**Observed:** Cursor 3.11.19 · 2026-07-14` stamp shape, the `## Troubleshooting`/`### ` structure, and the `docs/cursor/permissions.md` link target are used identically across the guide, the JSON files, and the test assertions. ✓
