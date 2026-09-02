---
id: 105
slug: 'finalize-s-local-gate-continuation-is-persisted-in-the-owned'
title: 'Finalize''s local-gate continuation is persisted in the owned rebase receipt'
status: 'Accepted'
date: '2026-09-02'
supersedes: []
reverses: []
relates_to: [98]
change: 396
---

## Context

During finalize of change 0364, the async local gate returned `disposition: waiting` from the rebase seam with a continuation (drive id + owner generation), but no CLI path both advanced THAT drive to terminal and recorded finalize evidence through the finalize seam — so operators reached for `gate drive advance` plus an out-of-band `evidence.record`, orphaning detached gate drives and hand-minting evidence (observed live on 0364). ADR-0098's fingerprinted-handoff rule (only the exact owner advances a drive; the owner generation is a caller-held secret) made a keyless CLI re-entry impossible without an owner-private store — and finalize already owned one: the `rebase-receipt.json` in the workspace meta dir.

## Decision

The finalize local gate's WAITING continuation (drive id + owner generation) is persisted in the owned `rebase-receipt.json` and never carried by the caller. The WAITING re-entry is the IDENTICAL `finalize.rebase` invocation (same `--id --version --head`); its receipt-recovery path reads the persisted pair and advances the SAME drive, and on PASSED the finalize seam mints the evidence block and returns `rebased`/`unchanged` as a single-slice run does. The owner generation is receipt-private: the `waiting` CLI document carries `gate.continuation.drive_id` only and never the generation (the in-process type still carries it; only the JSON projection narrows via `json:"-"`). The pair is written on WAITING and cleared in the same call that maps any terminal (passed, failed, every halted cause, seam error), so a dead continuation never wedges the receipt. This REFINES ADR-0098 for finalize: the generation stays a caller-held secret, and finalize's "caller" for this purpose is the receipt, not the human or skill above it. It closes the misuse channel that pushed operators to `gate drive advance`.

## Consequences

A finalize whose gate spans multiple slices resumes with no new operation and no new flag; `FinalizeRebaseRequest.Continuation` is retired. Cost: the receipt gains two optional scalar fields (`gate_drive_id`/`gate_owner_generation`) validated both-empty-or-both-set, and the receipt stays `==`-comparable. The `docket-finalize-change` skill gains an explicit `waiting` route bound to re-running `finalize.rebase` (never `gate drive advance`).

## Alternatives considered

(1) Add a dedicated `finalize.rebase-resume` operation taking the continuation as flags — rejected: it would force the owner generation out of the process and into the caller, breaking ADR-0098's caller-held-secret rule and re-opening the same misuse channel in a blessed form. (2) Leave operators on `gate drive advance` plus an out-of-band `evidence.record` — rejected: it detaches the drive from the finalize seam, so evidence is hand-minted and the receipt never learns the gate reached terminal. (3) Carry the continuation in the finalize skill's own notes across slices — rejected: skill notes are not durable state, and a dropped or stale pair silently orphans a live drive.
