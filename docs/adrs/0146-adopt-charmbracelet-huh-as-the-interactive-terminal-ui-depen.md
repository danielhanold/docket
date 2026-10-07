---
id: 146
slug: 'adopt-charmbracelet-huh-as-the-interactive-terminal-ui-depen'
title: 'Adopt charmbracelet/huh as the interactive terminal UI dependency'
status: 'Accepted'
date: '2026-10-07'
supersedes: []
reverses: []
relates_to: []
change:
---

## Context

Change 538 lets `repository init` and `repository configure-harnesses` choose agent harnesses interactively. That needs a multi-select checkbox picker in the terminal. docket had no interactive UI library; its existing prompts are plain y/N line reads. The options were a numbered line prompt, a hand-rolled picker on golang.org/x/term, or an established TUI form library. Source: spec docs/superpowers/specs/2026-10-07-choose-agent-harnesses-during-repository-init-design.md (Decision 3).

## Decision

docket adopts github.com/charmbracelet/huh (v1.0.0) as its interactive terminal UI dependency. Its first use is the agent-harness checkbox picker in internal/cli/harness_picker.go; huh stays in the CLI layer and internal/app never imports it. The picker is offered only when stdin and stdout are both terminals and --json is off; otherwise the non-interactive path applies. Later interactive prompts reuse huh rather than adding another UI library. Existing y/N prompts are not converted.

## Consequences

A real, accessible checkbox picker with keyboard navigation and no hand-maintained terminal handling. Cost: about 1.3-1.8 MB (roughly 12-13%) more binary (default build 14,751 KB to 16,556 KB; stripped 10,583 KB to 11,950 KB) and 42 new modules in the dependency graph, against 2 for golang.org/x/term. Keeping huh out of internal/app keeps the application layer testable and UI-free. Future prompts have one library to use, avoiding UI-library sprawl.

## Alternatives considered

Numbered line prompt (type the numbers of the harnesses to enable): zero new dependencies, but a clumsy and error-prone UX for a multi-select. Hand-rolled picker on golang.org/x/term: only 2 new modules and a smaller binary, but docket would own raw-mode terminal handling, key decoding, redraw, and cross-platform edge cases indefinitely. Both rejected in favour of huh despite its size cost.
