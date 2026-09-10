<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0420 — Prevent build workers from assigning zsh's read-only status parameter](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-10-0420-prevent-build-workers-from-assigning-zsh-s-read-only-status.md)**
<!-- docket:backlink:end -->
# Prevent build workers from assigning zsh's read-only status parameter — Results

## Outcome

The maintained gate-call capture instructions now name explicit shell-safe variables and
forbid assigning zsh read-only special parameters, closing the failure mode where a resumed
build worker executed `status=$?` after capturing a `gate.drive.start` JSON response and zsh
aborted the shell (`read-only variable: status`) before the drive id and owner generation could
be parsed.

Three surfaces changed, source and embedded mirror kept byte-aligned via the existing generator:

- `docket-build/references/gate-caller-loop.md` gained a "Shell-safe capture names" paragraph:
  capture the emitted document into `gate_reply` and the invocation's exit code into `gate_rc`,
  both valid in zsh and bash, and never assign `status`/`pipestatus`.
- `docket-build/SKILL.md`, `docket-build-task/SKILL.md`, and `docket-implement-next/SKILL.md`
  each name `gate_reply`/`gate_rc` at their gate-start capture site and forbid the reserved
  parameters, while preserving the existing rule that a missing or malformed first response
  halts rather than rerunning `gate.drive.start`.

A new whole-repository syntactic guard (`internal/repoguard/gatecapture_reserved_param_test.go`)
covers the maintained workflow-markdown corpus (source `skills/` and `agents/` plus their embedded
mirrors). It rejects any reserved-name-abutting-`=` assignment shape and asserts the shell-safe
names actually appear at the gate-capture sites (population floors, so deleting the instruction or
displacing the scan reddens rather than passing vacuously). The skill line/byte budgets in
`internal/repoguard/budgets_test.go` were re-baselined upward once to carry the new naming prose.

## Verification performed

- The reserved-parameter guard is keyed on syntactic shape — a reserved name (`status`,
  `pipestatus`, `signals`, `ARGC`) immediately followed by `=`, left-bounded — never on an
  enumerated list of RHS spellings; its in-file mutation assertions prove that inserting
  `status=$?` at a covered site reddens the guard for the intended reason, while backticked
  parameter names without a trailing `=` (the shape maintained prose uses to reference a
  reserved name) stay legal.
- Source and generated skill surfaces were confirmed byte-aligned after regenerating the
  embedded mirror through the existing generator (the assets drift check passes in the suite).
- The full configured build gate (`go run ./cmd/docket development test`) was driven through the
  native gate driver to a PASSED disposition at the final feature head; the durable build-evidence
  record names that head.

## Findings and limitations

### Guard scope is deliberately bounded to a plausible reserved-name set

The reserved-name set is the zsh read-only special parameters a capture site plausibly reaches for
(`status`, `pipestatus`, `signals`, `ARGC`), derived from `zshparam(1)`, not from repository
spellings. A TOML-style `status = "x"` with spaces around `=` is out of shape and out of scope; the
guard targets the shell capture-site assignment form specifically. This is documented as a stated
limitation in the guard file itself.
