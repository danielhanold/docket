# `bootstrap` Facade Verb Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `bootstrap` verb to the `docket.sh` facade so the convention's Step-0 `CREATE_ORPHAN` path routes through the facade like every other operation, retiring the last direct-helper (`docket-config.sh --bootstrap`) invocation from skill prose and flipping the 0072 wiring guard from tolerating that carve-out to forbidding it.

**Architecture:** Pure routing/surface change (ADR-0029 boundary). `docket.sh bootstrap` is a thin `exec "$SCRIPTS_DIR"/docket-config.sh --bootstrap "$@"` arm — a third named verb alongside `env`/`preflight` (op name != helper basename), NOT a `WRAPPED_OPS` member and NOT a composite. `docket-config.sh`'s behavior is untouched. Two guard files move in lockstep: the facade sentinel (`test_docket_facade.sh`) learns the third verb + new case arm; the wiring guard (`test_skill_facade_wiring.sh`) stops stripping the carve-out and asserts zero prefixed `docket-config.sh` invocations in skill prose.

**Tech Stack:** Bash (facade + resolver), markdown (skill prose, contract docs), the repo's self-contained shell test harness (`assert(){ eval "$2"; }`, no `tests/lib/`).

## Global Constraints

- **Out of scope — do NOT change `docket-config.sh` behavior** (the `¬DOCKET ∧ ¬LIVE` cell guard, the orphan create, the push, the `.gitignore` seed). This is routing only. (Spec §Out-of-scope.)
- **`WRAPPED_OPS` is unchanged** — `bootstrap` is a verb, not a wrapped op; ADR-0029's "11 wrapped helper operations" statement stays true. (Spec §1.)
- **`bootstrap` is not a composite** — it does NOT run preflight or print the env block itself beyond what `docket-config.sh --bootstrap` already emits. `preflight` stays the only composite verb. (Spec §1.)
- **ADR-0030's discriminator stays intact** — the wiring guard keys on the *invocation prefix* `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket-config.sh`, NEVER the bare `docket-config.sh --bootstrap` string; prose may still NAME the script/flag descriptively. Do NOT tighten the guard to forbid all `.sh` tokens (ADR-0030's rejected alternative must stay rejected). (Spec §3, §Out-of-scope.)
- **Guards-are-code (LEARNINGS):** every assertion added or changed must be mutation-tested — strip the feature / restore the old prose and watch it redden. An assert whose pattern leads with `--` must use `grep -qF -- "<pat>"` (a leading-`--` pattern is parsed as a grep option → exit 2 → a `!`-negated assert reads a false green). Prove each new assert can FIRE against a tree where the guarded thing IS present.
- **Enumerated floor (LEARNINGS):** the edit-site lists below are a FLOOR. Before finishing each task, grep the whole file for the phrase you are updating (`two verbs`, `env` and `preflight`, `preflight/env`) and update every hit; do not trust the enumerated list to be complete.
- **Mock fidelity (LEARNINGS, spec §5):** the bootstrap functional fixtures must run the REAL `docket-config.sh` (default `SCRIPTS_DIR`, no stub) — a stub that omits it routes every test through a degraded path. Fixtures push to a real bare origin; keep fixture stderr out of stdout captures (`2>/dev/null`).
- **Shell portability (LEARNINGS):** BSD + GNU safe. `pwd -P` both sides before stripping a path prefix; `grep -qF --` for fixed patterns; no leading-`--` bare patterns.
- **`STOP_MIGRATE` exit code is 0, not non-zero (spec §5 correction — VERIFIED empirically 2026-07-14):** `docket-config.sh --bootstrap` in a `STOP_MIGRATE`-shaped repo emits `BOOTSTRAP=STOP_MIGRATE`, performs NO write, and exits **0** (the resolver reports the verdict; fail-closed is `preflight`'s job). The guarded-cell functional test asserts exit 0 + `STOP_MIGRATE` + no write — NOT "exits non-zero." Record this discrepancy in the results file.

## Metadata-branch deliverables (NOT feature-branch — handled by the implementer outside this plan)

These are done by `docket-implement-next` on `metadata_branch` (`docket`), never committed on `feat/bootstrap-facade-verb` (the feature branch never modifies docket metadata / ADRs):

- **ADR-0030 `## Update` note** — append a dated update recording that change 0074 retired the `docket-config.sh --bootstrap` carve-out: the wiring guard now strips only the canonical `docket.sh <op>` form and asserts zero prefixed `docket-config.sh` invocations; the invocation-prefix discriminator is unchanged. Never edit the Decision. `adrs: [29, 30]` already lists it, so terminal-publish re-copies the updated ADR-0030 onto `main` at merge (LEARNINGS #17). ADR-0029 untouched.

---

## Task 1: The `bootstrap` facade verb — routing arm, inventory row, sentinel, functional coverage

**Files:**
- Modify: `scripts/docket.sh` (add the `bootstrap)` case arm, the usage-header line, the `reject()` supported-ops line)
- Modify: `scripts/docket.md` (add the inventory row + update every "two verbs" / "env and preflight" prose site to include `bootstrap`)
- Test: `tests/test_docket_facade.sh` (teach the sentinel the third verb + new case arm; add bootstrap functional coverage)

**Interfaces:**
- Produces: the facade operation `docket.sh bootstrap` → `exec "$SCRIPTS_DIR"/docket-config.sh --bootstrap "$@"`. Task 2's SKILL.md rewire consumes it (and relies on `bootstrap` being a `docket.md` inventory op, which the wiring guard's op-inventory check enforces).

- [ ] **Step 1: Write the failing functional tests (real `docket-config.sh`, real bare origin)**

Append a new section to `tests/test_docket_facade.sh`, immediately BEFORE the `# ===...` "Inventory sentinel" banner (currently ~line 73). These call the facade with the default `SCRIPTS_DIR` so the REAL resolver runs (mock fidelity):

```bash
# --- (F) bootstrap verb: routes to docket-config.sh --bootstrap; the cell guard holds ---------
# CREATE_ORPHAN cell (fresh: no docket branch, no live planning surface on main) → creates the orphan.
fbare="$tmp/f.git"; fwork="$tmp/f"
git init --quiet --bare "$fbare"; git clone --quiet "$fbare" "$fwork" 2>/dev/null
git -C "$fwork" config user.email t@t.test; git -C "$fwork" config user.name Test
git -C "$fwork" checkout --quiet -b main; : > "$fwork/README.md"
git -C "$fwork" add README.md; git -C "$fwork" commit --quiet -m init; git -C "$fwork" push --quiet -u origin main
boot_out="$(cd "$fwork" && XDG_CONFIG_HOME="$tmp/void" bash "$FACADE" bootstrap 2>/dev/null)"; boot_rc=$?
assert "bootstrap exits zero in the CREATE_ORPHAN cell" '[ "$boot_rc" -eq 0 ]'
assert "bootstrap emits BOOTSTRAP=PROCEED after the orphan create" 'printf "%s\n" "$boot_out" | grep -qxF "BOOTSTRAP=PROCEED"'
assert "bootstrap created + pushed the orphan docket branch" \
  'git -C "$fbare" rev-parse --verify --quiet refs/heads/docket >/dev/null'
assert "bootstrap seeded the managed .gitignore block" \
  'grep -qF "# docket:start" "$fwork/.gitignore"'
# a subsequent preflight now verdicts PROCEED (the repo is migrated)
pf_boot="$(cd "$fwork" && XDG_CONFIG_HOME="$tmp/void" bash "$FACADE" preflight 2>/dev/null)"; pf_boot_rc=$?
assert "preflight after bootstrap exits zero" '[ "$pf_boot_rc" -eq 0 ]'
assert "preflight after bootstrap verdicts PROCEED" 'printf "%s\n" "$pf_boot" | grep -qxF "BOOTSTRAP=PROCEED"'

# STOP_MIGRATE cell (live planning surface on main, no docket branch) → cell guard: NO write, exits 0.
sbare="$tmp/s.git"; swork="$tmp/s"
git init --quiet --bare "$sbare"; git clone --quiet "$sbare" "$swork" 2>/dev/null
git -C "$swork" config user.email t@t.test; git -C "$swork" config user.name Test
git -C "$swork" checkout --quiet -b main; : > "$swork/README.md"
mkdir -p "$swork/docs/changes/active"; echo x > "$swork/docs/changes/active/0001-x.md"
git -C "$swork" add README.md docs/changes/active/0001-x.md
git -C "$swork" commit --quiet -m init; git -C "$swork" push --quiet -u origin main
smig_out="$(cd "$swork" && XDG_CONFIG_HOME="$tmp/void" bash "$FACADE" bootstrap 2>/dev/null)"; smig_rc=$?
# NOTE (spec §5 correction, verified 2026-07-14): the resolver reports the verdict and exits 0;
# fail-closed is preflight's job. The cell guard is about the WRITE, not the exit code.
assert "bootstrap in STOP_MIGRATE cell exits zero (resolver reports the verdict)" '[ "$smig_rc" -eq 0 ]'
assert "bootstrap in STOP_MIGRATE cell emits BOOTSTRAP=STOP_MIGRATE" \
  'printf "%s\n" "$smig_out" | grep -qxF "BOOTSTRAP=STOP_MIGRATE"'
assert "bootstrap in STOP_MIGRATE cell writes NO docket branch" \
  '! git -C "$sbare" rev-parse --verify --quiet refs/heads/docket >/dev/null'
assert "bootstrap in STOP_MIGRATE cell writes NO .gitignore" '[ ! -f "$swork/.gitignore" ]'
```

- [ ] **Step 2: Run to verify the new asserts fail (bootstrap not yet a verb)**

Run: `bash tests/test_docket_facade.sh`
Expected: the eight new `(F)` asserts FAIL (`docket.sh bootstrap` currently exits 2 "unknown operation", so exit-0 / orphan-branch / PROCEED / STOP_MIGRATE asserts are NOT OK). Every pre-existing assert still passes.

- [ ] **Step 3: Add the `bootstrap` routing arm + usage line + reject line to `scripts/docket.sh`**

In the usage-comment block, add the `bootstrap` line immediately after the `preflight` line (it is part of the Step-0 story):

```
#   preflight                 Step-0 side effects (sync the metadata worktree), then print env
#   bootstrap                 guarded CREATE_ORPHAN orphan-`docket` create (fresh repo, once, human-attended)
#   env                       print resolved KEY=value config (read-only)
```

In the dispatch `case`, add the arm immediately after the `preflight)` arm:

```bash
  preflight)
    docket_preflight "$SELF_DIR" || exit 1
    exec "$SCRIPTS_DIR"/docket-config.sh --export --format plain ;;
  bootstrap)
    exec "$SCRIPTS_DIR"/docket-config.sh --bootstrap "$@" ;;
```

Update `reject()` so the supported-operations line names the third verb:

```bash
reject(){ printf 'docket: unknown operation: %s\n' "${1:-<none>}" >&2; printf 'supported operations: preflight env bootstrap %s\n' "$WRAPPED_OPS" >&2; exit 2; }
```

- [ ] **Step 4: Teach the facade sentinel the third verb + new case arm (`tests/test_docket_facade.sh`)**

The parity check and the case-label sentinel must both learn `bootstrap`, or they redden. Change the `sh_ops` derivation to include the third verb:

```bash
sh_ops="$(printf 'preflight\nenv\nbootstrap\n%s\n' "$sh_wrapped" | tr ' ' '\n' | sed '/^$/d' | sort -u)"
```

Add `bootstrap` to the expected dispatch-case labels:

```bash
expected_labels="$(printf '%s\n' '-h|--help' '""' 'env' 'preflight' 'bootstrap' '*' | sort -u)"
```

- [ ] **Step 5: Add the `bootstrap` inventory row + prose updates to `scripts/docket.md`**

Add the row to the Subcommand inventory table, immediately after the `env` row:

```
| `bootstrap` | `docket-config.sh --bootstrap` | guarded `CREATE_ORPHAN` orphan-`docket` create (fresh repo, once, human-attended); the facade's sole write-path verb, reached only via this verb |
```

Then update every "two verbs" / "reached through `env` and `preflight`" prose site to include `bootstrap` (this list is a FLOOR — grep the file for `two verbs`, ``env`` `` and ``preflight``, `preflight/env`, `preflight` `and` `env` and update every hit):

1. Purpose para (`(except for the two verbs \`preflight\` and \`env\`)`):
   → `(except for the three verbs \`preflight\`, \`env\`, and \`bootstrap\`)`
2. Under the table (`for every row except the two verbs \`preflight\` and \`env\`, whose \`Wraps\` column names an implementation rather than a same-named script (there is no \`scripts/preflight.sh\` or \`scripts/env.sh\`).`):
   → `for every row except the three verbs \`preflight\`, \`env\`, and \`bootstrap\`, whose \`Wraps\` column names an implementation or a flagged resolver invocation rather than a same-named script (there is no \`scripts/preflight.sh\`, \`scripts/env.sh\`, or \`scripts/bootstrap.sh\`).`
3. Behavior — dispatch para (`A match on \`preflight\` or \`env\` runs the verb-specific logic below instead of execing a same-named script.` and the `supported operations: preflight env <op>...` line):
   → `A match on \`preflight\`, \`env\`, or \`bootstrap\` runs the verb-specific logic below instead of execing a same-named script.` and `supported operations: preflight env bootstrap <op>...`
   (Leave "A match on one of the 11 wrapped ops execs …" — `bootstrap` is not a wrapped op, so 11 is still correct.)
4. Behavior — add a one-line description of the verb after the escape-hatch paragraph (or beside `env`/`preflight`):
   `**\`bootstrap\`.** \`docket.sh bootstrap\` execs \`docket-config.sh --bootstrap "$@"\` — the guarded \`CREATE_ORPHAN\` orphan-\`docket\` create (fresh repo, once, human-attended). Args are forwarded verbatim (so \`--repo-dir\` stays usable in fixtures); it is pure routing, not a composite (it does not sync the worktree or re-run \`preflight\`). Outside the \`CREATE_ORPHAN\` cell it performs no write and exits with the resolver's own status (\`env\`-like), because failing closed on a non-\`PROCEED\` verdict is \`preflight\`'s job, not this verb's.`
5. "Not exposed" — the `docket-config.sh` bullet (`reached only indirectly, through the \`env\` and \`preflight\` verbs, which call it with a fixed, non-caller-controlled flag set.`):
   → `reached only indirectly, through the \`env\`, \`preflight\`, and \`bootstrap\` verbs, each of which prepends a fixed resolver flag (\`--export\` / \`--bootstrap\`); \`docket-config\` is never itself a routable op.`
6. Exit codes table — the "other" row (`or from \`docket_preflight\`/\`docket-config.sh\` failure (for \`preflight\`/\`env\`).`):
   → `… (for \`preflight\`/\`env\`/\`bootstrap\`).`
7. Invariants — sentinel bullet (`derives the \`docket.sh\`-side op set by grepping the \`WRAPPED_OPS=\` line plus the two verbs`):
   → `… plus the three verbs`
8. Invariants — "Operation name = helper basename" bullet (`except the \`preflight\`/\`env\` verbs (documented above …)`):
   → `except the \`preflight\`/\`env\`/\`bootstrap\` verbs (documented above …)`

Note: the "Not exposed" invariant list of scripts that may never gain a row (`docket-config`, …) stays as-is — `docket-config` is still not a routable op; the `bootstrap` verb reaches it internally exactly as `env`/`preflight` do.

- [ ] **Step 6: Run the facade tests — expect all green**

Run: `bash tests/test_docket_facade.sh`
Expected: PASS — the eight `(F)` asserts now pass; the parity sentinel (`docket.sh op set == docket.md documented op set`) passes with `bootstrap` on both sides; the dispatch-case sentinel passes with `bootstrap)` in `expected_labels`; every not-exposed / escape-hatch assert still passes (`bootstrap` is a named finite op, not an escape hatch; `docket-config` is still not an op).

- [ ] **Step 7: Mutation-test the sentinel (guards-are-code, spec §4) — prove it bites both surfaces**

Prove parity bites the doc side: delete the `| \`bootstrap\` |` row from `scripts/docket.md`, run `bash tests/test_docket_facade.sh` → the parity assert goes NOT OK (`sh=[…bootstrap…] md=[… no bootstrap …]`). Restore the row.

Prove the case-label sentinel bites a rogue arm: temporarily insert a bogus arm inside the `case` (e.g. `  rogue-op) exit 0 ;;`), run → the "dispatch case has ONLY the known arms" assert goes NOT OK. Remove the bogus arm.

Re-run `bash tests/test_docket_facade.sh` → all green. (Do NOT commit either mutation.)

- [ ] **Step 8: Commit**

```bash
git add scripts/docket.sh scripts/docket.md tests/test_docket_facade.sh
git commit -m "feat(0074): add the bootstrap facade verb + inventory + sentinel"
```

---

## Task 2: Step-0 rewire + wiring-guard flip (carve-out retired)

**Files:**
- Modify: `tests/test_skill_facade_wiring.sh` (delete the bootstrap strip clause; replace the `carve == 1` assertion with a prefixed `== 0` assertion; refresh the header + `strip_canonical` comments)
- Modify: `skills/docket-convention/SKILL.md` (rewire the Step-0 `CREATE_ORPHAN` clause to `docket.sh bootstrap`; delete the trailing justification sentence)

**Interfaces:**
- Consumes: `docket.sh bootstrap` and its `scripts/docket.md` inventory row from Task 1 (the wiring guard's op-inventory check requires `bootstrap` to be an inventory op; without Task 1 the rewired `docket.sh bootstrap` in SKILL.md would redden the "names an inventory op" assert).

> **TDD shape here is guard-first:** editing the guard against the *un-rewired* prose is the RED; rewiring the prose is the GREEN. This mirrors how the 0072 guard was itself built (its header comment calls the pre-rewiring RED "the reverse-mutation proof that each guard bites").

- [ ] **Step 1: Flip the wiring guard (`tests/test_skill_facade_wiring.sh`)**

Delete the bootstrap strip clause from `strip_canonical` so only the canonical facade form is stripped:

```bash
# Strip the single byte-exact canonical facade form so only the haystack remains.
strip_canonical() {
  sed -E \
    -e 's#"\$\{DOCKET_SCRIPTS_DIR:\?run docket/install\.sh\}"/docket\.sh##g'
}
```

Replace the `carve == 1` block (the `carve="$(grep -hoF 'docket-config.sh --bootstrap' …)"` line and its `assert "CREATE_ORPHAN carve-out … occurs exactly once …"`) with an explicit `== 0` assertion keyed on the *prefixed invocation form* (NOT the bare string — ADR-0030):

```bash
# The CREATE_ORPHAN carve-out is RETIRED (change 0074): skill prose must reach docket-config.sh
# ZERO times as an invocation. Key on the prefixed invocation form, never the bare
# `docket-config.sh --bootstrap` string — prose may still NAME the script/flag descriptively
# (ADR-0030), and only the `${DOCKET_SCRIPTS_DIR`-prefixed spelling is an actual invocation.
# `grep -F --` is mandatory: the pattern is a fixed string that must never be read as an option.
CFG_INVOKE='"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket-config.sh'
cfg_invocations="$(grep -hoF -- "$CFG_INVOKE" "${SCOPE[@]}" | wc -l | tr -d ' ')"
assert "no prefixed docket-config.sh invocation survives in skill prose (carve-out retired)" \
  '[ "$cfg_invocations" = "0" ]'
```

Refresh the two now-stale comments:
- Header block (currently "the Step-0 preamble + mid-run re-sync + CREATE_ORPHAN carve-out are PRESENT"):
  → "the Step-0 preamble + mid-run re-sync are PRESENT, and the CREATE_ORPHAN path routes through the `docket.sh bootstrap` facade verb — the direct-helper carve-out is RETIRED (change 0074), so a prefixed `docket-config.sh` invocation reappearing in skill prose reddens via BOTH the Layer-1 sweep and the `cfg_invocations == 0` assert (two independent scans)."
- `strip_canonical` comment (currently describes stripping two byte-exact forms, bootstrap first):
  → "Strip the single byte-exact canonical facade form (`…/docket.sh`) so only the haystack remains. After 0074 there is no second form: a prefixed `…/docket-config.sh --bootstrap` is no longer stripped, so it survives into `$hay` and trips the Layer-1 `$P_PREFIX` sweep."

- [ ] **Step 2: Run the wiring guard — expect it RED against the un-rewired prose (reverse-mutation proof)**

Run: `bash tests/test_skill_facade_wiring.sh`
Expected: NOT OK on TWO asserts, both pointing at `skills/docket-convention/SKILL.md` — (a) the Layer-1 `no non-facade / non-byte-exact helper invocation prefix survives in skills/docket-convention/SKILL.md` (the carve-out's `${DOCKET_SCRIPTS_DIR` prefix is no longer stripped) and (b) `no prefixed docket-config.sh invocation survives in skill prose (carve-out retired)` (`cfg_invocations == 1`). This RED is the proof both guards bite the pre-rewiring prose.

- [ ] **Step 3: Rewire the Step-0 `CREATE_ORPHAN` clause (`skills/docket-convention/SKILL.md`)**

In the Step-0 preamble's verdict sentence (item 3), replace the carve-out invocation with the facade verb and DELETE the trailing justification sentence. Exact edit — from:

```
`CREATE_ORPHAN` (fresh repo, once, human-attended) → run the one sanctioned direct-helper spelling `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket-config.sh --bootstrap`, then re-run `docket.sh preflight`. This carve-out exists only here, because the facade deliberately does not expose `docket-config.sh` and `preflight` fails closed.
```

to:

```
`CREATE_ORPHAN` (fresh repo, once, human-attended) → run `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh bootstrap`, then re-run `docket.sh preflight`.
```

- [ ] **Step 4: Run the wiring guard — expect all green**

Run: `bash tests/test_skill_facade_wiring.sh`
Expected: PASS. The rewired line contains `…/docket.sh bootstrap` (the prefix is stripped by `strip_canonical`, leaving ` bootstrap` — no `${DOCKET_SCRIPTS_DIR` survives) and `docket.sh preflight`; `bootstrap` is an inventory op (Task 1), so the op-inventory check passes; `cfg_invocations == 0`. Layers 2 and 3 are unaffected.

- [ ] **Step 5: Mutation-test the retire+replace in BOTH directions (guards-are-code; spec §3)**

Prove both new/kept scans bite the forbidden prose: temporarily restore the old carve-out spelling in `skills/docket-convention/SKILL.md` (put `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket-config.sh --bootstrap` back), run `bash tests/test_skill_facade_wiring.sh` → BOTH the Layer-1 prefix sweep AND `cfg_invocations == 0` go NOT OK (two independent scans). Revert to the rewired form; re-run → green.

Prove non-vacuity of the `== 0` assert directly: confirm `grep -hoF -- "$CFG_INVOKE" skills/docket-convention/SKILL.md` prints nothing NOW (0), and printed exactly the carve-out line during the mutation above (it CAN fire).

- [ ] **Step 6: Commit**

```bash
git add tests/test_skill_facade_wiring.sh skills/docket-convention/SKILL.md
git commit -m "feat(0074): retire the docket-config.sh --bootstrap carve-out; route Step-0 through docket.sh bootstrap"
```

---

## Task 3: 0073 framing check + results file

**Files:**
- Create: `docs/results/2026-07-14-bootstrap-facade-verb-results.md`

**Interfaces:**
- Consumes: the completed Tasks 1–2 (the verdict is computed against the rewired skill surface).

- [ ] **Step 1: Grep the whole skill surface for any remaining non-facade invocation shape (spec §7)**

Run (from the feature worktree root):

```bash
grep -rEn '\$\{DOCKET_SCRIPTS_DIR[^}]*\}"/[a-z-]+\.sh' skills/ | grep -vF '/docket.sh'
```

Expected: NO invocation hits — the only `${DOCKET_SCRIPTS_DIR`-prefixed invocation left in skill prose is `…/docket.sh <op>`. (Descriptive NOUN mentions of other scripts, which carry no prefix, are permitted and are not invocations.) If any hit appears, it is a residual second command shape and MUST be resolved or explicitly recorded as what still prevents the "two command shapes" claim.

- [ ] **Step 2: Author the results file**

Create `docs/results/2026-07-14-bootstrap-facade-verb-results.md` from `skills/docket-implement-next/results-template.md`. It MUST record:
- **0073 framing verdict (§7):** state whether docket's agent-facing runtime surface is now exactly two command shapes (`docket.sh <op>` and nothing else reached via `DOCKET_SCRIPTS_DIR` from skill prose), citing the Step-1 grep result. Note this verdict is input to change 0073's own groom; change 0074 does NOT edit 0073's stub.
- **Spec §5 exit-code discrepancy (LEARNINGS #21):** the spec said the guarded cell "exits non-zero"; verified behavior is exit 0 emitting `BOOTSTRAP=STOP_MIGRATE` with no write (the resolver reports the verdict; fail-closed is preflight's job). The functional test asserts the true behavior; `docket-config.sh` behavior was NOT changed (out of scope).
- **Guard-file drift folded at reconcile:** change 0071 (PR #81) reshaped `test_skill_facade_wiring.sh` (added the Layer-3 board-surfaces sentinel) after the spec was groomed; the 0074 edits are keyed by shape and are independent of Layer 3.
- **Manual/interactive checks for the merge gate** (if any) and any notable plan deviations.

- [ ] **Step 3: Commit**

```bash
git add docs/results/2026-07-14-bootstrap-facade-verb-results.md
git commit -m "docs(0074): results — 0073 framing verdict + STOP_MIGRATE exit-code note"
```

---

## Final verification (before PR)

- [ ] Run the FULL test suite (not just the two touched files — LEARNINGS: an out-of-goal regression is exactly what the tests outside the goal set catch), from the feature worktree root. Discover the runner (`ls tests/`); run every `tests/test_*.sh`. All green.
- [ ] `git log --oneline origin/main..HEAD` shows only the three task commits (plan/results are the only docs; no docket metadata touched on the feature branch).

## Self-review notes (author)

- **Spec coverage:** §1 verb → Task 1 (arm + usage + reject); §1 docket.md row → Task 1; §2 Step-0 rewire → Task 2; §3 guard flips (delete strip, replace carve with prefixed `==0`, mutation tests) → Task 2; §4 inventory sentinel (`sh_ops` third verb, `expected_labels` arm, parity + case mutation) → Task 1; §5 functional coverage (CREATE_ORPHAN + guarded-cell, real resolver) → Task 1 (with the exit-code correction); §6 ADR hygiene → metadata-branch deliverable (out of feature-branch scope); §7 0073 framing → Task 3.
- **No placeholders:** every code step shows the exact bytes.
- **Type/name consistency:** `bootstrap` verb name, `CFG_INVOKE` prefix string, `cfg_invocations` counter, `expected_labels` — consistent across tasks.
