---
slug: absent-target-certifies-permission
hook: "A resolver that treats every absent config layer as 'use the default' reads a NONEXISTENT target as a fully-defaulted one — so the most permissive verdict is what absence produces."
topics: [config, validation, cli]
changes: [305]
created: 2026-08-13
updated: 2026-08-13
promotion_state: candidate
promoted_to:
---

## Apply
Layered config resolution is built to tolerate absence: a missing user file, a missing machine
file, a missing repo file each mean "this layer contributes nothing," and the resolver falls
through to the shipped default. That tolerance is correct **per layer** and wrong **for the
target**. Point such a resolver at a path that does not exist and every layer reports absent for
the same reason a pristine repo would — so the resolved snapshot is indistinguishable from a
valid repo that has simply configured nothing.

The damage is not the empty snapshot; it is whatever **verdict** is computed from it. A
certification phrased as "no blocking keys found" reads a nonexistent target as *clean*, and the
more permissive the verdict's default, the louder the failure: absence does not merely produce a
weak answer, it produces the strongest possible one.

Validate the **target's existence** at the argument boundary — before any layer is consulted, so
the check cannot be skipped by a code path that resolves lazily — and fail with a distinct error
kind. "Target does not exist" and "target exists and is unconfigured" must never collapse into
one exit code, because the caller's next action differs completely. Mutation-test it the usual
way ([[guards-are-code]]): pass a path you know is absent and watch the command **refuse**, not
approve. Every absence-tolerant reader inherits this exposure — the question to ask of one is
"what does it say about a thing that isn't there?", never "does it handle a missing file?"

Related: [[opt-in-signal-not-file-presence]] (gate on an explicit key, not on the config file's
presence) and [[validate-the-whole-input-set-first]].

## War story
- 2026-08-13 (#305, PR #205) — the Go `docket diagnostic config` command accepted `--repo-dir` and
  resolved the four config layers under it. A **nonexistent** `--repo-dir` found no layers, which
  is exactly what a repo with no config files looks like, so the sparse-defaults snapshot came
  back clean and `--for-mutation` **certified a nonexistent repository as mutation-allowed**. Review
  caught it (important-severity finding #1); the fix (`f11cc46a`) validates the directory at the
  argument boundary and returns a distinct error rather than resolving. Nothing about the resolver
  was wrong — each layer honestly reported itself absent; the missing step was asking whether the
  thing being resolved existed at all.
