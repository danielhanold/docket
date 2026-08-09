<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0252 — Harden test fixtures and hermeticity into tests-lib](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0252-harden-test-fixtures-and-hermeticity-into-tests-lib.md)**
<!-- docket:backlink:end -->

# Design: harden test fixtures and hermeticity into tests/lib (change 0252)

Consolidates #0243 (unchecked git fixture setup), #0177 (0174 template-helper robustness
gaps), and #0182 (facade tests reading the developer's real global config) into one
shared home: a sourceable helper library under `tests/lib/` (precedent:
`tests/lib/sync_agents_common.sh`, change 0227), plus adoption at the existing sites.

## Deliverables

### 1. `tests/lib/fixture_lib.sh` — a narrow, sourceable mechanics library

Not a prologue. Unlike `sync_agents_common.sh` (which owns `set -uo pipefail`, `REPO`,
`assert`, `fail`), this file provides ONLY fixture mechanics, because every adopting
file already has its own prologue and assert idiom. It sets no shell options and
defines no `assert`. The `tests/test_*.sh` discovery glob does not match `tests/lib/`,
so it never runs as a test. Contents:

- **`fx()` — the checked fixture-step runner.** `fx cmd args…` runs the command; on
  non-zero exit it prints `FIXTURE FAILURE (<file>): <argv>` to stderr and `exit 1`.
  Fixture setup failure invalidates every later assert in the file, so the posture is
  hard-exit (loud red for the whole file), never `fail=1`-and-continue, and never
  retry — fail-loudly is the house discipline (#0243 boundary). Caveat documented in
  the helper header: `exit` from inside a command substitution or process substitution
  kills only the subshell; builders that print paths (e.g. `new_repo`) must print ONLY
  after all `fx` steps succeed, so a failed subshell yields empty output — and every
  substitution consumer needs its own hard stop on that empty output (see the
  adoption list in Deliverable 3 for which files already carry one).
- **`fx_mktemp_d()` — guarded mktemp.** Echoes a new temp dir; exits loudly if
  `mktemp -d` fails or returns empty (#0177 gap 2). Because callers consume it in a
  command substitution (where the helper's `exit` kills only the subshell), the
  prescribed call form is `VAR="$(fx_mktemp_d)" || exit 1` — the assignment
  propagates the substitution's exit status, so the parent dies too and no
  `cp -R`/`rm -rf` ever runs against an empty root.
- **`fx_defer_rm <path>` — registered cleanup with a single lib-owned EXIT trap.**
  Appends the path to a file-scope array; the first call installs one `trap … EXIT`
  that removes every registered path. The trap's array expansion must use the
  `${arr[@]+"${arr[@]}"}` guard: adopters run `set -u` and the suite's bash floor is
  GNU Bash 4+, where an empty array errors under `-u` before 4.4. *Amendment
  2026-08-09: change #0222 (groomed, ruled 2026-08-07) raises the floor to exactly 4.4
  and deletes this idiom repo-wide. If 0222 lands first, write the plain
  `"${arr[@]}"` expansion here instead; if this change lands first, the guard ships
  as written and 0222's build-time sweep (grep-derived, not hand-listed) picks the
  site up.* Adopting files
  route their existing `trap 'rm -rf …' EXIT` lines through it, because a later
  `trap` for the same signal REPLACES an earlier one — that replacement is exactly
  how `test_closeout.sh` leaks its template root today (its only EXIT trap, line
  ~611, covers `$tmp` only) (#0177 gap 4). The same replacement hazard applies to the
  existing traps at `test_board_checks.sh:97`, `test_docket_config.sh:41`,
  `test_ensure_claude_settings.sh:51`, and `test_docket_example_yml.sh:13` —
  convert those in adopting files too, and fix board_checks' "this file has no
  other EXIT trap" comment, which the conversion makes false. In-passing (builder's
  option, two tokens per file): the file-scope `tmp="$(mktemp -d)"` assignments in
  those same files may adopt the `fx_mktemp_d … || exit 1` form, since an empty
  `$tmp` feeds `mkrepo "$tmp/a"` whose pre-clean would become `rm -rf "/a"`.
- **`fx_pin_hermetic_config <sandbox>` — the hermeticity standard.** Creates
  `<sandbox>/xdg-void` and exports `XDG_CONFIG_HOME` to it. The correct pattern is
  PIN-to-void, never bare `unset`. It covers only the PIN half of the pattern: sites
  whose reachable script needs a seeded global config (as
  `test_docket_example_yml.sh:14-20` seeds a `runtime.bash` fake for
  `ensure-global-config.sh`) keep their own seeding on top of the pin. Second
  in-repo precedent: `test_docket_config.sh:112-113`.

### 2. `tests/test_fixture_lib.sh` — self-test for the library

Guards are code. Minimal suite proving: a failing `fx` step reddens loudly (run a
child bash sourcing the lib with a false step; assert non-zero exit + the FIXTURE
FAILURE marker on stderr); `fx_mktemp_d` output is non-empty; `fx_defer_rm` removes
registered paths on exit AND survives a second registration (no trap clobbering);
`fx_pin_hermetic_config` leaves `XDG_CONFIG_HOME` inside the sandbox. The fx red-path
test is the mutation proof for the whole change class: it interrupts, per the
transient-resource-lifecycle learning, rather than only exercising the happy path.

### 3. Checked fixture setup at the `mkrepo`/`new_repo` sites (#0243)

Builders stay PER-FILE; only the mechanics are shared. The bodies genuinely differ
(closeout's template carries a change file + two ADRs; board_checks' carries
plans/results; example_yml's is a plain clone) — per the
consolidation-flattens-caller-variance learning, consolidating the builder bodies
would either flatten that variance or grow a parameter thicket. The shared primitive
is `fx`, not a shared `mkrepo`.

Adoption: prefix each fixture git/cp/mkdir step with `fx` (or route the existing
`git_quiet` wrappers through it) — EXCEPT expected-failure steps already marked
`|| true` (e.g. `git rm -rf .` on an orphan checkout from an empty origin,
`test_mint_stub.sh:24`, `test_reclaim_claims.sh:34`, and the same step where it
happens to succeed in `test_board_checks.sh:71` / closeout's template build):
`fx`'s failure branch is an in-shell `exit 1`, which a caller's `|| true` cannot
suppress, so wrapping a tolerated-failure step turns the file permanently red.
Never `fx`-wrap a `|| true` step. Files:
- `tests/test_docket_example_yml.sh` (`mkrepo` :24-32 and the fidelity-fixture
  `cp`/`add`/`commit`/`push` at :45-50 — the site that reddened 0190's gate; its
  hand-written non-vacuity guard stays, now backed by loud setup). *Amendment
  2026-08-09 (absorbed #0278): this exact site reddened a second live gate — 0271's
  finalize merge, `SUITE files=100 passed=99 failed=1`, green on human-directed
  re-run — confirming the flake is recurrent, not a one-off; the ruling here
  (hard abort, no retry) stands,*
- `tests/test_docket_config.sh`, `tests/test_ensure_claude_settings.sh` (`mkrepo`),
- `tests/test_closeout.sh`, `tests/test_board_checks.sh` (template-based `new_repo`
  builders),
- `tests/test_reclaim_claims.sh`, `tests/test_mint_stub.sh`,
  `tests/test_sync_integration_branch.sh` (per-call from-scratch `new_repo`s — no
  template, no `NEW_REPO_TEMPLATE`; only the `fx` prefixing applies).

At every consumer of a builder via command/process substitution (`W="$(new_repo)"`,
`read -r W O < <(new_repo)`), `fx`'s `exit` kills only the subshell, so EVERY
substitution consumer needs a one-line vacuity check (`[ -n "$W" ] || exit 1`, or
`|| exit 1` on a plain assignment). The 0174 independence asserts cover only the
file-scope dead-template mode (which `fx` on the top-level template build now
hard-stops anyway); they do NOT cover a per-call `new_repo` failure mid-file — the
~40 `read -r … < <(new_repo)` sites in `test_closeout.sh` and ~30 in
`test_board_checks.sh` are unguarded there too. Apply the check at all substitution
consumers, including those two files; `test_mint_stub.sh`, `test_reclaim_claims.sh`,
and `test_sync_integration_branch.sh` likewise. If the closeout/board_checks
volume argues for a wrapper instead, a checked consumer helper in the lib
(`fx_new_repo` calling the file-local builder and verifying non-empty output) is
acceptable; what is not acceptable is leaving the read sites bare.

### 4. 0174 template-helper hardening (#0177)

- **Sticky global**: build into a local, assign `MKREPO_TEMPLATE`/`NEW_REPO_TEMPLATE`
  only after the last build step succeeds. With `fx` on every step this is
  belt-and-braces (a failed build now kills the file before any consumer runs), but
  the late assignment keeps the invariant local and survives future `fx` removal.
  The eager-at-file-scope build (subshell-consumer rationale) is untouched.
- **Unguarded mktemp**: `NEW_REPO_TEMPLATE="$(fx_mktemp_d)" || exit 1` in both
  definers (the `|| exit 1` is load-bearing — see Deliverable 1's subshell caveat).
- **Destructive pre-clean**: keep `mkrepo`'s `rm -rf "$dir" "$bare"`, add the
  rationale comment in the code (verified safe against all call sites at 0174; any
  future test seeding `$dir` first loses state silently — the comment is the warning).
- **Leaked closeout root**: register the template root and `$tmp` via `fx_defer_rm`;
  delete the now-false "No cleanup trap here" comment block and the line-611 trap.

### 5. Hermeticity sweep (#0182)

- `tests/test_runner_dispatch.sh`: replace the top-of-file `unset XDG_CONFIG_HOME`
  with a file-scope `fx_pin_hermetic_config` into a file-level sandbox dir, so the
  pre-existing facade sections (:131-161, :220-232 — the layer-resolution block) stop
  reading `$HOME/.config/docket/config.yml`. The 0173-era per-invocation
  `DOCKET_HARNESS_ROOT="$SBX"` pins stay (they pin a stronger, per-fixture property).
- Sweep every file that `unset`s `XDG_CONFIG_HOME` (`test_bash_runtime_install.sh`,
  `test_ensure_global_config.sh`, `test_install.sh`, `test_ensure_claude_settings.sh`,
  `test_sync_agents_cursor.sh`, `test_sync_agents_codex.sh`,
  `tests/lib/sync_agents_common.sh`) and every suite invoking config-reading scripts.
  The leak criterion is PER-READER — the global-layer readers do not share one
  fallback shape: `scripts/runner-dispatch.sh:86` resolves
  `${XDG_CONFIG_HOME:-${DOCKET_HARNESS_ROOT:-$HOME}/.config}` (a `DOCKET_HARNESS_ROOT`
  pin compensates); `scripts/docket-config.sh:284` resolves
  `${XDG_CONFIG_HOME:-$HOME/.config}/docket` and IGNORES `DOCKET_HARNESS_ROOT`
  (only an XDG pin compensates); `scripts/ensure-global-config.sh:8` honors
  `HARNESS_ROOT`. Classify each invocation against the reader it actually reaches.
  Convert leaks to the pin-to-void pattern; leave already-hermetic unsets alone but
  add the standard one-line comment naming the compensating pin. Suites that TEST the
  global layer itself (`test_ensure_global_config.sh`) pin into their own sandbox,
  not void.
- Severity scoping (state this in the code comments and results, or the builder will
  misdescribe the fix): under the suite runner, `scripts/run-tests.sh:230-236`
  exports per-job `HOME`/`TMPDIR`/`XDG_CONFIG_HOME` into a job sandbox, so a bare
  unset falls through to a sandboxed `$HOME` and the developer's real config is
  never read at the gate. The #0182 exposure is real only under each file's
  documented direct-invocation mode (`# run: bash tests/…`).
- No static meta-guard over `unset XDG_CONFIG_HOME`: hermeticity is a property of
  each invocation's env, not of file text — a grep guard would false-positive every
  compensated unset and false-negative an inherited XDG. The standard lives in the
  helper + its comment idiom.

### 6. In-passing comment fix

`tests/test_skill_facade_wiring.sh:26` claims "there is NO tests/lib/" — already
false since 0227, and this change widens the gap. Reword to name the sourcing rule
actually in force (lib files are shared helpers, not discovered tests).

## Coordination

Change 0253 hoists `flatten()` into the same `tests/lib/` directory. Keep the files
separate (`fixture_lib.sh` vs 0253's guard-pattern helper) so neither change blocks
the other; whichever lands second rebases trivially (new sibling file). No
`depends_on` either way.

## Out of scope

- Retry semantics for fixture setup (fail-loudly is the discipline).
- The `flatten()` hoist and prose-guard conversions (#0253).
- Any change to what adopted tests assert, or to the 0174 template-copy design.
- Prologue unification (`set` options, `assert`, `REPO`) across test files.

## Assumptions

1. **Helper shape: narrow mechanics library, not a prologue.** Chosen: `fixture_lib.sh`
   defines only fixture functions; adopting files keep their own `set` options and
   `assert`. Rejected: extending `sync_agents_common.sh` (it is a shard-family
   prologue that unsets XDG and would fight adopters' own setups); a full shared
   prologue (touches every file's contract for no stated benefit — out of stub scope).
2. **Fail-loud = hard `exit 1` from the checked runner, no retry, no `fail=1`.**
   Chosen because a broken fixture invalidates all later asserts and #0243 explicitly
   rules retry out. Rejected: `set -e` inside builders (suites run `set +e` by design
   and `-e`'s context-dependence is a known trap); mark-and-continue (green-looking
   noise after a dead fixture).
3. **Builders stay per-file; only mechanics are shared.** Chosen per the
   consolidation-flattens-caller-variance learning — the five builder bodies differ in
   content, not just boilerplate. Rejected: one parameterized shared `mkrepo`
   (parameter thicket, silently rewrites variant callers); leaving everything inline
   with no lib (re-invents the checked runner in 8 files — the #0243 complaint).
4. **Sticky-global fix = assign-after-success AND `fx` on every step.** Rejected:
   clear-on-failure via trap/ERR (fragile under `set +e`); relying on `fx` alone
   (leaves the invariant implicit).
5. **Cleanup = single lib-owned EXIT trap with path registration.** Chosen because
   bash replaces same-signal traps, which is the exact cause of the closeout leak.
   Rejected: trap-chaining by reading `trap -p` (brittle quoting); leaving the one
   leaked root documented-but-leaked (the stub asks for the fix).
6. **Hermeticity standard = pin XDG to a void/sandbox dir, never bare unset.** Chosen
   from the proven `test_docket_example_yml.sh:14-20` pattern (second precedent:
   `test_docket_config.sh:112-113`); covers both read and write hazards
   (config-layer-write-and-read-hazards learning), and works against ALL readers —
   `docket-config.sh:284` ignores `DOCKET_HARNESS_ROOT`, so an XDG pin is the only
   universally honored knob. Sweep classification is per-reader; exposure is scoped
   to direct invocation (run-tests.sh sandboxes the gate) — both stated in
   Deliverable 5. Rejected: pinning `DOCKET_HARNESS_ROOT` everywhere (not honored by
   `docket-config.sh`, and per-invocation pins are easy to forget, which is how
   #0182 happened); a static grep meta-guard (false positives on compensated unsets,
   blind to inherited env).
7. **Self-test file included.** Chosen: house discipline treats guards as code, and
   the unhappy path (the entire point of the change) is otherwise never executed.
   Rejected: trusting adoption-site greens (the happy path cannot exercise `fx`'s
   failure branch).
8. **Dependency state**: #0243/#0177/#0182 are killed-consolidated into this change
   (verified in archive); #0253 is a sibling `related:` change, proposed, not yet
   groomed — coordination is by directory convention only, no ordering constraint.
