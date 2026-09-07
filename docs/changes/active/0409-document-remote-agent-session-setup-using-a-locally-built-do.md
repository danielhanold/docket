---
id: 409
slug: 'document-remote-agent-session-setup-using-a-locally-built-do'
title: 'Document remote-agent session setup using a locally built Docket binary'
status: 'proposed'
priority: 'medium'
type: 'docs'
created: '2026-09-07'
updated: '2026-09-07'
depends_on: []
stacked_on:
related: [311, 322, 340, 351, 392, 394, 399]
discovered_from: []
adrs: [104]
spec:
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| ADRs | [ADR-0104](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0104-the-capability-catalog-is-the-authoritative-executable-cli-s.md) |
<!-- docket:artifacts:end -->

## Why

Remote agents can have GitHub write access while lacking the local Docket binary, installed workflow assets, shell Git credentials, or full repository history that Docket operations require. The existing machine-install documentation does not give an agent starting in another repository a complete session bootstrap playbook. Agents consequently improvise bare go build commands, lose build identity, miss PATH or harness registration requirements, or hit repository.prepare refusals on shallow clones. Ephemeral containers repeat the problem every session.

The requested playbook should make a remote agent able to bootstrap the tracked installer from public source, establish the consuming repository's prerequisites, and verify readiness before performing metadata operations. GitHub connector permissions, public clone access, and authenticated shell Git push access must be treated as separate capabilities.

## What changes

Add a discoverable remote-session setup playbook to the installation documentation, linked from the installation entry point and relevant harness pages. Cover a manual first-session sequence and an opt-in consuming-repository startup-hook example using the same sequence.

1. Make the public danielhanold/docket repository available to the session through the host's supported attachment or Git proxy mechanism where required. Clone its source outside the consuming repository, for example under ~/docket. Keep the source checkout available for the lifetime of the development install's linked skills. Distinguish the source checkout from the consuming repository and record the source revision used.
2. Check for Git and Go, plus GitHub CLI availability where the intended workflow requires it. Read the Go requirements from the checked-out go.mod rather than permanently duplicating versions. At proposal time it declares go 1.26.0, toolchain go1.26.5, two direct dependencies and two indirect dependencies. Explain automatic toolchain selection when enabled, and the network access required for toolchain/module downloads and verification. If Go is absent or downloads are unavailable, report the concrete prerequisite; do not assume every remote host shares the original session's allow-list.
3. Use the tracked development installer from the source checkout. Preserve this concrete Claude example:

```sh
cd "$HOME/docket"
go run ./cmd/docket development install \
  --source "$HOME/docket" \
  --harness claude \
  --repo-dir /path/to/your/repo
```

Select the actual supported harness for other environments, including Codex. Explain that the installer builds and stamps the binary, links skills, generates agents, and reconciles repository dispatch surfaces only under the repository's explicit agent_harnesses opt-in. Document the committed versus machine-local opt-in choice and make resulting tracked instruction-file changes visible. Do not substitute a bare go build, which skips the install workflow and build-identity stamping.
4. Put ~/.local/bin on PATH for the active session and every subsequent agent/tool shell. A child hook's export alone does not persist into its parent process: use the host's supported environment-persistence mechanism. If a different writable binary directory already on PATH is needed, pass the installer's --bin-dir option; /usr/local/bin is an environment-specific possibility, not a universal requirement or reason to bypass the installer.
5. Ensure the consuming repository has full history before repository.prepare. Check git rev-parse --is-shallow-repository and run git fetch --unshallow only when true; verify completion and the required remote refs. Explain that unshallowing alone does not necessarily repair a narrow single-branch fetch configuration. Surface unavailable history or shell Git authentication rather than silently proceeding.
6. Provide a repeatable SessionStart hook example for a harness that supports it, and describe the equivalent pre-session setup point for other hosts. Rebuild/reinstall in each fresh container; make repeated runs within a session safe. Preserve existing checkouts, unrelated hooks, repository changes, and configuration. Establish when the hook runs relative to skills/agent discovery: a successful mid-session install must not promise that newly generated wrappers are registered. Document a fresh harness process where required.
7. Finish with explicit readiness checks: binary resolution on PATH, truthful source commit/build date, installed assets, actual harness registration when the requested workflow needs it, full consuming-repository history, and the authorized Git transport. Bootstrap and validate docket capabilities --json, resolve repository.prepare through that catalog, and honor its typed outcome before continuing. Subsequent request shapes come from the catalog-resolved schema operation. Preserve existing init/migrate and refusal contracts.

Acceptance: exercise the documented installer in a clean remote-style container against a separate consuming repository; demonstrate meaningful build identity, PATH in a subsequent tool shell, conditional unshallow behavior for shallow and already-full clones, repeat setup, and a successful repository.prepare. Check the documented failure paths for missing Go/network access, missing shell Git write capability, missing repo harness opt-in, and late agent registration. Include Claude and Codex setup guidance without claiming untested hosts are supported. Keep the proposal as planned documentation work; implementation and detailed grooming follow separately.

## Out of scope

Replacing the tracked installer or metadata transaction engine; hand-editing BOARD.md or allocating change IDs through the GitHub contents API; adding a hosted Docket service or connector-to-Git credential bridge; bypassing host permissions, network restrictions, installer ownership checks, or repository.prepare refusals; automatically migrating consuming repositories; globally enabling startup hooks without repository opt-in; implementing the playbook as part of this capture.
