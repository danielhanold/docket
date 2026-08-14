---
slug: promised-file-mode-needs-explicit-chmod
hook: "A create-time permission argument is a request, not a promise — the process umask masks it, so chmod what you documented."
topics: [filesystem, permissions, testing]
changes: [311]
created: 2026-08-14
updated: 2026-08-14
promotion_state: candidate
promoted_to:
---

## Apply
When a component *promises* a permission on a file or directory it installs — a documented 0755
binary, a 0644 asset, a mode a downstream reader depends on — the mode argument passed at creation
time (`os.WriteFile`, `os.MkdirAll`, `open(…, mode)`, `install -m`) does not deliver it: the
process umask subtracts from it, silently. Enforce the mode with an explicit `chmod` after
creation, on every write path **and every rollback/restore path**, and pin it with a regression
test that runs under a hostile umask (`umask 077`) rather than the developer's ambient one. A test
that only ever runs under the default `umask 022` cannot observe the bug, so the assertion must set
the umask itself.

The generalization past file modes: any ambient process-inherited setting that *subtracts* from a
requested value — umask, `ulimit`, a locale, an inherited env var — turns a documented guarantee
into a coincidence of the environment the test happened to run in. A guarantee is only guaranteed
where it is asserted under the adversarial setting.

## War story
- 2026-08-14 (#311, PR #207) — The installer's journaled transaction engine trusted
  `os.WriteFile`'s creation mode, so targets it documented as 0755/0644 landed as 0700/0600
  whenever the process ran under `umask 077`. Every test passed on developer machines and in the
  build loop; the defect became observable only when finalize's detached gate runner executed the
  suite under a different umask, which is a real production defect the ordinary environment was
  hiding rather than an environment-bound false RED. Fix: explicit `Chmod` enforcement on both
  apply and rollback, plus a umask-077 regression test that sets the umask inside the test.
  Same gate run also surfaced a second, purely test-side failure — a doubled interior slash in the
  per-job `TMPDIR` compared against `filepath.Join`-cleaned paths — a reminder that a
  path-equality assertion must compare cleaned against cleaned.
