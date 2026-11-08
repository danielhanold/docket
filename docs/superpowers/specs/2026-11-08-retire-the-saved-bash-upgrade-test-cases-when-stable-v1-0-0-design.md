<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0514 — Retire the saved Bash upgrade test cases when stable v1.0.0 ships](../../changes/archive/2026-11-08-0514-retire-the-saved-bash-upgrade-test-cases-when-stable-v1-0-0.md)**
<!-- docket:backlink:end -->

# Retire the saved Bash upgrade test cases — design

## Intent

`docs/release/upgrading-from-bash.md` moves a Bash docket `v0.9.2`/`v0.9.3` install to the docket
binary. During the v1.0.0 pre-releases, a test kept the guide accurate as the code changed: it
restored saved copies of real Bash installs from `testdata/bash-upgrade/` and ran the guide's
marked steps against the binary. Stable v1.0.0 has shipped, so that test has done its job. Users
never ran it, and it costs suite time on every build.

This change deletes the saved cases and everything that exists only for them. The guide stays,
edited so it describes only the upgrade, with no mention of a test that will no longer exist. The
human's rule: documentation must not keep referring to code that is gone.

## Goals

1. Remove the Bash upgrade proof and every artifact that exists only to support it.
2. Keep the one test in that package that is not about upgrades: the `dckt` alias check.
3. Leave the guide accurate after the removal: same steps, tables and commands, with every
   sentence about "the test" gone or rewritten as the guide's own scope.

## Non-goals

- Changing any upgrade step, command, findings table, or the guide's structure.
- Changing the installer's v0.9.2 takeover behavior.
- Editing point-in-time records that mention the test: release `decisions.md` and
  `candidate/run.txt` files, archived specs, plans and results, and the frozen export corpus under
  `internal/export/testdata/corpus/`. They stay as written.

## Anti-goals

- Losing `TestAliasInvocationMatchesDocket` along with the package it happens to live in.
- Leaving a "last verified against v1.0.0" line or any other note pointing at the retired test.
  The human explicitly rejected one.

## What gets deleted

- `testdata/bash-upgrade/` — the whole tree (`PROVENANCE.md`, `v0.9.2/`, `v0.9.3/`).
- `internal/bashupgrade/` — the whole package (`doc.go` and every `*_test.go`), after the alias
  test is moved out (below).
- `tests/test_go_integration_bashupgrade.sh` — the integration shard (prefix
  `TestIntegrationBashUpgrade`).
- The `tests/test_go_integration_bashupgrade.sh` row in `tests/runtime-budgets.tsv`.
- The `## Root \`testdata/bash-upgrade/<tag>/\`` section of `testdata/README.md`.

The builder re-runs a whole-repo grep for `bashupgrade`, `bash-upgrade`, `BashUpgrade` and
`upgrade-step` at build time and sorts each hit into maintained source (fix) or point-in-time
record (leave). At grooming time the only maintained hits were the items above and the guide.

## What moves

`TestAliasInvocationMatchesDocket` (currently `internal/bashupgrade/alias_invocation_test.go`)
checks that the binary invoked through a `dckt` symlink (`install.AliasName`) gives the same
`version --json` output as when invoked as `docket`. It sat in `internal/bashupgrade` only to reuse
that package's one-time binary build. Move it into the `cmd/docket` package, whose `TestMain`
(in `main_test.go`) already builds the binary once into `binPath`, so the move adds no build. The assertion stays the
same; only the binary source changes from `buildDocket(t)` to `binPath`.

## The guide edit (`docs/release/upgrading-from-bash.md`)

Each passage about the test, by the sentence it opens with today:

| Passage | Change |
|---|---|
| Intro: "An automated test runs every marked step below against saved copies…" (whole paragraph) | Delete. |
| §2: "The test covers repositories with no change in progress." | Becomes "The guide covers repositories with no change in progress." |
| §3: "The download and checksum lines are checked by the release's own verification, not by this guide's test." | Delete. Keep the "must print `install.sh: OK`" instruction before it. |
| §7: "The test covers zsh, where those lines are in `~/.zshenv`. Other shells keep them in a different startup file, which the test does not cover." | Becomes "These lines are in `~/.zshenv` for zsh. Other shells keep them in a different startup file." |
| §7: "The saved installs the test runs carry no repository Cursor agent files, so the test does not prove the `.cursor/agents` line." | Delete. |
| §7: "This guide does not cover that file: the test leaves it in place and does not show whether deleting it is safe." | Becomes "This guide does not cover that file and does not say whether deleting it is safe." |
| All 20 `<!-- upgrade-step: <name> -->` marker comments | Delete. |

These are the passages found at grooming time. The builder greps the guide for `test` once more
and applies the same rule to any other sentence about the retired test: delete it if it only
describes test coverage, or rewrite it as the guide's own scope if the limit still matters to a
reader. The links to the guide from `README.md` and `docs/install/install.md` stay.

## Verification

- The full suite passes (`build.test_command`). The budget runner no longer lists the deleted shard,
  and no runtime-budgets row is orphaned.
- `TestAliasInvocationMatchesDocket` runs in the `cmd/docket` package and passes. As a mutation
  check, break the comparison and confirm the test goes red.
- The whole-repo grep above returns only point-in-time records.

## Left to the builder

- The exact wording around the deleted sentences, as long as no step, command or table changes.
- Whether the moved alias test keeps its own file in `cmd/docket` or joins `main_test.go`.
