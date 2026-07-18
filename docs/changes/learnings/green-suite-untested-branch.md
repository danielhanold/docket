---
slug: green-suite-untested-branch
hook: "Green tests are not proof the hard branch was exercised — a mock that omits the tool routes every test through the degrade path."
topics: [testing, fixtures, mocks]
changes: [16, 22, 25, 26, 35, 58, 62, 69, 93]
created: 2026-07-11
updated: 2026-07-18
promotion_state: retained
promoted_to:
---

## Apply
A tool-output mock must mirror the real tool's response shape, nesting and all — and
when the code under test has a best-effort/degrade branch, a mock that OMITS the tool silently routes
every test through that branch, so at least one fixture must carry the REAL tool (and its `lib/`).
Fixtures need real-SHAPED field values and PLURALITY (≥2 of every kind rendered as a list); smoke
against real data inside a real worktree before merge; to cover a conflict/CAS path the competing
writer must DIVERGE the same contended path (mutation-confirmed); give a tool writing to BOTH a user
and a project location SEPARATE dirs; keep fixture stderr 0-byte. A fixture pinning a
*read-modify-write* guarantee must carry a value that DIFFERS from the default the code would
otherwise write — when the stub value equals the fallback, the assertion passes identically against
the blind-set implementation the guarantee exists to forbid. When one change adds TWO independent
filters over the same data, a fixture that keeps them agreeing tests neither's independence — build
the crossed case where one filter's decision contradicts the other's. Green tests ≠ the hard branch
was exercised.

## War story
- 2026-07-11/13 (#58 PR #65; #69 PR #77; #16 PR #30; #22 PR #35; #25 PR #36; #26 PR #38; #35 PR #44
  — merged, one green-suite-untested-branch family) — Seven green suites that never exercised the
  branch they existed to cover. **Mock fidelity:** (a) a `gh api graphql` jq path read one level too
  shallow (`.data.pN.mergedAt` vs `.data.pN.pullRequest.mergedAt`), and the bug hid because the mock
  returned a *flattened* JSON shape `gh` never emits; (b) worse, every full-pass `docket-status`
  fixture pointed `SCRIPTS_DIR` at a mock dir containing **no `render-board.sh` at all** — and because
  the new digest call is best-effort, the missing tool degraded silently on every full-pass test, so
  the change's two headline claims had ZERO real coverage. **Fixture realism:** (c) a golden fixture
  used `pr: 142` where real changes store a full URL and had a single `done` change, so neither the
  URL-format path nor the multi-id concatenation bug was hit; (d) a generator test set
  `DOCKET_HARNESS_ROOT` to the repo root, so the user-level and project-level passes wrote ONE dir and
  an "unlisted skill gets no project file" assertion passed vacuously; (e) a CAS conflict-retry branch
  shipped uncovered because the competing-writer test touched an *unrelated* file, hitting only the
  clean if-branch; (f) a renderer branching on git-remote resolution was smoked in a `/tmp` fixture
  with no origin, so only the degraded bare-path fallback ran; (g) fixtures cloning a fresh
  `init --bare` origin emit `warning: You appear to have cloned an empty repository`, leaving noisy
  stderr.
- 2026-07-17 (#62, PR #94) — `setup-auto-approve.sh` promised it would never *blind-set* the repo's
  `default_workflow_permissions`: it reads the current value and preserves it. The test that existed
  to pin that guarantee stubbed the API's current value as **the same value the fallback would have
  written**, so a blind-set implementation passed the assertion byte-for-byte — the guarantee was
  decoration and its test could not tell the two implementations apart. The fixture now stubs a
  non-default `write` and asserts it survives the round-trip. The discriminating input is the whole
  test: when the fixture value and the default coincide, there is no experiment.
- 2026-07-18 (#93, PR #96) — One change gave `render-board.sh` two independent output filters over the
  same archived-`done` set: a count-based recency window that COLLAPSES old dones out of the archive
  table, and a mermaid pruning rule that KEEPS a done node styled `:::done` when an active change's
  `depends_on` still references it. Each filter had its own assertions and the suite was green, but
  the large-archive fixture pointed its active dependency at a done id that was *inside* the verbatim
  window — so the two filters never disagreed, and the state that actually proves they are independent
  (a done collapsed out of the table yet still styled in the graph) was verified only by reading the
  code. Review caught it as Minor; the fixture now aims the dependency at a collapsed month. Two
  filters that always agree in the fixture are, as far as the suite knows, one filter.
