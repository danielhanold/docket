<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0336 — Finalize selects the best merge method permitted by repository and branch policy](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0336-finalize-s-go-merge-verb-hardcodes-merge-honor-the-repo-s-al.md)**
<!-- docket:backlink:end -->

# Finalize effective merge-method selection

**Change:** 0336 · **Date:** 2026-08-21 · **Type:** fix · **Priority:** medium

## Problem

The Go finalize runtime does not choose a merge method. `internal/githubcli.MergePullRequest`
unconditionally invokes `gh pr merge --merge`, so every repository that disables merge commits
rejects the final effect even when it permits rebase or squash. This repository permits rebase and
squash but not merge commits; the defect first blocked the closeout of change 0330 and required a
manual `gh pr merge --rebase` before merged recovery could finish.

The documented workflow has a second defect before that merge point. CLI commands describe an
omitted `--repo-dir` as "default: current directory", but their Cobra flags default to an empty
string and handlers pass that value into application operations. GitHub repository discovery
rejects the empty directory. In `context finalize`, the per-PR probe failure is converted into
unknown facts, so a real PR is falsely surfaced as `pr-unknown`; mutating finalize verbs similarly
cannot resolve the repository. The finalize skill intentionally omits `--repo-dir` and relies on
the advertised default, so merge-method selection cannot work end to end while this seam remains
broken.

## Decision summary

1. Fulfil the CLI contract centrally: every command that advertises an omitted `--repo-dir` as the
   current directory resolves it at the CLI boundary before entering application code.
2. Add no merge-method config knob. Select from methods effectively permitted for the PR's actual
   base branch using the fixed preference order **rebase → merge commit → squash**.
3. Derive the effective set from both repository-enabled methods and active branch rules. Multiple
   applicable restrictions compose by intersection; required linear history excludes merge commits.
4. If the effective set is empty, return `blocked / merge-method-unavailable` before issuing a
   merge. If policy cannot be observed, return `unknown`. Neither condition authorizes an effect.
5. Attempt exactly one method. A server-side rejection remains `denied`; Docket never interprets a
   generic denial as permission to try a lower-priority method.
6. Report the method Docket attempted. Already-merged recovery carries no method because Docket did
   not choose the historical merge's method.
7. Preserve exact-head matching, the attended explicit-id `--admin` gate, no branch deletion,
   authoritative post-attempt reprobe, destination reachability verification, and every other
   finalize conjunct unchanged.

This design follows ADR-0010's finalize chokepoint and verification gate, ADR-0011's consent model,
and ADR-0043's established rebase-first single-maintainer path. It broadens ADR-0043's concrete
rebase invocation into a portable preference order rather than assuming every consuming repository
enables the same method.

## Current-directory resolution at the CLI boundary

Introduce one shared CLI helper for the local `--repo-dir` flags whose help text promises a
current-directory default. The helper reads the flag once:

- A non-empty explicit value is returned unchanged.
- An empty value resolves through the process's current working directory.
- Failure to determine the current directory is an argument error returned before dependencies are
  constructed or an application operation runs.

Every handler carrying that defaulted flag uses the helper instead of calling `GetString` directly.
This includes `context finalize`, all `finalize` verbs, and the other command families that make the
same promise. A command whose contract deliberately requires `--repo-dir` and uses it verbatim,
such as the mutation diagnostic, retains that required explicit interface.

Resolution belongs in `internal/cli`, not `githubcli.DiscoverRepository`. The application and
adapter layers continue to require a concrete directory, which keeps empty input invalid at those
boundaries and avoids teaching only the GitHub path a default that Git, workspace, document, and
repository operations do not share. The resolved directory is the invocation working directory;
it is not silently replaced with a primary worktree or another Git-discovered root.

## Merge-method vocabulary and capability probes

Add a closed `MergeMethod` value type in `internal/githubcli` with exactly three values:

- `rebase` → `gh pr merge --rebase`
- `merge` → `gh pr merge --merge`
- `squash` → `gh pr merge --squash`

The repository capability probe reads `allow_rebase_merge`, `allow_merge_commit`, and
`allow_squash_merge` from GitHub's repository endpoint. All three booleans must decode explicitly;
missing, malformed, or unrecognized data is invalid external output, never a permissive default.

The branch capability probe reads the active rules for the exact PR base branch from GitHub's
branch-rules endpoint. It uses the validated repository host explicitly, constructs the REST path
from the validated owner/name, and path-escapes the full base-branch name so stacked destinations
such as `feat/parent` are not split into endpoint segments. It extracts:

- each `pull_request.parameters.allowed_merge_methods` restriction, intersecting all applicable
  arrays; and
- `required_linear_history`, which removes merge-commit semantics from the permitted set.

No method-specific branch rule means the branch contributes no additional restriction. An empty
intersection is a known incompatible policy. A failed request, malformed rule, empty required
method array, or unknown method token is unobservable/invalid policy and fails closed as `unknown`.
Constraints GitHub does not expose through these read surfaces can still reject the later command;
that remains an ordinary `denied` outcome and does not trigger fallback.

Repository permissions and active branch rules compose as an intersection. Selection is then a
pure ordered function over that effective set:

```text
rebase, else merge, else squash, else unavailable
```

`--admin` does not widen the method set or change its order. It retains only its existing attended,
explicit-id authorization semantics.

## Merge flow

`MergePullRequest` keeps its existing public inputs: repository identity, PR number, expected head,
and the authorized admin boolean. Its authoritative pre-decision PR snapshot already supplies the
actual base branch needed by the branch-policy probe.

The flow is:

1. Validate repository, PR number, and expected full object id.
2. Read the authoritative PR snapshot.
3. Preserve the existing already-merged, head-moved, closed, conflicting, and unknown short
   circuits. Already-merged recovery performs no capability probes because no method will be chosen.
4. For an open, mergeable PR at the expected head, read repository and branch capabilities.
5. Compute the effective set and select the highest-priority available method.
6. If none is available, return `MergeMethodUnavailable` without invoking `gh pr merge`.
7. Issue exactly one merge using the selected method, explicit repository, exact
   `--match-head-commit`, and the already-authorized `--admin` when present. Never request branch
   deletion.
8. Reprobe authoritatively and route the observed state through the existing three-outcome
   discipline.

A non-zero merge exit is still only a candidate denial. The fresh reprobe decides whether the PR
landed, the head moved, the PR became non-mergeable, or it remained cleanly open and mergeable at
the expected head. In the last case the outcome is `denied`. Docket does not try the next preference:
the rejection could be permissions, approval policy, a merge queue, or a settings race rather than
proof that the selected method alone was unavailable.

## Results, observability, and verification

Extend `MergeOutcome` with `method-unavailable`. `FinalizeMerge` maps it to:

```text
result: blocked
disposition: blocked
reason: merge-method-unavailable
```

The message identifies the repository-enabled and branch-permitted method sets so the human can
correct the conflicting setting. This is not `merge-denied`, because no merge was attempted, and
not `unknown`, because the incompatible configuration was observed successfully.

Method is attempt metadata, not a merged fact. Replace `MergePullRequest`'s loose outcome/facts
tuple with a small result value carrying `Outcome`, optional `Method`, and `MergedFacts`; keep the
read-only `ProbeMerged` interface unchanged. Populate `Method` whenever Docket issued the merge
command, including an authoritative denial or an attempt recovered as merged. Leave it empty for
validation failures, pre-effect blocks, and already-merged recovery. `FinalizeMergeResult` exposes
that optional value in its protocol document. The field is evidence of Docket's choice, not an
inference about how another actor historically merged a PR.

Post-merge verification remains graph-shape independent. GitHub must report the authorized PR
merged with the expected original head and base, and its reported merge-result commit must be
reachable from the freshly fetched destination tip. A merge commit, a rebased final commit, and a
squash commit have different ancestry, but all must satisfy that reachability proof. No test or
implementation path may require the merge-result commit to have two parents or equal the original
PR head.

## Components

- `internal/cli` — add the shared defaulted-repository-directory resolver and route every command
  advertising the default through it; retain intentionally required/verbatim flags.
- `internal/githubcli` — define the method vocabulary, decode repository method settings and active
  branch rules, compute the effective set, select the fixed priority, and emit the selected
  `gh pr merge` flag.
- `internal/app/finalize_context.go` and the finalize operation entry points — benefit from the
  resolved directory without adding application-layer defaults.
- `internal/app/finalize_merge.go` — map `method-unavailable`, surface the attempted method, and keep
  all existing merge conjuncts and reachability checks intact.
- CLI, GitHub-adapter, application, and end-to-end tests — cover the working-directory default,
  capability decoding, policy intersection, priority selection, closed outcomes, exact arguments,
  and all three merge graph shapes.
- Protocol fixtures and test fakes implementing the finalize GitHub seam — extend mechanically for
  the new outcome/result field; do not hand-author a second selection policy in fakes.

No `.docket.yml` key, resolver export, sample configuration, README option, or convention config
surface is added. The preference order is product policy, not user configuration.

## Verification

### CLI default

Test the shared resolver with an explicit path, an omitted path, and a current-directory lookup
failure. Exercise `context finalize` and `finalize merge` without `--repo-dir` from a repository
working directory and prove they reach live/fake repository discovery rather than producing
`pr-unknown` or `repo-unresolved`. Retain explicit-path coverage and include at least one non-finalize
command family so the shared contract cannot regress into a finalize-only special case.

### Pure method policy

Use a table covering every repository method combination plus:

- no branch restriction;
- one and multiple `allowed_merge_methods` rules;
- intersecting and conflicting rules;
- required linear history with merge otherwise enabled;
- all enabled → rebase;
- merge plus squash → merge;
- squash only → squash; and
- empty effective set → unavailable.

Unknown tokens, missing required booleans, malformed JSON, empty required arrays, failed probes, and
incorrectly escaped slash-bearing base branches must fail closed. Mutation checks remove each
higher-priority candidate in turn and require the selected method to advance to the next permitted
one.

### GitHub and application behavior

The protocol-faithful fake `gh` runner verifies the full read/read/act/reprobe sequence and exact
arguments for `--rebase`, `--merge`, and `--squash`. Every act retains explicit `--repo`,
`--match-head-commit`, optional authorized `--admin`, and the absence of `--delete-branch`. Empty
effective policy and every probe failure issue zero merge commands. A denied command issues no
second method attempt.

Application tests pin `method-unavailable` to the new blocked reason, preserve `denied` for an
attempted rejection, and check selected-method presence on attempts and absence on already-merged
recovery. End-to-end fixtures exercise successful rebase, merge-commit, and squash graph shapes and
require the reported merge-result commit to be reachable from the destination without assuming a
two-parent commit.

Run the configured whole suite with uncached Go verification where mutation probes are involved.
Because GitHub's live repository fields, active-rule response, and merge behavior are outside-repo
truth, the eventual results record must carry named human verification items for:

1. observing the repository and effective base-branch capability payloads on a real GitHub repo;
2. finalizing change 0336 itself through this repository's rebase-first path; and
3. finalizing a scratch squash-only repository to certify the last fallback and its reachability
   proof.

## Out of scope

- A configurable merge method or configurable preference order.
- Changing repository merge settings, branch rules, branch protection, or merge queues.
- Retrying another method after GitHub rejects an attempted merge.
- Inferring or reporting the historical method of a PR already merged by another actor.
- Changing approval requirements, explicit-id authorization, `--admin` policy, gate execution,
  rebase/repair behavior, terminal closeout, merged recovery, or branch cleanup.
- Change 0327's stacked-child reachability and stale-worktree protections.
- Change 0330's closeout-note channel or change 0331's evidence re-mint path.
- Reintroducing a Bash finalize fallback or altering repository-side merge-method defaults.
