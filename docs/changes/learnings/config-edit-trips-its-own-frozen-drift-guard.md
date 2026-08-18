---
slug: config-edit-trips-its-own-frozen-drift-guard
hook: "Editing a file a frozen fixture byte-copies trips that fixture's drift guard — the sanctioned fix (new versioned tree + re-derive) is the guard's OWN remedy message, so obey the guard, not a spec exclusion that names the guard's directory but did not foresee it."
topics: [fixtures, config, testing, drift-guards]
changes: [326]
created: 2026-08-18
updated: 2026-08-18
promotion_state: candidate
promoted_to:
---

## Apply

Some tests keep a **frozen byte-for-byte copy** of a live file and fail the instant the live file
drifts from the copy (a drift guard). Docket's `TestFixtureDocketSelf` copies the repo's own
`.docket.yml`; the harness/status fixtures copy other live inputs. The moment a change edits the live
file — even a legitimate, intended edit — the guard reddens.

The reflex is to treat that red as collateral damage to route around. It is not: the guard's **own
failure message tells you the fix**, and it is a specific protocol, not "edit the frozen copy":

- **Never edit a frozen fixture in place** — it is an immutable input. A new upstream state gets a
  **new versioned tree** (e.g. `testdata/repositories/v0.9.4/…`), copied from the prior version with
  only the changed file overwritten, everything else carried verbatim, plus a `PROVENANCE.md`.
- **Re-point only the one test** to the new tree and **re-derive its expectations by running the
  resolver and reading the actual output** — never by guessing the new expected set.
- The guard stays **live** afterward (still byte-compares, still fires on the next drift) and is not
  weakened.

**The spec-exclusion trap.** A change spec may say "do not modify `internal/config`" (or whatever
directory) to protect the real logic — the classifier, the schema, the resolver. A drift-guard's
fixture and its re-derived expectations usually live in that same directory. Read the exclusion by
**intent** (don't change the decision logic), not by **path** (never touch a file under here): a
green-suite requirement plus a config edit *forces* the fixture re-baseline, so a literal path
reading makes the spec self-contradictory. When the two collide, the drift guard's own remedy is the
tie-breaker, and it is a maintainer call worth surfacing — but it is a fixture-maintenance decision,
never a logic change. Verify afterward that the protected logic file is byte-untouched by the branch.

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
