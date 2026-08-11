---
id: 91
slug: every-backtick-in-a-double-quoted-region-is-a-violation
title: "Every backtick in a double-quoted region is a violation, including escaped ones"
status: Accepted
date: 2026-08-11
supersedes: []
reverses: []
relates_to: [54]
change: 221
---

## Context

Change 0221 adds `scripts/check-test-source-hygiene.sh`, a checker `scripts/run-tests.sh` runs as a
synchronous preflight over every test file before the first launch, aborting with exit 5 having
executed zero test files.

It exists because during change 0212's build, a backticked `git checkout .` sitting inside a
verbatim-quoted guard anchor executed and silently reverted a worker's uncommitted edits while the
test printed `ok`. This is not an exotic authoring accident: docket's guards deliberately anchor on
verbatim clauses lifted from skill bodies (AGENTS.md; ADR-0054), and those clauses routinely contain
backticked code spans. The mandated guard style is exactly the style that feeds backticks into test
source.

One of the checker's rules bans **any** backtick inside a double-quoted region, bare or
backslash-escaped. The escaped case looks safe and the rule looks over-broad, so the obvious
optimization is to narrow it — either to bare backticks only, or to backticks appearing in an
`assert` call's second argument (the condition string that reaches `eval "$2"`), on the reasoning
that argument one is only a description printed through `printf '%s'` and is therefore inert.

## Decision

The ban is total: any backtick in a double-quoted region is a violation, regardless of escaping and
regardless of argument position.

The narrower argument-position rule was considered and **rejected on a probe, not on principle**.
This shell:

    cond="[ -n \"\`printf %s EXECUTED\`\" ]"
    assert "d" "$cond"

prints `EXECUTED` and then reports `ok`. The backslash-escape is consumed at the **assignment**, so
`$cond` carries a bare backtick; `eval "$2"` then re-parses it and runs it. That assignment is a
plain statement at word index 0 — an argument-position rule would never inspect it, so narrowing
would have reopened a demonstrated execution path while leaving the suite green.

The general mechanism worth recording: the executing vector is **source evaluation**, not the
`assert` helper interpolating its description. Parameter expansion does not re-trigger command
substitution, so a backtick held in a variable's *value* is inert through `printf '%s' "$1"`.
Normalizing the helper to a non-interpolating `printf` form (which 0221 also did, across 114
definitions) is ledger alignment and drift control — it is **not** the safety mechanism. A checker
aimed at the helper would have prevented nothing.

## Consequences

- Enables: a checkable rule with no exception list, and a guard that cannot be argued down site by
  site. The remedy set is small and stated in `tests/README.md` — carry verbatim anchors in
  single-quoted literals or quoted-delimiter heredocs (`<<'EOF'`), escape backticks inside assert
  conditions (the house idiom `'grep -qF "\`span\`" "$f"'`), and where a pattern needs a literal
  backtick beside a `$var`, concatenate a file-local single-quoted `BT` variable, which is inert.
- Costs: 35 pre-existing sites across 18 test files had to be rewritten during calibration, and
  future authors lose the escaped-backtick-in-double-quotes spelling entirely, even where it would
  have been harmless.
- Gives up: precision. The rule knowingly flags inert sites, because the distinction between an
  inert escaped backtick and one that survives into an `eval`'d string is not decidable from source
  position alone — as the probe above shows.
- Residual, documented in the checker header: conditions assembled from variables at runtime are not
  modeled. The scanner reads source, not values.
