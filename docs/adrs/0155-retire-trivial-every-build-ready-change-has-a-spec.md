---
id: 155
slug: 'retire-trivial-every-build-ready-change-has-a-spec'
title: 'Retire trivial: every build-ready change has a spec'
status: 'Accepted'
date: '2026-10-19'
supersedes: [6]
reverses: []
relates_to: [153]
change: 573
---

## Context

Grooming had a `trivial` exit that made a change build-ready without a spec. In practice that exit came after a design conversation and squeezed the resulting decisions into the change body, which lives only on the metadata branch and never reaches the PR or main. It also saved nothing: implement-next still plans, builds, reviews, and writes results for a trivial change. ADR-0006 bounded autonomous grooming (spec, trivial, or abstain exits; an adversarial critic; kill and defer never autonomous). ADR-0153 added the hold exception to build selection.

## Decision

Every build-ready change carries a spec. The `trivial` flag is retired from the domain model, groom outcomes, the board, `open`, and the skills. Autonomous grooming has two exits: spec and abstain.

ADR-0006's two bounds carry forward unchanged: an adversarial critic gates every build-ready exit, and kill and defer are never autonomous.

ADR-0153's hold exception is kept: build-ready does not mean the autonomous builder will build a held change.

Legacy `trivial: true` lines in existing records stay as inert data (an unknown frontmatter key); nothing rewrites them.

## Consequences

The design for every built change rides the PR as a spec, so reviewers and main see it. Grooming and auto-groom lose one exit, and build-readiness reduces to `proposed` + `spec:` + satisfied dependencies (+ the stacking and hold conditions). A small change now needs a short spec instead of a flag; the cost is minor since the build pipeline already ran in full for trivial changes. Old records carrying `trivial: true` are read without error and without effect.

## Alternatives considered

Repair the trivial exit by requiring decisions to be written into the change body: rejected, because the design still stays off the PR and it saves nothing over a short spec.

Keep `trivial` only for capture (new-change): rejected, it was last used around #358 and keeping it preserves the model complexity for no use.
