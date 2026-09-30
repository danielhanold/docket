<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0474 — Rename re-arm to re-enable and retire the lifecycle 'terminal' and non-run 'fence' names](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0474-rename-re-arm-to-re-enable-and-the-terminal-fence-lifecycle.md)**
<!-- docket:backlink:end -->
# Rename re-arm to re-enable and retire the lifecycle 'terminal' and non-run 'fence' names — Results

**Human action:** Yes, after merge. Rebuild the installed `docket` binary as AGENTS.md requires. Then re-run `docket install` in every consumer repository, because their installed skills still say `outcome: rearm` and `terminal-close-out.md`. Until they do, a groom that uses those skills gets a loud `invalid-outcome` refusal.

## Outcome

This is ADR-0129 family (d), the last vocabulary family. It is a hard cut with no aliases. Only names change; behavior stays the same.

- **Re-enable.** `change.groom` `outcome: rearm` is now `outcome: re-enable`, and `nothing-to-rearm` is now `nothing-to-re-enable`. The old outcome is refused as `invalid-outcome`, and a new test pins that. The groom-next, convention and auto-groom skills, the CLI help, the result text and the glossary all say "re-enable" now.
- **Shared-setting guard.** The config warning `fenced-setting-ignored` is now `shared-setting-ignored`. The config package's fence-named identifiers are renamed too: `CodeSharedSettingIgnored`, `scopeRepoOnly`, `applySharedSettingGuard`, and so on. Tests are renamed to match.
- **Final.** Seven lifecycle codes are renamed:
  - `final-backlink-pending`
  - `final-notes-frozen`
  - `change-final-claim-stamp`
  - `drop-final-claimed-at`
  - `not-final`
  - `skipped-final`
  - `adr-update-after-final`

  `Status.Terminal()` is now `Status.Final()`. Messages and the two closeout commit subjects ("final backlinks …") are updated. The CLI help for finalize and maintenance now says "closing half". About 70 comment lines in the lifecycle sense were reworded.
- **Close-out reference.** `skills/docket-convention/references/terminal-close-out.md` is now `close-out.md`. All four skill references and the size budget moved with it.
- **Other fences.** These names no longer use "fence", so "fence" now means only the run fence, a Markdown code fence or a frontmatter fence:
  - `resolveBoardSurface`
  - `requireLearningsEnabled`
  - `haltPreflight`
  - the `…Refuses…` test names
- **Seal.** Eleven family (d) rows were added to `retiredVocabulary`, so the table floor goes from 78 to 89. Negative controls were added for the kept senses.

Departures from the spec:

- **Extra sites.** The build renamed more than the grooming trace listed: the `repoFenced`/`wantRepoFenced` locals, "repo-fenced" in `edge-paths.md`, a comment in `install.go`, owned-ref comments in `gitcli/rebase.go`, the `allowTerminal` parameter, a learnings preflight test separator, and an owned-ref integration test message.
- **Kept as run sense.** "a durable terminal record" in `finalize_cleanup.go` and "its terminal write" in `finalize_rebase.go` describe gate-run and receipt records, not the change lifecycle.
- **Diagnostic order.** Warnings are sorted alphabetically by code. `TestDiagnosticOrdering` now expects `obsolete-setting` before `shared-setting-ignored` on the same path, because the renamed code sorts later than `fenced-…` did. The sort rule itself did not change.

## Human actions and testing

### Important — reinstall docket in consumer repos after the merge

**Why:** installed skill copies outside this repo still name the retired `outcome: rearm` and `terminal-close-out.md`. Until they are reinstalled, groom-next's re-enable exit is refused.

**Prerequisite:** the post-merge binary rebuild from AGENTS.md is done.

1. In each consumer repository, run `docket install`.
   Expected: it completes without error.
2. Run `ls ~/.claude/skills/docket-convention/references/`.
   Expected: `close-out.md` is listed and `terminal-close-out.md` is not.
3. Run `grep -rn "outcome: rearm" ~/.claude/skills/`.
   Expected: no output.

### Optional — watch the hard cut refuse the old outcome

1. Build the binary from this branch.
2. Run `docket change groom` with an input file containing `outcome: rearm` for any proposed change id.
   Expected: the command is refused with `invalid-outcome`, and nothing is written.

## Verification performed

- Each task ran focused, driver-run gates with TDD evidence: RED as a compile failure on the new identifiers, then GREEN.
- Hand mutations:
  - `outcome: rearm` planted in groom-next: the seal went red naming `re-enable`, and passed again after revert.
  - `"skipped-terminal"` planted in `finalize_retarget.go`: the seal went red naming `skipped-final`.
  - `"skipped-final"` changed to `"skipped-finalX"`: `TestFinalLifecycleCodeSpellings` went red.
  - `shared-setting-ignored` mutated: `TestDiagnosticOrdering` went red.
  - A `rearm` alias added to the groom switch: the hard-cut test went red.
- Shards: all three renamed integration tests keep their shard prefixes. The test population went from 3658 to 3662.
- Unchanged: no frozen path under `testdata/` or in point-in-time records was touched, and no harness golden changed.
- The closing whole-repo grep over maintained source classified every remaining hit:
  - `rearm`/`re-arm`: only the hard-cut test, the frozen "0382: +typed rearm exit" budget history comment, the seal rows, and the `suiterunner` signal re-arm (kept).
  - `fence`: Markdown code fences and frontmatter fences, the run fence, the frozen `fenced-machine-keys` fixture name, the ADR-0019 filename link, and "defence".
  - `terminal`: process and run sense, decision-procedure "terminal branch", TTY, the historical `DOCKET_STATUSES_TERMINAL` comment, and one "terminal period" (punctuation).
  - No lifecycle "terminal" and no non-run "fence" remains.

## Known issues and follow-ups

- **Pre-existing gofmt drift (confirmed, unrelated).** `internal/githubcli/comment_integration_test.go` was already gofmt-dirty on `main` before this change. It has no functional impact. Next step: a one-line gofmt cleanup in any later change.
- **"not final" wording for a killed change (suspected, carried over).** If a killed change can reach `finalize cleanup`'s default branch, its "change is not final" message is inaccurate, just as the old "not terminal" one was. This change keeps behavior identical and does not fix it. Next step: a human decides whether to capture a change with `docket change create`.
- **Row 59 `skipped-final` on stacked-merged children (known, accepted).** `skipped-final` is also emitted for stacked-merged children, which are not final. This trade-off was accepted at grooming (spec D3) and is documented on the Go constant and in the glossary.
