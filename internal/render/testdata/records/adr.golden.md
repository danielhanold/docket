---
id: 74
slug: 'canonical-record-serializers'
title: 'Canonical record serializers emit through the shared builder'
status: 'Accepted'
date: '2026-08-16'
supersedes: []
reverses: []
relates_to: [62, 71]
change: 312
---

## Context

New records must be well-formed by construction.

## Decision

All frontmatter is emitted through document.New.

## Consequences

Quoting becomes a construction property, not a runtime check.

## Alternatives considered

A conditional-quoting writer was rejected for needing an enumeration oracle.
