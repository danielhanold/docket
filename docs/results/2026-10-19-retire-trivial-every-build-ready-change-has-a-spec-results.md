<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0573 — Retire trivial: every build-ready change has a spec](../changes/archive/2026-10-20-0573-retire-trivial-every-build-ready-change-has-a-spec.md)**
<!-- docket:backlink:end -->

# Retire trivial: every build-ready change has a spec — results

**Human action:** None is required to merge. One optional check is listed below.

## Outcome

`trivial` is gone from the domain model. A change is now build-ready only when it has a spec, on top of the existing dependency, stack-base, and hold rules.

- **Groom:** `change.groom` has no `trivial` outcome. A request naming it is refused as `invalid-outcome` and writes nothing.
- **Board and `docket open`:** the board's `build-ready (trivial)` cell is gone, and so is the `trivial-no-spec` reason in `docket open`. A change with no spec now gets the existing `artifact-unset` answer.
- **New records:** they no longer carry a `trivial:` line.
- **Skills:** the grooming skills tell the brainstorm that its artifact is always a written spec, sized to the change. Auto-groom now has two exits, spec and abstain.
- **Skill, agent, cursor-rule, and user-doc prose** no longer mentions the flag.
- **Legacy records:** a `trivial: true` line is now inert data. A legacy record with no spec reads as needs-grooming, and a groom leaves that line byte-identical.
- **In-flight builds:** `workspace.commit-spec` keeps its quiet `no-spec` no-op, so a build claimed before the upgrade can still finish.
- **ADR-0155** supersedes ADR-0006.

## Human actions and testing

- **Optional: see a legacy record drop back to needs-grooming.**
  - *Prerequisites:* the binary rebuilt from this branch, and any repo whose metadata branch has a `proposed` change with `trivial: true` and no `spec:`.
  - *Steps:* run `docket status`.
  - *Expected:* that change shows as needs-grooming, not build-ready.
  - *Cleanup:* none, because the command only reads.

## Verification performed

- **Full suite:** `go run ./cmd/docket development test` passed at the final head. The first gate run failed in `TestE2EHaltResumeAndReclaim`: an e2e fixture linked a spec file it never wrote. A premium repair worker fixed it.
- **Mutation checks:**
  - The new legacy-record tests (decode, status, claim, groom, open) go red when trivial-as-readiness is put back in the decoder.
  - The new groom-skill guard goes red when "a trivial verdict" is put back in a skill.
- **Review:** a deep-tier review found 1 minor issue and it was fixed. The no-trivial prose guards had checked a hand-picked list of spellings. They now check that the whole word "trivial" does not appear (any case) in the three grooming skills.
- **Budget report:** the budget lines were screening watches only, with no breaches.

## Known issues and follow-ups

- **Five e2e fixtures link a spec file they never write.** The affected tests use `newImplEnv`: `reachImplemented` and the stacked-change tests in `internal/app/finalize_e2e_test.go`.
  - *Impact:* none today. They pass because `env.implement` claims directly and never reads the spec. If a later change makes the claim, reconcile, or prepare step read the linked spec, these tests will fail in the same way `TestE2EHaltResumeAndReclaim` did during this build.
  - *Status:* confirmed latent. There is no workaround and none is needed yet.
  - *Suggested next action:* give those fixtures their spec files, as the repair did for `reachInProgress`. No existing change fits (checked 22). This needs a new change that a human captures.
