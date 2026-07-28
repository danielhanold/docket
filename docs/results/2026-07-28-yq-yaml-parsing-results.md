<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0018 — Record the pure-bash YAML/frontmatter parsing stance as an ADR](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-07-28-0018-yq-yaml-parsing.md)**
<!-- docket:backlink:end -->

# Record the pure-bash YAML/frontmatter parsing stance as an ADR — results
Change: #0018 · Branch: feat/yq-yaml-parsing · PR: (opened at step 7) · Plan: docs/superpowers/plans/2026-07-28-yq-yaml-parsing.md · ADRs: 57, 58, 62

**This PR's diff is a plan file and this results file — by design.** Change 0018 is a metadata-only
change whose entire deliverable is one ADR, and the feature branch never carries metadata. The
deliverable is [ADR-0062 — *YAML and frontmatter are parsed by in-repo shell readers — no external
YAML parser*](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0062-in-repo-shell-yaml-readers-no-external-parser.md),
authored on the `docket` branch. An empty-looking diff here is the expected shape, not a broken build.

## Verify (human)

- [ ] **Read ADR-0062 on the `docket` branch and confirm you agree with the decision as recorded.**
      This is a permanent decision record with no automated test behind it — your read *is* the gate.
      In particular check the boundary claim: the ADR bans an external *YAML parser*, and deliberately
      does NOT claim docket has zero external requirements (change 0132 validates a configured GNU
      Bash 4+ runtime).
- [ ] **Confirm the delivery route before merging.** ADR-0062 must reach `main` through *this change's
      own terminal publish at merge*, driven by `adrs: [57, 58, 62]` on the change file. It is
      deliberately NOT on `main` yet. `terminal-publish --adr 62` was **not** run and must not be —
      it would publish the ADR while leaving change 0018 with no route to `done`.

## Findings

- **The ADR is the acceptance criterion, not an incidental step-6 artifact.** The change file called
  this out explicitly, and the plan carried it as its own task, so the usual "record any non-obvious
  decision made during implementation" condition never applied — no decision was made during this
  build; the ADR *is* the build.
- **Verified against the tree rather than taken from the change file's claims** (the change was
  written 2026-06-16 and re-scoped 2026-07-26, so its premises were re-proved at reconcile and again
  at review): zero `yq` invocations exist repo-wide; ADR-0057 and ADR-0058 are `Accepted` and about
  the in-repo readers; `depends_on: [16]` is satisfied (0016 archived `done`); and every reader
  function the ADR names actually exists — `section_body()` / `field_of()` / `harness_agent_line()` in
  `sync-agents.sh`, and `field_raw` / `field` / `fm_field` / `list_field` / `int_field` in
  `scripts/lib/docket-frontmatter.sh`.
- **Two drifts were folded in at reconcile.** The `(no yq)` comment in `scripts/docket-config.sh` had
  moved from line 97 to line 100 — so the ADR cites the file and never a line number. And
  `scripts/lib/docket-runtime.sh` (changes 0133/0152) turned out to be a *fourth* hand-rolled reader,
  under the tightest constraint of all (it must run on macOS system Bash 3.2); it is named in the ADR
  and strengthens the stance rather than complicating it.
- **The one-way link was preserved deliberately.** ADR-0062 carries `relates_to: [57, 58]`; ADR-0057
  and ADR-0058 keep `relates_to: []` and were not touched. Both are `Accepted` and immutable except
  their `status:` line, so the reciprocal link is intentionally absent — the ADR says so in its own
  Consequences, so a future editor does not "fix" it.
- **Review found no Critical or Important issues.** Two prose-precision corrections were applied to
  ADR-0062's Context section before the PR opened: "centralizing the readers" was narrowed to
  "centralizing frontmatter reading" (only frontmatter reading was centralized — four independent
  readers remain), and the `(no yq)` comment is now described as what it is (an aside explaining why
  the `.docket.yml` reader is a flat-scalar reader) rather than as a declared standing property.

## Follow-ups

- **Change #0165** (auto-captured from this build, `type: refactor`) — `scripts/docket-config.sh`
  documents that `migrate-to-docket.sh` carries an identical duplicate copy of the flat-scalar
  `.docket.yml` reader, explicitly left as-is. ADR-0062 makes the hand-rolled readers the permanent
  implementation rather than a stopgap, which raises the cost of that silent divergence. 0165 decides
  between consolidating the two copies and documenting the duplication as intentional (with a
  divergence guard) if `migrate-to-docket.sh`'s pre-install standalone constraint forbids sourcing a
  shared helper.
