<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0254 — BSD tool-default sweep: templated mktemp and non-interactive mv](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-08-0254-bsd-tool-default-sweep-templated-mktemp-and-non-interactive.md)**
<!-- docket:backlink:end -->

# BSD tool-default sweep: templated mktemp and non-interactive mv — design

Change 0254 · 2026-08-07 · consolidates killed #0188 (bare `mktemp -d` ignores TMPDIR on macOS)
and #0189 (bare `mv` self-answers `n` and exits 0 on a tty). Root class: a BSD tool's default
behavior silently defeats a guard. Origin: change 0186 (done, PR #148), which fixed one site of
each shape and deliberately deferred the sweep.

## What gets built

### 1. Template every untemplated `mktemp` (both `-d` and file form)

House form (in-repo precedent `migrate-to-docket.sh:213`):

```sh
mktemp -d "${TMPDIR:-/tmp}/<script-name>.XXXXXX"     # dirs
mktemp "${TMPDIR:-/tmp}/<script-name>.XXXXXX"        # files
```

`<script-name>` is the owning script's basename sans `.sh` (greppable provenance for leaked
debris — the property 0186's sweep-by-signature relied on). `${TMPDIR:-/tmp}` (`:-`, not `-`)
covers both unset and empty.

Inventory verified 2026-08-07 against the working tree (point-in-time; the build re-derives it
by grep, never from this list):

- Bare `mktemp -d` (7): `backfill-change-types.sh:113`, `profile-one-test.sh:73`,
  `profile-asserts.sh:73`, `run-tests.sh:185`, `terminal-publish.sh:112` and `:270`
  (the `"$(mktemp -d)/pub"` nested form), `sync-agents.sh:1364` (repo root).
- Bare file `mktemp` (16): `render-artifact-backlink.sh:90,101`, `render-change-links.sh:157,164`,
  `docket-config.sh:260,274`, `mark-publish-deferred.sh:94`, `mint-stub.sh:139,192` (two on one
  line), `archive-change.sh:70`, `reclaim-claims.sh:48`, `ensure-claude-settings.sh:59`,
  `runners/codex.sh:89`, plus repo-root `sync-agents.sh:1382` and `migrate-to-docket.sh:344`.

### 2. `mv -f` at every bare atomic-replace/rename `mv`

Uniform `mv -f` (0186's form, `backfill-change-types.sh:169` with rationale at `:152`). 16 sites
(re-derived 2026-08-08 at build claim; the stub's "15 in `scripts/`" counted the `git mv` this
section then carves out): 14 in `scripts/*.sh` (`archive-change.sh:71`, `board-refresh.sh:128`,
`docket-status.sh:1042`, `ensure-claude-settings.sh:68`, `ensure-docket-env.sh:92,119`,
`ensure-global-config.sh:169`, `mark-publish-deferred.sh:116,192`, `mint-stub.sh:148,201`,
`reclaim-claims.sh:49`, `render-artifact-backlink.sh:117`, `render-change-links.sh:181`) plus
repo-root `sync-agents.sh:128` and `migrate-to-docket.sh:345`. Carve-out: `archive-change.sh:95`
is `git mv` — different tool, different prompting semantics, untouched, and the guard's pattern
must not match it.

### 3. Shape-keyed repo-wide guard: new `tests/test_bsd_tool_defaults.sh`

Scope (both directions of the guard): `scripts/*.sh`, `scripts/lib/*.sh`, `scripts/runners/*.sh`,
and repo-root `install.sh`, `sync-agents.sh`, `migrate-to-docket.sh`. `tests/` is excluded (the
tests/lib fixture change owns test-side hygiene).

- **mv guard:** negative grep for the bare-install shape, spelled as **two patterns** —
  `^mv "` and `[^-[:alnum:]]mv "` — and run under a **pinned `/usr/bin/grep`**. Probed
  2026-08-07: the single combined ERE `(^|[^-[:alnum:]])mv "` silently matches nothing under
  PATH grep (ugrep 7.5.0) while `/usr/bin/grep` matches correctly, so the combined spelling
  would make the guard vacuous exactly where the suite usually runs; the two split patterns
  agree under both engines, and the pin removes the engine variable entirely. Every match must
  be `mv -f`, with a `git mv`
  allowance keyed on the invocation shape actually in the tree: `archive-change.sh:95` spells it
  `$GIT -C "$WT" mv …`, so the allowance must match `(git|\$GIT)[^|]* mv ` — a literal `git mv`
  allowance would not fire and the guard would redden on the carve-out. Key on shape, never a
  file list (AGENTS.md rule).
- **mktemp guard:** every line containing the call shape `$(mktemp` must contain a **template**
  (an `XXXXXX` argument). One predicate, no option parsing, both `-d` and file form. The
  predicate is deliberately *template-required*, not *TMPDIR-required*: six in-scope sites
  (`ensure-docket-env.sh:85,104,111`, `board-refresh.sh:95`, `docket-status.sh:1032`,
  `ensure-global-config.sh:149`) are already templated **beside their destination** so the final
  `mv` is a same-filesystem atomic rename — a documented guarantee (`board-refresh.md:92`,
  `ensure-global-config.sh`'s own "beside $DEST" die text). A TMPDIR-required predicate would
  redden on them and push a builder into breaking that atomicity. They are correct and stay
  untouched.
- **Population floors** so neither guard can pass vacuously (the 0186 sentinel's own device):
  assert a minimum count of `mv -f ` sites and of templated-`mktemp` sites in scope. Note the
  floors count positives (literals both grep engines handle) and therefore cannot detect a
  vacuous *negative* predicate — the pinned grep binary plus the mutation test are what cover
  that direction. Failure text must state the rule, not "bump the count" (learning:
  guard-remedy-must-not-teach-the-evasion).
- **Brittleness posture:** the 0186 test comment (test_backfill_change_types.sh:241-242) rejected
  a whole-file negative grep *inside a single script's behavioral test*, where a reformat
  produces a false failure with no population floor. This guard differs on both counts: it is
  repo-wide policy keyed on call shape, and the floors keep it from going vacuous. Patterns stay
  POSIX-ERE (no bounded repetition near 255, no PCRE), and the guard invokes the pinned
  `/usr/bin/grep` rather than trusting PATH — engine agreement must never be assumed, only
  probed (learning: shell-portability, its grep-is-ugrep hazard); mutation-test both
  directions (AGENTS.md).
- No `file:line` anchors in test comments (ADR-0054 / comment-anchor guard).

### 4. Behavioral pin: TMPDIR honored where a fixture already depends on it

Extend `tests/test_backfill_change_types.sh`'s rollback fixtures (the ones passing
`TMPDIR="$drb/tmpdir"`, whose committed comment claims cleanup containment): assert post-run
that a `backfill-change-types.*` stage remnant exists **under** the fixture tmpdir. With the fix,
the `uchg`-blocked stage survives the script's own trap *inside* the fixture, where the existing
`chflags -R nouchg "$drb"` cleanup reaches it — so the assert pins TMPDIR-honoring and the leak
goes to zero. Other sites rely on the shape guard only; no per-script behavioral tmp tests.

### 5. AGENTS.md: two bullets under `## Shell`

- Non-interactive flags on tools that can prompt are load-bearing, not style: BSD `mv` on an
  unwritable destination with a tty prompts, self-answers `n` at EOF, and **exits 0**, so `||
  die` guards are unreachable — always `mv -f` on install/replace paths (`git mv` excepted).
- Bare `mktemp` — with or without `-d` — ignores `TMPDIR` on macOS; always pass a template:
  `"${TMPDIR:-/tmp}/<name>.XXXXXX"`, unless the temp file must sit **beside its destination**
  for a same-filesystem atomic rename, in which case template it there.

This is a direct rule addition through the PR gate (like the existing Shell bullets), not a
learnings promotion — no `promotion_state` flips.

### 6. cp/rm audit — done at groom time, re-verify at build

Two different shapes (BSD semantics, per this machine's manpages): `cp` prompts only under
`-i` — audit = grep for `cp -i…`; **`rm` prompts *without* `-i`** when the target is
write-protected and stdin is a terminal (the exact class this change targets), and only `-f`
suppresses it (`-I` also exists on BSD `rm`) — audit = grep for any `rm` invocation **lacking
`-f`**. Verified 2026-08-07 over the full scope: zero `cp -i` sites, zero bare-`rm` sites
(every `rm` is `-f`/`-rf`). Expected chaff when re-running: two `git rm` invocations (`sync-agents.sh:1197`,
inside a constructed remedy string, and `migrate-to-docket.sh:322`) plus comment mentions —
different tool, same carve-out logic as `git mv` in §2; anything beyond that chaff is a
finding. No code change; the build re-runs both greps as specified here and records the
result in the results file.

### 7. Docs

Update a script's sibling `.md` only where it states tmp-dir or install behavior that the sweep
changes (grep during build; expected: few or none).

## Assumptions

- **A1 — mktemp template form: `"${TMPDIR:-/tmp}/<script-name>.XXXXXX"`, 6 X's.** Matches the
  in-repo precedent (`migrate-to-docket.sh:213`) exactly. Rejected: #0188's 8-X suggestion
  (needless divergence from precedent); a shared helper function (23 one-line mechanical edits
  don't justify new lib surface or the churn of threading a helper into repo-root scripts).
- **A2 — sweep covers bare *file* `mktemp` too, not only `-d`.** Probe-verified 2026-08-07 on
  this machine: bare `mktemp` (no `-d`, no template) also lands outside a redirected TMPDIR.
  The stub's literal bullet says `mktemp -d`, but its own "audit analogues while sweeping"
  instruction surfaces the file form as live, and sweeping only `-d` would force either a false
  AGENTS.md rule ("bare `mktemp -d` ignores TMPDIR" — so does the file form) or a rule the repo
  violates in 16 places with a guard that can't cover it. Rejected: `-d`-only sweep (leaves the
  rule/guard/repo mutually inconsistent); deferring the file form to a follow-up stub (auto-groom
  cannot mint stubs, and the edits are the same one-line shape).
- **A3 — mv form: uniform `mv -f`; `git mv` carved out.** 0186's probe-confirmed minimal edit;
  applied uniformly per #0189 ("decide the house form once"). `archive-change.sh:95` stays
  `git mv` (different tool; `-f` there means force-overwrite tracked targets, a semantics
  change). Rejected: `</dev/null` stdin redirection per site (hides the class, 0186 explicitly
  declined it at runner level); `command mv -f` hardening (no aliasing exposure in `sh` scripts).
- **A4 — scope includes repo-root entry scripts (`install.sh`, `sync-agents.sh`,
  `migrate-to-docket.sh`) and `scripts/lib` + `scripts/runners`; `tests/` excluded.** The stub
  itself lists `sync-agents.sh:1364`, and the ensure-*/install path is precisely where a human
  tty exists (#0189's point); its "scripts/-scoped" line excludes *tests*, not repo-root
  helpers. Rejected: `scripts/*.sh`-only (leaves the stub's own named site unswept and the
  install path — the worst exposure — unguarded).
- **A5 — guard = new `tests/test_bsd_tool_defaults.sh`: two negative shape greps + population
  floors, POSIX-ERE, mutation-tested.** The mktemp predicate is *template-required*
  (`XXXXXX` present on every `$(mktemp` line), NOT TMPDIR-required — six documented
  beside-destination atomic-rename sites are templated without TMPDIR and must stay that way
  (§3). The `git mv` allowance keys on the tree's actual `$GIT … mv` spelling (§3). Rejected:
  a TMPDIR-required predicate (reddens on the six correct sites and invites an atomicity-
  breaking "fix"); a literal `git mv` allowance (never fires); extending 0186's site-scoped
  sentinel in test_backfill_change_types.sh (its comment scopes it to one call site by
  design); a ShellCheck-style lint pass (no such infrastructure in-repo).
- **A6 — behavioral coverage only at backfill-change-types (§4).** #0188 required the
  TMPDIR-under assert there ("without that assert the fix is invisible to the suite") because
  fixtures already depend on the redirect; nowhere else does. Rejected: per-script TMPDIR
  behavioral tests (23 sites of pure mechanical shape — the shape guard is the regression
  surface, and the suite already runs near its runtime budget).
- **A7 — AGENTS.md gets the two rules directly via this PR, not via learnings promotion.**
  AGENTS.md's human promotion gate governs learnings-finding graduation; both stubs #0188/#0189
  and the consolidated stub explicitly order the promotion, and the PR merge is the human gate.
  Rejected: routing through a learnings finding + `promotion_state` (this is design output, not
  a harvest).
- **A8 — cp/rm audit closed as "none found" under the corrected shapes (§6), re-verified at
  build.** `cp` prompts only under `-i`; BSD `rm` prompts *without* `-f` on a write-protected
  target with a tty, so the rm audit shape is "any `rm` lacking `-f`" — grepping `rm -i` alone
  verifies nothing. Both audits return zero sites today. Rejected: adding `cp`/`rm` shapes to
  the guard (zero live sites; a bare-`rm` negative guard would false-positive on any future
  legitimate interactive use, and the AGENTS.md rule covers the class prospectively).
- **A9 — coupling with change 0118 (file collision, `related:` not `depends_on:`).** 0118's
  fresh spec (2026-08-07) adds a mark call inside `sweep_execute_one`'s `skipped-publish`
  branch of `scripts/docket-status.sh` (~line 832 region); this change edits the same file at
  the learnings-README install (`:1042`, different function). No semantic interaction; either
  may land first; at rebase, compose (learning: concurrent-edits-compose-at-rebase). Recorded
  as `related: [118]`, mirroring 0118's already-written `related: [254]`.
- **A10 — no sweep of historical `uchg` debris.** Stays out of scope per the stub; the fix
  stops new accumulation, and an age-gated cleanup is separable work a human can stub if the
  ~11k legacy dirs ever matter.

## Out of scope

- `tests/` fixture hygiene (owned elsewhere); historical debris cleanup (A10); any BSD-vs-GNU
  audit beyond the two swept shapes and the closed cp/rm audit; re-litigating 0186's site.
