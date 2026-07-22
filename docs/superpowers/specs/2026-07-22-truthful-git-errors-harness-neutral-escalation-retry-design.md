# Truthful Git failures and harness-neutral sandbox escalation retry — design

**Change:** #0128 · **Date:** 2026-07-22 · **Status:** design settled, ready to build

## 1. Incident and root cause

The incident occurred while invoking `docket-implement-next 127` from Codex. Step 0 ran the
canonical Docket preflight and returned:

```text
docket-config: cannot reach origin (git fetch failed) — check the remote/network
docket-preflight: config export failed
```

The implementer treated that as the skill's hard-error case, returned the terminal disposition
`halted`, and stopped before selection or claim. Change 127 was not modified and no feature work
began.

The stated diagnosis was false. The active Codex configuration had
`sandbox_workspace_write.network_access = true`, and a read-only `git ls-remote --symref origin
HEAD` succeeded over the repository's SSH remote. Re-running the fetch-shaped operation exposed
the real failure:

```text
error: cannot lock ref 'refs/remotes/origin/HEAD': Unable to create
'.../.git/refs/remotes/origin/HEAD.lock': Operation not permitted
```

Codex's `workspace-write` sandbox protects `.git` as read-only even when the workspace itself is
writable. A fetch needs to update remote refs under `.git`, so the operation must cross the
harness approval boundary. A retry of the same `docket.sh preflight` command through Codex's
elevated-execution path succeeded immediately and printed `BOOTSTRAP=PROCEED`.

Two independent defects combined:

1. `scripts/docket-config.sh` redirects fetch stderr to `/dev/null` and maps every nonzero return
   to a network-specific message. The script has evidence only that `git fetch origin` failed; it
   does not know whether the cause was network, authentication, filesystem permissions, sandbox
   policy, or remote state.
2. Docket's agent contract defines a nonzero preflight as fail-closed, but does not first require
   the agent to recover from a host-sandbox denial through the host's native approval mechanism.
   Once the real stderr is hidden, the agent has no evidence that this recovery applies.

This is not specific to change 127. In a Codex session using the recommended least-privilege
`workspace-write` posture, any Docket run whose first preflight fetch needs to update protected Git
metadata can stop at the same false network gate. The observed run is recorded as
`discovered_from: [127]`; change 127 is not a dependency.

## 2. Goals and non-goals

### Goals

- Report the actual Git failure deterministically, without asserting an unproven cause.
- Let every Docket agent use the same recovery contract regardless of whether it is hosted by
  Codex, Claude Code, Cursor, or a future harness.
- Preserve least privilege: cross the sandbox only for the exact failed command, through the
  harness's approval machinery, and at most once.
- Retain Docket's existing fail-closed postures after the recovery opportunity is exhausted.

### Non-goals

- A shell script that elevates itself. A child process cannot and must not bypass the sandbox that
  launched it.
- Automatic approval, `sudo`, a persistent broad allow rule, or a switch to
  `danger-full-access`/equivalent.
- Treating every Git failure as a sandbox failure. Network, DNS, SSH authentication, missing refs,
  non-fast-forward pushes, conflicts, and repository corruption keep their existing handling.
- A new error-code taxonomy for Git, whose stderr and exit status do not provide a portable
  sandbox-specific code.
- A redesign of `docket.sh`, runner delegation, or the facade's command inventory.

## 3. Chosen design

### 3.1 Deterministic resolver diagnostic

Stage 1 of `scripts/docket-config.sh` will capture `git fetch --quiet origin` stderr. On success,
the captured value is discarded and resolver behavior is byte-for-byte unchanged. On failure, the
resolver will:

1. emit a neutral Docket-owned line such as `docket-config: git fetch origin failed`;
2. replay the captured Git stderr without paraphrasing or dropping its substantive content;
3. exit nonzero; and
4. emit no `KEY=value` stdout.

The implementation may capture stderr in a shell variable or a temporary file, provided it
preserves multiline diagnostics and cleanup. It must not use a pipeline whose consumer exits
early. Empty Git stderr still produces the neutral wrapper, so every failure remains diagnosable
at least to the failed operation.

The script and `scripts/docket-config.md` will no longer call this condition “origin
unreachable.” The contract is “`git fetch origin` failed”; reachability is one possible cause, not
the resolver's conclusion. The existing invariant remains: the abort keys on the fetch return
code before cached `origin/HEAD` can be read as authoritative.

This portion is entirely deterministic shell behavior. No agent or model decides what text the
resolver emits.

### 3.2 Harness-neutral recovery contract

The shared `docket-convention` will own one recovery rule inherited by every operating skill:

> When a required Docket facade command or direct Git command fails with evidence that the host
> sandbox or permission boundary denied access, retry the exact command once through the host
> harness's native approval/escalation mechanism. Do not change arguments, introduce `sudo`, or
> broaden the session sandbox. If the mechanism is unavailable, approval is denied, or the retry
> fails, surface the preserved diagnostic and continue with the caller's existing failure posture.

“Exact command” is load-bearing. For the Step-0 incident, the agent retries the outer canonical
`docket.sh preflight` invocation, not an extracted internal `git fetch`. This preserves the facade
as the trust boundary (ADR-0029/ADR-0033), repeats the whole fail-closed operation atomically, and
does not teach the agent private helper internals.

“Once” prevents retry loops and keeps denials meaningful. A successful elevated retry continues
the skill normally. A second failure is the authoritative result for that run.

“Evidence” may come from structured harness denial metadata or the preserved command diagnostic.
The rule does not require a brittle shell regex enumerating every operating system's wording.
Examples such as `Operation not permitted` while creating a lock below a protected `.git` path
are sufficient evidence; an SSH timeout or authentication rejection is not.

### 3.3 Harness ownership

The convention names the capability by role, not by one product's tool syntax:

- Codex requests that the command execute outside `workspace-write` through its scoped sandbox
  approval path. It still runs as the user's ordinary operating-system account; this is not
  `sudo`.
- Cursor or Claude Code uses its own native permission/approval boundary when available.
- A harness with no escalation capability reports that fact and follows the existing hard-error
  or step-specific failure posture.

An autonomous Docket agent's “no human interaction” contract does not mean “never issue an
approval request.” It means the agent does not ask a conversational design question. The harness
may route the scoped request to a human or an automatic approval reviewer according to local
configuration. Docket neither grants nor predicts the verdict.

No harness-specific command or configuration key belongs in the normative rule. Product-specific
examples may be explanatory, but the behavior is expressed as a capability so generated wrappers
and future runners inherit it without a new Docket design.

### 3.4 Placement and skill behavior

The recovery rule belongs beside the Step-0 preamble in `docket-convention`, because every
operating skill loads that contract before acting. It applies both to:

- the canonical preflight call; and
- later facade or direct Git operations whose existing step semantics already say to stop, retry
  a CAS, degrade, or report on failure.

The sandbox retry happens before interpreting the command's ordinary failure posture. It does not
replace that posture. For example, `docket-implement-next` may declare `halted` for a genuinely
failed preflight only after the one eligible sandbox retry has failed or could not run. A
non-fast-forward push still follows the skill's CAS loop, not the sandbox rule.

The operating skills should point to the shared rule rather than copy product-specific prose.
The implementation must derive the set of executable Docket/Git call sites by whole-repository
search and verify that no local instruction contradicts the shared ordering.

## 4. Error flow

```text
docket.sh preflight
  -> docket-config.sh
     -> git fetch origin fails
     -> neutral wrapper + original Git stderr
  -> preflight reports config export failed
  -> agent inspects the preserved cause
     -> ordinary Git failure: apply existing hard-error posture
     -> sandbox/permission denial: request one exact-command harness retry
        -> success: continue normally
        -> unavailable / denied / failed: apply existing hard-error posture with real cause
```

The generic agent decision is intentionally outside the script. Moving it into shell would either
be impossible (the process cannot expand its own sandbox) or would couple Docket to one harness.

## 5. Security properties

- Escalation is command-scoped and approval-mediated.
- The retry uses identical command text and arguments.
- No `sudo`, credential change, remote rewrite, sandbox-mode change, or blanket Git permission is
  introduced.
- There is at most one sandbox recovery retry per failed invocation before normal failure handling
  resumes.
- Preserving stderr may reveal ordinary Git diagnostics already intended for the invoking user; it
  must not add environment dumps, credential output, or command tracing.
- Automatic approval policy remains entirely under the harness/user configuration.

## 6. Testing and verification

### 6.1 Resolver tests

Extend `tests/test_docket_config.sh` through the existing `GIT` seam with a hermetic fake Git
program that succeeds for the initial repository probe and fails the Stage-1 fetch with a unique,
multiline permission diagnostic. Assert all of the following:

- nonzero exit;
- no `KEY=value` stdout;
- the neutral `git fetch origin failed` wrapper is present;
- every substantive line from fake Git stderr is present; and
- the old “cannot reach origin” / “check the remote/network” claim is absent.

Keep the existing destroyed-remote and stale-cache cases. Rename their expectations and contract
language from “unreachable” to “fetch failed” where necessary, without weakening the return-code
and empty-stdout guards.

Mutation-test the diagnostic guard by restoring the current `2>/dev/null` plus network-specific
`die` line: the new fake-permission case must go red. Also mutate away the neutral wrapper while
leaving stderr replayed; the wrapper assertion must go red. These witnesses prove both halves of
the deterministic contract.

### 6.2 Workflow-contract guard

Extend the existing skill/facade wiring tests to pin a single shared recovery section in
`docket-convention`, rather than hand-listing operating skills. The guard must establish the
structural requirements: eligible sandbox/permission denial, exact-command retry, harness-native
approval boundary, one-attempt limit, and fallback to the caller's existing posture. Existing
tests already establish that operating skills load the convention and route Step 0 through the
canonical facade.

Mutation-test this guard by deleting the shared recovery section and confirming the test fails.
Do not duplicate the rule into each skill merely to make a grep pass.

### 6.3 Live acceptance check

Under Codex `workspace-write` with network enabled and `.git` protected:

1. run the canonical preflight normally and observe the preserved lock/permission cause;
2. approve the exact-command elevated retry; and
3. verify preflight reaches `BOOTSTRAP=PROCEED`.

This is a manual acceptance receipt, not a hermetic suite dependency. The automated suite must not
depend on Codex, GitHub, SSH credentials, or a live network.

### 6.4 Build gate

Run the entire repository suite, not only `test_docket_config.sh` and the skill wiring tests.

## 7. Documentation and records

- Update `scripts/docket-config.md` Stage 1, exit table, and invariants to say “fetch failed” and
  describe stderr preservation.
- Update `docket-convention` with the shared recovery contract.
- Keep the incident evidence and current Codex effect in this specification and change record.
  A broader Codex permissions guide is unnecessary for this behavior fix.
- Reconcile should determine whether the implementation makes a non-obvious architectural
  decision beyond ADR-0029, ADR-0033, ADR-0037, and ADR-0038. This design expects no new ADR: it
  preserves the facade boundary and applies the established harness-owned permission model.

## 8. Out of scope

- Editing user-level Codex configuration.
- Enabling full-access mode for Docket agents.
- Persistent auto-approval rules for broad `git` prefixes.
- Shell-based privilege elevation or harness detection inside `docket-config.sh`.
- A general typed Git error classifier.
- Reworking unrelated suppressed stderr sites unless whole-repository reconciliation proves they
  prevent this shared recovery rule at an in-scope Docket/Git boundary.
