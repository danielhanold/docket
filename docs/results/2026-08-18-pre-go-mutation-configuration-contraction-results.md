<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0326 — Pre-Go mutation configuration contraction](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0326-pre-go-mutation-configuration-contraction.md)**
<!-- docket:backlink:end -->

# Results — 0326 Pre-Go mutation configuration contraction

**Change:** 0326 · **Date:** 2026-08-18 · **Branch:** `feat/pre-go-mutation-configuration-contraction`
**Spec:** `docs/superpowers/specs/2026-08-18-pre-go-mutation-configuration-contraction-design.md`
**Plan:** `docs/superpowers/plans/2026-08-18-pre-go-mutation-configuration-contraction.md`

## Implementation route (non-standard)

Implemented and finalized through the immutable **`v0.9.2` Bash workflow** (the sanctioned migration
baseline), from the `/Users/homer/dev/docket-v0.9.2` checkout with its skills preloaded by absolute
path and every `docket.sh` routed through its `scripts/`. The current Go implementer was NOT used; no
Go transaction verb (`change`/`workspace`/`evidence`/`pr`/`run`) touched shared metadata. Only the
read-only `go run ./cmd/docket diagnostic config --for-mutation` was invoked, which the capability
fence permits. Feature branch cut from current `origin/main`.

## What shipped (tracked, in the PR)

1. **Committed `.docket.yml`** — three deferred switches set explicitly `false`, preserving every
   other key/comment and the file's order: `terminal_publish`, `finalize.skip_results_only_delta`,
   `build.checkpoint`. The diff is exactly those three flips.
2. **`internal/app/config_test.go`** — new `TestMigrationHostContraction`: a representative
   four-layer state proving the classifier is layer-aware (a **global** agent pin is supported; the
   **same** agent leaf as a **repository-local** pin is deferred), that the contracted state reports
   `MutationAllowed: true` with zero deferred blockers, and one-at-a-time negatives still fail closed.
3. **Drift-guard re-baseline** — contracting `.docket.yml` trips `internal/config`'s
   `TestFixtureDocketSelf`, a drift guard that byte-compares live `.docket.yml` to a frozen copy. Per
   the guard's OWN remedy (and the frozen-fixture protocol in `testdata/README.md`), a new versioned
   tree `testdata/repositories/v0.9.4/docket-self/` was cut (byte-identical contracted `.docket.yml`;
   the fixture's `xdg/docket/config.yml` carried unchanged), and `TestFixtureDocketSelf` re-pointed +
   re-derived. **Scope note:** this touches `internal/config/fixtures_test.go`, which the spec's
   literal "do not modify internal/config" exclusion names. It was an **explicit maintainer decision
   (2026-08-18)**: re-baselining a drift-guard fixture is the guard's own sanctioned remedy and the
   only way to have both a contracted config and a green suite — it changes no classifier/schema/
   resolver logic and does not weaken the fence (the guard still fires on drift). The re-derived
   `docket-self` result stays `MutationAllowed: false` with a residual `auto_capture.enabled` from the
   fixture's global layer, because 0326 does not touch that frozen layer (see the operator note).
4. Two content-asserting bash tests updated to the contracted reality without weakening
   (`test_docket_example_yml.sh` "keeps its set values" → `terminal_publish: false`;
   `test_finalize_gate.sh` arm-assertion → explicitly disarmed `false`, its paired invisibility-guard
   assertion retained as a standing invariant). `test_skip_allowlist_invisibility.sh` untouched.

## Operator step (machine-local; NOT in the PR — records no private values)

The migration host's gitignored `.docket.local.yml` cannot ride the tracked PR. The operator applies,
on the migration host:
- **remove the repository-local `agents.*` block** — a repository-layer model/effort pin requests the
  per-repository agent routing Go v1 defers (global pins remain the supported override layer); and
- **`auto_capture.enabled: false`** (or remove `auto_capture`) — Go v1 defers repository-local
  automatic capture.

No private configuration values are copied into the repository.

## Verification (AC3) — Go read-only diagnostic

Run against the full post-contraction four-layer state (contracted committed `.docket.yml` +
operator-contracted `.docket.local.yml` + the machine's global config + built-in), using the reviewed
Go binary at commit `63544e16` via `go run ./cmd/docket diagnostic config --for-mutation --json`:

```
mutation_allowed: true
result:           applied
active deferred blockers: []   (none)
```

Negative direction is covered by `TestMigrationHostContraction`'s one-at-a-time fixtures (each
re-activated blocker → `MutationAllowed: false`) and by the classifier's own fence tests, all green.

## Verification (suite)

- Full suite green at branch HEAD (`scripts/run-tests.sh`): `files=122 passed=122 failed=0`.
- Review returned 2 minor findings (comment fidelity; layer-isolation using the same agent leaf),
  both fixed in-branch and mutation-verified; `internal/config/capability.go` confirmed untouched by
  the branch.

## Human verification items (post-merge, on the real migration host)

1. After this PR lands on `main`, apply the `.docket.local.yml` operator step above on the migration
   host, then re-run `docket diagnostic config --repo-dir <checkout> --for-mutation --json` with the
   reviewed installed binary and confirm `mutation_allowed: true` with no active unsupported
   capability from any layer. A malformed layer or remaining active blocker fails the check and does
   NOT authorize Go mutation.
2. Change **0316** (finalize/recovery) may start only after this check passes and `install check`
   (from change 0322's installed binary) accepts the machine.

## Boundaries

Installation/adoption remains 0322 (done); finalize/recovery remains 0316; remaining self-hosting,
Bash removal, docs, release, and hard cutover remain 0318. This change touched no classifier/schema/
resolver logic, added no migration override, and changed no global model/effort pins.
