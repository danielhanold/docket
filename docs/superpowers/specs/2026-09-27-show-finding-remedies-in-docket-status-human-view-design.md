<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0456 — Show finding remedies in docket status human view](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0456-show-finding-remedies-in-docket-status-human-view.md)**
<!-- docket:backlink:end -->

# Status human view: all severities, grouped, with remedies — design

Change 0456. Groomed 2026-09-27.

## Problem

`docket status`'s human renderer (`StatusResult.HumanText`, `internal/app/status_human.go`) shows
only part of the health surface the JSON document carries:

1. **Notices are invisible.** `countFindings` tallies only `error` and `warning`, and `HumanText`
   calls `writeFinding` only for those two groups. Findings of severity `notice` — today the
   configuration diagnostics normalized from config `info` (`deferred-setting`, `inert-setting`;
   five of them in docket's own repository) — appear in `--json` and nowhere in the human view.
   The health line reads `health: 1 error, 0 warnings` with no hint that notices exist.
2. **Remedies are invisible.** `writeFinding` prints severity, code, locator, and message, and
   drops `StatusFinding.Remedy`. Change 0454's `branch-malformed` finding carries a filled-in
   `docket change repair-identity …` command as its remedy — the actionable part — which a human
   running plain `docket status` never sees. The refusal path (`refusalFindings`) carries
   classifier remedies the same way.

Every sibling human renderer already prints remedies by default: `repository prepare`
(`RepositoryPrepareResult.HumanText`), `repository check` and config refusals
(`appendFindingBlock`), and `diagnostic config`. Status is the outlier.

## Decision

The status human view shows **every finding severity, grouped under a heading per severity**, in
fixed order errors → warnings → notices, and prints each finding's **remedy** on an indented
continuation line. Always on — no flag.

### Health line

The health line counts all three severities. Notices never make a repository unhealthy: `ok`
still means zero errors and zero warnings.

```
health: ok (0 errors, 0 warnings, 5 notices)
health: 1 error, 0 warnings, 5 notices
```

Counts use the existing `pluralize` helper for every severity (`1 notice` / `5 notices`). They are
derived from the rendered findings themselves (the existing `countFindings` pattern, extended to
`notice`), so the header can never disagree with the rows beneath it. The JSON `summary` is not
consulted and not changed.

### Grouped findings

After the health line, one block per **non-empty** severity, in fixed order, each preceded by a
blank line and a heading: `errors:`, `warnings:`, `notices:`. An empty severity emits no heading.
A repository with no findings at all prints only the health line (unchanged: `health: ok (0
errors, 0 warnings, 0 notices)`).

Within a group, rows keep landed report order (the order of `r.Findings`), exactly as today's
two-pass error/warning loops do.

A row drops the padded severity column (the heading now carries it) and otherwise keeps today's
shape: two-space indent, code, optional locator (`findingLocator`, unchanged), ` — `, message.

When `Remedy` is non-empty, one continuation line follows the row, indented four spaces:
`    remedy: <remedy>`. A remedy containing embedded newlines has each subsequent line indented to
the same four-space column so the block cannot collapse into the next row; trailing newlines are
trimmed. An empty remedy emits nothing.

```
health: 1 error, 0 warnings, 5 notices

errors:
  branch-malformed change 0454 (branch) — change 0454 records branch: "…", which is not a valid git branch name
    remedy: run: docket change repair-identity --id 454 --expect-version … --expect-pr 312 --expect-head <…>

notices:
  deferred-setting — .docket.yml: build.checkpoint names a capability Go v1 defers; the declaration is inactive and changes nothing
  inert-setting — .docket.yml: runners.opencode.permissions …
```

Config-derived notices carry only a `Field` (no entity, no path), so `findingLocator` — unchanged
— prints no locator for them; their message already names the setting.

### Severity set

The status DTO's severity is a closed set: `error | warning | notice` (`normalizeSeverity` maps
config `info` → `notice`; domain and reposetup findings are `error`/`warning`). The renderer groups
exactly these three. No other severity reaches the DTO today; this design adds no catch-all group
for a hypothetical one.

### Unchanged

- Sections 1–4 of the report (revisions, counts, ready queue, displayed changes) and the failure
  `reason:`/`message:` lines.
- The JSON document: `StatusFinding`, `StatusSummary` (no notice counter is added), and
  `Findings` ordering.
- Which findings exist, their messages, and remedy text.
- `StatusFinding.Related` stays JSON-only.
- Other commands' human renderers.

## Code touched

Confined to `internal/app/status_human.go`:

- `HumanText` §5 — health line with three counts; grouped blocks with headings; doc comment §5
  wording updated ("health totals followed by errors, warnings, and notices, each under its own
  heading").
- `countFindings` — also returns the notice count; its comment no longer says notices are
  excluded.
- `writeFinding` — no severity column; emits the remedy continuation line(s).

No new files, types, flags, or schema changes. The 0310 status spec (§Human report) explicitly
states the human report "is not required to preserve … incidental wording", so the layout change
is within contract; §Human report item 5 is superseded by this design for the health section.

## Testing

Golden tests in `internal/app/status_human_test.go`:

- Rewrite the existing goldens (`TestStatusHumanTextHealthy`, `…Unhealthy`,
  `…FilteredEmptyProjection`, `…EmptyReady`, `…Deterministic`) to the new layout — intentional
  wording changes.
- New: healthy repository with notices only → `health: ok (…, N notices)` plus a `notices:` group.
- New: all three severities interleaved in `r.Findings` → groups render errors, warnings,
  notices in that order, each preserving relative landed order.
- New: a finding with a single-line remedy, and one with a multi-line remedy (indentation pinned).
- New: a severity with zero findings emits no heading.
- Singular/plural: `1 notice` vs `2 notices`.

Mutation probes — each must turn at least one test red:

- remove the notices group;
- swap the group order;
- drop the remedy continuation line;
- count notices toward "not ok" (e.g. `errs == 0 && warns == 0 && notices == 0` for `ok`);
- drop the multi-line continuation indent.

The build gate runs the complete configured suite, not only these tests.

## Out of scope

- JSON output shape, including a `notice_findings` summary counter and adding entity kind to
  `related`.
- Rendering `related` in the human view.
- The set of findings, their severities, messages, or remedy wording.
- `repository check`, `repository prepare`, and `diagnostic config` renderers.
- A flag to toggle any of this.
