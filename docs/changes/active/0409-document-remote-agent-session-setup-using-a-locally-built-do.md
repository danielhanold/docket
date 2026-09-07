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
spec: 'docs/superpowers/specs/2026-09-07-document-remote-agent-session-setup-using-a-locally-built-do-design.md'
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
| Spec | [2026-09-07-document-remote-agent-session-setup-using-a-locally-built-do-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-07-document-remote-agent-session-setup-using-a-locally-built-do-design.md) |
| ADRs | [ADR-0104](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0104-the-capability-catalog-is-the-authoritative-executable-cli-s.md) |
<!-- docket:artifacts:end -->

## Why

Remote agents can have repository connector write access while lacking the local Docket binary, installed workflow assets, shell Git credentials, or complete repository history. Existing machine-install guidance does not give an agent starting in another repository a complete, verifiable session setup sequence. Ephemeral containers repeat these gaps, and installing files mid-session does not prove that the harness registered them.

Provide a discoverable playbook that uses the tracked development installer and supports both native Git publication and authorized connector publication of Docket-generated proposed changes.

## What changes

Add a remote-session installation guide and a canonical agent-facing reference, linked from the existing installation, harness, convention, new-change, and grooming entry points.

Cover source cloning outside the consuming repository, Go/toolchain and network prerequisites, the tracked development installer, explicit repository harness opt-in, persistent PATH, conditional unshallowing and required refs, opt-in startup automation, and separate binary/registration/publication readiness checks.

Define connector publication by harness-agnostic capabilities. For attended change.create and change.groom only, Docket remains the author of all metadata and generated views. A connector may publish a uniquely attributed surviving candidate after shell transport/authentication failure, preserving its complete tree, original parent and transaction message. Require safe atomic ref update semantics, reject stale bases and ambiguous candidates, and verify remote bytes and ancestry before reporting success. Specify lost-response and replay behavior without inventing an export CLI or relying on harness-specific tool names.

The linked design specifies the documentation surfaces, supported operation boundary, recovery checks, and implementation acceptance matrix. The user explicitly selected connector support and harness-agnostic specification during grooming.

## Out of scope

New Docket transport or candidate-export implementation; connector-to-shell credential bridges; hosted services; automatic migration or hook installation; unattended or arbitrary-operation connector recovery; manual change allocation or generated-view rendering; force pushes; changes to claim, gate, PR, or merge semantics. This grooming publishes a design only.
