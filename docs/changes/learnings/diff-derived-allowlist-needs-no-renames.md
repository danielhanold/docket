---
slug: diff-derived-allowlist-needs-no-renames
hook: "A safety predicate that asks 'did every changed path stay inside this directory?' must pass `--no-renames` — git's default rename detection prints only a move's DESTINATION, hiding the source path that left the allowlist."
topics: [git, guards, security, review]
changes: [190]
created: 2026-08-07
updated: 2026-08-07
promotion_state: candidate
promoted_to:
---

## Apply

When a guard derives an allowlist verdict from `git diff --name-only`, pass **`--no-renames`**.
Git's default rename detection collapses a move into a single entry and prints only the
**destination** path. A file moved *out of* a watched tree and *into* the allowlisted one therefore
reports as a delta wholly inside the allowlist — the source path is never listed, so the predicate
answers "nothing left the tree" when something did.

Verify it directly rather than trusting the shape of the output:

```
git mv scripts/a.sh docs/results/a.sh
git diff --name-only HEAD~1 HEAD        # -> docs/results/a.sh          (source invisible)
git diff --no-renames --name-only ...   # -> docs/results/a.sh
                                        #    scripts/a.sh              (both, as the guard needs)
```

The general rule: a predicate about **what changed** must not be built on a porcelain default tuned
for **human readability**. Rename detection, similarity thresholds, and `-M`/`-C` all exist to make
diffs pleasant to read; each one discards information a safety check depends on. Ask what the
default is summarizing away before deriving a security decision from it.

## War story

- 2026-08-07 (#190, PR #173) — Change 0190 let `docket-finalize-change` skip its post-rebase suite
  run when the only commits since the green build evidence touched `<results_dir>/`. The whole
  safety argument was that the results tree is suite-invisible: nothing under it can change what the
  suite says. Whole-branch review found the derivation used a bare `git diff --name-only`, so a
  post-gate `git mv scripts/some-test-helper.sh docs/results/` would present as a 100% docs-only
  delta and earn the skip — while having removed a component of the suite being vouched for. The
  fix was one flag, `--no-renames`; the finding was rated a blocker because the exploit path ran
  straight through the guard the change existed to justify. Nobody had questioned the default,
  because the *reading* of that diff was obviously correct — it was the *summarizing* that wasn't.
