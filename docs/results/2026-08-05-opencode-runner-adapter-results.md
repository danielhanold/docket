<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0205 — opencode runner adapter — delegate build workers to OpenRouter models](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0205-opencode-runner-adapter.md)**
<!-- docket:backlink:end -->

# opencode runner adapter — results
Change: #0205 · Branch: feat/opencode-runner-adapter · PR: (opened at close-out) · Plan: docs/superpowers/plans/2026-08-05-opencode-runner-adapter.md · ADRs: 0067

## Verify (human)

Every item below is outside what any in-repo test can settle. Docket keeps no vendor allowlist
(ADR-0015) and every mirror assertion compares generated output against the sidecar that generated
it — both sides move together — so no assert here can be an oracle for a value whose truth lives in
opencode or OpenRouter. These are named checks, not gaps that better tests would close.

- [ ] **Model IDs resolve under your credentials.** Run `opencode models openrouter` and confirm
      `openrouter/deepseek/deepseek-v4-flash-0731` and `openrouter/moonshotai/kimi-k3` appear,
      spelled exactly as shipped in the recipe. **Catalog presence is not entitlement** — an ID can
      be listed and still fail against your account. Note the spec proposed
      `openrouter/x-ai/grok-4.5` for the premium/max rungs; the shipped recipe uses the Kimi ID that
      `agents/harness-defaults.yml` already carries, so nothing new was pointed at the outside world.
- [ ] **`--variant` omission yields the provider default**, not an error or a silent substitution.
      Change 0192 explicitly flagged its own equivalent case as unprobed, so this was not assumed.
- [ ] **`--auto` semantics and the deny-list interaction.** Both were read from a single line of
      `opencode run --help`; the deny-list behavior is inferred, not tested. If a workable deny-list
      spelling exists, decide whether the recipe should recommend a starter set — deliberately *not*
      grown into a third `permissions` value in this change.
- [ ] **Relay legibility.** Confirm opencode's default formatted stdout relays usefully through the
      shim. `opencode run` has no `--output-last-message` analogue, so the adapter relays stdout
      verbatim rather than parsing `--format json`'s unversioned event schema (a wrong parse would
      silently truncate). If decoration makes it unreadable, `--format json` plus a documented
      extractor is the recorded escape hatch — see `scripts/runners/opencode.md`.
- [ ] **Auth preflight semantics.** The adapter checks the binary only, not authentication:
      `opencode auth list`'s exit code on a machine with **zero** credentials could not be
      established without destroying real credentials, and a probe with unknown failure semantics
      would turn an unusual-but-working setup into a hard abort. If it reliably exits nonzero,
      adding an auth probe (as `codex.sh` does with `codex login status`) is a cheap follow-up.
- [ ] **One live end-to-end delegated dispatch.** A real `build-economy` task on a real branch,
      confirmed to have executed in opencode rather than Claude Code. Change 0192 certified its
      rungs natively; this is the same bar for the runner path, and nothing here is proven by the
      suite alone. **While doing this, check where the child actually starts** — see the first
      finding below.

## Findings

**ADR-0067 — a `runner:`-bearing agent must carry a user-configured model.** Recorded because this
reverses documented behavior: `scripts/runners/codex.md` said "Omitted ⇒ the child's own default
model," and that is now a generation-time error on every runner. Applied runner-wide rather than
opencode-only so "is a model required?" is not an adapter-by-adapter fact a user learns twice. It is
a breaking change for any existing model-less codex/cursor configuration, **including ones in config
layers docket cannot reach or migrate** — a machine-local `.docket.local.yml` or the global
`~/.config/docket/config.yml`. Affected users get a generation-time error and must add a `model:`.

**Delegated runs are anchored at the main worktree.** `runner-dispatch.sh` exports
`DOCKET_REPO_ROOT` from `docket_main_worktree()`, which is cwd-independent by design (ADR-0034), and
each adapter hands that to the child. Correct for `status`/`adr`, the only agents delegation had
shipped for; wrong for a build worker, whose own contract requires it to stay inside the feature
worktree on its branch. This change ships the first recipe delegating `build-*`, so it is the first
to expose the mismatch. Captured as change #0206 rather than fixed here — it is a framework decision
touching all three adapters and likely ADR-0034's reasoning, not an opencode detail.

**The required-model rule is loud but not fail-before-write.** `emit_wrapper`'s call sites redirect
its stdout into the target path, so the shell truncates the wrapper before the function body runs;
the offending agent is left with a zero-length file and later agents in glob order are not
regenerated. The tests encode `! -s` (absent *or* empty) rather than `! -e` for exactly this reason.
Making it fail-before-write would mean resolving every (harness, agent) pair twice — a larger
refactor than the rule warranted. Captured as change #0207.

**Two plan defects, both caught and corrected during the build rather than followed.** (1) The plan
asserted the registry-parity guard ran one direction only and asked for a new reverse loop;
`tests/test_sync_agents.sh` already carried both directions, so adding the block verbatim would have
duplicated an existing assert under a second label. The genuinely missing property was the
non-vacuity companion, which was added to the existing loop instead. (2) The plan predicted
`test_docket_example_yml.sh`'s nested-key floor moving 18 → 20 (correct, verified from the guard's
own diagnostic) but missed two further guards in the same file that the new keys trip:
`expected_key_count` 38 → 40, and the `consumers=` allowlist, which rejects a file as "not a declared
consumer" regardless of whether it mentions the key.

**Change 0078's status was corrected during reconcile.** Both the change body and the spec described
it as "being deferred as built on outdated logic"; it is in fact `implemented` with PR #89 open. No
scope effect — it was out of scope either way.

## Follow-ups

- **#0206** — delegated runner runs are anchored at the main worktree, not the feature worktree
  (`fix`, auto-captured).
- **#0207** — `sync-agents` aborts mid-loop on a bad runner config, leaving a zero-length wrapper and
  stale siblings (`fix`, auto-captured).
- **Review findings — all closed on this branch.** The first deep review returned 0 blockers, 5
  important, 4 minor; a second deep review after the fix wave returned 2 blockers, 2 important, 5
  minor. Every one was fixed in-branch rather than deferred, at the human's direction. Notably
  `f80e3a5c` closed the two framework-doc findings this section previously listed as open (`codex.md`
  and `cursor.md` documenting "Omitted ⇒ the child's own default"; `agent-layer.md` listing the
  shipped runners as `codex, cursor`). The per-finding record with commit SHAs is the PR body's
  findings table.
- **The `build-*` recipe is a preview, not yet usable** — see #0206 below. All three user-facing
  surfaces now carry that warning.
