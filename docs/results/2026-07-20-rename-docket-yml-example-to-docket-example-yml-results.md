# Rename `.docket.yml.example` → `.docket.example.yml` — results
Change: #109 · Branch: feat/rename-docket-yml-example-to-docket-example-yml · PR: <url> · Plan: docs/superpowers/plans/2026-07-20-rename-docket-yml-example-to-docket-example-yml.md · ADRs: 48

## Verify (human)

<!-- The automated suite cannot see the thing this change exists to fix — highlighting is a
     rendering property of GitHub and your editor, not of the repo. Check it at the merge gate. -->
- [ ] On the PR's **Files changed** tab, confirm GitHub renders `.docket.example.yml` with YAML
      syntax highlighting (the old `.docket.yml.example` rendered as plain text). This is the
      entire point of the change and no test can assert it.
- [ ] Open `.docket.example.yml` in your editor and confirm it highlights and folds as YAML.
- [ ] After merge, confirm ADR-0048's `## Update` note reaches `main` via terminal-publish
      (the change carries `adrs: [48]`, so finalize should carry it; it is committed on
      `origin/docket` at `0ef4a2a` and was deliberately NOT published onto `main` in this run).

## Findings

**Two shell hazards the build hit — both fixed, both worth remembering.** Neither is a defect in
docket's own scripts (verified below); both are traps for *agent-authored sed sweeps*, which is
exactly what this change was.

1. **The temp-file + `mv` idiom drops the executable bit.** The plan's Global Constraints prescribe
   writing through a temp file and `mv` instead of `sed -i`, for BSD/GNU portability. But `mktemp`
   output is non-executable and `mv` replaces the inode wholesale, so the sweep silently turned
   `scripts/docket-config.sh` and `scripts/ensure-global-config.sh` from `100755` into `100644`.
   It did not surface where you would expect: the plan's named per-file checks all passed, and it
   appeared only as three unrelated-looking failures in the whole-suite run. Fixed with an explicit
   `chmod 755`, verified via `git ls-tree` and `git diff --summary` (no mode change in the commit).
   The remedy for next time is `cp -p "$f" "$tmp"` before writing, or `chmod --reference`.
   **This is why AGENTS.md's "run the whole suite at the build gate, never only the tests the plan
   enumerates" rule earned its keep** — the enumerated checks were all green while two scripts were
   broken.

   *Not a live bug in docket's scripts.* Every in-repo temp+`mv` target is a non-executable data or
   doc file, and the two scripts that write through a temp file to a tracked artifact already handle
   mode explicitly — `board-refresh.sh:121` normalizes to 644 with a comment, and
   `ensure-docket-env.sh:55-58` captures the prior mode and restores it with `chmod`. So this is
   ledger material, not follow-up work; no stub was minted.

2. **`for f in $FILES` over an unquoted multi-line scalar silently no-ops under zsh.** zsh does not
   word-split an unquoted parameter the way bash does, so the sweep loop iterated **zero** times
   while still printing its `sweep done` success line. The first sweep attempt touched no files and
   claimed success. Fixed by running the loop under `bash` explicitly. The lesson is the shape:
   a success message printed unconditionally after a loop is not evidence the loop ran.

Both hazards are now annotated in the committed plan file (commit `e80bbff`) so the next agent
copying those recipes is warned.

**A plan claim that was wrong — in the safe direction.** The plan states that a missed escaped-ERE
replacement would leave the `(8)` guard "silently vacuous while still reporting ok". The final
review mutation-tested exactly that (regressing the ERE back to the old form) and found both `(8)`
pointer asserts redden with `<no link>` — the guard's existing `[ -n "$sn_ptr" ]` floor assert
already catches it. The hazard was real; the guard already covered it. Recorded because a plan's
risk claim is not an oracle either.

**Environment note (not a repo issue).** The agent's interactive shell defines a `grep` function
shadowing real grep with `ugrep`, which strips the leading `./` from paths. The plan's verification
greps filter on `^\./docs/...`, so those exclusions silently matched nothing and every historical
artifact leaked into the results, making a clean sweep look dirty. Verification was done with
`command grep` and cross-checked with `git grep`. Worth knowing before trusting any plan-authored
grep-cleanliness assertion in this harness.

## Follow-ups

None minted. The two shell hazards are learnings-ledger material for the close-out harvest rather
than changes of their own — see the *Findings* note above for why neither is a live defect in
docket's scripts. Suggested harvest targets: extend `shell-portability` with the zsh
word-splitting case, and extend `atomic-generated-write` (or a new finding) with the mode-loss
face of the temp+`mv` shape, which that finding currently prescribes without the caveat.

Four active stubs (0102, 0103, 0106, 0108) and two learnings files still name the old filename on
the metadata branch. That is metadata, not the code line; each picks up the new name at its own
reconcile. 0106 is `in-progress` under another agent and was deliberately left untouched.
