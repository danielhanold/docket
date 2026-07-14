# Design: A `bootstrap` facade verb — retire the last direct-helper carve-out (change 0074)

Date: 2026-07-14
Change: 0074-bootstrap-facade-verb
Status: approved (groomed interactively; both stub open questions settled)

## Context

Change 0068 built `scripts/docket.sh` — a finite-operation facade with no escape hatch — and
change 0072 rewired the seven operating skills and the convention's Step-0 preamble onto it. Both
are done. Exactly one direct-helper invocation survives in skill prose: the `CREATE_ORPHAN` path in
the convention's Step-0 still instructs `"${DOCKET_SCRIPTS_DIR:?…}"/docket-config.sh --bootstrap`
directly, because the facade has no verb for it. The 0072 wiring guard
(`tests/test_skill_facade_wiring.sh`) tolerates it via a byte-exact strip (line ~78) plus a
"carve-out occurs exactly once" assertion (line ~110). This change routes bootstrap through the
facade, rewires Step-0, and flips the guard from tolerating the carve-out to forbidding it.

This is a routing/surface change only. What bootstrap *does* — the `¬DOCKET ∧ ¬LIVE` cell guard,
the orphan-`docket` create + push, the `.gitignore` block seed — is untouched.

## Settled open questions

1. **Write-path asymmetry: the cell guard suffices.** No confirmation prompt, no `--yes` flag, no
   distinct exit code. `docket-config.sh` already writes only in the `CREATE_ORPHAN` cell and only
   under `--bootstrap`; `preflight` fails closed before the verb is ever reached; the path is
   human-attended by definition. An interactive confirmation would also break the abort-and-report
   rule for agents. The `scripts/docket.md` table row describes the verb honestly (guarded
   fresh-repo orphan create — a write), which is inventory completeness, not a formal write-path
   marking scheme.
2. **Dispatch placement: a dedicated arm, not `WRAPPED_OPS`.** The `WRAPPED_OPS` loop's contract is
   *op name == helper basename* with args forwarded to `<op>.sh`; `bootstrap` is a flagged
   invocation of the resolver (`docket-config.sh --bootstrap`), so it structurally cannot ride the
   loop. It joins `env`/`preflight` as a named arm. No new inventory sentinel mechanism is needed —
   the existing dispatch-case sentinel's known-arms set grows by one (see §4).

## 1. The verb (`scripts/docket.sh`)

A thin routing arm, placed after `preflight` in the `case` (and after `preflight` in the usage
header — it is part of the Step-0 story):

```bash
bootstrap)
  exec "$SCRIPTS_DIR"/docket-config.sh --bootstrap "$@" ;;
```

- Pure routing boundary per ADR-0029: args forwarded verbatim (keeps `--repo-dir` usable in test
  fixtures), exit code + stderr unmasked, `exec` like every other arm.
- NOT a composite: it does not run preflight or print the env block afterwards. `preflight` stays
  the facade's only composite verb (ADR-0029 decision 1). The rejected alternative — a
  bootstrap+preflight+env composite collapsing Step-0 to one call on a fresh repo — was declined:
  it adds a second composite verb and makes "bootstrap" also mean "sync", for a path that runs
  once per repo.
- `WRAPPED_OPS` is unchanged; ADR-0029's "11 wrapped helper operations" statement stays true.

`scripts/docket.md` (the permission inventory): add the `bootstrap` row to the subcommand table,
after `preflight`, described as the guarded `CREATE_ORPHAN` orphan-branch create (fresh repo, once,
human-attended; the facade's sole write-path verb). The docket.sh usage header gains the matching
line.

## 2. Step-0 rewire (`skills/docket-convention/SKILL.md`)

In the Step-0 preamble's verdict sentence, the `CREATE_ORPHAN` clause becomes: run
`"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh bootstrap`, then re-run
`docket.sh preflight`. Delete the trailing justification sentence ("This carve-out exists only
here, because the facade deliberately does not expose `docket-config.sh` and `preflight` fails
closed.") — there is no carve-out left to justify. Sweep the rest of skill prose for any other
`docket-config.sh --bootstrap` invocation-position mention (build-time grep of the whole repo, per
the enumerated-set learning — the spec's site list is a floor).

## 3. Guard flips (`tests/test_skill_facade_wiring.sh`)

- **Delete the byte-exact bootstrap strip** (the
  `s#"…"/docket-config\.sh --bootstrap##g` sed clause). After deletion, any prefixed
  `docket-config.sh` invocation in skill prose reddens via the existing Layer-1
  `${DOCKET_SCRIPTS_DIR`-prefix sweep with no new machinery.
- **Replace the `carve == 1` assertion with an explicit `== 0` assertion** on the *prefixed
  invocation form* — count occurrences of the literal
  `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket-config.sh` across the skill scope and
  assert zero. Key on the invocation prefix, NOT the bare `docket-config.sh --bootstrap` string:
  ADR-0030's descriptive-noun permission must stay intact (prose may still NAME the script/flag),
  and ADR-0030's rejected broad reading must stay rejected.
- Mutation tests (guards-are-code): (a) restore the old Step-0 carve-out sentence → both the
  Layer-1 sweep and the new `== 0` assertion must go red (two independent scans, per the
  independent-scan learning); (b) confirm the new assertion's anchor occurs where expected —
  re-derive counts, don't trust green.

## 4. Inventory sentinel (`tests/test_docket_facade.sh`)

- Add `bootstrap` to the dispatch-case sentinel's known-arms set (alongside `env`/`preflight`) and
  to the docket.sh ↔ docket.md parity check.
- Mutation-test both directions: drop the docket.md `bootstrap` row → parity goes red; hand-add a
  rogue `case` arm → the consuming-surface scan still goes red (the #68 "wrong surface" learning —
  the sentinel asserts the dispatch `case`, not just the array).
- Escape-hatch scans (no `run`/`exec`/`shell`/`eval` op; docket.sh never calls `eval`) must stay
  green unmodified — `bootstrap` is a named, finite op, not an escape hatch.

## 5. Functional coverage

- Fresh-repo fixture (the `¬DOCKET ∧ ¬LIVE` cell): `docket.sh bootstrap` exits 0, creates and
  pushes the orphan `docket` branch, seeds the `.gitignore` block; a subsequent `docket.sh
  preflight` verdicts `PROCEED`. Fixture must carry the real `docket-config.sh` (mock-fidelity
  learning: a mock that omits the tool routes every test through the degraded branch).
- Guarded-cell fixture: in a `STOP_MIGRATE`-shaped repo, `docket.sh bootstrap` exits non-zero and
  writes nothing (the cell guard, exercised through the facade).
- Unknown-op rejection behavior unchanged.

## 6. ADR hygiene

- **ADR-0030**: append a dated `## Update` note — the `docket-config.sh --bootstrap` carve-out is
  retired by change 0074; the guard now strips only the canonical `docket.sh <op>` facade form and
  asserts zero prefixed `docket-config.sh` invocations. The decision's discriminator (invocation
  prefix, not bare `.sh` tokens) is unchanged.
- **ADR-0029**: untouched — "11 wrapped helper operations", the routing-boundary decision, and the
  no-escape-hatch consequences all remain true.
- Both ids are already in the change's `adrs: [29, 30]`, so terminal-publish re-copies the updated
  ADR-0030 onto the integration branch at merge (the #17 atomic-ADR-update learning). No new ADR:
  this is a routing/surface change; the settled open questions above are recorded here and in
  ADR-0030's update note.

## 7. The 0073 framing check

At build time, grep the whole skill surface for any remaining non-facade invocation shape and
record the verdict in the results file: either "the runtime surface is now exactly two command
shapes (`docket.sh <op>` and nothing else via `DOCKET_SCRIPTS_DIR`)" or what still prevents that
claim. Change 0074 does NOT edit change 0073's stub — the recorded verdict is input to 0073's own
groom.

## Out of scope

- Any change to bootstrap's behavior (cell guard, orphan create, push, `.gitignore` seed).
- Broadening the facade beyond this one verb; revisiting ADR-0029's finite-subcommand posture.
- Tightening the wiring guard to forbid all `.sh` tokens (ADR-0030's rejected alternative stays
  rejected).
- Editing change 0073's stub.
