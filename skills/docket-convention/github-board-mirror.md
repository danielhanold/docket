# GitHub board mirror — mechanics

> On-demand detail for the convention's GitHub board mirror. Read this when `board_surfaces`
> includes `github`. The core contract (one-way · change-files-authoritative · script-owned)
> lives in `SKILL.md`'s *GitHub board mirror* stub; this file is the full mechanics.

The `github` board surface mirrors each change to one GitHub issue (and one Projects v2 item) — **strictly one-way**: change files are the source of truth, the mirror is derived output that is **never read back** (no comments, labels, assignments, or state flow into change files). It rides in the Board pass (`docket-status`) and is **best-effort**, identical to the inline board rule: it needs network + `gh` auth, it self-heals on the next pass, and it **never aborts a build**. The mirror's external-write mechanics are owned by the deterministic `github-mirror.sh` (not agent-constructed `gh` calls); the Board pass only invokes it.

**`issue:` field.** One issue per change, upserted idempotently on the per-change `issue:` field (shape of `pr:`), minted on first sync and persisted into the change file on `metadata_branch`.

**Status → issue mapping (all seven).** Active states (`proposed`, `in-progress`, `blocked`, `deferred`, `implemented`) keep the issue **open**; terminal states close it with the native reason — `done` → closed as **completed**, `killed` → closed as **not planned**. The sync is the **sole writer** of issue open/closed state and reason: a PR may *reference* its mirror issue (a plain `#N` link, for the linked-PR "awaiting merge" view) but never `Closes #N`, which would make GitHub a second writer that cannot express `killed → not planned`.

**Labels — `docket:` namespace only.** Mirror labels are prefixed `docket:` (`docket:status/<state>`, `docket:priority/<p>`, and the derived `docket:readiness/<needs-brainstorm|auto-groom-blocked|build-ready>` / `docket:waiting/<needs-your-merge|not-yet-built>`). docket creates/updates only labels inside that namespace and never touches a label it did not mint, so existing repo labels are collision-proof.

**Issue body.** A visibility pointer, never a second home for the content: a one-way banner, a one-line frontmatter digest, the `## Why` distilled to a sentence or two, and hrefs to every relevant artifact (the change file on `metadata_branch`, the `spec:`, each ADR in `adrs:`, and `plan:`/`results:` once those resolve on the integration branch).

**Projects v2.** The optional half of `github`. When `github_project` is unset, first sync mints a **private** Projects v2 board under the integration repo's owner (Status single-select seeded from the active statuses) and writes its `{owner, number}` back into `.docket.yml` on the default branch — a one-time config commit that keeps later runs idempotent. Missing `project` token scope or any GraphQL failure ⇒ skip Projects and still mirror Issues + labels.
