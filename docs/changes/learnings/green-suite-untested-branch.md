---
slug: green-suite-untested-branch
hook: "Green tests are not proof the hard branch was exercised — a mock that omits the tool routes every test through the degrade path."
topics: [testing, fixtures, mocks]
changes: [16, 22, 25, 26, 35, 58, 69]
created: 2026-07-11
updated: 2026-07-13
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
and a project location SEPARATE dirs; keep fixture stderr 0-byte. Green tests ≠ the hard branch was
exercised.

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
