# Change manifest and ADR fields

The exact fields of a change file and of an ADR (an architecture decision record: one file per
decision, immutable once accepted) are defined once, in the `docket-convention` skill, and this
page only points you there. Nothing is copied here, because a copy would silently fall behind the
skill the tooling actually reads.

Owner: [`../../skills/docket-convention/SKILL.md`](../../skills/docket-convention/SKILL.md). Read
the sections named below (cited by their exact heading text, so the pointer stays greppable if the
skill is reorganized).

## Where each field group is owned

- **Change manifest fields** (the frontmatter block at the top of every change file — a change is
  one unit of planned work, roughly one pull request, tracked as one markdown file) — owned by the
  convention section **"Change manifest (frontmatter at the top of each change file)"**. That
  section is the live list of every manifest key and its meaning: identity and title, lifecycle
  status, priority and type, dependencies (`depends_on`), the claim fields that record which branch
  will carry the work and when it was taken, and the spec/plan/results/pr links.
- **Change body sections** (the headings the manifest's prose body may carry) — owned by the
  convention section **"Change body sections"**.
- **ADR fields and body** — owned by the convention section **"ADR file (`<adrs_dir>/<NNNN>-<slug>.md`)"**,
  which fixes the frontmatter an ADR carries and the required body sections (`## Context`,
  `## Decision`, `## Consequences`).

To see the fields on a real change or ADR in your own repo rather than in prose, open any file
under the configured changes directory or ADR directory (their locations are themselves config
keys — see [`config-keys.md`](config-keys.md)).
