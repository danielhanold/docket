# stack-children.sh — the stacked descendants of one change

## Purpose

Answers the question every merge of a stack parent has to ask first: **what is stacked on this
change right now?** It prints the transitive `stacked_on:` descendants of one change — parents
before children — with each one's status and PR, so a caller can hard-block on the open ones,
retarget their PRs, and decide whether the stack close-out has anything to do.

This is the **oracle** for `docket-finalize-change`'s open-children gate (spec §8), its step 3.5
close-out gate (spec §7), and any other surface that needs the child set. The parent's derived
`## Stacked children` row is a *human view* of the same scan, not an oracle: `render-change-links.sh`
regenerates it on a link-bearing write **to the parent**, so a child stacked on an already
`implemented` parent — created after the parent's last such write — does not appear in it until
something writes the parent again. Spec §11 settles the point directly: the child's `stacked_on:` is
the single source of truth, and the gate "derives the child set by scanning … never by reading a
parent-side list".

Pure read. No writes, no commits, no network — and, unlike `stack-base.sh`, no git calls at all:
nothing here turns on a remote ref. The graph walk itself lives in `scripts/lib/docket-stack.sh` as
`stack_descendants`; this script is the CLI over it.

## Usage

```
stack-children.sh \
  --changes-dir DIR \
  --id N \
  [--open-only]
```

- `--changes-dir` — the docket changes directory: the parent of `active/` and `archive/`
  (required). Both are scanned — an archived descendant is still a descendant.
- `--id` — the change id, padded or bare (required). `0298` and `298` are the same change; the id is
  canonicalized with `10#` at the argument boundary.
- `--open-only` — keep only the descendants a merge of this change would **strand**: everything
  neither terminal (`DOCKET_STATUSES_TERMINAL` — `done`, `killed`) nor `stacked-merged`. This is
  spec §8's gate set. A `stacked-merged` child's code is already in this branch and rides the merge;
  a `done` or `killed` one is settled.

The whole flag set is validated before any work runs, so a caller fixing one usage error is not sent
back for the next one a call later.

## Behavior

One line per descendant, **parents before children** (breadth-first — the order the close-out
promotes in and the order a report should name them in):

```
<padded id> <status> <pr-or-dash>
```

`<pr-or-dash>` is the change's verbatim `pr:` value, or `-` when it has none, so every row keeps the
same three whitespace-separated columns and a caller can `awk '{print $1}'` without a special case.

`stacked_on:` and `pr:` are read with the **anchored** `fm_field` (both are optional keys, and this
repo's change bodies discuss those field names in ordinary prose); `status:` with the unanchored
`field`, which the change template guarantees. A `stacked_on:` line in a change's *body* names no
parent and creates no phantom child.

**`--open-only` does not consult GitHub.** Spec §8 describes an open child as one "with a PR whose
base is this parent's branch"; this script answers the status half from local metadata only. The
caller that retargets PRs checks the base itself.

## Exit codes

| Code | Meaning | What it obliges the caller to do |
|------|---------|----------------------------------|
| 0 | The scan completed. Descendants, if any, are on stdout. | Read **stdout**, not the status, for the has-descendants question: an empty stdout at exit 0 means this change has no stacked descendants. |
| 2 | Usage error: a missing or unknown flag, a non-numeric `--id`, or a `--changes-dir` that does not exist. | Fix the invocation. |
| 4 | `--id` names **no change** in this tree. | Treat as a **data repair** — most often a mistyped id. Never read it as "no children". |

## Invariants

- **An unknown root is exit 4, never an empty answer.** `stack_descendants` answers an unknown root
  and a childless root identically, which is right for a library every renderer calls on every file
  and wrong for a gate's oracle: a mistyped id would read as "nothing to block on" and let a merge
  through for a reason that has nothing to do with the stack. Same "repair the data, never fall
  back" posture as `stack-base.sh`'s exits 3 and 4.
- **`--open-only` is keyed on the shared status vocabulary, not a hand-listed set.** A status added
  to `DOCKET_STATUSES_TERMINAL` is dropped here the day it lands. `stacked-merged` is the one named
  exception, and it is named because it is an **active** status: a filter written as "drop the
  terminal ones" alone would keep it and hard-block every finalize of a stack root — the exact merge
  the close-out exists to let through.
- **The walk terminates and emits each descendant once.** `stack_descendants` seeds its visited set
  with the root and grows it with every emitted id, so a cyclic `stacked_on:` graph — a data defect
  the `stack-invalid` health check names — does not hang the caller.
- **Ids are canonicalized with `10#` at every boundary.** Docket displays zero-padded 4-digit ids,
  and bash reads a leading `0` as an octal prefix: `0030` would silently become 24 and `0008` would
  not parse at all.
- **Read-only.** No writes, no git, no network.
