---
slug: display-format-is-not-a-parse-format
hook: "An id your system DISPLAYS padded will be typed back padded — and bash reads a leading zero as octal, so `printf %d 0237` is 159, silently."
topics: [shell, cli, validation]
changes: [237]
created: 2026-08-08
updated: 2026-08-08
promotion_state: retained
promoted_to:
---

## Apply
When a system renders an identifier in a padded or otherwise decorated display form, that form
becomes the form humans and agents type back at it. Every entry point that accepts the id must
therefore canonicalize the *display* form, not just the *canonical* one — the two differ precisely
because someone chose to decorate the output.

In bash the specific trap is numeric: `printf %d`, `$(( ))`, and `[ x -eq y ]` all read a leading
`0` as an **octal** literal. `0237` is a valid octal number, so it converts to 159 with no error,
no diagnostic, and exit 0. The failure is silent and it lands on a *different, real* record.

- Canonicalize with the explicit base prefix: `id=$(( 10#$raw ))` (or `printf '%d' "10#$raw"`).
- Do it once, at the argument boundary, not at each use site.
- Grep the repo for existing precedent before inventing your own — a codebase that displays padded
  ids has almost certainly already hit this somewhere, and matching the existing idiom is cheaper
  than a second convention.
- The general form outlives bash: any parser with a base-prefix rule (octal, hex, leading `+`) will
  accept the decorated string and mean something else by it.

## War story
- 2026-08-08 (#237, PR #176) — `verify-run 0237` reported on **change 0159** and exited 0. docket
  displays zero-padded 4-digit ids in filenames, the board, every skill's prose, and the slash
  command that invoked this very close-out — so the padded form is the *default* thing to type,
  not an edge case. The fix was `10#` canonicalization at the argument boundary, matching the
  precedent already sitting in `board-checks.sh` and `adr-checks.sh`: two prior scripts had hit
  the same trap and fixed it locally, and the third one re-hit it because the fix had never been
  written down as a rule. See [[validate-the-whole-input-set-first]].
