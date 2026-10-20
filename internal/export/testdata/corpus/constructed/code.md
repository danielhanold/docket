# Code samples stay exactly as written

Plain prose keeps a bare URL https://go.dev/doc/toolchain, a bare #559, PR #433, ADR-0012, and the unknown 0998.

Inline code keeps `change 0559`, `[a link](https://x.example/y)`, `<!-- docket:artifacts:start -->`, `<https://go.dev/x>`, and `#0563`.

A double-backtick span keeps ``a `change 0567` b`` too.

```text
<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0563 — a title](../changes/archive/x.md)**
<!-- docket:backlink:end -->
Fixed in change 0567 (0563, 0559). See <https://go.dev/x> and [the docs](https://go.dev/doc).


Two blank lines above stay inside the fence.
```

~~~markdown
## Reconcile log
---
title: not frontmatter
---
[ref]: https://example.com/ref
Change #547, groomed 2026-10-19.
<!-- dckt:artifacts:start -->
~~~

Closing prose names change 0998, which no record claims, so it stays.
