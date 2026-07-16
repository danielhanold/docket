---
slug: escape-ere-metacharacters-in-key
hook: "Escape ERE metacharacters in a key before building a grep -E match from it — and note the un-fixed twin of a duplicated helper."
topics: [shell, grep, regex]
changes: [26]
created: 2026-06-19
updated: 2026-06-19
promotion_state: retained
promoted_to:
---

## Apply
Escape ERE metacharacters in a key before building a `grep -E`/regex match from it — and when you fix one
copy of a duplicated shell helper, note the un-fixed twin so the divergence is tracked, not silent.

## War story
- 2026-06-19 (#26, PR #38) — A `.docket.yml` reader interpolated the lookup key straight into an ERE
  (`^[[:space:]]*<key>`), so any future key carrying a regex metacharacter would match unintended
  lines; the same unescaped helper is still copy-pasted in `migrate-to-docket.sh`.
