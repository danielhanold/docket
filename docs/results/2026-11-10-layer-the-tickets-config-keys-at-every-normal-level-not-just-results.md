<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0592 — Layer the tickets.* config keys at every normal level, not just the repo](../changes/active/0592-layer-the-tickets-config-keys-at-every-normal-level-not-just.md)**
<!-- docket:backlink:end -->

# Layer the tickets.* config keys at every normal level — results

**Human action:** None needed. Everything the change promised is covered by automated tests, and the review found nothing.

## Outcome

The eight ticket settings `tickets.provider`, `tickets.site` and `tickets.project`, plus the five `tickets.auto.*` switches, now layer like any other key: repo-local > repo-committed > global > built-in. A value in the global config or `.docket.local.yml` is used now, where before it was warned about and ignored, and `diagnostic.config` reports which layer it came from. To cancel a global value, a repository writes the default explicitly in `.docket.yml` or `.docket.local.yml`: `""` for a string, `false` for a switch. `docs/reference/config-keys.md` and `.docket.example.yml` explain this, and the stale "no docket command reads these settings" wording is gone. A bare empty value is still rejected on any string key that accepts an empty value, and the error now says to write `""`.

Two side effects follow from `config.RepoOnlyPaths()`:

- **Visibility switches.** When a repository switches visibility, a ticket key in `.docket.local.yml` now moves with the config instead of being dropped.
- **Private repositories.** A private repository reads the global value, and it cancels that value in `.git/dckt/config.yml`.

No skill text needed changing. The ticket skills never said these values come only from the repository config.

## Verification performed

- The full suite passed at f4b62758b with `go run ./cmd/docket development test`: 90 files, 606 asserts. The budget report held only screening notes (parallel-sensitive timings and a serial confirmation).
- Each task wrote its tests first and watched them fail before the fix. Mutation probes confirmed the guards catch a regression: putting `tickets.site` back to repo-only fails the new tests, and so does restoring the old "No docket command reads these settings" line.
- A standard-tier whole-branch review (docket-review-standard) returned no findings.
