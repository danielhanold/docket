---
slug: config-edit-trips-its-own-frozen-drift-guard
hook: 'Editing docket''s own `.docket.yml` can fail `TestFixtureDocketSelf`. The fix is to re-derive its expectations from the resolver''s real output, and a spec exclusion naming `internal/config` means ''don''t change the logic'', not ''don''t touch that test''.'
topics: [fixtures, config, testing, drift-guards]
changes: [326, 548]
created: 2026-08-18
updated: '2026-10-16'
promotion_state: candidate
promoted_to:
---

## Apply

docket's tests read `.docket.yml` and `agents/harness-defaults.yml` where they live, so editing either file fails only a test whose expectations the edit actually changes. `TestFixtureDocketSelf` is the usual one: it resolves docket's own `.docket.yml` and checks the finalize test command, the blocker set, and each blocker's layer.

- **Re-derive the expectations by running the resolver and reading its actual output**, never by guessing the new expected set.
- **Samples under `testdata/repositories/` follow the `testdata/README.md` rule:** they are data docket must keep reading, so never edit one to make a failing test pass. Remove one only when docket deliberately drops support for it, recording why in its `PROVENANCE.md`. Adding a new sample is always fine.
- **The spec-exclusion trap.** A change spec may say "do not modify `internal/config`" to protect the real logic (the classifier, the schema, the resolver). The test whose expectations an edit changes usually lives in that same directory. Read the exclusion by **intent** (don't change the decision logic), not by **path** (never touch a file under here): a green-suite requirement plus a config edit forces the expectation update, so a literal path reading makes the spec contradict itself. Surface the call when the two collide, and verify afterward that the protected logic file is byte-untouched by the branch.

## War story
- 2026-08-18 (#326, PR #220 — merged) — Change 0326 contracts docket's own committed `.docket.yml`
  (three deferred switches → `false`) so the Go capability fence permits mutation. That three-line
  edit reddened `internal/config`'s `TestFixtureDocketSelf`, which byte-compares live `.docket.yml`
  to a frozen `testdata/repositories/v0.9.2/docket-self/repo/.docket.yml`. The spec listed
  "do not modify `internal/config`" as an exclusion — and the drift guard lives there. First a
  premium build worker correctly returned **BLOCKED** on the contradiction rather than guessing.
  Resolution (explicit maintainer decision): cut a new `v0.9.4/docket-self` tree with the contracted
  file (global layer carried unchanged), re-point + re-derive `TestFixtureDocketSelf` — the guard's
  own remedy message. The re-derived result stayed `MutationAllowed: false` with a residual
  `auto_capture.enabled` from the fixture's untouched global layer (the fixture models a config shape
  the change doesn't touch), and `capability.go` was confirmed byte-untouched by the branch. Read the
  guard's remedy, treat the exclusion as "don't change the logic," and surface the call.
  ([[migration-host-builds-through-the-frozen-prior-workflow]], [[frozen-corpus-covers-what-it-contains]])
